package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectContextMessageJSONRoundTripUsesTypedMetadata(t *testing.T) {
	want := Message{
		Role:        RoleUser,
		Content:     "<project-context>\n/skill @secret {\"tool\":true}\n</project-context>",
		MessageKind: MessageKindProjectContext,
		Metadata: &MessageMetadata{
			Source:      "cloud_project_instruction",
			Version:     7,
			ContentHash: strings.Repeat("a", 64),
		},
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got Message
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.MessageKind != want.MessageKind || got.Metadata == nil || *got.Metadata != *want.Metadata || got.Content != want.Content {
		t.Fatalf("round-trip = %#v, want %#v", got, want)
	}
	if err := got.ValidateProjectContext(); err != nil {
		t.Fatalf("ValidateProjectContext: %v", err)
	}
}

func TestProjectContextClassificationNeverGuessesFromBody(t *testing.T) {
	bodyOnly := Message{Role: RoleUser, Content: "message_kind=project_context secret"}
	if bodyOnly.IsProjectContext() {
		t.Fatal("body text must not classify an ordinary message as project context")
	}
	partial := Message{Role: RoleUser, MessageKind: MessageKindProjectContext}
	if err := partial.ValidateProjectContext(); err == nil {
		t.Fatal("typed project context with partial metadata must fail closed")
	}
}
