package modelbilling

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestSQLiteSubscriptionUsageReaderReadsMultiModelCallsInCallIDOrder(t *testing.T) {
	db := openSchemaTestDB(t)
	task, initialEvidence := createReaderRunningSubscriptionTask(t, db, "task_reader_multi", testTaskScope(BillingSubscriptionPool))
	secondEvidence := PricingEvidence{
		EvidenceID: "evidence_reader_model_02", SnapshotID: "snapshot_reader_model_02",
		TaskID: task.TaskID, Scope: task.Scope, ModelID: "model_02", PricingGroup: initialEvidence.PricingGroup,
		State: PricingFresh, CapturedAt: storeNow().Add(4 * time.Minute),
	}
	insertReaderEvidence(t, db, task, secondEvidence)
	addReaderCompletedCall(t, db, task, "call_b", task.InitialModelID, initialEvidence, storeNow().Add(5*time.Minute))
	addReaderCompletedCall(t, db, task, "call_a", secondEvidence.ModelID, secondEvidence, storeNow().Add(6*time.Minute))
	task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
	before := readReaderStateSnapshot(t, db, task.TaskID)
	installReaderWriteGuards(t, db)

	input, err := NewSQLiteSubscriptionUsageReader(db).ReadUsageInput(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if input.Task.TaskID != task.TaskID || input.Task.State != TaskUsagePending || input.Task.Freeze == nil {
		t.Fatalf("task = %+v", input.Task)
	}
	if len(input.Calls) != 2 || input.Calls[0].Call.CallID != "call_a" || input.Calls[1].Call.CallID != "call_b" {
		t.Fatalf("call order = %+v", input.Calls)
	}
	if input.Calls[0].Call.ModelID != secondEvidence.ModelID || input.Calls[0].Evidence.EvidenceID != secondEvidence.EvidenceID ||
		input.Calls[1].Call.ModelID != task.InitialModelID || input.Calls[1].Evidence.EvidenceID != initialEvidence.EvidenceID {
		t.Fatalf("call evidence = %+v", input.Calls)
	}
	first := input.Calls[0].Call
	if first.Provider != "fixture" || first.ProviderRequestID == nil || *first.ProviderRequestID != "request_call_a" ||
		first.State != CallCompleted || first.AttemptCount != 1 || first.StartedAt == nil || first.FinishedAt == nil ||
		first.Usage.InputTokens == nil || *first.Usage.InputTokens != 1 || first.Usage.BreakdownStatus != UsageExact ||
		string(first.RawUsageRedacted) != `{"input_tokens":1}` {
		t.Fatalf("terminal call fields = %+v", first)
	}
	if after := readReaderStateSnapshot(t, db, task.TaskID); after != before {
		t.Fatalf("reader wrote state: before=%+v after=%+v", before, after)
	}
}

func TestSQLiteSubscriptionUsageReaderAllowsTaskWithoutCalls(t *testing.T) {
	db := openSchemaTestDB(t)
	task, _ := createReaderRunningSubscriptionTask(t, db, "task_reader_empty", testTaskScope(BillingSubscriptionPool))
	task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))

	input, err := NewSQLiteSubscriptionUsageReader(db).ReadUsageInput(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Calls) != 0 {
		t.Fatalf("calls = %+v", input.Calls)
	}
}

func TestSQLiteSubscriptionUsageReaderClassifiesMalformedPersistedCall(t *testing.T) {
	db := openSchemaTestDB(t)
	task, evidence := createReaderRunningSubscriptionTask(t, db, "task_reader_bad_call", testTaskScope(BillingSubscriptionPool))
	addReaderCompletedCall(t, db, task, "call_reader_bad", task.InitialModelID, evidence, storeNow().Add(5*time.Minute))
	task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
	if _, err := db.Exec(`UPDATE model_calls SET normalized_usage_json='{bad' WHERE call_id=?`, "call_reader_bad"); err != nil {
		t.Fatal(err)
	}

	_, err := NewSQLiteSubscriptionUsageReader(db).ReadUsageInput(context.Background(), task.Scope, task.TaskID)
	if ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("malformed persisted call error = %v, want contract_invalid", err)
	}
}

func TestSQLiteSubscriptionUsageReaderClassifiesMalformedPersistedEvidence(t *testing.T) {
	db := openSchemaTestDB(t)
	task, evidence := createReaderRunningSubscriptionTask(t, db, "task_reader_bad_evidence", testTaskScope(BillingSubscriptionPool))
	addReaderCompletedCall(t, db, task, "call_reader_bad_evidence", task.InitialModelID, evidence, storeNow().Add(5*time.Minute))
	task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
	if _, err := db.Exec(`UPDATE pricing_evidence SET captured_at='not-a-time' WHERE evidence_id=?`, evidence.EvidenceID); err != nil {
		t.Fatal(err)
	}

	_, err := NewSQLiteSubscriptionUsageReader(db).ReadUsageInput(context.Background(), task.Scope, task.TaskID)
	if ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("malformed persisted evidence error = %v, want contract_invalid", err)
	}
}

