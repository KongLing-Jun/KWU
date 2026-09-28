package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/userctx"
)

type failingAccountPlanClient struct {
	err error
}

func (c failingAccountPlanClient) ListSubscriptionPlans(context.Context, cloud.BindingScope) ([]cloud.SubscriptionPlan, error) {
	return nil, c.err
}

func TestRefreshRemoteSubscriptionDoesNotReuseCachedPlans(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	binding, err := runtime.bindings.ConfirmBinding(ctx, actor, "usr_plan_cache")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.assets.StoreSubscription(ctx, scope, cloud.SubscriptionView{
		Plans: []cloud.SubscriptionPlan{{
			PlanID: "cached-pro", Plan: "pro", DisplayName: "Cached Pro",
			PriceAmount: "3.00", Currency: "CNY", CreditAmount: "120000.0000",
			PeriodDays: 30, Enabled: true, SortOrder: 30, ConfigVersion: "cached-v1",
		}},
		Subscription: cloud.Subscription{Scope: scope, Plan: "free", Status: cloud.SubscriptionFree},
	}); err != nil {
		t.Fatal(err)
	}
	runtime.planClient = failingAccountPlanClient{err: errors.New("plan catalog unavailable")}

	view, err := NewApp().refreshRemoteSubscription(runtime, scope, cloud.Subscription{
		Scope: scope, Plan: "free", Status: cloud.SubscriptionFree,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Current.Plan != "free" || view.Current.Status != cloud.SubscriptionFree || len(view.Plans) != 0 {
		t.Fatalf("subscription view = %#v", view)
	}
}

func TestCloudBootstrapWithoutFakeFlagUsesDevelopmentRealCloudWithoutPanic(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("NewApp/CloudBootstrap panicked without fake flag: %v", recovered)
		}
	}()
	app := NewApp()
	view, err := app.CloudBootstrap()
	if err != nil {
		t.Fatalf("CloudBootstrap without KWU_CLOUD_FAKE error = %v", err)
	}
	if !view.Available || !view.Capabilities.Registration || view.Capabilities.RegistrationMode != string(cloudRegistrationReal) {
		t.Fatalf("CloudBootstrap view = %#v, want development real cloud", view)
	}
}

func TestCloudCachedAssetsReadsOnlyCurrentCompleteBindingScope(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	binding, err := runtime.bindings.ConfirmBinding(ctx, actor, "usr_cached")
	if err != nil {
		t.Fatal(err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	balance := cloud.BalanceSnapshot{
		TotalBalance: "90", FreeBalance: "40", SubscriptionBalance: "50",
		AvailableBalance: "80", FreeAvailableBalance: "35", SubscriptionAvailableBalance: "45",
		FrozenBalance: "10", FreeFrozenBalance: "5", SubscriptionFrozenBalance: "5", Version: 7,
	}
	if _, err := runtime.assets.StoreBalance(ctx, scope, balance); err != nil {
		t.Fatal(err)
	}
	subscription := cloud.SubscriptionView{Subscription: cloud.Subscription{
		Scope: scope, Plan: "pro", Status: cloud.SubscriptionActive,
		PeriodEnd: time.Date(2026, time.August, 29, 0, 0, 0, 0, time.UTC), Version: 9,
	}}
	if err := runtime.assets.StoreSubscription(ctx, scope, subscription); err != nil {
		t.Fatal(err)
	}

	// The cache API must remain usable with every network-capable application
	// removed from the runtime.
	runtime.accounts = nil
	runtime.remoteAccount = nil
	runtime.balance = nil
	runtime.subscription = nil
	runtime.paymentRecovery = nil
	runtime.planClient = nil

	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	view, err := app.CloudCachedAssets()
	if err != nil {
		t.Fatalf("CloudCachedAssets: %v", err)
	}
	if view.ProfileID != scope.ProfileID || view.CloudUserID != scope.CloudUserID || view.BindingGeneration != scope.BindingGeneration {
		t.Fatalf("cache scope = %#v, want %#v", view, scope)
	}
	if view.Balance == nil || view.Balance.Version != balance.Version || view.Balance.TotalBalance != balance.TotalBalance {
		t.Fatalf("cached balance = %#v", view.Balance)
	}
	if view.Subscription == nil || view.Subscription.Current.Version != subscription.Subscription.Version || view.Subscription.Current.Plan != subscription.Subscription.Plan {
		t.Fatalf("cached subscription = %#v", view.Subscription)
	}

	rebound, err := runtime.bindings.Rebind(ctx, actor, "usr_rebound")
	if err != nil {
		t.Fatal(err)
	}
	reboundScope, err := rebound.Scope()
	if err != nil {
		t.Fatal(err)
	}
	view, err = app.CloudCachedAssets()
	if err != nil {
		t.Fatalf("CloudCachedAssets after rebind: %v", err)
	}
	if view.ProfileID != reboundScope.ProfileID || view.CloudUserID != reboundScope.CloudUserID || view.BindingGeneration != reboundScope.BindingGeneration {
		t.Fatalf("rebound cache scope = %#v, want %#v", view, reboundScope)
	}
	if view.Balance != nil || view.Subscription != nil {
		t.Fatalf("rebound cache leaked old snapshots: %#v", view)
	}
}

func TestCloudCachedAssetsRejectsUnboundOrWrongActor(t *testing.T) {
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	for _, actor := range []userctx.ActorContext{
		{ActorID: "local", ProfileID: "default"},
		{ActorID: "other", ProfileID: "other-profile"},
	} {
		app.setCurrentActorContext(actor)
		if view, err := app.CloudCachedAssets(); err == nil {
			t.Fatalf("CloudCachedAssets(%#v) = %#v, want scope error", actor, view)
		} else {
			var cloudErr *cloud.Error
			if !errors.As(err, &cloudErr) || cloudErr.Code != cloud.ErrorProfileScopeMismatch {
				t.Fatalf("CloudCachedAssets(%#v) error = %#v", actor, err)
			}
		}
	}
}

func TestCloudBootstrapProjectsRejectedDurableStateAsExpired(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	root := t.TempDir()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	provider := desktopCloudTestProfilePathProvider(root)
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "binding.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{corrupt durable json"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	view, err := app.CloudBootstrap()
	if err != nil {
		t.Fatalf("CloudBootstrap rejected durable state error = %v", err)
	}
	if !view.Available || view.Account.Status != cloud.AccountExpired || view.Balance != nil || view.Subscription != nil || len(view.Payments) != 0 {
		t.Fatalf("CloudBootstrap rejected durable view = %#v", view)
	}
}

func TestCloudAccountViewKeepsExpiredOrderingIdentityButHidesDisplayIdentity(t *testing.T) {
	syncedAt := time.Date(2026, time.July, 27, 8, 0, 0, 0, time.UTC)
	view := cloudAccountView("profile-expired", cloud.AuthView{
		State: cloud.AccountExpired,
		Binding: cloud.ProfileBinding{
			ProfileID: "profile-expired", CloudUserID: "ordering-user", BindingGeneration: 7, Status: cloud.BindingExpired,
		},
		User:       cloud.CloudUser{CloudUserID: "ordering-user", DisplayName: "Old Name", Role: "admin"},
		LastSyncAt: syncedAt,
	}, cloud.AccountState{})
	if view.Status != cloud.AccountExpired || view.CloudUserID != "ordering-user" || view.BindingGeneration != 7 {
		t.Fatalf("expired ordering identity = %#v", view)
	}
	if view.DisplayName != "" || view.Role != "" {
		t.Fatalf("expired display identity was not sanitized: %#v", view)
	}
}

func TestCloudBootstrapDoesNotSwallowOrdinaryRecoveryError(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	root := t.TempDir()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "expected-profile"}.Normalize()
	provider := func(userctx.ActorContext) config.ProfilePathResolver {
		return config.NewProfilePathResolverWithOptions(config.ProfilePathResolverOptions{
			Actor:     userctx.ActorContext{ActorID: actor.ActorID, ProfileID: "wrong-profile"},
			HomeRoot:  filepath.Join(root, "home"),
			StateRoot: filepath.Join(root, "state"),
			CacheRoot: filepath.Join(root, "cache"),
		})
	}
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	view, err := app.CloudBootstrap()
	if err == nil || view.Account.Status == cloud.AccountExpired {
		t.Fatalf("ordinary recovery error was swallowed: view=%#v err=%v", view, err)
	}
}

func TestCloudWailsErrorSanitizesWrappedTypedError(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "fixture-scenario-secret.json")
	wrapped := fmt.Errorf("fixture scenario at %s: %w", secret, &cloud.Error{
		Code: cloud.ErrorAuthRequired, Message: "raw fixture authentication detail", Details: json.RawMessage(`{"scenario":"secret"}`),
	})
	safe := cloudWailsError("cloud.test", wrapped)
	var cloudErr *cloud.Error
	if !errors.As(safe, &cloudErr) || cloudErr.Code != cloud.ErrorAuthRequired || cloudErr.Message != "authentication is required" {
		t.Fatalf("wrapped typed error = %#v, want safe AUTH_REQUIRED", safe)
	}
	if cloudErr.Details != nil || cloudErr.Retryable || strings.Contains(safe.Error(), secret) || strings.Contains(safe.Error(), "fixture") || strings.Contains(safe.Error(), "scenario") {
		t.Fatalf("wrapped typed error leaked internal data: %#v", safe)
	}
}

