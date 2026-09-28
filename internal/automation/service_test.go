package automation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/event"
)

func TestServiceListTasksStableOrder(t *testing.T) {
	store := NewStore(t.TempDir())
	first := validStoreTask()
	first.ID = "b"
	first.Name = "Beta"
	second := validStoreTask()
	second.ID = "a"
	second.Name = "alpha"
	if err := store.SaveTasks([]Task{first, second}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}

	got, err := newService(store, nil).ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("ListTasks order = %+v, want alpha then Beta", got)
	}
}

func TestServiceListTasksIncludesBackendOwnedNextRun(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	task := validStoreTask()
	task.CreatedAt = now
	task.Schedule = Schedule{Kind: ScheduleKindInterval, Every: "30m"}
	if err := store.SaveTasks([]Task{task}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	svc := newService(store, nil)
	svc.now = func() time.Time { return now }
	views, err := svc.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	want := now.Add(30 * time.Minute)
	if len(views) != 1 || views[0].NextRunAt == nil || !views[0].NextRunAt.Equal(want) {
		t.Fatalf("views = %+v, want nextRunAt %s", views, want)
	}
}

func TestTaskViewJSONIncludesBackendOwnedNextRun(t *testing.T) {
	task := validStoreTask()
	task.UnknownFields = map[string]json.RawMessage{
		"futureOwner": []byte(`{"mode":"keep"}`),
		"nextRunAt":   []byte(`"stale"`),
	}
	nextRunAt := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)

	encoded, err := json.Marshal(TaskView{Task: task, NextRunAt: &nextRunAt})
	if err != nil {
		t.Fatalf("json.Marshal TaskView: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("json.Unmarshal TaskView: %v", err)
	}
	if string(fields["futureOwner"]) != `{"mode":"keep"}` {
		t.Errorf("futureOwner = %s, want preserved object", fields["futureOwner"])
	}
	if raw, ok := fields["nextRunAt"]; !ok {
		t.Errorf("nextRunAt is missing")
	} else {
		var gotNextRunAt string
		if err := json.Unmarshal(raw, &gotNextRunAt); err != nil {
			t.Errorf("json.Unmarshal nextRunAt: %v", err)
		} else if gotNextRunAt != nextRunAt.Format(time.RFC3339) {
			t.Errorf("nextRunAt = %q, want %q", gotNextRunAt, nextRunAt.Format(time.RFC3339))
		}
	}

	encoded, err = json.Marshal(TaskView{Task: task})
	if err != nil {
		t.Fatalf("json.Marshal nil TaskView: %v", err)
	}
	fields = nil
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("json.Unmarshal nil TaskView: %v", err)
	}
	if _, ok := fields["nextRunAt"]; ok {
		t.Errorf("nextRunAt = %s, want omitted when NextRunAt is nil", fields["nextRunAt"])
	}
}

func runSummaryIDs(runs []RunSummary) []string {
	ids := make([]string, 0, len(runs))
	for _, run := range runs {
		ids = append(ids, run.RunID)
	}
	return ids
}

func TestServiceSaveTasksPreservesRunMetadata(t *testing.T) {
	store := NewStore(t.TempDir())
	created := time.Date(2026, 6, 27, 8, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	task := validStoreTask()
	task.CreatedAt = created
	task.UpdatedAt = created
	task.LastRunAt = lastRun
	task.LastRunID = "run-1"
	if err := store.SaveTasks([]Task{task}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}

	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	svc := newService(store, nil)
	svc.now = func() time.Time { return now }
	changed := validStoreTask()
	changed.Name = "Changed"
	changed.Revision = 1
	if err := svc.SaveTasks(context.Background(), SaveTasksRequest{Tasks: []Task{changed}}); err != nil {
		t.Fatalf("service SaveTasks: %v", err)
	}

	got, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("tasks len = %d, want 1", len(got))
	}
	if !got[0].CreatedAt.Equal(created) || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = created %s updated %s, want %s/%s", got[0].CreatedAt, got[0].UpdatedAt, created, now)
	}
	if !got[0].LastRunAt.Equal(lastRun) || got[0].LastRunID != "run-1" {
		t.Fatalf("run metadata = %s/%q, want preserved", got[0].LastRunAt, got[0].LastRunID)
	}
}

