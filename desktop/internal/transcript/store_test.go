package transcript

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"kwu/internal/event"
)

func TestDurableConcurrentSessionsAndIdempotency(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	for _, session := range []string{"a", "b"} {
		if err := store.Begin(session, "journal-"+session, json.RawMessage(`[]`)); err != nil {
			t.Fatal(err)
		}
	}
	var failed atomic.Int32
	done := make(chan struct{}, 2)
	for _, session := range []string{"a", "b"} {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 30; i++ {
				id := fmt.Sprint(i)
				payload, _ := json.Marshal(map[string]string{"text": session + id})
				_, first, err := store.Append(session, id, payload)
				if err != nil {
					t.Error(err)
					failed.Add(1)
					return
				}
				_, retry, err := store.Append(session, id, payload)
				if err != nil || retry != first {
					t.Errorf("retry cursor = %d, want %d: %v", retry, first, err)
					failed.Add(1)
				}
			}
		}()
	}
	<-done
	<-done
	if failed.Load() != 0 {
		t.FailNow()
	}
	// Recreate the store: recovery must read disk, not a process-local cache.
	reopened := Store{Path: store.Path}
	for _, session := range []string{"a", "b"} {
		page, err := reopened.Read(session, "", 0, 0)
		if err != nil || page == nil || len(page.Entries) != 30 {
			t.Fatalf("recover %s: %+v, %v", session, page, err)
		}
		for i, entry := range page.Entries {
			var payload map[string]string
			if err := json.Unmarshal(entry.Payload, &payload); err != nil || payload["text"] != session+fmt.Sprint(i) {
				t.Fatalf("mixed or reordered session event: %s, %v", entry.Payload, err)
			}
		}
	}
}

