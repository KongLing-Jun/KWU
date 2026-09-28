// Package operationaldiag stores a bounded metadata-only projection of
// operational prompt events. It intentionally cannot accept prompt payloads,
// answers, tool data, arbitrary attributes, or filesystem paths.
package operationaldiag

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const SchemaVersion = 2

type AggregateKind string
type PromptKind string
type Kind string
type Stage string
type Outcome string
type Severity string
type PrivacyClass string
type ReceiptStatus string
type UserImpact string
type UserAction string
type EvidenceMode string

const (
	AggregatePrompt         AggregateKind = "prompt"
	AggregateTask           AggregateKind = "task"
	AggregateModelCall      AggregateKind = "model_call"
	AggregateToolCall       AggregateKind = "tool_call"
	AggregateSession        AggregateKind = "session"
	AggregateDashboardWrite AggregateKind = "dashboard_write"
	AggregatePlan           AggregateKind = "plan"
	PromptApproval          PromptKind    = "approval"
	PromptAsk               PromptKind    = "ask"

	KindPromptRegistered     Kind = "prompt.registered"
	KindPromptEmitted        Kind = "prompt.emitted"
	KindPromptReplayed       Kind = "prompt.replayed"
	KindPromptReplyRequested Kind = "prompt.reply_requested"
	KindPromptResolved       Kind = "prompt.resolved"
	KindPromptReplyFailed    Kind = "prompt.reply_failed"
	KindPromptCancelled      Kind = "prompt.cancelled"

	StageSource     Stage = "source"
	StageController Stage = "controller"
	StageTransport  Stage = "transport"
	StageProjection Stage = "projection"
	StageReply      Stage = "reply"
	StageAgent      Stage = "agent"

	OutcomeRegistered Outcome = "registered"
	OutcomePending    Outcome = "pending"
	OutcomeRequested  Outcome = "requested"
	OutcomeAccepted   Outcome = "accepted"
	OutcomeResolved   Outcome = "resolved"
	OutcomeCancelled  Outcome = "cancelled"
	OutcomeFailed     Outcome = "failed"
	OutcomeDuplicate  Outcome = "duplicate"
	OutcomeLate       Outcome = "late"
	OutcomeQueueFull  Outcome = "queue_full"
	OutcomeDisabled   Outcome = "disabled"

	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"

	PrivacyMetadataOnly PrivacyClass = "metadata_only"

	ReceiptAccepted        ReceiptStatus = "accepted"
	ReceiptAlreadyResolved ReceiptStatus = "already_resolved"
	ReceiptPromptNotFound  ReceiptStatus = "prompt_not_found"
	ReceiptWrongTab        ReceiptStatus = "wrong_tab"
	ReceiptCancelled       ReceiptStatus = "cancelled"
	ReceiptFailed          ReceiptStatus = "failed"

	ImpactNone              UserImpact = "none"
	ImpactWaiting           UserImpact = "waiting"
	ImpactWaitingForUser    UserImpact = "waiting_for_user"
	ImpactWaitingForBackend UserImpact = "waiting_for_backend"
	ImpactActionRequired    UserImpact = "action_required"
	ImpactBlocked           UserImpact = "blocked"

	ActionRetry  UserAction = "retry"
	ActionCancel UserAction = "cancel"
	ActionReport UserAction = "report"

	ModeDisabled   EvidenceMode = "disabled"
	ModeMemory     EvidenceMode = "memory"
	ModePersistent EvidenceMode = "persistent"
)

type Aggregate struct {
	Kind     AggregateKind `json:"kind"`
	ID       string        `json:"id"`
	Sequence uint64        `json:"sequence"`
}

