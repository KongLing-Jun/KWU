package pythonruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/tool"
)

func TestResolver_ResolveAutoOrderAndPyArguments(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	root := t.TempDir()
	managed := writeRuntime(t, root, PinnedVersion, nil, "")
	realExecutable := filepath.Join(workspace, "real-python.exe")
	if err := os.WriteFile(realExecutable, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	resolver := NewResolverWithDependencies(Options{
		WorkspaceRoot: workspace,
		Interpreter:   filepath.Join(workspace, "explicit-python.exe"),
	}, Dependencies{
		LookPath: func(name string) (string, error) {
			return filepath.Join(workspace, name+".exe"), nil
		},
		Probe: func(_ context.Context, executable string, args ...string) (ProbeResult, error) {
			calls = append(calls, executable+" "+strings.Join(args, " "))
			if strings.HasSuffix(executable, "python3.exe") {
				return ProbeResult{Executable: realExecutable, Major: 3, Minor: 11, Patch: 7}, nil
			}
			return ProbeResult{}, errors.New("unusable")
		},
		Manager: fixtureManager(t, root, managed.Root),
	})

	got, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	wantVenv := filepath.Join(workspace, ".venv", "bin", "python")
	if runtime.GOOS == "windows" {
		wantVenv = filepath.Join(workspace, ".venv", "Scripts", "python.exe")
	}
	wantCalls := []string{
		wantVenv + " ",
		filepath.Join(workspace, "explicit-python.exe") + " ",
		filepath.Join(workspace, "py.exe") + " -3",
		filepath.Join(workspace, "python3.exe") + " ",
	}
	if fmt.Sprint(calls) != fmt.Sprint(wantCalls) {
		t.Fatalf("probe calls = %q, want %q", calls, wantCalls)
	}
	if got.Source != SourceSystem || got.Version != "3.11.7" {
		t.Fatalf("Resolve() = %+v, want validated system Python", got)
	}
	if got.Env["KWU_PYTHON"] != got.Executable || got.Env["PATH"] != filepath.Dir(got.Executable) {
		t.Fatalf("task environment = %v", got.Env)
	}
	if managed.Executable == "" {
		t.Fatal("managed fixture was not created")
	}
}

func TestResolver_ResolveAutoFallbacks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		pyPath     string
		pyVersion  ProbeResult
		pyErr      error
		wantSource Source
	}{
		{
			name:       "Store shim falls back to managed",
			pyPath:     filepath.Join("C:\\", "Users", "tester", "AppData", "Local", "Microsoft", "WindowsApps", "python.exe"),
			wantSource: SourceManaged,
		},
		{
			name:       "old Python falls back to managed",
			pyPath:     filepath.Join(t.TempDir(), "python.exe"),
			pyVersion:  ProbeResult{Executable: filepath.Join(t.TempDir(), "python-real.exe"), Major: 3, Minor: 9, Patch: 18},
			wantSource: SourceManaged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			managed := writeRuntime(t, root, PinnedVersion, nil, "")
			resolver := NewResolverWithDependencies(Options{}, Dependencies{
				LookPath: func(name string) (string, error) {
					if name == "py" {
						return tt.pyPath, nil
					}
					return "", errors.New("not found")
				},
				Probe: func(_ context.Context, executable string, _ ...string) (ProbeResult, error) {
					if filepath.Clean(executable) == filepath.Clean(managed.Executable) {
						return ProbeResult{Executable: managed.Executable, Major: 3, Minor: 12, Patch: 10}, nil
					}
					return tt.pyVersion, tt.pyErr
				},
				Manager: fixtureManager(t, root, managed.Root),
			})
			got, err := resolver.Resolve(context.Background())
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Source != tt.wantSource {
				t.Fatalf("Resolve().Source = %q, want %q", got.Source, tt.wantSource)
			}
		})
	}
}

