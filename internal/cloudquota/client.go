// Package cloudquota adapts the legacy Reasonix Cloud MaaS quota REST contract
// to the normalized model-billing boundary. It deliberately owns wire DTOs so
// cloud fields never leak into the durable domain model by accidental JSON reuse.
package cloudquota

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

const (
	partialPath = "/api/v1/usage-reports/partial"
	finalPath   = "/api/v1/usage-reports/finalize"
)

// Client uses the same authenticated session executor as the account and
// freeze clients, including its single refresh-and-replay behavior.
type Client struct {
	executor  cloud.AuthenticatedRequestExecutor
	transport *cloud.HTTPSessionClient
}

func New(executor cloud.AuthenticatedRequestExecutor, transport *cloud.HTTPSessionClient) (*Client, error) {
	if executor == nil || transport == nil {
		return nil, fmt.Errorf("cloudquota: authenticated transport is required")
	}
	return &Client{executor: executor, transport: transport}, nil
}

func (c *Client) UploadPartialQuota(
	ctx context.Context,
	scope cloud.BindingScope,
	report modelbilling.PartialQuotaReport,
) error {
	wire, err := partialRequest(report, scope)
	if err != nil {
		return err
	}
	var receipt partialReceiptWire
	if err := c.execute(ctx, scope, http.MethodPost, partialPath, wire, report.IdempotencyKey, &receipt); err != nil {
		return err
	}
	return validatePartialReceipt(receipt, report)
}

func (c *Client) FinalizeQuota(
	ctx context.Context,
	scope cloud.BindingScope,
	report modelbilling.FinalQuotaReport,
) (modelbilling.QuotaSettlementResult, error) {
	wire, err := finalRequest(report, scope)
	if err != nil {
		return modelbilling.QuotaSettlementResult{}, err
	}
	var response settlementResultWire
	if err := c.execute(ctx, scope, http.MethodPost, finalPath, wire, report.IdempotencyKey, &response); err != nil {
		return modelbilling.QuotaSettlementResult{}, err
	}
	return response.domain(report.TaskID)
}

func (c *Client) GetQuotaSettlement(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID string,
) (modelbilling.QuotaSettlementResult, error) {
	if err := scope.Validate(); err != nil {
		return modelbilling.QuotaSettlementResult{}, scopeError(err)
	}
	if !safePathIdentifier(taskID) {
		return modelbilling.QuotaSettlementResult{}, contractError("task_id must be a safe path segment")
	}
	var response settlementResultWire
	path := "/api/v1/task-settlements/" + url.PathEscape(taskID)
	if err := c.execute(ctx, scope, http.MethodGet, path, nil, "", &response); err != nil {
		return modelbilling.QuotaSettlementResult{}, err
	}
	return response.domain(taskID)
}

func (c *Client) execute(
	ctx context.Context,
	scope cloud.BindingScope,
	method, path string,
	body any,
	idempotencyKey string,
	destination any,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return scopeError(err)
	}
	headers := make(http.Header)
	if idempotencyKey != "" {
		headers.Set("X-Idempotency-Key", idempotencyKey)
	}
	var raw json.RawMessage
	err := c.executor.Do(ctx, scope, func(ctx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(ctx, method, path, body, token, headers, &raw)
	})
	if err != nil {
		return mapCloudError(err)
	}
	if err := decodeStrict(raw, destination); err != nil {
		return contractError("cloud response data is invalid: " + err.Error())
	}
	return nil
}

type partialRequestWire struct {
	TaskID          string                  `json:"task_id"`
	FreezeID        string                  `json:"freeze_id"`
	PartialSequence uint64                  `json:"partial_sequence"`
	ReportMode      modelbilling.ReportMode `json:"report_mode"`
	CapturedAt      time.Time               `json:"captured_at"`
	Calls           []quotaCallWire         `json:"calls"`
}

