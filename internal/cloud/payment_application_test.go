package cloud

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestPaymentApplicationCreatesScopedIdempotentPurchaseAndUpgrade(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := paymentApplicationScope(t, actor, "usr_01", 1)
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "purchase_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)}
	store := NewMemoryPaymentOrderRepository()
	app := newPaymentApplication(t, bindings, client, store, nil)

	first, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
	if err != nil || second != first || client.createSubscriptionCalls != 1 {
		t.Fatalf("idempotent purchase = %#v, %v, calls=%d", second, err, client.createSubscriptionCalls)
	}
	attempt, found, err := store.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found {
		t.Fatalf("saved purchase attempt = %#v, %v, %v", attempt, found, err)
	}
	key, err := BuildIdempotencyKey(IdempotencyActionCreateSubscriptionOrder, scope, "plus\x00alipay\x00"+attempt.AttemptID)
	if err != nil || client.createSubscriptionRequest.IdempotencyKey != key || client.createSubscriptionRequest.Scope != scope {
		t.Fatalf("purchase request = %#v, key=%q, err=%v", client.createSubscriptionRequest, key, err)
	}

	client.createUpgradeResult = paymentApplicationResult(scope, "upgrade_01", PaymentOrderSubscriptionUpgrade, PaymentChannelWeChat, PaymentPending)
	client.createUpgradeResult.Order.Plan = "pro"
	upgrade, err := app.CreateUpgradeOrder(ctx, actor, "pro", PaymentChannelWeChat)
	if err != nil || upgrade.State != PaymentUIStatePaying || client.createUpgradeCalls != 1 {
		t.Fatalf("upgrade = %#v, %v, calls=%d", upgrade, err, client.createUpgradeCalls)
	}
	upgradeAttempt, found, err := store.LatestAttempt(ctx, scope, PaymentOrderSubscriptionUpgrade, "pro", PaymentChannelWeChat)
	if err != nil || !found {
		t.Fatalf("saved upgrade attempt = %#v, %v, %v", upgradeAttempt, found, err)
	}
	upgradeKey, err := BuildIdempotencyKey(IdempotencyActionCreateUpgradeOrder, scope, "pro\x00wechat\x00"+upgradeAttempt.AttemptID)
	if err != nil || client.createUpgradeRequest.IdempotencyKey != upgradeKey || client.createUpgradeRequest.Scope != scope {
		t.Fatalf("upgrade request = %#v, key=%q, err=%v", client.createUpgradeRequest, upgradeKey, err)
	}
}

func TestPaymentApplicationKeepsEntrylessCreatingAttempt(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	creating := paymentApplicationResult(scope, "order_creating", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentCreated)
	creating.Order.PlanID = "plan-plus-v1"
	creating.Entry = PaymentEntry{}
	client := &paymentApplicationClient{createSubscriptionResult: creating, getOrder: creating.Order}
	app := newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), nil)

	first, err := app.CreateSubscriptionOrder(ctx, actor, "plan-plus-v1", PaymentChannelAlipay)
	if err != nil || first.Result.Entry != (PaymentEntry{}) || first.State != PaymentUIStateCreated {
		t.Fatalf("entryless creating result = %#v, %v", first, err)
	}
	second, err := app.CreateSubscriptionOrder(ctx, actor, "plan-plus-v1", PaymentChannelAlipay)
	if err != nil || second.Result.Order.OrderID != first.Result.Order.OrderID ||
		client.createSubscriptionCalls != 1 || client.getCalls != 1 {
		t.Fatalf("reused creating attempt = %#v, %v, creates=%d, gets=%d", second, err, client.createSubscriptionCalls, client.getCalls)
	}
}

