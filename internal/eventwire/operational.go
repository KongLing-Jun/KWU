package eventwire

import (
	"time"

	"kwu/internal/event"
)

type OperationalAggregate struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Sequence uint64 `json:"sequence"`
}

type OperationalScope struct {
	TabID            string `json:"tabId,omitempty"`
	SessionID        string `json:"sessionId,omitempty"`
	TurnID           string `json:"turnId,omitempty"`
	CandidateID      string `json:"candidateId,omitempty"`
	PromptID         string `json:"promptId,omitempty"`
	TaskID           string `json:"taskId,omitempty"`
	CallID           string `json:"callId,omitempty"`
	ToolCallID       string `json:"toolCallId,omitempty"`
	PlanID           string `json:"planId,omitempty"`
	DashboardWriteID string `json:"dashboardWriteId,omitempty"`
}

type LifecycleOperationalPayload struct {
	State         string `json:"state"`
	PreviousState string `json:"previousState,omitempty"`
	Role          string `json:"role,omitempty"`
}

type PromptOperationalPayload struct {
	Kind      string               `json:"kind"`
	Lifecycle string               `json:"lifecycle"`
	Approval  *Approval            `json:"approval,omitempty"`
	Ask       *Ask                 `json:"ask,omitempty"`
	Reply     *PromptReplyMetadata `json:"reply,omitempty"`
}

type PromptReplyMetadata struct {
	Action       string `json:"action"`
	SessionGrant bool   `json:"sessionGrant"`
	Persistent   bool   `json:"persistent"`
	AnswerCount  int    `json:"answerCount"`
}

type OperationalEnvelope struct {
	SchemaVersion  int                          `json:"schemaVersion"`
	EventID        string                       `json:"eventId"`
	OperationID    string                       `json:"operationId"`
	CausationID    string                       `json:"causationId,omitempty"`
	OccurrenceID   string                       `json:"occurrenceId,omitempty"`
	Aggregate      OperationalAggregate         `json:"aggregate"`
	Scope          OperationalScope             `json:"scope"`
	Kind           string                       `json:"kind"`
	Stage          string                       `json:"stage"`
	Outcome        string                       `json:"outcome"`
	Severity       string                       `json:"severity"`
	UserImpact     string                       `json:"userImpact"`
	Retryable      bool                         `json:"retryable"`
	PrivacyClass   string                       `json:"privacyClass"`
	OccurredAt     time.Time                    `json:"occurredAt"`
	Prompt         *PromptOperationalPayload    `json:"prompt,omitempty"`
	Task           *LifecycleOperationalPayload `json:"task,omitempty"`
	ModelCall      *LifecycleOperationalPayload `json:"modelCall,omitempty"`
	ToolCall       *LifecycleOperationalPayload `json:"toolCall,omitempty"`
	Session        *LifecycleOperationalPayload `json:"session,omitempty"`
	DashboardWrite *LifecycleOperationalPayload `json:"dashboardWrite,omitempty"`
	Plan           *LifecycleOperationalPayload `json:"plan,omitempty"`
}

type PromptReplyReceipt struct {
	PromptID     string `json:"promptId"`
	Status       string `json:"status"`
	OccurrenceID string `json:"occurrenceId,omitempty"`
	Retryable    bool   `json:"retryable"`
}

type PromptDiagnostic struct {
	SchemaVersion    int       `json:"schemaVersion"`
	EventID          string    `json:"eventId"`
	OperationID      string    `json:"operationId"`
	CausationID      string    `json:"causationId,omitempty"`
	OccurrenceID     string    `json:"occurrenceId,omitempty"`
	AggregateKind    string    `json:"aggregateKind"`
	AggregateID      string    `json:"aggregateId"`
	Sequence         uint64    `json:"sequence"`
	TabID            string    `json:"tabId,omitempty"`
	SessionID        string    `json:"sessionId,omitempty"`
	TurnID           string    `json:"turnId,omitempty"`
	PromptID         string    `json:"promptId"`
	TaskID           string    `json:"taskId,omitempty"`
	CallID           string    `json:"callId,omitempty"`
	ToolCallID       string    `json:"toolCallId,omitempty"`
	PlanID           string    `json:"planId,omitempty"`
	DashboardWriteID string    `json:"dashboardWriteId,omitempty"`
	PromptKind       string    `json:"promptKind"`
	ReplyAction      string    `json:"replyAction,omitempty"`
	SessionGrant     bool      `json:"sessionGrant"`
	Persistent       bool      `json:"persistent"`
	AnswerCount      int       `json:"answerCount"`
	Kind             string    `json:"kind"`
	Stage            string    `json:"stage"`
	Outcome          string    `json:"outcome"`
	Severity         string    `json:"severity"`
	UserImpact       string    `json:"userImpact"`
	Retryable        bool      `json:"retryable"`
	OccurredAt       time.Time `json:"occurredAt"`
}

