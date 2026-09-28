package tasktree

import (
	"context"
	"sync"
)

// StateStore is the orchestrator boundary for mode-specific task-tree state.
// Implementations may be in-memory, session metadata-backed, or sidecar-backed;
// Controller still only persists ordinary chat messages through agent.Session.
type StateStore interface {
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
	LoadSnapshot(ctx context.Context) (Snapshot, bool, error)
}

// MemoryStateStore is the first in-process state container used by the runner
// skeleton and tests. It clones snapshots through the JSON contract so callers
// cannot mutate stored task-tree state by keeping pointers.
type MemoryStateStore struct {
	mu       sync.RWMutex
	snapshot Snapshot
	ok       bool
}

func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{}
}

func (s *MemoryStateStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cloned, err := cloneSnapshot(snapshot)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloned
	s.ok = true
	return nil
}

func (s *MemoryStateStore) LoadSnapshot(ctx context.Context) (Snapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, false, err
	}
	s.mu.RLock()
	snapshot, ok := s.snapshot, s.ok
	s.mu.RUnlock()
	if !ok {
		return Snapshot{}, false, nil
	}
	cloned, err := cloneSnapshot(snapshot)
	if err != nil {
		return Snapshot{}, false, err
	}
	return cloned, true, nil
}

func cloneSnapshot(snapshot Snapshot) (Snapshot, error) {
	b, err := MarshalSnapshot(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	return UnmarshalSnapshot(b)
}
