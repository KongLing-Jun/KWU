package cloud

import (
	"bytes"
	_ "embed"
	"fmt"
)

//go:embed testdata/a0-v1.json
var canonicalA0Fixture []byte

// LoadCanonicalA0Fixture returns the strict, embedded fake-cloud bundle used
// by production Desktop composition when KWU_CLOUD_FAKE=1.
func LoadCanonicalA0Fixture() (FixtureBundle, error) {
	return DecodeFixture(bytes.NewReader(canonicalA0Fixture))
}

func validateAuthFixtures(fixtures []AuthFixture, scope BindingScope) error {
	names, err := validateAuthFixtureEntries(fixtures)
	if err != nil {
		return err
	}
	if err := validateNamedSet("auth fixtures", names, []string{
		"auth_success", "auth_success_secondary", "access_expired", "refresh_success", "refresh_failed", "profile_mismatch", "sms_code_invalid",
	}); err != nil {
		return err
	}
	seenPhones := make(map[string]string)
	for _, fixture := range fixtures {
		if err := validateAuthFixtureScenario(fixture, scope); err != nil {
			return err
		}
		if fixture.LoginCredential == nil {
			continue
		}
		phone := fixture.LoginCredential.Phone
		if previous, ok := seenPhones[phone]; ok {
			return fmt.Errorf("cloud: auth fixtures %q and %q use duplicate login phone", previous, fixture.Name)
		}
		seenPhones[phone] = fixture.Name
	}
	return nil
}

func validateAuthFixtureEntries(fixtures []AuthFixture) ([]string, error) {
	names := make([]string, 0, len(fixtures))
	for i, fixture := range fixtures {
		if err := validateAuthFixtureEntry(fixture, i); err != nil {
			return nil, err
		}
		names = append(names, fixture.Name)
	}
	return names, nil
}

func validateAuthFixtureEntry(fixture AuthFixture, index int) error {
	if err := requireTrimmed("auth fixture name", fixture.Name); err != nil {
		return fmt.Errorf("cloud: auth fixture %d: %w", index, err)
	}
	if !fixture.FixtureOnly {
		return fmt.Errorf("cloud: auth fixture %d must be fixture-only", index)
	}
	if fixture.LoginCredential != nil {
		if err := fixture.LoginCredential.Validate(); err != nil {
			return fmt.Errorf("cloud: auth fixture %d: %w", index, err)
		}
	}
	if err := fixture.Binding.Validate(); err != nil {
		return fmt.Errorf("cloud: auth fixture %d: %w", index, err)
	}
	if err := validateSessionState(fixture.State); err != nil {
		return fmt.Errorf("cloud: auth fixture %d: %w", index, err)
	}
	if err := validateAuthFixtureSession(fixture, index); err != nil {
		return err
	}
	return validateAuthFixtureError(fixture, index)
}

func validateAuthFixtureSession(fixture AuthFixture, index int) error {
	if fixture.Session == nil {
		return nil
	}
	if err := fixture.Session.Validate(); err != nil {
		return fmt.Errorf("cloud: auth fixture %d: %w", index, err)
	}
	if fixture.Session.State != fixture.State {
		return fmt.Errorf("cloud: auth fixture %d session state does not match", index)
	}
	return nil
}

func validateAuthFixtureError(fixture AuthFixture, index int) error {
	if fixture.Error == nil {
		return nil
	}
	if err := validateFixtureError(fixture.Error); err != nil {
		return fmt.Errorf("cloud: auth fixture %d: %w", index, err)
	}
	return nil
}

func validateAuthFixtureScenario(fixture AuthFixture, scope BindingScope) error {
	bindingScope, err := validateAuthFixtureScopeAndCredential(fixture)
	if err != nil {
		return err
	}
	switch fixture.Name {
	case "auth_success", "refresh_success":
		return validateBoundAuthenticatedAuthFixture(fixture, bindingScope, scope)
	case "auth_success_secondary":
		return validateSecondBoundAuthenticatedAuthFixture(fixture, bindingScope, scope)
	case "access_expired":
		return validateExpiredAuthFixture(fixture, bindingScope, scope)
	case "refresh_failed":
		return validateRefreshFailedAuthFixture(fixture, bindingScope, scope)
	case "profile_mismatch":
		return validateProfileMismatchAuthFixture(fixture, bindingScope, scope)
	case "sms_code_invalid":
		return validateSMSCodeInvalidAuthFixture(fixture, bindingScope, scope)
	}
	return nil
}

