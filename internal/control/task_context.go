package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/fileutil"
	"kwu/internal/modelbilling"
	"kwu/internal/project"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

const (
	projectContextOpen  = "<project-context-snapshot>"
	projectContextClose = "</project-context-snapshot>"
)

type ProjectContextInput struct {
	ProjectID        string
	HumanDescription string
	AgentInstruction string
	Version          uint64
	ContentHash      string
}

type projectContextEnvelope struct {
	ProjectID        string `json:"project_id"`
	HumanDescription string `json:"human_description"`
	AgentInstruction string `json:"agent_instruction"`
}

func BuildProjectContextMessage(input ProjectContextInput) (provider.Message, bool, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" || projectID != input.ProjectID {
		return provider.Message{}, false, fmt.Errorf("control: invalid project_id")
	}
	if input.Version == 0 {
		return provider.Message{}, false, fmt.Errorf("control: instruction version must be positive")
	}
	if input.ContentHash != project.InstructionContentSHA256(input.HumanDescription, input.AgentInstruction) {
		return provider.Message{}, false, fmt.Errorf("control: project instruction content hash mismatch")
	}
	if input.HumanDescription == "" && input.AgentInstruction == "" {
		return provider.Message{}, false, nil
	}
	payload, err := json.Marshal(projectContextEnvelope{ProjectID: projectID, HumanDescription: input.HumanDescription, AgentInstruction: input.AgentInstruction})
	if err != nil {
		return provider.Message{}, false, err
	}
	message := provider.Message{Role: provider.RoleUser, Content: projectContextOpen + "\n" + string(payload) + "\n" + projectContextClose, MessageKind: provider.MessageKindProjectContext, Metadata: &provider.MessageMetadata{Source: provider.ProjectContextSourceCloudInstruction, Version: input.Version, ContentHash: input.ContentHash}}
	return message, true, message.ValidateProjectContext()
}

func (c *Controller) AppendProjectContext(message provider.Message) error {
	if c == nil {
		return fmt.Errorf("control: project context requires an active session")
	}
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	return c.appendProjectContextLocked(message)
}

func (c *Controller) appendProjectContextLocked(message provider.Message) error {
	if err := ValidateProjectContextMessage(message); err != nil {
		return err
	}
	if c.executor == nil || c.executor.Session() == nil {
		return fmt.Errorf("control: project context requires an active session")
	}
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	if running {
		return ErrTurnRunning
	}
	for _, existing := range c.executor.Session().Snapshot() {
		if existing.IsProjectContext() {
			return fmt.Errorf("control: project context already exists")
		}
		if existing.Role != provider.RoleSystem {
			return fmt.Errorf("control: project context must precede the first task")
		}
	}
	c.executor.Session().Add(message)
	return nil
}

type ProjectContextTransaction struct {
	ExpectedPath string
	Message      provider.Message
	Present      bool
	CommitMeta   func() error
	RollbackMeta func() error
}

// PrepareProjectContextTransaction owns the target Controller's session
// lifecycle while validating the expected blank session, committing its pin,
// appending the typed context and persisting the transcript. NewSession,
// Resume, SetSessionPath and every turn share sessionLifecycleMu, so none can
// change the path/history between validation and persistence.
func (c *Controller) PrepareProjectContextTransaction(input ProjectContextTransaction) error {
	if c == nil {
		return fmt.Errorf("control: project context requires an active session")
	}
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	preparation, err := c.newProjectContextPreparation(input)
	if err != nil {
		return err
	}
	return preparation.commit()
}

type projectContextPreparation struct {
	controller       *Controller
	input            ProjectContextTransaction
	expectedPath     string
	targetSession    *agent.Session
	targetGeneration string
	previous         []provider.Message
	transcriptBefore []byte
	transcriptExists bool
}

