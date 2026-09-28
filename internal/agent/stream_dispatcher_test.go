package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type dispatcherBoundaryTool struct{ err error }

func (dispatcherBoundaryTool) Name() string            { return "derived" }
func (dispatcherBoundaryTool) Description() string     { return "derived call" }
func (dispatcherBoundaryTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (dispatcherBoundaryTool) ReadOnly() bool          { return false }
func (t dispatcherBoundaryTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "", t.err
}

type dispatcherBoundaryGate struct{ err error }

func (g dispatcherBoundaryGate) Check(context.Context, string, json.RawMessage, bool) (bool, string, error) {
	return false, "", g.err
}

type dispatcherTestProvider struct{ calls int }

func (p *dispatcherTestProvider) Name() string { return "base" }
func (p *dispatcherTestProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	p.calls++
	ch := make(chan provider.Chunk, 1)
	close(ch)
	return ch, nil
}

type dispatcherTestStream struct {
	calls int
	base  provider.Provider
	reply string
}

func (d *dispatcherTestStream) Stream(_ context.Context, base provider.Provider, _ provider.Request) (<-chan provider.Chunk, error) {
	d.calls++
	d.base = base
	ch := make(chan provider.Chunk, 1)
	if d.reply == "" {
		close(ch)
		return ch, nil
	}
	ch <- provider.Chunk{Type: provider.ChunkText, Text: d.reply}
	close(ch)
	return ch, nil
}

type nilChunkTerminalDispatcher struct{}

func (nilChunkTerminalDispatcher) Stream(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, error) {
	return nil, nil
}

func (nilChunkTerminalDispatcher) StreamWithTerminalResult(context.Context, provider.Provider, provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	return nil, nil, nil
}

type nilChunkProvider struct{}

func (nilChunkProvider) Name() string { return "legacy-nil" }
func (nilChunkProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	return nil, nil
}

func TestAgentDispatchersRejectNilChunkChannel(t *testing.T) {
	t.Run("main", func(t *testing.T) {
		a := New(&dispatcherTestProvider{}, tool.NewRegistry(), NewSession("system"), Options{
			StreamDispatcher: nilChunkTerminalDispatcher{},
		}, event.Discard)
		if _, _, _, _, _, _, _, err := a.stream(context.Background(), 1, nil); !errors.Is(err, errStreamDispatcherNilChannel) {
			t.Fatalf("Agent.stream error = %v, want nil-channel contract error", err)
		}
	})

	t.Run("derived", func(t *testing.T) {
		a := New(&dispatcherTestProvider{}, tool.NewRegistry(), NewSession("system"), Options{
			DerivedStreamDispatcher: nilChunkTerminalDispatcher{},
		}, event.Discard)
		if _, err := a.summarize(context.Background(), []provider.Message{{Role: provider.RoleUser, Content: "old"}}, ""); !errors.Is(err, errStreamDispatcherNilChannel) {
			t.Fatalf("Agent.summarize error = %v, want nil-channel contract error", err)
		}
	})
}

func TestAgentDispatchStreamKeepsLegacyNilProviderResult(t *testing.T) {
	a := New(nilChunkProvider{}, tool.NewRegistry(), NewSession("system"), Options{}, event.Discard)
	chunks, terminalResult, err := a.dispatchStream(context.Background(), provider.Request{})
	if err != nil || chunks != nil || terminalResult != nil {
		t.Fatalf("legacy dispatch result = (%v, %v, %v), want unchanged nil result", chunks, terminalResult, err)
	}
}

func TestAgentStreamUsesCallScopedDispatcher(t *testing.T) {
	base := &dispatcherTestProvider{}
	dispatcher := &dispatcherTestStream{}
	a := New(base, tool.NewRegistry(), NewSession("system"), Options{StreamDispatcher: dispatcher}, event.Discard)
	if _, _, _, _, _, _, _, err := a.stream(context.Background(), 1, nil); err != nil {
		t.Fatal(err)
	}
	if dispatcher.calls != 1 || dispatcher.base != base || base.calls != 0 {
		t.Fatalf("dispatcher/base calls = %d/%d; want dispatcher once and base never directly called", dispatcher.calls, base.calls)
	}
}

func TestAgentStreamUsesContextDispatcherOverrideForOneLogicalCall(t *testing.T) {
	base := &dispatcherTestProvider{}
	configured := &dispatcherTestStream{}
	override := &dispatcherTestStream{}
	a := New(base, tool.NewRegistry(), NewSession("system"), Options{StreamDispatcher: configured}, event.Discard)
	ctx := WithStreamDispatcher(context.Background(), a, override)
	if _, _, _, _, _, _, _, err := a.stream(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}
	if override.calls != 1 || override.base != base || configured.calls != 0 || base.calls != 0 {
		t.Fatalf("override/configured/base calls = %d/%d/%d", override.calls, configured.calls, base.calls)
	}
	if _, _, _, _, _, _, _, err := a.stream(context.Background(), 2, nil); err != nil {
		t.Fatal(err)
	}
	if configured.calls != 1 || override.calls != 1 {
		t.Fatalf("configured/override calls after normal call = %d/%d", configured.calls, override.calls)
	}
}

func TestAgentStreamDispatcherOverrideIsBoundToTargetAgent(t *testing.T) {
	targetBase := &dispatcherTestProvider{}
	otherBase := &dispatcherTestProvider{}
	targetConfigured := &dispatcherTestStream{}
	otherConfigured := &dispatcherTestStream{}
	override := &dispatcherTestStream{}
	target := New(targetBase, tool.NewRegistry(), NewSession("target"), Options{StreamDispatcher: targetConfigured}, event.Discard)
	other := New(otherBase, tool.NewRegistry(), NewSession("other"), Options{StreamDispatcher: otherConfigured}, event.Discard)
	ctx := WithStreamDispatcher(context.Background(), target, override)

	if _, _, _, _, _, _, _, err := other.stream(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, _, _, _, err := target.stream(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}
	if otherConfigured.calls != 1 || targetConfigured.calls != 0 || override.calls != 1 {
		t.Fatalf("other/target/override calls = %d/%d/%d", otherConfigured.calls, targetConfigured.calls, override.calls)
	}
}

func TestAgentSummarizeUsesDedicatedDerivedDispatcher(t *testing.T) {
	base := &dispatcherTestProvider{}
	main := &dispatcherTestStream{}
	compaction := &dispatcherTestStream{reply: "summary"}
	a := New(base, tool.NewRegistry(), NewSession("system"), Options{
		StreamDispatcher:        main,
		DerivedStreamDispatcher: compaction,
	}, event.Discard)
	got, err := a.summarize(context.Background(), []provider.Message{{Role: provider.RoleUser, Content: "old"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "summary" || compaction.calls != 1 || compaction.base != base || main.calls != 0 || base.calls != 0 {
		t.Fatalf("summary/calls = %q/%d/%d/%d", got, compaction.calls, main.calls, base.calls)
	}
}

func TestAgentStreamChunkErrorPreservesFollowingDispatcherTerminalFailure(t *testing.T) {
	want := errors.New("usage terminal persistence failed")
	dispatcher := &compactionChunkThenTerminalDispatcher{err: want}
	a := New(&dispatcherTestProvider{}, tool.NewRegistry(), NewSession("system"), Options{StreamDispatcher: dispatcher}, event.Discard)

	_, _, _, _, _, _, _, err := a.stream(context.Background(), 1, nil)
	if !errors.Is(err, want) || !IsStreamDispatcherTerminalError(err) {
		t.Fatalf("Agent.stream error = %v, want terminal persistence failure", err)
	}
	if dispatcher.calls != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", dispatcher.calls)
	}
}

func TestAgentPropagatesDispatcherCloseTimeoutFromToolAndGateBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		gate Gate
		tool tool.Tool
	}{
		{name: "tool", tool: dispatcherBoundaryTool{err: ErrMainStreamDispatcherCloseTimeout}},
		{name: "gate", gate: dispatcherBoundaryGate{err: ErrMainStreamDispatcherCloseTimeout}, tool: dispatcherBoundaryTool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prov := &mockProvider{name: "root", streams: [][]provider.Chunk{
				{{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{ID: "derived-call", Name: "derived", Arguments: `{}`}}, {Type: provider.ChunkDone}},
				{{Type: provider.ChunkText, Text: "must not continue"}, {Type: provider.ChunkDone}},
			}}
			registry := tool.NewRegistry()
			registry.Add(tc.tool)
			agentRunner := New(prov, registry, NewSession("system"), Options{Gate: tc.gate}, event.Discard)

			err := agentRunner.Run(context.Background(), "run derived call")
			if !errors.Is(err, ErrMainStreamDispatcherCloseTimeout) {
				t.Fatalf("Agent.Run error = %v", err)
			}
			if len(prov.requests) != 1 {
				t.Fatalf("provider requests = %d, want no fallback call after close-timeout", len(prov.requests))
			}
		})
	}
}

type cancellationBarrierDispatcher struct {
	initialDelivered chan struct{}
	drainObserved    chan struct{}
	release          chan struct{}
	abort            chan struct{}
}

func (d *cancellationBarrierDispatcher) Stream(ctx context.Context, _ provider.Provider, _ provider.Request) (<-chan provider.Chunk, error) {
	out := make(chan provider.Chunk)
	go func() {
		defer close(out)
		out <- provider.Chunk{Type: provider.ChunkText, Text: "partial"}
		close(d.initialDelivered)
		<-ctx.Done()
		select {
		case out <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{}}:
			close(d.drainObserved)
		case <-d.abort:
			return
		}
		select {
		case <-d.release:
		case <-d.abort:
		}
	}()
	return out, nil
}

