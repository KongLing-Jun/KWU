package cloud

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func loadS0Fixture(t *testing.T) FixtureBundle {
	return loadFixture(t, "s0-v1.json")
}

func loadA0Fixture(t *testing.T) FixtureBundle {
	return loadFixture(t, "a0-v1.json")
}

func loadFixture(t *testing.T, name string) FixtureBundle {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	bundle, err := DecodeFixture(f)
	if err != nil {
		t.Fatalf("DecodeFixture: %v", err)
	}
	return bundle
}

func TestLoadCanonicalA0FixtureStrictlyLoadsEmbeddedBundle(t *testing.T) {
	bundle, err := LoadCanonicalA0Fixture()
	if err != nil {
		t.Fatalf("LoadCanonicalA0Fixture: %v", err)
	}
	if bundle.Version != A0FixtureVersion || !bundle.Metadata.MockOnly || bundle.Metadata.L2Integrated {
		t.Fatalf("canonical fixture metadata = version %q, metadata %#v", bundle.Version, bundle.Metadata)
	}
}

func TestCanonicalCloudFixturesHaveNoThirdScenarioFile(t *testing.T) {
	files, err := filepath.Glob("testdata/*-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || filepath.Base(files[0]) != "a0-v1.json" || filepath.Base(files[1]) != "s0-v1.json" {
		t.Fatalf("canonical fixture files = %v, want only a0-v1.json and s0-v1.json", files)
	}
}

func TestFixtureEnvironmentSupportsBothDevelopmentTracks(t *testing.T) {
	bundle := loadS0Fixture(t)
	env, err := NewFixtureEnvironment(bundle)
	if err != nil {
		t.Fatalf("NewFixtureEnvironment: %v", err)
	}
	binding, err := env.BindingResolver.ResolveBinding(context.Background(), userctx.Default(t.TempDir()))
	if err != nil {
		t.Fatalf("ResolveBinding: %v", err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatalf("binding.Scope: %v", err)
	}
	if _, ok, err := env.SessionStore.Load(context.Background(), scope); err != nil || !ok {
		t.Fatalf("Load session = ok %v, err %v", ok, err)
	}
	result, err := env.CreditClient.CreateFreeze(context.Background(), bundle.FreezeCases[0].Request)
	if err != nil {
		t.Fatalf("CreateFreeze: %v", err)
	}
	repeated, err := env.CreditClient.CreateFreeze(context.Background(), bundle.FreezeCases[0].Request)
	if err != nil {
		t.Fatalf("CreateFreeze repeat: %v", err)
	}
	if result.Freeze.FreezeID != repeated.Freeze.FreezeID {
		t.Fatalf("repeat freeze IDs differ: %q != %q", result.Freeze.FreezeID, repeated.Freeze.FreezeID)
	}
	events, err := env.EventSource.Connect(context.Background(), scope)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	var got []Event
	for event := range events {
		got = append(got, event)
	}
	if len(got) != len(bundle.Events) {
		t.Fatalf("events = %d, want %d", len(got), len(bundle.Events))
	}
	if got[1].CloudUserID == scope.CloudUserID {
		t.Fatal("fixture must retain the other-user event for isolation tests")
	}
}

func TestFixtureCreditClientReturnsConfiguredErrors(t *testing.T) {
	bundle := loadS0Fixture(t)
	client, err := NewFixtureCreditClient(bundle.FreezeCases)
	if err != nil {
		t.Fatalf("NewFixtureCreditClient: %v", err)
	}
	_, err = client.CreateFreeze(context.Background(), bundle.FreezeCases[1].Request)
	var cloudErr *Error
	if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorInsufficientBalance {
		t.Fatalf("CreateFreeze error = %v, want %s", err, ErrorInsufficientBalance)
	}
	mismatched := bundle.FreezeCases[0].Request
	mismatched.ModelID = "another_model"
	if _, err := client.CreateFreeze(context.Background(), mismatched); err == nil {
		t.Fatal("CreateFreeze accepted a changed request for an existing task_id")
	}
}

func TestFixtureCreditClientClonesInput(t *testing.T) {
	bundle := loadS0Fixture(t)
	client, err := NewFixtureCreditClient(bundle.FreezeCases)
	if err != nil {
		t.Fatalf("NewFixtureCreditClient: %v", err)
	}
	want := bundle.FreezeCases[0].Result.Freeze.FreezeID
	bundle.FreezeCases[0].Result.Freeze.FreezeID = "mutated"
	result, err := client.CreateFreeze(context.Background(), bundle.FreezeCases[0].Request)
	if err != nil {
		t.Fatalf("CreateFreeze: %v", err)
	}
	if result.Freeze.FreezeID != want {
		t.Fatalf("freeze ID = %q after caller mutation, want %q", result.Freeze.FreezeID, want)
	}
}

func TestFixtureValidationRejectsContractDrift(t *testing.T) {
	bundle := loadS0Fixture(t)
	bundle.FreezeCases[0].Request.IdempotencyKey = "different"
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted an idempotency key outside the shared rule")
	}
	bundle = loadS0Fixture(t)
	bundle.FreezeCases[0].Result.Balance.TotalBalance = ""
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted a missing balance field")
	}
}

