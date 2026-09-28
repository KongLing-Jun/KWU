package modelbilling

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/provider"
)

func TestJoinCausalErrorsDeduplicatesOnlyRelatedErrors(t *testing.T) {
	cause := errors.New("provider failed")
	wrapped := fmt.Errorf("gateway: %w", cause)

	if got := joinCausalErrors(cause, cause); got != cause {
		t.Fatalf("same error = %v, want original", got)
	}
	if got := joinCausalErrors(wrapped, cause); got != wrapped {
		t.Fatalf("wrapped error = %v, want outer error", got)
	}
	if got := joinCausalErrors(cause, wrapped); got != wrapped {
		t.Fatalf("reverse wrapped error = %v, want outer error", got)
	}

	first := errors.New("same text")
	second := errors.New("same text")
	got := joinCausalErrors(first, second)
	if !errors.Is(got, first) || !errors.Is(got, second) {
		t.Fatalf("distinct errors = %v, want both causes", got)
	}
	many, ok := got.(interface{ Unwrap() []error })
	if !ok || len(many.Unwrap()) != 2 {
		t.Fatalf("distinct errors = %#v, want a two-cause join", got)
	}
}

type usagePendingRecoverySpy struct {
	scopes  []TaskScope
	taskIDs []string
	err     error
}

func (s *usagePendingRecoverySpy) RecoverUsagePendingTask(_ context.Context, scope TaskScope, taskID string) error {
	s.scopes = append(s.scopes, scope)
	s.taskIDs = append(s.taskIDs, taskID)
	return s.err
}

