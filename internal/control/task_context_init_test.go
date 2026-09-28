package control

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

type recordingTaskContextInitializer struct {
	mu       sync.Mutex
	tasks    []modelbilling.TaskContext
	err      error
	requests []TaskContextInitRequest
}

type blockingTaskContextInitializer struct {
	mu          sync.Mutex
	task        modelbilling.TaskContext
	started     chan struct{}
	release     <-chan struct{}
	calls       int
	startedOnce sync.Once
}

type panicOnceTaskContextInitializer struct {
	mu        sync.Mutex
	task      modelbilling.TaskContext
	panicNext bool
	calls     int
}

type controlTaskLifecycleStore struct {
	mu        sync.Mutex
	updates   []modelbilling.TaskStateUpdate
	updateErr error
}

type compactTaskContextProvider struct {
	mu       sync.Mutex
	calls    int
	contexts []modelbilling.TaskContext
}

type recordingSubscriptionCallStore struct {
	mu       sync.Mutex
	contexts []modelbilling.TaskContext
}

type subscriptionSummaryGateway struct{}

type subscriptionSummaryEvidenceResolver struct{}

type subscriptionSummaryIDGenerator struct{}

func (*compactTaskContextProvider) Name() string { return "compact-test" }

func (p *compactTaskContextProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	p.calls++
	if task, ok := modelbilling.TaskContextFrom(ctx); ok {
		p.contexts = append(p.contexts, task)
	}
	p.mu.Unlock()
	chunks := make(chan provider.Chunk, 2)
	chunks <- provider.Chunk{Type: provider.ChunkText, Text: "compact summary"}
	chunks <- provider.Chunk{Type: provider.ChunkDone}
	close(chunks)
	return chunks, nil
}

func (p *compactTaskContextProvider) snapshot() (int, []modelbilling.TaskContext) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls, append([]modelbilling.TaskContext(nil), p.contexts...)
}

func (s *recordingSubscriptionCallStore) Prepare(_ context.Context, task modelbilling.TaskContext, _ modelbilling.ModelCall) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contexts = append(s.contexts, task)
	return nil
}

func (*recordingSubscriptionCallStore) BeginAttempt(context.Context, modelbilling.TaskContext, string, time.Time) error {
	return nil
}

func (*recordingSubscriptionCallStore) Complete(context.Context, modelbilling.TaskContext, modelbilling.ModelCall) error {
	return nil
}

func (*recordingSubscriptionCallStore) ListByTask(context.Context, modelbilling.TaskScope, string) ([]modelbilling.ModelCall, error) {
	return nil, nil
}

func (s *recordingSubscriptionCallStore) snapshot() []modelbilling.TaskContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]modelbilling.TaskContext(nil), s.contexts...)
}

func (subscriptionSummaryGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (subscriptionSummaryGateway) Stream(context.Context, cloud.BindingScope, modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	chunks := make(chan modelbilling.GatewayChunk, 2)
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "subscription summary"}}
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkDone}}
	close(chunks)
	return chunks, nil
}

func (subscriptionSummaryEvidenceResolver) ResolveForCall(_ context.Context, task modelbilling.TaskContext, modelID string) (modelbilling.PricingEvidence, error) {
	return modelbilling.PricingEvidence{
		EvidenceID: task.PricingEvidenceID, SnapshotID: "subscription_summary_snapshot",
		Scope: task.Scope, TaskID: task.TaskID, ModelID: modelID,
		PricingGroup: "default", State: modelbilling.PricingFresh, CapturedAt: time.Now().UTC(),
	}, nil
}

func (subscriptionSummaryIDGenerator) NewID() (string, error) {
	return "subscription_summary_call", nil
}

func (*controlTaskLifecycleStore) CreateTaskWithEvidence(context.Context, modelbilling.ModelTask, modelbilling.PricingEvidence) error {
	return nil
}

func (*controlTaskLifecycleStore) RecordFreezeRequest(context.Context, modelbilling.TaskContext, *cloud.FreezeRequest, time.Time) error {
	return nil
}

func (*controlTaskLifecycleStore) RecordFreezeResult(context.Context, modelbilling.TaskContext, *cloud.Freeze, *modelbilling.TaskEntitlementSnapshot, time.Time) error {
	return nil
}

func (s *controlTaskLifecycleStore) UpdateState(_ context.Context, update modelbilling.TaskStateUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, update)
	return s.updateErr
}

func (*controlTaskLifecycleStore) Get(context.Context, modelbilling.TaskScope, string) (modelbilling.ModelTask, error) {
	return modelbilling.ModelTask{}, nil
}

func (*controlTaskLifecycleStore) ListRecoverable(context.Context, modelbilling.TaskScope) ([]modelbilling.ModelTask, error) {
	return nil, nil
}

func (s *controlTaskLifecycleStore) snapshot() []modelbilling.TaskStateUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]modelbilling.TaskStateUpdate(nil), s.updates...)
}

func (f *panicOnceTaskContextInitializer) InitializeTaskContext(context.Context, TaskContextInitRequest) (modelbilling.TaskContext, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.panicNext {
		f.panicNext = false
		panic("initializer panic")
	}
	return f.task, nil
}

func (f *panicOnceTaskContextInitializer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *blockingTaskContextInitializer) InitializeTaskContext(ctx context.Context, _ TaskContextInitRequest) (modelbilling.TaskContext, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	f.startedOnce.Do(func() { close(f.started) })
	select {
	case <-f.release:
		return f.task, nil
	case <-ctx.Done():
		return modelbilling.TaskContext{}, ctx.Err()
	}
}

