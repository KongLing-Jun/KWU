package config

import (
	"strings"

	"kwu/internal/brand"
)

const (
	kwuDisplayName          = brand.DisplayName
	kwuOfficialDefaultModel = "deepseek-v4-flash"
	// OfficialProviderBaseURL is the built-in AirClaude OpenAI-compatible API.
	OfficialProviderBaseURL   = "https://api.airclaudeai.com/v1"
	OfficialProviderModelsURL = OfficialProviderBaseURL + "/models"
	legacyOfficialBaseURL     = "http://192.168.10.231:8080/v1"
	legacyOfficialModelsURL   = legacyOfficialBaseURL + "/models"
)

func kwuOfficialModelList() []string {
	return []string{kwuOfficialDefaultModel, "gpt-5.4", "gpt-5.4-mini"}
}

func kwuOfficialProviderEntry() ProviderEntry {
	return ProviderEntry{
		Name:              brand.ProviderID,
		Kind:              "openai",
		BaseURL:           OfficialProviderBaseURL,
		Models:            kwuOfficialModelList(),
		Default:           kwuOfficialDefaultModel,
		APIKeyEnv:         brand.OfficialAPIKeyEnv,
		ContextWindow:     1_000_000,
		ReasoningProtocol: "openai",
		SupportedEfforts:  []string{"low", "medium", "high"},
	}
}

// OfficialProviderTemplate returns the in-memory AirClaude provider used by
// runtime subscription routes. It must not be persisted to user TOML.
func OfficialProviderTemplate() ProviderEntry {
	return kwuOfficialProviderEntry()
}

// IsOfficialKWUProvider reports whether p is the built-in KWU provider. The
// provider name and the exact current/retired endpoint are both required so a
// same-named user provider remains a normal custom provider.
func IsOfficialKWUProvider(p *ProviderEntry) bool {
	return officialProviderKind(p) == brand.ProviderID
}

// normalizeKWUOfficialEndpoint migrates only the retired built-in endpoint.
// Custom provider URLs and custom model-list overrides remain untouched.
func normalizeKWUOfficialEndpoint(c *Config) bool {
	if c == nil {
		return false
	}
	changed := false
	for i := range c.Providers {
		p := &c.Providers[i]
		if strings.TrimSpace(p.Name) != brand.ProviderID || (!sameEndpoint(p.BaseURL, legacyOfficialBaseURL) && !sameEndpoint(p.BaseURL, OfficialProviderBaseURL)) {
			continue
		}
		if sameEndpoint(p.BaseURL, legacyOfficialBaseURL) {
			p.BaseURL = OfficialProviderBaseURL
			changed = true
		}
		if sameEndpoint(p.ModelsURL, legacyOfficialModelsURL) {
			p.ModelsURL = OfficialProviderModelsURL
			changed = true
		}
	}
	return changed
}

func sameEndpoint(raw, expected string) bool {
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(raw), "/"), expected)
}

func (c *Config) retargetLegacyProviderRef(ref string) string {
	if c == nil {
		return ref
	}
	provider, model, hasModel := strings.Cut(strings.TrimSpace(ref), "/")
	if provider != brand.LegacyProviderID {
		return ref
	}
	if _, ok := c.Provider(brand.ProviderID); !ok {
		return ref
	}
	if hasModel {
		return brand.ProviderID + "/" + model
	}
	return brand.ProviderID
}

func (c *Config) retargetDesktopProviderRef(ref string) string {
	access := desktopProviderAccessMap(c.Desktop.ProviderAccess)
	if len(access) == 0 {
		return ref
	}
	return retargetDesktopOfficialRef(ref, access)
}

func (c *Config) canonicalProviderNameForEdit(name string) string {
	name = strings.TrimSpace(name)
	if name == brand.LegacyProviderID {
		if _, ok := c.Provider(brand.ProviderID); ok {
			return brand.ProviderID
		}
	}
	return name
}

func canonicalDesktopOfficialProviderName(name string) string {
	switch strings.TrimSpace(name) {
	case "deepseek-flash", "deepseek-pro":
		return "deepseek"
	case brand.ProviderID:
		return brand.ProviderID
	default:
		return strings.TrimSpace(name)
	}
}

