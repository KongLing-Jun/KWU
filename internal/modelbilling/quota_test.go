package modelbilling

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestNormalizeQuotaEvidencePreservesPresenceTypeAndPrecedence(t *testing.T) {
	observedAt := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	requestHeaders := map[string][]string{"x-oneapi-request-id": {"request_01"}}
	tests := []struct {
		name       string
		headers    map[string][]string
		payload    string
		status     QuotaEvidenceStatus
		quota      string
		source     QuotaSource
		issues     []QuotaEvidenceIssue
		billingRaw QuotaJSONType
		usageRaw   QuotaJSONType
	}{
		{name: "equivalent decimal numbers", headers: requestHeaders, payload: `{"billing":{"quota":50},"usage":{"quota":50.0}}`, status: QuotaEvidenceComplete, quota: "50", source: QuotaSourceBothConsistent, issues: []QuotaEvidenceIssue{}, billingRaw: QuotaJSONNumber, usageRaw: QuotaJSONNumber},
		{name: "billing only", headers: requestHeaders, payload: `{"billing":{"quota":1.25}}`, status: QuotaEvidenceComplete, quota: "1.25", source: QuotaSourceBilling, issues: []QuotaEvidenceIssue{}, billingRaw: QuotaJSONNumber},
		{name: "both missing", headers: requestHeaders, payload: `{}`, status: QuotaEvidenceMissing, issues: []QuotaEvidenceIssue{QuotaIssueMissing}},
		{name: "request id missing", payload: `{"usage":{"quota":2}}`, status: QuotaEvidenceMissing, quota: "2", source: QuotaSourceUsage, issues: []QuotaEvidenceIssue{QuotaIssueRequestIDMissing}, usageRaw: QuotaJSONNumber},
		{name: "invalid beats missing", payload: `{"billing":{"quota":"2"}}`, status: QuotaEvidenceInvalid, source: QuotaSourceBilling, issues: []QuotaEvidenceIssue{QuotaIssueRequestIDMissing, QuotaIssueInvalid}, billingRaw: QuotaJSONString},
		{name: "conflict beats missing", payload: `{"billing":{"quota":1},"usage":{"quota":2}}`, status: QuotaEvidenceConflicting, issues: []QuotaEvidenceIssue{QuotaIssueRequestIDMissing, QuotaIssueConflicting}, billingRaw: QuotaJSONNumber, usageRaw: QuotaJSONNumber},
		{name: "explicit null is invalid not absent", headers: requestHeaders, payload: `{"billing":{"quota":null}}`, status: QuotaEvidenceInvalid, source: QuotaSourceBilling, issues: []QuotaEvidenceIssue{QuotaIssueInvalid}, billingRaw: QuotaJSONNull},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
				Endpoint: MaasEndpointResponses, Headers: test.headers, FinalJSON: json.RawMessage(test.payload), ObservedAt: observedAt,
			})
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Revision != 1 || evidence.Status != test.status || !reflect.DeepEqual(evidence.Issues, test.issues) {
				t.Fatalf("evidence status = revision %d, %q, issues %v", evidence.Revision, evidence.Status, evidence.Issues)
			}
			if optionalDecimalText(evidence.Quota) != test.quota || optionalQuotaSourceText(evidence.Source) != string(test.source) {
				t.Fatalf("evidence projection = quota %v, source %v", evidence.Quota, evidence.Source)
			}
			if evidence.BillingQuotaRaw.Type != test.billingRaw || evidence.UsageQuotaRaw.Type != test.usageRaw {
				t.Fatalf("raw types = billing %q, usage %q", evidence.BillingQuotaRaw.Type, evidence.UsageQuotaRaw.Type)
			}
		})
	}
}

