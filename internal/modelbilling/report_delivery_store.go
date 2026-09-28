package modelbilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"kwu/internal/cloud"
)

func (s *SQLiteReportStore) GetByIdempotencyKey(ctx context.Context, scope TaskScope, key string) (PendingReport, bool, error) {
	if s == nil || s.db == nil {
		return PendingReport{}, false, newError(ErrorContractInvalid, "report store database is required")
	}
	if err := requireTrimmed("idempotency_key", key); err != nil {
		return PendingReport{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return PendingReport{}, false, err
	}
	defer tx.Rollback()
	report, found, err := getPendingByKey(ctx, tx, scope, key)
	if err != nil || !found {
		return PendingReport{}, found, err
	}
	if err := tx.Commit(); err != nil {
		return PendingReport{}, false, err
	}
	return report, true, nil
}

func (s *SQLiteReportStore) ListByTask(ctx context.Context, scope TaskScope, taskID string) ([]PendingReport, error) {
	if err := validateReportStoreInput(s, taskID); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := loadReportTask(ctx, tx, scope, taskID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT idempotency_key FROM usage_reports WHERE task_id=? ORDER BY report_kind,sequence,idempotency_key`, taskID)
	if err != nil {
		return nil, err
	}
	keys, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	reports := make([]PendingReport, 0, len(keys))
	for _, key := range keys {
		report, found, err := getPendingByKey(ctx, tx, scope, key)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, newError(ErrorContractInvalid, "report disappeared during read transaction")
		}
		reports = append(reports, report)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return reports, nil
}

// ClaimDue atomically selects the oldest due row, including an expired
// delivering lease, and returns the incremented delivery attempt.
func (s *SQLiteReportStore) ClaimDue(ctx context.Context, scope TaskScope, now, leaseUntil time.Time) (PendingReport, bool, error) {
	if err := validateClaimInput(s, now, leaseUntil); err != nil {
		return PendingReport{}, false, err
	}
	scopeKey, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return PendingReport{}, false, err
	}
	due, err := s.hasDueReport(ctx, scopeKey, now)
	if err != nil || !due {
		return PendingReport{}, false, err
	}
	return s.claimDue(ctx, scope, scopeKey, now, leaseUntil)
}

func (s *SQLiteReportStore) claimDue(
	ctx context.Context,
	scope TaskScope,
	scopeKey string,
	now time.Time,
	leaseUntil time.Time,
) (PendingReport, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PendingReport{}, false, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `UPDATE usage_reports SET
		state=?, attempt_count=attempt_count+1, next_attempt_at=?, updated_at=?
		WHERE idempotency_key=(
			SELECT r.idempotency_key FROM usage_reports r
			JOIN model_tasks t ON t.task_id=r.task_id
			WHERE t.scope_key=? AND t.billing_source=? AND r.contract_version=?
				AND r.state IN (?,?,?)
				AND r.next_attempt_at<=?
			ORDER BY r.next_attempt_at,r.created_at,r.idempotency_key
			LIMIT 1
		)
		AND contract_version=? AND state IN (?,?,?) AND next_attempt_at<=?
		RETURNING idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,
			next_attempt_at,last_error_code,created_at,updated_at,contract_version,payload_hash`,
		ReportDelivering, reportQueueStamp(leaseUntil), reportQueueStamp(now), scopeKey, BillingSubscriptionPool, QuotaContractV1,
		ReportPending, ReportRetry, ReportDelivering, reportQueueStamp(now),
		QuotaContractV1, ReportPending, ReportRetry, ReportDelivering, reportQueueStamp(now))
	record, err := scanPendingRow(row)
	if err == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return PendingReport{}, false, err
		}
		return PendingReport{}, false, nil
	}
	if err != nil {
		return PendingReport{}, false, err
	}
	report, err := hydratePendingRow(ctx, tx, scope, record)
	if err != nil {
		found, err := finishFailedClaimHydration(ctx, tx, record, err, now)
		return PendingReport{}, found, err
	}
	if report.Kind == ReportFinal {
		if err := updateQuotaWorkflowInTx(ctx, tx, report.TaskID, QuotaWorkflowFinalUploading, nil, nil,
			now, QuotaWorkflowFinalUploadPending, QuotaWorkflowRetryPending); err != nil {
			return PendingReport{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return PendingReport{}, false, err
	}
	return report, true, nil
}

// hasDueReport keeps an idle poll read-only. ClaimDue still performs the
// authoritative selection and CAS, so a concurrent claimant remains safe and
// a report inserted after this probe is simply handled by the next poll.
func (s *SQLiteReportStore) hasDueReport(ctx context.Context, scopeKey string, now time.Time) (bool, error) {
	var due bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM usage_reports r
		JOIN model_tasks t ON t.task_id=r.task_id
		WHERE t.scope_key=? AND t.billing_source=? AND r.contract_version=?
			AND r.state IN (?,?,?) AND r.next_attempt_at<=?
	)`, scopeKey, BillingSubscriptionPool, QuotaContractV1, ReportPending, ReportRetry, ReportDelivering, reportQueueStamp(now)).Scan(&due)
	return due, err
}

func finishFailedClaimHydration(
	ctx context.Context,
	tx *sql.Tx,
	record pendingRow,
	hydrationErr error,
	now time.Time,
) (bool, error) {
	code, isolate := claimDueIsolationErrorCode(hydrationErr)
	if !isolate {
		return false, hydrationErr
	}
	workflowErr, err := isolateClaimedReport(ctx, tx, record, code, now)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if workflowErr != nil {
		return true, errors.Join(hydrationErr, fmt.Errorf("isolate report workflow: %w", workflowErr))
	}
	return true, hydrationErr
}

func claimDueIsolationErrorCode(err error) (ErrorCode, bool) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", false
	}
	code := ErrorCodeOf(err)
	switch code {
	case ErrorContractInvalid, ErrorScopeMismatch, ErrorIdempotencyConflict, ErrorBillingSourceMismatch,
		ErrorQuotaEvidenceIncomplete, ErrorQuotaCallIDConflict, ErrorQuotaRequestIDConflict,
		ErrorQuotaRequestIDReused, ErrorQuotaSummaryMismatch, ErrorQuotaFinalPayloadConflict,
		ErrorQuotaFinalIdempotencyConflict, ErrorQuotaPartialSnapshotConflict, ErrorQuotaTimeInvalid,
		ErrorLegacyReportContract:
		return code, true
	default:
		return "", false
	}
}

