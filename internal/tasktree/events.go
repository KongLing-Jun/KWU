package tasktree

import (
	"sort"
	"strings"

	"kwu/internal/event"
)

func (o *Orchestrator) emitTreeCreated(tree *TaskTree) {
	o.emitTaskTree(event.TaskTreeCreated, taskTreePayload(tree))
}

func (o *Orchestrator) emitTreeSummary(tree *TaskTree, summary string) {
	payload := taskTreePayload(tree)
	if payload == nil {
		return
	}
	payload.Summary = strings.TrimSpace(summary)
	o.emitTaskTree(event.TaskTreeSummary, payload)
}

func (o *Orchestrator) emitTreeCompleted(tree *TaskTree) {
	o.emitTaskTree(event.TaskTreeCompleted, taskTreePayload(tree))
}

func (o *Orchestrator) emitNodeStarted(tree *TaskTree, nodeID string) {
	o.emitTaskTree(event.TaskNodeStarted, taskTreeNodePayload(tree, nodeID))
}

func (o *Orchestrator) emitNodeOutput(tree *TaskTree, nodeID, delta string) {
	if strings.TrimSpace(delta) == "" {
		return
	}
	payload := taskTreeNodePayload(tree, nodeID)
	if payload == nil {
		return
	}
	payload.OutputDelta = delta
	o.emitTaskTree(event.TaskNodeOutput, payload)
}

func (o *Orchestrator) emitNodeTerminal(tree *TaskTree, nodeID string, result NodeResult) {
	payload := taskTreeNodePayload(tree, nodeID)
	if payload == nil {
		return
	}
	payload.Result = result.UserVisibleSummary()
	status := result.targetStatus()
	kind := event.TaskNodeCompleted
	if status != StatusCompleted {
		kind = event.TaskNodeFailed
		payload.Error = result.Error
		if payload.Error == "" {
			payload.Error = result.Blocker
		}
	}
	o.emitTaskTree(kind, payload)
}

func (o *Orchestrator) emitTaskTree(kind event.Kind, payload *event.TaskTreePayload) {
	if o == nil || o.sink == nil || payload == nil {
		return
	}
	o.sink.Emit(event.Event{
		Kind:     kind,
		TaskTree: payload,
		Source:   SnapshotModeTaskTree,
	})
}

func taskTreePayload(tree *TaskTree) *event.TaskTreePayload {
	if tree == nil {
		return nil
	}
	payload := &event.TaskTreePayload{
		TreeID:  tree.ID,
		Status:  string(tree.Status),
		Title:   tree.Goal,
		RootIDs: append([]string(nil), tree.RootIDs...),
	}
	for _, id := range orderedNodeIDsForEvent(tree) {
		if node, ok := tree.Node(id); ok {
			payload.Nodes = append(payload.Nodes, taskTreeNodeListPayload(node))
		}
	}
	return payload
}

func taskTreeNodePayload(tree *TaskTree, nodeID string) *event.TaskTreePayload {
	if tree == nil {
		return nil
	}
	node, ok := tree.Node(nodeID)
	if !ok {
		return nil
	}
	return &event.TaskTreePayload{
		TreeID:   tree.ID,
		NodeID:   node.ID,
		Status:   string(node.Status),
		Title:    node.Title,
		ParentID: node.ParentID,
	}
}

func taskTreeNodeListPayload(node *TaskNode) event.TaskTreeNodePayload {
	return event.TaskTreeNodePayload{
		NodeID:    node.ID,
		Status:    string(node.Status),
		Title:     node.Title,
		ParentID:  node.ParentID,
		DependsOn: append([]string(nil), node.DependsOn...),
	}
}

func orderedNodeIDsForEvent(tree *TaskTree) []string {
	ids, err := tree.OrderedNodeIDs()
	if err == nil {
		return ids
	}
	ids = make([]string, 0, len(tree.Nodes))
	for id := range tree.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
