package modelbillingworker

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

var startupRecoveryNow = time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)

type startupTaskStoreFake struct {
	tasks           map[string]modelbilling.ModelTask
	getErrors       map[string]error
	updateErrors    map[string]error
	quarantineError error
	updates         []modelbilling.TaskStateUpdate
	quarantined     []string
	getCalls        map[string]int
}

func (s *startupTaskStoreFake) ListStartupRecoveryIDs(_ context.Context, scope modelbilling.TaskScope) ([]string, error) {
	var ids []string
	for id, task := range s.tasks {
		if task.Scope.Equal(scope) && startupTaskState(task.State) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *startupTaskStoreFake) Get(_ context.Context, scope modelbilling.TaskScope, taskID string) (modelbilling.ModelTask, error) {
	if s.getCalls == nil {
		s.getCalls = make(map[string]int)
	}
	s.getCalls[taskID]++
	if err := s.getErrors[taskID]; err != nil {
		return modelbilling.ModelTask{}, err
	}
	task, ok := s.tasks[taskID]
	if !ok {
		return modelbilling.ModelTask{}, errors.New("missing task")
	}
	if !task.Scope.Equal(scope) {
		return modelbilling.ModelTask{}, errors.New("scope mismatch")
	}
	return task, nil
}

func (s *startupTaskStoreFake) UpdateState(_ context.Context, update modelbilling.TaskStateUpdate) error {
	if err := s.updateErrors[update.TaskID]; err != nil {
		return err
	}
	task := s.tasks[update.TaskID]
	if task.State != update.From {
		return errors.New("state mismatch")
	}
	task.State = update.To
	task.Result = update.Result
	task.LastErrorCode = update.LastErrorCode
	task.UpdatedAt = update.At
	if update.To == modelbilling.TaskUsagePending || update.To == modelbilling.TaskFailed {
		ended := update.At
		task.EndedAt = &ended
	}
	s.tasks[update.TaskID] = task
	s.updates = append(s.updates, update)
	return nil
}

func (s *startupTaskStoreFake) QuarantineStartupTask(_ context.Context, scope modelbilling.TaskScope, taskID string, at time.Time, code string) error {
	if s.quarantineError != nil {
		return s.quarantineError
	}
	task := s.tasks[taskID]
	if !task.Scope.Equal(scope) {
		return errors.New("scope mismatch")
	}
	if task.State == modelbilling.TaskCreated {
		result := modelbilling.TaskResultFailed
		task.State, task.Result = modelbilling.TaskFailed, &result
		ended := at
		task.EndedAt = &ended
	} else {
		task.State = modelbilling.TaskRecoveryRequired
	}
	task.UpdatedAt = at
	task.LastErrorCode = &code
	s.tasks[taskID] = task
	s.quarantined = append(s.quarantined, taskID)
	return nil
}

type startupCallStoreFake struct {
	calls         map[string][]modelbilling.ModelCall
	listErrors    map[string]error
	completeError map[string]error
	completed     []modelbilling.ModelCall
}

func (s *startupCallStoreFake) ListByTask(_ context.Context, _ modelbilling.TaskScope, taskID string) ([]modelbilling.ModelCall, error) {
	if err := s.listErrors[taskID]; err != nil {
		return nil, err
	}
	return append([]modelbilling.ModelCall(nil), s.calls[taskID]...), nil
}

func (s *startupCallStoreFake) Complete(_ context.Context, _ modelbilling.TaskContext, call modelbilling.ModelCall) error {
	if err := s.completeError[call.CallID]; err != nil {
		return err
	}
	s.completed = append(s.completed, call)
	calls := s.calls[call.TaskID]
	for i := range calls {
		if calls[i].CallID == call.CallID {
			calls[i] = call
		}
	}
	s.calls[call.TaskID] = calls
	return nil
}

func startupTaskState(state modelbilling.TaskState) bool {
	switch state {
	case modelbilling.TaskCreated, modelbilling.TaskFreezePending, modelbilling.TaskReady, modelbilling.TaskRunning:
		return true
	default:
		return false
	}
}

func startupSubscriptionScope(generation uint64) modelbilling.TaskScope {
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "usr-startup", BindingGeneration: generation}
	return modelbilling.TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}
}

