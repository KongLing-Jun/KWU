package modelbilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// SQLiteCallStore persists intent and terminal facts; it never dispatches a model call.
type SQLiteCallStore struct{ db *sql.DB }

func NewSQLiteCallStore(db *sql.DB) *SQLiteCallStore { return &SQLiteCallStore{db: db} }

func (s *SQLiteCallStore) Prepare(ctx context.Context, taskContext TaskContext, call ModelCall) error {
	call = preparedCallQuotaDefaults(call)
	if err := validatePrepareStore(s, call); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadCallTask(ctx, tx, taskContext)
	if err != nil {
		return err
	}
	if err := validatePreparedCallEvidence(ctx, tx, task, call); err != nil {
		return err
	}
	if err := insertOrVerifyPreparedCall(ctx, tx, task, call); err != nil {
		return err
	}
	return tx.Commit()
}

func validatePrepareStore(s *SQLiteCallStore, call ModelCall) error {
	if s == nil || s.db == nil {
		return newError(ErrorContractInvalid, "call store database is required")
	}
	if !cleanPreparedIntent(call) {
		return newError(ErrorContractInvalid, "prepared call cannot contain response or usage facts")
	}
	if err := validatePreparedCallQuota(call); err != nil {
		return err
	}
	return nil
}

func cleanPreparedIntent(call ModelCall) bool {
	return cleanPreparedLifecycle(call) && cleanPreparedResponseFacts(call) && cleanPreparedQuotaFacts(call)
}

func cleanPreparedLifecycle(call ModelCall) bool {
	return call.State == CallPrepared && call.AttemptCount == 0 && call.StartedAt == nil && call.FinishedAt == nil
}

func cleanPreparedResponseFacts(call ModelCall) bool {
	return call.ProviderRequestID == nil && call.LastErrorCode == nil && len(call.RawUsageRedacted) == 0 &&
		!hasUsageFact(call.Usage) && call.Usage.BreakdownStatus == ""
}

func cleanPreparedQuotaFacts(call ModelCall) bool {
	return call.OccurredAt == nil && call.QuotaEvidence == nil && call.QuotaReportLastErrorCode == nil &&
		len(call.quotaRequestIDs) == 0 && len(call.quotaEvidenceRevisions) == 0
}

func preparedCallQuotaDefaults(call ModelCall) ModelCall {
	if call.QuotaReportStatus == "" {
		call.QuotaReportStatus = QuotaReportUnreported
	}
	return call
}

func validatePreparedCallQuota(call ModelCall) error {
	if err := call.QuotaReportStatus.Validate(); err != nil {
		return err
	}
	if call.BillingSource == BillingSubscriptionPool {
		return call.Endpoint.Validate()
	}
	if call.Endpoint != "" {
		return newError(ErrorBillingSourceMismatch, "BYOK call cannot contain Maas endpoint")
	}
	return nil
}
func loadCallTask(ctx context.Context, tx *sql.Tx, value TaskContext) (ModelTask, error) {
	task, err := loadTask(ctx, tx, value.TaskID)
	if err != nil {
		return ModelTask{}, taskStoreLoadError(err)
	}
	if err := validatePersistedTaskContext(value, task); err != nil {
		return ModelTask{}, err
	}
	return task, nil
}
func validatePreparedCallEvidence(ctx context.Context, tx *sql.Tx, task ModelTask, call ModelCall) error {
	if task.State != TaskReady && task.State != TaskRunning {
		return newError(ErrorInvalidStateTransition, "call intent requires ready or running task")
	}
	if err := call.ValidateForTask(task); err != nil {
		return err
	}
	evidence, owner, err := loadPricingEvidence(ctx, tx, call.PricingEvidenceID)
	if err != nil {
		return taskStoreLoadError(err)
	}
	if owner.TaskID != task.TaskID || !owner.Scope.Equal(task.Scope) || owner.BillingSource != task.BillingSource {
		return newError(ErrorScopeMismatch, "call evidence does not belong to persisted task")
	}
	return evidence.ValidateForCall(task, call)
}
func insertOrVerifyPreparedCall(ctx context.Context, tx *sql.Tx, task ModelTask, call ModelCall) error {
	result, err := tx.ExecContext(ctx, `INSERT INTO model_calls (call_id,task_id,role,model_id,provider,endpoint,pricing_evidence_id,state,attempt_count,quota_report_status)
		SELECT ?,?,?,?,?,?,?,?,?,? WHERE EXISTS (
			SELECT 1 FROM model_tasks mt JOIN pricing_evidence pe ON pe.task_id=mt.task_id
			WHERE mt.task_id=? AND pe.evidence_id=? AND pe.model_id=?
		)`, call.CallID, task.TaskID, call.Role, call.ModelID, call.Provider, nullableMaasEndpoint(call.Endpoint), call.PricingEvidenceID, CallPrepared, 0, call.QuotaReportStatus, task.TaskID, call.PricingEvidenceID, call.ModelID)
	if err == nil {
		return exactlyOne(result, "prepare call")
	}
	if !isConstraint(err) {
		return err
	}
	existing, err := loadCall(ctx, tx, task, call.CallID)
	if err != nil {
		return err
	}
	if !callEqual(existing, call) {
		return newError(ErrorIdempotencyConflict, "call id reused with different content")
	}
	return nil
}

