package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	"kwu/internal/userctx"
)

func TestCloudEventWorkerSwitchWaitsBeforeRebindAndStartsNewScope(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)

	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	runA := receiveDesktopTest(t, "first event worker start", runner.started)
	if runA.scope.CloudUserID != "usr_01" || runA.scope.BindingGeneration != 1 {
		t.Fatalf("first worker scope = %#v", runA.scope)
	}

	type switchResult struct {
		view CloudBootstrapView
		err  error
	}
	switchDone := make(chan switchResult, 1)
	go func() {
		view, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
		switchDone <- switchResult{view: view, err: err}
	}()
	waitDesktopSignal(t, "first event worker cancellation", runA.canceled)
	if runtime.operationMu.TryLock() {
		runtime.operationMu.Unlock()
		t.Fatal("CloudSwitchAccount released operationMu before old worker exit")
	}
	if runA.cancelBinding != authViewBinding(loggedIn.Account, actor.ActorID) {
		t.Fatalf("binding when old worker observed cancellation = %#v, want A", runA.cancelBinding)
	}
	current, err := runtime.bindings.ResolveBinding(ctx, actor)
	if err != nil || current != runA.cancelBinding {
		t.Fatalf("binding before old worker exit = %#v, %v; want A", current, err)
	}
	runA.releaseWorker()
	switched := receiveDesktopTest(t, "account switch completion", switchDone)
	if switched.err != nil || switched.view.Account.CloudUserID != "usr_02" || switched.view.Account.BindingGeneration != 2 {
		t.Fatalf("CloudSwitchAccount = %#v, %v", switched.view, switched.err)
	}
	runB := receiveDesktopTest(t, "second event worker start", runner.started)
	if runB.scope.CloudUserID != "usr_02" || runB.scope.BindingGeneration != 2 {
		t.Fatalf("second worker scope = %#v", runB.scope)
	}
	if _, found, err := runtime.eventState.Load(ctx, runA.scope); err != nil || found {
		t.Fatalf("old late cursor after switch = found %t, err %v", found, err)
	}
	if _, found, err := runtime.eventState.Load(ctx, runB.scope); err != nil || found {
		t.Fatalf("old worker wrote new scope cursor = found %t, err %v", found, err)
	}
	closeCloudRuntimeAfterWorker(t, runtime, runB)
}

func TestCloudEventWorkerStartsOnlyAfterRuntimeStartAndAuthenticatedBootstrap(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runtime.lifecycleMu.Lock()
	workerBeforeStart := runtime.worker
	startedBeforeStart := runtime.started
	runtime.lifecycleMu.Unlock()
	if workerBeforeStart != nil || startedBeforeStart {
		t.Fatalf("unstarted runtime created worker=%#v started=%t", workerBeforeStart, startedBeforeStart)
	}
	runtime.start(ctx)
	if _, err := app.CloudBootstrap(); err != nil {
		t.Fatal(err)
	}
	run := receiveDesktopTest(t, "lazy event worker start", runner.started)
	if run.scope.CloudUserID != "usr_01" || run.scope.BindingGeneration != 1 {
		t.Fatalf("lazy worker scope = %#v", run.scope)
	}
	closeCloudRuntimeAfterWorker(t, runtime, run)
}

func TestCloudEventWorkerDoesNotStartForAnonymousBootstrap(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	view, err := app.CloudBootstrap()
	if err != nil || view.Account.Status != cloud.AccountAnonymous {
		t.Fatalf("anonymous CloudBootstrap = %#v, %v", view, err)
	}
	runtime.lifecycleMu.Lock()
	worker := runtime.worker
	runtime.lifecycleMu.Unlock()
	if worker != nil {
		t.Fatalf("anonymous bootstrap started worker %#v", worker)
	}
	runtime.close()
}

