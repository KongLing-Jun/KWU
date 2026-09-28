package automation

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestStoreDefaultDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("KWU_HOME", home)

	if got, want := DefaultDir(), filepath.Join(home, "automation"); got != want {
		t.Fatalf("DefaultDir() = %q, want %q", got, want)
	}
}

func TestStoreLoadTasksMissing(t *testing.T) {
	tasks, err := NewStore(t.TempDir()).LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks returned error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("LoadTasks returned %d tasks, want 0", len(tasks))
	}
}

func TestStoreLoadTasksEmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), []byte("  \n"), 0o600); err != nil {
		t.Fatalf("write empty tasks file: %v", err)
	}

	tasks, err := NewStore(dir).LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks returned error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("LoadTasks returned %d tasks, want 0", len(tasks))
	}
}

func TestStoreLoadTasksPreservesCloudProjectOwnership(t *testing.T) {
	dir := t.TempDir()
	raw := `{"tasks":[{"schemaVersion":2,"revision":1,"id":"project-task","name":"Project task","enabled":true,"scope":"project","projectId":"project-1","prompt":{"prompt":"Run in project"},"schedule":{"kind":"interval","every":"1h"}},{"schemaVersion":2,"revision":1,"id":"personal-task","name":"Personal task","enabled":true,"scope":"global","prompt":{"prompt":"Run globally"},"schedule":{"kind":"interval","every":"1h"}}]}`
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	tasks, err := NewStore(dir).LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 2 || tasks[0].ID != "project-task" || tasks[0].ProjectID != "project-1" || tasks[0].Scope != "project" {
		t.Fatalf("tasks = %+v, want preserved project ownership", tasks)
	}
	persisted, err := os.ReadFile(filepath.Join(dir, tasksFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(persisted, []byte(`"projectId":"project-1"`)) {
		t.Fatalf("cloud project ownership was not persisted: %s", persisted)
	}
}

func TestStoreLoadTasksRemovesRetiredCloudAutomationLinks(t *testing.T) {
	dir := t.TempDir()
	raw := `{"tasks":[{"schemaVersion":2,"revision":1,"id":"retired-link","name":"Retired link","enabled":true,"scope":"project","projectId":"project-1","cloudProject":{"projectId":"project-1","automationId":"automation-1"},"prompt":{"prompt":"old"},"schedule":{"kind":"interval","every":"1h"}},{"schemaVersion":2,"revision":1,"id":"personal-task","name":"Personal task","enabled":true,"scope":"global","prompt":{"prompt":"run"},"schedule":{"kind":"interval","every":"1h"}}]}`
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	tasks, err := NewStore(dir).LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "personal-task" {
		t.Fatalf("tasks = %+v, want retired Cloud automation link removed", tasks)
	}
}

func TestStoreSaveLoadTasks(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	task.Prompt.BillingSource = "newapi_byok"

	if err := store.SaveTasks([]Task{task}); err != nil {
		t.Fatalf("SaveTasks returned error: %v", err)
	}
	if _, err := os.Stat(store.tasksPath()); err != nil {
		t.Fatalf("tasks file was not created: %v", err)
	}

	got, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks returned error: %v", err)
	}
	want, err := NormalizeTask(task)
	if err != nil {
		t.Fatalf("NormalizeTask returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []Task{want}) {
		t.Fatalf("LoadTasks = %+v, want %+v", got, []Task{want})
	}
}

func TestStoreTaskRevisionCRUD(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	created, err := store.CreateTask(validStoreTask())
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}
	if created.Revision != 1 || created.SchemaVersion != CurrentTaskSchemaVersion {
		t.Fatalf("created schema/revision = %d/%d", created.SchemaVersion, created.Revision)
	}

	created.Name = "Updated task"
	updated, err := store.UpdateTask(created, 1)
	if err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}
	if updated.Revision != 2 || updated.Name != "Updated task" {
		t.Fatalf("updated task = %+v", updated)
	}
	if _, err := store.UpdateTask(updated, 1); !errors.Is(err, ErrTaskRevisionConflict) {
		t.Fatalf("stale UpdateTask error = %v, want %v", err, ErrTaskRevisionConflict)
	}

	disabled, err := store.SetTaskEnabled(updated.ID, false, 2)
	if err != nil {
		t.Fatalf("SetTaskEnabled returned error: %v", err)
	}
	if disabled.Enabled || disabled.Revision != 3 {
		t.Fatalf("disabled task = %+v", disabled)
	}
	if err := store.DeleteTask(disabled.ID, 2); !errors.Is(err, ErrTaskRevisionConflict) {
		t.Fatalf("stale DeleteTask error = %v, want %v", err, ErrTaskRevisionConflict)
	}
	if err := store.DeleteTask(disabled.ID, 3); err != nil {
		t.Fatalf("DeleteTask returned error: %v", err)
	}
	if _, err := store.Task(disabled.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("Task after delete error = %v, want %v", err, ErrTaskNotFound)
	}
}

