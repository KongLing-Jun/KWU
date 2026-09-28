package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"kwu/internal/provider"
)

func TestProjectContextMetadataIsNotForwardedToAnthropicWire(t *testing.T) {
	request := (&client{model: "test"}).buildRequest(provider.Request{Messages: []provider.Message{{
		Role: provider.RoleUser, Content: "context body", MessageKind: provider.MessageKindProjectContext,
		Metadata: &provider.MessageMetadata{Source: provider.ProjectContextSourceCloudInstruction, Version: 2, ContentHash: strings.Repeat("a", 64)},
	}}})
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "context body") || strings.Contains(text, "message_kind") || strings.Contains(text, "cloud_project_instruction") || strings.Contains(text, "content_hash") {
		t.Fatalf("Anthropic wire leaked local metadata: %s", text)
	}
}