func TestPaymentApplicationRestoresEntrylessCreatingAsPayableOrder(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	creating := paymentApplicationResult(scope, "order_creating", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentCreated)
	creating.Order.PlanID = "plan-plus-v1"
	creating.Order.Version = 1
	creating.Entry = PaymentEntry{}
	pending := paymentApplicationResult(scope, "order_creating", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
	pending.Order.PlanID = "plan-plus-v1"
	pending.Order.Version = 2
	base := &paymentApplicationClient{createSubscriptionResult: creating}
	client := &paymentApplicationFullResultClient{paymentApplicationClient: base, getResult: pending}
	provider := testProfilePathProvider(t.TempDir())
	store := NewDurablePaymentOrderRepository(provider)
	first := newPaymentApplication(t, bindings, client, store, nil)
	if _, err := first.CreateSubscriptionOrder(ctx, actor, "plan-plus-v1", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}

	restartedStore := NewDurablePaymentOrderRepository(provider)
	restarted := newPaymentApplication(t, bindings, client, restartedStore, nil)
	view, err := restarted.CreateSubscriptionOrder(ctx, actor, "plan-plus-v1", PaymentChannelAlipay)
	if err != nil || !view.Payable || view.Result.Entry.QRCode == "" ||
		client.createSubscriptionCalls != 1 || client.resultCalls != 1 {
		t.Fatalf("restored payable order = %#v, %v, creates=%d, gets=%d", view, err, client.createSubscriptionCalls, client.resultCalls)
	}
	record, found, err := restartedStore.Get(ctx, scope, "order_creating")
	if err != nil || !found || record.Result.Entry.QRCode == "" || record.Result.Order.Version != 2 {
		t.Fatalf("persisted payable order = %#v, found=%v, err=%v", record, found, err)
	}
}

func TestPaymentApplicationPersistsEntrylessCreateFailedResult(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	failed := paymentApplicationResult(scope, "order_create_failed", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentFailed)
	failed.Order.PlanID = "plan-plus-v1"
	failed.Entry = PaymentEntry{}
	store := NewMemoryPaymentOrderRepository()
	client := &paymentApplicationClient{createSubscriptionResult: failed}
	app := newPaymentApplication(t, bindings, client, store, nil)

	view, err := app.CreateSubscriptionOrder(ctx, actor, "plan-plus-v1", PaymentChannelAlipay)
	if err != nil || view.Result.Entry != (PaymentEntry{}) || view.State != PaymentUIStateFailed {
		t.Fatalf("entryless create_failed result = %#v, %v", view, err)
	}
	attempt, found, err := store.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plan-plus-v1", PaymentChannelAlipay)
	if err != nil || !found || attempt.OrderID != "order_create_failed" {
		t.Fatalf("saved create_failed attempt = %#v, found=%v, err=%v", attempt, found, err)
	}
	record, found, err := store.Get(ctx, scope, attempt.OrderID)
	if err != nil || !found || record.Result.Entry != (PaymentEntry{}) || record.Result.Order.Status != PaymentFailed {
		t.Fatalf("saved create_failed order = %#v, found=%v, err=%v", record, found, err)
	}
}

func TestPaymentApplicationRestoresUnboundAttemptAndReusesKeyAfterRestart(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	provider := testProfilePathProvider(t.TempDir())
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	failedClient := &paymentApplicationClient{
		createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending),
		createSubscriptionErr:    networkErr,
	}
	first := newPaymentApplication(t, bindings, failedClient, NewDurablePaymentOrderRepository(provider), nil)
	if _, err := first.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); !errors.Is(err, networkErr) {
		t.Fatalf("first create error = %v", err)
	}
	saved, found, err := NewDurablePaymentOrderRepository(provider).LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found || saved.OrderID != "" {
		t.Fatalf("unbound attempt = %#v, %v, %v", saved, found, err)
	}

	retryClient := &paymentApplicationClient{
		createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending),
	}
	restarted := newPaymentApplication(t, bindings, retryClient, NewDurablePaymentOrderRepository(provider), nil)
	view, err := restarted.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
	if err != nil || view.Result.Order.OrderID != "order_01" || retryClient.createSubscriptionRequest.IdempotencyKey != saved.IdempotencyKey {
		t.Fatalf("restarted create = %#v, %v, request=%#v", view, err, retryClient.createSubscriptionRequest)
	}
	bound, found, err := NewDurablePaymentOrderRepository(provider).LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found || bound.AttemptID != saved.AttemptID || bound.OrderID != "order_01" {
		t.Fatalf("bound attempt = %#v, %v, %v", bound, found, err)
	}
}

func TestPaymentApplicationStartsNewAttemptOnlyAfterOrderCannotBePaid(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	store := NewMemoryPaymentOrderRepository()
	client := &paymentApplicationClient{
		createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending),
	}
	app := newPaymentApplication(t, bindings, client, store, nil)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	firstAttempt, found, err := store.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found {
		t.Fatalf("first attempt = %#v, %v, %v", firstAttempt, found, err)
	}

	client.getOrder = paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentTimeout).Order
	if _, err := app.RefreshOrder(ctx, actor, "order_01"); err != nil {
		t.Fatal(err)
	}
	client.createSubscriptionResult = paymentApplicationResult(scope, "order_02", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
	view, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
	if err != nil || view.Result.Order.OrderID != "order_02" || client.createSubscriptionCalls != 2 {
		t.Fatalf("new terminal attempt = %#v, %v, calls=%d", view, err, client.createSubscriptionCalls)
	}
	secondAttempt, found, err := store.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found || secondAttempt.AttemptID == firstAttempt.AttemptID || secondAttempt.IdempotencyKey == firstAttempt.IdempotencyKey {
		t.Fatalf("second attempt = %#v, first=%#v, found=%v, err=%v", secondAttempt, firstAttempt, found, err)
	}
}

