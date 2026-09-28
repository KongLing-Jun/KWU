package control

import (
	"context"
	"errors"
	"fmt"
	"html"
	"slices"
	"strings"
	"time"
	"unicode"

	"kwu/internal/agent"
	"kwu/internal/brand"
	"kwu/internal/i18n"
	"kwu/internal/planmode"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/skill"
	"kwu/internal/tool"
)

// SetTimezone updates the client/session IANA timezone without rebuilding the
// model session or rewriting prior messages. Invalid names leave it unchanged.
func (c *Controller) SetTimezone(name string) error {
	if c.temporal == nil {
		return errors.New("temporal service is unavailable")
	}
	return c.temporal.SetOverride(name)
}

// Timezone returns the currently resolved IANA timezone label.
func (c *Controller) Timezone() string {
	if c.temporal == nil {
		return ""
	}
	name, _ := c.temporal.Timezone()
	return name
}

func (c *Controller) submitTimeCommand(input string) bool {
	if input != "/time" {
		return false
	}
	if c.temporal == nil {
		c.notice("system time is unavailable")
		return true
	}
	snapshot := c.temporal.Snapshot()
	c.notice(fmt.Sprintf(
		i18n.M.CurrentTimeFmt,
		snapshot.Local.Format(time.RFC3339),
		snapshot.Weekday.String(),
		snapshot.Timezone,
		snapshot.UTCOffset,
		snapshot.UTC.Format(time.RFC3339),
		snapshot.UnixSeconds,
	))
	return true
}

// Submit is the one-call entry for a simple frontend: it takes raw user input
// and does everything — slash-command dispatch, @-reference expansion, plan-mode
// composition — emitting all output as events. The HTTP/SSE server uses this so
// a browser client only POSTs the typed line.
//
// Slash commands route to the matching primitive: /compact, /new, and /clear
// run their session op and emit a Notice; /mcp__server__prompt and custom /commands
// resolve to a turn; an unknown slash emits a Notice. Anything else is a normal
// turn with its @-references resolved first.
func (c *Controller) Submit(input string) {
	c.submit(input, "")
}

// SubmitHTTP accepts input from the unauthenticated localhost HTTP frontend. It
// deliberately omits the trusted TUI-only "!cmd" shell shortcut and resolves file
// references only through the controller's workspace root.
func (c *Controller) SubmitHTTP(input string) {
	c.submitHTTP(input, "")
}

// SubmitDisplay runs input as a turn while remembering the user-facing display
// text for transcript replay when controller-side composition expands input.
func (c *Controller) SubmitDisplay(display, input string) {
	c.submit(input, display)
}

// SubmitUserTurn starts a normal model turn without interpreting shell or slash
// commands. It still resolves references, so callers can submit trusted
// user-authored prompt text without expanding the command surface.
func (c *Controller) SubmitUserTurn(input, display string) {
	c.runRefTurn(input, display)
}

// CapabilityTurn carries inline /<name> skill expansions and the MCP namespaces
// exposed to this model turn.
type CapabilityTurn struct {
	Input   string
	Display string
	// TitleText is the user-authored text used by hosts for automatic task
	// titles. PreferTitleText distinguishes an intentionally empty title (for
	// example, a turn containing only skill tokens) from an unspecified one.
	TitleText       string
	PreferTitleText bool
	SkillNames      []string
	MCPServerNames  []string
	// CommandEnv is supplied only by a trusted host. It is not part of the
	// frontend composer DTO or any persisted task model.
	CommandEnv map[string]string
	// ToolAllowlist is the immutable connector-tool snapshot authorized for
	// this turn. An empty list exposes no MCP tools, even when a server is live.
	ToolAllowlist []string
	// RestrictTools distinguishes automation's fail-closed snapshot from the
	// interactive SubmitCapabilities API, which historically grants all tools
	// exposed by explicitly selected MCP servers.
	RestrictTools bool
}

