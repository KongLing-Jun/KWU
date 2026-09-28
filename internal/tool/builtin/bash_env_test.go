package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/pythonruntime"
	"kwu/internal/sandbox"
	"kwu/internal/tool"
)

func TestBashCommandEnvAppliesTaskSnapshotWithoutChangingProcess(t *testing.T) {
	const configKey = "CONNECTOR_CONFIG_DIR"
	const noticeKey = "CONNECTOR_NO_UPDATE"
	root := t.TempDir()
	nodeDir := filepath.Join(root, "node")
	cliDir := filepath.Join(root, "cli")
	configDir := filepath.Join(root, "config")
	env, err := tool.ValidateCommandEnv(map[string]string{
		tool.CommandEnvPATH: nodeDir + string(filepath.ListSeparator) + cliDir,
		configKey:           configDir,
		noticeKey:           "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	before := append([]string(nil), os.Environ()...)
	got, err := bashCommandEnv(tool.WithCommandEnv(context.Background(), env), "node --version")
	if err != nil {
		t.Fatal(err)
	}
	pathValue, ok := envValue(got, "PATH")
	if !ok || len(filepath.SplitList(pathValue)) < 2 || filepath.SplitList(pathValue)[0] != nodeDir || filepath.SplitList(pathValue)[1] != cliDir {
		t.Fatalf("managed PATH was not prepended: %q", pathValue)
	}
	for key, want := range map[string]string{
		configKey: configDir,
		noticeKey: "1",
	} {
		if value, _ := envValue(got, key); value != want {
			t.Fatalf("%s = %q, want %q", key, value, want)
		}
	}
	after := os.Environ()
	if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
		t.Fatal("bashCommandEnv changed the process environment")
	}
}

func TestBashCommandEnvOrdinaryTaskHasNoConnectorKeys(t *testing.T) {
	keys := []string{"CONNECTOR_CONFIG_DIR", "CONNECTOR_NO_UPDATE"}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	env, err := bashCommandEnv(context.Background(), "git status")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if value, _ := envValue(env, key); value != "" {
			t.Fatalf("ordinary task inherited connector value %s=%q", key, value)
		}
	}
}

func TestBashCommandEnvRemovesExplicitlyUnsetVariable(t *testing.T) {
	const rogue = "PYTHONPATH"
	t.Setenv(rogue, filepath.Join(t.TempDir(), "sitecustomize"))
	env, err := tool.ValidateCommandEnv(map[string]string{rogue: ""})
	if err != nil {
		t.Fatal(err)
	}
	got, err := bashCommandEnv(tool.WithCommandEnv(context.Background(), env), "git status")
	if err != nil {
		t.Fatal(err)
	}
	if value, exists := envValue(got, rogue); exists {
		t.Fatalf("%s remained in child environment as %q", rogue, value)
	}
	if value := os.Getenv(rogue); value == "" {
		t.Fatal("bashCommandEnv changed the process environment")
	}
}

type lazyBashCommandEnvProvider struct {
	path     string
	selected string
	err      error
	calls    atomic.Int32
}

type requestAwareBashCommandEnvProvider struct {
	legacyCalls atomic.Int32
	calls       atomic.Int32
	requests    []tool.CommandEnvRequest
	python      map[string]string
	python3     map[string]string
}

func (p *requestAwareBashCommandEnvProvider) ResolveCommandEnv(context.Context) (map[string]string, error) {
	p.legacyCalls.Add(1)
	return nil, errors.New("legacy provider method must not be called")
}

func (p *requestAwareBashCommandEnvProvider) ResolveCommandEnvForRequest(_ context.Context, request tool.CommandEnvRequest) (map[string]string, error) {
	p.calls.Add(1)
	p.requests = append(p.requests, request)
	for _, command := range request.Commands {
		if strings.EqualFold(filepath.Base(command), pythonFixtureName("python3")) || strings.EqualFold(filepath.Base(command), "python3") {
			return p.python3, nil
		}
	}
	return p.python, nil
}

func (p *lazyBashCommandEnvProvider) ResolveCommandEnv(context.Context) (map[string]string, error) {
	p.calls.Add(1)
	if p.err != nil {
		return nil, p.err
	}
	return map[string]string{tool.CommandEnvPATH: p.path, "KWU_PYTHON": p.selected}, nil
}

