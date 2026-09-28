package modelbilling

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/provider"
)

func TestTaskLifecycleRunnerWaitsForForegroundBarrierBeforeFinalState(t *testing.T) {
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 2)}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	registered := make(chan struct{})
	releaseFirst := make(chan struct{})
	releaseSecond := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
			first, err := registerForegroundCall(ctx, task.TaskID)
			if err != nil {
				return err
			}
			second, err := registerForegroundCall(ctx, task.TaskID)
			if err != nil {
				return err
			}
			close(registered)
			go func() {
				<-releaseFirst
				first.Complete(nil)
			}()
			go func() {
				<-releaseSecond
				second.Complete(nil)
			}()
			return nil
		})
	}()

	if update := <-store.updates; update.To != TaskRunning {
		t.Fatalf("first update = %#v", update)
	}
	<-registered
	select {
	case err := <-result:
		t.Fatalf("runner returned before barrier completion: %v", err)
	case update := <-store.updates:
		t.Fatalf("runner finalized before barrier completion: %#v", update)
	default:
	}
	close(releaseFirst)
	select {
	case err := <-result:
		t.Fatalf("runner returned after only one foreground call completed: %v", err)
	case update := <-store.updates:
		t.Fatalf("runner finalized after only one foreground call completed: %#v", update)
	default:
	}
	close(releaseSecond)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if update := <-store.updates; update.To != TaskCompleted {
		t.Fatalf("final update = %#v", update)
	}
}

func TestTaskLifecycleRunnerWaitsForDerivedTurnWorkLease(t *testing.T) {
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 2)}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	registered := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
			complete, err := agent.RegisterTurnWork(ctx)
			if err != nil {
				return err
			}
			close(registered)
			go func() {
				<-release
				complete(nil)
			}()
			return nil
		})
	}()
	if update := <-store.updates; update.To != TaskRunning {
		t.Fatalf("first update = %#v", update)
	}
	<-registered
	select {
	case err := <-result:
		t.Fatalf("runner returned before derived work completion: %v", err)
	case update := <-store.updates:
		t.Fatalf("runner finalized before derived work completion: %#v", update)
	default:
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if update := <-store.updates; update.To != TaskCompleted {
		t.Fatalf("final update = %#v", update)
	}
}

func TestTaskLifecycleRunnerPropagatesBarrierPersistenceErrorWithoutNormalTerminal(t *testing.T) {
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 2)}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	completeErr := errors.New("complete failed")
	recoveryErr := errors.New("recovery failed")
	want := newTerminalPersistenceError(completeErr, recoveryErr)

	err = (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
		call, err := registerForegroundCall(ctx, task.TaskID)
		if err != nil {
			return err
		}
		call.Complete(want)
		return nil
	})
	if !errors.Is(err, completeErr) || !errors.Is(err, recoveryErr) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("runner error = %v", err)
	}
	updates := store.snapshot()
	if len(updates) != 1 || updates[0].To != TaskRunning {
		t.Fatalf("updates = %#v", updates)
	}
}

func TestTaskLifecycleRunnerCloseTimeoutDoesNotWaitForPendingForegroundCall(t *testing.T) {
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 4)}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	var pending *foregroundCallRegistration
	err = (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
		var registerErr error
		pending, registerErr = registerForegroundCall(ctx, task.TaskID)
		if registerErr != nil {
			return registerErr
		}
		return agent.ErrMainStreamDispatcherCloseTimeout
	})
	if pending == nil {
		t.Fatal("foreground call was not registered")
	}
	// Completion after RunTask returns must remain safe and idempotent.
	pending.Complete(nil)
	pending.Complete(errors.New("ignored duplicate completion"))
	if !errors.Is(err, agent.ErrMainStreamDispatcherCloseTimeout) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("runner error = %v", err)
	}
	assertCloseTimeoutRecoveryUpdates(t, store.snapshot())
}

