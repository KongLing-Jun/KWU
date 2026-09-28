package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/automation"
	"kwu/internal/brand"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestAutomationModelsMarksOnlyTheTargetDefault(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	configureDesktopTestBYOKRoute(t)
	app := &App{}
	root := t.TempDir()
	tests := []struct {
		name, model, source string
		wantCurrent         bool
	}{
		{"available custom default", "custom/chat-model", "newapi_byok", true},
		{"legacy default alias", ":custom/chat-model", "newapi_byok", true},
		{"unavailable default keeps alternatives", "kwu/unavailable", "subscription_pool", false},
		{"same reference wrong source", "custom/chat-model", "subscription_pool", false},
		{"removed default", "custom/removed", "newapi_byok", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contents := "default_model = \"" + test.model + "\"\n[desktop]\ndefault_billing_source = \"" + test.source + "\"\n"
			path := filepath.Join(root, brand.ProjectConfigName)
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			models := app.AutomationModels(root)
			if len(models) != 1 || models[0].Ref != "custom/chat-model" || models[0].Current != test.wantCurrent {
				t.Fatalf("target catalog = %+v, want current %v", models, test.wantCurrent)
			}
			route, err := app.resolveAutomationTaskRoute(automation.Task{Scope: "project", WorkspaceRoot: root})
			if (err == nil) != test.wantCurrent {
				t.Fatalf("catalog/resolver disagree: %+v, %v", route, err)
			}
			if test.wantCurrent && (route.ModelRef != models[0].Ref || string(route.BillingSource) != models[0].BillingSource) {
				t.Fatalf("default pair differs: %+v, %+v", route, models)
			}
			global := app.AutomationModels("")
			if len(global) != 1 || !global[0].Current || models[0].Current != test.wantCurrent {
				t.Fatalf("target markers leaked: global %+v, target %+v", global, models)
			}
			stored, err := os.ReadFile(path)
			if err != nil || string(stored) != contents {
				t.Fatalf("catalog changed configuration: %v", err)
			}
		})
	}
}

func TestAutomationRouteRejectionHasSafeCode(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	configureDesktopTestBYOKRoute(t)
	app := &App{}
	_, err := app.resolveAutomationTaskRoute(automation.Task{
		Prompt: automation.PromptConfig{Model: "kwu/unavailable", BillingSource: "subscription_pool"},
	})
	if err == nil || !strings.HasPrefix(err.Error(), "AUTOMATION_MODEL_UNAVAILABLE:") {
		t.Fatalf("missing model error code: %v", err)
	}
}

func TestAutomationModelsKeepsBYOKAvailableWithoutCloudAccount(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	profileRoot := isolateDesktopUserDirs(t)
	t.Setenv(devProfileRootEnv, filepath.Join(profileRoot, "dev-profile"))
	// Profile initialization may rewrite directory roots. Restore isolation before writing fixtures.
	if _, err := ensureDesktopProfileInitialized(); err != nil {
		t.Fatalf("initialize desktop profile: %v", err)
	}
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "custom/chat-model"

[desktop]
provider_access = ["custom"]
default_billing_source = "newapi_byok"

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
models = ["chat-model", "image-generation-model"]
default = "chat-model"
api_key_env = "CUSTOM_CHAT_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	models := app.AutomationModels("")
	if len(models) != 1 || models[0].Ref != "custom/chat-model" || models[0].BillingSource != string(modelbilling.BillingNewAPIByok) {
		t.Fatalf("automation models = %+v", models)
	}
	route, err := app.resolveAutomationTaskRoute(automation.Task{
		Prompt: automation.PromptConfig{Model: "custom/chat-model", BillingSource: string(modelbilling.BillingNewAPIByok)},
	})
	if err != nil || route.ModelRef != "custom/chat-model" || route.BillingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("route = %+v, err = %v", route, err)
	}
	defaultRoute, err := app.resolveAutomationTaskRoute(automation.Task{})
	if err != nil || defaultRoute != route {
		t.Fatalf("default automation route = %+v, err = %v; want %+v", defaultRoute, err, route)
	}
	residualSourceRoute, err := app.resolveAutomationTaskRoute(automation.Task{
		Prompt: automation.PromptConfig{BillingSource: string(modelbilling.BillingSubscriptionPool)},
	})
	if err != nil || residualSourceRoute != route {
		t.Fatalf("default route with residual source = %+v, err = %v; want %+v", residualSourceRoute, err, route)
	}
	if _, err := app.resolveAutomationTaskRoute(automation.Task{
		Prompt: automation.PromptConfig{Model: "kwu/gpt-5.4", BillingSource: string(modelbilling.BillingSubscriptionPool)},
	}); err == nil {
		t.Fatal("signed-out official automation route was accepted")
	}
}