func TestBashCommandEnvResolvesProviderOnlyForPythonCommand(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	writeExecutableLink(t, selected, filepath.Join(dir, pythonFixtureName("python3")))
	provider := &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx := tool.WithCommandEnvProvider(context.Background(), provider)
	gitEnv, err := bashCommandEnv(ctx, "git status --short")
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 0 {
		t.Fatal("non-Python command resolved the runtime provider")
	}
	if _, exists := envValue(gitEnv, "KWU_PYTHON"); exists {
		t.Fatal("non-Python command received a Python runtime override")
	}

	pythonEnv, err := bashCommandEnv(ctx, "python -m unittest")
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls.Load())
	}
	if value, exists := envValue(pythonEnv, "KWU_PYTHON"); !exists || value == "" {
		t.Fatalf("Python command environment KWU_PYTHON = %q, exists = %v", value, exists)
	}
	if _, err := bashCommandEnv(ctx, "env CASE=2 python3 -c 'print(1)'"); err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("provider snapshot resolved %d times, want once", provider.calls.Load())
	}
}

func TestBashCommandEnvAcceptsAliasesBoundToSelectedInterpreter(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	for _, alias := range []string{"python.exe", "python3", "python3.exe"} {
		writeExecutableLink(t, selected, filepath.Join(dir, alias))
	}
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})

	for _, alias := range []string{"python", "python.exe", "python3", "python3.exe"} {
		t.Run(alias, func(t *testing.T) {
			if _, err := bashCommandEnv(ctx, alias+" -V"); err != nil {
				t.Fatalf("bashCommandEnv() rejected exact alias: %v", err)
			}
		})
	}
}

func TestBashCommandEnvPassesLiteralAliasesAndFreezesFirstSelection(t *testing.T) {
	pythonDir := t.TempDir()
	python := writeExecutableFixture(t, pythonDir, pythonFixtureName("python"), "")
	python3Dir := t.TempDir()
	python3 := writeExecutableFixture(t, python3Dir, pythonFixtureName("python3"), "")
	provider := &requestAwareBashCommandEnvProvider{
		python:  map[string]string{tool.CommandEnvPATH: pythonDir, "KWU_PYTHON": python},
		python3: map[string]string{tool.CommandEnvPATH: python3Dir, "KWU_PYTHON": python3},
	}
	ctx := tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "python -V"); err != nil {
		t.Fatal(err)
	}
	if provider.calls.Load() != 1 || provider.legacyCalls.Load() != 0 {
		t.Fatalf("provider calls = request:%d legacy:%d, want 1/0", provider.calls.Load(), provider.legacyCalls.Load())
	}
	if len(provider.requests) != 1 || len(provider.requests[0].Commands) != 1 || provider.requests[0].Commands[0] != "python" {
		t.Fatalf("provider request = %#v, want literal python binding", provider.requests)
	}

	_, err := bashCommandEnv(ctx, "python3 -V")
	if err == nil || !strings.Contains(err.Error(), "python3") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("later alias error = %v, want frozen python selection to reject python3", err)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("later alias reselected provider: calls = %d, want 1", provider.calls.Load())
	}
}

func TestBashCommandEnvPassesEveryLiteralAliasInFirstRequest(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	writeExecutableLink(t, selected, filepath.Join(dir, pythonFixtureName("python3")))
	provider := &requestAwareBashCommandEnvProvider{
		python:  map[string]string{tool.CommandEnvPATH: dir, "KWU_PYTHON": selected},
		python3: map[string]string{tool.CommandEnvPATH: dir, "KWU_PYTHON": selected},
	}
	ctx := tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "python -V; python3 -V"); err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 1 || fmt.Sprint(provider.requests[0].Commands) != "[python python3]" {
		t.Fatalf("provider commands = %#v, want both literal bindings", provider.requests)
	}
}

func TestBashCommandEnvRejectsPyLauncher(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	for _, alias := range []string{"py", "py.exe"} {
		writeExecutableLink(t, selected, filepath.Join(dir, alias))
	}
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})

	for _, alias := range []string{"py", "py.exe"} {
		t.Run(alias, func(t *testing.T) {
			_, err := bashCommandEnv(ctx, alias+" -3 -V")
			if err == nil || !errorContainsPath(err, selected) || !strings.Contains(err.Error(), alias) || !strings.Contains(err.Error(), "may select a different interpreter") {
				t.Fatalf("bashCommandEnv() error = %v, want selected path and rejected launcher", err)
			}
		})
	}
}

func TestBashCommandEnvRejectsUnprovableWrapperChain(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})

	_, err := bashCommandEnv(ctx, "sudo -u build env CASE=1 python -V")
	if err == nil || !errorContainsPath(err, selected) || !strings.Contains(err.Error(), "sudo>env") || !strings.Contains(err.Error(), "cannot be proven exact") {
		t.Fatalf("bashCommandEnv() error = %v, want full unsafe wrapper chain", err)
	}
}

