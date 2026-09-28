package tasktree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kwu/internal/event"
)

var (
	// ErrNoTaskTreeState reports that serial execution was asked to run before a
	// task-tree snapshot exists.
	ErrNoTaskTreeState = errors.New("task_tree state is not available")
	// ErrNodeNotReady reports a request to execute a node the scheduler would not
	// currently select.
	ErrNodeNotReady = errors.New("task node is not ready to run")
	// ErrNodeDispatcherUnavailable reports that no execution boundary is wired.
	ErrNodeDispatcherUnavailable = errors.New("task node dispatcher is not configured")
)

// NodeDispatcher is the serial executor's callout boundary. Later phases can
// route through task/read_only_task/main executors without changing ExecuteOne.
type NodeDispatcher interface {
	DispatchNode(ctx context.Context, tree *TaskTree, nodeID string, nodeContext NodeContext) (NodeResult, error)
}

// NodeDispatcherFunc adapts a function to NodeDispatcher for tests.
type NodeDispatcherFunc func(context.Context, *TaskTree, string, NodeContext) (NodeResult, error)

func (f NodeDispatcherFunc) DispatchNode(ctx context.Context, tree *TaskTree, nodeID string, nodeContext NodeContext) (NodeResult, error) {
	return f(ctx, tree, nodeID, nodeContext)
}

type executorNodeDispatcher struct {
	executor Executor
}

func (d executorNodeDispatcher) DispatchNode(ctx context.Context, _ *TaskTree, _ string, nodeContext NodeContext) (NodeResult, error) {
	if d.executor == nil {
		return NodeResult{}, ErrNodeDispatcherUnavailable
	}
	if err := d.executor.Run(ctx, RenderNodeExecutionPrompt(nodeContext)); err != nil {
		return NodeResult{Summary: "Node execution failed.", Error: err.Error()}, err
	}
	return NodeResult{Summary: "Node execution completed."}, nil
}

// RenderNodeExecutionPrompt renders the local node context sent to the current
// execution boundary. It intentionally excludes the full conversation.
func RenderNodeExecutionPrompt(nodeContext NodeContext) string {
	b, err := json.MarshalIndent(nodeContext, "", "  ")
	if err != nil {
		return fmt.Sprintf("Execute task node %q.", nodeContext.NodeID)
	}
	return "Execute this task-tree node using only the provided local context.\n\n" + string(b)
}

// ExecuteNext selects and executes the next serially runnable node. ok is false
// when the tree has no currently ready node.
func (o *Orchestrator) ExecuteNext(ctx context.Context) (string, bool, error) {
	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return "", false, err
	}
	nodeID, ok, err := NextReadyNodeID(tree)
	if err != nil || !ok {
		return "", ok, err
	}
	if err := o.ExecuteOne(ctx, nodeID); err != nil {
		return nodeID, true, err
	}
	return nodeID, true, nil
}

// ExecuteOne runs exactly one scheduler-ready node, records its result, and
// persists the tree before and after dispatch.
func (o *Orchestrator) ExecuteOne(ctx context.Context, nodeID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if o == nil {
		return ErrNoTaskTreeState
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return fmt.Errorf("%w: node id is required", ErrNodeNotReady)
	}

	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return err
	}
	ready, err := tree.NodeReadyToRun(nodeID)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("%w: %q", ErrNodeNotReady, nodeID)
	}

	node, _ := tree.Node(nodeID)
	if node.Status == StatusPending {
		if err := tree.TransitionNode(nodeID, StatusReady); err != nil {
			return err
		}
	}
	if err := tree.TransitionNode(nodeID, StatusRunning); err != nil {
		return err
	}
	tree.UpdatedAt = time.Now().UTC()
	if err := o.saveTaskTree(ctx, tree); err != nil {
		return err
	}
	o.emitNodeStarted(tree, nodeID)
	o.emitNodePhase("started", nodeID)

	nodeContext, err := tree.BuildNodeContext(nodeID)
	if err != nil {
		result := NodeResult{Summary: "Node execution failed.", Error: err.Error()}
		return o.finishNodeAfterDispatchError(ctx, tree, nodeID, result, err)
	}

	result, dispatchErr := o.dispatchNode(ctx, tree, nodeID, nodeContext)
	if dispatchErr != nil {
		return o.finishNodeAfterDispatchError(ctx, tree, nodeID, result, dispatchErr)
	}
	if result.Summary == "" && result.Error == "" && result.Blocker == "" {
		result.Summary = "Node execution completed."
	}
	if err := tree.RecordResult(nodeID, result); err != nil {
		return err
	}
	tree.UpdatedAt = time.Now().UTC()
	if err := o.saveTaskTree(ctx, tree); err != nil {
		return err
	}
	o.emitNodeFinished(tree, nodeID, result)
	return nil
}

