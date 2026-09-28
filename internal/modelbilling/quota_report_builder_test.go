package modelbilling

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestQuotaReportBuilderBuildsCanonicalFinalAndExactSummary(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_final")
	callB := quotaReportBuilderCall(t, input.Task, "call_b", "0.2", true, 6)
	callA := quotaReportBuilderCall(t, input.Task, "call_a", "0.1", true, 5)
	// These legacy facts are deliberately unusable. Quota settlement must not
	// inspect token usage or pricing evidence.
	callA.Usage = NormalizedUsage{BreakdownStatus: UsageExact}
	callA.PricingEvidenceID = ""
	input.Calls = []ModelCall{callB, callA}

	first, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Disposition != ReportBuildDeliverable || first.Final == nil || first.Partial != nil {
		t.Fatalf("result = %+v", first)
	}
	if !bytes.Equal(first.CanonicalPayload, second.CanonicalPayload) {
		t.Fatalf("final payload is not deterministic:\n%s\n%s", first.CanonicalPayload, second.CanonicalPayload)
	}
	final := first.Final
	if final.ContractVersion != QuotaContractV1 || len(final.Calls) != 2 ||
		final.Calls[0].CallID != "call_a" || final.Calls[1].CallID != "call_b" ||
		final.ClientSummary.TaskTotalQuota != "0.3" || final.ClientSummary.ModelCallCount != 2 {
		t.Fatalf("final = %+v", final)
	}
	for _, forbidden := range [][]byte{[]byte(`"usage"`), []byte(`"pricing"`), []byte(`"input_tokens"`)} {
		if bytes.Contains(first.CanonicalPayload, forbidden) {
			t.Fatalf("quota payload contains legacy field %s: %s", forbidden, first.CanonicalPayload)
		}
	}
	if err := final.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestQuotaReportBuilderAllowsIncompletePartialButDefersFinal(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_incomplete")
	input.Task.State = TaskRunning
	input.Task.Result = nil
	input.Task.EndedAt = nil
	input.Calls = []ModelCall{
		quotaReportBuilderCall(t, input.Task, "call_complete", "1.25", true, 4),
		quotaReportBuilderCall(t, input.Task, "call_missing", "", false, 7),
	}

	capturedAt := input.Task.UpdatedAt.Add(time.Minute)
	partial, err := buildPartialQuotaReport(input, 3, capturedAt)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Disposition != ReportBuildDeliverable || partial.Partial == nil || partial.Final != nil {
		t.Fatalf("partial = %+v", partial)
	}
	if partial.Partial.Sequence != 3 || len(partial.Partial.Calls) != 2 ||
		partial.Partial.Calls[1].EvidenceStatus != QuotaEvidenceMissing ||
		!partial.Partial.CapturedAt.Equal(capturedAt) {
		t.Fatalf("partial = %+v", partial.Partial)
	}
	if err := partial.Partial.Validate(); err != nil {
		t.Fatal(err)
	}

	endedAt := input.Task.CreatedAt.Add(12 * time.Minute)
	result := TaskResultCompleted
	input.Task.State = TaskUsagePending
	input.Task.Result = &result
	input.Task.EndedAt = &endedAt
	final, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if final.Disposition != ReportBuildEvidencePending || final.Final != nil || final.Partial != nil || len(final.CanonicalPayload) != 0 {
		t.Fatalf("final = %+v", final)
	}
}

func TestQuotaReportBuilderRequestsReleaseWhenEveryFailedCallLacksAcceptanceEvidence(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_rejected_before_acceptance")
	failed := quotaReportBuilderCall(t, input.Task, "call_rejected", "", false, 4)
	failed.State = CallFailed
	failed.ProviderRequestID = nil
	failed.RawUsageRedacted = nil
	failed.Usage = NormalizedUsage{BreakdownStatus: UsageUnavailable}
	code := string(ErrorGatewayUnavailable)
	failed.LastErrorCode = &code
	result := TaskResultFailed
	input.Task.Result = &result
	input.Calls = []ModelCall{failed}

	built, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if built.Disposition != ReportBuildNoReportRequired || built.Final != nil ||
		built.Partial != nil || len(built.CanonicalPayload) != 0 {
		t.Fatalf("pre-acceptance failure result = %+v", built)
	}
}

func TestQuotaReportBuilderPartialSkipsPreparedAndRunningCalls(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_in_flight")
	input.Task.State, input.Task.Result, input.Task.EndedAt = TaskRunning, nil, nil
	completed := quotaReportBuilderCall(t, input.Task, "call_completed", "1.25", true, 4)
	prepared := ModelCall{
		CallID: "call_prepared", TaskID: input.Task.TaskID, Scope: input.Task.Scope,
		BillingSource: BillingSubscriptionPool, State: CallPrepared, AttemptCount: 3,
	}
	running := quotaReportBuilderCall(t, input.Task, "call_running", "", false, 7)
	running.State = CallRunning
	running.FinishedAt = nil
	running.OccurredAt = nil
	running.QuotaEvidence = nil
	input.Calls = []ModelCall{completed, prepared, running}

	built, err := buildPartialQuotaReport(input, 1, input.Task.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if built.Disposition != ReportBuildDeliverable || built.Partial == nil || len(built.Partial.Calls) != 1 ||
		built.Partial.Calls[0].CallID != completed.CallID {
		t.Fatalf("partial with in-flight call = %+v", built)
	}
}

func TestQuotaReportBuilderUsesExplicitCaptureAfterSupplementObservation(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_supplement_capture")
	input.Task.State, input.Task.Result, input.Task.EndedAt = TaskRunning, nil, nil
	call := quotaReportBuilderCall(t, input.Task, "call_supplemented", "1", true, 4)
	observedAt := call.FinishedAt.Add(3 * time.Minute)
	call.QuotaEvidence.Revision = 2
	call.QuotaEvidence.ObservedAt = observedAt
	input.Calls = []ModelCall{call}

	capturedAt := input.Task.UpdatedAt.Add(time.Minute)
	built, err := buildPartialQuotaReport(input, 1, capturedAt)
	if err != nil {
		t.Fatal(err)
	}
	if built.Partial == nil || !built.Partial.CapturedAt.Equal(capturedAt) {
		t.Fatalf("partial captured_at = %+v, want %s", built.Partial, capturedAt)
	}
}

func TestQuotaReportBuilderSuppressesZeroActualCallsAndHasNoCallLimitGate(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_zero")
	input.Calls = nil
	zero, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if zero.Disposition != ReportBuildNoReportRequired || zero.Final != nil || len(zero.CanonicalPayload) != 0 {
		t.Fatalf("zero-call result = %+v", zero)
	}

	input.Calls = make([]ModelCall, 257)
	for i := range input.Calls {
		input.Calls[i] = quotaReportBuilderCall(t, input.Task, fmt.Sprintf("call_%03d", i), "1", true, 5)
	}
	large, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if large.Disposition != ReportBuildDeliverable || large.Final == nil || len(large.Final.Calls) != 257 ||
		large.Final.ClientSummary.TaskTotalQuota != "257" {
		t.Fatalf("large result = %+v", large)
	}
}

func TestQuotaReportBuilderFoldsExactDuplicatesAndRejectsFingerprintConflicts(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_conflict")
	call := quotaReportBuilderCall(t, input.Task, "call_01", "2", true, 5)
	input.Calls = []ModelCall{call, call}

	folded, err := buildFinalQuotaReport(input)
	if err != nil {
		t.Fatal(err)
	}
	if folded.Final == nil || len(folded.Final.Calls) != 1 || folded.Final.ClientSummary.TaskTotalQuota != "2" {
		t.Fatalf("folded = %+v", folded)
	}

	changed := call
	changed.QuotaEvidence = quotaReportBuilderEvidence(t, "request_call_01", "3", changed.FinishedAt.UTC())
	input.Calls = []ModelCall{call, changed}
	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorQuotaCallIDConflict {
		t.Fatalf("call conflict error = %v", err)
	}

	other := quotaReportBuilderCall(t, input.Task, "call_02", "4", true, 6)
	other.QuotaEvidence = quotaReportBuilderEvidence(t, "request_call_01", "4", other.FinishedAt.UTC())
	input.Calls = []ModelCall{call, other}
	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorQuotaRequestIDConflict {
		t.Fatalf("request conflict error = %v", err)
	}
}

func TestQuotaReportBuilderRejectsBYOKAndInvalidActualLifecycle(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_invalid")
	call := quotaReportBuilderCall(t, input.Task, "call_01", "1", true, 5)
	call.State = CallRunning
	input.Calls = []ModelCall{call}
	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("running lifecycle error = %v", err)
	}

	prepared := ModelCall{
		CallID: "call_prepared", TaskID: input.Task.TaskID, Scope: input.Task.Scope,
		BillingSource: BillingSubscriptionPool, State: CallPrepared,
	}
	input.Calls = []ModelCall{prepared}
	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("prepared lifecycle error = %v", err)
	}

	input = quotaReportBuilderInput(t, "task_quota_byok")
	input.Task.BillingSource = BillingNewAPIByok
	input.Task.Scope = TaskScope{ActorID: "local", ProfileID: "default"}
	input.Task.Freeze = nil
	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorBillingSourceMismatch {
		t.Fatalf("BYOK error = %v", err)
	}
}

