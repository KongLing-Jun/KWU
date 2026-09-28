package boot

import (
	"strings"

	"kwu/internal/config"
)

// resolveSubagentModelEntry keeps an unqualified model override on the
// parent's provider when that provider offers the model. This matters when
// multiple providers expose the same model name: Config.ResolveModel preserves
// its historical first-provider behavior, while a delegated child should not
// silently leave the provider already selected by its parent. Explicit
// provider names and provider/model references retain their existing meaning.
func resolveSubagentModelEntry(cfg *config.Config, base *config.ProviderEntry, modelRef string) (*config.ProviderEntry, bool) {
	ref := strings.TrimSpace(modelRef)
	if ref == "" {
		if base == nil {
			return nil, false
		}
		entry := *base
		return &entry, true
	}
	if cfg != nil {
		_, namesProvider := cfg.Provider(ref)
		if !strings.Contains(ref, "/") && !namesProvider && base != nil && base.HasModel(ref) {
			entry := *base
			entry.Model = ref
			entry.Price = entry.PriceForModel(ref)
			return &entry, true
		}
		return cfg.ResolveModel(ref)
	}
	if base != nil && base.HasModel(ref) {
		entry := *base
		entry.Model = ref
		entry.Price = entry.PriceForModel(ref)
		return &entry, true
	}
	return nil, false
}
