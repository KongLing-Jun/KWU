package cloud

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/userctx"
)

func TestEventSyncSettlementEventUsesStrictWorkerBalanceCheckpointOrder(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	trace := []string{}
	deps := newEventSyncDependencies(scope)
	deps.State = &eventSyncTracingState{EventStateRepository: NewMemoryEventStateRepository(), trace: &trace}
	deps.Balance.(*eventSyncBalance).trace = &trace
	reconciler := &eventSyncSettlementReconciler{trace: &trace}
	deps.Settlements = reconciler
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"settlement-event", EventSettlementCompleted, scope.CloudUserID, 1,
		`{"task_id":"task-1","settlement_id":"settlement-1"}`,
	)

	if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatal(err)
	}
	if want := []string{"reconcile", "balance", "checkpoint"}; !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
	if reconciler.calls != 1 || reconciler.actor != eventSyncActor().Normalize() || reconciler.scope != scope ||
		reconciler.taskID != "task-1" || reconciler.settlementID != "settlement-1" {
		t.Fatalf("reconcile identity = %+v", reconciler)
	}
	if tasks := deps.Tasks.(*eventSyncTasks); tasks.calls != 0 || tasks.exactCalls != 0 {
		t.Fatalf("settlement event used task recovery: pending=%d exact=%d", tasks.calls, tasks.exactCalls)
	}
}

func TestEventSyncSettlementEventStopsAtEachFailedStep(t *testing.T) {
	scope := eventSyncScope()
	event := eventSyncEvent(
		"settlement-stop", EventSettlementCompleted, scope.CloudUserID, 1,
		`{"task_id":"task-stop","settlement_id":"settlement-stop"}`,
	)

	t.Run("reconcile", func(t *testing.T) {
		deps := newEventSyncDependencies(scope)
		state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
		deps.State = state
		failed := errors.New("reconcile failed")
		reconciler := &eventSyncSettlementReconciler{err: failed}
		deps.Settlements = reconciler
		app, err := NewEventSyncApplication(deps)
		if err != nil {
			t.Fatal(err)
		}

		if err := app.apply(context.Background(), eventSyncActor(), scope, event); !errors.Is(err, failed) {
			t.Fatalf("apply = %v, want reconcile failure", err)
		}
		if reconciler.calls != 1 || deps.Balance.(*eventSyncBalance).calls != 0 || state.commits != 0 {
			t.Fatalf("reconcile failure effects = reconcile:%d balance:%d commit:%d", reconciler.calls, deps.Balance.(*eventSyncBalance).calls, state.commits)
		}
	})

	t.Run("balance", func(t *testing.T) {
		deps := newEventSyncDependencies(scope)
		state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
		deps.State = state
		failed := errors.New("balance failed")
		deps.Balance.(*eventSyncBalance).err = failed
		reconciler := &eventSyncSettlementReconciler{}
		deps.Settlements = reconciler
		app, err := NewEventSyncApplication(deps)
		if err != nil {
			t.Fatal(err)
		}

		if err := app.apply(context.Background(), eventSyncActor(), scope, event); !errors.Is(err, failed) {
			t.Fatalf("apply = %v, want balance failure", err)
		}
		if reconciler.calls != 1 || deps.Balance.(*eventSyncBalance).calls != 1 || state.commits != 0 {
			t.Fatalf("balance failure effects = reconcile:%d balance:%d commit:%d", reconciler.calls, deps.Balance.(*eventSyncBalance).calls, state.commits)
		}
	})

	t.Run("binding recheck", func(t *testing.T) {
		deps := newEventSyncDependencies(scope)
		state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
		deps.State = state
		deps.Balance.(*eventSyncBalance).after = func() {
			deps.Bindings.(*eventSyncResolver).scope.BindingGeneration++
		}
		reconciler := &eventSyncSettlementReconciler{}
		deps.Settlements = reconciler
		app, err := NewEventSyncApplication(deps)
		if err != nil {
			t.Fatal(err)
		}

		if err := app.apply(context.Background(), eventSyncActor(), scope, event); !errors.Is(err, ErrEventBindingChanged) {
			t.Fatalf("apply = %v, want binding change", err)
		}
		if reconciler.calls != 1 || deps.Balance.(*eventSyncBalance).calls != 1 || state.commits != 0 {
			t.Fatalf("binding change effects = reconcile:%d balance:%d commit:%d", reconciler.calls, deps.Balance.(*eventSyncBalance).calls, state.commits)
		}
	})
}

