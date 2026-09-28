// Package modelbilling defines the normalized desktop contract for model
// consumption. Cloud and provider wire formats belong behind adapters.
package modelbilling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/provider"
)

type BillingSource string

const (
	BillingSubscriptionPool BillingSource = "subscription_pool"
	BillingNewAPIByok       BillingSource = "newapi_byok"
)

type CallRole string

const (
	CallRoleMain            CallRole = "main"
	CallRolePlanner         CallRole = "planner"
	CallRoleExecutor        CallRole = "executor"
	CallRoleSubagent        CallRole = "subagent"
	CallRoleCompaction      CallRole = "compaction"
	CallRoleClassifier      CallRole = "classifier"
	CallRoleTasktreePlanner CallRole = "tasktree_planner"
	CallRoleTitle           CallRole = "title"
	CallRoleGuardian        CallRole = "guardian"
)

type TaskResult string

const (
	TaskResultCompleted TaskResult = "completed"
	TaskResultFailed    TaskResult = "failed"
	TaskResultCancelled TaskResult = "cancelled"
)

type ReportMode string

const ReportModeCumulative ReportMode = "cumulative"

type QuotaType string

const (
	QuotaTypeToken      QuotaType = "token"
	QuotaTypePerRequest QuotaType = "per_request"
)

type BillingMode string

const (
	BillingModeTokenRatio  BillingMode = "token_ratio"
	BillingModePerRequest  BillingMode = "per_request"
	BillingModeTieredExpr  BillingMode = "tiered_expr"
	BillingModeUnavailable BillingMode = "unavailable"
)

type TaskState string

const (
	TaskCreated          TaskState = "created"
	TaskFreezePending    TaskState = "freeze_pending"
	TaskReady            TaskState = "ready"
	TaskRunning          TaskState = "running"
	TaskUsagePending     TaskState = "usage_pending"
	TaskSettling         TaskState = "settling"
	TaskCompleted        TaskState = "completed"
	TaskFailed           TaskState = "failed"
	TaskCancelled        TaskState = "cancelled"
	TaskRecoveryRequired TaskState = "recovery_required"
)

type CallState string

const (
	CallPrepared     CallState = "prepared"
	CallRunning      CallState = "running"
	CallCompleted    CallState = "completed"
	CallFailed       CallState = "failed"
	CallCancelled    CallState = "cancelled"
	CallInterrupted  CallState = "interrupted"
	CallUsageMissing CallState = "usage_missing"
)

type UsageBreakdownStatus string

const (
	UsageExact       UsageBreakdownStatus = "exact"
	UsagePartial     UsageBreakdownStatus = "partial"
	UsageUnavailable UsageBreakdownStatus = "unavailable"
	UsageMissing     UsageBreakdownStatus = "usage_missing"
)

type ReportKind string

const (
	ReportPartial ReportKind = "partial"
	ReportFinal   ReportKind = "final"
)

type ReportState string

const (
	ReportPending    ReportState = "pending"
	ReportDelivering ReportState = "delivering"
	ReportAccepted   ReportState = "accepted"
	ReportRetry      ReportState = "retry"
	ReportBlocked    ReportState = "blocked"
)

type PricingState string

const (
	PricingFresh        PricingState = "fresh"
	PricingStaleUsable  PricingState = "stale_usable"
	PricingStaleBlocked PricingState = "stale_blocked"
	PricingUnavailable  PricingState = "unavailable"
)

type SettlementState string

const (
	SettlementPending        SettlementState = "pending"
	SettlementPricingPending SettlementState = "pricing_pending"
	SettlementSettled        SettlementState = "settled"
	SettlementException      SettlementState = "exception"
)

type TaskScope struct {
	ActorID      string              `json:"actor_id"`
	ProfileID    string              `json:"profile_id"`
	CloudBinding *cloud.BindingScope `json:"cloud_binding,omitempty"`
}

