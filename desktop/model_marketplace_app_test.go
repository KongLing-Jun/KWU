package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

func TestDefaultModelMarketplaceUsesOnlyAirClaudeInternationalSource(t *testing.T) {
	if len(defaultModelMarketplaceSources) != 1 {
		t.Fatalf("default sources = %+v", defaultModelMarketplaceSources)
	}
	source := defaultModelMarketplaceSources[0]
	if source.ID != "international" || source.Label != "国际模型" ||
		source.Endpoint != "https://api.airclaudeai.com/api/pricing" ||
		source.LandingURL != "https://www.airclaudeai.com/models" {
		t.Fatalf("default source = %+v", source)
	}
}

func TestNewAppKeepsMarketplaceSeparateFromTaskPricePreparation(t *testing.T) {
	t.Setenv("KWU_CACHE_HOME", t.TempDir())
	app := NewApp()
	if app.pricingReader == nil || app.pricePreparationReader == nil || app.modelMarketplace == nil {
		t.Fatalf("pricing dependencies = reader %p preparation %p marketplace %p", app.pricingReader, app.pricePreparationReader, app.modelMarketplace)
	}
	if app.modelMarketplace.cache == nil || app.modelMarketplace.transport == nil {
		t.Fatalf("marketplace dependencies = cache %T transport %T", app.modelMarketplace.cache, app.modelMarketplace.transport)
	}
}

