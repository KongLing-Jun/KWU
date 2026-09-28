package modelbillingworker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

// RetryConfig is explicit because the normalized quota contract has no authoritative production retry
// defaults yet. The composition root must provide every value.
type RetryConfig struct {
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	JitterFraction float64
	LeaseDuration  time.Duration
	Clock          func() time.Time
	Jitter         func(time.Duration) time.Duration
}

// SettlementTaskStore is the narrow task persistence boundary used by the quota worker.
type SettlementTaskStore interface {
	Get(context.Context, modelbilling.TaskScope, string) (modelbilling.ModelTask, error)
	UpdateState(context.Context, modelbilling.TaskStateUpdate) error
	ListSettling(context.Context, modelbilling.TaskScope) ([]modelbilling.ModelTask, error)
}

type QuotaWorkflowStore interface {
	Get(context.Context, modelbilling.TaskScope, string) (modelbilling.QuotaWorkflow, error)
	Update(context.Context, modelbilling.QuotaWorkflowUpdate) error
	modelbilling.QuotaSettlementStore
}

type Worker struct {
	reports     modelbilling.ReportStore
	corrector   modelbilling.FinalQuotaReportCorrector
	tasks       SettlementTaskStore
	cloud       modelbilling.CloudQuotaClient
	workflow    QuotaWorkflowStore
	settlements modelbilling.QuotaSettlementStore
	observer    modelbilling.SettlementObserver
	retry       RetryConfig
}

func New(
	reports modelbilling.ReportStore,
	tasks SettlementTaskStore,
	cloudClient modelbilling.CloudQuotaClient,
	observer modelbilling.SettlementObserver,
	retry RetryConfig,
	workflow ...QuotaWorkflowStore,
) (*Worker, error) {
	if reports == nil || tasks == nil || cloudClient == nil || observer == nil {
		return nil, contractError("report store, task store, cloud quota client, and settlement observer are required")
	}
	if err := retry.Validate(); err != nil {
		return nil, err
	}
	if len(workflow) > 1 || len(workflow) == 1 && workflow[0] == nil {
		return nil, contractError("at most one non-nil quota workflow store is allowed")
	}
	var workflowStore QuotaWorkflowStore
	if len(workflow) == 1 {
		workflowStore = workflow[0]
	}
	corrector, _ := reports.(modelbilling.FinalQuotaReportCorrector)
	return &Worker{
		reports: reports, corrector: corrector, tasks: tasks, cloud: cloudClient,
		workflow: workflowStore, settlements: workflowStore, observer: observer, retry: retry,
	}, nil
}

func (c RetryConfig) Validate() error {
	if c.BaseDelay <= 0 || c.MaxDelay < c.BaseDelay || c.LeaseDuration <= 0 {
		return contractError("retry delays and lease duration are invalid")
	}
	if math.IsNaN(c.JitterFraction) || math.IsInf(c.JitterFraction, 0) || c.JitterFraction < 0 || c.JitterFraction > 1 {
		return contractError("jitter fraction must be between zero and one")
	}
	if c.Clock == nil || c.Jitter == nil {
		return contractError("retry clock and jitter source are required")
	}
	return nil
}

// DeliverOne claims and delivers at most one report for a subscription scope.
func (w *Worker) DeliverOne(ctx context.Context, scope modelbilling.TaskScope) (bool, error) {
	if err := validateSubscriptionScope(scope); err != nil {
		return false, err
	}
	now, err := w.now()
	if err != nil {
		return false, err
	}
	report, found, err := w.reports.ClaimDue(ctx, scope, now, now.Add(w.retry.LeaseDuration))
	if err != nil {
		localErr := markLocalPersistenceError(err)
		if found && isDeterministicLocalPersistenceError(localErr) {
			return true, continueDeliveryPass(localErr)
		}
		return found, err
	}
	if !found {
		return false, nil
	}
	err = w.deliverClaimed(ctx, scope, report)
	return true, err
}

