package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestEventSyncApplicationDispatchesOnlyAuthoritativeEntrypoints(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	for _, tc := range []struct {
		name  string
		event Event
		want  eventSyncCalls
	}{
		{"balance", eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`), eventSyncCalls{balance: 2, tasks: 1, subscription: 1, payment: 1}},
		{"freeze", eventSyncEvent("freeze", EventFreezeCreated, scope.CloudUserID, 1, `{"task_id":"task-freeze","freeze_id":"freeze"}`), eventSyncCalls{balance: 2, tasks: 1, exact: 1, subscription: 1, payment: 1}},
		{"expired freeze", eventSyncEvent("expired-freeze", EventFreezeExpired, scope.CloudUserID, 1, `{"task_id":"task-expired","freeze_id":"expired-freeze"}`), eventSyncCalls{balance: 2, tasks: 1, exact: 1, subscription: 1, payment: 1}},
		{"settlement", eventSyncEvent("settlement", EventSettlementCompleted, scope.CloudUserID, 1, `{"task_id":"task-settlement","settlement_id":"settlement"}`), eventSyncCalls{balance: 2, tasks: 1, settlements: 1, subscription: 1, payment: 1}},
		{"exception settlement", eventSyncEvent("exception-settlement", EventSettlementException, scope.CloudUserID, 1, `{"task_id":"task-exception","settlement_id":"exception-settlement"}`), eventSyncCalls{balance: 2, tasks: 1, settlements: 1, subscription: 1, payment: 1}},
		{"subscription", eventSyncEvent("subscription", EventSubscriptionUpdated, scope.CloudUserID, 1, `{}`), eventSyncCalls{balance: 1, tasks: 1, subscription: 2, payment: 1}},
		{"payment", eventSyncEvent("payment", EventPaymentUpdated, scope.CloudUserID, 1, `{}`), eventSyncCalls{balance: 1, tasks: 1, subscription: 1, payment: 2}},
		{"third party", eventSyncEvent("sync", EventThirdPartySyncUpdated, scope.CloudUserID, 1, `{"status":"succeeded","version":3}`), eventSyncCalls{balance: 1, tasks: 1, subscription: 1, payment: 1, sync: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			app := newEventSyncApplication(t, deps, tc.event)
			if err := app.Run(ctx, eventSyncActor()); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := deps.calls(); got != tc.want {
				t.Fatalf("authority calls = %#v, want %#v", got, tc.want)
			}
			checkpoint, found, err := deps.State.Load(ctx, scope)
			if err != nil || !found || checkpoint.Cursor != tc.event.EventID {
				t.Fatalf("checkpoint = %#v, %t, %v", checkpoint, found, err)
			}
		})
	}
}

func TestEventSyncApplicationObservesOnlyAppliedAssetEvents(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	for _, tc := range []struct {
		name  string
		event Event
		want  []EventAssetChanges
	}{
		{"balance", eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`), []EventAssetChanges{{BalanceChanged: true}}},
		{"freeze", eventSyncEvent("freeze", EventFreezeCreated, scope.CloudUserID, 1, `{"task_id":"task-freeze","freeze_id":"freeze"}`), []EventAssetChanges{{BalanceChanged: true}}},
		{"expired freeze", eventSyncEvent("expired-freeze", EventFreezeExpired, scope.CloudUserID, 1, `{"task_id":"task-expired","freeze_id":"expired-freeze"}`), []EventAssetChanges{{BalanceChanged: true}}},
		{"settlement", eventSyncEvent("settlement", EventSettlementCompleted, scope.CloudUserID, 1, `{"task_id":"task-settlement","settlement_id":"settlement"}`), []EventAssetChanges{{BalanceChanged: true}}},
		{"exception settlement", eventSyncEvent("exception-settlement", EventSettlementException, scope.CloudUserID, 1, `{"task_id":"task-exception","settlement_id":"exception-settlement"}`), []EventAssetChanges{{BalanceChanged: true}}},
		{"subscription", eventSyncEvent("subscription", EventSubscriptionUpdated, scope.CloudUserID, 1, `{}`), []EventAssetChanges{{SubscriptionChanged: true}}},
		{"payment excluded", eventSyncEvent("payment", EventPaymentUpdated, scope.CloudUserID, 1, `{}`), nil},
		{"third-party sync excluded", eventSyncEvent("sync", EventThirdPartySyncUpdated, scope.CloudUserID, 1, `{"status":"succeeded","version":3}`), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			var observed []EventAssetChanges
			deps.AssetObserver = func(changes EventAssetChanges) {
				checkpoint, found, err := deps.State.Load(ctx, scope)
				if err != nil || !found || checkpoint.Cursor != tc.event.EventID {
					t.Fatalf("observer ran before checkpoint: checkpoint=%#v found=%t err=%v", checkpoint, found, err)
				}
				if deps.Tasks.(*eventSyncTasks).ackCalls != 0 {
					t.Fatal("observer ran after event recovery acknowledgement")
				}
				observed = append(observed, changes)
			}
			if err := newEventSyncApplication(t, deps, tc.event).Run(ctx, eventSyncActor()); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !reflect.DeepEqual(observed, tc.want) {
				t.Fatalf("observed changes = %#v, want %#v", observed, tc.want)
			}
		})
	}
}

func TestEventSyncApplicationDoesNotObserveUnappliedEvents(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	first := eventSyncEvent("first", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	for _, tc := range []struct {
		name      string
		configure func(*EventSyncApplicationDependencies)
		event     Event
	}{
		{
			name: "refresh failure",
			configure: func(deps *EventSyncApplicationDependencies) {
				deps.Balance.(*eventSyncBalance).err = errors.New("refresh failed")
			},
			event: first,
		},
		{
			name: "binding change",
			configure: func(deps *EventSyncApplicationDependencies) {
				deps.Balance.(*eventSyncBalance).after = func() {
					deps.Bindings.(*eventSyncResolver).scope.BindingGeneration++
				}
			},
			event: first,
		},
		{
			name: "checkpoint failure",
			configure: func(deps *EventSyncApplicationDependencies) {
				deps.State = &eventSyncCommitErrorState{
					EventStateRepository: deps.State,
					err:                  errors.New("checkpoint failed"),
				}
			},
			event: first,
		},
		{
			name: "duplicate",
			configure: func(deps *EventSyncApplicationDependencies) {
				mustCommitEvent(t, deps.State, scope, first)
			},
			event: first,
		},
		{
			name: "old version",
			configure: func(deps *EventSyncApplicationDependencies) {
				mustCommitEvent(t, deps.State, scope, eventSyncEvent("newer", EventBalanceUpdated, scope.CloudUserID, 2, `{}`))
			},
			event: first,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			observed := 0
			deps.AssetObserver = func(EventAssetChanges) { observed++ }
			tc.configure(&deps)
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = app.consume(ctx, eventSyncActor(), scope, eventStream(tc.event), 0)
			if observed != 0 {
				t.Fatalf("observer calls = %d, want 0", observed)
			}
		})
	}
}

