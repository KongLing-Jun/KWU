package modelbilling

import (
	"context"
	"database/sql"
	"math/big"
	"strings"
)

type SubscriptionUsageCall struct {
	Call     ModelCall
	Evidence PricingEvidence
}

type SubscriptionUsageInput struct {
	Task  ModelTask
	Calls []SubscriptionUsageCall
}

type SubscriptionUsageReader interface {
	ReadUsageInput(context.Context, TaskScope, string) (SubscriptionUsageInput, error)
	ListUsagePending(context.Context, TaskScope) ([]ModelTask, error)
}

// SubscriptionUsagePendingIDScanner lists only persisted identities so one
// malformed task cannot prevent the coordinator from visiting later tasks.
type SubscriptionUsagePendingIDScanner interface {
	ListUsagePendingIDs(context.Context, TaskScope) ([]string, error)
}

type SQLiteSubscriptionUsageReader struct{ db *sql.DB }

func NewSQLiteSubscriptionUsageReader(db *sql.DB) *SQLiteSubscriptionUsageReader {
	return &SQLiteSubscriptionUsageReader{db: db}
}

func (r *SQLiteSubscriptionUsageReader) ReadUsageInput(ctx context.Context, scope TaskScope, taskID string) (SubscriptionUsageInput, error) {
	if r == nil || r.db == nil {
		return SubscriptionUsageInput{}, newError(ErrorContractInvalid, "subscription usage reader database is required")
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return SubscriptionUsageInput{}, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SubscriptionUsageInput{}, err
	}
	defer tx.Rollback()
	task, err := loadTask(ctx, tx, taskID)
	if err != nil {
		return SubscriptionUsageInput{}, taskStoreLoadError(err)
	}
	if !task.Scope.Equal(scope) {
		return SubscriptionUsageInput{}, newError(ErrorScopeMismatch, "task scope does not match persisted task")
	}
	if task.BillingSource != BillingSubscriptionPool {
		return SubscriptionUsageInput{}, newError(ErrorBillingSourceMismatch, "usage input requires a subscription task")
	}
	calls, err := loadSubscriptionUsageCalls(ctx, tx, task)
	if err != nil {
		return SubscriptionUsageInput{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubscriptionUsageInput{}, err
	}
	return SubscriptionUsageInput{Task: task, Calls: calls}, nil
}

func (r *SQLiteSubscriptionUsageReader) ListUsagePending(ctx context.Context, scope TaskScope) ([]ModelTask, error) {
	if r == nil || r.db == nil {
		return nil, newError(ErrorContractInvalid, "subscription usage reader database is required")
	}
	scopeKey, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	tasks, err := loadUsagePendingTasks(ctx, tx, scope, scopeKey)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *SQLiteSubscriptionUsageReader) ListUsagePendingIDs(ctx context.Context, scope TaskScope) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, newError(ErrorContractInvalid, "subscription usage reader database is required")
	}
	scopeKey, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT task_id FROM model_tasks
		WHERE scope_key=? AND billing_source=? AND state=?
		ORDER BY updated_at, task_id`, scopeKey, BillingSubscriptionPool, TaskUsagePending)
	if err != nil {
		return nil, err
	}
	return collectIDs(rows)
}

func loadUsagePendingTasks(ctx context.Context, tx *sql.Tx, scope TaskScope, scopeKey string) ([]ModelTask, error) {
	rows, err := tx.QueryContext(ctx, `SELECT task_id FROM model_tasks
		WHERE scope_key=? AND billing_source=? AND state=?
		ORDER BY updated_at, task_id`, scopeKey, BillingSubscriptionPool, TaskUsagePending)
	if err != nil {
		return nil, err
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	tasks := make([]ModelTask, 0, len(ids))
	for _, taskID := range ids {
		task, err := loadTask(ctx, tx, taskID)
		if err != nil {
			return nil, taskStoreLoadError(err)
		}
		if err := validateUsagePendingTask(scope, task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func validateUsagePendingTask(scope TaskScope, task ModelTask) error {
	if !task.Scope.Equal(scope) {
		return newError(ErrorScopeMismatch, "usage-pending task scope mismatch")
	}
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "usage-pending query returned a BYOK task")
	}
	if task.State != TaskUsagePending {
		return newError(ErrorContractInvalid, "usage-pending query returned an invalid task state")
	}
	return nil
}

func loadSubscriptionUsageCalls(ctx context.Context, tx *sql.Tx, task ModelTask) ([]SubscriptionUsageCall, error) {
	rows, err := tx.QueryContext(ctx, `SELECT call_id FROM model_calls WHERE task_id=? ORDER BY call_id`, task.TaskID)
	if err != nil {
		return nil, err
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	calls := make([]SubscriptionUsageCall, 0, len(ids))
	for _, callID := range ids {
		call, err := loadCall(ctx, tx, task, callID)
		if err != nil {
			return nil, taskStoreLoadError(err)
		}
		evidence, owner, err := loadPricingEvidence(ctx, tx, call.PricingEvidenceID)
		if err != nil {
			return nil, taskStoreLoadError(err)
		}
		if owner.TaskID != task.TaskID || !owner.Scope.Equal(task.Scope) {
			return nil, newError(ErrorScopeMismatch, "call pricing evidence does not belong to task scope")
		}
		if owner.BillingSource != task.BillingSource {
			return nil, newError(ErrorBillingSourceMismatch, "call pricing evidence billing source mismatch")
		}
		if err := evidence.ValidateForCall(task, call); err != nil {
			return nil, err
		}
		calls = append(calls, SubscriptionUsageCall{Call: call, Evidence: evidence})
	}
	return calls, nil
}

var _ SubscriptionUsageReader = (*SQLiteSubscriptionUsageReader)(nil)
var _ SubscriptionUsagePendingIDScanner = (*SQLiteSubscriptionUsageReader)(nil)

var forbiddenRawUsageCanonicalKeys = map[string]bool{
	"cookie": true, "setcookie": true, "bearer": true, "token": true, "key": true,
	"xapikey": true, "apikey": true, "credential": true, "secret": true, "password": true,
	"accesstoken": true, "refreshtoken": true, "apitoken": true, "authorization": true,
	"prompt": true, "messages": true, "content": true, "output": true, "completion": true,
}

func canonicalRawUsageKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	return strings.ReplaceAll(key, "-", "")
}

func containsForbiddenUsageKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if forbiddenRawUsageCanonicalKeys[canonicalRawUsageKey(key)] || containsForbiddenUsageKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsForbiddenUsageKey(child) {
				return true
			}
		}
	}
	return false
}

func (u NormalizedUsage) ValidateOptional() error {
	if u.BreakdownStatus != "" && !validUsageStatus(u.BreakdownStatus) {
		return newError(ErrorContractInvalid, "invalid usage breakdown status")
	}
	if err := validateUsageValues(u); err != nil {
		return err
	}
	return validateUsageAggregates(u)
}

func (u NormalizedUsage) ValidateForReport() error {
	if !validUsageStatus(u.BreakdownStatus) {
		return newError(ErrorContractInvalid, "report usage requires a valid breakdown status")
	}
	return u.ValidateOptional()
}

func validateUsageValues(usage NormalizedUsage) error {
	values := []*int64{
		usage.InputTokens, usage.NormalInputTokens, usage.CachedInputTokens, usage.CacheReadTokens,
		usage.CacheWriteTokens, usage.OutputTokens, usage.CachedOutputTokens, usage.ImageTokens,
		usage.AudioInputTokens, usage.AudioOutputTokens, usage.RequestCount, usage.ProviderTotalTokens,
	}
	for _, value := range values {
		if value != nil && *value < 0 {
			return newError(ErrorContractInvalid, "usage values cannot be negative")
		}
	}
	return nil
}

func validateUsageAggregates(usage NormalizedUsage) error {
	if completeInputSplit(usage) &&
		!usageSumMatches(*usage.InputTokens, *usage.NormalInputTokens, *usage.CacheReadTokens, *usage.CacheWriteTokens) {
		return newError(ErrorContractInvalid, "input token total conflicts with billing components")
	}
	if completeCachedInputSplit(usage) &&
		!usageSumMatches(*usage.CachedInputTokens, *usage.CacheReadTokens, *usage.CacheWriteTokens) {
		return newError(ErrorContractInvalid, "cached input total conflicts with cache components")
	}
	if usage.CachedOutputTokens != nil && usage.OutputTokens != nil && *usage.CachedOutputTokens > *usage.OutputTokens {
		return newError(ErrorContractInvalid, "cached output tokens exceed output total")
	}
	return nil
}

func completeInputSplit(usage NormalizedUsage) bool {
	return usage.InputTokens != nil && usage.NormalInputTokens != nil &&
		usage.CacheReadTokens != nil && usage.CacheWriteTokens != nil
}

func completeCachedInputSplit(usage NormalizedUsage) bool {
	return usage.CachedInputTokens != nil && usage.CacheReadTokens != nil && usage.CacheWriteTokens != nil
}

func usageSumMatches(total int64, parts ...int64) bool {
	sum := new(big.Int)
	for _, part := range parts {
		sum.Add(sum, big.NewInt(part))
	}
	return sum.Cmp(big.NewInt(total)) == 0
}

func hasUsageFact(usage NormalizedUsage) bool {
	values := []*int64{
		usage.InputTokens, usage.NormalInputTokens, usage.CachedInputTokens,
		usage.CacheReadTokens, usage.CacheWriteTokens, usage.OutputTokens,
		usage.CachedOutputTokens, usage.ImageTokens, usage.AudioInputTokens,
		usage.AudioOutputTokens, usage.RequestCount, usage.ProviderTotalTokens,
	}
	for _, value := range values {
		if value != nil {
			return true
		}
	}
	return false
}

type SubscriptionQuotaInput struct {
	Task  ModelTask
	Calls []ModelCall
}

type SubscriptionQuotaReader interface {
	ReadQuotaInput(context.Context, TaskScope, string) (SubscriptionQuotaInput, error)
}

type SQLiteSubscriptionQuotaReader struct{ db *sql.DB }

func NewSQLiteSubscriptionQuotaReader(db *sql.DB) *SQLiteSubscriptionQuotaReader {
	return &SQLiteSubscriptionQuotaReader{db: db}
}

func (r *SQLiteSubscriptionQuotaReader) ReadQuotaInput(
	ctx context.Context,
	scope TaskScope,
	taskID string,
) (SubscriptionQuotaInput, error) {
	if r == nil || r.db == nil {
		return SubscriptionQuotaInput{}, newError(ErrorContractInvalid, "subscription quota reader database is required")
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return SubscriptionQuotaInput{}, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SubscriptionQuotaInput{}, err
	}
	defer tx.Rollback()
	task, err := loadTask(ctx, tx, taskID)
	if err != nil {
		return SubscriptionQuotaInput{}, taskStoreLoadError(err)
	}
	if !task.Scope.Equal(scope) {
		return SubscriptionQuotaInput{}, newError(ErrorScopeMismatch, "task scope does not match quota input scope")
	}
	if task.BillingSource != BillingSubscriptionPool {
		return SubscriptionQuotaInput{}, newError(ErrorBillingSourceMismatch, "quota input requires a subscription task")
	}
	calls, err := loadSubscriptionQuotaCalls(ctx, tx, task)
	if err != nil {
		return SubscriptionQuotaInput{}, err
	}
	if err := tx.Commit(); err != nil {
		return SubscriptionQuotaInput{}, err
	}
	return SubscriptionQuotaInput{Task: task, Calls: calls}, nil
}

func loadSubscriptionQuotaCalls(ctx context.Context, tx *sql.Tx, task ModelTask) ([]ModelCall, error) {
	rows, err := tx.QueryContext(ctx, `SELECT call_id FROM model_calls WHERE task_id=? ORDER BY call_id`, task.TaskID)
	if err != nil {
		return nil, err
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return nil, err
	}
	calls := make([]ModelCall, 0, len(ids))
	for _, callID := range ids {
		call, err := loadCall(ctx, tx, task, callID)
		if err != nil {
			return nil, taskStoreLoadError(err)
		}
		if call.TaskID != task.TaskID || !call.Scope.Equal(task.Scope) {
			return nil, newError(ErrorScopeMismatch, "quota call does not belong to task scope")
		}
		if call.BillingSource != BillingSubscriptionPool {
			return nil, newError(ErrorBillingSourceMismatch, "quota reader returned a BYOK call")
		}
		calls = append(calls, call)
	}
	return calls, nil
}

// ListUsagePendingIDs follows the existing local lifecycle state name while
// keeping the active settlement scanner independent from token/pricing reads.
func (r *SQLiteSubscriptionQuotaReader) ListUsagePendingIDs(
	ctx context.Context,
	scope TaskScope,
) ([]string, error) {
	if r == nil || r.db == nil {
		return nil, newError(ErrorContractInvalid, "subscription quota reader database is required")
	}
	scopeKey, err := BuildTaskScopeKey(scope, BillingSubscriptionPool)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT task_id FROM model_tasks
		WHERE scope_key=? AND billing_source=? AND state=?
		ORDER BY updated_at,task_id`, scopeKey, BillingSubscriptionPool, TaskUsagePending)
	if err != nil {
		return nil, err
	}
	return collectIDs(rows)
}

var _ SubscriptionQuotaReader = (*SQLiteSubscriptionQuotaReader)(nil)
var _ SubscriptionUsagePendingIDScanner = (*SQLiteSubscriptionQuotaReader)(nil)
