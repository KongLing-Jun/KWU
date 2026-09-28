package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/userctx"
)

func TestFixtureAccountClientUsesOnlyA0AuthScenariosAndCountsSMS(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Login: "not-a-fixture"})
	if err == nil {
		t.Fatal("NewFixtureAccountClient accepted unknown scenario")
	}
	client, err = NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	request := fixtureRequest(t, actor, AccountOperationSendRegisterSMS, map[string]string{"phone": "13800000000", "captcha_ticket": "captcha_ticket_01"})
	if err := client.SendRegisterSMS(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if got := client.SendRegisterSMSCalls(); got != 1 {
		t.Fatalf("SMS calls = %d", got)
	}
	state, session, err := client.Login(context.Background(), fixtureRequest(t, actor, AccountOperationLogin, map[string]string{"phone": "13800000000", "password": "test-password-01"}))
	if err != nil || state.State != AccountAuthenticated || session.RefreshToken == "" {
		t.Fatalf("Login = %#v %#v %v", state, session, err)
	}
}

func TestFixtureAccountClientAuthenticatesPhoneAndPasswordDigest(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Login: "auth_success"})
	if err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	valid := fixtureRequest(t, actor, AccountOperationLogin, map[string]string{
		"phone": "13800000000", "password": "test-password-01",
	})
	if state, _, err := client.Login(context.Background(), valid); err != nil || state.State != AccountAuthenticated {
		t.Fatalf("valid phone/password Login = %#v, %v", state, err)
	}
	wrongPassword := "wrong-password-must-not-leak"
	invalid := fixtureRequest(t, actor, AccountOperationLogin, map[string]string{
		"phone": "13800000000", "password": wrongPassword,
	})
	if _, _, err := client.Login(context.Background(), invalid); err == nil {
		t.Fatal("Login accepted a password that does not match the fixture digest")
	} else {
		var cloudErr *Error
		if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorAuthRequired {
			t.Fatalf("invalid Login error = %v, want %s", err, ErrorAuthRequired)
		}
		if strings.Contains(err.Error(), wrongPassword) {
			t.Fatal("invalid Login error exposed the submitted password")
		}
	}
}

