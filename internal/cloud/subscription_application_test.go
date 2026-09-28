package cloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestHTTPSubscriptionPlanClientReadsPublicCloudCatalog(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != subscriptionPlansPath || request.URL.RawQuery != "" {
			t.Fatalf("request = %s %s?%s", request.Method, request.URL.Path, request.URL.RawQuery)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Fatalf("public plan catalog Authorization = %q", authorization)
		}
		_, _ = w.Write([]byte(`{"data":{"plans":[{"plan_id":"plan_lite_v1","plan":"lite","display_name":"Lite","price_amount":"9.90","currency":"CNY","credit_amount":"20000.0000","period_days":30,"enabled":true,"sort_order":10,"config_version":"catalog-v1"},{"plan_id":"plan_plus_v1","plan":"plus","display_name":"Plus","price_amount":"29.90","currency":"CNY","credit_amount":"60000.0000","period_days":30,"enabled":true,"sort_order":20,"config_version":"catalog-v1"}]},"error":null,"request_id":"req-plans","server_time":"2026-07-18T04:00:00Z"}`))
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPSubscriptionPlanClient(transport)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := client.ListSubscriptionPlans(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Plan != "lite" || plans[1].Plan != "plus" || plans[1].PriceAmount != "29.90" || plans[1].ConfigVersion != "catalog-v1" {
		t.Fatalf("plans = %#v", plans)
	}
}

func TestHTTPSubscriptionPlanClientRejectsInvalidCloudCatalog(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_remote", BindingGeneration: 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"plans":[{"plan":"plus","display_name":"Plus","price_amount":"29.90","currency":"USD","credit_amount":"60000.0000","period_days":30,"enabled":true,"sort_order":20,"config_version":"catalog-v1"}]},"error":null,"request_id":"req-plans","server_time":"2026-07-18T04:00:00Z"}`))
	}))
	defer server.Close()
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPSubscriptionPlanClient(transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListSubscriptionPlans(context.Background(), scope); err == nil || !isCloudCode(err, ErrorCode("CLOUD_PROTOCOL_ERROR")) {
		t.Fatalf("invalid catalog error = %v", err)
	}
}

func TestSubscriptionApplicationRefreshUsesOnlyCloudSnapshots(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	plans := []SubscriptionPlan{{Plan: "premium", DisplayName: "Premium", PriceAmount: "31.25", Currency: "CNY", CreditAmount: "54321.0000", PeriodDays: 45, Enabled: true, SortOrder: 7, ConfigVersion: "3"}}
	state := subscriptionTestState(scope, Subscription{Scope: scope, Plan: "free", Status: SubscriptionFree, UpdatedAt: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)})
	state.Balance.TotalBalance = "not-an-entitlement"
	accounts := &subscriptionAccountClient{states: map[string]AccountState{scope.key(): state}}
	payments := &subscriptionPaymentClient{plans: map[string][]SubscriptionPlan{scope.key(): plans}}
	app := newSubscriptionApplication(t, bindings, accounts, payments)

	got, err := app.Refresh(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Plans, plans) || got.Subscription != state.Subscription {
		t.Fatalf("view = %#v, want cloud plans and subscription", got)
	}
	if got.Subscription.Status != SubscriptionFree || !got.Subscription.PeriodEnd.IsZero() {
		t.Fatalf("local refresh rewrote cloud free entitlement: %#v", got.Subscription)
	}
	plans[0].PriceAmount = "locally-mutated"
	if cached, ok, err := app.View(ctx, actor); err != nil || !ok || cached.Plans[0].PriceAmount != "31.25" {
		t.Fatalf("cached cloud snapshot = %#v, %v, %v", cached, ok, err)
	}
}