func TestPaymentAttemptCanRestartOnlyForTerminalFailureOrExpiredEntry(t *testing.T) {
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	now := time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC)
	for _, item := range []struct {
		name   string
		status PaymentStatus
		expiry time.Time
		want   bool
	}{
		{name: "pending payable", status: PaymentPending, expiry: now.Add(time.Minute), want: false},
		{name: "pending expired", status: PaymentPending, expiry: now, want: true},
		{name: "closed", status: PaymentClosed, expiry: now.Add(time.Minute), want: true},
		{name: "timeout", status: PaymentTimeout, expiry: now.Add(time.Minute), want: true},
		{name: "failed", status: PaymentFailed, expiry: now.Add(time.Minute), want: true},
		{name: "verifying expired", status: PaymentVerifying, expiry: now, want: false},
		{name: "granting expired", status: PaymentGranting, expiry: now, want: false},
		{name: "granted", status: PaymentGranted, expiry: now, want: false},
		{name: "manual review", status: PaymentVerifyFailedManualRequired, expiry: now, want: false},
	} {
		t.Run(item.name, func(t *testing.T) {
			record := paymentOrderRepositoryRecord(t, scope, "order_01", "idem_01", item.status)
			record.Result.Entry.ExpiresAt = item.expiry
			if got := paymentAttemptCanRestart(record, now); got != item.want {
				t.Fatalf("paymentAttemptCanRestart(%s) = %v, want %v", item.status, got, item.want)
			}
		})
	}
}

func TestPaymentApplicationRefreshesExpiredPendingBeforeStartingAnotherOrder(t *testing.T) {
	now := time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC)
	for _, item := range []struct {
		status      PaymentStatus
		wantOrderID string
		wantCreates int
	}{
		{status: PaymentPending, wantOrderID: "order_02", wantCreates: 2},
		{status: PaymentTimeout, wantOrderID: "order_02", wantCreates: 2},
		{status: PaymentVerifying, wantOrderID: "order_01", wantCreates: 1},
		{status: PaymentGranted, wantOrderID: "order_01", wantCreates: 1},
	} {
		t.Run(string(item.status), func(t *testing.T) {
			ctx := context.Background()
			actor, bindings, scope := paymentApplicationBinding(t)
			first := paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
			first.Order.ExpiresAt = now
			first.Entry.ExpiresAt = now
			client := &paymentApplicationClient{createSubscriptionResult: first}
			app := newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), nil)
			if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
				t.Fatal(err)
			}
			remote := first.Order
			remote.Status = item.status
			if item.status == PaymentGranted {
				remote.GrantStatus = GrantGranted
			}
			client.getOrder = remote
			client.createSubscriptionResult = paymentApplicationResult(scope, "order_02", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)

			view, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
			if err != nil || view.Result.Order.OrderID != item.wantOrderID || client.createSubscriptionCalls != item.wantCreates || client.getCalls != 1 {
				t.Fatalf("expired retry = %#v, %v, creates=%d, gets=%d", view, err, client.createSubscriptionCalls, client.getCalls)
			}
		})
	}
}

func TestPaymentApplicationDoesNotRestartExpiredOrderWhenRefreshFails(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	now := time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC)
	first := paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
	first.Order.ExpiresAt = now
	first.Entry.ExpiresAt = now
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	client := &paymentApplicationClient{createSubscriptionResult: first}
	app := newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), nil)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	client.getErr = networkErr
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); !errors.Is(err, networkErr) {
		t.Fatalf("refresh error = %v", err)
	}
	if client.createSubscriptionCalls != 1 || client.getCalls != 1 {
		t.Fatalf("calls after refresh failure: create=%d get=%d", client.createSubscriptionCalls, client.getCalls)
	}
}

func TestPaymentApplicationBindsLocallySavedOrderWithoutRepeatingCreate(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	provider := testProfilePathProvider(t.TempDir())
	store := NewDurablePaymentOrderRepository(provider)
	client := &paymentApplicationClient{}
	app := newPaymentApplication(t, bindings, client, store, nil)
	candidate, err := app.newPaymentAttempt(scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil {
		t.Fatal(err)
	}
	attempt, created, err := store.ReserveAttempt(ctx, "", candidate)
	if err != nil || !created {
		t.Fatalf("reserve attempt = %#v, %v, %v", attempt, created, err)
	}
	record, err := NewPaymentOrderRecord(
		scope,
		paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending),
		PaymentOrderSubscriptionPurchase,
		attempt.IdempotencyKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}

	restarted := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), nil)
	view, err := restarted.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
	if err != nil || view.Result.Order.OrderID != "order_01" || client.createSubscriptionCalls != 0 {
		t.Fatalf("recovered create = %#v, %v, calls=%d", view, err, client.createSubscriptionCalls)
	}
	bound, found, err := store.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found || bound.OrderID != "order_01" {
		t.Fatalf("bound attempt = %#v, %v, %v", bound, found, err)
	}
}

