package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/brand"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

// settings_app.go is the desktop Settings panel's command surface: it reads the
// resolved config and applies edits through internal/config/edit.go (the
// purpose-built mutation API), then rebuilds the controller so the change takes
// effect live — the same snapshot→reload→resume pattern as SetModel. Secrets are
// the exception: they go to the current profile's credentials file, since config
// stores only the env-var name, not the key.

// --- read ---

type ProviderView struct {
	Name              string   `json:"name"`
	BuiltIn           bool     `json:"builtIn"`
	Added             bool     `json:"added"`
	Kind              string   `json:"kind"`
	BaseURL           string   `json:"baseUrl"`
	Models            []string `json:"models"`
	ChatModels        []string `json:"chatModels"`
	VisionModels      []string `json:"visionModels"`
	VisionModelsSet   bool     `json:"visionModelsConfigured"`
	ModelsURL         string   `json:"modelsUrl"`
	Default           string   `json:"default"`
	APIKeyEnv         string   `json:"apiKeyEnv"`
	KeySet            bool     `json:"keySet"` // the env var currently resolves to a non-empty value
	RequiresKey       bool     `json:"requiresKey"`
	Configured        bool     `json:"configured"` // selectable: either key is present or no key is required
	KeySource         string   `json:"keySource,omitempty"`
	KeySourcePath     string   `json:"keySourcePath,omitempty"`
	BalanceURL        string   `json:"balanceUrl"`
	ContextWindow     int      `json:"contextWindow"`
	ReasoningProtocol string   `json:"reasoningProtocol"`
	SupportedEfforts  []string `json:"supportedEfforts"`
	DefaultEffort     string   `json:"defaultEffort"`
	BillingSource     string   `json:"billingSource"`
	Locked            bool     `json:"locked"`
	Availability      string   `json:"availability,omitempty"`
}

type PermissionsView struct {
	Mode  string   `json:"mode"`
	Allow []string `json:"allow"`
	Ask   []string `json:"ask"`
	Deny  []string `json:"deny"`
}

type SandboxView struct {
	Bash          string   `json:"bash"`
	Network       bool     `json:"network"`
	WorkspaceRoot string   `json:"workspaceRoot"`
	AllowWrite    []string `json:"allowWrite"`
	Shell         string   `json:"shell"` // [tools.shell] prefer: auto|bash|powershell|pwsh
}

type AgentView struct {
	Temperature       float64 `json:"temperature"`
	MaxSteps          int     `json:"maxSteps"`
	PlannerMaxSteps   int     `json:"plannerMaxSteps"`
	SystemPrompt      string  `json:"systemPrompt"`
	ColdResumePrune   bool    `json:"coldResumePrune"`
	ReasoningLanguage string  `json:"reasoningLanguage"`
}

type BotAllowlistView struct {
	Enabled      bool     `json:"enabled"`
	AllowAll     bool     `json:"allowAll"`
	QQUsers      []string `json:"qqUsers"`
	FeishuUsers  []string `json:"feishuUsers"`
	WeixinUsers  []string `json:"weixinUsers"`
	QQGroups     []string `json:"qqGroups"`
	FeishuGroups []string `json:"feishuGroups"`
	WeixinGroups []string `json:"weixinGroups"`
}

type QQBotView struct {
	Enabled      bool   `json:"enabled"`
	AppID        string `json:"appId"`
	AppSecretEnv string `json:"appSecretEnv"`
	SecretSet    bool   `json:"secretSet"`
	Sandbox      bool   `json:"sandbox"`
}

type FeishuBotView struct {
	Enabled           bool   `json:"enabled"`
	Domain            string `json:"domain"`
	AppID             string `json:"appId"`
	AppSecretEnv      string `json:"appSecretEnv"`
	SecretSet         bool   `json:"secretSet"`
	VerificationToken string `json:"verificationToken"`
	Mode              string `json:"mode"`
	WebhookPort       int    `json:"webhookPort"`
	RequireMention    bool   `json:"requireMention"`
}

type WeixinBotView struct {
	Enabled   bool   `json:"enabled"`
	AccountID string `json:"accountId"`
	TokenEnv  string `json:"tokenEnv"`
	TokenSet  bool   `json:"tokenSet"`
	APIBase   string `json:"apiBase"`
}

type BotSettingsView struct {
	Enabled          bool                `json:"enabled"`
	Model            string              `json:"model"`
	ToolApprovalMode string              `json:"toolApprovalMode"`
	MaxSteps         int                 `json:"maxSteps"`
	DebounceMs       int                 `json:"debounceMs"`
	Allowlist        BotAllowlistView    `json:"allowlist"`
	QQ               QQBotView           `json:"qq"`
	Feishu           FeishuBotView       `json:"feishu"`
	Weixin           WeixinBotView       `json:"weixin"`
	Connections      []BotConnectionView `json:"connections"`
}

