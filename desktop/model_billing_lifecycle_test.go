package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/modelbillingworker"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

type desktopSettlementReconcilerSpy struct {
	mu    sync.Mutex
	calls []string
	err   error
}

type desktopUsagePendingRecoverySpy struct {
	scopes  []modelbilling.TaskScope
	taskIDs []string
	err     error
}

func (s *desktopUsagePendingRecoverySpy) RecoverUsagePendingTask(_ context.Context, scope modelbilling.TaskScope, taskID string) error {
	s.scopes = append(s.scopes, cloneDesktopBillingTaskScope(scope))
	s.taskIDs = append(s.taskIDs, taskID)
	return s.err
}

type desktopBlockingSettlementReconciler struct {
	started chan struct{}
}

func (r *desktopBlockingSettlementReconciler) ReconcileSettlementEvent(
	ctx context.Context,
	_ userctx.ActorContext,
	_ cloud.BindingScope,
	_ string,
	_ string,
) error {
	r.started <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}

type desktopGatewaySpy struct{ endpoints, streams int }

type desktopCreditConfigSpy struct{ calls int }

func (s *desktopCreditConfigSpy) GetCreditConfig(context.Context, cloud.BindingScope) (modelbilling.CreditConfig, error) {
	s.calls++
	return modelbilling.CreditConfig{}, nil
}

func (s *desktopGatewaySpy) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	s.endpoints++
	return modelbilling.MaasEndpoint("fixture://authorized"), nil
}

func (s *desktopGatewaySpy) Stream(context.Context, cloud.BindingScope, modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	s.streams++
	ch := make(chan modelbilling.GatewayChunk)
	close(ch)
	return ch, nil
}

func (s *desktopSettlementReconcilerSpy) ReconcileSettlementEvent(
	_ context.Context,
	_ userctx.ActorContext,
	binding cloud.BindingScope,
	taskID string,
	settlementID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprintf("%s/%d:%s:%s", binding.CloudUserID, binding.BindingGeneration, taskID, settlementID))
	return s.err
}

func desktopLifecycleAuth(actor userctx.ActorContext, state cloud.AccountStatus, status cloud.BindingStatus, cloudUserID string, generation uint64) cloud.AuthView {
	actor = actor.Normalize()
	return cloud.AuthView{
		State: state,
		Binding: cloud.ProfileBinding{
			ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: cloudUserID,
			Status: status, BindingGeneration: generation,
		},
	}
}

func TestDesktopBillingWorkerManagerReconcilesAuthAndKeepsOfflineProfileDB(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "lifecycle-actor", ProfileID: "lifecycle-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	started := make(chan modelbilling.TaskScope, 3)
	stopped := make(chan modelbilling.TaskScope, 3)
	var recovered []modelbilling.TaskScope
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-ctx.Done()
			stopped <- cloneDesktopBillingTaskScope(scope)
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(_ context.Context, _ *sql.DB, scope modelbilling.TaskScope) error {
		recovered = append(recovered, cloneDesktopBillingTaskScope(scope))
		return nil
	}
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	authenticated := desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)
	if err := manager.reconcileAuth(context.Background(), actor, authenticated); err != nil {
		t.Fatal(err)
	}
	first := receiveDesktopBillingScope(t, "authenticated start", started)
	firstDB := manager.db
	if len(recovered) != 1 || !recovered[0].Equal(first) {
		t.Fatalf("startup recovery = %+v, want [%+v]", recovered, first)
	}
	if err := manager.authorizeSubscription(actor, binding); err != nil {
		t.Fatalf("authenticated authorization = %v", err)
	}

	offline := authenticated
	offline.State = cloud.AccountOffline
	if err := manager.reconcileAuth(context.Background(), actor, offline); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingScope(t, "offline stop", stopped)
	if manager.active != nil || manager.db != firstDB {
		t.Fatalf("offline ownership = active:%v dbPreserved:%v", manager.active != nil, manager.db == firstDB)
	}
	if got, ok := manager.bindingForController(actor); !ok || got != binding {
		t.Fatalf("offline binding = %#v, %t; want retained %#v", got, ok, binding)
	}
	offlineErr := manager.authorizeSubscription(actor, binding)
	var typedOffline *modelbilling.Error
	if code := modelbilling.ErrorCodeOf(offlineErr); code != modelbilling.ErrorGatewayUnavailable || !errors.As(offlineErr, &typedOffline) || !typedOffline.Retryable {
		t.Fatalf("offline gate = %#v, want retryable %s", offlineErr, modelbilling.ErrorGatewayUnavailable)
	}

	expired := authenticated
	expired.State = cloud.AccountExpired
	expired.Binding.Status = cloud.BindingExpired
	if err := manager.reconcileAuth(context.Background(), actor, expired); err != nil {
		t.Fatal(err)
	}
	if code := modelbilling.ErrorCodeOf(manager.authorizeSubscription(actor, binding)); code != modelbilling.ErrorSessionExpired {
		t.Fatalf("expired gate = %s, want %s", code, modelbilling.ErrorSessionExpired)
	}
	if got, ok := manager.bindingForController(actor); ok || got != (cloud.BindingScope{}) {
		t.Fatalf("expired controller binding = %#v, %t; want cleared", got, ok)
	}

	if err := manager.reconcileAuth(context.Background(), actor, authenticated); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingScope(t, "online resume", started)
	if manager.db != firstDB || len(recovered) != 2 {
		t.Fatalf("online resume = dbReused:%v recoveries:%d", manager.db == firstDB, len(recovered))
	}

	unbound := desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingUnbound, "", 0)
	if err := manager.reconcileAuth(context.Background(), actor, unbound); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingScope(t, "unbound stop", stopped)
	if code := modelbilling.ErrorCodeOf(manager.authorizeSubscription(actor, binding)); code != modelbilling.ErrorAuthRequired {
		t.Fatalf("unbound gate = %s, want %s", code, modelbilling.ErrorAuthRequired)
	}
	if _, ok := manager.bindingForController(actor); ok {
		t.Fatal("unbound controller binding was not cleared")
	}
	mismatch := desktopLifecycleAuth(actor, cloud.AccountOffline, cloud.BindingMismatch, binding.CloudUserID, binding.BindingGeneration)
	if err := manager.reconcileAuth(context.Background(), actor, mismatch); err != nil {
		t.Fatal(err)
	}
	if code := modelbilling.ErrorCodeOf(manager.authorizeSubscription(actor, binding)); code != modelbilling.ErrorScopeMismatch {
		t.Fatalf("mismatch gate = %s, want %s", code, modelbilling.ErrorScopeMismatch)
	}
	if _, ok := manager.bindingForController(actor); ok {
		t.Fatal("mismatch controller binding was not cleared")
	}

	anonymous := desktopLifecycleAuth(actor, cloud.AccountAnonymous, cloud.BindingUnbound, "", 0)
	if err := manager.reconcileAuth(context.Background(), actor, anonymous); err != nil {
		t.Fatal(err)
	}
	if code := modelbilling.ErrorCodeOf(manager.authorizeSubscription(actor, binding)); code != modelbilling.ErrorAuthRequired {
		t.Fatalf("logout gate = %s, want %s", code, modelbilling.ErrorAuthRequired)
	}
}

