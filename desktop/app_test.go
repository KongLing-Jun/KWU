package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/guardian"
	"kwu/internal/jobs"
	"kwu/internal/memory"
	"kwu/internal/modelbilling"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/store"
	"kwu/internal/temporal"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

const (
	testKWUDefaultModelRef = "kwu/deepseek-v4-flash"
	testKWUFlashModelRef   = "kwu/gpt-5.4-mini"
)

func desktopMCPHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]any{"name": "h", "version": "0"},
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "greet",
				"description": "Greet someone.",
				"inputSchema": map[string]any{"type": "object"},
			}}}
		default:
			result = map[string]any{}
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// setTestCtrl creates a minimal workspace tab (if needed) and sets its
// controller, so tests don't depend on the old App.ctrl field.
func (a *App) setTestCtrl(ctrl control.SessionAPI, model string) {
	if len(a.tabRuntimes) == 0 {
		tab := &WorkspaceTab{
			ID:          "test",
			Scope:       "global",
			Ready:       true,
			disabledMCP: map[string]ServerView{},
		}
		a.tabRuntimes = map[string]*WorkspaceTab{"test": tab}
		a.activeTabID = "test"
	}
	tab := a.tabRuntimes["test"]
	tab.Ctrl = ctrl
	a.bindControllerDisplayRecorder(ctrl)
	tab.model = model
}

func TestSetClientTimezoneUpdatesOpenTabsAndFutureBootOptions(t *testing.T) {
	app := NewApp()
	times := temporal.New(temporal.Options{Configured: "UTC"})
	ctrl := control.New(control.Options{Temporal: times})
	app.setTestCtrl(ctrl, "test/model")

	if err := app.SetClientTimezone("Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	if got := ctrl.Timezone(); got != "Asia/Shanghai" {
		t.Fatalf("open tab timezone = %q", got)
	}
	if got := app.ClientTimezone(); got != "Asia/Shanghai" {
		t.Fatalf("client timezone = %q", got)
	}
	options := boot.Options{}
	app.applyClientTimezone(&options)
	if options.Timezone != "Asia/Shanghai" {
		t.Fatalf("future boot timezone = %q", options.Timezone)
	}

	if err := app.SetClientTimezone("invalid/zone"); err == nil {
		t.Fatal("invalid timezone was accepted")
	}
	if got := ctrl.Timezone(); got != "Asia/Shanghai" {
		t.Fatalf("invalid update changed open tab timezone to %q", got)
	}
}

func isolateDesktopUserDirs(t *testing.T) string {
	t.Helper()
	home := robustTempDir(t)
	xdg := filepath.Join(home, ".config")
	appData := filepath.Join(home, "AppData")
	for _, dir := range []string{xdg, appData} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("KWU_HOME", filepath.Join(home, ".kwu"))
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("KWU_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("KWU_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("AppData", appData)
	return home
}

func setDesktopTestCredential(t *testing.T, key, value string) {
	t.Helper()
	if _, err := config.SetCredential(key, value); err != nil {
		t.Fatalf("SetCredential(%s): %v", key, err)
	}
}

const (
	desktopTestBYOKModelRef      = "custom/chat-model"
	desktopTestBYOKBillingSource = "newapi_byok"
)

func configureDesktopTestBYOKRoute(t *testing.T) {
	t.Helper()
	setDesktopTestCredential(t, "DESKTOP_TEST_BYOK_KEY", "sk-test")
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
model = "chat-model"
api_key_env = "DESKTOP_TEST_BYOK_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}
}

const desktopActorProviderKind = "desktop-actor-test"

var desktopActorProviderOnce sync.Once

func registerDesktopActorProvider() {
	desktopActorProviderOnce.Do(func() {
		provider.Register(desktopActorProviderKind, func(provider.Config) (provider.Provider, error) {
			return stubProvider{}, nil
		})
	})
}

func TestNeedsOnboardingIgnoresInheritedEnv(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv(onboardingKeyEnv, "inherited-key")

	app := NewApp()
	if !app.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should require a key saved in KWU global .env")
	}
	setDesktopTestCredential(t, onboardingKeyEnv, "saved-key")
	if app.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should be false after saving the global credential")
	}
}

func TestNeedsOnboardingTreatsBlankSavedKeyAsMissing(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserCredentialsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserCredentialsPath(), []byte(onboardingKeyEnv+"=\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	if !app.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should require a non-empty saved credential")
	}
}

func TestNeedsOnboardingUsesActiveProfileCredentials(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	t.Setenv(onboardingKeyEnv, "")
	os.Unsetenv(onboardingKeyEnv)

	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"
	pathsA := config.NewProfilePathResolver(actorA)
	pathsB := config.NewProfilePathResolver(actorB)

	if _, err := config.SetCredentialWithPaths(pathsA, onboardingKeyEnv, "profile-a-key"); err != nil {
		t.Fatalf("SetCredentialWithPaths profile A: %v", err)
	}
	setDesktopTestCredential(t, onboardingKeyEnv, "default-key")

	appA := &App{
		tabRuntimes: map[string]*WorkspaceTab{"a": {
			ID:            "a",
			WorkspaceRoot: project,
			actor:         actorA,
		}},
		activeTabID: "a",
	}
	if appA.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should be false when the active profile has the onboarding key")
	}

	appB := &App{
		tabRuntimes: map[string]*WorkspaceTab{"b": {
			ID:            "b",
			WorkspaceRoot: project,
			actor:         actorB,
		}},
		activeTabID: "b",
	}
	if config.CredentialStoredWithPaths(pathsB, onboardingKeyEnv) {
		t.Fatal("test setup error: profile B should not have the onboarding key")
	}
	if !appB.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should ignore the default profile key when the active profile has no key")
	}
}