// ListSettling exposes the scope-safe scan needed by the later B3C lifecycle.
func (w *Worker) ListSettling(ctx context.Context, scope modelbilling.TaskScope) ([]modelbilling.ModelTask, error) {
	if err := validateSubscriptionScope(scope); err != nil {
		return nil, err
	}
	tasks, err := w.tasks.ListSettling(ctx, scope)
	if err != nil {
		return nil, markLocalPersistenceError(err)
	}
	for _, task := range tasks {
		if err := validateWorkerTask(task, scope, task.TaskID); err != nil {
			return nil, markLocalPersistenceError(err)
		}
		if task.State != modelbilling.TaskSettling {
			return nil, markLocalPersistenceError(contractError("settling task query returned another task state"))
		}
	}
	return tasks, nil
}

// ReconcileTask resumes an accepted final or queries an already-settling task.
// It never calls FinalizeQuota.
func (w *Worker) ReconcileTask(ctx context.Context, scope modelbilling.TaskScope, taskID string) error {
	return w.reconcileTask(ctx, scope, taskID, "")
}

func (w *Worker) reconcileTask(ctx context.Context, scope modelbilling.TaskScope, taskID, expectedSettlementID string) error {
	if err := validateSubscriptionScope(scope); err != nil {
		return err
	}
	task, err := w.tasks.Get(ctx, scope, taskID)
	if err != nil {
		return markLocalPersistenceError(err)
	}
	if err := validateWorkerTask(task, scope, taskID); err != nil {
		return markLocalPersistenceError(err)
	}
	switch task.State {
	case modelbilling.TaskCompleted:
		if expectedSettlementID != "" {
			return w.verifyCompletedSettlement(ctx, task, expectedSettlementID)
		}
		return nil
	case modelbilling.TaskUsagePending:
		return w.reconcileUsagePending(ctx, task, expectedSettlementID)
	case modelbilling.TaskSettling:
		return w.querySettlement(ctx, task, expectedSettlementID)
	default:
		return markLocalPersistenceError(contractError(fmt.Sprintf("task state %s cannot reconcile settlement", task.State)))
	}
}

func (w *Worker) verifyCompletedSettlement(ctx context.Context, task modelbilling.ModelTask, expectedSettlementID string) error {
	expectedTotal, err := w.acceptedFinalTaskTotal(ctx, task)
	if err != nil {
		return err
	}
	settlement, found, err := w.cachedSettlement(ctx, task)
	if err != nil {
		return err
	}
	if !found {
		return contractError("completed task requires a persisted terminal settlement")
	}
	if settlement.Settlement.SettlementID != expectedSettlementID {
		return contractError("settlement ID does not match event identity")
	}
	if settlement.Settlement.Status == modelbilling.QuotaSettlementPending {
		return contractError("completed task requires a persisted terminal settlement")
	}
	_, err = validateSettlementResult(task, expectedTotal, settlement)
	return err
}

// ReconcileSettlementEvent adapts one scope-bound settlement event to the
// existing task reconciliation path. Event identity constraints are applied by
// the same worker that persists the settlement and notifies its observer.
func (w *Worker) ReconcileSettlementEvent(
	ctx context.Context,
	actor userctx.ActorContext,
	binding cloud.BindingScope,
	taskID string,
	expectedSettlementID string,
) error {
	actor = actor.Normalize()
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	if strings.TrimSpace(expectedSettlementID) == "" || strings.TrimSpace(expectedSettlementID) != expectedSettlementID {
		return contractError("expected settlement ID must be non-empty and trimmed")
	}
	return w.reconcileTask(ctx, scope, taskID, expectedSettlementID)
}

