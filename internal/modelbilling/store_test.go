package modelbilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func storeNow() time.Time { return time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC) }

func storeTask(source BillingSource, id string) (ModelTask, PricingEvidence) {
	now := storeNow()
	task := ModelTask{TaskID: id, Scope: testTaskScope(source), BillingSource: source, InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0.0000", PricingEvidenceID: "evidence_" + id, State: TaskCreated, CreatedAt: now, UpdatedAt: now}
	evidence := PricingEvidence{EvidenceID: task.PricingEvidenceID, SnapshotID: "snapshot_01", TaskID: id, Scope: task.Scope, ModelID: "model_01", PricingGroup: "default", State: PricingFresh, CapturedAt: now}
	if source == BillingNewAPIByok {
		evidence.State = PricingUnavailable
		evidence.SnapshotID = "pricing-unavailable-v2:domestic:group_unavailable"
		evidence.PricingGroup = "__kwu_unavailable__"
	}
	return task, evidence
}
func storeContext(t *testing.T, task ModelTask) TaskContext {
	t.Helper()
	value, err := NewTaskContext(TaskContextInput{TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func updateTask(store *SQLiteTaskStore, task ModelTask, from, to TaskState) error {
	return store.UpdateState(context.Background(), TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: from, To: to, At: storeNow()})
}
func activeFreeze(task ModelTask) *cloud.Freeze {
	return &cloud.Freeze{Scope: *task.Scope.CloudBinding, FreezeID: "freeze_" + task.TaskID, TaskID: task.TaskID, Status: cloud.FreezeActive, FreezeAmount: "0", FreeFrozenAmount: "0", SubscriptionFrozenAmount: "0", ExpiresAt: storeNow().Add(time.Hour)}
}
func freezeRequest(task ModelTask) *cloud.FreezeRequest {
	return &cloud.FreezeRequest{Scope: *task.Scope.CloudBinding, TaskID: task.TaskID, ModelID: task.InitialModelID, ClientTaskType: task.ClientTaskType, ClientEstimatedCost: task.ClientEstimatedCost, IdempotencyKey: "freeze_request_" + task.TaskID}
}

func TestSQLiteTaskStoreBootstrapAndFreezeWriteAhead(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingSubscriptionPool, "task_store")
	ctx := storeContext(t, task)
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatalf("idempotent bootstrap: %v", err)
	}
	changedEvidence := evidence
	changedEvidence.CapturedAt = evidence.CapturedAt.Add(time.Second)
	if ErrorCodeOf(store.CreateTaskWithEvidence(context.Background(), task, changedEvidence)) != ErrorIdempotencyConflict {
		t.Fatal("changed evidence accepted")
	}
	changed := task
	changed.ClientEstimatedCost = "1"
	if ErrorCodeOf(store.CreateTaskWithEvidence(context.Background(), changed, evidence)) != ErrorIdempotencyConflict {
		t.Fatal("changed task accepted")
	}
	request := freezeRequest(task)
	if err := store.RecordFreezeRequest(context.Background(), ctx, request, storeNow().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordFreezeRequest(context.Background(), ctx, request, storeNow().Add(time.Minute)); err != nil {
		t.Fatalf("idempotent request: %v", err)
	}
	bad := *request
	bad.ModelID = "other"
	if ErrorCodeOf(store.RecordFreezeRequest(context.Background(), ctx, &bad, storeNow())) != ErrorContractInvalid {
		t.Fatal("cross model request accepted")
	}
	entitlement := &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}
	freeze := activeFreeze(task)
	if err := store.RecordFreezeResult(context.Background(), ctx, freeze, entitlement, storeNow().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordFreezeResult(context.Background(), ctx, freeze, entitlement, storeNow().Add(2*time.Minute)); err != nil {
		t.Fatalf("idempotent result: %v", err)
	}
	changedFreeze := *freeze
	changedFreeze.FreezeAmount = "1"
	if ErrorCodeOf(store.RecordFreezeResult(context.Background(), ctx, &changedFreeze, entitlement, storeNow().Add(2*time.Minute))) != ErrorIdempotencyConflict {
		t.Fatal("changed freeze result accepted")
	}
	got, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || got.State != TaskReady || got.FreezeRequest == nil || got.Freeze == nil || got.Entitlement == nil {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestSQLiteTaskStoreBYOKAndRecoveryIsolation(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_byok")
	ctx := storeContext(t, task)
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	request := &cloud.FreezeRequest{}
	if ErrorCodeOf(store.RecordFreezeRequest(context.Background(), ctx, request, storeNow())) != ErrorBillingSourceMismatch {
		t.Fatalf("BYOK freeze = %v", store.RecordFreezeRequest(context.Background(), ctx, request, storeNow()))
	}
	if err := updateTask(store, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(store, task, TaskReady, TaskRecoveryRequired); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListRecoverable(context.Background(), task.Scope)
	if err != nil || len(items) != 1 || items[0].TaskID != task.TaskID {
		t.Fatalf("recoverable = %+v, %v", items, err)
	}
	other := task.Scope
	other.ProfileID = "other"
	items, err = store.ListRecoverable(context.Background(), other)
	if err != nil || len(items) != 0 {
		t.Fatalf("cross-profile recovery leak = %+v, %v", items, err)
	}
}

func TestSQLiteTaskStoreListsOnlyScopedSettlingInStableOrder(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	scope := testTaskScope(BillingSubscriptionPool)
	base := storeNow().Add(20 * time.Minute)
	result := TaskResultCompleted

	toSettling := func(taskID string, taskScope TaskScope, at time.Time) {
		t.Helper()
		task, _ := createReaderRunningSubscriptionTask(t, db, taskID, taskScope)
		task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
		if err := store.UpdateState(context.Background(), TaskStateUpdate{
			Scope: task.Scope, TaskID: task.TaskID, From: TaskUsagePending, To: TaskSettling,
			Result: &result, At: at,
		}); err != nil {
			t.Fatal(err)
		}
	}

	toSettling("task_settling_b", scope, base)
	toSettling("task_settling_a", scope, base)
	toSettling("task_settling_late", scope, base.Add(time.Minute))
	createReaderRunningSubscriptionTask(t, db, "task_settling_running", scope)
	pending, _ := createReaderRunningSubscriptionTask(t, db, "task_settling_pending", scope)
	markReaderTaskUsagePending(t, db, pending, base)

	otherScope := testTaskScope(BillingSubscriptionPool)
	otherScope.ActorID = "other_actor"
	toSettling("task_settling_other_scope", otherScope, base)

	byok, byokEvidence := storeTask(BillingNewAPIByok, "task_settling_byok")
	if err := store.CreateTaskWithEvidence(context.Background(), byok, byokEvidence); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET state=?,task_result=?,updated_at=? WHERE task_id=?`,
		TaskSettling, result, stamp(base), byok.TaskID); err != nil {
		t.Fatal(err)
	}

	tasks, err := store.ListSettling(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"task_settling_a", "task_settling_b", "task_settling_late"}
	if len(tasks) != len(want) {
		t.Fatalf("tasks = %+v", tasks)
	}
	for i := range want {
		if tasks[i].TaskID != want[i] || tasks[i].BillingSource != BillingSubscriptionPool ||
			tasks[i].State != TaskSettling || !tasks[i].Scope.Equal(scope) {
			t.Fatalf("tasks[%d] = %+v, want %s", i, tasks[i], want[i])
		}
	}
}

func TestSQLiteTaskStoreListsSettlingIDsWithoutHydratingMalformedTask(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	scope := testTaskScope(BillingSubscriptionPool)
	result := TaskResultCompleted
	for _, taskID := range []string{"task_settling_bad", "task_settling_good"} {
		task, _ := createReaderRunningSubscriptionTask(t, db, taskID, scope)
		task = markReaderTaskUsagePending(t, db, task, storeNow().Add(10*time.Minute))
		if err := store.UpdateState(context.Background(), TaskStateUpdate{
			Scope: task.Scope, TaskID: task.TaskID, From: TaskUsagePending, To: TaskSettling,
			Result: &result, At: storeNow().Add(20 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE model_tasks SET freeze_expires_at='not-a-time' WHERE task_id=?`, "task_settling_bad"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListSettling(context.Background(), scope); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("hydrating ListSettling error = %v, want contract_invalid", err)
	}
	ids, err := store.ListSettlingIDs(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"task_settling_bad", "task_settling_good"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestSQLiteTaskStoreMarkRecoveryRequiredPreservesTaskFacts(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, _ := createReaderRunningSubscriptionTask(t, db, "task_recovery_cas", testTaskScope(BillingSubscriptionPool))
	result := TaskResultFailed
	code := "original_failure"
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskUsagePending,
		Result: &result, LastErrorCode: &code, At: storeNow().Add(10 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	at := storeNow().Add(30 * time.Minute)
	if err := store.MarkRecoveryRequired(context.Background(), task.Scope, task.TaskID, TaskSettling, at); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("wrong-state recovery error = %v", err)
	}
	if err := store.MarkRecoveryRequired(context.Background(), task.Scope, task.TaskID, TaskUsagePending, at); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRecoveryRequired(context.Background(), task.Scope, task.TaskID, TaskUsagePending, at); err != nil {
		t.Fatalf("idempotent recovery: %v", err)
	}
	after, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State != TaskRecoveryRequired || !after.UpdatedAt.Equal(at) ||
		!optionalTaskResultEqual(after.Result, before.Result) ||
		!optionalTimePointerEqual(after.EndedAt, before.EndedAt) ||
		!optionalStringEqual(after.LastErrorCode, before.LastErrorCode) {
		t.Fatalf("recovery changed task facts: before=%+v after=%+v", before, after)
	}
}

func TestSQLiteTaskStoreFreezeResultErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		edit func(*cloud.Freeze)
		code ErrorCode
	}{
		{"empty_freeze_id", func(freeze *cloud.Freeze) { freeze.FreezeID = "" }, ErrorContractInvalid},
		{"invalid_status", func(freeze *cloud.Freeze) { freeze.Status = cloud.FreezeStatus("invalid") }, ErrorContractInvalid},
		{"missing_expires_at", func(freeze *cloud.Freeze) { freeze.ExpiresAt = time.Time{} }, ErrorContractInvalid},
		{"task_id_mismatch", func(freeze *cloud.Freeze) { freeze.TaskID = "other_task" }, ErrorScopeMismatch},
		{"binding_generation_mismatch", func(freeze *cloud.Freeze) { freeze.Scope.BindingGeneration++ }, ErrorScopeMismatch},
		{"non_active", func(freeze *cloud.Freeze) { freeze.Status = cloud.FreezeCreated }, ErrorContractInvalid},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			db := openSchemaTestDB(t)
			store := NewSQLiteTaskStore(db)
			task, evidence := storeTask(BillingSubscriptionPool, "task_freeze_"+item.name)
			contextValue := storeContext(t, task)
			if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
				t.Fatal(err)
			}
			if err := store.RecordFreezeRequest(context.Background(), contextValue, freezeRequest(task), storeNow()); err != nil {
				t.Fatal(err)
			}
			freeze := activeFreeze(task)
			item.edit(freeze)
			err := store.RecordFreezeResult(context.Background(), contextValue, freeze, &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow())
			if ErrorCodeOf(err) != item.code {
				t.Fatalf("freeze error = %v, want %s", err, item.code)
			}
		})
	}
}

func TestSQLiteCallStoreLifecycleRetryAndTerminalIdempotency(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks := NewSQLiteTaskStore(db)
	calls := NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_call")
	ctx := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	call := ModelCall{CallID: "call_01", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleClassifier, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared, Usage: NormalizedUsage{}}
	if err := calls.Prepare(context.Background(), ctx, call); err != nil {
		t.Fatal(err)
	}
	if err := calls.Prepare(context.Background(), ctx, call); err != nil {
		t.Fatalf("idempotent prepare: %v", err)
	}
	if err := calls.BeginAttempt(context.Background(), ctx, call.CallID, storeNow()); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskReady, TaskRunning); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(context.Background(), ctx, call.CallID, storeNow().Add(time.Second)); err != nil {
		t.Fatalf("retry: %v", err)
	}
	finished := storeNow().Add(2 * time.Second)
	count := int64(1)
	call.State = CallCompleted
	call.FinishedAt = &finished
	call.Usage = NormalizedUsage{RequestCount: &count, BreakdownStatus: UsageExact}
	if err := calls.Complete(context.Background(), ctx, call); err != nil {
		t.Fatal(err)
	}
	if err := calls.Complete(context.Background(), ctx, call); err != nil {
		t.Fatalf("idempotent terminal: %v", err)
	}
	items, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil || len(items) != 1 || items[0].AttemptCount != 2 || items[0].State != CallCompleted {
		t.Fatalf("calls = %+v, %v", items, err)
	}
	call.State = CallFailed
	if ErrorCodeOf(calls.Complete(context.Background(), ctx, call)) != ErrorIdempotencyConflict {
		t.Fatal("terminal rewrite accepted")
	}
}

