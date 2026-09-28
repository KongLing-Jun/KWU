package boot

import (
	"strings"

	"kwu/internal/config"
	"kwu/internal/plugin"
)

func excludeManagedMCPEntries(entries []config.PluginEntry, managedNames []string) []config.PluginEntry {
	if len(entries) == 0 || len(managedNames) == 0 {
		return entries
	}
	managed := make(map[string]struct{}, len(managedNames))
	for _, name := range managedNames {
		if name = strings.TrimSpace(name); name != "" {
			managed[name] = struct{}{}
		}
	}
	out := make([]config.PluginEntry, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		_, found := managed[name]
		if !found {
			for managedName := range managed {
				if strings.HasPrefix(name, managedName+"--") {
					found = true
					break
				}
			}
		}
		if !found {
			out = append(out, entry)
		}
	}
	return out
}

func pluginSpecsWithoutManagedConnectors(entries []config.PluginEntry, workspaceRoot string) []plugin.Spec {
	specs := make([]plugin.Spec, 0, len(entries))
	for _, entry := range entries {
		specs = append(specs, pluginSpecFromEntry(entry, workspaceRoot))
	}
	return specs
}

func applyKnownPluginOverrides(specs []plugin.Spec, workspaceRoot string) []plugin.Spec {
	out := make([]plugin.Spec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, plugin.ApplyKnownOverrides(spec, workspaceRoot))
	}
	return out
}