func isolateClaimedReport(
	ctx context.Context,
	tx *sql.Tx,
	record pendingRow,
	code ErrorCode,
	now time.Time,
) (workflowErr error, fatalErr error) {
	result, err := tx.ExecContext(ctx, `UPDATE usage_reports SET
		state=?,next_attempt_at=NULL,last_error_code=?,updated_at=?
		WHERE idempotency_key=? AND state=? AND attempt_count=?`,
		ReportBlocked, string(code), reportQueueStamp(now), record.key, ReportDelivering, record.attempts)
	if err != nil {
		return nil, err
	}
	if err := exactlyOne(result, "isolate claimed report"); err != nil {
		return nil, err
	}
	if record.contractVersion != QuotaContractV1 {
		return nil, nil
	}
	codeText := string(code)
	if _, err := tx.ExecContext(ctx, `UPDATE model_calls SET quota_report_last_error_code=?
		WHERE task_id=? AND EXISTS (SELECT 1 FROM usage_report_calls rc
			WHERE rc.idempotency_key=? AND rc.task_id=model_calls.task_id AND rc.call_id=model_calls.call_id)`,
		codeText, record.taskID, record.key); err != nil {
		return nil, err
	}
	if ReportKind(record.kind) == ReportFinal {
		workflowErr := updateQuotaWorkflowInTx(ctx, tx, record.taskID, QuotaWorkflowFailedManualRequired, nil, &codeText,
			now, QuotaWorkflowFinalUploadPending, QuotaWorkflowFinalUploading, QuotaWorkflowRetryPending)
		return workflowErr, nil
	}
	return nil, nil
}

