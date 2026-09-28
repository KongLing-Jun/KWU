package main

import (
	"testing"

	"kwu/internal/event"
)

func TestScopeOperationalPromptAddsExactTabWithoutMutatingSource(t *testing.T) {
	original := &event.OperationalEnvelope{
		SchemaVersion: event.OperationalSchemaVersion,
		EventID:       "event-1",
		OperationID:   "operation-1",
		Aggregate:     event.OperationalAggregate{Kind: "prompt", ID: "prompt-1", Sequence: 2},
		Scope:         event.OperationalScope{SessionID: "session-1", PromptID: "prompt-1"},
		Kind:          event.PromptEmitted,
	}
	sink := &tabEventSink{tabID: "tab-owner"}
	scoped := sink.scopeOperationalPrompt(event.Event{Kind: event.Operational, EventID: "event-1", TurnID: "turn-1", CandidateID: "candidate-1", Operational: original})

	if original.Scope.TabID != "" {
		t.Fatalf("source envelope was mutated: %+v", original.Scope)
	}
	if scoped.Operational == original {
		t.Fatal("scoped event reused source envelope pointer")
	}
	if got := scoped.Operational.Scope; got.TabID != "tab-owner" || got.SessionID != "session-1" || got.TurnID != "turn-1" || got.CandidateID != "candidate-1" || got.PromptID != "prompt-1" {
		t.Fatalf("scoped identity = %+v", got)
	}
	wire := toWireTab(scoped, sink.tabID)
	if wire.TabID != "tab-owner" || wire.Operational == nil || wire.Operational.Scope.TabID != "tab-owner" {
		t.Fatalf("wire scope = tab:%q operational:%+v", wire.TabID, wire.Operational)
	}
}

func TestScopeOperationalPromptLeavesNonOperationalEventUntouched(t *testing.T) {
	sink := &tabEventSink{tabID: "tab-owner"}
	original := event.Event{Kind: event.Notice, EventID: "event-legacy", Text: "fixture"}
	if got := sink.scopeOperationalPrompt(original); got.Kind != original.Kind || got.EventID != original.EventID || got.Text != original.Text || got.Operational != nil {
		t.Fatalf("non-operational event changed: got %+v want %+v", got, original)
	}
}
