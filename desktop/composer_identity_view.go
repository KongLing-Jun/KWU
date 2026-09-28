package main

import (
	"strings"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/control"
)

// composerIdentityView is read-only presentation metadata used to render the
// target Composer before its runtime activation finishes. Closed sessions keep
// the same safe defaults used by openTopicTabWithActivationAndActorForRoute;
// runtime-only approval and collaboration state is never persisted here.
type composerIdentityView struct {
	modelRef          string
	billingSource     string
	effort            *string
	expertID          string
	expertName        string
	collaborationMode string
	toolApprovalMode  string
	tokenMode         string
	goal              string
	goalStatus        string
}

func defaultComposerIdentityView() composerIdentityView {
	return composerIdentityView{
		collaborationMode: "normal",
		toolApprovalMode:  control.ToolApprovalAsk,
		tokenMode:         boot.TokenModeFull,
		goalStatus:        control.GoalStatusStopped,
	}
}

func storedSessionComposerIdentityView(info agent.SessionInfo) composerIdentityView {
	view := defaultComposerIdentityView()
	view.expertID = strings.TrimSpace(info.ExpertID)
	view.expertName = strings.TrimSpace(info.ExpertName)
	if state, ok := loadSessionComposerState(info.Path); ok {
		view.modelRef = state.model
		view.billingSource = string(state.billingSource)
		if state.effortKnown {
			view.effort = cloneStringPtr(state.effort)
		}
	}
	return view
}

func runtimeComposerIdentityView(tab *WorkspaceTab, fallback composerIdentityView) composerIdentityView {
	if tab == nil {
		return fallback
	}
	view := fallback
	view.modelRef = strings.TrimSpace(tab.model)
	view.billingSource = strings.TrimSpace(tab.billingSource)
	view.effort = cloneStringPtr(tab.effort)
	view.collaborationMode = currentTabCollaborationMode(tab)
	view.toolApprovalMode = currentTabToolApprovalMode(tab)
	view.tokenMode = currentTabTokenMode(tab)
	view.goal = strings.TrimSpace(currentTabGoal(tab))
	view.goalStatus = currentTabGoalStatus(tab)
	if state, ok := currentExpertState(tab.Ctrl); ok && strings.TrimSpace(state.SelectedID) != "" {
		expertID := strings.TrimSpace(state.SelectedID)
		if expertID != view.expertID {
			view.expertName = ""
		}
		view.expertID = expertID
		if expertName := strings.TrimSpace(state.SelectedName); expertName != "" {
			view.expertName = expertName
		}
	}
	return view
}

func applyComposerIdentityToSessionMeta(meta *SessionMeta, view composerIdentityView) {
	if meta == nil {
		return
	}
	meta.ModelRef = view.modelRef
	meta.BillingSource = view.billingSource
	meta.Effort = cloneStringPtr(view.effort)
	meta.ExpertID = view.expertID
	meta.ExpertName = view.expertName
	meta.CollaborationMode = view.collaborationMode
	meta.ToolApprovalMode = view.toolApprovalMode
	meta.TokenMode = view.tokenMode
	meta.Goal = view.goal
	meta.GoalStatus = view.goalStatus
}

func applyComposerIdentityToProjectNode(node *ProjectNode, view composerIdentityView) {
	if node == nil {
		return
	}
	node.ModelRef = view.modelRef
	node.BillingSource = view.billingSource
	node.Effort = cloneStringPtr(view.effort)
	node.ExpertID = view.expertID
	node.ExpertName = view.expertName
	node.CollaborationMode = view.collaborationMode
	node.ToolApprovalMode = view.toolApprovalMode
	node.TokenMode = view.tokenMode
	node.Goal = view.goal
	node.GoalStatus = view.goalStatus
}