func providerEntryMatchesCanonicalOfficialAccess(p *ProviderEntry, canonical string) bool {
	if p == nil {
		return false
	}
	switch canonical {
	case "deepseek":
		if canonicalDesktopOfficialProviderName(p.Name) != "deepseek" {
			return false
		}
		return officialProviderKind(p) == "deepseek"
	case brand.ProviderID:
		if canonicalDesktopOfficialProviderName(p.Name) != brand.ProviderID {
			return false
		}
		return officialProviderKind(p) == brand.ProviderID
	default:
		return false
	}
}

func desktopProviderAccessMap(names []string) map[string]bool {
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

func LegacyDeepSeekFamilyForLanguage(pricingLanguage string) []ProviderEntry {
	cfg := &Config{
		Language: pricingLanguage,
		Providers: []ProviderEntry{
			{
				Name:          "deepseek-flash",
				Kind:          "openai",
				BaseURL:       "https://api.deepseek.com",
				Model:         "deepseek-v4-flash",
				Models:        []string{"deepseek-v4-flash"},
				Default:       "deepseek-v4-flash",
				APIKeyEnv:     "DEEPSEEK_API_KEY",
				BalanceURL:    "https://api.deepseek.com/user/balance",
				ContextWindow: 1_000_000,
			},
			{
				Name:          "deepseek-pro",
				Kind:          "openai",
				BaseURL:       "https://api.deepseek.com",
				Model:         "deepseek-v4-pro",
				Models:        []string{"deepseek-v4-pro"},
				Default:       "deepseek-v4-pro",
				APIKeyEnv:     "DEEPSEEK_API_KEY",
				BalanceURL:    "https://api.deepseek.com/user/balance",
				ContextWindow: 1_000_000,
			},
		},
	}
	cfg.ApplyDeepSeekOfficialDefaultPricing()
	return cfg.Providers
}

func WithBuiltinFamiliesForLanguage(providers []ProviderEntry, pricingLanguage string) []ProviderEntry {
	haveName := map[string]bool{}
	for _, p := range providers {
		haveName[p.Name] = true
	}
	hasLegacyDeepSeek := haveName["deepseek-flash"] || haveName["deepseek-pro"]
	defaults := Default()
	defaults.Language = pricingLanguage
	defaults.ApplyDeepSeekOfficialDefaultPricing()
	for _, bp := range defaults.Providers {
		if !haveName[bp.Name] {
			providers = append(providers, bp)
			haveName[bp.Name] = true
		}
	}
	if hasLegacyDeepSeek {
		for _, bp := range LegacyDeepSeekFamilyForLanguage(pricingLanguage) {
			if !haveName[bp.Name] {
				providers = append(providers, bp)
				haveName[bp.Name] = true
			}
		}
	}
	return providers
}

func ensureKWUOfficialProvider(c *Config) bool {
	if p, ok := c.Provider(brand.ProviderID); ok {
		if officialProviderKind(p) == brand.ProviderID {
			backfillOfficialContextWindow(p, 1_000_000)
		}
		return false
	}
	if p, ok := c.Provider(brand.LegacyProviderID); ok && officialProviderKind(p) == brand.ProviderID {
		p.Name = brand.ProviderID
		if strings.TrimSpace(p.APIKeyEnv) == brand.LegacyOfficialAPIKeyEnv {
			p.APIKeyEnv = brand.OfficialAPIKeyEnv
		}
		backfillOfficialContextWindow(p, 1_000_000)
		return true
	}
	entry := kwuOfficialProviderEntry()
	backfillOfficialContextWindow(&entry, 1_000_000)
	c.Providers = append(c.Providers, entry)
	return true
}

func ensureKWUOfficialProviderForModelRefs(c *Config) bool {
	if c == nil || hasProviderNamed(c, brand.ProviderID) || !configReferencesKWU(c) {
		return false
	}
	entry := kwuOfficialProviderEntry()
	c.Providers = append(c.Providers, entry)
	return true
}

func hasProviderNamed(c *Config, name string) bool {
	if c == nil {
		return false
	}
	for _, p := range c.Providers {
		if strings.TrimSpace(p.Name) == name {
			return true
		}
	}
	return false
}

func configReferencesKWU(c *Config) bool {
	if c == nil {
		return false
	}
	refs := []string{
		c.DefaultModel,
		c.Agent.PlannerModel,
		c.Agent.SubagentModel,
		c.Agent.AutoPlanClassifier,
		c.Bot.Model,
	}
	for _, ref := range c.Agent.SubagentModels {
		refs = append(refs, ref)
	}
	for _, conn := range c.Bot.Connections {
		refs = append(refs, conn.Model)
	}
	for _, ref := range refs {
		providerName, _, hasModel := strings.Cut(strings.TrimSpace(ref), "/")
		if providerName == brand.ProviderID && (hasModel || strings.TrimSpace(ref) == brand.ProviderID) {
			return true
		}
	}
	return false
}

func removeOfficialKWUProviders(c *Config) bool {
	if c == nil || len(c.Providers) == 0 {
		return false
	}
	out := c.Providers[:0]
	removed := false
	for i := range c.Providers {
		p := c.Providers[i]
		if IsOfficialKWUProvider(&p) {
			removed = true
			continue
		}
		out = append(out, p)
	}
	c.Providers = out
	return removed
}

func normalizeKWUOfficialModels(c *Config) bool {
	if c == nil {
		return false
	}
	changed := false
	replacements := map[string]string{}
	legacyBuiltinProviders := map[string]bool{}
	for i := range c.Providers {
		p := &c.Providers[i]
		providerChanged, legacyBuiltin, official := normalizeOneKWUOfficialProvider(p)
		if !official {
			continue
		}
		changed = changed || providerChanged
		recordLegacyKWUBuiltinProvider(legacyBuiltinProviders, p, legacyBuiltin)
		recordKWUDefaultReplacement(replacements, p)
	}
	if retargetLegacyKWUBuiltinRefs(c, legacyBuiltinProviders) {
		changed = true
	}
	return retargetDeprecatedKWUModelRefs(c, replacements) || changed
}

func normalizeOneKWUOfficialProvider(p *ProviderEntry) (changed, legacyBuiltin, official bool) {
	if officialProviderKind(p) != brand.ProviderID {
		return false, false, false
	}
	legacyBuiltin = migrateLegacyKWUBuiltinCatalog(p)
	changed = legacyBuiltin
	if normalizeKWUOfficialProviderModels(p) {
		changed = true
	}
	return changed, legacyBuiltin, true
}

func recordLegacyKWUBuiltinProvider(providers map[string]bool, p *ProviderEntry, legacy bool) {
	if !legacy {
		return
	}
	providers[p.Name] = true
	if p.Name == brand.ProviderID {
		providers[brand.LegacyProviderID] = true
	}
}

func recordKWUDefaultReplacement(replacements map[string]string, p *ProviderEntry) {
	fallback := p.DefaultModel()
	if fallback == "" {
		return
	}
	replacements[p.Name] = fallback
	if p.Name == brand.ProviderID {
		replacements[brand.LegacyProviderID] = fallback
	}
}

func migrateLegacyKWUBuiltinCatalog(p *ProviderEntry) bool {
	if p == nil || p.Default != "gpt-5.5" {
		return false
	}
	models := p.ModelList()
	if len(models) != 2 || models[0] != "gpt-5.5" || models[1] != "gpt-5.4" {
		return false
	}
	p.Models = kwuOfficialModelList()
	p.Model = ""
	p.Default = kwuOfficialDefaultModel
	return true
}

func retargetLegacyKWUBuiltinRefs(c *Config, providers map[string]bool) bool {
	if c == nil || len(providers) == 0 {
		return false
	}
	changed := false
	retarget := func(ref *string) {
		providerName, model, ok := strings.Cut(strings.TrimSpace(*ref), "/")
		if ok && providers[providerName] && model == "gpt-5.5" {
			*ref = providerName + "/" + kwuOfficialDefaultModel
			changed = true
		}
	}
	retarget(&c.DefaultModel)
	retarget(&c.Agent.PlannerModel)
	retarget(&c.Agent.SubagentModel)
	retarget(&c.Agent.AutoPlanClassifier)
	for skill, ref := range c.Agent.SubagentModels {
		next := ref
		retarget(&next)
		c.Agent.SubagentModels[skill] = next
	}
	return changed
}

func normalizeKWUOfficialProviderModels(p *ProviderEntry) bool {
	if p == nil {
		return false
	}
	models := p.ModelList()
	if len(models) == 0 {
		p.Models = kwuOfficialModelList()
		p.Model = ""
		p.Default = kwuOfficialDefaultModel
		return true
	}
	cleaned, removed := cleanKWUOfficialModelList(models)
	if !removed {
		return false
	}
	if len(cleaned) == 0 {
		cleaned = kwuOfficialModelList()
	}
	p.Models = cleaned
	if isDeprecatedKWUFallbackModel(p.Model) || !stringInSlice(p.Model, cleaned) {
		p.Model = ""
	}
	if isDeprecatedKWUFallbackModel(p.Default) || !stringInSlice(p.Default, cleaned) {
		p.Default = cleaned[0]
	}
	return true
}

func cleanKWUOfficialModelList(models []string) ([]string, bool) {
	cleaned := make([]string, 0, len(models))
	removed := false
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if isDeprecatedKWUFallbackModel(model) {
			removed = true
			continue
		}
		if !stringInSlice(model, cleaned) {
			cleaned = append(cleaned, model)
		}
	}
	return cleaned, removed
}

