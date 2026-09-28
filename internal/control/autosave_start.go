package control

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"kwu/internal/agent"
	"kwu/internal/provider"
)

type pendingSnapshotMeta struct {
	path         string
	modelRef     string
	preview      string
	turns        int
	markActivity bool
}

func (c *Controller) startAutosaveWhileRunning(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	c.autosaveWG.Add(1)
	go func() {
		defer c.autosaveWG.Done()
		defer close(done)
		c.autosaveWhileRunning(ctx)
	}()
	return done
}

func (c *Controller) snapshot(markActivity bool) error {
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	return c.snapshotLocked(markActivity)
}

// SnapshotWithTimeout is used by UI surface transitions that must not remain
// blocked forever behind an in-flight autosave. It only bounds acquisition of
// the snapshot lock; once acquired, persistence keeps the same atomic behavior
// as Snapshot.
func (c *Controller) SnapshotWithTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("snapshot lock timeout must be positive")
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for !c.snapshotMu.TryLock() {
		select {
		case <-deadline.C:
			return fmt.Errorf("snapshot lock remained busy for %s", timeout)
		case <-ticker.C:
		}
	}
	defer c.snapshotMu.Unlock()
	return c.snapshotLocked(false)
}

func (c *Controller) snapshotLocked(markActivity bool) error {
	c.mu.Lock()
	path := c.sessionPath
	modelRef := c.modelRef
	c.mu.Unlock()
	if c.executor == nil {
		return c.flushPendingSnapshotMeta()
	}
	s := c.executor.Session()
	if !s.HasContent() {
		// Nothing to persist yet (e.g. a fresh session with only a system
		// prompt). A sidecar failure from the previous session still needs its
		// path-bound retry after a session switch.
		return c.flushPendingSnapshotMeta()
	}
	if !s.HasSystemMessage() {
		slog.Warn("controller: refusing to snapshot session with content but no system message",
			"label", c.Label(), "session_dir", c.SessionDir(), "message_count", len(s.Snapshot()))
		return nil
	}
	if path == "" {
		// There IS content but nowhere to write it: this silently dropped whole
		// bot conversations (#4414). Surface it loudly instead of returning nil
		// so the missing session path can be diagnosed and fixed at the source.
		slog.Warn("controller: session has content but no session path; conversation will not be persisted",
			"label", c.Label(), "session_dir", c.SessionDir())
		return errNoSessionPath
	}
	return c.persistSnapshot(s, path, modelRef, markActivity)
}

func (c *Controller) persistSnapshot(s *agent.Session, path, modelRef string, markActivity bool) error {
	messages, saved, err := c.saveSessionSnapshot(s, path)
	if err != nil {
		return err
	}
	if saved {
		c.saveExpertMeta(path)
	}
	c.saveGuardianSnapshot()
	if err := c.persistCurrentSnapshotMeta(s, path, modelRef, messages, saved, markActivity); err != nil {
		return err
	}
	return c.flushPendingSnapshotMeta()
}

func (c *Controller) saveSessionSnapshot(s *agent.Session, path string) ([]provider.Message, bool, error) {
	if !c.disableDirtyAutosave {
		return s.SaveIfChanged(path)
	}
	if err := s.Save(path); err != nil {
		return nil, false, err
	}
	return s.Snapshot(), true, nil
}

func (c *Controller) persistCurrentSnapshotMeta(
	s *agent.Session,
	path string,
	modelRef string,
	messages []provider.Message,
	saved bool,
	markActivity bool,
) error {
	pathKey := sessionPathKey(path)
	pending, pendingForPath := c.pendingSnapshotMeta[pathKey]
	if !saved && !markActivity && !pendingForPath {
		return nil
	}
	if !saved {
		messages = s.Snapshot()
	}
	if pendingForPath {
		markActivity = markActivity || pending.markActivity
	}
	// Persist listing-only fields from the exact transcript snapshot. A regular
	// autosave preserves UpdatedAt; a real turn (or a pending retry) advances it.
	preview, turns := agent.SessionPreviewFromMessages(messages)
	if err := agent.UpdateSessionMeta(path, modelRef, preview, turns, markActivity); err != nil {
		if c.pendingSnapshotMeta == nil {
			c.pendingSnapshotMeta = make(map[string]pendingSnapshotMeta)
		}
		c.pendingSnapshotMeta[pathKey] = pendingSnapshotMeta{
			path:         path,
			modelRef:     modelRef,
			preview:      preview,
			turns:        turns,
			markActivity: markActivity,
		}
		return err
	}
	if pendingForPath {
		delete(c.pendingSnapshotMeta, pathKey)
	}
	return nil
}

func (c *Controller) flushPendingSnapshotMeta() error {
	for pathKey, pending := range c.pendingSnapshotMeta {
		if err := agent.UpdateSessionMeta(
			pending.path,
			pending.modelRef,
			pending.preview,
			pending.turns,
			pending.markActivity,
		); err != nil {
			return err
		}
		delete(c.pendingSnapshotMeta, pathKey)
	}
	return nil
}

func sessionPathKey(path string) string {
	return filepath.Clean(path)
}

func (c *Controller) saveGuardianSnapshot() {
	// Persist guardian session so the prefix cache stays warm after restart.
	if c.guardianSess != nil {
		gp := c.guardianPath
		if gp != "" {
			if gerr := c.guardianSess.Save(gp); gerr != nil {
				slog.Warn("controller: guardian snapshot", "err", gerr)
			}
		}
	}
}
