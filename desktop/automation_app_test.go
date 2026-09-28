package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/automation"
	"kwu/internal/config"
	"kwu/internal/project"
)

func TestAutomationBindingsSaveAndReloadTasks(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	setAnonymousAutomationQuota(t, app)
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)

	task := automation.Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Prompt:    automation.PromptConfig{Prompt: "check status"},
		Schedule:  automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := app.AutomationSaveTasks([]automation.Task{task}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}

	tasks, err := app.AutomationReloadTasks()
	if err != nil {
		t.Fatalf("AutomationReloadTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks len = %d, want 1: %+v", len(tasks), tasks)
	}
	if tasks[0].ID != task.ID || tasks[0].Name != task.Name || tasks[0].Schedule.Every != "30m" ||
		tasks[0].Prompt.Model != desktopTestBYOKModelRef ||
		tasks[0].Prompt.BillingSource != desktopTestBYOKBillingSource ||
		tasks[0].Capabilities.ModelRef != desktopTestBYOKModelRef {
		t.Fatalf("task = %+v, want saved task", tasks[0])
	}
}

func TestAutomationTaskCreateAndUpdateMaterializeRouteAndScope(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	fake := &fakeAutomationService{}
	app.automationService = fake

	created, err := app.AutomationCreateTask(automation.Task{
		ID:            "task-create",
		Name:          "Create task",
		Scope:         "global",
		WorkspaceRoot: t.TempDir(),
		Prompt: automation.PromptConfig{
			Prompt:        "run",
			BillingSource: "subscription_pool",
		},
		Capabilities: automation.CapabilitySelection{ModelRef: "stale/model"},
		Schedule:     automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	})
	if err != nil {
		t.Fatalf("AutomationCreateTask: %v", err)
	}
	assertMaterializedAutomationRoute(t, created, "global", "")
	if fake.createCalls != 1 || len(fake.savedTasks) != 1 {
		t.Fatalf("create calls/tasks = %d/%+v, want one materialized task", fake.createCalls, fake.savedTasks)
	}
	assertMaterializedAutomationRoute(t, fake.savedTasks[0], "global", "")

	relativeRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := app.AutomationUpdateTask(automation.Task{
		ID:            "task-update",
		Name:          "Update task",
		Scope:         "project",
		WorkspaceRoot: ".",
		Prompt: automation.PromptConfig{
			Prompt:        "run again",
			BillingSource: "subscription_pool",
		},
		Capabilities: automation.CapabilitySelection{ModelRef: "stale/model"},
		Schedule:     automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "1h"},
	}, 3)
	if err != nil {
		t.Fatalf("AutomationUpdateTask: %v", err)
	}
	assertMaterializedAutomationRoute(t, updated, "project", relativeRoot)
	if fake.updateCalls != 1 || len(fake.updatedTasks) != 1 {
		t.Fatalf("update calls/tasks = %d/%+v, want one materialized task", fake.updateCalls, fake.updatedTasks)
	}
	assertMaterializedAutomationRoute(t, fake.updatedTasks[0], "project", relativeRoot)
}

