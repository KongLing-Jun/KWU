package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/userctx"
)

func TestMigrateLegacyMemoryCompilerForRootRemovesActiveProfileAndProjectKeys(t *testing.T) {
	isolateUserConfigHome(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)
	userPath := paths.UserConfigPath()
	projectPath := ProjectConfigPathForRoot(root)
	legacyStatePath := filepath.Join(paths.MemoryUserDir(), "projects", WorkspaceSlug(root), "memory", "compiler", "state.json")

	writeRetiredMemoryCompilerTestFile(t, userPath, "[agent]\nmemory_compiler = { enabled = true }\ntemperature = 0.2\n")
	writeRetiredMemoryCompilerTestFile(t, projectPath, "[agent]\nmemory_compiler = { enabled = false }\nreasoning_language = \"zh\"\n")
	writeRetiredMemoryCompilerTestFile(t, legacyStatePath, `{"legacy":true}`)

	changed, err := MigrateLegacyMemoryCompilerForRoot(root, paths)
	if err != nil {
		t.Fatalf("MigrateLegacyMemoryCompilerForRoot: %v", err)
	}
	if !changed {
		t.Fatal("migration did not report the retired keys")
	}
	for _, path := range []string{userPath, projectPath} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "memory_compiler") {
			t.Fatalf("retired key survived in %s:\n%s", path, body)
		}
	}
	legacyState, err := os.ReadFile(legacyStatePath)
	if err != nil || string(legacyState) != `{"legacy":true}` {
		t.Fatalf("legacy state was changed: body %q, err %v", legacyState, err)
	}
	changed, err = MigrateLegacyMemoryCompilerForRoot(root, paths)
	if err != nil || changed {
		t.Fatalf("second migration = changed %v, err %v; want no-op", changed, err)
	}
}

func TestStripLegacyMemoryCompilerLinesPreservesMultilinePrompt(t *testing.T) {
	raw := `[agent]
system_prompt = """
Historical example:
memory_compiler = { enabled = true }
"""
memory_compiler = { enabled = false }
temperature = 0.2
`
	wantPrompt := "system_prompt = \"\"\"\nHistorical example:\nmemory_compiler = { enabled = true }\n\"\"\""
	got, changed := stripLegacyMemoryCompilerLines(raw)
	if !changed {
		t.Fatal("real retired key was not removed")
	}
	if !strings.Contains(got, wantPrompt) {
		t.Fatalf("multiline prompt changed:\n%s", got)
	}
	if strings.Contains(got, "memory_compiler = { enabled = false }") {
		t.Fatalf("real retired key survived:\n%s", got)
	}
}

func TestMigrateLegacyMemoryCompilerForRootReportsUnreadableTarget(t *testing.T) {
	isolateUserConfigHome(t)
	root := t.TempDir()
	paths := NewProfilePathResolver(userctx.Default(root))
	projectPath := ProjectConfigPathForRoot(root)
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	changed, err := MigrateLegacyMemoryCompilerForRoot(root, paths)
	if err == nil || changed {
		t.Fatalf("migration = changed %v, err %v; want unchanged error", changed, err)
	}
}

func TestMigrateLegacyMemoryCompilerFilePreservesSymlinkWithRetiredKey(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.toml")
	link := filepath.Join(dir, "config.toml")
	body := "[agent]\nmemory_compiler = { enabled = true }\ntemperature = 0.2\n"
	writeRetiredMemoryCompilerTestFile(t, target, body)
	createRetiredMemoryCompilerSymlink(t, target, link)

	changed, err := migrateLegacyMemoryCompilerFile(link)
	if err == nil || changed {
		t.Fatalf("symlink migration = changed %v, err %v; want unchanged error", changed, err)
	}
	if !strings.Contains(err.Error(), link) || !strings.Contains(err.Error(), "refusing to rewrite symlinked config") {
		t.Fatalf("symlink migration error = %v", err)
	}
	assertRetiredMemoryCompilerSymlinkUnchanged(t, link, target, body)
}

