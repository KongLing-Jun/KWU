package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"kwu/internal/brand"
	"kwu/internal/userctx"
)

// canonicalUserDataOnly is enabled by the Windows and macOS desktop entry point
// before any profile or configuration initialization. Linux desktop and CLI
// entry points retain the compatibility behavior by default.
var canonicalUserDataOnly atomic.Bool

// UseCanonicalUserDataOnly disables former-product filesystem, environment and
// credential fallbacks for this process. It is intentionally one-way: callers
// must select the policy during single-threaded startup, before config loading.
// Existing data is neither read nor deleted.
func UseCanonicalUserDataOnly() {
	canonicalUserDataOnly.Store(true)
}

func canonicalUserDataOnlyEnabled() bool {
	return canonicalUserDataOnly.Load()
}

func legacyKeyringCredentialValueFallback(key string) (string, bool) {
	if canonicalUserDataOnlyEnabled() {
		return "", false
	}
	return legacyKeyringCredentialValueLookup(key)
}

func userConfigPath() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

func userConfigDir() string {
	return kwuHomeDir()
}

func kwuHomeDir() string {
	primary := canonicalHomeDir()
	if primary == "" || canonicalUserDataOnlyEnabled() || hasHomeDirOverride() || pathExists(primary) || !automaticUserDataMigrationEnabled {
		return primary
	}
	for _, dir := range legacyHomeDirs() {
		if pathExists(dir) {
			return dir
		}
	}
	return primary
}

func canonicalHomeDir() string {
	if canonicalUserDataOnlyEnabled() {
		if dir := cleanEnvDir(brand.PathEnvPrefix + "_HOME"); dir != "" {
			return dir
		}
		return defaultCanonicalHomeDir()
	}
	if dir := cleanEnvDirAliases(
		brand.PathEnvPrefix+"_HOME",
		brand.EnvPrefix+"_HOME",
		"SUIYUAN_HOME",
		"REASONIX_HOME",
	); dir != "" {
		return dir
	}
	return defaultCanonicalHomeDir()
}

func defaultCanonicalHomeDir() string {
	if runtime.GOOS != "windows" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, brand.ProjectDirName)
		}
		return ""
	}
	dir := osUserConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, brand.EnglishName)
}

func hasHomeDirOverride() bool {
	for _, name := range []string{
		brand.PathEnvPrefix + "_HOME",
		brand.EnvPrefix + "_HOME",
		"SUIYUAN_HOME",
		"REASONIX_HOME",
	} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

func hasStateDirOverride() bool {
	for _, name := range []string{
		brand.PathEnvPrefix + "_STATE_HOME",
		brand.EnvPrefix + "_STATE_HOME",
		"SUIYUAN_STATE_HOME",
		"REASONIX_STATE_HOME",
	} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

func legacyHomeDirs() []string {
	if canonicalUserDataOnlyEnabled() {
		return nil
	}
	var roots []string
	add := func(path string) {
		if path == "" || samePath(path, canonicalHomeDir()) {
			return
		}
		for _, existing := range roots {
			if samePath(existing, path) {
				return
			}
		}
		roots = append(roots, filepath.Clean(path))
	}
	if runtime.GOOS == "windows" {
		if dir := osUserConfigDir(); dir != "" {
			add(filepath.Join(dir, "kwu"))
			add(filepath.Join(dir, "suiyuan"))
			add(filepath.Join(dir, "reasonix"))
		}
		return roots
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".kwu"))
		add(filepath.Join(home, ".suiyuan"))
		add(filepath.Join(home, ".reasonix"))
	}
	return roots
}

func userConfigLoadPath() string {
	for _, path := range userConfigCandidatePaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return userConfigPath()
}

func legacyUserConfigPath() string {
	current := userConfigPath()
	for _, path := range legacyUserConfigPaths() {
		if !samePath(path, current) {
			return path
		}
	}
	return ""
}

func userConfigCandidatePaths() []string {
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
	add(userConfigPath())
	for _, path := range legacyUserConfigPaths() {
		add(path)
	}
	return paths
}

