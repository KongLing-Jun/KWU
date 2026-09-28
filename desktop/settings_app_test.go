package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

func TestDesktopSettingsFallbacksDefaultToLightTheme(t *testing.T) {
	if got := desktopStartupSettingsFromConfig(nil).DesktopTheme; got != "light" {
		t.Fatalf("startup fallback theme = %q, want light", got)
	}
	if got := defaultSettingsCoreView().DesktopTheme; got != "light" {
		t.Fatalf("settings fallback theme = %q, want light", got)
	}
}

func TestSettingsCoreViewExcludesProviderCredentialState(t *testing.T) {
	cfg := config.Default()
	cfg.DefaultModel = "custom/model-a"
	cfg.Desktop.DefaultBillingSource = "newapi_byok"
	cfg.Providers = []config.ProviderEntry{{
		Name:      "custom",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Models:    []string{"model-a"},
		APIKeyEnv: "SETTINGS_CORE_MUST_NOT_READ_THIS_KEY",
	}}
	t.Setenv("SETTINGS_CORE_MUST_NOT_READ_THIS_KEY", "secret-value")

	view := new(App).settingsCoreViewFromConfig(cfg, "config.toml", userctx.ActorContext{})
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(raw)
	for _, forbidden := range []string{"providers", "officialProviders", "providerKinds", "apiKeyEnv", "keySet", "secret-value"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("SettingsCoreView leaked deferred provider field %q: %s", forbidden, payload)
		}
	}
	if got := view.SelectedModelSummary; got.Ref != "custom/model-a" || got.Provider != "custom" ||
		got.Model != "model-a" || got.BillingSource != "newapi_byok" || got.Availability != "unverified" {
		t.Fatalf("SelectedModelSummary = %+v, want credential-independent BYOK summary", got)
	}
}

type captureTurnRunner struct {
	inputs []string
}

func (r *captureTurnRunner) Run(_ context.Context, input string) error {
	r.inputs = append(r.inputs, input)
	return nil
}

func TestWithFreshSystemPromptReplacesExistingSystemMessage(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "old", ReasoningContent: "stale", ReasoningSignature: "sig", ToolCalls: []provider.ToolCall{{ID: "call", Name: "noop"}}, ToolCallID: "tool", Name: "name"},
		{Role: provider.RoleUser, Content: "hello"},
	}

	got := withFreshSystemPrompt(msgs, "new")
	if got[0].Content != "new" {
		t.Fatalf("system prompt = %q, want new", got[0].Content)
	}
	if got[0].ReasoningContent != "" || got[0].ReasoningSignature != "" || len(got[0].ToolCalls) != 0 || got[0].ToolCallID != "" || got[0].Name != "" {
		t.Fatalf("system metadata should be cleared, got %+v", got[0])
	}
	if got[1].Content != "hello" {
		t.Fatalf("non-system message changed: %+v", got[1])
	}
	if msgs[0].Content != "old" {
		t.Fatalf("input slice was mutated: %+v", msgs[0])
	}
}

func TestWithFreshSystemPromptPrependsMissingSystemMessage(t *testing.T) {
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hello"}}

	got := withFreshSystemPrompt(msgs, "new")
	if len(got) != 2 || got[0].Role != provider.RoleSystem || got[0].Content != "new" {
		t.Fatalf("expected prepended system prompt, got %+v", got)
	}
	if got[1].Content != "hello" {
		t.Fatalf("existing user message changed: %+v", got[1])
	}
}

func TestWithFreshSystemPromptPreservesLegacyUserOnlySession(t *testing.T) {
	sess := agent.NewSession("")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "legacy"})

	got := sessionWithFreshSystemPrompt(sess, "new")
	history := got.Snapshot()
	if len(history) != 1 || history[0].Role != provider.RoleUser || history[0].Content != "legacy" {
		t.Fatalf("legacy user-only session should not gain a system prompt: %+v", history)
	}
}

func TestProviderViewFromEntry_PreservesNonChatModelsInSettings(t *testing.T) {
	p := config.ProviderEntry{
		Name: "mimo-api",
		Models: []string{
			"mimo-v2", "mimo-v2-pro",
			"mimo-v2-asr", "mimo-v2-tts",
			"mimo-v2-tts-voiceclone", "mimo-v2-tts-voicedesign",
			"gpt-image-2",
		},
		VisionModels: []string{"mimo-v2", "mimo-v2-asr", "mimo-v2-omni"},
	}
	view := providerViewFromEntry(p, true, false)
	want := []string{
		"mimo-v2", "mimo-v2-pro",
		"mimo-v2-asr", "mimo-v2-tts",
		"mimo-v2-tts-voiceclone", "mimo-v2-tts-voicedesign",
		"gpt-image-2",
	}
	if !reflect.DeepEqual(view.Models, want) {
		t.Errorf("ProviderView.Models = %v, want %v", view.Models, want)
	}
	if got, want := view.VisionModels, []string{"mimo-v2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ProviderView.VisionModels = %v, want %v", got, want)
	}
	if !view.VisionModelsSet {
		t.Fatal("ProviderView.VisionModelsSet = false, want true for configured vision_models")
	}
}

func TestProviderViewFromEntry_MigratesProviderWideVision(t *testing.T) {
	p := config.ProviderEntry{
		Name:   "custom",
		Models: []string{"text-only", "qwen-vl-plus"},
		Vision: true,
	}
	view := providerViewFromEntry(p, false, true)
	if got, want := view.VisionModels, []string{"text-only", "qwen-vl-plus"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ProviderView.VisionModels = %v, want %v", got, want)
	}
	if !view.VisionModelsSet {
		t.Fatal("ProviderView.VisionModelsSet = false, want true for provider-wide vision")
	}
}

func TestProviderViewFromEntryShowsKeySource(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("TEST_PROVIDER_KEY_SOURCE", "")
	os.Unsetenv("TEST_PROVIDER_KEY_SOURCE")
	if _, err := config.SetCredential("TEST_PROVIDER_KEY_SOURCE", "sk-test"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}

	view := providerViewFromEntry(config.ProviderEntry{
		Name:      "custom",
		APIKeyEnv: "TEST_PROVIDER_KEY_SOURCE",
	}, false, true)
	if !view.KeySet {
		t.Fatal("KeySet = false, want true")
	}
	if !view.Configured {
		t.Fatal("Configured = false, want true from resolved credentials")
	}
	if view.KeySource == "" || !strings.Contains(view.KeySource, "credentials") {
		t.Fatalf("KeySource = %q, want credentials source", view.KeySource)
	}
}

