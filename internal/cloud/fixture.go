package cloud

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	S0FixtureVersion = "s0-v1"
	A0FixtureVersion = "a0-v1"
	FixtureVersion   = S0FixtureVersion
)

// FixtureMetadata identifies a local normalized fixture. It must never be
// treated as a cloud wire contract or evidence of L2 integration.
type FixtureMetadata struct {
	MockOnly     bool     `json:"mock_only"`
	L2Integrated bool     `json:"l2_integrated"`
	Gates        []string `json:"gates"`
}

type AuthFixture struct {
	Name            string                  `json:"name"`
	FixtureOnly     bool                    `json:"fixture_only"`
	LoginCredential *FixtureLoginCredential `json:"login_credential,omitempty"`
	Binding         ProfileBinding          `json:"binding"`
	State           SessionState            `json:"state"`
	Session         *Session                `json:"session,omitempty"`
	Error           *Error                  `json:"error,omitempty"`
}

// FixtureLoginCredential is a fake-only user-form credential. The fixture
// stores only a one-way digest; plaintext passwords never enter embedded data.
type FixtureLoginCredential struct {
	Phone          string `json:"phone"`
	PasswordDigest string `json:"password_digest"`
}

func (c FixtureLoginCredential) Validate() error {
	if len(c.Phone) != 11 || c.Phone[0] != '1' || strings.TrimSpace(c.Phone) != c.Phone {
		return fmt.Errorf("cloud: fixture login phone must be an 11-digit user-form phone")
	}
	for _, digit := range c.Phone {
		if digit < '0' || digit > '9' {
			return fmt.Errorf("cloud: fixture login phone must contain only digits")
		}
	}
	digest, err := hex.DecodeString(c.PasswordDigest)
	if err != nil || len(digest) != 32 || c.PasswordDigest != strings.ToLower(c.PasswordDigest) {
		return fmt.Errorf("cloud: fixture login password_digest must be lowercase SHA-256")
	}
	return nil
}

type BalanceFixture struct {
	Name    string          `json:"name"`
	Scope   BindingScope    `json:"scope"`
	Balance BalanceSnapshot `json:"balance"`
}

// AccountSnapshotFixture is a fixture-only normalized account read model. It
// is not a cloud wire contract, recovery response, or production protocol.
type AccountSnapshotFixture struct {
	Scope        BindingScope    `json:"scope"`
	Balance      BalanceSnapshot `json:"balance"`
	Subscription Subscription    `json:"subscription"`
}

type PaymentFixture struct {
	Name         string             `json:"name"`
	Result       PaymentOrderResult `json:"result"`
	EntryExpired bool               `json:"entry_expired,omitempty"`
}

type PaymentCreateFixture struct {
	Key    string             `json:"key"`
	Result PaymentOrderResult `json:"result"`
}

type PaymentTapeFixture struct {
	OrderID                 string                   `json:"order_id"`
	Results                 []PaymentOrderResult     `json:"results"`
	GrantedAccountSnapshots []AccountSnapshotFixture `json:"granted_account_snapshots,omitempty"`
}

// RecoveryFixture is a fixture-only REST recovery snapshot after a normalized
// event stream disconnect. It does not model a REST endpoint or transport.
type RecoveryFixture struct {
	Name                     string          `json:"name"`
	Scope                    BindingScope    `json:"scope"`
	DisconnectedAfterEventID string          `json:"disconnected_after_event_id"`
	LastEventID              string          `json:"last_event_id"`
	Balance                  BalanceSnapshot `json:"balance"`
	Subscription             Subscription    `json:"subscription"`
	Freezes                  []FreezeResult  `json:"freezes"`
	PaymentOrders            []PaymentOrder  `json:"payment_orders"`
}

