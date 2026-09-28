package main

import (
	"kwu/internal/agent"
	"kwu/internal/event"
)

// scopeOperationalEvent adds Desktop's exact routing identity without
// mutating the Controller-owned envelope. Delivery continues through the
// existing ordered runtime emitter; this adapter owns no state or locks.
func (s *tabEventSink) scopeOperationalEvent(e event.Event) event.Event {
	if s == nil || e.Kind != event.Operational || e.Operational == nil {
		return e
	}
	envelope := *e.Operational
	envelope.Scope = e.Operational.Scope
	envelope.Scope.TabID = s.currentTabID()
	if envelope.Scope.SessionID == "" {
		envelope.Scope.SessionID = agent.BranchID(s.conversationRoute().sessionPath)
	}
	if envelope.EventID == "" {
		envelope.EventID = e.EventID
	}
	if envelope.Scope.TurnID == "" {
		envelope.Scope.TurnID = e.TurnID
	}
	if envelope.Scope.CandidateID == "" {
		envelope.Scope.CandidateID = e.CandidateID
	}
	e.Operational = &envelope
	return e
}

// scopeOperationalPrompt is kept for focused compatibility tests and older
// adapters. All runtime traffic uses scopeOperationalEvent.
func (s *tabEventSink) scopeOperationalPrompt(e event.Event) event.Event {
	return s.scopeOperationalEvent(e)
}
