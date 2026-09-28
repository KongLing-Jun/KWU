package tasktree

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBuildNodeContextIncludesGoalPathConstraintsAndHints(t *testing.T) {
	tree := sampleExecutionTree()

	ctx, err := tree.BuildNodeContext("patch")
	if err != nil {
		t.Fatalf("BuildNodeContext: %v", err)
	}

	if ctx.Goal != tree.Goal {
		t.Fatalf("context goal = %q, want %q", ctx.Goal, tree.Goal)
	}
	if ctx.NodeID != "patch" || ctx.NodeTitle != "Patch implementation" {
		t.Fatalf("context node identity = %s/%s", ctx.NodeID, ctx.NodeTitle)
	}
	if ctx.NodeDescription != "Apply the context contract" {
		t.Fatalf("context node description = %q", ctx.NodeDescription)
	}
	if got := ctx.NodePath; !reflect.DeepEqual(got, []string{"Implement task tree context", "Patch implementation"}) {
		t.Fatalf("node path = %v", got)
	}
	if got := ctx.ParentConstraints; !reflect.DeepEqual(got, []string{"Keep changes inside internal/tasktree"}) {
		t.Fatalf("parent constraints = %v", got)
	}
	if got := ctx.RelatedFiles; !reflect.DeepEqual(got, []string{"internal/tasktree/context.go"}) {
		t.Fatalf("related files = %v", got)
	}
	if got := ctx.ToolHints; !reflect.DeepEqual(got, []string{"go test ./internal/tasktree"}) {
		t.Fatalf("tool hints = %v", got)
	}
	if ctx.ExpectedOutput != "Go structs and tests" {
		t.Fatalf("expected output = %q", ctx.ExpectedOutput)
	}

	ctx.ParentConstraints[0] = "mutated"
	ctx.RelatedFiles[0] = "mutated"
	ctx.ToolHints[0] = "mutated"
	if tree.Nodes["root"].Constraints[0] != "Keep changes inside internal/tasktree" {
		t.Fatal("context should not expose mutable parent constraints")
	}
	if tree.Nodes["patch"].RelatedFiles[0] != "internal/tasktree/context.go" {
		t.Fatal("context should not expose mutable related files")
	}
	if tree.Nodes["patch"].ToolHints[0] != "go test ./internal/tasktree" {
		t.Fatal("context should not expose mutable tool hints")
	}
}

func TestBuildNodeContextIncludesCompletedDependencyResults(t *testing.T) {
	tree := sampleExecutionTree()

	ctx, err := tree.BuildNodeContext("patch")
	if err != nil {
		t.Fatalf("BuildNodeContext: %v", err)
	}
	if len(ctx.DependencyResults) != 1 {
		t.Fatalf("dependency result count = %d, want 1", len(ctx.DependencyResults))
	}
	dep := ctx.DependencyResults[0]
	if dep.NodeID != "inspect" || dep.Summary != "Found task and parallel task contracts" {
		t.Fatalf("dependency result = %+v", dep)
	}
	if dep.RawOutputRef != "subagent://inspect/raw" {
		t.Fatalf("raw output ref = %q", dep.RawOutputRef)
	}
	if dep.Artifacts[0].Data["files"] != float64(2) {
		t.Fatalf("artifact data was not preserved: %+v", dep.Artifacts)
	}

	ctx.DependencyResults[0].Artifacts[0].Data["files"] = float64(9)
	if tree.Nodes["inspect"].Result.Artifacts[0].Data["files"] != float64(2) {
		t.Fatal("context should not expose mutable artifact maps from node results")
	}
}

func TestAssignExecutorMarksNodeWithValidatedExecutor(t *testing.T) {
	node := &TaskNode{ID: "patch"}

	for _, kind := range []ExecutorKind{ExecutorMain, ExecutorTask, ExecutorReadOnlyTask, ExecutorParallelTasks} {
		if err := node.AssignExecutor(ExecutorSpec{Kind: kind}); err != nil {
			t.Fatalf("AssignExecutor(%s): %v", kind, err)
		}
		if node.Executor == nil || node.Executor.Kind != kind {
			t.Fatalf("node executor = %+v, want %s", node.Executor, kind)
		}
	}

	if err := node.AssignExecutor(ExecutorSpec{Kind: ExecutorDedicatedAgent, Agent: "reviewer"}); err != nil {
		t.Fatalf("AssignExecutor(dedicated): %v", err)
	}
	if node.Executor.Agent != "reviewer" {
		t.Fatalf("dedicated agent = %q", node.Executor.Agent)
	}

	if err := node.AssignExecutor(ExecutorSpec{Kind: ExecutorDedicatedAgent}); err == nil {
		t.Fatal("dedicated agent without name should be rejected")
	}
	if err := node.AssignExecutor(ExecutorSpec{Kind: ExecutorKind("unknown")}); err == nil {
		t.Fatal("unknown executor kind should be rejected")
	}
}