type FixtureBundle struct {
	Version          string                   `json:"fixture_version"`
	Metadata         FixtureMetadata          `json:"metadata"`
	Binding          ProfileBinding           `json:"binding"`
	Sessions         []Session                `json:"sessions"`
	AuthCases        []AuthFixture            `json:"auth_cases"`
	BalanceCases     []BalanceFixture         `json:"balance_cases"`
	AccountSnapshots []AccountSnapshotFixture `json:"account_snapshots"`
	FreezeCases      []FreezeFixture          `json:"freeze_cases"`
	PlanCases        []SubscriptionPlan       `json:"plan_cases"`
	PaymentCases     []PaymentFixture         `json:"payment_cases"`
	PaymentCreates   []PaymentCreateFixture   `json:"payment_creates"`
	PaymentTapes     []PaymentTapeFixture     `json:"payment_tapes"`
	SyncCases        []ThirdPartySync         `json:"sync_cases"`
	Events           []Event                  `json:"events"`
	RecoveryCases    []RecoveryFixture        `json:"recovery_cases"`
}

func DecodeFixture(r io.Reader) (FixtureBundle, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return FixtureBundle{}, fmt.Errorf("cloud: read fixture: %w", err)
	}
	var version struct {
		Version string `json:"fixture_version"`
	}
	if err := decodeFixtureJSON(data, &version, false); err != nil {
		return FixtureBundle{}, err
	}
	switch version.Version {
	case S0FixtureVersion:
		var fixture s0FixtureSchema
		if err := decodeFixtureJSON(data, &fixture, true); err != nil {
			return FixtureBundle{}, err
		}
		bundle := FixtureBundle{
			Version:     fixture.Version,
			Binding:     fixture.Binding,
			Sessions:    fixture.Sessions,
			FreezeCases: fixture.FreezeCases,
			Events:      fixture.Events,
		}
		if err := bundle.Validate(); err != nil {
			return FixtureBundle{}, err
		}
		return bundle, nil
	case A0FixtureVersion:
		var bundle FixtureBundle
		if err := decodeFixtureJSON(data, &bundle, true); err != nil {
			return FixtureBundle{}, err
		}
		if err := bundle.Validate(); err != nil {
			return FixtureBundle{}, err
		}
		return bundle, nil
	default:
		return FixtureBundle{}, fmt.Errorf("cloud: unsupported fixture_version %q", version.Version)
	}
}

type s0FixtureSchema struct {
	Version     string          `json:"fixture_version"`
	Binding     ProfileBinding  `json:"binding"`
	Sessions    []Session       `json:"sessions"`
	FreezeCases []FreezeFixture `json:"freeze_cases"`
	Events      []Event         `json:"events"`
}

func decodeFixtureJSON(data []byte, target any, strict bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("cloud: decode fixture: %w", err)
	}
	return ensureJSONEOF(decoder)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("cloud: fixture contains multiple JSON values")
		}
		return fmt.Errorf("cloud: decode fixture trailer: %w", err)
	}
	return nil
}

func (b FixtureBundle) Validate() error {
	switch b.Version {
	case S0FixtureVersion:
		return b.validateS0()
	case A0FixtureVersion:
		return b.validateA0()
	default:
		return fmt.Errorf("cloud: unsupported fixture_version %q", b.Version)
	}
}

func (b FixtureBundle) validateS0() error {
	if err := b.Binding.Validate(); err != nil {
		return fmt.Errorf("cloud: binding fixture: %w", err)
	}
	scope, err := b.Binding.Scope()
	if err != nil {
		return fmt.Errorf("cloud: binding fixture: %w", err)
	}
	if len(b.Sessions) == 0 || len(b.FreezeCases) == 0 || len(b.Events) == 0 {
		return fmt.Errorf("cloud: fixture requires sessions, freeze_cases, and events")
	}
	if err := validateFixtureSessions(b.Sessions, scope); err != nil {
		return err
	}
	if err := validateS0FixtureFreezes(b.FreezeCases, scope); err != nil {
		return err
	}
	return validateS0FixtureEvents(b.Events)
}

