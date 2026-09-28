package cloud

import (
	"context"
	"testing"
)

func TestEventSyncFreezeWithoutLocalExactRecordRefreshesAndCheckpointsWithoutRemoteRecovery(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	credit := &taskRecoveryCreditClient{}
	deps := newEventSyncDependencies(scope)
	deps.Tasks = NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, NewMemoryTaskRecoveryStore())
	state := NewMemoryEventStateRepository()
	deps.State = state
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"freeze-missing-local", EventFreezeExpired, scope.CloudUserID, 1,
		`{"task_id":"task-missing","freeze_id":"freeze-missing"}`,
	)

	if err := app.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatal(err)
	}
	if credit.getCalls != 0 {
		t.Fatalf("missing local record queried remote freeze %d times", credit.getCalls)
	}
	if calls := deps.Balance.(*eventSyncBalance).calls; calls != 1 {
		t.Fatalf("balance refresh calls = %d, want 1", calls)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("checkpoint = (%+v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncDuplicateAfterRestartRetriesOnlyDurablyStagedCleanupBatch(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	provider := testProfilePathProvider(t.TempDir())
	durable := NewDurableTaskRecoveryStore(provider)
	original := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-original", FreezeID: "freeze-original",
		IdempotencyKey: "key-original",
	}
	if err := durable.Save(ctx, original); err != nil {
		t.Fatal(err)
	}
	deleteErr := context.DeadlineExceeded
	failingStore := &taskRecoveryDeleteErrorStore{TaskRecoveryStore: durable, err: deleteErr}
	credit1 := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		original.FreezeID: recoveryNamedFreezeResult(scope, original.TaskID, original.FreezeID, FreezeExpiredReleased),
	}}
	deps1 := newEventSyncDependencies(scope)
	deps1.State = NewDurableEventStateRepository(provider)
	deps1.Tasks = NewTaskRecoveryApplicationWithStore(credit1, deps1.Bindings, failingStore)
	app1, err := NewEventSyncApplication(deps1)
	if err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"freeze-cleanup-restart", EventFreezeExpired, scope.CloudUserID, 1,
		`{"task_id":"task-original","freeze_id":"freeze-original"}`,
	)

	if err := app1.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatalf("first apply = %v", err)
	}
	if failingStore.calls != 1 {
		t.Fatalf("first cleanup calls = %d, want 1", failingStore.calls)
	}
	if records, err := durable.Load(ctx, scope); err != nil || len(records) != 1 || records[0] != original {
		t.Fatalf("records after failed cleanup = %+v, %v", records, err)
	}

	newRecord := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-new", FreezeID: "freeze-new",
		IdempotencyKey: "key-new",
	}
	if err := durable.Save(ctx, newRecord); err != nil {
		t.Fatal(err)
	}
	credit2 := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		original.FreezeID:  recoveryNamedFreezeResult(scope, original.TaskID, original.FreezeID, FreezeExpiredReleased),
		newRecord.FreezeID: recoveryNamedFreezeResult(scope, newRecord.TaskID, newRecord.FreezeID, FreezeExpiredReleased),
	}}
	tasks2 := NewTaskRecoveryApplicationWithStore(credit2, deps1.Bindings, NewDurableTaskRecoveryStore(provider))
	if decisions, err := tasks2.RecoverPendingForEventSync(ctx, eventSyncActor(), TaskRecoveryExclusion{}); err != nil || len(decisions) != 2 {
		t.Fatalf("new-process recovery = %+v, %v", decisions, err)
	}
	remoteCallsBeforeDuplicate := credit2.getCalls
	deps2 := newEventSyncDependencies(scope)
	deps2.State = NewDurableEventStateRepository(provider)
	deps2.Tasks = tasks2
	app2, err := NewEventSyncApplication(deps2)
	if err != nil {
		t.Fatal(err)
	}

	if err := app2.apply(ctx, eventSyncActor(), scope, event); err != nil {
		t.Fatalf("duplicate after restart = %v", err)
	}
	records, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || len(records) != 1 || records[0] != newRecord {
		t.Fatalf("restart cleanup records = %+v, %v; want only new recovery", records, err)
	}
	if credit2.getCalls != remoteCallsBeforeDuplicate {
		t.Fatalf("duplicate queried remote recovery: before=%d after=%d", remoteCallsBeforeDuplicate, credit2.getCalls)
	}
}

func TestEventSyncRestartSweepsCheckpointedRecoveryWithoutEventReplay(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	provider := testProfilePathProvider(t.TempDir())
	store := NewDurableTaskRecoveryStore(provider)
	record := TaskRecoveryRecord{
		Scope: scope, TaskID: "task-checkpointed", FreezeID: "freeze-checkpointed",
		IdempotencyKey: "key-checkpointed",
	}
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	event := eventSyncEvent(
		"checkpointed-without-replay", EventFreezeExpired, scope.CloudUserID, 1,
		`{"task_id":"task-checkpointed","freeze_id":"freeze-checkpointed"}`,
	)
	if err := store.StageEventSync(ctx, scope, event.EventID, []TaskRecoveryRecord{record}); err != nil {
		t.Fatal(err)
	}
	state := NewDurableEventStateRepository(provider)
	if err := state.CommitApplied(ctx, scope, event); err != nil {
		t.Fatal(err)
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		record.FreezeID: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
	}}
	deps := newEventSyncDependencies(scope)
	deps.State = state
	deps.Tasks = NewTaskRecoveryApplicationWithStore(credit, deps.Bindings, NewDurableTaskRecoveryStore(provider))
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatal(err)
	}
	records, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || len(records) != 0 {
		t.Fatalf("records after restart sweep = %+v, %v", records, err)
	}
	if credit.getCalls != 0 {
		t.Fatalf("restart sweep queried remote freeze %d times", credit.getCalls)
	}
}

func TestTaskRecoveryEventGCStageNeverRegressesAuthorizedRecord(t *testing.T) {
	for _, test := range []struct {
		name  string
		store func(*testing.T) TaskRecoveryStore
	}{
		{name: "memory", store: func(*testing.T) TaskRecoveryStore { return NewMemoryTaskRecoveryStore() }},
		{name: "durable", store: func(t *testing.T) TaskRecoveryStore {
			return NewDurableTaskRecoveryStore(testProfilePathProvider(t.TempDir()))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scope := eventSyncScope()
			record := TaskRecoveryRecord{Scope: scope, TaskID: "task", FreezeID: "freeze", IdempotencyKey: "key"}
			store := test.store(t)
			gc := store.(taskRecoveryEventGCStore)
			if err := store.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
			if err := gc.StageEventSync(ctx, scope, "event", []TaskRecoveryRecord{record}); err != nil {
				t.Fatal(err)
			}
			if err := gc.AuthorizeEventSync(ctx, scope, "event"); err != nil {
				t.Fatal(err)
			}
			if err := gc.StageEventSync(ctx, scope, "event", []TaskRecoveryRecord{record}); err != nil {
				t.Fatal(err)
			}
			authorized, err := gc.LoadAuthorizedEventSync(ctx, scope, "event")
			if err != nil || len(authorized) != 1 || authorized[0] != record {
				t.Fatalf("authorized after restage = %+v, %v", authorized, err)
			}

			updated := record
			updated.HasUsage = true
			if err := store.Save(ctx, updated); err != nil {
				t.Fatal(err)
			}
			authorized, err = gc.LoadAuthorizedEventSync(ctx, scope, "event")
			if err != nil || len(authorized) != 0 {
				t.Fatalf("authorization followed updated snapshot = %+v, %v", authorized, err)
			}
		})
	}
}