func TestEventSyncSettlementEventCheckpointFailureCanReplay(t *testing.T) {
	scope := eventSyncScope()
	checkpointErr := errors.New("checkpoint failed")
	state := &eventSyncCountingState{
		EventStateRepository: NewMemoryEventStateRepository(),
		errs:                 []error{checkpointErr, nil},
	}
	deps := newEventSyncDependencies(scope)
	deps.State = state
	reconciler := &eventSyncSettlementReconciler{}
	deps.Settlements = reconciler
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"settlement-replay", EventSettlementCompleted, scope.CloudUserID, 1,
		`{"task_id":"task-replay","settlement_id":"settlement-replay"}`,
	)

	if err := app.apply(context.Background(), eventSyncActor(), scope, event); !errors.Is(err, checkpointErr) {
		t.Fatalf("first apply = %v, want checkpoint failure", err)
	}
	if err := app.apply(context.Background(), eventSyncActor(), scope, event); err != nil {
		t.Fatalf("replay apply = %v", err)
	}
	checkpoint, found, err := state.Load(context.Background(), scope)
	if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("checkpoint after replay = (%+v, %t, %v)", checkpoint, found, err)
	}
	if reconciler.calls != 2 || deps.Balance.(*eventSyncBalance).calls != 2 || state.commits != 2 {
		t.Fatalf("replay effects = reconcile:%d balance:%d commit:%d", reconciler.calls, deps.Balance.(*eventSyncBalance).calls, state.commits)
	}
}

func TestEventSyncFreezeEventRecoversOnlyTheNamedRecordBeforeBalanceAndCheckpoint(t *testing.T) {
	scope := eventSyncScope()
	trace := []string{}
	deps := newEventSyncDependencies(scope)
	deps.State = &eventSyncTracingState{EventStateRepository: NewMemoryEventStateRepository(), trace: &trace}
	deps.Tasks.(*eventSyncTasks).trace = &trace
	deps.Balance.(*eventSyncBalance).trace = &trace
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"freeze-target", EventFreezeCreated, scope.CloudUserID, 1,
		`{"task_id":"task-target","freeze_id":"freeze-target"}`,
	)

	if err := app.apply(context.Background(), eventSyncActor(), scope, event); err != nil {
		t.Fatal(err)
	}
	tasks := deps.Tasks.(*eventSyncTasks)
	if want := []string{"freeze", "balance", "checkpoint"}; !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
	if tasks.calls != 0 || tasks.exactCalls != 1 || tasks.exactActor != eventSyncActor().Normalize() ||
		tasks.exactScope != scope || tasks.taskID != "task-target" || tasks.freezeID != "freeze-target" {
		t.Fatalf("targeted recovery = %+v", tasks)
	}
}

