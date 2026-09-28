package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

func testTaskContext() context.Context {
	return WithParentSession(context.Background(), "parent-session")
}

// TestTaskToolReturnsSubAgentFinalAnswer runs a task against a mock provider
// that emits a single text turn, and verifies the tool returns that text with a
// transcript reference — sub-agent intermediate state isn't supposed to leak.
func TestTaskToolReturnsSubAgentFinalAnswer(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "found 3 callers of Foo"},
		{Type: provider.ChunkDone},
	}}
	parentReg := tool.NewRegistry()
	task := newTestTaskTool(t, sub, parentReg, "test-sys-prompt", "", "", nil)

	out, err := task.Execute(testTaskContext(), []byte(`{"prompt":"find callers of Foo"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	_ = subagentRefFromOutput(t, out)
	if !strings.Contains(out, "found 3 callers of Foo") {
		t.Errorf("got %q, want sub-agent final answer", out)
	}
	if !strings.Contains(out, "To continue this same subagent transcript in a later call, pass this ref as `continue_from`. Start a fresh subagent when the next task is independent.") {
		t.Errorf("got %q, want continuation guidance", out)
	}

	// The sub-agent must have received the prompt as its user message and
	// the configured system prompt at the top — proving the session was
	// fresh, not the parent's.
	if sys := sub.lastReq.Messages[0]; sys.Role != provider.RoleSystem || sys.Content != "test-sys-prompt" {
		t.Errorf("first message = %+v, want system 'test-sys-prompt'", sys)
	}
	if got := lastUser(sub.lastReq); got != "find callers of Foo" {
		t.Errorf("sub-agent user = %q, want the prompt verbatim", got)
	}
}

func TestTaskToolDefaultPromptExcludesParentVisibleWorkUpdates(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "done"},
		{Type: provider.ChunkDone},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "", "", "", nil)

	if _, err := task.Execute(testTaskContext(), []byte(`{"prompt":"focused child task"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	system := sub.lastReq.Messages[0]
	if system.Role != provider.RoleSystem || system.Content != DefaultTaskSystemPrompt {
		t.Fatalf("child system prompt = %+v, want DefaultTaskSystemPrompt", system)
	}
	if strings.Contains(system.Content, "Visible work updates:") {
		t.Fatalf("child system prompt inherited parent visible work update policy:\n%s", system.Content)
	}
}

func TestTaskToolCancelDuringStuckProviderReturnsPromptly(t *testing.T) {
	task := newTestTaskTool(t, stuckStreamProvider{}, tool.NewRegistry(), "sys", "", "", nil)

	ctx, cancel := context.WithCancel(testTaskContext())
	done := make(chan error, 1)
	go func() {
		_, err := task.Execute(ctx, []byte(`{"prompt":"wait on stuck provider"}`))
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Execute returned nil after context cancellation")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute error = %v, want context cancellation", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TaskTool.Execute did not return promptly after cancellation")
	}
}

func TestTaskToolSchemaExposesOnlyContinueFromForPersistence(t *testing.T) {
	task := NewTaskTool(&mockProvider{name: "sub"}, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil)
	schema := string(task.Schema())
	if !strings.Contains(schema, `"continue_from"`) {
		t.Fatalf("task schema = %s, want continue_from", schema)
	}
	if strings.Contains(schema, "fork_from") {
		t.Fatalf("task schema = %s, want no fork_from", schema)
	}
	if strings.Contains(schema, "run_in_background") {
		t.Fatalf("task schema = %s, want unified asynchronous semantics without run_in_background", schema)
	}
	for _, field := range []string{`"independent"`, `"depends_on"`} {
		if !strings.Contains(schema, field) {
			t.Fatalf("task schema = %s, want %s", schema, field)
		}
	}
}

func TestWaitTaskWaitsWithoutModelVisiblePollingTimeout(t *testing.T) {
	wait := NewWaitTaskTool(nil)
	base := 30 * time.Second
	if schema := string(wait.Schema()); strings.Contains(schema, "timeout_seconds") {
		t.Fatalf("wait_task schema = %s, want no model-visible polling timeout", schema)
	}
	for _, args := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"timeout_seconds":45}`)} {
		if got := wait.toolTimeoutBudget(base, args); got != tool.NoTimeoutBudget {
			t.Fatalf("wait budget for %s = %v, want NoTimeoutBudget", args, got)
		}
	}
}

func TestTaskToolCarriesDedicatedSubagentAndCompactionDispatchers(t *testing.T) {
	base := &mockProvider{name: "sub"}
	main := &dispatcherTestStream{}
	compaction := &dispatcherTestStream{}
	task := NewTaskTool(base, nil, tool.NewRegistry(), 20, 100, 2, 0.5, 0.8, 0.9, 0, "", "sys", nil, 0, "", "", nil).
		WithStreamDispatcherResolver(func(modelRef, effort string, got provider.Provider) (SubagentStreamDispatchers, error) {
			if modelRef != "model/sub" || effort != "high" || got != base {
				t.Fatalf("resolver args = %q/%q/%T", modelRef, effort, got)
			}
			return SubagentStreamDispatchers{Main: main, Compaction: compaction}, nil
		})

	runtime, err := task.resolveSubSessionRuntime("model/sub", "high")
	if err != nil {
		t.Fatal(err)
	}
	for name, opts := range map[string]Options{
		"foreground": task.subagentOptions(context.Background(), 5, runtime),
		"parallel":   parallelSubagentOptions(task, 5, runtime),
	} {
		if opts.StreamDispatcher != main || opts.DerivedStreamDispatcher != compaction {
			t.Fatalf("%s dispatchers = %T/%T", name, opts.StreamDispatcher, opts.DerivedStreamDispatcher)
		}
	}
}

func TestTaskToolBackgroundPolicyRejectsBeforeTranscriptProviderOrJob(t *testing.T) {
	policyErr := errors.New("background disabled")
	storeRoot := t.TempDir()
	store := NewSubagentStore(storeRoot)
	base := &mockProvider{name: "base"}
	providerResolutions := 0
	policyCalls := 0
	task := NewTaskTool(base, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0, "", "sys", nil, 0, "", "", func(string, string) (provider.Provider, *provider.Pricing, int, error) {
		providerResolutions++
		return base, nil, 0, nil
	}).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort").
		WithBackgroundPolicy(func(context.Context) error {
			policyCalls++
			return policyErr
		})

	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)
	_, err := task.Execute(ctx, []byte(`{"prompt":"background task","model":"other","run_in_background":true}`))
	if !errors.Is(err, policyErr) {
		t.Fatalf("Execute error = %v, want policy error", err)
	}
	if policyCalls != 1 || providerResolutions != 0 || len(base.requests) != 0 {
		t.Fatalf("policy/provider/stream calls = %d/%d/%d", policyCalls, providerResolutions, len(base.requests))
	}
	entries, readErr := os.ReadDir(storeRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("transcript side effects = %v", entries)
	}
	if running := jm.RunningForSession("parent-session"); len(running) != 0 {
		t.Fatalf("background jobs = %+v", running)
	}
}

func TestTaskToolTurnScopedAsyncDoesNotUseLegacyBackgroundPolicy(t *testing.T) {
	sub := &gatedTaskProvider{started: make(chan struct{}), release: make(chan struct{})}
	policyCalls := 0
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil).
		WithBackgroundPolicy(func(context.Context) error {
			policyCalls++
			return errors.New("legacy background disabled")
		})
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)
	start, err := executeToolWithTimeout(ctx, "task", time.Second, func(callCtx context.Context) (string, error) {
		return task.Execute(callCtx, []byte(`{"prompt":"turn-scoped task"}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	if policyCalls != 0 || !strings.Contains(start, "Turn status: running") {
		t.Fatalf("policy calls/start = %d/%q", policyCalls, start)
	}
	select {
	case <-sub.started:
	case <-time.After(time.Second):
		t.Fatal("turn-scoped child did not start")
	}
	ref := subagentRefFromOutput(t, start)
	type waitResult struct {
		output string
		err    error
	}
	cancelledCtx, cancelWait := context.WithCancel(ctx)
	cancelled := make(chan waitResult, 1)
	go func() {
		output, waitErr := NewWaitTaskTool(task).Execute(cancelledCtx, []byte(`{"subagent_ref":"`+ref+`"}`))
		cancelled <- waitResult{output: output, err: waitErr}
	}()
	time.Sleep(50 * time.Millisecond)
	cancelWait()
	select {
	case result := <-cancelled:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("cancelled wait_task error = %v, want context.Canceled", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("parent context cancellation did not stop wait_task")
	}
	waited := make(chan waitResult, 1)
	go func() {
		output, waitErr := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`"}`))
		waited <- waitResult{output: output, err: waitErr}
	}()
	select {
	case result := <-waited:
		t.Fatalf("wait_task returned before the child reached terminal state: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}
	close(sub.release)
	var result waitResult
	select {
	case result = <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("wait_task did not wake when the child reached terminal state")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	waitOut := result.output
	if !strings.Contains(waitOut, `"turnStatus":"completed"`) {
		t.Fatalf("wait output = %q, want child to survive outer tool return", waitOut)
	}
}

func TestTaskToolAllowsFourConcurrentPersistentTasksAndRejectsFifth(t *testing.T) {
	release := make(chan struct{})
	sub := &concurrentTaskProvider{started: make(chan struct{}, maxPersistentTasksPerParentTurn), release: release}
	storeRoot := t.TempDir()
	store := NewSubagentStore(storeRoot)
	task := NewTaskTool(sub, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)

	refs := make([]string, 0, maxPersistentTasksPerParentTurn)
	for i := 0; i < maxPersistentTasksPerParentTurn; i++ {
		args := fmt.Sprintf(`{"prompt":"concurrent task %d"}`, i+1)
		if i > 0 {
			args = fmt.Sprintf(`{"prompt":"concurrent task %d","independent":true}`, i+1)
		}
		out, err := task.Execute(ctx, []byte(args))
		if err != nil {
			t.Fatalf("task %d Execute: %v", i+1, err)
		}
		refs = append(refs, subagentRefFromOutput(t, out))
	}
	if got := jm.RunningKindCountForSession("parent-session", "task"); got != maxPersistentTasksPerParentTurn {
		t.Fatalf("running task count = %d, want %d", got, maxPersistentTasksPerParentTurn)
	}
	entriesBefore, err := os.ReadDir(storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	fifth, err := task.Execute(ctx, []byte(`{"prompt":"fifth concurrent task","independent":true}`))
	if err != nil || !strings.Contains(fifth, `"status":"capacity_wait_required"`) {
		t.Fatalf("fifth Execute = %q, %v; want structured capacity result", fifth, err)
	}
	entriesAfter, err := os.ReadDir(storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("fifth task created transcript artifacts: before=%d after=%d", len(entriesBefore), len(entriesAfter))
	}

	for i := 0; i < maxPersistentTasksPerParentTurn; i++ {
		select {
		case <-sub.started:
		case <-time.After(time.Second):
			t.Fatalf("task %d did not start", i+1)
		}
	}
	close(release)
	waitArgs, err := json.Marshal(map[string]any{"subagent_refs": refs, "timeout_seconds": 5})
	if err != nil {
		t.Fatal(err)
	}
	waitOut, err := NewWaitTaskTool(task).Execute(ctx, waitArgs)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(waitOut, `"turnStatus":"completed"`); got != maxPersistentTasksPerParentTurn {
		t.Fatalf("completed receipt count = %d, want %d: %s", got, maxPersistentTasksPerParentTurn, waitOut)
	}
}

func TestPersistentTaskAdmissionRequiresExplicitIndependenceWithoutSideEffects(t *testing.T) {
	release := make(chan struct{})
	sub := &concurrentTaskProvider{started: make(chan struct{}, 2), release: release}
	storeRoot := t.TempDir()
	task := NewTaskTool(sub, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(NewSubagentStore(storeRoot), t.TempDir(), "base-model", "base-effort")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)

	first, err := task.Execute(ctx, []byte(`{"prompt":"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first, "Task run ID: task-") {
		t.Fatalf("start receipt = %q, want task run id", first)
	}
	entriesBefore, err := os.ReadDir(storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := task.Execute(ctx, []byte(`{"prompt":"not declared independent"}`))
	if err != nil || !strings.Contains(rejected, `"status":"dispatch_decision_required"`) {
		t.Fatalf("rejected dispatch = %q, %v", rejected, err)
	}
	entriesAfter, err := os.ReadDir(storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesAfter) != len(entriesBefore) || jm.RunningKindCountForSession("parent-session", "task") != 1 {
		t.Fatalf("rejected dispatch created side effects: entries %d -> %d, running=%d", len(entriesBefore), len(entriesAfter), jm.RunningKindCountForSession("parent-session", "task"))
	}
	second, err := task.Execute(ctx, []byte(`{"prompt":"independent","independent":true}`))
	if err != nil || !strings.Contains(second, "Task run ID: task-") {
		t.Fatalf("independent dispatch = %q, %v", second, err)
	}
	close(release)
}

func TestPersistentTaskDependencyAdmissionStatuses(t *testing.T) {
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)
	release := make(chan struct{})
	running := jm.StartControlledForSession("parent-session", "task", "sa_running", "running", func(context.Context, io.Writer) (string, error) {
		<-release
		return "done", nil
	})

	result, admitted := admitPersistentTaskDispatch(ctx, taskParams{DependsOn: []string{running.ID}})
	if admitted || !strings.Contains(result, `"status":"dispatch_decision_required"`) {
		t.Fatalf("running dependency = %q, admitted=%v", result, admitted)
	}
	result, admitted = admitPersistentTaskDispatch(ctx, taskParams{DependsOn: []string{"task-missing"}})
	if admitted || !strings.Contains(result, `"status":"dependency_not_satisfied"`) {
		t.Fatalf("missing dependency = %q, admitted=%v", result, admitted)
	}
	foreign := jm.StartControlledForSession("foreign-session", "task", "sa_foreign", "foreign", func(context.Context, io.Writer) (string, error) { return "done", nil })
	_ = jm.WaitForSession(context.Background(), "foreign-session", []string{foreign.ID}, 5)
	result, admitted = admitPersistentTaskDispatch(ctx, taskParams{DependsOn: []string{foreign.ID}})
	if admitted || !strings.Contains(result, `"status":"dependency_not_satisfied"`) || strings.Contains(result, "foreign-session") {
		t.Fatalf("foreign dependency = %q, admitted=%v", result, admitted)
	}
	failed := jm.StartControlledForSession("parent-session", "task", "sa_failed", "failed", func(context.Context, io.Writer) (string, error) { return "", errors.New("boom") })
	_ = jm.WaitForSession(context.Background(), "parent-session", []string{failed.ID}, 5)
	result, admitted = admitPersistentTaskDispatch(ctx, taskParams{DependsOn: []string{failed.ID}})
	if admitted || !strings.Contains(result, `"status":"dependency_not_satisfied"`) || strings.Contains(result, "boom") {
		t.Fatalf("failed dependency = %q, admitted=%v", result, admitted)
	}
	close(release)
	_ = jm.WaitForSession(context.Background(), "parent-session", []string{running.ID}, 5)
	result, admitted = admitPersistentTaskDispatch(ctx, taskParams{DependsOn: []string{running.ID}})
	if !admitted || result != "" {
		t.Fatalf("successful dependency = %q, admitted=%v", result, admitted)
	}
}

func TestPersistentTaskContinuationUsesDistinctRunIDsAndReceipts(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "first"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "second"}, {Type: provider.ChunkDone}},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil)
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)
	first, err := task.Execute(ctx, []byte(`{"prompt":"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	ref := subagentRefFromOutput(t, first)
	firstRun := taskRunIDFromOutput(t, first)
	firstWait, err := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`"}`))
	if err != nil || !strings.Contains(firstWait, `"taskRunId":"`+firstRun+`"`) {
		t.Fatalf("first wait = %q, %v", firstWait, err)
	}
	second, err := task.Execute(ctx, []byte(`{"prompt":"second","continue_from":"`+ref+`","depends_on":["`+firstRun+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	secondRun := taskRunIDFromOutput(t, second)
	if secondRun == firstRun || subagentRefFromOutput(t, second) != ref {
		t.Fatalf("continuation identities: ref=%q run1=%q run2=%q output=%q", ref, firstRun, secondRun, second)
	}
	secondWait, err := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`"}`))
	if err != nil || !strings.Contains(secondWait, `"taskRunId":"`+secondRun+`"`) || !strings.Contains(secondWait, `"dependsOn":["`+firstRun+`"]`) {
		t.Fatalf("second wait = %q, %v", secondWait, err)
	}
}

func TestRunningPersistentTaskBlocksParentFinalReadiness(t *testing.T) {
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx, cancel := context.WithCancel(jobs.WithSession(context.Background(), "parent-session"))
	job := jm.StartForTurn(ctx, "parent-session", "task", "child", func(ctx context.Context, _ io.Writer) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	a := &Agent{jobs: jm}
	if got := a.runningTaskFinalReadiness(ctx); !strings.Contains(got, "wait_task") {
		t.Fatalf("running task readiness = %q", got)
	}
	cancel()
	_ = jm.WaitForSession(context.Background(), "parent-session", []string{job.ID}, 5)
	if got := a.runningTaskFinalReadiness(ctx); got != "" {
		t.Fatalf("terminal task readiness = %q, want empty", got)
	}
}

type taskContextMarkerKey struct{}

type gatedTaskProvider struct {
	started chan struct{}
	release chan struct{}
}

type concurrentTaskProvider struct {
	started chan struct{}
	release <-chan struct{}
}

func (*concurrentTaskProvider) Name() string { return "concurrent-subagent" }

func (p *concurrentTaskProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	go func() {
		defer close(ch)
		p.started <- struct{}{}
		select {
		case <-p.release:
			ch <- provider.Chunk{Type: provider.ChunkText, Text: "concurrent answer"}
			ch <- provider.Chunk{Type: provider.ChunkDone}
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

func (*gatedTaskProvider) Name() string { return "gated-subagent" }

func (p *gatedTaskProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	close(p.started)
	go func() {
		defer close(ch)
		select {
		case <-p.release:
			ch <- provider.Chunk{Type: provider.ChunkText, Text: "done"}
			ch <- provider.Chunk{Type: provider.ChunkDone}
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

type taskContextCapturingDispatcher struct {
	seen []string
}

func (d *taskContextCapturingDispatcher) Stream(ctx context.Context, _ provider.Provider, _ provider.Request) (<-chan provider.Chunk, error) {
	marker, _ := ctx.Value(taskContextMarkerKey{}).(string)
	d.seen = append(d.seen, marker)
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: "subagent answer"}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func TestTaskToolContinuationUsesCurrentTurnContextNotTranscriptContext(t *testing.T) {
	base := &mockProvider{name: "must-not-run"}
	dispatcher := &taskContextCapturingDispatcher{}
	task := NewTaskTool(base, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(NewSubagentStore(t.TempDir()), t.TempDir(), "base-model", "base-effort").
		WithStreamDispatcherResolver(func(string, string, provider.Provider) (SubagentStreamDispatchers, error) {
			return SubagentStreamDispatchers{Main: dispatcher}, nil
		})
	firstCtx := context.WithValue(testTaskContext(), taskContextMarkerKey{}, "new-parent-task-1")
	first, err := task.Execute(firstCtx, []byte(`{"prompt":"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	ref := subagentRefFromOutput(t, first)
	secondCtx := context.WithValue(testTaskContext(), taskContextMarkerKey{}, "new-parent-task-2")
	if _, err := task.Execute(secondCtx, []byte(`{"prompt":"continue","continue_from":"`+ref+`"}`)); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.seen) != 2 || dispatcher.seen[0] != "new-parent-task-1" || dispatcher.seen[1] != "new-parent-task-2" {
		t.Fatalf("subagent contexts = %#v", dispatcher.seen)
	}
	if len(base.requests) != 0 {
		t.Fatalf("base provider calls = %d", len(base.requests))
	}
}

func TestParallelTasksSchemaDoesNotExposePersistentContinuation(t *testing.T) {
	task := NewTaskTool(&mockProvider{name: "sub"}, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil)
	parallel := NewParallelTasksTool(task, tool.NewRegistry())
	schema := string(parallel.Schema())
	if strings.Contains(schema, "continue_from") || strings.Contains(schema, "fork_from") {
		t.Fatalf("parallel_tasks schema = %s, want no persistent continuation fields", schema)
	}
}

func TestTaskToolInheritsReasoningLanguageFromContext(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "done"},
		{Type: provider.ChunkDone},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil)

	ctx := WithReasoningLanguagePreference(testTaskContext(), "zh")
	if _, err := task.Execute(ctx, []byte(`{"prompt":"inspect auth"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := lastUser(sub.lastReq)
	if !strings.HasPrefix(got, "<reasoning-language>") || !strings.Contains(got, "Simplified Chinese") || !strings.HasSuffix(got, "inspect auth") {
		t.Fatalf("sub-agent user = %q, want reasoning-language-prefixed prompt", got)
	}
}

// TestTaskToolFiltersTools verifies the whitelist behaviour: when the caller
// names a subset of tools, the sub-agent's registry contains exactly that set
// with subagent/skill meta-tools stripped to prevent recursive delegation.
func TestTaskToolFiltersTools(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "ok"},
		{Type: provider.ChunkDone},
	}}
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "read_file", readOnly: true})
	parentReg.Add(fakeTool{name: "write_file", readOnly: false})
	parentReg.Add(fakeTool{name: "bash", readOnly: false})
	task := newTestTaskTool(t, sub, parentReg, "sys", "", "", nil)
	parentReg.Add(task) // simulate the wiring in cli.setup
	parentReg.Add(fakeTool{name: "run_skill", readOnly: false})
	parentReg.Add(fakeTool{name: "read_only_skill", readOnly: true})
	parentReg.Add(fakeTool{name: "research", readOnly: false})

	args := []byte(`{"prompt":"x","tools":["read_file","task","write_file","run_skill","read_only_skill","research"]}`)
	if _, err := task.Execute(testTaskContext(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// The sub-agent's tool schemas should reflect the whitelist minus meta-tools.
	got := map[string]bool{}
	for _, s := range sub.lastReq.Tools {
		got[s.Name] = true
	}
	if !got["read_file"] || !got["write_file"] || got["task"] || got["run_skill"] || got["read_only_skill"] || got["research"] || got["bash"] {
		t.Errorf("sub-agent tools = %v, want {read_file, write_file} (meta-tools stripped, bash not requested)", got)
	}
}

func TestTaskToolNormalizesCommonFileToolAliases(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "write", Name: "write_file", Arguments: `{}`}},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "ok"},
			{Type: provider.ChunkDone},
		},
	}}
	var writeCalls int32
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "read_file", readOnly: true})
	parentReg.Add(fakeTool{name: "write_file", readOnly: false, calls: &writeCalls})
	parentReg.Add(fakeTool{name: "bash", readOnly: false})
	parentReg.Add(fakeTool{name: "glob", readOnly: true})
	parentReg.Add(fakeTool{name: "grep", readOnly: true})
	parentReg.Add(fakeTool{name: "ls", readOnly: true})
	task := newTestTaskTool(t, sub, parentReg, "sys", "", "", nil)

	args := []byte(`{"prompt":"inspect tool aliases","tools":["bash","read","write","glob","grep","search","directory_tree","file_survey","write"]}`)
	if _, err := task.Execute(testTaskContext(), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := map[string]bool{}
	for _, schema := range sub.lastReq.Tools {
		got[schema.Name] = true
	}
	for _, name := range []string{"bash", "read_file", "write_file", "glob", "grep", "ls"} {
		if !got[name] {
			t.Fatalf("sub-agent tools = %v, want %q after alias normalization", got, name)
		}
	}
	if len(got) != 6 {
		t.Fatalf("sub-agent tools = %v, want six unique normalized tools", got)
	}
	if writeCalls != 1 {
		t.Fatalf("write_file calls = %d, want aliased writer to execute once", writeCalls)
	}
}

func TestTaskSchemasEnumerateExactAvailableToolNames(t *testing.T) {
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "read_file", readOnly: true})
	parentReg.Add(fakeTool{name: "write_file", readOnly: false})
	parentReg.Add(fakeTool{name: "bash", readOnly: false})
	parentReg.Add(fakeTool{name: "wait", readOnly: true})
	task := newTestTaskTool(t, &mockProvider{name: "sub"}, parentReg, "sys", "", "", nil)

	for name, schema := range map[string]string{
		"task":           string(task.Schema()),
		"parallel_tasks": string(NewParallelTasksTool(task, parentReg).Schema()),
	} {
		if !strings.Contains(schema, `"enum":["read_file","write_file","bash"]`) {
			t.Fatalf("%s schema = %s, want exact available writer tool enum", name, schema)
		}
		if strings.Contains(schema, `+subagentToolBoundarySummary+`) || strings.Contains(schema, `"wait"`) {
			t.Fatalf("%s schema leaks a template marker or excluded wait tool: %s", name, schema)
		}
	}

	readOnlySchema := string(NewReadOnlyTaskTool(task).Schema())
	if !strings.Contains(readOnlySchema, `"enum":["read_file","bash"]`) {
		t.Fatalf("read_only_task schema = %s, want exact read-only tool enum", readOnlySchema)
	}
	if strings.Contains(readOnlySchema, `"write_file"`) || strings.Contains(readOnlySchema, `"wait"`) {
		t.Fatalf("read_only_task schema exposes writer or wait tool: %s", readOnlySchema)
	}
}

// TestTaskToolDefaultsToParentToolsWithoutMetaTools covers the no-whitelist
// path: the sub-agent inherits parent tools except subagent/skill meta-tools.
func TestTaskToolDefaultsToParentToolsWithoutMetaTools(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "ok"},
		{Type: provider.ChunkDone},
	}}
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "read_file", readOnly: true})
	parentReg.Add(fakeTool{name: "grep", readOnly: true})
	task := newTestTaskTool(t, sub, parentReg, "sys", "", "", nil)
	parentReg.Add(task)
	parentReg.Add(fakeTool{name: "run_skill", readOnly: false})
	parentReg.Add(fakeTool{name: "read_only_skill", readOnly: true})
	parentReg.Add(fakeTool{name: "explore", readOnly: false})
	parentReg.Add(fakeTool{name: "research", readOnly: false})
	parentReg.Add(fakeTool{name: "review", readOnly: false})
	parentReg.Add(fakeTool{name: "security_review", readOnly: false})
	parentReg.Add(fakeTool{name: "remember", readOnly: false})

	if _, err := task.Execute(testTaskContext(), []byte(`{"prompt":"x"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := map[string]bool{}
	for _, s := range sub.lastReq.Tools {
		got[s.Name] = true
	}
	if !got["read_file"] || !got["grep"] || !got["remember"] ||
		got["task"] || got["run_skill"] || got["read_only_skill"] || got["explore"] || got["research"] || got["review"] || got["security_review"] {
		t.Errorf("default sub-agent tools = %v, want normal tools inherited and meta-tools stripped", got)
	}
}

func TestTaskToolIgnoresLegacyEffortForExecutionAndIdentity(t *testing.T) {
	parent := &mockProvider{name: "parent", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "parent answer"},
		{Type: provider.ChunkDone},
	}}
	resolved := &mockProvider{name: "resolved", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "resolved answer"},
		{Type: provider.ChunkDone},
	}}
	var gotModel, gotEffort string
	resolve := func(model, effort string) (provider.Provider, *provider.Pricing, int, error) {
		gotModel, gotEffort = model, effort
		return resolved, nil, 0, nil
	}
	store := NewSubagentStore(t.TempDir())
	task := newTestTaskTool(t, parent, tool.NewRegistry(), "sys", "deepseek-pro", "medium", resolve).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	out, err := task.Execute(testTaskContext(), []byte(`{"prompt":"x","model":"override-model","effort":"high"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "resolved answer") {
		t.Fatalf("sub-agent did not use resolved provider, got %q", out)
	}
	if gotModel != "override-model" || gotEffort != "medium" {
		t.Fatalf("resolved profile = %q/%q, want override-model/medium", gotModel, gotEffort)
	}
	meta, err := store.LoadMeta(subagentRefFromOutput(t, out))
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.Model != "override-model" || meta.Effort != "medium" {
		t.Fatalf("persisted profile = %q/%q, want override-model/medium", meta.Model, meta.Effort)
	}
}

func TestTaskToolIgnoresLegacyEffortWithoutConfiguredEffort(t *testing.T) {
	parent := &mockProvider{name: "parent", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "parent answer"},
		{Type: provider.ChunkDone},
	}}
	resolve := func(string, string) (provider.Provider, *provider.Pricing, int, error) {
		return nil, nil, 0, errors.New("legacy effort reached resolver")
	}
	task := newTestTaskTool(t, parent, tool.NewRegistry(), "sys", "", "", resolve)

	if _, err := task.Execute(testTaskContext(), []byte(`{"prompt":"x","effort":"high"}`)); err != nil {
		t.Fatalf("legacy effort should preserve automatic profile without error: %v", err)
	}
}