func TestEventSyncApplicationIsolatesAssetObserverPanic(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	deps.AssetObserver = func(EventAssetChanges) { panic("observer failed") }
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	first := eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	second := eventSyncEvent("subscription", EventSubscriptionUpdated, scope.CloudUserID, 2, `{}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(first, second), 0); err != nil || reconnect {
		t.Fatalf("consume after observer panic = (%t, %v)", reconnect, err)
	}
	checkpoint, found, err := deps.State.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != second.EventID || !checkpoint.HasApplied(first.EventID) {
		t.Fatalf("checkpoint after observer panic = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationContinuesAfterSafelySkippedEvents(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	first := eventSyncEvent("first", EventBalanceUpdated, scope.CloudUserID, 2, `{}`)
	for _, tc := range []struct {
		name    string
		skipped Event
	}{
		{"duplicate id", first},
		{"same version", eventSyncEvent("same", EventBalanceUpdated, scope.CloudUserID, 2, `{}`)},
		{"old version", eventSyncEvent("old", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)},
		{"old cross-type version", eventSyncEvent("old-settlement", EventSettlementCompleted, scope.CloudUserID, 1, `{"task_id":"old-task","settlement_id":"old-settlement"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			mustCommitEvent(t, deps.State, scope, first)
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			valid := eventSyncEvent("valid", EventSubscriptionUpdated, scope.CloudUserID, 3, `{}`)
			reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(tc.skipped, valid), 0)
			if err != nil || reconnect {
				t.Fatalf("consume = (%t, %v), want continued stream", reconnect, err)
			}

			want := eventSyncCalls{subscription: 1}
			if got := deps.calls(); got != want {
				t.Fatalf("authority calls = %#v, want %#v", got, want)
			}
			checkpoint, found, err := deps.State.Load(ctx, scope)
			if err != nil || !found || checkpoint.Cursor != valid.EventID || (tc.skipped.EventID != first.EventID && checkpoint.HasApplied(tc.skipped.EventID)) {
				t.Fatalf("checkpoint after skip = (%#v, %t, %v)", checkpoint, found, err)
			}
		})
	}
}

func TestEventSyncApplicationRecoversVersionGapBeforeDispatchAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	state := NewMemoryEventStateRepository()
	first := eventSyncEvent("balance-4", EventBalanceUpdated, scope.CloudUserID, 4, `{}`)
	mustCommitEvent(t, state, scope, first)

	otherScopes := []BindingScope{
		{ProfileID: "other-profile", CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration},
		{ProfileID: scope.ProfileID, CloudUserID: "other-cloud", BindingGeneration: scope.BindingGeneration},
		{ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration + 1},
	}
	for i, other := range otherScopes {
		mustCommitEvent(t, state, other, eventSyncEvent("other-scope-"+string(rune('a'+i)), EventBalanceUpdated, other.CloudUserID, 99, `{}`))
	}

	trace := make([]string, 0, 8)
	deps := newEventSyncDependencies(scope)
	deps.State = &eventSyncTracingState{EventStateRepository: state, trace: &trace}
	attachEventSyncTrace(deps, &trace)
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	gap := eventSyncEvent("sync-6", EventThirdPartySyncUpdated, scope.CloudUserID, 6, `{"status":"succeeded","version":6}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(gap), 0); err != nil || reconnect {
		t.Fatalf("consume gap = (%t, %v)", reconnect, err)
	}
	wantGapTrace := []string{"tasks", "payment", "balance", "subscription", "sync", "checkpoint"}
	if !reflect.DeepEqual(trace, wantGapTrace) {
		t.Errorf("gap trace = %v, want recovery, dispatch, checkpoint in order %v", trace, wantGapTrace)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != gap.EventID || checkpoint.Version(EventBalanceUpdated) != 4 || checkpoint.Version(EventThirdPartySyncUpdated) != 6 || !checkpoint.HasApplied(first.EventID) || !checkpoint.HasApplied(gap.EventID) {
		t.Fatalf("checkpoint after gap = (%#v, %t, %v)", checkpoint, found, err)
	}

	trace = trace[:0]
	sameVersion := eventSyncEvent("payment-same-6", EventPaymentUpdated, scope.CloudUserID, 6, `{}`)
	backward := eventSyncEvent("settlement-backward-5", EventSettlementCompleted, scope.CloudUserID, 5, `{"task_id":"backward-task","settlement_id":"backward-settlement"}`)
	next := eventSyncEvent("payment-7", EventPaymentUpdated, scope.CloudUserID, 7, `{}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(gap, sameVersion, backward, next), 0); err != nil || reconnect {
		t.Fatalf("consume duplicate and out-of-order events = (%t, %v)", reconnect, err)
	}
	wantAdvanceTrace := []string{"payment", "checkpoint"}
	if !reflect.DeepEqual(trace, wantAdvanceTrace) {
		t.Errorf("duplicate/out-of-order trace = %v, want only next event %v", trace, wantAdvanceTrace)
	}
	checkpoint, found, err = state.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != next.EventID || checkpoint.Version(EventPaymentUpdated) != next.Version || checkpoint.HasApplied(sameVersion.EventID) || checkpoint.HasApplied(backward.EventID) {
		t.Fatalf("checkpoint after duplicate/out-of-order events = (%#v, %t, %v)", checkpoint, found, err)
	}
	for _, other := range otherScopes {
		otherCheckpoint, otherFound, loadErr := state.Load(ctx, other)
		if loadErr != nil || !otherFound || otherCheckpoint.latestVersion() != 99 {
			t.Fatalf("other scope %#v changed = (%#v, %t, %v)", other, otherCheckpoint, otherFound, loadErr)
		}
	}
}

