package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"kwu/internal/userctx"
)

// ErrEventBindingChanged reports that the binding changed after a stream was
// connected. The old stream must not produce side effects or a checkpoint.
var ErrEventBindingChanged = errors.New("cloud: event binding changed")

// EventBalanceRefresher is satisfied by BalanceApplication.Refresh.
type EventBalanceRefresher interface {
	Refresh(context.Context, userctx.ActorContext) (BalanceView, error)
}

// EventTaskRecovery is satisfied by TaskRecoveryApplication's event-safe
// recovery entrypoints.
type EventTaskRecovery interface {
	RecoverPendingForEventSync(context.Context, userctx.ActorContext, TaskRecoveryExclusion) ([]TaskRecoveryDecision, error)
	RecoverExact(context.Context, userctx.ActorContext, BindingScope, string, string) (TaskRecoveryDecision, bool, error)
	StageEventSync(context.Context, BindingScope, string, []TaskRecoveryDecision) error
	AcknowledgeEventSync(context.Context, BindingScope, string) error
}

// eventTaskRecoveryGCRetrier is optional so narrow test adapters do not need
// to own recovery GC. TaskRecoveryApplication implements it for duplicate and
// old events, which may retry only cleanup authorized by an earlier checkpoint.
type eventTaskRecoveryGCRetrier interface {
	RetryAcknowledgedEventSync(context.Context, BindingScope, string) error
}

type eventTaskRecoveryGCSweeper interface {
	ReconcileAppliedEventSync(context.Context, BindingScope, EventCheckpoint) error
}

// EventSettlementQuotaReconciler is the narrow event-to-worker boundary. It
// deliberately carries only actor, full binding identity, and event IDs.
type EventSettlementQuotaReconciler interface {
	ReconcileSettlementEvent(context.Context, userctx.ActorContext, BindingScope, string, string) error
}

// EventSubscriptionRefresher is satisfied by SubscriptionApplication.Refresh.
type EventSubscriptionRefresher interface {
	Refresh(context.Context, userctx.ActorContext) (SubscriptionView, error)
}

// EventPaymentRestorer is satisfied by PaymentRecoveryApplication.Restore.
type EventPaymentRestorer interface {
	Restore(context.Context, userctx.ActorContext) ([]PaymentRecoveryResult, error)
}

// EventThirdPartySyncApplier is satisfied by AuthApplication.ApplyThirdPartySync.
type EventThirdPartySyncApplier interface {
	ApplyThirdPartySync(context.Context, userctx.ActorContext, ThirdPartySync) error
}

// EventAssetChanges reports which cloud-authoritative asset caches were
// refreshed by an applied event. It deliberately excludes event versions,
// binding identity, and asset values because consumers use it only as a wakeup.
type EventAssetChanges struct {
	BalanceChanged      bool
	SubscriptionChanged bool
}

// EventAssetObserver receives a best-effort post-checkpoint asset wakeup. It
// cannot reject an already applied event.
type EventAssetObserver func(EventAssetChanges)

// EventSyncApplicationDependencies contains only normalized L1 boundaries and
// the existing authoritative A1-A3 application entrypoints.
type EventSyncApplicationDependencies struct {
	Bindings      BindingResolver
	Source        CloudEventSource
	State         EventStateRepository
	Balance       EventBalanceRefresher
	Tasks         EventTaskRecovery
	Settlements   EventSettlementQuotaReconciler
	Subscription  EventSubscriptionRefresher
	Payment       EventPaymentRestorer
	Sync          EventThirdPartySyncApplier
	AssetObserver EventAssetObserver

	// MaxReconnects bounds local attempts after a stream closes or Connect
	// fails. It does not imply a server cursor, replay, or pending-list API.
	MaxReconnects int
}

// EventSyncApplication consumes normalized events only. It never interprets
// event data as a balance, freeze, payment, or entitlement state.
type EventSyncApplication struct {
	bindings      BindingResolver
	source        CloudEventSource
	state         EventStateRepository
	balance       EventBalanceRefresher
	tasks         EventTaskRecovery
	settlements   EventSettlementQuotaReconciler
	subscription  EventSubscriptionRefresher
	payment       EventPaymentRestorer
	sync          EventThirdPartySyncApplier
	assetObserver EventAssetObserver
	maxReconnects int
}

