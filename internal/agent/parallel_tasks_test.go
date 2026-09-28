package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type parallelCancelFatalDispatcher struct {
	started chan struct{}
	release <-chan struct{}
}

func TestParallelBranchSinkHidesBranchInternals(t *testing.T) {
	var got []event.Event
	sink := parallelBranchSink(event.FuncSink(func(e event.Event) {
		got = append(got, e)
	}))

	sink.Emit(event.Event{Kind: event.Reasoning, Reasoning: "hidden reasoning"})
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "child-tool", Name: "bash"}})
	sink.Emit(event.Event{Kind: event.Text, Text: "answer delta"})
	sink.Emit(event.Event{Kind: event.Usage, Usage: &provider.Usage{TotalTokens: 3}})

	if len(got) != 1 {
		t.Fatalf("forwarded events = %+v, want usage only", got)
	}
	if got[0].Kind != event.Usage || got[0].UsageSource != event.UsageSourceSubagent {
		t.Fatalf("event = %+v, want attributed usage", got[0])
	}
}

type parallelIgnoringCancelTool struct {
	entered chan struct{}
	release <-chan struct{}
}

func (parallelIgnoringCancelTool) Name() string        { return "block" }
func (parallelIgnoringCancelTool) Description() string { return "blocks without observing context" }
func (parallelIgnoringCancelTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}
func (parallelIgnoringCancelTool) ReadOnly() bool { return true }
func (t parallelIgnoringCancelTool) Execute(context.Context, json.RawMessage) (string, error) {
	t.entered <- struct{}{}
	<-t.release
	return "released", nil
}

func (d parallelCancelFatalDispatcher) Stream(ctx context.Context, base provider.Provider, request provider.Request) (<-chan provider.Chunk, error) {
	chunks, _, err := d.StreamWithTerminalResult(ctx, base, request)
	return chunks, err
}

func (d parallelCancelFatalDispatcher) StreamWithTerminalResult(ctx context.Context, _ provider.Provider, _ provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	d.started <- struct{}{}
	<-ctx.Done()
	<-d.release
	return nil, nil, ErrMainStreamDispatcherCloseTimeout
}

func TestParallelTasksToolIsWriteCapable(t *testing.T) {
	if (&ParallelTasksTool{}).ReadOnly() {
		t.Fatal("parallel_tasks must be write-capable because spawned sub-agents can invoke writer tools")
	}
}

func TestParallelTasksFatalErrorPreservesDispatcherCloseTimeout(t *testing.T) {
	persistenceErr := &streamDispatcherTerminalError{cause: errors.New("usage terminal persistence failed")}
	err := parallelTasksFatalError([]error{errors.New("ordinary child failure"), ErrMainStreamDispatcherCloseTimeout, persistenceErr})
	if !errors.Is(err, ErrMainStreamDispatcherCloseTimeout) {
		t.Fatalf("parallel fatal error = %v", err)
	}
	if !errors.Is(err, persistenceErr.cause) {
		t.Fatalf("parallel fatal error lost terminal persistence failure: %v", err)
	}
	if err := parallelTasksFatalError([]error{errors.New("ordinary child failure")}); err != nil {
		t.Fatalf("ordinary parallel failure became fatal: %v", err)
	}
}

func TestParallelTasksCancellationWaitsForStartedChildFatalErrors(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	dispatcher := parallelCancelFatalDispatcher{started: started, release: release}
	task := newTestTaskTool(t, &mockProvider{name: "base"}, tool.NewRegistry(), "", "", "", nil).
		WithStreamDispatcherResolver(func(string, string, provider.Provider) (SubagentStreamDispatchers, error) {
			return SubagentStreamDispatchers{Main: dispatcher}, nil
		})
	parallel := NewParallelTasksTool(task, tool.NewRegistry())
	ctx, cancel := context.WithCancel(withCallContext(context.Background(), "parallel-parent", event.Discard, nil, false))
	type executeResult struct {
		output string
		err    error
	}
	result := make(chan executeResult, 1)
	go func() {
		output, err := parallel.Execute(ctx, json.RawMessage(`{"tasks":[{"prompt":"one"},{"prompt":"two"}]}`))
		result <- executeResult{output: output, err: err}
	}()
	<-started
	<-started
	cancel()

	timer := time.NewTimer(200 * time.Millisecond)
	select {
	case got := <-result:
		close(release)
		timer.Stop()
		t.Fatalf("parallel_tasks returned before started children settled: %v", got.err)
	case <-timer.C:
	}
	close(release)
	got := <-result
	if !errors.Is(got.err, context.Canceled) || !errors.Is(got.err, ErrMainStreamDispatcherCloseTimeout) {
		t.Fatalf("parallel cancellation error = %v; output=%q", got.err, got.output)
	}
}