func (c *Controller) newProjectContextPreparation(input ProjectContextTransaction) (*projectContextPreparation, error) {
	expectedPath, err := c.validateProjectContextTransaction(input)
	if err != nil {
		return nil, err
	}
	if err := validateBlankProjectSession(c.executor.Session().Snapshot()); err != nil {
		return nil, err
	}
	// A project context is conversation content, so a selectable Draft scene
	// must be committed while the session is still blank. The normal first-turn
	// boundary remains an idempotent no-op after this transaction.
	if err := c.commitDraftScene(); err != nil {
		return nil, err
	}
	targetSession := c.executor.Session()
	targetGeneration, err := projectSessionGeneration(targetSession)
	if err != nil {
		return nil, err
	}
	previous := targetSession.Snapshot()
	if err := validateBlankProjectSession(previous); err != nil {
		return nil, err
	}
	transcriptBefore, transcriptExists, err := readProjectTranscript(expectedPath)
	if err != nil {
		return nil, err
	}
	return &projectContextPreparation{
		controller: c, input: input, expectedPath: expectedPath,
		targetSession: targetSession, targetGeneration: targetGeneration,
		previous: previous, transcriptBefore: transcriptBefore, transcriptExists: transcriptExists,
	}, nil
}

func (c *Controller) validateProjectContextTransaction(input ProjectContextTransaction) (string, error) {
	if c.executor == nil || c.executor.Session() == nil {
		return "", fmt.Errorf("control: project context requires an active session")
	}
	expectedPath := strings.TrimSpace(input.ExpectedPath)
	if expectedPath == "" || expectedPath != input.ExpectedPath {
		return "", fmt.Errorf("control: expected project session path is invalid")
	}
	c.mu.Lock()
	running := c.running
	currentPath := c.sessionPath
	c.mu.Unlock()
	if running {
		return "", ErrTurnRunning
	}
	if currentPath != expectedPath {
		return "", fmt.Errorf("control: project session path changed before commit")
	}
	if input.Present {
		if err := ValidateProjectContextMessage(input.Message); err != nil {
			return "", err
		}
	}
	if input.CommitMeta == nil {
		return "", fmt.Errorf("control: project context metadata commit is required")
	}
	return expectedPath, nil
}

func validateBlankProjectSession(messages []provider.Message) error {
	for _, message := range messages {
		if message.Role != provider.RoleSystem {
			return fmt.Errorf("control: project context requires a blank session")
		}
	}
	return nil
}

func (p *projectContextPreparation) commit() error {
	if err := p.input.CommitMeta(); err != nil {
		return err
	}
	if err := p.validateIdentity(p.targetGeneration); err != nil {
		return p.rollback(err, false, false)
	}
	if !p.input.Present {
		p.controller.clearProjectSessionPreparationBlock(p.expectedPath)
		return nil
	}
	p.targetSession.Add(p.input.Message)
	appendedGeneration, err := projectSessionGeneration(p.targetSession)
	if err != nil {
		return p.rollback(err, false, true)
	}
	if appendedGeneration == p.targetGeneration {
		return p.rollback(fmt.Errorf("control: project session generation did not advance"), false, true)
	}
	if err := p.validateIdentity(appendedGeneration); err != nil {
		return p.rollback(err, false, true)
	}
	if err := p.controller.Snapshot(); err != nil {
		return p.rollback(err, true, true)
	}
	if err := p.validateIdentity(appendedGeneration); err != nil {
		return p.rollback(err, true, true)
	}
	p.controller.clearProjectSessionPreparationBlock(p.expectedPath)
	return nil
}

func (p *projectContextPreparation) validateIdentity(generation string) error {
	return p.controller.validateProjectSessionIdentity(p.targetSession, p.expectedPath, generation)
}

func (p *projectContextPreparation) rollback(cause error, restoreTranscript, restoreMemory bool) error {
	return p.controller.rollbackProjectContextPreparation(
		cause, p.input.RollbackMeta, p.expectedPath, p.targetSession, p.previous,
		p.transcriptBefore, p.transcriptExists, restoreTranscript, restoreMemory,
	)
}

func projectSessionGeneration(session *agent.Session) (string, error) {
	if session == nil {
		return "", fmt.Errorf("control: project session is unavailable")
	}
	_, _, _, generation, err := session.SnapshotRangeWithGeneration(0, 0, "")
	return generation, err
}

func (c *Controller) validateProjectSessionIdentity(target *agent.Session, expectedPath, expectedGeneration string) error {
	c.mu.Lock()
	currentPath := c.sessionPath
	c.mu.Unlock()
	if currentPath != expectedPath || c.executor == nil || c.executor.Session() != target {
		return fmt.Errorf("control: project session identity changed before commit")
	}
	generation, err := projectSessionGeneration(target)
	if err != nil {
		return err
	}
	if generation != expectedGeneration {
		return fmt.Errorf("control: project session generation changed before commit")
	}
	return nil
}

