package jobs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"kwu/internal/event"
)

func TestCompletedJobPersistsOutputAndReleasesMemory(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	m := NewManager(event.Discard)
	defer m.Close()
	m.SetActiveSessionPath("session", sessionPath)

	j := m.StartForSession("session", "bash", "persist", func(_ context.Context, out io.Writer) (string, error) {
		_, _ = io.WriteString(out, strings.Repeat("x", defaultTailBytes+1024))
		return "", nil
	})
	<-j.done

	j.mu.Lock()
	tailLen := len(j.tail)
	result := j.result
	artifactPath := j.artifactPath
	j.mu.Unlock()

	if tailLen != 0 {
		t.Fatalf("completed artifact-backed job kept %d tail bytes, want 0", tailLen)
	}
	if result != "" {
		t.Fatalf("completed artifact-backed job kept result %q, want empty", result)
	}
	if artifactPath == "" {
		t.Fatal("artifact path should be set")
	}

	res := m.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || len(res[0].Output) != defaultTailBytes+1024 {
		t.Fatalf("wait output len = %d, want %d", len(res[0].Output), defaultTailBytes+1024)
	}
}

func TestRestoreSessionArtifactsAndAdvanceSequence(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	first := NewManager(event.Discard)
	first.SetActiveSessionPath("session", sessionPath)
	j := first.StartControlledForSession("session", "task", "sa_persisted", "answer", func(context.Context, io.Writer) (string, error) {
		return "persisted answer", nil
	})
	<-j.done
	first.Close()

	second := NewManager(event.Discard)
	defer second.Close()
	second.SetActiveSessionPath("session", sessionPath)

	res := second.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || !strings.Contains(res[0].Output, "persisted answer") {
		t.Fatalf("restored wait = %+v, want persisted answer", res)
	}
	controlled, ok := second.WaitControlledForSession(context.Background(), "session", "task", "sa_persisted", 1)
	if !ok || !strings.Contains(controlled.Output, "persisted answer") {
		t.Fatalf("restored controlled wait = %+v, ok=%v", controlled, ok)
	}
	if got := second.WaitForSession(context.Background(), "session", nil, 1); len(got) != 0 {
		t.Fatalf("wait without ids should ignore restored completed artifacts, got %+v", got)
	}

	next := second.StartForSession("session", "bash", "next", func(context.Context, io.Writer) (string, error) {
		return "", nil
	})
	<-next.done
	if next.ID == j.ID {
		t.Fatalf("new job reused restored id %q", next.ID)
	}
}

func TestTaskRunSnapshotIsSessionScopedAndRestoresDependencies(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	first := NewManager(event.Discard)
	first.SetActiveSessionPath("parent-a", sessionPath)
	job := first.StartControlledForSessionWithDependencies("parent-a", "task", "sa_task", "dependent", []string{"task-1", "task-2"}, func(context.Context, io.Writer) (string, error) {
		return "completed", nil
	})
	<-job.done

	snapshot, ok := first.TaskRunForSession("parent-a", job.ID)
	if !ok || snapshot.ID != job.ID || snapshot.Kind != "task" || snapshot.Status != Done || snapshot.ControlID != "sa_task" {
		t.Fatalf("task run snapshot = %+v, ok=%v", snapshot, ok)
	}
	if got := strings.Join(snapshot.DependsOn, ","); got != "task-1,task-2" {
		t.Fatalf("dependencies = %q", got)
	}
	if _, ok := first.TaskRunForSession("parent-b", job.ID); ok {
		t.Fatal("foreign session lookup exposed task run")
	}
	first.Close()

	second := NewManager(event.Discard)
	defer second.Close()
	second.SetActiveSessionPath("parent-a", sessionPath)
	restored, ok := second.TaskRunForSession("parent-a", job.ID)
	if !ok || restored.Status != Done || strings.Join(restored.DependsOn, ",") != "task-1,task-2" {
		t.Fatalf("restored task run = %+v, ok=%v", restored, ok)
	}

	bash := second.StartForSession("parent-a", "bash", "not task", func(context.Context, io.Writer) (string, error) { return "", nil })
	<-bash.done
	if _, ok := second.TaskRunForSession("parent-a", bash.ID); ok {
		t.Fatal("non-task job was exposed as a persistent task run")
	}
}