func TestBashCommandEnvValidatesExplicitPythonPaths(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	wrong := writeExecutableFixture(t, t.TempDir(), pythonFixtureName("python3"), "")
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})

	if _, err := bashCommandEnv(ctx, `"`+selected+`" -V`); err != nil {
		t.Fatalf("bashCommandEnv() rejected selected explicit path: %v", err)
	}
	_, err := bashCommandEnv(ctx, `"`+wrong+`" -V`)
	if err == nil || !errorContainsPath(err, selected) || !errorContainsPath(err, wrong) || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("bashCommandEnv() error = %v, want explicit mismatch", err)
	}
	_, err = bashCommandEnv(ctx, "."+string(filepath.Separator)+pythonFixtureName("python")+" -V")
	if err == nil || !strings.Contains(err.Error(), "not absolute") {
		t.Fatalf("bashCommandEnv() error = %v, want relative explicit path rejection", err)
	}
}

func TestBashCommandEnvRejectsCommandPATHMutation(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})

	for _, test := range []struct {
		command string
		shell   sandbox.ShellKind
	}{
		{command: "PATH=/tmp python -V", shell: sandbox.ShellBash},
		{command: `$env:PATH = "C:\\wrong"; python -V`, shell: sandbox.ShellPowerShell},
	} {
		t.Run(test.command, func(t *testing.T) {
			_, err := bashCommandEnvForShell(ctx, test.command, test.shell)
			if err == nil || !errorContainsPath(err, selected) || !strings.Contains(err.Error(), "mutates PATH") {
				t.Fatalf("bashCommandEnv() error = %v, want PATH mutation rejection", err)
			}
		})
	}
}

func TestBashCommandEnvScannerProofGaps(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	writeExecutableLink(t, selected, filepath.Join(dir, pythonFixtureName("python3")))
	provider := &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx := tool.WithCommandEnvProvider(context.Background(), provider)

	if _, err := bashCommandEnv(ctx, "2>/dev/null python -V"); err != nil {
		t.Fatalf("leading fd redirect hid Python invocation: %v", err)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1 after redirected Python", provider.calls.Load())
	}
	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "cat <<'EOF'\npython -V\nEOF"); err != nil {
		t.Fatalf("heredoc body produced Python binding: %v", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatalf("heredoc-only provider calls = %d, want 0", provider.calls.Load())
	}
	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "cat <<-'EOF'\n\tpython -V\n\tEOF"); err != nil {
		t.Fatalf("tab-stripped heredoc body produced Python binding: %v", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatalf("tab-stripped heredoc-only provider calls = %d, want 0", provider.calls.Load())
	}
	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "cat <<EOF\npython -V\nEOF\npython3 -V"); err != nil {
		t.Fatalf("command after heredoc delimiter was not validated: %v", err)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("post-heredoc provider calls = %d, want 1", provider.calls.Load())
	}
	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "eval 'echo ready'"); err != nil {
		t.Fatalf("static non-Python eval resolved provider: %v", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatalf("static non-Python eval provider calls = %d, want 0", provider.calls.Load())
	}
	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnvForShell(ctx, `$items = 1,2; $items | ForEach-Object { $_ }`, sandbox.ShellPowerShell); err != nil {
		t.Fatalf("PowerShell value pipeline produced Python binding: %v", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatalf("PowerShell value pipeline provider calls = %d, want 0", provider.calls.Load())
	}
	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnvForShell(ctx, `& $PYTHON -V`, sandbox.ShellPowerShell); err == nil || !strings.Contains(err.Error(), "dynamic command word") {
		t.Fatalf("PowerShell invocation operator error = %v, want dynamic command rejection", err)
	}

	for _, test := range []struct {
		command string
		want    string
	}{
		{"source env.sh; python -V", "source command"},
		{". ./env.sh; python3 -V", "source command"},
		{"eval 'python -V'", "nested command string"},
		{`eval "$COMMAND"`, "nested command string"},
		{`cmd='; python -V'; eval echo "$cmd"`, "nested command string"},
		{`$PYTHON -V`, "dynamic command word"},
	} {
		t.Run(test.command, func(t *testing.T) {
			fresh := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})
			_, err := bashCommandEnv(fresh, test.command)
			if err == nil || !errorContainsPath(err, selected) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("bashCommandEnv() error = %v, want fail-closed proof containing %q", err, test.want)
			}
		})
	}
}

