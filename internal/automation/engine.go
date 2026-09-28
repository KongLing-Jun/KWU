package automation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kwu/internal/event"
)

const (
	defaultEngineTickInterval = 30 * time.Second
	defaultEngineLeaseTTL     = 2 * time.Minute
	maxCatchUpRunsPerTick     = 3
)

// Runner executes one automation run. The first version is intentionally mockable.
type Runner interface {
	Run(context.Context, RunRequest) (RunResult, error)
}

var (
	ErrTaskNotFound           = errors.New("automation: task not found")
	ErrShuttingDown           = errors.New("automation: service is shutting down")
	ErrDependencyInitializing = errors.New("automation: dependency initializing")
)

// DependencyInitializingError reports that a manual run cannot start until a
// task dependency becomes ready. Scheduled runs consume this signal internally
// and remain due for a later tick.
type DependencyInitializingError struct {
	TaskID string
	Reason string
}

func (e *DependencyInitializingError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "task dependencies are initializing"
	}
	if taskID := strings.TrimSpace(e.TaskID); taskID != "" {
		return fmt.Sprintf("automation: task %s: %s", taskID, reason)
	}
	return "automation: " + reason
}

func (e *DependencyInitializingError) Unwrap() error { return ErrDependencyInitializing }

// TriggerRequest asks the Engine to run one task immediately through the same
// lease, condition, runner, and archive path used by scheduled ticks.
type TriggerRequest struct {
	TaskID string
	Reason string
	RunID  string
}

type EngineChange struct {
	Kind       string    `json:"kind"`
	TaskID     string    `json:"taskId,omitempty"`
	RunID      string    `json:"runId,omitempty"`
	DeliveryID string    `json:"deliveryId,omitempty"`
	Status     string    `json:"status,omitempty"`
	Trigger    Trigger   `json:"trigger,omitempty"`
	Sequence   uint64    `json:"sequence,omitempty"`
	At         time.Time `json:"at"`
}

type archiveChangeSink struct {
	engine *Engine
	taskID string
	runID  string
	mu     sync.Mutex
	seq    uint64
}

func (s *archiveChangeSink) Emit(event.Event) {
	s.mu.Lock()
	s.seq++
	sequence := s.seq
	s.mu.Unlock()
	s.engine.emitChange(EngineChange{Kind: "run_event", TaskID: s.taskID, RunID: s.runID, Sequence: sequence})
}

// Engine scans stored tasks and dispatches due runs serially.
type Engine struct {
	store            *Store
	owner            string
	runner           Runner
	checker          Checker
	taskQuota        *TaskQuotaPolicy
	tickInterval     time.Duration
	leaseTTL         time.Duration
	now              func() time.Time
	deliveryAdapters map[string]DeliveryAdapter
	running          *runningRunRegistry
	deferredRuns     atomic.Pointer[deferredOccurrenceSet]

	mu         sync.Mutex
	deliveryMu sync.Mutex
	observerMu sync.RWMutex
	observer   func(EngineChange)
	adapterMu  sync.RWMutex
	cancel     context.CancelFunc
	done       chan struct{}
}

func (e *Engine) RegisterDeliveryAdapter(kind string, adapter DeliveryAdapter) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" || adapter == nil {
		return fmt.Errorf("automation: delivery adapter kind and implementation are required")
	}
	e.adapterMu.Lock()
	e.deliveryAdapters[kind] = adapter
	e.adapterMu.Unlock()
	return nil
}

func (e *Engine) SetChangeObserver(observer func(EngineChange)) {
	e.observerMu.Lock()
	e.observer = observer
	e.observerMu.Unlock()
}

func (e *Engine) emitChange(change EngineChange) {
	change.At = change.At.UTC()
	if change.At.IsZero() {
		change.At = e.now().UTC()
	}
	e.observerMu.RLock()
	observer := e.observer
	e.observerMu.RUnlock()
	if observer != nil {
		observer(change)
	}
}

// NewEngine creates an automation engine with conservative defaults.
func NewEngine(store *Store, owner string, runner Runner) *Engine {
	return &Engine{
		store:            store,
		owner:            owner,
		runner:           runner,
		tickInterval:     defaultEngineTickInterval,
		leaseTTL:         defaultEngineLeaseTTL,
		now:              time.Now,
		deliveryAdapters: map[string]DeliveryAdapter{"local_test": NewLocalDeliveryAdapter()},
		running:          newRunningRunRegistry(),
	}
}

