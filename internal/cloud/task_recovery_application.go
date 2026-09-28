package cloud

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"kwu/internal/userctx"
)

// TaskRecoveryRecord is the local pending state needed to resume a task
// after a restart or an interrupted start. CLOUD-09 deliberately limits
// recovery to records that were already saved locally.
type TaskRecoveryRecord struct {
	Scope          BindingScope
	TaskID         string
	FreezeID       string
	IdempotencyKey string
	HasUsage       bool
}

// NewTaskRecoveryRecord extracts the durable, scope-bound identifiers from a
// task-start result. It does not create a new freeze or infer ledger values.
func NewTaskRecoveryRecord(result TaskStartResult, hasUsage bool) (TaskRecoveryRecord, error) {
	record := TaskRecoveryRecord{
		Scope:          result.Scope,
		TaskID:         result.TaskID,
		FreezeID:       result.FreezeID,
		IdempotencyKey: result.IdempotencyKey,
		HasUsage:       hasUsage,
	}
	return record, record.Validate()
}

func (r TaskRecoveryRecord) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{"task_id", r.TaskID},
		{"freeze_id", r.FreezeID},
		{"idempotency_key", r.IdempotencyKey},
	} {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

// TaskRecoveryStore owns locally persisted pending records. Load is scoped by
// the complete binding identity; CLOUD-09 does not permit remote pending/list
// discovery as a substitute for this local state.
type TaskRecoveryStore interface {
	Save(context.Context, TaskRecoveryRecord) error
	Load(context.Context, BindingScope) ([]TaskRecoveryRecord, error)
	// Delete atomically removes the durable identity only when its current
	// value still equals the supplied snapshot.
	Delete(context.Context, TaskRecoveryRecord) error
}

// taskRecoveryEventGCStore durably associates terminal recovery snapshots with
// the exact normalized event that recovered them. Authorization is separate so
// only a successfully checkpointed event can delete its own batch.
type taskRecoveryEventGCStore interface {
	StageEventSync(context.Context, BindingScope, string, []TaskRecoveryRecord) error
	AuthorizeEventSync(context.Context, BindingScope, string) error
	LoadAuthorizedEventSync(context.Context, BindingScope, string) ([]TaskRecoveryRecord, error)
	LoadEventSyncGCIDs(context.Context, BindingScope) ([]string, error)
}

// MemoryTaskRecoveryStore is the L1 fake store. Its state deliberately
// outlives an application instance so tests can model process restart without
// claiming reinstall or cross-device recovery.
type MemoryTaskRecoveryStore struct {
	mu      sync.RWMutex
	records map[string]map[string]memoryTaskRecoveryEntry
}

type memoryTaskRecoveryEntry struct {
	Record     TaskRecoveryRecord
	Pending    map[string]struct{}
	Authorized map[string]struct{}
}

func NewMemoryTaskRecoveryStore() *MemoryTaskRecoveryStore {
	return &MemoryTaskRecoveryStore{records: make(map[string]map[string]memoryTaskRecoveryEntry)}
}

func (s *MemoryTaskRecoveryStore) Save(ctx context.Context, record TaskRecoveryRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.records[record.Scope.key()]
	if bucket == nil {
		bucket = make(map[string]memoryTaskRecoveryEntry)
		s.records[record.Scope.key()] = bucket
	}
	key := taskRecoveryRecordKey(record)
	if current, found := bucket[key]; found && current.Record == record {
		return nil
	}
	bucket[key] = memoryTaskRecoveryEntry{Record: record}
	return nil
}

func (s *MemoryTaskRecoveryStore) Load(ctx context.Context, scope BindingScope) ([]TaskRecoveryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	bucket := s.records[scope.key()]
	keys := make([]string, 0, len(bucket))
	for key := range bucket {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	records := make([]TaskRecoveryRecord, 0, len(keys))
	for _, key := range keys {
		records = append(records, bucket[key].Record)
	}
	s.mu.RUnlock()
	return records, nil
}

func (s *MemoryTaskRecoveryStore) Delete(ctx context.Context, record TaskRecoveryRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.records[record.Scope.key()]
	if bucket == nil {
		return nil
	}
	key := taskRecoveryRecordKey(record)
	if current, found := bucket[key]; !found || current.Record != record {
		return nil
	}
	delete(bucket, key)
	if len(bucket) == 0 {
		delete(s.records, record.Scope.key())
	}
	return nil
}

func (s *MemoryTaskRecoveryStore) StageEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
	records []TaskRecoveryRecord,
) error {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, eventID, records); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.records[scope.key()]
	for _, record := range records {
		key := taskRecoveryRecordKey(record)
		entry, found := bucket[key]
		if !found || entry.Record != record {
			continue
		}
		if _, authorized := entry.Authorized[eventID]; authorized {
			continue
		}
		if entry.Pending == nil {
			entry.Pending = make(map[string]struct{})
		}
		entry.Pending[eventID] = struct{}{}
		bucket[key] = entry
	}
	return nil
}

