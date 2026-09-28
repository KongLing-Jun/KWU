package cloud

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"kwu/internal/userctx"
)

// PaymentEntitlementRefresher refreshes cloud-authoritative entitlement
// snapshots after the cloud has confirmed a granted order. It deliberately
// does not expose a local balance or subscription mutation path.
type PaymentEntitlementRefresher interface {
	RefreshPaymentEntitlements(context.Context, userctx.ActorContext) error
}

type PaymentApplicationDependencies struct {
	Bindings     BindingResolver
	Payments     CloudPaymentClient
	Orders       PaymentOrderRepository
	Entitlements PaymentEntitlementRefresher
}

// PaymentView contains the cloud order result suitable for display. A stale
// payment entry is redacted rather than reused or refreshed through an
// unconfirmed API path.
type PaymentView struct {
	Result             PaymentOrderResult
	State              PaymentUIState
	Payable            bool
	EntitlementGranted bool
}

// PaymentApplication owns creation and local-ID recovery of payment orders.
// CLOUD-08 leaves entry refresh unimplemented and CLOUD-09 limits recovery to
// records previously saved through the repository.
type PaymentApplication struct {
	bindings     BindingResolver
	payments     CloudPaymentClient
	orders       PaymentOrderRepository
	entitlements PaymentEntitlementRefresher
	now          func() time.Time
	newAttemptID func() (string, error)

	mu sync.Mutex
}

type paymentOrderResultReader interface {
	getPaymentOrderResult(context.Context, BindingScope, string) (PaymentOrderResult, error)
}

type paymentOrderRecordResultReader interface {
	getPaymentOrderResultFor(context.Context, BindingScope, PaymentOrder) (PaymentOrderResult, error)
}

func NewPaymentApplication(deps PaymentApplicationDependencies) (*PaymentApplication, error) {
	if isNilDependency(deps.Bindings) || isNilDependency(deps.Payments) || isNilDependency(deps.Orders) {
		return nil, fmt.Errorf("cloud: payment application is not configured")
	}
	return &PaymentApplication{
		bindings:     deps.Bindings,
		payments:     deps.Payments,
		orders:       deps.Orders,
		entitlements: deps.Entitlements,
		now:          time.Now,
		newAttemptID: newPaymentAttemptID,
	}, nil
}

func (a *PaymentApplication) CreateSubscriptionOrder(ctx context.Context, actor userctx.ActorContext, planID string, channel PaymentChannel) (PaymentView, error) {
	if err := requireTrimmed("plan_id", planID); err != nil {
		return PaymentView{}, err
	}
	if err := channel.Validate(); err != nil {
		return PaymentView{}, err
	}
	return a.createOrder(ctx, actor, PaymentOrderSubscriptionPurchase, planID, channel)
}

func (a *PaymentApplication) CreateUpgradeOrder(ctx context.Context, actor userctx.ActorContext, targetPlan string, channel PaymentChannel) (PaymentView, error) {
	if err := requireTrimmed("target_plan", targetPlan); err != nil {
		return PaymentView{}, err
	}
	if err := channel.Validate(); err != nil {
		return PaymentView{}, err
	}
	return a.createOrder(ctx, actor, PaymentOrderSubscriptionUpgrade, targetPlan, channel)
}

