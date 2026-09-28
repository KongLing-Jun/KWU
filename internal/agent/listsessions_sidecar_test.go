package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/provider"
)

func writeSessionFile(t *testing.T, path string, msgs []provider.Message) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, m := range msgs {
		if err := enc.Encode(m); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
}

// SessionPreviewFromMessages must match a from-disk decode byte-for-byte, since
// Session.Save persists exactly the messages it is handed.
func TestSessionPreviewFromMessagesMatchesDecode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260101-000000-deepseek-chat.jsonl")
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "you are helpful"},
		{Role: provider.RoleUser, Content: "first question about the bug"},
		{Role: provider.RoleAssistant, Content: "here is an answer", ReasoningContent: "thinking"},
		{Role: provider.RoleUser, Content: "follow up"},
		{Role: provider.RoleAssistant, Content: "more"},
	}
	writeSessionFile(t, path, msgs)

	filePreview, fileTurns := previewSession(path)
	memPreview, memTurns := SessionPreviewFromMessages(msgs)
	if fileTurns != memTurns || filePreview != memPreview {
		t.Fatalf("mismatch: file=(%q,%d) mem=(%q,%d)", filePreview, fileTurns, memPreview, memTurns)
	}
	if memTurns != 2 {
		t.Fatalf("expected 2 user turns, got %d", memTurns)
	}
}

// When the sidecar records Turns/Preview, ListSessions must trust them and not
// re-derive from the .jsonl. We prove that by planting counts that disagree with
// the file: if ListSessions returns the planted values, it used the sidecar.
func TestListSessionsUsesSidecarWithoutDecoding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260101-000000-deepseek-chat.jsonl")
	writeSessionFile(t, path, []provider.Message{
		{Role: provider.RoleUser, Content: "real content in the file"},
		{Role: provider.RoleAssistant, Content: "a"},
	})
	// Sidecar deliberately disagrees with the file (3 turns, custom preview).
	if err := UpdateSessionMeta(path, "", "cached preview line", 3, true); err != nil {
		t.Fatalf("UpdateSessionMeta: %v", err)
	}

	infos, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 session, got %d", len(infos))
	}
	if infos[0].Turns != 3 || infos[0].Preview != "cached preview line" {
		t.Fatalf("expected sidecar values (3, %q), got (%d, %q)", "cached preview line", infos[0].Turns, infos[0].Preview)
	}
}

// A legacy session whose sidecar has no recorded turn count must be decoded once
// and then backfilled, so the second listing reads the sidecar.
func TestListSessionsBackfillsLegacySession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260101-000000-deepseek-chat.jsonl")
	writeSessionFile(t, path, []provider.Message{
		{Role: provider.RoleUser, Content: "legacy question"},
		{Role: provider.RoleAssistant, Content: "answer"},
		{Role: provider.RoleUser, Content: "again"},
		{Role: provider.RoleAssistant, Content: "ok"},
	})
	// Sidecar exists but predates the counts (no Turns/Preview), with a fixed
	// UpdatedAt we expect backfill to preserve.
	updated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := SaveBranchMetaPreserveUpdated(path, BranchMeta{ID: BranchID(path), CreatedAt: updated, UpdatedAt: updated, Scope: "global"}); err != nil {
		t.Fatalf("SaveBranchMetaPreserveUpdated: %v", err)
	}

	infos, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(infos) != 1 || infos[0].Turns != 2 {
		t.Fatalf("expected 1 session with 2 turns, got %+v", infos)
	}

	// Backfill must have written the counts into the sidecar without bumping
	// activity time (ordering must stay stable).
	meta, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta: ok=%v err=%v", ok, err)
	}
	if meta.Turns != 2 || meta.Preview != "legacy question" {
		t.Fatalf("backfill missing: turns=%d preview=%q", meta.Turns, meta.Preview)
	}
	if !meta.UpdatedAt.Equal(updated) {
		t.Fatalf("backfill bumped UpdatedAt: got %v want %v", meta.UpdatedAt, updated)
	}
}

// A counts-authoritative sidecar (SchemaVersion stamped) that records Turns == 0
// must be trusted as empty and skipped WITHOUT decoding the .jsonl. We prove the
// version gate by planting a file that actually has content but a meta claiming
// it is empty: if the session is decoded it would be listed; trusting the meta
// skips it.
func TestListSessionsTrustsRecordedEmptyWithoutDecoding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260101-000000-deepseek-chat.jsonl")
	writeSessionFile(t, path, []provider.Message{
		{Role: provider.RoleUser, Content: "real content the meta will lie about"},
		{Role: provider.RoleAssistant, Content: "a"},
	})
	if err := SaveBranchMeta(path, BranchMeta{ID: BranchID(path), Turns: 0, SchemaVersion: BranchMetaCountsVersion}); err != nil {
		t.Fatalf("SaveBranchMeta: %v", err)
	}

	infos, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("a counts-authoritative empty session should be skipped without decoding; got %d", len(infos))
	}
}

// A genuinely-empty legacy session (no counts-aware meta) must be decoded once
// and then recorded as authoritative-empty, so later listings trust it instead
// of re-decoding the .jsonl on every refresh (the Turns == 0 overload bug).
func TestListSessionsRecordsEmptyLegacySessionOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260101-000000-deepseek-chat.jsonl")
	writeSessionFile(t, path, []provider.Message{
		{Role: provider.RoleSystem, Content: "system prompt"},
		{Role: provider.RoleAssistant, Content: "a greeting with no user turn"},
	})

	infos, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("empty session must not be listed; got %d", len(infos))
	}

	meta, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("empty session should have been stamped: ok=%v err=%v", ok, err)
	}
	if meta.SchemaVersion != BranchMetaCountsVersion || meta.Turns != 0 {
		t.Fatalf("empty session not recorded as authoritative-empty: version=%d turns=%d", meta.SchemaVersion, meta.Turns)
	}
}

