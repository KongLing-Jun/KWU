package cloud

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestBalanceApplicationRefreshShowsOnlyCurrentTotal(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	client := &balanceTestClient{states: map[string]AccountState{}}
	scope := balanceScope(t, actor, "usr_01", 1)
	client.states[scope.key()] = balanceState(scope, 7, "21000.0000")
	app := newBalanceApplication(t, bindings, client)

	view, err := app.Refresh(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if view.TotalBalance != "21000.0000" {
		t.Fatalf("total balance = %q", view.TotalBalance)
	}
	if _, ok := any(view).(interface{ FreeBalance() string }); ok {
		t.Fatal("balance view exposes a source-specific balance")
	}
	if got, ok, err := app.View(ctx, actor); err != nil || !ok || got != view {
		t.Fatalf("cached view = %#v, %v, %v", got, ok, err)
	}
}

func TestBalanceApplicationUsesResolvedNonLocalActorBinding(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "desktop-remote", ProfileID: "remote-profile"}
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr_01", 1)
	remote, err := client.GetCurrentUser(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	app := newBalanceApplication(t, bindings, client)

	view, err := app.Refresh(ctx, actor)
	if err != nil || view.TotalBalance != remote.Balance.TotalBalance {
		t.Fatalf("non-local actor refresh = %#v, %v; want total %q", view, err, remote.Balance.TotalBalance)
	}
}

func TestBalanceApplicationIsolatesProfileCloudUserAndGeneration(t *testing.T) {
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
	client := &balanceTestClient{states: map[string]AccountState{}}
	firstScope := balanceScope(t, first, "usr_01", 1)
	secondScope := balanceScope(t, second, "usr_02", 1)
	client.states[firstScope.key()] = balanceState(firstScope, 2, "10.0000")
	client.states[secondScope.key()] = balanceState(secondScope, 3, "20.0000")
	app := newBalanceApplication(t, bindings, client)

	if view, err := app.Refresh(ctx, first); err != nil || view.TotalBalance != "10.0000" {
		t.Fatalf("first refresh = %#v, %v", view, err)
	}
	if view, err := app.Refresh(ctx, second); err != nil || view.TotalBalance != "20.0000" {
		t.Fatalf("second refresh = %#v, %v", view, err)
	}
	if _, err := bindings.Rebind(ctx, first, "usr_03"); err != nil {
		t.Fatal(err)
	}
	currentScope := balanceScope(t, first, "usr_03", 2)
	client.states[currentScope.key()] = balanceState(currentScope, 1, "30.0000")
	if view, err := app.Refresh(ctx, first); err != nil || view.TotalBalance != "30.0000" {
		t.Fatalf("rebound refresh = %#v, %v", view, err)
	}
	if got, ok, err := app.snapshots.LoadBalance(ctx, firstScope); err != nil || !ok || got.TotalBalance != "10.0000" {
		t.Fatalf("old generation snapshot = %#v, %v, %v", got, ok, err)
	}
	if got, ok, err := app.snapshots.LoadBalance(ctx, secondScope); err != nil || !ok || got.TotalBalance != "20.0000" {
		t.Fatalf("second profile snapshot = %#v, %v, %v", got, ok, err)
	}
}

func TestBalanceApplicationDoesNotReplaceNewerSnapshot(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr_01", 1)
	client := &balanceTestClient{states: map[string]AccountState{scope.key(): balanceState(scope, 8, "80.0000")}}
	app := newBalanceApplication(t, bindings, client)
	if _, err := app.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	client.states[scope.key()] = balanceState(scope, 8, "81.0000")
	if view, err := app.Refresh(ctx, actor); err != nil || view.TotalBalance != "80.0000" {
		t.Fatalf("same-version refresh = %#v, %v", view, err)
	}
	client.states[scope.key()] = balanceState(scope, 7, "70.0000")
	if view, err := app.Refresh(ctx, actor); err != nil || view.TotalBalance != "80.0000" {
		t.Fatalf("old-version refresh = %#v, %v", view, err)
	}
}

func TestBalanceApplicationRejectsUnboundAndRemoteScopeWithoutPersistingCandidateMismatch(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	client := &balanceTestClient{states: map[string]AccountState{}}
	app := newBalanceApplication(t, bindings, client)
	if _, err := app.Refresh(ctx, actor); !isProfileMismatch(err) {
		t.Fatalf("unbound refresh error = %v", err)
	}
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr_01", 1)
	client.states[scope.key()] = balanceState(scope, 3, "30.0000")
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_02"); !isProfileMismatch(err) {
		t.Fatalf("mismatch confirmation error = %v", err)
	}
	if view, err := app.Refresh(ctx, actor); err != nil || view.TotalBalance != "30.0000" {
		t.Fatalf("refresh after candidate = %#v, %v", view, err)
	}

	bound := NewBindingRepository()
	if _, err := bound.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	client = &balanceTestClient{states: map[string]AccountState{scope.key(): balanceState(scope, 3, "30.0000")}}
	app = newBalanceApplication(t, bound, client)
	if _, err := app.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	wrongScope := balanceScope(t, actor, "usr_02", 1)
	client.states[scope.key()] = balanceState(wrongScope, 4, "40.0000")
	if view, err := app.Refresh(ctx, actor); !isProfileMismatch(err) || view.TotalBalance != "30.0000" {
		t.Fatalf("scope mismatch refresh = %#v, %v", view, err)
	}
}

