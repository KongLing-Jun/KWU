package boot

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTokenEconomyBuiltinsExposeDeliveryWithoutHiddenVerifier(t *testing.T) {
	got := tokenEconomyBuiltins(nil)
	if containsToolName(got, "verify_artifact") {
		t.Fatalf("tokenEconomyBuiltins(nil) = %v, hidden verifier must not be model-visible", got)
	}
	if !containsToolName(got, "deliver_files") {
		t.Fatalf("tokenEconomyBuiltins(nil) = %v, want root delivery tool included", got)
	}
}

func TestTokenEconomyBuiltinsPreserveConfiguredListWithoutInjection(t *testing.T) {
	got := tokenEconomyBuiltins([]string{"grep", "unknown_builtin", "read_file", "grep"})
	want := []string{"grep", "read_file"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokenEconomyBuiltins(configured) = %v, want %v", got, want)
	}
}

func TestTokenEconomyBuiltinsRetainConfiguredDeliverFiles(t *testing.T) {
	got := tokenEconomyBuiltins([]string{"read_file", "deliver_files"})
	want := []string{"read_file", "deliver_files"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokenEconomyBuiltins(configured) = %v, want %v", got, want)
	}
}

func containsToolName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestToolSourceConnectorExecuteRoutesInstallers(t *testing.T) {
	tests := []struct {
		name   string
		source string
		set    func(*toolSourceConnector, func(context.Context) (string, error))
	}{
		{name: "skills", source: "skills", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.skills = fn }},
		{name: "read only skill alias", source: "read-only-skill", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.readOnlySkill = fn }},
		{name: "task", source: "task", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.task = fn }},
		{name: "read only task alias", source: "research_task", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.readOnlyTask = fn }},
		{name: "install", source: "install", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.install = fn }},
		{name: "web fetch", source: "webfetch", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.webFetch = fn }},
		{name: "lsp", source: "lsp", set: func(c *toolSourceConnector, fn func(context.Context) (string, error)) { c.lsp = fn }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := &toolSourceConnector{}
			tt.set(connector, func(context.Context) (string, error) { return "installed " + tt.name, nil })
			args, err := json.Marshal(map[string]string{"source": tt.source})
			if err != nil {
				t.Fatal(err)
			}
			got, err := connector.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got != "installed "+tt.name {
				t.Fatalf("Execute output = %q, want %q", got, "installed "+tt.name)
			}
		})
	}
}

func TestToolSourceConnectorExecuteMCPListAndConnect(t *testing.T) {
	connector := &toolSourceConnector{
		mcpNames: []string{"zeta", "alpha"},
		mcp: func(_ context.Context, name string) (string, error) {
			return "connected " + name, nil
		},
	}

	got, err := connector.Execute(context.Background(), json.RawMessage(`{"source":"mcp"}`))
	if err != nil {
		t.Fatalf("list MCP: %v", err)
	}
	if !strings.Contains(got, "alpha, zeta") {
		t.Fatalf("MCP list = %q, want sorted names", got)
	}

	got, err = connector.Execute(context.Background(), json.RawMessage(`{"source":"mcp","name":"zeta"}`))
	if err != nil {
		t.Fatalf("connect MCP: %v", err)
	}
	if got != "connected zeta" {
		t.Fatalf("MCP connect output = %q, want connected zeta", got)
	}
}

func TestToolSourceConnectorExecutePreservesArgumentErrors(t *testing.T) {
	connector := &toolSourceConnector{}
	if _, err := connector.Execute(context.Background(), json.RawMessage(`{`)); err == nil || !strings.Contains(err.Error(), "invalid args:") {
		t.Fatalf("invalid JSON error = %v, want invalid args", err)
	}
	if _, err := connector.Execute(context.Background(), json.RawMessage(`{"source":"unknown"}`)); err == nil || !strings.Contains(err.Error(), `unknown tool source "unknown"`) {
		t.Fatalf("unknown source error = %v", err)
	}
}