func TestMigrateLegacyMemoryCompilerFileIgnoresSymlinkWithoutRetiredKey(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.toml")
	link := filepath.Join(dir, "config.toml")
	body := "[agent]\ntemperature = 0.2\n"
	writeRetiredMemoryCompilerTestFile(t, target, body)
	createRetiredMemoryCompilerSymlink(t, target, link)

	changed, err := migrateLegacyMemoryCompilerFile(link)
	if err != nil || changed {
		t.Fatalf("symlink no-op migration = changed %v, err %v; want no-op", changed, err)
	}
	assertRetiredMemoryCompilerSymlinkUnchanged(t, link, target, body)
}

func TestMigrateLegacyMemoryCompilerForRootContinuesAfterSymlinkError(t *testing.T) {
	isolateUserConfigHome(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)
	userPath := paths.UserConfigPath()
	projectPath := ProjectConfigPathForRoot(root)
	projectTarget := filepath.Join(t.TempDir(), "project.toml")
	projectBody := "[agent]\nmemory_compiler = { enabled = false }\nreasoning_language = \"zh\"\n"

	writeRetiredMemoryCompilerTestFile(t, userPath, "[agent]\nmemory_compiler = { enabled = true }\ntemperature = 0.2\n")
	writeRetiredMemoryCompilerTestFile(t, projectTarget, projectBody)
	createRetiredMemoryCompilerSymlink(t, projectTarget, projectPath)

	changed, err := MigrateLegacyMemoryCompilerForRoot(root, paths)
	if err == nil || !changed {
		t.Fatalf("partial migration = changed %v, err %v; want changed with error", changed, err)
	}
	userBody, readErr := os.ReadFile(userPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(userBody), "memory_compiler") {
		t.Fatalf("writable user config was not migrated:\n%s", userBody)
	}
	if !strings.Contains(err.Error(), projectPath) {
		t.Fatalf("partial migration error = %v, want project path %s", err, projectPath)
	}
	assertRetiredMemoryCompilerSymlinkUnchanged(t, projectPath, projectTarget, projectBody)
}

func createRetiredMemoryCompilerSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
}

func assertRetiredMemoryCompilerSymlinkUnchanged(t *testing.T, link, target, want string) {
	t.Helper()
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config path %s is no longer a symlink: mode %v", link, info.Mode())
	}
	for _, path := range []string{link, target} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != want {
			t.Fatalf("%s changed through symlink migration:\n%s", path, body)
		}
	}
}

func writeRetiredMemoryCompilerTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
}