// NewEngineWithChecker keeps non-Desktop callers on the legacy constructor
// while allowing Desktop to validate its dynamic model+route catalog before a
// run is archived or dispatched.
func NewEngineWithChecker(store *Store, owner string, runner Runner, checker Checker) *Engine {
	engine := NewEngine(store, owner, runner)
	engine.checker = checker
	return engine
}

// Tick 扫描一次任务列表，并按串行方式执行到期且满足条件的任务。
// 第一版故意不做多任务并发；需要吞吐时再在这里引入受控队列。
func (e *Engine) Tick(ctx context.Context) error {
	ctx, finish, err := e.running.admit(ctx)
	if err != nil {
		return err
	}
	defer finish()
	if err := e.ready(); err != nil {
		return err
	}
	tasks, err := e.store.LoadTasks()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := e.tickTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) tickTask(ctx context.Context, task Task) error {
	if !task.Enabled {
		return nil
	}
	limit := 1
	if task.MissedRunPolicy == MissedRunCatchUp {
		limit = maxCatchUpRunsPerTick
	}
	for attempt := 0; attempt < limit; attempt++ {
		keepGoing, err := e.runScheduledAttempt(ctx, task)
		if err != nil || !keepGoing {
			return err
		}
	}
	return nil
}

func (e *Engine) runScheduledAttempt(ctx context.Context, task Task) (bool, error) {
	current, err := e.store.Task(task.ID)
	if errors.Is(err, ErrTaskNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	due := Due(current, e.now())
	if !due.Due {
		return false, nil
	}
	runID, err := e.runTask(ctx, current, TriggerSchedule, due, "")
	if errors.Is(err, ErrTaskLimitExceeded) || errors.Is(err, ErrTaskQuotaUnavailable) {
		return false, nil
	}
	if errors.Is(err, ErrDependencyInitializing) || errors.Is(err, ErrLeaseUnavailable) {
		return false, nil
	}
	return runID != "" && task.MissedRunPolicy == MissedRunCatchUp, err
}

// Trigger runs one task immediately without bypassing Engine ownership checks.
func (e *Engine) Trigger(ctx context.Context, req TriggerRequest) (string, error) {
	ctx, finish, err := e.running.admit(ctx)
	if err != nil {
		return "", err
	}
	defer finish()
	return e.trigger(ctx, req)
}

func (e *Engine) trigger(ctx context.Context, req TriggerRequest) (string, error) {
	if err := e.ready(); err != nil {
		return "", err
	}
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		return "", ErrTaskNotFound
	}
	tasks, err := e.store.LoadTasks()
	if err != nil {
		return "", err
	}
	for _, task := range tasks {
		if task.ID == taskID {
			return e.runTask(ctx, task, TriggerManual, DueResult{Due: true, SkipReason: strings.TrimSpace(req.Reason)}, req.RunID)
		}
	}
	return "", fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
}

