package cloud

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"kwu/internal/config"
	"kwu/internal/userctx"
)

func TestDurableTaskRecoveryStoreRestoresCompleteRecordsAndIsolatesScope(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	scope := durableRecoveryScope(t, actor, "usr-a", 1)
	records := []TaskRecoveryRecord{
		durableRecoveryRecord(t, scope, "task-a", "freeze-a", "key-a", false),
		durableRecoveryRecord(t, scope, "task-b", "freeze-b", "key-b", true),
	}
	store := NewDurableTaskRecoveryStore(provider)
	for _, record := range records {
		if err := store.Save(ctx, record); err != nil {
			t.Fatalf("Save(%q): %v", record.TaskID, err)
		}
	}

	restarted := NewDurableTaskRecoveryStore(provider)
	got, err := restarted.Load(ctx, scope)
	if err != nil || !reflect.DeepEqual(got, records) {
		t.Fatalf("Load = %#v, %v; want %#v", got, err, records)
	}
	for _, other := range []BindingScope{
		durableRecoveryScope(t, userctx.ActorContext{ActorID: "local", ProfileID: "profile-b"}, "usr-a", 1),
		durableRecoveryScope(t, actor, "usr-b", 1),
		durableRecoveryScope(t, actor, "usr-a", 2),
	} {
		got, err := restarted.Load(ctx, other)
		if err != nil || len(got) != 0 {
			t.Fatalf("old records visible to scope %#v: %#v, %v", other, got, err)
		}
	}
	if err := restarted.Delete(ctx, records[0]); err != nil {
		t.Fatal(err)
	}
	got, err = NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || !reflect.DeepEqual(got, records[1:]) {
		t.Fatalf("Load after Delete = %#v, %v; want %#v", got, err, records[1:])
	}
}

func TestDurableTaskRecoveryStoreHonorsCancellationBeforePathIO(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scope := durableRecoveryScope(t, userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}, "usr-a", 1)
	record := durableRecoveryRecord(t, scope, "task-a", "freeze-a", "key-a", false)
	pathCalls := 0
	store := NewDurableTaskRecoveryStore(func(actor userctx.ActorContext) config.ProfilePathResolver {
		pathCalls++
		return testProfilePathProvider(t.TempDir())(actor)
	})
	if err := store.Save(ctx, record); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save error = %v, want context.Canceled", err)
	}
	if _, err := store.Load(ctx, scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("Load error = %v, want context.Canceled", err)
	}
	if err := store.Delete(ctx, record); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete error = %v, want context.Canceled", err)
	}
	if pathCalls != 0 {
		t.Fatalf("path provider calls = %d, want 0 before canceled operations", pathCalls)
	}
}

func TestDurableTaskRecoveryStoreDuplicateSaveOverwritesAndDeleteUsesCompleteKey(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := durableRecoveryScope(t, userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}, "usr-a", 1)
	record := durableRecoveryRecord(t, scope, "task-a", "freeze-a", "key-a", false)
	updated := record
	updated.HasUsage = true
	otherKey := durableRecoveryRecord(t, scope, "task-a", "freeze-a", "key-b", false)
	store := NewDurableTaskRecoveryStore(provider)
	for _, saved := range []TaskRecoveryRecord{record, record, updated, otherKey} {
		if err := store.Save(ctx, saved); err != nil {
			t.Fatal(err)
		}
	}
	got, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || len(got) != 2 || !reflect.DeepEqual(got, []TaskRecoveryRecord{updated, otherKey}) {
		t.Fatalf("Load after duplicate Save = %#v, %v", got, err)
	}
	if err := store.Delete(ctx, otherKey); err != nil {
		t.Fatal(err)
	}
	got, err = NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || !reflect.DeepEqual(got, []TaskRecoveryRecord{updated}) {
		t.Fatalf("Load after exact Delete = %#v, %v", got, err)
	}
}

func TestDurableTaskRecoveryStoreDeleteDoesNotRemoveUpdatedSnapshot(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := durableRecoveryScope(t, userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}, "usr-a", 1)
	original := durableRecoveryRecord(t, scope, "task-updated", "freeze-updated", "key-updated", false)
	updated := original
	updated.HasUsage = true
	store := NewDurableTaskRecoveryStore(provider)
	if err := store.Save(ctx, original); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, updated); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete(ctx, original); err != nil {
		t.Fatal(err)
	}
	records, err := store.Load(ctx, scope)
	if err != nil || len(records) != 1 || records[0] != updated {
		t.Fatalf("records after stale Delete = %+v, %v; want updated snapshot", records, err)
	}
}

