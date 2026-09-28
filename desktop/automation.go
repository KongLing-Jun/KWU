package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"kwu/internal/automation"
	"kwu/internal/modelbilling"
)

func (a *App) startAutomation(ctx context.Context) {
	_, engine := a.ensureAutomationService()
	if engine != nil {
		engine.Start(ctx)
	}
	a.emitRuntimeEvent("automation:catalog_changed", map[string]string{"reason": "startup"})
}

func (a *App) shutdownAutomation(ctx context.Context) {
	a.automationMu.Lock()
	svc := a.automationService
	a.automationMu.Unlock()
	if svc != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := svc.Shutdown(cleanupCtx); err != nil {
			log.Printf("[automation] shutdown: %v", err)
			var shutdownErr *automation.ShutdownError
			if !errors.As(err, &shutdownErr) || !shutdownErr.Drained {
				// OnShutdown cannot veto process exit. Keep the Store available to
				// remaining cleanup until exit; startup recovery handles interruption.
				return
			}
		}
	}
	a.automationMu.Lock()
	store := a.automationStore
	a.automationStore = nil
	a.automationMu.Unlock()
	if store != nil {
		_ = store.Close()
	}
}

func (a *App) ensureAutomationService() (automation.Service, *automation.Engine) {
	a.automationMu.Lock()
	defer a.automationMu.Unlock()
	if a.automationService != nil {
		return a.automationService, a.automationEngine
	}
	store := automation.NewLocalPersistentStore("", probeDesktopAutomationOwner)
	if _, err := store.RecoverInterruptedRuns(context.Background(), ""); err != nil {
		log.Printf("[automation] startup recovery: %v", err)
	}
	runner := newDesktopAutomationRunner(a)
	engine := automation.NewEngineWithChecker(store, automationDesktopOwner(), runner, automation.Checker{
		Preflight: a.automationTaskPreflight,
		TaskResolver: func(task automation.Task) (automation.Task, error) {
			resolved, _, err := a.resolveAutomationTask(task)
			return resolved, err
		},
		ModelResolver: func(task automation.Task) error {
			_, err := a.resolveAutomationTaskRoute(task)
			return err
		},
		CapabilityResolver: a.resolveAutomationCapabilities,
	})
	botDelivery := newDesktopAutomationDeliveryAdapter(a)
	for _, kind := range desktopAutomationDeliveryAdapterTypes() {
		if err := engine.RegisterDeliveryAdapter(kind, botDelivery); err != nil {
			log.Printf("[automation] register %s delivery adapter: %v", kind, err)
		}
	}

	engine.SetChangeObserver(func(change automation.EngineChange) {
		if change.Kind == "delivery" {
			a.emitRuntimeEvent("automation:delivery_changed", change)
			return
		}
		if change.Kind == "run_event" {
			a.emitRuntimeEvent("automation:run_event", change)
			return
		}
		a.emitRuntimeEvent("automation:run_changed", change)
	})
	svc := automation.NewServiceWithTaskQuota(store, engine, a.automationTaskQuotaResolver())
	logAutomationHeartbeatMigrationStatus(svc)
	a.automationEngine = engine
	a.automationService = svc
	a.automationStore = store
	return svc, engine
}

func (a *App) automationTaskPreflight(task automation.Task) automation.PreflightResult {
	projectID := strings.TrimSpace(task.ProjectID)
	workspaceRoot := strings.TrimSpace(task.WorkspaceRoot)
	legacyManagedTarget := projectID == "" && strings.TrimSpace(task.Scope) == "project" && workspaceRoot != "" &&
		managedCloudProjectWorkspaceInStateRoots(workspaceRoot, a.cloudProjectWorkspaceStateRoots())
	if !legacyManagedTarget && (projectID == "" || workspaceRoot != "") {
		return automation.PreflightResult{Disposition: automation.PreflightReady}
	}
	if a == nil || a.cloudRuntime == nil || !a.cloudRuntime.available || a.cloudRuntime.closed.Load() || a.cloudRuntime.initErr != nil {
		return automation.PreflightResult{Disposition: automation.PreflightReady}
	}
	if a.cloudRuntime.projectReadinessSnapshot().Phase == cloudProjectReadinessPending {
		return automation.PreflightResult{
			Disposition: automation.PreflightDefer,
			Reason:      "cloud project services are initializing",
		}
	}
	return automation.PreflightResult{Disposition: automation.PreflightReady}
}

