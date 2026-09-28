package modelbilling

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/userctx"
)

func (s BillingSource) Validate() error {
	switch s {
	case BillingSubscriptionPool, BillingNewAPIByok:
		return nil
	default:
		return newError(ErrorContractInvalid, fmt.Sprintf("invalid billing source %q", s))
	}
}

func (r CallRole) Validate() error {
	switch r {
	case CallRoleMain, CallRolePlanner, CallRoleExecutor, CallRoleSubagent, CallRoleCompaction,
		CallRoleClassifier, CallRoleTasktreePlanner, CallRoleTitle, CallRoleGuardian:
		return nil
	default:
		return newError(ErrorContractInvalid, fmt.Sprintf("invalid call role %q", r))
	}
}

func (r TaskResult) Validate() error {
	switch r {
	case TaskResultCompleted, TaskResultFailed, TaskResultCancelled:
		return nil
	default:
		return newError(ErrorContractInvalid, fmt.Sprintf("invalid task result %q", r))
	}
}

func (m ReportMode) Validate() error {
	if m == ReportModeCumulative {
		return nil
	}
	return newError(ErrorContractInvalid, fmt.Sprintf("invalid report mode %q", m))
}

func (q QuotaType) Validate() error {
	switch q {
	case QuotaTypeToken, QuotaTypePerRequest:
		return nil
	default:
		return newError(ErrorContractInvalid, fmt.Sprintf("invalid quota type %q", q))
	}
}

func (m BillingMode) Validate() error {
	switch m {
	case BillingModeTokenRatio, BillingModePerRequest, BillingModeTieredExpr, BillingModeUnavailable:
		return nil
	default:
		return newError(ErrorContractInvalid, fmt.Sprintf("invalid billing mode %q", m))
	}
}

func (s TaskScope) Validate(source BillingSource) error {
	if err := source.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("actor_id", s.ActorID); err != nil {
		return err
	}
	if !userctx.ValidateProfileID(s.ProfileID) {
		return newError(ErrorContractInvalid, "invalid profile_id")
	}
	if source == BillingNewAPIByok {
		if s.CloudBinding != nil {
			return newError(ErrorBillingSourceMismatch, "BYOK scope cannot contain a cloud binding")
		}
		return nil
	}
	if s.CloudBinding == nil {
		return newError(ErrorScopeMismatch, "subscription scope requires a cloud binding")
	}
	if err := s.CloudBinding.Validate(); err != nil {
		return newError(ErrorScopeMismatch, err.Error())
	}
	if s.CloudBinding.ProfileID != s.ProfileID {
		return newError(ErrorScopeMismatch, "cloud binding profile does not match task profile")
	}
	return nil
}

func (s TaskScope) Equal(other TaskScope) bool {
	if s.ActorID != other.ActorID || s.ProfileID != other.ProfileID {
		return false
	}
	if s.CloudBinding == nil || other.CloudBinding == nil {
		return s.CloudBinding == nil && other.CloudBinding == nil
	}
	return *s.CloudBinding == *other.CloudBinding
}

func (t ModelTask) Validate() error {
	if err := t.validateCore(); err != nil {
		return err
	}
	if t.BillingSource == BillingNewAPIByok {
		return t.validateBYOKOwnership()
	}
	return validateSubscriptionTask(t)
}

func (t ModelTask) validateCore() error {
	if err := requireTrimmed("task_id", t.TaskID); err != nil {
		return err
	}
	if err := validateTaskStartFacts(t); err != nil {
		return err
	}
	if err := t.Scope.Validate(t.BillingSource); err != nil {
		return err
	}
	if !validTaskState(t.State) {
		return newError(ErrorContractInvalid, "invalid task state")
	}
	if err := validateTaskResult(t.BillingSource, t.State, t.Result); err != nil {
		return err
	}
	if err := requireTrimmed("pricing_evidence_id", t.PricingEvidenceID); err != nil {
		return err
	}
	return validateTaskLifecycleTimes(t)
}

