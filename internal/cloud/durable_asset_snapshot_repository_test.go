package cloud

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestDurableAssetSnapshotRepositoryRestoresAndIsolatesCompleteScope(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	repository := NewDurableAssetSnapshotRepository(provider)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	scope := balanceScope(t, actor, "usr-a", 1)
	balance := balanceState(scope, 9, "777.0000").Balance
	plans := []SubscriptionPlan{{Plan: "plus", DisplayName: "Plus", PriceAmount: "19.90", Currency: "CNY", CreditAmount: "20000.0000", PeriodDays: 31, Enabled: true, SortOrder: 2, ConfigVersion: "catalog-20260715-user-v1"}}
	view := subscriptionView(plans, subscriptionActive(scope, "plus"))
	if got, err := repository.StoreBalance(ctx, scope, balance); err != nil || got != balance {
		t.Fatalf("StoreBalance = %#v, %v", got, err)
	}
	lower := balanceState(scope, 8, "999.0000").Balance
	if got, err := NewDurableAssetSnapshotRepository(provider).StoreBalance(ctx, scope, lower); err != nil || got != balance {
		t.Fatalf("lower StoreBalance = %#v, %v", got, err)
	}
	if err := repository.StoreSubscription(ctx, scope, view); err != nil {
		t.Fatal(err)
	}

	restarted := NewDurableAssetSnapshotRepository(provider)
	if got, ok, err := restarted.LoadBalance(ctx, scope); err != nil || !ok || got != balance {
		t.Fatalf("LoadBalance = %#v, %v, %v", got, ok, err)
	}
	if got, ok, err := restarted.LoadSubscription(ctx, scope); err != nil || !ok || !reflect.DeepEqual(got, view) {
		t.Fatalf("LoadSubscription = %#v, %v, %v", got, ok, err)
	}
	for _, other := range []BindingScope{
		balanceScope(t, userctx.ActorContext{ActorID: "local", ProfileID: "profile-b"}, "usr-a", 1),
		balanceScope(t, actor, "usr-b", 1),
		balanceScope(t, actor, "usr-a", 2),
	} {
		if _, ok, err := restarted.LoadBalance(ctx, other); err != nil || ok {
			t.Fatalf("old snapshot visible to scope %#v: ok=%v err=%v", other, ok, err)
		}
	}
}

func TestDurableAssetSnapshotRepositoryLoadsLegacyNumericConfigVersion(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "asset_snapshots.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"subscription":{"plans":[{"plan":"plus","display_name":"Plus","price_amount":"19.90","currency":"CNY","credit_amount":"100.0000","period_days":30,"enabled":true,"sort_order":1,"config_version":1}],"subscription":{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"plan":"plus","status":"active","period_end":"2026-08-11T00:00:00Z","updated_at":"2026-07-11T00:00:00Z"}}}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr-a", 1)
	view, ok, err := NewDurableAssetSnapshotRepository(provider).LoadSubscription(ctx, scope)
	if err != nil || !ok {
		t.Fatalf("LoadSubscription = %#v, %v, %v", view, ok, err)
	}
	if len(view.Plans) != 1 || view.Plans[0].ConfigVersion != "1" {
		t.Fatalf("legacy config version = %#v, want 1", view.Plans)
	}
}

