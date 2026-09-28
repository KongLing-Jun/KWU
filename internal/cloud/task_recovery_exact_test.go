package cloud

import (
	"context"
	"errors"
	"testing"
)

func TestTaskRecoveryApplicationRecoverExactOnlyQueriesNamedRecord(t *testing.T) {
	ctx := context.Background()
	scope := recoveryScope(t)
	target := recoveryNamedRecord(t, scope, "task-target", "freeze-target", true)
	other := recoveryNamedRecord(t, scope, "task-other", "freeze-other", true)
	store := NewMemoryTaskRecoveryStore()
	for _, record := range []TaskRecoveryRecord{target, other} {
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	credit := &taskRecoveryCreditClient{result: recoveryNamedFreezeResult(scope, target.TaskID, target.FreezeID, FreezeActive)}
	app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)

	decision, found, err := app.RecoverExact(ctx, recoveryActor(), scope, target.TaskID, target.FreezeID)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("RecoverExact did not find the saved identity")
	}
	if decision.Record != target || decision.Status != TaskRecoveryContinueOrFinalize {
		t.Fatalf("decision = %+v", decision)
	}
	if credit.getCalls != 1 || credit.scope != scope || credit.freezeID != target.FreezeID {
		t.Fatalf("freeze query = calls:%d scope:%+v freeze:%q", credit.getCalls, credit.scope, credit.freezeID)
	}
	records, err := store.Load(ctx, scope)
	if err != nil || len(records) != 2 || !recoveryContainsTask(records, target.TaskID) || !recoveryContainsTask(records, other.TaskID) {
		t.Fatalf("stored records = %+v, %v", records, err)
	}
}

func TestTaskRecoveryApplicationRecoverExactReportsMissingWithoutRemoteQuery(t *testing.T) {
	ctx := context.Background()
	scope := recoveryScope(t)
	credit := &taskRecoveryCreditClient{}
	app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), NewMemoryTaskRecoveryStore())

	decision, found, err := app.RecoverExact(ctx, recoveryActor(), scope, "task-missing", "freeze-missing")
	if err != nil || found || decision != (TaskRecoveryDecision{}) {
		t.Fatalf("RecoverExact missing = (%+v, %t, %v)", decision, found, err)
	}
	if credit.getCalls != 0 {
		t.Fatalf("missing local identity queried remote freeze %d times", credit.getCalls)
	}
}

func TestTaskRecoveryApplicationRecoverExactRejectsAmbiguousLocalIdentity(t *testing.T) {
	ctx := context.Background()
	scope := recoveryScope(t)
	store := NewMemoryTaskRecoveryStore()
	for _, key := range []string{"key-first", "key-second"} {
		record := TaskRecoveryRecord{Scope: scope, TaskID: "task-same", FreezeID: "freeze-same", IdempotencyKey: key}
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	credit := &taskRecoveryCreditClient{}
	app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)

	if _, _, err := app.RecoverExact(ctx, recoveryActor(), scope, "task-same", "freeze-same"); err == nil {
		t.Fatal("RecoverExact accepted ambiguous local identity")
	}
	if credit.getCalls != 0 {
		t.Fatalf("ambiguous local identity queried remote freeze %d times", credit.getCalls)
	}
}

