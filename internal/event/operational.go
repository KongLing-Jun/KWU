package event

import "time"

const OperationalSchemaVersion = 1

type OperationalKind string

type OperationalAggregateKind string

const (
	AggregatePrompt         OperationalAggregateKind = "prompt"
	AggregateTask           OperationalAggregateKind = "task"
	AggregateModelCall      OperationalAggregateKind = "model_call"
	AggregateToolCall       OperationalAggregateKind = "tool_call"
	AggregateSession        OperationalAggregateKind = "session"
	AggregateDashboardWrite OperationalAggregateKind = "dashboard_write"
	AggregatePlan           OperationalAggregateKind = "plan"
)

const (
	PromptRegistered     OperationalKind = "prompt.registered"
	PromptEmitted        OperationalKind = "prompt.emitted"
	PromptReplayed       OperationalKind = "prompt.replayed"
	PromptReplyRequested OperationalKind = "prompt.reply_requested"
	PromptResolved       OperationalKind = "prompt.resolved"
	PromptReplyFailed    OperationalKind = "prompt.reply_failed"
	PromptCancelled      OperationalKind = "prompt.cancelled"

	TaskCreated          OperationalKind = "task.created"
	TaskFreezePending    OperationalKind = "task.freeze_pending"
	TaskReady            OperationalKind = "task.ready"
	TaskRunning          OperationalKind = "task.running"
	TaskUsagePending     OperationalKind = "task.usage_pending"
	TaskSettling         OperationalKind = "task.settling"
	TaskCompleted        OperationalKind = "task.completed"
	TaskFailed           OperationalKind = "task.failed"
	TaskCancelled        OperationalKind = "task.cancelled"
	TaskRecoveryRequired OperationalKind = "task.recovery_required"

	ModelCallPrepared     OperationalKind = "model_call.prepared"
	ModelCallRunning      OperationalKind = "model_call.running"
	ModelCallCompleted    OperationalKind = "model_call.completed"
	ModelCallFailed       OperationalKind = "model_call.failed"
	ModelCallCancelled    OperationalKind = "model_call.cancelled"
	ModelCallInterrupted  OperationalKind = "model_call.interrupted"
	ModelCallUsageMissing OperationalKind = "model_call.usage_missing"

	ToolCallProposed        OperationalKind = "tool_call.proposed"
	ToolCallApprovalPending OperationalKind = "tool_call.approval_pending"
	ToolCallRunning         OperationalKind = "tool_call.running"
	ToolCallProgress        OperationalKind = "tool_call.progress"
	ToolCallCompleted       OperationalKind = "tool_call.completed"
	ToolCallFailed          OperationalKind = "tool_call.failed"
	ToolCallCancelled       OperationalKind = "tool_call.cancelled"

	SessionCreated   OperationalKind = "session.created"
	SessionActivated OperationalKind = "session.activated"
	SessionSuspended OperationalKind = "session.suspended"
	SessionRecovered OperationalKind = "session.recovered"
	SessionClosed    OperationalKind = "session.closed"

	DashboardWriteRequested       OperationalKind = "dashboard_write.requested"
	DashboardWriteApprovalPending OperationalKind = "dashboard_write.approval_pending"
	DashboardWriteCommitted       OperationalKind = "dashboard_write.committed"
	DashboardWriteRejected        OperationalKind = "dashboard_write.rejected"
	DashboardWriteFailed          OperationalKind = "dashboard_write.failed"

	PlanProposed  OperationalKind = "plan.proposed"
	PlanRevising  OperationalKind = "plan.revising"
	PlanApproved  OperationalKind = "plan.approved"
	PlanStarted   OperationalKind = "plan.started"
	PlanExited    OperationalKind = "plan.exited"
	PlanCancelled OperationalKind = "plan.cancelled"
)

type PromptKind string

const (
	PromptKindApproval PromptKind = "approval"
	PromptKindAsk      PromptKind = "ask"
)

type PromptLifecycle string

const (
	PromptLifecycleRegistered  PromptLifecycle = "registered"
	PromptLifecyclePending     PromptLifecycle = "pending"
	PromptLifecycleReplying    PromptLifecycle = "replying"
	PromptLifecycleResolved    PromptLifecycle = "resolved"
	PromptLifecycleReplyFailed PromptLifecycle = "reply_failed"
	PromptLifecycleCancelled   PromptLifecycle = "cancelled"
)

type PrivacyClass string

const (
	PrivacyMetadataOnly    PrivacyClass = "metadata_only"
	PrivacyUserContent     PrivacyClass = "user_content"
	PrivacySensitiveLocal  PrivacyClass = "sensitive_local"
	PrivacySecretForbidden PrivacyClass = "secret_forbidden"
)

type OperationalAggregate struct {
	Kind     OperationalAggregateKind
	ID       string
	Sequence uint64
}

type OperationalScope struct {
	TabID            string
	SessionID        string
	TurnID           string
	CandidateID      string
	PromptID         string
	TaskID           string
	CallID           string
	ToolCallID       string
	PlanID           string
	DashboardWriteID string
}

// LifecycleOperationalPayload is deliberately content-free. It carries only
// the durable lifecycle transition and a bounded role/classification value.
// Model identifiers, tool arguments, paths, prompts, and widget data never
// enter the operational stream through this type.
type LifecycleOperationalPayload struct {
	State         string
	PreviousState string
	Role          string
}

