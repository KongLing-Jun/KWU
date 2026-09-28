package boot

import (
	"errors"
	"strings"
	"testing"

	"kwu/internal/event"
)

func TestEmitMemoryCompilerRetirementNotice(t *testing.T) {
	var events []event.Event
	sink := event.FuncSink(func(e event.Event) { events = append(events, e) })

	emitMemoryCompilerRetirementNotice(sink, false, nil)
	if len(events) != 0 {
		t.Fatalf("unchanged migration emitted %+v", events)
	}

	emitMemoryCompilerRetirementNotice(sink, true, nil)
	if len(events) != 1 || events[0].Level != event.LevelInfo || !strings.Contains(events[0].Text, "no longer runs") {
		t.Fatalf("successful migration notice = %+v", events)
	}

	events = nil
	emitMemoryCompilerRetirementNotice(sink, false, errors.New("locked"))
	if len(events) != 1 || events[0].Level != event.LevelWarn || !strings.Contains(events[0].Text, "ignored") || !strings.Contains(events[0].Text, "not rewritten") || !strings.Contains(events[0].Text, "no longer runs") || !strings.Contains(events[0].Text, "locked") {
		t.Fatalf("failed migration notice = %+v", events)
	}

	events = nil
	emitMemoryCompilerRetirementNotice(sink, true, errors.New("symlinked project config"))
	if len(events) != 1 || events[0].Level != event.LevelWarn || !strings.Contains(events[0].Text, "removed from writable configs") || !strings.Contains(events[0].Text, "ignored") || !strings.Contains(events[0].Text, "no longer runs") || !strings.Contains(events[0].Text, "symlinked project config") {
		t.Fatalf("partial migration notice = %+v", events)
	}
}
