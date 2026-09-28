package modelbillingworker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"kwu/internal/modelbilling"
)

// QuotaSupplementRequest is already normalized. Provider URL, auth, wire
// parsing, and rounding deliberately stay outside this local recovery seam.
type QuotaSupplementRequest struct {
	Scope           modelbilling.TaskScope
	TaskID          string
	CallID          string
	Endpoint        modelbilling.MaasEndpoint
	CurrentEvidence modelbilling.QuotaEvidenceRevision
}

func (r QuotaSupplementRequest) Validate() error {
	if err := r.Scope.Validate(modelbilling.BillingSubscriptionPool); err != nil {
		return err
	}
	if strings.TrimSpace(r.TaskID) != r.TaskID || r.TaskID == "" ||
		strings.TrimSpace(r.CallID) != r.CallID || r.CallID == "" {
		return contractError("quota supplement requires canonical task and call ids")
	}
	if err := r.Endpoint.Validate(); err != nil {
		return err
	}
	return r.CurrentEvidence.Validate()
}

type NormalizedQuotaSupplementer interface {
	Supplement(context.Context, QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error)
}

type QuotaSupplementWorkflowStore interface {
	ListIDs(context.Context, modelbilling.TaskScope, ...modelbilling.QuotaWorkflowStatus) ([]string, error)
	Get(context.Context, modelbilling.TaskScope, string) (modelbilling.QuotaWorkflow, error)
	Update(context.Context, modelbilling.QuotaWorkflowUpdate) error
	MarkSupplementFailedManualRequired(context.Context, modelbilling.TaskScope, string, time.Time, string) error
}

type QuotaSupplementUsageReader interface {
	ReadUsageInput(context.Context, modelbilling.TaskScope, string) (modelbilling.SubscriptionUsageInput, error)
}

type QuotaSupplementEvidenceStore interface {
	Current(context.Context, modelbilling.TaskScope, string, string) (modelbilling.QuotaEvidenceRevision, bool, error)
	Append(context.Context, modelbilling.TaskScope, string, string, modelbilling.QuotaEvidenceRevision) error
}

// QuotaSupplementRunner owns only the evidence_pending/supplementing slice of
// the durable quota workflow. Final identity and queue insertion remain owned
// by FinalReportEnqueuer.
type QuotaSupplementRunner struct {
	workflows  QuotaSupplementWorkflowStore
	usage      QuotaSupplementUsageReader
	evidence   QuotaSupplementEvidenceStore
	reports    FinalReportEnqueuer
	supplement NormalizedQuotaSupplementer
	clock      func() time.Time
	runMu      sync.Mutex
}

func NewQuotaSupplementRunner(
	workflows QuotaSupplementWorkflowStore,
	usage QuotaSupplementUsageReader,
	evidence QuotaSupplementEvidenceStore,
	reports FinalReportEnqueuer,
	supplement NormalizedQuotaSupplementer,
	clock func() time.Time,
) (*QuotaSupplementRunner, error) {
	if workflows == nil || usage == nil || evidence == nil || reports == nil || clock == nil {
		return nil, contractError("quota supplement stores, report enqueuer, and clock are required")
	}
	return &QuotaSupplementRunner{
		workflows: workflows, usage: usage, evidence: evidence, reports: reports,
		supplement: supplement, clock: clock,
	}, nil
}

// RunOnce returns per-task recoveries as warnings and reserves the fatal error
// for scan/cancellation failures. A nil supplementer is an explicit WAIT state:
// it performs no scan and leaves durable workflows untouched.
func (r *QuotaSupplementRunner) RunOnce(
	ctx context.Context,
	scope modelbilling.TaskScope,
) ([]error, error) {
	if r == nil {
		return nil, contractError("quota supplement runner is required")
	}
	r.runMu.Lock()
	defer r.runMu.Unlock()
	if err := scope.Validate(modelbilling.BillingSubscriptionPool); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.supplement == nil {
		return nil, nil
	}
	ids, err := r.workflows.ListIDs(
		ctx, scope, modelbilling.QuotaWorkflowEvidencePending, modelbilling.QuotaWorkflowSupplementing,
	)
	if err != nil {
		return nil, fmt.Errorf("scan quota supplement workflows: %w", err)
	}
	return r.recoverIDs(ctx, scope, ids)
}