func validateAuthFixtureScopeAndCredential(fixture AuthFixture) (BindingScope, error) {
	bindingScope, err := fixture.Binding.Scope()
	if err != nil {
		return BindingScope{}, fmt.Errorf("cloud: auth fixture %q: %w", fixture.Name, err)
	}
	if fixture.Session != nil && fixture.Session.Scope != bindingScope {
		return BindingScope{}, fmt.Errorf("cloud: auth fixture %q session belongs to another binding", fixture.Name)
	}
	hasLoginCredential := fixture.LoginCredential != nil
	requiresLoginCredential := fixture.Name == "auth_success" || fixture.Name == "auth_success_secondary"
	if hasLoginCredential != requiresLoginCredential {
		return BindingScope{}, fmt.Errorf("cloud: auth fixture %q has an invalid login credential assignment", fixture.Name)
	}
	return bindingScope, nil
}

func validateBoundAuthenticatedAuthFixture(fixture AuthFixture, bindingScope, scope BindingScope) error {
	if fixture.Binding.Status != BindingBound || bindingScope != scope || fixture.State != SessionAuthenticated || fixture.Session == nil || fixture.Error != nil {
		return fmt.Errorf("cloud: auth fixture %q must be a bound authenticated fixture", fixture.Name)
	}
	return nil
}

func validateSecondBoundAuthenticatedAuthFixture(fixture AuthFixture, bindingScope, scope BindingScope) error {
	if fixture.Binding.Status != BindingBound ||
		bindingScope.ProfileID != scope.ProfileID ||
		bindingScope.CloudUserID == scope.CloudUserID ||
		bindingScope.BindingGeneration != scope.BindingGeneration+1 ||
		fixture.State != SessionAuthenticated || fixture.Session == nil || fixture.Error != nil {
		return fmt.Errorf("cloud: auth_success_secondary fixture is invalid")
	}
	return nil
}

func validateExpiredAuthFixture(fixture AuthFixture, bindingScope, scope BindingScope) error {
	if fixture.Binding.Status != BindingBound || bindingScope != scope || fixture.State != SessionExpired || fixture.Session == nil || fixture.Error != nil {
		return fmt.Errorf("cloud: access_expired fixture is invalid")
	}
	return nil
}

func validateRefreshFailedAuthFixture(fixture AuthFixture, bindingScope, scope BindingScope) error {
	if fixture.Binding.Status != BindingExpired || bindingScope != scope || fixture.State != SessionExpired || fixture.Error == nil {
		return fmt.Errorf("cloud: refresh_failed fixture is invalid")
	}
	return nil
}

func validateProfileMismatchAuthFixture(fixture AuthFixture, bindingScope, scope BindingScope) error {
	if fixture.Binding.Status != BindingMismatch || bindingScope == scope || fixture.State != SessionOffline || fixture.Error == nil || fixture.Error.Code != ErrorProfileScopeMismatch {
		return fmt.Errorf("cloud: profile_mismatch fixture is invalid")
	}
	return nil
}

func validateSMSCodeInvalidAuthFixture(fixture AuthFixture, bindingScope, scope BindingScope) error {
	if fixture.Binding.Status != BindingBound || bindingScope != scope || fixture.State != SessionOffline || fixture.Session != nil || fixture.Error == nil || fixture.Error.Code != ErrorSMSCodeInvalid {
		return fmt.Errorf("cloud: sms_code_invalid fixture is invalid")
	}
	return nil
}

func validateBalanceFixtures(fixtures []BalanceFixture, scope BindingScope) error {
	names := make([]string, 0, len(fixtures))
	for i, fixture := range fixtures {
		if err := requireTrimmed("balance fixture name", fixture.Name); err != nil {
			return fmt.Errorf("cloud: balance fixture %d: %w", i, err)
		}
		if fixture.Scope != scope {
			return fmt.Errorf("cloud: balance fixture %d belongs to another binding", i)
		}
		if err := fixture.Balance.Validate(); err != nil {
			return fmt.Errorf("cloud: balance fixture %d: %w", i, err)
		}
		names = append(names, fixture.Name)
	}
	return validateNamedSet("balance fixtures", names, []string{"balance_split"})
}

func validateAccountSnapshots(snapshots []AccountSnapshotFixture, authFixtures []AuthFixture) error {
	expectedScopes, err := successfulAuthScopes(authFixtures)
	if err != nil {
		return err
	}
	if len(snapshots) != len(expectedScopes) {
		return fmt.Errorf("cloud: account snapshots count = %d, want %d", len(snapshots), len(expectedScopes))
	}
	seen := make(map[BindingScope]bool, len(snapshots))
	for i, snapshot := range snapshots {
		if err := validateAccountSnapshot(snapshot, i, expectedScopes, seen); err != nil {
			return err
		}
		seen[snapshot.Scope] = true
	}
	for scope := range expectedScopes {
		if !seen[scope] {
			return fmt.Errorf("cloud: missing account snapshot for successful auth scope %#v", scope)
		}
	}
	return nil
}

