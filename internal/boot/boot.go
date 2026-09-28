// Package boot assembles a ready-to-drive control.Controller from configuration:
// it loads config, resolves the model(s), builds the tool registry (built-ins +
// plugins), wires the permission gate, and constructs the executor — optionally
// wrapping it in a two-model Coordinator. It is the one place that turns "what the
// user configured" into "a Controller a frontend can drive", so every frontend —
// the terminal TUI, the HTTP/SSE server, the desktop webview — shares the exact
// same assembly instead of each re-deriving it. Frontends pass only a sink and a
// couple of run knobs; everything else comes from config.
package boot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/agentmode"
	"kwu/internal/command"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/guardian"
	"kwu/internal/history"
	"kwu/internal/installsource"
	"kwu/internal/instruction"
	"kwu/internal/jobs"
	"kwu/internal/lsp"
	"kwu/internal/memory"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/outputstyle"
	"kwu/internal/permission"
	"kwu/internal/planmode"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/sandbox"
	"kwu/internal/scene"
	"kwu/internal/skill"
	"kwu/internal/tasktree"
	"kwu/internal/temporal"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
	"kwu/internal/tool/sessiontool"
	"kwu/internal/userctx"
)

// ErrUnknownModel is returned by Build when the configured model can't be
// resolved to a provider — e.g. a default_model left over from a renamed or
// removed provider. Callers can detect it (errors.Is) to re-run setup.
var ErrUnknownModel = errors.New("unknown model")

func agentKeepPolicy(keep []string) agent.KeepPolicy {
	if keep == nil {
		return agent.KeepErrors
	}
	var p agent.KeepPolicy
	for _, k := range keep {
		switch strings.TrimSpace(k) {
		case "errors":
			p |= agent.KeepErrors
		case "user_marked":
			p |= agent.KeepUserMarked
		}
	}
	return p
}

// Options carries the per-run knobs a frontend chooses; everything else is read
// from configuration. Model "" falls back to the configured default_model;
// MaxSteps 0 uses the config/default. RequireKey forces the executor's API key to
// be present (run/serve pass true so a missing key fails fast; chat/desktop pass
// false so the UI is reachable before a key is set). Sink receives the agent's
// typed event stream.
type Options struct {
	Model      string
	MaxSteps   int
	RequireKey bool
	Sink       event.Sink
	// AgentMode is a request-scoped override for a new/one-shot session. Empty
	// uses the configured default mode.
	AgentMode string
	// SessionAgentMode is mode metadata restored from an existing session. When
	// present it wins over request and config defaults so resume is stable.
	SessionAgentMode string
	// EffortOverride is a session-local reasoning effort override. Nil means use
	// the resolved provider config; a non-nil empty string means provider default.
	EffortOverride *string
	// Stderr is the writer for diagnostic warnings and plugin subprocess
	// stderr output. When nil, defaults to os.Stderr. Set to io.Discard
	// during model switch inside a bubbletea session to prevent any output
	// from corrupting the TUI's terminal raw mode.
	Stderr io.Writer
	// WorkspaceRoot is the project root directory for config, skills, memory,
	// commands, hooks, and tool confinement. When empty, the current working
	// directory is used (CLI default). Desktop tabs pass their project root here
	// so each tab loads its own config and skills without changing the process
	// cwd — enabling concurrent multi-project sessions.
	WorkspaceRoot string
	// Timezone is a host/client supplied IANA timezone for this session. Empty
	// falls back to user-level agent.timezone and then the host timezone.
	Timezone string
	// Actor is the runtime actor/profile context for this controller.
	Actor userctx.ActorContext
	ModelBillingOptions
	// ExtraPlugins are session-scoped MCP servers supplied by a host transport
	// (for example ACP session/new). They are connected eagerly for this
	// controller but are not persisted to clouvia.toml.
	ExtraPlugins []plugin.Spec
	// ManagedMCPConfigNames are host-owned connector IDs/runtime names that must
	// not be started from user-editable static MCP configuration.
	ManagedMCPConfigNames []string
	// ManagedSkillPaths are verified, host-owned skill roots. They are not
	// persisted into user-editable custom skill settings.
	ManagedSkillPaths []string
	// TokenMode selects how much optional context/tool surface this session exposes
	// at boot. Empty/full preserves the normal capability surface. "economy" keeps
	// the core coding tools visible and moves skills, MCP, LSP, web_fetch,
	// install_source, and task behind connect_tool_source.
	TokenMode string
	// DesktopVersion is supplied by the Desktop host for the user-facing product
	// identity policy. Other hosts leave it empty.
	DesktopVersion string
	// SessionDir overrides where persisted chat transcripts are written. When
	// empty, the actor profile's CLI/global session directory is used.
	SessionDir string
	// SessionSendGuard is an optional host authorization check evaluated at the
	// controller's central task boundary before every model-bound turn.
	SessionSendGuard func(context.Context, string) error
	// SessionToolProvider supplies trusted tools for the current persisted
	// session after SessionSendGuard authorizes the turn.
	SessionToolProvider control.SessionToolProvider
	// SharedHost is an optional plugin.Host shared across controllers for the
	// same workspace root. When set, boot.Build reuses its running clients
	// instead of creating new subprocesses, and the caller manages the host's
	// lifecycle. When nil, Build creates and owns a new host as before.
	SharedHost *plugin.Host
	// CleanupPendingReconciler retries delayed physical cleanup for session
	// artifacts left by a previous process. Nil uses the core physical-delete
	// reconciler; frontends with different deletion semantics can override it.
	CleanupPendingReconciler func(sessionDir string) error
	// ApprovalTimeout bounds how long a tool-approval or ask prompt blocks for a
	// user decision. Zero (default) waits forever — correct for an interactive
	// terminal. Headless/bot frontends pass a positive value so an unanswered
	// prompt can't wedge the session indefinitely (#4626, #4402).
	ApprovalTimeout time.Duration
	// ExpertID is an explicit session-level expert choice for new sessions.
	ExpertID string
	// SessionExpertID is restored from session metadata and wins over ExpertID
	// and config defaults.
	SessionExpertID string
	// Scene selects the Desktop prompt fragment. The zero value is internal
	// unset, preserving the pre-scene prompt for every unadopted host.
	Scene      scene.Scene
	SceneState scene.State
	// UserExpertPath overrides the local user expert store. Empty uses the
	// process-wide user store; tests pass an isolated path.
	UserExpertPath string
	// ExpertStorePath overrides the SQLite expert catalog. Empty uses the
	// process-wide user state database; tests pass an isolated path.
	ExpertStorePath string
	// ExpertCatalogLoader overrides packaged and SQLite expert loading. Desktop
	// uses it for service-backed catalogs held in process memory.
	ExpertCatalogLoader func() (*expert.Catalog, error)
	// DisableBuiltinExperts prevents this host from seeding the packaged system
	// catalog. Desktop enables it when system experts are supplied by Cloud.
	DisableBuiltinExperts bool
}

