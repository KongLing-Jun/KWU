package eventwire

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
)

func TestToWireOperationalPromptAndReceipt(t *testing.T) {
	now := time.Unix(123, 0).UTC()
	in := event.Event{Kind: event.Operational, EventID: "e-2", Operational: &event.OperationalEnvelope{
		SchemaVersion: 1, EventID: "e-2", OperationID: "o-1", CausationID: "e-1",
		Aggregate: event.OperationalAggregate{Kind: "prompt", ID: "p-1", Sequence: 2},
		Scope:     event.OperationalScope{SessionID: "s-1", PromptID: "p-1"},
		Kind:      event.PromptEmitted, Stage: "controller", Outcome: "pending", Severity: "info",
		UserImpact: "waiting_for_user", PrivacyClass: event.PrivacyUserContent, OccurredAt: now,
		Prompt: &event.PromptOperationalPayload{Kind: event.PromptKindApproval, Lifecycle: event.PromptLifecyclePending,
			Approval: &event.Approval{ID: "p-1", Tool: "bash", Subject: "go test"},
			Reply:    &event.PromptReplyMetadata{Action: event.PromptReplyAllow, SessionGrant: true}},
	}}
	b, err := json.Marshal(ToWire(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"kind":"operational"`, `"schemaVersion":1`, `"causationId":"e-1"`, `"sequence":2`, `"promptId":"p-1"`, `"kind":"prompt.emitted"`, `"lifecycle":"pending"`, `"reply":{"action":"allow","sessionGrant":true,"persistent":false,"answerCount":0}`} {
		if !strings.Contains(s, want) {
			t.Fatalf("wire JSON %s missing %s", s, want)
		}
	}
	receipt, err := json.Marshal(ToWirePromptReplyReceipt(event.PromptReplyReceipt{PromptID: "p-1", Status: event.PromptReplyFailedStatus, OccurrenceID: "occ-1", Retryable: true}))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(receipt); got != `{"promptId":"p-1","status":"failed","occurrenceId":"occ-1","retryable":true}` {
		t.Fatalf("receipt JSON = %s", got)
	}
}

func TestPromptDiagnosticWireRejectsDisplayPoison(t *testing.T) {
	poison := "SECRET_BODY_C:/users/private"
	e := event.OperationalEnvelope{
		SchemaVersion: 1, EventID: "e-1", OperationID: "o-1",
		Aggregate: event.OperationalAggregate{Kind: "prompt", ID: "p-1", Sequence: 1},
		Scope:     event.OperationalScope{PromptID: "p-1"}, Kind: event.PromptReplyFailed,
		Prompt: &event.PromptOperationalPayload{Kind: event.PromptKindAsk, Lifecycle: event.PromptLifecycleReplyFailed,
			Ask:   &event.Ask{ID: "p-1", Questions: []event.AskQuestion{{Prompt: poison}}},
			Reply: &event.PromptReplyMetadata{Action: event.PromptReplyAnswered, AnswerCount: 2}},
	}
	b, err := json.Marshal(ToWirePromptDiagnostic(e))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), poison) {
		t.Fatalf("diagnostic wire leaked poison: %s", b)
	}
	for _, want := range []string{`"replyAction":"answered"`, `"answerCount":2`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("diagnostic wire %s missing safe metadata %s", b, want)
		}
	}
}

func TestToWireOperationalCarriesContentFreeLifecycleScope(t *testing.T) {
	in := &event.OperationalEnvelope{
		SchemaVersion: 1, EventID: "event-1", OperationID: "call-1",
		Aggregate: event.OperationalAggregate{Kind: event.AggregateModelCall, ID: "call-1", Sequence: 2},
		Scope:     event.OperationalScope{TaskID: "task-1", CallID: "call-1"},
		Kind:      event.ModelCallRunning,
		ModelCall: &event.LifecycleOperationalPayload{State: "running", PreviousState: "prepared", Role: "main"},
	}
	wire := ToWireOperational(in)
	if wire.Aggregate.Kind != "model_call" || wire.Scope.TaskID != "task-1" || wire.Scope.CallID != "call-1" || wire.ModelCall == nil || wire.ModelCall.Role != "main" {
		t.Fatalf("wire lifecycle = %+v", wire)
	}
}