func validateClaimInput(store *SQLiteReportStore, now, leaseUntil time.Time) error {
	if store == nil || store.db == nil || now.IsZero() || leaseUntil.IsZero() || !leaseUntil.After(now) {
		return newError(ErrorContractInvalid, "report store, claim time, and future lease are required")
	}
	return nil
}

func (s *SQLiteReportStore) MarkAccepted(ctx context.Context, scope TaskScope, key string, expectedAttempt int, at time.Time) error {
	return s.markDelivery(ctx, scope, key, expectedAttempt, ReportAccepted, nil, nil, at)
}

func (s *SQLiteReportStore) MarkRetry(ctx context.Context, scope TaskScope, key string, expectedAttempt int, next time.Time, code string, at time.Time) error {
	if next.IsZero() || next.Before(at) {
		return newError(ErrorContractInvalid, "retry time must not precede update time")
	}
	return s.markDelivery(ctx, scope, key, expectedAttempt, ReportRetry, &next, &code, at)
}

func (s *SQLiteReportStore) MarkBlocked(ctx context.Context, scope TaskScope, key string, expectedAttempt int, code string, at time.Time) error {
	return s.markDelivery(ctx, scope, key, expectedAttempt, ReportBlocked, nil, &code, at)
}

func (s *SQLiteReportStore) markDelivery(ctx context.Context, scope TaskScope, key string, expectedAttempt int, state ReportState, next *time.Time, code *string, at time.Time) error {
	if err := validateDeliveryMarkInput(s, key, expectedAttempt, state, code, at); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	report, err := loadDeliveringReport(ctx, tx, scope, key, expectedAttempt)
	if err != nil {
		return err
	}
	if err := validateFinalAcceptance(ctx, tx, scope, report, state); err != nil {
		return err
	}
	if err := updateReportDeliveryRow(ctx, tx, report, expectedAttempt, state, next, code, at); err != nil {
		return err
	}
	if err := updateReportCallsAfterDelivery(ctx, tx, report, state, code); err != nil {
		return err
	}
	if err := updateReportDeliveryWorkflow(ctx, tx, report, state, code, at); err != nil {
		return err
	}
	return tx.Commit()
}

func loadDeliveringReport(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	key string,
	expectedAttempt int,
) (PendingReport, error) {
	report, found, err := getPendingByKey(ctx, tx, scope, key)
	if err != nil {
		return PendingReport{}, err
	}
	if !found {
		return PendingReport{}, newError(ErrorContractInvalid, "persisted report not found")
	}
	if report.ContractVersion != QuotaContractV1 {
		return PendingReport{}, newError(ErrorLegacyReportContract, "legacy token report cannot be delivered")
	}
	if report.State != ReportDelivering || report.AttemptCount != expectedAttempt {
		return PendingReport{}, newError(ErrorInvalidStateTransition, "delivery attempt is stale or no longer delivering")
	}
	return report, nil
}

func validateFinalAcceptance(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	report PendingReport,
	state ReportState,
) error {
	if state != ReportAccepted || report.Kind != ReportFinal {
		return nil
	}
	task, err := loadReportTask(ctx, tx, scope, report.TaskID)
	if err != nil {
		return err
	}
	var final FinalQuotaReport
	if err := json.Unmarshal(report.CanonicalPayload, &final); err != nil {
		return newError(ErrorContractInvalid, "decode final quota report for acceptance")
	}
	return rejectAcceptedRequestIDReuse(ctx, tx, task, final.Calls)
}

