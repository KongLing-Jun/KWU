package modelbilling

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"time"

	"kwu/internal/cloud"
)

// QuotaReportCall is the complete canonical fingerprint sent for one Maas
// call. Token usage and NewAPI pricing evidence intentionally do not appear.
type QuotaReportCall struct {
	CallID           string               `json:"call_id"`
	Endpoint         MaasEndpoint         `json:"endpoint"`
	Model            string               `json:"model"`
	OneAPIRequestID  *string              `json:"oneapi_request_id"`
	BillingQuotaRaw  QuotaRawValue        `json:"billing_quota_raw"`
	UsageQuotaRaw    QuotaRawValue        `json:"usage_quota_raw"`
	Quota            *DecimalString       `json:"quota"`
	QuotaSource      *QuotaSource         `json:"quota_source"`
	EvidenceStatus   QuotaEvidenceStatus  `json:"evidence_status"`
	EvidenceIssues   []QuotaEvidenceIssue `json:"evidence_issues"`
	EvidenceRevision uint64               `json:"evidence_revision"`
	OccurredAt       time.Time            `json:"occurred_at"`
}

type PartialQuotaReport struct {
	ContractVersion string             `json:"contract_version"`
	TaskID          string             `json:"task_id"`
	FreezeID        string             `json:"freeze_id"`
	Sequence        uint64             `json:"partial_sequence"`
	ReportMode      ReportMode         `json:"report_mode"`
	CapturedAt      time.Time          `json:"captured_at"`
	Calls           []QuotaReportCall  `json:"calls"`
	Scope           cloud.BindingScope `json:"-"`
	IdempotencyKey  string             `json:"-"`
}

type FinalQuotaReport struct {
	ContractVersion string             `json:"contract_version"`
	TaskID          string             `json:"task_id"`
	FreezeID        string             `json:"freeze_id"`
	ClientSummary   QuotaTaskSummary   `json:"client_summary"`
	Calls           []QuotaReportCall  `json:"calls"`
	Scope           cloud.BindingScope `json:"-"`
	IdempotencyKey  string             `json:"-"`
	TaskResult      TaskResult         `json:"-"`
	CreatedAt       time.Time          `json:"-"`
}

type QuotaTaskSummary struct {
	TaskTotalQuota DecimalString `json:"task_total_quota"`
	ModelCallCount int           `json:"model_call_count"`
	StartedAt      time.Time     `json:"started_at"`
	FinishedAt     time.Time     `json:"finished_at"`
}

type CloudQuotaClient interface {
	UploadPartialQuota(context.Context, cloud.BindingScope, PartialQuotaReport) error
	FinalizeQuota(context.Context, cloud.BindingScope, FinalQuotaReport) (QuotaSettlementResult, error)
	GetQuotaSettlement(context.Context, cloud.BindingScope, string) (QuotaSettlementResult, error)
}

func (c QuotaReportCall) Validate() error {
	if err := validateQuotaReportCallIdentity(c); err != nil {
		return err
	}
	if err := validateQuotaReportCallRawEvidence(c); err != nil {
		return err
	}
	if err := validateQuotaReportCallMetadata(c); err != nil {
		return err
	}
	if !quotaReportCallProjectionMatches(c) {
		return newError(ErrorContractInvalid, "quota report call does not match its raw evidence")
	}
	return nil
}

func validateQuotaReportCallIdentity(call QuotaReportCall) error {
	if err := requireTrimmed("call_id", call.CallID); err != nil {
		return err
	}
	if err := call.Endpoint.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("model", call.Model); err != nil {
		return err
	}
	if call.OneAPIRequestID != nil {
		if err := requireTrimmed("oneapi_request_id", *call.OneAPIRequestID); err != nil {
			return err
		}
	}
	return nil
}

func validateQuotaReportCallRawEvidence(call QuotaReportCall) error {
	if err := call.BillingQuotaRaw.Validate(); err != nil {
		return err
	}
	return call.UsageQuotaRaw.Validate()
}