func TestQuotaReportBuilderRequiresProvenTaskStart(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_missing_start")
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_01", "1", true, 5)}
	input.Task.StartedAt = nil

	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
		t.Fatalf("missing task started_at error = %v", err)
	}
}

func TestQuotaReportBuilderRejectsCallBeforeTaskStart(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_quota_call_before_start")
	input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_01", "1", true, 0)}

	if _, err := buildFinalQuotaReport(input); ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
		t.Fatalf("call before task start error = %v", err)
	}
}

func TestQuotaReportBuilderRejectsCaptureBeforePersistedFacts(t *testing.T) {
	newRunningInput := func() SubscriptionQuotaInput {
		input := quotaReportBuilderInput(t, "task_quota_capture_boundary")
		input.Task.State, input.Task.Result, input.Task.EndedAt = TaskRunning, nil, nil
		input.Task.UpdatedAt = input.Task.CreatedAt.Add(2 * time.Minute)
		input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_01", "1", true, 4)}
		return input
	}

	t.Run("task updated_at", func(t *testing.T) {
		input := newRunningInput()
		capturedAt := input.Task.UpdatedAt.Add(-time.Nanosecond)
		if _, err := buildPartialQuotaReport(input, 1, capturedAt); ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
			t.Fatalf("capture before task update error = %v", err)
		}
	})
	t.Run("call finished_at", func(t *testing.T) {
		input := newRunningInput()
		capturedAt := input.Calls[0].FinishedAt.Add(-time.Nanosecond)
		if _, err := buildPartialQuotaReport(input, 1, capturedAt); ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
			t.Fatalf("capture before call finish error = %v", err)
		}
	})
	t.Run("evidence observed_at", func(t *testing.T) {
		input := newRunningInput()
		observedAt := input.Calls[0].FinishedAt.Add(time.Minute)
		input.Calls[0].QuotaEvidence.ObservedAt = observedAt
		capturedAt := observedAt.Add(-time.Nanosecond)
		if _, err := buildPartialQuotaReport(input, 1, capturedAt); ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
			t.Fatalf("capture before evidence observation error = %v", err)
		}
	})
}