func (a *App) startCloudProjectBootstrap() {
	if a == nil || a.cloudRuntime == nil || !a.cloudRuntime.available || a.cloudRuntime.closed.Load() || a.cloudRuntime.initErr != nil {
		return
	}
	a.goSafe("bootstrapCloudProjects", func() {
		if _, _, err := a.runSharedCloudBootstrap("cloud.bootstrap", true); err != nil {
			log.Printf("[cloud] backend project bootstrap: %v", err)
		}
	})
}

func desktopAutomationDeliveryAdapterTypes() []string {
	return []string{
		desktopAutomationDeliveryTypeFeishu,
		desktopAutomationDeliveryTypeLark,
		desktopAutomationDeliveryTypeWeixinILink,
		desktopAutomationDeliveryTypeWeCom,
		"weixin",
		"ilink",
	}
}

func (a *App) resolveAutomationTask(task automation.Task) (automation.Task, modelbilling.BillingRouteSelection, error) {
	task, err := a.normalizeManagedAutomationTaskTarget(task)
	if err != nil {
		return automation.Task{}, modelbilling.BillingRouteSelection{}, err
	}
	route, err := a.resolveAutomationTaskRoute(task)
	if err != nil {
		return automation.Task{}, modelbilling.BillingRouteSelection{}, err
	}
	task.Prompt.Model = route.ModelRef
	task.Prompt.BillingSource = string(route.BillingSource)
	task.Capabilities.ModelRef = route.ModelRef
	return task, route, nil
}

func (a *App) normalizeManagedAutomationTaskTarget(task automation.Task) (automation.Task, error) {
	if strings.TrimSpace(task.ProjectID) != "" {
		task.Scope = "project"
		return task, nil
	}
	if strings.TrimSpace(task.Scope) != "project" || strings.TrimSpace(task.WorkspaceRoot) == "" {
		return task, nil
	}
	workspaceRoot := normalizeProjectRoot(task.WorkspaceRoot)
	catalog := a.cloudProjectTreeCatalog(nil)
	projectIDs := make(map[string]struct{})
	for managedRoot, projectID := range catalog.managedWorkspaceIDs {
		if sameDesktopPath(managedRoot, workspaceRoot) && strings.TrimSpace(projectID) != "" {
			projectIDs[projectID] = struct{}{}
		}
	}
	if len(projectIDs) == 1 {
		for projectID := range projectIDs {
			task.Scope = "project"
			task.ProjectID = projectID
			task.WorkspaceRoot = ""
			task.TopicID = ""
			return task, nil
		}
	}
	if len(projectIDs) > 1 {
		return automation.Task{}, fmt.Errorf("automation: managed cloud project workspace matches multiple projects")
	}
	if managedCloudProjectWorkspaceInStateRoots(workspaceRoot, a.cloudProjectWorkspaceStateRoots()) {
		return automation.Task{}, fmt.Errorf("automation: managed cloud project workspace is not associated with a visible project")
	}
	return task, nil
}

func (a *App) materializeAutomationTask(task automation.Task) (automation.Task, error) {
	scope, workspaceRoot := automationTopicScope(task)
	task.Scope = scope
	task.WorkspaceRoot = workspaceRoot
	resolved, _, err := a.resolveAutomationTask(task)
	if err != nil {
		return automation.Task{}, err
	}
	return resolved, nil
}

