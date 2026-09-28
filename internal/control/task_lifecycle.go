package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/jobs"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/tool"
)

type SceneSystemPromptBuilder func(scene.Scene, string, string) (string, expert.Selection, error)

type SceneSessionSaver func(*agent.Session, string) error

type SceneStatusSaver func(string, scene.Status) error

// runRootRunner is the single Controller boundary for top-level Runner
// dispatch. Desktop Draft scenes are committed before any Runner variant sees
// the request; unadopted hosts and already-locked sessions remain no-ops.
func (c *Controller) runRootRunner(ctx context.Context, input string) error {
	if err := c.commitDraftScene(); err != nil {
		return err
	}
	if err := c.persistExpertMeta(c.SessionPath(), true); err != nil {
		return fmt.Errorf("record expert-team use: %w", err)
	}
	var session *agent.Session
	var compactionVersion uint64
	if c.executor != nil {
		session = c.executor.Session()
		if session != nil {
			compactionVersion = session.CompactionVersion()
		}
	}
	defer c.invalidateCheckpointBoundsAfterCompaction(session, compactionVersion)
	return c.runner.Run(ctx, input)
}

func (c *Controller) invalidateCheckpointBoundsAfterCompaction(session *agent.Session, before uint64) {
	if session != nil && session.CompactionVersion() != before {
		c.checkpoints.clearBounds()
	}
}

func sceneStatusNeedsCommit(status scene.Status) (bool, error) {
	if status.State != scene.Draft {
		if status.State == scene.StateUnset || status.State == scene.Legacy || status.State == scene.Locked {
			return false, nil
		}
		return false, fmt.Errorf("invalid scene state %q", status.State)
	}
	if err := status.Validate(); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Controller) commitDraftScene() error {
	c.sceneCommitMu.Lock()
	defer c.sceneCommitMu.Unlock()
	status := c.SceneStatus()
	needsCommit, err := sceneStatusNeedsCommit(status)
	if err != nil || !needsCommit {
		return err
	}
	if c.executor == nil || c.executor.Session() == nil {
		return fmt.Errorf("cannot commit scene without an active session")
	}
	if c.executor.Session().HasContent() {
		return fmt.Errorf("cannot commit draft scene after conversation content exists")
	}

	c.mu.Lock()
	selection := c.expert
	path := strings.TrimSpace(c.sessionPath)
	c.mu.Unlock()
	if path == "" {
		return errNoSessionPath
	}
	sessionExpertID := expertIDForPrompt(selection)
	prompt, nextSelection, err := c.buildSystemPromptForScene(status.Scene, sessionExpertID, "")
	if err != nil {
		return fmt.Errorf("build scene prompt: %w", err)
	}
	nextSession := sessionWithSystemPrompt(c.executor.Session().Snapshot(), prompt)
	locked := scene.Status{Scene: status.Scene, State: scene.Locked}
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()

	if err := c.saveSceneCommitSession(nextSession, path); err != nil {
		return fmt.Errorf("save scene session: %w", err)
	}
	if err := c.saveSceneCommitMeta(nextSession, path, locked); err != nil {
		return fmt.Errorf("save locked scene: %w", err)
	}
	c.executor.SetSession(nextSession)
	c.mu.Lock()
	c.systemPrompt = prompt
	c.expert = nextSelection
	c.sceneState = scene.Locked
	c.mu.Unlock()
	c.saveExpertMeta(path)
	return nil
}