// SubmitCapabilities starts a normal user turn with host-validated capabilities.
// Validation of discoverable skills and live MCP servers is performed by the
// desktop boundary before this method is called.
func (c *Controller) SubmitCapabilities(turn CapabilityTurn) {
	c.runGuarded(func(ctx context.Context) error {
		ctx, err := c.bindCapabilityTaskContext(ctx, turn.CommandEnv)
		if err != nil {
			return err
		}
		return c.runTaskLifecycle(ctx, func(ctx context.Context) error { return c.runCapabilityTurn(ctx, turn) })
	})
}

// RunCapabilityTurn is the blocking transport boundary used by automation.
// It applies the same lifecycle as RunTurnDisplay while enforcing the frozen
// skill, connector, and exact tool selection for this run only.
func (c *Controller) RunCapabilityTurn(ctx context.Context, turn CapabilityTurn) error {
	return c.runForegroundTurnBound(ctx, func(ctx context.Context) (context.Context, error) {
		return c.bindCapabilityTaskContext(ctx, turn.CommandEnv)
	}, func(ctx context.Context) error { return c.runCapabilityTurn(ctx, turn) })
}

// expandCapabilityCommand preserves native slash semantics when a turn also
// carries a task-scoped MCP registry. Only slash forms that produce a model
// turn are expanded here; built-in and management commands stay on the normal
// Submit path.
func (c *Controller) expandCapabilityCommand(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return input, false
	}
	// Keep the controller's native precedence: a custom command shadows a skill
	// with the same name.
	if expanded, ok := c.CustomCommand(trimmed); ok {
		return expanded, true
	}
	return c.RunSkill(trimmed)
}

// PrepareCapabilities validates skills without loading or executing them.
func (c *Controller) PrepareCapabilities(_ context.Context, turn CapabilityTurn) (CapabilityTurn, error) {
	commandEnv, err := tool.ValidateCommandEnv(turn.CommandEnv)
	if err != nil {
		return CapabilityTurn{}, err
	}
	turn.CommandEnv = commandEnv
	seen := make(map[string]bool, len(turn.SkillNames))
	for _, rawName := range turn.SkillNames {
		name := strings.TrimSpace(rawName)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if _, ok := c.skills.byName(name); !ok {
			return CapabilityTurn{}, fmt.Errorf("selected skill %q is unavailable", name)
		}
	}
	return turn, nil
}

func registryForMCPServers(base *tool.Registry, serverNames []string) *tool.Registry {
	if base == nil {
		return tool.NewRegistry()
	}
	return registryForCapabilities(base, serverNames, base.Names())
}

func registryForCapabilities(base *tool.Registry, serverNames, toolAllowlist []string) *tool.Registry {
	filtered := tool.NewRegistry()
	if base == nil {
		return filtered
	}
	selectedPrefixes := make(map[string]bool, len(serverNames))
	for _, name := range serverNames {
		if name = strings.TrimSpace(name); name != "" {
			selectedPrefixes[plugin.ToolPrefix(name)] = true
		}
	}
	for _, name := range base.Names() {
		if strings.HasPrefix(name, tool.MCPNamePrefix) {
			if !matchesMCPPrefix(name, selectedPrefixes) || !slices.Contains(toolAllowlist, name) {
				continue
			}
		}
		if registered, ok := base.Get(name); ok {
			filtered.Add(registered)
		}
	}
	return filtered
}

func matchesMCPPrefix(toolName string, selectedPrefixes map[string]bool) bool {
	for prefix := range selectedPrefixes {
		if strings.HasPrefix(toolName, prefix) {
			return true
		}
	}
	return false
}

func selectedMCPContext(serverNames []string, registry *tool.Registry) string {
	if registry == nil {
		return ""
	}
	if len(serverNames) == 0 {
		return "<enabled-mcp-connectors>\nEnabled for this turn: none.\n</enabled-mcp-connectors>\n\n"
	}
	availableTools := registry.Names()
	seen := make(map[string]bool, len(serverNames))
	availableServers := make([]string, 0, len(serverNames))
	for _, rawName := range serverNames {
		name := strings.TrimSpace(rawName)
		if name == "" || seen[name] {
			continue
		}
		prefix := plugin.ToolPrefix(name)
		available := false
		for _, toolName := range availableTools {
			if strings.HasPrefix(toolName, prefix) {
				available = true
				break
			}
		}
		if !available {
			continue
		}
		seen[name] = true
		availableServers = append(availableServers, html.EscapeString(name))
	}
	if len(availableServers) == 0 {
		return ""
	}
	return "<enabled-mcp-connectors>\n" +
		"Enabled for this turn: " + strings.Join(availableServers, ", ") + ".\n" +
		"Their tools are available in the tool schema. When the user's request clearly matches an enabled connector, use an appropriate tool instead of only describing what could be done. Selection means availability, not that a tool has already run. Never claim a connector was used unless its tool call succeeds.\n" +
		"</enabled-mcp-connectors>\n\n"
}