func validateTaskLifecycleTimes(task ModelTask) error {
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return newError(ErrorContractInvalid, "task timestamps are required")
	}
	if task.UpdatedAt.Before(task.CreatedAt) {
		return newError(ErrorQuotaTimeInvalid, "task updated_at precedes created_at")
	}
	if task.StartedAt != nil && invalidTaskStartedAt(task) {
		return newError(ErrorQuotaTimeInvalid, "task started_at is outside its lifecycle interval")
	}
	return nil
}

func invalidTaskStartedAt(task ModelTask) bool {
	return task.StartedAt.IsZero() || task.StartedAt.Before(task.CreatedAt) || task.StartedAt.After(task.UpdatedAt) ||
		task.EndedAt != nil && task.EndedAt.Before(*task.StartedAt)
}

func validateTaskStartFacts(t ModelTask) error {
	if err := requireTrimmed("initial_model_id", t.InitialModelID); err != nil {
		return err
	}
	if err := requireTrimmed("client_task_type", t.ClientTaskType); err != nil {
		return err
	}
	return DecimalString(t.ClientEstimatedCost).Validate()
}

func validateTaskResult(source BillingSource, state TaskState, result *TaskResult) error {
	if result != nil {
		if err := result.Validate(); err != nil {
			return err
		}
	}
	if taskStateHasNoResult(state) {
		if result != nil {
			return newError(ErrorContractInvalid, "active task cannot have task_result")
		}
		return nil
	}
	return validateEndedTaskResult(source, state, result)
}

func taskStateHasNoResult(state TaskState) bool {
	return state == TaskCreated || state == TaskFreezePending || state == TaskReady || state == TaskRunning
}

func validateEndedTaskResult(source BillingSource, state TaskState, result *TaskResult) error {
	switch state {
	case TaskFailed:
		return requireTaskResult(result, TaskResultFailed)
	case TaskCancelled:
		return requireTaskResult(result, TaskResultCancelled)
	case TaskUsagePending, TaskSettling, TaskCompleted:
		if result == nil {
			return newError(ErrorContractInvalid, "ended task lifecycle requires task_result")
		}
		if state == TaskCompleted && source == BillingNewAPIByok && *result != TaskResultCompleted {
			return newError(ErrorContractInvalid, "completed BYOK task requires completed result")
		}
	case TaskRecoveryRequired:
		return nil
	}
	return nil
}

func requireTaskResult(result *TaskResult, expected TaskResult) error {
	if result == nil || *result != expected {
		return newError(ErrorContractInvalid, fmt.Sprintf("task state requires %s result", expected))
	}
	return nil
}

func ValidateTaskResultTransition(from, to *TaskResult) error {
	if to != nil {
		if err := to.Validate(); err != nil {
			return err
		}
	}
	if from == nil {
		return nil
	}
	if to == nil || *from != *to {
		return newError(ErrorInvalidStateTransition, "task_result is immutable once set")
	}
	return nil
}

func (t ModelTask) validateBYOKOwnership() error {
	if t.FreezeRequest != nil || t.Freeze != nil || t.Entitlement != nil {
		return newError(ErrorBillingSourceMismatch, "BYOK task cannot contain cloud freeze or entitlement data")
	}
	return nil
}

func validateSubscriptionTask(task ModelTask) error {
	if taskStateRequiresFreezeRequest(task.State) && task.FreezeRequest == nil {
		return newError(ErrorContractInvalid, "subscription task requires freeze request")
	}
	if task.FreezeRequest != nil {
		if err := validateTaskFreezeRequest(task); err != nil {
			return err
		}
	}
	if taskStateNeedsFreeze(task.State) {
		return validateTaskFreezeSnapshot(task)
	}
	if task.Freeze != nil {
		if task.FreezeRequest == nil {
			return newError(ErrorContractInvalid, "freeze snapshot requires freeze request")
		}
		if err := validateFreezeForTask(task); err != nil {
			return err
		}
	}
	if task.Entitlement != nil {
		return validateEntitlementSnapshot(task.Entitlement)
	}
	return nil
}

