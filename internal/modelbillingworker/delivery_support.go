package modelbillingworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"kwu/internal/modelbilling"
)

type deliveryErrorAction uint8

const (
	deliveryLeave deliveryErrorAction = iota
	deliveryRetry
	deliveryBlock
)

type deliveryPassContinueError struct {
	cause error
}

func (e *deliveryPassContinueError) Error() string {
	if e == nil || e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func (e *deliveryPassContinueError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func continueDeliveryPass(err error) error {
	if err == nil || canContinueDeliveryPass(err) {
		return err
	}
	return &deliveryPassContinueError{cause: err}
}

func canContinueDeliveryPass(err error) bool {
	for err != nil {
		if _, ok := err.(*deliveryPassContinueError); ok {
			return true
		}
		switch wrapped := err.(type) {
		case interface{ Unwrap() []error }:
			return false
		case interface{ Unwrap() error }:
			err = wrapped.Unwrap()
		default:
			return false
		}
	}
	return false
}

func classifyDeliveryError(err error) (deliveryErrorAction, modelbilling.ErrorCode) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return deliveryLeave, ""
	}
	if isDeterministicLocalPersistenceError(err) {
		return deliveryBlock, modelbilling.ErrorCodeOf(err)
	}
	var domainErr *modelbilling.Error
	if !errors.As(err, &domainErr) {
		return deliveryLeave, ""
	}
	switch domainErr.Code {
	case modelbilling.ErrorUsageRejected, modelbilling.ErrorLegacyReportContract,
		modelbilling.ErrorQuotaEvidenceIncomplete, modelbilling.ErrorQuotaCallIDConflict,
		modelbilling.ErrorQuotaRequestIDConflict, modelbilling.ErrorQuotaRequestIDReused,
		modelbilling.ErrorQuotaSummaryMismatch, modelbilling.ErrorQuotaTimeInvalid,
		modelbilling.ErrorQuotaFinalPayloadConflict, modelbilling.ErrorQuotaFinalIdempotencyConflict,
		modelbilling.ErrorQuotaPartialSnapshotConflict:
		return deliveryBlock, domainErr.Code
	case modelbilling.ErrorUsageRetryable, modelbilling.ErrorGatewayUnavailable,
		modelbilling.ErrorSettlementException:
		return deliveryRetry, domainErr.Code
	case modelbilling.ErrorAuthRequired, modelbilling.ErrorSessionExpired, modelbilling.ErrorScopeMismatch,
		modelbilling.ErrorContractInvalid, modelbilling.ErrorIdempotencyConflict,
		modelbilling.ErrorBillingSourceMismatch, modelbilling.ErrorInvalidStateTransition:
		return deliveryLeave, domainErr.Code
	default:
		if domainErr.Retryable {
			return deliveryRetry, domainErr.Code
		}
		return deliveryLeave, domainErr.Code
	}
}

func (w *Worker) finishDeliveryError(
	ctx context.Context,
	scope modelbilling.TaskScope,
	report modelbilling.PendingReport,
	deliveryErr error,
) error {
	code := modelbilling.ErrorCodeOf(deliveryErr)
	if handled, err := w.handleRejectedFinalCorrection(ctx, scope, report, code, deliveryErr); handled {
		return err
	}
	action, code := classifyDeliveryError(deliveryErr)
	switch action {
	case deliveryLeave:
		return deliveryErr
	case deliveryBlock:
		return w.blockDeliveryError(ctx, scope, report, code, deliveryErr)
	case deliveryRetry:
		return w.retryDeliveryError(ctx, scope, report, code, deliveryErr)
	default:
		return deliveryErr
	}
}

func (w *Worker) handleRejectedFinalCorrection(
	ctx context.Context,
	scope modelbilling.TaskScope,
	report modelbilling.PendingReport,
	code modelbilling.ErrorCode,
	deliveryErr error,
) (bool, error) {
	if report.Kind != modelbilling.ReportFinal {
		return false, nil
	}
	if code != modelbilling.ErrorQuotaSummaryMismatch && code != modelbilling.ErrorQuotaTimeInvalid {
		return false, nil
	}
	return w.correctRejectedFinal(ctx, scope, report, code, deliveryErr)
}

