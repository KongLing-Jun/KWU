package tool

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type commandEnvProviderFunc func(context.Context) (map[string]string, error)

func (f commandEnvProviderFunc) ResolveCommandEnv(ctx context.Context) (map[string]string, error) {
	return f(ctx)
}

type requestCommandEnvProvider struct {
	calls    atomic.Int32
	requests chan CommandEnvRequest
	entered  chan struct{}
	release  chan struct{}
	env      map[string]string
}

func (p *requestCommandEnvProvider) ResolveCommandEnv(context.Context) (map[string]string, error) {
	return nil, errors.New("legacy provider method must not be called")
}

func (p *requestCommandEnvProvider) ResolveCommandEnvForRequest(_ context.Context, request CommandEnvRequest) (map[string]string, error) {
	p.calls.Add(1)
	p.requests <- request
	if p.entered != nil {
		close(p.entered)
	}
	if p.release != nil {
		<-p.release
	}
	return p.env, nil
}

func testCommandEnv(root string) map[string]string {
	return map[string]string{
		CommandEnvPATH:         filepath.Join(root, "node") + string(filepath.ListSeparator) + filepath.Join(root, "cli"),
		"CONNECTOR_CONFIG_DIR": filepath.Join(root, "config"), "CONNECTOR_NOTICE": "1",
	}
}

func TestValidateCommandEnvClonesGenericManagedValues(t *testing.T) {
	input := testCommandEnv(t.TempDir())
	got, err := ValidateCommandEnv(input)
	if err != nil {
		t.Fatal(err)
	}
	input["CONNECTOR_NOTICE"] = "changed"
	if got["CONNECTOR_NOTICE"] != "1" {
		t.Fatal("validated command environment retained caller map")
	}
}

func TestValidateCommandEnvRejectsInvalidNamesValuesAndPATH(t *testing.T) {
	tests := []func(map[string]string){
		func(env map[string]string) { env["BAD-NAME"] = "value" },
		func(env map[string]string) { env[CommandEnvPATH] = "" },
		func(env map[string]string) { env[CommandEnvPATH] = "relative" },
		func(env map[string]string) { env["CONNECTOR_CONFIG_DIR"] += "\nsecret" },
	}
	for i, mutate := range tests {
		env := testCommandEnv(t.TempDir())
		mutate(env)
		if _, err := ValidateCommandEnv(env); err == nil {
			t.Fatalf("case %d: expected rejection", i)
		}
	}
}

func TestValidateCommandEnvAllowsEmptyValueAsUnset(t *testing.T) {
	got, err := ValidateCommandEnv(map[string]string{"PYTHONPATH": ""})
	if err != nil {
		t.Fatal(err)
	}
	if value, exists := got["PYTHONPATH"]; !exists || value != "" {
		t.Fatalf("PYTHONPATH = %q, exists = %v; want explicit unset", value, exists)
	}
}