func TestCloudWailsErrorSanitizesUnknownFixtureWrapper(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "unknown-fixture-secret.json")
	wrapped := fmt.Errorf("fixture scenario at %s: %w", secret, &cloud.Error{
		Code: "UNKNOWN_FIXTURE_ERROR", Message: "raw fixture detail", Details: json.RawMessage(`{"scenario":"secret"}`),
	})
	safe := cloudWailsError("cloud.test", wrapped)
	var cloudErr *cloud.Error
	if !errors.As(safe, &cloudErr) || cloudErr.Code != cloud.ErrorCode("CLOUD_ERROR") || cloudErr.Message != "cloud request failed" {
		t.Fatalf("unknown fixture error = %#v, want safe CLOUD_ERROR", safe)
	}
	for _, forbidden := range []string{secret, "UNKNOWN_FIXTURE_ERROR", "fixture", "scenario", "raw fixture detail"} {
		if strings.Contains(safe.Error(), forbidden) {
			t.Fatalf("unknown fixture error exposed %q: %v", forbidden, safe)
		}
	}
}

func TestCloudWailsErrorMapsWrappedCaptchaSentinels(t *testing.T) {
	tests := []struct {
		name    string
		cause   error
		code    cloud.ErrorCode
		message string
	}{
		{"required", cloud.ErrCaptchaVerificationRequired, cloud.ErrorSliderCaptchaRequired, "captcha verification is required"},
		{"expired", cloud.ErrCaptchaPermitExpired, cloud.ErrorSliderCaptchaExpired, "captcha verification expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe := cloudWailsError("cloud.test", fmt.Errorf("internal captcha detail: %w", tt.cause))
			var cloudErr *cloud.Error
			if !errors.As(safe, &cloudErr) || cloudErr.Code != tt.code || cloudErr.Message != tt.message {
				t.Fatalf("wrapped captcha error = %#v, want %s", safe, tt.code)
			}
		})
	}
}

func TestCloudBootstrapRecoversDurableAccountAndAssetsAfterRuntimeRestart(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())

	first, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.auth.Login(ctx, actor, cloud.LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	if _, err := first.balance.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := first.subscription.Refresh(ctx, actor); err != nil {
		t.Fatal(err)
	}
	first.close()

	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.close)
	restoredBalance, found, err := restarted.balance.View(ctx, actor)
	if err != nil || !found || restoredBalance.TotalBalance != "21000.0000" {
		t.Fatalf("restarted balance before remote refresh = %#v, found %v, err %v", restoredBalance, found, err)
	}
	restoredSubscription, found, err := restarted.subscription.View(ctx, actor)
	if err != nil || !found || restoredSubscription.Subscription.Plan != "plus" || len(restoredSubscription.Plans) == 0 {
		t.Fatalf("restarted subscription before remote refresh = %#v, found %v, err %v", restoredSubscription, found, err)
	}
	app := NewApp()
	app.cloudRuntime = restarted
	app.setCurrentActorContext(actor)
	view, err := app.CloudBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if !view.Available || view.Account.Status != cloud.AccountAuthenticated || view.Account.ProfileID != actor.ProfileID || view.Account.CloudUserID != "usr_01" || view.Account.BindingGeneration != 1 {
		t.Fatalf("restarted account = %#v", view.Account)
	}
	if view.Balance == nil || view.Balance.TotalBalance != "21000.0000" {
		t.Fatalf("restarted balance = %#v", view.Balance)
	}
	if view.Subscription == nil || view.Subscription.Current.Plan != "plus" || len(view.Subscription.Plans) == 0 {
		t.Fatalf("restarted subscription = %#v", view.Subscription)
	}
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "test-password-01", "fixture_version", "scenario", filepath.Clean(provider(actor).MemoryUserDir())} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("CloudBootstrap exposed %q: %s", forbidden, payload)
		}
	}
}

func TestCloudPaymentPendingOrderRestoresAfterRuntimeRestart(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-restart"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())

	first, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	firstApp := NewApp()
	firstApp.cloudRuntime = first
	firstApp.setCurrentActorContext(actor)
	if _, err := firstApp.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		first.close()
		t.Fatal(err)
	}
	created, err := firstApp.CloudCreateSubscriptionOrder("plus", "alipay")
	if err != nil {
		first.close()
		t.Fatal(err)
	}
	if created.Payment.State != cloud.PaymentUIStatePaying || !created.Payment.Payable || created.Payment.PaymentEntry == nil {
		first.close()
		t.Fatalf("created payment = %#v", created)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_01", BindingGeneration: 1}
	before, found, err := cloud.NewDurablePaymentOrderRepository(provider).Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || before.Result.Order.Status != cloud.PaymentPending {
		first.close()
		t.Fatalf("durable pending before restart = %#v, %v, %v", before, found, err)
	}
	first.close()

	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.close)
	direct, found, err := cloud.NewDurablePaymentOrderRepository(provider).Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || direct.Result.Order.Status != cloud.PaymentPending {
		t.Fatalf("durable pending before bootstrap = %#v, %v, %v", direct, found, err)
	}
	restartedApp := NewApp()
	restartedApp.cloudRuntime = restarted
	restartedApp.setCurrentActorContext(actor)
	bootstrap, err := restartedApp.CloudBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if len(bootstrap.Payments) != 1 || bootstrap.Payments[0].OrderID != created.Payment.OrderID || bootstrap.Payments[0].State != cloud.PaymentUIStatePaying || !bootstrap.Payments[0].Payable {
		t.Fatalf("restarted payments = %#v", bootstrap.Payments)
	}
	after, found, err := cloud.NewDurablePaymentOrderRepository(provider).Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || after.Result.Order.Status != cloud.PaymentPending || after != direct {
		t.Fatalf("bootstrap advanced durable pending = %#v, %v, %v", after, found, err)
	}
}

