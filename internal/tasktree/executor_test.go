package tasktree

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/event"
)

func TestExecuteOneRunsReadyNodeAndWritesCompletedResult(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	sink := &captureSink{}
	var gotContext NodeContext
	var dispatchCalls int
	orch := NewOrchestrator(Options{
		Store: store,
		Sink:  sink,
		Dispatcher: NodeDispatcherFunc(func(ctx context.Context, tree *TaskTree, nodeID string, nodeContext NodeContext) (NodeResult, error) {
			dispatchCalls++
			gotContext = nodeContext
			if nodeID != "patch" {
				t.Fatalf("dispatch nodeID = %q, want patch", nodeID)
			}
			if tree.Nodes["patch"].Status != StatusRunning {
				t.Fatalf("dispatch tree status = %s, want running", tree.Nodes["patch"].Status)
			}
			snapshot, ok, err := store.LoadSnapshot(ctx)
			if err != nil {
				t.Fatalf("LoadSnapshot during dispatch: %v", err)
			}
			if !ok || snapshot.Tree.Nodes["patch"].Status != StatusRunning {
				t.Fatalf("stored snapshot during dispatch = %+v", snapshot.Tree.Nodes["patch"])
			}
			return NodeResult{Summary: "patched implementation"}, nil
		}),
	})

	if err := orch.ExecuteOne(context.Background(), "patch"); err != nil {
		t.Fatalf("ExecuteOne: %v", err)
	}
	if dispatchCalls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", dispatchCalls)
	}
	if gotContext.Goal != tree.Goal || gotContext.NodeID != "patch" {
		t.Fatalf("node context = %+v", gotContext)
	}
	if got := gotContext.NodePath; !reflect.DeepEqual(got, []string{"Implement task tree context", "Patch implementation"}) {
		t.Fatalf("node path = %v", got)
	}
	if len(gotContext.DependencyResults) != 1 || gotContext.DependencyResults[0].NodeID != "inspect" {
		t.Fatalf("dependency results = %+v", gotContext.DependencyResults)
	}

	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot final: %v", err)
	}
	if !ok {
		t.Fatal("final snapshot missing")
	}
	node := final.Tree.Nodes["patch"]
	if node.Status != StatusCompleted {
		t.Fatalf("patch status = %s, want completed", node.Status)
	}
	if node.Result == nil || node.Result.Summary != "patched implementation" {
		t.Fatalf("patch result = %+v", node.Result)
	}
	assertTaskTreePhase(t, sink.events, "task_tree: node_started patch")
	assertTaskTreePhase(t, sink.events, "task_tree: node_completed patch")
	started := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeStarted, "patch", string(StatusRunning))
	if started.TreeID != "tree-ctx" || started.Title != "Patch implementation" || started.ParentID != "root" {
		t.Fatalf("started payload = %+v", started)
	}
	completed := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeCompleted, "patch", string(StatusCompleted))
	if completed.Result != "patched implementation" {
		t.Fatalf("completed payload = %+v", completed)
	}
}

func TestExecuteNextRunsOneReadyNodeAndRecomputesAfterCompletion(t *testing.T) {
	store := NewMemoryStateStore()
	tree := NewTaskTree("tree-serial-exec", "execute serial nodes", schedulerNow())
	tree.RootIDs = []string{"design", "implement"}
	tree.Nodes["design"] = schedulerNode("design", StatusPending)
	tree.Nodes["implement"] = schedulerNode("implement", StatusPending)
	tree.Nodes["implement"].DependsOn = []string{"design"}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	var dispatched []string
	orch := NewOrchestrator(Options{
		Store: store,
		Dispatcher: NodeDispatcherFunc(func(_ context.Context, _ *TaskTree, nodeID string, _ NodeContext) (NodeResult, error) {
			dispatched = append(dispatched, nodeID)
			return NodeResult{Summary: "done " + nodeID}, nil
		}),
	})

	id, ok, err := orch.ExecuteNext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteNext first: %v", err)
	}
	if !ok || id != "design" {
		t.Fatalf("first ExecuteNext = %q/%v, want design/true", id, ok)
	}
	id, ok, err = orch.ExecuteNext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteNext second: %v", err)
	}
	if !ok || id != "implement" {
		t.Fatalf("second ExecuteNext = %q/%v, want implement/true", id, ok)
	}
	if !reflect.DeepEqual(dispatched, []string{"design", "implement"}) {
		t.Fatalf("dispatch order = %v", dispatched)
	}
}

func TestExecuteOneDispatchErrorFailsNodeAndStopsDependentScheduling(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["root"].Children = append(tree.Nodes["root"].Children, "verify")
	tree.Nodes["verify"] = &TaskNode{
		ID:             "verify",
		ParentID:       "root",
		Title:          "Verify patch",
		DependsOn:      []string{"patch"},
		Status:         StatusPending,
		ExpectedOutput: "Verification result",
	}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	dispatchErr := errors.New("executor crashed")
	sink := &captureSink{}
	orch := NewOrchestrator(Options{
		Store: store,
		Sink:  sink,
		Dispatcher: NodeDispatcherFunc(func(context.Context, *TaskTree, string, NodeContext) (NodeResult, error) {
			return NodeResult{}, dispatchErr
		}),
	})

	err := orch.ExecuteOne(context.Background(), "patch")
	if !errors.Is(err, dispatchErr) {
		t.Fatalf("ExecuteOne error = %v, want dispatch error", err)
	}
	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot final: %v", err)
	}
	if !ok {
		t.Fatal("final snapshot missing")
	}
	patch := final.Tree.Nodes["patch"]
	if patch.Status != StatusFailed {
		t.Fatalf("patch status = %s, want failed", patch.Status)
	}
	if patch.Result == nil || !strings.Contains(patch.Result.Error, "executor crashed") {
		t.Fatalf("patch result = %+v", patch.Result)
	}
	ready, err := ReadyNodeIDs(final.Tree)
	if err != nil {
		t.Fatalf("ReadyNodeIDs final: %v", err)
	}
	for _, id := range ready {
		if id == "verify" {
			t.Fatalf("dependent verify should not be ready after patch failure; ready=%v", ready)
		}
	}
	assertTaskTreePhase(t, sink.events, "task_tree: node_failed patch")
	assertTaskTreeNotice(t, sink.events, "task_tree node patch failed")
	failed := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeFailed, "patch", string(StatusFailed))
	if !strings.Contains(failed.Error, "executor crashed") {
		t.Fatalf("failed payload = %+v", failed)
	}
}