func updateReportDeliveryRow(
	ctx context.Context,
	tx *sql.Tx,
	report PendingReport,
	expectedAttempt int,
	state ReportState,
	next *time.Time,
	code *string,
	at time.Time,
) error {
	result, err := tx.ExecContext(ctx, `UPDATE usage_reports SET state=?,next_attempt_at=?,last_error_code=?,updated_at=?
		WHERE idempotency_key=? AND contract_version=? AND state=? AND attempt_count=?`,
		state, nullableReportQueueTime(next), nullableString(code), reportQueueStamp(at), report.IdempotencyKey, QuotaContractV1,
		ReportDelivering, expectedAttempt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return newError(ErrorInvalidStateTransition, "delivery attempt is stale or no longer delivering")
	}
	return nil
}

func updateReportDeliveryWorkflow(
	ctx context.Context,
	tx *sql.Tx,
	report PendingReport,
	state ReportState,
	code *string,
	at time.Time,
) error {
	if report.Kind != ReportFinal {
		return nil
	}
	return updateFinalWorkflowAfterDelivery(ctx, tx, report.TaskID, state, code, at)
}

func updateReportCallsAfterDelivery(ctx context.Context, tx *sql.Tx, report PendingReport, state ReportState, code *string) error {
	switch state {
	case ReportAccepted:
		if report.Kind == ReportPartial {
			_, err := tx.ExecContext(ctx, `UPDATE model_calls SET
				quota_report_status=CASE WHEN quota_report_status=? THEN ? ELSE quota_report_status END,
				quota_report_last_error_code=NULL
				WHERE task_id=? AND quota_report_status IN (?,?)
				AND EXISTS (SELECT 1 FROM usage_report_calls rc
					WHERE rc.idempotency_key=? AND rc.task_id=model_calls.task_id AND rc.call_id=model_calls.call_id)`,
				QuotaReportUnreported, QuotaReportPartialReported, report.TaskID,
				QuotaReportUnreported, QuotaReportPartialReported, report.IdempotencyKey)
			return err
		}
		var invalid int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_calls c
			JOIN usage_report_calls rc ON rc.task_id=c.task_id AND rc.call_id=c.call_id
			WHERE rc.idempotency_key=? AND c.quota_report_status<>?`, report.IdempotencyKey, QuotaReportFinalPending).Scan(&invalid); err != nil {
			return err
		}
		if invalid != 0 {
			return newError(ErrorInvalidStateTransition, "final report call status is not pending")
		}
		_, err := tx.ExecContext(ctx, `UPDATE model_calls SET quota_report_status=?,quota_report_last_error_code=NULL
			WHERE task_id=? AND EXISTS (SELECT 1 FROM usage_report_calls rc
				WHERE rc.idempotency_key=? AND rc.task_id=model_calls.task_id AND rc.call_id=model_calls.call_id)`,
			QuotaReportFinalReported, report.TaskID, report.IdempotencyKey)
		return err
	case ReportRetry, ReportBlocked:
		query := `UPDATE model_calls SET quota_report_last_error_code=?
			WHERE task_id=? AND EXISTS (SELECT 1 FROM usage_report_calls rc
				WHERE rc.idempotency_key=? AND rc.task_id=model_calls.task_id AND rc.call_id=model_calls.call_id)`
		args := []any{nullableString(code), report.TaskID, report.IdempotencyKey}
		if report.Kind == ReportPartial {
			query += ` AND quota_report_status IN (?,?)`
			args = append(args, QuotaReportUnreported, QuotaReportPartialReported)
		}
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	default:
		return newError(ErrorContractInvalid, "unsupported report delivery result")
	}
}

func updateFinalWorkflowAfterDelivery(ctx context.Context, tx *sql.Tx, taskID string, state ReportState, code *string, at time.Time) error {
	switch state {
	case ReportAccepted:
		return updateQuotaWorkflowInTx(ctx, tx, taskID, QuotaWorkflowSettlementPending, nil, nil, at, QuotaWorkflowFinalUploading)
	case ReportRetry:
		stage := QuotaRetryFinalUpload
		return updateQuotaWorkflowInTx(ctx, tx, taskID, QuotaWorkflowRetryPending, &stage, code, at, QuotaWorkflowFinalUploading)
	case ReportBlocked:
		return updateQuotaWorkflowInTx(ctx, tx, taskID, QuotaWorkflowFailedManualRequired, nil, code, at, QuotaWorkflowFinalUploading)
	default:
		return newError(ErrorContractInvalid, "unsupported final quota workflow result")
	}
}

func updateQuotaWorkflowInTx(
	ctx context.Context,
	tx *sql.Tx,
	taskID string,
	to QuotaWorkflowStatus,
	retryStage *QuotaRetryStage,
	lastError *string,
	at time.Time,
	from ...QuotaWorkflowStatus,
) error {
	if err := validateReportQuotaWorkflowUpdateInput(to, retryStage, lastError, at); err != nil {
		return err
	}
	current, err := loadReportQuotaWorkflow(ctx, tx, taskID)
	if err != nil {
		return err
	}
	idempotent, err := validateReportQuotaWorkflowTransition(current, to, retryStage, lastError, at, from)
	if err != nil || idempotent {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_task_quota_workflow
		SET status=?,retry_stage=?,last_error_code=?,updated_at=? WHERE task_id=? AND status=?`,
		to, nullableRetryStage(retryStage), nullableString(lastError), stamp(at), taskID, current.status)
	if err != nil {
		return err
	}
	return exactlyOne(result, "update quota workflow for report")
}