func TestPythonShellMutationBindings(t *testing.T) {
	tests := []struct {
		name       string
		powerShell bool
		command    string
		wantReason string
	}{
		{name: "argument text", command: `python -c 'print("alias python")'`},
		{name: "quoted command text", command: `echo 'alias python=wrong'; python -V`},
		{name: "heredoc body", command: "python - <<'PY'\nprint(\"alias python\")\nPY"},
		{name: "local PATH assignment consumed by other command", command: `PATH=/wrong echo ready; python -V`},
		{name: "local PATH assignment", command: `PATH=/wrong python -V`, wantReason: "mutates PATH"},
		{name: "persistent PATH assignment", command: `PATH=/wrong; python -V`, wantReason: "mutates PATH"},
		{name: "export PATH", command: `export PATH=/wrong; python -V`, wantReason: "mutates PATH"},
		{name: "unset PATHEXT", command: `unset PATHEXT; python -V`, wantReason: "mutates PATH"},
		{name: "env unset short", command: `env -u PATH python -V`, wantReason: "mutates PATH"},
		{name: "env unset long", command: `env --unset=PATHEXT python -V`, wantReason: "mutates PATH"},
		{name: "env assignment", command: `env PATH=/wrong python -V`, wantReason: "mutates PATH"},
		{name: "alias target", command: `alias python=/wrong; python -V`, wantReason: "shell alias or function"},
		{name: "function keyword target", command: `function python { echo wrong; }; python -V`, wantReason: "shell alias or function"},
		{name: "function declaration target", command: `python() { echo wrong; }; python -V`, wantReason: "shell alias or function"},
		{name: "hash target", command: `hash -p /wrong/python python; python -V`, wantReason: "hash table"},
		{name: "powershell argument text", powerShell: true, command: `Write-Output 'Set-Alias python wrong'; python -V`},
		{name: "powershell PATH", powerShell: true, command: `$env:PATH = "C:\\wrong"; python -V`, wantReason: "mutates PATH"},
		{name: "powershell inline PATH", powerShell: true, command: `$env:PATH="C:\\wrong"; python -V`, wantReason: "mutates PATH"},
		{name: "powershell compound PATH", powerShell: true, command: `$env:PATH += ";C:\\wrong"; python -V`, wantReason: "mutates PATH"},
		{name: "powershell null-coalescing PATH", powerShell: true, command: `$env:PATH??="C:\\wrong"; python -V`, wantReason: "mutates PATH"},
		{name: "powershell PATHEXT", powerShell: true, command: `$env:PATHEXT = ".BAD"; python -V`, wantReason: "mutates PATH"},
		{name: "powershell Set-Alias positional", powerShell: true, command: `Set-Alias python wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell Set-Alias named", powerShell: true, command: `Set-Alias -Name python -Value wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell New-Alias", powerShell: true, command: `New-Alias python wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell sal", powerShell: true, command: `sal python wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell nal", powerShell: true, command: `nal python wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell Set-Item environment", powerShell: true, command: `Set-Item Env:PATH wrong; python -V`, wantReason: "mutates PATH"},
		{name: "powershell provider slash environment", powerShell: true, command: `Set-Item Env:\Path wrong; python -V`, wantReason: "mutates PATH"},
		{name: "powershell provider slash alias", powerShell: true, command: `Set-Item Alias:\python wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell New-Item provider", powerShell: true, command: `New-Item Alias:/python -Value wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell Clear-Item provider", powerShell: true, command: `Clear-Item Env:\Path; python -V`, wantReason: "mutates PATH"},
		{name: "powershell Remove-Item provider", powerShell: true, command: `Remove-Item Function:\python; python -V`, wantReason: "shell alias or function"},
		{name: "powershell Remove-Item alias", powerShell: true, command: `ri Alias:\python; python -V`, wantReason: "shell alias or function"},
		{name: "powershell si alias", powerShell: true, command: `si Alias:python wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell Set-Item function", powerShell: true, command: `Set-Item -Path Function:python -Value wrong; python -V`, wantReason: "shell alias or function"},
		{name: "powershell function", powerShell: true, command: `function global:python { echo wrong }; python -V`, wantReason: "shell alias or function"},
		{name: "bash eval PATH mutation", command: `eval 'export PATH=/wrong'; python -V`, wantReason: "mutates PATH"},
		{name: "bash eval function mutation", command: `eval 'function python { echo wrong; }'; python -V`, wantReason: "shell alias or function"},
		{name: "powershell iex PATH mutation", powerShell: true, command: `iex 'Set-Item Env:PATH wrong'; python -V`, wantReason: "mutates PATH"},
		{name: "powershell Invoke-Expression alias mutation", powerShell: true, command: `Invoke-Expression 'Set-Alias python wrong'; python -V`, wantReason: "shell alias or function"},
		{name: "powershell pipeline Invoke-Expression", powerShell: true, command: `$command | iex; python -V`, wantReason: "Invoke-Expression"},
		{name: "special builtin assignment", command: `PATH=/wrong export PATH; python -V`, wantReason: "mutates PATH"},
		{name: "local declaration", command: `local PATH=/wrong; python -V`, wantReason: "mutates PATH"},
		{name: "declare declaration", command: `declare -x PATH=/wrong; python -V`, wantReason: "mutates PATH"},
		{name: "typeset declaration", command: `typeset PATHEXT=/wrong; python -V`, wantReason: "mutates PATH"},
		{name: "readonly declaration", command: `readonly PATH=/wrong; python -V`, wantReason: "mutates PATH"},
		{name: "declaration special assignment", command: `PATH=/wrong readonly PATH; python -V`, wantReason: "mutates PATH"},
		{name: "builtin source", command: `builtin source env.sh; python -V`, wantReason: "source command"},
		{name: "bash child mutation is isolated", command: `bash -c 'export PATH=/wrong'; python -V`},
		{name: "powershell child mutation is isolated", powerShell: true, command: `pwsh -Command '$env:PATH=wrong'; python -V`},
		{name: "quoted heredoc then Python", command: "cat <<'EOF'\npython text\nEOF\npython -V"},
		{name: "backslash quoted heredoc then Python", command: "cat <<\\EOF\npython text\nEOF\npython -V"},
		{name: "unquoted heredoc substitution", command: "cat <<EOF\n$(python -V)\nEOF", wantReason: "heredoc command substitution"},
		{name: "unquoted heredoc backticks", command: "cat <<EOF\n`python -V`\nEOF", wantReason: "heredoc command substitution"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := pythonruntime.ScanPythonShellBindings(tt.command, pythonShellCommands)
			if tt.powerShell {
				bindings = pythonruntime.ScanPowerShellPythonBindings(tt.command, pythonShellCommands)
			}
			if len(bindings) == 0 {
				t.Fatal("scanner did not find the final Python command")
			}
			got := bindings[len(bindings)-1].Unprovable
			if tt.wantReason == "" && got != "" {
				t.Fatalf("binding unexpectedly unprovable: %q", got)
			}
			if tt.wantReason != "" && !strings.Contains(got, tt.wantReason) {
				t.Fatalf("binding reason = %q, want containing %q", got, tt.wantReason)
			}
		})
	}

	t.Run("persistent mutation reaches every later binding", func(t *testing.T) {
		bindings := pythonruntime.ScanPythonShellBindings(`PATH=/wrong; $PYTHON -V; python -V`, pythonShellCommands)
		if len(bindings) != 2 {
			t.Fatalf("bindings = %#v, want dynamic and literal Python bindings", bindings)
		}
		for _, binding := range bindings {
			if !strings.Contains(binding.Unprovable, "mutates PATH") {
				t.Fatalf("binding %#v is missing the persistent mutation", binding)
			}
		}
	})

	t.Run("PowerShell assignment syntax is shell-specific", func(t *testing.T) {
		bindings := pythonruntime.ScanPythonShellBindings(`$env:PATH = wrong; python -V`, pythonShellCommands)
		if len(bindings) == 0 {
			t.Fatal("scanner did not find Bash bindings")
		}
		if got := bindings[len(bindings)-1].Unprovable; strings.Contains(got, "mutates PATH") {
			t.Fatalf("Bash scanner inherited PowerShell assignment semantics: %q", got)
		}
	})
}

func TestBashCommandEnvForShellRejectsStructuredMutations(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	tests := []struct {
		name    string
		shell   sandbox.ShellKind
		command string
		want    string
	}{
		{name: "Set-Item environment provider", shell: sandbox.ShellPowerShell, command: `Set-Item Env:\Path wrong; python -V`, want: "mutates PATH"},
		{name: "New-Item alias provider", shell: sandbox.ShellPowerShell, command: `New-Item Alias:\python -Value wrong; python -V`, want: "shell alias or function"},
		{name: "Clear-Item environment provider", shell: sandbox.ShellPowerShell, command: `Clear-Item Env:/Path; python -V`, want: "mutates PATH"},
		{name: "Remove-Item function provider", shell: sandbox.ShellPowerShell, command: `Remove-Item Function:\python; python -V`, want: "shell alias or function"},
		{name: "pipeline Invoke-Expression", shell: sandbox.ShellPowerShell, command: `$command | iex; python -V`, want: "Invoke-Expression"},
		{name: "local declaration", shell: sandbox.ShellBash, command: `local PATH=/wrong; python -V`, want: "mutates PATH"},
		{name: "declare special assignment", shell: sandbox.ShellBash, command: `PATH=/wrong declare PATH; python -V`, want: "mutates PATH"},
		{name: "typeset declaration", shell: sandbox.ShellBash, command: `typeset PATH=/wrong; python -V`, want: "mutates PATH"},
		{name: "readonly declaration", shell: sandbox.ShellBash, command: `readonly PATHEXT=/wrong; python -V`, want: "mutates PATH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &lazyBashCommandEnvProvider{path: dir, selected: selected}
			ctx := tool.WithCommandEnvProvider(context.Background(), provider)
			_, err := bashCommandEnvForShell(ctx, tt.command, tt.shell)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("bashCommandEnvForShell() error = %v, want containing %q", err, tt.want)
			}
			if provider.calls.Load() != 1 {
				t.Fatalf("provider calls = %d, want 1", provider.calls.Load())
			}
		})
	}
}

func TestBashCommandEnvHeredocExpansionSemantics(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	for _, command := range []string{
		"cat <<'EOF'\npython text\nEOF\npython -V",
		"cat <<\\EOF\npython text\nEOF\npython -V",
	} {
		provider := &lazyBashCommandEnvProvider{path: dir, selected: selected}
		ctx := tool.WithCommandEnvProvider(context.Background(), provider)
		if _, err := bashCommandEnv(ctx, command); err != nil {
			t.Fatalf("quoted heredoc rejected following Python: %v", err)
		}
		if provider.calls.Load() != 1 {
			t.Fatalf("quoted heredoc provider calls = %d, want 1", provider.calls.Load())
		}
	}

	provider := &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx := tool.WithCommandEnvProvider(context.Background(), provider)
	_, err := bashCommandEnv(ctx, "cat <<EOF\n$(python -V)\nEOF")
	if err == nil || !strings.Contains(err.Error(), "heredoc command substitution") {
		t.Fatalf("unquoted heredoc substitution error = %v, want fail-closed", err)
	}
	if provider.calls.Load() != 1 {
		t.Fatalf("unquoted heredoc substitution provider calls = %d, want 1", provider.calls.Load())
	}

	provider = &lazyBashCommandEnvProvider{path: dir, selected: selected}
	ctx = tool.WithCommandEnvProvider(context.Background(), provider)
	if _, err := bashCommandEnv(ctx, "cat <<EOF\npython -V\nEOF"); err != nil {
		t.Fatalf("plain heredoc text produced Python binding: %v", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatalf("plain heredoc provider calls = %d, want 0", provider.calls.Load())
	}
}

func TestBashCommandEnvRejectsMismatchedAndMissingAliases(t *testing.T) {
	selectedDir := t.TempDir()
	selected := writeExecutableFixture(t, selectedDir, pythonFixtureName("python"), "")
	provider := &lazyBashCommandEnvProvider{path: selectedDir, selected: selected}

	t.Run("missing", func(t *testing.T) {
		ctx := tool.WithCommandEnvProvider(context.Background(), provider)
		_, err := bashCommandEnv(ctx, "python3 -V")
		if err == nil || !errorContainsPath(err, selected) || !strings.Contains(err.Error(), "python3") || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("bashCommandEnv() error = %v, want selected path and missing alias", err)
		}
	})

	for _, name := range []string{"other directory", "Windows Store shim directory"} {
		t.Run(name, func(t *testing.T) {
			wrongDir := t.TempDir()
			if strings.Contains(name, "Store") {
				wrongDir = filepath.Join(wrongDir, "Microsoft", "WindowsApps")
				if err := os.MkdirAll(wrongDir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			wrong := writeExecutableFixture(t, wrongDir, pythonFixtureName("python3"), "")
			t.Setenv("PATH", wrongDir)
			ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: selectedDir, selected: selected})
			_, err := bashCommandEnv(ctx, "python3 -V")
			if err == nil || !errorContainsPath(err, selected) || !errorContainsPath(err, wrong) || !strings.Contains(err.Error(), "does not match") {
				t.Fatalf("bashCommandEnv() error = %v, want selected and mismatched alias paths", err)
			}
		})
	}
}

func TestBashRejectsWrongPythonAliasBeforeStartingShell(t *testing.T) {
	selectedDir := t.TempDir()
	selected := writeExecutableFixture(t, selectedDir, pythonFixtureName("python"), "")
	wrongDir := t.TempDir()
	writeExecutableFixture(t, wrongDir, pythonFixtureName("python3"), "")
	t.Setenv("PATH", wrongDir)
	marker := filepath.Join(t.TempDir(), "shell-started")
	shell := writeMarkerShell(t, marker)
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: selectedDir, selected: selected})
	args, _ := json.Marshal(map[string]string{"command": "python3 -V"})

	_, err := (bash{shell: sandbox.Shell{Kind: sandbox.ShellBash, Path: shell}}).Execute(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Execute() error = %v, want exact-binding rejection", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("shell child started before binding rejection: %v", statErr)
	}
}

func TestBashRejectsUnprovableNestedCmdBeforeStartingShell(t *testing.T) {
	dir := t.TempDir()
	selected := writeExecutableFixture(t, dir, pythonFixtureName("python"), "")
	marker := filepath.Join(t.TempDir(), "shell-started")
	shell := writeMarkerShell(t, marker)
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{path: dir, selected: selected})
	args, _ := json.Marshal(map[string]string{"command": "cmd /c python -V"})

	_, err := (bash{shell: sandbox.Shell{Kind: sandbox.ShellBash, Path: shell}}).Execute(ctx, args)
	if err == nil || !errorContainsPath(err, selected) || !strings.Contains(err.Error(), "resolution cannot be proven exact") {
		t.Fatalf("Execute() error = %v, want unprovable cmd rejection", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("shell child started before nested cmd rejection: %v", statErr)
	}
}

func TestBashPropagatesPythonProviderErrorBeforeStartingShell(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "shell-started")
	shell := writeMarkerShell(t, marker)
	want := errors.New("no usable Python interpreter: all candidates failed")
	ctx := tool.WithCommandEnvProvider(context.Background(), &lazyBashCommandEnvProvider{err: want})
	args, _ := json.Marshal(map[string]string{"command": "python -V"})

	_, err := (bash{shell: sandbox.Shell{Kind: sandbox.ShellBash, Path: shell}}).Execute(ctx, args)
	if !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want provider error", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("shell child started before provider error: %v", statErr)
	}
}

func pythonFixtureName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func writeExecutableFixture(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeExecutableLink(t *testing.T, selected, alias string) {
	t.Helper()
	if filepath.Clean(selected) == filepath.Clean(alias) {
		return
	}
	if err := os.Link(selected, alias); err != nil {
		t.Fatalf("link alias %q: %v", alias, err)
	}
}

func writeMarkerShell(t *testing.T, marker string) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		return writeExecutableFixture(t, dir, "shell.cmd", "@echo off\r\ntype nul > \""+marker+"\"\r\n")
	}
	return writeExecutableFixture(t, dir, "shell", "#!/bin/sh\n: > \""+marker+"\"\n")
}

func errorContainsPath(err error, path string) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	if runtime.GOOS == "windows" {
		return strings.Contains(strings.ToLower(message), strings.ToLower(strconv.Quote(path)))
	}
	return strings.Contains(message, strconv.Quote(path))
}

func TestPythonShellCommandDetection(t *testing.T) {
	tests := map[string]bool{
		"git status":                                            false,
		"echo python":                                           false,
		"echo python\npython --version":                         true,
		"printf '%s' python3.exe > result.txt":                  false,
		"env -u python echo python":                             false,
		"env -S 'echo python'":                                  false,
		"sudo echo python":                                      false,
		"call echo python":                                      false,
		"timeout 30s echo python":                               false,
		"cmd /c echo python":                                    false,
		"cmd.exe /d /c \"call echo python\"":                    false,
		"pwsh -Command 'echo python'":                           false,
		"Start-Process echo -ArgumentList python":               false,
		"python -m unittest":                                    true,
		"echo ready && python.exe test.py":                      true,
		"2>/dev/null python -V":                                 true,
		"3>&1 python3 -V":                                       true,
		"/usr/local/bin/python3 -c 'print(1)'":                  true,
		`"C:\Program Files\Python312\python.exe" -V`:            true,
		`& "C:\Program Files\Python312\python.exe" -V`:          true,
		"cmd.exe /d /c python -V":                               true,
		"cmd.exe /D /C python -V":                               true,
		"cmd.exe /d /c \"call python -V\"":                      true,
		"pwsh -NoProfile -Command python -V":                    true,
		"powershell.exe -Command 'python3 -V'":                  true,
		"Start-Process python":                                  true,
		"Start-Process -FilePath python.exe -ArgumentList '-V'": true,
		"py -3 -V":                                  true,
		"py.exe -3 -V":                              true,
		"env CASE=1 python3 -c 'print(1)'":          true,
		"env --split-string 'python -V'":            true,
		"sudo -u build env CASE=1 python -V":        true,
		"timeout --signal TERM 30s python3 -V":      true,
		"nohup python -m unittest":                  true,
		"bash -C python":                            false,
		"bash -c 'python -V'":                       true,
		"if python -V; then echo ok; fi":            true,
		"while python -V; do echo ok; done":         true,
		"! python -V":                               true,
		"if echo python; then echo ok; fi":          false,
		"(python -V)":                               true,
		"if ($true) { python -V }":                  true,
		"{ python -V; }":                            true,
		"if ($true) { echo python }":                false,
		"echo ${python}":                            false,
		"echo $(python3 -c 'print(1)')":             true,
		"echo $(echo python)":                       false,
		"cat <<'EOF'\npython -V\nEOF":               false,
		"cat <<EOF\npython -V\nEOF\npython -V":      true,
		"cat <<-'EOF'\n\tpython -V\n\tEOF":          false,
		"cat <<-EOF\n\tpython -V\n\tEOF\npython -V": true,
		"source env.sh":                             false,
		"source env.sh; python -V":                  true,
		". ./env.sh; python3 -V":                    true,
		"eval 'echo ready'":                         false,
		"eval 'python -V'":                          true,
		`eval "$COMMAND"`:                           true,
		`$PYTHON -V`:                                true,
		`echo $PYTHON`:                              false,
		`$p = "python"`:                             false,
		`$p = Get-Command python`:                   false,
		`$p = Get-Command python; & $p -V`:          true,
		`if ($false) { echo python }`:               false,
		`if ($null) { echo python }`:                false,
		`if ($?) { echo python }`:                   false,
		`[ "$name" = python ]`:                      false,
	}
	for command, want := range tests {
		if got := hasShellCommandWord(command, pythonShellCommands); got != want {
			t.Errorf("hasShellCommandWord(%q) = %v, want %v", command, got, want)
		}
	}
}

func TestPythonShellCommandDetectionClosingParenthesisDoesNotHang(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		done <- hasShellCommandWord("(echo python)", pythonShellCommands)
	}()
	select {
	case got := <-done:
		if got {
			t.Fatal("argument-only Python text was detected as a command")
		}
	case <-time.After(time.Second):
		t.Fatal("Python command scanner hung on a closing parenthesis")
	}
}

func TestPythonShellCommandDetectionClosingBraceDoesNotHang(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		done <- hasShellCommandWord("{ echo python; }", pythonShellCommands)
	}()
	select {
	case got := <-done:
		if got {
			t.Fatal("argument-only Python text was detected as a command")
		}
	case <-time.After(time.Second):
		t.Fatal("Python command scanner hung on a closing brace")
	}
}

func TestBashMergesLoginShellPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login shell PATH probing is POSIX-only")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	probe := filepath.Join(bin, "kwu-path-probe")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\nprintf 'shell-path-ok\\n'\n"), 0o755); err != nil {
		t.Fatalf("write probe: %v", err)
	}

	// Inject a deterministic login-shell PATH instead of spawning a real login
	// shell. The real probe (defaultBashShellPATH) runs up to three
	// interactive-login shells with a 2s timeout each; under the CPU load of
	// `go test ./...` it times out and returns an empty PATH, so this test failed
	// with command-not-found only in the full suite, never in isolation. This
	// test covers merging the probed PATH into the exec environment; the probe's
	// own parsing/merging is covered by TestParseShellPATH and TestMergePathLists.
	prev := bashShellPATH
	bashShellPATH = func(context.Context) string { return bin + ":/usr/bin:/bin" }
	t.Cleanup(func() { bashShellPATH = prev })

	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	b := bash{shell: sandbox.Shell{Kind: sandbox.ShellBash, Path: "/bin/sh"}}
	args, _ := json.Marshal(map[string]string{"command": "kwu-path-probe"})

	out, err := b.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("command should resolve through merged login-shell PATH: %v (out=%q)", err, out)
	}
	if !strings.Contains(out, "shell-path-ok") {
		t.Fatalf("output = %q, want shell-path-ok", out)
	}
}

func TestParseShellPATH(t *testing.T) {
	const marker = "__KWU_BASH_PATH__="
	cases := []struct {
		name string
		out  string
		want string
	}{
		{"simple", marker + "/usr/local/bin:/usr/bin\n", "/usr/local/bin:/usr/bin"},
		{"crlf", "noise\r\n" + marker + "/opt/bin:/bin\r\n", "/opt/bin:/bin"},
		{"last marker wins", marker + "/early\n" + marker + "/late\n", "/late"},
		{"ignores surrounding output", "login banner\n" + marker + "/p\ntrailing\n", "/p"},
		{"absent", "no marker here\n", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseShellPATH([]byte(c.out), marker); got != c.want {
				t.Fatalf("parseShellPATH(%q) = %q, want %q", c.out, got, c.want)
			}
		})
	}
}

func TestMergePathLists(t *testing.T) {
	sep := string(os.PathListSeparator)
	cases := []struct {
		name      string
		primary   string
		secondary string
		want      string
	}{
		{"dedupes, primary first", "/a" + sep + "/b", "/b" + sep + "/c", "/a" + sep + "/b" + sep + "/c"},
		{"empty secondary", "/a" + sep + "/b", "", "/a" + sep + "/b"},
		{"empty primary", "", "/x" + sep + "/y", "/x" + sep + "/y"},
		{"skips blank entries", "/a" + sep + sep + "/b", "", "/a" + sep + "/b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mergePathLists(c.primary, c.secondary); got != c.want {
				t.Fatalf("mergePathLists(%q, %q) = %q, want %q", c.primary, c.secondary, got, c.want)
			}
		})
	}
}
