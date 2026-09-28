package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestSubscriptionPlanConfigVersionAcceptsOpaqueAndLegacyValues(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "opaque cloud version", json: `"catalog-20260715-user-v1"`, want: "catalog-20260715-user-v1"},
		{name: "legacy numeric snapshot", json: `1`, want: "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`{"plan":"plus","display_name":"Plus","price_amount":"19.90","currency":"CNY","credit_amount":"20000.0000","period_days":30,"enabled":true,"sort_order":2,"config_version":` + tt.json + `}`)
			var plan SubscriptionPlan
			if err := json.Unmarshal(data, &plan); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if plan.ConfigVersion != tt.want {
				t.Fatalf("ConfigVersion = %q, want %q", plan.ConfigVersion, tt.want)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("SubscriptionPlan.Validate: %v", err)
			}
		})
	}
}

func TestBuildIdempotencyKeyIsStableAndBindingScoped(t *testing.T) {
	scope, err := NewBindingScope(userctx.Default(t.TempDir()), "usr_01", 1)
	if err != nil {
		t.Fatalf("NewBindingScope: %v", err)
	}
	first, err := BuildIdempotencyKey(IdempotencyActionCreateFreeze, scope, "task_01")
	if err != nil {
		t.Fatalf("BuildIdempotencyKey: %v", err)
	}
	second, err := BuildIdempotencyKey(IdempotencyActionCreateFreeze, scope, "task_01")
	if err != nil {
		t.Fatalf("BuildIdempotencyKey repeat: %v", err)
	}
	if first != second {
		t.Fatalf("idempotency key changed: %q != %q", first, second)
	}
	other, err := BuildIdempotencyKey(IdempotencyActionCreateFreeze, BindingScope{
		ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 2,
	}, "task_01")
	if err != nil {
		t.Fatalf("BuildIdempotencyKey other generation: %v", err)
	}
	if first == other {
		t.Fatal("idempotency key must change with binding generation")
	}
}

func TestBuildIdempotencyKeySupportsErrorReportSubmission(t *testing.T) {
	scope := BindingScope{
		ProfileID:         "profile-feedback",
		CloudUserID:       "user-feedback",
		BindingGeneration: 3,
	}
	key, err := BuildIdempotencyKey(IdempotencyActionSubmitErrorReport, scope, "attempt-01")
	if err != nil {
		t.Fatalf("BuildIdempotencyKey: %v", err)
	}
	if !strings.HasPrefix(key, "kwu_") || strings.Contains(key, scope.CloudUserID) {
		t.Fatalf("feedback idempotency key is not opaque: %q", key)
	}
}

func TestReleaseFreezeRequestRequiresStableTaskIdentity(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1}
	key, err := BuildIdempotencyKey(IdempotencyActionReleaseFreeze, scope, "task_01")
	if err != nil {
		t.Fatal(err)
	}
	request := ReleaseFreezeRequest{Scope: scope, TaskID: "task_01", IdempotencyKey: key}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	request.TaskID = " task_01"
	if err := request.Validate(); err == nil {
		t.Fatal("release request accepted an untrimmed task_id")
	}
}

