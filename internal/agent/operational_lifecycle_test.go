package agent

import (
	"context"
	"testing"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func TestExecuteOneEmitsToolLifecycleFromAuthoritativeBoundary(t *testing.T) {
	registry := tool.NewRegistry()
	registry.Add(fakeTool{name: "read_file", readOnly: true})
	var facts []event.OperationalEnvelope
	emitter := &event.OperationalEmitter{Sink: event.FuncSink(func(value event.Event) {
		facts = append(facts, *value.Operational)
	})}
	ctx := event.WithOperationalEmitter(context.Background(), emitter)
	ctx = event.WithOperationalScope(ctx, event.OperationalScope{SessionID: "session-1", TaskID: "task-1"})
	modelEventID := emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateModelCall, AggregateID: "model-call-1",
		Scope: event.OperationalScope{SessionID: "session-1", TaskID: "task-1", CallID: "model-call-1"},
		Kind:  event.ModelCallCompleted, Stage: "persistence", Outcome: "completed", Severity: "info", UserImpact: "none",
	})
	agent := New(nil, registry, NewSession(""), Options{}, event.Discard)
	outcome := agent.executeOne(ctx, provider.ToolCall{ID: "tool-1", Name: "read_file", Arguments: `{"path":"ignored"}`})
	if outcome.errMsg != "" {
		t.Fatalf("tool outcome = %+v", outcome)
	}
	if len(facts) != 4 || facts[1].Kind != event.ToolCallProposed || facts[2].Kind != event.ToolCallRunning || facts[3].Kind != event.ToolCallCompleted {
		t.Fatalf("tool lifecycle = %+v", facts)
	}
	facts = facts[1:]
	for index, fact := range facts {
		if fact.Aggregate.ID != "tool-1" || fact.Aggregate.Sequence != uint64(index+1) {
			t.Fatalf("fact %d = %+v", index, fact)
		}
		if fact.Scope.CallID != "model-call-1" || fact.Scope.TaskID != "task-1" || fact.Scope.SessionID != "session-1" {
			t.Fatalf("fact %d scope = %+v", index, fact.Scope)
		}
	}
	if facts[0].CausationID != modelEventID || facts[1].CausationID != facts[0].EventID || facts[2].CausationID != facts[1].EventID {
		t.Fatalf("tool causal chain = %+v", facts)
	}
}
