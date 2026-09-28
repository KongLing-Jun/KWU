package agent

import (
	"context"
	"testing"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func TestRunTurnIDPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		contextTurnID string
		scopeTurnID   string
		wantTurnID    string
	}{
		{
			name:          "inherits controller context",
			contextTurnID: "controller-turn",
			wantTurnID:    "controller-turn",
		},
		{
			name:          "explicit operational scope wins",
			contextTurnID: "controller-turn",
			scopeTurnID:   "explicit-turn",
			wantTurnID:    "explicit-turn",
		},
		{name: "generates identity when neither source is set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &mockProvider{name: "test", chunks: []provider.Chunk{
				{Type: provider.ChunkText, Text: "done"},
				{Type: provider.ChunkDone},
			}}
			var started event.Event
			a := New(prov, tool.NewRegistry(), NewSession("system"), Options{}, event.FuncSink(func(e event.Event) {
				if e.Kind == event.TurnStarted {
					started = e
				}
			}))

			ctx := context.Background()
			if tt.contextTurnID != "" {
				ctx = event.WithTurnID(ctx, tt.contextTurnID)
			}
			if tt.scopeTurnID != "" {
				ctx = event.WithOperationalScope(ctx, event.OperationalScope{TurnID: tt.scopeTurnID})
			}
			if err := a.Run(ctx, "finish"); err != nil {
				t.Fatalf("Run: %v", err)
			}

			if started.Kind != event.TurnStarted {
				t.Fatal("TurnStarted was not emitted")
			}
			if tt.wantTurnID == "" {
				if started.TurnID == "" {
					t.Fatal("TurnStarted TurnID is empty")
				}
				return
			}
			if started.TurnID != tt.wantTurnID {
				t.Fatalf("TurnStarted TurnID = %q, want %q", started.TurnID, tt.wantTurnID)
			}
		})
	}
}
