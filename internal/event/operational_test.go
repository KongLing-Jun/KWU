package event

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPromptIdentitiesRemainUniqueAcrossOwners(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id := NewPromptID()
		if seen[id] || !strings.HasPrefix(id, "p-") {
			t.Fatalf("prompt identity %q is duplicate or malformed", id)
		}
		seen[id] = true
	}
}

func TestPromptDiagnosticDTOIsMetadataOnly(t *testing.T) {
	poison := "SECRET_PROMPT_/absolute/path_token"
	envelope := OperationalEnvelope{
		SchemaVersion: OperationalSchemaVersion,
		EventID:       "e-1", OperationID: "o-1",
		Aggregate: OperationalAggregate{Kind: "prompt", ID: "p-1", Sequence: 3},
		Scope:     OperationalScope{PromptID: "p-1", SessionID: "s-1"},
		Kind:      PromptReplyFailed, PrivacyClass: PrivacyUserContent,
		Prompt: &PromptOperationalPayload{Kind: PromptKindApproval, Lifecycle: PromptLifecycleReplyFailed,
			Approval: &Approval{ID: "p-1", Tool: "bash", Subject: poison, Reason: poison},
			Reply:    &PromptReplyMetadata{Action: PromptReplyDeny, Persistent: true}},
	}
	b, err := json.Marshal(envelope.Diagnostic())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), poison) {
		t.Fatalf("diagnostic leaked poison data: %s", b)
	}
	if diagnostic := envelope.Diagnostic(); diagnostic.ReplyAction != PromptReplyDeny || !diagnostic.Persistent {
		t.Fatalf("diagnostic omitted safe reply metadata: %+v", diagnostic)
	}
	for _, forbidden := range []string{"Approval", "Ask", "Answer", "Subject", "Reason", "Path", "ActorID", "ProfileID"} {
		if _, ok := reflect.TypeOf(PromptDiagnosticDTO{}).FieldByName(forbidden); ok {
			t.Fatalf("diagnostic DTO exposes forbidden field %q", forbidden)
		}
	}
}