// Start begins periodic ticks until Stop or ctx cancellation.
func (e *Engine) Start(ctx context.Context) {
	e.mu.Lock()
	if e.cancel != nil {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	e.cancel = cancel
	e.done = done
	interval := e.tickInterval
	if interval <= 0 {
		interval = defaultEngineTickInterval
	}
	e.mu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			_ = e.Tick(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Stop ends a running engine loop.
func (e *Engine) Stop() {
	_ = e.stop(context.Background())
}

func (e *Engine) stop(ctx context.Context) error {
	e.mu.Lock()
	cancel := e.cancel
	done := e.done
	e.mu.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
		e.mu.Lock()
		if e.done == done {
			e.cancel = nil
			e.done = nil
		}
		e.mu.Unlock()
	}
	return nil
}

func (e *Engine) ready() error {
	if e.store == nil {
		return fmt.Errorf("automation: engine store is required")
	}
	if e.runner == nil {
		return fmt.Errorf("automation: engine runner is required")
	}
	return nil
}

func (e *Engine) recoverScheduledRun(ctx context.Context, task Task, due DueResult, runID string, startedAt time.Time) (string, bool, error) {
	if scheduledRunID(task, due) != runID {
		return "", true, nil
	}
	prior, err := e.store.ReadRunManifest(task.ID, runID)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", true, err
	}
	recordedAt := prior.ScheduledFor
	if recordedAt.IsZero() {
		recordedAt = due.NextRunAt
	}
	if err := e.store.RecordTaskRun(task.ID, prior.RunID, recordedAt); err != nil {
		return "", true, err
	}
	return prior.RunID, true, nil
}

func scheduledRunID(task Task, due DueResult) string {
	key := fmt.Sprintf("%s\x00%s\x00%d", strings.TrimSpace(task.ID), due.NextRunAt.UTC().Format(time.RFC3339Nano), task.Revision)
	digest := sha256.Sum256([]byte(key))
	return fmt.Sprintf("scheduled-%x", digest[:12])
}

func (e *Engine) isMissedRun(due DueResult, now time.Time) bool {
	if due.NextRunAt.IsZero() || !due.NextRunAt.Before(now) {
		return false
	}
	grace := e.tickInterval
	if grace <= 0 {
		grace = defaultEngineTickInterval
	}
	return now.Sub(due.NextRunAt) > grace
}

func (e *Engine) skipMissedRun(ctx context.Context, task Task, due DueResult, runID string, startedAt time.Time) (string, error) {
	snapshot, capabilityErr := resolveRunCapabilities(e.conditionChecker(), task, startedAt)
	run, _, archive, err := e.createRunArchive(task, TriggerSchedule, due, runID, startedAt, snapshot)
	if err != nil {
		return run.RunID, err
	}
	e.consumeScheduledDeferred(task, due)
	defer archive.Close()
	if capabilityErr != nil {
		run.Condition = conditionBlocked(startedAt, "capability unavailable", capabilityErr.Error())
		return run.RunID, e.finishRunWithArchive(ctx, archive, task.ID, run, conditionRunResult(run.Condition), nil)
	}
	result := RunResult{Status: RunStatusSkipped, Summary: "missed run skipped", ErrorCode: RunErrorConditionRejected, ErrorMessage: "missed run skipped"}
	return run.RunID, e.finishRunWithArchive(ctx, archive, task.ID, run, result, nil)
}

func (e *Engine) checkTaskQuota(ctx context.Context, taskID string) error {
	if e.taskQuota == nil {
		return nil
	}
	tasks, err := e.store.LoadTasks()
	if err != nil {
		return err
	}
	view, err := e.taskQuota.View(ctx, tasks)
	if err != nil {
		return err
	}
	if view.State != TaskQuotaStateReady {
		return ErrTaskQuotaUnavailable
	}
	if !taskQuotaAllows(view, taskID) {
		return &TaskLimitError{TaskID: taskID, Plan: view.Plan, Used: view.Used, Limit: view.Limit}
	}
	return nil
}

func (e *Engine) loadRunnableTask(taskID, runID string, trigger Trigger, due DueResult) (Task, DueResult, bool, error) {
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
	running, err := e.taskHasRunningRun(latest.ID, runID)
	if err != nil || running {
		return Task{}, due, false, err
	}
	return latest, due, true, nil
}

func (e *Engine) executeTaskRun(ctx context.Context, latest Task, trigger Trigger, due DueResult, runID string, startedAt time.Time) (string, error) {
	checker := e.conditionChecker()
	resolved, resolveErr := resolveRunTask(checker, latest)
	if resolveErr == nil {
		latest = resolved
	}
	snapshot, capabilityErr := resolveRunCapabilities(checker, latest, startedAt)
	run, paths, archive, err := e.createRunArchive(latest, trigger, due, runID, startedAt, snapshot)
	if err != nil {
		return run.RunID, err
	}
	if trigger == TriggerSchedule {
		e.consumeScheduledDeferred(latest, due)
	}
	defer archive.Close()
	finished, err := e.evaluateRunConditions(ctx, archive, checker, latest, due, resolveErr, capabilityErr, &run)
	if err != nil || finished {
		return run.RunID, err
	}
	return e.dispatchAutomationRun(ctx, archive, latest, paths, run)
}

func (e *Engine) createRunArchive(latest Task, trigger Trigger, due DueResult, runID string, startedAt time.Time, snapshot CapabilitySnapshot) (RunManifest, RunPaths, *ArchiveSink, error) {
	run, paths, err := e.store.createRun(latest, string(trigger), due.SkipReason, e.owner, runID, startedAt)
	if err != nil {
		return RunManifest{}, RunPaths{}, nil, err
	}
	run.TaskRevision = latest.Revision
	run.ScheduledFor = due.NextRunAt.UTC()
	snapshot.ApprovalMode = latest.Execution.ApprovalMode
	run.Capabilities = snapshot
	run.Delivery = latest.Delivery
	if err := e.store.SaveRunManifest(run); err != nil {
		return run, paths, nil, err
	}
	e.emitChange(EngineChange{Kind: "run", TaskID: run.TaskID, RunID: run.RunID, Status: string(run.Status), Trigger: run.Trigger, At: startedAt})
	archive, err := NewArchiveSink(paths.Events, ArchiveEventMeta{TaskID: run.TaskID, RunID: run.RunID}, &archiveChangeSink{
		engine: e, taskID: run.TaskID, runID: run.RunID,
	})
	if err != nil {
		err = e.finishRun(context.Background(), latest.ID, run, failedRunResult(err), err)
		return run, paths, nil, err
	}
	_ = archive.EmitRunStatus(RunStatusQueued)
	return run, paths, archive, nil
}

func (e *Engine) evaluateRunConditions(ctx context.Context, archive *ArchiveSink, checker Checker, latest Task, due DueResult, resolveErr, capabilityErr error, run *RunManifest) (bool, error) {
	if err := run.Transition(RunStatusCheckingCondition, "", e.now()); err != nil {
		return false, err
	}
	if err := e.store.SaveRunManifest(*run); err != nil {
		return false, err
	}
	e.emitChange(EngineChange{Kind: "run", TaskID: run.TaskID, RunID: run.RunID, Status: string(run.Status), Trigger: run.Trigger, At: e.now()})
	_ = archive.EmitRunStatus(RunStatusCheckingCondition)
	if capabilityErr != nil {
		run.Condition = conditionBlocked(e.now().UTC(), "capability unavailable", capabilityErr.Error())
	} else if resolveErr != nil {
		run.Condition = conditionBlocked(e.now().UTC(), "model unavailable", resolveErr.Error())
	} else {
		run.Condition = checker.Check(ctx, latest, due, e.now())
	}
	if !run.Condition.ShouldRun {
		err := e.finishRunWithArchive(ctx, archive, latest.ID, *run, conditionRunResult(run.Condition), nil)
		return true, err
	}
	run.ConditionReason = run.Condition.Reason
	return false, nil
}

func (e *Engine) dispatchAutomationRun(ctx context.Context, archive *ArchiveSink, latest Task, paths RunPaths, run RunManifest) (string, error) {
	if err := run.Transition(RunStatusRunning, "", e.now()); err != nil {
		return run.RunID, err
	}
	if err := e.store.SaveRunManifest(run); err != nil {
		return run.RunID, err
	}
	e.emitChange(EngineChange{Kind: "run", TaskID: run.TaskID, RunID: run.RunID, Status: string(run.Status), At: e.now()})
	_ = archive.EmitRunStatus(RunStatusRunning)

	result, runErr := e.runner.Run(ctx, RunRequest{
		Task:     latest,
		Manifest: run,
		RunDir:   paths.Dir,
		Trigger:  run.Trigger,
		Owner:    e.owner,
		Sink:     archive,
	})
	return run.RunID, e.finishRunWithArchive(ctx, archive, latest.ID, run, result, runErr)
}

func resolveRunTask(checker Checker, task Task) (Task, error) {
	if checker.TaskResolver == nil {
		return task, nil
	}
	return checker.TaskResolver(task)
}

func resolveRunCapabilities(checker Checker, task Task, capturedAt time.Time) (CapabilitySnapshot, error) {
	fallback := CapabilitySnapshot{
		ModelRef:     task.Capabilities.ModelRef,
		SkillIDs:     append([]string(nil), task.Capabilities.SkillIDs...),
		ExpertID:     task.Capabilities.ExpertID,
		ConnectorIDs: append([]string(nil), task.Capabilities.ConnectorIDs...),
		CapturedAt:   capturedAt.UTC(),
	}
	if checker.CapabilityResolver == nil {
		return fallback, nil
	}
	return checker.CapabilityResolver(task, capturedAt.UTC())
}

func (e *Engine) conditionChecker() Checker {
	checker := e.checker
	if checker.RecentRuns == nil {
		checker.RecentRuns = e.store.ListRuns
	}
	return checker
}

func conditionRunResult(condition ConditionResult) RunResult {
	status := RunStatusSkipped
	code := RunErrorConditionRejected
	message := condition.Reason
	if condition.Error != "" {
		status = RunStatusBlocked
		code = classifyConditionError(condition)
		message = condition.Error
	}
	return RunResult{
		Status:       status,
		Summary:      condition.Reason,
		ErrorCode:    code,
		ErrorMessage: message,
		Error:        condition.Error,
	}
}

func (e *Engine) finishRun(ctx context.Context, taskID string, run RunManifest, result RunResult, runErr error) error {
	status := result.Status
	if status == "" {
		status = RunStatusSucceeded
	}
	if err := run.Transition(status, result.Summary, e.now()); err != nil {
		return err
	}
	if result.SessionPath != "" {
		run.SessionPath = result.SessionPath
	}
	if result.ResultPath != "" {
		run.ResultPath = result.ResultPath
	}
	run.Summary = result.Summary
	if run.Condition.Reason != "" {
		run.ConditionReason = run.Condition.Reason
	}
	if run.Status == RunStatusSkipped && run.SkipReason == "" {
		run.SkipReason = result.Summary
	}
	run.ErrorCode = result.ErrorCode
	run.ErrorMessage = result.ErrorMessage
	run.Error = result.Error
	if run.Error == "" {
		run.Error = run.ErrorMessage
	}
	run.Usage = result.Usage
	if runErr != nil {
		classified := classifyRunError(runErr)
		if result.Status != "" {
			run.Status = result.Status
		} else {
			run.Status = classified.Status
		}
		if run.ErrorCode == "" {
			run.ErrorCode = classified.ErrorCode
		}
		if run.Error == "" {
			run.Error = runErr.Error()
		}
		if run.ErrorMessage == "" {
			run.ErrorMessage = runErr.Error()
		}
	}
	return e.persistFinishedRun(ctx, taskID, run)
}

func (e *Engine) persistFinishedRun(ctx context.Context, taskID string, run RunManifest) error {
	if err := e.store.SaveRunManifest(run); err != nil {
		return err
	}
	e.emitChange(EngineChange{Kind: "run", TaskID: run.TaskID, RunID: run.RunID, Status: string(run.Status), Trigger: run.Trigger, At: run.FinishedAt})
	var deliveryErr error
	if run.Status == RunStatusSucceeded && run.Delivery.Enabled {
		deliveryErr = e.enqueueRunDeliveries(ctx, run)
	}
	if !shouldRecordTaskRun(run) {
		return deliveryErr
	}
	recordedAt := e.runRecordTime(taskID, run)
	recordErr := e.recordTaskRun(taskID, run.RunID, recordedAt)
	return errors.Join(recordErr, deliveryErr)
}

func (e *Engine) runRecordTime(taskID string, run RunManifest) time.Time {
	if run.Trigger != TriggerSchedule || run.ScheduledFor.IsZero() {
		return run.FinishedAt
	}
	task, err := e.store.Task(taskID)
	if err == nil && task.MissedRunPolicy == MissedRunCatchUp {
		return run.ScheduledFor
	}
	return run.FinishedAt
}

func (e *Engine) enqueueRunDeliveries(ctx context.Context, run RunManifest) error {
	paths, err := e.store.RunPaths(run.TaskID, run.RunID)
	if err != nil {
		return err
	}
	store, err := OpenDeliveryStore(paths.Deliveries)
	if err != nil {
		return err
	}
	queuedAt := run.FinishedAt
	if queuedAt.IsZero() {
		queuedAt = e.now().UTC()
	}
	var deliveryErr error
	for index, target := range run.Delivery.Targets {
		key := fmt.Sprintf("%s:%s:%s:%s", run.TaskID, run.RunID, target.Type, target.Ref)
		attempt, created, err := store.Enqueue(DeliveryAttemptInput{
			ID: fmt.Sprintf("%s-%d", run.RunID, index+1), IdempotencyKey: key,
			TaskID: run.TaskID, RunID: run.RunID, Target: target,
		}, queuedAt)
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, err)
			continue
		}
		if created {
			if err := e.dispatchDelivery(ctx, store, attempt, run); err != nil {
				deliveryErr = errors.Join(deliveryErr, err)
			}
			if delivered, ok := store.Get(attempt.ID); ok && delivered.Status == DeliveryFailed {
				deliveryErr = errors.Join(deliveryErr, errors.New(delivered.Failure.Message))
			}
		}
	}
	return deliveryErr
}

