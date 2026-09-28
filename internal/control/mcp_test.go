package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"kwu/internal/config"
	"kwu/internal/plugin"
	"kwu/internal/tool"
)

func TestConnectManagedMCPRejectsLocalAndNonHTTPSSpecs(t *testing.T) {
	host := plugin.NewHost()
	defer host.Close()
	controller := New(Options{Host: host, Registry: tool.NewRegistry(), PluginCtx: context.Background()})

	tests := []plugin.Spec{
		{Name: "stdio", Type: "stdio", Command: "node"},
		{Name: "http", Type: "streamable-http", URL: "http://mcp.example.com/mcp"},
		{Name: "localhost", Type: "streamable-http", URL: "https://localhost/mcp"},
		{Name: "query-token", Type: "streamable-http", URL: "https://mcp.example.com/mcp?access_token=value"},
		{Name: "command", Type: "streamable-http", URL: "https://mcp.example.com/mcp", Command: "node"},
		{Name: "sse", Type: "sse", URL: "https://mcp.example.com/sse"},
	}
	for _, spec := range tests {
		if _, err := controller.ConnectManagedMCPServer(context.Background(), spec); err == nil {
			t.Fatalf("managed MCP accepted prohibited spec: %+v", spec)
		}
	}
}

func TestConnectRemoteMCPRecoversCanceledPluginContext(t *testing.T) {
	var calls atomic.Int32
	pluginCtx, cancel := context.WithCancel(context.Background())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			JSONRPC string `json:"jsonrpc"`
			ID      *int   `json:"id"`
			Method  string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode MCP request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			// Model a Desktop session rebuild after initialize but before the
			// remote server's (potentially large) tools/list response completes.
			cancel()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		result := any(map[string]any{})
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]any{"name": "test", "version": "1"},
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "read_doc",
				"description": "Read a document",
				"inputSchema": map[string]any{"type": "object"},
			}}}
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": request.JSONRPC,
			"id":      request.ID,
			"result":  result,
		}); err != nil {
			t.Errorf("encode MCP response: %v", err)
		}
	}))
	defer server.Close()

	controller := New(Options{PluginCtx: pluginCtx})

	toolCount, err := controller.ConnectMCPServer(config.PluginEntry{
		Name: "remote",
		Type: "http",
		URL:  server.URL,
	})
	if err != nil {
		t.Fatalf("ConnectMCPServer: %v", err)
	}
	if toolCount != 1 {
		t.Fatalf("tool count = %d, want 1", toolCount)
	}
	manager := newMcpManager(nil, nil, pluginCtx)
	explicitCount, err := manager.connectSpecWithContext(context.Background(), plugin.Spec{
		Name: "managed", Type: "http", URL: server.URL,
	})
	if err != nil {
		t.Fatalf("connectSpecWithContext: %v", err)
	}
	if explicitCount != 1 {
		t.Fatalf("explicit-context tool count = %d, want 1", explicitCount)
	}
	rebuiltRegistry := tool.NewRegistry()
	rebuiltManager := newMcpManager(manager.hostRef(), rebuiltRegistry, context.Background())
	rehydratedCount, err := rebuiltManager.connectSpecWithContext(context.Background(), plugin.Spec{
		Name: "managed", Type: "http", URL: server.URL,
	})
	if err != nil {
		t.Fatalf("rehydrate shared managed MCP: %v", err)
	}
	if rehydratedCount != 1 || rebuiltManager.registeredToolCount("managed") != 1 {
		t.Fatalf("rehydrated tool counts = %d / %d, want 1 / 1", rehydratedCount, rebuiltManager.registeredToolCount("managed"))
	}
	if _, ok := rebuiltRegistry.Get("mcp__managed__read_doc"); !ok {
		t.Fatal("rebuilt controller registry is missing shared-host MCP tool")
	}
	if calls.Load() != 6 {
		t.Fatalf("MCP request count = %d, want 6 without a second tools/list during rehydration", calls.Load())
	}
	if host := controller.Host(); host != nil {
		host.Close()
	}
	if host := manager.hostRef(); host != nil {
		host.Close()
	}
}
