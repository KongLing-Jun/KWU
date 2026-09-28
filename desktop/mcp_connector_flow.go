package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/plugin"
)

// MCPConnectionTestResult describes discovery counts and actionable connection
// diagnostics returned by MCP installation and connection tests.
type MCPConnectionTestResult struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	Phase             string `json:"phase"`
	LatencyMS         int64  `json:"latencyMs"`
	Tools             int    `json:"tools"`
	Resources         int    `json:"resources"`
	Prompts           int    `json:"prompts"`
	Error             string `json:"error,omitempty"`
	RepairAction      string `json:"repairAction,omitempty"`
	AlreadyConfigured bool   `json:"alreadyConfigured,omitempty"`
	SessionAttached   bool   `json:"sessionAttached,omitempty"`
}

// ComposerCapabilitySubmit is the structured capability selection for one
// composer turn.
type ComposerCapabilitySubmit struct {
	Text           string   `json:"text"`
	TitleText      *string  `json:"title_text,omitempty"`
	SkillNames     []string `json:"skill_names"`
	MCPServerNames []string `json:"mcp_server_names"`
}

var nonCredentialRune = regexp.MustCompile(`[^A-Z0-9_]+`)

const (
	mcpStatusConnected = "connected"
	mcpStatusFailed    = "failed"

	mcpPhaseDependency  = "dependency"
	mcpPhaseCredentials = "credentials"
	mcpPhaseComplete    = "complete"
	mcpPhaseDuplicate   = "duplicate"
	mcpPhaseDiscovery   = "discovery"
	mcpPhaseHandshake   = "handshake"
)

// InstallMCPServer runs dependency and credential checks, stores secrets,
// performs the MCP handshake and discovery, persists the configuration, and
// attaches the server to the active session.
func (a *App) InstallMCPServer(in MCPServerInput) (MCPConnectionTestResult, error) {
	if a.managedConnectorConfigName(in.Name) || a.managedConnectorConfigName(in.CatalogID) {
		return failedMCPConnectionResult(in.Name, mcpPhaseCredentials, errors.New("managed connector names cannot be installed as static MCP presets")), nil
	}
	tab := a.activeTab()
	if tab == nil || tab.Ctrl == nil {
		return MCPConnectionTestResult{}, fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return MCPConnectionTestResult{}, rebuildControllerActiveWorkError("MCP server")
	}
	if err := validateCatalogMCPInput(in); err != nil {
		return failedMCPConnectionResult(strings.TrimSpace(in.Name), mcpPhaseCredentials, err), nil
	}
	var err error
	in, err = a.resolveCatalogMCPInput(in)
	if err != nil {
		return failedMCPConnectionResult(strings.TrimSpace(in.Name), mcpPhaseDependency, err), nil
	}
	if err := validateCatalogMCPRuntimeInput(in); err != nil {
		return failedMCPConnectionResult(strings.TrimSpace(in.Name), mcpPhaseDependency, err), nil
	}
	if err := ensureCatalogMCPRuntime(in); err != nil {
		return failedMCPConnectionResult(strings.TrimSpace(in.Name), mcpPhaseDependency, err), nil
	}

	entry := mcpEntryFromInput(in)
	entry, _ = config.NormalizePluginCommandLine(entry)
	if err := preflightMCPDependency(entry); err != nil {
		return failedMCPConnectionResult(entry.Name, mcpPhaseDependency, err), nil
	}
	if err := validateMCPRequiredCredentials(in); err != nil {
		return failedMCPConnectionResult(entry.Name, mcpPhaseCredentials, err), nil
	}

	if existing, found, err := a.desktopMCPServerForEdit(entry.Name); err != nil {
		return MCPConnectionTestResult{}, err
	} else if found {
		result := currentMCPConnectionResult(tab.Ctrl, existing.Name, 0)
		result.AlreadyConfigured = true
		result.SessionAttached = result.Status == mcpStatusConnected
		if result.Status != mcpStatusConnected {
			result.Status = mcpStatusFailed
			result.Phase = mcpPhaseDuplicate
			result.Error = fmt.Sprintf("连接器 %s 已配置，但当前不可用", existing.Name)
			result.RepairAction = "打开配置页修正后点击“测试连接”"
		}
		return result, nil
	}

	secured, err := a.secureMCPServerInput(in)
	if err != nil {
		return failedMCPConnectionResult(entry.Name, mcpPhaseCredentials, err), nil
	}
	entry = mcpEntryFromInput(secured)
	entry, _ = config.NormalizePluginCommandLine(entry)

	started := time.Now()
	if _, err := tab.Ctrl.ConnectMCPServer(entry); err != nil {
		recordMCPFailure(tab.Ctrl, entry, err)
		return failedMCPConnectionResultWithLatency(entry, mcpPhaseHandshake, time.Since(started), err), nil
	}
	result := currentMCPConnectionResult(tab.Ctrl, entry.Name, time.Since(started))
	if result.Status != mcpStatusConnected {
		tab.Ctrl.DisconnectMCPServer(entry.Name)
		return result, nil
	}
	if err := a.saveDesktopMCPServer(entry); err != nil {
		tab.Ctrl.DisconnectMCPServer(entry.Name)
		return MCPConnectionTestResult{}, err
	}
	a.mu.Lock()
	delete(tab.disabledMCP, entry.Name)
	if !containsMCPName(tab.mcpOrder, entry.Name) {
		tab.mcpOrder = append(tab.mcpOrder, entry.Name)
	}
	a.mu.Unlock()
	result.SessionAttached = true
	return result, nil
}

