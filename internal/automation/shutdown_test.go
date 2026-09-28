package automation

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

type shutdownRunnerFunc func(context.Context, RunRequest) (RunResult, error)

func (f shutdownRunnerFunc) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	return f(ctx, req)
}

func TestShutdownDrainsManualAndScheduledArchives(t *testing.T) {
	for _, scheduled := range []bool{false, true} {
		name := "manual"
		if scheduled {
			name = "scheduled"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			task := Task{ID: "drain", Name: "Drain", Enabled: true, Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: now.Add(-time.Minute)}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
			store := newEngineTestStore(t, task)
			started, archiving, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			runner := shutdownRunnerFunc(func(ctx context.Context, _ RunRequest) (RunResult, error) {
				close(started)
				<-ctx.Done()
				return RunResult{}, ctx.Err()
			})
			engine := newEngineTestEngine(store, "test:drain", runner)
			engine.SetChangeObserver(func(change EngineChange) {
				if change.Kind == "run" && change.Status == string(RunStatusCanceled) {
					close(archiving)
					<-release
				}
			})
			svc := newService(store, engine)
			triggered := make(chan error, 1)
			if scheduled {
				engine.Start(context.Background())
			} else {
				go func() {
					_, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: task.ID})
					triggered <- err
				}()
			}
			waitShutdownSignal(t, started)
			shutdown := make(chan error, 2)
			for range 2 {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					shutdown <- svc.Shutdown(ctx)
				}()
			}
			waitShutdownSignal(t, archiving)
			select {
			case err := <-shutdown:
				t.Fatalf("shutdown before archive/lease cleanup: %v", err)
			default:
			}
			if response, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: task.ID}); !errors.Is(err, ErrShuttingDown) || response.RunID != "" {
				t.Fatalf("new admission = %+v, %v", response, err)
			}
			close(release)
			for range 2 {
				if err := <-shutdown; err != nil {
					t.Fatal(err)
				}
			}
			if !scheduled {
				if err := <-triggered; err != nil {
					t.Fatal(err)
				}
			}
			runs, err := store.ListRuns(task.ID, 0)
			if err != nil || len(runs) != 1 || runs[0].Status != RunStatusCanceled || runs[0].FinishedAt.IsZero() {
				t.Fatalf("runs=%+v, err=%v", runs, err)
			}
			path, _ := store.leasePath(task.ID)
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("lease remains: %v", err)
			}
		})
	}
}

func TestShutdownTimeoutRetainsAdmissionUntilCleanup(t *testing.T) {
	store := newEngineTestStore(t)
	engine := newEngineTestEngine(store, "test:timeout", &mockRunner{})
	svc := newService(store, engine)
	admitted, finish, err := svc.running.admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = svc.Shutdown(ctx)
	var shutdownErr *ShutdownError
	if !errors.As(err, &shutdownErr) || shutdownErr.Drained || !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown = %v", err)
	}
	if admitted.Err() != context.Canceled {
		t.Fatal("in-flight operation was not canceled")
	}
	if _, _, err := svc.running.admit(context.Background()); !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("admit = %v", err)
	}
	finish()
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownWaitsForAdmittedPreflight(t *testing.T) {
	task := Task{ID: "preflight", Name: "Preflight", Enabled: true, Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: time.Now().Add(time.Hour)}}
	store := newEngineTestStore(t, task)
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "test:preflight", runner)
	entered, release := make(chan struct{}), make(chan struct{})
	engine.checker.Preflight = func(Task) PreflightResult {
		close(entered)
		<-release
		return PreflightResult{Disposition: PreflightReady}
	}
	svc := newService(store, engine)
	triggered := make(chan error, 1)
	go func() {
		_, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: task.ID})
		triggered <- err
	}()
	waitShutdownSignal(t, entered)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := svc.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown = %v", err)
	}
	close(release)
	if err := <-triggered; !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("TriggerNow = %v", err)
	}
	if err := svc.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.count() != 0 {
		t.Fatal("runner started after draining")
	}
}

func waitShutdownSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle barrier timed out")
	}
}
