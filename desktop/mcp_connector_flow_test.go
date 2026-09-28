package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/config"
)

func TestValidateMCPRequiredCredentials(t *testing.T) {
	input := MCPServerInput{
		Name:            "github",
		RequiredEnvKeys: []string{"GITHUB_PERSONAL_ACCESS_TOKEN"},
		SecretEnv:       map[string]string{},
	}
	err := validateMCPRequiredCredentials(input)
	if err == nil || !strings.Contains(err.Error(), "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Fatalf("error = %v, want exact missing credential", err)
	}
	input.SecretEnv["GITHUB_PERSONAL_ACCESS_TOKEN"] = "test-token"
	if err := validateMCPRequiredCredentials(input); err != nil {
		t.Fatalf("credential should satisfy validation: %v", err)
	}
}

func TestValidateCatalogMCPInput(t *testing.T) {
	catalogIDs := []string{
		"antv-chart",
		"chrome-devtools",
		"fetch",
		"filesystem",
		"git",
		"markitdown",
		"mcp-docs",
		"memory",
		"office-mcp",
		"playwright",
		"sequential-thinking",
		"time",
		"wechat-devtools",
	}
	if len(noAuthMCPConnectorIDs) != len(catalogIDs) {
		t.Fatalf("no-auth catalog size = %d, want %d", len(noAuthMCPConnectorIDs), len(catalogIDs))
	}
	for _, catalogID := range catalogIDs {
		input := MCPServerInput{
			Name:      catalogID,
			CatalogID: catalogID,
			AuthMode:  "none",
			Command:   "test-mcp-command",
		}
		if err := validateCatalogMCPInput(input); err != nil {
			t.Fatalf("valid no-auth catalog input %q: %v", catalogID, err)
		}
	}

	valid := MCPServerInput{
		Name:      "playwright",
		CatalogID: "playwright",
		AuthMode:  "none",
		Command:   "cmd /c npx -y @playwright/mcp@latest",
	}

	withSecret := valid
	withSecret.SecretEnv = map[string]string{"TOKEN": "test-only"}
	if err := validateCatalogMCPInput(withSecret); err == nil || !strings.Contains(err.Error(), "does not accept credentials") {
		t.Fatalf("secret catalog input error = %v, want no-auth rejection", err)
	}

	futureOAuth := valid
	futureOAuth.AuthMode = "oauth"
	if err := validateCatalogMCPInput(futureOAuth); err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("future OAuth input error = %v, want reserved-mode rejection", err)
	}

	unknown := valid
	unknown.CatalogID = "unknown"
	if err := validateCatalogMCPInput(unknown); err == nil || !strings.Contains(err.Error(), "unknown MCP catalog connector") {
		t.Fatalf("unknown catalog input error = %v, want catalog rejection", err)
	}

	custom := valid
	custom.CatalogID = ""
	custom.AuthMode = ""
	custom.SecretEnv = map[string]string{"FUTURE_TOKEN": "test-only"}
	if err := validateCatalogMCPInput(custom); err != nil {
		t.Fatalf("custom MCP interface should remain available for future authenticated integrations: %v", err)
	}
}

func TestResolveCatalogMCPInputForEnvironment(t *testing.T) {
	workspaceRoot := t.TempDir()
	dataRoot := filepath.Join(t.TempDir(), "mcp-data")
	wechatCLI := filepath.Join(t.TempDir(), "cli.bat")
	input := MCPServerInput{
		CatalogID: "wechat-devtools",
		Command:   "uvx",
		Args:      []string{"server", mcpWorkspaceRootToken},
		WorkDir:   mcpWorkspaceRootToken,
		Env: map[string]string{
			"MEMORY_FILE_PATH":    mcpDataRootToken + "/memory.jsonl",
			"WECHAT_DEVTOOLS_CLI": mcpWechatCLIToken,
			"WECHAT_PROJECT_PATH": mcpWorkspaceRootToken,
		},
	}
	got := resolveCatalogMCPInputForEnvironment(input, workspaceRoot, dataRoot, wechatCLI)
	if got.Args[1] != workspaceRoot || got.WorkDir != workspaceRoot {
		t.Fatalf("workspace defaults = args %q workdir %q, want %q", got.Args[1], got.WorkDir, workspaceRoot)
	}
	if got.Env["WECHAT_DEVTOOLS_CLI"] != wechatCLI || got.Env["WECHAT_PROJECT_PATH"] != workspaceRoot {
		t.Fatalf("WeChat defaults = %#v", got.Env)
	}
	if filepath.Clean(got.Env["MEMORY_FILE_PATH"]) != filepath.Join(dataRoot, "memory.jsonl") {
		t.Fatalf("memory path = %q", got.Env["MEMORY_FILE_PATH"])
	}
	if input.Args[1] != mcpWorkspaceRootToken || input.Env["WECHAT_DEVTOOLS_CLI"] != mcpWechatCLIToken {
		t.Fatal("environment resolution mutated the catalog preset")
	}
}

