package modelbillingworker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

// CoordinatorConfig has no production defaults. Its composition root owns
// polling, operation bounds, and delivery batch sizing.
type CoordinatorConfig struct {
	PollInterval         time.Duration
	OperationTimeout     time.Duration
	MaxDeliveriesPerPass int
	Clock                func() time.Time
	OnError              func(error)
	Supplement           QuotaSupplementPhase
	FreezeReleaser       cloud.CloudCreditFreezeReleaser
}

type QuotaSupplementPhase interface {
	RunOnce(context.Context, modelbilling.TaskScope) ([]error, error)
}

type UsagePendingIDScanner interface {
	ListUsagePendingIDs(context.Context, modelbilling.TaskScope) ([]string, error)
}

type FinalReportEnqueuer interface {
	EnqueueFinal(context.Context, modelbilling.TaskScope, string) (modelbilling.UsageReportBuildResult, *modelbilling.PendingReport, error)
}

type CoordinatorTaskStore interface {
	ListSettlingIDs(context.Context, modelbilling.TaskScope) ([]string, error)
	MarkRecoveryRequired(context.Context, modelbilling.TaskScope, string, modelbilling.TaskState, time.Time) error
}

type CoordinatorWorker interface {
	DeliverOne(context.Context, modelbilling.TaskScope) (bool, error)
	ReconcileTask(context.Context, modelbilling.TaskScope, string) error
}

type Coordinator struct {
	usage   UsagePendingIDScanner
	reports FinalReportEnqueuer
	tasks   CoordinatorTaskStore
	worker  CoordinatorWorker
	config  CoordinatorConfig

	suppressionMu sync.Mutex
	noReport      map[noReportKey]struct{}
}

type noReportKey struct {
	actorID           string
	profileID         string
	cloudUserID       string
	bindingGeneration uint64
	taskID            string
}

func NewCoordinator(
	usage UsagePendingIDScanner,
	reports FinalReportEnqueuer,
	tasks CoordinatorTaskStore,
	worker CoordinatorWorker,
	config CoordinatorConfig,
) (*Coordinator, error) {
	if usage == nil || reports == nil || tasks == nil || worker == nil {
		return nil, contractError("coordinator usage scanner, report store, task store, and worker are required")
	}
	if config.PollInterval <= 0 || config.OperationTimeout <= 0 || config.MaxDeliveriesPerPass <= 0 || config.Clock == nil {
		return nil, contractError("coordinator polling, operation timeout, delivery limit, and clock are required")
	}
	return &Coordinator{
		usage: usage, reports: reports, tasks: tasks, worker: worker, config: config,
		noReport: make(map[noReportKey]struct{}),
	}, nil
}

// Run recovers immediately and then repeats at the configured interval. Pass
// errors are reported but do not stop later recovery attempts.
func (c *Coordinator) Run(ctx context.Context, scope modelbilling.TaskScope) error {
	if err := validateSubscriptionScope(scope); err != nil {
		return err
	}
	if err := c.RunOnce(ctx, scope); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.reportError(err)
	}
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.RunOnce(ctx, scope); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				c.reportError(err)
			}
		}
	}
}

// RunOnce performs settling recovery, final construction, optional evidence
// supplementation, then bounded queue delivery. Locally recovered errors are
// retained while phase-fatal errors stop the pass immediately.
func (c *Coordinator) RunOnce(ctx context.Context, scope modelbilling.TaskScope) error {
	if err := validateSubscriptionScope(scope); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var passErrs []error
	warnings, err := c.recoverSettling(ctx, scope)
	passErrs = append(passErrs, warnings...)
	if err != nil {
		return joinCoordinatorErrors(passErrs, err)
	}
	warnings, err = c.recoverUsagePending(ctx, scope)
	passErrs = append(passErrs, warnings...)
	if err != nil {
		return joinCoordinatorErrors(passErrs, err)
	}
	if c.config.Supplement != nil {
		warnings, err = c.config.Supplement.RunOnce(ctx, scope)
		passErrs = append(passErrs, warnings...)
		if err != nil {
			return joinCoordinatorErrors(passErrs, err)
		}
	}
	warnings, err = c.deliver(ctx, scope)
	passErrs = append(passErrs, warnings...)
	if err != nil {
		return joinCoordinatorErrors(passErrs, err)
	}
	return errors.Join(passErrs...)
}