func TestDesktopBillingReconcileClearsCatalogForInvalidBindingButKeepsOfflineBoundCatalog(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "catalog-actor", ProfileID: "catalog-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "catalog-user", BindingGeneration: 3}
	manager := newDesktopBillingWorkerManager(nil)
	app := &App{
		billingWorkerRuntime: manager,
		subscriptionModels:   newDesktopSubscriptionModelCatalog(),
	}
	authenticated := desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)
	if err := app.reconcileDesktopBillingAuth(nil, actor, authenticated); err != nil {
		t.Fatal(err)
	}
	app.subscriptionModels.replace(binding, cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, Key: "catalog-key"}, []string{"catalog-model"})

	offline := authenticated
	offline.State = cloud.AccountOffline
	if err := app.reconcileDesktopBillingAuth(nil, actor, offline); err != nil {
		t.Fatal(err)
	}
	if got, ok := manager.bindingForController(actor); !ok || got != binding {
		t.Fatalf("offline binding = %#v, %t; want retained", got, ok)
	}
	if got := app.subscriptionModels.current(binding); !reflect.DeepEqual(got, []string{"catalog-model"}) {
		t.Fatalf("offline catalog = %#v; want retained", got)
	}

	invalid := []cloud.AuthView{
		desktopLifecycleAuth(actor, cloud.AccountExpired, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration),
		desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingExpired, binding.CloudUserID, binding.BindingGeneration),
		desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingMismatch, binding.CloudUserID, binding.BindingGeneration),
		desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingUnbound, "", 0),
	}
	for index, auth := range invalid {
		if err := app.reconcileDesktopBillingAuth(nil, actor, authenticated); err != nil {
			t.Fatal(err)
		}
		app.subscriptionModels.replace(binding, cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, Key: "catalog-key"}, []string{"catalog-model"})
		if err := app.reconcileDesktopBillingAuth(nil, actor, auth); err != nil {
			t.Fatalf("invalid transition %d: %v", index, err)
		}
		if _, ok := manager.bindingForController(actor); ok {
			t.Fatalf("invalid transition %d retained controller binding", index)
		}
		if got := app.subscriptionModels.current(binding); len(got) != 0 {
			t.Fatalf("invalid transition %d retained catalog %#v", index, got)
		}
	}
}

func TestDesktopBillingWorkerManagerSwitchesOnlyAfterOldWorkerStops(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "switch-actor", ProfileID: "switch-profile"}.Normalize()
	first := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	second := first
	second.BindingGeneration = 2

	var mu sync.Mutex
	var events []string
	started := make(chan struct{}, 2)
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		mu.Lock()
		events = append(events, fmt.Sprintf("build:%d", scope.CloudBinding.BindingGeneration))
		mu.Unlock()
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			mu.Lock()
			events = append(events, fmt.Sprintf("stop:%d", scope.CloudBinding.BindingGeneration))
			mu.Unlock()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(_ context.Context, _ *sql.DB, scope modelbilling.TaskScope) error {
		mu.Lock()
		events = append(events, fmt.Sprintf("recover:%d", scope.CloudBinding.BindingGeneration))
		mu.Unlock()
		return nil
	}
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, first.CloudUserID, first.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "first worker", started)
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, second.CloudUserID, second.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "second worker", started)

	mu.Lock()
	got := append([]string(nil), events...)
	mu.Unlock()
	want := []string{"build:1", "recover:1", "stop:1", "build:2", "recover:2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("switch order = %v, want %v", got, want)
	}
	if code := modelbilling.ErrorCodeOf(manager.authorizeSubscription(actor, first)); code != modelbilling.ErrorScopeMismatch {
		t.Fatalf("old generation gate = %s, want %s", code, modelbilling.ErrorScopeMismatch)
	}
	if err := manager.authorizeSubscription(actor, second); err != nil {
		t.Fatalf("new generation gate = %v", err)
	}
}

func TestDesktopBillingWorkerManagerFailedSwitchStaysInactive(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "failed-switch-actor", ProfileID: "failed-switch-profile"}.Normalize()
	first := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	second := first
	second.BindingGeneration = 2
	started := make(chan struct{}, 2)
	builds := 0
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		builds++
		if builds == 2 {
			return desktopBillingWorkerRuntime{}, errors.New("candidate build failed")
		}
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, first.CloudUserID, first.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "old worker", started)
	err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, second.CloudUserID, second.BindingGeneration))
	if err == nil || !strings.Contains(err.Error(), "candidate build failed") {
		t.Fatalf("failed switch = %v", err)
	}
	if manager.active != nil || manager.authorized {
		t.Fatalf("failed switch ownership = active:%v authorized:%v", manager.active != nil, manager.authorized)
	}
	select {
	case <-started:
		t.Fatal("failed switch launched a second worker")
	default:
	}
}

func TestDesktopBillingWorkerManagerRecoversBYOKWithoutCloudSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "byok-recovery-actor", ProfileID: "byok-recovery-profile"}.Normalize()
	var scopes []modelbilling.TaskScope
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{}, errors.New("subscription worker must not be built")
	})
	manager.recoverScope = func(_ context.Context, _ *sql.DB, scope modelbilling.TaskScope) error {
		scopes = append(scopes, cloneDesktopBillingTaskScope(scope))
		return nil
	}
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	if err := manager.recoverBYOK(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 1 || scopes[0].ActorID != actor.ActorID || scopes[0].ProfileID != actor.ProfileID || scopes[0].CloudBinding != nil {
		t.Fatalf("BYOK recovery scopes = %+v", scopes)
	}
	if manager.db == nil || manager.profileID != actor.ProfileID || manager.active != nil {
		t.Fatalf("BYOK recovery ownership = db:%v profile:%q active:%v", manager.db != nil, manager.profileID, manager.active != nil)
	}
}

func TestDesktopBillingWorkerSettlementProxyUsesOnlyCurrentAuthorizedScope(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "event-actor", ProfileID: "event-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "event-cloud", BindingGeneration: 3}
	spy := &desktopSettlementReconcilerSpy{}
	started := make(chan struct{}, 1)
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{
			run: func(ctx context.Context) error {
				started <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			},
			settlements: spy,
		}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "event worker", started)

	wrong := binding
	wrong.BindingGeneration++
	if code := modelbilling.ErrorCodeOf(manager.ReconcileSettlementEvent(context.Background(), actor, wrong, "task", "settlement")); code != modelbilling.ErrorScopeMismatch {
		t.Fatalf("wrong event scope = %s, want %s", code, modelbilling.ErrorScopeMismatch)
	}
	if err := manager.ReconcileSettlementEvent(context.Background(), actor, binding, "task", "settlement"); err != nil {
		t.Fatal(err)
	}
	spy.mu.Lock()
	got := append([]string(nil), spy.calls...)
	spy.mu.Unlock()
	if want := []string{"event-cloud/3:task:settlement"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("settlement calls = %v, want %v", got, want)
	}

	offline := desktopLifecycleAuth(actor, cloud.AccountOffline, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)
	if err := manager.reconcileAuth(context.Background(), actor, offline); err != nil {
		t.Fatal(err)
	}
	offlineEventErr := manager.ReconcileSettlementEvent(context.Background(), actor, binding, "task", "settlement")
	var typedOfflineEvent *modelbilling.Error
	if code := modelbilling.ErrorCodeOf(offlineEventErr); code != modelbilling.ErrorGatewayUnavailable || !errors.As(offlineEventErr, &typedOfflineEvent) || !typedOfflineEvent.Retryable {
		t.Fatalf("offline event = %#v, want retryable %s", offlineEventErr, modelbilling.ErrorGatewayUnavailable)
	}
}

