package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type usageProvider struct {
	usage *provider.Usage
}

func (p usageProvider) Name() string { return "usage" }

func (p usageProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 2)
	ch <- provider.Chunk{Type: provider.ChunkText, Text: "ok"}
	ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: p.usage}
	close(ch)
	return ch, nil
}

func TestTelemetryLoadsLegacyReadFileArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl.telemetry.json")
	if err := os.WriteFile(path, []byte(`[{"path":"README.md","turn":2,"time":1000}]`), 0o644); err != nil {
		t.Fatalf("write legacy telemetry: %v", err)
	}

	got := loadTelemetry(path)
	if len(got.ReadFiles) != 1 || got.ReadFiles[0].Path != "README.md" {
		t.Fatalf("legacy read files = %+v", got.ReadFiles)
	}
	if got.Usage.RequestCount != 0 {
		t.Fatalf("legacy usage request count = %d, want 0", got.Usage.RequestCount)
	}
}

func TestWorkspaceTabAggregatesSessionUsageTelemetry(t *testing.T) {
	tab := &WorkspaceTab{}
	start := time.Now().Add(-2 * time.Second).UnixMilli()
	tab.recordTurnStarted(start)
	tab.recordUsage(event.Event{
		Usage:       &provider.Usage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140, CacheHitTokens: 70, CacheMissTokens: 30, ReasoningTokens: 10},
		UsageSource: event.UsageSourceSubagent,
		SessionHit:  70,
		SessionMiss: 30,
		Pricing:     &provider.Pricing{CacheHit: 1, Input: 2, Output: 3, Currency: "¥"},
	})
	tab.recordTurnDone(start + 1500)

	got := tab.telemetrySnapshot().Usage
	if got.RequestCount != 1 || got.PromptTokens != 100 || got.CompletionTokens != 40 || got.TotalTokens != 140 || got.ReasoningTokens != 10 {
		t.Fatalf("usage tokens = %+v", got)
	}
	if got.CacheHitTokens != 70 || got.CacheMissTokens != 30 {
		t.Fatalf("cache tokens = hit %d miss %d", got.CacheHitTokens, got.CacheMissTokens)
	}
	if got.ElapsedMs != 1500 {
		t.Fatalf("elapsed = %d, want 1500", got.ElapsedMs)
	}
	if got.SessionCost <= 0 || got.SessionCurrency != "¥" {
		t.Fatalf("cost = %f %q, want positive ¥", got.SessionCost, got.SessionCurrency)
	}
	if got.Sources[event.UsageSourceSubagent].SessionCost <= 0 || got.Sources[event.UsageSourceSubagent].RequestCount != 1 {
		t.Fatalf("subagent source stats = %+v, want one costed request", got.Sources[event.UsageSourceSubagent])
	}

	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab": tab}}
	if context := app.ContextUsageForTab("tab"); context.SessionTokens != 140 {
		t.Fatalf("context usage session tokens = %d, want 140", context.SessionTokens)
	}
	if panel := app.ContextPanel("tab"); panel.TotalTokens != 140 {
		t.Fatalf("context panel total tokens = %d, want 140", panel.TotalTokens)
	}
}

func TestWorkspaceTabTracksActiveTurnUsageWithoutResettingDuplicateStart(t *testing.T) {
	tab := &WorkspaceTab{}
	tab.recordTurnStarted(1000)
	tab.recordUsage(event.Event{Usage: &provider.Usage{CompletionTokens: 12, TotalTokens: 12}})
	tab.recordTurnStarted(2000)
	tab.recordUsage(event.Event{Usage: &provider.Usage{CompletionTokens: 8, TotalTokens: 8}})

	sessionTokens, startedAt, completionTokens := tab.contextUsageSnapshot()
	if sessionTokens != 20 || startedAt != 1000 || completionTokens != 20 {
		t.Fatalf("active usage = session:%d started:%d completion:%d, want 20/1000/20", sessionTokens, startedAt, completionTokens)
	}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab": tab}}
	contextInfo := app.ContextUsageForTab("tab")
	if contextInfo.ActiveTurnStartedAt != 1000 || contextInfo.ActiveTurnCompletionTokens != 20 {
		t.Fatalf("context active usage = started:%d completion:%d, want 1000/20", contextInfo.ActiveTurnStartedAt, contextInfo.ActiveTurnCompletionTokens)
	}
	meta := app.tabMeta(tab, true)
	if meta.ActiveTurnStartedAt != 1000 || meta.ActiveTurnCompletionTokens != 20 {
		t.Fatalf("tab meta active usage = started:%d completion:%d, want 1000/20", meta.ActiveTurnStartedAt, meta.ActiveTurnCompletionTokens)
	}
	persisted := tab.telemetrySnapshot().Usage
	if persisted.activeTurnStartedAt != 0 || persisted.activeTurnCompletionTokens != 0 {
		t.Fatalf("persisted active usage = started:%d completion:%d, want 0/0", persisted.activeTurnStartedAt, persisted.activeTurnCompletionTokens)
	}

	tab.recordTurnDone(2500)
	_, startedAt, completionTokens = tab.contextUsageSnapshot()
	if startedAt != 0 || completionTokens != 0 {
		t.Fatalf("completed active usage = started:%d completion:%d, want 0/0", startedAt, completionTokens)
	}
}

