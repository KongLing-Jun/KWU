package tasktree

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type captureSink struct {
	mu     sync.Mutex
	events []event.Event
}

func (s *captureSink) Emit(e event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

type testRunner struct {
	inputs []string
}

type terminalSequenceDispatcher struct{ err error }

func (d terminalSequenceDispatcher) Stream(ctx context.Context, base provider.Provider, request provider.Request) (<-chan provider.Chunk, error) {
	chunks, _, err := d.StreamWithTerminalResult(ctx, base, request)
	return chunks, err
}

func (d terminalSequenceDispatcher) StreamWithTerminalResult(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	chunks := make(chan provider.Chunk)
	terminal := make(chan error, 1)
	go func() {
		chunks <- provider.Chunk{Type: provider.ChunkError, Err: d.err}
		terminal <- d.err
		close(terminal)
		close(chunks)
	}()
	return chunks, terminal, nil
}

type unusedTaskTreeProvider struct{}

func (unusedTaskTreeProvider) Name() string { return "unused" }
func (unusedTaskTreeProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	panic("base provider must not bypass dispatcher")
}

func (r *testRunner) Run(_ context.Context, input string) error {
	r.inputs = append(r.inputs, input)
	return nil
}

func TestOrchestratorRunReportsNotImplementedWhenPlannerMissing(t *testing.T) {
	sink := &captureSink{}
	orch := NewOrchestrator(Options{Sink: sink})

	err := orch.Run(context.Background(), "build a task tree")
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Run error = %v, want ErrNotImplemented", err)
	}
	if len(sink.events) != 3 {
		t.Fatalf("events = %v, want TurnStarted, planning phase, and Notice", sink.events)
	}
	if sink.events[0].Kind != event.TurnStarted {
		t.Fatalf("first event kind = %v, want TurnStarted", sink.events[0].Kind)
	}
	if sink.events[1].Kind != event.Phase || sink.events[1].Text != "task_tree: planning" {
		t.Fatalf("second event = %+v, want planning phase", sink.events[1])
	}
	if sink.events[2].Kind != event.Notice || sink.events[2].Level != event.LevelWarn {
		t.Fatalf("third event = %+v, want warning notice", sink.events[2])
	}
}

func TestOrchestratorPropagatesNodeTerminalPersistenceFailure(t *testing.T) {
	want := errors.New("usage terminal persistence failed")
	child := agent.New(unusedTaskTreeProvider{}, tool.NewRegistry(), agent.NewSession("system"), agent.Options{
		StreamDispatcher: terminalSequenceDispatcher{err: want},
	}, event.Discard)
	tree := sampleOrchestratorTree()
	orchestrator := NewOrchestrator(Options{
		Planner: PlannerFunc(func(context.Context, string) (*TaskTree, error) { return tree, nil }),
		Store:   NewMemoryStateStore(),
		Dispatcher: NodeDispatcherFunc(func(ctx context.Context, _ *TaskTree, _ string, _ NodeContext) (NodeResult, error) {
			return NodeResult{}, child.Run(ctx, "node")
		}),
	})

	err := orchestrator.Run(context.Background(), "execute")
	if !errors.Is(err, want) || !agent.IsStreamDispatcherTerminalError(err) {
		t.Fatalf("Orchestrator.Run error = %v, want terminal persistence failure", err)
	}
}

func TestOrchestratorRunSurfacesPlannerFailureNotice(t *testing.T) {
	sink := &captureSink{}
	orch := NewOrchestrator(Options{
		Sink: sink,
		Planner: PlannerFunc(func(context.Context, string) (*TaskTree, error) {
			return nil, ErrInvalidPlannerOutput
		}),
	})

	err := orch.Run(context.Background(), "build a task tree")
	if !errors.Is(err, ErrInvalidPlannerOutput) {
		t.Fatalf("Run error = %v, want ErrInvalidPlannerOutput", err)
	}
	if len(sink.events) != 3 {
		t.Fatalf("events = %+v, want TurnStarted, planning phase, and Notice", sink.events)
	}
	notice := sink.events[2]
	if notice.Kind != event.Notice || notice.Level != event.LevelWarn {
		t.Fatalf("third event = %+v, want warning notice", notice)
	}
	if !strings.Contains(notice.Text, "task_tree planning failed") {
		t.Fatalf("notice text = %q", notice.Text)
	}
}