func TestAgentStreamWithDispatcherDrainsAfterCancellationBeforeReturning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatcher := &cancellationBarrierDispatcher{
		initialDelivered: make(chan struct{}),
		drainObserved:    make(chan struct{}),
		release:          make(chan struct{}),
		abort:            make(chan struct{}),
	}
	defer close(dispatcher.abort)
	a := New(&dispatcherTestProvider{}, tool.NewRegistry(), NewSession("system"), Options{StreamDispatcher: dispatcher}, event.Discard)
	result := make(chan error, 1)
	go func() {
		_, _, _, _, _, _, _, err := a.stream(ctx, 1, nil)
		result <- err
	}()
	select {
	case <-dispatcher.initialDelivered:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not deliver the initial chunk")
	}
	cancel()
	select {
	case <-dispatcher.drainObserved:
	case <-time.After(time.Second):
		t.Fatal("agent returned on cancellation without draining the dispatcher")
	}
	select {
	case err := <-result:
		t.Fatalf("agent returned before the dispatcher closed: %v", err)
	default:
	}
	close(dispatcher.release)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stream error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("agent did not return after dispatcher close")
	}
}

func TestWaitForStreamCloseIsBounded(t *testing.T) {
	ch := make(chan provider.Chunk)
	if err := waitForStreamClose(ch, time.Millisecond); !errors.Is(err, ErrMainStreamDispatcherCloseTimeout) {
		t.Fatalf("wait error = %v, want dispatcher close timeout", err)
	}
}

