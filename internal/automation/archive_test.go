package automation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunIndexRebuildsWhenMissingCorruptOrDirtyAndTracksManifestUpdates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "automation")
	store := NewStore(root)
	manifest := RunManifest{
		RunID: "run-1", TaskID: "task-1", TaskName: "Daily digest", Scope: "project",
		Status: RunStatusQueued, Trigger: TriggerManual, StartedAt: time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC),
	}
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}
	assertIndexedRunStatus(t, store, RunStatusQueued)

	manifest.Status = RunStatusSucceeded
	manifest.FinishedAt = manifest.StartedAt.Add(time.Minute)
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest update: %v", err)
	}
	assertIndexedRunStatus(t, store, RunStatusSucceeded)

	for _, damage := range []func() error{
		func() error { return os.Remove(filepath.Join(root, runsIndexFileName)) },
		func() error { return os.WriteFile(filepath.Join(root, runsIndexFileName), []byte("not sqlite"), 0o600) },
		func() error {
			return os.WriteFile(filepath.Join(root, runsIndexDirtyFileName), []byte("dirty\n"), 0o600)
		},
	} {
		if err := damage(); err != nil {
			t.Fatalf("damage run index: %v", err)
		}
		assertIndexedRunStatus(t, store, RunStatusSucceeded)
		if _, err := os.Stat(filepath.Join(root, runsIndexDirtyFileName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("dirty marker remains after rebuild: %v", err)
		}
	}
}

func TestRunIndexQueryHonorsCanceledContext(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := store.indexedRunPage(ctx, RunFilter{Limit: 20}, "", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("indexedRunPage error = %v, want context.Canceled", err)
	}
}