func TestTaskRecoveryApplicationRecoverExactRejectsWrongIdentity(t *testing.T) {
	t.Run("event identity is not one saved record", func(t *testing.T) {
		ctx := context.Background()
		scope := recoveryScope(t)
		first := recoveryNamedRecord(t, scope, "task-first", "freeze-first", true)
		second := recoveryNamedRecord(t, scope, "task-second", "freeze-second", true)
		store := NewMemoryTaskRecoveryStore()
		for _, record := range []TaskRecoveryRecord{first, second} {
			if err := store.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
		}
		credit := &taskRecoveryCreditClient{}
		app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)

		if _, found, err := app.RecoverExact(ctx, recoveryActor(), scope, first.TaskID, second.FreezeID); err != nil || found {
			t.Fatalf("RecoverExact missing identity = found:%t err:%v", found, err)
		}
		if credit.getCalls != 0 {
			t.Fatalf("wrong local identity queried freeze %d times", credit.getCalls)
		}
	})

	t.Run("authoritative freeze belongs to another task", func(t *testing.T) {
		ctx := context.Background()
		scope := recoveryScope(t)
		record := recoveryNamedRecord(t, scope, "task-target", "freeze-target", true)
		store := NewMemoryTaskRecoveryStore()
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
		credit := &taskRecoveryCreditClient{result: recoveryNamedFreezeResult(scope, "task-other", record.FreezeID, FreezeActive)}
		app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)

		if _, _, err := app.RecoverExact(ctx, recoveryActor(), scope, record.TaskID, record.FreezeID); err == nil {
			t.Fatal("RecoverExact accepted authoritative freeze for another task")
		}
		if credit.getCalls != 1 {
			t.Fatalf("authoritative identity queries = %d", credit.getCalls)
		}
	})

	t.Run("binding generation", func(t *testing.T) {
		ctx := context.Background()
		scope := recoveryScope(t)
		record := recoveryNamedRecord(t, scope, "task-target", "freeze-target", true)
		store := NewMemoryTaskRecoveryStore()
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
		credit := &taskRecoveryCreditClient{}
		app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)
		wrong := scope
		wrong.BindingGeneration++

		if _, _, err := app.RecoverExact(ctx, recoveryActor(), wrong, record.TaskID, record.FreezeID); err == nil {
			t.Fatal("RecoverExact accepted another binding generation")
		}
		if credit.getCalls != 0 {
			t.Fatalf("wrong scope queried freeze %d times", credit.getCalls)
		}
	})
}

func TestTaskRecoveryApplicationEventSyncRecoveryRetainsTerminationAndSuppressesRepeatedQueries(t *testing.T) {
	ctx := context.Background()
	scope := recoveryScope(t)
	record := recoveryNamedRecord(t, scope, "task-terminate", "freeze-terminate", false)
	store := NewMemoryTaskRecoveryStore()
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		record.FreezeID: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
	}}
	app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)

	for i := 0; i < 2; i++ {
		decisions, err := app.RecoverPendingForEventSync(ctx, recoveryActor(), TaskRecoveryExclusion{})
		if err != nil || len(decisions) != 1 || decisions[0].Status != TaskRecoveryTerminate || decisions[0].Record != record {
			t.Fatalf("RecoverPendingForEventSync[%d] = %+v, %v", i, decisions, err)
		}
	}
	decision, found, err := app.RecoverExact(ctx, recoveryActor(), scope, record.TaskID, record.FreezeID)
	if err != nil || !found || decision.Status != TaskRecoveryTerminate || decision.Record != record {
		t.Fatalf("RecoverExact = %+v, %t, %v", decision, found, err)
	}
	if err := app.StageEventSync(ctx, scope, "event-1", []TaskRecoveryDecision{decision}); err != nil {
		t.Fatal(err)
	}
	if err := app.AcknowledgeEventSync(ctx, scope, "event-1"); err != nil {
		t.Fatal(err)
	}
	records, err := store.Load(ctx, scope)
	if err != nil || len(records) != 0 {
		t.Fatalf("records after acknowledge = %+v, %v", records, err)
	}
	if credit.getCalls != 1 {
		t.Fatalf("GetFreeze calls = %d, want 1", credit.getCalls)
	}
	if len(app.eventTerminations) != 0 {
		t.Fatalf("cached terminations after acknowledge = %+v", app.eventTerminations)
	}
}

func TestTaskRecoveryApplicationEventSyncAcknowledgeKeepsIdentityWhenDeleteFails(t *testing.T) {
	ctx := context.Background()
	scope := recoveryScope(t)
	record := recoveryNamedRecord(t, scope, "task-delete-failure", "freeze-delete-failure", false)
	memory := NewMemoryTaskRecoveryStore()
	if err := memory.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	deleteErr := errors.New("delete failed")
	store := &taskRecoveryDeleteErrorStore{TaskRecoveryStore: memory, err: deleteErr}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		record.FreezeID: recoveryNamedFreezeResult(scope, record.TaskID, record.FreezeID, FreezeExpiredReleased),
	}}
	app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)
	decisions, err := app.RecoverPendingForEventSync(ctx, recoveryActor(), TaskRecoveryExclusion{})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.StageEventSync(ctx, scope, "event-delete", decisions); err != nil {
		t.Fatal(err)
	}

	if err := app.AcknowledgeEventSync(ctx, scope, "event-delete"); !errors.Is(err, deleteErr) {
		t.Fatalf("AcknowledgeEventSync = %v, want %v", err, deleteErr)
	}
	records, err := memory.Load(ctx, scope)
	if err != nil || len(records) != 1 || records[0] != record || len(app.eventTerminations) != 1 {
		t.Fatalf("retained after delete failure = records:%+v cache:%+v err:%v", records, app.eventTerminations, err)
	}
	store.err = nil
	if err := app.RetryAcknowledgedEventSync(ctx, scope, "event-delete"); err != nil {
		t.Fatal(err)
	}
	records, err = memory.Load(ctx, scope)
	if err != nil || len(records) != 0 || len(app.eventTerminations) != 0 {
		t.Fatalf("cleanup retry = records:%+v cache:%+v err:%v", records, app.eventTerminations, err)
	}
}

