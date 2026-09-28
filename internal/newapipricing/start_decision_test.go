package newapipricing

import (
	"slices"
	"testing"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

func TestDecideModelStartAllowsReliableSubscription(t *testing.T) {
	cached := estimateTestSnapshot()
	cached.Provenance.Status.Authenticity = SourceReady
	config := modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}
	estimate := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage(),
		CreditConfig: &CreditConfigInput{Config: config, Authenticity: SourceReady},
	})
	evidence := startTestEvidence(t, cached, modelbilling.BillingSubscriptionPool)
	decision := DecideModelStart(StartDecisionInput{
		BillingSource: modelbilling.BillingSubscriptionPool, Estimate: estimate,
		TaskID: evidence.TaskID, Scope: evidence.Scope, Evidence: &evidence, RequireCredits: true,
	})
	if decision.Action != StartAllow || decision.BillingSource != modelbilling.BillingSubscriptionPool {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestDecideModelStartDegradesStaleOrMockSubscription(t *testing.T) {
	for _, mutate := range []func(*CachedSnapshot){
		func(cached *CachedSnapshot) { cached.Snapshot.State = modelbilling.PricingStaleUsable },
		func(cached *CachedSnapshot) {},
	} {
		cached := estimateTestSnapshot()
		mutate(&cached)
		config := modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}
		estimate := Estimate(EstimateInput{
			CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage(),
			CreditConfig: &CreditConfigInput{Config: config, Authenticity: SourceMockOnly},
		})
		evidence := startTestEvidence(t, cached, modelbilling.BillingSubscriptionPool)
		decision := DecideModelStart(StartDecisionInput{
			BillingSource: modelbilling.BillingSubscriptionPool, Estimate: estimate,
			TaskID: evidence.TaskID, Scope: evidence.Scope, Evidence: &evidence, RequireCredits: true,
		})
		if decision.Action != StartAllowDegraded {
			t.Fatalf("decision = %+v", decision)
		}
	}
}

func TestDecideModelStartBlocksUnreliableSubscription(t *testing.T) {
	cached := estimateTestSnapshot()
	partial := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default",
		Usage: modelbilling.NormalizedUsage{NormalInputTokens: ptrInt64(1), BreakdownStatus: modelbilling.UsagePartial},
	})
	evidence := startTestEvidence(t, cached, modelbilling.BillingSubscriptionPool)
	tests := []struct {
		name   string
		input  StartDecisionInput
		reason StartReason
	}{
		{name: "partial", input: StartDecisionInput{BillingSource: modelbilling.BillingSubscriptionPool, TaskID: evidence.TaskID, Scope: evidence.Scope, Estimate: partial, Evidence: &evidence}, reason: StartEstimateIncomplete},
		{name: "missing evidence", input: StartDecisionInput{BillingSource: modelbilling.BillingSubscriptionPool, Estimate: partial}, reason: StartEvidenceMissing},
	}
	completeWithoutCredits := Estimate(EstimateInput{CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage()})
	tests = append(tests, struct {
		name   string
		input  StartDecisionInput
		reason StartReason
	}{name: "credits", input: StartDecisionInput{BillingSource: modelbilling.BillingSubscriptionPool, TaskID: evidence.TaskID, Scope: evidence.Scope, Estimate: completeWithoutCredits, Evidence: &evidence, RequireCredits: true}, reason: StartCreditsUnavailable})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := DecideModelStart(test.input)
			if decision.Action != StartBlock || !slices.Contains(decision.Reasons, test.reason) {
				t.Fatalf("decision = %+v", decision)
			}
		})
	}
}

func TestDecideModelStartRejectsCrossTaskOrScopeEvidence(t *testing.T) {
	cached := estimateTestSnapshot()
	evidence := startTestEvidence(t, cached, modelbilling.BillingSubscriptionPool)
	estimate := Estimate(EstimateInput{CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage()})
	decision := DecideModelStart(StartDecisionInput{
		BillingSource: modelbilling.BillingSubscriptionPool, TaskID: "other_task",
		Scope: evidence.Scope, Estimate: estimate, Evidence: &evidence,
	})
	if decision.Action != StartBlock || !slices.Contains(decision.Reasons, StartEvidenceMismatch) {
		t.Fatalf("cross-task decision = %+v", decision)
	}
}