func ToWireOperational(e *event.OperationalEnvelope) *OperationalEnvelope {
	if e == nil {
		return nil
	}
	w := &OperationalEnvelope{
		SchemaVersion: e.SchemaVersion, EventID: e.EventID, OperationID: e.OperationID,
		CausationID: e.CausationID, OccurrenceID: e.OccurrenceID,
		Aggregate: OperationalAggregate{Kind: string(e.Aggregate.Kind), ID: e.Aggregate.ID, Sequence: e.Aggregate.Sequence},
		Scope: OperationalScope{
			TabID: e.Scope.TabID, SessionID: e.Scope.SessionID, TurnID: e.Scope.TurnID, CandidateID: e.Scope.CandidateID,
			PromptID: e.Scope.PromptID, TaskID: e.Scope.TaskID, CallID: e.Scope.CallID, ToolCallID: e.Scope.ToolCallID,
			PlanID: e.Scope.PlanID, DashboardWriteID: e.Scope.DashboardWriteID,
		},
		Kind: string(e.Kind), Stage: e.Stage, Outcome: e.Outcome, Severity: e.Severity,
		UserImpact: e.UserImpact, Retryable: e.Retryable, PrivacyClass: string(e.PrivacyClass), OccurredAt: e.OccurredAt,
	}
	if p := e.Prompt; p != nil {
		wp := &PromptOperationalPayload{Kind: string(p.Kind), Lifecycle: string(p.Lifecycle)}
		if p.Approval != nil {
			wp.Approval = &Approval{ID: p.Approval.ID, Tool: p.Approval.Tool, Subject: p.Approval.Subject, Reason: p.Approval.Reason, Audit: ToWireDecisionAudit(p.Approval.Audit)}
		}
		if p.Ask != nil {
			wp.Ask = ToWireAsk(*p.Ask)
		}
		if p.Reply != nil {
			wp.Reply = &PromptReplyMetadata{Action: string(p.Reply.Action), SessionGrant: p.Reply.SessionGrant, Persistent: p.Reply.Persistent, AnswerCount: p.Reply.AnswerCount}
		}
		w.Prompt = wp
	}
	w.Task = toWireLifecycle(e.Task)
	w.ModelCall = toWireLifecycle(e.ModelCall)
	w.ToolCall = toWireLifecycle(e.ToolCall)
	w.Session = toWireLifecycle(e.Session)
	w.DashboardWrite = toWireLifecycle(e.DashboardWrite)
	w.Plan = toWireLifecycle(e.Plan)
	return w
}

func toWireLifecycle(value *event.LifecycleOperationalPayload) *LifecycleOperationalPayload {
	if value == nil {
		return nil
	}
	return &LifecycleOperationalPayload{State: value.State, PreviousState: value.PreviousState, Role: value.Role}
}

func ToWirePromptReplyReceipt(r event.PromptReplyReceipt) PromptReplyReceipt {
	return PromptReplyReceipt{PromptID: r.PromptID, Status: string(r.Status), OccurrenceID: r.OccurrenceID, Retryable: r.Retryable}
}

func ToWirePromptDiagnostic(e event.OperationalEnvelope) PromptDiagnostic {
	d := e.Diagnostic()
	return PromptDiagnostic{
		SchemaVersion: d.SchemaVersion, EventID: d.EventID, OperationID: d.OperationID,
		CausationID: d.CausationID, OccurrenceID: d.OccurrenceID,
		AggregateKind: d.AggregateKind, AggregateID: d.AggregateID, Sequence: d.Sequence,
		TabID: d.TabID, SessionID: d.SessionID, TurnID: d.TurnID, PromptID: d.PromptID,
		TaskID: d.TaskID, CallID: d.CallID, ToolCallID: d.ToolCallID, PlanID: d.PlanID, DashboardWriteID: d.DashboardWriteID,
		PromptKind: string(d.PromptKind), ReplyAction: string(d.ReplyAction), SessionGrant: d.SessionGrant, Persistent: d.Persistent, AnswerCount: d.AnswerCount,
		Kind: string(d.Kind), Stage: d.Stage, Outcome: d.Outcome,
		Severity: d.Severity, UserImpact: d.UserImpact, Retryable: d.Retryable, OccurredAt: d.OccurredAt,
	}
}
