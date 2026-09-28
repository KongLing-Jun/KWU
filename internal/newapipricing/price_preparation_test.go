package newapipricing

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

func TestPricePreparationReaderSubscriptionUsesCacheAndRequiresCredits(t *testing.T) {
	reader, transportCalls, statusCalls := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatalf("NewPricePreparationReader: %v", err)
	}

	input := pricePreparationTestInput(modelbilling.BillingSubscriptionPool, "model-a")
	withoutCredits := preparation.Prepare(input)
	if withoutCredits.CachedSnapshot == nil || withoutCredits.Evidence == nil ||
		withoutCredits.Decision.Action != StartBlock ||
		!slices.Contains(withoutCredits.Decision.Reasons, StartCreditsUnavailable) {
		t.Fatalf("preparation without credits = %+v", withoutCredits)
	}

	input.CreditConfig = &CreditConfigInput{
		Config:       modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart},
		Authenticity: SourceMockOnly,
	}
	withCredits := preparation.Prepare(input)
	if withCredits.Failure != nil || withCredits.Evidence == nil ||
		withCredits.Evidence.ModelID != input.ModelID ||
		withCredits.Decision.Action != StartAllowDegraded ||
		withCredits.Decision.BillingSource != modelbilling.BillingSubscriptionPool {
		t.Fatalf("preparation with credits = %+v", withCredits)
	}
	if transportCalls.Load() != 1 || statusCalls.Load() != 1 {
		t.Fatalf("Prepare performed I/O: transport=%d status=%d", transportCalls.Load(), statusCalls.Load())
	}
}

func TestPricePreparationReaderBYOKUsageUnavailableUsesRealSnapshotEvidence(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	input := pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a")
	input.Usage = modelbilling.NormalizedUsage{BreakdownStatus: modelbilling.UsageMissing}

	result := preparation.Prepare(input)
	if result.Estimate.State != EstimateUnavailable || result.Evidence == nil || result.CachedSnapshot == nil ||
		result.Evidence.SnapshotID != result.CachedSnapshot.Snapshot.SnapshotID ||
		result.Evidence.State != result.CachedSnapshot.Snapshot.State ||
		result.Decision.Action != StartAllowDegraded ||
		result.Decision.BillingSource != modelbilling.BillingNewAPIByok ||
		result.Failure == nil || result.Failure.Code != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("BYOK preparation = %+v", result)
	}
}

func TestPricePreparationReaderClassifiesMissingCacheByBillingSource(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	reader, err := NewReader(cache)
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}

	subscription := preparation.Prepare(pricePreparationTestInput(modelbilling.BillingSubscriptionPool, "model-a"))
	if subscription.Decision.Action != StartBlock || subscription.Evidence != nil || subscription.Failure == nil ||
		subscription.Failure.Code != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("subscription missing cache = %+v", subscription)
	}

	byokInput := pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a")
	byok := preparation.Prepare(byokInput)
	if byok.Decision.Action != StartAllowDegraded || byok.Evidence == nil || byok.CachedSnapshot != nil ||
		byok.Failure == nil || byok.Failure.Code != modelbilling.ErrorPricingUnavailable ||
		byok.Failure.Reason != ReasonSnapshotUnavailable || byok.Estimate.State != EstimateUnavailable ||
		byok.Estimate.Complete || byok.Estimate.SnapshotID != "pricing-unavailable-v1:domestic" ||
		byok.Estimate.ModelID != byokInput.ModelID || byok.Estimate.PricingGroup != byokInput.PricingGroup ||
		byok.Estimate.PricingState != modelbilling.PricingUnavailable ||
		byok.Evidence.SnapshotID != byok.Estimate.SnapshotID || byok.Evidence.State != modelbilling.PricingUnavailable ||
		byok.Evidence.PricingVersion != nil || byok.Evidence.PricingHash != nil {
		t.Fatalf("BYOK missing cache = %+v", byok)
	}
}

