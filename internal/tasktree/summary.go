package tasktree

import (
	"context"
	"fmt"
	"strings"

	"kwu/internal/event"
)

// SummaryInput is the structured contract for a task-tree final answer. A
// dedicated summarizer can consume this value directly; the deterministic
// renderer below is the first-version fallback.
type SummaryInput struct {
	Goal        string              `json:"goal"`
	Tree        SummaryTreeState    `json:"tree"`
	Outcome     Outcome             `json:"outcome"`
	Nodes       []SummaryNodeState  `json:"nodes"`
	Completed   []SummaryNodeResult `json:"completed"`
	Failed      []SummaryNodeResult `json:"failed"`
	Blocked     []SummaryNodeResult `json:"blocked"`
	Skipped     []SummaryNodeResult `json:"skipped"`
	Unfinished  []SummaryNodeState  `json:"unfinished"`
	Impacts     []OutcomeImpact     `json:"impacts,omitempty"`
	NextActions []string            `json:"next_actions,omitempty"`
	Decisions   []SummaryDecision   `json:"decisions,omitempty"`
}

// SummaryTreeState is the tree-level state included in the summarizer input.
type SummaryTreeState struct {
	TreeID  string   `json:"tree_id"`
	Status  Status   `json:"status"`
	RootIDs []string `json:"root_ids,omitempty"`
}

// SummaryNodeState is a stable snapshot of one node in tree order.
type SummaryNodeState struct {
	NodeID    string             `json:"node_id"`
	Title     string             `json:"title"`
	ParentID  string             `json:"parent_id,omitempty"`
	Status    Status             `json:"status"`
	Depth     int                `json:"depth"`
	DependsOn []string           `json:"depends_on,omitempty"`
	Children  []string           `json:"children,omitempty"`
	Result    *SummaryNodeResult `json:"result,omitempty"`
}