func (a *App) materializeAutomationTasks(tasks []automation.Task) ([]automation.Task, error) {
	materialized := make([]automation.Task, len(tasks))
	for i, task := range tasks {
		resolved, err := a.materializeAutomationTask(task)
		if err != nil {
			return nil, fmt.Errorf("automation task %d: %w", i, err)
		}
		materialized[i] = resolved
	}
	return materialized, nil
}

func (a *App) saveAutomationTasks(tasks []automation.Task) error {
	svc, _ := a.ensureAutomationService()
	return svc.SaveTasks(a.bootContext(), automation.SaveTasksRequest{Tasks: tasks})
}

// AutomationListTasks returns persisted automation tasks.
func (a *App) AutomationListTasks() ([]automation.TaskView, error) {
	return a.AutomationReloadTasks()
}

// AutomationReloadTasks reloads automation tasks from the shared automation store.
func (a *App) AutomationReloadTasks() ([]automation.TaskView, error) {
	svc, _ := a.ensureAutomationService()
	views, err := svc.ListTasks(a.bootContext())
	if err != nil {
		return nil, err
	}
	for index := range views {
		normalized, normalizeErr := a.normalizeManagedAutomationTaskTarget(views[index].Task)
		if normalizeErr == nil {
			views[index].Task = normalized
			if _, legacy := routePickerCompatibilityModel(normalized.Prompt.Model); legacy {
				if resolved, _, resolveErr := a.resolveAutomationTask(normalized); resolveErr == nil {
					views[index].Task = resolved
				}
			}
		}
	}
	return views, nil
}

// AutomationSaveTasks replaces the full automation task list.
func (a *App) AutomationSaveTasks(tasks []automation.Task) error {
	materialized, err := a.materializeAutomationTasks(tasks)
	if err != nil {
		return err
	}
	err = a.saveAutomationTasks(materialized)
	if err == nil {
		a.emitRuntimeEvent("automation:task_changed", map[string]string{"action": "bulk_saved"})
	}
	return err
}

// AutomationCreateTask creates one task using the revisioned Task V2 contract.
func (a *App) AutomationCreateTask(task automation.Task) (automation.Task, error) {
	materialized, err := a.materializeAutomationTask(task)
	if err != nil {
		return automation.Task{}, err
	}
	svc, _ := a.ensureAutomationService()
	view, err := svc.CreateTask(a.bootContext(), automation.CreateTaskRequest{Task: materialized})
	if err == nil {
		a.emitRuntimeEvent("automation:task_changed", map[string]any{"action": "created", "taskId": view.ID, "revision": view.Revision})
	}
	return view.Task, err
}

// AutomationUpdateTask updates one task when expectedRevision is still current.
func (a *App) AutomationUpdateTask(task automation.Task, expectedRevision int64) (automation.Task, error) {
	materialized, err := a.materializeAutomationTask(task)
	if err != nil {
		return automation.Task{}, err
	}
	svc, _ := a.ensureAutomationService()
	view, err := svc.UpdateTask(a.bootContext(), automation.UpdateTaskRequest{Task: materialized, ExpectedRevision: expectedRevision})
	if err == nil {
		a.emitRuntimeEvent("automation:task_changed", map[string]any{"action": "updated", "taskId": view.ID, "revision": view.Revision})
	}
	return view.Task, err
}

// AutomationSetTaskEnabled changes only the enabled flag under revision control.
func (a *App) AutomationSetTaskEnabled(id string, enabled bool, expectedRevision int64) (automation.Task, error) {
	svc, _ := a.ensureAutomationService()
	view, err := svc.SetTaskEnabled(a.bootContext(), automation.SetTaskEnabledRequest{ID: id, Enabled: enabled, ExpectedRevision: expectedRevision})
	if err == nil {
		a.emitRuntimeEvent("automation:task_changed", map[string]any{"action": "enabled_changed", "taskId": view.ID, "revision": view.Revision})
	}
	return view.Task, err
}

