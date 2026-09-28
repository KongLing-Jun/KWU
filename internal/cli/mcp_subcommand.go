package cli

import "fmt"

type mcpSubcommandHandler func(*chatTUI, []string)

var mcpSubcommandHandlers = map[string]mcpSubcommandHandler{
	"list":          mcpListSubcommand,
	"ls":            mcpListSubcommand,
	"show":          mcpShowSubcommand,
	"tools":         mcpToolsSubcommand,
	"add":           mcpAddSubcommand,
	"connect":       mcpConnectSubcommand,
	"remove":        mcpRemoveSubcommand,
	"rm":            mcpRemoveSubcommand,
	"import":        mcpImportSubcommand,
	"trust-project": mcpTrustProjectSubcommand,
}

// runMCPSubcommand dispatches explicit MCP management commands. Connecting and
// adding are synchronous because the user explicitly requested the handshake.
func (m *chatTUI) runMCPSubcommand(input string) {
	args := tokenizeArgs(input)
	if len(args) < 2 {
		m.openMCPManager("")
		return
	}
	handler, ok := mcpSubcommandHandlers[args[1]]
	if !ok {
		m.notice("unknown /mcp subcommand " + args[1] + " — try: /mcp, /mcp list, /mcp show, /mcp add, /mcp connect, /mcp import, /mcp remove, /mcp trust-project")
		return
	}
	handler(m, args)
}

func mcpListSubcommand(m *chatTUI, _ []string) { m.showMCPStatus() }

func mcpShowSubcommand(m *chatTUI, args []string) {
	if len(args) < 3 {
		m.notice("usage: /mcp show <name>")
		return
	}
	m.openMCPManager(args[2])
}

func mcpToolsSubcommand(m *chatTUI, args []string) {
	if len(args) < 3 {
		m.notice("usage: /mcp tools <name>")
		return
	}
	m.openMCPManager(args[2])
	if m.mcp != nil {
		m.mcp.stage = mcpStageTools
	}
}

func mcpAddSubcommand(m *chatTUI, args []string) {
	entry, err := parseMCPAdd(args[2:])
	if err != nil {
		m.notice(err.Error())
		return
	}
	n, err := m.ctrl.AddMCPServer(entry)
	if err != nil {
		m.notice("mcp add: " + err.Error())
		return
	}
	m.notice(fmt.Sprintf("connected %s — %d tools, saved to config (available next message)", entry.Name, n))
}

func mcpConnectSubcommand(m *chatTUI, args []string) {
	if len(args) < 3 {
		m.notice("usage: /mcp connect <name>")
		return
	}
	n, err := m.ctrl.ConnectConfiguredMCPServer(args[2])
	if err != nil {
		m.notice("mcp connect: " + err.Error())
		return
	}
	m.host = m.ctrl.Host()
	m.notice(fmt.Sprintf("connected %s — %d tools (available next message)", args[2], n))
}

func mcpRemoveSubcommand(m *chatTUI, args []string) {
	if len(args) < 3 {
		m.notice("usage: /mcp remove <name>")
		return
	}
	name := args[2]
	disconnected, err := m.ctrl.RemoveMCPServer(name)
	if err != nil {
		m.notice("mcp remove: " + err.Error())
		return
	}
	if disconnected {
		m.notice("disconnected " + name + " and removed it from config")
		return
	}
	m.notice("removed " + name + " from config")
}

func mcpImportSubcommand(m *chatTUI, _ []string) { m.openMCPImportPicker() }

func mcpTrustProjectSubcommand(m *chatTUI, _ []string) {
	if err := m.ctrl.TrustProjectForMCP(); err != nil {
		m.notice("mcp trust-project: " + err.Error())
		return
	}
	m.notice("trusted this project for local MCP auto-start — restart the session to apply")
}
