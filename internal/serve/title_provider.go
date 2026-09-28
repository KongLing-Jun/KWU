package serve

import (
	"strings"

	"kwu/internal/config"
)

func titleProviderModelRef(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.DefaultModel) != "" {
		return cfg.DefaultModel
	}
	return config.Default().DefaultModel
}

func titleProviderExtra(entry *config.ProviderEntry) map[string]any {
	return map[string]any{"reasoning_protocol": config.ReasoningProtocolForEntry(entry)}
}