func TestCloudPaymentCreateMatrixUsesCurrentActorAndSafeDTO(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime

	type createCase struct {
		operation string
		plan      string
		channel   string
		amount    string
		unit      cloud.AmountUnit
	}
	primary := userctx.ActorContext{ActorID: "local", ProfileID: "payment-primary"}
	secondary := userctx.ActorContext{ActorID: "local", ProfileID: "payment-secondary"}
	groups := []struct {
		actor    userctx.ActorContext
		phone    string
		password string
		userID   string
		cases    []createCase
	}{
		{primary, "13800000000", "test-password-01", "usr_01", []createCase{
			{"purchase", "lite", "alipay", "9.90", cloud.AmountUnitYuan},
			{"purchase", "lite", "wechat", "990", cloud.AmountUnitFen},
			{"purchase", "plus", "alipay", "29.90", cloud.AmountUnitYuan},
			{"purchase", "plus", "wechat", "2990", cloud.AmountUnitFen},
			{"purchase", "pro", "alipay", "59.90", cloud.AmountUnitYuan},
			{"purchase", "pro", "wechat", "5990", cloud.AmountUnitFen},
			{"upgrade", "pro", "alipay", "30.00", cloud.AmountUnitYuan},
			{"upgrade", "pro", "wechat", "3000", cloud.AmountUnitFen},
		}},
		{secondary, "13900000000", "test-password-02", "usr_02", []createCase{
			{"upgrade", "plus", "alipay", "20.00", cloud.AmountUnitYuan},
			{"upgrade", "plus", "wechat", "2000", cloud.AmountUnitFen},
			{"upgrade", "pro", "alipay", "50.00", cloud.AmountUnitYuan},
			{"upgrade", "pro", "wechat", "5000", cloud.AmountUnitFen},
		}},
	}
	createdCount := 0
	for _, group := range groups {
		app.setCurrentActorContext(group.actor)
		if _, err := app.CloudLogin(CloudLoginRequest{Phone: group.phone, Password: group.password}); err != nil {
			t.Fatalf("CloudLogin(%s): %v", group.actor.ProfileID, err)
		}
		for _, item := range group.cases {
			var got CloudPaymentOperationView
			if item.operation == "purchase" {
				got, err = app.CloudCreateSubscriptionOrder(item.plan, item.channel)
			} else {
				got, err = app.CloudCreateUpgradeOrder(item.plan, item.channel)
			}
			if err != nil {
				t.Fatalf("%s %s/%s: %v", item.operation, item.plan, item.channel, err)
			}
			createdCount++
			if got.Account.ProfileID != group.actor.ProfileID || got.Account.CloudUserID != group.userID || got.Account.BindingGeneration != 1 {
				t.Fatalf("%s %s/%s account = %#v", item.operation, item.plan, item.channel, got.Account)
			}
			payment := got.Payment
			if payment.Plan != item.plan || payment.Amount != item.amount || payment.AmountUnit != item.unit || payment.PaymentChannel != cloud.PaymentChannel(item.channel) ||
				payment.State != cloud.PaymentUIStatePaying || !payment.Payable || payment.PaymentEntry == nil || payment.EntitlementGranted {
				t.Fatalf("%s %s/%s payment = %#v", item.operation, item.plan, item.channel, payment)
			}
			if item.channel == "alipay" {
				if payment.PaymentEntry.EntryType != cloud.PaymentEntryQRCode || payment.PaymentEntry.QRCode == "" || payment.PaymentEntry.CodeURL != "" {
					t.Fatalf("Alipay entry = %#v", payment.PaymentEntry)
				}
			} else if payment.PaymentEntry.EntryType != cloud.PaymentEntryNativeQR || payment.PaymentEntry.CodeURL == "" || payment.PaymentEntry.QRCode != "" {
				t.Fatalf("WeChat entry = %#v", payment.PaymentEntry)
			}
			payload, err := json.Marshal(payment)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"scope", "profile_id", "cloud_user_id", "binding_generation", "idempotency", "entitlements_refreshed", "fixture", "scenario", "token", "path", "raw"} {
				if strings.Contains(string(payload), forbidden) {
					t.Fatalf("payment DTO exposed %q: %s", forbidden, payload)
				}
			}
		}
	}
	if createdCount != 12 {
		t.Fatalf("public payment creates = %d, want 12", createdCount)
	}
}

func TestCloudPaymentRefreshesAssetsOnlyAfterGrantedAndOnce(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	root := t.TempDir()
	provider := desktopCloudTestProfilePathProvider(root)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-grant"}
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		t.Fatal(err)
	}

	created, err := app.CloudCreateUpgradeOrder("plus", "alipay")
	if err != nil {
		t.Fatal(err)
	}
	if created.Balance != nil || created.Subscription != nil || created.Payment.State != cloud.PaymentUIStatePaying {
		t.Fatalf("created operation = %#v", created)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	initialBalance, found, err := runtime.balance.View(ctx, actor)
	if err != nil || !found || initialBalance.TotalBalance != "5000.0000" {
		t.Fatalf("initial balance = %#v, %v, %v", initialBalance, found, err)
	}
	initialSubscription, found, err := runtime.subscription.View(ctx, actor)
	if err != nil || !found || initialSubscription.Subscription.Plan != "lite" {
		t.Fatalf("initial subscription = %#v, %v, %v", initialSubscription, found, err)
	}
	beforeConfirm, found, err := runtime.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || beforeConfirm.Result.Order.Status != cloud.PaymentPending {
		t.Fatalf("pending record = %#v, %v, %v", beforeConfirm, found, err)
	}

	confirming, err := app.CloudConfirmPayment(created.Payment.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if confirming.Payment.State != cloud.PaymentUIStateConfirming || confirming.Payment.Payable || confirming.Payment.PaymentEntry != nil || confirming.Balance != nil || confirming.Subscription != nil {
		t.Fatalf("confirm operation = %#v", confirming)
	}
	afterConfirm, found, err := runtime.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || afterConfirm != beforeConfirm {
		t.Fatalf("confirm persisted transient = %#v, %v, %v", afterConfirm, found, err)
	}

	verifying, err := app.CloudRefreshPayment(created.Payment.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if verifying.Payment.State != cloud.PaymentUIStateConfirming || verifying.Payment.PaymentEntry != nil || verifying.Balance != nil || verifying.Subscription != nil {
		t.Fatalf("verifying operation = %#v", verifying)
	}
	unchangedBalance, _, err := runtime.balance.View(ctx, actor)
	if err != nil || unchangedBalance != initialBalance {
		t.Fatalf("verifying changed balance = %#v, %v", unchangedBalance, err)
	}
	unchangedSubscription, _, err := runtime.subscription.View(ctx, actor)
	if err != nil || !reflect.DeepEqual(unchangedSubscription, initialSubscription) {
		t.Fatalf("verifying changed subscription = %#v, %v", unchangedSubscription, err)
	}

	granted, err := app.CloudRefreshPayment(created.Payment.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if granted.Payment.State != cloud.PaymentUIStateSucceeded || !granted.Payment.EntitlementGranted || granted.Payment.PaymentEntry != nil ||
		granted.Balance == nil || granted.Balance.TotalBalance != "45000.0000" || granted.Subscription == nil || granted.Subscription.Current.Plan != "plus" {
		t.Fatalf("granted operation = %#v", granted)
	}
	marked, found, err := runtime.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || !marked.EntitlementsRefreshed {
		t.Fatalf("granted marker = %#v, %v, %v", marked, found, err)
	}
	assetPath := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "asset_snapshots.json")
	assetsAfterGrant, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}

	again, err := app.CloudRefreshPayment(created.Payment.OrderID)
	if err != nil || again.Payment.State != cloud.PaymentUIStateSucceeded || again.Balance == nil || again.Balance.TotalBalance != granted.Balance.TotalBalance {
		t.Fatalf("repeated granted = %#v, %v", again, err)
	}
	assetsAfterRepeat, err := os.ReadFile(assetPath)
	if err != nil || !reflect.DeepEqual(assetsAfterRepeat, assetsAfterGrant) {
		t.Fatalf("repeated granted rewrote assets: equal=%v err=%v", reflect.DeepEqual(assetsAfterRepeat, assetsAfterGrant), err)
	}
}

type paymentGrantedMaaSKeyClient struct {
	calls int
}

type maasRechargeTestClient struct {
	scope          cloud.BindingScope
	amount         string
	paymentMethod  string
	idempotencyKey string
	created        cloud.MaaSRechargeOrder
	queried        cloud.MaaSRechargeOrder
}

type maasRechargeBalanceTestClient struct {
	scope   cloud.BindingScope
	balance cloud.MaaSRechargeKeyBalance
	err     error
}

type maasBalanceTestClient struct {
	scope   cloud.BindingScope
	balance cloud.MaaSKeyBalance
	err     error
}

func (c *maasRechargeTestClient) Create(_ context.Context, scope cloud.BindingScope, amount, paymentMethod, idempotencyKey string) (cloud.MaaSRechargeOrder, error) {
	c.scope = scope
	c.amount = amount
	c.paymentMethod = paymentMethod
	c.idempotencyKey = idempotencyKey
	return c.created, nil
}

func (c *maasRechargeTestClient) Get(_ context.Context, scope cloud.BindingScope, orderID string) (cloud.MaaSRechargeOrder, error) {
	c.scope = scope
	if orderID != c.queried.OrderID {
		return cloud.MaaSRechargeOrder{}, errors.New("unexpected MaaS recharge order")
	}
	return c.queried, nil
}

func (c *maasRechargeBalanceTestClient) Get(_ context.Context, scope cloud.BindingScope) (cloud.MaaSRechargeKeyBalance, error) {
	c.scope = scope
	return c.balance, c.err
}

func (c *maasBalanceTestClient) Get(_ context.Context, scope cloud.BindingScope) (cloud.MaaSKeyBalance, error) {
	c.scope = scope
	return c.balance, c.err
}

func TestCloudMaaSRechargeUsesAuthenticatedBinding(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "maas-recharge"}.Normalize()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}

	pending := cloud.MaaSRechargeOrder{OrderID: "maasr-test", Amount: "100.00", Status: cloud.MaaSRechargePaymentPending, Version: 2}
	credited := pending
	credited.Status = cloud.MaaSRechargeCredited
	credited.Paid = true
	credited.Credited = true
	credited.Version = 7
	client := &maasRechargeTestClient{created: pending, queried: credited}
	packageBalanceClient := &maasBalanceTestClient{balance: cloud.MaaSKeyBalance{
		AvailableBalance: "120000.0000", RemainQuota: 4110000, Source: "maas_api_key",
		ConversionRuleVersion: "maas-quota-credit-v1", QuotaPerUSD: "500000", CNYPerUSD: "7.3", CreditPerCNY: "2000",
		QueriedAt: time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC),
	}}
	balanceClient := &maasRechargeBalanceTestClient{balance: cloud.MaaSRechargeKeyBalance{
		KeyName: "usr_01cz", AvailableBalance: "229200.0272", RemainQuota: 7849316, BalanceCNY: "114.6000",
		Source: "maas_recharge_api_key", ConversionRuleVersion: "maas-quota-credit-v1", CNYPerUSD: "7.3", QuotaPerUSD: "500000", CreditPerCNY: "2000",
		QueriedAt: time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC),
	}}
	runtime.maasRecharge = client
	runtime.maasBalance = packageBalanceClient
	runtime.maasRechargeBalance = balanceClient

	created, err := app.CloudCreateMaaSRechargeOrder("100.00", "alipay", "maas-attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	if created.OrderID != pending.OrderID || client.scope.ProfileID != actor.ProfileID || client.scope.CloudUserID != "usr_01" ||
		client.amount != "100.00" || client.paymentMethod != "alipay" || client.idempotencyKey != "maas-attempt-1" {
		t.Fatalf("created = %#v client = %#v", created, client)
	}
	queried, err := app.CloudGetMaaSRechargeOrder(created.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if queried.Status != cloud.MaaSRechargeCredited || !queried.Credited {
		t.Fatalf("queried = %#v", queried)
	}
	balance, err := app.CloudGetMaaSRechargeKeyBalance()
	if err != nil {
		t.Fatal(err)
	}
	if balance.KeyName != "usr_01cz" || balance.AvailableCredits != "229200.0272" || balance.PackageAvailableCredits != "120000.0000" ||
		balanceClient.scope != client.scope || packageBalanceClient.scope != client.scope {
		t.Fatalf("balance = %#v client = %#v", balance, balanceClient)
	}
}

func TestCloudMaaSRechargeBalanceTreatsMissingRechargeAccountAsZero(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "maas-no-recharge"}.Normalize()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runtime.maasBalance = &maasBalanceTestClient{balance: cloud.MaaSKeyBalance{AvailableBalance: "146000.0000"}}
	runtime.maasRechargeBalance = &maasRechargeBalanceTestClient{balance: cloud.MaaSRechargeKeyBalance{AvailableBalance: "0.0000", NotFound: true}}

	balance, err := app.CloudGetMaaSRechargeKeyBalance()
	if err != nil {
		t.Fatal(err)
	}
	if balance.AvailableCredits != "0.0000" || balance.PackageAvailableCredits != "146000.0000" {
		t.Fatalf("balance = %#v", balance)
	}
}

