package config

import (
	"reflect"
	"strings"
	"testing"
)

func testModelFallbackConfig(t *testing.T) *Config {
	t.Helper()

	c := Default()
	c.DefaultModel = "prov-a"
	c.Providers = []ProviderEntry{
		{Name: "prov-a", Kind: "openai", BaseURL: "https://a.example.com", Model: "model-a1", Models: []string{"model-a1", "model-a2"}, APIKeyEnv: "KWU_TEST_KEY", resolvedAPIKey: "sk-test"},
		{Name: "prov-b", Kind: "openai", BaseURL: "https://b.example.com", Model: "model-b1", Models: []string{"model-b1", "model-b2"}, APIKeyEnv: "KWU_TEST_KEY", resolvedAPIKey: "sk-test"},
		{Name: "prov-nokey", Kind: "openai", BaseURL: "https://nk.example.com", Model: "model-nk", APIKeyEnv: "KWU_TEST_EMPTY"},
	}
	return c
}

func TestDefaultConfigUsesKWUOfficialProvider(t *testing.T) {
	cfg := Default()
	if cfg.DefaultModel != "kwu/deepseek-v4-flash" {
		t.Fatalf("DefaultModel = %q, want kwu/deepseek-v4-flash", cfg.DefaultModel)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("providers = %d, want one built-in official provider", len(cfg.Providers))
	}
	p, ok := cfg.Provider("kwu")
	if !ok {
		t.Fatal("default provider kwu missing")
	}
	if p.Kind != "openai" {
		t.Fatalf("Kind = %q, want openai", p.Kind)
	}
	if p.BaseURL != OfficialProviderBaseURL {
		t.Fatalf("BaseURL = %q", p.BaseURL)
	}
	if p.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("APIKeyEnv = %q", p.APIKeyEnv)
	}
	if p.BalanceURL != "" {
		t.Fatalf("BalanceURL = %q, want empty until balance API exists", p.BalanceURL)
	}
	if p.Default != "deepseek-v4-flash" {
		t.Fatalf("Default = %q, want deepseek-v4-flash", p.Default)
	}
	if got := p.ModelList(); !reflect.DeepEqual(got, []string{"deepseek-v4-flash", "gpt-5.4", "gpt-5.4-mini"}) {
		t.Fatalf("ModelList = %#v", got)
	}
	if p.ContextWindow != 1_000_000 {
		t.Fatalf("ContextWindow = %d, want 1000000", p.ContextWindow)
	}
	if p.ReasoningProtocol != "openai" {
		t.Fatalf("ReasoningProtocol = %q, want openai", p.ReasoningProtocol)
	}
	if !reflect.DeepEqual(p.SupportedEfforts, []string{"low", "medium", "high"}) {
		t.Fatalf("SupportedEfforts = %#v, want low/medium/high", p.SupportedEfforts)
	}
	t.Setenv("KWU_TEST_EMPTY", "")
	t.Setenv("LOCAL_GATEWAY_TEST_EMPTY", "")
	official := ProviderEntry{Name: "kwu", Kind: "openai", BaseURL: OfficialProviderBaseURL, Models: []string{"gpt-5.5"}, APIKeyEnv: "KWU_TEST_EMPTY"}
	if !official.RequiresAPIKey() {
		t.Fatal("kwu official provider should require an API key")
	}
	if official.Configured() {
		t.Fatal("kwu official provider without a key should not be configured")
	}
	privateGateway := ProviderEntry{Name: "local-gateway", Kind: "openai", BaseURL: "http://192.168.10.232:8080/v1", Models: []string{"model-a"}, APIKeyEnv: "LOCAL_GATEWAY_TEST_EMPTY"}
	if privateGateway.RequiresAPIKey() {
		t.Fatal("ordinary private OpenAI-compatible gateway should still allow missing API key")
	}
	resolved, ok := cfg.ResolveModel("kwu/deepseek-v4-flash")
	if !ok {
		t.Fatal("ResolveModel(kwu/deepseek-v4-flash) failed")
	}
	if resolved.Name != "kwu" || resolved.Model != "deepseek-v4-flash" {
		t.Fatalf("resolved = %s/%s, want kwu/deepseek-v4-flash", resolved.Name, resolved.Model)
	}
	secondary, ok := cfg.ResolveModel("kwu/gpt-5.4-mini")
	if !ok {
		t.Fatal("ResolveModel(kwu/gpt-5.4-mini) failed")
	}
	if secondary.Name != "kwu" || secondary.Model != "gpt-5.4-mini" {
		t.Fatalf("secondary resolved = %s/%s, want kwu/gpt-5.4-mini", secondary.Name, secondary.Model)
	}
}

