package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	managedconnector "kwu/desktop/internal/connector"
	"kwu/desktop/internal/diagnostics"
	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/control"
	"kwu/internal/plugin"
	"kwu/internal/userctx"
)

// applyConnectorCapabilities restores every connected schema 2 capability into
// a rebuilt Agent controller. The package manifest, not a connector ID table,
// is the only source of MCP specs and Skills roots.
func (a *App) applyConnectorCapabilities(ctx context.Context, options *boot.Options, actor userctx.ActorContext) error {
	if options == nil || a.connectorService == nil {
		return nil
	}
	catalog, err := a.connectorService.Catalog()
	if err != nil {
		return nil
	}
	for _, item := range catalog.Items {
		options.ManagedMCPConfigNames = append(options.ManagedMCPConfigNames, item.ID)
	}
	scope, err := a.connectorOwnerScopeForActorWithRetry(ctx, actor)
	if err != nil {
		return nil
	}
	for _, item := range catalog.Items {
		snapshot, err := a.connectorService.Snapshot(item.ID, scope)
		if err != nil {
			return err
		}
		if snapshot.Status != managedconnector.StatusConnected {
			continue
		}
		// A startup/controller rebuild is one of the frozen upgrade checkpoints.
		// The installer preserves a previously connected owner only after the new
		// immutable package has been downloaded, validated, and atomically made active.
		snapshot, err = a.connectorService.Install(ctx, item.ID, scope, func(ctx context.Context, _ managedconnector.OwnerScope) error {
			return a.confirmConnectorOwnerScope(ctx, actor, scope)
		})
		if err != nil {
			return err
		}
		if snapshot.Status != managedconnector.StatusConnected {
			continue
		}
		_, packageRoot, manifest, err := a.connectorService.Manifest(item.ID)
		if err != nil {
			return err
		}
		plan, err := managedconnector.BuildExecutionPlan(manifest, packageRoot, desktopConnectorRuntimePolicy().OS)
		if err != nil {
			return err
		}
		specs := make([]plugin.Spec, 0, len(plan.MCP))
		connectorUnavailable := false
		for _, execution := range plan.MCP {
			options.ManagedMCPConfigNames = append(options.ManagedMCPConfigNames, execution.RuntimeName)
			spec, err := a.connectorMCPSpec(ctx, actor, scope, manifest, execution)
			if err != nil {
				if _, ok := connectorOAuthFailureDetails(err); ok {
					a.recordConnectorOAuthRuntimeFailure(ctx, actor, scope, item.ID, execution.ServerID, err)
					connectorUnavailable = true
					break
				}
				return err
			}
			specs = append(specs, spec)
		}
		if connectorUnavailable {
			continue
		}
		options.ExtraPlugins = append(options.ExtraPlugins, specs...)
		if plan.Skills != nil && connectorMCPsConnectedForSkills(ctx, options.SharedHost, specs) {
			options.ManagedSkillPaths = append(options.ManagedSkillPaths, plan.Skills.Root)
		}
	}
	return nil
}

func (a *App) connectorOwnerScopeForActorWithRetry(ctx context.Context, actor userctx.ActorContext) (managedconnector.OwnerScope, error) {
	delays := a.connectorOAuthRetryDelays
	if delays == nil {
		delays = defaultConnectorOAuthCredentialRetryDelays
	}
	maxAttempts := len(delays) + 1
	for attempt := 1; ; attempt++ {
		scope, err := a.connectorOwnerScopeForActor(ctx, actor)
		if err == nil {
			return scope, nil
		}
		failure := &connectorOAuthFailure{Stage: connectorOAuthStageBinding, Class: connectorOAuthClassBindingRead, Retryable: true, cause: err}
		var cloudErr *cloud.Error
		if errors.As(err, &cloudErr) && cloudErr.Code == cloud.ErrorProfileScopeMismatch {
			failure.Class = connectorOAuthClassOwnerChanged
			failure.Retryable = false
		}
		if !failure.Retryable || attempt >= maxAttempts || ctx.Err() != nil {
			logConnectorOAuthFailure("all", "all", failure, attempt, maxAttempts, false)
			return managedconnector.OwnerScope{}, err
		}
		logConnectorOAuthFailure("all", "all", failure, attempt, maxAttempts, true)
		if err := waitConnectorOAuthRetry(ctx, delays[attempt-1]); err != nil {
			return managedconnector.OwnerScope{}, err
		}
	}
}

func connectorMCPsConnectedForSkills(ctx context.Context, host *plugin.Host, specs []plugin.Spec) bool {
	if host == nil || len(specs) == 0 {
		return false
	}
	for _, spec := range specs {
		if host.HasClient(spec.Name) {
			tools, err := host.ToolsFor(ctx, spec.Name)
			if err == nil && len(tools) > 0 {
				continue
			}
		}
		startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		tools, err := host.AddWithLifecycle(ctx, startupCtx, spec)
		cancel()
		if err == nil && len(tools) > 0 {
			continue
		}
		tools, err = host.ToolsFor(ctx, spec.Name)
		if err != nil || len(tools) == 0 {
			return false
		}
	}
	return true
}

type connectorTurnExpansion struct {
	MCPNames   []string
	CommandEnv map[string]string
	Input      string
}

