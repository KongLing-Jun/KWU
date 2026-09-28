package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"kwu/internal/plugin"
	"kwu/internal/tool"
)

// mcpManager owns the session's live tool/plugin surface: the MCP plugin Host
// (live server connections), the tool Registry the executor reads each turn, and
// the session-scoped context a hot-added stdio server binds its subprocess to.
// Like approvalManager it holds the live plumbing behind its own lock, off c.mu —
// the Controller keeps the config-facing orchestration (adding to profile config,
// removing from the owning source, and building specs from entries).
//
// mu guards the lazy host creation and host-pointer reads. The registry is
// internally thread-safe (its own RWMutex) and pluginCtx is write-once, so the
// lock is held only briefly — never across the host's network/subprocess I/O.
// host is either injected at construction (the desktop shared-host path) or
// created lazily on the first connect; once set it never reverts to nil.
type mcpManager struct {
	mu        sync.Mutex
	host      *plugin.Host
	reg       *tool.Registry
	pluginCtx context.Context
}

func newMcpManager(host *plugin.Host, reg *tool.Registry, pluginCtx context.Context) mcpManager {
	return mcpManager{host: host, reg: reg, pluginCtx: pluginCtx}
}

// hostRef returns the live plugin host (nil until one is injected or lazily
// created), for the SessionAPI Host() accessor and the nil-safe read wrappers.
func (m *mcpManager) hostRef() *plugin.Host {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.host
}

// connectSpec connects (or attaches to an already-connected) MCP server and
// registers its tools, replacing any prior tools under the same prefix. Returns
// the tool count. The host's network/subprocess I/O runs off mu.
func (m *mcpManager) connectSpec(s plugin.Spec) (int, error) {
	// Explicit hot-adds must finish their bounded startup RPCs even if the
	// owning session is rebuilt while initialize/tools/list are in flight. The
	// session-scoped pluginCtx still owns the connected server's lifetime and
	// auxiliary surfaces; transport calls apply their own default deadline.
	return m.connectSpecWithContext(context.Background(), s)
}

// connectSpecWithContext lets a caller-owned flow bound the startup handshake
// while pluginCtx owns the lifetime of a successfully connected stdio child.
func (m *mcpManager) connectSpecWithContext(ctx context.Context, s plugin.Spec) (int, error) {
	m.mu.Lock()
	if m.host == nil {
		m.host = plugin.NewHost()
	}
	host, reg := m.host, m.reg
	m.mu.Unlock()
	ctx, lifeCtx := connectionContexts(ctx, m.pluginCtx, s.Type)

	tools, err := host.AddWithLifecycle(lifeCtx, ctx, s)
	if err != nil {
		if !plugin.IsServerAlreadyConnected(err) {
			return 0, err
		}
		toolsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		tools, err = host.ToolsFor(toolsCtx, s.Name)
		if err != nil {
			return 0, err
		}
	}
	if reg != nil {
		reg.ResumePrefix(plugin.ToolPrefix(s.Name))
		reg.RemovePrefix(plugin.ToolPrefix(s.Name))
		for _, t := range tools {
			reg.Add(t)
		}
	}
	return len(tools), nil
}

// connectionContexts separates the bounded startup attempt from the transport
// lifecycle. Remote HTTP connections are host-scoped because they own no child
// process; stdio children remain tied to the session-scoped plugin context.
func connectionContexts(callCtx, pluginCtx context.Context, transportType string) (context.Context, context.Context) {
	if callCtx == nil || callCtx.Err() != nil {
		callCtx = context.Background()
	}
	if pluginCtx == nil || isRemoteHTTPTransport(transportType) {
		pluginCtx = context.Background()
	}
	return callCtx, pluginCtx
}

func isRemoteHTTPTransport(transportType string) bool {
	switch strings.ToLower(strings.TrimSpace(transportType)) {
	case "http", "streamable-http", "streamable_http":
		return true
	default:
		return false
	}
}

// registeredToolCount reports how many tools from one MCP server are already
// present in this controller's registry. A shared Host can outlive a rebuilt
// controller, so Host connectivity alone does not prove the current Agent can
// see the server's tools.
func (m *mcpManager) registeredToolCount(name string) int {
	reg := m.registry()
	if reg == nil {
		return 0
	}
	prefix := plugin.ToolPrefix(name)
	count := 0
	for _, toolName := range reg.Names() {
		if strings.HasPrefix(toolName, prefix) {
			count++
		}
	}
	return count
}