func TestAirClaudeMarketplaceProjectsOnlyReturnedFactsWithExactNumbers(t *testing.T) {
	config := defaultModelMarketplaceSources[0]
	cached, err := buildAirClaudeMarketplaceCache(config, []byte(`{
		"success": true,
		"pricing_version": "air-v1",
		"vendors": [{"id": 49, "name": "Anthropic", "icon": "Claude.Color"}],
		"data": [
			{
				"model_name": "claude-test", "vendor_id": 49, "billing_type": "per_token", "quota_type": 0,
				"icon": "Claude.Color",
				"tags": "Claude,Chat", "enable_groups": ["default"], "supported_endpoint_types": ["anthropic"],
				"input_price": 0, "output_price": 0.30000000000000004,
				"context_tiered_price": [{"max_tokens": -1, "input_price": 2, "cache_read_price": 0.6000000000000001}],
				"future_field": "ignored"
			},
			{"model_name": "request-test", "billing_type": "per_request", "quota_type": 1, "request_price": 12.50},
			{"vendor_id": 49, "input_price": 9}
		],
		"future_top_level": true
	}`), time.Date(2026, 7, 16, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !cached.Partial || cached.PricingVersion != "air-v1" || len(cached.Models) != 2 {
		t.Fatalf("cache = %+v", cached)
	}
	model := cached.Models[0]
	if model.Name != "claude-test" || model.Icon != "Claude.Color" || model.Vendor != "Anthropic" || model.VendorIcon != "Claude.Color" ||
		model.BillingType != "per_token" || model.QuotaType == nil || *model.QuotaType != 0 {
		t.Fatalf("model identity = %+v", model)
	}
	if got := marketplaceFact(model.Prices, "input_price"); got != "0" {
		t.Fatalf("input_price = %q", got)
	}
	if got := marketplaceFact(model.Prices, "output_price"); got != "0.30000000000000004" {
		t.Fatalf("output_price = %q", got)
	}
	if got := marketplaceFact(model.Prices, "cache_read_price"); got != "" {
		t.Fatalf("missing cache_read_price was fabricated: %q", got)
	}
	if len(model.PricingTiers) != 1 || model.PricingTiers[0].MaxTokens == nil || *model.PricingTiers[0].MaxTokens != "-1" ||
		marketplaceFact(model.PricingTiers[0].Prices, "cache_read_price") != "0.6000000000000001" {
		t.Fatalf("pricing tiers = %+v", model.PricingTiers)
	}
	request := cached.Models[1]
	if got := marketplaceFact(request.Prices, "request_price"); got != "12.50" {
		t.Fatalf("request_price = %q", got)
	}
}

func TestAirClaudeMarketplaceSortsModelsByNameDeterministically(t *testing.T) {
	orders := [][]airClaudePricingModel{
		{{ModelName: "zeta"}, {ModelName: "beta"}, {ModelName: "alpha"}, {ModelName: "Alpha"}},
		{{ModelName: "Alpha"}, {ModelName: "zeta"}, {ModelName: "alpha"}, {ModelName: "beta"}},
	}
	want := []string{"Alpha", "alpha", "beta", "zeta"}
	for index, data := range orders {
		models, partial := projectAirClaudeModels(airClaudePricingEnvelope{Data: data})
		if partial || len(models) != len(want) {
			t.Fatalf("order %d projection = %+v, partial=%t", index, models, partial)
		}
		for modelIndex, name := range want {
			if models[modelIndex].Name != name {
				t.Fatalf("order %d model names[%d] = %q, want %q", index, modelIndex, models[modelIndex].Name, name)
			}
		}
	}
}

func TestModelMarketplaceSourceFromCacheSortsLegacyModelsWithoutMutation(t *testing.T) {
	cached := modelMarketplaceCacheFile{
		Models: []ModelMarketplaceModel{
			{ID: "zeta", Name: "zeta"},
			{ID: "beta", Name: "beta"},
			{ID: "alpha", Name: "alpha"},
			{ID: "Alpha", Name: "Alpha"},
		},
	}

	source := modelMarketplaceSourceFromCache(defaultModelMarketplaceSources[0], cached, time.Now())
	requireMarketplaceModelNames(t, source.Models, "Alpha", "alpha", "beta", "zeta")
	requireMarketplaceModelNames(t, cached.Models, "zeta", "beta", "alpha", "Alpha")
}

func TestModelMarketplaceReadIsCacheOnlyAndRefreshFallsBackToReturnedCache(t *testing.T) {
	config := defaultModelMarketplaceSources[0]
	cache, err := newModelMarketplaceDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	transport := &modelMarketplaceTestTransport{body: []byte(`{
		"success":true,
		"pricing_version":"air-v1",
		"vendors":[{"id":52,"name":"Google"}],
		"data":[{"model_name":"gemini-test","vendor_id":52,"billing_type":"per_token","quota_type":0,"input_price":0.5}]
	}`)}
	service := &modelMarketplaceService{
		cache: cache, transport: transport, sources: []modelMarketplaceSourceConfig{config},
		refreshTimeout: time.Second, now: func() time.Time { return time.Date(2026, 7, 16, 1, 2, 3, 0, time.UTC) },
	}

	empty := service.Read()
	if transport.calls.Load() != 0 || len(empty.Sources) != 1 || empty.Sources[0].PricingState != string(modelbilling.PricingUnavailable) {
		t.Fatalf("cache-only empty view = %+v calls=%d", empty, transport.calls.Load())
	}

	refreshed := service.Refresh(context.Background())
	if transport.calls.Load() != 1 || len(refreshed.Sources) != 1 || len(refreshed.Sources[0].Models) != 1 {
		t.Fatalf("refreshed view = %+v calls=%d", refreshed, transport.calls.Load())
	}
	source := refreshed.Sources[0]
	if source.SourceURL != "https://api.airclaudeai.com/api/pricing" || source.PricingURL != "https://www.airclaudeai.com/models" ||
		source.PricingVersion != "air-v1" || marketplaceFact(source.Models[0].Prices, "input_price") != "0.5" {
		t.Fatalf("refreshed source = %+v", source)
	}

	read := service.Read()
	if transport.calls.Load() != 1 || len(read.Sources) != 1 || len(read.Sources[0].Models) != 1 ||
		marketplaceFact(read.Sources[0].Models[0].Prices, "input_price") != "0.5" {
		t.Fatalf("read performed network or lost refreshed cache: %+v calls=%d", read, transport.calls.Load())
	}

	transport.setError(errors.New("injected network failure"))
	fallback := service.Refresh(context.Background()).Sources[0]
	if transport.calls.Load() != 2 || fallback.ErrorCode != string(modelbilling.ErrorPricingUnavailable) ||
		len(fallback.Models) != 1 || marketplaceFact(fallback.Models[0].Prices, "input_price") != "0.5" {
		t.Fatalf("fallback = %+v calls=%d", fallback, transport.calls.Load())
	}
}

func TestModelMarketplaceReadAndFallbackSortLegacyCache(t *testing.T) {
	config := defaultModelMarketplaceSources[0]
	cache, err := newModelMarketplaceDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	legacy := modelMarketplaceCacheFile{
		Version: modelMarketplaceCacheVersion, SourceID: config.ID, SourceURL: config.Endpoint,
		PricingVersion: "legacy-v1", FetchedAt: time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC),
		Models: []ModelMarketplaceModel{
			{ID: "zeta", Name: "zeta"},
			{ID: "beta", Name: "beta"},
			{ID: "alpha", Name: "alpha"},
			{ID: "Alpha", Name: "Alpha"},
		},
	}
	if err := cache.Write(legacy); err != nil {
		t.Fatal(err)
	}
	transport := &modelMarketplaceTestTransport{err: errors.New("injected network failure")}
	service := &modelMarketplaceService{
		cache: cache, transport: transport, sources: []modelMarketplaceSourceConfig{config}, refreshTimeout: time.Second,
	}

	read := service.Read()
	if transport.calls.Load() != 0 || len(read.Sources) != 1 || read.Sources[0].PricingVersion != "legacy-v1" {
		t.Fatalf("read performed network or lost legacy cache: %+v calls=%d", read, transport.calls.Load())
	}
	requireMarketplaceModelNames(t, read.Sources[0].Models, "Alpha", "alpha", "beta", "zeta")

	fallback := service.Refresh(context.Background()).Sources[0]
	if transport.calls.Load() != 1 || fallback.ErrorCode != string(modelbilling.ErrorPricingUnavailable) ||
		fallback.PricingVersion != "legacy-v1" {
		t.Fatalf("fallback = %+v calls=%d", fallback, transport.calls.Load())
	}
	requireMarketplaceModelNames(t, fallback.Models, "Alpha", "alpha", "beta", "zeta")
}

func TestModelMarketplaceReadRecomputesCacheFreshness(t *testing.T) {
	config := defaultModelMarketplaceSources[0]
	cache, err := newModelMarketplaceDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fetchedAt := time.Date(2026, 7, 16, 1, 2, 3, 0, time.UTC)
	if err := cache.Write(modelMarketplaceCacheFile{
		Version: modelMarketplaceCacheVersion, SourceID: config.ID, SourceURL: config.Endpoint,
		PricingVersion: "freshness-v1", FetchedAt: fetchedAt,
		Models: []ModelMarketplaceModel{{ID: "gpt-5.6-luna", Name: "gpt-5.6-luna"}},
	}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		at   time.Time
		want modelbilling.PricingState
	}{
		{name: "fresh", at: fetchedAt.Add(11*time.Hour + 59*time.Minute + 59*time.Second), want: modelbilling.PricingFresh},
		{name: "stale usable boundary", at: fetchedAt.Add(12 * time.Hour), want: modelbilling.PricingStaleUsable},
		{name: "stale usable", at: fetchedAt.Add(23*time.Hour + 59*time.Minute + 59*time.Second), want: modelbilling.PricingStaleUsable},
		{name: "stale blocked boundary", at: fetchedAt.Add(24 * time.Hour), want: modelbilling.PricingStaleBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &modelMarketplaceService{
				cache: cache, sources: []modelMarketplaceSourceConfig{config}, now: func() time.Time { return tt.at },
			}
			source := service.Read().Sources[0]
			if source.PricingState != string(tt.want) || source.MetadataStatus != marketplaceMetadataReady ||
				len(source.Models) != 1 || source.Models[0].Name != "gpt-5.6-luna" {
				t.Fatalf("source at %v = %+v, want state %q with retained models", tt.at, source, tt.want)
			}
		})
	}
}