func TestDesktopBillingWorkerRecoveryProxyUsesOnlyCurrentAuthorizedScope(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "recovery-actor", ProfileID: "recovery-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "recovery-cloud", BindingGeneration: 3}
	spy := &desktopUsagePendingRecoverySpy{}
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{
			run: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			recovery: spy,
		}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	if err := manager.RecoverUsagePendingTask(context.Background(), scope, "task-recovery"); err != nil {
		t.Fatal(err)
	}
	if len(spy.taskIDs) != 1 || spy.taskIDs[0] != "task-recovery" || !spy.scopes[0].Equal(scope) {
		t.Fatalf("recovery calls = ids:%v scopes:%+v", spy.taskIDs, spy.scopes)
	}
	wrong := cloneDesktopBillingTaskScope(scope)
	wrong.CloudBinding.BindingGeneration++
	if code := modelbilling.ErrorCodeOf(manager.RecoverUsagePendingTask(context.Background(), wrong, "task-wrong")); code != modelbilling.ErrorScopeMismatch {
		t.Fatalf("wrong recovery scope = %s, want %s", code, modelbilling.ErrorScopeMismatch)
	}
	if len(spy.taskIDs) != 1 {
		t.Fatalf("unauthorized recovery reached coordinator: %v", spy.taskIDs)
	}
}

func TestReconcileDesktopBillingAuthStopsEventBeforeBillingTransition(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "lock-order-actor", ProfileID: "lock-order-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "lock-order-cloud", BindingGeneration: 1}
	settlements := &desktopBlockingSettlementReconciler{started: make(chan struct{}, 1)}
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{
			run: func(workerCtx context.Context) error {
				<-workerCtx.Done()
				return workerCtx.Err()
			},
			settlements: settlements,
		}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(ctx)
	defer func() { _ = manager.close() }()
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}

	eventCtx, cancelEvent := context.WithCancel(ctx)
	eventDone := make(chan struct{})
	runtime := &cloudRuntime{}
	runtime.worker = &cloudEventWorker{cancel: cancelEvent, done: eventDone}
	defer cancelEvent()
	go func() {
		_ = manager.ReconcileSettlementEvent(eventCtx, actor, binding, "task-lock-order", "settlement-lock-order")
		close(eventDone)
	}()
	receiveDesktopBillingSignal(t, "blocking settlement event", settlements.started)

	app := NewApp()
	app.ctx = ctx
	app.billingWorkerRuntime = manager
	offline := desktopLifecycleAuth(actor, cloud.AccountOffline, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)
	completed := make(chan error, 1)
	go func() {
		completed <- app.reconcileDesktopBillingAuth(runtime, actor, offline)
	}()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(250 * time.Millisecond):
		cancelEvent()
		<-completed
		t.Fatal("billing transition waited on an event that it had not canceled")
	}
}

func TestDesktopShutdownStopsEventBeforeClosingBillingManager(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "shutdown-order-actor", ProfileID: "shutdown-order-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "shutdown-order-cloud", BindingGeneration: 1}
	settlements := &desktopBlockingSettlementReconciler{started: make(chan struct{}, 1)}
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{
			run: func(workerCtx context.Context) error {
				<-workerCtx.Done()
				return workerCtx.Err()
			},
			settlements: settlements,
		}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(ctx)
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}

	eventCtx, cancelEvent := context.WithCancel(ctx)
	eventDone := make(chan struct{})
	runtime := &cloudRuntime{}
	runtime.worker = &cloudEventWorker{cancel: cancelEvent, done: eventDone}
	defer cancelEvent()
	go func() {
		_ = manager.ReconcileSettlementEvent(eventCtx, actor, binding, "task-shutdown-order", "settlement-shutdown-order")
		close(eventDone)
	}()
	receiveDesktopBillingSignal(t, "shutdown blocking settlement event", settlements.started)

	app := NewApp()
	app.cloudRuntime = runtime
	app.billingWorkerRuntime = manager
	completed := make(chan struct{})
	go func() {
		app.shutdown(ctx)
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(250 * time.Millisecond):
		cancelEvent()
		<-completed
		t.Fatal("shutdown waited for billing before canceling its event caller")
	}
}

func TestDesktopSubscriptionControllerAdaptersRejectInactiveAndOldScopes(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "adapter-actor", ProfileID: "adapter-profile"}.Normalize()
	first := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "adapter-cloud", BindingGeneration: 1}
	second := first
	second.BindingGeneration++
	started := make(chan struct{}, 2)
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	bootstrap := desktopFixtureSubscriptionBootstrap{authorization: manager}
	if _, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("inactive bootstrap = %v, want auth_required", err)
	}
	innerGateway := &desktopGatewaySpy{}
	gateway := desktopAuthorizedSubscriptionGateway{inner: innerGateway, actor: actor, authorization: manager}
	if _, err := gateway.EndpointFor(modelbilling.GatewayRequest{}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired || innerGateway.endpoints != 0 {
		t.Fatalf("inactive endpoint = %v, inner calls=%d", err, innerGateway.endpoints)
	}

	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, first.CloudUserID, first.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "first adapter worker", started)
	if _, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{}); err != nil {
		t.Fatalf("active bootstrap = %v", err)
	}
	if _, err := gateway.EndpointFor(modelbilling.GatewayRequest{}); err != nil || innerGateway.endpoints != 1 {
		t.Fatalf("active endpoint = %v, inner calls=%d", err, innerGateway.endpoints)
	}

	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, second.CloudUserID, second.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "second adapter worker", started)
	current, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{})
	if err != nil || current.Scope != second {
		t.Fatalf("new bootstrap = %+v, %v; want %+v", current, err, second)
	}
	if _, err := gateway.Stream(context.Background(), first, modelbilling.GatewayRequest{ProviderRequest: provider.Request{}}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorScopeMismatch || innerGateway.streams != 0 {
		t.Fatalf("old gateway stream = %v, inner calls=%d", err, innerGateway.streams)
	}
}