func TestChatSettingsAndAutomationShareTheSameRouteCatalog(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	t.Setenv("HIDDEN_CHAT_KEY", "")
	isolateDesktopUserDirs(t)
	app := NewApp()
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

	cfg := config.Default()
	cfg.Desktop.ProviderAccess = []string{"custom", "hidden"}
	cfg.Providers = append(cfg.Providers,
		config.ProviderEntry{
			Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
			Models:  []string{"chat-one", "chat-two", "image-generation-model"},
			Default: "chat-one", APIKeyEnv: "CUSTOM_CHAT_KEY",
		},
		config.ProviderEntry{
			Name: "hidden", Kind: "openai", BaseURL: "https://example.invalid/v1",
			Model: "hidden-chat", APIKeyEnv: "HIDDEN_CHAT_KEY",
		},
	)
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	actor := userctx.Default("")
	scope := cloud.BindingScope{
		ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1,
	}
	taskScope := modelbilling.TaskScope{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &scope,
	}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		authState: cloud.AccountAuthenticated, bindingStatus: cloud.BindingBound,
		authorized: true, authorizedActor: actor, authorizedScope: scope,
		bindingActor: actor, bindingScope: scope, hasBinding: true,
		active: &desktopBillingWorkerProcess{scope: taskScope, done: make(chan struct{})},
	}
	officialModels := []string{"claude-fable-5", "claude-opus-4-6"}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, officialModels)
	tab := &WorkspaceTab{
		ID: "aligned-routes", Ready: true, actor: actor,
		model: "custom/chat-one", billingSource: string(modelbilling.BillingNewAPIByok),
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	wantRoutes := map[string]bool{
		"subscription_pool\x00kwu/claude-fable-5":  true,
		"subscription_pool\x00kwu/claude-opus-4-6": true,
		"newapi_byok\x00custom/chat-one":           true,
		"newapi_byok\x00custom/chat-two":           true,
	}
	assertRoutes := func(label string, models []ModelInfo) {
		t.Helper()
		got := make(map[string]bool, len(models))
		for _, model := range models {
			got[model.BillingSource+"\x00"+model.Ref] = true
		}
		if len(models) != len(wantRoutes) {
			t.Fatalf("%s routes = %+v, want %d unique routes", label, models, len(wantRoutes))
		}
		for route := range wantRoutes {
			if !got[route] {
				t.Fatalf("%s routes = %+v, missing %q", label, models, route)
			}
		}
		if len(got) != len(wantRoutes) {
			t.Fatalf("%s routes = %+v, want exact set %+v", label, models, wantRoutes)
		}
	}
	assertRoutes("main chat", app.ModelsForTab(tab.ID))
	assertRoutes("automation", app.AutomationModels(""))

	settings := app.Settings()
	providerChatModels := map[string]map[string]bool{}
	providerModels := map[string]map[string]bool{}
	settingsRoutes := make([]ModelInfo, 0)
	for _, provider := range settingsProvidersIncludingOfficial(settings) {
		models := make(map[string]bool, len(provider.ChatModels))
		allModels := make(map[string]bool, len(provider.Models))
		for _, model := range provider.Models {
			allModels[model] = true
		}
		for _, model := range provider.ChatModels {
			models[model] = true
			if provider.Added && provider.Configured {
				settingsRoutes = append(settingsRoutes, ModelInfo{
					Ref: provider.Name + "/" + model, Provider: provider.Name, Model: model,
					BillingSource: provider.BillingSource,
				})
			}
		}
		providerChatModels[provider.Name] = models
		providerModels[provider.Name] = allModels
	}
	assertRoutes("settings", settingsRoutes)
	for _, model := range officialModels {
		if !providerChatModels["kwu"][model] {
			t.Fatalf("settings official chat models = %+v, missing %q", providerChatModels["kwu"], model)
		}
	}
	for _, model := range []string{"chat-one", "chat-two"} {
		if !providerChatModels["custom"][model] {
			t.Fatalf("settings custom chat models = %+v, missing %q", providerChatModels["custom"], model)
		}
	}
	if providerChatModels["custom"]["image-generation-model"] {
		t.Fatalf("settings chat picker exposed non-chat model: %+v", providerChatModels["custom"])
	}
	if !providerModels["custom"]["image-generation-model"] {
		t.Fatalf("settings lost the configured non-chat model: %+v", providerModels["custom"])
	}

	for route := range wantRoutes {
		separator := strings.IndexByte(route, 0)
		source, model := route[:separator], route[separator+1:]
		resolved, err := app.resolveAutomationTaskRoute(automation.Task{
			Prompt: automation.PromptConfig{Model: model, BillingSource: source},
		})
		if err != nil || resolved.ModelRef != model || string(resolved.BillingSource) != source {
			t.Fatalf("automation route %q = %+v, err = %v", route, resolved, err)
		}
	}
	if _, err := app.resolveAutomationTaskRoute(automation.Task{
		Prompt: automation.PromptConfig{
			Model: "kwu/not-in-catalog", BillingSource: string(modelbilling.BillingSubscriptionPool),
		},
	}); err == nil {
		t.Fatal("automation accepted an unavailable official model")
	}
}