func TestCloudEventWorkerStopsForUnavailableSessionAndRestartsOnOnlineBootstrap(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	run := receiveDesktopTest(t, "initial event worker start", runner.started)
	bound := cloud.ProfileBinding{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID,
		CloudUserID: loggedIn.Account.CloudUserID, Status: cloud.BindingBound,
		BindingGeneration: loggedIn.Account.BindingGeneration,
	}

	for _, unavailable := range []struct {
		name string
		auth cloud.AuthView
	}{
		{"offline", cloud.AuthView{State: cloud.AccountOffline, Binding: bound}},
		{"expired", cloud.AuthView{State: cloud.AccountExpired, Binding: bound}},
		{"unbound", cloud.AuthView{State: cloud.AccountAnonymous, Binding: cloud.ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: cloud.BindingUnbound}}},
	} {
		t.Run(unavailable.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- app.reconcileDesktopBillingAuth(runtime, actor, unavailable.auth) }()
			waitDesktopSignal(t, unavailable.name+" event worker cancellation", run.canceled)
			run.releaseWorker()
			if err := receiveDesktopTest(t, unavailable.name+" lifecycle reconciliation", done); err != nil {
				t.Fatal(err)
			}
			runtime.lifecycleMu.Lock()
			worker := runtime.worker
			runtime.lifecycleMu.Unlock()
			if worker != nil {
				t.Fatalf("%s lifecycle left event worker running", unavailable.name)
			}
			if _, err := app.CloudBootstrap(); err != nil {
				t.Fatalf("authenticated bootstrap after %s: %v", unavailable.name, err)
			}
			run = receiveDesktopTest(t, "event worker restart after "+unavailable.name, runner.started)
			if run.scope.CloudUserID != bound.CloudUserID || run.scope.BindingGeneration != bound.BindingGeneration {
				t.Fatalf("worker scope after %s = %#v", unavailable.name, run.scope)
			}
		})
	}
	closeCloudRuntimeAfterWorker(t, runtime, run)
}

func TestCloudEventWorkerPrepareFailureKeepsCurrentWorkerRunning(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runA := receiveDesktopTest(t, "current event worker start", runner.started)

	if _, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13900000000", Password: "wrong-password"}); err == nil {
		t.Fatal("CloudSwitchAccount with wrong password succeeded")
	}
	if runA.ctx.Err() != nil {
		t.Fatalf("prepare failure canceled current worker: %v", runA.ctx.Err())
	}
	current, err := runtime.bindings.ResolveBinding(ctx, actor)
	if err != nil || current.CloudUserID != "usr_01" || current.BindingGeneration != 1 {
		t.Fatalf("binding after prepare failure = %#v, %v", current, err)
	}
	closeCloudRuntimeAfterWorker(t, runtime, runA)
}

func TestCloudEventWorkerLogoutCancelsAndWaitsBeforeIdentityCleanup(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runA := receiveDesktopTest(t, "logout event worker start", runner.started)
	type logoutResult struct {
		view CloudBootstrapView
		err  error
	}
	logoutDone := make(chan logoutResult, 1)
	go func() {
		view, err := app.CloudLogout()
		logoutDone <- logoutResult{view: view, err: err}
	}()
	waitDesktopSignal(t, "logout event worker cancellation", runA.canceled)
	current, err := runtime.bindings.ResolveBinding(ctx, actor)
	if err != nil || current.CloudUserID != "usr_01" || current.BindingGeneration != 1 {
		t.Fatalf("binding before logout worker exit = %#v, %v", current, err)
	}
	runA.releaseWorker()
	result := receiveDesktopTest(t, "logout completion", logoutDone)
	if result.err != nil || result.view.Account.Status != cloud.AccountAnonymous {
		t.Fatalf("CloudLogout = %#v, %v", result.view, result.err)
	}
	if _, found, err := runtime.eventState.Load(ctx, runA.scope); err != nil || found {
		t.Fatalf("late old cursor after logout = found %t, err %v", found, err)
	}
	runtime.close()
}

func TestCloudEventWorkerCloseCancelsWaitsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runner := newDesktopBlockingEventRunner(t, runtime.bindings, runtime.eventState)
	runtime.eventSync = runner
	cleanupDesktopBlockingRuntime(t, runtime, runner)
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runA := receiveDesktopTest(t, "close event worker start", runner.started)
	closeDone := make(chan struct{})
	go func() {
		runtime.close()
		close(closeDone)
	}()
	waitDesktopSignal(t, "close event worker cancellation", runA.canceled)
	if !runtime.closed.Load() {
		t.Fatal("runtime was not marked closed before waiting for worker")
	}
	if runtime.operationMu.TryLock() {
		runtime.operationMu.Unlock()
		t.Fatal("close did not coordinate with public operations while waiting")
	}
	current, err := runtime.bindings.ResolveBinding(ctx, actor)
	if err != nil || current.CloudUserID != "usr_01" || current.BindingGeneration != 1 {
		t.Fatalf("binding before close worker exit = %#v, %v", current, err)
	}
	runA.releaseWorker()
	waitDesktopSignal(t, "runtime close completion", closeDone)
	runtime.close()
}

