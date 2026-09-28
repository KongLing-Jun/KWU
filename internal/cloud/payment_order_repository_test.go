package cloud

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryPaymentOrderRepositoryIsolatesFullBindingScope(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	base := paymentOrderRepositoryScope("default", "usr_01", 1)
	scopes := []BindingScope{
		base,
		paymentOrderRepositoryScope("secondary", "usr_01", 1),
		paymentOrderRepositoryScope("default", "usr_02", 1),
		paymentOrderRepositoryScope("default", "usr_01", 2),
	}

	records := make([]PaymentOrderRecord, 0, len(scopes))
	for index, scope := range scopes {
		record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPending)
		if err := repository.Save(ctx, record); err != nil {
			t.Fatalf("Save(%d): %v", index, err)
		}
		records = append(records, record)
	}
	for index, record := range records {
		scope := record.Scope
		byOrder, ok, err := repository.Get(ctx, scope, "pay_01")
		if err != nil || !ok || byOrder != record {
			t.Fatalf("Get(%d) = %#v, %v, %v", index, byOrder, ok, err)
		}
		byKey, ok, err := repository.FindByIdempotencyKey(ctx, scope, "idem_01")
		if err != nil || !ok || byKey != record {
			t.Fatalf("FindByIdempotencyKey(%d) = %#v, %v, %v", index, byKey, ok, err)
		}
	}
}

func TestMemoryPaymentOrderRepositoryReadsAndPreservesIdempotency(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPending)
	if err := repository.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, record); err != nil {
		t.Fatalf("idempotent Save: %v", err)
	}

	if got, ok, err := repository.Get(ctx, scope, record.Result.Order.OrderID); err != nil || !ok || got != record {
		t.Fatalf("Get = %#v, %v, %v", got, ok, err)
	}
	if got, ok, err := repository.FindByIdempotencyKey(ctx, scope, record.IdempotencyKey); err != nil || !ok || got != record {
		t.Fatalf("FindByIdempotencyKey = %#v, %v, %v", got, ok, err)
	}
	updated := record
	updated.Result = paymentOrderRepositoryResult(scope, "pay_01", PaymentGranted)
	if err := repository.Save(ctx, updated); err != nil {
		t.Fatalf("Save authoritative update: %v", err)
	}
	updated.GrantSequence = 1
	if got, ok, err := repository.Get(ctx, scope, "pay_01"); err != nil || !ok || got != updated {
		t.Fatalf("Get updated record = %#v, %v, %v", got, ok, err)
	}
	if got, ok, err := repository.FindByIdempotencyKey(ctx, scope, "idem_01"); err != nil || !ok || got != updated {
		t.Fatalf("FindByIdempotencyKey updated record = %#v, %v, %v", got, ok, err)
	}

	if err := repository.Save(ctx, paymentOrderRepositoryRecord(t, scope, "pay_02", "idem_01", PaymentPending)); err == nil {
		t.Fatal("Save accepted an idempotency key for another order")
	}
	if err := repository.Save(ctx, paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_02", PaymentPending)); err == nil {
		t.Fatal("Save accepted an order for another idempotency key")
	}
}

