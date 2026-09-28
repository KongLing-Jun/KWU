package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/config"
	"kwu/internal/userctx"
)

func TestDurablePaymentOrderRepositoryRestoresAndIsolatesCompleteScope(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	scope := paymentOrderRepositoryScope(actor.ProfileID, "usr-a", 1)
	records := []PaymentOrderRecord{
		paymentOrderRepositoryRecord(t, scope, "pay-b", "idem-b", PaymentPaidVerified),
		paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending),
	}
	repository := NewDurablePaymentOrderRepository(provider)
	for _, record := range records {
		if err := repository.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	restarted := NewDurablePaymentOrderRepository(provider)
	if got, ok, err := restarted.Get(ctx, scope, "pay-a"); err != nil || !ok || got != records[1] {
		t.Fatalf("Get = %#v, %v, %v", got, ok, err)
	}
	if got, ok, err := restarted.FindByIdempotencyKey(ctx, scope, "idem-b"); err != nil || !ok || got != records[0] {
		t.Fatalf("FindByIdempotencyKey = %#v, %v, %v", got, ok, err)
	}
	if got, err := restarted.List(ctx, scope); err != nil || !reflect.DeepEqual(got, []PaymentOrderRecord{records[1], records[0]}) {
		t.Fatalf("List = %#v, %v", got, err)
	}
	for _, other := range []BindingScope{
		paymentOrderRepositoryScope("profile-b", "usr-a", 1),
		paymentOrderRepositoryScope("profile-a", "usr-b", 1),
		paymentOrderRepositoryScope("profile-a", "usr-a", 2),
	} {
		if got, err := restarted.List(ctx, other); err != nil || len(got) != 0 {
			t.Fatalf("records visible to %#v: %#v, %v", other, got, err)
		}
	}
}

func TestDurablePaymentOrderRepositoryUsesSeparateProfilePaths(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actors := []userctx.ActorContext{{ActorID: "local", ProfileID: "profile-a"}, {ActorID: "local", ProfileID: "profile-b"}}
	for _, actor := range actors {
		scope := paymentOrderRepositoryScope(actor.ProfileID, "usr-a", 1)
		if err := NewDurablePaymentOrderRepository(provider).Save(ctx, paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending)); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(provider(actors[0]).MemoryUserDir(), "cloud", "v1", "payment_orders.json")
	second := filepath.Join(provider(actors[1]).MemoryUserDir(), "cloud", "v1", "payment_orders.json")
	if first == second {
		t.Fatalf("profile paths are equal: %q", first)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("profile file %q: %v", path, err)
		}
	}
}

func TestDurablePaymentOrderRepositoryHonorsCancellationBeforePathIO(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	record := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentGranted)
	pathCalls := 0
	repository := NewDurablePaymentOrderRepository(func(actor userctx.ActorContext) config.ProfilePathResolver {
		pathCalls++
		return testProfilePathProvider(t.TempDir())(actor)
	})
	if err := repository.Save(ctx, record); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save error = %v", err)
	}
	if _, _, err := repository.Get(ctx, scope, "pay-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get error = %v", err)
	}
	if _, _, err := repository.FindByIdempotencyKey(ctx, scope, "idem-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Find error = %v", err)
	}
	if _, err := repository.List(ctx, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("List error = %v", err)
	}
	if err := repository.MarkEntitlementsRefreshed(ctx, scope, "pay-a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Mark error = %v", err)
	}
	if pathCalls != 0 {
		t.Fatalf("path provider called %d times", pathCalls)
	}
}

