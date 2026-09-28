package tasktree

import (
	"context"
	"strings"
	"testing"
	"time"

	"kwu/internal/event"
)

func TestBuildSummaryInputGroupsStructuredTreeState(t *testing.T) {
	tree := sampleSummaryTree()
	decision := SummaryDecision{
		NodeID:             "deploy",
		Decision:           "Stop before deployment",
		Rationale:          "production approval is missing",
		UserActionRequired: true,
	}

	input := BuildSummaryInput(tree, decision)

	if input.Goal != "Ship release safely" {
		t.Fatalf("goal = %q", input.Goal)
	}
	if input.Tree.TreeID != "tree-summary" || input.Tree.Status != StatusRunning {
		t.Fatalf("tree state = %+v", input.Tree)
	}
	if len(input.Nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(input.Nodes))
	}
	if input.Nodes[1].NodeID != "inspect" || input.Nodes[1].Depth != 1 {
		t.Fatalf("ordered node/depth = %+v", input.Nodes[1])
	}
	if len(input.Completed) != 1 || input.Completed[0].NodeID != "inspect" {
		t.Fatalf("completed = %+v", input.Completed)
	}
	if input.Completed[0].Summary != "Found the release files and risks." {
		t.Fatalf("completed summary = %+v", input.Completed[0])
	}
	if len(input.Failed) != 1 || input.Failed[0].Reason != "tests failed" {
		t.Fatalf("failed = %+v", input.Failed)
	}
	if len(input.Blocked) != 1 || input.Blocked[0].Reason != "needs production approval" {
		t.Fatalf("blocked = %+v", input.Blocked)
	}
	if len(input.Skipped) != 1 || input.Skipped[0].Reason != "skipped after deployment was blocked" {
		t.Fatalf("skipped = %+v", input.Skipped)
	}
	if len(input.Unfinished) != 1 || input.Unfinished[0].NodeID != "root" {
		t.Fatalf("unfinished = %+v", input.Unfinished)
	}
	if len(input.Decisions) != 1 || input.Decisions[0].Decision != "Stop before deployment" {
		t.Fatalf("decisions = %+v", input.Decisions)
	}

	input.Completed[0].Artifacts[0].Data["mutated"] = true
	if _, ok := tree.Nodes["inspect"].Result.Artifacts[0].Data["mutated"]; ok {
		t.Fatal("summary input should clone node artifacts")
	}
}

func TestRenderSummaryAnswersOriginalGoalAndPreservesProblems(t *testing.T) {
	input := BuildSummaryInput(sampleSummaryTree(), SummaryDecision{
		NodeID:             "deploy",
		Decision:           "Ask user to approve deploy",
		Rationale:          "deployment is outside automatic permissions",
		UserActionRequired: true,
	})

	out := RenderSummary(input)

	for _, want := range []string{
		"Original goal",
		"Ship release safely",
		"Direct answer",
		`Partially completed "Ship release safely"`,
		"Completed items",
		"Found the release files and risks.",
		"Failures and blockers",
		"tests failed",
		"needs production approval",
		"skipped after deployment was blocked",
		"User decisions needed",
		"Ask user to approve deploy",
		"Task tree state",
		"[verify] Run verification - failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.HasPrefix(out, "Found the release files") {
		t.Fatalf("summary should be goal-first, not a child-output concat:\n%s", out)
	}
	if out == "Found the release files and risks.\ntests failed\nneeds production approval" {
		t.Fatal("summary must not mechanically concatenate child outputs")
	}
}

func TestRenderSummaryCompletedGoal(t *testing.T) {
	tree := NewTaskTree("tree-done", "Document the API", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.Status = StatusCompleted
	tree.RootIDs = []string{"doc"}
	tree.Nodes["doc"] = &TaskNode{
		ID:     "doc",
		Title:  "Write API docs",
		Status: StatusCompleted,
		Result: &NodeResult{Summary: "API docs were written."},
	}

	out := RenderSummary(BuildSummaryInput(tree))

	if !strings.Contains(out, `Completed "Document the API"`) {
		t.Fatalf("completed direct answer missing:\n%s", out)
	}
	if strings.Contains(out, "failed/blocked/skipped") {
		t.Fatalf("completed summary should not report partial failure:\n%s", out)
	}
}

func TestOrchestratorSummarizeEmitsTaskTreeSummaryAndFinalMessage(t *testing.T) {
	store := NewMemoryStateStore()
	if err := store.SaveSnapshot(context.Background(), Snapshot{Tree: sampleSummaryTree()}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	sink := &captureSink{}
	orch := NewOrchestrator(Options{Store: store, Sink: sink})

	summary, err := orch.Summarize(context.Background(), SummaryDecision{Decision: "Stop after blocked deploy"})
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if !strings.Contains(summary, "Ship release safely") {
		t.Fatalf("summary = %q", summary)
	}

	payload := assertTaskTreePayloadEvent(t, sink.events, event.TaskTreeSummary, "", string(StatusRunning))
	if payload.TreeID != "tree-summary" || payload.Summary != summary {
		t.Fatalf("summary payload = %+v, summary %q", payload, summary)
	}
	if len(sink.events) != 3 {
		t.Fatalf("events = %+v, want TaskTreeSummary, Text, Message", sink.events)
	}
	if sink.events[1].Kind != event.Text || sink.events[1].Text != summary {
		t.Fatalf("text event = %+v", sink.events[1])
	}
	if sink.events[2].Kind != event.Message || sink.events[2].Text != summary {
		t.Fatalf("message event = %+v", sink.events[2])
	}
}

func sampleSummaryTree() *TaskTree {
	tree := NewTaskTree("tree-summary", "Ship release safely", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	tree.Status = StatusRunning
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = &TaskNode{
		ID:       "root",
		Title:    "Ship release",
		Status:   StatusRunning,
		Children: []string{"inspect", "verify", "deploy", "announce"},
	}
	tree.Nodes["inspect"] = &TaskNode{
		ID:       "inspect",
		ParentID: "root",
		Title:    "Inspect release state",
		Status:   StatusCompleted,
		Result: &NodeResult{
			Summary: "Found the release files and risks.",
			Artifacts: []NodeArtifact{{
				Type: "note",
				Name: "risk",
				Data: map[string]any{"severity": "medium"},
			}},
		},
	}
	tree.Nodes["verify"] = &TaskNode{
		ID:        "verify",
		ParentID:  "root",
		Title:     "Run verification",
		Status:    StatusFailed,
		DependsOn: []string{"inspect"},
		Result:    &NodeResult{Summary: "Verification did not pass.", Error: "tests failed"},
	}
	tree.Nodes["deploy"] = &TaskNode{
		ID:        "deploy",
		ParentID:  "root",
		Title:     "Deploy release",
		Status:    StatusBlocked,
		DependsOn: []string{"verify"},
		Result:    &NodeResult{Summary: "Deployment paused.", Blocker: "needs production approval"},
	}
	tree.Nodes["announce"] = &TaskNode{
		ID:        "announce",
		ParentID:  "root",
		Title:     "Announce release",
		Status:    StatusSkipped,
		DependsOn: []string{"deploy"},
		Result:    &NodeResult{Summary: "skipped after deployment was blocked"},
	}
	return tree
}