func assertIndexedRunStatus(t *testing.T, store *Store, want RunStatus) {
	t.Helper()
	page, _, err := store.indexedRunPage(context.Background(), RunFilter{Scope: "project", Limit: 20}, "", nil)
	if err != nil {
		t.Fatalf("indexedRunPage: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != want {
		t.Fatalf("indexed page = %+v, want status %s", page, want)
	}
}

func TestCreateRunWritesManifest(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	task.Prompt.Model = "test-model"
	task.Prompt.BillingSource = "subscription_pool"
	task.WorkspaceRoot = "D:/project/KWU"
	task.Revision = 7
	task.Execution.ApprovalMode = ApprovalModeYolo
	task.EffectiveRange = EffectiveRange{
		From:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
	task.Capabilities = CapabilitySelection{ModelRef: "test-model", SkillIDs: []string{"review"}, ExpertID: "architect", ConnectorIDs: []string{"github"}}
	task.Delivery = DeliveryPolicy{Enabled: true, Targets: []DeliveryTarget{{Type: "local_test", Ref: "inbox"}}}

	manifest, paths, err := store.CreateRun(task, string(TriggerManual), "desktop:123")
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if manifest.RunID == "" || manifest.Status != RunStatusQueued || manifest.Trigger != TriggerManual || manifest.TriggerReason == "" {
		t.Fatalf("manifest has wrong basics: %+v", manifest)
	}
	if manifest.SessionPath != paths.Session || manifest.ResultPath != paths.Result {
		t.Fatalf("manifest paths = %q/%q, want %q/%q", manifest.SessionPath, manifest.ResultPath, paths.Session, paths.Result)
	}
	if manifest.Owner != "desktop:123" || manifest.LeaseID != manifest.RunID || manifest.EventsPath != paths.Events {
		t.Fatalf("manifest observability fields = %+v, want owner lease and events path", manifest)
	}
	if manifest.Model != task.Prompt.Model || manifest.BillingSource != task.Prompt.BillingSource {
		t.Fatalf("manifest route = %q/%q, want %q/%q", manifest.Model, manifest.BillingSource, task.Prompt.Model, task.Prompt.BillingSource)
	}
	if manifest.TaskRevision != 7 || manifest.Capabilities.ModelRef != "test-model" || !reflect.DeepEqual(manifest.Capabilities.ConnectorIDs, []string{"github"}) {
		t.Fatalf("manifest capability snapshot = revision %d %+v", manifest.TaskRevision, manifest.Capabilities)
	}
	if manifest.Capabilities.ApprovalMode != ApprovalModeYolo || !reflect.DeepEqual(manifest.Schedule, task.Schedule) || !reflect.DeepEqual(manifest.EffectiveRange, task.EffectiveRange) {
		t.Fatalf("manifest execution snapshot = approval %q schedule %+v range %+v", manifest.Capabilities.ApprovalMode, manifest.Schedule, manifest.EffectiveRange)
	}
	if !manifest.Delivery.Enabled || len(manifest.Delivery.Targets) != 1 || manifest.Delivery.Targets[0].Ref != "inbox" {
		t.Fatalf("manifest delivery policy = %+v", manifest.Delivery)
	}
	if filepath.Base(paths.Manifest) != manifestFileName || filepath.Base(paths.Events) != eventsFileName || filepath.Base(paths.Session) != sessionFileName || filepath.Base(paths.Result) != resultFileName {
		t.Fatalf("run paths are not stable: %+v", paths)
	}
	if _, err := os.Stat(paths.Dir); err != nil {
		t.Fatalf("run dir was not created: %v", err)
	}
	got, err := store.ReadRunManifest(task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("ReadRunManifest returned error: %v", err)
	}
	if !reflect.DeepEqual(got, manifest) {
		t.Fatalf("ReadRunManifest = %+v, want %+v", got, manifest)
	}
}

func TestRunManifestObservabilityFields(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	task.Prompt.Model = "test-model"
	task.Prompt.BillingSource = "newapi_byok"
	task.WorkspaceRoot = "D:/project/KWU"
	startedAt := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)

	manifest, paths, err := store.createRun(task, string(TriggerManual), "manual test", "serve:42", "run-1", startedAt)
	if err != nil {
		t.Fatalf("createRun returned error: %v", err)
	}
	manifest.Condition = ConditionResult{Reason: "condition ok"}
	manifest.ConditionReason = manifest.Condition.Reason
	manifest.ErrorCode = RunErrorRunnerFailed
	manifest.ErrorMessage = "runner failed"
	manifest.Error = manifest.ErrorMessage
	if err := manifest.Transition(RunStatusFailed, "", startedAt.Add(2*time.Second)); err != nil {
		t.Fatalf("Transition failed: %v", err)
	}
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest returned error: %v", err)
	}

	b, err := os.ReadFile(paths.Manifest)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("manifest JSON decode: %v", err)
	}
	for _, key := range []string{"taskId", "runId", "status", "trigger", "triggerReason", "owner", "leaseId", "model", "billingSource", "workspaceRoot", "startedAt", "finishedAt", "durationMs", "sessionPath", "eventsPath", "resultPath", "errorCode", "errorMessage"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("manifest missing %q: %s", key, b)
		}
	}
}

func TestRunManifestIncludesZeroDuration(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	manifest, paths, err := store.CreateRun(task, string(TriggerSchedule), "serve:1")
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if manifest.DurationMs != 0 {
		t.Fatalf("DurationMs = %d, want zero before finish", manifest.DurationMs)
	}
	b, err := os.ReadFile(paths.Manifest)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	if !strings.Contains(string(b), `"durationMs": 0`) {
		t.Fatalf("manifest should include zero durationMs: %s", b)
	}
}

func TestRunPathsStayUnderRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "automation")
	store := NewStore(root)
	paths, err := store.RunPaths(`..\..\outside`, `run/../../x`)
	if err != nil {
		t.Fatalf("RunPaths returned error: %v", err)
	}
	for _, path := range []string{paths.Dir, paths.Manifest, paths.Events, paths.Session, paths.Result} {
		assertUnderRoot(t, root, path)
	}
	if strings.Contains(filepath.ToSlash(paths.Dir), "../") || strings.Contains(filepath.ToSlash(paths.Dir), "/..") {
		t.Fatalf("path was not encoded safely: %q", paths.Dir)
	}
	if _, err := store.RunPaths("", "run"); err == nil {
		t.Fatal("RunPaths should reject empty task id")
	}
}

