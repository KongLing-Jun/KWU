package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"kwu/internal/event"
)

type promptFactRecorder struct {
	mu              sync.Mutex
	facts           []event.OperationalEnvelope
	legacyApprovals []event.Approval
	legacyAsks      []event.Ask
}

func (r *promptFactRecorder) Emit(e event.Event) {
	r.mu.Lock()
	if e.Operational != nil {
		r.facts = append(r.facts, *e.Operational)
	}
	if e.Kind == event.ApprovalRequest {
		r.legacyApprovals = append(r.legacyApprovals, e.Approval)
	}
	if e.Kind == event.AskRequest {
		r.legacyAsks = append(r.legacyAsks, e.Ask)
	}
	r.mu.Unlock()
}

func (r *promptFactRecorder) snapshot() []event.OperationalEnvelope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event.OperationalEnvelope(nil), r.facts...)
}

func (r *promptFactRecorder) legacySnapshot() ([]event.Approval, []event.Ask) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]event.Approval(nil), r.legacyApprovals...), append([]event.Ask(nil), r.legacyAsks...)
}

func waitPromptFacts(t *testing.T, recorder *promptFactRecorder, count int) []event.OperationalEnvelope {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		facts := recorder.snapshot()
		if len(facts) >= count {
			return facts
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("prompt facts = %d, want at least %d", len(recorder.snapshot()), count)
	return nil
}

func TestApprovalOperationalLifecycleAndTypedReceipts(t *testing.T) {
	recorder := &promptFactRecorder{}
	c := New(Options{Sink: recorder})
	done := make(chan error, 1)
	go func() {
		_, _, err := gateApprover{c}.Approve(context.Background(), "bash", "go test ./...", nil)
		done <- err
	}()
	facts := waitPromptFacts(t, recorder, 2)
	if facts[0].Kind != event.PromptRegistered || facts[1].Kind != event.PromptEmitted {
		t.Fatalf("initial lifecycle = %s, %s", facts[0].Kind, facts[1].Kind)
	}
	promptID := facts[0].Scope.PromptID
	if promptID == "" || facts[1].Scope.PromptID != promptID || facts[0].Aggregate.Sequence != 1 || facts[1].Aggregate.Sequence != 2 {
		t.Fatalf("initial prompt identity/sequence = %+v / %+v", facts[0], facts[1])
	}
	legacyApprovals, _ := recorder.legacySnapshot()
	if len(legacyApprovals) != 1 || legacyApprovals[0].ID != promptID || legacyApprovals[0].Tool != "bash" || legacyApprovals[0].Subject != "go test ./..." {
		t.Fatalf("legacy ApprovalRequest projection = %+v", legacyApprovals)
	}
	if receipt := c.ApproveWithReceipt(promptID, true, false, false); receipt.Status != event.PromptReplyAccepted {
		t.Fatalf("Approve receipt = %+v", receipt)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	facts = waitPromptFacts(t, recorder, 4)
	if facts[2].Kind != event.PromptReplyRequested || facts[3].Kind != event.PromptResolved {
		t.Fatalf("terminal lifecycle = %s, %s", facts[2].Kind, facts[3].Kind)
	}
	for _, fact := range facts[2:4] {
		if fact.Prompt == nil || fact.Prompt.Reply == nil || fact.Prompt.Reply.Action != event.PromptReplyAllow {
			t.Fatalf("approval reply metadata = %+v", fact.Prompt)
		}
	}
	for i, fact := range facts[:4] {
		if fact.Scope.PromptID != promptID || fact.Aggregate.Sequence != uint64(i+1) {
			t.Fatalf("fact %d identity/sequence = %+v", i, fact)
		}
		if i > 0 && fact.CausationID != facts[i-1].EventID {
			t.Fatalf("fact %d causation = %q, want %q", i, fact.CausationID, facts[i-1].EventID)
		}
	}
	if receipt := c.ApproveWithReceipt(promptID, true, false, false); receipt.Status != event.PromptReplyAlreadyResolved {
		t.Fatalf("duplicate receipt = %+v", receipt)
	}
	facts = waitPromptFacts(t, recorder, 5)
	if facts[4].EventID != facts[3].EventID || facts[4].Aggregate.Sequence != facts[3].Aggregate.Sequence || facts[4].Kind != event.PromptResolved {
		t.Fatalf("authoritative terminal resync = %+v, want original %+v", facts[4], facts[3])
	}
	if receipt := c.ApproveWithReceipt("p-unknown", true, false, false); receipt.Status != event.PromptReplyNotFound {
		t.Fatalf("unknown receipt = %+v", receipt)
	}
}

func TestApprovalDecisionMetadataContracts(t *testing.T) {
	tests := []struct {
		name                    string
		allow, session, persist bool
		wantAction              event.PromptReplyAction
	}{
		{name: "allow", allow: true, wantAction: event.PromptReplyAllow},
		{name: "deny", wantAction: event.PromptReplyDeny},
		{name: "session", allow: true, session: true, wantAction: event.PromptReplyAllow},
		{name: "persist", allow: true, persist: true, wantAction: event.PromptReplyAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &promptFactRecorder{}
			c := New(Options{Sink: recorder})
			result := make(chan bool, 1)
			go func() {
				allow, _, _ := gateApprover{c}.Approve(context.Background(), "bash", "echo safe", nil)
				result <- allow
			}()
			facts := waitPromptFacts(t, recorder, 2)
			id := facts[0].Scope.PromptID
			if got := c.ApproveWithReceipt(id, tt.allow, tt.session, tt.persist); got.Status != event.PromptReplyAccepted {
				t.Fatalf("receipt = %+v", got)
			}
			if got := <-result; got != tt.allow {
				t.Fatalf("decision = %v, want %v", got, tt.allow)
			}
			facts = waitPromptFacts(t, recorder, 4)
			for _, fact := range facts[2:4] {
				reply := fact.Prompt.Reply
				if reply == nil || reply.Action != tt.wantAction || reply.SessionGrant != (tt.allow && tt.session) || reply.Persistent != (tt.allow && tt.persist) {
					t.Fatalf("reply metadata = %+v", reply)
				}
			}
			legacy, _ := recorder.legacySnapshot()
			if len(legacy) != 1 || legacy[0].ID != id {
				t.Fatalf("legacy projection = %+v", legacy)
			}
		})
	}
}

func TestAskMultiQuestionFailureAndRetryContract(t *testing.T) {
	recorder := &promptFactRecorder{}
	c := New(Options{Sink: recorder})
	questions := []event.AskQuestion{{ID: "q1", Prompt: "first"}, {ID: "q2", Prompt: "second"}}
	wantAnswers := []event.AskAnswer{{QuestionID: "q1", Selected: []string{"A"}}, {QuestionID: "q2", Selected: []string{"typed"}}}
	done := make(chan []event.AskAnswer, 1)
	go func() {
		answers, _ := c.Ask(context.Background(), questions)
		done <- answers
	}()
	facts := waitPromptFacts(t, recorder, 2)
	id := facts[0].Scope.PromptID
	_, legacyAsks := recorder.legacySnapshot()
	if len(legacyAsks) != 1 || legacyAsks[0].ID != id || len(legacyAsks[0].Questions) != 2 {
		t.Fatalf("legacy AskRequest projection = %+v", legacyAsks)
	}
	failure := c.RecordPromptReplyFailure(id)
	if failure.Status != event.PromptReplyFailedStatus || !failure.Retryable || failure.OccurrenceID == "" {
		t.Fatalf("failure receipt = %+v", failure)
	}
	if got := c.AnswerQuestionWithReceipt(id, wantAnswers); got.Status != event.PromptReplyAccepted {
		t.Fatalf("retry receipt = %+v", got)
	}
	if got := <-done; len(got) != 2 || got[0].QuestionID != "q1" || len(got[1].Selected) != 1 || got[1].Selected[0] != "typed" {
		t.Fatalf("answers = %+v", got)
	}
	facts = waitPromptFacts(t, recorder, 5)
	if facts[2].Kind != event.PromptReplyFailed || facts[3].Kind != event.PromptReplyRequested || facts[4].Kind != event.PromptResolved {
		t.Fatalf("ask failure/retry lifecycle = %+v", facts)
	}
	for _, fact := range facts[3:5] {
		reply := fact.Prompt.Reply
		if reply == nil || reply.Action != event.PromptReplyAnswered || reply.AnswerCount != 2 {
			t.Fatalf("ask reply metadata = %+v", reply)
		}
	}
}

func TestReplayCreatesNewCausalFactAndKeepsPromptIdentity(t *testing.T) {
	recorder := &promptFactRecorder{}
	c := New(Options{Sink: recorder})
	done := make(chan error, 1)
	go func() {
		_, err := c.Ask(context.Background(), []event.AskQuestion{{ID: "q1", Prompt: "pick"}})
		done <- err
	}()
	facts := waitPromptFacts(t, recorder, 2)
	c.ReplayPendingPrompts()
	facts = waitPromptFacts(t, recorder, 3)
	replay := facts[2]
	if replay.Kind != event.PromptReplayed || replay.EventID == facts[1].EventID || replay.CausationID != facts[1].EventID || replay.Scope.PromptID != facts[1].Scope.PromptID || replay.Aggregate.Sequence != 3 {
		t.Fatalf("replay fact = %+v, emitted = %+v", replay, facts[1])
	}
	c.AnswerQuestion(replay.Scope.PromptID, []event.AskAnswer{{QuestionID: "q1", Selected: []string{"A"}}})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReplyFailureKeepsPromptRetryableWithOccurrence(t *testing.T) {
	recorder := &promptFactRecorder{}
	c := New(Options{Sink: recorder})
	done := make(chan error, 1)
	go func() {
		_, _, err := gateApprover{c}.Approve(context.Background(), "bash", "go test", nil)
		done <- err
	}()
	facts := waitPromptFacts(t, recorder, 2)
	id := facts[0].Scope.PromptID
	receipt := c.RecordPromptReplyFailure(id)
	if receipt.Status != event.PromptReplyFailedStatus || !receipt.Retryable || receipt.OccurrenceID == "" || !c.PendingPrompt() {
		t.Fatalf("failure receipt/pending = %+v / %v", receipt, c.PendingPrompt())
	}
	facts = waitPromptFacts(t, recorder, 3)
	if facts[2].Kind != event.PromptReplyFailed || facts[2].OccurrenceID != receipt.OccurrenceID {
		t.Fatalf("reply failed fact = %+v", facts[2])
	}
	if got := c.ApproveWithReceipt(id, false, false, false); got.Status != event.PromptReplyAccepted {
		t.Fatalf("retry receipt = %+v", got)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type blockingReplyRequestedRecorder struct {
	promptFactRecorder
	blocked chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingReplyRequestedRecorder) Emit(e event.Event) {
	r.promptFactRecorder.Emit(e)
	if e.Operational != nil && e.Operational.Kind == event.PromptReplyRequested {
		r.once.Do(func() {
			close(r.blocked)
			<-r.release
		})
	}
}

func TestCancelVersusInflightReplyHasOneTerminalAndNoResurrection(t *testing.T) {
	recorder := &blockingReplyRequestedRecorder{blocked: make(chan struct{}), release: make(chan struct{})}
	c := New(Options{Sink: recorder})
	turnCtx, cancelTurn := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancel = cancelTurn
	c.running = true
	c.mu.Unlock()

	gateDone := make(chan error, 1)
	go func() {
		_, _, err := gateApprover{c}.Approve(turnCtx, "bash", "go test", nil)
		gateDone <- err
	}()
	facts := waitPromptFacts(t, &recorder.promptFactRecorder, 2)
	id := facts[0].Scope.PromptID
	receiptDone := make(chan event.PromptReplyReceipt, 1)
	go func() { receiptDone <- c.ApproveWithReceipt(id, true, false, false) }()
	select {
	case <-recorder.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("reply_requested did not reach blocked sink")
	}
	if uncertain := c.RecordPromptReplyFailure(id); uncertain.Status != event.PromptReplyAccepted {
		t.Fatalf("in-flight transport uncertainty must not reopen reply: %+v", uncertain)
	}
	if got := len(recorder.snapshot()); got != 3 {
		t.Fatalf("in-flight failure emitted a second lifecycle fact: %d", got)
	}

	c.Cancel()
	close(recorder.release)
	if receipt := <-receiptDone; receipt.Status != event.PromptReplyCancelled {
		t.Fatalf("in-flight receipt = %+v", receipt)
	}
	if err := <-gateDone; err == nil {
		t.Fatal("gate returned nil after cancellation won")
	}
	facts = waitPromptFacts(t, &recorder.promptFactRecorder, 4)
	terminal := facts[3]
	if terminal.Kind != event.PromptCancelled || c.OwnsPrompt(id) {
		t.Fatalf("terminal/prompt ownership = %+v / %v", terminal, c.OwnsPrompt(id))
	}
	if failure := c.RecordPromptReplyFailure(id); failure.Status != event.PromptReplyCancelled {
		t.Fatalf("post-terminal failure receipt = %+v", failure)
	}
	if duplicate := c.ApproveWithReceipt(id, true, false, false); duplicate.Status != event.PromptReplyCancelled {
		t.Fatalf("duplicate receipt = %+v", duplicate)
	}
	facts = waitPromptFacts(t, &recorder.promptFactRecorder, 5)
	if replay := facts[4]; replay.Kind != event.PromptCancelled || replay.EventID != terminal.EventID || replay.Aggregate.Sequence != terminal.Aggregate.Sequence {
		t.Fatalf("terminal resync = %+v, want original %+v", replay, terminal)
	}
	uniqueTerminalIDs := map[string]struct{}{}
	for _, fact := range facts {
		if fact.Kind == event.PromptResolved || fact.Kind == event.PromptCancelled {
			uniqueTerminalIDs[fact.EventID] = struct{}{}
		}
	}
	if len(uniqueTerminalIDs) != 1 {
		t.Fatalf("unique terminal identities = %v", uniqueTerminalIDs)
	}
}
