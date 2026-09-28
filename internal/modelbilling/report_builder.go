package modelbilling

import (
	"context"
	"encoding/json"
	"time"
)

type ReportBuildDisposition string

const (
	ReportBuildDeliverable      ReportBuildDisposition = "deliverable"
	ReportBuildNoReportRequired ReportBuildDisposition = "no_report_required"
	ReportBuildEvidencePending  ReportBuildDisposition = "evidence_pending"
	// ReportBuildBlocked remains for persisted legacy reports. The quota
	// builder never produces a blocked payload.
	ReportBuildBlocked ReportBuildDisposition = "blocked"
)

// ReportBlockCode is retained for loading legacy token reports. Maas quota
// construction uses ReportBuildEvidencePending instead of token-based gates.
type ReportBlockCode string

const (
	ReportBlockUsageNotExact           ReportBlockCode = "usage_not_exact"
	ReportBlockRequestCountMissing     ReportBlockCode = "request_count_missing"
	ReportBlockRequestCountNotPositive ReportBlockCode = "request_count_not_positive"
	ReportBlockUsageInvalid            ReportBlockCode = "usage_invalid"
	ReportBlockCallLimitExceeded       ReportBlockCode = "call_limit_exceeded"
)

// UsageReportBuildResult keeps its exported name so callers can migrate
// without changing the orchestration boundary. Its active payload is quota.
type UsageReportBuildResult struct {
	Disposition      ReportBuildDisposition
	Partial          *PartialQuotaReport
	Final            *FinalQuotaReport
	CanonicalPayload json.RawMessage
	BlockedCode      ReportBlockCode
	BlockedReason    string
}

// UsageReportBuilder keeps the old exported constructor name, but reads only
// the subscription quota projection.
type UsageReportBuilder struct {
	reader SubscriptionQuotaReader
}

func NewUsageReportBuilder(reader SubscriptionQuotaReader) *UsageReportBuilder {
	return &UsageReportBuilder{reader: reader}
}

func (b *UsageReportBuilder) BuildFinal(ctx context.Context, scope TaskScope, taskID string) (UsageReportBuildResult, error) {
	if b == nil || b.reader == nil {
		return UsageReportBuildResult{}, newError(ErrorContractInvalid, "quota report reader is required")
	}
	if err := scope.Validate(BillingSubscriptionPool); err != nil {
		return UsageReportBuildResult{}, err
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return UsageReportBuildResult{}, err
	}
	input, err := b.reader.ReadQuotaInput(ctx, scope, taskID)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	if input.Task.TaskID != taskID || !input.Task.Scope.Equal(scope) {
		return UsageReportBuildResult{}, newError(ErrorScopeMismatch, "quota input does not match requested task scope")
	}
	return buildFinalQuotaReport(input)
}

func buildFinalQuotaReport(input SubscriptionQuotaInput) (UsageReportBuildResult, error) {
	task := input.Task
	if err := validateReportTask(task, ReportFinal); err != nil {
		return UsageReportBuildResult{}, err
	}
	calls, evidencePending, err := buildPersistedQuotaCalls(task, input.Calls, false)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	if len(calls) == 0 {
		return UsageReportBuildResult{Disposition: ReportBuildNoReportRequired}, nil
	}
	if evidencePending {
		return UsageReportBuildResult{Disposition: ReportBuildEvidencePending}, nil
	}
	total, err := SumQuotaReportCalls(calls)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	key, err := BuildFinalIdempotencyKey(*task.Scope.CloudBinding, task.TaskID, task.Freeze.FreezeID)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	endedAt := task.EndedAt.UTC()
	report := FinalQuotaReport{
		ContractVersion: QuotaContractV1,
		TaskID:          task.TaskID,
		FreezeID:        task.Freeze.FreezeID,
		ClientSummary: QuotaTaskSummary{
			TaskTotalQuota: total,
			ModelCallCount: len(calls),
			StartedAt:      task.StartedAt.UTC(),
			FinishedAt:     endedAt,
		},
		Calls:          calls,
		Scope:          *task.Scope.CloudBinding,
		IdempotencyKey: key,
		TaskResult:     *task.Result,
		CreatedAt:      endedAt,
	}
	if err := validateBuiltFinalQuotaReport(report, task); err != nil {
		return UsageReportBuildResult{}, err
	}
	payload, err := CanonicalJSON(report)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	return UsageReportBuildResult{
		Disposition:      ReportBuildDeliverable,
		Final:            &report,
		CanonicalPayload: payload,
	}, nil
}

