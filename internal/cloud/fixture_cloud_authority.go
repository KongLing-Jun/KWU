package cloud

import (
	"context"
	"fmt"
)

// FixtureCloudAuthority is the single canonical A0 fact source shared by the
// account and payment fake adapters. Payment progression is derived from the
// injected order repository; the authority keeps no cursor or call counter.
type FixtureCloudAuthority struct {
	plans          []SubscriptionPlan
	accounts       map[string]AccountSnapshotFixture
	creates        map[fixturePaymentMatrixKey]PaymentCreateFixture
	tapes          map[string]PaymentTapeFixture
	orders         PaymentOrderRepository
	fixtureVersion string
}

type fixturePaymentMatrixKey struct {
	Operation PaymentOrderType
	FromPlan  string
	Plan      string
	Channel   PaymentChannel
}

func NewFixtureCloudAuthority(bundle FixtureBundle, orders PaymentOrderRepository) (*FixtureCloudAuthority, error) {
	if bundle.Version != A0FixtureVersion {
		return nil, fmt.Errorf("cloud: fixture authority requires %s", A0FixtureVersion)
	}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	if isNilDependency(orders) {
		return nil, fmt.Errorf("cloud: fixture authority requires a payment order repository")
	}
	authority := &FixtureCloudAuthority{
		plans:          append([]SubscriptionPlan(nil), bundle.PlanCases...),
		accounts:       make(map[string]AccountSnapshotFixture, len(bundle.AccountSnapshots)),
		creates:        make(map[fixturePaymentMatrixKey]PaymentCreateFixture, len(bundle.PaymentCreates)),
		tapes:          make(map[string]PaymentTapeFixture, len(bundle.PaymentTapes)),
		orders:         orders,
		fixtureVersion: bundle.Version,
	}
	for _, snapshot := range bundle.AccountSnapshots {
		if _, exists := authority.accounts[snapshot.Scope.CloudUserID]; exists {
			return nil, fmt.Errorf("cloud: duplicate canonical account authority identity")
		}
		authority.accounts[snapshot.Scope.CloudUserID] = snapshot
	}
	for _, create := range bundle.PaymentCreates {
		order := create.Result.Order
		key := fixturePaymentMatrixKey{
			Operation: order.OrderType,
			FromPlan:  order.FromPlan,
			Plan:      order.Plan,
			Channel:   order.PaymentChannel,
		}
		if _, exists := authority.creates[key]; exists {
			return nil, fmt.Errorf("cloud: duplicate canonical payment create dimensions")
		}
		authority.creates[key] = create
	}
	for _, tape := range bundle.PaymentTapes {
		if _, exists := authority.tapes[tape.OrderID]; exists {
			return nil, fmt.Errorf("cloud: duplicate canonical payment tape order")
		}
		authority.tapes[tape.OrderID] = tape
	}
	return authority, nil
}

func (a *FixtureCloudAuthority) canonicalAccount(scope BindingScope) (AccountSnapshotFixture, error) {
	if a == nil || a.fixtureVersion != A0FixtureVersion {
		return AccountSnapshotFixture{}, fmt.Errorf("cloud: fixture authority is not configured")
	}
	if err := scope.Validate(); err != nil {
		return AccountSnapshotFixture{}, err
	}
	snapshot, ok := a.accounts[scope.CloudUserID]
	if !ok {
		return AccountSnapshotFixture{}, paymentScopeMismatch("cloud: fixture account belongs to another binding")
	}
	return snapshot, nil
}

func (a *FixtureCloudAuthority) currentAccount(ctx context.Context, scope BindingScope) (AccountSnapshotFixture, error) {
	snapshot, err := a.canonicalAccount(scope)
	if err != nil {
		return AccountSnapshotFixture{}, err
	}
	records, err := a.orders.List(ctx, scope)
	if err != nil {
		return AccountSnapshotFixture{}, err
	}
	snapshot, err = a.latestGrantedAccountSnapshot(scope, snapshot, records)
	if err != nil {
		return AccountSnapshotFixture{}, err
	}
	snapshot.Scope = scope
	snapshot.Subscription.Scope = scope
	return snapshot, nil
}

func (a *FixtureCloudAuthority) latestGrantedAccountSnapshot(scope BindingScope, snapshot AccountSnapshotFixture, records []PaymentOrderRecord) (AccountSnapshotFixture, error) {
	var latestGrantSequence uint64
	seenGrantSequences := make(map[uint64]struct{})
	for _, record := range records {
		if !isGrantedPaymentRecord(record) {
			continue
		}
		if err := validateGrantedPaymentSequence(record, seenGrantSequences); err != nil {
			return AccountSnapshotFixture{}, err
		}
		tape, err := a.canonicalGrantedPaymentTape(scope, record)
		if err != nil {
			return AccountSnapshotFixture{}, err
		}
		candidate, err := currentUserGrantedSnapshot(tape.GrantedAccountSnapshots, scope)
		if err != nil {
			return AccountSnapshotFixture{}, err
		}
		if record.GrantSequence > latestGrantSequence {
			snapshot = candidate
			latestGrantSequence = record.GrantSequence
		}
	}
	return snapshot, nil
}

func isGrantedPaymentRecord(record PaymentOrderRecord) bool {
	return record.Result.Order.Status == PaymentGranted && record.Result.Order.GrantStatus == GrantGranted
}

func validateGrantedPaymentSequence(record PaymentOrderRecord, seen map[uint64]struct{}) error {
	if record.GrantSequence == 0 {
		return paymentScopeMismatch("cloud: granted payment record has no durable grant sequence")
	}
	if _, duplicate := seen[record.GrantSequence]; duplicate {
		return paymentScopeMismatch("cloud: granted payment records have duplicate durable grant sequences")
	}
	seen[record.GrantSequence] = struct{}{}
	return nil
}

func (a *FixtureCloudAuthority) canonicalGrantedPaymentTape(scope BindingScope, record PaymentOrderRecord) (PaymentTapeFixture, error) {
	order := record.Result.Order
	tape, ok := a.tapes[order.OrderID]
	if !ok || len(tape.Results) == 0 {
		return PaymentTapeFixture{}, paymentScopeMismatch("cloud: granted payment record has no canonical account snapshot")
	}
	projected, err := projectFixturePaymentResult(tape.Results[len(tape.Results)-1], scope)
	if err != nil {
		return PaymentTapeFixture{}, err
	}
	if !samePaymentOrderIdentity(order, projected.Order) || order.Status != projected.Order.Status || order.GrantStatus != projected.Order.GrantStatus {
		return PaymentTapeFixture{}, paymentScopeMismatch("cloud: granted payment record drifted from the canonical tape")
	}
	return tape, nil
}

func currentUserGrantedSnapshot(snapshots []AccountSnapshotFixture, scope BindingScope) (AccountSnapshotFixture, error) {
	for _, canonical := range snapshots {
		if canonical.Scope.CloudUserID == scope.CloudUserID {
			canonical.Scope = scope
			canonical.Subscription.Scope = scope
			return canonical, nil
		}
	}
	return AccountSnapshotFixture{}, paymentScopeMismatch("cloud: granted payment record has no snapshot for the current cloud user")
}

func (a *FixtureCloudAuthority) savedOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrderRecord, error) {
	record, found, err := a.orders.Get(ctx, scope, orderID)
	if err != nil {
		return PaymentOrderRecord{}, err
	}
	if !found || record.Scope != scope || record.Result.Order.OrderID != orderID {
		return PaymentOrderRecord{}, paymentScopeMismatch("cloud: fixture payment order is not saved for the current binding")
	}
	return record, nil
}
