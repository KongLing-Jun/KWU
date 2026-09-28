package modelbilling

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type QuotaSettlementSnapshot struct {
	TaskID      string
	Revision    uint64
	Result      QuotaSettlementResult
	PayloadHash string
	ObservedAt  time.Time
}

type SQLiteQuotaSettlementStore struct{ db *sql.DB }

// CurrentQuotaSettlements reads a bounded batch in one SQLite snapshot. A nil
// value means an owned task has no settlement yet; absent keys are unavailable.
func (s *SQLiteQuotaSettlementStore) CurrentQuotaSettlements(ctx context.Context, scope TaskScope, ids []string) (map[string]*QuotaSettlementResult, error) {
	if s == nil || s.db == nil || len(ids) == 0 || len(ids) > 64 {
		return nil, newError(ErrorContractInvalid, "invalid settlement batch")
	}
	if err := scope.Validate(BillingSubscriptionPool); err != nil {
		return nil, err
	}
	args := []any{scope.ActorID, scope.ProfileID, scope.CloudBinding.CloudUserID, scope.CloudBinding.BindingGeneration, string(BillingSubscriptionPool)}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.task_id,
		COALESCE(r.settlement_revision,0),COALESCE(r.settlement_id,''),COALESCE(r.status,''),
		COALESCE(r.normalized_payload,''),COALESCE(r.payload_hash,''),COALESCE(r.observed_at,'')
		FROM model_tasks t LEFT JOIN model_task_quota_settlement_current r ON t.task_id=r.task_id
		WHERE t.actor_id=? AND t.profile_id=? AND t.cloud_user_id=? AND t.binding_generation=? AND t.billing_source=?
		AND t.task_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readQuotaSettlementBatch(rows)
}

func readQuotaSettlementBatch(rows *sql.Rows) (map[string]*QuotaSettlementResult, error) {
	results := make(map[string]*QuotaSettlementResult)
	for rows.Next() {
		var id string
		var fields quotaSettlementRowFields
		if err := rows.Scan(&id, &fields.revision, &fields.settlementID, &fields.status, &fields.payload, &fields.payloadHash, &fields.observedAt); err != nil {
			return nil, err
		}
		if fields.revision == 0 {
			results[id] = nil
			continue
		}
		result, err := decodeQuotaSettlementResult(fields.payload, fields.payloadHash)
		if err != nil {
			return nil, err
		}
		if err := validateQuotaSettlementRowIdentity(fields, id, result); err != nil {
			return nil, err
		}
		results[id] = &result
	}
	return results, rows.Err()
}

func NewSQLiteQuotaSettlementStore(db *sql.DB) *SQLiteQuotaSettlementStore {
	return &SQLiteQuotaSettlementStore{db: db}
}

func (s *SQLiteQuotaSettlementStore) SaveQuotaSettlement(
	ctx context.Context,
	scope TaskScope,
	result QuotaSettlementResult,
	at time.Time,
) error {
	payload, payloadHash, err := prepareQuotaSettlementSave(s, scope, result, at)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveQuotaSettlementInTx(ctx, tx, scope, result, at, payload, payloadHash); err != nil {
		return err
	}
	return tx.Commit()
}

func prepareQuotaSettlementSave(
	store *SQLiteQuotaSettlementStore,
	scope TaskScope,
	result QuotaSettlementResult,
	at time.Time,
) (json.RawMessage, string, error) {
	if store == nil || store.db == nil || at.IsZero() {
		return nil, "", newError(ErrorContractInvalid, "quota settlement store and observation time are required")
	}
	if err := scope.Validate(BillingSubscriptionPool); err != nil {
		return nil, "", err
	}
	if err := result.Validate(); err != nil {
		return nil, "", err
	}
	payload, err := CanonicalJSON(result)
	if err != nil {
		return nil, "", err
	}
	payloadHash, err := QuotaPayloadHash(payload)
	if err != nil {
		return nil, "", err
	}
	return payload, payloadHash, nil
}

func saveQuotaSettlementInTx(
	ctx context.Context,
	tx *sql.Tx,
	scope TaskScope,
	result QuotaSettlementResult,
	at time.Time,
	payload json.RawMessage,
	payloadHash string,
) error {
	task, err := loadTask(ctx, tx, result.Settlement.TaskID)
	if err != nil {
		return taskStoreLoadError(err)
	}
	if err := validateQuotaSettlementSaveTask(task, scope, result); err != nil {
		return err
	}
	current, found, err := loadCurrentQuotaSettlement(ctx, tx, task.TaskID)
	if err != nil {
		return err
	}
	if found {
		retry, err := validateCurrentQuotaSettlement(current, result, at, payload, payloadHash)
		if err != nil {
			return err
		}
		if retry {
			return nil
		}
	}
	revision := uint64(1)
	if found {
		revision = current.Revision + 1
	}
	return insertQuotaSettlementRevision(ctx, tx, task.TaskID, revision, result, at, payload, payloadHash)
}