func TestListRunsLimit(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	base := time.Date(2026, 6, 27, 9, 0, 0, 0, time.UTC)

	var newest RunManifest
	for i := 0; i < 3; i++ {
		manifest, _, err := store.CreateRun(task, string(TriggerSchedule), "serve:1")
		if err != nil {
			t.Fatalf("CreateRun %d returned error: %v", i, err)
		}
		manifest.StartedAt = base.Add(time.Duration(i) * time.Minute)
		manifest.Summary = manifest.RunID
		if err := store.SaveRunManifest(manifest); err != nil {
			t.Fatalf("SaveRunManifest %d returned error: %v", i, err)
		}
		newest = manifest
	}

	runs, err := store.ListRuns(task.ID, 2)
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("ListRuns returned %d runs, want 2", len(runs))
	}
	if runs[0].RunID != newest.RunID {
		t.Fatalf("latest run = %q, want %q", runs[0].RunID, newest.RunID)
	}
}

func TestListRunsUsesManifest(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	task.Prompt.BillingSource = "subscription_pool"
	manifest, paths, err := store.CreateRun(task, string(TriggerManual), "desktop:123")
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	manifest.Summary = "from manifest"
	manifest.ErrorCode = RunErrorRunnerFailed
	manifest.ErrorMessage = "from manifest"
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest returned error: %v", err)
	}
	if err := os.WriteFile(paths.Events, []byte("{not event json"), 0o600); err != nil {
		t.Fatalf("write broken events: %v", err)
	}

	runs, err := store.ListRuns(task.ID, 1)
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Summary != "from manifest" ||
		runs[0].BillingSource != task.Prompt.BillingSource || runs[0].ErrorCode != RunErrorRunnerFailed {
		t.Fatalf("runs = %+v, want manifest projection", runs)
	}
}

