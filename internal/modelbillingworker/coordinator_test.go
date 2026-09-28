package modelbillingworker

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

type coordinatorUsageStub struct {
	ids    []string
	err    error
	calls  int
	events *[]string
}

func (s *coordinatorUsageStub) ListUsagePendingIDs(context.Context, modelbilling.TaskScope) ([]string, error) {
	s.calls++
	emit(s.events, "scan_usage")
	return append([]string(nil), s.ids...), s.err
}

type coordinatorReportStub struct {
	builds map[string]modelbilling.UsageReportBuildResult
	errors map[string]error
	calls  map[string]int
	events *[]string
}

func (s *coordinatorReportStub) EnqueueFinal(_ context.Context, _ modelbilling.TaskScope, taskID string) (modelbilling.UsageReportBuildResult, *modelbilling.PendingReport, error) {
	s.calls[taskID]++
	emit(s.events, "enqueue_"+taskID)
	return s.builds[taskID], nil, s.errors[taskID]
}

type recoveryCall struct {
	taskID string
	from   modelbilling.TaskState
}

type coordinatorTaskStub struct {
	settling  []string
	scanErr   error
	recovery  []recoveryCall
	markError func(string, modelbilling.TaskState) error
	calls     int
	events    *[]string
}

func (s *coordinatorTaskStub) ListSettlingIDs(context.Context, modelbilling.TaskScope) ([]string, error) {
	s.calls++
	emit(s.events, "scan_settling")
	return append([]string(nil), s.settling...), s.scanErr
}

func (s *coordinatorTaskStub) MarkRecoveryRequired(_ context.Context, _ modelbilling.TaskScope, taskID string, from modelbilling.TaskState, _ time.Time) error {
	s.recovery = append(s.recovery, recoveryCall{taskID: taskID, from: from})
	emit(s.events, "recover_"+taskID+"_"+string(from))
	if s.markError != nil {
		return s.markError(taskID, from)
	}
	return nil
}

type coordinatorDelivery struct {
	found bool
	err   error
}

type coordinatorWorkerStub struct {
	reconcileErrors map[string]error
	reconcile       func(context.Context, string) error
	reconciled      []string
	deliveries      []coordinatorDelivery
	deliveryCalls   int
	events          *[]string
	deliver         func(context.Context) (bool, error)
}

type coordinatorFreezeReleaser struct {
	results  map[string]cloud.FreezeResult
	errors   map[string]error
	requests []cloud.ReleaseFreezeRequest
}

func (r *coordinatorFreezeReleaser) ReleaseFreeze(
	_ context.Context,
	request cloud.ReleaseFreezeRequest,
) (cloud.FreezeResult, error) {
	r.requests = append(r.requests, request)
	return r.results[request.TaskID], r.errors[request.TaskID]
}

func (s *coordinatorWorkerStub) ReconcileTask(ctx context.Context, _ modelbilling.TaskScope, taskID string) error {
	s.reconciled = append(s.reconciled, taskID)
	emit(s.events, "reconcile_"+taskID)
	if s.reconcile != nil {
		return s.reconcile(ctx, taskID)
	}
	return s.reconcileErrors[taskID]
}

func (s *coordinatorWorkerStub) DeliverOne(ctx context.Context, _ modelbilling.TaskScope) (bool, error) {
	s.deliveryCalls++
	emit(s.events, "deliver")
	if s.deliver != nil {
		return s.deliver(ctx)
	}
	if len(s.deliveries) == 0 {
		return false, nil
	}
	result := s.deliveries[0]
	s.deliveries = s.deliveries[1:]
	return result.found, result.err
}

func coordinatorConfig(onError func(error)) CoordinatorConfig {
	return CoordinatorConfig{
		PollInterval: time.Hour, OperationTimeout: time.Second, MaxDeliveriesPerPass: 8,
		Clock: func() time.Time { return workerTestNow }, OnError: onError,
	}
}

func newCoordinatorForTest(
	t *testing.T,
	usage *coordinatorUsageStub,
	reports *coordinatorReportStub,
	tasks *coordinatorTaskStub,
	worker *coordinatorWorkerStub,
	config CoordinatorConfig,
) *Coordinator {
	t.Helper()
	coordinator, err := NewCoordinator(usage, reports, tasks, worker, config)
	if err != nil {
		t.Fatal(err)
	}
	return coordinator
}