func (f *blockingTaskContextInitializer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *recordingTaskContextInitializer) InitializeTaskContext(_ context.Context, request TaskContextInitRequest) (modelbilling.TaskContext, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, request)
	if f.err != nil {
		return modelbilling.TaskContext{}, f.err
	}
	if len(f.tasks) == 0 {
		return modelbilling.TaskContext{}, nil
	}
	task := f.tasks[0]
	f.tasks = f.tasks[1:]
	return task, nil
}

func (f *recordingTaskContextInitializer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

type taskContextRunner struct {
	contexts []modelbilling.TaskContext
}

func (r *taskContextRunner) Run(ctx context.Context, _ string) error {
	task, ok := modelbilling.TaskContextFrom(ctx)
	if !ok {
		return newErrorForTest("runner missing task context")
	}
	r.contexts = append(r.contexts, task)
	return nil
}

type panickingTaskContextRunner struct {
	contexts []modelbilling.TaskContext
}

func (r *panickingTaskContextRunner) Run(ctx context.Context, _ string) error {
	task, ok := modelbilling.TaskContextFrom(ctx)
	if !ok {
		return newErrorForTest("runner missing task context")
	}
	r.contexts = append(r.contexts, task)
	panic("runner panic")
}

type taskContextClassifier struct {
	contexts []modelbilling.TaskContext
}

type taskContextScriptRunner struct {
	session  *agent.Session
	contexts []modelbilling.TaskContext
	outputs  []string
	charges  []provider.ToolChargeMetadata
}

func (r *taskContextScriptRunner) Run(ctx context.Context, _ string) error {
	task, ok := modelbilling.TaskContextFrom(ctx)
	if !ok {
		return newErrorForTest("script runner missing task context")
	}
	r.contexts = append(r.contexts, task)
	for _, charge := range r.charges {
		if err := tool.RecordCredit(ctx, charge); err != nil {
			return err
		}
	}
	if len(r.outputs) > 0 {
		r.session.Add(provider.Message{Role: provider.RoleAssistant, Content: r.outputs[0]})
		r.outputs = r.outputs[1:]
	}
	return nil
}

func (c *taskContextClassifier) NeedsPlan(ctx context.Context, _ string, _ int) (bool, string, error) {
	task, ok := modelbilling.TaskContextFrom(ctx)
	if !ok {
		return false, "", newErrorForTest("classifier missing task context")
	}
	c.contexts = append(c.contexts, task)
	return false, "", nil
}

type testTaskContextError string

func (e testTaskContextError) Error() string { return string(e) }

func newErrorForTest(message string) error { return testTaskContextError(message) }

func testActor() userctx.ActorContext {
	return userctx.ActorContext{ActorID: "actor_01", ProfileID: "profile_01"}.Normalize()
}

func testTaskContext(t *testing.T, taskID string, actor userctx.ActorContext) modelbilling.TaskContext {
	t.Helper()
	task, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: taskID,
		Scope: modelbilling.TaskScope{
			ActorID: actor.ActorID, ProfileID: actor.ProfileID,
		},
		BillingSource:     modelbilling.BillingNewAPIByok,
		InitialModelID:    "model_01",
		PricingEvidenceID: "evidence_" + taskID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func testSubscriptionTaskContext(t *testing.T, taskID string, actor userctx.ActorContext) modelbilling.TaskContext {
	t.Helper()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud_user_01", BindingGeneration: 1}
	task, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: taskID,
		Scope: modelbilling.TaskScope{
			ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding,
		},
		BillingSource:     modelbilling.BillingSubscriptionPool,
		InitialModelID:    "subscription_model_01",
		PricingEvidenceID: "evidence_" + taskID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestTaskContextInitializerBindsBeforeClassifierAndRunner(t *testing.T) {
	actor := testActor()
	rawActor := userctx.ActorContext{ActorID: " actor_01 ", ProfileID: " profile_01 "}
	task := testTaskContext(t, "task_classifier", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	runner := &taskContextRunner{}
	classifier := &taskContextClassifier{}
	c := New(Options{Actor: rawActor, Runner: runner, Classifier: classifier, AutoPlan: "on", TaskContextInitializer: initializer})

	if err := c.RunTurn(context.Background(), "implement a small change"); err != nil {
		t.Fatal(err)
	}
	if initializer.callCount() != 1 || len(classifier.contexts) != 1 || len(runner.contexts) != 1 {
		t.Fatalf("initializer/classifier/runner = %d/%d/%d", initializer.callCount(), len(classifier.contexts), len(runner.contexts))
	}
	if classifier.contexts[0] != task || runner.contexts[0] != task {
		t.Fatal("classifier and runner did not receive the initialized task context")
	}
	if initializer.requests[0].Actor.ActorID != actor.ActorID || initializer.requests[0].Actor.ProfileID != actor.ProfileID {
		t.Fatalf("initializer request = %+v", initializer.requests[0])
	}
}

func TestRunTurnPersistsAndEmitsFinalAssistantMetadata(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_turn_metadata", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	session := agent.NewSession("")
	runner := &taskContextScriptRunner{
		session: session,
		outputs: []string{"final answer"},
		charges: []provider.ToolChargeMetadata{
			{Tool: "generate_image", Kind: provider.ToolChargeMaaSQuota, Value: "35800"},
			{Tool: "generate_video", Kind: provider.ToolChargeCredits, Value: "1074.0000"},
		},
	}
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	selection := modelbilling.BillingRouteSelection{
		ModelRef: "provider/glm-5.2", BillingSource: task.BillingSource,
	}
	var turnDone event.Event
	c := New(Options{
		Actor: actor, Runner: runner, Executor: executor,
		TaskContextInitializer: initializer,
		TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
		BillingRouteSelection:  selection,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				turnDone = e
			}
		}),
	})

	if err := c.RunTurn(context.Background(), "answer this"); err != nil {
		t.Fatal(err)
	}
	want := provider.TurnMetadata{
		TaskID: task.TaskID, ModelRef: selection.ModelRef, ModelID: task.InitialModelID, BillingSource: string(task.BillingSource),
		ToolMaaSQuota: "35800", ToolChargedCredits: "1074",
	}
	if turnDone.Turn == nil || *turnDone.Turn != want {
		t.Fatalf("TurnDone metadata = %#v, want %#v", turnDone.Turn, want)
	}
	messages := session.Snapshot()
	if len(messages) != 1 || messages[0].Role != provider.RoleAssistant {
		t.Fatalf("session messages = %#v", messages)
	}
	if messages[0].Turn == nil || *messages[0].Turn != want {
		t.Fatalf("persisted assistant metadata = %#v, want %#v", messages[0].Turn, want)
	}
}

func TestRunTurnEmitsMetadataWithoutAssistantMessage(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_turn_without_message", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	session := agent.NewSession("")
	runner := &taskContextScriptRunner{session: session}
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	selection := modelbilling.BillingRouteSelection{
		ModelRef: "provider/glm-5.2", BillingSource: task.BillingSource,
	}
	var turnDone event.Event
	c := New(Options{
		Actor: actor, Runner: runner, Executor: executor,
		TaskContextInitializer: initializer,
		TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
		BillingRouteSelection:  selection,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				turnDone = e
			}
		}),
	})

	if err := c.RunTurn(context.Background(), "answer this"); err != nil {
		t.Fatal(err)
	}
	want := provider.TurnMetadata{
		TaskID: task.TaskID, ModelRef: selection.ModelRef, ModelID: task.InitialModelID, BillingSource: string(task.BillingSource),
	}
	if turnDone.Turn == nil || *turnDone.Turn != want {
		t.Fatalf("TurnDone metadata = %#v, want %#v", turnDone.Turn, want)
	}
	if messages := session.Snapshot(); len(messages) != 0 {
		t.Fatalf("session messages = %#v, want none", messages)
	}
}

