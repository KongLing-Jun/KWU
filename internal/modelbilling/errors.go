package modelbilling

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"kwu/internal/cloud"
)

type ErrorCode string

const (
	ErrorAuthRequired                  ErrorCode = "auth_required"
	ErrorSessionExpired                ErrorCode = "session_expired"
	ErrorByokCredentialInvalid         ErrorCode = "byok_credential_invalid"
	ErrorScopeMismatch                 ErrorCode = "scope_mismatch"
	ErrorInsufficientCredits           ErrorCode = "insufficient_credits"
	ErrorPricingUnavailable            ErrorCode = "pricing_unavailable"
	ErrorGatewayUnavailable            ErrorCode = "gateway_unavailable"
	ErrorUsagePersistFailed            ErrorCode = "usage_persist_failed"
	ErrorUsageRetryable                ErrorCode = "usage_retryable"
	ErrorUsageRejected                 ErrorCode = "usage_rejected"
	ErrorSettlementException           ErrorCode = "settlement_exception"
	ErrorContractInvalid               ErrorCode = "contract_invalid"
	ErrorInvalidStateTransition        ErrorCode = "invalid_state_transition"
	ErrorBillingSourceMismatch         ErrorCode = "billing_source_mismatch"
	ErrorIdempotencyConflict           ErrorCode = "idempotency_conflict"
	ErrorQuotaEvidenceIncomplete       ErrorCode = "quota_evidence_incomplete"
	ErrorQuotaCallIDConflict           ErrorCode = "quota_call_id_conflict"
	ErrorQuotaRequestIDConflict        ErrorCode = "quota_request_id_conflict"
	ErrorQuotaRequestIDReused          ErrorCode = "quota_request_id_reused"
	ErrorQuotaSummaryMismatch          ErrorCode = "quota_summary_mismatch"
	ErrorQuotaFinalPayloadConflict     ErrorCode = "quota_final_payload_conflict"
	ErrorQuotaFinalIdempotencyConflict ErrorCode = "quota_final_idempotency_conflict"
	ErrorQuotaPartialSnapshotConflict  ErrorCode = "quota_partial_snapshot_conflict"
	ErrorQuotaTimeInvalid              ErrorCode = "quota_time_invalid"
	ErrorLegacyReportContract          ErrorCode = "legacy_token_report_contract"
	ErrorSettlementBalanceInconsistent ErrorCode = "settlement_balance_inconsistent"
)

func (c ErrorCode) Validate() error {
	switch c {
	case ErrorAuthRequired, ErrorSessionExpired, ErrorByokCredentialInvalid, ErrorScopeMismatch,
		ErrorInsufficientCredits, ErrorPricingUnavailable, ErrorGatewayUnavailable,
		ErrorUsagePersistFailed, ErrorUsageRetryable, ErrorUsageRejected, ErrorSettlementException,
		ErrorContractInvalid, ErrorInvalidStateTransition, ErrorBillingSourceMismatch,
		ErrorIdempotencyConflict, ErrorQuotaEvidenceIncomplete, ErrorQuotaCallIDConflict,
		ErrorQuotaRequestIDConflict, ErrorQuotaRequestIDReused, ErrorQuotaSummaryMismatch,
		ErrorQuotaFinalPayloadConflict, ErrorQuotaFinalIdempotencyConflict,
		ErrorQuotaPartialSnapshotConflict, ErrorQuotaTimeInvalid, ErrorLegacyReportContract,
		ErrorSettlementBalanceInconsistent:
		return nil
	default:
		return newError(ErrorContractInvalid, fmt.Sprintf("unknown error code %q", c))
	}
}

type Error struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable,omitempty"`
}

type terminalPersistenceError struct {
	completeErr error
	recoveryErr error
}

func (e *terminalPersistenceError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf("%s: terminal call persistence failed: %v", ErrorUsagePersistFailed, e.completeErr)
	if e.recoveryErr != nil {
		message += fmt.Sprintf("; recovery update failed: %v", e.recoveryErr)
	}
	return message
}

func (e *terminalPersistenceError) Unwrap() []error {
	if e == nil {
		return nil
	}
	if e.recoveryErr == nil {
		return []error{e.completeErr}
	}
	return []error{e.completeErr, e.recoveryErr}
}

func newTerminalPersistenceError(completeErr, recoveryErr error) error {
	return &terminalPersistenceError{completeErr: completeErr, recoveryErr: recoveryErr}
}

func isTerminalPersistenceError(err error) bool {
	var target *terminalPersistenceError
	return errors.As(err, &target)
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func newError(code ErrorCode, message string) error {
	return &Error{Code: code, Message: message}
}

func ErrorCodeOf(err error) ErrorCode {
	if isTerminalPersistenceError(err) {
		return ErrorUsagePersistFailed
	}
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return ""
}

func BuildPartialIdempotencyKey(scope cloud.BindingScope, taskID, freezeID string, sequence uint64) (string, error) {
	if sequence == 0 {
		return "", newError(ErrorContractInvalid, "partial sequence must be positive")
	}
	return buildReportIdempotencyKey(scope, taskID, freezeID, ReportPartial, sequence)
}

func BuildFinalIdempotencyKey(scope cloud.BindingScope, taskID, freezeID string) (string, error) {
	return buildReportIdempotencyKey(scope, taskID, freezeID, ReportFinal, 0)
}

func BuildTaskScopeKey(scope TaskScope, source BillingSource) (string, error) {
	if err := scope.Validate(source); err != nil {
		return "", err
	}
	parts := []string{string(source), scope.ActorID, scope.ProfileID}
	if source == BillingSubscriptionPool {
		parts = append(parts, scope.CloudBinding.CloudUserID, fmt.Sprint(scope.CloudBinding.BindingGeneration))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "kwu_scope_" + hex.EncodeToString(sum[:16]), nil
}

func buildReportIdempotencyKey(scope cloud.BindingScope, taskID, freezeID string, kind ReportKind, sequence uint64) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", newError(ErrorScopeMismatch, err.Error())
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return "", err
	}
	if err := requireTrimmed("freeze_id", freezeID); err != nil {
		return "", err
	}
	payload := fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s\x00%d", scope.ProfileID, scope.CloudUserID,
		scope.BindingGeneration, taskID, freezeID, sequence)
	payload += "\x00" + string(kind)
	sum := sha256.Sum256([]byte(payload))
	return "kwu_" + hex.EncodeToString(sum[:16]), nil
}

func CanonicalJSON(value any) (json.RawMessage, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, newError(ErrorContractInvalid, "marshal canonical payload")
	}
	return payload, nil
}

func ValidateCanonicalPayload(existing, candidate json.RawMessage) error {
	if !json.Valid(existing) || !json.Valid(candidate) {
		return newError(ErrorContractInvalid, "canonical payload must be valid JSON")
	}
	if !bytes.Equal(existing, candidate) {
		return newError(ErrorIdempotencyConflict, "idempotency key reused with a different canonical payload")
	}
	return nil
}
