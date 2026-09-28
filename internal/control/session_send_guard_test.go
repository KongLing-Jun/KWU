package control

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
)

type sessionGuardRunnerFunc func(context.Context, string) error

func (f sessionGuardRunnerFunc) Run(ctx context.Context, input string) error { return f(ctx, input) }

func TestSessionSendGuardBlocksRunAndRunTurnBeforeRunner(t *testing.T) {
	want := errors.New("project session is read only")
	guardCalls := 0
	done := make(chan event.Event, 2)
	executor := agent.New(nil, nil, agent.NewSession("system"), agent.Options{}, event.Discard)
	c := New(Options{
		Runner: sessionGuardRunnerFunc(func(context.Context, string) error {
			t.Fatal("runner reached after session guard rejection")
			return nil
		}),
		Executor: executor,
		Sink: event.FuncSink(func(evt event.Event) {
			if evt.Kind == event.TurnDone {
				done <- evt
			}
		}),
		SessionPath: "project-session.jsonl",
		SessionSendGuard: func(_ context.Context, path string) error {
			guardCalls++
			if path != "project-session.jsonl" {
				t.Fatalf("guard path = %q", path)
			}
			return want
		},
	})
	if err := c.Run(context.Background(), "headless"); !errors.Is(err, want) {
		t.Fatalf("Run error = %v", err)
	}
	if err := c.RunTurn(context.Background(), "foreground"); !errors.Is(err, want) {
		t.Fatalf("RunTurn error = %v", err)
	}
	<-done // RunTurn emits its synchronous completion through the shared sink.
	for _, start := range []func(){
		func() { c.Send("async") },
		func() { c.SubmitCapabilities(CapabilityTurn{Input: "capability"}) },
	} {
		start()
		select {
		case evt := <-done:
			if !errors.Is(evt.Err, want) {
				t.Fatalf("async guard error = %v", evt.Err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for guarded async turn")
		}
	}
	if err := c.Compact(context.Background(), ""); !errors.Is(err, want) {
		t.Fatalf("Compact error = %v", err)
	}
	if err := c.SummarizeFrom(context.Background(), 1); !errors.Is(err, want) {
		t.Fatalf("SummarizeFrom error = %v", err)
	}
	if err := c.SummarizeUpTo(context.Background(), 1); !errors.Is(err, want) {
		t.Fatalf("SummarizeUpTo error = %v", err)
	}
	if guardCalls != 7 {
		t.Fatalf("guard calls = %d, want 7", guardCalls)
	}
}
