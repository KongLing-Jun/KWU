package pythonruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"kwu/internal/tool"
)

const defaultProbeTimeout = 5 * time.Second

const (
	ModeAuto    = "auto"
	ModeManaged = "managed"
)

type Source string

const (
	SourceWorkspace Source = "workspace"
	SourceExplicit  Source = "explicit"
	SourceSystem    Source = "system"
	SourceManaged   Source = "managed"
)

// Options controls Python resolution for one task.
type Options struct {
	Mode          string
	WorkspaceRoot string
	Interpreter   string
	ManagedRoot   string
	ProbeTimeout  time.Duration
}

// Result contains a validated interpreter and task-scoped environment values.
type Result struct {
	Executable string
	Version    string
	Source     Source
	Env        map[string]string
}

// ProbeResult is the normalized identity reported by a running interpreter.
type ProbeResult struct {
	Executable string
	Major      int
	Minor      int
	Patch      int
}

// Dependencies exposes deterministic seams for resolver tests.
type Dependencies struct {
	LookPath func(string) (string, error)
	Probe    func(context.Context, string, ...string) (ProbeResult, error)
	Manager  *Manager
}

// Resolver validates candidates without changing process-global environment.
// It is safe for concurrent use when injected dependencies are concurrency-safe.
type Resolver struct {
	options  Options
	lookPath func(string) (string, error)
	probe    func(context.Context, string, ...string) (ProbeResult, error)
	manager  *Manager
}

func NewResolver(options Options) *Resolver {
	return NewResolverWithDependencies(options, Dependencies{})
}

func NewResolverWithDependencies(options Options, dependencies Dependencies) *Resolver {
	options.Mode = strings.ToLower(strings.TrimSpace(options.Mode))
	if options.Mode == "" {
		options.Mode = ModeAuto
	}
	if options.ProbeTimeout <= 0 {
		options.ProbeTimeout = defaultProbeTimeout
	}
	if dependencies.LookPath == nil {
		dependencies.LookPath = exec.LookPath
	}
	if dependencies.Probe == nil {
		dependencies.Probe = probeInterpreter
	}
	if dependencies.Manager == nil {
		dependencies.Manager = NewManager(options.ManagedRoot)
	}
	return &Resolver{
		options:  options,
		lookPath: dependencies.LookPath,
		probe:    dependencies.Probe,
		manager:  dependencies.Manager,
	}
}

type candidate struct {
	executable string
	args       []string
	source     Source
	managed    *ManagedRuntime
}

// Resolve returns the first valid candidate according to the configured mode.
func (r *Resolver) Resolve(ctx context.Context) (Result, error) {
	return r.resolve(ctx, nil, "")
}

func (r *Resolver) resolve(ctx context.Context, commands []string, pathExt string) (Result, error) {
	switch r.options.Mode {
	case ModeAuto:
		return r.resolveAuto(ctx, commands, pathExt)
	case ModeManaged:
		return r.resolveManaged(ctx, commands, pathExt)
	default:
		return Result{}, fmt.Errorf("unsupported Python resolution mode %q", r.options.Mode)
	}
}

func (r *Resolver) resolveManaged(ctx context.Context, commands []string, pathExt string) (Result, error) {
	managed, err := r.manager.Exact()
	if err != nil {
		return Result{}, fmt.Errorf("managed Python %s is required: %w", PinnedVersion, err)
	}
	result, err := r.validate(ctx, candidate{executable: managed.Executable, source: SourceManaged, managed: &managed})
	if err != nil {
		return Result{}, fmt.Errorf("managed Python %s is invalid: %w", PinnedVersion, err)
	}
	if result.Version != PinnedVersion {
		return Result{}, fmt.Errorf("managed Python reported version %s, want %s", result.Version, PinnedVersion)
	}
	if err := validateRequestedCommands(result, commands, pathExt); err != nil {
		return Result{}, fmt.Errorf("managed Python %s is invalid: %w", PinnedVersion, err)
	}
	return result, nil
}

