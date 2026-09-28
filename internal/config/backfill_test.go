package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"

	"kwu/internal/provider"
)

func hasModel(c *Config, model string) *ProviderEntry {
	for i := range c.Providers {
		for _, m := range c.Providers[i].ModelList() {
			if m == model {
				return &c.Providers[i]
			}
		}
	}
	return nil
}

func TestBackfillDeepSeekProRestoresPro(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	backfillDeepSeekPro(c)
	pro := hasModel(c, "deepseek-v4-pro")
	if pro == nil {
		t.Fatal("deepseek-v4-pro not restored")
	} else if pro.Price == nil || pro.Price.Output != 6 || pro.Price.Currency != "¥" {
		t.Errorf("pro price not the preset: %+v", pro.Price)
	}
}

func TestBackfillDeepSeekProUsesConfiguredLanguage(t *testing.T) {
	c := &Config{Language: "zh", Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	backfillDeepSeekPro(c)
	pro := hasModel(c, "deepseek-v4-pro")
	if pro == nil {
		t.Fatal("deepseek-v4-pro not restored")
	} else if pro.Price == nil || pro.Price.Output != 6 || pro.Price.Currency != "¥" {
		t.Errorf("pro price = %+v, want CNY preset", pro.Price)
	}
}

func TestBackfillDeepSeekProInheritsKeyEnv(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "MY_DS_KEY"},
	}}
	backfillDeepSeekPro(c)
	if pro := hasModel(c, "deepseek-v4-pro"); pro == nil || pro.APIKeyEnv != "MY_DS_KEY" {
		t.Errorf("pro should inherit the flash key env, got %+v", pro)
	}
}

func TestBackfillDeepSeekProNoopWhenProPresent(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek-flash", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash"},
		{Name: "deepseek-pro", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro"},
	}}
	backfillDeepSeekPro(c)
	if n := len(c.Providers); n != 2 {
		t.Errorf("providers grew to %d; should be a no-op when pro is present", n)
	}
}

func TestBackfillDeepSeekProSkipsCustomEndpoint(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "myproxy", BaseURL: "https://proxy.example.com/v1", Model: "deepseek-v4-flash"},
	}}
	backfillDeepSeekPro(c)
	if hasModel(c, "deepseek-v4-pro") != nil {
		t.Error("must not add pro for a non-official endpoint that may not serve it")
	}
}

func TestBackfillDeepSeekProSkipsNonDeepSeek(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "mimo-flash", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5"},
	}}
	backfillDeepSeekPro(c)
	if len(c.Providers) != 1 {
		t.Error("unrelated config must be untouched")
	}
}

func TestNormalizeLegacyProviderModelsRepairsOfficialProvider(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:      "deepseek-flash",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}}
	normalizeLegacyProviderModels(c)
	if got := c.Providers[0].Model; got != "deepseek-v4-flash" {
		t.Fatalf("deepseek-flash model = %q, want deepseek-v4-flash", got)
	}
}

func TestNormalizeLegacyProviderModelsLeavesCustomProviderUntouched(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:    "custom",
		Kind:    "openai",
		BaseURL: "https://proxy.example.com/v1",
	}}}
	normalizeLegacyProviderModels(c)
	if got := c.Providers[0].Model; got != "" {
		t.Fatalf("custom provider model = %q, want empty", got)
	}
}

func TestDefaultConfigResolvesExplicitLegacyDeepSeekProviders(t *testing.T) {
	c := Default()
	c.Providers = append(c.Providers,
		ProviderEntry{
			Name:      "deepseek-flash",
			Kind:      "openai",
			BaseURL:   "https://api.deepseek.com",
			Model:     "deepseek-v4-flash",
			APIKeyEnv: "DEEPSEEK_API_KEY",
		},
		ProviderEntry{
			Name:      "deepseek-pro",
			Kind:      "openai",
			BaseURL:   "https://api.deepseek.com",
			Model:     "deepseek-v4-pro",
			APIKeyEnv: "DEEPSEEK_API_KEY",
		},
		ProviderEntry{
			Name:      "deepseek",
			Kind:      "openai",
			BaseURL:   "https://api.deepseek.com",
			Models:    []string{"deepseek-v4-flash", "deepseek-v4-pro"},
			Default:   "deepseek-v4-flash",
			APIKeyEnv: "DEEPSEEK_API_KEY",
		},
	)

	tests := []struct {
		ref       string
		wantName  string
		wantModel string
	}{
		{ref: "deepseek-flash", wantName: "deepseek-flash", wantModel: "deepseek-v4-flash"},
		{ref: "deepseek-pro", wantName: "deepseek-pro", wantModel: "deepseek-v4-pro"},
		{ref: "deepseek/deepseek-v4-pro", wantName: "deepseek", wantModel: "deepseek-v4-pro"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got, ok := c.ResolveModel(tt.ref)
			if !ok {
				t.Fatalf("legacy ref %q should resolve when explicitly configured", tt.ref)
			}
			if got.Name != tt.wantName || got.Model != tt.wantModel {
				t.Fatalf("ResolveModel(%q) = %s/%s, want %s/%s", tt.ref, got.Name, got.Model, tt.wantName, tt.wantModel)
			}
		})
	}
}