func (s *MemoryTaskRecoveryStore) AuthorizeEventSync(ctx context.Context, scope BindingScope, eventID string) error {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, eventID, nil); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.records[scope.key()] {
		if _, pending := entry.Pending[eventID]; !pending {
			continue
		}
		if entry.Authorized == nil {
			entry.Authorized = make(map[string]struct{})
		}
		entry.Authorized[eventID] = struct{}{}
		delete(entry.Pending, eventID)
		s.records[scope.key()][key] = entry
	}
	return nil
}

func (s *MemoryTaskRecoveryStore) LoadAuthorizedEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
) ([]TaskRecoveryRecord, error) {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, eventID, nil); err != nil {
		return nil, err
	}
	s.mu.RLock()
	records := make([]TaskRecoveryRecord, 0)
	for _, entry := range s.records[scope.key()] {
		if _, authorized := entry.Authorized[eventID]; authorized {
			records = append(records, entry.Record)
		}
	}
	s.mu.RUnlock()
	sort.Slice(records, func(i, j int) bool { return taskRecoveryRecordKey(records[i]) < taskRecoveryRecordKey(records[j]) })
	return records, nil
}

func (s *MemoryTaskRecoveryStore) LoadEventSyncGCIDs(ctx context.Context, scope BindingScope) ([]string, error) {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, "event", nil); err != nil {
		return nil, err
	}
	s.mu.RLock()
	ids := make(map[string]struct{})
	for _, entry := range s.records[scope.key()] {
		for eventID := range entry.Pending {
			ids[eventID] = struct{}{}
		}
		for eventID := range entry.Authorized {
			ids[eventID] = struct{}{}
		}
	}
	s.mu.RUnlock()
	ordered := make([]string, 0, len(ids))
	for eventID := range ids {
		ordered = append(ordered, eventID)
	}
	sort.Strings(ordered)
	return ordered, nil
}

func validateTaskRecoveryEventGCInput(
	ctx context.Context,
	scope BindingScope,
	eventID string,
	records []TaskRecoveryRecord,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("event_id", eventID); err != nil {
		return err
	}
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return err
		}
		if record.Scope != scope {
			return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: event recovery batch belongs to another binding"}
		}
	}
	return nil
}

func taskRecoveryRecordKey(record TaskRecoveryRecord) string {
	return record.TaskID + "\x00" + record.FreezeID + "\x00" + record.IdempotencyKey
}

type TaskRecoveryStatus string

const (
	TaskRecoveryContinueOrFinalize           TaskRecoveryStatus = "continue_or_finalize"
	TaskRecoveryTerminate                    TaskRecoveryStatus = "terminate"
	TaskRecoveryRetainForLateFinal           TaskRecoveryStatus = "retain_for_late_final"
	TaskRecoveryRetainForManualReview        TaskRecoveryStatus = "retain_for_manual_review"
	TaskRecoveryQueryAuthoritativeSettlement TaskRecoveryStatus = "query_authoritative_settlement"
	TaskRecoveryRetainForInvestigation       TaskRecoveryStatus = "retain_for_investigation"
	TaskRecoveryRetry                        TaskRecoveryStatus = "retry"
)