func TestCloudMaaSRechargeBalancePreservesCombinedQueryFailures(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "maas-balance-error"}.Normalize()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	runtime.maasBalance = &maasBalanceTestClient{balance: cloud.MaaSKeyBalance{AvailableBalance: "146000.0000"}}
	runtime.maasRechargeBalance = &maasRechargeBalanceTestClient{err: &cloud.Error{Code: cloud.ErrorNetworkUnavailable, Message: "network down", Retryable: true}}

	_, err = app.CloudGetMaaSRechargeKeyBalance()
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorNetworkUnavailable {
		t.Fatalf("error = %#v", err)
	}
}

func TestMaaSQuotaCreditsUsesExactCloudRate(t *testing.T) {
	credits, err := maasQuotaCredits(6849316, "500000", "7.3", "2000")
	if err != nil {
		t.Fatal(err)
	}
	if credits != "200000.0272" {
		t.Fatalf("credits = %q, want 200000.0272", credits)
	}
	remaining, err := subtractCredits("349200.0272", credits)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != "149200.0000" {
		t.Fatalf("remaining = %q, want 149200.0000", remaining)
	}
}

func (c *paymentGrantedMaaSKeyClient) Get(ctx context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
	if err := ctx.Err(); err != nil {
		return cloud.MaaSKeyAccount{}, err
	}
	c.calls++
	keyID := int64(9)
	return cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, KeyID: &keyID, Key: "sk-payment-granted",
	}, nil
}