func (c *Coordinator) recoverSettling(ctx context.Context, scope modelbilling.TaskScope) ([]error, error) {
	var ids []string
	err := c.operation(ctx, func(operationCtx context.Context) error {
		var scanErr error
		ids, scanErr = c.tasks.ListSettlingIDs(operationCtx, scope)
		return scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("scan settling tasks: %w", err)
	}
	var warnings []error
	for _, taskID := range ids {
		if err := ctx.Err(); err != nil {
			return warnings, err
		}
		err := c.operation(ctx, func(operationCtx context.Context) error {
			return c.worker.ReconcileTask(operationCtx, scope, taskID)
		})
		if err == nil {
			continue
		}
		recovered, taskErr := c.recoverDeterministicError(
			ctx, scope, taskID, err, modelbilling.TaskSettling,
		)
		taskErr = fmt.Errorf("reconcile settling task %q: %w", taskID, taskErr)
		if !recovered {
			if ctx.Err() != nil ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) ||
				isDeterministicLocalPersistenceError(err) {
				return warnings, taskErr
			}
			// A malformed or temporarily unavailable settlement belongs to this
			// task. Keep it settling and continue the pass so one poisoned task
			// cannot starve later usage reports, releases, and settlements.
			warnings = append(warnings, taskErr)
			continue
		}
		warnings = append(warnings, taskErr)
	}
	return warnings, nil
}

