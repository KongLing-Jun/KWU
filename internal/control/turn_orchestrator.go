package control

import (
	"context"
	"errors"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/jobs"
	"kwu/internal/tool"
)

func (c *Controller) runCapabilityTurn(ctx context.Context, turn CapabilityTurn) error {
	baseRegistry := agent.TaskToolRegistry(ctx, c.mcp.registry())
	turnRegistry := registryForMCPServers(baseRegistry, turn.MCPServerNames)
	if turn.RestrictTools {
		turnRegistry = registryForCapabilities(baseRegistry, turn.MCPServerNames, turn.ToolAllowlist)
	}
	if c.executor != nil {
		ctx = agent.WithTaskToolRegistry(ctx, c.executor, turnRegistry)
	}
	input, rawInput, err := c.expandCapabilityTurnInput(turn, turnRegistry)
	if err != nil {
		return err
	}
	refInput := turn.Display
	if strings.TrimSpace(refInput) == "" {
		refInput = turn.Input
	}
	block, refErrors := c.ResolveRefs(ctx, refInput)
	for _, refErr := range refErrors {
		c.notice(refErr)
	}
	if block != "" {
		input = "Referenced context:\n\n" + block + "\n\n" + input
	}
	if turn.PreferTitleText {
		ctx = context.WithValue(ctx, acceptedTitleTextContextKey{}, acceptedTitleText{text: turn.TitleText})
	}
	return newTurnOrchestrator(c).runGoalLoopWithRawDisplay(ctx, input, rawInput, turn.Display)
}

func (c *Controller) expandCapabilityTurnInput(turn CapabilityTurn, turnRegistry *tool.Registry) (string, string, error) {
	input, rawInput := turn.Input, turn.Input
	if expanded, ok := c.expandCapabilityCommand(input); ok {
		input, rawInput = expanded, expanded
	}
	if len(turn.SkillNames) > 0 {
		expanded, err := c.expandSelectedSkills(input, turn.SkillNames)
		if err != nil {
			return "", "", err
		}
		input = expanded
	}
	if connectorContext := selectedMCPContext(turn.MCPServerNames, turnRegistry); connectorContext != "" {
		input = connectorContext + input
	}
	return input, rawInput, nil
}

// turnOrchestrator owns foreground turn execution while Controller keeps the
// public ports, run-state guard, and session-scoped dependencies.
type turnOrchestrator struct {
	c *Controller
}

type syntheticTurnContextKey struct{}
type acceptedTitleTextContextKey struct{}

type acceptedTitleText struct {
	text string
}

func newTurnOrchestrator(c *Controller) *turnOrchestrator {
	return &turnOrchestrator{c: c}
}