type Scope struct {
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

type Record struct {
	SchemaVersion int           `json:"schemaVersion"`
	Ordinal       uint64        `json:"ordinal"`
	EventID       string        `json:"eventId"`
	OperationID   string        `json:"operationId"`
	CausationID   string        `json:"causationId,omitempty"`
	OccurrenceID  string        `json:"occurrenceId,omitempty"`
	Aggregate     Aggregate     `json:"aggregate"`
	Scope         Scope         `json:"scope"`
	PromptKind    PromptKind    `json:"promptKind"`
	Kind          Kind          `json:"kind"`
	Stage         Stage         `json:"stage"`
	Outcome       Outcome       `json:"outcome"`
	Severity      Severity      `json:"severity"`
	UserImpact    UserImpact    `json:"userImpact"`
	Retryable     bool          `json:"retryable"`
	PrivacyClass  PrivacyClass  `json:"privacyClass"`
	ReceiptStatus ReceiptStatus `json:"receiptStatus,omitempty"`
	Actions       []UserAction  `json:"actions,omitempty"`
	OccurredAt    time.Time     `json:"occurredAt"`
}

type Counters struct {
	Received                 uint64 `json:"received"`
	Recorded                 uint64 `json:"recorded"`
	Invalid                  uint64 `json:"invalid"`
	Duplicate                uint64 `json:"duplicate"`
	BufferFull               uint64 `json:"bufferFull"`
	QueueFull                uint64 `json:"queueFull"`
	Disabled                 uint64 `json:"disabled"`
	Late                     uint64 `json:"late"`
	DeliveryFailed           uint64 `json:"deliveryFailed"`
	ProjectorFailed          uint64 `json:"projectorFailed"`
	PersistenceEnqueued      uint64 `json:"persistenceEnqueued"`
	PersistenceWritten       uint64 `json:"persistenceWritten"`
	PersistenceQueueFull     uint64 `json:"persistenceQueueFull"`
	PersistenceWriteFailed   uint64 `json:"persistenceWriteFailed"`
	PersistenceRotated       uint64 `json:"persistenceRotated"`
	PersistenceFlushed       uint64 `json:"persistenceFlushed"`
	PersistenceDropped       uint64 `json:"persistenceDropped"`
	PersistenceRecovered     uint64 `json:"persistenceRecovered"`
	PersistenceTailTruncated uint64 `json:"persistenceTailTruncated"`
}

type counterState struct {
	received                 atomic.Uint64
	recorded                 atomic.Uint64
	invalid                  atomic.Uint64
	duplicate                atomic.Uint64
	bufferFull               atomic.Uint64
	queueFull                atomic.Uint64
	disabled                 atomic.Uint64
	late                     atomic.Uint64
	deliveryFailed           atomic.Uint64
	projectorFailed          atomic.Uint64
	persistenceEnqueued      atomic.Uint64
	persistenceWritten       atomic.Uint64
	persistenceQueueFull     atomic.Uint64
	persistenceWriteFailed   atomic.Uint64
	persistenceRotated       atomic.Uint64
	persistenceFlushed       atomic.Uint64
	persistenceDropped       atomic.Uint64
	persistenceRecovered     atomic.Uint64
	persistenceTailTruncated atomic.Uint64
}

type traceState struct {
	enabled     bool
	nextOrdinal uint64
	records     []Record
	seen        map[string]struct{}
}

type Option func(*Trace)

func WithOccurrenceGenerator(generate func() string) Option {
	return func(trace *Trace) {
		if generate != nil {
			trace.generateOccurrence = generate
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(trace *Trace) {
		if now != nil {
			trace.now = now
		}
	}
}

// Trace writes into one bounded ring under a short critical section and copies
// only when evidence is requested. This avoids copying the complete ring for
// every production event while keeping snapshots race-free.
type Trace struct {
	capacity           int
	mu                 sync.RWMutex
	state              traceState
	generateOccurrence func() string
	now                func() time.Time
	mode               EvidenceMode
	writer             *persistentWriter
	counters           counterState
}

func New(capacity int, options ...Option) *Trace {
	if capacity < 1 {
		capacity = 1
	}
	trace := &Trace{capacity: capacity, generateOccurrence: randomOccurrenceID, now: time.Now, mode: ModeMemory}
	for _, option := range options {
		if option != nil {
			option(trace)
		}
	}
	trace.state = traceState{enabled: true, seen: make(map[string]struct{}, capacity)}
	return trace
}

type PersistentOptions struct {
	Directory       string
	RunID           string
	Capacity        int
	QueueCapacity   int
	TerminalReserve int
	MaxFileBytes    int64
	MaxFiles        int
	Now             func() time.Time
	beforeWrite     func()
}

func NewPersistent(options PersistentOptions, traceOptions ...Option) (*Trace, error) {
	trace := New(options.Capacity, traceOptions...)
	trace.mode = ModePersistent
	if options.Now != nil {
		trace.now = options.Now
	}
	writer, err := newPersistentWriter(options, &trace.counters)
	if err != nil {
		return nil, err
	}
	trace.writer = writer
	trace.state.nextOrdinal = writer.CommittedOrdinal()
	return trace, nil
}

func NewDisabled(capacity int, options ...Option) *Trace {
	trace := New(capacity, options...)
	trace.mode = ModeDisabled
	trace.SetEnabled(false)
	return trace
}

func (t *Trace) Mode() EvidenceMode {
	if t == nil {
		return ModeDisabled
	}
	return t.mode
}

func (t *Trace) SetEnabled(enabled bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.state.enabled = enabled
	t.mu.Unlock()
}

func (t *Trace) Record(record Record) bool {
	if t == nil {
		return false
	}
	t.counters.received.Add(1)
	record, valid := normalizeRecord(record)
	if !valid {
		t.counters.invalid.Add(1)
		return false
	}
	t.mu.Lock()
	if !t.state.enabled {
		t.mu.Unlock()
		t.counters.disabled.Add(1)
		return false
	}
	if _, duplicate := t.state.seen[record.EventID]; duplicate {
		t.mu.Unlock()
		t.counters.duplicate.Add(1)
		return false
	}
	t.state.nextOrdinal++
	record.Ordinal = t.state.nextOrdinal
	full := len(t.state.records) == t.capacity
	if full {
		delete(t.state.seen, t.state.records[0].EventID)
		copy(t.state.records, t.state.records[1:])
		t.state.records[len(t.state.records)-1] = record
	} else {
		t.state.records = append(t.state.records, record)
	}
	t.state.seen[record.EventID] = struct{}{}
	t.mu.Unlock()
	if full {
		t.counters.bufferFull.Add(1)
	}
	t.counters.recorded.Add(1)
	if t.writer != nil {
		t.writer.Enqueue(record)
	}
	return true
}

func (t *Trace) ObserveTransport(outcome Outcome) {
	if t == nil {
		return
	}
	switch outcome {
	case OutcomeQueueFull:
		t.counters.queueFull.Add(1)
	case OutcomeDisabled:
		t.counters.disabled.Add(1)
	case OutcomeLate:
		t.counters.late.Add(1)
	case OutcomeFailed:
		t.counters.deliveryFailed.Add(1)
	}
}

func (t *Trace) NewOccurrenceID() string {
	if t == nil {
		return ""
	}
	value := normalizeOpaqueID(t.generateOccurrence())
	if value == "" {
		return randomOccurrenceID()
	}
	return value
}

func (t *Trace) Snapshot() []Record {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	records := cloneRecords(t.state.records)
	t.mu.RUnlock()
	return records
}

func (t *Trace) Counters() Counters {
	if t == nil {
		return Counters{}
	}
	return Counters{
		Received: t.counters.received.Load(), Recorded: t.counters.recorded.Load(), Invalid: t.counters.invalid.Load(),
		Duplicate: t.counters.duplicate.Load(), BufferFull: t.counters.bufferFull.Load(), QueueFull: t.counters.queueFull.Load(),
		Disabled: t.counters.disabled.Load(), Late: t.counters.late.Load(), DeliveryFailed: t.counters.deliveryFailed.Load(),
		ProjectorFailed:     t.counters.projectorFailed.Load(),
		PersistenceEnqueued: t.counters.persistenceEnqueued.Load(), PersistenceWritten: t.counters.persistenceWritten.Load(),
		PersistenceQueueFull: t.counters.persistenceQueueFull.Load(), PersistenceWriteFailed: t.counters.persistenceWriteFailed.Load(),
		PersistenceRotated: t.counters.persistenceRotated.Load(), PersistenceFlushed: t.counters.persistenceFlushed.Load(),
		PersistenceDropped:       t.counters.persistenceDropped.Load(),
		PersistenceRecovered:     t.counters.persistenceRecovered.Load(),
		PersistenceTailTruncated: t.counters.persistenceTailTruncated.Load(),
	}
}

type Evidence struct {
	SchemaVersion    int          `json:"schemaVersion"`
	CaptureID        string       `json:"captureId"`
	CapturedAt       time.Time    `json:"capturedAt"`
	Mode             EvidenceMode `json:"mode"`
	RunID            string       `json:"runId,omitempty"`
	Capacity         int          `json:"capacity"`
	FirstOrdinal     uint64       `json:"firstOrdinal"`
	LastOrdinal      uint64       `json:"lastOrdinal"`
	CommittedOrdinal uint64       `json:"committedOrdinal,omitempty"`
	Truncated        bool         `json:"truncated"`
	Quiescent        bool         `json:"quiescent"`
	Counters         Counters     `json:"counters"`
	Records          []Record     `json:"records"`
}

func (t *Trace) MarshalEvidence() ([]byte, error) { return json.MarshalIndent(t.Evidence(), "", "  ") }

func (t *Trace) Evidence() Evidence { return t.Capture(false) }

func (t *Trace) CaptureForTab(tabID string, quiescent bool) Evidence {
	evidence := t.Capture(quiescent)
	if tabID == "" {
		return evidence
	}
	records := make([]Record, 0, len(evidence.Records))
	for _, record := range evidence.Records {
		if record.Scope.TabID == tabID {
			records = append(records, record)
		}
	}
	evidence.Records = records
	return evidence
}

func (t *Trace) Capture(quiescent bool) Evidence {
	if t == nil {
		return Evidence{SchemaVersion: SchemaVersion, Mode: ModeDisabled, Records: []Record{}}
	}
	t.mu.RLock()
	records := cloneRecords(t.state.records)
	lastOrdinal := t.state.nextOrdinal
	t.mu.RUnlock()
	firstOrdinal := uint64(0)
	if len(records) > 0 {
		firstOrdinal = records[0].Ordinal
	}
	committed := uint64(0)
	runID := ""
	if t.writer != nil {
		committed = t.writer.CommittedOrdinal()
		runID = t.writer.runID
	}
	now := t.now
	if now == nil {
		now = time.Now
	}
	return Evidence{
		SchemaVersion: SchemaVersion, CaptureID: t.NewOccurrenceID(), CapturedAt: now().UTC(), Mode: t.mode, RunID: runID,
		Capacity: t.capacity, FirstOrdinal: firstOrdinal, LastOrdinal: lastOrdinal, CommittedOrdinal: committed,
		Truncated: firstOrdinal > 1 || t.counters.persistenceDropped.Load() > 0 || t.counters.persistenceTailTruncated.Load() > 0, Quiescent: quiescent,
		Counters: t.Counters(), Records: records,
	}
}

func (t *Trace) Flush(ctx context.Context) (uint64, error) {
	if t == nil || t.writer == nil {
		if t == nil {
			return 0, nil
		}
		t.mu.RLock()
		ordinal := t.state.nextOrdinal
		t.mu.RUnlock()
		return ordinal, nil
	}
	return t.writer.Flush(ctx)
}

func (t *Trace) Close(ctx context.Context) error {
	if t == nil || t.writer == nil {
		return nil
	}
	return t.writer.Close(ctx)
}

type IsolatedProjector struct {
	trace   *Trace
	project func(Record) bool
}

func NewIsolatedProjector(trace *Trace, project func(Record) bool) *IsolatedProjector {
	if project == nil && trace != nil {
		project = trace.Record
	}
	return &IsolatedProjector{trace: trace, project: project}
}

func (p *IsolatedProjector) Project(record Record) (accepted bool) {
	if p == nil || p.project == nil {
		if p != nil && p.trace != nil {
			p.trace.counters.disabled.Add(1)
		}
		return false
	}
	defer func() {
		if recover() != nil {
			accepted = false
			if p.trace != nil {
				p.trace.counters.projectorFailed.Add(1)
			}
		}
	}()
	return p.project(record)
}

func normalizeRecord(record Record) (Record, bool) {
	record.SchemaVersion = SchemaVersion
	record.Ordinal = 0
	record.EventID = normalizeOpaqueID(record.EventID)
	record.OperationID = normalizeOpaqueID(record.OperationID)
	record.CausationID = normalizeOpaqueID(record.CausationID)
	record.OccurrenceID = normalizeOpaqueID(record.OccurrenceID)
	record.Aggregate.Kind = normalizeAggregateKind(record.Aggregate.Kind)
	record.Aggregate.ID = normalizeOpaqueID(record.Aggregate.ID)
	record.Scope = normalizeScope(record.Scope)
	record.PromptKind = normalizePromptKind(record.PromptKind)
	record.Kind = normalizeKind(record.Kind)
	record.Stage = normalizeStage(record.Stage)
	record.Outcome = normalizeOutcome(record.Outcome)
	record.Severity = normalizeSeverity(record.Severity)
	record.UserImpact = normalizeUserImpact(record.UserImpact)
	record.PrivacyClass = PrivacyMetadataOnly
	record.ReceiptStatus = normalizeReceiptStatus(record.ReceiptStatus)
	record.Actions = normalizeActions(record.Actions)
	if !record.OccurredAt.IsZero() {
		record.OccurredAt = record.OccurredAt.UTC()
	}
	return record, record.EventID != "" && record.OperationID != "" && record.Aggregate.Kind != "" && record.Aggregate.ID != "" && scopeMatchesAggregate(record.Aggregate.Kind, record.Scope) && record.Kind != "" && record.Stage != "" && record.Outcome != "" && record.Severity != ""
}

func scopeMatchesAggregate(kind AggregateKind, scope Scope) bool {
	switch kind {
	case AggregatePrompt:
		return scope.PromptID != ""
	case AggregateTask:
		return scope.TaskID != ""
	case AggregateModelCall:
		return scope.CallID != "" && scope.TaskID != ""
	case AggregateToolCall:
		return scope.ToolCallID != ""
	case AggregateSession:
		return scope.SessionID != ""
	case AggregateDashboardWrite:
		return scope.DashboardWriteID != ""
	case AggregatePlan:
		return scope.PlanID != ""
	default:
		return false
	}
}

func normalizeScope(scope Scope) Scope {
	return Scope{
		TabID: normalizeOpaqueID(scope.TabID), SessionID: normalizeOpaqueID(scope.SessionID), TurnID: normalizeOpaqueID(scope.TurnID), CandidateID: normalizeOpaqueID(scope.CandidateID),
		PromptID: normalizeOpaqueID(scope.PromptID), TaskID: normalizeOpaqueID(scope.TaskID), CallID: normalizeOpaqueID(scope.CallID), ToolCallID: normalizeOpaqueID(scope.ToolCallID),
		PlanID: normalizeOpaqueID(scope.PlanID), DashboardWriteID: normalizeOpaqueID(scope.DashboardWriteID),
	}
}

func normalizeOpaqueID(value string) string {
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' || char == ':' {
			continue
		}
		return ""
	}
	return value
}

func normalizeAggregateKind(value AggregateKind) AggregateKind {
	switch value {
	case AggregatePrompt, AggregateTask, AggregateModelCall, AggregateToolCall, AggregateSession, AggregateDashboardWrite, AggregatePlan:
		return value
	}
	return ""
}
func normalizePromptKind(value PromptKind) PromptKind {
	if value == PromptApproval || value == PromptAsk {
		return value
	}
	return ""
}
func normalizeKind(value Kind) Kind {
	switch value {
	case KindPromptRegistered, KindPromptEmitted, KindPromptReplayed, KindPromptReplyRequested, KindPromptResolved, KindPromptReplyFailed, KindPromptCancelled:
		return value
	default:
		for _, prefix := range []string{"task.", "model_call.", "tool_call.", "session.", "dashboard_write.", "plan."} {
			if strings.HasPrefix(string(value), prefix) && len(value) <= 64 {
				return value
			}
		}
		return ""
	}
}
func normalizeStage(value Stage) Stage {
	switch value {
	case StageSource, StageController, StageTransport, StageProjection, StageReply, StageAgent, "persistence", "host":
		return value
	default:
		return ""
	}
}
func normalizeOutcome(value Outcome) Outcome {
	switch value {
	case OutcomeRegistered, OutcomePending, OutcomeRequested, OutcomeAccepted, OutcomeResolved, OutcomeCancelled, OutcomeFailed, OutcomeDuplicate, OutcomeLate, OutcomeQueueFull, OutcomeDisabled:
		return value
	default:
		if normalized := normalizeOpaqueID(string(value)); normalized != "" && len(normalized) <= 64 {
			return Outcome(normalized)
		}
		return ""
	}
}
func normalizeSeverity(value Severity) Severity {
	if value == SeverityInfo || value == SeverityWarn || value == SeverityError {
		return value
	}
	return ""
}
func normalizeUserImpact(value UserImpact) UserImpact {
	if value == ImpactNone || value == ImpactWaiting || value == ImpactWaitingForUser || value == ImpactWaitingForBackend || value == ImpactActionRequired || value == ImpactBlocked {
		return value
	}
	return ImpactNone
}
func normalizeReceiptStatus(value ReceiptStatus) ReceiptStatus {
	switch value {
	case "", ReceiptAccepted, ReceiptAlreadyResolved, ReceiptPromptNotFound, ReceiptWrongTab, ReceiptCancelled, ReceiptFailed:
		return value
	default:
		return ""
	}
}
func normalizeActions(actions []UserAction) []UserAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]UserAction, 0, 3)
	seen := map[UserAction]bool{}
	for _, action := range actions {
		if action != ActionRetry && action != ActionCancel && action != ActionReport {
			continue
		}
		if !seen[action] {
			seen[action] = true
			out = append(out, action)
		}
	}
	return out
}
func cloneRecords(records []Record) []Record {
	if len(records) == 0 {
		return []Record{}
	}
	out := make([]Record, len(records))
	for index, record := range records {
		out[index] = record
		out[index].Actions = append([]UserAction(nil), record.Actions...)
	}
	return out
}

var fallbackOccurrenceSequence atomic.Uint64

func randomOccurrenceID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "o-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("o-fallback-%016x", fallbackOccurrenceSequence.Add(1))
}
