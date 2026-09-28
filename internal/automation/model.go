package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kwu/internal/provider"
)

// ScheduleKind identifies how an automation task is scheduled.
type ScheduleKind string

const (
	// ScheduleKindOnce runs a task at one specific time.
	ScheduleKindOnce ScheduleKind = "once"
	// ScheduleKindInterval runs a task on a fixed interval.
	ScheduleKindInterval ScheduleKind = "interval"
	// ScheduleKindCalendar runs a task on calendar recurrence settings.
	ScheduleKindCalendar ScheduleKind = "calendar"
)

// RunStatus is the persisted state of an automation run.
type RunStatus string

const (
	// RunStatusQueued means a run has been created but not started.
	RunStatusQueued RunStatus = "queued"
	// RunStatusCheckingCondition means pre-run conditions are being evaluated.
	RunStatusCheckingCondition RunStatus = "checking_condition"
	// RunStatusSkipped means conditions prevented the run from executing.
	RunStatusSkipped RunStatus = "skipped"
	// RunStatusRunning means task execution is in progress.
	RunStatusRunning RunStatus = "running"
	// RunStatusSucceeded means the run completed successfully.
	RunStatusSucceeded RunStatus = "succeeded"
	// RunStatusFailed means the run completed with an error.
	RunStatusFailed RunStatus = "failed"
	// RunStatusBlocked means the run is blocked by approval or another gate.
	RunStatusBlocked RunStatus = "blocked"
	// RunStatusCanceled means the run was canceled before completion.
	RunStatusCanceled RunStatus = "canceled"
)

// RunErrorCode classifies why a run stopped or skipped before normal success.
type RunErrorCode string

const (
	RunErrorModelUnavailable     RunErrorCode = "model_unavailable"
	RunErrorWorkspaceMissing     RunErrorCode = "workspace_missing"
	RunErrorConditionRejected    RunErrorCode = "condition_rejected"
	RunErrorConditionInvalidJSON RunErrorCode = "condition_invalid_json"
	RunErrorLeaseUnavailable     RunErrorCode = "lease_unavailable"
	RunErrorApprovalTimeout      RunErrorCode = "approval_timeout"
	RunErrorRunnerFailed         RunErrorCode = "runner_failed"
	RunErrorSnapshotFailed       RunErrorCode = "snapshot_failed"
	RunErrorArchiveFailed        RunErrorCode = "archive_failed"
	RunErrorCanceled             RunErrorCode = "canceled"
	RunErrorDataQualityBlocked   RunErrorCode = "data_quality_blocked"
)

// Trigger records why an automation run was created.
type Trigger string

const (
	// TriggerSchedule marks a run created by the scheduler.
	TriggerSchedule Trigger = "schedule"
	// TriggerManual marks a run created by explicit user action.
	TriggerManual Trigger = "manual"
)

// ApprovalMode controls how background runs handle tool approval.
type ApprovalMode string

const (
	// ApprovalModeAsk requires explicit user approval before approved steps.
	ApprovalModeAsk ApprovalMode = "ask"
	// ApprovalModeAuto allows the system's normal approval behavior.
	ApprovalModeAuto ApprovalMode = "auto"
	// ApprovalModeYolo allows unattended tool execution when explicitly selected.
	ApprovalModeYolo ApprovalMode = "yolo"

	// DefaultApprovalMode is safe for unattended defaults and never implies YOLO.
	DefaultApprovalMode = ApprovalModeAuto

	// DefaultApprovalTimeout bounds unattended background approvals.
	DefaultApprovalTimeout       = 2 * time.Minute
	DefaultApprovalTimeoutString = "2m"
)

