package boot

import (
	"context"
	"errors"
	"os"
	"testing"

	"kwu/internal/config"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
)

func TestBuildUsesRuntimeSubscriptionModelCatalog(t *testing.T) {
	isolateConfigHome(t)
	root := robustTempDir(t)
	writeFile(t, root, "kwu.toml", `
default_model = "kwu/gpt-5.4"

[[providers]]
name = "kwu"
kind = "openai"
base_url = "https://custom.invalid/v1"
models = ["gpt-5.4"]
default = "gpt-5.4"
api_key_env = "KWU_RUNTIME_CATALOG_TEST_KEY"
`)
	configPath := root + string(os.PathSeparator) + "kwu.toml"
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	const modelRef = "kwu/claude-opus-4-7"
	ctrl, err := Build(context.Background(), Options{
		Model:         modelRef,
		WorkspaceRoot: root,
		Sink:          event.Discard,
		ModelBillingOptions: ModelBillingOptions{
			RequireTaskContext:     true,
			TaskContextInitializer: bootTaskContextInitializer{},
			BillingRouteSelection: modelbilling.BillingRouteSelection{
				ModelRef: modelRef, BillingSource: modelbilling.BillingSubscriptionPool,
			},
			SubscriptionModelCatalog: []string{" claude-opus-4-7 ", "gpt-5.4", "claude-opus-4-7"},
		},
	})
	if err != nil {
		t.Fatalf("Build dynamic subscription model: %v", err)
	}
	defer ctrl.Close()
	if got := ctrl.Label(); got != "claude-opus-4-7" {
		t.Fatalf("controller label = %q, want claude-opus-4-7", got)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("runtime subscription catalog modified the persisted provider config")
	}
}

func TestApplySubscriptionModelCatalogMaterializesOfficialProviderInMemory(t *testing.T) {
	cfg := &config.Config{
		DefaultModel: "custom/custom-model",
		Providers: []config.ProviderEntry{
			{
				Name: " kwu ", Kind: "openai", BaseURL: "https://custom.invalid/v1",
				Models: []string{"custom-model"}, Default: "custom-model", APIKeyEnv: "CUSTOM_KEY",
			},
			{
				Name: "custom", Kind: "openai", BaseURL: "https://other.invalid/v1",
				Model: "custom-model", APIKeyEnv: "OTHER_KEY",
			},
		},
	}

	if err := applySubscriptionModelCatalog(cfg, []string{"runtime-model"}); err != nil {
		t.Fatal(err)
	}
	official, ok := cfg.Provider("kwu")
	if !ok {
		t.Fatal("subscription config did not materialize the official provider")
	}
	if official.BaseURL != config.OfficialProviderBaseURL || official.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("subscription official provider = %+v", official)
	}
	if official.Default != "runtime-model" || len(official.Models) != 1 || official.Models[0] != "runtime-model" {
		t.Fatalf("subscription runtime catalog = %+v", official)
	}
	if custom, ok := cfg.Provider("custom"); !ok || custom.BaseURL != "https://other.invalid/v1" {
		t.Fatalf("custom provider changed while materializing subscription provider: %+v", custom)
	}
}

func TestBuildKeepsRuntimeSubscriptionCatalogRouteScoped(t *testing.T) {
	isolateConfigHome(t)
	root := robustTempDir(t)
	writeFile(t, root, "kwu.toml", `
default_model = "kwu/gpt-5.4"

[[providers]]
name = "kwu"
kind = "openai"
base_url = "https://custom.invalid/v1"
models = ["gpt-5.4"]
default = "gpt-5.4"
api_key_env = "KWU_RUNTIME_CATALOG_TEST_KEY"
`)

	const modelRef = "kwu/claude-opus-4-7"
	for name, tc := range map[string]struct {
		source  modelbilling.BillingSource
		catalog []string
	}{
		"subscription catalog miss": {source: modelbilling.BillingSubscriptionPool, catalog: []string{"gpt-5.4"}},
		"BYOK ignores catalog":      {source: modelbilling.BillingNewAPIByok, catalog: []string{"claude-opus-4-7"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Build(context.Background(), Options{
				Model:         modelRef,
				WorkspaceRoot: root,
				Sink:          event.Discard,
				ModelBillingOptions: ModelBillingOptions{
					RequireTaskContext:     true,
					TaskContextInitializer: bootTaskContextInitializer{},
					BillingRouteSelection: modelbilling.BillingRouteSelection{
						ModelRef: modelRef, BillingSource: tc.source,
					},
					SubscriptionModelCatalog: tc.catalog,
				},
			})
			if !errors.Is(err, ErrUnknownModel) {
				t.Fatalf("route-scoped dynamic model error = %v, want ErrUnknownModel", err)
			}
		})
	}
}