func (w *Worker) deliverClaimed(ctx context.Context, scope modelbilling.TaskScope, pending modelbilling.PendingReport) error {
	if err := validateClaimedReport(scope, pending); err != nil {
		return w.finishDeliveryError(ctx, scope, pending, markLocalPersistenceError(err))
	}
	decoded, err := decodePendingReport(pending)
	if err != nil {
		return w.finishDeliveryError(ctx, scope, pending, localImmutableError(err))
	}
	if decoded.partial != nil {
		err = w.cloud.UploadPartialQuota(ctx, pending.Scope, *decoded.partial)
		if err != nil {
			return w.finishDeliveryError(ctx, scope, pending, err)
		}
		return w.markAccepted(ctx, scope, pending)
	}
	task, err := w.tasks.Get(ctx, scope, pending.TaskID)
	if err != nil {
		return w.finishDeliveryError(ctx, scope, pending, markLocalPersistenceError(err))
	}
	if err := validateWorkerTask(task, scope, pending.TaskID); err != nil {
		return w.finishDeliveryError(ctx, scope, pending, markLocalPersistenceError(err))
	}
	final := *decoded.final
	if err := hydrateAndValidateFinalForTask(&final, task); err != nil {
		return w.finishDeliveryError(ctx, scope, pending, err)
	}
	settlement, err := w.cloud.FinalizeQuota(ctx, pending.Scope, final)
	if err != nil {
		return w.finishDeliveryError(ctx, scope, pending, err)
	}
	if err := w.markAccepted(ctx, scope, pending); err != nil {
		return w.finishDeliveryError(ctx, scope, pending, markLocalPersistenceError(err))
	}
	return w.finishAcceptedFinal(ctx, scope, pending.TaskID, final.ClientSummary.TaskTotalQuota, settlement)
}

func (w *Worker) markAccepted(ctx context.Context, scope modelbilling.TaskScope, report modelbilling.PendingReport) error {
	at, err := w.now()
	if err != nil {
		return err
	}
	return w.reports.MarkAccepted(ctx, scope, report.IdempotencyKey, report.AttemptCount, at)
}
func (w *Worker) reconcileUsagePending(ctx context.Context, task modelbilling.ModelTask, expectedSettlementID string) error {
	final, found, err := w.findFinal(ctx, task)
	if err != nil {
		return w.finishFinalLookupError(ctx, task, err)
	}
	if !found {
		if expectedSettlementID != "" {
			return contractError("settlement event requires one accepted final quota report")
		}
		return nil
	}
	if expectedSettlementID != "" && final.State != modelbilling.ReportAccepted {
		return contractError("settlement event requires an accepted final quota report")
	}
	if final.ContractVersion != modelbilling.QuotaContractV1 {
		return w.recoverUsagePending(ctx, task, localImmutableError(&modelbilling.Error{
			Code: modelbilling.ErrorLegacyReportContract, Message: "legacy final report requires manual recovery",
		}))
	}
	switch final.State {
	case modelbilling.ReportAccepted:
		return w.reconcileAcceptedFinal(ctx, task, final, expectedSettlementID)
	case modelbilling.ReportBlocked:
		_, err := w.transitionTask(ctx, task, modelbilling.TaskRecoveryRequired)
		return err
	case modelbilling.ReportPending, modelbilling.ReportRetry, modelbilling.ReportDelivering:
		return nil
	default:
		return markLocalPersistenceError(contractError("final report has an invalid reconciliation state"))
	}
}

func (w *Worker) finishFinalLookupError(
	ctx context.Context,
	task modelbilling.ModelTask,
	err error,
) error {
	if isDeterministicLocalPersistenceError(err) {
		return w.recoverUsagePending(ctx, task, err)
	}
	return err
}

func (w *Worker) reconcileAcceptedFinal(
	ctx context.Context,
	task modelbilling.ModelTask,
	final modelbilling.PendingReport,
	expectedSettlementID string,
) error {
	if err := validateAcceptedFinal(final, task); err != nil {
		return w.recoverUsagePending(ctx, task, err)
	}
	settling, err := w.transitionTask(ctx, task, modelbilling.TaskSettling)
	if err != nil {
		return err
	}
	return w.resumeSettling(ctx, settling, expectedSettlementID)
}

func (w *Worker) findFinal(ctx context.Context, task modelbilling.ModelTask) (modelbilling.PendingReport, bool, error) {
	reports, err := w.reports.ListByTask(ctx, task.Scope, task.TaskID)
	if err != nil {
		return modelbilling.PendingReport{}, false, markLocalPersistenceError(err)
	}
	var final modelbilling.PendingReport
	found := false
	for _, report := range reports {
		if report.Kind != modelbilling.ReportFinal {
			continue
		}
		if found {
			return modelbilling.PendingReport{}, false, markLocalPersistenceError(&modelbilling.Error{
				Code: modelbilling.ErrorIdempotencyConflict, Message: "task has multiple final quota reports",
			})
		}
		final, found = report, true
	}
	return final, found, nil
}

