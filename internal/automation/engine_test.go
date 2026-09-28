package automation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
)

type mockRunner struct {
	mu           sync.Mutex
	calls        int
	result       RunResult
	err          error
	events       []event.Event
	breakArchive bool
	started      chan struct{}
	block        chan struct{}
}

type deliveryAdapterFunc func(context.Context, DeliveryRequest) (DeliveryResult, error)

func (fn deliveryAdapterFunc) Deliver(ctx context.Context, req DeliveryRequest) (DeliveryResult, error) {
	return fn(ctx, req)
}

type classifiedDeliveryTestError struct{}

func (classifiedDeliveryTestError) Error() string { return "unsafe provider diagnostic" }
func (classifiedDeliveryTestError) DeliveryFailureMetadata() DeliveryFailure {
	return DeliveryFailure{Class: DeliveryFailureRateLimited, Code: "provider_rate_limited", Message: "delivery was rate limited"}
}

func (r *mockRunner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	if r.started != nil {
		close(r.started)
	}
	if r.block != nil {
		select {
		case <-ctx.Done():
			return RunResult{}, ctx.Err()
		case <-r.block:
		}
	}
	for _, ev := range r.events {
		req.Sink.Emit(ev)
	}
	if r.breakArchive {
		if closer, ok := req.Sink.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		req.Sink.Emit(event.Event{Kind: event.Text, Text: "after close"})
	}
	return r.result, r.err
}

func (r *mockRunner) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func TestEngineTickRunsDueTask(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.count())
	}
	runs, err := store.ListRuns("task-1", 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusSucceeded {
		t.Fatalf("runs = %+v, want one succeeded run", runs)
	}
	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if tasks[0].LastRunID == "" || !tasks[0].LastRunAt.Equal(now) {
		t.Fatalf("task run fields = %+v, want last run at %s", tasks[0], now)
	}
}

func TestEngineRecordsRunnerResultMetadata(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{result: RunResult{
		Status:     RunStatusSucceeded,
		Summary:    "final summary",
		ResultPath: "runs/task-1/run-1/result.md",
		Usage:      &provider.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
	}}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns("task-1", 1)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	manifest, err := store.ReadRunManifest("task-1", runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.ResultPath != "runs/task-1/run-1/result.md" || manifest.Summary != "final summary" {
		t.Fatalf("manifest result fields = %+v, want result path and summary", manifest)
	}
	if manifest.Usage == nil || manifest.Usage.TotalTokens != 5 {
		t.Fatalf("manifest usage = %+v, want total tokens", manifest.Usage)
	}
}

func TestEngineSkipsDisabled(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   false,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
}

func TestEngineNoTasksDoesNotRun(t *testing.T) {
	store := newEngineTestStore(t)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	if _, err := os.Stat(filepath.Join(store.dir, runsDirName)); !os.IsNotExist(err) {
		t.Fatalf("runs dir stat error = %v, want not exists", err)
	}
}

func TestEngineDisabledTasksDoNotRun(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t,
		Task{
			ID:        "task-1",
			Name:      "Task 1",
			Enabled:   false,
			Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		},
		Task{
			ID:        "task-2",
			Name:      "Task 2",
			Enabled:   false,
			Schedule:  Schedule{Kind: ScheduleKindInterval, Every: "1m"},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		},
	)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	if _, err := os.Stat(filepath.Join(store.dir, runsDirName)); !os.IsNotExist(err) {
		t.Fatalf("runs dir stat error = %v, want not exists", err)
	}
}

func TestEngineSkipsNotDue(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
}

