package newapipricing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

func TestAdapterMapsPricingWireToNormalizedSnapshot(t *testing.T) {
	raw := []byte(`{
		"success": true,
		"pricing_version": "pricing-v1",
		"group_ratio": {"default": 1, "vip": 5e-1},
		"usable_group": {"vip": "VIP", "default": "Default"},
		"vendors": [{"id": 3, "name": "Anthropic", "icon": "anthropic"}],
		"data": [
			{
				"model_name": "claude-test", "vendor_id": 3, "quota_type": 0,
				"model_ratio": 2.5, "completion_ratio": 5, "cache_ratio": 1e-7,
				"enable_groups": ["default"], "supported_endpoint_types": ["anthropic", "openai"]
			},
			{
				"model_name": "image-test", "quota_type": 1, "model_price": 0.5,
				"enable_groups": ["vip"], "supported_endpoint_types": ["image-generation", "openai"]
			},
			{
				"model_name": "tiered-test", "quota_type": 0, "model_ratio": 1,
				"billing_mode": "tiered_expr", "billing_expr": "tier(\"base\", p * 2)",
				"enable_groups": ["default"], "supported_endpoint_types": ["openai-response"]
			}
		]
	}`)
	fetchedAt := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	header := SnapshotHeader{
		SnapshotID: "snapshot-01", Source: "newapi_pricing", SourceHash: "sha256:test", Currency: "CNY",
		FetchedAt: fetchedAt, FreshUntil: fetchedAt.Add(12 * time.Hour), StaleUntil: fetchedAt.Add(24 * time.Hour),
		State: modelbilling.PricingFresh,
	}

	catalog, snapshot, err := (Adapter{}).Adapt(raw, header)
	if err != nil {
		t.Fatalf("Adapt: %v", err)
	}
	if len(catalog.Issues) != 0 || len(snapshot.Models) != 3 {
		t.Fatalf("issues/models = %d/%d, want 0/3", len(catalog.Issues), len(snapshot.Models))
	}
	if catalog.Availability() != CatalogComplete {
		t.Fatalf("Availability = %q, want complete", catalog.Availability())
	}
	if snapshot.SourceVersion == nil || *snapshot.SourceVersion != "pricing-v1" {
		t.Fatalf("SourceVersion = %v", snapshot.SourceVersion)
	}
	if snapshot.GroupRatios["default"] != "1" || snapshot.GroupRatios["vip"] != "0.5" {
		t.Fatalf("GroupRatios = %+v", snapshot.GroupRatios)
	}
	if strings.Join(snapshot.UsableGroups, ",") != "default,vip" {
		t.Fatalf("UsableGroups = %+v", snapshot.UsableGroups)
	}

	token := snapshot.Models[0]
	if token.ModelID != "claude-test" || token.ProviderModelID != "claude-test" || token.ModelType != "chat" {
		t.Fatalf("token identity/type = %+v", token)
	}
	if token.QuotaType != modelbilling.QuotaTypeToken || token.BillingMode != modelbilling.BillingModeTokenRatio {
		t.Fatalf("token quota/mode = %q/%q", token.QuotaType, token.BillingMode)
	}
	if token.CacheRatio == nil || *token.CacheRatio != "0.0000001" {
		t.Fatalf("CacheRatio = %v, want canonical exponent expansion", token.CacheRatio)
	}
	if catalog.Models[0].Vendor == nil || catalog.Models[0].Vendor.ID != 3 || catalog.Models[0].Vendor.Name != "Anthropic" {
		t.Fatalf("vendor mapping = %+v", catalog.Models[0].Vendor)
	}

	perRequest := snapshot.Models[1]
	if perRequest.QuotaType != modelbilling.QuotaTypePerRequest || perRequest.BillingMode != modelbilling.BillingModePerRequest {
		t.Fatalf("request quota/mode = %q/%q", perRequest.QuotaType, perRequest.BillingMode)
	}
	if perRequest.ModelType != "image" || perRequest.ModelPrice == nil || *perRequest.ModelPrice != "0.5" {
		t.Fatalf("request model mapping = %+v", perRequest)
	}

	tiered := snapshot.Models[2]
	if tiered.BillingMode != modelbilling.BillingModeTieredExpr || tiered.BillingExpression == nil {
		t.Fatalf("tiered mapping = %+v", tiered)
	}
}