func startupTask(id string, scope modelbilling.TaskScope, state modelbilling.TaskState) modelbilling.ModelTask {
	source := modelbilling.BillingNewAPIByok
	if scope.CloudBinding != nil {
		source = modelbilling.BillingSubscriptionPool
	}
	return modelbilling.ModelTask{
		TaskID: id, Scope: scope, BillingSource: source, InitialModelID: "model-startup",
		ClientTaskType: "chat", ClientEstimatedCost: "0", PricingEvidenceID: "price-" + id,
		State: state, CreatedAt: startupRecoveryNow.Add(-time.Hour), UpdatedAt: startupRecoveryNow.Add(-time.Minute),
	}
}

func newStartupRecoveryForTest(t *testing.T, tasks *startupTaskStoreFake, calls *startupCallStoreFake) *StartupExecutionRecovery {
	t.Helper()
	recovery, err := NewStartupExecutionRecovery(tasks, calls, func() time.Time { return startupRecoveryNow })
	if err != nil {
		t.Fatal(err)
	}
	return recovery
}

func TestStartupExecutionRecoveryRecoversPreExecutionWindows(t *testing.T) {
	tests := []struct {
		name       string
		scope      modelbilling.TaskScope
		state      modelbilling.TaskState
		wantState  modelbilling.TaskState
		wantResult *modelbilling.TaskResult
	}{
		{name: "subscription created", scope: startupSubscriptionScope(1), state: modelbilling.TaskCreated, wantState: modelbilling.TaskFailed, wantResult: taskResult(modelbilling.TaskResultFailed)},
		{name: "subscription freeze pending", scope: startupSubscriptionScope(1), state: modelbilling.TaskFreezePending, wantState: modelbilling.TaskRecoveryRequired},
		{name: "subscription ready", scope: startupSubscriptionScope(1), state: modelbilling.TaskReady, wantState: modelbilling.TaskRecoveryRequired},
		{name: "BYOK created", scope: modelbilling.TaskScope{ActorID: "local", ProfileID: "default"}, state: modelbilling.TaskCreated, wantState: modelbilling.TaskFailed, wantResult: taskResult(modelbilling.TaskResultFailed)},
		{name: "BYOK ready", scope: modelbilling.TaskScope{ActorID: "local", ProfileID: "default"}, state: modelbilling.TaskReady, wantState: modelbilling.TaskRecoveryRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := startupTask("task-window", tt.scope, tt.state)
			tasks := &startupTaskStoreFake{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			calls := &startupCallStoreFake{}
			if err := newStartupRecoveryForTest(t, tasks, calls).RecoverScope(context.Background(), tt.scope); err != nil {
				t.Fatal(err)
			}
			got := tasks.tasks[task.TaskID]
			if got.State != tt.wantState || !sameTaskResult(got.Result, tt.wantResult) {
				t.Fatalf("task = state %s result %v, want %s/%v", got.State, got.Result, tt.wantState, tt.wantResult)
			}
			if got.LastErrorCode == nil || *got.LastErrorCode != StartupRecoveryErrorCode {
				t.Fatalf("last_error_code = %v", got.LastErrorCode)
			}
		})
	}
}

