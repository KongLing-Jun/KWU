package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/provider"
)

// --- NewSession ---

func TestNewSessionEmpty(t *testing.T) {
	s := NewSession("")
	if len(s.Messages) != 0 {
		t.Errorf("empty session should have 0 messages, got %d", len(s.Messages))
	}
}

func TestSnapshotRangeKeepsToolCallGroupTogether(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "old"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "old answer"})
	s.Add(provider.Message{Role: provider.RoleUser, Content: "new"})
	s.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "call-1", Name: "read_file"}}})
	s.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "call-1", Name: "read_file", Content: "result"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "final"})

	page, before, hasMore := s.SnapshotRange(0, 2)
	if before != 3 || !hasMore {
		t.Fatalf("before/hasMore = %d/%v, want 3/true", before, hasMore)
	}
	if len(page) != 3 || page[0].Role != provider.RoleAssistant || page[1].Role != provider.RoleTool {
		t.Fatalf("tool-call group was split: %+v", page)
	}
	older, olderBefore, olderMore := s.SnapshotRange(before, 2)
	if olderBefore != 1 || !olderMore || len(older) != 2 {
		t.Fatalf("older page = before %d more %v messages %+v", olderBefore, olderMore, older)
	}
}

func TestNewSessionWithSystem(t *testing.T) {
	s := NewSession("You are a helpful assistant.")
	if len(s.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != provider.RoleSystem {
		t.Errorf("role = %q, want system", s.Messages[0].Role)
	}
	if s.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("content = %q", s.Messages[0].Content)
	}
}

// --- Session.Add ---

func TestSessionAdd(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "hi there"})
	if len(s.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != provider.RoleUser {
		t.Errorf("first role = %q", s.Messages[0].Role)
	}
	if s.Messages[1].Role != provider.RoleAssistant {
		t.Errorf("second role = %q", s.Messages[1].Role)
	}
}

func TestAttachTurnMetadataMarksOnlyLastAssistantInNewTurn(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "old prompt"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "old answer"})
	start := len(s.Snapshot())
	s.Add(provider.Message{Role: provider.RoleUser, Content: "new prompt"})
	s.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "call-1", Name: "read_file"}}})
	s.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "call-1", Name: "read_file", Content: "result"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "final answer"})

	metadata := provider.TurnMetadata{
		TaskID: "task-1", ModelRef: "provider/glm-5.2", ModelID: "glm-5.2", BillingSource: "subscription_pool",
	}
	if !s.AttachTurnMetadata(start, metadata) {
		t.Fatal("AttachTurnMetadata returned false")
	}
	messages := s.Snapshot()
	if messages[1].Turn != nil || messages[3].Turn != nil {
		t.Fatalf("non-final assistant messages were marked: old=%#v tool-call=%#v", messages[1].Turn, messages[3].Turn)
	}
	if got := messages[5].Turn; got == nil || *got != metadata {
		t.Fatalf("final assistant metadata = %#v, want %#v", got, metadata)
	}
}

func TestAttachTurnMetadataRequiresNewAssistant(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "old answer"})
	start := len(s.Snapshot())
	s.Add(provider.Message{Role: provider.RoleUser, Content: "new prompt"})

	if s.AttachTurnMetadata(start, provider.TurnMetadata{TaskID: "task-1"}) {
		t.Fatal("AttachTurnMetadata marked an assistant from an earlier turn")
	}
	if got := s.Snapshot()[0].Turn; got != nil {
		t.Fatalf("earlier assistant metadata = %#v, want nil", got)
	}
}

