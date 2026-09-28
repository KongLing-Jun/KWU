package boot

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/command"
	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/history"
	"kwu/internal/installsource"
	"kwu/internal/memory"
	"kwu/internal/permission"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/skill"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
	"kwu/internal/tool/sessiontool"
)

func (b *buildAssembly) loadAgentTools() error {
	if err := b.prepareAgentRuntime(); err != nil {
		return err
	}
	b.configureTaskTools()
	if err := b.addStandardTools(); err != nil {
		return err
	}
	b.configureSkillTools()
	if b.tokenEconomy {
		b.addToolSourceConnector()
	}
	return nil
}

func (b *buildAssembly) prepareAgentRuntime() error {
	b.maxSteps = b.cfg.Agent.MaxSteps
	if b.opts.MaxSteps > 0 {
		b.maxSteps = b.opts.MaxSteps
	}
	var err error
	b.subagentStore, err = newSubagentStore(b.sessionDir)
	if err != nil {
		return err
	}
	if b.subagentStore != nil {
		b.subagentStore.WithDestroyedChecker(b.jobs.IsDestroying)
	}
	b.policy = permission.New(b.cfg.Permissions.Mode, b.cfg.Permissions.Allow, b.cfg.Permissions.Ask, b.cfg.Permissions.Deny)
	b.headlessGate = permission.NewGate(b.policy, nil)
	b.taskModel = firstNonEmpty(b.cfg.Agent.SubagentModels["task"], b.cfg.Agent.SubagentModel)
	b.taskEffort = firstNonEmpty(b.cfg.Agent.SubagentEfforts["task"], b.cfg.Agent.SubagentEffort)
	b.commands, _ = command.Load(config.CommandDirsForRoot(b.root)...)
	return nil
}

func (b *buildAssembly) resolveSubagentProvider(modelRef, effort string) (provider.Provider, *provider.Pricing, int, error) {
	entry := *b.entry
	if strings.TrimSpace(modelRef) != "" {
		resolved, ok := resolveSubagentModelEntry(b.cfg, b.entry, modelRef)
		if !ok {
			return nil, nil, 0, fmt.Errorf("unknown model %q", modelRef)
		}
		entry = *resolved
	}
	if strings.TrimSpace(effort) != "" {
		normalized, err := config.NormalizeEffort(&entry, effort)
		if err != nil {
			return nil, nil, 0, config.FriendlyEffortError(&entry, effort, err)
		}
		entry.Effort = normalized
		if entry.Kind == "anthropic" && strings.TrimSpace(entry.Effort) != "" && strings.TrimSpace(entry.Thinking) == "" {
			entry.Thinking = "adaptive"
		}
	}
	resolvedProvider, err := newProviderWithProxyForOwner(&entry, b.proxySpec, b.credentialOwner)
	if err != nil {
		return nil, nil, 0, err
	}
	if b.opts.MainStreamDispatcher != nil {
		resolvedProvider = provider.FailClosedProvider{Base: resolvedProvider, Message: "B2C subagent route is not connected"}
	}
	return resolvedProvider, entry.Price, entry.ContextWindow, nil
}

func (b *buildAssembly) subagentIdentity(modelRef, effort string) (string, string) {
	return subagentEffectiveIdentity(b.cfg, b.modelName, b.entry, modelRef, effort)
}

func (b *buildAssembly) configureTaskTools() {
	if b.tokenEconomy {
		return
	}
	_, _ = b.enableTask(b.ctx)
	_, _ = b.enableReadOnlyTask(b.ctx)
}

func (b *buildAssembly) newTaskTool() *agent.TaskTool {
	return agent.NewTaskTool(b.execProvider, b.entry.Price, b.registry, b.maxSteps,
		b.entry.ContextWindow, b.cfg.Agent.RecentKeep, b.cfg.Agent.SoftCompactRatio, b.cfg.Agent.CompactRatio, b.cfg.Agent.CompactForceRatio,
		b.cfg.Agent.Temperature, b.archiveDir, "", b.headlessGate, b.keepPolicy,
		b.taskModel, b.taskEffort, b.resolveSubagentProvider).
		WithTranscripts(b.subagentStore, b.root, b.modelName, b.entry.Effort).
		WithSubagentEffortLevels(subagentEffortLevels(b.cfg, b.entry, b.taskModel)).
		WithActorContext(b.actor).
		WithTranscriptIdentityResolver(b.subagentIdentity)
}