func TestProviderViewFromEntryExposesNoAuthAvailability(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("LOCAL_API_KEY", "")
	os.Unsetenv("LOCAL_API_KEY")

	noAuth := providerViewFromEntry(config.ProviderEntry{
		Name:    "local",
		Kind:    "openai",
		BaseURL: "http://127.0.0.1:23333/v1",
		Models:  []string{"model-a"},
	}, false, true)
	if noAuth.RequiresKey {
		t.Fatal("no-auth provider RequiresKey = true, want false")
	}
	if !noAuth.Configured {
		t.Fatal("no-auth provider Configured = false, want true")
	}
	if noAuth.KeySet {
		t.Fatal("no-auth provider KeySet = true, want false")
	}

	legacyLoopback := providerViewFromEntry(config.ProviderEntry{
		Name:      "local",
		Kind:      "openai",
		BaseURL:   "http://127.0.0.1:23333/v1",
		Models:    []string{"model-a"},
		APIKeyEnv: "LOCAL_API_KEY",
	}, false, true)
	if legacyLoopback.RequiresKey {
		t.Fatal("loopback provider with missing legacy key env RequiresKey = true, want false")
	}
	if !legacyLoopback.Configured {
		t.Fatal("loopback provider with missing legacy key env Configured = false, want true")
	}

	official := providerViewFromEntry(config.ProviderEntry{
		Name:    "deepseek",
		Kind:    "openai",
		BaseURL: "https://api.deepseek.com",
		Models:  []string{"deepseek-v4-flash"},
	}, true, true)
	if !official.RequiresKey {
		t.Fatal("official provider RequiresKey = false, want true")
	}
	if official.Configured {
		t.Fatal("official provider without key Configured = true, want false")
	}
}

func TestSetProviderKeyDoesNotWarnWhenProjectEnvAlsoDefinesSavedKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("TEST_PROVIDER_SHADOW=old-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_SHADOW", "")
	os.Unsetenv("TEST_PROVIDER_SHADOW")

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"project": {ID: "project", WorkspaceRoot: project}},
		activeTabID: "project",
	}
	warning, err := app.SetProviderKey("TEST_PROVIDER_SHADOW", "new-key")
	if err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	if warning != "" {
		t.Fatalf("SetProviderKey warning = %q, want no warning because provider keys use global credentials only", warning)
	}
	data, readErr := os.ReadFile(config.UserCredentialsPath())
	if readErr != nil {
		t.Fatalf("read credentials: %v", readErr)
	}
	if !strings.Contains(string(data), "TEST_PROVIDER_SHADOW=new-key") {
		t.Fatalf("saved credentials missing new key:\n%s", data)
	}
}

func TestSetProviderKeyDoesNotWarnWhenEnvironmentAlsoDefinesSavedKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("TEST_PROVIDER_EMPTY_ENV", "")

	app := &App{}
	warning, err := app.SetProviderKey("TEST_PROVIDER_EMPTY_ENV", "new-key")
	if err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	if warning != "" {
		t.Fatalf("SetProviderKey warning = %q, want no warning because provider keys use global credentials only", warning)
	}
	data, readErr := os.ReadFile(config.UserCredentialsPath())
	if readErr != nil {
		t.Fatalf("read credentials: %v", readErr)
	}
	if !strings.Contains(string(data), "TEST_PROVIDER_EMPTY_ENV=new-key") {
		t.Fatalf("saved credentials missing new key:\n%s", data)
	}
}

func TestSetProviderKeyDoesNotWarnWhenEmptyProjectEnvAlsoDefinesSavedKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("TEST_PROVIDER_EMPTY_PROJECT=\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_PROVIDER_EMPTY_PROJECT", "")
	os.Unsetenv("TEST_PROVIDER_EMPTY_PROJECT")

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"project": {ID: "project", WorkspaceRoot: project}},
		activeTabID: "project",
	}
	warning, err := app.SetProviderKey("TEST_PROVIDER_EMPTY_PROJECT", "new-key")
	if err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	if warning != "" {
		t.Fatalf("SetProviderKey warning = %q, want no warning because provider keys use global credentials only", warning)
	}
}

func TestSetProviderKeyWritesActiveProfileCredentials(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	key := "TEST_PROVIDER_PROFILE_SCOPED"
	t.Setenv(key, "")
	os.Unsetenv(key)

	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"
	pathsA := config.NewProfilePathResolver(actorA)
	pathsB := config.NewProfilePathResolver(actorB)
	for _, paths := range []config.ProfilePathResolver{pathsA, pathsB} {
		cfg := config.Default()
		cfg.Providers = []config.ProviderEntry{{
			Name:      "custom",
			Kind:      "openai",
			BaseURL:   "https://example.invalid/v1",
			Models:    []string{"model-a"},
			APIKeyEnv: key,
		}}
		if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
			t.Fatalf("save profile config: %v", err)
		}
	}

	appA := &App{
		tabRuntimes: map[string]*WorkspaceTab{"a": {
			ID:            "a",
			WorkspaceRoot: project,
			actor:         actorA,
		}},
		activeTabID: "a",
	}
	if warning, err := appA.SetProviderKey(key, "profile-a-key"); err != nil {
		t.Fatalf("SetProviderKey profile A: %v", err)
	} else if warning != "" {
		t.Fatalf("SetProviderKey warning = %q, want empty", warning)
	}
	if !config.CredentialStoredWithPaths(pathsA, key) {
		t.Fatal("profile A credential was not stored")
	}
	if config.CredentialStoredWithPaths(pathsB, key) {
		t.Fatal("profile B should not see profile A credential")
	}

	appB := &App{
		tabRuntimes: map[string]*WorkspaceTab{"b": {
			ID:            "b",
			WorkspaceRoot: project,
			actor:         actorB,
		}},
		activeTabID: "b",
	}
	settingsB := appB.Settings()
	if len(settingsB.Providers) != 1 {
		t.Fatalf("profile B providers = %+v, want one configured provider", settingsB.Providers)
	}
	if settingsB.Providers[0].KeySet {
		t.Fatalf("profile B provider keySet = true, want false for profile A key")
	}
}