func (b FixtureBundle) validateA0() error {
	scope, err := b.validateA0Binding()
	if err != nil {
		return err
	}
	if err := validateRequiredA0FixtureCollections(b); err != nil {
		return err
	}
	return validateA0FixtureCollections(b, scope)
}

func (b FixtureBundle) validateA0Binding() (BindingScope, error) {
	if err := b.Metadata.Validate(); err != nil {
		return BindingScope{}, err
	}
	if err := b.Binding.Validate(); err != nil {
		return BindingScope{}, fmt.Errorf("cloud: binding fixture: %w", err)
	}
	scope, err := b.Binding.Scope()
	if err != nil {
		return BindingScope{}, fmt.Errorf("cloud: binding fixture: %w", err)
	}
	return scope, nil
}

func validateRequiredA0FixtureCollections(b FixtureBundle) error {
	for _, count := range []int{
		len(b.Sessions), len(b.AuthCases), len(b.BalanceCases), len(b.FreezeCases),
		len(b.AccountSnapshots), len(b.PlanCases), len(b.PaymentCases), len(b.PaymentCreates), len(b.PaymentTapes),
		len(b.SyncCases), len(b.Events), len(b.RecoveryCases),
	} {
		if count == 0 {
			return fmt.Errorf("cloud: fixture requires every normalized scenario collection")
		}
	}
	return nil
}

func validateA0FixtureCollections(b FixtureBundle, scope BindingScope) error {
	if err := validateA0AccountCollections(b, scope); err != nil {
		return err
	}
	if err := validateA0PaymentCollections(b, scope); err != nil {
		return err
	}
	if err := validateSyncFixtures(b.SyncCases, scope); err != nil {
		return err
	}
	if err := validateFixtureEvents(b.Events, scope); err != nil {
		return err
	}
	return validateRecoveryFixtures(b.RecoveryCases, b.Events, scope)
}

func validateA0AccountCollections(b FixtureBundle, scope BindingScope) error {
	if err := validateFixtureSessions(b.Sessions, scope); err != nil {
		return err
	}
	if err := validateFixtureFreezes(b.FreezeCases, scope); err != nil {
		return err
	}
	if err := validateAuthFixtures(b.AuthCases, scope); err != nil {
		return err
	}
	if err := validateAccountSnapshots(b.AccountSnapshots, b.AuthCases); err != nil {
		return err
	}
	if err := validateBalanceFixtures(b.BalanceCases, scope); err != nil {
		return err
	}
	return nil
}

func validateA0PaymentCollections(b FixtureBundle, scope BindingScope) error {
	if err := validatePlans(b.PlanCases); err != nil {
		return err
	}
	if err := validatePaymentFixtures(b.PaymentCases, scope); err != nil {
		return err
	}
	if err := validatePaymentCreateTapeContract(b.PaymentCreates, b.PaymentTapes, b.AccountSnapshots); err != nil {
		return err
	}
	return nil
}

func (m FixtureMetadata) Validate() error {
	if !m.MockOnly || m.L2Integrated {
		return fmt.Errorf("cloud: fixture must be mock-only and not L2 integrated")
	}
	return validateNamedSet("fixture gates", m.Gates, []string{
		"CLOUD-01", "CLOUD-02", "CLOUD-03", "CLOUD-05",
		"CLOUD-07", "CLOUD-08", "CLOUD-09", "CLOUD-10",
	})
}

func validateFixtureSessions(sessions []Session, scope BindingScope) error {
	for i, session := range sessions {
		if err := session.Validate(); err != nil {
			return fmt.Errorf("cloud: session fixture %d: %w", i, err)
		}
		if session.Scope != scope {
			return fmt.Errorf("cloud: session fixture %d belongs to another binding", i)
		}
	}
	return nil
}