func TestNormalizeQuotaEvidenceRevisionsPreservesAllCanonicalHeaderIDs(t *testing.T) {
	input := QuotaObservationInput{
		Endpoint: MaasEndpointChatCompletions,
		Headers: map[string][]string{
			"X-Oneapi-Request-Id": {"request-a"},
			"x-oneapi-request-id": {"request-b", "request-a"},
		},
		FinalJSON:  json.RawMessage(`{"billing":{"quota":3}}`),
		ObservedAt: time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC),
	}
	revisions, err := NormalizeQuotaEvidenceRevisions(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0].Revision != 1 || revisions[0].OneAPIRequestID == nil || *revisions[0].OneAPIRequestID != "request-a" ||
		revisions[1].Revision != 2 || revisions[1].OneAPIRequestID == nil || *revisions[1].OneAPIRequestID != "request-b" {
		t.Fatalf("request ID revisions = %+v", revisions)
	}
	if _, err := NormalizeQuotaEvidence(input); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("singular normalization error = %v", err)
	}
}

func TestNormalizeQuotaEvidenceRequestIDCanonicality(t *testing.T) {
	base := QuotaObservationInput{
		Endpoint:   MaasEndpointResponses,
		FinalJSON:  json.RawMessage(`{"usage":{"quota":1}}`),
		ObservedAt: time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC),
	}
	base.Headers = map[string][]string{"X-Oneapi-Request-Id": {""}}
	evidence, err := NormalizeQuotaEvidence(base)
	if err != nil || evidence.OneAPIRequestID != nil || !fixtureContainsQuotaIssue(evidence.Issues, QuotaIssueRequestIDMissing) {
		t.Fatalf("blank header evidence/error = %+v/%v", evidence, err)
	}
	for _, requestID := range []string{" request-a", "request-a ", "   "} {
		base.Headers = map[string][]string{"X-Oneapi-Request-Id": {requestID}}
		if _, err := NormalizeQuotaEvidence(base); ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("request ID %q error = %v", requestID, err)
		}
	}
}

func TestQuotaRawValueWireDistinguishesAbsentAndNull(t *testing.T) {
	absentJSON, err := json.Marshal(QuotaRawValue{})
	if err != nil {
		t.Fatal(err)
	}
	nullValue, err := normalizeQuotaRawJSON(json.RawMessage(`null`))
	if err != nil {
		t.Fatal(err)
	}
	nullJSON, err := json.Marshal(nullValue)
	if err != nil {
		t.Fatal(err)
	}
	if string(absentJSON) != `{"present":false,"json_type":null,"value":null}` ||
		string(nullJSON) != `{"present":true,"json_type":"null","value":null}` {
		t.Fatalf("wire values = absent %s, null %s", absentJSON, nullJSON)
	}
	for _, wire := range [][]byte{absentJSON, nullJSON} {
		var decoded QuotaRawValue
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatalf("decode %s: %v", wire, err)
		}
	}
	for _, invalid := range []string{`{}`, `{"present":false,"json_type":""}`, `{"present":false,"json_type":"","value":null,"extra":true}`} {
		var decoded QuotaRawValue
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatalf("accepted non-canonical quota raw wire %s", invalid)
		}
	}
}

func TestSQLiteCallStoreRequiresExplicitSubscriptionEndpoint(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingSubscriptionPool, "task_endpoint_required")
	taskContext := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := tasks.RecordFreezeRequest(context.Background(), taskContext, freezeRequest(task), storeNow().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.RecordFreezeResult(context.Background(), taskContext, activeFreeze(task),
		&TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	call := ModelCall{
		CallID: "call_endpoint_required", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared,
	}
	if err := NewSQLiteCallStore(db).Prepare(context.Background(), taskContext, call); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("missing endpoint error = %v", err)
	}
}

