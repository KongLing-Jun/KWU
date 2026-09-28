package boot

import (
	"fmt"
	"log/slog"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/agentmode"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/guardian"
	"kwu/internal/provider"
	"kwu/internal/tasktree"
)

func (b *buildAssembly) loadRunner() error {
	b.initializeExecutor()
	if err := b.configureAutoPlanClassifier(); err != nil {
		return err
	}
	if err := b.configurePlannerRunner(); err != nil {
		return err
	}
	if err := b.wrapAgentModeRunner(); err != nil {
		return err
	}
	if b.opts.RequireTaskContext && b.opts.MainStreamDispatcher != nil {
		if b.opts.ModelBillingTaskStore == nil {
			return fmt.Errorf("boot: model billing task store is required with main dispatcher")
		}
	}
	return nil
}

func (b *buildAssembly) initializeExecutor() {
	var derivedProvider provider.Provider
	if b.opts.MainStreamDispatcher != nil {
		derivedProvider = provider.FailClosedProvider{Base: b.execProvider, Message: "B2C compaction route is not connected"}
	}
	b.executor = agent.New(b.execProvider, b.registry, agent.NewSession(b.systemPrompt), agent.Options{
		MaxSteps:             b.maxSteps,
		Temperature:          b.cfg.Agent.Temperature,
		Pricing:              b.entry.Price,
		Gate:                 b.headlessGate,
		Jobs:                 b.jobs,
		ProjectChecks:        b.projectChecks,
		ContextWindow:        b.entry.ContextWindow,
		SoftCompactRatio:     b.cfg.Agent.SoftCompactRatio,
		CompactRatio:         b.cfg.Agent.CompactRatio,
		CompactForceRatio:    b.cfg.Agent.CompactForceRatio,
		RecentKeep:           b.cfg.Agent.RecentKeep,
		ArchiveDir:           b.archiveDir,
		KeepPolicy:           b.keepPolicy,
		ReasoningLanguage:    b.cfg.ReasoningLanguage(),
		PlanModeAllowedTools: b.cfg.Agent.PlanModeAllowedTools,
		StreamDispatcher:     b.opts.MainStreamDispatcher,
		DerivedProvider:      derivedProvider,
	}, b.sink)
	b.runner = b.executor
	b.label = b.entry.Model
	b.taskTreePlannerProvider = b.execProvider
	if b.opts.MainStreamDispatcher != nil {
		b.taskTreePlannerProvider = provider.FailClosedProvider{Base: b.execProvider, Message: "B2C task-tree planner route is not connected"}
	}
}

func (b *buildAssembly) configureAutoPlanClassifier() error {
	if b.tokenEconomy || strings.EqualFold(strings.TrimSpace(b.cfg.Agent.AutoPlan), "off") || b.cfg.Agent.AutoPlanClassifier == "" {
		return nil
	}
	model := b.cfg.Agent.AutoPlanClassifier
	entry, ok := b.cfg.ResolveModel(model)
	if !ok {
		return fmt.Errorf("auto_plan_classifier %q is not a configured provider", model)
	}
	classifierProvider, err := newProviderWithProxyForOwner(entry, b.proxySpec, b.credentialOwner)
	if err != nil {
		return fmt.Errorf("auto_plan_classifier %q: %w", model, err)
	}
	if b.opts.MainStreamDispatcher != nil {
		classifierProvider = provider.FailClosedProvider{Base: classifierProvider, Message: "B2C classifier route is not connected"}
	}
	b.classifier = control.NewBillableProviderAutoPlanClassifier(classifierProvider, entry.Price, b.sink)
	return nil
}

func (b *buildAssembly) configurePlannerRunner() error {
	model := b.cfg.Agent.PlannerModel
	if model == "" || b.tokenEconomy {
		return nil
	}
	entry, ok := b.cfg.ResolveModel(model)
	if !ok {
		return fmt.Errorf("planner_model %q is not a configured provider", model)
	}
	if entry.Model == b.entry.Model {
		return nil
	}
	plannerProvider, err := newProviderWithProxyForOwner(entry, b.proxySpec, b.credentialOwner)
	if err != nil {
		return fmt.Errorf("planner %q: %w", model, err)
	}
	if b.opts.MainStreamDispatcher != nil {
		plannerProvider = provider.FailClosedProvider{Base: plannerProvider, Message: "B2C planner route is not connected"}
	}
	b.taskTreePlannerProvider = plannerProvider
	plannerSession := agent.NewSession(agent.PlannerPromptWithContext(b.memory.Block()))
	plannerTools := agent.PlannerToolRegistry(b.registry)
	b.runner = agent.NewCoordinator(plannerProvider, plannerSession, entry.Price, plannerTools, agent.Options{
		MaxSteps:          b.cfg.Agent.PlannerMaxSteps,
		MaxStepsKey:       "agent.planner_max_steps",
		Gate:              b.headlessGate,
		ContextWindow:     entry.ContextWindow,
		SoftCompactRatio:  b.cfg.Agent.SoftCompactRatio,
		CompactRatio:      b.cfg.Agent.CompactRatio,
		CompactForceRatio: b.cfg.Agent.CompactForceRatio,
		RecentKeep:        b.cfg.Agent.RecentKeep,
		ArchiveDir:        config.ArchiveDir(),
		KeepPolicy:        b.keepPolicy,
		ReasoningLanguage: b.cfg.ReasoningLanguage(),
	}, b.executor, b.cfg.Agent.Temperature, b.sink, control.NewPlannerGate(b.classifier))
	b.label = b.entry.Model + " + planner " + entry.Model
	return nil
}