func TestTabEventSinkEmitsAuthoritativeActiveTurnMetrics(t *testing.T) {
	tab := &WorkspaceTab{ID: "tab"}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab": tab}}
	sink := &tabEventSink{tabID: "tab", app: app, ctx: context.Background()}
	delivered := make(chan wireEventTab, 2)
	sink.runtimeEvents.emit = func(_ context.Context, _ string, payload ...interface{}) {
		delivered <- payload[0].(wireEventTab)
	}

	sink.Emit(event.Event{Kind: event.TurnStarted})
	sink.Emit(event.Event{
		Kind:  event.Usage,
		Usage: &provider.Usage{CompletionTokens: 17, TotalTokens: 17},
	})

	started := <-delivered
	usage := <-delivered
	if started.ActiveTurnStartedAt <= 0 || started.ActiveTurnCompletionTokens != 0 {
		t.Fatalf("turn_started metrics = started:%d completion:%d, want positive/0", started.ActiveTurnStartedAt, started.ActiveTurnCompletionTokens)
	}
	if usage.ActiveTurnStartedAt != started.ActiveTurnStartedAt || usage.ActiveTurnCompletionTokens != 17 {
		t.Fatalf("usage metrics = started:%d completion:%d, want %d/17", usage.ActiveTurnStartedAt, usage.ActiveTurnCompletionTokens, started.ActiveTurnStartedAt)
	}
}

func TestApplyRuntimeTabPreservesActiveTurnUsage(t *testing.T) {
	source := &WorkspaceTab{}
	source.recordTurnStarted(1000)
	source.recordUsage(event.Event{Usage: &provider.Usage{CompletionTokens: 7, TotalTokens: 7}})
	target := &WorkspaceTab{ID: "target"}

	applyRuntimeTab(target, source, "session-key", context.Background(), NewApp())

	_, startedAt, completionTokens := target.contextUsageSnapshot()
	if startedAt != 1000 || completionTokens != 7 {
		t.Fatalf("reattached active usage = started:%d completion:%d, want 1000/7", startedAt, completionTokens)
	}
}

func TestWorkspaceTabSubagentUsageDoesNotOverwriteExecutorSessionCache(t *testing.T) {
	tab := &WorkspaceTab{}
	tab.recordUsage(event.Event{
		Usage:       &provider.Usage{PromptTokens: 1000, CompletionTokens: 10, TotalTokens: 1010, CacheHitTokens: 0, CacheMissTokens: 0},
		UsageSource: event.UsageSourceExecutor,
		SessionHit:  700,
		SessionMiss: 300,
	})
	tab.recordUsage(event.Event{
		Usage:       &provider.Usage{PromptTokens: 20, CompletionTokens: 5, TotalTokens: 25, CacheHitTokens: 5, CacheMissTokens: 10},
		UsageSource: event.UsageSourceSubagent,
		SessionHit:  999,
		SessionMiss: 999,
	})
	tab.recordUsage(event.Event{
		Usage:       &provider.Usage{PromptTokens: 200, CompletionTokens: 20, TotalTokens: 220, CacheHitTokens: 100, CacheMissTokens: 100},
		UsageSource: event.UsageSourceExecutor,
		SessionHit:  800,
		SessionMiss: 400,
	})

	got := tab.telemetrySnapshot().Usage
	if got.CacheHitTokens != 805 || got.CacheMissTokens != 410 {
		t.Fatalf("cache tokens = hit %d miss %d, want executor deltas plus subagent delta 805/410", got.CacheHitTokens, got.CacheMissTokens)
	}
	if got.Sources[event.UsageSourceSubagent].CacheHitTokens != 5 || got.Sources[event.UsageSourceSubagent].CacheMissTokens != 10 {
		t.Fatalf("subagent cache source = %+v, want usage delta 5/10", got.Sources[event.UsageSourceSubagent])
	}
}

func TestContextPanelUsesLastUsageBreakdownWithTelemetryTotal(t *testing.T) {
	lastUsage := &provider.Usage{
		PromptTokens:     10,
		CompletionTokens: 4,
		TotalTokens:      14,
		CacheHitTokens:   7,
		CacheMissTokens:  3,
		ReasoningTokens:  2,
	}
	ag := agent.New(
		usageProvider{usage: lastUsage},
		tool.NewRegistry(),
		agent.NewSession("system"),
		agent.Options{ContextWindow: 200},
		event.Discard,
	)
	if err := ag.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	tab := &WorkspaceTab{
		ID:    "tab",
		Ctrl:  control.New(control.Options{Executor: ag, Sink: event.Discard}),
		Scope: "global",
		Ready: true,
	}
	tab.recordUsage(event.Event{
		Usage: &provider.Usage{
			PromptTokens:     100,
			CompletionTokens: 40,
			TotalTokens:      140,
			CacheHitTokens:   70,
			CacheMissTokens:  30,
			ReasoningTokens:  10,
		},
	})
	app := &App{tabRuntimes: map[string]*WorkspaceTab{"tab": tab}}

	panel := app.ContextPanel("tab")
	if panel.TotalTokens != 140 {
		t.Fatalf("context panel total tokens = %d, want telemetry total 140", panel.TotalTokens)
	}
	if panel.PromptTokens != 10 || panel.CompletionTokens != 4 || panel.ReasoningTokens != 2 {
		t.Fatalf("context panel breakdown = prompt:%d completion:%d reasoning:%d, want last usage 10/4/2",
			panel.PromptTokens, panel.CompletionTokens, panel.ReasoningTokens)
	}
	if panel.CacheHitTokens != 7 || panel.CacheMissTokens != 3 {
		t.Fatalf("context panel cache breakdown = hit:%d miss:%d, want last usage 7/3",
			panel.CacheHitTokens, panel.CacheMissTokens)
	}
}