func validateAccountSnapshot(snapshot AccountSnapshotFixture, index int, expectedScopes, seen map[BindingScope]bool) error {
	if err := snapshot.Scope.Validate(); err != nil {
		return fmt.Errorf("cloud: account snapshot %d: %w", index, err)
	}
	if !expectedScopes[snapshot.Scope] {
		return fmt.Errorf("cloud: account snapshot %d belongs to an unsuccessful auth scope", index)
	}
	if seen[snapshot.Scope] {
		return fmt.Errorf("cloud: duplicate account snapshot for scope %#v", snapshot.Scope)
	}
	if err := snapshot.Balance.Validate(); err != nil {
		return fmt.Errorf("cloud: account snapshot %d: %w", index, err)
	}
	if err := snapshot.Subscription.Validate(); err != nil || snapshot.Subscription.Scope != snapshot.Scope {
		return fmt.Errorf("cloud: account snapshot %d has an invalid cross-scope subscription", index)
	}
	return nil
}
func successfulAuthScopes(fixtures []AuthFixture) (map[BindingScope]bool, error) {
	scopes := make(map[BindingScope]bool)
	for _, fixture := range fixtures {
		if fixture.Binding.Status != BindingBound || fixture.State != SessionAuthenticated || fixture.Session == nil || fixture.Error != nil {
			continue
		}
		scope, err := fixture.Binding.Scope()
		if err != nil {
			return nil, err
		}
		scopes[scope] = true
	}
	return scopes, nil
}

func validatePlans(plans []SubscriptionPlan) error {
	seen := make(map[string]bool, len(plans))
	for i, plan := range plans {
		if err := plan.Validate(); err != nil {
			return fmt.Errorf("cloud: plan fixture %d: %w", i, err)
		}
		if seen[plan.Plan] {
			return fmt.Errorf("cloud: duplicate plan fixture %q", plan.Plan)
		}
		seen[plan.Plan] = true
	}
	return nil
}

func validatePaymentFixtures(fixtures []PaymentFixture, scope BindingScope) error {
	names := make([]string, 0, len(fixtures))
	entryExpired := 0
	for i, fixture := range fixtures {
		if err := requireTrimmed("payment fixture name", fixture.Name); err != nil {
			return fmt.Errorf("cloud: payment fixture %d: %w", i, err)
		}
		if err := fixture.Result.Validate(); err != nil {
			return fmt.Errorf("cloud: payment fixture %d: %w", i, err)
		}
		if fixture.Result.Order.Scope != scope || fixture.Name != string(fixture.Result.Order.Status) {
			return fmt.Errorf("cloud: payment fixture %d belongs to another binding or has the wrong name", i)
		}
		if fixture.EntryExpired {
			entryExpired++
		}
		names = append(names, fixture.Name)
	}
	if err := validateNamedSet("payment fixtures", names, []string{
		"created", "payment_pending", "callback_received", "verifying", "paid_verified", "granting", "granted", "closed", "timeout", "failed", "verify_failed_manual_required",
	}); err != nil {
		return err
	}
	if entryExpired != 1 {
		return fmt.Errorf("cloud: fixture requires exactly one expired payment entry")
	}
	return nil
}

func validatePaymentCreateTapeContract(creates []PaymentCreateFixture, tapes []PaymentTapeFixture, accounts []AccountSnapshotFixture) error {
	if err := validatePaymentTapes(tapes, accounts); err != nil {
		return err
	}
	seenKeys := make(map[string]bool, len(creates))
	for i, create := range creates {
		if err := validatePaymentCreateFixture(create, i, seenKeys); err != nil {
			return err
		}
		matches := matchingPaymentTapeCount(create.Result.Order, tapes)
		if matches != 1 {
			return fmt.Errorf("cloud: payment create fixture %d matches %d tapes, want exactly one", i, matches)
		}
	}
	return validateNamedSet("payment create fixtures", mapKeys(seenKeys), []string{
		"purchase-lite-alipay", "purchase-lite-wechat",
		"purchase-plus-alipay", "purchase-plus-wechat",
		"purchase-pro-alipay", "purchase-pro-wechat",
		"upgrade-lite-plus-alipay", "upgrade-lite-plus-wechat",
		"upgrade-lite-pro-alipay", "upgrade-lite-pro-wechat",
		"upgrade-plus-pro-alipay", "upgrade-plus-pro-wechat",
	})
}