func TestCloudSwitchAccountRoundTripPreservesOldTaskAndPaymentScope(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	oldScope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_01", BindingGeneration: 1}
	if loggedIn.Account.CloudUserID != oldScope.CloudUserID || loggedIn.Account.BindingGeneration != oldScope.BindingGeneration {
		t.Fatalf("initial account = %#v", loggedIn.Account)
	}
	payment, err := app.CloudCreateSubscriptionOrder("pro", "alipay")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := cloud.LoadCanonicalA0Fixture()
	if err != nil {
		t.Fatal(err)
	}
	task := canonicalDesktopTaskRecord(t, bundle, oldScope)
	if err := runtime.taskStore.Save(ctx, task); err != nil {
		t.Fatal(err)
	}
	seed := cloud.Event{
		EventID: "desktop-old-cursor", EventType: cloud.EventBalanceUpdated, CloudUserID: oldScope.CloudUserID,
		OccurredAt: time.Now().UTC(), Version: 1, Data: json.RawMessage(`{}`),
	}
	if err := runtime.eventState.CommitApplied(ctx, oldScope, seed); err != nil {
		t.Fatal(err)
	}

	candidate, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil || candidate.OperationError == nil || candidate.OperationError.Code != cloud.ErrorProfileScopeMismatch || candidate.Account.CloudUserID != "usr_01" {
		t.Fatalf("candidate CloudLogin = %#v, %v", candidate, err)
	}
	switched, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil || switched.Account.CloudUserID != "usr_02" || switched.Account.BindingGeneration != 2 {
		t.Fatalf("switch to B = %#v, %v", switched, err)
	}
	bScope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 2}
	if _, found, err := runtime.sessionStore.Load(ctx, oldScope); err != nil || found {
		t.Fatalf("old session after switch = found %t, err %v", found, err)
	}
	if _, found, err := runtime.eventState.Load(ctx, oldScope); err != nil || found {
		t.Fatalf("old cursor after switch = found %t, err %v", found, err)
	}
	if _, found, err := runtime.assets.LoadBalance(ctx, oldScope); err != nil || found {
		t.Fatalf("old balance after switch = found %t, err %v", found, err)
	}
	if _, found, err := runtime.assets.LoadSubscription(ctx, oldScope); err != nil || found {
		t.Fatalf("old subscription after switch = found %t, err %v", found, err)
	}
	if _, found, err := runtime.paymentOrders.Get(ctx, oldScope, payment.Payment.OrderID); err != nil || !found {
		t.Fatalf("old payment after switch = found %t, err %v", found, err)
	}
	if records, err := runtime.taskStore.Load(ctx, oldScope); err != nil || len(records) != 1 || records[0] != task {
		t.Fatalf("old tasks after switch = %#v, %v", records, err)
	}
	if _, found, err := runtime.sessionStore.Load(ctx, bScope); err != nil || !found {
		t.Fatalf("B session after switch = found %t, err %v", found, err)
	}
	if _, found, err := runtime.assets.LoadBalance(ctx, bScope); err != nil || !found {
		t.Fatalf("B balance after switch = found %t, err %v", found, err)
	}
	if records, err := runtime.taskStore.Load(ctx, bScope); err != nil || len(records) != 0 {
		t.Fatalf("B inherited tasks = %#v, %v", records, err)
	}
	if _, found, err := runtime.paymentOrders.Get(ctx, bScope, payment.Payment.OrderID); err != nil || found {
		t.Fatalf("B inherited old payment = found %t, err %v", found, err)
	}

	back, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil || back.Account.CloudUserID != "usr_01" || back.Account.BindingGeneration != 3 {
		t.Fatalf("switch back to A = %#v, %v", back, err)
	}
	runtime.close()
	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.close()
	restartedApp := NewApp()
	restartedApp.cloudRuntime = restarted
	restartedApp.setCurrentActorContext(actor)
	recovered, err := restartedApp.CloudBootstrap()
	if err != nil || recovered.Account.CloudUserID != "usr_01" || recovered.Account.BindingGeneration != 3 {
		t.Fatalf("restart current binding = %#v, %v", recovered, err)
	}
	if _, found, err := restarted.sessionStore.Load(ctx, bScope); err != nil || found {
		t.Fatalf("restart retained non-current B session = found %t, err %v", found, err)
	}
	if _, found, err := restarted.paymentOrders.Get(ctx, oldScope, payment.Payment.OrderID); err != nil || !found {
		t.Fatalf("restart lost old payment = found %t, err %v", found, err)
	}
	if records, err := restarted.taskStore.Load(ctx, oldScope); err != nil || len(records) != 1 || records[0] != task {
		t.Fatalf("restart lost old tasks = %#v, %v", records, err)
	}
}