func TestEventSyncApplicationRecoversInitialVersionGapBeforeDispatchAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	state := NewMemoryEventStateRepository()
	trace := make([]string, 0, 8)
	deps := newEventSyncDependencies(scope)
	deps.State = &eventSyncTracingState{EventStateRepository: state, trace: &trace}
	attachEventSyncTrace(deps, &trace)
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	gap := eventSyncEvent("sync-2", EventThirdPartySyncUpdated, scope.CloudUserID, 2, `{"status":"succeeded","version":2}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(gap), 0); err != nil || reconnect {
		t.Fatalf("consume initial gap = (%t, %v)", reconnect, err)
	}
	wantTrace := []string{"tasks", "payment", "balance", "subscription", "sync", "checkpoint"}
	if !reflect.DeepEqual(trace, wantTrace) {
		t.Errorf("initial gap trace = %v, want recovery, dispatch, checkpoint in order %v", trace, wantTrace)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != gap.EventID || checkpoint.latestVersion() != gap.Version || !checkpoint.HasApplied(gap.EventID) {
		t.Fatalf("checkpoint after initial gap = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationRestoresV1PerTypeVersionsUsingScopeWideMaximum(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	provider := testProfilePathProvider(t.TempDir())
	path := writeHandcraftedEventStateV1(t, provider)
	state := NewDurableEventStateRepository(provider)
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || checkpoint.latestVersion() != 7 || checkpoint.Version(EventBalanceUpdated) != 7 || checkpoint.Version(EventSubscriptionUpdated) != 3 {
		t.Fatalf("Load handcrafted v1 = (%#v, %t, %v)", checkpoint, found, err)
	}

	trace := make([]string, 0, 8)
	deps := newEventSyncDependencies(scope)
	deps.State = &eventSyncTracingState{EventStateRepository: state, trace: &trace}
	attachEventSyncTrace(deps, &trace)
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	contiguous := eventSyncEvent("sync-8", EventThirdPartySyncUpdated, scope.CloudUserID, 8, `{"status":"succeeded","version":8}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(contiguous), 0); err != nil || reconnect {
		t.Fatalf("consume contiguous event after v1 restore = (%t, %v)", reconnect, err)
	}
	if want := []string{"sync", "checkpoint"}; !reflect.DeepEqual(trace, want) {
		t.Errorf("contiguous trace = %v, want max(v1 versions) to avoid recovery %v", trace, want)
	}

	trace = trace[:0]
	gap := eventSyncEvent("sync-10", EventThirdPartySyncUpdated, scope.CloudUserID, 10, `{"status":"succeeded","version":10}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(gap), 0); err != nil || reconnect {
		t.Fatalf("consume gap after v1 restore = (%t, %v)", reconnect, err)
	}
	wantGapTrace := []string{"tasks", "payment", "balance", "subscription", "sync", "checkpoint"}
	if !reflect.DeepEqual(trace, wantGapTrace) {
		t.Errorf("v1 gap trace = %v, want recovery from scope-wide max before dispatch and checkpoint %v", trace, wantGapTrace)
	}
	checkpoint, found, err = state.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != gap.EventID || checkpoint.latestVersion() != 10 || checkpoint.Version(EventBalanceUpdated) != 7 || checkpoint.Version(EventSubscriptionUpdated) != 3 || checkpoint.Version(EventThirdPartySyncUpdated) != 10 || !checkpoint.HasApplied(contiguous.EventID) || !checkpoint.HasApplied(gap.EventID) {
		t.Fatalf("checkpoint after v1 restore and gap = (%#v, %t, %v)", checkpoint, found, err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := json.Unmarshal(document["version"], &version); err != nil || version != 1 || len(document) != 4 {
		t.Fatalf("durable document schema changed: version=%d fields=%v err=%v", version, document, err)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(document["entries"], &entries); err != nil || len(entries) != 1 || len(entries[0]) != 5 || entries[0]["versions"] == nil {
		t.Fatalf("durable v1 entry shape changed: entries=%v err=%v", entries, err)
	}
}

func TestEventSyncApplicationSkipsValidSettlementPayloadBelowV1Watermark(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	provider := testProfilePathProvider(t.TempDir())
	writeHandcraftedEventStateV1(t, provider)
	state := NewDurableEventStateRepository(provider)
	deps := newEventSyncDependencies(scope)
	deps.State = state
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := eventSyncEvent("balance-7", EventSettlementCompleted, scope.CloudUserID, 7, `{"task_id":"duplicate-task","settlement_id":"duplicate-settlement"}`)
	backward := eventSyncEvent("legacy-settlement-6", EventSettlementCompleted, scope.CloudUserID, 6, `{"task_id":"backward-task","settlement_id":"backward-settlement"}`)
	next := eventSyncEvent("payment-8", EventPaymentUpdated, scope.CloudUserID, 8, `{}`)

	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(duplicate, backward, next), 0); err != nil || reconnect {
		t.Fatalf("consume legacy payloads = (%t, %v)", reconnect, err)
	}
	if got := deps.calls(); got != (eventSyncCalls{payment: 1}) {
		t.Fatalf("authority calls = %+v", got)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != next.EventID || checkpoint.HasApplied(backward.EventID) {
		t.Fatalf("checkpoint = (%+v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationStopsGapEventWhenBindingGenerationChangesDuringRecovery(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	state := NewMemoryEventStateRepository()
	mustCommitEvent(t, state, scope, eventSyncEvent("balance-1", EventBalanceUpdated, scope.CloudUserID, 1, `{}`))

	trace := make([]string, 0, 8)
	deps := newEventSyncDependencies(scope)
	deps.State = &eventSyncTracingState{EventStateRepository: state, trace: &trace}
	attachEventSyncTrace(deps, &trace)
	deps.Balance.(*eventSyncBalance).after = func() {
		deps.Bindings.(*eventSyncResolver).scope.BindingGeneration++
	}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}

	gap := eventSyncEvent("sync-3", EventThirdPartySyncUpdated, scope.CloudUserID, 3, `{"status":"succeeded","version":3}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(gap), 0); reconnect || !errors.Is(err, ErrEventBindingChanged) {
		t.Fatalf("consume gap after generation change = (%t, %v), want binding change", reconnect, err)
	}
	wantTrace := []string{"tasks", "payment", "balance", "subscription"}
	if !reflect.DeepEqual(trace, wantTrace) {
		t.Errorf("generation-change trace = %v, want recovery only before stale dispatch %v", trace, wantTrace)
	}
	checkpoint, found, err := state.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != "balance-1" || checkpoint.HasApplied(gap.EventID) {
		t.Fatalf("old-generation checkpoint changed = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationStopsOnMalformedEnvelopeBeforeFollowingEvent(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	validEnvelope := eventSyncEvent("malformed", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	for _, tc := range []struct {
		name   string
		mutate func(*Event)
	}{
		{"empty event type", func(event *Event) { event.EventType = "" }},
		{"empty event id", func(event *Event) { event.EventID = "" }},
		{"empty cloud user", func(event *Event) { event.CloudUserID = "" }},
		{"zero version", func(event *Event) { event.Version = 0 }},
		{"zero occurred at", func(event *Event) { event.OccurredAt = time.Time{} }},
		{"malformed data", func(event *Event) { event.Data = []byte(`{`) }},
		{"empty data", func(event *Event) { event.Data = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			malformed := validEnvelope
			tc.mutate(&malformed)
			valid := eventSyncEvent("valid", EventSubscriptionUpdated, scope.CloudUserID, 1, `{}`)
			if _, err := app.consume(ctx, eventSyncActor(), scope, eventStream(malformed, valid), 0); err == nil {
				t.Fatal("consume accepted a malformed event envelope")
			}
			if got := deps.calls(); got != (eventSyncCalls{}) {
				t.Fatalf("authority calls = %#v, want none", got)
			}
			if checkpoint, found, err := deps.State.Load(ctx, scope); err != nil || found || checkpoint.Cursor != "" {
				t.Fatalf("checkpoint = (%#v, %t, %v), want absent", checkpoint, found, err)
			}
		})
	}
}