func validateAcceptedFinal(pending modelbilling.PendingReport, task modelbilling.ModelTask) error {
	decoded, err := decodePendingReport(pending)
	if err != nil {
		return localImmutableError(err)
	}
	if decoded.final == nil {
		return markLocalPersistenceError(contractError("accepted report is not a final quota report"))
	}
	return hydrateAndValidateFinalForTask(decoded.final, task)
}

func hydrateAndValidateFinalForTask(report *modelbilling.FinalQuotaReport, task modelbilling.ModelTask) error {
	if report == nil || task.Result == nil || task.EndedAt == nil || task.Scope.CloudBinding == nil {
		return localImmutableError(contractError("final quota report has incomplete persisted task facts"))
	}
	report.Scope = *task.Scope.CloudBinding
	report.TaskResult = *task.Result
	report.CreatedAt = task.EndedAt.UTC()
	return localImmutableError(report.ValidateForTask(task))
}

func localImmutableError(err error) error {
	return markLocalPersistenceError(err)
}

type localPersistenceError struct {
	cause error
}

func (e *localPersistenceError) Error() string {
	if e == nil || e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func (e *localPersistenceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func markLocalPersistenceError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if _, marked := localPersistenceCause(err); marked || !hasSingleErrorCause(err) {
		return err
	}
	if !isDeterministicPersistenceCode(modelbilling.ErrorCodeOf(err)) {
		return err
	}
	return &localPersistenceError{cause: err}
}

func isDeterministicLocalPersistenceError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	cause, marked := localPersistenceCause(err)
	if !marked || !hasSingleErrorCause(cause) {
		return false
	}
	return isDeterministicPersistenceCode(modelbilling.ErrorCodeOf(cause))
}

func isDeterministicPersistenceCode(code modelbilling.ErrorCode) bool {
	switch code {
	case modelbilling.ErrorContractInvalid, modelbilling.ErrorScopeMismatch,
		modelbilling.ErrorIdempotencyConflict, modelbilling.ErrorBillingSourceMismatch,
		modelbilling.ErrorLegacyReportContract, modelbilling.ErrorQuotaCallIDConflict,
		modelbilling.ErrorQuotaRequestIDConflict, modelbilling.ErrorQuotaRequestIDReused,
		modelbilling.ErrorQuotaSummaryMismatch, modelbilling.ErrorQuotaFinalPayloadConflict,
		modelbilling.ErrorQuotaFinalIdempotencyConflict, modelbilling.ErrorQuotaPartialSnapshotConflict,
		modelbilling.ErrorQuotaTimeInvalid, modelbilling.ErrorSettlementBalanceInconsistent:
		return true
	default:
		return false
	}
}

func localPersistenceCause(err error) (error, bool) {
	for err != nil {
		if marked, ok := err.(*localPersistenceError); ok {
			return marked.cause, true
		}
		switch wrapped := err.(type) {
		case interface{ Unwrap() []error }:
			return nil, false
		case interface{ Unwrap() error }:
			err = wrapped.Unwrap()
		default:
			return nil, false
		}
	}
	return nil, false
}

func hasSingleErrorCause(err error) bool {
	for err != nil {
		switch wrapped := err.(type) {
		case interface{ Unwrap() []error }:
			return false
		case interface{ Unwrap() error }:
			err = wrapped.Unwrap()
		default:
			return true
		}
	}
	return true
}

func (w *Worker) recoverUsagePending(ctx context.Context, task modelbilling.ModelTask, cause error) error {
	if _, err := w.transitionTask(ctx, task, modelbilling.TaskRecoveryRequired); err != nil {
		return errors.Join(cause, fmt.Errorf("persist usage_pending recovery: %w", err))
	}
	return cause
}

func (w *Worker) finishAcceptedFinal(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
	expectedTotal modelbilling.DecimalString,
	settlement modelbilling.QuotaSettlementResult,
) error {
	task, err := w.tasks.Get(ctx, scope, taskID)
	if err != nil {
		return markLocalPersistenceError(err)
	}
	if err := validateWorkerTask(task, scope, taskID); err != nil {
		return markLocalPersistenceError(err)
	}
	switch task.State {
	case modelbilling.TaskUsagePending:
		task, err = w.transitionTask(ctx, task, modelbilling.TaskSettling)
		if err != nil {
			return err
		}
	case modelbilling.TaskSettling:
	case modelbilling.TaskCompleted:
		return nil
	default:
		return markLocalPersistenceError(contractError("accepted final task is not ready for settlement"))
	}
	if task.State == modelbilling.TaskCompleted {
		return nil
	}
	if task.State != modelbilling.TaskSettling {
		return markLocalPersistenceError(contractError("accepted final transition did not leave task settling"))
	}
	return w.applySettlement(ctx, task, expectedTotal, settlement)
}

