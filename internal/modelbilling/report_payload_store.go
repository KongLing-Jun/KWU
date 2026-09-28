package modelbilling

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"
)

func (s *SQLiteReportStore) CorrectUnacceptedFinal(
	ctx context.Context,
	scope TaskScope,
	key string,
	expectedAttempt int,
	reasonCode ErrorCode,
	at time.Time,
) (PendingReport, error) {
	return NewSQLiteFinalQuotaReportCorrector(s.db).CorrectUnacceptedFinal(
		ctx, scope, key, expectedAttempt, reasonCode, at,
	)
}

type SQLiteFinalQuotaReportCorrector struct{ db *sql.DB }

func NewSQLiteFinalQuotaReportCorrector(db *sql.DB) *SQLiteFinalQuotaReportCorrector {
	return &SQLiteFinalQuotaReportCorrector{db: db}
}

func (c *SQLiteFinalQuotaReportCorrector) CorrectUnacceptedFinal(
	ctx context.Context,
	scope TaskScope,
	key string,
	expectedAttempt int,
	reasonCode ErrorCode,
	at time.Time,
) (PendingReport, error) {
	if err := validateFinalCorrectionInput(c, key, expectedAttempt, reasonCode, at); err != nil {
		return PendingReport{}, err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return PendingReport{}, err
	}
	defer tx.Rollback()
	snapshot, err := loadFinalCorrectionSnapshot(ctx, tx, scope, key, expectedAttempt)
	if err != nil {
		return PendingReport{}, err
	}
	newPayload, newHash, err := rebuildFinalCorrection(ctx, tx, snapshot, key)
	if err != nil {
		return PendingReport{}, err
	}
	return persistFinalCorrection(ctx, tx, scope, snapshot, key, expectedAttempt, at, newPayload, newHash)
}

type finalCorrectionSnapshot struct {
	record    pendingRow
	task      ModelTask
	oldReport FinalQuotaReport
}

func validateFinalCorrectionInput(
	corrector *SQLiteFinalQuotaReportCorrector,
	key string,
	expectedAttempt int,
	reasonCode ErrorCode,
	at time.Time,
) error {
	if corrector == nil || corrector.db == nil {
		return newError(ErrorContractInvalid, "final quota corrector is required")
	}
	if expectedAttempt <= 0 || at.IsZero() {
		return newError(ErrorContractInvalid, "final quota correction attempt and update time are required")
	}
	if err := requireTrimmed("idempotency_key", key); err != nil {
		return err
	}
	switch reasonCode {
	case ErrorQuotaSummaryMismatch, ErrorQuotaTimeInvalid:
		return nil
	default:
		return newError(ErrorContractInvalid, "unsupported final quota correction reason")
	}
}

func loadFinalCorrectionSnapshot(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	key string,
	expectedAttempt int,
) (finalCorrectionSnapshot, error) {
	record, err := loadPendingRecord(ctx, tx, key)
	if err != nil {
		return finalCorrectionSnapshot{}, err
	}
	if err := validateFinalCorrectionRecord(record, expectedAttempt); err != nil {
		return finalCorrectionSnapshot{}, err
	}
	task, err := loadReportTask(ctx, tx, scope, record.taskID)
	if err != nil {
		return finalCorrectionSnapshot{}, err
	}
	created, _, _, _, err := decodePendingRowMetadata(record)
	if err != nil {
		return finalCorrectionSnapshot{}, err
	}
	oldReport, err := decodeStoredFinalQuotaReport(task, record, created)
	if err != nil {
		return finalCorrectionSnapshot{}, err
	}
	oldCanonical, err := CanonicalJSON(oldReport)
	if err != nil || !bytes.Equal(oldCanonical, record.payload) {
		return finalCorrectionSnapshot{}, newError(ErrorQuotaFinalPayloadConflict, "stored final quota payload is not canonical")
	}
	return finalCorrectionSnapshot{record: record, task: task, oldReport: oldReport}, nil
}

func validateFinalCorrectionRecord(record pendingRow, expectedAttempt int) error {
	if record.contractVersion != QuotaContractV1 || ReportKind(record.kind) != ReportFinal {
		return newError(ErrorLegacyReportContract, "only quota final reports can be corrected")
	}
	if ReportState(record.state) != ReportDelivering || int(record.attempts) != expectedAttempt {
		return newError(ErrorInvalidStateTransition, "final quota correction requires the current delivering attempt")
	}
	if !record.payloadHash.Valid || record.payloadHash.String != quotaPayloadHash(record.payload) {
		return newError(ErrorIdempotencyConflict, "stored final quota hash is invalid")
	}
	return nil
}

