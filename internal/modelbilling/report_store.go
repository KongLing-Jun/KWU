package modelbilling

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"time"

	"kwu/internal/cloud"
)

const reportQueueTimeLayout = "2006-01-02T15:04:05.000000000Z"

// SQLiteReportStore persists immutable normalized reports and provides the
// atomic queue operations needed by a later delivery worker.
type SQLiteReportStore struct {
	db    *sql.DB
	clock func() time.Time
}

func NewSQLiteReportStore(db *sql.DB) *SQLiteReportStore {
	return NewSQLiteReportStoreWithClock(db, time.Now)
}

func NewSQLiteReportStoreWithClock(db *sql.DB, clock func() time.Time) *SQLiteReportStore {
	return &SQLiteReportStore{db: db, clock: clock}
}

// EnqueueFinal reconstructs and inserts the final from the same persisted
// snapshot. Callers cannot supply report identity, timestamps, or payload.
func (s *SQLiteReportStore) EnqueueFinal(ctx context.Context, scope TaskScope, taskID string) (UsageReportBuildResult, *PendingReport, error) {
	if err := validateReportStoreInput(s, taskID); err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	tx, task, calls, err := s.beginReportBuild(ctx, scope, taskID)
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	defer tx.Rollback()
	build, err := buildFinalQuotaReport(SubscriptionQuotaInput{Task: task, Calls: calls})
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	workflowAt, err := s.finalBuildWorkflowTime(ctx, tx, task, build.Disposition)
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	if build.Disposition != ReportBuildDeliverable {
		if err := finishNoReportFinalBuild(ctx, tx, task, build, workflowAt); err != nil {
			return UsageReportBuildResult{}, nil, err
		}
		if err := tx.Commit(); err != nil {
			return UsageReportBuildResult{}, nil, err
		}
		return build, nil, nil
	}
	stored, err := storeBuiltFinal(ctx, tx, scope, task, build, workflowAt)
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	return build, &stored, nil
}

func (s *SQLiteReportStore) finalBuildWorkflowTime(
	ctx context.Context,
	tx *sql.Tx,
	task ModelTask,
	disposition ReportBuildDisposition,
) (time.Time, error) {
	switch disposition {
	case ReportBuildEvidencePending, ReportBuildDeliverable:
		return s.finalWorkflowObservationTime(ctx, tx, task)
	default:
		return time.Time{}, nil
	}
}

func finishNoReportFinalBuild(
	ctx context.Context,
	tx *sql.Tx,
	task ModelTask,
	build UsageReportBuildResult,
	workflowAt time.Time,
) error {
	if err := validateNoReportFinalBuild(build); err != nil {
		return err
	}
	if build.Disposition == ReportBuildNoReportRequired {
		return nil
	}
	if build.Disposition != ReportBuildEvidencePending {
		return newError(ErrorContractInvalid, "unexpected non-deliverable final quota build")
	}
	return updateQuotaWorkflowInTx(ctx, tx, task.TaskID, QuotaWorkflowEvidencePending, nil, nil,
		workflowAt, QuotaWorkflowCollecting, QuotaWorkflowSupplementing)
}

// storeBuiltFinal only accepts the package-private builder output paired with
// the task loaded in the same transaction. Blocked facts intentionally skip
// deliverable usage validation so invalid aggregates remain auditable.
func storeBuiltFinal(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	task ModelTask,
	build UsageReportBuildResult,
	workflowAt time.Time,
) (PendingReport, error) {
	pending, err := prepareBuiltFinal(task, build)
	if err != nil {
		return PendingReport{}, err
	}
	if err := rejectAcceptedRequestIDReuse(ctx, tx, task, build.Final.Calls); err != nil {
		return PendingReport{}, err
	}
	stored, err := insertOrVerifyReport(ctx, tx, scope, pending, quotaReportCallIDs(build.Final.Calls))
	if err != nil {
		return PendingReport{}, err
	}
	if stored.State != ReportPending {
		return stored, nil
	}
	if err := updateQuotaWorkflowInTx(ctx, tx, task.TaskID, QuotaWorkflowFinalUploadPending, nil, nil,
		workflowAt, QuotaWorkflowCollecting, QuotaWorkflowEvidencePending, QuotaWorkflowSupplementing); err != nil {
		return PendingReport{}, err
	}
	return stored, nil
}