func TestCoordinatorRunOnceIsolatesTasksAndReportsInStateOrder(t *testing.T) {
	scope := workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1})
	var events []string
	usage := &coordinatorUsageStub{ids: []string{"zero", "evidence_pending", "enqueue_bad", "accepted", "blocked"}, events: &events}
	reports := &coordinatorReportStub{
		builds: map[string]modelbilling.UsageReportBuildResult{
			"zero":             {Disposition: modelbilling.ReportBuildNoReportRequired},
			"evidence_pending": {Disposition: modelbilling.ReportBuildEvidencePending},
			"accepted":         {Disposition: modelbilling.ReportBuildDeliverable},
			"blocked":          {Disposition: modelbilling.ReportBuildBlocked},
		},
		errors: map[string]error{
			"enqueue_bad": &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "malformed task"},
		},
		calls: make(map[string]int), events: &events,
	}
	tasks := &coordinatorTaskStub{settling: []string{"settling_bad", "settling_ok"}, events: &events}
	worker := &coordinatorWorkerStub{
		reconcileErrors: map[string]error{
			"settling_bad": markLocalPersistenceError(&modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "malformed scope"}),
		},
		deliveries: []coordinatorDelivery{
			{found: true, err: continueDeliveryPass(errors.New("poison report isolated"))},
			{found: true, err: continueDeliveryPass(&modelbilling.Error{Code: modelbilling.ErrorUsageRetryable, Retryable: true})},
			{found: true},
			{found: false},
		},
		events: &events,
	}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))

	if err := coordinator.RunOnce(context.Background(), scope); err == nil {
		t.Fatal("RunOnce error = nil, want joined isolated errors")
	}
	if !reflect.DeepEqual(worker.reconciled, []string{"settling_bad", "settling_ok", "accepted", "blocked"}) {
		t.Fatalf("reconciled = %v", worker.reconciled)
	}
	if !reflect.DeepEqual(tasks.recovery, []recoveryCall{
		{taskID: "settling_bad", from: modelbilling.TaskSettling},
		{taskID: "enqueue_bad", from: modelbilling.TaskUsagePending},
	}) {
		t.Fatalf("recovery = %+v", tasks.recovery)
	}
	if worker.deliveryCalls != 4 {
		t.Fatalf("delivery calls = %d, want 4", worker.deliveryCalls)
	}
	wantPrefix := []string{
		"scan_settling", "reconcile_settling_bad", "recover_settling_bad_settling", "reconcile_settling_ok",
		"scan_usage", "enqueue_zero", "enqueue_evidence_pending", "enqueue_enqueue_bad", "recover_enqueue_bad_usage_pending",
		"enqueue_accepted", "reconcile_accepted", "enqueue_blocked", "reconcile_blocked",
	}
	if len(events) < len(wantPrefix) || !reflect.DeepEqual(events[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("events = %v, want prefix %v", events, wantPrefix)
	}
}

func TestCoordinatorSuppressesNoReportPerLifecycleAndFullScope(t *testing.T) {
	scope := workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1})
	usage := &coordinatorUsageStub{ids: []string{"zero"}}
	reports := &coordinatorReportStub{
		builds: map[string]modelbilling.UsageReportBuildResult{"zero": {Disposition: modelbilling.ReportBuildNoReportRequired}},
		errors: map[string]error{}, calls: make(map[string]int),
	}
	tasks := &coordinatorTaskStub{}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{}}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))

	for range 3 {
		if err := coordinator.RunOnce(context.Background(), scope); err != nil {
			t.Fatal(err)
		}
	}
	if reports.calls["zero"] != 1 || len(worker.reconciled) != 0 {
		t.Fatalf("same-scope no-report calls=%d reconciled=%v", reports.calls["zero"], worker.reconciled)
	}
	otherScope := workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 2})
	if err := coordinator.RunOnce(context.Background(), otherScope); err != nil {
		t.Fatal(err)
	}
	if reports.calls["zero"] != 2 {
		t.Fatalf("full-scope no-report calls = %d, want 2", reports.calls["zero"])
	}
	restarted := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))
	if err := restarted.RunOnce(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if reports.calls["zero"] != 3 {
		t.Fatalf("restart no-report calls = %d, want 3", reports.calls["zero"])
	}
}

