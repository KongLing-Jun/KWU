package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"kwu/internal/event"
	"kwu/internal/jobs"
	"kwu/internal/sandbox"
	"kwu/internal/tool"
)

const (
	backgroundConnectorConfig = "CONNECTOR_CONFIG_HOME"
	backgroundConnectorUpdate = "CONNECTOR_NO_UPDATE"
	backgroundConnectorSkills = "CONNECTOR_NO_SKILLS_NOTICE"
)

// End-to-end through the actual tools: a background bash job runs under a manager
// injected on the context, the wait tool collects its output, and bash_output
// reads it — the same path the agent drives.
func TestBackgroundBashWaitAndOutput(t *testing.T) {
	m := jobs.NewManager(event.Discard)
	defer m.Close()
	ctx := jobs.WithManager(context.Background(), m)

	start, err := bash{}.Execute(ctx, []byte(`{"command":"printf hello; sleep 0.3","run_in_background":true}`))
	if err != nil {
		t.Fatalf("bash background: %v", err)
	}
	if !strings.Contains(start, "Started background job") {
		t.Fatalf("unexpected start message: %q", start)
	}

	// The job is registered and running synchronously before Execute returns.
	running := m.Running()
	if len(running) != 1 {
		t.Fatalf("want 1 running job, got %d", len(running))
	}
	id := running[0].ID

	// wait blocks until it finishes, then returns its output.
	wout, err := waitJob{}.Execute(ctx, []byte(`{"job_ids":["`+id+`"]}`))
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if !strings.Contains(wout, "done") || !strings.Contains(wout, "hello") {
		t.Errorf("wait output = %q, want it to report done with hello", wout)
	}

	// bash_output reads the buffered output (wait doesn't consume the read cursor).
	bo, err := bashOutput{}.Execute(ctx, []byte(`{"job_id":"`+id+`"}`))
	if err != nil {
		t.Fatalf("bash_output: %v", err)
	}
	if !strings.Contains(bo, "hello") {
		t.Errorf("bash_output = %q, want hello", bo)
	}
}

// kill_shell terminates a long-running background job.
func TestBackgroundKill(t *testing.T) {
	m := jobs.NewManager(event.Discard)
	defer m.Close()
	ctx := jobs.WithManager(context.Background(), m)

	if _, err := (bash{}).Execute(ctx, []byte(`{"command":"sleep 120","run_in_background":true}`)); err != nil {
		t.Fatalf("bash background: %v", err)
	}
	id := m.Running()[0].ID

	kout, err := killShell{}.Execute(ctx, []byte(`{"job_id":"`+id+`"}`))
	if err != nil {
		t.Fatalf("kill_shell: %v", err)
	}
	if !strings.Contains(kout, "Killed") {
		t.Errorf("kill_shell = %q, want it to report Killed", kout)
	}
	// 120s natural duration keeps the job far from finishing on its own, so the
	// reap window is the only thing this measures: a loaded machine's slow
	// process-tree teardown (up to ~bashWaitDelay) still fits, while a genuinely
	// broken kill trips the 40s timeout. Pairing the sleep with the timeout (as
	// 10/10 did) raced natural completion against the reap.
	res := m.Wait(ctx, []string{id}, 40)
	if len(res) != 1 || res[0].Status != jobs.Killed {
		t.Fatalf("want killed, got %+v", res)
	}
}

// Without a manager on the context the background tools degrade to a clear error
// rather than panicking.
func TestBackgroundToolsNoManager(t *testing.T) {
	ctx := context.Background()
	if _, err := (bashOutput{}).Execute(ctx, []byte(`{"job_id":"bash-1"}`)); err == nil {
		t.Error("bash_output without a manager should error")
	}
	if _, err := (bash{}).Execute(ctx, []byte(`{"command":"echo hi","run_in_background":true}`)); err == nil {
		t.Error("background bash without a manager should error")
	}
}