func TestCloudGrantedPaymentRefreshesMaaSKeyAndCatalog(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	ctx := context.Background()
	root := t.TempDir()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-maas", WorkspaceRoot: root}.Normalize()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	app.billingWorkerRuntime.factory = func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(workerCtx context.Context) error {
			<-workerCtx.Done()
			return nil
		}}, nil
	}
	app.billingWorkerRuntime.start(ctx)
	t.Cleanup(func() { _ = app.billingWorkerRuntime.close() })
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	if err := cfg.SaveTo(config.NewProfilePathResolver(actor).UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	tab := app.createTabEntryWithIDAndActorForRoute("project", root, "", "kwu/gpt-5.4", actor, "")
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	maasKeys := &paymentGrantedMaaSKeyClient{}
	runtime.maasKeys = maasKeys
	fetchCalls := 0
	app.subscriptionModels.fetch = func(_ context.Context, _ netclient.ProxySpec, key string) ([]string, error) {
		fetchCalls++
		if key != "sk-payment-granted" {
			t.Fatalf("MaaS key passed to catalog = %q", key)
		}
		return []string{"gpt-5.4"}, nil
	}

	// The fixture payment flow is reused here, while the post-grant refresh
	// runs through the same non-fixture MaaS path as the real runtime.
	t.Setenv("KWU_CLOUD_FAKE", "")
	created, err := app.CloudCreateSubscriptionOrder("lite", "alipay")
	if err != nil {
		t.Fatal(err)
	}
	if maasKeys.calls != 0 || fetchCalls != 0 {
		t.Fatalf("pending payment refreshed MaaS: key=%d catalog=%d", maasKeys.calls, fetchCalls)
	}
	if _, err := app.CloudRefreshPayment(created.Payment.OrderID); err != nil {
		t.Fatal(err)
	}
	if maasKeys.calls != 0 || fetchCalls != 0 {
		t.Fatalf("confirming payment refreshed MaaS: key=%d catalog=%d", maasKeys.calls, fetchCalls)
	}
	granted, err := app.CloudRefreshPayment(created.Payment.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if !granted.Payment.EntitlementGranted || maasKeys.calls != 1 || fetchCalls != 1 {
		t.Fatalf("granted payment refresh = granted:%t key:%d catalog:%d", granted.Payment.EntitlementGranted, maasKeys.calls, fetchCalls)
	}
}

func TestCloudPaymentSafeErrorsAndExpiredEntryAreRedacted(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-errors"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		t.Fatal(err)
	}

	assertSafeCode := func(err error, code cloud.ErrorCode, message string) {
		t.Helper()
		var cloudErr *cloud.Error
		if !errors.As(err, &cloudErr) || cloudErr.Code != code || cloudErr.Message != message || cloudErr.Details != nil || cloudErr.Retryable {
			t.Fatalf("safe error = %#v, want %s/%q", err, code, message)
		}
	}
	_, err = app.CloudCreateUpgradeOrder("lite", "alipay")
	assertSafeCode(err, cloud.ErrorUpgradeNotAllowed, "subscription upgrade is not allowed")

	created, err := app.CloudCreateUpgradeOrder("plus", "alipay")
	if err != nil {
		t.Fatal(err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	record, found, err := runtime.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found {
		t.Fatalf("created record = %#v, %v, %v", record, found, err)
	}
	expiredOrderID := "test-only-expired-order"
	expiredResult := record.Result
	expiredResult.Order.OrderID = expiredOrderID
	expiredResult.Order.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	expiredResult.Entry.ExpiresAt = expiredResult.Order.ExpiresAt
	expiredRecord, err := cloud.NewPaymentOrderRecord(
		scope,
		expiredResult,
		record.Operation,
		"test-only-expired-idempotency-key",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.paymentOrders.Save(ctx, expiredRecord); err != nil {
		t.Fatal(err)
	}
	_, err = app.CloudConfirmPayment(expiredOrderID)
	assertSafeCode(err, cloud.ErrorPaymentNotVerified, "payment has not been verified")
	bootstrap, err := app.CloudBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	var expired *CloudPaymentView
	for index := range bootstrap.Payments {
		if bootstrap.Payments[index].OrderID == expiredOrderID {
			expired = &bootstrap.Payments[index]
			break
		}
	}
	if expired == nil || expired.Payable || expired.PaymentEntry != nil {
		t.Fatalf("expired payment DTO = %#v", expired)
	}

	granted, err := app.CloudCreateSubscriptionOrder("pro", "alipay")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloudRefreshPayment(granted.Payment.OrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloudRefreshPayment(granted.Payment.OrderID); err != nil {
		t.Fatal(err)
	}
	_, err = app.CloudConfirmPayment(granted.Payment.OrderID)
	assertSafeCode(err, cloud.ErrorOrderAlreadyGranted, "payment entitlements are already granted")
}

func TestCloudPaymentOperationRejectsMissingGrantedAssets(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-missing-assets"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		t.Fatal(err)
	}
	created, err := runtime.payment.CreateUpgradeOrder(ctx, actor, "plus", cloud.PaymentChannelAlipay)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.payment.RefreshOrder(ctx, actor, created.Result.Order.OrderID); err != nil {
		t.Fatal(err)
	}
	granted, err := runtime.payment.RefreshOrder(ctx, actor, created.Result.Order.OrderID)
	if err != nil || !granted.EntitlementGranted {
		t.Fatalf("granted domain view = %#v, %v", granted, err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	if err := runtime.assets.Clear(ctx, scope); err != nil {
		t.Fatal(err)
	}
	view, err := app.cloudPaymentOperationForActor(runtime, actor, granted)
	if err == nil {
		t.Fatalf("missing granted assets returned successful zero DTO: %#v", view)
	}
}

func TestCloudPaymentFixedFailureKeepsAssets(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-fixed-failure"}
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(actor)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		t.Fatal(err)
	}
	baseBalance, _, err := runtime.balance.View(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	baseSubscription, _, err := runtime.subscription.View(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	created, err := app.CloudCreateSubscriptionOrder("lite", "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.CloudRefreshPayment(created.Payment.OrderID); err != nil {
		t.Fatal(err)
	}
	failed, err := app.CloudRefreshPayment(created.Payment.OrderID)
	if err != nil || failed.Payment.State != cloud.PaymentUIStateFailed || failed.Payment.EntitlementGranted || failed.Balance != nil || failed.Subscription != nil {
		t.Fatalf("fixed failure = %#v, %v", failed, err)
	}
	afterBalance, _, err := runtime.balance.View(ctx, actor)
	if err != nil || afterBalance != baseBalance {
		t.Fatalf("failure changed balance = %#v, %v", afterBalance, err)
	}
	afterSubscription, _, err := runtime.subscription.View(ctx, actor)
	if err != nil || !reflect.DeepEqual(afterSubscription, baseSubscription) {
		t.Fatalf("failure changed subscription = %#v, %v", afterSubscription, err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	record, found, err := runtime.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || record.Result.Order.Status != cloud.PaymentFailed || record.EntitlementsRefreshed {
		t.Fatalf("failed durable record = %#v, %v, %v", record, found, err)
	}
}

func TestCloudPaymentGrantedOrderRestoresAfterRuntimeRestart(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-granted-restart"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())
	first, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	firstApp := NewApp()
	firstApp.cloudRuntime = first
	firstApp.setCurrentActorContext(actor)
	if _, err := firstApp.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		first.close()
		t.Fatal(err)
	}
	created, err := firstApp.CloudCreateUpgradeOrder("plus", "alipay")
	if err != nil {
		first.close()
		t.Fatal(err)
	}
	if _, err := firstApp.CloudRefreshPayment(created.Payment.OrderID); err != nil {
		first.close()
		t.Fatal(err)
	}
	if _, err := firstApp.CloudRefreshPayment(created.Payment.OrderID); err != nil {
		first.close()
		t.Fatal(err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	beforeOrder, found, err := cloud.NewDurablePaymentOrderRepository(provider).Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || !beforeOrder.EntitlementsRefreshed || beforeOrder.Result.Order.Status != cloud.PaymentGranted {
		first.close()
		t.Fatalf("granted order before restart = %#v, %v, %v", beforeOrder, found, err)
	}
	beforeBalance, found, err := cloud.NewDurableAssetSnapshotRepository(provider).LoadBalance(ctx, scope)
	if err != nil || !found || beforeBalance.TotalBalance != "45000.0000" {
		first.close()
		t.Fatalf("granted balance before restart = %#v, %v, %v", beforeBalance, found, err)
	}
	beforeSubscription, found, err := cloud.NewDurableAssetSnapshotRepository(provider).LoadSubscription(ctx, scope)
	if err != nil || !found || beforeSubscription.Subscription.Plan != "plus" {
		first.close()
		t.Fatalf("granted subscription before restart = %#v, %v, %v", beforeSubscription, found, err)
	}
	first.close()

	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.close)
	directOrder, found, err := restarted.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || directOrder != beforeOrder {
		t.Fatalf("direct order before bootstrap = %#v, %v, %v", directOrder, found, err)
	}
	directBalance, found, err := restarted.assets.LoadBalance(ctx, scope)
	if err != nil || !found || directBalance != beforeBalance {
		t.Fatalf("direct balance before bootstrap = %#v, %v, %v", directBalance, found, err)
	}
	directSubscription, found, err := restarted.assets.LoadSubscription(ctx, scope)
	if err != nil || !found || !reflect.DeepEqual(directSubscription, beforeSubscription) {
		t.Fatalf("direct subscription before bootstrap = %#v, %v, %v", directSubscription, found, err)
	}
	restartedApp := NewApp()
	restartedApp.cloudRuntime = restarted
	restartedApp.setCurrentActorContext(actor)
	bootstrap, err := restartedApp.CloudBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if bootstrap.Balance == nil || bootstrap.Balance.TotalBalance != "45000.0000" || bootstrap.Subscription == nil || bootstrap.Subscription.Current.Plan != "plus" || len(bootstrap.Payments) != 1 ||
		bootstrap.Payments[0].OrderID != created.Payment.OrderID || bootstrap.Payments[0].State != cloud.PaymentUIStateSucceeded || !bootstrap.Payments[0].EntitlementGranted {
		t.Fatalf("granted bootstrap = %#v", bootstrap)
	}
	afterOrder, found, err := restarted.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || afterOrder != beforeOrder {
		t.Fatalf("bootstrap changed granted marker/order = %#v, %v, %v", afterOrder, found, err)
	}
}

func TestCloudPaymentBootstrapCompletesGrantedCrashWindowOnce(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "payment-crash-window"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())
	first, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	firstApp := NewApp()
	firstApp.cloudRuntime = first
	firstApp.setCurrentActorContext(actor)
	if _, err := firstApp.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		first.close()
		t.Fatal(err)
	}
	created, err := firstApp.CloudCreateUpgradeOrder("plus", "alipay")
	if err != nil {
		first.close()
		t.Fatal(err)
	}
	if _, err := firstApp.CloudRefreshPayment(created.Payment.OrderID); err != nil {
		first.close()
		t.Fatal(err)
	}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	record, found, err := first.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || record.Result.Order.Status != cloud.PaymentVerifying {
		first.close()
		t.Fatalf("verifying crash record = %#v, %v, %v", record, found, err)
	}
	grantedOrder, err := first.paymentClient.GetPaymentOrder(ctx, scope, created.Payment.OrderID)
	if err != nil || grantedOrder.Status != cloud.PaymentGranted {
		first.close()
		t.Fatalf("canonical granted fact = %#v, %v", grantedOrder, err)
	}
	record.Result.Order = grantedOrder
	if err := first.paymentOrders.Save(ctx, record); err != nil {
		first.close()
		t.Fatal(err)
	}
	oldBalance, found, err := first.assets.LoadBalance(ctx, scope)
	if err != nil || !found || oldBalance.TotalBalance != "5000.0000" {
		first.close()
		t.Fatalf("crash-window old balance = %#v, %v, %v", oldBalance, found, err)
	}
	first.close()

	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.close)
	before, found, err := restarted.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || before.EntitlementsRefreshed || before.Result.Order.Status != cloud.PaymentGranted {
		t.Fatalf("crash-window durable order before bootstrap = %#v, %v, %v", before, found, err)
	}
	restartedApp := NewApp()
	restartedApp.cloudRuntime = restarted
	restartedApp.setCurrentActorContext(actor)
	bootstrap, err := restartedApp.CloudBootstrap()
	if err != nil || bootstrap.Balance == nil || bootstrap.Balance.TotalBalance != "45000.0000" || bootstrap.Subscription == nil || bootstrap.Subscription.Current.Plan != "plus" {
		t.Fatalf("crash-window bootstrap = %#v, %v", bootstrap, err)
	}
	marked, found, err := restarted.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || !marked.EntitlementsRefreshed {
		t.Fatalf("crash-window marker = %#v, %v, %v", marked, found, err)
	}
	assetsAfterFirst, err := os.ReadFile(filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "asset_snapshots.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restartedApp.CloudBootstrap(); err != nil {
		t.Fatal(err)
	}
	assetsAfterSecond, err := os.ReadFile(filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "asset_snapshots.json"))
	if err != nil || !reflect.DeepEqual(assetsAfterSecond, assetsAfterFirst) {
		t.Fatalf("second bootstrap changed authoritative assets: equal=%v err=%v", reflect.DeepEqual(assetsAfterSecond, assetsAfterFirst), err)
	}
	again, found, err := restarted.paymentOrders.Get(ctx, scope, created.Payment.OrderID)
	if err != nil || !found || again != marked {
		t.Fatalf("second bootstrap changed marker/order = %#v, %v, %v", again, found, err)
	}
}

func TestCloudPaymentOrdersAreHiddenAcrossActorScopes(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)
	app := NewApp()
	app.cloudRuntime = runtime
	owner := userctx.ActorContext{ActorID: "local", ProfileID: "payment-owner"}
	other := userctx.ActorContext{ActorID: "local", ProfileID: "payment-other"}
	app.setCurrentActorContext(owner)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"}); err != nil {
		t.Fatal(err)
	}
	created, err := app.CloudCreateSubscriptionOrder("pro", "wechat")
	if err != nil {
		t.Fatal(err)
	}
	app.setCurrentActorContext(other)
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	otherView, err := app.CloudBootstrap()
	if err != nil || len(otherView.Payments) != 0 {
		t.Fatalf("other scope payments = %#v, %v", otherView.Payments, err)
	}
	app.setCurrentActorContext(owner)
	ownerView, err := app.CloudBootstrap()
	if err != nil || len(ownerView.Payments) != 1 || ownerView.Payments[0].OrderID != created.Payment.OrderID {
		t.Fatalf("owner scope payments = %#v, %v", ownerView.Payments, err)
	}
}

func TestCloudLoginSecondaryAccountProjectsAssetsAndRestoresAfterRestart(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "secondary-fresh"}
	provider := desktopCloudTestProfilePathProvider(t.TempDir())

	first, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = first
	app.setCurrentActorContext(actor)
	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		first.close()
		t.Fatalf("secondary CloudLogin: %v", err)
	}
	if loggedIn.Account.Status != cloud.AccountAuthenticated || loggedIn.Account.CloudUserID != "usr_02" || loggedIn.Account.BindingGeneration != 1 {
		first.close()
		t.Fatalf("secondary account = %#v, want authenticated usr_02 generation 1", loggedIn.Account)
	}
	if loggedIn.Balance == nil || loggedIn.Balance.TotalBalance != "5000.0000" {
		first.close()
		t.Fatalf("secondary balance = %#v, want canonical secondary balance", loggedIn.Balance)
	}
	if loggedIn.Subscription == nil || loggedIn.Subscription.Current.Plan != "lite" || len(loggedIn.Subscription.Plans) == 0 {
		first.close()
		t.Fatalf("secondary subscription = %#v, want canonical lite plan and plan list", loggedIn.Subscription)
	}
	first.close()

	restarted, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.close)
	restoredBalance, found, err := restarted.balance.View(ctx, actor)
	if err != nil || !found || restoredBalance.TotalBalance != "5000.0000" {
		t.Fatalf("restarted secondary balance before remote refresh = %#v, found %v, err %v", restoredBalance, found, err)
	}
	restoredSubscription, found, err := restarted.subscription.View(ctx, actor)
	if err != nil || !found || restoredSubscription.Subscription.Plan != "lite" || len(restoredSubscription.Plans) == 0 {
		t.Fatalf("restarted secondary subscription before remote refresh = %#v, found %v, err %v", restoredSubscription, found, err)
	}
	restartedApp := NewApp()
	restartedApp.cloudRuntime = restarted
	restartedApp.setCurrentActorContext(actor)
	restored, err := restartedApp.CloudBootstrap()
	if err != nil {
		t.Fatalf("secondary CloudBootstrap after restart: %v", err)
	}
	if restored.Account.Status != cloud.AccountAuthenticated || restored.Account.CloudUserID != "usr_02" || restored.Account.BindingGeneration != 1 {
		t.Fatalf("restored secondary account = %#v", restored.Account)
	}
	if restored.Balance == nil || restored.Balance.TotalBalance != "5000.0000" {
		t.Fatalf("restored secondary balance = %#v", restored.Balance)
	}
	if restored.Subscription == nil || restored.Subscription.Current.Plan != "lite" || len(restored.Subscription.Plans) == 0 {
		t.Fatalf("restored secondary subscription = %#v", restored.Subscription)
	}
}