// Build loads config, resolves the model(s), and returns a Controller wrapping a
// single Agent, or a two-model Coordinator when agent.planner_model is set. The
// returned controller owns plugin subprocesses; call Close (via Controller.Close)
// to release them.
func Build(ctx context.Context, opts Options) (*control.Controller, error) {
	billing, err := newBootModelBilling(opts)
	if err != nil {
		return nil, err
	}
	defer billing.closeOnBuildError()
	if err := validateTaskContextOptions(opts); err != nil {
		return nil, err
	}
	stderr := defaultStderr(opts.Stderr)
	root := resolveWorkspaceRoot(opts.WorkspaceRoot)
	actor := opts.Actor.Normalize()
	actor.WorkspaceRoot = root
	credentialPaths := config.NewProfilePathResolver(actor)
	credentialOwner := config.CredentialOwnerForProfile(actor.ProfileID).Description()
	routeConfig, err := loadBootRouteConfiguration(opts, actor, root)
	if err != nil {
		return nil, err
	}
	subscriptionRoute := routeConfig.subscriptionRoute
	// One-time import of v1/v0.5 legacy config — runs before Load so the newly
	// migrated config and profile credentials are picked up this same boot. The
	// `kwu` CLI also runs it before config-only commands; this remains the shared
	// frontend fallback.
	cfg := routeConfig.cfg
	resolvedAgentMode, err := ResolveAgentMode(opts, cfg)
	if err != nil {
		return nil, err
	}
	modelName := opts.Model
	if modelName == "" {
		modelName = cfg.DefaultModel
	}
	config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, modelName)
	tokenMode := NormalizeTokenMode(opts.TokenMode)
	tokenEconomy := tokenMode == TokenModeEconomy
	keepPolicy := agentKeepPolicy(cfg.Agent.Keep)
	entry, ok := cfg.ResolveModel(modelName)
	if !ok {
		return nil, fmt.Errorf("%w %q (configured: %s); note: defining [[providers]] replaces the built-in presets, so add a [[providers]] entry for it or use a configured name, or run `kwu setup` to reconfigure", ErrUnknownModel, modelName, providerNames(cfg))
	}
	modelRef := entry.Name + "/" + entry.Model
	if err := validateBillingRouteExecutorModel(opts, modelRef); err != nil {
		return nil, err
	}
	if opts.EffortOverride != nil {
		entry.Effort = *opts.EffortOverride
		if entry.Kind == "anthropic" && strings.TrimSpace(entry.Effort) != "" && strings.TrimSpace(entry.Thinking) == "" {
			entry.Thinking = "adaptive"
		}
	}
	if err := validateExecutorCredential(opts, subscriptionRoute, cfg, modelName, entry, credentialOwner, credentialPaths); err != nil {
		return nil, err
	}
	// Serialize concurrent turn/background emissions; jobs end with Controller.Close.
	sink := event.Sync(opts.Sink)
	timeService := temporal.New(temporal.Options{
		Override:   opts.Timezone,
		Configured: cfg.Agent.Timezone,
		Warn: func(message string) {
			sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: message})
		},
	})
	if ignored := (planmode.Policy{AllowedTools: cfg.Agent.PlanModeAllowedTools}).IgnoredAllowedTools(); len(ignored) > 0 {
		sink.Emit(event.Event{
			Kind:  event.Notice,
			Level: event.LevelWarn,
			Text:  fmt.Sprintf("plan_mode_allowed_tools ignored known blocked entries: %s; this setting only declares extra read-only custom tools and cannot unlock known blocked tools or unsafe bash", strings.Join(ignored, ", ")),
		})
	}
	emitMigrationNotices(sink, routeConfig.migrated, routeConfig.migrationErr)
	emitMemoryCompilerRetirementNotice(sink, routeConfig.memoryCompilerConfigMigrated, routeConfig.memoryCompilerConfigErr)
	// Surface a missing API key before the first request appears to fail silently.
	if !subscriptionRoute && !opts.RequireKey && entry.RequiresAPIKey() && entry.APIKey() == "" {
		sink.Emit(event.Event{Kind: event.Notice, Text: missingAPIKeyNotice(modelName, entry, credentialOwner, credentialPaths.UserCredentialsPath())})
	}
	jm := jobs.NewManager(sink, jobs.WithStalledWarningAfter(time.Duration(cfg.BackgroundJobStalledWarningSeconds())*time.Second))
	sessionDir := opts.SessionDir
	if sessionDir == "" {
		sessionDir = credentialPaths.SessionDir()
	}
	archiveDir := credentialPaths.ArchiveDir()
	reconcileCleanupPending := opts.CleanupPendingReconciler
	if reconcileCleanupPending == nil {
		reconcileCleanupPending = control.ReconcileCleanupPending
	}
	if err := reconcileCleanupPending(sessionDir); err != nil {
		sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "cleanup-pending reconciliation failed: " + err.Error()})
	}
	proxySpec := cfg.OutboundProxySpec()
	if err := netclient.Validate(proxySpec); err != nil {
		return nil, err
	}
	balanceClient, err := netclient.NewHTTPClient(proxySpec, netclient.TransportOptions{})
	if err != nil {
		return nil, err
	}
	execProv, err := newExecutorProviderForRoute(subscriptionRoute, entry, proxySpec, credentialOwner)
	if err != nil {
		return nil, err
	}
	basePrompt, err := cfg.ResolveSystemPromptForRoot(root)
	if err != nil {
		return nil, err
	}
	// Fold the selected output style into the cache-stable prompt prefix.
	if st, ok := outputstyle.Resolve(cfg.Agent.OutputStyle, outputstyle.Dirs()); ok {
		basePrompt = outputstyle.Apply(basePrompt, st)
	}

	// Persistent memory (canonical CLOUVIA.md, compatible legacy/cross-tool docs,
	// and the auto-memory index)
	// folds into the system prompt exactly here, once: it becomes part of the
	// durable, cache-stable prefix every turn reuses, so memory costs nothing per
	// turn. Mid-session changes never touch this prefix — they ride the
	// controller's transient turn-injection and fold in on the next session.
	mem := memory.Load(memory.Options{CWD: root, UserDir: credentialPaths.MemoryUserDir(), ProjectStoreRoot: credentialPaths.MemoryProjectStoreRoot()})
	projectChecks := instruction.ExtractHostChecks(mem.Docs)

	// Skills: discover playbooks (built-in + project/custom/global) and fold their
	// one-liner index into the same cache-stable prefix — names + descriptions
	// only; bodies load on demand via run_skill or "/<name>". Bodies never enter
	// the prefix, so the index costs a fixed, small amount per turn.
	kwuHomeDir := credentialPaths.KWUHomeDir()
	baseSkillOptions := skill.Options{
		KWUHomeDir:    kwuHomeDir,
		ProjectRoot:   root,
		CustomPaths:   cfg.SkillCustomPaths(),
		ExcludedPaths: cfg.SkillExcludedPaths(),
		DisabledNames: cfg.DisabledSkillNames(),
		MaxDepth:      cfg.SkillMaxDepth(),
		Stderr:        opts.Stderr,
	}
	if err := validateManagedSkillPaths(baseSkillOptions, opts.ManagedSkillPaths); err != nil {
		return nil, err
	}
	managedCustomPaths := append(append([]string(nil), baseSkillOptions.CustomPaths...), opts.ManagedSkillPaths...)
	skillOptions := baseSkillOptions
	skillOptions.CustomPaths = managedCustomPaths
	skillStore := skill.New(skillOptions)
	skills := skillStore.List()
	allSkillStore := skill.New(skill.Options{KWUHomeDir: kwuHomeDir, ProjectRoot: root, CustomPaths: managedCustomPaths, ExcludedPaths: cfg.SkillExcludedPaths(), MaxDepth: cfg.SkillMaxDepth(), Stderr: io.Discard})
	allSkills := allSkillStore.List()

	userExpertPath := opts.UserExpertPath
	if strings.TrimSpace(userExpertPath) == "" {
		userExpertPath = config.UserExpertsPath()
	}
	loadExpertCatalog := resolveExpertCatalogLoader(ctx, opts, userExpertPath)
	expertCatalog, err := loadExpertCatalog()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Agent.DefaultExpert) != "" {
		if _, err := expertCatalog.Resolve(strings.TrimSpace(cfg.Agent.DefaultExpert)); err != nil {
			return nil, fmt.Errorf("agent.default_expert: %w", err)
		}
	}
	buildSystemPrompt := func(promptScene scene.Scene, sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
		currentCatalog, err := loadExpertCatalog()
		if err != nil {
			return "", expert.Selection{}, err
		}
		selection, err := expert.ResolveSelection(currentCatalog, sessionExpertID, requestExpertID, cfg.Agent.DefaultExpert)
		if err != nil {
			if strings.TrimSpace(sessionExpertID) != "" && strings.TrimSpace(requestExpertID) == "" {
				selection, err = expert.ResolveSelection(currentCatalog, expert.ExplicitNoneID, "", cfg.Agent.DefaultExpert)
				if err != nil {
					return "", selection, err
				}
			} else {
				return "", selection, err
			}
		}
		sysPrompt := basePrompt
		if selection.Selected {
			sysPrompt = expert.ApplyPrompt(sysPrompt, selection.Expert)
		}
		fragment, err := scene.Fragment(promptScene)
		if err != nil {
			return "", selection, err
		}
		if fragment != "" {
			sysPrompt += "\n\n" + fragment
		}
		if promptScene.IsSelectable() {
			sysPrompt += "\n\n" + config.VisibleWorkUpdatePolicy
		}
		sysPrompt += "\n\n" + config.UserDecisionPolicy + "\n\n" + config.ArtifactDeliveryPolicy
		sysPrompt += "\n\n" + config.PersistentTaskDispatchPolicy
		sysPrompt += "\n\n" + config.TemporalPolicy
		sysPrompt += "\n\n" + config.LanguagePolicy + "\n\n" + config.ModelIdentityPolicy(entry.Model)
		if tokenEconomy {
			sysPrompt += "\n\n" + tokenEconomyPrompt
		}
		sysPrompt = memory.Compose(sysPrompt, mem)
		if !tokenEconomy {
			sysPrompt = skill.ApplyIndex(sysPrompt, skills)
		}
		sysPrompt = appendDesktopProductIdentityPolicy(sysPrompt, opts.DesktopVersion)
		return sysPrompt, selection, nil
	}
	sysPrompt, expertSelection, err := buildSystemPrompt(opts.Scene, opts.SessionExpertID, opts.ExpertID)
	if err != nil {
		return nil, err
	}

	reg := tool.NewRegistry()
	bashSpec := sandbox.Spec{Mode: cfg.BashMode(), WriteRoots: cfg.WriteRootsForRoot(root), Network: cfg.Sandbox.Network}
	shell := sandbox.ResolveShell(cfg.Tools.Shell.Prefer, cfg.Tools.Shell.Path, stderr)
	bashSpec.Shell = shell
	if bashSpec.Mode == "enforce" && !sandbox.Available() {
		fmt.Fprintln(stderr, "warning: bash sandbox requested but unavailable on this platform; running bash unconfined")
	}
	if autoShellPrefer(cfg.Tools.Shell.Prefer) && shell.Kind == sandbox.ShellPowerShell {
		fmt.Fprintln(stderr, "warning: bash not found on PATH; the shell tool will run commands under Windows PowerShell. Install Git for Windows or WSL to use bash, or set [tools.shell] prefer=\"powershell\" to silence this.")
	}
	searchSpec := builtin.ResolveSearch(cfg.Tools.Search.Engine, cfg.Tools.Search.RgPath, stderr)
	toolTimeout := time.Duration(cfg.ToolTimeoutSeconds()) * time.Second
	bashTimeout := time.Duration(cfg.BashTimeoutSeconds()) * time.Second
	enabledBuiltins := cfg.Tools.Enabled
	if tokenEconomy {
		enabledBuiltins = tokenEconomyBuiltins(enabledBuiltins)
	}
	addBuiltins(reg, enabledBuiltins, cfg.WriteRootsForRoot(root), bashSpec, bashTimeout, searchSpec, stderr, root, proxySpec)
	// Use the caller-supplied shared host when set, so controllers for the same
	// workspace root reuse running MCP processes (e.g. one CodeGraph daemon
	// instead of one per tab). Otherwise construct a private host per controller.
	pluginHost := opts.SharedHost
	if pluginHost == nil {
		pluginHost = plugin.NewHost()
	}

	// Partition configured plugins by tier so eager can block when explicitly
	// requested while every other enabled MCP warms up in the background.
	projectTrusted := projectTrustedForActor(root, actor, credentialPaths)
	autoStartEntries := excludeManagedMCPEntries(projectMCPAutoStartEntries(cfg.AutoStartPlugins(), projectTrusted, sink), opts.ManagedMCPConfigNames)
	eagerEntries, bgEntries := partitionByTier(autoStartEntries)
	extraSpecs := applyPlanModeAllowedMCPToolTrust(applyKnownPluginOverrides(opts.ExtraPlugins, root), cfg.Agent.PlanModeAllowedTools)
	onDemandMCPSpecs := map[string]plugin.Spec{}
	onDemandMCPNames := []string{}
	if tokenEconomy {
		for _, spec := range append(PluginSpecsForRootWithPlanModeAllowedTools(autoStartEntries, root, cfg.Agent.PlanModeAllowedTools), extraSpecs...) {
			name := strings.TrimSpace(spec.Name)
			if name == "" {
				continue
			}
			if _, exists := onDemandMCPSpecs[name]; !exists {
				onDemandMCPNames = append(onDemandMCPNames, name)
			}
			onDemandMCPSpecs[name] = spec
		}
		eagerEntries, bgEntries = nil, nil
	}
	trustedMCPServers := planModeTrustedMCPServers(onDemandMCPSpecs)

	// Auto-demote: any eager plugin that has been chronically slow (recent
	// samples repeatedly hit the blocking startup budget) drops to background
	// for this session. The user keeps eager intent, just doesn't pay for it
	// on a server that's been misbehaving. A notice surfaces the demotion.
	var demoteMessages []string
	budget := plugin.DefaultStartupBudget()
	kept := eagerEntries[:0]
	for _, e := range eagerEntries {
		rec := plugin.Recommend(e.Name, budget, 0)
		if rec.Demote {
			demoteMessages = append(demoteMessages, rec.Reason)
			bgEntries = append(bgEntries, e)
			continue
		}
		kept = append(kept, e)
	}
	eagerEntries = kept

	eagerSpecs := PluginSpecsForRootWithPlanModeAllowedTools(eagerEntries, root, cfg.Agent.PlanModeAllowedTools)
	bgSpecs := PluginSpecsForRootWithPlanModeAllowedTools(bgEntries, root, cfg.Agent.PlanModeAllowedTools)

	if !tokenEconomy {
		eagerSpecs = append(eagerSpecs, extraSpecs...)
	}

	// Apply caller-supplied stderr override to every spec across tiers.
	if opts.Stderr != nil {
		for i := range eagerSpecs {
			eagerSpecs[i].Stderr = opts.Stderr
		}
		for i := range bgSpecs {
			bgSpecs[i].Stderr = opts.Stderr
		}
	}

	// Eager: block until handshake. Unavailable servers stay out of the tool
	// registry and remain inspectable through /mcp without interrupting chat.
	if len(eagerSpecs) > 0 {
		// When using a shared host, reuse already-connected clients and
		// add new ones directly to the host instead of creating a separate one.
		if opts.SharedHost != nil {
			for _, s := range eagerSpecs {
				if pluginHost.HasClient(s.Name) {
					tools, err := pluginHost.ToolsFor(ctx, s.Name)
					if err == nil {
						for _, t := range tools {
							reg.Add(t)
						}
						continue
					}
				}
				// Use a bounded per-plugin timeout matching StartAvailable's
				// defaultStartTimeout (5s) so a hanging MCP server doesn't
				// block the tab boot indefinitely.
				addCtx, addCancel := context.WithTimeout(ctx, 5*time.Second)
				tools, err := pluginHost.AddWithLifecycle(ctx, addCtx, s)
				addCancel()
				if err != nil {
					if plugin.IsServerAlreadyConnected(err) || errors.Is(err, plugin.ErrSpawningInFlight) {
						// Race: another tab connected the same server between
						// HasClient and Add, or is currently spawning it.
						// Fetch tools from the existing client, or wait briefly.
						tools, err2 := pluginHost.ToolsFor(ctx, s.Name)
						if err2 == nil {
							for _, t := range tools {
								reg.Add(t)
							}
							continue
						}
					}
					recordUnavailableMCP(stderr, pluginHost, s, err)
					continue
				}
				for _, t := range tools {
					reg.Add(t)
				}
			}
		} else {
			host, ptools := plugin.StartAvailable(ctx, eagerSpecs)
			pluginHost = host
			for _, t := range ptools {
				reg.Add(t)
			}
			// PhaseB (prompts + resources) runs on the boot ctx — which is the
			// controller's session-scoped PluginCtx — so the auxiliary surfaces
			// keep streaming in after Start returns without holding up the agent.
			go host.StartPhaseB(ctx, sink)
		}
	}

	// Background: register placeholder tools now and kick off the real spawn.
	// Everything shares the same pluginHost so /mcp status, hot-add, and Close
	// see one cohesive set of servers.
	registerBackground := func(specs []plugin.Spec) {
		for _, s := range specs {
			// Already running on the shared host? Register tools directly.
			if pluginHost.HasClient(s.Name) {
				tools, err := pluginHost.ToolsFor(ctx, s.Name)
				if err == nil {
					for _, t := range tools {
						reg.Add(t)
					}
					continue
				}
			}
			if opts.SharedHost != nil {
				// Shared host relies on Host's spawn guard to avoid duplicate
				// processes across tabs for the same workspace root.
				cs, _ := plugin.LoadCachedSchema(s.Name, plugin.SpecFingerprint(s))
				for _, t := range plugin.LazyToolset(s, cs, pluginHost, reg, ctx, true) {
					reg.Add(t)
				}
			} else {
				cs, _ := plugin.LoadCachedSchema(s.Name, plugin.SpecFingerprint(s))
				for _, t := range plugin.LazyToolset(s, cs, pluginHost, reg, ctx, true) {
					reg.Add(t)
				}
			}
		}
	}
	registerBackground(bgSpecs)

	for _, msg := range demoteMessages {
		sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: msg})
	}

	cleanup := pluginHost.Close
	if opts.SharedHost != nil {
		// The caller owns the shared host's lifecycle; the controller must not
		// close it. A no-op cleanup keeps Controller.Close happy without
		// shutting down MCP processes that other controllers still use.
		cleanup = func() {}
	}

	// LSP tools resolve their servers on PATH and spawn lazily on first query, so
	// registering them is cheap even when no server is installed (a query then
	// returns an install hint). The manager is session-scoped; chain its shutdown
	// into the controller's cleanup so servers stop with the session, not the turn.
	var lspMgr *lsp.Manager
	lspToolsAdded := false
	addLSPTools := func() []string {
		if lspToolsUnavailable(lspMgr, lspToolsAdded) {
			return nil
		}
		lspToolsAdded = true
		return addTools(reg, lsp.Tools(lspMgr))
	}
	if cfg.LSP.Enabled {
		lspMgr = lsp.NewManager(root, LSPSpecs(cfg.LSP))
		if !tokenEconomy {
			addLSPTools()
		}
		prev := cleanup
		cleanup = func() { prev(); lspMgr.Close() }
	}

	maxSteps := cfg.Agent.MaxSteps
	if opts.MaxSteps > 0 {
		maxSteps = opts.MaxSteps
	}
	subagentStore, err := newSubagentStore(sessionDir)
	if err != nil {
		return nil, err
	}
	if subagentStore != nil {
		subagentStore.WithDestroyedChecker(jm.IsDestroying)
	}

	// Permission policy gates every tool call. The headless gate (no Approver)
	// resolves "ask" to allow — preserving `kwu run` autonomy — while deny
	// rules hard-block in every mode. Interactive frontends (chat, desktop) swap
	// in an interactive gate later via Controller.EnableInteractiveApproval.
	// Sub-agents always run headless: they have no UI to answer a prompt, so they
	// inherit this same gate.
	policy := permission.New(cfg.Permissions.Mode, cfg.Permissions.Allow, cfg.Permissions.Ask, cfg.Permissions.Deny)
	headlessGate := permission.NewGate(policy, nil)

	// The `task` tool spawns sub-agents that reuse the parent's provider and
	// tool registry. Wired here after the built-ins / plugins are loaded so
	// sub-agents inherit the full tool set (minus `task` itself, to keep
	// nesting out of the picture). It registers into the same reg the
	// executor uses, so the model surfaces it like any other tool.
	subagentResolver := bootSubagentResolver{
		cfg: cfg, defaultEntry: entry, billing: billing, subscription: subscriptionRoute,
		proxy: proxySpec, owner: credentialOwner,
	}
	resolveSubagentProvider := subagentResolver.provider
	resolveSubagentDispatchers := subagentResolver.dispatchers
	subagentIdentity := func(modelRef, effort string) (string, string) {
		return subagentEffectiveIdentity(cfg, modelName, entry, modelRef, effort)
	}
	taskModel := firstNonEmpty(cfg.Agent.SubagentModels["task"], cfg.Agent.SubagentModel)
	taskEffort := firstNonEmpty(cfg.Agent.SubagentEfforts["task"], cfg.Agent.SubagentEffort)
	taskToolAdded := false
	readOnlyTaskToolAdded := false
	var taskTool *agent.TaskTool
	newTaskTool := func() *agent.TaskTool {
		task := agent.NewTaskTool(execProv, entry.Price, reg, maxSteps,
			entry.ContextWindow, cfg.Agent.RecentKeep, cfg.Agent.SoftCompactRatio, cfg.Agent.CompactRatio, cfg.Agent.CompactForceRatio,
			cfg.Agent.Temperature, archiveDir, "", headlessGate,
			keepPolicy,
			taskModel, taskEffort, resolveSubagentProvider).
			WithToolTimeout(toolTimeout).
			WithTranscripts(subagentStore, root, modelName, entry.Effort).
			WithSubagentEffortLevels(subagentEffortLevels(cfg, entry, taskModel)).
			WithActorContext(actor).
			WithTranscriptIdentityResolver(subagentIdentity).
			WithStreamDispatcherResolver(resolveSubagentDispatchers)
		if opts.RejectBackgroundSubagents {
			task.WithBackgroundPolicy(func(context.Context) error {
				return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "Desktop strict/fake background subagent is disabled"}
			})
		}
		return task
	}
	addTaskTool := func() string {
		if taskToolAdded {
			return "task tool is already enabled."
		}
		taskToolAdded = true
		if taskTool == nil {
			taskTool = newTaskTool()
		}
		reg.Add(taskTool)
		reg.Add(agent.NewWaitTaskTool(taskTool))
		reg.Add(agent.NewStopTaskTool(taskTool))
		reg.Add(agent.NewParallelTasksTool(taskTool, reg))
		return "enabled task."
	}
	addReadOnlyTaskTool := func() string {
		if readOnlyTaskToolAdded {
			return "read_only_task tool is already enabled."
		}
		readOnlyTaskToolAdded = true
		if taskTool == nil {
			taskTool = newTaskTool()
		}
		reg.Add(agent.NewReadOnlyTaskTool(taskTool))
		return "enabled read_only_task."
	}
	if !tokenEconomy {
		addTaskTool()
		addReadOnlyTaskTool()
	}

	// The `memory` tool searches/reads saved facts on demand; `remember` persists
	// durable facts to the project's auto-memory store; `forget` prunes ones that
	// turn out wrong. The saved index loads into the prefix on the next session.
	addCurrentDatetimeAndMediaTool(
		reg, builtin.NewCurrentDatetime(timeService), opts.MediaToolCredential,
		opts.MediaImageQuotaPricing, opts.MediaVideoClient, opts.MediaUnderstandingClient, opts.MediaVideoScope,
		balanceClient, filepath.Join(root, ".clouvia", "generated-media"), root,
	)
	reg.Add(history.NewTool(history.Options{SessionDir: sessionDir, GlobalSessionDir: credentialPaths.SessionDir(), ArchiveDir: archiveDir}))

	// Session history tools let the AI discover and read past conversations.
	// `list_sessions` returns all saved session files; `read_session` loads one
	// and renders the full conversation as readable text.
	reg.Add(sessiontool.NewListSessionsTool(sessionDir))
	reg.Add(sessiontool.NewReadSessionTool(sessionDir))

	reg.Add(memory.NewRecallTool(mem.Store))
	reg.Add(memory.NewRememberTool(mem.Store))
	reg.Add(memory.NewForgetTool(mem.Store))

	// The `ask` tool puts structured multiple-choice questions to the user. It
	// reaches them through the Asker on the call context, which interactive
	// frontends wire to the controller (EnableInteractiveApproval); a headless run
	// has none, so ask resolves to "decide for yourself".
	reg.Add(agent.NewAskTool())

	// Skill tools: read_only_skill is a narrow plan-mode-safe entry point; the
	// full skills source adds run_skill / install_skill plus the dedicated
	// subagent wrappers (explore / research / review / security_review). Read-only
	// subagent skills run ephemerally with the same registry boundary as
	// read_only_task, so they cannot write, install, mutate memory, resume/fork
	// transcripts, or delegate further.
	readOnlySkillRunner := func(sctx context.Context, sk skill.Skill, task string, runOpts skill.SubagentRunOptions) (string, error) {
		if strings.TrimSpace(runOpts.ContinueFrom) != "" || strings.TrimSpace(runOpts.ForkFrom) != "" {
			return "", fmt.Errorf("read_only_skill does not support continue_from/fork_from")
		}
		sk = skill.WithCodeGraphTools(sk, skill.CodeGraphReadTools(agent.TaskToolRegistry(sctx, reg)))
		prov, price, ctxWin := execProv, entry.Price, entry.ContextWindow
		modelRef := subagentModelRef(cfg, sk)
		effortRef := subagentEffortRef(cfg, sk)
		if modelRef != "" || effortRef != "" {
			p, pr, cw, err := resolveSubagentProvider(modelRef, effortRef)
			if err != nil {
				return "", fmt.Errorf("read-only subagent skill %q profile: %w", sk.Name, err)
			}
			prov, price, ctxWin = p, pr, cw
		}
		dispatchers, err := resolveSubagentDispatchers(modelRef, effortRef, prov)
		if err != nil {
			return "", fmt.Errorf("read-only subagent skill %q dispatcher: %w", sk.Name, err)
		}
		subReg := agent.ReadOnlySubagentToolRegistry(agent.TaskToolRegistry(sctx, reg), sk.AllowedTools)
		if subReg.Len() == 0 {
			return "", fmt.Errorf("read_only_skill: skill %q has no read-only tools available", sk.Name)
		}
		steps := maxSteps
		if steps > 0 {
			if steps /= 2; steps < 5 {
				steps = 5
			}
		}
		sysPrompt := agent.DefaultReadOnlyTaskSystemPrompt + "\n\nSkill instructions:\n" + sk.Body
		return agent.RunSubAgentWithSession(sctx, prov, subReg, agent.NewSession(sysPrompt), task, agent.Options{
			MaxSteps:                steps,
			Temperature:             cfg.Agent.Temperature,
			Pricing:                 price,
			UsageSource:             event.UsageSourceSubagent,
			Gate:                    headlessGate,
			ToolTimeout:             toolTimeout,
			ContextWindow:           ctxWin,
			RecentKeep:              cfg.Agent.RecentKeep,
			SoftCompactRatio:        cfg.Agent.SoftCompactRatio,
			CompactRatio:            cfg.Agent.CompactRatio,
			CompactForceRatio:       cfg.Agent.CompactForceRatio,
			ArchiveDir:              config.ArchiveDir(),
			KeepPolicy:              keepPolicy,
			ReasoningLanguage:       agent.ReasoningLanguageFromContext(sctx),
			StreamDispatcher:        dispatchers.Main,
			DerivedStreamDispatcher: dispatchers.Compaction,
		}, agent.NestedSink(sctx, event.Discard))
	}
	// Writer-capable subagent skills reuse the sub-agent machinery via this
	// runner: an isolated loop with the skill body as system prompt, a tool set
	// scoped to the skill's allowed-tools (minus recursive meta-tools), optional
	// per-skill model, and resumable transcripts when the parent session supports
	// them. Its tool activity nests under the invoking call, like `task`.
	skillRunner := func(sctx context.Context, sk skill.Skill, task string, runOpts skill.SubagentRunOptions) (string, error) {
		sk = skill.WithCodeGraphTools(sk, skill.CodeGraphReadTools(agent.TaskToolRegistry(sctx, reg)))
		prov, price, ctxWin := execProv, entry.Price, entry.ContextWindow
		modelRef := subagentModelRef(cfg, sk)
		effortRef := subagentEffortRef(cfg, sk)
		if modelRef != "" || effortRef != "" {
			p, pr, cw, err := resolveSubagentProvider(modelRef, effortRef)
			if err != nil {
				return "", fmt.Errorf("subagent skill %q profile: %w", sk.Name, err)
			}
			prov, price, ctxWin = p, pr, cw
		}
		dispatchers, err := resolveSubagentDispatchers(modelRef, effortRef, prov)
		if err != nil {
			return "", fmt.Errorf("subagent skill %q dispatcher: %w", sk.Name, err)
		}
		subReg := agent.SubagentToolRegistry(agent.TaskToolRegistry(sctx, reg), sk.AllowedTools)
		continueFrom := strings.TrimSpace(runOpts.ContinueFrom)
		legacyForkFrom := strings.TrimSpace(runOpts.ForkFrom)
		if continueFrom != "" && legacyForkFrom != "" {
			return "", fmt.Errorf("continue_from and fork_from are mutually exclusive; pass only continue_from")
		}
		parentID, _, _, _ := agent.CallContext(sctx)
		parentSession := agent.ParentSession(sctx)
		var run *agent.SubagentRun
		if subagentStore == nil || parentSession == "" {
			// Headless runs (e.g. `kwu run`) have no persistent session to
			// own a transcript. Run the skill sub-agent ephemerally, as before
			// persisted transcripts existed, instead of failing. Continuation needs
			// a persisted owner, so it errors here.
			if continueFrom != "" || legacyForkFrom != "" {
				return "", fmt.Errorf("subagent continuation requires a persisted session; none is active in this run")
			}
			run = agent.EphemeralSubagentRun(sk.Body)
		} else {
			identityModel, identityEffort := subagentIdentity(modelRef, effortRef)
			spec := agent.SubagentSpec{
				Kind:             "skill",
				Name:             sk.Name,
				WorkspaceRoot:    root,
				ParentSession:    parentSession,
				ParentToolCallID: parentID,
				SystemPrompt:     sk.Body,
				Registry:         subReg,
				Model:            identityModel,
				Effort:           identityEffort,
				Actor:            actor,
			}
			var prepErr error
			if continueFrom != "" {
				run, prepErr = subagentStore.PrepareContinue(continueFrom, spec)
			} else if legacyForkFrom != "" {
				run, prepErr = subagentStore.PrepareLegacyForkFrom(legacyForkFrom, spec)
			} else {
				run, prepErr = subagentStore.PrepareFresh(spec)
			}
			if prepErr != nil {
				return "", prepErr
			}
		}
		defer run.Release()
		steps := maxSteps
		if steps > 0 {
			if steps /= 2; steps < 5 {
				steps = 5
			}
		}
		answer, err := agent.RunSubAgentWithSession(sctx, prov, subReg, run.Session, task, agent.Options{
			MaxSteps:                steps,
			Temperature:             cfg.Agent.Temperature,
			Pricing:                 price,
			UsageSource:             event.UsageSourceSubagent,
			Gate:                    headlessGate,
			ToolTimeout:             toolTimeout,
			ContextWindow:           ctxWin,
			RecentKeep:              cfg.Agent.RecentKeep,
			ArchiveDir:              config.ArchiveDir(),
			KeepPolicy:              keepPolicy,
			ReasoningLanguage:       agent.ReasoningLanguageFromContext(sctx),
			StreamDispatcher:        dispatchers.Main,
			DerivedStreamDispatcher: dispatchers.Compaction,
		}, agent.NestedSink(sctx, event.Discard))
		if err != nil {
			return "", errors.Join(err, subagentStore.SaveFailed(run))
		}
		if err := subagentStore.SaveCompleted(run); err != nil {
			return "", errors.Join(err, subagentStore.SaveFailed(run))
		}
		return agent.FormatSubagentRunResult(answer, run, false), nil
	}
	skillProfile := func(sk skill.Skill) *event.Profile {
		model, effort := subagentModelRef(cfg, sk), subagentEffortRef(cfg, sk)
		if model == "" && effort == "" {
			return nil
		}
		return &event.Profile{Model: model, Effort: effort}
	}
	// Custom slash commands (canonical .clouvia/commands plus compatible global,
	// legacy, and cross-tool directories). Best-effort: a malformed file is skipped,
	// and a load error never blocks the session.
	cmds, _ := command.Load(config.CommandDirsForRoot(root)...)
	addSlashCommandTool := func(includeSkills bool) {
		// Expose loaded slash commands to the model via slash_command. In economy
		// mode skills join this list only after the skills source is enabled.
		var slashEntries []command.SlashEntry
		if includeSkills {
			for _, sk := range skills {
				sk := sk
				slashEntries = append(slashEntries, command.SlashEntry{
					Name:        sk.Name,
					Description: sk.Description,
					Render:      func(args []string) string { return skill.Render(sk, strings.Join(args, " ")) },
				})
			}
		}
		for _, cmd := range cmds {
			cmd := cmd
			slashEntries = append(slashEntries, command.SlashEntry{
				Name:        cmd.Name,
				Description: cmd.Description,
				ArgHint:     cmd.ArgHint,
				Render:      func(args []string) string { return cmd.Render(args) },
			})
		}
		reg.Add(command.NewSlashCommandTool(slashEntries))
	}
	installSourceAdded := false
	addInstallSourceTool := func() string {
		if installSourceAdded {
			return "install_source is already enabled."
		}
		installSourceAdded = true
		reg.Add(installsource.NewTool(installsource.Options{
			ProjectRoot: root,
			HTTPClient:  balanceClient,
			ConnectMCP: func(e config.PluginEntry) (installsource.MCPConnectResult, error) {
				spec := pluginSpecFromEntry(e, root)
				if opts.Stderr != nil {
					spec.Stderr = opts.Stderr
				}
				tools, err := pluginHost.Add(ctx, spec)
				if err != nil {
					return installsource.MCPConnectResult{}, err
				}
				reg.RemovePrefix(plugin.ToolPrefix(spec.Name))
				for _, t := range tools {
					reg.Add(t)
				}
				// Disconnect closes the server and drops its namespaced tools.
				// Used by the install_source rollback path when SaveTo fails.
				disconnect := func() {
					if prefix, ok := pluginHost.Remove(spec.Name); ok {
						reg.RemovePrefix(prefix)
					}
				}
				return installsource.MCPConnectResult{
					ToolCount:  len(tools),
					Disconnect: disconnect,
				}, nil
			},
			OnDisconnect: func(serverName string) bool {
				if prefix, ok := pluginHost.Remove(serverName); ok {
					reg.RemovePrefix(prefix)
					return true
				}
				return false
			},
		}))
		return "enabled install_source."
	}
	readOnlySkillToolsAdded := false
	addReadOnlySkillTools := func() string {
		if readOnlySkillToolsAdded {
			return "read_only_skill tool is already enabled.\n\n" + skill.ReadOnlyIndexBlock(skills)
		}
		readOnlySkillToolsAdded = true
		reg.Add(skill.NewReadOnlySkillTool(skillStore, readOnlySkillRunner, skillProfile))
		return "enabled read_only_skill. Use read_only_skill for inline skills or read-only subagent skills on the next model request.\n\n" + skill.ReadOnlyIndexBlock(skills)
	}
	skillToolsAdded := false
	addSkillTools := func() string {
		if skillToolsAdded {
			return "skills are already enabled.\n\n" + skill.IndexBlock(skills)
		}
		skillToolsAdded = true
		addReadOnlySkillTools()
		reg.Add(skill.NewRunSkillTool(skillStore, skillRunner, skillProfile))
		reg.Add(skill.NewReadSkillTool(skillStore))
		reg.Add(skill.NewInstallSkillTool(skillStore, nil))
		for _, t := range skill.BuiltinSubagentTools(skillStore, skillRunner, skillProfile) {
			reg.Add(t)
		}
		addSlashCommandTool(true)
		return "enabled skills. Use run_skill/read_skill/read_only_skill or the dedicated skill tools on the next model request.\n\n" + skill.IndexBlock(skills)
	}
	if tokenEconomy {
		addSlashCommandTool(false)
	} else {
		addInstallSourceTool()
		addSkillTools()
	}
	if tokenEconomy {
		reg.Add(&toolSourceConnector{
			skills: func(context.Context) (string, error) {
				return addSkillTools(), nil
			},
			task: func(context.Context) (string, error) {
				return addTaskTool(), nil
			},
			readOnlyTask: func(context.Context) (string, error) {
				return addReadOnlyTaskTool(), nil
			},
			readOnlySkill: func(context.Context) (string, error) {
				return addReadOnlySkillTools(), nil
			},
			install: func(context.Context) (string, error) {
				return addInstallSourceTool(), nil
			},
			webFetch: func(context.Context) (string, error) {
				if !builtinToolEnabled(cfg.Tools.Enabled, "web_fetch") {
					return "web_fetch is disabled by [tools].enabled.", nil
				}
				names := addTools(reg, builtin.Workspace{
					Dir:         root,
					WriteRoots:  cfg.WriteRootsForRoot(root),
					Bash:        bashSpec,
					BashTimeout: bashTimeout,
					Search:      searchSpec,
					ProxySpec:   proxySpec,
				}.Tools("web_fetch"))
				if len(names) == 0 {
					return "web_fetch is already enabled or unavailable.", nil
				}
				return "enabled " + strings.Join(names, ", ") + ".", nil
			},
			lsp: func(context.Context) (string, error) {
				if lspMgr == nil {
					return "", fmt.Errorf("LSP is disabled in config")
				}
				names := addLSPTools()
				if len(names) == 0 {
					return "LSP tools are already enabled.", nil
				}
				return "enabled " + strings.Join(names, ", ") + ".", nil
			},
			mcp: func(_ context.Context, name string) (string, error) {
				spec, ok := onDemandMCPSpecs[name]
				if !ok {
					return "", fmt.Errorf("no configured MCP server named %q", name)
				}
				if opts.Stderr != nil {
					spec.Stderr = opts.Stderr
				}
				tools, err := pluginHost.Add(ctx, spec)
				if err != nil {
					// On a shared host the server may already be connected
					// (e.g. another tab started it). Fall back to fetching
					// its tools from the existing client.
					if errors.Is(err, plugin.ErrServerAlreadyConnected) || errors.Is(err, plugin.ErrSpawningInFlight) {
						tools, err2 := pluginHost.ToolsFor(ctx, spec.Name)
						if err2 != nil {
							return "", err2
						}
						reg.RemovePrefix(plugin.ToolPrefix(spec.Name))
						names := addTools(reg, tools)
						if len(names) == 0 {
							return fmt.Sprintf("MCP server %q connected but exposed no tools.", spec.Name), nil
						}
						return fmt.Sprintf("enabled MCP server %q tools: %s.", spec.Name, strings.Join(names, ", ")), nil
					}
					return "", err
				}
				reg.RemovePrefix(plugin.ToolPrefix(spec.Name))
				names := addTools(reg, tools)
				if len(names) == 0 {
					return fmt.Sprintf("MCP server %q connected but exposed no tools.", spec.Name), nil
				}
				return fmt.Sprintf("enabled MCP server %q tools: %s.", spec.Name, strings.Join(names, ", ")), nil
			},
			mcpNames:                 onDemandMCPNames,
			planModeAllowedTools:     cfg.Agent.PlanModeAllowedTools,
			planModeTrustedMCPServer: trustedMCPServers,
		})
	}

	execSess := agent.NewSession(sysPrompt)
	executorCompactionDispatcher, err := billing.roleBoundStreamDispatcher(modelbilling.CallRoleCompaction, entry.Model)
	if err != nil {
		return nil, fmt.Errorf("executor compaction dispatcher: %w", err)
	}
	executorSyntheticDispatcher, err := billing.roleBoundStreamDispatcher(modelbilling.CallRoleExecutor, entry.Model)
	if err != nil {
		return nil, fmt.Errorf("synthetic executor dispatcher: %w", err)
	}
	executor := agent.New(execProv, reg, execSess, agent.Options{
		MaxSteps:                maxSteps,
		Temperature:             cfg.Agent.Temperature,
		Pricing:                 entry.Price,
		Gate:                    headlessGate,
		ToolTimeout:             toolTimeout,
		Jobs:                    jm,
		ProjectChecks:           projectChecks,
		ContextWindow:           entry.ContextWindow,
		SoftCompactRatio:        cfg.Agent.SoftCompactRatio,
		CompactRatio:            cfg.Agent.CompactRatio,
		CompactForceRatio:       cfg.Agent.CompactForceRatio,
		RecentKeep:              cfg.Agent.RecentKeep,
		ArchiveDir:              archiveDir,
		KeepPolicy:              keepPolicy,
		ReasoningLanguage:       cfg.ReasoningLanguage(),
		PlanModeAllowedTools:    cfg.Agent.PlanModeAllowedTools,
		StreamDispatcher:        billing.dispatcher,
		DerivedStreamDispatcher: executorCompactionDispatcher,
	}, sink)

	var runner agent.Runner = executor
	classifier, err := buildBootAutoPlanClassifier(cfg, tokenEconomy, subscriptionRoute, billing, proxySpec, credentialOwner, sink)
	if err != nil {
		return nil, err
	}
	plannerResult, err := buildBootPlanner(bootPlannerOptions{
		cfg: cfg, tokenEconomy: tokenEconomy, entry: entry, baseProvider: execProv,
		baseRunner: runner, executor: executor, registry: reg, gate: headlessGate,
		toolTimeout: toolTimeout, keepPolicy: keepPolicy, memoryBlock: mem.Block(), classifier: classifier,
		billing: billing, subscription: subscriptionRoute, proxy: proxySpec, owner: credentialOwner, sink: sink,
	})
	if err != nil {
		return nil, err
	}
	runner = plannerResult.runner
	taskTreePlannerProvider := plannerResult.taskTreePlannerProvider
	label := plannerResult.label
	runner, err = agentmode.BuildRunner(resolvedAgentMode.Mode, agentmode.RunnerDeps{
		Normal:   runner,
		Executor: executor,
		Registry: reg,
		Sink:     sink,
		TaskTreePlanner: tasktree.NewLLMPlanner(tasktree.LLMPlannerOptions{
			Provider:    taskTreePlannerProvider,
			Temperature: cfg.Agent.Temperature,
		}),
		TaskTreeStore: tasktree.NewMemoryStateStore(),
	})
	if err != nil {
		return nil, err
	}
	ctrlOpts := control.Options{
		Runner:                   runner,
		Executor:                 executor,
		ExecutorStreamDispatcher: executorSyntheticDispatcher,
		Sink:                     sink,
		Policy:                   policy,
		Label:                    label,
		ModelRef:                 modelRef,
		AgentMode:                resolvedAgentMode.Mode,
		AgentModeSource:          resolvedAgentMode.Source,
		SystemPrompt:             sysPrompt,
		Scene:                    opts.Scene,
		SceneState:               opts.SceneState,
		Experts:                  expertCatalog,
		Expert:                   expertSelection,
		ScenePromptBuilder:       buildSystemPrompt,
		ExpertCatalogLoader:      loadExpertCatalog,
		UserExpertPath:           userExpertPath,
		SessionDir:               sessionDir,
		SessionSendGuard:         opts.SessionSendGuard,
		SessionToolProvider:      opts.SessionToolProvider,
		Host:                     pluginHost,
		Commands:                 cmds,
		Skills:                   skills,
		AllSkills:                allSkills,
		SkillStore:               skillStore,
		AllSkillStore:            allSkillStore,
		Memory:                   mem,
		Cleanup:                  cleanup,
		BalanceURL:               balanceURLForRoute(entry, subscriptionRoute),
		BalanceKey:               balanceKeyForRoute(entry, subscriptionRoute),
		BalanceClient:            balanceClient,
		Jobs:                     jm,
		Registry:                 reg,
		PluginCtx:                ctx,
		WorkspaceRoot:            root,
		CommandEnvProvider:       pythonCommandEnvProvider(cfg, root),
		Actor:                    actor,
		AutoPlan:                 cfg.Agent.AutoPlan,
		ResponseLanguage:         cfg.ResponseLanguage(),
		ReasoningLanguage:        cfg.ReasoningLanguage(),
		DisableColdResumePrune:   !cfg.ColdResumePruneEnabled(),
		Shell:                    shell,
		PlanModeAllowedTools:     cfg.Agent.PlanModeAllowedTools,
		ApprovalTimeout:          opts.ApprovalTimeout,
		Temporal:                 timeService,
		OnRemember: func(rule string) control.RememberResult {
			return rememberPermissionRuleForActor(actor, rule)
		},
	}
	// Guardian: when guardian_model is configured, spawn an LLM safety reviewer
	// that can auto-allow safe Ask decisions and annotate risky ones before
	// escalating to the human approval prompt.
	if guardianModel := cfg.Agent.GuardianModel; guardianModel != "" {
		ge, ok := cfg.ResolveModel(guardianModel)
		if !ok {
			slog.Warn("guardian model is not a configured provider — guardian disabled", "model", guardianModel)
			sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: fmt.Sprintf("guardian_model %q not found — guardian disabled", guardianModel)})
		} else if subscriptionRoute {
			guardianReg := agent.FilterReadOnlyRegistry(reg, agent.SubagentMetaTools()...)
			guardianDispatcher, err := billing.roleBoundStreamDispatcher(modelbilling.CallRoleGuardian, ge.Model)
			if err != nil {
				return nil, fmt.Errorf("guardian %q dispatcher: %w", guardianModel, err)
			}
			guardianCompactionDispatcher, err := billing.roleBoundStreamDispatcher(modelbilling.CallRoleCompaction, ge.Model)
			if err != nil {
				return nil, fmt.Errorf("guardian %q compaction dispatcher: %w", guardianModel, err)
			}
			ctrlOpts.Guardian = guardian.NewSessionWithDispatchers(provider.FailClosedProvider{Message: "subscription guardian is dispatcher-only"}, guardianReg, guardian.PolicyPrompt(), guardianModel, cfg.Agent.GuardianTemperature, ge.Price, sink, guardianDispatcher, guardianCompactionDispatcher)
			sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: fmt.Sprintf("guardian enabled; model=%s", ge.Model)})
		} else {
			pProv, err := newProviderWithProxyForOwner(ge, proxySpec, credentialOwner)
			if err != nil {
				slog.Warn("guardian provider construction failed — guardian disabled", "model", guardianModel, "err", err)
				sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: fmt.Sprintf("guardian construction failed: %v — guardian disabled", err)})
			} else {
				guardianReg := agent.FilterReadOnlyRegistry(reg, agent.SubagentMetaTools()...)
				guardianDispatcher, dispatchErr := billing.roleBoundStreamDispatcher(modelbilling.CallRoleGuardian, ge.Model)
				if dispatchErr != nil {
					return nil, fmt.Errorf("guardian %q dispatcher: %w", guardianModel, dispatchErr)
				}
				guardianCompactionDispatcher, dispatchErr := billing.roleBoundStreamDispatcher(modelbilling.CallRoleCompaction, ge.Model)
				if dispatchErr != nil {
					return nil, fmt.Errorf("guardian %q compaction dispatcher: %w", guardianModel, dispatchErr)
				}
				ctrlOpts.Guardian = guardian.NewSessionWithDispatchers(pProv, guardianReg, guardian.PolicyPrompt(), guardianModel, cfg.Agent.GuardianTemperature, ge.Price, sink, guardianDispatcher, guardianCompactionDispatcher)
				sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: fmt.Sprintf("guardian enabled · model=%s", ge.Model)})
			}
		}
	}
	if classifier != nil {
		ctrlOpts.Classifier = classifier
	}
	ctrlOpts.Cleanup = billing.chain(ctrlOpts.Cleanup)
	controller := control.New(withTaskContextOptions(ctrlOpts, opts))
	billing.handedOff = true
	return controller, nil
}

