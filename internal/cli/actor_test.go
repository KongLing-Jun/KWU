package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/userctx"
)

func TestSetupDefaultActorContextIsLocalProfile(t *testing.T) {
	actor := defaultCLIActorContext().Normalize()
	if actor.ActorID != userctx.DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, userctx.DefaultActorID)
	}
	if actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want %q", actor.ProfileID, userctx.DefaultProfileID)
	}
	if actor.AuthMethod != userctx.DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, userctx.DefaultAuthMethod)
	}
	if actor.WorkspaceRoot != "" {
		t.Fatalf("WorkspaceRoot = %q, want boot.Build to resolve it", actor.WorkspaceRoot)
	}
}

func TestDefaultCLIActorContextUsesKWUProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "work")

	actor := defaultCLIActorContext().Normalize()
	if actor.ActorID != userctx.DefaultActorID {
		t.Fatalf("ActorID = %q, want %q", actor.ActorID, userctx.DefaultActorID)
	}
	if actor.ProfileID != "work" {
		t.Fatalf("ProfileID = %q, want work", actor.ProfileID)
	}
	if actor.AuthMethod != userctx.DefaultAuthMethod {
		t.Fatalf("AuthMethod = %q, want %q", actor.AuthMethod, userctx.DefaultAuthMethod)
	}
}

func TestDefaultCLIActorContextRejectsUnsafeKWUProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "../work")

	if actor := defaultCLIActorContext().Normalize(); actor.ProfileID != userctx.DefaultProfileID {
		t.Fatalf("ProfileID = %q, want default for unsafe profile", actor.ProfileID)
	}
}

func TestResolveSetupTargetsUsesCurrentCLIProfile(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "work")
	paths := config.NewProfilePathResolver(defaultCLIActorContext())

	targets := resolveSetupTargets(nil)
	if filepath.Clean(targets.config) != filepath.Clean(paths.UserConfigPath()) {
		t.Fatalf("setup config target = %q, want %q", targets.config, paths.UserConfigPath())
	}
	if filepath.Clean(targets.env) != filepath.Clean(paths.UserCredentialsPath()) {
		t.Fatalf("setup env target = %q, want %q", targets.env, paths.UserCredentialsPath())
	}

	localTargets := resolveSetupTargets([]string{"--local"})
	if localTargets.config != "clouvia.toml" {
		t.Fatalf("--local setup config target = %q, want clouvia.toml", localTargets.config)
	}
	if filepath.Clean(localTargets.env) != filepath.Clean(paths.UserCredentialsPath()) {
		t.Fatalf("--local setup env target = %q, want %q", localTargets.env, paths.UserCredentialsPath())
	}
}

func TestResolveCLISessionDirUsesCurrentProfileProjectDir(t *testing.T) {
	isolateCLIConfigHome(t)
	cwd := mustGetwd(t)
	t.Setenv(cliProfileEnv, "work")
	paths := config.NewProfilePathResolver(defaultCLIActorContext())

	got := resolveCLISessionDir()
	want := paths.ProjectSessionDir(cwd)
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("resolveCLISessionDir() = %q, want current profile project dir %q", got, want)
	}
	if filepath.Clean(got) == filepath.Clean(config.ProjectSessionDir(cwd)) {
		t.Fatalf("resolveCLISessionDir() used default profile project dir %q", got)
	}
}

func TestLoadCLIConfigUsesProfileConfigAndProjectPriority(t *testing.T) {
	isolateCLIConfigHome(t)
	t.Setenv(cliProfileEnv, "work")
	paths := config.NewProfilePathResolver(defaultCLIActorContext())

	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`default_model = "default/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`default_model = "work/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatalf("loadCLIConfig without project config: %v", err)
	}
	if cfg.DefaultModel != "work/profile" {
		t.Fatalf("DefaultModel = %q, want profile config", cfg.DefaultModel)
	}

	if err := os.WriteFile("kwu.toml", []byte(`default_model = "project/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadCLIConfig()
	if err != nil {
		t.Fatalf("loadCLIConfig with project config: %v", err)
	}
	if cfg.DefaultModel != "project/profile" {
		t.Fatalf("DefaultModel = %q, want project config priority", cfg.DefaultModel)
	}
}

func TestSetupQuietWithActorCarriesDerivedActorContext(t *testing.T) {
	isolateCLIConfigHome(t)
	cwd := mustGetwd(t)
	if err := os.WriteFile(filepath.Join(cwd, "kwu.toml"), []byte(`
default_model = "test-model"

[[providers]]
name = "test-model"
kind = "openai"
base_url = "https://example.invalid"
model = "x"
api_key_env = "KWU_TEST_KEY_UNSET"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := userctx.ActorContext{
		ActorID:       "cli-actor-1",
		ProfileID:     "stage2-profile",
		AuthMethod:    "cli-test",
		ParentActorID: "parent-actor",
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: "cli-test",
			UserID:   "remote-user-1",
		},
	}.Normalize()
	ctrl, err := setupQuietWithActor(context.Background(), "test-model", 0, false, event.Discard, want)
	if err != nil {
		t.Fatalf("setupQuietWithActor: %v", err)
	}
	defer ctrl.Close()

	got := ctrl.ActorContext()
	if got.ActorID != want.ActorID ||
		got.ProfileID != want.ProfileID ||
		got.AuthMethod != want.AuthMethod ||
		got.ParentActorID != want.ParentActorID {
		t.Fatalf("ActorContext identity = %+v, want %+v", got, want)
	}
	if got.RemoteIdentity == nil ||
		got.RemoteIdentity.Provider != want.RemoteIdentity.Provider ||
		got.RemoteIdentity.UserID != want.RemoteIdentity.UserID {
		t.Fatalf("RemoteIdentity = %+v, want %+v", got.RemoteIdentity, want.RemoteIdentity)
	}
	if filepath.Clean(got.WorkspaceRoot) != filepath.Clean(cwd) {
		t.Fatalf("WorkspaceRoot = %q, want %q", got.WorkspaceRoot, cwd)
	}
	wantSessionDir := config.NewProfilePathResolver(want).ProjectSessionDir(cwd)
	if filepath.Clean(ctrl.SessionDir()) != filepath.Clean(wantSessionDir) {
		t.Fatalf("SessionDir = %q, want %q", ctrl.SessionDir(), wantSessionDir)
	}
}