func (w *Worker) blockDeliveryError(
	ctx context.Context,
	scope modelbilling.TaskScope,
	report modelbilling.PendingReport,
	code modelbilling.ErrorCode,
	deliveryErr error,
) error {
	at, err := w.now()
	if err != nil {
		return errors.Join(deliveryErr, fmt.Errorf("prepare delivery outcome: %w", err))
	}
	if err := w.reports.MarkBlocked(ctx, scope, report.IdempotencyKey, report.AttemptCount, string(code), at); err != nil {
		return errors.Join(deliveryErr, fmt.Errorf("mark report blocked: %w", err))
	}
	return continueDeliveryPass(deliveryErr)
}

func (w *Worker) retryDeliveryError(
	ctx context.Context,
	scope modelbilling.TaskScope,
	report modelbilling.PendingReport,
	code modelbilling.ErrorCode,
	deliveryErr error,
) error {
	at, err := w.now()
	if err != nil {
		return errors.Join(deliveryErr, fmt.Errorf("prepare delivery outcome: %w", err))
	}
	next, err := w.nextRetry(report.AttemptCount, at)
	if err != nil {
		return errors.Join(deliveryErr, fmt.Errorf("schedule report retry: %w", err))
	}
	if err := w.reports.MarkRetry(ctx, scope, report.IdempotencyKey, report.AttemptCount, next, string(code), at); err != nil {
		return errors.Join(deliveryErr, fmt.Errorf("mark report retry: %w", err))
	}
	return continueDeliveryPass(deliveryErr)
}

func (w *Worker) correctRejectedFinal(
	ctx context.Context,
	scope modelbilling.TaskScope,
	report modelbilling.PendingReport,
	code modelbilling.ErrorCode,
	deliveryErr error,
) (bool, error) {
	if w.corrector == nil {
		return false, nil
	}
	at, err := w.now()
	if err != nil {
		return true, errors.Join(deliveryErr, fmt.Errorf("prepare final quota correction: %w", err))
	}
	if _, err := w.corrector.CorrectUnacceptedFinal(
		ctx, scope, report.IdempotencyKey, report.AttemptCount, code, at,
	); err == nil {
		return true, continueDeliveryPass(deliveryErr)
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		!isDeterministicPersistenceCode(modelbilling.ErrorCodeOf(err)) {
		return true, errors.Join(deliveryErr, fmt.Errorf("correct rejected final quota report: %w", err))
	}
	return false, nil
}

func (w *Worker) nextRetry(attempt int, now time.Time) (time.Time, error) {
	if attempt <= 0 {
		return time.Time{}, contractError("delivery attempt must be positive")
	}
	raw := cappedExponential(w.retry.BaseDelay, w.retry.MaxDelay, attempt)
	limit := retryJitterLimit(raw, w.retry.MaxDelay, w.retry.JitterFraction)
	jitter := w.retry.Jitter(limit)
	if jitter < 0 || jitter > limit {
		return time.Time{}, contractError("jitter source returned a value outside its bound")
	}
	return now.Add(raw + jitter), nil
}

func cappedExponential(base, maximum time.Duration, attempt int) time.Duration {
	delay := base
	for exponent := 1; exponent < attempt && delay < maximum; exponent++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}

func retryJitterLimit(raw, maximum time.Duration, fraction float64) time.Duration {
	remaining := maximum - raw
	if remaining <= 0 || fraction == 0 {
		return 0
	}
	fractional := float64(raw) * fraction
	if fractional >= float64(remaining) {
		return remaining
	}
	return time.Duration(fractional)
}

type decodedReport struct {
	partial *modelbilling.PartialQuotaReport
	final   *modelbilling.FinalQuotaReport
}

func decodePendingReport(pending modelbilling.PendingReport) (decodedReport, error) {
	if err := validatePendingReportEnvelope(pending); err != nil {
		return decodedReport{}, err
	}
	switch pending.Kind {
	case modelbilling.ReportPartial:
		return decodePendingPartial(pending)
	case modelbilling.ReportFinal:
		return decodePendingFinal(pending)
	default:
		return decodedReport{}, contractError("unsupported pending report kind")
	}
}

func validatePendingReportEnvelope(pending modelbilling.PendingReport) error {
	if err := pending.Validate(); err != nil {
		return err
	}
	if pending.ContractVersion != modelbilling.QuotaContractV1 {
		return &modelbilling.Error{
			Code: modelbilling.ErrorLegacyReportContract, Message: "legacy token report is not deliverable",
		}
	}
	hash, err := modelbilling.QuotaPayloadHash(pending.CanonicalPayload)
	if err != nil {
		return err
	}
	if pending.PayloadHash == "" || pending.PayloadHash != hash {
		return contractError("quota report payload hash mismatch")
	}
	return nil
}

func decodePendingPartial(pending modelbilling.PendingReport) (decodedReport, error) {
	var report modelbilling.PartialQuotaReport
	if err := strictDecode(pending.CanonicalPayload, &report); err != nil {
		return decodedReport{}, err
	}
	if err := validateDecodedPartial(pending, &report); err != nil {
		return decodedReport{}, err
	}
	return decodedReport{partial: &report}, nil
}

func decodePendingFinal(pending modelbilling.PendingReport) (decodedReport, error) {
	var report modelbilling.FinalQuotaReport
	if err := strictDecode(pending.CanonicalPayload, &report); err != nil {
		return decodedReport{}, err
	}
	if err := validateDecodedFinal(pending, &report); err != nil {
		return decodedReport{}, err
	}
	return decodedReport{final: &report}, nil
}

func strictDecode(payload json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return contractError("decode canonical report payload: " + err.Error())
	}
	var trailer any
	if err := decoder.Decode(&trailer); err != io.EOF {
		if err == nil {
			return contractError("canonical report payload contains multiple JSON values")
		}
		return contractError("decode canonical report trailer: " + err.Error())
	}
	return nil
}