// SetDraftScene changes only a new Desktop session's draft selection. The
// regular submit API never carries a Scene override, and Locked/legacy sessions
// cannot be changed in place.
func (c *Controller) SetDraftScene(next scene.Scene) error {
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	if !next.IsSelectable() {
		return fmt.Errorf("invalid selectable scene %q", next)
	}
	c.sceneCommitMu.Lock()
	defer c.sceneCommitMu.Unlock()
	status := c.SceneStatus()
	if status.State != scene.Draft {
		return fmt.Errorf("scene is %s and cannot be changed", status.State)
	}
	if c.executor == nil || c.executor.Session() == nil || c.executor.Session().HasContent() {
		return fmt.Errorf("scene can only be changed before the first turn")
	}
	c.mu.Lock()
	path := strings.TrimSpace(c.sessionPath)
	c.mu.Unlock()
	if path == "" {
		return errNoSessionPath
	}
	nextStatus := scene.Status{Scene: next, State: scene.Draft}
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	if err := c.saveSceneCommitStatus(path, nextStatus); err != nil {
		return fmt.Errorf("save draft scene: %w", err)
	}
	c.mu.Lock()
	c.scene = next
	c.mu.Unlock()
	return nil
}

func expertIDForPrompt(selection expert.Selection) string {
	if selection.Selected {
		return selection.ID
	}
	return expert.ExplicitNoneID
}

func (c *Controller) saveSceneCommitSession(session *agent.Session, path string) error {
	c.mu.Lock()
	saver := c.sceneSessionSave
	c.mu.Unlock()
	if saver != nil {
		return saver(session, path)
	}
	return session.Save(path)
}

func (c *Controller) saveSceneCommitStatus(path string, status scene.Status) error {
	c.mu.Lock()
	saver := c.sceneStatusSave
	c.mu.Unlock()
	if saver != nil {
		return saver(path, status)
	}
	return agent.MutateBranchMeta(path, false, func(meta *agent.BranchMeta) error {
		return meta.SetSceneStatus(status)
	})
}

func (c *Controller) saveSceneCommitMeta(session *agent.Session, path string, status scene.Status) error {
	c.mu.Lock()
	saver := c.sceneStatusSave
	modelRef := c.modelRef
	c.mu.Unlock()
	if saver != nil {
		return saver(path, status)
	}
	preview, turns := agent.SessionPreviewFromMessages(session.Snapshot())
	return agent.MutateBranchMeta(path, false, func(meta *agent.BranchMeta) error {
		if strings.TrimSpace(modelRef) != "" {
			meta.Model = strings.TrimSpace(modelRef)
		}
		meta.Preview = preview
		meta.Turns = turns
		meta.SchemaVersion = agent.BranchMetaCountsVersion
		return meta.SetSceneStatus(status)
	})
}

func (c *Controller) addSceneStatus(meta *agent.BranchMeta) {
	if sceneValue, stateValue, ok := c.SceneStatus().Persisted(); ok {
		meta.Scene = sceneValue
		meta.SceneState = stateValue
	}
}

func (c *Controller) buildSystemPromptForScene(promptScene scene.Scene, sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
	c.mu.Lock()
	sceneBuilder := c.scenePromptBuild
	legacyBuilder := c.promptBuild
	c.mu.Unlock()
	if sceneBuilder != nil {
		return sceneBuilder(promptScene, sessionExpertID, requestExpertID)
	}
	if legacyBuilder != nil {
		return legacyBuilder(sessionExpertID, requestExpertID)
	}
	return "", expert.Selection{}, fmt.Errorf("expert prompt builder is not configured")
}

func (c *Controller) buildSystemPrompt(sessionExpertID, requestExpertID string) (string, expert.Selection, error) {
	c.mu.Lock()
	promptScene := c.scene
	c.mu.Unlock()
	return c.buildSystemPromptForScene(promptScene, sessionExpertID, requestExpertID)
}

