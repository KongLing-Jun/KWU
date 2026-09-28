package modelbilling

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestQuotaFixtureRejectsMissingPresenceWhitespaceAndImplicitRounding(t *testing.T) {
	tests := map[string][]byte{
		"missing nullable key": bytes.Replace(
			embeddedQuotaFixture, []byte(`          "quota_per_unit": "500000",`), nil, 1,
		),
		"noncanonical case name": bytes.Replace(
			embeddedQuotaFixture, []byte(`"name": "settled"`), []byte(`"name": " settled "`), 1,
		),
		"charge formula rounding": bytes.Replace(
			embeddedQuotaFixture, []byte(`"task_total_quota": "50"`), []byte(`"task_total_quota": "51"`), 1,
		),
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeQuotaFixture(bytes.NewReader(payload)); err == nil {
				t.Fatal("invalid quota fixture was accepted")
			}
		})
	}
}

func TestEmbeddedQuotaFixtureAndClientIdempotency(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled", SettlementCaseName: "settled",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := quotaReportBuilderInput(t, "task_fixture_quota")
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_01", "50", true, 5)}
	build, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.FinalizeQuota(context.Background(), bundle.Scope, *build.Final)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.FinalizeQuota(context.Background(), bundle.Scope, *build.Final)
	if err != nil {
		t.Fatal(err)
	}
	if first.Settlement.SettlementID != second.Settlement.SettlementID {
		t.Fatalf("exact retry changed settlement: %q != %q", first.Settlement.SettlementID, second.Settlement.SettlementID)
	}

	changed := quotaReportBuilderInput(t, input.Task.TaskID)
	changed.Calls = []ModelCall{quotaReportBuilderCall(t, changed.Task, "call_01", "51", true, 5)}
	changedBuild, err := buildFinalQuotaReport(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *changedBuild.Final); ErrorCodeOf(err) != ErrorQuotaFinalPayloadConflict {
		t.Fatalf("changed final error = %v", err)
	}
}

func TestEmbeddedQuotaFixtureSettledZeroAcceptsMultiCallFinal(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled_zero", SettlementCaseName: "settled_zero",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := quotaReportBuilderInput(t, "task_fixture_zero")
	input.Calls = []ModelCall{
		quotaReportBuilderCall(t, input.Task, "call_zero_01", "0", true, 5),
		quotaReportBuilderCall(t, input.Task, "call_zero_02", "0", true, 6),
	}
	build, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.FinalizeQuota(context.Background(), bundle.Scope, *build.Final)
	if err != nil {
		t.Fatal(err)
	}
	settlement := result.Settlement
	for name, value := range map[string]*DecimalString{
		"task total": settlement.TaskTotalQuota, "charged": settlement.ChargedCredits,
		"consumed": settlement.ConsumedFrozenAmount, "additional": settlement.AdditionalChargedAmount,
		"uncovered": settlement.UncoveredAmount,
	} {
		if value == nil || !decimalValuesEqual(*value, "0") {
			t.Fatalf("%s = %v", name, value)
		}
	}
	if settlement.ReleasedFrozenAmount == nil || !decimalValuesEqual(*settlement.ReleasedFrozenAmount, "300") ||
		settlement.Balance == nil || !decimalValuesEqual(*settlement.Balance, "21000") ||
		settlement.BalanceVersion == nil || *settlement.BalanceVersion != 14 {
		t.Fatalf("zero settlement = %+v", settlement)
	}
	if result.Balance == nil || result.Balance.TotalBalance != "21000.0000" ||
		result.Balance.FreeBalance != "1000.0000" || result.Balance.SubscriptionBalance != "20000.0000" ||
		result.Balance.FrozenBalance != "0.0000" || result.Balance.Version != 14 {
		t.Fatalf("zero settlement balance = %+v", result.Balance)
	}
}

