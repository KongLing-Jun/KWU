package tasktree

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const DefaultMaxParallelNodes = 4

var (
	// ErrInvalidBatch reports malformed batch input before any node is started.
	ErrInvalidBatch = errors.New("invalid task node batch")
	// ErrBatchExecutionFailed reports that at least one node in a batch failed.
	ErrBatchExecutionFailed = errors.New("task node batch execution failed")
)

// BatchNodeResult keeps each node's final result attached to its original ID.
// Results are returned in the same order as the input ids.
type BatchNodeResult struct {
	NodeID string
	Status Status
	Result NodeResult
	Err    error
}

// BatchError summarizes a batch that finished with one or more failed nodes.
type BatchError struct {
	Results []BatchNodeResult
}

func (e *BatchError) Error() string {
	failed := 0
	for _, result := range e.Results {
		if result.Err != nil {
			failed++
		}
	}
	return fmt.Sprintf("%s: %d node(s) failed", ErrBatchExecutionFailed, failed)
}

func (e *BatchError) Unwrap() []error {
	errs := []error{ErrBatchExecutionFailed}
	for _, result := range e.Results {
		if result.Err != nil {
			errs = append(errs, result.Err)
		}
	}
	return errs
}

// ExecuteReadyBatch selects the next conservative ready batch and executes it.
// ok is false when no node is ready.
func (o *Orchestrator) ExecuteReadyBatch(ctx context.Context) ([]BatchNodeResult, bool, error) {
	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return nil, false, err
	}
	ids, err := ReadyBatchNodeIDs(tree)
	if err != nil {
		return nil, false, err
	}
	if len(ids) == 0 {
		return nil, false, nil
	}
	results, err := o.ExecuteBatch(ctx, ids)
	return results, true, err
}

// ExecuteBatch runs scheduler-approved nodes concurrently, bounded by
// MaxParallel. It records every node result before returning, even when one
// worker fails.
func (o *Orchestrator) ExecuteBatch(ctx context.Context, ids []string) ([]BatchNodeResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o == nil {
		return nil, ErrNoTaskTreeState
	}
	ids, err := normalizeBatchIDs(ids)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []BatchNodeResult{}, nil
	}

	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateReadyBatchMembership(tree, ids); err != nil {
		return nil, err
	}

	for _, id := range ids {
		node, _ := tree.Node(id)
		if node.Status == StatusPending {
			if err := tree.TransitionNode(id, StatusReady); err != nil {
				return nil, err
			}
		}
		if err := tree.TransitionNode(id, StatusRunning); err != nil {
			return nil, err
		}
	}
	tree.UpdatedAt = time.Now().UTC()
	if err := o.saveTaskTree(ctx, tree); err != nil {
		return nil, err
	}
	for _, id := range ids {
		o.emitNodeStarted(tree, id)
		o.emitNodePhase("started", id)
	}

	results := o.dispatchBatch(ctx, tree, ids)
	for i, result := range results {
		nodeResult := normalizeDispatchResult(result.Result, result.Err)
		if err := tree.RecordResult(result.NodeID, nodeResult); err != nil {
			return results, err
		}
		node, _ := tree.Node(result.NodeID)
		results[i].Status = node.Status
		results[i].Result = nodeResult
		results[i].Err = batchResultErr(nodeResult, result.Err)
	}
	tree.UpdatedAt = time.Now().UTC()
	if err := o.saveTaskTree(ctx, tree); err != nil {
		return results, err
	}
	for _, result := range results {
		o.emitNodeFinished(tree, result.NodeID, result.Result)
	}
	if batchHasFailure(results) {
		return results, &BatchError{Results: cloneBatchNodeResults(results)}
	}
	return results, nil
}

func (o *Orchestrator) dispatchBatch(ctx context.Context, tree *TaskTree, ids []string) []BatchNodeResult {
	type workerResult struct {
		index  int
		result BatchNodeResult
	}
	sem := make(chan struct{}, o.MaxParallel())
	out := make(chan workerResult, len(ids))
	var wg sync.WaitGroup

	for i, id := range ids {
		i, id := i, id
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out <- workerResult{index: i, result: BatchNodeResult{
					NodeID: id,
					Result: NodeResult{Summary: "Node execution failed.", Error: ctx.Err().Error()},
					Err:    ctx.Err(),
				}}
				return
			}

			nodeContext, err := tree.BuildNodeContext(id)
			if err != nil {
				out <- workerResult{index: i, result: BatchNodeResult{
					NodeID: id,
					Result: NodeResult{Summary: "Node execution failed.", Error: err.Error()},
					Err:    err,
				}}
				return
			}
			dispatchTree, err := cloneTaskTreeForDispatch(tree)
			if err != nil {
				out <- workerResult{index: i, result: BatchNodeResult{
					NodeID: id,
					Result: NodeResult{Summary: "Node execution failed.", Error: err.Error()},
					Err:    err,
				}}
				return
			}
			result, err := o.dispatchNode(ctx, dispatchTree, id, nodeContext)
			out <- workerResult{index: i, result: BatchNodeResult{
				NodeID: id,
				Result: result,
				Err:    err,
			}}
		}()
	}

	wg.Wait()
	close(out)

	results := make([]BatchNodeResult, len(ids))
	for item := range out {
		results[item.index] = item.result
	}
	return results
}

func normalizeBatchIDs(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for i, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: node id at index %d is empty", ErrInvalidBatch, i)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("%w: duplicate node id %q", ErrInvalidBatch, id)
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func validateReadyBatchMembership(tree *TaskTree, ids []string) error {
	ready, err := ReadyBatchNodeIDs(tree)
	if err != nil {
		return err
	}
	readySet := make(map[string]struct{}, len(ready))
	for _, id := range ready {
		readySet[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := readySet[id]; !ok {
			return fmt.Errorf("%w: %q is not in the current ready batch", ErrNodeNotReady, id)
		}
	}
	return nil
}

func normalizeDispatchResult(result NodeResult, dispatchErr error) NodeResult {
	if dispatchErr != nil {
		if result.Summary == "" {
			result.Summary = "Node execution failed."
		}
		result.Blocker = ""
		if result.Error == "" {
			result.Error = dispatchErr.Error()
		}
		return result
	}
	if result.Summary == "" && result.Error == "" && result.Blocker == "" {
		result.Summary = "Node execution completed."
	}
	return result
}

func batchResultErr(result NodeResult, dispatchErr error) error {
	if dispatchErr != nil {
		return dispatchErr
	}
	if strings.TrimSpace(result.Error) != "" {
		return errors.New(result.Error)
	}
	return nil
}

func batchHasFailure(results []BatchNodeResult) bool {
	for _, result := range results {
		if result.Err != nil {
			return true
		}
	}
	return false
}

func cloneBatchNodeResults(in []BatchNodeResult) []BatchNodeResult {
	out := make([]BatchNodeResult, len(in))
	copy(out, in)
	return out
}

func cloneTaskTreeForDispatch(tree *TaskTree) (*TaskTree, error) {
	snapshot, err := cloneSnapshot(Snapshot{Tree: tree})
	if err != nil {
		return nil, err
	}
	return snapshot.Tree, nil
}