func TestNeedsOnboardingUsesActiveProfileConfiguredProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := t.TempDir()
	t.Setenv(onboardingKeyEnv, "")
	os.Unsetenv(onboardingKeyEnv)

	actorA := userctx.Default(project)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(project)
	actorB.ProfileID = "profile-b"
	pathsA := config.NewProfilePathResolver(actorA)
	if err := os.MkdirAll(filepath.Dir(pathsA.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir profile config dir: %v", err)
	}
	if err := os.WriteFile(pathsA.UserConfigPath(), []byte(`
default_model = "local/model-a"

[desktop]
provider_access = ["local"]

[[providers]]
name = "local"
kind = "openai"
base_url = "http://127.0.0.1:23333/v1"
models = ["model-a"]
default = "model-a"
api_key_env = "LOCAL_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write profile config: %v", err)
	}

	appA := &App{
		tabRuntimes: map[string]*WorkspaceTab{"a": {
			ID:            "a",
			WorkspaceRoot: project,
			actor:         actorA,
		}},
		activeTabID: "a",
	}
	if appA.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should be false when the active profile has a configured accessible chat provider")
	}

	appB := &App{
		tabRuntimes: map[string]*WorkspaceTab{"b": {
			ID:            "b",
			WorkspaceRoot: project,
			actor:         actorB,
		}},
		activeTabID: "b",
	}
	if !appB.NeedsOnboarding() {
		t.Fatal("NeedsOnboarding should only use the active profile's configured providers")
	}
}

func providerNamesFromView(providers []ProviderView) []string {
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Name)
	}
	return out
}

func settingsProvidersIncludingOfficial(view SettingsView) []ProviderView {
	providers := append([]ProviderView(nil), view.Providers...)
	return append(providers, view.OfficialProviders...)
}

func modelRefsFromView(models []ModelInfo) map[string]bool {
	out := map[string]bool{}
	for _, m := range models {
		out[m.Ref] = true
	}
	return out
}

type desktopFakeTool struct {
	name string
}

func (t desktopFakeTool) Name() string { return t.name }

func (desktopFakeTool) Description() string { return "fake desktop tool" }

func (desktopFakeTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }

func (desktopFakeTool) Execute(context.Context, json.RawMessage) (string, error) { return "", nil }

func (desktopFakeTool) ReadOnly() bool { return true }

type desktopAskRuntimeRunner struct {
	ask func(context.Context) error
}

func (r *desktopAskRuntimeRunner) Run(ctx context.Context, _ string) error {
	if r.ask == nil {
		return nil
	}
	return r.ask(ctx)
}

func TestCommandsIncludesEffortNotThinking(t *testing.T) {
	app := NewApp()
	cmds := app.Commands()
	if !hasCommand(cmds, "effort") {
		t.Fatalf("Commands() should include effort: %+v", cmds)
	}
	if hasCommand(cmds, "thinking") {
		t.Fatalf("Commands() should not include thinking: %+v", cmds)
	}
}

func TestMetaForTabIncludesWorkspaceContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	isolateDesktopUserDirs(t)
	resetWorkspaceGitBranchMetaCacheForTest(t)

	repo := t.TempDir()
	configuredSandboxRoot := filepath.Join(t.TempDir(), "sandbox")
	cfg := config.LoadForEdit(config.UserConfigPath())
	cfg.Sandbox.WorkspaceRoot = configuredSandboxRoot
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	runGit(t, "init")
	runGit(t, "checkout", "-b", "feature/meta")

	app := NewApp()
	sessionPath := filepath.Join(repo, "session.jsonl")
	app.tabRuntimes = map[string]*WorkspaceTab{"tab-1": {
		ID:            "tab-1",
		Scope:         "project",
		WorkspaceRoot: repo,
		SessionPath:   sessionPath,
		Label:         "provider/model + planner provider/planner",
		model:         "provider/model",
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}}
	app.activeTabID = "tab-1"

	got := app.MetaForTab("tab-1")
	if got.Cwd != repo || got.WorkspaceRoot != repo || got.WorkspacePath != repo {
		t.Fatalf("workspace fields = cwd:%q root:%q path:%q, want %q", got.Cwd, got.WorkspaceRoot, got.WorkspacePath, repo)
	}
	if got.WorkspaceName != filepath.Base(repo) {
		t.Fatalf("workspaceName = %q, want %q", got.WorkspaceName, filepath.Base(repo))
	}
	if got.SessionPath != sessionPath {
		t.Fatalf("sessionPath = %q, want %q", got.SessionPath, sessionPath)
	}
	if got.ModelRef != "provider/model" {
		t.Fatalf("modelRef = %q, want saved model independent of label %q", got.ModelRef, got.Label)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	if strings.Contains(string(raw), "sandboxPath") || strings.Contains(string(raw), configuredSandboxRoot) {
		t.Fatalf("meta should not expose configured sandbox root as sandboxPath: %s", raw)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if got = app.MetaForTab("tab-1"); got.GitBranch == "feature/meta" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("gitBranch = %q, want feature/meta after async refresh", got.GitBranch)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestListTabsDoesNotExposeConfiguredSandboxPath(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspace := t.TempDir()
	configuredSandboxRoot := filepath.Join(t.TempDir(), "sandbox")
	cfg := config.LoadForEdit(config.UserConfigPath())
	cfg.Sandbox.WorkspaceRoot = configuredSandboxRoot
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"tab-1": {
		ID:            "tab-1",
		Scope:         "project",
		WorkspaceRoot: workspace,
		Label:         "provider/model + planner provider/planner",
		model:         "provider/model",
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}}
	app.activeTabID = "tab-1"
	app.tabOrder = []string{"tab-1"}

	tabs := app.ListTabs()
	if len(tabs) != 1 || tabs[0].ModelRef != "provider/model" {
		t.Fatalf("tab modelRef = %+v, want saved model independent of label", tabs)
	}

	raw, err := json.Marshal(tabs)
	if err != nil {
		t.Fatalf("marshal tabRuntimes: %v", err)
	}
	if strings.Contains(string(raw), "sandboxPath") || strings.Contains(string(raw), configuredSandboxRoot) {
		t.Fatalf("tab metadata should not expose configured sandbox root as sandboxPath: %s", raw)
	}
}

func TestListTabsExposesStructuredRuntimeStatus(t *testing.T) {
	asks := make(chan event.Ask, 1)
	done := make(chan event.Event, 1)
	runner := &desktopAskRuntimeRunner{}
	ctrl := control.New(control.Options{
		Runner: runner,
		Sink: event.FuncSink(func(e event.Event) {
			switch e.Kind {
			case event.AskRequest:
				asks <- e.Ask
			case event.TurnDone:
				done <- e
			}
		}),
	})
	runner.ask = func(ctx context.Context) error {
		_, err := ctrl.Ask(ctx, []event.AskQuestion{{
			ID:      "choice",
			Prompt:  "Pick one",
			Options: []event.AskOption{{Label: "A"}, {Label: "B"}},
		}})
		return err
	}

	app := NewApp()
	app.setTestCtrl(ctrl, "prov/model")
	app.tabOrder = []string{"test"}
	ctrl.Send("ask user")
	select {
	case <-asks:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ask request")
	}

	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs = %d, want 1", len(tabs))
	}
	if !tabs[0].Running || !tabs[0].PendingPrompt || !tabs[0].Cancellable || tabs[0].CancelRequested {
		t.Fatalf("tab runtime = running:%v pending:%v cancellable:%v cancel:%v", tabs[0].Running, tabs[0].PendingPrompt, tabs[0].Cancellable, tabs[0].CancelRequested)
	}

	app.CancelTab("test")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turn_done")
	}
}

func TestMetaForTabLeavesGitBranchEmptyOutsideGit(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspace := t.TempDir()
	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"tab-1": {
		ID:            "tab-1",
		Scope:         "project",
		WorkspaceRoot: workspace,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}}
	app.activeTabID = "tab-1"

	if got := app.MetaForTab("tab-1"); got.GitBranch != "" {
		t.Fatalf("gitBranch = %q, want empty", got.GitBranch)
	}
}

func TestEffortDefaultsBeforeStartup(t *testing.T) {
	isolateDesktopUserDirs(t)

	got := NewApp().Effort()
	if !got.Supported || got.Current != "auto" || got.Default != "auto" ||
		!hasLevel(got.Levels, "auto") || !hasLevel(got.Levels, "low") ||
		!hasLevel(got.Levels, "medium") || !hasLevel(got.Levels, "high") {
		t.Fatalf("pre-startup Effort() = %+v, want KWU OpenAI effort defaults", got)
	}
}

func TestMemoryViewReturnsNonNilArraysBeforeStartup(t *testing.T) {
	isolateDesktopUserDirs(t)

	view := NewApp().Memory()
	if !view.Available {
		t.Fatal("Memory() should load file-backed memory before a controller starts")
	}
	if view.Docs == nil || view.Facts == nil || view.Archives == nil || view.Scopes == nil {
		t.Fatalf("Memory() arrays must be non-nil before startup: %+v", view)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal Memory(): %v", err)
	}
	for _, bad := range []string{`"docs":null`, `"facts":null`, `"archives":null`, `"scopes":null`} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("Memory() JSON contains %s; frontend expects []: %s", bad, raw)
		}
	}
}

func TestMemoryForRestoredTabWorksBeforeControllerStarts(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspace := t.TempDir()
	docPath := filepath.Join(workspace, "KWU.md")
	if err := os.WriteFile(docPath, []byte("# Existing memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"restored": {
			ID:            "restored",
			Scope:         "project",
			WorkspaceRoot: workspace,
			Ready:         false,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.tabOrder = []string{"restored"}
	app.activeTabID = "restored"

	view := app.MemoryForTab("restored")
	if !view.Available || len(view.Docs) != 1 || view.Docs[0].Path != docPath {
		t.Fatalf("MemoryForTab() before controller = %+v, want workspace doc", view)
	}

	written, err := app.RememberForTab("restored", "project", "remember while booting")
	if err != nil {
		t.Fatalf("RememberForTab: %v", err)
	}
	if written != docPath {
		t.Fatalf("RememberForTab path = %q, want %q", written, docPath)
	}
	body, err := os.ReadFile(docPath)
	if err != nil || !strings.Contains(string(body), "remember while booting") {
		t.Fatalf("remembered doc = %q, %v", body, err)
	}

	if _, err := app.SaveDocForTab("restored", docPath, "# Edited memory"); err != nil {
		t.Fatalf("SaveDocForTab: %v", err)
	}
	body, err = os.ReadFile(docPath)
	if err != nil || string(body) != "# Edited memory\n" {
		t.Fatalf("saved doc = %q, %v", body, err)
	}

	actor := actorContextForWorkspace(userctx.ActorContext{}.Normalize(), workspace)
	paths := config.NewProfilePathResolver(actor)
	set := memory.Load(memory.Options{
		CWD:              workspace,
		UserDir:          paths.MemoryUserDir(),
		ProjectStoreRoot: paths.MemoryProjectStoreRoot(),
	})
	if _, err := set.Store.Save(memory.Memory{Name: "stale-fact", Description: "remove me", Type: memory.TypeProject}); err != nil {
		t.Fatalf("save fact: %v", err)
	}
	if err := app.ForgetForTab("restored", "stale-fact"); err != nil {
		t.Fatalf("ForgetForTab: %v", err)
	}
	view = app.MemoryForTab("restored")
	if len(view.Facts) != 0 || len(view.Archives) != 1 || view.Archives[0].Name != "stale-fact" {
		t.Fatalf("memory after forget = %+v", view)
	}

	if missing := app.MemoryForTab("missing"); missing.Available {
		t.Fatalf("missing tab must not fall back to active workspace: %+v", missing)
	}
}

func TestMemoryViewIncludesActiveAndArchivedFacts(t *testing.T) {
	isolateDesktopUserDirs(t)
	userDir := t.TempDir()
	cwd := t.TempDir()
	store := memory.Store{Dir: filepath.Join(userDir, "projects", "test", "memory")}
	if _, err := store.Save(memory.Memory{
		Name:        "active-fact",
		Title:       "Active fact",
		Description: "Still applies",
		Type:        memory.TypeProject,
		Body:        "Active body",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(memory.Memory{
		Name:        "archived-fact",
		Description: "No longer applies",
		Type:        memory.TypeFeedback,
		Body:        "Archived body",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Archive("archived-fact"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Memory: &memory.Set{
		Docs:    []memory.Source{{Path: filepath.Join(cwd, "AGENTS.md"), Scope: memory.ScopeProject, Body: "Project instructions"}},
		Store:   store,
		CWD:     cwd,
		UserDir: userDir,
	}}), "test-model")

	view := app.Memory()
	if !view.Available || view.StoreDir != store.Dir {
		t.Fatalf("Memory() availability/store = %v/%q, want true/%q", view.Available, view.StoreDir, store.Dir)
	}
	if len(view.Docs) != 1 || view.Docs[0].Scope != "project" || !strings.Contains(view.Docs[0].Body, "Project instructions") {
		t.Fatalf("Memory() docs = %+v", view.Docs)
	}
	if len(view.Facts) != 1 || view.Facts[0].Name != "active-fact" || view.Facts[0].Type != "project" {
		t.Fatalf("Memory() active facts = %+v", view.Facts)
	}
	if len(view.Archives) != 1 || view.Archives[0].Name != "archived-fact" || view.Archives[0].Type != "feedback" ||
		view.Archives[0].Path == "" || view.Archives[0].ArchivedAt == "" {
		t.Fatalf("Memory() archived facts = %+v", view.Archives)
	}
	if len(view.Scopes) != 3 {
		t.Fatalf("Memory() scopes = %+v, want user/project/local", view.Scopes)
	}
}

func TestMemoryViewHidesProfileScopedFactsFromSharedProjectStore(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "workspace")
	userDir := filepath.Join(root, "profiles", "alpha")
	store := memory.StoreForProjectRoot(userDir, root, cwd)

	if _, err := store.Save(memory.Memory{
		Name:        "alpha-user",
		Description: "current profile user",
		Type:        memory.TypeUser,
		Body:        "Current profile user body",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(memory.Memory{
		Name:        "shared-project",
		Description: "workspace shared project",
		Type:        memory.TypeProject,
		Body:        "Shared project body",
	}); err != nil {
		t.Fatal(err)
	}
	writeDesktopMemoryFact(t, store.Dir, "leaked-user", memory.TypeUser, "other profile user")
	writeDesktopArchivedMemoryFact(t, store.Dir, "20260102-010000.000-leaked-archived.md", "leaked-archived", memory.TypeFeedback, "other profile archived")
	writeDesktopArchivedMemoryFact(t, store.Dir, "20260102-020000.000-shared-archived.md", "shared-archived", memory.TypeProject, "workspace shared archived")

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Memory: &memory.Set{
		Store:            store,
		CWD:              cwd,
		UserDir:          userDir,
		ProjectStoreRoot: root,
	}}), "test-model")

	view := app.Memory()
	factNames := map[string]bool{}
	for _, f := range view.Facts {
		factNames[f.Name] = true
	}
	for _, want := range []string{"alpha-user", "shared-project"} {
		if !factNames[want] {
			t.Fatalf("Memory() facts missing %q: %+v", want, view.Facts)
		}
	}
	if factNames["leaked-user"] {
		t.Fatalf("Memory() facts leaked profile-scoped shared project memory: %+v", view.Facts)
	}
	archiveNames := map[string]bool{}
	for _, f := range view.Archives {
		archiveNames[f.Name] = true
	}
	if !archiveNames["shared-archived"] {
		t.Fatalf("Memory() archives missing shared project memory: %+v", view.Archives)
	}
	if archiveNames["leaked-archived"] {
		t.Fatalf("Memory() archives leaked profile-scoped shared project memory: %+v", view.Archives)
	}
}

func writeDesktopMemoryFact(t *testing.T, dir, name string, typ memory.Type, desc string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\ntype: " + string(typ) + "\n---\n\n" + desc + " body\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDesktopArchivedMemoryFact(t *testing.T, dir, file, name string, typ memory.Type, desc string) {
	t.Helper()
	archiveDir := filepath.Join(dir, ".archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\ntype: " + string(typ) + "\n---\n\n" + desc + " body\n"
	if err := os.WriteFile(filepath.Join(archiveDir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBeforeCloseAllowsSystemQuitWhenBackgroundCloseEnabled(t *testing.T) {
	isolateDesktopUserDirs(t)
	consumeSystemQuitRequested()
	t.Cleanup(func() { consumeSystemQuitRequested() })

	userCfg := config.LoadForEdit(config.UserConfigPath())
	if err := userCfg.SetDesktopCloseBehavior("background"); err != nil {
		t.Fatal(err)
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	markSystemQuitRequested()
	if prevent := NewApp().beforeClose(context.Background()); prevent {
		t.Fatal("system quit should bypass background close-to-tray behavior")
	}
	if consumeSystemQuitRequested() {
		t.Fatal("system quit marker should be consumed by beforeClose")
	}
}

func TestShutdownDoesNotReadWindowStateAfterFrontendTeardown(t *testing.T) {
	// A plain context has no Wails frontend. Accessing WindowGetSize with it
	// panics, which models the invalid runtime context seen after the Windows
	// native window has already been destroyed.
	app := &App{ctx: context.Background()}
	app.shutdown(context.Background())
}

func TestBackgroundCloseHideStrategyByPlatform(t *testing.T) {
	tests := []struct {
		goos string
		want bool
	}{
		{goos: "darwin", want: true},
		{goos: "windows", want: false},
		{goos: "linux", want: false},
		{goos: "freebsd", want: false},
	}
	for _, tt := range tests {
		if got := backgroundCloseUsesApplicationHide(tt.goos); got != tt.want {
			t.Fatalf("backgroundCloseUsesApplicationHide(%q) = %v, want %v", tt.goos, got, tt.want)
		}
	}
}

func TestBackgroundRestoreMaximiseStrategy(t *testing.T) {
	tests := []struct {
		goos      string
		maximised bool
		want      bool
	}{
		{goos: "windows", maximised: true, want: true},
		{goos: "linux", maximised: true, want: true},
		{goos: "darwin", maximised: true, want: false},
		{goos: "windows", maximised: false, want: false},
	}
	for _, tt := range tests {
		if got := backgroundRestoreShouldMaximise(tt.goos, tt.maximised); got != tt.want {
			t.Fatalf("backgroundRestoreShouldMaximise(%q, %v) = %v, want %v", tt.goos, tt.maximised, got, tt.want)
		}
	}
}

func TestBackgroundRestorePlanAvoidsNormalWindowFlash(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		maximised bool
		want      backgroundRestorePlan
	}{
		{
			name:      "maximised Windows window",
			goos:      "windows",
			maximised: true,
			want:      backgroundRestorePlan{maximiseBeforeShow: true},
		},
		{
			name:      "normal Windows window",
			goos:      "windows",
			maximised: false,
			want:      backgroundRestorePlan{unminimiseAfterShow: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backgroundRestorePlanFor(tt.goos, tt.maximised)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("backgroundRestorePlanFor(%q, %v) = %v, want %v", tt.goos, tt.maximised, got, tt.want)
			}
		})
	}
}

func TestEmitReadyInvokesReadyHook(t *testing.T) {
	app := NewApp()
	var calls int32
	app.readyHook = func() {
		atomic.AddInt32(&calls, 1)
	}

	app.emitReady(context.TODO())

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("ready hook calls = %d, want 1", got)
	}
}

func TestSetEffortWithoutActiveSessionKeepsAutoAndDoesNotPersist(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if err := app.SetEffort("high"); err != nil {
		t.Fatalf("SetEffort(high): %v", err)
	}
	if got := app.Effort().Current; got != "auto" {
		t.Fatalf("inactive-session effort = %q, want auto", got)
	}
	body, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	bodyText := string(body)
	for _, forbidden := range []string{
		`[[providers]]`,
		config.OfficialProviderBaseURL,
		config.OfficialProviderModelsURL,
		`effort      =`,
		`default_effort`,
	} {
		if strings.Contains(bodyText, forbidden) {
			t.Fatalf("inactive-session effort wrote %q to config:\n%s", forbidden, bodyText)
		}
	}
}

func TestSettingsUsesUserDesktopPreferencesNotProjectConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[desktop]
language = "zh"
layout_style = "workbench"
theme = "light"
theme_style = "glacier"
close_behavior = "quit"
status_bar_style = "icon"
status_bar_items = ["cost", "balance"]
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	userCfg := config.LoadForEdit(config.UserConfigPath())
	if err := userCfg.SetDesktopLanguage("en"); err != nil {
		t.Fatalf("set desktop language: %v", err)
	}
	if err := userCfg.SetDesktopLayoutStyle("classic"); err != nil {
		t.Fatalf("set desktop layout style: %v", err)
	}
	if err := userCfg.SetDesktopAppearance("dark", "graphite"); err != nil {
		t.Fatalf("set desktop appearance: %v", err)
	}
	userCfg.Desktop.ThemeStyle = "graphite"
	if err := userCfg.SetDesktopCloseBehavior("background"); err != nil {
		t.Fatalf("set desktop close behavior: %v", err)
	}
	if err := userCfg.SetDesktopStatusBarStyle("text"); err != nil {
		t.Fatalf("set desktop status bar style: %v", err)
	}
	if err := userCfg.SetDesktopStatusBarItems([]string{"model", "balance", "cache"}); err != nil {
		t.Fatalf("set desktop status bar items: %v", err)
	}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	got := NewApp().Settings()
	if got.DesktopLanguage != "en" || got.DesktopLayoutStyle != "classic" || got.DesktopTheme != "dark" || got.DesktopThemeStyle != "graphite" || got.CloseBehavior != "background" || got.StatusBarStyle != "text" {
		t.Fatalf("desktop settings = lang:%q layout:%q theme:%q style:%q close:%q status:%q, want user-level desktop prefs", got.DesktopLanguage, got.DesktopLayoutStyle, got.DesktopTheme, got.DesktopThemeStyle, got.CloseBehavior, got.StatusBarStyle)
	}
	if want := []string{"model", "balance", "cache"}; !reflect.DeepEqual(got.StatusBarItems, want) {
		t.Fatalf("desktop status bar items = %v, want user-level %v", got.StatusBarItems, want)
	}
}

func TestDesktopStartupSettingsUsesUserDesktopPreferencesWithoutFullSettingsPayload(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	paths := app.activeProfilePathResolver()
	configPath := paths.UserConfigPath()

	userCfg := config.LoadForEditWithPaths(paths)
	if err := userCfg.SetDesktopLanguage("en"); err != nil {
		t.Fatalf("set desktop language: %v", err)
	}
	if err := userCfg.SetDesktopLayoutStyle("classic"); err != nil {
		t.Fatalf("set desktop layout style: %v", err)
	}
	if err := userCfg.SetDesktopAppearance("dark", "graphite"); err != nil {
		t.Fatalf("set desktop appearance: %v", err)
	}
	userCfg.Desktop.ThemeStyle = "graphite"
	if err := userCfg.SetDesktopStatusBarStyle("icon"); err != nil {
		t.Fatalf("set desktop status bar style: %v", err)
	}
	if err := userCfg.SetDesktopStatusBarItems([]string{"workspace", "git_branch", "model"}); err != nil {
		t.Fatalf("set desktop status bar items: %v", err)
	}
	if err := userCfg.SetDesktopCheckUpdates(false); err != nil {
		t.Fatalf("set desktop check updates: %v", err)
	}
	userCfg.Bot.Enabled = true
	userCfg.Bot.Allowlist.Enabled = true
	userCfg.Bot.Allowlist.QQUsers = []string{"alice"}
	if err := userCfg.SaveTo(configPath); err != nil {
		t.Fatalf("save user config: %v", err)
	}
	controlBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read control config: %v", err)
	}
	if !strings.Contains(string(controlBody), "\ntheme_style = \"graphite\"") {
		t.Fatalf("control config must retain historical desktop theme style:\n%s", controlBody)
	}

	got := app.DesktopStartupSettings()
	if got.DesktopLanguage != "en" || got.DesktopLayoutStyle != "classic" || got.DesktopTheme != "dark" || got.DesktopThemeStyle != "" || got.DisplayMode != "standard" || got.StatusBarStyle != "icon" || got.CheckUpdates {
		t.Fatalf("DesktopStartupSettings desktop prefs = %+v, want user-level startup prefs", got)
	}
	experimentBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(experimentBody), "\ntheme_style = \"graphite\"") {
		t.Fatalf("migrated config retained historical desktop theme style:\n%s", experimentBody)
	}
	migrated := config.LoadForEditWithPaths(paths)
	if migrated.DesktopTheme() != "dark" || migrated.DesktopLanguage() != "en" || migrated.DesktopLayoutStyle() != "classic" || migrated.DesktopStatusBarStyle() != "icon" {
		t.Fatalf("migration changed unrelated desktop preferences: %+v", migrated.Desktop)
	}

	stableTime := time.Unix(946684800, 0)
	if err := os.Chtimes(configPath, stableTime, stableTime); err != nil {
		t.Fatalf("set migrated config timestamp: %v", err)
	}
	second := NewApp().DesktopStartupSettings()
	if second.DesktopThemeStyle != "" {
		t.Fatalf("second DesktopStartupSettings style = %q, want empty", second.DesktopThemeStyle)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat migrated config: %v", err)
	}
	if !info.ModTime().Equal(stableTime) {
		t.Fatalf("second startup rewrote migrated config: modtime %v, want %v", info.ModTime(), stableTime)
	}
	if want := []string{"workspace", "git_branch", "model"}; !reflect.DeepEqual(got.StatusBarItems, want) {
		t.Fatalf("DesktopStartupSettings status bar items = %v, want %v", got.StatusBarItems, want)
	}
	if !got.Bot.Enabled || !got.Bot.Allowlist.Enabled || !reflect.DeepEqual(got.Bot.Allowlist.QQUsers, []string{"alice"}) {
		t.Fatalf("DesktopStartupSettings bot settings = %+v, want lightweight bot snapshot", got.Bot)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal DesktopStartupSettings: %v", err)
	}
	if strings.Contains(string(raw), "providers") || strings.Contains(string(raw), "officialProviders") || strings.Contains(string(raw), "providerKinds") {
		t.Fatalf("DesktopStartupSettings must not include full Settings provider payload: %s", raw)
	}
}

func TestDesktopStartupSettingsMigratesThemeStyleWithIsolatedResolvers(t *testing.T) {
	isolateDesktopUserDirs(t)

	tests := []struct {
		name  string
		actor userctx.ActorContext
	}{
		{name: "production", actor: userctx.Default("")},
		{name: "development_profile", actor: userctx.ActorContext{ProfileID: "development-isolated"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			paths := config.NewProfilePathResolverWithOptions(config.ProfilePathResolverOptions{
				Actor:     tt.actor,
				HomeRoot:  filepath.Join(root, "home"),
				StateRoot: filepath.Join(root, "state"),
				CacheRoot: filepath.Join(root, "cache"),
			})
			cfg := config.Default()
			cfg.Desktop.Theme = "auto"
			cfg.Desktop.ThemeStyle = "graphite"
			cfg.Desktop.Language = "zh"
			if err := cfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
				t.Fatalf("save control config: %v", err)
			}

			got, err := NewApp().desktopStartupSettingsWithPaths(paths, root)
			if err != nil {
				t.Fatalf("desktop startup settings: %v", err)
			}
			if got.DesktopTheme != "auto" || got.DesktopThemeStyle != "" || got.DesktopLanguage != "zh" {
				t.Fatalf("startup settings = %+v, want theme/language preserved and style cleared", got)
			}
			saved := config.LoadForEditWithPaths(paths)
			if saved.Desktop.Theme != "auto" || saved.Desktop.ThemeStyle != "" || saved.Desktop.Language != "zh" {
				t.Fatalf("migrated config = %+v, want theme/language preserved and style cleared", saved.Desktop)
			}
		})
	}
}

func BenchmarkDesktopSettingsPayloads(b *testing.B) {
	home := b.TempDir()
	xdg := filepath.Join(home, ".config")
	appData := filepath.Join(home, "AppData")
	for _, dir := range []string{xdg, appData} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
	}
	b.Setenv("HOME", home)
	b.Setenv("KWU_CREDENTIALS_STORE", "file")
	b.Setenv("USERPROFILE", home)
	b.Setenv("XDG_CONFIG_HOME", xdg)
	b.Setenv("KWU_STATE_HOME", filepath.Join(home, "state"))
	b.Setenv("KWU_CACHE_HOME", filepath.Join(home, "cache"))
	b.Setenv("AppData", appData)
	b.Setenv("SHARED_PROVIDER_KEY", "sk-test")

	cfg := config.LoadForEdit(config.UserConfigPath())
	for i := 0; i < 40; i++ {
		cfg.Providers = append(cfg.Providers, config.ProviderEntry{
			Name:      fmt.Sprintf("custom-%02d", i),
			Kind:      "openai",
			BaseURL:   "https://example.invalid/v1",
			APIKeyEnv: "SHARED_PROVIDER_KEY",
			Models:    []string{"model-a", "model-b"},
			Default:   "model-a",
		})
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		b.Fatalf("save config: %v", err)
	}
	app := NewApp()

	b.Run("Settings", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = app.Settings()
		}
	})
	b.Run("DesktopStartupSettings", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = app.DesktopStartupSettings()
		}
	})
}

func TestSettingsIgnoresActiveWorkspaceDotEnvCredentialsWithUserConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	launch := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("WORKSPACE_ONLY_KEY=from-project\n"), 0o600); err != nil {
		t.Fatalf("write project env: %v", err)
	}
	userCfg := config.LoadForEdit(config.UserConfigPath())
	if err := userCfg.UpsertProvider(config.ProviderEntry{
		Name:      "workspace-provider",
		Kind:      "openai",
		BaseURL:   "https://workspace.example/v1",
		Model:     "workspace-model",
		APIKeyEnv: "WORKSPACE_ONLY_KEY",
	}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	userCfg.Desktop.ProviderAccess = []string{"workspace-provider"}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}
	t.Setenv("WORKSPACE_ONLY_KEY", "")
	os.Unsetenv("WORKSPACE_ONLY_KEY")
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(launch); err != nil {
		t.Fatalf("chdir launch: %v", err)
	}

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"project": {ID: "project", WorkspaceRoot: project}}
	app.activeTabID = "project"
	got := app.Settings()
	for _, p := range got.Providers {
		if p.Name == "workspace-provider" {
			if p.KeySet {
				t.Fatalf("workspace provider keySet = true, want false because workspace .env is ignored: %+v", p)
			}
			if p.Configured {
				t.Fatalf("workspace provider configured = true, want false because workspace .env is ignored: %+v", p)
			}
			return
		}
	}
	t.Fatalf("workspace provider missing from settings: %+v", got.Providers)
}

func TestSettingsShowsGlobalCredentialWithoutMutatingWorkspaceEnv(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	launch := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("SHARED_SETTINGS_KEY=from-project\n"), 0o600); err != nil {
		t.Fatalf("write project env: %v", err)
	}
	if _, err := config.SetCredential("SHARED_SETTINGS_KEY", "from-credentials"); err != nil {
		t.Fatalf("SetCredential: %v", err)
	}
	userCfg := config.LoadForEditWithoutCredentials(config.UserConfigPath())
	if err := userCfg.UpsertProvider(config.ProviderEntry{
		Name:      "settings-provider",
		Kind:      "openai",
		BaseURL:   "https://settings.example/v1",
		Model:     "settings-model",
		APIKeyEnv: "SHARED_SETTINGS_KEY",
	}); err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	userCfg.Desktop.ProviderAccess = []string{"settings-provider"}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}
	t.Setenv("SHARED_SETTINGS_KEY", "from-project")
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(launch); err != nil {
		t.Fatalf("chdir launch: %v", err)
	}

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"project": {ID: "project", WorkspaceRoot: project}}
	app.activeTabID = "project"
	got := app.Settings()
	for _, p := range got.Providers {
		if p.Name != "settings-provider" {
			continue
		}
		if !p.KeySet || !strings.Contains(p.KeySource, "云智企办 credentials") {
			t.Fatalf("settings-provider key = set:%v source:%q, want 云智企办 credentials: %+v", p.KeySet, p.KeySource, p)
		}
		if env := os.Getenv("SHARED_SETTINGS_KEY"); env != "from-project" {
			t.Fatalf("Settings mutated SHARED_SETTINGS_KEY = %q, want existing project env", env)
		}
		return
	}
	t.Fatalf("settings provider missing from settings: %+v", got.Providers)
}

func TestSettingsSeedsMissingUserConfigFromLegacyProjectConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	project := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
default_model = "legacy-provider/legacy-model"

[desktop]
language = "zh"
layout_style = "workbench"
theme = "light"
theme_style = "glacier"
close_behavior = "quit"
status_bar_style = "text"
status_bar_items = ["model", "cache", "balance"]
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir project: %v", err)
	}

	app := NewApp()
	got := app.Settings()
	if got.ConfigPath != config.UserConfigPath() {
		t.Fatalf("Settings configPath = %q, want user config %q", got.ConfigPath, config.UserConfigPath())
	}
	if got.DefaultModel != "legacy-provider/legacy-model" || got.DesktopLanguage != "zh" || got.DesktopLayoutStyle != "workbench" || got.DesktopTheme != "light" || got.DesktopThemeStyle != "glacier" || got.CloseBehavior != "quit" || got.StatusBarStyle != "text" {
		t.Fatalf("Settings did not seed from legacy project config: %+v", got)
	}
	if want := []string{"model", "cache", "balance"}; !reflect.DeepEqual(got.StatusBarItems, want) {
		t.Fatalf("Settings did not seed status bar items from legacy project config: got %v want %v", got.StatusBarItems, want)
	}
	if _, err := os.Stat(config.UserConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("Settings() should not write user config before an edit, stat err = %v", err)
	}
	if err := app.SetDesktopLanguage("en"); err != nil {
		t.Fatalf("SetDesktopLanguage: %v", err)
	}
	userCfg := config.LoadForEdit(config.UserConfigPath())
	if userCfg.DesktopLanguage() != "en" || userCfg.DesktopLayoutStyle() != "workbench" || userCfg.DesktopTheme() != "light" || userCfg.DesktopThemeStyle() != "glacier" || userCfg.DesktopCloseBehavior() != "quit" || userCfg.DesktopStatusBarStyle() != "text" {
		t.Fatalf("saved user config did not preserve seeded desktop prefs: lang:%q layout:%q theme:%q style:%q close:%q status:%q", userCfg.DesktopLanguage(), userCfg.DesktopLayoutStyle(), userCfg.DesktopTheme(), userCfg.DesktopThemeStyle(), userCfg.DesktopCloseBehavior(), userCfg.DesktopStatusBarStyle())
	}
	if want := []string{"model", "cache", "balance"}; !reflect.DeepEqual(userCfg.DesktopStatusBarItems(), want) {
		t.Fatalf("saved user config did not preserve seeded status bar items: got %v want %v", userCfg.DesktopStatusBarItems(), want)
	}
}

func TestSettingsSubagentDefaultsRoundTrip(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "test-key")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "kwu/gpt-5.5"

[[providers]]
name = "kwu"
kind = "openai"
base_url = "http://192.168.10.231:8080/v1"
models = ["gpt-5.5", "gpt-5.4"]
default = "gpt-5.5"
api_key_env = "KWU_API_KEY"
reasoning_protocol = "openai"
supported_efforts = ["low", "medium", "high"]
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app := NewApp()
	if err := app.SetSubagentModel("kwu/gpt-5.4"); err != nil {
		t.Fatalf("SetSubagentModel: %v", err)
	}
	if err := app.SetSubagentEffort("high"); err != nil {
		t.Fatalf("SetSubagentEffort: %v", err)
	}

	got := app.Settings()
	if got.SubagentModel != "kwu/gpt-5.4" || got.SubagentEffort != "high" {
		t.Fatalf("subagent settings = model:%q effort:%q", got.SubagentModel, got.SubagentEffort)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.Agent.SubagentModel != "kwu/gpt-5.4" || cfg.Agent.SubagentEffort != "high" {
		t.Fatalf("saved config = model:%q effort:%q", cfg.Agent.SubagentModel, cfg.Agent.SubagentEffort)
	}
}

func TestSettingsSubscriptionSubagentEffortUsesAuthorizedCatalog(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	app := NewApp()
	actor := app.cloudActorContext()
	cfg := config.Default()
	cfg.DefaultModel = "kwu/deepseek-v4-flash"
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	cfg.Agent.SubagentModel = "kwu/deepseek-v4-flash"
	if err := cfg.SaveTo(app.activeProfilePathResolver().UserConfigPath()); err != nil {
		t.Fatalf("save subscription config: %v", err)
	}

	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		bindingActor: actor, bindingScope: scope, hasBinding: true,
	}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"deepseek-v4-flash"})
	app.setCurrentActorContext(actor)

	if err := app.SetSubagentEffort("high"); err != nil {
		t.Fatalf("SetSubagentEffort with authorized subscription catalog: %v", err)
	}
	saved := config.LoadForEditWithPaths(app.activeProfilePathResolver())
	if saved.Agent.SubagentEffort != "high" {
		t.Fatalf("saved subagent effort = %q, want high", saved.Agent.SubagentEffort)
	}
}

func TestSettingsSurfacesOfficialProviderTemplatesSeparately(t *testing.T) {
	isolateDesktopUserDirs(t)

	got := NewApp().Settings()
	providers := providerAccessSet(providerNamesFromView(got.Providers))
	official := providerAccessSet(providerNamesFromView(got.OfficialProviders))
	if providers["mimo-api"] {
		t.Fatalf("mimo-api should not be mixed into configured providers: %+v", got.Providers)
	}
	if !official["kwu"] || official["deepseek"] || official["mimo-api"] || official["mimo-token-plan"] {
		t.Fatalf("official providers = %+v, want only kwu", got.OfficialProviders)
	}
}

func TestSettingsRepairsLegacyOfficialProviderWithoutModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash"

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
api_key_env = "DEEPSEEK_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := NewApp().Settings()
	for _, p := range settingsProvidersIncludingOfficial(got) {
		if p.Name != "kwu" {
			continue
		}
		if !p.BuiltIn {
			t.Fatalf("kwu provider should be marked built-in for official endpoint: %+v", p)
		}
		if !p.Added || !p.Locked || p.KeySet || p.Configured || len(p.Models) != 0 || p.Default != "" || p.APIKeyEnv != "" {
			t.Fatalf("kwu provider = %+v, want locked signed-out points channel without models or local key", p)
		}
		if got.DefaultModel != "kwu/deepseek-v4-flash" {
			t.Fatalf("default_model = %q, want kwu/deepseek-v4-flash", got.DefaultModel)
		}
		return
	}
	t.Fatalf("settings providers missing kwu: providers=%+v official=%+v", got.Providers, got.OfficialProviders)
}

func TestSettingsTreatsReservedProviderNameWithExternalEndpointAsCustom(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek/deepseek-v4-Flash"

[desktop]
provider_access = ["deepseek"]

[[providers]]
name = "deepseek"
kind = "openai"
base_url = "https://opencode.ai/zen/go/v1"
models = ["deepseek-v4-Flash", "deepseek-v4-pro", "glm-5"]
default = "deepseek-v4-Flash"
api_key_env = "DEEPSEEK_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := NewApp().Settings()
	var custom *ProviderView
	for i := range got.Providers {
		if got.Providers[i].Name == "deepseek" {
			custom = &got.Providers[i]
			break
		}
	}
	if custom == nil {
		t.Fatalf("settings providers missing deepseek: %+v", got.Providers)
	}
	if custom.BuiltIn {
		t.Fatalf("external deepseek endpoint should be custom, got built-in provider: %+v", *custom)
	}
	if !custom.Added || !custom.KeySet || custom.BaseURL != "https://opencode.ai/zen/go/v1" {
		t.Fatalf("external deepseek provider = %+v, want added key-set custom opencode endpoint", *custom)
	}
	for _, p := range got.OfficialProviders {
		if p.Name == "deepseek" && p.Added {
			t.Fatalf("official DeepSeek template should not be marked added by external endpoint: %+v", p)
		}
	}
}

func TestSettingsInfersLegacyProviderAccessWhenMissing(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	setDesktopTestCredential(t, "KWU_API_KEY", "sk-test")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash/deepseek-v4-pro"

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
models = ["deepseek-v4-flash", "deepseek-v4-pro"]
default = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo-pro"
kind = "openai"
base_url = "https://token-plan-cn.xiaomimimo.com/v1"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := NewApp().Settings()
	providers := map[string]ProviderView{}
	for _, p := range settingsProvidersIncludingOfficial(got) {
		providers[p.Name] = p
	}
	if !providers["kwu"].Added || !providers["kwu"].Locked || providers["kwu"].KeySet {
		t.Fatalf("kwu provider = %+v, want permanent account-managed points channel", providers["kwu"])
	}
	if !providers["mimo-pro"].Added || !providers["mimo-pro"].KeySet || providers["mimo-pro"].BuiltIn {
		t.Fatalf("mimo-pro provider = %+v, want inferred custom key-set provider", providers["mimo-pro"])
	}
	if got.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("default_model = %q, want kwu/deepseek-v4-flash", got.DefaultModel)
	}
}

func TestSettingsDoesNotInferProviderAccessWhenExplicitlyEmpty(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash/deepseek-v4-flash"

[desktop]
provider_access = []

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
models = ["deepseek-v4-flash"]
default = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got := NewApp().Settings()
	for _, p := range got.Providers {
		if p.Added && p.Name != "kwu" {
			t.Fatalf("provider %+v should not be inferred as added when provider_access is explicit empty", p)
		}
	}
}

func TestSettingsInfersConfiguredBuiltInsWithoutConfigFile(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "test-key")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	got := NewApp().Settings()
	providers := map[string]ProviderView{}
	for _, p := range settingsProvidersIncludingOfficial(got) {
		providers[p.Name] = p
	}
	if !providers["kwu"].Added || !providers["kwu"].Locked || providers["kwu"].KeySet {
		t.Fatalf("kwu provider = %+v, want permanent channel that ignores local key configuration", providers["kwu"])
	}
	if _, ok := providers["mimo-token-plan"]; ok {
		t.Fatalf("mimo-token-plan should not be inferred from MIMO_API_KEY alone: %+v", providers["mimo-token-plan"])
	}
}

func TestSettingsDoesNotInferBuiltInsWithoutKeys(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("MIMO_API_KEY", "")

	got := NewApp().Settings()
	for _, p := range got.Providers {
		if p.Added && p.Name != "kwu" {
			t.Fatalf("provider %+v should not be inferred as added without a configured key", p)
		}
	}
}

func TestAddOfficialProviderAccessRejectsManagedChannelWithoutSavingKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_API_KEY", "")
	os.Unsetenv("KWU_API_KEY")

	_, err := NewApp().AddOfficialProviderAccess("kwu", "test-key")
	if err == nil || !strings.Contains(err.Error(), "managed by the signed-in account") {
		t.Fatalf("AddOfficialProviderAccess error = %v, want managed-channel rejection", err)
	}
	if data, readErr := os.ReadFile(config.UserCredentialsPath()); readErr == nil && (strings.Contains(string(data), "KWU_API_KEY") || strings.Contains(string(data), "KWU_API_KEY")) {
		t.Fatalf("official key should not be saved after rejected add access:\n%s", data)
	}
}

func TestSetProviderKeyDoesNotConfigureOfficialPointsChannel(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_API_KEY", "")
	os.Unsetenv("KWU_API_KEY")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "kwu/gpt-5.5"

[desktop]
provider_access = []

[[providers]]
name = "kwu"
kind = "openai"
base_url = "http://192.168.10.231:8080/v1"
models = ["gpt-5.5", "gpt-5.4"]
default = "gpt-5.5"
api_key_env = "KWU_API_KEY"
reasoning_protocol = "openai"
supported_efforts = ["low", "medium", "high"]
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := NewApp().SetProviderKey("KWU_API_KEY", "test-key"); err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	cfg := config.LoadForEdit(config.UserConfigPath())
	if !providerAccessSet(cfg.Desktop.ProviderAccess)["kwu"] {
		t.Fatalf("provider_access = %+v, want kwu restored", cfg.Desktop.ProviderAccess)
	}
	got := NewApp().Settings()
	for _, p := range settingsProvidersIncludingOfficial(got) {
		if p.Name == "kwu" {
			if !p.Added || !p.Locked || p.KeySet || p.Configured {
				t.Fatalf("kwu settings = %+v, want account-managed signed-out points channel", p)
			}
			return
		}
	}
	t.Fatalf("settings providers missing kwu: providers=%+v official=%+v", got.Providers, got.OfficialProviders)
}

func TestSetProviderKeyKeepsCustomAliasProviderAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("PROXY_DEEPSEEK_KEY", "")
	os.Unsetenv("PROXY_DEEPSEEK_KEY")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
[desktop]
provider_access = []

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://proxy.example/v1"
model = "deepseek-v4-flash"
api_key_env = "PROXY_DEEPSEEK_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := NewApp().SetProviderKey("PROXY_DEEPSEEK_KEY", "sk-test"); err != nil {
		t.Fatalf("SetProviderKey: %v", err)
	}
	cfg := config.LoadForEditWithoutCredentials(config.UserConfigPath())
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	if !access["deepseek-flash"] {
		t.Fatalf("provider_access = %+v, want custom alias deepseek-flash", cfg.Desktop.ProviderAccess)
	}
	if access["deepseek"] {
		t.Fatalf("provider_access = %+v, should not canonicalize custom proxy to deepseek", cfg.Desktop.ProviderAccess)
	}
}

func TestRemoveOfficialProviderAccessIsRejectedWithoutChangingCustomDefaults(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "deepseek-flash/deepseek-v4-pro"

[desktop]
provider_access = ["deepseek-flash", "mimo-pro"]

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
models = ["deepseek-v4-flash", "deepseek-v4-pro"]
default = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo-pro"
kind = "openai"
base_url = "https://token-plan-cn.xiaomimimo.com/v1"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	before, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := NewApp().RemoveProviderAccess("kwu"); err == nil || !strings.Contains(err.Error(), "cannot be removed") {
		t.Fatalf("RemoveProviderAccess error = %v, want permanent points-channel rejection", err)
	}
	if err := NewApp().DeleteProvider("kwu"); err == nil || !strings.Contains(err.Error(), "cannot be removed") {
		t.Fatalf("DeleteProvider error = %v, want permanent points-channel rejection", err)
	}
	after, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("config changed after rejected official provider removal")
	}
}

func TestModelsForTabOnlyListsProviderAccessWhenConfigured(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "sk-test")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek-flash/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek-flash", "mimo-pro"}
	cfg.Providers = []config.ProviderEntry{
		{
			Name:      "deepseek-flash",
			Kind:      "openai",
			BaseURL:   "https://api.deepseek.com",
			Models:    []string{"deepseek-v4-flash", "deepseek-v4-pro"},
			Default:   "deepseek-v4-flash",
			APIKeyEnv: "DEEPSEEK_API_KEY",
		},
		{
			Name:      "mimo-pro",
			Kind:      "openai",
			BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
			Models:    []string{"mimo-v2.5-pro", "mimo-v2.5"},
			Default:   "mimo-v2.5-pro",
			APIKeyEnv: "MIMO_API_KEY",
		},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	for _, want := range []string{"mimo-pro/mimo-v2.5-pro", "mimo-pro/mimo-v2.5"} {
		if !refs[want] {
			t.Fatalf("Models() refs = %+v, missing %s", models, want)
		}
	}
	for _, hidden := range []string{"kwu/gpt-5.4-mini", "kwu/gpt-5.4"} {
		if refs[hidden] {
			t.Fatalf("Models() refs = %+v, signed-out official model %s should be hidden", models, hidden)
		}
	}
	for _, hidden := range []string{
		"deepseek/deepseek-v4-flash",
		"deepseek-pro/deepseek-v4-pro",
		"mimo-flash/mimo-v2.5",
	} {
		if refs[hidden] {
			t.Fatalf("Models() refs = %+v, should not include hidden provider %s", models, hidden)
		}
	}
	if len(models) != 2 {
		t.Fatalf("Models() len = %d, want 2 custom models: %+v", len(models), models)
	}
}

func TestModelsForTabListsCustomMultiModelProviderWithoutMetadata(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "LOCAL_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "local/model-a"

[desktop]
provider_access = ["local"]

[[providers]]
name = "local"
kind = "openai"
base_url = "http://127.0.0.1:23333/v1"
models = ["model-a", "model-b"]
default = "model-a"
api_key_env = "LOCAL_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	for _, want := range []string{"local/model-a", "local/model-b"} {
		if !refs[want] {
			t.Fatalf("Models() refs = %+v, missing %s", models, want)
		}
	}
	if len(models) != 2 {
		t.Fatalf("Models() len = %d, want 2: %+v", len(models), models)
	}
}

func TestModelsForTabHidesNonChatModelsFromCustomProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "LOCAL_API_KEY", "sk-test")
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "local/gpt-5.5"

[desktop]
provider_access = ["local"]

[[providers]]
name = "local"
kind = "openai"
base_url = "http://192.168.10.231:8080/v1"
models = ["gpt-5.5", "gpt-image-2"]
default = "gpt-5.5"
api_key_env = "LOCAL_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	if !refs["local/gpt-5.5"] || refs["local/gpt-image-2"] {
		t.Fatalf("Models() refs = %+v, want only the chat-capable model", models)
	}
}

func TestModelsForTabListsKeylessCustomMultiModelProvider(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "local/model-a"

[desktop]
provider_access = ["local"]

[[providers]]
name = "local"
kind = "openai"
base_url = "http://127.0.0.1:23333/v1"
models = ["model-a", "model-b"]
default = "model-a"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	for _, want := range []string{"local/model-a", "local/model-b"} {
		if !refs[want] {
			t.Fatalf("Models() refs = %+v, missing %s", models, want)
		}
	}
}

func TestModelsForTabListsLoopbackCustomProviderWithMissingKeyEnv(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "local/model-a"

[desktop]
provider_access = ["local"]

[[providers]]
name = "local"
kind = "openai"
base_url = "http://127.0.0.1:23333/v1"
models = ["model-a", "model-b"]
default = "model-a"
api_key_env = "LOCAL_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	for _, want := range []string{"local/model-a", "local/model-b"} {
		if !refs[want] {
			t.Fatalf("Models() refs = %+v, missing %s", models, want)
		}
	}
}

func TestModelsForTabListsMimoAPIPaidAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "mimo-api/mimo-v2.5-pro"
	cfg.Desktop.ProviderAccess = []string{"mimo-api"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	models := NewApp().Models()
	refs := modelRefsFromView(models)
	for _, want := range []string{
		"mimo-api/mimo-v2.5-pro",
		"mimo-api/mimo-v2.5",
		"mimo-api/mimo-v2-omni",
	} {
		if !refs[want] {
			t.Fatalf("Models() refs = %+v, missing %s", models, want)
		}
	}
	if len(models) != 3 {
		t.Fatalf("Models() len = %d, want 3: %+v", len(models), models)
	}
}

func TestModelsForTabKeepsUserProvidersWithProjectConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "sk-test")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	userCfg := config.Default()
	userCfg.DefaultModel = "mimo-pro/mimo-v2.5-pro"
	userCfg.Desktop.ProviderAccess = []string{"deepseek-flash", "mimo-pro"}
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	projectRoot := t.TempDir()
	projectConfig := `default_model = "deepseek-flash/deepseek-v4-flash"

[desktop]
provider_access = ["deepseek-flash"]

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
model = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"
`
	if err := os.WriteFile(filepath.Join(projectRoot, "kwu.toml"), []byte(projectConfig), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	app := NewApp()
	tab := &WorkspaceTab{ID: "project", WorkspaceRoot: projectRoot, Ready: true}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.activeTabID = tab.ID

	models := app.ModelsForTab(tab.ID)
	refs := modelRefsFromView(models)
	for _, want := range []string{"mimo-pro/mimo-v2.5-pro"} {
		if !refs[want] {
			t.Fatalf("ModelsForTab refs = %+v, missing %s", models, want)
		}
	}
	if refs["kwu/gpt-5.4"] {
		t.Fatalf("ModelsForTab refs = %+v, signed-out official models should be hidden", models)
	}
}

func TestTabModelHelpersUseTabProfileCredentials(t *testing.T) {
	cases := []struct {
		name          string
		key           string
		defaultKey    bool
		profileKey    bool
		wantAvailable bool
	}{
		{
			name:          "profile key is enough when default has none",
			key:           "KWU_TEST_TAB_PROFILE_ONLY_KEY",
			profileKey:    true,
			wantAvailable: true,
		},
		{
			name:       "default key does not configure profile tab",
			key:        "KWU_TEST_TAB_DEFAULT_ONLY_KEY",
			defaultKey: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			t.Setenv(tc.key, "")
			os.Unsetenv(tc.key)

			projectRoot := t.TempDir()
			projectConfig := fmt.Sprintf(`
default_model = "profiled/model-a"

[desktop]
provider_access = ["profiled"]

[[providers]]
name = "profiled"
kind = "openai"
base_url = "https://example.invalid/v1"
models = ["model-a"]
default = "model-a"
api_key_env = %q
supported_efforts = ["low", "high"]
default_effort = "high"
`, tc.key)
			if err := os.WriteFile(filepath.Join(projectRoot, "kwu.toml"), []byte(projectConfig), 0o644); err != nil {
				t.Fatalf("write project config: %v", err)
			}
			if tc.defaultKey {
				setDesktopTestCredential(t, tc.key, "default-key")
			}

			actor := userctx.Default(projectRoot)
			actor.ProfileID = "work-profile"
			if tc.profileKey {
				if _, err := config.SetCredentialWithPaths(config.NewProfilePathResolver(actor), tc.key, "profile-key"); err != nil {
					t.Fatalf("SetCredentialWithPaths profile: %v", err)
				}
			}

			app := NewApp()
			tab := &WorkspaceTab{ID: "profile-tab", Scope: "project", WorkspaceRoot: projectRoot, Ready: true, actor: actor, model: "missing/model"}
			app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
			app.tabOrder = []string{tab.ID}
			app.activeTabID = tab.ID

			models := app.ModelsForTab(tab.ID)
			if got := modelRefsFromView(models)["profiled/model-a"]; got != tc.wantAvailable {
				t.Fatalf("ModelsForTab availability = %v, want %v; models = %+v", got, tc.wantAvailable, models)
			}

			resolved, fallback, err := app.resolvedModelForTab(tab)
			if tc.wantAvailable {
				if err != nil || !fallback || resolved != "profiled/model-a" {
					t.Fatalf("resolvedModelForTab = %q, fallback=%v, err=%v; want profiled/model-a fallback", resolved, fallback, err)
				}
				if effort := app.EffortForTab(tab.ID); !effort.Supported || effort.Default != "high" {
					t.Fatalf("EffortForTab = %+v, want supported with default high", effort)
				}
				return
			}
			if err == nil {
				t.Fatalf("resolvedModelForTab = %q, fallback=%v, want profile-scoped missing key error", resolved, fallback)
			}
			if effort := app.EffortForTab(tab.ID); effort.Supported || len(effort.Levels) != 0 {
				t.Fatalf("EffortForTab = %+v, want unsupported for keyless profile", effort)
			}
		})
	}
}

func TestSlashArgsEffortUsesActiveTabProfile(t *testing.T) {
	isolateDesktopUserDirs(t)
	projectRoot := t.TempDir()
	defaultConfig := `default_model = "default-provider/default-model"

[[providers]]
name = "default-provider"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "default-model"
supported_efforts = ["max"]
`
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(defaultConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	actor := userctx.Default(projectRoot)
	actor.ProfileID = "work-profile"
	paths := config.NewProfilePathResolver(actor)
	profileConfig := `default_model = "work-provider/work-model"

[[providers]]
name = "work-provider"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "work-model"
supported_efforts = ["low", "high"]
`
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(profileConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	ctrl := control.New(control.Options{Actor: actor, WorkspaceRoot: projectRoot})
	defer ctrl.Close()
	tab := &WorkspaceTab{
		ID:            "profile-tab",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		Ready:         true,
		Ctrl:          ctrl,
		actor:         actor,
		model:         "work-provider/work-model",
		disabledMCP:   map[string]ServerView{},
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	got := app.SlashArgs("/effort ")
	labels := map[string]bool{}
	for _, item := range got.Items {
		labels[item.Label] = true
	}
	for _, want := range []string{"auto", "low", "high"} {
		if !labels[want] {
			t.Fatalf("SlashArgs /effort missing %q: %+v", want, got.Items)
		}
	}
	if labels["max"] {
		t.Fatalf("SlashArgs /effort leaked default profile levels: %+v", got.Items)
	}
}

func TestDesktopSlashArgsOnlyAdvertisesSupportedMCPCommands(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	ctrl := control.New(control.Options{})
	defer ctrl.Close()
	app.setTestCtrl(ctrl, "test/model")

	got := app.SlashArgs("/mcp ")
	if len(got.Items) != 1 || got.Items[0].Label != "connect" {
		t.Fatalf("desktop /mcp suggestions = %+v, want only connect", got.Items)
	}
}

func TestSetModelForTabRejectsProviderOutsideAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "DEEPSEEK_API_KEY", "sk-test")
	setDesktopTestCredential(t, "MIMO_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek-flash/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek-flash"}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{Name: "other", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "other-model"})
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{ID: "tab_a", Scope: "global", Ready: true, model: "deepseek-flash/deepseek-v4-flash"}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	err := app.SetModelForTab(tab.ID, "other/other-model")
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("SetModelForTab hidden provider error = %v, want not available", err)
	}
}

func TestSetModelForTabAllowsImageModelSelection(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)

	const providerName = "vision-provider"
	cfg := config.Default()
	cfg.DefaultModel = providerName + "/text-model"
	cfg.Desktop.ProviderAccess = []string{providerName}
	cfg.Providers = []config.ProviderEntry{{
		Name:    providerName,
		Kind:    desktopActorProviderKind,
		BaseURL: "https://example.invalid/v1",
		Models:  []string{"text-model", "gpt-image-2"},
		Default: "text-model",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: providerName + "/text-model"})
	app.setTestCtrl(old, providerName+"/text-model")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetModelForTab("test", providerName+"/gpt-image-2"); err != nil {
		t.Fatalf("SetModelForTab(image): %v", err)
	}
	if got := app.activeTab().model; got != providerName+"/gpt-image-2" {
		t.Fatalf("tab model = %q, want %s/gpt-image-2", got, providerName)
	}
}

func TestSubmitToTabFastFailsImageModelSelection(t *testing.T) {
	isolateDesktopUserDirs(t)

	const providerName = "vision-provider"
	cfg := config.Default()
	cfg.DefaultModel = providerName + "/text-model"
	cfg.Desktop.ProviderAccess = []string{providerName}
	cfg.Providers = []config.ProviderEntry{{
		Name:    providerName,
		Kind:    "openai",
		BaseURL: "https://example.invalid/v1",
		Models:  []string{"text-model", "gpt-image-2"},
		Default: "text-model",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Label: providerName + "/gpt-image-2"}), providerName+"/gpt-image-2")
	defer app.activeCtrl().Close()

	err := app.SubmitToTab("test", "hello")
	if err == nil || !strings.Contains(err.Error(), "chat entry point") {
		t.Fatalf("SubmitToTab(image) error = %v, want chat entry point fast-fail", err)
	}
}

func TestSetModelForTabCanonicalizesSameEffectiveLegacyDeepSeekRef(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek/deepseek-v4-pro"
	cfg.Desktop.ProviderAccess = []string{"deepseek"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "deepseek",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Models:    []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		Default:   "deepseek-v4-flash",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	ctrl := control.New(control.Options{Label: "deepseek-v4-pro"})
	app.setTestCtrl(ctrl, "deepseek-pro")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetModelForTab("test", "deepseek-pro"); err != nil {
		t.Fatalf("SetModelForTab(deepseek-pro): %v", err)
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if tab.model != "kwu/deepseek-v4-flash" {
		t.Fatalf("tab model = %q, want canonical kwu/deepseek-v4-flash", tab.model)
	}
	saved := loadTabsFile()
	if len(saved.Tabs) != 1 || saved.Tabs[0].Model != "kwu/deepseek-v4-flash" {
		t.Fatalf("saved tabs = %+v, want canonical model", saved.Tabs)
	}
}

func TestSetModelForTabPreservesActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "old-model"},
		{Name: "new", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "new-model"},
	}
	if err := cfg.SaveTo(filepath.Join(root, "kwu.toml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	old := control.New(control.Options{
		Label: "old/old-model",
		Actor: userctx.ActorContext{
			ActorID:    "actor-desktop",
			ProfileID:  "profile-desktop",
			AuthMethod: "oauth",
		},
	})
	tab := &WorkspaceTab{ID: "tab_a", WorkspaceRoot: root, Ready: true, model: "old/old-model", Ctrl: old}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	defer tab.Ctrl.Close()

	actor := tab.Ctrl.ActorContext()
	if actor.ActorID != "actor-desktop" || actor.ProfileID != "profile-desktop" || actor.AuthMethod != "oauth" {
		t.Fatalf("switched ActorContext() = %+v", actor)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, root)
	}
}

func TestSetModelForTabRefreshesCarriedSystemPrompt(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	app.ctx = context.Background()
	setDesktopTestCredential(t, "OLD_MODEL_KEY", "sk-test")
	setDesktopTestCredential(t, "NEW_MODEL_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old", "new"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "old-model", APIKeyEnv: "OLD_MODEL_KEY"},
		{Name: "new", Kind: "openai", BaseURL: "https://example.invalid/v1", Model: "new-model", APIKeyEnv: "NEW_MODEL_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	if err := os.MkdirAll(config.MemoryUserDir(), 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	const freshRule = "Fresh global AGENTS rule for model switch"
	if err := os.WriteFile(filepath.Join(config.MemoryUserDir(), "AGENTS.md"), []byte(freshRule), 0o644); err != nil {
		t.Fatalf("write global AGENTS.md: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	oldSession := agent.NewSession("old system prompt without memory\n\n" + config.ModelIdentityPolicy("old-model"))
	oldSession.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	oldExec := agent.New(nil, nil, oldSession, agent.Options{}, event.Discard)
	oldPath := filepath.Join(dir, "old.jsonl")
	oldCtrl := control.New(control.Options{Executor: oldExec, SessionDir: dir, SessionPath: oldPath, Label: "old"})

	tab := &WorkspaceTab{
		ID:            "tab_a",
		Scope:         "global",
		Scene:         scene.Office,
		SceneState:    scene.Locked,
		Ready:         true,
		model:         "old/old-model",
		billingSource: string(modelbilling.BillingNewAPIByok),
		Ctrl:          oldCtrl,
		sink:          &tabEventSink{tabID: "tab_a", app: app},
		disabledMCP:   map[string]ServerView{},
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	t.Cleanup(func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	})

	if err := app.SetModelForTab(tab.ID, "new/new-model"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	history := tab.Ctrl.History()
	if len(history) < 2 {
		t.Fatalf("history length = %d, want system + user", len(history))
	}
	if history[0].Role != provider.RoleSystem {
		t.Fatalf("first message role = %s, want system", history[0].Role)
	}
	if !strings.Contains(history[0].Content, freshRule) {
		t.Fatalf("refreshed system prompt missing global AGENTS rule:\n%s", history[0].Content)
	}
	if !strings.Contains(history[0].Content, config.ModelIdentityPolicy("new-model")) {
		t.Fatalf("refreshed system prompt missing new model identity:\n%s", history[0].Content)
	}
	if strings.Contains(history[0].Content, config.ModelIdentityPolicy("old-model")) {
		t.Fatalf("refreshed system prompt retained old model identity:\n%s", history[0].Content)
	}
	if history[1].Role != provider.RoleUser || history[1].Content != "hello" {
		t.Fatalf("carried user message changed: %+v", history[1])
	}
}

func TestSetEffortForTabPreservesActorContext(t *testing.T) {
	registerDesktopActorProvider()
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	cfg := config.Default()
	cfg.DefaultModel = "old/old-model"
	cfg.Desktop.ProviderAccess = []string{"old"}
	cfg.Providers = []config.ProviderEntry{
		{Name: "old", Kind: desktopActorProviderKind, BaseURL: "https://example.invalid", Model: "old-model", SupportedEfforts: []string{"low", "high"}},
	}
	if err := cfg.SaveTo(filepath.Join(root, "kwu.toml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	old := control.New(control.Options{
		Label: "old/old-model",
		Actor: userctx.ActorContext{
			ActorID:    "actor-effort",
			ProfileID:  "profile-effort",
			AuthMethod: "oauth",
		},
	})
	tab := &WorkspaceTab{ID: "tab_a", WorkspaceRoot: root, Ready: true, model: "old/old-model", Ctrl: old}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.SetEffortForTab(tab.ID, "high"); err != nil {
		t.Fatalf("SetEffortForTab: %v", err)
	}
	defer tab.Ctrl.Close()

	actor := tab.Ctrl.ActorContext()
	if actor.ActorID != "actor-effort" || actor.ProfileID != "profile-effort" || actor.AuthMethod != "oauth" {
		t.Fatalf("switched ActorContext() = %+v", actor)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, root)
	}
}

func TestSetTokenModeForTabPreservesActorContext(t *testing.T) {
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
	old := control.New(control.Options{
		Label: "old/old-model",
		Actor: userctx.ActorContext{
			ActorID:    "actor-token",
			ProfileID:  "profile-token",
			AuthMethod: "oauth",
			RemoteIdentity: &userctx.RemoteIdentity{
				Provider: "lark",
				Domain:   "example.invalid",
				UserID:   "remote-user",
				ChatID:   "remote-chat",
				ThreadID: "remote-thread",
			},
			ParentActorID: "parent-token",
		},
	})
	tab := &WorkspaceTab{ID: "tab_a", WorkspaceRoot: root, Ready: true, model: "old/old-model", tokenMode: boot.TokenModeFull, Ctrl: old}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.SetTokenModeForTab(tab.ID, boot.TokenModeEconomy); err != nil {
		t.Fatalf("SetTokenModeForTab: %v", err)
	}
	defer tab.Ctrl.Close()

	actor := tab.Ctrl.ActorContext()
	if actor.ActorID != "actor-token" || actor.ProfileID != "profile-token" || actor.AuthMethod != "oauth" {
		t.Fatalf("rebuilt ActorContext() = %+v", actor)
	}
	if actor.ParentActorID != "parent-token" {
		t.Fatalf("ParentActorID = %q, want parent-token", actor.ParentActorID)
	}
	if actor.RemoteIdentity == nil {
		t.Fatalf("RemoteIdentity = nil")
	}
	remote := *actor.RemoteIdentity
	if remote.Provider != "lark" || remote.Domain != "example.invalid" || remote.UserID != "remote-user" || remote.ChatID != "remote-chat" || remote.ThreadID != "remote-thread" {
		t.Fatalf("RemoteIdentity = %+v", remote)
	}
	if filepath.Clean(actor.WorkspaceRoot) != filepath.Clean(root) {
		t.Fatalf("WorkspaceRoot = %q, want %q", actor.WorkspaceRoot, root)
	}
	if got := currentTabTokenMode(tab); got != boot.TokenModeEconomy {
		t.Fatalf("token mode = %q, want %q", got, boot.TokenModeEconomy)
	}
}

func TestSetDefaultModelRejectsProviderWithoutKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("MIMO_API_KEY", "")

	cfg := config.Default()
	cfg.Desktop.ProviderAccess = []string{"mimo-api"}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	tab := &WorkspaceTab{ID: "tab_a", Scope: "global", Ready: true, model: "deepseek-flash/deepseek-v4-flash"}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	err := app.SetDefaultModel("mimo-api/mimo-v2.5-pro")
	if err == nil || !strings.Contains(err.Error(), "has no key") {
		t.Fatalf("SetDefaultModel no-key error = %v, want has no key", err)
	}
	if tab.model != "deepseek-flash/deepseek-v4-flash" {
		t.Fatalf("tab model after failed default change = %q, want previous", tab.model)
	}
}

func TestSaveProviderPersistsReasoningProtocol(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if err := app.SaveProvider(ProviderView{
		Name:              "deepseek-proxy",
		Kind:              "openai",
		BaseURL:           "https://proxy.example.com/v1",
		Models:            []string{"deepseek-v4-flash"},
		Default:           "deepseek-v4-flash",
		APIKeyEnv:         "DEEPSEEK_PROXY_KEY",
		ReasoningProtocol: "none",
		SupportedEfforts:  []string{"high", "max"},
		DefaultEffort:     "max",
	}); err != nil {
		t.Fatalf("SaveProvider: %v", err)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	got, ok := cfg.Provider("deepseek-proxy")
	if !ok {
		t.Fatal("saved provider not found")
	}
	if got.ReasoningProtocol != "none" || got.DefaultEffort != "max" {
		t.Fatalf("saved provider = %+v, want reasoning_protocol none and default_effort max", got)
	}

	view := app.Settings()
	for _, p := range view.Providers {
		if p.Name == "deepseek-proxy" {
			if p.ReasoningProtocol != "none" {
				t.Fatalf("settings reasoningProtocol = %q, want none", p.ReasoningProtocol)
			}
			return
		}
	}
	t.Fatalf("Settings() missing saved provider: %+v", view.Providers)
}

func TestDeleteProviderMigratesConfigAndOpenTabs(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_TEST_KEY", "sk-test")
	app := NewApp()
	if _, err := config.SetCredentialWithPaths(app.activeProfilePathResolver(), "KWU_TEST_KEY", "sk-test"); err != nil {
		t.Fatalf("set active profile credential: %v", err)
	}

	cfg := config.Default()
	cfg.DefaultModel = "prov-a/model-a2"
	cfg.Providers = []config.ProviderEntry{
		{Name: "prov-a", Kind: "openai", BaseURL: "https://a.example.com", Model: "model-a1", Models: []string{"model-a1", "model-a2"}, APIKeyEnv: "KWU_TEST_KEY"},
		{Name: "prov-b", Kind: "openai", BaseURL: "https://b.example.com", Model: "model-b1", APIKeyEnv: "KWU_TEST_KEY"},
	}
	cfg.Agent.PlannerModel = "prov-a"
	cfg.Desktop.ProviderAccess = []string{"prov-a", "prov-b"}
	configPath := app.activeProfilePathResolver().UserConfigPath()
	if err := cfg.SaveToScope(configPath, config.RenderScopeUser); err != nil {
		t.Fatalf("save config: %v", err)
	}

	ctrl := control.New(control.Options{Label: "old"})
	defer ctrl.Close()
	tab := &WorkspaceTab{ID: "tab_a", Scope: "global", Ctrl: ctrl, Label: "prov-a/model-a1", Ready: true, model: "prov-a/model-a1"}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.DeleteProvider("prov-a"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}

	got := config.LoadForEdit(configPath)
	if _, ok := got.Provider("prov-a"); ok {
		t.Fatal("prov-a should be removed")
	}
	if got.DefaultModel != "prov-b" || got.Agent.PlannerModel != "prov-b" {
		t.Fatalf("model refs after delete = default:%q planner:%q, want prov-b", got.DefaultModel, got.Agent.PlannerModel)
	}
	if providerAccessSet(got.Desktop.ProviderAccess)["prov-a"] {
		t.Fatalf("provider access still contains prov-a: %+v", got.Desktop.ProviderAccess)
	}
	if tab.model != "prov-b/model-b1" || tab.Label != "prov-b/model-b1" {
		t.Fatalf("tab model after delete = model:%q label:%q, want prov-b/model-b1", tab.model, tab.Label)
	}
	if tab.Ctrl != nil {
		t.Fatal("tab controller should be closed and cleared when retargeted without a running app context")
	}
}

func TestDeleteProviderRetargetsBlankTabToCurrentDefaultRoute(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_TEST_KEY", "sk-test")

	app := NewApp()
	configPath := app.activeProfilePathResolver().UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`
default_model = "kwu/deepseek-v4-flash"

[desktop]
provider_access = ["gpt", "other", "kwu"]
default_billing_source = "subscription_pool"

[[providers]]
name = "gpt"
kind = "openai"
base_url = "http://127.0.0.1:18080/v1"
model = "gpt-5.6"
api_key_env = "KWU_TEST_KEY"

[[providers]]
name = "other"
kind = "openai"
base_url = "http://127.0.0.1:18081/v1"
model = "other-chat"
api_key_env = "KWU_TEST_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctrl := control.New(control.Options{Label: "gpt/gpt-5.6"})
	defer ctrl.Close()
	tab := &WorkspaceTab{
		ID:            "blank-gpt",
		Scope:         "global",
		Ctrl:          ctrl,
		Label:         "gpt/gpt-5.6",
		Ready:         true,
		model:         "gpt/gpt-5.6",
		billingSource: string(modelbilling.BillingNewAPIByok),
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if loaded, path, err := app.loadDesktopUserConfigForEdit(); err != nil {
		t.Fatalf("load desktop config for edit: %v", err)
	} else if _, ok := loaded.Provider("gpt"); !ok {
		t.Fatalf("desktop edit config %q missing gpt provider: %+v", path, loaded.Providers)
	}
	if err := app.DeleteProvider("gpt"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}

	if tab.model != "kwu/deepseek-v4-flash" || tab.Label != "kwu/deepseek-v4-flash" {
		t.Fatalf("blank tab model after delete = model:%q label:%q, want current default", tab.model, tab.Label)
	}
	if tab.billingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("blank tab billing source after delete = %q, want %q", tab.billingSource, modelbilling.BillingSubscriptionPool)
	}
	if tab.Ctrl != nil {
		t.Fatal("blank tab controller should be closed and cleared when retargeted without a running app context")
	}
}

func TestDeleteProviderRejectsRunningAffectedTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_TEST_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "prov-a/model-a1"
	cfg.Providers = []config.ProviderEntry{
		{Name: "prov-a", Kind: "openai", BaseURL: "https://a.example.com", Model: "model-a1", APIKeyEnv: "KWU_TEST_KEY"},
		{Name: "prov-b", Kind: "openai", BaseURL: "https://b.example.com", Model: "model-b1", APIKeyEnv: "KWU_TEST_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Runner: runner}), "prov-a/model-a1")
	ctrl := app.activeCtrl()
	ctrl.Submit("work")
	<-runner.started

	err := app.DeleteProvider("prov-a")
	if err == nil || !strings.Contains(err.Error(), "finish or cancel") {
		t.Fatalf("DeleteProvider while running error = %v, want finish/cancel guard", err)
	}
	if _, ok := config.LoadForEdit(config.UserConfigPath()).Provider("prov-a"); !ok {
		t.Fatal("provider should remain after rejected deletion")
	}

	close(runner.release)
	waitNotRunning(t, ctrl)
	ctrl.Close()
}

func TestDeleteProviderRejectsAffectedBackgroundJobs(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_TEST_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "prov-a/model-a1"
	cfg.Providers = []config.ProviderEntry{
		{Name: "prov-a", Kind: "openai", BaseURL: "https://a.example.com", Model: "model-a1", APIKeyEnv: "KWU_TEST_KEY"},
		{Name: "prov-b", Kind: "openai", BaseURL: "https://b.example.com", Model: "model-b1", APIKeyEnv: "KWU_TEST_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "provider-job.jsonl")
	jm := jobs.NewManager(event.Discard)
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	defer ctrl.Close()
	app := NewApp()
	app.setTestCtrl(ctrl, "prov-a/model-a1")
	jm.StartForSession(agent.BranchID(path), "bash", "provider job", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	err := app.DeleteProvider("prov-a")
	if err == nil || !strings.Contains(err.Error(), "active work") {
		t.Fatalf("DeleteProvider with background job error = %v, want active-work guard", err)
	}
	if _, ok := config.LoadForEdit(config.UserConfigPath()).Provider("prov-a"); !ok {
		t.Fatal("provider should remain after rejected deletion")
	}
}

func TestDeleteProviderRejectsUnaffectedBackgroundJobsBeforeSavingConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_TEST_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "prov-b/model-b1"
	cfg.Providers = []config.ProviderEntry{
		{Name: "prov-a", Kind: "openai", BaseURL: "https://a.example.com", Model: "model-a1", APIKeyEnv: "KWU_TEST_KEY"},
		{Name: "prov-b", Kind: "openai", BaseURL: "https://b.example.com", Model: "model-b1", APIKeyEnv: "KWU_TEST_KEY"},
	}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.setTestCtrl(newBackgroundJobController(t, "provider-unaffected-job"), "prov-b/model-b1")

	err := app.DeleteProvider("prov-a")
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("DeleteProvider with unaffected background job error = %v, want active-work guard", err)
	}
	if _, ok := config.LoadForEdit(config.UserConfigPath()).Provider("prov-a"); !ok {
		t.Fatal("unaffected provider should remain after rejected deletion")
	}
}

func TestRemoveOfficialProviderAccessRejectsBeforeBackgroundJobChecks(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "mimo-pro/mimo-v2.5-pro"

[desktop]
provider_access = ["deepseek-flash", "mimo-pro"]

[[providers]]
name = "deepseek-flash"
kind = "openai"
base_url = "https://api.deepseek.com"
models = ["deepseek-v4-flash", "deepseek-v4-pro"]
default = "deepseek-v4-flash"
api_key_env = "DEEPSEEK_API_KEY"

[[providers]]
name = "mimo-pro"
kind = "openai"
base_url = "https://token-plan-cn.xiaomimimo.com/v1"
model = "mimo-v2.5-pro"
api_key_env = "MIMO_API_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.setTestCtrl(newBackgroundJobController(t, "provider-access-unaffected-job"), "mimo-token-plan/mimo-v2.5-pro")

	before, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	err = app.RemoveProviderAccess("kwu")
	if err == nil || !strings.Contains(err.Error(), "cannot be removed") {
		t.Fatalf("RemoveProviderAccess error = %v, want permanent points-channel rejection", err)
	}
	after, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("config changed after rejected official provider removal")
	}
}

func TestConnectKeyRejectsBackgroundJobsBeforeSavingKey(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_API_KEY", "")
	os.Unsetenv("KWU_API_KEY")

	app := NewApp()
	app.ctx = context.Background()
	app.setTestCtrl(newBackgroundJobController(t, "connect-key-job"), testKWUDefaultModelRef)

	_, err := app.ConnectKey("sk-test")
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("ConnectKey with background job error = %v, want active-work guard", err)
	}
	if data, readErr := os.ReadFile(config.UserCredentialsPath()); readErr == nil && strings.Contains(string(data), "KWU_API_KEY") {
		t.Fatalf("onboarding key should not be saved after rejected connect:\n%s", data)
	}
}

func TestOnboardingModelsURLUsesOfficialAirClaudeEndpoint(t *testing.T) {
	if onboardingModelsURL != config.OfficialProviderModelsURL {
		t.Fatalf("onboardingModelsURL = %q, want %q", onboardingModelsURL, config.OfficialProviderModelsURL)
	}
}

func TestConnectKeyFetchesKWUModels(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_API_KEY", "")
	os.Unsetenv("KWU_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"gpt-image-1"},{"id":"gpt-5.5"}],"object":"list"}`)
	}))
	defer srv.Close()
	oldURL := onboardingModelsURL
	onboardingModelsURL = srv.URL + "/v1/models"
	t.Cleanup(func() { onboardingModelsURL = oldURL })

	app := NewApp()
	app.ctx = context.Background()
	got, err := app.ConnectKey("test-key")
	if err != nil {
		t.Fatalf("ConnectKey: %v", err)
	}
	if want := []string{"gpt-image-1", "gpt-5.5"}; !reflect.DeepEqual(got.Models, want) {
		t.Fatalf("Models = %#v, want %#v", got.Models, want)
	}
	if got.DefaultModel != "gpt-5.5" {
		t.Fatalf("DefaultModel = %q, want gpt-5.5", got.DefaultModel)
	}
	if got.UsedFallback {
		t.Fatalf("UsedFallback = true, want false")
	}
}