func prepareBuiltFinal(task ModelTask, build UsageReportBuildResult) (PendingReport, error) {
	if build.Final == nil || build.Partial != nil {
		return PendingReport{}, newError(ErrorContractInvalid, "final build result is invalid")
	}
	if build.Disposition != ReportBuildDeliverable {
		return PendingReport{}, newError(ErrorContractInvalid, "final quota build is not deliverable")
	}
	payload, err := CanonicalJSON(*build.Final)
	if err != nil {
		return PendingReport{}, err
	}
	if err := ValidateCanonicalPayload(build.CanonicalPayload, payload); err != nil {
		return PendingReport{}, err
	}
	if err := validateBuiltFinalQuotaIdentity(*build.Final, task); err != nil {
		return PendingReport{}, err
	}
	return newPendingReport(task, ReportFinal, 0, build.Final.IdempotencyKey,
		build.CanonicalPayload, build.Final.CreatedAt, ReportPending, nil)
}

// EnqueuePartial allocates the next sequence, reconstructs the persisted input,
// builds the cumulative report, and inserts it in one SQLite write transaction.
func (s *SQLiteReportStore) EnqueuePartial(ctx context.Context, scope TaskScope, taskID string) (UsageReportBuildResult, *PendingReport, error) {
	if err := validateReportStoreInput(s, taskID); err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	tx, task, calls, err := s.beginReportBuild(ctx, scope, taskID)
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	defer tx.Rollback()
	sequence, err := nextPartialSequence(ctx, tx, taskID)
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	capturedAt, err := s.partialCaptureTime()
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	build, err := buildPartialQuotaReport(SubscriptionQuotaInput{Task: task, Calls: calls}, sequence, capturedAt)
	if err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	var stored *PendingReport
	if build.Disposition != ReportBuildNoReportRequired {
		value, err := storeBuiltPartial(ctx, tx, scope, task, sequence, build)
		if err != nil {
			return UsageReportBuildResult{}, nil, err
		}
		stored = &value
	}
	return commitPartialBuild(tx, build, stored)
}

func commitPartialBuild(
	tx *sql.Tx,
	build UsageReportBuildResult,
	stored *PendingReport,
) (UsageReportBuildResult, *PendingReport, error) {
	if build.Disposition == ReportBuildNoReportRequired {
		if err := validateNoReportPartialBuild(build); err != nil {
			return UsageReportBuildResult{}, nil, err
		}
		if err := tx.Commit(); err != nil {
			return UsageReportBuildResult{}, nil, err
		}
		return build, nil, nil
	}
	if stored == nil {
		return UsageReportBuildResult{}, nil, newError(ErrorContractInvalid, "deliverable partial build was not stored")
	}
	if err := tx.Commit(); err != nil {
		return UsageReportBuildResult{}, nil, err
	}
	return build, stored, nil
}

func (s *SQLiteReportStore) beginReportBuild(ctx context.Context, scope TaskScope, taskID string) (*sql.Tx, ModelTask, []ModelCall, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, ModelTask{}, nil, err
	}
	if err := lockReportTask(ctx, tx, taskID); err != nil {
		tx.Rollback()
		return nil, ModelTask{}, nil, err
	}
	task, err := loadReportTask(ctx, tx, scope, taskID)
	if err != nil {
		tx.Rollback()
		return nil, ModelTask{}, nil, err
	}
	calls, err := loadSubscriptionQuotaCalls(ctx, tx, task)
	if err != nil {
		tx.Rollback()
		return nil, ModelTask{}, nil, err
	}
	return tx, task, calls, nil
}

