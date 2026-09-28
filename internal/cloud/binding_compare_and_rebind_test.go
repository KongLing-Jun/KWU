package cloud

import (
	"context"
	"errors"
	"sync"
	"testing"

	"kwu/internal/userctx"
)

func TestBindingRepositoryCompareAndRebindRequiresExactCurrentBinding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	repository := NewBindingRepository()
	expected, err := repository.ConfirmBinding(ctx, actor, "usr-a")
	if err != nil {
		t.Fatal(err)
	}

	rebound, err := repository.CompareAndRebind(ctx, actor, expected, "usr-b")
	if err != nil {
		t.Fatalf("CompareAndRebind: %v", err)
	}
	if rebound.CloudUserID != "usr-b" || rebound.BindingGeneration != 2 || rebound.Status != BindingBound {
		t.Fatalf("rebound binding = %#v, want usr-b generation 2", rebound)
	}

	if _, err := repository.CompareAndRebind(ctx, actor, expected, "usr-c"); !isProfileMismatch(err) {
		t.Fatalf("stale CompareAndRebind error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	current, err := repository.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if current != rebound {
		t.Fatalf("binding after stale compare = %#v, want %#v", current, rebound)
	}
}

func TestDurableBindingRepositoryCompareAndRebindIsAtomicAcrossInstances(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	first := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	second := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	expected, err := first.ConfirmBinding(ctx, actor, "usr-a")
	if err != nil {
		t.Fatal(err)
	}

	rebound, err := second.CompareAndRebind(ctx, actor, expected, "usr-b")
	if err != nil {
		t.Fatalf("CompareAndRebind: %v", err)
	}
	if rebound.CloudUserID != "usr-b" || rebound.BindingGeneration != 2 || rebound.Status != BindingBound {
		t.Fatalf("rebound binding = %#v, want usr-b generation 2", rebound)
	}

	if _, err := first.CompareAndRebind(ctx, actor, expected, "usr-c"); !isProfileMismatch(err) {
		t.Fatalf("stale CompareAndRebind error = %v, want PROFILE_SCOPE_MISMATCH", err)
	}
	current, err := first.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if current != rebound {
		t.Fatalf("durable binding after stale compare = %#v, want %#v", current, rebound)
	}
}

func TestBindingRepositoryCompareAndRebindRejectsCancellationWhileWaitingForLock(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	repository := NewBindingRepository()
	expected, err := repository.ConfirmBinding(ctx, actor, "usr-a")
	if err != nil {
		t.Fatal(err)
	}

	base, cancel := context.WithCancel(ctx)
	observed := newFirstErrObservedContext(base)
	repository.mu.Lock()
	result := make(chan error, 1)
	go func() {
		_, err := repository.CompareAndRebind(observed, actor, expected, "usr-b")
		result <- err
	}()
	<-observed.firstErr
	cancel()
	repository.mu.Unlock()

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("CompareAndRebind error = %v, want context.Canceled", err)
	}
	current, err := repository.ResolveBinding(ctx, actor)
	if err != nil || current != expected {
		t.Fatalf("binding after canceled wait = %#v, %v; want %#v", current, err, expected)
	}
}

func TestDurableBindingRepositoryCompareAndRebindRejectsCancellationWhileWaitingForLock(t *testing.T) {
	ctx := context.Background()
	actor := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}
	provider := testProfilePathProvider(t.TempDir())
	repository := NewDurableBindingRepository(provider, testEnvironmentIdentity())
	expected, err := repository.ConfirmBinding(ctx, actor, "usr-a")
	if err != nil {
		t.Fatal(err)
	}
	path, err := durableCloudFile(provider, actor, "binding.json")
	if err != nil {
		t.Fatal(err)
	}

	base, cancel := context.WithCancel(ctx)
	observed := newFirstErrObservedContext(base)
	lock := durablePathLock(path)
	lock.Lock()
	result := make(chan error, 1)
	go func() {
		_, err := repository.CompareAndRebind(observed, actor, expected, "usr-b")
		result <- err
	}()
	<-observed.firstErr
	cancel()
	lock.Unlock()

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("CompareAndRebind error = %v, want context.Canceled", err)
	}
	current, err := repository.ResolveBinding(ctx, actor)
	if err != nil || current != expected {
		t.Fatalf("durable binding after canceled wait = %#v, %v; want %#v", current, err, expected)
	}
}

type firstErrObservedContext struct {
	context.Context
	once     sync.Once
	firstErr chan struct{}
}

func newFirstErrObservedContext(ctx context.Context) *firstErrObservedContext {
	return &firstErrObservedContext{Context: ctx, firstErr: make(chan struct{})}
}

func (c *firstErrObservedContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.firstErr) })
	return err
}