func TestCloudRegisterRequiresCaptchaForSMSAndNeverPersistsPlaintextPassword(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	root := t.TempDir()
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(root))
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(userctx.ActorContext{ActorID: "local", ProfileID: "default"})
	bootstrap, err := app.CloudBootstrap()
	if err != nil {
		t.Fatal(err)
	}
	if !bootstrap.Capabilities.Registration || bootstrap.Capabilities.RegistrationMode != string(cloudRegistrationFixture) {
		t.Fatalf("fixture registration capabilities = %#v", bootstrap.Capabilities)
	}

	err = app.CloudSendRegisterSMS(CloudSendRegisterSMSRequest{Phone: "13800000000"})
	var cloudErr *cloud.Error
	if !errors.As(err, &cloudErr) || cloudErr.Code != cloud.ErrorSliderCaptchaRequired || cloudErr.Message != "captcha verification is required" {
		t.Fatalf("CloudSendRegisterSMS without captcha error = %#v, want safe slider code", err)
	}
	if calls := runtime.accounts.SendRegisterSMSCalls(); calls != 0 {
		t.Fatalf("SMS calls without captcha = %d", calls)
	}
	if err := app.CloudSendRegisterSMS(CloudSendRegisterSMSRequest{Phone: "13800000000", CaptchaPassed: true}); err != nil {
		t.Fatal(err)
	}
	if calls := runtime.accounts.SendRegisterSMSCalls(); calls != 1 {
		t.Fatalf("SMS calls after captcha = %d, want 1", calls)
	}

	plaintext := "registration-password-must-not-persist"
	view, err := app.CloudRegister(CloudRegisterRequest{
		Phone: "13800000000", Password: plaintext, SMSCode: "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Account.Status != cloud.AccountAuthenticated || view.Account.CloudUserID != "usr_01" || view.Balance == nil || view.Subscription == nil {
		t.Fatalf("CloudRegister view = %#v", view)
	}
	if calls := runtime.accounts.RegisterCalls(); calls != 1 {
		t.Fatalf("register calls = %d, want 1", calls)
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), plaintext) {
			t.Fatalf("plaintext password persisted in %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCloudLoginRefreshCandidateAndLogoutUseCurrentActorOnly(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(userctx.ActorContext{ActorID: "local", ProfileID: "default"})

	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.Account.Status != cloud.AccountAuthenticated || loggedIn.Account.CloudUserID != "usr_01" || loggedIn.Balance == nil || loggedIn.Subscription == nil {
		t.Fatalf("CloudLogin view = %#v", loggedIn)
	}
	refreshed, err := app.CloudRefreshAssets()
	if err != nil || refreshed.Balance == nil || refreshed.Balance.TotalBalance != loggedIn.Balance.TotalBalance {
		t.Fatalf("CloudRefreshAssets = %#v, %v", refreshed, err)
	}

	candidate, err := app.CloudLogin(CloudLoginRequest{Phone: "13900000000", Password: "test-password-02"})
	if err != nil {
		t.Fatalf("candidate CloudLogin error = %v, want successful Wails envelope", err)
	}
	if candidate.OperationError == nil || candidate.OperationError.Code != cloud.ErrorProfileScopeMismatch || candidate.OperationError.Message != "account belongs to another profile" {
		t.Fatalf("candidate operation error = %#v", candidate.OperationError)
	}
	if candidate.Candidate == nil || candidate.Candidate.CloudUserID != "usr_02" || candidate.Candidate.BindingGeneration != 2 {
		t.Fatalf("candidate identity = %#v", candidate.Candidate)
	}
	if candidate.Account.CloudUserID != "usr_01" || candidate.Account.BindingGeneration != 1 || candidate.Balance == nil || candidate.Subscription == nil {
		t.Fatalf("current account/assets after candidate = %#v", candidate)
	}

	payload, err := json.Marshal(struct {
		LoggedIn  CloudBootstrapView `json:"logged_in"`
		Candidate CloudBootstrapView `json:"candidate"`
	}{LoggedIn: loggedIn, Candidate: candidate})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "test-password-01", "test-password-02", "fixture_version", "scenario"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("cloud API exposed %q: %s", forbidden, payload)
		}
	}

	loggedOut, err := app.CloudLogout()
	if err != nil {
		t.Fatal(err)
	}
	if loggedOut.Account.Status != cloud.AccountAnonymous || loggedOut.Balance != nil || loggedOut.Subscription != nil {
		t.Fatalf("CloudLogout view = %#v", loggedOut)
	}
}