var (
	// ErrInvalidTask marks a task definition that cannot be normalized or stored.
	ErrInvalidTask = errors.New("automation: invalid task")
	// ErrInvalidSchedule marks unsupported or incomplete schedule configuration.
	ErrInvalidSchedule = errors.New("automation: invalid schedule")
	// ErrInvalidApprovalMode marks an unsupported approval policy value.
	ErrInvalidApprovalMode = errors.New("automation: invalid approval mode")
	// ErrInvalidApprovalTimeout marks an unparsable or non-positive approval timeout.
	ErrInvalidApprovalTimeout = errors.New("automation: invalid approval timeout")
	// ErrInvalidRunTransition marks an impossible run status transition.
	ErrInvalidRunTransition = errors.New("automation: invalid run transition")
	// ErrLeaseUnavailable marks a task whose lease is currently owned elsewhere.
	ErrLeaseUnavailable = errors.New("automation: lease unavailable")
	// ErrDuplicateTaskID marks two stored task definitions with the same normalized ID.
	ErrDuplicateTaskID = errors.New("automation: duplicate task id")
	// ErrTaskRevisionConflict marks an update based on a stale task revision.
	ErrTaskRevisionConflict = errors.New("automation: task revision conflict")
)

// Task is the JSON-friendly persisted definition of one automation task.
type Task struct {
	SchemaVersion          int                        `json:"schemaVersion,omitempty"`
	Revision               int64                      `json:"revision,omitempty"`
	ID                     string                     `json:"id"`
	Name                   string                     `json:"name"`
	Enabled                bool                       `json:"enabled"`
	Scope                  string                     `json:"scope,omitempty"`
	ProjectID              string                     `json:"projectId,omitempty"`
	WorkspaceRoot          string                     `json:"workspaceRoot,omitempty"`
	TopicID                string                     `json:"topicId,omitempty"`
	NewConversationEachRun bool                       `json:"newConversationEachRun,omitempty"`
	Prompt                 PromptConfig               `json:"prompt"`
	Schedule               Schedule                   `json:"schedule"`
	EffectiveRange         EffectiveRange             `json:"effectiveRange,omitempty"`
	MissedRunPolicy        MissedRunPolicy            `json:"missedRunPolicy,omitempty"`
	Capabilities           CapabilitySelection        `json:"capabilities,omitempty"`
	Delivery               DeliveryPolicy             `json:"delivery,omitempty"`
	Conditions             Conditions                 `json:"conditions,omitempty"`
	Execution              ExecutionPolicy            `json:"execution,omitempty"`
	Archive                ArchivePolicy              `json:"archive,omitempty"`
	BusinessDiagnostics    *BusinessDiagnosticsConfig `json:"businessDiagnostics,omitempty"`
	CreatedAt              time.Time                  `json:"createdAt"`
	UpdatedAt              time.Time                  `json:"updatedAt"`
	LastRunAt              time.Time                  `json:"lastRunAt,omitempty"`
	LastRunID              string                     `json:"lastRunId,omitempty"`
	UnknownFields          map[string]json.RawMessage `json:"-"`
}

// PromptConfig is the prompt and model metadata used when a task runs.
type PromptConfig struct {
	Model          string   `json:"model,omitempty"`
	BillingSource  string   `json:"billingSource,omitempty"`
	Skills         []string `json:"skills,omitempty"`
	Role           string   `json:"role,omitempty"`
	Prompt         string   `json:"prompt"`
	ConditionModel string   `json:"conditionModel,omitempty"`
}

// Schedule stores the first-version schedule configuration.
type Schedule struct {
	Kind     ScheduleKind     `json:"kind"`
	RunAt    time.Time        `json:"runAt,omitempty"`
	Every    string           `json:"every,omitempty"`
	Weekdays []string         `json:"weekdays,omitempty"`
	Calendar CalendarSchedule `json:"calendar,omitempty"`
	Timezone string           `json:"timezone,omitempty"`
	Window   TimeWindow       `json:"window,omitempty"`
}

