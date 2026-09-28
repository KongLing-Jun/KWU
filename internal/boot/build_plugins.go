package boot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/lsp"
	"kwu/internal/plugin"
	"kwu/internal/sandbox"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
)

func (b *buildAssembly) loadTools() error {
	b.initializeRegistry()
	b.configurePlugins()
	b.configureLSP()
	return nil
}

func (b *buildAssembly) initializeRegistry() {
	b.registry = tool.NewRegistry()
	b.bashSpec = sandbox.Spec{Mode: b.cfg.BashMode(), WriteRoots: b.cfg.WriteRootsForRoot(b.root), Network: b.cfg.Sandbox.Network}
	b.shell = sandbox.ResolveShell(b.cfg.Tools.Shell.Prefer, b.cfg.Tools.Shell.Path, b.stderr)
	b.bashSpec.Shell = b.shell
	b.emitShellWarnings()
	b.searchSpec = builtin.ResolveSearch(b.cfg.Tools.Search.Engine, b.cfg.Tools.Search.RgPath, b.stderr)
	b.bashTimeout = time.Duration(b.cfg.BashTimeoutSeconds()) * time.Second
	enabled := b.cfg.Tools.Enabled
	if b.tokenEconomy {
		enabled = tokenEconomyBuiltins(enabled)
	}
	addBuiltins(b.registry, enabled, b.cfg.WriteRootsForRoot(b.root), b.bashSpec, b.bashTimeout, b.searchSpec, b.stderr, b.root, b.proxySpec)
}

func (b *buildAssembly) emitShellWarnings() {
	if b.bashSpec.Mode == "enforce" && !sandbox.Available() {
		fmt.Fprintln(b.stderr, "warning: bash sandbox requested but unavailable on this platform; running bash unconfined")
	}
	if autoShellPrefer(b.cfg.Tools.Shell.Prefer) && b.shell.Kind == sandbox.ShellPowerShell {
		fmt.Fprintln(b.stderr, "warning: bash not found on PATH; the shell tool will run commands under Windows PowerShell. Install Git for Windows or WSL to use bash, or set [tools.shell] prefer=\"powershell\" to silence this.")
	}
}

func (b *buildAssembly) configurePlugins() {
	b.pluginHost = b.opts.SharedHost
	if b.pluginHost == nil {
		b.pluginHost = plugin.NewHost()
	}
	b.projectTrusted = projectTrustedForActor(b.root, b.actor, b.credentialPaths)
	autoStart := excludeManagedMCPEntries(projectMCPAutoStartEntries(b.cfg.AutoStartPlugins(), b.projectTrusted, b.sink), b.opts.ManagedMCPConfigNames)
	eagerEntries, backgroundEntries := partitionByTier(autoStart)
	extraSpecs := applyPlanModeAllowedMCPToolTrust(applyKnownPluginOverrides(b.opts.ExtraPlugins, b.root), b.cfg.Agent.PlanModeAllowedTools)
	if b.tokenEconomy {
		b.prepareOnDemandPlugins(autoStart, extraSpecs)
		eagerEntries, backgroundEntries = nil, nil
	}
	b.planModeTrustedMCPServers = planModeTrustedMCPServers(b.onDemandMCPSpecs)
	eagerEntries, backgroundEntries, demotions := demoteEagerPlugins(eagerEntries, backgroundEntries)
	eagerSpecs := PluginSpecsForRootWithPlanModeAllowedTools(eagerEntries, b.root, b.cfg.Agent.PlanModeAllowedTools)
	backgroundSpecs := PluginSpecsForRootWithPlanModeAllowedTools(backgroundEntries, b.root, b.cfg.Agent.PlanModeAllowedTools)
	if !b.tokenEconomy {
		eagerSpecs = append(eagerSpecs, extraSpecs...)
	}
	b.applyPluginStderr(eagerSpecs, backgroundSpecs)
	b.startEagerPlugins(eagerSpecs)
	b.registerBackgroundPlugins(backgroundSpecs)
	for _, message := range demotions {
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: message})
	}
	b.cleanup = b.pluginHost.Close
	if b.opts.SharedHost != nil {
		b.cleanup = func() {}
	}
}