func TestDecodeFixtureRejectsUnknownFieldsAndVersions(t *testing.T) {
	unknown := `{"fixture_version":"s0-v1","unknown":true}`
	if _, err := DecodeFixture(strings.NewReader(unknown)); err == nil {
		t.Fatal("DecodeFixture accepted an unknown field")
	}
	bundle := loadS0Fixture(t)
	bundle.Version = "s0-v2"
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted an unsupported fixture version")
	}
}

func TestA0FixtureCoversVersionedMockContractScenarios(t *testing.T) {
	bundle := loadA0Fixture(t)
	if !bundle.Metadata.MockOnly || bundle.Metadata.L2Integrated {
		t.Fatalf("fixture metadata = %#v, want mock-only and not L2 integrated", bundle.Metadata)
	}
	if len(bundle.Metadata.Gates) != 8 {
		t.Fatalf("fixture gates = %d, want CLOUD-01/02/03/05/07/08/09/10", len(bundle.Metadata.Gates))
	}
	if len(bundle.AuthCases) != 7 || len(bundle.BalanceCases) == 0 || len(bundle.PlanCases) == 0 || len(bundle.PaymentCases) != 11 || len(bundle.SyncCases) != 6 {
		t.Fatalf("fixture coverage auth=%d balance=%d plans=%d payment=%d sync=%d", len(bundle.AuthCases), len(bundle.BalanceCases), len(bundle.PlanCases), len(bundle.PaymentCases), len(bundle.SyncCases))
	}
	if len(bundle.FreezeCases) != 7 || len(bundle.RecoveryCases) == 0 {
		t.Fatalf("fixture coverage freezes=%d recovery=%d", len(bundle.FreezeCases), len(bundle.RecoveryCases))
	}
	if bundle.AuthCases[4].Binding.Status != BindingMismatch {
		t.Fatalf("profile mismatch fixture status = %q, want %q", bundle.AuthCases[4].Binding.Status, BindingMismatch)
	}
	if bundle.AuthCases[4].Binding.CloudUserID == bundle.Binding.CloudUserID || bundle.AuthCases[4].Binding.BindingGeneration == bundle.Binding.BindingGeneration {
		t.Fatal("profile mismatch fixture must declare a second cloud account and binding generation")
	}
	if bundle.RecoveryCases[0].Scope.BindingGeneration != bundle.Binding.BindingGeneration {
		t.Fatalf("recovery generation = %d, want %d", bundle.RecoveryCases[0].Scope.BindingGeneration, bundle.Binding.BindingGeneration)
	}
}

func TestA0FixtureStrictlyLoadsRequiredAuthScenarios(t *testing.T) {
	bundle := loadA0Fixture(t)
	if len(bundle.AuthCases) != 7 {
		t.Fatalf("auth fixture count = %d, want 7", len(bundle.AuthCases))
	}
	for _, name := range []string{
		"auth_success", "auth_success_secondary", "access_expired", "refresh_success", "refresh_failed", "profile_mismatch", "sms_code_invalid",
	} {
		if _, ok := authFixtureByName(bundle.AuthCases, name); !ok {
			t.Fatalf("missing required auth fixture %q", name)
		}
	}
}

