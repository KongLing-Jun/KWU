package tasktree

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestReadyNodeIDsSelectsDependencySatisfiedNodes(t *testing.T) {
	tree := sampleSchedulerTree()
	tree.Nodes["setup"].Status = StatusCompleted

	ready, err := ReadyNodeIDs(tree)
	if err != nil {
		t.Fatalf("ReadyNodeIDs: %v", err)
	}
	want := []string{"lint", "unit"}
	if !reflect.DeepEqual(ready, want) {
		t.Fatalf("ready node IDs = %v, want %v", ready, want)
	}
}

func TestNextReadyNodeIDSupportsSerialDependencyChain(t *testing.T) {
	tree := NewTaskTree("tree-serial", "run a serial chain", schedulerNow())
	tree.RootIDs = []string{"design", "implement", "verify"}
	tree.Nodes["design"] = schedulerNode("design", StatusPending)
	tree.Nodes["implement"] = schedulerNode("implement", StatusPending)
	tree.Nodes["implement"].DependsOn = []string{"design"}
	tree.Nodes["verify"] = schedulerNode("verify", StatusPending)
	tree.Nodes["verify"].DependsOn = []string{"implement"}

	id, ok, err := NextReadyNodeID(tree)
	if err != nil {
		t.Fatalf("NextReadyNodeID first: %v", err)
	}
	if !ok || id != "design" {
		t.Fatalf("first ready = %q/%v, want design/true", id, ok)
	}

	tree.Nodes["design"].Status = StatusCompleted
	id, ok, err = NextReadyNodeID(tree)
	if err != nil {
		t.Fatalf("NextReadyNodeID second: %v", err)
	}
	if !ok || id != "implement" {
		t.Fatalf("second ready = %q/%v, want implement/true", id, ok)
	}

	tree.Nodes["implement"].Status = StatusCompleted
	tree.Nodes["verify"].Status = StatusReady
	id, ok, err = NextReadyNodeID(tree)
	if err != nil {
		t.Fatalf("NextReadyNodeID third: %v", err)
	}
	if !ok || id != "verify" {
		t.Fatalf("third ready = %q/%v, want verify/true", id, ok)
	}
}

func TestReadyBatchNodeIDsReturnsParallelSiblingBatchInStableOrder(t *testing.T) {
	tree := NewTaskTree("tree-parallel", "run independent checks", schedulerNow())
	tree.RootIDs = []string{"quality"}
	tree.Nodes["quality"] = schedulerNode("quality", StatusRunning)
	tree.Nodes["quality"].Children = []string{"lint", "unit", "docs"}
	tree.Nodes["lint"] = schedulerChild("lint", "quality", StatusPending)
	tree.Nodes["unit"] = schedulerChild("unit", "quality", StatusReady)
	tree.Nodes["docs"] = schedulerChild("docs", "quality", StatusPending)

	batch, err := ReadyBatchNodeIDs(tree)
	if err != nil {
		t.Fatalf("ReadyBatchNodeIDs: %v", err)
	}
	want := []string{"lint", "unit", "docs"}
	if !reflect.DeepEqual(batch, want) {
		t.Fatalf("ready batch = %v, want %v", batch, want)
	}
}

func TestReadyBatchNodeIDsScansAncestorBeforeDescendants(t *testing.T) {
	tree := NewTaskTree("tree-ancestor", "avoid ancestor descendant batch", schedulerNow())
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = schedulerNode("root", StatusPending)
	tree.Nodes["root"].Children = []string{"child"}
	tree.Nodes["child"] = schedulerChild("child", "root", StatusPending)

	batch, err := ReadyBatchNodeIDs(tree)
	if err != nil {
		t.Fatalf("ReadyBatchNodeIDs before root completes: %v", err)
	}
	if want := []string{"root"}; !reflect.DeepEqual(batch, want) {
		t.Fatalf("ready batch = %v, want %v", batch, want)
	}

	tree.Nodes["root"].Status = StatusCompleted
	batch, err = ReadyBatchNodeIDs(tree)
	if err != nil {
		t.Fatalf("ReadyBatchNodeIDs after root completes: %v", err)
	}
	if want := []string{"child"}; !reflect.DeepEqual(batch, want) {
		t.Fatalf("ready batch after root completes = %v, want %v", batch, want)
	}
}