// SetDisplayRecorder installs an optional hook used by frontends that persist a
// shorter user-facing transcript than the fully composed model prompt.
func (c *Controller) SetDisplayRecorder(fn func(content, display string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.displayRecorder = fn
}

// SetAcceptedUserTurnHandler installs an optional observer for user-authored
// turns that passed command dispatch and the running-turn guard. The observer is
// invoked at the foreground turn boundary before auto-plan or hooks run.
//
// This is intentionally not part of SessionAPI: hosts that do not need this
// lifecycle detail should not have to implement it.
func (c *Controller) SetAcceptedUserTurnHandler(fn func(raw, display, titleText string, preferTitleText bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acceptedUserTurn = fn
}

func (c *Controller) notifyAcceptedUserTurn(ctx context.Context, raw, display, titleText string, preferTitleText bool) {
	if synthetic, _ := ctx.Value(syntheticTurnContextKey{}).(bool); synthetic {
		return
	}
	c.mu.Lock()
	notify := c.acceptedUserTurn
	c.mu.Unlock()
	if notify != nil {
		notify(raw, display, titleText, preferTitleText)
	}
}

func (c *Controller) recordDisplay(content, display string) {
	if strings.TrimSpace(display) == "" || content == display {
		return
	}
	c.mu.Lock()
	record := c.displayRecorder
	c.mu.Unlock()
	if record != nil {
		record(content, display)
	}
}

func (c *Controller) recordDisplayForNewUser(startMessages int, display string) {
	if strings.TrimSpace(display) == "" {
		return
	}
	msgs := c.History()
	if startMessages > len(msgs) {
		startMessages = len(msgs)
	}
	for _, m := range msgs[startMessages:] {
		if m.Role == provider.RoleUser {
			c.recordDisplay(m.Content, display)
			return
		}
	}
}

func (c *Controller) submit(input, display string) {
	trimmed := strings.TrimSpace(input)
	if c.submitTimeCommand(trimmed) {
		return
	}
	if note, ok := MemoryQuickAddNote(trimmed); ok {
		c.rememberProjectNote(note)
		return
	}
	if note, ok := RememberCommandNote(trimmed); ok {
		c.rememberProjectNote(note)
		return
	}
	if c.applyGoalCommand(trimmed, display) {
		return
	}
	if strings.HasPrefix(trimmed, "!") {
		c.RunShell(trimmed[1:])
		return
	}
	c.submitCommandOrTurn(trimmed, input, display, false)
}

func (c *Controller) submitHTTP(input, display string) {
	trimmed := strings.TrimSpace(input)
	if c.submitTimeCommand(trimmed) {
		return
	}
	if note, ok := MemoryQuickAddNote(trimmed); ok {
		c.rememberProjectNote(note)
		return
	}
	if note, ok := RememberCommandNote(trimmed); ok {
		c.rememberProjectNote(note)
		return
	}
	if c.applyGoalCommand(trimmed, display) {
		return
	}
	if strings.HasPrefix(trimmed, "!") {
		c.notice("shell commands are unavailable from this frontend")
		return
	}
	c.submitCommandOrTurn(trimmed, input, display, true)
}

// PlanModeMarker is prepended to every user turn while plan mode is on. It rides
// in the user message (not the system prompt or tools), so the cache-stable
// prompt prefix is left untouched and the toggle costs nothing in cache hits.
const PlanModeMarker = planmode.Marker

const legacyPlanModeMarker = "[Plan mode — read-only. Explore the codebase first (read_file, ls, grep, glob, web_fetch, task, ask are available; writers are refused by the harness). Before planning, if a decision that is genuinely the user's — tech stack, an ambiguous requirement, scope, an irreversible choice — would materially shape the plan and you can't settle it from the codebase or a sensible default, use the ask tool to clarify it first; otherwise pick the obvious default and state the assumption in the plan instead of asking. Then present a LAYERED plan as your reply and stop — do not write files, edit, or run side-effecting bash. Structure the plan as a two-level markdown list so it becomes a layered task list: each PHASE is a top-level numbered list item (a coherent milestone, e.g. \"1. Add the config loader\"), and each phase's concrete, verifiable sub-steps are bullets indented beneath it (e.g. \"   - parse the TOML into Config\"). Use plain numbered list items for phases — do NOT write phases as markdown headings (##, ###) — so both levels parse. Keep phases few (about 2-6). The user will be asked to approve before any changes are made.]"

const (
	activeGoalOpen  = "<active-goal>"
	activeGoalClose = "</active-goal>"
)

const (
	GoalStatusRunning  = "running"
	GoalStatusComplete = "complete"
	GoalStatusBlocked  = "blocked"
	GoalStatusStopped  = "stopped"
)

type GoalResearchMode int

const (
	GoalResearchAuto GoalResearchMode = iota
	GoalResearchOn
	GoalResearchOff
)

// StripComposePrefixes removes controller-injected prefixes from a composed
// user message so that the display text matches what the user actually typed.
// It strips the PlanModeMarker plus transient XML blocks such as
// <reasoning-language>, <memory-update>, and <background-jobs> that Compose
// prepends to user turns. This is used as a fallback when no .display.json
// sidecar recording exists (e.g. sessions created before the display-recording
// feature, or synthetic user messages injected by the controller).
func StripComposePrefixes(content string) string {
	s := agent.StripTransientUserBlocks(content)
	s = stripComposeMarker(s, PlanModeMarker)
	s = stripComposeMarker(s, legacyPlanModeMarker)
	s = stripLeadingActiveGoalBlockForDisplay(s)
	s = agent.StripHiddenSystemPromptBlocks(s)
	s = strings.TrimRight(s, " \t\r\n")
	if display := automationPromptDisplay(s); display != "" {
		return display
	}
	return s
}

func stripLeadingActiveGoalBlockForDisplay(content string) string {
	if !strings.HasPrefix(strings.TrimLeft(content, " \t\r\n"), activeGoalOpen) {
		return content
	}
	return stripActiveGoalBlock(content)
}

func automationPromptDisplay(s string) string {
	if !strings.HasPrefix(strings.TrimSpace(s), "# Automation Task") {
		return ""
	}
	const marker = "\n## Task Prompt\n"
	start := strings.Index(s, marker)
	if start < 0 {
		return ""
	}
	body := s[start+len(marker):]
	if end := strings.Index(body, "\n## "); end >= 0 {
		body = body[:end]
	}
	return strings.TrimSpace(body)
}

func stripComposeMarker(s, marker string) string {
	s = strings.TrimPrefix(s, marker+"\n\n")
	return strings.TrimPrefix(s, marker)
}

// StripReferencedContextPrefix removes the "Referenced context:" preamble and
// the trailing XML reference blocks (<file>, <dir>, <resource>, <image>) that
// controller.ResolveRefs injects when the user @-references files or resources.
// The user's actual input follows the reference blocks after a blank line.
// Used for title generation and previews so the displayed text matches what
// the user typed, not the injected context preamble (#4954).
func StripReferencedContextPrefix(content string) string {
	const preamble = "Referenced context:"
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, preamble) {
		return content
	}
	// Skip past the preamble.
	s = strings.TrimSpace(s[len(preamble):])
	// Skip past all XML reference blocks: <file ...>...</file>, <dir ...>...</dir>,
	// <resource ...>...</resource>, <image ...>...</image>.
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		// Check for a reference block start.
		if !strings.HasPrefix(s, "<file ") && !strings.HasPrefix(s, "<dir ") &&
			!strings.HasPrefix(s, "<resource ") && !strings.HasPrefix(s, "<image ") {
			break
		}
		// Find the matching close tag.
		tagEnd := strings.IndexByte(s, ' ')
		if tagEnd < 0 {
			break
		}
		tagName := s[1:tagEnd]
		closeTag := "</" + tagName + ">"
		closeIdx := strings.Index(s, closeTag)
		if closeIdx < 0 {
			break
		}
		s = strings.TrimSpace(s[closeIdx+len(closeTag):])
	}
	return s
}