func validateNoReportFinalBuild(build UsageReportBuildResult) error {
	if build.Partial != nil || build.Final != nil || len(build.CanonicalPayload) != 0 || build.BlockedCode != "" {
		return newError(ErrorContractInvalid, "no-report final build contains report data")
	}
	return nil
}

func validateNoReportPartialBuild(build UsageReportBuildResult) error {
	if build.Partial != nil || len(build.CanonicalPayload) != 0 || build.BlockedCode != "" {
		return newError(ErrorContractInvalid, "no-report partial build contains report data")
	}
	return nil
}

func storeBuiltPartial(ctx context.Context, tx *sql.Tx, scope TaskScope, task ModelTask, sequence uint64, build UsageReportBuildResult) (PendingReport, error) {
	if build.Partial == nil || build.Final != nil || build.Partial.Sequence != sequence {
		return PendingReport{}, newError(ErrorContractInvalid, "partial build result is invalid")
	}
	if build.Disposition != ReportBuildDeliverable {
		return PendingReport{}, newError(ErrorContractInvalid, "partial quota build is not deliverable")
	}
	payload, err := CanonicalJSON(*build.Partial)
	if err != nil {
		return PendingReport{}, err
	}
	if err := ValidateCanonicalPayload(build.CanonicalPayload, payload); err != nil {
		return PendingReport{}, err
	}
	if err := validateBuiltPartialQuotaIdentity(*build.Partial, task); err != nil {
		return PendingReport{}, err
	}
	if err := validateCumulativePartial(ctx, tx, task, *build.Partial); err != nil {
		return PendingReport{}, err
	}
	pending, err := newPendingReport(task, ReportPartial, sequence, build.Partial.IdempotencyKey,
		build.CanonicalPayload, build.Partial.CapturedAt, ReportPending, nil)
	if err != nil {
		return PendingReport{}, err
	}
	return insertOrVerifyReport(ctx, tx, scope, pending, quotaReportCallIDs(build.Partial.Calls))
}

func initialReportState(build UsageReportBuildResult) (ReportState, *string, error) {
	switch build.Disposition {
	case ReportBuildDeliverable:
		if build.BlockedCode != "" || build.BlockedReason != "" {
			return "", nil, newError(ErrorContractInvalid, "deliverable build contains blocked details")
		}
		return ReportPending, nil, nil
	case ReportBuildBlocked:
		if build.BlockedCode == "" || build.BlockedReason == "" {
			return "", nil, newError(ErrorContractInvalid, "blocked build requires a stable code and reason")
		}
		code := string(build.BlockedCode)
		return ReportBlocked, &code, nil
	default:
		return "", nil, newError(ErrorContractInvalid, "invalid report build disposition")
	}
}

func validateInitialReportState(state ReportState, code *string) error {
	switch state {
	case ReportPending:
		if code != nil {
			return newError(ErrorContractInvalid, "pending report cannot have a blocked code")
		}
	case ReportBlocked:
		if code == nil {
			return newError(ErrorContractInvalid, "blocked report requires an error code")
		}
		if err := requireTrimmed("last_error_code", *code); err != nil {
			return err
		}
	default:
		return newError(ErrorContractInvalid, "new report must be pending or blocked")
	}
	return nil
}

func validateReportStoreInput(store *SQLiteReportStore, taskID string) error {
	if store == nil || store.db == nil || store.clock == nil {
		return newError(ErrorContractInvalid, "report store database is required")
	}
	return requireTrimmed("task_id", taskID)
}

func (s *SQLiteReportStore) partialCaptureTime() (time.Time, error) {
	if s == nil || s.clock == nil {
		return time.Time{}, newError(ErrorContractInvalid, "report store clock is required")
	}
	capturedAt := s.clock()
	if capturedAt.IsZero() {
		return time.Time{}, newError(ErrorQuotaTimeInvalid, "partial capture clock returned zero")
	}
	return capturedAt.UTC(), nil
}