func TestSQLiteCallIDIsScopedByTaskAcrossQuotaEvidenceAndReportLinks(t *testing.T) {
	db := openSchemaTestDB(t)
	scope := testTaskScope(BillingSubscriptionPool)
	firstTask, firstPricing := createReaderRunningSubscriptionTask(t, db, "task_shared_call_first", scope)
	secondTask, secondPricing := createReaderRunningSubscriptionTask(t, db, "task_shared_call_second", scope)
	const callID = "call_shared_between_tasks"

	callFor := func(task ModelTask, pricing PricingEvidence) ModelCall {
		return ModelCall{
			CallID: callID, TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
			Role: CallRoleMain, ModelID: pricing.ModelID, Provider: "fixture",
			Endpoint: MaasEndpointChatCompletions, PricingEvidenceID: pricing.EvidenceID, State: CallPrepared,
		}
	}
	calls := NewSQLiteCallStore(db)
	firstCall := callFor(firstTask, firstPricing)
	firstContext := storeContext(t, firstTask)
	if err := calls.Prepare(context.Background(), firstContext, firstCall); err != nil {
		t.Fatal(err)
	}
	if err := calls.Prepare(context.Background(), firstContext, firstCall); err != nil {
		t.Fatalf("same-task exact prepare retry: %v", err)
	}
	conflicting := firstCall
	conflicting.Provider = "different-provider"
	if err := calls.Prepare(context.Background(), firstContext, conflicting); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("same-task conflicting prepare error = %v", err)
	}

	secondCall := callFor(secondTask, secondPricing)
	secondContext := storeContext(t, secondTask)
	if err := calls.Prepare(context.Background(), secondContext, secondCall); err != nil {
		t.Fatalf("second task reusing call_id: %v", err)
	}

	tests := []struct {
		task      ModelTask
		context   TaskContext
		call      ModelCall
		requestID string
		payload   string
		minute    int
	}{
		{firstTask, firstContext, firstCall, "request_shared_first", `{"billing":{"quota":1}}`, 4},
		{secondTask, secondContext, secondCall, "", `{}`, 5},
	}
	for _, test := range tests {
		startedAt := storeNow().Add(time.Duration(test.minute) * time.Minute)
		if err := calls.BeginAttempt(context.Background(), test.context, callID, startedAt); err != nil {
			t.Fatalf("begin %s: %v", test.task.TaskID, err)
		}
		finishedAt := startedAt.Add(time.Minute)
		var headers map[string][]string
		if test.requestID != "" {
			headers = map[string][]string{"X-Oneapi-Request-Id": {test.requestID}}
		}
		evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
			Endpoint:  MaasEndpointChatCompletions,
			Headers:   headers,
			FinalJSON: json.RawMessage(test.payload), ObservedAt: finishedAt,
		})
		if err != nil {
			t.Fatal(err)
		}
		requestCount := int64(1)
		test.call.State, test.call.FinishedAt, test.call.QuotaEvidence = CallCompleted, &finishedAt, &evidence
		test.call.Usage = NormalizedUsage{RequestCount: &requestCount, BreakdownStatus: UsageExact}
		if err := calls.Complete(context.Background(), test.context, test.call); err != nil {
			t.Fatalf("complete %s: %v", test.task.TaskID, err)
		}
	}

	supplementQuotaEvidence(t, db, firstTask, callID, 2, "request_shared_first", "11", storeNow().Add(8*time.Minute))
	supplementQuotaEvidence(t, db, secondTask, callID, 2, "request_shared_second", "22", storeNow().Add(9*time.Minute))
	supplementQuotaEvidence(t, db, secondTask, callID, 3, "request_shared_second", "23", storeNow().Add(10*time.Minute))
	evidenceStore := NewSQLiteQuotaEvidenceStore(db)
	for _, test := range []struct {
		task      ModelTask
		requestID string
		quota     DecimalString
		revision  uint64
	}{
		{firstTask, "request_shared_first", "11", 2},
		{secondTask, "request_shared_second", "23", 3},
	} {
		loaded, err := calls.ListByTask(context.Background(), test.task.Scope, test.task.TaskID)
		if err != nil || len(loaded) != 1 || loaded[0].CallID != callID {
			t.Fatalf("calls for %s = %+v, %v", test.task.TaskID, loaded, err)
		}
		history, err := evidenceStore.History(context.Background(), test.task.Scope, test.task.TaskID, callID)
		if err != nil || len(history) != int(test.revision) {
			t.Fatalf("history for %s = %+v, %v", test.task.TaskID, history, err)
		}
		current, found, err := evidenceStore.Current(context.Background(), test.task.Scope, test.task.TaskID, callID)
		if err != nil || !found || current.Revision != test.revision || current.OneAPIRequestID == nil ||
			*current.OneAPIRequestID != test.requestID || current.Quota == nil || *current.Quota != test.quota {
			t.Fatalf("current evidence for %s = %+v, %v, %v", test.task.TaskID, current, found, err)
		}
	}

	now := stamp(storeNow().Add(12 * time.Minute))
	for _, report := range []struct{ key, taskID string }{
		{"report_shared_first", firstTask.TaskID},
		{"report_shared_second", secondTask.TaskID},
	} {
		if _, err := db.Exec(`INSERT INTO usage_reports
			(idempotency_key,task_id,report_kind,sequence,canonical_payload,state,next_attempt_at,created_at,updated_at,
			 contract_version,payload_hash)
			VALUES (?,?,'partial',1,'{}','pending',?,?,?,'quota-contract-v1',
			'0000000000000000000000000000000000000000000000000000000000000000')`,
			report.key, report.taskID, now, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id) VALUES (?,?,?)`,
			report.key, report.taskID, callID); err != nil {
			t.Fatalf("link report %s: %v", report.key, err)
		}
	}
	var linked int
	if err := db.QueryRow(`SELECT count(*) FROM usage_report_calls WHERE call_id=?`, callID).Scan(&linked); err != nil || linked != 2 {
		t.Fatalf("shared call report links = %d, %v", linked, err)
	}

	if _, err := db.Exec(`UPDATE model_calls SET quota_report_status='final_pending' WHERE task_id=? AND call_id=?`,
		firstTask.TaskID, callID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO model_call_quota_evidence_revisions (
		call_id,task_id,evidence_revision,oneapi_request_id,
		billing_quota_present,billing_quota_type,billing_quota_value,
		usage_quota_present,usage_quota_type,usage_quota_value,
		quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
	) SELECT call_id,task_id,3,oneapi_request_id,
		billing_quota_present,billing_quota_type,billing_quota_value,
		usage_quota_present,usage_quota_type,usage_quota_value,
		quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
		FROM model_call_quota_evidence_revisions
		WHERE task_id=? AND call_id=? AND evidence_revision=2`, firstTask.TaskID, callID); err == nil {
		t.Fatal("schema accepted quota evidence after the call became final-bound")
	}
}