func (b *buildAssembly) enableTask(context.Context) (string, error) {
	if b.taskToolAdded {
		return "task tool is already enabled.", nil
	}
	b.taskToolAdded = true
	if b.taskTool == nil {
		b.taskTool = b.newTaskTool()
	}
	b.registry.Add(b.taskTool)
	b.registry.Add(agent.NewWaitTaskTool(b.taskTool))
	b.registry.Add(agent.NewStopTaskTool(b.taskTool))
	b.registry.Add(agent.NewParallelTasksTool(b.taskTool, b.registry))
	return "enabled task.", nil
}

func (b *buildAssembly) enableReadOnlyTask(context.Context) (string, error) {
	if b.readOnlyTaskToolAdded {
		return "read_only_task tool is already enabled.", nil
	}
	b.readOnlyTaskToolAdded = true
	if b.taskTool == nil {
		b.taskTool = b.newTaskTool()
	}
	b.registry.Add(agent.NewReadOnlyTaskTool(b.taskTool))
	return "enabled read_only_task.", nil
}

func (b *buildAssembly) addStandardTools() error {
	addCurrentDatetimeAndMediaTool(
		b.registry, builtin.NewCurrentDatetime(b.temporal),
		b.opts.MediaToolCredential, b.opts.MediaImageQuotaPricing, b.opts.MediaVideoClient, b.opts.MediaUnderstandingClient,
		b.opts.MediaVideoScope, b.balanceClient,
		filepath.Join(b.root, ".clouvia", "generated-media"), b.root,
	)
	b.registry.Add(history.NewTool(history.Options{SessionDir: b.sessionDir, GlobalSessionDir: b.credentialPaths.SessionDir(), ArchiveDir: b.archiveDir}))
	b.registry.Add(sessiontool.NewListSessionsTool(b.sessionDir))
	b.registry.Add(sessiontool.NewReadSessionTool(b.sessionDir))
	b.registry.Add(memory.NewRecallTool(b.memory.Store))
	b.registry.Add(memory.NewRememberTool(b.memory.Store))
	b.registry.Add(memory.NewForgetTool(b.memory.Store))
	b.registry.Add(agent.NewAskTool())
	return nil
}

func (b *buildAssembly) resolveSkillProvider(sk skill.Skill) (provider.Provider, *provider.Pricing, int, string, string, error) {
	modelRef := subagentModelRef(b.cfg, sk)
	effortRef := subagentEffortRef(b.cfg, sk)
	if modelRef == "" && effortRef == "" {
		return b.execProvider, b.entry.Price, b.entry.ContextWindow, modelRef, effortRef, nil
	}
	resolved, price, contextWindow, err := b.resolveSubagentProvider(modelRef, effortRef)
	return resolved, price, contextWindow, modelRef, effortRef, err
}

func (b *buildAssembly) readOnlySkillRunner(ctx context.Context, sk skill.Skill, task string, opts skill.SubagentRunOptions) (string, error) {
	if strings.TrimSpace(opts.ContinueFrom) != "" || strings.TrimSpace(opts.ForkFrom) != "" {
		return "", fmt.Errorf("read_only_skill does not support continue_from/fork_from")
	}
	parentRegistry := agent.TaskToolRegistry(ctx, b.registry)
	sk = skill.WithCodeGraphTools(sk, skill.CodeGraphReadTools(parentRegistry))
	prov, price, contextWindow, _, _, err := b.resolveSkillProvider(sk)
	if err != nil {
		return "", fmt.Errorf("read-only subagent skill %q profile: %w", sk.Name, err)
	}
	subRegistry := agent.ReadOnlySubagentToolRegistry(parentRegistry, sk.AllowedTools)
	if subRegistry.Len() == 0 {
		return "", fmt.Errorf("read_only_skill: skill %q has no read-only tools available", sk.Name)
	}
	return agent.RunSubAgentWithSession(ctx, prov, subRegistry, agent.NewSession(agent.DefaultReadOnlyTaskSystemPrompt+"\n\nSkill instructions:\n"+sk.Body), task, agent.Options{
		MaxSteps:          subagentStepLimit(b.maxSteps),
		Temperature:       b.cfg.Agent.Temperature,
		Pricing:           price,
		UsageSource:       event.UsageSourceSubagent,
		Gate:              b.headlessGate,
		ContextWindow:     contextWindow,
		RecentKeep:        b.cfg.Agent.RecentKeep,
		SoftCompactRatio:  b.cfg.Agent.SoftCompactRatio,
		CompactRatio:      b.cfg.Agent.CompactRatio,
		CompactForceRatio: b.cfg.Agent.CompactForceRatio,
		ArchiveDir:        config.ArchiveDir(),
		KeepPolicy:        b.keepPolicy,
		ReasoningLanguage: agent.ReasoningLanguageFromContext(ctx),
	}, agent.NestedSink(ctx, event.Discard))
}