// disconnect drops a live server and its tools from the registry. Reports whether
// a live server was removed.
func (m *mcpManager) disconnect(name string) bool {
	host := m.hostRef()
	if host == nil {
		return false
	}
	prefix, ok := host.Remove(name)
	if ok {
		if reg := m.registry(); reg != nil {
			reg.RemovePrefix(prefix)
		}
	}
	return ok
}

// removeToolPrefix drops a server's tools from the registry without touching the
// host — the placeholder / not-connected path. Returns the number removed.
func (m *mcpManager) removeToolPrefix(name string) int {
	reg := m.registry()
	if reg == nil {
		return 0
	}
	return reg.RemovePrefix(plugin.ToolPrefix(name))
}

// suspendToolPrefix hides a server's tools from this session's registry while a
// shared host keeps the client alive for sibling sessions.
func (m *mcpManager) suspendToolPrefix(name string) bool {
	reg := m.registry()
	if reg == nil {
		return false
	}
	reg.SuspendPrefix(plugin.ToolPrefix(name))
	return true
}

// registerTool adds a built-in tool to the live registry (e.g. the slash-command
// tool rebuilt by ReloadCommands). No-op when no registry is bound.
func (m *mcpManager) registerTool(t tool.Tool) {
	if reg := m.registry(); reg != nil {
		reg.Add(t)
	}
}

// registry returns the shared tool registry under mu (write-once, but read under
// the lock for consistency with the host pointer).
func (m *mcpManager) registry() *tool.Registry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reg
}

// serverNames lists the live server names (nil when no host is connected).
func (m *mcpManager) serverNames() []string {
	if h := m.hostRef(); h != nil {
		return h.ServerNames()
	}
	return nil
}

// hasServer reports whether a server is live.
func (m *mcpManager) hasServer(name string) bool {
	for _, n := range m.serverNames() {
		if n == name {
			return true
		}
	}
	return false
}

// prompts lists the live MCP prompts (nil when no host is connected).
func (m *mcpManager) prompts() []plugin.Prompt {
	if h := m.hostRef(); h != nil {
		return h.Prompts()
	}
	return nil
}

// failures lists the recorded MCP startup failures (nil when no host).
func (m *mcpManager) failures() []plugin.Failure {
	if h := m.hostRef(); h != nil {
		return h.Failures()
	}
	return nil
}

// readResource reads an MCP resource. Errors when no host is connected.
func (m *mcpManager) readResource(ctx context.Context, server, uri string) (string, error) {
	h := m.hostRef()
	if h == nil {
		return "", fmt.Errorf("no MCP servers connected")
	}
	return h.ReadResource(ctx, server, uri)
}

// ConnectManagedMCPServer is the non-persistent entry point for a validated
// schema 2 connector package. Desktop owns package/auth validation; this seam
// only rejects malformed runtime specs and attaches them to the existing MCP
// host and tool registry.
func (c *Controller) ConnectManagedMCPServer(ctx context.Context, spec plugin.Spec) (int, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" || !validManagedMCPSpec(spec) {
		return 0, errors.New("invalid managed MCP specification")
	}
	if tools := c.mcp.registeredToolCount(name); tools > 0 && c.mcp.hasServer(name) {
		return tools, nil
	}
	return c.mcp.connectSpecWithContext(ctx, spec)
}

func validManagedMCPSpec(spec plugin.Spec) bool {
	typeName := strings.ToLower(strings.TrimSpace(spec.Type))
	return typeName == "streamable-http" && spec.Command == "" && validManagedMCPURL(spec.URL)
}

func validManagedMCPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	return validManagedMCPHost(host) && !managedMCPQueryContainsCredential(parsed) && !strings.ContainsAny(raw, "{}")
}

func validManagedMCPHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !ip.IsMulticast()
}

func managedMCPQueryContainsCredential(parsed *url.URL) bool {
	for name := range parsed.Query() {
		normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		if strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "api-key") || strings.Contains(normalized, "authorization") || strings.Contains(normalized, "credential") || strings.Contains(normalized, "password") {
			return true
		}
	}
	return false
}
