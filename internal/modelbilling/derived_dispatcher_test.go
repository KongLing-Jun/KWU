package modelbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/provider"
)

type fixedCallEvidenceResolver struct {
	evidence PricingEvidence
	err      error
	calls    []string
}

type tracedPrepareCallStore struct {
	recordingCallStore
	events *[]string
}

func (s *tracedPrepareCallStore) Prepare(ctx context.Context, task TaskContext, call ModelCall) error {
	*s.events = append(*s.events, "call_prepare")
	return s.recordingCallStore.Prepare(ctx, task, call)
}

func (r *fixedCallEvidenceResolver) ResolveForCall(_ context.Context, _ TaskContext, modelID string) (PricingEvidence, error) {
	r.calls = append(r.calls, modelID)
	return r.evidence, r.err
}

func TestRoleBoundStreamDispatcherBarrierRejectsSwallowedEvidencePersistenceError(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID: "task_derived_evidence_failure", Scope: TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model_main", PricingEvidenceID: "evidence_main",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("evidence persistence failed")
	resolver := &fixedCallEvidenceResolver{err: want}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions}
	dispatcher := &MainExecutorDispatcher{Calls: &recordingCallStore{}, Gateway: gateway, Evidence: resolver, IDs: dispatcherIDGenerator{id: "call"}}
	bound, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: CallRoleCompaction, ModelID: "model_compaction"})
	if err != nil {
		t.Fatal(err)
	}
	tasks := &recordingTaskStore{}
	err = (TaskLifecycleRunner{Tasks: tasks}).RunTask(ctx, func(ctx context.Context) error {
		_, _, _ = bound.StreamWithTerminalResult(ctx, &forbiddenProvider{}, provider.Request{})
		return nil
	})
	if !errors.Is(err, want) || gateway.calls != 0 {
		t.Fatalf("runner error/gateway calls = %v/%d", err, gateway.calls)
	}
	if len(tasks.updates) != 2 || tasks.updates[1].To != TaskUsagePending || tasks.updates[1].Result == nil || *tasks.updates[1].Result != TaskResultFailed {
		t.Fatalf("task updates = %#v", tasks.updates)
	}
}

func TestRoleBoundStreamDispatcherUsesOnlyRoleAndModelSpec(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID: "task_derived", Scope: TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model_main", PricingEvidenceID: "evidence_main",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	resolved := PricingEvidence{
		EvidenceID: "evidence_planner", SnapshotID: "snapshot_planner", TaskID: task.TaskID,
		Scope: task.Scope, ModelID: "model_planner", PricingGroup: "default", State: PricingFresh,
		CapturedAt: time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC),
	}
	resolver := &fixedCallEvidenceResolver{evidence: resolved}
	calls := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{{Chunk: provider.Chunk{Type: provider.ChunkDone}}}}
	dispatcher := &MainExecutorDispatcher{Calls: calls, Gateway: gateway, Evidence: resolver, IDs: dispatcherIDGenerator{id: "call_planner"}}
	bound, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: CallRolePlanner, ModelID: "model_planner"})
	if err != nil {
		t.Fatal(err)
	}
	var _ agent.StreamDispatcherWithTerminalResult = bound
	tasks := &recordingTaskStore{}
	err = (TaskLifecycleRunner{Tasks: tasks}).RunTask(ctx, func(ctx context.Context) error {
		chunks, terminal, err := bound.StreamWithTerminalResult(ctx, &forbiddenProvider{}, provider.Request{})
		if err != nil {
			return err
		}
		for range chunks {
		}
		return <-terminal
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != "model_planner" || len(calls.prepared) != 1 {
		t.Fatalf("resolver/prepared = %#v/%#v", resolver.calls, calls.prepared)
	}
	prepared := calls.prepared[0]
	if prepared.TaskID != task.TaskID || !prepared.Scope.Equal(task.Scope) || prepared.BillingSource != task.BillingSource || prepared.Role != CallRolePlanner || prepared.ModelID != "model_planner" || prepared.PricingEvidenceID != resolved.EvidenceID {
		t.Fatalf("prepared derived call = %+v", prepared)
	}
	if gateway.request.Role != CallRolePlanner || gateway.request.ModelID != "model_planner" {
		t.Fatalf("gateway request = %+v", gateway.request)
	}
}

func TestRoleBoundStreamDispatcherPersistsDerivedEvidenceBeforeCallIntent(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID: "task_evidence_before_call", Scope: TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model_main", PricingEvidenceID: "evidence_main",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	initial := PricingEvidence{
		EvidenceID: task.PricingEvidenceID, SnapshotID: "snapshot_main", TaskID: task.TaskID,
		Scope: task.Scope, ModelID: task.InitialModelID, PricingGroup: "default", State: PricingFresh,
		CapturedAt: time.Date(2026, 7, 14, 1, 0, 0, 0, time.UTC),
	}
	events := []string{}
	evidenceStore := &recordingCallEvidenceStore{evidence: initial, events: &events}
	resolver := DefaultCallEvidenceResolver{
		Store: evidenceStore, Preparer: &recordingCallEvidencePreparer{events: &events},
		IDs: dispatcherIDGenerator{id: "evidence_derived"},
	}
	calls := &tracedPrepareCallStore{events: &events}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{{Chunk: provider.Chunk{Type: provider.ChunkDone}}}}
	dispatcher := &MainExecutorDispatcher{Calls: calls, Gateway: gateway, Evidence: resolver, IDs: dispatcherIDGenerator{id: "call_derived"}}
	bound, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: CallRoleGuardian, ModelID: "model_derived"})
	if err != nil {
		t.Fatal(err)
	}
	if err := (TaskLifecycleRunner{Tasks: &recordingTaskStore{}}).RunTask(ctx, func(ctx context.Context) error {
		chunks, terminal, err := bound.StreamWithTerminalResult(ctx, &forbiddenProvider{}, provider.Request{})
		if err != nil {
			return err
		}
		for range chunks {
		}
		return <-terminal
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"load", "prepare", "store", "call_prepare"}
	if len(events) != len(want) {
		t.Fatalf("events = %#v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %#v, want %#v", events, want)
		}
	}
}

func TestRoleBoundStreamDispatcherRequiresLifecycleBarrierBeforeAnyProviderWork(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID: "task_derived_no_barrier", Scope: TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model_main", PricingEvidenceID: "evidence_main",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &fixedCallEvidenceResolver{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions}
	dispatcher := &MainExecutorDispatcher{Calls: &recordingCallStore{}, Gateway: gateway, Evidence: resolver, IDs: dispatcherIDGenerator{id: "call"}}
	bound, err := NewRoleBoundStreamDispatcher(dispatcher, CallDispatchSpec{Role: CallRoleGuardian, ModelID: "model_guardian"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := bound.StreamWithTerminalResult(ctx, &forbiddenProvider{}, provider.Request{}); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("missing barrier error = %v", err)
	}
	if len(resolver.calls) != 0 || gateway.calls != 0 {
		t.Fatalf("resolver/provider calls = %d/%d", len(resolver.calls), gateway.calls)
	}
}