// TaskRecoveryDecision leaves execution, usage reporting, settlement and
// balance refresh to their respective owners. Record is retained verbatim so
// callers can persist or retry it without crossing a binding scope.
type TaskRecoveryDecision struct {
	Status TaskRecoveryStatus
	Record TaskRecoveryRecord
}

// TaskRecoveryExclusion identifies records an event gap must leave for the
// event's exact authority path. An empty FreezeID excludes every record for
// TaskID; the zero value excludes nothing.
type TaskRecoveryExclusion struct {
	TaskID   string
	FreezeID string
}

func (e TaskRecoveryExclusion) Validate() error {
	if e.TaskID == "" {
		if e.FreezeID != "" {
			return fmt.Errorf("cloud: task recovery exclusion freeze_id requires task_id")
		}
		return nil
	}
	if err := requireTrimmed("task_id", e.TaskID); err != nil {
		return err
	}
	if e.FreezeID != "" {
		return requireTrimmed("freeze_id", e.FreezeID)
	}
	return nil
}

func (e TaskRecoveryExclusion) matches(record TaskRecoveryRecord) bool {
	return e.TaskID != "" && record.TaskID == e.TaskID && (e.FreezeID == "" || record.FreezeID == e.FreezeID)
}

// TaskRecoveryApplication checks one locally saved freeze against the current
// binding. It never discovers remote pending work, starts a model, creates a
// freeze, or changes balances and ledger splits.
type TaskRecoveryApplication struct {
	credit            CloudCreditClient
	bindings          BindingResolver
	store             TaskRecoveryStore
	eventMu           sync.Mutex
	eventTerminations map[string]TaskRecoveryDecision
}

func NewTaskRecoveryApplication(credit CloudCreditClient, bindings BindingResolver) *TaskRecoveryApplication {
	return &TaskRecoveryApplication{credit: credit, bindings: bindings}
}

func NewTaskRecoveryApplicationWithStore(credit CloudCreditClient, bindings BindingResolver, store TaskRecoveryStore) *TaskRecoveryApplication {
	return &TaskRecoveryApplication{credit: credit, bindings: bindings, store: store}
}

// Recover returns a conservative action for one saved record. For retryable
// query failures the record is returned with the original error so the caller
// can retain it and schedule a later attempt.
func (a *TaskRecoveryApplication) Recover(ctx context.Context, actor userctx.ActorContext, record TaskRecoveryRecord) (TaskRecoveryDecision, error) {
	if err := record.Validate(); err != nil {
		return TaskRecoveryDecision{}, err
	}
	if err := a.currentScopeMatches(ctx, actor, record.Scope); err != nil {
		return TaskRecoveryDecision{}, err
	}
	if isNilDependency(a.credit) {
		return TaskRecoveryDecision{}, fmt.Errorf("cloud: task recovery credit client is not configured")
	}

	result, err := a.credit.GetFreeze(ctx, record.Scope, record.FreezeID)
	if err != nil {
		return taskRecoveryErrorDecision(record, err)
	}
	if err := result.Validate(); err != nil {
		return TaskRecoveryDecision{Status: TaskRecoveryRetainForInvestigation, Record: record}, err
	}
	if result.Freeze.Scope != record.Scope || result.Freeze.FreezeID != record.FreezeID || result.Freeze.TaskID != record.TaskID {
		return TaskRecoveryDecision{Status: TaskRecoveryRetainForInvestigation, Record: record}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: recovered freeze does not belong to saved task scope"}
	}

	return TaskRecoveryDecision{Status: taskRecoveryStatus(result.Freeze.Status, record.HasUsage), Record: record}, nil
}

// RecoverPending restores records already held by the local store for the
// current binding only. A terminated no-usage expired record is deleted;
// active, late-final, manual-review, settlement-query, investigation and
// retry records remain until their owning workflow reaches an authoritative
// terminal result.
func (a *TaskRecoveryApplication) RecoverPending(ctx context.Context, actor userctx.ActorContext) ([]TaskRecoveryDecision, error) {
	return a.recoverPending(ctx, actor, TaskRecoveryExclusion{}, false)
}