// ResolveAgentMode applies boot's mode precedence without constructing a
// controller. The runner factory added in the next task should consume this same
// resolved value rather than re-reading entry-surface strings.
func ResolveAgentMode(opts Options, cfg *config.Config) (agentmode.ResolvedMode, error) {
	configMode := ""
	if cfg != nil {
		configMode = cfg.Agent.Mode
	}
	return agentmode.ResolveModeSource(agentmode.ModeSource{
		Session: opts.SessionAgentMode,
		Request: opts.AgentMode,
		Config:  configMode,
	})
}

func rememberPermissionRule(workspaceRoot, rule string) control.RememberResult {
	return rememberPermissionRuleForActor(userctx.Default(workspaceRoot), rule)
}

func rememberPermissionRuleForActor(actor userctx.ActorContext, rule string) control.RememberResult {
	actor = actor.Normalize()
	paths := config.NewProfilePathResolver(actor)
	path := paths.UserConfigPath()
	result := control.RememberResult{Rule: strings.TrimSpace(rule), Path: path}
	if path == "" {
		result.Err = fmt.Errorf("cannot resolve user config directory")
		return result
	}
	// Persisting an approval rule needs settings metadata only. Never hydrate a
	// provider credential as a side effect of a permission choice; in particular
	// a subscription controller must not observe a local BYOK key here.
	edit := config.LoadForEditWithoutCredentialsWithPaths(paths)
	if coveredBy := coveredPermissionRule(edit.Permissions.Allow, result.Rule); coveredBy != "" {
		result.CoveredBy = coveredBy
		return result
	}
	edit.Permissions.Allow = pruneCoveredPermissionRules(edit.Permissions.Allow, result.Rule)
	if err := edit.AddPermissionRule("allow", result.Rule); err != nil {
		slog.Warn("persist permission rule", "rule", rule, "err", err)
		result.Err = err
		return result
	}
	if err := edit.SaveToScope(path, config.RenderScopeUser); err != nil {
		slog.Warn("save config after permission rule", "err", err)
		result.Err = err
		return result
	}
	result.Saved = true
	return result
}

