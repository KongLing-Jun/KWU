package boot

import (
	"fmt"

	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/nilutil"
)

func validateTaskContextOptions(opts Options) error {
	if opts.RequireTaskContext && nilutil.IsNil(opts.TaskContextInitializer) {
		return fmt.Errorf("boot: task context initializer is required")
	}
	if opts.RequireTaskContext {
		if _, err := modelbilling.NewBillingRouteSelection(opts.BillingRouteSelection); err != nil {
			return fmt.Errorf("boot: explicit billing route selection is required: %w", err)
		}
	}
	return nil
}

func validateBillingRouteExecutorModel(opts Options, modelRef string) error {
	if !opts.RequireTaskContext {
		return nil
	}
	if opts.BillingRouteSelection.ModelRef != modelRef {
		return fmt.Errorf("boot: billing route model %q does not match executor model %q", opts.BillingRouteSelection.ModelRef, modelRef)
	}
	return nil
}

func withTaskContextOptions(target control.Options, source Options) control.Options {
	target.TaskContextInitializer = source.TaskContextInitializer
	target.RequireTaskContext = source.RequireTaskContext
	target.BillingRouteSelection = source.BillingRouteSelection
	if source.MainStreamDispatcher != nil {
		target.TaskLifecycle = modelbilling.TaskLifecycleRunner{
			Tasks: source.ModelBillingTaskStore, Recovery: source.TaskLifecycleRecovery, Now: source.TaskLifecycleNow,
		}
	} else {
		target.TaskLifecycle = nil
	}
	return target
}
