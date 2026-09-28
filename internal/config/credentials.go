package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"kwu/internal/brand"
	"kwu/internal/userctx"
)

const (
	CredentialsStoreAuto    = "auto"
	CredentialsStoreKeyring = "keyring"
	CredentialsStoreFile    = "file"

	credentialsKeyringService = "kwu"
	credentialClearedPrefix   = "# kwu-cleared "
)

const (
	CredentialSourceEnvironment = "environment"
	CredentialSourceProjectEnv  = "project_env"
	CredentialSourceCredentials = "credentials"
	CredentialSourceHomeEnv     = "home_env"
	CredentialSourceLegacy      = "legacy_credentials"
)

type CredentialSource struct {
	Kind  string `json:"kind"`
	Path  string `json:"path,omitempty"`
	Label string `json:"label,omitempty"`
}

type CredentialResolution struct {
	Name     string             `json:"name"`
	Set      bool               `json:"set"`
	Value    string             `json:"-"`
	Source   CredentialSource   `json:"source,omitempty"`
	Shadowed []CredentialSource `json:"shadowed,omitempty"`
}

type trackedCredentialSource struct {
	source CredentialSource
	value  string
}

var credentialSourceTracker = struct {
	sync.Mutex
	byKey map[string]trackedCredentialSource
}{byKey: map[string]trackedCredentialSource{}}

var storedCredentialValueLookup = storedCredentialValue
var storedCredentialValueWithPathsLookup = storedCredentialValueWithPaths
var keyringCredentialValueLookup = keyringCredentialValue
var keyringCredentialDeleteLookup = keyringCredentialDelete
var legacyKeyringCredentialValueLookup = legacyKeyringCredentialValue

// CredentialResolver resolves credentials repeatedly for one caller-owned view
// build. It keeps expensive global credential-store lookups bounded to one per
// key while preserving the same source/shadow reporting as the one-shot helpers.
type CredentialResolver struct {
	root     string
	paths    ProfilePathResolver
	usePaths bool

	mu               sync.Mutex
	globalFirstCache map[string]CredentialResolution
}

// NewCredentialResolverForRoot returns a resolver scoped to a workspace root.
func NewCredentialResolverForRoot(root string) *CredentialResolver {
	return &CredentialResolver{root: resolveRoot(root)}
}

// NewCredentialResolverForRootWithPaths returns a resolver scoped to a workspace
// root and profile-aware credential paths.
func NewCredentialResolverForRootWithPaths(root string, paths ProfilePathResolver) *CredentialResolver {
	root = resolveRoot(root)
	return &CredentialResolver{root: root, paths: paths.withWorkspaceRoot(root), usePaths: true}
}

// ResolveGlobalFirst resolves key from the current profile's credentials store.
// Repeated calls for the same key reuse the first result so UI views with
// multiple provider entries sharing api_key_env stay consistent.
func (r *CredentialResolver) ResolveGlobalFirst(key string) CredentialResolution {
	key = strings.TrimSpace(key)
	if key == "" {
		return CredentialResolution{Name: key}
	}
	if r == nil {
		return resolveCredentialForRootGlobalFirst(".", key)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.globalFirstCache == nil {
		r.globalFirstCache = map[string]CredentialResolution{}
	}
	if cached, ok := r.globalFirstCache[key]; ok {
		return cloneCredentialResolution(cached)
	}
	var res CredentialResolution
	if r.usePaths {
		res = resolveCredentialForRootGlobalFirstWithPaths(r.root, r.paths, key)
	} else {
		res = resolveCredentialForRootGlobalFirst(r.root, key)
	}
	r.globalFirstCache[key] = cloneCredentialResolution(res)
	return res
}

func cloneCredentialResolution(res CredentialResolution) CredentialResolution {
	if len(res.Shadowed) > 0 {
		res.Shadowed = append([]CredentialSource(nil), res.Shadowed...)
	}
	return res
}

func normalizeCredentialsStore(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case CredentialsStoreKeyring:
		return CredentialsStoreKeyring
	case CredentialsStoreFile:
		return CredentialsStoreFile
	default:
		return CredentialsStoreAuto
	}
}