func containsMCPName(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

var noAuthMCPConnectorIDs = map[string]struct{}{
	"antv-chart":          {},
	"chrome-devtools":     {},
	"fetch":               {},
	"filesystem":          {},
	"git":                 {},
	"markitdown":          {},
	"mcp-docs":            {},
	"memory":              {},
	"office-mcp":          {},
	"playwright":          {},
	"sequential-thinking": {},
	"time":                {},
	"wechat-devtools":     {},
}

const (
	mcpWorkspaceRootToken = "{{workspaceRoot}}"
	mcpDataRootToken      = "{{mcpDataRoot}}"
	mcpWechatCLIToken     = "{{wechatDevtoolsCli}}"
)

func (a *App) resolveCatalogMCPInput(in MCPServerInput) (MCPServerInput, error) {
	if strings.TrimSpace(in.CatalogID) == "" {
		return in, nil
	}
	workspaceRoot := strings.TrimSpace(a.activeWorkspaceRoot())
	if workspaceRoot == "" {
		workspaceRoot = "."
	}
	absoluteRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return in, fmt.Errorf("无法解析当前工作区路径: %w", err)
	}
	dataRoot := filepath.Join(a.activeProfilePathResolver().KWUHomeDir(), "mcp-data")
	return resolveCatalogMCPInputForEnvironment(in, absoluteRoot, dataRoot, findWechatDevtoolsCLI()), nil
}

func resolveCatalogMCPInputForEnvironment(in MCPServerInput, workspaceRoot, dataRoot, wechatCLI string) MCPServerInput {
	replace := func(value string) string {
		value = strings.ReplaceAll(value, mcpWorkspaceRootToken, workspaceRoot)
		value = strings.ReplaceAll(value, mcpDataRootToken, dataRoot)
		value = strings.ReplaceAll(value, mcpWechatCLIToken, wechatCLI)
		return value
	}
	in.Command = replace(in.Command)
	in.WorkDir = replace(in.WorkDir)
	in.Args = append([]string(nil), in.Args...)
	for index := range in.Args {
		in.Args[index] = replace(in.Args[index])
	}
	in.Env = cloneStringMap(in.Env)
	for key, value := range in.Env {
		in.Env[key] = replace(value)
	}
	return in
}