func TestEventSyncApplicationRejectsForeignBindingAndUnknownTypeWithoutRecovery(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	for _, event := range []Event{
		eventSyncEvent("foreign", EventSettlementCompleted, "other-cloud-user", 1, `{"task_id":"foreign-task","settlement_id":"foreign-settlement"}`),
		eventSyncEvent("foreign-missing-task", EventSettlementCompleted, "other-cloud-user", 1, `{"settlement_id":"foreign-settlement"}`),
		eventSyncEvent("unknown", EventType("unknown"), scope.CloudUserID, 1, `{}`),
	} {
		deps := newEventSyncDependencies(scope)
		app, err := NewEventSyncApplication(deps)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := app.consume(ctx, eventSyncActor(), scope, eventStream(event), 0); err == nil {
			t.Fatalf("consume accepted event %#v", event)
		}
		if got := deps.calls(); got != (eventSyncCalls{}) {
			t.Fatalf("invalid event authority calls = %#v", got)
		}
		if checkpoint, found, err := deps.State.Load(ctx, scope); err != nil || found || checkpoint.Cursor != "" {
			t.Fatalf("invalid event checkpoint = (%#v, %t, %v)", checkpoint, found, err)
		}
	}
}

func TestEventSyncApplicationStopsOnContextCancellationBeforeFollowingEvent(t *testing.T) {
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	first := eventSyncEvent("first", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	valid := eventSyncEvent("valid", EventSubscriptionUpdated, scope.CloudUserID, 1, `{}`)
	if _, err := app.consume(ctx, eventSyncActor(), scope, eventStream(first, valid), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("consume canceled = %v", err)
	}
	if got := deps.calls(); got != (eventSyncCalls{}) {
		t.Fatalf("authority calls = %#v, want none", got)
	}
	if checkpoint, found, err := deps.State.Load(context.Background(), scope); err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint after cancellation = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationRechecksCurrentBindingBeforeActionAndCommit(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	for _, tc := range []struct {
		name         string
		changeBefore bool
		changedScope BindingScope
		wantCalls    int
	}{
		{"profile", true, BindingScope{ProfileID: "other-profile", CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration}, 0},
		{"cloud user", true, BindingScope{ProfileID: scope.ProfileID, CloudUserID: "other-user", BindingGeneration: scope.BindingGeneration}, 0},
		{"generation", true, BindingScope{ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration + 1}, 0},
		{"changed during refresh", false, BindingScope{ProfileID: scope.ProfileID, CloudUserID: "other-user", BindingGeneration: 2}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			if tc.changeBefore {
				deps.Bindings.(*eventSyncResolver).scopes = []BindingScope{scope, tc.changedScope}
			} else {
				deps.Balance.(*eventSyncBalance).after = func() { deps.Bindings.(*eventSyncResolver).scope = tc.changedScope }
			}
			event := eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
			app := newEventSyncApplication(t, deps, event)
			if err := app.Run(ctx, eventSyncActor()); err == nil {
				t.Fatal("Run accepted a changed binding")
			}
			if got := deps.Balance.(*eventSyncBalance).calls; got != tc.wantCalls {
				t.Fatalf("balance calls = %d, want %d", got, tc.wantCalls)
			}
			checkpoint, found, err := deps.State.Load(ctx, scope)
			if err != nil || found || checkpoint.Cursor != "" {
				t.Fatalf("checkpoint = %#v, %t, %v; want absent", checkpoint, found, err)
			}
		})
	}
}

func TestEventSyncApplicationReconnectsOnlyForTheCurrentReboundScope(t *testing.T) {
	ctx := context.Background()
	oldScope := eventSyncScope()
	newScope := BindingScope{ProfileID: oldScope.ProfileID, CloudUserID: "new-cloud", BindingGeneration: oldScope.BindingGeneration + 1}
	deps := newEventSyncDependencies(oldScope)
	deps.MaxReconnects = 1
	deps.Bindings.(*eventSyncResolver).scopes = []BindingScope{oldScope, newScope}
	deps.Source.(*eventSyncSource).streams = []<-chan Event{
		eventStream(eventSyncEvent("old", EventBalanceUpdated, oldScope.CloudUserID, 1, `{}`)),
		eventStream(eventSyncEvent("new", EventBalanceUpdated, newScope.CloudUserID, 1, `{}`)),
	}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	source := deps.Source.(*eventSyncSource)
	if source.calls != 2 || source.scopes[0] != oldScope || source.scopes[1] != newScope {
		t.Fatalf("Connect scopes = %#v", source.scopes)
	}
	oldCheckpoint, oldFound, err := deps.State.Load(ctx, oldScope)
	if err != nil || oldFound || oldCheckpoint.Cursor != "" {
		t.Fatalf("old checkpoint = %#v, %t, %v; want absent", oldCheckpoint, oldFound, err)
	}
	newCheckpoint, newFound, err := deps.State.Load(ctx, newScope)
	if err != nil || !newFound || newCheckpoint.Cursor != "new" {
		t.Fatalf("new checkpoint = %#v, %t, %v", newCheckpoint, newFound, err)
	}
}

func TestEventSyncApplicationDoesNotAdvanceWhenAuthorityFails(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	deps.Balance.(*eventSyncBalance).err = errors.New("refresh failed")
	app := newEventSyncApplication(t, deps, eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`))
	if err := app.Run(ctx, eventSyncActor()); err == nil {
		t.Fatal("Run accepted failed refresh")
	}
	checkpoint, found, err := deps.State.Load(ctx, scope)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint = %#v, %t, %v; want absent", checkpoint, found, err)
	}
	deps.Balance.(*eventSyncBalance).err = nil
	deps.Source.(*eventSyncSource).streams = []<-chan Event{eventStream(eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`))}
	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("retry Run: %v", err)
	}
	checkpoint, found, err = deps.State.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != "balance" {
		t.Fatalf("retry checkpoint = %#v, %t, %v", checkpoint, found, err)
	}
}