func TestResolver_ResolveAutoDefersManagedIntegrityValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		userProbe ProbeResult
		userErr   error
		wantRead  int64
		wantWalk  int64
		want      Source
	}{
		{
			name:      "user candidate succeeds without touching managed runtime",
			userProbe: ProbeResult{Major: 3, Minor: 12, Patch: 9},
			want:      SourceSystem,
		},
		{
			name:     "user candidate failure validates managed fallback",
			userErr:  errors.New("unusable"),
			wantRead: 1,
			wantWalk: 1,
			want:     SourceManaged,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			managed := writeRuntime(t, root, PinnedVersion, nil, "")
			manager := fixtureManager(t, root, managed.Root)
			var reads atomic.Int64
			readFile := manager.fs.readFile
			manager.fs.readFile = func(path string) ([]byte, error) {
				reads.Add(1)
				return readFile(path)
			}
			var walks atomic.Int64
			walkDir := manager.fs.walkDir
			manager.fs.walkDir = func(root string, fn fs.WalkDirFunc) error {
				walks.Add(1)
				return walkDir(root, fn)
			}
			userExecutable := filepath.Join(t.TempDir(), "python.exe")
			if err := os.WriteFile(userExecutable, []byte("fixture"), 0o755); err != nil {
				t.Fatal(err)
			}
			resolver := NewResolverWithDependencies(Options{}, Dependencies{
				LookPath: func(name string) (string, error) {
					if name == "python" {
						return userExecutable, nil
					}
					return "", errors.New("not found")
				},
				Probe: func(_ context.Context, executable string, _ ...string) (ProbeResult, error) {
					if executable == userExecutable {
						result := tt.userProbe
						result.Executable = userExecutable
						return result, tt.userErr
					}
					return ProbeResult{Executable: managed.Executable, Major: 3, Minor: 12, Patch: 10}, nil
				},
				Manager: manager,
			})

			got, err := resolver.Resolve(context.Background())
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Source != tt.want || reads.Load() != tt.wantRead || walks.Load() != tt.wantWalk {
				t.Fatalf("Resolve() = %+v, manifest reads = %d, walks = %d; want source %q, reads %d, walks %d", got, reads.Load(), walks.Load(), tt.want, tt.wantRead, tt.wantWalk)
			}
		})
	}
}

func TestResolver_ResolveManaged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		install      bool
		probeVersion ProbeResult
		wantErr      string
	}{
		{name: "missing pinned runtime", wantErr: "is required"},
		{name: "wrong runtime version", install: true, probeVersion: ProbeResult{Major: 3, Minor: 12, Patch: 9}, wantErr: "want 3.12.10"},
		{name: "exact pinned runtime", install: true, probeVersion: ProbeResult{Major: 3, Minor: 12, Patch: 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			var managed ManagedRuntime
			if tt.install {
				managed = writeRuntime(t, root, PinnedVersion, nil, "")
			}
			resolver := NewResolverWithDependencies(Options{Mode: ModeManaged}, Dependencies{
				LookPath: func(string) (string, error) { return "", errors.New("must not be called") },
				Probe: func(_ context.Context, _ string, _ ...string) (ProbeResult, error) {
					result := tt.probeVersion
					result.Executable = managed.Executable
					return result, nil
				},
				Manager: func() *Manager {
					if !tt.install {
						return NewManager(root)
					}
					return fixtureManager(t, root, managed.Root)
				}(),
			})
			got, err := resolver.Resolve(context.Background())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Resolve() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got.Source != SourceManaged || got.Env["PYTHONHOME"] != managed.Root || got.Env["PYTHONNOUSERSITE"] != "1" || got.Env["PYTHONPATH"] != "" || got.Env["PYTHONUTF8"] != "1" {
				t.Fatalf("managed result = %+v", got)
			}
		})
	}
}

func TestResolver_ResolveCommandEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	managed := writeRuntime(t, root, PinnedVersion, nil, "")
	resolver := NewResolverWithDependencies(Options{Mode: ModeManaged}, Dependencies{
		Probe: func(_ context.Context, _ string, _ ...string) (ProbeResult, error) {
			return ProbeResult{Executable: managed.Executable, Major: 3, Minor: 12, Patch: 10}, nil
		},
		Manager: fixtureManager(t, root, managed.Root),
	})

	env, err := resolver.ResolveCommandEnv(context.Background())
	if err != nil {
		t.Fatalf("ResolveCommandEnv() error = %v", err)
	}
	if env["KWU_PYTHON"] != managed.Executable || env["PYTHONHOME"] != managed.Root {
		t.Fatalf("ResolveCommandEnv() = %v", env)
	}
}

func TestResolver_ResolveCommandEnvAutoWithoutPythonFailsClosed(t *testing.T) {
	resolver := NewResolverWithDependencies(Options{}, Dependencies{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
		Manager:  NewManager(t.TempDir()),
	})
	env, err := resolver.ResolveCommandEnv(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no usable Python interpreter") {
		t.Fatalf("ResolveCommandEnv() = (%#v, %v), want joined resolver error", env, err)
	}
	if env != nil {
		t.Fatalf("ResolveCommandEnv() env = %#v, want nil on error", env)
	}
}