// AutomationDeleteTask removes one task when expectedRevision is still current.
func (a *App) AutomationDeleteTask(id string, expectedRevision int64) error {
	svc, _ := a.ensureAutomationService()
	err := svc.DeleteTask(a.bootContext(), automation.DeleteTaskRequest{ID: id, ExpectedRevision: expectedRevision})
	if err == nil {
		a.emitRuntimeEvent("automation:task_changed", map[string]any{"action": "deleted", "taskId": id, "revision": expectedRevision})
	}
	return err
}

type AutomationDraftValidation struct {
	Task     automation.Task `json:"task"`
	NextRuns []string        `json:"nextRuns"`
	Warnings []string        `json:"warnings,omitempty"`
}

// AutomationGetScheduleCatalog returns the backend-owned scheduling options.
func (a *App) AutomationGetScheduleCatalog() []automation.ScheduleCatalogEntry {
	return automation.ScheduleCatalog()
}

// AutomationListDeliveryTargets returns the profile- and workspace-scoped bot
// destinations that may be referenced by an automation task. The projection is
// intentionally opaque: remote IDs and credentials never cross the bridge.
func (a *App) AutomationListDeliveryTargets() ([]DesktopAutomationDeliveryTargetView, error) {
	return desktopAutomationDeliveryCatalogForApp(a.bootContext(), a, newDesktopAutomationDeliveryAdapter(a))
}

// AutomationValidateDraft normalizes and validates a draft, then previews its next runs.
func (a *App) AutomationValidateDraft(task automation.Task) (AutomationDraftValidation, error) {
	normalized, err := automation.NormalizeTask(task)
	if err != nil {
		return AutomationDraftValidation{}, err
	}
	nextRuns, err := automation.PreviewNextRuns(normalized, time.Now().UTC(), 3)
	if err != nil {
		return AutomationDraftValidation{}, err
	}
	formattedRuns := make([]string, len(nextRuns))
	for index, nextRun := range nextRuns {
		formattedRuns[index] = nextRun.UTC().Format(time.RFC3339)
	}
	return AutomationDraftValidation{
		Task:     normalized,
		NextRuns: formattedRuns,
		Warnings: []string{"local_desktop_online_required"},
	}, nil
}

// AutomationTaskQuota returns current local usage and plan-derived eligibility.
func (a *App) AutomationTaskQuota() (automation.TaskQuotaView, error) {
	svc, _ := a.ensureAutomationService()
	quotaService, ok := svc.(automation.TaskQuotaService)
	if !ok {
		return automation.TaskQuotaView{}, automation.ErrTaskQuotaUnavailable
	}
	return quotaService.TaskQuota(a.bootContext())
}

// AutomationTriggerNow manually runs a task through the shared automation engine.
func (a *App) AutomationTriggerNow(id string) (automation.TriggerNowResponse, error) {
	svc, _ := a.ensureAutomationService()
	return svc.TriggerNow(a.bootContext(), automation.TriggerNowRequest{TaskID: id, Reason: "desktop manual trigger"})
}

// AutomationListRuns returns recent archived runs for a task.
func (a *App) AutomationListRuns(taskID string, limit int) ([]automation.RunSummary, error) {
	svc, _ := a.ensureAutomationService()
	return svc.ListRuns(a.bootContext(), automation.RunFilter{TaskID: taskID, Limit: limit})
}

// AutomationListRunsV2 returns a cursor-addressed page across one or all tasks.
func (a *App) AutomationListRunsV2(filter automation.RunFilter, cursor string) (automation.RunPage, error) {
	svc, _ := a.ensureAutomationService()
	return svc.ListRunsV2(a.bootContext(), filter, cursor)
}