func TestConnectKeyReturnsErrorWhenModelsUnavailable(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_API_KEY", "")
	os.Unsetenv("KWU_API_KEY")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	}))
	defer srv.Close()
	oldURL := onboardingModelsURL
	onboardingModelsURL = srv.URL + "/v1/models"
	t.Cleanup(func() { onboardingModelsURL = oldURL })

	app := NewApp()
	app.ctx = context.Background()
	if _, err := app.ConnectKey("test-key"); err == nil || !strings.Contains(err.Error(), "fetch models") {
		t.Fatalf("ConnectKey error = %v, want fetch models error", err)
	}
	if data, readErr := os.ReadFile(config.UserCredentialsPath()); readErr == nil && strings.Contains(string(data), "KWU_API_KEY") {
		t.Fatalf("onboarding key should not be saved when model discovery fails:\n%s", data)
	}
}

func TestSafeExportFilenameUsesKWUFallback(t *testing.T) {
	if got := safeExportFilename(" \t "); got != "云智企办-会话.md" {
		t.Fatalf("safeExportFilename blank = %q, want 云智企办-会话.md", got)
	}
}

func TestCompleteOnboardingModelSelectionWritesKWUDefault(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	if err := app.CompleteOnboardingModelSelection(" gpt-image-1 ", []string{" gpt-5.5 ", "gpt-image-1", "gpt-5.5"}); err != nil {
		t.Fatalf("CompleteOnboardingModelSelection: %v", err)
	}

	body, err := os.ReadFile(config.UserConfigPath())
	if err != nil {
		t.Fatalf("read onboarding config: %v", err)
	}
	bodyText := string(body)
	if !strings.Contains(bodyText, `default_model = "kwu/gpt-5.5"`) || !strings.Contains(bodyText, `provider_access = ["kwu"]`) {
		t.Fatalf("onboarding root selections were not persisted:\n%s", bodyText)
	}
	if strings.Contains(bodyText, config.OfficialProviderBaseURL) || strings.Contains(bodyText, `[[providers]]`) {
		t.Fatalf("onboarding persisted the built-in provider:\n%s", bodyText)
	}

	cfg := config.LoadForEdit(config.UserConfigPath())
	if cfg.DefaultModel != "kwu/gpt-5.5" {
		t.Fatalf("DefaultModel = %q, want kwu/gpt-5.5", cfg.DefaultModel)
	}
	p, ok := cfg.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing")
	}
	if p.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("APIKeyEnv = %q, want KWU_API_KEY", p.APIKeyEnv)
	}
	applyDesktopAirClaudeSubscriptionCatalogModels(cfg, []string{"gpt-5.5", "gpt-image-1"})
	resolved, ok := cfg.ResolveModel(cfg.DefaultModel)
	if !ok || resolved.Model != "gpt-5.5" {
		t.Fatalf("selected onboarding model did not resolve from runtime catalog: %+v/%v", resolved, ok)
	}
	if p.Default != "gpt-5.5" {
		t.Fatalf("provider default = %q, want gpt-5.5", p.Default)
	}
	if want := []string{"gpt-5.5", "gpt-image-1"}; !reflect.DeepEqual(p.ModelList(), want) {
		t.Fatalf("ModelList = %#v, want %#v", p.ModelList(), want)
	}
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	if !access["kwu"] {
		t.Fatalf("provider_access = %#v, want kwu", cfg.Desktop.ProviderAccess)
	}
}