func (c *Controller) rollbackProjectContextPreparation(
	cause error,
	rollbackMeta func() error,
	expectedPath string,
	targetSession *agent.Session,
	previous []provider.Message,
	transcriptBefore []byte,
	transcriptExisted bool,
	restoreTranscriptFile bool,
	restoreMemory bool,
) error {
	if restoreMemory && targetSession != nil {
		targetSession.Replace(previous)
	}
	var transcriptErr error
	if restoreTranscriptFile {
		transcriptErr = restoreProjectTranscript(expectedPath, transcriptBefore, transcriptExisted)
	}
	var metaErr error
	if rollbackMeta == nil {
		metaErr = fmt.Errorf("control: project context metadata rollback is required")
	} else {
		metaErr = rollbackMeta()
	}
	rollbackErr := errors.Join(transcriptErr, metaErr)
	if rollbackErr != nil {
		c.mu.Lock()
		c.sessionPrepareBlockedPath = expectedPath
		c.mu.Unlock()
	}
	return errors.Join(cause, rollbackErr)
}

func (c *Controller) clearProjectSessionPreparationBlock(path string) {
	c.mu.Lock()
	if c.sessionPrepareBlockedPath == path {
		c.sessionPrepareBlockedPath = ""
	}
	c.mu.Unlock()
}

