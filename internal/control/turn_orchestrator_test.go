package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/agentmode"
	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func TestTurnOrchestratorRunsForegroundUnit(t *testing.T) {
	runner := &fakeTurnRunner{}
	c := New(Options{Runner: runner})
	c.SetPlanMode(true)

	o := newTurnOrchestrator(c)
	if err := o.runTurnWithRawDisplay(context.Background(), "draft the plan", "draft the plan", ""); err != nil {
		t.Fatal(err)
	}

	if len(runner.inputs) != 1 {
		t.Fatalf("runner inputs = %d, want 1", len(runner.inputs))
	}
	if !strings.HasPrefix(runner.inputs[0], PlanModeMarker) {
		t.Fatalf("orchestrator should compose plan marker before running, got %q", runner.inputs[0])
	}
}

type acceptedTurnObservingClassifier struct {
	t        *testing.T
	accepted *bool
	called   bool
}

func (c *acceptedTurnObservingClassifier) NeedsPlan(context.Context, string, int) (bool, string, error) {
	c.called = true
	if !*c.accepted {
		c.t.Error("auto-plan classifier ran before accepted-user-turn observer")
	}
	return false, "", nil
}

func TestTurnOrchestratorNotifiesAcceptedUserTurnBeforeAutoPlan(t *testing.T) {
	runner := &fakeTurnRunner{}
	accepted := false
	classifier := &acceptedTurnObservingClassifier{t: t, accepted: &accepted}
	c := New(Options{
		Runner:     runner,
		AutoPlan:   autoPlanOn,
		Classifier: classifier,
	})
	var gotRaw, gotDisplay string
	c.SetAcceptedUserTurnHandler(func(raw, display, _ string, _ bool) {
		accepted = true
		gotRaw = raw
		gotDisplay = display
	})

	const (
		raw     = "refactor the parser"
		display = "重构解析器"
	)
	if err := newTurnOrchestrator(c).runTurnWithRawDisplay(context.Background(), "expanded model input", raw, display); err != nil {
		t.Fatal(err)
	}

	if !classifier.called {
		t.Fatal("auto-plan classifier was not exercised")
	}
	if gotRaw != raw || gotDisplay != display {
		t.Fatalf("accepted turn = (%q, %q), want (%q, %q)", gotRaw, gotDisplay, raw, display)
	}
}

func TestCapabilityTurnNotifiesAcceptedUserTurnWithTitleText(t *testing.T) {
	c := New(Options{Runner: &fakeTurnRunner{}})
	var gotTitle string
	var gotPreferred bool
	c.SetAcceptedUserTurnHandler(func(_, _, titleText string, preferTitleText bool) {
		gotTitle = titleText
		gotPreferred = preferTitleText
	})

	if err := c.RunCapabilityTurn(context.Background(), CapabilityTurn{
		Input:           "11",
		Display:         "/document 11",
		TitleText:       "11",
		PreferTitleText: true,
	}); err != nil {
		t.Fatal(err)
	}
	if gotTitle != "11" || !gotPreferred {
		t.Fatalf("accepted title = %q preferred=%v, want %q preferred=true", gotTitle, gotPreferred, "11")
	}
}