func credentialKeyAliases(key string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if key != brand.OfficialAPIKeyEnv && key != brand.LegacyOfficialAPIKeyEnv {
		return []string{key}
	}
	aliases := []string{brand.OfficialAPIKeyEnv}
	if brand.LegacyOfficialAPIKeyEnv != brand.OfficialAPIKeyEnv {
		aliases = append(aliases, brand.LegacyOfficialAPIKeyEnv)
	}
	return aliases
}

func credentialsStoreMode() string {
	return credentialsStoreModeWithPaths(defaultProfilePathResolver(""))
}

func credentialsStoreModeWithPaths(paths ProfilePathResolver) string {
	if mode := strings.TrimSpace(os.Getenv(brand.EnvPrefix + "_CREDENTIALS_STORE")); mode != "" {
		return normalizeCredentialsStore(mode)
	}
	if mode := strings.TrimSpace(os.Getenv(brand.LegacyEnvPrefix + "_CREDENTIALS_STORE")); mode != "" {
		return normalizeCredentialsStore(mode)
	}
	paths = paths.withWorkspaceRoot("")
	var partial struct {
		CredentialsStore string `toml:"credentials_store"`
	}
	if path := paths.UserConfigLoadPath(); path != "" {
		_, _ = toml.DecodeFile(path, &partial)
	}
	return normalizeCredentialsStore(partial.CredentialsStore)
}

func credentialEnvNamesForRoot(root string) []string {
	return credentialEnvNamesForRootWithPaths(root, defaultProfilePathResolver(root))
}

func credentialEnvNamesForRootWithPaths(root string, paths ProfilePathResolver) []string {
	root = resolveRoot(root)
	paths = paths.withWorkspaceRoot(root)
	cfg := Default()

	projectTOML := projectConfigPathForRoot(root)
	if uc := paths.UserConfigLoadPath(); uc != "" {
		_ = mergeFile(cfg, uc)
	}
	_ = mergeFile(cfg, projectTOML)
	var tomlSources []string
	if uc := paths.UserConfigLoadPath(); uc != "" {
		tomlSources = append(tomlSources, uc)
	}
	tomlSources = append(tomlSources, projectTOML)
	if providers, _, _, ok, err := mergeTOMLProviders(tomlSources); err == nil && ok {
		cfg.Providers = providers
	}

	return credentialEnvNamesFromConfig(cfg)
}

func credentialEnvNamesFromConfig(cfg *Config) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range cfg.Providers {
		addCredentialEnvName(&out, seen, p.APIKeyEnv)
	}
	addCredentialEnvName(&out, seen, cfg.Bot.QQ.AppSecretEnv)
	addCredentialEnvName(&out, seen, cfg.Bot.Feishu.AppSecretEnv)
	addCredentialEnvName(&out, seen, cfg.Bot.Weixin.TokenEnv)
	for _, conn := range cfg.Bot.Connections {
		addCredentialEnvName(&out, seen, conn.Credential.AppSecretEnv)
		addCredentialEnvName(&out, seen, conn.Credential.TokenEnv)
	}
	for _, plugin := range cfg.Plugins {
		addPluginCredentialEnvNames(&out, seen, plugin)
	}
	sort.Strings(out)
	return out
}

func addCredentialEnvName(out *[]string, seen map[string]bool, name string) {
	name = strings.TrimSpace(name)
	if name == "" || seen[name] {
		return
	}
	seen[name] = true
	*out = append(*out, name)
	for _, alias := range credentialKeyAliases(name) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[alias] {
			continue
		}
		seen[alias] = true
		*out = append(*out, alias)
	}
}

func addPluginCredentialEnvNames(out *[]string, seen map[string]bool, plugin PluginEntry) {
	addCredentialRefs(out, seen, plugin.Command)
	addCredentialRefs(out, seen, plugin.WorkDir)
	addCredentialRefs(out, seen, plugin.URL)
	for _, arg := range plugin.Args {
		addCredentialRefs(out, seen, arg)
	}
	for _, value := range plugin.Env {
		addCredentialRefs(out, seen, value)
	}
	for _, value := range plugin.Headers {
		addCredentialRefs(out, seen, value)
	}
}

