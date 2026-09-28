package modelbilling

import (
	"context"
	"database/sql"
	"time"
)

type QuotaWorkflowStatus string

const (
	QuotaWorkflowCollecting           QuotaWorkflowStatus = "collecting"
	QuotaWorkflowEvidencePending      QuotaWorkflowStatus = "evidence_pending"
	QuotaWorkflowSupplementing        QuotaWorkflowStatus = "supplementing"
	QuotaWorkflowFinalUploadPending   QuotaWorkflowStatus = "final_upload_pending"
	QuotaWorkflowFinalUploading       QuotaWorkflowStatus = "final_uploading"
	QuotaWorkflowSettlementPending    QuotaWorkflowStatus = "settlement_pending"
	QuotaWorkflowSettled              QuotaWorkflowStatus = "settled"
	QuotaWorkflowSettledWithUncovered QuotaWorkflowStatus = "settled_with_uncovered_amount"
	QuotaWorkflowRetryPending         QuotaWorkflowStatus = "retry_pending"
	QuotaWorkflowFailedManualRequired QuotaWorkflowStatus = "failed_manual_required"
)

type QuotaRetryStage string

const (
	QuotaRetryFinalUpload     QuotaRetryStage = "final_upload"
	QuotaRetrySettlementQuery QuotaRetryStage = "settlement_query"
)

type QuotaWorkflow struct {
	TaskID        string              `json:"task_id"`
	Scope         TaskScope           `json:"scope"`
	Status        QuotaWorkflowStatus `json:"status"`
	RetryStage    *QuotaRetryStage    `json:"retry_stage,omitempty"`
	LastErrorCode *string             `json:"last_error_code,omitempty"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

func (w QuotaWorkflow) Validate() error {
	if err := requireTrimmed("task_id", w.TaskID); err != nil {
		return err
	}
	if err := w.Scope.Validate(BillingSubscriptionPool); err != nil {
		return err
	}
	if !validQuotaWorkflowStatus(w.Status) || w.UpdatedAt.IsZero() || w.UpdatedAt.Location() != time.UTC {
		return newError(ErrorContractInvalid, "invalid quota workflow status or updated_at")
	}
	if err := validateQuotaWorkflowRetryFacts(w.Status, w.RetryStage); err != nil {
		return err
	}
	if w.LastErrorCode != nil {
		return requireTrimmed("last_error_code", *w.LastErrorCode)
	}
	return nil
}

type QuotaWorkflowUpdate struct {
	Scope         TaskScope
	TaskID        string
	From          QuotaWorkflowStatus
	To            QuotaWorkflowStatus
	RetryStage    *QuotaRetryStage
	LastErrorCode *string
	At            time.Time
}

type SQLiteQuotaWorkflowStore struct{ db *sql.DB }

func NewSQLiteQuotaWorkflowStore(db *sql.DB) *SQLiteQuotaWorkflowStore {
	return &SQLiteQuotaWorkflowStore{db: db}
}

func (s *SQLiteQuotaWorkflowStore) Get(ctx context.Context, scope TaskScope, taskID string) (QuotaWorkflow, error) {
	if s == nil || s.db == nil {
		return QuotaWorkflow{}, newError(ErrorContractInvalid, "quota workflow store database is required")
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return QuotaWorkflow{}, taskStoreLoadError(err)
	}
	if task.BillingSource != BillingSubscriptionPool {
		return QuotaWorkflow{}, newError(ErrorBillingSourceMismatch, "quota workflow requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return QuotaWorkflow{}, newError(ErrorScopeMismatch, "quota workflow task scope mismatch")
	}
	return loadQuotaWorkflow(ctx, s.db, task)
}

func (s *SQLiteQuotaWorkflowStore) Update(ctx context.Context, update QuotaWorkflowUpdate) error {
	candidate, err := validateQuotaWorkflowUpdateInput(s, update)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := loadQuotaWorkflowForUpdate(ctx, tx, update)
	if err != nil {
		return err
	}
	idempotent, err := validateQuotaWorkflowChange(current, candidate, update.From)
	if err != nil {
		return err
	}
	if idempotent {
		return tx.Commit()
	}
	if err := persistQuotaWorkflowUpdate(ctx, tx, current, candidate); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkSupplementFailedManualRequired is the raw, scope-safe fallback for a
// supplement workflow whose task or workflow row cannot be hydrated. It only
// performs transitions already allowed by quotaWorkflowTransitions.
func (s *SQLiteQuotaWorkflowStore) MarkSupplementFailedManualRequired(
	ctx context.Context,
	scope TaskScope,
	taskID string,
	at time.Time,
	code string,
) error {
	if err := validateSupplementManualRecovery(s, at, code); err != nil {
		return err
	}
	key, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := quarantineQuotaSupplementWorkflow(ctx, tx, key, taskID, at, code); err != nil {
		return err
	}
	return tx.Commit()
}

func validateSupplementManualRecovery(s *SQLiteQuotaWorkflowStore, at time.Time, code string) error {
	if s == nil || s.db == nil || at.IsZero() {
		return newError(ErrorContractInvalid, "quota workflow store and recovery time are required")
	}
	return requireTrimmed("quota workflow recovery error code", code)
}

func quarantineQuotaSupplementWorkflow(
	ctx context.Context,
	tx *sql.Tx,
	key string,
	taskID string,
	at time.Time,
	code string,
) error {
	var status, updatedAt string
	if err := tx.QueryRowContext(ctx, `SELECT w.status,w.updated_at
		FROM model_task_quota_workflow w
		JOIN model_tasks t ON t.task_id=w.task_id
		WHERE w.task_id=? AND t.scope_key=? AND t.billing_source=?`,
		taskID, key, BillingSubscriptionPool).Scan(&status, &updatedAt); err != nil {
		return taskStoreLoadError(err)
	}
	current := QuotaWorkflowStatus(status)
	if current != QuotaWorkflowEvidencePending && current != QuotaWorkflowSupplementing {
		return newError(ErrorInvalidStateTransition, "quota workflow cannot enter manual recovery from its current state")
	}
	when := at.UTC()
	if persisted, parseErr := parseStamp(updatedAt); parseErr == nil && persisted.After(when) {
		when = persisted
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_task_quota_workflow
		SET status=?,retry_stage=NULL,last_error_code=?,updated_at=?
		WHERE task_id=? AND status=?`,
		QuotaWorkflowFailedManualRequired, code, stamp(when), taskID, current)
	if err != nil {
		return err
	}
	if err := exactlyOne(result, "quarantine quota supplement workflow"); err != nil {
		return err
	}
	return nil
}

