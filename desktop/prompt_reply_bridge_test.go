package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"kwu/internal/control"
	"kwu/internal/event"
)

type desktopPromptRecorder struct {
	mu       sync.Mutex
	promptID string
}

func (r *desktopPromptRecorder) Emit(e event.Event) {
	if e.Operational == nil || e.Operational.Scope.PromptID == "" {
		return
	}
	r.mu.Lock()
	r.promptID = e.Operational.Scope.PromptID
	r.mu.Unlock()
}

func (r *desktopPromptRecorder) waitPromptID(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		id := r.promptID
		r.mu.Unlock()
		if id != "" {
			return id
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for operational prompt")
	return ""
}

func TestAnswerQuestionForTabReturnsWrongTabAndAcceptedReceipts(t *testing.T) {
	recorder := &desktopPromptRecorder{}
	owner := control.New(control.Options{Sink: recorder})
	other := control.New(control.Options{Sink: event.Discard})
	app := &App{tabRuntimes: map[string]*WorkspaceTab{
		"tab-A": {ID: "tab-A", Ctrl: owner},
		"tab-B": {ID: "tab-B", Ctrl: other},
	}}

	done := make(chan error, 1)
	go func() {
		_, err := owner.Ask(context.Background(), []event.AskQuestion{{ID: "q-1", Prompt: "fixture"}})
		done <- err
	}()
	promptID := recorder.waitPromptID(t)

	wrongTab := app.AnswerQuestionForTab("tab-B", promptID, []QuestionAnswer{{QuestionID: "q-1", Selected: []string{"A"}}})
	if wrongTab.Status != string(event.PromptReplyWrongTab) || !owner.OwnsPrompt(promptID) {
		t.Fatalf("wrong-tab receipt/pending = %+v / %v", wrongTab, owner.OwnsPrompt(promptID))
	}
	accepted := app.AnswerQuestionForTab("tab-A", promptID, []QuestionAnswer{{QuestionID: "q-1", Selected: []string{"A"}}})
	if accepted.Status != string(event.PromptReplyAccepted) {
		t.Fatalf("accepted receipt = %+v", accepted)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("accepted answer did not unblock owner controller")
	}
}

func TestApproveTabReturnsPromptNotFound(t *testing.T) {
	ctrl := control.New(control.Options{Sink: event.Discard})
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab-A": {ID: "tab-A", Ctrl: ctrl}}}
	receipt := app.ApproveTab("tab-A", "p-missing", true, false, false)
	if receipt.Status != string(event.PromptReplyNotFound) || receipt.PromptID != "p-missing" {
		t.Fatalf("missing receipt = %+v", receipt)
	}
}

func TestRecordPromptReplyFailureForTabPreservesWrongTabAndReturnsOccurrence(t *testing.T) {
	recorder := &desktopPromptRecorder{}
	owner := control.New(control.Options{Sink: recorder})
	other := control.New(control.Options{Sink: event.Discard})
	app := &App{tabRuntimes: map[string]*WorkspaceTab{
		"tab-A": {ID: "tab-A", Ctrl: owner},
		"tab-B": {ID: "tab-B", Ctrl: other},
	}}

	done := make(chan error, 1)
	go func() {
		_, err := owner.Ask(context.Background(), []event.AskQuestion{{ID: "q-1", Prompt: "fixture"}})
		done <- err
	}()
	promptID := recorder.waitPromptID(t)

	wrongTab := app.RecordPromptReplyFailureForTab("tab-B", promptID)
	if wrongTab.Status != string(event.PromptReplyWrongTab) || wrongTab.OccurrenceID != "" || !owner.OwnsPrompt(promptID) {
		t.Fatalf("wrong-tab failure receipt/pending = %+v / %v", wrongTab, owner.OwnsPrompt(promptID))
	}
	failed := app.RecordPromptReplyFailureForTab("tab-A", promptID)
	if failed.Status != string(event.PromptReplyFailedStatus) || failed.OccurrenceID == "" || !failed.Retryable || !owner.OwnsPrompt(promptID) {
		t.Fatalf("owner failure receipt/pending = %+v / %v", failed, owner.OwnsPrompt(promptID))
	}

	accepted := app.AnswerQuestionForTab("tab-A", promptID, []QuestionAnswer{{QuestionID: "q-1", Selected: []string{"A"}}})
	if accepted.Status != string(event.PromptReplyAccepted) {
		t.Fatalf("accepted receipt after retryable failure = %+v", accepted)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("accepted answer did not unblock owner controller")
	}
}
