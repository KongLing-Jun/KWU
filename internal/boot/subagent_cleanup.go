package boot

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"kwu/internal/agent"
)

type subagentCleanupRun struct {
	done chan struct{}
	err  error
}

type subagentCleanupCoordinator struct {
	mu   sync.Mutex
	runs map[string]*subagentCleanupRun
}

func newSubagentCleanupCoordinator() *subagentCleanupCoordinator {
	return &subagentCleanupCoordinator{runs: make(map[string]*subagentCleanupRun)}
}

func (c *subagentCleanupCoordinator) do(key string, cleanup func() error) error {
	c.mu.Lock()
	if run, ok := c.runs[key]; ok {
		c.mu.Unlock()
		<-run.done
		return run.err
	}
	run := &subagentCleanupRun{done: make(chan struct{})}
	c.runs[key] = run
	c.mu.Unlock()

	run.err = cleanup()
	c.mu.Lock()
	close(run.done)
	if run.err != nil {
		delete(c.runs, key)
	}
	c.mu.Unlock()
	return run.err
}

func subagentCleanupKey(dir string) (string, error) {
	key, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key, nil
}

var processSubagentCleanup = newSubagentCleanupCoordinator()

func newSubagentStore(sessionDir string) (*agent.SubagentStore, error) {
	sessionDir = strings.TrimSpace(sessionDir)
	if sessionDir == "" {
		return nil, nil
	}
	dir := filepath.Join(sessionDir, "subagents")
	key, err := subagentCleanupKey(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve subagent cleanup directory: %w", err)
	}
	store := agent.NewSubagentStore(dir)
	if err := processSubagentCleanup.do(key, func() error {
		_, err := store.CleanupStaleRunning()
		return err
	}); err != nil {
		return nil, fmt.Errorf("cleanup stale subagents: %w", err)
	}
	return store, nil
}
