package cloud

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestTaskRecoveryRecordFromTaskStartResultPreservesPendingIdentity(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	result := TaskStartResult{Scope: scope, TaskID: "task-1", FreezeID: "freeze-1", IdempotencyKey: "key-1"}
	record, err := NewTaskRecoveryRecord(result, true)
	if err != nil {
		t.Fatalf("NewTaskRecoveryRecord: %v", err)
	}
	if record.Scope != scope || record.TaskID != result.TaskID || record.FreezeID != result.FreezeID || record.IdempotencyKey != result.IdempotencyKey || !record.HasUsage {
		t.Fatalf("record = %#v", record)
	}
}

func TestMemoryTaskRecoveryStoreDeleteDoesNotRemoveUpdatedSnapshot(t *testing.T) {
	ctx := context.Background()
	scope := recoveryScope(t)
	original := recoveryNamedRecord(t, scope, "task-updated", "freeze-updated", false)
	updated := original
	updated.HasUsage = true
	store := NewMemoryTaskRecoveryStore()
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

func TestTaskRecoveryApplicationUsesSavedFreezeAndReturnsDocumentedDecisions(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	cases := []struct {
		name     string
		status   FreezeStatus
		hasUsage bool
		want     TaskRecoveryStatus
	}{
		{name: "active continues", status: FreezeActive, want: TaskRecoveryContinueOrFinalize},
		{name: "expired released without usage stops", status: FreezeExpiredReleased, want: TaskRecoveryTerminate},
		{name: "expired released with usage retains late final", status: FreezeExpiredReleased, hasUsage: true, want: TaskRecoveryRetainForLateFinal},
		{name: "expired pending review without usage retains", status: FreezeExpiredPendingReview, want: TaskRecoveryRetainForManualReview},
		{name: "expired pending review with usage retains", status: FreezeExpiredPendingReview, hasUsage: true, want: TaskRecoveryRetainForManualReview},
		{name: "settled queries authority", status: FreezeSettled, want: TaskRecoveryQueryAuthoritativeSettlement},
		{name: "released queries authority", status: FreezeReleased, want: TaskRecoveryQueryAuthoritativeSettlement},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			credit := &taskRecoveryCreditClient{result: recoveryFreezeResult(scope, tc.status)}
			app := recoveryApplication(t, scope, credit)
			record := recoveryRecord(t, scope, tc.hasUsage)

			decision, err := app.Recover(context.Background(), recoveryActor(), record)
			if err != nil {
				t.Fatalf("Recover: %v", err)
			}
			if decision.Status != tc.want || decision.Record != record {
				t.Fatalf("decision = %#v, want status %q and retained record", decision, tc.want)
			}
			if credit.getCalls != 1 || credit.createCalls != 0 || credit.scope != scope || credit.freezeID != record.FreezeID {
				t.Fatalf("credit calls = get %d create %d scope %#v freeze %q", credit.getCalls, credit.createCalls, credit.scope, credit.freezeID)
			}
		})
	}
}

func TestTaskRecoveryApplicationConvertsFailedOrCanceledStartResultsToActivePendingRecovery(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	for _, startErr := range []error{errors.New("model start failed"), context.Canceled} {
		t.Run(startErr.Error(), func(t *testing.T) {
			binding := ProfileBinding{ActorID: "actor-1", ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration, Status: BindingBound}
			resolver, err := NewStaticBindingResolver(binding)
			if err != nil {
				t.Fatal(err)
			}
			started, err := NewTaskStartApplication(&taskRecoveryStartCreditClient{result: recoveryFreezeResult(scope, FreezeActive)}, resolver).Start(context.Background(), recoveryActor(), TaskStartInput{
				Binding: binding, Session: Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access"}, Price: TaskPriceGate{Status: TaskPriceFresh},
				TaskID: "task-1", ModelID: "model-1", ClientTaskType: "chat", ClientEstimatedCost: "5",
				Start: func(context.Context, TaskStartAuthorization) error { return startErr },
			})
			if !errors.Is(err, startErr) || started.Started || started.FreezeID != "freeze-1" {
				t.Fatalf("Start = %#v, %v", started, err)
			}
			record, err := NewTaskRecoveryRecord(started, false)
			if err != nil {
				t.Fatalf("NewTaskRecoveryRecord: %v", err)
			}
			credit := &taskRecoveryCreditClient{result: recoveryFreezeResult(scope, FreezeActive)}
			decision, err := recoveryApplication(t, scope, credit).Recover(context.Background(), recoveryActor(), record)
			if err != nil || decision.Status != TaskRecoveryContinueOrFinalize || decision.Record != record {
				t.Fatalf("Recover after %v = %#v, %v", startErr, decision, err)
			}
		})
	}
}

