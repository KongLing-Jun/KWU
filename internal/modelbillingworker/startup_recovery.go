package modelbillingworker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kwu/internal/modelbilling"
)

const StartupRecoveryErrorCode = "local_restart_interrupted"

type StartupRecoveryTaskStore interface {
	ListStartupRecoveryIDs(context.Context, modelbilling.TaskScope) ([]string, error)
	Get(context.Context, modelbilling.TaskScope, string) (modelbilling.ModelTask, error)
	UpdateState(context.Context, modelbilling.TaskStateUpdate) error
	QuarantineStartupTask(context.Context, modelbilling.TaskScope, string, time.Time, string) error
}

type StartupRecoveryCallStore interface {
	ListByTask(context.Context, modelbilling.TaskScope, string) ([]modelbilling.ModelCall, error)
	Complete(context.Context, modelbilling.TaskContext, modelbilling.ModelCall) error
}

// StartupExecutionRecovery closes only local execution crash windows. It does
// not own quota reports, settlement, freeze release, or any cloud operation.
type StartupExecutionRecovery struct {
	tasks StartupRecoveryTaskStore
	calls StartupRecoveryCallStore
	clock func() time.Time
}

func NewStartupExecutionRecovery(
	tasks StartupRecoveryTaskStore,
	calls StartupRecoveryCallStore,
	clock func() time.Time,
) (*StartupExecutionRecovery, error) {
	if tasks == nil || calls == nil || clock == nil {
		return nil, contractError("startup task store, call store, and clock are required")
	}
	return &StartupExecutionRecovery{tasks: tasks, calls: calls, clock: clock}, nil
}

// RecoverScope scans opaque IDs in one exact scope, then hydrates and recovers
// each row independently. A successfully quarantined corrupt row is complete
// recovery work: it does not block the remaining rows or the caller's worker.
// Scan, context, non-deterministic store, and quarantine failures remain fatal.
func (r *StartupExecutionRecovery) RecoverScope(ctx context.Context, scope modelbilling.TaskScope) error {
	if ctx == nil {
		ctx = context.Background()
	}
	now, err := r.recoveryTime()
	if err != nil {
		return err
	}
	ids, err := r.tasks.ListStartupRecoveryIDs(ctx, scope)
	if err != nil {
		return fmt.Errorf("scan startup tasks: %w", err)
	}
	for _, taskID := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.recoverStartupTask(ctx, scope, taskID, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *StartupExecutionRecovery) recoveryTime() (time.Time, error) {
	if r == nil || r.tasks == nil || r.calls == nil || r.clock == nil {
		return time.Time{}, contractError("startup execution recovery dependencies are required")
	}
	now := r.clock().UTC()
	if now.IsZero() {
		return time.Time{}, contractError("startup recovery clock returned zero time")
	}
	return now, nil
}

func (r *StartupExecutionRecovery) recoverStartupTask(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
	now time.Time,
) error {
	err := r.recoverTask(ctx, scope, taskID, now)
	if err == nil {
		return nil
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return errors.Join(err, contextErr)
	}
	if !isDeterministicLocalPersistenceError(err) {
		return fmt.Errorf("recover startup task %q: %w", taskID, err)
	}
	if quarantineErr := r.tasks.QuarantineStartupTask(ctx, scope, taskID, now, StartupRecoveryErrorCode); quarantineErr != nil {
		return fmt.Errorf("quarantine startup task %q: %w", taskID, errors.Join(err, quarantineErr))
	}
	return nil
}

func (r *StartupExecutionRecovery) recoverTask(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
	now time.Time,
) error {
	task, err := r.tasks.Get(ctx, scope, taskID)
	if err != nil {
		return markLocalPersistenceError(err)
	}
	if !task.Scope.Equal(scope) || task.BillingSource != startupBillingSource(scope) {
		return markLocalPersistenceError(contractError("startup task identity does not match recovery scope"))
	}
	switch task.State {
	case modelbilling.TaskCreated:
		return r.updateTask(ctx, task, modelbilling.TaskFailed, taskResultPointer(modelbilling.TaskResultFailed), now)
	case modelbilling.TaskFreezePending, modelbilling.TaskReady:
		return r.updateTask(ctx, task, modelbilling.TaskRecoveryRequired, nil, now)
	case modelbilling.TaskRunning:
		if task.StartedAt == nil {
			return r.updateTask(ctx, task, modelbilling.TaskRecoveryRequired, nil, now)
		}
		return r.recoverRunningTask(ctx, task, now)
	default:
		// The state may have advanced after the ID scan. It is no longer a
		// startup crash window and must not be moved backwards.
		return nil
	}
}

func (r *StartupExecutionRecovery) recoverRunningTask(ctx context.Context, task modelbilling.ModelTask, now time.Time) error {
	taskContext, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	})
	if err != nil {
		return markLocalPersistenceError(err)
	}
	calls, err := r.calls.ListByTask(ctx, task.Scope, task.TaskID)
	if err != nil {
		return markLocalPersistenceError(err)
	}
	for _, call := range calls {
		if call.State != modelbilling.CallRunning {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		finishedAt := now
		code := StartupRecoveryErrorCode
		call.State = modelbilling.CallInterrupted
		call.FinishedAt = &finishedAt
		call.LastErrorCode = &code
		if err := r.calls.Complete(ctx, taskContext, call); err != nil {
			return fmt.Errorf("interrupt running call %q: %w", call.CallID, markLocalPersistenceError(err))
		}
	}
	if task.BillingSource == modelbilling.BillingSubscriptionPool {
		return r.updateTask(ctx, task, modelbilling.TaskUsagePending, taskResultPointer(modelbilling.TaskResultFailed), now)
	}
	return r.updateTask(ctx, task, modelbilling.TaskFailed, taskResultPointer(modelbilling.TaskResultFailed), now)
}

func (r *StartupExecutionRecovery) updateTask(
	ctx context.Context,
	task modelbilling.ModelTask,
	to modelbilling.TaskState,
	result *modelbilling.TaskResult,
	at time.Time,
) error {
	code := StartupRecoveryErrorCode
	return markLocalPersistenceError(r.tasks.UpdateState(ctx, modelbilling.TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: task.State, To: to,
		Result: result, LastErrorCode: &code, At: at,
	}))
}

func startupBillingSource(scope modelbilling.TaskScope) modelbilling.BillingSource {
	if scope.CloudBinding != nil {
		return modelbilling.BillingSubscriptionPool
	}
	return modelbilling.BillingNewAPIByok
}

func taskResultPointer(result modelbilling.TaskResult) *modelbilling.TaskResult { return &result }