func TestBackgroundBashKeepsLaunchCommandEnvironmentSnapshot(t *testing.T) {
	root := t.TempDir()
	nodeA, cliA := filepath.Join(root, "node-a"), filepath.Join(root, "cli-a")
	configA := filepath.Join(root, "config-a")
	inputA := map[string]string{
		tool.CommandEnvPATH:       nodeA + string(filepath.ListSeparator) + cliA,
		backgroundConnectorConfig: configA, backgroundConnectorUpdate: "1", backgroundConnectorSkills: "1",
	}
	validatedA, err := tool.ValidateCommandEnv(inputA)
	if err != nil {
		t.Fatal(err)
	}
	m := jobs.NewManager(event.Discard)
	defer m.Close()
	ctxA := tool.WithCommandEnv(jobs.WithManager(context.Background(), m), validatedA)
	gate := filepath.Join(root, "release")
	command := backgroundEnvSnapshotCommand((bash{}).resolved().Kind, gate)
	args, err := json.Marshal(map[string]any{"command": command, "run_in_background": true})
	if err != nil {
		t.Fatal(err)
	}
	processBefore := normalizedProcessEnvironment()
	if _, err := (bash{}).Execute(ctxA, args); err != nil {
		t.Fatalf("start background bash: %v", err)
	}
	if !reflect.DeepEqual(processBefore, normalizedProcessEnvironment()) {
		t.Fatal("starting background bash changed the process environment")
	}
	running := m.Running()
	if len(running) != 1 {
		t.Fatalf("want 1 running job, got %d", len(running))
	}

	nodeB, cliB := filepath.Join(root, "node-b"), filepath.Join(root, "cli-b")
	configB := filepath.Join(root, "config-b")
	inputA[tool.CommandEnvPATH] = nodeB + string(filepath.ListSeparator) + cliB
	inputA[backgroundConnectorConfig] = configB
	inputA[backgroundConnectorUpdate] = "changed"
	inputA[backgroundConnectorSkills] = "changed"
	contextCopy := tool.CommandEnvFromContext(ctxA)
	contextCopy[tool.CommandEnvPATH] = inputA[tool.CommandEnvPATH]
	contextCopy[backgroundConnectorConfig] = configB
	validatedB, err := tool.ValidateCommandEnv(map[string]string{
		tool.CommandEnvPATH: inputA[tool.CommandEnvPATH], backgroundConnectorConfig: configB,
		backgroundConnectorUpdate: "1", backgroundConnectorSkills: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctxB := tool.WithCommandEnv(context.Background(), validatedB)
	envB, err := bashCommandEnv(ctxB, "git status")
	if err != nil {
		t.Fatal(err)
	}
	pathB, _ := envValue(envB, "PATH")
	if parts := filepath.SplitList(pathB); len(parts) < 2 || parts[0] != nodeB || parts[1] != cliB {
		t.Fatalf("later context did not contain environment B: %q", pathB)
	}

	restoreProcess := replaceProcessEnvironment(t, map[string]string{
		"PATH": inputA[tool.CommandEnvPATH], backgroundConnectorConfig: configB,
		backgroundConnectorUpdate: "process-b", backgroundConnectorSkills: "process-b",
	})
	defer restoreProcess()
	if err := os.WriteFile(gate, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	results := m.Wait(ctxA, []string{running[0].ID}, 10)
	if len(results) != 1 || results[0].Status != jobs.Done {
		t.Fatalf("background result = %+v", results)
	}
	values := parseBackgroundEnvSnapshot(results[0].Output)
	pathSnapshot := strings.ToLower(strings.ReplaceAll(values["SNAP_PATH"], "\\", "/"))
	nodeIndex := strings.Index(pathSnapshot, "/node-a")
	cliIndex := strings.Index(pathSnapshot, "/cli-a")
	if nodeIndex < 0 || cliIndex <= nodeIndex || strings.Contains(pathSnapshot, "/node-b") || strings.Contains(pathSnapshot, "/cli-b") {
		t.Fatalf("background PATH did not preserve managed A ordering: %q", values["SNAP_PATH"])
	}
	for key, want := range map[string]string{
		"SNAP_CONFIG": configA,
		"SNAP_UPDATE": "1",
		"SNAP_SKILLS": "1",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
	restoreProcess()
	if !reflect.DeepEqual(processBefore, normalizedProcessEnvironment()) {
		t.Fatal("background bash changed the process environment")
	}
}

func backgroundEnvSnapshotCommand(kind sandbox.ShellKind, gate string) string {
	if kind == sandbox.ShellPowerShell {
		gate = strings.ReplaceAll(gate, "'", "''")
		return "while (-not (Test-Path -LiteralPath '" + gate + "')) { Start-Sleep -Milliseconds 20 }; " +
			"[Console]::WriteLine('SNAP_PATH=' + $env:PATH); " +
			"[Console]::WriteLine('SNAP_CONFIG=' + $env:CONNECTOR_CONFIG_HOME); " +
			"[Console]::WriteLine('SNAP_UPDATE=' + $env:CONNECTOR_NO_UPDATE); " +
			"[Console]::WriteLine('SNAP_SKILLS=' + $env:CONNECTOR_NO_SKILLS_NOTICE)"
	}
	gate = filepath.ToSlash(gate)
	gate = "'" + strings.ReplaceAll(gate, "'", "'\"'\"'") + "'"
	return "while [ ! -f " + gate + " ]; do sleep 0.02; done; " +
		"printf 'SNAP_PATH=%s\\nSNAP_CONFIG=%s\\nSNAP_UPDATE=%s\\nSNAP_SKILLS=%s\\n' " +
		"\"$PATH\" \"$CONNECTOR_CONFIG_HOME\" \"$CONNECTOR_NO_UPDATE\" \"$CONNECTOR_NO_SKILLS_NOTICE\""
}

func parseBackgroundEnvSnapshot(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func normalizedProcessEnvironment() map[string]string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		values[key] = value
	}
	return values
}

func replaceProcessEnvironment(t *testing.T, replacements map[string]string) func() {
	t.Helper()
	type savedValue struct {
		value string
		found bool
	}
	saved := make(map[string]savedValue, len(replacements))
	for key, value := range replacements {
		previous, found := os.LookupEnv(key)
		saved[key] = savedValue{value: previous, found: found}
		if err := os.Setenv(key, value); err != nil {
			t.Fatal(err)
		}
	}
	restored := false
	return func() {
		if restored {
			return
		}
		restored = true
		for key, previous := range saved {
			var err error
			if previous.found {
				err = os.Setenv(key, previous.value)
			} else {
				err = os.Unsetenv(key)
			}
			if err != nil {
				t.Errorf("restore %s: %v", key, err)
			}
		}
	}
}