func TestNormalizeDesktopOfficialProviderAccessMigratesOfficialDeepSeekToKWU(t *testing.T) {
	c := Default()
	c.DefaultModel = "deepseek-flash/deepseek-v4-pro"
	c.Desktop.ProviderAccess = []string{"deepseek-flash", "mimo-pro"}
	normalizeDesktopOfficialProviderAccess(c)
	if len(c.Desktop.ProviderAccess) != 2 || c.Desktop.ProviderAccess[0] != "kwu" || c.Desktop.ProviderAccess[1] != "mimo-pro" {
		t.Fatalf("provider_access = %+v, want official DeepSeek migrated to KWU", c.Desktop.ProviderAccess)
	}
	if c.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("default_model = %q, want kwu/deepseek-v4-flash", c.DefaultModel)
	}
	if _, ok := c.Provider("kwu"); !ok {
		t.Fatal("kwu provider missing")
	}
	if _, ok := c.Provider("deepseek"); ok {
		t.Fatal("official deepseek provider should be removed")
	}
	if _, ok := c.Provider("mimo-token-plan"); ok {
		t.Fatal("mimo-token-plan should not be injected as an official provider")
	}
}

func TestNormalizeDesktopOfficialProviderAccessBackfillsOfficialContextWindow(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"deepseek-flash", "mimo-api", "mimo-token-plan"}},
		Providers: []ProviderEntry{
			{
				Name:      "deepseek-flash",
				Kind:      "openai",
				BaseURL:   "https://api.deepseek.com",
				Model:     "deepseek-v4-flash",
				APIKeyEnv: "DEEPSEEK_API_KEY",
			},
			{
				Name:      "mimo-api",
				Kind:      "openai",
				BaseURL:   "https://api.xiaomimimo.com/v1",
				Model:     "mimo-v2.5-pro",
				APIKeyEnv: "MIMO_API_KEY",
			},
			{
				Name:      "mimo-token-plan",
				Kind:      "openai",
				BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
				Model:     "mimo-v2.5-pro",
				APIKeyEnv: "MIMO_API_KEY",
			},
		},
	}

	normalizeDesktopOfficialProviderAccess(c)

	kwu, ok := c.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing")
	}
	if kwu.ContextWindow != 1_000_000 {
		t.Fatalf("kwu context_window = %d, want official default", kwu.ContextWindow)
	}
	if _, ok := c.Provider("deepseek-flash"); ok {
		t.Fatal("official deepseek-flash provider should be removed")
	}
	mimoAPI, ok := c.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing")
	}
	if mimoAPI.ContextWindow != 1_048_576 {
		t.Fatalf("mimo-api context_window = %d, want migrated MiMo default", mimoAPI.ContextWindow)
	}
	mimoTokenPlan, ok := c.Provider("mimo-token-plan")
	if !ok {
		t.Fatal("mimo-token-plan provider missing")
	}
	if mimoTokenPlan.ContextWindow != 1_048_576 {
		t.Fatalf("mimo-token-plan context_window = %d, want migrated MiMo default", mimoTokenPlan.ContextWindow)
	}
}

func TestNormalizeDesktopOfficialProviderAccessEnsuresKWUOfficialProvider(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"kwu"}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	p, ok := c.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing")
	}
	if len(c.Desktop.ProviderAccess) != 1 || c.Desktop.ProviderAccess[0] != "kwu" {
		t.Fatalf("provider_access = %+v, want kwu", c.Desktop.ProviderAccess)
	}
	if p.Kind != "openai" {
		t.Fatalf("kind = %q, want openai", p.Kind)
	}
	if p.BaseURL != OfficialProviderBaseURL {
		t.Fatalf("base_url = %q", p.BaseURL)
	}
	if p.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("api_key_env = %q", p.APIKeyEnv)
	}
	if p.BalanceURL != "" {
		t.Fatalf("balance_url = %q, want empty", p.BalanceURL)
	}
	if got := p.ModelList(); !reflect.DeepEqual(got, []string{"deepseek-v4-flash", "gpt-5.4", "gpt-5.4-mini"}) {
		t.Fatalf("models = %#v, want KWU official fallbacks", got)
	}
	if p.Default != "deepseek-v4-flash" {
		t.Fatalf("default = %q, want deepseek-v4-flash", p.Default)
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("context_window = %d, want official default", p.ContextWindow)
	}
	if p.ReasoningProtocol != "openai" {
		t.Fatalf("reasoning_protocol = %q, want openai", p.ReasoningProtocol)
	}
	if !reflect.DeepEqual(p.SupportedEfforts, []string{"low", "medium", "high"}) {
		t.Fatalf("supported_efforts = %#v, want low/medium/high", p.SupportedEfforts)
	}
}

func TestNormalizeDesktopOfficialProviderAccessBackfillsKWUContextWindow(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"kwu"}},
		Providers: []ProviderEntry{{
			Name:      "kwu",
			Kind:      "openai",
			BaseURL:   "http://192.168.10.231:8080/v1",
			Models:    []string{"deepseek-v4-pro", "deepseek-v4-flash"},
			Default:   "deepseek-v4-pro",
			APIKeyEnv: "KWU_API_KEY",
		}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	p, ok := c.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing")
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("context_window = %d, want official default", p.ContextWindow)
	}
}

