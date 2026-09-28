package automation

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
)

func TestArchiveSinkWritesJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	sink, err := NewArchiveSink(path, ArchiveEventMeta{TaskID: "task-1", RunID: "run-1"}, nil)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}
	sink.now = func() time.Time { return time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC) }

	sink.Emit(event.Event{Kind: event.TurnStarted})
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "tool-1", Name: "bash", Args: `{"cmd":"date"}`}})
	sink.Emit(event.Event{Kind: event.Usage, Usage: &provider.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}})
	sink.Emit(event.Event{Kind: event.TurnDone, Err: errors.New("done failed")})
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := readArchivedEvents(path)
	if err != nil {
		t.Fatalf("readArchivedEvents: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("events len = %d, want 4", len(got))
	}
	if got[0].TaskID != "task-1" || got[0].RunID != "run-1" || got[0].Kind != event.TurnStarted {
		t.Fatalf("first event = %+v", got[0])
	}
	if got[1].Tool == nil || got[1].Tool.Name != "bash" {
		t.Fatalf("tool event = %+v", got[1])
	}
	if got[2].Usage == nil || got[2].Usage.TotalTokens != 10 {
		t.Fatalf("usage event = %+v", got[2])
	}
	if got[3].Error != "done failed" {
		t.Fatalf("error = %q, want done failed", got[3].Error)
	}
}

func TestArchiveSinkRedactsToolPayloadsBeforePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	sink, err := NewArchiveSink(path, ArchiveEventMeta{TaskID: "task-1", RunID: "run-1"}, nil)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{
		ID: "tool-1", Name: "shell", Args: `{"token":"secret-value"}`,
		Output: "secret output", Err: "secret error", FileDiff: event.FileDiff{Diff: "secret diff"},
	}})
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := readArchivedEvents(path)
	if err != nil {
		t.Fatalf("readArchivedEvents: %v", err)
	}
	if len(got) != 1 || got[0].Tool == nil {
		t.Fatalf("events = %+v, want one tool event", got)
	}
	if got[0].Tool.Args != "" || got[0].Tool.Output != "" || got[0].Tool.Err != "" || got[0].Tool.Diff != "" {
		t.Fatalf("archived tool payload was not redacted: %+v", got[0].Tool)
	}
	if got[0].Event.Tool.Args != "" || got[0].Event.Tool.Output != "" || got[0].Event.Tool.Err != "" || got[0].Event.Tool.Diff != "" {
		t.Fatalf("nested archived event payload was not redacted: %+v", got[0].Event.Tool)
	}
}

func TestArchiveSinkSerializesConcurrentEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	sink, err := NewArchiveSink(path, ArchiveEventMeta{TaskID: "task-1", RunID: "run-1"}, nil)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sink.Emit(event.Event{Kind: event.Text, Text: "chunk"})
		}()
	}
	wg.Wait()
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := readArchivedEvents(path)
	if err != nil {
		t.Fatalf("readArchivedEvents: %v", err)
	}
	if len(got) != 50 {
		t.Fatalf("events len = %d, want 50", len(got))
	}
}

func TestArchiveSinkForwardsEvents(t *testing.T) {
	var forwarded []event.Event
	forward := event.FuncSink(func(ev event.Event) {
		forwarded = append(forwarded, ev)
	})
	sink, err := NewArchiveSink(filepath.Join(t.TempDir(), "events.jsonl"), ArchiveEventMeta{TaskID: "task-1", RunID: "run-1"}, forward)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}

	sink.Emit(event.Event{Kind: event.Notice, Text: "hello"})
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(forwarded) != 1 || forwarded[0].Text != "hello" {
		t.Fatalf("forwarded = %+v, want hello notice", forwarded)
	}
}

func TestArchiveSinkRecordsWriteError(t *testing.T) {
	forwarded := 0
	forward := event.FuncSink(func(event.Event) { forwarded++ })
	sink, err := NewArchiveSink(filepath.Join(t.TempDir(), "events.jsonl"), ArchiveEventMeta{TaskID: "task-1", RunID: "run-1"}, forward)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sink.Emit(event.Event{Kind: event.Text, Text: "after close"})
	if sink.Err() == nil {
		t.Fatal("Err() = nil, want write error")
	}
	if forwarded != 0 {
		t.Fatalf("forwarded = %d, want no invalidation before durable append", forwarded)
	}
}

func readArchivedEvents(path string) ([]ArchivedEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []ArchivedEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev ArchivedEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, scanner.Err()
}

func hasArchivedStatus(events []ArchivedEvent, status RunStatus) bool {
	for _, ev := range events {
		if ev.Status == status {
			return true
		}
	}
	return false
}

func hasArchivedKind(events []ArchivedEvent, kind event.Kind) bool {
	for _, ev := range events {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}