func NewEventSyncApplication(deps EventSyncApplicationDependencies) (*EventSyncApplication, error) {
	if !eventSyncDependenciesConfigured(deps) {
		return nil, fmt.Errorf("cloud: event sync application is not configured")
	}
	if deps.MaxReconnects < 0 {
		return nil, fmt.Errorf("cloud: event sync max reconnects must not be negative")
	}
	return &EventSyncApplication{
		bindings: deps.Bindings, source: deps.Source, state: deps.State,
		balance: deps.Balance, tasks: deps.Tasks, subscription: deps.Subscription,
		settlements: deps.Settlements, payment: deps.Payment, sync: deps.Sync,
		assetObserver: deps.AssetObserver, maxReconnects: deps.MaxReconnects,
	}, nil
}

func eventSyncDependenciesConfigured(deps EventSyncApplicationDependencies) bool {
	values := []any{
		deps.Bindings, deps.Source, deps.State, deps.Balance, deps.Tasks,
		deps.Settlements, deps.Subscription, deps.Payment, deps.Sync,
	}
	for _, value := range values {
		if isNilDependency(value) {
			return false
		}
	}
	return true
}

// Run connects only to the actor's current binding. A closed stream performs
// local-ID recovery and may reconnect a bounded number of times; it never
// claims a server replay guarantee.
func (a *EventSyncApplication) Run(ctx context.Context, actor userctx.ActorContext) error {
	for attempt := 0; ; attempt++ {
		scope, err := a.prepareRunScope(ctx, actor)
		if err != nil {
			return err
		}
		connectionCtx, cancel := context.WithCancel(ctx)
		events, err := a.source.Connect(connectionCtx, scope)
		if err != nil {
			cancel()
			retry, retryErr := a.retryAfterConnectionError(ctx, actor, err, attempt)
			if retryErr != nil {
				return retryErr
			}
			if retry {
				continue
			}
		}

		reconnect, consumeErr := a.consume(ctx, actor, scope, events, attempt)
		cancel()
		if consumeErr != nil {
			return consumeErr
		}
		if reconnect {
			continue
		}
		if err := a.recoverLocalIDs(ctx, actor); err != nil {
			return err
		}
		if attempt >= a.maxReconnects {
			return nil
		}
	}
}

func (a *EventSyncApplication) prepareRunScope(ctx context.Context, actor userctx.ActorContext) (BindingScope, error) {
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if err := a.reconcileAppliedEventRecovery(ctx, scope); err != nil {
		return BindingScope{}, err
	}
	return scope, nil
}

func (a *EventSyncApplication) reconcileAppliedEventRecovery(ctx context.Context, scope BindingScope) error {
	sweeper, ok := a.tasks.(eventTaskRecoveryGCSweeper)
	if !ok {
		return nil
	}
	checkpoint, found, err := a.state.Load(ctx, scope)
	if err != nil || !found {
		return err
	}
	return sweeper.ReconcileAppliedEventSync(ctx, scope, checkpoint)
}

func (a *EventSyncApplication) retryAfterConnectionError(ctx context.Context, actor userctx.ActorContext, connectErr error, attempt int) (bool, error) {
	if recoveryErr := a.recoverLocalIDs(ctx, actor); recoveryErr != nil {
		return false, errors.Join(connectErr, recoveryErr)
	}
	if attempt >= a.maxReconnects {
		return false, connectErr
	}
	return true, nil
}

func (a *EventSyncApplication) consume(ctx context.Context, actor userctx.ActorContext, scope BindingScope, events <-chan Event, attempt int) (bool, error) {
	for event := range events {
		if err := a.apply(ctx, actor, scope, event); err != nil {
			if attempt < a.maxReconnects && (errors.Is(err, ErrEventBindingChanged) || isRetryableCloudError(err)) {
				return true, nil
			}
			return false, err
		}
	}
	return false, nil
}

func isRetryableCloudError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var cloudErr *Error
	return errors.As(err, &cloudErr) && cloudErr.Retryable
}

func (a *EventSyncApplication) apply(ctx context.Context, actor userctx.ActorContext, connected BindingScope, event Event) error {
	skip, err := a.shouldSkipEvent(ctx, actor, connected, event)
	if err != nil || skip {
		return err
	}
	if err := a.dispatch(ctx, actor, connected, event); err != nil {
		return err
	}
	if err := a.confirmScope(ctx, actor, connected); err != nil {
		return err
	}
	if err := a.state.CommitApplied(ctx, connected, event); err != nil {
		return err
	}
	a.observeAppliedAssetChange(event.EventType)
	// The checkpoint is the last authoritative event step. Retained recovery
	// identities are post-checkpoint garbage collection: a cleanup failure must
	// neither roll back an applied event nor turn it into a false retry failure.
	_ = a.tasks.AcknowledgeEventSync(ctx, connected, event.EventID)
	return nil
}

