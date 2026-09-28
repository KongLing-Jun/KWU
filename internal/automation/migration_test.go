package automation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeartbeatMigrationIdempotent(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	writeLegacyHeartbeatFile(t, filepath.Join(stateDir, legacyHeartbeatFileName), legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
	}})

	first, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("first MigrateHeartbeatIfNeeded: %v", err)
	}
	if first.Status != HeartbeatMigrationStatusMigrated {
		t.Fatalf("first status = %q, want %q", first.Status, HeartbeatMigrationStatusMigrated)
	}
	second, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("second MigrateHeartbeatIfNeeded: %v", err)
	}
	if second.Status != HeartbeatMigrationStatusAlreadyMigrated {
		t.Fatalf("second status = %q, want %q", second.Status, HeartbeatMigrationStatusAlreadyMigrated)
	}

	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "heartbeat-daily" {
		t.Fatalf("tasks = %+v, want one migrated task", tasks)
	}
}

func TestHeartbeatMigrationMissingFile(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.Status != HeartbeatMigrationStatusNotFound || result.SourcePath != filepath.Join(stateDir, legacyHeartbeatFileName) {
		t.Fatalf("result = %+v, want not_found for missing legacy file", result)
	}
	assertHeartbeatMigrationTaskCount(t, store, 0)
}

func TestHeartbeatMigrationEmptyFile(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	if err := os.WriteFile(filepath.Join(stateDir, legacyHeartbeatFileName), []byte("  \n"), 0o600); err != nil {
		t.Fatalf("WriteFile legacy config: %v", err)
	}

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.Status != HeartbeatMigrationStatusMigrated || len(result.TaskIDs) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v, want empty migration without skipped tasks", result)
	}
	assertHeartbeatMigrationTaskCount(t, store, 0)
}

func TestHeartbeatMigrationWritesMarker(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	sourcePath := filepath.Join(stateDir, legacyHeartbeatFileName)
	writeLegacyHeartbeatFile(t, sourcePath, legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
	}})

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.Status != HeartbeatMigrationStatusMigrated {
		t.Fatalf("status = %q, want migrated", result.Status)
	}

	marker := readHeartbeatMigrationMarkerForTest(t, store)
	if marker.Version != heartbeatMigrationVersion || marker.SourcePath != sourcePath || marker.SourceHash == "" || marker.MigratedAt.IsZero() {
		t.Fatalf("marker = %+v, want populated marker", marker)
	}
	if got := marker.TaskIDs["daily"]; got != "heartbeat-daily" {
		t.Fatalf("marker TaskIDs[daily] = %q, want heartbeat-daily", got)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("legacy heartbeat file should remain: %v", err)
	}
}

func TestHeartbeatMigrationSourceChanged(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	sourcePath := filepath.Join(stateDir, legacyHeartbeatFileName)
	writeLegacyHeartbeatFile(t, sourcePath, legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
	}})
	if _, err := MigrateHeartbeatIfNeeded(context.Background(), store); err != nil {
		t.Fatalf("first MigrateHeartbeatIfNeeded: %v", err)
	}
	writeLegacyHeartbeatFile(t, sourcePath, legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check changed", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
		{ID: "weekly", Title: "Weekly check", Prompt: "check", Interval: "168h|weekly:fri@09:00", Enabled: true},
	}})

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("changed MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.Status != HeartbeatMigrationStatusSourceChanged {
		t.Fatalf("status = %q, want source_changed", result.Status)
	}
	if !strings.Contains(result.Notice, "automation task list") {
		t.Fatalf("notice = %q, want automation task list guidance", result.Notice)
	}
	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Name != "Daily check" {
		t.Fatalf("tasks after changed source = %+v, want original single task", tasks)
	}
}

func TestHeartbeatMigrationMarkerFailureNoDuplicates(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	writeLegacyHeartbeatFile(t, filepath.Join(stateDir, legacyHeartbeatFileName), legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
	}})
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		t.Fatalf("MkdirAll store dir: %v", err)
	}
	blocker := filepath.Join(store.dir, heartbeatMigrationDirName)
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile blocker: %v", err)
	}

	first, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("first MigrateHeartbeatIfNeeded: %v", err)
	}
	if first.Status != HeartbeatMigrationStatusMarkerFailed {
		t.Fatalf("first status = %q, want marker_failed", first.Status)
	}
	assertHeartbeatMigrationTaskCount(t, store, 1)

	if err := os.Remove(blocker); err != nil {
		t.Fatalf("Remove blocker: %v", err)
	}
	second, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("second MigrateHeartbeatIfNeeded: %v", err)
	}
	if second.Status != HeartbeatMigrationStatusMigrated {
		t.Fatalf("second status = %q, want migrated", second.Status)
	}
	assertHeartbeatMigrationTaskCount(t, store, 1)
}