// A failed stream with no request identity or metering facts cannot be
// supplemented. Keep the attempt in the local ledger, but do not let it hold
// up settlement of the task's metered calls. Successful calls and attempts
// with any billing evidence still require complete quota evidence.
func terminalCallWithoutBillingEvidence(call ModelCall) bool {
	switch call.State {
	case CallFailed, CallInterrupted, CallCancelled:
	default:
		return false
	}
	return call.ProviderRequestID == nil &&
		len(call.RawUsageRedacted) == 0 &&
		!hasUsageFact(call.Usage) &&
		call.QuotaEvidence != nil &&
		call.QuotaEvidence.OneAPIRequestID == nil &&
		!call.QuotaEvidence.BillingQuotaRaw.Present &&
		!call.QuotaEvidence.UsageQuotaRaw.Present &&
		call.QuotaEvidence.Status == QuotaEvidenceMissing
}

// buildPartialQuotaReport is package-private so the SQLite report store can
// allocate sequence, build the cumulative snapshot, and enqueue atomically.
func buildPartialQuotaReport(input SubscriptionQuotaInput, sequence uint64, capturedAt time.Time) (UsageReportBuildResult, error) {
	if sequence == 0 {
		return UsageReportBuildResult{}, newError(ErrorContractInvalid, "partial sequence must be positive")
	}
	task := input.Task
	if err := validateReportTask(task, ReportPartial); err != nil {
		return UsageReportBuildResult{}, err
	}
	calls, _, err := buildPersistedQuotaCalls(task, input.Calls, true)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	if err := validatePartialCaptureTime(task, input.Calls, capturedAt); err != nil {
		return UsageReportBuildResult{}, err
	}
	if len(calls) == 0 {
		return UsageReportBuildResult{Disposition: ReportBuildNoReportRequired}, nil
	}
	key, err := BuildPartialIdempotencyKey(*task.Scope.CloudBinding, task.TaskID, task.Freeze.FreezeID, sequence)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	report := PartialQuotaReport{
		ContractVersion: QuotaContractV1,
		TaskID:          task.TaskID,
		FreezeID:        task.Freeze.FreezeID,
		Sequence:        sequence,
		ReportMode:      ReportModeCumulative,
		CapturedAt:      capturedAt,
		Calls:           calls,
		Scope:           *task.Scope.CloudBinding,
		IdempotencyKey:  key,
	}
	if err := report.Validate(); err != nil {
		return UsageReportBuildResult{}, err
	}
	payload, err := CanonicalJSON(report)
	if err != nil {
		return UsageReportBuildResult{}, err
	}
	return UsageReportBuildResult{
		Disposition:      ReportBuildDeliverable,
		Partial:          &report,
		CanonicalPayload: payload,
	}, nil
}

func validateReportTask(task ModelTask, kind ReportKind) error {
	if err := validateReportTaskBase(task); err != nil {
		return err
	}
	switch kind {
	case ReportFinal:
		return validateFinalReportTask(task)
	case ReportPartial:
		return validatePartialReportTask(task)
	default:
		return newError(ErrorContractInvalid, "invalid quota report kind")
	}
}

func validateReportTaskBase(task ModelTask) error {
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "quota report requires a subscription task")
	}
	if err := requireTrimmed("task_id", task.TaskID); err != nil {
		return err
	}
	if err := task.Scope.Validate(BillingSubscriptionPool); err != nil {
		return err
	}
	if task.Freeze == nil {
		return newError(ErrorContractInvalid, "quota report requires persisted freeze facts")
	}
	if err := task.Freeze.Validate(); err != nil {
		return newError(ErrorContractInvalid, err.Error())
	}
	if !quotaReportFreezeMatchesTask(task) {
		return newError(ErrorScopeMismatch, "quota report freeze does not match task identity")
	}
	if task.CreatedAt.IsZero() {
		return newError(ErrorQuotaTimeInvalid, "quota report task requires created_at")
	}
	if invalidQuotaReportTaskStart(task) {
		return newError(ErrorQuotaTimeInvalid, "quota report task requires a proven UTC started_at")
	}
	return nil
}