func validateQuotaWorkflowUpdateInput(
	s *SQLiteQuotaWorkflowStore,
	update QuotaWorkflowUpdate,
) (QuotaWorkflow, error) {
	if s == nil || s.db == nil || update.At.IsZero() {
		return QuotaWorkflow{}, newError(ErrorContractInvalid, "quota workflow store and update time are required")
	}
	if !validQuotaWorkflowStatus(update.From) || !validQuotaWorkflowStatus(update.To) {
		return QuotaWorkflow{}, newError(ErrorContractInvalid, "invalid quota workflow transition")
	}
	candidate := QuotaWorkflow{
		TaskID: update.TaskID, Scope: update.Scope, Status: update.To, RetryStage: cloneRetryStage(update.RetryStage),
		LastErrorCode: cloneOptionalString(update.LastErrorCode), UpdatedAt: update.At.UTC(),
	}
	return candidate, candidate.Validate()
}

func loadQuotaWorkflowForUpdate(
	ctx context.Context,
	tx *sql.Tx,
	update QuotaWorkflowUpdate,
) (QuotaWorkflow, error) {
	task, err := loadTask(ctx, tx, update.TaskID)
	if err != nil {
		return QuotaWorkflow{}, taskStoreLoadError(err)
	}
	if task.BillingSource != BillingSubscriptionPool {
		return QuotaWorkflow{}, newError(ErrorBillingSourceMismatch, "quota workflow requires a subscription task")
	}
	if !task.Scope.Equal(update.Scope) {
		return QuotaWorkflow{}, newError(ErrorScopeMismatch, "quota workflow task scope mismatch")
	}
	return loadQuotaWorkflow(ctx, tx, task)
}

func persistQuotaWorkflowUpdate(
	ctx context.Context,
	tx *sql.Tx,
	current, candidate QuotaWorkflow,
) error {
	result, err := tx.ExecContext(ctx, `UPDATE model_task_quota_workflow
		SET status=?,retry_stage=?,last_error_code=?,updated_at=?
		WHERE task_id=? AND status=?`, candidate.Status, nullableRetryStage(candidate.RetryStage), nullableString(candidate.LastErrorCode),
		stamp(candidate.UpdatedAt), candidate.TaskID, current.Status)
	if err != nil {
		return err
	}
	return exactlyOne(result, "update quota workflow")
}