func validateDecodedPartial(pending modelbilling.PendingReport, report *modelbilling.PartialQuotaReport) error {
	if report == nil {
		return contractError("partial quota payload is required")
	}
	report.Scope = pending.Scope
	report.IdempotencyKey = pending.IdempotencyKey
	if err := report.Validate(); err != nil {
		return err
	}
	if report.TaskID != pending.TaskID || report.FreezeID != pending.FreezeID {
		return contractError("partial payload identity does not match pending report")
	}
	if report.Sequence != pending.Sequence || report.IdempotencyKey != pending.IdempotencyKey {
		return &modelbilling.Error{Code: modelbilling.ErrorIdempotencyConflict, Message: "partial payload key or sequence does not match pending report"}
	}
	return validateCanonicalReport(pending.CanonicalPayload, *report)
}

func validateDecodedFinal(pending modelbilling.PendingReport, report *modelbilling.FinalQuotaReport) error {
	if report == nil {
		return contractError("final quota payload is required")
	}
	report.Scope = pending.Scope
	report.IdempotencyKey = pending.IdempotencyKey
	report.TaskResult = modelbilling.TaskResultCompleted
	report.CreatedAt = report.ClientSummary.FinishedAt
	if err := report.Validate(); err != nil {
		return err
	}
	if report.TaskID != pending.TaskID || report.FreezeID != pending.FreezeID {
		return contractError("final payload identity does not match pending report")
	}
	return validateCanonicalReport(pending.CanonicalPayload, *report)
}

func validateCanonicalReport(payload json.RawMessage, report any) error {
	canonical, err := modelbilling.CanonicalJSON(report)
	if err != nil {
		return contractError("encode canonical report payload: " + err.Error())
	}
	return modelbilling.ValidateCanonicalPayload(payload, canonical)
}
