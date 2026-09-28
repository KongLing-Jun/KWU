package tasktree

import (
	"errors"
	"testing"
)

func TestDependenciesCompletedRequiresAllDependenciesCompleted(t *testing.T) {
	tree := sampleTree()

	completed, err := tree.DependenciesCompleted("events")
	if err != nil {
		t.Fatalf("DependenciesCompleted(events): %v", err)
	}
	if !completed {
		t.Fatal("events should have all dependencies completed")
	}

	completed, err = tree.DependenciesCompleted("verify")
	if err != nil {
		t.Fatalf("DependenciesCompleted(verify): %v", err)
	}
	if completed {
		t.Fatal("verify should wait for design to complete")
	}

	tree.Nodes["design"].Status = StatusCompleted
	tree.Nodes["verify"].DependsOn = append(tree.Nodes["verify"].DependsOn, "missing")
	if _, err := tree.DependenciesCompleted("verify"); err == nil {
		t.Fatal("missing dependency should return an error")
	}
}

func TestCanEnterReadyRequiresPendingNodeAndCompletedDependencies(t *testing.T) {
	tree := sampleTree()

	ready, err := tree.CanEnterReady("events")
	if err != nil {
		t.Fatalf("CanEnterReady(events): %v", err)
	}
	if !ready {
		t.Fatal("pending node with completed dependencies should enter ready")
	}

	ready, err = tree.CanEnterReady("model")
	if err != nil {
		t.Fatalf("CanEnterReady(model): %v", err)
	}
	if ready {
		t.Fatal("completed node should not re-enter ready")
	}

	ready, err = tree.CanEnterReady("verify")
	if err != nil {
		t.Fatalf("CanEnterReady(verify): %v", err)
	}
	if ready {
		t.Fatal("node with running dependency should not enter ready")
	}
}

func TestCanEnterReadyStaysFalseWhenDependencyFailedOrBlocked(t *testing.T) {
	for _, depStatus := range []Status{StatusFailed, StatusBlocked} {
		tree := sampleTree()
		tree.Nodes["model"].Status = depStatus

		ready, err := tree.CanEnterReady("events")
		if err != nil {
			t.Fatalf("CanEnterReady(events) with dependency %s: %v", depStatus, err)
		}
		if ready {
			t.Fatalf("node should not enter ready when dependency is %s", depStatus)
		}
		if !depStatus.BlocksDependents() {
			t.Fatalf("%s should block dependent nodes", depStatus)
		}
	}
}

func TestTransitionToAllowsCoreLifecycle(t *testing.T) {
	node := &TaskNode{ID: "n1", Status: StatusPending}

	for _, next := range []Status{StatusReady, StatusRunning, StatusCompleted} {
		if err := node.TransitionTo(next); err != nil {
			t.Fatalf("TransitionTo(%s): %v", next, err)
		}
		if node.Status != next {
			t.Fatalf("node status = %s, want %s", node.Status, next)
		}
	}

	if err := node.TransitionTo(StatusCompleted); err != nil {
		t.Fatalf("terminal self-transition should be allowed: %v", err)
	}
}

func TestTransitionNodeRequiresCompletedDependenciesBeforeReady(t *testing.T) {
	tree := sampleTree()

	err := tree.TransitionNode("verify", StatusReady)
	if !errors.Is(err, ErrDependenciesNotReady) {
		t.Fatalf("verify -> ready error = %v, want ErrDependenciesNotReady", err)
	}
	if tree.Nodes["verify"].Status != StatusPending {
		t.Fatalf("failed ready transition changed status to %s", tree.Nodes["verify"].Status)
	}

	tree.Nodes["design"].Status = StatusCompleted
	if err := tree.TransitionNode("verify", StatusReady); err != nil {
		t.Fatalf("verify -> ready after dependency completion: %v", err)
	}
	if tree.Nodes["verify"].Status != StatusReady {
		t.Fatalf("verify status = %s, want ready", tree.Nodes["verify"].Status)
	}

	tree.Nodes["verify"].Status = StatusCompleted
	err = tree.TransitionNode("verify", StatusReady)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("completed -> ready error = %v, want ErrInvalidTransition", err)
	}
}

func TestTransitionToRejectsIllegalOrUnknownStatus(t *testing.T) {
	node := &TaskNode{ID: "n1", Status: StatusCompleted}

	err := node.TransitionTo(StatusRunning)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("completed -> running error = %v, want ErrInvalidTransition", err)
	}
	if node.Status != StatusCompleted {
		t.Fatalf("illegal transition changed status to %s", node.Status)
	}

	node.Status = Status("waiting")
	err = node.TransitionTo(StatusReady)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("unknown current status error = %v, want ErrInvalidStatus", err)
	}

	node.Status = StatusPending
	err = node.TransitionTo(Status("waiting"))
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("unknown next status error = %v, want ErrInvalidStatus", err)
	}
}

func TestStatusTerminalSet(t *testing.T) {
	for _, status := range []Status{StatusCompleted, StatusFailed, StatusBlocked, StatusSkipped} {
		if !status.Terminal() {
			t.Fatalf("%s should be terminal", status)
		}
	}
	for _, status := range []Status{StatusPending, StatusReady, StatusRunning} {
		if status.Terminal() {
			t.Fatalf("%s should not be terminal", status)
		}
	}
}