func coveredPermissionRule(rules []string, rule string) string {
	for _, existing := range rules {
		if permission.RuleCoversString(existing, rule) {
			return strings.TrimSpace(existing)
		}
	}
	return ""
}

func pruneCoveredPermissionRules(rules []string, rule string) []string {
	out := rules[:0]
	for _, existing := range rules {
		if strings.TrimSpace(existing) == "" || permission.RuleCoversString(rule, existing) {
			continue
		}
		out = append(out, existing)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func validateErrorIsMissingAPIKey(entry *config.ProviderEntry) bool {
	return entry != nil && entry.Kind != "" && entry.BaseURL != "" && entry.RequiresAPIKey() && entry.APIKey() == ""
}

func missingAPIKeyError(modelName string, entry *config.ProviderEntry, owner, credentialsPath string) error {
	return fmt.Errorf("provider %q: missing env %s for %s%s", modelName, entry.APIKeyEnv, owner, credentialPathSuffix(credentialsPath))
}

func missingAPIKeyNotice(modelName string, entry *config.ProviderEntry, owner, credentialsPath string) string {
	return fmt.Sprintf("model %q is selected but its API key %s is not set for %s%s - requests will fail until you set it",
		modelName, entry.APIKeyEnv, owner, credentialPathSuffix(credentialsPath))
}

func credentialPathSuffix(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return " (credentials: " + path + ")"
}

func subagentModelRef(cfg *config.Config, sk skill.Skill) string {
	if cfg != nil {
		for _, key := range subagentModelKeys(sk.Name) {
			if m := strings.TrimSpace(cfg.Agent.SubagentModels[key]); m != "" {
				return m
			}
		}
	}
	if m := strings.TrimSpace(sk.Model); m != "" {
		return m
	}
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Agent.SubagentModel)
}

func subagentEffortRef(cfg *config.Config, sk skill.Skill) string {
	if cfg != nil {
		for _, key := range subagentModelKeys(sk.Name) {
			if e := strings.TrimSpace(cfg.Agent.SubagentEfforts[key]); e != "" {
				return e
			}
		}
	}
	if e := strings.TrimSpace(sk.Effort); e != "" {
		return e
	}
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Agent.SubagentEffort)
}