func TestA0NormalizedContractValidationAndPaymentMapping(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1}
	binding := ProfileBinding{
		ActorID: "local", ProfileID: "default", CloudUserID: "usr_01",
		Status: BindingBound, BindingGeneration: 1,
	}
	balance := BalanceSnapshot{
		TotalBalance: "30", FreeBalance: "10", SubscriptionBalance: "20",
		AvailableBalance: "30", FreeAvailableBalance: "10", SubscriptionAvailableBalance: "20",
		FrozenBalance: "0", FreeFrozenBalance: "0", SubscriptionFrozenBalance: "0", Version: 1,
	}
	subscription := Subscription{Scope: scope, Plan: "free", Status: SubscriptionFree, UpdatedAt: time.Now()}
	state := AccountState{
		Binding: binding, State: AccountAuthenticated,
		User: CloudUser{CloudUserID: "usr_01"}, Balance: balance, Subscription: subscription,
		LastSyncAt: time.Now(),
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("AccountState.Validate: %v", err)
	}

	plan := SubscriptionPlan{
		Plan: "plus", DisplayName: "Plus", PriceAmount: "29.90", Currency: "CNY",
		CreditAmount: "20000", PeriodDays: 30, Enabled: true, SortOrder: 10, ConfigVersion: "1",
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("SubscriptionPlan.Validate: %v", err)
	}
	request := CreateSubscriptionOrderRequest{
		Scope: scope, Plan: plan.Plan, PaymentChannel: PaymentChannelWeChat, IdempotencyKey: "kwu_order",
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("CreateSubscriptionOrderRequest.Validate: %v", err)
	}

	order := PaymentOrder{
		Scope: scope, OrderID: "pay_01", OrderType: PaymentOrderSubscriptionPurchase,
		Plan: plan.Plan, Amount: "2990", AmountUnit: AmountUnitFen, Currency: "CNY",
		PaymentChannel: PaymentChannelWeChat, Status: PaymentPending, GrantStatus: GrantNotGranted,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := order.Validate(); err != nil {
		t.Fatalf("PaymentOrder.Validate: %v", err)
	}
	ui, err := MapPaymentStatus(order.Status)
	if err != nil || ui != PaymentUIStatePaying {
		t.Fatalf("MapPaymentStatus(payment_pending) = %q, %v", ui, err)
	}
	order.Status, order.GrantStatus = PaymentGranted, GrantGranted
	ui, err = MapPaymentStatus(order.Status)
	if err != nil || ui != PaymentUIStateSucceeded {
		t.Fatalf("MapPaymentStatus(granted) = %q, %v", ui, err)
	}
	order.Status, order.GrantStatus = PaymentClientPaidUnverified, GrantNotGranted
	if err := order.Validate(); err == nil {
		t.Fatal("client_paid_unverified must not validate as a persistable order status")
	}
	ui, err = MapPaymentStatus(PaymentClientPaidUnverified)
	if err != nil || ui != PaymentUIStateConfirming {
		t.Fatalf("MapPaymentStatus(client_paid_unverified) = %q, %v", ui, err)
	}

	sync := ThirdPartySync{Scope: scope, Status: ThirdPartySyncPending, UpdatedAt: time.Now(), Version: 1}
	if err := sync.Validate(); err != nil {
		t.Fatalf("ThirdPartySync.Validate: %v", err)
	}
	sync.Status = "unrecognized"
	if err := sync.Validate(); err == nil {
		t.Fatal("ThirdPartySync.Validate accepted an unknown status")
	}
}

func TestAccountStateValidatesCompleteUIStates(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1}
	account := AccountState{
		Binding: ProfileBinding{
			ActorID: "local", ProfileID: "default", CloudUserID: "usr_01",
			Status: BindingBound, BindingGeneration: 1,
		},
		User: CloudUser{CloudUserID: "usr_01"},
		Balance: BalanceSnapshot{
			TotalBalance: "30", FreeBalance: "10", SubscriptionBalance: "20",
			AvailableBalance: "30", FreeAvailableBalance: "10", SubscriptionAvailableBalance: "20",
			FrozenBalance: "0", FreeFrozenBalance: "0", SubscriptionFrozenBalance: "0", Version: 1,
		},
		Subscription: Subscription{Scope: scope, Plan: "free", Status: SubscriptionFree, UpdatedAt: time.Now()},
		LastSyncAt:   time.Now(),
	}

	tests := []struct {
		name    string
		state   AccountStatus
		wantErr bool
	}{
		{name: "anonymous", state: AccountAnonymous},
		{name: "authenticating", state: AccountAuthenticating},
		{name: "authenticated", state: AccountAuthenticated},
		{name: "refreshing", state: AccountRefreshing},
		{name: "expired", state: AccountExpired},
		{name: "offline", state: AccountOffline},
		{name: "unknown", state: AccountStatus("unknown"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account.State = tt.state
			err := account.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("AccountState.Validate(%q) error = %v, wantErr %v", tt.state, err, tt.wantErr)
			}
		})
	}
}