func (c *Controller) SceneStatus() scene.Status {
	if c == nil {
		return scene.Status{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return scene.Status{Scene: c.scene, State: c.sceneState}
}

func (c *Controller) freshSessionPrompt() (string, expert.Selection, scene.Status, error) {
	c.mu.Lock()
	status := scene.Status{Scene: c.scene, State: c.sceneState}
	sysPrompt := c.systemPrompt
	selection := c.expert
	projectExpertIDs := append([]string(nil), c.expertMeta.ProjectExpertIDs...)
	c.mu.Unlock()
	if status.State == scene.StateUnset {
		return sysPrompt, selection, status, nil
	}
	next := scene.Status{Scene: scene.Office, State: scene.Draft}
	if len(projectExpertIDs) > 1 {
		nextPrompt, nextSelection, err := c.buildProjectExpertTeamPromptForScene(next.Scene, projectExpertIDs, expert.SourceSession)
		return nextPrompt, nextSelection, next, err
	}
	sessionExpertID := expertIDForPrompt(selection)
	nextPrompt, nextSelection, err := c.buildSystemPromptForScene(next.Scene, sessionExpertID, "")
	if err != nil {
		return "", expert.Selection{}, status, err
	}
	return nextPrompt, nextSelection, next, nil
}

func systemPromptFromMessages(messages []provider.Message) string {
	for _, message := range messages {
		if message.Role == provider.RoleSystem {
			return message.Content
		}
	}
	return ""
}

// TaskLifecycle owns one billing task around a complete top-level user
// submission. The callback may invoke the underlying runner multiple times.
type TaskLifecycle interface {
	RunTask(context.Context, func(context.Context) error) error
}

// SessionToolProvider resolves trusted host tools for one persisted session.
// The controller invokes it only after the session send guard succeeds and
// binds the returned tools to the current top-level turn without mutating the
// controller's long-lived registry.
type SessionToolProvider func(context.Context, string) ([]tool.Tool, error)

func (c *Controller) runTaskLifecycle(ctx context.Context, run func(context.Context) error) error {
	if err := c.authorizeSessionSend(ctx); err != nil {
		return err
	}
	ctx, toolCredits := tool.WithCreditRecorder(ctx)
	startMessages := c.messageCount()
	var artifactTurnID string
	var artifactObservation *agent.WorkspaceArtifactObservation
	var completedDelivery provider.ArtifactDelivery
	artifactCommitted := false
	artifactStarted := false
	metadataRecorded := false
	runErr := c.runBoundTaskLifecycle(ctx, func(ctx context.Context) error {
		if err := c.commitDraftScene(); err != nil {
			return err
		}
		startMessages = c.messageCount()
		artifactTurnID, artifactObservation = c.beginArtifactDelivery(ctx)
		artifactStarted = true
		defer c.endArtifactDelivery(artifactTurnID)
		ctx = agent.WithWorkspaceArtifactObservation(ctx, artifactObservation)
		boundCtx, err := c.bindSessionTools(ctx)
		if err != nil {
			return err
		}
		if err := run(boundCtx); err != nil {
			return err
		}
		toolMaaSQuota, toolChargedCredits := toolCredits.Summary()
		c.recordCompletedTurnMetadata(ctx, startMessages, toolMaaSQuota, toolChargedCredits)
		metadataRecorded = true
		completedDelivery, artifactCommitted, err = c.commitArtifactDelivery(artifactTurnID, artifactObservation)
		return err
	})
	if !metadataRecorded {
		toolMaaSQuota, toolChargedCredits := toolCredits.Summary()
		c.recordCompletedTurnMetadata(ctx, startMessages, toolMaaSQuota, toolChargedCredits)
	}
	if !artifactStarted {
		return runErr
	}
	if runErr != nil {
		c.completedArtifactDelivery.Store(nil)
		return c.persistUncommittedArtifactFinal(artifactTurnID, runErr)
	}
	if artifactCommitted {
		c.setCompletedArtifactDelivery(completedDelivery)
	}
	return nil
}

func (c *Controller) beginArtifactDelivery(ctx context.Context) (string, *agent.WorkspaceArtifactObservation) {
	if c.executor == nil {
		return "", agent.NewWorkspaceArtifactObservation(c.workspaceRoot)
	}
	artifactTurnID := event.TurnIDFromContext(ctx)
	if artifactTurnID == "" {
		artifactTurnID = event.NewTurnID()
	}
	session := c.executor.Session()
	session.BeginArtifactDeliveryTurn(artifactTurnID)
	previous := agent.PreviousArtifactDeliveryPaths(session.Snapshot())
	return artifactTurnID, agent.NewWorkspaceArtifactObservation(c.workspaceRoot, previous...)
}

func (c *Controller) endArtifactDelivery(artifactTurnID string) {
	if c.executor != nil {
		c.executor.Session().EndArtifactDeliveryTurn(artifactTurnID)
	}
}

type artifactDeliveryRoot struct {
	lexical string
	real    string
}

func (c *Controller) finalizeArtifactDelivery(artifactTurnID string, deliveredAt int64, _ *agent.WorkspaceArtifactObservation) (provider.ArtifactDelivery, bool) {
	delivery, ok := c.buildArtifactDelivery(artifactTurnID, deliveredAt)
	if !ok {
		return provider.ArtifactDelivery{}, false
	}
	return c.executor.Session().CommitArtifactDelivery(artifactTurnID, delivery)
}

func (c *Controller) buildArtifactDelivery(artifactTurnID string, deliveredAt int64) (provider.ArtifactDelivery, bool) {
	if c.executor == nil || artifactTurnID == "" || deliveredAt <= 0 {
		return provider.ArtifactDelivery{}, false
	}
	messages := c.executor.Session().Snapshot()
	turn := make([]provider.Message, 0)
	for _, message := range messages {
		if message.ArtifactTurnID == artifactTurnID {
			turn = append(turn, message)
		}
	}
	candidates := []string(nil)
	if observed, participated, valid := c.executor.ArtifactDeliveryTurnDecision(artifactTurnID); participated {
		// Once the real Agent participates, its exact-turn host ledger is
		// authoritative. Missing or mutation-invalidated decisions fail closed;
		// transcript fallback here could revive a stale pre-mutation manifest.
		if valid {
			candidates = observed
		}
	} else {
		// Custom Runners do not populate the Agent ledger, so retain the legacy
		// compatibility path while still restricting it to this exact turn.
		candidates = agent.ArtifactDeliveryCandidates(turn)
	}
	delivery := provider.ArtifactDelivery{
		Version:     provider.ArtifactDeliveryVersion,
		Paths:       resolvedArtifactDeliveryPaths(c.workspaceRoot, candidates),
		DeliveredAt: deliveredAt,
	}
	return delivery, true
}

func resolvedArtifactDeliveryPaths(workspaceRoot string, candidates []string) []string {
	root, ok := resolveArtifactDeliveryRoot(workspaceRoot)
	if !ok {
		return []string{}
	}
	paths := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		relative, ok := root.resolveCandidate(candidate)
		if !ok {
			continue
		}
		key := artifactDeliveryRelativePathKey(relative)
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, relative)
		if len(paths) == provider.MaxArtifactDeliveryPaths {
			break
		}
	}
	return paths
}

