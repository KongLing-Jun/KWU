package automation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestTaskLimitForPlan(t *testing.T) {
	tests := map[string]int{
		TaskQuotaPlanAnonymous: 5,
		TaskQuotaPlanFree:      10,
		TaskQuotaPlanLite:      25,
		TaskQuotaPlanPlus:      50,
		TaskQuotaPlanPro:       0,
	}
	for plan, want := range tests {
		if got := TaskLimitForPlan(plan); got != want {
			t.Fatalf("TaskLimitForPlan(%q) = %d, want %d", plan, got, want)
		}
	}
}

func TestProTaskQuotaAllowsGrowthPastLegacyLimit(t *testing.T) {
	store := NewStore(t.TempDir())
	resolver := TaskQuotaResolverFunc(func(context.Context) (TaskQuotaEntitlement, error) {
		return entitlementForPlan(TaskQuotaPlanPro), nil
	})
	svc := NewServiceWithTaskQuota(store, nil, resolver)
	tasks := make([]Task, 120)
	for index := range tasks {
		task := validStoreTask()
		task.ID = fmt.Sprintf("pro-task-%03d", index)
		task.Name = fmt.Sprintf("Pro task %03d", index)
		tasks[index] = task
	}
	if err := svc.SaveTasks(context.Background(), SaveTasksRequest{Tasks: tasks}); err != nil {
		t.Fatalf("save 120 Pro tasks: %v", err)
	}
	view, err := svc.(TaskQuotaService).TaskQuota(context.Background())
	if err != nil {
		t.Fatalf("TaskQuota: %v", err)
	}
	if !view.Unlimited || !view.CanCreate || view.Limit != 0 || view.Used != len(tasks) || len(view.PausedTaskIDs) != 0 {
		t.Fatalf("Pro quota = %+v, want unlimited with all tasks active", view)
	}
}