func validateTaskFreezeRequest(task ModelTask) error {
	if task.FreezeRequest == nil {
		return newError(ErrorContractInvalid, "subscription task requires freeze request")
	}
	if task.FreezeRequest != nil {
		if err := task.FreezeRequest.Validate(); err != nil {
			return newError(ErrorContractInvalid, err.Error())
		}
		if task.FreezeRequest.TaskID != task.TaskID || task.FreezeRequest.Scope != *task.Scope.CloudBinding {
			return newError(ErrorScopeMismatch, "freeze request does not match task identity")
		}
		if task.FreezeRequest.ModelID != task.InitialModelID ||
			task.FreezeRequest.ClientTaskType != task.ClientTaskType ||
			task.FreezeRequest.ClientEstimatedCost != task.ClientEstimatedCost {
			return newError(ErrorContractInvalid, "freeze request does not match task start facts")
		}
	}
	return nil
}

func validateTaskFreezeSnapshot(task ModelTask) error {
	if task.Freeze == nil || task.Entitlement == nil {
		return newError(ErrorContractInvalid, "ready subscription task requires freeze and entitlement snapshots")
	}
	if err := validateFreezeForTask(task); err != nil {
		return err
	}
	return validateEntitlementSnapshot(task.Entitlement)
}

func validateFreezeForTask(task ModelTask) error {
	if err := task.Freeze.Validate(); err != nil {
		return newError(ErrorContractInvalid, err.Error())
	}
	if task.Freeze.TaskID != task.TaskID || task.Freeze.Scope != *task.Scope.CloudBinding {
		return newError(ErrorScopeMismatch, "freeze result does not match task identity")
	}
	if (task.State == TaskReady || task.State == TaskRunning) && task.Freeze.Status != cloud.FreezeActive {
		return newError(ErrorContractInvalid, "ready or running task requires an active freeze")
	}
	return nil
}

func validateEntitlementSnapshot(entitlement *TaskEntitlementSnapshot) error {
	if err := requireTrimmed("entitlement plan", entitlement.Plan); err != nil || entitlement.CapturedAt.IsZero() {
		return newError(ErrorContractInvalid, "entitlement snapshot is incomplete")
	}
	if entitlement.EntitlementVersion != nil {
		if err := requireTrimmed("entitlement version", *entitlement.EntitlementVersion); err != nil {
			return err
		}
	}
	return nil
}

func taskStateNeedsFreeze(state TaskState) bool {
	switch state {
	case TaskReady, TaskRunning, TaskUsagePending, TaskSettling, TaskCompleted:
		return true
	default:
		return false
	}
}

func taskStateRequiresFreezeRequest(state TaskState) bool {
	switch state {
	case TaskFreezePending, TaskReady, TaskRunning, TaskUsagePending, TaskSettling,
		TaskCompleted, TaskRecoveryRequired:
		return true
	default:
		return false
	}
}

func (c ModelCall) ValidateForTask(task ModelTask) error {
	if err := validateModelCallIdentity(c, task); err != nil {
		return err
	}
	if !validCallState(c.State) || c.AttemptCount < 0 {
		return newError(ErrorContractInvalid, "invalid call state or attempt count")
	}
	if err := ValidateRawUsageRedacted(c.RawUsageRedacted); err != nil {
		return err
	}
	return validateModelCallUsageForState(c)
}

func validateModelCallIdentity(call ModelCall, task ModelTask) error {
	for name, value := range map[string]string{
		"call_id": call.CallID, "task_id": call.TaskID, "role": string(call.Role), "model_id": call.ModelID,
		"provider": call.Provider, "pricing_evidence_id": call.PricingEvidenceID,
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if err := call.Role.Validate(); err != nil {
		return err
	}
	if call.TaskID != task.TaskID || call.BillingSource != task.BillingSource {
		return newError(ErrorBillingSourceMismatch, "call must inherit task billing source")
	}
	if !call.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "call scope does not match task scope")
	}
	return nil
}