func (e *Engine) dispatchDelivery(ctx context.Context, store *DeliveryStore, attempt DeliveryAttempt, run RunManifest) error {
	sending, err := attempt.MarkSending(e.now())
	if err != nil {
		return err
	}
	if err := store.Append(sending); err != nil {
		return err
	}
	e.emitChange(EngineChange{Kind: "delivery", TaskID: sending.TaskID, RunID: sending.RunID, DeliveryID: sending.ID, Status: string(sending.Status), At: sending.StartedAt})
	e.adapterMu.RLock()
	adapter := e.deliveryAdapters[strings.ToLower(strings.TrimSpace(attempt.Target.Type))]
	e.adapterMu.RUnlock()
	if adapter == nil {
		return e.recordDeliveryFailure(store, sending, DeliveryFailure{Class: DeliveryFailurePermanent, Code: "adapter_unavailable", Message: "delivery adapter is unavailable"})
	}
	result, deliverErr := adapter.Deliver(ctx, DeliveryRequest{
		AttemptID: attempt.ID, TaskID: attempt.TaskID, RunID: attempt.RunID,
		WorkspaceRoot: run.WorkspaceRoot, Principal: run.Capabilities.Principal,
		Target: attempt.Target, Body: run.Summary,
	})
	if deliverErr != nil {
		return e.recordDeliveryFailure(store, sending, deliveryFailureMetadata(deliverErr))
	}
	succeeded, err := sending.MarkSucceeded(e.now(), result.ProviderMessageID)
	if err != nil {
		return err
	}
	if err := store.Append(succeeded); err != nil {
		return err
	}
	e.emitChange(EngineChange{Kind: "delivery", TaskID: succeeded.TaskID, RunID: succeeded.RunID, DeliveryID: succeeded.ID, Status: string(succeeded.Status), At: succeeded.FinishedAt})
	return nil
}

