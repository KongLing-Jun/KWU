package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kwu/internal/userctx"
)

func TestLoadForRootWithPathsUsesProfileUserConfigAndProjectPriority(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	project := t.TempDir()
	actor := userctx.Default(project)
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)

	if err := os.MkdirAll(filepath.Dir(UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserConfigPath(), []byte(`default_model = "default/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`default_model = "profile/user"`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadForRootWithPaths(project, paths)
	if err != nil {
		t.Fatalf("LoadForRootWithPaths without project config: %v", err)
	}
	if cfg.DefaultModel != "profile/user" {
		t.Fatalf("DefaultModel = %q, want profile user config", cfg.DefaultModel)
	}

	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`default_model = "project/workspace"`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadForRootWithPaths(project, paths)
	if err != nil {
		t.Fatalf("LoadForRootWithPaths with project config: %v", err)
	}
	if cfg.DefaultModel != "project/workspace" {
		t.Fatalf("DefaultModel = %q, want project config to override profile user config", cfg.DefaultModel)
	}
}

func TestLoadForRootCredentialPolicyKeepsMetadataCredentialFreeAcrossLoadOrder(t *testing.T) {
	for _, sequence := range []struct {
		name     string
		metadata []bool
	}{
		{"metadata_then_byok", []bool{true, false}},
		{"byok_then_metadata", []bool{false, true}},
	} {
		t.Run(sequence.name, func(t *testing.T) {
			_, _, _ = isolateProfilePathEnv(t)
			project := t.TempDir()
			key := "PROFILE_CREDENTIAL_POLICY_TEST_KEY"
			t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreKeyring)
			t.Setenv(key, "")
			os.Unsetenv(key)

			actor := userctx.Default(project)
			actor.ProfileID = "byok-profile"
			paths := NewProfilePathResolver(actor)
			defaultPaths := NewProfilePathResolver(userctx.Default(project))
			for _, credential := range []struct {
				paths ProfilePathResolver
				value string
			}{
				{defaultPaths, "default-profile-value"},
				{paths, "current-profile-value"},
			} {
				if err := os.MkdirAll(filepath.Dir(credential.paths.UserCredentialsPath()), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(credential.paths.UserCredentialsPath(), []byte(key+"="+credential.value+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
default_model = "fixture/model"
[[providers]]
name = "fixture"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "model"
api_key_env = "`+key+`"
`), 0o644); err != nil {
				t.Fatal(err)
			}

			lookups := 0
			owner := CredentialOwnerForPaths(paths)
			oldLookup := keyringCredentialValueLookup
			keyringCredentialValueLookup = func(gotOwner CredentialOwner, gotKey string) (string, bool) {
				lookups++
				if gotOwner.Normalize() != owner {
					t.Fatalf("keyring owner = %+v, want current profile %+v", gotOwner.Normalize(), owner)
				}
				if gotKey != key {
					t.Fatalf("keyring key = %q, want %q", gotKey, key)
				}
				return "", false
			}
			t.Cleanup(func() { keyringCredentialValueLookup = oldLookup })

			for _, metadata := range sequence.metadata {
				before := lookups
				var cfg *Config
				var err error
				if metadata {
					cfg, err = LoadForRootMetadataWithActor(project, actor)
				} else {
					cfg, err = LoadForRootWithActor(project, actor)
				}
				if err != nil {
					t.Fatal(err)
				}
				entry, ok := cfg.Provider("fixture")
				if !ok {
					t.Fatal("config lost fixture provider")
				}
				if metadata {
					if lookups != before {
						t.Fatalf("metadata loader keyring lookups = %d, want %d", lookups, before)
					}
					if got := entry.APIKey(); got != "" {
						t.Fatalf("metadata APIKey = %q, want empty", got)
					}
					if lookups != before {
						t.Fatalf("metadata APIKey keyring lookups = %d, want %d", lookups, before)
					}
					if !entry.resolvedAPIKeySet {
						t.Fatal("metadata provider remains eligible for lazy credential lookup")
					}
					continue
				}
				if got := entry.APIKey(); got != "current-profile-value" {
					t.Fatalf("BYOK APIKey = %q, want current profile credential", got)
				}
				if lookups != before+1 {
					t.Fatalf("BYOK keyring lookups = %d, want %d", lookups, before+1)
				}
			}
		})
	}
}