func validateQuotaReportCallMetadata(call QuotaReportCall) error {
	if call.EvidenceRevision == 0 || call.OccurredAt.IsZero() || call.OccurredAt.Location() != time.UTC {
		return newError(ErrorContractInvalid, "quota report call requires revision and UTC occurred_at")
	}
	if call.EvidenceIssues == nil {
		return newError(ErrorContractInvalid, "quota report evidence issues must be an array")
	}
	issues, err := normalizeEvidenceIssues(call.EvidenceIssues)
	if err != nil {
		return err
	}
	if !quotaEvidenceIssuesEqual(issues, call.EvidenceIssues) {
		return newError(ErrorContractInvalid, "quota evidence issues are not canonical")
	}
	return nil
}

func quotaReportCallProjectionMatches(call QuotaReportCall) bool {
	expected := deriveQuotaEvidence(call.OneAPIRequestID, call.BillingQuotaRaw, call.UsageQuotaRaw)
	return optionalStringEqual(expected.OneAPIRequestID, call.OneAPIRequestID) &&
		optionalDecimalEqual(expected.Quota, call.Quota) &&
		optionalQuotaSourceEqual(expected.Source, call.QuotaSource) &&
		expected.Status == call.EvidenceStatus &&
		quotaEvidenceIssuesEqual(expected.Issues, call.EvidenceIssues)
}

func (c QuotaReportCall) ValidateForFinal() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.EvidenceStatus != QuotaEvidenceComplete || len(c.EvidenceIssues) != 0 ||
		c.OneAPIRequestID == nil || c.Quota == nil || c.QuotaSource == nil {
		return newError(ErrorQuotaEvidenceIncomplete, "final quota report requires complete call evidence")
	}
	return nil
}

func (r PartialQuotaReport) Validate() error {
	if r.ContractVersion != QuotaContractV1 {
		return newError(ErrorLegacyReportContract, "partial report is not a quota contract payload")
	}
	if err := validateQuotaReportIdentity(r.Scope, r.TaskID, r.FreezeID, r.IdempotencyKey, r.CapturedAt); err != nil {
		return err
	}
	if r.Sequence == 0 || r.ReportMode != ReportModeCumulative {
		return newError(ErrorContractInvalid, "partial quota report requires positive sequence and cumulative mode")
	}
	if err := validatePartialQuotaReportKey(r); err != nil {
		return err
	}
	if err := validatePartialQuotaReportCalls(r.Calls); err != nil {
		return err
	}
	return validatePartialQuotaReportTimes(r.Calls, r.CapturedAt)
}

func validatePartialQuotaReportKey(report PartialQuotaReport) error {
	expected, err := BuildPartialIdempotencyKey(report.Scope, report.TaskID, report.FreezeID, report.Sequence)
	if err != nil {
		return err
	}
	if expected != report.IdempotencyKey {
		return newError(ErrorIdempotencyConflict, "partial quota idempotency key does not match identity")
	}
	return nil
}

func validatePartialQuotaReportCalls(calls []QuotaReportCall) error {
	if len(calls) == 0 {
		return newError(ErrorContractInvalid, "partial quota report requires calls")
	}
	canonical, err := CanonicalizeQuotaReportCalls(calls)
	if err != nil {
		return err
	}
	if !quotaReportCallsEqual(canonical, calls) {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota calls are not a canonical snapshot")
	}
	return nil
}

func validatePartialQuotaReportTimes(calls []QuotaReportCall, capturedAt time.Time) error {
	for _, call := range calls {
		if call.OccurredAt.After(capturedAt) {
			return newError(ErrorQuotaTimeInvalid, "partial call occurred after captured_at")
		}
	}
	return nil
}