func validateModelCallUsageForState(call ModelCall) error {
	if call.State == CallCompleted {
		if !hasUsageFact(call.Usage) {
			return newError(ErrorContractInvalid, "completed call requires explicit usage facts")
		}
		return call.Usage.ValidateForReport()
	}
	if call.State == CallUsageMissing && call.Usage.BreakdownStatus != UsageMissing {
		return newError(ErrorContractInvalid, "usage_missing call requires usage_missing breakdown status")
	}
	return call.Usage.ValidateOptional()
}

func (r PendingReport) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return newError(ErrorScopeMismatch, err.Error())
	}
	for name, value := range map[string]string{
		"task_id": r.TaskID, "freeze_id": r.FreezeID, "idempotency_key": r.IdempotencyKey,
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if err := validateReportKindSequence(r.Kind, r.Sequence); err != nil {
		return err
	}
	expected, err := buildPendingReportIdempotencyKey(r)
	if err != nil {
		return err
	}
	if r.IdempotencyKey != expected {
		return newError(ErrorIdempotencyConflict, "pending report idempotency key does not match its identity")
	}
	return validatePendingReportPayload(r)
}

func buildPendingReportIdempotencyKey(report PendingReport) (string, error) {
	if report.Kind == ReportPartial {
		return BuildPartialIdempotencyKey(report.Scope, report.TaskID, report.FreezeID, report.Sequence)
	}
	return BuildFinalIdempotencyKey(report.Scope, report.TaskID, report.FreezeID)
}

func validateReportKindSequence(kind ReportKind, sequence uint64) error {
	if kind != ReportPartial && kind != ReportFinal {
		return newError(ErrorContractInvalid, "invalid report kind")
	}
	if (kind == ReportFinal && sequence != 0) || (kind == ReportPartial && sequence == 0) {
		return newError(ErrorContractInvalid, "partial sequence must be positive and final sequence must be zero")
	}
	return nil
}

func validatePendingReportPayload(report PendingReport) error {
	if !validReportState(report.State) || report.AttemptCount < 0 {
		return newError(ErrorContractInvalid, "invalid report state or attempt count")
	}
	if len(report.CanonicalPayload) == 0 || !json.Valid(report.CanonicalPayload) {
		return newError(ErrorContractInvalid, "canonical payload must be valid JSON")
	}
	if err := validatePendingReportHash(report); err != nil {
		return err
	}
	return validatePendingReportTimes(report)
}

func validatePendingReportHash(report PendingReport) error {
	switch report.ContractVersion {
	case QuotaContractV1:
		if report.PayloadHash == "" || report.PayloadHash != quotaPayloadHash(report.CanonicalPayload) {
			return newError(ErrorIdempotencyConflict, "quota report payload hash does not match canonical payload")
		}
	case LegacyTokenContractV1:
		if report.PayloadHash != "" && report.PayloadHash != quotaPayloadHash(report.CanonicalPayload) {
			return newError(ErrorIdempotencyConflict, "legacy report payload hash does not match canonical payload")
		}
	default:
		return newError(ErrorContractInvalid, "invalid report contract version")
	}
	return nil
}

func validatePendingReportTimes(report PendingReport) error {
	if report.CreatedAt.IsZero() || report.UpdatedAt.IsZero() {
		return newError(ErrorContractInvalid, "report timestamps are required")
	}
	requiresNextAttempt := report.State == ReportPending || report.State == ReportRetry || report.State == ReportDelivering
	if requiresNextAttempt && report.NextAttemptAt == nil {
		return newError(ErrorContractInvalid, "pending, retry, and delivering reports require next_attempt_at")
	}
	if !requiresNextAttempt && report.NextAttemptAt != nil {
		return newError(ErrorContractInvalid, "accepted and blocked reports cannot have next_attempt_at")
	}
	return nil
}

func (c CreditConfig) Validate() error {
	if err := c.CreditPerCNY.Validate(); err != nil {
		return err
	}
	if c.FetchedAt.IsZero() {
		return newError(ErrorContractInvalid, "credit config fetched_at is required")
	}
	if err := requireTrimmed("credit config pricing_group", c.PricingGroup); err != nil {
		return err
	}
	if c.Version != nil {
		if err := requireTrimmed("credit config version", *c.Version); err != nil {
			return err
		}
	}
	return nil
}

