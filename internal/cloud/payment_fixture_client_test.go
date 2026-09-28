package cloud

import (
	"context"
	"errors"
	"testing"

	"kwu/internal/userctx"
)

func TestFixturePaymentClientCreatesExactCanonicalMatrixThroughDomainInputs(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	authority, err := NewFixtureCloudAuthority(bundle, NewMemoryPaymentOrderRepository())
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[string]BindingScope{
		"usr_01": {ProfileID: "profile-primary", CloudUserID: "usr_01", BindingGeneration: 7},
		"usr_02": {ProfileID: "profile-secondary", CloudUserID: "usr_02", BindingGeneration: 1},
	}
	if len(bundle.PaymentCreates) != 12 {
		t.Fatalf("canonical creates = %d, want 12", len(bundle.PaymentCreates))
	}
	for index, fixture := range bundle.PaymentCreates {
		canonical := fixture.Result.Order
		userID := "usr_01"
		if canonical.OrderType == PaymentOrderSubscriptionUpgrade && canonical.FromPlan == "lite" || canonical.OrderType == PaymentOrderSubscriptionPurchase && index%2 == 1 {
			userID = "usr_02"
		}
		scope := scopes[userID]
		var got PaymentOrderResult
		if canonical.OrderType == PaymentOrderSubscriptionPurchase {
			got, err = payments.CreateSubscriptionOrder(ctx, CreateSubscriptionOrderRequest{
				Scope: scope, Plan: canonical.Plan, PaymentChannel: canonical.PaymentChannel, IdempotencyKey: "matrix-" + fixture.Key,
			})
		} else {
			got, err = payments.CreateUpgradeOrder(ctx, CreateUpgradeOrderRequest{
				Scope: scope, TargetPlan: canonical.Plan, PaymentChannel: canonical.PaymentChannel, IdempotencyKey: "matrix-" + fixture.Key,
			})
		}
		if err != nil {
			t.Fatalf("%s: %v", fixture.Key, err)
		}
		want := fixture.Result
		want.Order.Scope = scope
		if got != want {
			t.Fatalf("%s = %#v, want canonical projection %#v", fixture.Key, got, want)
		}
	}
}

func TestFixturePaymentClientProjectsLocalScopeAndAdvancesCanonicalTape(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatalf("NewFixtureCloudAuthority: %v", err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatalf("NewFixturePaymentClient: %v", err)
	}

	localScope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_02", BindingGeneration: 1}
	request := CreateUpgradeOrderRequest{
		Scope:          localScope,
		TargetPlan:     "plus",
		PaymentChannel: PaymentChannelWeChat,
		IdempotencyKey: "fixture-upgrade-idempotency",
	}
	created, err := payments.CreateUpgradeOrder(ctx, request)
	if err != nil {
		t.Fatalf("CreateUpgradeOrder: %v", err)
	}
	if created.Order.Scope != localScope || created.Order.Status != PaymentPending || created.Order.GrantStatus != GrantNotGranted {
		t.Fatalf("projected create = %#v, want local pending order", created)
	}
	if created.Order.Amount != "2000" || created.Order.AmountUnit != AmountUnitFen || created.Entry.EntryType != PaymentEntryNativeQR || created.Entry.CodeURL == "" {
		t.Fatalf("projected WeChat create = %#v, want fixture amount/unit/entry", created)
	}

	record, err := NewPaymentOrderRecord(localScope, created, PaymentOrderSubscriptionUpgrade, request.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	verifying, err := payments.GetPaymentOrder(ctx, localScope, created.Order.OrderID)
	if err != nil {
		t.Fatalf("GetPaymentOrder pending: %v", err)
	}
	if verifying.Scope != localScope || verifying.Status != PaymentVerifying || verifying.GrantStatus != GrantNotGranted {
		t.Fatalf("first progression = %#v, want local verifying order", verifying)
	}
	record.Result.Order = verifying
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}

	// Reconstruct every fake object while retaining only the saved order fact.
	authority, err = NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	payments, err = NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	granted, err := payments.GetPaymentOrder(ctx, localScope, created.Order.OrderID)
	if err != nil {
		t.Fatalf("GetPaymentOrder after client restart: %v", err)
	}
	if granted.Scope != localScope || granted.Status != PaymentGranted || granted.GrantStatus != GrantGranted {
		t.Fatalf("restart progression = %#v, want local granted order", granted)
	}
	record.Result.Order = granted
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	again, err := payments.GetPaymentOrder(ctx, localScope, created.Order.OrderID)
	if err != nil || again != granted {
		t.Fatalf("terminal progression = %#v, %v; want stable granted order", again, err)
	}
}