func TestEngineMissedRunSkipRecordsSkipWithoutRunner(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "task-skip", Name: "Skip missed", Enabled: true,
		Schedule:        Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-2 * time.Hour)},
		MissedRunPolicy: MissedRunSkip, CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:skip", runner)
	engine.now = func() time.Time { return now }
	capabilityResolutions := 0
	engine.checker.CapabilityResolver = func(got Task, capturedAt time.Time) (CapabilitySnapshot, error) {
		capabilityResolutions++
		if got.ID != task.ID || !capturedAt.Equal(now) {
			t.Fatalf("capability request = task %q at %s", got.ID, capturedAt)
		}
		return CapabilitySnapshot{ModelRef: "model-skip", CapturedAt: capturedAt}, nil
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	if capabilityResolutions != 1 {
		t.Fatalf("capability resolutions = %d, want 1", capabilityResolutions)
	}
	runs, _ := store.ListRuns(task.ID, 1)
	if len(runs) != 1 || runs[0].Status != RunStatusSkipped || runs[0].Summary != "missed run skipped" {
		t.Fatalf("runs = %+v", runs)
	}
	manifest, err := store.ReadRunManifest(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.Capabilities.ModelRef != "model-skip" || !manifest.Capabilities.CapturedAt.Equal(now) {
		t.Fatalf("capability snapshot = %+v", manifest.Capabilities)
	}
	saved, _ := store.LoadTasks()
	if saved[0].LastRunID == "" || !saved[0].LastRunAt.Equal(now) {
		t.Fatalf("task metadata not advanced after skipped occurrence: %+v", saved[0])
	}
}

func TestEngineMissedRunSkipPersistsBlockedCapabilityResolution(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "task-skip-capability", Name: "Skip missed with revoked capability", Enabled: true,
		Schedule:        Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-2 * time.Hour)},
		MissedRunPolicy: MissedRunSkip, CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:skip-capability", runner)
	engine.now = func() time.Time { return now }
	engine.checker.CapabilityResolver = func(Task, time.Time) (CapabilitySnapshot, error) {
		return CapabilitySnapshot{
			ModelRef: "model-revoked", ToolAllowlist: []string{"mcp__connector-a__read"}, CapturedAt: now,
		}, fmt.Errorf("%w: connector-a was revoked", ErrCapabilityUnavailable)
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	runs, err := store.ListRuns(task.ID, 1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns = %+v, %v", runs, err)
	}
	manifest, err := store.ReadRunManifest(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.Status != RunStatusBlocked || manifest.Condition.Reason != "capability unavailable" {
		t.Fatalf("manifest status/condition = %s/%+v", manifest.Status, manifest.Condition)
	}
	if manifest.Capabilities.ModelRef != "model-revoked" || len(manifest.Capabilities.ToolAllowlist) != 1 {
		t.Fatalf("capability snapshot = %+v", manifest.Capabilities)
	}
	saved, err := store.Task(task.ID)
	if err != nil {
		t.Fatalf("Task: %v", err)
	}
	if saved.LastRunID != "" || !saved.LastRunAt.IsZero() {
		t.Fatalf("task metadata advanced after blocked capability resolution: %+v", saved)
	}
}

func TestEngineCatchUpAdvancesByScheduledOccurrence(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	created := now.Add(-3 * time.Hour)
	task := Task{ID: "task-catch-up", Name: "Catch up", Enabled: true,
		Schedule:        Schedule{Kind: ScheduleKindInterval, Every: "1h"},
		MissedRunPolicy: MissedRunCatchUp, CreatedAt: created, UpdatedAt: created}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:catch-up", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	saved, _ := store.LoadTasks()
	want := created.Add(3 * time.Hour)
	if !saved[0].LastRunAt.Equal(want) {
		t.Fatalf("lastRunAt = %s, want bounded catch-up through %s", saved[0].LastRunAt, want)
	}
	if runner.count() != maxCatchUpRunsPerTick {
		t.Fatalf("runner calls = %d, want bounded batch %d", runner.count(), maxCatchUpRunsPerTick)
	}
}

func TestEngineDeferredScheduledTaskDoesNotBlockOtherTasks(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	cloudTask := Task{ID: "cloud-task", Name: "Cloud task", Enabled: true, ProjectID: "project-1",
		Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)}, MissedRunPolicy: MissedRunCatchUp,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	localTask := Task{ID: "local-task", Name: "Local task", Enabled: true, WorkspaceRoot: t.TempDir(),
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, cloudTask, localTask)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:preflight", runner)
	engine.now = func() time.Time { return now }
	cloudReady := false
	cloudChecks := 0
	engine.checker.Preflight = func(task Task) PreflightResult {
		if task.ID == cloudTask.ID {
			cloudChecks++
			if !cloudReady {
				return PreflightResult{Disposition: PreflightDefer, Reason: "cloud project services are initializing"}
			}
		}
		return PreflightResult{Disposition: PreflightReady}
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("deferred Tick: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want local task only", runner.count())
	}
	if cloudChecks != 1 {
		t.Fatalf("cloud preflight checks = %d, want one check without catch-up spin", cloudChecks)
	}
	assertTaskHasNoRuns(t, store, cloudTask.ID)
	deferred, err := store.Task(cloudTask.ID)
	if err != nil {
		t.Fatalf("Task: %v", err)
	}
	if deferred.LastRunID != "" || !deferred.LastRunAt.IsZero() {
		t.Fatalf("deferred task metadata = %+v, want no recorded run", deferred)
	}

	cloudReady = true
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("ready Tick: %v", err)
	}
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("idempotency Tick: %v", err)
	}
	if runner.count() != 2 {
		t.Fatalf("runner calls = %d, want each task exactly once", runner.count())
	}
	runs, err := store.ListRuns(cloudTask.ID, 0)
	if err != nil || len(runs) != 1 || runs[0].Status != RunStatusSucceeded {
		t.Fatalf("cloud runs = %+v, %v; want one succeeded run", runs, err)
	}
}

