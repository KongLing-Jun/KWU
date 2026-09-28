package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/automation"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
)

type desktopAutomationRunner struct {
	app *App
}

func newDesktopAutomationRunner(app *App) automation.Runner {
	return &desktopAutomationRunner{app: app}
}

func (r *desktopAutomationRunner) Run(ctx context.Context, req automation.RunRequest) (automation.RunResult, error) {
	if r == nil || r.app == nil {
		err := errors.New("automation: desktop app is required")
		return automation.RunResult{Status: automation.RunStatusFailed, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err), Error: err.Error()}, err
	}
	task, route, err := r.app.resolveAutomationTask(req.Task)
	if err != nil {
		return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err), Error: err.Error()}, err
	}
	if projectID := strings.TrimSpace(task.ProjectID); projectID != "" && strings.TrimSpace(task.WorkspaceRoot) == "" {
		services, servicesErr := r.app.projectServicesForOperation()
		if servicesErr != nil {
			return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(servicesErr), Error: servicesErr.Error()}, servicesErr
		}
		projectCtx, projectCancel := services.operationContext(ctx)
		workspaceRoot, workspaceErr := ensureCloudProjectWorkspace(projectCtx, services, projectID)
		projectCancel()
		if workspaceErr != nil {
			wrapped := fmt.Errorf("automation: prepare dedicated project workspace: %w", workspaceErr)
			return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(wrapped), Error: wrapped.Error()}, wrapped
		}
		task.WorkspaceRoot = workspaceRoot
	}
	req.Task = task
	task, tab, err := r.app.bindAutomationTaskTopic(task, route)
	if err != nil {
		return automation.RunResult{Status: automation.RunStatusFailed, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err), Error: err.Error()}, err
	}
	rebuildRoute, waitForCurrentBuild := r.app.automationTopicRouteRepairState(tab.ID, route)
	if waitForCurrentBuild {
		_, waitErr := r.app.waitAutomationTopicController(ctx, tab.ID)
		if waitErr != nil {
			if !r.app.automationTopicControllerWaitNeedsRouteRepair(ctx, tab.ID, waitErr) {
				return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(waitErr), Error: waitErr.Error()}, waitErr
			}
			rebuildRoute = true
		} else if ctx.Err() == nil {
			rebuildRoute, _ = r.app.automationTopicRouteRepairState(tab.ID, route)
		} else {
			rebuildRoute = false
		}
	}
	if rebuildRoute {
		if err := r.app.setModelRouteForTabWithPersistence(tab.ID, route.ModelRef, route.BillingSource, false); err != nil {
			return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err), Error: err.Error()}, err
		}
	}
	ctrl, err := r.app.waitAutomationTopicController(ctx, tab.ID)
	if err != nil {
		return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err), Error: err.Error()}, err
	}
	if err := r.app.verifyAutomationTopicRoute(tab.ID, route); err != nil {
		return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err), Error: err.Error()}, err
	}
	if heartbeatControllerBusy(ctrl) {
		return automation.RunResult{
			Status:       automation.RunStatusSkipped,
			ErrorCode:    automation.RunErrorLeaseUnavailable,
			ErrorMessage: "automation topic is busy",
			Summary:      "automation topic is busy",
			SessionPath:  ctrl.SessionPath(),
		}, nil
	}
	if projectID := strings.TrimSpace(task.ProjectID); projectID != "" {
		executionTarget := "cloud"
		workspacePath := ""
		if strings.TrimSpace(task.WorkspaceRoot) != "" {
			executionTarget = "local"
			workspacePath = task.WorkspaceRoot
		}
		_, bindErr := r.app.NewCloudProjectSession(NewCloudProjectSessionInput{
			ProjectID: projectID, TabID: tab.ID, WorkspacePath: workspacePath, ExecutionTarget: executionTarget,
		})
		r.app.clearPendingAutomationCloudProject(tab.ID, projectID)
		if bindErr != nil {
			wrapped := fmt.Errorf("automation: prepare cloud project context: %w", bindErr)
			return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(wrapped), Error: wrapped.Error()}, wrapped
		}
	}

	baseHistoryLen := len(ctrl.History())
	previousApprovalMode := ctrl.ToolApprovalMode()
	ctrl.SetToolApprovalMode(string(automation.ApprovalModeAuto))
	defer ctrl.SetToolApprovalMode(previousApprovalMode)
	if mode := strings.TrimSpace(string(task.Execution.ApprovalMode)); mode != "" {
		ctrl.SetToolApprovalMode(mode)
	}
	manifest := req.Manifest
	manifest.WorkspaceRoot = task.WorkspaceRoot
	assist, assistPath, err := automation.PrepareAutomationAssist(task, req.RunDir, ctrl)
	if err != nil {
		result := desktopAutomationFailed(err)
		result.SessionPath = ctrl.SessionPath()
		result.Usage = ctrl.LastUsage()
		return result, err
	}
	automation.EmitAutomationHelperNotice(req.Sink, task, assistPath)
	capabilityRunner, ok := ctrl.(interface {
		RunCapabilityTurn(context.Context, control.CapabilityTurn) error
	})
	if !ok {
		err := errors.New("automation: controller cannot enforce the capability snapshot")
		result := desktopAutomationFailed(err)
		result.SessionPath = ctrl.SessionPath()
		return result, err
	}
	runErr := capabilityRunner.RunCapabilityTurn(ctx, control.CapabilityTurn{
		Input:          automation.PromptCompiler{}.CompileWithOptions(task, manifest, automation.PromptCompileOptions{AssistPath: assistPath, Assist: assist}),
		Display:        strings.TrimSpace(task.Prompt.Prompt),
		SkillNames:     append([]string(nil), manifest.Capabilities.SkillIDs...),
		MCPServerNames: append([]string(nil), manifest.Capabilities.ConnectorIDs...),
		ToolAllowlist:  append([]string(nil), manifest.Capabilities.ToolAllowlist...),
		RestrictTools:  true,
	})
	snapshotErr := ctrl.Snapshot()
	history := ctrl.History()
	if baseHistoryLen > 0 && baseHistoryLen <= len(history) {
		history = history[baseHistoryLen:]
	}
	extracted := automation.ExtractResult(nil, history)
	extractable := desktopAutomationHasAssistantResult(history)
	resultPath := filepath.Join(req.RunDir, "result.md")
	status := desktopAutomationRunStatus(runErr)
	var writeErr error
	if status == automation.RunStatusSucceeded || extractable {
		writeErr = automation.WriteResult(resultPath, extracted)
	}
	err = errors.Join(runErr, snapshotErr, writeErr)

	result := automation.RunResult{
		Status:      status,
		SessionPath: ctrl.SessionPath(),
		Usage:       ctrl.LastUsage(),
	}
	if status == automation.RunStatusSucceeded || extractable {
		result.Summary = extracted.Summary
	}
	if writeErr == nil && (status == automation.RunStatusSucceeded || extractable) {
		result.ResultPath = resultPath
	}
	if err != nil {
		if snapshotErr != nil {
			result.Status = automation.RunStatusFailed
			result.ErrorCode = automation.RunErrorSnapshotFailed
			result.ErrorMessage = desktopAutomationUserMessage(snapshotErr)
		} else if writeErr != nil {
			result.Status = automation.RunStatusFailed
			result.ErrorCode = automation.RunErrorArchiveFailed
			result.ErrorMessage = desktopAutomationUserMessage(writeErr)
		} else {
			classified := desktopAutomationClassifyRunError(runErr)
			result.Status = classified.Status
			result.ErrorCode = classified.ErrorCode
			result.ErrorMessage = classified.ErrorMessage
		}
		result.Error = err.Error()
	}
	return result, err
}