// CalendarSchedule stores coarse calendar recurrence settings.
type CalendarSchedule struct {
	Frequency string   `json:"frequency,omitempty"`
	TimeOfDay string   `json:"timeOfDay,omitempty"`
	Weekdays  []string `json:"weekdays,omitempty"`
	MonthDays []int    `json:"monthDays,omitempty"`
	Months    []int    `json:"months,omitempty"`
}

// TimeWindow restricts a task to a daily local-time window.
type TimeWindow struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// Conditions stores optional pre-run condition configuration.
type Conditions struct {
	ConditionPrompt string `json:"conditionPrompt,omitempty"`
	FailureCooldown string `json:"failureCooldown,omitempty"`
}

// ExecutionPolicy stores per-task execution limits and approval behavior.
type ExecutionPolicy struct {
	ApprovalMode      ApprovalMode `json:"approvalMode,omitempty"`
	ApprovalTimeout   string       `json:"approvalTimeout,omitempty"`
	MaxConcurrentRuns int          `json:"maxConcurrentRuns,omitempty"`
}

// ArchivePolicy stores how much run archive data should be retained or shown.
type ArchivePolicy struct {
	Visible     bool   `json:"visible,omitempty"`
	Retention   string `json:"retention,omitempty"`
	KeepSession bool   `json:"keepSession,omitempty"`
	KeepEvents  bool   `json:"keepEvents,omitempty"`
}

// RunManifest is the machine-readable summary stored in each run directory.
type RunManifest struct {
	RunID           string             `json:"runId"`
	TaskID          string             `json:"taskId"`
	TaskName        string             `json:"taskName"`
	DisplayName     string             `json:"displayName,omitempty"`
	TaskRevision    int64              `json:"taskRevision,omitempty"`
	Scope           string             `json:"scope,omitempty"`
	ProjectID       string             `json:"projectId,omitempty"`
	Status          RunStatus          `json:"status"`
	Trigger         Trigger            `json:"trigger"`
	TriggerReason   string             `json:"triggerReason"`
	SkipReason      string             `json:"skipReason,omitempty"`
	ConditionReason string             `json:"conditionReason,omitempty"`
	Owner           string             `json:"owner,omitempty"`
	HostOwner       string             `json:"hostOwner,omitempty"`
	LeaseID         string             `json:"leaseId,omitempty"`
	StartedAt       time.Time          `json:"startedAt"`
	FinishedAt      time.Time          `json:"finishedAt,omitempty"`
	DurationMs      int64              `json:"durationMs"`
	Condition       ConditionResult    `json:"condition,omitempty"`
	Model           string             `json:"model,omitempty"`
	BillingSource   string             `json:"billingSource,omitempty"`
	WorkspaceRoot   string             `json:"workspaceRoot,omitempty"`
	ScheduledFor    time.Time          `json:"scheduledFor,omitempty"`
	Schedule        Schedule           `json:"schedule,omitempty"`
	EffectiveRange  EffectiveRange     `json:"effectiveRange,omitempty"`
	Capabilities    CapabilitySnapshot `json:"capabilities,omitempty"`
	Delivery        DeliveryPolicy     `json:"delivery,omitempty"`
	SessionPath     string             `json:"sessionPath,omitempty"`
	EventsPath      string             `json:"eventsPath,omitempty"`
	ResultPath      string             `json:"resultPath,omitempty"`
	Summary         string             `json:"summary,omitempty"`
	ErrorCode       RunErrorCode       `json:"errorCode,omitempty"`
	ErrorMessage    string             `json:"errorMessage,omitempty"`
	Error           string             `json:"error,omitempty"`
	Usage           *provider.Usage    `json:"usage,omitempty"`
}

// ConditionResult records the outcome of pre-run condition checks.
type ConditionResult struct {
	ShouldRun bool      `json:"shouldRun"`
	Reason    string    `json:"reason,omitempty"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitempty"`
}