func TestSessionValidateRetainsPersistedStatesOnly(t *testing.T) {
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1}
	tests := []struct {
		name    string
		session Session
		wantErr bool
	}{
		{name: "authenticated", session: Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access"}},
		{name: "expired", session: Session{Scope: scope, State: SessionExpired}},
		{name: "offline", session: Session{Scope: scope, State: SessionOffline}},
		{name: "ui only state", session: Session{Scope: scope, State: SessionState("anonymous")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Session.Validate(%q) error = %v, wantErr %v", tt.session.State, err, tt.wantErr)
			}
		})
	}
}

func TestFreezeValidatesCompleteCloudStatuses(t *testing.T) {
	freeze := Freeze{
		Scope:                    BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1},
		FreezeID:                 "frz_01",
		TaskID:                   "task_01",
		FreezeAmount:             "30",
		FreeFrozenAmount:         "10",
		SubscriptionFrozenAmount: "20",
		ExpiresAt:                time.Now().Add(time.Hour),
	}

	tests := []struct {
		name    string
		status  FreezeStatus
		wantErr bool
	}{
		{name: "created", status: FreezeCreated},
		{name: "active", status: FreezeActive},
		{name: "settling", status: FreezeSettling},
		{name: "settled", status: FreezeSettled},
		{name: "released", status: FreezeReleased},
		{name: "expired released", status: FreezeExpiredReleased},
		{name: "expired pending review", status: FreezeExpiredPendingReview},
		{name: "unknown", status: FreezeStatus("unknown"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freeze.Status = tt.status
			err := freeze.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Freeze.Validate(%q) error = %v, wantErr %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

func TestA0AccountFixtureRequestIsProfileBoundAndVersioned(t *testing.T) {
	request := AccountFixtureRequest{
		Binding:   ProfileBinding{ActorID: "local", ProfileID: "default", Status: BindingBinding},
		Operation: AccountOperationLogin, FixtureVersion: "auth-v1", Payload: []byte(`{"fixture":true}`),
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("AccountFixtureRequest.Validate: %v", err)
	}
	request.Operation = "wire-login"
	if err := request.Validate(); err == nil {
		t.Fatal("AccountFixtureRequest.Validate accepted an unnormalized operation")
	}
}

func TestThirdPartySyncUsesCompleteNormalizedStatuses(t *testing.T) {
	base := ThirdPartySync{
		Scope:     BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1},
		UpdatedAt: time.Now(),
		Version:   1,
	}
	for _, status := range []ThirdPartySyncStatus{
		ThirdPartySyncPending,
		ThirdPartySyncRunning,
		ThirdPartySyncSucceeded,
		ThirdPartySyncFailedRetryable,
		ThirdPartySyncFailedManualRequired,
		ThirdPartySyncSkipped,
	} {
		base.Status = status
		if err := base.Validate(); err != nil {
			t.Errorf("ThirdPartySync.Validate(%q): %v", status, err)
		}
	}
	base.Status = "failed"
	if err := base.Validate(); err == nil {
		t.Fatal("ThirdPartySync.Validate accepted ambiguous failed status")
	}
}

func TestMapPaymentStatusOnlyGrantedSucceeds(t *testing.T) {
	cases := []struct {
		status PaymentStatus
		want   PaymentUIState
	}{
		{PaymentCreated, PaymentUIStateCreated},
		{PaymentPending, PaymentUIStatePaying},
		{PaymentCallbackReceived, PaymentUIStateConfirming},
		{PaymentVerifying, PaymentUIStateConfirming},
		{PaymentPaidVerified, PaymentUIStateConfirming},
		{PaymentGranting, PaymentUIStateConfirming},
		{PaymentClientPaidUnverified, PaymentUIStateConfirming},
		{PaymentGranted, PaymentUIStateSucceeded},
		{PaymentClosed, PaymentUIStateClosed},
		{PaymentTimeout, PaymentUIStateTimeout},
		{PaymentFailed, PaymentUIStateFailed},
		{PaymentVerifyFailedManualRequired, PaymentUIStateManualRequired},
	}
	for _, tc := range cases {
		got, err := MapPaymentStatus(tc.status)
		if err != nil || got != tc.want {
			t.Errorf("MapPaymentStatus(%q) = %q, %v; want %q", tc.status, got, err, tc.want)
		}
		if tc.status != PaymentGranted && got == PaymentUIStateSucceeded {
			t.Errorf("MapPaymentStatus(%q) must not succeed before grant", tc.status)
		}
	}
}

func TestPaymentStatusTransitionAllowsOnlyForwardOrSticky(t *testing.T) {
	allowed := [][2]PaymentStatus{
		{PaymentCreated, PaymentCreated},
		{PaymentCreated, PaymentPending},
		{PaymentCreated, PaymentGranted},
		{PaymentPending, PaymentCallbackReceived},
		{PaymentPending, PaymentVerifying},
		{PaymentCallbackReceived, PaymentVerifying},
		{PaymentVerifying, PaymentPaidVerified},
		{PaymentPaidVerified, PaymentGranting},
		{PaymentGranting, PaymentGranted},
		{PaymentPending, PaymentClosed},
		{PaymentPending, PaymentFailed},
		{PaymentPending, PaymentTimeout},
		{PaymentPending, PaymentVerifyFailedManualRequired},
		{PaymentGranted, PaymentGranted},
	}
	for _, transition := range allowed {
		if err := validatePaymentStatusTransition(transition[0], transition[1]); err != nil {
			t.Errorf("allowed transition %s -> %s: %v", transition[0], transition[1], err)
		}
	}

	rejected := [][2]PaymentStatus{
		{PaymentPending, PaymentCreated},
		{PaymentVerifying, PaymentCallbackReceived},
		{PaymentPaidVerified, PaymentPending},
		{PaymentGranted, PaymentPending},
		{PaymentClosed, PaymentPending},
		{PaymentFailed, PaymentClosed},
		{PaymentTimeout, PaymentFailed},
		{PaymentVerifyFailedManualRequired, PaymentGranted},
		{PaymentPending, PaymentClientPaidUnverified},
		{PaymentClientPaidUnverified, PaymentPending},
	}
	for _, transition := range rejected {
		if err := validatePaymentStatusTransition(transition[0], transition[1]); err == nil {
			t.Errorf("accepted transition %s -> %s", transition[0], transition[1])
		}
	}
}

func TestPaymentOrderRejectsEmptyOrSamePlanUpgrade(t *testing.T) {
	bundle := loadA0Fixture(t)
	var upgrade PaymentOrder
	for _, fixture := range bundle.PaymentCreates {
		if fixture.Result.Order.OrderType == PaymentOrderSubscriptionUpgrade {
			upgrade = fixture.Result.Order
			break
		}
	}
	if upgrade.OrderID == "" {
		t.Fatal("canonical fixture has no upgrade order")
	}
	for _, fromPlan := range []string{"", upgrade.Plan} {
		order := upgrade
		order.FromPlan = fromPlan
		if err := order.Validate(); err == nil {
			t.Fatalf("PaymentOrder.Validate accepted upgrade from_plan %q to %q", fromPlan, order.Plan)
		}
	}
}

var _ = func(CloudAccountClient, CloudPaymentClient) {}

func TestStaticBindingResolverRejectsAnotherProfile(t *testing.T) {
	resolver, err := NewStaticBindingResolver(ProfileBinding{
		ActorID: "local", ProfileID: "profile-a", CloudUserID: "usr_01",
		Status: BindingBound, BindingGeneration: 1,
	})
	if err != nil {
		t.Fatalf("NewStaticBindingResolver: %v", err)
	}
	_, err = resolver.ResolveBinding(context.Background(), userctx.ActorContext{ProfileID: "profile-b"})
	var cloudErr *Error
	if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorProfileScopeMismatch {
		t.Fatalf("ResolveBinding error = %v, want %s", err, ErrorProfileScopeMismatch)
	}
}

func TestMemorySessionStoreIsolatesBindingGeneration(t *testing.T) {
	store := NewMemorySessionStore()
	session := Session{
		Scope: BindingScope{ProfileID: "default", CloudUserID: "usr_01", BindingGeneration: 1},
		State: SessionAuthenticated, AccessToken: "access",
	}
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok, err := store.Load(context.Background(), session.Scope); err != nil || !ok {
		t.Fatalf("Load saved session = ok %v, err %v", ok, err)
	}
	newGeneration := session.Scope
	newGeneration.BindingGeneration = 2
	if _, ok, err := store.Load(context.Background(), newGeneration); err != nil || ok {
		t.Fatalf("Load other generation = ok %v, err %v", ok, err)
	}
	if err := store.Clear(context.Background(), session.Scope); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, ok, err := store.Load(context.Background(), session.Scope); err != nil || ok {
		t.Fatalf("Load cleared session = ok %v, err %v", ok, err)
	}
}

func TestRegistrationPasswordCryptoValidateRequiresDocumentedContract(t *testing.T) {
	now := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
	valid := validRegistrationPasswordCrypto(now)
	tests := map[string]func(*RegistrationPasswordCrypto){
		"algorithm":           func(value *RegistrationPasswordCrypto) { value.Algorithm = "RSA" },
		"key bits":            func(value *RegistrationPasswordCrypto) { value.KeySizeBits = 256 },
		"mode":                func(value *RegistrationPasswordCrypto) { value.Mode = "GCM" },
		"padding":             func(value *RegistrationPasswordCrypto) { value.Padding = "NoPadding" },
		"output":              func(value *RegistrationPasswordCrypto) { value.Output = "Hex" },
		"ciphertext encoding": func(value *RegistrationPasswordCrypto) { value.CiphertextEncoding = "base64(ciphertext)" },
		"key version":         func(value *RegistrationPasswordCrypto) { value.KeyVersion = "" },
		"key base64":          func(value *RegistrationPasswordCrypto) { value.KeyBase64 = "not-base64" },
		"key length": func(value *RegistrationPasswordCrypto) {
			value.KeyBase64 = base64.StdEncoding.EncodeToString([]byte("short"))
		},
		"expiry": func(value *RegistrationPasswordCrypto) { value.ExpiresAt = now },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			if err := value.Validate(now); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := valid.Validate(now); err != nil {
		t.Fatalf("valid registration crypto: %v", err)
	}
	metadata := valid.Metadata()
	if metadata.KeySize != 128 || metadata.KeyVersion != valid.KeyVersion {
		t.Fatalf("registration metadata = %#v", metadata)
	}
}

func TestRegistrationRequestRejectsNonASCIICodeAndOversizedCiphertext(t *testing.T) {
	request := validRegistrationRequest()
	request.SMSCode = "１２３４５６"
	if err := request.Validate(); err == nil {
		t.Fatal("expected ASCII SMS validation error")
	}
	request = validRegistrationRequest()
	request.EncryptedPassword = strings.Repeat("x", 4097)
	if err := request.Validate(); err == nil {
		t.Fatal("expected encrypted password size error")
	}
	request = validRegistrationRequest()
	request.PasswordCrypto.KeySize = 256
	if err := request.Validate(); err == nil {
		t.Fatal("expected registration metadata error")
	}
	if err := validRegistrationRequest().Validate(); err != nil {
		t.Fatalf("valid registration request: %v", err)
	}
}

func TestRegistrationSMSContractsValidateTicketAndResult(t *testing.T) {
	request := SendRegistrationSMSRequest{Phone: "+8613900000002", CaptchaTicket: strings.Repeat("x", 8192), CaptchaProvider: LoginCaptchaProviderAliyunCaptchaV2}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid SMS request: %v", err)
	}
	request.CaptchaTicket = strings.Repeat("x", 8193)
	if err := request.Validate(); err == nil {
		t.Fatal("expected captcha ticket size error")
	}
	request.CaptchaTicket = "captcha-ticket"
	request.CaptchaProvider = "unsupported"
	if err := request.Validate(); err == nil {
		t.Fatal("expected captcha provider error")
	}
	result := RegistrationSMSResult{Phone: "+8613900000002", ExpiresInSeconds: 300}
	if err := result.Validate(); err != nil {
		t.Fatalf("valid SMS result: %v", err)
	}
	result.ExpiresInSeconds = 0
	if err := result.Validate(); err == nil {
		t.Fatal("expected SMS expiry error")
	}
}

func TestCaptchaConfigRejectsUnsupportedAndDevelopmentValues(t *testing.T) {
	valid := CaptchaConfig{Provider: LoginCaptchaProviderAliyunCaptchaV2, Region: "cn", Prefix: "public-prefix", SceneID: "register-scene"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid CAPTCHA config: %v", err)
	}
	for name, mutate := range map[string]func(*CaptchaConfig){
		"provider":   func(value *CaptchaConfig) { value.Provider = "unsupported" },
		"region":     func(value *CaptchaConfig) { value.Region = "us" },
		"prefix":     func(value *CaptchaConfig) { value.Prefix = " reasonix " },
		"dev prefix": func(value *CaptchaConfig) { value.Prefix = "reasonix-dev" },
		"dev scene":  func(value *CaptchaConfig) { value.SceneID = "reasonix-register-dev" },
	} {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			if err := value.Validate(); err == nil {
				t.Fatal("expected CAPTCHA config validation error")
			}
		})
	}
}