func TestRecordResultWritesSummaryArtifactsRawRefAndStatus(t *testing.T) {
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Status = StatusRunning

	result := NodeResult{
		Summary:      "Implemented context and result structs",
		RawOutputRef: "subagent://patch/raw",
		Artifacts: []NodeArtifact{{
			Type: "patch",
			Name: "tasktree context",
			Ref:  "git://worktree/internal/tasktree/context.go",
			Data: map[string]any{"tests": float64(3)},
		}},
	}
	if err := tree.RecordResult("patch", result); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	node := tree.Nodes["patch"]
	if node.Status != StatusCompleted {
		t.Fatalf("node status = %s, want completed", node.Status)
	}
	if node.Result == nil {
		t.Fatal("node result was not written")
	}
	if node.Result.UserVisibleSummary() != result.Summary {
		t.Fatalf("visible summary = %q", node.Result.UserVisibleSummary())
	}
	if node.Result.RawOutputRef != result.RawOutputRef {
		t.Fatalf("raw output ref = %q", node.Result.RawOutputRef)
	}

	result.Artifacts[0].Data["tests"] = float64(99)
	if node.Result.Artifacts[0].Data["tests"] != float64(3) {
		t.Fatal("RecordResult should copy structured artifact maps")
	}
}

func TestRecordResultWritesErrorsAndBlockers(t *testing.T) {
	tests := []struct {
		name   string
		result NodeResult
		status Status
	}{
		{
			name:   "error",
			result: NodeResult{Summary: "failed", Error: "test failure"},
			status: StatusFailed,
		},
		{
			name:   "blocker",
			result: NodeResult{Summary: "blocked", Blocker: "missing credentials"},
			status: StatusBlocked,
		},
		{
			name:   "blocker wins over error",
			result: NodeResult{Summary: "blocked", Error: "also failed", Blocker: "needs user decision"},
			status: StatusBlocked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := sampleExecutionTree()
			tree.Nodes["patch"].Status = StatusRunning
			if err := tree.RecordResult("patch", tt.result); err != nil {
				t.Fatalf("RecordResult: %v", err)
			}
			if tree.Nodes["patch"].Status != tt.status {
				t.Fatalf("status = %s, want %s", tree.Nodes["patch"].Status, tt.status)
			}
			if tree.Nodes["patch"].Result == nil || tree.Nodes["patch"].Result.Summary != tt.result.Summary {
				t.Fatalf("result not written: %+v", tree.Nodes["patch"].Result)
			}
		})
	}
}

func TestRecordResultRejectsInvalidTransitionWithoutMutatingNode(t *testing.T) {
	tree := sampleExecutionTree()
	tree.Nodes["patch"].Status = StatusCompleted

	err := tree.RecordResult("patch", NodeResult{Summary: "try again", Error: "late failure"})
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("RecordResult error = %v, want ErrInvalidTransition", err)
	}
	if tree.Nodes["patch"].Result != nil {
		t.Fatal("invalid result transition should not mutate node result")
	}
}

func sampleExecutionTree() *TaskTree {
	tree := NewTaskTree("tree-ctx", "Add task tree execution context", sampleTime())
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = &TaskNode{
		ID:          "root",
		Title:       "Implement task tree context",
		Description: "Define context, executor, and result structures",
		Children:    []string{"inspect", "patch"},
		Constraints: []string{"Keep changes inside internal/tasktree"},
		Status:      StatusRunning,
	}
	tree.Nodes["inspect"] = &TaskNode{
		ID:       "inspect",
		ParentID: "root",
		Title:    "Inspect sub-agent contracts",
		Status:   StatusCompleted,
		Result: &NodeResult{
			Summary:      "Found task and parallel task contracts",
			RawOutputRef: "subagent://inspect/raw",
			Artifacts: []NodeArtifact{{
				Type: "analysis",
				Name: "agent contracts",
				Data: map[string]any{"files": float64(2)},
			}},
		},
	}
	tree.Nodes["patch"] = &TaskNode{
		ID:             "patch",
		ParentID:       "root",
		Title:          "Patch implementation",
		Description:    "Apply the context contract",
		DependsOn:      []string{"inspect"},
		RelatedFiles:   []string{"internal/tasktree/context.go"},
		ToolHints:      []string{"go test ./internal/tasktree"},
		ExpectedOutput: "Go structs and tests",
		Status:         StatusPending,
	}
	return tree
}

func sampleTime() time.Time {
	return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
}