type ModelTask struct {
	TaskID              string                   `json:"task_id"`
	Scope               TaskScope                `json:"scope"`
	BillingSource       BillingSource            `json:"billing_source"`
	InitialModelID      string                   `json:"initial_model_id"`
	ClientTaskType      string                   `json:"client_task_type"`
	ClientEstimatedCost string                   `json:"client_estimated_cost"`
	FreezeRequest       *cloud.FreezeRequest     `json:"freeze_request,omitempty"`
	Freeze              *cloud.Freeze            `json:"freeze,omitempty"`
	Entitlement         *TaskEntitlementSnapshot `json:"entitlement,omitempty"`
	PricingEvidenceID   string                   `json:"pricing_evidence_id"`
	State               TaskState                `json:"state"`
	Result              *TaskResult              `json:"task_result,omitempty"`
	CreatedAt           time.Time                `json:"created_at"`
	StartedAt           *time.Time               `json:"started_at,omitempty"`
	UpdatedAt           time.Time                `json:"updated_at"`
	EndedAt             *time.Time               `json:"ended_at,omitempty"`
	LastErrorCode       *string                  `json:"last_error_code,omitempty"`
}

type ModelCall struct {
	CallID                   string                 `json:"call_id"`
	TaskID                   string                 `json:"task_id"`
	Scope                    TaskScope              `json:"scope"`
	BillingSource            BillingSource          `json:"billing_source"`
	Role                     CallRole               `json:"role"`
	ModelID                  string                 `json:"model_id"`
	Provider                 string                 `json:"provider"`
	Endpoint                 MaasEndpoint           `json:"endpoint,omitempty"`
	ProviderRequestID        *string                `json:"provider_request_id,omitempty"`
	PricingEvidenceID        string                 `json:"pricing_evidence_id"`
	State                    CallState              `json:"state"`
	AttemptCount             int                    `json:"attempt_count"`
	StartedAt                *time.Time             `json:"started_at,omitempty"`
	FinishedAt               *time.Time             `json:"finished_at,omitempty"`
	OccurredAt               *time.Time             `json:"occurred_at,omitempty"`
	Usage                    NormalizedUsage        `json:"usage"`
	RawUsageRedacted         json.RawMessage        `json:"raw_usage_redacted,omitempty"`
	LastErrorCode            *string                `json:"last_error_code,omitempty"`
	QuotaEvidence            *QuotaEvidenceRevision `json:"quota_evidence,omitempty"`
	QuotaReportStatus        QuotaReportStatus      `json:"quota_report_status,omitempty"`
	QuotaReportLastErrorCode *string                `json:"quota_report_last_error_code,omitempty"`
	quotaRequestIDs          []string
	quotaEvidenceRevisions   []QuotaEvidenceRevision
}