func TestDesktopSubscriptionControllerBuiltUnboundUsesCurrentBindingForNewTasksAndCalls(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, _ := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_DYNAMIC_BINDING_KEY")
	first := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "dynamic-cloud-a", BindingGeneration: 7}
	second := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "dynamic-cloud-b", BindingGeneration: 8}
	started := make(chan struct{}, 2)
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAnonymous, cloud.BindingUnbound, "", 0)); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.billingWorkerRuntime = manager
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	bootstrap := runtime.initializer.inner.Subscriptions.(desktopFixtureSubscriptionBootstrap)
	if _, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("unbound bootstrap = %v, want auth_required", err)
	}

	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, first.CloudUserID, first.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "first dynamic binding worker", started)
	firstBootstrap, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{})
	if err != nil || firstBootstrap.Scope != first {
		t.Fatalf("first bootstrap = %+v, %v; want %+v", firstBootstrap, err, first)
	}
	firstTask, err := runtime.initializer.InitializeTaskContext(context.Background(), controlTaskContextRequest(actor, modelbilling.BillingRouteSelection{ModelRef: "fixture/model_fixture_01", BillingSource: modelbilling.BillingSubscriptionPool}))
	if err != nil || firstTask.Scope.CloudBinding == nil || *firstTask.Scope.CloudBinding != first {
		t.Fatalf("first task = %+v, %v; want binding %+v", firstTask, err, first)
	}

	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, second.CloudUserID, second.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "second dynamic binding worker", started)
	secondBootstrap, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{})
	if err != nil || secondBootstrap.Scope != second {
		t.Fatalf("second bootstrap = %+v, %v; want %+v", secondBootstrap, err, second)
	}
	secondTask, err := runtime.initializer.InitializeTaskContext(context.Background(), controlTaskContextRequest(actor, modelbilling.BillingRouteSelection{ModelRef: "fixture/model_fixture_01", BillingSource: modelbilling.BillingSubscriptionPool}))
	if err != nil || secondTask.Scope.CloudBinding == nil || *secondTask.Scope.CloudBinding != second {
		t.Fatalf("second task = %+v, %v; want binding %+v", secondTask, err, second)
	}
	bundle, err := modelbilling.LoadEmbeddedGatewayFixture()
	if err != nil {
		t.Fatal(err)
	}
	var gatewayRequest modelbilling.GatewayRequest
	for _, fixture := range bundle.Cases {
		if fixture.Name == "desktop_stream_zero" {
			gatewayRequest = fixture.Request
			break
		}
	}
	if err := gatewayRequest.Validate(); err != nil {
		t.Fatalf("desktop gateway fixture request = %+v, %v", gatewayRequest, err)
	}
	gateway := runtime.dispatcher.Gateway.(desktopAuthorizedSubscriptionGateway)
	if _, err := gateway.EndpointFor(gatewayRequest); err != nil {
		t.Fatalf("new generation endpoint = %v", err)
	}
	stream, err := gateway.Stream(context.Background(), second, gatewayRequest)
	if err != nil {
		t.Fatalf("new generation gateway = %v", err)
	}
	for range stream {
	}

	if _, err := runtime.initializer.inner.Freeze.CreateFreeze(context.Background(), cloud.FreezeRequest{Scope: first}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorScopeMismatch {
		t.Fatalf("old generation freeze = %v, want scope_mismatch", err)
	}
	if _, err := gateway.Stream(context.Background(), first, modelbilling.GatewayRequest{}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorScopeMismatch {
		t.Fatalf("old generation gateway = %v, want scope_mismatch", err)
	}
}

func TestDesktopBillingWorkerCloseTimeoutRevokesControllerAndEventAccess(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "close-timeout-actor", ProfileID: "close-timeout-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "close-timeout-cloud", BindingGeneration: 1}
	workerStarted := make(chan struct{}, 1)
	releaseWorker := make(chan struct{})
	var releaseOnce sync.Once
	settlements := &desktopSettlementReconcilerSpy{}
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{
			run: func(context.Context) error {
				workerStarted <- struct{}{}
				<-releaseWorker
				return nil
			},
			settlements: settlements,
		}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = 10 * time.Millisecond
	manager.start(context.Background())
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseWorker) })
		manager.waitTimeout = time.Second
		_ = manager.close()
	})
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "close timeout worker", workerStarted)
	innerGateway := &desktopGatewaySpy{}
	gateway := desktopAuthorizedSubscriptionGateway{inner: innerGateway, actor: actor, authorization: manager}

	if err := manager.close(); err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("close timeout = %v", err)
	}
	if _, err := gateway.EndpointFor(modelbilling.GatewayRequest{}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorSessionExpired || innerGateway.endpoints != 0 {
		t.Fatalf("post-close gateway = %v, inner calls=%d", err, innerGateway.endpoints)
	}
	if err := manager.ReconcileSettlementEvent(context.Background(), actor, binding, "task", "settlement"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorSessionExpired {
		t.Fatalf("post-close settlement = %v", err)
	}
	settlements.mu.Lock()
	settlementCalls := len(settlements.calls)
	settlements.mu.Unlock()
	if settlementCalls != 0 {
		t.Fatalf("post-close settlement inner calls=%d", settlementCalls)
	}
	releaseOnce.Do(func() { close(releaseWorker) })
	manager.waitTimeout = time.Second
	if err := manager.close(); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopSubscriptionCreditAndFreezeRecheckAuthorization(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "prerequisite-actor", ProfileID: "prerequisite-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "prerequisite-cloud", BindingGeneration: 1}
	started := make(chan struct{}, 1)
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	creditInner := &desktopCreditConfigSpy{}
	credit := desktopFixtureCreditConfigProvider{inner: creditInner, actor: actor, authorization: manager}
	freeze := desktopFixtureFreezeClient{actor: actor, authorization: manager}
	if _, err := credit.GetCreditConfig(context.Background(), binding); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired || creditInner.calls != 0 {
		t.Fatalf("inactive credit = %v, inner calls=%d", err, creditInner.calls)
	}
	if _, err := freeze.CreateFreeze(context.Background(), cloud.FreezeRequest{Scope: binding}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("inactive freeze = %v", err)
	}
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "prerequisite worker", started)
	if _, err := credit.GetCreditConfig(context.Background(), binding); err != nil || creditInner.calls != 1 {
		t.Fatalf("active credit = %v, inner calls=%d", err, creditInner.calls)
	}
	offline := desktopLifecycleAuth(actor, cloud.AccountOffline, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)
	if err := manager.reconcileAuth(context.Background(), actor, offline); err != nil {
		t.Fatal(err)
	}
	if _, err := credit.GetCreditConfig(context.Background(), binding); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorGatewayUnavailable || creditInner.calls != 1 {
		t.Fatalf("offline credit = %v, inner calls=%d", err, creditInner.calls)
	}
	if _, err := freeze.CreateFreeze(context.Background(), cloud.FreezeRequest{Scope: binding}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorGatewayUnavailable {
		t.Fatalf("offline freeze = %v", err)
	}
}

func TestDesktopControllerBuildDoesNotActivateSubscriptionWorker(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_LIFECYCLE_BUILD_KEY")
	builds := 0
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		builds++
		return desktopBillingWorkerRuntime{}, errors.New("controller build must not construct app worker")
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	app := NewApp()
	app.billingWorkerRuntime = manager
	ctrl, err := app.buildDesktopController(
		context.Background(),
		boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
		actor, root, modelRef, string(modelbilling.BillingSubscriptionPool),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctrl.Close()
	if builds != 0 || manager.active != nil {
		t.Fatalf("controller build activated app worker: builds=%d active=%v", builds, manager.active != nil)
	}
}

func TestDesktopSubscriptionControllerResolvesCurrentBindingAtInvocation(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, _ := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_BINDING_SNAPSHOT_KEY")
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "current-cloud-user", BindingGeneration: 9}
	started := make(chan struct{}, 1)
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	if err := manager.reconcileAuth(context.Background(), actor, desktopLifecycleAuth(actor, cloud.AccountAuthenticated, cloud.BindingBound, binding.CloudUserID, binding.BindingGeneration)); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "binding snapshot worker", started)
	app := NewApp()
	app.billingWorkerRuntime = manager
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	bootstrap := runtime.initializer.inner.Subscriptions.(desktopFixtureSubscriptionBootstrap)
	gateway := runtime.dispatcher.Gateway.(desktopAuthorizedSubscriptionGateway)
	resolved, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{})
	if err != nil || resolved.Scope != binding || gateway.materialize == nil {
		t.Fatalf("controller binding = bootstrap:%+v err:%v gatewayDynamic:%v, want %+v", resolved, err, gateway.materialize != nil, binding)
	}
}