func TestParallelTasksCancellationDoesNotWaitForChildToolAfterModelCallSettles(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	registry := tool.NewRegistry()
	registry.Add(parallelIgnoringCancelTool{entered: entered, release: release})
	base := &mockProvider{name: "base", chunks: []provider.Chunk{
		{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "block-call", Name: "block", Arguments: `{}`}},
		{Type: provider.ChunkDone},
	}}
	task := newTestTaskTool(t, base, registry, "", "", "", nil)
	parallel := NewParallelTasksTool(task, registry)
	ctx, cancel := context.WithCancel(withCallContext(context.Background(), "parallel-parent", event.Discard, nil, false))
	result := make(chan error, 1)
	go func() {
		_, err := parallel.Execute(ctx, json.RawMessage(`{"tasks":[{"prompt":"one"},{"prompt":"two"}]}`))
		result <- err
	}()
	<-entered
	<-entered
	cancel()

	select {
	case err := <-result:
		close(release)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("parallel_tasks error = %v, want context cancellation", err)
		}
	case <-time.After(500 * time.Millisecond):
		close(release)
		<-result
		t.Fatal("parallel_tasks cancellation waited for a child tool with no active model call")
	}
}

func TestParallelTasksSchemaKeepsDependencyOrderingHidden(t *testing.T) {
	schema := string((&ParallelTasksTool{}).Schema())
	if strings.Contains(schema, "depends_on") {
		t.Fatal("parallel_tasks schema should not expose depends_on by default; changing tool schema hurts prompt-cache stability")
	}
}

func TestParallelTasksSchemaNeverExposesEffort(t *testing.T) {
	task := newTestTaskTool(t, &mockProvider{name: "p"}, tool.NewRegistry(), "", "", "", nil).
		WithSubagentEffortLevels([]string{"low", "medium", "high"})
	parallel := NewParallelTasksTool(task, tool.NewRegistry())

	schema := string(parallel.Schema())
	if strings.Contains(schema, `"effort"`) {
		t.Fatalf("parallel_tasks schema = %s, want no child task effort field", schema)
	}
}

func TestParallelTasksSchemaHidesEffortWhenUnsupported(t *testing.T) {
	task := newTestTaskTool(t, &mockProvider{name: "p"}, tool.NewRegistry(), "", "", "", nil).
		WithSubagentEffortLevels(nil)
	parallel := NewParallelTasksTool(task, tool.NewRegistry())

	schema := string(parallel.Schema())
	if strings.Contains(schema, `"effort"`) {
		t.Fatalf("parallel_tasks schema = %s, want no child task effort field", schema)
	}
}

func TestParallelTasksValidatesAllTasksBeforeRuntimeLookup(t *testing.T) {
	tool := &ParallelTasksTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"tasks": [
			{"prompt": "inspect the parser"},
			{"prompt": "   "}
		]
	}`))
	if err == nil {
		t.Fatal("Execute returned nil error for an empty later task")
	}
	if !strings.Contains(err.Error(), "task 2: prompt is required") {
		t.Fatalf("Execute error = %v, want task validation before runtime lookup", err)
	}
	if strings.Contains(err.Error(), "background jobs are not available") {
		t.Fatalf("Execute looked up background jobs before validating all tasks: %v", err)
	}
}

func TestParallelTasksRejectsDependencyCyclesBeforeRuntimeLookup(t *testing.T) {
	tool := &ParallelTasksTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"tasks": [
			{"prompt": "first", "depends_on": [1]},
			{"prompt": "second", "depends_on": [0]}
		]
	}`))
	if err == nil {
		t.Fatal("Execute returned nil error for cyclic dependencies")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("Execute error = %v, want dependency cycle validation", err)
	}
	if strings.Contains(err.Error(), "background jobs are not available") {
		t.Fatalf("Execute looked up background jobs before validating dependencies: %v", err)
	}
}

