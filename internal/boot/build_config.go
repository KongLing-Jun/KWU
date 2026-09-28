package boot

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/instruction"
	"kwu/internal/jobs"
	"kwu/internal/memory"
	"kwu/internal/migration"
	"kwu/internal/netclient"
	"kwu/internal/outputstyle"
	"kwu/internal/planmode"
	"kwu/internal/skill"
	"kwu/internal/temporal"
)

func (b *buildAssembly) loadCore() error {
	if err := b.loadConfiguration(); err != nil {
		return err
	}
	b.initializeSession()
	return b.initializeNetwork()
}

func (b *buildAssembly) loadConfiguration() error {
	if b.stderr == nil {
		b.stderr = os.Stderr
	}
	b.root = resolveWorkspaceRoot(b.opts.WorkspaceRoot)
	b.actor = b.opts.Actor.Normalize()
	b.actor.WorkspaceRoot = b.root
	b.credentialPaths = config.NewProfilePathResolver(b.actor)
	b.credentialOwner = config.CredentialOwnerForProfile(b.actor.ProfileID).Description()
	b.migrated, b.migrationErr = migrateLegacyConfigForActor(b.actor, b.root)
	b.memoryCompilerConfigMigrated, b.memoryCompilerConfigErr = config.MigrateLegacyMemoryCompilerForRoot(b.root, b.credentialPaths)
	var err error
	b.cfg, err = config.LoadForRootWithActor(b.root, b.actor)
	if err != nil {
		return err
	}
	b.agentMode, err = ResolveAgentMode(b.opts, b.cfg)
	if err != nil {
		return err
	}
	b.modelName = b.opts.Model
	if b.modelName == "" {
		b.modelName = b.cfg.DefaultModel
	}
	config.NormalizeLegacyMimoCustomProvidersForRefs(b.cfg, b.modelName)
	b.tokenEconomy = NormalizeTokenMode(b.opts.TokenMode) == TokenModeEconomy
	b.keepPolicy = agentKeepPolicy(b.cfg.Agent.Keep)
	var ok bool
	b.entry, ok = b.cfg.ResolveModel(b.modelName)
	if !ok {
		return fmt.Errorf("%w %q (configured: %s); note: defining [[providers]] replaces the built-in presets, so add a [[providers]] entry for it or use a configured name, or run `kwu setup` to reconfigure", ErrUnknownModel, b.modelName, providerNames(b.cfg))
	}
	b.modelRef = b.entry.Name + "/" + b.entry.Model
	b.applyEffortOverride()
	return b.validateRequiredKey()
}

func (b *buildAssembly) applyEffortOverride() {
	if b.opts.EffortOverride == nil {
		return
	}
	b.entry.Effort = *b.opts.EffortOverride
	if b.entry.Kind == "anthropic" && strings.TrimSpace(b.entry.Effort) != "" && strings.TrimSpace(b.entry.Thinking) == "" {
		b.entry.Thinking = "adaptive"
	}
}

func (b *buildAssembly) validateRequiredKey() error {
	if !b.opts.RequireKey {
		return nil
	}
	if err := b.cfg.Validate(b.modelName); err != nil {
		if validateErrorIsMissingAPIKey(b.entry) {
			return missingAPIKeyError(b.modelName, b.entry, b.credentialOwner, b.credentialPaths.UserCredentialsPath())
		}
		return err
	}
	return nil
}

func (b *buildAssembly) initializeSession() {
	b.sink = event.Sync(b.opts.Sink)
	b.temporal = temporal.New(temporal.Options{
		Override:   b.opts.Timezone,
		Configured: b.cfg.Agent.Timezone,
		Warn: func(message string) {
			b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: message})
		},
	})
	b.emitStartupNotices()
	b.jobs = jobs.NewManager(b.sink, jobs.WithStalledWarningAfter(time.Duration(b.cfg.BackgroundJobStalledWarningSeconds())*time.Second))
	b.sessionDir = b.opts.SessionDir
	if b.sessionDir == "" {
		b.sessionDir = b.credentialPaths.SessionDir()
	}
	b.archiveDir = b.credentialPaths.ArchiveDir()
	reconcile := b.opts.CleanupPendingReconciler
	if reconcile == nil {
		reconcile = control.ReconcileCleanupPending
	}
	if err := reconcile(b.sessionDir); err != nil {
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "cleanup-pending reconciliation failed: " + err.Error()})
	}
}

func (b *buildAssembly) emitStartupNotices() {
	if ignored := (planmode.Policy{AllowedTools: b.cfg.Agent.PlanModeAllowedTools}).IgnoredAllowedTools(); len(ignored) > 0 {
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: fmt.Sprintf("plan_mode_allowed_tools ignored known blocked entries: %s; this setting only declares extra read-only custom tools and cannot unlock known blocked tools or unsafe bash", strings.Join(ignored, ", "))})
	}
	if b.migrationErr != nil {
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "legacy user-data migration failed: " + b.migrationErr.Error()})
	} else if b.migrated != nil {
		b.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: b.migrated.Notice()})
	}
	emitMemoryCompilerRetirementNotice(b.sink, b.memoryCompilerConfigMigrated, b.memoryCompilerConfigErr)
	migration.MigrateLegacyMemorySources(b.sink)
	migration.MigrateLegacySessionSources(b.sink)
	if !b.opts.RequireKey && b.entry.RequiresAPIKey() && b.entry.APIKey() == "" {
		b.sink.Emit(event.Event{Kind: event.Notice, Text: missingAPIKeyNotice(b.modelName, b.entry, b.credentialOwner, b.credentialPaths.UserCredentialsPath())})
	}
}