func (r FinalQuotaReport) Validate() error {
	if r.ContractVersion != QuotaContractV1 {
		return newError(ErrorLegacyReportContract, "final report is not a quota contract payload")
	}
	if err := validateQuotaReportIdentity(r.Scope, r.TaskID, r.FreezeID, r.IdempotencyKey, r.CreatedAt); err != nil {
		return err
	}
	if err := validateFinalQuotaSummary(r.ClientSummary, r.TaskResult); err != nil {
		return err
	}
	if err := validateFinalQuotaReportKey(r); err != nil {
		return err
	}
	if err := validateFinalQuotaReportCalls(r.Calls, r.ClientSummary); err != nil {
		return err
	}
	return validateFinalQuotaReportTotal(r.Calls, r.ClientSummary)
}

func validateFinalQuotaSummary(summary QuotaTaskSummary, result TaskResult) error {
	if err := result.Validate(); err != nil {
		return err
	}
	if summary.StartedAt.IsZero() || summary.FinishedAt.IsZero() ||
		summary.StartedAt.Location() != time.UTC || summary.FinishedAt.Location() != time.UTC ||
		summary.FinishedAt.Before(summary.StartedAt) {
		return newError(ErrorQuotaTimeInvalid, "final quota summary timestamps are invalid")
	}
	return summary.TaskTotalQuota.Validate()
}

func validateFinalQuotaReportKey(report FinalQuotaReport) error {
	expectedKey, err := BuildFinalIdempotencyKey(report.Scope, report.TaskID, report.FreezeID)
	if err != nil {
		return err
	}
	if expectedKey != report.IdempotencyKey {
		return newError(ErrorIdempotencyConflict, "final quota idempotency key does not match identity")
	}
	return nil
}

func validateFinalQuotaReportCalls(calls []QuotaReportCall, summary QuotaTaskSummary) error {
	canonical, err := CanonicalizeQuotaReportCalls(calls)
	if err != nil {
		return err
	}
	if len(canonical) == 0 {
		return newError(ErrorContractInvalid, "final quota report requires calls")
	}
	if !quotaReportCallsEqual(canonical, calls) {
		return newError(ErrorQuotaCallIDConflict, "final quota calls are not canonical")
	}
	for _, call := range calls {
		if err := call.ValidateForFinal(); err != nil {
			return err
		}
		if call.OccurredAt.Before(summary.StartedAt) || call.OccurredAt.After(summary.FinishedAt) {
			return newError(ErrorQuotaTimeInvalid, "final call occurred outside task interval")
		}
	}
	return nil
}

func validateFinalQuotaReportTotal(calls []QuotaReportCall, summary QuotaTaskSummary) error {
	total, err := SumQuotaReportCalls(calls)
	if err != nil {
		return err
	}
	if summary.ModelCallCount != len(calls) || total != summary.TaskTotalQuota {
		return newError(ErrorQuotaSummaryMismatch, "final quota summary does not match canonical calls")
	}
	return nil
}

func (r FinalQuotaReport) ValidateForTask(task ModelTask) error {
	if err := validateFinalQuotaTask(task); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if err := validateFinalQuotaTaskResult(task, r); err != nil {
		return err
	}
	if err := validateFinalQuotaTaskInterval(task, r.ClientSummary); err != nil {
		return err
	}
	return validateFinalQuotaTaskOwnership(task, r)
}

func validateFinalQuotaTask(task ModelTask) error {
	if task.BillingSource != BillingSubscriptionPool {
		return newError(ErrorBillingSourceMismatch, "final quota report requires a subscription task")
	}
	return task.Validate()
}

func validateFinalQuotaTaskResult(task ModelTask, report FinalQuotaReport) error {
	if task.Result == nil || *task.Result != report.TaskResult {
		return newError(ErrorContractInvalid, "final quota task_result does not match persisted result")
	}
	return nil
}

