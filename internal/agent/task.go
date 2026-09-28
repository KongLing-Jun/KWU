package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"kwu/internal/event"
	"kwu/internal/jobs"
	"kwu/internal/planmode"
	"kwu/internal/provider"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

// DefaultTaskSystemPrompt steers a sub-agent toward focused, terse delivery —
// it doesn't see the parent's conversation so it must self-contain.
const DefaultTaskSystemPrompt = `You are a sub-agent invoked by a parent coding agent to carry out one focused task.
Use the provided tools to investigate or act. Return a single final answer that is concise
and self-contained — the parent will see only that answer, not your tool calls or reasoning.
Any local file intended as a deliverable must exist in the workspace and be declared in the final response with a Markdown link to the actual workspace file path. Inline content in a code block or instructions that ask the user to save it are not delivery. The parent must finish all writing, testing, review, and background waits before sign-off.
When invoked as a configured expert-team member, communicate only with the parent lead through the provided A2A task. Never address, delegate to, or exchange messages with sibling experts. Return task outputs and generated files to the parent as A2A artifacts.
If you need to ask for clarification, fail with a precise question instead of guessing.`

// DefaultReadOnlyTaskSystemPrompt steers read-only sub-agents toward isolated
// research. They never receive writer tools, persisted transcript controls, or
// background process controls, so their final answer is the only handoff.
const DefaultReadOnlyTaskSystemPrompt = `You are a read-only research sub-agent invoked by a parent coding agent.
Use only the provided read-only tools to inspect code, docs, history, and safe shell output.
Do not attempt to write files, install capabilities, mutate memory, control long-lived
processes, or delegate to another agent. Return a concise, self-contained final answer
with the evidence the parent needs.`

var subagentMetaTools = []string{
	"task",
	"wait_task",
	"stop_task",
	"read_only_task",
	"parallel_tasks",
	"run_skill",
	"read_only_skill",
	"read_skill",
	"install_skill",
	"install_source",
	"explore",
	"research",
	"review",
	"security_review",
	// Final delivery belongs to the root agent after it has collected and
	// reviewed every child result.
	"deliver_files",
}

var subagentJobTools = []string{
	"wait",
	"bash_output",
	"kill_shell",
}

var readOnlySubagentWorkflowTools = []string{
	"connect_tool_source",
}

const subagentToolBoundarySummary = "Recursive agent/skill tools and unsupported background job tools (wait, bash_output, kill_shell) are excluded; bash is exposed as foreground-only inside subagents."

// SubagentMetaTools returns the recursive/meta and root-only workflow tools that
// spawned agents should not inherit from the parent registry unless a future
// call site deliberately opts into a different boundary.
func SubagentMetaTools() []string {
	out := make([]string, len(subagentMetaTools))
	copy(out, subagentMetaTools)
	return out
}

// SubagentToolRegistry returns the tool set exposed inside spawned sub-agents:
// the requested whitelist (or every parent tool), minus meta tools that would
// spawn more agent work and job tools whose runtime manager is not injected into
// sub-agents. When bash is present, it is wrapped to advertise and allow only
// foreground execution.
func SubagentToolRegistry(parent *tool.Registry, names []string) *tool.Registry {
	exclude := append(SubagentMetaTools(), subagentJobTools...)
	sub := FilterRegistry(parent, normalizeSubagentToolNames(parent, names), exclude...)
	if bash, ok := sub.Get("bash"); ok {
		sub.Add(foregroundOnlyBash{inner: bash})
	}
	return sub
}

type foregroundOnlyBash struct {
	inner tool.Tool
}

func (b foregroundOnlyBash) Name() string { return "bash" }

func (b foregroundOnlyBash) Description() string {
	desc := strings.TrimSpace(b.inner.Description())
	if desc == "" {
		desc = "Execute a command in the shell and return combined stdout/stderr."
	}
	desc = strings.Replace(desc, "Execute a command in the shell", "Execute a foreground command in the shell", 1)
	return desc + " Background execution is unavailable inside subagents."
}

func (foregroundOnlyBash) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Shell command to execute in the foreground"}},"required":["command"]}`)
}

func (b foregroundOnlyBash) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		RunInBackground bool `json:"run_in_background"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if p.RunInBackground {
		return "", fmt.Errorf("background bash is unavailable in subagents; run a foreground command or ask the parent agent to start a background job")
	}
	return b.inner.Execute(ctx, args)
}

func (b foregroundOnlyBash) ReadOnly() bool { return b.inner.ReadOnly() }

type readOnlyBash struct {
	inner tool.Tool
}

func (b readOnlyBash) Name() string { return "bash" }

func (b readOnlyBash) Description() string {
	desc := strings.TrimSpace(b.inner.Description())
	if desc == "" {
		desc = "Execute a command in the shell and return combined stdout/stderr."
	}
	desc = strings.Replace(desc, "Execute a command in the shell", "Execute a foreground read-only command in the shell", 1)
	return desc + " Only plan-mode safe read-only commands are allowed; shell operators, background execution, process preservation, and write-capable arguments are blocked."
}

func (readOnlyBash) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Read-only shell command to execute in the foreground. Must match the plan-mode safe bash policy."}},"required":["command"]}`)
}