func TestLoadForRootWithPathsUsesProfileDerivedUserSettings(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	project := t.TempDir()
	actor := userctx.Default(project)
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)

	if err := os.MkdirAll(filepath.Dir(UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserConfigPath(), []byte(`
credentials_store = "keyring"

[agent]
auto_plan = "off"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`
credentials_store = "file"

[agent]
auto_plan = "ask"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
credentials_store = "keyring"

[agent]
auto_plan = "off"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadForRootWithPaths(project, paths)
	if err != nil {
		t.Fatalf("LoadForRootWithPaths: %v", err)
	}
	if cfg.CredentialsStore != CredentialsStoreFile {
		t.Fatalf("CredentialsStore = %q, want profile user config", cfg.CredentialsStore)
	}
	if cfg.Agent.AutoPlan != "on" {
		t.Fatalf("Agent.AutoPlan = %q, want profile user auto_plan normalized on", cfg.Agent.AutoPlan)
	}
}

func TestLoadForRootWithPathsProviderCredentialsDoNotFallbackToDefaultProfile(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "KEY_PROFILE_PROVIDER_NO_DEFAULT_FALLBACK"
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreFile)
	t.Setenv(key, "")
	os.Unsetenv(key)

	defaultPaths := NewProfilePathResolver(userctx.Default(project))
	actor := userctx.Default(project)
	actor.ProfileID = "work"
	workPaths := NewProfilePathResolver(actor)

	if err := os.MkdirAll(filepath.Dir(defaultPaths.UserCredentialsPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultPaths.UserCredentialsPath(), []byte(key+"=from_default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
default_model = "custom/m"
[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "m"
api_key_env = "`+key+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadForRootWithPaths(project, workPaths)
	if err != nil {
		t.Fatalf("LoadForRootWithPaths: %v", err)
	}
	provider, ok := cfg.Provider("custom")
	if !ok {
		t.Fatalf("provider missing: %+v", cfg.Providers)
	}
	if got := provider.APIKey(); got != "" {
		t.Fatalf("work profile provider API key = %q, want no fallback to default profile", got)
	}
	if got := os.Getenv(key); got != "" {
		t.Fatalf("process env = %q, want default profile credentials not loaded", got)
	}
}

func TestLoadForRootWithPathsProviderCredentialsIgnoreProjectDotEnv(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	project := t.TempDir()
	key := "KEY_PROFILE_PROVIDER_PROJECT_ENV"
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreFile)
	t.Setenv(key, "")
	os.Unsetenv(key)

	actor := userctx.Default(project)
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)

	if err := os.MkdirAll(filepath.Dir(paths.UserCredentialsPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserCredentialsPath(), []byte(key+"=from_profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte(key+"=from_project\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "kwu.toml"), []byte(`
default_model = "custom/m"
[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "m"
api_key_env = "`+key+`"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadForRootWithPaths(project, paths)
	if err != nil {
		t.Fatalf("LoadForRootWithPaths: %v", err)
	}
	provider, ok := cfg.Provider("custom")
	if !ok {
		t.Fatalf("provider missing: %+v", cfg.Providers)
	}
	if got := provider.APIKey(); got != "from_profile" {
		t.Fatalf("provider API key = %q, want profile credentials", got)
	}
	if got := provider.APIKeySourceLabel(); !strings.Contains(got, "profile work") {
		t.Fatalf("provider source label = %q, want profile owner", got)
	}
	res := ResolveCredentialForRootGlobalFirstWithPaths(project, paths, key)
	foundProjectShadow := false
	for _, shadowed := range res.Shadowed {
		if shadowed.Kind == CredentialSourceProjectEnv {
			foundProjectShadow = true
			break
		}
	}
	if !foundProjectShadow {
		t.Fatalf("shadowed sources = %+v, want project .env reported as shadowed", res.Shadowed)
	}
}

func TestLoadForEditWithPathsUsesProfileUserConfig(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	actor := userctx.Default("")
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)

	if err := os.MkdirAll(filepath.Dir(UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserConfigPath(), []byte(`default_model = "default/profile"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.UserConfigPath(), []byte(`default_model = "profile/user"`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadForEditWithPaths(paths)
	if cfg.DefaultModel != "profile/user" {
		t.Fatalf("DefaultModel = %q, want profile user config", cfg.DefaultModel)
	}
	cfg.DefaultModel = "profile/updated"
	if err := cfg.SaveForRootWithPaths("", paths); err != nil {
		t.Fatalf("SaveForRootWithPaths: %v", err)
	}
	if got := LoadForEdit(UserConfigPath()).DefaultModel; got != "default/profile" {
		t.Fatalf("default profile config was modified: DefaultModel = %q", got)
	}
	if got := LoadForEdit(paths.UserConfigPath()).DefaultModel; got != "profile/updated" {
		t.Fatalf("profile config DefaultModel = %q, want updated value", got)
	}
}

func TestLoadForEditWithPathsLoadsProfileCredentialsAndClearsStaleProfileEnv(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	key := "KEY_LOAD_EDIT_PROFILE_SECRET"
	t.Setenv(key, "")
	os.Unsetenv(key)

	defaultPaths := NewProfilePathResolver(userctx.Default(""))
	actorA := userctx.Default("")
	actorA.ProfileID = "profile-a"
	pathsA := NewProfilePathResolver(actorA)
	actorB := userctx.Default("")
	actorB.ProfileID = "profile-b"
	pathsB := NewProfilePathResolver(actorB)

	writeBotConfig := func(paths ProfilePathResolver) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `
[bot]
enabled = true

[bot.feishu]
enabled = true
app_id = "app"
app_secret_env = "` + key + `"
`
		if err := os.WriteFile(paths.UserConfigPath(), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeCredentials := func(paths ProfilePathResolver, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(paths.UserCredentialsPath()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.UserCredentialsPath(), []byte(key+"="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeCredentials(defaultPaths, "from_default")
	writeBotConfig(pathsA)
	writeBotConfig(pathsB)
	writeCredentials(pathsA, "from_profile_a")

	_ = LoadForEditWithPaths(pathsA)
	if got := os.Getenv(key); got != "from_profile_a" {
		t.Fatalf("profile A env = %q, want profile A credential", got)
	}

	_ = LoadForEditWithPaths(pathsB)
	if got := os.Getenv(key); got != "" {
		t.Fatalf("profile B missing env = %q, want no fallback to profile A or default", got)
	}

	writeCredentials(pathsB, "from_profile_b")
	_ = LoadForEditWithPaths(pathsB)
	if got := os.Getenv(key); got != "from_profile_b" {
		t.Fatalf("profile B env = %q, want profile B credential", got)
	}
}

func TestSaveForRootWithPathsWritesProfileUserConfigScope(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	actor := userctx.Default("")
	actor.ProfileID = "work"
	paths := NewProfilePathResolver(actor)

	cfg := Default()
	cfg.Desktop.Theme = "dark"
	cfg.Desktop.CloseBehavior = "background"
	if err := cfg.SaveForRootWithPaths("", paths); err != nil {
		t.Fatalf("SaveForRootWithPaths: %v", err)
	}

	body, err := os.ReadFile(paths.UserConfigPath())
	if err != nil {
		t.Fatalf("read profile config: %v", err)
	}
	if !strings.Contains(string(body), "[desktop]") || !strings.Contains(string(body), "close_behavior") {
		t.Fatalf("profile user config should be rendered with user scope:\n%s", body)
	}
	if info, err := os.Stat(paths.UserConfigPath()); err != nil {
		t.Fatalf("stat profile config: %v", err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("profile user config mode = %o, want 600", info.Mode().Perm())
	}
}
