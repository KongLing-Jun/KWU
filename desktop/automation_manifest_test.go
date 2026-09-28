package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/project"
	"kwu/internal/userctx"
)

type automationRouteCaptureRunner struct {
	requests map[string]automation.RunRequest
	calls    map[string]int
}

func (r *automationRouteCaptureRunner) Run(_ context.Context, request automation.RunRequest) (automation.RunResult, error) {
	if r.requests == nil {
		r.requests = map[string]automation.RunRequest{}
	}
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.requests[request.Task.ID] = request
	r.calls[request.Task.ID]++
	return automation.RunResult{Status: automation.RunStatusSucceeded}, nil
}

func TestAutomationResolvedRouteMatchesRunnerManifestAndSummaryAcrossModels(t *testing.T) {
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
model = "chat-model"
api_key_env = "CUSTOM_CHAT_KEY"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	tasks := []automation.Task{
		{
			ID: "default-route", Name: "Default route", Enabled: true,
			Prompt:    automation.PromptConfig{Prompt: "default"},
			Schedule:  automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "legacy-route", Name: "Legacy route", Enabled: true,
			Prompt:    automation.PromptConfig{Prompt: "legacy", Model: "custom/chat-model"},
			Schedule:  automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "official-fable", Name: "Official fable", Enabled: true,
			Prompt: automation.PromptConfig{
				Prompt: "fable", Model: "kwu/claude-fable-5",
				BillingSource: string(modelbilling.BillingSubscriptionPool),
			},
			Schedule:  automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "official-opus-picker-artifact", Name: "Official opus", Enabled: true,
			Prompt: automation.PromptConfig{
				Prompt: "opus", Model: ":kwu/claude-opus-4-6",
				BillingSource: string(modelbilling.BillingSubscriptionPool),
			},
			Schedule:  automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
			CreatedAt: now, UpdatedAt: now,
		},
	}
	store := automation.NewStore(t.TempDir())
	if err := store.SaveTasks(tasks); err != nil {
		t.Fatal(err)
	}
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
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-fixture",
	}, []string{"claude-fable-5", "claude-opus-4-6"})
	runner := &automationRouteCaptureRunner{}
	engine := automation.NewEngineWithChecker(store, "desktop:test", runner, automation.Checker{
		TaskResolver: func(task automation.Task) (automation.Task, error) {
			resolved, _, err := app.resolveAutomationTask(task)
			return resolved, err
		},
		ModelResolver: func(task automation.Task) error {
			_, err := app.resolveAutomationTaskRoute(task)
			return err
		},
	})

	wantRoutes := map[string]modelbilling.BillingRouteSelection{
		"default-route":  {ModelRef: "custom/chat-model", BillingSource: modelbilling.BillingNewAPIByok},
		"legacy-route":   {ModelRef: "custom/chat-model", BillingSource: modelbilling.BillingNewAPIByok},
		"official-fable": {ModelRef: "kwu/claude-fable-5", BillingSource: modelbilling.BillingSubscriptionPool},
		"official-opus-picker-artifact": {
			ModelRef: "kwu/claude-opus-4-6", BillingSource: modelbilling.BillingSubscriptionPool,
		},
	}
	for _, task := range tasks {
		runID, err := engine.Trigger(context.Background(), automation.TriggerRequest{TaskID: task.ID})
		if err != nil {
			t.Fatalf("%s trigger: %v", task.ID, err)
		}
		manifest, err := store.ReadRunManifest(task.ID, runID)
		if err != nil {
			t.Fatalf("%s manifest: %v", task.ID, err)
		}
		request, ok := runner.requests[task.ID]
		if !ok {
			t.Fatalf("%s runner request missing", task.ID)
		}
		if runner.calls[task.ID] != 1 {
			t.Fatalf("%s runner calls = %d, want 1", task.ID, runner.calls[task.ID])
		}
		want := wantRoutes[task.ID]
		if manifest.Model != want.ModelRef || manifest.BillingSource != string(want.BillingSource) {
			t.Fatalf("%s manifest route = %q/%q, want %s/%s", task.ID, manifest.BillingSource, manifest.Model, want.BillingSource, want.ModelRef)
		}
		summaries, err := store.ListRuns(task.ID, 1)
		if err != nil {
			t.Fatalf("%s summaries: %v", task.ID, err)
		}
		if len(summaries) != 1 ||
			summaries[0].Model != manifest.Model ||
			summaries[0].BillingSource != manifest.BillingSource {
			t.Fatalf("%s summary route = %+v; manifest %q/%q", task.ID, summaries, manifest.BillingSource, manifest.Model)
		}
		if request.Task.Prompt.Model != manifest.Model ||
			request.Task.Prompt.BillingSource != manifest.BillingSource ||
			request.Manifest.Model != manifest.Model ||
			request.Manifest.BillingSource != manifest.BillingSource {
			t.Fatalf("%s runner route = task %q/%q manifest %q/%q; archived %q/%q",
				task.ID,
				request.Task.Prompt.BillingSource, request.Task.Prompt.Model,
				request.Manifest.BillingSource, request.Manifest.Model,
				manifest.BillingSource, manifest.Model,
			)
		}
	}
}

