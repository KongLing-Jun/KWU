package tasktree

import (
	"reflect"
	"testing"
	"time"
)

func TestNewTaskTreeUsesStableIDAndInitialState(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

	tree := NewTaskTree("tree-1", "ship task tree mode", now)

	if tree.ID != "tree-1" {
		t.Fatalf("tree ID = %q, want stable caller-supplied ID", tree.ID)
	}
	if tree.Goal != "ship task tree mode" {
		t.Fatalf("tree goal = %q", tree.Goal)
	}
	if tree.Status != StatusPending {
		t.Fatalf("tree status = %q, want pending", tree.Status)
	}
	if !tree.CreatedAt.Equal(now) || !tree.UpdatedAt.Equal(now) {
		t.Fatalf("tree timestamps = %s/%s, want %s", tree.CreatedAt, tree.UpdatedAt, now)
	}
	if tree.Nodes == nil {
		t.Fatal("tree Nodes map should be initialized")
	}
}

func TestTaskTreeRepresentsNestedNodesInStableOrder(t *testing.T) {
	tree := sampleTree()

	roots, err := tree.RootNodes()
	if err != nil {
		t.Fatalf("RootNodes: %v", err)
	}
	if got := nodeIDs(roots); !reflect.DeepEqual(got, []string{"design", "verify"}) {
		t.Fatalf("root order = %v, want [design verify]", got)
	}

	children, err := tree.ChildNodes("design")
	if err != nil {
		t.Fatalf("ChildNodes: %v", err)
	}
	if got := nodeIDs(children); !reflect.DeepEqual(got, []string{"model", "events"}) {
		t.Fatalf("child order = %v, want [model events]", got)
	}

	events, ok := tree.Node("events")
	if !ok {
		t.Fatal("events node not found")
	}
	if events.ParentID != "design" {
		t.Fatalf("events parent = %q, want design", events.ParentID)
	}
}

func TestTaskNodeRepresentsDependenciesByID(t *testing.T) {
	tree := sampleTree()

	deps, err := tree.DependencyNodes("events")
	if err != nil {
		t.Fatalf("DependencyNodes: %v", err)
	}
	if got := nodeIDs(deps); !reflect.DeepEqual(got, []string{"model"}) {
		t.Fatalf("dependency order = %v, want [model]", got)
	}
	if tree.Nodes["events"].DependsOn[0] != "model" {
		t.Fatalf("raw dependency ID changed: %+v", tree.Nodes["events"].DependsOn)
	}
}

func TestTaskTreeLookupReportsMissingNodes(t *testing.T) {
	tree := sampleTree()

	if _, ok := tree.Node("missing"); ok {
		t.Fatal("missing node lookup returned ok")
	}
	tree.RootIDs = append(tree.RootIDs, "missing")
	if _, err := tree.RootNodes(); err == nil {
		t.Fatal("RootNodes should report missing node IDs")
	}
}

func TestStatusValidity(t *testing.T) {
	for _, status := range []Status{
		StatusPending,
		StatusReady,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
		StatusBlocked,
		StatusSkipped,
	} {
		if !status.Valid() {
			t.Fatalf("status %q should be valid", status)
		}
	}
	if Status("waiting").Valid() {
		t.Fatal("unknown status should be invalid")
	}
}

func sampleTree() *TaskTree {
	tree := NewTaskTree("tree-1", "implement task tree state", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.Status = StatusRunning
	tree.RootIDs = []string{"design", "verify"}
	tree.Nodes["design"] = &TaskNode{
		ID:          "design",
		Title:       "Design the contract",
		Description: "Define state shared by generation, scheduling, and UI",
		Children:    []string{"model", "events"},
		Status:      StatusRunning,
	}
	tree.Nodes["model"] = &TaskNode{
		ID:       "model",
		ParentID: "design",
		Title:    "Define TaskTree and TaskNode",
		Status:   StatusCompleted,
	}
	tree.Nodes["events"] = &TaskNode{
		ID:        "events",
		ParentID:  "design",
		Title:     "Map nodes to future events",
		DependsOn: []string{"model"},
		Status:    StatusPending,
	}
	tree.Nodes["verify"] = &TaskNode{
		ID:        "verify",
		Title:     "Verify the state contract",
		DependsOn: []string{"design"},
		Status:    StatusPending,
	}
	return tree
}

func nodeIDs(nodes []*TaskNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}