func TestA0FixtureSecondSuccessfulAccountUsesIndependentScope(t *testing.T) {
	bundle := loadA0Fixture(t)
	secondary, ok := authFixtureByName(bundle.AuthCases, "auth_success_secondary")
	if !ok {
		t.Fatal("missing second successful auth fixture")
	}
	secondaryScope, err := secondary.Binding.Scope()
	if err != nil {
		t.Fatalf("secondary binding.Scope: %v", err)
	}
	defaultScope, err := bundle.Binding.Scope()
	if err != nil {
		t.Fatalf("default binding.Scope: %v", err)
	}
	if secondary.Binding.Status != BindingBound || secondary.State != SessionAuthenticated || secondary.Session == nil || secondary.Error != nil {
		t.Fatalf("second successful auth fixture = %#v, want bound authenticated account/session outcome", secondary)
	}
	if secondaryScope.ProfileID != defaultScope.ProfileID ||
		secondaryScope.CloudUserID == defaultScope.CloudUserID ||
		secondaryScope.BindingGeneration != defaultScope.BindingGeneration+1 ||
		secondary.Session.Scope != secondaryScope {
		t.Fatalf("second successful auth fixture scopes = binding %#v session %#v default %#v", secondaryScope, secondary.Session.Scope, defaultScope)
	}
}

func TestA0FixtureSuccessfulLoginsUsePhoneAndIrreversiblePasswordDigest(t *testing.T) {
	bundle := loadA0Fixture(t)
	seenPhones := make(map[string]bool)
	for _, name := range []string{"auth_success", "auth_success_secondary"} {
		fixture, ok := authFixtureByName(bundle.AuthCases, name)
		if !ok || fixture.LoginCredential == nil {
			t.Fatalf("%s has no fake login credential", name)
		}
		credential := fixture.LoginCredential
		if len(credential.Phone) != 11 || !strings.HasPrefix(credential.Phone, "1") || seenPhones[credential.Phone] {
			t.Fatalf("%s phone = %q, want a distinct user-form phone", name, credential.Phone)
		}
		seenPhones[credential.Phone] = true
		digest, err := hex.DecodeString(credential.PasswordDigest)
		if err != nil || len(digest) != 32 {
			t.Fatalf("%s password digest is not lowercase SHA-256: %q (%v)", name, credential.PasswordDigest, err)
		}
		if credential.PasswordDigest != strings.ToLower(credential.PasswordDigest) {
			t.Fatalf("%s password digest is not lowercase", name)
		}
	}
}

func TestA0FixtureRejectsInvalidLoginCredentialDigest(t *testing.T) {
	bundle := loadA0Fixture(t)
	fixture, ok := authFixtureByName(bundle.AuthCases, "auth_success")
	if !ok || fixture.LoginCredential == nil {
		t.Fatal("missing auth_success login credential")
	}
	for i := range bundle.AuthCases {
		if bundle.AuthCases[i].Name == fixture.Name {
			bundle.AuthCases[i].LoginCredential.PasswordDigest = "plaintext-password"
			break
		}
	}
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted a non-digest login credential")
	}
}

func TestA0FixtureRejectsDuplicateLoginPhone(t *testing.T) {
	bundle := loadA0Fixture(t)
	primary, ok := authFixtureByName(bundle.AuthCases, "auth_success")
	if !ok || primary.LoginCredential == nil {
		t.Fatal("missing auth_success login credential")
	}
	for i := range bundle.AuthCases {
		if bundle.AuthCases[i].Name == "auth_success_secondary" {
			bundle.AuthCases[i].LoginCredential.Phone = primary.LoginCredential.Phone
			break
		}
	}
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted duplicate login phones")
	}
}