func TestAutomationManagedWorkspaceTargetCanonicalizesToCloudProject(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	configureDesktopTestBYOKRoute(t)
	projectID := "project-automation-managed-workspace"
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, projectID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := state.services.Cache.StoreDetail(context.Background(), state.scope, project.Project{
		ProjectID: projectID, OwnerUserID: state.scope.Binding.CloudUserID, Name: "Automation project",
		Status: project.ProjectActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	legacy := automation.Task{
		ID: "legacy-managed-workspace", Name: "Legacy managed workspace", Scope: "project", WorkspaceRoot: managedRoot,
		TopicID:  "legacy-local-topic",
		Prompt:   automation.PromptConfig{Prompt: "run"},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
	}
	fake := &fakeAutomationService{tasks: []automation.TaskView{{Task: legacy}}}
	app.automationService = fake

	views, err := app.AutomationReloadTasks()
	if err != nil {
		t.Fatalf("AutomationReloadTasks: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("task views = %+v, want one task", views)
	}
	assertAutomationCloudProjectTarget(t, views[0].Task, projectID)
	updated, err := app.AutomationUpdateTask(views[0].Task, views[0].Revision)
	if err != nil {
		t.Fatalf("AutomationUpdateTask: %v", err)
	}
	assertAutomationCloudProjectTarget(t, updated, projectID)
	assertAutomationCloudProjectTarget(t, fake.updatedTasks[len(fake.updatedTasks)-1], projectID)

	created, err := app.AutomationCreateTask(legacy)
	if err != nil {
		t.Fatalf("AutomationCreateTask: %v", err)
	}
	assertAutomationCloudProjectTarget(t, created, projectID)
	if fake.createCalls != 1 || len(fake.savedTasks) != 1 {
		t.Fatalf("create calls/tasks = %d/%+v, want one canonical task", fake.createCalls, fake.savedTasks)
	}
	assertAutomationCloudProjectTarget(t, fake.savedTasks[0], projectID)

	resolved, _, err := app.resolveAutomationTask(legacy)
	if err != nil {
		t.Fatalf("resolveAutomationTask: %v", err)
	}
	assertAutomationCloudProjectTarget(t, resolved, projectID)

	unknownManagedRoot := filepath.Join(
		state.services.StateRoot,
		cloudProjectWorkspacesDir,
		state.scope.Key,
		"project-"+strings.Repeat("a", 64),
	)
	legacy.WorkspaceRoot = unknownManagedRoot
	if _, _, err := app.resolveAutomationTask(legacy); err == nil {
		t.Fatal("resolveAutomationTask accepted an unassociated managed Cloud Project workspace")
	}
}

func assertAutomationCloudProjectTarget(t *testing.T, task automation.Task, projectID string) {
	t.Helper()
	if task.Scope != "project" || task.ProjectID != projectID || task.WorkspaceRoot != "" || task.TopicID != "" {
		t.Fatalf("automation target = scope %q project %q workspace %q topic %q, want fresh project %q target", task.Scope, task.ProjectID, task.WorkspaceRoot, task.TopicID, projectID)
	}
}

func TestAutomationBulkSaveRejectsBeforeServiceCall(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	fake := &fakeAutomationService{}
	app.automationService = fake

	err := app.AutomationSaveTasks([]automation.Task{
		{
			ID:     "valid",
			Name:   "Valid",
			Prompt: automation.PromptConfig{Prompt: "run", BillingSource: "subscription_pool"},
			Schedule: automation.Schedule{
				Kind:  automation.ScheduleKindInterval,
				Every: "30m",
			},
		},
		{
			ID:   "invalid",
			Name: "Invalid",
			Prompt: automation.PromptConfig{
				Prompt:        "run",
				Model:         desktopTestBYOKModelRef,
				BillingSource: "subscription_pool",
			},
			Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"},
		},
	})
	if err == nil {
		t.Fatal("AutomationSaveTasks accepted an unavailable route")
	}
	if fake.saveCalls != 0 || fake.savedTasks != nil {
		t.Fatalf("bulk service state = calls %d tasks %+v, want no save call", fake.saveCalls, fake.savedTasks)
	}
}

func assertMaterializedAutomationRoute(t *testing.T, task automation.Task, wantScope, wantRoot string) {
	t.Helper()
	if task.Scope != wantScope || task.WorkspaceRoot != wantRoot {
		t.Fatalf("scope/root = %q/%q, want %q/%q", task.Scope, task.WorkspaceRoot, wantScope, wantRoot)
	}
	if task.Prompt.Model != desktopTestBYOKModelRef || task.Prompt.BillingSource != desktopTestBYOKBillingSource || task.Capabilities.ModelRef != desktopTestBYOKModelRef {
		t.Fatalf("materialized route = prompt %q/%q capability %q, want %q/%q/%q", task.Prompt.Model, task.Prompt.BillingSource, task.Capabilities.ModelRef, desktopTestBYOKModelRef, desktopTestBYOKBillingSource, desktopTestBYOKModelRef)
	}
}

func TestPickAutomationSourceFileWithoutContextIsCancel(t *testing.T) {
	path, err := NewApp().PickAutomationSourceFile("csv")
	if err != nil {
		t.Fatalf("PickAutomationSourceFile: %v", err)
	}
	if path != "" {
		t.Fatalf("PickAutomationSourceFile path = %q, want empty cancel", path)
	}
}

func TestAutomationBindingsListTemplates(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	templateTask := automation.Task{
		ID:       "task-custom",
		Name:     "Manual template",
		Prompt:   automation.PromptConfig{Prompt: "manual prompt", Model: "custom/missing-model", BillingSource: "subscription_pool"},
		Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "1h"},
	}
	if _, err := app.AutomationSaveTaskTemplate(templateTask); err != nil {
		t.Fatalf("AutomationSaveTaskTemplate: %v", err)
	}
	templates, err := app.AutomationListTemplates()
	if err != nil {
		t.Fatalf("AutomationListTemplates: %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("AutomationListTemplates returned no templates")
	}
	hasBusinessTemplate := false
	hasUserTemplate := false
	for _, template := range templates {
		if template.Kind == "business_diagnostics" {
			hasBusinessTemplate = true
		}
		if template.ID == "user-task-custom" && template.Source == automation.TemplateSourceUser {
			hasUserTemplate = true
		}
	}
	if !hasBusinessTemplate {
		t.Fatal("AutomationListTemplates missing business diagnostics template")
	}
	if !hasUserTemplate {
		t.Fatal("AutomationListTemplates missing user-saved template")
	}
	if err := app.AutomationDeleteTaskTemplate("user-task-custom"); err != nil {
		t.Fatalf("AutomationDeleteTaskTemplate: %v", err)
	}
	templates, err = app.AutomationListTemplates()
	if err != nil {
		t.Fatalf("AutomationListTemplates after delete: %v", err)
	}
	for _, template := range templates {
		if template.ID == "user-task-custom" {
			t.Fatalf("deleted user template still listed: %+v", templates)
		}
	}
	if err := app.AutomationRestoreTaskTemplate("user-task-custom"); err != nil {
		t.Fatalf("AutomationRestoreTaskTemplate: %v", err)
	}
	templates, err = app.AutomationListTemplates()
	if err != nil {
		t.Fatalf("AutomationListTemplates after restore: %v", err)
	}
	hasUserTemplate = false
	for _, template := range templates {
		if template.ID == "user-task-custom" && template.Source == automation.TemplateSourceUser {
			hasUserTemplate = true
		}
	}
	if !hasUserTemplate {
		t.Fatal("AutomationRestoreTaskTemplate did not restore user-saved template")
	}
}

func TestAutomationStartupCreatesServiceAndNoRunsWithoutTasks(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	app.activeTabID = "active"
	app.startAutomation(context.Background())
	t.Cleanup(func() { app.shutdownAutomation(context.Background()) })

	if app.automationService == nil || app.automationEngine == nil {
		t.Fatal("automation service and engine should be created on startup")
	}
	if app.cachedActiveTabID() != "active" {
		t.Fatalf("activeTabID after startup = %q, want unchanged", app.activeTabID)
	}
	runsDir := filepath.Join(automation.DefaultDir(), "runs")
	if entries, err := os.ReadDir(runsDir); err == nil && len(entries) > 0 {
		t.Fatalf("runs dir = %+v, want no runs without tasks", entries)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir(%s): %v", runsDir, err)
	}
	app.shutdownAutomation(context.Background())
	if app.cachedActiveTabID() != "active" {
		t.Fatalf("activeTabID after shutdown = %q, want unchanged", app.activeTabID)
	}
}

func TestAutomationStartupKeepsServiceAfterHeartbeatMigrationFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	if err := os.MkdirAll(config.MemoryUserDir(), 0o755); err != nil {
		t.Fatalf("MkdirAll state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(config.MemoryUserDir(), "heartbeat-tasks.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("WriteFile legacy heartbeat config: %v", err)
	}
	app := NewApp()

	app.startAutomation(context.Background())
	t.Cleanup(func() { app.shutdownAutomation(context.Background()) })

	status, err := app.AutomationHeartbeatMigrationStatus()
	if err != nil {
		t.Fatalf("AutomationHeartbeatMigrationStatus: %v", err)
	}
	if status.Status != automation.HeartbeatMigrationStatusFailed || status.ErrorSummary == "" {
		t.Fatalf("migration status = %+v, want visible failure", status)
	}
	if app.automationService == nil {
		t.Fatal("automation service should remain available after migration failure")
	}
}

func TestAutomationBindingsUseServiceWithoutSwitchingTabs(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)
	app.activeTabID = "active"
	fake := &fakeAutomationService{
		tasks: []automation.TaskView{{Task: automation.Task{ID: "task-1", Name: "Task 1"}}},
		runs:  []automation.RunSummary{{TaskID: "task-1", RunID: "run-1", Status: automation.RunStatusSucceeded}},
		detail: automation.RunDetail{Manifest: automation.RunManifest{
			TaskID: "task-1",
			RunID:  "run-1",
			Status: automation.RunStatusSucceeded,
		}},
		trigger: automation.TriggerNowResponse{TaskID: "task-1", RunID: "run-1", Status: automation.RunStatusSucceeded},
		cancel:  automation.CancelRunResponse{TaskID: "task-1", RunID: "run-1", Canceled: true, Status: string(automation.RunStatusCanceled)},
	}
	app.automationService = fake

	tasks, err := app.AutomationListTasks()
	if err != nil {
		t.Fatalf("AutomationListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-1" {
		t.Fatalf("tasks = %+v, want fake task", tasks)
	}
	if err := app.AutomationSaveTasks([]automation.Task{{ID: "task-2", Name: "Task 2", Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"}}}); err != nil {
		t.Fatalf("AutomationSaveTasks: %v", err)
	}
	if _, err := app.AutomationListTemplates(); err != nil {
		t.Fatalf("AutomationListTemplates: %v", err)
	}
	if _, err := app.AutomationSaveTaskTemplate(automation.Task{ID: "task-2", Name: "Task 2", Prompt: automation.PromptConfig{Prompt: "save"}, Schedule: automation.Schedule{Kind: automation.ScheduleKindInterval, Every: "30m"}}); err != nil {
		t.Fatalf("AutomationSaveTaskTemplate: %v", err)
	}
	if err := app.AutomationDeleteTaskTemplate("user-task-2"); err != nil {
		t.Fatalf("AutomationDeleteTaskTemplate: %v", err)
	}
	if err := app.AutomationRestoreTaskTemplate("user-task-2"); err != nil {
		t.Fatalf("AutomationRestoreTaskTemplate: %v", err)
	}
	resp, err := app.AutomationTriggerNow("task-1")
	if err != nil {
		t.Fatalf("AutomationTriggerNow: %v", err)
	}
	if resp.RunID != "run-1" {
		t.Fatalf("trigger response = %+v, want run-1", resp)
	}
	runs, err := app.AutomationListRuns("task-1", 10)
	if err != nil {
		t.Fatalf("AutomationListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run-1" {
		t.Fatalf("runs = %+v, want fake run", runs)
	}
	page, err := app.AutomationListRunsV2(automation.RunFilter{Limit: 20}, "")
	if err != nil {
		t.Fatalf("AutomationListRunsV2: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].RunID != "run-1" {
		t.Fatalf("run page = %+v, want fake run", page)
	}
	detail, err := app.AutomationReadRun("task-1", "run-1")
	if err != nil {
		t.Fatalf("AutomationReadRun: %v", err)
	}
	if detail.Manifest.RunID != "run-1" {
		t.Fatalf("detail = %+v, want fake detail", detail)
	}
	exported, err := app.AutomationExportRunResult("task-1", "run-1", "docx")
	if err != nil {
		t.Fatalf("AutomationExportRunResult: %v", err)
	}
	if exported.Filename != "result.docx" {
		t.Fatalf("exported = %+v, want result.docx", exported)
	}
	cancel, err := app.AutomationCancelRun("task-1", "run-1")
	if err != nil {
		t.Fatalf("AutomationCancelRun: %v", err)
	}
	if !cancel.Canceled {
		t.Fatalf("cancel = %+v, want canceled", cancel)
	}
	if app.cachedActiveTabID() != "active" {
		t.Fatalf("activeTabID = %q, want unchanged", app.activeTabID)
	}
	if fake.triggerTaskID != "task-1" || fake.runFilter.TaskID != "task-1" || fake.runV2Filter.Limit != 20 || fake.readTaskID != "task-1" || fake.exportReq.Format != "docx" || fake.cancelReq.RunID != "run-1" {
		t.Fatalf("fake service calls not wired correctly: %+v", fake)
	}
	if len(fake.savedTemplates) != 1 || fake.savedTemplates[0].ID != "task-2" {
		t.Fatalf("fake saved templates = %+v, want task-2", fake.savedTemplates)
	}
	if fake.deletedTemplateID != "user-task-2" {
		t.Fatalf("fake deleted template id = %q, want user-task-2", fake.deletedTemplateID)
	}
}

func TestAutomationShutdownStopsService(t *testing.T) {
	app := NewApp()
	fake := &fakeAutomationService{}
	app.automationService = fake

	app.shutdownAutomation(context.Background())

	if fake.shutdowns != 1 {
		t.Fatalf("shutdowns = %d, want 1", fake.shutdowns)
	}
}

func TestAutomationShutdownKeepsStoreWhileCleanupIsInFlight(t *testing.T) {
	app := NewApp()
	fake := &fakeAutomationService{shutdownErr: &automation.ShutdownError{Err: context.DeadlineExceeded}}
	app.automationService = fake
	store := automation.NewPersistentStore(t.TempDir())
	app.automationStore = store
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.shutdownAutomation(ctx)
	if app.automationStore != store || fake.shutdownContextErr != nil {
		t.Fatal("shutdown closed an in-flight Store or reused the canceled execution context")
	}
	fake.shutdownErr = &automation.ShutdownError{Drained: true, Err: context.Canceled}
	app.shutdownAutomation(ctx)
	if app.automationStore != nil {
		t.Fatal("drained Store was not released")
	}
}

type fakeAutomationService struct {
	tasks []automation.TaskView
	runs  []automation.RunSummary
	page  automation.RunPage

	trigger automation.TriggerNowResponse
	detail  automation.RunDetail
	export  automation.RunResultExport
	cancel  automation.CancelRunResponse

	savedTasks         []automation.Task
	savedTemplates     []automation.Task
	triggerTaskID      string
	runFilter          automation.RunFilter
	runV2Filter        automation.RunFilter
	runV2Cursor        string
	readTaskID         string
	readRunID          string
	renameRunReq       automation.RenameRunRequest
	trashRunReq        automation.TrashRunRequest
	restoreRunReq      automation.RestoreRunRequest
	exportReq          automation.RunResultExportRequest
	cancelReq          automation.CancelRunRequest
	deletedTemplateID  string
	shutdowns          int
	shutdownErr        error
	shutdownContextErr error
	saveCalls          int
	createCalls        int
	updateCalls        int
	updatedTasks       []automation.Task
}

func (s *fakeAutomationService) MigrationStatus(context.Context) (automation.HeartbeatMigrationRunResult, error) {
	return automation.HeartbeatMigrationRunResult{Status: automation.HeartbeatMigrationStatusNotFound}, nil
}

func (s *fakeAutomationService) ListTasks(context.Context) ([]automation.TaskView, error) {
	return s.tasks, nil
}

func (s *fakeAutomationService) SaveTasks(_ context.Context, req automation.SaveTasksRequest) error {
	s.saveCalls++
	s.savedTasks = req.Tasks
	return nil
}

func (s *fakeAutomationService) CreateTask(_ context.Context, req automation.CreateTaskRequest) (automation.TaskView, error) {
	s.createCalls++
	s.savedTasks = append(s.savedTasks, req.Task)
	return automation.TaskView{Task: req.Task}, nil
}

func (s *fakeAutomationService) UpdateTask(_ context.Context, req automation.UpdateTaskRequest) (automation.TaskView, error) {
	s.updateCalls++
	s.updatedTasks = append(s.updatedTasks, req.Task)
	return automation.TaskView{Task: req.Task}, nil
}

func (s *fakeAutomationService) SetTaskEnabled(_ context.Context, req automation.SetTaskEnabledRequest) (automation.TaskView, error) {
	return automation.TaskView{Task: automation.Task{ID: req.ID, Enabled: req.Enabled, Revision: req.ExpectedRevision + 1}}, nil
}

func (s *fakeAutomationService) DeleteTask(context.Context, automation.DeleteTaskRequest) error {
	return nil
}

func (s *fakeAutomationService) ListTemplates(context.Context) ([]automation.TaskTemplate, error) {
	return automation.DefaultTaskTemplates(), nil
}

func (s *fakeAutomationService) SaveTaskTemplate(_ context.Context, req automation.SaveTaskTemplateRequest) (automation.TaskTemplate, error) {
	s.savedTemplates = append(s.savedTemplates, req.Task)
	return automation.TaskTemplateFromTask(req.Task), nil
}

func (s *fakeAutomationService) DeleteTaskTemplate(_ context.Context, req automation.DeleteTaskTemplateRequest) error {
	s.deletedTemplateID = req.ID
	return nil
}

func (s *fakeAutomationService) RestoreTaskTemplate(context.Context, automation.RestoreTaskTemplateRequest) error {
	return nil
}

func (s *fakeAutomationService) TriggerNow(_ context.Context, req automation.TriggerNowRequest) (automation.TriggerNowResponse, error) {
	s.triggerTaskID = req.TaskID
	return s.trigger, nil
}

func (s *fakeAutomationService) ListRuns(_ context.Context, filter automation.RunFilter) ([]automation.RunSummary, error) {
	s.runFilter = filter
	return s.runs, nil
}

func (s *fakeAutomationService) ListRunsV2(_ context.Context, filter automation.RunFilter, cursor string) (automation.RunPage, error) {
	s.runV2Filter = filter
	s.runV2Cursor = cursor
	if s.page.Items != nil {
		return s.page, nil
	}
	return automation.RunPage{Items: s.runs}, nil
}

func (s *fakeAutomationService) ReadRun(_ context.Context, taskID, runID string) (automation.RunDetail, error) {
	s.readTaskID = taskID
	s.readRunID = runID
	return s.detail, nil
}

func (s *fakeAutomationService) RenameRun(_ context.Context, req automation.RenameRunRequest) (automation.RunManifest, error) {
	s.renameRunReq = req
	scope := req.Scope
	if scope == "" {
		scope = "project"
	}
	return automation.RunManifest{TaskID: req.TaskID, RunID: req.RunID, Scope: scope, DisplayName: req.DisplayName}, nil
}

func (s *fakeAutomationService) TrashRun(_ context.Context, req automation.TrashRunRequest) (automation.RunManifest, error) {
	s.trashRunReq = req
	scope := req.Scope
	if scope == "" {
		scope = "project"
	}
	return automation.RunManifest{TaskID: req.TaskID, RunID: req.RunID, Scope: scope}, nil
}

func (s *fakeAutomationService) RestoreRun(_ context.Context, req automation.RestoreRunRequest) (automation.RunManifest, error) {
	s.restoreRunReq = req
	scope := req.Scope
	if scope == "" {
		scope = "project"
	}
	return automation.RunManifest{TaskID: req.TaskID, RunID: req.RunID, Scope: scope}, nil
}

func (s *fakeAutomationService) ListDeliveries(context.Context, string, string) ([]automation.DeliveryAttempt, error) {
	return []automation.DeliveryAttempt{}, nil
}

func (s *fakeAutomationService) RetryDelivery(_ context.Context, req automation.RetryDeliveryRequest) (automation.DeliveryAttempt, error) {
	return automation.DeliveryAttempt{ID: req.AttemptID, TaskID: req.TaskID, RunID: req.RunID}, nil
}

func (s *fakeAutomationService) ExportRunResult(_ context.Context, req automation.RunResultExportRequest) (automation.RunResultExport, error) {
	s.exportReq = req
	if s.export.Filename == "" {
		return automation.RunResultExport{Filename: "result." + req.Format, MimeType: "application/octet-stream", Payload: "cmVzdWx0", Base64Encoded: true}, nil
	}
	return s.export, nil
}

func (s *fakeAutomationService) CancelRun(_ context.Context, req automation.CancelRunRequest) (automation.CancelRunResponse, error) {
	s.cancelReq = req
	return s.cancel, nil
}

func (s *fakeAutomationService) Shutdown(ctx context.Context) error {
	s.shutdowns++
	s.shutdownContextErr = ctx.Err()
	return s.shutdownErr
}