func TestTaskContextInitializerReceivesExplicitBillingRouteSelection(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_route", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	runner := &taskContextRunner{}
	selection := modelbilling.BillingRouteSelection{ModelRef: "provider/model", BillingSource: modelbilling.BillingNewAPIByok}
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer, BillingRouteSelection: selection})
	if err := c.RunTurn(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if len(initializer.requests) != 1 || initializer.requests[0].BillingRouteSelection != selection {
		t.Fatalf("initializer requests = %+v, want route %+v", initializer.requests, selection)
	}
}

func TestTaskContextInitializerReusesExistingContext(t *testing.T) {
	actor := testActor()
	existing := testTaskContext(t, "task_existing", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{testTaskContext(t, "task_unused", actor)}}
	runner := &taskContextRunner{}
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer})
	ctx, err := modelbilling.WithTaskContext(context.Background(), existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Run(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if initializer.callCount() != 0 || len(runner.contexts) != 1 || runner.contexts[0] != existing {
		t.Fatal("existing task context was replaced or initializer ran")
	}
}

func TestTaskContextInitializerRejectsInvalidOrMismatchedResultBeforeModelEntry(t *testing.T) {
	actor := testActor()
	other := userctx.ActorContext{ActorID: "other", ProfileID: "profile_02"}.Normalize()
	for _, tc := range []struct {
		name string
		task modelbilling.TaskContext
		code modelbilling.ErrorCode
	}{
		{"invalid", modelbilling.TaskContext{}, modelbilling.ErrorContractInvalid},
		{"scope", testTaskContext(t, "task_other", other), modelbilling.ErrorScopeMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{tc.task}}
			runner := &taskContextRunner{}
			classifier := &taskContextClassifier{}
			c := New(Options{Actor: actor, Runner: runner, Classifier: classifier, AutoPlan: "on", TaskContextInitializer: initializer})
			err := c.RunTurn(context.Background(), "implement a small change")
			if modelbilling.ErrorCodeOf(err) != tc.code || len(runner.contexts) != 0 || len(classifier.contexts) != 0 {
				t.Fatalf("err=%v runner=%d classifier=%d", err, len(runner.contexts), len(classifier.contexts))
			}
		})
	}
}

func TestTaskContextInitializerReturnsItsOwnErrorBeforeModelEntry(t *testing.T) {
	actor := testActor()
	want := errors.New("initializer unavailable")
	initializer := &recordingTaskContextInitializer{err: want}
	runner := &taskContextRunner{}
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer})
	if err := c.Run(context.Background(), "hello"); !errors.Is(err, want) || len(runner.contexts) != 0 {
		t.Fatalf("initializer error = %v runner=%d", err, len(runner.contexts))
	}
}