func TestCoordinatorDoesNotReleaseNoReportFreezeAcrossRestart(t *testing.T) {
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1}
	scope := workerScope(binding)
	usage := &coordinatorUsageStub{ids: []string{"zero"}}
	reports := &coordinatorReportStub{
		builds: map[string]modelbilling.UsageReportBuildResult{
			"zero": {Disposition: modelbilling.ReportBuildNoReportRequired},
		},
		errors: map[string]error{}, calls: make(map[string]int),
	}
	releaser := &coordinatorFreezeReleaser{results: map[string]cloud.FreezeResult{}, errors: map[string]error{}}
	config := coordinatorConfig(nil)
	config.FreezeReleaser = releaser
	coordinator := newCoordinatorForTest(
		t,
		usage,
		reports,
		&coordinatorTaskStub{},
		&coordinatorWorkerStub{reconcileErrors: map[string]error{}},
		config,
	)

	for range 2 {
		if err := coordinator.RunOnce(context.Background(), scope); err != nil {
			t.Fatal(err)
		}
	}
	restarted := newCoordinatorForTest(
		t,
		usage,
		reports,
		&coordinatorTaskStub{},
		&coordinatorWorkerStub{reconcileErrors: map[string]error{}},
		config,
	)
	if err := restarted.RunOnce(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if len(releaser.requests) != 0 {
		t.Fatalf("no-report recovery sent release requests: %#v", releaser.requests)
	}
}

func TestCoordinatorRejectsBYOKBeforeDependencies(t *testing.T) {
	usage := &coordinatorUsageStub{ids: []string{"unexpected"}}
	reports := &coordinatorReportStub{builds: map[string]modelbilling.UsageReportBuildResult{}, errors: map[string]error{}, calls: make(map[string]int)}
	tasks := &coordinatorTaskStub{settling: []string{"unexpected"}}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{}}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))
	byok := modelbilling.TaskScope{ActorID: "local", ProfileID: "default"}

	if err := coordinator.RunOnce(context.Background(), byok); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorBillingSourceMismatch {
		t.Fatalf("BYOK error = %v", err)
	}
	if usage.calls != 0 || tasks.calls != 0 || len(reports.calls) != 0 || len(worker.reconciled) != 0 || worker.deliveryCalls != 0 {
		t.Fatalf("BYOK touched dependencies: usage=%d tasks=%d reports=%v worker=%+v", usage.calls, tasks.calls, reports.calls, worker)
	}
}

func TestCoordinatorRunStartsImmediatelyAndCancellationBoundsExit(t *testing.T) {
	scope := workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1})
	usage := &coordinatorUsageStub{}
	reports := &coordinatorReportStub{builds: map[string]modelbilling.UsageReportBuildResult{}, errors: map[string]error{}, calls: make(map[string]int)}
	tasks := &coordinatorTaskStub{}
	started := make(chan struct{})
	var once sync.Once
	worker := &coordinatorWorkerStub{
		reconcileErrors: map[string]error{},
		deliver: func(ctx context.Context) (bool, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return false, ctx.Err()
		},
	}
	config := coordinatorConfig(nil)
	config.OperationTimeout = time.Minute
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, config)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- coordinator.Run(ctx, scope) }()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("coordinator did not run immediately")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("coordinator did not stop after cancellation")
	}
}

func TestCoordinatorRecoversCurrentStateAfterUsageReconcileTransition(t *testing.T) {
	scope := workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1})
	usage := &coordinatorUsageStub{ids: []string{"transitioned"}}
	reports := &coordinatorReportStub{
		builds: map[string]modelbilling.UsageReportBuildResult{"transitioned": {Disposition: modelbilling.ReportBuildDeliverable}},
		errors: map[string]error{}, calls: make(map[string]int),
	}
	tasks := &coordinatorTaskStub{markError: func(_ string, from modelbilling.TaskState) error {
		if from == modelbilling.TaskUsagePending {
			return &modelbilling.Error{Code: modelbilling.ErrorInvalidStateTransition}
		}
		return nil
	}}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{
		"transitioned": markLocalPersistenceError(&modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "invalid settlement"}),
	}}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))

	if err := coordinator.RunOnce(context.Background(), scope); err == nil {
		t.Fatal("RunOnce error = nil")
	}
	if !reflect.DeepEqual(tasks.recovery, []recoveryCall{
		{taskID: "transitioned", from: modelbilling.TaskUsagePending},
		{taskID: "transitioned", from: modelbilling.TaskSettling},
	}) {
		t.Fatalf("recovery attempts = %+v", tasks.recovery)
	}
}

