package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestTabSendAvailabilityStates(t *testing.T) {
	tests := []struct {
		name       string
		tab        *WorkspaceTab
		state      string
		retryable  bool
		action     string
		detailPart string
	}{
		{name: "starting", tab: &WorkspaceTab{}, state: tabSendStarting},
		{name: "ready", tab: &WorkspaceTab{Ctrl: control.New(control.Options{}), Ready: true}, state: tabSendReady},
		{name: "ready requires controller", tab: &WorkspaceTab{Ready: true}, state: tabSendStartupFailed, retryable: true, action: "retry", detailPart: "controller is unavailable"},
		{name: "subscription signed out", tab: &WorkspaceTab{billingSource: string(modelbilling.BillingSubscriptionPool), Ready: true, subscriptionAuthPending: true, startupDetail: "signed out"}, state: tabSendSubscriptionAuthRequired, retryable: true, action: "login", detailPart: "signed out"},
		{name: "subscription session expired", tab: failedAvailabilityTab(modelbilling.BillingSubscriptionPool, modelbilling.ErrorSessionExpired, tabStartupFailureController, "expired"), state: tabSendSubscriptionAuthRequired, retryable: true, action: "login", detailPart: "expired"},
		{name: "subscription account action", tab: failedAvailabilityTab(modelbilling.BillingSubscriptionPool, modelbilling.ErrorInsufficientCredits, tabStartupFailureController, "credits"), state: tabSendSubscriptionAccountAction, retryable: true, action: "account", detailPart: "credits"},
		{name: "BYOK credential", tab: failedAvailabilityTab(modelbilling.BillingNewAPIByok, modelbilling.ErrorByokCredentialInvalid, tabStartupFailureController, "key"), state: tabSendBYOKConfigurationRequired, retryable: true, action: "configure_byok", detailPart: "key"},
		{name: "BYOK config load", tab: failedAvailabilityTab(modelbilling.BillingNewAPIByok, "", tabStartupFailureConfig, "base URL"), state: tabSendBYOKConfigurationRequired, retryable: true, action: "configure_byok", detailPart: "base URL"},
		{name: "service unavailable", tab: failedAvailabilityTab(modelbilling.BillingSubscriptionPool, modelbilling.ErrorGatewayUnavailable, tabStartupFailureController, "gateway"), state: tabSendServiceUnavailable, retryable: true, action: "retry", detailPart: "gateway"},
		{name: "unclassified pricing failure", tab: failedAvailabilityTab(modelbilling.BillingSubscriptionPool, modelbilling.ErrorPricingUnavailable, tabStartupFailureController, "pricing"), state: tabSendStartupFailed, retryable: true, action: "retry", detailPart: "pricing"},
		{name: "ordinary startup failure", tab: failedAvailabilityTab(modelbilling.BillingNewAPIByok, "", tabStartupFailureController, "boom"), state: tabSendStartupFailed, retryable: true, action: "retry", detailPart: "boom"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.tab.Ctrl != nil {
				t.Cleanup(test.tab.Ctrl.Close)
			}
			got := tabSendAvailability(test.tab)
			if got.State != test.state || got.Retryable != test.retryable || got.Action != test.action || !strings.Contains(got.Detail, test.detailPart) {
				t.Fatalf("availability = %+v, want state=%q retryable=%t action=%q detail~%q", got, test.state, test.retryable, test.action, test.detailPart)
			}
		})
	}
}

func failedAvailabilityTab(source modelbilling.BillingSource, code modelbilling.ErrorCode, kind tabStartupFailureKind, detail string) *WorkspaceTab {
	return &WorkspaceTab{
		Ready: true, StartupErr: detail, billingSource: string(source),
		startupErrorCode: code, startupFailureKind: kind, startupDetail: detail,
	}
}

func TestTabSendAvailabilityIsRuntimeOnly(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "topic-runtime-only", "runtime-only")
	tab.SessionPath = filepath.Join(desktopSessionDir(globalTabWorkspaceRoot()), "runtime-only.jsonl")
	setTabStartupFailureLocked(tab, &modelbilling.Error{Code: modelbilling.ErrorGatewayUnavailable, Message: "private detail"}, tabStartupFailureController)
	tab.Ready = true
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.mu.Lock()
	app.saveTabsLocked()
	app.mu.Unlock()

	persisted := loadTabsFileFromDir(app.desktopConfigDir())
	if len(persisted.Tabs) != 1 || persisted.Tabs[0].SessionPath != tab.SessionPath {
		t.Fatalf("persisted tabs = %+v", persisted)
	}
}

func TestRetryTabStartupPreservesSessionIdentityAndHistory(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "retry-same-session.jsonl")
	writeHistoryTestSession(t, path, "retry history")
	app, tab := newControllerlessHistoryTab(root, path)
	tab.TopicID = "retry-topic"
	tab.TopicTitle = "Retry topic"
	tab.ReadOnly = true
	tab.model = desktopTestBYOKModelRef
	tab.billingSource = desktopTestBYOKBillingSource
	setTabStartupFailureLocked(tab, errors.New("first startup failed"), tabStartupFailureController)
	tab.Ready = true

	meta, err := app.RetryTabStartup(tab.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tab.Ctrl == nil || meta.SendAvailability.State != tabSendReady {
		t.Fatalf("retry result = ctrl:%v availability:%+v", tab.Ctrl != nil, meta.SendAvailability)
	}
	t.Cleanup(tab.Ctrl.Close)
	if tab.ID != "history-tab" || tab.TopicID != "retry-topic" || tab.TopicTitle != "Retry topic" || tab.WorkspaceRoot != root || !tab.ReadOnly || filepath.Clean(tab.SessionPath) != filepath.Clean(path) {
		t.Fatalf("retry changed identity: %+v", tab)
	}
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "retry history" {
		t.Fatalf("retry history = %+v", history)
	}
}