// RecoverPendingForEventSync preserves terminal records as durable event
// identities. Repeated event-stream recovery in the same process reuses a
// terminal decision instead of repeatedly querying the same freeze.
func (a *TaskRecoveryApplication) RecoverPendingForEventSync(
	ctx context.Context,
	actor userctx.ActorContext,
	exclusion TaskRecoveryExclusion,
) ([]TaskRecoveryDecision, error) {
	if err := exclusion.Validate(); err != nil {
		return nil, err
	}
	return a.recoverPending(ctx, actor, exclusion, true)
}

// StageEventSync durably binds only this event's terminal recovery snapshots
// before the event checkpoint is committed. Staging never authorizes deletion.
func (a *TaskRecoveryApplication) StageEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
	decisions []TaskRecoveryDecision,
) error {
	records := make([]TaskRecoveryRecord, 0, len(decisions))
	for _, decision := range decisions {
		if decision.Status != TaskRecoveryTerminate {
			continue
		}
		if decision.Record.Scope != scope {
			return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: event recovery decision belongs to another binding"}
		}
		records = append(records, decision.Record)
	}
	if len(records) == 0 {
		return validateTaskRecoveryEventGCInput(ctx, scope, eventID, nil)
	}
	store, err := a.eventGCStore()
	if err != nil {
		return err
	}
	return store.StageEventSync(ctx, scope, eventID, records)
}

// AcknowledgeEventSync promotes and removes only the batch staged for this
// exact event after its durable checkpoint has advanced.
func (a *TaskRecoveryApplication) AcknowledgeEventSync(ctx context.Context, scope BindingScope, eventID string) error {
	store, err := a.eventGCStore()
	if err != nil {
		return err
	}
	if err := store.AuthorizeEventSync(ctx, scope, eventID); err != nil {
		return err
	}
	return a.cleanupAuthorizedEventSync(ctx, store, scope, eventID)
}

// RetryAcknowledgedEventSync is called only when the durable checkpoint says
// this exact event ID was applied. It can therefore finish a pending promotion
// after a crash without authorizing any later, unstaged recovery record.
func (a *TaskRecoveryApplication) RetryAcknowledgedEventSync(ctx context.Context, scope BindingScope, eventID string) error {
	return a.AcknowledgeEventSync(ctx, scope, eventID)
}

// ReconcileAppliedEventSync finishes post-checkpoint garbage collection from
// durable local state even when the normalized stream does not replay an event
// after restart.
func (a *TaskRecoveryApplication) ReconcileAppliedEventSync(
	ctx context.Context,
	scope BindingScope,
	checkpoint EventCheckpoint,
) error {
	store, err := a.eventGCStore()
	if err != nil {
		return err
	}
	eventIDs, err := store.LoadEventSyncGCIDs(ctx, scope)
	if err != nil {
		return err
	}
	for _, eventID := range eventIDs {
		if !checkpoint.HasApplied(eventID) {
			continue
		}
		if err := a.AcknowledgeEventSync(ctx, scope, eventID); err != nil {
			return err
		}
	}
	return nil
}

func (a *TaskRecoveryApplication) cleanupAuthorizedEventSync(
	ctx context.Context,
	store taskRecoveryEventGCStore,
	scope BindingScope,
	eventID string,
) error {
	records, err := store.LoadAuthorizedEventSync(ctx, scope, eventID)
	if err != nil {
		return err
	}
	sort.Slice(records, func(i, j int) bool {
		return eventTaskRecoveryRecordKey(records[i]) < eventTaskRecoveryRecordKey(records[j])
	})
	for _, record := range records {
		if err := a.store.Delete(ctx, record); err != nil {
			return err
		}
		a.forgetEventTermination(record)
	}
	return nil
}

func (a *TaskRecoveryApplication) eventGCStore() (taskRecoveryEventGCStore, error) {
	if isNilDependency(a.store) {
		return nil, fmt.Errorf("cloud: task recovery store is not configured")
	}
	store, ok := a.store.(taskRecoveryEventGCStore)
	if !ok || isNilDependency(store) {
		return nil, fmt.Errorf("cloud: task recovery store does not support event GC")
	}
	return store, nil
}