func TestEngineManualDeferredTaskReturnsTypedErrorWithoutRun(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "cloud-manual", Name: "Cloud manual", Enabled: true, ProjectID: "project-1",
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:manual-preflight", &mockRunner{})
	engine.now = func() time.Time { return now }
	engine.checker.Preflight = func(Task) PreflightResult {
		return PreflightResult{Disposition: PreflightDefer, Reason: "cloud project services are initializing"}
	}

	runID, err := engine.Trigger(context.Background(), TriggerRequest{TaskID: task.ID})
	if runID != "" {
		t.Fatalf("run ID = %q, want empty", runID)
	}
	if !errors.Is(err, ErrDependencyInitializing) {
		t.Fatalf("Trigger error = %v, want ErrDependencyInitializing", err)
	}
	var initializing *DependencyInitializingError
	if !errors.As(err, &initializing) || initializing.TaskID != task.ID {
		t.Fatalf("Trigger error = %#v, want typed task initialization error", err)
	}
	assertTaskHasNoRuns(t, store, task.ID)
}

func TestEngineUnknownPreflightDispositionDefers(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "unknown-preflight", Name: "Unknown preflight", Enabled: true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:unknown-preflight", &mockRunner{})
	engine.now = func() time.Time { return now }
	engine.checker.Preflight = func(Task) PreflightResult {
		return PreflightResult{Disposition: PreflightDisposition(99)}
	}

	if _, err := engine.Trigger(context.Background(), TriggerRequest{TaskID: task.ID}); !errors.Is(err, ErrDependencyInitializing) {
		t.Fatalf("Trigger error = %v, want unknown readiness to defer", err)
	}
	assertTaskHasNoRuns(t, store, task.ID)
}