func TestResolver_RequestAwareAliasSelection(t *testing.T) {
	workspace := t.TempDir()
	venvDir := filepath.Join(workspace, ".venv", "bin")
	venvName := "python"
	if runtime.GOOS == "windows" {
		venvDir = filepath.Join(workspace, ".venv", "Scripts")
		venvName = "python.exe"
	}
	if err := os.MkdirAll(venvDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workspacePython := filepath.Join(venvDir, venvName)
	if err := os.WriteFile(workspacePython, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	managed := writeRuntime(t, root, PinnedVersion, nil, "")
	if runtime.GOOS == "windows" {
		linkManagedAliasFixture(t, &managed)
	}
	newResolver := func() *Resolver {
		return NewResolverWithDependencies(Options{WorkspaceRoot: workspace}, Dependencies{
			LookPath: func(string) (string, error) { return "", errors.New("not found") },
			Probe: func(_ context.Context, executable string, _ ...string) (ProbeResult, error) {
				version := ProbeResult{Executable: executable, Major: 3, Minor: 12, Patch: 9}
				if filepath.Clean(executable) == filepath.Clean(managed.Executable) {
					version.Patch = 10
				}
				return version, nil
			},
			Manager: fixtureManager(t, root, managed.Root),
		})
	}

	t.Run("python keeps workspace candidate", func(t *testing.T) {
		env, err := newResolver().ResolveCommandEnvForRequest(context.Background(), tool.CommandEnvRequest{Commands: []string{"python"}})
		if err != nil {
			t.Fatal(err)
		}
		if env["KWU_PYTHON"] != workspacePython {
			t.Fatalf("selected = %q, want workspace %q", env["KWU_PYTHON"], workspacePython)
		}
	})

	t.Run("python3 falls back to managed", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("the bundled python3 alias is a Windows runtime contract")
		}
		env, err := newResolver().ResolveCommandEnvForRequest(context.Background(), tool.CommandEnvRequest{Commands: []string{"python3"}})
		if err != nil {
			t.Fatal(err)
		}
		if env["KWU_PYTHON"] != managed.Executable {
			t.Fatalf("selected = %q, want managed %q", env["KWU_PYTHON"], managed.Executable)
		}
	})

	t.Run("multi alias requires one candidate to bind all", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("the bundled python3 alias is a Windows runtime contract")
		}
		env, err := newResolver().ResolveCommandEnvForRequest(context.Background(), tool.CommandEnvRequest{Commands: []string{"python", "python3", "python"}})
		if err != nil {
			t.Fatal(err)
		}
		if env["KWU_PYTHON"] != managed.Executable {
			t.Fatalf("selected = %q, want managed candidate satisfying both aliases", env["KWU_PYTHON"])
		}
	})

	t.Run("exact user alias stays workspace", func(t *testing.T) {
		alias := filepath.Join(venvDir, "python3")
		if runtime.GOOS == "windows" {
			alias += ".exe"
		}
		if err := os.Link(workspacePython, alias); err != nil {
			t.Fatal(err)
		}
		env, err := newResolver().ResolveCommandEnvForRequest(context.Background(), tool.CommandEnvRequest{Commands: []string{"python3"}})
		if err != nil {
			t.Fatal(err)
		}
		if env["KWU_PYTHON"] != workspacePython {
			t.Fatalf("selected = %q, want exact workspace alias", env["KWU_PYTHON"])
		}
	})

	t.Run("py launcher fails closed", func(t *testing.T) {
		for _, command := range []string{"py", "py.exe"} {
			_, err := newResolver().ResolveCommandEnvForRequest(context.Background(), tool.CommandEnvRequest{Commands: []string{command}})
			if err == nil || !strings.Contains(err.Error(), "py launcher may select a different interpreter") {
				t.Fatalf("%s request error = %v, want explicit py launcher rejection", command, err)
			}
		}
	})
}