func (s PricingSourceStatus) Validate() error {
	if err := s.USDExchangeRate.Validate(); err != nil {
		return err
	}
	if err := s.RechargePrice.Validate(); err != nil {
		return err
	}
	if s.FetchedAt.IsZero() || (s.SourceVersion == nil && s.SourceHash == nil) {
		return newError(ErrorContractInvalid, "pricing status needs fetched_at and source version or hash")
	}
	return nil
}

func (p PricingSnapshot) Validate() error {
	if err := p.validateHeader(); err != nil {
		return err
	}
	if err := validateOptionalDecimals(p.ExchangeRate, p.RechargePrice); err != nil {
		return err
	}
	if err := validateGroupRatios(p.GroupRatios); err != nil {
		return err
	}
	for _, model := range p.Models {
		if err := model.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p PricingSnapshot) validateHeader() error {
	for name, value := range map[string]string{
		"snapshot_id": p.SnapshotID, "source": p.Source, "source_hash": p.SourceHash, "currency": p.Currency,
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if !validPricingState(p.State) || p.FetchedAt.IsZero() || p.FreshUntil.IsZero() || p.StaleUntil.IsZero() {
		return newError(ErrorContractInvalid, "pricing snapshot state and timestamps are required")
	}
	if p.FreshUntil.Before(p.FetchedAt) || p.StaleUntil.Before(p.FreshUntil) {
		return newError(ErrorContractInvalid, "pricing snapshot time bounds are invalid")
	}
	return nil
}

func validateOptionalDecimals(values ...*DecimalString) error {
	for _, value := range values {
		if value != nil {
			if err := value.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateGroupRatios(ratios map[string]DecimalString) error {
	for group, ratio := range ratios {
		if err := requireTrimmed("pricing group", group); err != nil {
			return err
		}
		if err := ratio.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p ModelPrice) Validate() error {
	for name, value := range map[string]string{
		"model_id": p.ModelID, "provider_model_id": p.ProviderModelID, "model_name": p.ModelName,
		"model_type": p.ModelType, "billing_mode": string(p.BillingMode),
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if err := p.QuotaType.Validate(); err != nil {
		return err
	}
	if err := p.BillingMode.Validate(); err != nil {
		return err
	}
	decimals := []*DecimalString{
		p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.CreateCacheRatio, p.ImageRatio,
		p.AudioRatio, p.AudioCompletionRatio, p.ModelPrice, p.InputPerMillion,
		p.CacheReadPerMillion, p.CacheWritePerMillion, p.OutputPerMillion,
		p.ImageInputPerMillion, p.AudioInputPerMillion, p.AudioOutputPerMillion, p.RequestPerUnit,
	}
	if err := validateOptionalDecimals(decimals...); err != nil {
		return err
	}
	return validateModelPriceMode(p, decimals)
}

func validateModelPriceMode(price ModelPrice, allPrices []*DecimalString) error {
	switch price.BillingMode {
	case BillingModeTokenRatio:
		return validateTokenRatioPrice(price)
	case BillingModePerRequest:
		return validatePerRequestPrice(price)
	case BillingModeTieredExpr:
		return validateTieredPrice(price)
	case BillingModeUnavailable:
		return validateUnavailablePrice(price, allPrices)
	}
	return nil
}

func validateTokenRatioPrice(price ModelPrice) error {
	if price.QuotaType != QuotaTypeToken || !hasTokenPriceBasis(price) {
		return newError(ErrorContractInvalid, "token_ratio requires token quota and token pricing basis")
	}
	return nil
}

func validatePerRequestPrice(price ModelPrice) error {
	if price.QuotaType != QuotaTypePerRequest || price.ModelPrice == nil {
		return newError(ErrorContractInvalid, "per_request requires per_request quota and model_price")
	}
	return nil
}

func validateTieredPrice(price ModelPrice) error {
	if price.QuotaType != QuotaTypeToken || price.BillingExpression == nil {
		return newError(ErrorContractInvalid, "tiered_expr requires token quota and billing_expression")
	}
	return requireTrimmed("billing_expression", *price.BillingExpression)
}

func validateUnavailablePrice(price ModelPrice, allPrices []*DecimalString) error {
	if hasDecimalValue(allPrices) || price.BillingExpression != nil {
		return newError(ErrorContractInvalid, "unavailable billing mode cannot contain estimable pricing")
	}
	return nil
}

func hasTokenPriceBasis(price ModelPrice) bool {
	return price.ModelRatio != nil || price.InputPerMillion != nil || price.CacheReadPerMillion != nil ||
		price.CacheWritePerMillion != nil || price.OutputPerMillion != nil || price.ImageInputPerMillion != nil ||
		price.AudioInputPerMillion != nil || price.AudioOutputPerMillion != nil
}

func hasDecimalValue(values []*DecimalString) bool {
	for _, value := range values {
		if value != nil {
			return true
		}
	}
	return false
}

func (r PartialUsageReport) Validate() error {
	if err := validateReportIdentity(r.Scope, r.TaskID, r.FreezeID, r.IdempotencyKey, r.CreatedAt); err != nil {
		return err
	}
	if r.Sequence == 0 {
		return newError(ErrorContractInvalid, "partial report requires positive sequence and cumulative mode")
	}
	if err := r.ReportMode.Validate(); err != nil {
		return err
	}
	expected, err := BuildPartialIdempotencyKey(r.Scope, r.TaskID, r.FreezeID, r.Sequence)
	if err != nil {
		return err
	}
	if r.IdempotencyKey != expected {
		return newError(ErrorIdempotencyConflict, "partial report idempotency key does not match its identity")
	}
	return validateModelCallUsage(r.Calls, true)
}

func (r FinalUsageReport) Validate() error {
	if err := validateReportIdentity(r.Scope, r.TaskID, r.FreezeID, r.IdempotencyKey, r.CreatedAt); err != nil {
		return err
	}
	if err := r.TaskResult.Validate(); err != nil {
		return err
	}
	if r.ClientSummary.ModelCallCount != len(r.Calls) {
		return newError(ErrorContractInvalid, "final call count is invalid")
	}
	if r.ClientSummary.StartedAt.IsZero() || r.ClientSummary.FinishedAt.IsZero() || r.ClientSummary.FinishedAt.Before(r.ClientSummary.StartedAt) {
		return newError(ErrorContractInvalid, "final usage summary timestamps are invalid")
	}
	expected, err := BuildFinalIdempotencyKey(r.Scope, r.TaskID, r.FreezeID)
	if err != nil {
		return err
	}
	if r.IdempotencyKey != expected {
		return newError(ErrorIdempotencyConflict, "final report idempotency key does not match its identity")
	}
	return validateModelCallUsage(r.Calls, true)
}

func (r FinalUsageReport) ValidateForTask(task ModelTask) error {
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "final usage report requires a subscription task")
	}
	if err := task.Validate(); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if task.Result == nil || *task.Result != r.TaskResult {
		return newError(ErrorContractInvalid, "final task_result does not match persisted task result")
	}
	return validateFinalReportTaskIdentity(r, task)
}

func validateFinalReportTaskIdentity(report FinalUsageReport, task ModelTask) error {
	if task.TaskID != report.TaskID || task.Scope.CloudBinding == nil || *task.Scope.CloudBinding != report.Scope {
		return newError(ErrorScopeMismatch, "final report does not match task identity")
	}
	if task.Freeze == nil || task.Freeze.FreezeID != report.FreezeID {
		return newError(ErrorScopeMismatch, "final report does not match task freeze")
	}
	return nil
}

func validateReportIdentity(scope cloud.BindingScope, taskID, freezeID, key string, createdAt time.Time) error {
	if err := scope.Validate(); err != nil {
		return newError(ErrorScopeMismatch, err.Error())
	}
	for name, value := range map[string]string{"task_id": taskID, "freeze_id": freezeID, "idempotency_key": key} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if createdAt.IsZero() {
		return newError(ErrorContractInvalid, "report created_at is required")
	}
	return nil
}

func validateModelCallUsage(calls []ModelCallUsage, required bool) error {
	if required && len(calls) == 0 {
		return newError(ErrorContractInvalid, "report calls are required")
	}
	seen := map[string]bool{}
	for _, call := range calls {
		if err := validateModelCallUsageIdentity(call); err != nil {
			return err
		}
		if seen[call.CallID] {
			return newError(ErrorContractInvalid, "report contains duplicate call_id")
		}
		seen[call.CallID] = true
		if err := validateModelCallUsageFacts(call); err != nil {
			return err
		}
	}
	return nil
}

func validateModelCallUsageIdentity(call ModelCallUsage) error {
	for name, value := range map[string]string{
		"call_id": call.CallID, "model_id": call.ModelID, "pricing_group": call.PricingGroup,
		"pricing_evidence_id": call.PricingEvidenceID,
	} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	return nil
}

func validateModelCallUsageFacts(call ModelCallUsage) error {
	if call.OccurredAt.IsZero() || call.OccurredAt.Location() != time.UTC {
		return newError(ErrorContractInvalid, "report call occurred_at is required in UTC")
	}
	if err := call.Usage.ValidateForReport(); err != nil {
		return err
	}
	return ValidateRawUsageRedacted(call.RawUsageRedacted)
}

func (s Settlement) Validate() error {
	if err := requireTrimmed("settlement_id", s.SettlementID); err != nil {
		return err
	}
	if err := requireTrimmed("task_id", s.TaskID); err != nil {
		return err
	}
	if !validSettlementState(s.State) || s.ObservedAt.IsZero() {
		return newError(ErrorContractInvalid, "invalid settlement state or observed_at")
	}
	decimals := []*DecimalString{
		s.ChargedAmount, s.ChargedFreeAmount, s.ChargedSubscriptionAmount,
		s.ReleasedFreeAmount, s.ReleasedSubscriptionAmount, s.UncoveredAmount,
	}
	if err := validateOptionalDecimals(decimals...); err != nil {
		return err
	}
	if s.State == SettlementException && s.ErrorCode == nil {
		return newError(ErrorContractInvalid, "exception settlement requires error_code")
	}
	return nil
}

func ValidateRawUsageRedacted(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return newError(ErrorContractInvalid, "raw usage must be valid JSON")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return newError(ErrorContractInvalid, "decode raw usage")
	}
	if containsForbiddenUsageKey(value) {
		return newError(ErrorContractInvalid, "raw usage contains model content or credentials")
	}
	return nil
}

func requireTrimmed(name, value string) error {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
		return newError(ErrorContractInvalid, name+" is required and must be trimmed")
	}
	return nil
}

func validTaskState(state TaskState) bool {
	switch state {
	case TaskCreated, TaskFreezePending, TaskReady, TaskRunning, TaskUsagePending,
		TaskSettling, TaskCompleted, TaskFailed, TaskCancelled, TaskRecoveryRequired:
		return true
	default:
		return false
	}
}

func validCallState(state CallState) bool {
	switch state {
	case CallPrepared, CallRunning, CallCompleted, CallFailed, CallCancelled, CallInterrupted, CallUsageMissing:
		return true
	default:
		return false
	}
}

func validUsageStatus(status UsageBreakdownStatus) bool {
	switch status {
	case UsageExact, UsagePartial, UsageUnavailable, UsageMissing:
		return true
	default:
		return false
	}
}

func validReportState(state ReportState) bool {
	switch state {
	case ReportPending, ReportDelivering, ReportAccepted, ReportRetry, ReportBlocked:
		return true
	default:
		return false
	}
}

func validPricingState(state PricingState) bool {
	switch state {
	case PricingFresh, PricingStaleUsable, PricingStaleBlocked, PricingUnavailable:
		return true
	default:
		return false
	}
}

func validSettlementState(state SettlementState) bool {
	switch state {
	case SettlementPending, SettlementPricingPending, SettlementSettled, SettlementException:
		return true
	default:
		return false
	}
}