// IsSyntheticUserMessage returns true if the content matches one of the known
// synthetic user messages injected by the controller or agent loop (plan
// approval, stream recovery, readiness retry, etc.). These should not be shown
// in the chat UI.
func IsSyntheticUserMessage(content string) bool {
	trimmed := strings.TrimSpace(agent.StripTransientUserBlocks(content))
	if trimmed == planApprovedMessage {
		return true
	}
	for _, prefix := range syntheticPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// syntheticPrefixes must be kept in sync with the synthetic user messages
// injected by the controller (planApprovedMessage, goal loop turns), agent loop
// (streamRecoveryMessage, finalReadinessRetryMessage, emptyFinalRetryMessage,
// executorHandoffRetryMessage in internal/agent/agent.go), and compaction
// folds (internal/agent/compact.go), which store summaries as user-role
// messages the chat UI must never render as user bubbles (#3653).
var syntheticPrefixes = []string{
	"Plan approved — plan mode is off",
	"Host final-answer readiness check",
	"You are already in the executor phase",
	"The previous assistant response was interrupted while a tool call",
	"The previous assistant response was interrupted during streaming",
	"The previous assistant response was interrupted before visible",
	"The previous assistant response finished without any visible answer",
	"<compaction-summary>",
	"Summary of the later conversation (compacted from here on):",
	"Summary of earlier conversation (compacted up to here):",
	"Continue pursuing the active goal.",
	"The agent signaled goal completion and all tasks are marked done.",
	"Goal signaled complete but issues remain:",
	"No tool calls in recent turns.",
}

// Compose applies the plan-mode marker to a turn's text when plan mode is on,
// returning the message to actually send to the model. The frontend keeps
// showing the raw text as the user bubble.
func (c *Controller) Compose(text string) string {
	c.mu.Lock()
	plan := c.planMode
	responseLanguage := c.responseLanguage
	reasoningLanguage := c.reasoningLanguage
	c.mu.Unlock()
	notes := c.memory.drainPending()
	goal, goalStatus, goalResearchMode := c.goals.snapshot()

	if strings.TrimSpace(goal) != "" && goalStatus == GoalStatusRunning {
		text = activeGoalBlock(goal, goalResearchMode) + "\n\n" + text
	}
	if plan {
		text = PlanModeMarker + "\n\n" + text
	}
	text = agent.WithResponseLanguage(text, responseLanguage)
	text = agent.WithReasoningLanguage(text, reasoningLanguage)
	// Memory added mid-session rides the turn (never the cached system prefix),
	// so it takes effect now without invalidating the prompt cache. It folds into
	// the system prefix on the next session, where it costs nothing per turn.
	if len(notes) > 0 {
		var b strings.Builder
		b.WriteString("<memory-update>\n")
		b.WriteString("The following project-memory changes were just made and apply from now on:\n")
		for _, n := range notes {
			b.WriteString("- " + n + "\n")
		}
		b.WriteString("</memory-update>\n\n")
		text = b.String() + text
	}

	// Background jobs that finished since the last turn ride the turn too, so the
	// model learns of completions even though the user-facing notices don't reach
	// its context. Like memory, this never touches the cache-stable prefix.
	if c.jobs != nil {
		if note := c.jobs.DrainCompletedNoteForSession(c.parentSessionID()); note != "" {
			text = "<background-jobs>\n" + note + "\n</background-jobs>\n\n" + text
		}
	}
	if c.temporal != nil {
		text = c.temporal.Snapshot().ContextBlock() + "\n\n" + text
	}
	return text
}

func reasoningLanguageBlock(lang string) string {
	return agent.ReasoningLanguageBlock(lang)
}

func (c *Controller) ComposeSynthetic(text string) string {
	c.mu.Lock()
	responseLang := c.responseLanguage
	lang := c.reasoningLanguage
	c.mu.Unlock()
	text = agent.WithResponseLanguage(text, responseLang)
	text = agent.WithReasoningLanguage(text, lang)
	if c.temporal != nil {
		text = c.temporal.Snapshot().ContextBlock() + "\n\n" + text
	}
	return text
}

func activeGoalBlock(goal string, researchMode GoalResearchMode) string {
	goal = strings.TrimSpace(goal)
	goal = strings.ReplaceAll(goal, activeGoalClose, "<\\/active-goal>")
	var b strings.Builder
	b.WriteString(activeGoalOpen)
	b.WriteString("\n")
	b.WriteString(goal)
	b.WriteString("\n\n")
	b.WriteString("Goal mode: pursue this goal autonomously. Keep working across turns until the goal is complete. Prefer sensible defaults over asking the user; use ask only when you are truly blocked on a user-owned decision. Do not stop after describing a plan; execute the next useful step. End every goal-mode assistant reply with exactly one status marker on its own line: [goal:continue], [goal:complete], or [goal:blocked:<short reason>].")
	if shouldUseAutoResearch(goal, researchMode) {
		b.WriteString("\n\n")
		b.WriteString(autoResearchGoalInstructions)
	}
	b.WriteString("\n")
	b.WriteString(activeGoalClose)
	return b.String()
}

const autoResearchGoalInstructions = `AutoResearch protocol: this goal looks like long-horizon research, debugging, optimization, or implementation work. Treat AutoResearch as a durable strategy for this Goal, not as a background daemon or a global skill.
- Say briefly in the first visible reply that the goal is being handled with AutoResearch and that state will live under .clouvia/autoresearch/<task-id>/.
- Keep dynamic state out of CLOUVIA.md, KWU.md, SUIYUAN.md, REASONIX.md, AGENTS.md, project memory, system prompts, and tool schemas. Use project-local .clouvia/autoresearch/ state only.
- For a new task, create a collision-resistant task id YYYYMMDD-HHMMSS-slug, check .clouvia/autoresearch/ first, and append -2, -3, etc. only on collision. Reuse an explicitly supplied legacy autoresearch path exactly.
- Maintain state/task_spec.md, state/progress.json, state/findings.jsonl, state/directions_tried.json, state/iteration_log.jsonl, and logs/heartbeat.jsonl. Record goal, scope, non-goals, allowed operations, success criteria, verification gates, iteration direction, evidence, stale_count, pivots, blockers, and completion summary.
- Before each iteration, read the existing state files as authoritative, append a heartbeat, choose a direction that differs materially from directions already tried, execute the smallest evidence-producing chunk, verify it, then persist JSON/JSONL state before reporting.
- Increment stale_count when an iteration lacks accepted evidence or repeats a prior direction. At stale_count >= 2, make a structural pivot such as changing evidence source, entrypoint, implementation boundary, test oracle, benchmark, decomposition, environment, platform, or refutation angle. At stale_count >= 4, stop autonomous digging and ask for the smallest external input needed.
- Workers or subagents may gather evidence, but the orchestrator owns canonical state writes. Workers must not publish, push, delete, contact external systems, or write canonical state unless explicitly designated.
- Complete only after auditing every success criterion in task_spec.md against direct evidence. Public publishing, destructive changes, credential use, payments, external notifications, privacy-sensitive output, and cache-sensitive changes still require the normal ` + brand.DisplayName + ` gates.`

func shouldUseAutoResearch(goal string, mode GoalResearchMode) bool {
	switch mode {
	case GoalResearchOn:
		return true
	case GoalResearchOff:
		return false
	}
	return isAutoResearchGoal(goal)
}

func shouldAutoStartResearchGoal(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "!") {
		return false
	}
	lower := strings.ToLower(trimmed)
	if containsAutoResearchPath(lower) {
		return true
	}
	for _, phrase := range autoResearchAutoStartPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	categories := autoResearchPhaseCount(lower)
	switch {
	case strings.Contains(lower, "彻底") && categories >= 3:
		return true
	case strings.Contains(lower, "完整") && categories >= 3:
		return true
	case strings.Contains(lower, "长期") && categories >= 2 && containsAnyGoalKeyword(lower, []string{"实验", "验证", "修复", "排查", "优化"}):
		return true
	case strings.Contains(lower, "thoroughly") && categories >= 3:
		return true
	case strings.Contains(lower, "complete") && categories >= 3:
		return true
	}
	return false
}