func subagentEffortLevels(cfg *config.Config, base *config.ProviderEntry, modelRef string) []string {
	entry := base
	if cfg != nil && strings.TrimSpace(modelRef) != "" {
		if resolved, ok := resolveSubagentModelEntry(cfg, base, modelRef); ok {
			entry = resolved
		}
	}
	cap := config.EffortCapabilityForEntry(entry)
	out := make([]string, 0, len(cap.Levels))
	for _, level := range cap.Levels {
		if level == "auto" {
			continue
		}
		out = append(out, level)
	}
	return out
}

func subagentModelKeys(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	keys := []string{name}
	for _, alias := range []string{
		strings.ReplaceAll(name, "-", "_"),
		strings.ReplaceAll(name, "_", "-"),
	} {
		if alias == "" {
			continue
		}
		seen := false
		for _, key := range keys {
			if key == alias {
				seen = true
				break
			}
		}
		if !seen {
			keys = append(keys, alias)
		}
	}
	return keys
}

func resolveWorkspaceRoot(explicit string) string {
	if explicit != "" {
		return explicit
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root, ok := nearestGitRoot(wd); ok {
		return root
	}
	return wd
}

func nearestGitRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		dir = filepath.Clean(start)
	}
	for {
		if isGitMarker(filepath.Join(dir, ".git")) {
			return dir, true
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", false
		}
		dir = next
	}
}