func TestEngineRechecksPreflightAfterLeaseAndTaskReload(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "cloud-revision", Name: "Cloud revision", Enabled: true, ProjectID: "project-1",
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:recheck", runner)
	engine.now = func() time.Time { return now }
	checks := 0
	engine.checker.Preflight = func(got Task) PreflightResult {
		checks++
		if checks == 1 {
			// Simulate a revisioned edit after the first authoritative reload.
			edited := got
			edited.Name = "Cloud revision updated"
			if _, err := store.UpdateTask(edited, got.Revision); err != nil {
				t.Fatalf("UpdateTask during preflight: %v", err)
			}
			return PreflightResult{Disposition: PreflightReady}
		}
		if got.Revision != 2 {
			t.Fatalf("second preflight revision = %d, want 2", got.Revision)
		}
		return PreflightResult{Disposition: PreflightDefer, Reason: "readiness changed while acquiring the lease"}
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if checks != 2 {
		t.Fatalf("preflight checks = %d, want pre-lease and post-reload checks", checks)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want none after second preflight deferred", runner.count())
	}
	assertTaskHasNoRuns(t, store, task.ID)
}

func TestEngineReadinessDeferralDoesNotTurnMissedRunSkipIntoSkip(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "cloud-skip", Name: "Cloud skip", Enabled: true, ProjectID: "project-1",
		Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-10 * time.Second)}, MissedRunPolicy: MissedRunSkip,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:deferred-skip", runner)
	engine.tickInterval = 30 * time.Second
	engine.now = func() time.Time { return now }
	ready := false
	engine.checker.Preflight = func(Task) PreflightResult {
		if !ready {
			return PreflightResult{Disposition: PreflightDefer, Reason: "cloud project services are initializing"}
		}
		return PreflightResult{Disposition: PreflightReady}
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("deferred Tick: %v", err)
	}
	now = now.Add(time.Minute)
	ready = true
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("ready Tick: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want deferred occurrence executed", runner.count())
	}
	runs, err := store.ListRuns(task.ID, 0)
	if err != nil || len(runs) != 1 || runs[0].Status != RunStatusSucceeded {
		t.Fatalf("runs = %+v, %v; want one succeeded run instead of missed-run skip", runs, err)
	}
}

func TestEngineReadinessDeferralSurvivesArchiveSetupFailure(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "cloud-archive-retry", Name: "Cloud archive retry", Enabled: true, ProjectID: "project-1",
		Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-10 * time.Second)}, MissedRunPolicy: MissedRunSkip,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:archive-retry", runner)
	engine.tickInterval = 30 * time.Second
	engine.now = func() time.Time { return now }
	ready := false
	engine.checker.Preflight = func(Task) PreflightResult {
		if !ready {
			return PreflightResult{Disposition: PreflightDefer}
		}
		return PreflightResult{Disposition: PreflightReady}
	}
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("deferred Tick: %v", err)
	}

	now = now.Add(time.Minute)
	ready = true
	runsPath := filepath.Join(store.dir, runsDirName)
	if err := os.WriteFile(runsPath, []byte("blocks run directory"), 0o600); err != nil {
		t.Fatalf("WriteFile runs blocker: %v", err)
	}
	if err := engine.Tick(context.Background()); err == nil {
		t.Fatal("Tick with blocked run directory succeeded")
	}
	if err := os.Remove(runsPath); err != nil {
		t.Fatalf("Remove runs blocker: %v", err)
	}
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("retry Tick: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want occurrence executed after archive recovery", runner.count())
	}
}

func TestEngineRestartReestablishesDeferredOccurrence(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "cloud-restart", Name: "Cloud restart", Enabled: true, ProjectID: "project-1",
		Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-10 * time.Second)}, MissedRunPolicy: MissedRunSkip,
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	pending := func(Task) PreflightResult { return PreflightResult{Disposition: PreflightDefer} }
	first := newEngineTestEngine(store, "test:before-restart", &mockRunner{})
	first.now = func() time.Time { return now }
	first.checker.Preflight = pending
	if err := first.Tick(context.Background()); err != nil {
		t.Fatalf("pre-restart Tick: %v", err)
	}

	now = now.Add(time.Minute)
	runner := &mockRunner{}
	restarted := newEngineTestEngine(store, "test:after-restart", runner)
	restarted.now = func() time.Time { return now }
	restarted.checker.Preflight = pending
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("post-restart pending Tick: %v", err)
	}
	restarted.checker.Preflight = func(Task) PreflightResult { return PreflightResult{Disposition: PreflightReady} }
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("post-restart ready Tick: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want restarted engine to execute deferred occurrence", runner.count())
	}
}

func TestTwoEnginesOneRun(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{started: make(chan struct{}), block: make(chan struct{})}
	engine1 := newEngineTestEngine(store, "test:1", runner)
	engine2 := newEngineTestEngine(store, "test:2", runner)
	engine1.now = func() time.Time { return now }
	engine2.now = func() time.Time { return now }

	errs := make(chan error, 2)
	go func() { errs <- engine1.Tick(context.Background()) }()
	<-runner.started
	go func() { errs <- engine2.Tick(context.Background()) }()
	close(runner.block)
	if err := <-errs; err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}

	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.count())
	}
	runs, err := store.ListRuns("task-1", 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1: %+v", len(runs), runs)
	}
}