func stringInSlice(needle string, values []string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}

func isDeprecatedKWUFallbackModel(model string) bool {
	switch strings.TrimSpace(model) {
	case "deepseek-v4-pro":
		return true
	default:
		return false
	}
}

func retargetDeprecatedKWUModelRefs(c *Config, replacements map[string]string) bool {
	if c == nil || len(replacements) == 0 {
		return false
	}
	// The desktop subscription catalog is dynamic and may legitimately expose
	// routes that are no longer part of the built-in/BYOK KWU fallback catalog.
	// Preserve derived routes only while the effective default is still an
	// official subscription model. A project-local default from another
	// provider must not inherit stale user-level KWU planner/subagent routes.
	defaultProvider, _, defaultHasModel := strings.Cut(strings.TrimSpace(c.DefaultModel), "/")
	if strings.TrimSpace(c.Desktop.DefaultBillingSource) == "subscription_pool" &&
		defaultHasModel &&
		defaultProvider == brand.ProviderID {
		return false
	}
	changed := false
	retarget := func(ref *string) {
		next := retargetDeprecatedKWUModelRef(*ref, replacements)
		if next != *ref {
			changed = true
			*ref = next
		}
	}
	retarget(&c.DefaultModel)
	retarget(&c.Agent.PlannerModel)
	retarget(&c.Agent.SubagentModel)
	retarget(&c.Agent.AutoPlanClassifier)
	for skill, ref := range c.Agent.SubagentModels {
		next := retargetDeprecatedKWUModelRef(ref, replacements)
		if next != ref {
			changed = true
			c.Agent.SubagentModels[skill] = next
		}
	}
	return changed
}

