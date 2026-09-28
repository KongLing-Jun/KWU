package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestDesktopRemoteSettlementObserverNotifiesAfterBalanceStore(t *testing.T) {
	observer, runtime, client, scope := newDesktopRemoteSettlementObserverTestHarness(t, nil)
	notifier := newCloudAssetsNotifier()
	deliveries := make(chan CloudAssetsChangedView, 2)
	notifier.bind(func(view CloudAssetsChangedView) { deliveries <- view })
	notifier.start()
	t.Cleanup(notifier.close)
	runtime.assetNotifier = notifier

	if err := observer.CreditSettlementObserved(context.Background(), scope, "task-01", "settlement-01"); err != nil {
		t.Fatal(err)
	}
	assertDesktopSettlementBalanceWakeup(t, deliveries)
	if client.currentUserCalls() != 1 {
		t.Fatalf("CurrentUser calls = %d, want 1", client.currentUserCalls())
	}
	stored, found, err := runtime.assets.LoadBalance(context.Background(), scope)
	if err != nil || !found || stored.Version != 2 || stored.TotalBalance != "90.0000" {
		t.Fatalf("stored balance = %#v, found=%t, err=%v", stored, found, err)
	}
}

func TestDesktopRemoteSettlementObserverEmitsExactTurnCredits(t *testing.T) {
	observer, runtime, _, binding := newDesktopRemoteSettlementObserverTestHarness(t, nil)
	charged := modelbilling.DecimalString("7.9000")
	observer.taskScope = modelbilling.TaskScope{
		ActorID: observer.actor.ActorID, ProfileID: observer.actor.ProfileID, CloudBinding: &binding,
	}
	observer.settlements = desktopStaticQuotaSettlementStore{result: modelbilling.QuotaSettlementResult{
		Settlement: modelbilling.QuotaSettlement{TaskID: "task-01", ChargedCredits: &charged},
	}, found: true}
	deliveries := make(chan TurnBillingSettledView, 1)
	runtime.bindTurnBillingSettledSender(func(view TurnBillingSettledView) { deliveries <- view })

	if err := observer.CreditSettlementObserved(context.Background(), binding, "task-01", "settlement-01"); err != nil {
		t.Fatal(err)
	}
	select {
	case view := <-deliveries:
		if view.TaskID != "task-01" || view.ChargedCredits != "7.9000" {
			t.Fatalf("turn billing view = %#v", view)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for exact turn billing notification")
	}
}

func TestDesktopRemoteSettlementObserverFailurePathsDoNotNotify(t *testing.T) {
	t.Run("remote read failure", func(t *testing.T) {
		remoteErr := errors.New("remote current user failed")
		observer, runtime, _, scope := newDesktopRemoteSettlementObserverTestHarness(t, remoteErr)
		notifier := newCloudAssetsNotifier()
		runtime.assetNotifier = notifier

		err := observer.CreditSettlementObserved(context.Background(), scope, "task-01", "settlement-01")
		if !errors.Is(err, remoteErr) {
			t.Fatalf("observer error = %v, want %v", err, remoteErr)
		}
		assertDesktopSettlementNotifierClean(t, notifier)
	})

	t.Run("scope mismatch", func(t *testing.T) {
		observer, runtime, _, scope := newDesktopRemoteSettlementObserverTestHarness(t, nil)
		notifier := newCloudAssetsNotifier()
		runtime.assetNotifier = notifier
		otherScope := scope
		otherScope.BindingGeneration++

		err := observer.CreditSettlementObserved(context.Background(), otherScope, "task-01", "settlement-01")
		if code := modelbilling.ErrorCodeOf(err); code != modelbilling.ErrorScopeMismatch {
			t.Fatalf("observer error = %v, code=%q, want %s", err, code, modelbilling.ErrorScopeMismatch)
		}
		assertDesktopSettlementNotifierClean(t, notifier)
		_, found, loadErr := runtime.assets.LoadBalance(context.Background(), scope)
		if loadErr != nil || found {
			t.Fatalf("balance stored on scope mismatch: found=%t, err=%v", found, loadErr)
		}
	})

	t.Run("balance cache failure", func(t *testing.T) {
		root := t.TempDir()
		observer, runtime, _, scope := newDesktopRemoteSettlementObserverTestHarnessAtRoot(t, root, nil)
		notifier := newCloudAssetsNotifier()
		runtime.assetNotifier = notifier
		actor := userctx.ActorContext{ProfileID: scope.ProfileID}
		assetPath := filepath.Join(
			desktopCloudTestProfilePathProvider(root)(actor).MemoryUserDir(),
			"cloud",
			"v1",
			"asset_snapshots.json",
		)
		if err := os.MkdirAll(filepath.Dir(assetPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(assetPath, []byte("{not-json"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := observer.CreditSettlementObserved(context.Background(), scope, "task-01", "settlement-01"); err == nil {
			t.Fatal("observer succeeded with a corrupt balance cache")
		}
		assertDesktopSettlementNotifierClean(t, notifier)
	})
}

func TestDesktopRemoteSettlementObserverUnavailableNotifierDoesNotChangeSettlement(t *testing.T) {
	tests := []struct {
		name     string
		notifier func() *cloudAssetsNotifier
	}{
		{name: "nil", notifier: func() *cloudAssetsNotifier { return nil }},
		{name: "not started", notifier: newCloudAssetsNotifier},
		{name: "not bound", notifier: func() *cloudAssetsNotifier {
			notifier := newCloudAssetsNotifier()
			notifier.start()
			return notifier
		}},
		{name: "closed", notifier: func() *cloudAssetsNotifier {
			notifier := newCloudAssetsNotifier()
			notifier.close()
			return notifier
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observer, runtime, _, scope := newDesktopRemoteSettlementObserverTestHarness(t, nil)
			notifier := test.notifier()
			runtime.assetNotifier = notifier
			if notifier != nil {
				t.Cleanup(notifier.close)
			}

			if err := observer.CreditSettlementObserved(context.Background(), scope, "task-01", "settlement-01"); err != nil {
				t.Fatalf("observer error = %v", err)
			}
			stored, found, err := runtime.assets.LoadBalance(context.Background(), scope)
			if err != nil || !found || stored.Version != 2 {
				t.Fatalf("stored balance = %#v, found=%t, err=%v", stored, found, err)
			}
		})
	}
}

func TestDesktopRemoteSettlementObserverAllowsRepeatedWakeups(t *testing.T) {
	observer, runtime, client, scope := newDesktopRemoteSettlementObserverTestHarness(t, nil)
	notifier := newCloudAssetsNotifier()
	deliveries := make(chan CloudAssetsChangedView, 2)
	notifier.bind(func(view CloudAssetsChangedView) { deliveries <- view })
	notifier.start()
	t.Cleanup(notifier.close)
	runtime.assetNotifier = notifier

	for index := range 2 {
		if err := observer.CreditSettlementObserved(context.Background(), scope, "task-01", "settlement-01"); err != nil {
			t.Fatalf("observer call %d: %v", index+1, err)
		}
		assertDesktopSettlementBalanceWakeup(t, deliveries)
	}
	if client.currentUserCalls() != 2 {
		t.Fatalf("CurrentUser calls = %d, want 2", client.currentUserCalls())
	}
}

type desktopSettlementSessionClient struct {
	mu       sync.Mutex
	snapshot cloud.CurrentUserSnapshot
	err      error
	calls    int
}

type desktopStaticQuotaSettlementStore struct {
	result modelbilling.QuotaSettlementResult
	found  bool
	err    error
}

func (s desktopStaticQuotaSettlementStore) SaveQuotaSettlement(context.Context, modelbilling.TaskScope, modelbilling.QuotaSettlementResult, time.Time) error {
	return nil
}

func (s desktopStaticQuotaSettlementStore) CurrentQuotaSettlement(context.Context, modelbilling.TaskScope, string) (modelbilling.QuotaSettlementResult, bool, error) {
	return s.result, s.found, s.err
}

func (c *desktopSettlementSessionClient) Login(context.Context, cloud.LoginRequest) (cloud.AuthenticationResult, error) {
	return cloud.AuthenticationResult{}, errors.New("unexpected login")
}

func (c *desktopSettlementSessionClient) Refresh(context.Context, string) (cloud.RemoteSession, error) {
	return cloud.RemoteSession{}, errors.New("unexpected refresh")
}

func (c *desktopSettlementSessionClient) Logout(context.Context, string) error {
	return errors.New("unexpected logout")
}

func (c *desktopSettlementSessionClient) CurrentUser(context.Context, string) (cloud.CurrentUserSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.snapshot, c.err
}

func (c *desktopSettlementSessionClient) currentUserCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func newDesktopRemoteSettlementObserverTestHarness(
	t *testing.T,
	remoteErr error,
) (desktopRemoteSettlementObserver, *cloudRuntime, *desktopSettlementSessionClient, cloud.BindingScope) {
	t.Helper()
	return newDesktopRemoteSettlementObserverTestHarnessAtRoot(t, t.TempDir(), remoteErr)
}

func newDesktopRemoteSettlementObserverTestHarnessAtRoot(
	t *testing.T,
	root string,
	remoteErr error,
) (desktopRemoteSettlementObserver, *cloudRuntime, *desktopSettlementSessionClient, cloud.BindingScope) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	actor := userctx.ActorContext{ActorID: "local-settlement", ProfileID: "settlement-profile"}.Normalize()
	bindings := cloud.NewBindingRepository()
	binding, err := bindings.ConfirmBinding(ctx, actor, "settlement-user")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	store := cloud.NewMemorySessionStore()
	sessions := cloud.NewProfileSessionRepository(bindings, store)
	if err := sessions.Save(ctx, actor, cloud.Session{
		Scope:                 scope,
		State:                 cloud.SessionAuthenticated,
		AccessToken:           "access-settlement",
		RefreshToken:          "refresh-settlement",
		DeviceID:              "device-settlement",
		AccessTokenExpiresAt:  now.Add(time.Hour),
		RefreshTokenExpiresAt: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	client := &desktopSettlementSessionClient{
		err: remoteErr,
		snapshot: cloud.CurrentUserSnapshot{
			User: cloud.CloudUser{CloudUserID: scope.CloudUserID, DisplayName: "Settlement User"},
			Balance: cloud.BalanceSnapshot{
				TotalBalance: "90.0000", FreeBalance: "10.0000", SubscriptionBalance: "80.0000",
				AvailableBalance: "90.0000", FreeAvailableBalance: "10.0000", SubscriptionAvailableBalance: "80.0000",
				FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000",
				Version: 2,
			},
			Subscription: cloud.CurrentSubscription{Plan: "plus", Status: cloud.SubscriptionActive, PeriodEnd: now.AddDate(0, 1, 0), Version: 3},
			ServerTime:   now,
		},
	}
	executor, err := cloud.NewAuthenticatedRequestExecutor(store, client, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	remoteAccount, err := cloud.NewRemoteAccountApplication(bindings, sessions, client, executor)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &cloudRuntime{
		remoteAccount: remoteAccount,
		assets:        cloud.NewDurableAssetSnapshotRepository(desktopCloudTestProfilePathProvider(root)),
	}
	return desktopRemoteSettlementObserver{runtime: runtime, actor: actor}, runtime, client, scope
}

func assertDesktopSettlementBalanceWakeup(t *testing.T, deliveries <-chan CloudAssetsChangedView) {
	t.Helper()
	select {
	case view := <-deliveries:
		if !view.BalanceChanged || view.SubscriptionChanged {
			t.Fatalf("asset wakeup = %#v, want balance only", view)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for settlement balance wakeup")
	}
}

func assertDesktopSettlementNotifierClean(t *testing.T, notifier *cloudAssetsNotifier) {
	t.Helper()
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if notifier.pending || notifier.dirty.BalanceChanged || notifier.dirty.SubscriptionChanged {
		t.Fatalf("unexpected asset notification state: pending=%t dirty=%#v", notifier.pending, notifier.dirty)
	}
}

var _ cloud.CloudSessionClient = (*desktopSettlementSessionClient)(nil)