type reportQuotaWorkflowState struct {
	status     QuotaWorkflowStatus
	retryStage sql.NullString
	lastError  sql.NullString
	updatedAt  time.Time
}

func validateReportQuotaWorkflowUpdateInput(
	to QuotaWorkflowStatus,
	retryStage *QuotaRetryStage,
	lastError *string,
	at time.Time,
) error {
	if !validQuotaWorkflowStatus(to) || at.IsZero() {
		return newError(ErrorContractInvalid, "invalid quota workflow update")
	}
	if to == QuotaWorkflowRetryPending {
		if retryStage == nil || !validQuotaRetryStage(*retryStage) {
			return newError(ErrorContractInvalid, "retry quota workflow requires retry stage")
		}
	} else if retryStage != nil {
		return newError(ErrorContractInvalid, "non-retry quota workflow cannot contain retry stage")
	}
	return validateOptionalReportCode(lastError)
}

func loadReportQuotaWorkflow(
	ctx context.Context,
	tx *sql.Tx,
	taskID string,
) (reportQuotaWorkflowState, error) {
	var current reportQuotaWorkflowState
	var updatedRaw string
	if err := tx.QueryRowContext(ctx, `SELECT status,retry_stage,last_error_code,updated_at
		FROM model_task_quota_workflow WHERE task_id=?`, taskID).Scan(
		&current.status, &current.retryStage, &current.lastError, &updatedRaw,
	); err != nil {
		return reportQuotaWorkflowState{}, taskStoreLoadError(err)
	}
	updatedAt, err := parseStamp(updatedRaw)
	if err != nil {
		return reportQuotaWorkflowState{}, newError(ErrorContractInvalid, "persisted quota workflow updated_at is invalid")
	}
	current.updatedAt = updatedAt
	return current, nil
}

