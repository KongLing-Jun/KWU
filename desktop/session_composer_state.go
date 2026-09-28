package main

import (
	"strings"

	"kwu/internal/agent"
	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

type sessionComposerState struct {
	model                 string
	billingSource         modelbilling.BillingSource
	effort                *string
	effortKnown           bool
	identityKnown         bool
	toolApprovalMode      string
	toolApprovalModeKnown bool
}

func inferredBillingSourceForModel(model string) (modelbilling.BillingSource, bool) {
	providerName, modelName, ok := strings.Cut(strings.TrimSpace(model), "/")
	if !ok || strings.TrimSpace(providerName) == "" || strings.TrimSpace(modelName) == "" {
		return "", false
	}
	if providerName == brand.ProviderID {
		return modelbilling.BillingSubscriptionPool, true
	}
	return modelbilling.BillingNewAPIByok, true
}

func composerStateFromValues(model, source string, effort *string) (sessionComposerState, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return sessionComposerState{}, false
	}
	billingSource := modelbilling.BillingSource(strings.TrimSpace(source))
	if billingSource.Validate() != nil {
		var ok bool
		billingSource, ok = inferredBillingSourceForModel(model)
		if !ok {
			return sessionComposerState{}, false
		}
	}
	state := sessionComposerState{
		model: model, billingSource: billingSource, effort: cloneStringPtr(effort), effortKnown: effort != nil,
		identityKnown: true, toolApprovalMode: control.ToolApprovalAsk,
	}
	if state.effort == nil {
		auto := ""
		state.effort = &auto
	}
	return state, true
}

func loadSessionComposerState(sessionPath string) (sessionComposerState, bool) {
	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil || !ok {
		return sessionComposerState{}, false
	}
	state, valid := composerStateFromValues(meta.Model, meta.BillingSource, meta.Effort)
	if !valid {
		state = sessionComposerState{toolApprovalMode: control.ToolApprovalAsk}
	}
	applyPersistedToolApprovalMode(&state, meta.ToolApprovalMode)
	return state, true
}

func loadSessionComposerStateWithFallback(sessionPath, model, source string, effort *string) (sessionComposerState, bool) {
	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err == nil && ok {
		if strings.TrimSpace(meta.Model) != "" {
			model = meta.Model
			if strings.TrimSpace(meta.BillingSource) == "" && !topicModelMatchesBillingSource(model, modelbilling.BillingSource(strings.TrimSpace(source))) {
				source = ""
			}
		}
		if strings.TrimSpace(meta.BillingSource) != "" {
			source = meta.BillingSource
		}
		if meta.Effort != nil {
			effort = meta.Effort
		}
	}
	state, valid := composerStateFromValues(model, source, effort)
	if valid && err == nil && ok {
		applyPersistedToolApprovalMode(&state, meta.ToolApprovalMode)
	}
	return state, valid
}

func applyPersistedToolApprovalMode(state *sessionComposerState, persisted *string) {
	if state == nil || persisted == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(*persisted)) {
	case control.ToolApprovalAsk, control.ToolApprovalAuto, control.ToolApprovalYolo:
		state.toolApprovalMode = strings.ToLower(strings.TrimSpace(*persisted))
		state.toolApprovalModeKnown = true
	}
}

func applySessionComposerState(tab *WorkspaceTab, state sessionComposerState) {
	if tab == nil {
		return
	}
	if state.identityKnown {
		tab.model = state.model
		tab.billingSource = string(state.billingSource)
		tab.effort = cloneStringPtr(state.effort)
	}
	tab.toolApprovalMode = normalizeToolApprovalMode(state.toolApprovalMode)
	tab.toolApprovalModeKnown = state.toolApprovalModeKnown
	tab.mode = tabModeFromAxes(tabModeHasPlan(currentTabMode(tab)), tab.toolApprovalMode == control.ToolApprovalYolo)
}

func (a *App) defaultEffortForNewSession(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource, model string) *string {
	cfg, err := a.desktopConfigForBillingRoute(workspaceRoot, actor, source)
	if err != nil {
		return nil
	}
	entry, ok := cfg.ResolveModel(model)
	if !ok || !config.EffortCapabilityForEntry(entry).Supported {
		return nil
	}
	effort, err := config.NormalizeEffort(entry, config.EffortDisplay(entry))
	if err != nil {
		return nil
	}
	return &effort
}