func (a *EventSyncApplication) observeAppliedAssetChange(eventType EventType) {
	if a.assetObserver == nil {
		return
	}
	var changes EventAssetChanges
	switch eventType {
	case EventBalanceUpdated, EventFreezeCreated, EventFreezeExpired,
		EventSettlementCompleted, EventSettlementException:
		changes.BalanceChanged = true
	case EventSubscriptionUpdated:
		changes.SubscriptionChanged = true
	default:
		return
	}
	// An observer is a post-checkpoint, best-effort side effect. A panic must
	// not turn a committed event into a false stream failure.
	defer func() {
		_ = recover()
	}()
	a.assetObserver(changes)
}

func (a *EventSyncApplication) shouldSkipEvent(ctx context.Context, actor userctx.ActorContext, connected BindingScope, event Event) (bool, error) {
	skip, err := a.preflightEvent(ctx, actor, connected, event)
	if err != nil || skip {
		return skip, err
	}
	checkpoint, found, err := a.state.Load(ctx, connected)
	if err != nil {
		return false, err
	}
	latest := checkpoint.latestVersion()
	if found && (checkpoint.HasApplied(event.EventID) || latest >= event.Version) {
		// A same-process duplicate may retry only cleanup already authorized by
		// an earlier checkpoint. Newly recovered identities stay pending until a
		// new event completes its own checkpoint.
		if checkpoint.HasApplied(event.EventID) {
			if retrier, ok := a.tasks.(eventTaskRecoveryGCRetrier); ok {
				_ = retrier.RetryAcknowledgedEventSync(ctx, connected, event.EventID)
			}
		}
		return true, nil
	}
	return false, a.recoverEventVersionGap(ctx, actor, connected, event, latest)
}

func (a *EventSyncApplication) preflightEvent(
	ctx context.Context,
	actor userctx.ActorContext,
	connected BindingScope,
	event Event,
) (bool, error) {
	if err := validateEventEnvelope(event); err != nil {
		return false, eventDataContractError(event.EventType, err)
	}
	if err := a.confirmScope(ctx, actor, connected); err != nil {
		return false, err
	}
	if err := event.Validate(); err != nil {
		return false, eventDataContractError(event.EventType, err)
	}
	if err := validateEventAuthorityData(event); err != nil {
		return false, err
	}
	if event.CloudUserID != connected.CloudUserID {
		return false, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: event belongs to another binding"}
	}
	return false, nil
}

func (a *EventSyncApplication) recoverEventVersionGap(
	ctx context.Context,
	actor userctx.ActorContext,
	connected BindingScope,
	event Event,
	latest uint64,
) error {
	if event.Version <= latest+1 {
		return nil
	}
	if err := a.recoverLocalIDsForEvent(ctx, actor, event); err != nil {
		return err
	}
	return a.confirmScope(ctx, actor, connected)
}

func validateEventEnvelope(event Event) error {
	if err := requireTrimmed("event_type", string(event.EventType)); err != nil {
		return err
	}
	envelope := event
	envelope.EventType = EventBalanceUpdated
	return envelope.Validate()
}

func (a *EventSyncApplication) dispatch(ctx context.Context, actor userctx.ActorContext, scope BindingScope, event Event) error {
	switch event.EventType {
	case EventBalanceUpdated:
		_, err := a.balance.Refresh(ctx, actor)
		return err
	case EventFreezeCreated, EventFreezeExpired:
		return a.dispatchFreezeEvent(ctx, actor, scope, event)
	case EventSettlementCompleted, EventSettlementException:
		return a.dispatchSettlementEvent(ctx, actor, scope, event)
	case EventSubscriptionUpdated:
		_, err := a.subscription.Refresh(ctx, actor)
		return err
	case EventPaymentUpdated:
		_, err := a.payment.Restore(ctx, actor)
		return err
	case EventThirdPartySyncUpdated:
		return a.dispatchThirdPartySyncEvent(ctx, actor, scope, event)
	default:
		return fmt.Errorf("cloud: unsupported normalized event type %q", event.EventType)
	}
}

func (a *EventSyncApplication) dispatchFreezeEvent(
	ctx context.Context,
	actor userctx.ActorContext,
	scope BindingScope,
	event Event,
) error {
	identity, err := decodeFreezeEventIdentity(event.Data)
	if err != nil {
		return err
	}
	decision, found, err := a.tasks.RecoverExact(ctx, actor, scope, identity.TaskID, identity.FreezeID)
	if err != nil {
		return err
	}
	if found {
		if err := a.tasks.StageEventSync(ctx, scope, event.EventID, []TaskRecoveryDecision{decision}); err != nil {
			return err
		}
	}
	_, err = a.balance.Refresh(ctx, actor)
	return err
}