func TestCommitArtifactDeliveryPersistsImmutableFinalAnswerContract(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("system")
	const artifactTurnID = "artifact-turn-1"
	s.BeginArtifactDeliveryTurn(artifactTurnID)
	s.Add(provider.Message{Role: provider.RoleUser, Content: "create a report"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "done", MessageKind: provider.MessageKindFinalAnswer})
	s.EndArtifactDeliveryTurn(artifactTurnID)
	if candidate := s.Snapshot()[2]; candidate.MessageKind != provider.MessageKindUncommittedFinalAnswer || candidate.ArtifactDelivery != nil {
		t.Fatalf("pre-commit candidate = %#v, want uncommitted without a contract", candidate)
	}

	input := provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"reports/result.md"}, DeliveredAt: 100}
	committed, ok := s.CommitArtifactDelivery(artifactTurnID, input)
	if !ok || committed.Version != provider.ArtifactDeliveryVersion || len(committed.Paths) != 1 || committed.Paths[0] != "reports/result.md" {
		t.Fatalf("CommitArtifactDelivery() = %#v, %v", committed, ok)
	}
	input.Paths[0] = "mutated-input.md"
	committed.Paths[0] = "mutated-return.md"

	replayed, ok := s.CommitArtifactDelivery(artifactTurnID, provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"replacement.md"}, DeliveredAt: 200})
	if !ok || len(replayed.Paths) != 1 || replayed.Paths[0] != "reports/result.md" || replayed.DeliveredAt != 100 {
		t.Fatalf("repeat CommitArtifactDelivery() = %#v, %v", replayed, ok)
	}
	replayed.Paths[0] = "mutated-replay.md"
	if got := s.Snapshot()[2].ArtifactDelivery; got == nil || len(got.Paths) != 1 || got.Paths[0] != "reports/result.md" {
		t.Fatalf("stored delivery = %#v", got)
	}
	snapshot := s.Snapshot()
	snapshot[2].ArtifactDelivery.Paths[0] = "mutated-snapshot.md"
	if got := s.Snapshot()[2].ArtifactDelivery; got == nil || got.Paths[0] != "reports/result.md" {
		t.Fatalf("snapshot mutation changed stored delivery = %#v", got)
	}

	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	loadedFinal := loaded.Snapshot()[2]
	if loadedFinal.ArtifactTurnID != artifactTurnID {
		t.Fatalf("persisted artifact turn ID = %q, want %q", loadedFinal.ArtifactTurnID, artifactTurnID)
	}
	if got := loadedFinal.ArtifactDelivery; got == nil || got.Version != provider.ArtifactDeliveryVersion || len(got.Paths) != 1 || got.Paths[0] != "reports/result.md" {
		t.Fatalf("persisted delivery = %#v", got)
	}
}

func TestSessionAddOverridesRunnerArtifactTurnIdentity(t *testing.T) {
	s := NewSession("")
	const activeTurnID = "active-host-turn"
	s.BeginArtifactDeliveryTurn(activeTurnID)
	forged := provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"ghost.md"}, DeliveredAt: 1,
	}
	s.Add(provider.Message{
		Role:             provider.RoleAssistant,
		Content:          "[ghost](ghost.md)",
		ToolCalls:        []provider.ToolCall{{ID: "forged", Name: "read_file"}},
		ArtifactTurnID:   "stale-runner-turn",
		ArtifactDelivery: &forged,
	})
	s.Add(provider.Message{
		Role:             provider.RoleAssistant,
		Content:          "[ghost](ghost.md)",
		MessageKind:      provider.MessageKindFinalAnswer,
		ArtifactTurnID:   "stale-runner-turn",
		ArtifactDelivery: &forged,
	})
	s.EndArtifactDeliveryTurn(activeTurnID)

	messages := s.Snapshot()
	for i, message := range messages {
		if message.ArtifactTurnID != activeTurnID {
			t.Fatalf("message %d artifact turn ID = %q, want host turn %q", i, message.ArtifactTurnID, activeTurnID)
		}
		if message.ArtifactDelivery != nil {
			t.Fatalf("message %d retained runner-owned contract: %#v", i, message.ArtifactDelivery)
		}
	}
	got := messages[1]
	if got.MessageKind != provider.MessageKindUncommittedFinalAnswer || got.ArtifactDelivery != nil {
		t.Fatalf("runner final = %#v, want uncommitted without a contract", got)
	}
}