func resolveArtifactDeliveryRoot(workspaceRoot string) (artifactDeliveryRoot, bool) {
	root := normalizeArtifactPathForOS(strings.TrimSpace(workspaceRoot))
	if root == "" {
		return artifactDeliveryRoot{}, false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return artifactDeliveryRoot{}, false
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	return artifactDeliveryRoot{lexical: rootAbs, real: realRoot}, err == nil
}

func (root artifactDeliveryRoot) resolveCandidate(candidate string) (string, bool) {
	path, ok := lexicalArtifactDeliveryCandidate(root.lexical, candidate)
	if !ok || !regularArtifactDeliveryFile(path) {
		return "", false
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil || !artifactPathWithinRoot(root.real, realPath) {
		return "", false
	}
	relative, err := filepath.Rel(root.real, realPath)
	if err != nil || relative == "." {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func lexicalArtifactDeliveryCandidate(root, candidate string) (string, bool) {
	candidate = normalizeArtifactPathForOS(strings.TrimSpace(candidate))
	if candidate == "" {
		return "", false
	}
	path := filepath.FromSlash(candidate)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	return path, err == nil && artifactPathWithinRoot(root, path)
}

func regularArtifactDeliveryFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func artifactDeliveryRelativePathKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func artifactPathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func normalizeArtifactPathForOS(path string) string {
	if runtime.GOOS != "windows" || len(path) < len("/mnt/x") || !strings.EqualFold(path[:5], "/mnt/") {
		return path
	}
	drive := path[5]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) || (len(path) > 6 && path[6] != '/') {
		return path
	}
	rest := strings.TrimPrefix(path[6:], "/")
	return strings.ToUpper(string(drive)) + ":/" + rest
}

func cloneCompletedArtifactDelivery(delivery provider.ArtifactDelivery) provider.ArtifactDelivery {
	return provider.ArtifactDelivery{
		Version:     delivery.Version,
		Paths:       append([]string{}, delivery.Paths...),
		DeliveredAt: delivery.DeliveredAt,
	}
}

func (c *Controller) setCompletedArtifactDelivery(delivery provider.ArtifactDelivery) {
	value := cloneCompletedArtifactDelivery(delivery)
	c.completedArtifactDelivery.Store(&value)
}

func (c *Controller) takeCompletedArtifactDelivery() *provider.ArtifactDelivery {
	delivery := c.completedArtifactDelivery.Swap(nil)
	if delivery == nil {
		return nil
	}
	value := cloneCompletedArtifactDelivery(*delivery)
	return &value
}

func (c *Controller) runBoundTaskLifecycle(ctx context.Context, run func(context.Context) error) error {
	if c.taskLifecycle == nil {
		return run(ctx)
	}
	return c.taskLifecycle.RunTask(ctx, func(ctx context.Context) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("internal error: %v", recovered)
			}
		}()
		return run(ctx)
	})
}