func TestServiceTaskCRUDUsesRevisionContract(t *testing.T) {
	store := NewStore(t.TempDir())
	svc := newService(store, nil)
	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	task := validStoreTask()
	task.ID = "task-crud"
	task.Name = "Initial"
	created, err := svc.CreateTask(context.Background(), CreateTaskRequest{Task: task})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.Revision != 1 || created.SchemaVersion != CurrentTaskSchemaVersion {
		t.Fatalf("created task version = schema %d revision %d", created.SchemaVersion, created.Revision)
	}

	updatedTask := created.Task
	updatedTask.Name = "Updated"
	updated, err := svc.UpdateTask(context.Background(), UpdateTaskRequest{
		Task:             updatedTask,
		ExpectedRevision: created.Revision,
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.Name != "Updated" || updated.Revision != 2 {
		t.Fatalf("updated task = %+v", updated.Task)
	}

	if _, err := svc.UpdateTask(context.Background(), UpdateTaskRequest{
		Task:             updatedTask,
		ExpectedRevision: created.Revision,
	}); !errors.Is(err, ErrTaskRevisionConflict) {
		t.Fatalf("stale UpdateTask error = %v, want ErrTaskRevisionConflict", err)
	}

	disabled, err := svc.SetTaskEnabled(context.Background(), SetTaskEnabledRequest{
		ID:               updated.ID,
		Enabled:          false,
		ExpectedRevision: updated.Revision,
	})
	if err != nil {
		t.Fatalf("SetTaskEnabled: %v", err)
	}
	if disabled.Enabled || disabled.Revision != 3 {
		t.Fatalf("disabled task = %+v", disabled.Task)
	}

	if err := svc.DeleteTask(context.Background(), DeleteTaskRequest{
		ID:               disabled.ID,
		ExpectedRevision: disabled.Revision,
	}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	views, err := svc.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("tasks after delete = %+v, want empty", views)
	}
}

func TestServiceListTemplatesIncludesBuiltInAndUserTemplates(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.SaveTaskTemplate(TaskTemplate{
		ID:       "user-task-1",
		Title:    "User template",
		Prompt:   "run user template",
		Interval: "1h",
	}); err != nil {
		t.Fatalf("SaveTaskTemplate: %v", err)
	}

	templates, err := newService(store, nil).ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	var hasBuiltin, hasUser bool
	for _, template := range templates {
		if template.ID == "daily-ai-news" && template.Source == TemplateSourceBuiltin {
			hasBuiltin = true
		}
		if template.ID == "user-task-1" && template.Source == TemplateSourceUser {
			hasUser = true
		}
	}
	if !hasBuiltin || !hasUser {
		t.Fatalf("templates missing builtin/user entries: %+v", templates)
	}
}

func TestServiceSaveTaskTemplatePersistsManualTaskAsUserTemplate(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	task.ID = "task-custom"
	task.Name = "Manual task"
	task.Prompt.Prompt = "manual prompt"
	task.Schedule = Schedule{Kind: ScheduleKindCalendar, Calendar: CalendarSchedule{Frequency: "weekly", TimeOfDay: "10:30", Weekdays: []string{"mon", "fri"}}}

	saved, err := newService(store, nil).SaveTaskTemplate(context.Background(), SaveTaskTemplateRequest{Task: task})
	if err != nil {
		t.Fatalf("SaveTaskTemplate: %v", err)
	}
	if saved.ID != "user-task-custom" || saved.Title != "Manual task" || saved.Prompt != "manual prompt" {
		t.Fatalf("saved template = %+v, want fields derived from task", saved)
	}
	if saved.Source != TemplateSourceUser || saved.Interval != "168h|weekly:mon,fri@10:30" {
		t.Fatalf("saved template metadata = %+v, want user weekly template", saved)
	}

	templates, err := store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates: %v", err)
	}
	if len(templates) != 1 || templates[0].ID != saved.ID {
		t.Fatalf("stored templates = %+v, want saved template", templates)
	}
}

func TestServiceDeleteTaskTemplateRemovesUserTemplate(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.SaveTaskTemplate(TaskTemplate{
		ID:       "user-task-1",
		Title:    "User template",
		Prompt:   "run user template",
		Interval: "1h",
	}); err != nil {
		t.Fatalf("SaveTaskTemplate: %v", err)
	}

	svc := newService(store, nil)
	if err := svc.DeleteTaskTemplate(context.Background(), DeleteTaskTemplateRequest{ID: "user-task-1"}); err != nil {
		t.Fatalf("DeleteTaskTemplate: %v", err)
	}

	templates, err := svc.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	for _, template := range templates {
		if template.ID == "user-task-1" {
			t.Fatalf("deleted template still returned: %+v", templates)
		}
	}
	if len(templates) == 0 {
		t.Fatal("built-in templates should still be returned")
	}
}

func TestServiceDeleteTaskTemplateHidesBuiltInTemplate(t *testing.T) {
	store := NewStore(t.TempDir())
	svc := newService(store, nil)
	builtinID := DefaultTaskTemplates()[0].ID

	if err := svc.DeleteTaskTemplate(context.Background(), DeleteTaskTemplateRequest{ID: builtinID}); err != nil {
		t.Fatalf("DeleteTaskTemplate built-in: %v", err)
	}

	templates, err := svc.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	for _, template := range templates {
		if template.ID == builtinID {
			t.Fatalf("deleted built-in template still returned: %+v", templates)
		}
	}
	if len(templates) != len(DefaultTaskTemplates())-1 {
		t.Fatalf("templates len = %d, want one fewer built-in", len(templates))
	}

	if err := svc.RestoreTaskTemplate(context.Background(), RestoreTaskTemplateRequest{ID: builtinID}); err != nil {
		t.Fatalf("RestoreTaskTemplate built-in: %v", err)
	}
	templates, err = svc.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates after restore: %v", err)
	}
	found := false
	for _, template := range templates {
		if template.ID == builtinID {
			found = true
		}
	}
	if !found {
		t.Fatalf("restored built-in template missing: %+v", templates)
	}
}