func isAutoResearchGoal(goal string) bool {
	trimmed := strings.TrimSpace(goal)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if containsAutoResearchPath(lower) {
		return true
	}
	for _, kw := range autoResearchStrongKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return autoResearchPhaseCount(lower) >= 4
}

func containsAutoResearchPath(lower string) bool {
	for _, path := range []string{
		".clouvia/autoresearch/",
		".kwu/autoresearch/",
		".suiyuan/autoresearch/",
		".reasonix/autoresearch/",
	} {
		if strings.Contains(lower, path) {
			return true
		}
	}
	return false
}

func autoResearchPhaseCount(lower string) int {
	categories := 0
	for _, group := range autoResearchPhaseKeywords {
		if containsAnyGoalKeyword(lower, group) {
			categories++
		}
	}
	return categories
}

var autoResearchAutoStartPhrases = []string{
	"直到根因",
	"根因明确",
	"多轮排查",
	"不要原地打转",
	"别原地打转",
	"完整做成方案",
	"完整方案并验证",
	"跑实验",
	"反复验证",
	"系统性研究",
	"持续研究",
	"持续排查",
	"持续推进",
	"长期跑",
	"until the root cause",
	"root cause is clear",
	"debug until",
	"do not spin",
	"don't spin",
	"keep researching",
	"long-horizon",
	"long horizon",
	"long-running",
}