func rebuildFinalCorrection(
	ctx context.Context,
	tx *sql.Tx,
	snapshot finalCorrectionSnapshot,
	key string,
) (json.RawMessage, string, error) {
	calls, err := loadSubscriptionQuotaCalls(ctx, tx, snapshot.task)
	if err != nil {
		return nil, "", err
	}
	build, err := buildFinalQuotaReport(SubscriptionQuotaInput{Task: snapshot.task, Calls: calls})
	if err != nil {
		return nil, "", err
	}
	if err := validateFinalCorrectionBuild(snapshot, key, build); err != nil {
		return nil, "", err
	}
	newPayload := append(json.RawMessage(nil), build.CanonicalPayload...)
	if bytes.Equal(newPayload, snapshot.record.payload) {
		return nil, "", newError(ErrorQuotaFinalPayloadConflict, "final quota payload does not require correction")
	}
	return newPayload, quotaPayloadHash(newPayload), nil
}

func validateFinalCorrectionBuild(snapshot finalCorrectionSnapshot, key string, build UsageReportBuildResult) error {
	if build.Disposition != ReportBuildDeliverable || build.Final == nil || build.Partial != nil {
		return newError(ErrorQuotaFinalPayloadConflict, "persisted facts cannot rebuild a deliverable final quota report")
	}
	if err := validateFinalCorrectionBusinessFacts(snapshot, key, *build.Final); err != nil {
		return err
	}
	return validateFinalCorrectionTimeFacts(snapshot)
}

func validateFinalCorrectionBusinessFacts(
	snapshot finalCorrectionSnapshot,
	key string,
	replacement FinalQuotaReport,
) error {
	if replacement.IdempotencyKey != key || replacement.TaskID != snapshot.oldReport.TaskID ||
		replacement.FreezeID != snapshot.oldReport.FreezeID ||
		!quotaCorrectionCallsEqual(snapshot.oldReport.Calls, replacement.Calls) {
		return newError(ErrorQuotaFinalPayloadConflict, "final quota correction would change business facts")
	}
	return nil
}

func validateFinalCorrectionTimeFacts(snapshot finalCorrectionSnapshot) error {
	if snapshot.task.StartedAt == nil || snapshot.task.EndedAt == nil ||
		!snapshot.oldReport.ClientSummary.StartedAt.Equal(*snapshot.task.StartedAt) ||
		!snapshot.oldReport.ClientSummary.FinishedAt.Equal(*snapshot.task.EndedAt) {
		return newError(ErrorQuotaFinalPayloadConflict, "final quota correction would change task time facts")
	}
	return nil
}

func persistFinalCorrection(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	snapshot finalCorrectionSnapshot,
	key string,
	expectedAttempt int,
	at time.Time,
	newPayload json.RawMessage,
	newHash string,
) (PendingReport, error) {
	result, err := tx.ExecContext(ctx, `UPDATE usage_reports SET canonical_payload=?,payload_hash=?,state=?,next_attempt_at=?,last_error_code=NULL,updated_at=?
		WHERE idempotency_key=? AND contract_version=? AND report_kind=? AND state=? AND attempt_count=? AND payload_hash=?`,
		[]byte(newPayload), newHash, ReportPending, reportQueueStamp(at), reportQueueStamp(at), key, QuotaContractV1, ReportFinal,
		ReportDelivering, expectedAttempt, snapshot.record.payloadHash.String)
	if err != nil {
		return PendingReport{}, err
	}
	if err := exactlyOne(result, "correct final quota report"); err != nil {
		return PendingReport{}, newError(ErrorInvalidStateTransition, "final quota correction attempt became stale")
	}
	if err := updateQuotaWorkflowInTx(ctx, tx, snapshot.task.TaskID, QuotaWorkflowFinalUploadPending, nil, nil,
		at, QuotaWorkflowFinalUploading); err != nil {
		return PendingReport{}, err
	}
	corrected, found, err := getPendingByKey(ctx, tx, scope, key)
	if err != nil {
		return PendingReport{}, err
	}
	if !found {
		return PendingReport{}, newError(ErrorContractInvalid, "corrected final quota report disappeared")
	}
	if err := tx.Commit(); err != nil {
		return PendingReport{}, err
	}
	return corrected, nil
}

func quotaCorrectionCallsEqual(left, right []QuotaReportCall) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := make([]QuotaReportCall, len(left))
	rightCopy := make([]QuotaReportCall, len(right))
	for i := range left {
		leftCopy[i] = cloneQuotaReportCall(left[i])
		leftCopy[i].OccurredAt = leftCopy[i].OccurredAt.UTC()
	}
	for i := range right {
		rightCopy[i] = cloneQuotaReportCall(right[i])
		rightCopy[i].OccurredAt = rightCopy[i].OccurredAt.UTC()
	}
	return quotaReportCallsEqual(leftCopy, rightCopy)
}