func TestSQLiteCallStorePrepareRejectsNonIntentFacts(t *testing.T) {
	zero := int64(0)
	now := storeNow()
	fields := []struct {
		name string
		edit func(*ModelCall)
	}{
		{"provider_request_id", func(call *ModelCall) { value := "request"; call.ProviderRequestID = &value }},
		{"last_error_code", func(call *ModelCall) { value := "error"; call.LastErrorCode = &value }},
		{"raw_usage", func(call *ModelCall) { call.RawUsageRedacted = json.RawMessage(`{"requests":0}`) }},
		{"usage_zero", func(call *ModelCall) { call.Usage.RequestCount = &zero }},
		{"breakdown_status", func(call *ModelCall) { call.Usage.BreakdownStatus = UsageExact }},
		{"started_at", func(call *ModelCall) { call.StartedAt = &now }},
		{"finished_at", func(call *ModelCall) { call.FinishedAt = &now }},
		{"attempt_count", func(call *ModelCall) { call.AttemptCount = 1 }},
		{"state", func(call *ModelCall) { call.State = CallRunning }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			db := openSchemaTestDB(t)
			tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
			task, evidence := storeTask(BillingNewAPIByok, "task_prepare_"+field.name)
			if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
				t.Fatal(err)
			}
			if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
				t.Fatal(err)
			}
			call := ModelCall{CallID: "call_prepare_" + field.name, TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
			field.edit(&call)
			if ErrorCodeOf(calls.Prepare(context.Background(), storeContext(t, task), call)) != ErrorContractInvalid {
				t.Fatal("non-intent fact accepted")
			}
		})
	}
	db := openSchemaTestDB(t)
	tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_prepare_clean")
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	clean := ModelCall{CallID: "call_prepare_clean", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
	if err := calls.Prepare(context.Background(), storeContext(t, task), clean); err != nil {
		t.Fatal(err)
	}
	if err := calls.Prepare(context.Background(), storeContext(t, task), clean); err != nil {
		t.Fatalf("clean intent retry: %v", err)
	}
}

func TestSQLiteCallStoreBeginAttemptRequiresRunnableTask(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_begin_recovery")
	contextValue := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	call := ModelCall{CallID: "call_begin_recovery", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
	if err := calls.Prepare(context.Background(), contextValue, call); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskReady, TaskRecoveryRequired); err != nil {
		t.Fatal(err)
	}
	if ErrorCodeOf(calls.BeginAttempt(context.Background(), contextValue, call.CallID, storeNow())) != ErrorInvalidStateTransition {
		t.Fatal("attempt started for recovery task")
	}
	items, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil || len(items) != 1 || items[0].State != CallPrepared || items[0].AttemptCount != 0 {
		t.Fatalf("rejected attempt rewrote call = %+v, %v", items, err)
	}
}