func findWechatDevtoolsCLI() string {
	candidates := make([]string, 0, 5)
	if runtime.GOOS == "windows" {
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
			if strings.TrimSpace(root) == "" {
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

func validateCatalogMCPRuntimeInput(in MCPServerInput) error {
	catalogID := strings.TrimSpace(in.CatalogID)
	if catalogID == "" {
		return nil
	}
	for _, value := range append([]string{in.Command, in.WorkDir}, in.Args...) {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return fmt.Errorf("连接器 %s 仍包含未解析的运行环境占位符", catalogID)
		}
	}
	for key, value := range in.Env {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return fmt.Errorf("连接器 %s 的 %s 仍包含未解析的运行环境占位符", catalogID, key)
		}
	}
	requireDirectory := func(label, path string) error {
		path = strings.TrimSpace(path)
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("%s 不存在或不是目录: %s", label, path)
		}
		return nil
	}
	switch catalogID {
	case "filesystem":
		if len(in.Args) == 0 {
			return fmt.Errorf("Filesystem MCP 缺少授权目录")
		}
		return requireDirectory("Filesystem MCP 授权目录", in.Args[len(in.Args)-1])
	case "git":
		if len(in.Args) == 0 {
			return fmt.Errorf("Git MCP 缺少仓库路径")
		}
		repository := in.Args[len(in.Args)-1]
		if err := requireDirectory("Git MCP 仓库", repository); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, "git", "-C", repository, "rev-parse", "--show-toplevel").CombinedOutput(); err != nil {
			return fmt.Errorf("Git MCP 路径不是可用的 Git 仓库: %s", strings.TrimSpace(string(output)))
		}
	case "office-mcp":
		return requireDirectory("office-mcp 默认目录", in.Env["OFFICE_MCP_DEFAULT_FOLDER"])
	case "wechat-devtools":
		projectPath := strings.TrimSpace(in.Env["WECHAT_PROJECT_PATH"])
		if err := requireDirectory("微信小程序项目目录", projectPath); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(projectPath, "project.config.json")); err != nil {
			return fmt.Errorf("当前工作区不是微信小程序根目录，缺少 project.config.json: %s", projectPath)
		}
		cli := strings.TrimSpace(in.Env["WECHAT_DEVTOOLS_CLI"])
		if info, err := os.Stat(cli); cli == "" || err != nil || info.IsDir() {
			return fmt.Errorf("未找到微信开发者工具 CLI；请先安装微信开发者工具，或填写 WECHAT_DEVTOOLS_CLI")
		}
	}
	return nil
}

func ensureCatalogMCPRuntime(in MCPServerInput) error {
	if strings.TrimSpace(in.CatalogID) != "memory" {
		return nil
	}
	memoryFile := strings.TrimSpace(in.Env["MEMORY_FILE_PATH"])
	if memoryFile == "" {
		return fmt.Errorf("Memory MCP 持久化文件路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(memoryFile), 0o700); err != nil {
		return fmt.Errorf("创建 Memory MCP 数据目录失败: %w", err)
	}
	return nil
}

// validateCatalogMCPInput enforces the product catalog contract at the desktop
// boundary. Custom MCP entries remain generic so authenticated integrations can
// be added later without weakening today's no-auth catalog.
func validateCatalogMCPInput(in MCPServerInput) error {
	catalogID := strings.TrimSpace(in.CatalogID)
	if catalogID == "" {
		return nil
	}
	if _, ok := noAuthMCPConnectorIDs[catalogID]; !ok {
		return fmt.Errorf("unknown MCP catalog connector %q", catalogID)
	}
	if strings.TrimSpace(in.AuthMode) != "none" {
		return fmt.Errorf("MCP catalog authentication mode %q is not enabled", in.AuthMode)
	}
	if len(in.SecretEnv) > 0 || len(in.SecretHeaders) > 0 ||
		len(in.RequiredEnvKeys) > 0 || len(in.RequiredHeaderKeys) > 0 ||
		len(in.Headers) > 0 {
		return fmt.Errorf("no-auth MCP catalog connector %q does not accept credentials", catalogID)
	}
	for key := range in.Env {
		upper := strings.ToUpper(strings.TrimSpace(key))
		if strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "API_KEY") ||
			strings.Contains(upper, "SECRET") ||
			strings.Contains(upper, "PASSWORD") ||
			strings.Contains(upper, "AUTHORIZATION") ||
			strings.Contains(upper, "CREDENTIAL") {
			return fmt.Errorf("no-auth MCP catalog connector %q does not accept credential environment variable %q", catalogID, key)
		}
	}
	return nil
}

