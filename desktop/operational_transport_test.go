package main

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/event"
	"kwu/internal/eventwire"
)

func TestOperationalControlLanePreemptsLegacyBacklog(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan string, 3)
	var calls atomic.Int32
	emitter := &asyncRuntimeEmitter{}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		delivered <- payload[0].(string)
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
	}
	ctx := context.Background()
	emitter.Emit(ctx, eventChannel, "legacy-one")
	waitSignal(t, entered, "legacy emitter start")
	emitter.Emit(ctx, eventChannel, "legacy-two")
	if got := emitter.EmitControl(ctx, eventChannel, "control"); got != controlEnqueueAccepted {
		t.Fatalf("control enqueue = %q", got)
	}
	close(release)
	for _, want := range []string{"legacy-one", "control", "legacy-two"} {
		if got := waitString(t, delivered, want); got != want {
			t.Fatalf("delivery = %q, want %q", got, want)
		}
	}
	waitControlStats(t, emitter, func(stats runtimeControlStats) bool { return stats.Emitted == 1 })
}

func TestOperationalControlLaneIsBoundedAndObservable(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan string, 3)
	var calls atomic.Int32
	emitter := &asyncRuntimeEmitter{controlCapacity: 2}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		delivered <- payload[0].(string)
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
	}
	ctx := context.Background()
	if got := emitter.EmitControl(ctx, eventChannel, "one"); got != controlEnqueueAccepted {
		t.Fatalf("first enqueue = %q", got)
	}
	waitSignal(t, entered, "first control delivery")
	for _, value := range []string{"two", "three"} {
		if got := emitter.EmitControl(ctx, eventChannel, value); got != controlEnqueueAccepted {
			t.Fatalf("enqueue %s = %q", value, got)
		}
	}
	if got := emitter.EmitControl(ctx, eventChannel, "overflow"); got != controlEnqueueQueueFull {
		t.Fatalf("overflow enqueue = %q", got)
	}
	close(release)
	for _, want := range []string{"one", "two", "three"} {
		if got := waitString(t, delivered, want); got != want {
			t.Fatalf("delivery = %q, want %q", got, want)
		}
	}
	waitControlStats(t, emitter, func(stats runtimeControlStats) bool { return stats.Emitted == 3 })
	if stats := emitter.ControlStats(); stats.Accepted != 3 || stats.QueueFull != 1 || stats.QueueDepth != 0 || stats.QueueHighWater != 2 || stats.QueueWaitSamples != 3 || stats.DeliverySamples != 3 {
		t.Fatalf("control stats = %+v", stats)
	}
}

func TestOperationalControlLaneSlowConsumerIsBoundedAndNonBlocking(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	emitter := &asyncRuntimeEmitter{controlCapacity: 4}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		if payload[0].(string) == "blocker" {
			close(entered)
			<-release
		}
	}
	ctx := context.Background()
	if got := emitter.EmitControl(ctx, eventChannel, "blocker"); got != controlEnqueueAccepted {
		t.Fatalf("blocker enqueue = %q", got)
	}
	waitSignal(t, entered, "slow consumer")
	started := time.Now()
	for _, value := range []string{"one", "two", "three", "four"} {
		if got := emitter.EmitControl(ctx, eventChannel, value); got != controlEnqueueAccepted {
			t.Fatalf("enqueue %s = %q", value, got)
		}
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("enqueue blocked behind slow consumer for %s", elapsed)
	}
	if got := emitter.EmitControl(ctx, eventChannel, "overflow"); got != controlEnqueueQueueFull {
		t.Fatalf("overflow enqueue = %q", got)
	}
	if stats := emitter.ControlStats(); stats.QueueDepth != 4 || stats.QueueHighWater != 4 || stats.QueueFull != 1 {
		t.Fatalf("saturated stats = %+v", stats)
	}
	time.Sleep(5 * time.Millisecond)
	close(release)
	waitControlStats(t, emitter, func(stats runtimeControlStats) bool { return stats.Emitted == 5 })
	if stats := emitter.ControlStats(); stats.QueueDepth != 0 || stats.QueueWaitSamples != 5 || stats.DeliverySamples != 5 || stats.QueueWaitMaxMicros == 0 || stats.DeliveryMaxMicros == 0 {
		t.Fatalf("drained stats = %+v", stats)
	}
}

func TestOperationalControlLaneSupersedesSameAggregateForTerminalConvergence(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan wireEventTab, 3)
	emitter := &asyncRuntimeEmitter{controlCapacity: 2}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		wire := payload[0].(wireEventTab)
		delivered <- wire
		if wire.Operational.Aggregate.ID == "blocker" {
			close(entered)
			<-release
		}
	}
	ctx := context.Background()
	emitter.EmitControl(ctx, eventChannel, operationalControlWire("blocker", event.PromptEmitted))
	waitSignal(t, entered, "terminal convergence blocker")
	emitter.EmitControl(ctx, eventChannel, operationalControlWire("prompt-A", event.PromptEmitted))
	emitter.EmitControl(ctx, eventChannel, operationalControlWire("prompt-B", event.PromptEmitted))
	if got := emitter.EmitControl(ctx, eventChannel, operationalControlWire("prompt-A", event.PromptResolved)); got != controlEnqueueAccepted {
		t.Fatalf("terminal replacement = %q", got)
	}
	close(release)
	first, second, third := waitControlWire(t, delivered, "blocker"), waitControlWire(t, delivered, "terminal"), waitControlWire(t, delivered, "remaining")
	if first.Operational.Aggregate.ID != "blocker" || second.Operational.Aggregate.ID != "prompt-A" || second.Operational.Kind != string(event.PromptResolved) || third.Operational.Aggregate.ID != "prompt-B" {
		t.Fatalf("unexpected convergence order")
	}
	waitControlStats(t, emitter, func(stats runtimeControlStats) bool { return stats.Emitted == 3 })
	if stats := emitter.ControlStats(); stats.Superseded != 1 || stats.QueueFull != 0 {
		t.Fatalf("control stats = %+v", stats)
	}
}