func TestSQLiteCallStorePersistsUsageMissingAndOtherTerminalStates(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_terminals")
	ctx := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	for _, state := range []CallState{CallUsageMissing, CallFailed, CallCancelled, CallInterrupted} {
		call := ModelCall{CallID: "call_" + string(state), TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
		if err := calls.Prepare(context.Background(), ctx, call); err != nil {
			t.Fatal(err)
		}
		if err := calls.BeginAttempt(context.Background(), ctx, call.CallID, storeNow()); err != nil {
			t.Fatal(err)
		}
		finished := storeNow().Add(time.Second)
		call.State, call.FinishedAt = state, &finished
		if state == CallUsageMissing {
			call.Usage.BreakdownStatus = UsageMissing
		}
		if err := calls.Complete(context.Background(), ctx, call); err != nil {
			t.Fatalf("complete %s: %v", state, err)
		}
	}
}

func TestSQLiteListQueriesReleaseRowsBeforeHydration(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_concurrent")
	contextValue := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	call := ModelCall{CallID: "call_concurrent", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
	if err := calls.Prepare(context.Background(), contextValue, call); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskReady, TaskRecoveryRequired); err != nil {
		t.Fatal(err)
	}
	queryContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var group sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 4; i++ {
		group.Add(2)
		go func() {
			defer group.Done()
			items, err := calls.ListByTask(queryContext, task.Scope, task.TaskID)
			if err == nil && (len(items) != 1 || items[0].CallID != call.CallID) {
				err = sql.ErrNoRows
			}
			results <- err
		}()
		go func() {
			defer group.Done()
			items, err := tasks.ListRecoverable(queryContext, task.Scope)
			if err == nil && (len(items) != 1 || items[0].TaskID != task.TaskID) {
				err = sql.ErrNoRows
			}
			results <- err
		}()
	}
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent list: %v", err)
		}
	}
}

func TestTaskStoreUpdateRejectsIllegalDeclaredFromBeforeIdempotency(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_illegal_from")
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(store, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	err := store.UpdateState(context.Background(), TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskCompleted, To: TaskReady, At: storeNow()})
	if ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("illegal declared from = %v", err)
	}
}

func TestTaskStoreRejectsBootstrapLifecycleFields(t *testing.T) {
	fields := []struct {
		name  string
		apply func(*ModelTask)
	}{
		{"ended_at", func(task *ModelTask) { value := storeNow(); task.EndedAt = &value }},
		{"last_error", func(task *ModelTask) { value := "failed"; task.LastErrorCode = &value }},
		{"result", func(task *ModelTask) { value := TaskResultFailed; task.Result = &value }},
		{"freeze_request", func(task *ModelTask) { task.FreezeRequest = &cloud.FreezeRequest{} }},
		{"freeze", func(task *ModelTask) { task.Freeze = &cloud.Freeze{} }},
		{"entitlement", func(task *ModelTask) { task.Entitlement = &TaskEntitlementSnapshot{} }},
	}
	for _, item := range fields {
		t.Run(item.name, func(t *testing.T) {
			db := openSchemaTestDB(t)
			store := NewSQLiteTaskStore(db)
			task, evidence := storeTask(BillingNewAPIByok, "task_bootstrap_"+item.name)
			item.apply(&task)
			if ErrorCodeOf(store.CreateTaskWithEvidence(context.Background(), task, evidence)) != ErrorContractInvalid {
				t.Fatal("bootstrap lifecycle field accepted")
			}
		})
	}
}

func TestTaskStoreCanonicalTimeIdempotency(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingSubscriptionPool, "task_canonical_time")
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	zone := time.FixedZone("same-instant", 8*60*60)
	retryTask, retryEvidence := task, evidence
	retryTask.CreatedAt, retryTask.UpdatedAt = task.CreatedAt.In(zone), task.UpdatedAt.In(zone)
	retryEvidence.CapturedAt = evidence.CapturedAt.In(zone)
	if err := store.CreateTaskWithEvidence(context.Background(), retryTask, retryEvidence); err != nil {
		t.Fatalf("canonical bootstrap retry: %v", err)
	}
	contextValue := storeContext(t, task)
	if err := store.RecordFreezeRequest(context.Background(), contextValue, freezeRequest(task), storeNow()); err != nil {
		t.Fatal(err)
	}
	freeze, entitlement := activeFreeze(task), &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}
	if err := store.RecordFreezeResult(context.Background(), contextValue, freeze, entitlement, storeNow()); err != nil {
		t.Fatal(err)
	}
	freezeRetry, entitlementRetry := *freeze, *entitlement
	freezeRetry.ExpiresAt = freeze.ExpiresAt.In(zone)
	entitlementRetry.CapturedAt = entitlement.CapturedAt.In(zone)
	if err := store.RecordFreezeResult(context.Background(), contextValue, &freezeRetry, &entitlementRetry, storeNow()); err != nil {
		t.Fatalf("canonical freeze retry: %v", err)
	}
}

func TestCollectIDsClosesRowsOnEveryPath(t *testing.T) {
	for _, item := range []struct {
		name string
		rows fakeIDRows
		want bool
	}{
		{"normal", fakeIDRows{values: []string{"one"}}, false},
		{"scan", fakeIDRows{values: []string{"one"}, scanErr: sql.ErrNoRows}, true},
		{"rows_err", fakeIDRows{err: sql.ErrNoRows}, true},
		{"close", fakeIDRows{closeErr: sql.ErrNoRows}, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			_, err := collectIDs(&item.rows)
			if (err != nil) != item.want || item.rows.closed == 0 {
				t.Fatalf("err=%v closed=%d", err, item.rows.closed)
			}
		})
	}
}

func TestTaskStoreEvidenceFailureRollsBackTask(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	first, firstEvidence := storeTask(BillingNewAPIByok, "task_evidence_first")
	if err := store.CreateTaskWithEvidence(context.Background(), first, firstEvidence); err != nil {
		t.Fatal(err)
	}
	task, evidence := storeTask(BillingNewAPIByok, "task_evidence_rollback")
	evidence.EvidenceID = firstEvidence.EvidenceID
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err == nil {
		t.Fatal("conflicting evidence accepted")
	}
	if _, err := store.Get(context.Background(), task.Scope, task.TaskID); err == nil {
		t.Fatal("task remained after evidence insert failure")
	}
}

func TestTaskStoreFreezeRejectsBYOKAndBindingMismatch(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	byok, byokEvidence := storeTask(BillingNewAPIByok, "task_byok_result")
	if err := store.CreateTaskWithEvidence(context.Background(), byok, byokEvidence); err != nil {
		t.Fatal(err)
	}
	if ErrorCodeOf(store.RecordFreezeResult(context.Background(), storeContext(t, byok), &cloud.Freeze{}, &TaskEntitlementSnapshot{}, storeNow())) != ErrorBillingSourceMismatch {
		t.Fatal("BYOK freeze result accepted")
	}

	task, evidence := storeTask(BillingSubscriptionPool, "task_binding_generation")
	contextValue := storeContext(t, task)
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	request := freezeRequest(task)
	request.Scope.BindingGeneration++
	if ErrorCodeOf(store.RecordFreezeRequest(context.Background(), contextValue, request, storeNow())) != ErrorScopeMismatch {
		t.Fatal("cross-binding request accepted")
	}
	other, otherEvidence := storeTask(BillingSubscriptionPool, "task_other_freeze")
	if err := store.CreateTaskWithEvidence(context.Background(), other, otherEvidence); err != nil {
		t.Fatal(err)
	}
	if ErrorCodeOf(store.RecordFreezeRequest(context.Background(), storeContext(t, other), freezeRequest(task), storeNow())) != ErrorScopeMismatch {
		t.Fatal("cross-task request accepted")
	}
	if err := store.RecordFreezeRequest(context.Background(), contextValue, freezeRequest(task), storeNow()); err != nil {
		t.Fatal(err)
	}
	notActive := activeFreeze(task)
	notActive.Status = cloud.FreezeCreated
	if ErrorCodeOf(store.RecordFreezeResult(context.Background(), contextValue, notActive, &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow())) != ErrorContractInvalid {
		t.Fatal("non-active freeze made task ready")
	}
	freeze := activeFreeze(task)
	freeze.Scope.BindingGeneration++
	if ErrorCodeOf(store.RecordFreezeResult(context.Background(), contextValue, freeze, &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: storeNow()}, storeNow())) != ErrorScopeMismatch {
		t.Fatal("cross-binding result accepted")
	}
}