// TestMCPServer performs a fresh handshake and discovery without deleting the
// saved configuration when the test fails.
func (a *App) TestMCPServer(name string) (MCPConnectionTestResult, error) {
	tab := a.activeTab()
	if tab == nil || tab.Ctrl == nil {
		return MCPConnectionTestResult{}, fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return MCPConnectionTestResult{}, rebuildControllerActiveWorkError("MCP server")
	}
	entry, found, err := a.desktopMCPServerForEdit(strings.TrimSpace(name))
	if err != nil {
		return MCPConnectionTestResult{}, err
	}
	if !found {
		return MCPConnectionTestResult{}, fmt.Errorf("no configured MCP server named %q", name)
	}
	if err := preflightMCPDependency(entry); err != nil {
		return failedMCPConnectionResult(entry.Name, mcpPhaseDependency, err), nil
	}

	tab.Ctrl.DisconnectMCPServer(entry.Name)
	if host := tab.Ctrl.Host(); host != nil {
		host.ClearFailure(entry.Name)
	}
	started := time.Now()
	if _, err := a.connectConfiguredMCPServerForTab(tab, entry.Name); err != nil {
		recordMCPFailure(tab.Ctrl, entry, err)
		return failedMCPConnectionResultWithLatency(entry, mcpPhaseHandshake, time.Since(started), err), nil
	}
	a.mu.Lock()
	delete(tab.disabledMCP, entry.Name)
	a.mu.Unlock()
	return currentMCPConnectionResult(tab.Ctrl, entry.Name, time.Since(started)), nil
}

// SubmitCapabilitiesToTab validates selected skills and live MCP servers before
// asynchronously starting a structured turn on the requested tab.
func (a *App) SubmitCapabilitiesToTab(tabID, display string, in ComposerCapabilitySubmit) error {
	return a.submitCapabilitiesToTab(tabID, display, in)
}

func (a *App) submitCapabilitiesToTab(tabID, display string, in ComposerCapabilitySubmit) error {
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return workspaceNotReadyErr(tab)
	}
	if err := a.chatEntryPointModelError(tab, in.Text); err != nil {
		return err
	}
	capabilityID := ""
	if scope, ok := a.capabilityTaskScopesByTabID()[tabID]; ok {
		capabilityID = scope.CapabilityID
		builtInServers, err := ensureCapabilityMCPRuntime(context.Background(), capabilityID, ctrl)
		if err != nil {
			return err
		}
		in.MCPServerNames = append(in.MCPServerNames, builtInServers...)
	}
	skills, err := validateSelectedSkills(ctrl, in.SkillNames)
	if err != nil {
		return err
	}
	expansion, err := a.expandConnectorSelections(tabID, ctrl, in.MCPServerNames, skills, in.Text)
	if err != nil {
		return err
	}
	servers, err := a.ensureSelectedMCPServers(tab, ctrl, expansion.MCPNames)
	if err != nil {
		return err
	}
	submitter, ok := ctrl.(interface {
		PrepareCapabilities(context.Context, control.CapabilityTurn) (control.CapabilityTurn, error)
		SubmitCapabilities(control.CapabilityTurn)
	})
	if !ok {
		return fmt.Errorf("structured capability submission is unavailable")
	}
	input, err := capabilitySubmitContract(capabilityID, expansion.Input)
	if err != nil {
		return err
	}
	turn, err := submitter.PrepareCapabilities(context.Background(), control.CapabilityTurn{
		Input:           input,
		Display:         display,
		TitleText:       stringPointerValue(in.TitleText),
		PreferTitleText: in.TitleText != nil,
		SkillNames:      skills,
		MCPServerNames:  servers,
		CommandEnv:      expansion.CommandEnv,
	})
	if err != nil {
		return err
	}
	submitter.SubmitCapabilities(turn)
	return nil
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (a *App) ensureSelectedMCPServers(tab *WorkspaceTab, ctrl control.SessionAPI, names []string) ([]string, error) {
	out := uniqueStrings(names)
	if len(out) == 0 {
		return out, nil
	}
	if tab == nil || ctrl == nil {
		return nil, fmt.Errorf("workspace is not ready")
	}
	connected := connectedMCPToolCounts(ctrl)
	for _, name := range out {
		if tools, ok := connected[name]; ok {
			if tools <= 0 {
				return nil, fmt.Errorf("MCP 连接器 %q 已连接，但没有发现可用工具；请检查服务端配置", name)
			}
			continue
		}
		a.mu.RLock()
		_, disabled := tab.disabledMCP[name]
		a.mu.RUnlock()
		if disabled {
			return nil, fmt.Errorf("MCP 连接器 %q 已停用；请先启用后重试", name)
		}

		if host := ctrl.Host(); host != nil {
			host.ClearFailure(name)
		}
		if _, err := a.connectConfiguredMCPServerForTab(tab, name); err != nil && !plugin.IsServerAlreadyConnected(err) {
			entry := config.PluginEntry{Name: name}
			if configured, found, configErr := a.desktopMCPServerForEdit(name); configErr == nil && found {
				entry = configured
			}
			recordMCPFailure(ctrl, entry, err)
			return nil, fmt.Errorf("MCP 连接器 %q 重新连接失败：%w", name, err)
		}
		connected = connectedMCPToolCounts(ctrl)
		if connected[name] <= 0 {
			return nil, fmt.Errorf("MCP 连接器 %q 重新连接后没有发现可用工具；请测试连接并检查服务端配置", name)
		}
	}
	return out, nil
}

