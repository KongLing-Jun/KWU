// Package tasktree contains the task-tree mode runner.
package tasktree

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/tool"
)

// ErrNotImplemented is returned by the first task-tree runner skeleton. Later
// task-tree phases replace Orchestrator.Run with generation and scheduling.
var ErrNotImplemented = errors.New("task_tree agent mode is not implemented yet")

// ErrNoTaskTree reports a planner boundary violation: a nil tree with no error.
var ErrNoTaskTree = errors.New("task_tree planner returned no tree")

// Planner is the task-tree generation boundary. Later phases can back it with a
// provider call, a deterministic parser, or a planner Agent without changing the
// Controller -> runner.Run(ctx, input) shape.
type Planner interface {
	PlanTaskTree(ctx context.Context, input string) (*TaskTree, error)
}

// PlannerFunc adapts a function to Planner for tests and small planner shims.
type PlannerFunc func(context.Context, string) (*TaskTree, error)

func (f PlannerFunc) PlanTaskTree(ctx context.Context, input string) (*TaskTree, error) {
	return f(ctx, input)
}

// NotImplementedPlanner is the explicit planner placeholder wired by boot until
// the goal-to-tree generation phase supplies a real planner.
func NotImplementedPlanner() Planner {
	return PlannerFunc(func(context.Context, string) (*TaskTree, error) {
		return nil, ErrNotImplemented
	})
}

// Executor is the callable execution boundary available to later scheduling
// phases. *agent.Agent satisfies this, and tests can provide a focused runner.
type Executor interface {
	Run(ctx context.Context, input string) error
}

// Options carries the dependencies the eventual task-tree orchestrator needs to
// generate, schedule, execute, and observe task nodes.
type Options struct {
	BaseRunner  agent.Runner
	Planner     Planner
	Executor    Executor
	Dispatcher  NodeDispatcher
	MaxParallel int
	Registry    *tool.Registry
	Sink        event.Sink
	Store       StateStore
}

// Orchestrator is the agent.Runner implementation for task_tree mode.
type Orchestrator struct {
	baseRunner  agent.Runner
	planner     Planner
	executor    Executor
	dispatcher  NodeDispatcher
	maxParallel int
	registry    *tool.Registry
	sink        event.Sink
	store       StateStore
}

// NewOrchestrator builds the task_tree runner skeleton.
func NewOrchestrator(opts Options) *Orchestrator {
	sink := opts.Sink
	if sink == nil {
		sink = event.Discard
	}
	planner := opts.Planner
	if planner == nil {
		planner = NotImplementedPlanner()
	}
	store := opts.Store
	if store == nil {
		store = NewMemoryStateStore()
	}
	dispatcher := opts.Dispatcher
	if dispatcher == nil {
		dispatcher = newRoutedNodeDispatcher(opts.Executor, opts.Registry, sink)
	}
	maxParallel := opts.MaxParallel
	if maxParallel <= 0 {
		maxParallel = DefaultMaxParallelNodes
	}
	return &Orchestrator{
		baseRunner:  opts.BaseRunner,
		planner:     planner,
		executor:    opts.Executor,
		dispatcher:  dispatcher,
		maxParallel: maxParallel,
		registry:    opts.Registry,
		sink:        sink,
		store:       store,
	}
}

// BaseRunner returns the normal-mode runner assembled before task_tree wrapping.
func (o *Orchestrator) BaseRunner() agent.Runner {
	if o == nil {
		return nil
	}
	return o.baseRunner
}

// Planner returns the task-tree generation boundary.
func (o *Orchestrator) Planner() Planner {
	if o == nil {
		return nil
	}
	return o.planner
}

// Executor returns the callable executor the orchestrator will dispatch through
// in later task-tree phases.
func (o *Orchestrator) Executor() Executor {
	if o == nil {
		return nil
	}
	return o.executor
}

// Dispatcher returns the node dispatch boundary used by serial/parallel
// task-tree execution.
func (o *Orchestrator) Dispatcher() NodeDispatcher {
	if o == nil {
		return nil
	}
	return o.dispatcher
}

// MaxParallel returns the concurrency limit used by ExecuteBatch.
func (o *Orchestrator) MaxParallel() int {
	if o == nil || o.maxParallel <= 0 {
		return DefaultMaxParallelNodes
	}
	return o.maxParallel
}

// Registry returns the session tool registry visible to the orchestrator.
func (o *Orchestrator) Registry() *tool.Registry {
	if o == nil {
		return nil
	}
	return o.registry
}

// StateStore returns the mode-specific state store owned by this orchestrator.
func (o *Orchestrator) StateStore() StateStore {
	if o == nil {
		return nil
	}
	return o.store
}