func retargetDeprecatedKWUModelRef(ref string, replacements map[string]string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if provider, model, ok := strings.Cut(ref, "/"); ok {
		return retargetDeprecatedKWUQualifiedModelRef(provider, model, ref, replacements)
	}
	if !isDeprecatedKWUFallbackModel(ref) {
		return ref
	}
	if fallback := replacements[brand.ProviderID]; fallback != "" {
		return brand.ProviderID + "/" + fallback
	}
	provider, fallback, ok := onlyModelReplacement(replacements)
	if !ok {
		return ref
	}
	return provider + "/" + fallback
}

func retargetDeprecatedKWUQualifiedModelRef(provider, model, original string, replacements map[string]string) string {
	fallback := replacements[provider]
	if fallback == "" || !isDeprecatedKWUFallbackModel(model) {
		return original
	}
	if provider == brand.LegacyProviderID && replacements[brand.ProviderID] != "" {
		provider = brand.ProviderID
	}
	return provider + "/" + fallback
}

func onlyModelReplacement(replacements map[string]string) (string, string, bool) {
	if len(replacements) != 1 {
		return "", "", false
	}
	for provider, fallback := range replacements {
		if fallback != "" {
			return provider, fallback, true
		}
	}
	return "", "", false
}

func removeLegacyDeepSeekOfficialProviders(c *Config) bool {
	if c == nil || len(c.Providers) == 0 {
		return false
	}
	out := c.Providers[:0]
	removed := false
	for _, p := range c.Providers {
		if providerEntryMatchesCanonicalOfficialAccess(&p, "deepseek") {
			removed = true
			continue
		}
		out = append(out, p)
	}
	c.Providers = out
	return removed
}