func (a *EventSyncApplication) dispatchSettlementEvent(
	ctx context.Context,
	actor userctx.ActorContext,
	scope BindingScope,
	event Event,
) error {
	identity, err := decodeSettlementEventIdentity(event.Data)
	if err != nil {
		return err
	}
	if err := a.settlements.ReconcileSettlementEvent(
		ctx, actor, scope, identity.TaskID, identity.SettlementID,
	); err != nil {
		return err
	}
	_, err = a.balance.Refresh(ctx, actor)
	return err
}

func (a *EventSyncApplication) dispatchThirdPartySyncEvent(
	ctx context.Context,
	actor userctx.ActorContext,
	scope BindingScope,
	event Event,
) error {
	syncState, err := decodeEventThirdPartySync(event, scope)
	if err != nil {
		return err
	}
	if err := syncState.Validate(); err != nil {
		return err
	}
	return a.sync.ApplyThirdPartySync(ctx, actor, syncState)
}

type settlementEventIdentity struct {
	TaskID       string `json:"task_id"`
	SettlementID string `json:"settlement_id"`
}

type freezeEventIdentity struct {
	TaskID   string `json:"task_id"`
	FreezeID string `json:"freeze_id"`
}

func decodeFreezeEventIdentity(data []byte) (freezeEventIdentity, error) {
	fields, err := requiredEventDataFields(data, "task_id", "freeze_id")
	if err != nil {
		return freezeEventIdentity{}, fmt.Errorf("cloud: freeze event contract: %w", err)
	}
	var identity freezeEventIdentity
	if err := json.Unmarshal(fields["task_id"], &identity.TaskID); err != nil {
		return freezeEventIdentity{}, fmt.Errorf("cloud: freeze event contract: %w", err)
	}
	if err := json.Unmarshal(fields["freeze_id"], &identity.FreezeID); err != nil {
		return freezeEventIdentity{}, fmt.Errorf("cloud: freeze event contract: %w", err)
	}
	if err := requireTrimmed("task_id", identity.TaskID); err != nil {
		return freezeEventIdentity{}, fmt.Errorf("cloud: freeze event contract: %w", err)
	}
	if err := requireTrimmed("freeze_id", identity.FreezeID); err != nil {
		return freezeEventIdentity{}, fmt.Errorf("cloud: freeze event contract: %w", err)
	}
	return identity, nil
}

func decodeSettlementEventIdentity(data []byte) (settlementEventIdentity, error) {
	fields, err := requiredEventDataFields(data, "task_id", "settlement_id")
	if err != nil {
		return settlementEventIdentity{}, fmt.Errorf("cloud: settlement event contract: %w", err)
	}
	var identity settlementEventIdentity
	if err := json.Unmarshal(fields["task_id"], &identity.TaskID); err != nil {
		return settlementEventIdentity{}, fmt.Errorf("cloud: settlement event contract: %w", err)
	}
	if err := json.Unmarshal(fields["settlement_id"], &identity.SettlementID); err != nil {
		return settlementEventIdentity{}, fmt.Errorf("cloud: settlement event contract: %w", err)
	}
	if err := requireTrimmed("task_id", identity.TaskID); err != nil {
		return settlementEventIdentity{}, fmt.Errorf("cloud: settlement event contract: %w", err)
	}
	if err := requireTrimmed("settlement_id", identity.SettlementID); err != nil {
		return settlementEventIdentity{}, fmt.Errorf("cloud: settlement event contract: %w", err)
	}
	return identity, nil
}

func requiredEventDataFields(data []byte, required ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("event data must be one JSON object")
	}
	for _, name := range required {
		if _, found := fields[name]; !found {
			return nil, fmt.Errorf("requires field %q", name)
		}
	}
	return fields, nil
}

func validateEventAuthorityData(event Event) error {
	switch event.EventType {
	case EventFreezeCreated, EventFreezeExpired:
		_, err := decodeFreezeEventIdentity(event.Data)
		return err
	case EventSettlementCompleted, EventSettlementException:
		_, err := decodeSettlementEventIdentity(event.Data)
		return err
	default:
		return nil
	}
}

func eventDataContractError(eventType EventType, err error) error {
	if err == nil {
		return nil
	}
	switch eventType {
	case EventFreezeCreated, EventFreezeExpired:
		return fmt.Errorf("cloud: freeze event contract: %w", err)
	case EventSettlementCompleted, EventSettlementException:
		return fmt.Errorf("cloud: settlement event contract: %w", err)
	default:
		return err
	}
}