func TestFixturePaymentClientAllowsCanonicalPurchaseForAnyKnownAccount(t *testing.T) {
	bundle := loadA0Fixture(t)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	localScope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_02", BindingGeneration: 1}
	result, err := payments.CreateSubscriptionOrder(context.Background(), CreateSubscriptionOrderRequest{
		Scope: localScope, Plan: "pro", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "secondary-purchase-pro",
	})
	if err != nil {
		t.Fatalf("CreateSubscriptionOrder for secondary account: %v", err)
	}
	if result.Order.Scope != localScope || result.Order.Plan != "pro" || result.Order.Amount != "59.90" ||
		result.Order.AmountUnit != AmountUnitYuan || result.Entry.EntryType != PaymentEntryQRCode || result.Entry.QRCode == "" {
		t.Fatalf("secondary projected purchase = %#v", result)
	}
}

func TestFixturePaymentGrantSelectsCanonicalAccountSnapshot(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
	if err != nil {
		t.Fatalf("NewFixtureAccountClientWithAuthority: %v", err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}

	localScope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_02", BindingGeneration: 1}
	base, err := accounts.GetCurrentUser(ctx, localScope)
	if err != nil {
		t.Fatal(err)
	}
	request := CreateUpgradeOrderRequest{Scope: localScope, TargetPlan: "plus", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "grant-snapshot-idempotency"}
	created, err := payments.CreateUpgradeOrder(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewPaymentOrderRecord(localScope, created, PaymentOrderSubscriptionUpgrade, request.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}

	verifying, err := payments.GetPaymentOrder(ctx, localScope, created.Order.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	record.Result.Order = verifying
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	beforeGrant, err := accounts.GetCurrentUser(ctx, localScope)
	if err != nil {
		t.Fatal(err)
	}
	if beforeGrant.Balance != base.Balance || beforeGrant.Subscription != base.Subscription {
		t.Fatalf("pre-grant account changed: base=%#v beforeGrant=%#v", base, beforeGrant)
	}

	granted, err := payments.GetPaymentOrder(ctx, localScope, created.Order.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	record.Result.Order = granted
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	afterGrant, err := accounts.GetCurrentUser(ctx, localScope)
	if err != nil {
		t.Fatal(err)
	}
	tape := bundle.PaymentTapes[0]
	for _, candidate := range bundle.PaymentTapes {
		if candidate.OrderID == created.Order.OrderID {
			tape = candidate
			break
		}
	}
	want, ok := grantedSnapshotForUser(tape, localScope.CloudUserID)
	if tape.OrderID != created.Order.OrderID || !ok {
		t.Fatalf("canonical tape %q has no granted account snapshot", created.Order.OrderID)
	}
	want.Scope = localScope
	want.Subscription.Scope = localScope
	if afterGrant.Balance != want.Balance || afterGrant.Subscription != want.Subscription {
		t.Fatalf("post-grant account = %#v, want canonical snapshot %#v", afterGrant, want)
	}
}

func grantedSnapshotForUser(tape PaymentTapeFixture, cloudUserID string) (AccountSnapshotFixture, bool) {
	for _, snapshot := range tape.GrantedAccountSnapshots {
		if snapshot.Scope.CloudUserID == cloudUserID {
			return snapshot, true
		}
	}
	return AccountSnapshotFixture{}, false
}

func TestFixturePaymentPurchaseGrantSelectsSnapshotForSecondaryAccount(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	localScope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_02", BindingGeneration: 1}
	request := CreateSubscriptionOrderRequest{Scope: localScope, Plan: "pro", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "secondary-purchase-grant"}
	created, err := payments.CreateSubscriptionOrder(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewPaymentOrderRecord(localScope, created, PaymentOrderSubscriptionPurchase, request.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	for _, wantStatus := range []PaymentStatus{PaymentVerifying, PaymentGranted} {
		order, err := payments.GetPaymentOrder(ctx, localScope, created.Order.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		if order.Status != wantStatus {
			t.Fatalf("progression status = %q, want %q", order.Status, wantStatus)
		}
		record.Result.Order = order
		if err := orders.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	state, err := accounts.GetCurrentUser(ctx, localScope)
	if err != nil {
		t.Fatal(err)
	}
	if state.Balance.TotalBalance != "125000.0000" || state.Balance.SubscriptionBalance != "124500.0000" ||
		state.Subscription.Plan != "pro" || state.Subscription.Scope != localScope {
		t.Fatalf("secondary post-grant account = %#v, want the explicit usr_02/pro snapshot", state)
	}
}

func TestFixturePaymentClientUsesGrantedPlanForSequentialUpgrade(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	scope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_02", BindingGeneration: 1}
	firstRequest := CreateUpgradeOrderRequest{Scope: scope, TargetPlan: "plus", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "lite-plus"}
	first, err := payments.CreateUpgradeOrder(ctx, firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewPaymentOrderRecord(scope, first, PaymentOrderSubscriptionUpgrade, firstRequest.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := orders.Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	for _, wantStatus := range []PaymentStatus{PaymentVerifying, PaymentGranted} {
		order, err := payments.GetPaymentOrder(ctx, scope, first.Order.OrderID)
		if err != nil || order.Status != wantStatus {
			t.Fatalf("first upgrade progression = %#v, %v; want %q", order, err, wantStatus)
		}
		record.Result.Order = order
		if err := orders.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	second, err := payments.CreateUpgradeOrder(ctx, CreateUpgradeOrderRequest{
		Scope: scope, TargetPlan: "pro", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "plus-pro",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Order.FromPlan != "plus" || second.Order.Amount != "30.00" || second.Order.Scope != scope {
		t.Fatalf("sequential upgrade = %#v, want fixture plus->pro 30.00 projected to current scope", second)
	}
}

func TestFixturePaymentPrimaryPurchaseLiteThenUpgradePlusUsesExplicitSnapshots(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	scope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_01", BindingGeneration: 1}

	purchaseRequest := CreateSubscriptionOrderRequest{Scope: scope, Plan: "lite", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "primary-purchase-lite"}
	purchase, err := payments.CreateSubscriptionOrder(ctx, purchaseRequest)
	if err != nil {
		t.Fatal(err)
	}
	purchaseRecord, err := NewPaymentOrderRecord(scope, purchase, PaymentOrderSubscriptionPurchase, purchaseRequest.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := orders.Save(ctx, purchaseRecord); err != nil {
		t.Fatal(err)
	}
	for _, status := range []PaymentStatus{PaymentVerifying, PaymentGranted} {
		order, err := payments.GetPaymentOrder(ctx, scope, purchase.Order.OrderID)
		if err != nil || order.Status != status {
			t.Fatalf("purchase progression = %#v, %v; want %s", order, err, status)
		}
		purchaseRecord.Result.Order = order
		if err := orders.Save(ctx, purchaseRecord); err != nil {
			t.Fatal(err)
		}
	}
	lite, err := accounts.GetCurrentUser(ctx, scope)
	if err != nil || lite.Subscription.Plan != "lite" {
		t.Fatalf("account after purchase Lite = %#v, %v", lite, err)
	}

	upgradeRequest := CreateUpgradeOrderRequest{Scope: scope, TargetPlan: "plus", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "primary-lite-plus"}
	upgrade, err := payments.CreateUpgradeOrder(ctx, upgradeRequest)
	if err != nil || upgrade.Order.FromPlan != "lite" || upgrade.Order.Amount != "20.00" {
		t.Fatalf("Lite to Plus create = %#v, %v", upgrade, err)
	}
	upgradeRecord, err := NewPaymentOrderRecord(scope, upgrade, PaymentOrderSubscriptionUpgrade, upgradeRequest.IdempotencyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := orders.Save(ctx, upgradeRecord); err != nil {
		t.Fatal(err)
	}
	for _, status := range []PaymentStatus{PaymentVerifying, PaymentGranted} {
		order, err := payments.GetPaymentOrder(ctx, scope, upgrade.Order.OrderID)
		if err != nil || order.Status != status {
			t.Fatalf("upgrade progression = %#v, %v; want %s", order, err, status)
		}
		upgradeRecord.Result.Order = order
		if err := orders.Save(ctx, upgradeRecord); err != nil {
			t.Fatal(err)
		}
	}
	plus, err := accounts.GetCurrentUser(ctx, scope)
	if err != nil {
		t.Fatalf("GetCurrentUser after Lite to Plus: %v", err)
	}
	if plus.Subscription.Plan != "plus" || plus.Subscription.Scope != scope || plus.Balance.TotalBalance != "81000.0000" || plus.Balance.SubscriptionBalance != "80000.0000" {
		t.Fatalf("account after Lite to Plus = %#v, want explicit usr_01 Plus snapshot", plus)
	}
}

func TestFixturePaymentAuthorityUsesGrantSequenceForCompleteUpgradeChains(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	for _, account := range []struct {
		cloudUserID             string
		wantTotalBalance        string
		wantSubscriptionBalance string
	}{
		{cloudUserID: "usr_01", wantTotalBalance: "81000.0000", wantSubscriptionBalance: "80000.0000"},
		{cloudUserID: "usr_02", wantTotalBalance: "105000.0000", wantSubscriptionBalance: "104500.0000"},
	} {
		t.Run(account.cloudUserID, func(t *testing.T) {
			orders := NewMemoryPaymentOrderRepository()
			authority, err := NewFixtureCloudAuthority(bundle, orders)
			if err != nil {
				t.Fatal(err)
			}
			accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
			if err != nil {
				t.Fatal(err)
			}
			payments, err := NewFixturePaymentClient(bundle, authority)
			if err != nil {
				t.Fatal(err)
			}
			scope := BindingScope{ProfileID: "profile-" + account.cloudUserID, CloudUserID: account.cloudUserID, BindingGeneration: 1}
			grant := func(result PaymentOrderResult, operation PaymentOrderType, idempotencyKey string, wantSequence uint64) {
				t.Helper()
				record, err := NewPaymentOrderRecord(scope, result, operation, idempotencyKey)
				if err != nil {
					t.Fatal(err)
				}
				if err := orders.Save(ctx, record); err != nil {
					t.Fatal(err)
				}
				for _, wantStatus := range []PaymentStatus{PaymentVerifying, PaymentGranted} {
					order, err := payments.GetPaymentOrder(ctx, scope, result.Order.OrderID)
					if err != nil || order.Status != wantStatus {
						t.Fatalf("%s progression = %#v, %v; want %s", result.Order.OrderID, order, err, wantStatus)
					}
					record.Result.Order = order
					if err := orders.Save(ctx, record); err != nil {
						t.Fatal(err)
					}
				}
				stored, found, err := orders.Get(ctx, scope, result.Order.OrderID)
				if err != nil || !found || stored.GrantSequence != wantSequence {
					t.Fatalf("%s grant sequence = %d, %v, %v; want %d", result.Order.OrderID, stored.GrantSequence, found, err, wantSequence)
				}
			}

			purchaseKey := account.cloudUserID + "-purchase-lite"
			purchase, err := payments.CreateSubscriptionOrder(ctx, CreateSubscriptionOrderRequest{
				Scope: scope, Plan: "lite", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: purchaseKey,
			})
			if err != nil {
				t.Fatal(err)
			}
			grant(purchase, PaymentOrderSubscriptionPurchase, purchaseKey, 1)
			lite, err := accounts.GetCurrentUser(ctx, scope)
			if err != nil || lite.Subscription.Plan != "lite" {
				t.Fatalf("account after Lite purchase = %#v, %v", lite, err)
			}

			plusKey := account.cloudUserID + "-upgrade-plus"
			plus, err := payments.CreateUpgradeOrder(ctx, CreateUpgradeOrderRequest{
				Scope: scope, TargetPlan: "plus", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: plusKey,
			})
			if err != nil || plus.Order.FromPlan != "lite" {
				t.Fatalf("Lite to Plus create = %#v, %v", plus, err)
			}
			grant(plus, PaymentOrderSubscriptionUpgrade, plusKey, 2)
			plusAccount, err := accounts.GetCurrentUser(ctx, scope)
			if err != nil || plusAccount.Subscription.Plan != "plus" {
				t.Fatalf("account after Lite to Plus = %#v, %v", plusAccount, err)
			}

			proKey := account.cloudUserID + "-upgrade-pro"
			pro, err := payments.CreateUpgradeOrder(ctx, CreateUpgradeOrderRequest{
				Scope: scope, TargetPlan: "pro", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: proKey,
			})
			if err != nil || pro.Order.FromPlan != "plus" {
				t.Fatalf("Plus to Pro create = %#v, %v", pro, err)
			}
			grant(pro, PaymentOrderSubscriptionUpgrade, proKey, 3)

			rebuiltAuthority, err := NewFixtureCloudAuthority(bundle, orders)
			if err != nil {
				t.Fatal(err)
			}
			rebuiltAccounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, rebuiltAuthority)
			if err != nil {
				t.Fatal(err)
			}
			final, err := rebuiltAccounts.GetCurrentUser(ctx, scope)
			if err != nil {
				t.Fatal(err)
			}
			if final.Subscription.Plan != "pro" || final.Subscription.Scope != scope ||
				final.Balance.TotalBalance != account.wantTotalBalance || final.Balance.SubscriptionBalance != account.wantSubscriptionBalance {
				t.Fatalf("rebuilt final account = %#v, want explicit Pro %s/%s", final, account.wantTotalBalance, account.wantSubscriptionBalance)
			}
		})
	}
}

func TestFixturePaymentAuthorityFailsClosedForGrantedRecordWithoutSequence(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	scope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_01", BindingGeneration: 1}
	var terminal PaymentOrderResult
	for _, tape := range bundle.PaymentTapes {
		if tape.OrderID != "ord_purchase_plus_alipay" {
			continue
		}
		terminal = tape.Results[len(tape.Results)-1]
		break
	}
	if terminal.Order.Status != PaymentGranted {
		t.Fatalf("canonical terminal = %#v", terminal)
	}
	projected, err := projectFixturePaymentResult(terminal, scope)
	if err != nil {
		t.Fatal(err)
	}
	record, err := NewPaymentOrderRecord(scope, projected, PaymentOrderSubscriptionPurchase, "legacy-granted")
	if err != nil || record.GrantSequence != 0 {
		t.Fatalf("legacy record = %#v, %v", record, err)
	}
	repository := &fixturePaymentRecordListRepository{
		PaymentOrderRepository: NewMemoryPaymentOrderRepository(),
		records:                []PaymentOrderRecord{record},
	}
	authority, err := NewFixtureCloudAuthority(bundle, repository)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.GetCurrentUser(ctx, scope); !isProfileMismatch(err) {
		t.Fatalf("zero-sequence granted account error = %v", err)
	}
}

func TestFixturePaymentAuthorityFailsClosedForDuplicateGrantSequence(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	scope := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_01", BindingGeneration: 1}
	wanted := map[string]bool{"ord_purchase_plus_alipay": true, "ord_purchase_pro_alipay": true}
	records := make([]PaymentOrderRecord, 0, len(wanted))
	for _, tape := range bundle.PaymentTapes {
		if !wanted[tape.OrderID] {
			continue
		}
		projected, err := projectFixturePaymentResult(tape.Results[len(tape.Results)-1], scope)
		if err != nil {
			t.Fatal(err)
		}
		record, err := NewPaymentOrderRecord(scope, projected, PaymentOrderSubscriptionPurchase, "duplicate-"+tape.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		record.GrantSequence = 1
		records = append(records, record)
	}
	if len(records) != 2 {
		t.Fatalf("duplicate-sequence records = %#v", records)
	}
	repository := &fixturePaymentRecordListRepository{
		PaymentOrderRepository: NewMemoryPaymentOrderRepository(),
		records:                records,
	}
	authority, err := NewFixtureCloudAuthority(bundle, repository)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.GetCurrentUser(ctx, scope); !isProfileMismatch(err) {
		t.Fatalf("duplicate-sequence granted account error = %v", err)
	}
}

func TestFixturePaymentClientFixedFailureNeverGrantsEntitlements(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-failure"}
	bindings := NewBindingRepository()
	if _, err := bindings.ConfirmBinding(ctx, actor, "usr_02"); err != nil {
		t.Fatal(err)
	}
	scope := paymentApplicationScope(t, actor, "usr_02", 1)
	orders := NewMemoryPaymentOrderRepository()
	authority, err := NewFixtureCloudAuthority(bundle, orders)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewFixtureAccountClientWithAuthority(bundle, FixtureAccountScenarios{}, authority)
	if err != nil {
		t.Fatal(err)
	}
	base, err := accounts.GetCurrentUser(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	refresher := &paymentApplicationRefresher{}
	app := newPaymentApplication(t, bindings, payments, orders, refresher)
	created, err := app.CreateSubscriptionOrder(ctx, actor, "lite", PaymentChannelWeChat)
	if err != nil || created.State != PaymentUIStatePaying {
		t.Fatalf("failure create = %#v, %v", created, err)
	}
	verifying, err := app.RefreshOrder(ctx, actor, created.Result.Order.OrderID)
	if err != nil || verifying.State != PaymentUIStateConfirming {
		t.Fatalf("failure verifying = %#v, %v", verifying, err)
	}
	failed, err := app.RefreshOrder(ctx, actor, created.Result.Order.OrderID)
	if err != nil || failed.State != PaymentUIStateFailed || failed.EntitlementGranted || refresher.calls != 0 {
		t.Fatalf("failure terminal = %#v, %v, entitlement calls=%d", failed, err, refresher.calls)
	}
	after, err := accounts.GetCurrentUser(ctx, scope)
	if err != nil || after.Balance != base.Balance || after.Subscription != base.Subscription {
		t.Fatalf("failed payment changed account = %#v, %v; base %#v", after, err, base)
	}
	for _, tape := range bundle.PaymentTapes {
		if tape.OrderID == created.Result.Order.OrderID && len(tape.GrantedAccountSnapshots) != 0 {
			t.Fatal("fixed failed tape contains a granted account snapshot")
		}
	}
}

func TestFixturePaymentClientFailsClosedForUnknownAccountAndIllegalUpgrade(t *testing.T) {
	ctx := context.Background()
	bundle := loadA0Fixture(t)
	authority, err := NewFixtureCloudAuthority(bundle, NewMemoryPaymentOrderRepository())
	if err != nil {
		t.Fatal(err)
	}
	payments, err := NewFixturePaymentClient(bundle, authority)
	if err != nil {
		t.Fatal(err)
	}
	unknown := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_unknown", BindingGeneration: 1}
	if _, err := payments.CreateSubscriptionOrder(ctx, CreateSubscriptionOrderRequest{Scope: unknown, Plan: "pro", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "unknown"}); !isProfileMismatch(err) {
		t.Fatalf("unknown account error = %v", err)
	}
	plus := BindingScope{ProfileID: "profile-x", CloudUserID: "usr_01", BindingGeneration: 1}
	_, err = payments.CreateUpgradeOrder(ctx, CreateUpgradeOrderRequest{Scope: plus, TargetPlan: "lite", PaymentChannel: PaymentChannelAlipay, IdempotencyKey: "downgrade"})
	var cloudErr *Error
	if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorUpgradeNotAllowed {
		t.Fatalf("illegal upgrade error = %v, want %s", err, ErrorUpgradeNotAllowed)
	}
}

type fixturePaymentRecordListRepository struct {
	PaymentOrderRepository
	records []PaymentOrderRecord
}

func (r *fixturePaymentRecordListRepository) List(context.Context, BindingScope) ([]PaymentOrderRecord, error) {
	return append([]PaymentOrderRecord(nil), r.records...), nil
}