func (a *TaskRecoveryApplication) forgetEventTermination(record TaskRecoveryRecord) {
	key := eventTaskRecoveryRecordKey(record)
	a.eventMu.Lock()
	if current, found := a.eventTerminations[key]; found && current.Record == record {
		delete(a.eventTerminations, key)
	}
	a.eventMu.Unlock()
}

func (a *TaskRecoveryApplication) recoverPending(
	ctx context.Context,
	actor userctx.ActorContext,
	exclusion TaskRecoveryExclusion,
	forEventSync bool,
) ([]TaskRecoveryDecision, error) {
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return nil, err
	}
	if isNilDependency(a.store) {
		return nil, fmt.Errorf("cloud: task recovery store is not configured")
	}
	records, err := a.store.Load(ctx, scope)
	if err != nil {
		return nil, err
	}

	decisions := make([]TaskRecoveryDecision, 0, len(records))
	var firstErr error
	for _, record := range records {
		if exclusion.matches(record) {
			continue
		}
		var decision TaskRecoveryDecision
		var include bool
		var recoverErr error
		if forEventSync {
			decision, include, recoverErr = a.recoverEventSyncRecord(ctx, actor, scope, record)
		} else {
			decision, include, recoverErr = a.recoverPendingRecord(ctx, actor, scope, record)
		}
		if include {
			decisions = append(decisions, decision)
		}
		if recoverErr != nil && firstErr == nil {
			firstErr = recoverErr
		}
	}
	return decisions, firstErr
}

// RecoverExact restores one event-named local recovery record.
func (a *TaskRecoveryApplication) RecoverExact(
	ctx context.Context,
	actor userctx.ActorContext,
	scope BindingScope,
	taskID string,
	freezeID string,
) (TaskRecoveryDecision, bool, error) {
	if err := validateExactTaskRecoveryInput(scope, taskID, freezeID); err != nil {
		return TaskRecoveryDecision{}, false, err
	}
	if err := a.currentScopeMatches(ctx, actor, scope); err != nil {
		return TaskRecoveryDecision{}, false, err
	}
	if isNilDependency(a.store) {
		return TaskRecoveryDecision{}, false, fmt.Errorf("cloud: task recovery store is not configured")
	}
	records, err := a.store.Load(ctx, scope)
	if err != nil {
		return TaskRecoveryDecision{}, false, err
	}
	matched, found, err := findExactTaskRecoveryRecord(records, scope, taskID, freezeID)
	if err != nil {
		return TaskRecoveryDecision{}, false, err
	}
	if !found {
		return TaskRecoveryDecision{}, false, nil
	}
	decision, err := a.recoverForEventSync(ctx, actor, matched)
	return decision, true, err
}

func validateExactTaskRecoveryInput(scope BindingScope, taskID, freezeID string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return err
	}
	return requireTrimmed("freeze_id", freezeID)
}

func findExactTaskRecoveryRecord(
	records []TaskRecoveryRecord,
	scope BindingScope,
	taskID, freezeID string,
) (TaskRecoveryRecord, bool, error) {
	var matched *TaskRecoveryRecord
	for i := range records {
		record := records[i]
		if record.Scope != scope {
			return TaskRecoveryRecord{}, false, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task recovery store returned another binding scope"}
		}
		if record.TaskID != taskID || record.FreezeID != freezeID {
			continue
		}
		if matched != nil {
			return TaskRecoveryRecord{}, false, fmt.Errorf("cloud: exact task recovery identity is ambiguous")
		}
		matched = &records[i]
	}
	if matched == nil {
		return TaskRecoveryRecord{}, false, nil
	}
	return *matched, true, nil
}

func (a *TaskRecoveryApplication) recoverEventSyncRecord(ctx context.Context, actor userctx.ActorContext, scope BindingScope, record TaskRecoveryRecord) (TaskRecoveryDecision, bool, error) {
	if record.Scope != scope {
		return TaskRecoveryDecision{}, false, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task recovery store returned another binding scope"}
	}
	decision, err := a.recoverForEventSync(ctx, actor, record)
	return decision, true, err
}