func TestCloudAuthenticationErrorPreservesCandidateWhenCurrentHydrationFails(t *testing.T) {
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = runtime
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	app.setCurrentActorContext(actor)

	primary, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatal(err)
	}
	primaryScope, err := cloud.NewBindingScope(actor, primary.Account.CloudUserID, primary.Account.BindingGeneration)
	if err != nil {
		t.Fatal(err)
	}
	session, found, err := runtime.sessionStore.Load(context.Background(), primaryScope)
	if err != nil || !found {
		t.Fatalf("primary session = %#v, found %t, err %v", session, found, err)
	}
	session.Scope.BindingGeneration++
	mismatchedStore := desktopMismatchedSessionStore{CloudSessionStore: runtime.sessionStore, session: session}
	sessions := cloud.NewProfileSessionRepository(runtime.bindings, mismatchedStore)
	captcha, err := cloud.NewMockCaptchaGateway("candidate-fallback", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	runtime.auth, err = cloud.NewAuthApplication(cloud.AuthApplicationDependencies{
		Bindings: runtime.bindings, Sessions: sessions,
		Cleanup:        cloud.NewIdentityCleanup(mismatchedStore, runtime.eventState, runtime.assets),
		Captcha:        captcha,
		Encryptor:      cloud.NewTestPasswordEncryptor(),
		Client:         runtime.accounts,
		FixtureVersion: cloud.A0FixtureVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	candidateBinding := cloud.ProfileBinding{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: "usr_02",
		Status: cloud.BindingBound, BindingGeneration: primary.Account.BindingGeneration + 1,
	}
	candidate := cloud.AuthView{
		State: cloud.AccountOffline, Binding: candidateBinding,
		User: cloud.CloudUser{CloudUserID: "usr_02", DisplayName: "Candidate"}, LastSyncAt: time.Now().UTC(),
	}
	view, err := app.cloudAuthenticationError("cloud.test", runtime, actor, candidate, &cloud.Error{
		Code: cloud.ErrorProfileScopeMismatch, Message: "account belongs to another profile",
	})
	if err != nil {
		t.Fatalf("cloudAuthenticationError: %v", err)
	}
	if view.OperationError == nil || view.OperationError.Code != cloud.ErrorProfileScopeMismatch {
		t.Fatalf("operation error = %#v", view.OperationError)
	}
	if view.Account.CloudUserID != primary.Account.CloudUserID || view.Account.Status != cloud.AccountOffline {
		t.Fatalf("fallback current account = %#v", view.Account)
	}
	if view.Candidate == nil || view.Candidate.CloudUserID != "usr_02" || view.Candidate.BindingGeneration != 2 {
		t.Fatalf("fallback candidate = %#v", view.Candidate)
	}
}

type desktopMismatchedSessionStore struct {
	cloud.CloudSessionStore
	session cloud.Session
}

func (s desktopMismatchedSessionStore) Load(context.Context, cloud.BindingScope) (cloud.Session, bool, error) {
	return s.session, true, nil
}

func TestCloudAPIsUseActiveTabActorWithoutTestSetter(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	ctx := context.Background()
	root := t.TempDir()
	provider := desktopCloudTestProfilePathProvider(root)
	runtime, err := newCloudRuntime(provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.close)

	defaultActor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}.Normalize()
	if _, err := runtime.auth.Login(ctx, defaultActor, cloud.LoginInput{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.balance.Refresh(ctx, defaultActor); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.subscription.Refresh(ctx, defaultActor); err != nil {
		t.Fatal(err)
	}
	defaultFiles := map[string][]byte{}
	for _, name := range []string{"binding.json", "sessions.json", "asset_snapshots.json"} {
		path := filepath.Join(provider(defaultActor).MemoryUserDir(), "cloud", "v1", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read default %s: %v", name, err)
		}
		defaultFiles[name] = data
	}

	activeActor := userctx.ActorContext{
		ActorID: "local", ProfileID: "work", AuthMethod: "local", WorkspaceRoot: filepath.Join(root, "workspace"),
	}.Normalize()
	app := NewApp()
	app.cloudRuntime = runtime
	tab := &WorkspaceTab{ID: "cloud-active", Scope: "project", WorkspaceRoot: activeActor.WorkspaceRoot, actor: activeActor}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.activeTabID = tab.ID

	loggedIn, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
	if err != nil {
		t.Fatalf("CloudLogin with active-tab actor error = %v", err)
	}
	if loggedIn.Account.ProfileID != activeActor.ProfileID || loggedIn.Account.CloudUserID != "usr_01" || loggedIn.Account.BindingGeneration != 1 {
		t.Fatalf("CloudLogin account = %#v, want active-tab work/usr_01/generation 1", loggedIn.Account)
	}
	activeBindingPath := filepath.Join(provider(activeActor).MemoryUserDir(), "cloud", "v1", "binding.json")
	if _, err := os.Stat(activeBindingPath); err != nil {
		t.Fatalf("active profile durable binding %q: %v", activeBindingPath, err)
	}
	workScope := cloud.BindingScope{ProfileID: activeActor.ProfileID, CloudUserID: "usr_01", BindingGeneration: 1}
	if _, ok, err := runtime.sessionStore.Load(ctx, workScope); err != nil || !ok {
		t.Fatalf("active profile durable session = ok %v, err %v", ok, err)
	}
	if _, ok, err := runtime.assets.LoadBalance(ctx, workScope); err != nil || !ok {
		t.Fatalf("active profile durable balance = ok %v, err %v", ok, err)
	}
	if _, ok, err := runtime.assets.LoadSubscription(ctx, workScope); err != nil || !ok {
		t.Fatalf("active profile durable subscription = ok %v, err %v", ok, err)
	}
	for name, before := range defaultFiles {
		path := filepath.Join(provider(defaultActor).MemoryUserDir(), "cloud", "v1", name)
		after, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(after, before) {
			t.Fatalf("default %s changed after active login: err %v", name, err)
		}
	}
	defaultState, err := runtime.auth.State(ctx, defaultActor)
	if err != nil || defaultState.State != cloud.AccountAuthenticated || defaultState.Binding.CloudUserID != "usr_01" {
		t.Fatalf("default account after active login = %#v, %v", defaultState, err)
	}
	payment, err := app.CloudCreateSubscriptionOrder("pro", "alipay")
	if err != nil || payment.Account.ProfileID != activeActor.ProfileID {
		t.Fatalf("active-tab payment = %#v, %v", payment, err)
	}
	activePayment, found, err := runtime.paymentOrders.Get(ctx, workScope, payment.Payment.OrderID)
	if err != nil || !found || activePayment.Scope != workScope {
		t.Fatalf("active-tab durable payment = %#v, %v, %v", activePayment, found, err)
	}
	defaultScope := cloud.BindingScope{ProfileID: defaultActor.ProfileID, CloudUserID: "usr_01", BindingGeneration: 1}
	if _, found, err := runtime.paymentOrders.Get(ctx, defaultScope, payment.Payment.OrderID); err != nil || found {
		t.Fatalf("active payment leaked to default scope: found=%v err=%v", found, err)
	}

	loggedOut, err := app.CloudLogout()
	if err != nil {
		t.Fatalf("CloudLogout with active-tab actor error = %v", err)
	}
	if loggedOut.Account.ProfileID != activeActor.ProfileID || loggedOut.Account.Status != cloud.AccountAnonymous {
		t.Fatalf("CloudLogout account = %#v, want anonymous active profile", loggedOut.Account)
	}
	if _, ok, err := runtime.sessionStore.Load(ctx, workScope); err != nil || ok {
		t.Fatalf("active profile session after logout = ok %v, err %v", ok, err)
	}
	if _, ok, err := runtime.assets.LoadBalance(ctx, workScope); err != nil || ok {
		t.Fatalf("active profile balance after logout = ok %v, err %v", ok, err)
	}
	if _, ok, err := runtime.assets.LoadSubscription(ctx, workScope); err != nil || ok {
		t.Fatalf("active profile subscription after logout = ok %v, err %v", ok, err)
	}
	if persisted, found, err := runtime.paymentOrders.Get(ctx, workScope, payment.Payment.OrderID); err != nil || !found || persisted != activePayment {
		t.Fatalf("logout changed payment recovery record = %#v, %v, %v", persisted, found, err)
	}
	for name, before := range defaultFiles {
		path := filepath.Join(provider(defaultActor).MemoryUserDir(), "cloud", "v1", name)
		after, err := os.ReadFile(path)
		if err != nil || !reflect.DeepEqual(after, before) {
			t.Fatalf("default %s changed after active logout: err %v", name, err)
		}
	}
	defaultState, err = runtime.auth.State(ctx, defaultActor)
	if err != nil || defaultState.State != cloud.AccountAuthenticated || defaultState.Binding.CloudUserID != "usr_01" {
		t.Fatalf("default account after active logout = %#v, %v", defaultState, err)
	}
}

func TestCloudWailsSurfaceHasNoCallerControlledIdentityOrFixtureFields(t *testing.T) {
	typeOfApp := reflect.TypeOf((*App)(nil))
	var cloudMethods []string
	for index := 0; index < typeOfApp.NumMethod(); index++ {
		name := typeOfApp.Method(index).Name
		if strings.HasPrefix(name, "Cloud") {
			cloudMethods = append(cloudMethods, name)
		}
	}
	sort.Strings(cloudMethods)
	want := []string{
		"CloudBootstrap", "CloudCachedAssets", "CloudCaptchaConfig", "CloudConfirmPayment", "CloudCreateMaaSRechargeOrder", "CloudCreateSubscriptionOrder", "CloudCreateUpgradeOrder", "CloudGetMaaSRechargeKeyBalance", "CloudGetMaaSRechargeOrder", "CloudLogin",
		"CloudLogout", "CloudRefreshAssets", "CloudRefreshPayment", "CloudRegister", "CloudResetPassword", "CloudSendPasswordResetSMS", "CloudSendRegisterSMS", "CloudSwitchAccount",
	}
	if !reflect.DeepEqual(cloudMethods, want) {
		t.Fatalf("Cloud Wails methods = %v, want %v", cloudMethods, want)
	}
	for _, requestType := range []reflect.Type{
		reflect.TypeOf(CloudLoginRequest{}),
		reflect.TypeOf(CloudRegisterRequest{}),
		reflect.TypeOf(CloudResetPasswordRequest{}),
		reflect.TypeOf(CloudSendPasswordResetSMSRequest{}),
		reflect.TypeOf(CloudSendRegisterSMSRequest{}),
	} {
		for index := 0; index < requestType.NumField(); index++ {
			field := requestType.Field(index)
			name := strings.ToLower(field.Name + " " + field.Tag.Get("json"))
			for _, forbidden := range []string{"actor", "profile", "cloud_user", "generation", "path", "fixture", "scenario", "token"} {
				if strings.Contains(name, forbidden) {
					t.Fatalf("%s exposes caller-controlled %q field %s", requestType, forbidden, field.Name)
				}
			}
		}
	}
	for name, inputs := range map[string]int{
		"CloudCreateMaaSRechargeOrder":   3,
		"CloudGetMaaSRechargeKeyBalance": 0,
		"CloudCreateSubscriptionOrder":   2,
		"CloudCreateUpgradeOrder":        2,
		"CloudConfirmPayment":            1,
		"CloudGetMaaSRechargeOrder":      1,
		"CloudRefreshPayment":            1,
	} {
		method, ok := typeOfApp.MethodByName(name)
		if !ok || method.Type.NumIn() != inputs+1 || method.Type.NumOut() != 2 {
			t.Fatalf("%s signature = %#v", name, method.Type)
		}
		for index := 1; index < method.Type.NumIn(); index++ {
			if method.Type.In(index).Kind() != reflect.String {
				t.Fatalf("%s input %d = %v, want string business input", name, index, method.Type.In(index))
			}
		}
	}
	switchMethod, ok := typeOfApp.MethodByName("CloudSwitchAccount")
	if !ok || switchMethod.Type.NumIn() != 2 || switchMethod.Type.In(1) != reflect.TypeOf(CloudLoginRequest{}) || switchMethod.Type.NumOut() != 2 {
		t.Fatalf("CloudSwitchAccount signature = %#v", switchMethod.Type)
	}
	assertJSONFields := func(value any, expected []string) {
		t.Helper()
		typeOfValue := reflect.TypeOf(value)
		actual := make([]string, 0, typeOfValue.NumField())
		for index := 0; index < typeOfValue.NumField(); index++ {
			name := strings.Split(typeOfValue.Field(index).Tag.Get("json"), ",")[0]
			actual = append(actual, name)
		}
		sort.Strings(actual)
		sort.Strings(expected)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s JSON fields = %v, want %v", typeOfValue, actual, expected)
		}
	}
	assertJSONFields(CloudPaymentEntryView{}, []string{"entry_type", "qr_code", "code_url", "qr_code_url", "expires_at"})
	assertJSONFields(CloudPaymentView{}, []string{
		"order_id", "plan_id", "order_type", "plan", "from_plan", "amount", "amount_unit", "currency", "payment_channel",
		"state", "payable", "entitlement_granted", "expires_at", "payment_entry", "version",
	})
	assertJSONFields(CloudPaymentOperationView{}, []string{"account", "payment", "balance", "subscription"})
	assertJSONFields(CloudCaptchaConfigView{}, []string{"provider", "region", "prefix", "scene_id"})
	assertJSONFields(CloudLoginRequest{}, []string{"phone", "password", "captcha_passed", "captcha_ticket", "captcha_provider"})
	assertJSONFields(CloudResetPasswordRequest{}, []string{"phone", "new_password", "sms_code"})
	assertJSONFields(CloudSendPasswordResetSMSRequest{}, []string{"phone", "captcha_ticket", "captcha_provider"})
	assertJSONFields(CloudSendRegisterSMSRequest{}, []string{"phone", "captcha_passed", "captcha_ticket", "captcha_provider"})
}

func TestCloudPaymentAPIsWithoutAuthenticationFailClosed(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	app := NewApp()
	var errs []error
	_, err := app.CloudCreateSubscriptionOrder("plus", "alipay")
	errs = append(errs, err)
	_, err = app.CloudCreateUpgradeOrder("pro", "wechat")
	errs = append(errs, err)
	_, err = app.CloudConfirmPayment("order")
	errs = append(errs, err)
	_, err = app.CloudRefreshPayment("order")
	errs = append(errs, err)
	for index, err := range errs {
		var cloudErr *cloud.Error
		if !errors.As(err, &cloudErr) || cloudErr.Code != cloud.ErrorProfileScopeMismatch {
			t.Fatalf("payment API %d without authentication error = %#v", index, err)
		}
	}
}

func TestCloudRuntimeClosesWithAppLifecycle(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.cloudRuntime = runtime
	app.shutdown(context.Background())
	if _, err := app.CloudBootstrap(); err == nil {
		t.Fatal("CloudBootstrap remained available after App shutdown")
	}
}

func desktopCloudTestProfilePathProvider(root string) cloud.ProfilePathProvider {
	return func(actor userctx.ActorContext) config.ProfilePathResolver {
		return config.NewProfilePathResolverWithOptions(config.ProfilePathResolverOptions{
			Actor:     actor,
			HomeRoot:  filepath.Join(root, "home"),
			StateRoot: filepath.Join(root, "state"),
			CacheRoot: filepath.Join(root, "cache"),
		})
	}
}