func TestModelMarketplaceRejectsUnsuccessfulResponseWithoutInventingModels(t *testing.T) {
	config := defaultModelMarketplaceSources[0]
	cache, err := newModelMarketplaceDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	transport := &modelMarketplaceTestTransport{body: []byte(`{"success":false,"data":[{"model_name":"must-not-show"}]}`)}
	service := &modelMarketplaceService{
		cache: cache, transport: transport, sources: []modelMarketplaceSourceConfig{config}, refreshTimeout: time.Second,
	}
	source := service.Refresh(context.Background()).Sources[0]
	if source.PricingState != string(modelbilling.PricingUnavailable) || len(source.Models) != 0 || source.ErrorCode == "" {
		t.Fatalf("source = %+v", source)
	}
}

type modelMarketplaceTestTransport struct {
	mu    sync.Mutex
	body  []byte
	err   error
	calls atomic.Int32
}

func (t *modelMarketplaceTestTransport) Fetch(ctx context.Context, _ string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t.calls.Add(1)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return nil, t.err
	}
	return append([]byte(nil), t.body...), nil
}

func (t *modelMarketplaceTestTransport) setError(err error) {
	t.mu.Lock()
	t.err = err
	t.mu.Unlock()
}

func marketplaceFact(facts []ModelMarketplacePriceFact, key string) string {
	for _, fact := range facts {
		if fact.Key == key {
			return fact.Value
		}
	}
	return ""
}

func requireMarketplaceModelNames(t *testing.T, models []ModelMarketplaceModel, want ...string) {
	t.Helper()
	if len(models) != len(want) {
		t.Fatalf("model count = %d, want %d: %+v", len(models), len(want), models)
	}
	for index, name := range want {
		if models[index].Name != name {
			t.Fatalf("model names[%d] = %q, want %q", index, models[index].Name, name)
		}
	}
}