func TestMigrateDesktopPreferencesDoesNotOverwriteExistingConfig(t *testing.T) {
	isolateDesktopUserDirs(t)

	userCfg := config.LoadForEdit(config.UserConfigPath())
	if err := userCfg.SetDesktopLanguage("en"); err != nil {
		t.Fatalf("set desktop language: %v", err)
	}
	if err := userCfg.SetDesktopLayoutStyle("workbench"); err != nil {
		t.Fatalf("set desktop layout style: %v", err)
	}
	if err := userCfg.SetDesktopAppearance("dark", "graphite"); err != nil {
		t.Fatalf("set desktop appearance: %v", err)
	}
	userCfg.Desktop.ThemeStyle = "graphite"
	if err := userCfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save user config: %v", err)
	}

	if err := NewApp().MigrateDesktopPreferences("zh", "light", "glacier"); err != nil {
		t.Fatalf("migrate desktop preferences: %v", err)
	}

	got := config.LoadForEdit(config.UserConfigPath())
	if got.DesktopLanguage() != "en" || got.DesktopLayoutStyle() != "workbench" || got.DesktopTheme() != "dark" || got.DesktopThemeStyle() != "" {
		t.Fatalf("desktop prefs after migration = lang:%q layout:%q theme:%q style:%q, want existing values except cleared style", got.DesktopLanguage(), got.DesktopLayoutStyle(), got.DesktopTheme(), got.DesktopThemeStyle())
	}
}

func TestSetDesktopAppearanceIgnoresLegacyStyleAndPersistsTheme(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	paths := app.activeProfilePathResolver()
	cfg := config.Default()
	cfg.Desktop.ThemeStyle = "graphite"
	if err := cfg.SaveToScope(paths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save legacy desktop appearance: %v", err)
	}

	if err := app.SetDesktopAppearance("light", "unsupported-legacy-style"); err != nil {
		t.Fatalf("SetDesktopAppearance: %v", err)
	}
	saved := config.LoadForEditWithPaths(paths)
	if saved.DesktopTheme() != "light" || saved.Desktop.ThemeStyle != "" {
		t.Fatalf("saved desktop appearance = theme:%q style:%q, want light and empty", saved.DesktopTheme(), saved.Desktop.ThemeStyle)
	}
}

func TestSetEffortRebuildsController(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, testKWUDefaultModelRef)
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetEffort("high"); err != nil {
		t.Fatalf("SetEffort(high): %v", err)
	}
	if c := app.activeCtrl(); c == nil {
		t.Fatal("SetEffort should leave a rebuilt controller")
	}
	if c := app.activeCtrl(); c == old {
		t.Fatal("SetEffort should rebuild the active controller so the provider sees the new effort")
	}
	if got := app.Effort().Current; got != "high" {
		t.Fatalf("Effort current = %q, want high", got)
	}
}

func TestSetEffortMigratesStaleOfficialDeepSeekTabModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "deepseek",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Model:     "glm-5",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "deepseek-flash/deepseek-v4-flash")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetEffort("high"); err != nil {
		t.Fatalf("SetEffort(high): %v", err)
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if tab.model != testKWUDefaultModelRef {
		t.Fatalf("tab model = %q, want migrated official ref", tab.model)
	}
}

func TestSetTokenModeRebuildsController(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, testKWUDefaultModelRef)
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetTokenMode("economy"); err != nil {
		t.Fatalf("SetTokenMode(economy): %v", err)
	}
	if c := app.activeCtrl(); c == nil {
		t.Fatal("SetTokenMode should leave a rebuilt controller")
	}
	if c := app.activeCtrl(); c == old {
		t.Fatal("SetTokenMode should rebuild the active controller so the provider sees the new tool profile")
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if got := currentTabTokenMode(tab); got != "economy" {
		t.Fatalf("token mode = %q, want economy", got)
	}
	if got := app.Meta().TokenMode; got != "economy" {
		t.Fatalf("Meta token mode = %q, want economy", got)
	}
	saved := loadTabsFile()
	if len(saved.Tabs) != 1 || saved.Tabs[0].TokenMode != "economy" {
		t.Fatalf("saved tabs = %+v, want economy token mode", saved.Tabs)
	}
}

func TestSetTokenModeMigratesStaleOfficialDeepSeekTabModel(t *testing.T) {
	isolateDesktopUserDirs(t)
	setDesktopTestCredential(t, "KWU_API_KEY", "sk-test")

	cfg := config.Default()
	cfg.DefaultModel = "deepseek/deepseek-v4-flash"
	cfg.Desktop.ProviderAccess = []string{"deepseek"}
	cfg.Providers = []config.ProviderEntry{{
		Name:      "deepseek",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Model:     "glm-5",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "deepseek-flash/deepseek-v4-flash")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetTokenMode("economy"); err != nil {
		t.Fatalf("SetTokenMode(economy): %v", err)
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if tab.model != testKWUDefaultModelRef {
		t.Fatalf("tab model = %q, want migrated official ref", tab.model)
	}
	if got := currentTabTokenMode(tab); got != "economy" {
		t.Fatalf("token mode = %q, want economy", got)
	}
}

func TestSetTokenModeKeepsControllerWhenRebuildFails(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("MIMO_API_KEY", "")

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	old := control.New(control.Options{Label: "old-controller"})
	app.setTestCtrl(old, "missing-token-mode-model")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	err := app.SetTokenMode("economy")
	if err == nil {
		t.Fatal("SetTokenMode(economy) with an unknown model should fail")
	}
	if c := app.activeCtrl(); c != old {
		t.Fatalf("SetTokenMode failure replaced controller: got %p want %p", c, old)
	}
	tab := app.activeTab()
	if tab == nil {
		t.Fatal("active tab missing")
	}
	if got := currentTabTokenMode(tab); got != "full" {
		t.Fatalf("token mode after failed rebuild = %q, want full", got)
	}
	if got := app.Meta().TokenMode; got != "full" {
		t.Fatalf("Meta token mode after failed rebuild = %q, want full", got)
	}
}

func TestSetEffortRejectsRunningTurn(t *testing.T) {
	isolateDesktopUserDirs(t)

	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Runner: runner}), "")
	app.activeCtrl().Submit("work")
	<-runner.started

	err := app.SetEffort("max")
	if err == nil || !strings.Contains(err.Error(), "finish or cancel") {
		t.Fatalf("SetEffort while running error = %v, want finish/cancel guard", err)
	}

	close(runner.release)
	waitNotRunning(t, app.activeCtrl())
}

func TestSetTokenModeRejectsRunningTurn(t *testing.T) {
	isolateDesktopUserDirs(t)

	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Runner: runner}), "")
	app.activeCtrl().Submit("work")
	<-runner.started

	err := app.SetTokenMode("economy")
	if err == nil || !strings.Contains(err.Error(), "finish or cancel") {
		t.Fatalf("SetTokenMode while running error = %v, want finish/cancel guard", err)
	}

	close(runner.release)
	waitNotRunning(t, app.activeCtrl())
}

func TestSetTokenModeRejectsBackgroundJobs(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "jobs.jsonl")
	jm := jobs.NewManager(event.Discard)
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	defer ctrl.Close()
	app := NewApp()
	app.setTestCtrl(ctrl, "")

	release := make(chan struct{})
	jm.StartForSession(agent.BranchID(path), "bash", "long job", func(ctx context.Context, _ io.Writer) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
			return "", nil
		}
	})
	defer close(release)

	err := app.SetTokenMode("economy")
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("SetTokenMode with background job error = %v, want background-job guard", err)
	}
}

func TestSettingsRebuildRejectsBackgroundJobs(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "settings-job.jsonl")
	jm := jobs.NewManager(event.Discard)
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	defer ctrl.Close()
	app := NewApp()
	app.ctx = context.Background()
	app.setTestCtrl(ctrl, "deepseek-flash/deepseek-v4-flash")

	jm.StartForSession(agent.BranchID(path), "bash", "settings job", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	err := app.SetSandbox("enforce", true, "", nil, "")
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("SetSandbox with background job error = %v, want background-job guard", err)
	}
}

func TestClearSessionCancelsRunningRuntimeAndKeepsTopic(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "clear-running.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	oldCtrl := control.New(control.Options{Runner: runner, SessionDir: dir, SessionPath: path, Label: "test"})
	app := NewApp()
	app.projectTreeChangedHook = func() {}
	app.setTestCtrl(oldCtrl, testKWUDefaultModelRef)
	app.tabRuntimes["test"].TopicID = "topic_clear"
	app.tabRuntimes["test"].TopicTitle = "Clear topic"
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	oldCtrl.Submit("work")
	<-runner.started
	if err := app.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	waitNotRunning(t, oldCtrl)
	tab := app.activeTab()
	if tab == nil || tab.Ctrl == nil {
		t.Fatalf("active tab/controller missing after clear")
	}
	if tab.Ctrl == oldCtrl {
		t.Fatalf("clear should replace the active controller after cancelling old work")
	}
	if tab.TopicID != "topic_clear" || tab.TopicTitle != "Clear topic" {
		t.Fatalf("clear changed topic identity: %+v", tab)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("old cleared session artifacts should be removed, stat err = %v", err)
	}
	if got := tab.currentSessionPath(); got == "" || got == path {
		t.Fatalf("new session path = %q, want fresh path", got)
	}
}

func TestClearSessionActiveRuntimePreservesActorContext(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "clear-actor.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	root := t.TempDir()
	actor := userctx.ActorContext{
		ActorID:       "actor-clear",
		ProfileID:     "profile-clear",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Clean(root),
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: "lark",
			Domain:   "example.invalid",
			UserID:   "remote-user",
			ChatID:   "remote-chat",
			ThreadID: "remote-thread",
		},
		ParentActorID: "parent-clear",
	}
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	oldCtrl := control.New(control.Options{
		Runner:        runner,
		SessionDir:    dir,
		SessionPath:   path,
		Label:         "test",
		WorkspaceRoot: root,
		Actor:         actor,
	})
	app := NewApp()
	app.projectTreeChangedHook = func() {}
	app.setTestCtrl(oldCtrl, testKWUDefaultModelRef)
	app.tabRuntimes["test"].WorkspaceRoot = root
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	oldCtrl.Submit("work")
	<-runner.started
	if err := app.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	waitNotRunning(t, oldCtrl)
	tab := app.activeTab()
	if tab == nil || tab.Ctrl == nil || tab.Ctrl == oldCtrl {
		t.Fatalf("active controller was not rebuilt after clear: %+v", tab)
	}
	got := tab.Ctrl.ActorContext()
	if got.ActorID != actor.ActorID || got.ProfileID != actor.ProfileID || got.AuthMethod != actor.AuthMethod {
		t.Fatalf("rebuilt ActorContext() = %+v", got)
	}
	if got.ParentActorID != actor.ParentActorID {
		t.Fatalf("ParentActorID = %q, want %q", got.ParentActorID, actor.ParentActorID)
	}
	if filepath.Clean(got.WorkspaceRoot) != filepath.Clean(actor.WorkspaceRoot) {
		t.Fatalf("WorkspaceRoot = %q, want %q", got.WorkspaceRoot, actor.WorkspaceRoot)
	}
	if got.RemoteIdentity == nil {
		t.Fatalf("RemoteIdentity = nil")
	}
	remote := *got.RemoteIdentity
	if remote.Provider != "lark" || remote.Domain != "example.invalid" || remote.UserID != "remote-user" || remote.ChatID != "remote-chat" || remote.ThreadID != "remote-thread" {
		t.Fatalf("RemoteIdentity = %+v", remote)
	}
}