func TestBYOKMissingConfigurationDoesNotRequireLogin(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
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
api_key_env = "MISSING_BYOK_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MISSING_BYOK_KEY", "")
	tab := app.createTabEntryWithIDAndActorForRoute("global", globalTabWorkspaceRoot(), "", "missing-byok", userctx.ActorContext{}, modelbilling.BillingNewAPIByok)
	tab.model = "custom/chat-model"
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.buildTabController(tab)

	availability := tabSendAvailability(tab)
	if tab.Ctrl != nil || availability.State != tabSendBYOKConfigurationRequired || availability.Action != "configure_byok" || tab.subscriptionAuthPending {
		t.Fatalf("missing BYOK = ctrl:%v pending:%t availability:%+v", tab.Ctrl != nil, tab.subscriptionAuthPending, availability)
	}
}

func TestSettingsRebuildFailureDoesNotExposeClosedControllerAsReady(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte("[[invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldCtrl := control.New(control.Options{Label: "custom/chat-model"})
	tab := app.createTabEntryWithIDAndActorForRoute("global", globalTabWorkspaceRoot(), "", "rebuild-missing-byok", userctx.ActorContext{}, modelbilling.BillingNewAPIByok)
	tab.model = "custom/chat-model"
	tab.Ctrl = oldCtrl
	tab.Ready = true
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.ctx = context.Background()
	app.readyHook = func() {}

	if err := app.rebuild(); err == nil {
		t.Fatal("settings rebuild with invalid config error = nil")
	}
	availability := tabSendAvailability(tab)
	if tab.Ctrl != nil || availability.State != tabSendStartupFailed || availability.Action != "retry" {
		t.Fatalf("failed settings rebuild = ctrl:%v availability:%+v", tab.Ctrl != nil, availability)
	}
}

func TestFailedSubscriptionCanSwitchToConfiguredBYOKOnSameSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "switch-to-byok.jsonl")
	writeHistoryTestSession(t, path, "same session history")
	app, tab := newControllerlessHistoryTab(root, path)
	tab.TopicID = "same-topic"
	tab.TopicTitle = "Same topic"
	tab.model = "kwu/gpt-5.4"
	tab.billingSource = string(modelbilling.BillingSubscriptionPool)
	tab.subscriptionAuthPending = true
	tab.Ready = true

	if err := app.SetModelRouteForTab(tab.ID, desktopTestBYOKModelRef, desktopTestBYOKBillingSource); err != nil {
		t.Fatal(err)
	}
	if tab.Ctrl == nil || tab.billingSource != desktopTestBYOKBillingSource || filepath.Clean(tab.SessionPath) != filepath.Clean(path) || tab.TopicID != "same-topic" || tab.TopicTitle != "Same topic" {
		t.Fatalf("BYOK switch changed session identity: ctrl:%v source:%q path:%q topic:%q/%q", tab.Ctrl != nil, tab.billingSource, tab.SessionPath, tab.TopicID, tab.TopicTitle)
	}
	t.Cleanup(tab.Ctrl.Close)
	if availability := tabSendAvailability(tab); availability.State != tabSendReady {
		t.Fatalf("BYOK availability = %+v", availability)
	}
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "same session history" {
		t.Fatalf("BYOK switch history = %+v", history)
	}
}

func TestRetryAndModelSwitchGenerationKeepsLatestControllerAndSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	_ = NewApp()
	configureDesktopTestBYOKRoute(t)
	root := globalTabWorkspaceRoot()
	dir := desktopSessionDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "retry-model-generation.jsonl")
	writeHistoryTestSession(t, path, "generation history")
	app, tab := newControllerlessHistoryTab(root, path)
	tab.model = desktopTestBYOKModelRef
	tab.billingSource = desktopTestBYOKBillingSource
	tab.Ready = true
	setTabStartupFailureLocked(tab, errors.New("retry me"), tabStartupFailureController)

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	app.tabControllerBuilder = func(ctx context.Context, options boot.Options, actor userctx.ActorContext, workspaceRoot, model, billingSource string) (*control.Controller, error) {
		call := calls.Add(1)
		if call == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		return app.buildDesktopController(ctx, options, actor, workspaceRoot, model, billingSource)
	}

	retryDone := make(chan error, 1)
	go func() {
		_, err := app.RetryTabStartup(tab.ID)
		retryDone <- err
	}()
	<-firstStarted
	if err := app.SetModelRouteForTab(tab.ID, desktopTestBYOKModelRef, desktopTestBYOKBillingSource); err != nil {
		t.Fatal(err)
	}
	latest := tab.Ctrl
	if latest == nil || calls.Load() != 2 {
		t.Fatalf("latest build = ctrl:%v calls:%d", latest != nil, calls.Load())
	}
	close(releaseFirst)
	select {
	case err := <-retryDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry build did not finish")
	}
	if tab.Ctrl != latest || filepath.Clean(tab.SessionPath) != filepath.Clean(path) || tabSendAvailability(tab).State != tabSendReady {
		t.Fatalf("stale retry overwrote latest build: sameCtrl=%t path=%q availability=%+v", tab.Ctrl == latest, tab.SessionPath, tabSendAvailability(tab))
	}
	t.Cleanup(latest.Close)
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "generation history" {
		t.Fatalf("generation history = %+v", history)
	}
}