func TestRegistrationResultValidatesAsyncSyncWithoutLocalScope(t *testing.T) {
	now := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
	result := validRegistrationResult(now)
	if err := result.Validate(); err != nil {
		t.Fatalf("valid registration result: %v", err)
	}
	result.ThirdPartySync.Status = "unknown"
	if err := result.Validate(); err == nil {
		t.Fatal("expected third-party sync status error")
	}
	result = validRegistrationResult(now)
	result.SignupReward.LedgerID = ""
	if err := result.Validate(); err == nil {
		t.Fatal("expected granted reward ledger error")
	}
}

func validRegistrationPasswordCrypto(now time.Time) RegistrationPasswordCrypto {
	return RegistrationPasswordCrypto{
		Algorithm: "AES", KeySizeBits: 128, Mode: "CBC", Padding: "PKCS7Padding", Output: "Base64",
		CiphertextEncoding: "base64(iv || ciphertext)", KeyVersion: "auth-key-v1",
		KeyBase64: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")), ExpiresAt: now.Add(5 * time.Minute),
	}
}

func validRegistrationRequest() RegistrationRequest {
	return RegistrationRequest{
		Phone: "+8613900000002", SMSCode: "123456", EncryptedPassword: "encoded-password",
		PasswordCrypto: RegistrationPasswordMetadata{
			Algorithm: "AES", KeySize: 128, Mode: "CBC", Padding: "PKCS7Padding", Output: "Base64", KeyVersion: "auth-key-v1",
		},
	}
}