func (w *Worker) querySettlement(ctx context.Context, task modelbilling.ModelTask, expectedSettlementID string) error {
	done, err := validateSettlementLookupState(task)
	if err != nil || done {
		return err
	}
	expectedTotal, err := w.acceptedFinalTaskTotal(ctx, task)
	if err != nil {
		return err
	}
	cached, found, err := w.cachedSettlement(ctx, task)
	if err != nil {
		return err
	}
	handled, err := w.applyCachedSettlementIfTerminal(
		ctx, task, expectedTotal, cached, found, expectedSettlementID,
	)
	if err != nil || handled {
		return err
	}
	settlement, err := w.cloud.GetQuotaSettlement(ctx, *task.Scope.CloudBinding, task.TaskID)
	if err != nil {
		return w.recordSettlementQueryError(ctx, task, err)
	}
	return w.applyExpectedSettlement(ctx, task, expectedTotal, settlement, expectedSettlementID)
}

func (w *Worker) applyCachedSettlementIfTerminal(
	ctx context.Context,
	task modelbilling.ModelTask,
	expectedTotal modelbilling.DecimalString,
	cached modelbilling.QuotaSettlementResult,
	found bool,
	expectedSettlementID string,
) (bool, error) {
	if !found {
		return false, nil
	}
	if expectedSettlementID != "" && cached.Settlement.SettlementID != expectedSettlementID {
		return true, contractError("settlement ID does not match event identity")
	}
	if cached.Settlement.Status == modelbilling.QuotaSettlementPending {
		return false, nil
	}
	return true, w.applyExpectedSettlement(ctx, task, expectedTotal, cached, expectedSettlementID)
}

func (w *Worker) applyExpectedSettlement(
	ctx context.Context,
	task modelbilling.ModelTask,
	expectedTotal modelbilling.DecimalString,
	result modelbilling.QuotaSettlementResult,
	expectedSettlementID string,
) error {
	if expectedSettlementID != "" && result.Settlement.SettlementID != expectedSettlementID {
		return contractError("settlement ID does not match event identity")
	}
	return w.applySettlement(ctx, task, expectedTotal, result)
}

func validateSettlementLookupState(task modelbilling.ModelTask) (bool, error) {
	if task.State == modelbilling.TaskCompleted {
		return true, nil
	}
	if task.State != modelbilling.TaskSettling {
		return false, markLocalPersistenceError(contractError("settlement lookup requires a settling task"))
	}
	return false, nil
}

func (w *Worker) cachedSettlement(
	ctx context.Context,
	task modelbilling.ModelTask,
) (modelbilling.QuotaSettlementResult, bool, error) {
	if w.settlements == nil {
		return modelbilling.QuotaSettlementResult{}, false, nil
	}
	cached, found, err := w.settlements.CurrentQuotaSettlement(ctx, task.Scope, task.TaskID)
	if err != nil {
		return modelbilling.QuotaSettlementResult{}, false, markLocalPersistenceError(err)
	}
	return cached, found, nil
}

func (w *Worker) recordSettlementQueryError(
	ctx context.Context,
	task modelbilling.ModelTask,
	queryErr error,
) error {
	code := modelbilling.ErrorCodeOf(queryErr)
	if code == "" {
		code = modelbilling.ErrorSettlementException
	}
	stage := modelbilling.QuotaRetrySettlementQuery
	text := string(code)
	if err := w.updateQuotaWorkflow(ctx, task, modelbilling.QuotaWorkflowRetryPending, &stage, &text); err != nil {
		return errors.Join(queryErr, fmt.Errorf("mark settlement query retry: %w", err))
	}
	return queryErr
}