func TestPaymentApplicationMigratesLegacyOrderWithoutDuplicatingPayableOrder(t *testing.T) {
	for _, item := range []struct {
		name        string
		status      PaymentStatus
		wantCreates int
		wantOrderID string
		wantRecords int
		wantVersion int
	}{
		{name: "payable", status: PaymentPending, wantOrderID: "legacy_01", wantRecords: 1, wantVersion: 1},
		{name: "terminal", status: PaymentTimeout, wantCreates: 1, wantOrderID: "order_02", wantRecords: 2, wantVersion: 2},
	} {
		t.Run(item.name, func(t *testing.T) {
			ctx := context.Background()
			actor, bindings, scope := paymentApplicationBinding(t)
			provider := testProfilePathProvider(t.TempDir())
			repository := NewDurablePaymentOrderRepository(provider)
			legacyKey, err := legacyPaymentOrderIdempotencyKey(PaymentOrderSubscriptionPurchase, scope, "plus", PaymentChannelAlipay)
			if err != nil {
				t.Fatal(err)
			}
			record := paymentOrderRepositoryRecord(t, scope, "legacy_01", legacyKey, item.status)
			path, err := repository.path(scope.ProfileID)
			if err != nil {
				t.Fatal(err)
			}
			document := durablePaymentOrdersDocument{
				Schema: durablePaymentOrdersSchema, Version: 1, ProfileID: scope.ProfileID,
				Entries: []durablePaymentOrderEntry{{
					Scope: scope, OrderID: record.Result.Order.OrderID,
					IdempotencyKey: record.IdempotencyKey, Record: record,
				}},
			}
			if err := durableWriteJSON(path, document); err != nil {
				t.Fatal(err)
			}
			client := &paymentApplicationClient{
				createSubscriptionResult: paymentApplicationResult(scope, "order_02", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending),
			}
			app := newPaymentApplication(t, bindings, client, repository, nil)
			view, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
			if err != nil || view.Result.Order.OrderID != item.wantOrderID || client.createSubscriptionCalls != item.wantCreates {
				t.Fatalf("legacy create = %#v, %v, calls=%d", view, err, client.createSubscriptionCalls)
			}
			records, err := repository.List(ctx, scope)
			if err != nil || len(records) != item.wantRecords {
				t.Fatalf("legacy records = %#v, %v", records, err)
			}
			var saved durablePaymentOrdersDocument
			if found, err := durableReadJSON(path, &saved); err != nil || !found || saved.Version != item.wantVersion {
				t.Fatalf("legacy document = %#v, %v, %v", saved, found, err)
			}
		})
	}
}

func TestPaymentApplicationConcurrentInstancesReserveOneAttempt(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	provider := testProfilePathProvider(t.TempDir())
	client := &concurrentPaymentApplicationClient{
		result: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending),
		keys:   make(map[string]int),
	}
	apps := make([]*PaymentApplication, 2)
	for index := range apps {
		apps[index] = newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), nil)
	}
	start := make(chan struct{})
	type outcome struct {
		view PaymentView
		err  error
	}
	outcomes := make(chan outcome, len(apps))
	var workers sync.WaitGroup
	for _, app := range apps {
		workers.Add(1)
		go func(app *PaymentApplication) {
			defer workers.Done()
			<-start
			view, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
			outcomes <- outcome{view: view, err: err}
		}(app)
	}
	close(start)
	workers.Wait()
	close(outcomes)
	for result := range outcomes {
		if result.err != nil || result.view.Result.Order.OrderID != "order_01" {
			t.Fatalf("concurrent create = %#v, %v", result.view, result.err)
		}
	}
	client.mu.Lock()
	keyCount := len(client.keys)
	client.mu.Unlock()
	if keyCount != 1 {
		t.Fatalf("cloud received %d distinct idempotency keys", keyCount)
	}
	repository := NewDurablePaymentOrderRepository(provider)
	attempt, found, err := repository.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
	if err != nil || !found || attempt.Sequence != 1 || attempt.OrderID != "order_01" {
		t.Fatalf("reserved attempt = %#v, %v, %v", attempt, found, err)
	}
}

func TestPaymentApplicationIsolatesScopeAndRejectsMismatchedOrders(t *testing.T) {
	ctx := context.Background()
	first := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	second := userctx.ActorContext{ActorID: "local", ProfileID: "secondary"}
	bindings := NewBindingRepository()
	for _, item := range []struct {
		actor userctx.ActorContext
		user  string
	}{{first, "usr_01"}, {second, "usr_02"}} {
		if _, err := bindings.ConfirmBinding(ctx, item.actor, item.user); err != nil {
			t.Fatal(err)
		}
	}
	firstScope := paymentApplicationScope(t, first, "usr_01", 1)
	secondScope := paymentApplicationScope(t, second, "usr_02", 1)
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(firstScope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)}
	store := NewMemoryPaymentOrderRepository()
	app := newPaymentApplication(t, bindings, client, store, nil)
	if _, err := app.CreateSubscriptionOrder(ctx, first, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RefreshOrder(ctx, second, "order_01"); !isProfileMismatch(err) || client.getCalls != 0 {
		t.Fatalf("cross scope refresh error=%v calls=%d", err, client.getCalls)
	}
	if _, err := bindings.Rebind(ctx, first, "usr_03"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RefreshOrder(ctx, first, "order_01"); !isProfileMismatch(err) || client.getCalls != 0 {
		t.Fatalf("old generation refresh error=%v calls=%d", err, client.getCalls)
	}
	if _, ok, err := store.Get(ctx, secondScope, "order_01"); err != nil || ok {
		t.Fatalf("cross scope store = %v, %v", ok, err)
	}
}

func TestPaymentApplicationDoesNotExposeExpiredEntryOrRefreshIt(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)}
	now := time.Date(2026, time.July, 11, 1, 0, 0, 0, time.UTC)
	client.createSubscriptionResult.Order.ExpiresAt = now
	client.createSubscriptionResult.Entry.ExpiresAt = now
	client.createSubscriptionResult.Entry.QRCode = ""
	client.createSubscriptionResult.Entry.QRCodeURL = "https://pay.example.invalid/order_01"
	app := newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), nil)
	app.now = func() time.Time { return now }

	view, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay)
	if err != nil || view.Payable || view.Result.Entry.QRCode != "" || view.Result.Entry.CodeURL != "" || view.Result.Entry.QRCodeURL != "" {
		t.Fatalf("expired entry = %#v, %v", view, err)
	}
	if _, err := app.RefreshPaymentEntry(ctx, actor, "order_01"); err == nil || client.getCalls != 0 {
		t.Fatalf("refresh old entry error=%v calls=%d", err, client.getCalls)
	}
}

