package agentmode

import (
	"context"
	"testing"

	"kwu/internal/tasktree"
)

type testRunner struct{}

func (*testRunner) Run(context.Context, string) error { return nil }

func TestBuildRunnerNormalReturnsExistingRunner(t *testing.T) {
	normal := &testRunner{}

	got, err := BuildRunner(ModeNormal, RunnerDeps{Normal: normal})
	if err != nil {
		t.Fatalf("BuildRunner normal returned error: %v", err)
	}
	if got == nil {
		t.Fatal("BuildRunner normal returned nil runner")
	}
	if got != normal {
		t.Fatalf("BuildRunner normal returned %T, want existing runner", got)
	}
}

func TestBuildRunnerTaskTreeReturnsOrchestrator(t *testing.T) {
	normal := &testRunner{}
	store := tasktree.NewMemoryStateStore()
	planner := tasktree.PlannerFunc(func(context.Context, string) (*tasktree.TaskTree, error) {
		return nil, tasktree.ErrNotImplemented
	})

	got, err := BuildRunner(ModeTaskTree, RunnerDeps{Normal: normal, TaskTreePlanner: planner, TaskTreeStore: store})
	if err != nil {
		t.Fatalf("BuildRunner task_tree returned error: %v", err)
	}
	orch, ok := got.(*tasktree.Orchestrator)
	if !ok {
		t.Fatalf("BuildRunner task_tree returned %T, want *tasktree.Orchestrator", got)
	}
	if orch.BaseRunner() != normal {
		t.Fatalf("task_tree base runner = %T, want existing normal runner", orch.BaseRunner())
	}
	if orch.Planner() == nil {
		t.Fatal("task_tree planner should be injected")
	}
	if orch.StateStore() != store {
		t.Fatalf("task_tree store = %T, want injected store", orch.StateStore())
	}
}

func TestBuildRunnerRejectsUnsupportedMode(t *testing.T) {
	got, err := BuildRunner(Mode("workflow"), RunnerDeps{})
	if err == nil {
		t.Fatalf("BuildRunner returned %T, want error", got)
	}
}