func TestEventSyncFreezeEventRejectsContractGapsBeforeRecovery(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "missing task", data: `{"freeze_id":"freeze-1"}`},
		{name: "uppercase task does not substitute", data: `{"TASK_ID":"task-1","freeze_id":"freeze-1"}`},
		{name: "missing freeze", data: `{"task_id":"task-1"}`},
		{name: "uppercase freeze does not substitute", data: `{"task_id":"task-1","FREEZE_ID":"freeze-1"}`},
		{name: "empty task", data: `{"task_id":"","freeze_id":"freeze-1"}`},
		{name: "empty freeze", data: `{"task_id":"task-1","freeze_id":" "}`},
		{name: "wrong json", data: `{`},
		{name: "null", data: `null`},
		{name: "array", data: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope := eventSyncScope()
			deps := newEventSyncDependencies(scope)
			state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
			deps.State = state
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			event := eventSyncEvent("invalid-freeze", EventFreezeExpired, scope.CloudUserID, 1, test.data)

			err = app.apply(context.Background(), eventSyncActor(), scope, event)
			if err == nil || !strings.Contains(err.Error(), "freeze event contract") {
				t.Fatalf("apply error = %v, want freeze event contract gap", err)
			}
			tasks := deps.Tasks.(*eventSyncTasks)
			if tasks.calls != 0 || tasks.exactCalls != 0 || deps.Balance.(*eventSyncBalance).calls != 0 || state.commits != 0 {
				t.Fatalf("invalid freeze effects = pending:%d exact:%d balance:%d load:%d commit:%d",
					tasks.calls, tasks.exactCalls, deps.Balance.(*eventSyncBalance).calls, state.loads, state.commits)
			}
		})
	}
}

func TestEventSyncFreezeEventStopsOnRecoveryIdentityFailure(t *testing.T) {
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
	deps.State = state
	identityErr := &Error{Code: ErrorProfileScopeMismatch, Message: "freeze belongs to another task"}
	deps.Tasks.(*eventSyncTasks).exactErr = identityErr
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"freeze-wrong-identity", EventFreezeExpired, scope.CloudUserID, 1,
		`{"task_id":"task-target","freeze_id":"freeze-target"}`,
	)

	if err := app.apply(context.Background(), eventSyncActor(), scope, event); !errors.Is(err, identityErr) {
		t.Fatalf("apply = %v, want recovery identity failure", err)
	}
	if deps.Tasks.(*eventSyncTasks).exactCalls != 1 || deps.Balance.(*eventSyncBalance).calls != 0 || state.commits != 0 {
		t.Fatalf("identity failure effects = exact:%d balance:%d commit:%d",
			deps.Tasks.(*eventSyncTasks).exactCalls, deps.Balance.(*eventSyncBalance).calls, state.commits)
	}
}

func TestEventSyncFreezeTerminationCanReplayAfterBalanceOrCheckpointFailure(t *testing.T) {
	for _, test := range []struct {
		name        string
		prepare     func(*eventSyncBalance, *eventSyncCountingState) error
		wantCommits int
	}{
		{
			name: "balance",
			prepare: func(balance *eventSyncBalance, _ *eventSyncCountingState) error {
				failed := errors.New("balance failed")
				balance.errs = []error{nil, failed, nil, nil}
				return failed
			},
			wantCommits: 1,
		},
		{
			name: "checkpoint",
			prepare: func(_ *eventSyncBalance, state *eventSyncCountingState) error {
				failed := errors.New("checkpoint failed")
				state.errs = []error{failed, nil}
				return failed
			},
			wantCommits: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scope := eventSyncScope()
			store := NewMemoryTaskRecoveryStore()
			record := TaskRecoveryRecord{
				Scope: scope, TaskID: "task-replay", FreezeID: "freeze-replay",
				IdempotencyKey: "freeze-replay-key", HasUsage: false,
			}
			if err := store.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
			credit := &taskRecoveryCreditClient{
				result: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
			}
			deps := newEventSyncDependencies(scope)
			deps.Tasks = NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, store)
			state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
			deps.State = state
			firstErr := test.prepare(deps.Balance.(*eventSyncBalance), state)
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			event := eventSyncEvent(
				"freeze-replay", EventFreezeExpired, scope.CloudUserID, 2,
				`{"task_id":"task-replay","freeze_id":"freeze-replay"}`,
			)

			if err := app.apply(ctx, eventSyncActor(), scope, event); !errors.Is(err, firstErr) {
				t.Fatalf("first apply = %v, want %v", err, firstErr)
			}
			records, err := store.Load(ctx, scope)
			if err != nil || len(records) != 1 || records[0] != record {
				t.Fatalf("record after failed apply = %+v, %v", records, err)
			}
			if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
				t.Fatalf("replay apply = %v", err)
			}
			checkpoint, found, err := state.Load(ctx, scope)
			if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
				t.Fatalf("checkpoint after replay = (%+v, %t, %v)", checkpoint, found, err)
			}
			if credit.getCalls != 1 || deps.Balance.(*eventSyncBalance).calls != 4 || state.commits != test.wantCommits {
				t.Fatalf("replay calls = freeze:%d balance:%d commit:%d",
					credit.getCalls, deps.Balance.(*eventSyncBalance).calls, state.commits)
			}
			records, err = store.Load(ctx, scope)
			if err != nil || len(records) != 0 {
				t.Fatalf("record after successful checkpoint = %+v, %v", records, err)
			}
		})
	}
}

