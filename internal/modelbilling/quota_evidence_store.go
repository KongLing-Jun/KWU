package modelbilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type SQLiteQuotaEvidenceStore struct{ db *sql.DB }

func NewSQLiteQuotaEvidenceStore(db *sql.DB) *SQLiteQuotaEvidenceStore {
	return &SQLiteQuotaEvidenceStore{db: db}
}

// Append adds one supplement revision. Revision 1 is owned by CallStore.Complete
// so terminal call facts and their initial evidence commit atomically.
func (s *SQLiteQuotaEvidenceStore) Append(
	ctx context.Context,
	scope TaskScope,
	taskID string,
	callID string,
	revision QuotaEvidenceRevision,
) error {
	if err := validateQuotaEvidenceAppendInput(s, taskID, callID, revision); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := appendQuotaEvidenceInTx(ctx, tx, scope, taskID, callID, revision); err != nil {
		return err
	}
	return tx.Commit()
}

func validateQuotaEvidenceAppendInput(
	store *SQLiteQuotaEvidenceStore,
	taskID, callID string,
	revision QuotaEvidenceRevision,
) error {
	if store == nil || store.db == nil {
		return newError(ErrorContractInvalid, "quota evidence store database is required")
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return err
	}
	if err := requireTrimmed("call_id", callID); err != nil {
		return err
	}
	return revision.Validate()
}

func appendQuotaEvidenceInTx(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	taskID, callID string,
	revision QuotaEvidenceRevision,
) error {
	task, err := loadTask(ctx, tx, taskID)
	if err != nil {
		return taskStoreLoadError(err)
	}
	if err := validateQuotaEvidenceTask(task, scope); err != nil {
		return err
	}
	reportStatus, err := loadQuotaEvidenceCallStatus(ctx, tx, taskID, callID)
	if err != nil {
		return taskStoreLoadError(err)
	}
	current, found, err := loadLatestQuotaEvidenceForAppend(ctx, tx, taskID, callID)
	if err != nil {
		return persistedFieldError("quota evidence", err)
	}
	if !found {
		return newError(ErrorInvalidStateTransition, "initial quota evidence must be persisted with call completion")
	}
	if revision.Revision <= current.Revision {
		return verifyQuotaEvidenceRetry(ctx, tx, taskID, callID, revision)
	}
	if err := validateQuotaEvidenceAdvance(reportStatus, current, revision); err != nil {
		return err
	}
	return insertQuotaEvidenceRevision(ctx, tx, taskID, callID, revision)
}

func validateQuotaEvidenceTask(task ModelTask, scope TaskScope) error {
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "quota evidence requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return newError(ErrorScopeMismatch, "quota evidence task scope mismatch")
	}
	return nil
}

func verifyQuotaEvidenceRetry(
	ctx context.Context,
	tx *sql.Tx,
	taskID, callID string,
	revision QuotaEvidenceRevision,
) error {
	persisted, found, err := loadQuotaEvidenceRevision(ctx, tx, taskID, callID, revision.Revision)
	if err != nil {
		return persistedFieldError("quota evidence", err)
	}
	if !found || !quotaEvidenceEqual(&persisted, &revision) {
		return newError(ErrorIdempotencyConflict, "quota evidence revision conflicts with persisted content")
	}
	return nil
}

func validateQuotaEvidenceAdvance(
	reportStatus QuotaReportStatus,
	current, revision QuotaEvidenceRevision,
) error {
	if reportStatus == QuotaReportFinalPending || reportStatus == QuotaReportFinalReported {
		return newError(ErrorInvalidStateTransition, "final-bound quota evidence is immutable")
	}
	if revision.Revision != current.Revision+1 {
		return newError(ErrorIdempotencyConflict, "quota evidence revision must append by one")
	}
	if revision.ObservedAt.Before(current.ObservedAt) {
		return newError(ErrorContractInvalid, "quota evidence observed_at cannot move backwards")
	}
	if current.Status == QuotaEvidenceComplete && revision.Status != QuotaEvidenceComplete {
		return newError(ErrorInvalidStateTransition, "complete quota evidence cannot regress")
	}
	return nil
}

func loadQuotaEvidenceCallStatus(
	ctx context.Context,
	q taskQuerier,
	taskID string,
	callID string,
) (QuotaReportStatus, error) {
	var stored string
	if err := q.QueryRowContext(ctx, `SELECT quota_report_status FROM model_calls
		WHERE task_id=? AND call_id=?`, taskID, callID).Scan(&stored); err != nil {
		return "", err
	}
	status := QuotaReportStatus(stored)
	if err := status.Validate(); err != nil {
		return "", persistedFieldError("call quota_report_status", err)
	}
	return status, nil
}

