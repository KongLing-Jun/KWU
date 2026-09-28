package config

import (
	"slices"
	"testing"
)

func TestCredentialEnvNamesIncludesMCPReferences(t *testing.T) {
	cfg := &Config{Plugins: []PluginEntry{{
		Name:    "github",
		Command: "${KWU_MCP_GITHUB_COMMAND}",
		Args:    []string{"--token", "${KWU_MCP_GITHUB_ARG}"},
		Env:     map[string]string{"TOKEN": "${KWU_MCP_GITHUB_TOKEN}"},
		Headers: map[string]string{"Authorization": "Bearer ${KWU_MCP_GITHUB_HEADER}"},
	}}}
	names := credentialEnvNamesFromConfig(cfg)
	for _, want := range []string{
		"KWU_MCP_GITHUB_ARG",
		"KWU_MCP_GITHUB_COMMAND",
		"KWU_MCP_GITHUB_HEADER",
		"KWU_MCP_GITHUB_TOKEN",
	} {
		if !slices.Contains(names, want) {
			t.Fatalf("credential refs = %v, missing %q", names, want)
		}
	}
}