func TestCanonicalA0PaymentCreateMatrixIsExact(t *testing.T) {
	bundle, err := LoadCanonicalA0Fixture()
	if err != nil {
		t.Fatalf("LoadCanonicalA0Fixture: %v", err)
	}

	wantPlans := map[string]SubscriptionPlan{
		"lite": {Plan: "lite", DisplayName: "Lite", PriceAmount: "9.90", Currency: "CNY", CreditAmount: "20000.0000", PeriodDays: 30, Enabled: true, SortOrder: 10, ConfigVersion: "1"},
		"plus": {Plan: "plus", DisplayName: "Plus", PriceAmount: "29.90", Currency: "CNY", CreditAmount: "60000.0000", PeriodDays: 30, Enabled: true, SortOrder: 20, ConfigVersion: "1"},
		"pro":  {Plan: "pro", DisplayName: "Pro", PriceAmount: "59.90", Currency: "CNY", CreditAmount: "120000.0000", PeriodDays: 30, Enabled: true, SortOrder: 30, ConfigVersion: "1"},
	}
	if len(bundle.PlanCases) != len(wantPlans) {
		t.Fatalf("canonical plan count = %d, want %d (Lite/Plus/Pro)", len(bundle.PlanCases), len(wantPlans))
	}
	for _, plan := range bundle.PlanCases {
		want, ok := wantPlans[plan.Plan]
		if !ok || plan != want {
			t.Fatalf("canonical plan %q = %#v, want %#v", plan.Plan, plan, want)
		}
	}

	type matrixCase struct {
		orderType PaymentOrderType
		fromPlan  string
		plan      string
		channel   PaymentChannel
		unit      AmountUnit
		entryType PaymentEntryType
	}
	wantCreates := map[string]matrixCase{}
	for _, plan := range []string{"lite", "plus", "pro"} {
		wantCreates["purchase-"+plan+"-alipay"] = matrixCase{PaymentOrderSubscriptionPurchase, "", plan, PaymentChannelAlipay, AmountUnitYuan, PaymentEntryQRCode}
		wantCreates["purchase-"+plan+"-wechat"] = matrixCase{PaymentOrderSubscriptionPurchase, "", plan, PaymentChannelWeChat, AmountUnitFen, PaymentEntryNativeQR}
	}
	for _, edge := range []struct{ from, to string }{{"lite", "plus"}, {"lite", "pro"}, {"plus", "pro"}} {
		wantCreates["upgrade-"+edge.from+"-"+edge.to+"-alipay"] = matrixCase{PaymentOrderSubscriptionUpgrade, edge.from, edge.to, PaymentChannelAlipay, AmountUnitYuan, PaymentEntryQRCode}
		wantCreates["upgrade-"+edge.from+"-"+edge.to+"-wechat"] = matrixCase{PaymentOrderSubscriptionUpgrade, edge.from, edge.to, PaymentChannelWeChat, AmountUnitFen, PaymentEntryNativeQR}
	}
	if len(bundle.PaymentCreates) != len(wantCreates) {
		t.Fatalf("canonical payment create count = %d, want %d", len(bundle.PaymentCreates), len(wantCreates))
	}
	seen := make(map[string]bool, len(bundle.PaymentCreates))
	for _, create := range bundle.PaymentCreates {
		want, ok := wantCreates[create.Key]
		if !ok || seen[create.Key] {
			t.Fatalf("unexpected or duplicate payment create key %q", create.Key)
		}
		seen[create.Key] = true
		order, entry := create.Result.Order, create.Result.Entry
		if order.OrderType != want.orderType || order.FromPlan != want.fromPlan || order.Plan != want.plan ||
			order.PaymentChannel != want.channel || order.AmountUnit != want.unit || entry.EntryType != want.entryType {
			t.Fatalf("payment create %q dimensions = %#v / %#v, want %#v", create.Key, order, entry, want)
		}
		if order.Status != PaymentPending || order.GrantStatus != GrantNotGranted {
			t.Fatalf("payment create %q status = %q/%q, want payment_pending/not_granted", create.Key, order.Status, order.GrantStatus)
		}
		matches := 0
		for _, tape := range bundle.PaymentTapes {
			if len(tape.Results) > 0 && samePaymentFixtureIdentity(order, tape.Results[0].Order) {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("payment create %q matches %d tapes, want exactly 1", create.Key, matches)
		}
	}
}

func TestA0FixturePaymentCreatesArePendingAndMatchExactlyOneTape(t *testing.T) {
	bundle := loadA0Fixture(t)
	if len(bundle.PaymentCreates) == 0 || len(bundle.PaymentTapes) == 0 {
		t.Fatal("canonical A0 fixture requires payment creates and tapes")
	}
	for _, create := range bundle.PaymentCreates {
		if err := create.Result.Validate(); err != nil {
			t.Fatalf("payment create %q result: %v", create.Key, err)
		}
		if create.Result.Order.Status != PaymentPending {
			t.Fatalf("payment create %q status = %q, want %q", create.Key, create.Result.Order.Status, PaymentPending)
		}
		matches := 0
		for _, tape := range bundle.PaymentTapes {
			if len(tape.Results) > 0 && samePaymentFixtureIdentity(create.Result.Order, tape.Results[0].Order) {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("payment create %q matches %d tapes, want exactly 1", create.Key, matches)
		}
	}
}

func TestA0FixtureRejectsInvalidPaymentCreateTapeContract(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FixtureBundle)
	}{
		{"create not pending", func(bundle *FixtureBundle) {
			bundle.PaymentCreates[0].Result.Order.Status = PaymentCreated
		}},
		{"create entry invalid", func(bundle *FixtureBundle) {
			bundle.PaymentCreates[0].Result.Entry.ExpiresAt = time.Time{}
		}},
		{"tape identity drift", func(bundle *FixtureBundle) {
			bundle.PaymentTapes[0].Results[0].Order.Amount = "999.99"
		}},
		{"create matches two tapes", func(bundle *FixtureBundle) {
			bundle.PaymentTapes = append(bundle.PaymentTapes, bundle.PaymentTapes[0])
		}},
		{"granted tape misses account snapshots", func(bundle *FixtureBundle) {
			bundle.PaymentTapes[0].GrantedAccountSnapshots = nil
		}},
		{"granted tape duplicates account user", func(bundle *FixtureBundle) {
			bundle.PaymentTapes[0].GrantedAccountSnapshots = append(bundle.PaymentTapes[0].GrantedAccountSnapshots, bundle.PaymentTapes[0].GrantedAccountSnapshots[0])
		}},
		{"granted snapshot changes target plan", func(bundle *FixtureBundle) {
			bundle.PaymentTapes[0].GrantedAccountSnapshots[0].Subscription.Plan = "pro"
		}},
		{"failed tape includes granted snapshot", func(bundle *FixtureBundle) {
			bundle.PaymentTapes[1].GrantedAccountSnapshots = append(bundle.PaymentTapes[1].GrantedAccountSnapshots, bundle.PaymentTapes[0].GrantedAccountSnapshots[0])
		}},
		{"tape skips verifying", func(bundle *FixtureBundle) {
			bundle.PaymentTapes[0].Results[1].Order.Status = PaymentCallbackReceived
		}},
		{"unexpected matrix ends failed", func(bundle *FixtureBundle) {
			last := len(bundle.PaymentTapes[0].Results) - 1
			bundle.PaymentTapes[0].Results[last].Order.Status = PaymentFailed
			bundle.PaymentTapes[0].Results[last].Order.GrantStatus = GrantNotGranted
			bundle.PaymentTapes[0].GrantedAccountSnapshots = nil
		}},
		{"payment create key drifts from dimensions", func(bundle *FixtureBundle) {
			bundle.PaymentCreates[0].Key = "purchase-pro-wechat-wrong"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := loadA0Fixture(t)
			tt.mutate(&bundle)
			if err := bundle.Validate(); err == nil {
				t.Fatal("Validate accepted an invalid payment create/tape contract")
			}
		})
	}
}

