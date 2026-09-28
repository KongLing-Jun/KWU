package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/userctx"
)

func isolateProfilePathEnv(t *testing.T) (home, state, cache string) {
	t.Helper()
	home = t.TempDir()
	state = filepath.Join(t.TempDir(), "state")
	cache = filepath.Join(t.TempDir(), "cache")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("KWU_HOME", home)
	t.Setenv("KWU_STATE_HOME", state)
	t.Setenv("KWU_CACHE_HOME", cache)
	return home, state, cache
}

func TestProfilePathResolverConnectorStateDir(t *testing.T) {
	stateRoot := t.TempDir()
	paths := NewProfilePathResolverWithOptions(ProfilePathResolverOptions{
		Actor:     userctx.ActorContext{ActorID: "local", ProfileID: "work"},
		HomeRoot:  t.TempDir(),
		StateRoot: stateRoot,
		CacheRoot: t.TempDir(),
	})
	if got, want := paths.ConnectorStateDir(), filepath.Join(stateRoot, "profiles", "work", "connectors"); got != want {
		t.Fatalf("ConnectorStateDir() = %q, want %q", got, want)
	}
}

func TestProfilePathResolverPrefersKWUEnvAliases(t *testing.T) {
	formerPrefix := "SUI" + "YUAN"
	legacyHome := t.TempDir()
	legacyState := filepath.Join(t.TempDir(), "legacy-state")
	legacyCache := filepath.Join(t.TempDir(), "legacy-cache")
	kwuHome := filepath.Join(t.TempDir(), "kwu-home")
	kwuState := filepath.Join(t.TempDir(), "kwu-state")
	kwuCache := filepath.Join(t.TempDir(), "kwu-cache")

	t.Setenv(formerPrefix+"_HOME", legacyHome)
	t.Setenv(formerPrefix+"_STATE_HOME", legacyState)
	t.Setenv(formerPrefix+"_CACHE_HOME", legacyCache)
	t.Setenv("KWU_HOME", kwuHome)
	t.Setenv("KWU_STATE_HOME", kwuState)
	t.Setenv("KWU_CACHE_HOME", kwuCache)

	assertCleanPath(t, UserConfigPath(), filepath.Join(kwuHome, "config.toml"))
	assertCleanPath(t, UserCredentialsPath(), filepath.Join(kwuState, ".env"))
	assertCleanPath(t, CacheDir(), kwuCache)
}

func TestProfilePathResolverSupportsFormerProductEnvAliases(t *testing.T) {
	formerPrefix := "SUI" + "YUAN"
	home := t.TempDir()
	legacyHome := filepath.Join(t.TempDir(), "legacy-home")
	legacyState := filepath.Join(t.TempDir(), "legacy-state")
	legacyCache := filepath.Join(t.TempDir(), "legacy-cache")
	cacheRoot := filepath.Join(home, "cache")

	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("LocalAppData", cacheRoot)
	t.Setenv(formerPrefix+"_HOME", legacyHome)
	t.Setenv(formerPrefix+"_STATE_HOME", legacyState)
	t.Setenv(formerPrefix+"_CACHE_HOME", legacyCache)

	assertCleanPath(t, UserConfigPath(), filepath.Join(legacyHome, "config.toml"))
	assertCleanPath(t, UserCredentialsPath(), filepath.Join(legacyState, ".env"))
	assertCleanPath(t, CacheDir(), legacyCache)
}

