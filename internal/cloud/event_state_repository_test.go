package cloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/config"
	"kwu/internal/userctx"
)

func TestMemoryEventStateRepositoryIsolatesFullBindingScope(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewMemoryEventStateRepository()
	base := eventStateScope("profile-a", "user-a", 1)
	for _, scope := range []BindingScope{
		eventStateScope("profile-b", "user-a", 1),
		eventStateScope("profile-a", "user-b", 1),
		eventStateScope("profile-a", "user-a", 2),
	} {
		if err := repository.CommitApplied(ctx, scope, eventStateEvent("event-"+scope.key(), EventBalanceUpdated, scope.CloudUserID, 1)); err != nil {
			t.Fatalf("CommitApplied(%+v): %v", scope, err)
		}
	}

	checkpoint, found, err := repository.Load(ctx, base)
	if err != nil || found || checkpoint.Cursor != "" {
		t.Fatalf("Load(base) = (%+v, %t, %v), want empty and absent", checkpoint, found, err)
	}
}

func TestMemoryEventStateRepositoryPersistsCheckpointForReuse(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewMemoryEventStateRepository()
	scope := eventStateScope("profile-a", "user-a", 1)
	event := eventStateEvent("event-1", EventBalanceUpdated, scope.CloudUserID, 7)
	if err := repository.CommitApplied(ctx, scope, event); err != nil {
		t.Fatalf("CommitApplied: %v", err)
	}

	checkpoint, found, err := repository.Load(ctx, scope)
	if err != nil || !found {
		t.Fatalf("Load = (%+v, %t, %v), want saved checkpoint", checkpoint, found, err)
	}
	if checkpoint.Cursor != event.EventID || checkpoint.Version(EventBalanceUpdated) != event.Version || !checkpoint.HasApplied(event.EventID) {
		t.Fatalf("checkpoint = %+v, want cursor, version, and event id", checkpoint)
	}
}

func TestEventStateRepositoriesClearOnlyExactScope(t *testing.T) {
	ctx := context.Background()
	first := eventStateScope("profile-a", "user-a", 1)
	second := eventStateScope("profile-a", "user-b", 2)
	for name, repository := range map[string]EventStateRepository{
		"memory":  NewMemoryEventStateRepository(),
		"durable": NewDurableEventStateRepository(testProfilePathProvider(t.TempDir())),
	} {
		t.Run(name, func(t *testing.T) {
			if err := repository.CommitApplied(ctx, first, eventStateEvent("first", EventBalanceUpdated, first.CloudUserID, 1)); err != nil {
				t.Fatal(err)
			}
			if err := repository.CommitApplied(ctx, second, eventStateEvent("second", EventBalanceUpdated, second.CloudUserID, 1)); err != nil {
				t.Fatal(err)
			}
			if err := repository.Clear(ctx, first); err != nil {
				t.Fatal(err)
			}
			if _, found, err := repository.Load(ctx, first); err != nil || found {
				t.Fatalf("cleared checkpoint present: found=%v err=%v", found, err)
			}
			if checkpoint, found, err := repository.Load(ctx, second); err != nil || !found || checkpoint.Cursor != "second" {
				t.Fatalf("other checkpoint removed: %#v found=%v err=%v", checkpoint, found, err)
			}
		})
	}
}

func TestMemoryEventStateRepositoryRequiresScopeWideVersionAdvance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewMemoryEventStateRepository()
	scope := eventStateScope("profile-a", "user-a", 1)
	if err := repository.CommitApplied(ctx, scope, eventStateEvent("balance-7", EventBalanceUpdated, scope.CloudUserID, 7)); err != nil {
		t.Fatalf("CommitApplied balance: %v", err)
	}
	subscription := eventStateEvent("subscription-1", EventSubscriptionUpdated, scope.CloudUserID, 1)
	if err := repository.CommitApplied(ctx, scope, subscription); !errors.Is(err, ErrEventCheckpointNotAdvanced) {
		t.Fatalf("CommitApplied stale subscription: %v", err)
	}

	checkpoint, found, err := repository.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != "balance-7" || checkpoint.Version(EventBalanceUpdated) != 7 || checkpoint.Version(EventSubscriptionUpdated) != 0 {
		t.Fatalf("Load after stale cross-type event = (%+v, %t, %v)", checkpoint, found, err)
	}
	newer := eventStateEvent("subscription-8", EventSubscriptionUpdated, scope.CloudUserID, 8)
	if err := repository.CommitApplied(ctx, scope, newer); err != nil {
		t.Fatalf("CommitApplied newer subscription: %v", err)
	}
	checkpoint, found, err = repository.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != newer.EventID || checkpoint.Version(EventBalanceUpdated) != 7 || checkpoint.Version(EventSubscriptionUpdated) != 8 {
		t.Fatalf("Load after newer cross-type event = (%+v, %t, %v)", checkpoint, found, err)
	}
}