type NormalizedUsage struct {
	InputTokens         *int64               `json:"input_tokens,omitempty"`
	NormalInputTokens   *int64               `json:"normal_input_tokens,omitempty"`
	CachedInputTokens   *int64               `json:"cached_input_tokens,omitempty"`
	CacheReadTokens     *int64               `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens    *int64               `json:"cache_write_tokens,omitempty"`
	OutputTokens        *int64               `json:"output_tokens,omitempty"`
	CachedOutputTokens  *int64               `json:"cached_output_tokens,omitempty"`
	ImageTokens         *int64               `json:"image_tokens,omitempty"`
	AudioInputTokens    *int64               `json:"audio_input_tokens,omitempty"`
	AudioOutputTokens   *int64               `json:"audio_output_tokens,omitempty"`
	RequestCount        *int64               `json:"request_count,omitempty"`
	ProviderTotalTokens *int64               `json:"provider_total_tokens,omitempty"`
	BreakdownStatus     UsageBreakdownStatus `json:"breakdown_status"`
}

type TaskEntitlementSnapshot struct {
	Plan               string    `json:"plan"`
	EntitlementVersion *string   `json:"entitlement_version,omitempty"`
	CapturedAt         time.Time `json:"captured_at"`
}

type PricingSnapshot struct {
	SnapshotID    string                   `json:"snapshot_id"`
	Source        string                   `json:"source"`
	SourceVersion *string                  `json:"source_version,omitempty"`
	SourceHash    string                   `json:"source_hash"`
	Currency      string                   `json:"currency"`
	ExchangeRate  *DecimalString           `json:"exchange_rate,omitempty"`
	RechargePrice *DecimalString           `json:"recharge_price,omitempty"`
	GroupRatios   map[string]DecimalString `json:"group_ratios"`
	UsableGroups  []string                 `json:"usable_groups"`
	FetchedAt     time.Time                `json:"fetched_at"`
	FreshUntil    time.Time                `json:"fresh_until"`
	StaleUntil    time.Time                `json:"stale_until"`
	State         PricingState             `json:"state"`
	Models        []ModelPrice             `json:"models"`
}

type ModelPrice struct {
	ModelID               string         `json:"model_id"`
	ProviderModelID       string         `json:"provider_model_id"`
	ModelName             string         `json:"model_name"`
	ModelType             string         `json:"model_type"`
	Capabilities          []string       `json:"capabilities"`
	QuotaType             QuotaType      `json:"quota_type"`
	ModelRatio            *DecimalString `json:"model_ratio,omitempty"`
	CompletionRatio       *DecimalString `json:"completion_ratio,omitempty"`
	CacheRatio            *DecimalString `json:"cache_ratio,omitempty"`
	CreateCacheRatio      *DecimalString `json:"create_cache_ratio,omitempty"`
	ImageRatio            *DecimalString `json:"image_ratio,omitempty"`
	AudioRatio            *DecimalString `json:"audio_ratio,omitempty"`
	AudioCompletionRatio  *DecimalString `json:"audio_completion_ratio,omitempty"`
	ModelPrice            *DecimalString `json:"model_price,omitempty"`
	EnableGroups          []string       `json:"enable_groups"`
	BillingMode           BillingMode    `json:"billing_mode"`
	BillingExpression     *string        `json:"billing_expression,omitempty"`
	InputPerMillion       *DecimalString `json:"input_per_million,omitempty"`
	CacheReadPerMillion   *DecimalString `json:"cache_read_per_million,omitempty"`
	CacheWritePerMillion  *DecimalString `json:"cache_write_per_million,omitempty"`
	OutputPerMillion      *DecimalString `json:"output_per_million,omitempty"`
	ImageInputPerMillion  *DecimalString `json:"image_input_per_million,omitempty"`
	AudioInputPerMillion  *DecimalString `json:"audio_input_per_million,omitempty"`
	AudioOutputPerMillion *DecimalString `json:"audio_output_per_million,omitempty"`
	RequestPerUnit        *DecimalString `json:"request_per_unit,omitempty"`
}

type PricingSourceStatus struct {
	USDExchangeRate DecimalString `json:"usd_exchange_rate"`
	RechargePrice   DecimalString `json:"recharge_price"`
	FetchedAt       time.Time     `json:"fetched_at"`
	SourceVersion   *string       `json:"source_version,omitempty"`
	SourceHash      *string       `json:"source_hash,omitempty"`
}

type PricingEvidence struct {
	EvidenceID     string       `json:"evidence_id"`
	SnapshotID     string       `json:"snapshot_id"`
	Scope          TaskScope    `json:"scope"`
	TaskID         string       `json:"task_id"`
	ModelID        string       `json:"model_id"`
	PricingGroup   string       `json:"pricing_group"`
	PricingVersion *string      `json:"pricing_version,omitempty"`
	PricingHash    *string      `json:"pricing_hash,omitempty"`
	State          PricingState `json:"state"`
	CapturedAt     time.Time    `json:"captured_at"`
}

type GatewayRequest struct {
	TaskID          string           `json:"task_id"`
	CallID          string           `json:"call_id"`
	ModelID         string           `json:"model_id"`
	Role            CallRole         `json:"role"`
	ProviderRequest provider.Request `json:"provider_request"`
}

type GatewayChunk struct {
	Chunk                  provider.Chunk          `json:"-"`
	Usage                  *NormalizedUsage        `json:"usage,omitempty"`
	ProviderRequestID      string                  `json:"provider_request_id,omitempty"`
	RawUsageRedacted       json.RawMessage         `json:"raw_usage_redacted,omitempty"`
	QuotaEvidence          *QuotaEvidenceRevision  `json:"quota_evidence,omitempty"`
	QuotaEvidenceRevisions []QuotaEvidenceRevision `json:"quota_evidence_revisions,omitempty"`
}

type CreditConfig struct {
	CreditPerCNY DecimalString `json:"credit_per_cny"`
	PricingGroup string        `json:"pricing_group"`
	Version      *string       `json:"version,omitempty"`
	FetchedAt    time.Time     `json:"fetched_at"`
}

type PartialUsageReport struct {
	TaskID         string             `json:"task_id"`
	Scope          cloud.BindingScope `json:"scope"`
	FreezeID       string             `json:"freeze_id"`
	Sequence       uint64             `json:"sequence"`
	ReportMode     ReportMode         `json:"report_mode"`
	Calls          []ModelCallUsage   `json:"calls"`
	IdempotencyKey string             `json:"idempotency_key"`
	CreatedAt      time.Time          `json:"created_at"`
}

type FinalUsageReport struct {
	TaskID         string             `json:"task_id"`
	Scope          cloud.BindingScope `json:"scope"`
	FreezeID       string             `json:"freeze_id"`
	Calls          []ModelCallUsage   `json:"calls"`
	ClientSummary  TaskUsageSummary   `json:"client_summary"`
	TaskResult     TaskResult         `json:"task_result"`
	IdempotencyKey string             `json:"idempotency_key"`
	CreatedAt      time.Time          `json:"created_at"`
}

type TaskUsageSummary struct {
	ModelCallCount int       `json:"model_call_count"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
}

