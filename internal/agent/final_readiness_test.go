package agent

import (
	"context"
	"io"
	"strings"
	"testing"

	"kwu/internal/event"
	"kwu/internal/evidence"
	"kwu/internal/instruction"
	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func readinessLedger(receipts ...evidence.Receipt) *evidence.Ledger {
	l := evidence.NewLedger()
	for _, r := range receipts {
		l.Record(r)
	}
	return l
}

func TestFinalReadinessFailureBranches(t *testing.T) {
	check := instruction.VerifyCheck{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	readOnly := evidence.Receipt{ToolName: "read_file", Success: true, Read: true, Paths: []string{"a.go"}}
	checkAfter := evidence.Receipt{ToolName: "bash", Success: true, Command: "go test ./..."}
	todo := evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{{Content: "edit", Status: "in_progress"}}}
	completeAfter := evidence.Receipt{ToolName: "complete_step", Success: true, Step: "edit"}
	doneTodo := evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{{Content: "edit", Status: "completed"}}}

	cases := []struct {
		name        string
		checks      []instruction.VerifyCheck
		evidence    *evidence.Ledger
		wantEmpty   bool
		wantContain string
	}{
		{"nil evidence never gates", []instruction.VerifyCheck{check}, nil, true, ""},
		{"no writer never gates", []instruction.VerifyCheck{check}, readinessLedger(checkAfter), true, ""},
		{"todo-only turn may end with incomplete list", nil, readinessLedger(todo), true, ""},
		{"read-only context plus todo may end with incomplete list", nil, readinessLedger(readOnly, todo), true, ""},
		{"completed todo without writer satisfies", nil, readinessLedger(doneTodo), true, ""},
		{"writer without checks or todo never gates", nil, readinessLedger(writer), true, ""},
		{"missing project check after writer is reported", []instruction.VerifyCheck{check}, readinessLedger(checkAfter, writer), false, "go test ./..."},
		{"project check run after writer satisfies", []instruction.VerifyCheck{check}, readinessLedger(writer, checkAfter), true, ""},
		{"todo writer without complete_step is reported", nil, readinessLedger(writer, todo), false, "incomplete items"},
		{"complete_step without final todo update is reported", nil, readinessLedger(writer, todo, completeAfter), false, "latest successful todo_write"},
		{"todo writer with complete_step and completed todo satisfies", nil, readinessLedger(writer, todo, completeAfter, doneTodo), true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &Agent{evidence: tc.evidence, projectChecks: tc.checks}
			got := a.finalReadinessFailure()
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("finalReadinessFailure() = %q, want empty (no gate)", got)
				}
				return
			}
			if got == "" {
				t.Fatalf("finalReadinessFailure() = empty, want a failure mentioning %q", tc.wantContain)
			}
			if !strings.Contains(got, tc.wantContain) {
				t.Fatalf("finalReadinessFailure() = %q, want it to mention %q", got, tc.wantContain)
			}
		})
	}
}

func TestFinalReadinessAllowsIncompleteTodosInPlanMode(t *testing.T) {
	todo := evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{{Content: "draft implementation plan", Status: "pending"}}}
	a := &Agent{evidence: readinessLedger(todo)}
	a.SetPlanMode(true)

	if got := a.finalReadinessFailure(); got != "" {
		t.Fatalf("finalReadinessFailure() = %q, want empty in plan mode", got)
	}
	if got := a.finalReadinessCheck(); got.applies {
		t.Fatalf("finalReadinessCheck() applies in plan mode: %+v", got)
	}
}

func TestFinalReadinessCheckAuditsIncompleteTodos(t *testing.T) {
	todo := evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{{Content: "edit", Status: "in_progress"}}}
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{evidence: readinessLedger(writer, todo)}

	got := a.finalReadinessCheck()
	if !got.applies {
		t.Fatalf("finalReadinessCheck() applies = false, want true")
	}
	if got.incompleteTodos != 1 {
		t.Fatalf("incompleteTodos = %d, want 1", got.incompleteTodos)
	}
	if !strings.Contains(got.reason, "latest successful todo_write") {
		t.Fatalf("reason = %q, want incomplete todo message", got.reason)
	}
	audit := got.audit(evidence.ReadinessBlocked, false)
	if audit.IncompleteTodos != 1 {
		t.Fatalf("audit.IncompleteTodos = %d, want 1", audit.IncompleteTodos)
	}
}

func TestFinalReadinessOnlyRequiresRedeliveryAfterDeclarationIsInvalidated(t *testing.T) {
	observed := evidence.Receipt{
		ToolName:      "bash",
		Success:       true,
		ArtifactPaths: []string{"report.docx"},
	}
	decision := evidence.Receipt{
		ToolName:         "deliver_files",
		Success:          true,
		ArtifactDecision: true,
	}

	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	a := &Agent{evidence: readinessLedger(observed), session: session}
	if got := a.finalReadinessCheck(); got.applies || got.reason != "" {
		t.Fatalf("first host-observed artifact unexpectedly required an explicit declaration: %+v", got)
	}

	a.evidence = readinessLedger(observed, decision)
	if got := a.finalReadinessCheck(); got.reason != "" {
		t.Fatalf("decision after observation remained blocked: %+v", got)
	}

	a.evidence = readinessLedger(observed, decision, evidence.Receipt{
		ToolName:         "write_file",
		Success:          true,
		Write:            true,
		ArtifactMutation: true,
		ArtifactPaths:    []string{"appendix.md"},
	})
	got := a.finalReadinessCheck()
	if !got.applies || !strings.Contains(got.reason, "deliver_files") || !strings.Contains(got.reason, "appendix.md") || strings.Contains(got.reason, "report.docx") {
		t.Fatalf("new observation did not invalidate the prior decision: %+v", got)
	}
}