func TestFixtureCloudQuotaClientRejectsCrossTaskRequestIDReuse(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled", SettlementCaseName: "settlement_pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	first := quotaReportBuilderInput(t, "task_request_owner")
	first.Calls = []ModelCall{quotaReportBuilderCall(t, first.Task, "call_01", "50", true, 5)}
	firstBuild, err := buildFinalQuotaReport(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *firstBuild.Final); err != nil {
		t.Fatal(err)
	}
	recovered, err := client.GetQuotaSettlement(context.Background(), bundle.Scope, first.Task.TaskID)
	if err != nil || recovered.Settlement.Status != QuotaSettlementSettled {
		t.Fatalf("terminal settlement regressed on query: %+v, %v", recovered, err)
	}

	second := quotaReportBuilderInput(t, "task_request_reuse")
	call := quotaReportBuilderCall(t, second.Task, "call_02", "50", true, 5)
	call.QuotaEvidence.OneAPIRequestID = cloneOptionalString(firstBuild.Final.Calls[0].OneAPIRequestID)
	second.Calls = []ModelCall{call}
	secondBuild, err := buildFinalQuotaReport(second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *secondBuild.Final); ErrorCodeOf(err) != ErrorQuotaRequestIDReused {
		t.Fatalf("request reuse error = %v", err)
	}
}

func TestFixtureCloudQuotaClientsShareAcceptedRequestAuthority(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	registry := &fixtureAcceptedRequestRegistry{owners: make(map[string]string)}
	newClient := func(fixture QuotaFixtureBundle, scope cloud.BindingScope) *MaterializedFixtureCloudQuotaClient {
		client, err := NewMaterializedFixtureCloudQuotaClientWithAcceptedRequests(
			fixture, scope,
			MaterializedQuotaFixtureSelection{FinalCaseName: "settled", SettlementCaseName: "settled"},
			registry,
		)
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	first := quotaReportBuilderInput(t, "task_shared_authority_first")
	first.Calls = []ModelCall{quotaReportBuilderCall(t, first.Task, "call_first", "50", true, 5)}
	firstBuild, err := buildFinalQuotaReport(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newClient(bundle, bundle.Scope).FinalizeQuota(context.Background(), bundle.Scope, *firstBuild.Final); err != nil {
		t.Fatal(err)
	}
	secondScope := bundle.Scope
	secondScope.ProfileID = "profile-other"
	secondScope.BindingGeneration++
	secondBundle := bundle
	secondBundle.Scope = secondScope
	second := quotaReportBuilderInput(t, "task_shared_authority_second")
	second.Task.Scope.ProfileID = secondScope.ProfileID
	second.Task.Scope.CloudBinding = &secondScope
	second.Task.Freeze.Scope = secondScope
	call := quotaReportBuilderCall(t, second.Task, "call_second", "50", true, 5)
	call.QuotaEvidence.OneAPIRequestID = cloneOptionalString(firstBuild.Final.Calls[0].OneAPIRequestID)
	second.Calls = []ModelCall{call}
	secondBuild, err := buildFinalQuotaReport(second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newClient(secondBundle, secondScope).FinalizeQuota(context.Background(), secondScope, *secondBuild.Final); ErrorCodeOf(err) != ErrorQuotaRequestIDReused {
		t.Fatalf("cross-client request reuse = %v", err)
	}
}

type fixtureAcceptedRequestRegistry struct {
	mu     sync.Mutex
	owners map[string]string
}

func (r *fixtureAcceptedRequestRegistry) AcceptQuotaRequestIDs(_ context.Context, cloudUserID, taskID string, requestIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, requestID := range requestIDs {
		key := cloudUserID + "\x00" + requestID
		if owner, found := r.owners[key]; found && owner != taskID {
			return newError(ErrorQuotaRequestIDReused, "oneapi request id was accepted for another task")
		}
		r.owners[key] = taskID
	}
	return nil
}

func TestFixtureCloudQuotaClientFoldsExactDuplicateFinalCalls(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled", SettlementCaseName: "settled",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := quotaReportBuilderInput(t, "task_duplicate_ingress")
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_duplicate", "50", true, 5)}
	build, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	duplicated := *build.Final
	duplicated.Calls = append(append([]QuotaReportCall(nil), build.Final.Calls...), build.Final.Calls[0])
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, duplicated); err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *build.Final); err != nil {
		t.Fatalf("canonical retry after duplicate folding = %v", err)
	}
}

func TestFixtureCloudQuotaClientRejectsSettlementTotalAndFreezeReuse(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled", SettlementCaseName: "settled",
	})
	if err != nil {
		t.Fatal(err)
	}
	wrong := quotaReportBuilderInput(t, "task_wrong_fixture_total")
	wrong.Calls = []ModelCall{quotaReportBuilderCall(t, wrong.Task, "call_wrong_total", "51", true, 5)}
	wrongBuild, err := buildFinalQuotaReport(wrong)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *wrongBuild.Final); ErrorCodeOf(err) != ErrorQuotaSummaryMismatch {
		t.Fatalf("fixture settlement total mismatch = %v", err)
	}

	first := quotaReportBuilderInput(t, "task_freeze_owner")
	first.Calls = []ModelCall{quotaReportBuilderCall(t, first.Task, "call_freeze_owner", "50", true, 5)}
	firstBuild, err := buildFinalQuotaReport(first)
	if err != nil {
		t.Fatal(err)
	}
	firstBuild.Final.FreezeID = wrongBuild.Final.FreezeID
	firstBuild.Final.IdempotencyKey, err = BuildFinalIdempotencyKey(bundle.Scope, first.Task.TaskID, firstBuild.Final.FreezeID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *firstBuild.Final); err != nil {
		t.Fatalf("failed final validation occupied freeze binding: %v", err)
	}
	second := quotaReportBuilderInput(t, "task_freeze_reuse")
	second.Calls = []ModelCall{quotaReportBuilderCall(t, second.Task, "call_freeze_reuse", "50", true, 5)}
	secondBuild, err := buildFinalQuotaReport(second)
	if err != nil {
		t.Fatal(err)
	}
	secondBuild.Final.FreezeID = firstBuild.Final.FreezeID
	secondBuild.Final.IdempotencyKey, err = BuildFinalIdempotencyKey(bundle.Scope, second.Task.TaskID, secondBuild.Final.FreezeID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *secondBuild.Final); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("cross-task freeze reuse = %v", err)
	}
}