func TestDurableTaskRecoveryStoreResolvesSeparateProfileRoots(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	actorA := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	actorB := userctx.ActorContext{ActorID: "local", ProfileID: "profile-b"}
	scopeA := durableRecoveryScope(t, actorA, "usr-a", 1)
	scopeB := durableRecoveryScope(t, actorB, "usr-a", 1)
	store := NewDurableTaskRecoveryStore(provider)
	if err := store.Save(ctx, durableRecoveryRecord(t, scopeA, "task-a", "freeze-a", "key-a", false)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, durableRecoveryRecord(t, scopeB, "task-b", "freeze-b", "key-b", false)); err != nil {
		t.Fatal(err)
	}
	pathA := filepath.Join(provider(actorA).MemoryUserDir(), "cloud", "v1", "task_recovery.json")
	pathB := filepath.Join(provider(actorB).MemoryUserDir(), "cloud", "v1", "task_recovery.json")
	if pathA == pathB {
		t.Fatalf("profile paths are equal: %q", pathA)
	}
	for _, path := range []string{pathA, pathB} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("profile file %q: %v", path, err)
		}
	}
}

func TestDurableTaskRecoveryStoreRejectsTampering(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	path := filepath.Join(provider(actor).MemoryUserDir(), "cloud", "v1", "task_recovery.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	scope := durableRecoveryScope(t, actor, "usr-a", 1)
	validScope := `{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":1}`
	validRecord := `{"scope":` + validScope + `,"task_id":"task-a","freeze_id":"freeze-a","idempotency_key":"key-a","has_usage":true}`
	validEntry := `{"scope":` + validScope + `,"key":"task-a\u0000freeze-a\u0000key-a","record":` + validRecord + `}`
	cases := map[string]string{
		"unknown field":        `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[],"extra":true}`,
		"trailing JSON":        `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[]} {}`,
		"wrong schema":         `{"schema":"wrong","version":1,"profile_id":"profile-a","entries":[]}`,
		"wrong version":        `{"schema":"reasonix.cloud.task_recovery","version":2,"profile_id":"profile-a","entries":[]}`,
		"envelope profile":     `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-b","entries":[]}`,
		"entry profile":        `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-b","cloud_user_id":"usr-a","binding_generation":1},"key":"task-a\u0000freeze-a\u0000key-a","record":` + validRecord + `}]}`,
		"record scope":         `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[{"scope":` + validScope + `,"key":"task-a\u0000freeze-a\u0000key-a","record":{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-b","binding_generation":1},"task_id":"task-a","freeze_id":"freeze-a","idempotency_key":"key-a","has_usage":true}}]}`,
		"entry key":            `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[{"scope":` + validScope + `,"key":"wrong","record":` + validRecord + `}]}`,
		"empty cloud user":     `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-a","cloud_user_id":"","binding_generation":1},"key":"task-a\u0000freeze-a\u0000key-a","record":` + validRecord + `}]}`,
		"zero generation":      `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[{"scope":{"profile_id":"profile-a","cloud_user_id":"usr-a","binding_generation":0},"key":"task-a\u0000freeze-a\u0000key-a","record":` + validRecord + `}]}`,
		"duplicate record key": `{"schema":"reasonix.cloud.task_recovery","version":1,"profile_id":"profile-a","entries":[` + validEntry + `,` + validEntry + `]}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope); err == nil {
				t.Fatal("tampered durable state was treated as missing")
			}
		})
	}
}

func TestDurableTaskRecoveryStoreConcurrentInstancesDoNotLoseRecords(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	const count = 16
	scopes := make([]BindingScope, count)
	records := make([]TaskRecoveryRecord, count)
	for i := 0; i < count; i++ {
		scopes[i] = durableRecoveryScope(t, actor, "usr-a", uint64(i%2+1))
		records[i] = durableRecoveryRecord(t, scopes[i], "task-"+string(rune('a'+i)), "freeze-"+string(rune('a'+i)), "key-"+string(rune('a'+i)), i%2 == 0)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for _, record := range records {
		record := record
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- NewDurableTaskRecoveryStore(provider).Save(ctx, record)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range []BindingScope{scopes[0], scopes[1]} {
		got, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
		if err != nil || len(got) != count/2 {
			t.Fatalf("Load(%#v) = %d records, %v", scope, len(got), err)
		}
	}
}

func TestDurableTaskRecoveryStoreConcurrentSaveDeleteKeepsValidState(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "profile-a"}
	scope := durableRecoveryScope(t, actor, "usr-a", 1)
	provider := testProfilePathProvider(t.TempDir())
	record := durableRecoveryRecord(t, scope, "task-a", "freeze-a", "key-a", true)
	if err := NewDurableTaskRecoveryStore(provider).Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	const count = 20
	var wg sync.WaitGroup
	errCh := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(save bool) {
			defer wg.Done()
			store := NewDurableTaskRecoveryStore(provider)
			if save {
				errCh <- store.Save(ctx, record)
				return
			}
			errCh <- store.Delete(ctx, record)
		}(i%2 == 0)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || len(got) > 1 || len(got) == 1 && got[0] != record {
		t.Fatalf("Load after Save/Delete race = %#v, %v", got, err)
	}
}

func TestTaskRecoveryApplicationRestoresDurableRecordsAndPersistsDeletionMatrix(t *testing.T) {
	ctx := context.Background()
	provider := testProfilePathProvider(t.TempDir())
	scope := recoveryScope(t)
	records := []TaskRecoveryRecord{
		durableRecoveryRecord(t, scope, "task-terminate", "freeze-terminate", "key-terminate", false),
		durableRecoveryRecord(t, scope, "task-active", "freeze-active", "key-active", false),
		durableRecoveryRecord(t, scope, "task-late", "freeze-late", "key-late", true),
		durableRecoveryRecord(t, scope, "task-review", "freeze-review", "key-review", false),
		durableRecoveryRecord(t, scope, "task-settled", "freeze-settled", "key-settled", false),
		durableRecoveryRecord(t, scope, "task-released", "freeze-released", "key-released", false),
		durableRecoveryRecord(t, scope, "task-not-found", "freeze-not-found", "key-not-found", false),
		durableRecoveryRecord(t, scope, "task-offline", "freeze-offline", "key-offline", false),
	}
	store := NewDurableTaskRecoveryStore(provider)
	for _, record := range records {
		if err := store.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	credit := &durableRecoveryCreditClient{
		results: map[string]FreezeResult{
			"freeze-terminate": recoveryNamedFreezeResult(scope, "task-terminate", "freeze-terminate", FreezeExpiredReleased),
			"freeze-active":    recoveryNamedFreezeResult(scope, "task-active", "freeze-active", FreezeActive),
			"freeze-late":      recoveryNamedFreezeResult(scope, "task-late", "freeze-late", FreezeExpiredReleased),
			"freeze-review":    recoveryNamedFreezeResult(scope, "task-review", "freeze-review", FreezeExpiredPendingReview),
			"freeze-settled":   recoveryNamedFreezeResult(scope, "task-settled", "freeze-settled", FreezeSettled),
			"freeze-released":  recoveryNamedFreezeResult(scope, "task-released", "freeze-released", FreezeReleased),
		},
		errors: map[string]error{
			"freeze-not-found": &Error{Code: ErrorFreezeNotFound},
			"freeze-offline":   &Error{Code: ErrorNetworkUnavailable, Retryable: true},
		},
	}
	app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), NewDurableTaskRecoveryStore(provider))
	decisions, err := app.RecoverPending(ctx, recoveryActor())
	if err == nil || len(decisions) != len(records) || credit.getCalls != len(records) {
		t.Fatalf("RecoverPending = %d decisions, %v; calls=%d", len(decisions), err, credit.getCalls)
	}
	wantStatuses := map[string]TaskRecoveryStatus{
		"task-terminate": TaskRecoveryTerminate,
		"task-active":    TaskRecoveryContinueOrFinalize,
		"task-late":      TaskRecoveryRetainForLateFinal,
		"task-review":    TaskRecoveryRetainForManualReview,
		"task-settled":   TaskRecoveryQueryAuthoritativeSettlement,
		"task-released":  TaskRecoveryQueryAuthoritativeSettlement,
		"task-not-found": TaskRecoveryRetainForInvestigation,
		"task-offline":   TaskRecoveryRetry,
	}
	for _, decision := range decisions {
		if decision.Status != wantStatuses[decision.Record.TaskID] {
			t.Fatalf("decision for %q = %q, want %q", decision.Record.TaskID, decision.Status, wantStatuses[decision.Record.TaskID])
		}
	}
	remaining, err := NewDurableTaskRecoveryStore(provider).Load(ctx, scope)
	if err != nil || len(remaining) != len(records)-1 || recoveryContainsTask(remaining, "task-terminate") {
		t.Fatalf("remaining after restart = %#v, %v", remaining, err)
	}
	for _, taskID := range []string{"task-active", "task-late", "task-review", "task-settled", "task-released", "task-not-found", "task-offline"} {
		if !recoveryContainsTask(remaining, taskID) {
			t.Fatalf("retained task %q missing after restart", taskID)
		}
	}
}

func durableRecoveryScope(t *testing.T, actor userctx.ActorContext, cloudUserID string, generation uint64) BindingScope {
	t.Helper()
	scope, err := NewBindingScope(actor, cloudUserID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func durableRecoveryRecord(t *testing.T, scope BindingScope, taskID, freezeID, key string, hasUsage bool) TaskRecoveryRecord {
	t.Helper()
	record := TaskRecoveryRecord{Scope: scope, TaskID: taskID, FreezeID: freezeID, IdempotencyKey: key, HasUsage: hasUsage}
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
	return record
}

type durableRecoveryCreditClient struct {
	results  map[string]FreezeResult
	errors   map[string]error
	getCalls int
}

func (*durableRecoveryCreditClient) CreateFreeze(context.Context, FreezeRequest) (FreezeResult, error) {
	return FreezeResult{}, errors.New("CreateFreeze must not be called by recovery")
}

func (c *durableRecoveryCreditClient) GetFreeze(_ context.Context, _ BindingScope, freezeID string) (FreezeResult, error) {
	c.getCalls++
	return c.results[freezeID], c.errors[freezeID]
}