func TestOrchestratorRunPlansExecutesAndEmitsSummary(t *testing.T) {
	sink := &captureSink{}
	store := NewMemoryStateStore()
	var plannerInput string
	tree := sampleOrchestratorTree()
	orch := NewOrchestrator(Options{
		Planner: PlannerFunc(func(_ context.Context, input string) (*TaskTree, error) {
			plannerInput = input
			return tree, nil
		}),
		Store: store,
		Sink:  sink,
		Dispatcher: NodeDispatcherFunc(func(_ context.Context, _ *TaskTree, nodeID string, _ NodeContext) (NodeResult, error) {
			return NodeResult{Summary: "executed " + nodeID}, nil
		}),
	})

	if err := orch.Run(context.Background(), "ship task tree mode"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if plannerInput != "ship task tree mode" {
		t.Fatalf("planner input = %q", plannerInput)
	}
	snapshot, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("state store did not receive a snapshot")
	}
	if snapshot.Mode != SnapshotModeTaskTree || snapshot.Tree == nil || snapshot.Tree.ID != "tree-runner" {
		t.Fatalf("stored snapshot = %+v", snapshot)
	}
	if snapshot.Tree.Status != StatusCompleted {
		t.Fatalf("tree status = %s, want completed", snapshot.Tree.Status)
	}
	if snapshot.Tree.Nodes["root"].Status != StatusCompleted {
		t.Fatalf("root status = %s, want completed", snapshot.Tree.Nodes["root"].Status)
	}
	if snapshot.Tree.Nodes["root"].Result == nil || snapshot.Tree.Nodes["root"].Result.Summary != "executed root" {
		t.Fatalf("root result = %+v", snapshot.Tree.Nodes["root"].Result)
	}
	if snapshot.Tree == tree {
		t.Fatal("store should keep a snapshot copy, not the planner's tree pointer")
	}

	if len(sink.events) < 10 {
		t.Fatalf("events = %+v, want full planning, execution, completion, and summary stream", sink.events)
	}
	if sink.events[0].Kind != event.TurnStarted {
		t.Fatalf("first event kind = %v, want TurnStarted", sink.events[0].Kind)
	}
	created := assertTaskTreePayloadEvent(t, sink.events, event.TaskTreeCreated, "", string(StatusPending))
	if created.TreeID != "tree-runner" || len(created.RootIDs) != 1 || created.RootIDs[0] != "root" {
		t.Fatalf("created task tree payload = %+v", created)
	}
	if len(created.Nodes) != 1 || created.Nodes[0].NodeID != "root" || created.Nodes[0].Title != "Root task" {
		t.Fatalf("created task tree nodes = %+v", created.Nodes)
	}
	if sink.events[3].Text != "task_tree: tree_created" {
		t.Fatalf("tree created phase = %+v", sink.events[3])
	}
	assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeStarted, "root", string(StatusRunning))
	completed := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeCompleted, "root", string(StatusCompleted))
	if completed.Result != "executed root" {
		t.Fatalf("completed payload = %+v", completed)
	}
	treeDone := assertTaskTreePayloadEvent(t, sink.events, event.TaskTreeCompleted, "", string(StatusCompleted))
	if treeDone.TreeID != "tree-runner" {
		t.Fatalf("tree completed payload = %+v", treeDone)
	}
	summary := assertTaskTreePayloadEvent(t, sink.events, event.TaskTreeSummary, "", string(StatusCompleted))
	if !strings.Contains(summary.Summary, `Completed "ship task tree mode"`) {
		t.Fatalf("summary payload = %+v", summary)
	}
	last := sink.events[len(sink.events)-1]
	if last.Kind != event.Message || !strings.Contains(last.Text, `Completed "ship task tree mode"`) {
		t.Fatalf("final message = %+v", last)
	}
}