func TestCloudSwitchAccountPostCommitHydrationFailureReturnsAuthoritativePartialView(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.close()
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)

	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil || loggedIn.Account.CloudUserID != "usr_01" || loggedIn.Balance == nil || loggedIn.Subscription == nil {
		t.Fatalf("initial A login = %#v, %v", loggedIn, err)
	}
	oldPayment, err := app.CloudCreateSubscriptionOrder("pro", "alipay")
	if err != nil {
		t.Fatal(err)
	}
	secret := "fixture-secret-post-commit-balance"
	failingBalance, err := cloud.NewBalanceApplication(cloud.BalanceApplicationDependencies{
		Bindings: runtime.bindings,
		Client: desktopGetCurrentUserFailureClient{
			CloudAccountClient: runtime.accounts,
			err:                &cloud.Error{Code: cloud.ErrorNetworkUnavailable, Message: secret},
		},
		Snapshots: runtime.assets,
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime.balance = failingBalance

	partial, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		binding, _ := runtime.bindings.ResolveBinding(ctx, actor)
		t.Fatalf("post-commit hydration returned Go error %v with durable binding %#v and view %#v", err, binding, partial)
	}
	if partial.Account.Status != cloud.AccountAuthenticated || partial.Account.CloudUserID != "usr_02" || partial.Account.BindingGeneration != 2 {
		t.Fatalf("post-commit partial account = %#v", partial.Account)
	}
	if partial.OperationError == nil || partial.OperationError.Code != cloud.ErrorNetworkUnavailable || partial.OperationError.Message != "cloud network is unavailable" {
		t.Fatalf("post-commit partial operation error = %#v", partial.OperationError)
	}
	if partial.Balance != nil || partial.Subscription != nil || len(partial.Payments) != 0 {
		t.Fatalf("post-commit partial leaked A assets: balance=%#v subscription=%#v payments=%#v", partial.Balance, partial.Subscription, partial.Payments)
	}
	if partial.Candidate != nil || partial.Account.CloudUserID == loggedIn.Account.CloudUserID || strings.Contains(partial.OperationError.Message, secret) {
		t.Fatalf("post-commit partial exposed stale identity or unsafe detail: %#v", partial)
	}
	if partial.Account.CloudUserID == oldPayment.Account.CloudUserID {
		t.Fatalf("post-commit partial retained old payment account %#v", oldPayment.Account)
	}
	binding, err := runtime.bindings.ResolveBinding(ctx, actor)
	if err != nil || binding.CloudUserID != "usr_02" || binding.BindingGeneration != 2 {
		t.Fatalf("durable binding after partial switch = %#v, %v", binding, err)
	}
}

