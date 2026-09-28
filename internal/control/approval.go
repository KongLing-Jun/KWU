package control

import (
	"context"
	"strings"
	"sync"
	"time"

	"kwu/internal/event"
	"kwu/internal/permission"
)

// approvalManager owns the approval/ask prompt bookkeeping and the runtime
// approval posture, behind its own locks and off the controller's c.mu. It is a
// strict leaf: its methods only touch its own state and never call back into the
// Controller. The Controller keeps the I/O orchestration (emitting events,
// rebuilding the executor gate) that needs its other collaborators
// — approval, unlike the goal FSM, blocks on user input and has side effects, so
// only the bookkeeping is extracted, not the orchestration.
type approvalManager struct {
	// policy is the immutable base permission policy, captured at construction.
	// Used to decide whether a tool call would auto-approve under the writer
	// fallback (autoApprovalWouldAllowLocked); the Controller keeps its own copy
	// for building the executor gate.
	policy permission.Policy

	// mu guards the prompt maps and posture fields; every critical section under
	// it is short and non-blocking.
	mu             sync.Mutex
	approvals      map[string]pendingApproval
	asks           map[string]pendingAsk
	granted        map[string]bool
	tombstones     map[string]promptTombstone
	tombstoneOrder []string
	// toolApprovalMode is the runtime approval posture: "ask" prompts, "auto"
	// lets the policy auto-approve the writer fallback while preserving ask/deny
	// rules, and "yolo" skips every tool approval prompt except plan approval.
	toolApprovalMode string
	// approvalTimeout bounds how long requestApproval/Ask block on a user
	// decision. Zero means wait indefinitely (correct for an interactive
	// terminal); bot/headless frontends set it so a walked-away user can't wedge
	// the session forever (#4626, #4402). Write-once at construction.
	approvalTimeout time.Duration
	// planAutoApprove auto-allows writer tool calls without prompting while a
	// just-approved plan executes. Set by the turn loop, read by the bypass
	// check. Plan approval is the go-ahead, so the model shouldn't re-prompt for
	// every write of the work it just got cleared to do.
	planAutoApprove bool

	// promptMu serializes outstanding prompts so at most one user decision is in
	// flight. Held across the blocking wait, so it must never be taken by the
	// resolve paths (Approve/AnswerQuestion).
	promptMu sync.Mutex
}

func newApprovalManager(policy permission.Policy, mode string, timeout time.Duration) approvalManager {
	return approvalManager{
		policy:           policy,
		approvals:        map[string]pendingApproval{},
		asks:             map[string]pendingAsk{},
		tombstones:       map[string]promptTombstone{},
		granted:          map[string]bool{},
		toolApprovalMode: mode,
		approvalTimeout:  timeout,
	}
}

// preApproved reports whether a tool call can skip the prompt — either the
// posture bypasses it (YOLO / plan-execution window) or a session grant already
// covers the scope.
func (a *approvalManager) preApproved(tool, subject string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.bypassAllowsLocked(tool) || a.sessionGrantAllowsLocked(tool, subject)
}

// register allocates an approval ID, records the pending prompt, and returns the
// reply channel the resolve path will signal.
func (a *approvalManager) register(tool, subject, reason string, audit *event.DecisionAudit) (string, chan approvalReply, []event.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sessionID := ""
	if audit != nil {
		sessionID = audit.SessionID
	}
	aggregate := newPromptAggregate(event.PromptKindApproval, sessionID)
	id := aggregate.promptID
	reply := make(chan approvalReply, 1)
	approval := event.Approval{ID: id, Tool: tool, Subject: subject, Reason: reason, Audit: audit}
	aggregate.approval = &approval
	a.approvals[id] = pendingApproval{tool: tool, subject: subject, reason: reason, audit: audit, autoDrain: a.autoApprovalWouldAllowLocked(tool, subject), reply: reply, aggregate: aggregate}
	registered := a.promptFactLocked(aggregate, event.PromptRegistered, event.PromptLifecycleRegistered, "", "")
	emitted := a.promptFactLocked(aggregate, event.PromptEmitted, event.PromptLifecyclePending, registered.EventID, "")
	return id, reply, []event.Event{registered, emitted, promptLegacyProjection(emitted)}
}