func TestCoordinatorRemoteSettlementErrorsStaySettling(t *testing.T) {
	scope := workerTestScope()
	for _, code := range []modelbilling.ErrorCode{
		modelbilling.ErrorScopeMismatch,
		modelbilling.ErrorContractInvalid,
	} {
		t.Run(string(code), func(t *testing.T) {
			taskID := "task_remote_" + string(code)
			before := workerTask(taskID, "freeze_"+taskID, scope, modelbilling.TaskSettling, modelbilling.TaskResultFailed)
			workerTasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{taskID: before}}
			reports := acceptedFinalStore(t, before)
			client := &recordingCloud{t: t, getResults: []cloudResult{{err: &modelbilling.Error{Code: code, Message: "remote rejection"}}}}
			worker := newWorkerForTest(t, reports, workerTasks, client, &observerSpy{}, timePointer(workerTestNow))
			usage := &coordinatorUsageStub{}
			tasks := &coordinatorTaskStub{settling: []string{taskID}}
			coordinator, err := NewCoordinator(usage, reports, tasks, worker, coordinatorConfig(nil))
			if err != nil {
				t.Fatal(err)
			}

			err = coordinator.RunOnce(context.Background(), scope)
			if modelbilling.ErrorCodeOf(err) != code {
				t.Fatalf("RunOnce error = %v", err)
			}
			if workerTasks.tasks[taskID].State != modelbilling.TaskSettling || len(workerTasks.updates) != 0 || len(tasks.recovery) != 0 {
				t.Fatalf("remote error changed task: state=%s updates=%d recovery=%v", workerTasks.tasks[taskID].State, len(workerTasks.updates), tasks.recovery)
			}
			if usage.calls != 1 || reports.claimCalls != 1 || client.gets != 1 {
				t.Fatalf("remote error starved later phases: usage=%d claims=%d gets=%d", usage.calls, reports.claimCalls, client.gets)
			}
			assertTaskFacts(t, workerTasks.tasks[taskID], before)
		})
	}
}

func TestCoordinatorDeliveryAuthStopsAfterFirstClaim(t *testing.T) {
	scope, pending := coordinatorPendingPartials(t, 32)
	reports := &scriptedReportStore{reports: pending}
	client := &recordingCloud{t: t, uploadErrors: []error{
		&modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "login required"},
	}}
	worker := newWorkerForTest(t, reports, &scriptedTaskStore{}, client, &observerSpy{}, timePointer(workerTestNow))
	usage := &coordinatorUsageStub{}
	tasks := &coordinatorTaskStub{}
	config := coordinatorConfig(nil)
	config.MaxDeliveriesPerPass = 32
	coordinator, err := NewCoordinator(usage, reports, tasks, worker, config)
	if err != nil {
		t.Fatal(err)
	}

	err = coordinator.RunOnce(context.Background(), scope)
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("RunOnce error = %v", err)
	}
	if reports.claimCalls != 1 || client.uploads != 1 || reports.reports[0].State != modelbilling.ReportDelivering {
		t.Fatalf("first report: claims=%d uploads=%d state=%s", reports.claimCalls, client.uploads, reports.reports[0].State)
	}
	for i, report := range reports.reports[1:] {
		if report.State != modelbilling.ReportPending || report.AttemptCount != 0 {
			t.Fatalf("report %d was claimed: state=%s attempt=%d", i+1, report.State, report.AttemptCount)
		}
	}
}