// Run satisfies agent.Runner. Controller remains unaware of task-tree internals:
// it submits one user input, while the orchestrator handles planning, state
// storage, task-tree events, and the final user-visible summary behind this
// runner boundary.
func (o *Orchestrator) Run(ctx context.Context, input string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if o == nil {
		return ErrNotImplemented
	}
	o.sink.Emit(event.Event{Kind: event.TurnStarted})
	o.sink.Emit(event.Event{Kind: event.Phase, Text: "task_tree: planning", Source: SnapshotModeTaskTree})

	tree, err := o.planner.PlanTaskTree(ctx, input)
	if err != nil {
		if ctx.Err() == nil {
			text := "task_tree planning failed: " + err.Error()
			if errors.Is(err, ErrNotImplemented) {
				text = "task_tree agent mode is selected, but task-tree planning is not configured yet."
			}
			o.sink.Emit(event.Event{
				Kind:   event.Notice,
				Level:  event.LevelWarn,
				Text:   text,
				Source: SnapshotModeTaskTree,
			})
		}
		return err
	}
	if tree == nil {
		o.sink.Emit(event.Event{
			Kind:   event.Notice,
			Level:  event.LevelWarn,
			Text:   "task_tree planning failed: planner returned no tree.",
			Source: SnapshotModeTaskTree,
		})
		return ErrNoTaskTree
	}
	if err := o.store.SaveSnapshot(ctx, Snapshot{
		Version: SnapshotVersion,
		Mode:    SnapshotModeTaskTree,
		Tree:    tree,
	}); err != nil {
		return fmt.Errorf("task tree state store: %w", err)
	}

	o.emitTreeCreated(tree)
	o.sink.Emit(event.Event{Kind: event.Phase, Text: "task_tree: tree_created", Source: SnapshotModeTaskTree})

	decisions, err := o.executePlannedTree(ctx)
	if err != nil {
		if ctx.Err() == nil {
			o.sink.Emit(event.Event{
				Kind:   event.Notice,
				Level:  event.LevelWarn,
				Text:   "task_tree execution failed: " + err.Error(),
				Source: SnapshotModeTaskTree,
			})
		}
		return err
	}
	if _, err := o.Summarize(ctx, decisions...); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) executePlannedTree(ctx context.Context) ([]SummaryDecision, error) {
	if err := o.setTreeStatus(ctx, StatusRunning); err != nil {
		return nil, err
	}
	o.sink.Emit(event.Event{Kind: event.Phase, Text: "task_tree: executing", Source: SnapshotModeTaskTree})

	var decisions []SummaryDecision
	for {
		if err := ctx.Err(); err != nil {
			return decisions, err
		}
		_, ok, err := o.ExecuteReadyBatch(ctx)
		if err != nil {
			if agent.IsStreamDispatcherTerminalError(err) {
				return decisions, err
			}
			if errors.Is(err, ErrBatchExecutionFailed) {
				break
			}
			return decisions, err
		}
		if !ok {
			break
		}
	}

	status, decision, err := o.finalizeTreeStatus(ctx)
	if err != nil {
		return decisions, err
	}
	if decision.Decision != "" {
		decisions = append(decisions, decision)
	}
	o.emitTreeCompletedStatus(ctx, status)
	o.sink.Emit(event.Event{Kind: event.Phase, Text: fmt.Sprintf("task_tree: %s", status), Source: SnapshotModeTaskTree})
	return decisions, nil
}

func (o *Orchestrator) setTreeStatus(ctx context.Context, status Status) error {
	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return err
	}
	if tree.Status == status {
		return nil
	}
	tree.Status = status
	tree.UpdatedAt = time.Now().UTC()
	return o.saveTaskTree(ctx, tree)
}

func (o *Orchestrator) finalizeTreeStatus(ctx context.Context) (Status, SummaryDecision, error) {
	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return "", SummaryDecision{}, err
	}
	outcome := OverallOutcome(tree)
	status := treeStatusForOutcome(outcome)
	if tree.Status != status {
		tree.Status = status
		tree.UpdatedAt = time.Now().UTC()
		if err := o.saveTaskTree(ctx, tree); err != nil {
			return "", SummaryDecision{}, err
		}
	}
	return status, decisionForOutcome(outcome), nil
}

func treeStatusForOutcome(outcome Outcome) Status {
	switch outcome {
	case OutcomeCompleted:
		return StatusCompleted
	case OutcomeBlocked:
		return StatusBlocked
	case OutcomePartialFailure:
		return StatusFailed
	default:
		return StatusRunning
	}
}

func decisionForOutcome(outcome Outcome) SummaryDecision {
	switch outcome {
	case OutcomeCompleted:
		return SummaryDecision{
			Decision:  "Finished automatic task-tree execution",
			Rationale: "all task nodes reached completed status.",
		}
	case OutcomeBlocked:
		return SummaryDecision{
			Decision:           "Stopped automatic task-tree execution",
			Rationale:          "at least one task node is blocked and requires user or environment input.",
			UserActionRequired: true,
		}
	case OutcomePartialFailure:
		return SummaryDecision{
			Decision:           "Stopped automatic task-tree execution",
			Rationale:          "at least one task node failed or was skipped; unresolved dependent nodes were left for follow-up.",
			UserActionRequired: true,
		}
	default:
		return SummaryDecision{
			Decision:           "Stopped automatic task-tree execution",
			Rationale:          "no runnable task nodes remain, but some nodes are still unfinished.",
			UserActionRequired: true,
		}
	}
}

func (o *Orchestrator) emitTreeCompletedStatus(ctx context.Context, status Status) {
	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return
	}
	tree.Status = status
	o.emitTreeCompleted(tree)
}