func (r *QuotaSupplementRunner) recoverIDs(
	ctx context.Context,
	scope modelbilling.TaskScope,
	ids []string,
) ([]error, error) {
	warnings := make([]error, 0)
	for _, taskID := range ids {
		if err := ctx.Err(); err != nil {
			return warnings, err
		}
		if err := r.recoverTask(ctx, scope, taskID); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return warnings, errors.Join(err, ctxErr)
			}
			warnings = append(warnings, fmt.Errorf("supplement quota task %q: %w", taskID, err))
		}
	}
	return warnings, nil
}

func (r *QuotaSupplementRunner) recoverTask(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
) error {
	workflow, err := r.beginTask(ctx, scope, taskID)
	if err != nil {
		return err
	}
	input, err := r.usage.ReadUsageInput(ctx, scope, taskID)
	if err != nil {
		return r.finishFailure(ctx, workflow, err, supplementFailureIsManual(err))
	}
	if err := validateSupplementUsageInput(input, scope, taskID); err != nil {
		return r.finishFailure(ctx, workflow, err, true)
	}
	if err := r.supplementCalls(ctx, scope, input); err != nil {
		return r.finishFailure(ctx, workflow, err, supplementFailureIsManual(err))
	}
	if err := r.enqueueSupplementedFinal(ctx, scope, taskID); err != nil {
		return r.finishFailure(ctx, workflow, err, supplementFailureIsManual(err))
	}
	return nil
}

func (r *QuotaSupplementRunner) beginTask(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
) (modelbilling.QuotaWorkflow, error) {
	workflow, err := r.workflows.Get(ctx, scope, taskID)
	if err != nil {
		return modelbilling.QuotaWorkflow{}, r.quarantineUnreadableWorkflow(ctx, scope, taskID, err)
	}
	if workflow.Status == modelbilling.QuotaWorkflowEvidencePending {
		if err := r.updateWorkflow(ctx, workflow, modelbilling.QuotaWorkflowSupplementing, nil); err != nil {
			return modelbilling.QuotaWorkflow{}, err
		}
		workflow.Status = modelbilling.QuotaWorkflowSupplementing
		workflow.LastErrorCode = nil
	} else if workflow.Status != modelbilling.QuotaWorkflowSupplementing {
		return modelbilling.QuotaWorkflow{}, contractError("quota supplement scanner returned an invalid workflow state")
	}
	return workflow, nil
}

func (r *QuotaSupplementRunner) quarantineUnreadableWorkflow(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
	cause error,
) error {
	localErr := markLocalPersistenceError(cause)
	if !isDeterministicLocalPersistenceError(localErr) {
		return cause
	}
	at := r.clock()
	if at.IsZero() {
		return errors.Join(cause, contractError("quota supplement clock returned zero time"))
	}
	code := modelbilling.ErrorCodeOf(localErr)
	if err := r.workflows.MarkSupplementFailedManualRequired(
		ctx, scope, taskID, at.UTC(), string(code),
	); err != nil {
		return errors.Join(cause, fmt.Errorf("quarantine quota supplement workflow: %w", err))
	}
	return cause
}

func validateSupplementUsageInput(
	input modelbilling.SubscriptionUsageInput,
	scope modelbilling.TaskScope,
	taskID string,
) error {
	if input.Task.TaskID != taskID || !input.Task.Scope.Equal(scope) ||
		input.Task.BillingSource != modelbilling.BillingSubscriptionPool {
		return contractError("quota supplement usage input identity mismatch")
	}
	return nil
}

func (r *QuotaSupplementRunner) supplementCalls(
	ctx context.Context,
	scope modelbilling.TaskScope,
	input modelbilling.SubscriptionUsageInput,
) error {
	for _, item := range input.Calls {
		if !terminalSupplementCall(item.Call.State) {
			continue
		}
		if err := r.supplementCall(ctx, scope, input.Task, item.Call); err != nil {
			return err
		}
	}
	return nil
}

func (r *QuotaSupplementRunner) enqueueSupplementedFinal(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
) error {
	build, pending, err := r.reports.EnqueueFinal(ctx, scope, taskID)
	if err != nil {
		return err
	}
	if build.Disposition != modelbilling.ReportBuildDeliverable || pending == nil {
		return contractError("supplemented quota evidence did not produce one final report")
	}
	return nil
}