func TestTaskContextRequiredFailsClosedAtAllTopLevelEntrances(t *testing.T) {
	actor := testActor()
	for _, tc := range []struct {
		name string
		run  func(*Controller) error
	}{
		{"run_turn", func(c *Controller) error { return c.RunTurn(context.Background(), "hello") }},
		{"headless", func(c *Controller) error { return c.Run(context.Background(), "hello") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &taskContextRunner{}
			c := New(Options{Actor: actor, Runner: runner, RequireTaskContext: true})
			if modelbilling.ErrorCodeOf(tc.run(c)) != modelbilling.ErrorContractInvalid || len(runner.contexts) != 0 {
				t.Fatal("required task context did not fail before runner")
			}
		})
	}
	t.Run("async_send", func(t *testing.T) {
		runner := &taskContextRunner{}
		done := make(chan event.Event, 1)
		c := New(Options{Actor: actor, Runner: runner, RequireTaskContext: true, Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				done <- e
			}
		})})
		c.Send("hello")
		event := <-done
		if modelbilling.ErrorCodeOf(event.Err) != modelbilling.ErrorContractInvalid || len(runner.contexts) != 0 {
			t.Fatal("async required task context did not fail before runner")
		}
	})
	var typedNil *recordingTaskContextInitializer
	c := New(Options{Actor: actor, Runner: &taskContextRunner{}, RequireTaskContext: true, TaskContextInitializer: typedNil})
	if modelbilling.ErrorCodeOf(c.Run(context.Background(), "hello")) != modelbilling.ErrorContractInvalid {
		t.Fatal("typed-nil initializer did not fail closed")
	}
}

func TestTaskContextInitializerLegacyAndIndependentTasks(t *testing.T) {
	actor := testActor()
	legacyRunner := &fakeTurnRunner{}
	legacy := New(Options{Actor: actor, Runner: legacyRunner})
	if err := legacy.Run(context.Background(), "legacy"); err != nil || len(legacyRunner.inputs) != 1 {
		t.Fatalf("legacy behavior changed: %v inputs=%d", err, len(legacyRunner.inputs))
	}
	first := testTaskContext(t, "task_first", actor)
	second := testTaskContext(t, "task_second", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{first, second}}
	runner := &taskContextRunner{}
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer})
	if err := c.Run(context.Background(), "first"); err != nil {
		t.Fatal(err)
	}
	if err := c.Run(context.Background(), "second"); err != nil {
		t.Fatal(err)
	}
	if initializer.callCount() != 2 || len(runner.contexts) != 2 || runner.contexts[0].TaskID != first.TaskID || runner.contexts[1].TaskID != second.TaskID {
		t.Fatal("independent top-level tasks did not initialize independently")
	}
}

func TestTaskLifecycleOwnsSynchronousTopLevelEntrances(t *testing.T) {
	for name, run := range map[string]func(*Controller) error{
		"run_turn": func(c *Controller) error { return c.RunTurn(context.Background(), "hello") },
		"headless": func(c *Controller) error { return c.Run(context.Background(), "hello") },
	} {
		t.Run(name, func(t *testing.T) {
			actor := testActor()
			task := testTaskContext(t, "task_lifecycle_"+name, actor)
			store := &controlTaskLifecycleStore{}
			initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
			runner := &taskContextRunner{}
			c := New(Options{
				Actor:                  actor,
				Runner:                 runner,
				TaskContextInitializer: initializer,
				TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
			})

			if err := run(c); err != nil {
				t.Fatal(err)
			}
			updates := store.snapshot()
			if len(updates) != 2 || updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning || updates[1].From != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskCompleted {
				t.Fatalf("task lifecycle updates = %#v", updates)
			}
			if initializer.callCount() != 1 || len(runner.contexts) != 1 {
				t.Fatalf("initializer/runner calls = %d/%d", initializer.callCount(), len(runner.contexts))
			}
		})
	}
}

func TestTaskLifecycleBeginFailureStopsClassifierAndRunner(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_lifecycle_begin_failure", actor)
	want := errors.New("begin persistence failed")
	store := &controlTaskLifecycleStore{updateErr: want}
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	runner := &taskContextRunner{}
	classifier := &taskContextClassifier{}
	c := New(Options{
		Actor:                  actor,
		Runner:                 runner,
		Classifier:             classifier,
		AutoPlan:               "on",
		TaskContextInitializer: initializer,
		TaskLifecycle:          modelbilling.TaskLifecycleRunner{Tasks: store},
	})

	err := c.RunTurn(context.Background(), "implement a small change")
	if !errors.Is(err, want) {
		t.Fatalf("begin error = %v, want %v", err, want)
	}
	if len(classifier.contexts) != 0 || len(runner.contexts) != 0 {
		t.Fatalf("classifier/runner calls after begin failure = %d/%d", len(classifier.contexts), len(runner.contexts))
	}
	updates := store.snapshot()
	if len(updates) != 1 || updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning {
		t.Fatalf("begin attempts = %#v", updates)
	}
}

func TestTaskContextInitializerKeepsPlanProposalAndSyntheticExecutionInOneTask(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_plan", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	session := agent.NewSession("")
	runner := &taskContextScriptRunner{session: session, outputs: []string{"1. Inspect the task", "Execution complete"}}
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	var c *Controller
	c = New(Options{Actor: actor, Runner: runner, Executor: executor, TaskContextInitializer: initializer, TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: store}, Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.ApprovalRequest && e.Approval.Tool == planApprovalTool {
			go c.Approve(e.Approval.ID, true, false, false)
		}
	})})
	c.SetPlanMode(true)
	if err := c.RunTurn(context.Background(), "plan the change"); err != nil {
		t.Fatal(err)
	}
	if initializer.callCount() != 1 || len(runner.contexts) != 2 || runner.contexts[0].TaskID != task.TaskID || runner.contexts[1].TaskID != task.TaskID {
		t.Fatal("plan proposal and synthetic execution did not share one task context")
	}
	updates := store.snapshot()
	if len(updates) != 2 || updates[0].To != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskCompleted {
		t.Fatalf("plan task lifecycle updates = %#v", updates)
	}
}