func loadLatestQuotaEvidenceForAppend(
	ctx context.Context,
	q taskQuerier,
	taskID string,
	callID string,
) (QuotaEvidenceRevision, bool, error) {
	row := q.QueryRowContext(ctx, quotaEvidenceSelect+`
		FROM model_call_quota_evidence_revisions
		WHERE task_id=? AND call_id=?
		ORDER BY evidence_revision DESC LIMIT 1`, taskID, callID)
	return scanOptionalQuotaEvidence(row)
}

func loadQuotaEvidenceRevision(
	ctx context.Context,
	q taskQuerier,
	taskID string,
	callID string,
	revision uint64,
) (QuotaEvidenceRevision, bool, error) {
	row := q.QueryRowContext(ctx, quotaEvidenceSelect+`
		FROM model_call_quota_evidence_revisions
		WHERE task_id=? AND call_id=? AND evidence_revision=?`, taskID, callID, revision)
	return scanOptionalQuotaEvidence(row)
}

func loadQuotaEvidenceHistory(
	ctx context.Context,
	q taskQuerier,
	taskID string,
	callID string,
) ([]QuotaEvidenceRevision, error) {
	var maximum sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT MAX(evidence_revision)
		FROM model_call_quota_evidence_revisions WHERE task_id=? AND call_id=?`, taskID, callID).Scan(&maximum); err != nil {
		return nil, err
	}
	if !maximum.Valid {
		return nil, nil
	}
	history := make([]QuotaEvidenceRevision, 0, maximum.Int64)
	for revision := int64(1); revision <= maximum.Int64; revision++ {
		value, found, err := loadQuotaEvidenceRevision(ctx, q, taskID, callID, uint64(revision))
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, newError(ErrorContractInvalid, "persisted quota evidence history is not contiguous")
		}
		history = append(history, value)
	}
	return history, nil
}

func scanOptionalQuotaEvidence(row quotaEvidenceScanner) (QuotaEvidenceRevision, bool, error) {
	revision, err := scanQuotaEvidence(row)
	if err == sql.ErrNoRows {
		return QuotaEvidenceRevision{}, false, nil
	}
	if err != nil {
		return QuotaEvidenceRevision{}, false, err
	}
	return revision, true, nil
}

func (s *SQLiteQuotaEvidenceStore) Current(
	ctx context.Context,
	scope TaskScope,
	taskID string,
	callID string,
) (QuotaEvidenceRevision, bool, error) {
	if s == nil || s.db == nil {
		return QuotaEvidenceRevision{}, false, newError(ErrorContractInvalid, "quota evidence store database is required")
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return QuotaEvidenceRevision{}, false, taskStoreLoadError(err)
	}
	if task.BillingSource != BillingSubscriptionPool {
		return QuotaEvidenceRevision{}, false, newError(ErrorBillingSourceMismatch, "quota evidence requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return QuotaEvidenceRevision{}, false, newError(ErrorScopeMismatch, "quota evidence task scope mismatch")
	}
	return loadCurrentQuotaEvidence(ctx, s.db, taskID, callID)
}

func (s *SQLiteQuotaEvidenceStore) History(
	ctx context.Context,
	scope TaskScope,
	taskID string,
	callID string,
) ([]QuotaEvidenceRevision, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "quota evidence store database is required")
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return nil, taskStoreLoadError(err)
	}
	if task.BillingSource != BillingSubscriptionPool {
		return nil, newError(ErrorBillingSourceMismatch, "quota evidence requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return nil, newError(ErrorScopeMismatch, "quota evidence task scope mismatch")
	}
	rows, err := s.db.QueryContext(ctx, quotaEvidenceSelect+`
		FROM model_call_quota_evidence_revisions
		WHERE task_id=? AND call_id=? ORDER BY evidence_revision`, taskID, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []QuotaEvidenceRevision
	for rows.Next() {
		revision, err := scanQuotaEvidence(rows)
		if err != nil {
			return nil, err
		}
		history = append(history, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return history, nil
}

const quotaEvidenceSelect = `SELECT evidence_revision,oneapi_request_id,
	billing_quota_present,billing_quota_type,billing_quota_value,
	usage_quota_present,usage_quota_type,usage_quota_value,
	quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at`

func loadCurrentQuotaEvidence(
	ctx context.Context,
	q taskQuerier,
	taskID string,
	callID string,
) (QuotaEvidenceRevision, bool, error) {
	if err := validateQuotaEvidenceRequestIDs(ctx, q, taskID, callID); err != nil {
		return QuotaEvidenceRevision{}, false, err
	}
	history, err := loadQuotaEvidenceHistory(ctx, q, taskID, callID)
	if err != nil {
		return QuotaEvidenceRevision{}, false, persistedFieldError("quota evidence history", err)
	}
	if err := validateCompleteQuotaEvidenceHistory(history); err != nil {
		return QuotaEvidenceRevision{}, false, err
	}
	return loadCurrentQuotaEvidenceRow(ctx, q, taskID, callID)
}

func validateQuotaEvidenceRequestIDs(ctx context.Context, q taskQuerier, taskID, callID string) error {
	var requestIDCount int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(DISTINCT oneapi_request_id)
		FROM model_call_quota_evidence_revisions
		WHERE task_id=? AND call_id=? AND oneapi_request_id IS NOT NULL`, taskID, callID).Scan(&requestIDCount); err != nil {
		return persistedFieldError("quota evidence request IDs", err)
	}
	if requestIDCount > 1 {
		return newError(
			ErrorContractInvalid,
			"multiple Maas request IDs for one call require an external contract",
		)
	}
	return nil
}