// grantSession records a session-scoped grant so future calls in the same scope
// short-circuit.
func (a *approvalManager) grantSession(tool, subject string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.granted[permission.SessionGrantRuleForScope(tool, subject)] = true
}

// cancel drops a pending approval (timeout/abort path).
func (a *approvalManager) cancel(id string) event.Event {
	return a.finishPrompt(id, event.PromptLifecycleCancelled)
}

// resolve removes and returns the pending approval for id (Approve path).
// registerAsk allocates an ask ID, records the pending question batch, and
// returns the reply channel.
func (a *approvalManager) registerAsk(questions []event.AskQuestion, audit *event.DecisionAudit) (string, chan []event.AskAnswer, []event.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sessionID := ""
	if audit != nil {
		sessionID = audit.SessionID
	}
	aggregate := newPromptAggregate(event.PromptKindAsk, sessionID)
	id := aggregate.promptID
	reply := make(chan []event.AskAnswer, 1)
	ask := event.Ask{ID: id, Questions: questions, Audit: audit}
	aggregate.ask = &ask
	a.asks[id] = pendingAsk{questions: questions, audit: audit, reply: reply, aggregate: aggregate}
	registered := a.promptFactLocked(aggregate, event.PromptRegistered, event.PromptLifecycleRegistered, "", "")
	emitted := a.promptFactLocked(aggregate, event.PromptEmitted, event.PromptLifecyclePending, registered.EventID, "")
	return id, reply, []event.Event{registered, emitted, promptLegacyProjection(emitted)}
}

// cancelAsk drops a pending ask (timeout/abort path).
func (a *approvalManager) cancelAsk(id string) event.Event {
	return a.finishPrompt(id, event.PromptLifecycleCancelled)
}

// clearAll cancels prompts whose reply has not been delivered. A delivered
// reply owns the terminal outcome and is left for its waiter to resolve.
func (a *approvalManager) clearAll() []event.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	facts := make([]event.Event, 0, len(a.approvals)+len(a.asks))
	for id, p := range a.approvals {
		if p.delivered {
			continue
		}
		fact := a.promptFactLocked(p.aggregate, event.PromptCancelled, event.PromptLifecycleCancelled, p.aggregate.lastEventID, "")
		facts = append(facts, fact)
		a.rememberTombstoneLocked(id, promptTombstone{lifecycle: event.PromptLifecycleCancelled, fact: fact})
		delete(a.approvals, id)
	}
	for id, p := range a.asks {
		if p.delivered {
			continue
		}
		fact := a.promptFactLocked(p.aggregate, event.PromptCancelled, event.PromptLifecycleCancelled, p.aggregate.lastEventID, "")
		facts = append(facts, fact)
		a.rememberTombstoneLocked(id, promptTombstone{lifecycle: event.PromptLifecycleCancelled, fact: fact})
		delete(a.asks, id)
	}
	return facts
}

// hasPending reports whether any prompt is awaiting a user decision.
func (a *approvalManager) hasPending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.approvals) > 0 || len(a.asks) > 0
}

// mode returns the normalized runtime approval posture.
func (a *approvalManager) mode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return normalizeToolApprovalMode(a.toolApprovalMode)
}

// setMode applies a (pre-normalized) posture and drains any pending approvals
// the new posture should auto-allow, returning them for the caller to signal
// after unlocking.
func (a *approvalManager) setMode(mode string) []pendingApproval {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toolApprovalMode = mode
	switch mode {
	case ToolApprovalAuto:
		return a.drainLocked(false)
	case ToolApprovalYolo:
		return a.drainLocked(true)
	}
	return nil
}