func (s *SQLiteReportStore) finalWorkflowObservationTime(
	ctx context.Context,
	tx *sql.Tx,
	task ModelTask,
) (time.Time, error) {
	if s == nil || s.clock == nil {
		return time.Time{}, newError(ErrorContractInvalid, "report store clock is required")
	}
	observedAt := s.clock()
	if observedAt.IsZero() {
		return time.Time{}, newError(ErrorQuotaTimeInvalid, "final workflow clock returned zero")
	}
	observedAt = observedAt.UTC()
	if observedAt.Before(task.UpdatedAt) {
		observedAt = task.UpdatedAt.UTC()
	}
	var workflowUpdatedRaw string
	if err := tx.QueryRowContext(ctx, `SELECT updated_at FROM model_task_quota_workflow WHERE task_id=?`, task.TaskID).
		Scan(&workflowUpdatedRaw); err != nil {
		return time.Time{}, taskStoreLoadError(err)
	}
	workflowUpdatedAt, err := parseStamp(workflowUpdatedRaw)
	if err != nil {
		return time.Time{}, persistedFieldError("quota workflow updated_at", err)
	}
	if observedAt.Before(workflowUpdatedAt) {
		observedAt = workflowUpdatedAt
	}
	return observedAt, nil
}

func lockReportTask(ctx context.Context, tx *sql.Tx, taskID string) error {
	result, err := tx.ExecContext(ctx, `UPDATE model_tasks SET updated_at=updated_at WHERE task_id=?`, taskID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return newError(ErrorContractInvalid, "persisted task not found")
	}
	return nil
}

func loadReportTask(ctx context.Context, q taskQuerier, scope TaskScope, taskID string) (ModelTask, error) {
	task, err := loadTask(ctx, q, taskID)
	if err != nil {
		return ModelTask{}, taskStoreLoadError(err)
	}
	if !task.Scope.Equal(scope) {
		return ModelTask{}, newError(ErrorScopeMismatch, "report task scope does not match persisted task")
	}
	if task.BillingSource != BillingSubscriptionPool {
		return ModelTask{}, newError(ErrorBillingSourceMismatch, "usage reports require a subscription task")
	}
	if task.Scope.CloudBinding == nil || task.Freeze == nil {
		return ModelTask{}, newError(ErrorContractInvalid, "usage report task has no persisted binding or freeze")
	}
	return task, nil
}

