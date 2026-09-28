package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/tool"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type contextAwareBody struct {
	ctx    context.Context
	reader io.Reader
}

func (b *contextAwareBody) Read(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.reader.Read(p)
}

func (*contextAwareBody) Close() error { return nil }

// mcpHTTPServer is a minimal Streamable HTTP MCP server for tests. When sse is
// true it replies as text/event-stream (prefixing a server notification event
// to prove the client skips non-matching messages); otherwise application/json.
// It assigns a session id on initialize and fails any later request that
// doesn't echo it, and requires the Authorization header — so the test proves
// session + header plumbing, not just the happy path.
func mcpHTTPServer(t *testing.T, sse bool) *httptest.Server {
	t.Helper()
	const sessionID = "sess-xyz"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", sessionID)
		} else if got := r.Header.Get("Mcp-Session-Id"); got != sessionID {
			http.Error(w, "missing session id", http.StatusBadRequest)
			return
		}

		if req.ID == nil { // notification
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": protocolVersion, "serverInfo": map[string]any{"name": "h", "version": "0"}}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "greet",
				"description": "Greet someone.",
				"inputSchema": map[string]any{"type": "object"},
				"annotations": map[string]any{"readOnlyHint": true},
			}}}
		case "tools/call":
			var p struct {
				Arguments struct {
					Name string `json:"name"`
				} `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": "hello " + p.Arguments.Name}}}
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result}
		b, _ := json.Marshal(resp)

		if sse {
			w.Header().Set("Content-Type", "text/event-stream")
			// A server notification first: the client must skip it and keep
			// reading for the id-matching response.
			fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{}}\n\n")
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
}

func runHTTPTransportTest(t *testing.T, sse bool) {
	srv := mcpHTTPServer(t, sse)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	host, tools, err := StartAll(ctx, []Spec{{
		Name:    "h",
		Type:    "http",
		URL:     srv.URL,
		Headers: map[string]string{"Authorization": "Bearer secret"},
	}})
	if err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer host.Close()

	if len(tools) != 1 || tools[0].Name() != "mcp__h__greet" {
		t.Fatalf("tools = %v, want [mcp__h__greet]", names(tools))
	}
	if !tools[0].ReadOnly() {
		t.Error("readOnlyHint not honoured over HTTP")
	}
	got, err := tools[0].Execute(ctx, json.RawMessage(`{"name":"sam"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got != "hello sam" {
		t.Errorf("Execute = %q, want %q", got, "hello sam")
	}
}

func TestHTTPTransportJSON(t *testing.T) { runHTTPTransportTest(t, false) }
func TestHTTPTransportSSE(t *testing.T)  { runHTTPTransportTest(t, true) }

func TestHTTPTransportReinitializesExpiredSession(t *testing.T) {
	var initializeCount atomic.Int32
	var toolCallCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		if req.Method == "initialize" {
			n := initializeCount.Add(1)
			w.Header().Set("Mcp-Session-Id", fmt.Sprintf("sess-%d", n))
			writeHTTPRPCResult(w, req.ID, map[string]any{
				"protocolVersion": protocolVersion,
				"serverInfo":      map[string]any{"name": "h", "version": "0"},
			})
			return
		}

		expectedSession := fmt.Sprintf("sess-%d", initializeCount.Load())
		if got := r.Header.Get("Mcp-Session-Id"); got != expectedSession {
			http.Error(w, "missing session id", http.StatusBadRequest)
			return
		}

		if req.ID == nil { // notifications/initialized
			w.WriteHeader(http.StatusAccepted)
			return
		}

		switch req.Method {
		case "tools/list":
			writeHTTPRPCResult(w, req.ID, map[string]any{"tools": []map[string]any{{
				"name":        "greet",
				"description": "Greet someone.",
				"inputSchema": map[string]any{"type": "object"},
			}}})
		case "tools/call":
			n := toolCallCount.Add(1)
			if n == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32001,"message":"Session not found"}}`, *req.ID)
				return
			}
			if got := r.Header.Get("Mcp-Session-Id"); got != "sess-2" {
				http.Error(w, "retry did not use the new session", http.StatusBadRequest)
				return
			}
			writeHTTPRPCResult(w, req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "hello retry"}},
			})
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	host, tools, err := StartAll(ctx, []Spec{{Name: "h", Type: "http", URL: srv.URL}})
	if err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	defer host.Close()
	host.mu.RLock()
	client := host.clients[0]
	host.mu.RUnlock()

	done := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-done:
				return
			default:
				_, _ = client.hasPrompts, client.hasResources
			}
		}
	}()
	defer func() {
		close(done)
		<-readerDone
	}()

	got, err := tools[0].Execute(ctx, json.RawMessage(`{"name":"sam"}`))
	if err != nil {
		t.Fatalf("Execute after expired session: %v", err)
	}
	if got != "hello retry" {
		t.Errorf("Execute = %q, want %q", got, "hello retry")
	}
	if got := initializeCount.Load(); got != 2 {
		t.Errorf("initialize count = %d, want 2", got)
	}
	if got := toolCallCount.Load(); got != 2 {
		t.Errorf("tools/call count = %d, want 2", got)
	}
}

// TestHTTPTransportRPCError checks a JSON-RPC error response surfaces as an
// error rather than an empty result.
func TestHTTPTransportRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID *int `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32000,"message":"boom"}}`, *req.ID)
	}))
	defer srv.Close()

	ctx := context.Background()
	_, _, err := StartAll(ctx, []Spec{{Name: "e", Type: "http", URL: srv.URL}})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want initialize to fail with rpc error, got %v", err)
	}
}

func TestHTTPTransportAppliesDefaultCallDeadline(t *testing.T) {
	var sawDeadline bool
	tr := &httpTransport{
		name: "h",
		url:  "http://example.test/mcp",
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			_, sawDeadline = r.Context().Deadline()
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		})},
	}

	resp, err := tr.do(context.Background(), []byte(`{"jsonrpc":"2.0","method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !sawDeadline {
		t.Fatal("HTTP MCP request context has no default deadline")
	}
}