func TestListRunsPageAcrossTasksUsesStableCursorAndFilters(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	for index, taskID := range []string{"personal-1", "project-1", "project-2"} {
		task := validStoreTask()
		task.ID = taskID
		if strings.HasPrefix(taskID, "project-") {
			task.Scope = "project"
		}
		manifest, _, err := store.createRun(task, string(TriggerManual), "test", "desktop:1", "run-"+taskID, base.Add(time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatalf("createRun(%s): %v", taskID, err)
		}
		if err := store.SaveRunManifest(manifest); err != nil {
			t.Fatalf("SaveRunManifest(%s): %v", taskID, err)
		}
	}
	first, err := store.ListRunsPage(RunFilter{Limit: 2}, "")
	if err != nil {
		t.Fatalf("ListRunsPage first: %v", err)
	}
	if len(first.Items) != 2 || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first page = %+v", first)
	}
	second, err := store.ListRunsPage(RunFilter{Limit: 2}, first.NextCursor)
	if err != nil {
		t.Fatalf("ListRunsPage second: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore || second.Items[0].TaskID != "personal-1" {
		t.Fatalf("second page = %+v", second)
	}
	projectRuns, err := store.ListRunsPage(RunFilter{Scope: "project", Limit: 10}, "")
	if err != nil {
		t.Fatalf("ListRunsPage project: %v", err)
	}
	if len(projectRuns.Items) != 2 {
		t.Fatalf("project page = %+v, want two project runs", projectRuns)
	}
	if _, err := store.ListRunsPage(RunFilter{Limit: 2}, "not-a-cursor"); err == nil {
		t.Fatal("invalid cursor should fail")
	}
	if _, err := store.ListRunsPage(RunFilter{TaskID: "personal-1", Limit: 2}, first.NextCursor); err == nil {
		t.Fatal("cursor from another filter should fail")
	}
	if _, err := store.ListRunsPage(RunFilter{Limit: 101}, ""); err == nil {
		t.Fatal("oversize page limit should fail")
	}
	if _, err := store.ListRunsPage(RunFilter{Limit: 2}, strings.Repeat("a", 2049)); err == nil {
		t.Fatal("oversize cursor should fail")
	}
	withoutAnchor := []RunSummary{second.Items[0]}
	start, err := runPageStart(withoutAnchor, first.NextCursor, RunFilter{Limit: 2})
	if err != nil || start != 0 {
		t.Fatalf("keyset cursor after anchor cleanup = start %d, err %v", start, err)
	}
}

func TestListRunsPageFiltersTriggerAndTaskNameAndBindsEveryCursorCondition(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	fixtures := []struct {
		id      string
		name    string
		trigger Trigger
		at      time.Time
	}{
		{id: "alpha", name: "Alpha Daily", trigger: TriggerSchedule, at: base.Add(2 * time.Minute)},
		{id: "beta", name: "Beta Manual", trigger: TriggerManual, at: base.Add(time.Minute)},
		{id: "gamma", name: "Gamma Manual", trigger: TriggerManual, at: base},
	}
	for _, fixture := range fixtures {
		task := validStoreTask()
		task.ID, task.Name, task.Scope = fixture.id, fixture.name, "project"
		if _, _, err := store.createRun(task, string(fixture.trigger), "test", "desktop:1", "run-"+fixture.id, fixture.at); err != nil {
			t.Fatalf("createRun(%s): %v", fixture.id, err)
		}
	}
	filtered, err := store.ListRunsPage(RunFilter{Scope: "PROJECT", Trigger: TriggerManual, Search: "  beta  ", Limit: 10}, "")
	if err != nil {
		t.Fatalf("ListRunsPage filtered: %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].TaskID != "beta" {
		t.Fatalf("filtered items = %+v, want beta only", filtered.Items)
	}

	filter := RunFilter{Scope: "project", Limit: 1}
	first, err := store.ListRunsPage(filter, "")
	if err != nil || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first page = %+v, err = %v", first, err)
	}
	changed := []RunFilter{
		{Scope: "project", TaskID: "alpha", Limit: 1},
		{Scope: "project", Trigger: TriggerManual, Limit: 1},
		{Scope: "project", Search: "alpha", Limit: 1},
		{Scope: "project", Limit: 2},
	}
	for _, changedFilter := range changed {
		if _, err := store.ListRunsPage(changedFilter, first.NextCursor); err == nil {
			t.Fatalf("cursor should reject changed filter %+v", changedFilter)
		}
	}
}

func TestCompletedRunRenameTrashRestorePreservesArtifactsAndSearch(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	manifest := RunManifest{
		RunID: "run-1", TaskID: "task-1", TaskName: "Original task snapshot", Scope: "project",
		Status: RunStatusSucceeded, Trigger: TriggerManual, StartedAt: time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 8, 13, 9, 1, 0, 0, time.UTC),
	}
	if err := store.SaveRunManifest(manifest); err != nil {
		t.Fatalf("SaveRunManifest: %v", err)
	}
	paths, _ := store.RunPaths(manifest.TaskID, manifest.RunID)
	for path, contents := range map[string]string{paths.Events: "event\n", paths.Session: "session\n", paths.Result: "result\n", paths.Deliveries: "delivery\n"} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write artifact %s: %v", path, err)
		}
	}
	updated, err := store.RenameRun(manifest.TaskID, manifest.RunID, "Quarterly close review")
	if err != nil {
		t.Fatalf("RenameRun: %v", err)
	}
	if updated.TaskName != manifest.TaskName || updated.DisplayName != "Quarterly close review" {
		t.Fatalf("renamed manifest = %+v", updated)
	}
	page, err := store.ListRunsPage(RunFilter{Scope: "project", Search: "quarterly", Limit: 20}, "")
	if err != nil || len(page.Items) != 1 || page.Items[0].DisplayName != updated.DisplayName {
		t.Fatalf("display-name search = %+v, %v", page, err)
	}
	if _, err := store.TrashRun(manifest.TaskID, manifest.RunID); err != nil {
		t.Fatalf("TrashRun: %v", err)
	}
	if _, err := os.Stat(paths.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active run remains after trash: %v", err)
	}
	trashDir, _ := store.runTrashPath(manifest.TaskID, manifest.RunID)
	for _, name := range []string{manifestFileName, eventsFileName, sessionFileName, resultFileName, deliveriesFileName} {
		if _, err := os.Stat(filepath.Join(trashDir, name)); err != nil {
			t.Fatalf("trashed artifact %s: %v", name, err)
		}
	}
	if runs, err := store.ListRuns(manifest.TaskID, 0); err != nil || len(runs) != 0 {
		t.Fatalf("runs after trash = %+v, %v", runs, err)
	}
	if _, err := store.RestoreRun(manifest.TaskID, manifest.RunID); err != nil {
		t.Fatalf("RestoreRun: %v", err)
	}
	restored, err := store.ReadRunManifest(manifest.TaskID, manifest.RunID)
	if err != nil || restored.DisplayName != updated.DisplayName || restored.TaskName != manifest.TaskName {
		t.Fatalf("restored = %+v, %v", restored, err)
	}
}

