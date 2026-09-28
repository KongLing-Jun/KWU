package plugin

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"kwu/internal/tool"
)

func TestPlaywrightMCPRealHandshakeAndCall(t *testing.T) {
	if os.Getenv("RUN_PLAYWRIGHT_MCP_INTEGRATION") != "1" {
		t.Skip("set RUN_PLAYWRIGHT_MCP_INTEGRATION=1 to run the real npm MCP sample")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	host, tools, err := StartAll(ctx, []Spec{{
		Name:    "playwright",
		Command: "cmd",
		Args:    []string{"/c", "npx", "-y", "@playwright/mcp@latest", "--headless"},
	}})
	if err != nil {
		t.Fatalf("Playwright MCP handshake/discovery failed: %v", err)
	}
	defer host.Close()
	if len(tools) == 0 {
		t.Fatal("Playwright MCP connected without discovering tools")
	}

	var navigate tool.Tool
	for _, candidate := range tools {
		if strings.HasSuffix(candidate.Name(), "__browser_navigate") {
			navigate = candidate
			break
		}
	}
	if navigate == nil {
		t.Fatalf("browser_navigate not discovered; got %d tools", len(tools))
	}
	output, err := navigate.Execute(ctx, json.RawMessage(`{"url":"data:text/html,<title>Reasonix MCP</title><main>connected</main>"}`))
	if err != nil {
		t.Fatalf("real Playwright MCP tool call failed: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Fatal("real Playwright MCP tool call returned no evidence")
	}
	t.Logf("Playwright MCP connected: tools=%d, browser_navigate output=%s", len(tools), output)
}