func TestValidateCommandEnvRejectsCaseInsensitiveDuplicatesOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows environment keys are case-insensitive")
	}
	root := t.TempDir()
	if _, err := ValidateCommandEnv(map[string]string{
		"PATH": filepath.Join(root, "one"),
		"Path": filepath.Join(root, "two"),
	}); err == nil {
		t.Fatal("case-insensitive duplicate PATH was accepted")
	}
	got, err := ValidateCommandEnv(map[string]string{"PythonPath": ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got["PYTHONPATH"]; !exists {
		t.Fatalf("Windows key was not canonicalized: %#v", got)
	}
}

func TestCommandEnvContextReturnsClone(t *testing.T) {
	env, err := ValidateCommandEnv(testCommandEnv(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithCommandEnv(context.Background(), env)
	got := CommandEnvFromContext(ctx)
	got["CONNECTOR_NOTICE"] = "changed"
	if again := CommandEnvFromContext(ctx); again["CONNECTOR_NOTICE"] != "1" {
		t.Fatal("context command environment was mutable")
	}
}

func TestResolveCommandEnvPrependsProviderPathAndPreservesStaticOverrides(t *testing.T) {
	root := t.TempDir()
	pythonDir := filepath.Join(root, "python")
	connectorDir := filepath.Join(root, "connector")
	provider := commandEnvProviderFunc(func(context.Context) (map[string]string, error) {
		return map[string]string{
			CommandEnvPATH: pythonDir,
			"KWU_PYTHON":   filepath.Join(pythonDir, "python.exe"),
			"PYTHONPATH":   "",
			"SHARED_KEY":   "provider",
		}, nil
	})
	ctx := WithCommandEnvProvider(context.Background(), provider)
	ctx = WithCommandEnv(ctx, map[string]string{
		CommandEnvPATH: connectorDir,
		"PYTHONPATH":   filepath.Join(root, "untrusted"),
		"SHARED_KEY":   "static",
	})
	got, err := ResolveCommandEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if parts := filepath.SplitList(got[CommandEnvPATH]); len(parts) != 2 || parts[0] != pythonDir || parts[1] != connectorDir {
		t.Fatalf("PATH = %q, want provider then static", got[CommandEnvPATH])
	}
	if got["SHARED_KEY"] != "static" {
		t.Fatalf("SHARED_KEY = %q, want static", got["SHARED_KEY"])
	}
	if value, exists := got["PYTHONPATH"]; !exists || value != "" {
		t.Fatalf("PYTHONPATH = %q, exists = %v; provider unset must win", value, exists)
	}
}

func TestCommandEnvProviderSnapshotResolvesOnceAcrossConcurrentCallers(t *testing.T) {
	root := t.TempDir()
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	provider := commandEnvProviderFunc(func(context.Context) (map[string]string, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return map[string]string{CommandEnvPATH: filepath.Join(root, "python")}, nil
	})
	ctx := WithCommandEnvProvider(context.Background(), provider)
	const callers = 32
	start := make(chan struct{})
	results := make(chan map[string]string, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			env, err := ResolveCommandEnv(ctx)
			results <- env
			errs <- err
		}()
	}
	close(start)
	<-entered
	close(release)
	wg.Wait()
	close(results)
	close(errs)
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for env := range results {
		env[CommandEnvPATH] = filepath.Join(root, "mutated")
	}
	again, err := ResolveCommandEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again[CommandEnvPATH] != filepath.Join(root, "python") {
		t.Fatal("provider snapshot leaked a mutable result map")
	}
}

func TestCommandEnvProviderUsesTaskContextAfterFirstCallerCancels(t *testing.T) {
	root := t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	provider := commandEnvProviderFunc(func(ctx context.Context) (map[string]string, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return map[string]string{CommandEnvPATH: filepath.Join(root, "python")}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	taskCtx, cancelTask := context.WithCancel(context.Background())
	defer cancelTask()
	taskCtx = WithCommandEnvProvider(taskCtx, provider)
	callerCtx, cancelCaller := context.WithCancel(taskCtx)
	first := make(chan error, 1)
	go func() {
		_, err := ResolveCommandEnv(callerCtx)
		first <- err
	}()
	<-entered
	cancelCaller()
	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("first caller error = %v, want context canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled caller remained blocked on provider")
	}

	second := make(chan map[string]string, 1)
	secondErr := make(chan error, 1)
	go func() {
		env, err := ResolveCommandEnv(taskCtx)
		second <- env
		secondErr <- err
	}()
	close(release)
	if err := <-secondErr; err != nil {
		t.Fatal(err)
	}
	if env := <-second; env[CommandEnvPATH] != filepath.Join(root, "python") {
		t.Fatalf("surviving caller environment = %#v", env)
	}
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want one task-scoped resolution", calls.Load())
	}
}

func TestCommandEnvRequestProviderFreezesFirstConcurrentRequest(t *testing.T) {
	root := t.TempDir()
	provider := &requestCommandEnvProvider{
		requests: make(chan CommandEnvRequest, 2),
		entered:  make(chan struct{}),
		release:  make(chan struct{}),
		env:      map[string]string{CommandEnvPATH: filepath.Join(root, "python")},
	}
	ctx := WithCommandEnvProvider(context.Background(), provider)
	first := make(chan error, 1)
	commands := []string{"python"}
	go func() {
		_, err := ResolveCommandEnvForExecutionRequest(ctx, true, CommandEnvRequest{Commands: commands})
		first <- err
	}()
	<-provider.entered
	commands[0] = "mutated"

	second := make(chan error, 1)
	go func() {
		_, err := ResolveCommandEnvForExecutionRequest(ctx, true, CommandEnvRequest{Commands: []string{"python3"}})
		second <- err
	}()
	close(provider.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
	request := <-provider.requests
	if len(request.Commands) != 1 || request.Commands[0] != "python" {
		t.Fatalf("frozen request = %#v, want deep-cloned first request", request)
	}
}

func TestCommandEnvRequestRejectsControlCharacters(t *testing.T) {
	provider := &requestCommandEnvProvider{
		requests: make(chan CommandEnvRequest, 1),
		env:      map[string]string{CommandEnvPATH: t.TempDir()},
	}
	ctx := WithCommandEnvProvider(context.Background(), provider)
	_, err := ResolveCommandEnvForExecutionRequest(ctx, true, CommandEnvRequest{Commands: []string{"python\nmalicious"}})
	if err == nil || !strings.Contains(err.Error(), "invalid command environment request") {
		t.Fatalf("request error = %v, want control-character rejection", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatal("invalid request reached provider")
	}
}

func TestCommandEnvProviderForTaskReusesOnlyMatchingTaskIdentity(t *testing.T) {
	root := t.TempDir()
	var calls atomic.Int32
	provider := commandEnvProviderFunc(func(context.Context) (map[string]string, error) {
		calls.Add(1)
		return map[string]string{CommandEnvPATH: filepath.Join(root, "python")}, nil
	})
	first := WithCommandEnvProviderForTask(context.Background(), "task-a", provider)
	if _, err := ResolveCommandEnv(first); err != nil {
		t.Fatal(err)
	}
	derived := WithCommandEnvProviderForTask(context.WithValue(first, struct{}{}, "derived"), "task-a", provider)
	if _, err := ResolveCommandEnv(derived); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("same task provider calls = %d, want 1", got)
	}

	second := WithCommandEnvProviderForTask(derived, "task-b", provider)
	if _, err := ResolveCommandEnv(second); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("new task provider calls = %d, want fresh snapshot", got)
	}
	legacyRebind := WithCommandEnvProvider(second, provider)
	if _, err := ResolveCommandEnv(legacyRebind); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("legacy rebind provider calls = %d, want inherited snapshot compatibility", got)
	}
}
