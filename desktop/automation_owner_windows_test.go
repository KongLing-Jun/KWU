//go:build windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/automation"
)

type recoveryProcessRunner func(context.Context, automation.RunRequest) (automation.RunResult, error)

func (f recoveryProcessRunner) Run(ctx context.Context, req automation.RunRequest) (automation.RunResult, error) {
	return f(ctx, req)
}

func TestDesktopAutomationOwnerProbe(t *testing.T) {
	if got := probeDesktopAutomationOwner(automationDesktopOwner()); got != automation.OwnerAlive {
		t.Fatalf("current process=%v", got)
	}
	for _, owner := range []string{"", "serve:1", "desktop:0", "desktop:-1", "desktop:4294967296", "desktop:unknown", "remote:desktop:1"} {
		if got := probeDesktopAutomationOwner(owner); got != automation.OwnerUnknown {
			t.Fatalf("owner %q=%v", owner, got)
		}
	}
}

func TestDesktopAutomationRecoversActualExitedProcess(t *testing.T) {
	root := t.TempDir()
	store := automation.NewLocalPersistentStore(root, probeDesktopAutomationOwner)
	defer store.Close()
	task := automation.Task{ID: "crash", Name: "Crash fixture", Enabled: true, Schedule: automation.Schedule{Kind: automation.ScheduleKindOnce, RunAt: time.Now().Add(time.Hour)}}
	if err := store.SaveTasks([]automation.Task{task}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDesktopAutomationCrashHelper$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "KWU_AUTOMATION_CRASH_FIXTURE="+root)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == "AUTOMATION_READY" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			_ = cmd.Wait()
			t.Fatalf("child exited before ready: %s", stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child not ready")
	}
	owner := fmt.Sprintf("desktop:%d", cmd.Process.Pid)
	if got := probeDesktopAutomationOwner(owner); got != automation.OwnerAlive {
		t.Fatalf("live child=%v", got)
	}
	if changes, err := store.RecoverInterruptedRuns(context.Background(), ""); err != nil || len(changes) != 0 {
		t.Fatalf("live recovery=%+v,%v", changes, err)
	}
	before, err := store.ReadRunManifest(task.ID, "crashed-run")
	if err != nil || before.Status != automation.RunStatusRunning {
		t.Fatalf("before=%+v,%v", before, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if got := probeDesktopAutomationOwner(owner); got != automation.OwnerDead {
		t.Fatalf("exited child=%v", got)
	}
	changes, err := store.RecoverInterruptedRuns(context.Background(), "")
	if err != nil || len(changes) != 1 {
		t.Fatalf("recovery=%+v,%v", changes, err)
	}
	old, err := store.ReadRunManifest(task.ID, "crashed-run")
	if err != nil || old.Status != automation.RunStatusFailed || old.FinishedAt.IsZero() {
		t.Fatalf("old=%+v,%v", old, err)
	}
	calls := 0
	engine := automation.NewEngineWithChecker(store, automationDesktopOwner(), recoveryProcessRunner(func(context.Context, automation.RunRequest) (automation.RunResult, error) {
		calls++
		return automation.RunResult{}, nil
	}), automation.Checker{ModelResolver: func(automation.Task) error { return nil }})
	if id, err := engine.Trigger(context.Background(), automation.TriggerRequest{TaskID: task.ID, RunID: "new-run"}); err != nil || id != "new-run" || calls != 1 {
		t.Fatalf("new run=%q,%v,calls=%d", id, err, calls)
	}
	if data, err := os.ReadFile(filepath.Join(root, "preserved.txt")); err != nil || string(data) != "partial output" {
		t.Fatalf("partial output=%q,%v", data, err)
	}
	if _, err := store.RecoverInterruptedRuns(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	again, _ := store.ReadRunManifest(task.ID, "crashed-run")
	if again.Status != automation.RunStatusFailed || !again.FinishedAt.Equal(old.FinishedAt) {
		t.Fatal("old run was revived or re-finalized")
	}
}

func TestDesktopAutomationCrashHelper(t *testing.T) {
	root := os.Getenv("KWU_AUTOMATION_CRASH_FIXTURE")
	if root == "" {
		t.Skip("subprocess helper")
	}
	store := automation.NewLocalPersistentStore(root, probeDesktopAutomationOwner)
	runner := recoveryProcessRunner(func(ctx context.Context, _ automation.RunRequest) (automation.RunResult, error) {
		if err := os.WriteFile(filepath.Join(root, "preserved.txt"), []byte("partial output"), 0600); err != nil {
			return automation.RunResult{}, err
		}
		fmt.Fprintln(os.Stdout, "AUTOMATION_READY")
		<-ctx.Done()
		return automation.RunResult{}, ctx.Err()
	})
	engine := automation.NewEngineWithChecker(store, automationDesktopOwner(), runner, automation.Checker{ModelResolver: func(automation.Task) error { return nil }})
	if _, err := engine.Trigger(context.Background(), automation.TriggerRequest{TaskID: "crash", RunID: "crashed-run"}); err != nil {
		t.Fatal(err)
	}
}