func validateReportQuotaWorkflowTransition(
	current reportQuotaWorkflowState,
	to QuotaWorkflowStatus,
	retryStage *QuotaRetryStage,
	lastError *string,
	at time.Time,
	from []QuotaWorkflowStatus,
) (bool, error) {
	if at.Before(current.updatedAt) {
		return false, newError(ErrorInvalidStateTransition, "quota workflow update time cannot move backwards")
	}
	if current.status == to {
		if !optionalRetryStageMatches(current.retryStage, retryStage) ||
			!optionalStringMatches(current.lastError, lastError) {
			return false, newError(ErrorIdempotencyConflict, "quota workflow idempotent update conflicts with persisted details")
		}
		return true, nil
	}
	if current.status == QuotaWorkflowRetryPending {
		if err := validateReportRetryExit(current.retryStage, to); err != nil {
			return false, err
		}
	}
	if !quotaWorkflowSourceAllowed(current.status, from) {
		return false, newError(ErrorInvalidStateTransition, "quota workflow state does not allow report transition")
	}
	return false, nil
}

func validateReportRetryExit(retryStage sql.NullString, to QuotaWorkflowStatus) error {
	if !retryStage.Valid || QuotaRetryStage(retryStage.String) != QuotaRetryFinalUpload ||
		(to != QuotaWorkflowFinalUploading && to != QuotaWorkflowFailedManualRequired) {
		return newError(ErrorInvalidStateTransition, "quota retry stage does not allow report transition")
	}
	return nil
}

func quotaWorkflowSourceAllowed(current QuotaWorkflowStatus, allowed []QuotaWorkflowStatus) bool {
	for _, status := range allowed {
		if current == status {
			return true
		}
	}
	return false
}

func optionalRetryStageMatches(stored sql.NullString, expected *QuotaRetryStage) bool {
	if expected == nil {
		return !stored.Valid
	}
	return stored.Valid && stored.String == string(*expected)
}

func optionalStringMatches(stored sql.NullString, expected *string) bool {
	if expected == nil {
		return !stored.Valid
	}
	return stored.Valid && stored.String == *expected
}

func validateDeliveryMarkInput(store *SQLiteReportStore, key string, expectedAttempt int, state ReportState, code *string, at time.Time) error {
	if store == nil || store.db == nil || expectedAttempt <= 0 || at.IsZero() {
		return newError(ErrorContractInvalid, "report store, positive attempt, and update time are required")
	}
	if err := requireTrimmed("idempotency_key", key); err != nil {
		return err
	}
	if err := validateOptionalReportCode(code); err != nil {
		return err
	}
	if state != ReportAccepted && state != ReportRetry && state != ReportBlocked {
		return newError(ErrorContractInvalid, "invalid delivery result state")
	}
	return nil
}

func validateOptionalReportCode(code *string) error {
	if code == nil {
		return nil
	}
	return requireTrimmed("last_error_code", *code)
}

func (s *SQLiteReportStore) validateReportOwnership(ctx context.Context, scope TaskScope, key string) error {
	var taskID string
	if err := s.db.QueryRowContext(ctx, `SELECT task_id FROM usage_reports WHERE idempotency_key=?`, key).Scan(&taskID); err != nil {
		if err == sql.ErrNoRows {
			return newError(ErrorContractInvalid, "persisted report not found")
		}
		return err
	}
	_, err := loadReportTask(ctx, s.db, scope, taskID)
	return err
}

type acceptedRequestRecord struct {
	taskID     string
	requestIDs []string
}

// BackfillAcceptedQuotaRequestIDs migrates durable accepted quota-v1 facts into
// a cloud-user-wide request authority. It is read-only for the report store and
// never replays FinalizeQuota.
func (s *SQLiteReportStore) BackfillAcceptedQuotaRequestIDs(
	ctx context.Context,
	cloudUserID string,
	registry AcceptedQuotaRequestRegistry,
) error {
	if s == nil || s.db == nil || registry == nil {
		return newError(ErrorContractInvalid, "accepted quota request backfill dependencies are required")
	}
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return err
	}
	schemaVersion, err := acceptedQuotaBackfillSchemaVersion(ctx, s.db)
	if err != nil {
		return err
	}
	// quota-contract-v1 first became durable in V4. Older profile databases
	// cannot contain accepted quota identities and must not block another
	// profile's activation merely because they have not been migrated yet.
	if schemaVersion < SchemaVersionV4 {
		return nil
	}
	records, err := loadAcceptedQuotaRequestRecords(ctx, s.db, cloudUserID)
	if err != nil {
		return err
	}
	return registerAcceptedQuotaRequestRecords(ctx, registry, cloudUserID, records)
}

func acceptedQuotaBackfillSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM model_billing_schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("modelbilling: inspect accepted quota backfill schema: %w", err)
	}
	return version, nil
}

func loadAcceptedQuotaRequestRecords(ctx context.Context, db *sql.DB, cloudUserID string) ([]acceptedRequestRecord, error) {
	rows, err := db.QueryContext(ctx, `SELECT r.task_id,t.profile_id,t.cloud_user_id,t.binding_generation,t.freeze_id,t.task_result,
			r.idempotency_key,r.created_at,r.canonical_payload,r.payload_hash
		FROM usage_reports r
		JOIN model_tasks t ON t.task_id=r.task_id
		WHERE t.cloud_user_id=? AND t.billing_source=?
			AND r.contract_version=? AND r.report_kind=? AND r.state=?
		ORDER BY r.task_id,r.idempotency_key`, cloudUserID, BillingSubscriptionPool,
		QuotaContractV1, ReportFinal, ReportAccepted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []acceptedRequestRecord
	for rows.Next() {
		record, err := scanAcceptedQuotaRequestRecord(rows, cloudUserID)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func scanAcceptedQuotaRequestRecord(rows *sql.Rows, cloudUserID string) (acceptedRequestRecord, error) {
	var taskID, profileID, storedCloudUserID, freezeID, taskResult, idempotencyKey, createdAtRaw string
	var generation int64
	var payload []byte
	var payloadHash string
	if err := rows.Scan(
		&taskID, &profileID, &storedCloudUserID, &generation, &freezeID, &taskResult,
		&idempotencyKey, &createdAtRaw, &payload, &payloadHash,
	); err != nil {
		return acceptedRequestRecord{}, err
	}
	if generation <= 0 {
		return acceptedRequestRecord{}, newError(ErrorScopeMismatch, "accepted final quota task binding generation is invalid")
	}
	binding := cloud.BindingScope{
		ProfileID: profileID, CloudUserID: storedCloudUserID, BindingGeneration: uint64(generation),
	}
	accepted, err := decodeAcceptedFinalQuotaReport(payload, payloadHash)
	if err != nil {
		return acceptedRequestRecord{}, err
	}
	createdAt, err := parseStamp(createdAtRaw)
	if err != nil {
		return acceptedRequestRecord{}, newError(ErrorContractInvalid, "decode accepted final quota created_at: "+err.Error())
	}
	accepted.Scope = binding
	accepted.IdempotencyKey = idempotencyKey
	accepted.TaskResult = TaskResult(taskResult)
	accepted.CreatedAt = createdAt
	if err := accepted.Validate(); err != nil {
		return acceptedRequestRecord{}, err
	}
	if accepted.TaskID != taskID || accepted.FreezeID != freezeID || accepted.Scope != binding || storedCloudUserID != cloudUserID {
		return acceptedRequestRecord{}, newError(ErrorIdempotencyConflict, "accepted final quota identity does not match its durable task")
	}
	return acceptedRequestRecord{taskID: taskID, requestIDs: quotaFinalAcceptedRequestIDs(accepted.Calls)}, nil
}

func registerAcceptedQuotaRequestRecords(
	ctx context.Context,
	registry AcceptedQuotaRequestRegistry,
	cloudUserID string,
	records []acceptedRequestRecord,
) error {
	for _, record := range records {
		if err := registry.AcceptQuotaRequestIDs(ctx, cloudUserID, record.taskID, record.requestIDs); err != nil {
			return err
		}
	}
	return nil
}