func TestAdapterDistinguishesNoPricesFromPartialCatalog(t *testing.T) {
	header := validSnapshotHeader()
	t.Run("empty data", func(t *testing.T) {
		raw := []byte(`{"success":true,"data":[]}`)
		catalog, err := (Adapter{}).Decode(raw)
		if err != nil || catalog.Availability() != CatalogNoUsablePrices {
			t.Fatalf("Decode = availability %q, error %v", catalog.Availability(), err)
		}
		returned, snapshot, err := (Adapter{}).Adapt(raw, header)
		if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
			t.Fatalf("Adapt error = %v", err)
		}
		if returned.Availability() != CatalogNoUsablePrices || len(snapshot.Models) != 0 || snapshot.State != "" {
			t.Fatalf("Adapt returned catalog/snapshot = %q/%+v", returned.Availability(), snapshot)
		}
	})

	t.Run("all models invalid", func(t *testing.T) {
		raw := []byte(`{
			"success":true,
			"data":[
				{"model_name":"missing-price","quota_type":0,"supported_endpoint_types":["openai"]},
				{"model_name":"unknown-quota","quota_type":9,"model_ratio":1,"supported_endpoint_types":["openai"]}
			]
		}`)
		catalog, snapshot, err := (Adapter{}).Adapt(raw, header)
		if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
			t.Fatalf("Adapt error = %v", err)
		}
		if catalog.Availability() != CatalogNoUsablePrices || len(catalog.Issues) != 2 || snapshot.State != "" {
			t.Fatalf("catalog/snapshot = %+v/%+v", catalog, snapshot)
		}
	})

	t.Run("partial catalog", func(t *testing.T) {
		raw := []byte(`{
			"success":true,
			"data":[
				{"model_name":"valid","quota_type":0,"model_ratio":1,"supported_endpoint_types":["openai"]},
				{"model_name":"unknown-quota","quota_type":9,"model_ratio":1,"supported_endpoint_types":["openai"]}
			]
		}`)
		catalog, snapshot, err := (Adapter{}).Adapt(raw, header)
		if err != nil {
			t.Fatalf("Adapt: %v", err)
		}
		if catalog.Availability() != CatalogPartial || len(catalog.Issues) != 1 || len(snapshot.Models) != 1 {
			t.Fatalf("catalog/snapshot = %+v/%+v", catalog, snapshot)
		}
	})
}

func TestCanonicalDecimalHandlesFloat64WireExponentRangeExactly(t *testing.T) {
	positiveBoundary := "17976931348623157" + strings.Repeat("0", 292)
	negativeBoundary := "0." + strings.Repeat("0", 323) + "5"
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "5e-7", want: "0.0000005"},
		{raw: "0.5e1", want: "5"},
		{raw: "1.20e2", want: "120"},
		{raw: "1.7976931348623157e308", want: positiveBoundary},
		{raw: "5e-324", want: negativeBoundary},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := canonicalDecimal(tt.raw)
			if err != nil || got != tt.want {
				t.Fatalf("canonicalDecimal(%q) = %q, %v", tt.raw, got, err)
			}
		})
	}

	for _, raw := range []string{"1e309", "1e-325", "9e308", "1e999999"} {
		t.Run("reject "+raw, func(t *testing.T) {
			if _, err := canonicalDecimal(raw); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
				t.Fatalf("canonicalDecimal(%q) error = %v", raw, err)
			}
		})
	}

	hugeExponent := "1e" + strings.Repeat("9", 100_000)
	done := make(chan error, 1)
	go func() {
		_, err := canonicalDecimal(hugeExponent)
		done <- err
	}()
	select {
	case err := <-done:
		if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
			t.Fatalf("huge exponent error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("huge exponent was not rejected promptly")
	}
}

