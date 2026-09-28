package tasktree

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidStatus reports a status value outside the task-tree contract.
	ErrInvalidStatus = errors.New("invalid task status")
	// ErrInvalidTransition reports a disallowed status transition.
	ErrInvalidTransition = errors.New("invalid task status transition")
	// ErrDependenciesNotReady reports an attempt to ready a blocked node.
	ErrDependenciesNotReady = errors.New("task dependencies are not completed")
)

// Valid reports whether status is one of the first-version task-tree states.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusReady, StatusRunning, StatusCompleted, StatusFailed, StatusBlocked, StatusSkipped:
		return true
	default:
		return false
	}
}

// Terminal reports whether the status should no longer be advanced by the
// generic scheduler without a separate recovery policy.
func (s Status) Terminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusBlocked, StatusSkipped:
		return true
	default:
		return false
	}
}

// BlocksDependents reports whether this status prevents dependent nodes from
// becoming ready automatically.
func (s Status) BlocksDependents() bool {
	return s == StatusFailed || s == StatusBlocked
}

// CanTransition reports whether a node may move from one status to another.
func CanTransition(from, to Status) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	if from == to {
		return true
	}
	if from.Terminal() {
		return false
	}
	switch from {
	case StatusPending:
		return to == StatusReady || to == StatusBlocked || to == StatusSkipped
	case StatusReady:
		return to == StatusRunning || to == StatusBlocked || to == StatusSkipped
	case StatusRunning:
		return to == StatusCompleted || to == StatusFailed || to == StatusBlocked
	default:
		return false
	}
}

// TransitionTo validates and applies a node status transition.
func (n *TaskNode) TransitionTo(next Status) error {
	if n == nil {
		return fmt.Errorf("task node is nil")
	}
	if !n.Status.Valid() {
		return fmt.Errorf("%w: current %q", ErrInvalidStatus, n.Status)
	}
	if !next.Valid() {
		return fmt.Errorf("%w: next %q", ErrInvalidStatus, next)
	}
	if !CanTransition(n.Status, next) {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, n.Status, next)
	}
	n.Status = next
	return nil
}

// TransitionNode validates dependency gates and applies a node status transition.
func (t *TaskTree) TransitionNode(nodeID string, next Status) error {
	node, ok := t.Node(nodeID)
	if !ok {
		return fmt.Errorf("task node %q not found", nodeID)
	}
	if !CanTransition(node.Status, next) {
		return node.TransitionTo(next)
	}
	if node.Status != next && next == StatusReady {
		ready, err := t.CanEnterReady(nodeID)
		if err != nil {
			return err
		}
		if !ready {
			return fmt.Errorf("%w: node %q", ErrDependenciesNotReady, nodeID)
		}
	}
	return node.TransitionTo(next)
}

// DependenciesCompleted reports whether every dependency of nodeID completed.
func (t *TaskTree) DependenciesCompleted(nodeID string) (bool, error) {
	node, ok := t.Node(nodeID)
	if !ok {
		return false, fmt.Errorf("task node %q not found", nodeID)
	}
	for _, depID := range node.DependsOn {
		dep, ok := t.Node(depID)
		if !ok {
			return false, fmt.Errorf("task dependency %q for node %q not found", depID, nodeID)
		}
		if !dep.Status.Valid() {
			return false, fmt.Errorf("%w: dependency %q has %q", ErrInvalidStatus, depID, dep.Status)
		}
		if dep.Status != StatusCompleted {
			return false, nil
		}
	}
	return true, nil
}

// CanEnterReady reports whether a pending node's dependencies allow it to
// become ready.
func (t *TaskTree) CanEnterReady(nodeID string) (bool, error) {
	node, ok := t.Node(nodeID)
	if !ok {
		return false, fmt.Errorf("task node %q not found", nodeID)
	}
	if !node.Status.Valid() {
		return false, fmt.Errorf("%w: node %q has %q", ErrInvalidStatus, nodeID, node.Status)
	}
	if node.Status != StatusPending {
		return false, nil
	}
	return t.DependenciesCompleted(nodeID)
}