func TestStoreRecordTaskRunDoesNotOverwriteConcurrentRevisionedEdit(t *testing.T) {
	store := NewStore(t.TempDir())
	created, err := store.CreateTask(validStoreTask())
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	updatedInput := created
	updatedInput.Name = "Edited while run finishes"
	runAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, err := store.UpdateTask(updatedInput, created.Revision)
		errs <- err
	}()
	go func() {
		<-start
		errs <- store.RecordTaskRun(created.ID, "run-concurrent", runAt)
	}()
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent operation: %v", err)
		}
	}

	got, err := store.Task(created.ID)
	if err != nil {
		t.Fatalf("Task: %v", err)
	}
	if got.Name != updatedInput.Name || got.Revision != created.Revision+1 {
		t.Fatalf("revisioned edit was overwritten: %+v", got)
	}
	if got.LastRunID != "run-concurrent" || !got.LastRunAt.Equal(runAt) {
		t.Fatalf("run metadata was lost: %+v", got)
	}
}

func TestStoreCreateTaskRejectsDuplicateID(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	if _, err := store.CreateTask(validStoreTask()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateTask(validStoreTask()); !errors.Is(err, ErrDuplicateTaskID) {
		t.Fatalf("duplicate CreateTask error = %v, want %v", err, ErrDuplicateTaskID)
	}
}

func TestStoreLoadsLegacyTaskWithoutBillingSource(t *testing.T) {
	dir := t.TempDir()
	payload := `{"tasks":[{"id":"legacy","name":"Legacy task","prompt":{"model":"provider/model","prompt":"run"},"schedule":{"kind":"once"}}]}`
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), []byte(payload), 0o600); err != nil {
		t.Fatalf("write legacy tasks: %v", err)
	}

	tasks, err := NewStore(dir).LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Prompt.BillingSource != "" {
		t.Fatalf("legacy tasks = %+v, want empty billing source", tasks)
	}
}

func TestStoreMigratesLegacyTasksWithBackupAndMarker(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	legacy := []byte(`{"tasks":[{"id":"legacy-v1","name":"Legacy","enabled":true,"schedule":{"kind":"interval","every":"30m"}}]}`)
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), legacy, 0o600); err != nil {
		t.Fatalf("WriteFile legacy tasks: %v", err)
	}

	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].SchemaVersion != CurrentTaskSchemaVersion || tasks[0].Revision != 1 {
		t.Fatalf("migrated tasks = %+v", tasks)
	}
	backup, err := os.ReadFile(filepath.Join(dir, tasksV2BackupFileName))
	if err != nil {
		t.Fatalf("ReadFile backup: %v", err)
	}
	if string(backup) != string(legacy) {
		t.Fatalf("backup = %s, want original %s", backup, legacy)
	}
	if _, err := os.Stat(filepath.Join(dir, tasksV2MarkerFileName)); err != nil {
		t.Fatalf("migration marker: %v", err)
	}
	persisted, err := os.ReadFile(filepath.Join(dir, tasksFileName))
	if err != nil || !strings.Contains(string(persisted), `"schemaVersion": 2`) {
		t.Fatalf("persisted v2 = %s, %v", persisted, err)
	}
}

func TestStoreKeepsRoutePickerArtifactForCatalogAwareResolver(t *testing.T) {
	dir := t.TempDir()
	payload := `{"tasks":[` +
		`{"id":"official","name":"Official","prompt":{"model":":kwu/claude-fable-5","billingSource":"subscription_pool","prompt":"run"},"schedule":{"kind":"once"}},` +
		`{"id":"byok","name":"BYOK","prompt":{"model":":custom/chat-model","billingSource":"newapi_byok","prompt":"run"},"schedule":{"kind":"once"}},` +
		`{"id":"legacy","name":"Legacy","prompt":{"model":":custom/chat-model","prompt":"run"},"schedule":{"kind":"once"}}` +
		`]}`
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), []byte(payload), 0o600); err != nil {
		t.Fatalf("write route tasks: %v", err)
	}

	tasks, err := NewStore(dir).LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks returned error: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("LoadTasks returned %d tasks, want 3", len(tasks))
	}
	if tasks[0].Prompt.Model != ":kwu/claude-fable-5" ||
		tasks[1].Prompt.Model != ":custom/chat-model" ||
		tasks[2].Prompt.Model != ":custom/chat-model" {
		t.Fatalf("loaded models = %q, %q, %q", tasks[0].Prompt.Model, tasks[1].Prompt.Model, tasks[2].Prompt.Model)
	}
}