func TestResolveModelKeepsLegacyDeepSeekProviders(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderEntry{
			{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY"},
			{Name: "deepseek-pro", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", APIKeyEnv: "DEEPSEEK_API_KEY"},
		},
	}
	if got, ok := cfg.ResolveModel("deepseek-flash"); !ok || got.Name != "deepseek-flash" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("ResolveModel(deepseek-flash) = %+v, %v; want legacy flash", got, ok)
	}
	if got, ok := cfg.ResolveModel("deepseek-pro"); !ok || got.Name != "deepseek-pro" || got.Model != "deepseek-v4-pro" {
		t.Fatalf("ResolveModel(deepseek-pro) = %+v, %v; want legacy pro", got, ok)
	}
}

func TestResolveModelWithFallback(t *testing.T) {
	c := testModelFallbackConfig(t)

	cases := []struct {
		name         string
		ref          string
		wantResolved string
		wantFallback bool
		wantOK       bool
	}{
		{"direct provider model", "prov-a/model-a2", "prov-a/model-a2", false, true},
		{"provider name", "prov-b", "prov-b/model-b1", false, true},
		{"bare model", "model-a1", "prov-a/model-a1", false, true},
		{"empty falls back", "", "prov-a/model-a1", true, true},
		{"stale falls back", "deleted/model", "prov-a/model-a1", true, true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, fallback, ok := c.ResolveModelWithFallback(tt.ref)
			if got != tt.wantResolved || fallback != tt.wantFallback || ok != tt.wantOK {
				t.Fatalf("ResolveModelWithFallback(%q) = (%q, %v, %v), want (%q, %v, %v)", tt.ref, got, fallback, ok, tt.wantResolved, tt.wantFallback, tt.wantOK)
			}
		})
	}

	c.Providers = nil
	if got, fallback, ok := c.ResolveModelWithFallback("deleted/model"); got != "" || fallback || ok {
		t.Fatalf("no providers fallback = (%q, %v, %v), want empty false false", got, fallback, ok)
	}
}

func TestResolveModelWithFallbackSkipsKeylessProvider(t *testing.T) {
	c := testModelFallbackConfig(t)
	// Make the first provider keyless. A fallback must skip it and pick the next
	// configured provider, rather than booting a tab onto a provider with no API
	// key (which just fails on first use).
	c.Providers[0].APIKeyEnv = "KWU_TEST_EMPTY"
	c.Providers[0].resolvedAPIKey = ""

	got, fallback, ok := c.ResolveModelWithFallback("")
	if !ok || !fallback {
		t.Fatalf("ResolveModelWithFallback(\"\") = (%q, %v, %v), want a fallback", got, fallback, ok)
	}
	if got != "prov-b/model-b1" {
		t.Errorf("fallback = %q, want prov-b/model-b1 (prov-a is keyless and must be skipped)", got)
	}
}

// TestResolveModelWithFallbackHonorsDefaultModel verifies that when the ref is
// stale or empty, the function tries c.DefaultModel before iterating providers
// in order. Without this, the first provider in the config always wins
// even when the user has configured a different default_model (#3801).
func TestResolveModelWithFallbackHonorsDefaultModel(t *testing.T) {
	c := testModelFallbackConfig(t)
	// Set DefaultModel to prov-b (not the first provider). An empty/stale ref
	// should fall back to prov-b, not prov-a.
	c.DefaultModel = "prov-b"

	// Empty ref → must use DefaultModel = prov-b
	got, fallback, ok := c.ResolveModelWithFallback("")
	if !ok || !fallback {
		t.Fatalf("ResolveModelWithFallback(\"\") = (%q, %v, %v), want fallback to prov-b", got, fallback, ok)
	}
	if got != "prov-b/model-b1" {
		t.Errorf("empty ref fallback = %q, want prov-b/model-b1 (DefaultModel)", got)
	}

	// Stale ref → same: must use DefaultModel
	got, fallback, ok = c.ResolveModelWithFallback("deleted/model")
	if !ok || !fallback {
		t.Fatalf("ResolveModelWithFallback(\"deleted/model\") = (%q, %v, %v), want fallback", got, fallback, ok)
	}
	if got != "prov-b/model-b1" {
		t.Errorf("stale ref fallback = %q, want prov-b/model-b1 (DefaultModel)", got)
	}

	// DefaultModel pointing to a keyless provider must be skipped
	c.Providers[1].APIKeyEnv = "KWU_TEST_EMPTY" // prov-b, the DefaultModel
	c.Providers[1].resolvedAPIKey = ""
	got, fallback, ok = c.ResolveModelWithFallback("stale/ref")
	if !ok || !fallback {
		t.Fatalf("keyless DefaultModel fallback = (%q, %v, %v), want next configured", got, fallback, ok)
	}
	if got != "prov-a/model-a1" {
		t.Errorf("keyless DefaultModel fallback = %q, want prov-a/model-a1 (only configured left)", got)
	}
}

