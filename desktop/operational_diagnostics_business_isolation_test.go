package main

import (
	"context"
	"testing"
	"time"

	"kwu/internal/control"
	"kwu/internal/event"
)

func TestDiagnosticsDisabledDoesNotAffectAskBusinessLifecycle(t *testing.T) {
	delivered := make(chan wireEventTab, 8)
	sink := &tabEventSink{tabID: "tab-diagnostics-off", ctx: context.Background()}
	sink.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name == eventChannel {
			delivered <- payload[0].(wireEventTab)
		}
	}
	trace := sink.operationalDiagnosticTrace()
	trace.SetEnabled(false)
	controller := control.New(control.Options{Sink: sink})
	answers, errs := make(chan []event.AskAnswer, 1), make(chan error, 1)
	go func() {
		result, err := controller.Ask(context.Background(), []event.AskQuestion{{ID: "question-fixture", Prompt: "deterministic fixture question"}})
		answers <- result
		errs <- err
	}()
	promptID := waitForOperationalKind(t, delivered, "prompt.emitted").Operational.Scope.PromptID
	want := []event.AskAnswer{{QuestionID: "question-fixture", Selected: []string{"fixture-answer"}}}
	if receipt := controller.AnswerQuestionWithReceipt(promptID, want); receipt.Status != event.PromptReplyAccepted {
		t.Fatalf("reply receipt = %+v", receipt)
	}
	resolved := waitForOperationalKind(t, delivered, "prompt.resolved")
	if resolved.Operational.Scope.PromptID != promptID {
		t.Fatalf("resolved prompt = %q, want %q", resolved.Operational.Scope.PromptID, promptID)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	got := <-answers
	if len(got) != 1 || got[0].QuestionID != want[0].QuestionID || len(got[0].Selected) != 1 || got[0].Selected[0] != want[0].Selected[0] {
		t.Fatalf("Ask result = %+v", got)
	}
	if records := trace.Snapshot(); len(records) != 0 {
		t.Fatalf("disabled diagnostics recorded %d entries", len(records))
	}
	if counters := trace.Counters(); counters.Disabled == 0 {
		t.Fatalf("diagnostic counters = %+v", counters)
	}
}

func waitForOperationalKind(t *testing.T, delivered <-chan wireEventTab, kind string) wireEventTab {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case wire := <-delivered:
			if wire.Operational != nil && wire.Operational.Kind == kind {
				return wire
			}
		case <-deadline:
			t.Fatalf("timed out waiting for operational kind %q", kind)
		}
	}
}