func readProjectTranscript(path string) ([]byte, bool, error) {
	payload, err := os.ReadFile(path)
	if err == nil {
		return payload, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, err
}

func restoreProjectTranscript(path string, payload []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return fileutil.AtomicWriteFileStrict(path, payload, 0o600)
}

// ValidatePinnedProjectContext proves that a pinned-unsent transcript is the
// exact typed snapshot named by BranchMeta, not merely a session with no normal
// user task. Empty instructions are represented by the canonical empty hash and
// exactly zero context records.
func ValidatePinnedProjectContext(messages []provider.Message, projectID string, version uint64, contentHash string) error {
	contextMessage, err := singlePinnedProjectContext(messages)
	if err != nil {
		return err
	}
	expectContext := contentHash != project.InstructionContentSHA256("", "")
	if expectContext && contextMessage == nil {
		return fmt.Errorf("control: pinned project context is missing")
	}
	if !expectContext && contextMessage != nil {
		return fmt.Errorf("control: empty pinned project context must not have a transcript record")
	}
	if contextMessage == nil {
		return nil
	}
	return validatePinnedProjectContextMessage(*contextMessage, projectID, version, contentHash)
}

func singlePinnedProjectContext(messages []provider.Message) (*provider.Message, error) {
	var contextMessage *provider.Message
	for _, message := range messages {
		if message.Role == provider.RoleSystem {
			continue
		}
		if !message.IsProjectContext() {
			return nil, fmt.Errorf("control: pinned project session contains an ordinary task")
		}
		if contextMessage != nil {
			return nil, fmt.Errorf("control: pinned project session contains multiple project contexts")
		}
		if err := ValidateProjectContextMessage(message); err != nil {
			return nil, err
		}
		copy := message
		contextMessage = &copy
	}
	return contextMessage, nil
}

func validatePinnedProjectContextMessage(message provider.Message, projectID string, version uint64, contentHash string) error {
	encoded, err := projectContextEnvelopeJSON(message.Content)
	if err != nil {
		return err
	}
	payload, err := decodeProjectContextEnvelope(encoded)
	if err != nil {
		return err
	}
	if payload.ProjectID != projectID || message.Metadata.Version != version || message.Metadata.ContentHash != contentHash {
		return fmt.Errorf("control: pinned project context does not match metadata")
	}
	return nil
}

// ValidatePinnedProjectContextSession applies the same session lifecycle lock
// used by project preparation and turns, so reuse is proven against the exact
// expected controller path at one instant.
func (c *Controller) ValidatePinnedProjectContextSession(expectedPath, projectID string, version uint64, contentHash string) error {
	if c == nil {
		return fmt.Errorf("control: project context requires an active session")
	}
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	c.mu.Lock()
	running := c.running
	currentPath := c.sessionPath
	c.mu.Unlock()
	if running {
		return ErrTurnRunning
	}
	if currentPath != expectedPath {
		return fmt.Errorf("control: project session path changed before reuse")
	}
	return ValidatePinnedProjectContext(c.History(), projectID, version, contentHash)
}

func ValidateProjectContextMessage(message provider.Message) error {
	if err := message.ValidateProjectContext(); err != nil {
		return err
	}
	return validateProjectContextEnvelope(message)
}

func validateProjectContextEnvelope(message provider.Message) error {
	encoded, err := projectContextEnvelopeJSON(message.Content)
	if err != nil {
		return err
	}
	payload, err := decodeProjectContextEnvelope(encoded)
	if err != nil {
		return err
	}
	if err := validateProjectContextEnvelopePayload(payload); err != nil {
		return err
	}
	if project.InstructionContentSHA256(payload.HumanDescription, payload.AgentInstruction) != message.Metadata.ContentHash {
		return fmt.Errorf("control: project context envelope hash mismatch")
	}
	return nil
}

func projectContextEnvelopeJSON(content string) (string, error) {
	prefix, suffix := projectContextOpen+"\n", "\n"+projectContextClose
	canonical := strings.HasPrefix(content, prefix) && strings.HasSuffix(content, suffix) && strings.Count(content, projectContextOpen) == 1 && strings.Count(content, projectContextClose) == 1
	if !canonical {
		return "", fmt.Errorf("control: project context envelope is not canonical")
	}
	return strings.TrimSuffix(strings.TrimPrefix(content, prefix), suffix), nil
}

func decodeProjectContextEnvelope(encoded string) (projectContextEnvelope, error) {
	var payload projectContextEnvelope
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("control: decode project context envelope: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return payload, fmt.Errorf("control: project context envelope has trailing data")
	}
	canonical, err := json.Marshal(payload)
	if err != nil || !bytes.Equal(canonical, []byte(encoded)) {
		return payload, fmt.Errorf("control: project context envelope is not canonical JSON")
	}
	return payload, nil
}

func validateProjectContextEnvelopePayload(payload projectContextEnvelope) error {
	if strings.TrimSpace(payload.ProjectID) == "" || payload.ProjectID != strings.TrimSpace(payload.ProjectID) {
		return fmt.Errorf("control: invalid project context project_id")
	}
	if payload.HumanDescription == "" && payload.AgentInstruction == "" {
		return fmt.Errorf("control: empty project context must not be appended")
	}
	return nil
}

// TaskContextInitRequest contains only controller-owned facts. The initializer
// owns task creation, pricing evidence, and billing-source selection.
type TaskContextInitRequest struct {
	Actor                 userctx.ActorContext
	BillingRouteSelection modelbilling.BillingRouteSelection
}

// Compact runs one compaction pass on the executor's session on demand.
// instructions is optional `/compact <focus>` guidance steering what to keep.
func (c *Controller) Compact(ctx context.Context, instructions string) (err error) {
	// Preserve the command contract: an already-running foreground turn rejects
	// manual compaction immediately instead of waiting behind its lifecycle read
	// lock. The in-lock check below still closes the race with a new turn.
	if c.requireTaskContext && c.Running() {
		return ErrTurnRunning
	}
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	if c.executor == nil {
		return nil
	}
	session := c.executor.Session()
	compactionVersion := uint64(0)
	if session != nil {
		compactionVersion = session.CompactionVersion()
	}
	defer c.invalidateCheckpointBoundsAfterCompaction(session, compactionVersion)
	return c.runManualModelTask(ctx, func(ctx context.Context) error {
		return c.executor.CompactNow(ctx, instructions)
	})
}

// runManualModelTask owns the fresh billing task and cancellable foreground
// state for synchronous, user-initiated model work outside an ordinary turn.
// Callers must hold sessionLifecycleMu so the selected session cannot change.
func (c *Controller) runManualModelTask(ctx context.Context, run func(context.Context) error) (err error) {
	if !c.requireTaskContext {
		if err := c.authorizeSessionSend(ctx); err != nil {
			return err
		}
		return run(ctx)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		cancel()
		return ErrTurnRunning
	}
	c.cancel = cancel
	c.running = true
	c.canceling = false
	c.mu.Unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("internal error: %v", recovered)
		}
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		c.canceling = false
		c.mu.Unlock()
		cancel()
	}()

	ctx, err = c.initializeTaskContext(ctx)
	if err != nil {
		return err
	}
	return c.runTaskLifecycle(ctx, run)
}

// TaskContextInitializer returns a persisted task and its bootstrap evidence.
// It must not dispatch a model call.
type TaskContextInitializer interface {
	InitializeTaskContext(context.Context, TaskContextInitRequest) (modelbilling.TaskContext, error)
}

