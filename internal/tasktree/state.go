package tasktree

import (
	"fmt"
	"time"
)

// Status is the shared state label for a task tree or task node.
type Status string

const (
	StatusPending   Status = "pending"
	StatusReady     Status = "ready"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusBlocked   Status = "blocked"
	StatusSkipped   Status = "skipped"
)

// TaskTree is the structured state shared by task-tree generation, scheduling,
// event rendering, and future session restore.
type TaskTree struct {
	ID        string               `json:"id"`
	Goal      string               `json:"goal"`
	RootIDs   []string             `json:"root_ids"`
	Nodes     map[string]*TaskNode `json:"nodes"`
	Status    Status               `json:"status"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// TaskNode is one stable, addressable unit in a TaskTree.
type TaskNode struct {
	ID          string   `json:"id"`
	ParentID    string   `json:"parent_id,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Children    []string `json:"children,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Status      Status   `json:"status"`

	Executor       *ExecutorSpec `json:"executor,omitempty"`
	Constraints    []string      `json:"constraints,omitempty"`
	RelatedFiles   []string      `json:"related_files,omitempty"`
	ToolHints      []string      `json:"tool_hints,omitempty"`
	ExpectedOutput string        `json:"expected_output,omitempty"`
	Result         *NodeResult   `json:"result,omitempty"`
}

// NewTaskTree constructs an empty tree with a caller-supplied stable ID.
func NewTaskTree(id, goal string, now time.Time) *TaskTree {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &TaskTree{
		ID:        id,
		Goal:      goal,
		Nodes:     map[string]*TaskNode{},
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Node returns the node for id without changing tree state.
func (t *TaskTree) Node(id string) (*TaskNode, bool) {
	if t == nil || id == "" {
		return nil, false
	}
	node, ok := t.Nodes[id]
	if !ok || node == nil {
		return nil, false
	}
	return node, true
}

// RootNodes returns root nodes in the stable display order declared by RootIDs.
func (t *TaskTree) RootNodes() ([]*TaskNode, error) {
	if t == nil {
		return nil, fmt.Errorf("task tree is nil")
	}
	return t.nodesInOrder(t.RootIDs)
}

// ChildNodes returns a node's children in the stable display order declared by
// TaskNode.Children.
func (t *TaskTree) ChildNodes(parentID string) ([]*TaskNode, error) {
	parent, ok := t.Node(parentID)
	if !ok {
		return nil, fmt.Errorf("task node %q not found", parentID)
	}
	return t.nodesInOrder(parent.Children)
}

// DependencyNodes returns a node's dependencies in the order declared by
// TaskNode.DependsOn.
func (t *TaskTree) DependencyNodes(nodeID string) ([]*TaskNode, error) {
	node, ok := t.Node(nodeID)
	if !ok {
		return nil, fmt.Errorf("task node %q not found", nodeID)
	}
	return t.nodesInOrder(node.DependsOn)
}

func (t *TaskTree) nodesInOrder(ids []string) ([]*TaskNode, error) {
	out := make([]*TaskNode, 0, len(ids))
	for _, id := range ids {
		node, ok := t.Node(id)
		if !ok {
			return nil, fmt.Errorf("task node %q not found", id)
		}
		out = append(out, node)
	}
	return out, nil
}