// ProfilePathResolver centralizes profile-aware user config and state paths.
// It only computes filesystem locations; callers keep ownership of loading,
// editing, migrating, and validating the data at those paths.
type ProfilePathResolver struct {
	profileID              string
	homeRoot               string
	stateRoot              string
	cacheRoot              string
	workspaceRoot          string
	memoryProjectStoreRoot string
}

// ProfilePathResolverOptions allows tests and future entry surfaces to inject
// already-resolved roots without re-reading the process environment.
type ProfilePathResolverOptions struct {
	Actor         userctx.ActorContext
	HomeRoot      string
	StateRoot     string
	CacheRoot     string
	WorkspaceRoot string
}

// NewProfilePathResolver returns the path resolver for actor's normalized
// profile. The default profile uses the active Clouvia home/state roots, with
// former-product roots retained as compatibility fallbacks. Cache uses the
// canonical Clouvia location while accepting legacy environment-variable aliases.
// Other profiles live under profiles/<profile_id>.
func NewProfilePathResolver(actor userctx.ActorContext) ProfilePathResolver {
	return NewProfilePathResolverWithOptions(ProfilePathResolverOptions{Actor: actor})
}

func defaultProfilePathResolver(workspaceRoot string) ProfilePathResolver {
	return NewProfilePathResolver(userctx.Default(workspaceRoot))
}

func NewProfilePathResolverWithOptions(opts ProfilePathResolverOptions) ProfilePathResolver {
	actor := opts.Actor.Normalize()
	if root := strings.TrimSpace(opts.WorkspaceRoot); root != "" {
		actor.WorkspaceRoot = root
	}

	homeRoot := cleanExplicitDir(opts.HomeRoot)
	if homeRoot == "" {
		homeRoot = kwuHomeDir()
	}
	stateRoot := cleanExplicitDir(opts.StateRoot)
	if stateRoot == "" {
		stateRoot = userSupportDir()
	}
	if stateRoot == "" {
		stateRoot = homeRoot
	}
	memoryProjectStoreRoot := stateRoot
	cacheRoot := cleanExplicitDir(opts.CacheRoot)
	if cacheRoot == "" {
		cacheRoot = userCacheDir()
	}

	profileID := actor.ProfileID
	if profileID != userctx.DefaultProfileID {
		if stateRoot != "" {
			profileRoot := filepath.Join(stateRoot, "profiles", profileID)
			homeRoot = profileRoot
			stateRoot = profileRoot
		} else {
			homeRoot = ""
		}
		if cacheRoot != "" {
			cacheRoot = filepath.Join(cacheRoot, "profiles", profileID)
		}
	}

	return ProfilePathResolver{
		profileID:              profileID,
		homeRoot:               homeRoot,
		stateRoot:              stateRoot,
		cacheRoot:              cacheRoot,
		workspaceRoot:          actor.WorkspaceRoot,
		memoryProjectStoreRoot: memoryProjectStoreRoot,
	}
}

func cleanExplicitDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	dir = ExpandVars(dir)
	if dir == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = home
		}
	} else if strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, dir[2:])
		}
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	return filepath.Clean(dir)
}

func (p ProfilePathResolver) ProfileID() string { return p.profileID }

// KWUHomeDir returns the profile-scoped Clouvia home root used for config.toml
// and global skills. The method name is retained for API compatibility.
func (p ProfilePathResolver) KWUHomeDir() string { return p.homeRoot }

func (p ProfilePathResolver) withWorkspaceRoot(root string) ProfilePathResolver {
	if p.profileID == "" {
		return defaultProfilePathResolver(root)
	}
	if strings.TrimSpace(p.workspaceRoot) == "" {
		p.workspaceRoot = strings.TrimSpace(root)
	}
	return p
}

func (p ProfilePathResolver) UserConfigPath() string {
	if p.homeRoot == "" {
		return ""
	}
	return filepath.Join(p.homeRoot, "config.toml")
}

func (p ProfilePathResolver) UserConfigLoadPath() string {
	for _, path := range p.UserConfigCandidatePaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return p.UserConfigPath()
}