func TestValidateAndEnsureCatalogMCPRuntimeInput(t *testing.T) {
	workspaceRoot := t.TempDir()
	memoryFile := filepath.Join(t.TempDir(), "nested", "memory.jsonl")

	filesystem := MCPServerInput{CatalogID: "filesystem", Args: []string{workspaceRoot}}
	if err := validateCatalogMCPRuntimeInput(filesystem); err != nil {
		t.Fatalf("valid filesystem runtime: %v", err)
	}
	office := MCPServerInput{CatalogID: "office-mcp", Env: map[string]string{"OFFICE_MCP_DEFAULT_FOLDER": workspaceRoot}}
	if err := validateCatalogMCPRuntimeInput(office); err != nil {
		t.Fatalf("valid office runtime: %v", err)
	}
	memory := MCPServerInput{CatalogID: "memory", Env: map[string]string{"MEMORY_FILE_PATH": memoryFile}}
	if err := validateCatalogMCPRuntimeInput(memory); err != nil {
		t.Fatalf("valid memory runtime: %v", err)
	}
	if err := ensureCatalogMCPRuntime(memory); err != nil {
		t.Fatalf("prepare memory runtime: %v", err)
	}
	if info, err := os.Stat(filepath.Dir(memoryFile)); err != nil || !info.IsDir() {
		t.Fatalf("memory directory was not created: %v", err)
	}

	unresolved := MCPServerInput{CatalogID: "filesystem", Args: []string{mcpWorkspaceRootToken}}
	if err := validateCatalogMCPRuntimeInput(unresolved); err == nil || !strings.Contains(err.Error(), "未解析") {
		t.Fatalf("unresolved runtime error = %v", err)
	}
	wechat := MCPServerInput{
		CatalogID: "wechat-devtools",
		Env: map[string]string{
			"WECHAT_PROJECT_PATH": workspaceRoot,
			"WECHAT_DEVTOOLS_CLI": "",
		},
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "project.config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateCatalogMCPRuntimeInput(wechat); err == nil || !strings.Contains(err.Error(), "微信开发者工具 CLI") {
		t.Fatalf("missing WeChat CLI error = %v", err)
	}
	if err := validateCatalogMCPRuntimeInput(MCPServerInput{
		CatalogID: "wechat-devtools",
		Env: map[string]string{
			"WECHAT_PROJECT_PATH": t.TempDir(),
			"WECHAT_DEVTOOLS_CLI": "cli.bat",
		},
	}); err == nil || !strings.Contains(err.Error(), "project.config.json") {
		t.Fatalf("non-mini-program workspace error = %v", err)
	}

	if _, err := exec.LookPath("git"); err == nil {
		repository := t.TempDir()
		if output, initErr := exec.Command("git", "init", repository).CombinedOutput(); initErr != nil {
			t.Fatalf("git init: %v: %s", initErr, output)
		}
		gitInput := MCPServerInput{CatalogID: "git", Args: []string{"mcp-server-git", "--repository", repository}}
		if err := validateCatalogMCPRuntimeInput(gitInput); err != nil {
			t.Fatalf("valid Git runtime: %v", err)
		}
	}
}

func TestExplainMCPConnectionFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "auth", err: errors.New("server returned 401 unauthorized"), want: "服务返回 401"},
		{name: "oauth", err: errors.New("OAuth authorization required"), want: "OAuth 尚未授权"},
		{name: "timeout", err: context.DeadlineExceeded, want: "MCP 握手超时"},
		{name: "dependency", err: errors.New("markitdown-mcp 未安装"), want: "markitdown-mcp 未安装"},
		{name: "docker", err: errors.New("Docker 未运行"), want: "Docker 未运行"},
		{name: "wechat cli", err: errors.New("未找到微信开发者工具 CLI"), want: "未找到微信开发者工具 CLI"},
		{name: "wechat project", err: errors.New("当前工作区不是微信小程序根目录"), want: "当前工作区不是微信小程序根目录"},
		{name: "git repository", err: errors.New("Git MCP 路径不是可用的 Git 仓库"), want: "Git MCP 路径不是可用的 Git 仓库"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, action := explainMCPConnectionFailure(config.PluginEntry{Name: "test"}, tt.err)
			if message != tt.want || strings.TrimSpace(action) == "" {
				t.Fatalf("message/action = %q/%q, want %q and repair action", message, action, tt.want)
			}
		})
	}
}