func TestCloudSettlementReconcilerProxyForwardsOnlyAfterBinding(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "proxy-actor", ProfileID: "proxy-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "proxy-cloud", BindingGeneration: 1}
	proxy := &cloudSettlementReconcilerProxy{}
	if code := modelbilling.ErrorCodeOf(proxy.ReconcileSettlementEvent(context.Background(), actor, binding, "task", "settlement")); code != modelbilling.ErrorAuthRequired {
		t.Fatalf("unbound settlement proxy = %s, want %s", code, modelbilling.ErrorAuthRequired)
	}
	spy := &desktopSettlementReconcilerSpy{}
	proxy.bind(spy)
	if err := proxy.ReconcileSettlementEvent(context.Background(), actor, binding, "task", "settlement"); err != nil {
		t.Fatal(err)
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if want := []string{"proxy-cloud/1:task:settlement"}; !reflect.DeepEqual(spy.calls, want) {
		t.Fatalf("proxy calls = %v, want %v", spy.calls, want)
	}
}

func TestCloudAccountViewIncludesBindingStatus(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "view-actor", ProfileID: "view-profile"}.Normalize()
	auth := desktopLifecycleAuth(actor, cloud.AccountOffline, cloud.BindingExpired, "view-cloud", 4)
	auth.User.Role = "admin"
	view := cloudAccountView(actor.ProfileID, auth, cloud.AccountState{})
	if view.BindingStatus != cloud.BindingExpired || view.Role != "admin" {
		t.Fatalf("account view = %#v, want expired binding and admin role", view)
	}
}

func TestCloudLifecycleActivatesSwitchesAndQuiescesSubscriptionWorker(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "cloud-lifecycle"}.Normalize()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runtime.start(ctx)
	defer runtime.close()
	started := make(chan modelbilling.TaskScope, 3)
	stopped := make(chan modelbilling.TaskScope, 3)
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-ctx.Done()
			stopped <- cloneDesktopBillingTaskScope(scope)
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(ctx)
	defer func() { _ = manager.close() }()
	runtime.settlements.bind(manager)
	app := NewApp()
	app.ctx = ctx
	app.cloudRuntime = runtime
	app.billingWorkerRuntime = manager
	app.setCurrentActorContext(actor)

	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	first := receiveDesktopBillingScope(t, "login worker", started)
	if loggedIn.Account.Status != cloud.AccountAuthenticated || loggedIn.Account.BindingStatus != cloud.BindingBound || first.CloudBinding == nil || first.CloudBinding.CloudUserID != "usr_01" || first.CloudBinding.BindingGeneration != 1 {
		t.Fatalf("login account/scope = %#v / %+v", loggedIn.Account, first)
	}

	switched, err := app.CloudSwitchAccount(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "switch old stop", stopped); !got.Equal(first) {
		t.Fatalf("switch stopped = %+v, want %+v", got, first)
	}
	second := receiveDesktopBillingScope(t, "switch new start", started)
	if switched.Account.CloudUserID != "usr_02" || switched.Account.BindingGeneration != 2 || second.CloudBinding == nil || second.CloudBinding.CloudUserID != "usr_02" || second.CloudBinding.BindingGeneration != 2 {
		t.Fatalf("switch account/scope = %#v / %+v", switched.Account, second)
	}

	loggedOut, err := app.CloudLogout()
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "logout stop", stopped); !got.Equal(second) {
		t.Fatalf("logout stopped = %+v, want %+v", got, second)
	}
	if loggedOut.Account.Status != cloud.AccountAnonymous || manager.active != nil || manager.authorized {
		t.Fatalf("logout state = %#v active:%v authorized:%v", loggedOut.Account, manager.active != nil, manager.authorized)
	}
}