func TestEngineScheduledOccurrenceIsIdempotentAfterArchiveBeforeTaskMetadata(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "task-crash", Name: "Crash recovery", Enabled: true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	saved, _ := store.Task(task.ID)
	due := Due(saved, now)
	runID := scheduledRunID(saved, due)
	manifest, _, err := store.createRun(saved, string(TriggerSchedule), "scheduled", "crashed-owner", runID, now.Add(-30*time.Second))
	if err != nil {
		t.Fatalf("create interrupted run: %v", err)
	}
	manifest.ScheduledFor = due.NextRunAt
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}

	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "restarted-owner", runner)
	engine.now = func() time.Time { return now }
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick after restart: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want existing occurrence reused", runner.count())
	}
	runs, _ := store.ListRuns(task.ID, 0)
	if len(runs) != 1 || runs[0].RunID != runID {
		t.Fatalf("runs = %+v, want one deterministic occurrence", runs)
	}
	updated, _ := store.Task(task.ID)
	if updated.LastRunID != runID || !updated.LastRunAt.Equal(due.NextRunAt) {
		t.Fatalf("task metadata was not repaired: %+v", updated)
	}
}

func TestEngineFailedRun(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{err: errors.New("runner failed")}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns("task-1", 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusFailed || runs[0].Error != "runner failed" {
		t.Fatalf("runs = %+v, want one failed run", runs)
	}
	if runs[0].ErrorCode != RunErrorRunnerFailed || runs[0].ErrorMessage != "runner failed" {
		t.Fatalf("run error = %q/%q, want runner_failed/runner failed", runs[0].ErrorCode, runs[0].ErrorMessage)
	}
}

func TestEngineFinishesRunnerSkippedAfterRunning(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{result: RunResult{
		Status:       RunStatusSkipped,
		ErrorCode:    RunErrorLeaseUnavailable,
		ErrorMessage: "automation topic is busy",
		Summary:      "automation topic is busy",
	}}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns("task-1", 1)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusSkipped || runs[0].SkipReason != "automation topic is busy" {
		t.Fatalf("runs = %+v, want one skipped busy run", runs)
	}
}

func TestEngineRecordsSkippedScheduledRunToAvoidRepeatedTicks(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.WorkspaceRoot = t.TempDir()
	task.Conditions.ConditionPrompt = "Run only when needed."
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	model := &mockConditionModel{raw: `{"should_run": false, "reason": "nothing changed"}`}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }
	engine.checker = Checker{
		ConditionModel: model,
		ModelResolver:  func(Task) error { return nil },
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick returned error: %v", err)
	}
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	if model.calls != 1 {
		t.Fatalf("condition model calls = %d, want 1", model.calls)
	}
	runs, err := store.ListRuns(task.ID, 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusSkipped {
		t.Fatalf("runs = %+v, want one skipped run", runs)
	}
	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if tasks[0].LastRunID != runs[0].RunID || !tasks[0].LastRunAt.Equal(now) {
		t.Fatalf("task run fields = %+v, want skipped run recorded at %s", tasks[0], now)
	}
}

func TestEngineArchivesRunEvents(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{events: []event.Event{
		{Kind: event.TurnStarted},
		{Kind: event.Notice, Level: event.LevelWarn, Text: "heads up"},
		{Kind: event.TurnDone},
	}}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns("task-1", 1)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	paths, err := store.RunPaths("task-1", runs[0].RunID)
	if err != nil {
		t.Fatalf("RunPaths: %v", err)
	}
	events, err := readArchivedEvents(paths.Events)
	if err != nil {
		t.Fatalf("readArchivedEvents: %v", err)
	}
	if !hasArchivedStatus(events, RunStatusQueued) || !hasArchivedStatus(events, RunStatusRunning) || !hasArchivedStatus(events, RunStatusSucceeded) {
		t.Fatalf("events missing automation statuses: %+v", events)
	}
	if !hasArchivedKind(events, event.TurnStarted) || !hasArchivedKind(events, event.TurnDone) {
		t.Fatalf("events missing turn lifecycle: %+v", events)
	}
}

func TestEngineRecordsArchiveWriteFailureInManifest(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:1", &mockRunner{breakArchive: true})
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	runs, err := store.ListRuns(task.ID, 1)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	manifest, err := store.ReadRunManifest(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.Status != RunStatusFailed || manifest.ErrorCode != RunErrorArchiveFailed || manifest.ErrorMessage == "" {
		t.Fatalf("manifest = %+v, want failed archive error", manifest)
	}
}

func TestEngineLeaseAcquireFailureDoesNotCreateFailedRun(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	if _, ok, err := store.AcquireLease("task-1", "other-run", "other:1", time.Minute); err != nil || !ok {
		t.Fatalf("AcquireLease ok=%v err=%v", ok, err)
	}
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	runs, err := store.ListRuns("task-1", 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("runs = %+v, want no failed run", runs)
	}
}

func TestEngineIgnoresStaleRunningRunAfterLeaseTakeover(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
	store := newEngineTestStore(t, task)
	run, _, err := store.CreateRun(task, string(TriggerSchedule), "other:1")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	run.Status = RunStatusRunning
	if err := store.SaveRunManifest(run); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.count())
	}
	runs, err := store.ListRuns(task.ID, 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	succeeded := 0
	running := 0
	for _, run := range runs {
		if run.Status == RunStatusSucceeded {
			succeeded++
		}
		if run.Status == RunStatusRunning {
			running++
		}
	}
	if len(runs) != 2 || succeeded != 1 || running != 1 {
		t.Fatalf("runs = %+v, want new succeeded run and old stale running run", runs)
	}
}

func TestEngineStartStop(t *testing.T) {
	store := newEngineTestStore(t)
	engine := newEngineTestEngine(store, "test:1", &mockRunner{})
	engine.tickInterval = time.Hour

	engine.Start(context.Background())
	engine.Stop()
}

func TestEngineFreezesCapabilitiesAndBlocksUnavailableSelection(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{
		ID: "task-capability", Name: "Capability task", Enabled: true, Revision: 4,
		Schedule:     Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		Capabilities: CapabilitySelection{ModelRef: "model-a", ConnectorIDs: []string{"connector-a"}},
		CreatedAt:    now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:capability", runner)
	engine.now = func() time.Time { return now }
	engine.checker.CapabilityResolver = func(Task, time.Time) (CapabilitySnapshot, error) {
		return CapabilitySnapshot{
			ModelRef: "model-a", ConnectorIDs: []string{"connector-a"},
			ToolAllowlist: []string{"mcp__connector-a__read"}, CapturedAt: now,
		}, fmt.Errorf("%w: connector-a was revoked", ErrCapabilityUnavailable)
	}

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want blocked before dispatch", runner.count())
	}
	runs, err := store.ListRuns(task.ID, 1)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ListRuns = %+v, %v", runs, err)
	}
	manifest, err := store.ReadRunManifest(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.Status != RunStatusBlocked || manifest.TaskRevision != 4 {
		t.Fatalf("manifest status/revision = %s/%d", manifest.Status, manifest.TaskRevision)
	}
	if len(manifest.Capabilities.ToolAllowlist) != 1 || manifest.Capabilities.ToolAllowlist[0] != "mcp__connector-a__read" {
		t.Fatalf("capability snapshot = %+v", manifest.Capabilities)
	}
	if manifest.Condition.Reason != "capability unavailable" {
		t.Fatalf("condition = %+v", manifest.Condition)
	}
}

func TestEngineQueuesDeliveryWithoutChangingSuccessfulRun(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	workspace := t.TempDir()
	task := Task{
		ID: "task-delivery", Name: "Delivery task", Enabled: true,
		Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		Delivery: DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{
			{Type: "local_test", Ref: "inbox"},
			{Type: "local_test", Ref: "archive"},
		}},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), WorkspaceRoot: workspace,
	}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:delivery", &mockRunner{result: RunResult{Summary: "done"}})
	engine.now = func() time.Time { return now }
	engine.checker.CapabilityResolver = func(Task, time.Time) (CapabilitySnapshot, error) {
		return CapabilitySnapshot{Principal: "user-1/profile-a", CapturedAt: now}, nil
	}
	localAdapter := engine.deliveryAdapters["local_test"].(*LocalDeliveryAdapter)
	var changes []EngineChange
	engine.SetChangeObserver(func(change EngineChange) { changes = append(changes, change) })

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	runs, err := store.ListRuns(task.ID, 1)
	if err != nil || len(runs) != 1 || runs[0].Status != RunStatusSucceeded {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	deliveries, err := store.ListDeliveries(task.ID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 2 || deliveries[0].Status != DeliverySucceeded || deliveries[0].Target.Ref != "inbox" || deliveries[1].Status != DeliverySucceeded || deliveries[1].Target.Ref != "archive" {
		t.Fatalf("deliveries = %+v", deliveries)
	}
	messages := localAdapter.Messages()
	if len(messages) != 2 || messages[0].Request.TaskID != task.ID || messages[0].Request.WorkspaceRoot != workspace || messages[0].Request.Principal != "user-1/profile-a" {
		t.Fatalf("delivery request ownership = %+v", messages)
	}
	var sawQueued, sawRunning, sawSucceeded, sawDelivery, sawRunEvent bool
	for _, change := range changes {
		switch {
		case change.Kind == "run" && change.Status == string(RunStatusQueued):
			sawQueued = true
		case change.Kind == "run" && change.Status == string(RunStatusRunning):
			sawRunning = true
		case change.Kind == "run" && change.Status == string(RunStatusSucceeded):
			sawSucceeded = true
		case change.Kind == "delivery" && change.Status == string(DeliverySucceeded):
			sawDelivery = true
		case change.Kind == "run_event" && change.Sequence > 0:
			sawRunEvent = true
		}
	}
	if !sawQueued || !sawRunning || !sawSucceeded || !sawDelivery || !sawRunEvent {
		t.Fatalf("engine changes = %+v, want run lifecycle and delivery completion", changes)
	}
}

func TestEngineContinuesDeliveryAfterTargetFailure(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{
		ID: "task-delivery-failure-isolation", Name: "Delivery failure isolation", Enabled: true,
		Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		Delivery: DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{
			{Type: "retry_test", Ref: "failing"},
			{Type: "retry_test", Ref: "succeeding"},
		}},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:delivery-failure-isolation", &mockRunner{result: RunResult{Summary: "done"}})
	engine.now = func() time.Time { return now }
	var delivered []string
	engine.deliveryAdapters["retry_test"] = deliveryAdapterFunc(func(_ context.Context, req DeliveryRequest) (DeliveryResult, error) {
		delivered = append(delivered, req.Target.Ref)
		if req.Target.Ref == "failing" {
			return DeliveryResult{}, classifiedDeliveryTestError{}
		}
		return DeliveryResult{ProviderMessageID: "sent"}, nil
	})

	err := engine.Tick(context.Background())
	if err == nil {
		t.Fatal("Tick error = nil, want aggregated delivery failure")
	}
	if strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("Tick error = %q, want sanitized delivery failure", err)
	}
	if got, want := strings.Join(delivered, ","), "failing,succeeding"; got != want {
		t.Fatalf("delivered targets = %q, want %q", got, want)
	}
	runs, listErr := store.ListRuns(task.ID, 1)
	if listErr != nil || len(runs) != 1 || runs[0].Status != RunStatusSucceeded {
		t.Fatalf("runs = %+v, %v", runs, listErr)
	}
	deliveries, listErr := store.ListDeliveries(task.ID, runs[0].RunID)
	if listErr != nil {
		t.Fatalf("ListDeliveries: %v", listErr)
	}
	if len(deliveries) != 2 || deliveries[0].Status != DeliveryFailed || deliveries[1].Status != DeliverySucceeded {
		t.Fatalf("deliveries = %+v", deliveries)
	}
}