func TestFileTaskQuotaResolverUsesVerifiedPlanUntilPeriodEnd(t *testing.T) {
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	resolver := NewFileTaskQuotaResolver(t.TempDir(), TaskQuotaStateLoading)
	resolver.now = func() time.Time { return now }

	missing, err := resolver.ResolveTaskQuota(context.Background())
	if err != nil || missing.State != TaskQuotaStateLoading {
		t.Fatalf("missing quota = %+v, %v, want loading", missing, err)
	}

	periodEnd := now.Add(time.Hour)
	if err := resolver.Save(TaskQuotaSnapshot{Plan: TaskQuotaPlanPlus, PeriodEnd: &periodEnd}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	active, err := resolver.ResolveTaskQuota(context.Background())
	if err != nil || active.Plan != TaskQuotaPlanPlus || active.Limit != 50 {
		t.Fatalf("active quota = %+v, %v, want Plus/50", active, err)
	}

	resolver.now = func() time.Time { return periodEnd.Add(time.Second) }
	expired, err := resolver.ResolveTaskQuota(context.Background())
	if err != nil || expired.Plan != TaskQuotaPlanFree || expired.Limit != 10 {
		t.Fatalf("expired quota = %+v, %v, want Free/10", expired, err)
	}
	if err := resolver.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	cleared, err := resolver.ResolveTaskQuota(context.Background())
	if err != nil || cleared.State != TaskQuotaStateLoading {
		t.Fatalf("cleared quota = %+v, %v, want loading", cleared, err)
	}
}

func TestTaskQuotaPolicyPausesNewestTasksDeterministically(t *testing.T) {
	at := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	tasks := []Task{
		{ID: "b", CreatedAt: at},
		{ID: "c", CreatedAt: at.Add(time.Minute)},
		{ID: "a", CreatedAt: at},
	}
	policy := NewTaskQuotaPolicy(TaskQuotaResolverFunc(func(context.Context) (TaskQuotaEntitlement, error) {
		return TaskQuotaEntitlement{State: TaskQuotaStateReady, Plan: TaskQuotaPlanFree, Limit: 2}, nil
	}))
	view, err := policy.View(context.Background(), tasks)
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if len(view.PausedTaskIDs) != 1 || view.PausedTaskIDs[0] != "c" {
		t.Fatalf("paused = %v, want [c]", view.PausedTaskIDs)
	}
}

func TestServiceTaskQuotaBlocksOnlyNetTaskGrowth(t *testing.T) {
	store := NewStore(t.TempDir())
	first := validStoreTask()
	first.ID = "first"
	if err := store.SaveTasks([]Task{first}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	resolver := TaskQuotaResolverFunc(func(context.Context) (TaskQuotaEntitlement, error) {
		return TaskQuotaEntitlement{State: TaskQuotaStateReady, Plan: TaskQuotaPlanFree, Limit: 1}, nil
	})
	svc := NewServiceWithTaskQuota(store, nil, resolver)

	edited := first
	edited.Name = "edited"
	edited.Revision = 1
	if err := svc.SaveTasks(context.Background(), SaveTasksRequest{Tasks: []Task{edited}}); err != nil {
		t.Fatalf("same-count edit: %v", err)
	}
	second := validStoreTask()
	second.ID = "second"
	current, _ := store.Task(first.ID)
	edited.Revision = current.Revision
	if err := svc.SaveTasks(context.Background(), SaveTasksRequest{Tasks: []Task{edited, second}}); !errors.Is(err, ErrTaskLimitExceeded) {
		t.Fatalf("growth error = %v, want ErrTaskLimitExceeded", err)
	}
	if err := svc.DeleteTask(context.Background(), DeleteTaskRequest{ID: current.ID, ExpectedRevision: current.Revision}); err != nil {
		t.Fatalf("revisioned delete while at limit: %v", err)
	}
}

func TestEngineQuotaSkipsScheduledExcessAndBlocksManualRun(t *testing.T) {
	now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	oldest := Task{
		ID:        "oldest",
		Name:      "Oldest",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-2 * time.Hour),
	}
	newest := oldest
	newest.ID = "newest"
	newest.Name = "Newest"
	newest.CreatedAt = now.Add(-time.Hour)
	newest.UpdatedAt = newest.CreatedAt
	store := newEngineTestStore(t, oldest, newest)
	runner := &mockRunner{}
	engine := NewEngine(store, "test:quota", runner)
	engine.now = func() time.Time { return now }
	engine.checker.ModelResolver = func(Task) error { return nil }
	resolver := TaskQuotaResolverFunc(func(context.Context) (TaskQuotaEntitlement, error) {
		return TaskQuotaEntitlement{State: TaskQuotaStateReady, Plan: TaskQuotaPlanFree, Limit: 1}, nil
	})
	NewServiceWithTaskQuota(store, engine, resolver)

	if err := engine.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want only oldest scheduled task", runner.count())
	}
	if _, err := engine.Trigger(context.Background(), TriggerRequest{TaskID: newest.ID}); !errors.Is(err, ErrTaskLimitExceeded) {
		t.Fatalf("newest trigger error = %v, want ErrTaskLimitExceeded", err)
	}
}

func TestLoadingTaskQuotaAllowsEditsButPausesRuns(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	if err := store.SaveTasks([]Task{task}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	runner := &mockRunner{}
	engine := NewEngine(store, "test:loading-quota", runner)
	resolver := TaskQuotaResolverFunc(func(context.Context) (TaskQuotaEntitlement, error) {
		return TaskQuotaEntitlement{State: TaskQuotaStateLoading}, nil
	})
	svc := NewServiceWithTaskQuota(store, engine, resolver)

	task.Name = "edited"
	task.Revision = 1
	if err := svc.SaveTasks(context.Background(), SaveTasksRequest{Tasks: []Task{task}}); err != nil {
		t.Fatalf("edit while loading: %v", err)
	}
	second := task
	second.ID = "second"
	second.Revision = 0
	current, _ := store.Task(task.ID)
	task.Revision = current.Revision
	if err := svc.SaveTasks(context.Background(), SaveTasksRequest{Tasks: []Task{task, second}}); !errors.Is(err, ErrTaskQuotaUnavailable) {
		t.Fatalf("growth error = %v, want ErrTaskQuotaUnavailable", err)
	}
	if _, err := engine.Trigger(context.Background(), TriggerRequest{TaskID: task.ID}); !errors.Is(err, ErrTaskQuotaUnavailable) {
		t.Fatalf("trigger error = %v, want ErrTaskQuotaUnavailable", err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.count())
	}
}
