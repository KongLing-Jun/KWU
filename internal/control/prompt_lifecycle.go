package control

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
)

const promptTombstoneLimit = 256

type promptAggregate struct {
	promptID    string
	operationID string
	sessionID   string
	kind        event.PromptKind
	sequence    uint64
	lastEventID string
	lifecycle   event.PromptLifecycle
	approval    *event.Approval
	ask         *event.Ask
	reply       *event.PromptReplyMetadata
}

type promptTombstone struct {
	lifecycle    event.PromptLifecycle
	occurrenceID string
	fact         event.Event
}

func newPromptAggregate(kind event.PromptKind, sessionID string) *promptAggregate {
	return &promptAggregate{
		promptID: event.NewPromptID(), operationID: event.NewOperationID(),
		sessionID: sessionID, kind: kind,
	}
}

func (a *approvalManager) promptFactLocked(p *promptAggregate, kind event.OperationalKind, lifecycle event.PromptLifecycle, causationID, occurrenceID string) event.Event {
	p.sequence++
	eventID := event.NewEventID()
	p.lastEventID = eventID
	p.lifecycle = lifecycle
	outcome, impact, retryable := promptFactProjection(lifecycle)
	payload := &event.PromptOperationalPayload{Kind: p.kind, Lifecycle: lifecycle, Approval: p.approval, Ask: p.ask, Reply: p.reply}
	envelope := &event.OperationalEnvelope{
		SchemaVersion: event.OperationalSchemaVersion,
		EventID:       eventID, OperationID: p.operationID, CausationID: causationID, OccurrenceID: occurrenceID,
		Aggregate: event.OperationalAggregate{Kind: "prompt", ID: p.promptID, Sequence: p.sequence},
		Scope:     event.OperationalScope{SessionID: p.sessionID, PromptID: p.promptID},
		Kind:      kind, Stage: "controller", Outcome: outcome, Severity: promptSeverity(lifecycle),
		UserImpact: impact, Retryable: retryable, PrivacyClass: event.PrivacyUserContent,
		OccurredAt: time.Now().UTC(), Prompt: payload,
	}
	return event.Event{Kind: event.Operational, EventID: eventID, Operational: envelope}
}

func promptFactProjection(lifecycle event.PromptLifecycle) (outcome, impact string, retryable bool) {
	switch lifecycle {
	case event.PromptLifecycleRegistered:
		return "registered", "none", false
	case event.PromptLifecyclePending:
		return "pending", "waiting_for_user", false
	case event.PromptLifecycleReplying:
		return "requested", "waiting_for_backend", false
	case event.PromptLifecycleResolved:
		return "resolved", "none", false
	case event.PromptLifecycleReplyFailed:
		return "failed", "action_required", true
	case event.PromptLifecycleCancelled:
		return "cancelled", "none", false
	default:
		return "unknown", "none", false
	}
}

func promptSeverity(lifecycle event.PromptLifecycle) string {
	if lifecycle == event.PromptLifecycleReplyFailed {
		return "error"
	}
	return "info"
}

func promptLegacyProjection(fact event.Event) event.Event {
	if fact.Operational == nil || fact.Operational.Prompt == nil {
		return event.Event{}
	}
	p := fact.Operational.Prompt
	if p.Approval != nil {
		return event.Event{Kind: event.ApprovalRequest, EventID: fact.EventID, Approval: *p.Approval}
	}
	if p.Ask != nil {
		return event.Event{Kind: event.AskRequest, EventID: fact.EventID, Ask: *p.Ask}
	}
	return event.Event{}
}

func (a *approvalManager) rememberTombstoneLocked(id string, tombstone promptTombstone) {
	if _, exists := a.tombstones[id]; !exists {
		a.tombstoneOrder = append(a.tombstoneOrder, id)
	}
	a.tombstones[id] = tombstone
	for len(a.tombstoneOrder) > promptTombstoneLimit {
		oldest := a.tombstoneOrder[0]
		a.tombstoneOrder = a.tombstoneOrder[1:]
		delete(a.tombstones, oldest)
	}
}