func TestAdaptSnapshotDoesNotAliasCatalogPrices(t *testing.T) {
	raw := []byte(`{
		"success":true,
		"pricing_version":"v1",
		"data":[
			{
				"model_name":"token","quota_type":0,"model_ratio":1,"completion_ratio":2,
				"cache_ratio":0.1,"create_cache_ratio":1.25,"image_ratio":3,
				"audio_ratio":4,"audio_completion_ratio":5,"model_price":6,
				"enable_groups":["default"],"supported_endpoint_types":["openai"]
			},
			{
				"model_name":"tiered","quota_type":0,"model_ratio":1,
				"billing_mode":"tiered_expr","billing_expr":"tier(\"base\", p * 2)",
				"enable_groups":["default"],"supported_endpoint_types":["openai"]
			}
		]
	}`)
	catalog, snapshot, err := (Adapter{}).Adapt(raw, validSnapshotHeader())
	if err != nil {
		t.Fatalf("Adapt: %v", err)
	}

	changed := modelbilling.DecimalString("9")
	for _, value := range []*modelbilling.DecimalString{
		catalog.Prices[0].ModelRatio, catalog.Prices[0].CompletionRatio,
		catalog.Prices[0].CacheRatio, catalog.Prices[0].CreateCacheRatio,
		catalog.Prices[0].ImageRatio, catalog.Prices[0].AudioRatio,
		catalog.Prices[0].AudioCompletionRatio, catalog.Prices[0].ModelPrice,
	} {
		*value = changed
	}
	catalog.Prices[0].Capabilities[0] = "changed"
	catalog.Prices[0].EnableGroups[0] = "changed"
	*catalog.Prices[1].BillingExpression = "changed"
	*catalog.SourceVersion = "changed"

	got := snapshot.Models[0]
	if *got.ModelRatio != "1" || *got.CompletionRatio != "2" || *got.CacheRatio != "0.1" ||
		*got.CreateCacheRatio != "1.25" || *got.ImageRatio != "3" || *got.AudioRatio != "4" ||
		*got.AudioCompletionRatio != "5" || *got.ModelPrice != "6" {
		t.Fatalf("snapshot decimal pointers changed: %+v", got)
	}
	if got.Capabilities[0] != "text" || got.EnableGroups[0] != "default" {
		t.Fatalf("snapshot slices changed: %+v", got)
	}
	if *snapshot.Models[1].BillingExpression != "tier(\"base\", p * 2)" || *snapshot.SourceVersion != "v1" {
		t.Fatalf("snapshot strings changed: %+v version=%v", snapshot.Models[1], snapshot.SourceVersion)
	}
}

func TestAdapterIsolatesInvalidModelsWithStableContractErrors(t *testing.T) {
	raw := []byte(`{
		"success": true,
		"data": [
			{"model_name":"valid", "quota_type":0, "model_ratio":1, "supported_endpoint_types":["openai"]},
			{"model_name":" ", "quota_type":0, "model_ratio":1, "supported_endpoint_types":["openai"]},
			{"model_name":"unknown-type", "quota_type":0, "model_ratio":1, "supported_endpoint_types":["future-protocol"]},
			{"model_name":"unknown-quota", "quota_type":2, "model_ratio":1, "supported_endpoint_types":["openai"]},
			{"model_name":"unknown-mode", "quota_type":0, "model_ratio":1, "billing_mode":"future_mode", "supported_endpoint_types":["openai"]},
			{"model_name":"missing-price", "quota_type":0, "supported_endpoint_types":["openai"]}
		]
	}`)

	catalog, err := (Adapter{}).Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(catalog.Prices) != 1 || catalog.Prices[0].ModelID != "valid" {
		t.Fatalf("valid prices = %+v", catalog.Prices)
	}
	if len(catalog.Models) != 5 {
		t.Fatalf("display models = %d, want named valid and rejected models", len(catalog.Models))
	}
	if len(catalog.Issues) != 5 {
		t.Fatalf("issues = %+v", catalog.Issues)
	}
	for _, issue := range catalog.Issues {
		if modelbilling.ErrorCodeOf(issue.Err) != modelbilling.ErrorContractInvalid {
			t.Fatalf("issue %q error = %v", issue.ModelName, issue.Err)
		}
	}
}

