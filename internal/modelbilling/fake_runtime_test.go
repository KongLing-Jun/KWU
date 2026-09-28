package modelbilling

import (
	"context"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestMaterializedFixtureCloudUsageClientStrictRuntimeIdentityAndIdempotency(t *testing.T) {
	usage, settlements := loadRuntimeUsageFixtures(t)
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_runtime", BindingGeneration: 7}
	client := newRuntimeUsageClient(t, usage, settlements, scope)

	partial := runtimePartialFixtureReport(t, usage, scope)
	if err := client.UploadPartial(context.Background(), scope, partial); err != nil {
		t.Fatalf("UploadPartial: %v", err)
	}
	if err := client.UploadPartial(context.Background(), scope, partial); err != nil {
		t.Fatalf("UploadPartial repeat: %v", err)
	}
	changed := partial
	changed.CreatedAt = changed.CreatedAt.Add(time.Second)
	if err := client.UploadPartial(context.Background(), scope, changed); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("changed canonical payload error = %v, want %s", err, ErrorIdempotencyConflict)
	}

	final := runtimeFinalFixtureReport(t, usage, scope)
	got, err := client.FinalizeUsage(context.Background(), scope, final)
	if err != nil {
		t.Fatalf("FinalizeUsage: %v", err)
	}
	if got.TaskID != final.TaskID || got.State != SettlementSettled {
		t.Fatalf("settlement = %+v", got)
	}
	repeat, err := client.FinalizeUsage(context.Background(), scope, final)
	if err != nil || repeat.SettlementID != got.SettlementID {
		t.Fatalf("FinalizeUsage repeat = %+v, %v", repeat, err)
	}

	changedFinal := final
	changedFinal.Calls = append([]ModelCallUsage(nil), final.Calls...)
	changedFinal.Calls[0].Usage.OutputTokens = runtimeInt64Pointer(5)
	if _, err := client.FinalizeUsage(context.Background(), scope, changedFinal); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("changed final error = %v, want %s", err, ErrorIdempotencyConflict)
	}
}

func TestMaterializedFixtureCloudUsageClientKeepsNormalizedSemanticsStrict(t *testing.T) {
	usage, settlements := loadRuntimeUsageFixtures(t)
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_runtime", BindingGeneration: 7}
	cases := []struct {
		name   string
		mutate func(*FinalUsageReport)
	}{
		{name: "model", mutate: func(report *FinalUsageReport) { report.Calls[0].ModelID = "another_model" }},
		{name: "pricing", mutate: func(report *FinalUsageReport) {
			value := "unexpected-pricing-version"
			report.Calls[0].PricingVersion = &value
		}},
		{name: "usage", mutate: func(report *FinalUsageReport) {
			report.Calls[0].Usage.OutputTokens = runtimeInt64Pointer(5)
		}},
		{name: "raw_usage", mutate: func(report *FinalUsageReport) {
			report.Calls[0].RawUsageRedacted = []byte(`{"input_tokens":10,"output_tokens":5,"total_tokens":15}`)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client := newRuntimeUsageClient(t, usage, settlements, scope)
			report := runtimeFinalFixtureReport(t, usage, scope)
			test.mutate(&report)
			if _, err := client.FinalizeUsage(context.Background(), scope, report); ErrorCodeOf(err) != ErrorIdempotencyConflict {
				t.Fatalf("changed normalized semantics error = %v, want %s", err, ErrorIdempotencyConflict)
			}
		})
	}

	client := newRuntimeUsageClient(t, usage, settlements, scope)
	report := runtimeFinalFixtureReport(t, usage, scope)
	wrongScope := scope
	wrongScope.BindingGeneration++
	if _, err := client.FinalizeUsage(context.Background(), wrongScope, report); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("wrong scope error = %v, want %s", err, ErrorScopeMismatch)
	}
}