// automationTopicRouteRepairState preserves the ordinary wait for a controller
// that is still building. A restored tab whose build already failed (or whose
// controller is otherwise missing after Ready) can instead use the existing
// atomic route rebuild path and recover without waiting for a controller that
// will never arrive.
func (a *App) automationTopicRouteRepairState(tabID string, route modelbilling.BillingRouteSelection) (rebuild, waitForCurrentBuild bool) {
	if a == nil {
		return false, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		return false, false
	}
	routeMismatch := strings.TrimSpace(tab.model) != strings.TrimSpace(route.ModelRef) ||
		strings.TrimSpace(tab.billingSource) != strings.TrimSpace(string(route.BillingSource))
	if tab.Ctrl != nil {
		return routeMismatch, false
	}
	if strings.TrimSpace(tab.StartupErr) != "" || tab.Ready {
		return true, false
	}
	return false, true
}

func (a *App) automationTopicControllerWaitNeedsRouteRepair(ctx context.Context, tabID string, waitErr error) bool {
	if (ctx != nil && ctx.Err() != nil) || errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded) {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab := a.tabByIDLocked(tabID)
	return tab != nil && tab.Ctrl == nil && (strings.TrimSpace(tab.StartupErr) != "" || tab.Ready)
}

func (a *App) verifyAutomationTopicRoute(tabID string, route modelbilling.BillingRouteSelection) error {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return fmt.Errorf("automation: topic %q is unavailable", tabID)
	}
	model := strings.TrimSpace(tab.model)
	billingSource := strings.TrimSpace(tab.billingSource)
	a.mu.RUnlock()
	if model != strings.TrimSpace(route.ModelRef) || billingSource != strings.TrimSpace(string(route.BillingSource)) {
		return fmt.Errorf("automation: topic route mismatch: task %q/%q, tab %q/%q", route.ModelRef, route.BillingSource, model, billingSource)
	}
	return nil
}

