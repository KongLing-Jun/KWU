package acp

import (
	"encoding/json"
	"strings"
	"testing"

	"kwu/internal/provider"
)

func TestACPProjectContextExcludedFromReplayAndTitle(t *testing.T) {
	secret := "PRIVATE ACP PROJECT CONTEXT"
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: secret, MessageKind: provider.MessageKindProjectContext, Metadata: &provider.MessageMetadata{Source: "cloud_project_instruction", Version: 3, ContentHash: strings.Repeat("a", 64)}},
		{Role: provider.RoleUser, Content: "ordinary task"},
	}
	if got := titleFromHistory(msgs); got != "ordinary task" {
		t.Fatalf("title = %q", got)
	}
	fake := &fakeNotifier{}
	(&updateSink{conn: fake, sessionID: "sess-1"}).replay(msgs)
	if len(fake.notifs) != 1 {
		t.Fatalf("replay notifications = %#v", fake.notifs)
	}
	raw, err := json.Marshal(fake.notifs[0].params)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || !strings.Contains(string(raw), "ordinary task") {
		t.Fatalf("replay payload leaked context or lost task: %s", raw)
	}
}
