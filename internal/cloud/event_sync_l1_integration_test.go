package cloud

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"kwu/internal/userctx"
)

// These tests deliberately use only the frozen A0 normalized fixture and L1
// fakes. CLOUD-05 and CLOUD-09 remain blocked: no transport, replay, or
// remote pending-list capability is exercised here.
func TestL1EventSyncFixtureDrivesAuthoritativeApplicationsAndRestart(t *testing.T) {
	ctx := context.Background()
	system := newL1EventSyncSystem(t)
	events := []Event{
		l1FixtureEvent(t, system.bundle, "evt_settlement_01"),
		l1FixtureEvent(t, system.bundle, "evt_subscription_updated"),
		l1FixtureEvent(t, system.bundle, "evt_payment_pending"),
		l1FixtureEvent(t, system.bundle, "evt_sync_updated"),
	}
	source, err := NewFixtureEventSource(events)
	if err != nil {
		t.Fatal(err)
	}
	if err := system.run(source); err != nil {
		t.Fatalf("Run fixture events: %v", err)
	}

	if view, ok, err := system.balance.View(ctx, system.actor); err != nil || !ok || view.TotalBalance != "21000.0000" {
		t.Fatalf("authoritative balance view = %#v, %t, %v", view, ok, err)
	}
	if view, ok, err := system.subscription.View(ctx, system.actor); err != nil || !ok || view.Subscription.Plan != "plus" || len(view.Plans) != len(system.bundle.PlanCases) {
		t.Fatalf("authoritative subscription view = %#v, %t, %v", view, ok, err)
	}
	if records, err := system.tasksStore.Load(ctx, system.scope); err != nil || len(records) != 1 || records[0].FreezeID != "frz_01" {
		t.Fatalf("local task recovery records = %#v, %v", records, err)
	}
	for _, orderID := range []string{"ord_pending", "ord_granted"} {
		record, found, err := system.orders.Get(ctx, system.scope, orderID)
		if err != nil || !found {
			t.Fatalf("local payment record %q = %#v, %t, %v", orderID, record, found, err)
		}
		if record.EntitlementsRefreshed != (orderID == "ord_granted") {
			t.Fatalf("payment record %q entitlement marker = %t", orderID, record.EntitlementsRefreshed)
		}
	}
	if system.entitlements.calls != 1 {
		t.Fatalf("entitlement refresh calls = %d, want one granted-order refresh", system.entitlements.calls)
	}
	if system.sync.calls != 1 {
		t.Fatalf("ApplyThirdPartySync calls = %d, want 1", system.sync.calls)
	}
	checkpoint, found, err := system.state.Load(ctx, system.scope)
	if err != nil || !found || checkpoint.Cursor != "evt_sync_updated" {
		t.Fatalf("checkpoint = %#v, %t, %v", checkpoint, found, err)
	}

	// Reusing the state repository models a process restart. The duplicate must
	// be skipped before the real AuthApplication can receive it, while the next
	// valid event still advances the checkpoint cursor contract.
	next := l1FixtureEvent(t, system.bundle, "evt_balance_updated")
	next.EventID = "evt_balance_updated_v20"
	next.Version = 20
	restartedSource, err := NewFixtureEventSource([]Event{l1FixtureEvent(t, system.bundle, "evt_sync_updated"), next})
	if err != nil {
		t.Fatal(err)
	}
	if err := system.run(restartedSource); err != nil {
		t.Fatalf("restart duplicate followed by valid event: %v", err)
	}
	if system.sync.calls != 1 {
		t.Fatalf("duplicate reached ApplyThirdPartySync: calls = %d", system.sync.calls)
	}
	checkpoint, found, err = system.state.Load(ctx, system.scope)
	if err != nil || !found || checkpoint.Cursor != next.EventID {
		t.Fatalf("checkpoint after restart duplicate = %#v, %t, %v", checkpoint, found, err)
	}
}

func TestL1EventSyncFixtureRejectsOtherUserBeforeFollowingEvent(t *testing.T) {
	ctx := context.Background()
	system := newL1EventSyncSystem(t)
	otherUser := l1FixtureEvent(t, system.bundle, "evt_other_user")
	next := l1FixtureEvent(t, system.bundle, "evt_subscription_updated")
	source, err := NewFixtureEventSource([]Event{otherUser, next})
	if err != nil {
		t.Fatal(err)
	}
	if err := system.run(source); !isCloudCode(err, ErrorProfileScopeMismatch) {
		t.Fatalf("Run other-user event = %v, want profile scope mismatch", err)
	}
	checkpoint, found, err := system.state.Load(ctx, system.scope)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint after other-user rejection = %#v, %t, %v", checkpoint, found, err)
	}
}