func (b *buildAssembly) initializeNetwork() error {
	b.proxySpec = b.cfg.OutboundProxySpec()
	if err := netclient.Validate(b.proxySpec); err != nil {
		return err
	}
	var err error
	b.balanceClient, err = netclient.NewHTTPClient(b.proxySpec, netclient.TransportOptions{})
	if err != nil {
		return err
	}
	b.execProvider, err = newProviderWithProxyForOwner(b.entry, b.proxySpec, b.credentialOwner)
	return err
}

func (b *buildAssembly) loadPrompt() error {
	if err := b.loadPromptSources(); err != nil {
		return err
	}
	if err := b.loadExperts(); err != nil {
		return err
	}
	var err error
	b.systemPrompt, b.expertSelection, err = b.buildSystemPrompt(b.opts.SessionExpertID, b.opts.ExpertID)
	return err
}

func (b *buildAssembly) loadPromptSources() error {
	var err error
	b.basePrompt, err = b.cfg.ResolveSystemPromptForRoot(b.root)
	if err != nil {
		return err
	}
	if style, ok := outputstyle.Resolve(b.cfg.Agent.OutputStyle, outputstyle.Dirs()); ok {
		b.basePrompt = outputstyle.Apply(b.basePrompt, style)
	}
	b.memory = memory.Load(memory.Options{CWD: b.root, UserDir: b.credentialPaths.MemoryUserDir(), ProjectStoreRoot: b.credentialPaths.MemoryProjectStoreRoot()})
	b.projectChecks = instruction.ExtractHostChecks(b.memory.Docs)
	home := b.credentialPaths.KWUHomeDir()
	common := skill.Options{KWUHomeDir: home, ProjectRoot: b.root, CustomPaths: b.cfg.SkillCustomPaths(), ExcludedPaths: b.cfg.SkillExcludedPaths(), MaxDepth: b.cfg.SkillMaxDepth()}
	common.DisabledNames = b.cfg.DisabledSkillNames()
	common.Stderr = b.opts.Stderr
	b.skillStore = skill.New(common)
	b.skills = b.skillStore.List()
	common.DisabledNames = nil
	common.Stderr = io.Discard
	b.allSkillStore = skill.New(common)
	b.allSkills = b.allSkillStore.List()
	return nil
}

func (b *buildAssembly) loadExperts() error {
	b.userExpertPath = b.opts.UserExpertPath
	if strings.TrimSpace(b.userExpertPath) == "" {
		b.userExpertPath = config.UserExpertsPath()
	}
	if b.opts.ExpertCatalogLoader == nil {
		b.expertStorePath = strings.TrimSpace(b.opts.ExpertStorePath)
		if b.expertStorePath == "" {
			if dir := strings.TrimSpace(config.MemoryUserDir()); dir != "" {
				b.expertStorePath = filepath.Join(dir, "experts.sqlite")
			}
		}
	}
	var err error
	b.expertCatalog, err = b.loadExpertCatalog()
	if err != nil {
		return err
	}
	if defaultExpert := strings.TrimSpace(b.cfg.Agent.DefaultExpert); defaultExpert != "" {
		if _, err := b.expertCatalog.Resolve(defaultExpert); err != nil {
			return fmt.Errorf("agent.default_expert: %w", err)
		}
	}
	return nil
}

func (b *buildAssembly) loadExpertCatalog() (*expert.Catalog, error) {
	if b.opts.ExpertCatalogLoader != nil {
		return b.opts.ExpertCatalogLoader()
	}
	if b.expertStorePath != "" {
		builtinExperts := expert.BuiltinDefinitions()
		if b.opts.DisableBuiltinExperts {
			builtinExperts = nil
		}
		store := expert.NewStore(b.expertStorePath, builtinExperts)
		return expert.LoadStoreCatalog(b.ctx, store, b.userExpertPath)
	}
	return expert.LoadCatalog(b.userExpertPath)
}

func appendDesktopProductIdentityPolicy(prompt, version string) string {
	if version == "" {
		return prompt
	}
	return prompt + "\n\n" + config.DesktopProductIdentityPolicy(version)
}

func (b *buildAssembly) buildSystemPrompt(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
	catalog, err := b.loadExpertCatalog()
	if err != nil {
		return "", expert.Selection{}, err
	}
	selection, err := expert.ResolveSelection(catalog, sessionExpertID, requestExpertID, b.cfg.Agent.DefaultExpert)
	if err != nil && strings.TrimSpace(sessionExpertID) != "" && strings.TrimSpace(requestExpertID) == "" {
		selection, err = expert.ResolveSelection(catalog, expert.ExplicitNoneID, "", b.cfg.Agent.DefaultExpert)
	}
	if err != nil {
		return "", selection, err
	}
	prompt := b.basePrompt
	if selection.Selected {
		prompt = expert.ApplyPrompt(prompt, selection.Expert)
	}
	prompt += "\n\n" + config.UserDecisionPolicy
	prompt += "\n\n" + config.ArtifactDeliveryPolicy
	prompt += "\n\n" + config.PersistentTaskDispatchPolicy
	prompt += "\n\n" + config.TemporalPolicy
	prompt += "\n\n" + config.LanguagePolicy
	if b.entry != nil {
		prompt += "\n\n" + config.ModelIdentityPolicy(b.entry.Model)
	}
	if b.tokenEconomy {
		prompt += "\n\n" + tokenEconomyPrompt
	}
	prompt = memory.Compose(prompt, b.memory)
	if !b.tokenEconomy {
		prompt = skill.ApplyIndex(prompt, b.skills)
	}
	prompt = appendDesktopProductIdentityPolicy(prompt, b.opts.DesktopVersion)
	return prompt, selection, nil
}
