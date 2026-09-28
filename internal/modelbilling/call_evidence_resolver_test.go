package modelbilling

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingCallEvidenceStore struct {
	evidence PricingEvidence
	loadErr  error
	storeErr error
	loaded   []string
	stored   []PricingEvidence
	events   *[]string
}

func (s *recordingCallEvidenceStore) LoadForCall(_ context.Context, _ TaskContext, evidenceID string) (PricingEvidence, error) {
	s.loaded = append(s.loaded, evidenceID)
	if s.events != nil {
		*s.events = append(*s.events, "load")
	}
	return s.evidence, s.loadErr
}

func (s *recordingCallEvidenceStore) StoreForCall(_ context.Context, _ TaskContext, evidence PricingEvidence) error {
	s.stored = append(s.stored, evidence)
	if s.events != nil {
		*s.events = append(*s.events, "store")
	}
	return s.storeErr
}

type recordingCallEvidencePreparer struct {
	prepared []CallEvidencePreparationInput
	evidence PricingEvidence
	err      error
	events   *[]string
}

func (p *recordingCallEvidencePreparer) PrepareForCall(_ context.Context, input CallEvidencePreparationInput) (PricingEvidence, error) {
	p.prepared = append(p.prepared, input)
	if p.events != nil {
		*p.events = append(*p.events, "prepare")
	}
	if p.err != nil {
		return PricingEvidence{}, p.err
	}
	evidence := p.evidence
	if evidence.EvidenceID == "" {
		evidence = input.InitialEvidence
		evidence.EvidenceID = input.EvidenceID
		evidence.ModelID = input.ModelID
		evidence.SnapshotID = "snapshot_derived"
	}
	return evidence, nil
}

