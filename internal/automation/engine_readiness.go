package automation

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (e *Engine) runTask(ctx context.Context, task Task, trigger Trigger, due DueResult, runID string) (string, error) {
	recovered, err := e.store.RecoverInterruptedRuns(ctx, task.ID)
	if err != nil {
		return "", err
	}
	for _, run := range recovered {
		e.emitChange(EngineChange{Kind: "run", TaskID: run.TaskID, RunID: run.RunID, Status: string(run.Status), At: run.FinishedAt})
	}
	latest, due, runnable, err := e.prepareRunTask(task.ID, trigger, due)
	if err != nil || !runnable {
		return "", err
	}
	startedAt, runID := e.runIdentity(latest, trigger, due, runID)
	runCtx, cancel := context.WithCancel(ctx)
	if !e.running.add(latest.ID, runID, cancel) {
		cancel()
		return "", ErrShuttingDown
	}
	defer func() {
		cancel()
		e.running.remove(latest.ID, runID)
	}()
	if _, ok, err := e.store.AcquireLease(latest.ID, runID, e.owner, e.leaseTTL); err != nil {
		return "", err
	} else if !ok {
		return "", ErrLeaseUnavailable
	}
	defer func() { e.running.recordCleanupError(e.store.ReleaseLease(latest.ID, runID, e.owner)) }()
	return e.runLeasedTask(runCtx, latest.ID, trigger, due, runID, startedAt)
}

func (e *Engine) prepareRunTask(taskID string, trigger Trigger, due DueResult) (Task, DueResult, bool, error) {
	latest, due, runnable, err := e.loadPreflightTask(taskID, trigger, due)
	if err != nil || !runnable {
		return latest, due, runnable, err
	}
	err = e.preflightTaskForRun(latest, trigger, due)
	return latest, due, err == nil, err
}

func (e *Engine) runIdentity(task Task, trigger Trigger, due DueResult, runID string) (time.Time, string) {
	startedAt := e.now().UTC()
	runID = strings.TrimSpace(runID)
	if runID != "" {
		return startedAt, runID
	}
	if trigger == TriggerSchedule {
		return startedAt, scheduledRunID(task, due)
	}
	return startedAt, newRunID(startedAt)
}

func (e *Engine) runLeasedTask(ctx context.Context, taskID string, trigger Trigger, due DueResult, runID string, startedAt time.Time) (string, error) {
	latest, due, runnable, err := e.loadRunnableTask(taskID, runID, trigger, due)
	if err != nil || !runnable {
		return "", err
	}
	result, handled, err := e.recoverLeasedScheduledRun(ctx, latest, trigger, due, runID, startedAt)
	if err != nil || result != "" {
		return result, err
	}
	if err := e.preflightTaskForRun(latest, trigger, due); err != nil {
		return "", err
	}
	if handled {
		return "", nil
	}
	if err := e.checkTaskQuota(ctx, latest.ID); err != nil {
		return "", err
	}
	if e.shouldSkipMissedRun(latest, trigger, due, startedAt) {
		return e.skipMissedRun(ctx, latest, due, runID, startedAt)
	}
	return e.executeTaskRun(ctx, latest, trigger, due, runID, startedAt)
}

func (e *Engine) recoverLeasedScheduledRun(ctx context.Context, task Task, trigger Trigger, due DueResult, runID string, startedAt time.Time) (string, bool, error) {
	if trigger != TriggerSchedule {
		return "", false, nil
	}
	result, handled, err := e.recoverScheduledRun(ctx, task, due, runID, startedAt)
	if result != "" {
		e.consumeScheduledDeferred(task, due)
	}
	return result, handled, err
}

func (e *Engine) shouldSkipMissedRun(task Task, trigger Trigger, due DueResult, startedAt time.Time) bool {
	if trigger != TriggerSchedule || task.MissedRunPolicy != MissedRunSkip {
		return false
	}
	return !e.hasScheduledDeferred(task, due) && e.isMissedRun(due, startedAt)
}

func (e *Engine) preflightTaskForRun(task Task, trigger Trigger, due DueResult) error {
	err := e.preflightTask(task)
	if err != nil && trigger == TriggerSchedule && errors.Is(err, ErrDependencyInitializing) {
		e.markScheduledDeferred(task, due)
	}
	return err
}

func (e *Engine) loadPreflightTask(taskID string, trigger Trigger, due DueResult) (Task, DueResult, bool, error) {
	latest, ok, err := e.reloadTask(taskID)
	if err != nil || !ok {
		return Task{}, due, false, err
	}
	if trigger == TriggerSchedule {
		due = Due(latest, e.now())
		if !latest.Enabled || !due.Due {
			return Task{}, due, false, nil
		}
	}
	return latest, due, true, nil
}

func (e *Engine) preflightTask(task Task) error {
	if e.checker.Preflight == nil {
		return nil
	}
	result := e.checker.Preflight(task)
	switch result.Disposition {
	case PreflightReady:
		return nil
	case PreflightDefer:
		return &DependencyInitializingError{TaskID: task.ID, Reason: result.Reason}
	default:
		return &DependencyInitializingError{TaskID: task.ID, Reason: "task dependency readiness is unknown"}
	}
}

type deferredOccurrenceSet map[string]string

func (e *Engine) markScheduledDeferred(task Task, due DueResult) {
	key := scheduledRunID(task, due)
	for {
		current := e.deferredRuns.Load()
		next := make(deferredOccurrenceSet, 1)
		if current != nil {
			next = make(deferredOccurrenceSet, len(*current)+1)
			for taskID, occurrence := range *current {
				next[taskID] = occurrence
			}
		}
		next[task.ID] = key
		if e.deferredRuns.CompareAndSwap(current, &next) {
			return
		}
	}
}

func (e *Engine) consumeScheduledDeferred(task Task, due DueResult) bool {
	want := scheduledRunID(task, due)
	for {
		current := e.deferredRuns.Load()
		if current == nil {
			return false
		}
		got, ok := (*current)[task.ID]
		if !ok {
			return false
		}
		next := make(deferredOccurrenceSet, len(*current)-1)
		for taskID, occurrence := range *current {
			if taskID != task.ID {
				next[taskID] = occurrence
			}
		}
		if e.deferredRuns.CompareAndSwap(current, &next) {
			return got == want
		}
	}
}

func (e *Engine) hasScheduledDeferred(task Task, due DueResult) bool {
	current := e.deferredRuns.Load()
	if current == nil {
		return false
	}
	got, ok := (*current)[task.ID]
	return ok && got == scheduledRunID(task, due)
}
