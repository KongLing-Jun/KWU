package jobs

import (
	"context"
	"io"
	"testing"
	"time"

	"kwu/internal/event"
)

type typedNilJobSink struct{}

type turnContextMarkerKey struct{}

func (*typedNilJobSink) Emit(event.Event) {}

func TestNewManagerTreatsTypedNilSinkAsDiscard(t *testing.T) {
	var sink *typedNilJobSink
	m := NewManager(sink)
	defer m.Close()

	j := m.Start("bash", "typed nil sink", func(context.Context, io.Writer) (string, error) {
		return "done", nil
	})
	res := m.Wait(context.Background(), []string{j.ID}, 1000)
	if len(res) != 1 || res[0].Status != Done {
		t.Fatalf("job result = %+v, want one done job", res)
	}
}

func TestRunningKindCountForSessionScopesBySessionAndKind(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()
	release := make(chan struct{})
	start := func(session, kind string) *Job {
		return m.StartForSession(session, kind, "counted", func(context.Context, io.Writer) (string, error) {
			<-release
			return "done", nil
		})
	}
	jobs := []*Job{
		start("parent", "task"),
		start("parent", "task"),
		start("parent", "task"),
		start("parent", "bash"),
		start("other", "task"),
	}
	if got := m.RunningKindCountForSession("parent", "task"); got != 3 {
		t.Fatalf("RunningKindCountForSession(parent, task) = %d, want 3", got)
	}
	if !m.HasRunningKindForSession("parent", "task") {
		t.Fatal("HasRunningKindForSession(parent, task) = false, want true")
	}
	close(release)
	parentIDs := []string{jobs[0].ID, jobs[1].ID, jobs[2].ID, jobs[3].ID}
	_ = m.WaitForSession(context.Background(), "parent", parentIDs, 5)
	_ = m.WaitForSession(context.Background(), "other", []string{jobs[4].ID}, 5)
	if got := m.RunningKindCountForSession("parent", "task"); got != 0 {
		t.Fatalf("RunningKindCountForSession(parent, task) after completion = %d, want 0", got)
	}
}

func TestStartForTurnInheritsParentContextAndCancelsWithIt(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), turnContextMarkerKey{}, "parent-turn"))
	seen := make(chan string, 1)
	job := m.StartForTurn(parent, "session", "task", "turn-scoped", func(ctx context.Context, _ io.Writer) (string, error) {
		seen <- ctx.Value(turnContextMarkerKey{}).(string)
		<-ctx.Done()
		return "", ctx.Err()
	})
	if got := <-seen; got != "parent-turn" {
		t.Fatalf("turn context marker = %q", got)
	}
	cancel()
	result := m.WaitForSession(context.Background(), "session", []string{job.ID}, 5)
	if len(result) != 1 || result[0].Status != Killed {
		t.Fatalf("turn-scoped result = %+v, want killed", result)
	}
}

// --- Wait with timeout ---

