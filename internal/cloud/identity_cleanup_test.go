package cloud

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestIdentityCleanupClearsOnlyTheProvidedBindingScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := BindingScope{ProfileID: "profile-a", CloudUserID: "usr-a", BindingGeneration: 2}
	sessions := NewMemorySessionStore()
	if err := sessions.Save(ctx, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access-a", RefreshToken: "refresh-a"}); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	cursors := &recordingScopedStore{}
	snapshots := &recordingScopedStore{}
	cleanup := NewIdentityCleanup(sessions, cursors, snapshots)

	if err := cleanup.Clear(ctx, scope); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, ok, err := sessions.Load(ctx, scope); err != nil || ok {
		t.Fatalf("Load after clear = (_, %t, %v), want absent session", ok, err)
	}
	if !reflect.DeepEqual(cursors.scopes, []BindingScope{scope}) || !reflect.DeepEqual(snapshots.scopes, []BindingScope{scope}) {
		t.Fatalf("cleared scopes = cursors:%+v snapshots:%+v, want only %+v", cursors.scopes, snapshots.scopes, scope)
	}
}

func TestIdentityCleanupPreservesOtherScopePaymentAndTaskRecords(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	scope := bundle.PaymentCreates[0].Result.Order.Scope
	other := BindingScope{ProfileID: scope.ProfileID, CloudUserID: "usr-other", BindingGeneration: scope.BindingGeneration + 1}

	sessions := NewMemorySessionStore()
	if err := sessions.Save(ctx, Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	events := NewMemoryEventStateRepository()
	for _, candidate := range []BindingScope{scope, other} {
		if err := events.CommitApplied(ctx, candidate, eventStateEvent("event-"+candidate.CloudUserID, EventBalanceUpdated, candidate.CloudUserID, 1)); err != nil {
			t.Fatal(err)
		}
	}
	assets := NewMemoryAssetSnapshotRepository()
	primary := accountSnapshotForScope(t, bundle, scope)
	for _, candidate := range []BindingScope{scope, other} {
		if _, err := assets.StoreBalance(ctx, candidate, primary.Balance); err != nil {
			t.Fatal(err)
		}
		subscription := primary.Subscription
		subscription.Scope = candidate
		if err := assets.StoreSubscription(ctx, candidate, subscriptionView(bundle.PlanCases, subscription)); err != nil {
			t.Fatal(err)
		}
	}

	payments := NewMemoryPaymentOrderRepository()
	payment, err := NewPaymentOrderRecord(scope, bundle.PaymentCreates[0].Result, bundle.PaymentCreates[0].Result.Order.OrderType, "cleanup-payment-idem")
	if err != nil {
		t.Fatal(err)
	}
	if err := payments.Save(ctx, payment); err != nil {
		t.Fatal(err)
	}
	tasks := NewMemoryTaskRecoveryStore()
	task := TaskRecoveryRecord{Scope: scope, TaskID: "cleanup-task", FreezeID: "cleanup-freeze", IdempotencyKey: "cleanup-task-idem"}
	if err := tasks.Save(ctx, task); err != nil {
		t.Fatal(err)
	}

	cleanup := NewIdentityCleanup(sessions, events, assets)
	if err := cleanup.Clear(ctx, scope); err != nil {
		t.Fatal(err)
	}
	if _, found, err := events.Load(ctx, scope); err != nil || found {
		t.Fatalf("target event checkpoint remained: found=%v err=%v", found, err)
	}
	if _, found, err := events.Load(ctx, other); err != nil || !found {
		t.Fatalf("other event checkpoint was removed: found=%v err=%v", found, err)
	}
	if _, found, err := assets.LoadBalance(ctx, scope); err != nil || found {
		t.Fatalf("target asset remained: found=%v err=%v", found, err)
	}
	if _, found, err := assets.LoadBalance(ctx, other); err != nil || !found {
		t.Fatalf("other asset was removed: found=%v err=%v", found, err)
	}
	if _, found, err := payments.Get(ctx, scope, payment.Result.Order.OrderID); err != nil || !found {
		t.Fatalf("payment record was removed: found=%v err=%v", found, err)
	}
	if records, err := tasks.Load(ctx, scope); err != nil || len(records) != 1 || records[0] != task {
		t.Fatalf("task records after cleanup = %#v, %v", records, err)
	}
}

func TestIdentityCleanupRejectsMissingDependenciesCancelledContextAndInvalidScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := BindingScope{ProfileID: "profile-a", CloudUserID: "usr-a", BindingGeneration: 1}
	for _, dependency := range []string{"sessions", "cursors", "snapshots"} {
		t.Run(dependency, func(t *testing.T) {
			calls := []string{}
			sessions := &orderedSessionStore{calls: &calls}
			cursors := &orderedScopedStore{name: "cursor", calls: &calls}
			snapshots := &orderedScopedStore{name: "snapshot", calls: &calls}
			switch dependency {
			case "sessions":
				sessions = nil
			case "cursors":
				cursors = nil
			case "snapshots":
				snapshots = nil
			}
			cleanup := NewIdentityCleanup(sessions, cursors, snapshots)
			if err := cleanup.Clear(ctx, scope); err == nil {
				t.Fatalf("Clear without %s succeeded, want configuration error", dependency)
			}
			if len(calls) != 0 {
				t.Fatalf("calls after missing %s = %v, want none", dependency, calls)
			}
		})
	}

	calls := []string{}
	sessions := &orderedSessionStore{calls: &calls}
	cleanup := NewIdentityCleanup(sessions, &orderedScopedStore{name: "cursor", calls: &calls}, &orderedScopedStore{name: "snapshot", calls: &calls})
	if err := cleanup.Clear(ctx, BindingScope{}); err == nil {
		t.Fatal("Clear invalid scope succeeded, want validation error")
	}
	if len(calls) != 0 {
		t.Fatalf("calls after invalid scope = %v, want none", calls)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := cleanup.Clear(cancelled, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("Clear cancelled context error = %v, want context.Canceled", err)
	}
	if len(calls) != 0 {
		t.Fatalf("calls after cancelled context = %v, want none", calls)
	}
}

func TestIdentityCleanupStopsAtTheFirstFailedDependencyInOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scope := BindingScope{ProfileID: "profile-a", CloudUserID: "usr-a", BindingGeneration: 1}

	t.Run("session", func(t *testing.T) {
		calls := []string{}
		errExpected := errors.New("session clear failed")
		cleanup := NewIdentityCleanup(
			&orderedSessionStore{calls: &calls, clearErr: errExpected},
			&orderedScopedStore{name: "cursor", calls: &calls},
			&orderedScopedStore{name: "snapshot", calls: &calls},
		)
		if err := cleanup.Clear(ctx, scope); !errors.Is(err, errExpected) {
			t.Fatalf("Clear error = %v, want %v", err, errExpected)
		}
		if want := []string{"session"}; !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	})

	t.Run("cursor", func(t *testing.T) {
		calls := []string{}
		errExpected := errors.New("cursor clear failed")
		cleanup := NewIdentityCleanup(
			&orderedSessionStore{calls: &calls},
			&orderedScopedStore{name: "cursor", calls: &calls, clearErr: errExpected},
			&orderedScopedStore{name: "snapshot", calls: &calls},
		)
		if err := cleanup.Clear(ctx, scope); !errors.Is(err, errExpected) {
			t.Fatalf("Clear error = %v, want %v", err, errExpected)
		}
		if want := []string{"session", "cursor"}; !reflect.DeepEqual(calls, want) {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	})
}

type recordingScopedStore struct {
	scopes []BindingScope
}

func (s *recordingScopedStore) Clear(_ context.Context, scope BindingScope) error {
	s.scopes = append(s.scopes, scope)
	return nil
}

type orderedSessionStore struct {
	calls    *[]string
	clearErr error
}

func (s *orderedSessionStore) Save(context.Context, Session) error { return nil }

func (s *orderedSessionStore) Load(context.Context, BindingScope) (Session, bool, error) {
	return Session{}, false, nil
}

func (s *orderedSessionStore) Clear(_ context.Context, _ BindingScope) error {
	*s.calls = append(*s.calls, "session")
	return s.clearErr
}

type orderedScopedStore struct {
	name     string
	calls    *[]string
	clearErr error
}

func (s *orderedScopedStore) Clear(_ context.Context, _ BindingScope) error {
	*s.calls = append(*s.calls, s.name)
	return s.clearErr
}