func validateQuotaSettlementSaveTask(task ModelTask, scope TaskScope, result QuotaSettlementResult) error {
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "quota settlement requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return newError(ErrorScopeMismatch, "quota settlement task scope mismatch")
	}
	return result.Settlement.ValidateForTask(task)
}

func validateCurrentQuotaSettlement(
	current quotaSettlementSnapshot,
	result QuotaSettlementResult,
	at time.Time,
	payload json.RawMessage,
	payloadHash string,
) (bool, error) {
	if bytes.Equal(current.ResultPayload(), payload) && current.PayloadHash == payloadHash {
		return true, nil
	}
	if at.UTC().Before(current.ObservedAt) {
		return false, newError(ErrorInvalidStateTransition, "quota settlement observation time cannot move backwards")
	}
	if current.Result.Settlement.SettlementID != result.Settlement.SettlementID ||
		current.Result.Settlement.Status != QuotaSettlementPending {
		return false, newError(ErrorIdempotencyConflict, "quota settlement retry changed an accepted snapshot")
	}
	if err := validateQuotaSettlementBalanceAdvance(current.Result, result); err != nil {
		return false, err
	}
	return false, nil
}

func insertQuotaSettlementRevision(
	ctx context.Context,
	tx *sql.Tx,
	taskID string,
	revision uint64,
	result QuotaSettlementResult,
	at time.Time,
	payload json.RawMessage,
	payloadHash string,
) error {
	insert, err := tx.ExecContext(ctx, `INSERT INTO model_task_quota_settlement_revisions
		(task_id,settlement_revision,settlement_id,status,normalized_payload,payload_hash,observed_at)
		VALUES (?,?,?,?,?,?,?)`, taskID, revision, result.Settlement.SettlementID,
		result.Settlement.Status, []byte(payload), payloadHash, stamp(at.UTC()))
	if err != nil {
		return err
	}
	if err := exactlyOne(insert, "append quota settlement revision"); err != nil {
		return err
	}
	return nil
}

func validateQuotaSettlementBalanceAdvance(current, next QuotaSettlementResult) error {
	currentVersion, currentConfirmed := confirmedQuotaSettlementBalanceVersion(current)
	if !currentConfirmed {
		return nil
	}
	nextVersion, nextConfirmed := confirmedQuotaSettlementBalanceVersion(next)
	if !nextConfirmed {
		return newError(ErrorIdempotencyConflict, "quota settlement update omitted a confirmed balance")
	}
	if nextVersion <= currentVersion {
		return newError(ErrorIdempotencyConflict, "quota settlement balance_version did not advance")
	}
	return nil
}

func confirmedQuotaSettlementBalanceVersion(result QuotaSettlementResult) (uint64, bool) {
	if result.Balance != nil {
		return result.Balance.Version, true
	}
	if result.Settlement.Balance != nil && result.Settlement.BalanceVersion != nil {
		return *result.Settlement.BalanceVersion, true
	}
	return 0, false
}

