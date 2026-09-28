package connector

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ExecutionPlan is the schema 2 execution/readiness table after expanding one
// connector package for the current Desktop platform. It deliberately contains
// no product IDs.
type ExecutionPlan struct {
	ConnectorID string
	MCP         []MCPExecution
	Skills      *SkillsExecution
}

type MCPExecution struct {
	ServerID     string
	RuntimeName  string
	Transport    string
	AuthType     string
	AuthProvider string
	URL          string
	Headers      map[string]string
}

type SkillsExecution struct {
	Root string
}

func BuildExecutionPlan(manifest Manifest, packageRoot, osName string) (ExecutionPlan, error) {
	_ = osName
	root := filepath.Clean(packageRoot)
	if root == "." || !filepath.IsAbs(root) {
		return ExecutionPlan{}, errors.New("connector execution: package root must be absolute")
	}
	if manifest.MCP == nil || manifest.Connector.Entrypoints.MCP != "mcp.json" || manifest.Connector.Kind != "mcp" && manifest.Connector.Kind != "composite" {
		return ExecutionPlan{}, errors.New("connector execution: HTTPS MCP entrypoint is required")
	}
	plan := ExecutionPlan{ConnectorID: manifest.Connector.ID}
	plan.MCP = make([]MCPExecution, 0, len(manifest.MCP.Servers))
	for _, server := range manifest.MCP.Servers {
		if server.Transport != "streamable-http" || !validMCPHTTPSURL(server.URL) {
			return ExecutionPlan{}, errors.New("connector execution: only public HTTPS streamable MCP is supported")
		}
		plan.MCP = append(plan.MCP, MCPExecution{
			ServerID: server.ID, RuntimeName: RuntimeMCPName(manifest.Connector.ID, server.ID),
			Transport: server.Transport, AuthType: server.Auth.Type, AuthProvider: server.Auth.Provider, URL: server.URL,
			Headers: cloneStringMap(server.StaticHeaders),
		})
	}
	if manifest.Connector.Entrypoints.Skills != "" {
		root, err := resolveSkillsRoot(root, manifest.Connector.Entrypoints.Skills)
		if err != nil {
			return ExecutionPlan{}, err
		}
		plan.Skills = &SkillsExecution{Root: root}
	}
	return plan, nil
}

// RuntimeMCPName prevents server IDs from colliding across connector packages.
// Both IDs have already passed the schema 2 package parser before this is used.
func RuntimeMCPName(connectorID, serverID string) string {
	return connectorID + "--" + serverID
}

func resolveSkillsRoot(packageRoot, relative string) (string, error) {
	root := filepath.Join(packageRoot, filepath.FromSlash(relative))
	rel, err := filepath.Rel(packageRoot, root)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("connector execution: skills root escapes package")
	}
	resolvedPackage, err := filepath.EvalSymlinks(packageRoot)
	if err != nil {
		return "", errors.New("connector execution: active package is unavailable")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errors.New("connector execution: skills root is unavailable")
	}
	resolvedRel, err := filepath.Rel(resolvedPackage, resolvedRoot)
	if err != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(os.PathSeparator)) {
		return "", errors.New("connector execution: skills root escapes package")
	}
	return root, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