func TestAdapterRejectsInvalidEnvelopes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		code modelbilling.ErrorCode
	}{
		{name: "failure", raw: `{"success":false,"message":"blocked"}`, code: modelbilling.ErrorPricingUnavailable},
		{name: "missing success", raw: `{"data":[]}`, code: modelbilling.ErrorContractInvalid},
		{name: "missing data", raw: `{"success":true}`, code: modelbilling.ErrorContractInvalid},
		{name: "invalid json", raw: `{`, code: modelbilling.ErrorContractInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (Adapter{}).Decode([]byte(tt.raw))
			if modelbilling.ErrorCodeOf(err) != tt.code {
				t.Fatalf("error = %v, want code %q", err, tt.code)
			}
		})
	}
}

func TestHTTPTransportOwnsOnlyRequestAndResponseBytes(t *testing.T) {
	var accept, userAgentHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		userAgentHeader = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer server.Close()

	body, err := NewHTTPTransport(server.Client()).Fetch(context.Background(), server.URL)
	if err != nil || string(body) != `{"success":true,"data":[]}` {
		t.Fatalf("Fetch = %s, %v", body, err)
	}
	if accept != "application/json" || userAgentHeader != userAgent {
		t.Fatalf("headers = %q/%q", accept, userAgentHeader)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	_, err = NewHTTPTransport(failing.Client()).Fetch(context.Background(), failing.URL)
	var domainErr *modelbilling.Error
	if !errors.As(err, &domainErr) || domainErr.Code != modelbilling.ErrorPricingUnavailable || !domainErr.Retryable {
		t.Fatalf("503 error = %#v", err)
	}
}

func TestHTTPTransportPreservesContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer server.Close()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewHTTPTransport(server.Client()).Fetch(cancelled, server.URL)
	if !errors.Is(err, context.Canceled) || modelbilling.ErrorCodeOf(err) != "" {
		t.Fatalf("pre-cancelled error = %#v", err)
	}

	expired, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer deadlineCancel()
	_, err = NewHTTPTransport(server.Client()).Fetch(expired, server.URL)
	if !errors.Is(err, context.DeadlineExceeded) || modelbilling.ErrorCodeOf(err) != "" {
		t.Fatalf("expired deadline error = %#v", err)
	}
}

func TestHTTPTransportPreservesCancellationWhileReadingResponse(t *testing.T) {
	headersSent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(headersSent)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := NewHTTPTransport(server.Client()).Fetch(ctx, server.URL)
		result <- err
	}()
	<-headersSent
	cancel()
	err := <-result
	if !errors.Is(err, context.Canceled) || modelbilling.ErrorCodeOf(err) != "" {
		t.Fatalf("in-flight cancellation error = %#v", err)
	}
}

func validSnapshotHeader() SnapshotHeader {
	fetchedAt := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	return SnapshotHeader{
		SnapshotID: "snapshot-test", Source: "newapi_pricing", SourceHash: "sha256:test", Currency: "CNY",
		FetchedAt: fetchedAt, FreshUntil: fetchedAt.Add(12 * time.Hour), StaleUntil: fetchedAt.Add(24 * time.Hour),
		State: modelbilling.PricingFresh,
	}
}

type fixtureStatusSource struct {
	status modelbilling.PricingSourceStatus
	err    error
}

func (s fixtureStatusSource) FetchStatus(context.Context) (modelbilling.PricingSourceStatus, error) {
	return s.status, s.err
}

func TestStatusRemainsNormalizedFixtureOnly(t *testing.T) {
	file, err := os.Open("../modelbilling/testdata/NEWAPI-STATUS-01/normalized-v1.json")
	if err != nil {
		t.Fatalf("open status fixture: %v", err)
	}
	defer file.Close()
	bundle, err := modelbilling.DecodeStatusFixture(file)
	if err != nil {
		t.Fatalf("DecodeStatusFixture: %v", err)
	}

	got, err := LoadStatus(context.Background(), fixtureStatusSource{status: *bundle.Cases[0].Status})
	if err != nil || got.SourceVersion == nil || *got.SourceVersion != "fixture-v1" {
		t.Fatalf("LoadStatus = %+v, %v", got, err)
	}
	fixtureErr := bundle.Cases[1].Error
	_, err = LoadStatus(context.Background(), fixtureStatusSource{err: &modelbilling.Error{
		Code: fixtureErr.Code, Message: fixtureErr.Message, Retryable: fixtureErr.Retryable,
	}})
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("fixture status error = %v", err)
	}
}