func TestDurablePaymentOrderRepositoryPreservesIndexesAndMarker(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	repository := NewDurablePaymentOrderRepository(provider)
	record := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending)
	if err := repository.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	updated := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentGranted)
	if err := NewDurablePaymentOrderRepository(provider).Save(ctx, updated); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, paymentOrderRepositoryRecord(t, scope, "pay-b", "idem-a", PaymentPending)); err == nil {
		t.Fatal("accepted idempotency key conflict")
	}
	if err := repository.Save(ctx, paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-b", PaymentPending)); err == nil {
		t.Fatal("accepted order ID conflict")
	}
	if err := NewDurablePaymentOrderRepository(provider).MarkEntitlementsRefreshed(ctx, scope, "pay-a"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := NewDurablePaymentOrderRepository(provider).Get(ctx, scope, "pay-a")
	if err != nil || !ok || !got.EntitlementsRefreshed || got.GrantSequence != 1 || got.Result != updated.Result {
		t.Fatalf("marked record = %#v, %v, %v", got, ok, err)
	}
	if err := repository.Save(ctx, updated); err != nil {
		t.Fatalf("repeated granted Save: %v", err)
	}
	got, ok, err = NewDurablePaymentOrderRepository(provider).Get(ctx, scope, "pay-a")
	if err != nil || !ok || !got.EntitlementsRefreshed || got.GrantSequence != 1 {
		t.Fatalf("stale granted Save cleared monotonic state = %#v, %v, %v", got, ok, err)
	}
	pending := paymentOrderRepositoryRecord(t, scope, "pay-pending", "idem-pending", PaymentPending)
	if err := repository.Save(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkEntitlementsRefreshed(ctx, scope, "pay-pending"); err == nil {
		t.Fatal("marked non-granted order")
	}
	transient := paymentOrderRepositoryRecordUnchecked(scope, "pay-transient", "idem-transient", PaymentClientPaidUnverified)
	if err := repository.Save(ctx, transient); err == nil {
		t.Fatal("persisted client_paid_unverified")
	}
}

func TestDurablePaymentOrderRepositoryConcurrentInstancesRemainAtomic(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	const count = 12
	records := make([]PaymentOrderRecord, count)
	for i := range records {
		records[i] = paymentOrderRepositoryRecord(t, scope, "pay-"+string(rune('a'+i)), "idem-"+string(rune('a'+i)), PaymentPending)
	}
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for _, record := range records {
		record := record
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- NewDurablePaymentOrderRepository(provider).Save(ctx, record)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got, err := NewDurablePaymentOrderRepository(provider).List(ctx, scope); err != nil || len(got) != count {
		t.Fatalf("List = %d records, %v", len(got), err)
	}

	conflicts := []PaymentOrderRecord{
		paymentOrderRepositoryRecord(t, scope, "pay-conflict-a", "idem-conflict", PaymentPending),
		paymentOrderRepositoryRecord(t, scope, "pay-conflict-b", "idem-conflict", PaymentPending),
	}
	start := make(chan struct{})
	errs = make(chan error, 2)
	for _, record := range conflicts {
		record := record
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- NewDurablePaymentOrderRepository(provider).Save(ctx, record)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	var successes int
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("conflicting successes = %d", successes)
	}
	if _, err := NewDurablePaymentOrderRepository(provider).List(ctx, scope); err != nil {
		t.Fatalf("file unreadable after conflict race: %v", err)
	}
}

func TestDurablePaymentOrderRepositoryRejectsStaleVerifyingAfterGrantedMarker(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	first := NewDurablePaymentOrderRepository(provider)
	second := NewDurablePaymentOrderRepository(provider)
	pending := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending)
	if err := first.Save(ctx, pending); err != nil {
		t.Fatal(err)
	}
	stale, found, err := second.Get(ctx, scope, "pay-a")
	if err != nil || !found {
		t.Fatalf("second read pending = %#v, %v, %v", stale, found, err)
	}
	granted := pending
	granted.Result = paymentOrderRepositoryResult(scope, "pay-a", PaymentGranted)
	if err := first.Save(ctx, granted); err != nil {
		t.Fatal(err)
	}
	if err := first.MarkEntitlementsRefreshed(ctx, scope, "pay-a"); err != nil {
		t.Fatal(err)
	}
	stale.Result = paymentOrderRepositoryResult(scope, "pay-a", PaymentVerifying)
	if err := second.Save(ctx, stale); err == nil {
		t.Fatal("stale verifying save replaced granted order")
	}
	stored, found, err := first.Get(ctx, scope, "pay-a")
	if err != nil || !found || stored.Result.Order.Status != PaymentGranted || !stored.EntitlementsRefreshed {
		t.Fatalf("stored granted record = %#v, %v, %v", stored, found, err)
	}
}

func TestDurablePaymentOrderRepositoryAssignsGrantSequenceAtomicallyAcrossInstances(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	first := NewDurablePaymentOrderRepository(provider)
	second := NewDurablePaymentOrderRepository(provider)
	firstPending := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending)
	secondPending := paymentOrderRepositoryRecord(t, scope, "pay-b", "idem-b", PaymentPending)
	if err := first.Save(ctx, firstPending); err != nil {
		t.Fatal(err)
	}
	if err := second.Save(ctx, secondPending); err != nil {
		t.Fatal(err)
	}
	firstGranted := firstPending
	firstGranted.Result = paymentOrderRepositoryResult(scope, "pay-a", PaymentGranted)
	secondGranted := secondPending
	secondGranted.Result = paymentOrderRepositoryResult(scope, "pay-b", PaymentGranted)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for _, update := range []struct {
		repository *DurablePaymentOrderRepository
		record     PaymentOrderRecord
	}{
		{first, firstGranted},
		{second, secondGranted},
	} {
		update := update
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errs <- update.repository.Save(ctx, update.record)
		}()
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	restarted := NewDurablePaymentOrderRepository(provider)
	records, err := restarted.List(ctx, scope)
	if err != nil || len(records) != 2 {
		t.Fatalf("restarted records = %#v, %v", records, err)
	}
	sequences := map[uint64]bool{}
	for _, record := range records {
		sequences[record.GrantSequence] = true
	}
	if len(sequences) != 2 || !sequences[1] || !sequences[2] {
		t.Fatalf("atomic grant sequences = %#v, want {1,2}", sequences)
	}
}