func quotaReportFreezeMatchesTask(task ModelTask) bool {
	return task.Freeze.TaskID == task.TaskID && task.Freeze.Scope == *task.Scope.CloudBinding
}

func invalidQuotaReportTaskStart(task ModelTask) bool {
	return task.StartedAt == nil || task.StartedAt.IsZero() || task.StartedAt.Location() != time.UTC ||
		task.StartedAt.Before(task.CreatedAt) || task.StartedAt.After(task.UpdatedAt)
}

func validateFinalReportTask(task ModelTask) error {
	if task.State != TaskUsagePending {
		return newError(ErrorContractInvalid, "final quota report requires a usage_pending task")
	}
	if task.Result == nil {
		return newError(ErrorContractInvalid, "final quota report requires task_result")
	}
	if err := task.Result.Validate(); err != nil {
		return err
	}
	if task.EndedAt == nil || task.EndedAt.IsZero() || task.EndedAt.Before(*task.StartedAt) {
		return newError(ErrorQuotaTimeInvalid, "final quota task timestamps are invalid")
	}
	return nil
}

func validatePartialReportTask(task ModelTask) error {
	if task.State != TaskRunning && task.State != TaskUsagePending {
		return newError(ErrorContractInvalid, "partial quota report requires a running or usage_pending task")
	}
	if task.State == TaskUsagePending {
		return validateFinalReportTask(task)
	}
	return nil
}

func buildPersistedQuotaCalls(task ModelTask, input []ModelCall, skipInFlight bool) ([]QuotaReportCall, bool, error) {
	items := make([]QuotaReportCall, 0, len(input))
	for _, call := range input {
		item, include, err := persistedQuotaReportCall(task, call, skipInFlight)
		if err != nil {
			return nil, false, err
		}
		if include {
			items = append(items, item)
		}
	}
	canonical, err := CanonicalizeQuotaReportCalls(items)
	if err != nil {
		return nil, false, err
	}
	return canonical, quotaReportCallsEvidencePending(canonical), nil
}

func persistedQuotaReportCall(task ModelTask, call ModelCall, skipInFlight bool) (QuotaReportCall, bool, error) {
	if err := validatePersistedQuotaCallIdentity(task, call); err != nil {
		return QuotaReportCall{}, false, err
	}
	if call.State == CallPrepared || call.State == CallRunning {
		if skipInFlight {
			return QuotaReportCall{}, false, nil
		}
		return QuotaReportCall{}, false, newError(ErrorContractInvalid, "final quota report rejects an in-flight call")
	}
	item, err := quotaReportCallFromPersisted(task, call)
	if err == nil && terminalCallWithoutBillingEvidence(call) {
		return QuotaReportCall{}, false, nil
	}
	return item, err == nil, err
}

func quotaReportCallsEvidencePending(calls []QuotaReportCall) bool {
	for _, call := range calls {
		if call.EvidenceStatus != QuotaEvidenceComplete {
			return true
		}
	}
	return false
}

func validatePartialCaptureTime(task ModelTask, calls []ModelCall, capturedAt time.Time) error {
	if partialCapturePrecedesTask(task, capturedAt) {
		return newError(ErrorQuotaTimeInvalid, "partial captured_at precedes persisted task facts")
	}
	for _, call := range calls {
		if quotaCallInFlight(call) {
			continue
		}
		if partialCapturePrecedesCall(call, capturedAt) {
			return newError(ErrorQuotaTimeInvalid, "partial captured_at precedes call or quota evidence facts")
		}
	}
	return nil
}

func partialCapturePrecedesTask(task ModelTask, capturedAt time.Time) bool {
	return capturedAt.IsZero() || capturedAt.Location() != time.UTC || capturedAt.Before(task.UpdatedAt) ||
		task.EndedAt != nil && capturedAt.Before(*task.EndedAt)
}

func quotaCallInFlight(call ModelCall) bool {
	return call.State == CallPrepared || call.State == CallRunning
}

func partialCapturePrecedesCall(call ModelCall, capturedAt time.Time) bool {
	return call.FinishedAt == nil || capturedAt.Before(*call.FinishedAt) ||
		call.QuotaEvidence != nil && capturedAt.Before(call.QuotaEvidence.ObservedAt)
}