func (r *QuotaSupplementRunner) supplementCall(
	ctx context.Context,
	scope modelbilling.TaskScope,
	task modelbilling.ModelTask,
	call modelbilling.ModelCall,
) error {
	current, found, err := r.evidence.Current(ctx, scope, task.TaskID, call.CallID)
	if err != nil {
		return err
	}
	if !found {
		return contractError("terminal quota call is missing its initial evidence revision")
	}
	if current.Status == modelbilling.QuotaEvidenceComplete {
		return nil
	}
	request := QuotaSupplementRequest{
		Scope: scope, TaskID: task.TaskID, CallID: call.CallID,
		Endpoint: call.Endpoint, CurrentEvidence: current,
	}
	if err := request.Validate(); err != nil {
		return err
	}
	next, err := r.supplement.Supplement(ctx, request)
	if err != nil {
		return err
	}
	if err := validateSupplementResponse(request, next); err != nil {
		return err
	}
	return r.evidence.Append(ctx, scope, task.TaskID, call.CallID, next)
}

func validateSupplementResponse(
	request QuotaSupplementRequest,
	next modelbilling.QuotaEvidenceRevision,
) error {
	if err := next.Validate(); err != nil {
		return err
	}
	if next.Status != modelbilling.QuotaEvidenceComplete ||
		next.Revision != request.CurrentEvidence.Revision+1 ||
		next.ObservedAt.Before(request.CurrentEvidence.ObservedAt) {
		return contractError("quota supplement response must append one complete evidence revision")
	}
	currentID, nextID := request.CurrentEvidence.OneAPIRequestID, next.OneAPIRequestID
	if currentID != nil && (nextID == nil || *currentID != *nextID) {
		return contractError("quota supplement response changed the Maas request identity")
	}
	return nil
}

func (r *QuotaSupplementRunner) finishFailure(
	ctx context.Context,
	workflow modelbilling.QuotaWorkflow,
	cause error,
	manual bool,
) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(cause, ctxErr)
	}
	target := modelbilling.QuotaWorkflowEvidencePending
	code := modelbilling.ErrorCodeOf(cause)
	if manual {
		target = modelbilling.QuotaWorkflowFailedManualRequired
		if code == "" {
			code = modelbilling.ErrorContractInvalid
		}
	} else if code == "" {
		code = modelbilling.ErrorUsageRetryable
	}
	value := string(code)
	if err := r.updateWorkflow(ctx, workflow, target, &value); err != nil {
		return errors.Join(cause, fmt.Errorf("persist quota supplement failure: %w", err))
	}
	return cause
}

func (r *QuotaSupplementRunner) updateWorkflow(
	ctx context.Context,
	workflow modelbilling.QuotaWorkflow,
	target modelbilling.QuotaWorkflowStatus,
	lastError *string,
) error {
	at := r.clock()
	if at.IsZero() {
		return contractError("quota supplement clock returned zero time")
	}
	if at.Before(workflow.UpdatedAt) {
		at = workflow.UpdatedAt
	}
	return r.workflows.Update(ctx, modelbilling.QuotaWorkflowUpdate{
		Scope: workflow.Scope, TaskID: workflow.TaskID, From: workflow.Status, To: target,
		LastErrorCode: lastError, At: at.UTC(),
	})
}

func terminalSupplementCall(state modelbilling.CallState) bool {
	switch state {
	case modelbilling.CallCompleted, modelbilling.CallFailed, modelbilling.CallCancelled,
		modelbilling.CallInterrupted, modelbilling.CallUsageMissing:
		return true
	default:
		return false
	}
}

func supplementFailureIsManual(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var domainErr *modelbilling.Error
	if errors.As(err, &domainErr) && domainErr.Retryable {
		return false
	}
	switch modelbilling.ErrorCodeOf(err) {
	case modelbilling.ErrorAuthRequired, modelbilling.ErrorSessionExpired,
		modelbilling.ErrorScopeMismatch, modelbilling.ErrorGatewayUnavailable,
		modelbilling.ErrorUsageRetryable, modelbilling.ErrorUsagePersistFailed,
		modelbilling.ErrorInvalidStateTransition, modelbilling.ErrorIdempotencyConflict:
		return false
	case modelbilling.ErrorContractInvalid, modelbilling.ErrorBillingSourceMismatch,
		modelbilling.ErrorQuotaEvidenceIncomplete, modelbilling.ErrorQuotaCallIDConflict,
		modelbilling.ErrorQuotaRequestIDConflict:
		return true
	default:
		return false
	}
}

var _ QuotaSupplementWorkflowStore = (*modelbilling.SQLiteQuotaWorkflowStore)(nil)
var _ QuotaSupplementUsageReader = (*modelbilling.SQLiteSubscriptionUsageReader)(nil)
var _ QuotaSupplementEvidenceStore = (*modelbilling.SQLiteQuotaEvidenceStore)(nil)