func (a *approvalManager) terminalReceiptLocked(id string) event.PromptReplyReceipt {
	t, ok := a.tombstones[id]
	if !ok {
		return event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyNotFound}
	}
	if t.lifecycle == event.PromptLifecycleResolved {
		return event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAlreadyResolved}
	}
	if t.lifecycle == event.PromptLifecycleCancelled {
		return event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyCancelled}
	}
	return event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyFailedStatus, OccurrenceID: t.occurrenceID, Retryable: true}
}

func (a *approvalManager) terminalFactLocked(id string) event.Event {
	if t, ok := a.tombstones[id]; ok {
		return t.fact
	}
	return event.Event{}
}

func (a *approvalManager) ownsPrompt(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.approvals[id]; ok {
		return true
	}
	_, ok := a.asks[id]
	return ok
}

func (a *approvalManager) beginApprovalReply(id string, allow, session, persist bool) (pendingApproval, event.Event, event.PromptReplyReceipt, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.approvals[id]
	if !ok {
		return pendingApproval{}, a.terminalFactLocked(id), a.terminalReceiptLocked(id), false
	}
	if p.replying {
		return pendingApproval{}, event.Event{}, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, false
	}
	p.replying = true
	action := event.PromptReplyDeny
	if allow {
		action = event.PromptReplyAllow
	}
	p.aggregate.reply = &event.PromptReplyMetadata{Action: action, SessionGrant: allow && session, Persistent: allow && persist}
	fact := a.promptFactLocked(p.aggregate, event.PromptReplyRequested, event.PromptLifecycleReplying, p.aggregate.lastEventID, "")
	a.approvals[id] = p
	return p, fact, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, true
}

func (a *approvalManager) beginAskReply(id string, answers []event.AskAnswer) (pendingAsk, event.Event, event.PromptReplyReceipt, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.asks[id]
	if !ok {
		return pendingAsk{}, a.terminalFactLocked(id), a.terminalReceiptLocked(id), false
	}
	if p.replying {
		return pendingAsk{}, event.Event{}, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, false
	}
	p.replying = true
	action := event.PromptReplyAnswered
	if len(answers) == 0 {
		action = event.PromptReplyDismissed
	}
	p.aggregate.reply = &event.PromptReplyMetadata{Action: action, AnswerCount: len(answers)}
	fact := a.promptFactLocked(p.aggregate, event.PromptReplyRequested, event.PromptLifecycleReplying, p.aggregate.lastEventID, "")
	a.asks[id] = p
	return p, fact, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, true
}

func (a *approvalManager) deliverApprovalReply(id string, reply approvalReply) (pendingApproval, event.PromptReplyReceipt, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.approvals[id]
	if !ok {
		return pendingApproval{}, a.terminalReceiptLocked(id), false
	}
	if !p.replying || p.delivered {
		return p, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, false
	}
	p.delivered = true
	p.reply <- reply
	a.approvals[id] = p
	return p, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, true
}

func (a *approvalManager) deliverAskReply(id string, answers []event.AskAnswer) (pendingAsk, event.PromptReplyReceipt, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.asks[id]
	if !ok {
		return pendingAsk{}, a.terminalReceiptLocked(id), false
	}
	if !p.replying || p.delivered {
		return p, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, false
	}
	p.delivered = true
	p.reply <- answers
	a.asks[id] = p
	return p, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}, true
}

func (a *approvalManager) finishPrompt(id string, lifecycle event.PromptLifecycle) event.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.approvals[id]; ok {
		kind := event.PromptResolved
		if lifecycle == event.PromptLifecycleCancelled {
			kind = event.PromptCancelled
		}
		fact := a.promptFactLocked(p.aggregate, kind, lifecycle, p.aggregate.lastEventID, "")
		delete(a.approvals, id)
		a.rememberTombstoneLocked(id, promptTombstone{lifecycle: lifecycle, fact: fact})
		return fact
	}
	if p, ok := a.asks[id]; ok {
		kind := event.PromptResolved
		if lifecycle == event.PromptLifecycleCancelled {
			kind = event.PromptCancelled
		}
		fact := a.promptFactLocked(p.aggregate, kind, lifecycle, p.aggregate.lastEventID, "")
		delete(a.asks, id)
		a.rememberTombstoneLocked(id, promptTombstone{lifecycle: lifecycle, fact: fact})
		return fact
	}
	return event.Event{}
}