func desktopAutomationFailed(err error) automation.RunResult {
	return automation.RunResult{
		Status:       automation.RunStatusFailed,
		ErrorCode:    automation.RunErrorRunnerFailed,
		ErrorMessage: desktopAutomationUserMessage(err),
		Error:        err.Error(),
	}
}

func desktopAutomationUserMessage(err error) string {
	if err == nil {
		return ""
	}
	return control.ExplainError(err).Error()
}

const automationTopicControllerWaitTimeout = time.Minute

func (a *App) waitAutomationTopicController(ctx context.Context, tabID string) (control.SessionAPI, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(automationTopicControllerWaitTimeout)
	defer timer.Stop()
	for {
		a.mu.RLock()
		tab := a.tabByIDLocked(tabID)
		if tab == nil {
			a.mu.RUnlock()
			return nil, fmt.Errorf("automation: topic %q is unavailable", tabID)
		}
		ctrl := tab.Ctrl
		ready := tab.Ready
		startupErr := strings.TrimSpace(tab.StartupErr)
		a.mu.RUnlock()
		if ctrl != nil {
			return ctrl, nil
		}
		if startupErr != "" {
			return nil, fmt.Errorf("automation: topic controller failed to start: %s", startupErr)
		}
		if ready {
			return nil, errors.New("automation: topic controller not ready")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, errors.New("automation: topic controller startup timed out")
		case <-ticker.C:
		}
	}
}

func desktopAutomationRunStatus(err error) automation.RunStatus {
	return desktopAutomationClassifyRunError(err).Status
}

func desktopAutomationClassifyRunError(err error) automation.RunResult {
	switch {
	case err == nil:
		return automation.RunResult{Status: automation.RunStatusSucceeded}
	case errors.Is(err, context.Canceled):
		return automation.RunResult{Status: automation.RunStatusCanceled, ErrorCode: automation.RunErrorCanceled, ErrorMessage: desktopAutomationUserMessage(err)}
	case errors.Is(err, context.DeadlineExceeded):
		return automation.RunResult{Status: automation.RunStatusBlocked, ErrorCode: automation.RunErrorApprovalTimeout, ErrorMessage: desktopAutomationUserMessage(err)}
	default:
		return automation.RunResult{Status: automation.RunStatusFailed, ErrorCode: automation.RunErrorRunnerFailed, ErrorMessage: desktopAutomationUserMessage(err)}
	}
}

func desktopAutomationHasAssistantResult(history []provider.Message) bool {
	for _, msg := range history {
		if msg.Role == provider.RoleAssistant && strings.TrimSpace(msg.Content) != "" {
			return true
		}
	}
	return false
}