func TestEventSyncGapFreezeExcludesTargetFromBatchRecovery(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	store := NewMemoryTaskRecoveryStore()
	target := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-target", FreezeID: "freeze-target",
		IdempotencyKey: "target-key", HasUsage: false,
	}
	other := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-other", FreezeID: "freeze-other",
		IdempotencyKey: "other-key", HasUsage: false,
	}
	for _, record := range []TaskRecoveryRecord{target, other} {
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		target.FreezeID: recoveryNamedFreezeResult(scope, target.TaskID, target.FreezeID, FreezeExpiredPendingReview),
		other.FreezeID:  recoveryNamedFreezeResult(scope, other.TaskID, other.FreezeID, FreezeExpiredReleased),
	}}
	deps := newEventSyncDependencies(scope)
	deps.Tasks = NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, store)
	state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
	deps.State = state
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"freeze-gap", EventFreezeExpired, scope.CloudUserID, 2,
		`{"task_id":"task-target","freeze_id":"freeze-target"}`,
	)

	if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatal(err)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("checkpoint = (%+v, %t, %v)", checkpoint, found, err)
	}
	records, err := store.Load(ctx, scope)
	if err != nil || len(records) != 1 || records[0] != target {
		t.Fatalf("records after gap event = %+v, %v", records, err)
	}
	if credit.getCalls != 2 || deps.Balance.(*eventSyncBalance).calls != 2 || state.commits != 1 {
		t.Fatalf("gap calls = freeze:%d balance:%d commit:%d",
			credit.getCalls, deps.Balance.(*eventSyncBalance).calls, state.commits)
	}
}

func TestEventSyncSettlementGapExcludesEveryRecoveryRecordForEventTask(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	store := NewMemoryTaskRecoveryStore()
	records := []TaskRecoveryRecord{
		{Scope: scope, TaskID: "task-settlement", FreezeID: "freeze-settlement-1", IdempotencyKey: "key-1"},
		{Scope: scope, TaskID: "task-settlement", FreezeID: "freeze-settlement-2", IdempotencyKey: "key-2"},
		{Scope: scope, TaskID: "task-other", FreezeID: "freeze-other", IdempotencyKey: "key-other"},
	}
	for _, record := range records {
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		"freeze-settlement-1": recoveryNamedFreezeResult(scope, "task-settlement", "freeze-settlement-1", FreezeActive),
		"freeze-settlement-2": recoveryNamedFreezeResult(scope, "task-settlement", "freeze-settlement-2", FreezeActive),
		"freeze-other":        recoveryNamedFreezeResult(scope, "task-other", "freeze-other", FreezeExpiredReleased),
	}}
	deps := newEventSyncDependencies(scope)
	deps.Tasks = NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, store)
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"settlement-gap", EventSettlementCompleted, scope.CloudUserID, 2,
		`{"task_id":"task-settlement","settlement_id":"settlement-1"}`,
	)

	if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatal(err)
	}
	if credit.getCalls != 1 {
		t.Fatalf("GetFreeze calls = %d, want only unrelated task", credit.getCalls)
	}
	stored, err := store.Load(ctx, scope)
	if err != nil || len(stored) != 2 || !recoveryContainsTask(stored, "task-settlement") || recoveryContainsTask(stored, "task-other") {
		t.Fatalf("stored records = %+v, %v", stored, err)
	}
}