func TestMemoryPaymentOrderRepositoryListsOnlyCompleteScopeInStableOrder(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	for _, record := range []PaymentOrderRecord{
		paymentOrderRepositoryRecord(t, scope, "pay_b", "idem_b", PaymentPending),
		paymentOrderRepositoryRecord(t, scope, "pay_a", "idem_a", PaymentPending),
		paymentOrderRepositoryRecord(t, paymentOrderRepositoryScope("secondary", "usr_01", 1), "pay_other_profile", "idem_other_profile", PaymentPending),
		paymentOrderRepositoryRecord(t, paymentOrderRepositoryScope("default", "usr_02", 1), "pay_other_user", "idem_other_user", PaymentPending),
		paymentOrderRepositoryRecord(t, paymentOrderRepositoryScope("default", "usr_01", 2), "pay_old_generation", "idem_old_generation", PaymentPending),
	} {
		if err := repository.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	records, err := repository.List(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Result.Order.OrderID != "pay_a" || records[1].Result.Order.OrderID != "pay_b" {
		t.Fatalf("List = %#v", records)
	}
}

func TestMemoryPaymentOrderRepositoryMarksGrantedRefreshOnlyWithinScope(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	first := paymentOrderRepositoryScope("default", "usr_01", 1)
	second := paymentOrderRepositoryScope("default", "usr_01", 2)
	for _, scope := range []BindingScope{first, second} {
		if err := repository.Save(ctx, paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentGranted)); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.MarkEntitlementsRefreshed(ctx, first, "pay_01"); err != nil {
		t.Fatal(err)
	}
	marked, ok, err := repository.Get(ctx, first, "pay_01")
	if err != nil || !ok || !marked.EntitlementsRefreshed {
		t.Fatalf("marked record = %#v, %v, %v", marked, ok, err)
	}
	other, ok, err := repository.Get(ctx, second, "pay_01")
	if err != nil || !ok || other.EntitlementsRefreshed {
		t.Fatalf("another generation marker = %#v, %v, %v", other, ok, err)
	}
	if err := NewMemoryPaymentOrderRepository().MarkEntitlementsRefreshed(ctx, first, "pay_01"); !isProfileMismatch(err) {
		t.Fatalf("missing record marker error = %v", err)
	}
	pendingRepository := NewMemoryPaymentOrderRepository()
	if err := pendingRepository.Save(ctx, paymentOrderRepositoryRecord(t, first, "pay_pending", "idem_pending", PaymentPending)); err != nil {
		t.Fatal(err)
	}
	if err := pendingRepository.MarkEntitlementsRefreshed(ctx, first, "pay_pending"); err == nil {
		t.Fatal("MarkEntitlementsRefreshed accepted a non-granted order")
	}
}

func TestMemoryPaymentOrderRepositoryAssignsGrantSequencePerScope(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	firstScope := paymentOrderRepositoryScope("default", "usr_01", 1)
	secondScope := paymentOrderRepositoryScope("default", "usr_02", 1)

	first := paymentOrderRepositoryRecord(t, firstScope, "pay_01", "idem_01", PaymentPending)
	if err := repository.Save(ctx, first); err != nil {
		t.Fatal(err)
	}
	first.Result = paymentOrderRepositoryResult(firstScope, "pay_01", PaymentGranted)
	if err := repository.Save(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := paymentOrderRepositoryRecord(t, firstScope, "pay_02", "idem_02", PaymentGranted)
	if err := repository.Save(ctx, second); err != nil {
		t.Fatal(err)
	}
	otherScope := paymentOrderRepositoryRecord(t, secondScope, "pay_01", "idem_01", PaymentGranted)
	if err := repository.Save(ctx, otherScope); err != nil {
		t.Fatal(err)
	}

	for _, item := range []struct {
		scope   BindingScope
		orderID string
		want    uint64
	}{
		{firstScope, "pay_01", 1},
		{firstScope, "pay_02", 2},
		{secondScope, "pay_01", 1},
	} {
		stored, found, err := repository.Get(ctx, item.scope, item.orderID)
		if err != nil || !found || stored.GrantSequence != item.want {
			t.Fatalf("grant sequence for %s/%s = %d, %v, %v; want %d", item.scope.key(), item.orderID, stored.GrantSequence, found, err, item.want)
		}
	}
}

func TestMemoryPaymentOrderRepositoryPreservesGrantedSequenceAndMarker(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	stale := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentGranted)
	if err := repository.Save(ctx, stale); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkEntitlementsRefreshed(ctx, scope, "pay_01"); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, stale); err != nil {
		t.Fatalf("repeated granted Save: %v", err)
	}
	stored, found, err := repository.Get(ctx, scope, "pay_01")
	if err != nil || !found || stored.GrantSequence != 1 || !stored.EntitlementsRefreshed {
		t.Fatalf("preserved granted record = %#v, %v, %v", stored, found, err)
	}
}

func TestPaymentOrderRepositoriesRejectCallerAssignedGrantSequence(t *testing.T) {
	ctx := context.Background()
	for name, newRepository := range map[string]func(*testing.T) PaymentOrderRepository{
		"memory": func(*testing.T) PaymentOrderRepository { return NewMemoryPaymentOrderRepository() },
		"durable": func(t *testing.T) PaymentOrderRepository {
			return NewDurablePaymentOrderRepository(testProfilePathProvider(t.TempDir()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope := paymentOrderRepositoryScope("default", "usr_01", 1)
			record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentGranted)
			record.GrantSequence = 99
			repository := newRepository(t)
			if err := repository.Save(ctx, record); err == nil {
				t.Fatal("Save accepted caller-assigned grant sequence")
			}
			if records, err := repository.List(ctx, scope); err != nil || len(records) != 0 {
				t.Fatalf("rejected sequence persisted records = %#v, %v", records, err)
			}
		})
	}
}

func TestPaymentOrderRepositoriesReserveAttemptWithCompareAndSequence(t *testing.T) {
	ctx := context.Background()
	for name, newRepository := range map[string]func(*testing.T) PaymentOrderRepository{
		"memory": func(*testing.T) PaymentOrderRepository { return NewMemoryPaymentOrderRepository() },
		"durable": func(t *testing.T) PaymentOrderRepository {
			return NewDurablePaymentOrderRepository(testProfilePathProvider(t.TempDir()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope := paymentOrderRepositoryScope("default", "usr_01", 1)
			repository := newRepository(t)
			firstCandidate := paymentOrderRepositoryAttempt(t, scope, "attempt-01", "plus", PaymentChannelAlipay)
			secondCandidate := paymentOrderRepositoryAttempt(t, scope, "attempt-02", "plus", PaymentChannelAlipay)

			first, created, err := repository.ReserveAttempt(ctx, "", firstCandidate)
			if err != nil || !created || first.Sequence != 1 {
				t.Fatalf("first reserve = %#v, %v, %v", first, created, err)
			}
			winner, created, err := repository.ReserveAttempt(ctx, "", secondCandidate)
			if err != nil || created || winner.AttemptID != first.AttemptID || winner.Sequence != 1 {
				t.Fatalf("conflicting reserve = %#v, %v, %v", winner, created, err)
			}
			second, created, err := repository.ReserveAttempt(ctx, first.AttemptID, secondCandidate)
			if err != nil || !created || second.Sequence != 2 {
				t.Fatalf("successor reserve = %#v, %v, %v", second, created, err)
			}
			latest, found, err := repository.LatestAttempt(ctx, scope, PaymentOrderSubscriptionPurchase, "plus", PaymentChannelAlipay)
			if err != nil || !found || latest.AttemptID != second.AttemptID || latest.Sequence != 2 {
				t.Fatalf("latest attempt = %#v, %v, %v", latest, found, err)
			}
		})
	}
}

func TestPaymentOrderRepositoriesRejectInvalidAttemptReservation(t *testing.T) {
	ctx := context.Background()
	for name, newRepository := range map[string]func(*testing.T) PaymentOrderRepository{
		"memory": func(*testing.T) PaymentOrderRepository { return NewMemoryPaymentOrderRepository() },
		"durable": func(t *testing.T) PaymentOrderRepository {
			return NewDurablePaymentOrderRepository(testProfilePathProvider(t.TempDir()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope := paymentOrderRepositoryScope("default", "usr_01", 1)
			repository := newRepository(t)
			first := paymentOrderRepositoryAttempt(t, scope, "attempt-01", "plus", PaymentChannelAlipay)
			if _, created, err := repository.ReserveAttempt(ctx, "", first); err != nil || !created {
				t.Fatalf("first reserve = %v, %v", created, err)
			}
			collision := paymentOrderRepositoryAttempt(t, scope, first.AttemptID, "pro", PaymentChannelAlipay)
			if _, _, err := repository.ReserveAttempt(ctx, "", collision); err == nil {
				t.Fatal("ReserveAttempt accepted an existing attempt ID")
			}
			bound := paymentOrderRepositoryAttempt(t, scope, "attempt-02", "pro", PaymentChannelAlipay)
			bound.OrderID = "order_02"
			if _, _, err := repository.ReserveAttempt(ctx, "", bound); err == nil {
				t.Fatal("ReserveAttempt accepted a pre-bound attempt")
			}
		})
	}
}

func TestPaymentOrderRepositoriesIsolateAttemptsByBindingGeneration(t *testing.T) {
	ctx := context.Background()
	for name, newRepository := range map[string]func(*testing.T) PaymentOrderRepository{
		"memory": func(*testing.T) PaymentOrderRepository { return NewMemoryPaymentOrderRepository() },
		"durable": func(t *testing.T) PaymentOrderRepository {
			return NewDurablePaymentOrderRepository(testProfilePathProvider(t.TempDir()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			firstScope := paymentOrderRepositoryScope("default", "usr_01", 1)
			secondScope := paymentOrderRepositoryScope("default", "usr_01", 2)
			repository := newRepository(t)
			first, created, err := repository.ReserveAttempt(ctx, "", paymentOrderRepositoryAttempt(t, firstScope, "attempt-01", "plus", PaymentChannelAlipay))
			if err != nil || !created {
				t.Fatalf("first reserve = %#v, %v, %v", first, created, err)
			}
			second, created, err := repository.ReserveAttempt(ctx, "", paymentOrderRepositoryAttempt(t, secondScope, "attempt-02", "plus", PaymentChannelAlipay))
			if err != nil || !created || second.Sequence != 1 || second.IdempotencyKey == first.IdempotencyKey {
				t.Fatalf("second reserve = %#v, %v, %v", second, created, err)
			}
			record, err := NewPaymentOrderRecord(
				firstScope,
				paymentOrderRepositoryResult(firstScope, "order_01", PaymentPending),
				PaymentOrderSubscriptionPurchase,
				first.IdempotencyKey,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := repository.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
			if err := repository.BindAttempt(ctx, secondScope, second.AttemptID, "order_01"); !isProfileMismatch(err) {
				t.Fatalf("cross-generation bind error = %v", err)
			}
		})
	}
}

func TestPaymentOrderRepositoriesRejectQRCodeURLIdentityMutation(t *testing.T) {
	ctx := context.Background()
	for name, newRepository := range map[string]func(*testing.T) PaymentOrderRepository{
		"memory": func(*testing.T) PaymentOrderRepository { return NewMemoryPaymentOrderRepository() },
		"durable": func(t *testing.T) PaymentOrderRepository {
			return NewDurablePaymentOrderRepository(testProfilePathProvider(t.TempDir()))
		},
	} {
		t.Run(name, func(t *testing.T) {
			scope := paymentOrderRepositoryScope("default", "usr_01", 1)
			record := paymentOrderRepositoryRecord(t, scope, "order_01", "idem_01", PaymentPending)
			record.Result.Entry.QRCode = ""
			record.Result.Entry.QRCodeURL = "https://pay.example.invalid/first"
			repository := newRepository(t)
			if err := repository.Save(ctx, record); err != nil {
				t.Fatal(err)
			}
			changed := record
			changed.Result.Entry.QRCodeURL = "https://pay.example.invalid/changed"
			if err := repository.Save(ctx, changed); err == nil {
				t.Fatal("Save accepted a changed V3 QR code URL")
			}
		})
	}
}

func TestMemoryPaymentOrderRepositoryRejectsStatusRegression(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPending)
	if err := repository.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	advanced := record
	advanced.Result = paymentOrderRepositoryResult(scope, "pay_01", PaymentPaidVerified)
	if err := repository.Save(ctx, advanced); err != nil {
		t.Fatal(err)
	}
	regressed := record
	regressed.Result = paymentOrderRepositoryResult(scope, "pay_01", PaymentVerifying)
	if err := repository.Save(ctx, regressed); err == nil {
		t.Fatal("Save accepted payment status regression")
	}
	stored, found, err := repository.Get(ctx, scope, "pay_01")
	if err != nil || !found || stored.Result.Order.Status != PaymentPaidVerified {
		t.Fatalf("stored monotonic record = %#v, %v, %v", stored, found, err)
	}
}

func TestPaymentOrderRecordRejectsEntitlementRefreshMarkerBeforeGrant(t *testing.T) {
	ctx := context.Background()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	for _, status := range []PaymentStatus{
		PaymentCreated,
		PaymentPending,
		PaymentCallbackReceived,
		PaymentVerifying,
		PaymentPaidVerified,
		PaymentGranting,
		PaymentClosed,
		PaymentTimeout,
		PaymentFailed,
		PaymentVerifyFailedManualRequired,
	} {
		t.Run(string(status), func(t *testing.T) {
			record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", status)
			record.EntitlementsRefreshed = true
			if err := record.Validate(); err == nil {
				t.Fatal("Validate accepted an entitlement marker before grant")
			}
			if err := NewMemoryPaymentOrderRepository().Save(ctx, record); err == nil {
				t.Fatal("Save accepted an entitlement marker before grant")
			}
		})
	}
}

func TestMemoryPaymentOrderRepositoryConcurrentConflictingSaveIsRejected(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	records := []PaymentOrderRecord{
		paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPending),
		paymentOrderRepositoryRecord(t, scope, "pay_02", "idem_01", PaymentPending),
	}

	start := make(chan struct{})
	errs := make(chan error, len(records))
	var wait sync.WaitGroup
	for _, record := range records {
		wait.Add(1)
		go func(record PaymentOrderRecord) {
			defer wait.Done()
			<-start
			errs <- repository.Save(ctx, record)
		}(record)
	}
	close(start)
	wait.Wait()
	close(errs)

	var successes, failures int
	for err := range errs {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("conflicting concurrent saves: successes=%d failures=%d", successes, failures)
	}
}

func TestPaymentOrderRecordPersistsPaidVerifiedWithoutGrant(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryPaymentOrderRepository()
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPaidVerified)
	if err := repository.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	got, ok, err := repository.Get(ctx, scope, "pay_01")
	if err != nil || !ok {
		t.Fatalf("Get = %v, %v", ok, err)
	}
	if got.Result.Order.GrantStatus != GrantNotGranted {
		t.Fatalf("paid_verified grant status = %q", got.Result.Order.GrantStatus)
	}
	state, err := MapPaymentStatus(got.Result.Order.Status)
	if err != nil || state != PaymentUIStateConfirming {
		t.Fatalf("paid_verified state = %q, %v", state, err)
	}
}

func TestPaymentOrderRecordRejectsMismatchedAndTransientValues(t *testing.T) {
	scope := paymentOrderRepositoryScope("default", "usr_01", 1)
	record := paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPending)
	record.Scope.BindingGeneration++
	if err := record.Validate(); err == nil {
		t.Fatal("Validate accepted a record scope different from the order scope")
	}

	record = paymentOrderRepositoryRecord(t, scope, "pay_01", "idem_01", PaymentPending)
	record.Operation = PaymentOrderSubscriptionUpgrade
	if err := record.Validate(); err == nil {
		t.Fatal("Validate accepted an operation different from the order type")
	}

	transient := paymentOrderRepositoryRecordUnchecked(scope, "pay_01", "idem_01", PaymentClientPaidUnverified)
	if err := transient.Validate(); err == nil {
		t.Fatal("Validate accepted client_paid_unverified for persistence")
	}
	if err := NewMemoryPaymentOrderRepository().Save(context.Background(), transient); err == nil {
		t.Fatal("Save accepted client_paid_unverified for persistence")
	}
}

func paymentOrderRepositoryScope(profileID, cloudUserID string, generation uint64) BindingScope {
	return BindingScope{ProfileID: profileID, CloudUserID: cloudUserID, BindingGeneration: generation}
}

func paymentOrderRepositoryAttempt(t *testing.T, scope BindingScope, attemptID, plan string, channel PaymentChannel) PaymentAttempt {
	t.Helper()
	key, err := paymentOrderIdempotencyKey(PaymentOrderSubscriptionPurchase, scope, plan, channel, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	return PaymentAttempt{
		Scope: scope, AttemptID: attemptID, Operation: PaymentOrderSubscriptionPurchase,
		Plan: plan, PaymentChannel: channel, IdempotencyKey: key,
		CreatedAt: time.Date(2026, time.July, 11, 0, 30, 0, 0, time.UTC),
	}
}

func paymentOrderRepositoryRecord(t *testing.T, scope BindingScope, orderID, idempotencyKey string, status PaymentStatus) PaymentOrderRecord {
	t.Helper()
	record, err := NewPaymentOrderRecord(scope, paymentOrderRepositoryResult(scope, orderID, status), PaymentOrderSubscriptionPurchase, idempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func paymentOrderRepositoryRecordUnchecked(scope BindingScope, orderID, idempotencyKey string, status PaymentStatus) PaymentOrderRecord {
	return PaymentOrderRecord{
		Scope:          scope,
		Result:         paymentOrderRepositoryResult(scope, orderID, status),
		Operation:      PaymentOrderSubscriptionPurchase,
		IdempotencyKey: idempotencyKey,
	}
}

func paymentOrderRepositoryResult(scope BindingScope, orderID string, status PaymentStatus) PaymentOrderResult {
	now := time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC)
	grantStatus := GrantNotGranted
	if status == PaymentGranted {
		grantStatus = GrantGranted
	}
	return PaymentOrderResult{
		Order: PaymentOrder{
			Scope: scope, OrderID: orderID, OrderType: PaymentOrderSubscriptionPurchase,
			Plan: "plus", Amount: "29.90", AmountUnit: AmountUnitYuan, Currency: "CNY",
			PaymentChannel: PaymentChannelAlipay, Status: status, GrantStatus: grantStatus,
			ExpiresAt: now.Add(time.Hour),
		},
		Entry: PaymentEntry{Channel: PaymentChannelAlipay, EntryType: PaymentEntryQRCode, QRCode: "https://example.invalid/pay", ExpiresAt: now.Add(time.Hour)},
	}
}