func (a *PaymentApplication) createOrder(ctx context.Context, actor userctx.ActorContext, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentView, error) {
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return PaymentView{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for {
		attempt, view, complete, err := a.reusablePaymentAttempt(ctx, actor, scope, operation, plan, channel)
		if err != nil {
			return PaymentView{}, err
		}
		if complete {
			return view, nil
		}
		if attempt.AttemptID != "" && attempt.OrderID == "" {
			return a.createPaymentOrderForAttempt(ctx, actor, attempt)
		}
		candidate, err := a.newPaymentAttempt(scope, operation, plan, channel)
		if err != nil {
			return PaymentView{}, err
		}
		reserved, created, err := a.orders.ReserveAttempt(ctx, attempt.AttemptID, candidate)
		if err != nil {
			return PaymentView{}, err
		}
		if created {
			return a.createPaymentOrderForAttempt(ctx, actor, reserved)
		}
	}
}

func (a *PaymentApplication) newPaymentAttempt(scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentAttempt, error) {
	attemptID, err := a.newAttemptID()
	if err != nil {
		return PaymentAttempt{}, err
	}
	idempotencyKey, err := paymentOrderIdempotencyKey(operation, scope, plan, channel, attemptID)
	if err != nil {
		return PaymentAttempt{}, err
	}
	attempt := PaymentAttempt{
		Scope: scope, AttemptID: attemptID, Operation: operation, Plan: plan,
		PaymentChannel: channel, IdempotencyKey: idempotencyKey, CreatedAt: a.now().UTC(),
	}
	return attempt, attempt.Validate()
}

func (a *PaymentApplication) reusablePaymentAttempt(ctx context.Context, actor userctx.ActorContext, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentAttempt, PaymentView, bool, error) {
	attempt, found, err := a.orders.LatestAttempt(ctx, scope, operation, plan, channel)
	if err != nil {
		return PaymentAttempt{}, PaymentView{}, false, err
	}
	if !found {
		return a.reusableLegacyPaymentOrder(ctx, actor, scope, operation, plan, channel)
	}
	if attempt.OrderID == "" {
		return attempt, PaymentView{}, false, err
	}
	record, err := a.savedPaymentOrder(ctx, scope, attempt.OrderID)
	if err != nil {
		return PaymentAttempt{}, PaymentView{}, false, err
	}
	if err := validatePaymentAttemptOrder(attempt, record); err != nil {
		return PaymentAttempt{}, PaymentView{}, false, err
	}
	if paymentAttemptNeedsAuthoritativeRefresh(record, a.now()) {
		record, _, err = a.refreshSavedPaymentOrder(ctx, scope, record)
		if err != nil {
			return PaymentAttempt{}, PaymentView{}, false, err
		}
	}
	if paymentAttemptCanRestart(record, a.now()) {
		return attempt, PaymentView{}, false, nil
	}
	view, err := a.completeView(ctx, actor, record)
	return attempt, view, true, err
}

func (a *PaymentApplication) reusableLegacyPaymentOrder(ctx context.Context, actor userctx.ActorContext, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentAttempt, PaymentView, bool, error) {
	idempotencyKey, err := legacyPaymentOrderIdempotencyKey(operation, scope, plan, channel)
	if err != nil {
		return PaymentAttempt{}, PaymentView{}, false, err
	}
	record, found, err := a.orders.FindByIdempotencyKey(ctx, scope, idempotencyKey)
	if err != nil || !found {
		return PaymentAttempt{}, PaymentView{}, false, err
	}
	legacy := PaymentAttempt{
		Scope: scope, Operation: operation, Plan: plan, PaymentChannel: channel,
		IdempotencyKey: idempotencyKey, OrderID: record.Result.Order.OrderID,
	}
	if err := validatePaymentAttemptOrder(legacy, record); err != nil {
		return PaymentAttempt{}, PaymentView{}, false, err
	}
	if paymentAttemptNeedsAuthoritativeRefresh(record, a.now()) {
		record, _, err = a.refreshSavedPaymentOrder(ctx, scope, record)
		if err != nil {
			return PaymentAttempt{}, PaymentView{}, false, err
		}
	}
	if paymentAttemptCanRestart(record, a.now()) {
		return PaymentAttempt{}, PaymentView{}, false, nil
	}
	view, err := a.completeView(ctx, actor, record)
	return legacy, view, true, err
}

func (a *PaymentApplication) createPaymentOrderForAttempt(ctx context.Context, actor userctx.ActorContext, attempt PaymentAttempt) (PaymentView, error) {
	existing, found, err := a.orders.FindByIdempotencyKey(ctx, attempt.Scope, attempt.IdempotencyKey)
	if err != nil {
		return PaymentView{}, err
	}
	if found {
		if err := validatePaymentAttemptOrder(attempt, existing); err != nil {
			return PaymentView{}, err
		}
		if err := a.orders.BindAttempt(ctx, attempt.Scope, attempt.AttemptID, existing.Result.Order.OrderID); err != nil {
			return PaymentView{}, err
		}
		return a.completeView(ctx, actor, existing)
	}
	result, err := a.requestPaymentOrder(ctx, attempt.Scope, attempt.Operation, attempt.Plan, attempt.PaymentChannel, attempt.IdempotencyKey)
	if err != nil {
		return PaymentView{}, err
	}
	if err := validatePaymentResult(attempt.Scope, attempt.Operation, attempt.Plan, attempt.PaymentChannel, result); err != nil {
		return PaymentView{}, err
	}
	record, err := NewPaymentOrderRecord(attempt.Scope, result, attempt.Operation, attempt.IdempotencyKey)
	if err != nil {
		return PaymentView{}, err
	}
	if err := a.orders.Save(ctx, record); err != nil {
		return PaymentView{}, err
	}
	if err := a.orders.BindAttempt(ctx, attempt.Scope, attempt.AttemptID, result.Order.OrderID); err != nil {
		return PaymentView{}, err
	}
	return a.completeView(ctx, actor, record)
}

func paymentOrderIdempotencyKey(operation PaymentOrderType, scope BindingScope, plan string, channel PaymentChannel, attemptID string) (string, error) {
	action := IdempotencyActionCreateSubscriptionOrder
	if operation == PaymentOrderSubscriptionUpgrade {
		action = IdempotencyActionCreateUpgradeOrder
	}
	return BuildIdempotencyKey(action, scope, plan+"\x00"+string(channel)+"\x00"+attemptID)
}

func legacyPaymentOrderIdempotencyKey(operation PaymentOrderType, scope BindingScope, plan string, channel PaymentChannel) (string, error) {
	action := IdempotencyActionCreateSubscriptionOrder
	if operation == PaymentOrderSubscriptionUpgrade {
		action = IdempotencyActionCreateUpgradeOrder
	}
	return BuildIdempotencyKey(action, scope, plan+"\x00"+string(channel))
}

func newPaymentAttemptID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("cloud: generate payment attempt ID: %w", err)
	}
	return "attempt-" + hex.EncodeToString(value[:]), nil
}