func validateFixtureFreezes(fixtures []FreezeFixture, scope BindingScope) error {
	byName, err := indexFixtureFreezes(fixtures, scope)
	if err != nil {
		return err
	}
	if err := validateNamedSet("freeze fixtures", mapFreezeKeys(byName), []string{
		"freeze_granted", "insufficient_balance", "low_balance", "network_failure", "expired_released", "expired_pending_review", "not_found",
	}); err != nil {
		return err
	}
	if err := validateFreezeFixtureResults(byName); err != nil {
		return err
	}
	return validateFreezeFixtureErrors(byName)
}

func indexFixtureFreezes(fixtures []FreezeFixture, scope BindingScope) (map[string]FreezeFixture, error) {
	byName := make(map[string]FreezeFixture, len(fixtures))
	for i, fixture := range fixtures {
		if err := fixture.Validate(); err != nil {
			return nil, fmt.Errorf("cloud: freeze fixture %d: %w", i, err)
		}
		if fixture.Request.Scope != scope {
			return nil, fmt.Errorf("cloud: freeze fixture %d belongs to another binding", i)
		}
		if _, exists := byName[fixture.Name]; exists {
			return nil, fmt.Errorf("cloud: duplicate freeze fixture %q", fixture.Name)
		}
		byName[fixture.Name] = fixture
	}
	return byName, nil
}

func validateFreezeFixtureResults(byName map[string]FreezeFixture) error {
	if err := validateActiveFreezeFixture(byName["freeze_granted"], "freeze_granted", false); err != nil {
		return err
	}
	if err := validateActiveFreezeFixture(byName["low_balance"], "low_balance", true); err != nil {
		return err
	}
	if err := validateExpiredFreezeFixture(byName["expired_released"], "expired_released", FreezeExpiredReleased); err != nil {
		return err
	}
	return validateExpiredFreezeFixture(byName["expired_pending_review"], "expired_pending_review", FreezeExpiredPendingReview)
}

func validateActiveFreezeFixture(fixture FreezeFixture, name string, lowBalance bool) error {
	if fixture.Result == nil || fixture.Result.Freeze.Status != FreezeActive || fixture.Result.Freeze.LowBalanceMode != lowBalance {
		return fmt.Errorf("cloud: %s fixture is invalid", name)
	}
	return nil
}

func validateExpiredFreezeFixture(fixture FreezeFixture, name string, status FreezeStatus) error {
	if fixture.Result == nil || fixture.Result.Freeze.Status != status {
		return fmt.Errorf("cloud: %s fixture is invalid", name)
	}
	return nil
}

func validateFreezeFixtureErrors(byName map[string]FreezeFixture) error {
	for name, code := range map[string]ErrorCode{
		"insufficient_balance": ErrorInsufficientBalance,
		"network_failure":      ErrorNetworkUnavailable,
		"not_found":            ErrorFreezeNotFound,
	} {
		if byName[name].Error == nil || byName[name].Error.Code != code {
			return fmt.Errorf("cloud: %s fixture is invalid", name)
		}
	}
	return nil
}

func validateSyncFixtures(fixtures []ThirdPartySync, scope BindingScope) error {
	statuses := make([]string, 0, len(fixtures))
	for i, fixture := range fixtures {
		if err := fixture.Validate(); err != nil {
			return fmt.Errorf("cloud: third-party sync fixture %d: %w", i, err)
		}
		if fixture.Scope != scope {
			return fmt.Errorf("cloud: third-party sync fixture %d belongs to another binding", i)
		}
		statuses = append(statuses, string(fixture.Status))
	}
	return validateNamedSet("third-party sync fixtures", statuses, []string{
		"pending", "running", "succeeded", "failed_retryable", "failed_manual_required", "skipped",
	})
}

func validateS0FixtureFreezes(fixtures []FreezeFixture, scope BindingScope) error {
	for i, fixture := range fixtures {
		if err := fixture.Validate(); err != nil {
			return fmt.Errorf("cloud: freeze fixture %d: %w", i, err)
		}
		if fixture.Request.Scope != scope {
			return fmt.Errorf("cloud: freeze fixture %d belongs to another binding", i)
		}
	}
	return nil
}

