package boot

import (
	"fmt"
	"strings"

	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/projecttrust"
	"kwu/internal/userctx"
)

func projectTrustedForActor(root string, _ userctx.ActorContext, paths config.ProfilePathResolver) bool {
	return (projecttrust.Store{Path: paths.ProjectTrustPath(root), Fallbacks: paths.ProjectTrustFallbackPaths(root)}).IsTrusted(root)
}

func projectMCPAutoStartEntries(entries []config.PluginEntry, trusted bool, sink event.Sink) []config.PluginEntry {
	entries, blocked := filterProjectMCPAutoStart(entries, trusted)
	if len(blocked) > 0 {
		sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo,
			Text: fmt.Sprintf("this project defines local MCP servers but is not trusted, so auto-start is disabled for: %s; run /mcp trust-project and restart the session before auto-starting them", strings.Join(blocked, ", ")),
		})
	}
	return entries
}

func filterProjectMCPAutoStart(entries []config.PluginEntry, trusted bool) ([]config.PluginEntry, []string) {
	if trusted {
		return entries, nil
	}
	out := make([]config.PluginEntry, 0, len(entries))
	blocked := make([]string, 0)
	for _, e := range entries {
		if isProjectLocalStdioMCP(e) {
			blocked = append(blocked, e.Name)
			continue
		}
		out = append(out, e)
	}
	return out, blocked
}

func isProjectLocalStdioMCP(e config.PluginEntry) bool {
	if !e.ProjectMCP || strings.TrimSpace(e.Command) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(e.Type), "stdio") || strings.TrimSpace(e.Type) == ""
}