func TestRunManagementPreservesActiveRenameAndRejectsActiveTrash(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	running := RunManifest{RunID: "run-running", TaskID: "task-1", TaskName: "Task", Status: RunStatusRunning, Trigger: TriggerManual, StartedAt: time.Now().UTC()}
	if err := store.SaveRunManifest(running); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RenameRun(running.TaskID, running.RunID, "New name"); err != nil {
		t.Fatalf("RenameRun running run: %v", err)
	}
	if _, err := store.TrashRun(running.TaskID, running.RunID); err == nil {
		t.Fatal("TrashRun accepted running run")
	}
	running.Status = RunStatusCanceled
	if err := store.SaveRunManifest(running); err != nil {
		t.Fatal(err)
	}
	renamed, err := store.ReadRunManifest(running.TaskID, running.RunID)
	if err != nil || renamed.DisplayName != "New name" {
		t.Fatalf("active rename was overwritten by status save: %+v, %v", renamed, err)
	}
	blocked := running
	blocked.RunID = "run-blocked"
	blocked.Status = RunStatusBlocked
	if err := store.SaveRunManifest(blocked); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TrashRun(blocked.TaskID, blocked.RunID); err != nil {
		t.Fatalf("TrashRun blocked run: %v", err)
	}
	finished := running
	finished.RunID = "run-finished"
	finished.Status = RunStatusSucceeded
	if err := store.SaveRunManifest(finished); err != nil {
		t.Fatal(err)
	}
	if _, err := store.TrashRun(finished.TaskID, finished.RunID); err != nil {
		t.Fatal(err)
	}
	paths, _ := store.RunPaths(finished.TaskID, finished.RunID)
	if err := os.MkdirAll(paths.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RestoreRun(finished.TaskID, finished.RunID); err == nil {
		t.Fatal("RestoreRun accepted destination collision")
	}
}

func TestResultReadWrite(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "automation"))
	task := validStoreTask()
	manifest, _, err := store.CreateRun(task, string(TriggerManual), "desktop:123")
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	if err := store.WriteResult(task.ID, manifest.RunID, "# Result\n\nok\n"); err != nil {
		t.Fatalf("WriteResult returned error: %v", err)
	}
	got, err := store.ReadResult(task.ID, manifest.RunID)
	if err != nil {
		t.Fatalf("ReadResult returned error: %v", err)
	}
	if got != "# Result\n\nok\n" {
		t.Fatalf("ReadResult = %q", got)
	}
}

func assertUnderRoot(t *testing.T, root, path string) {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("Rel(%q, %q): %v", root, path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		t.Fatalf("%q escapes root %q", path, root)
	}
}