func TestTaskLifecycleRunnerTransitionsOnlyAfterWholeRun(t *testing.T) {
	store := &recordingTaskStore{}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := TaskLifecycleRunner{Tasks: store, Now: func() time.Time { return time.Date(2026, 7, 13, 1, 0, 0, 0, time.UTC) }}
	if err := runner.RunTask(ctx, func(context.Context) error {
		calls++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(store.updates) != 2 || store.updates[0].From != TaskReady || store.updates[0].To != TaskRunning || store.updates[1].From != TaskRunning || store.updates[1].To != TaskCompleted || store.updates[1].Result == nil || *store.updates[1].Result != TaskResultCompleted {
		t.Fatalf("calls/updates = %d/%#v", calls, store.updates)
	}
}

func TestTaskLifecycleRunnerSubscriptionEndsUsagePending(t *testing.T) {
	store := &recordingTaskStore{}
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{TaskID: "task_subscription", Scope: TaskScope{ActorID: "actor", ProfileID: "default", CloudBinding: &binding}, BillingSource: BillingSubscriptionPool, InitialModelID: "model", PricingEvidenceID: "evidence"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	runner := TaskLifecycleRunner{Tasks: store}
	if err := runner.RunTask(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(store.updates) != 2 || store.updates[1].To != TaskUsagePending || store.updates[1].Result == nil || *store.updates[1].Result != TaskResultCompleted {
		t.Fatalf("updates = %#v", store.updates)
	}
}

func TestTaskLifecycleRunnerSubscriptionFailureAndCancellationEndUsagePending(t *testing.T) {
	binding := testBindingScope()
	for _, test := range []struct {
		name       string
		runErr     error
		wantResult TaskResult
	}{
		{name: "failed", runErr: errors.New("model failed"), wantResult: TaskResultFailed},
		{name: "cancelled", runErr: context.Canceled, wantResult: TaskResultCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := NewTaskContext(TaskContextInput{
				TaskID:        "task_subscription_" + test.name,
				Scope:         TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
				BillingSource: BillingSubscriptionPool, InitialModelID: "model", PricingEvidenceID: "evidence",
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, err := WithTaskContext(context.Background(), task)
			if err != nil {
				t.Fatal(err)
			}
			store := &recordingTaskStore{}
			err = (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(context.Context) error { return test.runErr })
			if !errors.Is(err, test.runErr) {
				t.Fatalf("runner error = %v", err)
			}
			if len(store.updates) != 2 || store.updates[1].To != TaskUsagePending || store.updates[1].Result == nil || *store.updates[1].Result != test.wantResult {
				t.Fatalf("task updates = %#v", store.updates)
			}
		})
	}
}

func TestTaskLifecycleRunnerRecoversFailedSubscriptionAfterPersistence(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID:        "task_subscription_recovery",
		Scope:         TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model", PricingEvidenceID: "evidence",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	store := &recordingTaskStore{}
	recovery := &usagePendingRecoverySpy{}
	runErr := errors.New("model failed")
	err = (TaskLifecycleRunner{Tasks: store, Recovery: recovery}).RunTask(ctx, func(context.Context) error { return runErr })
	if !errors.Is(err, runErr) {
		t.Fatalf("runner error = %v, want original failure", err)
	}
	if len(store.updates) != 2 || store.updates[1].To != TaskUsagePending {
		t.Fatalf("task updates = %#v", store.updates)
	}
	if len(recovery.taskIDs) != 1 || recovery.taskIDs[0] != task.TaskID || !recovery.scopes[0].Equal(task.Scope) {
		t.Fatalf("recovery calls = ids:%v scopes:%+v", recovery.taskIDs, recovery.scopes)
	}
}

func TestTaskLifecycleRunnerReportsFailedSubscriptionRecoveryError(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID:        "task_subscription_recovery_error",
		Scope:         TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model", PricingEvidenceID: "evidence",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	runErr := errors.New("model failed")
	recoveryErr := errors.New("release unavailable")
	err = (TaskLifecycleRunner{
		Tasks: &recordingTaskStore{}, Recovery: &usagePendingRecoverySpy{err: recoveryErr},
	}).RunTask(ctx, func(context.Context) error { return runErr })
	if !errors.Is(err, runErr) || !errors.Is(err, recoveryErr) {
		t.Fatalf("runner error = %v, want original plus recovery failure", err)
	}
}

func TestTaskLifecycleRunnerKeepsTaskRunningAcrossMultipleMainCalls(t *testing.T) {
	tasks := &recordingTaskStore{}
	calls := &recordingCallStore{}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := MainExecutorDispatcher{Calls: calls, IDs: &sequenceIDGenerator{values: []string{"call_first", "call_second"}}}
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkDone}}}
	runner := TaskLifecycleRunner{Tasks: tasks}
	if err := runner.RunTask(ctx, func(ctx context.Context) error {
		for range []int{1, 2} {
			chunks, err := dispatcher.Stream(ctx, base, provider.Request{})
			if err != nil {
				return err
			}
			for range chunks {
			}
		}
		if len(tasks.updates) != 1 || tasks.updates[0].To != TaskRunning {
			return errors.New("task left running before the complete agent run")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(calls.prepared) != 2 || calls.prepared[0].CallID == calls.prepared[1].CallID {
		t.Fatalf("prepared call identities = %#v", calls.prepared)
	}
	if len(tasks.updates) != 2 || tasks.updates[1].To != TaskCompleted {
		t.Fatalf("task lifecycle = %#v", tasks.updates)
	}
}

func TestTaskLifecycleRunnerStopsWhenRunningPersistenceFails(t *testing.T) {
	want := errors.New("write failed")
	store := &failingLifecycleStore{err: want}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	runner := TaskLifecycleRunner{Tasks: store}
	if err := runner.RunTask(ctx, func(context.Context) error { called = true; return nil }); !errors.Is(err, want) || called {
		t.Fatalf("err/called = %v/%v", err, called)
	}
}

func TestTaskLifecycleRunnerMarksRecoveryWhenDispatcherCloseTimesOut(t *testing.T) {
	store := &recordingTaskStore{}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	runner := TaskLifecycleRunner{Tasks: store}
	if err := runner.RunTask(ctx, func(context.Context) error {
		return agent.ErrMainStreamDispatcherCloseTimeout
	}); !errors.Is(err, agent.ErrMainStreamDispatcherCloseTimeout) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("runner error = %v", err)
	}
	if len(store.updates) != 2 || store.updates[1].To != TaskRecoveryRequired || store.updates[1].Result != nil {
		t.Fatalf("task updates = %#v", store.updates)
	}
}

func TestTaskLifecycleRunnerCloseTimeoutPreservesRecoveryWriteFailure(t *testing.T) {
	recoveryErr := errors.New("recovery update failed")
	store := &recordingTaskStore{updateErr: recoveryErr, failUpdateTo: TaskRecoveryRequired}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	err = (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(context.Context) error {
		return agent.ErrMainStreamDispatcherCloseTimeout
	})
	if !errors.Is(err, agent.ErrMainStreamDispatcherCloseTimeout) || !errors.Is(err, recoveryErr) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("runner error = %v", err)
	}
	if len(store.updates) != 2 || store.updates[0].To != TaskRunning || store.updates[1].To != TaskRecoveryRequired {
		t.Fatalf("task updates = %#v", store.updates)
	}
}

func TestTaskLifecycleRunnerFinalizesOrdinaryFailureAndCancellation(t *testing.T) {
	ordinaryErr := errors.New("model failed")
	for _, test := range []struct {
		name       string
		runErr     error
		wantState  TaskState
		wantResult TaskResult
	}{
		{name: "failed", runErr: ordinaryErr, wantState: TaskFailed, wantResult: TaskResultFailed},
		{name: "cancelled", runErr: context.Canceled, wantState: TaskCancelled, wantResult: TaskResultCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingTaskStore{}
			task := dispatcherBYOKTask(t)
			ctx, err := WithTaskContext(context.Background(), task)
			if err != nil {
				t.Fatal(err)
			}
			runner := TaskLifecycleRunner{Tasks: store}
			err = runner.RunTask(ctx, func(context.Context) error { return test.runErr })
			if !errors.Is(err, test.runErr) {
				t.Fatalf("runner error = %v, want %v", err, test.runErr)
			}
			if len(store.updates) != 2 || store.updates[1].To != test.wantState || store.updates[1].Result == nil || *store.updates[1].Result != test.wantResult {
				t.Fatalf("task updates = %#v", store.updates)
			}
		})
	}
}

type failingLifecycleStore struct{ err error }

func (s *failingLifecycleStore) CreateTaskWithEvidence(context.Context, ModelTask, PricingEvidence) error {
	return nil
}
func (s *failingLifecycleStore) RecordFreezeRequest(context.Context, TaskContext, *cloud.FreezeRequest, time.Time) error {
	return nil
}
func (s *failingLifecycleStore) RecordFreezeResult(context.Context, TaskContext, *cloud.Freeze, *TaskEntitlementSnapshot, time.Time) error {
	return nil
}
func (s *failingLifecycleStore) UpdateState(context.Context, TaskStateUpdate) error { return s.err }
func (*failingLifecycleStore) Get(context.Context, TaskScope, string) (ModelTask, error) {
	return ModelTask{}, nil
}
func (*failingLifecycleStore) ListRecoverable(context.Context, TaskScope) ([]ModelTask, error) {
	return nil, nil
}