func TestUsagePendingRecordsTopLevelExecutionEnd(t *testing.T) {
	db := openSchemaTestDB(t)
	task, _ := createReaderRunningSubscriptionTask(t, db, "task_usage_pending_end", testTaskScope(BillingSubscriptionPool))
	endedAt := storeNow().Add(10 * time.Minute)
	task = markReaderTaskUsagePending(t, db, task, endedAt)
	if task.EndedAt == nil || !task.EndedAt.Equal(endedAt) {
		t.Fatalf("usage_pending ended_at = %v, want %v", task.EndedAt, endedAt)
	}
}

func TestUsagePendingPreservesExistingExecutionEnd(t *testing.T) {
	db := openSchemaTestDB(t)
	task, _ := createReaderRunningSubscriptionTask(t, db, "task_usage_pending_existing_end", testTaskScope(BillingSubscriptionPool))
	store := NewSQLiteTaskStore(db)
	result := TaskResultFailed
	errorCode := string(ErrorGatewayUnavailable)
	endedAt := storeNow().Add(10 * time.Minute)
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskFailed,
		Result: &result, LastErrorCode: &errorCode, At: endedAt,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskFailed, To: TaskUsagePending,
		Result: &result, LastErrorCode: &errorCode, At: endedAt.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	task, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.EndedAt == nil || !task.EndedAt.Equal(endedAt) {
		t.Fatalf("usage_pending ended_at = %v, want preserved %v", task.EndedAt, endedAt)
	}
}

func TestSubscriptionSettlementPreservesTopLevelExecutionEnd(t *testing.T) {
	db := openSchemaTestDB(t)
	task, _ := createReaderRunningSubscriptionTask(t, db, "task_settlement_preserves_end", testTaskScope(BillingSubscriptionPool))
	store := NewSQLiteTaskStore(db)
	result := TaskResultCompleted
	endedAt := storeNow().Add(10 * time.Minute)
	for _, update := range []TaskStateUpdate{
		{Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskUsagePending, Result: &result, At: endedAt},
		{Scope: task.Scope, TaskID: task.TaskID, From: TaskUsagePending, To: TaskSettling, Result: &result, At: endedAt.Add(time.Minute)},
		{Scope: task.Scope, TaskID: task.TaskID, From: TaskSettling, To: TaskCompleted, Result: &result, At: endedAt.Add(2 * time.Minute)},
	} {
		if err := store.UpdateState(context.Background(), update); err != nil {
			t.Fatal(err)
		}
	}
	task, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.EndedAt == nil || !task.EndedAt.Equal(endedAt) {
		t.Fatalf("completed ended_at = %v, want top-level end %v", task.EndedAt, endedAt)
	}
}