func TestSQLiteQuotaEvidenceRevisionHistoryAndWorkflow(t *testing.T) {
	db := openSchemaTestDB(t)
	task, evidence := createReaderRunningSubscriptionTask(t, db, "task_quota_evidence", testTaskScope(BillingSubscriptionPool))
	taskContext := storeContext(t, task)
	calls := NewSQLiteCallStore(db)
	call := ModelCall{
		CallID: "call_quota_evidence", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", Endpoint: MaasEndpointChatCompletions, PricingEvidenceID: evidence.EvidenceID,
		State: CallPrepared,
	}
	if err := calls.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	startedAt := storeNow().Add(4 * time.Minute)
	if err := calls.BeginAttempt(context.Background(), taskContext, call.CallID, startedAt); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	inputTokens := int64(1)
	call.State, call.FinishedAt = CallCompleted, &finishedAt
	call.Usage = NormalizedUsage{InputTokens: &inputTokens, BreakdownStatus: UsageExact}
	if err := calls.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}

	loaded, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Endpoint != MaasEndpointChatCompletions || loaded[0].OccurredAt == nil ||
		loaded[0].QuotaEvidence == nil || loaded[0].QuotaEvidence.Revision != 1 || loaded[0].QuotaEvidence.Status != QuotaEvidenceMissing {
		t.Fatalf("completed subscription call = %+v", loaded)
	}

	supplement, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:  MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {"request_supplement"}},
		FinalJSON: json.RawMessage(`{"billing":{"quota":3}}`), ObservedAt: finishedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	supplement.Revision = 2
	evidenceStore := NewSQLiteQuotaEvidenceStore(db)
	if err := evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, supplement); err != nil {
		t.Fatal(err)
	}
	if err := evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, supplement); err != nil {
		t.Fatalf("exact current revision retry = %v", err)
	}
	conflictingRetry := supplement
	conflictingRetry.ObservedAt = conflictingRetry.ObservedAt.Add(time.Second)
	if err := evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, conflictingRetry); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("conflicting current revision retry = %v", err)
	}
	history, err := evidenceStore.History(context.Background(), task.Scope, task.TaskID, call.CallID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history after retry = %+v", history)
	}
	if err := evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, history[0]); err != nil {
		t.Fatalf("exact older revision retry = %v", err)
	}
	conflictingOlder := history[0]
	conflictingOlder.ObservedAt = conflictingOlder.ObservedAt.Add(time.Second)
	if err := evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, conflictingOlder); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("conflicting older revision retry = %v", err)
	}
	skipped := supplement
	skipped.Revision = 4
	skipped.ObservedAt = skipped.ObservedAt.Add(2 * time.Minute)
	if err := evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, skipped); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("skipped revision append = %v", err)
	}
	current, ok, err := evidenceStore.Current(context.Background(), task.Scope, task.TaskID, call.CallID)
	if err != nil || !ok || len(history) != 2 || current.Revision != 2 || current.Status != QuotaEvidenceComplete {
		t.Fatalf("history/current = %d/%+v/%v, err %v", len(history), current, ok, err)
	}
	regression := missingQuotaEvidence(nil, supplement.ObservedAt.Add(time.Minute))
	regression.Revision = 3
	if ErrorCodeOf(evidenceStore.Append(context.Background(), task.Scope, task.TaskID, call.CallID, regression)) != ErrorInvalidStateTransition {
		t.Fatal("complete quota evidence regression was accepted")
	}
	if _, err := db.Exec(`DELETE FROM model_call_quota_evidence_revisions WHERE call_id=?`, call.CallID); err == nil {
		t.Fatal("immutable quota evidence revision was deleted")
	}

	workflowStore := NewSQLiteQuotaWorkflowStore(db)
	workflow, err := workflowStore.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || workflow.Status != QuotaWorkflowCollecting {
		t.Fatalf("initial workflow = %+v, %v", workflow, err)
	}
	if err := workflowStore.Update(context.Background(), QuotaWorkflowUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: QuotaWorkflowCollecting, To: QuotaWorkflowFinalUploadPending,
		At: finishedAt.Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
}