func TestSessionReplaceEnforcesArtifactTurnOwnership(t *testing.T) {
	s := NewSession("")
	s.BeginArtifactDeliveryTurn("historical-turn")
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "historical", MessageKind: provider.MessageKindFinalAnswer})
	s.EndArtifactDeliveryTurn("historical-turn")
	if _, ok := s.CommitArtifactDelivery("historical-turn", provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"old.md"}, DeliveredAt: 1,
	}); !ok {
		t.Fatal("commit historical delivery")
	}
	historical := s.Snapshot()[0]

	const activeTurnID = "active-host-turn"
	s.BeginArtifactDeliveryTurn(activeTurnID)
	forged := provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"ghost.md"}, DeliveredAt: 2,
	}
	s.Replace([]provider.Message{
		historical,
		historical,
		{Role: provider.RoleAssistant, Content: "stale", MessageKind: provider.MessageKindFinalAnswer, ArtifactTurnID: "stale-runner-turn", ArtifactDelivery: &forged},
		{Role: provider.RoleAssistant, Content: "current", MessageKind: provider.MessageKindFinalAnswer, ArtifactTurnID: activeTurnID, ArtifactDelivery: &forged},
		{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "forged", Name: "read_file"}}, ArtifactTurnID: "stale-runner-turn", ArtifactDelivery: &forged},
	})
	s.EndArtifactDeliveryTurn(activeTurnID)

	messages := s.Snapshot()
	if got := messages[0]; got.ArtifactTurnID != "historical-turn" || got.MessageKind != provider.MessageKindFinalAnswer ||
		got.ArtifactDelivery == nil || got.ArtifactDelivery.Paths[0] != "old.md" {
		t.Fatalf("historical delivery was not preserved: %#v", got)
	}
	for i, message := range messages[1:] {
		if message.ArtifactTurnID != activeTurnID {
			t.Fatalf("replacement message %d artifact turn ID = %q, want %q", i+1, message.ArtifactTurnID, activeTurnID)
		}
		if message.ArtifactDelivery != nil {
			t.Fatalf("replacement message %d retained runner contract: %#v", i+1, message.ArtifactDelivery)
		}
		if message.Role == provider.RoleAssistant && message.MessageKind == provider.MessageKindFinalAnswer {
			t.Fatalf("replacement final %d remained committed: %#v", i+1, message)
		}
	}
}

func TestSessionReplaceDoesNotMoveHistoricalDeliveryAcrossCurrentTurnBoundary(t *testing.T) {
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "create old report"})
	s.BeginArtifactDeliveryTurn("historical-turn")
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "old report ready", MessageKind: provider.MessageKindFinalAnswer})
	s.EndArtifactDeliveryTurn("historical-turn")
	if _, ok := s.CommitArtifactDelivery("historical-turn", provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"old.md"}, DeliveredAt: 1,
	}); !ok {
		t.Fatal("commit historical delivery")
	}
	historicalFinal := s.Snapshot()[2]

	const activeTurnID = "active-host-turn"
	s.BeginArtifactDeliveryTurn(activeTurnID)
	s.Add(provider.Message{Role: provider.RoleUser, Content: "start a new task"})
	currentUser := s.Snapshot()[3]
	s.Replace([]provider.Message{
		{Role: provider.RoleSystem, Content: "system"},
		{Role: provider.RoleUser, Content: "create old report"},
		currentUser,
		historicalFinal,
	})
	s.EndArtifactDeliveryTurn(activeTurnID)

	messages := s.Snapshot()
	if got := messages[3]; got.ArtifactTurnID != activeTurnID ||
		got.MessageKind != provider.MessageKindUncommittedFinalAnswer || got.ArtifactDelivery != nil {
		t.Fatalf("moved historical final = %#v, want current-turn uncommitted candidate without delivery", got)
	}
}

func TestCommitArtifactDeliveryRequiresFinalAnswerAndKnownVersion(t *testing.T) {
	s := NewSession("")
	s.BeginArtifactDeliveryTurn("ordinary-turn")
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "ordinary assistant message"})
	s.EndArtifactDeliveryTurn("ordinary-turn")
	if _, ok := s.CommitArtifactDelivery("ordinary-turn", provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, DeliveredAt: 1}); ok {
		t.Fatal("CommitArtifactDelivery accepted a non-final assistant message")
	}
	s.BeginArtifactDeliveryTurn("final-turn")
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "final", MessageKind: provider.MessageKindFinalAnswer})
	s.EndArtifactDeliveryTurn("final-turn")
	if _, ok := s.CommitArtifactDelivery("final-turn", provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion + 1, DeliveredAt: 1}); ok {
		t.Fatal("CommitArtifactDelivery accepted an unknown version")
	}
	if got := s.Snapshot()[1].ArtifactDelivery; got != nil {
		t.Fatalf("unknown version was persisted: %#v", got)
	}
}

