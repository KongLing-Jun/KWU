package tasktree

import (
	"strings"
	"testing"
	"time"
)

func TestOverallOutcomeDistinguishesTerminalStatuses(t *testing.T) {
	tests := []struct {
		name string
		tree *TaskTree
		want Outcome
	}{
		{name: "completed", tree: outcomeTree(StatusCompleted), want: OutcomeCompleted},
		{name: "incomplete", tree: outcomeTree(StatusPending), want: OutcomeIncomplete},
		{name: "partial failure", tree: outcomeTree(StatusCompleted, StatusFailed), want: OutcomePartialFailure},
		{name: "skipped is partial", tree: outcomeTree(StatusCompleted, StatusSkipped), want: OutcomePartialFailure},
		{name: "blocked takes precedence", tree: outcomeTree(StatusCompleted, StatusFailed, StatusBlocked), want: OutcomeBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OverallOutcome(tt.tree); got != tt.want {
				t.Fatalf("OverallOutcome = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutcomeReportKeepsCompletedWorkAndImpactedFollowups(t *testing.T) {
	report := BuildOutcomeReport(sampleSummaryTree())

	if report.Outcome != OutcomeBlocked {
		t.Fatalf("outcome = %q, want blocked", report.Outcome)
	}
	if len(report.Completed) != 1 || report.Completed[0].NodeID != "inspect" {
		t.Fatalf("completed work should remain visible: %+v", report.Completed)
	}
	if len(report.Failed) != 1 || report.Failed[0].NodeID != "verify" {
		t.Fatalf("failed nodes = %+v", report.Failed)
	}
	if len(report.Blocked) != 1 || report.Blocked[0].NodeID != "deploy" {
		t.Fatalf("blocked nodes = %+v", report.Blocked)
	}
	if len(report.Skipped) != 1 || report.Skipped[0].NodeID != "announce" {
		t.Fatalf("skipped nodes = %+v", report.Skipped)
	}
	if len(report.Impacts) != 1 {
		t.Fatalf("impacts = %+v, want one impacted follow-up", report.Impacts)
	}
	impact := report.Impacts[0]
	if impact.NodeID != "announce" || impact.CauseNodeID != "deploy" || impact.CauseStatus != StatusBlocked {
		t.Fatalf("impact = %+v, want announce blocked by deploy", impact)
	}
	if !strings.Contains(strings.Join(report.NextActions, "\n"), "Resolve blocker for [deploy]") {
		t.Fatalf("next actions missing blocker: %+v", report.NextActions)
	}
}

func TestRenderSummaryPartialFailureDoesNotClaimGoalCompleted(t *testing.T) {
	tree := partialFailureTree()
	out := RenderSummary(BuildSummaryInput(tree))

	if !strings.Contains(out, `Partially completed "Publish release notes"`) {
		t.Fatalf("partial failure direct answer missing:\n%s", out)
	}
	if strings.Contains(out, `Completed "Publish release notes" based on the structured task tree`) {
		t.Fatalf("partial failure summary must not claim full completion:\n%s", out)
	}
	for _, want := range []string{
		"Drafted release notes",
		"formatter crashed",
		"[publish] Publish notes remained pending because [format] Format notes is failed: formatter crashed",
		"After resolving [format] Format notes, revisit [publish] Publish notes.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("partial failure summary missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSummaryBlockedOutputNamesBlockerAndSkippedReason(t *testing.T) {
	out := RenderSummary(BuildSummaryInput(sampleSummaryTree()))

	for _, want := range []string{
		`Partially completed "Ship release safely", but blocked before full completion: needs production approval.`,
		"- completed [inspect] Inspect release state: Found the release files and risks.",
		"- failed [verify] Run verification: tests failed",
		"- blocked [deploy] Deploy release: needs production approval",
		"- skipped [announce] Announce release: skipped after deployment was blocked",
		"[announce] Announce release remained skipped because [deploy] Deploy release is blocked: needs production approval",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("blocked summary missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `Completed "Ship release safely" based on the structured task tree`) {
		t.Fatalf("blocked summary must not claim completion:\n%s", out)
	}
}

func outcomeTree(statuses ...Status) *TaskTree {
	tree := NewTaskTree("tree-outcome", "Classify outcome", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.Status = StatusCompleted
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = &TaskNode{
		ID:     "root",
		Title:  "Root",
		Status: StatusCompleted,
	}
	for i, status := range statuses {
		id := string(rune('a' + i))
		tree.Nodes["root"].Children = append(tree.Nodes["root"].Children, id)
		tree.Nodes[id] = &TaskNode{
			ID:       id,
			ParentID: "root",
			Title:    id,
			Status:   status,
			Result:   resultForStatus(status),
		}
		if status != StatusCompleted {
			tree.Status = StatusRunning
		}
	}
	return tree
}

func resultForStatus(status Status) *NodeResult {
	switch status {
	case StatusCompleted:
		return &NodeResult{Summary: "done"}
	case StatusFailed:
		return &NodeResult{Summary: "failed", Error: "failed"}
	case StatusBlocked:
		return &NodeResult{Summary: "blocked", Blocker: "blocked"}
	case StatusSkipped:
		return &NodeResult{Summary: "skipped"}
	default:
		return nil
	}
}

func partialFailureTree() *TaskTree {
	tree := NewTaskTree("tree-partial", "Publish release notes", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.Status = StatusRunning
	tree.RootIDs = []string{"draft", "format", "publish"}
	tree.Nodes["draft"] = &TaskNode{
		ID:     "draft",
		Title:  "Draft notes",
		Status: StatusCompleted,
		Result: &NodeResult{Summary: "Drafted release notes"},
	}
	tree.Nodes["format"] = &TaskNode{
		ID:        "format",
		Title:     "Format notes",
		Status:    StatusFailed,
		DependsOn: []string{"draft"},
		Result:    &NodeResult{Summary: "Formatting did not finish", Error: "formatter crashed"},
	}
	tree.Nodes["publish"] = &TaskNode{
		ID:        "publish",
		Title:     "Publish notes",
		Status:    StatusPending,
		DependsOn: []string{"format"},
	}
	return tree
}