// SummaryNodeResult keeps user-visible node results separate from raw child
// agent transcripts. RawOutputRef may point to internal storage, but RenderSummary
// uses Summary/Error/Blocker fields instead of concatenating raw output.
type SummaryNodeResult struct {
	NodeID       string         `json:"node_id"`
	Title        string         `json:"title"`
	Status       Status         `json:"status"`
	Summary      string         `json:"summary,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Error        string         `json:"error,omitempty"`
	Blocker      string         `json:"blocker,omitempty"`
	Artifacts    []NodeArtifact `json:"artifacts,omitempty"`
	RawOutputRef string         `json:"raw_output_ref,omitempty"`
}

// SummaryDecision records key orchestration decisions that are not themselves
// node output, for example why a branch was skipped or why execution stopped.
type SummaryDecision struct {
	NodeID             string `json:"node_id,omitempty"`
	Decision           string `json:"decision"`
	Rationale          string `json:"rationale,omitempty"`
	UserActionRequired bool   `json:"user_action_required,omitempty"`
}

// BuildSummaryInput converts the current structured tree state into the input
// contract used for final answering. It preserves terminal failure/blocker data
// and keeps node results attributed to node IDs.
func BuildSummaryInput(tree *TaskTree, decisions ...SummaryDecision) SummaryInput {
	if tree == nil {
		return SummaryInput{Decisions: normalizeSummaryDecisions(decisions)}
	}
	input := SummaryInput{
		Goal: strings.TrimSpace(tree.Goal),
		Tree: SummaryTreeState{
			TreeID:  tree.ID,
			Status:  tree.Status,
			RootIDs: cloneStrings(tree.RootIDs),
		},
		Decisions: normalizeSummaryDecisions(decisions),
	}
	for _, id := range orderedNodeIDsForEvent(tree) {
		node, ok := tree.Node(id)
		if !ok {
			continue
		}
		state := summaryNodeState(tree, node)
		input.Nodes = append(input.Nodes, state)
		switch node.Status {
		case StatusCompleted:
			input.Completed = append(input.Completed, summaryNodeResult(node))
		case StatusFailed:
			input.Failed = append(input.Failed, summaryNodeResult(node))
		case StatusBlocked:
			input.Blocked = append(input.Blocked, summaryNodeResult(node))
		case StatusSkipped:
			input.Skipped = append(input.Skipped, summaryNodeResult(node))
		default:
			input.Unfinished = append(input.Unfinished, state)
		}
	}
	report := OutcomeReportFromSummary(input)
	input.Outcome = report.Outcome
	input.Impacts = report.Impacts
	input.NextActions = report.NextActions
	return input
}

// RenderSummary renders a first-version final answer from SummaryInput. It is
// deliberately goal-first and status-aware instead of being a concatenation of
// sub-agent responses.
func RenderSummary(input SummaryInput) string {
	var b strings.Builder
	goal := input.Goal
	if strings.TrimSpace(goal) == "" {
		goal = "the original task-tree goal"
	}

	writeSection(&b, "Original goal")
	b.WriteString(goal)
	b.WriteString("\n\n")

	writeSection(&b, "Direct answer")
	b.WriteString(directSummaryAnswer(goal, input))
	b.WriteString("\n\n")

	writeSection(&b, "Completed items")
	writeNodeResults(&b, input.Completed)
	b.WriteString("\n")

	writeSection(&b, "Unfinished items")
	writeNodeStates(&b, input.Unfinished)
	b.WriteString("\n")

	writeSection(&b, "Failures and blockers")
	writeNodeResults(&b, appendNodeResults(input.Failed, input.Blocked, input.Skipped))
	b.WriteString("\n")

	writeSection(&b, "Failure impact scope")
	writeOutcomeImpacts(&b, input.Impacts)
	b.WriteString("\n")

	writeSection(&b, "User decisions needed")
	writeUserDecisions(&b, input)
	b.WriteString("\n")

	writeSection(&b, "Next actions")
	writeNextActions(&b, input.NextActions)
	b.WriteString("\n")

	writeSection(&b, "Key orchestration decisions")
	writeSummaryDecisions(&b, input.Decisions, false)
	b.WriteString("\n")

	writeSection(&b, "Task tree state")
	writeTreeState(&b, input)

	return strings.TrimRight(b.String(), "\n")
}

// Summarize builds, renders, and emits the task-tree final summary from the
// current store snapshot. It does not mutate normal agent final-answer readiness.
func (o *Orchestrator) Summarize(ctx context.Context, decisions ...SummaryDecision) (string, error) {
	tree, err := o.loadTaskTree(ctx)
	if err != nil {
		return "", err
	}
	summary := RenderSummary(BuildSummaryInput(tree, decisions...))
	o.emitTreeSummary(tree, summary)
	o.sink.Emit(event.Event{Kind: event.Text, Text: summary, Source: SnapshotModeTaskTree})
	o.sink.Emit(event.Event{Kind: event.Message, Text: summary, Source: SnapshotModeTaskTree})
	return summary, nil
}

func summaryNodeState(tree *TaskTree, node *TaskNode) SummaryNodeState {
	state := SummaryNodeState{
		NodeID:    node.ID,
		Title:     node.Title,
		ParentID:  node.ParentID,
		Status:    node.Status,
		Depth:     nodeDepth(tree, node),
		DependsOn: cloneStrings(node.DependsOn),
		Children:  cloneStrings(node.Children),
	}
	if node.Result != nil {
		result := summaryNodeResult(node)
		state.Result = &result
	}
	return state
}

func summaryNodeResult(node *TaskNode) SummaryNodeResult {
	result := SummaryNodeResult{
		NodeID: node.ID,
		Title:  node.Title,
		Status: node.Status,
		Reason: summaryReason(node),
	}
	if node.Result != nil {
		result.Summary = strings.TrimSpace(node.Result.Summary)
		result.Error = strings.TrimSpace(node.Result.Error)
		result.Blocker = strings.TrimSpace(node.Result.Blocker)
		result.Artifacts = cloneArtifacts(node.Result.Artifacts)
		result.RawOutputRef = strings.TrimSpace(node.Result.RawOutputRef)
	}
	return result
}

func summaryReason(node *TaskNode) string {
	if node == nil {
		return ""
	}
	if node.Result != nil {
		if text := strings.TrimSpace(node.Result.Blocker); text != "" {
			return text
		}
		if text := strings.TrimSpace(node.Result.Error); text != "" {
			return text
		}
		if text := strings.TrimSpace(node.Result.Summary); text != "" {
			return text
		}
	}
	switch node.Status {
	case StatusSkipped:
		return "skipped before execution"
	case StatusFailed:
		return "failed without a recorded error"
	case StatusBlocked:
		return "blocked without a recorded blocker"
	default:
		return ""
	}
}

func nodeDepth(tree *TaskTree, node *TaskNode) int {
	depth := 0
	seen := map[string]bool{node.ID: true}
	for parentID := node.ParentID; parentID != ""; {
		if seen[parentID] {
			return depth
		}
		parent, ok := tree.Node(parentID)
		if !ok {
			return depth
		}
		depth++
		seen[parentID] = true
		parentID = parent.ParentID
	}
	return depth
}

func normalizeSummaryDecisions(in []SummaryDecision) []SummaryDecision {
	out := make([]SummaryDecision, 0, len(in))
	for _, decision := range in {
		decision.NodeID = strings.TrimSpace(decision.NodeID)
		decision.Decision = strings.TrimSpace(decision.Decision)
		decision.Rationale = strings.TrimSpace(decision.Rationale)
		if decision.Decision == "" && decision.Rationale == "" {
			continue
		}
		out = append(out, decision)
	}
	return out
}

func directSummaryAnswer(goal string, input SummaryInput) string {
	completed := len(input.Completed)
	problems := len(input.Failed) + len(input.Blocked) + len(input.Skipped)
	unfinished := len(input.Unfinished)
	switch input.Outcome {
	case OutcomeCompleted:
		return fmt.Sprintf("Completed %q based on the structured task tree.", goal)
	case OutcomeBlocked:
		reason := firstProblemReason(input.Blocked)
		if completed > 0 {
			return fmt.Sprintf("Partially completed %q, but blocked before full completion%s.", goal, reasonSuffix(reason))
		}
		return fmt.Sprintf("Could not complete %q because the task tree is blocked%s.", goal, reasonSuffix(reason))
	case OutcomePartialFailure:
		return fmt.Sprintf("Partially completed %q: %d node(s) completed, with %d failed/blocked/skipped and %d still unfinished.", goal, completed, problems, unfinished)
	default:
		return fmt.Sprintf("Could not complete %q yet; %d node(s) remain unfinished.", goal, unfinished)
	}
}

func writeSection(b *strings.Builder, title string) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
}

func writeNodeResults(b *strings.Builder, results []SummaryNodeResult) {
	if len(results) == 0 {
		b.WriteString("- None.\n")
		return
	}
	for _, result := range results {
		b.WriteString("- ")
		b.WriteString(string(result.Status))
		b.WriteString(" [")
		b.WriteString(result.NodeID)
		b.WriteString("] ")
		b.WriteString(nonEmpty(result.Title, result.NodeID))
		detail := firstNonEmpty(result.Summary, result.Reason)
		if result.Status == StatusFailed || result.Status == StatusBlocked || result.Status == StatusSkipped {
			detail = firstNonEmpty(result.Reason, result.Summary)
		}
		if detail != "" {
			b.WriteString(": ")
			b.WriteString(detail)
		}
		b.WriteString("\n")
	}
}

func writeNodeStates(b *strings.Builder, states []SummaryNodeState) {
	if len(states) == 0 {
		b.WriteString("- None.\n")
		return
	}
	for _, state := range states {
		b.WriteString("- ")
		b.WriteString(string(state.Status))
		b.WriteString(" [")
		b.WriteString(state.NodeID)
		b.WriteString("] ")
		b.WriteString(nonEmpty(state.Title, state.NodeID))
		b.WriteString("\n")
	}
}

func writeUserDecisions(b *strings.Builder, input SummaryInput) {
	wrote := false
	for _, result := range input.Blocked {
		wrote = true
		b.WriteString("- Resolve blocker for [")
		b.WriteString(result.NodeID)
		b.WriteString("] ")
		b.WriteString(nonEmpty(result.Title, result.NodeID))
		if result.Reason != "" {
			b.WriteString(": ")
			b.WriteString(result.Reason)
		}
		b.WriteString("\n")
	}
	for _, decision := range input.Decisions {
		if !decision.UserActionRequired {
			continue
		}
		wrote = true
		writeDecision(b, decision)
	}
	if !wrote {
		b.WriteString("- None.\n")
	}
}

func writeOutcomeImpacts(b *strings.Builder, impacts []OutcomeImpact) {
	if len(impacts) == 0 {
		b.WriteString("- None.\n")
		return
	}
	for _, impact := range impacts {
		b.WriteString("- [")
		b.WriteString(impact.NodeID)
		b.WriteString("] ")
		b.WriteString(nonEmpty(impact.Title, impact.NodeID))
		b.WriteString(" remained ")
		b.WriteString(string(impact.Status))
		b.WriteString(" because [")
		b.WriteString(impact.CauseNodeID)
		b.WriteString("] ")
		b.WriteString(nonEmpty(impact.CauseTitle, impact.CauseNodeID))
		b.WriteString(" is ")
		b.WriteString(string(impact.CauseStatus))
		if impact.CauseReason != "" {
			b.WriteString(": ")
			b.WriteString(impact.CauseReason)
		}
		b.WriteString("\n")
	}
}

func writeNextActions(b *strings.Builder, actions []string) {
	if len(actions) == 0 {
		b.WriteString("- None.\n")
		return
	}
	for _, action := range actions {
		b.WriteString("- ")
		b.WriteString(action)
		b.WriteString("\n")
	}
}

func writeSummaryDecisions(b *strings.Builder, decisions []SummaryDecision, userOnly bool) {
	wrote := false
	for _, decision := range decisions {
		if userOnly && !decision.UserActionRequired {
			continue
		}
		wrote = true
		writeDecision(b, decision)
	}
	if !wrote {
		b.WriteString("- None.\n")
	}
}

func writeDecision(b *strings.Builder, decision SummaryDecision) {
	b.WriteString("- ")
	if decision.NodeID != "" {
		b.WriteString("[")
		b.WriteString(decision.NodeID)
		b.WriteString("] ")
	}
	b.WriteString(decision.Decision)
	if decision.Rationale != "" {
		b.WriteString(": ")
		b.WriteString(decision.Rationale)
	}
	b.WriteString("\n")
}

func writeTreeState(b *strings.Builder, input SummaryInput) {
	b.WriteString("- Tree [")
	b.WriteString(input.Tree.TreeID)
	b.WriteString("] status: ")
	b.WriteString(string(input.Tree.Status))
	b.WriteString("\n")
	for _, state := range input.Nodes {
		b.WriteString("- ")
		b.WriteString(strings.Repeat("  ", state.Depth))
		b.WriteString("[")
		b.WriteString(state.NodeID)
		b.WriteString("] ")
		b.WriteString(nonEmpty(state.Title, state.NodeID))
		b.WriteString(" - ")
		b.WriteString(string(state.Status))
		b.WriteString("\n")
	}
}

func appendNodeResults(groups ...[]SummaryNodeResult) []SummaryNodeResult {
	var total int
	for _, group := range groups {
		total += len(group)
	}
	out := make([]SummaryNodeResult, 0, total)
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

func firstProblemReason(results []SummaryNodeResult) string {
	for _, result := range results {
		if reason := firstNonEmpty(result.Reason, result.Summary); reason != "" {
			return reason
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}

func nonEmpty(value, fallback string) string {
	if text := strings.TrimSpace(value); text != "" {
		return text
	}
	return fallback
}
