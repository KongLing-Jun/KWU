package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNoAuthMCPCatalogRealHandshake(t *testing.T) {
	if os.Getenv("RUN_MCP_CATALOG_INTEGRATION") != "1" {
		t.Skip("set RUN_MCP_CATALOG_INTEGRATION=1 to run the real no-auth MCP catalog")
	}
	workspaceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	memoryFile := filepath.Join(t.TempDir(), "memory.jsonl")
	wechatCLI := integrationWechatDevtoolsCLI()

	tests := []struct {
		name           string
		spec           Spec
		requiresWeChat bool
	}{
		{name: "markitdown", spec: Spec{Name: "markitdown", Command: "uvx", Args: []string{"markitdown-mcp"}}},
		{name: "antv-chart", spec: Spec{Name: "antv-chart", Command: "cmd", Args: []string{"/c", "npx", "-y", "@antv/mcp-server-chart"}}},
		{name: "office-mcp", spec: Spec{
			Name:    "office-mcp",
			Command: "uvx",
			Args: []string{
				"--from",
				"git+https://github.com/gawirable/office-mcp@cf210cb6581f33cf3ceb57cf1defff4fc58d050b",
				"office-mcp",
			},
			Env: map[string]string{
				"PYTHONUTF8":                "1",
				"PYTHONIOENCODING":          "utf-8",
				"OFFICE_MCP_DEFAULT_FOLDER": workspaceRoot,
			},
		}},
		{name: "filesystem", spec: Spec{Name: "filesystem", Command: "cmd", Args: []string{"/c", "npx", "-y", "@modelcontextprotocol/server-filesystem", workspaceRoot}}},
		{name: "fetch", spec: Spec{Name: "fetch", Command: "uvx", Args: []string{"mcp-server-fetch"}}},
		{name: "time", spec: Spec{Name: "time", Command: "uvx", Args: []string{"mcp-server-time"}}},
		{name: "memory", spec: Spec{
			Name:    "memory",
			Command: "cmd",
			Args:    []string{"/c", "npx", "-y", "@modelcontextprotocol/server-memory"},
			Env:     map[string]string{"MEMORY_FILE_PATH": memoryFile},
		}},
		{name: "git", spec: Spec{Name: "git", Command: "uvx", Args: []string{"mcp-server-git", "--repository", workspaceRoot}}},
		{name: "sequential-thinking", spec: Spec{Name: "sequential-thinking", Command: "cmd", Args: []string{"/c", "npx", "-y", "@modelcontextprotocol/server-sequential-thinking"}}},
		{name: "mcp-docs", spec: Spec{Name: "mcp-docs", Type: "http", URL: "https://modelcontextprotocol.io/mcp"}},
		{name: "chrome-devtools", spec: Spec{Name: "chrome-devtools", Command: "cmd", Args: []string{"/c", "npx", "-y", "chrome-devtools-mcp@latest"}}},
		{name: "playwright", spec: Spec{Name: "playwright", Command: "cmd", Args: []string{"/c", "npx", "-y", "@playwright/mcp@latest", "--headless"}}},
		{name: "wechat-devtools", requiresWeChat: true, spec: Spec{
			Name:    "wechat-devtools",
			Command: "uvx",
			Args:    []string{"wechat-devtools-mcp==0.9.10"},
			Env: map[string]string{
				"WECHAT_DEVTOOLS_CLI": wechatCLI,
				"WECHAT_PROJECT_PATH": workspaceRoot,
				"PYTHONIOENCODING":    "utf-8",
			},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.requiresWeChat && wechatCLI == "" {
				t.Skip("微信开发者工具 CLI 未安装，不能进行真实握手")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			host, tools, err := StartAll(ctx, []Spec{test.spec})
			if err != nil {
				t.Fatalf("handshake/discovery failed: %v", err)
			}
			defer host.Close()
			if len(tools) == 0 {
				t.Fatal("connected without discovering tools")
			}
			t.Logf("connected: tools=%d", len(tools))
		})
	}
}

func integrationWechatDevtoolsCLI() string {
	if configured := os.Getenv("WECHAT_DEVTOOLS_CLI"); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured
		}
	}
	candidates := []string{}
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
			if root == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(root, "Tencent", "微信web开发者工具", "cli.bat"),
				filepath.Join(root, "Tencent", "微信开发者工具", "cli.bat"),
			)
		}
	} else if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/Applications/wechatwebdevtools.app/Contents/MacOS/cli")
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
