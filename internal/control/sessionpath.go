package control

import (
	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/guardian"
	"kwu/internal/provider"
	"kwu/internal/scene"
)

// Close stops the logical session and publishes its terminal lifecycle before
// releasing runtime resources.
func (c *Controller) Close() {
	c.mu.Lock()
	planMode, planID, planState := c.planMode, c.planID, c.planState
	c.mu.Unlock()
	if planMode {
		c.emitPlanLifecycle(planID, event.PlanCancelled, "cancelled", planState)
	}
	c.emitSessionLifecycle(c.SessionPath(), event.SessionClosed, "closed", "activated")
	c.close(closeJobsWithGrace)
}

// EnsureSessionPath pins a fresh auto-save file for this controller when none is
// set yet and a session dir is configured — the "fresh session" branch every
// surface runs right after building a controller. It is a no-op once a resume or
// continue has already pinned a path (SessionPath() != ""), so callers can run a
// conditional Resume and then invoke this unconditionally. Centralises the
// per-surface copies of this logic (the CLI chat/serve fresh branches and the
// bot's former ensureControllerSessionPath).
func (c *Controller) EnsureSessionPath() {
	if c.SessionPath() != "" || c.SessionDir() == "" {
		return
	}
	c.SetSessionPath(agent.NewSessionPath(c.SessionDir(), c.Label()))
}

// AdoptHistory makes a freshly built controller continue an existing
// conversation in path: it resumes the carried messages there when there are
// any, otherwise just points auto-save at path. An empty path with no messages
// is a no-op. This is the shared kernel of the model/effort switch across the
// CLI, the HTTP server, and ACP — each computes path its own way
// (ContinueSessionPath for the CLI/serve, the pinned transcript for ACP) and
// hands the carried history (Controller.History()) here. Keeping the
// Resume/SetSessionPath choice in one place avoids the orphaned-duplicate class
// of bug (#2807) recurring as each surface copied it.
func (c *Controller) AdoptHistory(msgs []provider.Message, path string) {
	if len(msgs) > 0 {
		c.Resume(agent.NewSessionFromMessages(msgs), path)
	} else if path != "" {
		c.SetSessionPath(path)
	}
}

// NewSession snapshots the current conversation, rotates to a fresh file, and
// resets the executor to a clean session carrying the same system prompt.
func (c *Controller) NewSession() error {
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	return c.newSessionLocked()
}

func (c *Controller) newSessionLocked() error {
	if c.executor == nil {
		return nil
	}
	sysPrompt, selection, status, err := c.freshSessionPrompt()
	if err != nil {
		return err
	}
	if err := c.Snapshot(); err != nil {
		return err
	}
	oldPath := c.SessionPath()
	if c.sessionDir != "" {
		c.mu.Lock()
		c.sessionPath = agent.NewSessionPath(c.sessionDir, c.label)
		c.guardianPath = guardian.PathFor(c.sessionPath)
		c.mu.Unlock()
	}
	c.setActiveJobSession(c.SessionPath())
	c.mu.Lock()
	c.systemPrompt = sysPrompt
	c.expert = selection
	c.expertMeta, _ = expertMetaForSelection(selection, c.expertMeta, false)
	c.expertTeamStarts = make(map[string]bool)
	c.scene = status.Scene
	c.sceneState = status.State
	c.mu.Unlock()
	c.executor.SetSession(agent.NewSession(sysPrompt))
	c.saveExpertMeta(c.SessionPath())
	if c.guardianSess != nil {
		c.guardianSess.Reset()
	}
	c.ResetPlannerSession()
	c.rebindCheckpoints(c.SessionPath())
	c.mu.Lock()
	if c.sessionPrepareBlockedPath != c.sessionPath {
		c.sessionPrepareBlockedPath = ""
	}
	c.mu.Unlock()
	c.emitSessionLifecycle(oldPath, event.SessionSuspended, "suspended", "activated")
	c.emitSessionLifecycle(c.SessionPath(), event.SessionCreated, "created", "")
	c.emitSessionLifecycle(c.SessionPath(), event.SessionActivated, "activated", "created")
	return nil
}