func TestFinalReadinessAllowsUndeclaredMutationAndBlocksOnlyStaleDeclaration(t *testing.T) {
	mutation := evidence.Receipt{
		ToolName:         "opaque_export",
		Success:          true,
		ArtifactMutation: true,
	}
	decision := evidence.Receipt{
		ToolName:         "deliver_files",
		Success:          true,
		ArtifactDecision: true,
	}

	session := NewSession("system")
	session.BeginArtifactDeliveryTurn("turn")
	a := &Agent{evidence: readinessLedger(mutation), session: session}
	if got := a.finalReadinessCheck(); got.applies || got.reason != "" {
		t.Fatalf("mutation without a prior declaration unexpectedly blocked final: %+v", got)
	}

	a.evidence = readinessLedger(mutation, decision)
	if got := a.finalReadinessCheck(); got.reason != "" {
		t.Fatalf("explicit empty decision remained blocked: %+v", got)
	}

	a.evidence = readinessLedger(decision, mutation)
	got := a.finalReadinessCheck()
	if !got.applies || !strings.Contains(got.reason, "deliver_files") || !strings.Contains(got.reason, "after the latest artifact manifest") {
		t.Fatalf("mutation after explicit declaration did not require redelivery: %+v", got)
	}
}

func TestFinalReadinessPreservesVisibleConditionsWithoutArtifactGate(t *testing.T) {
	t.Run("incomplete todo remains", func(t *testing.T) {
		writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"source.go"}}
		todo := evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{{Content: "finish review", Status: "in_progress"}}}
		a := &Agent{evidence: readinessLedger(writer, todo)}
		got := a.finalReadinessCheck()
		if !strings.Contains(got.reason, "incomplete items") {
			t.Fatalf("finalReadinessCheck().reason = %q, want incomplete todo block", got.reason)
		}
	})

	t.Run("project check remains", func(t *testing.T) {
		writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"source.go"}}
		a := &Agent{
			evidence:      readinessLedger(writer),
			projectChecks: []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
		}
		got := a.finalReadinessCheck()
		if !strings.Contains(got.reason, "go test ./...") {
			t.Fatalf("finalReadinessCheck().reason = %q, want project-check block", got.reason)
		}
	})

	t.Run("running persistent task remains", func(t *testing.T) {
		manager := jobs.NewManager(event.Discard)
		defer manager.Close()
		ctx, cancel := context.WithCancel(jobs.WithSession(context.Background(), "readonly-parent"))
		defer cancel()
		job := manager.StartForTurn(ctx, "readonly-parent", "task", "child", func(ctx context.Context, _ io.Writer) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		})
		a := &Agent{evidence: readinessLedger(), jobs: manager}
		if reason := a.runningTaskFinalReadiness(ctx); !strings.Contains(reason, "wait_task") {
			t.Fatalf("runningTaskFinalReadiness() = %q, want persistent-task block", reason)
		}
		cancel()
		_ = manager.WaitForSession(context.Background(), "readonly-parent", []string{job.ID}, 5)
	})
}

func TestNaturalLanguageFileRequestsDoNotBlockFinal(t *testing.T) {
	inputs := []string{
		"请生成一道简短的图表读数练习，轮换覆盖表格、柱状图、折线图、饼图和同比环比。先给出少量明确的模拟数据和问题，再给出答案与解读要点；明确标注数据为示例，不将其描述为真实业务数据。",
		"直接生成 report.pdf，但如果不能创建文件就用聊天说明结果。",
	}
	for _, input := range inputs {
		t.Run(input[:min(16, len(input))], func(t *testing.T) {
			prov := &mockProvider{name: "chat", streams: [][]provider.Chunk{{
				{Type: provider.ChunkText, Text: "这是一次正常的聊天回答。"},
				{Type: provider.ChunkDone},
			}}}
			a := New(prov, tool.NewRegistry(), NewSession("sys"), Options{}, event.Discard)
			if err := a.Run(context.Background(), input); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := len(prov.requests); got != 1 {
				t.Fatalf("provider requests = %d, want one final request", got)
			}
			if got := lastAssistantContent(a.session); strings.Contains(strings.ToLower(got), "readiness") {
				t.Fatalf("final answer unexpectedly mentions readiness: %q", got)
			}
		})
	}
}

func TestFinalReadinessRetryMessageKeepsUserChoicesInteractive(t *testing.T) {
	msg := finalReadinessRetryMessage("latest successful todo_write still has incomplete items: Ask user to review the doc: in_progress")
	lower := strings.ToLower(msg)
	for _, want := range []string{
		"ask tool",
		"wait for its tool result",
		"do not ask in prose",
		"do not claim the user answered",
	} {
		if !strings.Contains(lower, want) {
			t.Fatalf("finalReadinessRetryMessage() missing %q:\n%s", want, msg)
		}
	}
}

func TestFinalReadinessRetryMessageRequiresStandaloneRedelivery(t *testing.T) {
	msg := strings.ToLower(finalReadinessRetryMessage("latest successful todo_write still has incomplete items"))
	for _, want := range []string{
		"retracted from the user-visible record",
		"does not count as the delivery",
		"complete, standalone final answer",
		"outcome, substantive changes, validation, and limitations",
		"do not reply only with receipt/todo bookkeeping",
		"without repeating validation that already succeeded",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("finalReadinessRetryMessage() missing %q:\n%s", want, msg)
		}
	}
}