func (c *Controller) commitArtifactDelivery(artifactTurnID string, _ *agent.WorkspaceArtifactObservation) (provider.ArtifactDelivery, bool, error) {
	delivery, ok := c.buildArtifactDelivery(artifactTurnID, time.Now().UnixMilli())
	if !ok {
		return provider.ArtifactDelivery{}, false, nil
	}
	path := c.SessionPath()
	if path == "" {
		if c.SessionDir() != "" && len(delivery.Paths) > 0 {
			return provider.ArtifactDelivery{}, false, errNoSessionPath
		}
		// A headless controller with persistence explicitly disabled, or a
		// path-less transitional turn with no files, still owns a coherent
		// in-memory suppression contract for the duration of the process.
		committed, committedOK := c.executor.Session().CommitArtifactDelivery(artifactTurnID, delivery)
		return committed, committedOK, nil
	}
	committed, committedOK, err := c.executor.Session().CommitArtifactDeliveryAndSave(path, artifactTurnID, delivery)
	if err != nil {
		return provider.ArtifactDelivery{}, false, fmt.Errorf("persist artifact delivery transcript: %w", err)
	}
	return committed, committedOK, nil
}

// persistUncommittedArtifactFinal fail-closes an agent final answer when the
// enclosing task did not commit successfully. The candidate starts uncommitted;
// the mutation also repairs any older/custom Runner that reached a committed
// state. Saving only the atomic JSONL transcript deliberately separates this
// acceptance boundary from auxiliary sidecar failures.
func (c *Controller) persistUncommittedArtifactFinal(artifactTurnID string, cause error) error {
	if cause == nil || c.executor == nil {
		return cause
	}
	path := c.SessionPath()
	if path == "" {
		if c.SessionDir() == "" {
			c.executor.Session().DowngradeArtifactDeliveryFinal(artifactTurnID)
			return cause
		}
		return errors.Join(cause, errNoSessionPath)
	}
	_, err := c.executor.Session().DowngradeArtifactDeliveryFinalAndSave(path, artifactTurnID)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("persist uncommitted final transcript: %w", err))
	}
	return cause
}