func TestL1EventSyncFixtureRejectsOldGenerationAfterConnect(t *testing.T) {
	ctx := context.Background()
	system := newL1EventSyncSystem(t)
	source := &l1EventRebindingSource{
		event: l1FixtureEvent(t, system.bundle, "evt_balance_updated"),
		onConnect: func() {
			if _, err := system.bindings.Rebind(ctx, system.actor, "usr_rebound"); err != nil {
				t.Fatalf("Rebind: %v", err)
			}
		},
	}
	if err := system.run(source); !errors.Is(err, ErrEventBindingChanged) {
		t.Fatalf("Run old-generation event = %v, want ErrEventBindingChanged", err)
	}
	if _, ok, err := system.balance.View(ctx, system.actor); err != nil || ok {
		t.Fatalf("old-generation event mutated balance snapshot: found=%t err=%v", ok, err)
	}
	checkpoint, found, err := system.state.Load(ctx, system.scope)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("old-generation event advanced old checkpoint = %#v, %t, %v", checkpoint, found, err)
	}
}

func TestL1EventSyncFixtureRetriesFailedAuthoritativeRefreshWithoutCheckpointAdvance(t *testing.T) {
	ctx := context.Background()
	system := newL1EventSyncSystem(t)
	accounts, err := NewFixtureAccountClient(system.bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	client := &l1EventFailingAccountClient{CloudAccountClient: accounts, err: &Error{Code: ErrorNetworkUnavailable, Message: "fixture offline", Retryable: true}}
	system.balance, err = NewBalanceApplication(BalanceApplicationDependencies{Bindings: system.bindings, Client: client})
	if err != nil {
		t.Fatal(err)
	}
	event := l1FixtureEvent(t, system.bundle, "evt_balance_updated")
	failedSource, err := NewFixtureEventSource([]Event{event})
	if err != nil {
		t.Fatal(err)
	}
	if err := system.run(failedSource); err == nil {
		t.Fatal("Run accepted a failed authoritative balance refresh")
	}
	checkpoint, found, err := system.state.Load(ctx, system.scope)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("failed refresh advanced checkpoint = %#v, %t, %v", checkpoint, found, err)
	}

	client.err = nil
	retrySource, err := NewFixtureEventSource([]Event{event})
	if err != nil {
		t.Fatal(err)
	}
	if err := system.run(retrySource); err != nil {
		t.Fatalf("retry Run: %v", err)
	}
	checkpoint, found, err = system.state.Load(ctx, system.scope)
	if err != nil || !found || checkpoint.Cursor != event.EventID {
		t.Fatalf("retry checkpoint = %#v, %t, %v", checkpoint, found, err)
	}
}