func TestTaskRecoveryApplicationRejectsOldOrDifferentScopeBeforeRemoteQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := recoveryActor()
	bindings := NewBindingRepository()
	oldBinding, err := bindings.ConfirmBinding(ctx, actor, "cloud-1")
	if err != nil {
		t.Fatal(err)
	}
	oldScope, err := oldBinding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bindings.Rebind(ctx, actor, "cloud-2"); err != nil {
		t.Fatal(err)
	}
	credit := &taskRecoveryCreditClient{result: recoveryFreezeResult(oldScope, FreezeActive)}
	app := NewTaskRecoveryApplication(credit, bindings)

	for _, record := range []TaskRecoveryRecord{
		recoveryRecord(t, oldScope, false),
		{Scope: BindingScope{ProfileID: "profile-other", CloudUserID: "cloud-1", BindingGeneration: 1}, TaskID: "task-1", FreezeID: "freeze-1", IdempotencyKey: "key-1"},
	} {
		_, err := app.Recover(ctx, actor, record)
		if !isProfileMismatch(err) {
			t.Fatalf("Recover(%#v) error = %v, want scope mismatch", record.Scope, err)
		}
	}
	if credit.getCalls != 0 || credit.createCalls != 0 {
		t.Fatalf("remote calls = get %d create %d, want 0", credit.getCalls, credit.createCalls)
	}
}

func TestTaskRecoveryApplicationIsolatesEverySavedScopeComponent(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	cases := []struct {
		name  string
		scope BindingScope
	}{
		{name: "profile id", scope: BindingScope{ProfileID: "profile-other", CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration}},
		{name: "cloud user id", scope: BindingScope{ProfileID: scope.ProfileID, CloudUserID: "cloud-other", BindingGeneration: scope.BindingGeneration}},
		{name: "binding generation", scope: BindingScope{ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration + 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			credit := &taskRecoveryCreditClient{result: recoveryFreezeResult(tc.scope, FreezeActive)}
			_, err := recoveryApplication(t, scope, credit).Recover(context.Background(), recoveryActor(), recoveryRecord(t, tc.scope, false))
			if !isProfileMismatch(err) {
				t.Fatalf("Recover error = %v, want scope mismatch", err)
			}
			if credit.getCalls != 0 || credit.createCalls != 0 {
				t.Fatalf("remote calls = get %d create %d, want 0", credit.getCalls, credit.createCalls)
			}
		})
	}
}

func TestTaskRecoveryApplicationRetainsRecordForNotFoundAndOffline(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	record := recoveryRecord(t, scope, true)
	cases := []struct {
		name string
		err  error
		want TaskRecoveryStatus
	}{
		{name: "not found", err: &Error{Code: ErrorFreezeNotFound}, want: TaskRecoveryRetainForInvestigation},
		{name: "offline", err: &Error{Code: ErrorNetworkUnavailable, Retryable: true}, want: TaskRecoveryRetry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			credit := &taskRecoveryCreditClient{err: tc.err}
			decision, err := recoveryApplication(t, scope, credit).Recover(context.Background(), recoveryActor(), record)
			if !errors.Is(err, tc.err) || decision.Status != tc.want || decision.Record != record {
				t.Fatalf("Recover = %#v, %v; want retained %q and original error", decision, err, tc.want)
			}
			if credit.getCalls != 1 || credit.createCalls != 0 {
				t.Fatalf("remote calls = get %d create %d", credit.getCalls, credit.createCalls)
			}
		})
	}
}