func TestMaterializedFixtureCloudUsageClientRejectsFirstTimestampDrift(t *testing.T) {
	usage, settlements := loadRuntimeUsageFixtures(t)
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_runtime", BindingGeneration: 7}
	cases := []struct {
		name   string
		mutate func(*FinalUsageReport)
	}{
		{name: "created_at", mutate: func(report *FinalUsageReport) { report.CreatedAt = report.CreatedAt.Add(time.Second) }},
		{name: "summary_started_at", mutate: func(report *FinalUsageReport) {
			report.ClientSummary.StartedAt = report.ClientSummary.StartedAt.Add(time.Second)
		}},
		{name: "summary_finished_at", mutate: func(report *FinalUsageReport) {
			report.ClientSummary.FinishedAt = report.ClientSummary.FinishedAt.Add(time.Second)
		}},
		{name: "call_occurred_at", mutate: func(report *FinalUsageReport) {
			report.Calls[0].OccurredAt = report.Calls[0].OccurredAt.Add(time.Second)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client := newRuntimeUsageClient(t, usage, settlements, scope)
			report := runtimeFinalFixtureReport(t, usage, scope)
			test.mutate(&report)
			if _, err := client.FinalizeUsage(context.Background(), scope, report); ErrorCodeOf(err) != ErrorIdempotencyConflict {
				t.Fatalf("timestamp drift error = %v, want %s", err, ErrorIdempotencyConflict)
			}
		})
	}
}

func TestMaterializedFixtureSettlementIdentitySurvivesClientRestart(t *testing.T) {
	usage, settlements := loadRuntimeUsageFixtures(t)
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_runtime", BindingGeneration: 7}
	first := newRuntimeUsageClient(t, usage, settlements, scope)
	report := runtimeFinalFixtureReport(t, usage, scope)
	accepted, err := first.FinalizeUsage(context.Background(), scope, report)
	if err != nil {
		t.Fatal(err)
	}

	restarted := newRuntimeUsageClient(t, usage, settlements, scope)
	recovered, err := restarted.GetSettlement(context.Background(), scope, report.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.SettlementID != accepted.SettlementID || recovered.TaskID != accepted.TaskID {
		t.Fatalf("accepted/recovered identity = %+v/%+v", accepted, recovered)
	}
	if recovered.State != accepted.State || !recovered.ObservedAt.Equal(accepted.ObservedAt) {
		t.Fatalf("accepted/recovered state/time = %+v/%+v", accepted, recovered)
	}
	if !equalOptionalDecimal(recovered.ChargedAmount, accepted.ChargedAmount) ||
		!equalOptionalDecimal(recovered.UncoveredAmount, accepted.UncoveredAmount) {
		t.Fatalf("accepted/recovered common amounts = %+v/%+v", accepted, recovered)
	}
	if recovered.ChargedFreeAmount != nil || recovered.ChargedSubscriptionAmount != nil ||
		recovered.ReleasedFreeAmount != nil || recovered.ReleasedSubscriptionAmount != nil {
		t.Fatalf("query fixture optional split amounts = %+v, want nil", recovered)
	}
}

func TestMaterializedFixtureCloudUsageClientRejectsIncompatibleSettlementSelection(t *testing.T) {
	usage, settlements := loadRuntimeUsageFixtures(t)
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_runtime", BindingGeneration: 7}
	_, err := NewMaterializedFixtureCloudUsageClient(usage, settlements, scope, MaterializedUsageFixtureSelection{
		FixtureVersion: usage.Version, FinalCaseName: "final_settled", SettlementCaseName: "pricing_pending",
	})
	if ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("incompatible settlement error = %v, want %s", err, ErrorContractInvalid)
	}
}

