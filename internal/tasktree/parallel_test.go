package tasktree

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"kwu/internal/event"
)

func TestExecuteBatchRunsIndependentNodesWithConcurrencyLimit(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleParallelExecutionTree("a", "b", "c")
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	started := make(chan string, 3)
	release := make(chan struct{})
	var mu sync.Mutex
	active := 0
	maxActive := 0
	orch := NewOrchestrator(Options{
		Store:       store,
		MaxParallel: 2,
		Dispatcher: NodeDispatcherFunc(func(_ context.Context, _ *TaskTree, nodeID string, nodeContext NodeContext) (NodeResult, error) {
			if nodeContext.NodeID != nodeID {
				t.Fatalf("node context ID = %q, want %q", nodeContext.NodeID, nodeID)
			}
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()
			started <- nodeID
			<-release
			mu.Lock()
			active--
			mu.Unlock()
			return NodeResult{Summary: "done " + nodeID}, nil
		}),
	})

	done := make(chan struct{})
	var results []BatchNodeResult
	var runErr error
	go func() {
		results, runErr = orch.ExecuteBatch(context.Background(), []string{"a", "b", "c"})
		close(done)
	}()

	first := waitStartedNode(t, started)
	second := waitStartedNode(t, started)
	if first == second {
		t.Fatalf("same node started twice: %q", first)
	}
	select {
	case third := <-started:
		t.Fatalf("third node %q started before a concurrency slot was released", third)
	default:
	}

	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteBatch did not finish")
	}
	if runErr != nil {
		t.Fatalf("ExecuteBatch: %v", runErr)
	}
	if maxActive != 2 {
		t.Fatalf("max active workers = %d, want 2", maxActive)
	}
	if got := batchNodeIDs(results); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("result order = %v, want [a b c]", got)
	}

	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("final snapshot missing")
	}
	for _, id := range []string{"a", "b", "c"} {
		node := final.Tree.Nodes[id]
		if node.Status != StatusCompleted {
			t.Fatalf("%s status = %s, want completed", id, node.Status)
		}
		if node.Result == nil || node.Result.Summary != "done "+id {
			t.Fatalf("%s result = %+v", id, node.Result)
		}
	}
}

func TestExecuteBatchPreservesResultAttributionDespiteCompletionOrder(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleParallelExecutionTree("slow", "fast")
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	slowRelease := make(chan struct{})
	orch := NewOrchestrator(Options{
		Store:       store,
		MaxParallel: 2,
		Dispatcher: NodeDispatcherFunc(func(_ context.Context, _ *TaskTree, nodeID string, _ NodeContext) (NodeResult, error) {
			if nodeID == "slow" {
				<-slowRelease
			}
			return NodeResult{Summary: "summary for " + nodeID}, nil
		}),
	})

	done := make(chan struct{})
	var results []BatchNodeResult
	var runErr error
	go func() {
		results, runErr = orch.ExecuteBatch(context.Background(), []string{"slow", "fast"})
		close(done)
	}()
	close(slowRelease)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteBatch did not finish")
	}
	if runErr != nil {
		t.Fatalf("ExecuteBatch: %v", runErr)
	}
	if got := batchNodeIDs(results); !reflect.DeepEqual(got, []string{"slow", "fast"}) {
		t.Fatalf("result order = %v, want [slow fast]", got)
	}
	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("final snapshot missing")
	}
	for _, id := range []string{"slow", "fast"} {
		if got := final.Tree.Nodes[id].Result.Summary; got != "summary for "+id {
			t.Fatalf("%s summary = %q", id, got)
		}
	}
}