func TestCloudLegacyCanonicalEventStopsAtFreezeIdentityContractGapAcrossRestart(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopLegacyFreezeGapEvents(t, runtime)
	runtime.settlements.bind(&desktopSettlementReconcilerSpy{})
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	view, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: view.Account.CloudUserID, BindingGeneration: view.Account.BindingGeneration}
	key := cloudWorkerKey(actor, scope)
	assertDesktopLegacyCanonicalFreezeGap(t, runtime, scope, waitDesktopEventWorker(t, runtime, key))
	eventPath := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "event_state.json")
	before, err := os.ReadFile(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloudRefreshAssets(); err != nil {
		t.Fatal(err)
	}
	runtime.lifecycleMu.Lock()
	workerAfterRefresh := runtime.worker
	_, completed := runtime.workerResults[key]
	runtime.lifecycleMu.Unlock()
	if workerAfterRefresh != nil || !completed {
		t.Fatalf("same failed scope restarted: worker=%#v completed=%t", workerAfterRefresh, completed)
	}
	afterRefresh, err := os.ReadFile(eventPath)
	if err != nil || !bytes.Equal(afterRefresh, before) {
		t.Fatalf("same-runtime bootstrap advanced past contract gap: err %v", err)
	}
	runtime.close()

	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopLegacyFreezeGapEvents(t, restarted)
	restarted.settlements.bind(&desktopSettlementReconcilerSpy{})
	restarted.start(ctx)
	restartedApp := NewApp()
	restartedApp.cloudRuntime = restarted
	restartedApp.setCurrentActorContext(actor)
	if _, err := restartedApp.CloudBootstrap(); err != nil {
		t.Fatal(err)
	}
	assertDesktopLegacyCanonicalFreezeGap(t, restarted, scope, waitDesktopEventWorker(t, restarted, key))
	afterRestart, err := os.ReadFile(eventPath)
	if err != nil || !bytes.Equal(afterRestart, before) {
		t.Fatalf("restart advanced past canonical contract gap: err %v", err)
	}
	restarted.close()
}

func TestCloudLegacyCanonicalFreezeContractGapReappearsAfterLogoutRelogin(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopLegacyFreezeGapEvents(t, runtime)
	runtime.settlements.bind(&desktopSettlementReconcilerSpy{})
	defer runtime.close()
	runtime.start(ctx)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)

	first, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: first.Account.CloudUserID, BindingGeneration: first.Account.BindingGeneration}
	key := cloudWorkerKey(actor, scope)
	assertDesktopLegacyCanonicalFreezeGap(t, runtime, scope, waitDesktopEventWorker(t, runtime, key))
	runtime.lifecycleMu.Lock()
	_, completed := runtime.workerResults[key]
	runtime.lifecycleMu.Unlock()
	if !completed {
		t.Fatalf("first canonical worker did not cache terminal result for %q", key)
	}

	loggedOut, err := app.CloudLogout()
	if err != nil || loggedOut.Account.Status != cloud.AccountAnonymous {
		t.Fatalf("CloudLogout = %#v, %v", loggedOut, err)
	}
	if _, found, err := runtime.eventState.Load(ctx, scope); err != nil || found {
		t.Fatalf("event checkpoint after logout = found %t, err %v", found, err)
	}
	binding, err := runtime.bindings.ResolveBinding(ctx, actor)
	if err != nil || binding.CloudUserID != scope.CloudUserID || binding.BindingGeneration != 1 {
		t.Fatalf("binding after logout = %#v, %v", binding, err)
	}

	second, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Account.CloudUserID != scope.CloudUserID || second.Account.BindingGeneration != 1 {
		t.Fatalf("same-account relogin changed scope = %#v", second.Account)
	}
	assertDesktopLegacyCanonicalFreezeGap(t, runtime, scope, waitDesktopEventWorker(t, runtime, key))
}

func assertDesktopLegacyCanonicalFreezeGap(
	t *testing.T,
	runtime *cloudRuntime,
	scope cloud.BindingScope,
	workerErr error,
) {
	t.Helper()
	if workerErr == nil || !strings.Contains(workerErr.Error(), "freeze event contract") || !strings.Contains(workerErr.Error(), `requires field "task_id"`) {
		t.Fatalf("legacy canonical event worker error = %v", workerErr)
	}
	checkpoint, found, err := runtime.eventState.Load(context.Background(), scope)
	if err != nil || !found {
		t.Fatalf("legacy canonical checkpoint = %#v, found %t, err %v", checkpoint, found, err)
	}
	if checkpoint.Cursor != "evt_balance_updated" || !checkpoint.HasApplied("evt_settlement_01") || !checkpoint.HasApplied("evt_balance_updated") || checkpoint.HasApplied("evt_freeze_expired") || checkpoint.HasApplied("evt_payment_pending") || checkpoint.HasApplied("evt_sync_updated") {
		t.Fatalf("legacy canonical checkpoint advanced past contract gap = %#v", checkpoint)
	}
}