func TestPaymentApplicationRefreshMapsAuthoritativeStatusesAndCallbackOnce(t *testing.T) {
	cases := []struct {
		status PaymentStatus
		want   PaymentUIState
	}{
		{PaymentCreated, PaymentUIStateCreated},
		{PaymentPending, PaymentUIStatePaying},
		{PaymentCallbackReceived, PaymentUIStateConfirming},
		{PaymentVerifying, PaymentUIStateConfirming},
		{PaymentPaidVerified, PaymentUIStateConfirming},
		{PaymentGranting, PaymentUIStateConfirming},
		{PaymentClosed, PaymentUIStateClosed},
		{PaymentTimeout, PaymentUIStateTimeout},
		{PaymentFailed, PaymentUIStateFailed},
		{PaymentVerifyFailedManualRequired, PaymentUIStateManualRequired},
		{PaymentGranted, PaymentUIStateSucceeded},
	}
	for _, item := range cases {
		t.Run(string(item.status), func(t *testing.T) {
			ctx := context.Background()
			actor, bindings, scope := paymentApplicationBinding(t)
			created := paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentCreated)
			client := &paymentApplicationClient{createSubscriptionResult: created}
			refresher := &paymentApplicationRefresher{}
			app := newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), refresher)
			if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
				t.Fatal(err)
			}
			client.getOrder = paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, item.status).Order
			view, err := app.RefreshOrder(ctx, actor, "order_01")
			if err != nil || view.State != item.want || view.EntitlementGranted != (item.status == PaymentGranted) {
				t.Fatalf("refresh %s = %#v, %v", item.status, view, err)
			}
			wantRefreshes := 0
			if item.status == PaymentGranted {
				wantRefreshes = 1
				if _, err := app.RefreshOrder(ctx, actor, "order_01"); err != nil {
					t.Fatalf("repeated granted: %v", err)
				}
			}
			if refresher.calls != wantRefreshes {
				t.Fatalf("entitlement refresh calls = %d, want %d", refresher.calls, wantRefreshes)
			}
		})
	}
}

func TestPaymentApplicationRefreshKeepsLocalViewOnNetworkFailureAndClientPaidIsTransient(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)}
	store := NewMemoryPaymentOrderRepository()
	app := newPaymentApplication(t, bindings, client, store, nil)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	client.getErr = networkErr
	view, err := app.RefreshOrder(ctx, actor, "order_01")
	if !errors.Is(err, networkErr) || view.State != PaymentUIStatePaying {
		t.Fatalf("network view = %#v, %v", view, err)
	}
	client.getErr = nil
	client.verifyOrder = paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentGranted).Order
	app.now = func() time.Time { return time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC) }
	view, err = app.ClientPaidUnverified(ctx, actor, "order_01")
	if err != nil || view.State != PaymentUIStateConfirming || view.EntitlementGranted || client.verifyCalls != 1 {
		t.Fatalf("client-paid view = %#v, %v, calls=%d", view, err, client.verifyCalls)
	}
	record, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok || record.Result.Order.Status != PaymentPending {
		t.Fatalf("transient was persisted = %#v, %v, %v", record, ok, err)
	}
}