func TestExecuteOneWritesBlockedResult(t *testing.T) {
	store := NewMemoryStateStore()
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: sampleExecutionTree()}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	sink := &captureSink{}
	orch := NewOrchestrator(Options{
		Store: store,
		Sink:  sink,
		Dispatcher: NodeDispatcherFunc(func(context.Context, *TaskTree, string, NodeContext) (NodeResult, error) {
			return NodeResult{Summary: "blocked", Blocker: "needs user decision"}, nil
		}),
	})

	if err := orch.ExecuteOne(context.Background(), "patch"); err != nil {
		t.Fatalf("ExecuteOne: %v", err)
	}
	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("final snapshot missing")
	}
	node := final.Tree.Nodes["patch"]
	if node.Status != StatusBlocked {
		t.Fatalf("patch status = %s, want blocked", node.Status)
	}
	if node.Result == nil || node.Result.Blocker != "needs user decision" {
		t.Fatalf("patch result = %+v", node.Result)
	}
	assertTaskTreePhase(t, sink.events, "task_tree: node_blocked patch")
	assertTaskTreeNotice(t, sink.events, "needs user decision")
	blocked := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeFailed, "patch", string(StatusBlocked))
	if blocked.Error != "needs user decision" || blocked.Result != "blocked" {
		t.Fatalf("blocked payload = %+v", blocked)
	}
}

func TestExecuteOneRejectsNodeThatIsNotReady(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleExecutionTree()
	tree.Nodes["inspect"].Status = StatusFailed
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	var dispatchCalls int
	orch := NewOrchestrator(Options{
		Store: store,
		Dispatcher: NodeDispatcherFunc(func(context.Context, *TaskTree, string, NodeContext) (NodeResult, error) {
			dispatchCalls++
			return NodeResult{Summary: "should not run"}, nil
		}),
	})

	err := orch.ExecuteOne(context.Background(), "patch")
	if !errors.Is(err, ErrNodeNotReady) {
		t.Fatalf("ExecuteOne error = %v, want ErrNodeNotReady", err)
	}
	if dispatchCalls != 0 {
		t.Fatalf("dispatch calls = %d, want 0", dispatchCalls)
	}
	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("snapshot missing")
	}
	if final.Tree.Nodes["patch"].Status != StatusPending || final.Tree.Nodes["patch"].Result != nil {
		t.Fatalf("patch mutated despite not-ready error: %+v", final.Tree.Nodes["patch"])
	}
}

func TestDefaultNodeDispatcherCallsExecutorWithLocalContext(t *testing.T) {
	runner := &testRunner{}
	dispatcher := executorNodeDispatcher{executor: runner}
	nodeContext := NodeContext{Goal: "ship", NodeID: "node-1", NodeTitle: "Do it"}

	result, err := dispatcher.DispatchNode(context.Background(), nil, "node-1", nodeContext)
	if err != nil {
		t.Fatalf("DispatchNode: %v", err)
	}
	if result.Summary == "" {
		t.Fatal("default dispatcher should return a result summary")
	}
	if len(runner.inputs) != 1 {
		t.Fatalf("executor inputs = %d, want 1", len(runner.inputs))
	}
	if !strings.Contains(runner.inputs[0], `"node_id": "node-1"`) || !strings.Contains(runner.inputs[0], `"goal": "ship"`) {
		t.Fatalf("executor prompt missing local context:\n%s", runner.inputs[0])
	}
}

func assertTaskTreePhase(t *testing.T, events []event.Event, text string) {
	t.Helper()
	for _, e := range events {
		if e.Kind == event.Phase && e.Text == text && e.Source == SnapshotModeTaskTree {
			return
		}
	}
	t.Fatalf("missing phase %q in events %+v", text, events)
}

func assertTaskTreeNotice(t *testing.T, events []event.Event, want string) {
	t.Helper()
	for _, e := range events {
		if e.Kind == event.Notice && strings.Contains(e.Text, want) && e.Source == SnapshotModeTaskTree {
			return
		}
	}
	t.Fatalf("missing notice containing %q in events %+v", want, events)
}

func assertTaskTreePayloadEvent(t *testing.T, events []event.Event, kind event.Kind, nodeID, status string) *event.TaskTreePayload {
	t.Helper()
	for _, e := range events {
		if e.Kind != kind || e.Source != SnapshotModeTaskTree || e.TaskTree == nil {
			continue
		}
		if nodeID != "" && e.TaskTree.NodeID != nodeID {
			continue
		}
		if status != "" && e.TaskTree.Status != status {
			continue
		}
		return e.TaskTree
	}
	t.Fatalf("missing task tree payload event kind=%v node=%q status=%q in %+v", kind, nodeID, status, events)
	return nil
}