func (w *Worker) acceptedFinalTaskTotal(
	ctx context.Context,
	task modelbilling.ModelTask,
) (modelbilling.DecimalString, error) {
	final, found, err := w.findFinal(ctx, task)
	if err != nil {
		return "", err
	}
	if !found || final.State != modelbilling.ReportAccepted {
		return "", markLocalPersistenceError(contractError("settling task requires one accepted final quota report"))
	}
	decoded, err := decodePendingReport(final)
	if err != nil || decoded.final == nil {
		if err == nil {
			err = contractError("accepted report is not a final quota report")
		}
		return "", localImmutableError(err)
	}
	if err := hydrateAndValidateFinalForTask(decoded.final, task); err != nil {
		return "", err
	}
	return decoded.final.ClientSummary.TaskTotalQuota, nil
}

func (w *Worker) resumeSettling(ctx context.Context, task modelbilling.ModelTask, expectedSettlementID string) error {
	switch task.State {
	case modelbilling.TaskSettling:
		return w.querySettlement(ctx, task, expectedSettlementID)
	case modelbilling.TaskCompleted:
		return nil
	default:
		return markLocalPersistenceError(contractError("settlement transition returned an invalid task state"))
	}
}

func (w *Worker) applySettlement(
	ctx context.Context,
	task modelbilling.ModelTask,
	expectedTotal modelbilling.DecimalString,
	result modelbilling.QuotaSettlementResult,
) error {
	settlement, err := validateSettlementResult(task, expectedTotal, result)
	if err != nil {
		return w.settlementValidationFailed(ctx, task, err)
	}
	if err := w.persistSettlement(ctx, task, result); err != nil {
		return err
	}
	return w.applySettlementState(ctx, task, settlement)
}

func validateSettlementResult(
	task modelbilling.ModelTask,
	expectedTotal modelbilling.DecimalString,
	result modelbilling.QuotaSettlementResult,
) (modelbilling.QuotaSettlement, error) {
	if err := result.Validate(); err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	settlement := result.Settlement
	if err := settlement.ValidateTaskTotalQuota(expectedTotal); err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	if err := settlement.ValidateForTask(task); err != nil {
		return modelbilling.QuotaSettlement{}, err
	}
	return settlement, nil
}

func (w *Worker) persistSettlement(
	ctx context.Context,
	task modelbilling.ModelTask,
	result modelbilling.QuotaSettlementResult,
) error {
	if w.settlements == nil {
		return nil
	}
	at, err := w.now()
	if err != nil {
		return err
	}
	if err := w.settlements.SaveQuotaSettlement(ctx, task.Scope, result, at); err != nil {
		return markLocalPersistenceError(err)
	}
	return nil
}

func (w *Worker) applySettlementState(
	ctx context.Context,
	task modelbilling.ModelTask,
	settlement modelbilling.QuotaSettlement,
) error {
	switch settlement.Status {
	case modelbilling.QuotaSettlementPending:
		return w.updateQuotaWorkflow(ctx, task, modelbilling.QuotaWorkflowSettlementPending, nil, nil)
	case modelbilling.QuotaSettlementSettled, modelbilling.QuotaSettlementSettledUncovered:
		return w.completeSettledTask(ctx, task, settlement)
	case modelbilling.QuotaSettlementManualRequired:
		return w.requireManualSettlementRecovery(ctx, task)
	default:
		return contractError("unsupported normalized settlement state")
	}
}

func (w *Worker) completeSettledTask(
	ctx context.Context,
	task modelbilling.ModelTask,
	settlement modelbilling.QuotaSettlement,
) error {
	if err := w.observer.CreditSettlementObserved(
		ctx, *task.Scope.CloudBinding, task.TaskID, settlement.SettlementID,
	); err != nil {
		return err
	}
	status := modelbilling.QuotaWorkflowSettled
	if settlement.Status == modelbilling.QuotaSettlementSettledUncovered {
		status = modelbilling.QuotaWorkflowSettledWithUncovered
	}
	if err := w.updateQuotaWorkflow(ctx, task, status, nil, nil); err != nil {
		return err
	}
	_, err := w.transitionTask(ctx, task, modelbilling.TaskCompleted)
	return err
}

