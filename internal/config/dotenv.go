package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kwu/internal/brand"
	"kwu/internal/fileutil"
)

// loadDotEnv loads Clouvia's global .env for provider credentials. The
// workspace .env values returned by loadDotEnvForRoot are ignored here because
// loadDotEnv has no Config to carry a workspace-scoped expansion environment.
func loadDotEnv() {
	loadDotEnvForRoot(".")
}

// loadDotEnvForRoot returns workspace .env values for scoped plugin/MCP/proxy
// expansion, then loads Clouvia's global .env for provider credentials.
// Workspace .env values are deliberately not written into the process
// environment, so multiple desktop/ACP workspaces cannot leak tokens into each
// other and project files cannot redirect Clouvia's own config/credential
// paths.
func loadDotEnvForRoot(root string) map[string]string {
	projectEnv := loadProjectDotEnvForExpansion(root)
	loadCredentialStoreForRoot(root)
	return projectEnv
}

func loadProjectDotEnvForExpansion(root string) map[string]string {
	root = resolveRoot(root)
	path := ".env"
	if root != "." {
		path = filepath.Join(root, ".env")
	}
	if current := UserCredentialsPath(); current != "" && samePath(path, current) {
		return nil
	}
	return readDotEnvFileMap(path, func(key string) bool {
		return !isProjectDotEnvControlKey(key)
	})
}

func isProjectDotEnvControlKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return true
	}
	upper := strings.ToUpper(key)
	if strings.HasPrefix(upper, brand.EnvPrefix+"_") || strings.HasPrefix(upper, brand.LegacyEnvPrefix+"_") {
		return true
	}
	switch upper {
	case "HOME", "USERPROFILE", "APPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME":
		return true
	default:
		return false
	}
}

func legacyCredentialsPaths() []string {
	if canonicalUserDataOnlyEnabled() {
		return nil
	}
	current := UserCredentialsPath()
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if current != "" && samePath(path, current) {
			return
		}
		if seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	if dir := legacyOSSupportDir(); dir != "" {
		add(filepath.Join(dir, "credentials"))
	}
	if dir := userSupportDir(); dir != "" {
		add(filepath.Join(dir, "credentials"))
	}
	for _, cfg := range legacyXDGConfigPaths() {
		add(filepath.Join(filepath.Dir(cfg), "credentials"))
	}
	return paths
}

func loadDotEnvFileAs(path string, source CredentialSource) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists && source.Kind != CredentialSourceCredentials {
			recordExistingCredentialSource(key)
			continue
		}
		if err := os.Setenv(key, val); err == nil && source.Kind != "" {
			source.Path = path
			recordCredentialSource(key, val, source)
		}
	}
}

func readDotEnvFileMap(path string, allow func(string) bool) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || allow != nil && !allow(key) {
			continue
		}
		out[key] = strings.Trim(strings.TrimSpace(val), `"'`)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func envFileValue(path, wantKey string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key != wantKey {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		return val, true
	}
	return "", false
}

func storeCredentialsInFile(path string, assignments map[string]string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("credentials store unavailable")
	}
	lines, err := readCredentialFileLines(path)
	if err != nil {
		return err
	}
	lines = filterClearedCredentialLines(lines, assignments)
	replaced := replaceCredentialAssignments(lines, assignments)
	for _, key := range sortedCredentialKeys(assignments) {
		if !replaced[key] {
			lines = append(lines, key+"="+assignments[key])
		}
	}
	return writeCredentialFileLines(path, lines)
}

func filterClearedCredentialLines(lines []string, assignments map[string]string) []string {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if key, ok := credentialClearedLineKey(line); ok {
			if _, hit := assignments[key]; hit {
				continue
			}
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func replaceCredentialAssignments(lines []string, assignments map[string]string) map[string]bool {
	replaced := map[string]bool{}
	for i, line := range lines {
		key, ok := credentialLineKey(line)
		if !ok {
			continue
		}
		if value, hit := assignments[key]; hit {
			lines[i] = key + "=" + value
			replaced[key] = true
		}
	}
	return replaced
}

func sortedCredentialKeys(assignments map[string]string) []string {
	keys := make([]string, 0, len(assignments))
	for key := range assignments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func removeCredentialFromFile(path, key string) error {
	lines, err := readCredentialFileLines(path)
	if err != nil {
		return err
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if k, ok := credentialLineKey(line); ok && k == key {
			continue
		}
		if k, ok := credentialClearedLineKey(line); ok && k == key {
			continue
		}
		out = append(out, line)
	}
	out = append(out, credentialClearedPrefix+key)
	return writeCredentialFileLines(path, out)
}

func readCredentialFileLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func writeCredentialFileLines(path string, lines []string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("credentials store unavailable")
	}
	dir := filepath.Dir(path)
	if err := ensureCredentialFileDir(dir); err != nil {
		return err
	}
	tmpPath, err := writeCredentialTempFile(dir, credentialFileContents(lines))
	if err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := fileutil.ReplaceFile(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func credentialFileContents(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func ensureCredentialFileDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}

func writeCredentialTempFile(dir, contents string) (string, error) {
	tmp, err := os.CreateTemp(dir, "credentials.*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func credentialLineKey(line string) (string, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(line), "export ")
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	key, _, ok := strings.Cut(trimmed, "=")
	key = strings.TrimSpace(key)
	return key, ok && isCredentialKey(key)
}

func credentialClearedLineKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, credentialClearedPrefix) {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(trimmed, credentialClearedPrefix))
	return key, isCredentialKey(key)
}

func isCredentialKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if !isCredentialKeyRune(i, r) {
			return false
		}
	}
	return true
}

func isCredentialKeyRune(index int, r rune) bool {
	if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
		return true
	}
	return index > 0 && r >= '0' && r <= '9'
}

func envFileHasValue(path, key string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	value, ok := envFileValue(path, key)
	return ok && strings.TrimSpace(value) != ""
}

func envFileHasClearedKey(path, key string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	lines, err := readCredentialFileLines(path)
	if err != nil {
		return false
	}
	for _, line := range lines {
		if k, ok := credentialClearedLineKey(line); ok && k == key {
			return true
		}
	}
	return false
}