func hasLegacyDeepSeekOfficialProviders(c *Config) bool {
	if c == nil {
		return false
	}
	for i := range c.Providers {
		if providerEntryMatchesCanonicalOfficialAccess(&c.Providers[i], "deepseek") {
			return true
		}
	}
	return false
}

func retargetDesktopOfficialRefs(c *Config, access map[string]bool) bool {
	changed := false
	retarget := func(ref *string) {
		next := retargetDesktopOfficialRef(*ref, access)
		if next != *ref {
			changed = true
			*ref = next
		}
	}
	retarget(&c.DefaultModel)
	retarget(&c.Agent.PlannerModel)
	retarget(&c.Agent.SubagentModel)
	retarget(&c.Agent.AutoPlanClassifier)
	for skill, ref := range c.Agent.SubagentModels {
		next := retargetDesktopOfficialRef(ref, access)
		if next != ref {
			changed = true
			c.Agent.SubagentModels[skill] = next
		}
	}
	return changed
}

// RetargetDesktopOfficialModelRef maps restored desktop state written by older
// official DeepSeek builds onto the current KWU official provider.
func RetargetDesktopOfficialModelRef(ref string, providerAccess []string) string {
	return retargetDesktopOfficialRef(ref, desktopProviderAccessMap(providerAccess))
}

func retargetDesktopOfficialRef(ref string, access map[string]bool) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	provider, model, hasModel := strings.Cut(ref, "/")
	if access[provider] {
		return ref
	}
	switch provider {
	case brand.LegacyProviderID:
		if access[brand.ProviderID] {
			if hasModel {
				return brand.ProviderID + "/" + kwuModelForRetarget(model, hasModel)
			}
			return brand.ProviderID
		}
		return ref
	case "deepseek":
		return retargetDeepSeekOfficialRef(ref, model, hasModel, access)
	case "deepseek-flash":
		return retargetLegacyDeepSeekSplitRef(ref, "deepseek-v4-flash", model, hasModel, access)
	case "deepseek-pro":
		return retargetLegacyDeepSeekSplitRef(ref, "deepseek-v4-pro", model, hasModel, access)
	default:
		return ref
	}
}

func retargetDeepSeekOfficialRef(ref, model string, hasModel bool, access map[string]bool) string {
	if !access[brand.ProviderID] || access["deepseek"] {
		return ref
	}
	return brand.ProviderID + "/" + kwuModelForRetarget(model, hasModel)
}

func retargetLegacyDeepSeekSplitRef(ref, fallbackModel, model string, hasModel bool, access map[string]bool) string {
	if access[brand.ProviderID] {
		return brand.ProviderID + "/" + kwuModelForRetarget(model, hasModel)
	}
	if !access["deepseek"] {
		return ref
	}
	return "deepseek/" + fallbackModelForRetarget(model, fallbackModel, hasModel)
}

func kwuModelForRetarget(model string, hasModel bool) string {
	if !hasModel || strings.TrimSpace(model) == "" || isDeprecatedKWUFallbackModel(model) {
		return kwuOfficialDefaultModel
	}
	return model
}

func fallbackModelForRetarget(model, fallback string, hasModel bool) string {
	if !hasModel || strings.TrimSpace(model) == "" {
		return fallback
	}
	return model
}
