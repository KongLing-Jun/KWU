package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/automation"
	"kwu/internal/boot"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

const automationRouteTestLegacyModelRef = "custom/legacy-model"

func configureAutomationRunnerTestRouteWithLegacyModel(t *testing.T) {
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
models = ["chat-model", "legacy-model"]
api_key_env = "DESKTOP_TEST_BYOK_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}
}

type automationRouteWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func newAutomationRouteWaitContext(parent context.Context) *automationRouteWaitContext {
	return &automationRouteWaitContext{Context: parent, entered: make(chan struct{})}
}

func (c *automationRouteWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}

type automationRouteBuildCall struct {
	model  string
	source string
}

func newAutomationRouteController(options boot.Options, actor userctx.ActorContext, root, path, label string, runner agent.Runner) *control.Controller {
	sess := agent.NewSession("system")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	return control.New(control.Options{
		Runner:        runner,
		Executor:      exec,
		Sink:          event.Discard,
		SessionDir:    options.SessionDir,
		SessionPath:   path,
		Label:         label,
		Actor:         actor,
		WorkspaceRoot: root,
	})
}

func addAutomationRoutePendingTab(t *testing.T, app *App, tabID, scope, root, topicID string, actor userctx.ActorContext, route modelbilling.BillingRouteSelection) (*WorkspaceTab, string) {
	t.Helper()
	path, err := createEmptySessionFile(desktopSessionDirForActor(actor, root), "legacy-session")
	if err != nil {
		t.Fatalf("create pending session: %v", err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{
		Scope:         scope,
		WorkspaceRoot: root,
		TopicID:       topicID,
		Model:         automationRouteTestLegacyModelRef,
		BillingSource: string(route.BillingSource),
	}); err != nil {
		t.Fatalf("save pending session meta: %v", err)
	}
	tab := &WorkspaceTab{
		ID:            tabID,
		Scope:         scope,
		WorkspaceRoot: root,
		TopicID:       topicID,
		TopicTitle:    "Automation route test",
		SessionPath:   path,
		Ready:         false,
		model:         route.ModelRef,
		billingSource: string(route.BillingSource),
		actor:         actor,
		disabledMCP:   map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tabID, app: app}
	app.mu.Lock()
	app.tabRuntimes[tabID] = tab
	app.tabOrder = append(app.tabOrder, tabID)
	app.mu.Unlock()
	return tab, path
}

func automationRouteTestTask(id, scope, root, topicID string, route modelbilling.BillingRouteSelection) automation.Task {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	return automation.Task{
		ID: id, Name: "Automation route test", Enabled: true,
		Scope: scope, WorkspaceRoot: root, TopicID: topicID,
		Prompt: automation.PromptConfig{
			Prompt:        "run route test",
			Model:         route.ModelRef,
			BillingSource: string(route.BillingSource),
		},
		Capabilities: automation.CapabilitySelection{ModelRef: route.ModelRef},
		Schedule:     automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt:    now, UpdatedAt: now,
	}
}

func waitForAutomationRouteSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func TestAutomationRouteAwareOpenerPreservesExplicitAndOrdinarySessionRoutes(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureAutomationRunnerTestRouteWithLegacyModel(t)
	app.ctx = context.Background()
	app.readyHook = func() {}
	root := t.TempDir()
	actor := userctx.Default(root)
	route := modelbilling.BillingRouteSelection{
		ModelRef:      automationRunnerTestModelRef,
		BillingSource: modelbilling.BillingNewAPIByok,
	}

	buildStarted := make(chan struct{}, 2)
	releaseExplicit := make(chan struct{})
	releaseOrdinary := make(chan struct{})
	var mu sync.Mutex
	calls := make([]automationRouteBuildCall, 0, 2)
	app.tabControllerBuilder = func(_ context.Context, options boot.Options, actor userctx.ActorContext, workspaceRoot, model, billingSource string) (*control.Controller, error) {
		mu.Lock()
		calls = append(calls, automationRouteBuildCall{model: model, source: billingSource})
		callNo := len(calls)
		mu.Unlock()
		buildStarted <- struct{}{}
		if callNo == 1 {
			<-releaseExplicit
		} else {
			<-releaseOrdinary
		}
		return newAutomationRouteController(options, actor, workspaceRoot, options.SessionDir, model, &appendingDesktopRunner{session: agent.NewSession("system"), started: make(chan string, 1)}), nil
	}

	topic, err := app.CreateTopic("project", root, "Explicit automation route")
	if err != nil {
		t.Fatalf("CreateTopic explicit: %v", err)
	}
	explicitPath, err := createEmptySessionFile(desktopSessionDirForActor(actor, root), "legacy-session")
	if err != nil {
		t.Fatalf("create explicit session: %v", err)
	}
	if err := agent.SaveBranchMeta(explicitPath, agent.BranchMeta{
		Scope: "project", WorkspaceRoot: root, TopicID: topic.ID,
		Model: automationRouteTestLegacyModelRef, BillingSource: string(route.BillingSource),
	}); err != nil {
		t.Fatalf("save explicit session meta: %v", err)
	}
	meta, err := app.openTopicTabWithActivationAndActorForRoute("project", root, topic.ID, explicitPath, false, actor, &route)
	if err != nil {
		t.Fatalf("open explicit route tab: %v", err)
	}
	tab := app.tabByID(meta.ID)
	if tab == nil {
		t.Fatal("explicit route tab is missing")
	}
	waitForAutomationRouteSignal(t, buildStarted, "explicit route builder start")
	app.mu.RLock()
	if tab.model != route.ModelRef || tab.billingSource != string(route.BillingSource) || tab.Ctrl != nil || tab.Ready {
		t.Fatalf("pending explicit tab = %q/%q ctrl=%v ready=%v, want task route and no controller", tab.model, tab.billingSource, tab.Ctrl != nil, tab.Ready)
	}
	app.mu.RUnlock()
	close(releaseExplicit)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ctrl, err := app.waitAutomationTopicController(ctx, tab.ID); err != nil {
		t.Fatalf("wait explicit controller: %v", err)
	} else if ctrl != nil {
		t.Cleanup(func() { ctrl.Close() })
	}
	mu.Lock()
	if len(calls) != 1 || calls[0].model != route.ModelRef || calls[0].source != string(route.BillingSource) {
		t.Fatalf("explicit builder calls = %+v, want task route only", calls)
	}
	mu.Unlock()
	app.mu.RLock()
	if tab.model != route.ModelRef || tab.billingSource != string(route.BillingSource) {
		t.Fatalf("completed explicit tab = %q/%q, want task route", tab.model, tab.billingSource)
	}
	app.mu.RUnlock()
	if got, ok := agent.LoadSessionModel(explicitPath); !ok || got != route.ModelRef {
		t.Fatalf("explicit session model = %q/%v, want %q", got, ok, route.ModelRef)
	}

	ordinaryTopic, err := app.CreateTopic("project", root, "Ordinary session route")
	if err != nil {
		t.Fatalf("CreateTopic ordinary: %v", err)
	}
	ordinaryPath, err := createEmptySessionFile(desktopSessionDirForActor(actor, root), "legacy-session")
	if err != nil {
		t.Fatalf("create ordinary session: %v", err)
	}
	if err := agent.SaveBranchMeta(ordinaryPath, agent.BranchMeta{
		Scope: "project", WorkspaceRoot: root, TopicID: ordinaryTopic.ID,
		Model: automationRouteTestLegacyModelRef, BillingSource: string(route.BillingSource),
	}); err != nil {
		t.Fatalf("save ordinary session meta: %v", err)
	}
	ordinaryMeta, err := app.openTopicTabWithActivationAndActor("project", root, ordinaryTopic.ID, ordinaryPath, false, actor)
	if err != nil {
		t.Fatalf("open ordinary route tab: %v", err)
	}
	ordinaryTab := app.tabByID(ordinaryMeta.ID)
	if ordinaryTab == nil {
		t.Fatal("ordinary route tab is missing")
	}
	waitForAutomationRouteSignal(t, buildStarted, "ordinary route builder start")
	app.mu.RLock()
	if ordinaryTab.Ctrl != nil || ordinaryTab.Ready {
		t.Fatalf("ordinary tab was ready before completion: ctrl=%v ready=%v", ordinaryTab.Ctrl != nil, ordinaryTab.Ready)
	}
	app.mu.RUnlock()
	close(releaseOrdinary)
	if ctrl, err := app.waitAutomationTopicController(ctx, ordinaryTab.ID); err != nil {
		t.Fatalf("wait ordinary controller: %v", err)
	} else if ctrl != nil {
		t.Cleanup(func() { ctrl.Close() })
	}
	mu.Lock()
	if len(calls) != 2 || calls[1].model != automationRouteTestLegacyModelRef || calls[1].source != string(route.BillingSource) {
		t.Fatalf("ordinary builder calls = %+v, want restored session route", calls)
	}
	mu.Unlock()
	app.mu.RLock()
	if ordinaryTab.model != automationRouteTestLegacyModelRef {
		t.Fatalf("completed ordinary tab model = %q, want session model %q", ordinaryTab.model, automationRouteTestLegacyModelRef)
	}
	app.mu.RUnlock()
}

func TestBuildTabControllerWithUnavailableExplicitRouteDoesNotFallback(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureAutomationRunnerTestRoute(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	route := modelbilling.BillingRouteSelection{ModelRef: "custom/missing-model", BillingSource: modelbilling.BillingNewAPIByok}
	tab, _ := addAutomationRoutePendingTab(t, app, "unavailable-route", "project", root, "unavailable-topic", actor, route)
	var builderCalls int
	app.tabControllerBuilder = func(context.Context, boot.Options, userctx.ActorContext, string, string, string) (*control.Controller, error) {
		builderCalls++
		return nil, errors.New("builder must not be called")
	}
	loaded, err := app.preloadTabSession(tab, tab.SessionPath)
	if err != nil {
		t.Fatalf("preload route session: %v", err)
	}
	if err := app.scheduleTabControllerBuildWithRoute(tab, loaded, false, nil, &route); err != nil {
		t.Fatalf("schedule unavailable route: %v", err)
	}
	if builderCalls != 0 {
		t.Fatalf("builder calls = %d, want zero", builderCalls)
	}
	if tab.Ctrl != nil || !tab.Ready || !strings.Contains(tab.StartupErr, "unavailable") {
		t.Fatalf("unavailable route tab = ctrl:%v ready:%v startup:%q", tab.Ctrl != nil, tab.Ready, tab.StartupErr)
	}
	if tab.model != route.ModelRef || tab.billingSource != string(route.BillingSource) {
		t.Fatalf("unavailable route tab changed route to %q/%q", tab.model, tab.billingSource)
	}
}

func TestDesktopAutomationRunnerRepairsPendingOrdinaryBuildToTaskRoute(t *testing.T) {
	tests := []struct {
		name     string
		firstErr error
	}{
		{name: "successful ordinary build", firstErr: nil},
		{name: "pending startup failure", firstErr: errors.New("ordinary startup failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			app := NewApp()
			configureAutomationRunnerTestRouteWithLegacyModel(t)
			app.ctx = context.Background()
			app.readyHook = func() {}
			root := t.TempDir()
			actor := userctx.Default(root)
			route := modelbilling.BillingRouteSelection{ModelRef: automationRunnerTestModelRef, BillingSource: modelbilling.BillingNewAPIByok}
			topic, err := app.CreateTopic("project", root, "Pending route test")
			if err != nil {
				t.Fatalf("CreateTopic: %v", err)
			}
			tab, _ := addAutomationRoutePendingTab(t, app, "pending-route", "project", root, topic.ID, actor, route)
			runner := &appendingDesktopRunner{session: agent.NewSession("system"), started: make(chan string, 1)}
			firstStarted := make(chan struct{})
			repairStarted := make(chan struct{})
			releaseFirst := make(chan struct{})
			var mu sync.Mutex
			calls := make([]automationRouteBuildCall, 0, 2)
			app.tabControllerBuilder = func(_ context.Context, options boot.Options, builderActor userctx.ActorContext, workspaceRoot, model, billingSource string) (*control.Controller, error) {
				mu.Lock()
				calls = append(calls, automationRouteBuildCall{model: model, source: billingSource})
				callNo := len(calls)
				mu.Unlock()
				switch callNo {
				case 1:
					close(firstStarted)
					<-releaseFirst
					if tt.firstErr != nil {
						return nil, tt.firstErr
					}
				case 2:
					close(repairStarted)
				default:
					t.Fatalf("unexpected builder call %d", callNo)
				}
				return newAutomationRouteController(options, builderActor, workspaceRoot, tab.SessionPath, model, runner), nil
			}
			loaded, err := app.preloadTabSession(tab, tab.SessionPath)
			if err != nil {
				t.Fatalf("preload ordinary session: %v", err)
			}
			if err := app.scheduleTabControllerBuild(tab, loaded, true, nil); err != nil {
				t.Fatalf("schedule ordinary build: %v", err)
			}
			waitForAutomationRouteSignal(t, firstStarted, "ordinary builder start")
			waitCtx := newAutomationRouteWaitContext(context.Background())
			resultCh := make(chan struct {
				result automation.RunResult
				err    error
			}, 1)
			go func() {
				result, runErr := newDesktopAutomationRunner(app).Run(waitCtx, automation.RunRequest{
					Task:     automationRouteTestTask("pending-route-task", "project", root, topic.ID, route),
					Manifest: automation.RunManifest{RunID: "pending-route-run", TaskID: "pending-route-task", Trigger: automation.TriggerManual},
					RunDir:   t.TempDir(),
					Sink:     event.Discard,
				})
				resultCh <- struct {
					result automation.RunResult
					err    error
				}{result: result, err: runErr}
			}()
			waitForAutomationRouteSignal(t, waitCtx.entered, "pending controller wait")
			close(releaseFirst)
			waitForAutomationRouteSignal(t, repairStarted, "task route repair builder")
			out := <-resultCh
			if out.err != nil || out.result.Status != automation.RunStatusSucceeded {
				t.Fatalf("Run result = %+v err=%v, want success", out.result, out.err)
			}
			mu.Lock()
			gotCalls := append([]automationRouteBuildCall(nil), calls...)
			mu.Unlock()
			if len(gotCalls) != 2 || gotCalls[0].model != automationRouteTestLegacyModelRef || gotCalls[1].model != route.ModelRef || gotCalls[1].source != string(route.BillingSource) {
				t.Fatalf("builder calls = %+v, want legacy then task route", gotCalls)
			}
			app.mu.RLock()
			if tab.Ctrl == nil || tab.model != route.ModelRef || tab.billingSource != string(route.BillingSource) || !tab.Ready || tab.StartupErr != "" {
				t.Fatalf("repaired tab = ctrl:%v route:%q/%q ready:%v startup:%q", tab.Ctrl != nil, tab.model, tab.billingSource, tab.Ready, tab.StartupErr)
			}
			ctrl := tab.Ctrl
			app.mu.RUnlock()
			if ctrl != nil {
				t.Cleanup(func() { ctrl.Close() })
			}
			select {
			case <-runner.started:
			default:
				t.Fatal("RunCapabilityTurn did not reach the task runner")
			}
		})
	}
}

func TestDesktopAutomationRunnerRouteGuardBlocksControllerSideEffects(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureAutomationRunnerTestRoute(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	route := modelbilling.BillingRouteSelection{ModelRef: automationRunnerTestModelRef, BillingSource: modelbilling.BillingNewAPIByok}
	topic, err := app.CreateTopic("project", root, "Guard route test")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}
	tab, path := addAutomationRoutePendingTab(t, app, "guard-route", "project", root, topic.ID, actor, route)
	tab.Ready = true
	tab.StartupErr = "previous controller build failed"
	tab.model = automationRouteTestLegacyModelRef
	tab.toolApprovalMode = control.ToolApprovalYolo
	tab.toolApprovalModeKnown = true
	runner := &appendingDesktopRunner{session: agent.NewSession("system"), started: make(chan string, 1)}
	app.tabControllerBuilder = func(_ context.Context, options boot.Options, builderActor userctx.ActorContext, workspaceRoot, model, _ string) (*control.Controller, error) {
		ctrl := newAutomationRouteController(options, builderActor, workspaceRoot, path, model, runner)
		// buildTabControllerWithRoute has already committed the task model before
		// invoking this existing builder seam; completion attaches the controller
		// without rewriting the model, so the final runner guard sees this mismatch.
		app.mu.Lock()
		tab.model = "custom/mismatched-model"
		app.mu.Unlock()
		return ctrl, nil
	}
	result, runErr := newDesktopAutomationRunner(app).Run(context.Background(), automation.RunRequest{
		Task:     automationRouteTestTask("guard-route-task", "project", root, topic.ID, route),
		Manifest: automation.RunManifest{RunID: "guard-route-run", TaskID: "guard-route-task", Trigger: automation.TriggerManual},
		RunDir:   t.TempDir(),
		Sink:     event.Discard,
	})
	if runErr == nil || result.Status != automation.RunStatusBlocked || !strings.Contains(runErr.Error(), "route mismatch") {
		t.Fatalf("Run result = %+v err=%v, want blocked route mismatch", result, runErr)
	}
	app.mu.RLock()
	ctrl := tab.Ctrl
	app.mu.RUnlock()
	if ctrl == nil {
		t.Fatal("route repair controller is nil")
	}
	t.Cleanup(ctrl.Close)
	if got := ctrl.ToolApprovalMode(); got != control.ToolApprovalYolo {
		t.Fatalf("approval mode = %q, want unchanged %q", got, control.ToolApprovalYolo)
	}
	select {
	case <-runner.started:
		t.Fatal("RunCapabilityTurn ran after final route mismatch")
	default:
	}
}

func TestAutomationTopicControllerWaitRepairPolicyPreservesCanceledStartupFailure(t *testing.T) {
	app := NewApp()
	tab := &WorkspaceTab{ID: "canceled-startup-failure", Ready: true, StartupErr: "startup failed"}
	app.tabRuntimes[tab.ID] = tab
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if app.automationTopicControllerWaitNeedsRouteRepair(ctx, tab.ID, errors.New("startup failure")) {
		t.Fatal("canceled startup failure requested route repair")
	}
}
