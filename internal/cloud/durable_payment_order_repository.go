package cloud

import (
	"context"
	"fmt"
	"sort"

	"kwu/internal/userctx"
)

const (
	durablePaymentOrdersSchema  = "reasonix.cloud.payment_orders"
	durablePaymentOrdersVersion = 2
)

type durablePaymentOrdersDocument struct {
	Schema    string                       `json:"schema"`
	Version   int                          `json:"version"`
	ProfileID string                       `json:"profile_id"`
	Entries   []durablePaymentOrderEntry   `json:"entries"`
	Attempts  []durablePaymentAttemptEntry `json:"attempts,omitempty"`
}

type durablePaymentOrderEntry struct {
	Scope          BindingScope       `json:"scope"`
	OrderID        string             `json:"order_id"`
	IdempotencyKey string             `json:"idempotency_key"`
	Record         PaymentOrderRecord `json:"record"`
}

type durablePaymentAttemptEntry struct {
	Scope          BindingScope   `json:"scope"`
	AttemptID      string         `json:"attempt_id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Attempt        PaymentAttempt `json:"attempt"`
}

type durablePaymentOrdersState struct {
	entries  map[string]durablePaymentOrderEntry
	attempts map[string]durablePaymentAttemptEntry
}

// DurablePaymentOrderRepository persists only locally saved payment orders.
// Each profile has its own document and every lookup remains full-scope bound.
type DurablePaymentOrderRepository struct {
	paths ProfilePathProvider
}

func NewDurablePaymentOrderRepository(paths ProfilePathProvider) *DurablePaymentOrderRepository {
	return &DurablePaymentOrderRepository{paths: paths}
}

func (r *DurablePaymentOrderRepository) Save(ctx context.Context, record PaymentOrderRecord) error {
	if err := validatePaymentOrderRecordSave(ctx, record); err != nil {
		return err
	}
	path, err := r.path(record.Scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	state, err := readDurablePaymentOrders(path, record.Scope.ProfileID)
	if err != nil {
		return err
	}
	bucket := paymentOrderBucketForScope(state.entries, record.Scope)
	prepared, err := bucket.prepareSave(record)
	if err != nil {
		return err
	}
	entry := durablePaymentOrderEntry{
		Scope: prepared.Scope, OrderID: prepared.Result.Order.OrderID,
		IdempotencyKey: prepared.IdempotencyKey, Record: prepared,
	}
	state.entries[durablePaymentOrderEntryKey(entry)] = entry
	return writeDurablePaymentOrders(path, record.Scope.ProfileID, state)
}

func (r *DurablePaymentOrderRepository) Get(ctx context.Context, scope BindingScope, orderID string) (PaymentOrderRecord, bool, error) {
	if err := validatePaymentOrderLookup(ctx, scope, "order_id", orderID); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	state, err := r.read(scope.ProfileID)
	if err != nil {
		return PaymentOrderRecord{}, false, err
	}
	for _, entry := range state.entries {
		if entry.Scope == scope && entry.OrderID == orderID {
			return entry.Record, true, nil
		}
	}
	return PaymentOrderRecord{}, false, nil
}

func (r *DurablePaymentOrderRepository) FindByIdempotencyKey(ctx context.Context, scope BindingScope, idempotencyKey string) (PaymentOrderRecord, bool, error) {
	if err := validatePaymentOrderLookup(ctx, scope, "idempotency_key", idempotencyKey); err != nil {
		return PaymentOrderRecord{}, false, err
	}
	state, err := r.read(scope.ProfileID)
	if err != nil {
		return PaymentOrderRecord{}, false, err
	}
	for _, entry := range state.entries {
		if entry.Scope == scope && entry.IdempotencyKey == idempotencyKey {
			return entry.Record, true, nil
		}
	}
	return PaymentOrderRecord{}, false, nil
}

func (r *DurablePaymentOrderRepository) List(ctx context.Context, scope BindingScope) ([]PaymentOrderRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	state, err := r.read(scope.ProfileID)
	if err != nil {
		return nil, err
	}
	records := make([]PaymentOrderRecord, 0)
	for _, entry := range state.entries {
		if entry.Scope == scope {
			records = append(records, entry.Record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Result.Order.OrderID < records[j].Result.Order.OrderID
	})
	return records, nil
}

func (r *DurablePaymentOrderRepository) MarkEntitlementsRefreshed(ctx context.Context, scope BindingScope, orderID string) error {
	if err := validateEntitlementRefreshMark(ctx, scope, orderID); err != nil {
		return err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	state, err := readDurablePaymentOrders(path, scope.ProfileID)
	if err != nil {
		return err
	}
	for key, entry := range state.entries {
		if entry.Scope != scope || entry.OrderID != orderID {
			continue
		}
		record := entry.Record
		if record.Result.Order.Status != PaymentGranted || record.Result.Order.GrantStatus != GrantGranted {
			return fmt.Errorf("cloud: payment entitlements cannot be marked before grant")
		}
		if record.EntitlementsRefreshed {
			return nil
		}
		record.EntitlementsRefreshed = true
		entry.Record = record
		state.entries[key] = entry
		return writeDurablePaymentOrders(path, scope.ProfileID, state)
	}
	return paymentScopeMismatch("cloud: payment order is not saved for the current binding")
}

func (r *DurablePaymentOrderRepository) ReserveAttempt(ctx context.Context, expectedLatestID string, candidate PaymentAttempt) (PaymentAttempt, bool, error) {
	if err := ctx.Err(); err != nil {
		return PaymentAttempt{}, false, err
	}
	if err := candidate.Validate(); err != nil {
		return PaymentAttempt{}, false, err
	}
	path, err := r.path(candidate.Scope.ProfileID)
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	state, err := readDurablePaymentOrders(path, candidate.Scope.ProfileID)
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	attempts := paymentAttemptsForScope(state, candidate.Scope)
	latest, found := latestMatchingPaymentAttempt(attempts, candidate.Scope, candidate.Operation, candidate.Plan, candidate.PaymentChannel)
	if !paymentAttemptPredecessorMatches(latest, found, expectedLatestID) {
		return latest, false, nil
	}
	prepared, err := prepareNewPaymentAttempt(candidate, attempts)
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	key := durablePaymentAttemptKey(prepared.Scope, prepared.AttemptID)
	state.attempts[key] = durablePaymentAttemptEntry{
		Scope: prepared.Scope, AttemptID: prepared.AttemptID,
		IdempotencyKey: prepared.IdempotencyKey, Attempt: prepared,
	}
	if err := writeDurablePaymentOrders(path, prepared.Scope.ProfileID, state); err != nil {
		return PaymentAttempt{}, false, err
	}
	return prepared, true, nil
}

func (r *DurablePaymentOrderRepository) LatestAttempt(ctx context.Context, scope BindingScope, operation PaymentOrderType, plan string, channel PaymentChannel) (PaymentAttempt, bool, error) {
	if err := validatePaymentAttemptLookup(ctx, scope, operation, plan, channel); err != nil {
		return PaymentAttempt{}, false, err
	}
	state, err := r.read(scope.ProfileID)
	if err != nil {
		return PaymentAttempt{}, false, err
	}
	latest, found := latestMatchingPaymentAttempt(paymentAttemptsForScope(state, scope), scope, operation, plan, channel)
	return latest, found, nil
}

func (r *DurablePaymentOrderRepository) BindAttempt(ctx context.Context, scope BindingScope, attemptID, orderID string) error {
	if err := validatePaymentAttemptBinding(ctx, scope, attemptID, orderID); err != nil {
		return err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	state, err := readDurablePaymentOrders(path, scope.ProfileID)
	if err != nil {
		return err
	}
	attemptKey := durablePaymentAttemptKey(scope, attemptID)
	attemptEntry, found := state.attempts[attemptKey]
	if !found {
		return paymentScopeMismatch("cloud: payment attempt is not saved for the current binding")
	}
	orderEntry, found := state.entries[scope.key()+"\x00"+orderID]
	if !found {
		return paymentScopeMismatch("cloud: payment order is not saved for the current binding")
	}
	if err := validatePaymentAttemptOrder(attemptEntry.Attempt, orderEntry.Record); err != nil {
		return err
	}
	if attemptEntry.Attempt.OrderID != "" && attemptEntry.Attempt.OrderID != orderID {
		return fmt.Errorf("cloud: payment attempt is already bound to another order")
	}
	if attemptEntry.Attempt.OrderID == orderID {
		return nil
	}
	attemptEntry.Attempt.OrderID = orderID
	state.attempts[attemptKey] = attemptEntry
	return writeDurablePaymentOrders(path, scope.ProfileID, state)
}

func validatePaymentOrderLookup(ctx context.Context, scope BindingScope, name, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	return requireTrimmed(name, value)
}

func (r *DurablePaymentOrderRepository) read(profileID string) (durablePaymentOrdersState, error) {
	path, err := r.path(profileID)
	if err != nil {
		return durablePaymentOrdersState{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	return readDurablePaymentOrders(path, profileID)
}

func (r *DurablePaymentOrderRepository) path(profileID string) (string, error) {
	return durableCloudFile(r.paths, userctx.ActorContext{ProfileID: profileID}, "payment_orders.json")
}

func readDurablePaymentOrders(path, profileID string) (durablePaymentOrdersState, error) {
	var document durablePaymentOrdersDocument
	found, err := durableReadJSON(path, &document)
	if err != nil {
		return durablePaymentOrdersState{}, err
	}
	if !found {
		return newDurablePaymentOrdersState(), nil
	}
	state := newDurablePaymentOrdersState()
	if err := validateDurablePaymentOrdersDocument(document, profileID); err != nil {
		return durablePaymentOrdersState{}, err
	}
	if err := loadDurablePaymentOrderEntries(profileID, document.Entries, state); err != nil {
		return durablePaymentOrdersState{}, err
	}
	if err := loadDurablePaymentAttemptEntries(profileID, document.Attempts, state); err != nil {
		return durablePaymentOrdersState{}, err
	}
	return state, nil
}

func validateDurablePaymentOrdersDocument(document durablePaymentOrdersDocument, profileID string) error {
	supportedVersion := document.Version == 1 || document.Version == durablePaymentOrdersVersion
	if document.Schema != durablePaymentOrdersSchema || !supportedVersion {
		return fmt.Errorf("cloud: unsupported durable payment orders schema")
	}
	if document.ProfileID != profileID {
		return fmt.Errorf("cloud: durable payment orders belong to another profile")
	}
	if document.Version == 1 && len(document.Attempts) != 0 {
		return fmt.Errorf("cloud: legacy durable payment orders cannot contain attempts")
	}
	return nil
}

func loadDurablePaymentOrderEntries(profileID string, entries []durablePaymentOrderEntry, state durablePaymentOrdersState) error {
	buckets := make(map[string]paymentOrderRecordBucket)
	for _, entry := range entries {
		if err := validateDurablePaymentOrderEntry(profileID, entry); err != nil {
			return err
		}
		bucket := buckets[entry.Scope.key()]
		if err := bucket.validateIndexes(entry.Record); err != nil {
			return fmt.Errorf("cloud: invalid durable payment order indexes: %w", err)
		}
		bucket.save(entry.Record)
		buckets[entry.Scope.key()] = bucket
		key := durablePaymentOrderEntryKey(entry)
		if _, exists := state.entries[key]; exists {
			return fmt.Errorf("cloud: duplicate durable payment order")
		}
		state.entries[key] = entry
	}
	return nil
}

func loadDurablePaymentAttemptEntries(profileID string, entries []durablePaymentAttemptEntry, state durablePaymentOrdersState) error {
	for _, entry := range entries {
		if err := validateDurablePaymentAttemptEntry(profileID, entry); err != nil {
			return err
		}
		if err := addDurablePaymentAttemptEntry(state, entry); err != nil {
			return err
		}
	}
	return nil
}

func addDurablePaymentAttemptEntry(state durablePaymentOrdersState, entry durablePaymentAttemptEntry) error {
	key := durablePaymentAttemptKey(entry.Scope, entry.AttemptID)
	if _, exists := state.attempts[key]; exists {
		return fmt.Errorf("cloud: duplicate durable payment attempt")
	}
	for _, existing := range state.attempts {
		if existing.Scope == entry.Scope && existing.IdempotencyKey == entry.IdempotencyKey {
			return fmt.Errorf("cloud: duplicate durable payment attempt idempotency key")
		}
		if existing.Scope == entry.Scope && existing.Attempt.Sequence == entry.Attempt.Sequence {
			return fmt.Errorf("cloud: duplicate durable payment attempt sequence")
		}
	}
	if err := validateDurablePaymentAttemptOrderReference(state, entry); err != nil {
		return err
	}
	state.attempts[key] = entry
	return nil
}

func validateDurablePaymentAttemptOrderReference(state durablePaymentOrdersState, entry durablePaymentAttemptEntry) error {
	if entry.Attempt.OrderID == "" {
		return nil
	}
	orderEntry, found := state.entries[entry.Scope.key()+"\x00"+entry.Attempt.OrderID]
	if !found {
		return fmt.Errorf("cloud: durable payment attempt references an unknown order")
	}
	if err := validatePaymentAttemptOrder(entry.Attempt, orderEntry.Record); err != nil {
		return fmt.Errorf("cloud: invalid durable payment attempt order: %w", err)
	}
	return nil
}

func newDurablePaymentOrdersState() durablePaymentOrdersState {
	return durablePaymentOrdersState{
		entries:  make(map[string]durablePaymentOrderEntry),
		attempts: make(map[string]durablePaymentAttemptEntry),
	}
}

func validateDurablePaymentOrderEntry(profileID string, entry durablePaymentOrderEntry) error {
	if err := entry.Scope.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable payment order scope: %w", err)
	}
	if entry.Scope.ProfileID != profileID {
		return fmt.Errorf("cloud: durable payment order belongs to another profile")
	}
	if err := entry.Record.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable payment order record: %w", err)
	}
	if entry.Record.Scope != entry.Scope || entry.Record.Result.Order.Scope != entry.Scope {
		return fmt.Errorf("cloud: durable payment order record scope mismatch")
	}
	if entry.OrderID != entry.Record.Result.Order.OrderID {
		return fmt.Errorf("cloud: durable payment order ID mismatch")
	}
	if entry.IdempotencyKey != entry.Record.IdempotencyKey {
		return fmt.Errorf("cloud: durable payment idempotency key mismatch")
	}
	return nil
}

func validateDurablePaymentAttemptEntry(profileID string, entry durablePaymentAttemptEntry) error {
	if err := entry.Scope.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable payment attempt scope: %w", err)
	}
	if entry.Scope.ProfileID != profileID {
		return fmt.Errorf("cloud: durable payment attempt belongs to another profile")
	}
	if err := entry.Attempt.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable payment attempt: %w", err)
	}
	if entry.Scope != entry.Attempt.Scope {
		return fmt.Errorf("cloud: durable payment attempt scope mismatch")
	}
	if entry.AttemptID != entry.Attempt.AttemptID {
		return fmt.Errorf("cloud: durable payment attempt ID mismatch")
	}
	if entry.IdempotencyKey != entry.Attempt.IdempotencyKey {
		return fmt.Errorf("cloud: durable payment attempt idempotency key mismatch")
	}
	if entry.Attempt.Sequence == 0 {
		return fmt.Errorf("cloud: durable payment attempt sequence is required")
	}
	return nil
}

func paymentAttemptsForScope(state durablePaymentOrdersState, scope BindingScope) map[string]PaymentAttempt {
	attempts := make(map[string]PaymentAttempt)
	for _, entry := range state.attempts {
		if entry.Scope == scope {
			attempts[entry.AttemptID] = entry.Attempt
		}
	}
	return attempts
}

func paymentOrderBucketForScope(entries map[string]durablePaymentOrderEntry, scope BindingScope) paymentOrderRecordBucket {
	bucket := paymentOrderRecordBucket{}
	for _, entry := range entries {
		if entry.Scope == scope {
			bucket.save(entry.Record)
		}
	}
	return bucket
}

func durablePaymentOrderEntryKey(entry durablePaymentOrderEntry) string {
	return entry.Scope.key() + "\x00" + entry.OrderID
}

func durablePaymentAttemptKey(scope BindingScope, attemptID string) string {
	return scope.key() + "\x00" + attemptID
}

func writeDurablePaymentOrders(path, profileID string, state durablePaymentOrdersState) error {
	keys := make([]string, 0, len(state.entries))
	for key := range state.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]durablePaymentOrderEntry, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, state.entries[key])
	}
	attemptKeys := make([]string, 0, len(state.attempts))
	for key := range state.attempts {
		attemptKeys = append(attemptKeys, key)
	}
	sort.Strings(attemptKeys)
	attempts := make([]durablePaymentAttemptEntry, 0, len(attemptKeys))
	for _, key := range attemptKeys {
		attempts = append(attempts, state.attempts[key])
	}
	document := durablePaymentOrdersDocument{
		Schema: durablePaymentOrdersSchema, Version: durablePaymentOrdersVersion,
		ProfileID: profileID, Entries: ordered, Attempts: attempts,
	}
	return durableWriteJSON(path, document)
}

var _ PaymentOrderRepository = (*DurablePaymentOrderRepository)(nil)