func TestWaitTimeout(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	j := m.Start("bash", "", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	// Wait with a very short timeout — the job won't finish in time.
	res := m.Wait(context.Background(), []string{j.ID}, 1)
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	// Should still be running (timeout expired before completion).
	if res[0].Status != Running {
		t.Errorf("status = %q, want running", res[0].Status)
	}
	m.Kill(j.ID)
}

// --- Wait with empty ids waits for all running ---

func TestWaitAllRunning(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	// Jobs block until cancelled so they are still running when Wait resolves the
	// "all running" set — instant-returning jobs could finish first and be missed,
	// which is exactly the resolution this test must observe deterministically. A
	// short timeout returns the still-running snapshot.
	j1 := m.Start("bash", "", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	j2 := m.Start("bash", "", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	res := m.Wait(context.Background(), nil, 1)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
	ids := map[string]bool{res[0].ID: true, res[1].ID: true}
	if !ids[j1.ID] || !ids[j2.ID] {
		t.Errorf("results missing expected ids: %v", ids)
	}
	m.Kill(j1.ID)
	m.Kill(j2.ID)
}

// --- Output with unknown id ---

func TestOutputUnknownID(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	_, _, ok := m.Output("nonexistent-id")
	if ok {
		t.Error("Output for unknown id should return ok=false")
	}
}

// --- Kill with unknown id ---

func TestKillUnknownID(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	if m.Kill("nonexistent-id") {
		t.Error("Kill for unknown id should return false")
	}
}

// --- startedText ---

func TestStartedTextWithLabel(t *testing.T) {
	got := startedText("bash", "bash-1", "my-label")
	if got != "background bash started: bash-1 (my-label)" {
		t.Errorf("startedText = %q", got)
	}
}

func TestStartedTextWithoutLabel(t *testing.T) {
	got := startedText("task", "task-1", "")
	if got != "background task started: task-1" {
		t.Errorf("startedText = %q", got)
	}
}

// --- DrainCompletedNote with multiple jobs ---

func TestDrainMultiple(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	m.Start("bash", "", func(_ context.Context, _ io.Writer) (string, error) {
		return "", nil
	})
	m.Start("task", "label", func(_ context.Context, _ io.Writer) (string, error) {
		return "answer", nil
	})
	m.Wait(context.Background(), nil, 5)
	note := m.DrainCompletedNote()
	if note == "" {
		t.Fatal("drain should not be empty after 2 completions")
	}
}

// --- Close is idempotent ---

func TestCloseIdempotent(t *testing.T) {
	m := NewManager(event.Discard)
	m.Start("bash", "", func(_ context.Context, _ io.Writer) (string, error) {
		return "", nil
	})
	m.Close()
	m.Close() // should not panic
}

// --- Running with no jobs ---

func TestRunningEmpty(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()
	if r := m.Running(); len(r) != 0 {
		t.Errorf("Running() = %d, want 0", len(r))
	}
}

// --- Job with error sets Failed ---

func TestJobFailed(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	j := m.Start("bash", "", func(_ context.Context, _ io.Writer) (string, error) {
		return "", io.ErrUnexpectedEOF
	})
	res := m.Wait(context.Background(), []string{j.ID}, 5)
	if len(res) != 1 || res[0].Status != Failed {
		t.Fatalf("want Failed, got %+v", res)
	}
}

// --- Job with result and no error sets Done ---

func TestJobWithResult(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	j := m.Start("task", "", func(_ context.Context, _ io.Writer) (string, error) {
		return "final answer", nil
	})
	res := m.Wait(context.Background(), []string{j.ID}, 5)
	if len(res) != 1 || res[0].Status != Done {
		t.Fatalf("want Done, got %+v", res)
	}
	if res[0].Output != "final answer" {
		t.Errorf("output = %q, want \"final answer\"", res[0].Output)
	}
}

func TestControlledJobWaitAndKillUseStableIdentity(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	started := make(chan struct{})
	m.StartControlledForSession("parent", "task", "sa_one", "first turn", func(ctx context.Context, _ io.Writer) (string, error) {
		close(started)
		<-ctx.Done()
		return "partial answer", ctx.Err()
	})
	<-started

	if _, ok := m.WaitControlledForSession(context.Background(), "other", "task", "sa_one", 0); ok {
		t.Fatal("control identity must remain scoped to its parent session")
	}
	if !m.KillControlledForSession("parent", "task", "sa_one") {
		t.Fatal("KillControlledForSession should cancel the matching running job")
	}
	result, ok := m.WaitControlledForSession(context.Background(), "parent", "task", "sa_one", 5)
	if !ok {
		t.Fatal("WaitControlledForSession did not find the controlled job")
	}
	if result.Status != Killed || result.Output != "partial answer" {
		t.Fatalf("controlled result = %+v, want killed partial answer", result)
	}
}

func TestControlledJobSelectsLatestTurn(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	first := m.StartControlledForSession("parent", "task", "sa_one", "first", func(context.Context, io.Writer) (string, error) {
		return "first answer", nil
	})
	if got := m.WaitForSession(context.Background(), "parent", []string{first.ID}, 5); len(got) != 1 || got[0].Status != Done {
		t.Fatalf("first turn = %+v", got)
	}
	m.StartControlledForSession("parent", "task", "sa_one", "second", func(context.Context, io.Writer) (string, error) {
		return "second answer", nil
	})

	result, ok := m.WaitControlledForSession(context.Background(), "parent", "task", "sa_one", 5)
	if !ok || result.Output != "second answer" {
		t.Fatalf("latest controlled result = %+v, ok=%v", result, ok)
	}
}

// --- Context injection ---

func TestWithManagerFromContext(t *testing.T) {
	m := NewManager(event.Discard)
	defer m.Close()

	ctx := WithManager(context.Background(), m)
	got, ok := FromContext(ctx)
	if !ok || got != m {
		t.Error("FromContext should return the manager")
	}
}

func TestFromContextEmpty(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("plain context should return ok=false")
	}
}

// --- Status constants ---

func TestStatusConstants(t *testing.T) {
	if Running != "running" {
		t.Errorf("Running = %q", Running)
	}
	if Done != "done" {
		t.Errorf("Done = %q", Done)
	}
	if Failed != "failed" {
		t.Errorf("Failed = %q", Failed)
	}
	if Killed != "killed" {
		t.Errorf("Killed = %q", Killed)
	}
}

// --- nowMs ---

func TestNowMs(t *testing.T) {
	before := time.Now().UnixMilli()
	got := nowMs()
	after := time.Now().UnixMilli()
	if got < before || got > after {
		t.Errorf("nowMs() = %d, not in [%d, %d]", got, before, after)
	}
}
