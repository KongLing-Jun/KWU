package cloud

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// PaymentAttempt is persisted before the create-order request is sent. Retries
// of the same explicit attempt reuse its idempotency key, while a later
// terminal or expired order can start a new attempt.
type PaymentAttempt struct {
	Scope          BindingScope     `json:"scope"`
	AttemptID      string           `json:"attempt_id"`
	Operation      PaymentOrderType `json:"operation"`
	Plan           string           `json:"plan"`
	PaymentChannel PaymentChannel   `json:"payment_channel"`
	IdempotencyKey string           `json:"idempotency_key"`
	OrderID        string           `json:"order_id,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	// Sequence is assigned atomically by the repository within one binding
	// scope. Callers must leave it zero.
	Sequence uint64 `json:"sequence"`
}

func (a PaymentAttempt) Validate() error {
	if err := a.Scope.Validate(); err != nil {
		return err
	}
	if err := validatePaymentAttemptIdentity(a); err != nil {
		return err
	}
	if a.OrderID != "" {
		if err := requireTrimmed("order_id", a.OrderID); err != nil {
			return err
		}
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("cloud: payment attempt created_at is required")
	}
	return nil
}

func validatePaymentAttemptIdentity(a PaymentAttempt) error {
	if err := requireTrimmed("attempt_id", a.AttemptID); err != nil {
		return err
	}
	switch a.Operation {
	case PaymentOrderSubscriptionPurchase, PaymentOrderSubscriptionUpgrade:
	default:
		return fmt.Errorf("cloud: invalid payment attempt operation %q", a.Operation)
	}
	if err := requireTrimmed("plan", a.Plan); err != nil {
		return err
	}
	if err := a.PaymentChannel.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("idempotency_key", a.IdempotencyKey); err != nil {
		return err
	}
	expectedKey, err := paymentOrderIdempotencyKey(a.Operation, a.Scope, a.Plan, a.PaymentChannel, a.AttemptID)
	if err != nil {
		return err
	}
	if a.IdempotencyKey != expectedKey {
		return fmt.Errorf("cloud: payment attempt idempotency key does not match attempt identity")
	}
	return nil
}

// PaymentOrderRecord is the local, scope-bound recovery record for one order
// created through the frozen payment contract. It preserves the cloud result
// verbatim and deliberately does not infer entitlement state from it.
type PaymentOrderRecord struct {
	Scope          BindingScope       `json:"scope"`
	Result         PaymentOrderResult `json:"result"`
	Operation      PaymentOrderType   `json:"operation"`
	IdempotencyKey string             `json:"idempotency_key"`
	// GrantSequence is assigned atomically by the repository when this order
	// first becomes granted. Callers and fixtures must leave it zero.
	GrantSequence uint64 `json:"grant_sequence,omitempty"`
	// EntitlementsRefreshed is persisted only after a granted order's
	// cloud-authoritative entitlement snapshot has been refreshed.
	EntitlementsRefreshed bool `json:"entitlements_refreshed"`
}

func NewPaymentOrderRecord(scope BindingScope, result PaymentOrderResult, operation PaymentOrderType, idempotencyKey string) (PaymentOrderRecord, error) {
	record := PaymentOrderRecord{
		Scope:          scope,
		Result:         result,
		Operation:      operation,
		IdempotencyKey: idempotencyKey,
	}
	return record, record.Validate()
}

func (r PaymentOrderRecord) Validate() error {
	if err := r.Scope.Validate(); err != nil {
		return err
	}
	if err := r.Result.Validate(); err != nil {
		return err
	}
	if r.Scope != r.Result.Order.Scope {
		return fmt.Errorf("cloud: payment order record belongs to another binding scope")
	}
	if r.Operation != r.Result.Order.OrderType {
		return fmt.Errorf("cloud: payment order record operation does not match order type")
	}
	if err := r.validateGrantMetadata(); err != nil {
		return err
	}
	return requireTrimmed("idempotency_key", r.IdempotencyKey)
}

func (r PaymentOrderRecord) validateGrantMetadata() error {
	granted := r.Result.Order.Status == PaymentGranted && r.Result.Order.GrantStatus == GrantGranted
	if r.EntitlementsRefreshed && !granted {
		return fmt.Errorf("cloud: payment entitlement refresh marker requires a granted order")
	}
	if r.GrantSequence != 0 && !granted {
		return fmt.Errorf("cloud: payment grant sequence requires a granted order")
	}
	return nil
}

// PaymentOrderRepository preserves only locally created records. All reads are
// addressed by a complete BindingScope, so it cannot discover or expose orders
// from another profile, cloud user, or binding generation.
type PaymentOrderRepository interface {
	Save(context.Context, PaymentOrderRecord) error
	Get(context.Context, BindingScope, string) (PaymentOrderRecord, bool, error)
	FindByIdempotencyKey(context.Context, BindingScope, string) (PaymentOrderRecord, bool, error)
	List(context.Context, BindingScope) ([]PaymentOrderRecord, error)
	MarkEntitlementsRefreshed(context.Context, BindingScope, string) error
	ReserveAttempt(context.Context, string, PaymentAttempt) (PaymentAttempt, bool, error)
	LatestAttempt(context.Context, BindingScope, PaymentOrderType, string, PaymentChannel) (PaymentAttempt, bool, error)
	BindAttempt(context.Context, BindingScope, string, string) error
}

// MemoryPaymentOrderRepository is the L1 fake implementation. Save atomically
// reserves an order ID and idempotency key pair within one full binding scope.
type MemoryPaymentOrderRepository struct {
	mu       sync.RWMutex
	records  map[string]paymentOrderRecordBucket
	attempts map[string]map[string]PaymentAttempt
}

type paymentOrderRecordBucket struct {
	byOrderID        map[string]PaymentOrderRecord
	byIdempotencyKey map[string]PaymentOrderRecord
}

func NewMemoryPaymentOrderRepository() *MemoryPaymentOrderRepository {
	return &MemoryPaymentOrderRepository{
		records:  make(map[string]paymentOrderRecordBucket),
		attempts: make(map[string]map[string]PaymentAttempt),
	}
}

func (r *MemoryPaymentOrderRepository) Save(ctx context.Context, record PaymentOrderRecord) error {
	if err := validatePaymentOrderRecordSave(ctx, record); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	bucket := r.records[record.Scope.key()]
	prepared, err := bucket.prepareSave(record)
	if err != nil {
		return err
	}
	bucket.save(prepared)
	r.records[record.Scope.key()] = bucket
	return nil
}

func validatePaymentOrderRecordSave(ctx context.Context, record PaymentOrderRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return record.Validate()
}

func (b paymentOrderRecordBucket) validateIndexes(record PaymentOrderRecord) error {
	existingByOrderID, hasOrderID := b.byOrderID[record.Result.Order.OrderID]
	if hasOrderID && existingByOrderID.IdempotencyKey != record.IdempotencyKey {
		return fmt.Errorf("cloud: payment order ID is already reserved by another idempotency key")
	}
	existingByKey, hasKey := b.byIdempotencyKey[record.IdempotencyKey]
	if hasKey && existingByKey.Result.Order.OrderID != record.Result.Order.OrderID {
		return fmt.Errorf("cloud: payment idempotency key is already reserved by another order ID")
	}
	if hasOrderID && hasKey && existingByOrderID != existingByKey {
		return fmt.Errorf("cloud: payment order record indexes disagree")
	}
	return nil
}

func (b paymentOrderRecordBucket) prepareSave(record PaymentOrderRecord) (PaymentOrderRecord, error) {
	if err := b.validateIndexes(record); err != nil {
		return PaymentOrderRecord{}, err
	}
	existing, found := b.byOrderID[record.Result.Order.OrderID]
	if !found {
		return b.prepareNewPaymentOrderRecord(record)
	}
	return b.prepareExistingPaymentOrderRecord(existing, record)
}

func (b paymentOrderRecordBucket) prepareNewPaymentOrderRecord(record PaymentOrderRecord) (PaymentOrderRecord, error) {
	if record.GrantSequence != 0 {
		return PaymentOrderRecord{}, fmt.Errorf("cloud: payment grant sequence is repository-assigned")
	}
	if firstPaymentGrant(PaymentOrderRecord{}, false, record) {
		sequence, err := b.nextGrantSequence()
		if err != nil {
			return PaymentOrderRecord{}, err
		}
		record.GrantSequence = sequence
	}
	return record, record.Validate()
}

func (b paymentOrderRecordBucket) prepareExistingPaymentOrderRecord(existing, record PaymentOrderRecord) (PaymentOrderRecord, error) {
	if !samePaymentOrderRecordIdentity(existing, record) {
		return PaymentOrderRecord{}, fmt.Errorf("cloud: saved payment order identity does not match update")
	}
	if err := validatePaymentStatusTransition(existing.Result.Order.Status, record.Result.Order.Status); err != nil {
		return PaymentOrderRecord{}, err
	}
	if record.GrantSequence != 0 && record.GrantSequence != existing.GrantSequence {
		return PaymentOrderRecord{}, fmt.Errorf("cloud: payment grant sequence is repository-assigned")
	}
	if firstPaymentGrant(existing, true, record) {
		if record.GrantSequence != 0 {
			return PaymentOrderRecord{}, fmt.Errorf("cloud: payment grant sequence is repository-assigned")
		}
		sequence, err := b.nextGrantSequence()
		if err != nil {
			return PaymentOrderRecord{}, err
		}
		record.GrantSequence = sequence
	} else {
		record.GrantSequence = existing.GrantSequence
	}
	record.EntitlementsRefreshed = existing.EntitlementsRefreshed || record.EntitlementsRefreshed
	return record, record.Validate()
}

func firstPaymentGrant(existing PaymentOrderRecord, found bool, record PaymentOrderRecord) bool {
	return record.Result.Order.Status == PaymentGranted && (!found || existing.Result.Order.Status != PaymentGranted)
}

func (b paymentOrderRecordBucket) nextGrantSequence() (uint64, error) {
	var maximum uint64
	for _, record := range b.byOrderID {
		if record.GrantSequence > maximum {
			maximum = record.GrantSequence
		}
	}
	if maximum == ^uint64(0) {
		return 0, fmt.Errorf("cloud: payment grant sequence exhausted")
	}
	return maximum + 1, nil
}

func samePaymentOrderRecordIdentity(left, right PaymentOrderRecord) bool {
	return left.Scope == right.Scope &&
		left.Operation == right.Operation &&
		left.IdempotencyKey == right.IdempotencyKey &&
		samePaymentOrderIdentity(left.Result.Order, right.Result.Order) &&
		paymentEntryUpdateMatches(left, right)
}

func paymentEntryUpdateMatches(existing, updated PaymentOrderRecord) bool {
	if samePaymentEntryIdentity(existing.Result.Entry, updated.Result.Entry) {
		return true
	}
	return existing.Result.Order.Status == PaymentCreated &&
		existing.Result.Entry == (PaymentEntry{}) && updated.Result.Entry != (PaymentEntry{})
}

func samePaymentEntryIdentity(left, right PaymentEntry) bool {
	return left.Channel == right.Channel &&
		left.EntryType == right.EntryType &&
		left.QRCode == right.QRCode &&
		left.CodeURL == right.CodeURL &&
		left.QRCodeURL == right.QRCodeURL &&
		left.ExpiresAt.Equal(right.ExpiresAt)
}

func (b *paymentOrderRecordBucket) save(record PaymentOrderRecord) {
	if b.byOrderID == nil {
		b.byOrderID = make(map[string]PaymentOrderRecord)
		b.byIdempotencyKey = make(map[string]PaymentOrderRecord)
	}
	b.byOrderID[record.Result.Order.OrderID] = record
	b.byIdempotencyKey[record.IdempotencyKey] = record
}

func (r *MemoryPaymentOrderRepository) Get(ctx context.Context, scope BindingScope, orderID string) (PaymentOrderRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	if err := scope.Validate(); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	if err := requireTrimmed("order_id", orderID); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	r.mu.RLock()
	record, ok := r.records[scope.key()].byOrderID[orderID]
	r.mu.RUnlock()
	return record, ok, nil
}

func (r *MemoryPaymentOrderRepository) FindByIdempotencyKey(ctx context.Context, scope BindingScope, idempotencyKey string) (PaymentOrderRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	if err := scope.Validate(); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	if err := requireTrimmed("idempotency_key", idempotencyKey); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	r.mu.RLock()
	record, ok := r.records[scope.key()].byIdempotencyKey[idempotencyKey]
	r.mu.RUnlock()
	return record, ok, nil
}

// List returns only records previously saved under this complete binding scope.
// It deliberately provides no remote pending/list discovery capability.
func (r *MemoryPaymentOrderRepository) List(ctx context.Context, scope BindingScope) ([]PaymentOrderRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	bucket := r.records[scope.key()]
	records := make([]PaymentOrderRecord, 0, len(bucket.byOrderID))
	for _, record := range bucket.byOrderID {
		records = append(records, record)
	}
	r.mu.RUnlock()
	sort.Slice(records, func(i, j int) bool {
		return records[i].Result.Order.OrderID < records[j].Result.Order.OrderID
	})
	return records, nil
}

// MarkEntitlementsRefreshed atomically persists a successful entitlement
// refresh for a cloud-granted order. It cannot grant or infer entitlements.
func (r *MemoryPaymentOrderRepository) MarkEntitlementsRefreshed(ctx context.Context, scope BindingScope, orderID string) error {
	if err := validateEntitlementRefreshMark(ctx, scope, orderID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.markEntitlementsRefreshed(scope, orderID)
}

func validateEntitlementRefreshMark(ctx context.Context, scope BindingScope, orderID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	return requireTrimmed("order_id", orderID)
}

func (r *MemoryPaymentOrderRepository) markEntitlementsRefreshed(scope BindingScope, orderID string) error {
	bucket, ok := r.records[scope.key()]
	if !ok {
		return paymentScopeMismatch("cloud: payment order is not saved for the current binding")
	}
	record, ok := bucket.byOrderID[orderID]
	if !ok || record.Scope != scope || record.Result.Order.OrderID != orderID {
		return paymentScopeMismatch("cloud: payment order is not saved for the current binding")
	}
	if record.Result.Order.Status != PaymentGranted || record.Result.Order.GrantStatus != GrantGranted {
		return fmt.Errorf("cloud: payment entitlements cannot be marked before grant")
	}
	if record.EntitlementsRefreshed {
		return nil
	}
	record.EntitlementsRefreshed = true
	bucket.byOrderID[orderID] = record
	bucket.byIdempotencyKey[record.IdempotencyKey] = record
	r.records[scope.key()] = bucket
	return nil
}

func (r *MemoryPaymentOrderRepository) ReserveAttempt(ctx context.Context, expectedLatestID string, candidate PaymentAttempt) (PaymentAttempt, bool, error) {
	if err := ctx.Err(); err != nil {
		return PaymentAttempt{}, false, err
	}
	if err := candidate.Validate(); err != nil {
		return PaymentAttempt{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	scopeKey := candidate.Scope.key()
	bucket := r.attempts[scopeKey]
	latest, found := latestMatchingPaymentAttempt(bucket, candidate.Scope, candidate.Operation, candidate.Plan, candidate.PaymentChannel)
	if !paymentAttemptPredecessorMatches(latest, found, expectedLatestID) {
		return latest, false, nil
	}
	prepared, err := prepareNewPaymentAttempt(candidate, bucket)
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	if bucket == nil {
		bucket = make(map[string]PaymentAttempt)
	}
	bucket[prepared.AttemptID] = prepared
	r.attempts[scopeKey] = bucket
	return prepared, true, nil
}

func (r *MemoryPaymentOrderRepository) LatestAttempt(ctx context.Context, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentAttempt, bool, error) {
	if err := validatePaymentAttemptLookup(ctx, scope, operation, plan, channel); err != nil {
		return PaymentAttempt{}, false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	latest, found := latestMatchingPaymentAttempt(r.attempts[scope.key()], scope, operation, plan, channel)
	return latest, found, nil
}

func (r *MemoryPaymentOrderRepository) BindAttempt(ctx context.Context, scope BindingScope, attemptID, orderID string) error {
	if err := validatePaymentAttemptBinding(ctx, scope, attemptID, orderID); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	attempt, found := r.attempts[scope.key()][attemptID]
	if !found {
		return paymentScopeMismatch("cloud: payment attempt is not saved for the current binding")
	}
	record, found := r.records[scope.key()].byOrderID[orderID]
	if !found {
		return paymentScopeMismatch("cloud: payment order is not saved for the current binding")
	}
	if err := validatePaymentAttemptOrder(attempt, record); err != nil {
		return err
	}
	if attempt.OrderID != "" && attempt.OrderID != orderID {
		return fmt.Errorf("cloud: payment attempt is already bound to another order")
	}
	if attempt.OrderID == orderID {
		return nil
	}
	attempt.OrderID = orderID
	r.attempts[scope.key()][attemptID] = attempt
	return nil
}

func validatePaymentAttemptLookup(ctx context.Context, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	switch operation {
	case PaymentOrderSubscriptionPurchase, PaymentOrderSubscriptionUpgrade:
	default:
		return fmt.Errorf("cloud: invalid payment attempt operation %q", operation)
	}
	if err := requireTrimmed("plan", plan); err != nil {
		return err
	}
	return channel.Validate()
}

func validatePaymentAttemptBinding(ctx context.Context, scope BindingScope, attemptID, orderID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed("attempt_id", attemptID); err != nil {
		return err
	}
	return requireTrimmed("order_id", orderID)
}

func validatePaymentAttemptOrder(attempt PaymentAttempt, record PaymentOrderRecord) error {
	if attempt.Scope != record.Scope || attempt.Operation != record.Operation || attempt.IdempotencyKey != record.IdempotencyKey {
		return paymentScopeMismatch("cloud: payment attempt does not match saved order")
	}
	order := record.Result.Order
	planMatches := order.Plan == attempt.Plan
	if attempt.Operation == PaymentOrderSubscriptionPurchase {
		planMatches = order.PlanID == attempt.Plan || order.PlanID == "" && order.Plan == attempt.Plan
	}
	if !planMatches || order.PaymentChannel != attempt.PaymentChannel {
		return paymentScopeMismatch("cloud: payment attempt does not match saved order")
	}
	return nil
}

func samePaymentAttempt(left, right PaymentAttempt) bool {
	return left.Scope == right.Scope &&
		left.AttemptID == right.AttemptID &&
		left.Operation == right.Operation &&
		left.Plan == right.Plan &&
		left.PaymentChannel == right.PaymentChannel &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.OrderID == right.OrderID &&
		left.CreatedAt.Equal(right.CreatedAt) &&
		left.Sequence == right.Sequence
}

func paymentAttemptMatches(attempt PaymentAttempt, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) bool {
	return attempt.Scope == scope &&
		attempt.Operation == operation &&
		attempt.Plan == plan &&
		attempt.PaymentChannel == channel
}

func paymentAttemptNewer(candidate, current PaymentAttempt) bool {
	return candidate.Sequence > current.Sequence
}

func latestMatchingPaymentAttempt(attempts map[string]PaymentAttempt, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentAttempt, bool) {
	var latest PaymentAttempt
	found := false
	for _, attempt := range attempts {
		if paymentAttemptMatches(attempt, scope, operation, plan, channel) && (!found || paymentAttemptNewer(attempt, latest)) {
			latest = attempt
			found = true
		}
	}
	return latest, found
}

func prepareNewPaymentAttempt(attempt PaymentAttempt, existing map[string]PaymentAttempt) (PaymentAttempt, error) {
	if attempt.Sequence != 0 {
		return PaymentAttempt{}, fmt.Errorf("cloud: payment attempt sequence is repository-assigned")
	}
	if attempt.OrderID != "" {
		return PaymentAttempt{}, fmt.Errorf("cloud: new payment attempt cannot already be bound")
	}
	if _, found := existing[attempt.AttemptID]; found {
		return PaymentAttempt{}, fmt.Errorf("cloud: payment attempt ID is already reserved")
	}
	var maximum uint64
	for _, saved := range existing {
		if saved.IdempotencyKey == attempt.IdempotencyKey {
			return PaymentAttempt{}, fmt.Errorf("cloud: payment attempt idempotency key is already reserved")
		}
		if saved.Sequence > maximum {
			maximum = saved.Sequence
		}
	}
	if maximum == ^uint64(0) {
		return PaymentAttempt{}, fmt.Errorf("cloud: payment attempt sequence exhausted")
	}
	attempt.Sequence = maximum + 1
	return attempt, nil
}

func paymentAttemptPredecessorMatches(latest PaymentAttempt, found bool, expectedLatestID string) bool {
	if !found {
		return expectedLatestID == ""
	}
	return latest.AttemptID == expectedLatestID
}

var _ PaymentOrderRepository = (*MemoryPaymentOrderRepository)(nil)