func (s *SQLiteCallStore) BeginAttempt(ctx context.Context, taskContext TaskContext, callID string, now time.Time) error {
	if err := validateBeginAttemptInput(s, callID, now); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadTask(ctx, tx, taskContext.TaskID)
	if err != nil {
		return taskStoreLoadError(err)
	}
	if err := validatePersistedTaskContext(taskContext, task); err != nil {
		return err
	}
	if task.State != TaskReady && task.State != TaskRunning {
		return newError(ErrorInvalidStateTransition, "call attempt requires ready or running task")
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_calls SET state=?,attempt_count=attempt_count+1,started_at=COALESCE(started_at,?) WHERE call_id=? AND task_id=? AND state IN (?,?)`, CallRunning, stamp(now), callID, task.TaskID, CallPrepared, CallRunning)
	if err != nil {
		return err
	}
	if err := exactlyOne(result, "begin call attempt"); err != nil {
		return err
	}
	return tx.Commit()
}

func validateBeginAttemptInput(s *SQLiteCallStore, callID string, now time.Time) error {
	if s == nil || s.db == nil || now.IsZero() {
		return newError(ErrorContractInvalid, "call store and time are required")
	}
	return requireTrimmed("call_id", callID)
}

func (s *SQLiteCallStore) Complete(ctx context.Context, taskContext TaskContext, call ModelCall) error {
	if s == nil || s.db == nil {
		return newError(ErrorContractInvalid, "call store database is required")
	}
	if !terminalCallState(call.State) || call.FinishedAt == nil {
		return newError(ErrorContractInvalid, "terminal call and finished time are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadCallTask(ctx, tx, taskContext)
	if err != nil {
		return err
	}
	if err := completePersistedCall(ctx, tx, task, call); err != nil {
		return err
	}
	return tx.Commit()
}

func completePersistedCall(ctx context.Context, tx *sql.Tx, task ModelTask, call ModelCall) error {
	existing, err := loadCallBase(ctx, tx, task, call.CallID)
	if err != nil {
		return taskStoreLoadError(err)
	}
	if err := validateCallCompletionIdentity(call, existing, task); err != nil {
		return err
	}
	candidate := terminalCallCandidate(call, task, existing)
	if err := validateTerminalCandidate(candidate, task); err != nil {
		return err
	}
	if existing.State != CallRunning {
		history, err := loadQuotaEvidenceHistory(ctx, tx, task.TaskID, call.CallID)
		if err != nil {
			return err
		}
		return verifyTerminalCall(existing, candidate, history)
	}
	if err := ValidateCallTransition(existing.State, candidate.State); err != nil {
		return err
	}
	return updateTerminalCall(ctx, tx, task, candidate)
}
func terminalCallCandidate(call ModelCall, task ModelTask, existing ModelCall) ModelCall {
	call = inheritTerminalCallContext(call, task, existing)
	if task.BillingSource != BillingSubscriptionPool {
		return clearTerminalQuotaFacts(call)
	}
	return normalizeSubscriptionTerminalCandidate(call)
}

func inheritTerminalCallContext(call ModelCall, task ModelTask, existing ModelCall) ModelCall {
	call.TaskID, call.Scope, call.BillingSource, call.AttemptCount, call.StartedAt = task.TaskID, cloneTaskScope(task.Scope), task.BillingSource, existing.AttemptCount, existing.StartedAt
	call.Endpoint = existing.Endpoint
	call.QuotaReportStatus = existing.QuotaReportStatus
	call.QuotaReportLastErrorCode = cloneOptionalString(existing.QuotaReportLastErrorCode)
	return call
}

func normalizeSubscriptionTerminalCandidate(call ModelCall) ModelCall {
	normalizeTerminalRequestIDs(&call)
	normalizeTerminalOccurredAt(&call)
	normalizeTerminalQuotaEvidence(&call)
	call.quotaRequestIDs = append([]string(nil), call.quotaRequestIDs...)
	call.quotaEvidenceRevisions = cloneQuotaEvidenceRevisions(call.quotaEvidenceRevisions)
	mergeTerminalEvidenceRequestID(&call)
	expandTerminalEvidenceRevisions(&call)
	return call
}

func normalizeTerminalRequestIDs(call *ModelCall) {
	if len(call.quotaRequestIDs) == 0 && call.ProviderRequestID != nil {
		call.quotaRequestIDs = []string{*call.ProviderRequestID}
	}
	if call.ProviderRequestID == nil && len(call.quotaRequestIDs) != 0 {
		requestID := call.quotaRequestIDs[0]
		call.ProviderRequestID = &requestID
	}
}

func normalizeTerminalOccurredAt(call *ModelCall) {
	if call.OccurredAt == nil && call.FinishedAt != nil {
		occurredAt := call.FinishedAt.UTC()
		call.OccurredAt = &occurredAt
	}
}

func normalizeTerminalQuotaEvidence(call *ModelCall) {
	if call.QuotaEvidence != nil {
		call.QuotaEvidence = cloneQuotaEvidence(call.QuotaEvidence)
		return
	}
	if call.FinishedAt == nil {
		return
	}
	var requestID *string
	if len(call.quotaRequestIDs) != 0 {
		requestID = &call.quotaRequestIDs[0]
	}
	evidence := missingQuotaEvidence(requestID, *call.FinishedAt)
	call.QuotaEvidence = &evidence
}

func mergeTerminalEvidenceRequestID(call *ModelCall) {
	if call.QuotaEvidence == nil || len(call.quotaRequestIDs) != 1 || call.QuotaEvidence.OneAPIRequestID != nil {
		return
	}
	requestID := call.quotaRequestIDs[0]
	merged := deriveQuotaEvidence(&requestID, call.QuotaEvidence.BillingQuotaRaw, call.QuotaEvidence.UsageQuotaRaw)
	merged.Revision = call.QuotaEvidence.Revision
	merged.ObservedAt = call.QuotaEvidence.ObservedAt
	call.QuotaEvidence = &merged
}

func expandTerminalEvidenceRevisions(call *ModelCall) {
	if call.QuotaEvidence == nil || len(call.quotaEvidenceRevisions) != 0 || len(call.quotaRequestIDs) <= 1 {
		return
	}
	call.quotaEvidenceRevisions = quotaRevisionsForRequestIDs(*call.QuotaEvidence, call.quotaRequestIDs)
	call.QuotaEvidence = cloneQuotaEvidence(&call.quotaEvidenceRevisions[0])
}

func clearTerminalQuotaFacts(call ModelCall) ModelCall {
	call.OccurredAt = nil
	call.QuotaEvidence = nil
	call.quotaRequestIDs = nil
	call.quotaEvidenceRevisions = nil
	return call
}

func quotaRevisionsForRequestIDs(base QuotaEvidenceRevision, requestIDs []string) []QuotaEvidenceRevision {
	revisions := make([]QuotaEvidenceRevision, len(requestIDs))
	for i, requestID := range requestIDs {
		next := deriveQuotaEvidence(&requestID, base.BillingQuotaRaw, base.UsageQuotaRaw)
		next.Revision = uint64(i + 1)
		next.ObservedAt = base.ObservedAt
		revisions[i] = next
	}
	return revisions
}

func cloneQuotaEvidenceRevisions(input []QuotaEvidenceRevision) []QuotaEvidenceRevision {
	output := make([]QuotaEvidenceRevision, len(input))
	for i := range input {
		output[i] = *cloneQuotaEvidence(&input[i])
	}
	return output
}

func terminalQuotaEvidenceRevisions(call ModelCall) []QuotaEvidenceRevision {
	if len(call.quotaEvidenceRevisions) != 0 {
		return cloneQuotaEvidenceRevisions(call.quotaEvidenceRevisions)
	}
	if call.QuotaEvidence == nil {
		return nil
	}
	return []QuotaEvidenceRevision{*cloneQuotaEvidence(call.QuotaEvidence)}
}

func validateTerminalQuotaEvidenceRevisions(call ModelCall) error {
	revisions := terminalQuotaEvidenceRevisions(call)
	if len(revisions) == 0 || !quotaEvidenceEqual(&revisions[0], call.QuotaEvidence) {
		return newError(ErrorContractInvalid, "terminal quota evidence history must start with revision 1")
	}
	for i := range revisions {
		if revisions[i].Revision != uint64(i+1) {
			return newError(ErrorContractInvalid, "terminal quota evidence revisions must be contiguous")
		}
		if err := revisions[i].Validate(); err != nil {
			return err
		}
		if i > 0 {
			if revisions[i].ObservedAt.Before(revisions[i-1].ObservedAt) {
				return newError(ErrorContractInvalid, "terminal quota evidence observed_at cannot move backwards")
			}
			if revisions[i-1].Status == QuotaEvidenceComplete && revisions[i].Status != QuotaEvidenceComplete {
				return newError(ErrorInvalidStateTransition, "complete quota evidence cannot regress")
			}
		}
	}
	return nil
}
func verifyTerminalCall(existing, candidate ModelCall, persisted []QuotaEvidenceRevision) error {
	existing.QuotaEvidence = nil
	candidateEvidence := candidate.QuotaEvidence
	candidate.QuotaEvidence = nil
	if !terminalCallState(existing.State) || !callEqual(existing, candidate) {
		return newError(ErrorIdempotencyConflict, "terminal call conflicts with persisted content")
	}
	candidate.QuotaEvidence = candidateEvidence
	expected := terminalQuotaEvidenceRevisions(candidate)
	if len(persisted) != len(expected) {
		return newError(ErrorIdempotencyConflict, "terminal quota evidence conflicts with persisted history")
	}
	for i := range persisted {
		if !quotaEvidenceEqual(&persisted[i], &expected[i]) {
			return newError(ErrorIdempotencyConflict, "terminal quota evidence conflicts with persisted history")
		}
	}
	return nil
}
func validateTerminalCandidate(call ModelCall, task ModelTask) error {
	if err := validateTerminalCandidateBase(call, task); err != nil {
		return err
	}
	if task.BillingSource == BillingSubscriptionPool {
		return validateSubscriptionTerminalCandidate(call)
	}
	return validateBYOKTerminalCandidate(call)
}

func validateTerminalCandidateBase(call ModelCall, task ModelTask) error {
	if err := call.ValidateForTask(task); err != nil {
		return err
	}
	if call.StartedAt == nil || call.FinishedAt.Before(*call.StartedAt) {
		return newError(ErrorContractInvalid, "call timestamps are invalid")
	}
	if err := call.QuotaReportStatus.Validate(); err != nil {
		return err
	}
	return nil
}

func validateSubscriptionTerminalCandidate(call ModelCall) error {
	if err := call.Endpoint.Validate(); err != nil {
		return err
	}
	if !validSubscriptionOccurrence(call) {
		return newError(ErrorContractInvalid, "subscription call occurred_at is outside its call interval")
	}
	if call.QuotaEvidence == nil || call.QuotaEvidence.Revision != 1 {
		return newError(ErrorContractInvalid, "subscription call completion requires quota evidence revision 1")
	}
	if err := call.QuotaEvidence.Validate(); err != nil {
		return err
	}
	return validateTerminalQuotaEvidenceRevisions(call)
}

func validSubscriptionOccurrence(call ModelCall) bool {
	return call.OccurredAt != nil && call.OccurredAt.Location() == time.UTC &&
		!call.OccurredAt.Before(*call.StartedAt) && !call.OccurredAt.After(*call.FinishedAt)
}

func validateBYOKTerminalCandidate(call ModelCall) error {
	if call.Endpoint != "" || call.OccurredAt != nil || call.QuotaEvidence != nil || call.QuotaReportStatus != QuotaReportUnreported {
		return newError(ErrorBillingSourceMismatch, "BYOK call cannot contain Maas quota facts")
	}
	return nil
}
func updateTerminalCall(ctx context.Context, tx *sql.Tx, task ModelTask, call ModelCall) error {
	usage, err := json.Marshal(call.Usage)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_calls SET provider_request_id=?,state=?,normalized_usage_json=?,raw_usage_redacted_json=?,finished_at=?,occurred_at=?,last_error_code=? WHERE call_id=? AND task_id=? AND state=?`, nullableString(call.ProviderRequestID), call.State, usage, nullableRaw(call.RawUsageRedacted), stamp(*call.FinishedAt), nullableTime(call.OccurredAt), nullableString(call.LastErrorCode), call.CallID, task.TaskID, CallRunning)
	if err != nil {
		return err
	}
	if err := exactlyOne(result, "complete call"); err != nil {
		return err
	}
	for _, revision := range terminalQuotaEvidenceRevisions(call) {
		if err := insertQuotaEvidenceRevision(ctx, tx, task.TaskID, call.CallID, revision); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteCallStore) ListByTask(ctx context.Context, scope TaskScope, taskID string) ([]ModelCall, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "call store database is required")
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return nil, taskStoreLoadError(err)
	}
	if !task.Scope.Equal(scope) {
		return nil, newError(ErrorScopeMismatch, "task scope does not match persisted task")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT call_id FROM model_calls WHERE task_id=? ORDER BY COALESCE(started_at,''), call_id`, taskID)
	if err != nil {
		return nil, err
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	var calls []ModelCall
	for _, id := range ids {
		call, err := loadCall(ctx, s.db, task, id)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, nil
}

type callQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadCall(ctx context.Context, q callQuerier, task ModelTask, callID string) (ModelCall, error) {
	call, err := loadCallBase(ctx, q, task, callID)
	if err != nil {
		return ModelCall{}, err
	}
	evidence, ok, err := loadCurrentQuotaEvidence(ctx, q, task.TaskID, callID)
	if err != nil {
		return ModelCall{}, err
	}
	if ok {
		call.QuotaEvidence = &evidence
	}
	return call, nil
}

func loadCallBase(ctx context.Context, q callQuerier, task ModelTask, callID string) (ModelCall, error) {
	var call ModelCall
	var providerID, endpoint, usageJSON, raw, lastError, started, finished, occurredAt, quotaReportError sql.NullString
	var role, state, quotaReportStatus string
	err := q.QueryRowContext(ctx, `SELECT call_id,task_id,role,model_id,provider,provider_request_id,endpoint,pricing_evidence_id,state,attempt_count,normalized_usage_json,raw_usage_redacted_json,started_at,finished_at,occurred_at,last_error_code,quota_report_status,quota_report_last_error_code FROM model_calls WHERE call_id=? AND task_id=?`, callID, task.TaskID).Scan(&call.CallID, &call.TaskID, &role, &call.ModelID, &call.Provider, &providerID, &endpoint, &call.PricingEvidenceID, &state, &call.AttemptCount, &usageJSON, &raw, &started, &finished, &occurredAt, &lastError, &quotaReportStatus, &quotaReportError)
	if err != nil {
		return ModelCall{}, err
	}
	call.Scope, call.BillingSource, call.Role, call.State = cloneTaskScope(task.Scope), task.BillingSource, CallRole(role), CallState(state)
	call.QuotaReportStatus = QuotaReportStatus(quotaReportStatus)
	if err := hydrateCall(&call, providerID, endpoint, usageJSON, raw, lastError, started, finished, occurredAt, quotaReportError); err != nil {
		return ModelCall{}, err
	}
	if err := call.ValidateForTask(task); err != nil {
		return ModelCall{}, err
	}
	if err := call.QuotaReportStatus.Validate(); err != nil {
		return ModelCall{}, persistedFieldError("call quota_report_status", err)
	}
	return call, nil
}

func hydrateCall(call *ModelCall, providerID, endpoint, usageJSON, raw, lastError, started, finished, occurredAt, quotaReportError sql.NullString) error {
	hydrateCallStrings(call, providerID, endpoint, lastError, quotaReportError)
	if err := hydrateCallTimes(call, started, finished, occurredAt); err != nil {
		return err
	}
	return hydrateCallUsage(call, usageJSON, raw)
}

func hydrateCallStrings(call *ModelCall, providerID, endpoint, lastError, quotaReportError sql.NullString) {
	if providerID.Valid {
		v := providerID.String
		call.ProviderRequestID = &v
	}
	if lastError.Valid {
		v := lastError.String
		call.LastErrorCode = &v
	}
	if endpoint.Valid {
		call.Endpoint = MaasEndpoint(endpoint.String)
	}
	if quotaReportError.Valid {
		v := quotaReportError.String
		call.QuotaReportLastErrorCode = &v
	}
}

func hydrateCallTimes(call *ModelCall, started, finished, occurredAt sql.NullString) error {
	if err := hydrateCallTime(&call.StartedAt, started, "call started_at"); err != nil {
		return err
	}
	if err := hydrateCallTime(&call.FinishedAt, finished, "call finished_at"); err != nil {
		return err
	}
	return hydrateCallTime(&call.OccurredAt, occurredAt, "call occurred_at")
}

func hydrateCallTime(target **time.Time, value sql.NullString, field string) error {
	if !value.Valid {
		return nil
	}
	parsed, err := parseStamp(value.String)
	if err != nil {
		return persistedFieldError(field, err)
	}
	*target = &parsed
	return nil
}

func hydrateCallUsage(call *ModelCall, usageJSON, raw sql.NullString) error {
	if usageJSON.Valid {
		if err := json.Unmarshal([]byte(usageJSON.String), &call.Usage); err != nil {
			return persistedFieldError("call normalized_usage_json", err)
		}
	}
	if raw.Valid {
		call.RawUsageRedacted = json.RawMessage(raw.String)
	}
	return nil
}
func terminalCallState(state CallState) bool {
	switch state {
	case CallCompleted, CallFailed, CallCancelled, CallInterrupted, CallUsageMissing:
		return true
	}
	return false
}
func nullableRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}
func nullableMaasEndpoint(endpoint MaasEndpoint) any {
	if endpoint == "" {
		return nil
	}
	return string(endpoint)
}
func callEqual(a, b ModelCall) bool {
	return callIdentityEqual(a, b) && callLifecycleEqual(a, b) && usageEqual(a.Usage, b.Usage) &&
		bytesEqual(a.RawUsageRedacted, b.RawUsageRedacted) && optionalStringEqual(a.LastErrorCode, b.LastErrorCode) &&
		quotaEvidenceEqual(a.QuotaEvidence, b.QuotaEvidence) &&
		a.QuotaReportStatus == b.QuotaReportStatus && optionalStringEqual(a.QuotaReportLastErrorCode, b.QuotaReportLastErrorCode)
}
func callIdentityEqual(a, b ModelCall) bool {
	return a.CallID == b.CallID && a.TaskID == b.TaskID && a.Scope.Equal(b.Scope) &&
		a.BillingSource == b.BillingSource && a.Role == b.Role && a.ModelID == b.ModelID &&
		a.Provider == b.Provider && a.Endpoint == b.Endpoint && optionalStringEqual(a.ProviderRequestID, b.ProviderRequestID) &&
		a.PricingEvidenceID == b.PricingEvidenceID
}
func callLifecycleEqual(a, b ModelCall) bool {
	return a.State == b.State && a.AttemptCount == b.AttemptCount &&
		optionalTimePointerEqual(a.StartedAt, b.StartedAt) && optionalTimePointerEqual(a.FinishedAt, b.FinishedAt) &&
		optionalTimePointerEqual(a.OccurredAt, b.OccurredAt)
}
func usageEqual(a, b NormalizedUsage) bool {
	return inputUsageEqual(a, b) && outputUsageEqual(a, b) && requestUsageEqual(a, b) &&
		a.BreakdownStatus == b.BreakdownStatus
}
func inputUsageEqual(a, b NormalizedUsage) bool {
	return optionalInt64Equal(a.InputTokens, b.InputTokens) && optionalInt64Equal(a.NormalInputTokens, b.NormalInputTokens) &&
		optionalInt64Equal(a.CachedInputTokens, b.CachedInputTokens) && optionalInt64Equal(a.CacheReadTokens, b.CacheReadTokens) &&
		optionalInt64Equal(a.CacheWriteTokens, b.CacheWriteTokens)
}
func outputUsageEqual(a, b NormalizedUsage) bool {
	return optionalInt64Equal(a.OutputTokens, b.OutputTokens) && optionalInt64Equal(a.CachedOutputTokens, b.CachedOutputTokens) &&
		optionalInt64Equal(a.ImageTokens, b.ImageTokens) && optionalInt64Equal(a.AudioInputTokens, b.AudioInputTokens) &&
		optionalInt64Equal(a.AudioOutputTokens, b.AudioOutputTokens)
}
func requestUsageEqual(a, b NormalizedUsage) bool {
	return optionalInt64Equal(a.RequestCount, b.RequestCount) && optionalInt64Equal(a.ProviderTotalTokens, b.ProviderTotalTokens)
}
func optionalInt64Equal(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func validateCallCompletionIdentity(input, persisted ModelCall, task ModelTask) error {
	if !completionMatchesTask(input, persisted, task) {
		return newError(ErrorBillingSourceMismatch, "completed call does not match persisted task")
	}
	if !input.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "completed call scope does not match persisted task")
	}
	if !completionMatchesIntent(input, persisted) {
		return newError(ErrorIdempotencyConflict, "completed call rewrites immutable intent")
	}
	return nil
}

func completionMatchesTask(input, persisted ModelCall, task ModelTask) bool {
	return input.CallID == persisted.CallID && input.TaskID == task.TaskID && input.BillingSource == task.BillingSource
}

func completionMatchesIntent(input, persisted ModelCall) bool {
	return input.Role == persisted.Role && input.ModelID == persisted.ModelID && input.Provider == persisted.Provider &&
		input.PricingEvidenceID == persisted.PricingEvidenceID && (input.Endpoint == "" || input.Endpoint == persisted.Endpoint)
}