func TestNormalizeKWUOfficialEndpointMigratesOnlyRetiredBuiltinURL(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{
		{Name: "kwu", BaseURL: legacyOfficialBaseURL + "/", ModelsURL: legacyOfficialModelsURL + "/"},
		{Name: "kwu", BaseURL: OfficialProviderBaseURL, ModelsURL: legacyOfficialModelsURL},
		{Name: "kwu", BaseURL: "https://custom.example/v1", ModelsURL: "https://custom.example/catalog"},
		{Name: "custom", BaseURL: legacyOfficialBaseURL, ModelsURL: legacyOfficialModelsURL},
	}}

	if !normalizeKWUOfficialEndpoint(c) {
		t.Fatal("normalizeKWUOfficialEndpoint should report migration")
	}
	if got := c.Providers[0]; got.BaseURL != OfficialProviderBaseURL || got.ModelsURL != OfficialProviderModelsURL {
		t.Fatalf("migrated provider = %+v", got)
	}
	if got := c.Providers[1]; got.BaseURL != OfficialProviderBaseURL || got.ModelsURL != OfficialProviderModelsURL {
		t.Fatalf("partially migrated provider = %+v", got)
	}
	if got := c.Providers[2]; got.BaseURL != "https://custom.example/v1" || got.ModelsURL != "https://custom.example/catalog" {
		t.Fatalf("custom KWU endpoint changed: %+v", got)
	}
	if got := c.Providers[3]; got.BaseURL != legacyOfficialBaseURL || got.ModelsURL != legacyOfficialModelsURL {
		t.Fatalf("custom provider changed: %+v", got)
	}
}

func TestNormalizeKWUOfficialModelsMigratesOnlyLegacyBuiltinCatalog(t *testing.T) {
	c := &Config{
		DefaultModel: "kwu/gpt-5.5",
		Providers: []ProviderEntry{
			{Name: "kwu", Kind: "openai", BaseURL: OfficialProviderBaseURL, Models: []string{"gpt-5.5", "gpt-5.4"}, Default: "gpt-5.5"},
			{Name: "custom", Kind: "openai", BaseURL: "https://custom.example/v1", Models: []string{"gpt-5.5", "private-model"}, Default: "gpt-5.5"},
		},
	}
	if !normalizeKWUOfficialModels(c) {
		t.Fatal("normalizeKWUOfficialModels should report legacy built-in migration")
	}
	kwu, _ := c.Provider("kwu")
	if got := kwu.ModelList(); !reflect.DeepEqual(got, []string{"deepseek-v4-flash", "gpt-5.4", "gpt-5.4-mini"}) || kwu.Default != "deepseek-v4-flash" {
		t.Fatalf("kwu catalog = %v default=%q", got, kwu.Default)
	}
	if c.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("default_model = %q, want migrated built-in default", c.DefaultModel)
	}
	custom, _ := c.Provider("custom")
	if got := custom.ModelList(); !reflect.DeepEqual(got, []string{"gpt-5.5", "private-model"}) || custom.Default != "gpt-5.5" {
		t.Fatalf("custom catalog changed: %v default=%q", got, custom.Default)
	}
}

func TestNormalizeDesktopOfficialProviderAccessKeepsCurrentKWUDefaultWhenFetchedModelsExist(t *testing.T) {
	c := &Config{
		DefaultModel: "kwu/deepseek-v4-pro",
		Agent: AgentConfig{
			PlannerModel: "kwu/deepseek-v4-flash",
		},
		Desktop: DesktopConfig{ProviderAccess: []string{"kwu"}},
		Providers: []ProviderEntry{{
			Name:              "kwu",
			Kind:              "openai",
			BaseURL:           "http://192.168.10.231:8080/v1",
			Models:            []string{"deepseek-v4-pro", "deepseek-v4-flash", "gpt-5.5", "gpt-image-2"},
			Default:           "deepseek-v4-pro",
			APIKeyEnv:         "KWU_API_KEY",
			ReasoningProtocol: "openai",
		}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	p, ok := c.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing")
	}
	if got, want := p.ModelList(), []string{"deepseek-v4-flash", "gpt-5.5", "gpt-image-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %#v, want %#v", got, want)
	}
	if p.Default != "deepseek-v4-flash" {
		t.Fatalf("default = %q, want current default model", p.Default)
	}
	if c.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("default_model = %q, want current default model", c.DefaultModel)
	}
	if c.Agent.PlannerModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("planner_model = %q, want current default model", c.Agent.PlannerModel)
	}
}