func TestPaymentApplicationClientPaidRequiresPayablePendingOrder(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC)
	tests := []struct {
		name       string
		status     PaymentStatus
		expired    bool
		wantCode   ErrorCode
		wantVerify int
	}{
		{"payable pending", PaymentPending, false, "", 1},
		{"expired pending", PaymentPending, true, ErrorPaymentNotVerified, 0},
		{"verifying", PaymentVerifying, false, ErrorPaymentNotVerified, 0},
		{"failed", PaymentFailed, false, ErrorPaymentNotVerified, 0},
		{"granted", PaymentGranted, false, ErrorOrderAlreadyGranted, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor, bindings, scope := paymentApplicationBinding(t)
			record := paymentOrderRepositoryRecord(t, scope, "order_confirm", "idem_confirm", tt.status)
			if tt.expired {
				record.Result.Order.ExpiresAt = now
				record.Result.Entry.ExpiresAt = now
			}
			store := NewMemoryPaymentOrderRepository()
			if err := store.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
			record, found, err := store.Get(ctx, scope, record.Result.Order.OrderID)
			if err != nil || !found {
				t.Fatalf("saved confirm record = %#v, %v, %v", record, found, err)
			}
			client := &paymentApplicationClient{verifyOrder: record.Result.Order}
			app := newPaymentApplication(t, bindings, client, store, nil)
			app.now = func() time.Time { return now }
			view, err := app.ClientPaidUnverified(ctx, actor, record.Result.Order.OrderID)
			if tt.wantCode == "" {
				if err != nil || view.State != PaymentUIStateConfirming || view.Payable || view.EntitlementGranted {
					t.Fatalf("ClientPaidUnverified = %#v, %v", view, err)
				}
			} else {
				var cloudErr *Error
				if !errors.As(err, &cloudErr) || cloudErr.Code != tt.wantCode {
					t.Fatalf("ClientPaidUnverified error = %v, want %s", err, tt.wantCode)
				}
			}
			if client.verifyCalls != tt.wantVerify {
				t.Fatalf("verify calls = %d, want %d", client.verifyCalls, tt.wantVerify)
			}
			saved, found, err := store.Get(ctx, scope, record.Result.Order.OrderID)
			if err != nil || !found || saved != record {
				t.Fatalf("confirm persisted transient/change = %#v, %v, %v", saved, found, err)
			}
		})
	}
}

func TestPaymentApplicationRejectsMismatchedAuthoritativeRefresh(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)}
	store := NewMemoryPaymentOrderRepository()
	app := newPaymentApplication(t, bindings, client, store, nil)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	client.getOrder = paymentApplicationResult(paymentApplicationScope(t, actor, "usr_other", 1), "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentGranted).Order
	if _, err := app.RefreshOrder(ctx, actor, "order_01"); !isProfileMismatch(err) {
		t.Fatalf("mismatched refresh error = %v", err)
	}
	record, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok || record.Result.Order.Status != PaymentPending {
		t.Fatalf("mismatched refresh overwrote local record = %#v, %v, %v", record, ok, err)
	}
}

func TestPaymentApplicationRejectsStatusRegressionBeforeRepositorySave(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	created := paymentApplicationResult(scope, "order_status", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
	client := &paymentApplicationClient{createSubscriptionResult: created}
	store := &paymentApplicationCountingStore{PaymentOrderRepository: NewMemoryPaymentOrderRepository()}
	app := newPaymentApplication(t, bindings, client, store, nil)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	if store.saveCalls != 1 {
		t.Fatalf("create save calls = %d, want 1", store.saveCalls)
	}
	client.getOrder = created.Order
	client.getOrder.Status = PaymentCreated
	if _, err := app.RefreshOrder(ctx, actor, created.Order.OrderID); err == nil {
		t.Fatal("RefreshOrder accepted an authoritative status regression")
	}
	if store.saveCalls != 1 {
		t.Fatalf("regressed refresh reached repository Save: calls=%d", store.saveCalls)
	}
	stored, found, err := store.Get(ctx, scope, created.Order.OrderID)
	if err != nil || !found || stored.Result.Order.Status != PaymentPending {
		t.Fatalf("stored record after rejected regression = %#v, %v, %v", stored, found, err)
	}
}

func TestPaymentApplicationRejectsReturnedTargetOrIdentityDrift(t *testing.T) {
	t.Run("create target plan", func(t *testing.T) {
		ctx := context.Background()
		actor, bindings, scope := paymentApplicationBinding(t)
		result := paymentApplicationResult(scope, "order_wrong_plan", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
		result.Order.Plan = "lite"
		store := NewMemoryPaymentOrderRepository()
		app := newPaymentApplication(t, bindings, &paymentApplicationClient{createSubscriptionResult: result}, store, nil)
		if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err == nil {
			t.Fatal("CreateSubscriptionOrder accepted a result for another target plan")
		}
		if records, err := store.List(ctx, scope); err != nil || len(records) != 0 {
			t.Fatalf("rejected create persisted records = %#v, %v", records, err)
		}
	})

	t.Run("refresh complete identity", func(t *testing.T) {
		ctx := context.Background()
		actor, bindings, scope := paymentApplicationBinding(t)
		created := paymentApplicationResult(scope, "order_identity", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
		client := &paymentApplicationClient{createSubscriptionResult: created}
		store := NewMemoryPaymentOrderRepository()
		app := newPaymentApplication(t, bindings, client, store, nil)
		if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
			t.Fatal(err)
		}
		client.getOrder = created.Order
		client.getOrder.Amount = "999.99"
		if _, err := app.RefreshOrder(ctx, actor, created.Order.OrderID); err == nil {
			t.Fatal("RefreshOrder accepted an amount identity drift")
		}
		record, found, err := store.Get(ctx, scope, created.Order.OrderID)
		if err != nil || !found || record.Result.Order != created.Order {
			t.Fatalf("identity drift overwrote saved authority = %#v, %v, %v", record, found, err)
		}
	})
}

func TestPaymentApplicationDoesNotTreatNewOrderExpiryAsReplacementEntry(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)}
	store := NewMemoryPaymentOrderRepository()
	refresher := &paymentApplicationRefresher{}
	app := newPaymentApplication(t, bindings, client, store, refresher)
	app.now = func() time.Time { return time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC) }
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	saved, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok {
		t.Fatalf("saved order = %#v, %v, %v", saved, ok, err)
	}
	client.getOrder = saved.Result.Order
	client.getOrder.Status = PaymentGranted
	client.getOrder.GrantStatus = GrantGranted
	client.getOrder.ExpiresAt = saved.Result.Order.ExpiresAt.Add(time.Hour)
	if view, err := app.RefreshOrder(ctx, actor, "order_01"); err == nil || view.Payable || view.Result.Entry.QRCode != "" {
		t.Fatalf("refreshed replacement entry = %#v, %v", view, err)
	}
	stored, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok || !stored.Result.Entry.ExpiresAt.Equal(saved.Result.Entry.ExpiresAt) || stored.Result.Order != saved.Result.Order || refresher.calls != 0 {
		t.Fatalf("refresh replaced saved entry = %#v, %v, %v", stored, ok, err)
	}
}