func TestServiceTriggerNowUsesEngine(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{}
	engine := NewEngine(store, "test:service", runner)
	engine.now = func() time.Time { return now }
	engine.checker = Checker{ModelResolver: func(Task) error { return nil }}
	svc := newService(store, engine)
	svc.now = func() time.Time { return now }

	resp, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: "task-1", Reason: "manual test"})
	if err != nil {
		t.Fatalf("TriggerNow: %v", err)
	}
	if resp.RunID == "" || runner.count() != 1 {
		t.Fatalf("response=%+v runner calls=%d, want one engine run", resp, runner.count())
	}
	manifest, err := store.ReadRunManifest("task-1", resp.RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.Trigger != TriggerManual || manifest.TriggerReason != "manual test" || manifest.Status != RunStatusSucceeded {
		t.Fatalf("manifest = %+v, want manual succeeded", manifest)
	}
}

func TestTwoHostsTriggerSameTaskCreatesOneRun(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	root := t.TempDir()
	store := NewStore(root)
	if err := store.SaveTasks([]Task{{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	runner := &mockRunner{started: make(chan struct{}), block: make(chan struct{})}
	desktop := newServiceTestHost(root, "desktop:1", runner, now)
	serve := newServiceTestHost(root, "serve:2", runner, now)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, svc := range []*service{desktop, serve} {
		wg.Add(1)
		go func(svc *service) {
			defer wg.Done()
			<-start
			_, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: "task-1", Reason: "manual race"})
			errs <- err
		}(svc)
	}
	close(start)
	<-runner.started

	select {
	case err := <-errs:
		if !errors.Is(err, ErrLeaseUnavailable) {
			t.Fatalf("contending TriggerNow: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("contending TriggerNow did not return while lease was held")
	}
	close(runner.block)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("TriggerNow: %v", err)
		}
	}
	if runner.count() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.count())
	}
	runs, err := store.ListRuns("task-1", 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %+v, want one run", runs)
	}
}