func (w *Worker) requireManualSettlementRecovery(ctx context.Context, task modelbilling.ModelTask) error {
	code := string(modelbilling.ErrorSettlementException)
	if err := w.updateQuotaWorkflow(
		ctx, task, modelbilling.QuotaWorkflowFailedManualRequired, nil, &code,
	); err != nil {
		return err
	}
	_, err := w.transitionTask(ctx, task, modelbilling.TaskRecoveryRequired)
	return err
}

func (w *Worker) settlementValidationFailed(
	ctx context.Context,
	task modelbilling.ModelTask,
	cause error,
) error {
	stage := modelbilling.QuotaRetrySettlementQuery
	code := modelbilling.ErrorCodeOf(cause)
	if code == "" {
		code = modelbilling.ErrorSettlementBalanceInconsistent
	}
	text := string(code)
	if err := w.updateQuotaWorkflow(ctx, task, modelbilling.QuotaWorkflowRetryPending, &stage, &text); err != nil {
		return errors.Join(cause, fmt.Errorf("mark invalid settlement for REST recovery: %w", err))
	}
	return cause
}

func (w *Worker) transitionTask(
	ctx context.Context,
	task modelbilling.ModelTask,
	to modelbilling.TaskState,
) (modelbilling.ModelTask, error) {
	at, err := w.now()
	if err != nil {
		return modelbilling.ModelTask{}, err
	}
	update := modelbilling.TaskStateUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: task.State, To: to,
		Result: task.Result, LastErrorCode: task.LastErrorCode, At: at,
	}
	if err := w.tasks.UpdateState(ctx, update); err != nil {
		return modelbilling.ModelTask{}, err
	}
	updated, err := w.tasks.Get(ctx, task.Scope, task.TaskID)
	if err != nil {
		return modelbilling.ModelTask{}, markLocalPersistenceError(err)
	}
	return updated, nil
}

func (w *Worker) updateQuotaWorkflow(
	ctx context.Context,
	task modelbilling.ModelTask,
	to modelbilling.QuotaWorkflowStatus,
	retryStage *modelbilling.QuotaRetryStage,
	lastErrorCode *string,
) error {
	if w.workflow == nil {
		return nil
	}
	current, err := w.workflow.Get(ctx, task.Scope, task.TaskID)
	if err != nil {
		return markLocalPersistenceError(err)
	}
	at, err := w.now()
	if err != nil {
		return err
	}
	return w.workflow.Update(ctx, modelbilling.QuotaWorkflowUpdate{
		Scope: task.Scope, TaskID: task.TaskID, From: current.Status, To: to,
		RetryStage: retryStage, LastErrorCode: lastErrorCode, At: at.UTC(),
	})
}

func (w *Worker) now() (time.Time, error) {
	now := w.retry.Clock()
	if now.IsZero() {
		return time.Time{}, contractError("retry clock returned zero time")
	}
	return now, nil
}

func validateSubscriptionScope(scope modelbilling.TaskScope) error {
	if scope.CloudBinding == nil {
		return &modelbilling.Error{
			Code: modelbilling.ErrorBillingSourceMismatch, Message: "quota worker only accepts subscription scopes",
		}
	}
	return scope.Validate(modelbilling.BillingSubscriptionPool)
}

func validateClaimedReport(scope modelbilling.TaskScope, report modelbilling.PendingReport) error {
	if report.State != modelbilling.ReportDelivering || report.AttemptCount <= 0 {
		return contractError("claimed report is not an owned delivery attempt")
	}
	if scope.CloudBinding == nil || report.Scope != *scope.CloudBinding {
		return contractError("claimed report belongs to another scope")
	}
	return nil
}

func validateWorkerTask(task modelbilling.ModelTask, scope modelbilling.TaskScope, taskID string) error {
	if task.BillingSource != modelbilling.BillingSubscriptionPool {
		return &modelbilling.Error{Code: modelbilling.ErrorBillingSourceMismatch, Message: "quota worker only accepts subscription tasks"}
	}
	if task.TaskID != taskID || !task.Scope.Equal(scope) {
		return contractError("task identity does not match reconciliation scope")
	}
	if task.Scope.CloudBinding == nil {
		return contractError("subscription task has no cloud binding")
	}
	return nil
}

func contractError(message string) error {
	return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: message}
}