func TestEventSyncAuthorityPayloadIgnoresAdditionalSignals(t *testing.T) {
	for _, test := range []struct {
		name      string
		eventType EventType
		data      string
		verify    func(*testing.T, EventSyncApplicationDependencies)
	}{
		{
			name:      "freeze",
			eventType: EventFreezeExpired,
			data:      `{"task_id":"task-signal","freeze_id":"freeze-signal","TASK_ID":"wrong-task","FREEZE_ID":"wrong-freeze","status":"expired_released","balance_version":9,"signal":{"source":"normalized"}}`,
			verify: func(t *testing.T, deps EventSyncApplicationDependencies) {
				t.Helper()
				tasks := deps.Tasks.(*eventSyncTasks)
				if tasks.exactCalls != 1 || tasks.taskID != "task-signal" || tasks.freezeID != "freeze-signal" {
					t.Fatalf("freeze identity = %+v", tasks)
				}
			},
		},
		{
			name:      "settlement",
			eventType: EventSettlementCompleted,
			data:      `{"task_id":"task-signal","settlement_id":"settlement-signal","TASK_ID":"wrong-task","SETTLEMENT_ID":"wrong-settlement","status":"settled","balance_version":9,"signal":true}`,
			verify: func(t *testing.T, deps EventSyncApplicationDependencies) {
				t.Helper()
				reconciler := deps.Settlements.(*eventSyncSettlementReconciler)
				if reconciler.calls != 1 || reconciler.taskID != "task-signal" || reconciler.settlementID != "settlement-signal" {
					t.Fatalf("settlement identity = %+v", reconciler)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope := eventSyncScope()
			deps := newEventSyncDependencies(scope)
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			event := eventSyncEvent("authority-signal", test.eventType, scope.CloudUserID, 1, test.data)

			if err := app.apply(context.Background(), eventSyncActor(), scope, event); err != nil {
				t.Fatal(err)
			}
			test.verify(t, deps)
		})
	}
}

func TestEventSyncDisconnectRecoveryKeepsFreezeIdentityForLaterEventWithoutHotLoop(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	store := NewMemoryTaskRecoveryStore()
	record := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-after-disconnect", FreezeID: "freeze-after-disconnect",
		IdempotencyKey: "disconnect-key", HasUsage: false,
	}
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		record.FreezeID: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
	}}
	deps := newEventSyncDependencies(scope)
	deps.Tasks = NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, store)
	deps.MaxReconnects = 1
	event := eventSyncEvent(
		"freeze-after-disconnect", EventFreezeExpired, scope.CloudUserID, 1,
		`{"task_id":"task-after-disconnect","freeze_id":"freeze-after-disconnect"}`,
	)
	deps.Source.(*eventSyncSource).streams = []<-chan Event{closedEventStream(), eventStream(event)}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatal(err)
	}
	checkpoint, found, err := deps.State.Load(ctx, scope)
	if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("checkpoint = (%+v, %t, %v)", checkpoint, found, err)
	}
	records, err := store.Load(ctx, scope)
	if err != nil || len(records) != 0 {
		t.Fatalf("records after checkpoint = %+v, %v", records, err)
	}
	if credit.getCalls != 1 {
		t.Fatalf("GetFreeze calls = %d, want 1 across disconnect, exact event and close", credit.getCalls)
	}
}