// AutomationReadRun returns one archived run detail.
func (a *App) AutomationReadRun(taskID, runID string) (automation.RunDetail, error) {
	svc, _ := a.ensureAutomationService()
	return svc.ReadRun(a.bootContext(), taskID, runID)
}

// AutomationListDeliveries returns notification delivery states for one run.
func (a *App) AutomationListDeliveries(taskID, runID string) ([]automation.DeliveryAttempt, error) {
	svc, _ := a.ensureAutomationService()
	return svc.ListDeliveries(a.bootContext(), taskID, runID)
}

// AutomationRetryDelivery retries a failed retryable notification as a new attempt.
func (a *App) AutomationRetryDelivery(taskID, runID, attemptID string) (automation.DeliveryAttempt, error) {
	svc, _ := a.ensureAutomationService()
	return svc.RetryDelivery(a.bootContext(), automation.RetryDeliveryRequest{TaskID: taskID, RunID: runID, AttemptID: attemptID})
}

// AutomationExportRunResult returns one archived run result in the requested file format.
func (a *App) AutomationExportRunResult(taskID, runID, format string) (automation.RunResultExport, error) {
	svc, _ := a.ensureAutomationService()
	return svc.ExportRunResult(a.bootContext(), automation.RunResultExportRequest{TaskID: taskID, RunID: runID, Format: format})
}

// AutomationCancelRun cancels a run owned by this desktop process.
func (a *App) AutomationCancelRun(taskID, runID string) (automation.CancelRunResponse, error) {
	svc, _ := a.ensureAutomationService()
	return svc.CancelRun(a.bootContext(), automation.CancelRunRequest{TaskID: taskID, RunID: runID})
}

// AutomationListTemplates returns built-in and user-saved task templates.
func (a *App) AutomationListTemplates() ([]automation.TaskTemplate, error) {
	svc, _ := a.ensureAutomationService()
	return svc.ListTemplates(a.bootContext())
}

// AutomationSaveTaskTemplate saves a manual automation task as a reusable template.
func (a *App) AutomationSaveTaskTemplate(task automation.Task) (automation.TaskTemplate, error) {
	svc, _ := a.ensureAutomationService()
	return svc.SaveTaskTemplate(a.bootContext(), automation.SaveTaskTemplateRequest{Task: task})
}

// AutomationDeleteTaskTemplate removes a user-saved task template.
func (a *App) AutomationDeleteTaskTemplate(id string) error {
	svc, _ := a.ensureAutomationService()
	return svc.DeleteTaskTemplate(a.bootContext(), automation.DeleteTaskTemplateRequest{ID: id})
}

// AutomationRestoreTaskTemplate restores a soft-deleted task template.
func (a *App) AutomationRestoreTaskTemplate(id string) error {
	svc, _ := a.ensureAutomationService()
	return svc.RestoreTaskTemplate(a.bootContext(), automation.RestoreTaskTemplateRequest{ID: id})
}

// AutomationGenerateID returns a random automation task id.
func (a *App) AutomationGenerateID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task-" + hex.EncodeToString(b[:])
}

// AutomationHeartbeatMigrationStatus returns the last startup migration result.
func (a *App) AutomationHeartbeatMigrationStatus() (automation.HeartbeatMigrationRunResult, error) {
	svc, _ := a.ensureAutomationService()
	return svc.MigrationStatus(a.bootContext())
}

func automationDesktopOwner() string {
	return fmt.Sprintf("desktop:%d", os.Getpid())
}

func logAutomationHeartbeatMigrationStatus(svc automation.Service) {
	status, err := svc.MigrationStatus(context.Background())
	if err != nil {
		log.Printf("[automation] heartbeat migration status unavailable: %v", err)
		return
	}
	if status.ErrorSummary != "" {
		log.Printf("[automation] heartbeat migration %s: %s", status.Status, status.ErrorSummary)
		return
	}
	if status.Notice != "" {
		log.Printf("[automation] heartbeat migration %s: %s", status.Status, status.Notice)
	}
}