type ModelCallUsage struct {
	CallID            string          `json:"call_id"`
	ModelID           string          `json:"model_id"`
	PricingGroup      string          `json:"pricing_group"`
	PricingVersion    *string         `json:"pricing_version,omitempty"`
	PricingHash       *string         `json:"pricing_hash,omitempty"`
	ProviderRequestID *string         `json:"provider_request_id,omitempty"`
	PricingEvidenceID string          `json:"pricing_evidence_id"`
	OccurredAt        time.Time       `json:"occurred_at"`
	Usage             NormalizedUsage `json:"usage"`
	RawUsageRedacted  json.RawMessage `json:"raw_usage_redacted,omitempty"`
}

type Settlement struct {
	SettlementID               string          `json:"settlement_id"`
	TaskID                     string          `json:"task_id"`
	State                      SettlementState `json:"state"`
	ChargedAmount              *DecimalString  `json:"charged_amount,omitempty"`
	ChargedFreeAmount          *DecimalString  `json:"charged_free_amount,omitempty"`
	ChargedSubscriptionAmount  *DecimalString  `json:"charged_subscription_amount,omitempty"`
	ReleasedFreeAmount         *DecimalString  `json:"released_free_amount,omitempty"`
	ReleasedSubscriptionAmount *DecimalString  `json:"released_subscription_amount,omitempty"`
	UncoveredAmount            *DecimalString  `json:"uncovered_amount,omitempty"`
	PricingStatus              *string         `json:"pricing_status,omitempty"`
	BalanceVersion             *uint64         `json:"balance_version,omitempty"`
	ObservedAt                 time.Time       `json:"observed_at"`
	ErrorCode                  *string         `json:"error_code,omitempty"`
}

