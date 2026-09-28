package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"kwu/internal/automation"
	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

var errAutomationModelUnavailable = errors.New("AUTOMATION_MODEL_UNAVAILABLE")

// AutomationModels returns the route-aware chat catalog available to the
// current Desktop profile. Official models require an authenticated MaaS
// account; custom BYOK providers remain available without cloud registration.
func (a *App) AutomationModels(workspaceRoot string) []ModelInfo {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		target, err := a.normalizeManagedAutomationTaskTarget(automation.Task{Scope: "project", WorkspaceRoot: workspaceRoot})
		if err != nil {
			return []ModelInfo{}
		}
		workspaceRoot = target.WorkspaceRoot
	}
	actor := a.activeConfigActorForWorkspace(workspaceRoot)
	return a.automationModelsForActor(workspaceRoot, actor)
}

func (a *App) automationModelsForActor(workspaceRoot string, actor userctx.ActorContext) []ModelInfo {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	out := make([]ModelInfo, 0)

	cfg, err := config.LoadForRootWithActor(workspaceRoot, actor)
	if err == nil {
		access := providerAccessSet(cfg.Desktop.ProviderAccess)
		for i := range cfg.Providers {
			provider := &cfg.Providers[i]
			if provider.Name == brand.ProviderID || !modelProviderAccessAllowed(access, provider.Name) || !provider.Configured() {
				continue
			}
			for _, model := range provider.ChatModelList() {
				out = append(out, ModelInfo{
					Ref: provider.Name + "/" + model, Provider: provider.Name, Model: model,
					BillingSource: string(modelbilling.BillingNewAPIByok),
				})
			}
		}
	}

	_, subscriptionModels, authorized := a.desktopAuthorizedSubscriptionCatalogForActor(actor)
	if !authorized {
		subscriptionModels = nil
	}
	for _, model := range subscriptionModels {
		out = append(out, ModelInfo{
			Ref: brand.ProviderID + "/" + model, Provider: "airclaude_subscription", Model: model,
			BillingSource: string(modelbilling.BillingSubscriptionPool),
		})
	}
	// Current identifies this target's default, never a fallback from the catalog.
	if err == nil {
		route, routeErr := exactAutomationRouteWithPickerCompatibility(out, strings.TrimSpace(cfg.DefaultModel), desktopSettingsDefaultBillingSource(cfg))
		if routeErr == nil {
			for i := range out {
				out[i].Current = out[i].Ref == route.ModelRef && out[i].BillingSource == string(route.BillingSource)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BillingSource != out[j].BillingSource {
			return out[i].BillingSource < out[j].BillingSource
		}
		return out[i].Ref < out[j].Ref
	})
	if out == nil {
		return []ModelInfo{}
	}
	return out
}

func (a *App) resolveAutomationTaskRoute(task automation.Task) (modelbilling.BillingRouteSelection, error) {
	model, source, err := a.automationTaskRouteInput(task)
	if err != nil {
		return modelbilling.BillingRouteSelection{}, err
	}
	candidates := a.AutomationModels(task.WorkspaceRoot)
	var route modelbilling.BillingRouteSelection
	if source != "" {
		route, err = exactAutomationRouteWithPickerCompatibility(candidates, model, source)
	} else {
		route, err = uniqueLegacyAutomationRoute(candidates, model)
	}
	if err != nil {
		return modelbilling.BillingRouteSelection{}, fmt.Errorf("%w: %v", errAutomationModelUnavailable, err)
	}
	return route, nil
}

func (a *App) automationTaskRouteInput(task automation.Task) (string, modelbilling.BillingSource, error) {
	model := strings.TrimSpace(task.Prompt.Model)
	source := modelbilling.BillingSource(strings.TrimSpace(task.Prompt.BillingSource))
	workspaceRoot := strings.TrimSpace(task.WorkspaceRoot)

	if model != "" {
		return model, source, nil
	}
	actor := a.activeConfigActorForWorkspace(workspaceRoot)
	cfg, err := config.LoadForRootMetadataWithActor(workspaceRoot, actor)
	if err != nil {
		return "", "", err
	}
	model = strings.TrimSpace(cfg.DefaultModel)
	source = desktopSettingsDefaultBillingSource(cfg)
	if model == "" {
		return "", "", fmt.Errorf("%w: automation: model route is required", errAutomationModelUnavailable)
	}
	return model, source, nil
}

func exactAutomationRoute(candidates []ModelInfo, model string, source modelbilling.BillingSource) (modelbilling.BillingRouteSelection, error) {
	if err := source.Validate(); err != nil {
		return modelbilling.BillingRouteSelection{}, err
	}
	for _, candidate := range candidates {
		if candidate.Ref == model && candidate.BillingSource == string(source) {
			return modelbilling.BillingRouteSelection{ModelRef: model, BillingSource: source}, nil
		}
	}
	return modelbilling.BillingRouteSelection{}, fmt.Errorf("automation: model %q is unavailable from %s", model, source)
}

func exactAutomationRouteWithPickerCompatibility(candidates []ModelInfo, model string, source modelbilling.BillingSource) (modelbilling.BillingRouteSelection, error) {
	route, originalErr := exactAutomationRoute(candidates, model, source)
	if originalErr == nil {
		return route, nil
	}
	repaired, ok := routePickerCompatibilityModel(model)
	if !ok {
		return modelbilling.BillingRouteSelection{}, originalErr
	}
	route, err := exactAutomationRoute(candidates, repaired, source)
	if err != nil {
		return modelbilling.BillingRouteSelection{}, originalErr
	}
	return route, nil
}

func routePickerCompatibilityModel(model string) (string, bool) {
	if !strings.HasPrefix(model, ":") || strings.HasPrefix(model, "::") {
		return "", false
	}
	repaired := strings.TrimPrefix(model, ":")
	if !strings.Contains(repaired, "/") {
		return "", false
	}
	return repaired, true
}

func uniqueLegacyAutomationRoute(candidates []ModelInfo, model string) (modelbilling.BillingRouteSelection, error) {
	route, found, err := uniqueAutomationRouteCandidate(candidates, model)
	if err != nil {
		return modelbilling.BillingRouteSelection{}, err
	}
	if found {
		return route, nil
	}
	repaired, ok := routePickerCompatibilityModel(model)
	if !ok {
		return modelbilling.BillingRouteSelection{}, fmt.Errorf("automation: legacy model %q is unavailable; select it again", model)
	}
	route, found, err = uniqueAutomationRouteCandidate(candidates, repaired)
	if err != nil {
		return modelbilling.BillingRouteSelection{}, fmt.Errorf("automation: legacy model %q has multiple billing routes; select it again", model)
	}
	if !found {
		return modelbilling.BillingRouteSelection{}, fmt.Errorf("automation: legacy model %q is unavailable; select it again", model)
	}
	return route, nil
}

func uniqueAutomationRouteCandidate(candidates []ModelInfo, model string) (modelbilling.BillingRouteSelection, bool, error) {
	var match *ModelInfo
	for _, candidate := range candidates {
		if candidate.Ref != model {
			continue
		}
		if match != nil {
			return modelbilling.BillingRouteSelection{}, false, fmt.Errorf("automation: legacy model %q has multiple billing routes; select it again", model)
		}
		copy := candidate
		match = &copy
	}
	if match == nil {
		return modelbilling.BillingRouteSelection{}, false, nil
	}
	return modelbilling.BillingRouteSelection{
		ModelRef: match.Ref, BillingSource: modelbilling.BillingSource(match.BillingSource),
	}, true, nil
}