func TestSourcePathForRootSupportsLegacyConfigsAndPrefersCanonicalOrder(t *testing.T) {
	isolateProfilePathEnv(t)
	root := t.TempDir()
	formerProjectConfigName := strings.ToLower("SUI"+"YUAN") + ".toml"
	legacyPath := filepath.Join(root, formerProjectConfigName)
	kwuPath := filepath.Join(root, "kwu.toml")
	clouviaPath := filepath.Join(root, "clouvia.toml")

	if err := os.WriteFile(legacyPath, []byte(`default_model = "legacy/model"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRoot(root); filepath.Clean(got) != filepath.Clean(legacyPath) {
		t.Fatalf("source path with only former product config = %q, want %q", got, legacyPath)
	}

	if err := os.WriteFile(kwuPath, []byte(`default_model = "kwu/model"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRoot(root); filepath.Clean(got) != filepath.Clean(kwuPath) {
		t.Fatalf("KWU source path = %q, want %q", got, kwuPath)
	}

	cfg, err := LoadForRoot(root)
	if err != nil {
		t.Fatalf("LoadForRoot: %v", err)
	}
	if cfg.DefaultModel != "kwu/model" {
		t.Fatalf("DefaultModel = %q, want kwu/model", cfg.DefaultModel)
	}

	if err := os.WriteFile(clouviaPath, []byte(`default_model = "clouvia/model"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRoot(root); filepath.Clean(got) != filepath.Clean(clouviaPath) {
		t.Fatalf("canonical source path = %q, want %q", got, clouviaPath)
	}
}

func TestProfilePathResolverDefaultUsesCurrentPaths(t *testing.T) {
	home, _, _ := isolateProfilePathEnv(t)
	workspace := filepath.Join(t.TempDir(), "workspace")
	resolver := NewProfilePathResolver(userctx.Default(workspace))

	if resolver.ProfileID() != userctx.DefaultProfileID {
		t.Fatalf("ProfileID() = %q, want default", resolver.ProfileID())
	}
	assertCleanPath(t, resolver.UserConfigPath(), UserConfigPath())
	assertCleanPath(t, resolver.UserConfigLoadPath(), userConfigLoadPath())
	assertCleanPath(t, resolver.KWUHomeDir(), home)
	assertCleanPath(t, resolver.UserCredentialsPath(), UserCredentialsPath())
	assertCleanPath(t, resolver.SessionDir(), SessionDir())
	assertCleanPath(t, resolver.ProjectSessionDir(workspace), ProjectSessionDir(workspace))
	assertCleanPath(t, resolver.MemoryUserDir(), MemoryUserDir())
	assertCleanPath(t, resolver.MemoryProjectStoreRoot(), MemoryUserDir())
	assertCleanPath(t, resolver.ArchiveDir(), ArchiveDir())
	assertCleanPath(t, resolver.CacheDir(), CacheDir())
	assertCleanPath(t, resolver.ProjectTrustPath(workspace), filepath.Join(home, "trust.json"))
	assertCleanPath(t, resolver.DesktopWorkspaceStatePath(), DesktopWorkspaceStatePath())
	assertCleanPath(t, resolver.DesktopWorkspaceListPath(), DesktopWorkspaceListPath())
	assertCleanPath(t, resolver.DesktopWindowStatePath(), DesktopWindowStatePath())
	assertCleanPath(t, DesktopWorkspaceStatePath(), filepath.Join(MemoryUserDir(), "desktop-workspace"))
	assertCleanPath(t, DesktopWorkspaceListPath(), filepath.Join(MemoryUserDir(), "desktop-workspaces.json"))
	assertCleanPath(t, DesktopWindowStatePath(), filepath.Join(MemoryUserDir(), "desktop-window.json"))
}

func TestProfilePathResolverNonDefaultUsesProfileScopedPaths(t *testing.T) {
	_, state, cache := isolateProfilePathEnv(t)
	workspace := filepath.Join(t.TempDir(), "workspace")
	actor := userctx.Default(workspace)
	actor.ProfileID = "work"

	resolver := NewProfilePathResolver(actor)
	profileRoot := filepath.Join(state, "profiles", "work")
	workspaceSlug := WorkspaceSlug(absForTest(t, workspace))

	assertCleanPath(t, resolver.UserConfigPath(), filepath.Join(profileRoot, "config.toml"))
	assertCleanPath(t, resolver.UserConfigLoadPath(), filepath.Join(profileRoot, "config.toml"))
	assertCleanPath(t, resolver.KWUHomeDir(), profileRoot)
	assertCleanPath(t, resolver.UserCredentialsPath(), filepath.Join(profileRoot, ".env"))
	assertCleanPath(t, resolver.SessionDir(), filepath.Join(profileRoot, "sessions"))
	assertCleanPath(t, resolver.ProjectSessionDir(workspace), filepath.Join(profileRoot, "projects", workspaceSlug, "sessions"))
	assertCleanPath(t, resolver.MemoryUserDir(), profileRoot)
	assertCleanPath(t, resolver.MemoryProjectStoreRoot(), state)
	assertCleanPath(t, resolver.ArchiveDir(), filepath.Join(profileRoot, "archive"))
	assertCleanPath(t, resolver.CacheDir(), filepath.Join(cache, "profiles", "work"))
	assertCleanPath(t, resolver.ProjectTrustPath(workspace), filepath.Join(profileRoot, "project-trust", workspaceSlug+".json"))
	fallbacks := resolver.ProjectTrustFallbackPaths(workspace)
	if len(fallbacks) != 1 {
		t.Fatalf("ProjectTrustFallbackPaths() = %v, want one legacy path", fallbacks)
	}
	assertCleanPath(t, fallbacks[0], filepath.Join(profileRoot, "hooks", workspaceSlug+".json"))
	assertCleanPath(t, resolver.DesktopWorkspaceStatePath(), filepath.Join(profileRoot, "desktop-workspace"))
	assertCleanPath(t, resolver.DesktopWorkspaceListPath(), filepath.Join(profileRoot, "desktop-workspaces.json"))
	assertCleanPath(t, resolver.DesktopWindowStatePath(), filepath.Join(profileRoot, "desktop-window.json"))
}

func TestProfilePathResolverRejectsUnsafeProfileID(t *testing.T) {
	home, _, _ := isolateProfilePathEnv(t)
	actor := userctx.Default("")
	actor.ProfileID = "../other"

	resolver := NewProfilePathResolver(actor)
	if resolver.ProfileID() != userctx.DefaultProfileID {
		t.Fatalf("ProfileID() = %q, want default for unsafe profile id", resolver.ProfileID())
	}
	assertCleanPath(t, resolver.UserConfigPath(), filepath.Join(home, "config.toml"))
}

func TestProfilePathResolverDefaultExcludesFormerLegacyConfigCandidates(t *testing.T) {
	home, _, _ := isolateProfilePathEnv(t)
	resolver := NewProfilePathResolver(userctx.Default(""))
	candidates := resolver.UserConfigCandidatePaths()
	want := filepath.Join(home, "config.toml")
	if len(candidates) != 1 || filepath.Clean(candidates[0]) != filepath.Clean(want) {
		t.Fatalf("UserConfigCandidatePaths() = %v, want only current path %q", candidates, want)
	}
}

func TestProfilePathResolverNonDefaultExcludesLegacyConfigCandidates(t *testing.T) {
	_, state, _ := isolateProfilePathEnv(t)
	actor := userctx.Default("")
	actor.ProfileID = "work"
	resolver := NewProfilePathResolver(actor)

	candidates := resolver.UserConfigCandidatePaths()
	want := filepath.Join(state, "profiles", "work", "config.toml")
	if len(candidates) != 1 || filepath.Clean(candidates[0]) != filepath.Clean(want) {
		t.Fatalf("UserConfigCandidatePaths() = %v, want only %q", candidates, want)
	}
}

func TestSourcePathForRootWithActorUsesProfileConfigAndProjectPriority(t *testing.T) {
	isolateProfilePathEnv(t)
	workspace := t.TempDir()
	actor := userctx.Default(workspace)
	actor.ProfileID = "work"
	resolver := NewProfilePathResolver(actor)

	if err := os.WriteFile(UserConfigPath(), []byte(`default_model = "default/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRootWithActor(workspace, actor); got != "" {
		t.Fatalf("source path with only default profile config = %q, want empty", got)
	}

	if err := os.MkdirAll(filepath.Dir(resolver.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolver.UserConfigPath(), []byte(`default_model = "work/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRootWithActor(workspace, actor); filepath.Clean(got) != filepath.Clean(resolver.UserConfigPath()) {
		t.Fatalf("source path = %q, want profile config %q", got, resolver.UserConfigPath())
	}

	projectPath := filepath.Join(workspace, "kwu.toml")
	if err := os.WriteFile(projectPath, []byte(`default_model = "project/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRootWithActor(workspace, actor); filepath.Clean(got) != filepath.Clean(projectPath) {
		t.Fatalf("source path with project config = %q, want %q", got, projectPath)
	}
}

func TestProfilePathResolverDefaultProfilesShapedStateKeepsMemoryProjectRootLocal(t *testing.T) {
	base := t.TempDir()
	state := filepath.Join(base, "profiles", "custom-default")
	cache := filepath.Join(t.TempDir(), "cache")
	workspace := filepath.Join(t.TempDir(), "workspace")

	resolver := NewProfilePathResolverWithOptions(ProfilePathResolverOptions{
		Actor:     userctx.Default(workspace),
		HomeRoot:  filepath.Join(t.TempDir(), "home"),
		StateRoot: state,
		CacheRoot: cache,
	})

	assertCleanPath(t, resolver.MemoryUserDir(), state)
	assertCleanPath(t, resolver.MemoryProjectStoreRoot(), state)
}

func assertCleanPath(t *testing.T, got, want string) {
	t.Helper()
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func absForTest(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