func TestNormalizeConfigForEditKeepsSubscriptionPoolDynamicModelRefs(t *testing.T) {
	c := &Config{
		DefaultModel: "kwu/deepseek-v4-flash",
		Agent: AgentConfig{
			PlannerModel:       "kwu/deepseek-v4-flash",
			SubagentModel:      "kwu/deepseek-v4-pro",
			AutoPlanClassifier: "kwu/deepseek-v4-flash",
			SubagentModels: map[string]string{
				"research": "kwu/deepseek-v4-pro",
			},
		},
		Desktop: DesktopConfig{
			DefaultBillingSource: "subscription_pool",
			ProviderAccess:       []string{"kwu"},
		},
		Providers: []ProviderEntry{{
			Name:    "kwu",
			Kind:    "openai",
			BaseURL: OfficialProviderBaseURL,
			Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro", "gpt-5.4"},
			Default: "gpt-5.4",
		}},
	}

	normalizeConfigForEdit(c)

	if c.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("default_model = %q, want dynamic subscription route preserved", c.DefaultModel)
	}
	if c.Agent.PlannerModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("planner_model = %q, want dynamic subscription route preserved", c.Agent.PlannerModel)
	}
	if c.Agent.SubagentModel != "kwu/deepseek-v4-pro" {
		t.Fatalf("subagent_model = %q, want dynamic subscription route preserved", c.Agent.SubagentModel)
	}
	if c.Agent.AutoPlanClassifier != "kwu/deepseek-v4-flash" {
		t.Fatalf("auto_plan_classifier = %q, want dynamic subscription route preserved", c.Agent.AutoPlanClassifier)
	}
	if c.Agent.SubagentModels["research"] != "kwu/deepseek-v4-pro" {
		t.Fatalf("subagent_models[research] = %q, want dynamic subscription route preserved", c.Agent.SubagentModels["research"])
	}
}

func TestNormalizeConfigForEditRetargetsSubscriptionDerivedRefsAfterProjectDefaultOverride(t *testing.T) {
	c := &Config{
		DefaultModel: "fixture/custom-model",
		Agent: AgentConfig{
			PlannerModel:  "kwu/deepseek-v4-flash",
			SubagentModel: "kwu/deepseek-v4-pro",
		},
		Desktop: DesktopConfig{
			DefaultBillingSource: "subscription_pool",
		},
		Providers: []ProviderEntry{
			kwuOfficialProviderEntry(),
			{Name: "fixture", Kind: "openai", Model: "custom-model"},
		},
	}

	normalizeConfigForEdit(c)

	if c.Agent.PlannerModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("planner_model = %q, want stale subscription ref retargeted", c.Agent.PlannerModel)
	}
	if c.Agent.SubagentModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("subagent_model = %q, want stale subscription ref retargeted", c.Agent.SubagentModel)
	}
}

func TestNormalizeConfigForEditRetargetsDeprecatedBYOKDefault(t *testing.T) {
	for _, billingSource := range []string{"", "newapi_byok"} {
		t.Run(billingSource, func(t *testing.T) {
			c := &Config{
				DefaultModel: "kwu/deepseek-v4-flash",
				Desktop: DesktopConfig{
					DefaultBillingSource: billingSource,
					ProviderAccess:       []string{"kwu"},
				},
				Providers: []ProviderEntry{{
					Name:    "kwu",
					Kind:    "openai",
					BaseURL: OfficialProviderBaseURL,
					Models:  []string{"deepseek-v4-flash", "gpt-5.4"},
					Default: "gpt-5.4",
				}},
			}

			normalizeConfigForEdit(c)

			if c.DefaultModel != "kwu/deepseek-v4-flash" {
				t.Fatalf("default_model = %q, want current default ref preserved", c.DefaultModel)
			}
		})
	}
}

func TestNormalizeDesktopOfficialProviderAccessKeepsCustomAlias(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"deepseek-flash"}},
		Providers: []ProviderEntry{{
			Name:    "deepseek-flash",
			Kind:    "openai",
			BaseURL: "https://proxy.example/v1",
			Model:   "deepseek-v4-flash",
		}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	if len(c.Desktop.ProviderAccess) != 1 || c.Desktop.ProviderAccess[0] != "deepseek-flash" {
		t.Fatalf("provider_access = %+v, want custom alias preserved", c.Desktop.ProviderAccess)
	}
	if _, ok := c.Provider("deepseek"); ok {
		t.Fatal("custom deepseek-flash proxy should not create canonical deepseek provider")
	}
}

func TestNormalizeOfficialDeepSeekModelsMigratesCanonicalProviderToKWU(t *testing.T) {
	c := &Config{
		DefaultModel: "deepseek-flash/deepseek-v4-flash",
		Desktop:      DesktopConfig{ProviderAccess: []string{"deepseek"}},
		Providers: []ProviderEntry{{
			Name:      "deepseek",
			Kind:      "openai",
			BaseURL:   "https://api.deepseek.com",
			Model:     "glm-5",
			APIKeyEnv: "DEEPSEEK_API_KEY",
		}},
	}
	normalizeDesktopOfficialProviderAccess(c)
	normalizeOfficialDeepSeekModels(c)

	p, ok := c.Provider("kwu")
	if !ok {
		t.Fatal("kwu provider missing")
	}
	if !p.HasModel("gpt-5.4") || !p.HasModel("gpt-5.4-mini") {
		t.Fatalf("kwu models = %+v, want official fallback models", p.ModelList())
	}
	if _, ok := c.Provider("deepseek"); ok {
		t.Fatal("official deepseek provider should be removed")
	}
	if c.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("default_model = %q, want retargeted official ref", c.DefaultModel)
	}
	if _, ok := c.ResolveModel(c.DefaultModel); !ok {
		t.Fatalf("retargeted default_model %q should resolve", c.DefaultModel)
	}
}

