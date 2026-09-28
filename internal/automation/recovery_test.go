package automation

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestRecoverInterruptedRunAndLease(t *testing.T) {
	for _, state := range []OwnerState{OwnerDead, OwnerAlive, OwnerUnknown} {
		for _, expired := range []bool{false, true} {
			t.Run(string(rune('0'+state))+map[bool]string{false: "/fresh", true: "/expired"}[expired], func(t *testing.T) {
				store, task, run, paths := recoveryFixture(t)
				store.ownerProbe = func(string) OwnerState { return state }
				if expired {
					path, _ := store.leasePath(task.ID)
					lease, _ := readLease(path)
					lease.ExpiresAt = time.Now().Add(-time.Hour)
					if err := writeLease(path, lease); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(paths.Result, []byte("preserved output"), 0600); err != nil {
					t.Fatal(err)
				}
				changes, err := store.RecoverInterruptedRuns(context.Background(), "")
				if err != nil {
					t.Fatal(err)
				}
				got, err := store.ReadRunManifest(task.ID, run.RunID)
				if err != nil {
					t.Fatal(err)
				}
				if state == OwnerDead {
					if len(changes) != 1 || got.Status != RunStatusFailed || got.FinishedAt.IsZero() {
						t.Fatalf("recovered=%+v, run=%+v", changes, got)
					}
					if _, ok, err := store.AcquireLease(task.ID, "new", "new-owner", time.Hour); err != nil || !ok {
						t.Fatalf("new lease=%t, %v", ok, err)
					}
					firstFinished := got.FinishedAt
					// The next owner is alive. Repeating recovery preserves both terminal
					// metadata and the newer lease, including after index reconstruction.
					store.ownerProbe = func(owner string) OwnerState {
						if owner == "old-owner" {
							return OwnerDead
						}
						return OwnerAlive
					}
					if err := store.markRunsIndexDirtyLocked(); err != nil {
						t.Fatal(err)
					}
					if changes, err := store.RecoverInterruptedRuns(context.Background(), task.ID); err != nil || len(changes) != 0 {
						t.Fatalf("repeat=%+v, %v", changes, err)
					}
					got, _ = store.ReadRunManifest(task.ID, run.RunID)
					if !got.FinishedAt.Equal(firstFinished) {
						t.Fatal("terminal time changed")
					}
					page, err := store.ListRunsPage(RunFilter{TaskID: task.ID}, "")
					if err != nil || len(page.Items) != 1 || page.Items[0].Status != RunStatusFailed {
						t.Fatalf("index=%+v, %v", page, err)
					}
				} else {
					if len(changes) != 0 || got.Status != RunStatusRunning {
						t.Fatalf("live/unknown run changed: %+v", got)
					}
					if _, ok, err := store.AcquireLease(task.ID, "new", "new-owner", time.Hour); err != nil || ok {
						t.Fatalf("active/unknown lease takeover=%t, %v", ok, err)
					}
				}
				bytes, err := os.ReadFile(paths.Result)
				if err != nil || string(bytes) != "preserved output" {
					t.Fatalf("result changed: %q, %v", bytes, err)
				}
				current, _ := store.Task(task.ID)
				if !current.LastRunAt.IsZero() {
					t.Fatal("recovery changed task scheduling history")
				}
			})
		}
	}
}

func TestRecoveryPreservesNewLeaseAndDoesNotExecuteOldRun(t *testing.T) {
	store, task, old, _ := recoveryFixture(t)
	store.ownerProbe = func(owner string) OwnerState {
		if owner == "old-owner" {
			return OwnerDead
		}
		return OwnerAlive
	}
	path, _ := store.leasePath(task.ID)
	newer := Lease{TaskID: task.ID, RunID: "newer-run", Owner: "live-owner", ExpiresAt: time.Now().Add(time.Hour)}
	if err := writeLease(path, newer); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecoverInterruptedRuns(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	lease, err := readLease(path)
	if err != nil || lease.RunID != newer.RunID || lease.Owner != newer.Owner {
		t.Fatalf("new lease changed: %+v, %v", lease, err)
	}
	manifest, _ := store.ReadRunManifest(task.ID, old.RunID)
	if manifest.Status != RunStatusFailed {
		t.Fatalf("old status=%s", manifest.Status)
	}
}

func TestRecoveryClearsOrphanLeaseWithoutManifestAndTerminalLease(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		store, task, old, paths := recoveryFixture(t)
		store.ownerProbe = func(string) OwnerState { return OwnerDead }
		if terminal {
			if err := old.Transition(RunStatusSucceeded, "done", time.Now()); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveRunManifest(old); err != nil {
				t.Fatal(err)
			}
		} else if err := os.RemoveAll(paths.Dir); err != nil {
			t.Fatal(err)
		}
		if changes, err := store.RecoverInterruptedRuns(context.Background(), ""); err != nil || len(changes) != 0 {
			t.Fatalf("changes=%+v, %v", changes, err)
		}
		path, _ := store.leasePath(task.ID)
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("orphan lease remains: %v", err)
		}
		if terminal {
			got, _ := store.ReadRunManifest(task.ID, old.RunID)
			if got.Status != RunStatusSucceeded || !got.FinishedAt.Equal(old.FinishedAt) {
				t.Fatal("terminal overwritten")
			}
		}
	}
}

func TestRecoveryPrecedesExplicitNewTrigger(t *testing.T) {
	store, task, old, _ := recoveryFixture(t)
	store.ownerProbe = func(owner string) OwnerState {
		if owner == "old-owner" {
			return OwnerDead
		}
		return OwnerAlive
	}
	runner := &mockRunner{}
	engine := newEngineTestEngine(store, "current-owner", runner)
	svc := newService(store, engine)
	response, err := svc.TriggerNow(context.Background(), TriggerNowRequest{TaskID: task.ID})
	if err != nil || response.RunID == "" || response.RunID == old.RunID || response.Status != RunStatusSucceeded || runner.count() != 1 {
		t.Fatalf("trigger=%+v, %v, calls=%d", response, err, runner.count())
	}
	got, _ := store.ReadRunManifest(task.ID, old.RunID)
	if got.Status != RunStatusFailed {
		t.Fatalf("old run=%+v", got)
	}
	deliveries, err := store.ListDeliveries(task.ID, old.RunID)
	if err != nil || len(deliveries) != 0 {
		t.Fatalf("old deliveries=%+v,%v", deliveries, err)
	}
}

func recoveryFixture(t *testing.T) (*Store, Task, RunManifest, RunPaths) {
	t.Helper()
	task := Task{ID: "recovery", Name: "Recovery", Enabled: true, Schedule: Schedule{Kind: ScheduleKindOnce, RunAt: time.Now().Add(time.Hour)}}
	store := newEngineTestStore(t, task)
	run, paths, err := store.CreateRun(task, string(TriggerManual), "old-owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Transition(RunStatusRunning, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRunManifest(run); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.AcquireLease(task.ID, run.RunID, "old-owner", time.Hour); err != nil || !ok {
		t.Fatalf("lease=%t,%v", ok, err)
	}
	return store, task, run, paths
}
