package config

import (
	"os"
	"path/filepath"
	"testing"
)

func useCanonicalUserDataOnlyForTest(t *testing.T) {
	t.Helper()
	previous := canonicalUserDataOnly.Swap(true)
	t.Cleanup(func() {
		canonicalUserDataOnly.Store(previous)
	})
}

func TestCanonicalUserDataOnlyIgnoresFormerProductRootsAndAliases(t *testing.T) {
	useCanonicalUserDataOnlyForTest(t)

	root := t.TempDir()
	appData := filepath.Join(root, "AppData")
	localAppData := filepath.Join(root, "LocalAppData")
	legacyHome := filepath.Join(appData, "kwu")
	legacyState := filepath.Join(root, "legacy-state")
	legacyCache := filepath.Join(root, "legacy-cache")
	if err := os.MkdirAll(legacyHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyHome, "config.toml"), []byte("language = \"en\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("AppData", appData)
	t.Setenv("LocalAppData", localAppData)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-config"))
	t.Setenv("XDG_CACHE_HOME", localAppData)
	t.Setenv("KWU_HOME", legacyHome)
	t.Setenv("KWU_STATE_HOME", legacyState)
	t.Setenv("KWU_CACHE_HOME", legacyCache)
	t.Setenv("SUIYUAN_HOME", filepath.Join(root, "suiyuan"))
	t.Setenv("REASONIX_HOME", filepath.Join(root, "reasonix"))

	wantHome := defaultCanonicalHomeDir()
	wantCache := defaultCanonicalCacheDir()
	assertCleanPath(t, KWUHomeDir(), wantHome)
	assertCleanPath(t, UserConfigPath(), filepath.Join(wantHome, "config.toml"))
	assertCleanPath(t, UserConfigPath(), userConfigLoadPath())
	assertCleanPath(t, UserCredentialsPath(), filepath.Join(wantHome, ".env"))
	assertCleanPath(t, CacheDir(), wantCache)

	if candidates := LegacyUserConfigPaths(); len(candidates) != 0 {
		t.Fatalf("LegacyUserConfigPaths() = %v, want none", candidates)
	}
	if candidates := legacyCredentialsPaths(); len(candidates) != 0 {
		t.Fatalf("legacyCredentialsPaths() = %v, want none", candidates)
	}
	if AutomaticUserDataMigrationAllowed() {
		t.Fatal("automatic legacy migration must be disabled in canonical-only mode")
	}
}

func TestCanonicalUserDataOnlyHonorsCanonicalOverrides(t *testing.T) {
	useCanonicalUserDataOnlyForTest(t)

	home := filepath.Join(t.TempDir(), "clouvia-home")
	state := filepath.Join(t.TempDir(), "clouvia-state")
	cache := filepath.Join(t.TempDir(), "clouvia-cache")
	t.Setenv("CLOUVIA_HOME", home)
	t.Setenv("CLOUVIA_STATE_HOME", state)
	t.Setenv("CLOUVIA_CACHE_HOME", cache)

	assertCleanPath(t, UserConfigPath(), filepath.Join(home, "config.toml"))
	assertCleanPath(t, UserCredentialsPath(), filepath.Join(state, ".env"))
	assertCleanPath(t, CacheDir(), cache)
}

func TestCanonicalUserDataOnlyDoesNotReadLegacyKeyringSlot(t *testing.T) {
	useCanonicalUserDataOnlyForTest(t)

	calls := 0
	previous := legacyKeyringCredentialValueLookup
	legacyKeyringCredentialValueLookup = func(string) (string, bool) {
		calls++
		return "legacy-secret", true
	}
	t.Cleanup(func() {
		legacyKeyringCredentialValueLookup = previous
	})

	if value, ok := legacyKeyringCredentialValueFallback("API_KEY"); ok || value != "" {
		t.Fatalf("legacy keyring fallback = %q, %v; want empty and disabled", value, ok)
	}
	if calls != 0 {
		t.Fatalf("legacy keyring lookup called %d time(s), want 0", calls)
	}
}

func TestCanonicalUserDataOnlyRetainsWorkspaceConfigCompatibility(t *testing.T) {
	useCanonicalUserDataOnlyForTest(t)

	root := t.TempDir()
	workspaceConfig := filepath.Join(root, "kwu.toml")
	if err := os.WriteFile(workspaceConfig, []byte("default_model = \"workspace/model\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRoot(root); filepath.Clean(got) != filepath.Clean(workspaceConfig) {
		t.Fatalf("SourcePathForRoot() = %q, want workspace config %q", got, workspaceConfig)
	}
}