func TestOperationalControlLaneClassifiesLateConnectionEpoch(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan string, 2)
	emitter := &asyncRuntimeEmitter{}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		value := payload[0].(string)
		delivered <- value
		if value == "legacy-blocker" {
			close(entered)
			<-release
		}
	}
	ctx := context.Background()
	emitter.Emit(ctx, eventChannel, "legacy-blocker")
	waitSignal(t, entered, "legacy blocker")
	if got := emitter.EmitControl(ctx, eventChannel, "old-control"); got != controlEnqueueAccepted {
		t.Fatalf("control enqueue = %q", got)
	}
	emitter.BeginConnectionEpoch()
	close(release)
	if got := waitString(t, delivered, "legacy blocker"); got != "legacy-blocker" {
		t.Fatalf("legacy delivery = %q", got)
	}
	waitControlStats(t, emitter, func(stats runtimeControlStats) bool { return stats.Late == 1 })
	select {
	case got := <-delivered:
		t.Fatalf("late control was delivered: %q", got)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestOperationalControlLaneIsolatesEmitterPanic(t *testing.T) {
	delivered := make(chan string, 1)
	var calls atomic.Int32
	emitter := &asyncRuntimeEmitter{}
	emitter.emit = func(_ context.Context, _ string, payload ...interface{}) {
		if calls.Add(1) == 1 {
			panic("synthetic runtime failure")
		}
		delivered <- payload[0].(string)
	}
	ctx := context.Background()
	emitter.EmitControl(ctx, eventChannel, "panic")
	emitter.EmitControl(ctx, eventChannel, "survivor")
	if got := waitString(t, delivered, "post-panic delivery"); got != "survivor" {
		t.Fatalf("delivery = %q", got)
	}
	waitControlStats(t, emitter, func(stats runtimeControlStats) bool { return stats.EmitFailed == 1 && stats.Emitted == 1 })
}

func TestOperationalControlLaneRejectsDisabledContext(t *testing.T) {
	emitter := &asyncRuntimeEmitter{}
	if got := emitter.EmitControl(nil, eventChannel, "ignored"); got != controlEnqueueDisabled {
		t.Fatalf("nil-context enqueue = %q", got)
	}
	if stats := emitter.ControlStats(); stats.Disabled != 1 || stats.Accepted != 0 {
		t.Fatalf("control stats = %+v", stats)
	}
}

func TestOperationalTerminalClassificationCoversEveryAggregate(t *testing.T) {
	terminals := []event.OperationalKind{
		event.PromptResolved, event.TaskCompleted, event.ModelCallCompleted,
		event.ToolCallCompleted, event.SessionClosed, event.DashboardWriteCommitted, event.PlanExited,
	}
	for _, kind := range terminals {
		if !isOperationalTerminal(kind) {
			t.Errorf("%q was not classified terminal", kind)
		}
	}
	for _, kind := range []event.OperationalKind{event.PromptEmitted, event.TaskRunning, event.ModelCallRunning, event.ToolCallProgress, event.SessionActivated, event.DashboardWriteRequested, event.PlanApproved} {
		if isOperationalTerminal(kind) {
			t.Errorf("%q was classified terminal", kind)
		}
	}
}

func BenchmarkOperationalControlLaneEnqueue(b *testing.B) {
	emitter := &asyncRuntimeEmitter{controlCapacity: b.N + 1}
	emitter.emit = func(context.Context, string, ...interface{}) {}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if emitter.EmitControl(ctx, eventChannel, "benchmark") != controlEnqueueAccepted {
			b.Fatal("control lane unexpectedly saturated")
		}
	}
	b.StopTimer()
	deadline := time.Now().Add(5 * time.Second)
	for emitter.ControlStats().Emitted != uint64(b.N) && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if stats := emitter.ControlStats(); stats.Emitted != uint64(b.N) || stats.QueueDepth != 0 {
		b.Fatalf("control lane did not drain: %+v", stats)
	}
}

func operationalControlWire(aggregateID string, kind event.OperationalKind) wireEventTab {
	return wireEventTab{Event: eventwire.Event{Operational: &eventwire.OperationalEnvelope{Aggregate: eventwire.OperationalAggregate{Kind: "prompt", ID: aggregateID}, Kind: string(kind)}}}
}

func waitControlWire(t *testing.T, values <-chan wireEventTab, label string) wireEventTab {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
		return wireEventTab{}
	}
}
func waitSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}
func waitString(t *testing.T, values <-chan string, label string) string {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
		return ""
	}
}
func waitControlStats(t *testing.T, emitter *asyncRuntimeEmitter, ready func(runtimeControlStats) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ready(emitter.ControlStats()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for control stats: %+v", emitter.ControlStats())
}