func TestStoreSaveLoadTaskTemplate(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	template := TaskTemplate{
		ID:     " user-task-1 ",
		Title:  " Custom reminder ",
		Prompt: " remind me ",
	}

	saved, err := store.SaveTaskTemplate(template)
	if err != nil {
		t.Fatalf("SaveTaskTemplate returned error: %v", err)
	}
	if saved.ID != "user-task-1" || saved.Title != "Custom reminder" || saved.Prompt != "remind me" {
		t.Fatalf("saved template = %+v, want trimmed fields", saved)
	}
	if saved.Source != TemplateSourceUser || saved.Kind != TemplateKindTask || saved.Interval != "30m" {
		t.Fatalf("saved template metadata = %+v, want user task defaults", saved)
	}
	if _, err := os.Stat(filepath.Join(store.dir, "templates.sqlite")); err != nil {
		t.Fatalf("templates sqlite database was not created: %v", err)
	}
	if _, err := os.Stat(store.templatesPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("SaveTaskTemplate should not write templates.json, stat error = %v", err)
	}

	got, err := store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []TaskTemplate{saved}) {
		t.Fatalf("LoadTaskTemplates = %+v, want %+v", got, []TaskTemplate{saved})
	}
	if got := countTemplateRows(t, store.dir); got != 1 {
		t.Fatalf("sqlite template rows = %d, want 1", got)
	}
}

func TestStoreMigratesLegacyTemplatesJSONToSQLiteOnce(t *testing.T) {
	dir := t.TempDir()
	legacy := TaskTemplate{ID: "legacy-template", Title: "Legacy", Prompt: "legacy prompt", Interval: "45m"}
	b, err := json.Marshal(templatesFile{Templates: []TaskTemplate{legacy}})
	if err != nil {
		t.Fatalf("marshal legacy templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, templatesFileName), b, 0o600); err != nil {
		t.Fatalf("write legacy templates: %v", err)
	}
	store := NewStore(dir)

	got, err := store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "legacy-template" || got[0].Source != TemplateSourceUser {
		t.Fatalf("migrated templates = %+v, want one user legacy template", got)
	}
	if got := countTemplateRows(t, dir); got != 1 {
		t.Fatalf("sqlite template rows after migration = %d, want 1", got)
	}

	if err := store.DeleteTaskTemplate("legacy-template"); err != nil {
		t.Fatalf("DeleteTaskTemplate migrated template: %v", err)
	}
	got, err = store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates after delete: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("deleted migrated template was loaded again from legacy JSON: %+v", got)
	}
}

func TestStoreMigratesLegacyTemplatesJSONConcurrently(t *testing.T) {
	dir := t.TempDir()
	legacy := TaskTemplate{ID: "legacy-template", Title: "Legacy", Prompt: "legacy prompt", Interval: "45m"}
	b, err := json.Marshal(templatesFile{Templates: []TaskTemplate{legacy}})
	if err != nil {
		t.Fatalf("marshal legacy templates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, templatesFileName), b, 0o600); err != nil {
		t.Fatalf("write legacy templates: %v", err)
	}

	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			templates, err := NewStore(dir).LoadTaskTemplates()
			if err != nil {
				errs <- err
				return
			}
			if len(templates) != 1 || templates[0].ID != "legacy-template" {
				errs <- fmt.Errorf("templates = %+v, want migrated legacy template", templates)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		return
	}
	if got := countTemplateRows(t, dir); got != 1 {
		t.Fatalf("sqlite template rows after concurrent migration = %d, want 1", got)
	}
}

func TestStoreSaveTaskTemplateUpserts(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	if _, err := store.SaveTaskTemplate(TaskTemplate{ID: "user-task-1", Title: "Old", Prompt: "old", Interval: "30m"}); err != nil {
		t.Fatalf("SaveTaskTemplate old: %v", err)
	}
	saved, err := store.SaveTaskTemplate(TaskTemplate{ID: "user-task-1", Title: "New", Prompt: "new", Interval: "1h"})
	if err != nil {
		t.Fatalf("SaveTaskTemplate new: %v", err)
	}

	got, err := store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates: %v", err)
	}
	if len(got) != 1 || got[0].Title != "New" || got[0].Interval != "1h" || !reflect.DeepEqual(got[0], saved) {
		t.Fatalf("templates = %+v, want one upserted template %+v", got, saved)
	}
}