func optionalDecimalText(value *DecimalString) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func TestQuotaEvidenceMultipleRequestIDsRemainAuditableAndFailClosed(t *testing.T) {
	db := openSchemaTestDB(t)
	task, evidence := createReaderRunningSubscriptionTask(
		t, db, "quota_request_id_wait", testTaskScope(BillingSubscriptionPool),
	)
	taskContext := storeContext(t, task)
	calls := NewSQLiteCallStore(db)
	call := ModelCall{
		CallID: "call_quota_request_id_wait", TaskID: task.TaskID, Scope: task.Scope,
		BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID,
		Provider: "fixture", Endpoint: MaasEndpointChatCompletions,
		PricingEvidenceID: evidence.EvidenceID, State: CallPrepared,
	}
	if err := calls.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	startedAt := storeNow().Add(4 * time.Minute)
	if err := calls.BeginAttempt(context.Background(), taskContext, call.CallID, startedAt); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	first, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:  MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {"request_a"}},
		FinalJSON: json.RawMessage(`{}`), ObservedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestCount := int64(1)
	call.State, call.FinishedAt, call.QuotaEvidence = CallCompleted, &finishedAt, &first
	call.Usage = NormalizedUsage{RequestCount: &requestCount, BreakdownStatus: UsageExact}
	if err := calls.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}

	second, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:  MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {"request_b"}},
		FinalJSON: json.RawMessage(`{"billing":{"quota":3}}`), ObservedAt: finishedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	second.Revision = 2
	store := NewSQLiteQuotaEvidenceStore(db)
	if err := store.Append(context.Background(), task.Scope, task.TaskID, call.CallID, second); err != nil {
		t.Fatal(err)
	}
	third, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:  MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {"request_b"}},
		FinalJSON: json.RawMessage(`{"billing":{"quota":4}}`), ObservedAt: finishedAt.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	third.Revision = 3
	if err := store.Append(context.Background(), task.Scope, task.TaskID, call.CallID, third); err != nil {
		t.Fatalf("append after request ID ambiguity = %v", err)
	}
	if err := store.Append(context.Background(), task.Scope, task.TaskID, call.CallID, second); err != nil {
		t.Fatalf("ambiguous history ACK retry = %v", err)
	}
	history, err := store.History(context.Background(), task.Scope, task.TaskID, call.CallID)
	if err != nil || len(history) != 3 || history[0].OneAPIRequestID == nil || history[1].OneAPIRequestID == nil || history[2].OneAPIRequestID == nil ||
		*history[0].OneAPIRequestID != "request_a" || *history[1].OneAPIRequestID != "request_b" || *history[2].OneAPIRequestID != "request_b" {
		t.Fatalf("request ID history = %+v, err %v", history, err)
	}
	if _, _, err := store.Current(context.Background(), task.Scope, task.TaskID, call.CallID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("ambiguous current evidence error = %v", err)
	}
	if _, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("ambiguous call projection error = %v", err)
	}
	task = markReaderTaskUsagePending(t, db, task, finishedAt.Add(3*time.Minute))
	if _, pending, err := NewSQLiteReportStore(db).EnqueueFinal(context.Background(), task.Scope, task.TaskID); ErrorCodeOf(err) != ErrorContractInvalid || pending != nil {
		t.Fatalf("ambiguous final projection = pending %+v, err %v", pending, err)
	}
}