func TestSettingsWritesCurrentProfileWithoutActiveTab(t *testing.T) {
	isolateDesktopUserDirs(t)

	actorA := userctx.ActorContext{ActorID: "desktop-a", ProfileID: "profile-a", AuthMethod: "local"}
	actorB := userctx.ActorContext{ActorID: "desktop-b", ProfileID: "profile-b", AuthMethod: "local"}
	pathsA := config.NewProfilePathResolver(actorA)
	pathsB := config.NewProfilePathResolver(actorB)

	app := NewApp()
	app.setCurrentActorContext(actorA)
	if err := app.SetCloseBehavior("quit"); err != nil {
		t.Fatalf("SetCloseBehavior: %v", err)
	}

	cfgA := config.LoadForEditWithoutCredentialsWithPaths(pathsA)
	cfgB := config.LoadForEditWithoutCredentialsWithPaths(pathsB)
	if got := cfgA.DesktopCloseBehavior(); got != "quit" {
		t.Fatalf("profile A close behavior = %q, want quit", got)
	}
	if got := cfgB.DesktopCloseBehavior(); got == "quit" {
		t.Fatalf("profile B close behavior = %q, want untouched", got)
	}
}

func TestFetchProviderModelsPreservesOrderAndNonChatModelIDs(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	_, actor, _ := app.desktopActiveRouteSnapshot()
	if _, err := config.SetCredentialWithPaths(config.NewProfilePathResolver(actor), "TEST_PROVIDER_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]string{
				{"id": "z-model", "object": "model"},
				{"id": "gpt-image-1", "object": "model"},
				{"id": "a-model", "object": "model"},
			},
		})
	}))
	defer srv.Close()

	got, err := app.FetchProviderModels(ProviderView{
		Name:      "mimo-api",
		BaseURL:   srv.URL,
		APIKeyEnv: "TEST_PROVIDER_KEY",
	})
	if err != nil {
		t.Fatalf("FetchProviderModels: %v", err)
	}
	want := []string{"z-model", "gpt-image-1", "a-model"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FetchProviderModels = %v, want %v", got, want)
	}
}

func TestFetchProviderModelsIgnoresActiveSubscriptionRoute(t *testing.T) {
	isolateDesktopUserDirs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []map[string]string{{"id": "local-model", "object": "model"}},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"official": {
			ID:            "official",
			WorkspaceRoot: ".",
			billingSource: "subscription_pool",
		},
	}
	app.activeTabID = "official"
	_, actor, _ := app.desktopActiveRouteSnapshot()
	if _, err := config.SetCredentialWithPaths(config.NewProfilePathResolver(actor), "TEST_PROVIDER_KEY", "test-key"); err != nil {
		t.Fatalf("SetCredentialWithPaths: %v", err)
	}

	got, err := app.FetchProviderModels(ProviderView{
		Name:      "local-provider",
		BaseURL:   srv.URL,
		APIKeyEnv: "TEST_PROVIDER_KEY",
	})
	if err != nil {
		t.Fatalf("FetchProviderModels: %v", err)
	}
	if want := []string{"local-model"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FetchProviderModels = %v, want %v", got, want)
	}
}

func TestConfigOnlyProviderSetupIgnoresActiveSubscriptionAuthentication(t *testing.T) {
	isolateDesktopUserDirs(t)
	const keyEnv = "TEST_ANONYMOUS_BYOK_KEY"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data":   []map[string]string{{"id": "local-model", "object": "model"}},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.ctx = context.Background()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"official": {
			ID:            "official",
			WorkspaceRoot: ".",
			model:         "kwu/gpt-5.4",
			billingSource: "subscription_pool",
		},
	}
	app.activeTabID = "official"

	if warning, err := app.SetProviderKeyConfigOnly(keyEnv, "test-key"); err != nil {
		t.Fatalf("SetProviderKeyConfigOnly: %v", err)
	} else if warning != "" {
		t.Fatalf("SetProviderKeyConfigOnly warning = %q, want empty", warning)
	}
	providerView := ProviderView{
		Name:      "local-provider",
		Kind:      "openai",
		BaseURL:   srv.URL,
		APIKeyEnv: keyEnv,
		Models:    []string{"local-model"},
		Default:   "local-model",
	}
	got, err := app.FetchProviderModels(providerView)
	if err != nil {
		t.Fatalf("FetchProviderModels after config-only credential save: %v", err)
	}
	if want := []string{"local-model"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FetchProviderModels = %v, want %v", got, want)
	}
	if err := app.SaveProviderConfigOnly(providerView); err != nil {
		t.Fatalf("SaveProviderConfigOnly: %v", err)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	saved, ok := cfg.Provider("local-provider")
	if !ok {
		t.Fatal("config-only provider was not persisted")
	}
	if got, want := saved.ModelList(), []string{"local-model"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("saved provider models = %v, want %v", got, want)
	}
	if app.tabRuntimes["official"].Ctrl != nil {
		t.Fatal("config-only BYOK setup rebuilt the active subscription controller")
	}
}

func TestFetchProviderModelsUsesSavedCredentialBeforeEnvironment(t *testing.T) {
	isolateDesktopUserDirs(t)
	const keyEnv = "TEST_PROVIDER_FETCH_KEY"
	if _, err := config.SetCredential(keyEnv, "saved-key"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	t.Setenv(keyEnv, "stale-env-key")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer saved-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]string{
				{"id": "model-a", "object": "model"},
			},
		})
	}))
	defer srv.Close()

	got, err := NewApp().FetchProviderModels(ProviderView{
		Name:      "custom",
		BaseURL:   srv.URL,
		APIKeyEnv: keyEnv,
	})
	if err != nil {
		t.Fatalf("FetchProviderModels: %v", err)
	}
	if want := []string{"model-a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FetchProviderModels = %v, want %v", got, want)
	}
	if got := os.Getenv(keyEnv); got != "stale-env-key" {
		t.Fatalf("process env = %q, want stale env left untouched", got)
	}
}