func TestPaymentApplicationRejectsInvalidCloudResultAndCallbackFailurePreservesAuthority(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	invalid := paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentPending)
	invalid.Order.Scope.CloudUserID = "usr_other"
	client := &paymentApplicationClient{createSubscriptionResult: invalid}
	app := newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), nil)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); !isProfileMismatch(err) {
		t.Fatalf("invalid scope error = %v", err)
	}

	client.createSubscriptionResult = paymentApplicationResult(scope, "order_02", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentGranted)
	client.createSubscriptionResult.Order.Plan = "pro"
	refresher := &paymentApplicationRefresher{err: errors.New("refresh failed")}
	app = newPaymentApplication(t, bindings, client, NewMemoryPaymentOrderRepository(), refresher)
	view, err := app.CreateSubscriptionOrder(ctx, actor, "pro", PaymentChannelAlipay)
	if !errors.Is(err, refresher.err) || !view.EntitlementGranted || view.State != PaymentUIStateSucceeded {
		t.Fatalf("callback failure = %#v, %v", view, err)
	}
}

func TestPaymentApplicationPersistsGrantedEntitlementRefreshAcrossRestart(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	store := NewMemoryPaymentOrderRepository()
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentGranted)}
	refresher := &paymentApplicationRefresher{}
	app := newPaymentApplication(t, bindings, client, store, refresher)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil || refresher.calls != 1 {
		t.Fatalf("initial granted order error=%v calls=%d", err, refresher.calls)
	}
	saved, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok || !saved.EntitlementsRefreshed {
		t.Fatalf("saved entitlement marker = %#v, %v, %v", saved, ok, err)
	}

	client.getOrder = saved.Result.Order
	restarted := newPaymentApplication(t, bindings, client, store, refresher)
	if _, err := restarted.RefreshOrder(ctx, actor, "order_01"); err != nil || refresher.calls != 1 {
		t.Fatalf("restart granted refresh error=%v calls=%d", err, refresher.calls)
	}
}

func TestPaymentApplicationRetriesGrantedEntitlementRefreshAfterFailure(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	store := NewMemoryPaymentOrderRepository()
	client := &paymentApplicationClient{createSubscriptionResult: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentGranted)}
	refresher := &paymentApplicationRefresher{err: errors.New("refresh failed")}
	app := newPaymentApplication(t, bindings, client, store, refresher)
	if _, err := app.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); !errors.Is(err, refresher.err) {
		t.Fatalf("initial refresh error = %v", err)
	}
	saved, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok || saved.EntitlementsRefreshed {
		t.Fatalf("failed refresh marker = %#v, %v, %v", saved, ok, err)
	}

	refresher.err = nil
	client.getOrder = saved.Result.Order
	if _, err := app.RefreshOrder(ctx, actor, "order_01"); err != nil || refresher.calls != 2 {
		t.Fatalf("retry refresh error=%v calls=%d", err, refresher.calls)
	}
	saved, ok, err = store.Get(ctx, scope, "order_01")
	if err != nil || !ok || !saved.EntitlementsRefreshed {
		t.Fatalf("successful retry marker = %#v, %v, %v", saved, ok, err)
	}
}

func paymentApplicationBinding(t *testing.T) (userctx.ActorContext, *BindingRepository, BindingScope) {
	t.Helper()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	return actor, bindings, paymentApplicationScope(t, actor, "usr_01", 1)
}