func (a *App) expandConnectorSelections(tabID string, ctrl control.SessionAPI, selectedMCP, selectedSkills []string, input string) (connectorTurnExpansion, error) {
	expansion := connectorTurnExpansion{MCPNames: []string{}, CommandEnv: map[string]string{}, Input: input}
	selectedMCP, selectedSkills = uniqueStrings(selectedMCP), uniqueStrings(selectedSkills)
	// No selected capability needs connector discovery or owner bindings.
	if len(selectedMCP) == 0 && len(selectedSkills) == 0 {
		expansion.CommandEnv = nil
		return expansion, nil
	}
	if a.connectorService == nil {
		expansion.MCPNames = uniqueStrings(selectedMCP)
		return expansion, nil
	}
	catalog, err := connectorCatalogForTurn(a.bootContext(), a.connectorService)
	if err != nil {
		return connectorTurnExpansion{}, err
	}
	selectedIDs := map[string]bool{}
	ordinary := []string{}
	known := map[string]bool{}
	for _, item := range catalog.Items {
		known[item.ID] = true
	}
	for _, name := range uniqueStrings(selectedMCP) {
		if known[name] {
			selectedIDs[name] = true
		} else {
			ordinary = append(ordinary, name)
		}
	}
	ctx := a.bootContext()
	scope, actor, err := a.connectorOwnerScope(ctx, tabID)
	if err != nil {
		return connectorTurnExpansion{}, err
	}
	guidance := []string{}
	for _, item := range catalog.Items {
		snapshot, err := a.connectorService.Snapshot(item.ID, scope)
		if err != nil {
			return connectorTurnExpansion{}, err
		}
		if snapshot.Status != managedconnector.StatusConnected {
			if selectedIDs[item.ID] {
				return connectorTurnExpansion{}, fmt.Errorf("连接器“%s”尚未连接，请先在连接器页面完成连接", item.Name)
			}
			continue
		}
		_, packageRoot, manifest, err := a.connectorService.Manifest(item.ID)
		if err != nil {
			return connectorTurnExpansion{}, err
		}
		plan, err := managedconnector.BuildExecutionPlan(manifest, packageRoot, desktopConnectorRuntimePolicy().OS)
		if err != nil {
			return connectorTurnExpansion{}, err
		}
		selected := selectedIDs[item.ID] || connectorOwnsSelectedSkill(plan, selectedSkills)
		if !selected {
			continue
		}
		if err := a.ensureConnectorMCPsForTab(ctx, actor, scope, ctrl, manifest, plan); err != nil {
			return connectorTurnExpansion{}, err
		}
		for _, execution := range plan.MCP {
			expansion.MCPNames = append(expansion.MCPNames, execution.RuntimeName)
		}
		guidance = append(guidance, connectorTurnGuidance(item))
	}
	expansion.MCPNames = uniqueStrings(append(ordinary, expansion.MCPNames...))
	if len(expansion.CommandEnv) == 0 {
		expansion.CommandEnv = nil
	}
	if len(guidance) > 0 {
		expansion.Input = strings.Join(guidance, "\n") + "\n\nUser request:\n" + input
	}
	return expansion, nil
}

func connectorCatalogForTurn(ctx context.Context, service desktopConnectorRuntime) (managedconnector.Catalog, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return managedconnector.Catalog{}, err
	}
	catalog, err := service.Catalog()
	if err == nil {
		return catalog, nil
	}
	if recovery, ok := service.(interface {
		RecoverCatalog(context.Context) (managedconnector.Catalog, error)
	}); ok {
		catalog, err = recovery.RecoverCatalog(ctx)
	} else {
		catalog, err = service.RefreshCatalog(ctx)
	}
	if err == nil {
		return catalog, nil
	}
	if errors.Is(err, context.Canceled) {
		return managedconnector.Catalog{}, err
	}
	code := "CONNECTOR_CATALOG_UNAVAILABLE"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "CONNECTOR_CATALOG_TIMEOUT"
	} else if errors.Is(err, managedconnector.ErrCatalogRefreshInProgress) {
		code = "CONNECTOR_CATALOG_REFRESHING"
	}
	slog.Warn("connector catalog blocked capability submission", diagnostics.FailureMarkerKey, diagnostics.FailureMarker,
		"scope", "session.connector_catalog", "code", code, "error_type", fmt.Sprintf("%T", err))
	return managedconnector.Catalog{}, fmt.Errorf("%s: 连接器目录暂不可用，请稍后重试。", code)
}

func (a *App) ensureConnectorMCPsForTab(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, ctrl control.SessionAPI, manifest managedconnector.Manifest, plan managedconnector.ExecutionPlan) error {
	connected := connectedMCPToolCounts(ctrl)
	for _, execution := range plan.MCP {
		if connected[execution.RuntimeName] > 0 {
			continue
		}
		spec, err := a.connectorMCPSpec(ctx, actor, scope, manifest, execution)
		if err != nil {
			return err
		}
		managedCtrl, ok := ctrl.(managedMCPRegistrar)
		if !ok {
			return errors.New("managed MCP registration is unavailable")
		}
		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		count, connectErr := registerManagedMCPSpec(connectCtx, managedCtrl, spec)
		cancel()
		if connectErr != nil || count <= 0 {
			return fmt.Errorf("connector MCP %q is unavailable", execution.ServerID)
		}
	}
	return nil
}

func connectorOwnsSelectedSkill(plan managedconnector.ExecutionPlan, selected []string) bool {
	if plan.Skills == nil {
		return false
	}
	root := filepath.Clean(plan.Skills.Root)
	for _, name := range selected {
		path := filepath.Join(root, strings.TrimSpace(name), "SKILL.md")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func connectorTurnGuidance(item managedconnector.CatalogItem) string {
	text := fmt.Sprintf("The current task has enabled connector %q by %s. Use its registered MCP tools and/or Skills when relevant.", item.Name, item.Provider)
	if len(item.Examples) > 0 {
		text += " Example: " + item.Examples[0]
	}
	return text
}