func (b readOnlyBash) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	decision := planmode.Policy{}.Decide(planmode.Call{Name: "bash", Args: args})
	if decision.Blocked {
		return decision.Message, nil
	}
	return b.inner.Execute(ctx, args)
}

func (readOnlyBash) ReadOnly() bool { return true }

// TaskTool spawns a sub-agent in its own session for a focused sub-task. The
// sub-agent runs with a filtered tool whitelist and the same step budget shape
// as the parent (see Execute); its tool calls are forwarded to the parent's
// event stream nested under this call, while only its final assistant message is
// returned to the parent model. Use cases: keep noisy tool sequences (multi-file
// exploration, repeated grep / read_file) out of the parent's context budget, or
// parallel research across independent areas (the parallel-dispatch path picks
// these up only when readOnly, which task is not).
type TaskTool struct {
	prov               provider.Provider
	pricing            *provider.Pricing
	parentReg          *tool.Registry
	maxSteps           int
	contextWindow      int
	softCompactRatio   float64
	compactRatio       float64
	compactForceRatio  float64
	recentKeep         int
	temperature        float64
	toolTimeout        time.Duration
	archiveDir         string
	keepPolicy         KeepPolicy
	sysPrompt          string
	gate               Gate
	subagentModel      string
	subagentEffort     string
	subagentToolNames  []string
	readOnlyToolNames  []string
	resolveProvider    func(modelRef, effort string) (provider.Provider, *provider.Pricing, int, error)
	resolveDispatchers func(modelRef, effort string, base provider.Provider) (SubagentStreamDispatchers, error)
	backgroundPolicy   func(context.Context) error
	transcripts        *SubagentStore
	workspaceRoot      string
	baseModel          string
	baseEffort         string
	identityProfile    func(modelRef, effort string) (string, string)
	actor              userctx.ActorContext
}

// SubagentStreamDispatchers assigns independent model-billing call roles to a
// subagent's primary requests and its internal compaction requests.
type SubagentStreamDispatchers struct {
	Main       StreamDispatcher
	Compaction StreamDispatcher
}

type subSessionRuntime struct {
	provider      provider.Provider
	pricing       *provider.Pricing
	contextWindow int
	dispatchers   SubagentStreamDispatchers
}

// NewTaskTool wires a task tool to the parent agent's environment so its
// sub-agents can use the same provider and tools. sysPrompt is the system
// prompt every sub-agent starts with; pass "" for DefaultTaskSystemPrompt. gate
// is the permission gate sub-agents inherit — pass the headless variant so
// deny rules still bite while autonomous sub-agents are never blocked on an
// interactive prompt (there is no UI to answer one).
func NewTaskTool(prov provider.Provider, pricing *provider.Pricing, parentReg *tool.Registry,
	maxSteps, contextWindow, recentKeep int, softCompactRatio, compactRatio, compactForceRatio, temperature float64, archiveDir, sysPrompt string, gate Gate,
	keepPolicy KeepPolicy, subagentModel, subagentEffort string, resolveProvider func(string, string) (provider.Provider, *provider.Pricing, int, error)) *TaskTool {
	if sysPrompt == "" {
		sysPrompt = DefaultTaskSystemPrompt
	}
	return &TaskTool{
		prov:              prov,
		pricing:           pricing,
		parentReg:         parentReg,
		maxSteps:          maxSteps,
		contextWindow:     contextWindow,
		recentKeep:        recentKeep,
		softCompactRatio:  softCompactRatio,
		compactRatio:      compactRatio,
		compactForceRatio: compactForceRatio,
		temperature:       temperature,
		archiveDir:        archiveDir,
		keepPolicy:        keepPolicy,
		sysPrompt:         sysPrompt,
		gate:              gate,
		subagentModel:     subagentModel,
		subagentEffort:    subagentEffort,
		subagentToolNames: SubagentToolRegistry(parentReg, nil).Names(),
		readOnlyToolNames: ReadOnlySubagentToolRegistry(parentReg, nil).Names(),
		resolveProvider:   resolveProvider,
	}
}

// WithToolTimeout applies the same mandatory per-call timeout used by the
// parent agent to tools invoked inside this task's sub-agent.
func (t *TaskTool) WithToolTimeout(timeout time.Duration) *TaskTool {
	t.toolTimeout = timeout
	return t
}

// WithTranscripts enables persisted sub-agent transcript continuation for this
// task tool. The base model/effort are the parent provider identity used when no
// subagent override is configured.
func (t *TaskTool) WithTranscripts(store *SubagentStore, workspaceRoot, baseModel, baseEffort string) *TaskTool {
	t.transcripts = store
	t.workspaceRoot = strings.TrimSpace(workspaceRoot)
	t.baseModel = strings.TrimSpace(baseModel)
	t.baseEffort = strings.TrimSpace(baseEffort)
	return t
}