func TestCoordinatorDeliveryRetryContinuesToNextReport(t *testing.T) {
	scope, pending := coordinatorPendingPartials(t, 2)
	retryErr := &modelbilling.Error{Code: modelbilling.ErrorUsageRetryable, Message: "retry", Retryable: true}
	reports := &scriptedReportStore{reports: pending}
	client := &recordingCloud{t: t, uploadErrors: []error{retryErr, nil}}
	worker := newWorkerForTest(t, reports, &scriptedTaskStore{}, client, &observerSpy{}, timePointer(workerTestNow))
	coordinator, err := NewCoordinator(&coordinatorUsageStub{}, reports, &coordinatorTaskStub{}, worker, coordinatorConfig(nil))
	if err != nil {
		t.Fatal(err)
	}

	err = coordinator.RunOnce(context.Background(), scope)
	if !errors.Is(err, retryErr) {
		t.Fatalf("RunOnce error = %v", err)
	}
	if reports.claimCalls != 3 || client.uploads != 2 || reports.reports[0].State != modelbilling.ReportRetry || reports.reports[1].State != modelbilling.ReportAccepted {
		t.Fatalf("delivery progression: claims=%d uploads=%d states=%s/%s", reports.claimCalls, client.uploads, reports.reports[0].State, reports.reports[1].State)
	}
}

func TestCoordinatorDeliveryPoisonIsolationContinues(t *testing.T) {
	scope, pending := coordinatorPendingPartials(t, 1)
	poisonErr := &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "poison row"}
	reports := &scriptedReportStore{
		reports:      pending,
		claimResults: []coordinatorDelivery{{found: true, err: poisonErr}},
	}
	client := &recordingCloud{t: t, uploadErrors: []error{nil}}
	worker := newWorkerForTest(t, reports, &scriptedTaskStore{}, client, &observerSpy{}, timePointer(workerTestNow))
	coordinator, err := NewCoordinator(&coordinatorUsageStub{}, reports, &coordinatorTaskStub{}, worker, coordinatorConfig(nil))
	if err != nil {
		t.Fatal(err)
	}

	err = coordinator.RunOnce(context.Background(), scope)
	if !errors.Is(err, poisonErr) {
		t.Fatalf("RunOnce error = %v", err)
	}
	if reports.claimCalls != 3 || client.uploads != 1 || reports.reports[0].State != modelbilling.ReportAccepted || reports.blocked != 0 {
		t.Fatalf("poison progression: claims=%d uploads=%d state=%s extra_blocks=%d", reports.claimCalls, client.uploads, reports.reports[0].State, reports.blocked)
	}
}

func TestCoordinatorSettlingScannerFailureStopsPass(t *testing.T) {
	scanErr := errors.New("settling scanner failed")
	usage := &coordinatorUsageStub{ids: []string{"unexpected"}}
	reports := &coordinatorReportStub{builds: map[string]modelbilling.UsageReportBuildResult{}, errors: map[string]error{}, calls: make(map[string]int)}
	tasks := &coordinatorTaskStub{scanErr: scanErr}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{}}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))

	err := coordinator.RunOnce(context.Background(), workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1}))
	if !errors.Is(err, scanErr) {
		t.Fatalf("RunOnce error = %v", err)
	}
	if usage.calls != 0 || len(reports.calls) != 0 || len(worker.reconciled) != 0 || worker.deliveryCalls != 0 {
		t.Fatalf("scanner failure continued: usage=%d reports=%v reconcile=%v delivery=%d", usage.calls, reports.calls, worker.reconciled, worker.deliveryCalls)
	}
}

func TestCoordinatorUsageScannerFailureStopsDelivery(t *testing.T) {
	scanErr := errors.New("usage scanner failed")
	usage := &coordinatorUsageStub{ids: []string{"unexpected"}, err: scanErr}
	reports := &coordinatorReportStub{builds: map[string]modelbilling.UsageReportBuildResult{}, errors: map[string]error{}, calls: make(map[string]int)}
	tasks := &coordinatorTaskStub{}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{}}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))

	err := coordinator.RunOnce(context.Background(), workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1}))
	if !errors.Is(err, scanErr) {
		t.Fatalf("RunOnce error = %v", err)
	}
	if usage.calls != 1 || len(reports.calls) != 0 || worker.deliveryCalls != 0 {
		t.Fatalf("usage scanner failure continued: usage=%d reports=%v delivery=%d", usage.calls, reports.calls, worker.deliveryCalls)
	}
}