func TestMemoryEventStateRepositoryRejectsDuplicateAndNonIncreasingVersionsAtomically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewMemoryEventStateRepository()
	scope := eventStateScope("profile-a", "user-a", 1)
	first := eventStateEvent("event-2", EventBalanceUpdated, scope.CloudUserID, 2)
	if err := repository.CommitApplied(ctx, scope, first); err != nil {
		t.Fatalf("CommitApplied first: %v", err)
	}
	for _, event := range []Event{
		first,
		eventStateEvent("event-2-other", EventBalanceUpdated, scope.CloudUserID, 2),
		eventStateEvent("event-1", EventBalanceUpdated, scope.CloudUserID, 1),
	} {
		if err := repository.CommitApplied(ctx, scope, event); !errors.Is(err, ErrEventCheckpointNotAdvanced) {
			t.Fatalf("CommitApplied(%q, version %d) = %v, want ErrEventCheckpointNotAdvanced", event.EventID, event.Version, err)
		}
	}

	checkpoint, found, err := repository.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != first.EventID || checkpoint.Version(EventBalanceUpdated) != first.Version || !checkpoint.HasApplied(first.EventID) || checkpoint.HasApplied("event-1") || checkpoint.HasApplied("event-2-other") {
		t.Fatalf("Load after rejected commits = (%+v, %t, %v), want unchanged checkpoint", checkpoint, found, err)
	}
}

func TestMemoryEventStateRepositoryRejectsInvalidCheckpointWithoutMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := NewMemoryEventStateRepository()
	scope := eventStateScope("profile-a", "user-a", 1)
	valid := eventStateEvent("event-1", EventBalanceUpdated, scope.CloudUserID, 1)
	if err := repository.CommitApplied(ctx, scope, valid); err != nil {
		t.Fatalf("CommitApplied valid: %v", err)
	}

	invalid := eventStateEvent("event-invalid", EventSubscriptionUpdated, "other-user", 2)
	if err := repository.CommitApplied(ctx, scope, invalid); err == nil {
		t.Fatal("CommitApplied accepted an event for another cloud user")
	}
	checkpoint, found, err := repository.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != valid.EventID || checkpoint.Version(EventSubscriptionUpdated) != 0 || checkpoint.HasApplied(invalid.EventID) {
		t.Fatalf("Load after invalid commit = (%+v, %t, %v), want unchanged checkpoint", checkpoint, found, err)
	}

	if _, _, err := repository.Load(ctx, BindingScope{}); err == nil {
		t.Fatal("Load accepted invalid scope")
	}
}

func eventStateScope(profileID, cloudUserID string, generation uint64) BindingScope {
	return BindingScope{ProfileID: profileID, CloudUserID: cloudUserID, BindingGeneration: generation}
}

func eventStateEvent(eventID string, eventType EventType, cloudUserID string, version uint64) Event {
	return Event{
		EventID:     eventID,
		EventType:   eventType,
		CloudUserID: cloudUserID,
		OccurredAt:  time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC),
		Version:     version,
		Data:        []byte(`{"fixture":true}`),
	}
}

func TestDurableEventStateRepositoryRestoresAndIsolatesCompleteScope(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := eventStateScope("profile-a", "user-a", 1)
	repository := NewDurableEventStateRepository(provider)
	for _, event := range []Event{
		eventStateEvent("balance-7", EventBalanceUpdated, scope.CloudUserID, 7),
		eventStateEvent("subscription-8", EventSubscriptionUpdated, scope.CloudUserID, 8),
	} {
		if err := repository.CommitApplied(ctx, scope, event); err != nil {
			t.Fatal(err)
		}
	}

	restarted := NewDurableEventStateRepository(provider)
	checkpoint, found, err := restarted.Load(ctx, scope)
	if err != nil || !found || checkpoint.Cursor != "subscription-8" || checkpoint.Version(EventBalanceUpdated) != 7 || checkpoint.Version(EventSubscriptionUpdated) != 8 || !checkpoint.HasApplied("balance-7") || !checkpoint.HasApplied("subscription-8") {
		t.Fatalf("Load after restart = (%#v, %t, %v)", checkpoint, found, err)
	}
	for _, other := range []BindingScope{
		eventStateScope("profile-b", "user-a", 1),
		eventStateScope("profile-a", "user-b", 1),
		eventStateScope("profile-a", "user-a", 2),
	} {
		if got, ok, err := restarted.Load(ctx, other); err != nil || ok || got.Cursor != "" {
			t.Fatalf("Load(%#v) = (%#v, %t, %v), want absent", other, got, ok, err)
		}
	}
	checkpoint.versions[EventBalanceUpdated] = 99
	checkpoint.applied["forged"] = struct{}{}
	cloned, found, err := restarted.Load(ctx, scope)
	if err != nil || !found || cloned.Version(EventBalanceUpdated) != 7 || cloned.HasApplied("forged") {
		t.Fatalf("Load returned mutable checkpoint state: (%#v, %t, %v)", cloned, found, err)
	}
}