func TestParallelTasksForegroundCompletesAndClosesWorkers(t *testing.T) {
	task := newTestTaskTool(t, parallelStaticProvider{}, tool.NewRegistry(), "sys", "", "", nil)
	parallel := NewParallelTasksTool(task, tool.NewRegistry())
	ctx := withCallContext(context.Background(), "parallel-call", event.Discard, nil, false)

	done := make(chan error, 1)
	go func() {
		out, err := parallel.Execute(ctx, json.RawMessage(`{
			"tasks": [
				{"prompt": "first"},
				{"prompt": "second"}
			]
		}`))
		if err != nil {
			done <- err
			return
		}
		if !strings.Contains(out, "Completed 2 parallel tasks") {
			done <- stringsError("missing aggregate output: " + out)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel_tasks foreground execution did not return; workers likely waited on spawnCh forever")
	}
}

func TestParallelTasksIgnoresLegacyEffortForEveryBranch(t *testing.T) {
	profiles := make(chan [2]string, 2)
	task := newTestTaskTool(t, parallelStaticProvider{}, tool.NewRegistry(), "sys", "configured-model", "medium", func(model, effort string) (provider.Provider, *provider.Pricing, int, error) {
		profiles <- [2]string{model, effort}
		return parallelStaticProvider{}, nil, 0, nil
	})
	parallel := NewParallelTasksTool(task, tool.NewRegistry())
	ctx := withCallContext(context.Background(), "parallel-profile", event.Discard, nil, false)

	if _, err := parallel.Execute(ctx, json.RawMessage(`{
		"tasks": [
			{"prompt": "first", "effort": "high"},
			{"prompt": "second", "model": "override-model", "effort": "high"}
		]
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := map[[2]string]bool{<-profiles: true, <-profiles: true}
	for _, want := range [][2]string{{"configured-model", "medium"}, {"override-model", "medium"}} {
		if !got[want] {
			t.Fatalf("resolved profiles = %v, missing %q/%q", got, want[0], want[1])
		}
	}
}

func TestParallelTasksPropagatesToolTimeoutToSubagents(t *testing.T) {
	prov := &parallelToolTimeoutProvider{}
	reg := tool.NewRegistry()
	reg.Add(slowTool{})
	task := newTestTaskTool(t, prov, reg, "sys", "", "", nil).WithToolTimeout(25 * time.Millisecond)
	parallel := NewParallelTasksTool(task, reg)
	ctx, cancel := context.WithCancel(withCallContext(context.Background(), "parallel-timeout", event.Discard, nil, false))
	defer cancel()

	done := make(chan error, 1)
	go func() {
		out, err := parallel.Execute(ctx, json.RawMessage(`{
			"tasks": [
				{"prompt": "run the first slow tool", "tools": ["slow_tool"]},
				{"prompt": "run the second slow tool", "tools": ["slow_tool"]}
			]
		}`))
		if err == nil && !strings.Contains(out, "recovered after timeout") {
			err = stringsError("missing post-timeout child result: " + out)
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("parallel subagent ignored the configured 25ms tool timeout")
	}
}

func TestParallelTasksCancelReturnsPartialAggregate(t *testing.T) {
	task := newTestTaskTool(t, promptRoutingProvider{}, tool.NewRegistry(), "sys", "", "", nil)
	parallel := NewParallelTasksTool(task, tool.NewRegistry())

	ctx, cancel := context.WithCancel(withCallContext(context.Background(), "parallel-call", event.Discard, nil, false))
	defer cancel()
	done := make(chan struct {
		out string
		err error
	}, 1)
	go func() {
		out, err := parallel.Execute(ctx, json.RawMessage(`{
			"tasks": [
				{"prompt": "done child"},
				{"prompt": "stuck child"}
			]
		}`))
		done <- struct {
			out string
			err error
		}{out: out, err: err}
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("Execute error = %v, want context cancellation", got.err)
		}
		if strings.Contains(got.out, "Completed 2 parallel tasks") {
			t.Fatalf("cancelled aggregate reported full completion:\n%s", got.out)
		}
		if !strings.Contains(got.out, "done child ok") {
			t.Fatalf("cancelled aggregate lost completed child output:\n%s", got.out)
		}
		if !strings.Contains(strings.ToLower(got.out), "cancelled") {
			t.Fatalf("cancelled aggregate did not mark unfinished child:\n%s", got.out)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("parallel_tasks did not return promptly after cancellation")
	}
}

type parallelStaticProvider struct{}

func (parallelStaticProvider) Name() string { return "parallel-static" }

func (parallelStaticProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: "ok"}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

type parallelToolTimeoutProvider struct{}

func (*parallelToolTimeoutProvider) Name() string { return "parallel-tool-timeout" }

func (p *parallelToolTimeoutProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	hasToolResult := false
	for _, message := range req.Messages {
		if message.Role == provider.RoleTool {
			hasToolResult = true
			break
		}
	}
	if !hasToolResult {
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "slow-call", Name: "slow_tool", Arguments: `{"duration_ms":500}`}}
	} else {
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "recovered after timeout"}
	}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

type promptRoutingProvider struct{}

func (promptRoutingProvider) Name() string { return "prompt-routing" }

func (promptRoutingProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	if strings.Contains(lastUser(req), "stuck") {
		return make(chan provider.Chunk), nil
	}
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: lastUser(req) + " ok"}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

type stringsError string

func (e stringsError) Error() string { return string(e) }