func TestCommitArtifactDeliveryPreservesExplicitEmptyPaths(t *testing.T) {
	s := NewSession("")
	const artifactTurnID = "empty-delivery-turn"
	s.BeginArtifactDeliveryTurn(artifactTurnID)
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "done", MessageKind: provider.MessageKindFinalAnswer})
	s.EndArtifactDeliveryTurn(artifactTurnID)
	committed, ok := s.CommitArtifactDelivery(artifactTurnID, provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{}, DeliveredAt: 1})
	if !ok || committed.Paths == nil {
		t.Fatalf("empty artifact paths = %#v, %v; want an explicit empty list", committed.Paths, ok)
	}
	raw, err := json.Marshal(s.Snapshot()[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"artifact_delivery":{"version":1,"paths":[],"deliveredAt":1}`) {
		t.Fatalf("empty artifact delivery JSON = %s", raw)
	}
}

func TestDowngradeArtifactDeliveryFinalClearsOnlyMatchingAcceptedFinal(t *testing.T) {
	s := NewSession("")
	s.BeginArtifactDeliveryTurn("old-turn")
	s.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "old"})
	s.EndArtifactDeliveryTurn("old-turn")
	if _, ok := s.CommitArtifactDelivery("old-turn", provider.ArtifactDelivery{Version: 1, Paths: []string{"old.md"}, DeliveredAt: 10}); !ok {
		t.Fatal("commit old delivery")
	}
	s.BeginArtifactDeliveryTurn("failed-turn")
	s.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "[report](report.md)"})
	s.EndArtifactDeliveryTurn("failed-turn")
	if _, ok := s.CommitArtifactDelivery("failed-turn", provider.ArtifactDelivery{Version: 1, Paths: []string{"report.md"}, DeliveredAt: 20}); !ok {
		t.Fatal("commit failed-turn delivery")
	}

	if s.DowngradeArtifactDeliveryFinal("missing-turn") {
		t.Fatal("downgrade matched an unrelated turn")
	}
	if !s.DowngradeArtifactDeliveryFinal("failed-turn") {
		t.Fatal("downgrade did not find the accepted final")
	}
	messages := s.Snapshot()
	if messages[0].MessageKind != provider.MessageKindFinalAnswer || messages[0].ArtifactDelivery == nil {
		t.Fatalf("old final changed: %#v", messages[0])
	}
	if messages[1].MessageKind != provider.MessageKindUncommittedFinalAnswer || messages[1].ArtifactDelivery != nil {
		t.Fatalf("failed final = %#v, want uncommitted without a contract", messages[1])
	}
	if !s.DowngradeArtifactDeliveryFinal("failed-turn") {
		t.Fatal("downgrade should keep reporting the matching uncommitted candidate for durable persistence")
	}
}

// --- Session.HasContent ---

func TestHasContentEmpty(t *testing.T) {
	s := NewSession("")
	if s.HasContent() {
		t.Error("empty session should not have content")
	}
}

func TestHasContentSystemOnly(t *testing.T) {
	s := NewSession("system prompt")
	if s.HasContent() {
		t.Error("system-only session should not have content")
	}
}

func TestHasContentWithUser(t *testing.T) {
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	if !s.HasContent() {
		t.Error("session with user message should have content")
	}
}

func TestHasContentWithAssistant(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "response"})
	if !s.HasContent() {
		t.Error("session with assistant message should have content")
	}
}

func TestHasContentWithTool(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleTool, Content: "result", ToolCallID: "tc1"})
	if !s.HasContent() {
		t.Error("session with tool message should have content")
	}
}

// --- Session.HasSystemMessage ---

func TestHasSystemMessageWithSystem(t *testing.T) {
	s := NewSession("system prompt")
	if !s.HasSystemMessage() {
		t.Error("session with system message should report HasSystemMessage true")
	}
}

func TestHasSystemMessageWithoutSystem(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	if s.HasSystemMessage() {
		t.Error("session without system message should report HasSystemMessage false")
	}
}

func TestHasSystemMessageRequiresSystemAsFirstMessage(t *testing.T) {
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	s.Add(provider.Message{Role: provider.RoleSystem, Content: "late system"})
	if s.HasSystemMessage() {
		t.Error("session with non-leading system message should report HasSystemMessage false")
	}
}

func TestHasSystemMessageAfterReplaceWithoutSystem(t *testing.T) {
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "kept"})
	s.Replace([]provider.Message{
		{Role: provider.RoleUser, Content: "replaced"},
	})
	if !s.HasContent() {
		t.Error("session with user message should have content")
	}
	if s.HasSystemMessage() {
		t.Error("session replaced without system message should report HasSystemMessage false")
	}
}

func TestHasSystemMessageCompactedKeepsSystem(t *testing.T) {
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "answer"})
	s.Replace([]provider.Message{
		{Role: provider.RoleSystem, Content: "system"},
		{Role: provider.RoleUser, Content: "summary"},
	})
	if !s.HasSystemMessage() {
		t.Error("compacted session should keep system message at index 0")
	}
}

// --- Save / LoadSession round-trip ---

func TestSaveLoadSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	s := NewSession("system prompt")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	turn := provider.TurnMetadata{TaskID: "task-1", ModelRef: "provider/glm-5.2", ModelID: "glm-5.2", BillingSource: "subscription_pool"}
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "world", Turn: &turn})
	if err := s.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Messages) != 3 {
		t.Fatalf("want 3 messages, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Content != "system prompt" {
		t.Errorf("system = %q", loaded.Messages[0].Content)
	}
	if loaded.Messages[1].Content != "hello" {
		t.Errorf("user = %q", loaded.Messages[1].Content)
	}
	if loaded.Messages[2].Content != "world" {
		t.Errorf("assistant = %q", loaded.Messages[2].Content)
	}
	if loaded.Messages[2].Turn == nil || *loaded.Messages[2].Turn != turn {
		t.Errorf("assistant turn = %#v, want %#v", loaded.Messages[2].Turn, turn)
	}
}

func TestSaveEmptyPath(t *testing.T) {
	s := NewSession("")
	if err := s.Save(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestSaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "session.jsonl")
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "test"})
	if err := s.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("session file should exist")
	}
}

func TestSaveIfChangedSkipsPersistedGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "first"})

	snapshot, saved, err := s.SaveIfChanged(path)
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}
	if !saved || len(snapshot) != 2 {
		t.Fatalf("initial save = saved %v, %d messages; want true, 2", saved, len(snapshot))
	}
	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	snapshot, saved, err = s.SaveIfChanged(path)
	if err != nil {
		t.Fatalf("unchanged save: %v", err)
	}
	if saved || snapshot != nil {
		t.Fatalf("unchanged save = saved %v, snapshot %#v; want skipped", saved, snapshot)
	}
	second, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Fatalf("unchanged save rewrote transcript: first=%s second=%s", first.ModTime(), second.ModTime())
	}

	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "second"})
	snapshot, saved, err = s.SaveIfChanged(path)
	if err != nil {
		t.Fatalf("changed save: %v", err)
	}
	if !saved || len(snapshot) != 3 {
		t.Fatalf("changed save = saved %v, %d messages; want true, 3", saved, len(snapshot))
	}
}

func TestLoadedSessionSaveIfChangedStartsClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "persisted"})
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot, saved, err := loaded.SaveIfChanged(path); err != nil {
		t.Fatal(err)
	} else if saved || snapshot != nil {
		t.Fatalf("loaded clean session save = saved %v, snapshot %#v; want skipped", saved, snapshot)
	}
}

func TestOlderSavedSnapshotDoesNotClearNewerGeneration(t *testing.T) {
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	snapshotGeneration := s.contentGeneration
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "arrived during save"})

	s.markPersisted(filepath.Clean("session.jsonl"), snapshotGeneration)
	if s.persistedGeneration != snapshotGeneration {
		t.Fatalf("persisted generation = %d, want old snapshot %d", s.persistedGeneration, snapshotGeneration)
	}
	if s.persistedGeneration == s.contentGeneration {
		t.Fatalf("old snapshot cleared newer dirty generation %d", s.contentGeneration)
	}
}

func TestLoadSessionMissing(t *testing.T) {
	_, err := LoadSession("/nonexistent/session.jsonl")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("error should be os.IsNotExist, got %v", err)
	}
}

func TestLoadSessionMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(path, []byte("not valid json\n"), 0o644)
	_, err := LoadSession(path)
	if err == nil {
		t.Fatal("expected error for malformed JSONL")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error should mention decode: %v", err)
	}
}

// --- ListSessions ---

func TestListSessionsMissingDirReturnsNil(t *testing.T) {
	sessions, err := ListSessions("/nonexistent/dir")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %v", sessions)
	}
}

func TestListSessionsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("want 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsSorted(t *testing.T) {
	dir := t.TempDir()
	// Create two sessions with different content.
	s1 := NewSession("")
	s1.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	s1.Save(filepath.Join(dir, "a.jsonl"))

	s2 := NewSession("")
	s2.Add(provider.Message{Role: provider.RoleUser, Content: "second"})
	s2.Save(filepath.Join(dir, "b.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(sessions))
	}
	// Newest first.
	if sessions[0].ModTime.Before(sessions[1].ModTime) {
		t.Error("sessions should be sorted newest first")
	}
}

func TestListSessionsSkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	// A session with only a system prompt (no user interaction) should be skipped.
	s := NewSession("system only")
	s.Save(filepath.Join(dir, "empty.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("empty sessions should be skipped, got %d", len(sessions))
	}
}

func TestListSessionsSkipsNonJSONL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a session"), 0o644)
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "real"})
	s.Save(filepath.Join(dir, "real.jsonl"))

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("want 1 session, got %d", len(sessions))
	}
}

// --- previewSession ---

func TestPreviewSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "Help me debug the auth module"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "Sure, let me look..."})
	s.Save(path)

	preview, turns := previewSession(path)
	if turns != 1 {
		t.Errorf("turns = %d, want 1", turns)
	}
	if !strings.Contains(preview, "debug") {
		t.Errorf("preview = %q", preview)
	}
}

func TestPreviewSessionStripsTransientReasoningLanguageBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "<reasoning-language>\nVisible reasoning/thinking text preference: use Simplified Chinese.\n</reasoning-language>\n\nHelp me debug the auth module"})
	s.Save(path)

	preview, turns := previewSession(path)
	if turns != 1 {
		t.Errorf("turns = %d, want 1", turns)
	}
	if preview != "Help me debug the auth module" {
		t.Errorf("preview = %q, want user prompt", preview)
	}
}

func TestPreviewSessionStripsTransientResponseLanguageBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("system")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "<response-language>\nFinal answer language preference: use English.\n</response-language>\n\nHelp me debug the auth module"})
	s.Save(path)

	preview, turns := previewSession(path)
	if turns != 1 {
		t.Errorf("turns = %d, want 1", turns)
	}
	if preview != "Help me debug the auth module" {
		t.Errorf("preview = %q, want user prompt", preview)
	}
}

func TestPreviewSessionLongMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: strings.Repeat("a", 200)})
	s.Save(path)

	preview, _ := previewSession(path)
	if len([]rune(preview)) > 80 {
		t.Errorf("preview should be capped at 80 runes, got %d", len([]rune(preview)))
	}
	if !strings.HasSuffix(preview, "…") {
		t.Errorf("truncated preview should end with …, got %q", preview)
	}
}

func TestPreviewSessionMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	os.WriteFile(path, []byte("not json\n"), 0o644)
	preview, turns := previewSession(path)
	if turns != 0 {
		t.Errorf("turns = %d, want 0", turns)
	}
	if preview != "" {
		t.Errorf("preview = %q, want empty", preview)
	}
}

// --- NewSessionPath ---

func TestNewSessionPath(t *testing.T) {
	dir := t.TempDir()
	path := NewSessionPath(dir, "deepseek-chat")
	if !strings.HasSuffix(path, ".jsonl") {
		t.Errorf("should end with .jsonl: %s", path)
	}
	if !strings.Contains(path, "deepseek-chat") {
		t.Errorf("should contain model name: %s", path)
	}
	if !strings.HasPrefix(path, dir) {
		t.Errorf("should be under dir: %s", path)
	}
}

func TestNewSessionPathSanitizesSlashes(t *testing.T) {
	path := NewSessionPath("/dir", "provider/model")
	base := filepath.Base(path)
	if strings.Contains(base, "/") {
		t.Errorf("filename should not contain /: %s", base)
	}
	if !strings.Contains(base, "provider-model") {
		t.Errorf("slashes should be replaced: %s", base)
	}
}

func TestNewSessionPathEmptyModel(t *testing.T) {
	path := NewSessionPath("/dir", "")
	if !strings.Contains(path, "session") {
		t.Errorf("empty model should use 'session' fallback: %s", path)
	}
}
