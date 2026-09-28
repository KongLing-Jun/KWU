package boot

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSubagentCleanupCoordinatorSharesConcurrentCleanup(t *testing.T) {
	coordinator := newSubagentCleanupCoordinator()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	cleanup := func() error {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil
	}

	const callers = 8
	errs := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			errs <- coordinator.do("same-dir", cleanup)
		}()
	}
	ready.Wait()
	<-started
	close(release)
	for range callers {
		if err := <-errs; err != nil {
			t.Fatalf("coordinator.do: %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cleanup calls = %d, want 1", got)
	}
}

func TestSubagentCleanupCoordinatorScopesByDirectory(t *testing.T) {
	coordinator := newSubagentCleanupCoordinator()
	var calls atomic.Int32
	cleanup := func() error {
		calls.Add(1)
		return nil
	}

	if err := coordinator.do("first-dir", cleanup); err != nil {
		t.Fatalf("first directory: %v", err)
	}
	if err := coordinator.do("second-dir", cleanup); err != nil {
		t.Fatalf("second directory: %v", err)
	}
	if err := coordinator.do("first-dir", cleanup); err != nil {
		t.Fatalf("repeated first directory: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("cleanup calls = %d, want 2", got)
	}
}

func TestSubagentCleanupCoordinatorRetriesAfterFailure(t *testing.T) {
	coordinator := newSubagentCleanupCoordinator()
	wantErr := errors.New("temporary cleanup failure")
	var calls atomic.Int32
	cleanup := func() error {
		if calls.Add(1) == 1 {
			return wantErr
		}
		return nil
	}

	if err := coordinator.do("retry-dir", cleanup); !errors.Is(err, wantErr) {
		t.Fatalf("first cleanup error = %v, want %v", err, wantErr)
	}
	if err := coordinator.do("retry-dir", cleanup); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("cleanup calls = %d, want 2", got)
	}
}