func TestTaskStoreStateUpdatePersistsAndVerifiesRetry(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_state_update")
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(store, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	runningAt := storeNow().Add(100 * time.Millisecond)
	runningUpdate := TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: runningAt,
	}
	if err := store.UpdateState(context.Background(), runningUpdate); err != nil {
		t.Fatal(err)
	}
	running, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || running.StartedAt == nil || !running.StartedAt.Equal(runningAt) {
		t.Fatalf("running started_at = %+v, %v; want %s", running.StartedAt, err, runningAt)
	}
	if err := store.UpdateState(context.Background(), runningUpdate); err != nil {
		t.Fatalf("exact running retry: %v", err)
	}
	conflict := runningUpdate
	conflict.At = conflict.At.Add(time.Second)
	if ErrorCodeOf(store.UpdateState(context.Background(), conflict)) != ErrorIdempotencyConflict {
		t.Fatal("running retry with a different started_at was accepted")
	}
	result, errorCode := TaskResultFailed, "provider_timeout"
	update := TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskFailed, Result: &result, LastErrorCode: &errorCode, At: storeNow().Add(time.Minute)}
	if err := store.UpdateState(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || got.Result == nil || *got.Result != result || got.LastErrorCode == nil || *got.LastErrorCode != errorCode || got.EndedAt == nil {
		t.Fatalf("state update round trip = %+v, %v", got, err)
	}
	firstEnded := *got.EndedAt
	update.At = update.At.Add(time.Hour)
	if err := store.UpdateState(context.Background(), update); err != nil {
		t.Fatalf("same state retry: %v", err)
	}
	got, err = store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || !got.EndedAt.Equal(firstEnded) {
		t.Fatalf("retry overwrote terminal time = %+v, %v", got, err)
	}
	differentCode := "other"
	update.LastErrorCode = &differentCode
	if ErrorCodeOf(store.UpdateState(context.Background(), update)) != ErrorIdempotencyConflict {
		t.Fatal("different error retry accepted")
	}
	update.LastErrorCode = &errorCode
	update.Result = nil
	if ErrorCodeOf(store.UpdateState(context.Background(), update)) != ErrorInvalidStateTransition {
		t.Fatal("result clearing accepted")
	}
	if ErrorCodeOf(store.UpdateState(context.Background(), TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: storeNow()})) != ErrorInvalidStateTransition {
		t.Fatal("unrelated current state accepted")
	}
}

func TestTaskStoreRejectsRegressingTimeForEveryRealStateChange(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_state_time_regression")
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}

	err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskCreated, To: TaskReady,
		At: task.UpdatedAt.Add(-time.Nanosecond),
	})
	if ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
		t.Fatalf("regressing state time error = %v", err)
	}
	stored, getErr := store.Get(context.Background(), task.Scope, task.TaskID)
	if getErr != nil || stored.State != TaskCreated || !stored.UpdatedAt.Equal(task.UpdatedAt) {
		t.Fatalf("regressing update changed task = %+v, %v", stored, getErr)
	}
}

func TestTaskStoreReadyToRunningAcceptsFractionalStartAfterWholeSecondCreation(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_fractional_started_at")
	if task.CreatedAt.Nanosecond() != 0 {
		t.Fatalf("test requires a whole-second created_at: %s", task.CreatedAt)
	}
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskCreated, To: TaskReady, At: task.CreatedAt,
	}); err != nil {
		t.Fatal(err)
	}
	startedAt := task.CreatedAt.Add(100 * time.Millisecond)
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: startedAt,
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || stored.StartedAt == nil || !stored.StartedAt.Equal(startedAt) {
		t.Fatalf("fractional started_at = %+v, %v; want %s", stored.StartedAt, err, startedAt)
	}
}

func TestTaskStorePreservesStartedAtAcrossRecovery(t *testing.T) {
	db := openSchemaTestDB(t)
	store := NewSQLiteTaskStore(db)
	task, _ := createReaderRunningSubscriptionTask(t, db, "task_started_at_recovery", testTaskScope(BillingSubscriptionPool))
	startedAt := *task.StartedAt
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskRecoveryRequired, At: storeNow().Add(4 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRecoveryRequired, To: TaskRunning, At: storeNow().Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskRecoveryRequired, At: storeNow().Add(6 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskRecoveryRequired, To: TaskReady, At: storeNow().Add(7 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	resumedAt := storeNow().Add(8 * time.Minute)
	resume := TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: resumedAt,
	}
	if err := store.UpdateState(context.Background(), resume); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), resume); err != nil {
		t.Fatalf("exact recovered running retry: %v", err)
	}
	conflict := resume
	conflict.At = conflict.At.Add(time.Second)
	if ErrorCodeOf(store.UpdateState(context.Background(), conflict)) != ErrorIdempotencyConflict {
		t.Fatal("recovered running retry with a different update time was accepted")
	}
	stored, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil || stored.StartedAt == nil || !stored.StartedAt.Equal(startedAt) || !stored.UpdatedAt.Equal(resumedAt) {
		t.Fatalf("recovered lifecycle = started:%+v updated:%s, %v; want %s/%s", stored.StartedAt, stored.UpdatedAt, err, startedAt, resumedAt)
	}
}

func TestSQLiteCallStoreRoundTripAndCanonicalTerminalRetry(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_call_roundtrip")
	contextValue := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	call := ModelCall{CallID: "call_roundtrip", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
	if err := calls.Prepare(context.Background(), contextValue, call); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(context.Background(), contextValue, call.CallID, storeNow()); err != nil {
		t.Fatal(err)
	}
	count, providerID, errorCode := int64(1), "provider-request", "provider_partial"
	finished := storeNow().Add(time.Minute)
	call.State, call.FinishedAt, call.ProviderRequestID, call.LastErrorCode = CallCompleted, &finished, &providerID, &errorCode
	call.Usage = NormalizedUsage{RequestCount: &count, BreakdownStatus: UsageExact}
	call.RawUsageRedacted = json.RawMessage(`{"usage":{"requests":1}}`)
	if err := calls.Complete(context.Background(), contextValue, call); err != nil {
		t.Fatal(err)
	}
	zone := time.FixedZone("same-instant", 8*60*60)
	canonicalRetry := call
	canonicalFinished := finished.In(zone)
	canonicalRetry.FinishedAt = &canonicalFinished
	if err := calls.Complete(context.Background(), contextValue, canonicalRetry); err != nil {
		t.Fatalf("canonical terminal retry: %v", err)
	}
	items, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil || len(items) != 1 || items[0].ProviderRequestID == nil || *items[0].ProviderRequestID != providerID || items[0].LastErrorCode == nil || *items[0].LastErrorCode != errorCode || !bytesEqual(items[0].RawUsageRedacted, call.RawUsageRedacted) || items[0].StartedAt == nil || items[0].FinishedAt == nil {
		t.Fatalf("call round trip = %+v, %v", items, err)
	}
}

func TestSQLiteCallStorePrepareRejectsCrossFactsAndZeroRows(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks, calls := NewSQLiteTaskStore(db), NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_prepare")
	contextValue := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, task, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	base := ModelCall{CallID: "call_prepare", TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: evidence.EvidenceID, State: CallPrepared}
	for _, item := range []struct {
		name string
		edit func(*ModelCall)
	}{
		{"task", func(call *ModelCall) { call.TaskID = "other" }},
		{"source", func(call *ModelCall) { call.BillingSource = BillingSubscriptionPool }},
		{"scope", func(call *ModelCall) { call.Scope.ProfileID = "other" }},
		{"model", func(call *ModelCall) { call.ModelID = "other" }},
		{"evidence", func(call *ModelCall) { call.PricingEvidenceID = "other" }},
	} {
		t.Run(item.name, func(t *testing.T) {
			call := base
			call.CallID += "_" + item.name
			item.edit(&call)
			if err := calls.Prepare(context.Background(), contextValue, call); err == nil {
				t.Fatal("cross fact accepted")
			}
		})
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM pricing_evidence WHERE evidence_id=?`, evidence.EvidenceID); err != nil {
		t.Fatal(err)
	}
	zeroRows := base
	zeroRows.CallID = "call_zero_rows"
	if ErrorCodeOf(insertOrVerifyPreparedCall(context.Background(), tx, task, zeroRows)) != ErrorInvalidStateTransition {
		t.Fatal("zero-row prepare accepted")
	}
}

type fakeIDRows struct {
	values        []string
	index, closed int
	scanErr, err  error
	closeErr      error
}

func (r *fakeIDRows) Next() bool { return r.index < len(r.values) }
func (r *fakeIDRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*dest[0].(*string) = r.values[r.index]
	r.index++
	return nil
}
func (r *fakeIDRows) Err() error { return r.err }
func (r *fakeIDRows) Close() error {
	r.closed++
	return r.closeErr
}

func TestMigrateNonEmptyV1ToV2AndRollback(t *testing.T) {
	db, err := OpenSQLite(t.TempDir() + "/v1.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Exec(migrationTableSQL); err != nil {
		t.Fatal(err)
	}
	if err := applySchemaV1(ctx, db); err != nil {
		t.Fatal(err)
	}
	insertSchemaTask(t, db, "task_v1", BillingSubscriptionPool)
	if _, err := db.Exec(`INSERT INTO model_calls(call_id,task_id,role,model_id,provider,pricing_evidence_id,state) VALUES ('call_v1','task_v1','main','model_01','fixture','price_task_v1','completed')`); err != nil {
		t.Fatal(err)
	}
	now := stamp(storeNow())
	if _, err := db.Exec(`INSERT INTO usage_reports(idempotency_key,task_id,report_kind,sequence,canonical_payload,state,next_attempt_at,created_at,updated_at) VALUES ('report_v1','task_v1','partial',1,'{}','pending',?,?,?)`, now, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id) VALUES ('report_v1','task_v1','call_v1')`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"model_tasks", "pricing_evidence", "model_calls", "usage_reports", "usage_report_calls"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("preserved %s = %d, %v", table, count, err)
		}
	}
	for _, object := range []string{
		"idx_model_tasks_recoverable", "idx_model_calls_task", "idx_usage_reports_due", "idx_pricing_evidence_task",
		"usage_reports_subscription_only_insert", "usage_reports_subscription_only_update", "model_tasks_keep_subscription_reports",
	} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, object).Scan(&count); err != nil || count != 1 {
			t.Fatalf("preserved schema object %s = %d, %v", object, count, err)
		}
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, role := range []CallRole{CallRoleClassifier, CallRoleTasktreePlanner, CallRoleTitle} {
		if _, err := db.Exec(`INSERT INTO model_calls(call_id,task_id,role,model_id,provider,pricing_evidence_id,state) VALUES (?,?,?,'model_01','fixture','price_task_v1','prepared')`, "call_v2_"+string(role), "task_v1", role); err != nil {
			t.Fatalf("new role %s: %v", role, err)
		}
	}
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	failing, err := OpenSQLite(t.TempDir() + "/fail.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer failing.Close()
	if _, err := failing.Exec(migrationTableSQL); err != nil {
		t.Fatal(err)
	}
	if err := applySchemaV1(ctx, failing); err != nil {
		t.Fatal(err)
	}
	insertSchemaTask(t, failing, "task_fail", BillingNewAPIByok)
	if _, err := failing.Exec(`CREATE TABLE model_calls_v2 (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMigrations(ctx, failing); err == nil {
		t.Fatal("migration conflict accepted")
	}
	var version, countRows int
	if err := failing.QueryRow(`SELECT max(version) FROM model_billing_schema_migrations`).Scan(&version); err != nil || version != SchemaVersionV1 {
		t.Fatalf("version = %d, %v", version, err)
	}
	if err := failing.QueryRow(`SELECT count(*) FROM model_tasks WHERE task_id='task_fail'`).Scan(&countRows); err != nil || countRows != 1 {
		t.Fatalf("V1 row lost: %d, %v", countRows, err)
	}
	if err := failing.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='usage_report_calls_v2'`).Scan(&countRows); err != nil || countRows != 0 {
		t.Fatalf("migration left partial join table: %d, %v", countRows, err)
	}
}

