package tasktree

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"kwu/internal/agent/testutil"
	"kwu/internal/brand"
	"kwu/internal/provider"
)

func TestLLMPlannerGeneratesStructuredTreeFromSimpleGoal(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	prov := testutil.NewMock("planner", testutil.Turn{Text: validPlannerTreeJSON("write tests")})
	planner := NewLLMPlanner(LLMPlannerOptions{
		Provider:       prov,
		Temperature:    0.25,
		MaxTokens:      1200,
		SessionContext: "The repository uses Go tests.",
		Now:            func() time.Time { return now },
	})

	tree, err := planner.PlanTaskTree(context.Background(), "write tests")
	if err != nil {
		t.Fatalf("PlanTaskTree: %v", err)
	}
	if tree.ID != "tree-write-tests" || tree.Goal != "write tests" || tree.Status != StatusPending {
		t.Fatalf("tree identity/status = %+v", tree)
	}
	if len(tree.RootIDs) != 1 || tree.RootIDs[0] != "inspect" {
		t.Fatalf("root_ids = %v", tree.RootIDs)
	}
	if got := tree.Nodes["implement"].DependsOn; len(got) != 1 || got[0] != "inspect" {
		t.Fatalf("implement dependencies = %v", got)
	}
	if tree.Nodes["verify"].Executor == nil || tree.Nodes["verify"].Executor.Kind != ExecutorReadOnlyTask {
		t.Fatalf("verify executor = %+v", tree.Nodes["verify"].Executor)
	}
	if !tree.CreatedAt.Equal(now) || !tree.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %s/%s, want %s", tree.CreatedAt, tree.UpdatedAt, now)
	}

	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(reqs))
	}
	req := reqs[0]
	if len(req.Tools) != 0 {
		t.Fatalf("planner request exposed tools: %v", req.Tools)
	}
	if req.Temperature != 0.25 || req.MaxTokens != 1200 {
		t.Fatalf("request sampling = temperature %v max_tokens %d", req.Temperature, req.MaxTokens)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != provider.RoleSystem || req.Messages[1].Role != provider.RoleUser {
		t.Fatalf("request messages = %+v", req.Messages)
	}
	if !strings.Contains(req.Messages[1].Content, "write tests") || !strings.Contains(req.Messages[1].Content, "The repository uses Go tests.") {
		t.Fatalf("planner prompt missed goal/context:\n%s", req.Messages[1].Content)
	}
}

func TestTaskTreePlannerPromptNamesRequiredStructure(t *testing.T) {
	text := TaskTreePlannerSystemPrompt + "\n" + RenderTaskTreePrompt("ship feature", "use current session")
	for _, want := range []string{
		brand.DisplayName + " task_tree planner",
		"node hierarchy",
		"order",
		"dependencies",
		"executor suggestions",
		"expected results",
		"do not write files",
		"do not execute commands",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("planner prompt missing %q:\n%s", want, text)
		}
	}
}

func TestLLMPlannerRejectsMalformedTreeOutput(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{
			name: "missing node id",
			text: `{
  "id": "tree-bad",
  "goal": "bad",
  "root_ids": ["root"],
  "status": "pending",
  "nodes": {
    "root": {"title": "Root", "status": "pending", "expected_output": "A result"}
  }
}`,
		},
		{
			name: "missing node status",
			text: `{
  "id": "tree-bad",
  "goal": "bad",
  "root_ids": ["root"],
  "status": "pending",
  "nodes": {
    "root": {"id": "root", "title": "Root", "expected_output": "A result"}
  }
}`,
		},
		{
			name: "illegal dependency",
			text: `{
  "id": "tree-bad",
  "goal": "bad",
  "root_ids": ["root"],
  "status": "pending",
  "nodes": {
    "root": {"id": "root", "title": "Root", "depends_on": ["missing"], "status": "pending", "expected_output": "A result"}
  }
}`,
		},
		{
			name: "dependency cycle",
			text: `{
  "id": "tree-bad",
  "goal": "bad",
  "root_ids": ["a", "b"],
  "status": "pending",
  "nodes": {
    "a": {"id": "a", "title": "A", "depends_on": ["b"], "status": "pending", "expected_output": "A result"},
    "b": {"id": "b", "title": "B", "depends_on": ["a"], "status": "pending", "expected_output": "B result"}
  }
}`,
		},
		{
			name: "markdown plan",
			text: "- inspect\n- implement\n- verify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := testutil.NewMock("planner", testutil.Turn{Text: tt.text})
			planner := NewLLMPlanner(LLMPlannerOptions{Provider: prov})

			_, err := planner.PlanTaskTree(context.Background(), "bad")
			if !errors.Is(err, ErrInvalidPlannerOutput) {
				t.Fatalf("PlanTaskTree error = %v, want ErrInvalidPlannerOutput", err)
			}
		})
	}
}

func TestLLMPlannerRejectsToolCalls(t *testing.T) {
	prov := testutil.NewMock("planner", testutil.Turn{
		Chunks: []provider.Chunk{
			{Type: provider.ChunkToolCallStart, ToolCall: &provider.ToolCall{ID: "call-1", Name: "bash"}},
		},
	})
	planner := NewLLMPlanner(LLMPlannerOptions{Provider: prov})

	_, err := planner.PlanTaskTree(context.Background(), "write a file")
	if !errors.Is(err, ErrPlannerToolCall) {
		t.Fatalf("PlanTaskTree error = %v, want ErrPlannerToolCall", err)
	}
	reqs := prov.Requests()
	if len(reqs) != 1 {
		t.Fatalf("provider requests = %d, want 1", len(reqs))
	}
	if len(reqs[0].Tools) != 0 {
		t.Fatalf("planner request exposed tools: %v", reqs[0].Tools)
	}
}

func TestParseTreeJSONAcceptsFencedWrapper(t *testing.T) {
	tree, err := ParseTreeJSON("```json\n{\"tree\":" + validPlannerTreeJSON("wrapped") + "}\n```")
	if err != nil {
		t.Fatalf("ParseTreeJSON: %v", err)
	}
	tree, err = NormalizeAndValidateGeneratedTree(tree, "wrapped", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NormalizeAndValidateGeneratedTree: %v", err)
	}
	if tree.ID != "tree-write-tests" {
		t.Fatalf("tree ID = %q", tree.ID)
	}
}

func validPlannerTreeJSON(goal string) string {
	return `{
  "id": "tree-write-tests",
  "goal": "` + goal + `",
  "root_ids": ["inspect"],
  "status": "pending",
  "nodes": {
    "inspect": {
      "id": "inspect",
      "title": "Inspect current tests",
      "children": ["implement"],
      "status": "pending",
      "executor": {"kind": "read_only_task"},
      "expected_output": "A concise list of relevant test gaps"
    },
    "implement": {
      "id": "implement",
      "parent_id": "inspect",
      "title": "Implement focused tests",
      "children": ["verify"],
      "depends_on": ["inspect"],
      "status": "pending",
      "executor": {"kind": "main"},
      "expected_output": "Focused tests added to the repository"
    },
    "verify": {
      "id": "verify",
      "parent_id": "implement",
      "title": "Run verification",
      "depends_on": ["implement"],
      "status": "pending",
      "executor": {"kind": "read_only_task"},
      "tool_hints": ["go test ./internal/tasktree ./internal/provider"],
      "expected_output": "Relevant tests pass"
    }
  }
}`
}
