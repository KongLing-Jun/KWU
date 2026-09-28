package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestRenderScopesOmitOfficialKWUProviderButKeepCustomSameName(t *testing.T) {
	cfg := Default()
	cfg.Providers = []ProviderEntry{
		{Name: "kwu", Kind: "openai", BaseURL: OfficialProviderBaseURL, Models: []string{"gpt-5.4"}},
		{Name: "kwu", Kind: "openai", BaseURL: "https://custom.example/v1", Model: "custom-model"},
		{Name: "kwu", Kind: "openai", BaseURL: legacyOfficialBaseURL + "/", Model: "legacy-model"},
		{Name: "other", Kind: "openai", BaseURL: "https://other.example/v1", Model: "other-model"},
	}

	for _, scope := range []RenderScope{RenderScopeFull, RenderScopeUser, RenderScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			rendered := RenderTOMLForScope(cfg, scope)
			if strings.Contains(rendered, OfficialProviderBaseURL) || strings.Contains(rendered, legacyOfficialBaseURL) {
				t.Fatalf("rendered %s TOML contains a built-in endpoint:\n%s", scope, rendered)
			}
			if !strings.Contains(rendered, "https://custom.example/v1") || !strings.Contains(rendered, "https://other.example/v1") {
				t.Fatalf("rendered %s TOML dropped a custom provider:\n%s", scope, rendered)
			}
			var got Config
			if _, err := toml.Decode(rendered, &got); err != nil {
				t.Fatalf("decode rendered %s TOML: %v", scope, err)
			}
			if len(got.Providers) != 2 || got.Providers[0].Name != "kwu" || got.Providers[1].Name != "other" {
				t.Fatalf("decoded %s providers = %+v, want custom kwu and other", scope, got.Providers)
			}
		})
	}

	delta := RenderTOMLProjectDelta(cfg)
	if strings.Contains(delta, OfficialProviderBaseURL) || strings.Contains(delta, legacyOfficialBaseURL) {
		t.Fatalf("project delta contains a built-in endpoint:\n%s", delta)
	}
	if !strings.Contains(delta, "https://custom.example/v1") || !strings.Contains(delta, "https://other.example/v1") {
		t.Fatalf("project delta dropped a custom provider:\n%s", delta)
	}
}

func TestLoadMaterializesOfficialProviderAfterProviderReplacement(t *testing.T) {
	isolateUserConfigHome(t)
	root := t.TempDir()
	userPath := UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const source = `default_model = "kwu/deepseek-v4-flash"

[[providers]]
name = "custom"
kind = "openai"
base_url = "https://custom.example/v1"
model = "custom-model"
`
	if err := os.WriteFile(userPath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadForRoot(root)
	if err != nil {
		t.Fatalf("LoadForRoot: %v", err)
	}
	if _, ok := loaded.ResolveModel("kwu/deepseek-v4-flash"); !ok {
		t.Fatalf("LoadForRoot did not materialize the runtime provider: %+v", loaded.Providers)
	}
	custom, ok := loaded.Provider("custom")
	if !ok || custom.BaseURL != "https://custom.example/v1" {
		t.Fatalf("custom provider changed after LoadForRoot: %+v", custom)
	}
	t.Run("Load", func(t *testing.T) {
		t.Chdir(root)
		loaded, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if _, ok := loaded.ResolveModel("kwu/deepseek-v4-flash"); !ok {
			t.Fatalf("Load did not materialize the runtime provider: %+v", loaded.Providers)
		}
		if custom, ok := loaded.Provider("custom"); !ok || custom.BaseURL != "https://custom.example/v1" {
			t.Fatalf("custom provider changed after Load: %+v", custom)
		}
	})
	if err := loaded.SaveTo(userPath); err != nil {
		t.Fatalf("SaveTo after LoadForRoot: %v", err)
	}
	body, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte(OfficialProviderBaseURL)) || bytes.Contains(body, []byte(legacyOfficialBaseURL)) {
		t.Fatalf("runtime provider was persisted:\n%s", body)
	}

	edit := LoadForEditWithoutCredentials(userPath)
	if _, ok := edit.ResolveModel("kwu/deepseek-v4-flash"); !ok {
		t.Fatalf("LoadForEditWithoutCredentials did not materialize the runtime provider: %+v", edit.Providers)
	}
	if custom, ok := edit.Provider("custom"); !ok || custom.BaseURL != "https://custom.example/v1" {
		t.Fatalf("custom provider changed after edit load: %+v", custom)
	}
}