func TestClearSessionRemovesRunningJobArtifacts(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "clear-running-job.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	jm := jobs.NewManager(event.Discard)
	oldCtrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	app := NewApp()
	app.projectTreeChangedHook = func() {}
	app.setTestCtrl(oldCtrl, testKWUDefaultModelRef)
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	started := make(chan struct{})
	jm.StartForSession(agent.BranchID(path), "bash", "clear artifact", func(ctx context.Context, _ io.Writer) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	<-started
	jobsDir := jobs.ArtifactDir(path)
	if _, err := os.Stat(jobsDir); err != nil {
		t.Fatalf("job sidecar should exist before clear: %v", err)
	}

	if err := app.ClearSession(); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	if _, err := os.Stat(jobsDir); !os.IsNotExist(err) {
		t.Fatalf("old job sidecar should be removed after clear, stat err = %v", err)
	}
}

func TestSearchFileRefsFindsNestedBasename(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	dir := robustTempDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "wailsjs", "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "wailsjs", "runtime", "runtime.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "Thumbs.db"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", ".DS_Store"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "runtime.js"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, noise := range []string{".codex", ".npm", ".pnpm-store", "bin", "dist", "stage", "tmp"} {
		if err := os.MkdirAll(filepath.Join(dir, noise), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, noise, "runtime.js"), []byte("noise"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "desktop", "frontend", "wailsjs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "desktop", "frontend", "wailsjs", "runtime.js"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "product", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "product", "bin", "runtime.js"), []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	listed := app.ListDir("")
	for _, hidden := range []string{".codex", ".npm", ".pnpm-store", "bin", "dist", "stage", "tmp"} {
		if hasDirEntry(listed, hidden) {
			t.Fatalf("ListDir should hide local noise %q, got %+v", hidden, listed)
		}
	}
	desktopFrontend := app.ListDir("desktop/frontend")
	if hasDirEntry(desktopFrontend, "wailsjs") {
		t.Fatalf("ListDir should hide generated Wails bindings, got %+v", desktopFrontend)
	}
	frontendEntries := app.ListDir("frontend")
	for _, hidden := range []string{".DS_Store", "Thumbs.db"} {
		if hasDirEntry(frontendEntries, hidden) {
			t.Fatalf("ListDir should hide local noise file %q, got %+v", hidden, frontendEntries)
		}
	}

	got := app.SearchFileRefs("runtime.js")
	if !hasDirEntry(got, "frontend/wailsjs/runtime/runtime.js") {
		t.Fatalf("SearchFileRefs(runtime.js) should find nested workspace file, got %+v", got)
	}
	if !hasDirEntry(got, "product/bin/runtime.js") {
		t.Fatalf("SearchFileRefs should keep non-root bin directories searchable, got %+v", got)
	}
	if hasDirEntry(got, "node_modules/pkg/runtime.js") {
		t.Fatalf("SearchFileRefs should skip node_modules noise, got %+v", got)
	}
	for _, hidden := range []string{
		".codex/runtime.js",
		".npm/runtime.js",
		".pnpm-store/runtime.js",
		"bin/runtime.js",
		"desktop/frontend/wailsjs/runtime.js",
		"dist/runtime.js",
		"stage/runtime.js",
		"tmp/runtime.js",
	} {
		if hasDirEntry(got, hidden) {
			t.Fatalf("SearchFileRefs should skip local noise %q, got %+v", hidden, got)
		}
	}
	if noise := app.SearchFileRefs("Thumbs"); hasDirEntry(noise, "frontend/Thumbs.db") {
		t.Fatalf("SearchFileRefs should skip Thumbs.db noise, got %+v", noise)
	}
	if noise := app.SearchFileRefs(".DS"); hasDirEntry(noise, "frontend/.DS_Store") {
		t.Fatalf("SearchFileRefs should skip .DS_Store noise even for dot-prefixed search, got %+v", noise)
	}
}

func TestFileRefsUseActiveTabWorkspaceRoot(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	launchRoot := robustTempDir(t)
	projectRoot := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(launchRoot, "launch-only.txt"), []byte("wrong"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "frontend", "wailsjs", "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectFile := filepath.Join(projectRoot, "frontend", "wailsjs", "runtime", "runtime.js")
	if err := os.WriteFile(projectFile, []byte("right workspace"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(launchRoot); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := &WorkspaceTab{ID: "project", Scope: "project", WorkspaceRoot: projectRoot}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.activeTabID = tab.ID

	listed := app.ListDir("")
	if !hasDirEntry(listed, "frontend") {
		t.Fatalf("ListDir should list active project root, got %+v", listed)
	}
	if hasDirEntry(listed, "launch-only.txt") {
		t.Fatalf("ListDir leaked launch cwd entries, got %+v", listed)
	}

	found := app.SearchFileRefs("runtime.js")
	if !hasDirEntry(found, "frontend/wailsjs/runtime/runtime.js") {
		t.Fatalf("SearchFileRefs should search active project root, got %+v", found)
	}
	preview := app.ReadFile("frontend/wailsjs/runtime/runtime.js")
	if preview.Err != "" || preview.Body != "right workspace" {
		t.Fatalf("ReadFile active project preview = %+v, want project file", preview)
	}
}

func TestDeleteSessionCancelsActiveRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "active.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	app := NewApp()
	activeCtrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test"})
	keepPath := filepath.Join(dir, "keep.jsonl")
	if err := os.WriteFile(keepPath, []byte(`{"role":"user","content":"keep"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write keep session: %v", err)
	}
	keepCtrl := control.New(control.Options{SessionDir: dir, SessionPath: keepPath, Label: "keep"})
	defer keepCtrl.Close()
	app.setTestCtrl(activeCtrl, "")
	app.tabRuntimes["keep"] = &WorkspaceTab{ID: "keep", Scope: "global", Ctrl: keepCtrl, Ready: true}
	app.tabOrder = []string{"test", "keep"}
	persistTestTabState(t, app, "test")

	if err := app.DeleteSession(filepath.Base(path)); err != nil {
		t.Fatalf("DeleteSession(active basename): %v", err)
	}
	if _, ok := app.tabRuntimes["test"]; ok {
		t.Fatalf("deleted active session runtime should be removed")
	}
	if got := app.cachedActiveTabID(); got != "keep" {
		t.Fatalf("active tab after delete = %q, want keep", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("active session should be moved out of active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "active.jsonl", "active.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("active session should be moved to trash: %v", err)
	}
}

func TestDeleteSessionCancelsPreReadyBlankBuild(t *testing.T) {
	isolateDesktopUserDirs(t)

	globalRoot := globalTabWorkspaceRoot()
	dir := desktopSessionDir(globalRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "pre-ready-blank.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write blank session: %v", err)
	}
	cancelled := false
	blank := &WorkspaceTab{
		ID:            "blank",
		Scope:         "global",
		WorkspaceRoot: globalRoot,
		SessionPath:   path,
		buildCancel:   func() { cancelled = true },
		disabledMCP:   map[string]ServerView{},
	}
	keep := &WorkspaceTab{
		ID:            "keep",
		Scope:         "global",
		WorkspaceRoot: globalRoot,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"blank": blank, "keep": keep},
		tabOrder:    []string{"blank", "keep"},
		activeTabID: "blank",
	}
	persistTestTabState(t, app, "blank")

	if err := app.DeleteSession(filepath.Base(path)); err != nil {
		t.Fatalf("DeleteSession(pre-ready blank): %v", err)
	}
	if !cancelled {
		t.Fatal("pre-ready blank build was not cancelled")
	}
	if !blank.removed {
		t.Fatal("pre-ready blank tab was not marked removed")
	}
	if _, ok := app.tabRuntimes["blank"]; ok {
		t.Fatal("pre-ready blank tab should be removed")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("blank session should be moved out of active history, stat err = %v", err)
	}
}

func TestDeleteLastTopicSessionFallbackDoesNotReuseDeletedTopic(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_delete_last"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Delete last"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := writeTopicSession(t, dir, "delete-last.jsonl", topicID, "Delete last", projectRoot)
	ctrl := controllerWithContent(t, path)
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"only": {
				ID:            "only",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Delete last",
				Ctrl:          ctrl,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"only"},
		activeTabID: "only",
	}

	if err := app.DeleteSession(path); err != nil {
		t.Fatalf("DeleteSession(last topic session): %v", err)
	}

	if _, ok := app.tabRuntimes["only"]; ok {
		t.Fatalf("deleted topic session tab should be removed")
	}
	for id, tab := range app.tabRuntimes {
		if tab.TopicID == topicID {
			t.Fatalf("fallback tab %q reused deleted topic %q", id, topicID)
		}
		if strings.TrimSpace(tab.TopicID) == "" {
			t.Fatalf("fallback tab %q has empty topic ID", id)
		}
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "delete-last.jsonl", "delete-last.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("deleted session should be moved to trash: %v", err)
	}
}

func TestDeleteSessionFallbackKeepsTopicWithRemainingHistory(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_delete_keep_history"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Keep history"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := writeTopicSession(t, dir, "delete-one.jsonl", topicID, "Keep history", projectRoot)
	remainingPath := writeTopicSessionWithPrompt(t, dir, "remaining.jsonl", topicID, "Keep history", projectRoot, "remaining turn", time.Now().Add(-time.Minute))
	ctrl := controllerWithContent(t, path)
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"only": {
				ID:            "only",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Keep history",
				Ctrl:          ctrl,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"only"},
		activeTabID: "only",
	}

	if err := app.DeleteSession(path); err != nil {
		t.Fatalf("DeleteSession(topic with remaining history): %v", err)
	}

	found := false
	for _, tab := range app.tabRuntimes {
		if tab.TopicID == topicID {
			found = true
			if got := filepath.Clean(tab.currentSessionPath()); got != filepath.Clean(remainingPath) {
				t.Fatalf("fallback session path = %q, want remaining history %q", got, remainingPath)
			}
		}
	}
	if !found {
		t.Fatalf("fallback should keep topic %q when another session remains", topicID)
	}
}

func TestDeleteSessionWithStuckJobReturnsAfterSingleGrace(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "stuck-delete.jsonl")
	keepPath := filepath.Join(dir, "keep.jsonl")
	for _, p := range []string{path, keepPath} {
		if err := os.WriteFile(p, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
			t.Fatalf("write session %s: %v", p, err)
		}
	}

	grace := 500 * time.Millisecond
	slack := 300 * time.Millisecond
	jm := jobs.NewManager(event.Discard, jobs.WithTeardownGrace(grace))
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	keepCtrl := control.New(control.Options{SessionDir: dir, SessionPath: keepPath, Label: "keep"})
	releaseJob := startNonCooperativeSessionJob(t, jm, path)
	defer func() {
		releaseJob()
		ctrl.Close()
		keepCtrl.Close()
	}()

	app := NewApp()
	app.setTestCtrl(ctrl, "")
	app.tabRuntimes["keep"] = &WorkspaceTab{ID: "keep", Scope: "global", Ctrl: keepCtrl, Ready: true}
	app.tabOrder = []string{"test", "keep"}
	persistTestTabState(t, app, "test")

	start := time.Now()
	if err := app.DeleteSession(filepath.Base(path)); err != nil {
		t.Fatalf("DeleteSession(stuck job): %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > grace+slack {
		t.Fatalf("DeleteSession took %s, want one teardown grace plus scheduling slack", elapsed)
	}
	if !agent.IsCleanupPending(path) {
		t.Fatalf("stuck delete should mark cleanup pending")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stuck session file should remain until delayed cleanup: %v", err)
	}
}

func TestDeleteSessionTrashConflictKeepsRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "active-conflict.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, sessionTrashDir, filepath.Base(path)), 0o755); err != nil {
		t.Fatalf("create trash conflict: %v", err)
	}

	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := control.New(control.Options{Runner: runner, SessionDir: dir, SessionPath: path, Label: "test"})
	app := NewApp()
	app.setTestCtrl(ctrl, "")
	defer ctrl.Close()
	ctrl.Submit("work")
	<-runner.started

	err := app.DeleteSession(filepath.Base(path))
	if err != nil {
		t.Fatalf("DeleteSession should succeed after cleaning empty trash dir: %v", err)
	}
	if _, ok := app.tabRuntimes["test"]; ok {
		t.Fatalf("deleted session runtime should be removed from tabs")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file should be moved out of active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, filepath.Base(path), filepath.Base(path))
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("session should be moved to trash: %v", err)
	}

	close(runner.release)
	waitNotRunning(t, ctrl)
}

func TestDeleteSessionValidTrashRemovesEmptyLiveStub(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "stale-live.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write live stub: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, filepath.Base(path), filepath.Base(path))
	if err := os.MkdirAll(filepath.Dir(trashPath), 0o755); err != nil {
		t.Fatalf("create trash dir: %v", err)
	}
	if err := os.WriteFile(trashPath, []byte(`{"role":"user","content":"trashed"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write trash session: %v", err)
	}

	activePath := filepath.Join(dir, "active.jsonl")
	if err := os.WriteFile(activePath, []byte(`{"role":"user","content":"active"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write active session: %v", err)
	}
	activeCtrl := control.New(control.Options{SessionDir: dir, SessionPath: activePath, Label: "active"})
	defer activeCtrl.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"active": {ID: "active", Scope: "global", Ctrl: activeCtrl, Ready: true}},
		activeTabID: "active",
		tabOrder:    []string{"active"},
	}

	if err := app.DeleteSession(filepath.Base(path)); err != nil {
		t.Fatalf("DeleteSession should remove stale live stub: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("live stub should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("existing trash should remain authoritative: %v", err)
	}
}

func TestRestoreSessionRejectsOpenEmptyLiveStub(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "restore-open.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"trashed"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write trash source: %v", err)
	}
	if err := deleteSessionFile(dir, path); err != nil {
		t.Fatalf("trash source: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, filepath.Base(path), filepath.Base(path))
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write live stub: %v", err)
	}
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "open"})
	defer ctrl.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"open": {ID: "open", Scope: "global", Ctrl: ctrl, Ready: true}},
		tabOrder:    []string{"open"},
		activeTabID: "open",
	}

	err := app.RestoreSession(trashPath)
	if err == nil || !strings.Contains(err.Error(), "session is open") {
		t.Fatalf("RestoreSession error = %v, want open-session rejection", err)
	}
	if info, statErr := os.Stat(path); statErr != nil || info.Size() != 0 {
		t.Fatalf("open live stub should remain empty, info=%v err=%v", info, statErr)
	}
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("trash session should remain after rejected restore: %v", err)
	}
}

func TestListTrashedSessionsUsesActiveProfileSessionDir(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	actorA := userctx.Default(projectRoot)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(projectRoot)
	actorB.ProfileID = "profile-b"
	dirA := config.NewProfilePathResolver(actorA).ProjectSessionDir(projectRoot)
	dirB := config.NewProfilePathResolver(actorB).ProjectSessionDir(projectRoot)
	trashA := writeTestTrashedSession(t, dirA, "profile-a-trash.jsonl", "profile A")
	trashB := writeTestTrashedSession(t, dirB, "profile-b-trash.jsonl", "profile B")

	ctrlA := control.New(control.Options{SessionDir: dirA, SessionPath: filepath.Join(dirA, "active-a.jsonl"), Label: "a", Actor: actorA, WorkspaceRoot: projectRoot})
	ctrlB := control.New(control.Options{SessionDir: dirB, SessionPath: filepath.Join(dirB, "active-b.jsonl"), Label: "b", Actor: actorB, WorkspaceRoot: projectRoot})
	defer ctrlA.Close()
	defer ctrlB.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"a": {ID: "a", Scope: "project", WorkspaceRoot: projectRoot, Ctrl: ctrlA, Ready: true, actor: actorA},
			"b": {ID: "b", Scope: "project", WorkspaceRoot: projectRoot, Ctrl: ctrlB, Ready: true, actor: actorB},
		},
		tabOrder:    []string{"a", "b"},
		activeTabID: "b",
	}

	trashed := app.ListTrashedSessions()
	if len(trashed) != 1 {
		t.Fatalf("ListTrashedSessions len = %d, want 1: %+v", len(trashed), trashed)
	}
	if filepath.Clean(trashed[0].Path) != filepath.Clean(trashB) {
		t.Fatalf("ListTrashedSessions path = %q, want active profile trash %q", trashed[0].Path, trashB)
	}
	if filepath.Clean(trashed[0].Path) == filepath.Clean(trashA) {
		t.Fatalf("inactive profile trash leaked into active profile list: %+v", trashed)
	}
}

func TestRestoreAndPurgeTrashedSessionRejectCrossProfilePath(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	actorA := userctx.Default(projectRoot)
	actorA.ProfileID = "profile-a"
	actorB := userctx.Default(projectRoot)
	actorB.ProfileID = "profile-b"
	dirA := config.NewProfilePathResolver(actorA).ProjectSessionDir(projectRoot)
	dirB := config.NewProfilePathResolver(actorB).ProjectSessionDir(projectRoot)
	restorePath := writeTestTrashedSession(t, dirA, "restore-a.jsonl", "restore A")
	purgePath := writeTestTrashedSession(t, dirA, "purge-a.jsonl", "purge A")

	ctrlA := control.New(control.Options{SessionDir: dirA, SessionPath: filepath.Join(dirA, "active-a.jsonl"), Label: "a", Actor: actorA, WorkspaceRoot: projectRoot})
	ctrlB := control.New(control.Options{SessionDir: dirB, SessionPath: filepath.Join(dirB, "active-b.jsonl"), Label: "b", Actor: actorB, WorkspaceRoot: projectRoot})
	defer ctrlA.Close()
	defer ctrlB.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"a": {ID: "a", Scope: "project", WorkspaceRoot: projectRoot, Ctrl: ctrlA, Ready: true, actor: actorA},
			"b": {ID: "b", Scope: "project", WorkspaceRoot: projectRoot, Ctrl: ctrlB, Ready: true, actor: actorB},
		},
		tabOrder:    []string{"a", "b"},
		activeTabID: "b",
	}

	if err := app.RestoreSession(restorePath); err == nil || !strings.Contains(err.Error(), "outside active profile session dirs") {
		t.Fatalf("RestoreSession cross-profile error = %v, want active-profile rejection", err)
	}
	if _, err := os.Stat(restorePath); err != nil {
		t.Fatalf("cross-profile restore should leave trash in place: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirA, filepath.Base(restorePath))); !os.IsNotExist(err) {
		t.Fatalf("cross-profile restore should not recreate live session, stat err = %v", err)
	}

	if err := app.PurgeTrashedSession(purgePath); err == nil || !strings.Contains(err.Error(), "outside active profile session dirs") {
		t.Fatalf("PurgeTrashedSession cross-profile error = %v, want active-profile rejection", err)
	}
	if _, err := os.Stat(purgePath); err != nil {
		t.Fatalf("cross-profile purge should leave trash in place: %v", err)
	}
}

func TestDefaultProfileGlobalTrashIncludesLegacySessionDir(t *testing.T) {
	isolateDesktopUserDirs(t)

	globalRoot := globalWorkspaceRoot()
	actor := userctx.Default(globalRoot)
	activeDir := desktopSessionDirForActor(actor, globalRoot)
	legacyDir := config.SessionDir()
	legacyTrash := writeTestTrashedSession(t, legacyDir, "legacy-global.jsonl", "legacy")

	ctrl := control.New(control.Options{SessionDir: activeDir, SessionPath: filepath.Join(activeDir, "active.jsonl"), Label: "default", Actor: actor, WorkspaceRoot: globalRoot})
	defer ctrl.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"default": {ID: "default", Scope: "global", WorkspaceRoot: globalRoot, Ctrl: ctrl, Ready: true, actor: actor},
		},
		tabOrder:    []string{"default"},
		activeTabID: "default",
	}

	trashed := app.ListTrashedSessions()
	if len(trashed) != 1 || filepath.Clean(trashed[0].Path) != filepath.Clean(legacyTrash) {
		t.Fatalf("ListTrashedSessions legacy default trash = %+v, want %q", trashed, legacyTrash)
	}
	if err := app.RestoreSession(legacyTrash); err != nil {
		t.Fatalf("RestoreSession legacy default trash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, filepath.Base(legacyTrash))); err != nil {
		t.Fatalf("legacy default trash should restore to legacy session dir: %v", err)
	}
}

func writeTestTrashedSession(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, name)
	body := []byte(fmt.Sprintf(`{"role":"user","content":%q}`+"\n", content))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := deleteSessionFile(dir, path); err != nil {
		t.Fatalf("trash session: %v", err)
	}
	return filepath.Join(dir, sessionTrashDir, name, name)
}

func TestDeleteSessionValidTrashRejectsNonEmptyLive(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "real-live.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"new work"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write live session: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, filepath.Base(path), filepath.Base(path))
	if err := os.MkdirAll(filepath.Dir(trashPath), 0o755); err != nil {
		t.Fatalf("create trash dir: %v", err)
	}
	if err := os.WriteFile(trashPath, []byte(`{"role":"user","content":"trashed"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write trash session: %v", err)
	}

	activeCtrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "active"})
	defer activeCtrl.Close()
	app := NewApp()
	app.setTestCtrl(activeCtrl, "")

	err := app.DeleteSession(filepath.Base(path))
	if err == nil || !strings.Contains(err.Error(), "already exists in trash") {
		t.Fatalf("DeleteSession conflict error = %v, want valid trash conflict", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("non-empty live session should remain: %v", err)
	}
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("trash session should remain: %v", err)
	}
}

func TestDeleteSessionCancelsInactiveOpenRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	activePath := filepath.Join(dir, "active.jsonl")
	inactivePath := filepath.Join(dir, "inactive.jsonl")
	otherPath := filepath.Join(dir, "other.jsonl")
	for _, path := range []string{activePath, inactivePath, otherPath} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
			t.Fatalf("write session %s: %v", path, err)
		}
	}

	activeCtrl := control.New(control.Options{SessionDir: dir, SessionPath: activePath, Label: "active"})
	inactiveCtrl := control.New(control.Options{SessionDir: dir, SessionPath: inactivePath, Label: "inactive"})
	defer activeCtrl.Close()
	defer inactiveCtrl.Close()

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"active":   {ID: "active", Scope: "global", Ctrl: activeCtrl, Ready: true},
			"inactive": {ID: "inactive", Scope: "global", Ctrl: inactiveCtrl, Ready: true},
		},
		tabOrder:    []string{"active", "inactive"},
		activeTabID: "active",
	}

	if err := app.DeleteSession(filepath.Base(inactivePath)); err != nil {
		t.Fatalf("DeleteSession(inactive open basename): %v", err)
	}
	if _, ok := app.tabRuntimes["inactive"]; ok {
		t.Fatalf("deleted inactive session runtime should be removed")
	}
	if _, err := os.Stat(inactivePath); !os.IsNotExist(err) {
		t.Fatalf("inactive open session should be moved out of active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "inactive.jsonl", "inactive.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("inactive open session should be moved to trash: %v", err)
	}

	sessions := app.ListSessions()
	current := map[string]bool{}
	open := map[string]bool{}
	for _, s := range sessions {
		current[filepath.Base(s.Path)] = s.Current
		open[filepath.Base(s.Path)] = s.Open
	}
	if !current[filepath.Base(activePath)] {
		t.Fatalf("ListSessions should mark active session current, got %#v", current)
	}
	if current[filepath.Base(otherPath)] {
		t.Fatalf("ListSessions marked unopened session current, got %#v", current)
	}
	if !open[filepath.Base(activePath)] {
		t.Fatalf("ListSessions should mark active and inactive open sessions open, got %#v", open)
	}
	if open[filepath.Base(inactivePath)] || open[filepath.Base(otherPath)] {
		t.Fatalf("ListSessions marked unopened session open, got %#v", open)
	}
}

func TestTrashTopicWithStuckJobReturnsAfterSingleGrace(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_stuck_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Stuck trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "stuck-topic.jsonl", topicID, "Stuck trash", projectRoot)

	grace := 500 * time.Millisecond
	slack := 300 * time.Millisecond
	jm := jobs.NewManager(event.Discard, jobs.WithTeardownGrace(grace))
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: sessionPath, Label: "test", Jobs: jm, WorkspaceRoot: projectRoot})
	releaseJob := startNonCooperativeSessionJob(t, jm, sessionPath)
	defer func() {
		releaseJob()
		ctrl.Close()
	}()

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"stuck": {
				ID:            "stuck",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Stuck trash",
				Ctrl:          ctrl,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
			"keep": {
				ID:            "keep",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       "topic_keep",
				TopicTitle:    "Keep",
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"stuck", "keep"},
		activeTabID: "stuck",
	}
	persistTestTabState(t, app, "stuck")

	start := time.Now()
	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("TrashTopic(stuck job): %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > grace+slack {
		t.Fatalf("TrashTopic took %s, want one teardown grace plus scheduling slack", elapsed)
	}
	if !agent.IsCleanupPending(sessionPath) {
		t.Fatalf("stuck topic trash should mark cleanup pending")
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("stuck topic session should remain until delayed trash: %v", err)
	}
}

func TestRestoreSessionRejectsDestroyingSession(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := filepath.Join(dir, "trash-me.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := deleteSessionFile(dir, sessionPath); err != nil {
		t.Fatalf("deleteSessionFile: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, filepath.Base(sessionPath), filepath.Base(sessionPath))

	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: filepath.Join(dir, "active.jsonl"), Label: "active", Jobs: jm})
	defer ctrl.Close()
	destroy := ctrl.BeginDestroySession(sessionPath)
	defer destroy.Finish()

	app := NewApp()
	app.setTestCtrl(ctrl, "")
	if err := app.RestoreSession(trashPath); err == nil || !strings.Contains(err.Error(), "cleanup is still in progress") {
		t.Fatalf("RestoreSession while destroying error = %v, want cleanup-in-progress", err)
	}
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("trashed session should remain after rejected restore: %v", err)
	}

	destroy.Finish()
	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("RestoreSession after finish: %v", err)
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session should be restored: %v", err)
	}
}