func validatePaymentCreateFixture(create PaymentCreateFixture, index int, seenKeys map[string]bool) error {
	if err := requireTrimmed("payment create key", create.Key); err != nil {
		return fmt.Errorf("cloud: payment create fixture %d: %w", index, err)
	}
	if seenKeys[create.Key] {
		return fmt.Errorf("cloud: duplicate payment create key %q", create.Key)
	}
	seenKeys[create.Key] = true
	if err := create.Result.Validate(); err != nil {
		return fmt.Errorf("cloud: payment create fixture %d: %w", index, err)
	}
	if create.Result.Order.Status != PaymentPending {
		return fmt.Errorf("cloud: payment create fixture %d must be payment_pending", index)
	}
	expectedKey, err := canonicalPaymentCreateKey(create.Result.Order)
	if err != nil || create.Key != expectedKey {
		return fmt.Errorf("cloud: payment create fixture %d key does not match its dimensions", index)
	}
	return nil
}

func matchingPaymentTapeCount(order PaymentOrder, tapes []PaymentTapeFixture) int {
	matches := 0
	for _, tape := range tapes {
		if len(tape.Results) > 0 && samePaymentOrderIdentity(order, tape.Results[0].Order) {
			matches++
		}
	}
	return matches
}

func canonicalPaymentCreateKey(order PaymentOrder) (string, error) {
	channel := string(order.PaymentChannel)
	if order.OrderType == PaymentOrderSubscriptionPurchase {
		switch order.Plan {
		case "lite", "plus", "pro":
			return "purchase-" + order.Plan + "-" + channel, nil
		default:
			return "", fmt.Errorf("cloud: unsupported canonical purchase plan")
		}
	}
	if order.OrderType != PaymentOrderSubscriptionUpgrade {
		return "", fmt.Errorf("cloud: unsupported canonical payment operation")
	}
	edge := order.FromPlan + "-" + order.Plan
	switch edge {
	case "lite-plus", "lite-pro", "plus-pro":
		return "upgrade-" + edge + "-" + channel, nil
	default:
		return "", fmt.Errorf("cloud: unsupported canonical upgrade edge")
	}
}

func validatePaymentTapes(tapes []PaymentTapeFixture, accounts []AccountSnapshotFixture) error {
	accountsByUser := indexPaymentTapeAccounts(accounts)
	seenOrderIDs := make(map[string]bool, len(tapes))
	for i, tape := range tapes {
		if err := validatePaymentTapeIdentity(tape, i, seenOrderIDs); err != nil {
			return err
		}
		if err := validatePaymentTape(tape, i, accountsByUser); err != nil {
			return err
		}
	}
	return nil
}

func indexPaymentTapeAccounts(accounts []AccountSnapshotFixture) map[string]AccountSnapshotFixture {
	accountsByUser := make(map[string]AccountSnapshotFixture, len(accounts))
	for _, account := range accounts {
		accountsByUser[account.Scope.CloudUserID] = account
	}
	return accountsByUser
}

func validatePaymentTapeIdentity(tape PaymentTapeFixture, index int, seenOrderIDs map[string]bool) error {
	if err := requireTrimmed("payment tape order_id", tape.OrderID); err != nil {
		return fmt.Errorf("cloud: payment tape fixture %d: %w", index, err)
	}
	if seenOrderIDs[tape.OrderID] {
		return fmt.Errorf("cloud: duplicate payment tape order_id %q", tape.OrderID)
	}
	seenOrderIDs[tape.OrderID] = true
	return nil
}

func validatePaymentTape(tape PaymentTapeFixture, index int, accountsByUser map[string]AccountSnapshotFixture) error {
	if len(tape.Results) != 3 {
		return fmt.Errorf("cloud: payment tape fixture %d requires pending, verifying, and terminal results", index)
	}
	first := tape.Results[0].Order
	if err := validatePaymentTapeResults(tape, index, first); err != nil {
		return err
	}
	if tape.Results[0].Order.Status != PaymentPending || tape.Results[1].Order.Status != PaymentVerifying {
		return fmt.Errorf("cloud: payment tape fixture %d must start payment_pending then verifying", index)
	}
	terminal := tape.Results[len(tape.Results)-1].Order
	if err := validatePaymentTapeTerminal(first, terminal, index); err != nil {
		return err
	}
	return validatePaymentTapeSnapshots(tape, index, first, terminal, accountsByUser)
}