func TestExecuteBatchHandlesPartialSuccessAndFailure(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleParallelExecutionTree("ok", "bad", "after")
	tree.Nodes["after"].DependsOn = []string{"bad"}
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	dispatchErr := errors.New("bad failed")
	sink := &captureSink{}
	orch := NewOrchestrator(Options{
		Store:       store,
		MaxParallel: 2,
		Sink:        sink,
		Dispatcher: NodeDispatcherFunc(func(_ context.Context, _ *TaskTree, nodeID string, _ NodeContext) (NodeResult, error) {
			if nodeID == "bad" {
				return NodeResult{}, dispatchErr
			}
			return NodeResult{Summary: "done " + nodeID}, nil
		}),
	})

	results, err := orch.ExecuteBatch(context.Background(), []string{"ok", "bad"})
	if !errors.Is(err, ErrBatchExecutionFailed) {
		t.Fatalf("ExecuteBatch error = %v, want ErrBatchExecutionFailed", err)
	}
	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}
	if results[0].NodeID != "ok" || results[0].Status != StatusCompleted || results[0].Err != nil {
		t.Fatalf("ok result = %+v", results[0])
	}
	if results[1].NodeID != "bad" || results[1].Status != StatusFailed || !errors.Is(results[1].Err, dispatchErr) {
		t.Fatalf("bad result = %+v", results[1])
	}

	final, ok, err := store.LoadSnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if !ok {
		t.Fatal("final snapshot missing")
	}
	if final.Tree.Nodes["ok"].Status != StatusCompleted || final.Tree.Nodes["ok"].Result.Summary != "done ok" {
		t.Fatalf("ok node = %+v", final.Tree.Nodes["ok"])
	}
	if final.Tree.Nodes["bad"].Status != StatusFailed || final.Tree.Nodes["bad"].Result.Error != "bad failed" {
		t.Fatalf("bad node = %+v", final.Tree.Nodes["bad"])
	}
	assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeStarted, "ok", string(StatusRunning))
	assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeStarted, "bad", string(StatusRunning))
	assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeCompleted, "ok", string(StatusCompleted))
	failed := assertTaskTreePayloadEvent(t, sink.events, event.TaskNodeFailed, "bad", string(StatusFailed))
	if failed.Error != "bad failed" {
		t.Fatalf("failed payload = %+v", failed)
	}
	ready, err := ReadyNodeIDs(final.Tree)
	if err != nil {
		t.Fatalf("ReadyNodeIDs: %v", err)
	}
	for _, id := range ready {
		if id == "after" {
			t.Fatalf("dependent node should not be ready after dependency failure: %v", ready)
		}
	}
}

func TestExecuteBatchRejectsNodesOutsideReadyBatch(t *testing.T) {
	store := NewMemoryStateStore()
	tree := NewTaskTree("tree-reject-batch", "reject invalid parallel batch", schedulerNow())
	tree.RootIDs = []string{"first", "second"}
	tree.Nodes["first"] = schedulerNode("first", StatusPending)
	tree.Nodes["second"] = schedulerNode("second", StatusPending)
	tree.Nodes["second"].DependsOn = []string{"first"}
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

	_, err := orch.ExecuteBatch(context.Background(), []string{"second"})
	if !errors.Is(err, ErrNodeNotReady) {
		t.Fatalf("ExecuteBatch error = %v, want ErrNodeNotReady", err)
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
	if final.Tree.Nodes["second"].Status != StatusPending || final.Tree.Nodes["second"].Result != nil {
		t.Fatalf("second node mutated despite rejected batch: %+v", final.Tree.Nodes["second"])
	}
}

func TestExecuteReadyBatchReturnsFalseWhenNoNodesReady(t *testing.T) {
	store := NewMemoryStateStore()
	tree := sampleParallelExecutionTree("done")
	tree.Nodes["done"].Status = StatusCompleted
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	orch := NewOrchestrator(Options{Store: store})

	results, ok, err := orch.ExecuteReadyBatch(context.Background())
	if err != nil {
		t.Fatalf("ExecuteReadyBatch: %v", err)
	}
	if ok {
		t.Fatalf("ExecuteReadyBatch ok = true with results %+v, want false", results)
	}
}

func sampleParallelExecutionTree(ids ...string) *TaskTree {
	tree := NewTaskTree("tree-parallel-exec", "execute independent nodes", schedulerNow())
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = schedulerNode("root", StatusRunning)
	tree.Nodes["root"].Children = append([]string(nil), ids...)
	for _, id := range ids {
		tree.Nodes[id] = schedulerChild(id, "root", StatusPending)
	}
	return tree
}

func batchNodeIDs(results []BatchNodeResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.NodeID)
	}
	return out
}

func waitStartedNode(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case id := <-started:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a parallel worker to start")
	}
	return ""
}
