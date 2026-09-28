package config

import (
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type configWriteAuditSnapshot struct {
	DefaultModel string
	Providers    []string
}

type configWriteAuditFile struct {
	DefaultModel string `toml:"default_model"`
	Providers    []struct {
		Name string `toml:"name"`
	} `toml:"providers"`
}

func auditUserConfigWrite(path string, next *Config) {
	before, err := readConfigWriteAuditSnapshot(path)
	after := configWriteAuditSnapshotFor(next)
	added, removed := diffStringSets(before.Providers, after.Providers)
	operation := configWriteOperationName()

	args := []any{
		"operation", operation,
		"path", path,
		"providers_before", before.Providers,
		"providers_after", after.Providers,
		"providers_added", added,
		"providers_removed", removed,
		"default_model_before", before.DefaultModel,
		"default_model_after", after.DefaultModel,
	}
	if err != nil {
		args = append(args, "read_error", err.Error())
	}
	slog.Info("config: write audit", args...)

	if removedCustom := removedCustomProvidersToDefaultOfficial(before.Providers, after.Providers); len(removedCustom) > 0 {
		slog.Warn("config: possible custom provider loss",
			"operation", operation,
			"path", path,
			"removed_custom_providers", removedCustom,
			"providers_before", before.Providers,
			"providers_after", after.Providers,
			"default_model_before", before.DefaultModel,
			"default_model_after", after.DefaultModel,
			"high_priority", true,
		)
	}
}

func readConfigWriteAuditSnapshot(path string) (configWriteAuditSnapshot, error) {
	if strings.TrimSpace(path) == "" {
		return configWriteAuditSnapshot{}, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return configWriteAuditSnapshot{}, nil
		}
		return configWriteAuditSnapshot{}, err
	}
	var raw configWriteAuditFile
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return configWriteAuditSnapshot{}, err
	}
	names := make([]string, 0, len(raw.Providers))
	for _, p := range raw.Providers {
		if name := strings.TrimSpace(p.Name); name != "" {
			names = append(names, name)
		}
	}
	return configWriteAuditSnapshot{
		DefaultModel: strings.TrimSpace(raw.DefaultModel),
		Providers:    uniqueSortedStrings(names),
	}, nil
}

func configWriteAuditSnapshotFor(c *Config) configWriteAuditSnapshot {
	if c == nil {
		return configWriteAuditSnapshot{}
	}
	names := make([]string, 0, len(c.Providers))
	for _, p := range c.Providers {
		if name := strings.TrimSpace(p.Name); name != "" {
			names = append(names, name)
		}
	}
	return configWriteAuditSnapshot{
		DefaultModel: strings.TrimSpace(c.DefaultModel),
		Providers:    uniqueSortedStrings(names),
	}
}

func removedCustomProvidersToDefaultOfficial(before, after []string) []string {
	if !onlyDefaultOfficialProviderNames(after) {
		return nil
	}
	defaults := defaultOfficialProviderNameSet()
	var removed []string
	for _, name := range before {
		if defaults[name] {
			continue
		}
		if !auditContainsString(after, name) {
			removed = append(removed, name)
		}
	}
	return uniqueSortedStrings(removed)
}

func onlyDefaultOfficialProviderNames(names []string) bool {
	defaults := defaultOfficialProviderNameSet()
	if len(names) == 0 || len(names) != len(defaults) {
		return false
	}
	for _, name := range names {
		if !defaults[name] {
			return false
		}
	}
	return true
}

func defaultOfficialProviderNameSet() map[string]bool {
	out := map[string]bool{}
	for _, p := range Default().Providers {
		name := strings.TrimSpace(p.Name)
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func diffStringSets(before, after []string) (added, removed []string) {
	beforeSet := stringSet(before)
	afterSet := stringSet(after)
	for name := range afterSet {
		if !beforeSet[name] {
			added = append(added, name)
		}
	}
	for name := range beforeSet {
		if !afterSet[name] {
			removed = append(removed, name)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := stringSet(values)
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func auditContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func configWriteOperationName() string {
	var pcs [32]uintptr
	n := runtime.Callers(0, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		name := frame.Function
		if name != "" && !isConfigWriteAuditFrame(name) {
			return name
		}
		if !more {
			break
		}
	}
	return "unknown"
}

func isConfigWriteAuditFrame(name string) bool {
	return strings.Contains(name, "runtime.Callers") ||
		strings.Contains(name, "configWriteOperationName") ||
		strings.Contains(name, "auditUserConfigWrite") ||
		strings.Contains(name, "(*Config).SaveToScope") ||
		strings.Contains(name, "(*Config).SaveTo") ||
		strings.Contains(name, "(*Config).WriteFile")
}