func isGitMarker(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && (fi.IsDir() || fi.Mode().IsRegular())
}

func subagentEffectiveIdentity(cfg *config.Config, baseModelRef string, base *config.ProviderEntry, modelRef, effort string) (string, string) {
	var entry config.ProviderEntry
	if base != nil {
		entry = *base
	}
	ref := strings.TrimSpace(modelRef)
	if ref == "" {
		ref = strings.TrimSpace(baseModelRef)
	}
	if cfg != nil && ref != "" {
		if resolved, ok := resolveSubagentModelEntry(cfg, base, ref); ok {
			entry = *resolved
		} else if strings.TrimSpace(modelRef) != "" {
			entry.Model = ref
		}
	} else if strings.TrimSpace(modelRef) != "" {
		entry.Model = strings.TrimSpace(modelRef)
	}
	if rawEffort := strings.TrimSpace(effort); rawEffort != "" {
		if normalized, err := config.NormalizeEffort(&entry, rawEffort); err == nil {
			entry.Effort = normalized
		} else {
			entry.Effort = rawEffort
		}
	}
	modelID := strings.TrimSpace(entry.Name)
	model := strings.TrimSpace(entry.Model)
	if modelID != "" && model != "" {
		modelID += "/" + model
	} else if model != "" {
		modelID = model
	} else if modelID == "" {
		modelID = ref
	}
	return modelID, strings.TrimSpace(config.EffectiveEffort(&entry))
}