var autoResearchStrongKeywords = []string{
	"持续",
	"长期",
	"彻底",
	"直到根因",
	"根因明确",
	"多轮",
	"不要原地打转",
	"别原地打转",
	"完整方案",
	"完整做成方案",
	"跑实验",
	"反复验证",
	"长期优化",
	"系统性研究",
	"持续研究",
	"持续排查",
	"持续推进",
	"长期跑",
	"long-horizon",
	"long horizon",
	"long-running",
	"keep researching",
	"keep working",
	"root cause",
	"until the root cause",
	"do not spin",
	"don't spin",
	"thoroughly",
	"systematically",
}

var autoResearchPhaseKeywords = [][]string{
	{"研究", "调研", "排查", "分析", "定位", "诊断", "research", "investigate", "diagnose", "analyze", "analysis"},
	{"实现", "修复", "改造", "开发", "重构", "implement", "build", "fix", "refactor"},
	{"验证", "测试", "复现", "联调", "benchmark", "verify", "validate", "test", "reproduce"},
	{"优化", "完善", "提升", "收敛", "optimize", "improve", "tune", "polish"},
	{"文档", "方案", "说明", "总结", "document", "docs", "writeup", "plan"},
	{"发布", "上线", "提交", "pull request", "publish", "ship", "deploy"},
}