func TestTaskRecoveryApplicationRecoversOnlyCurrentScopeFromStoreAfterRestart(t *testing.T) {
	t.Parallel()
	for _, startErr := range []error{errors.New("model start failed"), context.Canceled} {
		t.Run(startErr.Error(), func(t *testing.T) {
			scope := recoveryScope(t)
			store := NewMemoryTaskRecoveryStore()
			failed := recoveryFailedStartResult(t, scope, startErr)
			current, err := NewTaskRecoveryRecord(failed, false)
			if err != nil {
				t.Fatal(err)
			}
			otherScopes := []BindingScope{
				{ProfileID: "profile-other", CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration},
				{ProfileID: scope.ProfileID, CloudUserID: "cloud-other", BindingGeneration: scope.BindingGeneration},
				{ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration + 1},
			}
			if err := store.Save(context.Background(), current); err != nil {
				t.Fatalf("Save current: %v", err)
			}
			for _, otherScope := range otherScopes {
				if err := store.Save(context.Background(), recoveryRecord(t, otherScope, false)); err != nil {
					t.Fatalf("Save other scope: %v", err)
				}
			}

			credit := &taskRecoveryCreditClient{result: recoveryFreezeResult(scope, FreezeActive)}
			// A distinct application instance models a process restart while the
			// L1 store retains only records saved before the interruption.
			app := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store)
			decisions, err := app.RecoverPending(context.Background(), recoveryActor())
			if err != nil || len(decisions) != 1 || decisions[0].Status != TaskRecoveryContinueOrFinalize || decisions[0].Record != current {
				t.Fatalf("RecoverPending = %#v, %v", decisions, err)
			}
			if credit.getCalls != 1 || credit.createCalls != 0 || credit.scope != scope || credit.freezeID != current.FreezeID {
				t.Fatalf("credit calls = get %d create %d scope %#v freeze %q", credit.getCalls, credit.createCalls, credit.scope, credit.freezeID)
			}
		})
	}
}

func TestTaskRecoveryApplicationStoreKeepsRetryAndManualRecordsAndDeletesOnlyTermination(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	store := NewMemoryTaskRecoveryStore()
	records := []TaskRecoveryRecord{
		recoveryNamedRecord(t, scope, "task-terminate", "freeze-terminate", false),
		recoveryNamedRecord(t, scope, "task-late-final", "freeze-late-final", true),
		recoveryNamedRecord(t, scope, "task-review", "freeze-review", false),
		recoveryNamedRecord(t, scope, "task-settled", "freeze-settled", false),
	}
	for _, record := range records {
		if err := store.Save(context.Background(), record); err != nil {
			t.Fatalf("Save(%q): %v", record.TaskID, err)
		}
	}
	credit := &taskRecoveryStoreCreditClient{results: map[string]FreezeResult{
		"freeze-terminate":  recoveryNamedFreezeResult(scope, "task-terminate", "freeze-terminate", FreezeExpiredReleased),
		"freeze-late-final": recoveryNamedFreezeResult(scope, "task-late-final", "freeze-late-final", FreezeExpiredReleased),
		"freeze-review":     recoveryNamedFreezeResult(scope, "task-review", "freeze-review", FreezeExpiredPendingReview),
		"freeze-settled":    recoveryNamedFreezeResult(scope, "task-settled", "freeze-settled", FreezeSettled),
	}}
	decisions, err := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store).RecoverPending(context.Background(), recoveryActor())
	if err != nil || len(decisions) != len(records) {
		t.Fatalf("RecoverPending = %#v, %v", decisions, err)
	}
	remaining, err := store.Load(context.Background(), scope)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(remaining) != 3 || recoveryContainsTask(remaining, "task-terminate") || !recoveryContainsTask(remaining, "task-late-final") || !recoveryContainsTask(remaining, "task-review") || !recoveryContainsTask(remaining, "task-settled") {
		t.Fatalf("remaining records = %#v", remaining)
	}
	if credit.createCalls != 0 || credit.getCalls != len(records) {
		t.Fatalf("credit calls = get %d create %d", credit.getCalls, credit.createCalls)
	}
}