func validateS0FixtureEvents(events []Event) error {
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("cloud: event fixture %d: %w", i, err)
		}
	}
	return nil
}

func validateFixtureEvents(events []Event, scope BindingScope) error {
	types, eventIDs, otherUser, outOfOrder, err := inspectFixtureEvents(events, scope)
	if err != nil {
		return err
	}
	if err := validateNamedSet("event fixtures", mapKeys(types), []string{
		"balance.updated", "freeze.created", "freeze.expired", "settlement.completed", "settlement.exception", "subscription.updated", "payment.updated", "third_party_sync.updated",
	}); err != nil {
		return err
	}
	return validateFixtureEventSamples(eventIDs, otherUser, outOfOrder)
}

func inspectFixtureEvents(events []Event, scope BindingScope) (map[string]bool, map[string]int, bool, bool, error) {
	types := make(map[string]bool, len(events))
	eventIDs := make(map[string]int, len(events))
	otherUser := false
	outOfOrder := false
	var previous Event
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, nil, false, false, fmt.Errorf("cloud: event fixture %d: %w", i, err)
		}
		types[string(event.EventType)] = true
		eventIDs[event.EventID]++
		otherUser = otherUser || event.CloudUserID != scope.CloudUserID
		outOfOrder = outOfOrder || (i > 0 && event.OccurredAt.Before(previous.OccurredAt))
		previous = event
	}
	return types, eventIDs, otherUser, outOfOrder, nil
}

func validateFixtureEventSamples(eventIDs map[string]int, otherUser, outOfOrder bool) error {
	if !hasDuplicateFixtureEventID(eventIDs) || !otherUser || !outOfOrder {
		return fmt.Errorf("cloud: fixture events require duplicate, other-user, and out-of-order samples")
	}
	return nil
}

func hasDuplicateFixtureEventID(eventIDs map[string]int) bool {
	for _, count := range eventIDs {
		if count > 1 {
			return true
		}
	}
	return false
}

func validateRecoveryFixtures(fixtures []RecoveryFixture, events []Event, scope BindingScope) error {
	eventIDs := recoveryFixtureEventIDs(events)
	names := make([]string, 0, len(fixtures))
	lastEventIDs := make(map[string]bool, len(fixtures))
	for i, fixture := range fixtures {
		if err := validateRecoveryFixture(fixture, i, scope, eventIDs, lastEventIDs); err != nil {
			return err
		}
		names = append(names, fixture.Name)
	}
	return validateNamedSet("recovery fixtures", names, []string{"disconnect_then_rest_restore"})
}

func recoveryFixtureEventIDs(events []Event) map[string]bool {
	eventIDs := make(map[string]bool, len(events))
	for _, event := range events {
		eventIDs[event.EventID] = true
	}
	return eventIDs
}

func validateRecoveryFixture(fixture RecoveryFixture, index int, scope BindingScope, eventIDs, lastEventIDs map[string]bool) error {
	if err := requireTrimmed("recovery fixture name", fixture.Name); err != nil {
		return fmt.Errorf("cloud: recovery fixture %d: %w", index, err)
	}
	if fixture.Scope != scope || !eventIDs[fixture.DisconnectedAfterEventID] || !eventIDs[fixture.LastEventID] {
		return fmt.Errorf("cloud: recovery fixture %d does not reference the active binding and event stream", index)
	}
	if lastEventIDs[fixture.LastEventID] {
		return fmt.Errorf("cloud: duplicate recovery last_event_id %q", fixture.LastEventID)
	}
	lastEventIDs[fixture.LastEventID] = true
	return validateRecoveryFixtureSnapshots(fixture, index, scope)
}