func TestEventSyncApplicationReconnectsAndReplaysRetryableAuthorityFailure(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	provider := testProfilePathProvider(t.TempDir())
	retryable := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	failed := eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	following := eventSyncEvent("subscription", EventSubscriptionUpdated, scope.CloudUserID, 2, `{}`)
	deps := newEventSyncDependencies(scope)
	deps.State = NewDurableEventStateRepository(provider)
	deps.MaxReconnects = 1
	deps.Balance.(*eventSyncBalance).errs = []error{retryable, nil}
	deps.Balance.(*eventSyncBalance).after = func() {
		if deps.Balance.(*eventSyncBalance).calls > 2 {
			return
		}
		checkpoint, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope)
		if err != nil || found || checkpoint.Cursor != "" {
			t.Fatalf("failed event was overtaken before replay: (%#v, %t, %v)", checkpoint, found, err)
		}
	}
	deps.Source.(*eventSyncSource).streams = []<-chan Event{
		eventStream(failed, following),
		eventStream(failed, following),
	}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("Run retryable authority failure: %v", err)
	}
	if source := deps.Source.(*eventSyncSource); source.calls != 2 {
		t.Fatalf("Connect calls = %d, want replay connection", source.calls)
	}
	if got := deps.Subscription.(*eventSyncSubscription).calls; got != 2 {
		t.Fatalf("subscription calls = %d, want one event action plus final recovery", got)
	}
	checkpoint, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != following.EventID || !checkpoint.HasApplied(failed.EventID) || !checkpoint.HasApplied(following.EventID) {
		t.Fatalf("checkpoint after replay = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationReturnsRetryableFailureWhenReconnectBudgetExhausted(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	retryable := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	event := eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	deps := newEventSyncDependencies(scope)
	deps.MaxReconnects = 1
	deps.Balance.(*eventSyncBalance).errs = []error{retryable, retryable}
	deps.Source.(*eventSyncSource).streams = []<-chan Event{eventStream(event), eventStream(event)}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); !errors.Is(err, retryable) {
		t.Fatalf("Run exhausted retry = %v, want retryable authority error", err)
	}
	if source := deps.Source.(*eventSyncSource); source.calls != 2 {
		t.Fatalf("Connect calls = %d, want bounded retry", source.calls)
	}
	if checkpoint, found, err := deps.State.Load(ctx, scope); err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint after exhausted retry = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationRejectsUnknownEventWithoutRecoveryOrReconnect(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	unknown := eventSyncEvent("unknown", EventType("future.event"), scope.CloudUserID, 1, `{}`)
	following := eventSyncEvent("subscription", EventSubscriptionUpdated, scope.CloudUserID, 1, `{}`)
	deps := newEventSyncDependencies(scope)
	deps.MaxReconnects = 1
	deps.Source.(*eventSyncSource).streams = []<-chan Event{eventStream(unknown, following)}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); err == nil {
		t.Fatal("Run accepted an unknown normalized event")
	}
	if source := deps.Source.(*eventSyncSource); source.calls != 1 {
		t.Fatalf("Connect calls = %d, want no contract-error reconnect", source.calls)
	}
	checkpoint, found, err := deps.State.Load(ctx, scope)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint after unknown event = (%#v, %t, %v)", checkpoint, found, err)
	}
	if got := deps.calls(); got != (eventSyncCalls{}) {
		t.Fatalf("unknown event authority calls = %#v", got)
	}
}

func TestEventSyncApplicationDoesNotRetryUnknownOrNonRetryableAuthorityErrors(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"plain error", errors.New("local failure")},
		{"non-retryable cloud error", &Error{Code: ErrorProfileScopeMismatch, Message: "mismatch", Retryable: false}},
		{"canceled retryable operation", errors.Join(context.Canceled, &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			deps.MaxReconnects = 2
			deps.Balance.(*eventSyncBalance).err = tc.err
			deps.Source.(*eventSyncSource).streams = []<-chan Event{
				eventStream(eventSyncEvent("failed", EventBalanceUpdated, scope.CloudUserID, 1, `{}`), eventSyncEvent("following", EventSubscriptionUpdated, scope.CloudUserID, 1, `{}`)),
			}
			app, err := NewEventSyncApplication(deps)
			if err != nil {
				t.Fatal(err)
			}
			if err := app.Run(ctx, eventSyncActor()); !errors.Is(err, tc.err) {
				t.Fatalf("Run = %v, want %v", err, tc.err)
			}
			if source := deps.Source.(*eventSyncSource); source.calls != 1 {
				t.Fatalf("Connect calls = %d, want no retry", source.calls)
			}
			if deps.Subscription.(*eventSyncSubscription).calls != 0 {
				t.Fatal("following event ran after terminal authority error")
			}
		})
	}
}

