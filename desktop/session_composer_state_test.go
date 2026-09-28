package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func configureSessionComposerTestProviders(t *testing.T) {
	t.Helper()
	if _, err := config.SetCredential("KWU_TEST_KEY", "sk-test"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`default_model = "provider-a/deepseek-v4-flash"
[desktop]
provider_access = ["provider-a", "provider-b"]
default_billing_source = "newapi_byok"
[[providers]]
name = "provider-a"
kind = "openai"
base_url = "https://a.invalid/v1"
model = "deepseek-v4-flash"
api_key_env = "KWU_TEST_KEY"
[[providers]]
name = "provider-b"
kind = "openai"
base_url = "https://b.invalid/v1"
model = "deepseek-v4-flash"
api_key_env = "KWU_TEST_KEY"
`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSessionComposerStateInfersLegacySourceAndAutoEffort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.jsonl")
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Model: "custom/deepseek-v4"}); err != nil {
		t.Fatal(err)
	}
	state, ok := loadSessionComposerState(path)
	if !ok || state.model != "custom/deepseek-v4" || state.billingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("legacy state = %+v, %v", state, ok)
	}
	if state.effort == nil || *state.effort != "" {
		t.Fatalf("legacy effort = %#v, want auto", state.effort)
	}
	if state.toolApprovalMode != control.ToolApprovalAsk || state.toolApprovalModeKnown {
		t.Fatalf("legacy approval = %q known=%v, want safe missing ask", state.toolApprovalMode, state.toolApprovalModeKnown)
	}
	state, ok = loadSessionComposerStateWithFallback(path, "kwu/gpt-5.4", string(modelbilling.BillingSubscriptionPool), nil)
	if !ok || state.billingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("legacy sidecar reused incompatible fallback route: %+v, %v", state, ok)
	}
}

func TestLoadSessionComposerStateRestoresApprovalWithoutLegacyModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "approval-only.jsonl")
	auto := control.ToolApprovalAuto
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{ToolApprovalMode: &auto}); err != nil {
		t.Fatal(err)
	}
	state, ok := loadSessionComposerState(path)
	if !ok || state.identityKnown || !state.toolApprovalModeKnown || state.toolApprovalMode != control.ToolApprovalAuto {
		t.Fatalf("approval-only state = %+v, %v", state, ok)
	}
	tab := &WorkspaceTab{model: "provider-a/model", billingSource: string(modelbilling.BillingNewAPIByok), toolApprovalMode: control.ToolApprovalYolo}
	applySessionComposerState(tab, state)
	if tab.model != "provider-a/model" || tab.billingSource != string(modelbilling.BillingNewAPIByok) || tab.toolApprovalMode != control.ToolApprovalAuto {
		t.Fatalf("approval-only apply changed identity or lost posture: %+v", tab)
	}
}

func TestSaveTabSessionMetaDoesNotBackfillLegacyMissingApproval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-missing.jsonl")
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Model: "custom/model"}); err != nil {
		t.Fatal(err)
	}
	tab := &WorkspaceTab{model: "custom/model", toolApprovalMode: control.ToolApprovalAsk, toolApprovalModeKnown: false}
	if err := saveTabSessionMeta(tab, path); err != nil {
		t.Fatal(err)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.ToolApprovalMode != nil {
		t.Fatalf("legacy missing approval was backfilled: %+v, %v, %v", meta.ToolApprovalMode, ok, err)
	}

	tab.toolApprovalModeKnown = true
	if err := saveTabSessionMeta(tab, path); err != nil {
		t.Fatal(err)
	}
	meta, ok, err = agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.ToolApprovalMode == nil || *meta.ToolApprovalMode != control.ToolApprovalAsk {
		t.Fatalf("explicit ask was not persisted: %+v, %v, %v", meta.ToolApprovalMode, ok, err)
	}
}