func TestResolver_RequestUsesStaticPATHEXTOverride(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PATHEXT command resolution is Windows-specific")
	}
	explicitDir := t.TempDir()
	explicit := filepath.Join(explicitDir, "python.cmd")
	if err := os.WriteFile(explicit, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	managed := writeRuntime(t, root, PinnedVersion, nil, "")
	linkManagedAliasFixture(t, &managed)
	resolver := NewResolverWithDependencies(Options{Interpreter: explicit}, Dependencies{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
		Probe: func(_ context.Context, executable string, _ ...string) (ProbeResult, error) {
			patch := 9
			if filepath.Clean(executable) == filepath.Clean(managed.Executable) {
				patch = 10
			}
			return ProbeResult{Executable: executable, Major: 3, Minor: 12, Patch: patch}, nil
		},
		Manager: fixtureManager(t, root, managed.Root),
	})
	t.Setenv("PATHEXT", ".CMD")
	ctx := tool.WithCommandEnvProvider(context.Background(), resolver)
	ctx = tool.WithCommandEnv(ctx, map[string]string{"PATHEXT": ".EXE"})
	env, err := tool.ResolveCommandEnvForExecutionRequest(ctx, true, tool.CommandEnvRequest{
		Commands: []string{"python"},
		PathExt:  os.Getenv("PATHEXT"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["KWU_PYTHON"] != managed.Executable {
		t.Fatalf("selected = %q, want managed %q after static PATHEXT override rejects explicit .cmd", env["KWU_PYTHON"], managed.Executable)
	}
	if env["PATHEXT"] != ".EXE" {
		t.Fatalf("final PATHEXT = %q, want static override", env["PATHEXT"])
	}
}

func linkManagedAliasFixture(t *testing.T, managed *ManagedRuntime) {
	t.Helper()
	alias := filepath.Join(managed.Root, "python3.exe")
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(managed.Executable, alias); err != nil {
		t.Fatal(err)
	}
	managed.Manifest.Files, managed.Manifest.PayloadSHA256 = fixturePayloadReceipt(t, managed.Root)
	data, err := json.Marshal(managed.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managed.Root, ManifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateShellCommandBindingsRejectsResolutionOverrides(t *testing.T) {
	dir := t.TempDir()
	name := "python"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	selected := filepath.Join(dir, name)
	if err := os.WriteFile(selected, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	baseEnv := []string{"KWU_PYTHON=" + selected, "PATH=" + dir, "PATHEXT=.EXE;.CMD"}
	bindings := []ShellCommandBinding{{Command: "python"}}

	if err := ValidateShellCommandBindings(baseEnv, bindings); err != nil {
		t.Fatalf("Python argument text must not look like a shell PATH mutation: %v", err)
	}
	if err := ValidateShellCommandBindings(baseEnv, []ShellCommandBinding{{Command: "python", Unprovable: "command defines a shell alias or function that can override interpreter resolution"}}); err == nil || !strings.Contains(err.Error(), "shell alias or function") {
		t.Fatalf("function override error = %v", err)
	}
	withExportedFunction := append(append([]string(nil), baseEnv...), "BASH_FUNC_python%%=() { echo wrong; }")
	if err := ValidateShellCommandBindings(withExportedFunction, bindings); err == nil || !strings.Contains(err.Error(), "exported shell function") {
		t.Fatalf("exported function error = %v", err)
	}
}

func TestResolver_ResolveProbeTimeout(t *testing.T) {
	t.Parallel()
	resolver := NewResolverWithDependencies(Options{Interpreter: filepath.Join(t.TempDir(), "python.exe"), ProbeTimeout: time.Millisecond}, Dependencies{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
		Probe: func(ctx context.Context, _ string, _ ...string) (ProbeResult, error) {
			<-ctx.Done()
			return ProbeResult{}, ctx.Err()
		},
		Manager: NewManager(t.TempDir()),
	})
	_, err := resolver.Resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Resolve() error = %v, want timeout", err)
	}
}

func TestResolver_ConcurrentResolve(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	managed := writeRuntime(t, root, PinnedVersion, nil, "")
	var probes atomic.Int64
	resolver := NewResolverWithDependencies(Options{Mode: ModeManaged}, Dependencies{
		Probe: func(_ context.Context, _ string, _ ...string) (ProbeResult, error) {
			probes.Add(1)
			return ProbeResult{Executable: managed.Executable, Major: 3, Minor: 12, Patch: 10}, nil
		},
		Manager: fixtureManager(t, root, managed.Root),
	})

	const count = 32
	done := make(chan error, count)
	for range count {
		go func() {
			result, err := resolver.Resolve(context.Background())
			if err == nil && result.Env["KWU_PYTHON"] != managed.Executable {
				err = fmt.Errorf("KWU_PYTHON = %q", result.Env["KWU_PYTHON"])
			}
			done <- err
		}()
	}
	for range count {
		if err := <-done; err != nil {
			t.Errorf("concurrent Resolve() error = %v", err)
		}
	}
	if got := probes.Load(); got != count {
		t.Fatalf("probe count = %d, want %d", got, count)
	}
}

func TestProbeArgumentsDisableSiteInitialization(t *testing.T) {
	t.Parallel()
	got := probeArguments([]string{"-3"}, "probe-script")
	want := []string{"-3", "-I", "-S", "-c", "probe-script"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("probeArguments() = %q, want %q; -S must prevent residual sitecustomize.py execution", got, want)
	}
}