// setPlanAutoApprove toggles the just-approved-plan execution window.
func (a *approvalManager) setPlanAutoApprove(on bool) {
	a.mu.Lock()
	a.planAutoApprove = on
	a.mu.Unlock()
}

// waitContext bounds the blocking wait by approvalTimeout when set.
func (a *approvalManager) waitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.approvalTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.approvalTimeout)
}

// snapshotPrompts copies the in-flight prompts for re-emission to a reconnected
// frontend (ReplayPendingPrompts).
func (a *approvalManager) snapshotPrompts() ([]event.Approval, []event.Ask) {
	a.mu.Lock()
	defer a.mu.Unlock()
	approvals := make([]event.Approval, 0, len(a.approvals))
	for id, p := range a.approvals {
		approvals = append(approvals, event.Approval{ID: id, Tool: p.tool, Subject: p.subject, Reason: p.reason, Audit: p.audit})
	}
	asks := make([]event.Ask, 0, len(a.asks))
	for id, p := range a.asks {
		asks = append(asks, event.Ask{ID: id, Questions: p.questions, Audit: p.audit})
	}
	return approvals, asks
}

func (a *approvalManager) replayPrompts() []event.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []event.Event
	for _, p := range a.approvals {
		fact := a.promptFactLocked(p.aggregate, event.PromptReplayed, event.PromptLifecyclePending, p.aggregate.lastEventID, "")
		out = append(out, fact, promptLegacyProjection(fact))
	}
	for _, p := range a.asks {
		fact := a.promptFactLocked(p.aggregate, event.PromptReplayed, event.PromptLifecyclePending, p.aggregate.lastEventID, "")
		out = append(out, fact, promptLegacyProjection(fact))
	}
	return out
}

// --- decision helpers (caller holds a.mu) ---

func (a *approvalManager) bypassAllowsLocked(tool string) bool {
	if requiresFreshApprovalTool(tool) {
		return false
	}
	return a.toolApprovalMode == ToolApprovalYolo || a.planAutoApprove
}

func (a *approvalManager) autoApprovalWouldAllowLocked(tool, subject string) bool {
	if requiresFreshApprovalTool(tool) {
		return false
	}
	policy := a.policy
	policy.Mode = permission.Allow
	return policy.DecideSubject(tool, false, subject) == permission.Allow
}

func (a *approvalManager) sessionGrantAllowsLocked(tool, subject string) bool {
	if requiresFreshApprovalTool(tool) {
		return false
	}
	for rule := range a.granted {
		if permission.RuleMatchesString(rule, tool, subject) {
			return true
		}
	}
	return false
}

// drainLocked removes every pending approval the new posture should auto-allow
// and returns them; caller holds a.mu and sends {allow:true} after unlocking.
func (a *approvalManager) drainLocked(includeExplicitAsk bool) []pendingApproval {
	pending := make([]pendingApproval, 0, len(a.approvals))
	for id, approval := range a.approvals {
		if requiresFreshApprovalTool(approval.tool) {
			continue
		}
		if !includeExplicitAsk && !approval.autoDrain {
			continue
		}
		approval.replying = true
		approval.aggregate.reply = &event.PromptReplyMetadata{Action: event.PromptReplyAllow}
		approval.replyRequested = a.promptFactLocked(approval.aggregate, event.PromptReplyRequested, event.PromptLifecycleReplying, approval.aggregate.lastEventID, "")
		a.approvals[id] = approval
		pending = append(pending, approval)
	}
	return pending
}

// --- pure approval helpers ---

func normalizeToolApprovalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ToolApprovalAuto, "approve", "allow":
		return ToolApprovalAuto
	case ToolApprovalYolo, "full", "full-access", "bypass":
		return ToolApprovalYolo
	default:
		return ToolApprovalAsk
	}
}

func requiresFreshApprovalTool(tool string) bool {
	switch tool {
	case planApprovalTool, memoryRememberTool, memoryForgetTool, "dashboard_add_widget":
		return true
	default:
		return false
	}
}