func TestSaveProviderPreservesOrderAndNonChatModelIDs(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if err := app.SaveProvider(ProviderView{
		Name:    "mimo-api",
		Kind:    "openai",
		BaseURL: "https://api.xiaomimimo.com/v1",
		Models:  []string{"z-model", "gpt-image-1", "a-model"},
		VisionModels: []string{
			"gpt-image-1",
		},
		VisionModelsSet: true,
		Default:         "gpt-image-1",
		APIKeyEnv:       "MIMO_API_KEY",
	}); err != nil {
		t.Fatalf("SaveProvider: %v", err)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	got, ok := cfg.Provider("mimo-api")
	if !ok {
		t.Fatal("saved provider not found")
	}
	want := []string{"z-model", "gpt-image-1", "a-model"}
	if !reflect.DeepEqual(got.ModelList(), want) {
		t.Errorf("saved provider models = %v, want %v", got.ModelList(), want)
	}
	if got.DefaultModel() != "gpt-image-1" {
		t.Errorf("saved provider default = %q, want gpt-image-1", got.DefaultModel())
	}
	if got, want := got.VisionModels, []string{"gpt-image-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("saved provider vision_models = %v, want %v", got, want)
	}
	raw, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved := string(raw)
	blockStart := strings.Index(saved, "\n[[providers]]\nname        = \"mimo-api\"")
	if blockStart < 0 {
		t.Fatalf("saved config missing mimo-api provider block:\n%s", raw)
	}
	block := saved[blockStart:]
	if next := strings.Index(block[len("\n[[providers]]"):], "\n[[providers]]"); next >= 0 {
		block = block[:len("\n[[providers]]")+next]
	}
	if !strings.Contains(block, `models      = ["z-model", "gpt-image-1", "a-model"]`) {
		t.Fatalf("saved provider block did not persist provider-order models array:\n%s", block)
	}
	if strings.Contains(block, `model       = "z-model"`) {
		t.Fatalf("saved provider block should not persist explicit selection as legacy model:\n%s", block)
	}
	if !strings.Contains(block, `vision_models = ["gpt-image-1"]`) {
		t.Fatalf("saved provider block did not persist selected vision_models:\n%s", block)
	}
}

func TestSaveProviderClearsProviderWideVisionForPerModelSelection(t *testing.T) {
	isolateDesktopUserDirs(t)

	cfg := config.LoadForEdit(config.UserConfigPath())
	cfg.Providers = []config.ProviderEntry{{
		Name:    "custom",
		Kind:    "openai",
		BaseURL: "https://proxy.example.com/v1",
		Models:  []string{"text-only", "qwen-vl-plus"},
		Default: "text-only",
		Vision:  true,
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	if err := NewApp().SaveProvider(ProviderView{
		Name:            "custom",
		Kind:            "openai",
		BaseURL:         "https://proxy.example.com/v1",
		Models:          []string{"text-only", "qwen-vl-plus"},
		VisionModels:    []string{"qwen-vl-plus"},
		VisionModelsSet: true,
		Default:         "text-only",
	}); err != nil {
		t.Fatalf("SaveProvider: %v", err)
	}

	gotCfg := config.LoadForEdit(config.UserConfigPath())
	got, ok := gotCfg.Provider("custom")
	if !ok {
		t.Fatal("saved provider not found")
	}
	if got.Vision {
		t.Fatal("saved provider kept provider-wide vision=true")
	}
	if got, want := got.VisionModels, []string{"qwen-vl-plus"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("saved provider vision_models = %v, want %v", got, want)
	}
	textOnly := *got
	textOnly.Model = "text-only"
	if config.EffectiveVision(&textOnly) {
		t.Fatal("unchecked text-only model should not inherit image input")
	}
	vision := *got
	vision.Model = "qwen-vl-plus"
	if !config.EffectiveVision(&vision) {
		t.Fatal("checked vision model should keep image input")
	}
}

func TestSaveProviderPreservesExplicitEmptyVisionModels(t *testing.T) {
	isolateDesktopUserDirs(t)

	if err := NewApp().SaveProvider(ProviderView{
		Name:            "custom",
		Kind:            "openai",
		BaseURL:         "https://proxy.example.com/v1",
		Models:          []string{"text-only", "qwen-vl-plus"},
		VisionModels:    []string{},
		VisionModelsSet: true,
		Default:         "text-only",
	}); err != nil {
		t.Fatalf("SaveProvider: %v", err)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	got, ok := cfg.Provider("custom")
	if !ok {
		t.Fatal("saved provider not found")
	}
	if got.VisionModels == nil || len(got.VisionModels) != 0 {
		t.Fatalf("saved provider vision_models = %#v, want explicit empty list", got.VisionModels)
	}
	raw, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(raw), `vision_models = []`) {
		t.Fatalf("saved config did not persist explicit empty vision_models:\n%s", raw)
	}
}

func TestDesktopSettingSaveRefusesMalformedConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	original := []byte("default_model = [\n")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), original, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := NewApp().SetDesktopTelemetry(false); err == nil {
		t.Fatal("SetDesktopTelemetry should reject malformed config")
	}
	got, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("malformed config was rewritten:\n%s", got)
	}
}

func TestOfficialMimoAPITemplateRemoved(t *testing.T) {
	if entries, keyEnv, err := officialProviderTemplate("mimo-api", "en"); err == nil {
		t.Fatalf("officialProviderTemplate(mimo-api) = entries=%v key=%q nil error, want unknown template", entries, keyEnv)
	}
}

func TestOfficialKWUTemplateDefaults(t *testing.T) {
	entries, keyEnv, err := officialProviderTemplate("kwu", "")
	if err != nil {
		t.Fatalf("officialProviderTemplate: %v", err)
	}
	if keyEnv != "KWU_API_KEY" || len(entries) != 1 {
		t.Fatalf("template = %v/%q, want one KWU_API_KEY entry", entries, keyEnv)
	}
	got := entries[0]
	if got.Name != "kwu" || got.Kind != "openai" {
		t.Fatalf("provider = %+v, want kwu/openai", got)
	}
	if got.BaseURL != config.OfficialProviderBaseURL {
		t.Fatalf("BaseURL = %q", got.BaseURL)
	}
	if !reflect.DeepEqual(got.Models, []string{"gpt-5.4", "gpt-5.4-mini"}) {
		t.Fatalf("Models = %#v, want gpt-5.4/gpt-5.4-mini", got.Models)
	}
	if got.Default != "gpt-5.4" {
		t.Fatalf("Default = %q, want gpt-5.4", got.Default)
	}
	if got.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("APIKeyEnv = %q, want KWU_API_KEY", got.APIKeyEnv)
	}
	if got.ContextWindow != 1_000_000 {
		t.Fatalf("ContextWindow = %d, want 1000000", got.ContextWindow)
	}
	if got.ReasoningProtocol != "openai" {
		t.Fatalf("ReasoningProtocol = %q, want openai", got.ReasoningProtocol)
	}
	if !reflect.DeepEqual(got.SupportedEfforts, []string{"low", "medium", "high"}) {
		t.Fatalf("SupportedEfforts = %#v, want low/medium/high", got.SupportedEfforts)
	}
	if got.BalanceURL != "" {
		t.Fatalf("BalanceURL = %q, want empty", got.BalanceURL)
	}
	view := providerViewFromEntry(got, true, false)
	if !view.RequiresKey || view.Configured {
		t.Fatalf("ProviderView RequiresKey/Configured = %v/%v, want true/false without KWU_API_KEY", view.RequiresKey, view.Configured)
	}
}

func TestSettingsOfficialProvidersOnlyExposeKWU(t *testing.T) {
	isolateDesktopUserDirs(t)

	got := NewApp().Settings()
	assertOnlyKWUOfficialProviders(t, got.OfficialProviders)

	root := t.TempDir()
	views := officialProviderViewsForRootWithResolver(map[string]bool{}, "", root, config.NewCredentialResolverForRoot(root))
	assertOnlyKWUOfficialProviders(t, views)
}

