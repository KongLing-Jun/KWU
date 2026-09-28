package tasktree

import (
	"fmt"
)

// OrderedNodeIDs returns the tree's stable pre-order traversal. The order comes
// only from root_ids and children arrays, never from map iteration.
func (t *TaskTree) OrderedNodeIDs() ([]string, error) {
	if t == nil {
		return nil, fmt.Errorf("task tree is nil")
	}
	visited := map[string]bool{}
	visiting := map[string]bool{}
	out := make([]string, 0, len(t.Nodes))

	var walk func(string) error
	walk = func(id string) error {
		if id == "" {
			return fmt.Errorf("task node id is empty")
		}
		node, ok := t.Node(id)
		if !ok {
			return fmt.Errorf("task node %q not found", id)
		}
		if visiting[id] {
			return fmt.Errorf("task node hierarchy cycle at %q", id)
		}
		if visited[id] {
			return fmt.Errorf("task node %q appears multiple times in hierarchy", id)
		}

		visiting[id] = true
		out = append(out, id)
		for _, childID := range node.Children {
			if err := walk(childID); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}

	for _, rootID := range t.RootIDs {
		if err := walk(rootID); err != nil {
			return nil, err
		}
	}
	for id := range t.Nodes {
		if !visited[id] {
			return nil, fmt.Errorf("task node %q is not reachable from root_ids", id)
		}
	}
	return out, nil
}

// ParentAllowsRun reports whether nodeID's ancestor chain permits scheduling.
// Failed, blocked, and skipped ancestors suppress their descendants.
func (t *TaskTree) ParentAllowsRun(nodeID string) (bool, error) {
	node, ok := t.Node(nodeID)
	if !ok {
		return false, fmt.Errorf("task node %q not found", nodeID)
	}
	seen := map[string]bool{node.ID: true}
	for parentID := node.ParentID; parentID != ""; {
		if seen[parentID] {
			return false, fmt.Errorf("task node parent cycle at %q", parentID)
		}
		seen[parentID] = true
		parent, ok := t.Node(parentID)
		if !ok {
			return false, fmt.Errorf("task parent %q for node %q not found", parentID, node.ID)
		}
		if !parent.Status.Valid() {
			return false, fmt.Errorf("%w: parent %q has %q", ErrInvalidStatus, parent.ID, parent.Status)
		}
		if parent.Status == StatusFailed || parent.Status == StatusBlocked || parent.Status == StatusSkipped {
			return false, nil
		}
		parentID = parent.ParentID
	}
	return true, nil
}

// NodeReadyToRun reports whether a node can be selected by the scheduler. It
// does not mutate status; callers decide whether to mark pending nodes as ready.
func (t *TaskTree) NodeReadyToRun(nodeID string) (bool, error) {
	node, ok := t.Node(nodeID)
	if !ok {
		return false, fmt.Errorf("task node %q not found", nodeID)
	}
	if !node.Status.Valid() {
		return false, fmt.Errorf("%w: node %q has %q", ErrInvalidStatus, node.ID, node.Status)
	}
	if node.Status != StatusPending && node.Status != StatusReady {
		return false, nil
	}
	parentAllowed, err := t.ParentAllowsRun(nodeID)
	if err != nil || !parentAllowed {
		return parentAllowed, err
	}
	return t.DependenciesCompleted(nodeID)
}

// ReadyNodeIDs returns every currently runnable node in stable tree order.
func ReadyNodeIDs(tree *TaskTree) ([]string, error) {
	if tree == nil {
		return nil, fmt.Errorf("task tree is nil")
	}
	ordered, err := tree.OrderedNodeIDs()
	if err != nil {
		return nil, err
	}
	ready := make([]string, 0, len(ordered))
	for _, id := range ordered {
		ok, err := tree.NodeReadyToRun(id)
		if err != nil {
			return nil, err
		}
		if ok {
			ready = append(ready, id)
		}
	}
	return ready, nil
}

// ReadyBatchNodeIDs returns the next conservative parallel batch. It scans one
// sibling group at a time in tree order and returns the first non-empty group of
// ready nodes, so ancestors and descendants are not scheduled together.
func ReadyBatchNodeIDs(tree *TaskTree) ([]string, error) {
	if tree == nil {
		return nil, fmt.Errorf("task tree is nil")
	}
	if _, err := tree.OrderedNodeIDs(); err != nil {
		return nil, err
	}

	var scanGroup func([]string) ([]string, error)
	scanGroup = func(ids []string) ([]string, error) {
		batch := []string{}
		for _, id := range ids {
			ok, err := tree.NodeReadyToRun(id)
			if err != nil {
				return nil, err
			}
			if ok {
				batch = append(batch, id)
			}
		}
		if len(batch) > 0 {
			return batch, nil
		}
		for _, id := range ids {
			node, ok := tree.Node(id)
			if !ok {
				return nil, fmt.Errorf("task node %q not found", id)
			}
			if len(node.Children) == 0 {
				continue
			}
			batch, err := scanGroup(node.Children)
			if err != nil || len(batch) > 0 {
				return batch, err
			}
		}
		return nil, nil
	}

	return scanGroup(tree.RootIDs)
}

// NextReadyNodeID returns the first node from the conservative ready batch.
func NextReadyNodeID(tree *TaskTree) (string, bool, error) {
	batch, err := ReadyBatchNodeIDs(tree)
	if err != nil {
		return "", false, err
	}
	if len(batch) == 0 {
		return "", false, nil
	}
	return batch[0], true, nil
}