func TestSQLiteSubscriptionUsageReaderChecksScopeBeforeBillingSource(t *testing.T) {
	db := openSchemaTestDB(t)
	subscription, _ := createReaderRunningSubscriptionTask(t, db, "task_reader_scope", testTaskScope(BillingSubscriptionPool))
	byok, byokEvidence := storeTask(BillingNewAPIByok, "task_reader_byok")
	if err := NewSQLiteTaskStore(db).CreateTaskWithEvidence(context.Background(), byok, byokEvidence); err != nil {
		t.Fatal(err)
	}
	wrongSubscriptionScope := subscription.Scope
	wrongSubscriptionScope.ActorID = "other_actor"
	wrongBYOKScope := byok.Scope
	wrongBYOKScope.ActorID = "other_actor"
	reader := NewSQLiteSubscriptionUsageReader(db)

	for _, test := range []struct {
		name   string
		scope  TaskScope
		taskID string
		code   ErrorCode
	}{
		{name: "subscription wrong scope", scope: wrongSubscriptionScope, taskID: subscription.TaskID, code: ErrorScopeMismatch},
		{name: "BYOK matching scope", scope: byok.Scope, taskID: byok.TaskID, code: ErrorBillingSourceMismatch},
		{name: "BYOK wrong scope wins", scope: wrongBYOKScope, taskID: byok.TaskID, code: ErrorScopeMismatch},
		{name: "missing task", scope: subscription.Scope, taskID: "task_reader_missing", code: ErrorContractInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := reader.ReadUsageInput(context.Background(), test.scope, test.taskID)
			if ErrorCodeOf(err) != test.code {
				t.Fatalf("error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestSQLiteSubscriptionUsageReaderRejectsCorruptEvidenceOwnershipAndModel(t *testing.T) {
	t.Run("cross task", func(t *testing.T) {
		db := openSchemaTestDB(t)
		task, evidence := createReaderRunningSubscriptionTask(t, db, "task_reader_owner", testTaskScope(BillingSubscriptionPool))
		addReaderCompletedCall(t, db, task, "call_reader_owner", task.InitialModelID, evidence, storeNow().Add(5*time.Minute))
		task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
		other, _ := createReaderRunningSubscriptionTask(t, db, "task_reader_other", testTaskScope(BillingSubscriptionPool))
		corruptReaderForeignKey(t, db, `UPDATE model_calls SET pricing_evidence_id=? WHERE call_id=?`, other.PricingEvidenceID, "call_reader_owner")

		_, err := NewSQLiteSubscriptionUsageReader(db).ReadUsageInput(context.Background(), task.Scope, task.TaskID)
		if ErrorCodeOf(err) != ErrorScopeMismatch {
			t.Fatalf("cross-task evidence error = %v", err)
		}
	})

	t.Run("cross model", func(t *testing.T) {
		db := openSchemaTestDB(t)
		task, evidence := createReaderRunningSubscriptionTask(t, db, "task_reader_model", testTaskScope(BillingSubscriptionPool))
		addReaderCompletedCall(t, db, task, "call_reader_model", task.InitialModelID, evidence, storeNow().Add(5*time.Minute))
		task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
		corruptReaderForeignKey(t, db, `UPDATE pricing_evidence SET model_id='other_model' WHERE evidence_id=?`, evidence.EvidenceID)

		_, err := NewSQLiteSubscriptionUsageReader(db).ReadUsageInput(context.Background(), task.Scope, task.TaskID)
		if ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("cross-model evidence error = %v", err)
		}
	})
}

func TestSQLiteSubscriptionUsageReaderListsOnlyScopedUsagePendingInStableOrder(t *testing.T) {
	db := openSchemaTestDB(t)
	scope := testTaskScope(BillingSubscriptionPool)
	base := storeNow().Add(20 * time.Minute)
	for _, item := range []struct {
		id string
		at time.Time
	}{
		{id: "task_pending_b", at: base},
		{id: "task_pending_a", at: base},
		{id: "task_pending_late", at: base.Add(time.Minute)},
	} {
		task, _ := createReaderRunningSubscriptionTask(t, db, item.id, scope)
		markReaderTaskUsagePending(t, db, task, item.at)
	}
	createReaderRunningSubscriptionTask(t, db, "task_reader_running", scope)
	otherScope := testTaskScope(BillingSubscriptionPool)
	otherScope.ActorID = "other_actor"
	other, _ := createReaderRunningSubscriptionTask(t, db, "task_pending_other_scope", otherScope)
	markReaderTaskUsagePending(t, db, other, base)
	byok, byokEvidence := storeTask(BillingNewAPIByok, "task_pending_byok")
	if err := NewSQLiteTaskStore(db).CreateTaskWithEvidence(context.Background(), byok, byokEvidence); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET state=?,task_result=?,updated_at=? WHERE task_id=?`,
		TaskUsagePending, TaskResultCompleted, stamp(base), byok.TaskID); err != nil {
		t.Fatal(err)
	}

	tasks, err := NewSQLiteSubscriptionUsageReader(db).ListUsagePending(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"task_pending_a", "task_pending_b", "task_pending_late"}
	if len(tasks) != len(want) {
		t.Fatalf("tasks = %+v", tasks)
	}
	for i := range want {
		if tasks[i].TaskID != want[i] || tasks[i].BillingSource != BillingSubscriptionPool || tasks[i].State != TaskUsagePending || !tasks[i].Scope.Equal(scope) {
			t.Fatalf("tasks[%d] = %+v, want %s", i, tasks[i], want[i])
		}
	}
}

func TestSQLiteSubscriptionUsageReaderListsIDsWithoutHydratingMalformedTask(t *testing.T) {
	db := openSchemaTestDB(t)
	scope := testTaskScope(BillingSubscriptionPool)
	for _, taskID := range []string{"task_pending_bad", "task_pending_good"} {
		task, _ := createReaderRunningSubscriptionTask(t, db, taskID, scope)
		markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
	}
	if _, err := db.Exec(`UPDATE model_tasks SET freeze_expires_at='not-a-time' WHERE task_id=?`, "task_pending_bad"); err != nil {
		t.Fatal(err)
	}
	reader := NewSQLiteSubscriptionUsageReader(db)
	if _, err := reader.ListUsagePending(context.Background(), scope); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("hydrating ListUsagePending error = %v, want contract_invalid", err)
	}
	ids, err := reader.ListUsagePendingIDs(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"task_pending_bad", "task_pending_good"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestSQLiteSubscriptionUsageReaderRecoversEveryEndedTaskResult(t *testing.T) {
	db := openSchemaTestDB(t)
	scope := testTaskScope(BillingSubscriptionPool)
	wantResults := map[string]TaskResult{
		"task_pending_completed": TaskResultCompleted,
		"task_pending_failed":    TaskResultFailed,
		"task_pending_cancelled": TaskResultCancelled,
	}
	store := NewSQLiteTaskStore(db)
	for taskID, result := range wantResults {
		task, _ := createReaderRunningSubscriptionTask(t, db, taskID, scope)
		var code *string
		if result != TaskResultCompleted {
			value := string(ErrorGatewayUnavailable)
			code = &value
		}
		if err := store.UpdateState(context.Background(), TaskStateUpdate{
			Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskUsagePending,
			Result: &result, LastErrorCode: code, At: storeNow().Add(15 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := NewSQLiteSubscriptionUsageReader(db).ListUsagePending(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != len(wantResults) {
		t.Fatalf("tasks = %+v", tasks)
	}
	for _, task := range tasks {
		want, ok := wantResults[task.TaskID]
		if !ok || task.Result == nil || *task.Result != want || task.EndedAt == nil {
			t.Fatalf("recovered task = %+v", task)
		}
	}
}

func createReaderRunningSubscriptionTask(t *testing.T, db *sql.DB, taskID string, scope TaskScope) (ModelTask, PricingEvidence) {
	t.Helper()
	tasks := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingSubscriptionPool, taskID)
	task.Scope = cloneTaskScope(scope)
	evidence.Scope = cloneTaskScope(scope)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	taskContext := storeContext(t, task)
	if err := tasks.RecordFreezeRequest(context.Background(), taskContext, freezeRequest(task), storeNow().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.RecordFreezeResult(context.Background(), taskContext, activeFreeze(task),
		&TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: storeNow().Add(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := tasks.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded, evidence
}

func markReaderTaskUsagePending(t *testing.T, db *sql.DB, task ModelTask, at time.Time) ModelTask {
	t.Helper()
	result := TaskResultCompleted
	tasks := NewSQLiteTaskStore(db)
	if err := tasks.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskUsagePending, Result: &result, At: at,
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := tasks.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func insertReaderEvidence(t *testing.T, db *sql.DB, task ModelTask, evidence PricingEvidence) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := InsertPricingEvidence(context.Background(), tx, PricingEvidenceOwner{
		TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
	}, evidence); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func addReaderCompletedCall(t *testing.T, db *sql.DB, task ModelTask, callID, modelID string, evidence PricingEvidence, at time.Time) {
	t.Helper()
	calls := NewSQLiteCallStore(db)
	taskContext := storeContext(t, task)
	call := ModelCall{
		CallID: callID, TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		Role: CallRoleMain, ModelID: modelID, Provider: "fixture", Endpoint: MaasEndpointChatCompletions, PricingEvidenceID: evidence.EvidenceID,
		State: CallPrepared,
	}
	if err := calls.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(context.Background(), taskContext, callID, at); err != nil {
		t.Fatal(err)
	}
	providerRequestID := "request_" + callID
	inputTokens := int64(1)
	call.ProviderRequestID = &providerRequestID
	call.RawUsageRedacted = []byte(`{"input_tokens":1}`)
	call.State = CallCompleted
	finishedAt := at.Add(time.Minute)
	call.FinishedAt = &finishedAt
	call.Usage = NormalizedUsage{InputTokens: &inputTokens, BreakdownStatus: UsageExact}
	if err := calls.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
}

func corruptReaderForeignKey(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`)
	if _, err := conn.ExecContext(context.Background(), statement, args...); err != nil {
		t.Fatal(err)
	}
}

type readerStateSnapshot struct {
	state, updatedAt                      string
	callCount, evidenceCount, reportCount int
}

func readReaderStateSnapshot(t *testing.T, db *sql.DB, taskID string) readerStateSnapshot {
	t.Helper()
	var value readerStateSnapshot
	if err := db.QueryRow(`SELECT state,updated_at,
		(SELECT count(*) FROM model_calls WHERE task_id=?),
		(SELECT count(*) FROM pricing_evidence WHERE task_id=?),
		(SELECT count(*) FROM usage_reports WHERE task_id=?)
		FROM model_tasks WHERE task_id=?`, taskID, taskID, taskID, taskID).Scan(
		&value.state, &value.updatedAt, &value.callCount, &value.evidenceCount, &value.reportCount); err != nil {
		t.Fatal(err)
	}
	return value
}

func installReaderWriteGuards(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"model_tasks", "pricing_evidence", "model_calls", "usage_reports", "usage_report_calls"} {
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			statement := fmt.Sprintf(`CREATE TRIGGER forbid_reader_%s_%s BEFORE %s ON %s
				BEGIN SELECT RAISE(ABORT, 'usage reader attempted a write'); END`, table, operation, operation, table)
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
		}
	}
}