func TestLeaseExpiryLetsAnotherHostTakeOver(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	root := t.TempDir()
	store := NewStore(root)
	if err := store.SaveTasks([]Task{{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	if _, ok, err := store.AcquireLease("task-1", "crashed-run", "desktop:1", time.Hour); err != nil || !ok {
		t.Fatalf("AcquireLease crashed owner ok=%v err=%v", ok, err)
	}
	runner := &mockRunner{}
	serve := newServiceTestHost(root, "serve:2", runner, now)
	if response, err := serve.TriggerNow(context.Background(), TriggerNowRequest{TaskID: "task-1"}); !errors.Is(err, ErrLeaseUnavailable) || response.RunID != "" || response.Status != "" {
		t.Fatalf("TriggerNow with active lease = %+v, %v; want no run and lease error", response, err)
	}
	if runner.count() != 0 {
		t.Fatalf("runner calls with active lease = %d, want 0", runner.count())
	}

	path, err := store.leasePath("task-1")
	if err != nil {
		t.Fatalf("leasePath: %v", err)
	}
	expiredAt := time.Now().Add(-time.Minute).UTC()
	if err := writeLease(path, Lease{
		TaskID:      "task-1",
		RunID:       "crashed-run",
		Owner:       "desktop:1",
		AcquiredAt:  expiredAt.Add(-time.Minute),
		HeartbeatAt: expiredAt.Add(-time.Minute),
		ExpiresAt:   expiredAt,
	}); err != nil {
		t.Fatalf("writeLease: %v", err)
	}

	resp, err := serve.TriggerNow(context.Background(), TriggerNowRequest{TaskID: "task-1"})
	if err != nil {
		t.Fatalf("TriggerNow after lease expiry: %v", err)
	}
	if resp.RunID == "" || runner.count() != 1 {
		t.Fatalf("response=%+v runner calls=%d, want takeover run", resp, runner.count())
	}
	manifest, err := store.ReadRunManifest("task-1", resp.RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.HostOwner != "serve:2" || manifest.Status != RunStatusSucceeded {
		t.Fatalf("manifest = %+v, want serve takeover succeeded", manifest)
	}
}

func TestServiceCancelCurrentProcessRun(t *testing.T) {
	now := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{started: make(chan struct{}), block: make(chan struct{})}
	engine := NewEngine(store, "test:service", runner)
	engine.now = func() time.Time { return now }
	engine.checker = Checker{ModelResolver: func(Task) error { return nil }}
	svc := newService(store, engine)
	svc.now = func() time.Time { return now }
	other := newServiceTestHost(store.dir, "serve:2", &mockRunner{}, now)

	done := make(chan error, 1)
	go func() {
		_, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: "task-1"})
		done <- err
	}()
	<-runner.started
	runID := waitForRunID(t, store, "task-1")
	otherResp, err := other.CancelRun(context.Background(), CancelRunRequest{TaskID: "task-1", RunID: runID})
	if err != nil {
		t.Fatalf("other CancelRun: %v", err)
	}
	if otherResp.Canceled || otherResp.Status != "not_cancelable" {
		t.Fatalf("other CancelRun response = %+v, want not cancelable", otherResp)
	}

	cancelResp, err := svc.CancelRun(context.Background(), CancelRunRequest{TaskID: "task-1", RunID: runID})
	if err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	if !cancelResp.Canceled {
		t.Fatalf("CancelRun response = %+v, want canceled", cancelResp)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("TriggerNow returned error after cancel: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("TriggerNow did not return after cancel")
	}
	manifest, err := store.ReadRunManifest("task-1", runID)
	if err != nil {
		t.Fatalf("ReadRunManifest: %v", err)
	}
	if manifest.Status != RunStatusCanceled {
		t.Fatalf("manifest status = %s, want canceled", manifest.Status)
	}
}

func TestServiceTrashCancelsActiveRunBeforeMovingArchive(t *testing.T) {
	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Scope:     "global",
		Schedule:  Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(time.Hour)},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	})
	runner := &mockRunner{started: make(chan struct{}), block: make(chan struct{})}
	engine := NewEngine(store, "test:service", runner)
	engine.now = func() time.Time { return now }
	engine.checker = Checker{ModelResolver: func(Task) error { return nil }}
	svc := newService(store, engine)
	svc.now = func() time.Time { return now }

	done := make(chan error, 1)
	go func() {
		_, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: "task-1"})
		done <- err
	}()
	<-runner.started
	runID := waitForRunID(t, store, "task-1")
	trashed, err := svc.TrashRun(context.Background(), TrashRunRequest{TaskID: "task-1", RunID: runID, Scope: "global"})
	if err != nil {
		t.Fatalf("TrashRun active run: %v", err)
	}
	if trashed.Status != RunStatusCanceled {
		t.Fatalf("trashed status = %s, want canceled", trashed.Status)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("TriggerNow returned error after trash: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("TriggerNow did not return after trash cancellation")
	}
	paths, _ := store.RunPaths("task-1", runID)
	if _, err := os.Stat(paths.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active archive still present after trash: %v", err)
	}
}