func addCredentialRefs(out *[]string, seen map[string]bool, value string) {
	for _, match := range varRef.FindAllStringSubmatch(value, -1) {
		if len(match) > 1 {
			addCredentialEnvName(out, seen, match[1])
		}
	}
}

func resolveProviderCredentialsForRoot(root string, cfg *Config) {
	if cfg == nil || len(cfg.Providers) == 0 {
		return
	}
	resolver := NewCredentialResolverForRoot(root)
	resolveProviderCredentialsWithResolver(cfg, resolver)
}

func resolveProviderCredentialsForRootWithPaths(root string, paths ProfilePathResolver, cfg *Config) {
	if cfg == nil || len(cfg.Providers) == 0 {
		return
	}
	resolver := NewCredentialResolverForRootWithPaths(root, paths)
	resolveProviderCredentialsWithResolver(cfg, resolver)
}

func resolveProviderCredentialsWithResolver(cfg *Config, resolver *CredentialResolver) {
	for i := range cfg.Providers {
		resolveProviderCredentialWithResolver(&cfg.Providers[i], resolver)
	}
}

// disableProviderCredentialLookup marks every provider entry as intentionally
// unresolved. This is stronger than merely omitting eager resolution: APIKey
// must not trigger its legacy lazy credential fallback later in the same route.
func disableProviderCredentialLookup(cfg *Config) {
	if cfg == nil {
		return
	}
	for i := range cfg.Providers {
		cfg.Providers[i].resolvedAPIKey = ""
		cfg.Providers[i].resolvedSource = CredentialSource{}
		cfg.Providers[i].resolvedAPIKeySet = true
	}
}

func resolveProviderCredentialWithResolver(entry *ProviderEntry, resolver *CredentialResolver) {
	if entry == nil {
		return
	}
	key := strings.TrimSpace(entry.APIKeyEnv)
	if key == "" {
		entry.resolvedAPIKey = ""
		entry.resolvedSource = CredentialSource{}
		entry.resolvedAPIKeySet = false
		return
	}
	if resolver == nil {
		resolver = NewCredentialResolverForRoot(".")
	}
	res := resolver.ResolveGlobalFirst(key)
	if !res.Set || res.Value == "" {
		entry.resolvedAPIKey = ""
		entry.resolvedSource = CredentialSource{}
		entry.resolvedAPIKeySet = true
		return
	}
	entry.resolvedAPIKey = res.Value
	entry.resolvedSource = res.Source
	entry.resolvedAPIKeySet = true
}

func (e *ProviderEntry) ResolveAPIKeyForRoot(root string) {
	resolveProviderCredentialWithResolver(e, NewCredentialResolverForRoot(root))
}

func (e *ProviderEntry) ResolveAPIKeyForRootWithPaths(root string, paths ProfilePathResolver) {
	resolveProviderCredentialWithResolver(e, NewCredentialResolverForRootWithPaths(root, paths))
}

func loadCredentialStoreForRoot(root string) {
	loadCredentialStoreForRootWithPaths(root, defaultProfilePathResolver(root))
}

func loadCredentialStoreForRootWithPaths(root string, paths ProfilePathResolver) {
	paths = paths.withWorkspaceRoot(root)
	names := credentialEnvNamesForRootWithPaths(root, paths)
	if len(names) == 0 {
		return
	}
	if p := paths.UserCredentialsPath(); p != "" {
		source := credentialSourceForCredentials(p, CredentialOwnerForPaths(paths))
		clearTrackedCredentialEnv(names)
		loadDotEnvFileAs(p, source)
	}
}

// StoreCredentialLines stores KEY=value assignments in Clouvia's default profile .env
// and pins them into the current process environment.
func StoreCredentialLines(lines []string) (string, error) {
	return StoreCredentialLinesWithPaths(defaultProfilePathResolver(""), lines)
}

// StoreCredentialLinesWithPaths stores KEY=value assignments in the supplied
// profile's credentials file and pins them into the current process environment.
func StoreCredentialLinesWithPaths(paths ProfilePathResolver, lines []string) (string, error) {
	paths = paths.withWorkspaceRoot("")
	assignments := parseCredentialLines(lines)
	path := paths.UserCredentialsPath()
	if len(assignments) == 0 {
		return path, nil
	}
	if err := storeCredentialsInFile(path, assignments); err != nil {
		return "", err
	}
	pinCredentialAssignmentsWithSource(assignments, credentialSourceForCredentials(path, CredentialOwnerForPaths(paths)))
	return path, nil
}