func TestTaskRecoveryApplicationEventSyncAcknowledgeOnlyCleansExactScope(t *testing.T) {
	ctx := context.Background()
	first := eventSyncScope()
	second := first
	second.BindingGeneration++
	firstRecord := TaskRecoveryRecord{Scope: first, TaskID: "task-first", FreezeID: "freeze-first", IdempotencyKey: "key-first"}
	secondRecord := TaskRecoveryRecord{Scope: second, TaskID: "task-second", FreezeID: "freeze-second", IdempotencyKey: "key-second"}
	store := NewMemoryTaskRecoveryStore()
	for _, record := range []TaskRecoveryRecord{firstRecord, secondRecord} {
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		firstRecord.FreezeID:  recoveryNamedFreezeResult(first, firstRecord.TaskID, firstRecord.FreezeID, FreezeExpiredReleased),
		secondRecord.FreezeID: recoveryNamedFreezeResult(second, secondRecord.TaskID, secondRecord.FreezeID, FreezeExpiredReleased),
	}}
	resolver := &eventSyncResolver{scope: first}
	app := NewTaskRecoveryApplicationWithStore(credit, resolver, store)
	firstDecisions, err := app.RecoverPendingForEventSync(ctx, eventSyncActor(), TaskRecoveryExclusion{})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.StageEventSync(ctx, first, "event-first", firstDecisions); err != nil {
		t.Fatal(err)
	}
	resolver.scope = second
	secondDecisions, err := app.RecoverPendingForEventSync(ctx, eventSyncActor(), TaskRecoveryExclusion{})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.StageEventSync(ctx, second, "event-second", secondDecisions); err != nil {
		t.Fatal(err)
	}

	if err := app.AcknowledgeEventSync(ctx, first, "event-first"); err != nil {
		t.Fatal(err)
	}
	firstRecords, firstErr := store.Load(ctx, first)
	secondRecords, secondErr := store.Load(ctx, second)
	if firstErr != nil || len(firstRecords) != 0 || secondErr != nil || len(secondRecords) != 1 || secondRecords[0] != secondRecord {
		t.Fatalf("scope cleanup = first:%+v/%v second:%+v/%v", firstRecords, firstErr, secondRecords, secondErr)
	}
	if len(app.eventTerminations) != 1 {
		t.Fatalf("other generation cache = %+v", app.eventTerminations)
	}
}

type taskRecoveryDeleteErrorStore struct {
	TaskRecoveryStore
	err   error
	calls int
}

func (s *taskRecoveryDeleteErrorStore) StageEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
	records []TaskRecoveryRecord,
) error {
	return s.TaskRecoveryStore.(taskRecoveryEventGCStore).StageEventSync(ctx, scope, eventID, records)
}

func (s *taskRecoveryDeleteErrorStore) AuthorizeEventSync(ctx context.Context, scope BindingScope, eventID string) error {
	return s.TaskRecoveryStore.(taskRecoveryEventGCStore).AuthorizeEventSync(ctx, scope, eventID)
}

func (s *taskRecoveryDeleteErrorStore) LoadAuthorizedEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
) ([]TaskRecoveryRecord, error) {
	return s.TaskRecoveryStore.(taskRecoveryEventGCStore).LoadAuthorizedEventSync(ctx, scope, eventID)
}

func (s *taskRecoveryDeleteErrorStore) LoadEventSyncGCIDs(ctx context.Context, scope BindingScope) ([]string, error) {
	return s.TaskRecoveryStore.(taskRecoveryEventGCStore).LoadEventSyncGCIDs(ctx, scope)
}

func (s *taskRecoveryDeleteErrorStore) Delete(ctx context.Context, record TaskRecoveryRecord) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	return s.TaskRecoveryStore.Delete(ctx, record)
}
