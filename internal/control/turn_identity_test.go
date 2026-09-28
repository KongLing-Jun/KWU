package control

import (
	"context"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type turnIdentityProvider struct{}

func (turnIdentityProvider) Name() string { return "turn-identity" }

func (turnIdentityProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	chunks := make(chan provider.Chunk, 2)
	chunks <- provider.Chunk{Type: provider.ChunkText, Text: "done"}
	chunks <- provider.Chunk{Type: provider.ChunkDone}
	close(chunks)
	return chunks, nil
}

func TestRunTurnUsesOneIdentityFromAgentStartThroughControllerDone(t *testing.T) {
	var events []event.Event
	sink := event.FuncSink(func(e event.Event) {
		events = append(events, e)
	})
	executor := agent.New(turnIdentityProvider{}, tool.NewRegistry(), agent.NewSession("system"), agent.Options{}, sink)
	controller := New(Options{Runner: executor, Executor: executor, Sink: sink})
	t.Cleanup(controller.Close)

	if err := controller.RunTurn(context.Background(), "finish"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	var started, message, done event.Event
	for _, emitted := range events {
		switch emitted.Kind {
		case event.TurnStarted:
			started = emitted
		case event.Message:
			message = emitted
		case event.TurnDone:
			done = emitted
		}
	}
	if started.TurnID == "" {
		t.Fatal("TurnStarted TurnID is empty")
	}
	if message.CandidateID == "" {
		t.Fatal("Message CandidateID is empty")
	}
	if done.TurnID != started.TurnID {
		t.Fatalf("event identities diverged: TurnStarted = %q, TurnDone = %q", started.TurnID, done.TurnID)
	}
}

func TestWithOperationalTaskScopeTurnIDPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		contextTurnID string
		scopeTurnID   string
		wantTurnID    string
		wantErr       bool
	}{
		{name: "inherits controller context", contextTurnID: "controller-turn", wantTurnID: "controller-turn"},
		{name: "preserves explicit scope", scopeTurnID: "explicit-turn", wantTurnID: "explicit-turn"},
		{name: "preserves matching identities", contextTurnID: "same-turn", scopeTurnID: "same-turn", wantTurnID: "same-turn"},
		{name: "generates identity when neither source is set"},
		{name: "rejects conflicting identities", contextTurnID: "controller-turn", scopeTurnID: "explicit-turn", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := New(Options{})
			t.Cleanup(controller.Close)
			ctx := context.Background()
			if tt.contextTurnID != "" {
				ctx = event.WithTurnID(ctx, tt.contextTurnID)
			}
			if tt.scopeTurnID != "" {
				ctx = event.WithOperationalScope(ctx, event.OperationalScope{TurnID: tt.scopeTurnID})
			}

			_, scope, err := controller.withOperationalTaskScope(ctx)
			if tt.wantErr {
				if err == nil {
					t.Fatal("withOperationalTaskScope returned no conflict error")
				}
				return
			}
			if err != nil {
				t.Fatalf("withOperationalTaskScope: %v", err)
			}
			if tt.wantTurnID == "" {
				if scope.TurnID == "" {
					t.Fatal("scope TurnID is empty")
				}
				return
			}
			if scope.TurnID != tt.wantTurnID {
				t.Fatalf("scope TurnID = %q, want %q", scope.TurnID, tt.wantTurnID)
			}
		})
	}
}