func connectedMCPToolCounts(ctrl control.SessionAPI) map[string]int {
	connected := map[string]int{}
	if ctrl == nil || ctrl.Host() == nil {
		return connected
	}
	for _, status := range ctrl.Host().Servers() {
		connected[status.Name] = status.Tools
	}
	return connected
}

func validateSelectedSkills(ctrl control.SessionAPI, names []string) ([]string, error) {
	available := map[string]bool{}
	for _, skill := range ctrl.Skills() {
		available[skill.Name] = true
	}
	out := uniqueStrings(names)
	for _, name := range out {
		if !available[name] {
			return nil, fmt.Errorf("技能 %q 不存在或未启用；请在技能设置中启用后重试", name)
		}
	}
	return out, nil
}

func mcpEntryFromInput(in MCPServerInput) config.PluginEntry {
	return config.PluginEntry{
		Name:                 strings.TrimSpace(in.Name),
		Type:                 normalizeMCPTransport(in.Transport),
		Command:              strings.TrimSpace(in.Command),
		Args:                 append([]string(nil), in.Args...),
		WorkDir:              strings.TrimSpace(in.WorkDir),
		URL:                  strings.TrimSpace(in.URL),
		Env:                  cloneStringMap(in.Env),
		Headers:              cloneStringMap(in.Headers),
		TrustedReadOnlyTools: uniqueStrings(in.TrustedReadOnlyTools),
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (a *App) secureMCPServerInput(in MCPServerInput) (MCPServerInput, error) {
	in.Env = cloneStringMap(in.Env)
	in.Headers = cloneStringMap(in.Headers)
	if in.Env == nil && len(in.SecretEnv) > 0 {
		in.Env = map[string]string{}
	}
	if in.Headers == nil && len(in.SecretHeaders) > 0 {
		in.Headers = map[string]string{}
	}
	store := func(field, value string) (string, error) {
		key := mcpCredentialKey(in.Name, field)
		if _, err := config.SetCredentialWithPaths(a.activeProfilePathResolver(), key, value); err != nil {
			return "", fmt.Errorf("保存 %s 凭据失败: %w", field, err)
		}
		return "${" + key + "}", nil
	}
	for field, value := range in.SecretEnv {
		if strings.TrimSpace(value) == "" {
			continue
		}
		ref, err := store(field, value)
		if err != nil {
			return in, err
		}
		in.Env[field] = ref
	}
	for field, value := range in.SecretHeaders {
		if strings.TrimSpace(value) == "" {
			continue
		}
		ref, err := store(field, value)
		if err != nil {
			return in, err
		}
		in.Headers[field] = ref
	}
	in.SecretEnv = nil
	in.SecretHeaders = nil
	return in, nil
}

func mcpCredentialKey(server, field string) string {
	raw := strings.ToUpper(strings.TrimSpace(server) + "_" + strings.TrimSpace(field))
	raw = nonCredentialRune.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "_")
	return "KWU_MCP_" + raw
}

func validateMCPRequiredCredentials(in MCPServerInput) error {
	for _, key := range uniqueStrings(in.RequiredEnvKeys) {
		value := strings.TrimSpace(in.SecretEnv[key])
		if value == "" {
			value = strings.TrimSpace(config.ExpandVars(in.Env[key]))
		}
		if value == "" {
			return fmt.Errorf("%s 为空", key)
		}
	}
	for _, key := range uniqueStrings(in.RequiredHeaderKeys) {
		value := strings.TrimSpace(in.SecretHeaders[key])
		if value == "" {
			value = strings.TrimSpace(config.ExpandVars(in.Headers[key]))
		}
		if value == "" {
			return fmt.Errorf("%s 为空", key)
		}
	}
	return nil
}

func preflightMCPDependency(entry config.PluginEntry) error {
	entry = entry.ExpandedPlugin()
	if entry.Type == "http" || entry.Type == "sse" {
		parsed, err := url.ParseRequestURI(strings.TrimSpace(entry.URL))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("HTTP MCP URL 无效")
		}
		return nil
	}
	command := strings.TrimSpace(entry.Command)
	if command == "" {
		return fmt.Errorf("MCP 启动命令为空")
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("%s 未安装", command)
	}
	if strings.EqualFold(strings.TrimSuffix(command, ".exe"), "docker") {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		check := exec.CommandContext(ctx, path, "info", "--format", "{{.ServerVersion}}")
		if output, checkErr := check.CombinedOutput(); checkErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("Docker 未运行（检查超时）")
			}
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = checkErr.Error()
			}
			return fmt.Errorf("Docker 未运行: %s", detail)
		}
	}
	return nil
}