func (a *EventSyncApplication) recoverLocalIDs(ctx context.Context, actor userctx.ActorContext) error {
	_, err := a.recoverLocalIDsExcept(ctx, actor, TaskRecoveryExclusion{})
	return err
}

func (a *EventSyncApplication) recoverLocalIDsForEvent(ctx context.Context, actor userctx.ActorContext, event Event) error {
	exclusion, err := taskRecoveryExclusionForEvent(event)
	if err != nil {
		return err
	}
	decisions, err := a.recoverLocalIDsExcept(ctx, actor, exclusion)
	if err != nil {
		return err
	}
	scope, err := a.currentScope(ctx, actor)
	if err != nil {
		return err
	}
	return a.tasks.StageEventSync(ctx, scope, event.EventID, decisions)
}

func taskRecoveryExclusionForEvent(event Event) (TaskRecoveryExclusion, error) {
	switch event.EventType {
	case EventFreezeCreated, EventFreezeExpired:
		identity, err := decodeFreezeEventIdentity(event.Data)
		if err != nil {
			return TaskRecoveryExclusion{}, err
		}
		return TaskRecoveryExclusion{TaskID: identity.TaskID, FreezeID: identity.FreezeID}, nil
	case EventSettlementCompleted, EventSettlementException:
		identity, err := decodeSettlementEventIdentity(event.Data)
		if err != nil {
			return TaskRecoveryExclusion{}, err
		}
		return TaskRecoveryExclusion{TaskID: identity.TaskID}, nil
	default:
		return TaskRecoveryExclusion{}, nil
	}
}

func (a *EventSyncApplication) recoverLocalIDsExcept(ctx context.Context, actor userctx.ActorContext, exclusion TaskRecoveryExclusion) ([]TaskRecoveryDecision, error) {
	if _, err := a.currentScope(ctx, actor); err != nil {
		return nil, err
	}
	decisions, err := a.tasks.RecoverPendingForEventSync(ctx, actor, exclusion)
	if err != nil {
		return nil, err
	}
	if _, err := a.payment.Restore(ctx, actor); err != nil {
		return nil, err
	}
	if _, err := a.balance.Refresh(ctx, actor); err != nil {
		return nil, err
	}
	_, err = a.subscription.Refresh(ctx, actor)
	return decisions, err
}

func (a *EventSyncApplication) currentScope(ctx context.Context, actor userctx.ActorContext) (BindingScope, error) {
	actor = actor.Normalize()
	binding, err := a.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if err := binding.Validate(); err != nil {
		return BindingScope{}, err
	}
	if binding.Status != BindingBound || binding.ActorID != actor.ActorID || binding.ProfileID != actor.ProfileID {
		return BindingScope{}, fmt.Errorf("%w: actor no longer owns a bound scope", ErrEventBindingChanged)
	}
	return binding.Scope()
}

func (a *EventSyncApplication) confirmScope(ctx context.Context, actor userctx.ActorContext, expected BindingScope) error {
	current, err := a.currentScope(ctx, actor)
	if err != nil {
		return err
	}
	if current != expected {
		return fmt.Errorf("%w: stream scope no longer current", ErrEventBindingChanged)
	}
	return nil
}

func decodeEventThirdPartySync(event Event, scope BindingScope) (ThirdPartySync, error) {
	if err := validateThirdPartySyncDataKeys(event.Data); err != nil {
		return ThirdPartySync{}, err
	}
	syncState := ThirdPartySync{Scope: scope, UpdatedAt: event.OccurredAt}
	decoder := json.NewDecoder(bytes.NewReader(event.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&syncState); err != nil {
		return ThirdPartySync{}, fmt.Errorf("cloud: decode third-party sync event: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ThirdPartySync{}, err
	}
	return syncState, nil
}

func validateThirdPartySyncDataKeys(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("cloud: decode third-party sync event fields: %w", err)
	}
	if len(fields) != 2 || fields["status"] == nil || fields["version"] == nil {
		return fmt.Errorf("cloud: third-party sync event data requires only status and version")
	}
	for name := range fields {
		if name != "status" && name != "version" {
			return fmt.Errorf("cloud: third-party sync event data has unsupported field %q", name)
		}
	}
	return nil
}

var (
	_ EventBalanceRefresher      = (*BalanceApplication)(nil)
	_ EventTaskRecovery          = (*TaskRecoveryApplication)(nil)
	_ EventSubscriptionRefresher = (*SubscriptionApplication)(nil)
	_ EventPaymentRestorer       = (*PaymentRecoveryApplication)(nil)
	_ EventThirdPartySyncApplier = (*AuthApplication)(nil)
)