func TestServiceTrashKeepsActiveRunWhenCancellationTimesOut(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	manifest := RunManifest{
		RunID: "run-timeout", TaskID: "task-1", TaskName: "Task 1", Scope: "global",
		Status: RunStatusRunning, Trigger: TriggerSchedule, StartedAt: time.Now().UTC(),
	}
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}
	svc := newService(store, nil)
	svc.runCancellationWaitTimeout = 40 * time.Millisecond
	svc.running.add(manifest.TaskID, manifest.RunID, func() {})
	t.Cleanup(func() { svc.running.remove(manifest.TaskID, manifest.RunID) })

	_, err := svc.TrashRun(context.Background(), TrashRunRequest{TaskID: manifest.TaskID, RunID: manifest.RunID, Scope: "global"})
	if !errors.Is(err, ErrRunCancellationTimeout) {
		t.Fatalf("TrashRun error = %v, want ErrRunCancellationTimeout", err)
	}
	kept, readErr := store.ReadRunManifest(manifest.TaskID, manifest.RunID)
	if readErr != nil || kept.Status != RunStatusRunning {
		t.Fatalf("active manifest after timeout = %+v, %v", kept, readErr)
	}
}

func TestServiceTrashKeepsUnregisteredActiveRun(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	manifest := RunManifest{
		RunID: "run-other-process", TaskID: "task-1", TaskName: "Task 1", Scope: "global",
		Status: RunStatusQueued, Trigger: TriggerManual, StartedAt: time.Now().UTC(),
	}
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}
	svc := newService(store, nil)
	_, err := svc.TrashRun(context.Background(), TrashRunRequest{TaskID: manifest.TaskID, RunID: manifest.RunID, Scope: "global"})
	if !errors.Is(err, ErrRunOwnedByAnotherProcess) {
		t.Fatalf("TrashRun error = %v, want ErrRunOwnedByAnotherProcess", err)
	}
	if _, readErr := store.ReadRunManifest(manifest.TaskID, manifest.RunID); readErr != nil {
		t.Fatalf("active manifest should be retained: %v", readErr)
	}
}

func TestServiceShutdownStopsEngineTicks(t *testing.T) {
	store := newEngineTestStore(t, Task{
		ID:        "task-1",
		Name:      "Task 1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleKindInterval, Every: "1ms"},
		CreatedAt: time.Now().Add(-time.Hour).UTC(),
		UpdatedAt: time.Now().Add(-time.Hour).UTC(),
	})
	runner := &mockRunner{}
	engine := NewEngine(store, "serve:1", runner)
	engine.tickInterval = 5 * time.Millisecond
	engine.checker = Checker{ModelResolver: func(Task) error { return nil }}
	svc := newService(store, engine)

	engine.Start(context.Background())
	t.Cleanup(engine.Stop)
	waitForRunnerCount(t, runner, 1)
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	stoppedAt := runner.count()
	time.Sleep(3 * engine.tickInterval)
	if got := runner.count(); got != stoppedAt {
		t.Fatalf("runner calls after Shutdown = %d, want %d", got, stoppedAt)
	}
}

func TestServiceReadRunReturnsSummaryAndEventMetadata(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	manifest, paths, err := store.CreateRun(task, string(TriggerManual), "test:service")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.WriteResult(task.ID, manifest.RunID, strings.Repeat("x", defaultResultSummaryRunes+10)); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}
	sink, err := NewArchiveSink(paths.Events, ArchiveEventMeta{TaskID: task.ID, RunID: manifest.RunID}, nil)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}
	_ = sink.EmitRunStatus(RunStatusRunning)
	sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: strings.Repeat("n", defaultEventTextRunes+10)})
	if err := sink.Close(); err != nil {
		t.Fatalf("ArchiveSink Close: %v", err)
	}

	detail, err := newService(store, nil).ReadRun(context.Background(), task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	if detail.Manifest.RunID != manifest.RunID || !detail.ResultTruncated || len([]rune(detail.ResultSummary)) != defaultResultSummaryRunes {
		t.Fatalf("detail result = %+v, want truncated summary", detail)
	}
	if len(detail.Events) != 2 || detail.Events[0].Status != RunStatusRunning {
		t.Fatalf("events = %+v, want status metadata", detail.Events)
	}
	if len([]rune(detail.Events[1].Text)) != defaultEventTextRunes {
		t.Fatalf("event text len = %d, want capped metadata text", len([]rune(detail.Events[1].Text)))
	}
}

