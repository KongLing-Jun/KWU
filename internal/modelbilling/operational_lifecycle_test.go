package modelbilling

import (
	"context"
	"testing"

	"kwu/internal/event"
)

func TestTaskAndModelCallLifecycleUseOneAggregateSequence(t *testing.T) {
	var facts []event.OperationalEnvelope
	emitter := &event.OperationalEmitter{Sink: event.FuncSink(func(value event.Event) {
		facts = append(facts, *value.Operational)
	})}
	ctx := event.WithOperationalEmitter(context.Background(), emitter)
	ctx = event.WithOperationalScope(ctx, event.OperationalScope{SessionID: "session-1", PlanID: "plan-1"})
	planEventID := emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregatePlan, AggregateID: "plan-1", Scope: event.OperationalScope{SessionID: "session-1", PlanID: "plan-1"},
		Kind: event.PlanStarted, Stage: "controller", Outcome: "started", Severity: "info", UserImpact: "none",
	})
	emitTaskLifecycle(ctx, "task-1", TaskReady, TaskRunning)
	emitTaskLifecycle(ctx, "task-1", TaskRunning, TaskCompleted)
	emitModelCallLifecycle(ctx, ModelCall{TaskID: "task-1", CallID: "call-1", Role: CallRoleMain}, CallPrepared, CallRunning)
	emitModelCallLifecycle(ctx, ModelCall{TaskID: "task-1", CallID: "call-1", Role: CallRoleMain}, CallRunning, CallCompleted)
	if len(facts) != 5 {
		t.Fatalf("facts = %d, want 5", len(facts))
	}
	facts = facts[1:]
	for index, want := range []struct {
		aggregate event.OperationalAggregateKind
		sequence  uint64
		kind      event.OperationalKind
	}{
		{event.AggregateTask, 1, event.TaskRunning},
		{event.AggregateTask, 2, event.TaskCompleted},
		{event.AggregateModelCall, 1, event.ModelCallRunning},
		{event.AggregateModelCall, 2, event.ModelCallCompleted},
	} {
		got := facts[index]
		if got.Aggregate.Kind != want.aggregate || got.Aggregate.Sequence != want.sequence || got.Kind != want.kind {
			t.Fatalf("fact %d = %+v", index, got)
		}
		if got.Scope.SessionID != "session-1" || got.Scope.PlanID != "plan-1" || got.Scope.TaskID != "task-1" {
			t.Fatalf("fact %d scope = %+v", index, got.Scope)
		}
	}
	if facts[0].CausationID != planEventID || facts[1].CausationID != facts[0].EventID || facts[2].CausationID != facts[1].EventID || facts[3].CausationID != facts[2].EventID {
		t.Fatalf("causal chain = %+v", []string{facts[0].CausationID, facts[1].CausationID, facts[2].CausationID, facts[3].CausationID})
	}
}