func TestMigrateNonEmptyV2ToV3PreservesDataAndSchema(t *testing.T) {
	db := openV2MigrationDB(t, "v2-to-v3")
	seedV2MigrationData(t, db, "task_v2")

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	assertV2MigrationData(t, db, "task_v2", true)
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM model_billing_schema_migrations`).Scan(&version); err != nil || version != SchemaVersionV6 {
		t.Fatalf("schema version = %d, %v; want %d", version, err, SchemaVersionV6)
	}
	if _, err := db.Exec(`INSERT INTO model_calls
		(call_id,task_id,role,model_id,provider,pricing_evidence_id,state)
		VALUES ('call_guardian_v3','task_v2',?,'model_01','fixture','price_task_v2','prepared')`,
		CallRoleGuardian); err != nil {
		t.Fatalf("insert guardian call after V3: %v", err)
	}

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("repeat migrations: %v", err)
	}
	var migrationCount, callCount int
	if err := db.QueryRow(`SELECT count(*) FROM model_billing_schema_migrations WHERE version=?`, SchemaVersionV3).Scan(&migrationCount); err != nil || migrationCount != 1 {
		t.Fatalf("V3 migration rows = %d, %v", migrationCount, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM model_calls WHERE task_id='task_v2'`).Scan(&callCount); err != nil || callCount != 2 {
		t.Fatalf("calls after repeat = %d, %v", callCount, err)
	}
	assertSchemaObjectsAndForeignKeys(t, db)
}

func TestMigrateV2UsagePendingBackfillsExecutionEnd(t *testing.T) {
	db := openV2MigrationDB(t, "v3-usage-pending-ended-at")
	insertSchemaTask(t, db, "task_v2_usage_pending", BillingSubscriptionPool)
	const transitionAt = "2026-07-11T00:06:00Z"
	if _, err := db.Exec(`UPDATE model_tasks
		SET state='usage_pending', task_result='completed', updated_at=?, ended_at=NULL
		WHERE task_id='task_v2_usage_pending'`, transitionAt); err != nil {
		t.Fatal(err)
	}

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var updatedAt, endedAt string
	if err := db.QueryRow(`SELECT updated_at,ended_at FROM model_tasks WHERE task_id='task_v2_usage_pending'`).Scan(&updatedAt, &endedAt); err != nil {
		t.Fatal(err)
	}
	if updatedAt != transitionAt || endedAt != transitionAt {
		t.Fatalf("usage_pending timestamps = %q/%q, want %q", updatedAt, endedAt, transitionAt)
	}
}

func TestMigrateV5KeepsHistoricalStartedAtUnknown(t *testing.T) {
	db := openV2MigrationDB(t, "v5-started-at")
	states := map[string]TaskState{
		"task_v5_created":  TaskCreated,
		"task_v5_running":  TaskRunning,
		"task_v5_recovery": TaskRecoveryRequired,
	}
	for taskID, state := range states {
		insertSchemaTask(t, db, taskID, BillingNewAPIByok)
		if _, err := db.Exec(`UPDATE model_tasks SET
			initial_model_id='model_01',client_task_type='chat',client_estimated_cost='0',
			state=?,updated_at='2026-07-11T00:01:00Z'
			WHERE task_id=?`, state, taskID); err != nil {
			t.Fatalf("seed historical task %s: %v", taskID, err)
		}
	}
	const subscriptionRecoveryID = "task_v5_subscription_recovery"
	insertSchemaTask(t, db, subscriptionRecoveryID, BillingSubscriptionPool)
	if _, err := db.Exec(`UPDATE model_tasks SET
		initial_model_id='model_01',client_task_type='chat',client_estimated_cost='0',
		freeze_idempotency_key='freeze-request-v5',state=?,updated_at='2026-07-11T00:01:00Z'
		WHERE task_id=?`, TaskRecoveryRequired, subscriptionRecoveryID); err != nil {
		t.Fatalf("seed subscription recovery task: %v", err)
	}

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"task_v5_created", "task_v5_running", "task_v5_recovery", subscriptionRecoveryID} {
		var startedAt sql.NullString
		if err := db.QueryRow(`SELECT started_at FROM model_tasks WHERE task_id=?`, taskID).Scan(&startedAt); err != nil {
			t.Fatal(err)
		}
		if startedAt.Valid {
			t.Fatalf("historical task %s started_at = %q, want NULL", taskID, startedAt.String)
		}
	}

	scope := TaskScope{ActorID: "local", ProfileID: "default"}
	store := NewSQLiteTaskStore(db)
	if err := store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: scope, TaskID: "task_v5_recovery", From: TaskRecoveryRequired, To: TaskRunning,
		At: time.Date(2026, 7, 11, 0, 2, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Get(context.Background(), scope, "task_v5_recovery")
	if err != nil || recovered.StartedAt != nil {
		t.Fatalf("legacy recovery fabricated started_at = %+v, %v", recovered.StartedAt, err)
	}

	subscriptionScope := TaskScope{
		ActorID: "local", ProfileID: "default",
		CloudBinding: &cloud.BindingScope{ProfileID: "default", CloudUserID: "usr_" + subscriptionRecoveryID, BindingGeneration: 1},
	}
	err = store.UpdateState(context.Background(), TaskStateUpdate{
		Scope: subscriptionScope, TaskID: subscriptionRecoveryID,
		From: TaskRecoveryRequired, To: TaskRunning,
		At: time.Date(2026, 7, 11, 0, 2, 0, 0, time.UTC),
	})
	if ErrorCodeOf(err) != ErrorQuotaTimeInvalid {
		t.Fatalf("subscription legacy recovery error = %v", err)
	}
	var persistedState TaskState
	if err := db.QueryRow(`SELECT state FROM model_tasks WHERE task_id=?`, subscriptionRecoveryID).Scan(&persistedState); err != nil {
		t.Fatal(err)
	}
	if persistedState != TaskRecoveryRequired {
		t.Fatalf("subscription legacy recovery state = %s", persistedState)
	}
}