func TestReadRunReturnsAvailableResultFormats(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	manifest, _, err := store.CreateRun(task, string(TriggerManual), "test:service")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.WriteResult(task.ID, manifest.RunID, "# Result\n\nok"); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	detail, err := newService(store, nil).ReadRun(context.Background(), task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	got := make([]string, 0, len(detail.AvailableResultFormats))
	for _, format := range detail.AvailableResultFormats {
		got = append(got, format.Format)
		if format.Label == "" || format.Extension == "" || format.MimeType == "" {
			t.Fatalf("format metadata is incomplete: %+v", format)
		}
	}
	want := []string{"md", "docx", "txt", "json", "xlsx", "pptx"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("available formats = %+v, want %+v", got, want)
	}
}

func TestReadRunReturnsNoAvailableResultFormatsWithoutResult(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	manifest, _, err := store.CreateRun(task, string(TriggerManual), "test:service")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	detail, err := newService(store, nil).ReadRun(context.Background(), task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("ReadRun: %v", err)
	}
	if len(detail.AvailableResultFormats) != 0 {
		t.Fatalf("available formats = %+v, want none", detail.AvailableResultFormats)
	}
}

func TestReadRunCompactsStreamingEventDeltasBeforeLimit(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	manifest, paths, err := store.CreateRun(task, string(TriggerManual), "test:service")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	sink, err := NewArchiveSink(paths.Events, ArchiveEventMeta{TaskID: task.ID, RunID: manifest.RunID}, nil)
	if err != nil {
		t.Fatalf("NewArchiveSink: %v", err)
	}
	_ = sink.EmitRunStatus(RunStatusQueued)
	_ = sink.EmitRunStatus(RunStatusRunning)
	for _, token := range []string{"The", "memory", "compiler", "is", "giving", "me", "what", "'s", "happening", ".", "Let", "me", "understand", "202", "6年"} {
		sink.Emit(event.Event{Kind: event.Reasoning, Level: event.LevelInfo, Text: token})
	}
	_ = sink.EmitRunStatus(RunStatusSucceeded)
	if err := sink.Close(); err != nil {
		t.Fatalf("ArchiveSink Close: %v", err)
	}

	events, err := readRunEventSummaries(paths.Events, 5)
	if err != nil {
		t.Fatalf("readRunEventSummaries: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("events len = %d, want 4: %+v", len(events), events)
	}
	if events[2].Kind != event.Reasoning || events[2].Text != "The memory compiler is giving me what's happening. Let me understand 2026年" {
		t.Fatalf("compacted reasoning event = %+v", events[2])
	}
	if events[3].Status != RunStatusSucceeded {
		t.Fatalf("last event status = %q, want %q", events[3].Status, RunStatusSucceeded)
	}
}

func TestReadRunPreservesArchivedNumericEventKinds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	raw := strings.Join([]string{
		`{"kind":19,"text":"guardian"}`,
		`{"kind":20,"text":"audit"}`,
		`{"kind":23,"text":"started one"}`,
		`{"kind":23,"text":"started two"}`,
		`{"kind":24,"text":"task"}`,
		`{"kind":24,"text":"output"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := readRunEventSummaries(path, 10)
	if err != nil {
		t.Fatalf("readRunEventSummaries: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("events len = %d, want 5: %+v", len(events), events)
	}
	if events[0].Kind != event.GuardianAssessment || events[1].Kind != event.AuditEvent {
		t.Fatalf("guardian/audit kinds = %d/%d, want %d/%d", events[0].Kind, events[1].Kind, event.GuardianAssessment, event.AuditEvent)
	}
	if events[2].Kind != event.TaskNodeStarted || events[3].Kind != event.TaskNodeStarted {
		t.Fatalf("kind 23 events were compacted or reclassified: %+v", events[2:4])
	}
	if events[4].Kind != event.TaskNodeOutput || events[4].Text != "task output" {
		t.Fatalf("kind 24 output = %+v, want compacted task output", events[4])
	}
}

func newServiceTestHost(root, owner string, runner Runner, now time.Time) *service {
	store := NewStore(root)
	engine := NewEngine(store, owner, runner)
	engine.now = func() time.Time { return now }
	engine.checker = Checker{ModelResolver: func(Task) error { return nil }}
	svc := newService(store, engine)
	svc.now = func() time.Time { return now }
	return svc
}

func waitForRunnerCount(t *testing.T, runner *mockRunner, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if runner.count() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runner calls = %d, want at least %d", runner.count(), want)
}

func waitForRunID(t *testing.T, store *Store, taskID string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		runs, err := store.ListRuns(taskID, 1)
		if err != nil {
			t.Fatalf("ListRuns: %v", err)
		}
		if len(runs) > 0 {
			return runs[0].RunID
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for run")
	return ""
}