func (r *Resolver) resolveAuto(ctx context.Context, commands []string, pathExt string) (Result, error) {
	candidates := r.userCandidates()
	var failures []error
	for _, item := range candidates {
		result, err := r.validate(ctx, item)
		if err == nil {
			err = validateRequestedCommands(result, commands, pathExt)
		}
		if err == nil {
			return result, nil
		}
		failures = append(failures, fmt.Errorf("%s candidate %q: %w", item.source, item.executable, err))
	}
	managed, managedErr := r.manager.Exact()
	if managedErr == nil {
		result, err := r.validate(ctx, candidate{executable: managed.Executable, source: SourceManaged, managed: &managed})
		if err == nil {
			err = validateRequestedCommands(result, commands, pathExt)
		}
		if err == nil {
			return result, nil
		}
		failures = append(failures, fmt.Errorf("%s candidate %q: %w", SourceManaged, managed.Executable, err))
	}
	if managedErr != nil {
		failures = append(failures, managedErr)
	}
	if len(failures) == 0 {
		return Result{}, errors.New("no Python candidates were found")
	}
	return Result{}, fmt.Errorf("no usable Python interpreter: %w", errors.Join(failures...))
}

// ResolveCommandEnv implements the task command environment provider contract.
func (r *Resolver) ResolveCommandEnv(ctx context.Context) (map[string]string, error) {
	result, err := r.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	return result.Env, nil
}

// ResolveCommandEnvForRequest selects the first candidate that can bind every
// literal command requested by the task's first Python execution.
func (r *Resolver) ResolveCommandEnvForRequest(ctx context.Context, request tool.CommandEnvRequest) (map[string]string, error) {
	result, err := r.resolve(ctx, request.Commands, request.PathExt)
	if err != nil {
		return nil, err
	}
	return result.Env, nil
}

func validateRequestedCommands(result Result, commands []string, pathExt string) error {
	seen := make(map[string]bool, len(commands))
	for _, command := range commands {
		key := command
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := ValidateCommandBinding(command, result.Executable, result.Env["PATH"], pathExt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) userCandidates() []candidate {
	result := make([]candidate, 0, 5)
	if workspace := strings.TrimSpace(r.options.WorkspaceRoot); workspace != "" {
		venv := filepath.Join(workspace, ".venv", "bin", "python")
		if runtime.GOOS == "windows" {
			venv = filepath.Join(workspace, ".venv", "Scripts", "python.exe")
		}
		result = append(result, candidate{executable: venv, source: SourceWorkspace})
	}
	if explicit := strings.TrimSpace(r.options.Interpreter); explicit != "" {
		result = append(result, candidate{executable: explicit, source: SourceExplicit})
	}
	for _, command := range []string{"py", "python3", "python"} {
		path, err := r.lookPath(command)
		if err != nil {
			continue
		}
		item := candidate{executable: path, source: SourceSystem}
		if command == "py" {
			item.args = []string{"-3"}
		}
		result = append(result, item)
	}
	return result
}

func (r *Resolver) validate(parent context.Context, item candidate) (Result, error) {
	if isStoreShim(item.executable) {
		return Result{}, errors.New("Microsoft Store app execution alias is not an interpreter")
	}
	probe, err := r.runProbe(parent, item)
	if err != nil {
		return Result{}, err
	}
	executable, version, err := validateProbeIdentity(probe)
	if err != nil {
		return Result{}, err
	}
	if err := validateManagedProbe(item.managed, executable, version); err != nil {
		return Result{}, err
	}
	env := map[string]string{
		"PATH":       filepath.Dir(executable),
		"KWU_PYTHON": executable,
	}
	if item.managed != nil {
		env["PYTHONHOME"] = item.managed.Root
		env["PYTHONNOUSERSITE"] = "1"
		env["PYTHONPATH"] = ""
		env["PYTHONUTF8"] = "1"
	}
	return Result{Executable: executable, Version: version, Source: item.source, Env: env}, nil
}

func (r *Resolver) runProbe(parent context.Context, item candidate) (ProbeResult, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, r.options.ProbeTimeout)
	defer cancel()
	probe, err := r.probe(ctx, item.executable, item.args...)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ProbeResult{}, errors.New("interpreter probe timed out")
		}
		return ProbeResult{}, fmt.Errorf("interpreter probe failed: %w", err)
	}
	return probe, nil
}