func (a *approvalManager) failPromptReply(id string) (event.Event, event.PromptReplyReceipt) {
	a.mu.Lock()
	defer a.mu.Unlock()
	occurrenceID := event.NewOccurrenceID()
	if p, ok := a.approvals[id]; ok {
		if p.replying {
			return event.Event{}, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}
		}
		p.replying = false
		fact := a.promptFactLocked(p.aggregate, event.PromptReplyFailed, event.PromptLifecycleReplyFailed, p.aggregate.lastEventID, occurrenceID)
		a.approvals[id] = p
		return fact, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyFailedStatus, OccurrenceID: occurrenceID, Retryable: true}
	}
	if p, ok := a.asks[id]; ok {
		if p.replying {
			return event.Event{}, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted}
		}
		p.replying = false
		fact := a.promptFactLocked(p.aggregate, event.PromptReplyFailed, event.PromptLifecycleReplyFailed, p.aggregate.lastEventID, occurrenceID)
		a.asks[id] = p
		return fact, event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyFailedStatus, OccurrenceID: occurrenceID, Retryable: true}
	}
	return event.Event{}, a.terminalReceiptLocked(id)
}

func emitEvent(sink event.Sink, e event.Event) {
	if e.EventID == "" && e.Operational == nil {
		return
	}
	sink.Emit(e)
}

func scopePromptFacts(ctx context.Context, facts []event.Event) []event.Event {
	emitter, _ := event.OperationalEmitterFrom(ctx)
	toolCallID, _, _, _ := agent.CallContext(ctx)
	scope := event.OperationalScopeFrom(ctx)
	scope.ToolCallID = toolCallID
	parentEventID := ""
	if emitter != nil {
		parentEventID = emitter.LastEvent(event.AggregateToolCall, toolCallID)
	}
	for index := range facts {
		if facts[index].Operational == nil {
			continue
		}
		facts[index].Operational.Scope = event.MergeOperationalScope(scope, facts[index].Operational.Scope)
		if facts[index].Operational.CausationID == "" {
			facts[index].Operational.CausationID = parentEventID
		}
	}
	return facts
}

func (c *Controller) emitToolApprovalPending(ctx context.Context, promptID, promptEventID, toolName string, args json.RawMessage) {
	toolCallID, _, _, ok := agent.CallContext(ctx)
	if !ok || toolCallID == "" {
		return
	}
	emitter, ok := event.OperationalEmitterFrom(ctx)
	if !ok {
		return
	}
	toolScope := event.OperationalScopeFrom(ctx)
	toolScope.ToolCallID, toolScope.PromptID = toolCallID, promptID
	toolEventID := emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateToolCall, AggregateID: toolCallID, Scope: toolScope,
		Kind: event.ToolCallApprovalPending, Stage: "controller", Outcome: "approval_pending", Severity: "info", UserImpact: "waiting_for_user",
		CausationID: promptEventID,
		Payload:     &event.LifecycleOperationalPayload{State: "approval_pending", PreviousState: "proposed"},
	})
	if toolName != "dashboard_add_widget" {
		return
	}
	var input struct {
		OperationID string `json:"operationId"`
	}
	if json.Unmarshal(args, &input) != nil || strings.TrimSpace(input.OperationID) == "" {
		return
	}
	scope := event.OperationalScopeFrom(ctx)
	scope.DashboardWriteID, scope.ToolCallID, scope.PromptID = input.OperationID, toolCallID, promptID
	emitter.Emit(event.OperationalFact{
		AggregateKind: event.AggregateDashboardWrite, AggregateID: input.OperationID, Scope: scope,
		Kind: event.DashboardWriteApprovalPending, Stage: "controller", Outcome: "approval_pending", Severity: "info", UserImpact: "waiting_for_user",
		CausationID: toolEventID,
		Payload:     &event.LifecycleOperationalPayload{State: "approval_pending"},
	})
}