func TestTaskContextInitializerKeepsGoalContinuationInOneTask(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_goal", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	session := agent.NewSession("")
	runner := &taskContextScriptRunner{session: session, outputs: []string{"working\n\n[goal:continue]", "done\n\n[goal:complete]"}}
	executor := agent.New(nil, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	c := New(Options{Actor: actor, Runner: runner, Executor: executor, TaskContextInitializer: initializer, TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: store}})
	c.SetGoal("finish the task")
	if err := c.runGoalLoopWithRawDisplay(context.Background(), "start", "start", ""); err != nil {
		t.Fatal(err)
	}
	if initializer.callCount() != 1 || len(runner.contexts) != 2 || runner.contexts[0].TaskID != task.TaskID || runner.contexts[1].TaskID != task.TaskID {
		t.Fatal("goal continuation did not share one task context")
	}
	updates := store.snapshot()
	if len(updates) != 2 || updates[0].To != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskCompleted {
		t.Fatalf("goal task lifecycle updates = %#v", updates)
	}
}

func TestRunTurnDisplayReservesTurnBeforeTaskContextInitialization(t *testing.T) {
	actor := testActor()
	release := make(chan struct{})
	initializer := &blockingTaskContextInitializer{
		task: testTaskContext(t, "task_reserved", actor), started: make(chan struct{}), release: release,
	}
	var releaseOnce sync.Once
	releaseTurn := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseTurn()
	runner := &taskContextRunner{}
	done := make(chan event.Event, 2)
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer, Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.TurnDone {
			done <- e
		}
	})})

	var turns sync.WaitGroup
	first := make(chan error, 1)
	turns.Add(1)
	go func() {
		defer turns.Done()
		first <- c.RunTurnDisplay(context.Background(), "", "first")
	}()
	select {
	case <-initializer.started:
	case <-time.After(time.Second):
		t.Fatal("initializer did not start")
	}

	secondCtx, secondCancel := context.WithTimeout(context.Background(), time.Second)
	defer secondCancel()
	second := make(chan error, 1)
	turns.Add(1)
	go func() {
		defer turns.Done()
		second <- c.RunTurnDisplay(secondCtx, "", "second")
	}()
	select {
	case err := <-second:
		if err != ErrTurnRunning {
			t.Fatalf("second RunTurnDisplay error = %v, want ErrTurnRunning", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second RunTurnDisplay did not reject while initializer was blocked")
	}
	if initializer.callCount() != 1 {
		t.Fatalf("initializer calls = %d, want 1", initializer.callCount())
	}

	releaseTurn()
	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first RunTurnDisplay error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first RunTurnDisplay did not finish")
	}
	waitForTaskContextTurns(t, &turns)
	if len(runner.contexts) != 1 || runner.contexts[0].TaskID != "task_reserved" {
		t.Fatalf("runner contexts = %+v", runner.contexts)
	}
	select {
	case <-done:
	default:
		t.Fatal("missing TurnDone for successful RunTurnDisplay")
	}
	select {
	case extra := <-done:
		t.Fatalf("unexpected extra TurnDone: %+v", extra)
	default:
	}
}

func TestRunTurnDisplayInitializerErrorRestoresIdleState(t *testing.T) {
	actor := testActor()
	initializer := &recordingTaskContextInitializer{err: errors.New("initializer failed")}
	runner := &taskContextRunner{}
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer})
	if err := c.RunTurnDisplay(context.Background(), "", "first"); err == nil {
		t.Fatal("expected initializer error")
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after initializer error = %+v, want idle", status)
	}
	initializer.mu.Lock()
	initializer.err = nil
	initializer.tasks = []modelbilling.TaskContext{testTaskContext(t, "task_after_error", actor)}
	initializer.mu.Unlock()
	if err := c.RunTurnDisplay(context.Background(), "", "second"); err != nil {
		t.Fatalf("second RunTurnDisplay error = %v", err)
	}
	if initializer.callCount() != 2 || len(runner.contexts) != 1 || runner.contexts[0].TaskID != "task_after_error" {
		t.Fatalf("initializer/runner after recovery = %d/%+v", initializer.callCount(), runner.contexts)
	}
}

func TestRunTurnDisplayCancelCancelsTaskContextInitialization(t *testing.T) {
	actor := testActor()
	initializer := &blockingTaskContextInitializer{
		task: testTaskContext(t, "task_cancel", actor), started: make(chan struct{}), release: make(chan struct{}),
	}
	runner := &taskContextRunner{}
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer})
	result := make(chan error, 1)
	var turn sync.WaitGroup
	turn.Add(1)
	go func() {
		defer turn.Done()
		result <- c.RunTurnDisplay(context.Background(), "", "cancel")
	}()
	select {
	case <-initializer.started:
	case <-time.After(time.Second):
		t.Fatal("initializer did not start")
	}
	c.Cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled initializer error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Cancel did not stop task context initialization")
	}
	waitForTaskContextTurns(t, &turn)
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after initialization cancellation = %+v, want idle", status)
	}
	if initializer.callCount() != 1 || len(runner.contexts) != 0 {
		t.Fatalf("initializer/runner after cancellation = %d/%d", initializer.callCount(), len(runner.contexts))
	}
}