func TestAutomationLegacyRouteUsesOnlyUniqueAvailableCandidate(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
[desktop]
provider_access = ["custom"]

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "chat-model"
api_key_env = "CUSTOM_CHAT_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	route, err := NewApp().resolveAutomationTaskRoute(automation.Task{
		Prompt: automation.PromptConfig{Model: "custom/chat-model"},
	})
	if err != nil || route.BillingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("legacy route = %+v, err = %v", route, err)
	}
}

func TestSettingsOfficialProviderUsesScopedMaaSCatalogAndCannotBeRemoved(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	app := NewApp()
	actor := userctx.Default("")
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	taskScope := modelbilling.TaskScope{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &scope,
	}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		authState: cloud.AccountAuthenticated, bindingStatus: cloud.BindingBound,
		authorized: true, authorizedActor: actor, authorizedScope: scope,
		bindingActor: actor, bindingScope: scope, hasBinding: true,
		active: &desktopBillingWorkerProcess{scope: taskScope, done: make(chan struct{})},
	}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"model-a", "model-b"})

	settings := app.Settings()
	providers := settingsProvidersIncludingOfficial(settings)
	var official *ProviderView
	for i := range providers {
		if providers[i].Name == "kwu" {
			official = &providers[i]
			break
		}
	}
	if official == nil || !official.Locked || !official.Added || !official.Configured ||
		official.APIKeyEnv != "" || official.BillingSource != string(modelbilling.BillingSubscriptionPool) ||
		len(official.Models) != 2 {
		t.Fatalf("official provider = %+v", official)
	}
	if err := app.RemoveProviderAccess("kwu"); err == nil {
		t.Fatal("official points channel was removable")
	}
}

func TestChatSettingsAndAutomationHideCachedSubscriptionCatalogAfterSessionExpires(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

	cfg := config.Default()
	cfg.DefaultModel = "custom/chat-model"
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingNewAPIByok)
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{
		Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
		Model: "chat-model", APIKeyEnv: "CUSTOM_CHAT_KEY",
	})
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	actor := userctx.Default("")
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	taskScope := modelbilling.TaskScope{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &scope,
	}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		authState: cloud.AccountExpired, bindingStatus: cloud.BindingBound,
		authorized: true, authorizedActor: actor, authorizedScope: scope,
		bindingActor: actor, bindingScope: scope, hasBinding: true,
		active: &desktopBillingWorkerProcess{scope: taskScope, done: make(chan struct{})},
	}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"official-live-model"})
	tab := &WorkspaceTab{
		ID: "expired-session", Ready: true, actor: actor,
		model: "custom/chat-model", billingSource: string(modelbilling.BillingNewAPIByok),
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	want := map[string]bool{"newapi_byok\x00custom/chat-model": true}
	assertExactRoutes := func(label string, models []ModelInfo) {
		t.Helper()
		got := make(map[string]bool, len(models))
		for _, model := range models {
			got[model.BillingSource+"\x00"+model.Ref] = true
		}
		if len(models) != len(want) || len(got) != len(want) || !got["newapi_byok\x00custom/chat-model"] {
			t.Fatalf("%s routes = %+v, want exact BYOK-only set", label, models)
		}
	}
	assertExactRoutes("main chat", app.ModelsForTab(tab.ID))
	assertExactRoutes("automation", app.AutomationModels(""))

	settingsRoutes := make([]ModelInfo, 0)
	for _, provider := range app.Settings().Providers {
		for _, model := range provider.ChatModels {
			if provider.Added && provider.Configured {
				settingsRoutes = append(settingsRoutes, ModelInfo{
					Ref: provider.Name + "/" + model, Provider: provider.Name, Model: model,
					BillingSource: provider.BillingSource,
				})
			}
		}
	}
	assertExactRoutes("settings", settingsRoutes)
	if got := app.subscriptionModels.current(scope); len(got) != 1 || got[0] != "official-live-model" {
		t.Fatalf("expired-session test did not retain the cached catalog: %v", got)
	}
}