func TestFixtureSettlementObserverValidatesTaskFreezeAndRecordsEvidence(t *testing.T) {
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_runtime", BindingGeneration: 7}
	settlementID, err := materializedSettlementID(scope, "task_runtime")
	if err != nil {
		t.Fatal(err)
	}
	resolveCalls := 0
	observer, err := NewFixtureSettlementObserver(scope, func(
		_ context.Context,
		gotScope cloud.BindingScope,
		taskID string,
	) (FixtureSettlementTaskIdentity, error) {
		resolveCalls++
		return FixtureSettlementTaskIdentity{Scope: gotScope, TaskID: taskID, FreezeID: "freeze_runtime"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := observer.CreditSettlementObserved(context.Background(), scope, "task_runtime", settlementID); err != nil {
		t.Fatal(err)
	}
	observations := observer.Observations()
	if resolveCalls != 1 || len(observations) != 1 || observations[0].FreezeID != "freeze_runtime" || observations[0].SettlementID != settlementID {
		t.Fatalf("calls/observations = %d/%+v", resolveCalls, observations)
	}
	observations[0].FreezeID = "mutated"
	if observer.Observations()[0].FreezeID != "freeze_runtime" {
		t.Fatal("observer exposed mutable evidence")
	}
	if err := observer.CreditSettlementObserved(context.Background(), scope, "task_runtime", "set_other"); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("wrong settlement error = %v, want %s", err, ErrorScopeMismatch)
	}
}

func loadRuntimeUsageFixtures(t *testing.T) (UsageFixtureBundle, SettlementFixtureBundle) {
	t.Helper()
	usage, err := LoadEmbeddedUsageFixture()
	if err != nil {
		t.Fatalf("LoadEmbeddedUsageFixture: %v", err)
	}
	settlements, err := LoadEmbeddedSettlementFixture()
	if err != nil {
		t.Fatalf("LoadEmbeddedSettlementFixture: %v", err)
	}
	return usage, settlements
}

func newRuntimeUsageClient(
	t *testing.T,
	usage UsageFixtureBundle,
	settlements SettlementFixtureBundle,
	scope cloud.BindingScope,
) *MaterializedFixtureCloudUsageClient {
	t.Helper()
	client, err := NewMaterializedFixtureCloudUsageClient(usage, settlements, scope, MaterializedUsageFixtureSelection{
		FixtureVersion: usage.Version, PartialCaseName: "partial_accepted", FinalCaseName: "final_settled", SettlementCaseName: "settled",
	})
	if err != nil {
		t.Fatalf("NewMaterializedFixtureCloudUsageClient: %v", err)
	}
	return client
}

func runtimePartialFixtureReport(t *testing.T, usage UsageFixtureBundle, scope cloud.BindingScope) PartialUsageReport {
	t.Helper()
	report, err := cloneFixture(*usage.Cases[0].Partial)
	if err != nil {
		t.Fatal(err)
	}
	report.TaskID = "task_runtime"
	report.Scope = scope
	report.FreezeID = "freeze_runtime"
	report.Calls[0].CallID = "call_runtime"
	report.Calls[0].PricingEvidenceID = "evidence_runtime"
	report.Calls[0].ProviderRequestID = stringPointer("provider_req_fixture")
	report.IdempotencyKey, err = BuildPartialIdempotencyKey(scope, report.TaskID, report.FreezeID, report.Sequence)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func runtimeFinalFixtureReport(t *testing.T, usage UsageFixtureBundle, scope cloud.BindingScope) FinalUsageReport {
	t.Helper()
	report, err := cloneFixture(*usage.Cases[2].Final)
	if err != nil {
		t.Fatal(err)
	}
	report.TaskID = "task_runtime"
	report.Scope = scope
	report.FreezeID = "freeze_runtime"
	report.Calls[0].CallID = "call_runtime"
	report.Calls[0].PricingEvidenceID = "evidence_runtime"
	report.Calls[0].ProviderRequestID = stringPointer("provider_req_fixture")
	report.IdempotencyKey, err = BuildFinalIdempotencyKey(scope, report.TaskID, report.FreezeID)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func runtimeInt64Pointer(value int64) *int64 { return &value }

func equalOptionalDecimal(left, right *DecimalString) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