type PendingReport struct {
	Scope            cloud.BindingScope `json:"scope"`
	TaskID           string             `json:"task_id"`
	FreezeID         string             `json:"freeze_id"`
	ContractVersion  string             `json:"contract_version"`
	PayloadHash      string             `json:"payload_hash,omitempty"`
	Kind             ReportKind         `json:"kind"`
	Sequence         uint64             `json:"sequence"`
	IdempotencyKey   string             `json:"idempotency_key"`
	CanonicalPayload json.RawMessage    `json:"canonical_payload"`
	State            ReportState        `json:"state"`
	AttemptCount     int                `json:"attempt_count"`
	NextAttemptAt    *time.Time         `json:"next_attempt_at,omitempty"`
	LastErrorCode    *string            `json:"last_error_code,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

type SubscriptionModelGateway interface {
	Stream(context.Context, cloud.BindingScope, GatewayRequest) (<-chan GatewayChunk, error)
}

type CloudUsageClient interface {
	UploadPartial(context.Context, cloud.BindingScope, PartialUsageReport) error
	FinalizeUsage(context.Context, cloud.BindingScope, FinalUsageReport) (Settlement, error)
	GetSettlement(context.Context, cloud.BindingScope, string) (Settlement, error)
}

type CreditConfigProvider interface {
	GetCreditConfig(context.Context, cloud.BindingScope) (CreditConfig, error)
}

type TaskStore interface {
	CreateTaskWithEvidence(context.Context, ModelTask, PricingEvidence) error
	RecordFreezeRequest(context.Context, TaskContext, *cloud.FreezeRequest, time.Time) error
	RecordFreezeResult(context.Context, TaskContext, *cloud.Freeze, *TaskEntitlementSnapshot, time.Time) error
	UpdateState(context.Context, TaskStateUpdate) error
	Get(context.Context, TaskScope, string) (ModelTask, error)
	ListRecoverable(context.Context, TaskScope) ([]ModelTask, error)
}

type TaskStateUpdate struct {
	Scope         TaskScope
	TaskID        string
	From          TaskState
	To            TaskState
	Result        *TaskResult
	LastErrorCode *string
	At            time.Time
}

type CallStore interface {
	Prepare(context.Context, TaskContext, ModelCall) error
	BeginAttempt(context.Context, TaskContext, string, time.Time) error
	Complete(context.Context, TaskContext, ModelCall) error
	ListByTask(context.Context, TaskScope, string) ([]ModelCall, error)
}

type ReportStore interface {
	EnqueueFinal(context.Context, TaskScope, string) (UsageReportBuildResult, *PendingReport, error)
	EnqueuePartial(context.Context, TaskScope, string) (UsageReportBuildResult, *PendingReport, error)
	GetByIdempotencyKey(context.Context, TaskScope, string) (PendingReport, bool, error)
	ListByTask(context.Context, TaskScope, string) ([]PendingReport, error)
	ClaimDue(context.Context, TaskScope, time.Time, time.Time) (PendingReport, bool, error)
	MarkAccepted(context.Context, TaskScope, string, int, time.Time) error
	MarkRetry(context.Context, TaskScope, string, int, time.Time, string, time.Time) error
	MarkBlocked(context.Context, TaskScope, string, int, string, time.Time) error
}

type FinalQuotaReportCorrector interface {
	CorrectUnacceptedFinal(context.Context, TaskScope, string, int, ErrorCode, time.Time) (PendingReport, error)
}

type SettlementObserver interface {
	CreditSettlementObserved(context.Context, cloud.BindingScope, string, string) error
}

// BillingRouteSelection is the user's explicit choice for a future task. It is
// deliberately separate from a model reference: a model never implies a
// billing source.
type BillingRouteSelection struct {
	ModelRef      string
	BillingSource BillingSource
}

func NewBillingRouteSelection(input BillingRouteSelection) (BillingRouteSelection, error) {
	selection := BillingRouteSelection{ModelRef: strings.TrimSpace(input.ModelRef), BillingSource: input.BillingSource}
	if err := requireTrimmed("model_ref", selection.ModelRef); err != nil {
		return BillingRouteSelection{}, err
	}
	if err := selection.BillingSource.Validate(); err != nil {
		return BillingRouteSelection{}, err
	}
	return selection, nil
}

// ResolvedBillingRoute is the adapter-owned, normalized form of one explicit
// selection. Provider is an adapter identity, never a credential.
type ResolvedBillingRoute struct {
	ModelRef      string
	ModelID       string
	Provider      string
	BillingSource BillingSource
}

func (r ResolvedBillingRoute) Validate() error {
	if _, err := NewBillingRouteSelection(BillingRouteSelection{ModelRef: r.ModelRef, BillingSource: r.BillingSource}); err != nil {
		return err
	}
	if err := requireTrimmed("model_id", r.ModelID); err != nil {
		return err
	}
	return requireTrimmed("provider", r.Provider)
}

var subscriptionTaskTransitions = map[TaskState]map[TaskState]bool{
	TaskCreated:       states(TaskFreezePending, TaskFailed, TaskCancelled),
	TaskFreezePending: states(TaskReady, TaskFailed, TaskCancelled, TaskRecoveryRequired),
	TaskReady:         states(TaskRunning, TaskFailed, TaskCancelled, TaskRecoveryRequired),
	TaskRunning:       states(TaskUsagePending, TaskFailed, TaskCancelled, TaskRecoveryRequired),
	TaskUsagePending:  states(TaskSettling, TaskFailed, TaskRecoveryRequired),
	TaskSettling:      states(TaskCompleted, TaskFailed, TaskRecoveryRequired),
	TaskFailed:        states(TaskUsagePending),
	TaskCancelled:     states(TaskUsagePending),
	TaskRecoveryRequired: states(
		TaskReady, TaskRunning, TaskUsagePending, TaskSettling, TaskFailed, TaskCancelled,
	),
}

var byokTaskTransitions = map[TaskState]map[TaskState]bool{
	TaskCreated: states(TaskReady, TaskFailed, TaskCancelled),
	TaskReady:   states(TaskRunning, TaskFailed, TaskCancelled, TaskRecoveryRequired),
	TaskRunning: states(TaskCompleted, TaskFailed, TaskCancelled, TaskRecoveryRequired),
	TaskRecoveryRequired: states(
		TaskReady, TaskRunning, TaskCompleted, TaskFailed, TaskCancelled,
	),
}

var callTransitions = map[CallState]map[CallState]bool{
	CallPrepared: states(CallRunning),
	CallRunning:  states(CallCompleted, CallFailed, CallCancelled, CallInterrupted, CallUsageMissing),
}

var reportTransitions = map[ReportState]map[ReportState]bool{
	ReportPending:    states(ReportDelivering),
	ReportDelivering: states(ReportAccepted, ReportRetry, ReportBlocked),
	ReportRetry:      states(ReportDelivering, ReportBlocked),
}

func ValidateTaskTransition(source BillingSource, from, to TaskState) error {
	if err := source.Validate(); err != nil {
		return err
	}
	transitions := subscriptionTaskTransitions
	if source == BillingNewAPIByok {
		transitions = byokTaskTransitions
	}
	if transitions[from][to] {
		return nil
	}
	return invalidTransition("task", from, to)
}

func ValidateTaskUpdate(source BillingSource, from, to TaskState, currentResult, nextResult *TaskResult) error {
	if err := ValidateTaskTransition(source, from, to); err != nil {
		return err
	}
	if err := ValidateTaskResultTransition(currentResult, nextResult); err != nil {
		return err
	}
	return validateTaskResult(source, to, nextResult)
}

func ValidateCallTransition(from, to CallState) error {
	if callTransitions[from][to] {
		return nil
	}
	return invalidTransition("call", from, to)
}

func ValidateReportTransition(from, to ReportState) error {
	if reportTransitions[from][to] {
		return nil
	}
	return invalidTransition("report", from, to)
}

func states[T comparable](values ...T) map[T]bool {
	out := make(map[T]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func invalidTransition(kind string, from, to any) error {
	return newError(ErrorInvalidStateTransition, fmt.Sprintf("invalid %s transition %s -> %s", kind, from, to))
}