func TestOrchestratorRunSummarizesNodeFailureWithoutRunnerError(t *testing.T) {
	sink := &captureSink{}
	store := NewMemoryStateStore()
	dispatchErr := errors.New("verification failed")
	orch := NewOrchestrator(Options{
		Planner: PlannerFunc(func(context.Context, string) (*TaskTree, error) {
			return sampleOrchestratorFailureTree(), nil
		}),
		Store: store,
		Sink:  sink,
		Dispatcher: NodeDispatcherFunc(func(_ context.Context, _ *TaskTree, nodeID string, _ NodeContext) (NodeResult, error) {
			if nodeID == "verify" {
				return NodeResult{Summary: "verification did not pass"}, dispatchErr
			}
			return NodeResult{Summary: "done " + nodeID}, nil
		}),
	})

	if err := orch.Run(context.Background(), "ship safely"); err != nil {
		t.Fatalf("Run should summarize node failure without returning runner error, got %v", err)
	}
	snapshot, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("state store did not receive a snapshot")
	}
	if snapshot.Tree.Status != StatusFailed {
		t.Fatalf("tree status = %s, want failed", snapshot.Tree.Status)
	}
	if snapshot.Tree.Nodes["inspect"].Status != StatusCompleted {
		t.Fatalf("inspect status = %s, want completed", snapshot.Tree.Nodes["inspect"].Status)
	}
	if snapshot.Tree.Nodes["verify"].Status != StatusFailed {
		t.Fatalf("verify status = %s, want failed", snapshot.Tree.Nodes["verify"].Status)
	}
	if snapshot.Tree.Nodes["publish"].Status != StatusPending {
		t.Fatalf("publish status = %s, want pending after dependency failure", snapshot.Tree.Nodes["publish"].Status)
	}
	failed := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeFailed, "verify", string(StatusFailed))
	if failed.Error != "verification failed" {
		t.Fatalf("failed payload = %+v", failed)
	}
	treeDone := assertTaskTreePayloadEvent(t, sink.events, event.TaskTreeCompleted, "", string(StatusFailed))
	if treeDone.TreeID != "tree-runner-failure" {
		t.Fatalf("tree completed payload = %+v", treeDone)
	}
	summary := assertTaskTreePayloadEvent(t, sink.events, event.TaskTreeSummary, "", string(StatusFailed))
	for _, want := range []string{
		`Partially completed "ship safely"`,
		"verification failed",
		"[publish] Publish remained pending because [verify] Verify is failed: verification failed",
	} {
		if !strings.Contains(summary.Summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary.Summary)
		}
	}
}

func TestOrchestratorExposesInjectedDependencies(t *testing.T) {
	base := &testRunner{}
	executor := &testRunner{}
	registry := tool.NewRegistry()
	store := NewMemoryStateStore()
	planner := PlannerFunc(func(context.Context, string) (*TaskTree, error) {
		return sampleOrchestratorTree(), nil
	})

	orch := NewOrchestrator(Options{
		BaseRunner: base,
		Planner:    planner,
		Executor:   executor,
		Registry:   registry,
		Store:      store,
	})

	if orch.BaseRunner() != base {
		t.Fatalf("BaseRunner = %T, want injected runner", orch.BaseRunner())
	}
	if orch.Planner() == nil {
		t.Fatal("Planner should be available")
	}
	if orch.Executor() != executor {
		t.Fatalf("Executor = %T, want injected executor", orch.Executor())
	}
	if orch.Registry() != registry {
		t.Fatalf("Registry = %p, want %p", orch.Registry(), registry)
	}
	if orch.StateStore() != store {
		t.Fatalf("StateStore = %T, want injected store", orch.StateStore())
	}
}

func TestMemoryStateStoreClonesSnapshots(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleOrchestratorTree()
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	first, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot first: %v", err)
	}
	if !ok {
		t.Fatal("LoadSnapshot ok = false")
	}
	first.Tree.Nodes["root"].Title = "mutated"

	second, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot second: %v", err)
	}
	if !ok {
		t.Fatal("LoadSnapshot second ok = false")
	}
	if second.Tree.Nodes["root"].Title != "Root task" {
		t.Fatalf("stored tree was mutated through loaded snapshot: %+v", second.Tree.Nodes["root"])
	}
}

func sampleOrchestratorTree() *TaskTree {
	tree := NewTaskTree("tree-runner", "ship task tree mode", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = &TaskNode{
		ID:     "root",
		Title:  "Root task",
		Status: StatusPending,
	}
	return tree
}

func sampleOrchestratorFailureTree() *TaskTree {
	tree := NewTaskTree("tree-runner-failure", "ship safely", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.RootIDs = []string{"inspect", "verify", "publish"}
	tree.Nodes["inspect"] = &TaskNode{
		ID:     "inspect",
		Title:  "Inspect",
		Status: StatusPending,
	}
	tree.Nodes["verify"] = &TaskNode{
		ID:        "verify",
		Title:     "Verify",
		DependsOn: []string{"inspect"},
		Status:    StatusPending,
	}
	tree.Nodes["publish"] = &TaskNode{
		ID:        "publish",
		Title:     "Publish",
		DependsOn: []string{"verify"},
		Status:    StatusPending,
	}
	return tree
}
