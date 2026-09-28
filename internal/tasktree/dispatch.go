package tasktree

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/tool"
)

var subagentRefPattern = regexp.MustCompile(`(?m)^Subagent reference(?: \(failed\))?:\s*(\S+)\s*$`)

type routedNodeDispatcher struct {
	main     NodeDispatcher
	registry *tool.Registry
	sink     event.Sink
}

func newRoutedNodeDispatcher(executor Executor, registry *tool.Registry, sink event.Sink) NodeDispatcher {
	return routedNodeDispatcher{
		main:     executorNodeDispatcher{executor: executor},
		registry: registry,
		sink:     sink,
	}
}

func (d routedNodeDispatcher) DispatchNode(ctx context.Context, tree *TaskTree, nodeID string, nodeContext NodeContext) (NodeResult, error) {
	route := d.NodeDispatchRoute(tree, nodeID)
	switch route {
	case "read_only_task", "task":
		return d.dispatchTool(ctx, tree, nodeID, nodeContext, route)
	default:
		return d.main.DispatchNode(ctx, tree, nodeID, nodeContext)
	}
}

func (d routedNodeDispatcher) NodeDispatchRoute(tree *TaskTree, nodeID string) string {
	node, ok := tree.Node(nodeID)
	if !ok || node.Executor == nil {
		return "main"
	}
	switch node.Executor.Kind {
	case ExecutorReadOnlyTask:
		if d.hasTool("read_only_task") {
			return "read_only_task"
		}
	case ExecutorTask, ExecutorDedicatedAgent, ExecutorParallelTasks:
		if d.hasTool("task") {
			return "task"
		}
	case ExecutorMain:
		return "main"
	}
	return "main"
}

func (d routedNodeDispatcher) hasTool(name string) bool {
	if d.registry == nil {
		return false
	}
	_, ok := d.registry.Get(name)
	return ok
}

func (d routedNodeDispatcher) dispatchTool(ctx context.Context, tree *TaskTree, nodeID string, nodeContext NodeContext, toolName string) (NodeResult, error) {
	tl, ok := d.registry.Get(toolName)
	if !ok {
		return NodeResult{}, fmt.Errorf("%w: %s", ErrNodeDispatcherUnavailable, toolName)
	}
	args, err := renderSubagentToolArgs(tree, nodeID, nodeContext)
	if err != nil {
		return NodeResult{Summary: "Node execution failed.", Error: err.Error()}, err
	}
	output, err := tl.Execute(d.nodeDispatchContext(ctx, tree, nodeID), args)
	if err != nil {
		result := WrapNodeOutput(output)
		if result.Summary == "" {
			result.Summary = "Node execution failed."
		}
		result.Error = err.Error()
		return result, err
	}
	result := WrapNodeOutput(output)
	if result.Summary == "" {
		result.Summary = "Node execution completed."
	}
	return result, nil
}

func (d routedNodeDispatcher) nodeDispatchContext(ctx context.Context, tree *TaskTree, nodeID string) context.Context {
	if tree == nil {
		return ctx
	}
	ctx = event.WithTaskTreeContext(ctx, tree.ID, nodeID)
	if d.sink != nil {
		ctx = agent.WithCallContext(ctx, taskTreeCallID(tree.ID, nodeID), d.sink, nil, false)
	}
	return ctx
}

func taskTreeCallID(treeID, nodeID string) string {
	treeID = strings.TrimSpace(treeID)
	nodeID = strings.TrimSpace(nodeID)
	if treeID == "" {
		treeID = "tree"
	}
	if nodeID == "" {
		nodeID = "node"
	}
	return "task_tree/" + treeID + "/" + nodeID
}

func renderSubagentToolArgs(tree *TaskTree, nodeID string, nodeContext NodeContext) (json.RawMessage, error) {
	node, ok := tree.Node(nodeID)
	if !ok {
		return nil, fmt.Errorf("task node %q not found", nodeID)
	}
	description := node.Title
	if strings.TrimSpace(description) == "" {
		description = node.ID
	}
	args := struct {
		Prompt      string `json:"prompt"`
		Description string `json:"description,omitempty"`
	}{
		Prompt:      RenderNodeExecutionPrompt(nodeContext),
		Description: description,
	}
	return json.Marshal(args)
}

// WrapNodeOutput converts a main/sub-agent final answer into NodeResult. It
// keeps transcript references out of the summary while preserving them in
// RawOutputRef for future continuation or inspection.
func WrapNodeOutput(output string) NodeResult {
	output = strings.TrimSpace(output)
	ref := extractSubagentRef(output)
	summary := stripSubagentEnvelope(output)
	result := NodeResult{Summary: summary}
	if ref != "" {
		result.RawOutputRef = "subagent://" + ref
	}
	return result
}

func extractSubagentRef(output string) string {
	match := subagentRefPattern.FindStringSubmatch(output)
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func stripSubagentEnvelope(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	if _, after, ok := strings.Cut(output, "\n\nFinal answer:\n"); ok {
		return strings.TrimSpace(after)
	}
	if _, after, ok := strings.Cut(output, "Final answer:\n"); ok {
		return strings.TrimSpace(after)
	}
	return output
}

type nodeDispatchRouter interface {
	NodeDispatchRoute(tree *TaskTree, nodeID string) string
}