// NormalizeTask trims canonical string fields, applies safe defaults, and validates.
func NormalizeTask(t Task) (Task, error) {
	if t.SchemaVersion == 0 {
		t.SchemaVersion = CurrentTaskSchemaVersion
	}
	if t.Revision == 0 {
		t.Revision = 1
	}
	t.ID = strings.TrimSpace(t.ID)
	t.Name = strings.TrimSpace(t.Name)
	t.Scope = strings.TrimSpace(t.Scope)
	t.ProjectID = strings.TrimSpace(t.ProjectID)
	t.WorkspaceRoot = strings.TrimSpace(t.WorkspaceRoot)
	if t.ProjectID != "" {
		// A Cloud Project is the task's logical owner. Its physical execution
		// target may still be Global when the project has no local workspace.
		t.Scope = "project"
	}
	t.TopicID = strings.TrimSpace(t.TopicID)
	t.Prompt.BillingSource = strings.TrimSpace(t.Prompt.BillingSource)
	t.Prompt.Model = strings.TrimSpace(t.Prompt.Model)
	t.MissedRunPolicy = normalizeMissedRunPolicy(t.MissedRunPolicy)
	t.Capabilities = normalizeCapabilitySelection(t.Capabilities, t.Prompt)
	t.Delivery = normalizeDeliveryPolicy(t.Delivery)
	t.Schedule.Kind = normalizeScheduleKind(t.Schedule.Kind)
	t.Execution.ApprovalMode = normalizeApprovalMode(t.Execution.ApprovalMode)
	t.Execution.ApprovalTimeout = normalizeApprovalTimeout(t.Execution.ApprovalTimeout)
	if t.BusinessDiagnostics != nil {
		cfg, err := NormalizeBusinessDiagnostics(*t.BusinessDiagnostics)
		if err != nil {
			return Task{}, err
		}
		t.BusinessDiagnostics = &cfg
	}
	if err := ValidateTask(t); err != nil {
		return Task{}, err
	}
	return t, nil
}

// ValidateTask rejects obviously invalid task definitions without mutating them.
func ValidateTask(t Task) error {
	if err := validateTaskIdentity(t); err != nil {
		return err
	}
	if err := validateTaskExecution(t.Execution); err != nil {
		return err
	}
	if err := validateTaskV2(t); err != nil {
		return err
	}
	return validateTaskPromptAndDiagnostics(t)
}

func validateTaskIdentity(t Task) error {
	if t.SchemaVersion < 0 || t.SchemaVersion > CurrentTaskSchemaVersion {
		return fmt.Errorf("%w: unsupported schemaVersion %d", ErrInvalidTask, t.SchemaVersion)
	}
	if t.Revision < 0 {
		return fmt.Errorf("%w: revision must not be negative", ErrInvalidTask)
	}
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidTask)
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidTask)
	}
	if !validScheduleKind(normalizeScheduleKind(t.Schedule.Kind)) {
		return fmt.Errorf("%w: kind %q", ErrInvalidSchedule, t.Schedule.Kind)
	}
	return nil
}

func validateTaskExecution(execution ExecutionPolicy) error {
	if !validApprovalMode(normalizeApprovalMode(execution.ApprovalMode)) {
		return fmt.Errorf("%w: mode %q", ErrInvalidApprovalMode, execution.ApprovalMode)
	}
	if !validApprovalTimeout(normalizeApprovalTimeout(execution.ApprovalTimeout)) {
		return fmt.Errorf("%w: %q", ErrInvalidApprovalTimeout, execution.ApprovalTimeout)
	}
	if execution.MaxConcurrentRuns < 0 || execution.MaxConcurrentRuns > 1 {
		return fmt.Errorf("%w: maxConcurrentRuns must be 0 or 1 until concurrent runs are supported", ErrInvalidTask)
	}
	return nil
}