func (t *TaskTool) WithSubagentEffortLevels(_ []string) *TaskTool {
	// Retained as a boot compatibility hook. Effort capabilities no longer alter
	// model-visible task schemas because configured subagent effort is the sole
	// runtime source.
	return t
}

func (t *TaskTool) WithTranscriptIdentityResolver(resolve func(modelRef, effort string) (string, string)) *TaskTool {
	t.identityProfile = resolve
	return t
}

func (t *TaskTool) WithActorContext(actor userctx.ActorContext) *TaskTool {
	t.actor = cloneTaskActorContext(actor)
	return t
}

// WithStreamDispatcherResolver lets boot bind the actual resolved model to the
// subagent and compaction call roles without placing billing facts in agent.
func (t *TaskTool) WithStreamDispatcherResolver(resolve func(modelRef, effort string, base provider.Provider) (SubagentStreamDispatchers, error)) *TaskTool {
	t.resolveDispatchers = resolve
	return t
}

// WithBackgroundPolicy installs the host capability check for the legacy
// cross-turn background request. Turn-scoped persistent tasks do not use this
// gate because they remain inside the current parent lifecycle.
func (t *TaskTool) WithBackgroundPolicy(policy func(context.Context) error) *TaskTool {
	t.backgroundPolicy = policy
	return t
}

func (t *TaskTool) Name() string { return "task" }

func (t *TaskTool) Description() string {
	return "Start a focused sub-agent task in its own persistent session. Dispatch only when every required input is ready. If another persistent task is running, set independent=true only when this task can complete correctly without that result. For dependent work, wait_task, inspect the result, compose a new prompt, then cite successful task_run_id values in depends_on. The call returns a subagent reference and task run ID immediately; use wait_task to collect the result or stop_task to interrupt the current turn. The sub-agent uses the same provider and a filtered tool list (defaults to every parent tool, then applies the subagent boundary: " + subagentToolBoundarySummary + ")."
}

func (t *TaskTool) Schema() json.RawMessage {
	schema := json.RawMessage(`{
"type":"object",
"properties":{
  "prompt":{"type":"string","description":"What the sub-agent should accomplish. Be specific about the deliverable — the sub-agent does not see this conversation."},
  "description":{"type":"string","description":"Short label for the sub-task (3-7 words). Surfaced in the dispatch line so the user sees what's running."},
  "tools":{"type":"array","items":{"type":"string"},"description":"Optional exact-name tool whitelist. Omit this field to inherit every available sub-agent tool."},
  "max_steps":{"type":"integer","description":"Optional cap on tool-call rounds. Defaults to half the parent's cap (min 5).","minimum":1},
  "model":{"type":"string","description":"Optional model override for the sub-agent (a configured provider/model name)."},
  "continue_from":{"type":"string","description":"Continue a prior compatible subagent transcript in the current conversation context. Pass only the 'sa_...' value from the prior result's 'Subagent reference: ...' line. If the ref belongs to an ancestor conversation, the framework continues a current-conversation copy."},
  "independent":{"type":"boolean","description":"Set true only when this task can complete correctly without results from any currently running persistent task."},
  "depends_on":{"type":"array","items":{"type":"string"},"description":"Successful task_run_id values from this parent session whose inspected results were used to compose this prompt."}
},
"required":["prompt"]
}`)
	return schemaWithTaskToolNames(schema, t.subagentToolNames)
}

// ReadOnly is false: a sub-agent can invoke any whitelisted tool, including
// writers. Conservative classification keeps the parallel-dispatch path from
// running two sub-agents at once and letting their writes race.
func (t *TaskTool) ReadOnly() bool { return false }

func (t *TaskTool) toolTimeoutBudget(base time.Duration, args json.RawMessage) time.Duration {
	return base
}