func TestFixtureAccountClientRoutesSecondAccountByCredential(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	request := fixtureRequest(t, actor, AccountOperationLogin, map[string]string{
		"phone": "13900000000", "password": "test-password-02",
	})
	state, session, err := client.Login(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	want := BindingScope{ProfileID: "default", CloudUserID: "usr_02", BindingGeneration: 2}
	if state.User.CloudUserID != want.CloudUserID || session.Scope != want {
		t.Fatalf("second account Login = user %#v, scope %#v, want %#v", state.User, session.Scope, want)
	}
}

func TestFixtureClientsProjectCanonicalAccountsIntoRequestProfiles(t *testing.T) {
	bundle := loadA0Fixture(t)
	accounts, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewFixturePlanClient(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	workActor := userctx.ActorContext{ActorID: "local", ProfileID: "work"}
	state, session, err := accounts.Login(ctx, fixtureRequest(t, workActor, AccountOperationLogin, map[string]string{
		"phone": "13900000000", "password": "test-password-02",
	}))
	if err != nil {
		t.Fatalf("work profile Login: %v", err)
	}
	workScope := BindingScope{ProfileID: "work", CloudUserID: "usr_02", BindingGeneration: 2}
	if state.Binding.ProfileID != workActor.ProfileID || state.Binding.CloudUserID != workScope.CloudUserID || state.Binding.BindingGeneration != workScope.BindingGeneration {
		t.Fatalf("work Login binding = %#v, want %#v", state.Binding, workScope)
	}
	if session.Scope != workScope || state.Subscription.Scope != workScope {
		t.Fatalf("work Login scopes = session %#v subscription %#v, want %#v", session.Scope, state.Subscription.Scope, workScope)
	}
	refreshed, err := accounts.RefreshSession(ctx, session)
	if err != nil || refreshed.Scope != workScope {
		t.Fatalf("work RefreshSession = %#v, %v, want scope %#v", refreshed, err, workScope)
	}
	unknown := session
	unknown.Scope.CloudUserID = "usr_unknown"
	if _, err := accounts.RefreshSession(ctx, unknown); !isProfileMismatch(err) {
		t.Fatalf("unknown identity RefreshSession error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	current, err := accounts.GetCurrentUser(ctx, workScope)
	if err != nil || current.Binding.ProfileID != workActor.ProfileID || current.Subscription.Scope != workScope {
		t.Fatalf("work GetCurrentUser = %#v, %v", current, err)
	}
	if got, err := plans.ListSubscriptionPlans(ctx, workScope); err != nil || !reflect.DeepEqual(got, bundle.PlanCases) {
		t.Fatalf("work ListSubscriptionPlans = %#v, %v", got, err)
	}

	otherActor := userctx.ActorContext{ActorID: "local", ProfileID: "personal"}
	other, otherSession, err := accounts.Login(ctx, fixtureRequest(t, otherActor, AccountOperationLogin, map[string]string{
		"phone": "13900000000", "password": "test-password-02",
	}))
	if err != nil {
		t.Fatalf("personal profile Login: %v", err)
	}
	otherScope := BindingScope{ProfileID: "personal", CloudUserID: "usr_02", BindingGeneration: 2}
	if other.Binding.ProfileID != otherActor.ProfileID || otherSession.Scope != otherScope || other.Subscription.Scope != otherScope {
		t.Fatalf("personal Login = state %#v session %#v, want isolated scope %#v", other, otherSession, otherScope)
	}
	again, err := accounts.GetCurrentUser(ctx, workScope)
	if err != nil || again.Binding.ProfileID != workActor.ProfileID || again.Subscription.Scope != workScope {
		t.Fatalf("work scope changed after personal Login = %#v, %v", again, err)
	}
}

func TestFixtureClientsProjectCanonicalSecondaryIntoRepositoryGeneration(t *testing.T) {
	bundle := loadA0Fixture(t)
	accounts, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewFixturePlanClient(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "fresh-secondary"}
	_, canonicalSession, err := accounts.Login(ctx, fixtureRequest(t, actor, AccountOperationLogin, map[string]string{
		"phone": "13900000000", "password": "test-password-02",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if canonicalSession.Scope.CloudUserID != "usr_02" || canonicalSession.Scope.BindingGeneration != 2 {
		t.Fatalf("canonical secondary Login scope = %#v, want usr_02 generation 2", canonicalSession.Scope)
	}

	localScope := BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_02", BindingGeneration: 1}
	localSession := canonicalSession
	localSession.Scope = localScope
	refreshed, err := accounts.RefreshSession(ctx, localSession)
	if err != nil || refreshed.Scope != localScope {
		t.Fatalf("local secondary RefreshSession = %#v, %v, want scope %#v", refreshed, err, localScope)
	}
	current, err := accounts.GetCurrentUser(ctx, localScope)
	if err != nil || current.Binding.ProfileID != localScope.ProfileID || current.Binding.CloudUserID != localScope.CloudUserID || current.Binding.BindingGeneration != localScope.BindingGeneration || current.Balance.TotalBalance != "5000.0000" || current.Subscription.Scope != localScope || current.Subscription.Plan != "lite" {
		t.Fatalf("local secondary GetCurrentUser = %#v, %v", current, err)
	}
	if got, err := plans.ListSubscriptionPlans(ctx, localScope); err != nil || !reflect.DeepEqual(got, bundle.PlanCases) {
		t.Fatalf("local secondary ListSubscriptionPlans = %#v, %v", got, err)
	}

	unknownScope := localScope
	unknownScope.CloudUserID = "usr_unknown"
	unknownSession := localSession
	unknownSession.Scope = unknownScope
	if _, err := accounts.RefreshSession(ctx, unknownSession); !isProfileMismatch(err) {
		t.Fatalf("unknown cloud user RefreshSession error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	if _, err := accounts.GetCurrentUser(ctx, unknownScope); !isProfileMismatch(err) {
		t.Fatalf("unknown cloud user GetCurrentUser error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	if _, err := plans.ListSubscriptionPlans(ctx, unknownScope); !isProfileMismatch(err) {
		t.Fatalf("unknown cloud user ListSubscriptionPlans error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}

	invalidScope := localScope
	invalidScope.BindingGeneration = 0
	invalidSession := localSession
	invalidSession.Scope = invalidScope
	if _, err := accounts.RefreshSession(ctx, invalidSession); err == nil {
		t.Fatal("RefreshSession accepted invalid local scope")
	}
	if _, err := accounts.GetCurrentUser(ctx, invalidScope); err == nil {
		t.Fatal("GetCurrentUser accepted invalid local scope")
	}
	if _, err := plans.ListSubscriptionPlans(ctx, invalidScope); err == nil {
		t.Fatal("ListSubscriptionPlans accepted invalid local scope")
	}
}

func fixtureRequest(t *testing.T, actor userctx.ActorContext, operation AccountOperation, payload any) AccountFixtureRequest {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return AccountFixtureRequest{Binding: ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: BindingBinding}, Operation: operation, FixtureVersion: A0FixtureVersion, Payload: data}
}

func TestFixtureAccountClientRejectsUnknownAccountForRefresh(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	session := *bundle.AuthCases[0].Session
	session.Scope.CloudUserID = "usr_unknown"
	_, err = client.RefreshSession(context.Background(), session)
	if !isProfileMismatch(err) {
		t.Fatalf("RefreshSession error = %v", err)
	}
	if !errors.Is(err, errFixtureAuth) {
		t.Fatalf("RefreshSession should preserve fixture boundary error: %v", err)
	}
}

func TestFixtureAccountClientRegisterRequiresEncryptedPasswordWithoutPlaintext(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	plaintextRequest := fixtureRequest(t, actor, AccountOperationRegister, map[string]any{
		"password":           "plaintext-must-not-pass",
		"encrypted_password": "fixture-ciphertext",
		"password_crypto":    map[string]any{"algorithm": "AES"},
	})
	if _, _, err := client.Register(context.Background(), plaintextRequest); err == nil {
		t.Fatal("Register accepted a plaintext password field")
	}
	validRequest := fixtureRequest(t, actor, AccountOperationRegister, map[string]any{
		"encrypted_password": "fixture-ciphertext",
		"password_crypto":    map[string]any{"algorithm": "AES"},
	})
	state, session, err := client.Register(context.Background(), validRequest)
	if err != nil || state.State != AccountAuthenticated || session.RefreshToken == "" {
		t.Fatalf("Register encrypted payload = %#v %#v %v", state, session, err)
	}
}

func TestFixtureAccountClientUsesOnlyMatchingScopeSnapshots(t *testing.T) {
	bundle := loadA0Fixture(t)
	for _, scenario := range []string{"auth_success", "auth_success_secondary"} {
		t.Run(scenario, func(t *testing.T) {
			fixture, ok := authFixtureByName(bundle.AuthCases, scenario)
			if !ok {
				t.Fatalf("missing %s auth fixture", scenario)
			}
			scope, err := fixture.Binding.Scope()
			if err != nil {
				t.Fatal(err)
			}
			snapshot, ok := accountSnapshotByScope(bundle.AccountSnapshots, scope)
			if !ok {
				t.Fatalf("A0 fixture has no account snapshot for scope %#v", scope)
			}
			client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{Login: scenario})
			if err != nil {
				t.Fatal(err)
			}
			actor := userctx.ActorContext{ActorID: fixture.Binding.ActorID, ProfileID: scope.ProfileID}
			phone, password := "13800000000", "test-password-01"
			if scenario == "auth_success_secondary" {
				phone, password = "13900000000", "test-password-02"
			}
			state, session, err := client.Login(context.Background(), fixtureRequest(t, actor, AccountOperationLogin, map[string]string{"phone": phone, "password": password}))
			if err != nil {
				t.Fatal(err)
			}
			if state.Binding != fixture.Binding || state.User.CloudUserID != scope.CloudUserID || session.Scope != scope {
				t.Fatalf("state scope = %#v, session scope = %#v, want %#v", state, session.Scope, scope)
			}
			if state.Balance != snapshot.Balance || state.Subscription != snapshot.Subscription || state.LastSyncAt != snapshot.Subscription.UpdatedAt {
				t.Fatalf("state used a snapshot outside its binding scope: %#v", state)
			}
		})
	}
}

func TestFixtureAccountClientRejectsInvalidAccountSnapshotScopes(t *testing.T) {
	base := loadA0Fixture(t)
	tests := []struct {
		name   string
		mutate func(*FixtureBundle)
	}{
		{"missing secondary", func(b *FixtureBundle) { b.AccountSnapshots = b.AccountSnapshots[:1] }},
		{"unknown scope", func(b *FixtureBundle) {
			b.AccountSnapshots[1].Scope.BindingGeneration++
			b.AccountSnapshots[1].Subscription.Scope.BindingGeneration++
		}},
		{"cross scope subscription", func(b *FixtureBundle) { b.AccountSnapshots[1].Subscription.Scope = b.AccountSnapshots[0].Scope }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := base
			bundle.AccountSnapshots = append([]AccountSnapshotFixture(nil), base.AccountSnapshots...)
			tt.mutate(&bundle)
			if _, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{}); err == nil {
				t.Fatal("NewFixtureAccountClient accepted invalid account snapshots")
			}
		})
	}
}

func TestFixtureClientsRejectAmbiguousCanonicalCloudUserID(t *testing.T) {
	bundle := loadA0Fixture(t)
	primaryCloudUserID := bundle.AccountSnapshots[0].Scope.CloudUserID
	for index := range bundle.AuthCases {
		if bundle.AuthCases[index].Name != "auth_success_secondary" {
			continue
		}
		bundle.AuthCases[index].Binding.CloudUserID = primaryCloudUserID
		bundle.AuthCases[index].Session.Scope.CloudUserID = primaryCloudUserID
	}
	bundle.AccountSnapshots[1].Scope.CloudUserID = primaryCloudUserID
	bundle.AccountSnapshots[1].Subscription.Scope.CloudUserID = primaryCloudUserID
	if _, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{}); err == nil {
		t.Fatal("NewFixtureAccountClient accepted an ambiguous canonical CloudUserID")
	}
	if _, err := NewFixturePlanClient(bundle); err == nil {
		t.Fatal("NewFixturePlanClient accepted an ambiguous canonical CloudUserID")
	}
}

func TestFixturePlanClientReturnsBundlePlansOnlyForAccountScopes(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixturePlanClient(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range bundle.AccountSnapshots {
		plans, err := client.ListSubscriptionPlans(context.Background(), snapshot.Scope)
		if err != nil || !reflect.DeepEqual(plans, bundle.PlanCases) {
			t.Fatalf("ListSubscriptionPlans(%#v) = %#v, %v", snapshot.Scope, plans, err)
		}
		plans[0].PriceAmount = "locally-mutated"
		again, err := client.ListSubscriptionPlans(context.Background(), snapshot.Scope)
		if err != nil || again[0].PriceAmount != bundle.PlanCases[0].PriceAmount {
			t.Fatalf("fixture plans were mutated by a caller: %#v, %v", again, err)
		}
	}
	unknown := bundle.AccountSnapshots[0].Scope
	unknown.CloudUserID = "usr_unknown"
	if _, err := client.ListSubscriptionPlans(context.Background(), unknown); !isProfileMismatch(err) {
		t.Fatalf("unknown scope error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
}

func TestFixtureAccountClientReturnsBundleSnapshotForEveryAccountScope(t *testing.T) {
	bundle := loadA0Fixture(t)
	client, err := NewFixtureAccountClient(bundle, FixtureAccountScenarios{})
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range bundle.AccountSnapshots {
		state, err := client.GetCurrentUser(context.Background(), snapshot.Scope)
		if err != nil {
			t.Fatalf("GetCurrentUser(%#v): %v", snapshot.Scope, err)
		}
		if state.Balance != snapshot.Balance || state.Subscription != snapshot.Subscription || state.User.CloudUserID != snapshot.Scope.CloudUserID {
			t.Fatalf("GetCurrentUser(%#v) = %#v, want bundle snapshot", snapshot.Scope, state)
		}
	}
}