func TestSettingsDetailsRedactOfficialProviderAndKeepSameNameCustom(t *testing.T) {
	app := NewApp()
	custom := config.ProviderEntry{
		Name:          "kwu",
		Kind:          "openai",
		BaseURL:       "https://custom.example.invalid/v1",
		Models:        []string{"custom-model"},
		Default:       "custom-model",
		APIKeyEnv:     "CUSTOM_KWU_API_KEY",
		ContextWindow: 128_000,
	}
	official := config.OfficialProviderTemplate()
	cfg := config.Default()
	cfg.DefaultModel = "kwu/custom-model"
	cfg.Desktop.ProviderAccess = []string{"kwu"}
	cfg.Providers = []config.ProviderEntry{official, custom}

	details := app.settingsModelDetailsFromConfig(cfg, t.TempDir(), userctx.ActorContext{})
	if len(details.OfficialProviders) != 1 {
		t.Fatalf("official providers = %+v, want one built-in provider", details.OfficialProviders)
	}
	officialView := details.OfficialProviders[0]
	if !officialView.BuiltIn || !officialView.Locked || officialView.BillingSource != string(modelbilling.BillingSubscriptionPool) || officialView.BaseURL != "" {
		t.Fatalf("official view = %+v, want locked subscription view with redacted URL", officialView)
	}
	if len(details.Providers) != 1 {
		t.Fatalf("providers = %+v, want one same-name custom provider", details.Providers)
	}
	customView := details.Providers[0]
	if customView.BuiltIn || customView.Locked || customView.BillingSource != string(modelbilling.BillingNewAPIByok) || customView.BaseURL != custom.BaseURL {
		t.Fatalf("custom view = %+v, want editable BYOK provider with visible URL", customView)
	}
	if cfg.Providers[0].BaseURL != official.BaseURL {
		t.Fatalf("settings details mutated the in-memory official provider URL: %q", cfg.Providers[0].BaseURL)
	}
}

func TestSettingsDetailsPreserveDeepSeekBuiltInProviderView(t *testing.T) {
	app := NewApp()
	cfg := config.Default()
	cfg.DefaultModel = "deepseek-flash/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek-flash"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "deepseek-flash",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Models:    []string{"deepseek-v4-flash"},
		Default:   "deepseek-v4-flash",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}

	details := app.settingsModelDetailsFromConfig(cfg, t.TempDir(), userctx.ActorContext{})
	if len(details.Providers) != 1 {
		t.Fatalf("providers = %+v, want one configured DeepSeek provider", details.Providers)
	}
	view := details.Providers[0]
	if !view.BuiltIn || view.BillingSource != string(modelbilling.BillingNewAPIByok) || view.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("DeepSeek view = %+v, want built-in BYOK provider with visible endpoint", view)
	}
}

func assertOnlyKWUOfficialProviders(t *testing.T, providers []ProviderView) {
	t.Helper()
	if len(providers) != 1 {
		t.Fatalf("official providers = %+v, want exactly one KWU provider", providers)
	}
	if providers[0].Name != "kwu" {
		t.Fatalf("official provider = %+v, want kwu", providers[0])
	}
	for _, p := range providers {
		if strings.Contains(strings.ToLower(p.Name), "deepseek") {
			t.Fatalf("new official providers should not include DeepSeek: %+v", providers)
		}
		if strings.Contains(strings.ToLower(p.BaseURL), "deepseek") {
			t.Fatalf("new official providers should not point at DeepSeek: %+v", providers)
		}
		if strings.Contains(strings.ToLower(p.APIKeyEnv), "deepseek") {
			t.Fatalf("new official providers should not use DeepSeek key env: %+v", providers)
		}
		if strings.Contains(strings.ToLower(p.BalanceURL), "deepseek") {
			t.Fatalf("new official providers should not expose DeepSeek balance URL: %+v", providers)
		}
	}
}

func TestSetProviderKeyRestoresKWUOfficialProviderAccess(t *testing.T) {
	isolateDesktopUserDirs(t)

	cfg := config.Default()
	cfg.DefaultModel = "custom/model-a"
	cfg.Desktop.ProviderAccess = []string{}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "custom",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Models:    []string{"model-a"},
		Default:   "model-a",
		APIKeyEnv: "CUSTOM_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if _, err := NewApp().SetProviderKey("KWU_API_KEY", "test-credential"); err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}

	got := config.LoadForEdit(config.UserConfigPath())
	p, ok := got.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing after saving KWU_API_KEY")
	}
	if p.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("kwu APIKeyEnv = %q, want KWU_API_KEY", p.APIKeyEnv)
	}
	if !providerAccessSet(got.Desktop.ProviderAccess)["kwu"] {
		t.Fatalf("ProviderAccess = %#v, want kwu", got.Desktop.ProviderAccess)
	}
}

func TestSetProviderKeyToleratesDeprecatedOfficialDeepSeekKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("DEEPSEEK_API_KEY", "")
	os.Unsetenv("DEEPSEEK_API_KEY")

	cfg := config.Default()
	cfg.DefaultModel = "custom/model-a"
	cfg.Desktop.ProviderAccess = []string{}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "custom",
		Kind:      "openai",
		BaseURL:   "https://example.invalid/v1",
		Models:    []string{"model-a"},
		Default:   "model-a",
		APIKeyEnv: "CUSTOM_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if _, err := NewApp().SetProviderKey("DEEPSEEK_API_KEY", "legacy-key"); err != nil {
		t.Fatalf("SetProviderKey legacy DeepSeek key: %v", err)
	}

	data, err := os.ReadFile(config.UserCredentialsPath())
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if !strings.Contains(string(data), "DEEPSEEK_API_KEY=legacy-key") {
		t.Fatalf("saved credentials missing legacy key:\n%s", data)
	}
	got := config.LoadForEdit(config.UserConfigPath())
	access := providerAccessSet(got.Desktop.ProviderAccess)
	if access["deepseek"] || access["kwu"] {
		t.Fatalf("ProviderAccess = %#v, want no official provider restored for deprecated DeepSeek key", got.Desktop.ProviderAccess)
	}
}