func TestEngineRetriesDeliveryAsNewAttempt(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "task-retry", Name: "Retry delivery", Enabled: true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		Delivery:  DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: "retry_test", Ref: "inbox"}}},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:retry", &mockRunner{result: RunResult{Summary: "done"}})
	engine.now = func() time.Time { return now }
	engine.deliveryAdapters["retry_test"] = deliveryAdapterFunc(func(context.Context, DeliveryRequest) (DeliveryResult, error) {
		return DeliveryResult{}, classifiedDeliveryTestError{}
	})
	if err := engine.Tick(context.Background()); err == nil {
		t.Fatal("Tick error = nil, want delivery failure")
	}
	runs, _ := store.ListRuns(task.ID, 1)
	deliveries, _ := store.ListDeliveries(task.ID, runs[0].RunID)
	if len(deliveries) != 1 || !deliveries[0].Retryable || deliveries[0].Failure.Class != DeliveryFailureRateLimited || strings.Contains(deliveries[0].Failure.Message, "unsafe") {
		t.Fatalf("initial deliveries = %+v", deliveries)
	}
	engine.deliveryAdapters["retry_test"] = NewLocalDeliveryAdapter()
	retried, err := engine.RetryDelivery(context.Background(), task.ID, runs[0].RunID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("RetryDelivery: %v", err)
	}
	if retried.Status != DeliverySucceeded || retried.ID == deliveries[0].ID {
		t.Fatalf("retried attempt = %+v", retried)
	}
}

