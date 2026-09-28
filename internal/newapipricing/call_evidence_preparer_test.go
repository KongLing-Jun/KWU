package newapipricing

import (
	"context"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

var callEvidenceCapturedAt = cacheTestStart.Add(5 * time.Minute)

type callEvidenceCreditConfigProvider struct {
	config modelbilling.CreditConfig
	err    error
	calls  int
	scope  cloud.BindingScope
}

func (p *callEvidenceCreditConfigProvider) GetCreditConfig(_ context.Context, scope cloud.BindingScope) (modelbilling.CreditConfig, error) {
	p.calls++
	p.scope = scope
	return p.config, p.err
}

func TestCallEvidencePreparerSubscriptionPreparesDifferentModelFromFrozenFacts(t *testing.T) {
	reader, transportCalls, statusCalls := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	version := "credit-v1"
	input := callEvidencePreparationInput(t, modelbilling.BillingSubscriptionPool, "model-b", "default", &version)
	creditConfig := &callEvidenceCreditConfigProvider{config: modelbilling.CreditConfig{
		CreditPerCNY: "2000", PricingGroup: "default", Version: &version, FetchedAt: cacheTestStart,
	}}
	beforeTransport, beforeStatus := transportCalls.Load(), statusCalls.Load()
	preparer := CallEvidencePreparer{
		Preparation: preparation, Source: "domestic", CreditConfig: creditConfig,
		CreditConfigAuthenticity: SourceMockOnly, Now: func() time.Time { return callEvidenceCapturedAt },
	}

	evidence, err := preparer.PrepareForCall(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.EvidenceID != input.EvidenceID || evidence.TaskID != input.Task.TaskID ||
		evidence.ModelID != input.ModelID || !evidence.Scope.Equal(input.Task.Scope) ||
		evidence.PricingGroup != input.InitialEvidence.PricingGroup || !evidence.CapturedAt.Equal(callEvidenceCapturedAt) {
		t.Fatalf("prepared evidence = %+v", evidence)
	}
	if creditConfig.calls != 1 || creditConfig.scope != *input.Task.Scope.CloudBinding {
		t.Fatalf("credit config calls/scope = %d/%+v", creditConfig.calls, creditConfig.scope)
	}
	if transportCalls.Load() != beforeTransport || statusCalls.Load() != beforeStatus {
		t.Fatalf("PrepareForCall performed a refresh: transport/status = %d/%d", transportCalls.Load(), statusCalls.Load())
	}
}

func TestCallEvidencePreparerRejectsSubscriptionCreditConfigDrift(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	frozenVersion := "credit-v1"
	for _, test := range []struct {
		name    string
		group   string
		version *string
	}{
		{name: "group", group: "other", version: &frozenVersion},
		{name: "version", group: "default", version: stringPointerForCallEvidence("credit-v2")},
		{name: "missing version", group: "default", version: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := callEvidencePreparationInput(t, modelbilling.BillingSubscriptionPool, "model-b", "default", &frozenVersion)
			creditConfig := &callEvidenceCreditConfigProvider{config: modelbilling.CreditConfig{
				CreditPerCNY: "2000", PricingGroup: test.group, Version: test.version, FetchedAt: cacheTestStart,
			}}
			preparer := CallEvidencePreparer{
				Preparation: preparation, Source: "domestic", CreditConfig: creditConfig,
				CreditConfigAuthenticity: SourceReady, Now: func() time.Time { return callEvidenceCapturedAt },
			}

			if _, err := preparer.PrepareForCall(context.Background(), input); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
				t.Fatalf("PrepareForCall error = %v, want contract_invalid", err)
			}
		})
	}
}

func TestCallEvidencePreparerBYOKPreservesGroupUnavailableSentinel(t *testing.T) {
	reader, _, _ := newPricePreparationTestReader(t)
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	input := callEvidencePreparationInput(t, modelbilling.BillingNewAPIByok, "model-b", "__kwu_unavailable__", nil)
	preparer := CallEvidencePreparer{
		Preparation: preparation, Source: "domestic", Now: func() time.Time { return callEvidenceCapturedAt },
	}

	evidence, err := preparer.PrepareForCall(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.EvidenceID != input.EvidenceID || evidence.TaskID != input.Task.TaskID ||
		evidence.ModelID != input.ModelID || evidence.PricingGroup != "__kwu_unavailable__" ||
		evidence.SnapshotID != "pricing-unavailable-v2:domestic:group_unavailable" ||
		!evidence.CapturedAt.Equal(callEvidenceCapturedAt) {
		t.Fatalf("BYOK group-unavailable evidence = %+v", evidence)
	}
}

func TestCallEvidencePreparerPreservesBlockedPrepareStartCode(t *testing.T) {
	cache := newCacheTestCache(t, t.TempDir(), &cacheTestClock{now: cacheTestStart})
	reader, err := NewReader(cache)
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := NewPricePreparationReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	version := "credit-v1"
	input := callEvidencePreparationInput(t, modelbilling.BillingSubscriptionPool, "model-b", "default", &version)
	creditConfig := &callEvidenceCreditConfigProvider{config: modelbilling.CreditConfig{
		CreditPerCNY: "2000", PricingGroup: "default", Version: &version, FetchedAt: cacheTestStart,
	}}
	preparer := CallEvidencePreparer{
		Preparation: preparation, Source: "domestic", CreditConfig: creditConfig,
		CreditConfigAuthenticity: SourceReady, Now: func() time.Time { return callEvidenceCapturedAt },
	}

	if _, err := preparer.PrepareForCall(context.Background(), input); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable {
		t.Fatalf("PrepareForCall error = %v, want pricing_unavailable", err)
	}
}

func callEvidencePreparationInput(t *testing.T, source modelbilling.BillingSource, modelID, group string, version *string) modelbilling.CallEvidencePreparationInput {
	t.Helper()
	scope := modelbilling.TaskScope{ActorID: "actor", ProfileID: "default"}
	if source == modelbilling.BillingSubscriptionPool {
		binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "user_01", BindingGeneration: 1}
		scope.CloudBinding = &binding
	}
	task, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: "task_01", Scope: scope, BillingSource: source,
		InitialModelID: "model-a", PricingEvidenceID: "evidence_initial",
	})
	if err != nil {
		t.Fatal(err)
	}
	initial := modelbilling.PricingEvidence{
		EvidenceID: "evidence_initial", SnapshotID: "initial-pricing", Scope: scope,
		TaskID: task.TaskID, ModelID: task.InitialModelID, PricingGroup: group,
		PricingVersion: version, State: modelbilling.PricingFresh, CapturedAt: cacheTestStart,
	}
	return modelbilling.CallEvidencePreparationInput{
		Task: task, InitialEvidence: initial, EvidenceID: "evidence_derived", ModelID: modelID,
	}
}

func stringPointerForCallEvidence(value string) *string { return &value }