func validateProbeIdentity(probe ProbeResult) (string, string, error) {
	if !filepath.IsAbs(probe.Executable) {
		return "", "", fmt.Errorf("interpreter reported non-absolute sys.executable %q", probe.Executable)
	}
	if isStoreShim(probe.Executable) {
		return "", "", errors.New("interpreter resolved to a Microsoft Store app execution alias")
	}
	version := fmt.Sprintf("%d.%d.%d", probe.Major, probe.Minor, probe.Patch)
	if probe.Major != 3 || probe.Minor < 10 {
		return "", "", fmt.Errorf("Python %s is unsupported; Python 3.10 or newer in the 3.x series is required", version)
	}
	executable := filepath.Clean(probe.Executable)
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("interpreter reported unusable sys.executable %q", executable)
	}
	return executable, version, nil
}

func validateManagedProbe(managed *ManagedRuntime, executable, version string) error {
	if managed == nil {
		return nil
	}
	if !sameExecutablePath(executable, managed.Executable) {
		return fmt.Errorf("managed interpreter reported sys.executable %q, want %q", executable, managed.Executable)
	}
	if version != managed.Manifest.Version {
		return fmt.Errorf("managed interpreter reported version %s, want %s", version, managed.Manifest.Version)
	}
	return nil
}

func sameExecutablePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// ShellCommandBinding is the shell scanner's proof input for one Python call.
type ShellCommandBinding struct {
	Command    string
	Wrapper    string
	Unprovable string
}

// ValidateShellCommandBindings validates every Python call against selected.
func ValidateShellCommandBindings(env []string, bindings []ShellCommandBinding) error {
	if len(bindings) == 0 {
		return nil
	}
	selected, _ := bindingEnvValue(env, "KWU_PYTHON")
	if strings.TrimSpace(selected) == "" {
		return fmt.Errorf("selected Python path %q cannot bind alias %q: KWU_PYTHON is missing", selected, bindings[0].Command)
	}
	selected = filepath.Clean(selected)
	if override := bindingEnvOverride(env); override != "" {
		return fmt.Errorf("selected Python path %q cannot bind alias %q: child environment contains %s", selected, bindings[0].Command, override)
	}
	pathValue, _ := bindingEnvValue(env, "PATH")
	pathext, _ := bindingEnvValue(env, "PATHEXT")
	for _, binding := range bindings {
		if binding.Unprovable != "" {
			via := ""
			if binding.Wrapper != "" {
				via = " through " + binding.Wrapper
			}
			return fmt.Errorf("selected Python path %q cannot bind alias %q%s: %s", selected, binding.Command, via, binding.Unprovable)
		}
		if !wrapperPreservesResolution(binding.Wrapper) {
			return fmt.Errorf("selected Python path %q cannot bind alias %q through %s: launcher resolution cannot be proven exact", selected, binding.Command, binding.Wrapper)
		}
		if err := ValidateCommandBinding(binding.Command, selected, pathValue, pathext); err != nil {
			return err
		}
	}
	return nil
}

func bindingEnvValue(env []string, key string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		name, value, ok := strings.Cut(env[i], "=")
		if ok && ((runtime.GOOS == "windows" && strings.EqualFold(name, key)) || name == key) {
			return value, true
		}
	}
	return "", false
}

func bindingEnvOverride(env []string) string {
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || value == "" {
			continue
		}
		lower := strings.ToLower(name)
		if lower == "bash_env" || lower == "env" {
			return name + " startup hook"
		}
		for _, alias := range []string{"py", "py.exe", "python", "python.exe", "python3", "python3.exe"} {
			if strings.HasPrefix(lower, "bash_func_"+alias+"%%") {
				return name + " exported shell function"
			}
		}
	}
	return ""
}

func wrapperPreservesResolution(wrapper string) bool {
	for _, item := range strings.Split(wrapper, ">") {
		switch item {
		case "", "transparent", "env", "timeout":
		default:
			return false
		}
	}
	return true
}

