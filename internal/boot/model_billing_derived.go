package boot

import (
	"fmt"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type bootSubagentResolver struct {
	cfg          *config.Config
	defaultEntry *config.ProviderEntry
	billing      bootModelBilling
	subscription bool
	proxy        netclient.ProxySpec
	owner        string
}

func (r bootSubagentResolver) entry(modelRef, effort string) (*config.ProviderEntry, error) {
	entry := *r.defaultEntry
	if strings.TrimSpace(modelRef) != "" {
		resolved, ok := resolveSubagentModelEntry(r.cfg, r.defaultEntry, modelRef)
		if !ok {
			return nil, fmt.Errorf("unknown model %q", modelRef)
		}
		entry = *resolved
	}
	if strings.TrimSpace(effort) == "" {
		return &entry, nil
	}
	normalized, err := config.NormalizeEffort(&entry, effort)
	if err != nil {
		return nil, config.FriendlyEffortError(&entry, effort, err)
	}
	entry.Effort = normalized
	if entry.Kind == "anthropic" && strings.TrimSpace(entry.Effort) != "" && strings.TrimSpace(entry.Thinking) == "" {
		entry.Thinking = "adaptive"
	}
	return &entry, nil
}

func (r bootSubagentResolver) provider(modelRef, effort string) (provider.Provider, *provider.Pricing, int, error) {
	entry, err := r.entry(modelRef, effort)
	if err != nil {
		return nil, nil, 0, err
	}
	if r.subscription {
		return provider.FailClosedProvider{Message: "subscription subagent is dispatcher-only"}, entry.Price, entry.ContextWindow, nil
	}
	prov, err := newProviderWithProxyForOwner(entry, r.proxy, r.owner)
	if err != nil {
		return nil, nil, 0, err
	}
	return prov, entry.Price, entry.ContextWindow, nil
}

func (r bootSubagentResolver) dispatchers(modelRef, effort string, _ provider.Provider) (agent.SubagentStreamDispatchers, error) {
	entry, err := r.entry(modelRef, effort)
	if err != nil {
		return agent.SubagentStreamDispatchers{}, err
	}
	main, err := r.billing.roleBoundStreamDispatcher(modelbilling.CallRoleSubagent, entry.Model)
	if err != nil {
		return agent.SubagentStreamDispatchers{}, err
	}
	compaction, err := r.billing.roleBoundStreamDispatcher(modelbilling.CallRoleCompaction, entry.Model)
	if err != nil {
		return agent.SubagentStreamDispatchers{}, err
	}
	return agent.SubagentStreamDispatchers{Main: main, Compaction: compaction}, nil
}

func buildBootAutoPlanClassifier(cfg *config.Config, tokenEconomy, subscription bool, billing bootModelBilling, proxy netclient.ProxySpec, owner string, sink event.Sink) (*control.ProviderAutoPlanClassifier, error) {
	if tokenEconomy || strings.EqualFold(strings.TrimSpace(cfg.Agent.AutoPlan), "off") || cfg.Agent.AutoPlanClassifier == "" {
		return nil, nil
	}
	modelRef := cfg.Agent.AutoPlanClassifier
	entry, ok := cfg.ResolveModel(modelRef)
	if !ok {
		return nil, fmt.Errorf("auto_plan_classifier %q is not a configured provider", modelRef)
	}
	classifierProv, err := bootDerivedProvider(entry, subscription, "subscription classifier is dispatcher-only", proxy, owner)
	if err != nil {
		return nil, fmt.Errorf("auto_plan_classifier %q: %w", modelRef, err)
	}
	classifierProv, err = billing.roleBoundProvider(classifierProv, modelbilling.CallRoleClassifier, entry.Model)
	if err != nil {
		return nil, fmt.Errorf("auto_plan_classifier %q dispatcher: %w", modelRef, err)
	}
	return control.NewBillableProviderAutoPlanClassifier(classifierProv, entry.Price, sink), nil
}

func bootDerivedProvider(entry *config.ProviderEntry, subscription bool, failClosedMessage string, proxy netclient.ProxySpec, owner string) (provider.Provider, error) {
	if subscription {
		return provider.FailClosedProvider{Message: failClosedMessage}, nil
	}
	return newProviderWithProxyForOwner(entry, proxy, owner)
}

type bootPlannerOptions struct {
	cfg          *config.Config
	tokenEconomy bool
	entry        *config.ProviderEntry
	baseProvider provider.Provider
	baseRunner   agent.Runner
	executor     *agent.Agent
	registry     *tool.Registry
	gate         agent.Gate
	toolTimeout  time.Duration
	keepPolicy   agent.KeepPolicy
	memoryBlock  string
	classifier   control.AutoPlanClassifier
	billing      bootModelBilling
	subscription bool
	proxy        netclient.ProxySpec
	owner        string
	sink         event.Sink
}

type bootPlannerResult struct {
	runner                  agent.Runner
	taskTreePlannerProvider provider.Provider
	label                   string
}

func buildBootPlanner(opts bootPlannerOptions) (bootPlannerResult, error) {
	result := bootPlannerResult{runner: opts.baseRunner, label: opts.entry.Model}
	taskTreeProvider, err := opts.billing.roleBoundProvider(opts.baseProvider, modelbilling.CallRoleTasktreePlanner, opts.entry.Model)
	if err != nil {
		return result, fmt.Errorf("task-tree planner dispatcher: %w", err)
	}
	result.taskTreePlannerProvider = taskTreeProvider
	modelRef := opts.cfg.Agent.PlannerModel
	if modelRef == "" || opts.tokenEconomy {
		return result, nil
	}
	entry, ok := opts.cfg.ResolveModel(modelRef)
	if !ok {
		return result, fmt.Errorf("planner_model %q is not a configured provider", modelRef)
	}
	if entry.Model == opts.entry.Model {
		return result, nil
	}
	plannerProv, err := bootDerivedProvider(entry, opts.subscription, "subscription planner is dispatcher-only", opts.proxy, opts.owner)
	if err != nil {
		return result, fmt.Errorf("planner %q: %w", modelRef, err)
	}
	plannerDispatcher, err := opts.billing.roleBoundStreamDispatcher(modelbilling.CallRolePlanner, entry.Model)
	if err != nil {
		return result, fmt.Errorf("planner %q dispatcher: %w", modelRef, err)
	}
	plannerCompactionDispatcher, err := opts.billing.roleBoundStreamDispatcher(modelbilling.CallRoleCompaction, entry.Model)
	if err != nil {
		return result, fmt.Errorf("planner %q compaction dispatcher: %w", modelRef, err)
	}
	taskTreeProvider, err = opts.billing.roleBoundProvider(plannerProv, modelbilling.CallRoleTasktreePlanner, entry.Model)
	if err != nil {
		return result, fmt.Errorf("task-tree planner %q dispatcher: %w", modelRef, err)
	}
	result.taskTreePlannerProvider = taskTreeProvider
	result.runner = agent.NewCoordinator(plannerProv, agent.NewSession(agent.PlannerPromptWithContext(opts.memoryBlock)), entry.Price, agent.PlannerToolRegistry(opts.registry), agent.Options{
		MaxSteps:                opts.cfg.Agent.PlannerMaxSteps,
		MaxStepsKey:             "agent.planner_max_steps",
		Gate:                    opts.gate,
		ToolTimeout:             opts.toolTimeout,
		ContextWindow:           entry.ContextWindow,
		SoftCompactRatio:        opts.cfg.Agent.SoftCompactRatio,
		CompactRatio:            opts.cfg.Agent.CompactRatio,
		CompactForceRatio:       opts.cfg.Agent.CompactForceRatio,
		RecentKeep:              opts.cfg.Agent.RecentKeep,
		ArchiveDir:              config.ArchiveDir(),
		KeepPolicy:              opts.keepPolicy,
		ReasoningLanguage:       opts.cfg.ReasoningLanguage(),
		StreamDispatcher:        plannerDispatcher,
		DerivedStreamDispatcher: plannerCompactionDispatcher,
	}, opts.executor, opts.cfg.Agent.Temperature, opts.sink, control.NewPlannerGate(opts.classifier))
	result.label = opts.entry.Model + " + planner " + entry.Model
	return result, nil
}