func validateCompleteQuotaEvidenceHistory(history []QuotaEvidenceRevision) error {
	var complete *QuotaEvidenceRevision
	for i := range history {
		if history[i].Status != QuotaEvidenceComplete {
			continue
		}
		if complete != nil && !quotaEvidenceProjectionEqual(complete, &history[i]) &&
			!history[i].ObservedAt.After(complete.ObservedAt) {
			return newError(
				ErrorContractInvalid,
				"conflicting complete Maas quota facts have no later observation",
			)
		}
		complete = &history[i]
	}
	return nil
}

func loadCurrentQuotaEvidenceRow(
	ctx context.Context,
	q taskQuerier,
	taskID, callID string,
) (QuotaEvidenceRevision, bool, error) {
	row := q.QueryRowContext(ctx, quotaEvidenceSelect+`
		FROM model_call_quota_evidence_current WHERE task_id=? AND call_id=?`, taskID, callID)
	revision, err := scanQuotaEvidence(row)
	if err == sql.ErrNoRows {
		return QuotaEvidenceRevision{}, false, nil
	}
	if err != nil {
		return QuotaEvidenceRevision{}, false, persistedFieldError("quota evidence", err)
	}
	return revision, true, nil
}

type quotaEvidenceScanner interface{ Scan(...any) error }

type quotaEvidenceRowFields struct {
	revisionNumber                                   int64
	requestID, billingType, usageType, quota, source sql.NullString
	billingPresent, usagePresent                     int
	billingValue, usageValue, issuesJSON             []byte
	status, observedAt                               string
}

func scanQuotaEvidence(row quotaEvidenceScanner) (QuotaEvidenceRevision, error) {
	fields, err := scanQuotaEvidenceFields(row)
	if err != nil {
		return QuotaEvidenceRevision{}, err
	}
	revision, err := fields.revision()
	if err != nil {
		return QuotaEvidenceRevision{}, err
	}
	if err := revision.Validate(); err != nil {
		return QuotaEvidenceRevision{}, err
	}
	return revision, nil
}

func scanQuotaEvidenceFields(row quotaEvidenceScanner) (quotaEvidenceRowFields, error) {
	var fields quotaEvidenceRowFields
	if err := row.Scan(
		&fields.revisionNumber, &fields.requestID,
		&fields.billingPresent, &fields.billingType, &fields.billingValue,
		&fields.usagePresent, &fields.usageType, &fields.usageValue,
		&fields.quota, &fields.source, &fields.status, &fields.issuesJSON, &fields.observedAt,
	); err != nil {
		return quotaEvidenceRowFields{}, err
	}
	return fields, nil
}

func (fields quotaEvidenceRowFields) revision() (QuotaEvidenceRevision, error) {
	if fields.revisionNumber <= 0 {
		return QuotaEvidenceRevision{}, newError(ErrorContractInvalid, "persisted quota evidence revision is invalid")
	}
	revision := QuotaEvidenceRevision{
		Revision:        uint64(fields.revisionNumber),
		BillingQuotaRaw: hydrateQuotaRawValue(fields.billingPresent, fields.billingType, fields.billingValue),
		UsageQuotaRaw:   hydrateQuotaRawValue(fields.usagePresent, fields.usageType, fields.usageValue),
		Status:          QuotaEvidenceStatus(fields.status),
	}
	if err := hydrateQuotaEvidenceOptionalFields(&revision, fields); err != nil {
		return QuotaEvidenceRevision{}, err
	}
	if err := hydrateQuotaEvidenceIssues(&revision, fields.issuesJSON); err != nil {
		return QuotaEvidenceRevision{}, err
	}
	observedAt, err := parseStamp(fields.observedAt)
	if err != nil {
		return QuotaEvidenceRevision{}, err
	}
	revision.ObservedAt = observedAt
	return revision, nil
}