func TestAutomationManagedWorkspaceCanonicalizationReachesManualAndScheduledRuns(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	configureDesktopTestBYOKRoute(t)
	projectID := "project-automation-run-canonicalization"
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, projectID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := state.services.Cache.StoreDetail(context.Background(), state.scope, project.Project{
		ProjectID: projectID, OwnerUserID: state.scope.Binding.CloudUserID, Name: "Automation run project",
		Status: project.ProjectActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	tasks := []automation.Task{
		{
			ID: "manual-managed-workspace", Name: "Manual managed workspace", Enabled: true,
			Scope: "project", WorkspaceRoot: managedRoot, TopicID: "legacy-manual-topic",
			Prompt:   automation.PromptConfig{Prompt: "manual"},
			Schedule: automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		},
		{
			ID: "scheduled-managed-workspace", Name: "Scheduled managed workspace", Enabled: true,
			Scope: "project", WorkspaceRoot: managedRoot, TopicID: "legacy-scheduled-topic",
			Prompt: automation.PromptConfig{Prompt: "scheduled"}, MissedRunPolicy: automation.MissedRunCatchUp,
			Schedule: automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: now.Add(-time.Second)},
		},
	}
	store := automation.NewStore(t.TempDir())
	if err := store.SaveTasks(tasks); err != nil {
		t.Fatal(err)
	}
	runner := &automationRouteCaptureRunner{}
	engine := automation.NewEngineWithChecker(store, "desktop:test", runner, automation.Checker{
		TaskResolver: func(task automation.Task) (automation.Task, error) {
			resolved, _, resolveErr := app.resolveAutomationTask(task)
			return resolved, resolveErr
		},
		ModelResolver: func(task automation.Task) error {
			_, resolveErr := app.resolveAutomationTaskRoute(task)
			return resolveErr
		},
	})

	manualRunID, err := engine.Trigger(context.Background(), automation.TriggerRequest{TaskID: tasks[0].ID})
	if err != nil {
		t.Fatalf("manual trigger: %v", err)
	}
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("scheduled tick: %v", err)
	}
	scheduledRuns, err := store.ListRuns(tasks[1].ID, 2)
	if err != nil || len(scheduledRuns) != 1 {
		t.Fatalf("scheduled runs = %+v, err = %v", scheduledRuns, err)
	}
	runIDs := map[string]string{tasks[0].ID: manualRunID, tasks[1].ID: scheduledRuns[0].RunID}
	for _, task := range tasks {
		request, ok := runner.requests[task.ID]
		if !ok || runner.calls[task.ID] != 1 {
			t.Fatalf("%s runner request/calls = %+v/%d", task.ID, request, runner.calls[task.ID])
		}
		assertAutomationCloudProjectTarget(t, request.Task, projectID)
		manifest, err := store.ReadRunManifest(task.ID, runIDs[task.ID])
		if err != nil {
			t.Fatalf("%s manifest: %v", task.ID, err)
		}
		if manifest.ProjectID != projectID || manifest.WorkspaceRoot != "" || request.Manifest.ProjectID != projectID || request.Manifest.WorkspaceRoot != "" {
			t.Fatalf("%s run target = archived %q/%q runner %q/%q", task.ID, manifest.ProjectID, manifest.WorkspaceRoot, request.Manifest.ProjectID, request.Manifest.WorkspaceRoot)
		}
	}
}