func TestStoreDeleteTaskTemplate(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	if _, err := store.SaveTaskTemplate(TaskTemplate{ID: "user-task-1", Title: "Delete me", Prompt: "delete", Interval: "30m"}); err != nil {
		t.Fatalf("SaveTaskTemplate first: %v", err)
	}
	if _, err := store.SaveTaskTemplate(TaskTemplate{ID: "user-task-2", Title: "Keep me", Prompt: "keep", Interval: "1h"}); err != nil {
		t.Fatalf("SaveTaskTemplate second: %v", err)
	}

	if err := store.DeleteTaskTemplate(" user-task-1 "); err != nil {
		t.Fatalf("DeleteTaskTemplate: %v", err)
	}
	got, err := store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates: %v", err)
	}
	if len(got) != 1 || got[0].ID != "user-task-2" {
		t.Fatalf("templates after delete = %+v, want only user-task-2", got)
	}
	if got := countTemplateRows(t, store.dir); got != 2 {
		t.Fatalf("sqlite template rows after soft delete = %d, want 2", got)
	}
	if err := store.RestoreTaskTemplate("user-task-1"); err != nil {
		t.Fatalf("RestoreTaskTemplate: %v", err)
	}
	got, err = store.LoadTaskTemplates()
	if err != nil {
		t.Fatalf("LoadTaskTemplates after restore: %v", err)
	}
	if len(got) != 2 || got[0].ID != "user-task-1" || got[1].ID != "user-task-2" {
		t.Fatalf("templates after restore = %+v, want both user templates", got)
	}
	if err := store.DeleteTaskTemplate("missing-template"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("DeleteTaskTemplate missing error = %v, want %v", err, ErrTaskNotFound)
	}
	if err := store.RestoreTaskTemplate("missing-template"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("RestoreTaskTemplate missing error = %v, want %v", err, ErrTaskNotFound)
	}
}

func TestStoreRejectsDuplicateTaskID(t *testing.T) {
	store := NewStore(t.TempDir())
	first := validStoreTask()
	second := validStoreTask()
	second.ID = " task-1 "
	second.Name = "Other"

	if err := store.SaveTasks([]Task{first, second}); !errors.Is(err, ErrDuplicateTaskID) {
		t.Fatalf("SaveTasks error = %v, want %v", err, ErrDuplicateTaskID)
	}
	if _, err := os.Stat(store.tasksPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("duplicate tasks should not create tasks file, stat error = %v", err)
	}
}

func TestStoreRejectsInvalidTask(t *testing.T) {
	store := NewStore(t.TempDir())
	task := validStoreTask()
	task.Schedule.Kind = "cron"

	if err := store.SaveTasks([]Task{task}); !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("SaveTasks error = %v, want %v", err, ErrInvalidSchedule)
	}
	if _, err := os.Stat(store.tasksPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid task should not create tasks file, stat error = %v", err)
	}
}

func TestStoreTasksJSONDoesNotContainSecrets(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.SaveTasks([]Task{validStoreTask()}); err != nil {
		t.Fatalf("SaveTasks returned error: %v", err)
	}
	b, err := os.ReadFile(store.tasksPath())
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	lower := strings.ToLower(string(b))
	for _, word := range []string{"apikey", "api_key", "token", "secret", "providerkey"} {
		if strings.Contains(lower, word) {
			t.Fatalf("tasks.json contains secret-like word %q: %s", word, b)
		}
	}
}

func TestStoreInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, tasksFileName), []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid tasks file: %v", err)
	}

	if _, err := NewStore(dir).LoadTasks(); err == nil {
		t.Fatal("LoadTasks should reject invalid JSON")
	} else if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("LoadTasks error = %v, want decode context", err)
	}
}

func countTemplateRows(t *testing.T, dir string) int {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "templates.sqlite"))
	if err != nil {
		t.Fatalf("open template sqlite db: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM task_templates`).Scan(&count); err != nil {
		t.Fatalf("count task_templates: %v", err)
	}
	return count
}

func validStoreTask() Task {
	return Task{
		ID:      "task-1",
		Name:    "Daily check",
		Enabled: true,
		Prompt: PromptConfig{
			Prompt: "Check the project.",
		},
		Schedule: Schedule{
			Kind:  ScheduleKindInterval,
			Every: "30m",
		},
		Execution: ExecutionPolicy{
			ApprovalMode:    ApprovalModeAuto,
			ApprovalTimeout: DefaultApprovalTimeoutString,
		},
	}
}

func TestPersistentStoreReusesAndReleasesTemplateDatabase(t *testing.T) {
	dir := t.TempDir()
	store := NewPersistentStore(dir)
	if _, err := store.LoadTaskTemplates(); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if _, err := store.LoadTaskTemplates(); err != nil {
		t.Fatalf("second load: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, templatesDBFileName)); err != nil {
		t.Fatalf("database remained open after Close: %v", err)
	}
}