func validRegistrationResult(now time.Time) RegistrationResult {
	return RegistrationResult{
		Authentication: AuthenticationResult{
			User: CloudUser{CloudUserID: "usr_registration", DisplayName: "Reasonix User"},
			Session: RemoteSession{
				AccessToken: "access-registration", AccessTokenExpiresAt: now.Add(time.Hour),
				RefreshToken: "refresh-registration", RefreshTokenExpiresAt: now.Add(30 * 24 * time.Hour), DeviceID: "device-registration",
			},
			ServerTime: now,
		},
		SignupReward: RegistrationSignupReward{Granted: true, Amount: "1000.0000", LedgerID: "led_registration"},
		ThirdPartySync: RegistrationThirdPartySync{
			SyncID: "tps_registration", Provider: "new-api", Status: ThirdPartySyncPending, Version: 1, UpdatedAt: now,
		},
		Balance: BalanceSnapshot{
			TotalBalance: "1000.0000", FreeBalance: "1000.0000", SubscriptionBalance: "0.0000",
			AvailableBalance: "1000.0000", FreeAvailableBalance: "1000.0000", SubscriptionAvailableBalance: "0.0000",
			FrozenBalance: "0.0000", FreeFrozenBalance: "0.0000", SubscriptionFrozenBalance: "0.0000", Version: 2,
		},
	}
}