type finalRequestWire struct {
	TaskID        string           `json:"task_id"`
	FreezeID      string           `json:"freeze_id"`
	ClientSummary quotaSummaryWire `json:"client_summary"`
	Calls         []quotaCallWire  `json:"calls"`
}

type quotaSummaryWire struct {
	TaskTotalQuota modelbilling.DecimalString `json:"task_total_quota"`
	ModelCallCount int                        `json:"model_call_count"`
	StartedAt      time.Time                  `json:"started_at"`
	FinishedAt     time.Time                  `json:"finished_at"`
}

type quotaCallWire struct {
	CallID           string                            `json:"call_id"`
	Endpoint         modelbilling.MaasEndpoint         `json:"endpoint"`
	Model            string                            `json:"model"`
	OneAPIRequestID  *string                           `json:"oneapi_request_id"`
	BillingQuotaRaw  modelbilling.QuotaRawValue        `json:"billing_quota_raw"`
	Quota            *modelbilling.DecimalString       `json:"quota"`
	QuotaSource      *modelbilling.QuotaSource         `json:"quota_source"`
	EvidenceStatus   modelbilling.QuotaEvidenceStatus  `json:"evidence_status"`
	EvidenceIssues   []modelbilling.QuotaEvidenceIssue `json:"evidence_issues"`
	EvidenceRevision uint64                            `json:"evidence_revision"`
	OccurredAt       time.Time                         `json:"occurred_at"`
}

func partialRequest(report modelbilling.PartialQuotaReport, scope cloud.BindingScope) (partialRequestWire, error) {
	if err := report.Validate(); err != nil {
		return partialRequestWire{}, err
	}
	if report.Scope != scope {
		return partialRequestWire{}, scopeError(errors.New("partial report belongs to another binding"))
	}
	if report.Sequence > math.MaxInt64 {
		return partialRequestWire{}, contractError("partial sequence exceeds cloud contract")
	}
	calls, err := quotaCalls(report.Calls)
	if err != nil {
		return partialRequestWire{}, err
	}
	return partialRequestWire{
		TaskID: report.TaskID, FreezeID: report.FreezeID, PartialSequence: report.Sequence,
		ReportMode: report.ReportMode, CapturedAt: report.CapturedAt, Calls: calls,
	}, nil
}

func finalRequest(report modelbilling.FinalQuotaReport, scope cloud.BindingScope) (finalRequestWire, error) {
	if err := report.Validate(); err != nil {
		return finalRequestWire{}, err
	}
	if report.Scope != scope {
		return finalRequestWire{}, scopeError(errors.New("final report belongs to another binding"))
	}
	calls, err := quotaCalls(report.Calls)
	if err != nil {
		return finalRequestWire{}, err
	}
	return finalRequestWire{
		TaskID: report.TaskID, FreezeID: report.FreezeID,
		ClientSummary: quotaSummaryWire{
			TaskTotalQuota: report.ClientSummary.TaskTotalQuota,
			ModelCallCount: report.ClientSummary.ModelCallCount,
			StartedAt:      report.ClientSummary.StartedAt, FinishedAt: report.ClientSummary.FinishedAt,
		},
		Calls: calls,
	}, nil
}