func TestRunTurnDisplayRecoversPanickingTaskContextInitializer(t *testing.T) {
	actor := testActor()
	initializer := &panicOnceTaskContextInitializer{task: testTaskContext(t, "task_after_initializer_panic", actor), panicNext: true}
	runner := &taskContextRunner{}
	classifier := &taskContextClassifier{}
	done := make(chan event.Event, 2)
	c := New(Options{Actor: actor, Runner: runner, Classifier: classifier, AutoPlan: "on", TaskContextInitializer: initializer, Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.TurnDone {
			done <- e
		}
	})})

	err := c.RunTurnDisplay(context.Background(), "", "implement a small change")
	if err == nil || err.Error() != "internal error: initializer panic" {
		t.Fatalf("initializer panic error = %v", err)
	}
	if len(classifier.contexts) != 0 || len(runner.contexts) != 0 {
		t.Fatalf("classifier/runner ran after initializer panic = %d/%d", len(classifier.contexts), len(runner.contexts))
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after initializer panic = %+v, want idle", status)
	}
	select {
	case event := <-done:
		t.Fatalf("initializer panic emitted TurnDone: %+v", event)
	default:
	}
	if err := c.RunTurnDisplay(context.Background(), "", "implement a small change"); err != nil {
		t.Fatalf("RunTurnDisplay after initializer panic = %v", err)
	}
	if initializer.callCount() != 2 || len(classifier.contexts) != 1 || len(runner.contexts) != 1 {
		t.Fatalf("recovery calls/classifier/runner = %d/%d/%d", initializer.callCount(), len(classifier.contexts), len(runner.contexts))
	}
	select {
	case <-done:
	default:
		t.Fatal("successful recovery did not emit TurnDone")
	}
}

func TestRunTurnDisplayRunnerPanicEmitsOneTurnDone(t *testing.T) {
	actor := testActor()
	task := testTaskContext(t, "task_runner_panic", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
	store := &controlTaskLifecycleStore{}
	runner := &panickingTaskContextRunner{}
	done := make(chan event.Event, 2)
	c := New(Options{Actor: actor, Runner: runner, TaskContextInitializer: initializer, TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: store}, Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.TurnDone {
			done <- e
		}
	})})

	err := c.RunTurnDisplay(context.Background(), "", "panic")
	if err == nil || err.Error() != "internal error: runner panic" {
		t.Fatalf("runner panic error = %v", err)
	}
	if len(runner.contexts) != 1 || runner.contexts[0].TaskID != "task_runner_panic" {
		t.Fatalf("runner contexts = %+v", runner.contexts)
	}
	updates := store.snapshot()
	if len(updates) != 2 || updates[0].To != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskFailed {
		t.Fatalf("task lifecycle after runner panic = %#v", updates)
	}
	select {
	case event := <-done:
		if event.Err == nil || event.Err.Error() != "internal error: runner panic" {
			t.Fatalf("TurnDone error = %v", event.Err)
		}
	default:
		t.Fatal("runner panic did not emit TurnDone")
	}
	select {
	case event := <-done:
		t.Fatalf("runner panic emitted duplicate TurnDone: %+v", event)
	default:
	}
}