func configureDesktopLegacyFreezeGapEvents(t *testing.T, runtime *cloudRuntime) {
	t.Helper()
	bundle, err := cloud.LoadCanonicalA0Fixture()
	if err != nil {
		t.Fatal(err)
	}
	events := make([]cloud.Event, 0, len(bundle.Events))
	for _, event := range bundle.Events {
		if event.EventID != "evt_other_user" {
			events = append(events, event)
		}
	}
	source, err := cloud.NewFixtureEventSource(events)
	if err != nil {
		t.Fatal(err)
	}
	eventSync, err := cloud.NewEventSyncApplication(cloud.EventSyncApplicationDependencies{
		Bindings: runtime.bindings, Source: source, State: runtime.eventState,
		Balance: runtime.balance, Tasks: runtime.tasks, Settlements: runtime.settlements, Subscription: runtime.subscription,
		Payment: runtime.paymentRecovery, Sync: runtime.auth, MaxReconnects: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime.eventSource = source
	runtime.eventSync = eventSync
}

func canonicalDesktopTaskRecord(t *testing.T, bundle cloud.FixtureBundle, scope cloud.BindingScope) cloud.TaskRecoveryRecord {
	t.Helper()
	for _, fixture := range bundle.FreezeCases {
		if fixture.Result == nil || fixture.Result.Freeze.Scope != scope {
			continue
		}
		return cloud.TaskRecoveryRecord{
			Scope: scope, TaskID: fixture.Result.Freeze.TaskID, FreezeID: fixture.Result.Freeze.FreezeID,
			IdempotencyKey: fixture.Request.IdempotencyKey,
		}
	}
	t.Fatalf("canonical fixture has no task record for %#v", scope)
	return cloud.TaskRecoveryRecord{}
}

func waitDesktopEventWorker(t *testing.T, runtime *cloudRuntime, key string) error {
	t.Helper()
	runtime.lifecycleMu.Lock()
	if result, ok := runtime.workerResults[key]; ok {
		runtime.lifecycleMu.Unlock()
		return result
	}
	worker := runtime.worker
	if worker == nil || worker.key != key {
		runtime.lifecycleMu.Unlock()
		t.Fatalf("event worker %q was not started", key)
	}
	done := worker.done
	runtime.lifecycleMu.Unlock()
	waitDesktopSignal(t, "event worker completion", done)
	runtime.lifecycleMu.Lock()
	result, ok := runtime.workerResults[key]
	runtime.lifecycleMu.Unlock()
	if !ok {
		t.Fatalf("event worker %q exited without a terminal result", key)
	}
	return result
}

type desktopBlockingEventRunner struct {
	bindings       cloud.BindingResolver
	state          cloud.EventStateRepository
	started        chan *desktopBlockingEventRun
	waitLimit      time.Duration
	mu             sync.Mutex
	runs           map[*desktopBlockingEventRun]struct{}
	releaseOnStart bool
}

type desktopGetCurrentUserFailureClient struct {
	cloud.CloudAccountClient
	err error
}

func (c desktopGetCurrentUserFailureClient) GetCurrentUser(context.Context, cloud.BindingScope) (cloud.AccountState, error) {
	return cloud.AccountState{}, c.err
}

type desktopBlockingEventRun struct {
	ctx           context.Context
	scope         cloud.BindingScope
	canceled      chan struct{}
	release       chan struct{}
	cancelBinding cloud.ProfileBinding
	once          sync.Once
}

func newDesktopBlockingEventRunner(t *testing.T, bindings cloud.BindingResolver, state cloud.EventStateRepository) *desktopBlockingEventRunner {
	t.Helper()
	return &desktopBlockingEventRunner{
		bindings: bindings, state: state, started: make(chan *desktopBlockingEventRun, 4),
		waitLimit: desktopTestWaitDuration(t), runs: make(map[*desktopBlockingEventRun]struct{}),
	}
}

func (r *desktopBlockingEventRunner) Run(ctx context.Context, actor userctx.ActorContext) error {
	binding, err := r.bindings.ResolveBinding(context.Background(), actor)
	if err != nil {
		return err
	}
	scope, err := binding.Scope()
	if err != nil {
		return err
	}
	run := &desktopBlockingEventRun{ctx: ctx, scope: scope, canceled: make(chan struct{}), release: make(chan struct{})}
	r.mu.Lock()
	r.runs[run] = struct{}{}
	releaseOnStart := r.releaseOnStart
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.runs, run)
		r.mu.Unlock()
	}()
	if releaseOnStart {
		run.releaseWorker()
	}
	r.started <- run
	if _, received := desktopReceiveWithin(ctx.Done(), r.waitLimit); !received {
		return errors.New("timed out waiting for blocking event worker cancellation")
	}
	run.cancelBinding, _ = r.bindings.ResolveBinding(context.Background(), actor)
	close(run.canceled)
	if _, received := desktopReceiveWithin(run.release, r.waitLimit); !received {
		return errors.New("timed out waiting to release blocking event worker")
	}
	event := cloud.Event{
		EventID:     "desktop-late-" + scope.CloudUserID,
		EventType:   cloud.EventBalanceUpdated,
		CloudUserID: scope.CloudUserID,
		OccurredAt:  time.Now().UTC(),
		Version:     1,
		Data:        json.RawMessage(`{}`),
	}
	if err := r.state.CommitApplied(context.Background(), scope, event); err != nil {
		return err
	}
	return ctx.Err()
}