func TestDurablePaymentOrderRepositoryKeepsTerminalStatusesStickyAcrossInstances(t *testing.T) {
	ctx := context.Background()
	for _, terminal := range []PaymentStatus{
		PaymentGranted,
		PaymentClosed,
		PaymentFailed,
		PaymentTimeout,
		PaymentVerifyFailedManualRequired,
	} {
		t.Run(string(terminal), func(t *testing.T) {
			provider := testProfilePathProvider(t.TempDir())
			scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
			first := NewDurablePaymentOrderRepository(provider)
			second := NewDurablePaymentOrderRepository(provider)
			pending := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending)
			if err := first.Save(ctx, pending); err != nil {
				t.Fatal(err)
			}
			stale, found, err := second.Get(ctx, scope, "pay-a")
			if err != nil || !found {
				t.Fatalf("second read pending = %#v, %v, %v", stale, found, err)
			}
			terminalRecord := pending
			terminalRecord.Result = paymentOrderRepositoryResult(scope, "pay-a", terminal)
			if err := first.Save(ctx, terminalRecord); err != nil {
				t.Fatal(err)
			}
			if err := second.Save(ctx, stale); err == nil {
				t.Fatalf("stale pending save replaced terminal %s", terminal)
			}
			stored, found, err := first.Get(ctx, scope, "pay-a")
			if err != nil || !found || stored.Result.Order.Status != terminal {
				t.Fatalf("stored terminal record = %#v, %v, %v", stored, found, err)
			}
		})
	}
}

func TestDurablePaymentOrderRepositoryRejectsTampering(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "payment_orders.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	record := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-a", PaymentPending)
	valid := durablePaymentOrderTestJSON(t, record)
	cases := map[string]string{
		"unknown field":      valid[:len(valid)-1] + `,"extra":true}`,
		"trailing JSON":      valid + ` {}`,
		"wrong schema":       replaceOnce(t, valid, `reasonix.cloud.payment_orders`, `wrong`),
		"wrong version":      replaceOnce(t, valid, `"version":1`, `"version":3`),
		"envelope profile":   replaceOnce(t, valid, `"profile_id":"profile-a"`, `"profile_id":"profile-b"`),
		"entry scope":        replaceNth(t, valid, `"cloud_user_id":"usr-a"`, `"cloud_user_id":"usr-b"`, 1),
		"record scope":       replaceNth(t, valid, `"cloud_user_id":"usr-a"`, `"cloud_user_id":"usr-b"`, 2),
		"result order scope": replaceNth(t, valid, `"cloud_user_id":"usr-a"`, `"cloud_user_id":"usr-b"`, 3),
		"entry order ID":     replaceOnce(t, valid, `"order_id":"pay-a"`, `"order_id":"wrong"`),
		"entry idempotency":  replaceOnce(t, valid, `"idempotency_key":"idem-a"`, `"idempotency_key":"wrong"`),
		"operation mismatch": replaceOnce(t, valid, `"operation":"subscription_purchase"`, `"operation":"subscription_upgrade"`),
		"empty cloud user":   replaceNth(t, valid, `"cloud_user_id":"usr-a"`, `"cloud_user_id":""`, 1),
		"zero generation":    replaceNth(t, valid, `"binding_generation":1`, `"binding_generation":0`, 1),
		"invalid marker":     replaceOnce(t, valid, `"entitlements_refreshed":false`, `"entitlements_refreshed":true`),
		"transient status":   replaceOnce(t, valid, `"status":"payment_pending"`, `"status":"client_paid_unverified"`),
		"duplicate exact":    replaceOnce(t, valid, `]}`, `,`+durablePaymentOrderEntryJSON(t, record)+`]}`),
	}
	orderConflict := paymentOrderRepositoryRecord(t, scope, "pay-a", "idem-b", PaymentPending)
	cases["duplicate order ID"] = replaceOnce(t, valid, `]}`, `,`+durablePaymentOrderEntryJSON(t, orderConflict)+`]}`)
	conflict := paymentOrderRepositoryRecord(t, scope, "pay-b", "idem-a", PaymentPending)
	cases["duplicate idempotency"] = replaceOnce(t, valid, `]}`, `,`+durablePaymentOrderEntryJSON(t, conflict)+`]}`)
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewDurablePaymentOrderRepository(provider).List(ctx, scope); err == nil {
				t.Fatal("tampered state was treated as missing")
			}
		})
	}
}