func (p ProfilePathResolver) UserConfigCandidatePaths() []string {
	primary := p.UserConfigPath()
	if primary == "" {
		return nil
	}
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
	add(primary)
	return paths
}

func (p ProfilePathResolver) UserCredentialsPath() string {
	if p.stateRoot == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, ".env")
}

func (p ProfilePathResolver) SessionDir() string {
	if p.stateRoot == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "sessions")
}

func (p ProfilePathResolver) ProjectSessionDir(workspaceRoot string) string {
	root := p.resolveWorkspaceRoot(workspaceRoot)
	if p.stateRoot == "" || root == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "projects", WorkspaceSlug(root), "sessions")
}

func (p ProfilePathResolver) MemoryUserDir() string {
	return p.stateRoot
}

func (p ProfilePathResolver) MemoryProjectStoreRoot() string {
	if p.memoryProjectStoreRoot != "" {
		return p.memoryProjectStoreRoot
	}
	return p.stateRoot
}

func (p ProfilePathResolver) ArchiveDir() string {
	if p.stateRoot == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "archive")
}

func (p ProfilePathResolver) CacheDir() string {
	return p.cacheRoot
}

// ConnectorStateDir is the profile-owned root for non-secret connector state.
// Shared packages remain under the process cache root.
func (p ProfilePathResolver) ConnectorStateDir() string {
	if p.stateRoot == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "connectors")
}

func (p ProfilePathResolver) ProjectTrustPath(workspaceRoot string) string {
	if p.profileID == userctx.DefaultProfileID {
		if p.homeRoot == "" {
			return ""
		}
		return filepath.Join(p.homeRoot, "trust.json")
	}
	root := p.resolveWorkspaceRoot(workspaceRoot)
	if p.stateRoot == "" || root == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "project-trust", WorkspaceSlug(root)+".json")
}

// ProjectTrustFallbackPaths preserves explicit MCP trust decisions written by
// older releases without keeping the removed shell-hook store as the canonical
// location.
func (p ProfilePathResolver) ProjectTrustFallbackPaths(workspaceRoot string) []string {
	if p.profileID == userctx.DefaultProfileID {
		return nil
	}
	root := p.resolveWorkspaceRoot(workspaceRoot)
	if p.stateRoot == "" || root == "" {
		return nil
	}
	return []string{filepath.Join(p.stateRoot, "hooks", WorkspaceSlug(root)+".json")}
}

func (p ProfilePathResolver) DesktopWorkspaceStatePath() string {
	if p.stateRoot == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "desktop-workspace")
}

func (p ProfilePathResolver) DesktopWorkspaceListPath() string {
	if p.stateRoot == "" {
		return ""
	}
	return filepath.Join(p.stateRoot, "desktop-workspaces.json")
}

func (p ProfilePathResolver) DesktopWindowStatePath() string {
	if p.stateRoot == "" && p.profileID != userctx.DefaultProfileID {
		return ""
	}
	return filepath.Join(p.stateRoot, "desktop-window.json")
}

func (p ProfilePathResolver) resolveWorkspaceRoot(workspaceRoot string) string {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		root = strings.TrimSpace(p.workspaceRoot)
	}
	if root == "" {
		return ""
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return filepath.Clean(root)
}

func legacyXDGConfigPaths() []string {
	if canonicalUserDataOnlyEnabled() {
		return nil
	}
	return compatibleXDGConfigPaths()
}