func paymentAttemptCanRestart(record PaymentOrderRecord, now time.Time) bool {
	switch record.Result.Order.Status {
	case PaymentClosed, PaymentTimeout, PaymentFailed:
		return true
	case PaymentCreated, PaymentPending:
		return record.Result.Entry != (PaymentEntry{}) && !now.Before(record.Result.Entry.ExpiresAt)
	default:
		return false
	}
}

func paymentAttemptNeedsAuthoritativeRefresh(record PaymentOrderRecord, now time.Time) bool {
	switch record.Result.Order.Status {
	case PaymentCreated:
		return record.Result.Entry == (PaymentEntry{}) || !now.Before(record.Result.Entry.ExpiresAt)
	case PaymentPending:
		return !now.Before(record.Result.Entry.ExpiresAt)
	default:
		return false
	}
}

func (a *PaymentApplication) requestPaymentOrder(ctx context.Context, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel, idempotencyKey string) (PaymentOrderResult, error) {
	if operation == PaymentOrderSubscriptionPurchase {
		request := CreateSubscriptionOrderRequest{Scope: scope, PlanID: plan, PaymentChannel: channel, IdempotencyKey: idempotencyKey}
		if err := request.Validate(); err != nil {
			return PaymentOrderResult{}, err
		}
		return a.payments.CreateSubscriptionOrder(ctx, request)
	}
	request := CreateUpgradeOrderRequest{Scope: scope, TargetPlan: plan, PaymentChannel: channel, IdempotencyKey: idempotencyKey}
	if err := request.Validate(); err != nil {
		return PaymentOrderResult{}, err
	}
	return a.payments.CreateUpgradeOrder(ctx, request)
}