type pendingRow struct {
	key, taskID, kind, state, created, updated, contractVersion string
	sequence, attempts                                          int64
	payload                                                     []byte
	next, lastError, payloadHash                                sql.NullString
}

type rowScanner interface{ Scan(...any) error }

func scanPendingRow(row rowScanner) (pendingRow, error) {
	var record pendingRow
	err := row.Scan(&record.key, &record.taskID, &record.kind, &record.sequence, &record.payload,
		&record.state, &record.attempts, &record.next, &record.lastError, &record.created, &record.updated,
		&record.contractVersion, &record.payloadHash)
	return record, err
}

func loadPendingRecord(ctx context.Context, q taskQuerier, key string) (pendingRow, error) {
	record, err := scanPendingRow(q.QueryRowContext(ctx, `SELECT idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,
		next_attempt_at,last_error_code,created_at,updated_at,contract_version,payload_hash FROM usage_reports WHERE idempotency_key=?`, key))
	if err == sql.ErrNoRows {
		return pendingRow{}, newError(ErrorContractInvalid, "persisted report not found")
	}
	return record, err
}

func getPendingByKey(ctx context.Context, q taskQuerier, scope TaskScope, key string) (PendingReport, bool, error) {
	record, err := scanPendingRow(q.QueryRowContext(ctx, `SELECT idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,
		next_attempt_at,last_error_code,created_at,updated_at,contract_version,payload_hash FROM usage_reports WHERE idempotency_key=?`, key))
	if err == sql.ErrNoRows {
		return PendingReport{}, false, nil
	}
	if err != nil {
		return PendingReport{}, false, err
	}
	report, err := hydratePendingRow(ctx, q, scope, record)
	return report, err == nil, err
}

func hydratePendingRow(ctx context.Context, q taskQuerier, scope TaskScope, record pendingRow) (PendingReport, error) {
	if err := validatePendingRowBase(record); err != nil {
		return PendingReport{}, err
	}
	task, err := loadReportTask(ctx, q, scope, record.taskID)
	if err != nil {
		return PendingReport{}, err
	}
	created, updated, next, lastError, err := decodePendingRowMetadata(record)
	if err != nil {
		return PendingReport{}, err
	}
	if err := validateReportKindSequence(ReportKind(record.kind), uint64(record.sequence)); err != nil {
		return PendingReport{}, err
	}
	if record.contractVersion == QuotaContractV1 {
		if err := validateStoredQuotaPayload(task, record, created); err != nil {
			return PendingReport{}, err
		}
	}
	report := PendingReport{
		Scope: *task.Scope.CloudBinding, TaskID: task.TaskID, FreezeID: task.Freeze.FreezeID,
		ContractVersion: record.contractVersion, PayloadHash: pendingRowPayloadHash(record),
		Kind: ReportKind(record.kind), Sequence: uint64(record.sequence), IdempotencyKey: record.key,
		CanonicalPayload: append(json.RawMessage(nil), record.payload...), State: ReportState(record.state),
		AttemptCount: int(record.attempts), NextAttemptAt: next, LastErrorCode: lastError,
		CreatedAt: created, UpdatedAt: updated,
	}
	if err := report.Validate(); err != nil {
		return PendingReport{}, err
	}
	return report, nil
}

func validatePendingRowBase(record pendingRow) error {
	if record.sequence < 0 || record.attempts < 0 {
		return newError(ErrorContractInvalid, "persisted report counters are invalid")
	}
	if record.contractVersion != QuotaContractV1 && record.contractVersion != LegacyTokenContractV1 {
		return newError(ErrorContractInvalid, "persisted report contract version is invalid")
	}
	return nil
}

func pendingRowPayloadHash(record pendingRow) string {
	if record.payloadHash.Valid {
		return record.payloadHash.String
	}
	return ""
}

func validateStoredQuotaPayload(task ModelTask, record pendingRow, created time.Time) error {
	if !record.payloadHash.Valid || record.payloadHash.String != quotaPayloadHash(record.payload) {
		return newError(ErrorIdempotencyConflict, "persisted quota report payload hash does not match")
	}
	canonical, err := canonicalStoredQuotaPayload(task, record, created)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, record.payload) {
		return newError(ErrorIdempotencyConflict, "persisted quota report payload is not canonical")
	}
	return nil
}