func hydrateQuotaEvidenceOptionalFields(revision *QuotaEvidenceRevision, fields quotaEvidenceRowFields) error {
	if fields.requestID.Valid {
		value := fields.requestID.String
		revision.OneAPIRequestID = &value
	}
	if fields.quota.Valid {
		value, err := NewDecimalString(fields.quota.String)
		if err != nil {
			return err
		}
		revision.Quota = &value
	}
	if fields.source.Valid {
		value := QuotaSource(fields.source.String)
		revision.Source = &value
	}
	return nil
}

func hydrateQuotaEvidenceIssues(revision *QuotaEvidenceRevision, issuesJSON []byte) error {
	if err := json.Unmarshal(issuesJSON, &revision.Issues); err != nil {
		return err
	}
	normalized, err := normalizeEvidenceIssues(revision.Issues)
	if err != nil || !quotaEvidenceIssuesEqual(normalized, revision.Issues) {
		return newError(ErrorContractInvalid, "persisted quota evidence issues are not normalized")
	}
	return nil
}

func hydrateQuotaRawValue(present int, typeValue sql.NullString, value []byte) QuotaRawValue {
	if present == 0 {
		return QuotaRawValue{}
	}
	return QuotaRawValue{Present: true, Type: QuotaJSONType(typeValue.String), Value: append(json.RawMessage(nil), value...)}
}

func insertQuotaEvidenceRevision(
	ctx context.Context,
	tx *sql.Tx,
	taskID string,
	callID string,
	revision QuotaEvidenceRevision,
) error {
	if err := revision.Validate(); err != nil {
		return err
	}
	issues, err := json.Marshal(revision.Issues)
	if err != nil {
		return newError(ErrorContractInvalid, "encode quota evidence issues")
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO model_call_quota_evidence_revisions (
		call_id,task_id,evidence_revision,oneapi_request_id,
		billing_quota_present,billing_quota_type,billing_quota_value,
		usage_quota_present,usage_quota_type,usage_quota_value,
		quota,quota_source,evidence_status,evidence_issues_json,evidence_observed_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		callID, taskID, revision.Revision, nullableString(revision.OneAPIRequestID),
		quotaRawPresent(revision.BillingQuotaRaw), quotaRawType(revision.BillingQuotaRaw), quotaRawJSON(revision.BillingQuotaRaw),
		quotaRawPresent(revision.UsageQuotaRaw), quotaRawType(revision.UsageQuotaRaw), quotaRawJSON(revision.UsageQuotaRaw),
		nullableDecimal(revision.Quota), nullableQuotaSource(revision.Source), revision.Status, issues, stamp(revision.ObservedAt),
	)
	if err != nil {
		return err
	}
	return exactlyOne(result, "append quota evidence revision")
}

func quotaRawPresent(value QuotaRawValue) int {
	if value.Present {
		return 1
	}
	return 0
}

func quotaRawType(value QuotaRawValue) any {
	if !value.Present {
		return nil
	}
	return string(value.Type)
}

func quotaRawJSON(value QuotaRawValue) any {
	if !value.Present {
		return nil
	}
	return []byte(value.Value)
}

func nullableDecimal(value *DecimalString) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func nullableQuotaSource(value *QuotaSource) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func quotaEvidenceEqual(left, right *QuotaEvidenceRevision) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Revision == right.Revision && left.ObservedAt.Equal(right.ObservedAt) &&
		quotaEvidenceProjectionEqual(left, right)
}

func quotaRawEqual(left, right QuotaRawValue) bool {
	return left.Present == right.Present && left.Type == right.Type && bytesEqual(left.Value, right.Value)
}

func missingQuotaEvidence(requestID *string, observedAt time.Time) QuotaEvidenceRevision {
	evidence := deriveQuotaEvidence(requestID, QuotaRawValue{}, QuotaRawValue{})
	evidence.Revision = 1
	evidence.ObservedAt = observedAt.UTC()
	return evidence
}

func cloneQuotaEvidence(value *QuotaEvidenceRevision) *QuotaEvidenceRevision {
	if value == nil {
		return nil
	}
	copy := *value
	copy.OneAPIRequestID = cloneOptionalString(value.OneAPIRequestID)
	copy.BillingQuotaRaw.Value = append(json.RawMessage(nil), value.BillingQuotaRaw.Value...)
	copy.UsageQuotaRaw.Value = append(json.RawMessage(nil), value.UsageQuotaRaw.Value...)
	if value.Quota != nil {
		quota := *value.Quota
		copy.Quota = &quota
	}
	if value.Source != nil {
		source := *value.Source
		copy.Source = &source
	}
	if value.Issues != nil {
		copy.Issues = append(make([]QuotaEvidenceIssue, 0, len(value.Issues)), value.Issues...)
	}
	return &copy
}