func TestNormalizeOfficialDeepSeekModelsLeavesExternalEndpointUntouched(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:    "deepseek",
		Kind:    "openai",
		BaseURL: "https://proxy.example.com/v1",
		Model:   "glm-5",
	}}}
	normalizeOfficialDeepSeekModels(c)

	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if p.HasModel("deepseek-v4-flash") || p.HasModel("deepseek-v4-pro") {
		t.Fatalf("external endpoint models = %+v, want untouched custom list", p.ModelList())
	}
}

func TestNormalizeLegacyMimoCustomProvidersEnsuresReferencedMimoAPI(t *testing.T) {
	c := Default()
	c.DefaultModel = "mimo-api/mimo-v2.5-pro"
	c.Desktop.ProviderAccess = []string{"mimo-api"}
	if !normalizeLegacyMimoCustomProviders(c) {
		t.Fatal("legacy MiMo migration did not report a change")
	}
	p, ok := c.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api paid provider missing")
	}
	if !p.HasModel("mimo-v2.5") || !p.HasModel("mimo-v2-omni") {
		t.Fatalf("mimo-api models = %v, want vision-capable MiMo models", p.ModelList())
	}
	normalizeDesktopOfficialProviderAccess(c)
	if got := c.Desktop.ProviderAccess; len(got) != 1 || got[0] != "mimo-api" {
		t.Fatalf("provider_access = %+v, want mimo-api", got)
	}
}

func TestNormalizeLegacyMimoCustomProvidersRecognizesBareModelRefs(t *testing.T) {
	c := Default()
	c.DefaultModel = "mimo-v2.5-pro"
	if !normalizeLegacyMimoCustomProviders(c) {
		t.Fatal("legacy bare MiMo model migration did not report a change")
	}
	p, ok := c.Provider("mimo-pro")
	if !ok {
		t.Fatal("mimo-pro provider missing")
	}
	if p.Model != "mimo-v2.5-pro" {
		t.Fatalf("mimo-pro model = %q, want mimo-v2.5-pro", p.Model)
	}
	if e, ok := c.ResolveModel("mimo-v2.5-pro"); !ok || e.Name != "mimo-pro" {
		t.Fatalf("bare MiMo model resolved to %+v/%v, want mimo-pro", e, ok)
	}
}

func TestNormalizeLegacyMimoCustomProvidersScansBotRefs(t *testing.T) {
	c := Default()
	c.Bot.Model = "mimo-pro"
	c.Bot.Connections = []BotConnectionConfig{{Model: "mimo-flash"}}
	if !normalizeLegacyMimoCustomProviders(c) {
		t.Fatal("legacy bot MiMo migration did not report a change")
	}
	if _, ok := c.Provider("mimo-pro"); !ok {
		t.Fatal("mimo-pro provider missing")
	}
	if _, ok := c.Provider("mimo-flash"); !ok {
		t.Fatal("mimo-flash provider missing")
	}
}

func TestNormalizeLegacyDesktopProviderAccessIncludesUnconfiguredMimoRefs(t *testing.T) {
	c := Default()
	c.DefaultModel = "mimo-pro"
	c.Bot.Connections = []BotConnectionConfig{{Model: "mimo-flash"}}
	normalizeLegacyMimoCustomProviders(c)
	NormalizeLegacyDesktopProviderAccess(c)
	access := desktopProviderAccessMap(c.Desktop.ProviderAccess)
	if !access["mimo-pro"] || !access["mimo-flash"] {
		t.Fatalf("provider_access = %+v, want unconfigured migrated MiMo refs visible", c.Desktop.ProviderAccess)
	}
}

func TestBackfillDeepSeekOfficialPrices(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:    "deepseek",
		Kind:    "openai",
		BaseURL: "https://api.deepseek.com",
		Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		Default: "deepseek-v4-flash",
	}}}
	backfillDeepSeekOfficialPrices(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if p.Prices["deepseek-v4-flash"].Output != 2 || p.Prices["deepseek-v4-pro"].Output != 6 {
		t.Fatalf("deepseek prices = %+v, want current V4 flash/pro prices", p.Prices)
	}
}