func TestModelRefsProvider(t *testing.T) {
	if !ModelRefsProvider("deepseek-flash", "deepseek-flash") {
		t.Fatal("bare provider ref should match provider")
	}
	if !ModelRefsProvider("deepseek-flash/deepseek-v4-flash", "deepseek-flash") {
		t.Fatal("provider/model ref should match provider")
	}
	if ModelRefsProvider("other/model", "deepseek-flash") {
		t.Fatal("different provider should not match")
	}
	if ModelRefsProvider("", "deepseek-flash") {
		t.Fatal("empty ref should not match")
	}
}

func TestRemoveProviderMigratesDanglingRefs(t *testing.T) {
	c := testModelFallbackConfig(t)
	c.DefaultModel = "model-a2"
	c.Agent.PlannerModel = "prov-a"
	c.Agent.SubagentModel = "prov-a/model-a1"
	c.Agent.SubagentModels = map[string]string{
		"review":  "prov-a/model-a2",
		"bare":    "model-a1",
		"explore": "prov-b/model-b1",
	}

	if err := c.RemoveProvider("prov-a"); err != nil {
		t.Fatalf("RemoveProvider: %v", err)
	}
	if _, ok := c.Provider("prov-a"); ok {
		t.Fatal("provider should be removed")
	}
	if c.DefaultModel != "prov-b" {
		t.Fatalf("default_model = %q, want prov-b", c.DefaultModel)
	}
	if c.Agent.PlannerModel != "prov-b" {
		t.Fatalf("planner_model = %q, want prov-b", c.Agent.PlannerModel)
	}
	if c.Agent.SubagentModel != "prov-b" {
		t.Fatalf("subagent_model = %q, want prov-b", c.Agent.SubagentModel)
	}
	if c.Agent.SubagentModels["review"] != "prov-b" {
		t.Fatalf("subagent_models.review = %q, want prov-b", c.Agent.SubagentModels["review"])
	}
	if c.Agent.SubagentModels["bare"] != "prov-b" {
		t.Fatalf("subagent_models.bare = %q, want prov-b", c.Agent.SubagentModels["bare"])
	}
	if c.Agent.SubagentModels["explore"] != "prov-b/model-b1" {
		t.Fatalf("unaffected subagent model changed to %q", c.Agent.SubagentModels["explore"])
	}
}

func TestRemoveProviderBlocksDefaultWithoutFallback(t *testing.T) {
	c := testModelFallbackConfig(t)
	c.DefaultModel = "prov-a/model-a1"
	c.Providers[1].APIKeyEnv = "KWU_TEST_EMPTY"
	c.Providers[1].resolvedAPIKey = ""

	err := c.RemoveProvider("prov-a")
	if err == nil {
		t.Fatal("expected removing default provider without fallback to fail")
	}
	if !strings.Contains(err.Error(), "default_model") {
		t.Fatalf("error = %q, want default_model mention", err)
	}
	if _, ok := c.Provider("prov-a"); !ok {
		t.Fatal("provider should remain after failed removal")
	}
}

func TestRemoveProviderClearsOptionalRefsWithoutFallback(t *testing.T) {
	c := testModelFallbackConfig(t)
	c.DefaultModel = "prov-b"
	c.Agent.PlannerModel = "prov-a/model-a1"
	c.Agent.SubagentModel = "prov-a"
	c.Agent.SubagentModels = map[string]string{"review": "prov-a/model-a2"}
	c.Providers[1].APIKeyEnv = "KWU_TEST_EMPTY"
	c.Providers[1].resolvedAPIKey = ""

	if err := c.RemoveProvider("prov-a"); err != nil {
		t.Fatalf("RemoveProvider: %v", err)
	}
	if c.Agent.PlannerModel != "" {
		t.Fatalf("planner_model = %q, want cleared", c.Agent.PlannerModel)
	}
	if c.Agent.SubagentModel != "" {
		t.Fatalf("subagent_model = %q, want cleared", c.Agent.SubagentModel)
	}
	if _, ok := c.Agent.SubagentModels["review"]; ok {
		t.Fatal("subagent_models.review should be removed")
	}
}