func (b *buildAssembly) prepareOnDemandPlugins(autoStart []config.PluginEntry, extra []plugin.Spec) {
	b.onDemandMCPSpecs = map[string]plugin.Spec{}
	all := append(PluginSpecsForRootWithPlanModeAllowedTools(autoStart, b.root, b.cfg.Agent.PlanModeAllowedTools), extra...)
	for _, spec := range all {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		if _, exists := b.onDemandMCPSpecs[name]; !exists {
			b.onDemandMCPNames = append(b.onDemandMCPNames, name)
		}
		b.onDemandMCPSpecs[name] = spec
	}
}

func demoteEagerPlugins(eager, background []config.PluginEntry) ([]config.PluginEntry, []config.PluginEntry, []string) {
	var messages []string
	kept := eager[:0]
	budget := plugin.DefaultStartupBudget()
	for _, entry := range eager {
		recommendation := plugin.Recommend(entry.Name, budget, 0)
		if recommendation.Demote {
			messages = append(messages, recommendation.Reason)
			background = append(background, entry)
			continue
		}
		kept = append(kept, entry)
	}
	return kept, background, messages
}

func (b *buildAssembly) applyPluginStderr(groups ...[]plugin.Spec) {
	if b.opts.Stderr == nil {
		return
	}
	for _, specs := range groups {
		for i := range specs {
			specs[i].Stderr = b.opts.Stderr
		}
	}
}

func (b *buildAssembly) startEagerPlugins(specs []plugin.Spec) {
	if len(specs) == 0 {
		return
	}
	if b.opts.SharedHost != nil {
		b.startSharedEagerPlugins(specs)
		return
	}
	host, tools := plugin.StartAvailable(b.ctx, specs)
	b.pluginHost = host
	for _, candidate := range tools {
		b.registry.Add(candidate)
	}
	go host.StartPhaseB(b.ctx, b.sink)
}

func (b *buildAssembly) startSharedEagerPlugins(specs []plugin.Spec) {
	for _, spec := range specs {
		b.startSharedEagerPlugin(spec)
	}
}

func (b *buildAssembly) startSharedEagerPlugin(spec plugin.Spec) {
	if b.pluginHost.HasClient(spec.Name) {
		if tools, err := b.pluginHost.ToolsFor(b.ctx, spec.Name); err == nil {
			for _, candidate := range tools {
				b.registry.Add(candidate)
			}
			return
		}
	}
	addCtx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	tools, err := b.pluginHost.AddWithLifecycle(b.ctx, addCtx, spec)
	cancel()
	if err == nil {
		for _, candidate := range tools {
			b.registry.Add(candidate)
		}
		return
	}
	if plugin.IsServerAlreadyConnected(err) || errors.Is(err, plugin.ErrSpawningInFlight) {
		if tools, fetchErr := b.pluginHost.ToolsFor(b.ctx, spec.Name); fetchErr == nil {
			for _, candidate := range tools {
				b.registry.Add(candidate)
			}
			return
		}
	}
	recordUnavailableMCP(b.stderr, b.pluginHost, spec, err)
}

func (b *buildAssembly) registerBackgroundPlugins(specs []plugin.Spec) {
	for _, spec := range specs {
		if b.registerConnectedPlugin(spec) {
			continue
		}
		cached, _ := plugin.LoadCachedSchema(spec.Name, plugin.SpecFingerprint(spec))
		for _, candidate := range plugin.LazyToolset(spec, cached, b.pluginHost, b.registry, b.ctx, true) {
			b.registry.Add(candidate)
		}
	}
}

func (b *buildAssembly) registerConnectedPlugin(spec plugin.Spec) bool {
	if !b.pluginHost.HasClient(spec.Name) {
		return false
	}
	tools, err := b.pluginHost.ToolsFor(b.ctx, spec.Name)
	if err != nil {
		return false
	}
	for _, candidate := range tools {
		b.registry.Add(candidate)
	}
	return true
}

func (b *buildAssembly) configureLSP() {
	if !b.cfg.LSP.Enabled {
		return
	}
	b.lspManager = lsp.NewManager(b.root, LSPSpecs(b.cfg.LSP))
	if !b.tokenEconomy {
		b.addLSPTools()
	}
	previous := b.cleanup
	b.cleanup = func() {
		previous()
		b.lspManager.Close()
	}
}

func (b *buildAssembly) addLSPTools() []string {
	if b.lspManager == nil || b.lspToolsAdded {
		return nil
	}
	b.lspToolsAdded = true
	return addTools(b.registry, lsp.Tools(b.lspManager))
}