func TestStrictManualCompactRejectsActiveTurnBeforeInitializerAndModel(t *testing.T) {
	actor := testActor()
	release := make(chan struct{})
	initializer := &blockingTaskContextInitializer{
		task: testTaskContext(t, "task_active_turn", actor), started: make(chan struct{}), release: release,
	}
	provider := &compactTaskContextProvider{}
	executor := newManualCompactExecutor(provider)
	c := New(Options{
		Actor: actor, Runner: &taskContextRunner{}, Executor: executor,
		TaskContextInitializer: initializer, RequireTaskContext: true,
	})

	turnResult := make(chan error, 1)
	go func() { turnResult <- c.RunTurnDisplay(context.Background(), "", "active") }()
	select {
	case <-initializer.started:
	case <-time.After(time.Second):
		t.Fatal("active turn initializer did not start")
	}

	compactCtx, cancelCompact := context.WithTimeout(context.Background(), time.Second)
	defer cancelCompact()
	if err := c.Compact(compactCtx, ""); err != ErrTurnRunning {
		t.Fatalf("Compact during active turn error = %v, want ErrTurnRunning", err)
	}
	if initializer.callCount() != 1 {
		t.Fatalf("initializer calls = %d, want active turn only", initializer.callCount())
	}
	if calls, _ := provider.snapshot(); calls != 0 {
		t.Fatalf("compact provider calls = %d, want 0", calls)
	}

	close(release)
	select {
	case err := <-turnResult:
		if err != nil {
			t.Fatalf("active turn error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("active turn did not finish")
	}
}

func TestStrictManualCompactCreatesIndependentTaskLifecycle(t *testing.T) {
	actor := testActor()
	turnTask := testTaskContext(t, "task_user_turn", actor)
	compactTask := testTaskContext(t, "task_manual_compact", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{turnTask, compactTask}}
	store := &controlTaskLifecycleStore{}
	runner := &taskContextRunner{}
	provider := &compactTaskContextProvider{}
	c := New(Options{
		Actor: actor, Runner: runner, Executor: newManualCompactExecutor(provider),
		TaskContextInitializer: initializer, RequireTaskContext: true,
		TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: store},
	})

	if err := c.Run(context.Background(), "ordinary turn"); err != nil {
		t.Fatal(err)
	}
	if err := c.Compact(context.Background(), "keep decisions"); err != nil {
		t.Fatal(err)
	}

	if initializer.callCount() != 2 {
		t.Fatalf("initializer calls = %d, want 2 independent tasks", initializer.callCount())
	}
	if len(runner.contexts) != 1 || runner.contexts[0].TaskID != turnTask.TaskID {
		t.Fatalf("ordinary turn contexts = %+v", runner.contexts)
	}
	calls, contexts := provider.snapshot()
	if calls != 1 || len(contexts) != 1 || contexts[0].TaskID != compactTask.TaskID {
		t.Fatalf("compact provider calls/contexts = %d/%+v", calls, contexts)
	}
	updates := store.snapshot()
	if len(updates) != 4 ||
		updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning ||
		updates[1].From != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskCompleted ||
		updates[2].TaskID != compactTask.TaskID || updates[2].From != modelbilling.TaskReady || updates[2].To != modelbilling.TaskRunning ||
		updates[3].TaskID != compactTask.TaskID || updates[3].From != modelbilling.TaskRunning || updates[3].To != modelbilling.TaskCompleted {
		t.Fatalf("task lifecycle updates = %#v", updates)
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after compact = %+v, want idle", status)
	}
}

func TestStrictSubscriptionMessageSummariesCreateTaskLifecycle(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Controller) error
	}{
		{name: "from", run: func(c *Controller) error { return c.SummarizeFrom(context.Background(), 0) }},
		{name: "up_to", run: func(c *Controller) error { return c.SummarizeUpTo(context.Background(), 0) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := testActor()
			task := testSubscriptionTaskContext(t, "task_subscription_summary_"+tt.name, actor)
			initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{task}}
			store := &controlTaskLifecycleStore{}
			dispatcher, calls := newSubscriptionSummaryDispatcher(t, store)
			c := New(Options{
				Actor: actor, Executor: newMessageSummarizeExecutor(dispatcher),
				SessionPath:            filepath.Join(t.TempDir(), "session.jsonl"),
				TaskContextInitializer: initializer, RequireTaskContext: true,
				TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: store},
			})
			c.checkpoints.begin("second turn", 3)

			if err := tt.run(c); err != nil {
				t.Fatal(err)
			}
			if initializer.callCount() != 1 {
				t.Fatalf("initializer calls = %d, want 1", initializer.callCount())
			}
			contexts := calls.snapshot()
			if len(contexts) != 1 || contexts[0].TaskID != task.TaskID || contexts[0].BillingSource != modelbilling.BillingSubscriptionPool {
				t.Fatalf("subscription summary contexts = %+v", contexts)
			}
			updates := store.snapshot()
			if len(updates) != 2 ||
				updates[0].TaskID != task.TaskID || updates[0].From != modelbilling.TaskReady || updates[0].To != modelbilling.TaskRunning ||
				updates[1].TaskID != task.TaskID || updates[1].From != modelbilling.TaskRunning || updates[1].To != modelbilling.TaskUsagePending {
				t.Fatalf("task lifecycle updates = %#v", updates)
			}
			if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
				t.Fatalf("status after message summary = %+v, want idle", status)
			}
		})
	}
}

func TestStrictMessageSummaryCancelDuringInitializationRestoresIdle(t *testing.T) {
	actor := testActor()
	store := &controlTaskLifecycleStore{}
	dispatcher, calls := newSubscriptionSummaryDispatcher(t, store)
	initializer := &blockingTaskContextInitializer{
		task:    testSubscriptionTaskContext(t, "task_subscription_summary_cancel", actor),
		started: make(chan struct{}), release: make(chan struct{}),
	}
	c := New(Options{
		Actor: actor, Executor: newMessageSummarizeExecutor(dispatcher),
		SessionPath:            filepath.Join(t.TempDir(), "session.jsonl"),
		TaskContextInitializer: initializer, RequireTaskContext: true,
	})
	c.checkpoints.begin("second turn", 3)

	result := make(chan error, 1)
	go func() { result <- c.SummarizeFrom(context.Background(), 0) }()
	select {
	case <-initializer.started:
	case <-time.After(time.Second):
		t.Fatal("message summary initializer did not start")
	}
	c.Cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled SummarizeFrom error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled SummarizeFrom did not return")
	}
	if contexts := calls.snapshot(); len(contexts) != 0 {
		t.Fatalf("summary dispatcher contexts = %+v, want none", contexts)
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after message summary cancellation = %+v, want idle", status)
	}
}

func TestStrictManualCompactReplacesCallerTaskContextWithFreshTask(t *testing.T) {
	actor := testActor()
	stale := testTaskContext(t, "task_stale_completed_turn", actor)
	fresh := testTaskContext(t, "task_fresh_manual_compact", actor)
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{fresh}}
	store := &controlTaskLifecycleStore{}
	provider := &compactTaskContextProvider{}
	c := New(Options{
		Actor: actor, Executor: newManualCompactExecutor(provider),
		TaskContextInitializer: initializer, RequireTaskContext: true,
		TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: store},
	})
	ctx, err := modelbilling.WithTaskContext(context.Background(), stale)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Compact(ctx, "keep decisions"); err != nil {
		t.Fatal(err)
	}
	if initializer.callCount() != 1 {
		t.Fatalf("initializer calls = %d, want a fresh manual compact task", initializer.callCount())
	}
	calls, contexts := provider.snapshot()
	if calls != 1 || len(contexts) != 1 || contexts[0].TaskID != fresh.TaskID {
		t.Fatalf("compact contexts = %d/%+v, want fresh task %q", calls, contexts, fresh.TaskID)
	}
	for _, update := range store.snapshot() {
		if update.TaskID == stale.TaskID {
			t.Fatalf("manual compact reused stale task: %+v", update)
		}
	}
}

