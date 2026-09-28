package automation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockConditionModel struct {
	raw    string
	err    error
	calls  int
	prompt string
}

func (m *mockConditionModel) Decide(ctx context.Context, task Task, prompt string) (string, error) {
	m.calls++
	m.prompt = prompt
	return m.raw, m.err
}

func TestConditionWorkspaceMissing(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.WorkspaceRoot = t.TempDir() + "-missing"
	store, runner := runConditionTick(t, now, task, Checker{
		ModelResolver: func(Task) error { return nil },
	})

	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	manifest := onlyRunManifest(t, store, task.ID)
	if manifest.Status != RunStatusBlocked || manifest.Condition.ShouldRun || manifest.ErrorCode != RunErrorWorkspaceMissing || manifest.Condition.Error == "" {
		t.Fatalf("manifest condition = %+v status=%s, want blocked workspace error", manifest.Condition, manifest.Status)
	}
}

func TestConditionModelUnavailable(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.WorkspaceRoot = t.TempDir()
	store, runner := runConditionTick(t, now, task, Checker{
		ModelResolver: func(Task) error { return errors.New("unknown model") },
	})

	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	manifest := onlyRunManifest(t, store, task.ID)
	if manifest.Status != RunStatusBlocked || manifest.ErrorCode != RunErrorModelUnavailable || !strings.Contains(manifest.Condition.Error, "unknown model") {
		t.Fatalf("manifest condition = %+v status=%s, want blocked model error", manifest.Condition, manifest.Status)
	}
}

func TestConditionPromptFalse(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.WorkspaceRoot = t.TempDir()
	task.Conditions.ConditionPrompt = "Run only when needed."
	model := &mockConditionModel{raw: `{"should_run": false, "reason": "nothing changed"}`}
	store, runner := runConditionTick(t, now, task, Checker{
		ConditionModel: model,
		ModelResolver:  func(Task) error { return nil },
	})

	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	if model.calls != 1 || model.prompt != task.Conditions.ConditionPrompt {
		t.Fatalf("condition model calls=%d prompt=%q", model.calls, model.prompt)
	}
	manifest := onlyRunManifest(t, store, task.ID)
	if manifest.Status != RunStatusSkipped || manifest.ErrorCode != RunErrorConditionRejected || manifest.Condition.ShouldRun || manifest.Condition.Reason != "nothing changed" {
		t.Fatalf("manifest condition = %+v status=%s, want skipped false prompt", manifest.Condition, manifest.Status)
	}
}

func TestConditionPromptInvalidJSON(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.WorkspaceRoot = t.TempDir()
	task.Conditions.ConditionPrompt = "Return JSON."
	model := &mockConditionModel{raw: `not json`}
	store, runner := runConditionTick(t, now, task, Checker{
		ConditionModel: model,
		ModelResolver:  func(Task) error { return nil },
	})

	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	manifest := onlyRunManifest(t, store, task.ID)
	if manifest.Status != RunStatusBlocked || manifest.ErrorCode != RunErrorConditionInvalidJSON || manifest.Condition.Error == "" {
		t.Fatalf("manifest condition = %+v status=%s, want blocked invalid JSON", manifest.Condition, manifest.Status)
	}
}

func TestConditionAllowsRun(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.WorkspaceRoot = t.TempDir()
	task.Conditions.ConditionPrompt = "Return JSON."
	model := &mockConditionModel{raw: `{"should_run": true, "reason": "ready"}`}
	store, runner := runConditionTick(t, now, task, Checker{
		ConditionModel: model,
		ModelResolver:  func(Task) error { return nil },
	})

	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.count())
	}
	manifest := onlyRunManifest(t, store, task.ID)
	if manifest.Status != RunStatusSucceeded || !manifest.Condition.ShouldRun || manifest.Condition.Reason != "ready" {
		t.Fatalf("manifest condition = %+v status=%s, want succeeded allowed prompt", manifest.Condition, manifest.Status)
	}
}

func TestConditionFailureCooldown(t *testing.T) {
	now := conditionTestNow()
	task := conditionTestTask(now)
	task.Conditions.FailureCooldown = "30m"
	store := newEngineTestStore(t, task)
	failed, _, err := store.CreateRun(task, string(TriggerSchedule), "test:old")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	failed.Status = RunStatusFailed
	failed.StartedAt = now.Add(-10 * time.Minute)
	failed.FinishedAt = now.Add(-9 * time.Minute)
	if err := store.SaveRunManifest(failed); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}

	runner := &mockRunner{}
	engine := NewEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }
	engine.checker = Checker{ModelResolver: func(Task) error { return nil }}
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
	runs, err := store.ListRuns(task.ID, 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 || runs[0].Status != RunStatusSkipped {
		t.Fatalf("runs = %+v, want newest skipped by cooldown", runs)
	}
}

func runConditionTick(t *testing.T, now time.Time, task Task, checker Checker) (*Store, *mockRunner) {
	t.Helper()
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := NewEngine(store, "test:1", runner)
	engine.now = func() time.Time { return now }
	engine.checker = checker
	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	return store, runner
}

func conditionTestTask(now time.Time) Task {
	return Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
}

func conditionTestNow() time.Time {
	return time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
}

func onlyRunManifest(t *testing.T, store *Store, taskID string) RunManifest {
	t.Helper()
	runs, err := store.ListRuns(taskID, 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs len = %d, want 1: %+v", len(runs), runs)
	}
	manifest, err := store.ReadRunManifest(taskID, runs[0].RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	return manifest
}