func isolateMigrationEnv(t *testing.T) (dest string, legacyRoots []string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	for _, name := range []string{
		"CLOUVIA_HOME", "KWU_HOME", "SUIYUAN_HOME", "REASONIX_HOME",
		"CLOUVIA_STATE_HOME", "KWU_STATE_HOME", "SUIYUAN_STATE_HOME", "REASONIX_STATE_HOME",
		"CLOUVIA_CACHE_HOME", "KWU_CACHE_HOME", "SUIYUAN_CACHE_HOME", "REASONIX_CACHE_HOME",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("KWU_CREDENTIALS_STORE", "file")
	t.Setenv("DEEPSEEK_API_KEY", "")

	previous := automaticUserDataMigrationEnabled
	automaticUserDataMigrationEnabled = true
	t.Cleanup(func() { automaticUserDataMigrationEnabled = previous })
	return canonicalHomeDir(), legacyHomeDirs()
}

func TestMigrateLegacyIfNeededCopiesFirstLegacyRootAndKeepsSource(t *testing.T) {
	dest, roots := isolateMigrationEnv(t)
	if len(roots) < 3 {
		t.Fatalf("legacy roots = %v, want KWU/SuiYuan/Reasonix", roots)
	}
	src := roots[0]
	legacyFile := filepath.Join(src, "nested", "config.toml")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i, root := range roots[1:] {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte{byte('2' + i)}, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	res, err := MigrateLegacyIfNeeded()
	if err != nil {
		t.Fatalf("MigrateLegacyIfNeeded() error = %v", err)
	}
	if res == nil || !samePath(res.From, src) || !samePath(res.To, dest) {
		t.Fatalf("migration result = %+v, want %s -> %s", res, src, dest)
	}
	got, err := os.ReadFile(filepath.Join(dest, "nested", "config.toml"))
	if err != nil || string(got) != "legacy-data" {
		t.Fatalf("canonical copy = %q, %v", got, err)
	}
	if got, err := os.ReadFile(legacyFile); err != nil || string(got) != "legacy-data" {
		t.Fatalf("legacy source was changed: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "source.txt")); !os.IsNotExist(err) {
		t.Fatalf("lower-priority legacy root was merged, stat err=%v", err)
	}
}

func TestMigrateLegacyIfNeededDoesNotOverwriteCanonicalRoot(t *testing.T) {
	dest, roots := isolateMigrationEnv(t)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	canonicalFile := filepath.Join(dest, "config.toml")
	if err := os.WriteFile(canonicalFile, []byte("canonical"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(roots[0], 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roots[0], "config.toml"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := MigrateLegacyIfNeeded()
	if err != nil || res != nil {
		t.Fatalf("MigrateLegacyIfNeeded() = (%+v, %v), want nil result and nil error", res, err)
	}
	got, err := os.ReadFile(canonicalFile)
	if err != nil || string(got) != "canonical" {
		t.Fatalf("canonical file = %q, %v", got, err)
	}
}

func TestMigrateLegacyIfNeededSkipsExplicitHomeOverride(t *testing.T) {
	_, roots := isolateMigrationEnv(t)
	if err := os.MkdirAll(roots[0], 0o755); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "explicit")
	t.Setenv("CLOUVIA_HOME", explicit)

	res, err := MigrateLegacyIfNeeded()
	if err != nil || res != nil {
		t.Fatalf("MigrateLegacyIfNeeded() = (%+v, %v), want nil result and nil error", res, err)
	}
	if _, err := os.Stat(explicit); !os.IsNotExist(err) {
		t.Fatalf("explicit home should not be created, stat err=%v", err)
	}
}

func TestAutomaticUserDataMigrationAllowedHonorsProcessIsolation(t *testing.T) {
	_, _ = isolateMigrationEnv(t)
	if !AutomaticUserDataMigrationAllowed() {
		t.Fatal("isolated ordinary process should allow automatic migration")
	}

	automaticUserDataMigrationEnabled = false
	if AutomaticUserDataMigrationAllowed() {
		t.Fatal("Go test process gate should disable automatic migration")
	}

	automaticUserDataMigrationEnabled = true
	t.Setenv("CLOUVIA_HOME", filepath.Join(t.TempDir(), "explicit"))
	if AutomaticUserDataMigrationAllowed() {
		t.Fatal("explicit home override should disable automatic migration")
	}

	t.Setenv("CLOUVIA_HOME", "")
	t.Setenv("KWU_STATE_HOME", filepath.Join(t.TempDir(), "explicit-state"))
	if AutomaticUserDataMigrationAllowed() {
		t.Fatal("explicit state override should disable automatic migration")
	}
}

func TestMigrateLegacyCredentialsForRootIsNoop(t *testing.T) {
	dest, _ := isolateMigrationEnv(t)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "config.toml"), []byte(`default_model = "deepseek/deepseek-chat"`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateLegacyCredentialsForRoot("."); err != nil {
		t.Fatalf("MigrateLegacyCredentialsForRoot() error = %v", err)
	}
	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "" {
		t.Fatalf("legacy credentials should not be imported, got %q", got)
	}
}

func TestMigrateMCPToUserConfigOnUpgradeIsNoop(t *testing.T) {
	dest, _ := isolateMigrationEnv(t)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
[[plugins]]
name = "local"
command = "local-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := MigrateMCPToUserConfigOnUpgrade([]string{project})
	if err != nil || res != nil {
		t.Fatalf("MigrateMCPToUserConfigOnUpgrade() = (%+v, %v), want nil result and nil error", res, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("MCP migration should not create user config, stat err=%v", err)
	}
}