func samePaymentFixtureIdentity(left, right PaymentOrder) bool {
	return left.OrderID == right.OrderID &&
		left.Scope == right.Scope &&
		left.OrderType == right.OrderType &&
		left.Plan == right.Plan &&
		left.FromPlan == right.FromPlan &&
		left.PaymentChannel == right.PaymentChannel &&
		left.Amount == right.Amount &&
		left.AmountUnit == right.AmountUnit &&
		left.Currency == right.Currency &&
		left.ExpiresAt.Equal(right.ExpiresAt)
}

func TestA0FixtureProvidesMockOnlySnapshotsForEverySuccessfulAccount(t *testing.T) {
	bundle := loadA0Fixture(t)
	primary, err := bundle.Binding.Scope()
	if err != nil {
		t.Fatalf("primary binding.Scope: %v", err)
	}
	secondary, ok := authFixtureByName(bundle.AuthCases, "auth_success_secondary")
	if !ok {
		t.Fatal("missing secondary successful auth fixture")
	}
	secondaryScope, err := secondary.Binding.Scope()
	if err != nil {
		t.Fatalf("secondary binding.Scope: %v", err)
	}
	if secondaryScope != (BindingScope{ProfileID: "default", CloudUserID: "usr_02", BindingGeneration: 2}) {
		t.Fatalf("secondary scope = %#v, want default/usr_02/generation=2", secondaryScope)
	}
	if len(bundle.AccountSnapshots) != 2 {
		t.Fatalf("account snapshots = %d, want one per successful account scope", len(bundle.AccountSnapshots))
	}
	for _, scope := range []BindingScope{primary, secondaryScope} {
		fixture, ok := accountSnapshotByScope(bundle.AccountSnapshots, scope)
		if !ok {
			t.Fatalf("missing account snapshot for scope %#v", scope)
		}
		if err := fixture.Balance.Validate(); err != nil {
			t.Fatalf("balance for scope %#v: %v", scope, err)
		}
		if err := fixture.Subscription.Validate(); err != nil || fixture.Subscription.Scope != scope {
			t.Fatalf("subscription for scope %#v = %#v, err %v", scope, fixture.Subscription, err)
		}
	}
}