func quotaCalls(calls []modelbilling.QuotaReportCall) ([]quotaCallWire, error) {
	result := make([]quotaCallWire, len(calls))
	for index, call := range calls {
		if call.UsageQuotaRaw.Present || call.UsageQuotaRaw.Type != "" || len(call.UsageQuotaRaw.Value) != 0 {
			return nil, rejectedError("usage_quota_raw is not accepted by the AirClaude quota contract")
		}
		if call.QuotaSource != nil && *call.QuotaSource != modelbilling.QuotaSourceBilling {
			return nil, rejectedError("quota_source must be billing.quota when present")
		}
		if call.EvidenceStatus == modelbilling.QuotaEvidenceConflicting {
			return nil, rejectedError("conflicting quota evidence cannot be uploaded")
		}
		if call.EvidenceRevision > math.MaxInt64 {
			return nil, contractError("evidence revision exceeds cloud contract")
		}
		issues := make([]modelbilling.QuotaEvidenceIssue, len(call.EvidenceIssues))
		copy(issues, call.EvidenceIssues)
		result[index] = quotaCallWire{
			CallID: call.CallID, Endpoint: call.Endpoint, Model: call.Model,
			OneAPIRequestID: call.OneAPIRequestID, BillingQuotaRaw: call.BillingQuotaRaw,
			Quota: call.Quota, QuotaSource: call.QuotaSource, EvidenceStatus: call.EvidenceStatus,
			EvidenceIssues:   issues,
			EvidenceRevision: call.EvidenceRevision, OccurredAt: call.OccurredAt,
		}
	}
	return result, nil
}

type partialReceiptWire struct {
	UsageReportID      string `json:"usage_report_id"`
	TaskID             string `json:"task_id"`
	ReportType         string `json:"report_type"`
	Status             string `json:"status"`
	PartialSequence    int64  `json:"partial_sequence"`
	AcceptedCallCount  int    `json:"accepted_call_count"`
	PayloadFingerprint string `json:"payload_fingerprint"`
}

func validatePartialReceipt(receipt partialReceiptWire, report modelbilling.PartialQuotaReport) error {
	if strings.TrimSpace(receipt.UsageReportID) != receipt.UsageReportID || receipt.UsageReportID == "" ||
		receipt.TaskID != report.TaskID || receipt.ReportType != "partial" || receipt.Status != "validated" ||
		receipt.PartialSequence != int64(report.Sequence) || receipt.AcceptedCallCount != len(report.Calls) ||
		!validSHA256(receipt.PayloadFingerprint) {
		return contractError("partial receipt does not match the uploaded report")
	}
	return nil
}

type settlementResultWire struct {
	Settlement settlementWire         `json:"settlement"`
	Balance    *cloud.BalanceSnapshot `json:"balance,omitempty"`
}

type settlementWire struct {
	SettlementID                        string            `json:"settlement_id"`
	TaskID                              string            `json:"task_id"`
	Status                              string            `json:"status"`
	TaskTotalQuota                      *string           `json:"task_total_quota,omitempty"`
	QuotaPolicyVersion                  *string           `json:"quota_policy_version,omitempty"`
	QuotaPerUnit                        *string           `json:"quota_per_unit,omitempty"`
	USDExchangeRate                     *string           `json:"usd_exchange_rate,omitempty"`
	CreditPerCNY                        *string           `json:"credit_per_cny,omitempty"`
	ChargedCredits                      *string           `json:"charged_credits,omitempty"`
	ConsumedFrozenAmount                *string           `json:"consumed_frozen_amount,omitempty"`
	ConsumedFrozenFreeAmount            *string           `json:"consumed_frozen_free_amount,omitempty"`
	ConsumedFrozenSubscriptionAmount    *string           `json:"consumed_frozen_subscription_amount,omitempty"`
	AdditionalChargedAmount             *string           `json:"additional_charged_amount,omitempty"`
	AdditionalChargedFreeAmount         *string           `json:"additional_charged_free_amount,omitempty"`
	AdditionalChargedSubscriptionAmount *string           `json:"additional_charged_subscription_amount,omitempty"`
	ReleasedFrozenAmount                *string           `json:"released_frozen_amount,omitempty"`
	ReleasedFrozenFreeAmount            *string           `json:"released_frozen_free_amount,omitempty"`
	ReleasedFrozenSubscriptionAmount    *string           `json:"released_frozen_subscription_amount,omitempty"`
	ChargedAmount                       *string           `json:"charged_amount,omitempty"`
	ChargedFreeAmount                   *string           `json:"charged_free_amount,omitempty"`
	ChargedSubscriptionAmount           *string           `json:"charged_subscription_amount,omitempty"`
	ReleasedFreeAmount                  *string           `json:"released_free_amount,omitempty"`
	ReleasedSubscriptionAmount          *string           `json:"released_subscription_amount,omitempty"`
	UncoveredAmount                     *string           `json:"uncovered_amount,omitempty"`
	Balance                             *string           `json:"balance,omitempty"`
	BalanceVersion                      *int64            `json:"balance_version,omitempty"`
	LedgerEntries                       []ledgerEntryWire `json:"ledger_entries,omitempty"`
	ErrorCode                           *string           `json:"error_code,omitempty"`
	ErrorMessage                        *string           `json:"error_message,omitempty"`
}