func (s *SQLiteQuotaWorkflowStore) ListIDs(
	ctx context.Context,
	scope TaskScope,
	statuses ...QuotaWorkflowStatus,
) ([]string, error) {
	if s == nil || s.db == nil || len(statuses) == 0 {
		return nil, newError(ErrorContractInvalid, "quota workflow store and statuses are required")
	}
	key, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return nil, err
	}
	query := `SELECT w.task_id FROM model_task_quota_workflow w
		JOIN model_tasks t ON t.task_id=w.task_id
		WHERE t.scope_key=? AND t.billing_source=? AND w.status IN (`
	args := []any{key, BillingSubscriptionPool}
	for index, status := range statuses {
		if !validQuotaWorkflowStatus(status) {
			return nil, newError(ErrorContractInvalid, "invalid quota workflow status")
		}
		if index > 0 {
			query += ","
		}
		query += "?"
		args = append(args, status)
	}
	query += `) ORDER BY w.updated_at,w.task_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collectIDs(rows)
}

func insertInitialQuotaWorkflow(ctx context.Context, tx *sql.Tx, task ModelTask) error {
	if task.BillingSource != BillingSubscriptionPool {
		return nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO model_task_quota_workflow
		(task_id,status,retry_stage,last_error_code,updated_at) VALUES (?,?,?,?,?)`,
		task.TaskID, QuotaWorkflowCollecting, nil, nil, stamp(task.CreatedAt))
	if err != nil {
		return err
	}
	return exactlyOne(result, "initialize quota workflow")
}

func loadQuotaWorkflow(ctx context.Context, q taskQuerier, task ModelTask) (QuotaWorkflow, error) {
	var status, updatedAt string
	var retryStage, lastError sql.NullString
	if err := q.QueryRowContext(ctx, `SELECT status,retry_stage,last_error_code,updated_at
		FROM model_task_quota_workflow WHERE task_id=?`, task.TaskID).Scan(&status, &retryStage, &lastError, &updatedAt); err != nil {
		return QuotaWorkflow{}, taskStoreLoadError(err)
	}
	updated, err := parseStamp(updatedAt)
	if err != nil {
		return QuotaWorkflow{}, persistedFieldError("quota workflow updated_at", err)
	}
	workflow := QuotaWorkflow{TaskID: task.TaskID, Scope: cloneTaskScope(task.Scope), Status: QuotaWorkflowStatus(status), UpdatedAt: updated}
	if retryStage.Valid {
		value := QuotaRetryStage(retryStage.String)
		workflow.RetryStage = &value
	}
	if lastError.Valid {
		value := lastError.String
		workflow.LastErrorCode = &value
	}
	if err := workflow.Validate(); err != nil {
		return QuotaWorkflow{}, err
	}
	return workflow, nil
}

func validQuotaWorkflowStatus(status QuotaWorkflowStatus) bool {
	switch status {
	case QuotaWorkflowCollecting, QuotaWorkflowEvidencePending, QuotaWorkflowSupplementing,
		QuotaWorkflowFinalUploadPending, QuotaWorkflowFinalUploading, QuotaWorkflowSettlementPending,
		QuotaWorkflowSettled, QuotaWorkflowSettledWithUncovered, QuotaWorkflowRetryPending,
		QuotaWorkflowFailedManualRequired:
		return true
	default:
		return false
	}
}

func validQuotaRetryStage(stage QuotaRetryStage) bool {
	return stage == QuotaRetryFinalUpload || stage == QuotaRetrySettlementQuery
}

func validateQuotaWorkflowRetryFacts(status QuotaWorkflowStatus, stage *QuotaRetryStage) error {
	if status == QuotaWorkflowRetryPending {
		if stage == nil || !validQuotaRetryStage(*stage) {
			return newError(ErrorContractInvalid, "retry_pending quota workflow requires retry_stage")
		}
		return nil
	}
	if stage != nil {
		return newError(ErrorContractInvalid, "non-retry quota workflow cannot contain retry_stage")
	}
	return nil
}