func (s *SQLiteQuotaSettlementStore) CurrentQuotaSettlement(
	ctx context.Context,
	scope TaskScope,
	taskID string,
) (QuotaSettlementResult, bool, error) {
	if s == nil || s.db == nil {
		return QuotaSettlementResult{}, false, newError(ErrorContractInvalid, "quota settlement store database is required")
	}
	if err := scope.Validate(BillingSubscriptionPool); err != nil {
		return QuotaSettlementResult{}, false, err
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return QuotaSettlementResult{}, false, taskStoreLoadError(err)
	}
	if task.BillingSource != BillingSubscriptionPool {
		return QuotaSettlementResult{}, false, newError(ErrorBillingSourceMismatch, "quota settlement requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return QuotaSettlementResult{}, false, newError(ErrorScopeMismatch, "quota settlement task scope mismatch")
	}
	snapshot, found, err := loadCurrentQuotaSettlement(ctx, s.db, taskID)
	if err != nil || !found {
		return QuotaSettlementResult{}, found, err
	}
	return snapshot.Result, true, nil
}

func (s *SQLiteQuotaSettlementStore) ListQuotaSettlements(
	ctx context.Context,
	scope TaskScope,
	taskID string,
) ([]QuotaSettlementSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, newError(ErrorContractInvalid, "quota settlement store database is required")
	}
	task, err := loadTask(ctx, s.db, taskID)
	if err != nil {
		return nil, taskStoreLoadError(err)
	}
	if task.BillingSource != BillingSubscriptionPool {
		return nil, newError(ErrorBillingSourceMismatch, "quota settlement requires a subscription task")
	}
	if !task.Scope.Equal(scope) {
		return nil, newError(ErrorScopeMismatch, "quota settlement task scope mismatch")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT settlement_revision,settlement_id,status,normalized_payload,payload_hash,observed_at
		FROM model_task_quota_settlement_revisions WHERE task_id=? ORDER BY settlement_revision`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snapshots []QuotaSettlementSnapshot
	for rows.Next() {
		snapshot, err := scanQuotaSettlementSnapshot(rows, taskID)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (s *SQLiteQuotaWorkflowStore) SaveQuotaSettlement(
	ctx context.Context,
	scope TaskScope,
	result QuotaSettlementResult,
	at time.Time,
) error {
	if s == nil {
		return newError(ErrorContractInvalid, "quota workflow store database is required")
	}
	return NewSQLiteQuotaSettlementStore(s.db).SaveQuotaSettlement(ctx, scope, result, at)
}

func (s *SQLiteQuotaWorkflowStore) CurrentQuotaSettlement(
	ctx context.Context,
	scope TaskScope,
	taskID string,
) (QuotaSettlementResult, bool, error) {
	if s == nil {
		return QuotaSettlementResult{}, false, newError(ErrorContractInvalid, "quota workflow store database is required")
	}
	return NewSQLiteQuotaSettlementStore(s.db).CurrentQuotaSettlement(ctx, scope, taskID)
}

type quotaSettlementScanner interface {
	Scan(...any) error
}

type quotaSettlementSnapshot struct {
	QuotaSettlementSnapshot
	payload json.RawMessage
}

func (s quotaSettlementSnapshot) ResultPayload() json.RawMessage {
	return append(json.RawMessage(nil), s.payload...)
}

func loadCurrentQuotaSettlement(
	ctx context.Context,
	q taskQuerier,
	taskID string,
) (quotaSettlementSnapshot, bool, error) {
	row := q.QueryRowContext(ctx, `SELECT settlement_revision,settlement_id,status,normalized_payload,payload_hash,observed_at
		FROM model_task_quota_settlement_current WHERE task_id=?`, taskID)
	snapshot, err := scanQuotaSettlementSnapshot(row, taskID)
	if err == sql.ErrNoRows {
		return quotaSettlementSnapshot{}, false, nil
	}
	if err != nil {
		return quotaSettlementSnapshot{}, false, err
	}
	payload, err := CanonicalJSON(snapshot.Result)
	if err != nil {
		return quotaSettlementSnapshot{}, false, err
	}
	return quotaSettlementSnapshot{QuotaSettlementSnapshot: snapshot, payload: payload}, true, nil
}

type quotaSettlementRowFields struct {
	revision                          uint64
	settlementID, status, payloadHash string
	observedAt                        string
	payload                           []byte
}

func scanQuotaSettlementSnapshot(scanner quotaSettlementScanner, taskID string) (QuotaSettlementSnapshot, error) {
	fields, err := scanQuotaSettlementFields(scanner)
	if err != nil {
		return QuotaSettlementSnapshot{}, err
	}
	result, err := decodeQuotaSettlementResult(fields.payload, fields.payloadHash)
	if err != nil {
		return QuotaSettlementSnapshot{}, err
	}
	if err := validateQuotaSettlementRowIdentity(fields, taskID, result); err != nil {
		return QuotaSettlementSnapshot{}, err
	}
	observed, err := parseStamp(fields.observedAt)
	if err != nil {
		return QuotaSettlementSnapshot{}, persistedFieldError("quota settlement observed_at", err)
	}
	return QuotaSettlementSnapshot{
		TaskID: taskID, Revision: fields.revision, Result: result,
		PayloadHash: fields.payloadHash, ObservedAt: observed,
	}, nil
}

func scanQuotaSettlementFields(scanner quotaSettlementScanner) (quotaSettlementRowFields, error) {
	var fields quotaSettlementRowFields
	if err := scanner.Scan(
		&fields.revision, &fields.settlementID, &fields.status,
		&fields.payload, &fields.payloadHash, &fields.observedAt,
	); err != nil {
		return quotaSettlementRowFields{}, err
	}
	return fields, nil
}

func decodeQuotaSettlementResult(payload []byte, payloadHash string) (QuotaSettlementResult, error) {
	if len(payload) == 0 || !json.Valid(payload) || quotaPayloadHash(payload) != payloadHash {
		return QuotaSettlementResult{}, newError(ErrorIdempotencyConflict, "persisted quota settlement payload/hash is invalid")
	}
	var result QuotaSettlementResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return QuotaSettlementResult{}, newError(ErrorContractInvalid, "decode persisted quota settlement")
	}
	canonical, err := CanonicalJSON(result)
	if err != nil || !bytes.Equal(canonical, payload) {
		return QuotaSettlementResult{}, newError(ErrorIdempotencyConflict, "persisted quota settlement payload is not canonical")
	}
	if err := result.Validate(); err != nil {
		return QuotaSettlementResult{}, err
	}
	return result, nil
}

func validateQuotaSettlementRowIdentity(
	fields quotaSettlementRowFields,
	taskID string,
	result QuotaSettlementResult,
) error {
	if result.Settlement.TaskID != taskID || result.Settlement.SettlementID != fields.settlementID ||
		result.Settlement.Status != QuotaSettlementState(fields.status) || fields.revision == 0 {
		return newError(ErrorIdempotencyConflict, "persisted quota settlement identity is inconsistent")
	}
	return nil
}