func TestMigrateNonEmptyV5ToV6PreservesScopedCallFacts(t *testing.T) {
	db := openV5MigrationDB(t, "v5-to-v6")
	seedV5MigrationData(t, db, "task_v5_to_v6")

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM model_billing_schema_migrations`).Scan(&version); err != nil || version != SchemaVersionV6 {
		t.Fatalf("schema version = %d, %v; want %d", version, err, SchemaVersionV6)
	}
	assertV5MigrationData(t, db, "task_v5_to_v6")

	callPrimaryKey := schemaPrimaryKey(t, db, "model_calls")
	if callPrimaryKey["task_id"] != 1 || callPrimaryKey["call_id"] != 2 {
		t.Fatalf("model_calls primary key = %v", callPrimaryKey)
	}
	evidencePrimaryKey := schemaPrimaryKey(t, db, "model_call_quota_evidence_revisions")
	if evidencePrimaryKey["task_id"] != 1 || evidencePrimaryKey["call_id"] != 2 || evidencePrimaryKey["evidence_revision"] != 3 {
		t.Fatalf("quota evidence primary key = %v", evidencePrimaryKey)
	}
	for _, object := range []string{
		"idx_model_calls_task", "idx_model_call_quota_evidence_task",
		"model_call_quota_evidence_current", "model_call_quota_evidence_subscription_only",
		"model_call_quota_evidence_next_revision", "model_call_quota_evidence_no_complete_regression",
		"model_call_quota_evidence_final_bound", "model_call_quota_evidence_immutable_update",
		"model_call_quota_evidence_immutable_delete", "usage_report_calls_accepted_immutable_insert",
		"usage_report_calls_accepted_immutable_update", "usage_report_calls_accepted_immutable_delete",
	} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, object).Scan(&count); err != nil || count != 1 {
			t.Fatalf("V6 schema object %s = %d, %v", object, count, err)
		}
	}
	if _, err := db.Exec(`UPDATE model_call_quota_evidence_revisions SET quota='8'
		WHERE task_id='task_v5_to_v6' AND call_id='call_task_v5_to_v6'`); err == nil {
		t.Fatal("V6 schema rewrote migrated quota evidence")
	}
	if _, err := db.Exec(`DELETE FROM usage_report_calls
		WHERE idempotency_key='report_task_v5_to_v6' AND task_id='task_v5_to_v6' AND call_id='call_task_v5_to_v6'`); err == nil {
		t.Fatal("V6 schema deleted a migrated accepted report link")
	}
	assertSchemaObjectsAndForeignKeys(t, db)
}

func TestMigrateV6FailureRollsBackNonEmptyV5(t *testing.T) {
	db := openV5MigrationDB(t, "v6-rollback")
	seedV5MigrationData(t, db, "task_v6_fail")
	if _, err := db.Exec(`CREATE TRIGGER model_call_quota_evidence_final_bound
		BEFORE INSERT ON model_tasks BEGIN SELECT 1; END`); err != nil {
		t.Fatal(err)
	}

	if err := ApplyMigrations(context.Background(), db); err == nil {
		t.Fatal("V6 migration conflict accepted")
	}
	var version int
	if err := db.QueryRow(`SELECT max(version) FROM model_billing_schema_migrations`).Scan(&version); err != nil || version != SchemaVersionV5 {
		t.Fatalf("schema version = %d, %v; want %d", version, err, SchemaVersionV5)
	}
	assertV5MigrationData(t, db, "task_v6_fail")
	for _, table := range []string{"model_calls_v6", "usage_report_calls_v6", "model_call_quota_evidence_revisions_v6"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rolled-back table %s = %d, %v", table, count, err)
		}
	}
	var conflictTriggerCount int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master
		WHERE type='trigger' AND name='model_call_quota_evidence_final_bound'`).Scan(&conflictTriggerCount); err != nil || conflictTriggerCount != 1 {
		t.Fatalf("preexisting V6 conflict trigger = %d, %v", conflictTriggerCount, err)
	}
	callPrimaryKey := schemaPrimaryKey(t, db, "model_calls")
	if callPrimaryKey["call_id"] != 1 || callPrimaryKey["task_id"] != 0 {
		t.Fatalf("rolled-back model_calls primary key = %v", callPrimaryKey)
	}
	if _, err := db.Exec(`UPDATE model_call_quota_evidence_revisions SET quota='8'
		WHERE task_id='task_v6_fail' AND call_id='call_task_v6_fail'`); err == nil {
		t.Fatal("rollback lost the V5 evidence immutability trigger")
	}
	if _, err := db.Exec(`DELETE FROM usage_report_calls
		WHERE idempotency_key='report_task_v6_fail' AND task_id='task_v6_fail' AND call_id='call_task_v6_fail'`); err == nil {
		t.Fatal("rollback lost the V5 accepted-link trigger")
	}
	assertSchemaObjectsAndForeignKeys(t, db)
}