func validatePaymentTapeResults(tape PaymentTapeFixture, index int, first PaymentOrder) error {
	for resultIndex, result := range tape.Results {
		if err := result.Validate(); err != nil {
			return fmt.Errorf("cloud: payment tape fixture %d result %d: %w", index, resultIndex, err)
		}
		if result.Order.OrderID != tape.OrderID || !samePaymentOrderIdentity(first, result.Order) {
			return fmt.Errorf("cloud: payment tape fixture %d result %d changes order identity", index, resultIndex)
		}
	}
	return nil
}

func validatePaymentTapeTerminal(first, terminal PaymentOrder, index int) error {
	failureMatrix := first.OrderType == PaymentOrderSubscriptionPurchase && first.Plan == "lite" && first.PaymentChannel == PaymentChannelWeChat
	if (failureMatrix && terminal.Status != PaymentFailed) || (!failureMatrix && terminal.Status != PaymentGranted) {
		return fmt.Errorf("cloud: payment tape fixture %d has the wrong deterministic terminal status", index)
	}
	return nil
}

func validatePaymentTapeSnapshots(tape PaymentTapeFixture, index int, first, terminal PaymentOrder, accountsByUser map[string]AccountSnapshotFixture) error {
	if terminal.Status == PaymentGranted && terminal.GrantStatus == GrantGranted {
		return validateGrantedPaymentTapeSnapshots(tape, index, first, accountsByUser)
	}
	if len(tape.GrantedAccountSnapshots) != 0 {
		return fmt.Errorf("cloud: non-granted payment tape fixture %d must not include account snapshots", index)
	}
	return nil
}

func validateGrantedPaymentTapeSnapshots(tape PaymentTapeFixture, index int, first PaymentOrder, accountsByUser map[string]AccountSnapshotFixture) error {
	seenUsers := make(map[string]bool, len(tape.GrantedAccountSnapshots))
	for snapshotIndex, snapshot := range tape.GrantedAccountSnapshots {
		account, ok := accountsByUser[snapshot.Scope.CloudUserID]
		if !ok || seenUsers[snapshot.Scope.CloudUserID] {
			return fmt.Errorf("cloud: payment tape fixture %d has an unexpected or duplicate granted account snapshot %d", index, snapshotIndex)
		}
		if snapshot.Scope != account.Scope {
			return fmt.Errorf("cloud: payment tape fixture %d granted account snapshot %d changes canonical identity", index, snapshotIndex)
		}
		if err := validateGrantedAccountSnapshot(snapshot, first); err != nil {
			return fmt.Errorf("cloud: payment tape fixture %d: %w", index, err)
		}
		if snapshot.Balance.Version <= account.Balance.Version {
			return fmt.Errorf("cloud: payment tape fixture %d granted snapshot version does not advance", index)
		}
		seenUsers[snapshot.Scope.CloudUserID] = true
	}
	if len(seenUsers) != len(accountsByUser) {
		return fmt.Errorf("cloud: payment tape fixture %d granted account snapshots cover %d users, want %d", index, len(seenUsers), len(accountsByUser))
	}
	return nil
}

func validateGrantedAccountSnapshot(snapshot AccountSnapshotFixture, order PaymentOrder) error {
	if snapshot.Subscription.Scope != snapshot.Scope {
		return fmt.Errorf("cloud: granted account snapshot has a cross-scope subscription")
	}
	if err := snapshot.Balance.Validate(); err != nil {
		return fmt.Errorf("cloud: granted account balance: %w", err)
	}
	if err := snapshot.Subscription.Validate(); err != nil {
		return fmt.Errorf("cloud: granted account subscription: %w", err)
	}
	if snapshot.Subscription.Plan != order.Plan || snapshot.Subscription.Status != SubscriptionActive {
		return fmt.Errorf("cloud: granted account snapshot does not contain the target plan")
	}
	return nil
}

func samePaymentOrderIdentity(left, right PaymentOrder) bool {
	return samePaymentOrderCoreIdentity(left, right) &&
		left.Amount == right.Amount &&
		left.AmountUnit == right.AmountUnit &&
		left.Currency == right.Currency &&
		left.ExpiresAt.Equal(right.ExpiresAt)
}

func samePaymentOrderCoreIdentity(left, right PaymentOrder) bool {
	return left.OrderID == right.OrderID &&
		left.Scope == right.Scope &&
		left.OrderType == right.OrderType &&
		(left.PlanID == right.PlanID || left.PlanID == "" || right.PlanID == "") &&
		left.Plan == right.Plan &&
		left.FromPlan == right.FromPlan &&
		left.PaymentChannel == right.PaymentChannel
}