type SelectedModelSummaryView struct {
	Ref           string `json:"ref"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	DisplayName   string `json:"displayName"`
	BillingSource string `json:"billingSource"`
	Availability  string `json:"availability"`
	AccountState  string `json:"accountState"`
}

// SettingsCoreView excludes provider catalogs and credential state so the
// Settings shell can render without waiting for keychain or environment reads.
type SettingsCoreView struct {
	DefaultModel            string                   `json:"defaultModel"`
	DefaultBillingSource    string                   `json:"defaultBillingSource"`
	SelectedModelSummary    SelectedModelSummaryView `json:"selectedModelSummary"`
	PlannerModel            string                   `json:"plannerModel"`
	SubagentModel           string                   `json:"subagentModel"`
	SubagentEffort          string                   `json:"subagentEffort"`
	AutoPlan                string                   `json:"autoPlan"`
	Permissions             PermissionsView          `json:"permissions"`
	Sandbox                 SandboxView              `json:"sandbox"`
	Agent                   AgentView                `json:"agent"`
	Bot                     BotSettingsView          `json:"bot"`
	DesktopLanguage         string                   `json:"desktopLanguage"`
	DesktopLayoutStyle      string                   `json:"desktopLayoutStyle"`
	DesktopTheme            string                   `json:"desktopTheme"`
	DesktopThemeStyle       string                   `json:"desktopThemeStyle"`
	CloseBehavior           string                   `json:"closeBehavior"`
	DisplayMode             string                   `json:"displayMode"`
	StatusBarStyle          string                   `json:"statusBarStyle"`
	StatusBarItems          []string                 `json:"statusBarItems"`
	DefaultToolApprovalMode string                   `json:"defaultToolApprovalMode"`
	CheckUpdates            bool                     `json:"checkUpdates"`
	Telemetry               bool                     `json:"telemetry"`
	Metrics                 bool                     `json:"metrics"`
	ExpandThinking          bool                     `json:"expandThinking"`
	ConfigPath              string                   `json:"configPath"`
	AutoApproveTools        bool                     `json:"autoApproveTools"`
	Bypass                  bool                     `json:"bypass"`
}

type SettingsModelDetailsView struct {
	Providers         []ProviderView `json:"providers"`
	OfficialProviders []ProviderView `json:"officialProviders"`
	ProviderKinds     []string       `json:"providerKinds"`
}

// SettingsView is the whole Settings panel payload.
type SettingsView struct {
	DefaultModel            string                   `json:"defaultModel"`
	DefaultBillingSource    string                   `json:"defaultBillingSource"`
	SelectedModelSummary    SelectedModelSummaryView `json:"selectedModelSummary"`
	PlannerModel            string                   `json:"plannerModel"`
	SubagentModel           string                   `json:"subagentModel"`
	SubagentEffort          string                   `json:"subagentEffort"`
	AutoPlan                string                   `json:"autoPlan"`
	Providers               []ProviderView           `json:"providers"`
	OfficialProviders       []ProviderView           `json:"officialProviders"`
	Permissions             PermissionsView          `json:"permissions"`
	Sandbox                 SandboxView              `json:"sandbox"`
	Agent                   AgentView                `json:"agent"`
	Bot                     BotSettingsView          `json:"bot"`
	DesktopLanguage         string                   `json:"desktopLanguage"`
	DesktopLayoutStyle      string                   `json:"desktopLayoutStyle"`
	DesktopTheme            string                   `json:"desktopTheme"`
	DesktopThemeStyle       string                   `json:"desktopThemeStyle"`
	CloseBehavior           string                   `json:"closeBehavior"`
	DisplayMode             string                   `json:"displayMode"`
	StatusBarStyle          string                   `json:"statusBarStyle"`
	StatusBarItems          []string                 `json:"statusBarItems"`
	DefaultToolApprovalMode string                   `json:"defaultToolApprovalMode"`
	CheckUpdates            bool                     `json:"checkUpdates"`
	Telemetry               bool                     `json:"telemetry"`
	Metrics                 bool                     `json:"metrics"`
	ExpandThinking          bool                     `json:"expandThinking"`
	ConfigPath              string                   `json:"configPath"`
	// ProviderKinds lists the provider implementations the kernel actually
	// registered (provider.Kinds()), so the editor's "kind" picker offers only
	// kinds that resolve — selecting an unregistered one would fail the rebuild.
	ProviderKinds    []string `json:"providerKinds"`
	AutoApproveTools bool     `json:"autoApproveTools"`
	Bypass           bool     `json:"bypass"`
}

// DesktopStartupSettingsView is the lightweight Settings subset needed during
// frontend startup. It deliberately excludes providers and credential state so
// slow keychain/env resolution stays off the first-render path.
type DesktopStartupSettingsView struct {
	Bot                BotSettingsView `json:"bot"`
	DesktopLanguage    string          `json:"desktopLanguage"`
	DesktopLayoutStyle string          `json:"desktopLayoutStyle"`
	DesktopTheme       string          `json:"desktopTheme"`
	DesktopThemeStyle  string          `json:"desktopThemeStyle"`
	DisplayMode        string          `json:"displayMode"`
	StatusBarStyle     string          `json:"statusBarStyle"`
	StatusBarItems     []string        `json:"statusBarItems"`
	CheckUpdates       bool            `json:"checkUpdates"`
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func providerRemovalFallbackRef(c *config.Config, name string) string {
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.Name == name || !p.Configured() || len(p.ModelList()) == 0 {
			continue
		}
		return p.Name + "/" + p.DefaultModel()
	}
	return ""
}

func providerRemovalFallbackRoute(c *config.Config, name string) (string, modelbilling.BillingSource) {
	if c == nil {
		return "", ""
	}
	source := desktopSettingsDefaultBillingSource(c)
	if !desktopModelRefsProvider(c, c.DefaultModel, name) {
		if source == modelbilling.BillingSubscriptionPool &&
			strings.HasPrefix(strings.TrimSpace(c.DefaultModel), brand.ProviderID+"/") {
			return strings.TrimSpace(c.DefaultModel), source
		}
		if entry, ok := c.ResolveModel(c.DefaultModel); ok &&
			(source == modelbilling.BillingSubscriptionPool || entry.Configured()) {
			return entry.Name + "/" + entry.Model, source
		}
	}
	ref := providerRemovalFallbackRef(c, name)
	if strings.HasPrefix(ref, brand.ProviderID+"/") {
		return ref, modelbilling.BillingSubscriptionPool
	}
	if ref != "" {
		return ref, modelbilling.BillingNewAPIByok
	}
	return "", ""
}

func desktopModelRefsProvider(c *config.Config, ref, name string) bool {
	if config.ModelRefsProvider(ref, name) {
		return true
	}
	if e, ok := c.ResolveModel(ref); ok {
		return e.Name == name
	}
	return false
}

func officialProviderHost(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func officialProviderKindFromEntry(p config.ProviderEntry) string {
	if config.CanonicalDesktopOfficialProviderName(p.Name) == brand.ProviderID {
		if config.IsOfficialKWUProvider(&p) {
			return brand.ProviderID
		}
		return ""
	}
	if config.CanonicalDesktopOfficialProviderName(p.Name) == "deepseek" && officialProviderHost(p.BaseURL) == "api.deepseek.com" {
		return "deepseek"
	}
	return ""
}

func isOfficialBuiltInProvider(p config.ProviderEntry) bool {
	return officialProviderKindFromEntry(p) != ""
}

func providerAccessSet(names []string) map[string]bool {
	out := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == brand.LegacyProviderID {
			name = brand.ProviderID
		}
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func addProviderAccess(c *config.Config, names ...string) {
	seen := providerAccessSet(c.Desktop.ProviderAccess)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == brand.LegacyProviderID {
			name = brand.ProviderID
		}
		if name == "" || seen[name] {
			continue
		}
		c.Desktop.ProviderAccess = append(c.Desktop.ProviderAccess, name)
		seen[name] = true
	}
}

func removeProviderAccess(c *config.Config, names ...string) {
	remove := providerAccessSet(names)
	if len(remove) == 0 {
		return
	}
	out := c.Desktop.ProviderAccess[:0]
	for _, name := range c.Desktop.ProviderAccess {
		if !remove[name] {
			out = append(out, name)
		}
	}
	c.Desktop.ProviderAccess = out
}

func providerViewFromEntry(p config.ProviderEntry, builtIn, added bool) ProviderView {
	return providerViewFromEntryForRoot(p, builtIn, added, ".")
}

func providerViewFromEntryForRoot(p config.ProviderEntry, builtIn, added bool, root string) ProviderView {
	return providerViewFromEntryForRootWithResolver(p, builtIn, added, root, nil)
}

func providerViewFromEntryForRootWithResolver(p config.ProviderEntry, builtIn, added bool, root string, resolver *config.CredentialResolver) ProviderView {
	return providerViewFromEntryForRootWithCredentialLookup(p, builtIn, added, root, resolver, true)
}

func providerViewFromEntryForRootWithCredentialLookup(p config.ProviderEntry, builtIn, added bool, root string, resolver *config.CredentialResolver, lookupCredential bool) ProviderView {
	// Settings shows the provider's raw model catalog. The chat picker follows
	// the same catalog and lets the submit path fail fast for incompatible model
	// families.
	models := p.ModelList()
	visionModels := p.VisionModels
	visionModelsSet := p.Vision || p.VisionModels != nil
	if p.Vision {
		visionModels = models
	}
	var key config.CredentialResolution
	if lookupCredential {
		if resolver == nil {
			resolver = config.NewCredentialResolverForRoot(root)
		}
		key = resolver.ResolveGlobalFirst(p.APIKeyEnv)
	}
	requiresKey := p.RequiresAPIKey()
	baseURL := p.BaseURL
	if builtIn && config.IsOfficialKWUProvider(&p) {
		baseURL = ""
	}
	return ProviderView{
		Name: p.Name, BuiltIn: builtIn, Added: added, Kind: p.Kind, BaseURL: baseURL,
		Models: nonNil(models), ChatModels: nonNil(p.ChatModelList()),
		VisionModels: nonNil(providerVisionModels(p.ChatModelList(), visionModels)), VisionModelsSet: visionModelsSet, ModelsURL: p.ModelsURL, Default: p.DefaultModel(),
		APIKeyEnv:         p.APIKeyEnv,
		KeySet:            key.Set,
		RequiresKey:       requiresKey,
		Configured:        !requiresKey || (lookupCredential && key.Set),
		KeySource:         key.Source.Label,
		KeySourcePath:     key.Source.Path,
		BalanceURL:        p.BalanceURL,
		ContextWindow:     p.ContextWindow,
		ReasoningProtocol: p.ReasoningProtocol,
		SupportedEfforts:  nonNil(p.SupportedEfforts),
		DefaultEffort:     p.DefaultEffort,
	}
}

func officialProviderViews(added map[string]bool, pricingLanguage string) []ProviderView {
	return officialProviderViewsForRoot(added, pricingLanguage, ".")
}

func officialProviderViewsForRoot(added map[string]bool, pricingLanguage, root string) []ProviderView {
	return officialProviderViewsForRootWithResolver(added, pricingLanguage, root, nil)
}

func officialProviderViewsForRootWithResolver(added map[string]bool, pricingLanguage, root string, resolver *config.CredentialResolver) []ProviderView {
	return officialProviderViewsForRootWithCredentialLookup(added, pricingLanguage, root, resolver, true)
}

func officialProviderViewsForRootWithCredentialLookup(added map[string]bool, pricingLanguage, root string, resolver *config.CredentialResolver, lookupCredential bool) []ProviderView {
	var out []ProviderView
	if lookupCredential && resolver == nil {
		resolver = config.NewCredentialResolverForRoot(root)
	}
	for _, kind := range []string{brand.ProviderID} {
		entries, _, err := officialProviderTemplate(kind, pricingLanguage)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			out = append(out, providerViewFromEntryForRootWithCredentialLookup(entry, true, added[entry.Name], root, resolver, lookupCredential))
		}
	}
	return out
}

func officialProviderAddedSet(cfg *config.Config) map[string]bool {
	out := map[string]bool{}
	if cfg == nil {
		return out
	}
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	for i := range cfg.Providers {
		p := cfg.Providers[i]
		name := config.CanonicalDesktopOfficialProviderName(p.Name)
		if !access[p.Name] && !access[name] {
			continue
		}
		if kind := officialProviderKindFromEntry(p); kind != "" {
			out[kind] = true
		}
	}
	return out
}

func desktopStartupSettingsFromConfig(cfg *config.Config) DesktopStartupSettingsView {
	if cfg == nil {
		return DesktopStartupSettingsView{
			Bot:                botSettingsView(config.BotConfig{}),
			DesktopLayoutStyle: "creation",
			DesktopTheme:       "light",
			DesktopThemeStyle:  "",
			DisplayMode:        "standard",
			StatusBarStyle:     "text",
			StatusBarItems:     config.DefaultDesktopStatusBarItems(),
			CheckUpdates:       true,
		}
	}
	return DesktopStartupSettingsView{
		Bot:                botSettingsView(cfg.Bot),
		DesktopLanguage:    cfg.DesktopLanguage(),
		DesktopLayoutStyle: cfg.DesktopLayoutStyle(),
		DesktopTheme:       cfg.DesktopTheme(),
		DesktopThemeStyle:  cfg.DesktopThemeStyle(),
		DisplayMode:        cfg.DesktopDisplayMode(),
		StatusBarStyle:     cfg.DesktopStatusBarStyle(),
		StatusBarItems:     cfg.DesktopStatusBarItems(),
		CheckUpdates:       cfg.DesktopCheckUpdates(),
	}
}

// DesktopStartupSettings returns only the desktop chrome preferences needed at
// app startup. Keep provider/key status in Settings(), where the Settings panel
// actually needs it.
func (a *App) DesktopStartupSettings() DesktopStartupSettingsView {
	paths := a.activeProfilePathResolver()
	out, err := a.desktopStartupSettingsWithPaths(paths, a.activeWorkspaceRoot())
	if err != nil {
		return desktopStartupSettingsFromConfig(nil)
	}
	return out
}

func (a *App) desktopStartupSettingsWithPaths(paths config.ProfilePathResolver, workspaceRoot string) (DesktopStartupSettingsView, error) {
	cfg, path, err := a.loadDesktopUserConfigForViewWithPaths(paths, workspaceRoot)
	if err != nil {
		return DesktopStartupSettingsView{}, err
	}
	if strings.TrimSpace(cfg.Desktop.ThemeStyle) != "" {
		cfg.Desktop.ThemeStyle = ""
		if err := cfg.SaveToScope(path, config.RenderScopeUser); err != nil {
			return DesktopStartupSettingsView{}, err
		}
	}
	out := desktopStartupSettingsFromConfig(cfg)
	out.Bot = botSettingsViewWithPaths(cfg.Bot, paths)
	return out, nil
}

func defaultSettingsCoreView() SettingsCoreView {
	return SettingsCoreView{
		SelectedModelSummary: SelectedModelSummaryView{Availability: "unavailable", AccountState: "unknown"},
		Permissions: PermissionsView{
			Mode:  "ask",
			Allow: []string{},
			Ask:   []string{},
			Deny:  []string{},
		},
		Sandbox:                 SandboxView{Bash: "enforce", AllowWrite: []string{}, Shell: "auto"},
		Agent:                   AgentView{PlannerMaxSteps: 0, ColdResumePrune: true, ReasoningLanguage: "auto"},
		Bot:                     botSettingsView(config.BotConfig{}),
		AutoPlan:                "off",
		DesktopLayoutStyle:      "creation",
		DesktopTheme:            "light",
		DesktopThemeStyle:       "",
		CloseBehavior:           "background",
		DisplayMode:             "standard",
		StatusBarStyle:          "text",
		StatusBarItems:          config.DefaultDesktopStatusBarItems(),
		DefaultToolApprovalMode: "ask",
		CheckUpdates:            true,
	}
}

func (a *App) selectedModelSummary(cfg *config.Config, actor userctx.ActorContext) SelectedModelSummaryView {
	if cfg == nil {
		return SelectedModelSummaryView{Availability: "unavailable", AccountState: "unknown"}
	}
	ref := strings.TrimSpace(cfg.DefaultModel)
	providerName, modelName := "", ref
	if entry, ok := cfg.ResolveModel(ref); ok {
		providerName, modelName = entry.Name, entry.Model
		ref = entry.Name + "/" + entry.Model
	} else if separator := strings.Index(ref, "/"); separator > 0 {
		providerName, modelName = ref[:separator], ref[separator+1:]
	}
	source := desktopSettingsDefaultBillingSource(cfg)
	summary := SelectedModelSummaryView{
		Ref:           ref,
		Provider:      providerName,
		Model:         modelName,
		DisplayName:   modelName,
		BillingSource: string(source),
		Availability:  "unverified",
		AccountState:  "not_applicable",
	}
	if ref == "" {
		summary.Availability = "unavailable"
		return summary
	}
	if source != modelbilling.BillingSubscriptionPool {
		return summary
	}
	summary.AccountState = "signed_out"
	summary.Availability = "unavailable"
	if desktopCloudFakeEnabled() {
		summary.AccountState = "ready"
		summary.Availability = "ready"
		return summary
	}
	if a == nil || a.subscriptionModels == nil {
		return summary
	}
	binding, models, authorized := a.desktopAuthorizedSubscriptionCatalogForActor(actor)
	if !authorized {
		return summary
	}
	state, _ := a.subscriptionModels.state(binding)
	summary.AccountState = state
	if state != string(cloud.MaaSKeyReady) {
		return summary
	}
	summary.Availability = "unavailable"
	for _, model := range models {
		if model == modelName || brand.ProviderID+"/"+model == ref {
			summary.Availability = "ready"
			break
		}
	}
	return summary
}

func (a *App) settingsCoreViewFromConfig(cfg *config.Config, cfgPath string, actor userctx.ActorContext) SettingsCoreView {
	if cfg == nil {
		return defaultSettingsCoreView()
	}
	defaultToolApprovalMode := cfg.DesktopDefaultToolApprovalMode()
	preferenceStore := newDesktopTabStore(desktopConfigDirForPaths(config.NewProfilePathResolver(actor)))
	if persistedMode, ok, err := preferenceStore.defaultToolApprovalMode(); err != nil {
		slog.Warn("load persisted desktop tool approval mode for settings failed", "path", preferenceStore.path(), "err", err)
	} else if ok {
		defaultToolApprovalMode = persistedMode
	}
	ctrl := a.activeCtrl()
	bash := cfg.Sandbox.Bash
	if bash == "" {
		bash = "enforce"
	}
	shell := cfg.Tools.Shell.Prefer
	if shell == "" {
		shell = "auto"
	}
	return SettingsCoreView{
		DefaultModel:         cfg.DefaultModel,
		DefaultBillingSource: string(desktopSettingsDefaultBillingSource(cfg)),
		SelectedModelSummary: a.selectedModelSummary(cfg, actor),
		PlannerModel:         cfg.Agent.PlannerModel,
		SubagentModel:        cfg.Agent.SubagentModel,
		SubagentEffort:       cfg.Agent.SubagentEffort,
		AutoPlan:             desktopAutoPlanMode(cfg.Agent.AutoPlan),
		Permissions: PermissionsView{
			Mode:  orDefault(cfg.Permissions.Mode, "ask"),
			Allow: nonNil(cfg.Permissions.Allow),
			Ask:   nonNil(cfg.Permissions.Ask),
			Deny:  nonNil(cfg.Permissions.Deny),
		},
		Sandbox: SandboxView{
			Bash: bash, Network: cfg.Sandbox.Network,
			WorkspaceRoot: cfg.Sandbox.WorkspaceRoot, AllowWrite: nonNil(cfg.Sandbox.AllowWrite),
			Shell: shell,
		},
		Agent:                   AgentView{Temperature: cfg.Agent.Temperature, MaxSteps: cfg.Agent.MaxSteps, PlannerMaxSteps: cfg.Agent.PlannerMaxSteps, SystemPrompt: cfg.Agent.SystemPrompt, ColdResumePrune: cfg.ColdResumePruneEnabled(), ReasoningLanguage: cfg.ReasoningLanguage()},
		Bot:                     botSettingsViewWithPaths(cfg.Bot, a.activeProfilePathResolver()),
		DesktopLanguage:         cfg.DesktopLanguage(),
		DesktopLayoutStyle:      cfg.DesktopLayoutStyle(),
		DesktopTheme:            cfg.DesktopTheme(),
		DesktopThemeStyle:       cfg.DesktopThemeStyle(),
		CloseBehavior:           cfg.DesktopCloseBehavior(),
		DisplayMode:             cfg.DesktopDisplayMode(),
		StatusBarStyle:          cfg.DesktopStatusBarStyle(),
		StatusBarItems:          cfg.DesktopStatusBarItems(),
		DefaultToolApprovalMode: defaultToolApprovalMode,
		CheckUpdates:            cfg.DesktopCheckUpdates(),
		Telemetry:               cfg.DesktopTelemetry() && reportEndpointAvailable(pingEndpoint),
		Metrics:                 cfg.DesktopMetrics() && reportEndpointAvailable(metricsEndpoint),
		ExpandThinking:          cfg.Desktop.ExpandThinking,
		ConfigPath:              cfgPath,
		AutoApproveTools:        ctrl != nil && ctrl.AutoApproveTools(),
		Bypass:                  ctrl != nil && ctrl.AutoApproveTools(),
	}
}

func (a *App) settingsModelDetailsFromConfig(cfg *config.Config, root string, actor userctx.ActorContext) SettingsModelDetailsView {
	v := SettingsModelDetailsView{
		Providers:         []ProviderView{},
		OfficialProviders: []ProviderView{},
		ProviderKinds:     nonNil(provider.Kinds()),
	}
	added := map[string]bool{brand.ProviderID: true}
	pricingLanguage := ""
	if cfg != nil {
		added = providerAccessSet(cfg.Desktop.ProviderAccess)
		added[brand.ProviderID] = true
		pricingLanguage = cfg.DeepSeekOfficialPricingLanguage()
	}
	lookupProviderCredentials := a.desktopProviderCredentialsAllowedForActiveRoute(root)
	var resolver *config.CredentialResolver
	if lookupProviderCredentials {
		resolver = config.NewCredentialResolverForRootWithPaths(root, config.NewProfilePathResolver(actor))
	}
	v.OfficialProviders = officialProviderViewsForRootWithCredentialLookup(map[string]bool{brand.ProviderID: true}, pricingLanguage, root, resolver, lookupProviderCredentials)
	for i := range v.OfficialProviders {
		v.OfficialProviders[i] = a.decorateDesktopProviderView(v.OfficialProviders[i], actor)
	}
	if cfg == nil {
		return v
	}
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if config.IsOfficialKWUProvider(p) {
			continue
		}
		view := providerViewFromEntryForRootWithCredentialLookup(*p, isOfficialBuiltInProvider(*p), added[p.Name], root, resolver, lookupProviderCredentials)
		if !lookupProviderCredentials {
			applyStoredCredentialPresence(&view, *p, config.NewProfilePathResolver(actor))
		}
		v.Providers = append(v.Providers, a.decorateDesktopProviderView(view, actor))
	}
	return v
}

// SettingsCore returns the fast Settings shell payload without provider or
// credential lookup.
func (a *App) SettingsCore() SettingsCoreView {
	cfg, cfgPath, err := a.loadDesktopUserConfigForView()
	if err != nil {
		return defaultSettingsCoreView()
	}
	_, actor, _ := a.desktopActiveRouteSnapshot()
	return a.settingsCoreViewFromConfig(cfg, cfgPath, actor)
}

// SettingsModelDetails returns provider catalogs and credential state on demand.
func (a *App) SettingsModelDetails() SettingsModelDetailsView {
	cfg, _, err := a.loadDesktopUserConfigForView()
	root, actor, _ := a.desktopActiveRouteSnapshot()
	if err != nil {
		cfg = nil
	}
	return a.settingsModelDetailsFromConfig(cfg, root, actor)
}

// Settings returns the current configuration for compatibility with older
// clients. It reads config once, then composes the same core and details views.
func (a *App) Settings() SettingsView {
	cfg, cfgPath, err := a.loadDesktopUserConfigForView()
	root, actor, _ := a.desktopActiveRouteSnapshot()
	if err != nil {
		cfg = nil
	}
	core := a.settingsCoreViewFromConfig(cfg, cfgPath, actor)
	details := a.settingsModelDetailsFromConfig(cfg, root, actor)
	return SettingsView{
		DefaultModel:            core.DefaultModel,
		DefaultBillingSource:    core.DefaultBillingSource,
		SelectedModelSummary:    core.SelectedModelSummary,
		PlannerModel:            core.PlannerModel,
		SubagentModel:           core.SubagentModel,
		SubagentEffort:          core.SubagentEffort,
		AutoPlan:                core.AutoPlan,
		Providers:               details.Providers,
		OfficialProviders:       details.OfficialProviders,
		Permissions:             core.Permissions,
		Sandbox:                 core.Sandbox,
		Agent:                   core.Agent,
		Bot:                     core.Bot,
		DesktopLanguage:         core.DesktopLanguage,
		DesktopLayoutStyle:      core.DesktopLayoutStyle,
		DesktopTheme:            core.DesktopTheme,
		DesktopThemeStyle:       core.DesktopThemeStyle,
		CloseBehavior:           core.CloseBehavior,
		DisplayMode:             core.DisplayMode,
		StatusBarStyle:          core.StatusBarStyle,
		StatusBarItems:          core.StatusBarItems,
		DefaultToolApprovalMode: core.DefaultToolApprovalMode,
		CheckUpdates:            core.CheckUpdates,
		Telemetry:               core.Telemetry,
		Metrics:                 core.Metrics,
		ExpandThinking:          core.ExpandThinking,
		ConfigPath:              core.ConfigPath,
		ProviderKinds:           details.ProviderKinds,
		AutoApproveTools:        core.AutoApproveTools,
		Bypass:                  core.Bypass,
	}
}

func applyStoredCredentialPresence(view *ProviderView, provider config.ProviderEntry, paths config.ProfilePathResolver) {
	if view == nil || !provider.RequiresAPIKey() {
		return
	}
	view.KeySet = config.CredentialStoredWithPaths(paths, provider.APIKeyEnv)
	view.Configured = view.KeySet
	if !view.KeySet && view.Added {
		view.Availability = "credential_unverified"
	}
}

func (a *App) decorateDesktopProviderView(view ProviderView, actor userctx.ActorContext) ProviderView {
	if !view.BuiltIn || view.Name != brand.ProviderID {
		view.BillingSource = string(modelbilling.BillingNewAPIByok)
		return view
	}
	view.BaseURL = ""
	view.BuiltIn = true
	view.Added = true
	view.Locked = true
	view.BillingSource = string(modelbilling.BillingSubscriptionPool)
	view.APIKeyEnv = ""
	view.KeySource = ""
	view.KeySourcePath = ""
	view.RequiresKey = false
	view.BalanceURL = ""
	view.KeySet = false
	view.Configured = false
	view.Models = nil
	view.ChatModels = nil
	view.Default = ""
	view.Availability = "signed_out"
	if desktopCloudFakeEnabled() {
		view.Models = append([]string(nil), desktopAirClaudeSubscriptionModels...)
		view.ChatModels = append([]string(nil), view.Models...)
		view.Default = view.Models[0]
		view.KeySet = true
		view.Configured = true
		view.Availability = "ready"
		return view
	}
	if a == nil || a.billingWorkerRuntime == nil || a.subscriptionModels == nil {
		return view
	}
	binding, models, authorized := a.desktopAuthorizedSubscriptionCatalogForActor(actor)
	if !authorized {
		return view
	}
	state, _ := a.subscriptionModels.state(binding)
	view.Availability = state
	if state != string(cloud.MaaSKeyReady) || len(models) == 0 {
		return view
	}
	view.Models = models
	view.ChatModels = append([]string(nil), models...)
	view.Default = models[0]
	view.KeySet = true
	view.Configured = true
	return view
}

func botSettingsView(b config.BotConfig) BotSettingsView {
	return botSettingsViewWithPaths(b, config.ProfilePathResolver{})
}

func botSettingsViewWithPaths(b config.BotConfig, paths config.ProfilePathResolver) BotSettingsView {
	mode := strings.TrimSpace(b.Feishu.Mode)
	if mode == "" {
		mode = "webhook"
	}
	secretSet := func(envName string) bool {
		if paths.ProfileID() != "" {
			return config.CredentialStoredWithPaths(paths, envName)
		}
		return envIsSet(envName)
	}
	return BotSettingsView{
		Enabled:          b.Enabled,
		Model:            b.Model,
		ToolApprovalMode: normalizeBotConnectionToolApprovalMode(b.ToolApprovalMode),
		MaxSteps:         b.MaxSteps,
		DebounceMs:       b.DebounceMs,
		Allowlist: BotAllowlistView{
			Enabled:      b.Allowlist.Enabled,
			AllowAll:     b.Allowlist.AllowAll,
			QQUsers:      nonNil(b.Allowlist.QQUsers),
			FeishuUsers:  nonNil(b.Allowlist.FeishuUsers),
			WeixinUsers:  nonNil(b.Allowlist.WeixinUsers),
			QQGroups:     nonNil(b.Allowlist.QQGroups),
			FeishuGroups: nonNil(b.Allowlist.FeishuGroups),
			WeixinGroups: nonNil(b.Allowlist.WeixinGroups),
		},
		QQ: QQBotView{
			Enabled:      b.QQ.Enabled,
			AppID:        b.QQ.AppID,
			AppSecretEnv: b.QQ.AppSecretEnv,
			SecretSet:    secretSet(b.QQ.AppSecretEnv),
			Sandbox:      b.QQ.Sandbox,
		},
		Feishu: FeishuBotView{
			Enabled:           b.Feishu.Enabled,
			Domain:            orDefault(strings.TrimSpace(b.Feishu.Domain), "feishu"),
			AppID:             b.Feishu.AppID,
			AppSecretEnv:      b.Feishu.AppSecretEnv,
			SecretSet:         secretSet(b.Feishu.AppSecretEnv),
			VerificationToken: b.Feishu.VerificationToken,
			Mode:              mode,
			WebhookPort:       b.Feishu.WebhookPort,
			RequireMention:    b.Feishu.RequireMention,
		},
		Weixin: WeixinBotView{
			Enabled:   b.Weixin.Enabled,
			AccountID: b.Weixin.AccountID,
			TokenEnv:  b.Weixin.TokenEnv,
			TokenSet:  secretSet(b.Weixin.TokenEnv),
			APIBase:   b.Weixin.APIBase,
		},
		Connections: botConnectionViewsWithPaths(b.Connections, paths),
	}
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func botDomainOrDefault(domain string) string {
	if strings.EqualFold(strings.TrimSpace(domain), "lark") {
		return "lark"
	}
	return "feishu"
}

// --- apply (write config, then rebuild the controller so it's live) ---

// applyConfigChange mutates the user-global config and rebuilds the controller so
// the change takes effect this session. Desktop settings such as providers and
// keys are account-level, not per-project: writing them to the global config
// rather than the cwd's kwu.toml is what lets them survive a workspace switch.
func (a *App) applyConfigChange(mutate func(*config.Config) error) error {
	if err := a.ensureActiveTabRebuildAllowed("settings"); err != nil {
		return err
	}
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	if err := cfg.SaveToScope(path, config.RenderScopeUser); err != nil {
		return err
	}
	return a.rebuild()
}

func (a *App) applyConfigOnly(mutate func(*config.Config) error) error {
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	return cfg.SaveToScope(path, config.RenderScopeUser)
}

func (a *App) applyConfigOnlyWithPaths(paths config.ProfilePathResolver, workspaceRoot string, mutate func(*config.Config) error) error {
	cfg, path, err := a.loadDesktopUserConfigForEditWithPaths(paths, workspaceRoot)
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	return cfg.SaveToScope(path, config.RenderScopeUser)
}

func (a *App) ensureActiveTabRebuildAllowed(setting string) error {
	if a.ctx == nil {
		return nil
	}
	tab := a.activeTab()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError(setting)
	}
	return nil
}

func (a *App) ensureLiveControllersRuntimeMutationAllowed(setting string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, tab := range a.tabRuntimes {
		if tab == nil {
			continue
		}
		if controllerHasActiveRuntimeWork(tab.Ctrl) {
			return rebuildControllerActiveWorkError(setting)
		}
	}
	return nil
}

func (a *App) loadDesktopUserConfigForEdit() (*config.Config, string, error) {
	paths := a.activeProfilePathResolver()
	return a.loadDesktopUserConfigForEditWithPaths(paths, a.activeWorkspaceRoot())
}

func (a *App) loadDesktopUserConfigForEditWithPaths(paths config.ProfilePathResolver, workspaceRoot string) (*config.Config, string, error) {
	if !a.desktopProviderCredentialsAllowedForActiveRoute(workspaceRoot) {
		return a.loadDesktopUserConfigForViewWithPaths(paths, workspaceRoot)
	}
	userPath := paths.UserConfigPath()
	if userPath == "" {
		return nil, "", fmt.Errorf("cannot resolve user config directory")
	}
	if _, err := os.Stat(userPath); err == nil {
		cfg := config.LoadForEditWithPaths(paths)
		if err := a.normalizeLegacyDesktopProviderAccessForSettings(cfg, userPath, workspaceRoot); err != nil {
			return nil, "", err
		}
		if err := a.migrateLegacyBotConfigToUserForWorkspace(cfg, userPath, workspaceRoot); err != nil {
			return nil, "", err
		}
		return cfg, userPath, nil
	}
	cfg := config.LoadForEditWithPaths(paths)
	legacyPath := config.SourcePathForRoot(workspaceRoot)
	if legacyPath == "" || sameConfigPath(legacyPath, userPath) {
		if err := a.normalizeLegacyDesktopProviderAccessForSettings(cfg, userPath, workspaceRoot); err != nil {
			return nil, "", err
		}
		return cfg, userPath, nil
	}
	legacyCfg := config.LoadForEdit(legacyPath)
	if err := a.normalizeLegacyDesktopProviderAccessForSettings(legacyCfg, legacyPath, workspaceRoot); err != nil {
		return nil, "", err
	}
	legacyCfg.ConfigVersion = config.Default().ConfigVersion
	if err := migrateLegacyBotConfigToUser(cfg, legacyCfg, userPath); err != nil {
		return nil, "", err
	}
	return legacyCfg, userPath, nil
}

func (a *App) loadDesktopUserConfigForView() (*config.Config, string, error) {
	paths := a.activeProfilePathResolver()
	return a.loadDesktopUserConfigForViewWithPaths(paths, a.activeWorkspaceRoot())
}

func (a *App) loadDesktopUserConfigForViewWithPaths(paths config.ProfilePathResolver, workspaceRoot string) (*config.Config, string, error) {
	userPath := paths.UserConfigPath()
	if userPath == "" {
		return nil, "", fmt.Errorf("cannot resolve user config directory")
	}
	if _, err := os.Stat(userPath); err == nil {
		cfg := config.LoadForEditWithoutCredentialsWithPaths(paths)
		if err := a.normalizeLegacyDesktopProviderAccessForSettings(cfg, userPath, workspaceRoot); err != nil {
			return nil, "", err
		}
		legacyPath := config.SourcePathForRoot(workspaceRoot)
		if legacyPath != "" && !sameConfigPath(legacyPath, userPath) {
			legacyCfg := config.LoadForEditWithoutCredentials(legacyPath)
			if err := migrateLegacyBotConfigToUser(cfg, legacyCfg, userPath); err != nil {
				return nil, "", err
			}
		}
		return cfg, userPath, nil
	}
	cfg := config.LoadForEditWithoutCredentialsWithPaths(paths)
	legacyPath := config.SourcePathForRoot(workspaceRoot)
	if legacyPath == "" || sameConfigPath(legacyPath, userPath) {
		if err := a.normalizeLegacyDesktopProviderAccessForSettings(cfg, userPath, workspaceRoot); err != nil {
			return nil, "", err
		}
		return cfg, userPath, nil
	}
	legacyCfg := config.LoadForEditWithoutCredentials(legacyPath)
	if err := a.normalizeLegacyDesktopProviderAccessForSettings(legacyCfg, legacyPath, workspaceRoot); err != nil {
		return nil, "", err
	}
	legacyCfg.ConfigVersion = config.Default().ConfigVersion
	if err := migrateLegacyBotConfigToUser(cfg, legacyCfg, userPath); err != nil {
		return nil, "", err
	}
	return legacyCfg, userPath, nil
}

func (a *App) migrateLegacyBotConfigToUser(userCfg *config.Config, userPath string) error {
	return a.migrateLegacyBotConfigToUserForWorkspace(userCfg, userPath, a.activeWorkspaceRoot())
}

func (a *App) migrateLegacyBotConfigToUserForWorkspace(userCfg *config.Config, userPath, workspaceRoot string) error {
	if userCfg == nil {
		return nil
	}
	legacyPath := config.SourcePathForRoot(workspaceRoot)
	if legacyPath == "" || sameConfigPath(legacyPath, userPath) {
		return nil
	}
	legacyCfg := config.LoadForEdit(legacyPath)
	return migrateLegacyBotConfigToUser(userCfg, legacyCfg, userPath)
}

func migrateLegacyBotConfigToUser(userCfg, legacyCfg *config.Config, userPath string) error {
	if userCfg == nil || legacyCfg == nil || desktopBotConfigConfigured(userCfg.Bot) {
		return nil
	}
	if !desktopBotConfigConfigured(legacyCfg.Bot) {
		return nil
	}
	userCfg.Bot = legacyCfg.Bot
	if err := userCfg.SaveToScope(userPath, config.RenderScopeUser); err != nil {
		return fmt.Errorf("migrate legacy bot config: %w", err)
	}
	return nil
}

func desktopBotConfigConfigured(bot config.BotConfig) bool {
	defaults := config.Default().Bot
	if bot.Enabled || strings.TrimSpace(bot.Model) != "" || len(bot.Connections) > 0 {
		return true
	}
	if (bot.MaxSteps != 0 && bot.MaxSteps != defaults.MaxSteps) || (bot.DebounceMs != 0 && bot.DebounceMs != defaults.DebounceMs) {
		return true
	}
	if bot.Allowlist.AllowAll ||
		len(bot.Allowlist.QQUsers)+len(bot.Allowlist.FeishuUsers)+len(bot.Allowlist.WeixinUsers) > 0 ||
		len(bot.Allowlist.QQGroups)+len(bot.Allowlist.FeishuGroups)+len(bot.Allowlist.WeixinGroups) > 0 {
		return true
	}
	if bot.QQ.Enabled || strings.TrimSpace(bot.QQ.AppID) != "" || bot.QQ.AppSecretEnv != defaults.QQ.AppSecretEnv || bot.QQ.Sandbox != defaults.QQ.Sandbox {
		return true
	}
	if bot.Feishu.Enabled ||
		strings.TrimSpace(bot.Feishu.AppID) != "" ||
		bot.Feishu.Domain != defaults.Feishu.Domain ||
		bot.Feishu.AppSecretEnv != defaults.Feishu.AppSecretEnv ||
		strings.TrimSpace(bot.Feishu.VerificationToken) != "" ||
		bot.Feishu.Mode != defaults.Feishu.Mode ||
		bot.Feishu.WebhookPort != defaults.Feishu.WebhookPort ||
		bot.Feishu.RequireMention != defaults.Feishu.RequireMention {
		return true
	}
	if bot.Weixin.Enabled ||
		bot.Weixin.AccountID != defaults.Weixin.AccountID ||
		bot.Weixin.TokenEnv != defaults.Weixin.TokenEnv ||
		bot.Weixin.APIBase != defaults.Weixin.APIBase {
		return true
	}
	return false
}

func (a *App) normalizeLegacyDesktopProviderAccessForSettings(cfg *config.Config, path, workspaceRoot string) error {
	// Metadata-only routes deliberately inspect provider metadata without
	// resolving local BYOK credentials. The legacy migration calls
	// Configured/APIKey, so it belongs only to the credential-aware policy.
	if !a.desktopProviderCredentialsAllowedForActiveRoute(workspaceRoot) {
		return nil
	}
	if cfg == nil || len(cfg.Desktop.ProviderAccess) > 0 || configDeclaresProviderAccess(path) {
		return nil
	}
	config.NormalizeLegacyDesktopProviderAccess(cfg)
	if len(cfg.Desktop.ProviderAccess) == 0 || strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return cfg.SaveTo(path)
}

func configDeclaresProviderAccess(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(body), "\n") {
		if before, _, ok := strings.Cut(line, "#"); ok {
			line = before
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "provider_access") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "provider_access"))
			return strings.HasPrefix(rest, "=")
		}
	}
	return false
}

func (a *App) activeWorkspaceRoot() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if tab := a.activeTabLocked(); tab != nil {
		return tab.WorkspaceRoot
	}
	return "."
}

func (a *App) activeProfilePathResolver() config.ProfilePathResolver {
	workspaceRoot := "."
	if tab := a.activeTab(); tab != nil {
		workspaceRoot = tab.WorkspaceRoot
	}
	return config.NewProfilePathResolver(a.activeConfigActorForWorkspace(workspaceRoot))
}

func (a *App) saveProviderCredential(apiKeyEnv, value string) (string, error) {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	value = strings.TrimSpace(value)
	if err := upsertDotEnvWithPaths(a.activeProfilePathResolver(), apiKeyEnv, value); err != nil {
		return "", err
	}
	return providerCredentialSourceNotice(apiKeyEnv, value), nil
}

func providerCredentialSourceNotice(apiKeyEnv, value string) string {
	return ""
}

func projectConfigPathForRoot(root string) string {
	return config.ProjectConfigPathForRoot(root)
}

func sameConfigPath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr == nil && bErr == nil {
		return filepath.Clean(aAbs) == filepath.Clean(bAbs)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// rebuild tears down the controller and rebuilds it from the (just-changed)
// config, carrying the conversation forward. It keeps the active model if it
// still resolves; otherwise it falls back to the new default. Mirrors SetModel.
func (a *App) rebuild() error {
	if a.ctx == nil {
		return nil
	}
	tab := a.activeTab()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("settings")
	}
	var carried []provider.Message
	prevPath := ""
	actor := tabRuntimeActorContext(tab)
	if tab.Ctrl != nil {
		prevPath = tab.Ctrl.SessionPath()
		if err := a.snapshotTab(tab); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.settings.rebuild", err)
		}
		carried = tab.Ctrl.History()
		tab.Ctrl.Close()
	}
	model := tab.model
	if cfg, err := a.desktopConfigForBillingRoute(tab.WorkspaceRoot, actor, modelbilling.BillingSource(tab.billingSource)); err == nil {
		if resolved, fallback, ok := cfg.ResolveModelWithFallback(model); ok {
			if fallback && strings.TrimSpace(model) != "" {
				a.noticeForTab(tab.ID, fmt.Sprintf("model %q is no longer available; switched to %s", model, resolved))
			}
			model = resolved
		}
	}
	sharedHost := a.lookupSharedHost(tab.SharedHostKey)
	bootOptions := boot.Options{
		Model: model, RequireKey: false,
		Sink:                     tab.sink,
		WorkspaceRoot:            tab.WorkspaceRoot,
		SessionDir:               tabSessionDir(tab),
		EffortOverride:           cloneStringPtr(tab.effort),
		TokenMode:                currentTabTokenMode(tab),
		SharedHost:               sharedHost,
		CleanupPendingReconciler: reconcileDesktopCleanupPending,
		Actor:                    actor,
	}
	applyTabSceneBootOptions(&bootOptions, tab)
	ctrl, err := a.buildDesktopController(a.bootContext(), bootOptions, actor, tab.WorkspaceRoot, model, tab.billingSource)
	if err != nil {
		a.mu.Lock()
		tab.Ctrl = nil
		setTabStartupFailureLocked(tab, err, tabStartupFailureController)
		tab.Ready = true
		a.saveTabsLocked()
		a.mu.Unlock()
		a.emitReady(a.ctx)
		return err
	}
	a.bindControllerDisplayRecorder(ctrl)
	a.mu.Lock()
	tab.Ctrl = ctrl
	tab.model = model
	tab.Label = ctrl.Label()
	clearTabStartupFailureLocked(tab)
	tab.Ready = true
	a.saveTabsLocked()
	a.mu.Unlock()
	a.emitReady(a.ctx)
	ctrl.EnableInteractiveApproval()
	applyTabModeToController(ctrl, tab.mode)
	path := agent.ContinueSessionPath(prevPath, ctrl.SessionDir(), ctrl.Label())
	resumeWithFreshSystemPrompt(ctrl, carried, path)
	a.persistTabSessionPath(tab, path)
	return nil
}

// SetDefaultModel sets the config default and switches the live model to it.
func (a *App) SetDefaultModel(ref string) error {
	source := modelbilling.BillingNewAPIByok
	if strings.HasPrefix(strings.TrimSpace(ref), brand.ProviderID+"/") {
		source = modelbilling.BillingSubscriptionPool
	}
	return a.SetDefaultModelRoute(ref, string(source))
}

// SetDefaultModelRoute persists the model and its explicit billing route as one
// desktop preference. Provider/model alone is never treated as a task billing
// decision after this boundary.
func (a *App) SetDefaultModelRoute(ref, billingSource string) error {
	tab := a.activeTab()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	source := modelbilling.BillingSource(strings.TrimSpace(billingSource))
	if err := source.Validate(); err != nil {
		return err
	}
	prevModel, prevSource := tab.model, tab.billingSource
	tab.model = ref
	tab.billingSource = string(source)
	actor := tabRuntimeActorContext(tab)
	if err := a.applyConfigChange(func(c *config.Config) error {
		if source == modelbilling.BillingSubscriptionPool {
			a.applyDesktopSubscriptionModels(c, actor)
		}
		resolved, err := a.selectableDesktopModelRefForSource(c, ref, source)
		if err != nil {
			return err
		}
		a.applyDefaultModelRoute(c, resolved, source)
		tab.model = resolved
		return nil
	}); err != nil {
		tab.model, tab.billingSource = prevModel, prevSource
		return err
	}
	return nil
}

func (a *App) applyDefaultModelRoute(c *config.Config, ref string, source modelbilling.BillingSource) {
	c.DefaultModel = ref
	c.Desktop.DefaultBillingSource = string(source)
	a.clearDerivedModelsOutsideBillingSource(c, source)
}

func (a *App) clearDerivedModelsOutsideBillingSource(c *config.Config, source modelbilling.BillingSource) {
	if c.Agent.PlannerModel != "" {
		if _, err := a.selectableDesktopModelRefForSource(c, c.Agent.PlannerModel, source); err != nil {
			c.Agent.PlannerModel = ""
		}
	}
	if c.Agent.SubagentModel != "" {
		if _, err := a.selectableDesktopModelRefForSource(c, c.Agent.SubagentModel, source); err != nil {
			c.Agent.SubagentModel = ""
		}
	}
	for skill, ref := range c.Agent.SubagentModels {
		if _, err := a.selectableDesktopModelRefForSource(c, ref, source); err != nil {
			delete(c.Agent.SubagentModels, skill)
		}
	}
}

// SetPlannerModel sets (or, with "", clears) the two-model planner.
func (a *App) SetPlannerModel(ref string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		if ref != "" {
			source := desktopSettingsDefaultBillingSource(c)
			if source == modelbilling.BillingSubscriptionPool {
				a.applyDesktopSubscriptionModels(c, a.cloudActorContext())
			}
			resolved, err := a.selectableDesktopModelRefForSource(c, ref, source)
			if err != nil {
				return err
			}
			ref = resolved
		}
		c.Agent.PlannerModel = ref
		return nil
	})
}

// SetSubagentModel sets (or clears) the default model used by subagent entry points.
func (a *App) SetSubagentModel(ref string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			source := desktopSettingsDefaultBillingSource(c)
			if source == modelbilling.BillingSubscriptionPool {
				a.applyDesktopSubscriptionModels(c, a.cloudActorContext())
			}
			resolved, err := a.selectableDesktopModelRefForSource(c, ref, source)
			if err != nil {
				return err
			}
			ref = resolved
		}
		c.Agent.SubagentModel = ref
		return nil
	})
}

func (a *App) selectableDesktopModelRefForSource(c *config.Config, ref string, source modelbilling.BillingSource) (string, error) {
	if source == "" {
		source = desktopSettingsDefaultBillingSource(c)
	}
	if err := source.Validate(); err != nil {
		return "", err
	}
	entry, ok := c.ResolveModel(ref)
	if !ok {
		return "", fmt.Errorf("unknown model %q", ref)
	}
	if source == modelbilling.BillingSubscriptionPool {
		if entry.Name != brand.ProviderID {
			return "", fmt.Errorf("model %q is not available from the points channel", ref)
		}
	} else {
		if entry.Name == brand.ProviderID {
			return "", fmt.Errorf("official points-channel model %q cannot use a custom provider route", ref)
		}
		if !modelProviderAccessAllowed(providerAccessSet(c.Desktop.ProviderAccess), entry.Name) {
			return "", fmt.Errorf("model %q is not available because provider %q is not added", ref, entry.Name)
		}
		if !entry.Configured() {
			return "", fmt.Errorf("model %q is not available because provider %q has no key", ref, entry.Name)
		}
	}
	if !config.IsLikelyChatModel(entry.Model) {
		return "", fmt.Errorf("model %q is not available for chat", ref)
	}
	return entry.Name + "/" + entry.Model, nil
}

func desktopSettingsDefaultBillingSource(c *config.Config) modelbilling.BillingSource {
	if c == nil {
		return ""
	}
	source := modelbilling.BillingSource(strings.TrimSpace(c.Desktop.DefaultBillingSource))
	if source == modelbilling.BillingSubscriptionPool || source == modelbilling.BillingNewAPIByok {
		return source
	}
	if strings.HasPrefix(strings.TrimSpace(c.DefaultModel), brand.ProviderID+"/") {
		return modelbilling.BillingSubscriptionPool
	}
	if strings.TrimSpace(c.DefaultModel) != "" {
		return modelbilling.BillingNewAPIByok
	}
	return ""
}

func selectableDesktopModelRef(c *config.Config, ref string) (string, error) {
	entry, ok := c.ResolveModel(ref)
	if !ok {
		return "", fmt.Errorf("unknown model %q", ref)
	}
	if !modelProviderAccessAllowed(providerAccessSet(c.Desktop.ProviderAccess), entry.Name) {
		return "", fmt.Errorf("model %q is not available because provider %q is not added", ref, entry.Name)
	}
	if !entry.Configured() {
		return "", fmt.Errorf("model %q is not available because provider %q has no key", ref, entry.Name)
	}
	if !config.IsLikelyChatModel(entry.Model) {
		return "", fmt.Errorf("model %q is not available for chat", ref)
	}
	return entry.Name + "/" + entry.Model, nil
}

// SetSubagentEffort sets (or clears) the default effort used by subagent entry points.
func (a *App) SetSubagentEffort(level string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		level = strings.TrimSpace(level)
		if level == "" || level == "auto" {
			c.Agent.SubagentEffort = ""
			return nil
		}
		model := strings.TrimSpace(c.Agent.SubagentModel)
		if model == "" {
			model = c.DefaultModel
		}
		if desktopSettingsDefaultBillingSource(c) == modelbilling.BillingSubscriptionPool {
			a.applyDesktopSubscriptionModels(c, a.cloudActorContext())
		}
		entry, ok := c.ResolveModel(model)
		if !ok {
			return fmt.Errorf("unknown subagent model %q", model)
		}
		effort, err := config.NormalizeEffort(entry, level)
		if err != nil {
			return err
		}
		c.Agent.SubagentEffort = effort
		return nil
	})
}

// SetAutoPlan updates the automatic plan-mode gate (off|on).
func (a *App) SetAutoPlan(mode string) error {
	if err := a.ensureLiveControllersRuntimeMutationAllowed("auto-plan"); err != nil {
		return err
	}
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := cfg.SetAutoPlan(mode); err != nil {
		return err
	}
	if err := cfg.SaveTo(path); err != nil {
		return err
	}
	a.applyAutoPlanToLiveControllers(cfg.Agent.AutoPlan)
	if desktopAutoPlanMode(cfg.Agent.AutoPlan) == "on" && strings.TrimSpace(cfg.Agent.AutoPlanClassifier) != "" {
		return a.rebuild()
	}
	return nil
}

// SetDefaultToolApprovalMode updates the database-backed Ask/Auto/YOLO default
// used for newly-created desktop sessions. Existing tabs keep their mode.
func (a *App) SetDefaultToolApprovalMode(mode string) error {
	store := newDesktopTabStore(a.desktopConfigDir())
	return store.setDefaultToolApprovalMode(mode)
}

func desktopAutoPlanMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on", "ask":
		return "on"
	default:
		return "off"
	}
}

func (a *App) applyAutoPlanToLiveControllers(fallback string) {
	type liveTab struct {
		root   string
		actor  userctx.ActorContext
		source modelbilling.BillingSource
		ctrl   control.SessionAPI
	}
	var tabs []liveTab
	a.mu.RLock()
	for _, tab := range a.tabRuntimes {
		if tab != nil && tab.Ctrl != nil {
			tabs = append(tabs, liveTab{root: tab.WorkspaceRoot, actor: tabRuntimeActorContext(tab), source: modelbilling.BillingSource(tab.billingSource), ctrl: tab.Ctrl})
		}
	}
	a.mu.RUnlock()
	for _, tab := range tabs {
		mode := fallback
		if cfg, _, err := desktopCatalogConfigForBillingRoute(tab.root, tab.actor, tab.source); err == nil {
			mode = cfg.Agent.AutoPlan
		}
		tab.ctrl.SetAutoPlan(mode)
	}
}

func officialProviderTemplate(kind, pricingLanguage string) ([]config.ProviderEntry, string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case brand.ProviderID, brand.ProviderID + "-official":
		return []config.ProviderEntry{{
			Name:              brand.ProviderID,
			Kind:              "openai",
			BaseURL:           config.OfficialProviderBaseURL,
			Models:            []string{"gpt-5.4", "gpt-5.4-mini"},
			Default:           "gpt-5.4",
			APIKeyEnv:         brand.OfficialAPIKeyEnv,
			ContextWindow:     1_000_000,
			ReasoningProtocol: "openai",
			SupportedEfforts:  []string{"low", "medium", "high"},
		}}, brand.OfficialAPIKeyEnv, nil
	default:
		return nil, "", fmt.Errorf("unknown official provider template %q", kind)
	}
}

func providerModels(models []string) []string {
	out := make([]string, 0, len(models))
	seen := map[string]bool{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		out = append(out, model)
	}
	return out
}

func providerVisionModels(models, visionModels []string) []string {
	enabled := map[string]bool{}
	for _, model := range models {
		enabled[model] = true
	}
	out := make([]string, 0, len(visionModels))
	for _, model := range providerModels(visionModels) {
		if enabled[model] {
			out = append(out, model)
		}
	}
	return out
}

func providerDefaultForModels(currentDefault string, models []string) string {
	currentDefault = strings.TrimSpace(currentDefault)
	if currentDefault != "" {
		for _, model := range models {
			if model == currentDefault {
				return currentDefault
			}
		}
	}
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

// SaveProvider adds or updates a provider. Enabled models are persisted through
// `models` even when only one model is selected, while `model` remains populated
// in-memory for validation/back-compat. The shared key/endpoint live on the entry.
func (a *App) SaveProvider(p ProviderView) error {
	return a.saveProvider(p, a.applyConfigChange)
}

// SaveProviderConfigOnly persists provider setup without rebuilding the active
// conversation. Settings uses this for BYOK setup so an unrelated unavailable
// subscription route cannot block editing or testing a custom provider.
func (a *App) SaveProviderConfigOnly(p ProviderView) error {
	return a.saveProvider(p, a.applyConfigOnly)
}

func (a *App) saveProvider(p ProviderView, persist func(func(*config.Config) error) error) error {
	if strings.TrimSpace(p.Name) == brand.ProviderID {
		return fmt.Errorf("the official points channel is managed by the signed-in account")
	}
	return persist(func(c *config.Config) error {
		e := config.ProviderEntry{Name: p.Name}
		for i := range c.Providers {
			if c.Providers[i].Name == p.Name {
				e = c.Providers[i]
				break
			}
		}
		e.Name = p.Name
		e.Kind = p.Kind
		e.BaseURL = p.BaseURL
		e.ModelsURL = p.ModelsURL
		e.APIKeyEnv = p.APIKeyEnv
		e.BalanceURL = strings.TrimSpace(p.BalanceURL)
		e.ContextWindow = p.ContextWindow
		e.ReasoningProtocol = p.ReasoningProtocol
		e.SupportedEfforts = p.SupportedEfforts
		e.DefaultEffort = p.DefaultEffort
		e.Model = ""
		e.Models = nil
		e.Default = ""
		e.VisionModels = nil
		models := providerModels(p.Models)
		if len(models) > 0 {
			e.Model = models[0] // also satisfies validateProvider's model requirement
			e.Models = models
			if p.VisionModelsSet || len(p.VisionModels) > 0 {
				e.Vision = false
				e.VisionModels = providerVisionModels(models, p.VisionModels)
			}
			if len(models) > 1 {
				e.Default = providerDefaultForModels(p.Default, models)
			}
		} else {
			e.Vision = false
			e.VisionModels = nil
		}
		if err := c.UpsertProvider(e); err != nil {
			return err
		}
		addProviderAccess(c, p.Name)
		return nil
	})
}

// AddOfficialProviderAccess adds one curated desktop provider template to the
// Settings > Model > Access list. The runtime default providers still exist
// independently; this only records the user's explicit access setup.
func (a *App) AddOfficialProviderAccess(kind, key string) (string, error) {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	if normalizedKind == brand.ProviderID || normalizedKind == brand.ProviderID+"-official" {
		return "", fmt.Errorf("the official points channel is built in and managed by the signed-in account")
	}
	cfg, _, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return "", err
	}
	entries, keyEnv, err := officialProviderTemplate(kind, cfg.DeepSeekOfficialPricingLanguage())
	if err != nil {
		return "", err
	}
	if err := a.ensureActiveTabRebuildAllowed("provider access"); err != nil {
		return "", err
	}
	keyWarning := ""
	if strings.TrimSpace(key) != "" && keyEnv != "" {
		var err error
		keyWarning, err = a.saveProviderCredential(keyEnv, key)
		if err != nil {
			return "", err
		}
	}
	if err := a.applyConfigChange(func(c *config.Config) error {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if err := c.UpsertProvider(e); err != nil {
				return err
			}
			names = append(names, e.Name)
		}
		if _, ok := c.ResolveModel(c.DefaultModel); !ok && len(entries) > 0 {
			c.DefaultModel = entries[0].Name + "/" + entries[0].DefaultModel()
		}
		addProviderAccess(c, names...)
		return nil
	}); err != nil {
		return "", err
	}
	return keyWarning, nil
}

// FetchProviderModels probes the provider's OpenAI-compatible model-list
// endpoint and returns the available model IDs. This is a settings-only helper:
// it never touches chat request serialization or provider-visible prompt data.
func (a *App) FetchProviderModels(p ProviderView) ([]string, error) {
	if strings.TrimSpace(p.Name) == brand.ProviderID {
		if err := a.RefreshOfficialModels(); err != nil {
			return []string{}, err
		}
		return a.desktopSubscriptionModels(), nil
	}
	// Provider discovery is an explicit settings action for the provider being
	// edited. It must not inherit the active chat tab's billing route: a user can
	// configure a local provider while the current conversation still uses the
	// official subscription channel.
	root, actor, _ := a.desktopActiveRouteSnapshot()
	e := config.ProviderEntry{
		Name:      p.Name,
		BaseURL:   p.BaseURL,
		ModelsURL: p.ModelsURL,
		APIKeyEnv: p.APIKeyEnv,
	}
	e.ResolveAPIKeyForRootWithPaths(root, config.NewProfilePathResolver(actor))
	ctx, cancel := context.WithTimeout(a.reqCtx(), 15*time.Second)
	defer cancel()
	models, err := e.FetchModels(ctx)
	if err != nil {
		return []string{}, err
	}
	return nonNil(providerModels(models)), nil
}

// DeleteProvider removes a provider and retargets open idle tabs that used it.
func (a *App) DeleteProvider(name string) error {
	return a.deleteProviderAndRetargetTabs(name)
}

// RemoveProviderAccess hides a provider from Settings > Model > Access and from
// settings model pickers. Built-in provider entries remain in the runtime config
// for back-compat, but visible defaults and idle tabs are retargeted away from
// the removed access entry when another accessed provider is available. Custom
// providers are deleted outright.
func (a *App) RemoveProviderAccess(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("remove provider access: empty provider name")
	}
	if name == brand.ProviderID {
		return fmt.Errorf("the official points channel cannot be removed")
	}
	cfg, _, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if p, ok := cfg.Provider(name); ok && isOfficialBuiltInProvider(*p) {
		return a.removeBuiltInProviderAccessAndRetargetTabs(name)
	}
	return a.deleteProviderAndRetargetTabs(name)
}

// RefreshOfficialModels refreshes the signed-in user's MaaS credential and
// capability-filtered official chat catalog without exposing the credential.
func (a *App) RefreshOfficialModels() error {
	ctx, cancel := context.WithTimeout(a.reqCtx(), 20*time.Second)
	defer cancel()
	return a.refreshDesktopSubscriptionModels(ctx, a.cloudActorContext())
}

type providerRemovalTab struct {
	id       string
	ctrl     control.SessionAPI
	readOnly bool
}

func providerAccessFallbackRef(c *config.Config, name string) string {
	name = strings.TrimSpace(name)
	for _, candidate := range c.Desktop.ProviderAccess {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == name {
			continue
		}
		p, ok := c.Provider(candidate)
		if !ok || len(p.ModelList()) == 0 {
			continue
		}
		return p.Name + "/" + p.DefaultModel()
	}
	return ""
}

func retargetProviderReferences(c *config.Config, name, fallbackRef string) {
	if strings.TrimSpace(fallbackRef) == "" {
		return
	}
	if desktopModelRefsProvider(c, c.DefaultModel, name) {
		c.DefaultModel = fallbackRef
	}
	if desktopModelRefsProvider(c, c.Agent.PlannerModel, name) {
		c.Agent.PlannerModel = fallbackRef
	}
	if desktopModelRefsProvider(c, c.Agent.SubagentModel, name) {
		c.Agent.SubagentModel = fallbackRef
	}
	for skill, ref := range c.Agent.SubagentModels {
		if desktopModelRefsProvider(c, ref, name) {
			c.Agent.SubagentModels[skill] = fallbackRef
		}
	}
}

func (a *App) removeBuiltInProviderAccessAndRetargetTabs(name string) error {
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	fallbackRef := providerAccessFallbackRef(cfg, name)

	var affected []providerRemovalTab
	if fallbackRef != "" {
		a.mu.RLock()
		for _, id := range a.orderedTabIDsLocked() {
			tab := a.tabRuntimes[id]
			if tab == nil {
				continue
			}
			ref := tab.model
			if strings.TrimSpace(ref) == "" {
				ref = cfg.DefaultModel
			}
			if !desktopModelRefsProvider(cfg, ref, name) {
				continue
			}
			if controllerHasActiveRuntimeWork(tab.Ctrl) {
				a.mu.RUnlock()
				return fmt.Errorf("finish or cancel active work using %q before removing the provider access", name)
			}
			affected = append(affected, providerRemovalTab{id: id, ctrl: tab.Ctrl, readOnly: tab.ReadOnly})
		}
		a.mu.RUnlock()
	}

	if len(affected) == 0 {
		if err := a.ensureActiveTabRebuildAllowed("provider access"); err != nil {
			return err
		}
	}
	retargetProviderReferences(cfg, name, fallbackRef)
	removeProviderAccess(cfg, name)
	if err := cfg.SaveTo(path); err != nil {
		return err
	}
	if len(affected) == 0 {
		return a.rebuild()
	}
	for _, item := range affected {
		if item.ctrl != nil {
			if !item.readOnly {
				if err := item.ctrl.Snapshot(); err != nil {
					recordDesktopBackgroundFailure("session.snapshot.provider.access.remove", err)
				}
			}
			item.ctrl.Close()
		}
	}

	var rebuildTabs []*WorkspaceTab
	a.mu.Lock()
	for _, item := range affected {
		tab := a.tabRuntimes[item.id]
		if tab == nil {
			continue
		}
		tab.Ctrl = nil
		tab.model = fallbackRef
		tab.Label = fallbackRef
		clearTabStartupFailureLocked(tab)
		tab.Ready = a.ctx == nil
		if a.ctx != nil {
			rebuildTabs = append(rebuildTabs, tab)
		}
	}
	a.saveTabsLocked()
	a.mu.Unlock()

	for _, tab := range rebuildTabs {
		go a.buildTabController(tab)
	}
	return nil
}

func (a *App) deleteProviderAndRetargetTabs(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("remove provider: empty provider name")
	}
	if name == brand.ProviderID {
		return fmt.Errorf("the official points channel cannot be removed")
	}
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	fallbackRef, fallbackSource := providerRemovalFallbackRoute(cfg, name)

	var affected []providerRemovalTab
	a.mu.RLock()
	for _, id := range a.orderedTabIDsLocked() {
		tab := a.tabRuntimes[id]
		if tab == nil {
			continue
		}
		ref := tab.model
		if strings.TrimSpace(ref) == "" {
			ref = cfg.DefaultModel
		}
		if !desktopModelRefsProvider(cfg, ref, name) {
			continue
		}
		if controllerHasActiveRuntimeWork(tab.Ctrl) {
			a.mu.RUnlock()
			return fmt.Errorf("finish or cancel active work using %q before deleting the provider", name)
		}
		affected = append(affected, providerRemovalTab{id: id, ctrl: tab.Ctrl, readOnly: tab.ReadOnly})
	}
	a.mu.RUnlock()

	if len(affected) > 0 && fallbackRef == "" {
		return fmt.Errorf("remove provider: %q is used by open tabs and no other configured provider exists", name)
	}
	if len(affected) == 0 {
		if err := a.ensureActiveTabRebuildAllowed("provider"); err != nil {
			return err
		}
	}
	if err := cfg.RemoveProvider(name); err != nil {
		return err
	}
	removeProviderAccess(cfg, name)
	if err := cfg.SaveTo(path); err != nil {
		return err
	}

	if len(affected) == 0 {
		return a.rebuild()
	}
	for _, item := range affected {
		if item.ctrl != nil {
			if !item.readOnly {
				if err := item.ctrl.Snapshot(); err != nil {
					recordDesktopBackgroundFailure("session.snapshot.provider.delete", err)
				}
			}
			item.ctrl.Close()
		}
	}

	var rebuildTabs []*WorkspaceTab
	a.mu.Lock()
	for _, item := range affected {
		tab := a.tabRuntimes[item.id]
		if tab == nil {
			continue
		}
		tab.Ctrl = nil
		tab.model = fallbackRef
		tab.billingSource = string(fallbackSource)
		tab.Label = fallbackRef
		clearTabStartupFailureLocked(tab)
		tab.Ready = a.ctx == nil
		if a.ctx != nil {
			rebuildTabs = append(rebuildTabs, tab)
		}
	}
	a.saveTabsLocked()
	a.mu.Unlock()

	for _, tab := range rebuildTabs {
		go a.buildTabController(tab)
	}
	return nil
}

// SetProviderKey writes a secret to current profile credentials under the given
// env-var name (the one a provider's api_key_env points at) and rebuilds so it
// resolves immediately.
func (a *App) SetProviderKey(apiKeyEnv, value string) (string, error) {
	if strings.TrimSpace(apiKeyEnv) == "" {
		return "", fmt.Errorf("this provider has no api_key_env set")
	}
	if err := a.ensureActiveTabRebuildAllowed("provider key"); err != nil {
		return "", err
	}
	warning, err := a.saveProviderCredential(apiKeyEnv, value)
	if err != nil {
		return "", err
	}
	if err := a.ensureProviderAccessForKey(apiKeyEnv); err != nil {
		return "", err
	}
	if err := a.rebuild(); err != nil {
		return "", err
	}
	return warning, nil
}

// SetProviderKeyConfigOnly stores a BYOK credential without rebuilding the
// active conversation. The provider discovery request resolves this credential
// directly, so testing a custom endpoint does not depend on subscription auth.
func (a *App) SetProviderKeyConfigOnly(apiKeyEnv, value string) (string, error) {
	if strings.TrimSpace(apiKeyEnv) == "" {
		return "", fmt.Errorf("this provider has no api_key_env set")
	}
	warning, err := a.saveProviderCredential(apiKeyEnv, value)
	if err != nil {
		return "", err
	}
	if err := a.ensureProviderAccessForKey(apiKeyEnv); err != nil {
		return "", err
	}
	return warning, nil
}

func (a *App) ensureProviderAccessForKey(apiKeyEnv string) error {
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv == "" {
		return nil
	}
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	changed := false
	addAccess := func(name string) {
		if name == "" || access[name] {
			return
		}
		addProviderAccess(cfg, name)
		access[name] = true
		changed = true
	}
	for i := range cfg.Providers {
		p := cfg.Providers[i]
		if strings.TrimSpace(p.APIKeyEnv) != apiKeyEnv {
			continue
		}
		if len(p.ModelList()) == 0 {
			continue
		}
		if isOfficialBuiltInProvider(p) {
			addAccess(config.CanonicalDesktopOfficialProviderName(p.Name))
		} else {
			addAccess(strings.TrimSpace(p.Name))
		}
	}
	if !changed && (apiKeyEnv == brand.OfficialAPIKeyEnv || apiKeyEnv == brand.LegacyOfficialAPIKeyEnv) {
		entries, _, err := officialProviderTemplate(brand.ProviderID, "")
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := cfg.UpsertProvider(e); err != nil {
				return err
			}
			addAccess(e.Name)
		}
	}
	if !changed && apiKeyEnv == "DEEPSEEK_API_KEY" {
		// DeepSeek used to be an official template, but current builds expose
		// KWU only. Keep legacy/custom DEEPSEEK_API_KEY saves non-fatal
		// without restoring removed official access.
		return nil
	}
	if !changed {
		return nil
	}
	return cfg.SaveTo(path)
}

// ClearProviderKey removes a provider secret from current profile credentials and
// rebuilds so the provider immediately becomes unauthenticated.
func (a *App) ClearProviderKey(apiKeyEnv string) error {
	if strings.TrimSpace(apiKeyEnv) == "" {
		return fmt.Errorf("this provider has no api_key_env set")
	}
	if err := a.ensureActiveTabRebuildAllowed("provider key"); err != nil {
		return err
	}
	if err := removeDotEnvWithPaths(a.activeProfilePathResolver(), apiKeyEnv); err != nil {
		return err
	}
	return a.rebuild()
}

// ClearProviderKeyConfigOnly removes a BYOK credential without rebuilding the
// active conversation.
func (a *App) ClearProviderKeyConfigOnly(apiKeyEnv string) error {
	if strings.TrimSpace(apiKeyEnv) == "" {
		return fmt.Errorf("this provider has no api_key_env set")
	}
	return removeDotEnvWithPaths(a.activeProfilePathResolver(), apiKeyEnv)
}

// SetPermissionMode sets the writer-fallback mode (ask|allow|deny).
func (a *App) SetPermissionMode(mode string) error {
	return a.applyConfigChange(func(c *config.Config) error { return c.SetPermissionMode(mode) })
}

// AddPermissionRule appends a rule to the allow/ask/deny list.
func (a *App) AddPermissionRule(list, rule string) error {
	return a.applyConfigChange(func(c *config.Config) error { return c.AddPermissionRule(list, rule) })
}

// RemovePermissionRule drops a rule from the allow/ask/deny list.
func (a *App) RemovePermissionRule(list, rule string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		_, err := c.RemovePermissionRule(list, rule)
		return err
	})
}

// SetSandbox updates the bash sandbox mode, network egress, and write roots.
func (a *App) SetSandbox(bash string, network bool, workspaceRoot string, allowWrite []string, shell string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		c.Sandbox.Bash = bash
		c.Sandbox.Network = network
		c.Sandbox.WorkspaceRoot = strings.TrimSpace(workspaceRoot)
		c.Sandbox.AllowWrite = trimList(allowWrite)
		c.Tools.Shell.Prefer = strings.TrimSpace(shell)
		return nil
	})
}

func (a *App) SetBotSettings(b BotSettingsView) error {
	err := a.applyConfigOnly(func(c *config.Config) error {
		c.Bot.Enabled = b.Enabled
		c.Bot.Model = strings.TrimSpace(b.Model)
		c.Bot.ToolApprovalMode = normalizeBotConnectionToolApprovalMode(b.ToolApprovalMode)
		c.Bot.MaxSteps = b.MaxSteps
		c.Bot.DebounceMs = b.DebounceMs
		c.Bot.Allowlist = config.BotAllowlist{
			Enabled:      b.Allowlist.Enabled,
			AllowAll:     b.Allowlist.AllowAll,
			QQUsers:      trimList(b.Allowlist.QQUsers),
			FeishuUsers:  trimList(b.Allowlist.FeishuUsers),
			WeixinUsers:  trimList(b.Allowlist.WeixinUsers),
			QQGroups:     trimList(b.Allowlist.QQGroups),
			FeishuGroups: trimList(b.Allowlist.FeishuGroups),
			WeixinGroups: trimList(b.Allowlist.WeixinGroups),
		}
		c.Bot.QQ = config.QQBotConfig{
			Enabled:      b.QQ.Enabled,
			AppID:        strings.TrimSpace(b.QQ.AppID),
			AppSecretEnv: strings.TrimSpace(b.QQ.AppSecretEnv),
			Sandbox:      b.QQ.Sandbox,
		}
		c.Bot.Feishu = config.FeishuBotConfig{
			Enabled:           b.Feishu.Enabled,
			Domain:            botDomainOrDefault(b.Feishu.Domain),
			AppID:             strings.TrimSpace(b.Feishu.AppID),
			AppSecretEnv:      strings.TrimSpace(b.Feishu.AppSecretEnv),
			VerificationToken: strings.TrimSpace(b.Feishu.VerificationToken),
			Mode:              strings.TrimSpace(b.Feishu.Mode),
			WebhookPort:       b.Feishu.WebhookPort,
			RequireMention:    b.Feishu.RequireMention,
		}
		c.Bot.Weixin = config.WeixinBotConfig{
			Enabled:   b.Weixin.Enabled,
			AccountID: strings.TrimSpace(b.Weixin.AccountID),
			TokenEnv:  strings.TrimSpace(b.Weixin.TokenEnv),
			APIBase:   strings.TrimRight(strings.TrimSpace(b.Weixin.APIBase), "/"),
		}
		connections := botConnectionConfigs(b.Connections)
		if err := validateBotConnectionCredentialReferences(connections); err != nil {
			return err
		}
		c.Bot.Connections = connections
		return nil
	})
	if err == nil {
		a.refreshBotRuntimeAsync()
	}
	return err
}

func (a *App) SetBotSecret(envName, value string) error {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return fmt.Errorf("bot secret env name is empty")
	}
	if err := upsertDotEnvWithPaths(a.activeProfilePathResolver(), envName, value); err != nil {
		return err
	}
	a.refreshBotRuntimeAsync()
	return nil
}

func (a *App) ClearBotSecret(envName string) error {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		return fmt.Errorf("bot secret env name is empty")
	}
	if err := removeDotEnvWithPaths(a.activeProfilePathResolver(), envName); err != nil {
		return err
	}
	a.refreshBotRuntimeAsync()
	return nil
}

// SetCloseBehavior updates desktop-only window close behavior without rebuilding
// the active controller. It must stay out of provider-visible prompt/request data.
func (a *App) SetCloseBehavior(mode string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopCloseBehavior(mode) })
}

// SetDisplayMode updates the transcript display mode. UI-only, no rebuild needed.
func (a *App) SetDisplayMode(mode string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopDisplayMode(mode) })
}

// SetStatusBarStyle updates the desktop status bar metric label style. UI-only,
// no rebuild needed.
func (a *App) SetStatusBarStyle(style string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopStatusBarStyle(style) })
}

// SetStatusBarItems updates the ordered visible desktop status bar items.
// UI-only, no rebuild needed.
func (a *App) SetStatusBarItems(items []string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopStatusBarItems(items) })
}

// SetDesktopLanguage updates the desktop UI language and the user-level response
// language preference used by model-facing desktop sessions.
func (a *App) SetDesktopLanguage(lang string) error {
	responseLanguage := ""
	if err := a.applyConfigOnly(func(c *config.Config) error {
		if err := c.SetDesktopLanguage(lang); err != nil {
			return err
		}
		if err := c.SetLanguage(lang); err != nil {
			return err
		}
		responseLanguage = c.ResponseLanguage()
		return nil
	}); err != nil {
		return err
	}
	a.updateTrayLocale(lang)
	a.applyResponseLanguageToLiveControllers(responseLanguage)
	return nil
}

// SetTrayLocale mirrors the resolved desktop UI language into the native tray
// menu. It is runtime-only; the persisted preference remains [desktop].language.
func (a *App) SetTrayLocale(locale string) error {
	if locale != "zh" {
		locale = "en"
	}
	a.updateTrayLocale(locale)
	return nil
}

// SetDesktopAppearance updates only desktop theme preferences. It does not
// rebuild the active controller and must stay out of provider-visible requests.
func (a *App) SetDesktopAppearance(theme, style string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopAppearance(theme, style) })
}

// SetDesktopLayoutStyle updates only the desktop layout style. It does not
// rebuild the active controller and must stay out of provider-visible requests.
func (a *App) SetDesktopLayoutStyle(style string) error {
	normalized := ""
	if err := a.applyConfigOnly(func(c *config.Config) error {
		if err := c.SetDesktopLayoutStyle(style); err != nil {
			return err
		}
		normalized = c.DesktopLayoutStyle()
		return nil
	}); err != nil {
		return err
	}
	if singleSurfaceLayoutStyle(normalized) {
		return a.applySingleSurfaceTabPolicy()
	}
	return nil
}

// SetDesktopCheckUpdates updates only the desktop startup update-check
// preference. Manual checks in Settings are unaffected.
func (a *App) SetDesktopCheckUpdates(enabled bool) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopCheckUpdates(enabled) })
}

// SetDesktopTelemetry sets whether the desktop sends the anonymous launch ping.
func (a *App) SetDesktopTelemetry(enabled bool) error {
	enabled = enabled && reportEndpointAvailable(pingEndpoint)
	if err := a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopTelemetry(enabled) }); err != nil {
		return err
	}
	if !enabled {
		_ = os.Remove(pendingCrashPath())
	}
	return nil
}

// SetDesktopMetrics sets whether the desktop sends aggregate desktop metrics,
// starting or stopping the live aggregator so the toggle takes effect immediately.
func (a *App) SetDesktopMetrics(enabled bool) error {
	enabled = enabled && reportEndpointAvailable(metricsEndpoint)
	if err := a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopMetrics(enabled) }); err != nil {
		return err
	}
	switch {
	case enabled && a.metrics.Load() == nil && version != "dev":
		a.metrics.Store(newMetricsAggregator(config.MemoryUserDir()))
		if cfg, err := a.desktopNonRouteConfig(); err == nil {
			a.recordSettingsMetricsSnapshot(cfg)
		}
	case !enabled:
		a.metrics.Store(nil)
		clearPendingMetrics()
	}
	return nil
}

// SetExpandThinking sets whether reasoning text is expanded by default on
// the desktop. It is desktop-only and does not rebuild the controller.
func (a *App) SetExpandThinking(on bool) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetExpandThinking(on) })
}

// MigrateDesktopPreferences imports old browser-local desktop preferences into
// the user config once. Existing [desktop] values win so stale localStorage never
// overwrites an explicit config edit.
func (a *App) MigrateDesktopPreferences(language, theme, style string) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		if strings.TrimSpace(c.Desktop.Language) == "" {
			if err := c.SetDesktopLanguage(language); err != nil {
				return err
			}
		}
		if strings.TrimSpace(c.Desktop.Theme) == "" {
			if err := c.SetDesktopAppearance(theme, ""); err != nil {
				return err
			}
		}
		_ = style
		c.Desktop.ThemeStyle = ""
		return nil
	})
}

// SetAgentParams updates sampling temperature, optional step guards, and the
// base system prompt.
func (a *App) SetAgentParams(temperature float64, maxSteps int, plannerMaxSteps int, systemPrompt string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		c.Agent.Temperature = temperature
		c.Agent.MaxSteps = maxSteps
		c.Agent.PlannerMaxSteps = plannerMaxSteps
		c.Agent.SystemPrompt = systemPrompt
		return nil
	})
}

func (a *App) SetColdResumePrune(enabled bool) error {
	return a.applyConfigChange(func(c *config.Config) error { return c.SetColdResumePrune(enabled) })
}

func (a *App) SetReasoningLanguage(lang string) error {
	if err := a.ensureLiveControllersRuntimeMutationAllowed("reasoning language"); err != nil {
		return err
	}
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := cfg.SetReasoningLanguage(lang); err != nil {
		return err
	}
	if err := cfg.SaveTo(path); err != nil {
		return err
	}
	a.applyReasoningLanguageToLiveControllers(cfg.ReasoningLanguage())
	return nil
}

func (a *App) applyReasoningLanguageToLiveControllers(fallback string) {
	type liveTab struct {
		root   string
		actor  userctx.ActorContext
		source modelbilling.BillingSource
		ctrl   control.SessionAPI
	}
	var tabs []liveTab
	a.mu.RLock()
	for _, tab := range a.tabRuntimes {
		if tab != nil && tab.Ctrl != nil {
			tabs = append(tabs, liveTab{root: tab.WorkspaceRoot, actor: tabRuntimeActorContext(tab), source: modelbilling.BillingSource(tab.billingSource), ctrl: tab.Ctrl})
		}
	}
	a.mu.RUnlock()
	for _, tab := range tabs {
		mode := fallback
		if cfg, _, err := desktopCatalogConfigForBillingRoute(tab.root, tab.actor, tab.source); err == nil {
			mode = cfg.ReasoningLanguage()
		}
		tab.ctrl.SetReasoningLanguage(mode)
	}
}

func (a *App) applyResponseLanguageToLiveControllers(fallback string) {
	type liveTab struct {
		root   string
		actor  userctx.ActorContext
		source modelbilling.BillingSource
		ctrl   control.SessionAPI
	}
	var tabs []liveTab
	a.mu.RLock()
	for _, tab := range a.tabRuntimes {
		if tab != nil && tab.Ctrl != nil {
			tabs = append(tabs, liveTab{root: tab.WorkspaceRoot, actor: tabRuntimeActorContext(tab), source: modelbilling.BillingSource(tab.billingSource), ctrl: tab.Ctrl})
		}
	}
	a.mu.RUnlock()
	for _, tab := range tabs {
		mode := fallback
		if cfg, _, err := desktopCatalogConfigForBillingRoute(tab.root, tab.actor, tab.source); err == nil {
			mode = cfg.ResponseLanguage()
		}
		tab.ctrl.SetResponseLanguage(mode)
	}
}

// trimList drops blank entries from a string slice (and returns a non-nil slice).
func trimList(in []string) []string {
	out := []string{}
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