func TestSubscriptionApplicationUsesResolvedNonLocalActorBinding(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "desktop-remote", ProfileID: "remote-profile"}
	bundle := loadA0Fixture(t)
	accounts, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewFixturePlanClient(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	remote, err := accounts.GetCurrentUser(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	app := newSubscriptionApplication(t, bindings, accounts, plans)

	view, err := app.Refresh(ctx, actor)
	if err != nil || view.Subscription != remote.Subscription || !reflect.DeepEqual(view.Plans, bundle.PlanCases) {
		t.Fatalf("non-local actor refresh = %#v, %v; want subscription %#v", view, err, remote.Subscription)
	}
}

func TestSubscriptionApplicationAcceptsPlanListOnlyClient(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	accounts := &subscriptionAccountClient{states: map[string]AccountState{
		scope.key(): subscriptionTestState(scope, subscriptionActive(scope, "plus")),
	}}
	plans := &subscriptionPlanOnlyClient{plans: []SubscriptionPlan{subscriptionPlan("plus", "19.90")}}
	app, err := NewSubscriptionApplication(SubscriptionApplicationDependencies{
		Bindings: bindings,
		Accounts: accounts,
		Plans:    plans,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view, err := app.Refresh(ctx, actor); err != nil || len(view.Plans) != 1 || view.Plans[0].Plan != "plus" {
		t.Fatalf("Refresh with plan-list-only client = %#v, %v", view, err)
	}
}

func TestSubscriptionApplicationIsolatesProfileCloudUserAndGeneration(t *testing.T) {
	ctx := context.Background()
	first := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	second := userctx.ActorContext{ActorID: "local", ProfileID: "secondary"}
	bindings := NewBindingRepository()
	for _, binding := range []struct {
		actor userctx.ActorContext
		user  string
	}{{first, "usr_01"}, {second, "usr_02"}} {
		if _, err := bindings.ConfirmBinding(ctx, binding.actor, binding.user); err != nil {
			t.Fatal(err)
		}
	}
	firstScope := subscriptionScope(t, first, "usr_01", 1)
	secondScope := subscriptionScope(t, second, "usr_02", 1)
	accounts := &subscriptionAccountClient{states: map[string]AccountState{
		firstScope.key():  subscriptionTestState(firstScope, subscriptionActive(firstScope, "basic")),
		secondScope.key(): subscriptionTestState(secondScope, subscriptionActive(secondScope, "pro")),
	}}
	payments := &subscriptionPaymentClient{plans: map[string][]SubscriptionPlan{
		firstScope.key():  {subscriptionPlan("basic", "10.00")},
		secondScope.key(): {subscriptionPlan("pro", "20.00")},
	}}
	app := newSubscriptionApplication(t, bindings, accounts, payments)
	if got, err := app.Refresh(ctx, first); err != nil || got.Subscription.Plan != "basic" {
		t.Fatalf("first refresh = %#v, %v", got, err)
	}
	if got, err := app.Refresh(ctx, second); err != nil || got.Subscription.Plan != "pro" {
		t.Fatalf("second refresh = %#v, %v", got, err)
	}
	if _, err := bindings.Rebind(ctx, first, "usr_03"); err != nil {
		t.Fatal(err)
	}
	currentScope := subscriptionScope(t, first, "usr_03", 2)
	accounts.states[currentScope.key()] = subscriptionTestState(currentScope, subscriptionActive(currentScope, "enterprise"))
	payments.plans[currentScope.key()] = []SubscriptionPlan{subscriptionPlan("enterprise", "30.00")}
	if got, ok, err := app.View(ctx, first); err != nil || ok || !reflect.DeepEqual(got, SubscriptionView{}) {
		t.Fatalf("old generation view = %#v, %v, %v", got, ok, err)
	}
	if got, err := app.Refresh(ctx, first); err != nil || got.Subscription.Plan != "enterprise" {
		t.Fatalf("rebound refresh = %#v, %v", got, err)
	}
}

func TestSubscriptionApplicationRejectsRemoteScopeAndKeepsConfirmedSnapshot(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	accounts := &subscriptionAccountClient{states: map[string]AccountState{scope.key(): subscriptionTestState(scope, subscriptionActive(scope, "basic"))}}
	payments := &subscriptionPaymentClient{plans: map[string][]SubscriptionPlan{scope.key(): {subscriptionPlan("basic", "10.00")}}}
	app := newSubscriptionApplication(t, bindings, accounts, payments)
	if _, err := app.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	wrongScope := subscriptionScope(t, actor, "usr_02", 1)
	accounts.states[scope.key()] = subscriptionTestState(wrongScope, subscriptionActive(wrongScope, "pro"))
	if got, err := app.Refresh(ctx, actor); !isProfileMismatch(err) || got.Subscription.Plan != "basic" {
		t.Fatalf("mismatched refresh = %#v, %v", got, err)
	}
}

func TestSubscriptionApplicationNetworkFailureUsesOnlyCurrentScopeCache(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	accounts := &subscriptionAccountClient{states: map[string]AccountState{scope.key(): subscriptionTestState(scope, subscriptionActive(scope, "basic"))}}
	payments := &subscriptionPaymentClient{plans: map[string][]SubscriptionPlan{scope.key(): {subscriptionPlan("basic", "10.00")}}}
	app := newSubscriptionApplication(t, bindings, accounts, payments)
	if _, err := app.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	payments.err = networkErr
	if got, err := app.Refresh(ctx, actor); !errors.Is(err, networkErr) || got.Subscription.Plan != "basic" {
		t.Fatalf("network refresh = %#v, %v", got, err)
	}
}

func TestSubscriptionApplicationRejectsInvalidPlanAndKeepsConfirmedSnapshot(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	accounts := &subscriptionAccountClient{states: map[string]AccountState{scope.key(): subscriptionTestState(scope, subscriptionActive(scope, "basic"))}}
	payments := &subscriptionPaymentClient{plans: map[string][]SubscriptionPlan{scope.key(): {subscriptionPlan("basic", "10.00")}}}
	app := newSubscriptionApplication(t, bindings, accounts, payments)
	if _, err := app.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	payments.plans[scope.key()] = []SubscriptionPlan{{Plan: "bad", DisplayName: "Bad", PriceAmount: "1", Currency: "USD", CreditAmount: "1", PeriodDays: 1, ConfigVersion: "1"}}
	if got, err := app.Refresh(ctx, actor); err == nil || got.Subscription.Plan != "basic" || got.Plans[0].Plan != "basic" {
		t.Fatalf("invalid plan refresh = %#v, %v", got, err)
	}
}

func TestSubscriptionApplicationRestoresDurableCloudSnapshot(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := subscriptionScope(t, actor, "usr_01", 1)
	plans := []SubscriptionPlan{{Plan: "premium", DisplayName: "Premium", PriceAmount: "31.25", Currency: "CNY", CreditAmount: "54321.0000", PeriodDays: 45, Enabled: true, SortOrder: 7, ConfigVersion: "3"}}
	state := subscriptionTestState(scope, subscriptionActive(scope, "premium"))
	accounts := &subscriptionAccountClient{states: map[string]AccountState{scope.key(): state}}
	payments := &subscriptionPaymentClient{plans: map[string][]SubscriptionPlan{scope.key(): plans}}
	provider := testProfilePathProvider(t.TempDir())
	first, err := NewSubscriptionApplication(SubscriptionApplicationDependencies{Bindings: bindings, Accounts: accounts, Plans: payments, Snapshots: NewDurableAssetSnapshotRepository(provider)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewSubscriptionApplication(SubscriptionApplicationDependencies{Bindings: bindings, Accounts: accounts, Plans: payments, Snapshots: NewDurableAssetSnapshotRepository(provider)})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := restarted.View(ctx, actor)
	if err != nil || !ok || !reflect.DeepEqual(got.Plans, plans) || got.Subscription != state.Subscription {
		t.Fatalf("restored subscription = %#v, %v, %v", got, ok, err)
	}
}

func newSubscriptionApplication(t *testing.T, bindings BindingResolver, accounts CloudAccountClient, plans SubscriptionPlanClient) *SubscriptionApplication {
	t.Helper()
	app, err := NewSubscriptionApplication(SubscriptionApplicationDependencies{Bindings: bindings, Accounts: accounts, Plans: plans})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func subscriptionScope(t *testing.T, actor userctx.ActorContext, cloudUserID string, generation uint64) BindingScope {
	t.Helper()
	scope, err := NewBindingScope(actor, cloudUserID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func subscriptionPlan(plan, price string) SubscriptionPlan {
	return SubscriptionPlan{Plan: plan, DisplayName: plan, PriceAmount: price, Currency: "CNY", CreditAmount: "123.0000", PeriodDays: 30, Enabled: true, ConfigVersion: "1"}
}

func subscriptionActive(scope BindingScope, plan string) Subscription {
	now := time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)
	return Subscription{Scope: scope, Plan: plan, Status: SubscriptionActive, PeriodEnd: now.AddDate(0, 1, 0), UpdatedAt: now}
}

func subscriptionTestState(scope BindingScope, subscription Subscription) AccountState {
	now := time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)
	return AccountState{
		Binding:      ProfileBinding{ActorID: "local", ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, Status: BindingBound, BindingGeneration: scope.BindingGeneration},
		State:        AccountAuthenticated,
		User:         CloudUser{CloudUserID: scope.CloudUserID},
		Balance:      BalanceSnapshot{TotalBalance: "1.0000", FreeBalance: "1.0000", SubscriptionBalance: "0.0000", AvailableBalance: "1.0000", FreeAvailableBalance: "1.0000", SubscriptionAvailableBalance: "0.0000", FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000", Version: 1},
		Subscription: subscription,
		LastSyncAt:   now,
	}
}

type subscriptionAccountClient struct {
	CloudAccountClient
	states map[string]AccountState
	err    error
}

func (c *subscriptionAccountClient) GetCurrentUser(_ context.Context, scope BindingScope) (AccountState, error) {
	if c.err != nil {
		return AccountState{}, c.err
	}
	return c.states[scope.key()], nil
}

type subscriptionPaymentClient struct {
	CloudPaymentClient
	plans map[string][]SubscriptionPlan
	err   error
}

type subscriptionPlanOnlyClient struct {
	plans []SubscriptionPlan
}

func (c *subscriptionPlanOnlyClient) ListSubscriptionPlans(context.Context, BindingScope) ([]SubscriptionPlan, error) {
	return append([]SubscriptionPlan(nil), c.plans...), nil
}

func (c *subscriptionPaymentClient) ListSubscriptionPlans(_ context.Context, scope BindingScope) ([]SubscriptionPlan, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.plans[scope.key()], nil
}
