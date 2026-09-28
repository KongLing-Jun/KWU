package newapipricing

import (
	"slices"
	"testing"
	"time"

	"kwu/internal/modelbilling"
)

func TestPrepareStartUsesMockSubscriptionConfigWithoutUsage(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	input := StartPreparationInput{Source: "domestic", BillingSource: modelbilling.BillingSubscriptionPool, EvidenceID: "start_evidence", TaskID: "start_task", Scope: pricePreparationTestInput(modelbilling.BillingSubscriptionPool, "model-a").Scope, ModelID: "model-a", CapturedAt: cacheTestStart, CreditConfig: &CreditConfigInput{Config: modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}, Authenticity: SourceMockOnly}}
	result := preparation.PrepareStart(input)
	if result.Decision.Action != StartAllowDegraded || result.Evidence == nil || result.Evidence.PricingGroup != "default" || !slices.Contains(result.Decision.Reasons, StartMockCreditConfig) {
		t.Fatalf("PrepareStart = %+v", result)
	}
}

func TestPrepareStartUsesGroupUnavailableBYOKEvidence(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	input := StartPreparationInput{Source: "domestic", BillingSource: modelbilling.BillingNewAPIByok, EvidenceID: "start_evidence", TaskID: "start_task", Scope: pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a").Scope, ModelID: "model-a", CapturedAt: cacheTestStart}
	result := preparation.PrepareStart(input)
	if result.Decision.Action != StartAllowDegraded || result.Evidence == nil || result.Evidence.PricingGroup != "__kwu_unavailable__" || !slices.Contains(result.Decision.Reasons, StartPricingGroupUnavailable) {
		t.Fatalf("PrepareStart = %+v", result)
	}
}

func TestPrepareStartDecisionMatrix(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	subscription := func(auth SourceAuthenticity) StartPreparationInput {
		return StartPreparationInput{Source: "domestic", BillingSource: modelbilling.BillingSubscriptionPool, EvidenceID: "e", TaskID: "t", Scope: pricePreparationTestInput(modelbilling.BillingSubscriptionPool, "model-a").Scope, ModelID: "model-a", CapturedAt: cacheTestStart, CreditConfig: &CreditConfigInput{Config: modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}, Authenticity: auth}}
	}
	if got := preparation.PrepareStart(subscription(SourceReady)); got.Decision.Action != StartAllow {
		t.Fatalf("READY fresh = %+v", got)
	}
	if got := preparation.PrepareStart(subscription(SourceMockOnly)); got.Decision.Action != StartAllowDegraded || got.Decision.Reasons[0] != StartMockCreditConfig {
		t.Fatalf("mock fresh = %+v", got)
	}
	bad := subscription(SourceReady)
	bad.CreditConfig.Config.PricingGroup = ""
	if got := preparation.PrepareStart(bad); got.Decision.Action != StartBlock || got.Failure == nil {
		t.Fatalf("missing group = %+v", got)
	}
	bad = subscription("UNKNOWN")
	if got := preparation.PrepareStart(bad); got.Decision.Action != StartBlock || got.Failure == nil {
		t.Fatalf("unknown authenticity = %+v", got)
	}

	empty := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	emptyReader, _ := NewReader(empty)
	emptyPreparation, _ := NewPricePreparationReader(emptyReader)
	byok := StartPreparationInput{Source: "domestic", BillingSource: modelbilling.BillingNewAPIByok, EvidenceID: "e", TaskID: "t", Scope: pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a").Scope, ModelID: "model-a", BYOKPricingGroup: "default", CapturedAt: cacheTestStart}
	if got := emptyPreparation.PrepareStart(byok); got.Decision.Action != StartAllowDegraded || got.Evidence == nil || got.Evidence.SnapshotID != "pricing-unavailable-v1:domestic" || !slices.Contains(got.Decision.Reasons, StartPricingUnavailable) || got.Failure.Reason != ReasonPricingUnavailable {
		t.Fatalf("BYOK unavailable = %+v", got)
	}
	if got := preparation.PrepareStart(byok); got.Decision.Action != StartAllow {
		t.Fatalf("BYOK fresh = %+v", got)
	}
	clock := &cacheTestClock{now: cacheTestStart}
	cache := newCacheTestCache(t, t.TempDir(), clock)
	mustRefreshCache(t, cache, cacheTestRequest("domestic", cacheTestPricingWithRequestModel()))
	staleReader, _ := NewReader(cache)
	stale, _ := NewPricePreparationReader(staleReader)
	clock.Set(cacheTestStart.Add(13 * time.Hour))
	if got := stale.PrepareStart(subscription(SourceReady)); got.Decision.Action != StartAllowDegraded || got.Decision.Reasons[0] != StartPricingStale {
		t.Fatalf("stale usable = %+v", got)
	}
	if got := stale.PrepareStart(byok); got.Decision.Action != StartAllowDegraded || got.Decision.Reasons[0] != StartPricingStale {
		t.Fatalf("BYOK stale usable = %+v", got)
	}
	if got := stale.PrepareStart(subscription(SourceMockOnly)); got.Decision.Action != StartAllowDegraded || !slices.Contains(got.Decision.Reasons, StartPricingStale) || !slices.Contains(got.Decision.Reasons, StartMockCreditConfig) {
		t.Fatalf("stale mock = %+v", got)
	}
	clock.Set(cacheTestStart.Add(25 * time.Hour))
	if got := stale.PrepareStart(subscription(SourceReady)); got.Decision.Action != StartBlock || got.Failure.Reason != ReasonStaleBlocked {
		t.Fatalf("stale blocked = %+v", got)
	}
}

func TestPrepareStartRejectsMixedInputsAndUsesPreciseFailureReasons(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	subscription := StartPreparationInput{Source: "domestic", BillingSource: modelbilling.BillingSubscriptionPool, EvidenceID: "e", TaskID: "t", Scope: pricePreparationTestInput(modelbilling.BillingSubscriptionPool, "model-a").Scope, ModelID: "model-a", CapturedAt: cacheTestStart, CreditConfig: &CreditConfigInput{Config: modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}, Authenticity: SourceReady}}
	byok := StartPreparationInput{Source: "domestic", BillingSource: modelbilling.BillingNewAPIByok, EvidenceID: "e", TaskID: "t", Scope: pricePreparationTestInput(modelbilling.BillingNewAPIByok, "model-a").Scope, ModelID: "model-a", BYOKPricingGroup: "default", CapturedAt: cacheTestStart}

	for _, tt := range []struct {
		name   string
		input  StartPreparationInput
		reason EstimateReason
	}{
		{name: "subscription BYOK group", input: func() StartPreparationInput { v := subscription; v.BYOKPricingGroup = "default"; return v }(), reason: ReasonPricingGroupInvalid},
		{name: "BYOK credit config", input: func() StartPreparationInput { v := byok; v.CreditConfig = subscription.CreditConfig; return v }(), reason: ReasonPricingContractAmbiguous},
		{name: "BYOK whitespace group", input: func() StartPreparationInput { v := byok; v.BYOKPricingGroup = " "; return v }(), reason: ReasonPricingGroupInvalid},
		{name: "model missing", input: func() StartPreparationInput { v := subscription; v.ModelID = ""; return v }(), reason: ReasonModelNotFound},
		{name: "model absent from snapshot", input: func() StartPreparationInput { v := subscription; v.ModelID = "other"; return v }(), reason: ReasonModelNotFound},
		{name: "billing source invalid", input: func() StartPreparationInput { v := subscription; v.BillingSource = "invalid"; return v }(), reason: ReasonBillingSourceInvalid},
		{name: "group absent from model", input: func() StartPreparationInput { v := byok; v.BYOKPricingGroup = "other"; return v }(), reason: ReasonPricingGroupInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := preparation.PrepareStart(tt.input)
			if got.Decision.Action != StartBlock || got.Failure == nil || got.Failure.Reason != tt.reason {
				t.Fatalf("PrepareStart = %+v, want reason %q", got, tt.reason)
			}
		})
	}

	empty := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	emptyReader, _ := NewReader(empty)
	emptyPreparation, _ := NewPricePreparationReader(emptyReader)
	if got := emptyPreparation.PrepareStart(subscription); got.Decision.Action != StartBlock || got.Failure == nil || got.Failure.Reason != ReasonSnapshotUnavailable {
		t.Fatalf("subscription no snapshot = %+v", got)
	}

	brokenEvidence := byok
	brokenEvidence.EvidenceID = ""
	if got := preparation.startReadError(brokenEvidence, "default", &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable, Message: "missing"}); got.Decision.Action != StartBlock || got.Failure == nil || got.Failure.Reason != ReasonEvidenceMismatch {
		t.Fatalf("unavailable evidence failure = %+v", got)
	}
}