func TestHTTPTransportKeepsDefaultDeadlineAliveWhileReadingBody(t *testing.T) {
	tr := &httpTransport{
		name: "h",
		url:  "http://example.test/mcp",
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       &contextAwareBody{ctx: r.Context(), reader: strings.NewReader("response body")},
			}, nil
		})},
	}

	resp, err := tr.do(context.Background(), []byte(`{"jsonrpc":"2.0","method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if string(got) != "response body" {
		t.Fatalf("body = %q", got)
	}
}

func TestHTTPTransportInjectsRuntimeHeadersPerRequest(t *testing.T) {
	var calls atomic.Int32
	tr := &httpTransport{
		name:    "managed",
		url:     "https://example.test/mcp",
		headers: map[string]string{"Authorization": "static-must-not-win"},
		runtimeHeaders: func(context.Context) (RuntimeHeaderSnapshot, error) {
			calls.Add(1)
			return RuntimeHeaderSnapshot{Headers: map[string]string{"Authorization": "Bearer runtime"}}, nil
		},
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "Bearer runtime" {
				t.Fatalf("Authorization = %q", got)
			}
			return &http.Response{StatusCode: http.StatusAccepted, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		})},
	}
	for range 2 {
		resp, err := tr.do(context.Background(), []byte(`{"jsonrpc":"2.0","method":"ping"}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	if calls.Load() != 2 {
		t.Fatalf("runtime header calls = %d", calls.Load())
	}
}

func TestHTTPTransportRefreshesAndReplays401Once(t *testing.T) {
	const body = `{"jsonrpc":"2.0","method":"tools/call","params":{"value":"same-body"}}`
	var attempts atomic.Int32
	var refreshes atomic.Int32
	tr := &httpTransport{
		name:    "managed",
		url:     "https://example.test/mcp?fixed=query",
		headers: map[string]string{"X-Static": "same-static"},
		session: "same-session",
		runtimeHeaders: func(context.Context) (RuntimeHeaderSnapshot, error) {
			return RuntimeHeaderSnapshot{Headers: map[string]string{"Authorization": "Bearer first"}, Observation: "credential-a"}, nil
		},
		runtimeRefresh: func(_ context.Context, observed string) (RuntimeHeaderSnapshot, error) {
			refreshes.Add(1)
			if observed != "credential-a" {
				t.Fatalf("observed credential = %q", observed)
			}
			return RuntimeHeaderSnapshot{Headers: map[string]string{"Authorization": "Bearer refreshed"}, Observation: "credential-b"}, nil
		},
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempt := attempts.Add(1)
			gotBody, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if r.Method != http.MethodPost || r.URL.String() != "https://example.test/mcp?fixed=query" || string(gotBody) != body || r.Header.Get("X-Static") != "same-static" || r.Header.Get("Mcp-Session-Id") != "same-session" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "application/json, text/event-stream" {
				t.Fatalf("attempt %d did not preserve request: method=%s url=%s body=%q headers=%v", attempt, r.Method, r.URL, gotBody, r.Header)
			}
			wantAuthorization := "Bearer first"
			status := http.StatusUnauthorized
			if attempt == 2 {
				wantAuthorization = "Bearer refreshed"
				status = http.StatusOK
			}
			if got := r.Header.Get("Authorization"); got != wantAuthorization {
				t.Fatalf("attempt %d Authorization = %q", attempt, got)
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("response"))}, nil
		})},
	}

	resp, err := tr.do(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || attempts.Load() != 2 || refreshes.Load() != 1 {
		t.Fatalf("status=%d attempts=%d refreshes=%d", resp.StatusCode, attempts.Load(), refreshes.Load())
	}
}

func TestHTTPTransportDoesNotRefreshAfterSecond401(t *testing.T) {
	var attempts atomic.Int32
	var refreshes atomic.Int32
	tr := &httpTransport{
		name: "managed",
		url:  "https://example.test/mcp",
		runtimeHeaders: func(context.Context) (RuntimeHeaderSnapshot, error) {
			return RuntimeHeaderSnapshot{Observation: "credential-a"}, nil
		},
		runtimeRefresh: func(context.Context, string) (RuntimeHeaderSnapshot, error) {
			refreshes.Add(1)
			return RuntimeHeaderSnapshot{Observation: "credential-b"}, nil
		},
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			attempts.Add(1)
			return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
		})},
	}

	resp, err := tr.do(context.Background(), []byte(`{"jsonrpc":"2.0","method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || attempts.Load() != 2 || refreshes.Load() != 1 {
		t.Fatalf("status=%d attempts=%d refreshes=%d", resp.StatusCode, attempts.Load(), refreshes.Load())
	}
}

// TestSSETransportUnsupported documents that the legacy sse transport is
// recognised but deferred with a clear, actionable error.
func TestSSETransportUnsupported(t *testing.T) {
	_, _, err := StartAll(context.Background(), []Spec{{Name: "legacy", Type: "sse", URL: "http://x"}})
	if err == nil || !strings.Contains(err.Error(), "http") {
		t.Fatalf("sse should error pointing to http, got %v", err)
	}
}

func writeHTTPRPCResult(w http.ResponseWriter, id *int, result any) {
	if id == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": *id, "result": result}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func names(ts []tool.Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name()
	}
	return out
}