func (b *buildAssembly) wrapAgentModeRunner() error {
	var err error
	b.runner, err = agentmode.BuildRunner(b.agentMode.Mode, agentmode.RunnerDeps{
		Normal:   b.runner,
		Executor: b.executor,
		Registry: b.registry,
		Sink:     b.sink,
		TaskTreePlanner: tasktree.NewLLMPlanner(tasktree.LLMPlannerOptions{
			Provider:    b.taskTreePlannerProvider,
			Temperature: b.cfg.Agent.Temperature,
		}),
		TaskTreeStore: tasktree.NewMemoryStateStore(),
	})
	return err
}

func (b *buildAssembly) buildController() (*control.Controller, error) {
	if err := validateTaskContextOptions(b.opts); err != nil {
		return nil, err
	}
	if err := validateBillingRouteExecutorModel(b.opts, b.modelRef); err != nil {
		return nil, err
	}
	opts := b.controllerOptions()
	b.configureGuardian(&opts)
	if b.classifier != nil {
		opts.Classifier = b.classifier
	}
	return control.New(opts), nil
}

func (b *buildAssembly) controllerOptions() control.Options {
	return withTaskContextOptions(control.Options{
		Runner:                 b.runner,
		Executor:               b.executor,
		Sink:                   b.sink,
		Policy:                 b.policy,
		Label:                  b.label,
		ModelRef:               b.modelRef,
		AgentMode:              b.agentMode.Mode,
		AgentModeSource:        b.agentMode.Source,
		SystemPrompt:           b.systemPrompt,
		Experts:                b.expertCatalog,
		Expert:                 b.expertSelection,
		PromptBuilder:          b.buildSystemPrompt,
		ExpertCatalogLoader:    b.loadExpertCatalog,
		UserExpertPath:         b.userExpertPath,
		SessionDir:             b.sessionDir,
		Host:                   b.pluginHost,
		Commands:               b.commands,
		Skills:                 b.skills,
		AllSkills:              b.allSkills,
		SkillStore:             b.skillStore,
		AllSkillStore:          b.allSkillStore,
		Memory:                 b.memory,
		Cleanup:                b.cleanup,
		BalanceURL:             balanceURLForRoute(b.entry, usesSubscriptionBillingRoute(b.opts)),
		BalanceKey:             balanceKeyForRoute(b.entry, usesSubscriptionBillingRoute(b.opts)),
		BalanceClient:          b.balanceClient,
		Jobs:                   b.jobs,
		Registry:               b.registry,
		PluginCtx:              b.ctx,
		WorkspaceRoot:          b.root,
		CommandEnvProvider:     pythonCommandEnvProvider(b.cfg, b.root),
		Actor:                  b.actor,
		AutoPlan:               b.cfg.Agent.AutoPlan,
		ResponseLanguage:       b.cfg.ResponseLanguage(),
		ReasoningLanguage:      b.cfg.ReasoningLanguage(),
		DisableColdResumePrune: !b.cfg.ColdResumePruneEnabled(),
		Shell:                  b.shell,
		PlanModeAllowedTools:   b.cfg.Agent.PlanModeAllowedTools,
		ApprovalTimeout:        b.opts.ApprovalTimeout,
		Temporal:               b.temporal,
		OnRemember:             b.rememberPermissionRule,
	}, b.opts)
}

func (b *buildAssembly) rememberPermissionRule(rule string) control.RememberResult {
	return rememberPermissionRuleForActor(b.actor, rule)
}

func (b *buildAssembly) configureGuardian(opts *control.Options) {
	model := b.cfg.Agent.GuardianModel
	if model == "" {
		return
	}
	entry, ok := b.cfg.ResolveModel(model)
	if !ok {
		slog.Warn("guardian model is not a configured provider — guardian disabled", "model", model)
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: fmt.Sprintf("guardian_model %q not found — guardian disabled", model)})
		return
	}
	guardianProvider, err := newProviderWithProxyForOwner(entry, b.proxySpec, b.credentialOwner)
	if err != nil {
		slog.Warn("guardian provider construction failed — guardian disabled", "model", model, "err", err)
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: fmt.Sprintf("guardian construction failed: %v — guardian disabled", err)})
		return
	}
	if b.opts.MainStreamDispatcher != nil {
		guardianProvider = provider.FailClosedProvider{Base: guardianProvider, Message: "B2C guardian route is not connected"}
	}
	guardianRegistry := agent.FilterReadOnlyRegistry(b.registry, agent.SubagentMetaTools()...)
	opts.Guardian = guardian.NewSession(guardianProvider, guardianRegistry, guardian.PolicyPrompt(), model, b.cfg.Agent.GuardianTemperature, entry.Price, b.sink)
	b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: fmt.Sprintf("guardian enabled · model=%s", entry.Model)})
}