func (c *Controller) bindTaskContext(ctx context.Context) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if task, ok := modelbilling.TaskContextFrom(ctx); ok {
		if err := validateControllerTaskContext(task, c.actor); err != nil {
			return nil, err
		}
		return tool.WithCommandEnvProviderForTask(ctx, task.TaskID, c.commandEnvProvider), nil
	}
	bound, err := c.initializeTaskContext(ctx)
	if err != nil {
		return nil, err
	}
	task, ok := modelbilling.TaskContextFrom(bound)
	if !ok {
		return tool.WithCommandEnvProvider(bound, c.commandEnvProvider), nil
	}
	return tool.WithCommandEnvProviderForTask(bound, task.TaskID, c.commandEnvProvider), nil
}

func (c *Controller) bindCapabilityTaskContext(ctx context.Context, input map[string]string) (context.Context, error) {
	commandEnv, err := tool.ValidateCommandEnv(input)
	if err != nil {
		return nil, err
	}
	ctx, err = c.bindTaskContext(ctx)
	if err != nil {
		return nil, err
	}
	return tool.WithCommandEnv(ctx, commandEnv), nil
}

// initializeTaskContext always asks the initializer for a new persisted task.
// Manual compaction uses this seam so a caller context cannot revive the task
// from an already-finished user turn.
func (c *Controller) initializeTaskContext(ctx context.Context) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, operationalScope, err := c.withOperationalTaskScope(ctx)
	if err != nil {
		return nil, err
	}
	if c.taskContextInitializer == nil {
		if c.requireTaskContext {
			return nil, newTaskContextRequiredError()
		}
		return ctx, nil
	}
	task, err := c.taskContextInitializer.InitializeTaskContext(ctx, TaskContextInitRequest{
		Actor: c.actor, BillingRouteSelection: c.billingRouteSelection,
	})
	if err != nil {
		return nil, err
	}
	if err := validateControllerTaskContext(task, c.actor); err != nil {
		return nil, err
	}
	ctx, err = modelbilling.WithNewTaskContext(ctx, task)
	if err != nil {
		return nil, err
	}
	operationalScope.TaskID = task.TaskID
	return event.WithOperationalScope(ctx, operationalScope), nil
}

func (c *Controller) withOperationalTaskScope(ctx context.Context) (context.Context, event.OperationalScope, error) {
	if _, ok := event.OperationalEmitterFrom(ctx); !ok {
		ctx = event.WithOperationalEmitter(ctx, c.operational)
	}
	operationalScope := event.OperationalScopeFrom(ctx)
	if operationalScope.SessionID == "" {
		operationalScope.SessionID = c.parentSessionID()
	}
	turnID, err := selectControllerTurnID(operationalScope.TurnID, event.TurnIDFromContext(ctx))
	if err != nil {
		return nil, event.OperationalScope{}, err
	}
	operationalScope.TurnID = turnID
	c.mu.Lock()
	if c.planMode && operationalScope.PlanID == "" {
		operationalScope.PlanID = c.planID
	}
	c.mu.Unlock()
	ctx = event.WithOperationalScope(ctx, operationalScope)
	return ctx, operationalScope, nil
}

func selectControllerTurnID(scopeID, contextID string) (string, error) {
	if scopeID != "" && contextID != "" && scopeID != contextID {
		return "", fmt.Errorf("control: operational turn identity %q conflicts with controller turn identity %q", scopeID, contextID)
	}
	if scopeID != "" {
		return scopeID, nil
	}
	if contextID != "" {
		return contextID, nil
	}
	return event.NewTurnID(), nil
}

func validateControllerTaskContext(task modelbilling.TaskContext, actor userctx.ActorContext) error {
	if _, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	}); err != nil {
		return err
	}
	if task.Scope.ActorID != actor.ActorID || task.Scope.ProfileID != actor.ProfileID {
		return newTaskContextScopeError()
	}
	return nil
}

func newTaskContextRequiredError() error {
	return modelbillingError(modelbilling.ErrorContractInvalid, "task context is required before model entry")
}

func newTaskContextScopeError() error {
	return modelbillingError(modelbilling.ErrorScopeMismatch, "task context actor or profile does not match controller")
}

func modelbillingError(code modelbilling.ErrorCode, message string) error {
	return &modelbilling.Error{Code: code, Message: message}
}