func subagentStepLimit(maxSteps int) int {
	if maxSteps <= 0 {
		return maxSteps
	}
	maxSteps /= 2
	if maxSteps < 5 {
		return 5
	}
	return maxSteps
}

func (b *buildAssembly) skillRunner(ctx context.Context, sk skill.Skill, task string, opts skill.SubagentRunOptions) (string, error) {
	parentRegistry := agent.TaskToolRegistry(ctx, b.registry)
	sk = skill.WithCodeGraphTools(sk, skill.CodeGraphReadTools(parentRegistry))
	prov, price, contextWindow, modelRef, effortRef, err := b.resolveSkillProvider(sk)
	if err != nil {
		return "", fmt.Errorf("subagent skill %q profile: %w", sk.Name, err)
	}
	subRegistry := agent.SubagentToolRegistry(parentRegistry, sk.AllowedTools)
	run, err := b.prepareSkillRun(ctx, sk, subRegistry, modelRef, effortRef, opts)
	if err != nil {
		return "", err
	}
	defer run.Release()
	answer, err := agent.RunSubAgentWithSession(ctx, prov, subRegistry, run.Session, task, agent.Options{
		MaxSteps:          subagentStepLimit(b.maxSteps),
		Temperature:       b.cfg.Agent.Temperature,
		Pricing:           price,
		UsageSource:       event.UsageSourceSubagent,
		Gate:              b.headlessGate,
		ContextWindow:     contextWindow,
		RecentKeep:        b.cfg.Agent.RecentKeep,
		ArchiveDir:        config.ArchiveDir(),
		KeepPolicy:        b.keepPolicy,
		ReasoningLanguage: agent.ReasoningLanguageFromContext(ctx),
	}, agent.NestedSink(ctx, event.Discard))
	if err != nil {
		return "", errors.Join(err, b.subagentStore.SaveFailed(run))
	}
	if err := b.subagentStore.SaveCompleted(run); err != nil {
		return "", errors.Join(err, b.subagentStore.SaveFailed(run))
	}
	return agent.FormatSubagentRunResult(answer, run, false), nil
}

func (b *buildAssembly) prepareSkillRun(ctx context.Context, sk skill.Skill, registry *tool.Registry, modelRef, effortRef string, opts skill.SubagentRunOptions) (*agent.SubagentRun, error) {
	continueFrom := strings.TrimSpace(opts.ContinueFrom)
	legacyForkFrom := strings.TrimSpace(opts.ForkFrom)
	if continueFrom != "" && legacyForkFrom != "" {
		return nil, fmt.Errorf("continue_from and fork_from are mutually exclusive; pass only continue_from")
	}
	parentID, _, _, _ := agent.CallContext(ctx)
	parentSession := agent.ParentSession(ctx)
	if b.subagentStore == nil || parentSession == "" {
		if continueFrom != "" || legacyForkFrom != "" {
			return nil, fmt.Errorf("subagent continuation requires a persisted session; none is active in this run")
		}
		return agent.EphemeralSubagentRun(sk.Body), nil
	}
	identityModel, identityEffort := b.subagentIdentity(modelRef, effortRef)
	spec := agent.SubagentSpec{Kind: "skill", Name: sk.Name, WorkspaceRoot: b.root, ParentSession: parentSession, ParentToolCallID: parentID, SystemPrompt: sk.Body, Registry: registry, Model: identityModel, Effort: identityEffort, Actor: b.actor}
	if continueFrom != "" {
		return b.subagentStore.PrepareContinue(continueFrom, spec)
	}
	if legacyForkFrom != "" {
		return b.subagentStore.PrepareLegacyForkFrom(legacyForkFrom, spec)
	}
	return b.subagentStore.PrepareFresh(spec)
}