func (c *Coordinator) recoverUsagePending(ctx context.Context, scope modelbilling.TaskScope) ([]error, error) {
	var ids []string
	err := c.operation(ctx, func(operationCtx context.Context) error {
		var scanErr error
		ids, scanErr = c.usage.ListUsagePendingIDs(operationCtx, scope)
		return scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("scan usage_pending tasks: %w", err)
	}
	var warnings []error
	for _, taskID := range ids {
		if err := ctx.Err(); err != nil {
			return warnings, err
		}
		recovered, err := c.recoverUsagePendingTask(ctx, scope, taskID)
		if err == nil {
			continue
		}
		if !recovered {
			return warnings, err
		}
		warnings = append(warnings, err)
	}
	return warnings, nil
}

func (c *Coordinator) recoverUsagePendingTask(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
) (bool, error) {
	key := makeNoReportKey(scope, taskID)
	if c.noReportSuppressed(key) {
		return false, nil
	}
	build, err := c.enqueueFinal(ctx, scope, taskID)
	if err != nil {
		recovered, recoveryErr := c.recoverDeterministicError(
			ctx, scope, taskID, err, modelbilling.TaskUsagePending,
		)
		return recovered, fmt.Errorf("enqueue final for task %q: %w", taskID, recoveryErr)
	}
	switch build.Disposition {
	case modelbilling.ReportBuildNoReportRequired:
		c.suppressNoReport(key)
		return false, nil
	case modelbilling.ReportBuildEvidencePending:
		// Evidence supplementation is an independent quota workflow. No final
		// row exists yet, so settlement reconciliation must not run.
		return false, nil
	case modelbilling.ReportBuildDeliverable, modelbilling.ReportBuildBlocked:
	default:
		err := markLocalPersistenceError(contractError("final build returned an invalid disposition"))
		recovered, recoveryErr := c.recoverDeterministicError(
			ctx, scope, taskID, err, modelbilling.TaskUsagePending,
		)
		return recovered, fmt.Errorf("enqueue final for task %q: %w", taskID, recoveryErr)
	}
	err = c.operation(ctx, func(operationCtx context.Context) error {
		return c.worker.ReconcileTask(operationCtx, scope, taskID)
	})
	if err == nil {
		return false, nil
	}
	recovered, recoveryErr := c.recoverDeterministicError(
		ctx, scope, taskID, err, modelbilling.TaskUsagePending, modelbilling.TaskSettling,
	)
	return recovered, fmt.Errorf("reconcile usage_pending task %q: %w", taskID, recoveryErr)
}

// RecoverUsagePendingTask performs the same durable, usage-aware recovery as
// the background scan for one freshly failed subscription task.
func (c *Coordinator) RecoverUsagePendingTask(ctx context.Context, scope modelbilling.TaskScope, taskID string) error {
	if c == nil {
		return contractError("usage-pending coordinator is required")
	}
	if err := scope.Validate(modelbilling.BillingSubscriptionPool); err != nil {
		return err
	}
	if strings.TrimSpace(taskID) == "" {
		return contractError("usage-pending task id is required")
	}
	_, err := c.recoverUsagePendingTask(ctx, scope, taskID)
	return err
}

func (c *Coordinator) enqueueFinal(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
) (modelbilling.UsageReportBuildResult, error) {
	var build modelbilling.UsageReportBuildResult
	err := c.operation(ctx, func(operationCtx context.Context) error {
		var enqueueErr error
		build, _, enqueueErr = c.reports.EnqueueFinal(operationCtx, scope, taskID)
		return enqueueErr
	})
	return build, markLocalPersistenceError(err)
}

func (c *Coordinator) recoverDeterministicError(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
	err error,
	states ...modelbilling.TaskState,
) (bool, error) {
	if contextErr := ctx.Err(); contextErr != nil {
		return false, errors.Join(err, contextErr)
	}
	if !isDeterministicLocalPersistenceError(err) {
		return false, err
	}
	if recoveryErr := c.recoverTask(ctx, scope, taskID, states...); recoveryErr != nil {
		return false, errors.Join(err, fmt.Errorf("mark task recovery_required: %w", recoveryErr))
	}
	return true, err
}

func (c *Coordinator) deliver(ctx context.Context, scope modelbilling.TaskScope) ([]error, error) {
	var warnings []error
	for attempt := 0; attempt < c.config.MaxDeliveriesPerPass; attempt++ {
		if err := ctx.Err(); err != nil {
			return warnings, err
		}
		var found bool
		err := c.operation(ctx, func(operationCtx context.Context) error {
			var deliveryErr error
			found, deliveryErr = c.worker.DeliverOne(operationCtx, scope)
			return deliveryErr
		})
		if err != nil {
			deliveryErr := fmt.Errorf("deliver report: %w", err)
			if found && canContinueDeliveryPass(err) {
				warnings = append(warnings, deliveryErr)
				continue
			}
			return warnings, deliveryErr
		}
		if !found {
			break
		}
	}
	return warnings, nil
}

func (c *Coordinator) recoverTask(
	ctx context.Context,
	scope modelbilling.TaskScope,
	taskID string,
	states ...modelbilling.TaskState,
) error {
	now := c.config.Clock()
	if now.IsZero() {
		return contractError("coordinator clock returned zero time")
	}
	var last error
	for _, state := range states {
		err := c.operation(ctx, func(operationCtx context.Context) error {
			return c.tasks.MarkRecoveryRequired(operationCtx, scope, taskID, state, now)
		})
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		last = err
		if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorInvalidStateTransition {
			return err
		}
	}
	return last
}

func (c *Coordinator) operation(ctx context.Context, operation func(context.Context) error) error {
	operationCtx, cancel := context.WithTimeout(ctx, c.config.OperationTimeout)
	defer cancel()
	err := operation(operationCtx)
	if operationErr := operationCtx.Err(); operationErr != nil {
		return errors.Join(err, operationErr)
	}
	return err
}

func joinCoordinatorErrors(warnings []error, fatal error) error {
	errs := append([]error(nil), warnings...)
	errs = append(errs, fatal)
	return errors.Join(errs...)
}

func (c *Coordinator) reportError(err error) {
	if c.config.OnError != nil {
		c.config.OnError(err)
	}
}

func (c *Coordinator) noReportSuppressed(key noReportKey) bool {
	c.suppressionMu.Lock()
	defer c.suppressionMu.Unlock()
	_, ok := c.noReport[key]
	return ok
}

func (c *Coordinator) suppressNoReport(key noReportKey) {
	c.suppressionMu.Lock()
	defer c.suppressionMu.Unlock()
	c.noReport[key] = struct{}{}
}

func makeNoReportKey(scope modelbilling.TaskScope, taskID string) noReportKey {
	key := noReportKey{actorID: scope.ActorID, profileID: scope.ProfileID, taskID: taskID}
	if scope.CloudBinding != nil {
		key.cloudUserID = scope.CloudBinding.CloudUserID
		key.bindingGeneration = scope.CloudBinding.BindingGeneration
	}
	return key
}