func SetCredential(key, value string) (string, error) {
	return SetCredentialWithPaths(defaultProfilePathResolver(""), key, value)
}

func SetCredentialWithPaths(paths ProfilePathResolver, key, value string) (string, error) {
	key = strings.TrimSpace(key)
	if !isCredentialKey(key) {
		return "", fmt.Errorf("invalid credential key %q", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("credential value for %s contains a newline", key)
	}
	return StoreCredentialLinesWithPaths(paths, []string{key + "=" + value})
}

func RemoveCredential(key string) error {
	return RemoveCredentialWithPaths(defaultProfilePathResolver(""), key)
}

func RemoveCredentialWithPaths(paths ProfilePathResolver, key string) error {
	paths = paths.withWorkspaceRoot("")
	key = strings.TrimSpace(key)
	if key == "" || !isCredentialKey(key) {
		return nil
	}
	var keyringErr error
	for _, name := range credentialKeyAliases(key) {
		if path := paths.UserCredentialsPath(); path != "" {
			if err := removeCredentialFromFile(path, name); err != nil {
				return err
			}
		}
		if credentialsStoreModeWithPaths(paths) == CredentialsStoreKeyring {
			if err := keyringCredentialDeleteLookup(CredentialOwnerForPaths(paths), name); err != nil && keyringErr == nil {
				keyringErr = err
			}
		}
		if err := os.Unsetenv(name); err != nil {
			return err
		}
	}
	return keyringErr
}

func CredentialIsSet(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	return CredentialStored(key)
}

func CredentialStored(key string) bool {
	return CredentialStoredWithPaths(defaultProfilePathResolver(""), key)
}

func CredentialStoredWithPaths(paths ProfilePathResolver, key string) bool {
	paths = paths.withWorkspaceRoot("")
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	for _, name := range credentialKeyAliases(key) {
		if envFileHasValue(paths.UserCredentialsPath(), name) {
			return true
		}
	}
	return false
}

func credentialCurrentStoreHasKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	return envFileHasValue(UserCredentialsPath(), key)
}

func credentialCurrentStoreClearedKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	return envFileHasClearedKey(UserCredentialsPath(), key)
}

func CredentialsTargetDescription() string {
	return UserCredentialsPath()
}

func CredentialsTargetDescriptionWithPaths(paths ProfilePathResolver) string {
	paths = paths.withWorkspaceRoot("")
	return paths.UserCredentialsPath()
}

func CredentialOwnerDescriptionWithPaths(paths ProfilePathResolver) string {
	return CredentialOwnerForPaths(paths).Description()
}

func parseCredentialLines(lines []string) map[string]string {
	out := map[string]string{}
	for _, raw := range lines {
		line := strings.TrimPrefix(strings.TrimSpace(raw), "export ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if !ok || !isCredentialKey(key) || strings.ContainsAny(value, "\r\n") {
			continue
		}
		out[key] = value
	}
	return out
}

func pinCredentialAssignments(assignments map[string]string) {
	pinCredentialAssignmentsWithSource(assignments, credentialSourceForCredentials(UserCredentialsPath(), CredentialOwnerForProfile(userctx.DefaultProfileID)))
}

func pinCredentialAssignmentsWithSource(assignments map[string]string, source CredentialSource) {
	for key, value := range assignments {
		_ = os.Setenv(key, value)
		recordCredentialSource(key, value, source)
	}
}

func recordExistingCredentialSource(key string) {
	key = strings.TrimSpace(key)
	value := os.Getenv(key)
	if key == "" || value == "" {
		return
	}
	credentialSourceTracker.Lock()
	defer credentialSourceTracker.Unlock()
	if current, ok := credentialSourceTracker.byKey[key]; ok && current.value == value {
		return
	}
	if _, ok := credentialSourceTracker.byKey[key]; ok {
		return
	}
	credentialSourceTracker.byKey[key] = trackedCredentialSource{
		source: CredentialSource{Kind: CredentialSourceEnvironment, Label: "environment variable"},
		value:  value,
	}
}

