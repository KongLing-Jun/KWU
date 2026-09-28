package control

import (
	"path/filepath"
	"testing"

	"kwu/internal/event"
)

func TestPlanModeEmitsIndependentPlanAggregate(t *testing.T) {
	var facts []event.OperationalEnvelope
	c := New(Options{Sink: event.FuncSink(func(value event.Event) {
		if value.Operational != nil && value.Operational.Aggregate.Kind == event.AggregatePlan {
			facts = append(facts, *value.Operational)
		}
	})})
	c.SetPlanMode(true)
	c.SetPlanMode(false)
	if len(facts) != 2 || facts[0].Kind != event.PlanStarted || facts[1].Kind != event.PlanExited || facts[0].Aggregate.ID != facts[1].Aggregate.ID || facts[1].Aggregate.Sequence != 2 {
		t.Fatalf("plan facts = %+v", facts)
	}
}

func TestSessionPathEmitsCreatedActivatedAndClosed(t *testing.T) {
	var facts []event.OperationalEnvelope
	c := New(Options{Sink: event.FuncSink(func(value event.Event) {
		if value.Operational != nil && value.Operational.Aggregate.Kind == event.AggregateSession {
			facts = append(facts, *value.Operational)
		}
	})})
	c.SetSessionPath(filepath.Join(t.TempDir(), "session.jsonl"))
	c.Close()
	if len(facts) != 3 || facts[0].Kind != event.SessionCreated || facts[1].Kind != event.SessionActivated || facts[2].Kind != event.SessionClosed {
		t.Fatalf("session facts = %+v", facts)
	}
}