func nextPartialSequence(ctx context.Context, tx *sql.Tx, taskID string) (uint64, error) {
	var maximum int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),0) FROM usage_reports WHERE task_id=? AND report_kind=?`, taskID, ReportPartial).Scan(&maximum); err != nil {
		return 0, err
	}
	if maximum < 0 || maximum == math.MaxInt64 {
		return 0, newError(ErrorContractInvalid, "partial sequence is exhausted")
	}
	return uint64(maximum + 1), nil
}

func validateBuiltPartialQuotaIdentity(report PartialQuotaReport, task ModelTask) error {
	if err := validateBuiltReportTaskIdentity(report.TaskID, report.Scope, report.FreezeID, task, "partial"); err != nil {
		return err
	}
	if report.ContractVersion != QuotaContractV1 || report.Sequence == 0 || report.ReportMode != ReportModeCumulative ||
		report.CapturedAt.IsZero() || report.CapturedAt.Location() != time.UTC {
		return newError(ErrorContractInvalid, "partial quota report identity is invalid")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	expected, err := BuildPartialIdempotencyKey(report.Scope, report.TaskID, report.FreezeID, report.Sequence)
	if err != nil {
		return err
	}
	if report.IdempotencyKey != expected {
		return newError(ErrorIdempotencyConflict, "partial report key does not match persisted identity")
	}
	return nil
}

func validateBuiltFinalQuotaIdentity(report FinalQuotaReport, task ModelTask) error {
	if err := validateFinalTaskLifecycle(task); err != nil {
		return err
	}
	if err := validateBuiltReportTaskIdentity(report.TaskID, report.Scope, report.FreezeID, task, "final"); err != nil {
		return err
	}
	if err := report.ValidateForTask(task); err != nil {
		return err
	}
	if !report.CreatedAt.Equal(*task.EndedAt) || !report.ClientSummary.StartedAt.Equal(*task.StartedAt) ||
		!report.ClientSummary.FinishedAt.Equal(*task.EndedAt) {
		return newError(ErrorContractInvalid, "final quota report does not match persisted lifecycle facts")
	}
	if report.ContractVersion != QuotaContractV1 {
		return newError(ErrorLegacyReportContract, "final report is not quota contract v1")
	}
	expected, err := BuildFinalIdempotencyKey(report.Scope, report.TaskID, report.FreezeID)
	if err != nil {
		return err
	}
	if report.IdempotencyKey != expected {
		return newError(ErrorIdempotencyConflict, "final report key does not match persisted identity")
	}
	return nil
}

func validateBuiltReportTaskIdentity(taskID string, scope cloud.BindingScope, freezeID string, task ModelTask, kind string) error {
	if task.Scope.CloudBinding == nil || taskID != task.TaskID || scope != *task.Scope.CloudBinding {
		return newError(ErrorScopeMismatch, kind+" report does not match persisted task")
	}
	if task.Freeze == nil || freezeID != task.Freeze.FreezeID {
		return newError(ErrorScopeMismatch, kind+" report does not match persisted freeze")
	}
	return nil
}

func validateFinalTaskLifecycle(task ModelTask) error {
	if task.State != TaskUsagePending || task.Result == nil || task.EndedAt == nil {
		return newError(ErrorContractInvalid, "final report requires an ended usage_pending task")
	}
	if task.StartedAt == nil {
		return newError(ErrorQuotaTimeInvalid, "final report requires persisted task started_at")
	}
	return nil
}

func newPendingReport(task ModelTask, kind ReportKind, sequence uint64, key string, payload json.RawMessage, created time.Time, state ReportState, code *string) (PendingReport, error) {
	if err := validateInitialReportState(state, code); err != nil {
		return PendingReport{}, err
	}
	var next *time.Time
	if state == ReportPending {
		value := created.UTC()
		next = &value
	}
	report := PendingReport{
		Scope: *task.Scope.CloudBinding, TaskID: task.TaskID, FreezeID: task.Freeze.FreezeID,
		ContractVersion: QuotaContractV1, PayloadHash: quotaPayloadHash(payload),
		Kind: kind, Sequence: sequence, IdempotencyKey: key, CanonicalPayload: append(json.RawMessage(nil), payload...),
		State: state, AttemptCount: 0, NextAttemptAt: next, LastErrorCode: cloneString(code),
		CreatedAt: created.UTC(), UpdatedAt: created.UTC(),
	}
	if err := report.Validate(); err != nil {
		return PendingReport{}, err
	}
	return report, nil
}

func insertOrVerifyReport(ctx context.Context, tx *sql.Tx, scope TaskScope, report PendingReport, callIDs []string) (PendingReport, error) {
	existing, found, err := getPendingByKey(ctx, tx, scope, report.IdempotencyKey)
	if err != nil {
		return PendingReport{}, err
	}
	if found {
		return verifyExistingReport(ctx, tx, existing, report, callIDs)
	}
	if err := insertPendingReportRow(ctx, tx, report); err != nil {
		return PendingReport{}, err
	}
	if err := insertReportCallLinks(ctx, tx, report, callIDs); err != nil {
		return PendingReport{}, err
	}
	return report, nil
}

func verifyExistingReport(
	ctx context.Context,
	tx *sql.Tx,
	existing PendingReport,
	candidate PendingReport,
	callIDs []string,
) (PendingReport, error) {
	verified, err := verifyEnqueueRetry(existing, candidate)
	if err != nil {
		return PendingReport{}, err
	}
	if err := verifyReportCallLinks(ctx, tx, candidate.TaskID, candidate.IdempotencyKey, callIDs); err != nil {
		return PendingReport{}, err
	}
	return verified, nil
}

func insertPendingReportRow(ctx context.Context, tx *sql.Tx, report PendingReport) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO usage_reports
		(idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,next_attempt_at,last_error_code,created_at,updated_at,contract_version,payload_hash)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, report.IdempotencyKey, report.TaskID, report.Kind, report.Sequence,
		[]byte(report.CanonicalPayload), report.State, report.AttemptCount, nullableReportQueueTime(report.NextAttemptAt),
		nullableString(report.LastErrorCode), reportQueueStamp(report.CreatedAt), reportQueueStamp(report.UpdatedAt),
		report.ContractVersion, report.PayloadHash)
	if err != nil {
		if isConstraint(err) {
			return reportIdentityConflict(ctx, tx, report, err)
		}
		return err
	}
	return nil
}

func insertReportCallLinks(ctx context.Context, tx *sql.Tx, report PendingReport, callIDs []string) error {
	for _, callID := range callIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_report_calls (idempotency_key,task_id,call_id) VALUES (?,?,?)`,
			report.IdempotencyKey, report.TaskID, callID); err != nil {
			return err
		}
	}
	if report.Kind == ReportFinal {
		if err := markFinalCallsPending(ctx, tx, report.TaskID, report.IdempotencyKey); err != nil {
			return err
		}
	}
	return nil
}