func TestSetActiveTabQuiescesOldProfileBeforeRecoveringNewProfile(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	oldActor := userctx.ActorContext{ActorID: "tab-old-actor", ProfileID: "tab-old-profile"}.Normalize()
	newActor := userctx.ActorContext{ActorID: "tab-new-actor", ProfileID: "tab-new-profile"}.Normalize()

	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	newAuth, err := runtime.auth.Login(ctx, newActor, cloud.LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	newBinding, err := newAuth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}

	type stoppedWorker struct {
		scope modelbilling.TaskScope
		actor userctx.ActorContext
	}
	started := make(chan modelbilling.TaskScope, 2)
	stopped := make(chan stoppedWorker, 2)
	var app *App
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(workerCtx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-workerCtx.Done()
			stopped <- stoppedWorker{scope: cloneDesktopBillingTaskScope(scope), actor: app.cloudActorContext()}
			return workerCtx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(ctx)
	runtime.start(ctx)
	runtime.settlements.bind(manager)
	defer func() {
		runtime.close()
		_ = manager.close()
	}()

	app = NewApp()
	app.ctx = ctx
	app.cloudRuntime = runtime
	app.billingWorkerRuntime = manager
	app.tabRuntimes = map[string]*WorkspaceTab{
		"old": {ID: "old", actor: oldActor},
		"new": {ID: "new", actor: newActor},
	}
	app.tabOrder = []string{"old", "new"}
	app.mu.Lock()
	app.setActiveTabIDLocked("old")
	app.saveTabsLocked()
	app.mu.Unlock()
	oldBinding := cloud.BindingScope{ProfileID: oldActor.ProfileID, CloudUserID: "old-cloud-user", BindingGeneration: 1}
	if err := manager.activate(oldActor, oldBinding); err != nil {
		t.Fatal(err)
	}
	oldScope := receiveDesktopBillingScope(t, "old profile worker", started)

	if err := app.SetActiveTab("new"); err != nil {
		t.Fatal(err)
	}
	oldStopped := receiveDesktopTest(t, "old profile stop", stopped)
	if !oldStopped.scope.Equal(oldScope) {
		t.Fatalf("stopped scope = %+v, want %+v", oldStopped.scope, oldScope)
	}
	if oldStopped.actor.ActorID != oldActor.ActorID || oldStopped.actor.ProfileID != oldActor.ProfileID {
		t.Fatalf("actor visible while old worker stopped = %+v, want old actor %+v", oldStopped.actor, oldActor)
	}
	newScope := receiveDesktopBillingScope(t, "new profile worker", started)
	if newScope.ActorID != newActor.ActorID || newScope.ProfileID != newActor.ProfileID || newScope.CloudBinding == nil || *newScope.CloudBinding != newBinding {
		t.Fatalf("new profile scope = %+v, want actor %+v binding %+v", newScope, newActor, newBinding)
	}
	if app.cachedActiveTabID() != "new" || manager.active == nil || !manager.authorized {
		t.Fatalf("new profile ownership = activeTab:%q worker:%v authorized:%v", app.cachedActiveTabID(), manager.active != nil, manager.authorized)
	}
}

func TestCloseActiveTabHandsOffToHeartbeatActorBeforeRemoval(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	oldActor := userctx.ActorContext{ActorID: "local-owner", ProfileID: "shared-profile"}.Normalize()
	heartbeatActor := userctx.ActorContext{ActorID: "heartbeat-service", ProfileID: oldActor.ProfileID}.Normalize()

	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	heartbeatAuth, err := runtime.auth.Login(ctx, heartbeatActor, cloud.LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	heartbeatBinding, err := heartbeatAuth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan modelbilling.TaskScope, 2)
	stopped := make(chan modelbilling.TaskScope, 2)
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(workerCtx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-workerCtx.Done()
			stopped <- cloneDesktopBillingTaskScope(scope)
			return workerCtx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(ctx)
	runtime.start(ctx)
	runtime.settlements.bind(manager)
	defer func() {
		runtime.close()
		_ = manager.close()
	}()

	app := NewApp()
	app.ctx = ctx
	app.cloudRuntime = runtime
	app.billingWorkerRuntime = manager
	app.tabRuntimes = map[string]*WorkspaceTab{
		"local":     {ID: "local", actor: oldActor},
		"heartbeat": {ID: "heartbeat", actor: heartbeatActor},
	}
	app.tabOrder = []string{"local", "heartbeat"}
	app.mu.Lock()
	app.setActiveTabIDLocked("local")
	app.saveTabsLocked()
	app.mu.Unlock()
	oldBinding := cloud.BindingScope{ProfileID: oldActor.ProfileID, CloudUserID: "old-cloud-user", BindingGeneration: 1}
	if err := manager.activate(oldActor, oldBinding); err != nil {
		t.Fatal(err)
	}
	oldScope := receiveDesktopBillingScope(t, "old actor worker", started)

	if err := app.CloseTab("local"); err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "old actor stop", stopped); !got.Equal(oldScope) {
		t.Fatalf("stopped scope = %+v, want %+v", got, oldScope)
	}
	newScope := receiveDesktopBillingScope(t, "heartbeat actor worker", started)
	if newScope.ActorID != heartbeatActor.ActorID || newScope.ProfileID != heartbeatActor.ProfileID ||
		newScope.CloudBinding == nil || *newScope.CloudBinding != heartbeatBinding {
		t.Fatalf("heartbeat scope = %+v, want actor %+v binding %+v", newScope, heartbeatActor, heartbeatBinding)
	}
	if app.cachedActiveTabID() != "heartbeat" || app.tabRuntimes["local"] != nil || manager.active == nil || !manager.authorized {
		t.Fatalf("close handoff = active:%q oldPresent:%v worker:%v authorized:%v",
			app.cachedActiveTabID(), app.tabRuntimes["local"] != nil, manager.active != nil, manager.authorized)
	}
}

func TestOpenTopicActivationHandsOffFromHeartbeatActor(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	localActor := userctx.ActorContext{ActorID: "local-owner", ProfileID: "local-profile"}.Normalize()
	heartbeatActor := userctx.ActorContext{ActorID: "heartbeat-service", ProfileID: "heartbeat-profile"}.Normalize()

	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	localAuth, err := runtime.auth.Login(ctx, localActor, cloud.LoginInput{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	localBinding, err := localAuth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	heartbeatAuth, err := runtime.auth.Login(ctx, heartbeatActor, cloud.LoginInput{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatal(err)
	}
	heartbeatBinding, err := heartbeatAuth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan modelbilling.TaskScope, 2)
	stopped := make(chan modelbilling.TaskScope, 2)
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(workerCtx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-workerCtx.Done()
			stopped <- cloneDesktopBillingTaskScope(scope)
			return workerCtx.Err()
		}}, nil
	})
	manager.recoverScope = func(context.Context, *sql.DB, modelbilling.TaskScope) error { return nil }
	manager.waitTimeout = time.Second
	manager.start(ctx)
	runtime.start(ctx)
	runtime.settlements.bind(manager)
	defer func() {
		runtime.close()
		_ = manager.close()
	}()

	app := NewApp()
	app.ctx = ctx
	app.cloudRuntime = runtime
	app.billingWorkerRuntime = manager
	app.tabRuntimes = map[string]*WorkspaceTab{
		"heartbeat": {ID: "heartbeat", actor: heartbeatActor},
		"local": {
			ID:            "local",
			Scope:         "global",
			WorkspaceRoot: globalWorkspaceRoot(),
			TopicID:       "local-topic",
			actor:         localActor,
		},
	}
	app.tabOrder = []string{"heartbeat", "local"}
	app.mu.Lock()
	app.setActiveTabIDLocked("heartbeat")
	app.saveTabsLocked()
	app.mu.Unlock()
	if err := manager.activate(heartbeatActor, heartbeatBinding); err != nil {
		t.Fatal(err)
	}
	heartbeatScope := receiveDesktopBillingScope(t, "heartbeat actor worker", started)

	opened, err := app.openTopicTabWithActivationAndActor("global", "", "local-topic", "", true, localActor)
	if err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "heartbeat actor stop", stopped); !got.Equal(heartbeatScope) {
		t.Fatalf("stopped scope = %+v, want %+v", got, heartbeatScope)
	}
	localScope := receiveDesktopBillingScope(t, "local actor worker", started)
	if localScope.ActorID != localActor.ActorID || localScope.ProfileID != localActor.ProfileID ||
		localScope.CloudBinding == nil || *localScope.CloudBinding != localBinding {
		t.Fatalf("local scope = %+v, want actor %+v binding %+v", localScope, localActor, localBinding)
	}
	if app.cachedActiveTabID() != opened.ID || manager.active == nil || !manager.authorized {
		t.Fatalf("open handoff = active:%q opened:%q worker:%v authorized:%v",
			app.cachedActiveTabID(), opened.ID, manager.active != nil, manager.authorized)
	}

	localSessionDir := desktopSessionDirForActor(localActor, globalWorkspaceRoot())
	localSessionPath, err := createEmptySessionFile(localSessionDir, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	app.mu.Lock()
	app.tabRuntimes[opened.ID].SessionPath = localSessionPath
	app.mu.Unlock()
	removed, fallback, err := app.removeSessionRuntimeBindings(localSessionDir, localSessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || fallback.needs {
		t.Fatalf("removed runtimes = %d, fallback = %+v", len(removed), fallback)
	}
	if got := receiveDesktopBillingScope(t, "local actor stop after delete", stopped); !got.Equal(localScope) {
		t.Fatalf("stopped scope = %+v, want %+v", got, localScope)
	}
	restoredHeartbeatScope := receiveDesktopBillingScope(t, "heartbeat actor restart after delete", started)
	if !restoredHeartbeatScope.Equal(heartbeatScope) {
		t.Fatalf("restored heartbeat scope = %+v, want %+v", restoredHeartbeatScope, heartbeatScope)
	}
	if app.cachedActiveTabID() != "heartbeat" || app.tabRuntimes[opened.ID] != nil || manager.active == nil || !manager.authorized {
		t.Fatalf("delete handoff = active:%q removedPresent:%v worker:%v authorized:%v",
			app.cachedActiveTabID(), app.tabRuntimes[opened.ID] != nil, manager.active != nil, manager.authorized)
	}
}

func TestNewAppStartupActorIsDefaultWithoutExplicitBootstrapActor(t *testing.T) {
	app := NewApp()
	if app.currentActorContextSpecified() {
		t.Fatal("NewApp unexpectedly marked a persisted actor as explicit")
	}
	got := app.currentActorContext()
	want := userctx.Default("")
	if got.ActorID != want.ActorID || got.ProfileID != want.ProfileID || got.AuthMethod != want.AuthMethod {
		t.Fatalf("NewApp actor = %+v, want default-only startup actor %+v", got, want)
	}
}

func TestDesktopStartupRecoversBYOKWhenCloudIsUnavailable(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "startup-local", ProfileID: "startup-local-profile"}.Normalize()
	var scopes []modelbilling.TaskScope
	manager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{}, errors.New("disabled cloud must not build subscription worker")
	})
	manager.recoverScope = func(_ context.Context, _ *sql.DB, scope modelbilling.TaskScope) error {
		scopes = append(scopes, cloneDesktopBillingTaskScope(scope))
		return nil
	}
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	app := NewApp()
	app.cloudRuntime = &cloudRuntime{}
	app.billingWorkerRuntime = manager
	app.setCurrentActorContext(actor)
	app.recoverDesktopStartupState(context.Background())
	if len(scopes) != 1 || scopes[0].CloudBinding != nil || scopes[0].ActorID != actor.ActorID || scopes[0].ProfileID != actor.ProfileID {
		t.Fatalf("disabled-cloud startup scopes = %+v", scopes)
	}
}