func validateRecoveryFixtureSnapshots(fixture RecoveryFixture, index int, scope BindingScope) error {
	if err := fixture.Balance.Validate(); err != nil {
		return fmt.Errorf("cloud: recovery fixture %d: %w", index, err)
	}
	if err := fixture.Subscription.Validate(); err != nil || fixture.Subscription.Scope != scope {
		return fmt.Errorf("cloud: recovery fixture %d has an invalid subscription", index)
	}
	if len(fixture.Freezes) == 0 || len(fixture.PaymentOrders) == 0 {
		return fmt.Errorf("cloud: recovery fixture %d requires freeze and payment snapshots", index)
	}
	if err := validateRecoveryFreezeSnapshots(fixture.Freezes, index, scope); err != nil {
		return err
	}
	return validateRecoveryPaymentSnapshots(fixture.PaymentOrders, index, scope)
}

func validateRecoveryFreezeSnapshots(freezes []FreezeResult, index int, scope BindingScope) error {
	for _, freeze := range freezes {
		if err := freeze.Validate(); err != nil || freeze.Freeze.Scope != scope {
			return fmt.Errorf("cloud: recovery fixture %d has an invalid freeze snapshot", index)
		}
	}
	return nil
}

func validateRecoveryPaymentSnapshots(orders []PaymentOrder, index int, scope BindingScope) error {
	for _, order := range orders {
		if err := order.Validate(); err != nil || order.Scope != scope {
			return fmt.Errorf("cloud: recovery fixture %d has an invalid payment snapshot", index)
		}
	}
	return nil
}

func validateSessionState(state SessionState) error {
	switch state {
	case SessionAuthenticated, SessionExpired, SessionOffline:
		return nil
	default:
		return fmt.Errorf("cloud: invalid session state %q", state)
	}
}

func validateFixtureError(err *Error) error {
	if err == nil {
		return fmt.Errorf("cloud: fixture error is required")
	}
	if err.Code == "" {
		return fmt.Errorf("cloud: fixture error code is required")
	}
	if err := requireTrimmed("fixture error message", err.Message); err != nil {
		return err
	}
	if len(err.Details) > 0 && !json.Valid(err.Details) {
		return fmt.Errorf("cloud: fixture error details must be valid JSON")
	}
	return nil
}

func validateNamedSet(label string, actual, expected []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("cloud: %s count = %d, want %d", label, len(actual), len(expected))
	}
	actualSet := make(map[string]bool, len(actual))
	for _, name := range actual {
		if actualSet[name] {
			return fmt.Errorf("cloud: duplicate %s entry %q", label, name)
		}
		actualSet[name] = true
	}
	want := make(map[string]bool, len(expected))
	for _, name := range expected {
		want[name] = true
	}
	for _, name := range actual {
		if !want[name] {
			return fmt.Errorf("cloud: unexpected %s entry %q", label, name)
		}
	}
	for _, name := range expected {
		if !actualSet[name] {
			return fmt.Errorf("cloud: missing %s entry %q", label, name)
		}
	}
	return nil
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func mapFreezeKeys(values map[string]FreezeFixture) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

type FixtureEnvironment struct {
	BindingResolver *StaticBindingResolver
	SessionStore    *MemorySessionStore
	CreditClient    *FixtureCreditClient
	EventSource     *FixtureEventSource
}

func NewFixtureEnvironment(bundle FixtureBundle) (*FixtureEnvironment, error) {
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	resolver, err := NewStaticBindingResolver(bundle.Binding)
	if err != nil {
		return nil, err
	}
	store := NewMemorySessionStore()
	for _, session := range bundle.Sessions {
		if err := store.Save(context.Background(), session); err != nil {
			return nil, err
		}
	}
	credit, err := NewFixtureCreditClient(bundle.FreezeCases)
	if err != nil {
		return nil, err
	}
	events, err := NewFixtureEventSource(bundle.Events)
	if err != nil {
		return nil, err
	}
	return &FixtureEnvironment{
		BindingResolver: resolver,
		SessionStore:    store,
		CreditClient:    credit,
		EventSource:     events,
	}, nil
}
