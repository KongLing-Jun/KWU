package boot

import (
	"testing"

	"kwu/internal/config"
	"kwu/internal/plugin"
)

func TestManagedConnectorNamesAreNotFilteredFromNormalMCPBoot(t *testing.T) {
	entries := []config.PluginEntry{
		{Name: "custom", Type: "http", URL: "https://example.test/mcp"},
		{Name: "unknown-lab-connector--documents", Type: "http", URL: "https://documents.example.test/mcp"},
	}
	specs := PluginSpecsForRoot(entries, "")
	if len(specs) != 2 || specs[1].Name != "unknown-lab-connector--documents" {
		t.Fatalf("normal specs = %+v", specs)
	}
	extra := applyKnownPluginOverrides([]plugin.Spec{{Name: "unknown-lab-connector--documents"}, {Name: "custom"}}, "")
	if len(extra) != 2 || extra[0].Name != "unknown-lab-connector--documents" {
		t.Fatalf("extra specs = %+v", extra)
	}
}

func TestHostManagedMCPNamesExcludeMatchingStaticConfiguration(t *testing.T) {
	entries := []config.PluginEntry{
		{Name: "custom", Type: "http", URL: "https://example.test/mcp"},
		{Name: "unknown-lab-connector", Type: "http", URL: "https://legacy.example.test/mcp"},
		{Name: "unknown-lab-connector--documents", Type: "http", URL: "https://legacy-runtime.example.test/mcp"},
	}
	filtered := excludeManagedMCPEntries(entries, []string{"unknown-lab-connector", "unknown-lab-connector--documents"})
	if len(filtered) != 1 || filtered[0].Name != "custom" {
		t.Fatalf("filtered entries = %+v", filtered)
	}
}