func TestFixtureCloudQuotaClientRejectsReportScopeMismatch(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	newClient := func(t *testing.T) *MaterializedFixtureCloudQuotaClient {
		t.Helper()
		client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
			FinalCaseName: "settled", SettlementCaseName: "settled",
		})
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	wrongScope := bundle.Scope
	wrongScope.CloudUserID = "usr_other"

	t.Run("partial", func(t *testing.T) {
		client := newClient(t)
		input := quotaReportBuilderInput(t, "task_partial_wrong_report_scope")
		input.Task.State, input.Task.Result, input.Task.EndedAt = TaskRunning, nil, nil
		input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_partial_wrong_scope", "1", true, 4)}
		build, err := buildPartialQuotaReport(input, 1, input.Task.UpdatedAt.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		build.Partial.Scope = wrongScope
		build.Partial.IdempotencyKey, err = BuildPartialIdempotencyKey(
			wrongScope, build.Partial.TaskID, build.Partial.FreezeID, build.Partial.Sequence,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.UploadPartialQuota(context.Background(), bundle.Scope, *build.Partial); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("partial report scope error = %v", err)
		}
	})

	t.Run("final", func(t *testing.T) {
		client := newClient(t)
		input := quotaReportBuilderInput(t, "task_final_wrong_report_scope")
		input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_final_wrong_scope", "50", true, 5)}
		build, err := buildFinalQuotaReport(input)
		if err != nil {
			t.Fatal(err)
		}
		build.Final.Scope = wrongScope
		build.Final.IdempotencyKey, err = BuildFinalIdempotencyKey(wrongScope, build.Final.TaskID, build.Final.FreezeID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, *build.Final); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("final report scope error = %v", err)
		}
	})
}