func TestHeartbeatMigrationPartialFailureKeepsValidTasks(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	writeLegacyHeartbeatFile(t, filepath.Join(stateDir, legacyHeartbeatFileName), legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
		{ID: "bad", Title: "Bad check", Prompt: "bad", Interval: "soon", Enabled: true},
	}})

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.Status != HeartbeatMigrationStatusMigrated {
		t.Fatalf("status = %q, want migrated", result.Status)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].LegacyID != "bad" {
		t.Fatalf("skipped = %+v, want bad task skipped", result.Skipped)
	}

	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "heartbeat-daily" {
		t.Fatalf("tasks = %+v, want only valid migrated task", tasks)
	}
}

func TestHeartbeatMigrationAllInvalidTasksRecordsFailureWithoutError(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	writeLegacyHeartbeatFile(t, filepath.Join(stateDir, legacyHeartbeatFileName), legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "bad", Title: "Bad check", Prompt: "bad", Interval: "soon", Enabled: true},
	}})

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.Status != HeartbeatMigrationStatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if len(result.Skipped) != 1 || !strings.Contains(result.ErrorSummary, "skipped 1") {
		t.Fatalf("result = %+v, want visible skipped failure", result)
	}
	assertHeartbeatMigrationTaskCount(t, store, 0)
}

func TestHeartbeatMigrationDeduplicatesExistingAutomationTaskID(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	if err := store.SaveTasks([]Task{{
		ID:       "heartbeat-daily",
		Name:     "Existing daily",
		Prompt:   PromptConfig{Prompt: "existing"},
		Schedule: Schedule{Kind: ScheduleKindInterval, Every: "30m"},
	}}); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	writeLegacyHeartbeatFile(t, filepath.Join(stateDir, legacyHeartbeatFileName), legacyHeartbeatConfig{Tasks: []legacyHeartbeatTask{
		{ID: "daily", Title: "Daily check", Prompt: "check", Interval: "24h|daily@09:00", Enabled: true},
		{ID: "weekly", Title: "Weekly check", Prompt: "check", Interval: "168h|weekly:fri@09:00", Enabled: true},
	}})

	result, err := MigrateHeartbeatIfNeeded(context.Background(), store)
	if err != nil {
		t.Fatalf("MigrateHeartbeatIfNeeded: %v", err)
	}
	if result.TaskIDs["daily"] != "heartbeat-daily" || result.TaskIDs["weekly"] != "heartbeat-weekly" {
		t.Fatalf("task IDs = %+v, want legacy id map", result.TaskIDs)
	}
	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("tasks len = %d, want existing daily plus weekly: %+v", len(tasks), tasks)
	}
}

func TestServiceMigrationStatusReportsInvalidJSON(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	if err := os.WriteFile(filepath.Join(stateDir, legacyHeartbeatFileName), []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("WriteFile legacy config: %v", err)
	}

	svc := NewService(store, nil)
	status, err := svc.MigrationStatus(context.Background())
	if err != nil {
		t.Fatalf("MigrationStatus: %v", err)
	}
	if status.Status != HeartbeatMigrationStatusFailed || !strings.Contains(status.ErrorSummary, "decode") {
		t.Fatalf("status = %+v, want visible decode failure", status)
	}
	tasks, err := svc.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks after migration failure: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks = %+v, want empty service after ignored migration failure", tasks)
	}
}

func TestServiceCreationIgnoresHeartbeatMigrationFailure(t *testing.T) {
	stateDir, store := newHeartbeatMigrationTestStore(t)
	if err := os.WriteFile(filepath.Join(stateDir, legacyHeartbeatFileName), []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("WriteFile legacy config: %v", err)
	}

	svc := NewService(store, nil)
	tasks, err := svc.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("ListTasks after migration failure: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks = %+v, want empty service after ignored migration failure", tasks)
	}
}

func newHeartbeatMigrationTestStore(t *testing.T) (string, *Store) {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("KWU_STATE_HOME", stateDir)
	return stateDir, NewStore(filepath.Join(stateDir, "automation"))
}

func readHeartbeatMigrationMarkerForTest(t *testing.T, store *Store) HeartbeatMigrationMarker {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(store.dir, heartbeatMigrationDirName, heartbeatMigrationMarkerFileName))
	if err != nil {
		t.Fatalf("ReadFile marker: %v", err)
	}
	var marker HeartbeatMigrationMarker
	if err := json.Unmarshal(b, &marker); err != nil {
		t.Fatalf("Unmarshal marker: %v", err)
	}
	return marker
}

func assertHeartbeatMigrationTaskCount(t *testing.T, store *Store, want int) {
	t.Helper()
	tasks, err := store.LoadTasks()
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != want {
		t.Fatalf("tasks len = %d, want %d: %+v", len(tasks), want, tasks)
	}
}