func TestStartupExecutionRecoveryDoesNotFabricateUnknownStartedAt(t *testing.T) {
	scope := startupSubscriptionScope(1)
	task := startupTask("task-no-start", scope, modelbilling.TaskRunning)
	tasks := &startupTaskStoreFake{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	calls := &startupCallStoreFake{calls: map[string][]modelbilling.ModelCall{task.TaskID: {{CallID: "call-running", TaskID: task.TaskID, State: modelbilling.CallRunning}}}}
	if err := newStartupRecoveryForTest(t, tasks, calls).RecoverScope(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	got := tasks.tasks[task.TaskID]
	if got.State != modelbilling.TaskRecoveryRequired || got.StartedAt != nil {
		t.Fatalf("task = state %s started_at %v", got.State, got.StartedAt)
	}
	if len(calls.completed) != 0 {
		t.Fatalf("completed calls = %d, want 0", len(calls.completed))
	}
}

func TestStartupExecutionRecoveryInterruptsOnlyRunningCallsThenEndsTaskLocally(t *testing.T) {
	for _, source := range []modelbilling.BillingSource{modelbilling.BillingSubscriptionPool, modelbilling.BillingNewAPIByok} {
		t.Run(string(source), func(t *testing.T) {
			scope := modelbilling.TaskScope{ActorID: "local", ProfileID: "default"}
			if source == modelbilling.BillingSubscriptionPool {
				scope = startupSubscriptionScope(1)
			}
			task := startupTask("task-running-"+string(source), scope, modelbilling.TaskRunning)
			started := startupRecoveryNow.Add(-30 * time.Minute)
			task.StartedAt = &started
			runningStarted := started.Add(time.Minute)
			running := modelbilling.ModelCall{CallID: "call-running", TaskID: task.TaskID, Scope: scope, BillingSource: source, Role: modelbilling.CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture", PricingEvidenceID: task.PricingEvidenceID, State: modelbilling.CallRunning, AttemptCount: 1, StartedAt: &runningStarted}
			prepared := running
			prepared.CallID, prepared.State, prepared.AttemptCount, prepared.StartedAt = "call-prepared", modelbilling.CallPrepared, 0, nil
			tasks := &startupTaskStoreFake{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			calls := &startupCallStoreFake{calls: map[string][]modelbilling.ModelCall{task.TaskID: {running, prepared}}}
			if err := newStartupRecoveryForTest(t, tasks, calls).RecoverScope(context.Background(), scope); err != nil {
				t.Fatal(err)
			}
			if len(calls.completed) != 1 || calls.completed[0].CallID != running.CallID || calls.completed[0].State != modelbilling.CallInterrupted {
				t.Fatalf("completed calls = %#v", calls.completed)
			}
			completed := calls.completed[0]
			if completed.FinishedAt == nil || !completed.FinishedAt.Equal(startupRecoveryNow) || completed.LastErrorCode == nil || *completed.LastErrorCode != StartupRecoveryErrorCode {
				t.Fatalf("interrupted call facts = %#v", completed)
			}
			if calls.calls[task.TaskID][1].State != modelbilling.CallPrepared {
				t.Fatalf("prepared call changed to %s", calls.calls[task.TaskID][1].State)
			}
			got := tasks.tasks[task.TaskID]
			wantState := modelbilling.TaskFailed
			if source == modelbilling.BillingSubscriptionPool {
				wantState = modelbilling.TaskUsagePending
			}
			if got.State != wantState || got.Result == nil || *got.Result != modelbilling.TaskResultFailed || got.StartedAt == nil || !got.StartedAt.Equal(started) {
				t.Fatalf("task after recovery = %#v", got)
			}
		})
	}
}

func TestStartupExecutionRecoveryQuarantinesDamagedTaskAndContinuesWithoutHotLoop(t *testing.T) {
	scope := startupSubscriptionScope(7)
	bad := startupTask("task-bad", scope, modelbilling.TaskReady)
	good := startupTask("task-good", scope, modelbilling.TaskCreated)
	tasks := &startupTaskStoreFake{
		tasks:     map[string]modelbilling.ModelTask{bad.TaskID: bad, good.TaskID: good},
		getErrors: map[string]error{bad.TaskID: &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "corrupt persisted identity"}},
	}
	recovery := newStartupRecoveryForTest(t, tasks, &startupCallStoreFake{})
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatalf("damaged task recovery = %v", err)
	}
	if tasks.tasks[bad.TaskID].State != modelbilling.TaskRecoveryRequired || tasks.tasks[good.TaskID].State != modelbilling.TaskFailed {
		t.Fatalf("states = bad:%s good:%s", tasks.tasks[bad.TaskID].State, tasks.tasks[good.TaskID].State)
	}
	if len(tasks.quarantined) != 1 || tasks.quarantined[0] != bad.TaskID {
		t.Fatalf("quarantined = %v", tasks.quarantined)
	}
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatalf("idempotent second pass: %v", err)
	}
	if tasks.getCalls[bad.TaskID] != 1 || len(tasks.updates) != 1 {
		t.Fatalf("second pass repeated work: gets=%v updates=%d", tasks.getCalls, len(tasks.updates))
	}
}

func TestStartupExecutionRecoveryQuarantinesRunningTaskWhenCallPersistenceFails(t *testing.T) {
	scope := startupSubscriptionScope(1)
	task := startupTask("task-call-error", scope, modelbilling.TaskRunning)
	started := startupRecoveryNow.Add(-time.Minute)
	task.StartedAt = &started
	call := modelbilling.ModelCall{CallID: "call-error", TaskID: task.TaskID, State: modelbilling.CallRunning}
	tasks := &startupTaskStoreFake{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	calls := &startupCallStoreFake{calls: map[string][]modelbilling.ModelCall{task.TaskID: {call}}, completeError: map[string]error{call.CallID: &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "corrupt call row"}}}
	if err := newStartupRecoveryForTest(t, tasks, calls).RecoverScope(context.Background(), scope); err != nil {
		t.Fatalf("quarantined call persistence failure = %v", err)
	}
	if got := tasks.tasks[task.TaskID]; got.State != modelbilling.TaskRecoveryRequired || got.StartedAt == nil || !got.StartedAt.Equal(started) {
		t.Fatalf("quarantined task = %#v", got)
	}
}

func TestStartupExecutionRecoveryReturnsQuarantineFailure(t *testing.T) {
	scope := startupSubscriptionScope(1)
	task := startupTask("task-quarantine-failure", scope, modelbilling.TaskReady)
	quarantineErr := errors.New("quarantine write failed")
	tasks := &startupTaskStoreFake{
		tasks: map[string]modelbilling.ModelTask{task.TaskID: task},
		getErrors: map[string]error{task.TaskID: &modelbilling.Error{
			Code: modelbilling.ErrorContractInvalid, Message: "corrupt task row",
		}},
		quarantineError: quarantineErr,
	}
	err := newStartupRecoveryForTest(t, tasks, &startupCallStoreFake{}).RecoverScope(context.Background(), scope)
	if !errors.Is(err, quarantineErr) {
		t.Fatalf("quarantine failure = %v, want %v", err, quarantineErr)
	}
}

func TestStartupExecutionRecoveryLeavesTransientStoreFailuresForRetry(t *testing.T) {
	transient := errors.New("sqlite busy")
	tests := []struct {
		name    string
		setup   func(modelbilling.TaskScope, modelbilling.ModelTask, *startupTaskStoreFake, *startupCallStoreFake)
		state   modelbilling.TaskState
		started bool
	}{
		{
			name: "get",
			setup: func(_ modelbilling.TaskScope, task modelbilling.ModelTask, tasks *startupTaskStoreFake, _ *startupCallStoreFake) {
				tasks.getErrors = map[string]error{task.TaskID: transient}
			},
			state: modelbilling.TaskReady,
		},
		{
			name: "list calls",
			setup: func(_ modelbilling.TaskScope, task modelbilling.ModelTask, _ *startupTaskStoreFake, calls *startupCallStoreFake) {
				calls.listErrors = map[string]error{task.TaskID: transient}
			},
			state: modelbilling.TaskRunning, started: true,
		},
		{
			name: "complete call",
			setup: func(scope modelbilling.TaskScope, task modelbilling.ModelTask, _ *startupTaskStoreFake, calls *startupCallStoreFake) {
				call := modelbilling.ModelCall{CallID: "call-transient", TaskID: task.TaskID, Scope: scope, BillingSource: task.BillingSource, State: modelbilling.CallRunning}
				calls.calls = map[string][]modelbilling.ModelCall{task.TaskID: {call}}
				calls.completeError = map[string]error{call.CallID: transient}
			},
			state: modelbilling.TaskRunning, started: true,
		},
		{
			name: "update task",
			setup: func(_ modelbilling.TaskScope, task modelbilling.ModelTask, tasks *startupTaskStoreFake, _ *startupCallStoreFake) {
				tasks.updateErrors = map[string]error{task.TaskID: transient}
			},
			state: modelbilling.TaskCreated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := startupSubscriptionScope(1)
			task := startupTask("task-transient", scope, tt.state)
			if tt.started {
				started := startupRecoveryNow.Add(-time.Minute)
				task.StartedAt = &started
			}
			tasks := &startupTaskStoreFake{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			calls := &startupCallStoreFake{}
			tt.setup(scope, task, tasks, calls)
			if err := newStartupRecoveryForTest(t, tasks, calls).RecoverScope(context.Background(), scope); !errors.Is(err, transient) {
				t.Fatalf("error = %v, want transient cause", err)
			}
			if len(tasks.quarantined) != 0 {
				t.Fatalf("transient failure quarantined tasks = %v", tasks.quarantined)
			}
			if got := tasks.tasks[task.TaskID]; got.State != tt.state {
				t.Fatalf("task state = %s, want preserved %s", got.State, tt.state)
			}
		})
	}
}

func TestStartupExecutionRecoveryDoesNotQuarantineConcurrentCASDrift(t *testing.T) {
	scope := startupSubscriptionScope(1)
	task := startupTask("task-cas-drift", scope, modelbilling.TaskReady)
	casErr := &modelbilling.Error{Code: modelbilling.ErrorInvalidStateTransition, Message: "state changed concurrently"}
	tasks := &startupTaskStoreFake{
		tasks:        map[string]modelbilling.ModelTask{task.TaskID: task},
		updateErrors: map[string]error{task.TaskID: casErr},
	}
	if err := newStartupRecoveryForTest(t, tasks, &startupCallStoreFake{}).RecoverScope(context.Background(), scope); !errors.Is(err, casErr) {
		t.Fatalf("error = %v, want CAS drift", err)
	}
	if len(tasks.quarantined) != 0 || tasks.tasks[task.TaskID].State != modelbilling.TaskReady {
		t.Fatalf("CAS drift changed task: quarantined=%v state=%s", tasks.quarantined, tasks.tasks[task.TaskID].State)
	}
}

func TestStartupExecutionRecoveryZeroCallRunningDoesNotCreateRemoteFacts(t *testing.T) {
	scope := startupSubscriptionScope(1)
	task := startupTask("task-zero-call", scope, modelbilling.TaskRunning)
	started := startupRecoveryNow.Add(-time.Minute)
	task.StartedAt = &started
	tasks := &startupTaskStoreFake{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	calls := &startupCallStoreFake{calls: map[string][]modelbilling.ModelCall{task.TaskID: nil}}
	if err := newStartupRecoveryForTest(t, tasks, calls).RecoverScope(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	got := tasks.tasks[task.TaskID]
	if got.State != modelbilling.TaskUsagePending || got.Result == nil || *got.Result != modelbilling.TaskResultFailed || len(calls.completed) != 0 {
		t.Fatalf("zero-call recovery = task %#v completed=%d", got, len(calls.completed))
	}
}

func TestStartupExecutionRecoverySQLiteClosesRunningCallsWithoutReportsOrSettlements(t *testing.T) {
	db := openWorkerSQLite(t)
	tasks, calls := modelbilling.NewSQLiteTaskStore(db), modelbilling.NewSQLiteCallStore(db)
	scope := startupSubscriptionScope(11)
	task, taskContext := persistStartupSQLiteRunningTask(t, tasks, scope, "task-startup-sqlite")
	running := modelbilling.ModelCall{
		CallID: "call-startup-running", TaskID: task.TaskID, Scope: scope, BillingSource: task.BillingSource,
		Role: modelbilling.CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture",
		Endpoint: modelbilling.MaasEndpointChatCompletions, PricingEvidenceID: task.PricingEvidenceID,
		State: modelbilling.CallPrepared,
	}
	prepared := running
	prepared.CallID = "call-startup-prepared"
	if err := calls.Prepare(context.Background(), taskContext, running); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(context.Background(), taskContext, running.CallID, startupRecoveryNow.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := calls.Prepare(context.Background(), taskContext, prepared); err != nil {
		t.Fatal(err)
	}
	recovery, err := NewStartupExecutionRecovery(tasks, calls, func() time.Time { return startupRecoveryNow })
	if err != nil {
		t.Fatal(err)
	}
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatal(err)
	}

	gotTask, err := tasks.Get(context.Background(), scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if gotTask.State != modelbilling.TaskUsagePending || gotTask.Result == nil || *gotTask.Result != modelbilling.TaskResultFailed {
		t.Fatalf("task = %#v", gotTask)
	}
	gotCalls, err := calls.ListByTask(context.Background(), scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	states := make(map[string]modelbilling.CallState, len(gotCalls))
	for _, call := range gotCalls {
		states[call.CallID] = call.State
	}
	if states[running.CallID] != modelbilling.CallInterrupted || states[prepared.CallID] != modelbilling.CallPrepared {
		t.Fatalf("call states = %v", states)
	}
	var reports, settlements int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage_reports WHERE task_id=?`, task.TaskID).Scan(&reports); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM model_task_quota_settlement_revisions WHERE task_id=?`, task.TaskID).Scan(&settlements); err != nil {
		t.Fatal(err)
	}
	if reports != 0 || settlements != 0 {
		t.Fatalf("remote facts = reports:%d settlements:%d", reports, settlements)
	}
}

func TestStartupExecutionRecoverySQLiteKeepsUnknownStartedAtNull(t *testing.T) {
	db := openWorkerSQLite(t)
	tasks, calls := modelbilling.NewSQLiteTaskStore(db), modelbilling.NewSQLiteCallStore(db)
	scope := startupSubscriptionScope(12)
	task, taskContext := persistStartupSQLiteRunningTask(t, tasks, scope, "task-startup-null-start")
	call := modelbilling.ModelCall{
		CallID: "call-null-start", TaskID: task.TaskID, Scope: scope, BillingSource: task.BillingSource,
		Role: modelbilling.CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture",
		Endpoint: modelbilling.MaasEndpointChatCompletions, PricingEvidenceID: task.PricingEvidenceID,
		State: modelbilling.CallPrepared,
	}
	if err := calls.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	if err := calls.BeginAttempt(context.Background(), taskContext, call.CallID, startupRecoveryNow.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	// V5 intentionally left historical running rows with NULL started_at. Drop
	// only the immutability guard in this isolated fixture to recreate such a
	// migrated row; recovery itself still runs against the real SQLite stores.
	if _, err := db.Exec(`DROP TRIGGER model_tasks_started_at_immutable`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE model_tasks SET started_at=NULL WHERE task_id=?`, task.TaskID); err != nil {
		t.Fatal(err)
	}
	recovery, err := NewStartupExecutionRecovery(tasks, calls, func() time.Time { return startupRecoveryNow })
	if err != nil {
		t.Fatal(err)
	}
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatalf("idempotent second recovery: %v", err)
	}
	gotTask, err := tasks.Get(context.Background(), scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if gotTask.State != modelbilling.TaskRecoveryRequired || gotTask.StartedAt != nil {
		t.Fatalf("task = state:%s started_at:%v", gotTask.State, gotTask.StartedAt)
	}
	gotCalls, err := calls.ListByTask(context.Background(), scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotCalls) != 1 || gotCalls[0].State != modelbilling.CallRunning {
		t.Fatalf("unknown task start rewrote calls = %#v", gotCalls)
	}
}

func TestStartupExecutionRecoverySQLiteResumesPartiallyInterruptedCallsIdempotently(t *testing.T) {
	db := openWorkerSQLite(t)
	tasks, calls := modelbilling.NewSQLiteTaskStore(db), modelbilling.NewSQLiteCallStore(db)
	scope := startupSubscriptionScope(13)
	task, taskContext := persistStartupSQLiteRunningTask(t, tasks, scope, "task-startup-partial")
	makeCall := func(id string) modelbilling.ModelCall {
		return modelbilling.ModelCall{
			CallID: id, TaskID: task.TaskID, Scope: scope, BillingSource: task.BillingSource,
			Role: modelbilling.CallRoleMain, ModelID: task.InitialModelID, Provider: "fixture",
			Endpoint: modelbilling.MaasEndpointChatCompletions, PricingEvidenceID: task.PricingEvidenceID,
			State: modelbilling.CallPrepared,
		}
	}
	first, second := makeCall("call-already-interrupted"), makeCall("call-still-running")
	for _, call := range []modelbilling.ModelCall{first, second} {
		if err := calls.Prepare(context.Background(), taskContext, call); err != nil {
			t.Fatal(err)
		}
		if err := calls.BeginAttempt(context.Background(), taskContext, call.CallID, startupRecoveryNow.Add(-2*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	firstFinished := startupRecoveryNow.Add(-time.Minute)
	firstCode := StartupRecoveryErrorCode
	first.State, first.FinishedAt, first.LastErrorCode = modelbilling.CallInterrupted, &firstFinished, &firstCode
	if err := calls.Complete(context.Background(), taskContext, first); err != nil {
		t.Fatal(err)
	}
	recovery, err := NewStartupExecutionRecovery(tasks, calls, func() time.Time { return startupRecoveryNow })
	if err != nil {
		t.Fatal(err)
	}
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if err := recovery.RecoverScope(context.Background(), scope); err != nil {
		t.Fatalf("idempotent second recovery: %v", err)
	}
	gotCalls, err := calls.ListByTask(context.Background(), scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotCalls) != 2 || gotCalls[0].State != modelbilling.CallInterrupted || gotCalls[1].State != modelbilling.CallInterrupted {
		t.Fatalf("partially recovered calls = %#v", gotCalls)
	}
	for _, id := range []string{first.CallID, second.CallID} {
		var revisions int
		if err := db.QueryRow(`SELECT COUNT(*) FROM model_call_quota_evidence_revisions WHERE call_id=?`, id).Scan(&revisions); err != nil {
			t.Fatal(err)
		}
		if revisions != 1 {
			t.Fatalf("call %s evidence revisions = %d, want 1", id, revisions)
		}
	}
}

func persistStartupSQLiteRunningTask(
	t *testing.T,
	tasks *modelbilling.SQLiteTaskStore,
	scope modelbilling.TaskScope,
	taskID string,
) (modelbilling.ModelTask, modelbilling.TaskContext) {
	t.Helper()
	created := startupRecoveryNow.Add(-10 * time.Minute)
	task := modelbilling.ModelTask{
		TaskID: taskID, Scope: scope, BillingSource: modelbilling.BillingSubscriptionPool,
		InitialModelID: "model-startup", ClientTaskType: "chat", ClientEstimatedCost: "0",
		PricingEvidenceID: "price-" + taskID, State: modelbilling.TaskCreated,
		CreatedAt: created, UpdatedAt: created,
	}
	evidence := modelbilling.PricingEvidence{
		EvidenceID: task.PricingEvidenceID, SnapshotID: "snapshot-startup", Scope: scope, TaskID: taskID,
		ModelID: task.InitialModelID, PricingGroup: "default", State: modelbilling.PricingFresh, CapturedAt: created,
	}
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	taskContext, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: task.TaskID, Scope: scope, BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := &cloud.FreezeRequest{
		Scope: *scope.CloudBinding, TaskID: taskID, ModelID: task.InitialModelID,
		ClientTaskType: task.ClientTaskType, ClientEstimatedCost: task.ClientEstimatedCost,
		IdempotencyKey: "freeze-request-" + taskID,
	}
	if err := tasks.RecordFreezeRequest(context.Background(), taskContext, request, created.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	freeze := &cloud.Freeze{
		Scope: *scope.CloudBinding, FreezeID: "freeze-" + taskID, TaskID: taskID, Status: cloud.FreezeActive,
		FreezeAmount: "0", FreeFrozenAmount: "0", SubscriptionFrozenAmount: "0",
		ExpiresAt: startupRecoveryNow.Add(time.Hour),
	}
	entitlement := &modelbilling.TaskEntitlementSnapshot{Plan: "fixture", CapturedAt: created.Add(2 * time.Minute)}
	if err := tasks.RecordFreezeResult(context.Background(), taskContext, freeze, entitlement, created.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), modelbilling.TaskStateUpdate{
		Scope: scope, TaskID: taskID, From: modelbilling.TaskReady, To: modelbilling.TaskRunning, At: created.Add(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := tasks.Get(context.Background(), scope, taskID)
	if err != nil {
		t.Fatal(err)
	}
	return stored, taskContext
}

func taskResult(value modelbilling.TaskResult) *modelbilling.TaskResult { return &value }

func sameTaskResult(first, second *modelbilling.TaskResult) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}