func TestSetAgentParamsPersistsStepLimitsToUserConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if err := app.SetAgentParams(0.35, 37, 9, "custom system"); err != nil {
		t.Fatalf("SetAgentParams: %v", err)
	}

	view := app.Settings()
	if view.Agent.MaxSteps != 37 || view.Agent.PlannerMaxSteps != 9 {
		t.Fatalf("Settings().Agent = %+v, want maxSteps=37 plannerMaxSteps=9", view.Agent)
	}
	if view.Agent.Temperature != 0.35 || view.Agent.SystemPrompt != "custom system" {
		t.Fatalf("Settings().Agent did not preserve other agent params: %+v", view.Agent)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.Agent.MaxSteps != 37 || cfg.Agent.PlannerMaxSteps != 9 {
		t.Fatalf("saved config agent steps = max:%d planner:%d, want 37/9", cfg.Agent.MaxSteps, cfg.Agent.PlannerMaxSteps)
	}
	if cfg.Agent.Temperature != 0.35 || cfg.Agent.SystemPrompt != "custom system" {
		t.Fatalf("saved config did not preserve other agent params: %+v", cfg.Agent)
	}
}

func TestSetReasoningLanguagePersistsToUserConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if err := app.SetReasoningLanguage("zh"); err != nil {
		t.Fatalf("SetReasoningLanguage: %v", err)
	}

	view := app.Settings()
	if view.Agent.ReasoningLanguage != "zh" {
		t.Fatalf("Settings().Agent.ReasoningLanguage = %q, want zh", view.Agent.ReasoningLanguage)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.Agent.ReasoningLanguage != "zh" || cfg.ReasoningLanguage() != "zh" {
		t.Fatalf("saved reasoning language = %q/%q, want zh", cfg.Agent.ReasoningLanguage, cfg.ReasoningLanguage())
	}
}