func TestA0FixtureValidationRejectsInvalidAccountSnapshots(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FixtureBundle)
	}{
		{"missing", func(b *FixtureBundle) { b.AccountSnapshots = b.AccountSnapshots[1:] }},
		{"duplicate", func(b *FixtureBundle) { b.AccountSnapshots[1] = b.AccountSnapshots[0] }},
		{"wrong generation", func(b *FixtureBundle) {
			b.AccountSnapshots[1].Scope.BindingGeneration++
			b.AccountSnapshots[1].Subscription.Scope.BindingGeneration++
		}},
		{"cross scope", func(b *FixtureBundle) { b.AccountSnapshots[0].Subscription.Scope = b.AccountSnapshots[1].Scope }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := loadA0Fixture(t)
			tt.mutate(&bundle)
			if err := bundle.Validate(); err == nil {
				t.Fatal("Validate accepted invalid account snapshots")
			}
		})
	}
}

func TestA0FixtureAccountSnapshotsDoNotExpandPrimaryScenarioCollections(t *testing.T) {
	bundle := loadA0Fixture(t)
	primary, err := bundle.Binding.Scope()
	if err != nil {
		t.Fatalf("primary binding.Scope: %v", err)
	}
	for i, fixture := range bundle.BalanceCases {
		if fixture.Scope != primary {
			t.Fatalf("balance fixture %d scope = %#v, want primary %#v", i, fixture.Scope, primary)
		}
	}
	for i, fixture := range bundle.FreezeCases {
		if fixture.Request.Scope != primary {
			t.Fatalf("freeze fixture %d scope = %#v, want primary %#v", i, fixture.Request.Scope, primary)
		}
	}
	for i, fixture := range bundle.PaymentCases {
		if fixture.Result.Order.Scope != primary {
			t.Fatalf("payment fixture %d scope = %#v, want primary %#v", i, fixture.Result.Order.Scope, primary)
		}
	}
	for i, fixture := range bundle.RecoveryCases {
		if fixture.Scope != primary || fixture.Subscription.Scope != primary {
			t.Fatalf("recovery fixture %d scope = %#v subscription = %#v, want primary %#v", i, fixture.Scope, fixture.Subscription.Scope, primary)
		}
	}
}

func TestA0FixtureIncludesSMSCodeInvalidOutcome(t *testing.T) {
	bundle := loadA0Fixture(t)
	fixture, ok := authFixtureByName(bundle.AuthCases, "sms_code_invalid")
	if !ok {
		t.Fatal("missing SMS_CODE_INVALID auth fixture")
	}
	if fixture.Error == nil || fixture.Error.Code != ErrorSMSCodeInvalid {
		t.Fatalf("SMS_CODE_INVALID outcome = %#v, want %s", fixture.Error, ErrorSMSCodeInvalid)
	}
}

func authFixtureByName(fixtures []AuthFixture, name string) (AuthFixture, bool) {
	for _, fixture := range fixtures {
		if fixture.Name == name {
			return fixture, true
		}
	}
	return AuthFixture{}, false
}

func accountSnapshotByScope(fixtures []AccountSnapshotFixture, scope BindingScope) (AccountSnapshotFixture, bool) {
	for _, fixture := range fixtures {
		if fixture.Scope == scope {
			return fixture, true
		}
	}
	return AccountSnapshotFixture{}, false
}

func withoutAuthFixture(fixtures []AuthFixture, name string) []AuthFixture {
	for i, fixture := range fixtures {
		if fixture.Name == name {
			return append(fixtures[:i:i], fixtures[i+1:]...)
		}
	}
	return fixtures
}