func paymentApplicationScope(t *testing.T, actor userctx.ActorContext, cloudUserID string, generation uint64) BindingScope {
	t.Helper()
	scope, err := NewBindingScope(actor, cloudUserID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func paymentApplicationResult(scope BindingScope, orderID string, operation PaymentOrderType, channel PaymentChannel, status PaymentStatus) PaymentOrderResult {
	now := time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC)
	grant := GrantNotGranted
	if status == PaymentGranted {
		grant = GrantGranted
	}
	result := PaymentOrderResult{Order: PaymentOrder{Scope: scope, OrderID: orderID, OrderType: operation, Plan: "plus", Amount: "29.90", Currency: "CNY", PaymentChannel: channel, Status: status, GrantStatus: grant, ExpiresAt: now.Add(time.Hour)}}
	if operation == PaymentOrderSubscriptionUpgrade {
		result.Order.FromPlan = "basic"
	}
	if channel == PaymentChannelAlipay {
		result.Order.AmountUnit = AmountUnitYuan
		result.Entry = PaymentEntry{Channel: channel, EntryType: PaymentEntryQRCode, QRCode: "qr_" + orderID, ExpiresAt: result.Order.ExpiresAt}
	} else {
		result.Order.AmountUnit = AmountUnitFen
		result.Entry = PaymentEntry{Channel: channel, EntryType: PaymentEntryNativeQR, CodeURL: "url_" + orderID, ExpiresAt: result.Order.ExpiresAt}
	}
	return result
}

func newPaymentApplication(t *testing.T, bindings BindingResolver, client CloudPaymentClient, store PaymentOrderRepository, refresher PaymentEntitlementRefresher) *PaymentApplication {
	t.Helper()
	app, err := NewPaymentApplication(PaymentApplicationDependencies{Bindings: bindings, Payments: client, Orders: store, Entitlements: refresher})
	if err != nil {
		t.Fatal(err)
	}
	app.now = func() time.Time {
		return time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC)
	}
	return app
}

type paymentApplicationClient struct {
	CloudPaymentClient
	createSubscriptionResult  PaymentOrderResult
	createUpgradeResult       PaymentOrderResult
	getOrder                  PaymentOrder
	verifyOrder               PaymentOrder
	createSubscriptionRequest CreateSubscriptionOrderRequest
	createUpgradeRequest      CreateUpgradeOrderRequest
	createSubscriptionCalls   int
	createUpgradeCalls        int
	getCalls                  int
	verifyCalls               int
	createSubscriptionErr     error
	getErr                    error
}

type paymentApplicationFullResultClient struct {
	*paymentApplicationClient
	getResult   PaymentOrderResult
	resultCalls int
}

func (c *paymentApplicationFullResultClient) getPaymentOrderResult(_ context.Context, _ BindingScope, _ string) (PaymentOrderResult, error) {
	c.resultCalls++
	return c.getResult, nil
}

func (c *paymentApplicationClient) CreateSubscriptionOrder(_ context.Context, request CreateSubscriptionOrderRequest) (PaymentOrderResult, error) {
	c.createSubscriptionCalls++
	c.createSubscriptionRequest = request
	return c.createSubscriptionResult, c.createSubscriptionErr
}

func (c *paymentApplicationClient) CreateUpgradeOrder(_ context.Context, request CreateUpgradeOrderRequest) (PaymentOrderResult, error) {
	c.createUpgradeCalls++
	c.createUpgradeRequest = request
	return c.createUpgradeResult, nil
}

func (c *paymentApplicationClient) GetPaymentOrder(_ context.Context, _ BindingScope, _ string) (PaymentOrder, error) {
	c.getCalls++
	if c.getErr != nil {
		return PaymentOrder{}, c.getErr
	}
	return c.getOrder, nil
}

func (c *paymentApplicationClient) VerifyPaymentOrder(_ context.Context, _ BindingScope, _ string) (PaymentOrder, error) {
	c.verifyCalls++
	return c.verifyOrder, nil
}

type concurrentPaymentApplicationClient struct {
	CloudPaymentClient
	mu     sync.Mutex
	result PaymentOrderResult
	keys   map[string]int
}

func (c *concurrentPaymentApplicationClient) CreateSubscriptionOrder(_ context.Context, request CreateSubscriptionOrderRequest) (PaymentOrderResult, error) {
	c.mu.Lock()
	c.keys[request.IdempotencyKey]++
	c.mu.Unlock()
	return c.result, nil
}

type paymentApplicationRefresher struct {
	calls int
	err   error
}

func (r *paymentApplicationRefresher) RefreshPaymentEntitlements(context.Context, userctx.ActorContext) error {
	r.calls++
	return r.err
}

type paymentApplicationCountingStore struct {
	PaymentOrderRepository
	saveCalls int
}

func (s *paymentApplicationCountingStore) Save(ctx context.Context, record PaymentOrderRecord) error {
	s.saveCalls++
	return s.PaymentOrderRepository.Save(ctx, record)
}