func validateFinalQuotaTaskInterval(task ModelTask, summary QuotaTaskSummary) error {
	if task.StartedAt == nil || task.EndedAt == nil ||
		!summary.StartedAt.Equal(*task.StartedAt) ||
		!summary.FinishedAt.Equal(*task.EndedAt) {
		return newError(ErrorQuotaTimeInvalid, "final quota summary does not match persisted task interval")
	}
	return nil
}

func validateFinalQuotaTaskOwnership(task ModelTask, report FinalQuotaReport) error {
	if task.TaskID != report.TaskID || task.Scope.CloudBinding == nil || *task.Scope.CloudBinding != report.Scope ||
		task.Freeze == nil || task.Freeze.FreezeID != report.FreezeID {
		return newError(ErrorScopeMismatch, "final quota report does not match task and freeze")
	}
	return nil
}

func CanonicalizeQuotaReportCalls(input []QuotaReportCall) ([]QuotaReportCall, error) {
	items, err := normalizeQuotaReportCalls(input)
	if err != nil {
		return nil, err
	}
	sortQuotaReportCalls(items)
	return deduplicateQuotaReportCalls(items)
}

func normalizeQuotaReportCalls(input []QuotaReportCall) ([]QuotaReportCall, error) {
	items := make([]QuotaReportCall, len(input))
	for i := range input {
		items[i] = cloneQuotaReportCall(input[i])
		issues, err := normalizeEvidenceIssues(items[i].EvidenceIssues)
		if err != nil {
			return nil, err
		}
		items[i].EvidenceIssues = issues
		if err := items[i].Validate(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func sortQuotaReportCalls(items []QuotaReportCall) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurredAt.Equal(items[j].OccurredAt) {
			return items[i].CallID < items[j].CallID
		}
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
}

func deduplicateQuotaReportCalls(items []QuotaReportCall) ([]QuotaReportCall, error) {
	out := make([]QuotaReportCall, 0, len(items))
	callFingerprints := make(map[string][]byte, len(items))
	requestOwners := make(map[string]string, len(items))
	for _, call := range items {
		fingerprint, err := CanonicalJSON(call)
		if err != nil {
			return nil, err
		}
		duplicate, err := checkQuotaCallFingerprint(callFingerprints, call.CallID, fingerprint)
		if err != nil {
			return nil, err
		}
		if duplicate {
			continue
		}
		if err := recordQuotaRequestOwner(requestOwners, call); err != nil {
			return nil, err
		}
		callFingerprints[call.CallID] = append([]byte(nil), fingerprint...)
		out = append(out, call)
	}
	return out, nil
}

func checkQuotaCallFingerprint(fingerprints map[string][]byte, callID string, fingerprint []byte) (bool, error) {
	prior, found := fingerprints[callID]
	if !found {
		return false, nil
	}
	if bytes.Equal(prior, fingerprint) {
		return true, nil
	}
	return false, newError(ErrorQuotaCallIDConflict, "same call_id has different quota fingerprint")
}

func recordQuotaRequestOwner(owners map[string]string, call QuotaReportCall) error {
	if call.OneAPIRequestID == nil {
		return nil
	}
	if owner, found := owners[*call.OneAPIRequestID]; found && owner != call.CallID {
		return newError(ErrorQuotaRequestIDConflict, "different calls reuse oneapi_request_id")
	}
	owners[*call.OneAPIRequestID] = call.CallID
	return nil
}

func SumQuotaReportCalls(calls []QuotaReportCall) (DecimalString, error) {
	values := make([]DecimalString, 0, len(calls))
	for _, call := range calls {
		if call.Quota == nil {
			return "", newError(ErrorQuotaEvidenceIncomplete, "cannot sum a missing quota")
		}
		values = append(values, *call.Quota)
	}
	return sumDecimalStrings(values)
}

func QuotaPayloadHash(payload json.RawMessage) (string, error) {
	if len(payload) == 0 || !json.Valid(payload) {
		return "", newError(ErrorContractInvalid, "quota payload must be valid JSON")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

type decimalParts struct {
	integer *big.Int
	scale   int
}

func sumDecimalStrings(values []DecimalString) (DecimalString, error) {
	parts, maxScale, err := decimalPartsForSum(values)
	if err != nil {
		return "", err
	}
	total := sumDecimalParts(parts, maxScale)
	return NewDecimalString(formatDecimalSum(total, maxScale))
}

func decimalPartsForSum(values []DecimalString) ([]decimalParts, int, error) {
	maxScale := 0
	parts := make([]decimalParts, len(values))
	for i, value := range values {
		if err := value.Validate(); err != nil {
			return nil, 0, err
		}
		part, ok := decimalPart(value)
		if !ok {
			return nil, 0, newError(ErrorContractInvalid, "invalid quota decimal")
		}
		parts[i] = part
		if part.scale > maxScale {
			maxScale = part.scale
		}
	}
	return parts, maxScale, nil
}

func decimalPart(value DecimalString) (decimalParts, bool) {
	text := string(value)
	scale := 0
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		scale = len(text) - dot - 1
		text = text[:dot] + text[dot+1:]
	}
	integer, ok := new(big.Int).SetString(text, 10)
	return decimalParts{integer: integer, scale: scale}, ok
}

func sumDecimalParts(parts []decimalParts, maxScale int) *big.Int {
	total := new(big.Int)
	ten := big.NewInt(10)
	for _, part := range parts {
		value := new(big.Int).Set(part.integer)
		if delta := maxScale - part.scale; delta > 0 {
			value.Mul(value, new(big.Int).Exp(ten, big.NewInt(int64(delta)), nil))
		}
		total.Add(total, value)
	}
	return total
}

func formatDecimalSum(total *big.Int, maxScale int) string {
	digits := total.String()
	if maxScale > 0 {
		if len(digits) <= maxScale {
			digits = strings.Repeat("0", maxScale-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-maxScale] + "." + digits[len(digits)-maxScale:]
		digits = strings.TrimRight(strings.TrimRight(digits, "0"), ".")
	}
	if digits == "" {
		return "0"
	}
	return digits
}

func validateQuotaReportIdentity(scope cloud.BindingScope, taskID, freezeID, key string, at time.Time) error {
	if err := scope.Validate(); err != nil {
		return newError(ErrorScopeMismatch, err.Error())
	}
	for name, value := range map[string]string{"task_id": taskID, "freeze_id": freezeID, "idempotency_key": key} {
		if err := requireTrimmed(name, value); err != nil {
			return err
		}
	}
	if at.IsZero() || at.Location() != time.UTC {
		return newError(ErrorQuotaTimeInvalid, "quota report time must be UTC")
	}
	return nil
}

func cloneQuotaReportCall(input QuotaReportCall) QuotaReportCall {
	output := input
	output.OneAPIRequestID = cloneOptionalString(input.OneAPIRequestID)
	output.Quota = cloneOptionalDecimal(input.Quota)
	if input.QuotaSource != nil {
		value := *input.QuotaSource
		output.QuotaSource = &value
	}
	output.BillingQuotaRaw.Value = append(json.RawMessage(nil), input.BillingQuotaRaw.Value...)
	output.UsageQuotaRaw.Value = append(json.RawMessage(nil), input.UsageQuotaRaw.Value...)
	if input.EvidenceIssues != nil {
		output.EvidenceIssues = append(make([]QuotaEvidenceIssue, 0, len(input.EvidenceIssues)), input.EvidenceIssues...)
	}
	return output
}

func cloneOptionalDecimal(value *DecimalString) *DecimalString {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func quotaReportCallsEqual(left, right []QuotaReportCall) bool {
	if len(left) != len(right) {
		return false
	}
	leftPayload, leftErr := CanonicalJSON(left)
	rightPayload, rightErr := CanonicalJSON(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftPayload, rightPayload)
}