// NewProvider builds a provider.Provider from a configured entry. Exported so
// custom assemblers (e.g. the ACP per-session factory) can reuse it without
// going through the full Build.
func NewProvider(e *config.ProviderEntry) (provider.Provider, error) {
	return NewProviderWithProxy(e, netclient.ProxySpec{Mode: netclient.ModeAuto})
}

// NewProviderWithProxy builds a provider.Provider with the configured ordinary
// network proxy settings.
func NewProviderWithProxy(e *config.ProviderEntry, proxy netclient.ProxySpec) (provider.Provider, error) {
	return newProviderWithProxyForOwner(e, proxy, "")
}

func newProviderWithProxyForOwner(e *config.ProviderEntry, proxy netclient.ProxySpec, credentialOwner string) (provider.Provider, error) {
	return provider.New(e.Kind, provider.Config{
		Name:    e.Name,
		BaseURL: e.BaseURL,
		Model:   e.Model,
		APIKey:  e.APIKey(),
		// Pass the key's env var so auth failures can name where to fix it, plus
		// provider-kind-specific knobs. EffectiveEffort applies a configured
		// default_effort when the user has not explicitly selected /effort.
		Extra: map[string]any{
			"api_key_env":        e.APIKeyEnv,
			"api_key_source":     e.APIKeySourceLabel(),
			"credential_owner":   credentialOwner,
			"thinking":           e.Thinking,
			"effort":             config.EffectiveEffort(e),
			"reasoning_protocol": config.ReasoningProtocolForEntry(e),
			"proxy_spec":         proxy,
			"vision":             config.EffectiveVision(e),
			"vision_detail":      e.VisionDetail,
		},
	})
}