func TestDefaultCallEvidenceResolverReusesPersistedInitialEvidence(t *testing.T) {
	task, evidence := callEvidenceTestFacts(t)
	store := &recordingCallEvidenceStore{evidence: evidence}
	preparer := &recordingCallEvidencePreparer{}
	resolver := DefaultCallEvidenceResolver{Store: store, Preparer: preparer, IDs: dispatcherIDGenerator{id: "unused"}}

	got, err := resolver.ResolveForCall(context.Background(), task, task.InitialModelID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EvidenceID != task.PricingEvidenceID || got.ModelID != task.InitialModelID {
		t.Fatalf("resolved initial evidence = %+v", got)
	}
	if len(store.loaded) != 1 || store.loaded[0] != task.PricingEvidenceID || len(store.stored) != 0 || len(preparer.prepared) != 0 {
		t.Fatalf("load/store/prepare = %#v/%#v/%#v", store.loaded, store.stored, preparer.prepared)
	}
}

func TestDefaultCallEvidenceResolverPreparesAndPersistsDerivedEvidenceInOrder(t *testing.T) {
	task, evidence := callEvidenceTestFacts(t)
	events := []string{}
	store := &recordingCallEvidenceStore{evidence: evidence, events: &events}
	preparer := &recordingCallEvidencePreparer{events: &events}
	resolver := DefaultCallEvidenceResolver{Store: store, Preparer: preparer, IDs: dispatcherIDGenerator{id: "evidence_derived"}}

	got, err := resolver.ResolveForCall(context.Background(), task, "model_derived")
	if err != nil {
		t.Fatal(err)
	}
	if got.EvidenceID != "evidence_derived" || got.ModelID != "model_derived" || got.PricingGroup != evidence.PricingGroup {
		t.Fatalf("resolved derived evidence = %+v", got)
	}
	if len(preparer.prepared) != 1 || preparer.prepared[0].Task != task || preparer.prepared[0].InitialEvidence.EvidenceID != evidence.EvidenceID || len(store.stored) != 1 {
		t.Fatalf("prepare/store = %#v/%#v", preparer.prepared, store.stored)
	}
	wantEvents := []string{"load", "prepare", "store"}
	if len(events) != len(wantEvents) {
		t.Fatalf("events = %#v", events)
	}
	for i := range wantEvents {
		if events[i] != wantEvents[i] {
			t.Fatalf("events = %#v, want %#v", events, wantEvents)
		}
	}
}

func TestDefaultCallEvidenceResolverRejectsChangedFrozenPricingGroup(t *testing.T) {
	task, evidence := callEvidenceTestFacts(t)
	changed := evidence
	changed.EvidenceID = "evidence_derived"
	changed.ModelID = "model_derived"
	changed.PricingGroup = "other"
	store := &recordingCallEvidenceStore{evidence: evidence}
	resolver := DefaultCallEvidenceResolver{
		Store: store, Preparer: &recordingCallEvidencePreparer{evidence: changed},
		IDs: dispatcherIDGenerator{id: "evidence_derived"},
	}

	_, err := resolver.ResolveForCall(context.Background(), task, "model_derived")
	if ErrorCodeOf(err) != ErrorContractInvalid || len(store.stored) != 0 {
		t.Fatalf("err/stored = %v/%#v", err, store.stored)
	}
}

func TestDefaultCallEvidenceResolverPreservesPreparationAndStoreErrors(t *testing.T) {
	task, evidence := callEvidenceTestFacts(t)
	for _, test := range []struct {
		name    string
		prepare error
		store   error
		want    error
	}{
		{name: "prepare", prepare: errors.New("prepare failed")},
		{name: "store", store: errors.New("store failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.want == nil {
				if test.prepare != nil {
					test.want = test.prepare
				} else {
					test.want = test.store
				}
			}
			store := &recordingCallEvidenceStore{evidence: evidence, storeErr: test.store}
			resolver := DefaultCallEvidenceResolver{Store: store, Preparer: &recordingCallEvidencePreparer{err: test.prepare}, IDs: dispatcherIDGenerator{id: "evidence_derived"}}
			_, err := resolver.ResolveForCall(context.Background(), task, "model_derived")
			if !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSQLiteCallEvidenceStoreUsesPersistedTaskOwnership(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks := NewSQLiteTaskStore(db)
	modelTask, initial := storeTask(BillingNewAPIByok, "task_call_evidence_store")
	if err := tasks.CreateTaskWithEvidence(context.Background(), modelTask, initial); err != nil {
		t.Fatal(err)
	}
	if err := updateTask(tasks, modelTask, TaskCreated, TaskReady); err != nil {
		t.Fatal(err)
	}
	task := taskContextForStore(t, modelTask)
	store := NewSQLiteCallEvidenceStore(db)

	loaded, err := store.LoadForCall(context.Background(), task, initial.EvidenceID)
	if err != nil || loaded.EvidenceID != initial.EvidenceID {
		t.Fatalf("LoadForCall = %+v, %v", loaded, err)
	}
	derived := initial
	derived.EvidenceID = "evidence_call_derived"
	derived.ModelID = "model_derived"
	derived.SnapshotID = "snapshot_derived"
	if err := store.StoreForCall(context.Background(), task, derived); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.LoadForCall(context.Background(), task, derived.EvidenceID)
	if err != nil || loaded.ModelID != derived.ModelID {
		t.Fatalf("derived LoadForCall = %+v, %v", loaded, err)
	}

	other := task
	other.Scope.ActorID = "other"
	if _, err := store.LoadForCall(context.Background(), other, derived.EvidenceID); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("cross-scope error = %v", err)
	}
}

func callEvidenceTestFacts(t *testing.T) (TaskContext, PricingEvidence) {
	t.Helper()
	task := dispatcherBYOKTask(t)
	evidence := PricingEvidence{
		EvidenceID: task.PricingEvidenceID, SnapshotID: "snapshot_initial", TaskID: task.TaskID,
		Scope: task.Scope, ModelID: task.InitialModelID, PricingGroup: "default",
		State: PricingFresh, CapturedAt: time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC),
	}
	return task, evidence
}

func taskContextForStore(t *testing.T, task ModelTask) TaskContext {
	t.Helper()
	value, err := NewTaskContext(TaskContextInput{
		TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