func TestBalanceApplicationNetworkFailureKeepsConfirmedSnapshot(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr_01", 1)
	client := &balanceTestClient{states: map[string]AccountState{scope.key(): balanceState(scope, 3, "30.0000")}}
	app := newBalanceApplication(t, bindings, client)
	if _, err := app.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	client.err = networkErr
	if view, err := app.Refresh(ctx, actor); !errors.Is(err, networkErr) || view.TotalBalance != "30.0000" {
		t.Fatalf("network refresh = %#v, %v", view, err)
	}
}

func TestBalanceApplicationRestoresDurableCloudSnapshot(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_01"); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr_01", 1)
	provider := testProfilePathProvider(t.TempDir())
	repository := NewDurableAssetSnapshotRepository(provider)
	client := &balanceTestClient{states: map[string]AccountState{scope.key(): balanceState(scope, 8, "900.0000")}}
	first, err := NewBalanceApplication(BalanceApplicationDependencies{Bindings: bindings, Client: client, Snapshots: repository})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := first.Refresh(ctx, actor); err != nil || got.TotalBalance != "900.0000" {
		t.Fatalf("first refresh = %#v, %v", got, err)
	}

	restarted, err := NewBalanceApplication(BalanceApplicationDependencies{Bindings: bindings, Client: client, Snapshots: NewDurableAssetSnapshotRepository(provider)})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok, err := restarted.View(ctx, actor); err != nil || !ok || got.TotalBalance != "900.0000" {
		t.Fatalf("restored view = %#v, %v, %v", got, ok, err)
	}
	client.states[scope.key()] = balanceState(scope, 7, "100.0000")
	if got, err := restarted.Refresh(ctx, actor); err != nil || got.TotalBalance != "900.0000" {
		t.Fatalf("older cloud version replaced durable snapshot: %#v, %v", got, err)
	}
}

func newBalanceApplication(t *testing.T, bindings BindingResolver, client CloudAccountClient) *BalanceApplication {
	t.Helper()
	app, err := NewBalanceApplication(BalanceApplicationDependencies{Bindings: bindings, Client: client})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func balanceScope(t *testing.T, actor userctx.ActorContext, cloudUserID string, generation uint64) BindingScope {
	t.Helper()
	scope, err := NewBindingScope(actor, cloudUserID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func balanceState(scope BindingScope, version uint64, total string) AccountState {
	now := time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)
	return AccountState{
		Binding:      ProfileBinding{ActorID: "local", ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, Status: BindingBound, BindingGeneration: scope.BindingGeneration},
		State:        AccountAuthenticated,
		User:         CloudUser{CloudUserID: scope.CloudUserID},
		Balance:      BalanceSnapshot{TotalBalance: total, FreeBalance: "1000.0000", SubscriptionBalance: "20000.0000", AvailableBalance: total, FreeAvailableBalance: "1000.0000", SubscriptionAvailableBalance: "20000.0000", FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000", Version: version},
		Subscription: Subscription{Scope: scope, Plan: "plus", Status: SubscriptionActive, PeriodEnd: now.AddDate(0, 1, 0), UpdatedAt: now},
		LastSyncAt:   now,
	}
}

type balanceTestClient struct {
	CloudAccountClient
	states map[string]AccountState
	err    error
}

func (c *balanceTestClient) GetCurrentUser(_ context.Context, scope BindingScope) (AccountState, error) {
	if c.err != nil {
		return AccountState{}, c.err
	}
	return c.states[scope.key()], nil
}