func TestSnapshotPagingExcludesConcurrentAppendsAndRejectsReset(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	if err := store.Begin("a", "original", json.RawMessage(`[{"role":"user","content":"old"}]`)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 502; i++ {
		if _, _, err := store.Append("a", fmt.Sprint(i), json.RawMessage(`{"text":"delta"}`)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.Read("a", "", 0, 0)
	if err != nil || first == nil || !first.HasMore || len(first.Entries) != 500 {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	if err := store.Begin("a", "new-turn", json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Append("a", "late", json.RawMessage(`{"text":"late"}`)); err != nil {
		t.Fatal(err)
	}
	last, err := store.Read("a", first.JournalID, first.Next, first.Through)
	if err != nil || last == nil || last.HasMore || len(last.Entries) != 2 || last.Through != first.Through {
		t.Fatalf("last page: %+v, %v", last, err)
	}
	if err := store.Reset("a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Begin("a", "replacement", json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("a", first.JournalID, first.Next, first.Through); err == nil {
		t.Fatal("stale cursor accepted after reset")
	}
	fresh, err := store.Read("a", "", 0, 0)
	if err != nil || fresh == nil || len(fresh.Entries) != 0 {
		t.Fatalf("old events survived reset: %+v, %v", fresh, err)
	}
}

func TestConcurrentWritersWithinOneTask(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	if err := store.Begin("task", "journal", json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{}, 4)
	for worker := 0; worker < 4; worker++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for token := 0; token < 10; token++ {
				id := fmt.Sprintf("%d-%d", worker, token)
				payload, _ := json.Marshal(map[string]string{"text": id})
				if _, _, err := store.Append("task", id, payload); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	for worker := 0; worker < 4; worker++ {
		<-done
	}
	page, err := store.Read("task", "", 0, 0)
	if err != nil || page == nil || len(page.Entries) != 40 {
		t.Fatalf("lost concurrent events: %+v, %v", page, err)
	}
	seen := map[string]bool{}
	var previous int64
	for _, entry := range page.Entries {
		var payload map[string]string
		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if seen[payload["text"]] || entry.Sequence <= previous {
			t.Fatalf("duplicate or unordered entry: %+v", entry)
		}
		seen[payload["text"]] = true
		previous = entry.Sequence
	}
}

func TestIncrementalSnapshotCapturesOnlyNewEvents(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	if err := store.Begin("task", "journal", json.RawMessage(`[{"role":"user","content":"baseline"}]`)); err != nil {
		t.Fatal(err)
	}
	_, first, err := store.Append("task", "first", json.RawMessage(`{"text":"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := store.Append("task", "second", json.RawMessage(`{"text":"second"}`))
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Read("task", "journal", first, 0)
	if err != nil || page == nil || page.Through != second || page.Next != second || len(page.Entries) != 1 || page.Baseline != nil {
		t.Fatalf("incremental snapshot: %+v, %v", page, err)
	}
	if _, _, err := store.Append("task", "late", json.RawMessage(`{"text":"late"}`)); err != nil {
		t.Fatal(err)
	}
	frozen, err := store.Read("task", "journal", first, page.Through)
	if err != nil || frozen == nil || len(frozen.Entries) != 1 || frozen.Through != second {
		t.Fatalf("incremental watermark changed: %+v, %v", frozen, err)
	}
	if err := store.Reset("task"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read("task", "journal", second, 0); err == nil {
		t.Fatal("incremental snapshot accepted a deleted journal")
	}
}

func TestRetriedEventKeepsOriginalPayloadAndCursor(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	if err := store.Begin("task", "journal", json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	_, first, err := store.Append("task", "event", json.RawMessage(`{"text":"original"}`))
	if err != nil {
		t.Fatal(err)
	}
	id, retry, err := store.Append("task", "event", json.RawMessage(`{"text":"duplicate"}`))
	if err != nil || id != "journal" || retry != first {
		t.Fatalf("retry cursor: %s/%d, %v", id, retry, err)
	}
	page, err := store.Read("task", "", 0, 0)
	if err != nil || page == nil || len(page.Entries) != 1 || string(page.Entries[0].Payload) != `{"text":"original"}` {
		t.Fatalf("retry changed persisted content: %+v, %v", page, err)
	}
}

func TestBeginJournalOwnership(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, session, id, wantID string
		wantErr                   bool
	}{
		{name: "new session", session: "c", id: "journal-c", wantID: "journal-c"},
		{name: "existing session", session: "a", id: "new-candidate", wantID: "journal-a"},
		{name: "same identity", session: "a", id: "journal-a", wantID: "journal-a"},
		{name: "new session collides", session: "c", id: "journal-a", wantErr: true},
		{name: "existing session with another journal candidate", session: "a", id: "journal-b", wantID: "journal-a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
			before := make(map[string]*Page)
			for _, session := range []string{"a", "b"} {
				baseline := json.RawMessage(fmt.Sprintf(`[{"role":"user","content":%q}]`, session))
				if err := store.Begin(session, "journal-"+session, baseline); err != nil {
					t.Fatal(err)
				}
				if _, _, err := store.Append(session, "same-event-id", json.RawMessage(`{"text":"retained"}`)); err != nil {
					t.Fatal(err)
				}
				page, err := store.Read(session, "", 0, 0)
				if err != nil {
					t.Fatal(err)
				}
				before[session] = page
			}
			err := store.Begin(test.session, test.id, json.RawMessage(`[]`))
			if (err != nil) != test.wantErr {
				t.Fatalf("Begin error=%v, wantErr=%t", err, test.wantErr)
			}
			page, err := store.Read(test.session, "", 0, 0)
			if err != nil || (test.wantErr && page != nil) || (!test.wantErr && (page == nil || page.JournalID != test.wantID)) {
				t.Fatalf("unexpected journal ownership: %+v, %v", page, err)
			}
			for session, original := range before {
				current, err := store.Read(session, "", 0, 0)
				if err != nil || !reflect.DeepEqual(current, original) {
					t.Fatalf("Begin changed existing session %s: %+v, %v", session, current, err)
				}
			}
		})
	}
}

func TestBeginConcurrentSameSessionKeepsOneBaseline(t *testing.T) {
	t.Parallel()
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	if err := store.Begin("other", "other-journal", json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	const writers = 4
	results := make(chan error, writers)
	for i := range writers {
		go func() {
			id := fmt.Sprintf("candidate-%d", i)
			baseline, _ := json.Marshal([]string{id})
			results <- store.Begin("same-session", id, baseline)
		}()
	}
	for range writers {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.Read("same-session", "", 0, 0)
	if err != nil || page == nil {
		t.Fatalf("missing shared journal: %+v, %v", page, err)
	}
	baseline, _ := json.Marshal([]string{page.JournalID})
	if string(page.Baseline) != string(baseline) {
		t.Fatalf("journal and baseline belong to different writers: %+v", page)
	}
}

func TestLegacyJournalAcceptsNamespacedEvents(t *testing.T) {
	t.Parallel()
	store := Store{Path: filepath.Join(t.TempDir(), "transcripts.sqlite")}
	baseline := json.RawMessage(`[{"role":"user","content":"legacy prompt"}]`)
	oldPayload := json.RawMessage(`{"kind":"text","eventId":"e-2","text":"legacy answer"}`)
	if err := store.Begin("legacy-session", "e-1", baseline); err != nil {
		t.Fatal(err)
	}
	_, oldSequence, err := store.Append("legacy-session", "e-2", oldPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Begin("legacy-session", event.NewEventID(), json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	id := event.NewEventID()
	newPayload, _ := json.Marshal(map[string]string{"kind": "text", "eventId": id, "text": "new answer"})
	journal, sequence, err := store.Append("legacy-session", id, newPayload)
	if err != nil || journal != "e-1" || sequence <= oldSequence {
		t.Fatalf("legacy append cursor=%s/%d: %v", journal, sequence, err)
	}
	_, retry, err := store.Append("legacy-session", id, newPayload)
	if err != nil || retry != sequence {
		t.Fatalf("retry did not retain cursor: %d, %v", retry, err)
	}
	page, err := store.Read("legacy-session", "", 0, 0)
	if err != nil || page == nil || len(page.Entries) != 2 {
		t.Fatalf("legacy replay: %+v, %v", page, err)
	}
	if string(page.Baseline) != string(baseline) || string(page.Entries[0].Payload) != string(oldPayload) || string(page.Entries[1].Payload) != string(newPayload) {
		t.Fatal("upgrade or retry rewrote persisted content")
	}
}