func (o *turnOrchestrator) runTurnWithRawDisplay(ctx context.Context, input, raw, display string) (err error) {
	c := o.c
	c.mu.Lock()
	c.turn++
	c.mu.Unlock()
	title, preferTitleText := ctx.Value(acceptedTitleTextContextKey{}).(acceptedTitleText)
	c.notifyAcceptedUserTurn(ctx, raw, display, title.text, preferTitleText)
	if err := c.maybeAutoPlan(ctx, raw); err != nil {
		return err
	}
	parentSession := c.parentSessionID()
	ctx = agent.WithParentSession(ctx, parentSession)
	ctx = agent.WithExpertTeamRuntime(ctx, controllerExpertTeamRuntime{c: c})
	ctx = jobs.WithSession(ctx, parentSession)
	ctx = agent.WithUserImages(ctx, c.inputImages(input))
	input = c.Compose(input)
	startMessages := c.messageCount()
	defer c.snapshotActivityIfChanged(startMessages)
	defer c.recordDisplayForNewUser(startMessages, display)
	// Open a checkpoint for this turn before the user message is appended, so the
	// recorded message boundary precedes it and pre-edit snapshots land here.
	c.beginCheckpoint(input)
	c.resetGuardianTurn()
	if err = c.runRootRunner(ctx, input); err != nil {
		// Preserve the messages already shown to the user and append a durable
		// interruption boundary for the next model turn. A stop is not a rewind.
		if errors.Is(err, context.Canceled) && c.CancelRequested() {
			c.markTurnInterrupted(startMessages)
		}
		return err
	}
	c.mu.Lock()
	plan := c.planMode
	c.mu.Unlock()
	if !plan {
		return nil
	}
	proposal := lastAssistantText(c.History())
	if proposal == "" {
		return nil // no substantive proposal to gate
	}
	c.mu.Lock()
	planID := c.planID
	c.mu.Unlock()
	c.emitPlanLifecycle(planID, event.PlanProposed, "proposed", "started")
	// The plan is already visible as the assistant's answer, so the request
	// carries no subject — it's purely the gate.
	allow, _, err := c.requestApproval(ctx, planApprovalTool, "", nil)
	if err != nil {
		return err
	}
	if !allow {
		c.emitPlanLifecycle(planID, event.PlanRevising, "revising", "proposed")
		return nil // keep planning; plan mode stays on
	}
	c.emitPlanLifecycle(planID, event.PlanApproved, "approved", "proposed")
	c.SetPlanMode(false)
	todoArgs := c.seedPlanTodos(proposal)
	execStart := c.sessionMessageCount()
	// The plan is the go-ahead: don't re-prompt for each write of the approved
	// work. Auto-approve writers for the duration of this execution turn only; a
	// later turn (even "continue") falls back to the normal per-tool approval.
	c.approval.setPlanAutoApprove(true)
	defer c.approval.setPlanAutoApprove(false)
	executionCtx := c.syntheticExecutionContext(ctx)
	if err = c.runRootRunner(executionCtx, c.ComposeSynthetic(planApprovedMessage)); err != nil {
		if errors.Is(err, context.Canceled) && c.CancelRequested() {
			c.markTurnInterrupted(execStart)
		}
		return err
	}
	if todoArgs != "" && !c.hasTodoUpdateSince(execStart) {
		c.completePlanTodos(todoArgs)
	}
	return nil
}

func (c *Controller) resetGuardianTurn() {
	if c.guardianSess != nil {
		c.guardianSess.ResetTurn()
	}
}

func (c *Controller) syntheticExecutionContext(ctx context.Context) context.Context {
	if c.executorStreamDispatcher == nil {
		return ctx
	}
	return agent.WithStreamDispatcher(ctx, c.executor, c.executorStreamDispatcher)
}

func (o *turnOrchestrator) runGoalLoopWithRawDisplay(ctx context.Context, input, raw, display string) error {
	if err := o.runTurnWithRawDisplay(ctx, input, raw, display); err != nil {
		if ctx.Err() != nil {
			o.c.stopGoal(GoalStatusStopped)
		}
		return err
	}
	return o.continueGoal(ctx)
}

func (o *turnOrchestrator) continueGoal(ctx context.Context) error {
	c := o.c
	for {
		cont := o.advanceGoalAfterTurn()
		if !cont {
			return nil
		}
		if err := ctx.Err(); err != nil {
			c.stopGoal(GoalStatusStopped)
			return err
		}
		turn := goalContinueTurn
		if msg, ok := c.goals.takeIntercept(); ok {
			turn = msg
			if evidence.IsHardReadiness(msg) {
				c.notice("goal intercept: hard requirement cannot be overridden; satisfy it before signaling [goal:complete]")
			} else {
				c.notice("goal intercept: incomplete todos remain (override with a second [goal:complete])")
			}
		}
		syntheticCtx := context.WithValue(ctx, syntheticTurnContextKey{}, true)
		if err := o.runTurnWithRawDisplay(syntheticCtx, turn, turn, ""); err != nil {
			if ctx.Err() != nil {
				c.stopGoal(GoalStatusStopped)
			}
			return err
		}
	}
}

func (o *turnOrchestrator) advanceGoalAfterTurn() bool {
	c := o.c
	// Gather every input the FSM needs off the goal lock: parse the marker,
	// snapshot the executor's todos + readiness, and check tool activity. None
	// of these touch goal state, so the machine's critical section stays pure.
	status, reason, _ := parseGoalStatusMarker(lastAssistantText(c.History()))
	var readiness string
	if c.executor != nil {
		readiness = c.executor.GoalReadinessFailure()
	}
	res := c.goals.advance(goalAdvanceInput{
		status:     status,
		reason:     reason,
		toolCalled: c.toolWasCalledLastTurn(),
		todos:      c.goalTodos(),
		readiness:  readiness,
	})
	c.persistGoalState(res.path, res.data, res.ok)
	if res.notice != "" {
		c.notice(res.notice)
	}
	return res.cont
}