func verifyReportCallLinks(
	ctx context.Context,
	tx *sql.Tx,
	taskID string,
	key string,
	want []string,
) error {
	rows, err := tx.QueryContext(ctx, `SELECT call_id FROM usage_report_calls
		WHERE task_id=? AND idempotency_key=? ORDER BY call_id`, taskID, key)
	if err != nil {
		return err
	}
	got, err := collectIDs(rows)
	if err != nil {
		return err
	}
	expected := append([]string(nil), want...)
	sort.Strings(expected)
	if len(got) != len(expected) {
		return newError(ErrorIdempotencyConflict, "report call links do not match canonical payload")
	}
	for i := range got {
		if got[i] != expected[i] {
			return newError(ErrorIdempotencyConflict, "report call links do not match canonical payload")
		}
	}
	return nil
}

func verifyEnqueueRetry(existing, candidate PendingReport) (PendingReport, error) {
	if existing.TaskID != candidate.TaskID || existing.Kind != candidate.Kind || existing.Sequence != candidate.Sequence ||
		existing.ContractVersion != candidate.ContractVersion || existing.PayloadHash != candidate.PayloadHash {
		return PendingReport{}, newError(ErrorIdempotencyConflict, "report identity conflicts with persisted report")
	}
	if err := ValidateCanonicalPayload(existing.CanonicalPayload, candidate.CanonicalPayload); err != nil {
		return PendingReport{}, err
	}
	return existing, nil
}

func reportIdentityConflict(ctx context.Context, tx *sql.Tx, report PendingReport, cause error) error {
	var key string
	err := tx.QueryRowContext(ctx, `SELECT idempotency_key FROM usage_reports WHERE task_id=? AND report_kind=? AND sequence=?`,
		report.TaskID, report.Kind, report.Sequence).Scan(&key)
	if err == nil {
		return newError(ErrorIdempotencyConflict, "report identity already has a different idempotency key")
	}
	if err != sql.ErrNoRows {
		return err
	}
	return cause
}

func quotaReportCallIDs(calls []QuotaReportCall) []string {
	ids := make([]string, len(calls))
	for i := range calls {
		ids[i] = calls[i].CallID
	}
	return ids
}