func quotaReportBuilderInput(t *testing.T, taskID string) SubscriptionQuotaInput {
	t.Helper()
	createdAt := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	endedAt := createdAt.Add(12 * time.Minute)
	startedAt := createdAt.Add(time.Minute)
	result := TaskResultCompleted
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_fixture", BindingGeneration: 1}
	scope := TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}
	task := ModelTask{
		TaskID: taskID, Scope: scope, BillingSource: BillingSubscriptionPool,
		InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0",
		State: TaskUsagePending, Result: &result, CreatedAt: createdAt, StartedAt: &startedAt,
		UpdatedAt: endedAt, EndedAt: &endedAt,
	}
	task.Freeze = &cloud.Freeze{
		Scope: binding, FreezeID: "freeze_" + taskID, TaskID: taskID, Status: cloud.FreezeActive,
		FreezeAmount: "0", FreeFrozenAmount: "0", SubscriptionFrozenAmount: "0",
		ExpiresAt: createdAt.Add(time.Hour),
	}
	return SubscriptionQuotaInput{Task: task}
}

func quotaReportBuilderCall(t *testing.T, task ModelTask, callID, quota string, complete bool, minute int) ModelCall {
	t.Helper()
	startedAt := task.CreatedAt.Add(time.Duration(minute) * time.Minute)
	finishedAt := startedAt.Add(time.Minute)
	occurredAt := startedAt.UTC()
	requestID := "request_" + callID
	var evidence *QuotaEvidenceRevision
	if complete {
		evidence = quotaReportBuilderEvidence(t, requestID, quota, finishedAt.UTC())
	} else {
		value, err := NormalizeQuotaEvidence(QuotaObservationInput{
			Endpoint: MaasEndpointChatCompletions, FinalJSON: []byte(`{}`), ObservedAt: finishedAt.UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		evidence = &value
	}
	return ModelCall{
		CallID: callID, TaskID: task.TaskID, Scope: task.Scope, BillingSource: BillingSubscriptionPool,
		ModelID: task.InitialModelID, Endpoint: MaasEndpointChatCompletions,
		State: CallCompleted, AttemptCount: 1, StartedAt: &startedAt, FinishedAt: &finishedAt,
		OccurredAt: &occurredAt, QuotaEvidence: evidence, QuotaReportStatus: QuotaReportUnreported,
	}
}

func quotaReportBuilderEvidence(t *testing.T, requestID, quota string, observedAt time.Time) *QuotaEvidenceRevision {
	t.Helper()
	evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:   MaasEndpointChatCompletions,
		Headers:    map[string][]string{"X-Oneapi-Request-Id": {requestID}},
		FinalJSON:  []byte(fmt.Sprintf(`{"billing":{"quota":%s}}`, quota)),
		ObservedAt: observedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &evidence
}