func TestDecideModelStartAllowsBYOKWithRealEvidenceWhenUsageUnavailable(t *testing.T) {
	cached := estimateTestSnapshot()
	estimate := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default",
		Usage: modelbilling.NormalizedUsage{BreakdownStatus: modelbilling.UsageMissing},
	})
	evidence := startTestEvidence(t, cached, modelbilling.BillingNewAPIByok)
	decision := DecideModelStart(StartDecisionInput{
		BillingSource: modelbilling.BillingNewAPIByok, TaskID: evidence.TaskID,
		Scope: evidence.Scope, Estimate: estimate, Evidence: &evidence,
	})
	if decision.Action != StartAllowDegraded || decision.BillingSource != modelbilling.BillingNewAPIByok ||
		!slices.Contains(decision.Reasons, StartEstimateUnavailable) {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestDecideModelStartAllowsBYOKWithMatchingExactEvidence(t *testing.T) {
	cached := estimateTestSnapshot()
	cached.Provenance.Status.Authenticity = SourceReady
	config := modelbilling.CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", FetchedAt: cacheTestStart}
	estimate := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage(),
		CreditConfig: &CreditConfigInput{Config: config, Authenticity: SourceReady},
	})
	evidence := startTestEvidence(t, cached, modelbilling.BillingNewAPIByok)
	decision := DecideModelStart(StartDecisionInput{
		BillingSource: modelbilling.BillingNewAPIByok, TaskID: evidence.TaskID,
		Scope: evidence.Scope, Estimate: estimate, Evidence: &evidence,
	})
	if estimate.State != EstimateExact || decision.Action != StartAllow || decision.BillingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("estimate=%+v decision=%+v", estimate, decision)
	}
}

func TestDecideModelStartBlocksBYOKWithoutMatchingEvidence(t *testing.T) {
	cached := estimateTestSnapshot()
	estimate := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage(),
	})
	evidence := startTestEvidence(t, cached, modelbilling.BillingNewAPIByok)
	mismatched := evidence
	mismatched.ModelID = "model-b"
	tests := []struct {
		name     string
		evidence *modelbilling.PricingEvidence
		reason   StartReason
	}{
		{name: "missing", reason: StartEvidenceMissing},
		{name: "mismatch", evidence: &mismatched, reason: StartEvidenceMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := DecideModelStart(StartDecisionInput{
				BillingSource: modelbilling.BillingNewAPIByok, TaskID: evidence.TaskID,
				Scope: evidence.Scope, Estimate: estimate, Evidence: test.evidence,
			})
			if decision.Action != StartBlock || !slices.Contains(decision.Reasons, test.reason) {
				t.Fatalf("decision = %+v", decision)
			}
		})
	}
}

func TestDecideModelStartAllowsBYOKWithUnavailableEvidence(t *testing.T) {
	readErr := &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable}
	request := PricingEvidenceRequest{
		EvidenceID: "evidence_unavailable", TaskID: "task_01",
		Scope:         modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"},
		BillingSource: modelbilling.BillingNewAPIByok,
		ModelID:       "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	}
	evidence, err := BuildUnavailablePricingEvidence("domestic", readErr, request)
	if err != nil {
		t.Fatal(err)
	}
	estimate := EstimateResult{
		State: EstimateUnavailable, SnapshotID: evidence.SnapshotID, ModelID: evidence.ModelID,
		PricingGroup: evidence.PricingGroup, PricingState: modelbilling.PricingUnavailable,
		Reasons: []EstimateReason{ReasonSnapshotUnavailable},
	}
	decision := DecideModelStart(StartDecisionInput{
		BillingSource: modelbilling.BillingNewAPIByok, TaskID: evidence.TaskID,
		Scope: evidence.Scope, Estimate: estimate, Evidence: &evidence,
	})
	if decision.Action != StartAllowDegraded || decision.BillingSource != modelbilling.BillingNewAPIByok {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestDecideModelStartBlocksStaleBlockedSubscriptionAsPricingBlocked(t *testing.T) {
	cached := estimateTestSnapshot()
	cached.Snapshot.State = modelbilling.PricingStaleBlocked
	estimate := Estimate(EstimateInput{
		CachedSnapshot: cached, ModelID: "model-a", PricingGroup: "default", Usage: exactZeroUsage(),
	})
	evidence := startTestEvidence(t, cached, modelbilling.BillingSubscriptionPool)
	decision := DecideModelStart(StartDecisionInput{
		BillingSource: modelbilling.BillingSubscriptionPool, TaskID: evidence.TaskID,
		Scope: evidence.Scope, Estimate: estimate, Evidence: &evidence, RequireCredits: true,
	})
	if decision.Action != StartBlock || len(decision.Reasons) != 1 || decision.Reasons[0] != StartPricingBlocked {
		t.Fatalf("decision = %+v", decision)
	}
}

func startTestEvidence(t *testing.T, cached CachedSnapshot, source modelbilling.BillingSource) modelbilling.PricingEvidence {
	t.Helper()
	scope := modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"}
	if source == modelbilling.BillingSubscriptionPool {
		binding := testBindingScopeForStart()
		scope.CloudBinding = &binding
	}
	evidence, err := BuildPricingEvidence(cached, PricingEvidenceRequest{
		EvidenceID: "evidence_01", TaskID: "task_01", Scope: scope, BillingSource: source,
		ModelID: "model-a", PricingGroup: "default", CapturedAt: cacheTestStart,
	})
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func testBindingScopeForStart() cloud.BindingScope {
	return cloud.BindingScope{ProfileID: "default", CloudUserID: "user_01", BindingGeneration: 1}
}