func TestEventSyncApplicationReconnectsCurrentScopeAfterRebindDuringAction(t *testing.T) {
	ctx := context.Background()
	oldScope := eventSyncScope()
	newScope := BindingScope{ProfileID: oldScope.ProfileID, CloudUserID: "rebound", BindingGeneration: oldScope.BindingGeneration + 1}
	deps := newEventSyncDependencies(oldScope)
	deps.MaxReconnects = 1
	deps.Balance.(*eventSyncBalance).after = func() { deps.Bindings.(*eventSyncResolver).scope = newScope }
	deps.Source.(*eventSyncSource).streams = []<-chan Event{
		eventStream(eventSyncEvent("old", EventBalanceUpdated, oldScope.CloudUserID, 1, `{}`)),
		eventStream(eventSyncEvent("new", EventSubscriptionUpdated, newScope.CloudUserID, 1, `{}`)),
	}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("Run rebind: %v", err)
	}
	source := deps.Source.(*eventSyncSource)
	if source.calls != 2 || source.scopes[0] != oldScope || source.scopes[1] != newScope {
		t.Fatalf("Connect scopes = %#v", source.scopes)
	}
	if checkpoint, found, err := deps.State.Load(ctx, oldScope); err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("old checkpoint = (%#v, %t, %v)", checkpoint, found, err)
	}
	checkpoint, found, err := deps.State.Load(ctx, newScope)
	if err != nil || !found || checkpoint.Cursor != "new" {
		t.Fatalf("new checkpoint = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationReconnectsFinitelyAndRecoversLocalIDs(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	deps.Source.(*eventSyncSource).streams = []<-chan Event{closedEventStream(), closedEventStream()}
	deps.MaxReconnects = 1
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if source := deps.Source.(*eventSyncSource); source.calls != 2 || len(source.scopes) != 2 || source.scopes[0] != scope || source.scopes[1] != scope {
		t.Fatalf("Connect calls = %d, scopes = %#v", source.calls, source.scopes)
	}
	if got := deps.calls(); got != (eventSyncCalls{tasks: 2, payment: 2, balance: 2, subscription: 2}) {
		t.Fatalf("disconnect recovery calls = %#v", got)
	}
}

func TestEventSyncApplicationConnectionFailureRecoversLocalIDsWithoutCheckpoint(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	networkErr := &Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}
	deps.Source.(*eventSyncSource).errs = []error{networkErr}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); !errors.Is(err, networkErr) {
		t.Fatalf("Run = %v, want network error", err)
	}
	if got := deps.calls(); got != (eventSyncCalls{tasks: 1, payment: 1, balance: 1, subscription: 1}) {
		t.Fatalf("connection recovery calls = %#v", got)
	}
	checkpoint, found, err := deps.State.Load(ctx, scope)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint = %#v, %t, %v; want absent", checkpoint, found, err)
	}
}

func TestEventSyncApplicationRetriesOneConnectionFailureThenReturns(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	deps := newEventSyncDependencies(scope)
	deps.MaxReconnects = 1
	deps.Source.(*eventSyncSource).errs = []error{&Error{Code: ErrorNetworkUnavailable, Message: "offline", Retryable: true}}
	deps.Source.(*eventSyncSource).streams = []<-chan Event{closedEventStream()}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if source := deps.Source.(*eventSyncSource); source.calls != 2 || source.scopes[0] != scope || source.scopes[1] != scope {
		t.Fatalf("Connect calls = %d, scopes = %#v", source.calls, source.scopes)
	}
	if got := deps.calls(); got != (eventSyncCalls{tasks: 2, payment: 2, balance: 2, subscription: 2}) {
		t.Fatalf("recovery calls = %#v", got)
	}
}

func TestEventSyncApplicationReusesStateAcrossRestart(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	event := eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	state := NewMemoryEventStateRepository()
	first := newEventSyncDependencies(scope)
	first.State = state
	if err := newEventSyncApplication(t, first, event).Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	second := newEventSyncDependencies(scope)
	second.State = state
	app, err := NewEventSyncApplication(second)
	if err != nil {
		t.Fatal(err)
	}
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(event), 0); err != nil || reconnect {
		t.Fatalf("second consume = (%t, %v), want safely skipped duplicate", reconnect, err)
	}
	if second.Balance.(*eventSyncBalance).calls != 0 {
		t.Fatal("duplicate event invoked an authority after restart")
	}
}