type PromptOperationalPayload struct {
	Kind      PromptKind
	Lifecycle PromptLifecycle
	Approval  *Approval
	Ask       *Ask
	Reply     *PromptReplyMetadata
}

type PromptReplyAction string

const (
	PromptReplyAllow     PromptReplyAction = "allow"
	PromptReplyDeny      PromptReplyAction = "deny"
	PromptReplyAnswered  PromptReplyAction = "answered"
	PromptReplyDismissed PromptReplyAction = "dismissed"
)

// PromptReplyMetadata contains only closed, content-free decision metadata.
// Ask selections and free-typed answers deliberately never enter this payload.
type PromptReplyMetadata struct {
	Action       PromptReplyAction
	SessionGrant bool
	Persistent   bool
	AnswerCount  int
}

// OperationalEnvelope is the additive v1 operational-event contract. Display
// projections may consume Prompt; diagnostic projections must use
// PromptDiagnosticDTO, whose type cannot carry user content.
type OperationalEnvelope struct {
	SchemaVersion  int
	EventID        string
	OperationID    string
	CausationID    string
	OccurrenceID   string
	Aggregate      OperationalAggregate
	Scope          OperationalScope
	Kind           OperationalKind
	Stage          string
	Outcome        string
	Severity       string
	UserImpact     string
	Retryable      bool
	PrivacyClass   PrivacyClass
	OccurredAt     time.Time
	Prompt         *PromptOperationalPayload
	Task           *LifecycleOperationalPayload
	ModelCall      *LifecycleOperationalPayload
	ToolCall       *LifecycleOperationalPayload
	Session        *LifecycleOperationalPayload
	DashboardWrite *LifecycleOperationalPayload
	Plan           *LifecycleOperationalPayload
}

type PromptReplyStatus string

const (
	PromptReplyAccepted        PromptReplyStatus = "accepted"
	PromptReplyAlreadyResolved PromptReplyStatus = "already_resolved"
	PromptReplyNotFound        PromptReplyStatus = "prompt_not_found"
	PromptReplyWrongTab        PromptReplyStatus = "wrong_tab"
	PromptReplyCancelled       PromptReplyStatus = "cancelled"
	PromptReplyFailedStatus    PromptReplyStatus = "failed"
)

type PromptReplyReceipt struct {
	PromptID     string
	Status       PromptReplyStatus
	OccurrenceID string
	Retryable    bool
}

// PromptDiagnosticDTO is deliberately metadata-only. In particular it has no
// prompt/answer text, tool subject/reason, path, actor, profile, or credential
// fields. Keeping this separate from OperationalEnvelope makes privacy trimming
// a producer-boundary property instead of an exporter convention.
type OperationalDiagnosticDTO struct {
	SchemaVersion    int
	EventID          string
	OperationID      string
	CausationID      string
	OccurrenceID     string
	AggregateKind    string
	AggregateID      string
	Sequence         uint64
	TabID            string
	SessionID        string
	TurnID           string
	CandidateID      string
	PromptID         string
	TaskID           string
	CallID           string
	ToolCallID       string
	PlanID           string
	DashboardWriteID string
	PromptKind       PromptKind
	ReplyAction      PromptReplyAction
	SessionGrant     bool
	Persistent       bool
	AnswerCount      int
	Kind             OperationalKind
	Stage            string
	Outcome          string
	Severity         string
	UserImpact       string
	Retryable        bool
	OccurredAt       time.Time
}

// PromptDiagnosticDTO remains an alias for source compatibility with the v1
// prompt-only exporter. New code should use OperationalDiagnosticDTO.
type PromptDiagnosticDTO = OperationalDiagnosticDTO

func (e OperationalEnvelope) Diagnostic() OperationalDiagnosticDTO {
	d := OperationalDiagnosticDTO{
		SchemaVersion: e.SchemaVersion, EventID: e.EventID, OperationID: e.OperationID,
		CausationID: e.CausationID, OccurrenceID: e.OccurrenceID,
		AggregateKind: string(e.Aggregate.Kind), AggregateID: e.Aggregate.ID, Sequence: e.Aggregate.Sequence,
		TabID: e.Scope.TabID, SessionID: e.Scope.SessionID, TurnID: e.Scope.TurnID, CandidateID: e.Scope.CandidateID, PromptID: e.Scope.PromptID,
		TaskID: e.Scope.TaskID, CallID: e.Scope.CallID, ToolCallID: e.Scope.ToolCallID,
		PlanID: e.Scope.PlanID, DashboardWriteID: e.Scope.DashboardWriteID,
		Kind: e.Kind, Stage: e.Stage, Outcome: e.Outcome, Severity: e.Severity,
		UserImpact: e.UserImpact, Retryable: e.Retryable, OccurredAt: e.OccurredAt,
	}
	if e.Prompt != nil {
		d.PromptKind = e.Prompt.Kind
		if e.Prompt.Reply != nil {
			d.ReplyAction = e.Prompt.Reply.Action
			d.SessionGrant = e.Prompt.Reply.SessionGrant
			d.Persistent = e.Prompt.Reply.Persistent
			d.AnswerCount = e.Prompt.Reply.AnswerCount
		}
	}
	return d
}