func validateTaskV2(t Task) error {
	if !t.EffectiveRange.From.IsZero() && !t.EffectiveRange.Until.IsZero() && t.EffectiveRange.Until.Before(t.EffectiveRange.From) {
		return fmt.Errorf("%w: effective range ends before it starts", ErrInvalidTask)
	}
	if !validMissedRunPolicy(t.MissedRunPolicy) {
		return fmt.Errorf("%w: missedRunPolicy %q", ErrInvalidTask, t.MissedRunPolicy)
	}
	if err := validateCapabilitySelection(t.Capabilities); err != nil {
		return err
	}
	if err := validateDeliveryPolicy(t.Delivery); err != nil {
		return err
	}
	return nil
}

func validateTaskPromptAndDiagnostics(t Task) error {
	if !validBillingSource(strings.TrimSpace(t.Prompt.BillingSource)) {
		return fmt.Errorf("%w: billingSource %q", ErrInvalidTask, t.Prompt.BillingSource)
	}
	if t.BusinessDiagnostics != nil {
		if err := ValidateBusinessDiagnostics(*t.BusinessDiagnostics); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTask, err)
		}
	}
	return nil
}

func validBillingSource(source string) bool {
	switch source {
	case "", "subscription_pool", "newapi_byok":
		return true
	default:
		return false
	}
}

func normalizeScheduleKind(kind ScheduleKind) ScheduleKind {
	return ScheduleKind(strings.ToLower(strings.TrimSpace(string(kind))))
}

func validScheduleKind(kind ScheduleKind) bool {
	switch kind {
	case ScheduleKindOnce, ScheduleKindInterval, ScheduleKindCalendar:
		return true
	default:
		return false
	}
}

func normalizeApprovalMode(mode ApprovalMode) ApprovalMode {
	normalized := ApprovalMode(strings.ToLower(strings.TrimSpace(string(mode))))
	if normalized == "" {
		return DefaultApprovalMode
	}
	return normalized
}

func validApprovalMode(mode ApprovalMode) bool {
	switch mode {
	case ApprovalModeAsk, ApprovalModeAuto, ApprovalModeYolo:
		return true
	default:
		return false
	}
}

func normalizeApprovalTimeout(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultApprovalTimeoutString
	}
	return raw
}

func validApprovalTimeout(raw string) bool {
	timeout, err := time.ParseDuration(strings.TrimSpace(raw))
	return err == nil && timeout > 0
}

func (m *RunManifest) Transition(next RunStatus, reason string, at time.Time) error {
	if !allowedRunTransition(m.Status, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidRunTransition, m.Status, next)
	}
	now := at.UTC()
	m.Status = next
	switch next {
	case RunStatusRunning:
		if m.StartedAt.IsZero() {
			m.StartedAt = now
		}
	case RunStatusSkipped:
		m.SkipReason = strings.TrimSpace(reason)
		finishRunManifest(m, now)
	case RunStatusSucceeded, RunStatusFailed, RunStatusBlocked, RunStatusCanceled:
		finishRunManifest(m, now)
	}
	return nil
}

func finishRunManifest(m *RunManifest, at time.Time) {
	if m.FinishedAt.IsZero() {
		m.FinishedAt = at.UTC()
	}
	if !m.StartedAt.IsZero() && m.DurationMs == 0 {
		m.DurationMs = m.FinishedAt.Sub(m.StartedAt).Milliseconds()
	}
}

func allowedRunTransition(from, next RunStatus) bool {
	switch from {
	case "":
		return next == RunStatusQueued
	case RunStatusQueued:
		switch next {
		case RunStatusCheckingCondition, RunStatusRunning, RunStatusSkipped, RunStatusFailed, RunStatusBlocked, RunStatusCanceled:
			return true
		}
	case RunStatusCheckingCondition:
		switch next {
		case RunStatusRunning, RunStatusSkipped, RunStatusFailed, RunStatusBlocked, RunStatusCanceled:
			return true
		}
	case RunStatusRunning:
		switch next {
		case RunStatusSucceeded, RunStatusFailed, RunStatusBlocked, RunStatusCanceled, RunStatusSkipped:
			return true
		}
	}
	return false
}
