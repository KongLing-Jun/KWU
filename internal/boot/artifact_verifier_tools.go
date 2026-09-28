package boot

import (
	"fmt"
	"io"
	"strings"

	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
)

const verifyArtifactTool = "verify_artifact"

func modelBuiltinNames(enabled []string) []string {
	if len(enabled) == 0 {
		return nil
	}
	normalized := normalizeExplicitBuiltinNames(enabled)
	out := make([]string, 0, len(normalized))
	for _, name := range normalized {
		if name != verifyArtifactTool {
			out = append(out, name)
		}
	}
	return out
}

func normalizeExplicitBuiltinNames(enabled []string) []string {
	out := make([]string, 0, len(enabled))
	seen := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func workspaceBuiltinsForModel(ws builtin.Workspace, enabled []string, stderr io.Writer) []tool.Tool {
	names := modelBuiltinNames(enabled)
	for _, name := range names {
		if _, ok := tool.LookupBuiltin(name); !ok {
			fmt.Fprintf(stderr, "warning: unknown built-in tool %q\n", name)
		}
	}
	if len(enabled) > 0 && len(names) == 0 {
		return nil
	}
	candidates := ws.Tools(names...)
	out := candidates[:0]
	for _, candidate := range candidates {
		if candidate.Name() != verifyArtifactTool {
			out = append(out, candidate)
		}
	}
	return out
}