func TestTaskRecoveryApplicationStoreKeepsOfflineRecordForNextRecovery(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	store := NewMemoryTaskRecoveryStore()
	record := recoveryRecord(t, scope, true)
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	offline := &Error{Code: ErrorNetworkUnavailable, Retryable: true}
	decisions, err := NewTaskRecoveryApplicationWithStore(&taskRecoveryCreditClient{err: offline}, recoveryResolver(t, scope), store).RecoverPending(context.Background(), recoveryActor())
	if !errors.Is(err, offline) || len(decisions) != 1 || decisions[0].Status != TaskRecoveryRetry || decisions[0].Record != record {
		t.Fatalf("offline RecoverPending = %#v, %v", decisions, err)
	}
	remaining, err := store.Load(context.Background(), scope)
	if err != nil || len(remaining) != 1 || remaining[0] != record {
		t.Fatalf("remaining after offline = %#v, %v", remaining, err)
	}
}

func TestTaskRecoveryApplicationRejectsRecordReturnedForAnotherScope(t *testing.T) {
	t.Parallel()
	scope := recoveryScope(t)
	other := BindingScope{ProfileID: scope.ProfileID, CloudUserID: "cloud-other", BindingGeneration: scope.BindingGeneration}
	credit := &taskRecoveryCreditClient{result: recoveryFreezeResult(other, FreezeActive)}
	store := &mismatchedTaskRecoveryStore{record: recoveryRecord(t, other, false)}
	decisions, err := NewTaskRecoveryApplicationWithStore(credit, recoveryResolver(t, scope), store).RecoverPending(context.Background(), recoveryActor())
	if !isProfileMismatch(err) || len(decisions) != 0 {
		t.Fatalf("RecoverPending = %#v, %v; want scope mismatch", decisions, err)
	}
	if credit.getCalls != 0 || store.deleteCalls != 0 {
		t.Fatalf("remote calls = %d, delete calls = %d; want 0", credit.getCalls, store.deleteCalls)
	}
}

func recoveryActor() userctx.ActorContext {
	return userctx.ActorContext{ActorID: "actor-1", ProfileID: "profile-1"}
}