func TestL1EventSyncFixtureReconnectsReplayBeforeFollowingEvent(t *testing.T) {
	ctx := context.Background()
	system := newL1EventSyncSystem(t)
	accounts, err := NewFixtureAccountClient(system.bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	retryable := &Error{Code: ErrorNetworkUnavailable, Message: "fixture offline", Retryable: true}
	client := &l1EventFailingAccountClient{CloudAccountClient: accounts, errs: []error{retryable, nil}}
	system.balance, err = NewBalanceApplication(BalanceApplicationDependencies{Bindings: system.bindings, Client: client})
	if err != nil {
		t.Fatal(err)
	}
	failed := l1FixtureEvent(t, system.bundle, "evt_balance_updated")
	following := l1FixtureEvent(t, system.bundle, "evt_subscription_updated")
	source := &eventSyncSource{streams: []<-chan Event{
		eventStream(failed, following),
		eventStream(failed, following),
	}}
	app, err := NewEventSyncApplication(EventSyncApplicationDependencies{
		Bindings: system.bindings, Source: source, State: system.state, Balance: system.balance, Tasks: system.tasks,
		Settlements: system.settlements, Subscription: system.subscription, Payment: system.payment, Sync: system.sync, MaxReconnects: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, system.actor); err != nil {
		t.Fatalf("Run retryable fixture replay: %v", err)
	}
	if source.calls != 2 {
		t.Fatalf("Connect calls = %d, want one replay connection", source.calls)
	}
	checkpoint, found, err := system.state.Load(ctx, system.scope)
	if err != nil || !found || checkpoint.Cursor != following.EventID || !checkpoint.HasApplied(failed.EventID) {
		t.Fatalf("checkpoint after replay = (%#v, %t, %v)", checkpoint, found, err)
	}
}

type l1EventSyncSystem struct {
	bundle       FixtureBundle
	actor        userctx.ActorContext
	scope        BindingScope
	bindings     *BindingRepository
	state        EventStateRepository
	balance      *BalanceApplication
	tasks        *TaskRecoveryApplication
	tasksStore   *MemoryTaskRecoveryStore
	settlements  EventSettlementQuotaReconciler
	subscription *SubscriptionApplication
	orders       *MemoryPaymentOrderRepository
	payment      *PaymentRecoveryApplication
	sync         *l1EventAuthSync
	entitlements *l1EventEntitlements
}

func newL1EventSyncSystem(t *testing.T) *l1EventSyncSystem {
	t.Helper()
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	actor := userctx.Default(t.TempDir())
	scope, err := bundle.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	bindings := NewBindingRepository()
	if binding, err := bindings.ConfirmBinding(ctx, actor, scope.CloudUserID); err != nil || binding != bundle.Binding {
		t.Fatalf("ConfirmBinding = %#v, %v", binding, err)
	}
	accounts, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	payments := newL1EventFixturePayments(t, bundle)
	balance, err := NewBalanceApplication(BalanceApplicationDependencies{Bindings: bindings, Client: accounts})
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := NewSubscriptionApplication(SubscriptionApplicationDependencies{Bindings: bindings, Accounts: accounts, Plans: payments})
	if err != nil {
		t.Fatal(err)
	}
	credit, err := NewFixtureCreditClient(bundle.FreezeCases)
	if err != nil {
		t.Fatal(err)
	}
	tasksStore := NewMemoryTaskRecoveryStore()
	freeze := l1FixtureFreeze(t, bundle, "freeze_granted")
	if err := tasksStore.Save(ctx, TaskRecoveryRecord{Scope: scope, TaskID: freeze.Result.Freeze.TaskID, FreezeID: freeze.Result.Freeze.FreezeID, IdempotencyKey: freeze.Request.IdempotencyKey}); err != nil {
		t.Fatal(err)
	}
	tasks := NewTaskRecoveryApplicationWithStore(credit, bindings, tasksStore)
	orders := NewMemoryPaymentOrderRepository()
	for _, name := range []string{"payment_pending", "granted"} {
		fixture := l1FixturePayment(t, bundle, name)
		record, err := NewPaymentOrderRecord(scope, fixture.Result, fixture.Result.Order.OrderType, "event-sync-"+name)
		if err != nil {
			t.Fatal(err)
		}
		if err := orders.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	entitlements := &l1EventEntitlements{}
	paymentApp, err := NewPaymentApplication(PaymentApplicationDependencies{Bindings: bindings, Payments: payments, Orders: orders, Entitlements: entitlements})
	if err != nil {
		t.Fatal(err)
	}
	payment, err := NewPaymentRecoveryApplication(paymentApp)
	if err != nil {
		t.Fatal(err)
	}
	sessionStore := NewMemorySessionStore()
	sessions := NewProfileSessionRepository(bindings, sessionStore)
	captcha, err := NewMockCaptchaGateway("event-sync-captcha", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	auth, err := NewAuthApplication(AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions, Cleanup: NewIdentityCleanup(sessionStore, l1EventNoopScopedStore{}, l1EventNoopScopedStore{}),
		Captcha: captcha, Encryptor: NewTestPasswordEncryptor(), Client: accounts, FixtureVersion: A0FixtureVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &l1EventSyncSystem{
		bundle: bundle, actor: actor, scope: scope, bindings: bindings, state: NewMemoryEventStateRepository(),
		balance: balance, tasks: tasks, tasksStore: tasksStore, settlements: &l1EventSettlementReconciler{}, subscription: subscription, orders: orders,
		payment: payment, sync: &l1EventAuthSync{auth: auth}, entitlements: entitlements,
	}
}

func (s *l1EventSyncSystem) run(source CloudEventSource) error {
	app, err := NewEventSyncApplication(EventSyncApplicationDependencies{
		Bindings: s.bindings, Source: source, State: s.state, Balance: s.balance, Tasks: s.tasks,
		Settlements: s.settlements, Subscription: s.subscription, Payment: s.payment, Sync: s.sync,
	})
	if err != nil {
		return err
	}
	return app.Run(context.Background(), s.actor)
}

type l1EventSettlementReconciler struct{}

func (*l1EventSettlementReconciler) ReconcileSettlementEvent(context.Context, userctx.ActorContext, BindingScope, string, string) error {
	return nil
}

type l1EventAuthSync struct {
	auth  *AuthApplication
	calls int
}

func (a *l1EventAuthSync) ApplyThirdPartySync(ctx context.Context, actor userctx.ActorContext, state ThirdPartySync) error {
	a.calls++
	return a.auth.ApplyThirdPartySync(ctx, actor, state)
}

type l1EventEntitlements struct{ calls int }

func (e *l1EventEntitlements) RefreshPaymentEntitlements(context.Context, userctx.ActorContext) error {
	e.calls++
	return nil
}

type l1EventNoopScopedStore struct{}

func (l1EventNoopScopedStore) Clear(context.Context, BindingScope) error { return nil }

type l1EventFixturePayments struct {
	plans  []SubscriptionPlan
	orders map[string]PaymentOrder
}

type l1EventFailingAccountClient struct {
	CloudAccountClient
	err  error
	errs []error
}

func (c *l1EventFailingAccountClient) GetCurrentUser(ctx context.Context, scope BindingScope) (AccountState, error) {
	if len(c.errs) > 0 {
		err := c.errs[0]
		c.errs = c.errs[1:]
		if err != nil {
			return AccountState{}, err
		}
	}
	if c.err != nil {
		return AccountState{}, c.err
	}
	return c.CloudAccountClient.GetCurrentUser(ctx, scope)
}

func newL1EventFixturePayments(t *testing.T, bundle FixtureBundle) *l1EventFixturePayments {
	t.Helper()
	orders := make(map[string]PaymentOrder, len(bundle.PaymentCases))
	for _, fixture := range bundle.PaymentCases {
		orders[fixture.Result.Order.OrderID] = fixture.Result.Order
	}
	return &l1EventFixturePayments{plans: append([]SubscriptionPlan(nil), bundle.PlanCases...), orders: orders}
}

func (c *l1EventFixturePayments) ListSubscriptionPlans(context.Context, BindingScope) ([]SubscriptionPlan, error) {
	return append([]SubscriptionPlan(nil), c.plans...), nil
}

func (c *l1EventFixturePayments) CreateSubscriptionOrder(context.Context, CreateSubscriptionOrderRequest) (PaymentOrderResult, error) {
	return PaymentOrderResult{}, fmt.Errorf("fixture payment creation is outside event recovery")
}

func (c *l1EventFixturePayments) CreateUpgradeOrder(context.Context, CreateUpgradeOrderRequest) (PaymentOrderResult, error) {
	return PaymentOrderResult{}, fmt.Errorf("fixture payment creation is outside event recovery")
}

func (c *l1EventFixturePayments) GetPaymentOrder(_ context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	order, ok := c.orders[orderID]
	if !ok || order.Scope != scope {
		return PaymentOrder{}, &Error{Code: ErrorProfileScopeMismatch, Message: "fixture payment order is not locally saved for this scope"}
	}
	return order, nil
}

func (c *l1EventFixturePayments) VerifyPaymentOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrder, error) {
	return c.GetPaymentOrder(ctx, scope, orderID)
}

type l1EventRebindingSource struct {
	event     Event
	onConnect func()
}

func (s *l1EventRebindingSource) Connect(context.Context, BindingScope) (<-chan Event, error) {
	s.onConnect()
	stream := make(chan Event, 1)
	stream <- s.event
	close(stream)
	return stream, nil
}

func l1FixtureEvent(t *testing.T, bundle FixtureBundle, id string) Event {
	t.Helper()
	for _, event := range bundle.Events {
		if event.EventID == id {
			return event
		}
	}
	t.Fatalf("missing frozen event %q", id)
	return Event{}
}

func l1FixtureFreeze(t *testing.T, bundle FixtureBundle, name string) FreezeFixture {
	t.Helper()
	for _, fixture := range bundle.FreezeCases {
		if fixture.Name == name {
			return fixture
		}
	}
	t.Fatalf("missing frozen freeze fixture %q", name)
	return FreezeFixture{}
}

func l1FixturePayment(t *testing.T, bundle FixtureBundle, name string) PaymentFixture {
	t.Helper()
	for _, fixture := range bundle.PaymentCases {
		if fixture.Name == name {
			return fixture
		}
	}
	t.Fatalf("missing frozen payment fixture %q", name)
	return PaymentFixture{}
}