func TestSubmitDoesNotNotifyAcceptedTurnForManagementCommandOrBusySubmit(t *testing.T) {
	session := agent.NewSession("sys")
	release := make(chan struct{})
	events := make(chan event.Event, 4)
	c := New(Options{
		Runner: blockingRunner{session: session, release: release},
		Sink: event.FuncSink(func(e event.Event) {
			events <- e
		}),
	})
	accepted := make(chan string, 2)
	c.SetAcceptedUserTurnHandler(func(raw, _, _ string, _ bool) {
		accepted <- raw
	})

	c.SubmitDisplay("first display", "first turn")
	select {
	case got := <-accepted:
		if got != "first turn" {
			t.Fatalf("accepted raw = %q, want first turn", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first accepted turn")
	}

	c.SubmitDisplay("busy display", "busy turn")
	c.SubmitDisplay("/time", "/time")
	select {
	case got := <-accepted:
		t.Fatalf("unexpected accepted turn while busy or handling management command: %q", got)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	waitForTurnDone(t, events)
}

func TestRunTurnUsesInjectedRunnerForTaskTreeModeMetadata(t *testing.T) {
	runner := &fakeTurnRunner{}
	c := New(Options{Runner: runner, AgentMode: agentmode.ModeTaskTree})

	if err := c.RunTurn(context.Background(), "plain turn"); err != nil {
		t.Fatal(err)
	}
	if got := c.AgentMode(); got != agentmode.ModeTaskTree {
		t.Fatalf("AgentMode() = %q, want task_tree", got)
	}
	if len(runner.inputs) != 1 || runner.inputs[0] != "plain turn" {
		t.Fatalf("RunTurn inputs = %v, want direct runner input", runner.inputs)
	}
}

type recordingSessionRunner struct {
	session *agent.Session
	inputs  []string
}

func (r *recordingSessionRunner) Run(ctx context.Context, input string) error {
	r.inputs = append(r.inputs, input)
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	return nil
}

type interceptNoticeErrorRunner struct {
	err error
}

func (r interceptNoticeErrorRunner) Run(context.Context, string) error {
	return r.err
}

func TestTurnOrchestratorGoalInterceptNoticeDistinguishesHardAndSoft(t *testing.T) {
	for _, tc := range []struct {
		name        string
		intercept   string
		want        string
		wantMissing string
	}{
		{
			name:        "hard requirement cannot be overridden",
			intercept:   evidence.HardReadiness("missing verified .pdf artifact"),
			want:        "hard requirement cannot be overridden",
			wantMissing: "override with a second [goal:complete]",
		},
		{
			name:      "soft intercept retains second completion override",
			intercept: "incomplete todos remain",
			want:      "override with a second [goal:complete]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess := agent.NewSession("")
			sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "still working\n\n[goal:continue]"})
			exec := agent.New(&scriptedTurns{}, tool.NewRegistry(), sess, agent.Options{}, event.Discard)
			stopErr := errors.New("stop after intercept notice")
			var notices []string
			c := New(Options{
				Runner:   interceptNoticeErrorRunner{err: stopErr},
				Executor: exec,
				Sink: event.FuncSink(func(e event.Event) {
					if e.Kind == event.Notice {
						notices = append(notices, e.Text)
					}
				}),
			})
			c.SetGoal("ship report.pdf")
			c.goals.mu.Lock()
			c.goals.interceptMsg = tc.intercept
			c.goals.mu.Unlock()

			err := newTurnOrchestrator(c).continueGoal(context.Background())
			if !errors.Is(err, stopErr) {
				t.Fatalf("continueGoal() error = %v, want %v", err, stopErr)
			}
			joined := strings.Join(notices, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("notices = %q, want %q", joined, tc.want)
			}
			if tc.wantMissing != "" && strings.Contains(joined, tc.wantMissing) {
				t.Fatalf("notices = %q, must not contain %q", joined, tc.wantMissing)
			}
		})
	}
}

func TestTurnOrchestratorRefTurnRecordsVisibleDisplay(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("referenced evidence"), 0o644); err != nil {
		t.Fatal(err)
	}
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	runner := &recordingSessionRunner{session: sess}
	events := make(chan event.Event, 4)
	c := New(Options{
		WorkspaceRoot: root,
		Runner:        runner,
		Executor:      exec,
		Sink: event.FuncSink(func(e event.Event) {
			events <- e
		}),
	})
	var gotContent, gotDisplay string
	c.SetDisplayRecorder(func(content, display string) {
		gotContent = content
		gotDisplay = display
	})

	const visible = "explain @notes.txt"
	c.runRefTurn(visible, visible)
	waitForTurnDone(t, events)

	if len(runner.inputs) != 1 {
		t.Fatalf("runner inputs = %d, want 1", len(runner.inputs))
	}
	if !strings.Contains(runner.inputs[0], "Referenced context:") || !strings.Contains(runner.inputs[0], "referenced evidence") {
		t.Fatalf("model input should include resolved reference context, got %q", runner.inputs[0])
	}
	if gotDisplay != visible {
		t.Fatalf("display recorder display = %q, want visible prompt %q", gotDisplay, visible)
	}
	if gotContent != runner.inputs[0] {
		t.Fatalf("display recorder content = %q, want persisted model input %q", gotContent, runner.inputs[0])
	}
}