func TestDesktopStartupRecoversAuthAndStartsWorkerWithoutController(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "startup-cloud-profile"}.Normalize()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.auth.Login(ctx, actor, cloud.LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runtime.start(ctx)
	defer runtime.close()
	started := make(chan modelbilling.TaskScope, 1)
	var recoveryScopes []modelbilling.TaskScope
	var recoveryDBs []*sql.DB
	var workerDB *sql.DB
	manager := newDesktopBillingWorkerManager(func(db *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		workerDB = db
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.recoverScope = func(_ context.Context, db *sql.DB, scope modelbilling.TaskScope) error {
		recoveryDBs = append(recoveryDBs, db)
		recoveryScopes = append(recoveryScopes, cloneDesktopBillingTaskScope(scope))
		return nil
	}
	manager.waitTimeout = time.Second
	manager.start(ctx)
	defer func() { _ = manager.close() }()
	runtime.settlements.bind(manager)
	app := NewApp()
	app.cloudRuntime = runtime
	app.billingWorkerRuntime = manager
	app.setCurrentActorContext(actor)
	app.recoverDesktopStartupState(ctx)
	scope := receiveDesktopBillingScope(t, "startup subscription worker", started)
	if len(recoveryScopes) != 2 || recoveryScopes[0].CloudBinding != nil || recoveryScopes[1].CloudBinding == nil || !recoveryScopes[1].Equal(scope) {
		t.Fatalf("startup recovery scopes = %+v, worker=%+v", recoveryScopes, scope)
	}
	if len(recoveryDBs) != 2 || recoveryDBs[0] != recoveryDBs[1] || workerDB != recoveryDBs[0] {
		t.Fatalf("startup DB reuse = recoveries:%v workerSame:%v", recoveryDBs[0] == recoveryDBs[1], workerDB == recoveryDBs[0])
	}
	if manager.active == nil || !manager.authorized {
		t.Fatalf("startup ownership = active:%v authorized:%v", manager.active != nil, manager.authorized)
	}
}

func TestDesktopFixtureQuotaSupplementerReturnsOneCompleteNextRevision(t *testing.T) {
	actor := userctx.ActorContext{ActorID: "supplement-actor", ProfileID: "supplement-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "supplement-cloud", BindingGeneration: 2}
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	endpoint := modelbilling.MaasEndpointChatCompletions
	current, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint: endpoint, Headers: map[string][]string{"X-Oneapi-Request-Id": {"request-supplement"}},
		FinalJSON: json.RawMessage(`{}`), ObservedAt: desktopFixtureTimestamp(0, 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := (desktopFixtureQuotaSupplementer{scope: scope}).Supplement(context.Background(), modelbillingworker.QuotaSupplementRequest{
		Scope: scope, TaskID: "task-supplement", CallID: "call-supplement", Endpoint: endpoint, CurrentEvidence: current,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != current.Revision+1 || got.Status != modelbilling.QuotaEvidenceComplete || got.Quota == nil || *got.Quota != "0" || got.OneAPIRequestID == nil || *got.OneAPIRequestID != "request-supplement" || !got.ObservedAt.After(current.ObservedAt) {
		t.Fatalf("supplement revision = %#v", got)
	}
	wrong := scope
	wrongBinding := binding
	wrongBinding.BindingGeneration++
	wrong.CloudBinding = &wrongBinding
	if _, err := (desktopFixtureQuotaSupplementer{scope: scope}).Supplement(context.Background(), modelbillingworker.QuotaSupplementRequest{
		Scope: wrong, TaskID: "task-supplement", CallID: "call-supplement", Endpoint: endpoint, CurrentEvidence: current,
	}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorScopeMismatch {
		t.Fatalf("wrong supplement scope = %v", err)
	}

	missingRequestID, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint: endpoint, FinalJSON: json.RawMessage(`{}`), ObservedAt: desktopFixtureTimestamp(0, 20),
	})
	if err != nil {
		t.Fatal(err)
	}
	requestIDs := make([]string, 0, 2)
	for _, taskID := range []string{"task-shared-call-a", "task-shared-call-b"} {
		revision, err := (desktopFixtureQuotaSupplementer{scope: scope}).Supplement(context.Background(), modelbillingworker.QuotaSupplementRequest{
			Scope: scope, TaskID: taskID, CallID: "shared-call", Endpoint: endpoint, CurrentEvidence: missingRequestID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if revision.OneAPIRequestID == nil {
			t.Fatalf("task %q supplement did not synthesize a request ID", taskID)
		}
		requestIDs = append(requestIDs, *revision.OneAPIRequestID)
	}
	if requestIDs[0] == requestIDs[1] {
		t.Fatalf("different tasks sharing one call ID synthesized the same request ID %q", requestIDs[0])
	}
}

func TestDesktopAcceptedQuotaRequestRegistryIsCloudUserWideAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "accepted-requests.sqlite")
	first := desktopAcceptedQuotaRequestRegistry{path: path}
	if err := first.AcceptQuotaRequestIDs(ctx, "cloud-user", "task-first", []string{"request-shared"}); err != nil {
		t.Fatal(err)
	}

	restarted := desktopAcceptedQuotaRequestRegistry{path: path}
	if err := restarted.AcceptQuotaRequestIDs(ctx, "cloud-user", "task-first", []string{"request-shared"}); err != nil {
		t.Fatalf("same task restart acceptance = %v", err)
	}
	if err := restarted.AcceptQuotaRequestIDs(ctx, "cloud-user", "task-second", []string{"request-shared"}); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorQuotaRequestIDReused {
		t.Fatalf("same cloud user reuse = %v", err)
	}
	if err := restarted.AcceptQuotaRequestIDs(ctx, "other-cloud-user", "task-second", []string{"request-shared"}); err != nil {
		t.Fatalf("different cloud user acceptance = %v", err)
	}
}

func TestDefaultDesktopBillingWorkerSupplementConvergesToSettledZero(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B4_SUPPLEMENT_KEY")
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_fixture", BindingGeneration: 1}
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}

	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.start(context.Background())
	managerClosed := false
	defer func() {
		if !managerClosed {
			_ = manager.close()
		}
	}()
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.billingWorkerRuntime = manager
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	bundle, err := modelbilling.LoadEmbeddedGatewayFixture()
	if err != nil {
		t.Fatal(err)
	}
	bundle.Scope = binding
	found := false
	for caseIndex := range bundle.Cases {
		if bundle.Cases[caseIndex].Name != "desktop_stream_zero" {
			continue
		}
		for chunkIndex := range bundle.Cases[caseIndex].Chunks {
			observation := bundle.Cases[caseIndex].Chunks[chunkIndex].QuotaObservation
			if observation != nil {
				observation.FinalJSON = json.RawMessage(`{}`)
				found = true
			}
		}
	}
	if !found {
		t.Fatal("desktop gateway fixture has no quota observation")
	}
	authorized, ok := runtime.dispatcher.Gateway.(desktopAuthorizedSubscriptionGateway)
	if !ok {
		t.Fatalf("subscription gateway = %T", runtime.dispatcher.Gateway)
	}
	authorized.materialize = func(binding cloud.BindingScope) (desktopSubscriptionGateway, error) {
		materialized := bundle
		materialized.Scope = binding
		return modelbilling.NewMaterializedFixtureGateway(materialized, modelbilling.GatewayFixtureSelection{
			FixtureVersion: materialized.Version, CaseName: "desktop_stream_zero",
		})
	}
	runtime.dispatcher.Gateway = authorized

	selection := modelbilling.BillingRouteSelection{ModelRef: modelRef, BillingSource: modelbilling.BillingSubscriptionPool}
	taskContext, err := runtime.initializer.InitializeTaskContext(context.Background(), controlTaskContextRequest(actor, selection))
	if err != nil {
		t.Fatal(err)
	}
	taskCtx, err := modelbilling.WithTaskContext(context.Background(), taskContext)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := modelbilling.TaskLifecycleRunner{
		Tasks: runtime.tasks,
		Now: newDesktopFixturePairClock(
			desktopFixtureTimestamp(0, 0),
			desktopFixtureTimestamp(0, 12),
		),
	}
	if err := lifecycle.RunTask(taskCtx, func(ctx context.Context) error {
		chunks, err := runtime.dispatcher.Stream(ctx, &desktopForbiddenBYOKProvider{}, provider.Request{
			Messages: []provider.Message{{Role: provider.RoleUser, Content: "supplement missing quota"}},
		})
		if err != nil {
			return err
		}
		for range chunks {
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	calls, err := runtime.calls.ListByTask(context.Background(), scope, taskContext.TaskID)
	if err != nil || len(calls) != 1 || calls[0].QuotaEvidence == nil || calls[0].QuotaEvidence.Status != modelbilling.QuotaEvidenceMissing {
		t.Fatalf("initial missing evidence calls = %#v, err=%v", calls, err)
	}
	if err := manager.close(); err != nil {
		t.Fatal(err)
	}
	managerClosed = true

	workerRuntime, err := defaultDesktopBillingWorkerFactory(runtime.db, scope)
	if err != nil {
		t.Fatal(err)
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan error, 1)
	go func() { workerDone <- workerRuntime.run(workerCtx) }()
	completed := waitDesktopBillingTaskState(t, runtime.tasks, scope, taskContext.TaskID, modelbilling.TaskCompleted)
	if completed.Result == nil || *completed.Result != modelbilling.TaskResultCompleted {
		t.Fatalf("completed task = %#v", completed)
	}
	cancel()
	if err := <-workerDone; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("worker stop = %v", err)
	}

	history, err := modelbilling.NewSQLiteQuotaEvidenceStore(runtime.db).History(context.Background(), scope, taskContext.TaskID, calls[0].CallID)
	if err != nil || len(history) != 2 || history[0].Status != modelbilling.QuotaEvidenceMissing || history[1].Status != modelbilling.QuotaEvidenceComplete || history[1].Quota == nil || *history[1].Quota != "0" {
		t.Fatalf("supplement evidence history = %#v, err=%v", history, err)
	}
	reports, err := modelbilling.NewSQLiteReportStore(runtime.db).ListByTask(context.Background(), scope, taskContext.TaskID)
	if err != nil || len(reports) != 1 || reports[0].Kind != modelbilling.ReportFinal || reports[0].State != modelbilling.ReportAccepted {
		t.Fatalf("supplement final reports = %#v, err=%v", reports, err)
	}
	settlement, settled, err := modelbilling.NewSQLiteQuotaSettlementStore(runtime.db).CurrentQuotaSettlement(context.Background(), scope, taskContext.TaskID)
	if err != nil || !settled || settlement.Settlement.Status != modelbilling.QuotaSettlementSettled || settlement.Settlement.TaskTotalQuota == nil || *settlement.Settlement.TaskTotalQuota != "0" {
		t.Fatalf("supplement settlement = %#v, settled=%v, err=%v", settlement, settled, err)
	}
}