func compatibleXDGConfigPaths() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	var paths []string
	add := func(path string) {
		if path == "" {
			return
		}
		for _, existing := range paths {
			if samePath(existing, path) {
				return
			}
		}
		paths = append(paths, filepath.Clean(path))
	}
	if dir := cleanEnvDir("XDG_CONFIG_HOME"); dir != "" {
		for _, name := range []string{"kwu", "suiyuan", "reasonix"} {
			add(filepath.Join(dir, name, "config.toml"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, name := range []string{"kwu", "suiyuan", "reasonix"} {
			add(filepath.Join(home, ".config", name, "config.toml"))
		}
	}
	return paths
}

func userSupportDir() string {
	if canonicalUserDataOnlyEnabled() {
		if dir := cleanEnvDir(brand.PathEnvPrefix + "_STATE_HOME"); dir != "" {
			return dir
		}
		return kwuHomeDir()
	}
	if dir := cleanEnvDirAliases(
		brand.PathEnvPrefix+"_STATE_HOME",
		brand.EnvPrefix+"_STATE_HOME",
		"SUIYUAN_STATE_HOME",
		"REASONIX_STATE_HOME",
	); dir != "" {
		return dir
	}
	return kwuHomeDir()
}

func legacyOSSupportDir() string {
	if canonicalUserDataOnlyEnabled() {
		return ""
	}
	for _, root := range legacyHomeDirs() {
		if !samePath(root, kwuHomeDir()) {
			return root
		}
	}
	return ""
}

func userCacheDir() string {
	if canonicalUserDataOnlyEnabled() {
		if dir := cleanEnvDir(brand.PathEnvPrefix + "_CACHE_HOME"); dir != "" {
			return dir
		}
		return defaultCanonicalCacheDir()
	}
	if dir := cleanEnvDirAliases(
		brand.PathEnvPrefix+"_CACHE_HOME",
		brand.EnvPrefix+"_CACHE_HOME",
		"SUIYUAN_CACHE_HOME",
		"REASONIX_CACHE_HOME",
	); dir != "" {
		return dir
	}
	return defaultCanonicalCacheDir()
}

func defaultCanonicalCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, brand.EnglishName)
}

func osUserConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return dir
}

func cleanEnvDir(name string) string {
	dir := strings.TrimSpace(os.Getenv(name))
	if dir == "" {
		return ""
	}
	dir = ExpandVars(dir)
	if dir == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = home
		}
	} else if strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, dir[2:])
		}
	}
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	return filepath.Clean(dir)
}

func cleanEnvDirAlias(primary, legacy string) string {
	return cleanEnvDirAliases(primary, legacy)
}

func cleanEnvDirAliases(names ...string) string {
	for _, name := range names {
		if dir := cleanEnvDir(name); dir != "" {
			return dir
		}
	}
	return ""
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, aerr := filepath.Abs(a)
	bb, berr := filepath.Abs(b)
	if aerr == nil {
		a = aa
	}
	if berr == nil {
		b = bb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// userConfigDisplayPath is userConfigPath collapsed to a ~-relative form for
// comments rendered into the user's own config.toml, so Windows users see the
// real location instead of the old hardcoded legacy ~/.kwu path.
func userConfigDisplayPath() string {
	p := userConfigPath()
	if p == "" {
		return "<os-config-dir>/Clouvia/config.toml"
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return p
}

// UserConfigPath is the user-global config.toml. New installs use
// CLOUVIA_HOME/config.toml, ~/.clouvia/config.toml on Unix-like systems,
// or %AppData%/Clouvia/config.toml on Windows. Legacy roots remain readable.
func UserConfigPath() string { return userConfigPath() }

// LegacyUserConfigPath returns the first former-product config path.
func LegacyUserConfigPath() string { return legacyUserConfigPath() }

func legacyUserConfigPaths() []string {
	if canonicalUserDataOnlyEnabled() {
		return nil
	}
	var paths []string
	for _, root := range legacyHomeDirs() {
		paths = append(paths, filepath.Join(root, "config.toml"))
	}
	paths = append(paths, legacyXDGConfigPaths()...)
	return paths
}

// LegacyUserConfigPaths lists KWU, SuiYuan and Reasonix config locations.
func LegacyUserConfigPaths() []string {
	return append([]string(nil), legacyUserConfigPaths()...)
}

// KWUHomeDir keeps its exported name for API compatibility. It returns the
// Clouvia home, falling back to an existing legacy root if migration cannot run.
func KWUHomeDir() string { return kwuHomeDir() }

// UserCredentialsPath is Clouvia's global .env file under the active user state root.
// It is the single source for provider credentials saved by the application, so
// stale shell, Windows, project, or home env vars cannot silently override keys
// the user saved through setup or settings. "" when the user state root can't be
// resolved.
func UserCredentialsPath() string {
	dir := userSupportDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, ".env")
}

// UserExpertsPath is the local user-defined expert store. These profiles are
// global to the user and are merged with built-in experts when sessions start.
func UserExpertsPath() string {
	dir := userSupportDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "experts.json")
}

// ArchiveDir is where compacted conversation history is archived for
// traceability (one timestamped .jsonl per compaction). Empty if the user state
// directory cannot be resolved, in which case archiving is skipped.
func ArchiveDir() string {
	dir := userSupportDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "archive")
}