func TestMigrateConfigOnUpgradeRemovesOfficialKWUPrecisely(t *testing.T) {
	cases := []struct {
		name             string
		providerURL      string
		defaultModel     string
		includeCustomKWU bool
		wantDefaultModel string
		wantCustomKWUURL string
	}{
		{
			name:             "current endpoint",
			providerURL:      OfficialProviderBaseURL,
			defaultModel:     "kwu/gpt-5.4",
			wantDefaultModel: "kwu/deepseek-v4-flash",
		},
		{
			name:             "retired endpoint with slash",
			providerURL:      legacyOfficialBaseURL + "/",
			defaultModel:     "kwu/gpt-5.4",
			wantDefaultModel: "kwu/deepseek-v4-flash",
		},
		{
			name:             "same named custom provider",
			providerURL:      OfficialProviderBaseURL,
			defaultModel:     "kwu/gpt-5.4",
			includeCustomKWU: true,
			wantDefaultModel: "kwu/gpt-5.4",
			wantCustomKWUURL: "https://custom.example/v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			source := "config_version = 3\ndefault_model = " + strconvQuote(tc.defaultModel) + "\n\n" +
				"[[providers]]\nname = \"kwu\"\nkind = \"openai\"\nbase_url = " + strconvQuote(tc.providerURL) + "\nmodels = [\"gpt-5.4\"]\ndefault = \"gpt-5.4\"\n\n" +
				"[[providers]]\nname = \"other\"\nkind = \"openai\"\nbase_url = \"https://other.example/v1\"\nmodel = \"other-model\"\n"
			if tc.includeCustomKWU {
				source += "\n[[providers]]\nname = \"kwu\"\nkind = \"openai\"\nbase_url = \"https://custom.example/v1\"\nmodel = \"custom-model\"\n"
			}
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			var header Config
			if _, err := toml.DecodeFile(path, &header); err != nil {
				t.Fatal(err)
			}
			if len(header.Providers) == 0 || !IsOfficialKWUProvider(&header.Providers[0]) {
				t.Fatalf("header providers = %+v, default = %q", header.Providers, header.DefaultModel)
			}
			changed, err := MigrateConfigOnUpgrade(path)
			if err != nil || !changed {
				t.Fatalf("MigrateConfigOnUpgrade = %v/%v, want changed", changed, err)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(body, []byte(OfficialProviderBaseURL)) || bytes.Contains(body, []byte(legacyOfficialBaseURL)) {
				t.Fatalf("migrated config contains built-in endpoint:\n%s", body)
			}
			var got Config
			if _, err := toml.Decode(string(body), &got); err != nil {
				t.Fatal(err)
			}
			if got.DefaultModel != tc.wantDefaultModel {
				t.Fatalf("default_model = %q, want %q; body:\n%s", got.DefaultModel, tc.wantDefaultModel, body)
			}
			if _, ok := got.Provider("other"); !ok {
				t.Fatal("other provider was not preserved")
			}
			if tc.wantCustomKWUURL != "" {
				custom, ok := got.Provider("kwu")
				if !ok || custom.BaseURL != tc.wantCustomKWUURL {
					t.Fatalf("custom kwu = %+v, want endpoint %q", custom, tc.wantCustomKWUURL)
				}
			} else if _, ok := got.Provider("kwu"); ok {
				t.Fatal("official kwu provider was not removed")
			}
		})
	}
}

func TestMigrateConfigOnUpgradeRejectsCorruptAndMissingFiles(t *testing.T) {
	corrupt := filepath.Join(t.TempDir(), "config.toml")
	want := []byte("config_version = 3\ndefault_model = [\n")
	if err := os.WriteFile(corrupt, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, err := MigrateConfigOnUpgrade(corrupt); err == nil || changed {
		t.Fatalf("corrupt migration = %v/%v, want error and unchanged", changed, err)
	}
	got, err := os.ReadFile(corrupt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("corrupt file changed: %q", got)
	}

	missing := filepath.Join(t.TempDir(), "missing.toml")
	if changed, err := MigrateConfigOnUpgrade(missing); err != nil || changed {
		t.Fatalf("missing migration = %v/%v, want no-op", changed, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing config was created: %v", err)
	}
}

func strconvQuote(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}
