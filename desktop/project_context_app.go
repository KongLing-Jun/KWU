package main

import (
	"fmt"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/project"
	"kwu/internal/provider"
)

// ProjectContextInspectionView is the only ordinary Desktop seam that exposes
// a project instruction body. ProjectContextForTab authorizes it against the
// current exact Cloud scope and a live active-project read first.
type ProjectContextInspectionView struct {
	ProjectID   string `json:"projectId"`
	Source      string `json:"source"`
	Version     uint64 `json:"version"`
	ContentHash string `json:"contentHash"`
	Body        string `json:"body"`
}

func (a *App) ProjectContextForTab(tabID string) (ProjectContextInspectionView, error) {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return ProjectContextInspectionView{}, fmt.Errorf("tab not found")
	}
	ctrl := tab.Ctrl
	sessionPath := strings.TrimSpace(tab.SessionPath)
	a.mu.RUnlock()
	if ctrl != nil {
		sessionPath = strings.TrimSpace(ctrl.SessionPath())
	}
	if sessionPath == "" {
		return ProjectContextInspectionView{}, fmt.Errorf("project context session is unavailable")
	}

	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil || !ok || meta.ValidateProjectContextPin() != nil || !meta.IsCloudProjectBound() {
		return ProjectContextInspectionView{}, fmt.Errorf("project context identity is unavailable")
	}
	services, err := a.projectServicesForOperation()
	if err != nil {
		return ProjectContextInspectionView{}, err
	}
	pin := meta.ProjectContext
	if !projectContextPinMatchesScope(*pin, services.Scope) {
		return ProjectContextInspectionView{}, fmt.Errorf("project context is outside the current cloud scope")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	current, err := services.Gateway.GetProject(ctx, pin.ProjectID)
	if err != nil || current.Project.Status != project.ProjectActive {
		return ProjectContextInspectionView{}, fmt.Errorf("project context is not currently authorized")
	}

	var messages []provider.Message
	if ctrl != nil {
		messages = ctrl.History()
	} else {
		loaded, loadErr := agent.LoadSession(sessionPath)
		if loadErr != nil {
			return ProjectContextInspectionView{}, loadErr
		}
		messages = loaded.Snapshot()
	}
	inspection := agent.InspectProjectContext(messages)
	if inspection == nil || inspection.Version != pin.InstructionVersion || inspection.ContentHash != pin.ContentHash {
		return ProjectContextInspectionView{}, fmt.Errorf("project context transcript does not match its pinned identity")
	}
	contextCount := 0
	for _, message := range messages {
		if !message.IsProjectContext() {
			continue
		}
		contextCount++
		if contextCount > 1 || control.ValidateProjectContextMessage(message) != nil {
			return ProjectContextInspectionView{}, fmt.Errorf("project context transcript is invalid")
		}
	}
	return ProjectContextInspectionView{
		ProjectID: pin.ProjectID, Source: inspection.Source, Version: inspection.Version,
		ContentHash: inspection.ContentHash, Body: inspection.Body,
	}, nil
}

func projectContextPinMatchesScope(pin agent.ProjectContextPin, scope project.Scope) bool {
	return pin.Environment.EnvironmentID == scope.Environment.EnvironmentID &&
		pin.Environment.CloudBaseURL == scope.Environment.CloudBaseURL &&
		pin.BindingScope.ProfileID == scope.Binding.ProfileID &&
		pin.BindingScope.CloudUserID == scope.Binding.CloudUserID &&
		pin.BindingScope.BindingGeneration == scope.Binding.BindingGeneration
}