func TestBackfillDeepSeekOfficialPricesUsesConfiguredLanguage(t *testing.T) {
	c := &Config{Language: "zh", Providers: []ProviderEntry{{
		Name:    "deepseek",
		Kind:    "openai",
		BaseURL: "https://api.deepseek.com",
		Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		Default: "deepseek-v4-flash",
	}}}
	backfillDeepSeekOfficialPrices(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if p.Prices["deepseek-v4-flash"].Output != 2 || p.Prices["deepseek-v4-flash"].Currency != "¥" || p.Prices["deepseek-v4-pro"].Output != 6 || p.Prices["deepseek-v4-pro"].Currency != "¥" {
		t.Fatalf("deepseek prices = %+v, want CNY flash/pro prices", p.Prices)
	}
}

func TestBackfillDeepSeekOfficialPricesKeepsProviderWidePrice(t *testing.T) {
	custom := &provider.Pricing{CacheHit: 9, Input: 9, Output: 9, Currency: "$"}
	c := &Config{Providers: []ProviderEntry{{
		Name:    "deepseek",
		Kind:    "openai",
		BaseURL: "https://api.deepseek.com",
		Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		Default: "deepseek-v4-flash",
		Price:   custom,
		Prices: map[string]*provider.Pricing{
			"deepseek-v4-flash": {CacheHit: 1, Input: 1, Output: 1, Currency: "$"},
		},
	}}}
	backfillDeepSeekOfficialPrices(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if len(p.Prices) != 1 {
		t.Fatalf("deepseek prices = %+v, want existing per-model prices only", p.Prices)
	}
	pro, ok := c.ResolveModel("deepseek/deepseek-v4-pro")
	if !ok {
		t.Fatal("deepseek pro did not resolve")
	}
	if pro.Price == nil || pro.Price.Output != 9 {
		t.Fatalf("pro price = %+v, want provider-wide custom price", pro.Price)
	}
	flash, ok := c.ResolveModel("deepseek")
	if !ok {
		t.Fatal("deepseek default did not resolve")
	}
	if flash.Price == nil || flash.Price.Output != 1 {
		t.Fatalf("flash price = %+v, want existing per-model custom price", flash.Price)
	}
}

func TestApplyDeepSeekOfficialDefaultPricingUsesConfiguredLanguage(t *testing.T) {
	c := configWithLegacyDeepSeekProviders()
	c.Language = "zh"
	applyDeepSeekOfficialDefaultPricing(c)
	flash, ok := c.Provider("deepseek-flash")
	if !ok {
		t.Fatal("deepseek-flash provider missing")
	}
	if flash.Price == nil || flash.Price.Output != 2 || flash.Price.Currency != "¥" {
		t.Fatalf("flash price = %+v, want CNY preset", flash.Price)
	}
	pro, ok := c.Provider("deepseek-pro")
	if !ok {
		t.Fatal("deepseek-pro provider missing")
	}
	if pro.Price == nil || pro.Price.Output != 6 || pro.Price.Currency != "¥" {
		t.Fatalf("pro price = %+v, want CNY preset", pro.Price)
	}
}

func TestApplyDeepSeekOfficialDefaultPricingKeepsCustomPrice(t *testing.T) {
	c := &Config{Language: "zh", Providers: []ProviderEntry{{
		Name:    "deepseek-flash",
		Kind:    "openai",
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-v4-flash",
		Price:   &provider.Pricing{CacheHit: 9, Input: 9, Output: 9, Currency: "$"},
	}}}
	applyDeepSeekOfficialDefaultPricing(c)
	p, ok := c.Provider("deepseek-flash")
	if !ok {
		t.Fatal("deepseek-flash provider missing")
	}
	if p.Price == nil || p.Price.Output != 9 || p.Price.Currency != "$" {
		t.Fatalf("custom price = %+v, want unchanged", p.Price)
	}
}

func TestMigrateConfigOnUpgradeRunsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kwu.toml")
	c := &Config{
		ConfigVersion: 2,
		Providers: []ProviderEntry{
			{
				Name:    "deepseek",
				Kind:    "openai",
				BaseURL: "https://api.deepseek.com",
				Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro"},
				Default: "deepseek-v4-flash",
				Price:   &provider.Pricing{CacheHit: 9, Input: 9, Output: 9, Currency: "$"},
				Prices: map[string]*provider.Pricing{
					"deepseek-v4-flash": {CacheHit: 8, Input: 8, Output: 8, Currency: "$"},
				},
			},
			{
				Name:    "mimo-api",
				Kind:    "openai",
				BaseURL: "https://api.xiaomimimo.com/v1",
				Models:  []string{"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-omni"},
				Default: "mimo-v2.5-pro",
				Price:   &provider.Pricing{CacheHit: 7, Input: 7, Output: 7, Currency: "$"},
			},
		},
	}
	if err := c.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	changed, err := MigrateConfigOnUpgrade(path)
	if err != nil {
		t.Fatalf("MigrateConfigOnUpgrade: %v", err)
	}
	if !changed {
		t.Fatal("upgrade reset did not run for config_version 2")
	}
	var got Config
	if _, err := toml.DecodeFile(path, &got); err != nil {
		t.Fatalf("decode migrated config: %v", err)
	}
	if got.ConfigVersion != Default().ConfigVersion {
		t.Fatalf("config_version = %d, want %d", got.ConfigVersion, Default().ConfigVersion)
	}
	if _, ok := got.Provider("deepseek"); ok {
		t.Fatal("official deepseek provider should be removed")
	}
	if _, ok := got.Provider("kwu"); ok {
		t.Fatal("built-in kwu provider should remain runtime-only")
	}
	mimo, ok := got.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing")
	}
	if mimo.Price == nil || mimo.Price.Currency != "$" || mimo.Price.Output != 7 {
		t.Fatalf("mimo provider-wide price = %+v, want custom price preserved", mimo.Price)
	}

	changed, err = MigrateConfigOnUpgrade(path)
	if err != nil {
		t.Fatalf("second MigrateConfigOnUpgrade: %v", err)
	}
	if changed {
		t.Fatal("upgrade reset ran again after config_version was updated")
	}
	got = Config{}
	if _, err := toml.DecodeFile(path, &got); err != nil {
		t.Fatalf("decode custom config: %v", err)
	}
	if _, ok := got.Provider("deepseek"); ok {
		t.Fatal("official deepseek provider should remain removed after second run")
	}
}