func canonicalStoredQuotaPayload(task ModelTask, record pendingRow, created time.Time) ([]byte, error) {
	switch ReportKind(record.kind) {
	case ReportPartial:
		return canonicalStoredPartialQuotaReport(task, record, created)
	case ReportFinal:
		return canonicalStoredFinalQuotaReport(task, record, created)
	default:
		return nil, newError(ErrorContractInvalid, "persisted quota report kind is invalid")
	}
}

func canonicalStoredPartialQuotaReport(task ModelTask, record pendingRow, created time.Time) ([]byte, error) {
	var report PartialQuotaReport
	if err := json.Unmarshal(record.payload, &report); err != nil {
		return nil, newError(ErrorContractInvalid, "decode persisted partial quota report")
	}
	report.Scope = *task.Scope.CloudBinding
	report.IdempotencyKey = record.key
	if report.TaskID != task.TaskID || report.FreezeID != task.Freeze.FreezeID ||
		report.Sequence != uint64(record.sequence) || !report.CapturedAt.Equal(created) {
		return nil, newError(ErrorIdempotencyConflict, "persisted partial quota identity does not match its row")
	}
	if err := report.Validate(); err != nil {
		return nil, err
	}
	return CanonicalJSON(report)
}

func canonicalStoredFinalQuotaReport(task ModelTask, record pendingRow, created time.Time) ([]byte, error) {
	report, err := decodeStoredFinalQuotaReport(task, record, created)
	if err != nil {
		return nil, err
	}
	if err := report.Validate(); err != nil {
		return nil, err
	}
	return CanonicalJSON(report)
}

func decodeStoredFinalQuotaReport(task ModelTask, record pendingRow, created time.Time) (FinalQuotaReport, error) {
	var report FinalQuotaReport
	if err := json.Unmarshal(record.payload, &report); err != nil {
		return FinalQuotaReport{}, newError(ErrorContractInvalid, "decode persisted final quota report")
	}
	if err := validateStoredFinalTaskFacts(task); err != nil {
		return FinalQuotaReport{}, err
	}
	report.Scope = *task.Scope.CloudBinding
	report.IdempotencyKey = record.key
	report.TaskResult = *task.Result
	report.CreatedAt = created
	if err := validateStoredFinalIdentity(task, record, report, created); err != nil {
		return FinalQuotaReport{}, err
	}
	return report, nil
}

func validateStoredFinalTaskFacts(task ModelTask) error {
	if task.Result == nil || task.EndedAt == nil {
		return newError(ErrorContractInvalid, "persisted final quota task has no result or end time")
	}
	if task.StartedAt == nil {
		return newError(ErrorQuotaTimeInvalid, "persisted final quota task has no started_at")
	}
	return nil
}

func validateStoredFinalIdentity(
	task ModelTask,
	record pendingRow,
	report FinalQuotaReport,
	created time.Time,
) error {
	if report.TaskID != task.TaskID || report.FreezeID != task.Freeze.FreezeID || record.sequence != 0 {
		return newError(ErrorIdempotencyConflict, "persisted final quota identity does not match its row")
	}
	if !report.ClientSummary.StartedAt.Equal(*task.StartedAt) ||
		!report.ClientSummary.FinishedAt.Equal(*task.EndedAt) || !created.Equal(*task.EndedAt) {
		return newError(ErrorIdempotencyConflict, "persisted final quota identity does not match its row")
	}
	return nil
}

func decodePendingRowMetadata(record pendingRow) (time.Time, time.Time, *time.Time, *string, error) {
	created, err := parseStamp(record.created)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil,
			newError(ErrorContractInvalid, "decode persisted report created_at: "+err.Error())
	}
	updated, err := parseStamp(record.updated)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil,
			newError(ErrorContractInvalid, "decode persisted report updated_at: "+err.Error())
	}
	next, err := decodeOptionalReportTime(record.next)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil,
			newError(ErrorContractInvalid, "decode persisted report next_attempt_at: "+err.Error())
	}
	return created, updated, next, decodeOptionalReportString(record.lastError), nil
}

func decodeOptionalReportTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	stamp, err := parseStamp(value.String)
	if err != nil {
		return nil, err
	}
	return &stamp, nil
}

func decodeOptionalReportString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func quotaPayloadHash(payload []byte) string {
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

func reportQueueStamp(value time.Time) string {
	return value.UTC().Format(reportQueueTimeLayout)
}

func nullableReportQueueTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return reportQueueStamp(*value)
}

var _ ReportStore = (*SQLiteReportStore)(nil)
var _ FinalQuotaReportCorrector = (*SQLiteReportStore)(nil)
var _ FinalQuotaReportCorrector = (*SQLiteFinalQuotaReportCorrector)(nil)