func TestTurnOrchestratorCheckpointBoundaryPrecedesUserMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	sess := agent.NewSession("sys")
	exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	runner := &recordingSessionRunner{session: sess}
	c := New(Options{
		Runner:      runner,
		Executor:    exec,
		SessionDir:  dir,
		SessionPath: path,
		Label:       "test",
	})

	o := newTurnOrchestrator(c)
	if err := o.runTurnWithRawDisplay(context.Background(), "write the test", "write the test", ""); err != nil {
		t.Fatal(err)
	}

	if !c.CheckpointHasBoundary(0) {
		t.Fatal("checkpoint boundary should be available for the orchestrated turn")
	}
	if len(sess.Messages) != 2 || sess.Messages[1].Content != "write the test" {
		t.Fatalf("session messages after turn = %+v, want system + user", sess.Messages)
	}
	loaded, err := agent.LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("saved messages = %d, want system + user", len(loaded.Messages))
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load branch meta ok=%v err=%v", ok, err)
	}
	if meta.UpdatedAt.IsZero() {
		t.Fatal("activity meta should be marked after transcript changes")
	}
	if err := c.Rewind(0, RewindConversation); err != nil {
		t.Fatal(err)
	}
	if len(sess.Messages) != 1 {
		t.Fatalf("session messages after rewind = %d, want boundary before user message", len(sess.Messages))
	}
}

// TestTurnOrchestratorCancelPreservesInterruptedTurn verifies that a user stop
// preserves everything already shown while appending a durable continuation
// boundary. A stop must never behave like rewind.
func TestTurnOrchestratorCancelPreservesInterruptedTurn(t *testing.T) {
	sess := agent.NewSession("you are a helpful agent")
	// Pre-populate with a few messages from an earlier turn.
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "previous work"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "done"})
	preCount := len(sess.Messages)

	// runner that simulates a cancelled turn: it adds the user message plus
	// partial tool-call messages the real agent would leave behind, then returns
	// context.Canceled.
	runner := &interruptedTurnRunner{
		session: sess,
		add: []provider.Message{
			{Role: provider.RoleAssistant, Content: "let me do that", ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "todo_write", Arguments: `{"todos":[{"content":"add abc","status":"in_progress"}]}`},
			}},
			{Role: provider.RoleTool, Content: "Todos updated: 1 total — 0 completed, 1 in_progress, 0 pending.", ToolCallID: "c1", Name: "todo_write"},
		},
		err: context.Canceled,
	}

	ex := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
	c := New(Options{Runner: runner, Executor: ex})
	// Simulate a user-initiated cancel: set the cancelling flag.
	c.mu.Lock()
	c.canceling = true
	c.mu.Unlock()

	// Pre-seed todoState as if a successful todo_write from the cancelled turn
	// had already updated it — this is the state the runner leaves behind before
	// returning context.Canceled, and what RebuildTodoState must clear.
	ex.ReplaceTodoState([]evidence.TodoItem{{Content: "add abc", Status: "in_progress"}})

	o := newTurnOrchestrator(c)
	err := o.runTurnWithRawDisplay(context.Background(), "add config file abc", "add config file abc", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// The interrupted user prompt, assistant and tool result remain durable,
	// followed by one host-authored interruption boundary.
	msgs := sess.Messages
	if len(msgs) != preCount+4 {
		t.Fatalf("session messages after cancel = %d, want %d: %+v", len(msgs), preCount+4, msgs)
	}
	if got := msgs[preCount].Content; got != "add config file abc" {
		t.Fatalf("interrupted user prompt = %q", got)
	}
	if !agent.IsTurnInterruptedMessage(msgs[len(msgs)-1].Content) {
		t.Fatalf("last message is not interruption boundary: %+v", msgs[len(msgs)-1])
	}

	// todoState must also be reset: the in_progress todo written by the
	// cancelled turn must not survive the interruption boundary.
	if todos := c.Todos(); len(todos) != 0 {
		t.Fatalf("Todos() after cancel = %v, want empty — cancelled todo_write leaked into canonical state", todos)
	}
}