func TestRestoreControlledTaskRunsKeepsNumericRunOrder(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	first := NewManager(event.Discard)
	first.SetActiveSessionPath("parent", sessionPath)

	var latest *Job
	for i := 1; i <= 10; i++ {
		dependsOn := []string(nil)
		if latest != nil {
			dependsOn = []string{latest.ID}
		}
		latest = first.StartControlledForSessionWithDependencies("parent", "task", "sa_shared", "turn", dependsOn, func(context.Context, io.Writer) (string, error) {
			return "completed", nil
		})
		result := first.WaitForSession(context.Background(), "parent", []string{latest.ID}, 5)
		if len(result) != 1 || result[0].Status != Done {
			t.Fatalf("run %d result = %+v", i, result)
		}
	}
	if latest == nil || latest.ID != "task-10" {
		t.Fatalf("latest run = %+v, want task-10", latest)
	}
	first.Close()

	second := NewManager(event.Discard)
	defer second.Close()
	second.SetActiveSessionPath("parent", sessionPath)
	controlled, ok := second.WaitControlledForSession(context.Background(), "parent", "task", "sa_shared", 1)
	if !ok || controlled.ID != "task-10" || strings.Join(controlled.DependsOn, ",") != "task-9" {
		t.Fatalf("restored latest controlled run = %+v, ok=%v; want task-10 depending on task-9", controlled, ok)
	}
	snapshot, ok := second.TaskRunForSession("parent", "task-10")
	if !ok || snapshot.ID != controlled.ID || strings.Join(snapshot.DependsOn, ",") != "task-9" {
		t.Fatalf("restored task run snapshot = %+v, ok=%v", snapshot, ok)
	}
}

func TestFinishDestroySessionPurgesOwnedJobs(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	j := m.StartForSession("session", "task", "done", func(context.Context, io.Writer) (string, error) {
		return "answer", nil
	})
	<-j.done

	done := m.DestroySession("session")
	if len(done) != 0 {
		t.Fatalf("finished job should not need destroy wait, got %d handles", len(done))
	}
	m.FinishDestroySession("session")

	if _, _, ok := m.OutputForSession("session", j.ID); ok {
		t.Fatalf("destroyed session job %s should be purged", j.ID)
	}
}

func TestSetActiveSessionPathMigratesRunningJobArtifacts(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	m := NewManager(event.Discard)
	defer m.Close()

	wroteBefore := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	j := m.StartForSession("session", "bash", "migrate", func(_ context.Context, out io.Writer) (string, error) {
		_, _ = io.WriteString(out, "before\n")
		close(wroteBefore)
		<-release
		_, _ = io.WriteString(out, "after\n")
		return "", nil
	})
	<-wroteBefore
	j.mu.Lock()
	oldPath := j.artifactPath
	j.mu.Unlock()

	m.SetActiveSessionPath("session", sessionPath)
	j.mu.Lock()
	gotPath := j.artifactPath
	j.mu.Unlock()
	if gotPath != oldPath {
		t.Fatalf("running artifact path = %q, want unchanged %q before completion", gotPath, oldPath)
	}

	releaseOnce.Do(func() { close(release) })
	<-j.done
	j.mu.Lock()
	donePath := j.artifactPath
	j.mu.Unlock()
	if !strings.HasPrefix(donePath, ArtifactDir(sessionPath)+string(filepath.Separator)) {
		t.Fatalf("completed artifact path = %q, want under %q", donePath, ArtifactDir(sessionPath))
	}
	res := m.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || !strings.Contains(res[0].Output, "before\n") || !strings.Contains(res[0].Output, "after\n") {
		t.Fatalf("wait after migration = %+v, want before and after output", res)
	}
}