func TestDurableEventStateRepositoryUsesProfilePathAndHonorsCancellation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	provider := testProfilePathProvider(root)
	scopes := []BindingScope{eventStateScope("profile-a", "user", 1), eventStateScope("profile-b", "user", 1)}
	for i, scope := range scopes {
		if err := NewDurableEventStateRepository(provider).CommitApplied(ctx, scope, eventStateEvent(fmt.Sprintf("event-%d", i), EventBalanceUpdated, scope.CloudUserID, 1)); err != nil {
			t.Fatal(err)
		}
	}
	paths := make(map[string]bool)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == "event_state.json" {
			paths[filepath.Clean(path)] = true
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("event state paths = %v, want one per profile", paths)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	called := false
	repository := NewDurableEventStateRepository(func(userctx.ActorContext) config.ProfilePathResolver {
		called = true
		return config.ProfilePathResolver{}
	})
	if _, _, err := repository.Load(canceled, scopes[0]); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load canceled = %v", err)
	}
	if err := repository.CommitApplied(canceled, scopes[0], eventStateEvent("never", EventBalanceUpdated, scopes[0].CloudUserID, 2)); !errors.Is(err, context.Canceled) {
		t.Fatalf("CommitApplied canceled = %v", err)
	}
	if called {
		t.Fatal("canceled operation resolved a durable path")
	}
}

func TestDurableEventStateRepositoryRejectsWithoutMutation(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := eventStateScope("profile-a", "user-a", 1)
	repository := NewDurableEventStateRepository(provider)
	first := eventStateEvent("first", EventBalanceUpdated, scope.CloudUserID, 3)
	if err := repository.CommitApplied(ctx, scope, first); err != nil {
		t.Fatal(err)
	}
	for _, event := range []Event{
		first,
		eventStateEvent("same", EventBalanceUpdated, scope.CloudUserID, 3),
		eventStateEvent("lower", EventBalanceUpdated, scope.CloudUserID, 2),
		eventStateEvent("other-user", EventSubscriptionUpdated, "other", 1),
		eventStateEvent("unknown", EventType("unknown"), scope.CloudUserID, 1),
	} {
		err := repository.CommitApplied(ctx, scope, event)
		if event.EventID == "first" || event.EventID == "same" || event.EventID == "lower" {
			if !errors.Is(err, ErrEventCheckpointNotAdvanced) {
				t.Fatalf("CommitApplied(%q) = %v", event.EventID, err)
			}
		} else if err == nil {
			t.Fatalf("CommitApplied(%q) accepted invalid event", event.EventID)
		}
	}
	got, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || got.Cursor != first.EventID || got.Version(EventBalanceUpdated) != first.Version || got.HasApplied("same") || got.HasApplied("lower") {
		t.Fatalf("checkpoint after rejects = (%#v, %t, %v)", got, found, err)
	}
}

