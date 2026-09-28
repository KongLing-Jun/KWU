package modelbilling

import (
	"context"
	"errors"
	"sync"
)

type foregroundCallBarrierKey struct{}

type foregroundCallBarrier struct {
	mu      sync.Mutex
	taskID  string
	pending int
	sealed  bool
	err     error
	done    chan struct{}
	closed  bool
}

type foregroundCallRegistration struct {
	once    sync.Once
	barrier *foregroundCallBarrier
}

func newForegroundCallBarrier(taskID string) *foregroundCallBarrier {
	return &foregroundCallBarrier{taskID: taskID, done: make(chan struct{})}
}

func withForegroundCallBarrier(ctx context.Context, barrier *foregroundCallBarrier) context.Context {
	return context.WithValue(ctx, foregroundCallBarrierKey{}, barrier)
}

func foregroundCallBarrierFrom(ctx context.Context) (*foregroundCallBarrier, bool) {
	if ctx == nil {
		return nil, false
	}
	barrier, ok := ctx.Value(foregroundCallBarrierKey{}).(*foregroundCallBarrier)
	return barrier, ok && barrier != nil
}

func registerForegroundCall(ctx context.Context, taskID string) (*foregroundCallRegistration, error) {
	barrier, ok := foregroundCallBarrierFrom(ctx)
	if !ok {
		return nil, newError(ErrorContractInvalid, "foreground model call requires task lifecycle barrier")
	}
	return barrier.register(taskID)
}

func (b *foregroundCallBarrier) register(taskID string) (*foregroundCallRegistration, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if taskID != b.taskID {
		return nil, newError(ErrorScopeMismatch, "foreground call barrier task does not match task context")
	}
	if b.sealed {
		return nil, newError(ErrorInvalidStateTransition, "foreground call registered after task run returned")
	}
	b.pending++
	return &foregroundCallRegistration{barrier: b}, nil
}

func (r *foregroundCallRegistration) Complete(err error) {
	if r == nil || r.barrier == nil {
		return
	}
	r.once.Do(func() { r.barrier.complete(err) })
}

func (b *foregroundCallBarrier) complete(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pending == 0 {
		return
	}
	b.pending--
	if err != nil {
		if b.err == nil {
			b.err = err
		} else {
			b.err = errors.Join(b.err, err)
		}
	}
	b.closeIfDone()
}

func (b *foregroundCallBarrier) seal() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sealed = true
	b.closeIfDone()
}

func (b *foregroundCallBarrier) closeIfDone() {
	if b.sealed && b.pending == 0 && !b.closed {
		b.closed = true
		close(b.done)
	}
}

func (b *foregroundCallBarrier) wait() error {
	<-b.done
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func (b *foregroundCallBarrier) collectedError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}
