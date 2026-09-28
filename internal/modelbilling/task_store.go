package modelbilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/userctx"
)

// SQLiteTaskStore owns task bootstrap and subscription freeze write-ahead facts.
type SQLiteTaskStore struct{ db *sql.DB }

func NewSQLiteTaskStore(db *sql.DB) *SQLiteTaskStore { return &SQLiteTaskStore{db: db} }

func (s *SQLiteTaskStore) CreateTaskWithEvidence(ctx context.Context, task ModelTask, evidence PricingEvidence) error {
	if err := validateTaskBootstrap(s, task, evidence); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("modelbilling: begin task bootstrap: %w", err)
	}
	defer tx.Rollback()
	if err := createOrVerifyTask(ctx, tx, task); err != nil {
		return err
	}
	if err := InsertPricingEvidence(ctx, tx, PricingEvidenceOwner{TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource}, evidence); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteTaskStore) RecordFreezeRequest(ctx context.Context, taskContext TaskContext, request *cloud.FreezeRequest, now time.Time) error {
	if err := validateFreezeRequestInput(s, request, now); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadSubscriptionTask(ctx, tx, taskContext)
	if err != nil {
		return err
	}
	if err := applyFreezeRequest(ctx, tx, task, *request, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteTaskStore) RecordFreezeResult(ctx context.Context, taskContext TaskContext, freeze *cloud.Freeze, entitlement *TaskEntitlementSnapshot, now time.Time) error {
	if err := validateFreezeResultInput(s, freeze, entitlement, now); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadSubscriptionTask(ctx, tx, taskContext)
	if err != nil {
		return err
	}
	if err := applyFreezeResult(ctx, tx, task, freeze, entitlement, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteTaskStore) UpdateState(ctx context.Context, update TaskStateUpdate) error {
	if err := validateTaskStateUpdate(s, update); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	task, err := loadAndValidateTaskContext(ctx, tx, update.Scope, update.TaskID)
	if err != nil {
		return err
	}
	if err := applyTaskStateUpdate(ctx, tx, task, update); err != nil {
		return err
	}
	return tx.Commit()
}

func validateTaskBootstrap(s *SQLiteTaskStore, task ModelTask, evidence PricingEvidence) error {
	if s == nil || s.db == nil {
		return newError(ErrorContractInvalid, "task store database is required")
	}
	if !cleanTaskBootstrap(task) {
		return newError(ErrorContractInvalid, "task bootstrap must be a clean created task")
	}
	if err := task.Validate(); err != nil {
		return err
	}
	return evidence.ValidateForTask(task)
}

func cleanTaskBootstrap(task ModelTask) bool {
	return task.State == TaskCreated && task.StartedAt == nil && task.EndedAt == nil && task.LastErrorCode == nil &&
		task.Result == nil && task.FreezeRequest == nil && task.Freeze == nil && task.Entitlement == nil
}

func createOrVerifyTask(ctx context.Context, tx *sql.Tx, task ModelTask) error {
	err := insertTask(ctx, tx, task)
	if err == nil {
		return nil
	}
	if !isConstraint(err) {
		return err
	}
	existing, loadErr := loadTask(ctx, tx, task.TaskID)
	if loadErr != nil {
		return err
	}
	if !taskEqual(existing, task) {
		return newError(ErrorIdempotencyConflict, "task id reused with different content")
	}
	return nil
}

func validateFreezeRequestInput(s *SQLiteTaskStore, request *cloud.FreezeRequest, now time.Time) error {
	if s == nil || s.db == nil || request == nil || now.IsZero() {
		return newError(ErrorContractInvalid, "freeze request, time, and task store are required")
	}
	return nil
}

func loadSubscriptionTask(ctx context.Context, tx *sql.Tx, value TaskContext) (ModelTask, error) {
	task, err := loadTask(ctx, tx, value.TaskID)
	if err != nil {
		return ModelTask{}, taskStoreLoadError(err)
	}
	if err := validatePersistedTaskContext(value, task); err != nil {
		return ModelTask{}, err
	}
	if task.BillingSource != BillingSubscriptionPool {
		return ModelTask{}, newError(ErrorBillingSourceMismatch, "BYOK task cannot record freeze")
	}
	return task, nil
}

func applyFreezeRequest(ctx context.Context, tx *sql.Tx, task ModelTask, request cloud.FreezeRequest, now time.Time) error {
	if err := validateFreezeRequestForTask(request, task); err != nil {
		return err
	}
	if task.State == TaskFreezePending {
		return verifyExistingFreezeRequest(task, request)
	}
	if task.State != TaskCreated {
		return newError(ErrorInvalidStateTransition, "freeze request requires created task")
	}
	return persistFreezeRequest(ctx, tx, task, request, now)
}

func verifyExistingFreezeRequest(task ModelTask, request cloud.FreezeRequest) error {
	if task.FreezeRequest != nil && freezeRequestEqual(*task.FreezeRequest, request) {
		return nil
	}
	return newError(ErrorIdempotencyConflict, "freeze request conflicts with persisted task")
}

func persistFreezeRequest(ctx context.Context, tx *sql.Tx, task ModelTask, request cloud.FreezeRequest, now time.Time) error {
	result, err := tx.ExecContext(ctx, `UPDATE model_tasks SET freeze_idempotency_key=?, state=?, updated_at=? WHERE task_id=? AND state=?`, request.IdempotencyKey, TaskFreezePending, stamp(now), task.TaskID, TaskCreated)
	if err != nil {
		return err
	}
	return exactlyOne(result, "record freeze request")
}

func validateFreezeResultInput(s *SQLiteTaskStore, freeze *cloud.Freeze, entitlement *TaskEntitlementSnapshot, now time.Time) error {
	if s == nil || s.db == nil || freeze == nil || entitlement == nil || now.IsZero() {
		return newError(ErrorContractInvalid, "freeze result, entitlement, time, and task store are required")
	}
	return nil
}

func applyFreezeResult(ctx context.Context, tx *sql.Tx, task ModelTask, freeze *cloud.Freeze, entitlement *TaskEntitlementSnapshot, now time.Time) error {
	if task.State == TaskReady {
		return verifyExistingFreezeResult(task, freeze, entitlement)
	}
	if err := validateFreezeResult(task, freeze, entitlement, now); err != nil {
		return err
	}
	payload, err := json.Marshal(entitlement)
	if err != nil {
		return err
	}
	return persistFreezeResult(ctx, tx, task, freeze, payload, now)
}

func verifyExistingFreezeResult(task ModelTask, freeze *cloud.Freeze, entitlement *TaskEntitlementSnapshot) error {
	if freezeEqual(task.Freeze, freeze) && entitlementEqual(task.Entitlement, entitlement) {
		return nil
	}
	return newError(ErrorIdempotencyConflict, "freeze result conflicts with persisted task")
}

func validateFreezeResult(task ModelTask, freeze *cloud.Freeze, entitlement *TaskEntitlementSnapshot, now time.Time) error {
	if task.State != TaskFreezePending || task.FreezeRequest == nil {
		return newError(ErrorInvalidStateTransition, "freeze result requires freeze_pending task")
	}
	if err := freeze.Validate(); err != nil {
		return newError(ErrorContractInvalid, "invalid freeze result: "+err.Error())
	}
	if freeze.TaskID != task.TaskID || freeze.Scope != *task.Scope.CloudBinding {
		return newError(ErrorScopeMismatch, "freeze result does not match persisted task")
	}
	if freeze.Status != cloud.FreezeActive {
		return newError(ErrorContractInvalid, "only an active freeze can make a task ready")
	}
	candidate := task
	candidate.State, candidate.Freeze, candidate.Entitlement, candidate.UpdatedAt = TaskReady, freeze, entitlement, now
	return candidate.Validate()
}

func persistFreezeResult(ctx context.Context, tx *sql.Tx, task ModelTask, freeze *cloud.Freeze, payload []byte, now time.Time) error {
	result, err := tx.ExecContext(ctx, `UPDATE model_tasks SET freeze_id=?, freeze_status=?, freeze_amount=?, free_frozen_amount=?, subscription_frozen_amount=?, low_balance_mode=?, freeze_expires_at=?, entitlement_snapshot_json=?, state=?, updated_at=? WHERE task_id=? AND state=?`,
		freeze.FreezeID, freeze.Status, freeze.FreezeAmount, freeze.FreeFrozenAmount, freeze.SubscriptionFrozenAmount, freeze.LowBalanceMode, stamp(freeze.ExpiresAt), payload, TaskReady, stamp(now), task.TaskID, TaskFreezePending)
	if err != nil {
		return err
	}
	return exactlyOne(result, "record freeze result")
}

func validateTaskStateUpdate(s *SQLiteTaskStore, update TaskStateUpdate) error {
	if s == nil || s.db == nil || update.At.IsZero() {
		return newError(ErrorContractInvalid, "task store and time are required")
	}
	if err := requireTrimmed("task_id", update.TaskID); err != nil {
		return err
	}
	if err := requireTrimmed("actor_id", update.Scope.ActorID); err != nil {
		return err
	}
	if !userctx.ValidateProfileID(update.Scope.ProfileID) {
		return newError(ErrorContractInvalid, "invalid profile_id")
	}
	if !validTaskState(update.From) || !validTaskState(update.To) {
		return newError(ErrorContractInvalid, "invalid task state update")
	}
	if update.LastErrorCode != nil {
		return requireTrimmed("last_error_code", *update.LastErrorCode)
	}
	return nil
}

func loadAndValidateTaskContext(ctx context.Context, q taskQuerier, scope TaskScope, taskID string) (ModelTask, error) {
	task, err := loadTask(ctx, q, taskID)
	if err != nil {
		return ModelTask{}, taskStoreLoadError(err)
	}
	if !task.Scope.Equal(scope) {
		return ModelTask{}, newError(ErrorScopeMismatch, "task scope does not match persisted task")
	}
	return task, nil
}

func applyTaskStateUpdate(ctx context.Context, tx *sql.Tx, task ModelTask, update TaskStateUpdate) error {
	if err := ValidateTaskUpdate(task.BillingSource, update.From, update.To, task.Result, update.Result); err != nil {
		return err
	}
	if task.State == update.To {
		return verifyTaskStateRetry(task, update)
	}
	if task.State != update.From {
		return newError(ErrorInvalidStateTransition, "declared task state does not match persisted task")
	}
	at := update.At.UTC()
	if at.Before(task.CreatedAt) || at.Before(task.UpdatedAt) {
		return newError(ErrorQuotaTimeInvalid, "task state update precedes persisted lifecycle facts")
	}
	if task.BillingSource == BillingSubscriptionPool && update.From == TaskRecoveryRequired &&
		update.To == TaskRunning && task.StartedAt == nil {
		return newError(ErrorQuotaTimeInvalid, "subscription task cannot resume running without a proven started_at")
	}
	return persistTaskStateUpdate(ctx, tx, task, update)
}

func persistTaskStateUpdate(ctx context.Context, tx *sql.Tx, task ModelTask, update TaskStateUpdate) error {
	started := nullableTime(task.StartedAt)
	if task.StartedAt == nil && update.From == TaskReady && update.To == TaskRunning {
		started = stamp(update.At)
	}
	ended := nullableTime(task.EndedAt)
	if task.EndedAt == nil && (update.To == TaskUsagePending || update.To == TaskCompleted || update.To == TaskFailed || update.To == TaskCancelled) {
		ended = stamp(update.At)
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_tasks SET state=?, task_result=?, last_error_code=?, started_at=?, updated_at=?, ended_at=? WHERE task_id=? AND state=?`, update.To, nullableTaskResult(update.Result), nullableString(update.LastErrorCode), started, stamp(update.At), ended, update.TaskID, update.From)
	if err != nil {
		return err
	}
	return exactlyOne(result, "update task state")
}

func verifyTaskStateRetry(task ModelTask, update TaskStateUpdate) error {
	if !optionalTaskResultEqual(task.Result, update.Result) || !optionalStringEqual(task.LastErrorCode, update.LastErrorCode) {
		return newError(ErrorIdempotencyConflict, "task state retry conflicts with persisted task")
	}
	if update.From == TaskReady && update.To == TaskRunning &&
		(task.StartedAt == nil || !task.UpdatedAt.Equal(update.At.UTC())) {
		return newError(ErrorIdempotencyConflict, "task running retry conflicts with persisted lifecycle facts")
	}
	return nil
}

func (s *SQLiteTaskStore) Get(ctx context.Context, scope TaskScope, taskID string) (ModelTask, error) {
	if s == nil || s.db == nil {
		return ModelTask{}, newError(ErrorContractInvalid, "task store database is required")
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return ModelTask{}, taskStoreLoadError(err)
	}
	if !task.Scope.Equal(scope) {
		return ModelTask{}, newError(ErrorScopeMismatch, "task scope does not match persisted task")
	}
	return task, nil
}

func (s *SQLiteTaskStore) ListRecoverable(ctx context.Context, scope TaskScope) ([]ModelTask, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "task store database is required")
	}
	source := BillingNewAPIByok
	if scope.CloudBinding != nil {
		source = BillingSubscriptionPool
	}
	key, err := BuildTaskScopeKey(scope, source)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id FROM model_tasks WHERE scope_key=? AND state=? ORDER BY created_at, task_id`, key, TaskRecoveryRequired)
	if err != nil {
		return nil, err
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	var tasks []ModelTask
	for _, id := range ids {
		task, err := loadTask(ctx, s.db, id)
		if err != nil {
			return nil, err
		}
		if !task.Scope.Equal(scope) {
			return nil, newError(ErrorScopeMismatch, "recoverable task scope mismatch")
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ListStartupRecoveryIDs returns only opaque task identifiers from one exact
// local billing scope. Hydration stays per task so one malformed record cannot
// prevent the remaining startup work from being recovered.
func (s *SQLiteTaskStore) ListStartupRecoveryIDs(ctx context.Context, scope TaskScope) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "task store database is required")
	}
	source := BillingNewAPIByok
	if scope.CloudBinding != nil {
		source = BillingSubscriptionPool
	}
	key, err := BuildTaskScopeKey(scope, source)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id FROM model_tasks
		WHERE scope_key=? AND state IN (?,?,?,?)
		ORDER BY created_at,task_id`, key, TaskCreated, TaskFreezePending, TaskReady, TaskRunning)
	if err != nil {
		return nil, err
	}
	return collectIDs(rows)
}

// QuarantineStartupTask is the raw, scope-safe fallback for a startup row that
// cannot be hydrated or advanced through the ordinary store. It uses only
// existing legal transitions and never invents a started_at or cloud result.
func (s *SQLiteTaskStore) QuarantineStartupTask(
	ctx context.Context,
	scope TaskScope,
	taskID string,
	at time.Time,
	code string,
) error {
	if s == nil || s.db == nil || at.IsZero() {
		return newError(ErrorContractInvalid, "task store and recovery time are required")
	}
	if err := requireTrimmed("startup recovery error code", code); err != nil {
		return err
	}
	source := BillingNewAPIByok
	if scope.CloudBinding != nil {
		source = BillingSubscriptionPool
	}
	key, err := BuildTaskScopeKey(scope, source)
	if err != nil {
		return err
	}
	when := stamp(at)
	result, err := s.db.ExecContext(ctx, `UPDATE model_tasks SET
		state=CASE WHEN state=? THEN ? ELSE ? END,
		task_result=CASE WHEN state=? THEN ? ELSE task_result END,
		ended_at=CASE WHEN state=? THEN COALESCE(ended_at,
			MAX(?,created_at,updated_at,COALESCE(started_at,''),COALESCE(ended_at,''))) ELSE ended_at END,
		last_error_code=?,
		updated_at=MAX(?,created_at,updated_at,COALESCE(started_at,''),COALESCE(ended_at,''))
		WHERE task_id=? AND scope_key=? AND state IN (?,?,?,?)`,
		TaskCreated, TaskFailed, TaskRecoveryRequired,
		TaskCreated, TaskResultFailed,
		TaskCreated, when,
		code, when,
		taskID, key, TaskCreated, TaskFreezePending, TaskReady, TaskRunning)
	if err != nil {
		return err
	}
	return exactlyOne(result, "quarantine startup task")
}

func (s *SQLiteTaskStore) ListSettling(ctx context.Context, scope TaskScope) ([]ModelTask, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "task store database is required")
	}
	key, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT task_id FROM model_tasks
		WHERE scope_key=? AND billing_source=? AND state=?
		ORDER BY created_at, task_id`, key, BillingSubscriptionPool, TaskSettling)
	if err != nil {
		return nil, err
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	tasks, err := loadSettlingTasks(ctx, tx, ids, scope)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ListSettlingIDs avoids hydrating the whole scope at once. The coordinator
// can therefore isolate a malformed task and continue with the remaining IDs.
func (s *SQLiteTaskStore) ListSettlingIDs(ctx context.Context, scope TaskScope) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "task store database is required")
	}
	key, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id FROM model_tasks
		WHERE scope_key=? AND billing_source=? AND state=?
		ORDER BY created_at, task_id`, key, BillingSubscriptionPool, TaskSettling)
	if err != nil {
		return nil, err
	}
	return collectIDs(rows)
}

// MarkRecoveryRequired is the narrow raw-state CAS used when a malformed task
// cannot be hydrated. It deliberately changes only state and updated_at so the
// persisted result, end time, and previous error facts remain untouched.
func (s *SQLiteTaskStore) MarkRecoveryRequired(
	ctx context.Context,
	scope TaskScope,
	taskID string,
	from TaskState,
	at time.Time,
) error {
	if err := validateRecoveryCAS(s, from, at); err != nil {
		return err
	}
	// taskID comes from the scope-safe ID scan and stays opaque here so even a
	// malformed persisted identity can be moved out of the hot scan.
	key, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE model_tasks SET state=?,updated_at=?
		WHERE task_id=? AND scope_key=? AND billing_source=? AND state=?`,
		TaskRecoveryRequired, stamp(at), taskID, key, BillingSubscriptionPool, from)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	return s.recoveryCASMiss(ctx, key, taskID)
}