func containsAnyGoalKeyword(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// MemoryQuickAddNote parses the "# <note>" memory shortcut. The space after
// "#" is intentional: "#7", "#issue", and "#标题" are ordinary user prompts,
// not memory writes. Multi-line input starting with "# " is NOT treated as a
// quick-add note — it is almost certainly a Markdown heading in a structured
// prompt (e.g. "# Context\n\n- file.go\n# Objective"). Only single-line input
// may be a quick-add note.
func MemoryQuickAddNote(input string) (note string, ok bool) {
	trimmed := strings.TrimSpace(input)
	if strings.Contains(trimmed, "\n") {
		return "", false
	}
	if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "#\t") {
		return strings.TrimSpace(trimmed[1:]), true
	}
	return "", false
}

// RememberCommandNote parses the explicit "/remember <note>" memory command.
func RememberCommandNote(input string) (note string, ok bool) {
	trimmed := strings.TrimSpace(input)
	switch {
	case trimmed == "/remember":
		return "", true
	case strings.HasPrefix(trimmed, "/remember ") || strings.HasPrefix(trimmed, "/remember\t"):
		return strings.TrimSpace(trimmed[len("/remember"):]), true
	default:
		return "", false
	}
}

type GoalCommandAction int

const (
	GoalCommandStatus GoalCommandAction = iota + 1
	GoalCommandSet
	GoalCommandClear
)