func recordCredentialSource(key, value string, source CredentialSource) {
	key = strings.TrimSpace(key)
	if key == "" || value == "" {
		return
	}
	source.Label = credentialSourceLabel(source)
	credentialSourceTracker.Lock()
	credentialSourceTracker.byKey[key] = trackedCredentialSource{source: source, value: value}
	credentialSourceTracker.Unlock()
}

func clearTrackedCredentialEnv(keys []string) {
	credentialSourceTracker.Lock()
	defer credentialSourceTracker.Unlock()
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		current, ok := credentialSourceTracker.byKey[key]
		if !ok || current.source.Kind != CredentialSourceCredentials {
			continue
		}
		if value := os.Getenv(key); value == current.value {
			_ = os.Unsetenv(key)
		}
		delete(credentialSourceTracker.byKey, key)
	}
}

func trackedCredential(key, value string) (CredentialSource, bool) {
	credentialSourceTracker.Lock()
	defer credentialSourceTracker.Unlock()
	current, ok := credentialSourceTracker.byKey[key]
	if !ok || current.value != value {
		return CredentialSource{}, false
	}
	return current.source, true
}

func credentialSourceLabel(source CredentialSource) string {
	if strings.TrimSpace(source.Label) != "" {
		return source.Label
	}
	switch source.Kind {
	case CredentialSourceProjectEnv:
		return "project .env"
	case CredentialSourceCredentials:
		return brand.DisplayName + " credentials"
	case CredentialSourceHomeEnv:
		return "home .env"
	case CredentialSourceLegacy:
		return "legacy " + brand.DisplayName + " credentials"
	case CredentialSourceEnvironment:
		return "environment variable"
	default:
		return ""
	}
}

func ResolveCredential(key string) CredentialResolution {
	return ResolveCredentialForRoot(".", key)
}

func ResolveCredentialForRoot(root, key string) CredentialResolution {
	key = strings.TrimSpace(key)
	res := CredentialResolution{Name: key}
	if key == "" {
		return res
	}
	actualKey := key
	value := ""
	for _, name := range credentialKeyAliases(key) {
		value = os.Getenv(name)
		if value != "" {
			actualKey = name
			break
		}
	}
	if value == "" {
		return res
	}
	res.Set = true
	res.Value = value
	if source, ok := trackedCredential(actualKey, value); ok {
		res.Source = source
	} else if source, ok := inferCredentialSource(root, actualKey, value); ok {
		res.Source = source
	} else {
		res.Source = CredentialSource{Kind: CredentialSourceEnvironment, Label: credentialSourceLabel(CredentialSource{Kind: CredentialSourceEnvironment})}
	}
	res.Source.Label = credentialSourceLabel(res.Source)
	res.Shadowed = shadowedCredentialSources(root, actualKey, value, res.Source)
	return res
}

func ResolveCredentialForRootGlobalFirst(root, key string) CredentialResolution {
	key = strings.TrimSpace(key)
	return NewCredentialResolverForRoot(root).ResolveGlobalFirst(key)
}

func ResolveCredentialForRootGlobalFirstWithPaths(root string, paths ProfilePathResolver, key string) CredentialResolution {
	return resolveCredentialForRootGlobalFirstWithPaths(root, paths, key)
}

func resolveCredentialForRootGlobalFirst(root, key string) CredentialResolution {
	root = resolveRoot(root)
	res := CredentialResolution{Name: key}
	if key == "" {
		return res
	}
	if value, source, ok := storedCredentialValueLookup(key); ok {
		res.Set = true
		res.Value = value
		res.Source = source
		res.Source.Label = credentialSourceLabel(res.Source)
		res.Shadowed = shadowedCredentialSources(root, key, value, res.Source)
		return res
	}
	return res
}

func resolveCredentialForRootGlobalFirstWithPaths(root string, paths ProfilePathResolver, key string) CredentialResolution {
	root = resolveRoot(root)
	paths = paths.withWorkspaceRoot(root)
	key = strings.TrimSpace(key)
	res := CredentialResolution{Name: key}
	if key == "" {
		return res
	}
	if value, source, ok := storedCredentialValueWithPathsLookup(paths, key); ok {
		res.Set = true
		res.Value = value
		res.Source = source
		res.Source.Label = credentialSourceLabel(res.Source)
		res.Shadowed = shadowedCredentialSourcesWithPaths(root, paths, key, value, res.Source)
		return res
	}
	return res
}

