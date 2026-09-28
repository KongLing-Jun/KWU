package config

import "strings"

// ResolvedTier returns the normalized tier ("eager"|"background") with the
// project default applied. Legacy lazy and unknown values fall back to
// background so enabled MCPs are available without manual connection.
func (e PluginEntry) ResolvedTier() string {
	return resolvedMCPTier(e.Tier)
}

func resolvedMCPTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "eager":
		return "eager"
	case "background", "lazy":
		return "background"
	case "":
		return "background"
	default:
		return "background"
	}
}
