package main

import (
	"log/slog"
	"strings"

	"kwu/internal/event"
	"kwu/internal/eventwire"
)

type tabEventIdentityState struct {
	sequence        uint64
	turnID          string
	rootCandidateID string
	stageArrivals   map[string]uint64
}

func rootCandidateEvent(e event.Event) bool {
	return strings.TrimSpace(e.ParentID) == "" && strings.TrimSpace(e.SubagentRef) == ""
}

func tracedTerminalEvent(kind event.Kind) bool {
	switch kind {
	case event.TurnStarted, event.Message, event.FinalCandidateRejected, event.TurnDone:
		return true
	default:
		return false
	}
}

// identifyAgentEvent attaches content-free delivery identity before the event
// enters tab routing. Candidate identity normally originates at the model-call
// boundary; the sink supplies it for legacy/provider message streams that omit
// it, then carries the same identity onto controller events such as TurnDone.
func (s *tabEventSink) identifyAgentEvent(e event.Event) event.Event {
	s.identityMu.Lock()
	defer s.identityMu.Unlock()

	if e.EventID == "" {
		e.EventID = event.NewEventID()
	}
	if e.Kind == event.TurnStarted {
		if e.TurnID == "" {
			e.TurnID = event.NewTurnID()
		}
		if e.TurnID != s.identity.turnID {
			s.identity.turnID = e.TurnID
			s.identity.rootCandidateID = ""
		}
	} else if e.TurnID == "" {
		e.TurnID = s.identity.turnID
	}

	currentTurn := e.TurnID == "" || e.TurnID == s.identity.turnID
	if rootCandidateEvent(e) && currentTurn {
		if e.CandidateID != "" {
			s.identity.rootCandidateID = e.CandidateID
		} else if e.Kind == event.Message {
			if s.identity.rootCandidateID == "" {
				s.identity.rootCandidateID = event.NewCandidateID()
			}
			e.CandidateID = s.identity.rootCandidateID
		} else if e.Kind == event.FinalCandidateRejected || e.Kind == event.TurnDone {
			e.CandidateID = s.identity.rootCandidateID
		}
	}

	s.identity.sequence += 1
	e.Sequence = s.identity.sequence
	if e.Kind == event.FinalCandidateRejected && rootCandidateEvent(e) && currentTurn {
		s.identity.rootCandidateID = ""
	}
	return e
}

// traceAgentDelivery records only opaque routing identities and counters. It
// deliberately accepts no text/reasoning argument, preventing response content
// from entering logs while still identifying the layer that duplicated an event.
func (s *tabEventSink) traceAgentDelivery(stage string, e event.Event) {
	if !tracedTerminalEvent(e.Kind) || !rootCandidateEvent(e) {
		return
	}
	s.identityMu.Lock()
	if s.identity.stageArrivals == nil {
		s.identity.stageArrivals = make(map[string]uint64)
	}
	s.identity.stageArrivals[stage] += 1
	arrival := s.identity.stageArrivals[stage]
	s.identityMu.Unlock()
	slog.Debug("desktop agent event delivery",
		"stage", stage,
		"tab", s.currentTabID(),
		"turn", e.TurnID,
		"kind", eventwire.ToWire(e).Kind,
		"candidate", e.CandidateID,
		"event", e.EventID,
		"source_sequence", e.Sequence,
		"arrival", arrival,
	)
}