func TestRestoreTabsPrefersSessionComposerStateOverStaleTabEntry(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureSessionComposerTestProviders(t)
	app := NewApp()
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session-b.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	high := "high"
	auto := control.ToolApprovalAuto
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Model: "provider-b/deepseek-v4-flash", BillingSource: string(modelbilling.BillingNewAPIByok), Effort: &high, ToolApprovalMode: &auto}); err != nil {
		t.Fatal(err)
	}
	low := "low"
	state := desktopTabsFile{Tabs: []desktopTabEntry{{
		ID: "restored", Scope: "global", WorkspaceRoot: root, SessionPath: path,
		Model: "provider-a/deepseek-v4-flash", BillingSource: string(modelbilling.BillingSubscriptionPool), Effort: &low,
		ToolApprovalMode: control.ToolApprovalYolo,
	}}, ActiveTab: "restored"}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(app.desktopConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.desktopConfigDir(), tabsFileName), body, 0o644); err != nil {
		t.Fatal(err)
	}
	app.restoreOrBuildTabs()
	tab := app.tabByID("restored")
	if tab == nil || tab.model != "provider-b/deepseek-v4-flash" || tab.billingSource != string(modelbilling.BillingNewAPIByok) || tab.effort == nil || *tab.effort != high || tab.toolApprovalMode != control.ToolApprovalAuto || !tab.toolApprovalModeKnown {
		t.Fatalf("restored tab state = %+v", tab)
	}
}

func TestRestoreTabsKeepsLegacyMissingApprovalAbsent(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureSessionComposerTestProviders(t)
	app := NewApp()
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "legacy-session.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{Model: "provider-a/deepseek-v4-flash", BillingSource: string(modelbilling.BillingNewAPIByok)}); err != nil {
		t.Fatal(err)
	}
	state := desktopTabsFile{Tabs: []desktopTabEntry{{
		ID: "restored", Scope: "global", WorkspaceRoot: root, SessionPath: path,
		Model: "provider-a/deepseek-v4-flash", BillingSource: string(modelbilling.BillingNewAPIByok),
		ToolApprovalMode: control.ToolApprovalYolo,
	}}, ActiveTab: "restored"}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(app.desktopConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app.desktopConfigDir(), tabsFileName), body, 0o644); err != nil {
		t.Fatal(err)
	}

	app.restoreOrBuildTabs()
	tab := app.tabByID("restored")
	if tab == nil || tab.toolApprovalMode != control.ToolApprovalAsk || tab.toolApprovalModeKnown {
		t.Fatalf("restored legacy approval = tab=%+v", tab)
	}
	if err := saveTabSessionMeta(tab, path); err != nil {
		t.Fatal(err)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.ToolApprovalMode != nil {
		t.Fatalf("legacy approval was backfilled = %+v, %v, %v", meta.ToolApprovalMode, ok, err)
	}
	saved := loadTabsFileFromDir(app.desktopConfigDir())
	if len(saved.Tabs) != 1 || saved.Tabs[0].ToolApprovalMode != "" {
		t.Fatalf("desktop tab snapshot reintroduced legacy approval = %+v", saved.Tabs)
	}
}

func TestRebindTabRestoresEachSessionComposerState(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureSessionComposerTestProviders(t)
	app := NewApp()
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSession := func(name, model, effort, approval string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := agent.SaveBranchMeta(path, agent.BranchMeta{Model: model, BillingSource: string(modelbilling.BillingNewAPIByok), Effort: &effort, ToolApprovalMode: &approval}); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pathA := writeSession("session-a.jsonl", "provider-a/deepseek-v4-flash", "low", control.ToolApprovalAuto)
	pathB := writeSession("session-b.jsonl", "provider-b/deepseek-v4-flash", "high", control.ToolApprovalYolo)
	low := "low"
	tab := &WorkspaceTab{ID: "tab", Scope: "global", WorkspaceRoot: root, SessionPath: pathA, model: "provider-a/deepseek-v4-flash", billingSource: string(modelbilling.BillingNewAPIByok), effort: &low, actor: userctx.Default(root)}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	app.tabControllerBuilder = func(_ context.Context, _ boot.Options, _ userctx.ActorContext, _ string, _, _ string) (*control.Controller, error) {
		return nil, errors.New("stop after capture")
	}
	assertRebind := func(path, wantModel, wantEffort, wantApproval string) {
		t.Helper()
		loaded, err := agent.LoadSession(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.rebindTabToLoadedSessionPath(tab, path, loaded); err != nil {
			t.Fatal(err)
		}
		if tab.model != wantModel || tab.effort == nil || *tab.effort != wantEffort || tab.toolApprovalMode != wantApproval || !tab.toolApprovalModeKnown {
			t.Fatalf("tab state after rebind = %+v", tab)
		}
	}
	assertRebind(pathB, "provider-b/deepseek-v4-flash", "high", control.ToolApprovalYolo)
	assertRebind(pathA, "provider-a/deepseek-v4-flash", "low", control.ToolApprovalAuto)
}