func TestTaskLifecycleRunnerCloseTimeoutPreservesCompletedBarrierErrorWithoutWaitingForPendingCall(t *testing.T) {
	store := &barrierTaskStore{updates: make(chan TaskStateUpdate, 4)}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	completeErr := errors.New("completed call persistence failed")
	var pending *foregroundCallRegistration
	err = (TaskLifecycleRunner{Tasks: store}).RunTask(ctx, func(ctx context.Context) error {
		completed, err := registerForegroundCall(ctx, task.TaskID)
		if err != nil {
			return err
		}
		pending, err = registerForegroundCall(ctx, task.TaskID)
		if err != nil {
			return err
		}
		completed.Complete(newTerminalPersistenceError(completeErr, nil))
		return agent.ErrMainStreamDispatcherCloseTimeout
	})
	if pending == nil {
		t.Fatal("pending foreground call was not registered")
	}
	pending.Complete(nil)
	if !errors.Is(err, agent.ErrMainStreamDispatcherCloseTimeout) || !errors.Is(err, completeErr) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("runner error = %v", err)
	}
	assertCloseTimeoutRecoveryUpdates(t, store.snapshot())
}

func assertCloseTimeoutRecoveryUpdates(t *testing.T, updates []TaskStateUpdate) {
	t.Helper()
	if len(updates) != 2 || updates[0].From != TaskReady || updates[0].To != TaskRunning ||
		updates[1].From != TaskRunning || updates[1].To != TaskRecoveryRequired || updates[1].Result != nil ||
		updates[1].LastErrorCode == nil || *updates[1].LastErrorCode != string(ErrorUsagePersistFailed) {
		t.Fatalf("task updates = %#v", updates)
	}
	for _, update := range updates {
		switch update.To {
		case TaskCompleted, TaskFailed, TaskCancelled, TaskUsagePending:
			t.Fatalf("unexpected ordinary terminal update: %#v", update)
		}
	}
}

func TestTaskLifecycleRunnerCancelsUndrainedForegroundStreamBeforeWaiting(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID: "task_undrained_foreground", Scope: TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model", PricingEvidenceID: "evidence",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, err = WithTaskContext(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	calls := &recordingCallStore{completeFinished: make(chan struct{})}
	dispatcher := MainExecutorDispatcher{
		Calls:   calls,
		Gateway: &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "unconsumed"}}}},
		IDs:     dispatcherIDGenerator{id: "call_undrained_foreground"},
	}
	tasks := &recordingTaskStore{}
	result := make(chan error, 1)
	go func() {
		result <- (TaskLifecycleRunner{Tasks: tasks}).RunTask(ctx, func(ctx context.Context) error {
			_, err := dispatcher.Stream(ctx, &forbiddenProvider{}, provider.Request{})
			return err
		})
	}()

	select {
	case <-calls.completeFinished:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("foreground terminal state was not persisted")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		cancel()
		<-result
		t.Fatal("runner blocked on an undrained foreground stream")
	}
	if len(calls.finished) != 1 || calls.finished[0].State != CallCancelled {
		t.Fatalf("terminal calls = %#v", calls.finished)
	}
	if len(tasks.updates) != 2 || tasks.updates[1].To != TaskUsagePending || tasks.updates[1].Result == nil || *tasks.updates[1].Result != TaskResultCompleted {
		t.Fatalf("task updates = %#v", tasks.updates)
	}
}

type barrierTaskStore struct {
	mu      sync.Mutex
	records []TaskStateUpdate
	updates chan TaskStateUpdate
}

func (s *barrierTaskStore) UpdateState(_ context.Context, update TaskStateUpdate) error {
	s.mu.Lock()
	s.records = append(s.records, update)
	s.mu.Unlock()
	s.updates <- update
	return nil
}

func (s *barrierTaskStore) snapshot() []TaskStateUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]TaskStateUpdate(nil), s.records...)
}

func (*barrierTaskStore) CreateTaskWithEvidence(context.Context, ModelTask, PricingEvidence) error {
	return nil
}
func (*barrierTaskStore) RecordFreezeRequest(context.Context, TaskContext, *cloud.FreezeRequest, time.Time) error {
	return nil
}
func (*barrierTaskStore) RecordFreezeResult(context.Context, TaskContext, *cloud.Freeze, *TaskEntitlementSnapshot, time.Time) error {
	return nil
}
func (*barrierTaskStore) Get(context.Context, TaskScope, string) (ModelTask, error) {
	return ModelTask{}, nil
}
func (*barrierTaskStore) ListRecoverable(context.Context, TaskScope) ([]ModelTask, error) {
	return nil, nil
}