// addBuiltins adds enabled built-in tools to reg. An empty list means all of
// them. writeRoots confines the file-writing built-ins to the workspace: after
// the (unconfined) defaults are added, each enabled writer is replaced by an
// instance bound to writeRoots (preserving registry order).
// When workDir is non-empty, tools resolve relative paths against it instead of
// the process cwd, enabling concurrent multi-project sessions.
func addBuiltins(reg *tool.Registry, enabled, writeRoots []string, bashSpec sandbox.Spec, bashTimeout time.Duration, searchSpec builtin.SearchSpec, stderr io.Writer, workDir string, proxySpec netclient.ProxySpec) {
	// If a workspace directory is set, use workspace-bound tools that resolve
	// paths relative to that directory. Otherwise fall back to the process-cwd
	// compile-time builtins.
	if workDir != "" {
		ws := builtin.Workspace{Dir: workDir, WriteRoots: writeRoots, Bash: bashSpec, BashTimeout: bashTimeout, Search: searchSpec, ProxySpec: proxySpec}
		for _, t := range workspaceBuiltinsForModel(ws, enabled, stderr) {
			reg.Add(t)
		}
		return
	}

	if len(enabled) == 0 {
		for _, t := range tool.Builtins() {
			if t.Name() != verifyArtifactTool {
				reg.Add(t)
			}
		}
	} else {
		for _, name := range modelBuiltinNames(enabled) {
			if t, ok := tool.LookupBuiltin(name); ok {
				reg.Add(t)
			} else {
				fmt.Fprintf(stderr, "warning: unknown built-in tool %q\n", name)
			}
		}
	}
	// Replace the unconfined defaults with confined instances (registry order is
	// preserved on replace): file-writers bound to the workspace, bash to the OS
	// sandbox, web_fetch to the proxy. Only replace tools actually enabled/present.
	confined := append(builtin.ConfineWriters(writeRoots), builtin.ConfineBash(bashSpec, bashTimeout), builtin.ConfineSearch(searchSpec), builtin.ConfineWebFetch(proxySpec))
	for _, t := range confined {
		if _, ok := reg.Get(t.Name()); ok {
			reg.Add(t)
		}
	}
}

func builtinToolEnabled(enabled []string, name string) bool {
	if len(enabled) == 0 {
		return true
	}
	name = strings.TrimSpace(name)
	for _, candidate := range enabled {
		if strings.TrimSpace(candidate) == name {
			return true
		}
	}
	return false
}

// partitionByTier splits configured plugin entries into eager (block boot until
// ready) and background (placeholder + start spawn now). Entries with an empty,
// legacy lazy, or unrecognised tier land in background.
func partitionByTier(entries []config.PluginEntry) (eager, bg []config.PluginEntry) {
	for _, e := range entries {
		switch e.ResolvedTier() {
		case "eager":
			eager = append(eager, e)
		default:
			bg = append(bg, e)
		}
	}
	return eager, bg
}

// PluginSpecs maps configured plugin entries to plugin.Spec, expanding ${VAR}
// references. Exported so custom assemblers can connect the config's plugins
// alongside their own (e.g. ACP's per-session MCP servers).
func PluginSpecs(entries []config.PluginEntry) []plugin.Spec {
	return PluginSpecsForRoot(entries, "")
}

// PluginSpecsForRoot maps configured plugin entries to plugin.Spec and applies
// workspace-aware compatibility overrides for known cwd-sensitive servers.
func PluginSpecsForRoot(entries []config.PluginEntry, workspaceRoot string) []plugin.Spec {
	return PluginSpecsForRootWithPlanModeAllowedTools(entries, workspaceRoot, nil)
}

// PluginSpecsForRootWithPlanModeAllowedTools also promotes model-visible MCP
// names declared in agent.plan_mode_allowed_tools to trusted read-only model
// names for their matching server. This keeps the planner/read-only research
// trust path aligned with the plan-mode execution escape valve.
func PluginSpecsForRootWithPlanModeAllowedTools(entries []config.PluginEntry, workspaceRoot string, allowedTools []string) []plugin.Spec {
	specs := pluginSpecsWithoutManagedConnectors(entries, workspaceRoot)
	return applyPlanModeAllowedMCPToolTrust(specs, allowedTools)
}

func pluginSpecFromEntry(e config.PluginEntry, workspaceRoot string) plugin.Spec {
	e = e.ExpandedPlugin() // resolve ${VAR} / ${VAR:-default} from the environment
	return plugin.ApplyKnownOverrides(plugin.Spec{
		Name:              e.Name,
		Type:              e.Type,
		Command:           e.Command,
		Args:              e.Args,
		Env:               e.Env,
		URL:               e.URL,
		Headers:           e.Headers,
		ReadOnlyToolNames: trustedRawReadOnlyToolNames(e.TrustedReadOnlyTools),
	}, workspaceRoot)
}

func applyPlanModeAllowedMCPToolTrust(specs []plugin.Spec, allowedTools []string) []plugin.Spec {
	if len(specs) == 0 || len(allowedTools) == 0 {
		return specs
	}
	out := make([]plugin.Spec, len(specs))
	for i, spec := range specs {
		out[i] = spec
		prefix := plugin.ToolPrefix(spec.Name)
		clonedModelNames := false
		for _, name := range allowedTools {
			name = strings.TrimSpace(name)
			if !strings.HasPrefix(name, prefix) || len(name) <= len(prefix) {
				continue
			}
			if out[i].ReadOnlyModelToolNames == nil {
				out[i].ReadOnlyModelToolNames = map[string]bool{}
				clonedModelNames = true
			} else if !clonedModelNames {
				out[i].ReadOnlyModelToolNames = cloneBoolMap(spec.ReadOnlyModelToolNames)
				clonedModelNames = true
			}
			out[i].ReadOnlyModelToolNames[name] = true
		}
	}
	return out
}

func trustedRawReadOnlyToolNames(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func planModeTrustedMCPServers(specs map[string]plugin.Spec) map[string]bool {
	if len(specs) == 0 {
		return nil
	}
	out := map[string]bool{}
	for name, spec := range specs {
		if len(spec.ReadOnlyToolNames) > 0 || len(spec.ReadOnlyModelToolNames) > 0 {
			out[name] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// autoShellPrefer reports whether [tools.shell] left the interpreter to
// auto-detection, so the "fell back to PowerShell" hint is suppressed once the
// user has explicitly chosen a shell.
func autoShellPrefer(prefer string) bool {
	p := strings.ToLower(strings.TrimSpace(prefer))
	return p == "" || p == "auto"
}

// recordUnavailableMCP keeps optional connector failures available to status
// surfaces and diagnostics without turning them into chat events.
func recordUnavailableMCP(stderr io.Writer, host *plugin.Host, spec plugin.Spec, err error) {
	host.RecordFailure(spec, err)
	fmt.Fprintf(stderr, "warning: MCP server %q unavailable: %v\n", spec.Name, err)
}

// LSPSpecs returns the language → server map: the built-in defaults overlaid with
// any user overrides. A user entry may set only the fields it wants to change;
// empty fields keep the default for that language.
func LSPSpecs(cfg config.LSPConfig) map[string]lsp.ServerSpec {
	specs := lsp.DefaultSpecs()
	for lang, s := range cfg.Servers {
		spec := specs[lang]
		if s.Command != "" {
			spec.Command = s.Command
		}
		if s.Args != nil {
			spec.Args = s.Args
		}
		if s.Env != nil {
			spec.Env = s.Env
		}
		if s.LanguageID != "" {
			spec.LanguageID = s.LanguageID
		}
		if s.Extensions != nil {
			spec.Extensions = s.Extensions
		}
		if s.InstallHint != "" {
			spec.InstallHint = s.InstallHint
		}
		if spec.LanguageID == "" {
			spec.LanguageID = lang
		}
		specs[lang] = spec
	}
	return specs
}

func providerNames(cfg *config.Config) string {
	names := make([]string, len(cfg.Providers))
	for i, p := range cfg.Providers {
		names[i] = p.Name
	}
	return strings.Join(names, "/")
}