func TestFixtureCloudQuotaClientBindsTaskAndFreezeBidirectionally(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	newClient := func(t *testing.T) *MaterializedFixtureCloudQuotaClient {
		t.Helper()
		client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
			FinalCaseName: "settled", SettlementCaseName: "settled",
		})
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	partial := func(t *testing.T, taskID string, sequence uint64) PartialQuotaReport {
		t.Helper()
		input := quotaReportBuilderInput(t, taskID)
		input.Task.State, input.Task.Result, input.Task.EndedAt = TaskRunning, nil, nil
		input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_"+taskID, "1", true, 4)}
		build, err := buildPartialQuotaReport(input, sequence, input.Task.UpdatedAt.Add(time.Duration(sequence)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		return *build.Partial
	}
	final := func(t *testing.T, taskID string) FinalQuotaReport {
		t.Helper()
		input := quotaReportBuilderInput(t, taskID)
		input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_"+taskID, "50", true, 5)}
		build, err := buildFinalQuotaReport(input)
		if err != nil {
			t.Fatal(err)
		}
		return *build.Final
	}

	t.Run("higher sequence partial cannot change freeze", func(t *testing.T) {
		client := newClient(t)
		first := partial(t, "task_partial_freeze_change", 1)
		if err := client.UploadPartialQuota(context.Background(), bundle.Scope, first); err != nil {
			t.Fatal(err)
		}
		second := partial(t, first.TaskID, 2)
		second.FreezeID += "_changed"
		second.IdempotencyKey, err = BuildPartialIdempotencyKey(bundle.Scope, second.TaskID, second.FreezeID, second.Sequence)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.UploadPartialQuota(context.Background(), bundle.Scope, second); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("higher partial freeze change error = %v", err)
		}
	})

	t.Run("final cannot change partial freeze", func(t *testing.T) {
		client := newClient(t)
		first := partial(t, "task_partial_final_freeze_change", 1)
		if err := client.UploadPartialQuota(context.Background(), bundle.Scope, first); err != nil {
			t.Fatal(err)
		}
		candidate := final(t, first.TaskID)
		candidate.FreezeID += "_changed"
		candidate.IdempotencyKey, err = BuildFinalIdempotencyKey(bundle.Scope, candidate.TaskID, candidate.FreezeID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, candidate); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("partial-to-final freeze change error = %v", err)
		}
	})

	t.Run("partial freeze cannot move to another task", func(t *testing.T) {
		client := newClient(t)
		first := partial(t, "task_partial_freeze_owner", 1)
		if err := client.UploadPartialQuota(context.Background(), bundle.Scope, first); err != nil {
			t.Fatal(err)
		}
		candidate := partial(t, "task_partial_freeze_reuse", 1)
		candidate.FreezeID = first.FreezeID
		candidate.IdempotencyKey, err = BuildPartialIdempotencyKey(bundle.Scope, candidate.TaskID, candidate.FreezeID, candidate.Sequence)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.UploadPartialQuota(context.Background(), bundle.Scope, candidate); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("cross-task partial freeze reuse error = %v", err)
		}
	})

	t.Run("final task cannot change freeze", func(t *testing.T) {
		client := newClient(t)
		first := final(t, "task_final_freeze_change")
		if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, first); err != nil {
			t.Fatal(err)
		}
		candidate := first
		candidate.FreezeID += "_changed"
		candidate.IdempotencyKey, err = BuildFinalIdempotencyKey(bundle.Scope, candidate.TaskID, candidate.FreezeID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.FinalizeQuota(context.Background(), bundle.Scope, candidate); ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("final freeze change error = %v", err)
		}
	})
}

