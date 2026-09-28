package tasktree

import "fmt"

// Outcome is the tree-level completion classification used by final summaries.
type Outcome string

const (
	OutcomeCompleted      Outcome = "completed"
	OutcomePartialFailure Outcome = "partial_failure"
	OutcomeBlocked        Outcome = "blocked"
	OutcomeIncomplete     Outcome = "incomplete"
)

// OutcomeImpact explains which later node did not complete because an upstream
// node failed, blocked, or was skipped.
type OutcomeImpact struct {
	NodeID      string `json:"node_id"`
	Title       string `json:"title"`
	Status      Status `json:"status"`
	CauseNodeID string `json:"cause_node_id"`
	CauseTitle  string `json:"cause_title"`
	CauseStatus Status `json:"cause_status"`
	CauseReason string `json:"cause_reason,omitempty"`
}

// OutcomeReport is the user-facing rule layer over SummaryInput. It keeps
// completed work visible while making unresolved statuses impossible to hide.
type OutcomeReport struct {
	Outcome     Outcome
	Completed   []SummaryNodeResult
	Failed      []SummaryNodeResult
	Blocked     []SummaryNodeResult
	Skipped     []SummaryNodeResult
	Unfinished  []SummaryNodeState
	Impacts     []OutcomeImpact
	NextActions []string
}

// OverallOutcome returns the first-version tree-level outcome. Blockers take
// precedence because they usually require an explicit user or environment change.
func OverallOutcome(tree *TaskTree) Outcome {
	return BuildOutcomeReport(tree).Outcome
}

// BuildOutcomeReport computes the final-output rule report directly from a tree.
func BuildOutcomeReport(tree *TaskTree) OutcomeReport {
	return OutcomeReportFromSummary(BuildSummaryInput(tree))
}

// OutcomeReportFromSummary computes final-output rules from the structured
// summary input. This avoids consulting child-agent raw transcripts.
func OutcomeReportFromSummary(input SummaryInput) OutcomeReport {
	report := OutcomeReport{
		Outcome:    outcomeFromSummary(input),
		Completed:  input.Completed,
		Failed:     input.Failed,
		Blocked:    input.Blocked,
		Skipped:    input.Skipped,
		Unfinished: input.Unfinished,
	}
	report.Impacts = outcomeImpacts(input)
	report.NextActions = outcomeNextActions(input, report.Impacts)
	return report
}

func outcomeFromSummary(input SummaryInput) Outcome {
	switch {
	case len(input.Blocked) > 0:
		return OutcomeBlocked
	case len(input.Failed) > 0 || len(input.Skipped) > 0:
		return OutcomePartialFailure
	case len(input.Nodes) > 0 && len(input.Unfinished) == 0:
		return OutcomeCompleted
	default:
		return OutcomeIncomplete
	}
}

func outcomeImpacts(input SummaryInput) []OutcomeImpact {
	nodes := make(map[string]SummaryNodeState, len(input.Nodes))
	for _, node := range input.Nodes {
		nodes[node.NodeID] = node
	}
	results := make(map[string]SummaryNodeResult, len(input.Failed)+len(input.Blocked)+len(input.Skipped))
	for _, result := range appendNodeResults(input.Failed, input.Blocked, input.Skipped) {
		results[result.NodeID] = result
	}

	impacts := []OutcomeImpact{}
	for _, node := range input.Nodes {
		if node.Status == StatusCompleted || node.Status == StatusFailed || node.Status == StatusBlocked {
			continue
		}
		cause, ok := upstreamProblem(node, nodes)
		if !ok {
			continue
		}
		causeResult := results[cause.NodeID]
		impacts = append(impacts, OutcomeImpact{
			NodeID:      node.NodeID,
			Title:       node.Title,
			Status:      node.Status,
			CauseNodeID: cause.NodeID,
			CauseTitle:  cause.Title,
			CauseStatus: cause.Status,
			CauseReason: firstNonEmpty(causeResult.Reason, causeResult.Summary),
		})
	}
	return impacts
}

func upstreamProblem(node SummaryNodeState, nodes map[string]SummaryNodeState) (SummaryNodeState, bool) {
	seen := map[string]bool{node.NodeID: true}
	for parentID := node.ParentID; parentID != ""; {
		if seen[parentID] {
			return SummaryNodeState{}, false
		}
		parent, ok := nodes[parentID]
		if !ok {
			return SummaryNodeState{}, false
		}
		if terminalProblemStatus(parent.Status) {
			return parent, true
		}
		seen[parentID] = true
		parentID = parent.ParentID
	}
	for _, depID := range node.DependsOn {
		dep, ok := nodes[depID]
		if ok && terminalProblemStatus(dep.Status) {
			return dep, true
		}
	}
	return SummaryNodeState{}, false
}

func terminalProblemStatus(status Status) bool {
	return status == StatusFailed || status == StatusBlocked || status == StatusSkipped
}

func outcomeNextActions(input SummaryInput, impacts []OutcomeImpact) []string {
	actions := []string{}
	for _, result := range input.Failed {
		actions = append(actions, fmt.Sprintf("Review or retry failed node [%s] %s%s.", result.NodeID, nonEmpty(result.Title, result.NodeID), reasonSuffix(result.Reason)))
	}
	for _, result := range input.Blocked {
		actions = append(actions, fmt.Sprintf("Resolve blocker for [%s] %s%s.", result.NodeID, nonEmpty(result.Title, result.NodeID), reasonSuffix(result.Reason)))
	}
	for _, result := range input.Skipped {
		actions = append(actions, fmt.Sprintf("Decide whether skipped node [%s] %s still needs to run%s.", result.NodeID, nonEmpty(result.Title, result.NodeID), reasonSuffix(result.Reason)))
	}
	for _, state := range input.Unfinished {
		if hasImpact(state.NodeID, impacts) {
			continue
		}
		actions = append(actions, fmt.Sprintf("Continue unfinished node [%s] %s.", state.NodeID, nonEmpty(state.Title, state.NodeID)))
	}
	for _, impact := range impacts {
		actions = append(actions, fmt.Sprintf("After resolving [%s] %s, revisit [%s] %s.", impact.CauseNodeID, nonEmpty(impact.CauseTitle, impact.CauseNodeID), impact.NodeID, nonEmpty(impact.Title, impact.NodeID)))
	}
	return actions
}

func hasImpact(nodeID string, impacts []OutcomeImpact) bool {
	for _, impact := range impacts {
		if impact.NodeID == nodeID {
			return true
		}
	}
	return false
}

func reasonSuffix(reason string) string {
	if reason == "" {
		return ""
	}
	return ": " + reason
}
