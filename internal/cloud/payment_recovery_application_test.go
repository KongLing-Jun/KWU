package cloud

import (
	"context"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestPaymentRecoveryApplicationRestoresSavedOrdersWithoutRemoteRefresh(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	cases := []struct {
		status PaymentStatus
		want   PaymentUIState
	}{
		{PaymentPending, PaymentUIStatePaying},
		{PaymentPaidVerified, PaymentUIStateConfirming},
		{PaymentGranting, PaymentUIStateConfirming},
		{PaymentGranted, PaymentUIStateSucceeded},
		{PaymentClosed, PaymentUIStateClosed},
		{PaymentTimeout, PaymentUIStateTimeout},
		{PaymentFailed, PaymentUIStateFailed},
	}
	for _, item := range cases {
		t.Run(string(item.status), func(t *testing.T) {
			store := NewMemoryPaymentOrderRepository()
			record := paymentOrderRepositoryRecord(t, scope, "order_01", "idem_01", item.status)
			if err := store.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
			client := &paymentApplicationClient{getOrder: paymentApplicationResult(scope, "order_01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, item.status).Order}
			app := newPaymentApplication(t, bindings, client, store, nil)
			recovery, err := NewPaymentRecoveryApplication(app)
			if err != nil {
				t.Fatal(err)
			}

			results, err := recovery.Restore(ctx, actor)
			if err != nil || len(results) != 1 || results[0].OrderID != "order_01" || results[0].Err != nil || results[0].View.State != item.want || results[0].View.EntitlementGranted != (item.status == PaymentGranted) || client.getCalls != 0 {
				t.Fatalf("Restore = %#v, %v, get calls=%d", results, err, client.getCalls)
			}
		})
	}
}

func TestPaymentRecoveryRestoreDoesNotAdvanceCanonicalTape(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	bundle := loadA0Fixture(t)
	store := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, store)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	created, err := payments.CreateSubscriptionOrder(ctx, CreateSubscriptionOrderRequest{
		Scope: scope, Plan: "plus", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "restore-does-not-advance",
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewPaymentOrderRecord(scope, created, PaymentOrderSubscriptionPurchase, "restore-does-not-advance")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	app := newPaymentApplication(t, bindings, payments, store, nil)
	recovery, err := NewPaymentRecoveryApplication(app)
	if err != nil {
		t.Fatal(err)
	}
	results, err := recovery.Restore(ctx, actor)
	if err != nil || len(results) != 1 || results[0].Err != nil || results[0].View.State != PaymentUIStatePaying {
		t.Fatalf("Restore = %#v, %v; want cached pending view", results, err)
	}
	saved, found, err := store.Get(ctx, scope, created.Order.OrderID)
	if err != nil || !found || saved.Result.Order.Status != PaymentPending {
		t.Fatalf("Restore advanced saved order = %#v, %v, %v", saved, found, err)
	}
}

func TestPaymentRecoveryRestartsPendingAndDoesNotRepeatGrantedEntitlements(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor, bindings, scope := paymentApplicationBinding(t)
	repository := NewDurablePaymentOrderRepository(provider)
	pending := paymentOrderRepositoryRecord(t, scope, "order_pending", "idem_pending", PaymentPending)
	granted := paymentOrderRepositoryRecord(t, scope, "order_granted", "idem_granted", PaymentGranted)
	if err := repository.Save(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, granted); err != nil {
		t.Fatal(err)
	}

	client := &paymentApplicationClient{}
	firstRefresher := &paymentApplicationRefresher{}
	first := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), firstRefresher)
	firstRecovery, err := NewPaymentRecoveryApplication(first)
	if err != nil {
		t.Fatal(err)
	}
	results, err := firstRecovery.Restore(ctx, actor)
	if err != nil || len(results) != 2 || client.getCalls != 0 || client.verifyCalls != 0 {
		t.Fatalf("first Restore = %#v, %v, get=%d verify=%d", results, err, client.getCalls, client.verifyCalls)
	}
	if firstRefresher.calls != 1 {
		t.Fatalf("crash-window entitlement refresh calls = %d, want 1", firstRefresher.calls)
	}
	marked, found, err := NewDurablePaymentOrderRepository(provider).Get(ctx, scope, granted.Result.Order.OrderID)
	if err != nil || !found || !marked.EntitlementsRefreshed {
		t.Fatalf("restored granted marker = %#v, %v, %v", marked, found, err)
	}
	stillPending, found, err := NewDurablePaymentOrderRepository(provider).Get(ctx, scope, pending.Result.Order.OrderID)
	if err != nil || !found || stillPending != pending {
		t.Fatalf("pending Restore changed record = %#v, %v, %v", stillPending, found, err)
	}

	restartedClient := &paymentApplicationClient{}
	restartedRefresher := &paymentApplicationRefresher{}
	restarted := newPaymentApplication(t, bindings, restartedClient, NewDurablePaymentOrderRepository(provider), restartedRefresher)
	restartedRecovery, err := NewPaymentRecoveryApplication(restarted)
	if err != nil {
		t.Fatal(err)
	}
	results, err = restartedRecovery.Restore(ctx, actor)
	if err != nil || len(results) != 2 || restartedClient.getCalls != 0 || restartedClient.verifyCalls != 0 || restartedRefresher.calls != 0 {
		t.Fatalf("restarted Restore = %#v, %v, get=%d verify=%d entitlement=%d", results, err, restartedClient.getCalls, restartedClient.verifyCalls, restartedRefresher.calls)
	}
}

func TestPaymentRecoveryApplicationKeepsCachedViewWithoutRemoteDiscovery(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	store := NewMemoryPaymentOrderRepository()
	if err := store.Save(ctx, paymentOrderRepositoryRecord(t, scope, "order_01", "idem_01", PaymentPending)); err != nil {
		t.Fatal(err)
	}
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	client := &paymentApplicationClient{getErr: networkErr}
	app := newPaymentApplication(t, bindings, client, store, nil)
	recovery, err := NewPaymentRecoveryApplication(app)
	if err != nil {
		t.Fatal(err)
	}

	results, err := recovery.Restore(ctx, actor)
	if err != nil || len(results) != 1 || results[0].Err != nil || results[0].View.State != PaymentUIStatePaying || client.getCalls != 0 {
		t.Fatalf("network Restore = %#v, %v, get calls=%d", results, err, client.getCalls)
	}
	saved, ok, err := store.Get(ctx, scope, "order_01")
	if err != nil || !ok || saved.Result.Order.Status != PaymentPending {
		t.Fatalf("network changed cached record = %#v, %v, %v", saved, ok, err)
	}

	empty := NewMemoryPaymentOrderRepository()
	emptyApp := newPaymentApplication(t, bindings, client, empty, nil)
	emptyRecovery, err := NewPaymentRecoveryApplication(emptyApp)
	if err != nil {
		t.Fatal(err)
	}
	client.getCalls = 0
	results, err = emptyRecovery.Restore(ctx, actor)
	if err != nil || len(results) != 0 || client.getCalls != 0 {
		t.Fatalf("empty Restore = %#v, %v, get calls=%d", results, err, client.getCalls)
	}
}

func TestPaymentRecoveryApplicationDoesNotExposeExpiredPaymentEntry(t *testing.T) {
	ctx := context.Background()
	actor, bindings, scope := paymentApplicationBinding(t)
	store := NewMemoryPaymentOrderRepository()
	record := paymentOrderRepositoryRecord(t, scope, "order_01", "idem_01", PaymentPending)
	if err := store.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	client := &paymentApplicationClient{getOrder: record.Result.Order}
	app := newPaymentApplication(t, bindings, client, store, nil)
	app.now = func() time.Time { return record.Result.Entry.ExpiresAt }
	recovery, err := NewPaymentRecoveryApplication(app)
	if err != nil {
		t.Fatal(err)
	}

	results, err := recovery.Restore(ctx, actor)
	if err != nil || len(results) != 1 || results[0].Err != nil || results[0].View.Payable || results[0].View.Result.Entry.QRCode != "" || results[0].View.Result.Entry.CodeURL != "" {
		t.Fatalf("expired Restore = %#v, %v", results, err)
	}
}

func TestPaymentRecoveryApplicationDoesNotQueryAnotherProfileOrGeneration(t *testing.T) {
	ctx := context.Background()
	first := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	second := userctx.ActorContext{ActorID: "local", ProfileID: "secondary"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, first, "usr_01"); err != nil {
		t.Fatal(err)
	}
	if _, err := bindings.ConfirmBinding(ctx, second, "usr_02"); err != nil {
		t.Fatal(err)
	}
	firstScope := paymentApplicationScope(t, first, "usr_01", 1)
	store := NewMemoryPaymentOrderRepository()
	if err := store.Save(ctx, paymentOrderRepositoryRecord(t, firstScope, "order_01", "idem_01", PaymentPending)); err != nil {
		t.Fatal(err)
	}
	client := &paymentApplicationClient{}
	app := newPaymentApplication(t, bindings, client, store, nil)
	recovery, err := NewPaymentRecoveryApplication(app)
	if err != nil {
		t.Fatal(err)
	}

	for _, actor := range []userctx.ActorContext{second, first} {
		if actor == first {
			if _, err := bindings.Rebind(ctx, first, "usr_03"); err != nil {
				t.Fatal(err)
			}
		}
		results, err := recovery.Restore(ctx, actor)
		if err != nil || len(results) != 0 || client.getCalls != 0 {
			t.Fatalf("Restore(%#v) = %#v, %v, get calls=%d", actor, results, err, client.getCalls)
		}
	}
}