func recoveryScope(t *testing.T) BindingScope {
	t.Helper()
	scope, err := NewBindingScope(recoveryActor(), "cloud-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func recoveryRecord(t *testing.T, scope BindingScope, hasUsage bool) TaskRecoveryRecord {
	t.Helper()
	record, err := NewTaskRecoveryRecord(TaskStartResult{Scope: scope, TaskID: "task-1", FreezeID: "freeze-1", IdempotencyKey: "key-1"}, hasUsage)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func recoveryNamedRecord(t *testing.T, scope BindingScope, taskID, freezeID string, hasUsage bool) TaskRecoveryRecord {
	t.Helper()
	record, err := NewTaskRecoveryRecord(TaskStartResult{Scope: scope, TaskID: taskID, FreezeID: freezeID, IdempotencyKey: "key-" + taskID}, hasUsage)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func recoveryApplication(t *testing.T, scope BindingScope, credit CloudCreditClient) *TaskRecoveryApplication {
	t.Helper()
	return NewTaskRecoveryApplication(credit, recoveryResolver(t, scope))
}

func recoveryResolver(t *testing.T, scope BindingScope) BindingResolver {
	t.Helper()
	resolver, err := NewStaticBindingResolver(ProfileBinding{ActorID: "actor-1", ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration, Status: BindingBound})
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

func recoveryFailedStartResult(t *testing.T, scope BindingScope, startErr error) TaskStartResult {
	t.Helper()
	binding := ProfileBinding{ActorID: "actor-1", ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration, Status: BindingBound}
	resolver, err := NewStaticBindingResolver(binding)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewTaskStartApplication(&taskRecoveryStartCreditClient{result: recoveryFreezeResult(scope, FreezeActive)}, resolver).Start(context.Background(), recoveryActor(), TaskStartInput{
		Binding: binding, Session: Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access"}, Price: TaskPriceGate{Status: TaskPriceFresh},
		TaskID: "task-1", ModelID: "model-1", ClientTaskType: "chat", ClientEstimatedCost: "5",
		Start: func(context.Context, TaskStartAuthorization) error { return startErr },
	})
	if !errors.Is(err, startErr) || result.Started || result.FreezeID != "freeze-1" {
		t.Fatalf("Start = %#v, %v", result, err)
	}
	return result
}

func recoveryFreezeResult(scope BindingScope, status FreezeStatus) FreezeResult {
	return recoveryNamedFreezeResult(scope, "task-1", "freeze-1", status)
}

func recoveryNamedFreezeResult(scope BindingScope, taskID, freezeID string, status FreezeStatus) FreezeResult {
	return FreezeResult{
		Freeze:  Freeze{Scope: scope, FreezeID: freezeID, TaskID: taskID, Status: status, FreezeAmount: "5", FreeFrozenAmount: "3", SubscriptionFrozenAmount: "2", ExpiresAt: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)},
		Balance: BalanceSnapshot{TotalBalance: "10", FreeBalance: "5", SubscriptionBalance: "5", AvailableBalance: "5", FreeAvailableBalance: "3", SubscriptionAvailableBalance: "2", FrozenBalance: "5", FreeFrozenBalance: "3", SubscriptionFrozenBalance: "2", Version: 1},
	}
}

func recoveryContainsTask(records []TaskRecoveryRecord, taskID string) bool {
	for _, record := range records {
		if record.TaskID == taskID {
			return true
		}
	}
	return false
}

type taskRecoveryCreditClient struct {
	result      FreezeResult
	err         error
	getCalls    int
	createCalls int
	scope       BindingScope
	freezeID    string
}

func (c *taskRecoveryCreditClient) CreateFreeze(context.Context, FreezeRequest) (FreezeResult, error) {
	c.createCalls++
	return FreezeResult{}, errors.New("CreateFreeze must not be called by recovery")
}

func (c *taskRecoveryCreditClient) GetFreeze(_ context.Context, scope BindingScope, freezeID string) (FreezeResult, error) {
	c.getCalls++
	c.scope, c.freezeID = scope, freezeID
	return c.result, c.err
}

type taskRecoveryStartCreditClient struct{ result FreezeResult }

func (c *taskRecoveryStartCreditClient) CreateFreeze(context.Context, FreezeRequest) (FreezeResult, error) {
	return c.result, nil
}

func (*taskRecoveryStartCreditClient) GetFreeze(context.Context, BindingScope, string) (FreezeResult, error) {
	return FreezeResult{}, errors.New("GetFreeze is not used by task start")
}

type taskRecoveryStoreCreditClient struct {
	results     map[string]FreezeResult
	getCalls    int
	createCalls int
}

type mismatchedTaskRecoveryStore struct {
	record      TaskRecoveryRecord
	deleteCalls int
}

func (s *mismatchedTaskRecoveryStore) Save(context.Context, TaskRecoveryRecord) error {
	return nil
}

func (s *mismatchedTaskRecoveryStore) Load(context.Context, BindingScope) ([]TaskRecoveryRecord, error) {
	return []TaskRecoveryRecord{s.record}, nil
}

func (s *mismatchedTaskRecoveryStore) Delete(context.Context, TaskRecoveryRecord) error {
	s.deleteCalls++
	return nil
}

func (c *taskRecoveryStoreCreditClient) CreateFreeze(context.Context, FreezeRequest) (FreezeResult, error) {
	c.createCalls++
	return FreezeResult{}, errors.New("CreateFreeze must not be called by recovery")
}

func (c *taskRecoveryStoreCreditClient) GetFreeze(_ context.Context, _ BindingScope, freezeID string) (FreezeResult, error) {
	c.getCalls++
	return c.results[freezeID], nil
}