func TestMigrateV3FailureRollsBackNonEmptyV2(t *testing.T) {
	db := openV2MigrationDB(t, "v3-rollback")
	seedV2MigrationData(t, db, "task_v3_fail")
	if _, err := db.Exec(`UPDATE model_tasks
		SET state='usage_pending', task_result='failed', updated_at='2026-07-11T00:06:00Z', ended_at=NULL
		WHERE task_id='task_v3_fail'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE usage_report_calls_v3 (sentinel TEXT)`); err != nil {
		t.Fatal(err)
	}

	if err := ApplyMigrations(context.Background(), db); err == nil {
		t.Fatal("V3 migration conflict accepted")
	}
	var version, count int
	if err := db.QueryRow(`SELECT max(version) FROM model_billing_schema_migrations`).Scan(&version); err != nil || version != SchemaVersionV2 {
		t.Fatalf("schema version = %d, %v; want V2", version, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='model_calls_v3'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled-back model_calls_v3 count = %d, %v", count, err)
	}
	var state, updatedAt string
	var endedAt sql.NullString
	if err := db.QueryRow(`SELECT state,updated_at,ended_at FROM model_tasks WHERE task_id='task_v3_fail'`).Scan(&state, &updatedAt, &endedAt); err != nil {
		t.Fatal(err)
	}
	if state != string(TaskUsagePending) || updatedAt != "2026-07-11T00:06:00Z" || endedAt.Valid {
		t.Fatalf("rolled-back task timestamps = %q/%q/%+v", state, updatedAt, endedAt)
	}
	assertV2MigrationData(t, db, "task_v3_fail", false)
	assertSchemaObjectsAndForeignKeys(t, db)
}

func openV2MigrationDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := OpenSQLite(t.TempDir() + "/" + name + ".sqlite")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.Exec(migrationTableSQL); err != nil {
		t.Fatal(err)
	}
	if err := applySchemaV1(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := applySchemaV2(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func openV5MigrationDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := OpenSQLite(t.TempDir() + "/" + name + ".sqlite")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.Exec(migrationTableSQL); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []func(context.Context, *sql.DB) error{
		applySchemaV1, applySchemaV2, applySchemaV3, applySchemaV4, applySchemaV5,
	} {
		if err := migration(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedV5MigrationData(t *testing.T, db *sql.DB, taskID string) {
	t.Helper()
	insertSchemaTask(t, db, taskID, BillingSubscriptionPool)
	if _, err := db.Exec(`UPDATE model_tasks SET
		initial_model_id='model_01',client_task_type='chat',client_estimated_cost='0',
		state='ready',updated_at='2026-07-11T00:01:00Z' WHERE task_id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET state='running',started_at='2026-07-11T00:02:00Z',
		updated_at='2026-07-11T00:02:00Z' WHERE task_id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	pricingID := "price_" + taskID + "_model_02"
	if _, err := db.Exec(`INSERT INTO pricing_evidence
		(evidence_id,snapshot_id,task_id,model_id,pricing_group,pricing_version,pricing_hash,state,captured_at)
		VALUES (?,'snapshot_v5',?,'model_02','group_v5','version_v5','hash_v5','fresh','2026-07-11T00:01:00Z')`,
		pricingID, taskID); err != nil {
		t.Fatal(err)
	}
	callID := "call_" + taskID
	if _, err := db.Exec(`INSERT INTO model_calls (
		call_id,task_id,role,model_id,provider,provider_request_id,pricing_evidence_id,state,attempt_count,
		normalized_usage_json,raw_usage_redacted_json,started_at,finished_at,last_error_code,
		endpoint,occurred_at,quota_report_status,quota_report_last_error_code
	) VALUES (?,?,'guardian','model_02','fixture','request_v5',?,'failed',2,
		'{"breakdown_status":"unavailable"}','{"error":"redacted"}',
		'2026-07-11T00:03:00Z','2026-07-11T00:04:00Z','gateway_unavailable',
		'/v1/chat/completions','2026-07-11T00:04:00Z','final_reported','quota_retryable')`,
		callID, taskID, pricingID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO model_call_quota_evidence_revisions (
		call_id,task_id,evidence_revision,oneapi_request_id,
		billing_quota_present,billing_quota_type,billing_quota_value,
		usage_quota_present,usage_quota_type,usage_quota_value,
		quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
	) VALUES (?,?,1,'request_v5',1,'number','7',0,NULL,NULL,'7','billing.quota','complete','[]','2026-07-11T00:04:00Z')`,
		callID, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO model_call_quota_evidence_revisions (
		call_id,task_id,evidence_revision,oneapi_request_id,
		billing_quota_present,billing_quota_type,billing_quota_value,
		usage_quota_present,usage_quota_type,usage_quota_value,
		quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
	) VALUES (?,?,2,'request_v5',1,'number','8',0,NULL,NULL,'8','billing.quota','complete','[]','2026-07-11T00:04:30Z')`,
		callID, taskID); err != nil {
		t.Fatal(err)
	}
	reportID := "report_" + taskID
	if _, err := db.Exec(`INSERT INTO usage_reports (
		idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,next_attempt_at,
		created_at,updated_at,contract_version,payload_hash
	) VALUES (?,?,'partial',1,'{"task":"v5"}','pending',1,'2026-07-11T00:05:00Z',
		'2026-07-11T00:04:00Z','2026-07-11T00:04:00Z','quota-contract-v1',
		'0000000000000000000000000000000000000000000000000000000000000000')`, reportID, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id) VALUES (?,?,?)`,
		reportID, taskID, callID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE usage_reports SET state='accepted',next_attempt_at=NULL,
		updated_at='2026-07-11T00:05:00Z' WHERE idempotency_key=?`, reportID); err != nil {
		t.Fatal(err)
	}
}

func assertV5MigrationData(t *testing.T, db *sql.DB, taskID string) {
	t.Helper()
	var taskState, taskStartedAt string
	if err := db.QueryRow(`SELECT state,started_at FROM model_tasks WHERE task_id=?`, taskID).Scan(&taskState, &taskStartedAt); err != nil {
		t.Fatal(err)
	}
	if taskState != string(TaskRunning) || taskStartedAt != "2026-07-11T00:02:00Z" {
		t.Fatalf("task V5 facts = %q/%q", taskState, taskStartedAt)
	}
	callID := "call_" + taskID
	var role, modelID, provider, requestID, pricingID, state, normalized, raw, startedAt, finishedAt string
	var lastError, endpoint, occurredAt, reportStatus, reportError string
	var attemptCount int
	if err := db.QueryRow(`SELECT role,model_id,provider,provider_request_id,pricing_evidence_id,state,attempt_count,
		CAST(normalized_usage_json AS TEXT),CAST(raw_usage_redacted_json AS TEXT),started_at,finished_at,last_error_code,
		endpoint,occurred_at,quota_report_status,quota_report_last_error_code
		FROM model_calls WHERE task_id=? AND call_id=?`, taskID, callID).Scan(
		&role, &modelID, &provider, &requestID, &pricingID, &state, &attemptCount,
		&normalized, &raw, &startedAt, &finishedAt, &lastError,
		&endpoint, &occurredAt, &reportStatus, &reportError,
	); err != nil {
		t.Fatal(err)
	}
	if role != string(CallRoleGuardian) || modelID != "model_02" || provider != "fixture" || requestID != "request_v5" ||
		pricingID != "price_"+taskID+"_model_02" || state != string(CallFailed) || attemptCount != 2 ||
		normalized != `{"breakdown_status":"unavailable"}` || raw != `{"error":"redacted"}` ||
		startedAt != "2026-07-11T00:03:00Z" || finishedAt != "2026-07-11T00:04:00Z" || lastError != "gateway_unavailable" ||
		endpoint != string(MaasEndpointChatCompletions) || occurredAt != "2026-07-11T00:04:00Z" ||
		reportStatus != string(QuotaReportFinalReported) || reportError != "quota_retryable" {
		t.Fatalf("call V5 facts changed: %q/%q/%q/%q/%q/%q/%d/%q/%q/%q/%q/%q/%q/%q/%q/%q",
			role, modelID, provider, requestID, pricingID, state, attemptCount, normalized, raw,
			startedAt, finishedAt, lastError, endpoint, occurredAt, reportStatus, reportError)
	}
	var revision int
	var evidenceTask, evidenceCall, evidenceRequest, quota, evidenceStatus string
	if err := db.QueryRow(`SELECT task_id,call_id,evidence_revision,oneapi_request_id,quota,evidence_status
		FROM model_call_quota_evidence_current WHERE task_id=? AND call_id=?`, taskID, callID).Scan(
		&evidenceTask, &evidenceCall, &revision, &evidenceRequest, &quota, &evidenceStatus,
	); err != nil {
		t.Fatal(err)
	}
	if evidenceTask != taskID || evidenceCall != callID || revision != 2 || evidenceRequest != "request_v5" ||
		quota != "8" || evidenceStatus != string(QuotaEvidenceComplete) {
		t.Fatalf("quota evidence V5 facts = %q/%q/%d/%q/%q/%q", evidenceTask, evidenceCall, revision, evidenceRequest, quota, evidenceStatus)
	}
	var evidenceCount int
	if err := db.QueryRow(`SELECT count(*) FROM model_call_quota_evidence_revisions
		WHERE task_id=? AND call_id=?`, taskID, callID).Scan(&evidenceCount); err != nil || evidenceCount != 2 {
		t.Fatalf("quota evidence V5 history count = %d, %v", evidenceCount, err)
	}
	var reportTask, reportCall, reportState, contractVersion, payloadHash string
	if err := db.QueryRow(`SELECT rc.task_id,rc.call_id,r.state,r.contract_version,r.payload_hash
		FROM usage_report_calls rc JOIN usage_reports r
		ON r.idempotency_key=rc.idempotency_key AND r.task_id=rc.task_id
		WHERE rc.idempotency_key=?`, "report_"+taskID).Scan(
		&reportTask, &reportCall, &reportState, &contractVersion, &payloadHash,
	); err != nil {
		t.Fatal(err)
	}
	if reportTask != taskID || reportCall != callID || reportState != string(ReportAccepted) ||
		contractVersion != QuotaContractV1 || len(payloadHash) != 64 {
		t.Fatalf("report link V5 facts = %q/%q/%q/%q/%q", reportTask, reportCall, reportState, contractVersion, payloadHash)
	}
}

func schemaPrimaryKey(t *testing.T, db *sql.DB, table string) map[string]int {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	primaryKey := make(map[string]int)
	for rows.Next() {
		var cid, notNull, order int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &order); err != nil {
			t.Fatal(err)
		}
		if order != 0 {
			primaryKey[name] = order
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return primaryKey
}

func seedV2MigrationData(t *testing.T, db *sql.DB, taskID string) {
	t.Helper()
	insertSchemaTask(t, db, taskID, BillingSubscriptionPool)
	secondEvidence := "price_" + taskID + "_model_02"
	if _, err := db.Exec(`INSERT INTO pricing_evidence
		(evidence_id,snapshot_id,task_id,model_id,pricing_group,pricing_version,pricing_hash,state,captured_at)
		VALUES (?,'snapshot_v2',?,'model_02','group_v2','version_v2','hash_v2','stale_usable','2026-07-11T00:01:00Z')`,
		secondEvidence, taskID); err != nil {
		t.Fatal(err)
	}
	callID := "call_" + taskID
	if _, err := db.Exec(`INSERT INTO model_calls
		(call_id,task_id,role,model_id,provider,provider_request_id,pricing_evidence_id,state,attempt_count,
		 normalized_usage_json,raw_usage_redacted_json,started_at,finished_at,last_error_code)
		VALUES (?,?,'title','model_02','fixture','request_v2',?,'failed',2,?,?,
		 '2026-07-11T00:02:00Z','2026-07-11T00:03:00Z','gateway_unavailable')`,
		callID, taskID, secondEvidence, []byte(`{"breakdown_status":"unavailable"}`), []byte(`{"error":"redacted"}`)); err != nil {
		t.Fatal(err)
	}
	reportID := "report_" + taskID
	if _, err := db.Exec(`INSERT INTO usage_reports
		(idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,next_attempt_at,last_error_code,created_at,updated_at)
		VALUES (?,?,'partial',2,?,'retry',2,'2026-07-11T00:05:00Z','usage_retryable',
		 '2026-07-11T00:03:00Z','2026-07-11T00:04:00Z')`, reportID, taskID, []byte(`{"task":"v2"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO usage_report_calls(idempotency_key,task_id,call_id) VALUES (?,?,?)`, reportID, taskID, callID); err != nil {
		t.Fatal(err)
	}
}

func assertV2MigrationData(t *testing.T, db *sql.DB, taskID string, migratedV4 bool) {
	t.Helper()
	expectedScope := TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &cloud.BindingScope{
		ProfileID: "default", CloudUserID: "usr_" + taskID, BindingGeneration: 1,
	}}
	expectedScopeKey, err := BuildTaskScopeKey(expectedScope, BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	var scopeKey, source, primaryEvidence string
	if err := db.QueryRow(`SELECT scope_key,billing_source,pricing_evidence_id FROM model_tasks WHERE task_id=?`, taskID).Scan(&scopeKey, &source, &primaryEvidence); err != nil {
		t.Fatal(err)
	}
	if scopeKey != expectedScopeKey || source != string(BillingSubscriptionPool) || primaryEvidence != schemaEvidenceID(taskID) {
		t.Fatalf("task ownership changed: %q/%q/%q", scopeKey, source, primaryEvidence)
	}
	var evidenceCount int
	if err := db.QueryRow(`SELECT count(*) FROM pricing_evidence WHERE task_id=?`, taskID).Scan(&evidenceCount); err != nil || evidenceCount != 2 {
		t.Fatalf("evidence rows = %d, %v", evidenceCount, err)
	}
	var snapshotID, evidenceTask, evidenceModel, group, version, hash, evidenceState, captured string
	if err := db.QueryRow(`SELECT snapshot_id,task_id,model_id,pricing_group,pricing_version,pricing_hash,state,captured_at
		FROM pricing_evidence WHERE evidence_id=?`, "price_"+taskID+"_model_02").Scan(
		&snapshotID, &evidenceTask, &evidenceModel, &group, &version, &hash, &evidenceState, &captured); err != nil {
		t.Fatal(err)
	}
	if snapshotID != "snapshot_v2" || evidenceTask != taskID || evidenceModel != "model_02" || group != "group_v2" ||
		version != "version_v2" || hash != "hash_v2" || evidenceState != string(PricingStaleUsable) || captured != "2026-07-11T00:01:00Z" {
		t.Fatalf("evidence changed: %q/%q/%q/%q/%q/%q/%q/%q", snapshotID, evidenceTask, evidenceModel, group, version, hash, evidenceState, captured)
	}
	var role, modelID, provider, requestID, evidenceID, state, normalized, raw, started, finished, lastError string
	var attempts int
	if err := db.QueryRow(`SELECT role,model_id,provider,provider_request_id,pricing_evidence_id,state,attempt_count,
		CAST(normalized_usage_json AS TEXT),CAST(raw_usage_redacted_json AS TEXT),started_at,finished_at,last_error_code
		FROM model_calls WHERE call_id=?`, "call_"+taskID).Scan(
		&role, &modelID, &provider, &requestID, &evidenceID, &state, &attempts,
		&normalized, &raw, &started, &finished, &lastError); err != nil {
		t.Fatal(err)
	}
	if role != string(CallRoleTitle) || modelID != "model_02" || provider != "fixture" || requestID != "request_v2" ||
		evidenceID != "price_"+taskID+"_model_02" || state != string(CallFailed) || attempts != 2 ||
		normalized != `{"breakdown_status":"unavailable"}` || raw != `{"error":"redacted"}` ||
		started != "2026-07-11T00:02:00Z" || finished != "2026-07-11T00:03:00Z" || lastError != "gateway_unavailable" {
		t.Fatalf("call changed: %q/%q/%q/%q/%q/%q/%d/%q/%q/%q/%q/%q", role, modelID, provider, requestID, evidenceID, state, attempts, normalized, raw, started, finished, lastError)
	}
	var reportTask, kind, payload, reportState, reportError, created, updated, contractVersion string
	var next sql.NullString
	var sequence, reportAttempts int
	columns := `task_id,report_kind,sequence,CAST(canonical_payload AS TEXT),state,attempt_count,
		next_attempt_at,last_error_code,created_at,updated_at`
	destinations := []any{&reportTask, &kind, &sequence, &payload, &reportState, &reportAttempts, &next, &reportError, &created, &updated}
	if migratedV4 {
		columns += `,contract_version`
		destinations = append(destinations, &contractVersion)
	}
	if err := db.QueryRow(`SELECT `+columns+` FROM usage_reports WHERE idempotency_key=?`, "report_"+taskID).Scan(destinations...); err != nil {
		t.Fatal(err)
	}
	if !migratedV4 {
		contractVersion = LegacyTokenContractV1
	}
	if reportTask != taskID || kind != string(ReportPartial) || sequence != 2 || payload != `{"task":"v2"}` ||
		reportState != map[bool]string{true: string(ReportBlocked), false: string(ReportRetry)}[migratedV4] || reportAttempts != 2 ||
		next.Valid != !migratedV4 || reportError != map[bool]string{true: "legacy_token_report_contract", false: "usage_retryable"}[migratedV4] ||
		created != "2026-07-11T00:03:00Z" || updated != "2026-07-11T00:04:00Z" ||
		contractVersion != LegacyTokenContractV1 {
		t.Fatalf("report migration changed unexpected fields: %q/%q/%d/%q/%q/%d/%v/%q/%q/%q/%q", reportTask, kind, sequence, payload, reportState, reportAttempts, next, reportError, created, updated, contractVersion)
	}
	var joinedTask, joinedCall string
	if err := db.QueryRow(`SELECT task_id,call_id FROM usage_report_calls WHERE idempotency_key=?`, "report_"+taskID).Scan(&joinedTask, &joinedCall); err != nil || joinedTask != taskID || joinedCall != "call_"+taskID {
		t.Fatalf("report join changed: %q/%q, %v", joinedTask, joinedCall, err)
	}
}

func assertSchemaObjectsAndForeignKeys(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, object := range []string{
		"idx_model_tasks_recoverable", "idx_model_calls_task", "idx_usage_reports_due", "idx_pricing_evidence_task",
		"usage_reports_subscription_only_insert", "usage_reports_subscription_only_update", "model_tasks_keep_subscription_reports",
	} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, object).Scan(&count); err != nil || count != 1 {
			t.Fatalf("schema object %s = %d, %v", object, count, err)
		}
	}
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

var _ = sql.ErrNoRows