func TestEventSyncSkippedOldEventDoesNotAuthorizeNewRecoveryCleanup(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	record := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-new-recovery", FreezeID: "freeze-new-recovery",
		IdempotencyKey: "new-recovery-key",
	}
	store := NewMemoryTaskRecoveryStore()
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		record.FreezeID: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
	}}
	deps := newEventSyncDependencies(scope)
	tasks := NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, store)
	deps.Tasks = tasks
	mustCommitEvent(t, deps.State, scope, eventSyncEvent("checkpoint-2", EventBalanceUpdated, scope.CloudUserID, 2, `{}`))
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tasks.RecoverPendingForEventSync(ctx, eventSyncActor(), TaskRecoveryExclusion{}); err != nil {
		t.Fatal(err)
	}
	old := eventSyncEvent("old-1", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	if err := app.apply(ctx, eventSyncActor(), scope, old); err != nil {
		t.Fatal(err)
	}

	records, err := store.Load(ctx, scope)
	if err != nil || len(records) != 1 || records[0] != record {
		t.Fatalf("records after skipped old event = %+v, %v; want uncheckpointed recovery retained", records, err)
	}
}

func TestEventSyncCheckpointCleanupFailureIsRetriedByDuplicate(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	record := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-cleanup-failure", FreezeID: "freeze-cleanup-failure",
		IdempotencyKey: "cleanup-failure-key",
	}
	memory := NewMemoryTaskRecoveryStore()
	if err := memory.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	deleteErr := errors.New("cleanup delete failed")
	store := &taskRecoveryDeleteErrorStore{TaskRecoveryStore: memory, err: deleteErr}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		record.FreezeID: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
	}}
	deps := newEventSyncDependencies(scope)
	tasks := NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, store)
	deps.Tasks = tasks
	state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
	deps.State = state
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"freeze-cleanup-failure", EventFreezeExpired, scope.CloudUserID, 1,
		`{"task_id":"task-cleanup-failure","freeze_id":"freeze-cleanup-failure"}`,
	)

	if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatalf("apply = %v", err)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("checkpoint = (%+v, %t, %v)", checkpoint, found, err)
	}
	records, err := memory.Load(ctx, scope)
	if err != nil || len(records) != 1 || records[0] != record || len(tasks.eventTerminations) != 1 {
		t.Fatalf("retained cleanup failure = records:%+v cache:%+v err:%v", records, tasks.eventTerminations, err)
	}
	if store.calls != 1 || state.commits != 1 || credit.getCalls != 1 {
		t.Fatalf("cleanup failure calls = delete:%d commit:%d freeze:%d", store.calls, state.commits, credit.getCalls)
	}

	store.err = nil
	if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatalf("replay after cleanup recovery = %v", err)
	}
	checkpoint, found, err = state.Load(ctx, scope)
	if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("replayed checkpoint = (%+v, %t, %v)", checkpoint, found, err)
	}
	records, err = memory.Load(ctx, scope)
	if err != nil || len(records) != 0 || len(tasks.eventTerminations) != 0 {
		t.Fatalf("replayed cleanup = records:%+v cache:%+v err:%v", records, tasks.eventTerminations, err)
	}
	if store.calls != 2 || state.commits != 1 || credit.getCalls != 1 {
		t.Fatalf("replayed calls = delete:%d commit:%d freeze:%d", store.calls, state.commits, credit.getCalls)
	}
}