func (r *desktopBlockingEventRunner) releaseAllWorkers() {
	r.mu.Lock()
	r.releaseOnStart = true
	runs := make([]*desktopBlockingEventRun, 0, len(r.runs))
	for run := range r.runs {
		runs = append(runs, run)
	}
	r.mu.Unlock()
	for _, run := range runs {
		run.releaseWorker()
	}
}

func (r *desktopBlockingEventRun) releaseWorker() {
	r.once.Do(func() {
		close(r.release)
	})
}

func cleanupDesktopBlockingRuntime(t *testing.T, runtime *cloudRuntime, runner *desktopBlockingEventRunner) {
	t.Helper()
	t.Cleanup(func() {
		runner.releaseAllWorkers()
		runtime.close()
	})
}

func closeCloudRuntimeAfterWorker(t *testing.T, runtime *cloudRuntime, run *desktopBlockingEventRun) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		runtime.close()
		close(done)
	}()
	waitDesktopSignal(t, "event worker cancellation during runtime close", run.canceled)
	run.releaseWorker()
	waitDesktopSignal(t, "runtime close after event worker release", done)
}

func receiveDesktopTest[T any](t *testing.T, stage string, channel <-chan T) T {
	t.Helper()
	wait := desktopTestWaitDuration(t)
	value, received := desktopReceiveWithin(channel, wait)
	if !received {
		t.Fatalf("timed out after %s waiting for %s", wait, stage)
	}
	return value
}

func waitDesktopSignal(t *testing.T, stage string, channel <-chan struct{}) {
	t.Helper()
	receiveDesktopTest(t, stage, channel)
}

func desktopReceiveWithin[T any](channel <-chan T, wait time.Duration) (T, bool) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case value := <-channel:
		return value, true
	case <-timer.C:
		var zero T
		return zero, false
	}
}

func desktopTestWaitDuration(t *testing.T) time.Duration {
	t.Helper()
	wait := 5 * time.Second
	if deadline, ok := t.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("test deadline expired before bounded channel wait")
		}
		if half := remaining / 2; half < wait {
			wait = half
		}
	}
	return wait
}

func authViewBinding(view CloudAccountView, actorID string) cloud.ProfileBinding {
	return cloud.ProfileBinding{
		ActorID: actorID, ProfileID: view.ProfileID, CloudUserID: view.CloudUserID,
		Status: cloud.BindingBound, BindingGeneration: view.BindingGeneration,
	}
}

var _ interface {
	Run(context.Context, userctx.ActorContext) error
} = (*desktopBlockingEventRunner)(nil)