func TestFixtureValidationRejectsNonMockOrIncompleteContractFixture(t *testing.T) {
	bundle := loadA0Fixture(t)
	bundle.Metadata.MockOnly = false
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted a fixture that claims real transport")
	}
	bundle = loadA0Fixture(t)
	bundle.AuthCases[4].Binding.Status = BindingBound
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted a missing profile-mismatch fixture")
	}
	bundle = loadA0Fixture(t)
	bundle.AuthCases = withoutAuthFixture(bundle.AuthCases, "auth_success_secondary")
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted a fixture without the second successful auth account")
	}
	for _, tt := range []struct {
		name   string
		mutate func(*AuthFixture, BindingScope)
	}{
		{"profile", func(fixture *AuthFixture, scope BindingScope) {
			fixture.Binding.ProfileID = "other_profile"
			fixture.Session.Scope.ProfileID = "other_profile"
		}},
		{"cloud user", func(fixture *AuthFixture, scope BindingScope) {
			fixture.Binding.CloudUserID = scope.CloudUserID
			fixture.Session.Scope.CloudUserID = scope.CloudUserID
		}},
		{"binding generation", func(fixture *AuthFixture, scope BindingScope) {
			fixture.Binding.BindingGeneration = scope.BindingGeneration
			fixture.Session.Scope.BindingGeneration = scope.BindingGeneration
		}},
	} {
		t.Run("secondary scope violates "+tt.name, func(t *testing.T) {
			bundle := loadA0Fixture(t)
			scope, err := bundle.Binding.Scope()
			if err != nil {
				t.Fatalf("binding.Scope: %v", err)
			}
			for i := range bundle.AuthCases {
				if bundle.AuthCases[i].Name == "auth_success_secondary" {
					tt.mutate(&bundle.AuthCases[i], scope)
					break
				}
			}
			if err := bundle.Validate(); err == nil {
				t.Fatal("Validate accepted a second successful auth account sharing a default scope field")
			}
		})
	}
	bundle = loadA0Fixture(t)
	for i := range bundle.AuthCases {
		if bundle.AuthCases[i].Name == "sms_code_invalid" {
			bundle.AuthCases[i].Error.Code = ErrorAuthRequired
			break
		}
	}
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted an SMS outcome other than SMS_CODE_INVALID")
	}
	bundle = loadA0Fixture(t)
	bundle.PaymentCases = bundle.PaymentCases[:10]
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted an incomplete payment-state fixture")
	}
	bundle = loadA0Fixture(t)
	bundle.RecoveryCases[0].Scope.BindingGeneration++
	if err := bundle.Validate(); err == nil {
		t.Fatal("Validate accepted recovery from another binding generation")
	}
}

func TestDecodeFixtureRejectsNestedSchemaDrift(t *testing.T) {
	fixture := `{"fixture_version":"s0-v1","metadata":{"unknown":true}}`
	if _, err := DecodeFixture(strings.NewReader(fixture)); err == nil || !strings.Contains(err.Error(), `unknown field "metadata"`) {
		t.Fatalf("DecodeFixture error = %v, want an unknown metadata field error", err)
	}
}

func TestDecodeFixtureSupportsS0AndA0Versions(t *testing.T) {
	if bundle := loadS0Fixture(t); bundle.Version != S0FixtureVersion {
		t.Fatalf("s0 fixture version = %q, want %q", bundle.Version, S0FixtureVersion)
	}
	if bundle := loadA0Fixture(t); bundle.Version != A0FixtureVersion {
		t.Fatalf("a0 fixture version = %q, want %q", bundle.Version, A0FixtureVersion)
	}
}

func TestFixtureValidationRejectsDuplicateScenarioKeys(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FixtureBundle)
	}{
		{"auth name", func(b *FixtureBundle) { b.AuthCases[1].Name = b.AuthCases[0].Name }},
		{"balance name", func(b *FixtureBundle) { b.BalanceCases = append(b.BalanceCases, b.BalanceCases[0]) }},
		{"payment name", func(b *FixtureBundle) {
			b.PaymentCases[1].Name = b.PaymentCases[0].Name
			b.PaymentCases[1].Result.Order.Status = b.PaymentCases[0].Result.Order.Status
		}},
		{"sync status", func(b *FixtureBundle) { b.SyncCases[1].Status = b.SyncCases[0].Status }},
		{"recovery name", func(b *FixtureBundle) { b.RecoveryCases = append(b.RecoveryCases, b.RecoveryCases[0]) }},
		{"recovery last event", func(b *FixtureBundle) {
			duplicate := b.RecoveryCases[0]
			duplicate.Name = "second_recovery"
			b.RecoveryCases = append(b.RecoveryCases, duplicate)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := loadA0Fixture(t)
			tt.mutate(&bundle)
			if err := bundle.Validate(); err == nil {
				t.Fatal("Validate accepted duplicate scenario key")
			}
		})
	}
}