func (b *buildAssembly) skillProfile(sk skill.Skill) *event.Profile {
	model, effort := subagentModelRef(b.cfg, sk), subagentEffortRef(b.cfg, sk)
	if model == "" && effort == "" {
		return nil
	}
	return &event.Profile{Model: model, Effort: effort}
}

func (b *buildAssembly) configureSkillTools() {
	if b.tokenEconomy {
		b.addSlashCommandTool(false)
		return
	}
	_, _ = b.enableInstallSource(b.ctx)
	_, _ = b.enableSkills(b.ctx)
}

func (b *buildAssembly) addSlashCommandTool(includeSkills bool) {
	var entries []command.SlashEntry
	if includeSkills {
		for _, candidate := range b.skills {
			sk := candidate
			entries = append(entries, command.SlashEntry{Name: sk.Name, Description: sk.Description, Render: func(args []string) string { return skill.Render(sk, strings.Join(args, " ")) }})
		}
	}
	for _, candidate := range b.commands {
		cmd := candidate
		entries = append(entries, command.SlashEntry{Name: cmd.Name, Description: cmd.Description, ArgHint: cmd.ArgHint, Render: func(args []string) string { return cmd.Render(args) }})
	}
	b.registry.Add(command.NewSlashCommandTool(entries))
}

func (b *buildAssembly) enableInstallSource(context.Context) (string, error) {
	if b.installSourceAdded {
		return "install_source is already enabled.", nil
	}
	b.installSourceAdded = true
	b.registry.Add(installsource.NewTool(installsource.Options{ProjectRoot: b.root, HTTPClient: b.balanceClient, ConnectMCP: b.connectInstalledMCP, OnDisconnect: b.disconnectInstalledMCP}))
	return "enabled install_source.", nil
}

func (b *buildAssembly) connectInstalledMCP(entry config.PluginEntry) (installsource.MCPConnectResult, error) {
	spec := pluginSpecFromEntry(entry, b.root)
	if b.opts.Stderr != nil {
		spec.Stderr = b.opts.Stderr
	}
	tools, err := b.pluginHost.Add(b.ctx, spec)
	if err != nil {
		return installsource.MCPConnectResult{}, err
	}
	b.registry.RemovePrefix(plugin.ToolPrefix(spec.Name))
	for _, candidate := range tools {
		b.registry.Add(candidate)
	}
	return installsource.MCPConnectResult{ToolCount: len(tools), Disconnect: func() { b.disconnectInstalledMCP(spec.Name) }}, nil
}

func (b *buildAssembly) disconnectInstalledMCP(serverName string) bool {
	prefix, ok := b.pluginHost.Remove(serverName)
	if !ok {
		return false
	}
	b.registry.RemovePrefix(prefix)
	return true
}

func (b *buildAssembly) enableReadOnlySkills(context.Context) (string, error) {
	if b.readOnlySkillToolsAdded {
		return "read_only_skill tool is already enabled.\n\n" + skill.ReadOnlyIndexBlock(b.skills), nil
	}
	b.readOnlySkillToolsAdded = true
	b.registry.Add(skill.NewReadOnlySkillTool(b.skillStore, b.readOnlySkillRunner, b.skillProfile))
	return "enabled read_only_skill. Use read_only_skill for inline skills or read-only subagent skills on the next model request.\n\n" + skill.ReadOnlyIndexBlock(b.skills), nil
}