func TestSetDesktopLanguagePersistsResponseLanguageAndUpdatesLiveTabs(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "kwu.toml"), []byte("language = \"zh\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	userCtrl := control.New(control.Options{})
	projectCtrl := control.New(control.Options{})
	app.tabRuntimes = map[string]*WorkspaceTab{
		"user": {
			ID:          "user",
			Scope:       "global",
			Ctrl:        userCtrl,
			Ready:       true,
			disabledMCP: map[string]ServerView{},
		},
		"project": {
			ID:            "project",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          projectCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "user"

	if err := app.SetDesktopLanguage("en"); err != nil {
		t.Fatalf("SetDesktopLanguage: %v", err)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.DesktopLanguage() != "en" || cfg.Language != "en" {
		t.Fatalf("saved language prefs = desktop:%q response:%q, want en/en", cfg.DesktopLanguage(), cfg.Language)
	}
	got := userCtrl.Compose("解释这个函数")
	if !strings.Contains(got, "<response-language>") || !strings.Contains(got, "use English") {
		t.Fatalf("live controller Compose = %q, want English response language", got)
	}
	projectComposed := projectCtrl.Compose("explain this function")
	if !strings.Contains(projectComposed, "use Simplified Chinese") {
		t.Fatalf("project controller Compose = %q, want project zh response language", projectComposed)
	}
}

func TestSetDesktopLanguageUpdatesLiveTabUsingControllerActorProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()

	defaultCfg := config.Default()
	if err := defaultCfg.SetLanguage("zh"); err != nil {
		t.Fatal(err)
	}
	if err := defaultCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.ActorContext{
		ActorID:       "desktop-work",
		ProfileID:     "work",
		AuthMethod:    "local",
		WorkspaceRoot: projectRoot,
	}
	workCtrl := control.New(control.Options{Actor: workActor})
	defer workCtrl.Close()

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"work": {
			ID:            "work",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          workCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "work"

	if err := app.SetDesktopLanguage("en"); err != nil {
		t.Fatalf("SetDesktopLanguage: %v", err)
	}

	got := workCtrl.Compose("解释这个函数")
	if !strings.Contains(got, "use English") {
		t.Fatalf("work profile controller Compose = %q, want English response language", got)
	}
}

func TestSetReasoningLanguageUpdatesLiveTabControllers(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "kwu.toml"), []byte("[agent]\nreasoning_language = \"en\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	userCtrl := control.New(control.Options{ReasoningLanguage: "auto"})
	projectCtrl := control.New(control.Options{ReasoningLanguage: "auto"})
	app.tabRuntimes = map[string]*WorkspaceTab{
		"user": {
			ID:          "user",
			Scope:       "global",
			Ctrl:        userCtrl,
			Ready:       true,
			disabledMCP: map[string]ServerView{},
		},
		"project": {
			ID:            "project",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          projectCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "user"

	if err := app.SetReasoningLanguage("zh"); err != nil {
		t.Fatalf("SetReasoningLanguage: %v", err)
	}

	userComposed := userCtrl.Compose("hi")
	if !strings.Contains(userComposed, "Simplified Chinese") {
		t.Fatalf("user-level tab Compose = %q, want zh reasoning language", userComposed)
	}
	projectComposed := projectCtrl.Compose("hi")
	if !strings.Contains(projectComposed, "use English") {
		t.Fatalf("project override tab Compose = %q, want en reasoning language", projectComposed)
	}
}

func TestSetReasoningLanguageUpdatesLiveTabUsingControllerActorProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()

	defaultCfg := config.Default()
	if err := defaultCfg.SetReasoningLanguage("zh"); err != nil {
		t.Fatal(err)
	}
	if err := defaultCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.ActorContext{
		ActorID:       "desktop-work",
		ProfileID:     "work",
		AuthMethod:    "local",
		WorkspaceRoot: projectRoot,
	}
	workCtrl := control.New(control.Options{Actor: workActor})
	defer workCtrl.Close()

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"work": {
			ID:            "work",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          workCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "work"

	if err := app.SetReasoningLanguage("en"); err != nil {
		t.Fatalf("SetReasoningLanguage: %v", err)
	}

	got := workCtrl.Compose("hi")
	if !strings.Contains(got, "use English") {
		t.Fatalf("work profile controller Compose = %q, want English reasoning language", got)
	}
}

func TestSetAutoPlanUpdatesLiveTabControllers(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	userRunner := &captureTurnRunner{}
	projectRunner := &captureTurnRunner{}
	userCtrl := control.New(control.Options{AutoPlan: "on", Runner: userRunner})
	projectCtrl := control.New(control.Options{AutoPlan: "on", Runner: projectRunner})
	app.tabRuntimes = map[string]*WorkspaceTab{
		"user": {
			ID:          "user",
			Scope:       "global",
			Ctrl:        userCtrl,
			Ready:       true,
			disabledMCP: map[string]ServerView{},
		},
		"project": {
			ID:            "project",
			Scope:         "project",
			WorkspaceRoot: t.TempDir(),
			Ctrl:          projectCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "user"

	if err := app.SetAutoPlan("off"); err != nil {
		t.Fatalf("SetAutoPlan: %v", err)
	}

	input := "实现 GitHub issue #2395：\n- 新增配置项\n- 自动判断复杂任务\n- 补测试和文档"
	if err := userCtrl.RunTurn(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := projectCtrl.RunTurn(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(userRunner.inputs) != 1 || strings.HasPrefix(userRunner.inputs[0], control.PlanModeMarker) {
		t.Fatalf("user tab should use updated auto_plan=off, inputs=%q", userRunner.inputs)
	}
	if len(projectRunner.inputs) != 1 || strings.HasPrefix(projectRunner.inputs[0], control.PlanModeMarker) {
		t.Fatalf("project tab without override should use updated auto_plan=off, inputs=%q", projectRunner.inputs)
	}
}

func TestSetAutoPlanIgnoresProjectOverrideForLiveTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "kwu.toml"), []byte("[agent]\nauto_plan = \"on\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	userRunner := &captureTurnRunner{}
	projectRunner := &captureTurnRunner{}
	userCtrl := control.New(control.Options{AutoPlan: "on", Runner: userRunner})
	projectCtrl := control.New(control.Options{AutoPlan: "on", Runner: projectRunner})
	app.tabRuntimes = map[string]*WorkspaceTab{
		"user": {
			ID:          "user",
			Scope:       "global",
			Ctrl:        userCtrl,
			Ready:       true,
			disabledMCP: map[string]ServerView{},
		},
		"project": {
			ID:            "project",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          projectCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "user"

	if err := app.SetAutoPlan("off"); err != nil {
		t.Fatalf("SetAutoPlan: %v", err)
	}

	input := "实现 GitHub issue #2395：\n- 新增配置项\n- 自动判断复杂任务\n- 补测试和文档"
	if err := userCtrl.RunTurn(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := projectCtrl.RunTurn(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(userRunner.inputs) != 1 || strings.HasPrefix(userRunner.inputs[0], control.PlanModeMarker) {
		t.Fatalf("user tab should use updated auto_plan=off, inputs=%q", userRunner.inputs)
	}
	if len(projectRunner.inputs) != 1 || strings.HasPrefix(projectRunner.inputs[0], control.PlanModeMarker) {
		t.Fatalf("project auto_plan should be ignored, inputs=%q", projectRunner.inputs)
	}
}

func TestSetAutoPlanUpdatesLiveTabUsingControllerActorProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()

	defaultActor := userctx.Default(projectRoot)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultCfg := config.Default()
	if err := defaultCfg.SetAutoPlan("on"); err != nil {
		t.Fatal(err)
	}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.Default(projectRoot)
	workActor.ProfileID = "work"
	workPaths := config.NewProfilePathResolver(workActor)
	workCfg := config.Default()
	if err := workCfg.SetAutoPlan("on"); err != nil {
		t.Fatal(err)
	}
	if err := workCfg.SaveToScope(workPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save work config: %v", err)
	}

	defaultRunner := &captureTurnRunner{}
	workRunner := &captureTurnRunner{}
	defaultCtrl := control.New(control.Options{Actor: defaultActor, WorkspaceRoot: projectRoot, AutoPlan: "on", Runner: defaultRunner})
	workCtrl := control.New(control.Options{Actor: workActor, WorkspaceRoot: projectRoot, AutoPlan: "on", Runner: workRunner})
	defer defaultCtrl.Close()
	defer workCtrl.Close()

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"default": {
			ID:            "default",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          defaultCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
		"work": {
			ID:            "work",
			Scope:         "project",
			WorkspaceRoot: projectRoot,
			Ctrl:          workCtrl,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "work"

	if err := app.SetAutoPlan("off"); err != nil {
		t.Fatalf("SetAutoPlan: %v", err)
	}

	input := "实现 GitHub issue #2395：\n- 新增配置项\n- 自动判断复杂任务\n- 补测试和文档"
	if err := defaultCtrl.RunTurn(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if err := workCtrl.RunTurn(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(defaultRunner.inputs) != 1 || !strings.HasPrefix(defaultRunner.inputs[0], control.PlanModeMarker) {
		t.Fatalf("default profile tab should keep auto_plan=on, inputs=%q", defaultRunner.inputs)
	}
	if len(workRunner.inputs) != 1 || strings.HasPrefix(workRunner.inputs[0], control.PlanModeMarker) {
		t.Fatalf("work profile tab should use updated auto_plan=off, inputs=%q", workRunner.inputs)
	}
}

func TestSetAutoPlanEnablingClassifierRebuildsActiveController(t *testing.T) {
	isolateDesktopUserDirs(t)

	cfg := config.LoadForEdit(config.UserConfigPath())
	cfg.Agent.AutoPlan = "off"
	cfg.Agent.AutoPlanClassifier = testKWUFlashModelRef
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{AutoPlan: "off", Label: "old-controller"})
	app.setTestCtrl(old, testKWUDefaultModelRef)
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetAutoPlan("on"); err != nil {
		t.Fatalf("SetAutoPlan(on): %v", err)
	}
	if c := app.activeCtrl(); c == nil {
		t.Fatal("SetAutoPlan should leave a rebuilt controller")
	}
	if c := app.activeCtrl(); c == old {
		t.Fatal("SetAutoPlan should rebuild when enabling a configured classifier")
	}

	got := config.LoadForEdit(config.UserConfigPath())
	if got.Agent.AutoPlan != "on" {
		t.Fatalf("saved auto_plan = %q, want on", got.Agent.AutoPlan)
	}
}

func TestSettingsRebuildPreservesActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "old-model"},
	}
	if err := cfg.SaveTo(filepath.Join(root, "kwu.toml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{
		Label: "old/old-model",
		Actor: userctx.ActorContext{
			ActorID:    "actor-settings",
			ProfileID:  "profile-settings",
			AuthMethod: "oauth",
		},
	})
	tab := &WorkspaceTab{ID: "tab_a", WorkspaceRoot: root, Ready: true, model: "old/old-model", Ctrl: old}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	defer tab.Ctrl.Close()

	actor := tab.Ctrl.ActorContext()
	if actor.ActorID != "actor-settings" || actor.ProfileID != "profile-settings" || actor.AuthMethod != "oauth" {
		t.Fatalf("rebuilt ActorContext() = %+v", actor)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, root)
	}
}

func TestSetReasoningLanguageRejectsBackgroundJobsBeforeSavingConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	app.setTestCtrl(newBackgroundJobController(t, "reasoning-language-job"), "")

	err := app.SetReasoningLanguage("zh")
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("SetReasoningLanguage with background job error = %v, want active-work guard", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.ReasoningLanguage() != "auto" {
		t.Fatalf("reasoning language changed after rejected update: %q", cfg.ReasoningLanguage())
	}
}

func TestSetDesktopCheckUpdatesPersistsToUserConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if !app.Settings().CheckUpdates {
		t.Fatal("Settings().CheckUpdates default = false, want true")
	}
	if err := app.SetDesktopCheckUpdates(false); err != nil {
		t.Fatalf("SetDesktopCheckUpdates: %v", err)
	}
	view := app.Settings()
	if view.CheckUpdates {
		t.Fatal("Settings().CheckUpdates = true, want false")
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.Desktop.CheckUpdates == nil || *cfg.Desktop.CheckUpdates {
		t.Fatalf("desktop.check_updates = %+v, want false", cfg.Desktop.CheckUpdates)
	}
	if cfg.DesktopCheckUpdates() {
		t.Fatal("DesktopCheckUpdates() = true, want false")
	}
}

func TestSetDesktopCheckUpdatesUsesActiveProfileResolver(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	actor := userctx.ActorContext{
		ActorID:    "actor-settings",
		ProfileID:  "profile-settings",
		AuthMethod: "oauth",
	}
	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"profile": {
			ID:            "profile",
			Scope:         "project",
			WorkspaceRoot: workspaceRoot,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
			actor:         actor,
		},
	}
	app.activeTabID = "profile"

	if err := app.SetDesktopCheckUpdates(false); err != nil {
		t.Fatalf("SetDesktopCheckUpdates: %v", err)
	}

	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("default profile config should not be written, stat err=%v", err)
	}
	actor.WorkspaceRoot = workspaceRoot
	profilePath := config.NewProfilePathResolver(actor).UserConfigPath()
	cfg := config.LoadForEdit(profilePath)
	if cfg.Desktop.CheckUpdates == nil || *cfg.Desktop.CheckUpdates {
		t.Fatalf("profile desktop.check_updates = %+v, want false", cfg.Desktop.CheckUpdates)
	}
}

func TestSettingsPermissionsUseActiveProfileConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	actorA := userctx.Default(workspaceRoot)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(workspaceRoot)
	actorB.ProfileID = "profile-b"
	pathsA := config.NewProfilePathResolver(actorA)
	pathsB := config.NewProfilePathResolver(actorB)

	cfgA := config.Default()
	cfgA.Permissions.Allow = []string{"Bash(profile-a*)"}
	if err := cfgA.SaveToScope(pathsA.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save profile A config: %v", err)
	}
	cfgB := config.Default()
	cfgB.Permissions.Allow = []string{"Bash(profile-b*)"}
	cfgB.Permissions.Ask = []string{"Edit(*)"}
	if err := cfgB.SaveToScope(pathsB.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save profile B config: %v", err)
	}

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"b": {
			ID:            "b",
			Scope:         "project",
			WorkspaceRoot: workspaceRoot,
			Ready:         true,
			disabledMCP:   map[string]ServerView{},
			actor:         actorB,
		},
	}
	app.activeTabID = "b"

	view := app.Settings()
	if view.ConfigPath != pathsB.UserConfigPath() {
		t.Fatalf("Settings().ConfigPath = %q, want profile B config %q", view.ConfigPath, pathsB.UserConfigPath())
	}
	if len(view.Permissions.Allow) != 1 || view.Permissions.Allow[0] != "Bash(profile-b*)" {
		t.Fatalf("Settings().Permissions.Allow = %v, want profile B rule", view.Permissions.Allow)
	}
	if len(view.Permissions.Ask) != 1 || view.Permissions.Ask[0] != "Edit(*)" {
		t.Fatalf("Settings().Permissions.Ask = %v, want profile B ask rule", view.Permissions.Ask)
	}
}

func TestSetDefaultToolApprovalModePersistsToDatabase(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if app.Settings().DefaultToolApprovalMode != control.ToolApprovalAsk {
		t.Fatalf("Settings().DefaultToolApprovalMode = %q, want ask", app.Settings().DefaultToolApprovalMode)
	}
	if err := app.SetDefaultToolApprovalMode(control.ToolApprovalAuto); err != nil {
		t.Fatalf("SetDefaultToolApprovalMode: %v", err)
	}
	view := app.Settings()
	if view.DefaultToolApprovalMode != control.ToolApprovalAuto {
		t.Fatalf("Settings().DefaultToolApprovalMode = %q, want auto", view.DefaultToolApprovalMode)
	}
	store := newDesktopTabStore(app.desktopConfigDir())
	if mode, ok, err := store.defaultToolApprovalMode(); err != nil || !ok || mode != control.ToolApprovalAuto {
		t.Fatalf("database default tool approval mode = %q, %v, %v", mode, ok, err)
	}
}

func TestDesktopReportingDefaultsOffAndCannotEnablePlaceholderEndpoint(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if app.Settings().Telemetry || app.Settings().Metrics {
		t.Fatalf("default desktop reporting = telemetry:%v metrics:%v, want both false", app.Settings().Telemetry, app.Settings().Metrics)
	}
	writePendingCrash("stale", "boom", []byte("stack"))
	metricsPath := filepath.Join(config.MemoryUserDir(), metricsPendingFile)
	writeCounters(metricsPath, counters{"turns": {"total": 1}})
	if err := app.SetDesktopTelemetry(true); err != nil {
		t.Fatalf("SetDesktopTelemetry: %v", err)
	}
	if err := app.SetDesktopMetrics(true); err != nil {
		t.Fatalf("SetDesktopMetrics: %v", err)
	}
	view := app.Settings()
	if view.Telemetry || view.Metrics {
		t.Fatalf("unavailable desktop reporting = telemetry:%v metrics:%v, want both false", view.Telemetry, view.Metrics)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.Desktop.Telemetry == nil || *cfg.Desktop.Telemetry {
		t.Fatalf("desktop.telemetry = %+v, want false", cfg.Desktop.Telemetry)
	}
	if cfg.Desktop.Metrics == nil || *cfg.Desktop.Metrics {
		t.Fatalf("desktop.metrics = %+v, want false", cfg.Desktop.Metrics)
	}
	if cfg.DesktopTelemetry() || cfg.DesktopMetrics() {
		t.Fatal("persisted desktop reporting should remain disabled")
	}
	if _, err := os.Stat(pendingCrashPath()); !os.IsNotExist(err) {
		t.Fatalf("disabled telemetry retained pending crash, stat err = %v", err)
	}
	if _, err := os.Stat(metricsPath); !os.IsNotExist(err) {
		t.Fatalf("disabled metrics retained pending data, stat err = %v", err)
	}
}