func TestSQLiteCallStorePersistsAmbiguousTerminalRequestIDsAtomically(t *testing.T) {
	db := openSchemaTestDB(t)
	task, evidence := createReaderRunningSubscriptionTask(
		t, db, "quota_terminal_request_id_wait", testTaskScope(BillingSubscriptionPool),
	)
	taskContext := storeContext(t, task)
	store := NewSQLiteCallStore(db)
	call := ModelCall{
		CallID: "call_terminal_request_id_wait", TaskID: task.TaskID, Scope: task.Scope,
		BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID,
		Provider: "fixture", Endpoint: MaasEndpointChatCompletions,
		PricingEvidenceID: evidence.EvidenceID, State: CallPrepared,
	}
	if err := store.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	startedAt := storeNow().Add(4 * time.Minute)
	if err := store.BeginAttempt(context.Background(), taskContext, call.CallID, startedAt); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	first, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:  MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {"terminal_request_a"}},
		FinalJSON: json.RawMessage(`{"billing":{"quota":2}}`), ObservedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestA, requestB := "terminal_request_a", "terminal_request_b"
	failedCode := string(ErrorContractInvalid)
	call.State, call.FinishedAt, call.OccurredAt = CallFailed, &finishedAt, &finishedAt
	call.ProviderRequestID, call.LastErrorCode = &requestA, &failedCode
	call.Usage = NormalizedUsage{BreakdownStatus: UsageUnavailable}
	call.QuotaEvidence = &first
	call.quotaRequestIDs = []string{requestA, requestB}
	call.quotaEvidenceRevisions = quotaRevisionsForRequestIDs(first, call.quotaRequestIDs)
	if err := store.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatalf("terminal persistence ACK retry = %v", err)
	}
	history, err := NewSQLiteQuotaEvidenceStore(db).History(
		context.Background(), task.Scope, task.TaskID, call.CallID,
	)
	if err != nil || len(history) != 2 || history[0].OneAPIRequestID == nil || history[1].OneAPIRequestID == nil ||
		*history[0].OneAPIRequestID != requestA || *history[1].OneAPIRequestID != requestB {
		t.Fatalf("terminal request ID history = %+v, err %v", history, err)
	}
	if _, err := store.ListByTask(context.Background(), task.Scope, task.TaskID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("ambiguous terminal projection error = %v", err)
	}
}