func TestEventSyncApplicationRestoresDurableCheckpointAcrossRestart(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := eventSyncScope()
	event := eventSyncEvent("balance", EventBalanceUpdated, scope.CloudUserID, 2, `{}`)
	first := newEventSyncDependencies(scope)
	first.State = NewDurableEventStateRepository(provider)
	if err := newEventSyncApplication(t, first, event).Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	second := newEventSyncDependencies(scope)
	second.State = NewDurableEventStateRepository(provider)
	app, err := NewEventSyncApplication(second)
	if err != nil {
		t.Fatal(err)
	}
	old := eventSyncEvent("old-balance", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	next := eventSyncEvent("subscription", EventSubscriptionUpdated, scope.CloudUserID, 3, `{}`)
	if reconnect, err := app.consume(ctx, eventSyncActor(), scope, eventStream(event, old, next), 0); err != nil || reconnect {
		t.Fatalf("second consume = (%t, %v), want duplicate and old version skipped before valid event", reconnect, err)
	}
	if second.Balance.(*eventSyncBalance).calls != 0 {
		t.Fatal("durable duplicate or old version invoked authority after restart")
	}
	checkpoint, found, err := second.State.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != next.EventID || !checkpoint.HasApplied("subscription") || checkpoint.HasApplied(old.EventID) {
		t.Fatalf("durable checkpoint after duplicate = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationDoesNotPersistFailedDurableAuthorityAction(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := eventSyncScope()
	absent := newEventSyncDependencies(scope)
	absent.State = NewDurableEventStateRepository(provider)
	absent.Tasks.(*eventSyncTasks).exactErr = errors.New("recovery failed")
	failedRecovery := eventSyncEvent("freeze-failed", EventFreezeCreated, scope.CloudUserID, 1, `{"task_id":"failed-task","freeze_id":"failed-freeze"}`)
	if err := newEventSyncApplication(t, absent, failedRecovery).Run(ctx, eventSyncActor()); err == nil {
		t.Fatal("Run accepted failed durable recovery action")
	}
	if checkpoint, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope); err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("checkpoint after failed recovery = (%#v, %t, %v), want absent", checkpoint, found, err)
	}

	state := NewDurableEventStateRepository(provider)
	old := eventSyncEvent("old", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	mustCommitEvent(t, state, scope, old)

	failed := newEventSyncDependencies(scope)
	failed.State = NewDurableEventStateRepository(provider)
	failed.Balance.(*eventSyncBalance).err = errors.New("refresh failed")
	next := eventSyncEvent("failed", EventBalanceUpdated, scope.CloudUserID, 2, `{}`)
	if err := newEventSyncApplication(t, failed, next).Run(ctx, eventSyncActor()); err == nil {
		t.Fatal("Run accepted failed durable authority action")
	}
	checkpoint, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != old.EventID || checkpoint.Version(EventBalanceUpdated) != old.Version || checkpoint.HasApplied(next.EventID) {
		t.Fatalf("checkpoint after failed authority = (%#v, %t, %v)", checkpoint, found, err)
	}

	retry := newEventSyncDependencies(scope)
	retry.State = NewDurableEventStateRepository(provider)
	if err := newEventSyncApplication(t, retry, next).Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("retry Run: %v", err)
	}
	checkpoint, found, err = NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != next.EventID || checkpoint.Version(EventBalanceUpdated) != next.Version || !checkpoint.HasApplied(old.EventID) || !checkpoint.HasApplied(next.EventID) {
		t.Fatalf("checkpoint after retry = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationDoesNotPersistDurableCheckpointAfterBindingChanges(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := eventSyncScope()
	old := eventSyncEvent("old", EventBalanceUpdated, scope.CloudUserID, 1, `{}`)
	mustCommitEvent(t, NewDurableEventStateRepository(provider), scope, old)
	next := eventSyncEvent("changed", EventBalanceUpdated, scope.CloudUserID, 2, `{}`)

	changed := newEventSyncDependencies(scope)
	changed.State = NewDurableEventStateRepository(provider)
	changed.Balance.(*eventSyncBalance).after = func() {
		changed.Bindings.(*eventSyncResolver).scope.BindingGeneration++
	}
	if err := newEventSyncApplication(t, changed, next).Run(ctx, eventSyncActor()); !errors.Is(err, ErrEventBindingChanged) {
		t.Fatalf("Run after binding change = %v", err)
	}
	checkpoint, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != old.EventID || checkpoint.Version(EventBalanceUpdated) != old.Version || checkpoint.HasApplied(next.EventID) {
		t.Fatalf("checkpoint after binding change = (%#v, %t, %v)", checkpoint, found, err)
	}

	retry := newEventSyncDependencies(scope)
	retry.State = NewDurableEventStateRepository(provider)
	if err := newEventSyncApplication(t, retry, next).Run(ctx, eventSyncActor()); err != nil {
		t.Fatalf("retry Run: %v", err)
	}
	checkpoint, found, err = NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != next.EventID || checkpoint.Version(EventBalanceUpdated) != next.Version || !checkpoint.HasApplied(old.EventID) || !checkpoint.HasApplied(next.EventID) {
		t.Fatalf("checkpoint after safe retry = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestEventSyncApplicationStrictlyDecodesThirdPartySync(t *testing.T) {
	ctx := context.Background()
	scope := eventSyncScope()
	for _, tc := range []struct {
		name string
		data string
		want bool
	}{
		{"valid", `{"status":"succeeded","version":3}`, true},
		{"missing status", `{"version":3}`, false},
		{"unknown field", `{"status":"succeeded","version":3,"unexpected":true}`, false},
		{"scope injection rejected", `{"status":"succeeded","version":3,"scope":{"profile_id":"p","cloud_user_id":"u","binding_generation":1}}`, false},
		{"invalid status", `{"status":"invalid","version":3}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := newEventSyncDependencies(scope)
			event := eventSyncEvent("sync", EventThirdPartySyncUpdated, scope.CloudUserID, 1, tc.data)
			err := newEventSyncApplication(t, deps, event).Run(ctx, eventSyncActor())
			if tc.want && err != nil {
				t.Fatalf("Run: %v", err)
			}
			if !tc.want && err == nil {
				t.Fatal("Run accepted invalid third-party payload")
			}
			sync := deps.Sync.(*eventSyncApplier)
			if (sync.calls == 1) != tc.want {
				t.Fatalf("ApplyThirdPartySync calls = %d", sync.calls)
			}
			if tc.want && (sync.state.Scope != scope || !sync.state.UpdatedAt.Equal(event.OccurredAt) || sync.state.Version != 3 || sync.state.Status != ThirdPartySyncSucceeded) {
				t.Fatalf("sync state = %#v", sync.state)
			}
		})
	}
}

func newEventSyncApplication(t *testing.T, deps EventSyncApplicationDependencies, events ...Event) *EventSyncApplication {
	t.Helper()
	deps.Source.(*eventSyncSource).streams = []<-chan Event{eventStream(events...)}
	app, err := NewEventSyncApplication(deps)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func newEventSyncDependencies(scope BindingScope) EventSyncApplicationDependencies {
	resolver := &eventSyncResolver{scope: scope}
	return EventSyncApplicationDependencies{
		Bindings: resolver, Source: &eventSyncSource{}, State: NewMemoryEventStateRepository(),
		Balance: &eventSyncBalance{}, Tasks: &eventSyncTasks{}, Settlements: &eventSyncSettlementReconciler{}, Subscription: &eventSyncSubscription{}, Payment: &eventSyncPayment{}, Sync: &eventSyncApplier{},
	}
}

func eventSyncActor() userctx.ActorContext {
	return userctx.ActorContext{ActorID: "actor", ProfileID: "profile"}
}

func eventSyncScope() BindingScope {
	return BindingScope{ProfileID: "profile", CloudUserID: "cloud", BindingGeneration: 1}
}

func eventSyncEvent(id string, kind EventType, user string, version uint64, data string) Event {
	return Event{EventID: id, EventType: kind, CloudUserID: user, OccurredAt: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC), Version: version, Data: []byte(data)}
}

func mustCommitEvent(t *testing.T, state EventStateRepository, scope BindingScope, event Event) {
	t.Helper()
	if err := state.CommitApplied(context.Background(), scope, event); err != nil {
		t.Fatal(err)
	}
}

func eventStream(events ...Event) <-chan Event {
	stream := make(chan Event, len(events))
	for _, event := range events {
		stream <- event
	}
	close(stream)
	return stream
}

func closedEventStream() <-chan Event { return eventStream() }

func writeHandcraftedEventStateV1(t *testing.T, provider ProfilePathProvider) string {
	t.Helper()
	path, err := durableCloudFile(provider, userctx.ActorContext{ProfileID: eventSyncScope().ProfileID}, "event_state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile","entries":[{"scope":{"profile_id":"profile","cloud_user_id":"cloud","binding_generation":1},"key":"profile\u0000cloud\u00001","cursor":"balance-7","versions":[{"event_type":"balance.updated","version":7},{"event_type":"subscription.updated","version":3}],"applied_event_ids":["balance-7","subscription-3"]}]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func attachEventSyncTrace(deps EventSyncApplicationDependencies, trace *[]string) {
	deps.Balance.(*eventSyncBalance).trace = trace
	deps.Tasks.(*eventSyncTasks).trace = trace
	deps.Subscription.(*eventSyncSubscription).trace = trace
	deps.Payment.(*eventSyncPayment).trace = trace
	deps.Sync.(*eventSyncApplier).trace = trace
}

func appendEventSyncTrace(trace *[]string, step string) {
	if trace != nil {
		*trace = append(*trace, step)
	}
}

type eventSyncTracingState struct {
	EventStateRepository
	trace *[]string
}

func (s *eventSyncTracingState) CommitApplied(ctx context.Context, scope BindingScope, event Event) error {
	appendEventSyncTrace(s.trace, "checkpoint")
	return s.EventStateRepository.CommitApplied(ctx, scope, event)
}

type eventSyncCommitErrorState struct {
	EventStateRepository
	err error
}

func (s *eventSyncCommitErrorState) CommitApplied(context.Context, BindingScope, Event) error {
	return s.err
}

type eventSyncResolver struct {
	scope  BindingScope
	scopes []BindingScope
}

func (r *eventSyncResolver) ResolveBinding(context.Context, userctx.ActorContext) (ProfileBinding, error) {
	if len(r.scopes) > 0 {
		r.scope = r.scopes[0]
		r.scopes = r.scopes[1:]
	}
	return ProfileBinding{ActorID: "actor", ProfileID: r.scope.ProfileID, CloudUserID: r.scope.CloudUserID, BindingGeneration: r.scope.BindingGeneration, Status: BindingBound}, nil
}

type eventSyncSource struct {
	streams []<-chan Event
	errs    []error
	calls   int
	scopes  []BindingScope
}

func (s *eventSyncSource) Connect(_ context.Context, scope BindingScope) (<-chan Event, error) {
	s.calls++
	s.scopes = append(s.scopes, scope)
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		return nil, err
	}
	if len(s.streams) == 0 {
		return closedEventStream(), nil
	}
	stream := s.streams[0]
	s.streams = s.streams[1:]
	return stream, nil
}

type eventSyncBalance struct {
	calls int
	err   error
	errs  []error
	after func()
	trace *[]string
}

func (a *eventSyncBalance) Refresh(context.Context, userctx.ActorContext) (BalanceView, error) {
	a.calls++
	appendEventSyncTrace(a.trace, "balance")
	if a.after != nil {
		a.after()
	}
	if len(a.errs) > 0 {
		err := a.errs[0]
		a.errs = a.errs[1:]
		return BalanceView{}, err
	}
	return BalanceView{}, a.err
}

type eventSyncTasks struct {
	calls      int
	exactCalls int
	ackCalls   int
	err        error
	exactErr   error
	ackErr     error
	errs       []error
	trace      *[]string
	exactActor userctx.ActorContext
	exactScope BindingScope
	taskID     string
	freezeID   string
	exclusions []TaskRecoveryExclusion
	ackScopes  []BindingScope
}

func (a *eventSyncTasks) RecoverPending(context.Context, userctx.ActorContext) ([]TaskRecoveryDecision, error) {
	a.calls++
	appendEventSyncTrace(a.trace, "tasks")
	if len(a.errs) > 0 {
		err := a.errs[0]
		a.errs = a.errs[1:]
		return nil, err
	}
	return nil, a.err
}

func (a *eventSyncTasks) RecoverPendingForEventSync(ctx context.Context, actor userctx.ActorContext, exclusion TaskRecoveryExclusion) ([]TaskRecoveryDecision, error) {
	a.exclusions = append(a.exclusions, exclusion)
	return a.RecoverPending(ctx, actor)
}

func (a *eventSyncTasks) RecoverExact(_ context.Context, actor userctx.ActorContext, scope BindingScope, taskID, freezeID string) (TaskRecoveryDecision, bool, error) {
	a.exactCalls++
	a.exactActor = actor.Normalize()
	a.exactScope = scope
	a.taskID = taskID
	a.freezeID = freezeID
	appendEventSyncTrace(a.trace, "freeze")
	return TaskRecoveryDecision{}, a.exactErr == nil, a.exactErr
}

func (a *eventSyncTasks) StageEventSync(context.Context, BindingScope, string, []TaskRecoveryDecision) error {
	return nil
}

func (a *eventSyncTasks) AcknowledgeEventSync(_ context.Context, scope BindingScope, _ string) error {
	a.ackCalls++
	a.ackScopes = append(a.ackScopes, scope)
	return a.ackErr
}

type eventSyncSubscription struct {
	calls int
	err   error
	trace *[]string
}

func (a *eventSyncSubscription) Refresh(context.Context, userctx.ActorContext) (SubscriptionView, error) {
	a.calls++
	appendEventSyncTrace(a.trace, "subscription")
	return SubscriptionView{}, a.err
}

type eventSyncPayment struct {
	calls int
	err   error
	trace *[]string
}

func (a *eventSyncPayment) Restore(context.Context, userctx.ActorContext) ([]PaymentRecoveryResult, error) {
	a.calls++
	appendEventSyncTrace(a.trace, "payment")
	return nil, a.err
}

type eventSyncApplier struct {
	calls int
	err   error
	state ThirdPartySync
	trace *[]string
}

func (a *eventSyncApplier) ApplyThirdPartySync(_ context.Context, _ userctx.ActorContext, state ThirdPartySync) error {
	a.calls++
	appendEventSyncTrace(a.trace, "sync")
	a.state = state
	return a.err
}

type eventSyncCalls struct{ balance, tasks, exact, settlements, subscription, payment, sync int }

func (d EventSyncApplicationDependencies) calls() eventSyncCalls {
	return eventSyncCalls{
		balance: d.Balance.(*eventSyncBalance).calls, tasks: d.Tasks.(*eventSyncTasks).calls,
		exact: d.Tasks.(*eventSyncTasks).exactCalls, settlements: d.Settlements.(*eventSyncSettlementReconciler).calls,
		subscription: d.Subscription.(*eventSyncSubscription).calls, payment: d.Payment.(*eventSyncPayment).calls,
		sync: d.Sync.(*eventSyncApplier).calls,
	}
}