func currentMCPConnectionResult(ctrl control.SessionAPI, name string, latency time.Duration) MCPConnectionTestResult {
	result := MCPConnectionTestResult{Name: name, Status: mcpStatusFailed, Phase: mcpPhaseDiscovery, LatencyMS: latency.Milliseconds()}
	if host := ctrl.Host(); host != nil {
		for _, status := range host.Servers() {
			if status.Name != name {
				continue
			}
			result.Tools = status.Tools
			result.Resources = status.Resources
			result.Prompts = status.Prompts
			if status.Tools > 0 {
				result.Status = mcpStatusConnected
				result.Phase = mcpPhaseComplete
			} else {
				result.Error = "MCP 握手成功，但未发现任何工具"
				result.RepairAction = "确认服务实现 tools/list，并检查服务端日志"
			}
			return result
		}
	}
	result.Error = "MCP 客户端未保持连接"
	result.RepairAction = "检查 MCP 进程是否在握手后退出，然后点击“测试连接”"
	return result
}

func failedMCPConnectionResult(name, phase string, err error) MCPConnectionTestResult {
	return failedMCPConnectionResultWithLatency(config.PluginEntry{Name: name}, phase, 0, err)
}

func failedMCPConnectionResultWithLatency(entry config.PluginEntry, phase string, latency time.Duration, err error) MCPConnectionTestResult {
	message, action := explainMCPConnectionFailure(entry, err)
	return MCPConnectionTestResult{
		Name: entry.Name, Status: mcpStatusFailed, Phase: phase, LatencyMS: latency.Milliseconds(),
		Error: message, RepairAction: action,
	}
}

func explainMCPConnectionFailure(entry config.PluginEntry, err error) (string, string) {
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return "服务返回 401", "重新授权或更新 Token，然后点击“测试连接”"
	case strings.Contains(lower, "oauth") || strings.Contains(lower, "authorization"):
		return "OAuth 尚未授权", "点击“授权”完成 OAuth 后再测试连接"
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout"):
		return "MCP 握手超时", "确认服务可启动且网络可达，然后重试"
	case strings.Contains(message, "未安装"):
		return message, "安装依赖并确认命令可在终端中执行"
	case strings.Contains(message, "Docker 未运行"):
		return message, "启动 Docker Desktop，等待引擎就绪后重试"
	case strings.Contains(message, "微信开发者工具 CLI"):
		return message, "安装微信开发者工具并启用 CLI/服务端口，然后重新安装连接器"
	case strings.Contains(message, "微信小程序根目录"):
		return message, "打开包含 project.config.json 的小程序项目后重试"
	case strings.Contains(message, "不存在或不是目录"):
		return message, "确认当前工作区和连接器授权目录存在，然后重试"
	case strings.Contains(message, "不是可用的 Git 仓库"):
		return message, "打开 Git 仓库根目录后重试"
	case strings.Contains(message, "为空"):
		return message, "补全凭据后重试；凭据不会写入普通连接器配置或日志"
	default:
		if strings.TrimSpace(entry.Command) != "" {
			return message, fmt.Sprintf("检查命令 %q、参数和工作目录，然后点击“测试连接”", entry.Command)
		}
		return message, "检查 URL、Headers、OAuth 与服务端日志，然后点击“测试连接”"
	}
}
