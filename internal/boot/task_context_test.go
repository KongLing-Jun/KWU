package boot

import (
	"context"
	"strings"
	"testing"
	"time"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
)

type bootTaskContextInitializer struct{}

func (bootTaskContextInitializer) InitializeTaskContext(context.Context, control.TaskContextInitRequest) (modelbilling.TaskContext, error) {
	return modelbilling.TaskContext{}, nil
}

type bootUsagePendingRecovery struct{}

func (bootUsagePendingRecovery) RecoverUsagePendingTask(context.Context, modelbilling.TaskScope, string) error {
	return nil
}

func TestApplyTaskContextOptionsForBothBootControllerPaths(t *testing.T) {
	initializer := bootTaskContextInitializer{}
	source := Options{ModelBillingOptions: ModelBillingOptions{TaskContextInitializer: initializer, RequireTaskContext: true}}
	assembly := newBuildAssembly(context.Background(), source)
	assembly.cfg = config.Default()
	assembly.entry = &config.ProviderEntry{}
	first := assembly.controllerOptions()
	second := withTaskContextOptions(control.Options{}, source)
	if first.TaskContextInitializer != initializer || second.TaskContextInitializer != initializer || !first.RequireTaskContext || !second.RequireTaskContext {
		t.Fatal("task context options were not forwarded")
	}
}

func TestApplyTaskContextOptionsInstallsTopLevelLifecycleWithDispatcher(t *testing.T) {
	initializer := bootTaskContextInitializer{}
	recovery := bootUsagePendingRecovery{}
	fixtureNow := time.Date(2026, 7, 11, 0, 12, 0, 0, time.UTC)
	source := Options{ModelBillingOptions: ModelBillingOptions{
		TaskContextInitializer: initializer,
		RequireTaskContext:     true,
		MainStreamDispatcher:   &modelbilling.MainExecutorDispatcher{},
		ModelBillingTaskStore:  new(modelbilling.SQLiteTaskStore),
		TaskLifecycleRecovery:  recovery,
		TaskLifecycleNow:       func() time.Time { return fixtureNow },
	}}
	assembly := newBuildAssembly(context.Background(), source)
	assembly.cfg = config.Default()
	assembly.entry = &config.ProviderEntry{}

	for name, opts := range map[string]control.Options{
		"assembly": assembly.controllerOptions(),
		"build":    withTaskContextOptions(control.Options{}, source),
	} {
		runner, ok := opts.TaskLifecycle.(modelbilling.TaskLifecycleRunner)
		if !ok {
			t.Fatalf("%s task lifecycle = %T, want modelbilling.TaskLifecycleRunner", name, opts.TaskLifecycle)
		}
		if runner.Now == nil || !runner.Now().Equal(fixtureNow) {
			t.Fatalf("%s task lifecycle clock was not forwarded", name)
		}
		if runner.Recovery != recovery {
			t.Fatalf("%s task lifecycle recovery was not forwarded", name)
		}
	}
	if got := withTaskContextOptions(control.Options{}, Options{}).TaskLifecycle; got != nil {
		t.Fatalf("legacy task lifecycle = %T, want nil", got)
	}
}

func TestBuildRequiresTaskContextInitializerWhenEnabled(t *testing.T) {
	_, err := Build(context.Background(), Options{ModelBillingOptions: ModelBillingOptions{RequireTaskContext: true}})
	if err == nil || !strings.Contains(err.Error(), "task context initializer is required") {
		t.Fatalf("Build required initializer error = %v", err)
	}
	var typedNil *bootTaskContextInitializer
	_, err = Build(context.Background(), Options{ModelBillingOptions: ModelBillingOptions{RequireTaskContext: true, TaskContextInitializer: typedNil}})
	if err == nil || !strings.Contains(err.Error(), "task context initializer is required") {
		t.Fatalf("Build typed-nil initializer error = %v", err)
	}
	_, err = newBuildAssembly(context.Background(), Options{ModelBillingOptions: ModelBillingOptions{RequireTaskContext: true}}).buildController()
	if err == nil || !strings.Contains(err.Error(), "task context initializer is required") {
		t.Fatalf("buildAssembly required initializer error = %v", err)
	}
}

func TestRequireTaskContextRejectsMissingBillingSourceForLegacyTab(t *testing.T) {
	opts := Options{ModelBillingOptions: ModelBillingOptions{RequireTaskContext: true, TaskContextInitializer: bootTaskContextInitializer{}, BillingRouteSelection: modelbilling.BillingRouteSelection{ModelRef: "provider/model"}}}
	if err := validateTaskContextOptions(opts); err == nil || !strings.Contains(err.Error(), "explicit billing route selection is required") {
		t.Fatalf("missing billing source error = %v", err)
	}
}

func TestRequireTaskContextRejectsRouteModelDifferentFromExecutor(t *testing.T) {
	opts := Options{ModelBillingOptions: ModelBillingOptions{RequireTaskContext: true, BillingRouteSelection: modelbilling.BillingRouteSelection{ModelRef: "provider/other", BillingSource: modelbilling.BillingNewAPIByok}}}
	if err := validateBillingRouteExecutorModel(opts, "provider/main"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatch error = %v", err)
	}
	opts.BillingRouteSelection.ModelRef = "provider/main"
	if err := validateBillingRouteExecutorModel(opts, "provider/main"); err != nil {
		t.Fatalf("same canonical ref: %v", err)
	}
}