func TestDesktopSessionAPIsUseControllerSessionDir(t *testing.T) {
	isolateDesktopUserDirs(t)

	dirA := filepath.Join(t.TempDir(), "workspace-a-sessions")
	dirB := filepath.Join(t.TempDir(), "workspace-b-sessions")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir dirA: %v", err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("mkdir dirB: %v", err)
	}
	pathA := filepath.Join(dirA, "a.jsonl")
	pathB := filepath.Join(dirB, "b.jsonl")
	if err := os.WriteFile(pathA, []byte(`{"role":"user","content":"workspace A"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write pathA: %v", err)
	}
	if err := os.WriteFile(pathB, []byte(`{"role":"user","content":"workspace B"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write pathB: %v", err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{SessionDir: dirA, SessionPath: pathA, Label: "test"}), "")
	defer app.activeCtrl().Close()

	sessions := app.ListSessions()
	if len(sessions) != 1 || sessions[0].Path != pathA || sessions[0].Preview != "workspace A" {
		t.Fatalf("ListSessions should read the active controller session dir only, got %+v", sessions)
	}
	if err := app.RenameSession(pathA, "A title"); err != nil {
		t.Fatalf("RenameSession in active session dir: %v", err)
	}
	if titles := loadSessionTitles(dirA); titles["a.jsonl"] != "A title" {
		t.Fatalf("title should be written beside the active session, got %+v", titles)
	}
	if titles := loadSessionTitles(dirB); len(titles) != 0 {
		t.Fatalf("inactive workspace title sidecar should remain untouched, got %+v", titles)
	}
}

func TestListAllSessionsAcrossConfiguredWorkspaces(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	projectA := t.TempDir()
	projectB := t.TempDir()
	if err := addProject(projectA, "Workspace A"); err != nil {
		t.Fatalf("add project A: %v", err)
	}
	if err := addProject(projectB, "Workspace B"); err != nil {
		t.Fatalf("add project B: %v", err)
	}
	dirA := desktopSessionDir(projectA)
	dirB := desktopSessionDir(projectB)
	for _, dir := range []string{dirA, dirB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	now := time.Now()
	pathA := writeTopicSessionWithPrompt(t, dirA, "workspace-a.jsonl", "topic-a", "Topic A", projectA, "workspace A", now.Add(-time.Hour))
	pathB := writeTopicSessionWithPrompt(t, dirB, "workspace-b.jsonl", "topic-b", "Topic B", projectB, "workspace B", now)

	app.setTestCtrl(control.New(control.Options{SessionDir: dirA, SessionPath: pathA, Label: "test", WorkspaceRoot: projectA}), projectA)
	defer app.activeCtrl().Close()

	scoped := app.ListSessions()
	if len(scoped) != 1 || filepath.Clean(scoped[0].Path) != filepath.Clean(pathA) {
		t.Fatalf("ListSessions should remain scoped to the active workspace, got %+v", scoped)
	}
	all := app.ListAllSessions()
	if len(all) != 2 {
		t.Fatalf("ListAllSessions len = %d, want 2: %+v", len(all), all)
	}
	if filepath.Clean(all[0].Path) != filepath.Clean(pathB) || filepath.Clean(all[1].Path) != filepath.Clean(pathA) {
		t.Fatalf("ListAllSessions order = [%q, %q], want newest workspace B then A", all[0].Path, all[1].Path)
	}
	if all[0].WorkspaceRoot != projectB || all[1].WorkspaceRoot != projectA {
		t.Fatalf("ListAllSessions workspace metadata = %+v", all)
	}

	if err := app.RenameSession(pathB, "Renamed B"); err != nil {
		t.Fatalf("rename inactive workspace session: %v", err)
	}
	if got := loadSessionTitles(dirB)["workspace-b.jsonl"]; got != "Renamed B" {
		t.Fatalf("inactive workspace title = %q, want Renamed B", got)
	}
	if err := app.DeleteSession(pathB); err != nil {
		t.Fatalf("delete inactive workspace session: %v", err)
	}
	if _, err := os.Stat(pathB); !os.IsNotExist(err) {
		t.Fatalf("deleted inactive workspace session should leave live list, stat err = %v", err)
	}
}

func TestListAllTrashedSessionsAcrossConfiguredWorkspaces(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	projectA := t.TempDir()
	projectB := t.TempDir()
	if err := addProject(projectA, "Workspace A"); err != nil {
		t.Fatalf("add project A: %v", err)
	}
	if err := addProject(projectB, "Workspace B"); err != nil {
		t.Fatalf("add project B: %v", err)
	}
	dirA := desktopSessionDir(projectA)
	dirB := desktopSessionDir(projectB)
	trashA := writeTestTrashedSession(t, dirA, "trash-a.jsonl", "trash A")
	trashB := writeTestTrashedSession(t, dirB, "trash-b.jsonl", "trash B")

	ctrl := control.New(control.Options{SessionDir: dirA, SessionPath: filepath.Join(dirA, "active.jsonl"), Label: "test", WorkspaceRoot: projectA})
	app.setTestCtrl(ctrl, projectA)
	defer app.activeCtrl().Close()

	scoped := app.ListTrashedSessions()
	if len(scoped) != 1 || filepath.Clean(scoped[0].Path) != filepath.Clean(trashA) {
		t.Fatalf("ListTrashedSessions should remain scoped to workspace A, got %+v", scoped)
	}
	all := app.ListAllTrashedSessions()
	if len(all) != 2 {
		t.Fatalf("ListAllTrashedSessions len = %d, want 2: %+v", len(all), all)
	}

	if err := app.RestoreSession(trashB); err != nil {
		t.Fatalf("restore inactive workspace trash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirB, "trash-b.jsonl")); err != nil {
		t.Fatalf("restored inactive workspace session: %v", err)
	}
	if err := app.PurgeTrashedSession(trashA); err != nil {
		t.Fatalf("purge active workspace trash: %v", err)
	}
	if _, err := os.Stat(trashA); !os.IsNotExist(err) {
		t.Fatalf("purged trash should be removed, stat err = %v", err)
	}
}

func TestListSessionsMarksAutoBotSessionAsChannel(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "bot-channel.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"from channel"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	cfg := config.Default()
	cfg.Bot.Connections = []config.BotConnectionConfig{{
		ID: "weixin-weixin", Provider: "weixin", Domain: "weixin", Label: "微信", Enabled: true, Status: "connected",
		SessionMappings: []config.BotConnectionSessionMapping{{
			RemoteID: "wx-chat-1", SessionID: "path:" + path, SessionSource: "auto",
		}},
	}}
	if err := cfg.SaveTo(config.UserConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{SessionDir: dir, SessionPath: filepath.Join(dir, "active.jsonl"), Label: "test"}), "")
	defer app.activeCtrl().Close()

	sessions := app.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("ListSessions len = %d, want 1: %+v", len(sessions), sessions)
	}
	got := sessions[0]
	if got.Kind != "channel" || got.Channel != "weixin" || got.ChannelLabel != "微信" || got.RemoteID != "wx-chat-1" || got.SessionSource != "auto" {
		t.Fatalf("channel session meta = %+v", got)
	}
}

func TestOpenChannelSessionForTabIsReadOnly(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "bot-channel.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"from channel"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	app := NewApp()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: filepath.Join(dir, "active.jsonl"), Label: "test"})
	app.setTestCtrl(ctrl, "")
	defer app.activeCtrl().Close()

	if _, err := app.OpenChannelSessionForTab("test", path); err != nil {
		t.Fatalf("OpenChannelSessionForTab: %v", err)
	}
	if meta := app.tabMeta(app.activeTab(), true); !meta.ReadOnly {
		t.Fatalf("channel tab should be read-only: %+v", meta)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	app.SubmitToTab("test", "must not append")
	app.RunShellForTab("test", "echo must-not-run")
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("read-only channel transcript changed:\nbefore=%s\nafter=%s", before, after)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := f.WriteString(`{"role":"user","content":"external follow-up"}` + "\n"); err != nil {
		f.Close()
		t.Fatalf("append external message: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close append: %v", err)
	}
	app.snapshotAllTabs()
	afterSnapshot, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after snapshot: %v", err)
	}
	if !strings.Contains(string(afterSnapshot), "external follow-up") {
		t.Fatalf("read-only channel snapshot overwrote external append:\n%s", afterSnapshot)
	}
}

func TestUserTriggeredCommandsReturnErrorsWhenUnavailable(t *testing.T) {
	tests := []struct {
		name string
		app  *App
		call func(*App) error
		want string
	}{
		{
			name: "submit read-only",
			app: &App{
				tabRuntimes: map[string]*WorkspaceTab{"test": {ID: "test", Scope: "global", ReadOnly: true}},
				activeTabID: "test",
			},
			call: func(app *App) error { return app.SubmitToTab("test", "hello") },
			want: "read-only",
		},
		{
			name: "submit workspace unavailable",
			app: &App{
				tabRuntimes: map[string]*WorkspaceTab{"test": {ID: "test", Scope: "global", StartupErr: "boom"}},
				activeTabID: "test",
			},
			call: func(app *App) error { return app.SubmitToTab("test", "hello") },
			want: "workspace failed to start: boom",
		},
		{
			name: "run shell workspace unavailable",
			app: &App{
				tabRuntimes: map[string]*WorkspaceTab{"test": {ID: "test", Scope: "global"}},
				activeTabID: "test",
			},
			call: func(app *App) error { return app.RunShellForTab("test", "echo hi") },
			want: "workspace is still starting",
		},
		{
			name: "set expert workspace unavailable",
			app: &App{
				tabRuntimes: map[string]*WorkspaceTab{"test": {ID: "test", Scope: "global", StartupErr: "boom"}},
				activeTabID: "test",
			},
			call: func(app *App) error {
				_, err := app.SetExpertForTab("test", "testing")
				return err
			},
			want: "workspace failed to start: boom",
		},
		{
			name: "steer workspace unavailable",
			app: &App{
				tabRuntimes: map[string]*WorkspaceTab{"test": {ID: "test", Scope: "global"}},
				activeTabID: "test",
			},
			call: func(app *App) error { return app.SteerForTab("test", "please continue") },
			want: "workspace is still starting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(tt.app)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err, tt.want)
			}
		})
	}
}

func TestCloseReadOnlyChannelTabDoesNotSnapshotTranscript(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, "bot-channel.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"from channel"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	app := NewApp()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: filepath.Join(dir, "active.jsonl"), Label: "test"})
	app.setTestCtrl(ctrl, "")
	defer ctrl.Close()

	if _, err := app.OpenChannelSessionForTab("test", path); err != nil {
		t.Fatalf("OpenChannelSessionForTab: %v", err)
	}
	app.mu.Lock()
	app.tabRuntimes["survivor"] = &WorkspaceTab{ID: "survivor", Scope: "global", Ready: true, disabledMCP: map[string]ServerView{}}
	app.tabOrder = []string{"test", "survivor"}
	app.activeTabID = "test"
	app.mu.Unlock()
	persistTestTabState(t, app, "test")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := f.WriteString(`{"role":"user","content":"external close follow-up"}` + "\n"); err != nil {
		f.Close()
		t.Fatalf("append external message: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close append: %v", err)
	}

	if err := app.CloseTab("test"); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	afterClose, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after close: %v", err)
	}
	if !strings.Contains(string(afterClose), "external close follow-up") {
		t.Fatalf("closing read-only channel tab overwrote external append:\n%s", afterClose)
	}
}

func TestResumeSessionRejectsCleanupPending(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	activePath := filepath.Join(dir, "active.jsonl")
	pendingPath := filepath.Join(dir, "pending.jsonl")
	for _, path := range []string{activePath, pendingPath} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	if err := agent.MarkCleanupPending(pendingPath, "delete"); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: activePath, Label: "test"})
	app.setTestCtrl(ctrl, "")
	defer app.activeCtrl().Close()

	if _, err := app.ResumeSession(pendingPath); err == nil || !strings.Contains(err.Error(), "pending cleanup") {
		t.Fatalf("ResumeSession cleanup-pending error = %v, want pending cleanup", err)
	}
	if got := app.activeCtrl().SessionPath(); filepath.Clean(got) != filepath.Clean(activePath) {
		t.Fatalf("active session path after rejected resume = %q, want %q", got, activePath)
	}
	if _, err := app.OpenChannelSessionForTab("test", pendingPath); err == nil || !strings.Contains(err.Error(), "pending cleanup") {
		t.Fatalf("OpenChannelSessionForTab cleanup-pending error = %v, want pending cleanup", err)
	}
	if meta := app.tabMeta(app.activeTab(), true); meta.ReadOnly {
		t.Fatalf("rejected channel open should not make tab read-only: %+v", meta)
	}
}

func TestResumeSessionRejectsPathOutsideControllerSessionDir(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	activePath := filepath.Join(dirA, "active.jsonl")
	outsidePath := filepath.Join(dirB, "outside.jsonl")
	for _, path := range []string{activePath, outsidePath} {
		if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{SessionDir: dirA, SessionPath: activePath, Label: "test"}), "")
	defer app.activeCtrl().Close()

	if _, err := app.ResumeSession(outsidePath); err == nil {
		t.Fatal("ResumeSession should reject a transcript outside the active session dir")
	}
	if _, err := app.PreviewSession(outsidePath); err == nil {
		t.Fatal("PreviewSession should reject a transcript outside the active session dir")
	}
}

func BenchmarkDesktopListSessionsScoped(b *testing.B) {
	dirA := filepath.Join(b.TempDir(), "workspace-a-sessions")
	dirB := filepath.Join(b.TempDir(), "workspace-b-sessions")
	for _, dir := range []string{dirA, dirB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("mkdir %s: %v", dir, err)
		}
		for i := 0; i < 120; i++ {
			path := filepath.Join(dir, fmt.Sprintf("session-%03d.jsonl", i))
			body := fmt.Sprintf(`{"role":"user","content":"session %03d"}`+"\n", i)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				b.Fatalf("write session: %v", err)
			}
		}
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{SessionDir: dirA, SessionPath: filepath.Join(dirA, "session-000.jsonl"), Label: "test"}), "")
	defer app.activeCtrl().Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessions := app.ListSessions()
		if len(sessions) != 120 {
			b.Fatalf("ListSessions len = %d, want 120", len(sessions))
		}
	}
}

type appendingDesktopRunner struct {
	session *agent.Session
	started chan string
}

func desktopAppForForkFailureTest(t *testing.T, workspace string) (*App, *control.Controller, string, []byte, string, []byte) {
	t.Helper()
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sourcePath := agent.NewSessionPath(dir, "fork-failure")
	sess := agent.NewSession("system")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	runner := &appendingDesktopRunner{session: sess, started: make(chan string, 1)}
	ctrl := control.New(control.Options{
		Runner:        runner,
		Executor:      exec,
		Sink:          event.Discard,
		SessionDir:    dir,
		SessionPath:   sourcePath,
		Label:         "fork-failure",
		WorkspaceRoot: workspace,
	})
	t.Cleanup(ctrl.Close)
	app := NewApp()
	app.setTestCtrl(ctrl, "deepseek/test")
	tab := app.tabRuntimes["test"]
	tab.Scope = "project"
	tab.WorkspaceRoot = workspace
	tab.TopicID = "topic_source"
	tab.TopicTitle = "Source topic"
	tab.SessionPath = sourcePath
	app.projectTreeChangedHook = func() {}
	ctrl.Submit("source prompt")
	<-runner.started
	waitNotRunning(t, ctrl)
	if err := ctrl.Snapshot(); err != nil {
		t.Fatalf("snapshot source: %v", err)
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	unrelatedPath := filepath.Join(dir, "unrelated.txt")
	unrelatedBytes := []byte("unrelated")
	if err := os.WriteFile(unrelatedPath, unrelatedBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return app, ctrl, sourcePath, sourceBytes, unrelatedPath, unrelatedBytes
}

func writeDesktopForkOwnedArtifacts(t *testing.T, path string) []string {
	t.Helper()
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Name: "injected desktop fork"}); err != nil {
		t.Fatalf("write branch meta: %v", err)
	}
	if err := agent.SaveSessionExpert(path, agent.ExpertMeta{ExpertID: "fork-expert"}); err != nil {
		t.Fatalf("write expert meta: %v", err)
	}
	guardianPath := guardian.PathFor(path)
	guardianCursorPath := guardian.CursorPathFor(path)
	for _, artifact := range []string{guardianPath, guardianCursorPath} {
		if err := os.WriteFile(artifact, []byte("guardian"), 0o644); err != nil {
			t.Fatalf("write guardian artifact: %v", err)
		}
	}
	checkpointDir := store.SessionCheckpointDir(path)
	jobDir := jobs.ArtifactDir(path)
	for _, dir := range []string{checkpointDir, jobDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir owned artifact: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "owned"), []byte("owned"), 0o644); err != nil {
			t.Fatalf("write owned artifact: %v", err)
		}
	}
	subagentDir := filepath.Join(filepath.Dir(path), "subagents")
	if err := os.MkdirAll(subagentDir, 0o755); err != nil {
		t.Fatalf("mkdir subagents: %v", err)
	}
	const ref = "sa_desktop_fork_cleanup"
	subagentSession := filepath.Join(subagentDir, ref+".jsonl")
	subagentMeta := filepath.Join(subagentDir, ref+".meta.json")
	if err := os.WriteFile(subagentSession, []byte("subagent"), 0o644); err != nil {
		t.Fatalf("write subagent session: %v", err)
	}
	metaBytes, err := json.Marshal(agent.SubagentMeta{Ref: ref, ParentSession: agent.BranchID(path)})
	if err != nil {
		t.Fatalf("marshal subagent meta: %v", err)
	}
	if err := os.WriteFile(subagentMeta, metaBytes, 0o644); err != nil {
		t.Fatalf("write subagent meta: %v", err)
	}
	if err := agent.MarkCleanupPending(path, "fork-test"); err != nil {
		t.Fatalf("mark cleanup pending: %v", err)
	}
	return []string{
		path,
		agent.BranchMetaPath(path),
		store.SessionExpert(path),
		guardianPath,
		guardianCursorPath,
		checkpointDir,
		jobDir,
		store.SessionCleanupPending(path),
		subagentSession,
		subagentMeta,
	}
}

func TestForkRegistrationFailureKeepsSourceRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, ctrl, sourcePath, sourceBytes, _, _ := desktopAppForForkFailureTest(t, t.TempDir())
	beforeTabs := app.ListTabs()
	beforeHistory := ctrl.History()
	workspaceRoot := app.tabRuntimes["test"].WorkspaceRoot
	titlesPath, err := topicTitlesPath(workspaceRoot)
	if err != nil {
		t.Fatalf("resolve topic titles path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(titlesPath), 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{broken topic titles")
	if err := os.WriteFile(titlesPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	if meta, err := app.Fork(0); err == nil || meta.ID != "" {
		t.Fatalf("Fork meta=%+v err=%v, want registration failure", meta, err)
	}
	if app.cachedActiveTabID() != "test" || !reflect.DeepEqual(app.ListTabs(), beforeTabs) {
		t.Fatalf("app tabs changed after registration failure: active=%q tabs=%+v want=%+v", app.activeTabID, app.ListTabs(), beforeTabs)
	}
	if ctrl.SessionPath() != sourcePath || !reflect.DeepEqual(ctrl.History(), beforeHistory) {
		t.Fatalf("source controller changed after registration failure: path=%q history=%+v", ctrl.SessionPath(), ctrl.History())
	}
	if got, err := os.ReadFile(sourcePath); err != nil || !bytes.Equal(got, sourceBytes) {
		t.Fatalf("source bytes changed: err=%v got=%q want=%q", err, got, sourceBytes)
	}
	if got, err := os.ReadFile(titlesPath); err != nil || !bytes.Equal(got, corrupt) {
		t.Fatalf("corrupt title index changed: err=%v got=%q want=%q", err, got, corrupt)
	}
	entries, err := os.ReadDir(ctrl.SessionDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".jsonl") && entry.Name() != filepath.Base(sourcePath) {
			t.Fatalf("fork session created before topic registration completed: %s", entry.Name())
		}
	}
}

func TestForkTopicMetaFailureCleansNewForkArtifacts(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, ctrl, sourcePath, sourceBytes, unrelatedPath, unrelatedBytes := desktopAppForForkFailureTest(t, t.TempDir())
	beforeTabs := app.ListTabs()
	beforeHistory := ctrl.History()
	originalSave := saveDesktopForkBranchMeta
	originalRemove := removeDesktopForkSessionArtifacts
	injected := errors.New("injected desktop fork topic meta failure")
	var forkPath string
	var ownedPaths []string
	saveDesktopForkBranchMeta = func(path string, _ agent.BranchMeta) error {
		forkPath = path
		ownedPaths = writeDesktopForkOwnedArtifacts(t, path)
		return injected
	}
	t.Cleanup(func() {
		saveDesktopForkBranchMeta = originalSave
		removeDesktopForkSessionArtifacts = originalRemove
	})

	if meta, err := app.Fork(0); !errors.Is(err, injected) || meta.ID != "" {
		t.Fatalf("Fork meta=%+v err=%v, want injected failure", meta, err)
	}
	if forkPath == "" {
		t.Fatal("fork path was not captured")
	}
	for _, path := range ownedPaths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("owned fork artifact still exists %s: %v", path, err)
		}
	}
	if app.cachedActiveTabID() != "test" || !reflect.DeepEqual(app.ListTabs(), beforeTabs) {
		t.Fatalf("app tabs changed after failed fork: active=%q tabs=%+v want=%+v", app.activeTabID, app.ListTabs(), beforeTabs)
	}
	if ctrl.SessionPath() != sourcePath || !reflect.DeepEqual(ctrl.History(), beforeHistory) {
		t.Fatalf("source controller changed after failed fork: path=%q history=%+v", ctrl.SessionPath(), ctrl.History())
	}
	if got, err := os.ReadFile(sourcePath); err != nil || !bytes.Equal(got, sourceBytes) {
		t.Fatalf("source bytes changed: err=%v got=%q want=%q", err, got, sourceBytes)
	}
	if got, err := os.ReadFile(unrelatedPath); err != nil || !bytes.Equal(got, unrelatedBytes) {
		t.Fatalf("unrelated bytes changed: err=%v got=%q want=%q", err, got, unrelatedBytes)
	}
}

func TestForkTopicMetaAndCleanupFailureAreJoined(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, ctrl, sourcePath, _, _, _ := desktopAppForForkFailureTest(t, robustTempDir(t))
	originalSave := saveDesktopForkBranchMeta
	originalRemove := removeDesktopForkSessionArtifacts
	saveErr := errors.New("injected desktop fork topic meta failure")
	cleanupErr := errors.New("injected desktop fork cleanup failure")
	var forkPath string
	saveDesktopForkBranchMeta = func(path string, _ agent.BranchMeta) error {
		forkPath = path
		return saveErr
	}
	removeDesktopForkSessionArtifacts = func(string) error { return cleanupErr }
	t.Cleanup(func() {
		saveDesktopForkBranchMeta = originalSave
		removeDesktopForkSessionArtifacts = originalRemove
		if forkPath != "" {
			_ = originalRemove(forkPath)
		}
	})

	_, err := app.Fork(0)
	if !errors.Is(err, saveErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Fork error = %v, want joined save and cleanup failures", err)
	}
	canonicalPath, absErr := filepath.Abs(forkPath)
	if absErr != nil {
		t.Fatal(absErr)
	}
	if !strings.Contains(err.Error(), canonicalPath) {
		t.Fatalf("Fork error %q does not contain fork path %q", err, canonicalPath)
	}
	if app.cachedActiveTabID() != "test" || ctrl.SessionPath() != sourcePath {
		t.Fatalf("source identity changed after failed cleanup: active=%q path=%q", app.activeTabID, ctrl.SessionPath())
	}
}

func (r *appendingDesktopRunner) Run(_ context.Context, input string) error {
	r.started <- input
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	r.session.Add(provider.Message{Role: provider.RoleAssistant, Content: "ok"})
	return nil
}

func TestSubmitToTabHistoryDisplaysRawInputAfterMemoryCompose(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "memory-display.jsonl")
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	runner := &appendingDesktopRunner{session: sess, started: make(chan string, 1)}
	ctrl := control.New(control.Options{
		Runner:      runner,
		Executor:    exec,
		Sink:        event.Discard,
		SessionDir:  dir,
		SessionPath: path,
		Label:       "test",
	})
	defer ctrl.Close()

	app := NewApp()
	app.setTestCtrl(ctrl, "deepseek/test")
	ctrl.QueueMemory(`Saved memory "kwu-contributions": contribution count updated`)

	const prompt = "?????"
	app.SubmitToTab("test", prompt)
	composed := <-runner.started
	waitNotRunning(t, ctrl)

	if !strings.Contains(composed, "<memory-update>") || !strings.HasSuffix(composed, prompt) {
		t.Fatalf("model input should include memory update followed by prompt, got %q", composed)
	}
	got := mustHistoryForTab(t, app, "test")
	if len(got) < 2 {
		t.Fatalf("history length = %d, want user + assistant", len(got))
	}
	if got[0].Role != "system" || got[1].Role != "user" {
		t.Fatalf("history roles = %+v, want system then user", got[:min(len(got), 2)])
	}
	if got[1].Content != prompt {
		t.Fatalf("displayed user content = %q, want %q", got[1].Content, prompt)
	}
	if strings.Contains(got[1].Content, "<memory-update>") {
		t.Fatalf("displayed user content leaked memory update: %q", got[1].Content)
	}
}