// ResolveProfile extracts model/effort from task args and applies config defaults.
func (t *TaskTool) ResolveProfile(args json.RawMessage) *event.Profile {
	var p struct {
		Model  string `json:"model"`
		Effort string `json:"effort"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil
	}
	model, effort := t.effectiveProfile(p.Model, p.Effort)
	if model == "" && effort == "" {
		return nil
	}
	return &event.Profile{Model: model, Effort: effort}
}

// ReadOnlyTaskTool runs an isolated sub-agent with a strictly read-only tool
// registry. It intentionally omits background execution and transcript
// continuation/fork controls so the call has no durable host side effects.
type ReadOnlyTaskTool struct {
	task *TaskTool
}

func NewReadOnlyTaskTool(task *TaskTool) *ReadOnlyTaskTool {
	return &ReadOnlyTaskTool{task: task}
}

func (*ReadOnlyTaskTool) Name() string { return "read_only_task" }

func (*ReadOnlyTaskTool) Description() string {
	return "Spawn a read-only research sub-agent for a focused investigation. The sub-agent runs in an isolated, ephemeral session with read-only tools only; bash is wrapped to allow only plan-mode safe foreground commands. It cannot write files, install capabilities, mutate memory, run background jobs, continue/fork transcripts, or delegate to other agents. Only its final answer is returned."
}

func (r *ReadOnlyTaskTool) Schema() json.RawMessage {
	var toolNames []string
	if r != nil && r.task != nil {
		toolNames = r.task.readOnlyToolNames
	}
	schema := json.RawMessage(`{
"type":"object",
"properties":{
  "prompt":{"type":"string","description":"What the read-only sub-agent should investigate. Be specific about the evidence or summary to return — the sub-agent does not see this conversation."},
  "description":{"type":"string","description":"Short label for the read-only sub-task (3-7 words). Surfaced in the dispatch line so the user sees what's running."},
  "tools":{"type":"array","items":{"type":"string"},"description":"Optional exact-name read-only tool whitelist. Omit this field to inherit every available read-only sub-agent tool."},
  "max_steps":{"type":"integer","description":"Optional cap on tool-call rounds. Defaults to half the parent's cap (min 5).","minimum":1},
  "model":{"type":"string","description":"Optional model override for the sub-agent (a configured provider/model name)."}
},
"required":["prompt"]
}`)
	return schemaWithTaskToolNames(schema, toolNames)
}

func (*ReadOnlyTaskTool) ReadOnly() bool { return true }

func (r *ReadOnlyTaskTool) toolTimeoutBudget(base time.Duration, args json.RawMessage) time.Duration {
	if r == nil || r.task == nil {
		return base
	}
	return r.task.toolTimeoutBudget(base, args)
}

// PlanModeSafe reports true: read_only_task spawns a strictly read-only research
// sub-agent (no writers, installers, memory mutation, background jobs, or
// delegation), so it is safe to run while planning.
func (*ReadOnlyTaskTool) PlanModeSafe() bool { return true }

func (r *ReadOnlyTaskTool) ResolveProfile(args json.RawMessage) *event.Profile {
	if r == nil || r.task == nil {
		return nil
	}
	return r.task.ResolveProfile(args)
}

func (r *ReadOnlyTaskTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if r == nil || r.task == nil {
		return "", fmt.Errorf("read_only_task is not configured")
	}
	p, err := decodeReadOnlyTaskParams(args)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(p.Prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}

	maxSteps := p.MaxSteps
	if maxSteps <= 0 && r.task.maxSteps > 0 {
		maxSteps = r.task.maxSteps / 2
		if maxSteps < 5 {
			maxSteps = 5
		}
	}

	parentReg := TaskToolRegistry(ctx, r.task.parentReg)
	subReg := ReadOnlySubagentToolRegistry(parentReg, p.Tools)
	if subReg.Len() == 0 {
		return "", fmt.Errorf("read_only_task has no read-only tools available")
	}
	modelRef, effortRef := r.task.effectiveProfile(p.Model, p.Effort)
	runtime, err := r.task.resolveSubSessionRuntime(modelRef, effortRef)
	if err != nil {
		return "", fmt.Errorf("read-only sub-agent profile: %w", err)
	}
	return r.task.runReadOnlySubSessionWithA2A(ctx, p.Prompt, subReg, maxSteps, runtime, p.A2A)
}

func (t *TaskTool) effectiveProfile(model, _ string) (string, string) {
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(t.subagentModel)
	}
	effort := strings.TrimSpace(t.subagentEffort)
	return model, effort
}

func (t *TaskTool) validateExecution(ctx context.Context, prompt string, legacyBackground bool) error {
	if prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if legacyBackground && t.backgroundPolicy != nil {
		return t.backgroundPolicy(ctx)
	}
	return nil
}

func (t *TaskTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	p, err := decodeTaskParams(args)
	if err != nil {
		return "", err
	}
	legacyBackground := taskLegacyBackground(args)
	if err := t.validateExecution(ctx, p.Prompt, legacyBackground); err != nil {
		return "", err
	}
	if result, admitted := admitPersistentTaskDispatch(ctx, p); !admitted {
		return result, nil
	}

	maxSteps := t.subagentMaxSteps(p.MaxSteps)

	subReg := t.buildSubReg(ctx, p.Tools)
	modelRef, effortRef := t.effectiveProfile(p.Model, p.Effort)
	parentID, parent, _, _ := CallContext(ctx)
	run, err := t.prepareTranscriptRun(subReg, modelRef, effortRef, ParentSession(ctx), parentID, p.ContinueFrom, p.ForkFrom)
	if err != nil {
		return "", err
	}
	runtime, err := t.resolveSubSessionRuntime(modelRef, effortRef)
	if err != nil {
		run.Release()
		return "", fmt.Errorf("sub-agent profile: %w", err)
	}

	if result, handled, err := t.startManagedTask(ctx, args, p, subReg, maxSteps, runtime, run, parentID, parent, legacyBackground); handled {
		return result, err
	}

	// Foreground: run synchronously, nesting events under this call.
	defer run.Release()
	nested := subSink(ctx)
	if t.transcripts != nil && run.Ref != "" {
		if err := t.markSubagentRunning(run, nested); err != nil {
			return "", err
		}
	}
	answer, err := t.runSubSessionWithArtifactHandoff(ctx, p.Prompt, subReg, nested, maxSteps, runtime, run.Session, p.A2A, run.Ref)
	if err != nil {
		return answer, errors.Join(err, t.saveSubagentFailed(run, nested))
	}
	if t.transcripts != nil && run.Ref != "" {
		if err := t.saveSubagentCompleted(run, nested); err != nil {
			return "", errors.Join(err, t.saveSubagentFailed(run, nested))
		}
		return FormatSubagentRunResult(answer, run, false), nil
	}
	return answer, nil
}

func (t *TaskTool) subagentMaxSteps(requested int) int {
	if requested > 0 || t.maxSteps <= 0 {
		return requested
	}
	requested = t.maxSteps / 2
	if requested < 5 {
		return 5
	}
	return requested
}

func formatTaskStart(label string, run *SubagentRun, job *jobs.Job, dependsOn []string, a2a *expertTeamA2ARequest, taskID string) string {
	result := fmt.Sprintf("Subagent reference: %s\nTask run ID: %s\nTurn status: running\nTask: %s", run.Ref, job.ID, label)
	if len(dependsOn) > 0 {
		result += "\nDependencies: " + strings.Join(dependsOn, ", ")
	}
	result += "\nUse wait_task to collect the result or stop_task to interrupt this turn."
	return appendExpertTeamA2AWorking(result, a2a, taskID)
}

func formatLegacyBackgroundTaskStart(jobID, label string, a2a *expertTeamA2ARequest, taskID string) string {
	result := fmt.Sprintf("Started background task %q (%s). It runs across turns; collect its final answer with wait.", jobID, label)
	return appendExpertTeamA2AWorking(result, a2a, taskID)
}

func startLegacyTaskResult(ctx context.Context, args json.RawMessage, jm *jobs.Manager, job *jobs.Job, label string, a2a *expertTeamA2ARequest, taskID string) string {
	cancelCtx := taskParentContext(ctx, args)
	if taskCancelWithParent(args) && cancelCtx.Done() != nil {
		go func() {
			<-cancelCtx.Done()
			jm.KillForSession(jobs.SessionFromContext(ctx), job.ID)
		}()
	}
	return formatLegacyBackgroundTaskStart(job.ID, label, a2a, taskID)
}

func (t *TaskTool) prepareTranscriptRun(subReg *tool.Registry, modelRef, effortRef, parentSession, parentID, continueFrom, legacyForkFrom string) (*SubagentRun, error) {
	continueFrom = strings.TrimSpace(continueFrom)
	legacyForkFrom = strings.TrimSpace(legacyForkFrom)
	parentSession = strings.TrimSpace(parentSession)
	if continueFrom != "" && legacyForkFrom != "" {
		return nil, fmt.Errorf("continue_from and fork_from are mutually exclusive; pass only continue_from")
	}
	if t.transcripts == nil {
		return nil, fmt.Errorf("subagent transcript store is required")
	}
	// Headless runs (e.g. `kwu run`) never mint a session path, so there is
	// no parent session to own a transcript. Run the sub-agent ephemerally —
	// exactly as before persisted transcripts existed — instead of failing the
	// call. Continuation/fork need a persisted owner, so they error here.
	if parentSession == "" {
		if continueFrom != "" || legacyForkFrom != "" {
			return nil, fmt.Errorf("subagent continuation requires a persisted session; none is active in this run")
		}
		return EphemeralSubagentRun(t.sysPrompt), nil
	}
	identityModel, identityEffort := t.effectiveIdentity(modelRef, effortRef)
	spec := SubagentSpec{
		Kind:             "task",
		Name:             "task",
		WorkspaceRoot:    t.workspaceRoot,
		ParentSession:    parentSession,
		ParentToolCallID: parentID,
		SystemPrompt:     t.sysPrompt,
		Registry:         subReg,
		Model:            identityModel,
		Effort:           identityEffort,
		Actor:            t.actor,
	}
	if continueFrom != "" {
		return t.transcripts.PrepareContinue(continueFrom, spec)
	}
	if legacyForkFrom != "" {
		return t.transcripts.PrepareLegacyForkFrom(legacyForkFrom, spec)
	}
	return t.transcripts.PrepareFresh(spec)
}

func cloneTaskActorContext(actor userctx.ActorContext) userctx.ActorContext {
	if actor.RemoteIdentity != nil {
		remote := *actor.RemoteIdentity
		actor.RemoteIdentity = &remote
	}
	return actor
}

func (t *TaskTool) effectiveIdentity(modelRef, effort string) (string, string) {
	if t.identityProfile != nil {
		model, eff := t.identityProfile(modelRef, effort)
		return strings.TrimSpace(model), strings.TrimSpace(eff)
	}
	return t.effectiveModelIdentity(modelRef), t.effectiveEffortIdentity(effort)
}

func (t *TaskTool) effectiveModelIdentity(modelRef string) string {
	if strings.TrimSpace(modelRef) != "" {
		return strings.TrimSpace(modelRef)
	}
	return strings.TrimSpace(t.baseModel)
}

func (t *TaskTool) effectiveEffortIdentity(effort string) string {
	if strings.TrimSpace(effort) != "" {
		return strings.TrimSpace(effort)
	}
	return strings.TrimSpace(t.baseEffort)
}

// buildSubReg returns the sub-agent's tool set: the named whitelist (minus
// unavailable sub-agent tools), or every parent tool except those tools.
func (t *TaskTool) buildSubReg(ctx context.Context, names []string) *tool.Registry {
	return SubagentToolRegistry(TaskToolRegistry(ctx, t.parentReg), names)
}

// FilterRegistry builds a sub-registry from parent: the named whitelist (empty =
// every parent tool), minus any excluded names. Used to scope what a spawned
// sub-agent — a `task` sub-agent or a subagent skill — may call, e.g. excluding
// `task` to bar recursive nesting, or restricting to a skill's allowed-tools.
func FilterRegistry(parent *tool.Registry, names []string, exclude ...string) *tool.Registry {
	sub := tool.NewRegistry()
	if parent == nil {
		return sub
	}
	ex := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		ex[e] = true
	}
	src := names
	if len(src) == 0 {
		src = parent.Names()
	}
	for _, name := range src {
		if ex[name] {
			continue
		}
		if tl, ok := parent.Get(name); ok {
			sub.Add(tl)
		}
	}
	return sub
}

var plannerNonResearchTools = []string{
	"ask",
	"bash_output",
	"complete_step",
	"slash_command",
	"todo_write",
	"wait",
}

// PlannerToolRegistry returns the tool set exposed to the two-model planner:
// read-only research tools only. It deliberately excludes workflow/meta tools
// that are technically read-only but can prompt the user, update visible task
// state, wait on jobs, or expand commands instead of inspecting context.
func PlannerToolRegistry(parent *tool.Registry) *tool.Registry {
	exclude := append(SubagentMetaTools(), plannerNonResearchTools...)
	return FilterReadOnlyRegistry(parent, exclude...)
}

// ReadOnlySubagentToolRegistry returns the tool set exposed to read-only
// sub-agents: read-only research tools plus a bash wrapper that enforces the
// plan-mode safe command policy at execution time. Workflow/meta tools are
// excluded even when their Tool.ReadOnly contract is true.
func ReadOnlySubagentToolRegistry(parent *tool.Registry, names []string) *tool.Registry {
	exclude := append(SubagentMetaTools(), subagentJobTools...)
	exclude = append(exclude, plannerNonResearchTools...)
	exclude = append(exclude, readOnlySubagentWorkflowTools...)
	ex := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		ex[e] = true
	}
	sub := tool.NewRegistry()
	if parent == nil {
		return sub
	}
	src := normalizeSubagentToolNames(parent, names)
	if len(src) == 0 {
		src = parent.Names()
	}
	for _, name := range src {
		if ex[name] {
			continue
		}
		tl, ok := parent.Get(name)
		if !ok {
			continue
		}
		if name == "bash" {
			sub.Add(readOnlyBash{inner: tl})
			continue
		}
		if !tl.ReadOnly() {
			continue
		}
		if u, ok := tl.(tool.PlanModeUntrustedReadOnly); ok && u.PlanModeUntrustedReadOnly() {
			// An external tool's self-reported readOnlyHint isn't trusted for a
			// read-only research sub-agent; exclude it like a writer.
			continue
		}
		sub.Add(tl)
	}
	return sub
}

// FilterReadOnlyRegistry builds a sub-registry containing only tools whose
// ReadOnly contract is true, minus explicit exclusions.
func FilterReadOnlyRegistry(parent *tool.Registry, exclude ...string) *tool.Registry {
	ex := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		ex[e] = true
	}
	sub := tool.NewRegistry()
	if parent == nil {
		return sub
	}
	for _, name := range parent.Names() {
		if ex[name] {
			continue
		}
		tl, ok := parent.Get(name)
		if !ok || !tl.ReadOnly() {
			continue
		}
		if u, ok := tl.(tool.PlanModeUntrustedReadOnly); ok && u.PlanModeUntrustedReadOnly() {
			continue
		}
		sub.Add(tl)
	}
	return sub
}

func (t *TaskTool) resolveSubSessionRuntime(modelRef, effort string) (subSessionRuntime, error) {
	prov, pricing, ctxWin := t.prov, t.pricing, t.contextWindow
	if t.resolveProvider != nil && (modelRef != "" || effort != "") {
		p, pr, cw, err := t.resolveProvider(modelRef, effort)
		if err != nil {
			return subSessionRuntime{}, err
		}
		prov, pricing, ctxWin = p, pr, cw
	}
	var dispatchers SubagentStreamDispatchers
	if t.resolveDispatchers != nil {
		resolved, err := t.resolveDispatchers(modelRef, effort, prov)
		if err != nil {
			return subSessionRuntime{}, err
		}
		dispatchers = resolved
	}
	return subSessionRuntime{provider: prov, pricing: pricing, contextWindow: ctxWin, dispatchers: dispatchers}, nil
}

func (t *TaskTool) subagentOptions(ctx context.Context, maxSteps int, runtime subSessionRuntime) Options {
	return Options{
		MaxSteps:                maxSteps,
		Temperature:             t.temperature,
		Pricing:                 runtime.pricing,
		UsageSource:             event.UsageSourceSubagent,
		Gate:                    t.gate,
		ToolTimeout:             t.toolTimeout,
		ContextWindow:           runtime.contextWindow,
		RecentKeep:              t.recentKeep,
		SoftCompactRatio:        t.softCompactRatio,
		CompactRatio:            t.compactRatio,
		CompactForceRatio:       t.compactForceRatio,
		ArchiveDir:              t.archiveDir,
		KeepPolicy:              t.keepPolicy,
		ResponseLanguage:        ResponseLanguageFromContext(ctx),
		ReasoningLanguage:       ReasoningLanguageFromContext(ctx),
		StreamDispatcher:        runtime.dispatchers.Main,
		DerivedStreamDispatcher: runtime.dispatchers.Compaction,
	}
}

func (t *TaskTool) runSubSession(ctx context.Context, prompt string, subReg *tool.Registry, sink event.Sink, maxSteps int, runtime subSessionRuntime, sess *Session) (string, error) {
	return RunSubAgentWithSession(ctx, runtime.provider, subReg, sess, prompt, t.subagentOptions(ctx, maxSteps, runtime), sink)
}

func FormatSubagentReference(run *SubagentRun) string {
	if run == nil || run.Ref == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Subagent reference: %s\n", run.Ref)
	if strings.TrimSpace(run.ForkedFrom) != "" {
		fmt.Fprintf(&b, "Forked from: %s\n", strings.TrimSpace(run.ForkedFrom))
		b.WriteString("The requested ref resolves to an ancestor conversation transcript, so the framework continues a copy owned by the current conversation. To continue this copied subagent transcript in a later call, pass ")
		b.WriteString(run.Ref)
		b.WriteString(" as `continue_from`. Start a fresh subagent when the next task is independent.")
		return b.String()
	}
	b.WriteString("To continue this same subagent transcript in a later call, pass this ref as `continue_from`. Start a fresh subagent when the next task is independent.")
	return b.String()
}

func FormatSubagentRunResult(answer string, run *SubagentRun, failed bool) string {
	if run == nil || run.Ref == "" {
		return answer
	}
	if failed {
		if answer == "" {
			return "Subagent reference (failed): " + run.Ref
		}
		return "Subagent reference (failed): " + run.Ref + "\n\nFinal answer:\n" + answer
	}
	return FormatSubagentReference(run) + "\n\nFinal answer:\n" + answer
}

// RunSubAgentWithSession continues an existing sub-agent session with prompt and
// returns the latest final assistant answer. Fresh sub-agents pass a newly-created
// session; continued sub-agents pass a loaded transcript session.
func RunSubAgentWithSession(ctx context.Context, prov provider.Provider, reg *tool.Registry, sess *Session, prompt string, opts Options, sink event.Sink) (string, error) {
	if sess == nil {
		return "", fmt.Errorf("sub-agent session is nil")
	}
	sub := New(prov, reg, sess, opts, sink)
	if err := sub.Run(ctx, prompt); err != nil {
		return "", fmt.Errorf("sub-agent: %w", err)
	}
	// Walk the session backwards for the last assistant message with content —
	// that's the sub-agent's final answer. Intermediate assistant messages with
	// tool_calls but no text don't count.
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		m := sess.Messages[i]
		if m.Role == provider.RoleAssistant && strings.TrimSpace(m.Content) != "" {
			return m.Content, nil
		}
	}
	return "", fmt.Errorf("sub-agent finished without producing a final answer")
}

// NestedSink returns a sink that forwards a sub-agent's tool activity to the
// parent stream, nested under the tool call carried by ctx, so a frontend shows
// it beneath that call (the same nesting `task` uses). Falls back to the given
// sink when ctx carries no call context. Used by subagent skills.
func NestedSink(ctx context.Context, fallback event.Sink) event.Sink {
	parentID, parent, _, ok := CallContext(ctx)
	if !ok || parent == nil {
		return fallback
	}
	taskCtx, _ := event.TaskTreeContextFromContext(ctx)
	return subSinkForContext(parentID, parent, taskCtx, "")
}

// subSink forwards a sub-agent's tool dispatch/result events and billable usage
// to the parent's event stream. Only tool activity is nested visually; the
// sub-agent's text/reasoning stays isolated and only its final answer is returned.
//
// The sub-agent's own turn/text/reasoning events are dropped — forwarding them
// would make the parent transcript noisy and could imply they belong to the
// parent model context, which they do not.
//
// Usage events are observability only, so forwarding them preserves billing
// totals without polluting the parent provider-visible prefix.
//
// Tool events are tagged with the parent task call's ID so a frontend nests them
// under it. The forwarded call IDs are namespaced with the parent ID so a
// sub-agent call can never collide with a parent call in the frontend's
// dispatch→result matching. Falls back to Discard when there's no parent stream
// (the headless run loop, or a direct Execute in tests).
func subSink(ctx context.Context) event.Sink {
	parentID, parent, _, ok := CallContext(ctx)
	if !ok || parent == nil {
		return event.Discard
	}
	taskCtx, _ := event.TaskTreeContextFromContext(ctx)
	return subSinkForContext(parentID, parent, taskCtx, "")
}

// subSinkFor builds the nesting sink from an already-captured parent ID + stream,
// for the background path where the job runs under a context that no longer
// carries the call context. Falls back to Discard when there's no parent stream.
func subSinkFor(parentID string, parent event.Sink) event.Sink {
	return subSinkForContext(parentID, parent, event.TaskTreeContext{}, "")
}

func subSinkForContext(parentID string, parent event.Sink, taskCtx event.TaskTreeContext, subagentRef string) event.Sink {
	if parent == nil {
		return event.Discard
	}
	return event.FuncSink(func(e event.Event) {
		projectSubagentEvent(parentID, parent, taskCtx, subagentRef, e)
	})
}

func projectSubagentEvent(parentID string, parent event.Sink, taskCtx event.TaskTreeContext, subagentRef string, e event.Event) {
	switch e.Kind {
	case event.Text:
		projectSubagentText(parentID, parent, taskCtx, subagentRef, e)
	case event.Reasoning, event.Message:
		emitSubagentTranscript(parentID, parent, subagentRef, e)
	case event.ToolDispatch, event.ToolResult, event.ToolProgress:
		projectSubagentTool(parentID, parent, taskCtx, subagentRef, e)
	case event.Usage:
		projectSubagentUsage(parent, e)
	case event.SubagentStatusChanged:
		projectSubagentStatus(parent, subagentRef, e)
	}
}

func projectSubagentText(parentID string, parent event.Sink, taskCtx event.TaskTreeContext, subagentRef string, e event.Event) {
	if taskCtx.TreeID != "" && taskCtx.NodeID != "" && e.Text != "" {
		parent.Emit(event.Event{
			Kind: event.TaskNodeOutput,
			TaskTree: &event.TaskTreePayload{
				TreeID:      taskCtx.TreeID,
				NodeID:      taskCtx.NodeID,
				OutputDelta: e.Text,
			},
			Source: "task_tree",
		})
	}
	emitSubagentTranscript(parentID, parent, subagentRef, e)
}

func emitSubagentTranscript(parentID string, parent event.Sink, subagentRef string, e event.Event) {
	e.ParentID = parentID
	e.SubagentRef = subagentRef
	parent.Emit(e)
}

func projectSubagentTool(parentID string, parent event.Sink, taskCtx event.TaskTreeContext, subagentRef string, e event.Event) {
	e.Tool.ParentID = parentID
	e.Tool.SubagentRef = subagentRef
	if e.Tool.ID != "" {
		e.Tool.SourceToolID = e.Tool.ID
		e.Tool.ID = parentID + "/" + e.Tool.ID
	}
	if taskCtx.TreeID != "" && taskCtx.NodeID != "" {
		e.Tool.TaskTreeID = taskCtx.TreeID
		e.Tool.NodeID = taskCtx.NodeID
	}
	parent.Emit(e)
}

func projectSubagentUsage(parent event.Sink, e event.Event) {
	if e.UsageSource == "" {
		e.UsageSource = event.UsageSourceSubagent
	}
	parent.Emit(e)
}

func projectSubagentStatus(parent event.Sink, subagentRef string, e event.Event) {
	if e.SubagentRef == "" {
		e.SubagentRef = subagentRef
	}
	parent.Emit(e)
}

func taskCancelWithParent(args json.RawMessage) bool {
	var p struct {
		CancelWithParent bool `json:"cancel_with_parent"`
	}
	return json.Unmarshal(args, &p) == nil && p.CancelWithParent
}

func taskLegacyBackground(args json.RawMessage) bool {
	var p struct {
		RunInBackground bool `json:"run_in_background"`
	}
	return json.Unmarshal(args, &p) == nil && p.RunInBackground
}

func taskParentContext(ctx context.Context, args json.RawMessage) context.Context {
	if taskCancelWithParent(args) {
		return toolCallParentContext(ctx)
	}
	return ctx
}

func startTaskJob(jm *jobs.Manager, turnCtx context.Context, parentSession, ref, label string, dependsOn []string, run func(context.Context, io.Writer) (string, error)) *jobs.Job {
	if turnCtx != nil {
		return jm.StartControlledForTurnWithDependencies(turnCtx, parentSession, "task", ref, label, dependsOn, run)
	}
	return jm.StartControlledForSessionWithDependencies(parentSession, "task", ref, label, dependsOn, run)
}