func TestMigrateConfigOnUpgradeDoesNotHydrateProviderCredential(t *testing.T) {
	_, _, _ = isolateProfilePathEnv(t)
	t.Setenv("KWU_CREDENTIALS_STORE", CredentialsStoreFile)
	const key = "B2B_PRICING_UPGRADE_KEY"
	t.Setenv(key, "profile-key")
	path := UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{ConfigVersion: 1, Providers: []ProviderEntry{{
		Name: "fixture", Kind: "openai", BaseURL: "https://fixture.invalid/v1", Model: "fixture-model", APIKeyEnv: key,
	}}}
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(UserCredentialsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserCredentialsPath(), []byte(key+"=profile-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}

	if changed, err := MigrateConfigOnUpgrade(path); err != nil || !changed {
		t.Fatalf("upgrade pricing changed/err = %v/%v", changed, err)
	}
	if got := os.Getenv(key); got != "" {
		t.Fatalf("pricing upgrade hydrated provider credential %q", got)
	}
}

func TestResolveModelUsesPerModelPricing(t *testing.T) {
	c := &Config{Providers: []ProviderEntry{{
		Name:    "deepseek",
		Kind:    "openai",
		BaseURL: "https://api.deepseek.com",
		Models:  []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		Default: "deepseek-v4-flash",
		Price:   &provider.Pricing{CacheHit: 9, Input: 9, Output: 9, Currency: "$"},
		Prices: map[string]*provider.Pricing{
			"deepseek-v4-flash": &provider.Pricing{CacheHit: 0.02, Input: 1, Output: 2, Currency: "¥"},
			"deepseek-v4-pro":   &provider.Pricing{CacheHit: 0.025, Input: 3, Output: 6, Currency: "¥"},
		},
	}}}
	pro, ok := c.ResolveModel("deepseek/deepseek-v4-pro")
	if !ok {
		t.Fatal("deepseek pro did not resolve")
	}
	if pro.Price == nil || pro.Price.Output != 6 {
		t.Fatalf("pro price = %+v, want model-specific pro price", pro.Price)
	}
	flash, ok := c.ResolveModel("deepseek")
	if !ok {
		t.Fatal("deepseek default did not resolve")
	}
	if flash.Price == nil || flash.Price.Output != 2 {
		t.Fatalf("flash price = %+v, want model-specific flash price", flash.Price)
	}
}

func TestNormalizeLegacyMimoProviderCatalogsBackfillsOfficialMimoAPIStub(t *testing.T) {
	c := &Config{
		DefaultModel: "mimo-api/mimo-v2.5-pro",
		Desktop:      DesktopConfig{ProviderAccess: []string{"mimo-api"}},
		Providers: []ProviderEntry{{
			Name:      "mimo-api",
			Kind:      "openai",
			BaseURL:   "https://api.xiaomimimo.com/v1",
			Model:     "mimo-v2.5-pro",
			APIKeyEnv: "MIMO_API_KEY",
		}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	p, ok := c.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing")
	}
	wantModels := []string{"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-omni"}
	if !reflect.DeepEqual(p.ModelList(), wantModels) {
		t.Fatalf("mimo-api models = %v, want %v", p.ModelList(), wantModels)
	}
	if p.Default != "mimo-v2.5-pro" {
		t.Fatalf("mimo-api default = %q, want mimo-v2.5-pro", p.Default)
	}
	if !reflect.DeepEqual(p.VisionModels, []string{"mimo-v2.5", "mimo-v2-omni"}) {
		t.Fatalf("mimo-api vision_models = %v, want vision-capable MiMo models", p.VisionModels)
	}
}

func TestNormalizeDesktopOfficialProviderAccessDoesNotBackfillCustomNamedMimoAPI(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"mimo-api"}},
		Providers: []ProviderEntry{{
			Name:    "mimo-api",
			Kind:    "openai",
			BaseURL: "https://proxy.example.com/v1",
			Model:   "mimo-v2.5-pro",
		}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	p, ok := c.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing")
	}
	if p.HasModel("mimo-v2.5") || p.HasModel("mimo-v2-omni") {
		t.Fatalf("custom mimo-api models = %v, want original custom list", p.ModelList())
	}
}

func TestNormalizeLegacyMimoProviderCatalogsBackfillsOfficialMimoTokenPlanStub(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"mimo-token-plan"}},
		Providers: []ProviderEntry{{
			Name:      "mimo-token-plan",
			Kind:      "openai",
			BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
			Model:     "mimo-v2.5-pro",
			APIKeyEnv: "MIMO_API_KEY",
			Price:     &provider.Pricing{CacheHit: 0.025, Input: 3, Output: 6, Currency: "CNY"},
		}},
	}

	normalizeDesktopOfficialProviderAccess(c)

	p, ok := c.Provider("mimo-token-plan")
	if !ok {
		t.Fatal("mimo-token-plan provider missing")
	}
	if !reflect.DeepEqual(p.ModelList(), []string{"mimo-v2.5-pro", "mimo-v2.5"}) {
		t.Fatalf("mimo-token-plan models = %v, want token plan catalog", p.ModelList())
	}
	if p.Price == nil || p.Price.Currency != "CNY" {
		t.Fatalf("mimo-token-plan price = %+v, want preserved custom provider-wide price", p.Price)
	}
	if !reflect.DeepEqual(p.VisionModels, []string{"mimo-v2.5"}) {
		t.Fatalf("mimo-token-plan vision_models = %v, want token plan vision metadata", p.VisionModels)
	}
}