func TestPricePreparationReaderBlocksInvalidSourceAndUnavailableReader(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	input := pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a")
	input.Source = "../domestic"
	invalidSource := preparation.Prepare(input)
	if invalidSource.Decision.Action != StartBlock || invalidSource.Evidence != nil || invalidSource.Failure == nil ||
		invalidSource.Failure.Code != modelbilling.ErrorContractInvalid {
		t.Fatalf("invalid source = %+v", invalidSource)
	}

	for name, unavailable := range map[string]*PricePreparationReader{
		"nil receiver": nil,
		"zero value":   {},
		"empty reader": {reader: &Reader{}},
	} {
		t.Run(name, func(t *testing.T) {
			result := unavailable.Prepare(pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a"))
			if result.Decision.Action != StartBlock || result.Evidence != nil || result.Failure == nil ||
				result.Failure.Code != modelbilling.ErrorContractInvalid {
				t.Fatalf("unavailable reader = %+v", result)
			}
		})
	}
}

func TestPricePreparationReaderBYOKStaleBlockedUsesRealSnapshotEvidence(t *testing.T) {
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, t.TempDir(), clock)
	mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricingWithRequestModel()))
	clock.Set(cacheTestStart.Add(25 * time.Hour))
	reader, err := NewReader(cache)
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}

	result := preparation.Prepare(pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a"))
	if result.Decision.Action != StartAllowDegraded || result.CachedSnapshot == nil || result.Evidence == nil ||
		result.CachedSnapshot.Snapshot.State != modelbilling.PricingStaleBlocked ||
		result.Evidence.State != modelbilling.PricingStaleBlocked ||
		result.Evidence.SnapshotID != result.CachedSnapshot.Snapshot.SnapshotID ||
		result.Evidence.SnapshotID == "pricing-unavailable-v1:domestic" {
		t.Fatalf("stale BYOK preparation = %+v", result)
	}
}

func TestPricePreparationReaderRejectsInputMismatch(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*PricePreparationInput){
		"billing source": func(input *PricePreparationInput) { input.BillingSource = "other" },
		"model":          func(input *PricePreparationInput) { input.ModelID = "missing" },
		"group":          func(input *PricePreparationInput) { input.PricingGroup = "missing" },
		"scope": func(input *PricePreparationInput) {
			input.Scope.CloudBinding = &cloud.BindingScope{ProfileID: "other", CloudUserID: "user_01", BindingGeneration: 1}
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := pricePreparationTestInput(modelbilling.BillingSubscriptionPool, "model-a")
			mutate(&input)
			result := preparation.Prepare(input)
			if result.Evidence != nil || result.Decision.Action != StartBlock || result.Failure == nil {
				t.Fatalf("mismatch result = %+v", result)
			}
		})
	}
}

func TestPricePreparationReaderBuildsSeparateEvidenceForEachModel(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, modelID := range []string{"model-a", "model-b"} {
		input := pricePreparationTestInput(modelbilling.BillingNewAPIByok, modelID)
		result := preparation.Prepare(input)
		if result.Evidence == nil || result.Evidence.ModelID != modelID || result.Estimate.ModelID != modelID {
			t.Fatalf("model %q preparation = %+v", modelID, result)
		}
	}
}

func newPricePreparationTestReader(t *testing.T) (*Reader, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	var transportCalls atomic.Int32
	var statusCalls atomic.Int32
	request := cacheTestRequest("domestic", cacheTestPricingWithRequestModel())
	request.Transport = transportFunc(func(context.Context, string) ([]byte, error) {
		transportCalls.Add(1)
		return []byte(cacheTestPricingWithRequestModel()), nil
	})
	request.StatusSource = statusSourceFunc(func(context.Context) (modelbilling.PricingSourceStatus, error) {
		statusCalls.Add(1)
		return cacheTestStatus(), nil
	})
	mustRefreshCache(t, cache, request)
	reader, err := NewReader(cache)
	if err != nil {
		t.Fatal(err)
	}
	return reader, &transportCalls, &statusCalls
}

func pricePreparationTestInput(source modelbilling.BillingSource, modelID string) PricePreparationInput {
	scope := modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"}
	if source == modelbilling.BillingSubscriptionPool {
		binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user_01", BindingGeneration: 1}
		scope.CloudBinding = &binding
	}
	return PricePreparationInput{
		Source: "domestic", BillingSource: source, EvidenceID: "evidence_" + modelID,
		TaskID: "task_01", Scope: scope, ModelID: modelID, PricingGroup: "default",
		Usage: exactZeroUsage(), CapturedAt: cacheTestStart,
	}
}

func cacheTestPricingWithRequestModel() string {
	return `{
		"success":true,
		"pricing_version":"pricing-v1",
		"group_ratio":{"default":1},
		"usable_group":{"default":"Default"},
		"data":[
			{"model_name":"model-a","quota_type":0,"model_ratio":1,"completion_ratio":2,"cache_ratio":0.1,"enable_groups":["default"],"supported_endpoint_types":["openai"]},
			{"model_name":"model-b","quota_type":0,"model_ratio":3,"completion_ratio":4,"cache_ratio":0.2,"enable_groups":["default"],"supported_endpoint_types":["openai"]}
		]
	}`
}