func TestReadyNodeIDsAvoidsFailedDependency(t *testing.T) {
	tree := sampleSchedulerTree()
	tree.Nodes["setup"].Status = StatusFailed

	ready, err := ReadyNodeIDs(tree)
	if err != nil {
		t.Fatalf("ReadyNodeIDs: %v", err)
	}
	want := []string{"lint"}
	if !reflect.DeepEqual(ready, want) {
		t.Fatalf("ready node IDs = %v, want %v", ready, want)
	}
}

func TestReadyNodeIDsAvoidsBlockedDependency(t *testing.T) {
	tree := sampleSchedulerTree()
	tree.Nodes["setup"].Status = StatusBlocked

	ready, err := ReadyNodeIDs(tree)
	if err != nil {
		t.Fatalf("ReadyNodeIDs: %v", err)
	}
	want := []string{"lint"}
	if !reflect.DeepEqual(ready, want) {
		t.Fatalf("ready node IDs = %v, want %v", ready, want)
	}
}

func TestReadyNodeIDsAvoidsBlockedParentChain(t *testing.T) {
	for _, status := range []Status{StatusFailed, StatusBlocked, StatusSkipped} {
		t.Run(string(status), func(t *testing.T) {
			tree := NewTaskTree("tree-parent-block", "parent status blocks children", schedulerNow())
			tree.RootIDs = []string{"parent"}
			tree.Nodes["parent"] = schedulerNode("parent", status)
			tree.Nodes["parent"].Children = []string{"child"}
			tree.Nodes["child"] = schedulerChild("child", "parent", StatusPending)

			ready, err := ReadyNodeIDs(tree)
			if err != nil {
				t.Fatalf("ReadyNodeIDs: %v", err)
			}
			if len(ready) != 0 {
				t.Fatalf("ready node IDs = %v, want none", ready)
			}
		})
	}
}

func TestReadyNodeIDsReportsInvalidSchedulerState(t *testing.T) {
	tree := sampleSchedulerTree()
	tree.Nodes["lint"].Status = Status("waiting")

	_, err := ReadyNodeIDs(tree)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("ReadyNodeIDs error = %v, want ErrInvalidStatus", err)
	}

	tree = sampleSchedulerTree()
	tree.Nodes["unit"].DependsOn = []string{"missing"}
	if _, err := ReadyNodeIDs(tree); err == nil {
		t.Fatal("missing dependency should return an error")
	}

	tree = sampleSchedulerTree()
	tree.Nodes["orphan"] = schedulerNode("orphan", StatusPending)
	if _, err := ReadyNodeIDs(tree); err == nil {
		t.Fatal("unreachable node should return an error")
	}
}

func sampleSchedulerTree() *TaskTree {
	tree := NewTaskTree("tree-scheduler", "schedule ready nodes", schedulerNow())
	tree.RootIDs = []string{"setup", "lint", "unit", "package"}
	tree.Nodes["setup"] = schedulerNode("setup", StatusRunning)
	tree.Nodes["lint"] = schedulerNode("lint", StatusPending)
	tree.Nodes["unit"] = schedulerNode("unit", StatusPending)
	tree.Nodes["unit"].DependsOn = []string{"setup"}
	tree.Nodes["package"] = schedulerNode("package", StatusPending)
	tree.Nodes["package"].DependsOn = []string{"unit"}
	return tree
}

func schedulerNode(id string, status Status) *TaskNode {
	return &TaskNode{
		ID:             id,
		Title:          id,
		Status:         status,
		ExpectedOutput: "result for " + id,
	}
}

func schedulerChild(id, parentID string, status Status) *TaskNode {
	node := schedulerNode(id, status)
	node.ParentID = parentID
	return node
}

func schedulerNow() time.Time {
	return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
}