// ValidateCommandBinding proves that command resolves to selected using only
// the final child environment. It never reads or mutates process-global PATH.
func ValidateCommandBinding(command, selected, pathValue, pathext string) error {
	selected = filepath.Clean(selected)
	base := strings.ToLower(filepath.Base(command))
	if base == "py" || base == "py.exe" {
		return fmt.Errorf("selected Python path %q cannot bind alias %q: the py launcher may select a different interpreter", selected, command)
	}
	resolved, err := resolveCommandBinding(command, pathValue, pathext)
	if err != nil {
		return fmt.Errorf("selected Python path %q cannot bind alias %q: %w", selected, command, err)
	}
	if !sameExecutableFile(resolved, selected) {
		if err := validatePinnedManagedAlias(selected, resolved); err != nil {
			return fmt.Errorf("selected Python path %q does not match alias %q resolved to %q and is not a trusted managed alias: %w", selected, command, resolved, err)
		}
	}
	return nil
}

func resolveCommandBinding(command, pathValue, pathext string) (string, error) {
	if strings.ContainsAny(command, `/\`) {
		if !filepath.IsAbs(command) {
			return "", fmt.Errorf("explicit path %q is not absolute", command)
		}
		if !isCommandExecutable(command) {
			return "", fmt.Errorf("explicit path %q is not an executable regular file", command)
		}
		return filepath.Clean(command), nil
	}
	for _, dir := range filepath.SplitList(pathValue) {
		if strings.TrimSpace(dir) == "" || !filepath.IsAbs(dir) {
			return "", fmt.Errorf("alias %q has an ambiguous PATH entry %q", command, dir)
		}
		for _, name := range commandExecutableNames(command, pathext) {
			candidate := filepath.Join(dir, name)
			if isCommandExecutable(candidate) {
				return filepath.Clean(candidate), nil
			}
		}
	}
	return "", fmt.Errorf("alias %q was not found in the final child PATH", command)
}

func commandExecutableNames(command, pathext string) []string {
	if runtime.GOOS != "windows" || filepath.Ext(command) != "" {
		return []string{command}
	}
	if strings.TrimSpace(pathext) == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	names := []string{command}
	seen := map[string]bool{strings.ToLower(command): true}
	for _, ext := range strings.Split(pathext, ";") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if ext[0] != '.' {
			ext = "." + ext
		}
		name := command + ext
		key := strings.ToLower(name)
		if !seen[key] {
			seen[key] = true
			names = append(names, name)
		}
	}
	return names
}

func isCommandExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

func sameExecutableFile(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return os.SameFile(aInfo, bInfo)
}

func probeInterpreter(ctx context.Context, executable string, prefixArgs ...string) (ProbeResult, error) {
	const script = `import json,sys; print(json.dumps({"executable":sys.executable,"version":[sys.version_info.major,sys.version_info.minor,sys.version_info.micro]}))`
	// Isolated mode ignores PYTHON* variables, the user site, and the current
	// working directory. Disabling site initialization also prevents a residual
	// sitecustomize.py in the interpreter root from running during the probe.
	args := probeArguments(prefixArgs, script)
	output, err := exec.CommandContext(ctx, executable, args...).Output()
	if err != nil {
		return ProbeResult{}, err
	}
	var response struct {
		Executable string `json:"executable"`
		Version    []int  `json:"version"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return ProbeResult{}, fmt.Errorf("decode probe output: %w", err)
	}
	if response.Executable == "" || len(response.Version) != 3 {
		return ProbeResult{}, errors.New("probe output is missing executable or version")
	}
	return ProbeResult{
		Executable: response.Executable,
		Major:      response.Version[0],
		Minor:      response.Version[1],
		Patch:      response.Version[2],
	}, nil
}

func probeArguments(prefixArgs []string, script string) []string {
	return append(append([]string{}, prefixArgs...), "-I", "-S", "-c", script)
}

func isStoreShim(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(normalized, "/microsoft/windowsapps/") ||
		strings.Contains(normalized, "/appdata/local/microsoft/windowsapps/")
}