type ledgerEntryWire struct {
	LedgerID   string `json:"ledger_id"`
	CreditType string `json:"credit_type"`
	Delta      string `json:"delta"`
}

func (wire settlementResultWire) domain(expectedTaskID string) (modelbilling.QuotaSettlementResult, error) {
	settlement, err := wire.Settlement.domain(expectedTaskID)
	if err != nil {
		return modelbilling.QuotaSettlementResult{}, err
	}
	result := modelbilling.QuotaSettlementResult{Settlement: settlement, Balance: wire.Balance}
	if err := result.Validate(); err != nil {
		return modelbilling.QuotaSettlementResult{}, contractError("settlement response is invalid: " + err.Error())
	}
	return result, nil
}

func (wire settlementWire) domain(expectedTaskID string) (modelbilling.QuotaSettlement, error) {
	if wire.TaskID != expectedTaskID {
		return modelbilling.QuotaSettlement{}, scopeError(errors.New("settlement task_id does not match request"))
	}
	status, err := settlementStatus(wire.Status)
	if err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	if settledSettlementStatus(status) && (wire.QuotaPolicyVersion == nil || !trimmed(*wire.QuotaPolicyVersion)) {
		return modelbilling.QuotaSettlement{}, contractError("terminal settlement requires quota_policy_version")
	}
	version, err := balanceVersion(wire.BalanceVersion)
	if err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	values, err := settlementDecimals(wire)
	if err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	if err := validateLegacySettlementFields(wire, values); err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	return modelbilling.QuotaSettlement{
		SettlementID: wire.SettlementID, TaskID: wire.TaskID, Status: status,
		QuotaPolicyVersion: cloneOptional(wire.QuotaPolicyVersion),
		TaskTotalQuota:     values[0], QuotaPerUnit: values[1], USDExchangeRate: values[2], CreditPerCNY: values[3],
		ChargedCredits: values[4], ConsumedFrozenAmount: values[5], ConsumedFrozenFreeAmount: values[6],
		ConsumedFrozenSubscriptionAmount: values[7], AdditionalChargedAmount: values[8],
		AdditionalChargedFreeAmount: values[9], AdditionalChargedSubscriptionAmount: values[10],
		ReleasedFrozenAmount: values[11], ReleasedFrozenFreeAmount: values[12],
		ReleasedFrozenSubscriptionAmount: values[13], UncoveredAmount: values[14], Balance: values[15],
		BalanceVersion: version, ErrorCode: cloneOptional(wire.ErrorCode),
	}, nil
}

func validateLegacySettlementFields(wire settlementWire, values []*modelbilling.DecimalString) error {
	checks := []struct {
		legacy *string
		parts  []*modelbilling.DecimalString
	}{
		{wire.ChargedAmount, []*modelbilling.DecimalString{values[5], values[8]}},
		{wire.ChargedFreeAmount, []*modelbilling.DecimalString{values[6], values[9]}},
		{wire.ChargedSubscriptionAmount, []*modelbilling.DecimalString{values[7], values[10]}},
		{wire.ReleasedFreeAmount, []*modelbilling.DecimalString{values[12]}},
		{wire.ReleasedSubscriptionAmount, []*modelbilling.DecimalString{values[13]}},
	}
	for _, check := range checks {
		if check.legacy != nil && !decimalEqualsSum(*check.legacy, check.parts) {
			return contractError("legacy and authoritative settlement amounts disagree")
		}
	}
	return nil
}