func markFinalCallsPending(ctx context.Context, tx *sql.Tx, taskID, key string) error {
	var invalid int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_calls c
		JOIN usage_report_calls rc ON rc.task_id=c.task_id AND rc.call_id=c.call_id
		WHERE rc.idempotency_key=? AND c.task_id=?
			AND c.quota_report_status NOT IN (?,?,?)`, key, taskID,
		QuotaReportUnreported, QuotaReportPartialReported, QuotaReportFinalPending).Scan(&invalid); err != nil {
		return err
	}
	if invalid != 0 {
		return newError(ErrorInvalidStateTransition, "final report calls are already final reported")
	}
	_, err := tx.ExecContext(ctx, `UPDATE model_calls SET quota_report_status=?,quota_report_last_error_code=NULL
		WHERE task_id=? AND EXISTS (
			SELECT 1 FROM usage_report_calls rc
			WHERE rc.idempotency_key=? AND rc.task_id=model_calls.task_id AND rc.call_id=model_calls.call_id
		)`, QuotaReportFinalPending, taskID, key)
	return err
}

func validateCumulativePartial(ctx context.Context, tx *sql.Tx, task ModelTask, candidate PartialQuotaReport) error {
	previous, err := loadPreviousPartialReport(ctx, tx, task, candidate.Sequence)
	if err != nil || previous == nil {
		return err
	}
	if err := validatePartialSnapshotAdvance(*previous, candidate); err != nil {
		return err
	}
	current := indexQuotaReportCalls(candidate.Calls)
	for _, oldCall := range previous.Calls {
		newCall, ok := current[oldCall.CallID]
		if err := validateCumulativePartialCall(oldCall, newCall, ok); err != nil {
			return err
		}
	}
	return nil
}

func loadPreviousPartialReport(
	ctx context.Context,
	tx *sql.Tx,
	task ModelTask,
	sequence uint64,
) (*PartialQuotaReport, error) {
	var key string
	err := tx.QueryRowContext(ctx, `SELECT idempotency_key FROM usage_reports
		WHERE task_id=? AND report_kind=? AND contract_version=? AND sequence<?
		ORDER BY sequence DESC LIMIT 1`, task.TaskID, ReportPartial, QuotaContractV1, sequence).Scan(&key)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	previousPending, found, err := getPendingByKey(ctx, tx, task.Scope, key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, newError(ErrorContractInvalid, "previous partial quota report disappeared")
	}
	var previous PartialQuotaReport
	if err := json.Unmarshal(previousPending.CanonicalPayload, &previous); err != nil {
		return nil, newError(ErrorContractInvalid, "decode previous partial quota report")
	}
	return &previous, nil
}

func validatePartialSnapshotAdvance(previous, candidate PartialQuotaReport) error {
	if candidate.Sequence <= previous.Sequence || candidate.CapturedAt.Before(previous.CapturedAt) {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota snapshot sequence or captured_at regressed")
	}
	return nil
}

func indexQuotaReportCalls(calls []QuotaReportCall) map[string]QuotaReportCall {
	current := make(map[string]QuotaReportCall, len(calls))
	for _, call := range calls {
		current[call.CallID] = call
	}
	return current
}

func validateCumulativePartialCall(oldCall, newCall QuotaReportCall, found bool) error {
	if err := validateCumulativeCallIdentity(oldCall, newCall, found); err != nil {
		return err
	}
	return validateCumulativeCallEvidence(oldCall, newCall)
}

func validateCumulativeCallIdentity(oldCall, newCall QuotaReportCall, found bool) error {
	if !found {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota snapshot removed a prior call")
	}
	if newCall.Endpoint != oldCall.Endpoint || newCall.Model != oldCall.Model ||
		!newCall.OccurredAt.Equal(oldCall.OccurredAt) {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota snapshot rewrote immutable call identity")
	}
	return nil
}

func validateCumulativeCallEvidence(oldCall, newCall QuotaReportCall) error {
	if newCall.EvidenceRevision < oldCall.EvidenceRevision {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota evidence revision regressed")
	}
	sameFingerprint := quotaReportCallsEqual([]QuotaReportCall{oldCall}, []QuotaReportCall{newCall})
	if newCall.EvidenceRevision == oldCall.EvidenceRevision && !sameFingerprint {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota evidence changed without a new revision")
	}
	if oldCall.EvidenceStatus == QuotaEvidenceComplete && newCall.EvidenceStatus != QuotaEvidenceComplete {
		return newError(ErrorQuotaPartialSnapshotConflict, "complete partial quota evidence regressed")
	}
	return nil
}

func rejectAcceptedRequestIDReuse(ctx context.Context, tx *sql.Tx, task ModelTask, calls []QuotaReportCall) error {
	wanted := quotaReportRequestIDs(calls)
	if len(wanted) == 0 {
		return nil
	}
	rows, err := queryAcceptedFinalQuotaReports(ctx, tx, task)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		accepted, err := scanAcceptedFinalQuotaReport(rows)
		if err != nil {
			return err
		}
		if err := rejectReusedRequestIDs(wanted, accepted.Calls); err != nil {
			return err
		}
	}
	return rows.Err()
}

func quotaReportRequestIDs(calls []QuotaReportCall) map[string]struct{} {
	wanted := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		if call.OneAPIRequestID != nil {
			wanted[*call.OneAPIRequestID] = struct{}{}
		}
	}
	return wanted
}

func queryAcceptedFinalQuotaReports(ctx context.Context, tx *sql.Tx, task ModelTask) (*sql.Rows, error) {
	if task.Scope.CloudBinding == nil {
		return nil, newError(ErrorScopeMismatch, "quota request id reuse check requires cloud binding")
	}
	return tx.QueryContext(ctx, `SELECT r.canonical_payload,r.payload_hash FROM usage_reports r
		JOIN model_tasks t ON t.task_id=r.task_id
		WHERE t.cloud_user_id=? AND t.billing_source=? AND r.task_id<>?
			AND r.contract_version=? AND r.report_kind=? AND r.state=?
		ORDER BY r.idempotency_key`, task.Scope.CloudBinding.CloudUserID, BillingSubscriptionPool, task.TaskID,
		QuotaContractV1, ReportFinal, ReportAccepted)
}

func scanAcceptedFinalQuotaReport(row rowScanner) (FinalQuotaReport, error) {
	var payload []byte
	var payloadHash string
	if err := row.Scan(&payload, &payloadHash); err != nil {
		return FinalQuotaReport{}, err
	}
	return decodeAcceptedFinalQuotaReport(payload, payloadHash)
}

func decodeAcceptedFinalQuotaReport(payload []byte, payloadHash string) (FinalQuotaReport, error) {
	if payloadHash != quotaPayloadHash(payload) {
		return FinalQuotaReport{}, newError(ErrorIdempotencyConflict, "accepted final quota payload hash is invalid")
	}
	var accepted FinalQuotaReport
	if err := json.Unmarshal(payload, &accepted); err != nil {
		return FinalQuotaReport{}, newError(ErrorContractInvalid, "decode accepted final quota report")
	}
	canonical, err := CanonicalJSON(accepted)
	if err != nil || !bytes.Equal(canonical, payload) || accepted.ContractVersion != QuotaContractV1 {
		return FinalQuotaReport{}, newError(ErrorIdempotencyConflict, "accepted final quota payload is not canonical")
	}
	return accepted, nil
}

func rejectReusedRequestIDs(wanted map[string]struct{}, accepted []QuotaReportCall) error {
	for _, call := range accepted {
		if call.OneAPIRequestID == nil {
			continue
		}
		if _, reused := wanted[*call.OneAPIRequestID]; reused {
			return newError(ErrorQuotaRequestIDReused, "oneapi request id was accepted for another task in the same scope")
		}
	}
	return nil
}