func TestEngineRetryDeliveryIsIdempotentPerFailedParent(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := Task{ID: "task-retry-idempotent", Name: "Idempotent retry", Enabled: true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		Delivery:  DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: "retry_test", Ref: "inbox"}}},
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	store := newEngineTestStore(t, task)
	engine := newEngineTestEngine(store, "test:retry-idempotent", &mockRunner{result: RunResult{Summary: "done"}})
	engine.now = func() time.Time { return now }
	sends := 0
	engine.deliveryAdapters["retry_test"] = deliveryAdapterFunc(func(context.Context, DeliveryRequest) (DeliveryResult, error) {
		sends++
		return DeliveryResult{}, classifiedDeliveryTestError{}
	})
	if err := engine.Tick(context.Background()); err == nil {
		t.Fatal("Tick error = nil, want delivery failure")
	}
	runs, _ := store.ListRuns(task.ID, 1)
	deliveries, _ := store.ListDeliveries(task.ID, runs[0].RunID)
	if len(deliveries) != 1 || !deliveries[0].Retryable || sends != 1 {
		t.Fatalf("initial deliveries/sends = %+v/%d", deliveries, sends)
	}

	firstChild, err := engine.RetryDelivery(context.Background(), task.ID, runs[0].RunID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("first RetryDelivery: %v", err)
	}
	repeated, err := engine.RetryDelivery(context.Background(), task.ID, runs[0].RunID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("repeated RetryDelivery: %v", err)
	}
	if repeated.ID != firstChild.ID || sends != 2 {
		t.Fatalf("repeated retry = %+v, first child = %+v, sends = %d; want the existing child and two total sends", repeated, firstChild, sends)
	}

	nextGeneration, err := engine.RetryDelivery(context.Background(), task.ID, runs[0].RunID, firstChild.ID)
	if err != nil {
		t.Fatalf("next-generation RetryDelivery: %v", err)
	}
	if nextGeneration.ID == firstChild.ID || sends != 3 {
		t.Fatalf("next generation = %+v, first child = %+v, sends = %d; want one new child send", nextGeneration, firstChild, sends)
	}
}

func newEngineTestStore(t *testing.T, tasks ...Task) *Store {
	t.Helper()
	store := NewStore(t.TempDir())
	if err := store.SaveTasks(tasks); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	return store
}

func newEngineTestEngine(store *Store, owner string, runner Runner) *Engine {
	engine := NewEngine(store, owner, runner)
	engine.checker.ModelResolver = func(Task) error { return nil }
	return engine
}

func assertTaskHasNoRuns(t *testing.T, store *Store, taskID string) {
	t.Helper()
	runs, err := store.ListRuns(taskID, 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("runs = %+v, want no archived runs", runs)
	}
}