func decimalEqualsSum(value string, parts []*modelbilling.DecimalString) bool {
	expected, ok := new(big.Rat).SetString(value)
	if !ok {
		return false
	}
	total := new(big.Rat)
	for _, part := range parts {
		if part == nil {
			return false
		}
		parsed, ok := new(big.Rat).SetString(string(*part))
		if !ok {
			return false
		}
		total.Add(total, parsed)
	}
	return total.Cmp(expected) == 0
}

func settlementDecimals(wire settlementWire) ([]*modelbilling.DecimalString, error) {
	raw := []*string{
		wire.TaskTotalQuota, wire.QuotaPerUnit, wire.USDExchangeRate, wire.CreditPerCNY, wire.ChargedCredits,
		wire.ConsumedFrozenAmount, wire.ConsumedFrozenFreeAmount, wire.ConsumedFrozenSubscriptionAmount,
		wire.AdditionalChargedAmount, wire.AdditionalChargedFreeAmount, wire.AdditionalChargedSubscriptionAmount,
		wire.ReleasedFrozenAmount, wire.ReleasedFrozenFreeAmount, wire.ReleasedFrozenSubscriptionAmount,
		wire.UncoveredAmount, wire.Balance,
	}
	result := make([]*modelbilling.DecimalString, len(raw))
	for index, value := range raw {
		if value == nil {
			continue
		}
		decimal, err := modelbilling.NewDecimalString(*value)
		if err != nil {
			return nil, contractError("settlement contains an invalid decimal")
		}
		result[index] = &decimal
	}
	return result, nil
}

func settlementStatus(value string) (modelbilling.QuotaSettlementState, error) {
	switch value {
	case string(modelbilling.QuotaSettlementPending):
		return modelbilling.QuotaSettlementPending, nil
	case string(modelbilling.QuotaSettlementSettled):
		return modelbilling.QuotaSettlementSettled, nil
	case string(modelbilling.QuotaSettlementSettledUncovered):
		return modelbilling.QuotaSettlementSettledUncovered, nil
	case string(modelbilling.QuotaSettlementManualRequired):
		return modelbilling.QuotaSettlementManualRequired, nil
	default:
		return "", contractError("unknown settlement status")
	}
}

func settledSettlementStatus(status modelbilling.QuotaSettlementState) bool {
	return status == modelbilling.QuotaSettlementSettled ||
		status == modelbilling.QuotaSettlementSettledUncovered
}

func balanceVersion(value *int64) (*uint64, error) {
	if value == nil {
		return nil, nil
	}
	if *value <= 0 {
		return nil, contractError("balance_version must be positive")
	}
	version := uint64(*value)
	return &version, nil
}

func decodeStrict(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("response data must contain one JSON value")
	}
	return nil
}

func mapCloudError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var remote *cloud.Error
	if !errors.As(err, &remote) {
		return err
	}
	code, retryable := mappedErrorCode(remote)
	domain := &modelbilling.Error{Code: code, Message: remote.Message, Retryable: retryable}
	return &mappedError{domain: domain, remote: remote}
}

func mappedErrorCode(remote *cloud.Error) (modelbilling.ErrorCode, bool) {
	if code, retryable, ok := mappedCloudBaseCode(string(remote.Code)); ok {
		return code, retryable
	}
	if code, ok := mappedQuotaEvidenceCode(string(remote.Code)); ok {
		return code, false
	}
	if code, ok := mappedQuotaIdentityCode(string(remote.Code)); ok {
		return code, false
	}
	if code, ok := mappedQuotaReportCode(string(remote.Code)); ok {
		return code, false
	}
	if remote.Retryable {
		return modelbilling.ErrorUsageRetryable, true
	}
	return modelbilling.ErrorUsageRejected, false
}