// ClearSession discards the current conversation without preserving it in
// resume/history, then rotates to a clean session carrying the same system prompt.
func (c *Controller) ClearSession() error {
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	if c.executor == nil {
		return nil
	}
	sysPrompt, selection, status, err := c.freshSessionPrompt()
	if err != nil {
		return err
	}
	oldPath, destroy, err := c.prepareClearSession()
	if err != nil {
		return err
	}
	if c.sessionDir != "" {
		c.mu.Lock()
		c.sessionPath = agent.NewSessionPath(c.sessionDir, c.label)
		c.guardianPath = guardian.PathFor(c.sessionPath)
		c.mu.Unlock()
	}
	c.setActiveJobSession(c.SessionPath())
	c.mu.Lock()
	c.systemPrompt = sysPrompt
	c.expert = selection
	c.expertMeta, _ = expertMetaForSelection(selection, c.expertMeta, false)
	c.expertTeamStarts = make(map[string]bool)
	c.scene = status.Scene
	c.sceneState = status.State
	if c.sessionPrepareBlockedPath != c.sessionPath {
		c.sessionPrepareBlockedPath = ""
	}
	c.mu.Unlock()
	c.executor.SetSession(agent.NewSession(sysPrompt))
	c.saveExpertMeta(c.SessionPath())
	if c.guardianSess != nil {
		c.guardianSess.Reset()
	}
	c.ResetPlannerSession()
	c.rebindCheckpoints(c.SessionPath())
	if destroy.Async {
		go c.finishClearSession(oldPath, destroy)
	}
	return nil
}

func (c *Controller) finishClearSession(oldPath string, destroy SessionDestroyHandle) {
	result := destroy.Wait()
	if result.HasTimedOut() && destroy.WaitAll != nil {
		if err := agent.MarkCleanupPending(oldPath, "clear"); err != nil {
			c.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "mark cleanup pending failed: " + err.Error()})
		}
		destroy.WaitAll()
	}
	if err := removeSessionArtifacts(oldPath); err != nil {
		c.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "clear session cleanup failed: " + err.Error()})
	}
	destroy.Finish()
}

// Resume seeds the session from a loaded transcript and pins the active file to
// its path so auto-save keeps appending there.
func (c *Controller) Resume(s *agent.Session, path string) {
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	oldPath := c.SessionPath()
	status := c.SceneStatus()
	if s != nil {
		switch status.State {
		case scene.Draft, scene.Locked:
			c.mu.Lock()
			system := c.systemPrompt
			c.mu.Unlock()
			s = sessionWithSystemPrompt(s.Snapshot(), system)
		case scene.Legacy:
			if system := systemPromptFromMessages(s.Snapshot()); system != "" {
				c.mu.Lock()
				c.systemPrompt = system
				c.mu.Unlock()
			}
		}
	}
	if c.executor != nil {
		c.executor.SetSession(s)
	}
	c.loadExpertMeta(path)
	c.ResetPlannerSession()
	c.mu.Lock()
	c.sessionPath = path
	c.guardianPath = guardian.PathFor(path)
	if c.sessionPrepareBlockedPath != path {
		c.sessionPrepareBlockedPath = ""
	}
	c.mu.Unlock()
	c.setActiveJobSession(path)
	c.rebindCheckpoints(path)
	c.loadGuardianSession()
	c.maybeColdResumePrune(path)
	if oldPath != path {
		c.emitSessionLifecycle(oldPath, event.SessionSuspended, "suspended", "activated")
	}
	c.emitSessionLifecycle(path, event.SessionRecovered, "recovered", "")
	c.emitSessionLifecycle(path, event.SessionActivated, "activated", "recovered")
}

// SetSessionPath pins where auto-save lands (a fresh session file minted by the
// caller when no resume path applies).
func (c *Controller) SetSessionPath(p string) {
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	oldPath := c.SessionPath()
	c.mu.Lock()
	c.sessionPath = p
	c.guardianPath = guardian.PathFor(p)
	if c.sessionPrepareBlockedPath != p {
		c.sessionPrepareBlockedPath = ""
	}
	c.mu.Unlock()
	c.setActiveJobSession(p)
	c.rebindCheckpoints(p)
	if oldPath != p {
		c.emitSessionLifecycle(oldPath, event.SessionSuspended, "suspended", "activated")
		c.emitSessionLifecycle(p, event.SessionCreated, "created", "")
	}
	c.emitSessionLifecycle(p, event.SessionActivated, "activated", "created")
}

func (c *Controller) emitSessionLifecycle(path string, kind event.OperationalKind, state, previous string) {
	if c == nil || c.operational == nil || path == "" {
		return
	}
	sessionID := agent.BranchID(path)
	if sessionID == "" {
		return
	}
	c.operational.Emit(event.OperationalFact{
		AggregateKind: event.AggregateSession,
		AggregateID:   sessionID,
		Scope:         event.OperationalScope{SessionID: sessionID},
		Kind:          kind, Stage: "controller", Outcome: state, Severity: "info", UserImpact: "none",
		Payload: &event.LifecycleOperationalPayload{State: state, PreviousState: previous},
	})
}
