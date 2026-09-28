package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"kwu/internal/brand"
	"kwu/internal/fileutil"
	fileencoding "kwu/internal/fileutil/encoding"
)

var retiredMemoryCompilerMigrationMu sync.Mutex

// MigrateLegacyMemoryCompilerForRoot removes the retired
// [agent].memory_compiler key from the active profile and project configs.
func MigrateLegacyMemoryCompilerForRoot(root string, paths ProfilePathResolver) (bool, error) {
	root = resolveRoot(root)
	paths = paths.withWorkspaceRoot(root)
	candidates := []string{paths.UserConfigLoadPath(), projectConfigPathForRoot(root)}
	seen := make(map[string]struct{}, len(candidates))
	changedAny := false
	var migrateErrs []error
	for _, path := range candidates {
		if strings.TrimSpace(path) == "" {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		changed, err := migrateLegacyMemoryCompilerFile(clean)
		if err != nil {
			migrateErrs = append(migrateErrs, fmt.Errorf("migrate deprecated memory_compiler in %s: %w", clean, err))
			continue
		}
		changedAny = changedAny || changed
	}
	return changedAny, errors.Join(migrateErrs...)
}

func migrateLegacyMemoryCompilerFile(path string) (bool, error) {
	retiredMemoryCompilerMigrationMu.Lock()
	defer retiredMemoryCompilerMigrationMu.Unlock()

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	encodingKind, content := fileencoding.Detect(raw)
	next, changed := stripLegacyMemoryCompilerLines(string(fileencoding.Decode(content, encodingKind)))
	if !changed {
		return false, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to rewrite symlinked config %s", path)
	}
	if err := fileutil.AtomicWriteFile(path, fileencoding.Encode(next, encodingKind), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

func stripLegacyMemoryCompilerLines(raw string) (string, bool) {
	lines := strings.Split(raw, "\n")
	currentSection := ""
	state := retiredTOMLOutside
	changed := false
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if state != retiredTOMLOutside {
			out = append(out, line)
			state = advanceRetiredTOMLStringState(state, line)
			continue
		}
		if header := retiredTOMLSectionHeader(line); header != "" {
			currentSection = header
		}
		next := advanceRetiredTOMLStringState(retiredTOMLOutside, line)
		if currentSection == "agent" && next == retiredTOMLOutside && retiredTOMLKeyAssignment(line, "memory_compiler") {
			changed = true
			continue
		}
		out = append(out, line)
		state = next
	}
	return strings.Join(out, "\n"), changed
}

type retiredTOMLStringState int

const (
	retiredTOMLOutside retiredTOMLStringState = iota
	retiredTOMLMultilineBasic
	retiredTOMLMultilineLiteral
)

func advanceRetiredTOMLStringState(state retiredTOMLStringState, line string) retiredTOMLStringState {
	switch state {
	case retiredTOMLMultilineBasic:
		return scanRetiredTOMLMultilineBasic(line)
	case retiredTOMLMultilineLiteral:
		return scanRetiredTOMLMultilineLiteral(line)
	default:
		return scanRetiredTOMLOutside(line)
	}
}

func scanRetiredTOMLMultilineBasic(line string) retiredTOMLStringState {
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if strings.HasPrefix(line[i:], `"""`) {
			return scanRetiredTOMLOutside(line[i+3:])
		}
	}
	return retiredTOMLMultilineBasic
}

func scanRetiredTOMLMultilineLiteral(line string) retiredTOMLStringState {
	if i := strings.Index(line, "'''"); i >= 0 {
		return scanRetiredTOMLOutside(line[i+3:])
	}
	return retiredTOMLMultilineLiteral
}

func scanRetiredTOMLOutside(line string) retiredTOMLStringState {
	for i := 0; i < len(line); {
		switch {
		case line[i] == '#':
			return retiredTOMLOutside
		case strings.HasPrefix(line[i:], `"""`):
			return scanRetiredTOMLMultilineBasic(line[i+3:])
		case strings.HasPrefix(line[i:], "'''"):
			return scanRetiredTOMLMultilineLiteral(line[i+3:])
		case line[i] == '"':
			i = skipRetiredTOMLBasicString(line, i+1)
		case line[i] == '\'':
			i = skipRetiredTOMLLiteralString(line, i+1)
		default:
			i++
		}
	}
	return retiredTOMLOutside
}

func skipRetiredTOMLBasicString(line string, i int) int {
	for i < len(line) && line[i] != '"' {
		if line[i] == '\\' {
			i++
		}
		i++
	}
	if i < len(line) {
		i++
	}
	return i
}

func skipRetiredTOMLLiteralString(line string, i int) int {
	for i < len(line) && line[i] != '\'' {
		i++
	}
	if i < len(line) {
		i++
	}
	return i
}

func retiredTOMLSectionHeader(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return ""
	}
	if i := strings.Index(trimmed, "#"); i >= 0 {
		trimmed = strings.TrimSpace(trimmed[:i])
	}
	if strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]") {
		return strings.TrimSpace(trimmed[2 : len(trimmed)-2])
	}
	if strings.HasSuffix(trimmed, "]") {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	return "other"
}