// ── Explicit model list: normalization must not override user selection ───────

func TestBackfillDeepSeekProSkipsWhenExplicitModelList(t *testing.T) {
	// User saved with only flash via Settings → Models = ["deepseek-v4-flash"].
	c := &Config{Providers: []ProviderEntry{
		{Name: "deepseek", Kind: "openai", BaseURL: "https://api.deepseek.com", Models: []string{"deepseek-v4-flash"}, Default: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
	}}
	backfillDeepSeekPro(c)
	if hasModel(c, "deepseek-v4-pro") != nil {
		t.Fatal("deepseek-v4-pro must not be added when user has an explicit model list")
	}
	if len(c.Providers) != 1 {
		t.Fatalf("providers = %d, want 1 (no new entry should be added)", len(c.Providers))
	}
}

func TestEnsureProviderModelsSkipsWhenExplicitModelList(t *testing.T) {
	// User saved with only flash via Settings → Models = ["deepseek-v4-flash"].
	p := &ProviderEntry{
		Name:    "deepseek",
		BaseURL: "https://api.deepseek.com",
		Models:  []string{"deepseek-v4-flash"},
		Default: "deepseek-v4-flash",
	}
	ensureProviderModels(p, []string{"deepseek-v4-flash", "deepseek-v4-pro"}, "deepseek-v4-flash")
	if p.HasModel("deepseek-v4-pro") {
		t.Fatal("ensureProviderModels must not merge required models when Models is explicitly set")
	}
	if len(p.Models) != 1 || p.Models[0] != "deepseek-v4-flash" {
		t.Fatalf("models = %v, want [deepseek-v4-flash]", p.Models)
	}
}

func TestMergeCuratedModelsIntoProviderSkipsWhenExplicitModelList(t *testing.T) {
	// User saved with only two mimo models via Settings.
	p := &ProviderEntry{
		Name:    "mimo-api",
		BaseURL: "https://api.xiaomimimo.com/v1",
		Models:  []string{"mimo-v2.5-pro", "mimo-v2.5"},
		Default: "mimo-v2.5-pro",
	}
	mergeCuratedModelsIntoProvider(p, []string{"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-omni"}, "mimo-v2.5-pro")
	if p.HasModel("mimo-v2-omni") {
		t.Fatal("mergeCuratedModelsIntoProvider must not add mimo-v2-omni when Models is explicitly set")
	}
	if len(p.Models) != 2 {
		t.Fatalf("models = %v, want 2 selected models", p.Models)
	}
}

func TestNormalizeOfficialMimoVisionModelsSkipsExplicitModelList(t *testing.T) {
	// User saved with only pro via Settings → Models = ["mimo-v2.5-pro"].
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"mimo-api"}},
		Providers: []ProviderEntry{{
			Name:      "mimo-api",
			Kind:      "openai",
			BaseURL:   "https://api.xiaomimimo.com/v1",
			Models:    []string{"mimo-v2.5-pro"},
			Default:   "mimo-v2.5-pro",
			APIKeyEnv: "MIMO_API_KEY",
		}},
	}
	normalizeDesktopOfficialProviderAccess(c)
	p, ok := c.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing")
	}
	if p.HasModel("mimo-v2.5") || p.HasModel("mimo-v2-omni") {
		t.Fatalf("mimo-api models = %v, want only explicitly selected pro model", p.ModelList())
	}
	if len(p.VisionModels) != 0 {
		t.Fatalf("mimo-api vision_models = %v, want empty for pro-only explicit model list", p.VisionModels)
	}
}

func TestNormalizeOfficialMimoVisionModelsPreservesExplicitEmptyList(t *testing.T) {
	c := &Config{
		Desktop: DesktopConfig{ProviderAccess: []string{"mimo-api"}},
		Providers: []ProviderEntry{{
			Name:         "mimo-api",
			Kind:         "openai",
			BaseURL:      "https://api.xiaomimimo.com/v1",
			Models:       []string{"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-omni"},
			Default:      "mimo-v2.5-pro",
			APIKeyEnv:    "MIMO_API_KEY",
			VisionModels: []string{},
		}},
	}
	normalizeDesktopOfficialProviderAccess(c)
	p, ok := c.Provider("mimo-api")
	if !ok {
		t.Fatal("mimo-api provider missing")
	}
	if p.VisionModels == nil || len(p.VisionModels) != 0 {
		t.Fatalf("mimo-api vision_models = %#v, want explicit empty list", p.VisionModels)
	}
}

func TestNormalizeOfficialDeepSeekModelsSkipsExplicitModelList(t *testing.T) {
	// User saved with only flash via Settings → Models = ["deepseek-v4-flash"].
	c := &Config{Providers: []ProviderEntry{{
		Name:      "deepseek",
		Kind:      "openai",
		BaseURL:   "https://api.deepseek.com",
		Models:    []string{"deepseek-v4-flash"},
		Default:   "deepseek-v4-flash",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}}}
	normalizeOfficialDeepSeekModels(c)
	p, ok := c.Provider("deepseek")
	if !ok {
		t.Fatal("deepseek provider missing")
	}
	if p.HasModel("deepseek-v4-pro") {
		t.Fatal("normalizeOfficialDeepSeekModels must not add pro when Models is explicitly set")
	}
}