func TestTaskToolResolveProfileIgnoresLegacyEffort(t *testing.T) {
	tests := []struct {
		name             string
		configuredModel  string
		configuredEffort string
		args             string
		wantModel        string
		wantEffort       string
	}{
		{name: "configured without model override", configuredModel: "configured-model", configuredEffort: "medium", args: `{"effort":"high"}`, wantModel: "configured-model", wantEffort: "medium"},
		{name: "configured with model override", configuredModel: "configured-model", configuredEffort: "medium", args: `{"model":"override-model","effort":"high"}`, wantModel: "override-model", wantEffort: "medium"},
		{name: "automatic", args: `{"effort":"high"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := newTestTaskTool(t, &mockProvider{name: "p"}, tool.NewRegistry(), "", tt.configuredModel, tt.configuredEffort, nil)
			profile := task.ResolveProfile(json.RawMessage(tt.args))
			if tt.wantModel == "" && tt.wantEffort == "" {
				if profile != nil {
					t.Fatalf("profile = %+v, want nil automatic profile", profile)
				}
				return
			}
			if profile == nil || profile.Model != tt.wantModel || profile.Effort != tt.wantEffort {
				t.Fatalf("profile = %+v, want %q/%q", profile, tt.wantModel, tt.wantEffort)
			}
		})
	}
}

func TestReadOnlyTaskIgnoresLegacyEffort(t *testing.T) {
	base := &mockProvider{name: "base", chunks: []provider.Chunk{{Type: provider.ChunkText, Text: "answer"}, {Type: provider.ChunkDone}}}
	resolved := &mockProvider{name: "resolved", chunks: []provider.Chunk{{Type: provider.ChunkText, Text: "answer"}, {Type: provider.ChunkDone}}}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	var gotModel, gotEffort string
	task := newTestTaskTool(t, base, reg, "sys", "configured-model", "medium", func(model, effort string) (provider.Provider, *provider.Pricing, int, error) {
		gotModel, gotEffort = model, effort
		return resolved, nil, 0, nil
	})
	if _, err := NewReadOnlyTaskTool(task).Execute(testTaskContext(), json.RawMessage(`{"prompt":"inspect","effort":"high"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotModel != "configured-model" || gotEffort != "medium" {
		t.Fatalf("resolved profile = %q/%q, want configured-model/medium", gotModel, gotEffort)
	}
}

func TestTaskToolRequiresTranscriptStore(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "answer"},
		{Type: provider.ChunkDone},
	}}
	task := NewTaskTool(sub, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil)

	_, err := task.Execute(testTaskContext(), []byte(`{"prompt":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "transcript store is required") {
		t.Fatalf("Execute error = %v, want transcript store requirement", err)
	}
}

// TestTaskToolRunsEphemerallyWithoutParentSession mirrors headless `kwu run`:
// the store is wired but the context carries no parent session, so the sub-agent
// must run without persistence and return its plain answer (no transcript ref).
func TestTaskToolRunsEphemerallyWithoutParentSession(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "headless answer"},
		{Type: provider.ChunkDone},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil)

	out, err := task.Execute(context.Background(), []byte(`{"prompt":"x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "headless answer") {
		t.Fatalf("got %q, want sub-agent final answer", out)
	}
	if strings.Contains(out, "Subagent reference") {
		t.Fatalf("ephemeral run should not emit a transcript reference: %q", out)
	}
}

func TestReadOnlyTaskToolRunsEphemerallyWithReadOnlyRegistry(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "read-only findings"},
		{Type: provider.ChunkDone},
	}}
	parentReg := tool.NewRegistry()
	parentReg.Add(fakeTool{name: "read_file", readOnly: true})
	parentReg.Add(fakeTool{name: "write_file", readOnly: false})
	parentReg.Add(fakeTool{name: "todo_write", readOnly: true})
	parentReg.Add(fakeTool{name: "complete_step", readOnly: true})
	parentReg.Add(fakeTool{name: "connect_tool_source", readOnly: true})
	parentReg.Add(fakeTool{name: "read_only_skill", readOnly: true})
	parentReg.Add(fakeTool{name: "bash", readOnly: false})
	task := newTestTaskTool(t, sub, parentReg, "writer sys", "", "", nil)
	readonly := NewReadOnlyTaskTool(task)
	parentReg.Add(task)
	parentReg.Add(readonly)

	out, err := readonly.Execute(testTaskContext(), []byte(`{"prompt":"inspect callers"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "read-only findings") {
		t.Fatalf("output = %q, want final answer", out)
	}
	if strings.Contains(out, "Subagent reference") {
		t.Fatalf("read_only_task should not persist transcript refs: %q", out)
	}
	if sys := sub.lastReq.Messages[0]; sys.Role != provider.RoleSystem || sys.Content != DefaultReadOnlyTaskSystemPrompt {
		t.Fatalf("read_only_task system prompt = %+v, want read-only prompt", sys)
	}

	got := map[string]bool{}
	for _, s := range sub.lastReq.Tools {
		got[s.Name] = true
	}
	for _, want := range []string{"read_file", "bash"} {
		if !got[want] {
			t.Fatalf("read_only_task sub-agent missing %q; tools=%v", want, toolSchemaNames(sub.lastReq.Tools))
		}
	}
	for _, hidden := range []string{"write_file", "todo_write", "complete_step", "connect_tool_source", "task", "read_only_task", "read_only_skill"} {
		if got[hidden] {
			t.Fatalf("read_only_task sub-agent should hide %q; tools=%v", hidden, toolSchemaNames(sub.lastReq.Tools))
		}
	}
}

func TestTaskToolRejectsContinuationWithoutParentSession(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "answer"},
		{Type: provider.ChunkDone},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil)

	_, err := task.Execute(context.Background(), []byte(`{"prompt":"x","continue_from":"sa_whatever"}`))
	if err == nil || !strings.Contains(err.Error(), "persisted session") {
		t.Fatalf("Execute error = %v, want persisted-session requirement", err)
	}
}

func TestTaskToolPersistsAndContinuesTranscript(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "first answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "second answer"},
			{Type: provider.ChunkDone},
		},
	}}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	store := NewSubagentStore(t.TempDir())
	task := newTestTaskTool(t, sub, reg, "sys", "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	first, err := task.Execute(testTaskContext(), []byte(`{"prompt":"first task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, first)
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.ParentSession != "parent-session" {
		t.Fatalf("parent session = %q, want parent-session", meta.ParentSession)
	}
	if !strings.Contains(first, "first answer") {
		t.Fatalf("first output = %q, want answer", first)
	}

	second, err := task.Execute(testTaskContext(), []byte(`{"prompt":"second task","continue_from":"`+ref+`"}`))
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if !strings.Contains(second, "second answer") {
		t.Fatalf("second output = %q, want answer", second)
	}
	if len(sub.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(sub.requests))
	}
	msgs := sub.requests[1].Messages
	if len(msgs) < 4 {
		t.Fatalf("continued request messages = %+v, want prior transcript plus new task", msgs)
	}
	if msgs[1].Content != "first task" || msgs[2].Content != "first answer" || lastUser(sub.requests[1]) != "second task" {
		t.Fatalf("continued request messages = %+v, want first task/answer then second task", msgs)
	}
}

func TestTaskToolPersistedMetaInheritsActorContext(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "actor answer"},
		{Type: provider.ChunkDone},
	}}
	store := NewSubagentStore(t.TempDir())
	actor := userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: filepath.Join(t.TempDir(), "workspace"),
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: "lark",
			UserID:   "remote-user",
		},
	}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil).
		WithTranscripts(store, actor.WorkspaceRoot, "base-model", "base-effort").
		WithActorContext(actor)

	out, err := task.Execute(testTaskContext(), []byte(`{"prompt":"x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, out)
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.ActorID != "parent-actor" || meta.ProfileID != "profile-a" || meta.AuthMethod != "oauth" {
		t.Fatalf("actor metadata = %+v, want inherited parent actor/profile/auth", meta)
	}
	if filepath.Clean(meta.ActorWorkspaceRoot) != filepath.Clean(actor.WorkspaceRoot) {
		t.Fatalf("actor workspace root = %q, want %q", meta.ActorWorkspaceRoot, actor.WorkspaceRoot)
	}
	if meta.ParentActorID != "parent-actor" {
		t.Fatalf("parent actor id = %q, want parent-actor", meta.ParentActorID)
	}
	if meta.RemoteIdentity == nil || meta.RemoteIdentity.UserID != "remote-user" {
		t.Fatalf("remote identity = %+v, want remote user", meta.RemoteIdentity)
	}
}

func TestTaskToolRejectsCrossProfileContinueWithSameParentSession(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "first answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "should not run"},
			{Type: provider.ChunkDone},
		},
	}}
	store := NewSubagentStore(t.TempDir())
	workspace := t.TempDir()
	actor := userctx.ActorContext{
		ActorID:       "parent-actor",
		ProfileID:     "profile-a",
		AuthMethod:    "oauth",
		WorkspaceRoot: workspace,
	}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil).
		WithTranscripts(store, workspace, "base-model", "base-effort").
		WithActorContext(actor)

	first, err := task.Execute(testTaskContext(), []byte(`{"prompt":"first task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, first)

	otherActor := actor
	otherActor.ProfileID = "profile-b"
	otherTask := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil).
		WithTranscripts(store, workspace, "base-model", "base-effort").
		WithActorContext(otherActor)
	_, err = otherTask.Execute(testTaskContext(), []byte(`{"prompt":"second task","continue_from":"`+ref+`"}`))
	if err == nil || !strings.Contains(err.Error(), "actor profile") {
		t.Fatalf("cross-profile continuation error = %v, want actor profile mismatch", err)
	}
	if len(sub.requests) != 1 {
		t.Fatalf("provider requests = %d, want only first profile run", len(sub.requests))
	}
}

func TestTaskToolContinueFromAncestorReturnsCopiedReferenceGuidance(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "root answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "child answer"},
			{Type: provider.ChunkDone},
		},
	}}
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	task := NewTaskTool(sub, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	rootCtx := WithParentSession(context.Background(), "root")
	first, err := task.Execute(rootCtx, []byte(`{"prompt":"root task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	rootRef := subagentRefFromOutput(t, first)

	if err := SaveBranchMeta(filepath.Join(sessionDir, "root.jsonl"), BranchMeta{}); err != nil {
		t.Fatalf("SaveBranchMeta root: %v", err)
	}
	if err := SaveBranchMeta(filepath.Join(sessionDir, "child.jsonl"), BranchMeta{ParentID: "root"}); err != nil {
		t.Fatalf("SaveBranchMeta child: %v", err)
	}

	childCtx := WithParentSession(context.Background(), "child")
	second, err := task.Execute(childCtx, []byte(`{"prompt":"child task","continue_from":"`+rootRef+`"}`))
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	childRef := subagentRefFromOutput(t, second)
	if childRef == rootRef {
		t.Fatalf("child ref = source ref %q, want copied ref", childRef)
	}
	if !strings.Contains(second, "Forked from: "+rootRef) {
		t.Fatalf("second output = %q, want Forked from source ref", second)
	}
	if !strings.Contains(second, "The requested ref resolves to an ancestor conversation transcript") {
		t.Fatalf("second output = %q, want ancestor-copy guidance", second)
	}
	if !strings.Contains(second, "Final answer:\nchild answer") {
		t.Fatalf("second output = %q, want final answer", second)
	}
}

func TestTaskToolLegacyForkFromAncestorConvertsToCopiedReference(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "root answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "child answer"},
			{Type: provider.ChunkDone},
		},
	}}
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	task := NewTaskTool(sub, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	rootCtx := WithParentSession(context.Background(), "root")
	first, err := task.Execute(rootCtx, []byte(`{"prompt":"root task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	rootRef := subagentRefFromOutput(t, first)

	if err := SaveBranchMeta(filepath.Join(sessionDir, "root.jsonl"), BranchMeta{}); err != nil {
		t.Fatalf("SaveBranchMeta root: %v", err)
	}
	if err := SaveBranchMeta(filepath.Join(sessionDir, "child.jsonl"), BranchMeta{ParentID: "root"}); err != nil {
		t.Fatalf("SaveBranchMeta child: %v", err)
	}

	childCtx := WithParentSession(context.Background(), "child")
	second, err := task.Execute(childCtx, []byte(`{"prompt":"child task","fork_from":"`+rootRef+`"}`))
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	childRef := subagentRefFromOutput(t, second)
	if childRef == rootRef {
		t.Fatalf("child ref = source ref %q, want copied ref", childRef)
	}
	if !strings.Contains(second, "Forked from: "+rootRef) ||
		!strings.Contains(second, "Final answer:\nchild answer") {
		t.Fatalf("second output = %q, want copied reference guidance and final answer", second)
	}
}

func TestTaskToolRejectsLegacyForkFromCurrentSession(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "first answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "should not run"},
			{Type: provider.ChunkDone},
		},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil).
		WithTranscripts(NewSubagentStore(t.TempDir()), t.TempDir(), "base-model", "base-effort")

	first, err := task.Execute(testTaskContext(), []byte(`{"prompt":"first task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, first)
	_, err = task.Execute(testTaskContext(), []byte(`{"prompt":"second task","fork_from":"`+ref+`"}`))
	if err == nil || !strings.Contains(err.Error(), "cannot be safely converted") {
		t.Fatalf("legacy fork error = %v, want unsafe conversion rejection", err)
	}
	if len(sub.requests) != 1 {
		t.Fatalf("provider requests = %d, want only first run", len(sub.requests))
	}
}

func TestTaskToolFailedContinuationPersistsAndAllowsReuse(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "first answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkError, Err: errors.New("provider failed")},
		},
		{
			{Type: provider.ChunkText, Text: "recovered answer"},
			{Type: provider.ChunkDone},
		},
	}}
	store := NewSubagentStore(t.TempDir())
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	task := NewTaskTool(sub, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	first, err := task.Execute(testTaskContext(), []byte(`{"prompt":"first task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, first)

	_, err = task.Execute(testTaskContext(), []byte(`{"prompt":"second task","continue_from":"`+ref+`"}`))
	if err == nil || !strings.Contains(err.Error(), "provider failed") {
		t.Fatalf("second Execute error = %v, want provider failure", err)
	}
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.Status != SubagentFailed {
		t.Fatalf("status = %q, want failed", meta.Status)
	}
	loaded, err := LoadSession(store.sessionPath(ref))
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	msgs := loaded.Snapshot()
	if len(msgs) != 4 || msgs[1].Content != "first task" || msgs[2].Content != "first answer" || msgs[3].Content != "second task" {
		t.Fatalf("failed continuation transcript = %+v, want first task/answer plus second task", msgs)
	}
	third, err := task.Execute(testTaskContext(), []byte(`{"prompt":"third task","continue_from":"`+ref+`"}`))
	if err != nil {
		t.Fatalf("reuse failed ref: %v", err)
	}
	if !strings.Contains(third, "recovered answer") || subagentRefFromOutput(t, third) != ref {
		t.Fatalf("third result = %q, want recovered answer on same ref", third)
	}
}

func TestTaskToolBackgroundPanicPersistsFailedMetadata(t *testing.T) {
	sub := panicProvider{name: "panic-sub"}
	store := NewSubagentStore(t.TempDir())
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	task := NewTaskTool(sub, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := testTaskContext()
	ctx = jobs.WithSession(ctx, "parent-session")
	ctx = jobs.WithManager(ctx, jm)
	out, err := task.Execute(ctx, []byte(`{"prompt":"panic task","run_in_background":true}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, out)
	waitOut, err := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`","timeout_seconds":5}`))
	if err != nil {
		t.Fatalf("wait_task: %v", err)
	}
	if !strings.Contains(waitOut, `"turnStatus":"failed"`) || !strings.Contains(waitOut, "internal error: panic") {
		t.Fatalf("wait_task output = %q, want failed panic receipt", waitOut)
	}
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.Status != SubagentFailed {
		t.Fatalf("status = %q, want failed", meta.Status)
	}
	model, effort := task.effectiveIdentity("", "")
	continued, err := store.PrepareContinue(ref, SubagentSpec{
		Kind: "task", Name: "task", WorkspaceRoot: task.workspaceRoot,
		ParentSession: "parent-session", SystemPrompt: task.sysPrompt,
		Registry: task.buildSubReg(context.Background(), nil), Model: model, Effort: effort,
	})
	if err != nil {
		t.Fatalf("failed ref should be lifecycle-resumable: %v", err)
	}
	continued.Release()
}

func TestTaskToolBackgroundResultIncludesReferenceGuidance(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "background answer"},
		{Type: provider.ChunkDone},
	}}
	store := NewSubagentStore(t.TempDir())
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	task := NewTaskTool(sub, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := testTaskContext()
	ctx = jobs.WithSession(ctx, "parent-session")
	ctx = jobs.WithManager(ctx, jm)
	out, err := task.Execute(ctx, []byte(`{"prompt":"background task","run_in_background":true}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, out)
	if !strings.Contains(out, "Turn status: running") || !strings.Contains(out, "Task run ID: task-") {
		t.Fatalf("start output = %q, want running ref and public task run id", out)
	}
	waitOut, err := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`","timeout_seconds":5}`))
	if err != nil {
		t.Fatalf("wait_task: %v", err)
	}
	if !strings.Contains(waitOut, `"subagentRef":"`+ref+`"`) ||
		!strings.Contains(waitOut, `"turnStatus":"completed"`) ||
		!strings.Contains(waitOut, `"finalAnswer":"background answer"`) ||
		!strings.Contains(waitOut, `"kind":"turn_status","success":true,"status":"completed"`) {
		t.Fatalf("wait_task output = %q, want structured completion", waitOut)
	}

	parent := &Agent{evidence: evidence.NewLedger()}
	parent.recordToolReceipt(provider.ToolCall{ID: "wait", Name: "wait_task", Arguments: `{"subagent_ref":"` + ref + `"}`}, NewWaitTaskTool(task), waitOut, nil, false)
	if !parent.evidence.HasSuccessfulTodoProgressReceipt() {
		t.Fatal("completed wait_task receipt was not accepted as parent progress")
	}
}

type persistedStatusObservation struct {
	event     event.Event
	persisted SubagentStatus
	loadErr   error
}

func observePersistedStatus(store *SubagentStore) (event.Sink, <-chan persistedStatusObservation) {
	observed := make(chan persistedStatusObservation, 8)
	return event.FuncSink(func(e event.Event) {
		if e.Kind != event.SubagentStatusChanged {
			return
		}
		meta, err := store.LoadMeta(e.SubagentRef)
		status := SubagentStatus("")
		if err == nil {
			status = meta.Status
		}
		observed <- persistedStatusObservation{event: e, persisted: status, loadErr: err}
	}), observed
}

func requirePersistedStatusSequence(t *testing.T, observed <-chan persistedStatusObservation, ref string, want ...SubagentStatus) {
	t.Helper()
	for _, status := range want {
		select {
		case got := <-observed:
			if got.loadErr != nil {
				t.Fatalf("load meta when %s event arrived: %v", status, got.loadErr)
			}
			if got.event.SubagentRef != ref || got.event.SubagentStatus != string(status) || got.persisted != status {
				t.Fatalf("status observation = %+v, want ref %q and persisted/event status %q", got, ref, status)
			}
			if got.event.Text != "" || got.event.ParentID != "" || got.event.Tool.Name != "" {
				t.Fatalf("status event leaked transcript/tool content: %+v", got.event)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s status event", status)
		}
	}
}

func TestTaskToolEmitsCompletedStatusAfterPersistence(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	parent, observed := observePersistedStatus(store)
	task := NewTaskTool(&mockProvider{name: "sub", chunks: []provider.Chunk{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}}}, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := WithCallContext(jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm), "task-call", parent, nil, false)
	start, err := task.Execute(ctx, []byte(`{"prompt":"complete task"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, start)
	if _, err := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`","timeout_seconds":5}`)); err != nil {
		t.Fatalf("wait_task: %v", err)
	}
	requirePersistedStatusSequence(t, observed, ref, SubagentRunning, SubagentCompleted)
}

func TestTaskToolEmitsFailedStatusAfterPersistence(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	parent, observed := observePersistedStatus(store)
	task := NewTaskTool(panicProvider{name: "panic-sub"}, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := WithCallContext(jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm), "task-call", parent, nil, false)
	start, err := task.Execute(ctx, []byte(`{"prompt":"panic task"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, start)
	if _, err := NewWaitTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`","timeout_seconds":5}`)); err != nil {
		t.Fatalf("wait_task: %v", err)
	}
	requirePersistedStatusSequence(t, observed, ref, SubagentRunning, SubagentFailed)
}

func TestTaskToolEmitsInterruptedStatusAfterPersistence(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	parent, observed := observePersistedStatus(store)
	task := NewTaskTool(stuckStreamProvider{}, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := WithCallContext(jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm), "task-call", parent, nil, false)
	start, err := task.Execute(ctx, []byte(`{"prompt":"interrupt task"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, start)
	if _, err := NewStopTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`"}`)); err != nil {
		t.Fatalf("stop_task: %v", err)
	}
	requirePersistedStatusSequence(t, observed, ref, SubagentRunning, SubagentInterrupted)
}

func TestMarkSubagentRunningAdvancesTimestampBeforeEmitting(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	run, err := store.PrepareFresh(SubagentSpec{
		ParentSession: "parent-session",
		SystemPrompt:  "sys",
		Registry:      tool.NewRegistry(),
	})
	if err != nil {
		t.Fatalf("PrepareFresh: %v", err)
	}
	defer run.Release()

	previous := time.Now().UTC().Add(time.Hour)
	run.Meta.UpdatedAt = previous
	parent, observed := observePersistedStatus(store)
	if err := (&TaskTool{transcripts: store}).markSubagentRunning(run, parent); err != nil {
		t.Fatalf("markSubagentRunning: %v", err)
	}
	if !run.Meta.UpdatedAt.After(previous) {
		t.Fatalf("UpdatedAt = %s, want after %s", run.Meta.UpdatedAt, previous)
	}
	requirePersistedStatusSequence(t, observed, run.Ref, SubagentRunning)
}

func TestTaskToolDoesNotEmitStatusWhenDestroyedParentSkipsPersistence(t *testing.T) {
	store := NewSubagentStore(t.TempDir()).WithDestroyedChecker(func(string) bool { return true })
	task := &TaskTool{transcripts: store}
	run := &SubagentRun{
		Ref:  "sa_destroyed",
		Meta: SubagentMeta{Ref: "sa_destroyed", ParentSession: "parent-session", Status: SubagentRunning, UpdatedAt: time.Unix(1, 0)},
	}
	var got []event.Event
	if err := task.saveSubagentFailed(run, event.FuncSink(func(e event.Event) { got = append(got, e) })); err != nil {
		t.Fatalf("saveSubagentFailed: %v", err)
	}
	if len(got) != 0 || run.Meta.Status != SubagentRunning {
		t.Fatalf("destroyed parent status = %q, events = %+v; want unchanged meta and no event", run.Meta.Status, got)
	}
}

func TestWaitTaskNonCompletedReceiptsCannotSatisfyParentProgress(t *testing.T) {
	for _, status := range []SubagentStatus{SubagentRunning, SubagentFailed, SubagentInterrupted} {
		t.Run(string(status), func(t *testing.T) {
			output, err := marshalTaskReceipts([]taskTurnReceipt{{
				SubagentRef: "sa_test", TurnStatus: status,
				Validations: []taskHostValidation{{Kind: "turn_status", Success: false, Status: string(status)}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			parent := &Agent{evidence: evidence.NewLedger()}
			parent.recordToolReceipt(provider.ToolCall{ID: "wait", Name: "wait_task", Arguments: `{"subagent_ref":"sa_test"}`}, NewWaitTaskTool(nil), output, nil, false)
			if parent.evidence.HasSuccessfulTodoProgressReceipt() {
				t.Fatalf("%s wait_task receipt satisfied parent progress", status)
			}
		})
	}
}

func TestNonDispatchedTaskReceiptCannotSatisfyParentProgress(t *testing.T) {
	for _, status := range []string{"dispatch_decision_required", "dependency_not_satisfied", "capacity_wait_required"} {
		parent := &Agent{evidence: evidence.NewLedger()}
		output := `{"status":"` + status + `","message":"retry"}`
		parent.recordToolReceipt(provider.ToolCall{ID: "task", Name: "task", Arguments: `{"prompt":"x"}`}, &TaskTool{}, output, nil, false)
		if parent.evidence.HasSuccessfulTodoProgressReceipt() {
			t.Fatalf("%s task scheduling receipt satisfied parent progress", status)
		}
	}
}

func TestStopTaskInterruptsTurnAndKeepsIncrementalTranscript(t *testing.T) {
	store := NewSubagentStore(t.TempDir())
	task := NewTaskTool(stuckStreamProvider{}, nil, tool.NewRegistry(), 20, 0, 0, 0, 0, 0, 0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	ctx := jobs.WithManager(jobs.WithSession(testTaskContext(), "parent-session"), jm)

	start, err := task.Execute(ctx, []byte(`{"prompt":"persist before stop"}`))
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	ref := subagentRefFromOutput(t, start)
	overlap, err := task.Execute(ctx, []byte(`{"prompt":"overlap","continue_from":"`+ref+`"}`))
	if err != nil || !strings.Contains(overlap, `"status":"dispatch_decision_required"`) {
		t.Fatalf("overlapping continuation = %q, %v; want dispatch decision", overlap, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		loaded, loadErr := LoadSession(store.sessionPath(ref))
		if loadErr == nil {
			msgs := loaded.Snapshot()
			if len(msgs) >= 2 && msgs[1].Content == "persist before stop" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("running child prompt was not incrementally persisted")
		}
		time.Sleep(25 * time.Millisecond)
	}

	stopped, err := NewStopTaskTool(task).Execute(ctx, []byte(`{"subagent_ref":"`+ref+`"}`))
	if err != nil {
		t.Fatalf("stop_task: %v", err)
	}
	if !strings.Contains(stopped, `"turnStatus":"interrupted"`) {
		t.Fatalf("stop_task output = %q, want interrupted", stopped)
	}
	meta, err := store.LoadMeta(ref)
	if err != nil {
		t.Fatal(err)
	}
	model, effort := task.effectiveIdentity("", "")
	continued, err := store.PrepareContinue(ref, SubagentSpec{
		Kind: "task", Name: "task", WorkspaceRoot: task.workspaceRoot,
		ParentSession: "parent-session", SystemPrompt: task.sysPrompt,
		Registry: task.buildSubReg(context.Background(), nil), Model: model, Effort: effort,
	})
	if err != nil {
		t.Fatalf("interrupted ref should be resumable: %v (meta=%+v)", err, meta)
	}
	continued.Release()
}

func TestTaskToolAsyncAncestorContinuationUsesCopiedReference(t *testing.T) {
	sub := &mockProvider{name: "sub", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkText, Text: "root answer"},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "child background answer"},
			{Type: provider.ChunkDone},
		},
	}}
	sessionDir := t.TempDir()
	store := NewSubagentStore(filepath.Join(sessionDir, "subagents"))
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	task := NewTaskTool(sub, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil).
		WithTranscripts(store, t.TempDir(), "base-model", "base-effort")

	rootCtx := WithParentSession(context.Background(), "root")
	rootOut, err := task.Execute(rootCtx, []byte(`{"prompt":"root task"}`))
	if err != nil {
		t.Fatalf("root Execute: %v", err)
	}
	rootRef := subagentRefFromOutput(t, rootOut)
	if err := SaveBranchMeta(filepath.Join(sessionDir, "root.jsonl"), BranchMeta{}); err != nil {
		t.Fatalf("SaveBranchMeta root: %v", err)
	}
	if err := SaveBranchMeta(filepath.Join(sessionDir, "child.jsonl"), BranchMeta{ParentID: "root"}); err != nil {
		t.Fatalf("SaveBranchMeta child: %v", err)
	}

	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	childCtx := WithParentSession(context.Background(), "child")
	childCtx = jobs.WithSession(childCtx, "child")
	childCtx = jobs.WithManager(childCtx, jm)
	startOut, err := task.Execute(childCtx, []byte(`{"prompt":"child task","continue_from":"`+rootRef+`","run_in_background":true}`))
	if err != nil {
		t.Fatalf("child Execute: %v", err)
	}
	childRef := subagentRefFromOutput(t, startOut)
	if childRef == rootRef {
		t.Fatalf("child ref = source ref %q, want copied ref", childRef)
	}
	if !strings.Contains(startOut, "Turn status: running") || strings.Contains(startOut, "child background answer") {
		t.Fatalf("start output = %q, want copied running ref without final answer", startOut)
	}
	waitOut, err := NewWaitTaskTool(task).Execute(childCtx, []byte(`{"subagent_ref":"`+childRef+`","timeout_seconds":5}`))
	if err != nil {
		t.Fatalf("wait_task: %v", err)
	}
	if !strings.Contains(waitOut, `"subagentRef":"`+childRef+`"`) ||
		!strings.Contains(waitOut, `"finalAnswer":"child background answer"`) {
		t.Fatalf("wait_task output = %q, want copied ref completion", waitOut)
	}
}

func TestTaskToolRejectsMismatchedContinuationProfile(t *testing.T) {
	sub := &mockProvider{name: "sub", chunks: []provider.Chunk{
		{Type: provider.ChunkText, Text: "answer"},
		{Type: provider.ChunkDone},
	}}
	task := newTestTaskTool(t, sub, tool.NewRegistry(), "sys", "", "", nil).
		WithTranscripts(NewSubagentStore(t.TempDir()), t.TempDir(), "base-model", "")

	out, err := task.Execute(testTaskContext(), []byte(`{"prompt":"first task"}`))
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	ref := subagentRefFromOutput(t, out)
	_, err = task.Execute(testTaskContext(), []byte(`{"prompt":"second task","continue_from":"`+ref+`","model":"other-model"}`))
	if err == nil || !strings.Contains(err.Error(), "model/effort") {
		t.Fatalf("mismatched model error = %v, want compatibility failure", err)
	}
}

func subagentRefFromOutput(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Subagent reference: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Subagent reference: "))
		}
	}
	t.Fatalf("no subagent reference in output:\n%s", out)
	return ""
}

func taskRunIDFromOutput(t *testing.T, out string) string {
	t.Helper()
	const prefix = "Task run ID: "
	start := strings.Index(out, prefix)
	if start < 0 {
		t.Fatalf("task output missing run id: %q", out)
	}
	value := out[start+len(prefix):]
	if end := strings.IndexByte(value, '\n'); end >= 0 {
		value = value[:end]
	}
	if value = strings.TrimSpace(value); value == "" {
		t.Fatalf("task output has empty run id: %q", out)
	}
	return value
}

func TestSubSinkForwardsUsageToParent(t *testing.T) {
	var got []event.Event
	parent := event.FuncSink(func(e event.Event) {
		got = append(got, e)
	})
	subSinkFor("task_1", parent).Emit(event.Event{
		Kind:        event.Usage,
		Usage:       &provider.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
		UsageSource: event.UsageSourceSubagent,
	})
	if len(got) != 1 || got[0].Usage == nil || got[0].UsageSource != event.UsageSourceSubagent {
		t.Fatalf("forwarded events = %+v, want subagent usage", got)
	}
}

func TestSubSinkForwardsPersistedStatusWithoutTranscriptOwnership(t *testing.T) {
	var got []event.Event
	subSinkForContext("task_1", event.FuncSink(func(e event.Event) { got = append(got, e) }), event.TaskTreeContext{}, "child-ref").Emit(event.Event{
		Kind:           event.SubagentStatusChanged,
		SubagentRef:    "child-ref",
		SubagentStatus: "completed",
	})
	if len(got) != 1 || got[0].Kind != event.SubagentStatusChanged || got[0].SubagentRef != "child-ref" || got[0].SubagentStatus != "completed" || got[0].ParentID != "" {
		t.Fatalf("forwarded status events = %+v, want content-free child status", got)
	}
}

func TestSubSinkAssociatesSubagentEventsWithTaskTreeNode(t *testing.T) {
	var got []event.Event
	parent := event.FuncSink(func(e event.Event) {
		got = append(got, e)
	})
	sink := subSinkForContext("task_1", parent, event.TaskTreeContext{TreeID: "tree-1", NodeID: "patch"}, "child-ref")

	sink.Emit(event.Event{Kind: event.Text, Text: "streamed answer"})
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "tool_1", Name: "bash", Args: `{"command":"go test"}`}})
	sink.Emit(event.Event{Kind: event.ToolProgress, Tool: event.Tool{ID: "tool_1", Output: "running\n"}})
	sink.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{ID: "tool_1", Name: "bash", Output: "ok"}})

	if len(got) != 5 {
		t.Fatalf("forwarded events = %+v, want 5", got)
	}
	if got[0].Kind != event.TaskNodeOutput || got[0].TaskTree == nil {
		t.Fatalf("first forwarded event = %+v, want TaskNodeOutput", got[0])
	}
	if got[0].TaskTree.TreeID != "tree-1" || got[0].TaskTree.NodeID != "patch" || got[0].TaskTree.OutputDelta != "streamed answer" {
		t.Fatalf("task node output payload = %+v", got[0].TaskTree)
	}
	if got[1].Kind != event.Text || got[1].ParentID != "task_1" || got[1].SubagentRef != "child-ref" || got[1].Text != "streamed answer" {
		t.Fatalf("child text event = %+v, want parented subagent text", got[1])
	}
	for _, e := range got[2:] {
		if e.Tool.ParentID != "task_1" || e.Tool.TaskTreeID != "tree-1" || e.Tool.NodeID != "patch" {
			t.Fatalf("tool event missing task-tree ownership: %+v", e)
		}
		if e.Tool.ID != "task_1/tool_1" {
			t.Fatalf("tool ID = %q, want namespaced child ID", e.Tool.ID)
		}
		if e.Tool.SubagentRef != "child-ref" || e.Tool.SourceToolID != "tool_1" {
			t.Fatalf("tool child identity = %+v, want ref and raw tool ID", e.Tool)
		}
	}
}

func TestSubSinkPreservesTranscriptUsageStatusAndIgnoresOtherEvents(t *testing.T) {
	var got []event.Event
	sink := subSinkForContext("task_1", event.FuncSink(func(e event.Event) { got = append(got, e) }), event.TaskTreeContext{}, "child-ref")
	sink.Emit(event.Event{Kind: event.Reasoning, Text: "thinking"})
	sink.Emit(event.Event{Kind: event.Message, Text: "answer", Reasoning: "thinking"})
	sink.Emit(event.Event{Kind: event.Usage, Usage: &provider.Usage{TotalTokens: 3}})
	sink.Emit(event.Event{Kind: event.SubagentStatusChanged, SubagentStatus: "running"})
	sink.Emit(event.Event{Kind: event.Notice, Text: "must stay isolated"})

	if len(got) != 4 {
		t.Fatalf("forwarded events = %+v, want 4", got)
	}
	for i := 0; i < 2; i++ {
		if got[i].ParentID != "task_1" || got[i].SubagentRef != "child-ref" {
			t.Fatalf("transcript event %d = %+v, want parent and child identity", i, got[i])
		}
	}
	if got[2].Kind != event.Usage || got[2].UsageSource != event.UsageSourceSubagent || got[2].ParentID != "" || got[2].SubagentRef != "" {
		t.Fatalf("usage event = %+v, want observability-only subagent usage", got[2])
	}
	if got[3].Kind != event.SubagentStatusChanged || got[3].SubagentRef != "child-ref" || got[3].SubagentStatus != "running" || got[3].ParentID != "" {
		t.Fatalf("status event = %+v, want content-free child status", got[3])
	}
}

func TestTaskToolCarriesRecentKeepIntoSubsessions(t *testing.T) {
	task := NewTaskTool(&mockProvider{name: "sub"}, nil, tool.NewRegistry(), 20, 0, 7, 0, 0, 0, 0.0, "", "sys", nil, 0, "", "", nil)
	if task.recentKeep != 7 {
		t.Fatalf("recentKeep = %d, want 7", task.recentKeep)
	}
}

func TestTaskSchemasNeverExposeEffort(t *testing.T) {
	task := newTestTaskTool(t, &mockProvider{name: "p"}, tool.NewRegistry(), "", "", "", nil).
		WithSubagentEffortLevels([]string{"low", "medium", "high"})

	for name, schema := range map[string]string{
		"task":           string(task.Schema()),
		"read_only_task": string(NewReadOnlyTaskTool(task).Schema()),
	} {
		if strings.Contains(schema, `"effort"`) {
			t.Fatalf("%s schema = %s, want no effort field", name, schema)
		}
	}
}

func TestTaskSchemaHidesEffortWhenUnsupported(t *testing.T) {
	task := newTestTaskTool(t, &mockProvider{name: "p"}, tool.NewRegistry(), "", "", "", nil).
		WithSubagentEffortLevels(nil)

	schema := string(task.Schema())
	if strings.Contains(schema, `"effort"`) {
		t.Fatalf("task schema = %s, want no effort field", schema)
	}
	readOnlySchema := string(NewReadOnlyTaskTool(task).Schema())
	if strings.Contains(readOnlySchema, `"effort"`) {
		t.Fatalf("read_only_task schema = %s, want no effort field", readOnlySchema)
	}
}

func TestTaskToolPropagatesToolTimeoutToSubagent(t *testing.T) {
	task := newTestTaskTool(t, &mockProvider{name: "sub"}, tool.NewRegistry(), "sys", "", "", nil)
	task.WithToolTimeout(45 * time.Second)
	if task.toolTimeout != 45*time.Second {
		t.Fatalf("task tool timeout = %s, want 45s", task.toolTimeout)
	}
}

func newTestTaskTool(t *testing.T, prov provider.Provider, reg *tool.Registry, sysPrompt, subagentModel, subagentEffort string, resolve func(string, string) (provider.Provider, *provider.Pricing, int, error)) *TaskTool {
	t.Helper()
	return NewTaskTool(prov, nil, reg, 20, 0, 0, 0, 0, 0, 0.0, "", sysPrompt, nil, 0, subagentModel, subagentEffort, resolve).
		WithTranscripts(NewSubagentStore(t.TempDir()), t.TempDir(), "base-model", "base-effort")
}

type panicProvider struct{ name string }

func (p panicProvider) Name() string { return p.name }

func (p panicProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	panic("subagent boom")
}
