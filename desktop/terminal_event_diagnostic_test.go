package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type terminalDiagnosticProvider struct {
	requests atomic.Int32
}

func TestTerminalDeliveryTraceIsContentFreeAndOrdered(t *testing.T) {
	delivered := make(chan wireEventTab, 3)
	sink := &tabEventSink{tabID: "tab-fixture", ctx: context.Background()}
	sink.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != eventChannel {
			return
		}
		wire, ok := payload[0].(wireEventTab)
		if !ok {
			t.Errorf("payload type = %T", payload[0])
			return
		}
		delivered <- wire
	}
	turnID := event.NewTurnID()
	candidateID := event.NewCandidateID()
	sink.Emit(event.Event{Kind: event.TurnStarted, EventID: event.NewEventID(), TurnID: turnID})
	sink.Emit(event.Event{Kind: event.Message, EventID: event.NewEventID(), CandidateID: candidateID, Text: "SECRET_BODY", Reasoning: "SECRET_REASONING"})
	sink.Emit(event.Event{Kind: event.TurnDone, EventID: event.NewEventID()})

	var wires []wireEventTab
	for len(wires) < 3 {
		select {
		case wire := <-delivered:
			wires = append(wires, wire)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for Wails fixture delivery")
		}
	}
	message := wires[1]
	if message.TurnID != turnID || message.CandidateID != candidateID || message.EventID == "" || message.EventSequence != 2 {
		t.Fatalf("message delivery identity = %+v", message.Event)
	}
	sink.identityMu.Lock()
	arrivals := map[string]uint64{}
	for stage, count := range sink.identity.stageArrivals {
		arrivals[stage] = count
	}
	sink.identityMu.Unlock()
	for _, stage := range []string{"agent_event", "tab_event_sink", "wails_agent_event"} {
		if arrivals[stage] != 3 {
			t.Fatalf("%s arrivals = %d, want 3", stage, arrivals[stage])
		}
	}
	t.Logf("redacted delivery counts: tab=tab-fixture turn=%s candidate=%s agent=%d sink=%d wails=%d", turnID, candidateID, arrivals["agent_event"], arrivals["tab_event_sink"], arrivals["wails_agent_event"])
}

func TestStaleTerminalDoesNotBorrowCurrentCandidateIdentity(t *testing.T) {
	sink := &tabEventSink{}
	sink.identifyAgentEvent(event.Event{Kind: event.TurnStarted, TurnID: "old-turn"})
	sink.identifyAgentEvent(event.Event{Kind: event.Message, TurnID: "old-turn", CandidateID: "old-candidate"})
	sink.identifyAgentEvent(event.Event{Kind: event.TurnStarted, TurnID: "current-turn"})
	sink.identifyAgentEvent(event.Event{Kind: event.Message, TurnID: "current-turn", CandidateID: "current-candidate"})

	stale := sink.identifyAgentEvent(event.Event{Kind: event.TurnDone, TurnID: "old-turn"})
	if stale.CandidateID != "" {
		t.Fatalf("stale terminal candidate = %q, want empty", stale.CandidateID)
	}
	sink.identityMu.Lock()
	currentTurn, currentCandidate := sink.identity.turnID, sink.identity.rootCandidateID
	sink.identityMu.Unlock()
	if currentTurn != "current-turn" || currentCandidate != "current-candidate" {
		t.Fatalf("current identity = %q/%q, want current-turn/current-candidate", currentTurn, currentCandidate)
	}
}

func TestMissingRootCandidateIdentityIsGeneratedAndPropagated(t *testing.T) {
	sink := &tabEventSink{}
	started := sink.identifyAgentEvent(event.Event{Kind: event.TurnStarted})
	first := sink.identifyAgentEvent(event.Event{Kind: event.Message, Text: "first"})
	second := sink.identifyAgentEvent(event.Event{Kind: event.Message, Text: "second"})
	done := sink.identifyAgentEvent(event.Event{Kind: event.TurnDone})

	if started.TurnID == "" {
		t.Fatal("turn_started did not receive a turn identity")
	}
	if first.CandidateID == "" {
		t.Fatal("root message did not receive a candidate identity")
	}
	if first.TurnID != started.TurnID || second.TurnID != started.TurnID || done.TurnID != started.TurnID {
		t.Fatalf("turn identities = %q/%q/%q/%q, want one turn", started.TurnID, first.TurnID, second.TurnID, done.TurnID)
	}
	if second.CandidateID != first.CandidateID || done.CandidateID != first.CandidateID {
		t.Fatalf("candidate identities = %q/%q/%q, want one candidate", first.CandidateID, second.CandidateID, done.CandidateID)
	}
}

func (p *terminalDiagnosticProvider) Name() string { return "terminal-diagnostic" }

func (p *terminalDiagnosticProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	p.requests.Add(1)
	chunks := make(chan provider.Chunk, 1)
	chunks <- provider.Chunk{Type: provider.ChunkText, Text: "fixture final"}
	close(chunks)
	return chunks, nil
}

// TestTerminalDiagnosticAuthority proves the controlled duplicate-delivery
// fixture has one authoritative upstream execution. The duplicate is injected
// only at the Wails/controller boundary by the frontend regression test.
func TestTerminalDiagnosticAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	model := &terminalDiagnosticProvider{}
	var messages atomic.Int32
	var turnDone atomic.Int32
	sink := event.FuncSink(func(e event.Event) {
		switch e.Kind {
		case event.Message:
			messages.Add(1)
		case event.TurnDone:
			turnDone.Add(1)
		}
	})
	session := agent.NewSession("system")
	executor := agent.New(model, tool.NewRegistry(), session, agent.Options{}, sink)
	controller := control.New(control.Options{
		Runner:      executor,
		Executor:    executor,
		Sink:        sink,
		SessionDir:  filepath.Dir(path),
		SessionPath: path,
		Label:       "fixture",
	})

	if err := controller.RunTurn(context.Background(), "fixture prompt"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if err := controller.Snapshot(); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open session JSONL: %v", err)
	}
	defer file.Close()
	finalRecords := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var message provider.Message
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			t.Fatalf("decode session JSONL: %v", err)
		}
		if message.Role == provider.RoleAssistant && message.MessageKind == provider.MessageKindFinalAnswer {
			finalRecords += 1
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan session JSONL: %v", err)
	}

	if got := model.requests.Load(); got != 1 {
		t.Fatalf("model requests = %d, want 1", got)
	}
	if got := messages.Load(); got != 1 {
		t.Fatalf("Message events = %d, want 1", got)
	}
	if got := turnDone.Load(); got != 1 {
		t.Fatalf("TurnDone events = %d, want 1", got)
	}
	if finalRecords != 1 {
		t.Fatalf("JSONL final_answer records = %d, want 1", finalRecords)
	}
	t.Logf("redacted counts: model_requests=%d message_events=%d turn_done_events=%d jsonl_final_answers=%d", model.requests.Load(), messages.Load(), turnDone.Load(), finalRecords)
}