var quotaWorkflowTransitions = map[QuotaWorkflowStatus]map[QuotaWorkflowStatus]bool{
	QuotaWorkflowCollecting:         states(QuotaWorkflowEvidencePending, QuotaWorkflowFinalUploadPending, QuotaWorkflowFailedManualRequired),
	QuotaWorkflowEvidencePending:    states(QuotaWorkflowSupplementing, QuotaWorkflowFinalUploadPending, QuotaWorkflowFailedManualRequired),
	QuotaWorkflowSupplementing:      states(QuotaWorkflowEvidencePending, QuotaWorkflowFinalUploadPending, QuotaWorkflowFailedManualRequired),
	QuotaWorkflowFinalUploadPending: states(QuotaWorkflowFinalUploading, QuotaWorkflowRetryPending, QuotaWorkflowFailedManualRequired),
	QuotaWorkflowFinalUploading:     states(QuotaWorkflowFinalUploadPending, QuotaWorkflowSettlementPending, QuotaWorkflowRetryPending, QuotaWorkflowFailedManualRequired),
	QuotaWorkflowSettlementPending:  states(QuotaWorkflowSettled, QuotaWorkflowSettledWithUncovered, QuotaWorkflowRetryPending, QuotaWorkflowFailedManualRequired),
	QuotaWorkflowRetryPending: states(
		QuotaWorkflowFinalUploading, QuotaWorkflowSettlementPending, QuotaWorkflowSettled,
		QuotaWorkflowSettledWithUncovered, QuotaWorkflowFailedManualRequired,
	),
}

func validateQuotaWorkflowChange(
	current, candidate QuotaWorkflow,
	declaredFrom QuotaWorkflowStatus,
) (bool, error) {
	if candidate.UpdatedAt.Before(current.UpdatedAt) {
		return false, newError(ErrorInvalidStateTransition, "quota workflow updated_at cannot move backwards")
	}
	if current.Status == candidate.Status {
		if quotaWorkflowContentEqual(current, candidate) {
			return true, nil
		}
		return false, newError(ErrorIdempotencyConflict, "quota workflow retry conflicts with persisted content")
	}
	if current.Status != declaredFrom {
		return false, newError(ErrorInvalidStateTransition, "declared quota workflow state does not match persisted state")
	}
	if !quotaWorkflowTransitions[current.Status][candidate.Status] {
		return false, newError(ErrorInvalidStateTransition, "invalid quota workflow transition")
	}
	if err := validateQuotaRetryTransition(current, candidate); err != nil {
		return false, err
	}
	return false, nil
}

func validateQuotaRetryTransition(current, candidate QuotaWorkflow) error {
	if candidate.Status == QuotaWorkflowRetryPending {
		expected, ok := quotaRetryStageForSource(current.Status)
		if !ok || candidate.RetryStage == nil || *candidate.RetryStage != expected {
			return newError(ErrorInvalidStateTransition, "quota retry stage does not match its source")
		}
	}
	if current.Status == QuotaWorkflowRetryPending &&
		(current.RetryStage == nil || !quotaRetryRecoveryAllowed(*current.RetryStage, candidate.Status)) {
		return newError(ErrorInvalidStateTransition, "quota retry stage does not allow the recovery target")
	}
	return nil
}

func quotaRetryStageForSource(status QuotaWorkflowStatus) (QuotaRetryStage, bool) {
	switch status {
	case QuotaWorkflowFinalUploadPending, QuotaWorkflowFinalUploading:
		return QuotaRetryFinalUpload, true
	case QuotaWorkflowSettlementPending:
		return QuotaRetrySettlementQuery, true
	default:
		return "", false
	}
}

func quotaRetryRecoveryAllowed(stage QuotaRetryStage, target QuotaWorkflowStatus) bool {
	switch stage {
	case QuotaRetryFinalUpload:
		return target == QuotaWorkflowFinalUploading
	case QuotaRetrySettlementQuery:
		return target == QuotaWorkflowSettlementPending || target == QuotaWorkflowSettled ||
			target == QuotaWorkflowSettledWithUncovered || target == QuotaWorkflowFailedManualRequired
	default:
		return false
	}
}

func cloneRetryStage(value *QuotaRetryStage) *QuotaRetryStage {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func nullableRetryStage(value *QuotaRetryStage) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func quotaWorkflowContentEqual(left, right QuotaWorkflow) bool {
	return left.TaskID == right.TaskID && left.Scope.Equal(right.Scope) && left.Status == right.Status &&
		optionalRetryStageEqual(left.RetryStage, right.RetryStage) && optionalStringEqual(left.LastErrorCode, right.LastErrorCode)
}

func optionalRetryStageEqual(left, right *QuotaRetryStage) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