func TestTurnOrchestratorCancelWithoutNewMessagesDoesNotMarkPriorTurn(t *testing.T) {
	sess := agent.NewSession("system")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "completed prompt"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "completed answer"})
	preCount := len(sess.Messages)
	c := New(Options{
		Runner:   noMutationCanceledRunner{},
		Executor: agent.New(nil, nil, sess, agent.Options{}, event.Discard),
	})
	c.mu.Lock()
	c.canceling = true
	c.mu.Unlock()

	o := newTurnOrchestrator(c)
	if err := o.runTurnWithRawDisplay(context.Background(), "not yet persisted", "not yet persisted", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(sess.Messages) != preCount {
		t.Fatalf("session messages after pre-mutation cancel = %d, want %d: %+v", len(sess.Messages), preCount, sess.Messages)
	}
}

// TestTurnOrchestratorCancelFlushesInterruptedTranscriptToDisk verifies that a
// restart restores the interrupted turn instead of a pre-turn truncation.
func TestTurnOrchestratorCancelFlushesInterruptedTranscriptToDisk(t *testing.T) {
	sess := agent.NewSession("system")
	sess.Add(provider.Message{Role: provider.RoleUser, Content: "earlier turn"})
	sess.Add(provider.Message{Role: provider.RoleAssistant, Content: "done"})
	runner := &interruptedTurnRunner{
		session: sess,
		add: []provider.Message{
			{Role: provider.RoleAssistant, Content: "working…", ToolCalls: []provider.ToolCall{
				{ID: "d1", Name: "todo_write", Arguments: `{"todos":[{"content":"task","status":"in_progress"}]}`},
			}},
			{Role: provider.RoleTool, Content: "Todos updated.", ToolCallID: "d1", Name: "todo_write"},
		},
		err: context.Canceled,
	}

	sessionPath := agent.NewSessionPath(t.TempDir(), "test-model")
	c := New(Options{
		Runner:      runner,
		Executor:    agent.New(nil, nil, sess, agent.Options{}, event.Discard),
		SessionPath: sessionPath,
	})
	c.mu.Lock()
	c.canceling = true
	c.mu.Unlock()

	o := newTurnOrchestrator(c)
	if err := o.runTurnWithRawDisplay(context.Background(), "do something", "do something", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Load the session file and verify the interrupted work and boundary survive.
	loaded, err := agent.LoadSession(sessionPath)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(loaded.Messages) != 7 {
		t.Fatalf("on-disk messages = %d, want 7: %+v", len(loaded.Messages), loaded.Messages)
	}
	if got := loaded.Messages[3].Content; got != "do something" {
		t.Fatalf("on-disk interrupted prompt = %q", got)
	}
	if !agent.IsTurnInterruptedMessage(loaded.Messages[len(loaded.Messages)-1].Content) {
		t.Fatalf("on-disk interruption boundary missing: %+v", loaded.Messages)
	}
}

// interruptedTurnRunner adds messages to a session then returns a fixed error,
// simulating an agent that was interrupted mid-turn.
type interruptedTurnRunner struct {
	session *agent.Session
	add     []provider.Message
	err     error
}

type noMutationCanceledRunner struct{}

func (noMutationCanceledRunner) Run(context.Context, string) error { return context.Canceled }

func (r *interruptedTurnRunner) Run(ctx context.Context, input string) error {
	r.session.Add(provider.Message{Role: provider.RoleUser, Content: input})
	for _, m := range r.add {
		r.session.Add(m)
	}
	return r.err
}