func TestCoordinatorOperationDeadlineStopsPass(t *testing.T) {
	usage := &coordinatorUsageStub{ids: []string{"unexpected"}}
	reports := &coordinatorReportStub{builds: map[string]modelbilling.UsageReportBuildResult{}, errors: map[string]error{}, calls: make(map[string]int)}
	tasks := &coordinatorTaskStub{settling: []string{"deadline", "later"}}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{}}
	worker.reconcile = func(ctx context.Context, taskID string) error {
		if taskID == "deadline" {
			<-ctx.Done()
		}
		return nil
	}
	config := coordinatorConfig(nil)
	config.OperationTimeout = 10 * time.Millisecond
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, config)
	parent := context.Background()

	err := coordinator.RunOnce(parent, workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1}))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunOnce error = %v", err)
	}
	if parent.Err() != nil || !reflect.DeepEqual(worker.reconciled, []string{"deadline"}) || usage.calls != 0 || worker.deliveryCalls != 0 {
		t.Fatalf("deadline continued: parent=%v reconcile=%v usage=%d delivery=%d", parent.Err(), worker.reconciled, usage.calls, worker.deliveryCalls)
	}
}

func TestCoordinatorLocalHydrateRecoveryContinues(t *testing.T) {
	scope := workerTestScope()
	cause := &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "corrupt local task"}
	good := workerTask("good", "freeze_good", scope, modelbilling.TaskCompleted, modelbilling.TaskResultCompleted)
	workerTasks := &scriptedTaskStore{
		tasks:     map[string]modelbilling.ModelTask{"good": good},
		getErrors: map[string][]error{"bad": {cause}},
	}
	reports := &scriptedReportStore{}
	worker := newWorkerForTest(t, reports, workerTasks, &recordingCloud{t: t}, &observerSpy{}, timePointer(workerTestNow))
	usage := &coordinatorUsageStub{}
	tasks := &coordinatorTaskStub{settling: []string{"bad", "good"}}
	coordinator, err := NewCoordinator(usage, reports, tasks, worker, coordinatorConfig(nil))
	if err != nil {
		t.Fatal(err)
	}

	err = coordinator.RunOnce(context.Background(), scope)
	if !errors.Is(err, cause) {
		t.Fatalf("RunOnce error = %v", err)
	}
	if !reflect.DeepEqual(tasks.recovery, []recoveryCall{{taskID: "bad", from: modelbilling.TaskSettling}}) || workerTasks.getCalls != 2 || usage.calls != 1 || reports.claimCalls != 1 {
		t.Fatalf("local recovery: recovery=%v gets=%d usage=%d claims=%d", tasks.recovery, workerTasks.getCalls, usage.calls, reports.claimCalls)
	}
}

func TestCoordinatorRecoveryCASFailureStopsPass(t *testing.T) {
	scope := workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 1})
	cause := &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "corrupt local task"}
	casErr := errors.New("recovery CAS failed")
	usage := &coordinatorUsageStub{ids: []string{"unexpected"}}
	reports := &coordinatorReportStub{builds: map[string]modelbilling.UsageReportBuildResult{}, errors: map[string]error{}, calls: make(map[string]int)}
	tasks := &coordinatorTaskStub{
		settling: []string{"bad", "later"},
		markError: func(string, modelbilling.TaskState) error {
			return casErr
		},
	}
	worker := &coordinatorWorkerStub{reconcileErrors: map[string]error{"bad": markLocalPersistenceError(cause)}}
	coordinator := newCoordinatorForTest(t, usage, reports, tasks, worker, coordinatorConfig(nil))

	err := coordinator.RunOnce(context.Background(), scope)
	if !errors.Is(err, cause) || !errors.Is(err, casErr) {
		t.Fatalf("RunOnce error lost cause: %v", err)
	}
	if !reflect.DeepEqual(worker.reconciled, []string{"bad"}) || usage.calls != 0 || worker.deliveryCalls != 0 {
		t.Fatalf("CAS failure continued: reconcile=%v usage=%d delivery=%d", worker.reconciled, usage.calls, worker.deliveryCalls)
	}
}

func coordinatorPendingPartials(t *testing.T, count int) (modelbilling.TaskScope, []modelbilling.PendingReport) {
	t.Helper()
	scope := workerTestScope()
	taskID := "task-coordinator-partials"
	freezeID := "freeze-coordinator-partials"
	reports := make([]modelbilling.PendingReport, 0, count)
	for i := 0; i < count; i++ {
		partial := newPartialQuotaReport(t, scope, taskID, freezeID, uint64(i+1), false)
		reports = append(reports, pendingPartial(t, partial))
	}
	return scope, reports
}