func TestDefaultRouteSwitchClearsDerivedModelsFromAnotherBillingSource(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

	cfg := config.Default()
	cfg.DefaultModel = "custom/chat-model"
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingNewAPIByok)
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Agent.PlannerModel = "custom/chat-model"
	cfg.Agent.SubagentModel = "custom/chat-model"
	cfg.Agent.SubagentModels = map[string]string{"review": "custom/chat-model"}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{
		Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
		Models: []string{"chat-model", "image-generation-model"}, Default: "chat-model", APIKeyEnv: "CUSTOM_CHAT_KEY",
	})
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := &WorkspaceTab{
		ID: "route", Ready: true, model: "custom/chat-model",
		billingSource: string(modelbilling.BillingNewAPIByok),
		actor:         userctx.Default(""),
	}
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	if err := app.SetDefaultModelRoute("kwu/gpt-5.4", string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}

	saved := config.LoadForEditWithoutCredentials(config.UserConfigPath())
	if saved.DefaultModel != "kwu/gpt-5.4" ||
		saved.Desktop.DefaultBillingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("saved default route = %q/%q", saved.Desktop.DefaultBillingSource, saved.DefaultModel)
	}
	if saved.Agent.PlannerModel != "" || saved.Agent.SubagentModel != "" || len(saved.Agent.SubagentModels) != 0 {
		t.Fatalf("cross-source derived models were retained: %+v", saved.Agent)
	}
}

func TestMainChatDefaultRoutePersistenceUsesTheSameDerivedModelPolicy(t *testing.T) {
	t.Run("cross source clears derived models", func(t *testing.T) {
		t.Setenv("KWU_CLOUD_FAKE", "")
		isolateDesktopUserDirs(t)
		setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

		cfg := config.Default()
		cfg.DefaultModel = "custom/chat-one"
		cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingNewAPIByok)
		cfg.Desktop.ProviderAccess = []string{"custom"}
		cfg.Agent.PlannerModel = "custom/chat-one"
		cfg.Agent.SubagentModel = "custom/chat-one"
		cfg.Agent.SubagentModels = map[string]string{"review": "custom/chat-one"}
		cfg.Providers = append(cfg.Providers, config.ProviderEntry{
			Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
			Models: []string{"chat-one", "chat-two"}, Default: "chat-one", APIKeyEnv: "CUSTOM_CHAT_KEY",
		})
		if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
			t.Fatal(err)
		}

		app := NewApp()
		actor := userctx.Default("")
		scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
		app.billingWorkerRuntime = &desktopBillingWorkerManager{
			bindingActor: actor, bindingScope: scope, hasBinding: true,
		}
		app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
			Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
		}, []string{"claude-fable-5"})

		if err := app.persistUserDefaultModelSelection(
			"kwu/claude-fable-5", modelbilling.BillingSubscriptionPool, actor,
		); err != nil {
			t.Fatal(err)
		}
		saved := config.LoadForEditWithoutCredentials(config.UserConfigPath())
		if saved.DefaultModel != "kwu/claude-fable-5" ||
			saved.Desktop.DefaultBillingSource != string(modelbilling.BillingSubscriptionPool) {
			t.Fatalf("saved route = %q/%q", saved.Desktop.DefaultBillingSource, saved.DefaultModel)
		}
		if saved.Agent.PlannerModel != "" || saved.Agent.SubagentModel != "" || len(saved.Agent.SubagentModels) != 0 {
			t.Fatalf("cross-source derived models were retained: %+v", saved.Agent)
		}
	})

	t.Run("same source keeps derived models", func(t *testing.T) {
		t.Setenv("KWU_CLOUD_FAKE", "")
		isolateDesktopUserDirs(t)
		setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

		cfg := config.Default()
		cfg.DefaultModel = "custom/chat-one"
		cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingNewAPIByok)
		cfg.Desktop.ProviderAccess = []string{"custom"}
		cfg.Agent.PlannerModel = "custom/chat-one"
		cfg.Agent.SubagentModel = "custom/chat-one"
		cfg.Agent.SubagentModels = map[string]string{"review": "custom/chat-one"}
		cfg.Providers = append(cfg.Providers, config.ProviderEntry{
			Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
			Models: []string{"chat-one", "chat-two"}, Default: "chat-one", APIKeyEnv: "CUSTOM_CHAT_KEY",
		})
		if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
			t.Fatal(err)
		}

		app := NewApp()
		if err := app.persistUserDefaultModelSelection(
			"custom/chat-two", modelbilling.BillingNewAPIByok, userctx.Default(""),
		); err != nil {
			t.Fatal(err)
		}
		saved := config.LoadForEditWithoutCredentials(config.UserConfigPath())
		if saved.Agent.PlannerModel != "custom/chat-one" ||
			saved.Agent.SubagentModel != "custom/chat-one" ||
			saved.Agent.SubagentModels["review"] != "custom/chat-one" {
			t.Fatalf("same-source derived models were cleared: %+v", saved.Agent)
		}
	})
}