func validateRecoveryCAS(s *SQLiteTaskStore, from TaskState, at time.Time) error {
	if s == nil || s.db == nil || at.IsZero() {
		return newError(ErrorContractInvalid, "task store and recovery time are required")
	}
	if from != TaskUsagePending && from != TaskSettling {
		return newError(ErrorContractInvalid, "recovery CAS requires usage_pending or settling")
	}
	return nil
}

func (s *SQLiteTaskStore) recoveryCASMiss(ctx context.Context, scopeKey, taskID string) error {
	var state TaskState
	if err := s.db.QueryRowContext(ctx, `SELECT state FROM model_tasks
		WHERE task_id=? AND scope_key=? AND billing_source=?`,
		taskID, scopeKey, BillingSubscriptionPool).Scan(&state); err != nil {
		return taskStoreLoadError(err)
	}
	if state == TaskRecoveryRequired {
		return nil
	}
	return newError(ErrorInvalidStateTransition, "task state changed before recovery CAS")
}

func loadSettlingTasks(ctx context.Context, q taskQuerier, ids []string, scope TaskScope) ([]ModelTask, error) {
	tasks := make([]ModelTask, 0, len(ids))
	for _, id := range ids {
		task, err := loadTask(ctx, q, id)
		if err != nil {
			return nil, taskStoreLoadError(err)
		}
		if err := validateSettlingTask(task, scope); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func validateSettlingTask(task ModelTask, scope TaskScope) error {
	if !task.Scope.Equal(scope) {
		return newError(ErrorScopeMismatch, "settling task scope mismatch")
	}
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "settling query returned a BYOK task")
	}
	if task.State != TaskSettling {
		return newError(ErrorContractInvalid, "settling query returned an invalid task state")
	}
	return nil
}

type idRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func collectIDs(rows idRows) (ids []string, err error) {
	defer func() {
		closeErr := rows.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return ids, nil
}

type taskQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type taskRecord struct {
	task                                                                                                                                          ModelTask
	cloudUser, requestKey, freezeID, freezeStatus, freezeAmount, freeAmount, subscriptionAmount, expiresAt, result, startedAt, endedAt, lastError sql.NullString
	generation                                                                                                                                    sql.NullInt64
	lowBalance                                                                                                                                    sql.NullBool
	entitlement                                                                                                                                   []byte
	source, state, createdAt, updatedAt                                                                                                           string
}

func loadTask(ctx context.Context, q taskQuerier, taskID string) (ModelTask, error) {
	record, err := scanTaskRecord(ctx, q, taskID)
	if err != nil {
		return ModelTask{}, err
	}
	if err := hydrateTaskIdentity(&record); err != nil {
		return ModelTask{}, err
	}
	if err := hydrateTaskLifecycle(&record); err != nil {
		return ModelTask{}, err
	}
	if err := hydrateTaskFreeze(&record); err != nil {
		return ModelTask{}, err
	}
	if err := hydrateTaskEntitlement(&record); err != nil {
		return ModelTask{}, err
	}
	if err := record.task.Validate(); err != nil {
		return ModelTask{}, err
	}
	return record.task, nil
}

func scanTaskRecord(ctx context.Context, q taskQuerier, taskID string) (taskRecord, error) {
	var record taskRecord
	err := q.QueryRowContext(ctx, `SELECT task_id,actor_id,profile_id,cloud_user_id,binding_generation,billing_source,initial_model_id,client_task_type,client_estimated_cost,freeze_idempotency_key,freeze_id,freeze_status,freeze_amount,free_frozen_amount,subscription_frozen_amount,low_balance_mode,freeze_expires_at,entitlement_snapshot_json,pricing_evidence_id,state,task_result,created_at,started_at,updated_at,ended_at,last_error_code FROM model_tasks WHERE task_id=?`, taskID).Scan(
		&record.task.TaskID, &record.task.Scope.ActorID, &record.task.Scope.ProfileID, &record.cloudUser, &record.generation, &record.source, &record.task.InitialModelID, &record.task.ClientTaskType, &record.task.ClientEstimatedCost, &record.requestKey, &record.freezeID, &record.freezeStatus, &record.freezeAmount, &record.freeAmount, &record.subscriptionAmount, &record.lowBalance, &record.expiresAt, &record.entitlement, &record.task.PricingEvidenceID, &record.state, &record.result, &record.createdAt, &record.startedAt, &record.updatedAt, &record.endedAt, &record.lastError)
	return record, err
}

func hydrateTaskIdentity(record *taskRecord) error {
	record.task.BillingSource = BillingSource(record.source)
	if record.cloudUser.Valid != record.generation.Valid {
		return newError(ErrorContractInvalid, "incomplete persisted cloud binding")
	}
	if record.cloudUser.Valid {
		record.task.Scope.CloudBinding = &cloud.BindingScope{ProfileID: record.task.Scope.ProfileID, CloudUserID: record.cloudUser.String, BindingGeneration: uint64(record.generation.Int64)}
	}
	return nil
}

func hydrateTaskLifecycle(record *taskRecord) error {
	created, err := parseStamp(record.createdAt)
	if err != nil {
		return persistedFieldError("task created_at", err)
	}
	updated, err := parseStamp(record.updatedAt)
	if err != nil {
		return persistedFieldError("task updated_at", err)
	}
	record.task.State, record.task.CreatedAt, record.task.UpdatedAt = TaskState(record.state), created, updated
	if record.startedAt.Valid {
		value, err := parseStamp(record.startedAt.String)
		if err != nil {
			return persistedFieldError("task started_at", err)
		}
		record.task.StartedAt = &value
	}
	if record.result.Valid {
		value := TaskResult(record.result.String)
		record.task.Result = &value
	}
	if record.endedAt.Valid {
		value, err := parseStamp(record.endedAt.String)
		if err != nil {
			return persistedFieldError("task ended_at", err)
		}
		record.task.EndedAt = &value
	}
	if record.lastError.Valid {
		value := record.lastError.String
		record.task.LastErrorCode = &value
	}
	return nil
}

func hydrateTaskFreeze(record *taskRecord) error {
	if record.requestKey.Valid {
		if record.task.Scope.CloudBinding == nil {
			return newError(ErrorBillingSourceMismatch, "persisted freeze request has no cloud binding")
		}
		record.task.FreezeRequest = &cloud.FreezeRequest{Scope: *record.task.Scope.CloudBinding, TaskID: record.task.TaskID, ModelID: record.task.InitialModelID, ClientTaskType: record.task.ClientTaskType, ClientEstimatedCost: record.task.ClientEstimatedCost, IdempotencyKey: record.requestKey.String}
	}
	if !record.freezeID.Valid {
		return nil
	}
	if record.task.Scope.CloudBinding == nil {
		return newError(ErrorBillingSourceMismatch, "persisted freeze has no cloud binding")
	}
	expires, err := parseStamp(record.expiresAt.String)
	if err != nil {
		return persistedFieldError("task freeze_expires_at", err)
	}
	record.task.Freeze = &cloud.Freeze{Scope: *record.task.Scope.CloudBinding, FreezeID: record.freezeID.String, TaskID: record.task.TaskID, Status: cloud.FreezeStatus(record.freezeStatus.String), FreezeAmount: record.freezeAmount.String, FreeFrozenAmount: record.freeAmount.String, SubscriptionFrozenAmount: record.subscriptionAmount.String, LowBalanceMode: record.lowBalance.Bool, ExpiresAt: expires}
	return nil
}

func hydrateTaskEntitlement(record *taskRecord) error {
	if len(record.entitlement) == 0 {
		return nil
	}
	var value TaskEntitlementSnapshot
	if err := json.Unmarshal(record.entitlement, &value); err != nil {
		return persistedFieldError("task entitlement_snapshot_json", err)
	}
	record.task.Entitlement = &value
	return nil
}

func insertTask(ctx context.Context, tx *sql.Tx, task ModelTask) error {
	key, err := BuildTaskScopeKey(task.Scope, task.BillingSource)
	if err != nil {
		return err
	}
	var user, generation any
	if task.Scope.CloudBinding != nil {
		user = task.Scope.CloudBinding.CloudUserID
		generation = task.Scope.CloudBinding.BindingGeneration
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO model_tasks (task_id,scope_key,actor_id,profile_id,cloud_user_id,binding_generation,billing_source,initial_model_id,client_task_type,client_estimated_cost,pricing_evidence_id,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, task.TaskID, key, task.Scope.ActorID, task.Scope.ProfileID, user, generation, task.BillingSource, task.InitialModelID, task.ClientTaskType, task.ClientEstimatedCost, task.PricingEvidenceID, task.State, stamp(task.CreatedAt), stamp(task.UpdatedAt))
	if err != nil {
		return err
	}
	if err := exactlyOne(result, "insert task"); err != nil {
		return err
	}
	return insertInitialQuotaWorkflow(ctx, tx, task)
}

func validatePersistedTaskContext(value TaskContext, task ModelTask) error {
	if _, err := NewTaskContext(TaskContextInput{TaskID: value.TaskID, Scope: value.Scope, BillingSource: value.BillingSource, InitialModelID: value.InitialModelID, PricingEvidenceID: value.PricingEvidenceID}); err != nil {
		return err
	}
	if value.TaskID != task.TaskID || value.BillingSource != task.BillingSource || value.InitialModelID != task.InitialModelID || value.PricingEvidenceID != task.PricingEvidenceID {
		return newError(ErrorIdempotencyConflict, "task context does not match persisted task")
	}
	if !value.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "task context scope does not match persisted task")
	}
	return nil
}
func validateFreezeRequestForTask(request cloud.FreezeRequest, task ModelTask) error {
	if err := request.Validate(); err != nil {
		return newError(ErrorContractInvalid, err.Error())
	}
	if task.Scope.CloudBinding == nil || request.TaskID != task.TaskID || request.Scope != *task.Scope.CloudBinding {
		return newError(ErrorScopeMismatch, "freeze request does not match persisted task")
	}
	if request.ModelID != task.InitialModelID || request.ClientTaskType != task.ClientTaskType || request.ClientEstimatedCost != task.ClientEstimatedCost {
		return newError(ErrorContractInvalid, "freeze request does not match task start facts")
	}
	return nil
}
func taskStoreLoadError(err error) error {
	if err == sql.ErrNoRows {
		return newError(ErrorContractInvalid, "persisted task not found")
	}
	return err
}
func persistedFieldError(field string, err error) error {
	return newError(ErrorContractInvalid, fmt.Sprintf("invalid persisted %s: %v", field, err))
}
func exactlyOne(result sql.Result, operation string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return newError(ErrorInvalidStateTransition, operation+" affected no rows")
	}
	return nil
}
func isConstraint(err error) bool {
	return err != nil && (contains(err.Error(), "constraint") || contains(err.Error(), "UNIQUE"))
}
func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
func stamp(value time.Time) string               { return value.UTC().Format(time.RFC3339Nano) }
func parseStamp(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return stamp(*value)
}
func nullableTaskResult(value *TaskResult) any {
	if value == nil {
		return nil
	}
	return string(*value)
}
func freezeRequestEqual(a, b cloud.FreezeRequest) bool { return a == b }
func freezeEqual(a, b *cloud.Freeze) bool {
	if a == nil || b == nil {
		return a == b
	}
	return freezeIdentityEqual(a, b) && freezeBalanceEqual(a, b) && optionalTimeEqual(a.ExpiresAt, b.ExpiresAt)
}
func freezeIdentityEqual(a, b *cloud.Freeze) bool {
	return a.Scope == b.Scope && a.FreezeID == b.FreezeID && a.TaskID == b.TaskID && a.Status == b.Status
}
func freezeBalanceEqual(a, b *cloud.Freeze) bool {
	return a.FreezeAmount == b.FreezeAmount && a.FreeFrozenAmount == b.FreeFrozenAmount &&
		a.SubscriptionFrozenAmount == b.SubscriptionFrozenAmount && a.LowBalanceMode == b.LowBalanceMode
}
func entitlementEqual(a, b *TaskEntitlementSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Plan == b.Plan && optionalStringEqual(a.EntitlementVersion, b.EntitlementVersion) && a.CapturedAt.Equal(b.CapturedAt)
}
func taskEqual(a, b ModelTask) bool {
	return taskIdentityEqual(a, b) && taskLifecycleEqual(a, b) &&
		optionalFreezeRequestEqual(a.FreezeRequest, b.FreezeRequest) && freezeEqual(a.Freeze, b.Freeze) &&
		entitlementEqual(a.Entitlement, b.Entitlement)
}
func taskIdentityEqual(a, b ModelTask) bool {
	return a.TaskID == b.TaskID && a.Scope.Equal(b.Scope) && a.BillingSource == b.BillingSource &&
		a.InitialModelID == b.InitialModelID && a.ClientTaskType == b.ClientTaskType &&
		a.ClientEstimatedCost == b.ClientEstimatedCost && a.PricingEvidenceID == b.PricingEvidenceID
}
func taskLifecycleEqual(a, b ModelTask) bool {
	return a.State == b.State && optionalTaskResultEqual(a.Result, b.Result) &&
		optionalTimeEqual(a.CreatedAt, b.CreatedAt) && optionalTimeEqual(a.UpdatedAt, b.UpdatedAt) &&
		optionalTimePointerEqual(a.StartedAt, b.StartedAt) && optionalTimePointerEqual(a.EndedAt, b.EndedAt) &&
		optionalStringEqual(a.LastErrorCode, b.LastErrorCode)
}
func optionalFreezeRequestEqual(a, b *cloud.FreezeRequest) bool {
	if a == nil || b == nil {
		return a == b
	}
	return freezeRequestEqual(*a, *b)
}
func optionalTimeEqual(a, b time.Time) bool { return a.Equal(b) }
func optionalTimePointerEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
func optionalTaskResultEqual(a, b *TaskResult) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