func deliveryFailureMetadata(err error) DeliveryFailure {
	failure := DeliveryFailure{Class: DeliveryFailureTemporary, Code: "delivery_failed", Message: "delivery is temporarily unavailable"}
	var classified ClassifiedDeliveryError
	if errors.As(err, &classified) {
		return classified.DeliveryFailureMetadata()
	}
	if errors.Is(err, context.Canceled) {
		return DeliveryFailure{Class: DeliveryFailurePermanent, Code: "delivery_canceled", Message: "delivery was canceled"}
	}
	return failure
}

func (e *Engine) recordDeliveryFailure(store *DeliveryStore, sending DeliveryAttempt, failure DeliveryFailure) error {
	failed, err := sending.MarkFailed(failure, e.now())
	if err != nil {
		return err
	}
	if err := store.Append(failed); err != nil {
		return err
	}
	e.emitChange(EngineChange{Kind: "delivery", TaskID: failed.TaskID, RunID: failed.RunID, DeliveryID: failed.ID, Status: string(failed.Status), At: failed.FinishedAt})
	return nil
}

// RetryDelivery creates a new immutable attempt for a retryable failure. The
// original attempt remains in the append-only history.
func (e *Engine) RetryDelivery(ctx context.Context, taskID, runID, attemptID string) (DeliveryAttempt, error) {
	e.deliveryMu.Lock()
	defer e.deliveryMu.Unlock()
	if err := ctx.Err(); err != nil {
		return DeliveryAttempt{}, err
	}
	paths, err := e.store.RunPaths(taskID, runID)
	if err != nil {
		return DeliveryAttempt{}, err
	}
	store, err := OpenDeliveryStore(paths.Deliveries)
	if err != nil {
		return DeliveryAttempt{}, err
	}
	prior, err := retryableDelivery(store, taskID, runID, attemptID)
	if err != nil {
		return DeliveryAttempt{}, err
	}
	attempt, created, err := e.enqueueDeliveryRetry(store, prior)
	if err != nil {
		return DeliveryAttempt{}, err
	}
	if !created && attempt.Status != DeliveryQueued {
		return attempt, nil
	}
	manifest, err := e.store.ReadRunManifest(taskID, runID)
	if err != nil {
		return DeliveryAttempt{}, err
	}
	if err := e.dispatchDelivery(ctx, store, attempt, manifest); err != nil {
		return DeliveryAttempt{}, err
	}
	result, _ := store.Get(attempt.ID)
	return result, nil
}