func storedCredentialValue(key string) (string, CredentialSource, bool) {
	return storedCredentialValueWithPaths(defaultProfilePathResolver(""), key)
}

func storedCredentialValueWithPaths(paths ProfilePathResolver, key string) (string, CredentialSource, bool) {
	paths = paths.withWorkspaceRoot("")
	owner := CredentialOwnerForPaths(paths)
	if credentialsStoreModeWithPaths(paths) == CredentialsStoreKeyring {
		for _, name := range credentialKeyAliases(key) {
			if value, ok := keyringCredentialValueLookup(owner, name); ok && value != "" {
				return value, credentialSourceForKeyring(owner), true
			}
		}
	}
	if p := paths.UserCredentialsPath(); p != "" {
		for _, name := range credentialKeyAliases(key) {
			if value, ok := envFileValue(p, name); ok && value != "" {
				return value, credentialSourceForCredentials(p, owner), true
			}
		}
	}
	return "", CredentialSource{}, false
}

func credentialSourceForKeyring(owner CredentialOwner) CredentialSource {
	label := brand.DisplayName + " credentials (keyring)"
	if !owner.isDefaultProfile() {
		label = brand.DisplayName + " credentials (" + owner.Description() + ", keyring)"
	}
	return CredentialSource{Kind: CredentialSourceCredentials, Label: label}
}

func credentialSourceForCredentials(path string, owner CredentialOwner) CredentialSource {
	label := brand.DisplayName + " credentials (.env)"
	if !owner.isDefaultProfile() {
		label = brand.DisplayName + " credentials (" + owner.Description() + ")"
	}
	return CredentialSource{Kind: CredentialSourceCredentials, Path: path, Label: label}
}

func inferCredentialSource(root, key, value string) (CredentialSource, bool) {
	for _, candidate := range credentialSourceCandidates(root) {
		if v, ok := envFileValue(candidate.Path, key); ok && v == value {
			candidate.Label = credentialSourceLabel(candidate)
			return candidate, true
		}
	}
	return CredentialSource{}, false
}

func shadowedCredentialSources(root, key, activeValue string, active CredentialSource) []CredentialSource {
	return shadowedCredentialSourcesWithPaths(root, defaultProfilePathResolver(root), key, activeValue, active)
}

func shadowedCredentialSourcesWithPaths(root string, paths ProfilePathResolver, key, activeValue string, active CredentialSource) []CredentialSource {
	var out []CredentialSource
	for _, candidate := range credentialSourceCandidatesWithPaths(root, paths) {
		if sameCredentialSource(candidate, active) {
			continue
		}
		if v, ok := envFileValue(candidate.Path, key); ok && v != activeValue {
			candidate.Label = credentialSourceLabel(candidate)
			out = append(out, candidate)
		}
	}
	return out
}

func credentialSourceCandidates(root string) []CredentialSource {
	return credentialSourceCandidatesWithPaths(root, defaultProfilePathResolver(root))
}

func credentialSourceCandidatesWithPaths(root string, paths ProfilePathResolver) []CredentialSource {
	root = resolveRoot(root)
	paths = paths.withWorkspaceRoot(root)
	var out []CredentialSource
	dotEnvPath := ".env"
	if root != "" && root != "." {
		dotEnvPath = filepath.Join(root, ".env")
	}
	out = append(out, CredentialSource{Kind: CredentialSourceProjectEnv, Path: dotEnvPath})
	if p := paths.UserCredentialsPath(); p != "" {
		out = append(out, credentialSourceForCredentials(p, CredentialOwnerForPaths(paths)))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, CredentialSource{Kind: CredentialSourceHomeEnv, Path: filepath.Join(home, ".env")})
	}
	return out
}

func sameCredentialSource(a, b CredentialSource) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Path == "" || b.Path == "" {
		return a.Path == b.Path
	}
	return samePath(a.Path, b.Path)
}
