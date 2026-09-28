package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadForEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kwu.toml")
	custom := `default_model = "custom"
[[providers]]
name = "custom"
kind = "openai"
base_url = "https://x"
model = "m"
api_key_env = "X_KEY"
`
	if err := os.WriteFile(path, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	// Existing file: its providers/default override the built-in defaults, so a
	// reconfigure preserves the user's setup.
	cfg := LoadForEdit(path)
	if cfg.DefaultModel != "custom" {
		t.Errorf("default_model = %q, want custom", cfg.DefaultModel)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "custom" {
		t.Errorf("providers = %v, want a single custom provider", cfg.Providers)
	}

	// Missing file: falls back to the built-in defaults.
	if cfg := LoadForEdit(filepath.Join(dir, "absent.toml")); cfg.DefaultModel != Default().DefaultModel {
		t.Errorf("missing-file default = %q, want %q", cfg.DefaultModel, Default().DefaultModel)
	}
}

func TestLoadForEditMalformedFallbackRefusesSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kwu.toml")
	original := []byte("default_model = [\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadForEdit(path)
	if cfg.DefaultModel != Default().DefaultModel {
		t.Fatalf("fallback default_model = %q, want %q", cfg.DefaultModel, Default().DefaultModel)
	}
	if err := cfg.SaveTo(path); err == nil {
		t.Fatal("SaveTo after malformed LoadForEdit should fail")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("malformed config was rewritten:\n%s", got)
	}
}

func TestLoadForEditMissingFallbackCanSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.toml")

	cfg := LoadForEdit(path)
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo after missing config LoadForEdit: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved config missing: %v", err)
	}
}

func TestSaveToUserConfigAuditsProviderAndDefaultModelChanges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KWU_HOME", dir)
	path := UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`default_model = "custom/old"

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "old"
api_key_env = "SECRET_ENV"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	origLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origLogger) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	cfg := LoadForEdit(path)
	cfg.Providers[0].Model = "new"
	cfg.DefaultModel = "custom/new"
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	logs := buf.String()
	for _, want := range []string{
		`"msg":"config: write audit"`,
		`"path":"` + strings.ReplaceAll(path, `\`, `\\`) + `"`,
		`"providers_before":["custom"]`,
		`"providers_after":["custom"]`,
		`"default_model_before":"custom/old"`,
		`"default_model_after":"custom/new"`,
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("audit log missing %s in:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "SECRET_ENV") || strings.Contains(logs, "DEEPSEEK_API_KEY") {
		t.Fatalf("audit log leaked credential names:\n%s", logs)
	}
}

func TestSaveToUserConfigWarnsWhenCustomProvidersDropToOfficialDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KWU_HOME", dir)
	path := UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`default_model = "custom/model"

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "model"
api_key_env = "SECRET_ENV"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	origLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origLogger) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	cfg := Default()
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	logs := buf.String()
	for _, want := range []string{
		`"level":"WARN"`,
		`"msg":"config: possible custom provider loss"`,
		`"removed_custom_providers":["custom"]`,
		`"providers_after":["kwu"]`,
		`"high_priority":true`,
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("warning log missing %s in:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "SECRET_ENV") || strings.Contains(logs, "DEEPSEEK_API_KEY") || strings.Contains(logs, "KWU_API_KEY") || strings.Contains(logs, "KWU_API_KEY") {
		t.Fatalf("warning log leaked credential names:\n%s", logs)
	}
}

func TestLoadForEditMigratesLegacyMCPTiers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kwu.toml")
	body := `
[[plugins]]
name = "playwright"
command = "npx"
tier = "lazy"

[[providers]]
name = "local"
kind = "openai"
base_url = "https://x"
model = "m"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadForEdit(path)
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].Tier != "" {
		t.Fatalf("plugins after migration = %+v, want empty tier", cfg.Plugins)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "\ntier") {
		t.Fatalf("legacy tier lines should be removed from file:\n%s", updated)
	}
	if !strings.Contains(string(updated), `command = "npx"`) || !strings.Contains(string(updated), `name = "local"`) {
		t.Fatalf("migration should preserve ordinary config:\n%s", updated)
	}
}

func TestLoadForEditIgnoresProjectDotEnvForProviderCredentials(t *testing.T) {
	project := t.TempDir()
	launch := t.TempDir()
	home := t.TempDir()
	path := filepath.Join(project, "kwu.toml")
	body := `default_model = "custom/m"
[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.invalid/v1"
model = "m"
api_key_env = "PROJECT_ONLY_KEY"
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte("PROJECT_ONLY_KEY=from-project\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(launch)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("PROJECT_ONLY_KEY", "")
	os.Unsetenv("PROJECT_ONLY_KEY")

	cfg := LoadForEdit(path)
	provider, ok := cfg.Provider("custom")
	if !ok {
		t.Fatalf("provider missing from edited config: %+v", cfg.Providers)
	}
	if provider.Configured() {
		t.Fatalf("provider should not resolve api_key_env from project .env next to edited config")
	}
	if got := ResolveCredentialForRootGlobalFirst(project, "PROJECT_ONLY_KEY"); got.Set {
		t.Fatalf("credential = %+v, want project .env ignored for provider key", got)
	}
}