func (a *TaskRecoveryApplication) recoverForEventSync(ctx context.Context, actor userctx.ActorContext, record TaskRecoveryRecord) (TaskRecoveryDecision, error) {
	key := eventTaskRecoveryRecordKey(record)
	a.eventMu.Lock()
	cached, found := a.eventTerminations[key]
	a.eventMu.Unlock()
	if found && cached.Record == record {
		return cached, nil
	}

	decision, err := a.Recover(ctx, actor, record)
	if err != nil || decision.Status != TaskRecoveryTerminate {
		return decision, err
	}
	a.eventMu.Lock()
	if a.eventTerminations == nil {
		a.eventTerminations = make(map[string]TaskRecoveryDecision)
	}
	a.eventTerminations[key] = decision
	a.eventMu.Unlock()
	return decision, nil
}

func eventTaskRecoveryRecordKey(record TaskRecoveryRecord) string {
	hasUsage := "0"
	if record.HasUsage {
		hasUsage = "1"
	}
	return record.Scope.key() + "\x00" + taskRecoveryRecordKey(record) + "\x00" + hasUsage
}

func (a *TaskRecoveryApplication) recoverPendingRecord(ctx context.Context, actor userctx.ActorContext, scope BindingScope, record TaskRecoveryRecord) (TaskRecoveryDecision, bool, error) {
	if record.Scope != scope {
		return TaskRecoveryDecision{}, false, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task recovery store returned another binding scope"}
	}
	decision, err := a.Recover(ctx, actor, record)
	if err != nil || decision.Status != TaskRecoveryTerminate {
		return decision, true, err
	}
	return decision, true, a.store.Delete(ctx, record)
}

func (a *TaskRecoveryApplication) currentScopeMatches(ctx context.Context, actor userctx.ActorContext, saved BindingScope) error {
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return err
	}
	if scope != saved {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task recovery record belongs to another binding"}
	}
	return nil
}

func (a *TaskRecoveryApplication) currentScope(ctx context.Context, actor userctx.ActorContext) (BindingScope, error) {
	actor = actor.Normalize()
	if isNilDependency(a.bindings) {
		return BindingScope{}, fmt.Errorf("cloud: task recovery binding resolver is not configured")
	}
	current, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if err := current.Validate(); err != nil {
		return BindingScope{}, err
	}
	if current.Status != BindingBound || current.ActorID != actor.ActorID || current.ProfileID != actor.ProfileID {
		return BindingScope{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task recovery binding does not belong to actor"}
	}
	scope, err := current.Scope()
	if err != nil {
		return BindingScope{}, err
	}
	return scope, nil
}

func taskRecoveryErrorDecision(record TaskRecoveryRecord, err error) (TaskRecoveryDecision, error) {
	decision := TaskRecoveryDecision{Status: TaskRecoveryRetainForInvestigation, Record: record}
	var cloudErr *Error
	if errors.As(err, &cloudErr) {
		switch cloudErr.Code {
		case ErrorNetworkUnavailable:
			decision.Status = TaskRecoveryRetry
		case ErrorFreezeNotFound:
			decision.Status = TaskRecoveryRetainForInvestigation
		}
	}
	return decision, err
}

func taskRecoveryStatus(status FreezeStatus, hasUsage bool) TaskRecoveryStatus {
	switch status {
	case FreezeActive:
		return TaskRecoveryContinueOrFinalize
	case FreezeExpiredReleased:
		if hasUsage {
			return TaskRecoveryRetainForLateFinal
		}
		return TaskRecoveryTerminate
	case FreezeExpiredPendingReview, FreezeCreated, FreezeSettling:
		return TaskRecoveryRetainForManualReview
	case FreezeSettled, FreezeReleased:
		return TaskRecoveryQueryAuthoritativeSettlement
	default:
		return TaskRecoveryRetainForInvestigation
	}
}