func TestDurableAssetSnapshotRepositoryRejectsTampering(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "asset_snapshots.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	scope := balanceScope(t, actor, "usr-a", 1)
	validBalance := `{"total_balance":"1","free_balance":"9","subscription_balance":"8","available_balance":"7","free_available_balance":"6","subscription_available_balance":"5","frozen_balance":"4","free_frozen_balance":"3","subscription_frozen_balance":"2","version":1}`
	validEntry := `{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"balance":` + validBalance + `}`
	validSubscription := `{"plans":[{"plan":"plus","display_name":"Plus","price_amount":"19.90","currency":"CNY","credit_amount":"100.0000","period_days":30,"enabled":true,"sort_order":1,"config_version":1}],"subscription":{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-b","binding_generation":1},"plan":"plus","status":"active","period_end":"2026-08-11T00:00:00Z","updated_at":"2026-07-11T00:00:00Z"}}`
	cases := map[string]string{
		"unknown field":            `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[],"extra":true}`,
		"trailing JSON":            `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[]} {}`,
		"wrong schema":             `{"schema":"wrong","version":1,"profile_id":"profile-a","entries":[]}`,
		"wrong version":            `{"schema":"reasonix.cloud.asset_snapshots","version":2,"profile_id":"profile-a","entries":[]}`,
		"envelope profile":         `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-b","entries":[]}`,
		"entry profile":            `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-b","cloud_user_id":"usr-a","binding_generation":1},"balance":` + validBalance + `}]}`,
		"subscription entry scope": `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"subscription":` + validSubscription + `}]}`,
		"empty cloud user":         `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-a","cloud_user_id":"","binding_generation":1},"balance":` + validBalance + `}]}`,
		"zero generation":          `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":0},"balance":` + validBalance + `}]}`,
		"duplicate scope":          `{"schema":"reasonix.cloud.asset_snapshots","version":1,"profile_id":"profile-a","entries":[` + validEntry + `,` + validEntry + `]}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := NewDurableAssetSnapshotRepository(provider).LoadBalance(ctx, scope); err == nil {
				t.Fatal("tampered durable state was treated as missing")
			}
		})
	}
}

func TestDurableAssetSnapshotRepositoryConcurrentScopesDoNotLoseUpdates(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	const count = 12
	scopes := make([]BindingScope, 0, count)
	for i := 1; i <= count; i++ {
		scopes = append(scopes, balanceScope(t, actor, "usr-"+string(rune('a'+i-1)), uint64(i)))
	}
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i, scope := range scopes {
		i, scope := i, scope
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NewDurableAssetSnapshotRepository(provider).StoreBalance(ctx, scope, balanceState(scope, uint64(i+1), "1.0000").Balance)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, scope := range scopes {
		if _, ok, err := NewDurableAssetSnapshotRepository(provider).LoadBalance(ctx, scope); err != nil || !ok {
			t.Fatalf("scope %d missing: ok=%v err=%v", i+1, ok, err)
		}
	}
}

func TestDurableAssetSnapshotRepositoryConcurrentSameScopeKeepsHighestVersion(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	scope := balanceScope(t, actor, "usr-a", 1)
	provider := testProfilePathProvider(t.TempDir())
	const count = 16
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for version := 1; version <= count; version++ {
		version := version
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NewDurableAssetSnapshotRepository(provider).StoreBalance(ctx, scope, balanceState(scope, uint64(version), "1.0000").Balance)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := NewDurableAssetSnapshotRepository(provider).LoadBalance(ctx, scope)
	if err != nil || !ok || got.Version != count {
		t.Fatalf("highest balance = %#v, %v, %v", got, ok, err)
	}
}

func TestMemoryAssetSnapshotRepositoryPreservesCloudAuthority(t *testing.T) {
	ctx := context.Background()
	scope := balanceScope(t, userctx.ActorContext{ActorID: "local", ProfileID: "default"}, "usr-a", 1)
	repository := NewMemoryAssetSnapshotRepository()
	higher := balanceState(scope, 10, "333.0000").Balance
	lower := balanceState(scope, 9, "999.0000").Balance
	if _, err := repository.StoreBalance(ctx, scope, higher); err != nil {
		t.Fatal(err)
	}
	got, err := repository.StoreBalance(ctx, scope, lower)
	if err != nil || got != higher || got.TotalBalance == got.FreeBalance+got.SubscriptionBalance {
		t.Fatalf("cloud balance authority lost: %#v, %v", got, err)
	}

	view := SubscriptionView{Plans: []SubscriptionPlan{{Plan: "annual", DisplayName: "Annual", PriceAmount: "88.88", Currency: "CNY", CreditAmount: "65432.1000", PeriodDays: 366, Enabled: false, SortOrder: 9, ConfigVersion: "42"}}, Subscription: Subscription{Scope: scope, Plan: "annual", Status: SubscriptionActive, PeriodEnd: time.Date(2027, 7, 11, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)}}
	if err := repository.StoreSubscription(ctx, scope, view); err != nil {
		t.Fatal(err)
	}
	gotView, ok, err := repository.LoadSubscription(ctx, scope)
	if err != nil || !ok || !reflect.DeepEqual(gotView, view) {
		t.Fatalf("subscription authority lost: %#v, %v, %v", gotView, ok, err)
	}
}

func TestAssetSnapshotRepositoriesClearOnlyExactScope(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	first := balanceScope(t, actor, "usr-a", 1)
	second := balanceScope(t, actor, "usr-b", 2)
	for name, repository := range map[string]AssetSnapshotRepository{
		"memory":  NewMemoryAssetSnapshotRepository(),
		"durable": NewDurableAssetSnapshotRepository(testProfilePathProvider(t.TempDir())),
	} {
		t.Run(name, func(t *testing.T) {
			for _, scope := range []BindingScope{first, second} {
				balance := balanceState(scope, scope.BindingGeneration, "10.0000").Balance
				if _, err := repository.StoreBalance(ctx, scope, balance); err != nil {
					t.Fatal(err)
				}
				view := subscriptionView([]SubscriptionPlan{subscriptionPlan("plus", "19.90")}, subscriptionActive(scope, "plus"))
				if err := repository.StoreSubscription(ctx, scope, view); err != nil {
					t.Fatal(err)
				}
			}
			if err := repository.Clear(ctx, first); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := repository.LoadBalance(ctx, first); err != nil || ok {
				t.Fatalf("cleared balance present: ok=%v err=%v", ok, err)
			}
			if _, ok, err := repository.LoadSubscription(ctx, first); err != nil || ok {
				t.Fatalf("cleared subscription present: ok=%v err=%v", ok, err)
			}
			if _, ok, err := repository.LoadBalance(ctx, second); err != nil || !ok {
				t.Fatalf("other balance removed: ok=%v err=%v", ok, err)
			}
			if _, ok, err := repository.LoadSubscription(ctx, second); err != nil || !ok {
				t.Fatalf("other subscription removed: ok=%v err=%v", ok, err)
			}
		})
	}
}