func mappedCloudBaseCode(code string) (modelbilling.ErrorCode, bool, bool) {
	switch code {
	case "AUTH_REQUIRED":
		return modelbilling.ErrorAuthRequired, false, true
	case "TOKEN_EXPIRED", "REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_EXPIRED", "REFRESH_TOKEN_REUSED", "DEVICE_MISMATCH", "ACCOUNT_DISABLED":
		return modelbilling.ErrorSessionExpired, false, true
	case "PROFILE_SCOPE_MISMATCH":
		return modelbilling.ErrorScopeMismatch, false, true
	case "NETWORK_UNAVAILABLE", "INTERNAL_ERROR", "CONCURRENT_OPERATION_RETRYABLE", "SETTLEMENT_NOT_FOUND", "SETTLEMENT_PENDING":
		return modelbilling.ErrorUsageRetryable, true, true
	case "SETTLEMENT_BALANCE_INCONSISTENT":
		return modelbilling.ErrorSettlementBalanceInconsistent, false, true
	case "IDEMPOTENCY_CONFLICT":
		return modelbilling.ErrorIdempotencyConflict, false, true
	default:
		return "", false, false
	}
}

func mappedQuotaEvidenceCode(code string) (modelbilling.ErrorCode, bool) {
	switch code {
	case "MAAS_REQUEST_ID_MISSING", "MAAS_QUOTA_MISSING", "MAAS_QUOTA_INVALID":
		return modelbilling.ErrorQuotaEvidenceIncomplete, true
	default:
		return "", false
	}
}

func mappedQuotaIdentityCode(code string) (modelbilling.ErrorCode, bool) {
	switch code {
	case "QUOTA_CALL_ID_CONFLICT":
		return modelbilling.ErrorQuotaCallIDConflict, true
	case "QUOTA_REQUEST_ID_CONFLICT":
		return modelbilling.ErrorQuotaRequestIDConflict, true
	case "QUOTA_REQUEST_ID_REUSED":
		return modelbilling.ErrorQuotaRequestIDReused, true
	default:
		return "", false
	}
}

func mappedQuotaReportCode(code string) (modelbilling.ErrorCode, bool) {
	switch code {
	case "QUOTA_SUMMARY_MISMATCH":
		return modelbilling.ErrorQuotaSummaryMismatch, true
	case "QUOTA_PARTIAL_SNAPSHOT_CONFLICT":
		return modelbilling.ErrorQuotaPartialSnapshotConflict, true
	case "QUOTA_FINAL_PAYLOAD_CONFLICT":
		return modelbilling.ErrorQuotaFinalPayloadConflict, true
	case "QUOTA_FINAL_IDEMPOTENCY_CONFLICT":
		return modelbilling.ErrorQuotaFinalIdempotencyConflict, true
	case "QUOTA_TIME_INVALID":
		return modelbilling.ErrorQuotaTimeInvalid, true
	default:
		return "", false
	}
}

type mappedError struct {
	domain *modelbilling.Error
	remote *cloud.Error
}

func (e *mappedError) Error() string   { return e.domain.Error() + ": " + e.remote.Error() }
func (e *mappedError) Unwrap() []error { return []error{e.domain, e.remote} }

func safePathIdentifier(value string) bool {
	return trimmed(value) && value != "." && value != ".." && url.PathEscape(value) == value
}

func trimmed(value string) bool { return value != "" && strings.TrimSpace(value) == value }

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			if char < 'a' || char > 'f' {
				return false
			}
		}
	}
	return true
}

func cloneOptional(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func contractError(message string) error {
	return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: message}
}

func rejectedError(message string) error {
	return &modelbilling.Error{Code: modelbilling.ErrorUsageRejected, Message: message}
}

func scopeError(err error) error {
	return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: err.Error()}
}

var _ modelbilling.CloudQuotaClient = (*Client)(nil)