func (o *Orchestrator) dispatchNode(ctx context.Context, tree *TaskTree, nodeID string, nodeContext NodeContext) (NodeResult, error) {
	if o.dispatcher == nil {
		return NodeResult{}, ErrNodeDispatcherUnavailable
	}
	route := "custom"
	if router, ok := o.dispatcher.(nodeDispatchRouter); ok {
		route = router.NodeDispatchRoute(tree, nodeID)
	}
	o.emitNodePhase("dispatch "+route, nodeID)
	return o.dispatcher.DispatchNode(ctx, tree, nodeID, nodeContext)
}

func (o *Orchestrator) finishNodeAfterDispatchError(ctx context.Context, tree *TaskTree, nodeID string, result NodeResult, dispatchErr error) error {
	if result.Summary == "" {
		result.Summary = "Node execution failed."
	}
	result.Blocker = ""
	if result.Error == "" && dispatchErr != nil {
		result.Error = dispatchErr.Error()
	}
	if err := tree.RecordResult(nodeID, result); err != nil {
		return err
	}
	tree.UpdatedAt = time.Now().UTC()
	if err := o.saveTaskTree(ctx, tree); err != nil {
		return err
	}
	o.emitNodeFinished(tree, nodeID, result)
	if dispatchErr != nil {
		return dispatchErr
	}
	return nil
}

func (o *Orchestrator) loadTaskTree(ctx context.Context) (*TaskTree, error) {
	if o == nil || o.store == nil {
		return nil, ErrNoTaskTreeState
	}
	snapshot, ok, err := o.store.LoadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if !ok || snapshot.Tree == nil {
		return nil, ErrNoTaskTreeState
	}
	return snapshot.Tree, nil
}

func (o *Orchestrator) saveTaskTree(ctx context.Context, tree *TaskTree) error {
	return o.store.SaveSnapshot(ctx, Snapshot{
		Version: SnapshotVersion,
		Mode:    SnapshotModeTaskTree,
		Tree:    tree,
	})
}

func (o *Orchestrator) emitNodePhase(phase, nodeID string) {
	o.sink.Emit(event.Event{
		Kind:   event.Phase,
		Text:   fmt.Sprintf("task_tree: node_%s %s", phase, nodeID),
		Source: SnapshotModeTaskTree,
	})
}

func (o *Orchestrator) emitNodeFinished(tree *TaskTree, nodeID string, result NodeResult) {
	status := result.targetStatus()
	o.emitNodeTerminal(tree, nodeID, result)
	o.emitNodePhase(string(status), nodeID)
	if status == StatusFailed || status == StatusBlocked {
		text := result.Error
		if status == StatusBlocked {
			text = result.Blocker
		}
		o.sink.Emit(event.Event{
			Kind:   event.Notice,
			Level:  event.LevelWarn,
			Text:   fmt.Sprintf("task_tree node %s %s: %s", nodeID, status, text),
			Source: SnapshotModeTaskTree,
		})
	}
}
