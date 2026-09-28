package modelbilling

import (
	"context"
	"testing"
	"time"
)

func TestQuotaReportSettlesMeteredCallsAfterUnmeteredFailure(t *testing.T) {
	for _, state := range []CallState{CallFailed, CallInterrupted, CallCancelled} {
		t.Run(string(state), func(t *testing.T) {
			input := quotaReportBuilderInput(t, "task_mixed_failure")
			failed := quotaReportBuilderCall(t, input.Task, "call_failed", "", false, 5)
			failed.State = state
			failed.Usage = NormalizedUsage{BreakdownStatus: UsageUnavailable}
			input.Calls = []ModelCall{
				quotaReportBuilderCall(t, input.Task, "call_before", "1.25", true, 3),
				failed,
				quotaReportBuilderCall(t, input.Task, "call_retry", "2.5", true, 7),
			}
			final, err := buildFinalQuotaReport(input)
			if err != nil || final.Disposition != ReportBuildDeliverable || final.Final == nil {
				t.Fatalf("final = %+v, err=%v", final, err)
			}
			if final.Final.ClientSummary.TaskTotalQuota != "3.75" || final.Final.ClientSummary.ModelCallCount != 2 {
				t.Fatalf("summary = %+v", final.Final.ClientSummary)
			}
			partial, err := buildPartialQuotaReport(input, 1, input.Task.UpdatedAt.Add(time.Minute))
			if err != nil || partial.Partial == nil || len(partial.Partial.Calls) != 2 {
				t.Fatalf("partial = %+v, err=%v", partial, err)
			}
			for i, call := range final.Final.Calls {
				if call.CallID == failed.CallID || call.CallID != partial.Partial.Calls[i].CallID {
					t.Fatalf("partial/final call mismatch: %+v / %+v", partial.Partial.Calls, final.Final.Calls)
				}
			}
		})
	}
}

func TestQuotaReportStillWaitsForObservedBillingEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*ModelCall)
	}{
		{"completed", func(call *ModelCall) { call.State = CallCompleted }},
		{"usage missing", func(call *ModelCall) { call.State = CallUsageMissing }},
		{"provider request", func(call *ModelCall) { id := "request_observed"; call.ProviderRequestID = &id }},
		{"token usage", func(call *ModelCall) { tokens := int64(1); call.Usage.OutputTokens = &tokens }},
		{"raw usage", func(call *ModelCall) { call.RawUsageRedacted = []byte(`{"output_tokens":1}`) }},
		{"quota request", func(call *ModelCall) {
			id := "request_observed"
			value := missingQuotaEvidence(&id, *call.FinishedAt)
			call.QuotaEvidence = &value
		}},
		{"billing quota", func(call *ModelCall) {
			value, err := NormalizeQuotaEvidence(QuotaObservationInput{
				Endpoint: call.Endpoint, FinalJSON: []byte(`{"billing":{"quota":1}}`), ObservedAt: *call.FinishedAt,
			})
			if err != nil {
				t.Fatal(err)
			}
			call.QuotaEvidence = &value
		}},
		{"usage quota", func(call *ModelCall) {
			value, err := NormalizeQuotaEvidence(QuotaObservationInput{
				Endpoint: call.Endpoint, FinalJSON: []byte(`{"usage":{"quota":1}}`), ObservedAt: *call.FinishedAt,
			})
			if err != nil {
				t.Fatal(err)
			}
			call.QuotaEvidence = &value
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := quotaReportBuilderInput(t, "task_incomplete_evidence")
			call := quotaReportBuilderCall(t, input.Task, "call_incomplete", "", false, 5)
			call.State = CallInterrupted
			test.change(&call)
			input.Calls = []ModelCall{quotaReportBuilderCall(t, input.Task, "call_complete", "1", true, 3), call}
			built, err := buildFinalQuotaReport(input)
			if err != nil || built.Disposition != ReportBuildEvidencePending || built.Final != nil {
				t.Fatalf("incomplete evidence was omitted: %+v, err=%v", built, err)
			}
		})
	}
}

func TestQuotaReportIncludesMeteredInterruptedCall(t *testing.T) {
	input := quotaReportBuilderInput(t, "task_metered_interruption")
	call := quotaReportBuilderCall(t, input.Task, "call_interrupted", "2.5", true, 5)
	call.State = CallInterrupted
	input.Calls = []ModelCall{call}
	built, err := buildFinalQuotaReport(input)
	if err != nil || built.Final == nil || built.Final.ClientSummary.TaskTotalQuota != "2.5" || len(built.Final.Calls) != 1 {
		t.Fatalf("metered interruption = %+v, err=%v", built, err)
	}
}

func TestSQLiteQuotaReportRecoversEvidencePendingAfterUnmeteredInterruption(t *testing.T) {
	db := openSchemaTestDB(t)
	task := createQuotaReportStoreTask(t, db, "task_interrupted_recovery", false, true, "request_complete")
	ctx := context.Background()
	call := ModelCall{
		CallID: "call_interrupted", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		Role: CallRoleSubagent, ModelID: task.InitialModelID, Provider: "fixture", Endpoint: MaasEndpointChatCompletions,
		PricingEvidenceID: task.PricingEvidenceID, State: CallPrepared,
	}
	calls := NewSQLiteCallStore(db)
	taskContext := storeContext(t, task)
	if err := calls.Prepare(ctx, taskContext, call); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(ctx, taskContext, call.CallID, storeNow().Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	finished := storeNow().Add(7 * time.Minute)
	call.State, call.FinishedAt = CallInterrupted, &finished
	call.Usage = NormalizedUsage{BreakdownStatus: UsageUnavailable}
	if err := calls.Complete(ctx, taskContext, call); err != nil {
		t.Fatal(err)
	}
	result := TaskResultCompleted
	if err := NewSQLiteTaskStore(db).UpdateState(ctx, TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskUsagePending,
		Result: &result, At: storeNow().Add(10 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	// Reproduce the state left by the previous builder; no evidence supplement
	// or mutation of the failed call is needed to recover it.
	if err := NewSQLiteQuotaWorkflowStore(db).Update(ctx, QuotaWorkflowUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: QuotaWorkflowCollecting, To: QuotaWorkflowEvidencePending,
		At: storeNow().Add(11 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	build, pending, err := NewSQLiteReportStore(db).EnqueueFinal(ctx, task.Scope, task.TaskID)
	if err != nil || pending == nil || build.Final == nil || build.Final.ClientSummary.TaskTotalQuota != "1.25" {
		t.Fatalf("recovered final = %+v, pending=%+v, err=%v", build, pending, err)
	}
	if status := quotaStoreWorkflow(t, db, task).Status; status != QuotaWorkflowFinalUploadPending {
		t.Fatalf("workflow = %s", status)
	}
	input, err := NewSQLiteSubscriptionQuotaReader(db).ReadQuotaInput(ctx, task.Scope, task.TaskID)
	if err != nil || len(input.Calls) != 2 {
		t.Fatalf("local call history lost: calls=%d, err=%v", len(input.Calls), err)
	}
}
