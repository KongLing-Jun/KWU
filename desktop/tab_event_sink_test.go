package main

import (
	"context"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/event"
)

func TestTabEventSinkDoesNotBlockOnRuntimeEventsEmit(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, 2)
	var calls atomic.Int32

	sink := &tabEventSink{tabID: "tab", ctx: context.Background()}
	sink.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != eventChannel {
			t.Errorf("event name = %q, want %q", name, eventChannel)
		}
		if len(payload) != 1 {
			t.Errorf("payload count = %d, want 1", len(payload))
			return
		}
		wire, ok := payload[0].(wireEventTab)
		if !ok {
			t.Errorf("payload type = %T, want wireEventTab", payload[0])
			return
		}
		delivered <- wire.Text
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
	}

	wrapped := event.Sync(sink)
	wrapped.Emit(event.Event{Kind: event.Text, Text: "one"})

	select {
	case <-entered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first runtime emit did not start")
	}

	done := make(chan struct{})
	go func() {
		wrapped.Emit(event.Event{Kind: event.Text, Text: "two"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second event blocked behind runtime EventsEmit")
	}

	close(release)
	if got := <-delivered; got != "one" {
		t.Fatalf("first delivered event = %q, want one", got)
	}
	select {
	case got := <-delivered:
		if got != "two" {
			t.Fatalf("second delivered event = %q, want two", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second queued event was not delivered")
	}
}

func TestDetachedTabEventSinkStillEmitsConversationActivity(t *testing.T) {
	sessionPath := sessionRuntimeKey(filepath.Join(t.TempDir(), "detached.jsonl"))
	tabID := detachedRuntimeTabID(sessionPath)
	app := &App{
		ctx:         context.Background(),
		tabRuntimes: map[string]*WorkspaceTab{},
		detachedSessions: map[string]*WorkspaceTab{
			sessionPath: {
				ID:          tabID,
				Scope:       "global",
				TopicID:     "topic-detached",
				SessionPath: sessionPath,
			},
		},
	}
	delivered := make(chan wireEventTab, 1)
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != conversationActivityEventChannel {
			return
		}
		wire, ok := payload[0].(wireEventTab)
		if !ok {
			t.Errorf("payload type = %T, want wireEventTab", payload[0])
			return
		}
		delivered <- wire
	}

	// A detached sink has no visible-tab context, so agent:event is suppressed.
	(&tabEventSink{tabID: tabID, app: app}).Emit(event.Event{Kind: event.TurnDone})

	select {
	case wire := <-delivered:
		if wire.Kind != "turn_done" || wire.ConversationSessionPath != sessionPath || wire.ConversationTopicID != "topic-detached" {
			t.Fatalf("detached conversation activity = %+v", wire)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("detached turn_done did not reach the app-scoped conversation activity channel")
	}
}

func TestEmitProjectTreeChangedDoesNotBlockOnRuntimeEventsEmit(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan uint64, 2)
	var calls atomic.Int32

	app := &App{ctx: context.Background()}
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != "project-tree:changed" {
			t.Errorf("event name = %q, want project-tree:changed", name)
		}
		if len(payload) != 1 {
			t.Errorf("payload count = %d, want 1", len(payload))
			return
		}
		revision, ok := payload[0].(uint64)
		if !ok {
			t.Errorf("payload type = %T, want uint64", payload[0])
			return
		}
		delivered <- revision
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
	}

	if got := app.ProjectTreeRevision(); got != 0 {
		t.Fatalf("initial project tree revision = %d, want 0", got)
	}
	app.emitProjectTreeChanged()
	if got := app.ProjectTreeRevision(); got != 1 {
		t.Fatalf("project tree revision after first emit = %d, want 1", got)
	}
	select {
	case <-entered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first project tree runtime emit did not start")
	}

	done := make(chan struct{})
	go func() {
		app.emitProjectTreeChanged()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("project tree event blocked behind runtime EventsEmit")
	}
	if got := app.ProjectTreeRevision(); got != 2 {
		t.Fatalf("project tree revision after second emit = %d, want 2", got)
	}

	close(release)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if calls.Load() >= 2 {
			first := <-delivered
			second := <-delivered
			if first != 1 || second != 2 {
				t.Fatalf("delivered revisions = [%d %d], want [1 2]", first, second)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("runtime emit calls = %d, want at least 2", calls.Load())
}

func TestEmitModelRouteChangedPreservesAffectedTab(t *testing.T) {
	delivered := make(chan string, 1)
	app := &App{ctx: context.Background()}
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != "model-route:changed" {
			t.Errorf("event name = %q, want model-route:changed", name)
			return
		}
		if len(payload) != 1 {
			t.Errorf("payload count = %d, want 1", len(payload))
			return
		}
		tabID, ok := payload[0].(string)
		if !ok {
			t.Errorf("payload type = %T, want string", payload[0])
			return
		}
		delivered <- tabID
	}

	app.emitModelRouteChanged("tab-two")
	select {
	case tabID := <-delivered:
		if tabID != "tab-two" {
			t.Fatalf("model route event tab = %q, want tab-two", tabID)
		}
	case <-time.After(time.Second):
		t.Fatal("model route change event was not delivered")
	}
}

func TestAsyncRuntimeEmitterDrainsBacklogInOrder(t *testing.T) {
	const backlog = 256

	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, backlog)
	var calls atomic.Int32

	emitter := &asyncRuntimeEmitter{}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		if len(payload) != 1 {
			t.Errorf("payload count = %d, want 1", len(payload))
			return
		}
		value, ok := payload[0].(string)
		if !ok {
			t.Errorf("payload type = %T, want string", payload[0])
			return
		}
		delivered <- value
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
	}

	ctx := context.Background()
	for i := 0; i < backlog; i++ {
		emitter.Emit(ctx, "agent:event", strconv.Itoa(i))
	}

	select {
	case <-entered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first runtime emit did not start")
	}
	close(release)

	for i := 0; i < backlog; i++ {
		select {
		case got := <-delivered:
			if want := strconv.Itoa(i); got != want {
				t.Fatalf("delivered[%d] = %q, want %q", i, got, want)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for delivered event %d", i)
		}
	}
}