type GoalCommand struct {
	Action       GoalCommandAction
	Text         string
	Strict       bool
	ResearchMode GoalResearchMode
}

func ParseGoalCommand(input string) (GoalCommand, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed != "/goal" && !strings.HasPrefix(trimmed, "/goal ") && !strings.HasPrefix(trimmed, "/goal\t") {
		return GoalCommand{}, false
	}
	args := strings.TrimSpace(trimmed[len("/goal"):])
	strict, researchMode, actionArgs := parseLeadingGoalFlags(args)

	switch strings.ToLower(actionArgs) {
	case "", "status":
		return GoalCommand{Action: GoalCommandStatus, Strict: strict, ResearchMode: researchMode}, true
	case "clear", "off", "stop", "done":
		return GoalCommand{Action: GoalCommandClear, Strict: strict, ResearchMode: researchMode}, true
	default:
		return GoalCommand{Action: GoalCommandSet, Text: actionArgs, Strict: strict, ResearchMode: researchMode}, true
	}
}

func parseLeadingGoalFlags(args string) (bool, GoalResearchMode, string) {
	strict := false
	mode := GoalResearchAuto
	rest := strings.TrimLeftFunc(args, unicode.IsSpace)
	for rest != "" {
		token, after := leadingGoalToken(rest)
		switch strings.ToLower(token) {
		case "--strict":
			strict = true
		case "--research", "--auto-research", "--deep":
			mode = GoalResearchOn
		case "--simple", "--no-research":
			mode = GoalResearchOff
		default:
			return strict, mode, strings.TrimSpace(rest)
		}
		rest = strings.TrimLeftFunc(after, unicode.IsSpace)
	}
	return strict, mode, ""
}

func leadingGoalToken(s string) (string, string) {
	for i, r := range s {
		if unicode.IsSpace(r) {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

// CustomCommand resolves a "/name args…" line against the loaded custom slash
// commands, returning the rendered prompt to send (found=false when no command
// matches). It does not apply the plan-mode marker — call Compose for that.
func (c *Controller) CustomCommand(input string) (sent string, found bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "", false
	}
	name := strings.TrimPrefix(fields[0], "/")
	for _, cmd := range c.Commands() {
		if cmd.Name == name {
			return cmd.Render(fields[1:]), true
		}
	}
	return "", false
}

// RunSkill resolves a "/<name> args…" line against the loaded skills, returning
// the skill's rendered body to send as a turn (found=false when no skill
// matches). Invoking a skill by slash always inlines its body — the model reads
// and follows the playbook in the main loop; a subagent skill's isolation is
// only engaged when the model calls it via run_skill / the dedicated tool. The
// caller applies Compose for plan-mode/memory framing.
func (c *Controller) RunSkill(input string) (sent string, found bool) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "", false
	}
	name := strings.TrimPrefix(fields[0], "/")
	if sk, ok := c.skills.byName(name); ok {
		return skill.Render(sk, strings.Join(fields[1:], " ")), true
	}
	return "", false
}

// MCPPrompt resolves a "/mcp__server__prompt args…" line: it maps the positional
// args onto the prompt's declared arguments and fetches the rendered prompt from
// the MCP server (an async prompts/get). found is false when no such prompt
// exists; err carries a fetch failure. Honours ctx.
func (c *Controller) MCPPrompt(ctx context.Context, input string) (sent string, found bool, err error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return "", false, nil
	}
	name := strings.TrimPrefix(fields[0], "/")

	prompts := c.mcp.prompts()
	idx := -1
	for i := range prompts {
		if prompts[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", false, nil
	}

	args := map[string]string{}
	for i, a := range prompts[idx].Args {
		if i+1 < len(fields) {
			args[a.Name] = fields[i+1]
		}
	}
	text, err := prompts[idx].Get(ctx, args)
	if err != nil {
		return "", true, err
	}
	return text, true, nil
}