func TestForkCreatesActiveTabWithoutSwitchingSourceController(t *testing.T) {
	isolateDesktopUserDirs(t)

	workspace := robustTempDir(t)
	if err := os.WriteFile(filepath.Join(workspace, "kwu.toml"), []byte(""), 0o644); err != nil {
		t.Fatalf("write workspace config: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := agent.NewSessionPath(dir, "test")
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	runner := &appendingDesktopRunner{session: sess, started: make(chan string, 2)}
	ctrl := control.New(control.Options{
		Runner:        runner,
		Executor:      exec,
		Sink:          event.Discard,
		SessionDir:    dir,
		SessionPath:   path,
		Label:         "test",
		WorkspaceRoot: workspace,
	})
	app := NewApp()
	app.setTestCtrl(ctrl, "deepseek/test")
	app.tabRuntimes["test"].Scope = "project"
	app.tabRuntimes["test"].WorkspaceRoot = workspace
	app.tabRuntimes["test"].TopicID = "topic_source"
	app.tabRuntimes["test"].TopicTitle = "Source topic"
	app.tabRuntimes["test"].billingSource = string(modelbilling.BillingNewAPIByok)
	effort := "high"
	app.tabRuntimes["test"].effort = &effort
	defer ctrl.Close()

	ctrl.Submit("first")
	<-runner.started
	waitNotRunning(t, ctrl)
	ctrl.Submit("second")
	<-runner.started
	waitNotRunning(t, ctrl)
	if got := len(ctrl.History()); got != 5 {
		t.Fatalf("source history len before fork = %d, want 5", got)
	}

	meta, err := app.Fork(1)
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if !meta.Active || meta.ID == "" || meta.ID == "test" {
		t.Fatalf("fork meta = %+v, want a new active tab", meta)
	}
	if got := app.cachedActiveTabID(); got != meta.ID {
		t.Fatalf("active tab = %q, want fork tab %q", got, meta.ID)
	}
	if got := ctrl.SessionPath(); got != path {
		t.Fatalf("source controller session path = %q, want %q", got, path)
	}
	if got := len(ctrl.History()); got != 5 {
		t.Fatalf("source history len after fork = %d, want 5", got)
	}
	if got, want := meta.TopicTitle, "Source topic · 分叉"; got != want {
		t.Fatalf("fork topic title = %q, want %q", got, want)
	}

	var forkPath string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read session dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		candidate := filepath.Join(dir, entry.Name())
		if candidate == path {
			continue
		}
		m, ok, err := agent.LoadBranchMeta(candidate)
		if err != nil {
			t.Fatalf("load fork meta: %v", err)
		}
		if ok && m.TopicID == meta.TopicID {
			forkPath = candidate
			if m.ParentID != agent.BranchID(path) || m.ForkTurn != 1 || m.ForkMessageIndex != 3 {
				t.Fatalf("fork branch meta = %+v, want parent %q turn 1 index 3", m, agent.BranchID(path))
			}
			if m.Scope != "project" || m.WorkspaceRoot != workspace || m.TopicTitle != "Source topic · 分叉" {
				t.Fatalf("fork topic meta = %+v", m)
			}
			if m.Model != "deepseek/test" || m.BillingSource != string(modelbilling.BillingNewAPIByok) || m.Effort == nil || *m.Effort != effort {
				t.Fatalf("fork composer meta = %+v", m)
			}
		}
	}
	if forkPath == "" {
		t.Fatalf("fork session with topic %q not found in %s", meta.TopicID, dir)
	}
	forkTab := app.tabByID(meta.ID)
	if forkTab == nil || forkTab.billingSource != string(modelbilling.BillingNewAPIByok) || forkTab.effort == nil || *forkTab.effort != effort {
		t.Fatalf("fork tab composer state = %+v", forkTab)
	}
}

func TestCapabilitiesShowsDefaultMCPAsAutomaticIdleNotDisabled(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "playwright"
command = "npx"
args = ["-y", "@playwright/mcp"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "playwright" {
			if s.Status != "deferred" || s.StartIntent != "automatic" || s.RuntimeState != "idle" {
				t.Fatalf("default MCP view = %+v, want deferred automatic idle", s)
			}
			return
		}
	}
	t.Fatalf("playwright MCP missing from Capabilities: %+v", view.Servers)
}

func TestDesktopSharedHostBackgroundMCPAutoConnectsOnBoot(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	srv := desktopMCPHTTPServer(t)
	defer srv.Close()
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(fmt.Sprintf(`
[[plugins]]
name = "h"
type = "http"
url = %q
`, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sharedHost := plugin.NewHost()
	defer sharedHost.Close()
	ctrl, err := boot.Build(ctx, boot.Options{
		WorkspaceRoot: dir,
		SessionDir:    filepath.Join(dir, "sessions"),
		SharedHost:    sharedHost,
		Stderr:        io.Discard,
	})
	if err != nil {
		t.Fatalf("boot.Build: %v", err)
	}
	defer ctrl.Close()

	deadline := time.Now().Add(3 * time.Second)
	for !sharedHost.HasClient("h") && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if !sharedHost.HasClient("h") {
		t.Fatalf("background MCP did not auto-connect; connecting=%v failures=%+v", sharedHost.ConnectingServers(), sharedHost.Failures())
	}

	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"test": {
			ID:            "test",
			Scope:         "global",
			WorkspaceRoot: dir,
			Ready:         true,
			Ctrl:          ctrl,
			SharedHostKey: dir,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "test"

	view := app.MCPServers()
	if len(view) != 1 || view[0].Name != "h" || view[0].Status != "connected" || view[0].StartIntent != "automatic" || view[0].RuntimeState != "ready" || view[0].Tools != 1 {
		t.Fatalf("MCPServers() = %+v, want h connected automatic ready with one tool", view)
	}
}

func TestDesktopSharedHostExtraMCPRemainsReusableAcrossTaskBuilds(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	srv := desktopMCPHTTPServer(t)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sharedHost := plugin.NewHost()
	defer sharedHost.Close()
	specs := []plugin.Spec{{Name: "managed", Type: "streamable-http", URL: srv.URL}}

	first, err := boot.Build(ctx, boot.Options{
		WorkspaceRoot: dir,
		SessionDir:    filepath.Join(dir, "sessions", "first"),
		SharedHost:    sharedHost,
		ExtraPlugins:  specs,
		Stderr:        io.Discard,
	})
	if err != nil {
		t.Fatalf("first boot.Build: %v", err)
	}
	defer first.Close()

	if tools, err := sharedHost.ToolsFor(ctx, "managed"); err != nil || len(tools) != 1 {
		t.Fatalf("ToolsFor after first build = %d, %v; want one reusable tool", len(tools), err)
	}

	second, err := boot.Build(ctx, boot.Options{
		WorkspaceRoot: dir,
		SessionDir:    filepath.Join(dir, "sessions", "second"),
		SharedHost:    sharedHost,
		ExtraPlugins:  specs,
		Stderr:        io.Discard,
	})
	if err != nil {
		t.Fatalf("second boot.Build: %v", err)
	}
	defer second.Close()
	if !second.UnregisterMCPServerTools("managed") {
		t.Fatal("second controller registry did not contain the reused managed tool")
	}

	if tools, err := sharedHost.ToolsFor(ctx, "managed"); err != nil || len(tools) != 1 {
		t.Fatalf("ToolsFor after second build = %d, %v; want one reused tool", len(tools), err)
	}
}

func TestMCPServersMatchesCapabilitiesServerProjection(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "playwright"
command = "npx"
args = ["-y", "@playwright/mcp"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if got, want := app.MCPServers(), app.Capabilities().Servers; !reflect.DeepEqual(got, want) {
		t.Fatalf("MCPServers() = %+v, want Capabilities().Servers %+v", got, want)
	}
}

func TestMCPSettingsUseActiveTabProfileConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	defaultActor := userctx.Default(dir)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultCfg := config.Default()
	defaultCfg.Plugins = []config.PluginEntry{{Name: "default-only", Command: "default-cmd"}}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.Default(dir)
	workActor.ProfileID = "work"
	workPaths := config.NewProfilePathResolver(workActor)
	workCfg := config.Default()
	workCfg.Plugins = []config.PluginEntry{{Name: "work-only", Command: "work-cmd"}}
	if err := workCfg.SaveToScope(workPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save work config: %v", err)
	}

	ctrl := control.New(control.Options{Actor: workActor, WorkspaceRoot: dir, Host: plugin.NewHost()})
	defer ctrl.Close()
	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"work": {
		ID:            "work",
		Scope:         "project",
		WorkspaceRoot: dir,
		Ready:         true,
		Ctrl:          ctrl,
		disabledMCP:   map[string]ServerView{},
	}}
	app.activeTabID = "work"

	view := app.MCPServers()
	if !serverViewHasName(view, "work-only") {
		t.Fatalf("MCPServers missing work profile plugin: %+v", view)
	}
	if serverViewHasName(view, "default-only") {
		t.Fatalf("MCPServers leaked default profile plugin: %+v", view)
	}
	if _, found, err := app.desktopMCPServerForEdit("default-only"); err != nil || found {
		t.Fatalf("desktopMCPServerForEdit(default-only) found=%v err=%v, want not found", found, err)
	}
	if p, found, err := app.desktopMCPServerForEdit("work-only"); err != nil || !found || p.Command != "work-cmd" {
		t.Fatalf("desktopMCPServerForEdit(work-only) = %+v found=%v err=%v", p, found, err)
	}
}

func TestEnsureSelectedMCPServersReconnectsFromTabProfileConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	srv := desktopMCPHTTPServer(t)
	defer srv.Close()

	defaultActor := userctx.Default(dir)
	defaultPaths := config.NewProfilePathResolver(defaultActor)
	defaultCfg := config.Default()
	defaultCfg.Plugins = []config.PluginEntry{{Name: "default-only", Type: "http", URL: srv.URL}}
	if err := defaultCfg.SaveToScope(defaultPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	workActor := userctx.Default(dir)
	workActor.ProfileID = "work"
	workPaths := config.NewProfilePathResolver(workActor)
	workCfg := config.Default()
	workCfg.Plugins = []config.PluginEntry{{Name: "work-only", Type: "http", URL: srv.URL}}
	if err := workCfg.SaveToScope(workPaths.UserConfigPath(), config.RenderScopeUser); err != nil {
		t.Fatalf("save work config: %v", err)
	}

	ctrl := control.New(control.Options{
		Actor:         workActor,
		WorkspaceRoot: dir,
		Host:          plugin.NewHost(),
		Registry:      tool.NewRegistry(),
		PluginCtx:     context.Background(),
	})
	defer ctrl.Close()
	tab := &WorkspaceTab{
		ID:            "work",
		Scope:         "project",
		WorkspaceRoot: dir,
		Ready:         true,
		Ctrl:          ctrl,
		disabledMCP:   map[string]ServerView{},
	}
	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{"work": tab}
	app.activeTabID = "work"

	selected, err := app.ensureSelectedMCPServers(tab, ctrl, []string{"work-only"})
	if err != nil {
		t.Fatalf("ensureSelectedMCPServers(work-only): %v", err)
	}
	if len(selected) != 1 || selected[0] != "work-only" {
		t.Fatalf("selected servers = %v, want [work-only]", selected)
	}
	if tools := connectedMCPToolCounts(ctrl)["work-only"]; tools != 1 {
		t.Fatalf("connected tools = %d, want 1", tools)
	}
	if !ctrl.Host().HasClient("work-only") {
		t.Fatal("work profile MCP server was not connected")
	}
	if _, err := app.ensureSelectedMCPServers(tab, ctrl, []string{"default-only"}); err == nil || !strings.Contains(err.Error(), "no configured MCP server") {
		t.Fatalf("ensureSelectedMCPServers(default-only) err = %v, want no configured MCP server", err)
	}
}

func serverViewHasName(servers []ServerView, name string) bool {
	for _, s := range servers {
		if s.Name == name {
			return true
		}
	}
	return false
}

func TestConfiguredMCPWithFormerBuiltInNameIsUserServer(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "time"
command = "custom-time"
args = ["serve"]
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	view := app.Capabilities()
	found := false
	for _, s := range view.Servers {
		if s.Name != "time" {
			continue
		}
		found = true
		if s.BuiltIn || !s.Configured || s.Command != "custom-time" || !reflect.DeepEqual(s.Args, []string{"serve"}) {
			t.Fatalf("configured time view = %+v, want ordinary user MCP config", s)
		}
	}
	if !found {
		t.Fatalf("configured time server missing from Capabilities: %+v", view.Servers)
	}

	if err := app.SetMCPServerEnabled("time", false); err != nil {
		t.Fatalf("SetMCPServerEnabled(time,false): %v", err)
	}
	view = app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "time" {
			if s.Status != "disabled" || s.BuiltIn || s.Command != "custom-time" {
				t.Fatalf("disabled configured time view = %+v, want disabled external config", s)
			}
			return
		}
	}
	t.Fatalf("time missing after disable: %+v", view.Servers)
}

func TestSetMCPServerEnabledSharedHostPreservesSiblingTabs(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	srv := desktopMCPHTTPServer(t)
	defer srv.Close()
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(fmt.Sprintf(`
[[plugins]]
name = "h"
type = "http"
url = %q
`, srv.URL)), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sharedHost := plugin.NewHost()
	defer sharedHost.Close()
	tools, err := sharedHost.Add(ctx, plugin.Spec{Name: "h", Type: "http", URL: srv.URL})
	if err != nil {
		t.Fatalf("sharedHost.Add: %v", err)
	}

	activeRegistry := tool.NewRegistry()
	siblingRegistry := tool.NewRegistry()
	for _, mt := range tools {
		activeRegistry.Add(mt)
		siblingRegistry.Add(mt)
	}
	activeCtrl := control.New(control.Options{Host: sharedHost, Registry: activeRegistry, PluginCtx: context.Background()})
	siblingCtrl := control.New(control.Options{Host: sharedHost, Registry: siblingRegistry, PluginCtx: context.Background()})
	app := NewApp()
	app.tabRuntimes = map[string]*WorkspaceTab{
		"active": {
			ID:            "active",
			Scope:         "global",
			WorkspaceRoot: dir,
			Ready:         true,
			Ctrl:          activeCtrl,
			SharedHostKey: dir,
			disabledMCP:   map[string]ServerView{},
		},
		"sibling": {
			ID:            "sibling",
			Scope:         "global",
			WorkspaceRoot: dir,
			Ready:         true,
			Ctrl:          siblingCtrl,
			SharedHostKey: dir,
			disabledMCP:   map[string]ServerView{},
		},
	}
	app.activeTabID = "active"

	if err := app.SetMCPServerEnabled("h", true); err != nil {
		t.Fatalf("SetMCPServerEnabled(h,true) for an already connected server: %v", err)
	}
	if _, found := activeRegistry.Get("mcp__h__greet"); !found {
		t.Fatal("idempotent enable removed the active connector tools")
	}

	if err := app.SetMCPServerEnabled("h", false); err != nil {
		t.Fatalf("SetMCPServerEnabled(h,false): %v", err)
	}
	if _, found := activeRegistry.Get("mcp__h__greet"); found {
		t.Fatal("active tab still has h tools after disabling the shared server")
	}
	if _, found := siblingRegistry.Get("mcp__h__greet"); !found {
		t.Fatal("sibling tab lost h tools when active tab disabled the shared server")
	}
	if !sharedHost.HasClient("h") {
		t.Fatal("shared host client was removed by a per-tab disable")
	}
	view := app.Capabilities()
	if len(view.Servers) != 1 || view.Servers[0].Name != "h" || view.Servers[0].Status != "disabled" {
		t.Fatalf("Capabilities after disable = %+v, want h disabled for the active tab", view.Servers)
	}

	if err := app.SetMCPServerEnabled("h", true); err != nil {
		t.Fatalf("SetMCPServerEnabled(h,true): %v", err)
	}
	if _, found := activeRegistry.Get("mcp__h__greet"); !found {
		t.Fatal("active tab did not re-register h tools from the existing shared client")
	}
	view = app.Capabilities()
	if len(view.Servers) != 1 || view.Servers[0].Name != "h" || view.Servers[0].Status != "connected" {
		t.Fatalf("Capabilities after re-enable = %+v, want h connected for the active tab", view.Servers)
	}
}

func TestSetMCPServerEnabledRejectsBackgroundJobs(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	app.setTestCtrl(newBackgroundJobController(t, "mcp-enabled-job"), "")

	err := app.SetMCPServerEnabled("time", false)
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("SetMCPServerEnabled with background job error = %v, want active-work guard", err)
	}
	if tab := app.activeTab(); tab == nil || len(tab.disabledMCP) != 0 {
		t.Fatalf("disabled MCP state changed after rejected toggle: %+v", tab)
	}
}

func TestEditAndRemoveConfiguredMCPWithBuiltInName(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "time"
command = "custom-time"
args = ["serve"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if err := app.UpdateMCPServer("time", MCPServerInput{
		Name:      "time",
		Transport: "stdio",
		Command:   "updated-time",
		Args:      []string{"run"},
	}); err != nil {
		t.Fatalf("UpdateMCPServer(time): %v", err)
	}
	cfg, err := config.LoadForRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	updated, ok := findPluginEntry(cfg.Plugins, "time")
	if !ok || updated.Command != "updated-time" || !reflect.DeepEqual(updated.Args, []string{"run"}) {
		t.Fatalf("updated time plugin = %+v, found=%v", updated, ok)
	}

	if err := app.RemoveMCPServer("time"); err != nil {
		t.Fatalf("RemoveMCPServer(time): %v", err)
	}
	cfg, err = config.LoadForRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findPluginEntry(cfg.Plugins, "time"); ok {
		t.Fatalf("time plugin still configured after remove: %+v", cfg.Plugins)
	}
}

func TestRemoveMCPServerDeletesProjectMCPJSONEntry(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{
  "mcpServers": {
    "codegraph": { "command": "codegraph", "args": ["serve", "--mcp"] },
    "keep": { "command": "keep-mcp" }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if err := app.RemoveMCPServer("codegraph"); err != nil {
		t.Fatalf("RemoveMCPServer(.mcp.json codegraph): %v", err)
	}
	cfg, err := config.LoadForRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findPluginEntry(cfg.Plugins, "codegraph"); ok {
		t.Fatalf("codegraph still merged after remove: %+v", cfg.Plugins)
	}
	if _, ok := findPluginEntry(cfg.Plugins, "keep"); !ok {
		t.Fatalf("unrelated .mcp.json server should be preserved: %+v", cfg.Plugins)
	}
}

func TestRemoveMCPServerClearsOrphanedFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)

	host := plugin.NewHost()
	host.RecordFailure(
		plugin.Spec{Name: "11", Type: "stdio", Command: "missing-command"},
		errors.New(`no configured MCP server named "11"`),
	)
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: host}), "")
	defer app.activeCtrl().Close()

	if err := app.RemoveMCPServer("11"); err != nil {
		t.Fatalf("RemoveMCPServer(orphaned failure): %v", err)
	}
	if failures := host.Failures(); len(failures) != 0 {
		t.Fatalf("orphaned failure should be cleared: %+v", failures)
	}
	for _, server := range app.MCPServers() {
		if server.Name == "11" {
			t.Fatalf("orphaned failure still visible after remove: %+v", server)
		}
	}
	if err := app.RemoveMCPServer("11"); err == nil {
		t.Fatal("removing a fully missing MCP server should still fail")
	}
}