func TestMainStreamDispatcherCloseTimeoutExceedsTerminalPersistenceBudget(t *testing.T) {
	const terminalPersistenceBudget = 2 * 5 * time.Second
	if mainStreamDispatcherCloseTimeout <= terminalPersistenceBudget {
		t.Fatalf("dispatcher close timeout = %s, must exceed terminal persistence budget %s", mainStreamDispatcherCloseTimeout, terminalPersistenceBudget)
	}
}

type legacyCancellationProvider struct {
	initialDelivered chan struct{}
	release          chan struct{}
}

func (*legacyCancellationProvider) Name() string { return "legacy" }

func (p *legacyCancellationProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	out := make(chan provider.Chunk)
	go func() {
		defer close(out)
		out <- provider.Chunk{Type: provider.ChunkText, Text: "partial"}
		close(p.initialDelivered)
		<-p.release
	}()
	return out, nil
}

func TestAgentStreamWithoutDispatcherKeepsLegacyCancellationBehavior(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := &legacyCancellationProvider{initialDelivered: make(chan struct{}), release: make(chan struct{})}
	defer close(provider.release)
	a := New(provider, tool.NewRegistry(), NewSession("system"), Options{}, event.Discard)
	result := make(chan error, 1)
	go func() {
		_, _, _, _, _, _, _, err := a.stream(ctx, 1, nil)
		result <- err
	}()
	select {
	case <-provider.initialDelivered:
	case <-time.After(time.Second):
		t.Fatal("legacy provider did not deliver the initial chunk")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stream error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("legacy provider path waited for an unclosed stream")
	}
}