func retryableDelivery(store *DeliveryStore, taskID, runID, attemptID string) (DeliveryAttempt, error) {
	prior, ok := store.Get(attemptID)
	if !ok || prior.RunID != runID || prior.TaskID != taskID {
		return DeliveryAttempt{}, fmt.Errorf("%w: delivery attempt not found", ErrInvalidDelivery)
	}
	if prior.Status != DeliveryFailed || !prior.Retryable {
		return DeliveryAttempt{}, fmt.Errorf("%w: delivery attempt is not retryable", ErrInvalidDelivery)
	}
	return prior, nil
}

func (e *Engine) enqueueDeliveryRetry(store *DeliveryStore, prior DeliveryAttempt) (DeliveryAttempt, bool, error) {
	attempt, created, err := store.Enqueue(DeliveryAttemptInput{
		ID:             prior.ID + "-retry",
		IdempotencyKey: prior.IdempotencyKey + ":retry",
		TaskID:         prior.TaskID, RunID: prior.RunID, Target: prior.Target,
	}, e.now())
	return attempt, created, err
}

func shouldRecordTaskRun(run RunManifest) bool {
	return run.Status == RunStatusSucceeded || (run.Trigger == TriggerSchedule && run.Status == RunStatusSkipped)
}

func (e *Engine) finishRunWithArchive(ctx context.Context, archive *ArchiveSink, taskID string, run RunManifest, result RunResult, runErr error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultRunCancellationWaitTimeout)
	defer cancel()
	status := result.Status
	if status == "" {
		status = statusFromRunErr(runErr)
	}
	if archive.Err() != nil {
		status = RunStatusFailed
		if result.ErrorCode == "" {
			result.ErrorCode = RunErrorArchiveFailed
		}
		if result.ErrorMessage == "" {
			result.ErrorMessage = archive.Err().Error()
		}
	}
	_ = archive.EmitRunStatus(status)
	result.Status = status
	closeErr := archive.Close()
	if closeErr != nil {
		result.Status = RunStatusFailed
		if result.ErrorCode == "" {
			result.ErrorCode = RunErrorArchiveFailed
		}
		if result.ErrorMessage == "" {
			result.ErrorMessage = closeErr.Error()
		}
	}
	runErr = errors.Join(runErr, closeErr)
	err := e.finishRun(ctx, taskID, run, result, runErr)
	e.running.recordCleanupError(errors.Join(closeErr, err))
	return err
}