// RefreshOrder queries the authority only for an order identifier already
// stored under the actor's complete scope. It intentionally does not provide a
// remote pending/list discovery path (CLOUD-09).
func (a *PaymentApplication) RefreshOrder(ctx context.Context, actor userctx.ActorContext, orderID string) (PaymentView, error) {
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return PaymentView{}, err
	}
	if err := requireTrimmed("order_id", orderID); err != nil {
		return PaymentView{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	record, err := a.savedPaymentOrder(ctx, scope, orderID)
	if err != nil {
		return PaymentView{}, err
	}
	updated, authoritative, err := a.refreshSavedPaymentOrder(ctx, scope, record)
	if err != nil {
		if authoritative {
			return PaymentView{}, err
		}
		view, viewErr := a.view(record.Result)
		if viewErr != nil {
			return PaymentView{}, viewErr
		}
		return view, err
	}
	return a.completeView(ctx, actor, updated)
}

func (a *PaymentApplication) refreshSavedPaymentOrder(ctx context.Context, scope BindingScope, record PaymentOrderRecord) (PaymentOrderRecord, bool, error) {
	result, err := a.getPaymentOrderResult(ctx, scope, record)
	if err != nil {
		return record, false, err
	}
	if err := validateRefreshedPaymentResult(scope, record, result); err != nil {
		return PaymentOrderRecord{}, true, err
	}
	updated := record
	updated.Result = result
	if err := updated.Validate(); err != nil {
		return PaymentOrderRecord{}, true, err
	}
	if err := a.orders.Save(ctx, updated); err != nil {
		return PaymentOrderRecord{}, true, err
	}
	return updated, true, nil
}

func (a *PaymentApplication) getPaymentOrderResult(ctx context.Context, scope BindingScope, record PaymentOrderRecord) (PaymentOrderResult, error) {
	if reader, ok := a.payments.(paymentOrderRecordResultReader); ok {
		return reader.getPaymentOrderResultFor(ctx, scope, record.Result.Order)
	}
	if reader, ok := a.payments.(paymentOrderResultReader); ok {
		return reader.getPaymentOrderResult(ctx, scope, record.Result.Order.OrderID)
	}
	order, err := a.payments.GetPaymentOrder(ctx, scope, record.Result.Order.OrderID)
	if err != nil {
		return PaymentOrderResult{}, err
	}
	result := record.Result
	result.Order = order
	return result, nil
}

func (a *PaymentApplication) savedPaymentOrder(ctx context.Context, scope BindingScope, orderID string) (PaymentOrderRecord, error) {
	record, found, err := a.orders.Get(ctx, scope, orderID)
	if err != nil {
		return PaymentOrderRecord{}, err
	}
	if !found || record.Scope != scope || record.Result.Order.OrderID != orderID {
		return PaymentOrderRecord{}, paymentScopeMismatch("cloud: payment order is not saved for the current binding")
	}
	return record, nil
}

// ClientPaidUnverified handles only a local transient signal. It never stores
// that signal nor treats verification as entitlement grant.
func (a *PaymentApplication) ClientPaidUnverified(ctx context.Context, actor userctx.ActorContext, orderID string) (PaymentView, error) {
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return PaymentView{}, err
	}
	if err := requireTrimmed("order_id", orderID); err != nil {
		return PaymentView{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	record, err := a.savedPaymentOrder(ctx, scope, orderID)
	if err != nil {
		return PaymentView{}, err
	}
	order := record.Result.Order
	if order.Status == PaymentGranted && order.GrantStatus == GrantGranted {
		return PaymentView{}, &Error{Code: ErrorOrderAlreadyGranted, Message: "cloud: payment order entitlements are already granted"}
	}
	if order.Status != PaymentPending || !a.now().Before(record.Result.Entry.ExpiresAt) {
		return PaymentView{}, &Error{Code: ErrorPaymentNotVerified, Message: "cloud: payment order is not payable"}
	}
	_, verifyErr := a.payments.VerifyPaymentOrder(ctx, scope, orderID)
	view, viewErr := a.view(record.Result)
	if viewErr != nil {
		return PaymentView{}, viewErr
	}
	view.State = PaymentUIStateConfirming
	view.Payable = false
	view.EntitlementGranted = false
	return view, verifyErr
}

// RefreshPaymentEntry remains disabled until CLOUD-08 freezes a replacement
// entry contract. In particular, it never reuses an expired entry.
func (a *PaymentApplication) RefreshPaymentEntry(context.Context, userctx.ActorContext, string) (PaymentView, error) {
	return PaymentView{}, fmt.Errorf("cloud: payment entry refresh is unavailable pending CLOUD-08")
}

func (a *PaymentApplication) completeView(ctx context.Context, actor userctx.ActorContext, record PaymentOrderRecord) (PaymentView, error) {
	view, err := a.view(record.Result)
	if err != nil {
		return PaymentView{}, err
	}
	if !view.EntitlementGranted || isNilDependency(a.entitlements) {
		return view, nil
	}
	if record.EntitlementsRefreshed {
		return view, nil
	}
	if err := a.entitlements.RefreshPaymentEntitlements(ctx, actor.Normalize()); err != nil {
		return view, err
	}
	if err := a.orders.MarkEntitlementsRefreshed(ctx, record.Scope, record.Result.Order.OrderID); err != nil {
		return view, err
	}
	return view, nil
}

func (a *PaymentApplication) view(result PaymentOrderResult) (PaymentView, error) {
	if err := result.Validate(); err != nil {
		return PaymentView{}, err
	}
	state, err := MapPaymentStatus(result.Order.Status)
	if err != nil {
		return PaymentView{}, err
	}
	view := PaymentView{
		Result:             result,
		State:              state,
		EntitlementGranted: result.Order.Status == PaymentGranted && result.Order.GrantStatus == GrantGranted,
	}
	if result.Order.Status == PaymentPending && a.now().Before(result.Entry.ExpiresAt) {
		view.Payable = true
		return view, nil
	}
	view.Result.Entry.QRCode = ""
	view.Result.Entry.CodeURL = ""
	view.Result.Entry.QRCodeURL = ""
	return view, nil
}

func (a *PaymentApplication) currentScope(ctx context.Context, actor userctx.ActorContext) (BindingScope, error) {
	actor = actor.Normalize()
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if err := binding.Validate(); err != nil {
		return BindingScope{}, err
	}
	if binding.Status != BindingBound || binding.ActorID != actor.ActorID || binding.ProfileID != actor.ProfileID {
		return BindingScope{}, paymentScopeMismatch("cloud: payment requires the actor's bound profile")
	}
	return binding.Scope()
}

func validatePaymentResult(scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel, result PaymentOrderResult) error {
	if err := result.Validate(); err != nil {
		return err
	}
	identityMatches := result.Order.Plan == plan
	if operation == PaymentOrderSubscriptionPurchase {
		identityMatches = result.Order.PlanID == plan || result.Order.PlanID == "" && result.Order.Plan == plan
	}
	if result.Order.Scope != scope || result.Order.OrderType != operation || !identityMatches || result.Order.PaymentChannel != channel || !paymentEntryMatchesChannel(result.Entry, channel) {
		return paymentScopeMismatch("cloud: payment result belongs to another binding or operation")
	}
	return nil
}

func paymentEntryMatchesChannel(entry PaymentEntry, channel PaymentChannel) bool {
	return entry == (PaymentEntry{}) || entry.Channel == channel
}

func validateRefreshedPaymentOrder(scope BindingScope, record PaymentOrderRecord, order PaymentOrder) error {
	if err := order.Validate(); err != nil {
		return err
	}
	if order.Scope != scope || !samePaymentOrderIdentity(record.Result.Order, order) {
		return paymentScopeMismatch("cloud: payment order refresh belongs to another binding or order")
	}
	if record.Result.Order.Version > 0 && order.Version < record.Result.Order.Version {
		return protocolError(fmt.Errorf("cloud: payment order version regressed"))
	}
	if order.Version > 0 && order.Version == record.Result.Order.Version && order.Status != record.Result.Order.Status {
		return protocolError(fmt.Errorf("cloud: payment order changed without a version increase"))
	}
	return validatePaymentStatusTransition(record.Result.Order.Status, order.Status)
}

func validateRefreshedPaymentResult(scope BindingScope, record PaymentOrderRecord, result PaymentOrderResult) error {
	if err := result.Validate(); err != nil {
		return err
	}
	if err := validateRefreshedPaymentOrder(scope, record, result.Order); err != nil {
		return err
	}
	if !paymentEntryMatchesChannel(result.Entry, result.Order.PaymentChannel) {
		return paymentScopeMismatch("cloud: payment entry belongs to another channel")
	}
	return nil
}

func paymentScopeMismatch(message string) error {
	return &Error{Code: ErrorProfileScopeMismatch, Message: message}
}

// PaymentRecoveryResult keeps each saved local order's restored view and any
// refresh error. A per-order error leaves the cached view available to callers.
type PaymentRecoveryResult struct {
	OrderID string
	View    PaymentView
	Err     error
}

// PaymentRecoveryApplication restores only locally saved payment order IDs for
// the actor's current complete binding scope (CLOUD-09).
type PaymentRecoveryApplication struct {
	payments *PaymentApplication
}

func NewPaymentRecoveryApplication(payments *PaymentApplication) (*PaymentRecoveryApplication, error) {
	if payments == nil {
		return nil, fmt.Errorf("cloud: payment recovery application is not configured")
	}
	return &PaymentRecoveryApplication{payments: payments}, nil
}

// Restore has no remote discovery or progression path. It obtains the current
// scope and reconstructs display views only from locally saved records; an
// explicit confirm/refresh action is required to progress an order. The sole
// side effect is retrying a granted record's unfinished entitlement refresh.
func (a *PaymentRecoveryApplication) Restore(ctx context.Context, actor userctx.ActorContext) ([]PaymentRecoveryResult, error) {
	scope, err := a.payments.currentScope(ctx, actor)
	if err != nil {
		return nil, err
	}
	records, err := a.payments.orders.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	results := make([]PaymentRecoveryResult, 0, len(records))
	for _, record := range records {
		if record.Scope != scope {
			return nil, paymentScopeMismatch("cloud: payment recovery record belongs to another binding")
		}
		view, viewErr := a.payments.completeView(ctx, actor, record)
		results = append(results, PaymentRecoveryResult{
			OrderID: record.Result.Order.OrderID,
			View:    view,
			Err:     viewErr,
		})
	}
	return results, nil
}

// BalanceView exposes the complete cloud-authoritative balance snapshot. The
// desktop displays these server-provided values directly and never derives
// available or frozen amounts locally.
type BalanceView = BalanceSnapshot

type BalanceApplicationDependencies struct {
	Bindings  BindingResolver
	Client    CloudAccountClient
	Snapshots AssetSnapshotRepository
}

// BalanceApplication refreshes cloud-authoritative balance snapshots for the
// current profile binding. Snapshots are isolated by the full binding scope.
type BalanceApplication struct {
	bindings  BindingResolver
	client    CloudAccountClient
	snapshots AssetSnapshotRepository
}

func NewBalanceApplication(deps BalanceApplicationDependencies) (*BalanceApplication, error) {
	if isNilDependency(deps.Bindings) || isNilDependency(deps.Client) {
		return nil, fmt.Errorf("cloud: balance application is not configured")
	}
	snapshots := deps.Snapshots
	if isNilDependency(snapshots) {
		snapshots = NewMemoryAssetSnapshotRepository()
	}
	return &BalanceApplication{
		bindings:  deps.Bindings,
		client:    deps.Client,
		snapshots: snapshots,
	}, nil
}

// Refresh obtains the current balance only after resolving a bound scope for
// actor. A failed refresh returns the last confirmed snapshot for that scope.
func (a *BalanceApplication) Refresh(ctx context.Context, actor userctx.ActorContext) (BalanceView, error) {
	binding, scope, err := a.currentBindingScope(ctx, actor)
	if err != nil {
		return BalanceView{}, err
	}

	cached, _, err := a.snapshots.LoadBalance(ctx, scope)
	if err != nil {
		return BalanceView{}, err
	}
	state, err := a.client.GetCurrentUser(ctx, scope)
	if err != nil {
		return balanceView(cached), err
	}
	if err := state.Validate(); err != nil {
		return balanceView(cached), err
	}
	stateScope, err := state.Binding.Scope()
	if err != nil {
		return balanceView(cached), err
	}
	if stateScope != scope {
		return balanceView(cached), &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: balance snapshot belongs to another binding"}
	}
	state.Binding = binding

	current, err := a.snapshots.StoreBalance(ctx, scope, state.Balance)
	if err != nil {
		return balanceView(cached), err
	}
	return balanceView(current), nil
}

// View returns the last confirmed balance for actor's current binding scope.
// It never falls back to a snapshot associated with an old binding generation.
func (a *BalanceApplication) View(ctx context.Context, actor userctx.ActorContext) (BalanceView, bool, error) {
	_, scope, err := a.currentBindingScope(ctx, actor)
	if err != nil {
		return BalanceView{}, false, err
	}
	snapshot, ok, err := a.snapshots.LoadBalance(ctx, scope)
	return balanceView(snapshot), ok, err
}

func (a *BalanceApplication) currentBindingScope(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, BindingScope, error) {
	actor = actor.Normalize()
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return ProfileBinding{}, BindingScope{}, err
	}
	if binding.ActorID != actor.ActorID || binding.ProfileID != actor.ProfileID || binding.Status != BindingBound {
		return ProfileBinding{}, BindingScope{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: balance requires the actor's bound profile"}
	}
	scope, err := binding.Scope()
	if err != nil {
		return ProfileBinding{}, BindingScope{}, err
	}
	return binding, scope, nil
}

func balanceView(snapshot BalanceSnapshot) BalanceView {
	return snapshot
}