func TestUpdateMCPServerEditsProjectMCPJSONEntry(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{
  "mcpServers": {
    "codegraph": { "command": "codegraph", "args": ["serve", "--mcp"] }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if err := app.UpdateMCPServer("codegraph", MCPServerInput{
		Name:      "codegraph",
		Transport: "stdio",
		Command:   "kwu-missing-mcp-binary",
		Args:      []string{"serve", "--mcp"},
		Env:       map[string]string{"CODEGRAPH_LOG": "debug"},
	}); err != nil {
		t.Fatalf("UpdateMCPServer(.mcp.json codegraph): %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.MCPServers["codegraph"]
	if got.Command != "kwu-missing-mcp-binary" || !reflect.DeepEqual(got.Args, []string{"serve", "--mcp"}) || got.Env["CODEGRAPH_LOG"] != "debug" {
		t.Fatalf(".mcp.json codegraph = %+v, want updated command/args/env", got)
	}
	if _, ok := findPluginEntry(config.LoadForEdit(config.UserConfigPath()).Plugins, "codegraph"); ok {
		t.Fatalf(".mcp.json update should not create a user config shadow entry")
	}
}

func TestTrustMCPServerToolPersistsTrustedReadOnlyTools(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "github"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]
trusted_read_only_tools = ["pull_request_read"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if err := app.TrustMCPServerTool("github", " issue_read "); err != nil {
		t.Fatalf("TrustMCPServerTool(github, issue_read): %v", err)
	}
	if err := app.TrustMCPServerTool("github", "issue_read"); err != nil {
		t.Fatalf("TrustMCPServerTool duplicate: %v", err)
	}
	if err := app.TrustMCPServerTools("github", []string{" search_issues ", "issue_read", ""}); err != nil {
		t.Fatalf("TrustMCPServerTools(github): %v", err)
	}
	if err := app.UntrustMCPServerTool("github", " pull_request_read "); err != nil {
		t.Fatalf("UntrustMCPServerTool(github, pull_request_read): %v", err)
	}
	cfg, err := config.LoadForRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	updated, ok := findPluginEntry(cfg.Plugins, "github")
	if !ok {
		t.Fatalf("github plugin missing: %+v", cfg.Plugins)
	}
	if !reflect.DeepEqual(updated.TrustedReadOnlyTools, []string{"issue_read", "search_issues"}) {
		t.Fatalf("trusted read-only tools = %+v", updated.TrustedReadOnlyTools)
	}
	for _, s := range app.MCPServers() {
		if s.Name == "github" {
			if !reflect.DeepEqual(s.TrustedReadOnlyTools, []string{"issue_read", "search_issues"}) {
				t.Fatalf("view trusted read-only tools = %+v", s.TrustedReadOnlyTools)
			}
			return
		}
	}
	t.Fatalf("github MCP missing from view")
}

func TestTrustMCPServerToolPersistsProjectMCPJSONEntry(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{
  "mcpServers": {
    "codegraph": { "command": "codegraph", "args": ["serve", "--mcp"] }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if err := app.TrustMCPServerTool("codegraph", "codegraph_context"); err != nil {
		t.Fatalf("TrustMCPServerTool(.mcp.json codegraph): %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		MCPServers map[string]struct {
			TrustedReadOnlyTools []string `json:"trusted_read_only_tools"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc.MCPServers["codegraph"].TrustedReadOnlyTools, []string{"codegraph_context"}) {
		t.Fatalf(".mcp.json trusted_read_only_tools = %+v", doc.MCPServers["codegraph"].TrustedReadOnlyTools)
	}
	cfg, err := config.LoadForRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	updated, ok := findPluginEntry(cfg.Plugins, "codegraph")
	if !ok || !reflect.DeepEqual(updated.TrustedReadOnlyTools, []string{"codegraph_context"}) {
		t.Fatalf("merged codegraph trusted_read_only_tools = %+v, found=%v", updated.TrustedReadOnlyTools, ok)
	}
}

func TestAddMCPServerPersistsRemoteHeaders(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	t.Setenv("STRIPE_TOKEN", "stripe-test-token")
	srv := desktopMCPHTTPServer(t)
	defer srv.Close()

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	tools, err := app.AddMCPServer(MCPServerInput{
		Name:      "stripe",
		Transport: "http",
		URL:       srv.URL,
		Headers: map[string]string{
			"Authorization": "Bearer ${STRIPE_TOKEN}",
			"X-Org":         "team",
		},
	})
	if err != nil {
		t.Fatalf("AddMCPServer(stripe): %v", err)
	}
	if tools != 1 {
		t.Fatalf("tools = %d, want 1", tools)
	}

	cfg, err := config.LoadForRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := findPluginEntry(cfg.Plugins, "stripe")
	if !ok {
		t.Fatalf("stripe plugin missing from config: %+v", cfg.Plugins)
	}
	if p.Type != "http" || p.URL != srv.URL {
		t.Fatalf("stripe plugin transport = %q url = %q", p.Type, p.URL)
	}
	if p.Headers["Authorization"] != "Bearer ${STRIPE_TOKEN}" || p.Headers["X-Org"] != "team" {
		t.Fatalf("stripe headers = %+v", p.Headers)
	}

	view := app.MCPServers()
	for _, s := range view {
		if s.Name == "stripe" {
			if !reflect.DeepEqual(s.HeaderKeys, []string{"Authorization", "X-Org"}) {
				t.Fatalf("stripe header keys = %+v", s.HeaderKeys)
			}
			return
		}
	}
	t.Fatalf("stripe MCP missing from view: %+v", view)
}

func TestCapabilitiesMarksBackgroundRemoteMCPAuthPossible(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "dida"
type = "http"
url = "https://mcp.dida365.com"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "dida" {
			if s.Status != "deferred" || s.StartIntent != "automatic" || s.RuntimeState != "idle" || s.AuthStatus != "possible" || s.AuthURL != "https://mcp.dida365.com" {
				t.Fatalf("dida auth diagnosis = %+v", s)
			}
			return
		}
	}
	t.Fatalf("dida MCP missing from Capabilities: %+v", view.Servers)
}

func TestCapabilitiesDoesNotMarkRemoteMCPWithAuthHeaderPossible(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "stripe"
type = "http"
url = "https://mcp.stripe.com"
headers = { Authorization = "Bearer ${STRIPE_TOKEN}" }
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "stripe" {
			if s.AuthStatus != "none" {
				t.Fatalf("stripe auth status = %q, want none; server = %+v", s.AuthStatus, s)
			}
			return
		}
	}
	t.Fatalf("stripe MCP missing from Capabilities: %+v", view.Servers)
}

func TestCapabilitiesMarksAuthFailureRequired(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "figma"
type = "http"
url = "https://mcp.figma.com/mcp"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "figma", Type: "http", URL: "https://mcp.figma.com/mcp"}, errors.New("connect: 401 unauthorized"))
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: host}), "")
	defer app.activeCtrl().Close()

	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "figma" {
			if s.Status != "failed" || s.AuthStatus != "required" || s.AuthURL != "https://mcp.figma.com/mcp" {
				t.Fatalf("figma auth diagnosis = %+v", s)
			}
			return
		}
	}
	t.Fatalf("figma MCP missing from Capabilities: %+v", view.Servers)
}

func TestClearMCPServerAuthenticationClearsConfigAndFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "figma"
type = "http"
url = "https://mcp.figma.com/mcp?access_token=abc&workspace=main"
headers = { Authorization = "Bearer ${FIGMA_TOKEN}", "X-Org" = "team" }
env = { FIGMA_TOKEN = "${FIGMA_TOKEN}", DEBUG = "1" }
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	host := plugin.NewHost()
	host.RecordFailure(plugin.Spec{Name: "figma", Type: "http", URL: "https://mcp.figma.com/mcp"}, errors.New("connect: 401 unauthorized"))
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: host}), "")
	defer app.activeCtrl().Close()

	if err := app.ClearMCPServerAuthentication("figma"); err != nil {
		t.Fatalf("ClearMCPServerAuthentication: %v", err)
	}
	if failures := host.Failures(); len(failures) != 0 {
		t.Fatalf("failure should be cleared: %+v", failures)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Plugins[0]
	if p.URL != "https://mcp.figma.com/mcp?workspace=main" {
		t.Fatalf("url = %q", p.URL)
	}
	if _, ok := p.Headers["Authorization"]; ok {
		t.Fatalf("auth header should be removed: %v", p.Headers)
	}
	if p.Headers["X-Org"] != "team" {
		t.Fatalf("ordinary header should be preserved: %v", p.Headers)
	}
	if _, ok := p.Env["FIGMA_TOKEN"]; ok {
		t.Fatalf("auth env should be removed: %v", p.Env)
	}
	if p.Env["DEBUG"] != "1" {
		t.Fatalf("ordinary env should be preserved: %v", p.Env)
	}
	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "figma" {
			if s.Status != "deferred" || s.StartIntent != "automatic" || s.RuntimeState != "idle" || s.AuthStatus != "possible" {
				t.Fatalf("figma should return to background possible auth: %+v", s)
			}
			return
		}
	}
	t.Fatalf("figma MCP missing from Capabilities: %+v", view.Servers)
}

func TestUpdateMCPServerMigratesLegacyTierToBackground(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "playwright"
command = "npx"
args = ["-y", "@playwright/mcp"]
env = { TOKEN = "${PLAYWRIGHT_TOKEN}" }
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.UpdateMCPServer("playwright", MCPServerInput{
		Name:      "playwright",
		Transport: "stdio",
		Command:   "node",
		Args:      []string{"server.js"},
	}); err != nil {
		t.Fatalf("UpdateMCPServer: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Plugins[0].Command; got != "node" {
		t.Fatalf("updated command = %q, want node", got)
	}
	if got := cfg.Plugins[0].Env["TOKEN"]; got != "${PLAYWRIGHT_TOKEN}" {
		t.Fatalf("env TOKEN = %q, want preserved env", got)
	}
	userCfg := config.LoadForEdit(config.UserConfigPath())
	userPlugin, ok := findPluginEntry(userCfg.Plugins, "playwright")
	if !ok {
		t.Fatalf("playwright should be migrated to user config: %+v", userCfg.Plugins)
	}
	if userPlugin.Command != "node" || userPlugin.Env["TOKEN"] != "${PLAYWRIGHT_TOKEN}" {
		t.Fatalf("user plugin after migration = %+v", userPlugin)
	}
	if userPlugin.Tier != "" {
		t.Fatalf("user plugin tier = %q, want migrated empty", userPlugin.Tier)
	}
	projectCfg := config.LoadForEdit(filepath.Join(dir, "kwu.toml"))
	if _, ok := findPluginEntry(projectCfg.Plugins, "playwright"); ok {
		t.Fatalf("project plugin should be removed after desktop migration: %+v", projectCfg.Plugins)
	}
	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "playwright" {
			if s.Status != "failed" {
				t.Fatalf("updated MCP status = %q, want failed after immediate reconnect attempt; server = %+v", s.Status, s)
			}
			if s.Command != "node" || len(s.Args) != 1 || s.Args[0] != "server.js" {
				t.Fatalf("server command not refreshed: %+v", s)
			}
			return
		}
	}
	t.Fatalf("playwright MCP missing from Capabilities: %+v", view.Servers)
}

func TestUpdateMCPServerSplitsPastedCommandLine(t *testing.T) {
	isolateDesktopUserDirs(t)
	t.Setenv("PATH", "")
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "playwright"
command = "npx"
args = ["-y", "@playwright/mcp"]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()
	app.tabRuntimes["test"].disabledMCP["playwright"] = ServerView{}

	if err := app.UpdateMCPServer("playwright", MCPServerInput{
		Name:      "playwright",
		Transport: "stdio",
		Command:   "npx -y @modelcontextprotocol/server-filesystem .",
	}); err != nil {
		t.Fatalf("UpdateMCPServer: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Plugins[0]
	if p.Command != "npx" {
		t.Fatalf("command = %q, want npx", p.Command)
	}
	if got := strings.Join(p.Args, "\x00"); got != strings.Join([]string{"-y", "@modelcontextprotocol/server-filesystem", "."}, "\x00") {
		t.Fatalf("args = %v", p.Args)
	}
}

func TestUpdateMCPServerRecordsReconnectFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "broken"
command = "npx"
tier = "background"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()

	if err := app.UpdateMCPServer("broken", MCPServerInput{
		Name:      "broken",
		Transport: "stdio",
		Command:   "kwu-missing-mcp-binary",
	}); err != nil {
		t.Fatalf("UpdateMCPServer should persist config even when reconnect fails: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Plugins[0].Command; got != "kwu-missing-mcp-binary" {
		t.Fatalf("updated command = %q, want missing binary", got)
	}
	if got := cfg.Plugins[0].Tier; got != "" {
		t.Fatalf("updated tier = %q, want migrated empty", got)
	}
	if !mcpFailed(app.activeCtrl(), "broken") {
		t.Fatalf("Host.Failures() = %+v, want broken failure recorded", app.activeCtrl().Host().Failures())
	}
	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "broken" {
			if s.Status != "failed" {
				t.Fatalf("server status = %q, want failed; server = %+v", s.Status, s)
			}
			if s.Command != "kwu-missing-mcp-binary" || s.Tier != "background" {
				t.Fatalf("server config not refreshed after failed reconnect: %+v", s)
			}
			return
		}
	}
	t.Fatalf("broken MCP missing from Capabilities: %+v", view.Servers)
}

func TestReconnectMCPServerClearsInitializingPlaceholderAndRecordsFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "codegraph"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	reg := tool.NewRegistry()
	reg.Add(desktopFakeTool{name: "mcp__codegraph__connect"})
	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost(), Registry: reg}), "")
	defer app.activeCtrl().Close()

	view := app.Capabilities()
	foundIdle := false
	for _, s := range view.Servers {
		if s.Name == "codegraph" {
			foundIdle = true
			if s.Status != "deferred" || s.StartIntent != "automatic" || s.RuntimeState != "idle" {
				t.Fatalf("initial codegraph server = %+v, want automatic idle background state", s)
			}
		}
	}
	if !foundIdle {
		t.Fatalf("codegraph missing before reconnect: %+v", view.Servers)
	}
	if _, ok := reg.Get("mcp__codegraph__connect"); !ok {
		t.Fatal("test setup expected stale codegraph connect placeholder")
	}

	if err := app.ReconnectMCPServer("codegraph"); err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("ReconnectMCPServer error = %v, want missing command", err)
	}
	if _, ok := reg.Get("mcp__codegraph__connect"); ok {
		t.Fatalf("stale codegraph placeholder still registered after reconnect failure; names=%v", reg.Names())
	}
	if !mcpFailed(app.activeCtrl(), "codegraph") {
		t.Fatalf("Host.Failures() = %+v, want codegraph failure recorded", app.activeCtrl().Host().Failures())
	}

	view = app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "codegraph" {
			if s.Status != "failed" || s.Error == "" {
				t.Fatalf("codegraph after failed reconnect = %+v, want failed with error", s)
			}
			return
		}
	}
	t.Fatalf("codegraph missing after reconnect: %+v", view.Servers)
}

func TestSetMCPServerTierRecordsConnectFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "broken"
command = "kwu-missing-mcp-binary"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer func() {
		if c := app.activeCtrl(); c != nil {
			c.Close()
		}
	}()

	if err := app.SetMCPServerTier("broken", "background"); err != nil {
		t.Fatalf("SetMCPServerTier legacy binding: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Plugins[0].Tier; got != "" {
		t.Fatalf("saved tier = %q, want migrated empty", got)
	}
	userCfg := config.LoadForEdit(config.UserConfigPath())
	userPlugin, ok := findPluginEntry(userCfg.Plugins, "broken")
	if !ok {
		t.Fatalf("broken should be migrated to user config: %+v", userCfg.Plugins)
	}
	if userPlugin.Tier != "" {
		t.Fatalf("user plugin tier = %q, want migrated empty", userPlugin.Tier)
	}
	projectCfg := config.LoadForEdit(filepath.Join(dir, "kwu.toml"))
	if _, ok := findPluginEntry(projectCfg.Plugins, "broken"); ok {
		t.Fatalf("project plugin should be removed after desktop migration: %+v", projectCfg.Plugins)
	}
	if !mcpFailed(app.activeCtrl(), "broken") {
		t.Fatalf("Host.Failures() = %+v, want broken failure recorded", app.activeCtrl().Host().Failures())
	}
	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "broken" {
			if s.Status != "failed" {
				t.Fatalf("server status = %q, want failed; server = %+v", s.Status, s)
			}
			if s.Tier != "background" {
				t.Fatalf("server tier = %q, want background so radio selection does not jump back", s.Tier)
			}
			return
		}
	}
	t.Fatalf("broken MCP missing from Capabilities: %+v", view.Servers)
}

func TestSetMCPServerTierRejectsBackgroundJobsBeforeSavingConfig(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	// Constructing the real Desktop runtime performs the one-time legacy MCP
	// tier migration. Write the fixture afterwards so this test isolates the
	// active-work guard rather than the independent startup migration.
	app := NewApp()
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
[[plugins]]
name = "broken"
command = "kwu-missing-mcp-binary"
tier = "lazy"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app.setTestCtrl(newBackgroundJobController(t, "mcp-tier-job"), "")

	err := app.SetMCPServerTier("broken", "background")
	if err == nil || !strings.Contains(err.Error(), "stop background jobs") {
		t.Fatalf("SetMCPServerTier with background job error = %v, want active-work guard", err)
	}
	data, readErr := os.ReadFile(config.UserConfigPath())
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if !strings.Contains(string(data), `tier = "lazy"`) {
		t.Fatalf("plugin config changed after rejected tier update:\n%s", data)
	}
}

func TestCapabilitiesMigratesFailedMCPConfiguredTierAfterRestart(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "kwu.toml"), []byte(`
[[plugins]]
name = "broken"
command = "kwu-missing-mcp-binary"
tier = "eager"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.setTestCtrl(control.New(control.Options{Host: plugin.NewHost()}), "")
	defer app.activeCtrl().Close()
	recordMCPFailure(app.activeCtrl(), config.PluginEntry{
		Name:    "broken",
		Command: "kwu-missing-mcp-binary",
		Tier:    "eager",
	}, errors.New("connect: missing binary"))

	view := app.Capabilities()
	for _, s := range view.Servers {
		if s.Name == "broken" {
			if s.Status != "failed" {
				t.Fatalf("server status = %q, want failed; server = %+v", s.Status, s)
			}
			if s.Tier != "background" {
				t.Fatalf("server tier = %q, want migrated background default", s.Tier)
			}
			if !s.Configured {
				t.Fatalf("server configured = false, want true; server = %+v", s)
			}
			return
		}
	}
	t.Fatalf("broken MCP missing from Capabilities: %+v", view.Servers)
}

func TestRunShellForTabRoutesToRequestedTab(t *testing.T) {
	isolateDesktopUserDirs(t)

	activeEvents := make(chan event.Event, 16)
	inactiveEvents := make(chan event.Event, 16)
	activeCtrl := control.New(control.Options{Sink: event.FuncSink(func(e event.Event) { activeEvents <- e })})
	inactiveCtrl := control.New(control.Options{Sink: event.FuncSink(func(e event.Event) { inactiveEvents <- e })})
	defer activeCtrl.Close()
	defer inactiveCtrl.Close()

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"active":   {ID: "active", Scope: "global", Ctrl: activeCtrl, Ready: true},
			"inactive": {ID: "inactive", Scope: "global", Ctrl: inactiveCtrl, Ready: true},
		},
		tabOrder:    []string{"active", "inactive"},
		activeTabID: "active",
	}

	app.RunShellForTab("inactive", "echo route-test")

	sawDispatch := false
	deadline := time.After(3 * time.Second)
	for {
		select {
		case e := <-inactiveEvents:
			if e.Kind == event.ToolDispatch && strings.Contains(e.Tool.Args, "route-test") {
				sawDispatch = true
			}
			if e.Kind == event.TurnDone {
				if !sawDispatch {
					t.Fatal("inactive tab finished without receiving shell dispatch")
				}
				select {
				case active := <-activeEvents:
					t.Fatalf("active tab received event for inactive shell: %+v", active)
				default:
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for inactive shell turn")
		}
	}
}

type blockingRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingRunner) Run(ctx context.Context, _ string) error {
	close(r.started)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.release:
		return nil
	}
}

func startNonCooperativeSessionJob(t *testing.T, jm *jobs.Manager, sessionPath string) func() {
	t.Helper()
	started := make(chan struct{})
	release := make(chan struct{})
	jm.StartForSession(agent.BranchID(sessionPath), "bash", "stuck job", func(ctx context.Context, _ io.Writer) (string, error) {
		close(started)
		<-ctx.Done()
		<-release
		return "", ctx.Err()
	})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background job never started")
	}
	released := false
	return func() {
		if released {
			return
		}
		released = true
		close(release)
	}
}

func waitNotRunning(t *testing.T, ctrl control.SessionAPI) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for ctrl.Running() {
		if time.Now().After(deadline) {
			t.Fatal("controller still running")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func newBackgroundJobController(t *testing.T, label string) *control.Controller {
	t.Helper()
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	path := filepath.Join(dir, label+".jsonl")
	jm := jobs.NewManager(event.Discard)
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "test", Jobs: jm})
	t.Cleanup(ctrl.Close)
	jm.StartForSession(agent.BranchID(path), "bash", label, func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	return ctrl
}

func hasLevel(levels []string, want string) bool {
	for _, level := range levels {
		if level == want {
			return true
		}
	}
	return false
}

func hasCommand(cmds []CommandInfo, name string) bool {
	for _, cmd := range cmds {
		if cmd.Name == name {
			return true
		}
	}
	return false
}

func hasDirEntry(entries []DirEntry, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}

func TestSessionActionsWithoutControllerReturnError(t *testing.T) {
	app := &App{tabRuntimes: map[string]*WorkspaceTab{}}
	if err := app.NewSession(); err == nil {
		t.Error("NewSession with no controller must surface an error, not silently no-op")
	}
	if err := app.ClearSession(); err == nil {
		t.Error("ClearSession with no controller must surface an error")
	}

	app = &App{
		tabRuntimes: map[string]*WorkspaceTab{"t1": {ID: "t1", StartupErr: "boot exploded"}},
		activeTabID: "t1",
	}
	err := app.NewSession()
	if err == nil || !strings.Contains(err.Error(), "boot exploded") {
		t.Errorf("error should carry the tab's startup failure, got %v", err)
	}
}

// --- Prompt history scanning tests ------------------------------------------

func identityPromptDisplay(text string) string { return text }

// TestCollectPromptHistoryEntriesLegacyEvent verifies that the legacy event format
// {"kind":"user.message","text":"..."} is correctly extracted.
func TestCollectPromptHistoryEntriesLegacyEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"kind":"user.message","text":"hello world"}
{"kind":"user.message","text":"second prompt"}
{"kind":"model.final","content":"response"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", entries[0].Text)
	}
	if entries[1].Text != "second prompt" {
		t.Errorf("expected 'second prompt', got %q", entries[1].Text)
	}
	if entries[0].Turn != 0 || entries[1].Turn != 1 {
		t.Errorf("expected turns 0,1; got %d,%d", entries[0].Turn, entries[1].Turn)
	}
	if entries[0].SessionPath != path {
		t.Errorf("expected session path %q, got %q", path, entries[0].SessionPath)
	}
}

// TestCollectPromptHistoryEntriesEarlyEvent verifies that the migrated legacy event
// format {"type":"user.message","text":"..."} is correctly extracted.
func TestCollectPromptHistoryEntriesEarlyEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"user.message","text":"v0 prompt"}
{"type":"model.final","content":"response"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Text != "v0 prompt" {
		t.Errorf("expected 'v0 prompt', got %q", entries[0].Text)
	}
}

// TestCollectPromptHistoryEntriesProviderMessage verifies that the current
// provider.Message format {"role":"user","content":"..."} is correctly extracted.
func TestCollectPromptHistoryEntriesProviderMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello from provider"}
{"role":"assistant","content":"response"}
{"role":"user","content":"another prompt"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "hello from provider" {
		t.Errorf("expected 'hello from provider', got %q", entries[0].Text)
	}
	if entries[1].Text != "another prompt" {
		t.Errorf("expected 'another prompt', got %q", entries[1].Text)
	}
}

// TestCollectPromptHistoryEntriesMixedFormats verifies that both formats in the
// same file are extracted.
func TestCollectPromptHistoryEntriesMixedFormats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"kind":"user.message","text":"legacy prompt"}
{"role":"user","content":"modern prompt"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "legacy prompt" {
		t.Errorf("expected 'legacy prompt', got %q", entries[0].Text)
	}
	if entries[1].Text != "modern prompt" {
		t.Errorf("expected 'modern prompt', got %q", entries[1].Text)
	}
}

func TestCollectPromptHistoryEntriesReadsEventTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	rfcTime := time.Date(2026, 6, 14, 10, 30, 5, 6_000_000, time.UTC)
	if err := os.WriteFile(path, []byte(`{"kind":"user.message","text":"legacy timed","time":1800000000123}
{"role":"user","content":"modern timed","createdAt":`+strconv.Quote(rfcTime.Format(time.RFC3339Nano))+`}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].At != 1800000000123 {
		t.Errorf("numeric event time = %d, want 1800000000123", entries[0].At)
	}
	if entries[1].At != rfcTime.UnixMilli() {
		t.Errorf("RFC3339 event time = %d, want %d", entries[1].At, rfcTime.UnixMilli())
	}
}

// TestCollectPromptHistoryEntriesUsesDisplayResolver verifies history recall uses
// the user-visible prompt text, not the controller-expanded model input.
func TestCollectPromptHistoryEntriesUsesDisplayResolver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	expanded := "<memory-update>\nSaved memory\n</memory-update>\n\nvisible prompt"
	if err := os.WriteFile(path, []byte(`{"role":"user","content":`+strconv.Quote(expanded)+`}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := recordSessionDisplay(dir, path, expanded, "visible prompt"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, sessionDisplayResolver(dir, path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Text != "visible prompt" {
		t.Errorf("expected visible prompt, got %q", entries[0].Text)
	}
}

func TestCollectPromptHistoryEntriesSkipsSyntheticMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"Plan approved — plan mode is off"}
{"role":"user","content":"real prompt"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Text != "real prompt" {
		t.Errorf("expected real prompt, got %q", entries[0].Text)
	}
}

// TestCollectPromptHistoryEntriesNoUserMessages verifies that a file with only
// assistant/tool messages returns no entries.
func TestCollectPromptHistoryEntriesNoUserMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"kind":"model.final","content":"response"}
{"kind":"tool.result","output":"done"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestCollectPromptHistoryEntriesEmptyFile verifies that an empty JSONL file
// returns no entries without error.
func TestCollectPromptHistoryEntriesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := collectPromptHistoryEntries(path, info, identityPromptDisplay)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestScanPromptHistoryFromDir verifies that scanPromptHistoryFromDir scans
// multiple JSONL files and returns prompts newest-first.
func TestScanPromptHistoryFromDir(t *testing.T) {
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"t1": {ID: "t1", Ctrl: nil, WorkspaceRoot: ""}}}
	_ = app

	dir := t.TempDir()
	// Write two session files with different mtimes (sleep to ensure ordering).
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(`{"role":"user","content":"older prompt"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "b.jsonl"), []byte(`{"role":"user","content":"newer prompt"}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := app.scanPromptHistoryFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Newest-first: "newer prompt" should be first.
	if entries[0].Text != "newer prompt" {
		t.Errorf("expected 'newer prompt' first, got %q", entries[0].Text)
	}
	if entries[1].Text != "older prompt" {
		t.Errorf("expected 'older prompt' second, got %q", entries[1].Text)
	}
}

func TestScanPromptHistoryFromDirUsesSessionActivityBeforeEventInterleaving(t *testing.T) {
	app := &App{}
	dir := t.TempDir()
	base := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	early := filepath.Join(dir, "early.jsonl")
	late := filepath.Join(dir, "late.jsonl")

	if err := os.WriteFile(early, []byte(fmt.Sprintf(`{"role":"user","content":"early first","time":%d}
{"role":"assistant","content":"ok"}
{"role":"user","content":"early second","time":%d}
`, base.UnixMilli(), base.Add(time.Minute).UnixMilli())), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(late, []byte(fmt.Sprintf(`{"role":"user","content":"late newest","time":%d}
`, base.Add(2*time.Minute).UnixMilli())), 0o644); err != nil {
		t.Fatal(err)
	}
	// Invert file mtimes: session activity should keep each session grouped
	// before event timestamps are considered within that session.
	if err := os.Chtimes(early, base.Add(3*time.Hour), base.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(late, base.Add(-3*time.Hour), base.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	entries, err := app.scanPromptHistoryFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []string{"early second", "early first", "late newest"}
	for i, w := range want {
		if entries[i].Text != w {
			t.Fatalf("entries[%d] = %q, want %q; all=%+v", i, entries[i].Text, w, entries)
		}
	}
}

func TestScanPromptHistoryFromDirUsesBranchMetaActivityFallback(t *testing.T) {
	app := &App{}
	dir := t.TempDir()
	base := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	early := filepath.Join(dir, "early.jsonl")
	late := filepath.Join(dir, "late.jsonl")

	if err := os.WriteFile(early, []byte(`{"role":"user","content":"early first"}
{"role":"assistant","content":"ok"}
{"role":"user","content":"early second"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(late, []byte(`{"role":"user","content":"late newest"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(early, agent.BranchMeta{
		CreatedAt: base,
		UpdatedAt: base.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(late, agent.BranchMeta{
		CreatedAt: base.Add(time.Minute),
		UpdatedAt: base.Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	// Invert file mtimes: branch UpdatedAt should be the activity clock.
	if err := os.Chtimes(early, base.Add(3*time.Hour), base.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(late, base.Add(-3*time.Hour), base.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	entries, err := app.scanPromptHistoryFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []string{"late newest", "early second", "early first"}
	for i, w := range want {
		if entries[i].Text != w {
			t.Fatalf("entries[%d] = %q, want %q; all=%+v", i, entries[i].Text, w, entries)
		}
	}
}

func TestScanPromptHistoryFromDirSkipsEmptyOrderedSessions(t *testing.T) {
	app := &App{}
	dir := t.TempDir()
	base := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	empty := filepath.Join(dir, "empty.jsonl")
	real := filepath.Join(dir, "real.jsonl")

	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte(`{"role":"user","content":"real prompt"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(empty, agent.BranchMeta{
		CreatedAt: base,
		UpdatedAt: base.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(real, agent.BranchMeta{
		CreatedAt: base,
		UpdatedAt: base,
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := app.scanPromptHistoryFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Text != "real prompt" {
		t.Fatalf("entries = %+v, want only real prompt after skipping empty session", entries)
	}
}

func TestScanPromptHistoryUsesCurrentSessionBeforeCrossSession(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current.jsonl")
	other := filepath.Join(dir, "other.jsonl")
	if err := os.WriteFile(current, []byte(`{"role":"user","content":"current first"}
{"role":"assistant","content":"ok"}
{"role":"user","content":"current second"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte(`{"role":"user","content":"other newest"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	if err := agent.SaveBranchMetaPreserveUpdated(current, agent.BranchMeta{
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(other, agent.BranchMeta{
		CreatedAt: now.Add(time.Minute),
		UpdatedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: current, Label: "test"})
	defer ctrl.Close()
	app.setTestCtrl(ctrl, "")

	result, err := app.ScanPromptHistory("")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("expected current-session entries followed by cross-session fallback, got %d: %+v", len(result.Entries), result.Entries)
	}
	want := []string{"current second", "current first", "other newest"}
	for i, w := range want {
		if result.Entries[i].Text != w {
			t.Fatalf("entries[%d] = %q, want %q; all=%+v", i, result.Entries[i].Text, w, result.Entries)
		}
	}
}

func TestScanPromptHistoryPaginatesCurrentSessionBeforeCrossSession(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current.jsonl")
	other := filepath.Join(dir, "other.jsonl")
	var lines []byte
	for i := range 55 {
		lines = append(lines, []byte(fmt.Sprintf(`{"role":"user","content":"current %d"}
`, i))...)
	}
	if err := os.WriteFile(current, lines, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte(`{"role":"user","content":"other newest"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)
	if err := agent.SaveBranchMetaPreserveUpdated(current, agent.BranchMeta{
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(other, agent.BranchMeta{
		CreatedAt: now.Add(time.Minute),
		UpdatedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: current, Label: "test"})
	defer ctrl.Close()
	app.setTestCtrl(ctrl, "")

	result, err := app.ScanPromptHistory("")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != promptHistoryPageLimit {
		t.Fatalf("expected %d entries, got %d", promptHistoryPageLimit, len(result.Entries))
	}
	if result.Entries[0].Text != "current 54" {
		t.Fatalf("first entry = %q, want current 54", result.Entries[0].Text)
	}
	if result.Entries[len(result.Entries)-1].Text != "current 5" {
		t.Fatalf("last first-page entry = %q, want current 5", result.Entries[len(result.Entries)-1].Text)
	}
	if !result.HasOlder || result.OlderCursor == "" {
		t.Fatalf("first page should expose an older cursor: %+v", result)
	}
	for _, entry := range result.Entries {
		if entry.Text == "other newest" {
			t.Fatalf("cross-session entry appeared before current-session page was exhausted: %+v", result.Entries)
		}
	}

	nextRequest, err := json.Marshal(promptHistoryRequest{Cursor: result.OlderCursor})
	if err != nil {
		t.Fatal(err)
	}
	next, err := app.ScanPromptHistory(string(nextRequest))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"current 4", "current 3", "current 2", "current 1", "current 0", "other newest"}
	if len(next.Entries) != len(want) {
		t.Fatalf("second page entries = %+v, want %d entries", next.Entries, len(want))
	}
	for i, w := range want {
		if next.Entries[i].Text != w {
			t.Fatalf("second page entries[%d] = %q, want %q; all=%+v", i, next.Entries[i].Text, w, next.Entries)
		}
	}
}

func TestScanPromptHistoryFromDirReadsAllEntriesForInternalHelper(t *testing.T) {
	app := &App{}
	dir := t.TempDir()
	var lines []byte
	for i := range 250 {
		lines = append(lines, []byte(fmt.Sprintf(`{"role":"user","content":"prompt %d"}
`, i))...)
	}
	if err := os.WriteFile(filepath.Join(dir, "many.jsonl"), lines, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := app.scanPromptHistoryFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 250 {
		t.Fatalf("expected 250 entries, got %d", len(entries))
	}
	if entries[0].Text != "prompt 249" {
		t.Errorf("expected newest 'prompt 249' first, got %q", entries[0].Text)
	}
}

// TestScanPromptHistoryFromDirEmpty verifies an empty directory returns nil.
func TestScanPromptHistoryFromDirEmpty(t *testing.T) {
	app := &App{}
	dir := t.TempDir()
	entries, err := app.scanPromptHistoryFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestScanPromptHistoryCacheHit verifies that ScanPromptHistory returns nil
// on cache hit (nonce matches).
func TestScanPromptHistoryCacheHit(t *testing.T) {
	app := &App{tabRuntimes: map[string]*WorkspaceTab{}}
	result, err := app.ScanPromptHistory("")
	if err != nil {
		t.Fatal(err)
	}
	nonce := result.Nonce
	if nonce == "" {
		t.Error("expected a non-empty nonce on first call")
	}

	// Second call with the same nonce should be a cache hit (nil entries).
	result2, err := app.ScanPromptHistory(nonce)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Entries != nil {
		t.Error("expected nil entries on cache hit")
	}
	if result2.Nonce != nonce {
		t.Errorf("expected nonce %q unchanged, got %q", nonce, result2.Nonce)
	}
}

func TestScanPromptHistoryCacheIsScopedBySessionDir(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	pathA := filepath.Join(dirA, "a.jsonl")
	pathB := filepath.Join(dirB, "b.jsonl")
	if err := os.WriteFile(pathA, []byte(`{"role":"user","content":"workspace A"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte(`{"role":"user","content":"workspace B"}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	ctrlA := control.New(control.Options{SessionDir: dirA, SessionPath: pathA, Label: "test"})
	ctrlB := control.New(control.Options{SessionDir: dirB, SessionPath: pathB, Label: "test"})
	defer ctrlA.Close()
	defer ctrlB.Close()

	app.setTestCtrl(ctrlA, "")
	first, err := app.ScanPromptHistory("")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Entries) != 1 || first.Entries[0].Text != "workspace A" {
		t.Fatalf("first entries = %+v, want workspace A", first.Entries)
	}

	app.setTestCtrl(ctrlB, "")
	second, err := app.ScanPromptHistory(first.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	if second.Entries == nil {
		t.Fatal("expected rescan after session dir changes, got cache hit")
	}
	if len(second.Entries) != 1 || second.Entries[0].Text != "workspace B" {
		t.Fatalf("second entries = %+v, want workspace B", second.Entries)
	}
}

func TestScanPromptHistoryCacheIsScopedBySessionPath(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.jsonl")
	pathB := filepath.Join(dir, "b.jsonl")
	if err := os.WriteFile(pathA, []byte(`{"role":"user","content":"session A"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte(`{"role":"user","content":"session B"}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	ctrlA := control.New(control.Options{SessionDir: dir, SessionPath: pathA, Label: "test"})
	ctrlB := control.New(control.Options{SessionDir: dir, SessionPath: pathB, Label: "test"})
	defer ctrlA.Close()
	defer ctrlB.Close()

	app.setTestCtrl(ctrlA, "")
	first, err := app.ScanPromptHistory("")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Entries) != 2 || first.Entries[0].Text != "session A" || first.Entries[1].Text != "session B" {
		t.Fatalf("first entries = %+v, want session A followed by session B", first.Entries)
	}

	app.setTestCtrl(ctrlB, "")
	second, err := app.ScanPromptHistory(first.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	if second.Entries == nil {
		t.Fatal("expected rescan after session path changes, got cache hit")
	}
	if len(second.Entries) != 2 || second.Entries[0].Text != "session B" || second.Entries[1].Text != "session A" {
		t.Fatalf("second entries = %+v, want session B followed by session A", second.Entries)
	}
}