func TestStrictManualCompactRestoresIdleAfterInitializerError(t *testing.T) {
	actor := testActor()
	want := errors.New("compact initializer failed")
	initializer := &recordingTaskContextInitializer{err: want}
	provider := &compactTaskContextProvider{}
	c := New(Options{
		Actor: actor, Executor: newManualCompactExecutor(provider),
		TaskContextInitializer: initializer, RequireTaskContext: true,
		TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: &controlTaskLifecycleStore{}},
	})

	if err := c.Compact(context.Background(), ""); !errors.Is(err, want) {
		t.Fatalf("Compact initializer error = %v, want %v", err, want)
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after initializer error = %+v, want idle", status)
	}
	initializer.mu.Lock()
	initializer.err = nil
	initializer.tasks = []modelbilling.TaskContext{testTaskContext(t, "task_after_compact_error", actor)}
	initializer.mu.Unlock()
	if err := c.Compact(context.Background(), ""); err != nil {
		t.Fatalf("Compact after initializer error = %v", err)
	}
}

func TestStrictManualCompactCancelDuringInitializationRestoresIdle(t *testing.T) {
	actor := testActor()
	initializer := &blockingTaskContextInitializer{
		task: testTaskContext(t, "task_compact_cancel", actor), started: make(chan struct{}), release: make(chan struct{}),
	}
	c := New(Options{
		Actor: actor, Executor: newManualCompactExecutor(&compactTaskContextProvider{}),
		TaskContextInitializer: initializer, RequireTaskContext: true,
	})

	result := make(chan error, 1)
	go func() { result <- c.Compact(context.Background(), "") }()
	select {
	case <-initializer.started:
	case <-time.After(time.Second):
		t.Fatal("compact initializer did not start")
	}
	c.Cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled Compact error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled Compact did not return")
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after compact cancellation = %+v, want idle", status)
	}
}

func TestStrictManualCompactRecoversInitializerPanicAndRestoresIdle(t *testing.T) {
	actor := testActor()
	initializer := &panicOnceTaskContextInitializer{
		task: testTaskContext(t, "task_after_compact_panic", actor), panicNext: true,
	}
	c := New(Options{
		Actor: actor, Executor: newManualCompactExecutor(&compactTaskContextProvider{}),
		TaskContextInitializer: initializer, RequireTaskContext: true,
		TaskLifecycle: modelbilling.TaskLifecycleRunner{Tasks: &controlTaskLifecycleStore{}},
	})

	err := c.Compact(context.Background(), "")
	if err == nil || err.Error() != "internal error: initializer panic" {
		t.Fatalf("Compact initializer panic error = %v", err)
	}
	if status := c.RuntimeStatus(); status.Running || status.Cancellable || status.CancelRequested {
		t.Fatalf("status after compact initializer panic = %+v, want idle", status)
	}
	if err := c.Compact(context.Background(), ""); err != nil {
		t.Fatalf("Compact after initializer panic = %v", err)
	}
	if initializer.callCount() != 2 {
		t.Fatalf("initializer calls = %d, want 2", initializer.callCount())
	}
}

func newManualCompactExecutor(prov provider.Provider) *agent.Agent {
	session := agent.NewSession("system")
	large := strings.Repeat("work output ", 100)
	for _, message := range []provider.Message{
		{Role: provider.RoleUser, Content: "initial task"},
		{Role: provider.RoleAssistant, Content: large},
		{Role: provider.RoleAssistant, Content: large},
		{Role: provider.RoleUser, Content: "preserve this decision"},
		{Role: provider.RoleAssistant, Content: large},
		{Role: provider.RoleUser, Content: "recent user turn"},
		{Role: provider.RoleAssistant, Content: "recent answer"},
	} {
		session.Add(message)
	}
	return agent.New(prov, tool.NewRegistry(), session, agent.Options{RecentKeep: 2, DerivedProvider: prov}, event.Discard)
}

func newMessageSummarizeExecutor(dispatcher agent.StreamDispatcher) *agent.Agent {
	session := agent.NewSession("system")
	for _, message := range []provider.Message{
		{Role: provider.RoleUser, Content: "first user turn"},
		{Role: provider.RoleAssistant, Content: "first answer"},
		{Role: provider.RoleUser, Content: "second user turn"},
		{Role: provider.RoleAssistant, Content: "second answer"},
	} {
		session.Add(message)
	}
	return agent.New(&compactTaskContextProvider{}, tool.NewRegistry(), session, agent.Options{
		DerivedStreamDispatcher: dispatcher,
	}, event.Discard)
}

func newSubscriptionSummaryDispatcher(t *testing.T, store *controlTaskLifecycleStore) (agent.StreamDispatcher, *recordingSubscriptionCallStore) {
	t.Helper()
	calls := &recordingSubscriptionCallStore{}
	dispatcher := &modelbilling.MainExecutorDispatcher{
		Calls: calls, Tasks: store, Gateway: subscriptionSummaryGateway{},
		Evidence: subscriptionSummaryEvidenceResolver{}, IDs: subscriptionSummaryIDGenerator{},
	}
	bound, err := modelbilling.NewRoleBoundStreamDispatcher(dispatcher, modelbilling.CallDispatchSpec{
		Role: modelbilling.CallRoleCompaction, ModelID: "subscription_model_01",
	})
	if err != nil {
		t.Fatal(err)
	}
	return bound, calls
}

func waitForTaskContextTurns(t *testing.T, turns *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		turns.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task context turns did not finish")
	}
}

var _ agent.Runner = (*taskContextRunner)(nil)
var _ agent.Runner = (*taskContextScriptRunner)(nil)
var _ agent.Runner = (*panickingTaskContextRunner)(nil)