func TestFixtureCloudQuotaClientFailedPartialValidationDoesNotBindFreeze(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled", SettlementCaseName: "settled",
	})
	if err != nil {
		t.Fatal(err)
	}
	invalidInput := quotaReportBuilderInput(t, "task_invalid_partial_binding")
	invalidInput.Task.State, invalidInput.Task.Result, invalidInput.Task.EndedAt = TaskRunning, nil, nil
	invalidInput.Calls = []ModelCall{quotaReportBuilderCall(t, invalidInput.Task, "call_invalid_partial", "1", true, 4)}
	invalidBuild, err := buildPartialQuotaReport(invalidInput, 1, invalidInput.Task.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	invalidBuild.Partial.Calls = nil
	if err := client.UploadPartialQuota(context.Background(), bundle.Scope, *invalidBuild.Partial); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("invalid partial error = %v", err)
	}

	validInput := quotaReportBuilderInput(t, "task_after_invalid_partial")
	validInput.Task.State, validInput.Task.Result, validInput.Task.EndedAt = TaskRunning, nil, nil
	validInput.Calls = []ModelCall{quotaReportBuilderCall(t, validInput.Task, "call_after_invalid_partial", "1", true, 4)}
	validBuild, err := buildPartialQuotaReport(validInput, 1, validInput.Task.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	validBuild.Partial.FreezeID = invalidBuild.Partial.FreezeID
	validBuild.Partial.IdempotencyKey, err = BuildPartialIdempotencyKey(
		bundle.Scope, validBuild.Partial.TaskID, validBuild.Partial.FreezeID, validBuild.Partial.Sequence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UploadPartialQuota(context.Background(), bundle.Scope, *validBuild.Partial); err != nil {
		t.Fatalf("failed partial validation occupied freeze binding: %v", err)
	}
}

func TestFixtureCloudQuotaClientRejectsNonCumulativePartial(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
		FinalCaseName: "settled", SettlementCaseName: "settled",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := quotaReportBuilderInput(t, "task_partial_cumulative")
	input.Task.State, input.Task.Result, input.Task.EndedAt = TaskRunning, nil, nil
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_kept", "1", true, 4)}
	first, err := buildPartialQuotaReport(input, 1, input.Task.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UploadPartialQuota(context.Background(), bundle.Scope, *first.Partial); err != nil {
		t.Fatal(err)
	}
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_new", "2", true, 6)}
	second, err := buildPartialQuotaReport(input, 2, input.Task.UpdatedAt.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UploadPartialQuota(context.Background(), bundle.Scope, *second.Partial); ErrorCodeOf(err) != ErrorQuotaPartialSnapshotConflict {
		t.Fatalf("non-cumulative partial = %v", err)
	}
}

func TestFixtureCloudQuotaClientCanRecoverSettlementAfterRecreation(t *testing.T) {
	bundle, err := LoadEmbeddedQuotaFixture()
	if err != nil {
		t.Fatal(err)
	}
	newClient := func() *MaterializedFixtureCloudQuotaClient {
		client, err := NewMaterializedFixtureCloudQuotaClient(bundle, bundle.Scope, MaterializedQuotaFixtureSelection{
			FinalCaseName: "settled", SettlementCaseName: "settled",
		})
		if err != nil {
			t.Fatal(err)
		}
		return client
	}
	first, err := newClient().GetQuotaSettlement(context.Background(), bundle.Scope, "task_restart")
	if err != nil {
		t.Fatal(err)
	}
	second, err := newClient().GetQuotaSettlement(context.Background(), bundle.Scope, "task_restart")
	if err != nil {
		t.Fatal(err)
	}
	if first.Settlement.SettlementID != second.Settlement.SettlementID {
		t.Fatal("fixture settlement identity is not deterministic across recreation")
	}
}