func TestSessionCatalogTracksMetadataAndExpertUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.jsonl")
	writeSessionFile(t, path, []provider.Message{{Role: provider.RoleUser, Content: "first"}})
	if _, err := ListSessions(dir); err != nil {
		t.Fatalf("build catalog: %v", err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for catalog: %v", err)
	}
	if err := UpdateSessionMeta(path, "", "updated preview", 4, true); err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if err := SaveSessionExpert(path, ExpertMeta{ExpertID: "expert-1", ExpertName: "Expert One"}); err != nil {
		t.Fatalf("save expert: %v", err)
	}

	infos, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	if len(infos) != 1 || infos[0].Preview != "updated preview" || infos[0].Turns != 4 ||
		infos[0].ExpertID != "expert-1" || infos[0].ExpertName != "Expert One" {
		t.Fatalf("incremental catalog row = %+v", infos)
	}
}

func TestSessionCatalogCorruptionFallsBackAndRepairs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repair.jsonl")
	writeSessionFile(t, path, []provider.Message{{Role: provider.RoleUser, Content: "recover me"}})
	if _, err := ListSessions(dir); err != nil {
		t.Fatalf("build catalog: %v", err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for initial catalog: %v", err)
	}
	if err := os.WriteFile(sessionCatalogPath(dir), []byte("not sqlite"), 0o600); err != nil {
		t.Fatalf("corrupt catalog: %v", err)
	}

	infos, pending, err := ListSessionsContext(context.Background(), dir)
	if err != nil || !pending || len(infos) != 1 {
		t.Fatalf("bounded corruption fallback = %+v pending=%v err=%v", infos, pending, err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for corruption repair: %v", err)
	}
	infos, pending, err = ListSessionsContext(context.Background(), dir)
	if err != nil || pending || len(infos) != 1 || infos[0].Preview != "recover me" {
		t.Fatalf("recovered sessions = %+v pending=%v err=%v", infos, pending, err)
	}
	corrupt, err := filepath.Glob(sessionCatalogPath(dir) + ".corrupt-*")
	if err != nil || len(corrupt) != 1 {
		t.Fatalf("quarantined catalogs = %v / %v", corrupt, err)
	}
}

func TestSessionCatalogMissedIncrementMarksRepairNeeded(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.jsonl")
	writeSessionFile(t, first, []provider.Message{{Role: provider.RoleUser, Content: "first"}})
	if _, err := ListSessions(dir); err != nil {
		t.Fatalf("build catalog: %v", err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for catalog: %v", err)
	}

	lock := sessionCatalogLock(dir)
	lock.Lock()
	second := filepath.Join(dir, "second.jsonl")
	if err := NewSessionFromMessages([]provider.Message{{Role: provider.RoleUser, Content: "second"}}).Save(second); err != nil {
		lock.Unlock()
		t.Fatalf("save while repair lock held: %v", err)
	}
	lock.Unlock()

	infos, pending, err := ListSessionsContext(context.Background(), dir)
	if err != nil || pending || len(infos) != 2 {
		t.Fatalf("small stale catalog should repair synchronously: infos=%d pending=%v err=%v", len(infos), pending, err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for repair: %v", err)
	}
	infos, pending, err = ListSessionsContext(context.Background(), dir)
	if err != nil || pending || len(infos) != 2 {
		t.Fatalf("repaired catalog: infos=%d pending=%v err=%v", len(infos), pending, err)
	}
}

func TestSessionCatalogBootstrapIsBoundedAndEventuallyComplete(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < sessionCatalogBootstrapLimit+12; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%03d.jsonl", i))
		writeSessionFile(t, path, []provider.Message{{Role: provider.RoleUser, Content: fmt.Sprintf("session %d", i)}})
	}
	infos, pending, err := ListSessionsContext(context.Background(), dir)
	if err != nil || !pending || len(infos) > sessionCatalogBootstrapLimit {
		t.Fatalf("bootstrap infos=%d pending=%v err=%v", len(infos), pending, err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for repair: %v", err)
	}
	infos, pending, err = ListSessionsContext(context.Background(), dir)
	if err != nil || pending || len(infos) != sessionCatalogBootstrapLimit+12 {
		t.Fatalf("complete infos=%d pending=%v err=%v", len(infos), pending, err)
	}
}

func TestSessionCatalogDetectsExternalTranscriptRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "external.jsonl")
	writeSessionFile(t, path, []provider.Message{{Role: provider.RoleUser, Content: "before"}})
	if _, err := ListSessions(dir); err != nil {
		t.Fatalf("build catalog: %v", err)
	}
	writeSessionFile(t, path, []provider.Message{
		{Role: provider.RoleUser, Content: "after"},
		{Role: provider.RoleAssistant, Content: "changed"},
	})
	infos, pending, err := ListSessionsContext(context.Background(), dir)
	if err != nil || pending || len(infos) != 1 || infos[0].Preview != "after" {
		t.Fatalf("small external rewrite should repair synchronously: infos=%+v pending=%v err=%v", infos, pending, err)
	}
	if err := WaitSessionCatalogRepair(context.Background(), dir); err != nil {
		t.Fatalf("wait for repair: %v", err)
	}
	infos, pending, err = ListSessionsContext(context.Background(), dir)
	if err != nil || pending || len(infos) != 1 || infos[0].Preview != "after" {
		t.Fatalf("repaired external rewrite = %+v pending=%v err=%v", infos, pending, err)
	}
}
