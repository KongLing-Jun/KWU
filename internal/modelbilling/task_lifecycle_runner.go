package modelbilling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
)

// TaskLifecycleRunner owns task state around one complete top-level user
// submission; MainExecutorDispatcher owns only individual calls inside it.
type TaskLifecycleRunner struct {
	Tasks    TaskStore
	Recovery UsagePendingRecovery
	Now      func() time.Time
}

// UsagePendingRecovery resolves a subscription task after its terminal state
// has been persisted. Implementations must inspect durable usage before
// releasing a freeze; a failed request alone is not proof of zero usage.
type UsagePendingRecovery interface {
	RecoverUsagePendingTask(context.Context, TaskScope, string) error
}

// RunTask owns the task lifecycle around one top-level user submission. The
// callback may invoke the underlying runner more than once while the task stays
// running.
func (r TaskLifecycleRunner) RunTask(ctx context.Context, run func(context.Context) error) error {
	if r.Tasks == nil || run == nil {
		return newError(ErrorContractInvalid, "task lifecycle dependencies are required")
	}
	task, ok := TaskContextFrom(ctx)
	if !ok {
		return newError(ErrorContractInvalid, "task lifecycle requires task context")
	}
	if err := r.update(ctx, task, TaskReady, TaskRunning, nil, nil); err != nil {
		return err
	}
	barrier := newForegroundCallBarrier(task.TaskID)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	runCtx = withForegroundCallBarrier(runCtx, barrier)
	runCtx = agent.WithTurnWorkRegistrar(runCtx, turnWorkRegistrar(barrier, task.TaskID))
	runErr := run(runCtx)
	// The callback owns every foreground stream it starts. Once it returns,
	// cancel any stream it abandoned so terminal persistence can finish before
	// the barrier is sealed and awaited.
	cancelRun()
	barrier.seal()
	if errors.Is(runErr, agent.ErrMainStreamDispatcherCloseTimeout) {
		if barrierErr := barrier.collectedError(); barrierErr != nil {
			runErr = joinCausalErrors(barrierErr, runErr)
		}
		return r.finishFailure(ctx, task, runErr)
	}
	barrierErr := barrier.wait()
	if barrierErr != nil {
		if runErr != nil {
			barrierErr = joinCausalErrors(barrierErr, runErr)
		}
		return r.finishFailure(ctx, task, barrierErr)
	}
	if runErr != nil {
		return r.finishFailure(ctx, task, runErr)
	}
	return r.finishSuccess(ctx, task)
}

func turnWorkRegistrar(barrier *foregroundCallBarrier, taskID string) func() (func(error), error) {
	return func() (func(error), error) {
		registration, err := barrier.register(taskID)
		if err != nil {
			return nil, err
		}
		return registration.Complete, nil
	}
}

func (r TaskLifecycleRunner) finishSuccess(ctx context.Context, task TaskContext) error {
	result := TaskResultCompleted
	terminal := TaskCompleted
	if task.BillingSource == BillingSubscriptionPool {
		terminal = TaskUsagePending
	}
	return r.update(ctx, task, TaskRunning, terminal, &result, nil)
}

func (r TaskLifecycleRunner) finishFailure(ctx context.Context, task TaskContext, runErr error) error {
	if errors.Is(runErr, agent.ErrMainStreamDispatcherCloseTimeout) {
		code := string(ErrorUsagePersistFailed)
		recoveryErr := r.update(ctx, task, TaskRunning, TaskRecoveryRequired, nil, &code)
		return newTerminalPersistenceError(runErr, recoveryErr)
	}
	if isTerminalPersistenceError(runErr) {
		return runErr
	}
	result, state := TaskResultFailed, TaskFailed
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		result, state = TaskResultCancelled, TaskCancelled
	}
	if task.BillingSource == BillingSubscriptionPool {
		state = TaskUsagePending
	}
	code := ErrorCodeOf(runErr)
	if code == "" {
		code = ErrorGatewayUnavailable
	}
	if err := r.update(ctx, task, TaskRunning, state, &result, stringPointer(string(code))); err != nil {
		return err
	}
	if err := r.recoverFailedSubscription(ctx, task); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

func (r TaskLifecycleRunner) recoverFailedSubscription(ctx context.Context, task TaskContext) error {
	if task.BillingSource != BillingSubscriptionPool || r.Recovery == nil {
		return nil
	}
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistenceTimeout)
	defer cancel()
	if err := r.Recovery.RecoverUsagePendingTask(recoveryCtx, task.Scope, task.TaskID); err != nil {
		return fmt.Errorf("recover failed subscription task: %w", err)
	}
	return nil
}

func (r TaskLifecycleRunner) update(ctx context.Context, task TaskContext, from, to TaskState, result *TaskResult, code *string) error {
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistenceTimeout)
	defer cancel()
	if err := r.Tasks.UpdateState(persistCtx, TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: from, To: to, Result: result, LastErrorCode: code, At: now}); err != nil {
		return err
	}
	emitTaskLifecycle(ctx, task.TaskID, from, to)
	return nil
}

func joinCausalErrors(a, b error) error {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case errors.Is(a, b):
		return a
	case errors.Is(b, a):
		return b
	default:
		return errors.Join(a, b)
	}
}

func stringPointer(value string) *string { return &value }

func emitTaskLifecycle(ctx context.Context, taskID string, from, to TaskState) {
	emitter, ok := event.OperationalEmitterFrom(ctx)
	if !ok {
		return
	}
	scope := event.OperationalScopeFrom(ctx)
	scope.TaskID = taskID
	emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateTask, AggregateID: taskID,
		Scope: scope, Kind: taskOperationalKind(to),
		Stage: "persistence", Outcome: string(to), Severity: lifecycleSeverity(string(to)), UserImpact: lifecycleImpact(string(to)),
		Retryable: to == TaskRecoveryRequired,
		Payload:   &event.LifecycleOperationalPayload{State: string(to), PreviousState: string(from)},
	})
}

func emitModelCallLifecycle(ctx context.Context, call ModelCall, from, to CallState) {
	emitter, ok := event.OperationalEmitterFrom(ctx)
	if !ok {
		return
	}
	scope := event.OperationalScopeFrom(ctx)
	scope.TaskID, scope.CallID = call.TaskID, call.CallID
	emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateModelCall, AggregateID: call.CallID,
		Scope: scope, Kind: event.OperationalKind("model_call." + string(to)),
		Stage: "persistence", Outcome: string(to), Severity: lifecycleSeverity(string(to)), UserImpact: lifecycleImpact(string(to)),
		Retryable: to == CallInterrupted || to == CallUsageMissing,
		Payload:   &event.LifecycleOperationalPayload{State: string(to), PreviousState: string(from), Role: string(call.Role)},
	})
}

func taskOperationalKind(state TaskState) event.OperationalKind {
	return event.OperationalKind("task." + string(state))
}

func lifecycleSeverity(state string) string {
	switch state {
	case "failed", "recovery_required", "usage_missing":
		return "error"
	case "cancelled", "interrupted":
		return "warn"
	default:
		return "info"
	}
}

func lifecycleImpact(state string) string {
	switch state {
	case "failed", "recovery_required", "usage_missing":
		return "blocked"
	case "running", "usage_pending", "settling":
		return "waiting_for_backend"
	default:
		return "none"
	}
}