func TestTerminalCallCandidateLeavesMissingQuotaEvidenceForValidation(t *testing.T) {
	task := ModelTask{BillingSource: BillingSubscriptionPool}
	existing := ModelCall{Endpoint: MaasEndpointChatCompletions}
	call := ModelCall{quotaRequestIDs: []string{"request-a", "request-b"}}
	candidate := terminalCallCandidate(call, task, existing)
	if candidate.QuotaEvidence != nil || len(candidate.quotaEvidenceRevisions) != 0 {
		t.Fatalf("invalid terminal input fabricated evidence = %+v", candidate)
	}
	if err := validateTerminalCandidate(candidate, task); err == nil {
		t.Fatal("invalid terminal input passed validation")
	}
}

func TestQuotaEvidenceLaterCompleteRevisionBecomesCurrent(t *testing.T) {
	db := openSchemaTestDB(t)
	task, pricing := createReaderRunningSubscriptionTask(
		t, db, "quota_same_request_conflict", testTaskScope(BillingSubscriptionPool),
	)
	const callID = "call_quota_same_request_conflict"
	addQuotaReportStoreCall(t, db, task, pricing, callID, true, "request_same", "3", 4)
	supplementQuotaEvidence(t, db, task, callID, 2, "request_same", "4", storeNow().Add(6*time.Minute))

	store := NewSQLiteQuotaEvidenceStore(db)
	history, err := store.History(context.Background(), task.Scope, task.TaskID, callID)
	if err != nil || len(history) != 2 {
		t.Fatalf("history = %+v, err %v", history, err)
	}
	current, found, err := store.Current(context.Background(), task.Scope, task.TaskID, callID)
	if err != nil || !found || current.Revision != 2 || current.Quota == nil || *current.Quota != "4" {
		t.Fatalf("current corrected evidence = %+v, %v, %v", current, found, err)
	}
	calls, err := NewSQLiteCallStore(db).ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil || len(calls) != 1 || calls[0].QuotaEvidence == nil || calls[0].QuotaEvidence.Revision != 2 ||
		calls[0].QuotaEvidence.Quota == nil || *calls[0].QuotaEvidence.Quota != "4" {
		t.Fatalf("corrected call projection = %+v, %v", calls, err)
	}
}

func TestQuotaEvidenceSimultaneousConflictingCompleteFactsFailClosed(t *testing.T) {
	db := openSchemaTestDB(t)
	task, pricing := createReaderRunningSubscriptionTask(
		t, db, "quota_same_observation_conflict", testTaskScope(BillingSubscriptionPool),
	)
	const callID = "call_quota_same_observation_conflict"
	observedAt := storeNow().Add(5 * time.Minute)
	addQuotaReportStoreCall(t, db, task, pricing, callID, true, "request_same", "3", 4)
	supplementQuotaEvidence(t, db, task, callID, 2, "request_same", "4", observedAt)

	store := NewSQLiteQuotaEvidenceStore(db)
	if _, _, err := store.Current(context.Background(), task.Scope, task.TaskID, callID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("simultaneous conflicting current evidence error = %v", err)
	}
	if _, err := NewSQLiteCallStore(db).ListByTask(context.Background(), task.Scope, task.TaskID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("simultaneous conflicting call projection error = %v", err)
	}
}

func optionalQuotaSourceText(value *QuotaSource) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