func (c *Controller) recordCompletedTurnMetadata(ctx context.Context, startMessages int, toolMaaSQuota, toolChargedCredits string) {
	task, ok := modelbilling.TaskContextFrom(ctx)
	if !ok || c.executor == nil {
		return
	}
	metadata := provider.TurnMetadata{
		TaskID:             strings.TrimSpace(task.TaskID),
		ModelRef:           strings.TrimSpace(c.billingRouteSelection.ModelRef),
		ModelID:            strings.TrimSpace(task.InitialModelID),
		BillingSource:      strings.TrimSpace(string(task.BillingSource)),
		ToolMaaSQuota:      toolMaaSQuota,
		ToolChargedCredits: toolChargedCredits,
	}
	if metadata.TaskID == "" {
		return
	}
	// TurnDone must retain the task identity even when a failed turn did not
	// append an assistant message. Successful transcripts additionally persist
	// the same metadata on their final assistant message.
	c.executor.Session().AttachTurnMetadata(startMessages, metadata)
	c.mu.Lock()
	c.completedTurn = &metadata
	c.mu.Unlock()
}

func (c *Controller) bindSessionTools(ctx context.Context) (context.Context, error) {
	if c.sessionToolProvider == nil || c.executor == nil {
		return ctx, nil
	}
	c.mu.Lock()
	path := c.sessionPath
	c.mu.Unlock()
	sessionTools, err := c.sessionToolProvider(ctx, path)
	if err != nil {
		return nil, err
	}
	if len(sessionTools) == 0 {
		return ctx, nil
	}
	scoped := tool.NewRegistry()
	if base := c.mcp.registry(); base != nil {
		for _, name := range base.Names() {
			if registered, ok := base.Get(name); ok {
				scoped.Add(registered)
			}
		}
	}
	for _, sessionTool := range sessionTools {
		if sessionTool != nil {
			scoped.Add(sessionTool)
		}
	}
	return agent.WithTaskToolRegistry(ctx, c.executor, scoped), nil
}

func (c *Controller) authorizeSessionSend(ctx context.Context) error {
	c.mu.Lock()
	path := c.sessionPath
	blockedPath := c.sessionPrepareBlockedPath
	c.mu.Unlock()
	if blockedPath != "" && blockedPath == path {
		return fmt.Errorf("session preparation rollback failed; switch sessions before sending")
	}
	if c.sessionSendGuard == nil {
		return nil
	}
	return c.sessionSendGuard(ctx, path)
}

// Run executes a turn synchronously, returning the agent's error. Used by the
// headless `kwu run` path, where the Sink renders to stdout and the caller
// just needs the exit status — no TurnDone event, no cancel bookkeeping.
func (c *Controller) Run(ctx context.Context, input string) (err error) {
	c.sessionLifecycleMu.RLock()
	defer c.sessionLifecycleMu.RUnlock()
	ctx, cancel := context.WithCancel(ctx)
	runtimeTurnID := event.NewTurnID()
	ctx = event.WithTurnID(ctx, runtimeTurnID)
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		cancel()
		return ErrTurnRunning
	}
	c.cancel = cancel
	c.running = true
	c.canceling = false
	c.completedTurn = nil
	c.completedArtifactDelivery.Store(nil)
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		c.completedTurn = nil
		c.completedArtifactDelivery.Store(nil)
		c.running = false
		c.cancel = nil
		c.canceling = false
		c.mu.Unlock()
	}()
	ctx, err = c.bindTaskContext(ctx)
	if err != nil {
		return err
	}
	return c.runTaskLifecycle(ctx, func(ctx context.Context) (runErr error) {
		parentSession := c.parentSessionID()
		ctx = agent.WithParentSession(ctx, parentSession)
		ctx = agent.WithExpertTeamRuntime(ctx, controllerExpertTeamRuntime{c: c})
		ctx = jobs.WithSession(ctx, parentSession)
		ctx = agent.WithUserImages(ctx, c.inputImages(input))
		input = c.Compose(input)
		startMessages := c.messageCount()
		defer c.snapshotActivityIfChanged(startMessages)
		if c.guardianSess != nil {
			c.guardianSess.ResetTurn()
		}
		runErr = c.runRootRunner(ctx, input)
		return runErr
	})
}