func TestEventSyncSettlementEventRejectsContractGapsBeforeAnyAuthorityOrCheckpoint(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		user string
	}{
		{name: "missing task", data: `{"settlement_id":"settlement-1"}`},
		{name: "uppercase task does not substitute", data: `{"TASK_ID":"task-1","settlement_id":"settlement-1"}`},
		{name: "missing settlement", data: `{"task_id":"task-1"}`},
		{name: "uppercase settlement does not substitute", data: `{"task_id":"task-1","SETTLEMENT_ID":"settlement-1"}`},
		{name: "empty task", data: `{"task_id":"","settlement_id":"settlement-1"}`},
		{name: "empty settlement", data: `{"task_id":"task-1","settlement_id":" "}`},
		{name: "wrong json", data: `{`},
		{name: "null", data: `null`},
		{name: "array", data: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			scope := eventSyncScope()
			deps := newEventSyncDependencies(scope)
			state := &eventSyncCountingState{EventStateRepository: NewMemoryEventStateRepository()}
			deps.State = state
			reconciler := &eventSyncSettlementReconciler{}
			deps.Settlements = reconciler
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			cloudUserID := test.user
			if cloudUserID == "" {
				cloudUserID = scope.CloudUserID
			}
			event := eventSyncEvent("invalid-settlement", EventSettlementException, cloudUserID, 1, test.data)

			err = app.apply(context.Background(), eventSyncActor(), scope, event)
			if err == nil || !strings.Contains(err.Error(), "settlement event contract") {
				t.Fatalf("apply error = %v, want settlement event contract gap", err)
			}
			if reconciler.calls != 0 || deps.Balance.(*eventSyncBalance).calls != 0 ||
				deps.Tasks.(*eventSyncTasks).calls != 0 || state.commits != 0 {
				t.Fatalf("invalid event effects = reconcile:%d balance:%d recovery:%d load:%d commit:%d",
					reconciler.calls, deps.Balance.(*eventSyncBalance).calls, deps.Tasks.(*eventSyncTasks).calls, state.loads, state.commits)
			}
		})
	}
}

func TestEventSyncStaleSettlementStillRejectsMissingAuthorityIdentity(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	repository := NewMemoryEventStateRepository()
	if err := repository.CommitApplied(ctx, scope, eventSyncEvent(
		"current-event", EventSubscriptionUpdated, scope.CloudUserID, 2, `{"fixture":true}`,
	)); err != nil {
		t.Fatal(err)
	}
	state := &eventSyncCountingState{EventStateRepository: repository}
	deps := newEventSyncDependencies(scope)
	deps.State = state
	reconciler := &eventSyncSettlementReconciler{}
	deps.Settlements = reconciler
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"stale-invalid-settlement", EventSettlementCompleted, scope.CloudUserID, 1,
		`{"settlement_id":"settlement-without-task"}`,
	)

	err = app.apply(ctx, eventSyncActor(), scope, event)
	if err == nil || !strings.Contains(err.Error(), "settlement event contract") {
		t.Fatalf("apply error = %v, want settlement event contract gap", err)
	}
	if reconciler.calls != 0 || deps.Balance.(*eventSyncBalance).calls != 0 ||
		deps.Tasks.(*eventSyncTasks).calls != 0 || state.commits != 0 {
		t.Fatalf("stale invalid event effects = reconcile:%d balance:%d recovery:%d commit:%d",
			reconciler.calls, deps.Balance.(*eventSyncBalance).calls,
			deps.Tasks.(*eventSyncTasks).calls, state.commits)
	}
}

type eventSyncCountingState struct {
	EventStateRepository
	loads   int
	commits int
	err     error
	errs    []error
}

func (s *eventSyncCountingState) Load(ctx context.Context, scope BindingScope) (EventCheckpoint, bool, error) {
	s.loads++
	return s.EventStateRepository.Load(ctx, scope)
}

func (s *eventSyncCountingState) CommitApplied(ctx context.Context, scope BindingScope, event Event) error {
	s.commits++
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return err
		}
	}
	if s.err != nil {
		return s.err
	}
	return s.EventStateRepository.CommitApplied(ctx, scope, event)
}

type eventSyncSettlementReconciler struct {
	calls        int
	actor        userctx.ActorContext
	scope        BindingScope
	taskID       string
	settlementID string
	err          error
	trace        *[]string
	after        func()
}

func (r *eventSyncSettlementReconciler) ReconcileSettlementEvent(
	_ context.Context,
	actor userctx.ActorContext,
	scope BindingScope,
	taskID string,
	settlementID string,
) error {
	r.calls++
	r.actor = actor.Normalize()
	r.scope = scope
	r.taskID = taskID
	r.settlementID = settlementID
	appendEventSyncTrace(r.trace, "reconcile")
	if r.after != nil {
		r.after()
	}
	return r.err
}