func TestDurablePaymentOrderRepositoryRejectsTamperedAttemptState(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	repository := NewDurablePaymentOrderRepository(provider)
	scope := paymentOrderRepositoryScope("profile-a", "usr-a", 1)
	candidate := paymentOrderRepositoryAttempt(t, scope, "attempt-01", "plus", PaymentChannelAlipay)
	attempt, created, err := repository.ReserveAttempt(ctx, "", candidate)
	if err != nil || !created {
		t.Fatalf("ReserveAttempt = %#v, %v, %v", attempt, created, err)
	}
	path, err := repository.path(scope.ProfileID)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"derived key": []byte(strings.ReplaceAll(string(valid), attempt.IdempotencyKey, "tampered-key")),
		"legacy version with attempts": []byte(strings.Replace(
			string(valid), `"version":2`, `"version":1`, 1,
		)),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := repository.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay); err == nil {
				t.Fatal("tampered payment attempt was accepted")
			}
		})
	}
}

func TestPaymentApplicationsRestoreDurableGrantedMarkerWithoutRepeatingRefresh(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor, bindings, scope := paymentApplicationBinding(t)
	created := paymentApplicationResult(scope, "order-01", PaymentOrderSubscriptionPurchase, PaymentChannelAlipay, PaymentGranted)
	client := &paymentApplicationClient{createSubscriptionResult: created, getOrder: created.Order}
	firstRefresher := &paymentApplicationRefresher{}
	first := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), firstRefresher)
	if _, err := first.CreateSubscriptionOrder(ctx, actor, "plus", PaymentChannelAlipay); err != nil {
		t.Fatal(err)
	}
	if firstRefresher.calls != 1 {
		t.Fatalf("first entitlement refresh calls = %d", firstRefresher.calls)
	}

	restartedRefresher := &paymentApplicationRefresher{}
	restarted := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), restartedRefresher)
	recovery, err := NewPaymentRecoveryApplication(restarted)
	if err != nil {
		t.Fatal(err)
	}
	results, err := recovery.Restore(ctx, actor)
	if err != nil || len(results) != 1 || results[0].Err != nil || results[0].View.State != PaymentUIStateSucceeded {
		t.Fatalf("Restore = %#v, %v", results, err)
	}
	if restartedRefresher.calls != 0 {
		t.Fatalf("restarted entitlement refresh calls = %d", restartedRefresher.calls)
	}
	marked, ok, err := NewDurablePaymentOrderRepository(provider).Get(ctx, scope, "order-01")
	if err != nil || !ok || !marked.EntitlementsRefreshed {
		t.Fatalf("durable marker = %#v, %v, %v", marked, ok, err)
	}
}