func (b *buildAssembly) enableSkills(ctx context.Context) (string, error) {
	if b.skillToolsAdded {
		return "skills are already enabled.\n\n" + skill.IndexBlock(b.skills), nil
	}
	b.skillToolsAdded = true
	_, _ = b.enableReadOnlySkills(ctx)
	b.registry.Add(skill.NewRunSkillTool(b.skillStore, b.skillRunner, b.skillProfile))
	b.registry.Add(skill.NewReadSkillTool(b.skillStore))
	b.registry.Add(skill.NewInstallSkillTool(b.skillStore, nil))
	for _, candidate := range skill.BuiltinSubagentTools(b.skillStore, b.skillRunner, b.skillProfile) {
		b.registry.Add(candidate)
	}
	b.addSlashCommandTool(true)
	return "enabled skills. Use run_skill/read_skill/read_only_skill or the dedicated skill tools on the next model request.\n\n" + skill.IndexBlock(b.skills), nil
}

func (b *buildAssembly) addToolSourceConnector() {
	b.registry.Add(&toolSourceConnector{
		skills:                   b.enableSkills,
		readOnlySkill:            b.enableReadOnlySkills,
		task:                     b.enableTask,
		readOnlyTask:             b.enableReadOnlyTask,
		install:                  b.enableInstallSource,
		webFetch:                 b.enableWebFetch,
		lsp:                      b.enableLSPSource,
		mcp:                      b.enableMCPSource,
		mcpNames:                 b.onDemandMCPNames,
		planModeAllowedTools:     b.cfg.Agent.PlanModeAllowedTools,
		planModeTrustedMCPServer: b.planModeTrustedMCPServers,
	})
}

func (b *buildAssembly) enableWebFetch(context.Context) (string, error) {
	if !builtinToolEnabled(b.cfg.Tools.Enabled, "web_fetch") {
		return "web_fetch is disabled by [tools].enabled.", nil
	}
	names := addTools(b.registry, builtin.Workspace{Dir: b.root, WriteRoots: b.cfg.WriteRootsForRoot(b.root), Bash: b.bashSpec, BashTimeout: b.bashTimeout, Search: b.searchSpec, ProxySpec: b.proxySpec}.Tools("web_fetch"))
	if len(names) == 0 {
		return "web_fetch is already enabled or unavailable.", nil
	}
	return "enabled " + strings.Join(names, ", ") + ".", nil
}

func (b *buildAssembly) enableLSPSource(context.Context) (string, error) {
	if b.lspManager == nil {
		return "", fmt.Errorf("LSP is disabled in config")
	}
	names := b.addLSPTools()
	if len(names) == 0 {
		return "LSP tools are already enabled.", nil
	}
	return "enabled " + strings.Join(names, ", ") + ".", nil
}

func (b *buildAssembly) enableMCPSource(_ context.Context, name string) (string, error) {
	spec, ok := b.onDemandMCPSpecs[name]
	if !ok {
		return "", fmt.Errorf("no configured MCP server named %q", name)
	}
	if b.opts.Stderr != nil {
		spec.Stderr = b.opts.Stderr
	}
	tools, err := b.pluginHost.Add(b.ctx, spec)
	if err != nil {
		return b.reuseConnectedMCP(spec, err)
	}
	return b.registerMCPTools(spec, tools), nil
}

func (b *buildAssembly) reuseConnectedMCP(spec plugin.Spec, addErr error) (string, error) {
	if !errors.Is(addErr, plugin.ErrServerAlreadyConnected) && !errors.Is(addErr, plugin.ErrSpawningInFlight) {
		return "", addErr
	}
	tools, err := b.pluginHost.ToolsFor(b.ctx, spec.Name)
	if err != nil {
		return "", err
	}
	return b.registerMCPTools(spec, tools), nil
}

func (b *buildAssembly) registerMCPTools(spec plugin.Spec, tools []tool.Tool) string {
	b.registry.RemovePrefix(plugin.ToolPrefix(spec.Name))
	names := addTools(b.registry, tools)
	if len(names) == 0 {
		return fmt.Sprintf("MCP server %q connected but exposed no tools.", spec.Name)
	}
	return fmt.Sprintf("enabled MCP server %q tools: %s.", spec.Name, strings.Join(names, ", "))
}
