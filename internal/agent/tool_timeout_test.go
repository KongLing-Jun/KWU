package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type timeoutTestTool struct {
	name     string
	release  <-chan struct{}
	canceled chan<- struct{}
	ignore   bool
}

type budgetedTimeoutTestTool struct {
	timeoutTestTool
	steps int
}

type exportedBudgetedTimeoutTestTool struct {
	timeoutTestTool
	steps int
}

type noTimeoutTestTool struct {
	timeoutTestTool
}

func (t budgetedTimeoutTestTool) toolTimeoutBudget(base time.Duration, _ json.RawMessage) time.Duration {
	return delegatedTaskTimeoutBudget(base, t.steps)
}

func (t exportedBudgetedTimeoutTestTool) ToolTimeoutBudget(base time.Duration, _ json.RawMessage) time.Duration {
	return delegatedTaskTimeoutBudget(base, t.steps)
}

func (noTimeoutTestTool) ToolTimeoutBudget(time.Duration, json.RawMessage) time.Duration {
	return tool.NoTimeoutBudget
}

func (t timeoutTestTool) Name() string          { return t.name }
func (timeoutTestTool) Description() string     { return "timeout test tool" }
func (timeoutTestTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (timeoutTestTool) ReadOnly() bool          { return true }

func (t timeoutTestTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	if t.ignore {
		<-t.release
		return "late", nil
	}
	select {
	case <-ctx.Done():
		if t.canceled != nil {
			close(t.canceled)
		}
		return "", context.Cause(ctx)
	case <-t.release:
		return "ok", nil
	}
}

func executeTimeoutTestCall(a *Agent, name string) toolOutcome {
	return a.executeOne(context.Background(), provider.ToolCall{ID: "call-1", Name: name, Arguments: `{}`})
}

func TestToolTimeoutCancelsCooperativeTool(t *testing.T) {
	canceled := make(chan struct{})
	reg := tool.NewRegistry()
	reg.Add(timeoutTestTool{name: "slow", canceled: canceled})
	a := New(nil, reg, NewSession(""), Options{ToolTimeout: 25 * time.Millisecond}, nil)

	out := executeTimeoutTestCall(a, "slow")
	if !strings.Contains(out.errMsg, `tool "slow" timed out after 25ms`) {
		t.Fatalf("errMsg = %q", out.errMsg)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("cooperative tool did not observe cancellation")
	}
}

func TestToolTimeoutUnblocksWhenToolIgnoresContext(t *testing.T) {
	release := make(chan struct{})
	reg := tool.NewRegistry()
	reg.Add(timeoutTestTool{name: "stuck", release: release, ignore: true})
	a := New(nil, reg, NewSession(""), Options{ToolTimeout: 25 * time.Millisecond}, nil)

	started := time.Now()
	out := executeTimeoutTestCall(a, "stuck")
	close(release)
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("tool call stayed blocked for %s", elapsed)
	}
	if !strings.Contains(out.errMsg, `tool "stuck" timed out after 25ms`) {
		t.Fatalf("errMsg = %q", out.errMsg)
	}
}

func TestToolTimeoutReturnsCompletedResult(t *testing.T) {
	release := make(chan struct{})
	close(release)
	reg := tool.NewRegistry()
	reg.Add(timeoutTestTool{name: "fast", release: release})
	a := New(nil, reg, NewSession(""), Options{ToolTimeout: time.Second}, nil)

	out := executeTimeoutTestCall(a, "fast")
	if out.errMsg != "" || out.output != "ok" {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestDelegatedTaskTimeoutUsesFiniteMultiStepBudget(t *testing.T) {
	release := make(chan struct{})
	reg := tool.NewRegistry()
	reg.Add(budgetedTimeoutTestTool{
		timeoutTestTool: timeoutTestTool{name: "delegated", release: release},
		steps:           4,
	})
	a := New(nil, reg, NewSession(""), Options{ToolTimeout: 50 * time.Millisecond}, nil)

	go func() {
		time.Sleep(75 * time.Millisecond)
		close(release)
	}()
	out := executeTimeoutTestCall(a, "delegated")
	if out.errMsg != "" || out.output != "ok" {
		t.Fatalf("delegated outcome = %+v, want completion within 200ms multi-step budget", out)
	}
}

func TestToolTimeoutUsesExportedToolBudget(t *testing.T) {
	release := make(chan struct{})
	reg := tool.NewRegistry()
	reg.Add(exportedBudgetedTimeoutTestTool{
		timeoutTestTool: timeoutTestTool{name: "long-running", release: release},
		steps:           4,
	})
	a := New(nil, reg, NewSession(""), Options{ToolTimeout: 50 * time.Millisecond}, nil)

	go func() {
		time.Sleep(75 * time.Millisecond)
		close(release)
	}()
	out := executeTimeoutTestCall(a, "long-running")
	if out.errMsg != "" || out.output != "ok" {
		t.Fatalf("exported-budget outcome = %+v, want completion within 200ms budget", out)
	}
}

func TestToolCanWaitUntilParentCancellationWithoutPerCallTimeout(t *testing.T) {
	release := make(chan struct{})
	reg := tool.NewRegistry()
	reg.Add(noTimeoutTestTool{timeoutTestTool{name: "wait-until-done", release: release}})
	a := New(nil, reg, NewSession(""), Options{ToolTimeout: 25 * time.Millisecond}, nil)

	go func() {
		time.Sleep(75 * time.Millisecond)
		close(release)
	}()
	out := executeTimeoutTestCall(a, "wait-until-done")
	if out.errMsg != "" || out.output != "ok" {
		t.Fatalf("no-timeout outcome = %+v, want completion after the ordinary timeout", out)
	}
}

func TestDelegatedTaskTimeoutBudgetIsCapped(t *testing.T) {
	base := 2 * time.Second
	if got, want := delegatedTaskTimeoutBudget(base, 1000), base*maxDelegatedTaskTimeoutSteps; got != want {
		t.Fatalf("budget = %s, want capped %s", got, want)
	}
}

func TestToolTimeoutPreservesShorterParentDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := executeToolWithTimeout(ctx, "slow", time.Second, func(callCtx context.Context) (string, error) {
		<-callCtx.Done()
		return "", context.Cause(callCtx)
	})
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("parent deadline was extended: %s", elapsed)
	}
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("err = %v", err)
	}
}

func TestToolTimeoutEmitsTerminalToolResult(t *testing.T) {
	prov := &mockProvider{name: "p", streams: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "timeout-call", Name: "slow", Arguments: `{}`}},
			{Type: provider.ChunkDone},
		},
		{
			{Type: provider.ChunkText, Text: "recovered"},
			{Type: provider.ChunkDone},
		},
	}}
	reg := tool.NewRegistry()
	reg.Add(timeoutTestTool{name: "slow"})
	sink := &recordSink{}
	a := New(prov, reg, NewSession(""), Options{ToolTimeout: 25 * time.Millisecond}, sink)
	if err := a.Run(context.Background(), "run the tool"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, e := range sink.kinds(event.ToolResult) {
		if e.Tool.ID == "timeout-call" && e.Tool.Err != "" {
			return
		}
	}
	t.Fatal("timed-out call did not emit a terminal ToolResult")
}