func TestPaymentRecoveryApplicationRestoresDurableExpiredCacheWithoutRemoteQuery(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor, bindings, scope := paymentApplicationBinding(t)
	record := paymentOrderRepositoryRecord(t, scope, "order-01", "idem-01", PaymentPending)
	if err := NewDurablePaymentOrderRepository(provider).Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	client := &paymentApplicationClient{getErr: networkErr}
	restarted := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), nil)
	restarted.now = func() time.Time { return record.Result.Entry.ExpiresAt.Add(time.Second) }
	recovery, err := NewPaymentRecoveryApplication(restarted)
	if err != nil {
		t.Fatal(err)
	}
	results, err := recovery.Restore(ctx, actor)
	if err != nil || len(results) != 1 || results[0].Err != nil || client.getCalls != 0 {
		t.Fatalf("Restore = %#v, %v", results, err)
	}
	view := results[0].View
	if view.Payable || view.Result.Entry.QRCode != "" || view.Result.Entry.CodeURL != "" || !view.Result.Entry.ExpiresAt.Equal(record.Result.Entry.ExpiresAt) {
		t.Fatalf("expired durable view = %#v", view)
	}
	saved, ok, err := NewDurablePaymentOrderRepository(provider).Get(ctx, scope, "order-01")
	if err != nil || !ok || saved != record {
		t.Fatalf("network failure changed durable cache = %#v, %v, %v", saved, ok, err)
	}
}

func TestPaymentApplicationRetriesFailedDurableEntitlementRefreshAfterRestart(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor, bindings, scope := paymentApplicationBinding(t)
	record := paymentOrderRepositoryRecord(t, scope, "order-01", "idem-01", PaymentGranted)
	if err := NewDurablePaymentOrderRepository(provider).Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	refreshErr := errors.New("refresh failed")
	failedRefresher := &paymentApplicationRefresher{err: refreshErr}
	client := &paymentApplicationClient{getOrder: record.Result.Order}
	failed := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), failedRefresher)
	if _, err := failed.RefreshOrder(ctx, actor, "order-01"); !errors.Is(err, refreshErr) {
		t.Fatalf("RefreshOrder error = %v", err)
	}
	saved, ok, err := NewDurablePaymentOrderRepository(provider).Get(ctx, scope, "order-01")
	if err != nil || !ok || saved.EntitlementsRefreshed {
		t.Fatalf("failed refresh marker = %#v, %v, %v", saved, ok, err)
	}

	retryRefresher := &paymentApplicationRefresher{}
	restarted := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), retryRefresher)
	if _, err := restarted.RefreshOrder(ctx, actor, "order-01"); err != nil {
		t.Fatal(err)
	}
	saved, ok, err = NewDurablePaymentOrderRepository(provider).Get(ctx, scope, "order-01")
	if err != nil || !ok || !saved.EntitlementsRefreshed || retryRefresher.calls != 1 {
		t.Fatalf("retry marker = %#v, %v, %v, calls=%d", saved, ok, err, retryRefresher.calls)
	}
}

func TestPaymentRecoveryApplicationDoesNotDiscoverUnsavedDurableOrders(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor, bindings, _ := paymentApplicationBinding(t)
	client := &paymentApplicationClient{}
	restarted := newPaymentApplication(t, bindings, client, NewDurablePaymentOrderRepository(provider), nil)
	recovery, err := NewPaymentRecoveryApplication(restarted)
	if err != nil {
		t.Fatal(err)
	}
	results, err := recovery.Restore(ctx, actor)
	if err != nil || len(results) != 0 || client.getCalls != 0 {
		t.Fatalf("Restore = %#v, %v, get calls=%d", results, err, client.getCalls)
	}
}

func durablePaymentOrderTestJSON(t *testing.T, record PaymentOrderRecord) string {
	t.Helper()
	return `{"schema":"reasonix.cloud.payment_orders","version":1,"profile_id":"profile-a","entries":[` + durablePaymentOrderEntryJSON(t, record) + `]}`
}

func durablePaymentOrderEntryJSON(t *testing.T, record PaymentOrderRecord) string {
	t.Helper()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return `{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1},"order_id":"` + record.Result.Order.OrderID + `","idempotency_key":"` + record.IdempotencyKey + `","record":` + string(data) + `}`
}

func replaceOnce(t *testing.T, value, old, replacement string) string {
	t.Helper()
	if !strings.Contains(value, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(value, old, replacement, 1)
}

func replaceNth(t *testing.T, value, old, replacement string, occurrence int) string {
	t.Helper()
	index := 0
	for i := 0; i < occurrence; i++ {
		next := strings.Index(value[index:], old)
		if next < 0 {
			t.Fatalf("fixture does not contain occurrence %d of %q", occurrence, old)
		}
		index += next
		if i+1 < occurrence {
			index += len(old)
		}
	}
	return value[:index] + replacement + value[index+len(old):]
}
