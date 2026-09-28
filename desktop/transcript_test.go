package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/eventwire"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

func TestTranscriptRestoresUndeliveredStreamAfterTabRecreation(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	actor := userctx.Default(root)
	dir := desktopSessionDirForActor(actor, root)
	path := filepath.Join(dir, "streaming.jsonl")
	session := agent.NewSession("")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "previous prompt"})
	session.Add(provider.Message{Role: provider.RoleAssistant, Content: "previous answer"})
	executor := agent.New(nil, nil, session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: executor, SessionDir: dir, SessionPath: path, Actor: actor})
	defer ctrl.Close()
	a := NewApp()
	tab := &WorkspaceTab{ID: "original-tab", Scope: "global", WorkspaceRoot: root, SessionPath: path, Ctrl: ctrl, Ready: true, actor: actor}
	sink := &tabEventSink{tabID: tab.ID, app: a} // No UI context: no frontend receives these events.
	tab.sink = sink
	a.tabRuntimes[tab.ID] = tab
	a.activeTabID = tab.ID
	a.recordAcceptedTranscript(ctrl, "expanded request", "visible request")
	sink.Emit(event.Event{Kind: event.TurnStarted, TurnID: "turn-a"})
	sink.Emit(event.Event{Kind: event.Reasoning, Text: "thinking before switch"})
	sink.Emit(event.Event{Kind: event.Text, Text: "streamed before switch"})
	sink.Emit(event.Event{Kind: event.Text, Text: " and in background"})
	// The model-facing history still lacks both the current prompt and answer.
	if got := len(ctrl.History()); got != 2 {
		t.Fatalf("unexpected model history length: %d", got)
	}
	// A recreated frontend/runtime tab must recover by session identity, even
	// without a live controller or any per-tab in-memory transcript cache.
	recreated := NewApp()
	recreated.tabRuntimes["recreated"] = &WorkspaceTab{
		ID: "recreated", Scope: "global", WorkspaceRoot: root, SessionPath: path, actor: actor,
	}
	page, err := recreated.TranscriptPageForTab("recreated", "", 0, 0)
	if err != nil || page == nil || len(page.Entries) != 5 {
		t.Fatalf("recover durable stream: %+v, %v", page, err)
	}
	var baseline []HistoryMessage
	if err := json.Unmarshal(page.Baseline, &baseline); err != nil || len(baseline) != 2 {
		t.Fatalf("baseline: %+v, %v", baseline, err)
	}
	var text, reasoning string
	for _, entry := range page.Entries {
		var wire wireEventTab
		if err := json.Unmarshal(entry.Payload, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.ConversationSessionPath != sessionRuntimeKey(path) {
			t.Fatalf("wrong session route: %+v", wire)
		}
		switch wire.Kind {
		case "text":
			text += wire.Text
		case "reasoning":
			reasoning += wire.Text
		}
	}
	if text != "streamed before switch and in background" || reasoning != "thinking before switch" {
		t.Fatalf("missing stream: text=%q reasoning=%q", text, reasoning)
	}
	var prompt wireEventTab
	if err := json.Unmarshal(page.Entries[0].Payload, &prompt); err != nil || prompt.TranscriptUser == nil || prompt.TranscriptUser.Content != "visible request" {
		t.Fatalf("missing visible user prompt: %+v, %v", prompt, err)
	}
	other := actor
	other.ProfileID = "unrelated-profile"
	if page, err := transcriptStoreForActor(other).Read(sessionRuntimeKey(path), "", 0, 0); err != nil || page != nil {
		t.Fatalf("transcript crossed profile boundary: %+v, %v", page, err)
	}
}

func TestAcceptedTranscriptSurvivesBeforeRuntimeAttachment(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	actor := userctx.Default(root)
	dir := desktopSessionDirForActor(actor, root)
	path := filepath.Join(dir, "unattached.jsonl")
	executor := agent.New(nil, nil, agent.NewSession(""), agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: executor, SessionDir: dir, SessionPath: path, Actor: actor})
	defer ctrl.Close()
	a := NewApp()
	a.recordAcceptedTranscript(ctrl, "accepted before attachment", "visible request")
	a.tabRuntimes["attached"] = &WorkspaceTab{
		ID: "attached", Scope: "global", WorkspaceRoot: root, SessionPath: path, actor: actor,
	}
	page, err := a.TranscriptPageForTab("attached", "", 0, 0)
	if err != nil || page == nil || len(page.Entries) != 1 {
		t.Fatalf("missing accepted turn: %+v, %v", page, err)
	}
	var prompt wireEventTab
	if err := json.Unmarshal(page.Entries[0].Payload, &prompt); err != nil || prompt.TranscriptUser == nil || prompt.TranscriptUser.Content != "visible request" || prompt.TranscriptUser.SubmitText != "accepted before attachment" {
		t.Fatalf("missing accepted user prompt: %+v, %v", prompt, err)
	}
	if prompt.ConversationSessionPath != sessionRuntimeKey(path) {
		t.Fatalf("accepted prompt routed to another session: %+v", prompt)
	}
	if prompt.TranscriptUser.CreatedAt <= 0 {
		t.Fatal("accepted prompt omitted its creation time")
	}
}

func TestTranscriptPersistenceFailureDoesNotClaimDeliveryCursor(t *testing.T) {
	isolateDesktopUserDirs(t)
	root := globalTabWorkspaceRoot()
	actor := userctx.Default(root)
	dir := desktopSessionDirForActor(actor, root)
	path := filepath.Join(dir, "failed-transcript.jsonl")
	ctrl := control.New(control.Options{
		Executor:   agent.New(nil, nil, agent.NewSession(""), agent.Options{}, event.Discard),
		SessionDir: dir, SessionPath: path, Actor: actor,
	})
	defer ctrl.Close()
	// A directory at the database path deterministically rejects persistence.
	if err := os.MkdirAll(transcriptStoreForActor(actor).Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := beginControllerTranscript(ctrl); err == nil {
		t.Fatal("Begin claimed success despite an unusable database")
	}
	wire := persistControllerTranscriptEvent(ctrl, wireEventTab{
		Event:                   eventwire.Event{Kind: "turn_started", EventID: event.NewEventID()},
		ConversationSessionPath: path,
	})
	if wire.TranscriptJournalID != "" || wire.TranscriptSequence != 0 {
		t.Fatalf("failed persistence returned a delivery cursor: %s/%d", wire.TranscriptJournalID, wire.TranscriptSequence)
	}
}