// SessionDir is where chat sessions are persisted (one .jsonl per session).
// Used by `kwu --continue` / `--resume` to find the recent ones. Empty
// if the user state dir can't be resolved — sessions then aren't saved.
func SessionDir() string {
	dir := userSupportDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "sessions")
}

// ProjectSessionDir is the per-workspace session directory the desktop sidebar
// lists: <state root>/projects/<slug>/sessions. Empty when either the state root
// or workspaceRoot doesn't resolve.
func ProjectSessionDir(workspaceRoot string) string {
	base := MemoryUserDir()
	root := strings.TrimSpace(workspaceRoot)
	if base == "" || root == "" {
		return ""
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return filepath.Join(base, "projects", WorkspaceSlug(root), "sessions")
}

// WorkspaceSlug flattens an absolute workspace path into the directory name
// used under <state root>/projects.
func WorkspaceSlug(absPath string) string {
	return strings.NewReplacer(string(os.PathSeparator), "-", "/", "-", "\\", "-", ":", "-").Replace(absPath)
}

// CacheDir is the per-user cache root for derived/regenerable artefacts: MCP
// handshake snapshots, plugin startup-latency telemetry. Empty when the OS dir is
// unavailable — callers must tolerate that (caching is best-effort).
func CacheDir() string {
	dir := userCacheDir()
	if dir == "" {
		return ""
	}
	return dir
}

// MemoryUserDir returns the active user state root (canonical Clouvia, with a
// legacy fallback when needed), under which the user-global CLOUVIA.md and the
// per-project auto-memory store live. Empty when the user state dir can't be
// resolved, which disables user-scoped memory.
func MemoryUserDir() string {
	return userSupportDir()
}

// DesktopWorkspaceStatePath stores the most recent desktop workspace. The
// default profile keeps the historical MemoryUserDir()/desktop-workspace path.
func DesktopWorkspaceStatePath() string {
	return defaultProfilePathResolver("").DesktopWorkspaceStatePath()
}

// DesktopWorkspaceListPath stores the desktop workspace MRU list. The default
// profile keeps the historical MemoryUserDir()/desktop-workspaces.json path.
func DesktopWorkspaceListPath() string {
	return defaultProfilePathResolver("").DesktopWorkspaceListPath()
}

// DesktopWindowStatePath stores desktop window geometry. The default profile
// keeps the historical MemoryUserDir()/desktop-window.json path.
func DesktopWindowStatePath() string {
	return defaultProfilePathResolver("").DesktopWindowStatePath()
}

// ConventionDirs are the parent directories scanned for agent assets (skills,
// commands), in canonical-first order. .clouvia is ours; .agents / .agent /
// .claude let users drop in assets authored for other agent tools without moving
// files. Shared so skills (internal/skill) and commands (CommandDirs) discover
// the same set.
var ConventionDirs = []string{
	brand.ProjectDirName,
	brand.LegacyProjectDirName,
	brand.SuiYuanProjectDirName,
	brand.ReasonixProjectDirName,
	".agents",
	".agent",
	".claude",
}

// SkillConventionDirs extends the shared agent-asset conventions with Codex's
// skill directory. Codex skills use the same SKILL.md package format, while its
// command layout is not part of the `kwu` CLI's slash-command compatibility contract.
var SkillConventionDirs = append(append([]string(nil), ConventionDirs...), ".codex")

// conventionSubdirsAsc joins sub under each ConventionDir of base, in ascending
// priority (reverse of ConventionDirs) so the canonical .clouvia ends up the
// highest-priority entry — command.Load lets a later directory win on a clash.
func conventionSubdirsAsc(base, sub string) []string {
	out := make([]string, 0, len(ConventionDirs))
	for i := len(ConventionDirs) - 1; i >= 0; i-- {
		out = append(out, filepath.Join(base, ConventionDirs[i], sub))
	}
	return out
}

// CommandDirs returns the directories scanned for custom slash commands, lowest
// priority first, so a later (more specific) directory overrides an earlier one
// on a name clash. Order: home-dir convention dirs (~/.claude/commands through
// ~/.clouvia/commands, including legacy former-product dirs), the active Clouvia
// config/state commands dirs, then the project's convention dirs
// (.claude/commands through .clouvia/commands). Scanning the .claude /
// .agents / .agent dirs lets commands authored for other agent tools (same .md +
// frontmatter format) work here unchanged.
func CommandDirs() []string {
	return CommandDirsForRoot(".")
}

// CommandDirsForRoot is like CommandDirs but resolves the project convention
// dirs under root instead of the current working directory. Global dirs are
// unchanged — they are always user-scoped.
func CommandDirsForRoot(root string) []string {
	root = resolveRoot(root)
	var dirs []string
	add := func(dir string) {
		if dir == "" {
			return
		}
		for _, existing := range dirs {
			if samePath(existing, dir) {
				return
			}
		}
		dirs = append(dirs, dir)
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range conventionSubdirsAsc(home, "commands") {
			add(dir)
		}
	}
	if dir := userConfigDir(); dir != "" {
		add(filepath.Join(dir, "commands"))
	}
	if dir := userSupportDir(); dir != "" && !samePath(dir, userConfigDir()) {
		add(filepath.Join(dir, "commands"))
	}
	for _, dir := range conventionSubdirsAsc(root, "commands") {
		add(dir)
	}
	return dirs
}

// SourcePath returns the highest-priority config file that exists, or "" if none.
func SourcePath() string {
	return SourcePathForRoot(".")
}

// SourcePathForRoot returns the highest-priority config file that exists under
// root, or "" if none. Equivalent to SourcePath() when root is ".".
func SourcePathForRoot(root string) string {
	return SourcePathForRootWithPaths(root, defaultProfilePathResolver(root))
}

func projectConfigPathForRoot(root string) string {
	root = resolveRoot(root)
	names := []string{
		brand.ProjectConfigName,
		brand.LegacyProjectConfigName,
		brand.SuiYuanProjectConfigName,
		brand.ReasonixProjectConfigName,
	}
	for _, name := range names {
		path := name
		if root != "." {
			path = filepath.Join(root, name)
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if root != "." {
		return filepath.Join(root, brand.ProjectConfigName)
	}
	return brand.ProjectConfigName
}

// ProjectConfigPathForRoot returns the preferred project config path for root:
// existing canonical or legacy config, otherwise a new clouvia.toml.
func ProjectConfigPathForRoot(root string) string {
	return projectConfigPathForRoot(root)
}

// SourcePathForRootWithActor returns the highest-priority config source visible
// to actor at root, preserving project-over-user precedence.
func SourcePathForRootWithActor(root string, actor userctx.ActorContext) string {
	actor = actor.Normalize()
	if strings.TrimSpace(actor.WorkspaceRoot) == "" {
		actor.WorkspaceRoot = root
	}
	return SourcePathForRootWithPaths(root, NewProfilePathResolver(actor))
}

// SourcePathForRootWithPaths returns the highest-priority config source visible
// through paths at root, preserving project-over-user precedence.
func SourcePathForRootWithPaths(root string, paths ProfilePathResolver) string {
	root = resolveRoot(root)
	paths = paths.withWorkspaceRoot(root)
	projectTOML := projectConfigPathForRoot(root)
	if _, err := os.Stat(projectTOML); err == nil {
		return projectTOML
	}
	if uc := paths.UserConfigLoadPath(); uc != "" {
		if _, err := os.Stat(uc); err == nil {
			return uc
		}
	}
	return ""
}