func TestDurableEventStateRepositoryConcurrentInstancesRemainAtomic(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scopes := make([]BindingScope, 12)
	for i := range scopes {
		scopes[i] = eventStateScope("profile-a", fmt.Sprintf("user-%02d", i), 1)
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(scopes))
	for i, scope := range scopes {
		wg.Add(1)
		go func(i int, scope BindingScope) {
			defer wg.Done()
			errs <- NewDurableEventStateRepository(provider).CommitApplied(ctx, scope, eventStateEvent(fmt.Sprintf("scope-%02d", i), EventBalanceUpdated, scope.CloudUserID, 1))
		}(i, scope)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range scopes {
		if _, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope); err != nil || !found {
			t.Fatalf("scope %#v lost: found=%t err=%v", scope, found, err)
		}
	}

	scope := eventStateScope("profile-a", "shared", 1)
	events := make([]Event, 20)
	for i := range events {
		events[i] = eventStateEvent(fmt.Sprintf("version-%02d", i+1), EventBalanceUpdated, scope.CloudUserID, uint64(i+1))
	}
	errs = make(chan error, len(events))
	for _, event := range events {
		wg.Add(1)
		go func(event Event) {
			defer wg.Done()
			errs <- NewDurableEventStateRepository(provider).CommitApplied(ctx, scope, event)
		}(event)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrEventCheckpointNotAdvanced) {
			t.Fatal(err)
		}
	}
	checkpoint, found, err := NewDurableEventStateRepository(provider).Load(ctx, scope)
	if err != nil || !found || checkpoint.Version(EventBalanceUpdated) != uint64(len(events)) || checkpoint.Cursor != events[len(events)-1].EventID {
		t.Fatalf("concurrent checkpoint = (%#v, %t, %v)", checkpoint, found, err)
	}

	typeScope := eventStateScope("profile-a", "types", 1)
	typeEvents := []Event{
		eventStateEvent("balance", EventBalanceUpdated, typeScope.CloudUserID, 4),
		eventStateEvent("subscription", EventSubscriptionUpdated, typeScope.CloudUserID, 6),
	}
	errs = make(chan error, len(typeEvents))
	for _, event := range typeEvents {
		wg.Add(1)
		go func(event Event) {
			defer wg.Done()
			errs <- NewDurableEventStateRepository(provider).CommitApplied(ctx, typeScope, event)
		}(event)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrEventCheckpointNotAdvanced) {
			t.Fatal(err)
		}
	}
	checkpoint, found, err = NewDurableEventStateRepository(provider).Load(ctx, typeScope)
	balanceVersion := checkpoint.Version(EventBalanceUpdated)
	if err != nil || !found || checkpoint.Cursor != "subscription" || checkpoint.Version(EventSubscriptionUpdated) != 6 || (balanceVersion != 0 && balanceVersion != 4) {
		t.Fatalf("concurrent event types = (%#v, %t, %v)", checkpoint, found, err)
	}
}

func TestDurableEventStateRepositoryRejectsTampering(t *testing.T) {
	scope := eventStateScope("profile-a", "user-a", 1)
	validEntry := `{"scope":{"profile_id":"profile-a","cloud_user_id":"user-a","binding_generation":1},"key":"profile-a\u0000user-a\u00001","cursor":"event-1","versions":[{"event_type":"balance.updated","version":1}],"applied_event_ids":["event-1"]}`
	tests := map[string]string{
		"unknown field":      `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[],"extra":true}`,
		"trailing":           `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[]} {}`,
		"schema":             `{"schema":"wrong","version":1,"profile_id":"profile-a","entries":[]}`,
		"version":            `{"schema":"reasonix.cloud.event_state","version":2,"profile_id":"profile-a","entries":[]}`,
		"envelope profile":   `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"other","entries":[]}`,
		"entry profile":      `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"profile_id":"profile-a"`, `"profile_id":"other"`, 1) + `]}`,
		"entry key":          `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"key":"profile-a\u0000user-a\u00001"`, `"key":"wrong"`, 1) + `]}`,
		"empty user":         `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"cloud_user_id":"user-a"`, `"cloud_user_id":""`, 1) + `]}`,
		"zero generation":    `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"binding_generation":1`, `"binding_generation":0`, 1) + `]}`,
		"unknown type":       `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"event_type":"balance.updated"`, `"event_type":"unknown"`, 1) + `]}`,
		"zero version":       `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"version":1`, `"version":0`, 1) + `]}`,
		"empty cursor":       `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"cursor":"event-1"`, `"cursor":""`, 1) + `]}`,
		"empty event id":     `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"applied_event_ids":["event-1"]`, `"applied_event_ids":[""]`, 1) + `]}`,
		"cursor not applied": `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"applied_event_ids":["event-1"]`, `"applied_event_ids":["other"]`, 1) + `]}`,
		"duplicate type":     `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `]`, `,{"event_type":"balance.updated","version":2}]`, 1) + `]}`,
		"duplicate event id": `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + strings.Replace(validEntry, `"applied_event_ids":["event-1"]`, `"applied_event_ids":["event-1","event-1"]`, 1) + `]}`,
		"duplicate scope":    `{"schema":"reasonix.cloud.event_state","version":1,"profile_id":"profile-a","entries":[` + validEntry + `,` + validEntry + `]}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			provider := testProfilePathProvider(t.TempDir())
			path, err := durableCloudFile(provider, userctx.ActorContext{ProfileID: scope.ProfileID}, "event_state.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if checkpoint, found, err := NewDurableEventStateRepository(provider).Load(context.Background(), scope); err == nil || found || checkpoint.Cursor != "" {
				t.Fatalf("Load tampered = (%#v, %t, %v), want safe failure", checkpoint, found, err)
			}
		})
	}
}