func retiredTOMLKeyAssignment(line, key string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, key) {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
	return strings.HasPrefix(rest, "=")
}

// legacyConfig is the subset of the v0.x (~/.kwu/config.json) schema this
// import carries forward. Fields absent here are dropped on purpose: desktop tab
// state is frontend-owned, and skills already live in the shared ~/.kwu/skills
// root that v1+ also scans, so they need no migration.
type legacyConfig struct {
	APIKey      string                       `json:"apiKey"`
	BaseURL     string                       `json:"baseUrl"`
	Model       string                       `json:"model"`
	Lang        string                       `json:"lang"`
	MCP         []string                     `json:"mcp"` // pre-mcpServers `--mcp`-format strings
	MCPServers  map[string]legacyMCPServer   `json:"mcpServers"`
	MCPEnv      map[string]map[string]string `json:"mcpEnv"`
	MCPDisabled []string                     `json:"mcpDisabled"`
	QQ          legacyQQConfig               `json:"qq"`
}

type legacyMCPServer struct {
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Transport string            `json:"transport"`
	Type      string            `json:"type"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Disabled  bool              `json:"disabled"`
}

type legacyQQConfig struct {
	AppID       string   `json:"appId"`
	AppSecret   string   `json:"appSecret"`
	Sandbox     bool     `json:"sandbox"`
	Enabled     bool     `json:"enabled"`
	OwnerOpenID string   `json:"ownerOpenId"`
	Allowlist   []string `json:"allowlist"`
}

// MigrationResult summarizes a one-time legacy import for the boot-time notice.
type MigrationResult struct {
	From     string
	To       string
	KeyToEnv bool
	Plugins  int
	Warnings []string
}

// MCPGlobalMigrationResult summarizes the v1.9.1 MCP backfill that lifts MCP
// servers from legacy and project-local sources into the user-global config.
type MCPGlobalMigrationResult struct {
	To      string
	Added   int
	Sources int
}

// Go test binaries must never migrate the developer's real profile. Migration
// tests enable this switch explicitly after redirecting all profile paths.
var automaticUserDataMigrationEnabled = !runningUnderGoTest()

// AutomaticUserDataMigrationAllowed reports whether startup may inspect and
// import the standard legacy user-data roots. Explicit home/state overrides and
// Go test processes must remain isolated from the real OS profile.
func AutomaticUserDataMigrationAllowed() bool {
	return !canonicalUserDataOnlyEnabled() &&
		automaticUserDataMigrationEnabled &&
		!hasHomeDirOverride() &&
		!hasStateDirOverride()
}

var (
	automaticUserDataMigrationMu       sync.Mutex
	automaticUserDataMigrationAttempts = map[string]bool{}
)

func runningUnderGoTest() bool {
	name := strings.ToLower(filepath.Base(os.Args[0]))
	return strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe")
}

func (r *MigrationResult) Notice() string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrated your previous configuration and data: %s → %s", r.From, r.To)
	if r.Plugins > 0 {
		fmt.Fprintf(&b, " (%d MCP server(s))", r.Plugins)
	}
	if r.KeyToEnv {
		b.WriteString("; API key saved to " + brand.DisplayName + " credentials store")
	}
	b.WriteString(". The old files were left untouched.")
	for _, w := range r.Warnings {
		b.WriteString("\n  note: " + w)
	}
	return b.String()
}

// MigrateLegacyIfNeeded copies the first existing KWU/SuiYuan/Reasonix
// user-data root into the canonical Clouvia location without deleting it.
func MigrateLegacyIfNeeded() (*MigrationResult, error) {
	return MigrateLegacyIfNeededForRoot(".")
}

func MigrateLegacyIfNeededForRoot(root string) (*MigrationResult, error) {
	_ = root // Project files are read in place; only the user-global root moves.
	if !AutomaticUserDataMigrationAllowed() {
		return nil, nil
	}
	dest := canonicalHomeDir()
	if dest == "" {
		return nil, nil
	}
	automaticUserDataMigrationMu.Lock()
	defer automaticUserDataMigrationMu.Unlock()
	ready, err := migrationDestinationReady(dest)
	if err != nil || ready {
		return nil, err
	}

	src := firstLegacyDataRoot()
	if src == "" {
		return nil, nil
	}
	if !markAutomaticMigrationAttempt(dest) {
		return nil, nil
	}
	return migrateLegacyDataRoot(src, dest)
}

func migrationDestinationReady(dest string) (bool, error) {
	info, err := os.Stat(dest)
	if err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("Clouvia data path is not a directory: %s", dest)
		}
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("inspect Clouvia data path %s: %w", dest, err)
}

func firstLegacyDataRoot() string {
	for _, candidate := range legacyHomeDirs() {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func markAutomaticMigrationAttempt(dest string) bool {
	key := filepath.Clean(dest)
	if automaticUserDataMigrationAttempts[key] {
		return false
	}
	automaticUserDataMigrationAttempts[key] = true
	return true
}

func migrateLegacyDataRoot(src, dest string) (*MigrationResult, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, fmt.Errorf("create Clouvia data parent: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dest), ".Clouvia.migrating-*")
	if err != nil {
		return nil, fmt.Errorf("create Clouvia migration directory: %w", err)
	}
	defer os.RemoveAll(tmp)

	warnings, err := copyDataRoot(src, tmp)
	if err != nil {
		return nil, fmt.Errorf("copy legacy data from %s: %w", src, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		// Another startup may have completed the same migration first.
		if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
			return nil, nil
		}
		return nil, fmt.Errorf("activate Clouvia data path %s: %w", dest, err)
	}
	return &MigrationResult{From: src, To: dest, Warnings: warnings}, nil
}

func MigrateLegacyCredentialsForRoot(root string) error {
	return nil
}

// MigrateMCPToUserConfigOnUpgrade is kept for older callers. New KWU
// development builds do not backfill MCP settings from previous layouts.
func MigrateMCPToUserConfigOnUpgrade(projectRoots []string) (*MCPGlobalMigrationResult, error) {
	return nil, nil
}

func migrateMCPToUserConfig(projectRoots []string) (*MCPGlobalMigrationResult, error) {
	dest := userConfigPath()
	if dest == "" {
		return nil, nil
	}
	userCfg, err := loadForEditStrict(dest, true)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(userCfg.Plugins))
	for _, p := range userCfg.Plugins {
		if name := strings.TrimSpace(p.Name); name != "" {
			have[name] = true
		}
	}

	result := &MCPGlobalMigrationResult{To: dest}
	addEntries := func(entries []PluginEntry) {
		if len(entries) == 0 {
			return
		}
		result.Sources++
		for _, entry := range entries {
			entry, _ = NormalizePluginCommandLine(entry)
			name := strings.TrimSpace(entry.Name)
			if name == "" || have[name] || validatePlugin(entry) != nil {
				continue
			}
			userCfg.Plugins = append(userCfg.Plugins, entry)
			have[name] = true
			result.Added++
		}
	}

	home, _ := os.UserHomeDir()
	for _, path := range mcpMigrationLegacyTOMLPaths(dest, home) {
		addEntries(loadPluginEntriesFromTOML(path))
	}
	for _, root := range normalizedMCPMigrationRoots(projectRoots) {
		addEntries(loadPluginEntriesFromTOML(filepath.Join(root, "kwu.toml")))
		if entries, err := loadMCPJSON(filepath.Join(root, mcpJSONFile)); err == nil {
			addEntries(entries)
		}
	}
	addEntries(loadLegacyConfigPlugins(legacyConfigPath()))

	if result.Sources == 0 {
		return nil, nil
	}
	if result.Added > 0 {
		if err := userCfg.SaveTo(dest); err != nil {
			return result, err
		}
	}
	return result, nil
}

func mcpGlobalMigrationMarkerPath() string {
	dir := userSupportDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mcp-global-migration-v1")
}

func mcpMigrationLegacyTOMLPaths(dest, home string) []string {
	var paths []string
	for _, path := range legacyTOMLPaths(dest, home) {
		if path == "" || samePath(path, dest) {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func loadPluginEntriesFromTOML(path string) []PluginEntry {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil
	}
	out := make([]PluginEntry, 0, len(cfg.Plugins))
	for _, p := range cfg.Plugins {
		p, _ = NormalizePluginCommandLine(p)
		out = append(out, p)
	}
	return out
}

func loadLegacyConfigPlugins(path string) []PluginEntry {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var legacy legacyConfig
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil
	}
	return legacyPlugins(legacy)
}

func normalizedMCPMigrationRoots(roots []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		root = filepath.Clean(root)
		if seen[root] {
			continue
		}
		seen[root] = true
		out = append(out, root)
	}
	return out
}

func migrateLegacyCredentialsIfNeededForRoot(root string) error {
	missing := map[string]string{}
	skip := func(key string) bool {
		return credentialCurrentStoreHasKey(key) || credentialCurrentStoreClearedKey(key)
	}
	for _, key := range credentialEnvNamesForRoot(root) {
		if skip(key) {
			continue
		}
		if value, ok := legacyKeyringCredentialValueFallback(key); ok {
			missing[key] = value
		}
	}
	for _, src := range legacyCredentialsPaths() {
		if src == "" {
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		assignments := parseCredentialLines(strings.Split(string(data), "\n"))
		for key, value := range assignments {
			if _, exists := missing[key]; !exists && !skip(key) {
				missing[key] = value
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	_, err := StoreCredentialLines(credentialLines(missing))
	return err
}

func credentialLines(assignments map[string]string) []string {
	keys := make([]string, 0, len(assignments))
	for key := range assignments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+assignments[key])
	}
	return lines
}

func migrateLegacyQQConfig(cfg *Config, legacy legacyQQConfig) {
	if cfg == nil || !legacyQQConfigured(legacy) {
		return
	}
	cfg.Bot.Enabled = cfg.Bot.Enabled || legacy.Enabled
	cfg.Bot.QQ.Enabled = legacy.Enabled
	cfg.Bot.QQ.AppID = strings.TrimSpace(legacy.AppID)
	cfg.Bot.QQ.AppSecretEnv = "QQ_BOT_APP_SECRET"
	cfg.Bot.QQ.Sandbox = legacy.Sandbox
	cfg.Bot.Allowlist.Enabled = true
	cfg.Bot.Allowlist.QQUsers = mergeUniqueTrimmed(cfg.Bot.Allowlist.QQUsers, legacy.OwnerOpenID)
	cfg.Bot.Allowlist.QQUsers = mergeUniqueTrimmed(cfg.Bot.Allowlist.QQUsers, legacy.Allowlist...)
}

func legacyQQConfigured(legacy legacyQQConfig) bool {
	return legacy.Enabled ||
		strings.TrimSpace(legacy.AppID) != "" ||
		strings.TrimSpace(legacy.AppSecret) != "" ||
		strings.TrimSpace(legacy.OwnerOpenID) != "" ||
		len(legacy.Allowlist) > 0 ||
		legacy.Sandbox
}

func mergeUniqueTrimmed(base []string, values ...string) []string {
	seen := make(map[string]bool, len(base)+len(values))
	out := make([]string, 0, len(base)+len(values))
	for _, value := range append(base, values...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func migrateLegacyTOMLIfNeeded(dest, home string) (*MigrationResult, error) {
	for _, src := range legacyTOMLPaths(dest, home) {
		if src == "" || filepath.Clean(src) == filepath.Clean(dest) {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		cfg := Default()
		if err := mergeFile(cfg, src); err != nil {
			return nil, fmt.Errorf("parse legacy config %s: %w", src, err)
		}
		cfg.ConfigVersion = Default().ConfigVersion
		if strings.TrimSpace(cfg.Desktop.CloseBehavior) == "" && strings.TrimSpace(cfg.UI.CloseBehavior) != "" {
			cfg.Desktop.CloseBehavior = cfg.DesktopCloseBehavior()
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}
		if err := cfg.WriteFile(dest); err != nil {
			return nil, fmt.Errorf("write %s: %w", dest, err)
		}
		res := &MigrationResult{From: src, To: dest, Plugins: len(cfg.Plugins)}
		legacyDir := filepath.Dir(src)
		newDir := filepath.Dir(dest)
		if !samePath(legacyDir, newDir) {
			if warnings := migrateSupportData(legacyDir, newDir); len(warnings) > 0 {
				res.Warnings = append(res.Warnings, warnings...)
			}
		}
		return res, nil
	}
	return nil, nil
}

func legacyTOMLPaths(dest, home string) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	if legacy := legacyUserConfigPath(); legacy != "" {
		add(legacy)
	}
	for _, legacy := range legacyXDGConfigPaths() {
		add(legacy)
		add(filepath.Join(filepath.Dir(legacy), "kwu.toml"))
	}
	add(filepath.Join(filepath.Dir(dest), "kwu.toml"))
	if home != "" {
		add(filepath.Join(home, ".kwu", "kwu.toml"))
	}
	return paths
}

func migrateLegacyBaseURL(cfg *Config, baseURL string) {
	baseURL = strings.TrimSpace(baseURL)
	if cfg == nil || baseURL == "" {
		return
	}
	for i := range cfg.Providers {
		if cfg.Providers[i].APIKeyEnv == "DEEPSEEK_API_KEY" {
			cfg.Providers[i].BaseURL = baseURL
		}
	}
}

func legacyPlugins(legacy legacyConfig) []PluginEntry {
	disabled := make(map[string]bool, len(legacy.MCPDisabled))
	for _, n := range legacy.MCPDisabled {
		disabled[n] = true
	}
	var out []PluginEntry
	index := make(map[string]int)
	add := func(pe PluginEntry, off bool) {
		if off {
			v := false
			pe.AutoStart = &v
		}
		pe, _ = NormalizePluginCommandLine(pe)
		if j, dup := index[pe.Name]; dup {
			out[j] = pe // mcpServers overrides the `mcp` list on a name collision, matching v0.x
			return
		}
		index[pe.Name] = len(out)
		out = append(out, pe)
	}
	for i, raw := range legacy.MCP {
		pe, ok := parseLegacyMCPSpec(raw)
		if !ok {
			continue
		}
		if pe.Name == "" {
			pe.Name = anonymousMCPName(i)
		} else if pe.Command != "" {
			pe.Env = mergeEnv(nil, legacy.MCPEnv[pe.Name])
		}
		add(pe, disabled[pe.Name])
	}
	names := make([]string, 0, len(legacy.MCPServers))
	for n := range legacy.MCPServers {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		s := legacy.MCPServers[name]
		pe := PluginEntry{
			Name:    name,
			Type:    normalizeTransport(firstNonEmpty(s.Type, s.Transport)),
			Command: s.Command,
			Args:    s.Args,
			Env:     mergeEnv(s.Env, legacy.MCPEnv[name]),
			URL:     s.URL,
			Headers: s.Headers,
		}
		add(pe, s.Disabled || disabled[name])
	}
	return out
}

// normalizeTransport maps the v0.x transport names to v1+ plugin types. stdio is
// the default, so it returns "" (RenderTOML then omits the field).
func normalizeTransport(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "http", "streamable-http":
		return "http"
	case "sse":
		return "sse"
	default:
		return ""
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// mergeEnv overlays the per-server env map onto the spec's own env (overlay wins,
// matching v0.x mcpEnv precedence). Returns nil when both are empty.
func mergeEnv(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

// writeCredentialsEnv merges lines into Clouvia's global .env
// and pins them into the current process env so the just-built session resolves
// the key without a restart. Falls back to ~/.env only when Clouvia home can't
// be resolved — never a project .env, so a migration keeps secrets out of the
// user's project tree.
func writeCredentialsEnv(home string, lines []string) error {
	if _, err := StoreCredentialLines(lines); err != nil {
		if UserCredentialsPath() == "" && home != "" {
			return os.WriteFile(filepath.Join(home, ".env"), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
		}
		return err
	}
	return nil
}

func migrateSupportData(legacyDir, newDir string) []string {
	var warnings []string
	items := []string{"sessions", "projects", "skills", "archive"}
	for _, item := range items {
		src := filepath.Join(legacyDir, item)
		fi, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("failed to read legacy item %s: %v", item, err))
			continue
		}
		dst := filepath.Join(newDir, item)
		if fi.IsDir() {
			if err := copyDir(src, dst); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to migrate directory %s: %v", item, err))
			} else {
				warnings = append(warnings, fmt.Sprintf("successfully migrated directory %s", item))
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to migrate file %s: %v", item, err))
			} else {
				warnings = append(warnings, fmt.Sprintf("successfully migrated file %s", item))
			}
		}
	}
	return warnings
}