func validatePersistedQuotaCallIdentity(task ModelTask, call ModelCall) error {
	if err := requireTrimmed("call_id", call.CallID); err != nil {
		return err
	}
	if call.TaskID != task.TaskID || call.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "quota call must belong to the subscription task")
	}
	if !call.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "quota call scope does not match task scope")
	}
	return nil
}

func quotaReportCallFromPersisted(task ModelTask, call ModelCall) (QuotaReportCall, error) {
	if err := validateQuotaReportCallFacts(task, call); err != nil {
		return QuotaReportCall{}, err
	}
	return quotaReportCallFromEvidence(call), nil
}

func validateQuotaReportCallFacts(task ModelTask, call ModelCall) error {
	if !terminalCallState(call.State) || call.AttemptCount <= 0 {
		return newError(ErrorContractInvalid, "actual quota call must be terminal with a positive attempt count")
	}
	if err := requireTrimmed("model", call.ModelID); err != nil {
		return err
	}
	if err := call.Endpoint.Validate(); err != nil {
		return err
	}
	if invalidActualQuotaCallTimes(call) {
		return newError(ErrorQuotaTimeInvalid, "actual quota call timestamps are invalid")
	}
	if invalidQuotaCallOccurrence(call) {
		return newError(ErrorQuotaTimeInvalid, "quota occurred_at is outside its call interval")
	}
	if quotaCallOutsideTask(task, call) {
		return newError(ErrorQuotaTimeInvalid, "quota call is outside its task interval")
	}
	if call.QuotaEvidence == nil {
		return newError(ErrorContractInvalid, "actual subscription call requires quota evidence")
	}
	if err := call.QuotaEvidence.Validate(); err != nil {
		return err
	}
	return nil
}

func invalidActualQuotaCallTimes(call ModelCall) bool {
	return call.StartedAt == nil || call.FinishedAt == nil || call.OccurredAt == nil ||
		call.StartedAt.IsZero() || call.FinishedAt.IsZero() || call.OccurredAt.IsZero() ||
		call.FinishedAt.Before(*call.StartedAt)
}

func invalidQuotaCallOccurrence(call ModelCall) bool {
	return call.OccurredAt.Location() != time.UTC || call.OccurredAt.Before(*call.StartedAt) || call.OccurredAt.After(*call.FinishedAt)
}

func quotaCallOutsideTask(task ModelTask, call ModelCall) bool {
	return task.StartedAt == nil || call.StartedAt.Before(*task.StartedAt) ||
		task.EndedAt != nil && call.FinishedAt.After(*task.EndedAt)
}

func quotaReportCallFromEvidence(call ModelCall) QuotaReportCall {
	evidence := call.QuotaEvidence
	return QuotaReportCall{
		CallID:           call.CallID,
		Endpoint:         call.Endpoint,
		Model:            call.ModelID,
		OneAPIRequestID:  cloneOptionalString(evidence.OneAPIRequestID),
		BillingQuotaRaw:  cloneQuotaRawValue(evidence.BillingQuotaRaw),
		UsageQuotaRaw:    cloneQuotaRawValue(evidence.UsageQuotaRaw),
		Quota:            cloneOptionalDecimal(evidence.Quota),
		QuotaSource:      cloneQuotaSource(evidence.Source),
		EvidenceStatus:   evidence.Status,
		EvidenceIssues:   append(make([]QuotaEvidenceIssue, 0, len(evidence.Issues)), evidence.Issues...),
		EvidenceRevision: evidence.Revision,
		OccurredAt:       *call.OccurredAt,
	}
}

func validateBuiltFinalQuotaReport(report FinalQuotaReport, task ModelTask) error {
	if err := report.Validate(); err != nil {
		return err
	}
	if task.Result == nil || report.TaskResult != *task.Result {
		return newError(ErrorContractInvalid, "final quota report result does not match persisted task")
	}
	if task.Scope.CloudBinding == nil || report.Scope != *task.Scope.CloudBinding ||
		report.TaskID != task.TaskID || task.Freeze == nil || report.FreezeID != task.Freeze.FreezeID {
		return newError(ErrorScopeMismatch, "final quota report does not match persisted task")
	}
	return nil
}

func cloneQuotaRawValue(value QuotaRawValue) QuotaRawValue {
	value.Value = append(json.RawMessage(nil), value.Value...)
	return value
}

func cloneQuotaSource(value *QuotaSource) *QuotaSource {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