func classifyConditionError(condition ConditionResult) RunErrorCode {
	reason := strings.ToLower(condition.Reason)
	switch {
	case strings.Contains(reason, "workspace"):
		return RunErrorWorkspaceMissing
	case strings.Contains(reason, "model"):
		return RunErrorModelUnavailable
	case strings.Contains(reason, "invalid json"), strings.Contains(reason, "missing should_run"):
		return RunErrorConditionInvalidJSON
	case strings.Contains(reason, "canceled"):
		return RunErrorCanceled
	default:
		return RunErrorConditionRejected
	}
}

func (e *Engine) reloadTask(taskID string) (Task, bool, error) {
	tasks, err := e.store.LoadTasks()
	if err != nil {
		return Task{}, false, err
	}
	for _, task := range tasks {
		if task.ID == taskID {
			return task, true, nil
		}
	}
	return Task{}, false, nil
}

func (e *Engine) recordTaskRun(taskID, runID string, at time.Time) error {
	return e.store.RecordTaskRun(taskID, runID, at)
}

func (e *Engine) taskHasRunningRun(taskID, runID string) (bool, error) {
	runs, err := e.store.ListRuns(taskID, 0)
	if err != nil {
		return false, err
	}
	runID = strings.TrimSpace(runID)
	for _, run := range runs {
		if run.Status == RunStatusRunning && run.RunID == runID {
			return true, nil
		}
	}
	return false, nil
}