func TestSubscriptionSettingsKeepsCustomProviderCredentialPresence(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "CUSTOM_CHAT_KEY", "sk-custom")

	cfg := config.Default()
	cfg.DefaultModel = "kwu/gpt-5.4"
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{
		Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
		Model: "chat-model", APIKeyEnv: "CUSTOM_CHAT_KEY",
	})
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := &WorkspaceTab{
		ID: "subscription", Ready: true, model: "kwu/gpt-5.4",
		billingSource: string(modelbilling.BillingSubscriptionPool),
		actor:         userctx.Default(""),
	}
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID

	settings := app.Settings()
	for _, item := range settings.Providers {
		if item.Name == "custom" {
			if !item.KeySet || !item.Configured || item.BillingSource != string(modelbilling.BillingNewAPIByok) {
				t.Fatalf("custom provider = %+v, want stored BYOK availability without secret loading", item)
			}
			if len(item.ChatModels) != 1 || item.ChatModels[0] != "chat-model" {
				t.Fatalf("custom chat models = %v", item.ChatModels)
			}
			return
		}
	}
	t.Fatalf("custom provider missing from settings: %+v", settings.Providers)
}

func TestSubscriptionMetadataHidesCustomRouteUntilCredentialIsStored(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)

	cfg := config.Default()
	cfg.DefaultModel = "kwu/gpt-5.4"
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	cfg.Desktop.ProviderAccess = []string{"custom"}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{
		Name: "custom", Kind: "openai", BaseURL: "https://example.invalid/v1",
		Model: "chat-model", APIKeyEnv: "KEYRING_ONLY_KEY",
	})
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := &WorkspaceTab{
		ID: "subscription", Ready: true, model: "kwu/gpt-5.4",
		billingSource: string(modelbilling.BillingSubscriptionPool),
		actor:         userctx.Default(""),
	}
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID

	settings := app.Settings()
	foundProvider := false
	for _, item := range settings.Providers {
		if item.Name != "custom" {
			continue
		}
		foundProvider = true
		if item.Configured || item.KeySet || item.Availability != "credential_unverified" {
			t.Fatalf("metadata-only custom provider = %+v", item)
		}
	}
	if !foundProvider {
		t.Fatalf("custom provider missing from settings: %+v", settings.Providers)
	}
	models := app.ModelsForTab(tab.ID)
	for _, model := range models {
		if model.Ref == "custom/chat-model" && model.BillingSource == string(modelbilling.BillingNewAPIByok) {
			t.Fatalf("custom route without credential is visible: %+v", models)
		}
	}
	for _, model := range app.AutomationModels("") {
		if model.Ref == "custom/chat-model" && model.BillingSource == string(modelbilling.BillingNewAPIByok) {
			t.Fatalf("automation route without credential is visible: %+v", app.AutomationModels(""))
		}
	}
	if _, err := config.SetCredentialWithPaths(config.NewProfilePathResolver(tab.actor), "KEYRING_ONLY_KEY", "sk-custom"); err != nil {
		t.Fatal(err)
	}
	models = app.ModelsForTab(tab.ID)
	automationModels := app.AutomationModels("")
	settings = app.Settings()
	modelVisible := false
	automationVisible := false
	settingsVisible := false
	for _, model := range models {
		if model.Ref == "custom/chat-model" && model.BillingSource == string(modelbilling.BillingNewAPIByok) {
			modelVisible = true
		}
	}
	for _, model := range automationModels {
		if model.Ref == "custom/chat-model" && model.BillingSource == string(modelbilling.BillingNewAPIByok) {
			automationVisible = true
		}
	}
	for _, item := range settings.Providers {
		if item.Name == "custom" && item.Added && item.Configured && len(item.ChatModels) == 1 && item.ChatModels[0] == "chat-model" {
			settingsVisible = true
		}
	}
	if !modelVisible || !automationVisible || !settingsVisible {
		t.Fatalf("custom route after credential: models=%+v automation=%+v settings=%+v", models, automationModels, settings.Providers)
	}
}