func TestArtifactFailureDoesNotFailSuccessfulJob(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(ArtifactDir(sessionPath), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(event.Discard)
	defer m.Close()
	m.SetActiveSessionPath("session", sessionPath)

	j := m.StartForSession("session", "task", "artifact fail", func(context.Context, io.Writer) (string, error) {
		return "successful result", nil
	})
	<-j.done

	res := m.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || res[0].Status != Done {
		t.Fatalf("wait = %+v, want one done result", res)
	}
	if !strings.Contains(res[0].Output, "successful result") || !strings.Contains(res[0].Output, "job artifact incomplete:") {
		t.Fatalf("output = %q, want result and artifact warning", res[0].Output)
	}
}

func TestMigrateArtifactDirFallsBackToCopyWhenRenameFails(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bash-1.log"), []byte("persisted output"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldRename := renamePath
	renamePath = func(_, _ string) error {
		return errors.New("forced rename failure")
	}
	t.Cleanup(func() { renamePath = oldRename })

	if err := migrateArtifactDir(src, dst); err != nil {
		t.Fatalf("migrateArtifactDir: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "bash-1.log"))
	if err != nil {
		t.Fatalf("read migrated artifact: %v", err)
	}
	if string(got) != "persisted output" {
		t.Fatalf("migrated artifact = %q, want persisted output", got)
	}
	if _, err := os.Stat(filepath.Join(src, "bash-1.log")); !os.IsNotExist(err) {
		t.Fatalf("source artifact should be removed after copy fallback, stat err = %v", err)
	}
}

func TestSetActiveSessionPathAdoptsUnscopedTemporaryJobs(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	m := NewManager(event.Discard)
	defer m.Close()

	j := m.StartForSession("", "task", "temporary", func(context.Context, io.Writer) (string, error) {
		return "temporary answer", nil
	})
	<-j.done

	m.SetActiveSessionPath("session", sessionPath)
	res := m.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || !strings.Contains(res[0].Output, "temporary answer") {
		t.Fatalf("adopted wait = %+v, want temporary answer", res)
	}
	if _, _, ok := m.OutputForSession("", j.ID); !ok {
		t.Fatalf("legacy unscoped lookup should still find adopted job %s", j.ID)
	}
	if _, err := os.Stat(filepath.Join(ArtifactDir(sessionPath), j.ID+jobLogExt)); err != nil {
		t.Fatalf("adopted artifact should be under persistent sidecar: %v", err)
	}
}

func TestSetActiveSessionPathAdoptsUnscopedJobsOnMigrationFailure(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(ArtifactDir(sessionPath), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(event.Discard)
	defer m.Close()

	j := m.StartForSession("", "task", "temporary", func(context.Context, io.Writer) (string, error) {
		return "temporary answer", nil
	})
	<-j.done

	m.SetActiveSessionPath("session", sessionPath)

	out, status, ok := m.OutputForSession("session", j.ID)
	if !ok || status != Done {
		t.Fatalf("scoped output ok/status = %v/%s, want true/done", ok, status)
	}
	if !strings.Contains(out, "temporary answer") || !strings.Contains(out, "job artifact incomplete: migration:") {
		t.Fatalf("scoped output = %q, want answer and migration error", out)
	}
	res := m.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || !strings.Contains(res[0].Output, "job artifact incomplete: migration:") {
		t.Fatalf("scoped wait = %+v, want migration error", res)
	}
	if note := m.DrainCompletedNoteForSession("session"); !strings.Contains(note, j.ID) {
		t.Fatalf("adopted completion note = %q, want job id %s", note, j.ID)
	}
}

func TestSetActiveSessionPathReportsMigrationFailure(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(ArtifactDir(sessionPath), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var notices []string
	m := NewManager(event.FuncSink(func(e event.Event) {
		if e.Kind == event.Notice {
			notices = append(notices, e.Text)
		}
	}))
	defer m.Close()

	j := m.StartForSession("session", "task", "migrate fail", func(context.Context, io.Writer) (string, error) {
		return "answer", nil
	})
	m.SetActiveSessionPath("session", sessionPath)
	<-j.done

	res := m.WaitForSession(context.Background(), "session", []string{j.ID}, 1)
	if len(res) != 1 || !strings.Contains(res[0].Output, "job artifact incomplete: migration:") {
		t.Fatalf("wait after migration failure = %+v, want artifact error", res)
	}
	found := false
	for _, notice := range notices {
		if strings.Contains(notice, "job artifact migration failed") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("migration failure notice not emitted, got %q", notices)
	}
}

func TestOutputReadsArtifactFromOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bash-1.log")
	prefix := strings.Repeat("x", 2*defaultTailBytes)
	suffix := "new output\n"
	if err := os.WriteFile(path, []byte(prefix+suffix), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(event.Discard)
	defer m.Close()
	j := &Job{
		ID:           "bash-1",
		Kind:         "bash",
		SessionID:    "session",
		status:       Running,
		readOffset:   int64(len(prefix)),
		artifactPath: path,
		done:         make(chan struct{}),
	}
	m.jobs[jobKey("session", j.ID)] = j
	m.order = append(m.order, jobKey("session", j.ID))

	text, status, ok := m.OutputForSession("session", j.ID)
	if !ok || status != Running {
		t.Fatalf("OutputForSession ok/status = %v/%s, want true/running", ok, status)
	}
	if text != suffix {
		t.Fatalf("OutputForSession text = %q, want %q", text, suffix)
	}
	if j.readOffset != int64(len(prefix)+len(suffix)) {
		t.Fatalf("readOffset = %d, want %d", j.readOffset, len(prefix)+len(suffix))
	}
}

func TestRestoredArtifactsAreScopedBySession(t *testing.T) {
	root := t.TempDir()
	pathA := filepath.Join(root, "a.jsonl")
	pathB := filepath.Join(root, "b.jsonl")

	first := NewManager(event.Discard)
	first.SetActiveSessionPath("session-a", pathA)
	jobA := first.StartForSession("session-a", "bash", "a", func(_ context.Context, out io.Writer) (string, error) {
		_, _ = io.WriteString(out, "from-a")
		return "", nil
	})
	<-jobA.done
	first.Close()

	second := NewManager(event.Discard)
	second.SetActiveSessionPath("session-b", pathB)
	jobB := second.StartForSession("session-b", "bash", "b", func(_ context.Context, out io.Writer) (string, error) {
		_, _ = io.WriteString(out, "from-b")
		return "", nil
	})
	<-jobB.done
	second.Close()

	if jobA.ID != "bash-1" || jobB.ID != "bash-1" {
		t.Fatalf("test setup expected duplicate ids, got %s and %s", jobA.ID, jobB.ID)
	}

	m := NewManager(event.Discard)
	defer m.Close()
	m.SetActiveSessionPath("session-a", pathA)
	m.SetActiveSessionPath("session-b", pathB)

	resA := m.WaitForSession(context.Background(), "session-a", []string{"bash-1"}, 1)
	if len(resA) != 1 || resA[0].Output != "from-a" {
		t.Fatalf("session-a wait = %+v, want from-a", resA)
	}
	resB := m.WaitForSession(context.Background(), "session-b", []string{"bash-1"}, 1)
	if len(resB) != 1 || resB[0].Output != "from-b" {
		t.Fatalf("session-b wait = %+v, want from-b", resB)
	}

	next := m.StartForSession("session-b", "bash", "next", func(context.Context, io.Writer) (string, error) {
		return "", nil
	})
	<-next.done
	if next.ID == "bash-1" {
		t.Fatal("new job should not reuse restored bash-1")
	}
}
