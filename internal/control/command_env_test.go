package control

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/tool"
)

type controlCommandEnvProvider struct {
	path  string
	calls atomic.Int32
}

func (p *controlCommandEnvProvider) ResolveCommandEnv(context.Context) (map[string]string, error) {
	p.calls.Add(1)
	return map[string]string{
		tool.CommandEnvPATH: p.path,
		"KWU_PYTHON":        filepath.Join(p.path, "python.exe"),
	}, nil
}

type controlCommandEnvRunner struct {
	results chan map[string]string
}

func (r *controlCommandEnvRunner) Run(ctx context.Context, _ string) error {
	env, err := tool.ResolveCommandEnv(ctx)
	if err != nil {
		return err
	}
	r.results <- env
	return nil
}

func TestControllerBindsCommandEnvProviderForOrdinaryAndCapabilityTurns(t *testing.T) {
	root := t.TempDir()
	pythonDir := filepath.Join(root, "python")
	connectorDir := filepath.Join(root, "connector")
	provider := &controlCommandEnvProvider{path: pythonDir}
	runner := &controlCommandEnvRunner{results: make(chan map[string]string, 4)}
	done := make(chan event.Event, 1)
	controller := New(Options{
		Runner:             runner,
		CommandEnvProvider: provider,
		Sink: event.FuncSink(func(evt event.Event) {
			if evt.Kind == event.TurnDone {
				done <- evt
			}
		}),
	})

	if err := controller.Run(context.Background(), "headless"); err != nil {
		t.Fatal(err)
	}
	assertControlCommandEnv(t, <-runner.results, pythonDir, "")

	if err := controller.RunTurnDisplay(context.Background(), "", "foreground"); err != nil {
		t.Fatal(err)
	}
	assertControlCommandEnv(t, <-runner.results, pythonDir, "")
	assertControlTurnDone(t, done)

	turn := CapabilityTurn{Input: "capability", CommandEnv: map[string]string{
		tool.CommandEnvPATH: connectorDir,
		"CONNECTOR_READY":   "1",
	}}
	if err := controller.RunCapabilityTurn(context.Background(), turn); err != nil {
		t.Fatal(err)
	}
	assertControlCommandEnv(t, <-runner.results, pythonDir, connectorDir)
	assertControlTurnDone(t, done)

	controller.SubmitCapabilities(turn)
	select {
	case evt := <-done:
		if evt.Err != nil {
			t.Fatal(evt.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for capability turn")
	}
	assertControlCommandEnv(t, <-runner.results, pythonDir, connectorDir)

	if got := provider.calls.Load(); got != 4 {
		t.Fatalf("provider calls = %d, want one snapshot per top-level task", got)
	}
}

func TestControllerRebindsCommandEnvSnapshotForNewTaskOnCompletedContext(t *testing.T) {
	actor := testActor()
	firstTask := testTaskContext(t, "task_command_env_first", actor)
	secondTask := testTaskContext(t, "task_command_env_second", actor)
	provider := &controlCommandEnvProvider{path: filepath.Join(t.TempDir(), "python")}
	initializer := &recordingTaskContextInitializer{tasks: []modelbilling.TaskContext{secondTask}}
	controller := New(Options{
		Actor:                  actor,
		CommandEnvProvider:     provider,
		TaskContextInitializer: initializer,
	})

	ctx, err := modelbilling.WithTaskContext(context.Background(), firstTask)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = controller.bindTaskContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.ResolveCommandEnv(ctx); err != nil {
		t.Fatal(err)
	}

	ctx, err = controller.initializeTaskContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = controller.bindTaskContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.ResolveCommandEnv(ctx); err != nil {
		t.Fatal(err)
	}
	if got := provider.calls.Load(); got != 2 {
		t.Fatalf("provider calls after new task = %d, want fresh snapshot", got)
	}

	derived, err := controller.bindTaskContext(context.WithValue(ctx, struct{}{}, "derived"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.ResolveCommandEnv(derived); err != nil {
		t.Fatal(err)
	}
	if got := provider.calls.Load(); got != 2 {
		t.Fatalf("same-task derived context calls = %d, want snapshot reuse", got)
	}
}

func assertControlTurnDone(t *testing.T, done <-chan event.Event) {
	t.Helper()
	select {
	case evt := <-done:
		if evt.Err != nil {
			t.Fatal(evt.Err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for turn completion")
	}
}

func assertControlCommandEnv(t *testing.T, env map[string]string, pythonDir, connectorDir string) {
	t.Helper()
	parts := filepath.SplitList(env[tool.CommandEnvPATH])
	if len(parts) == 0 || parts[0] != pythonDir {
		t.Fatalf("PATH = %q, want Python first", env[tool.CommandEnvPATH])
	}
	if connectorDir == "" {
		if len(parts) != 1 {
			t.Fatalf("ordinary PATH = %q, want only Python provider", env[tool.CommandEnvPATH])
		}
		return
	}
	if len(parts) != 2 || parts[1] != connectorDir || env["CONNECTOR_READY"] != "1" {
		t.Fatalf("capability environment = %#v, want merged connector snapshot", env)
	}
}
