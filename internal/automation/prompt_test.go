package automation

import (
	"strings"
	"testing"
	"time"
)

func TestPromptCompilerIncludesTaskContext(t *testing.T) {
	task := Task{
		ID:            "task-1",
		Name:          "Daily report",
		WorkspaceRoot: "D:/project/KWU",
		Prompt: PromptConfig{
			Role:   "Act as a reviewer.",
			Skills: []string{"daily-git-worktree-integration-workflow", "tdd"},
			Prompt: "Summarize the latest run.",
		},
	}
	run := RunManifest{
		RunID:         "run-1",
		TaskID:        "task-1",
		TaskName:      "Daily report",
		Trigger:       TriggerSchedule,
		HostOwner:     "desktop:123",
		WorkspaceRoot: "D:/project/KWU",
	}

	prompt := PromptCompiler{}.Compile(task, run)
	for _, want := range []string{
		"# Automation Task",
		"background",
		"Task name: Daily report",
		"Task id: task-1",
		"Run id: run-1",
		"Trigger: schedule",
		"Host owner: desktop:123",
		"Workspace: D:/project/KWU",
		"Act as a reviewer.",
		"- daily-git-worktree-integration-workflow",
		"- tdd",
		"Summarize the latest run.",
		"result.md",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("compiled prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestPromptCompilerIncludesAssist(t *testing.T) {
	task := Task{
		ID:   "task-assist",
		Name: "Assist run",
		Prompt: PromptConfig{
			Skills: []string{"review"},
			Prompt: "Run the assisted task.",
		},
	}
	run := RunManifest{RunID: "run-assist", TaskID: "task-assist", Trigger: TriggerManual}

	prompt := PromptCompiler{}.CompileWithOptions(task, run, PromptCompileOptions{
		AssistPath: "D:/runs/task-assist/run-assist/assist.md",
		Assist:     "rendered review skill",
	})

	for _, want := range []string{
		"## Automation Assist",
		"Assist path: D:/runs/task-assist/run-assist/assist.md",
		"rendered review skill",
		"Run the assisted task.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("compiled prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestPromptCompilerDoesNotAddSecretsOrPersistentTargets(t *testing.T) {
	prompt := PromptCompiler{}.Compile(Task{Prompt: PromptConfig{Prompt: "do work"}}, RunManifest{RunID: "run-1"})
	for _, forbidden := range []string{"api_key", "token", "secret", "kwu.toml", "cache-stable system prompt"} {
		if strings.Contains(strings.ToLower(prompt), forbidden) {
			t.Fatalf("compiled prompt contains forbidden term %q:\n%s", forbidden, prompt)
		}
	}
}

func TestPromptCompilerIncludesBusinessDiagnosticsSources(t *testing.T) {
	task := Task{
		ID:   "business-1",
		Name: "周度经营诊断",
		Prompt: PromptConfig{
			Prompt: "分析上传的经营数据。",
		},
		BusinessDiagnostics: &BusinessDiagnosticsConfig{
			TargetStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			TargetEnd:   time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
			Region:      "east",
			Recipients:  []string{"ops@example.com"},
			Sources: []BusinessDataSource{
				{ID: "sales", Kind: BusinessDataSales, Type: BusinessSourceCSV, Path: "D:/data/sales.csv"},
				{ID: "purchase", Kind: BusinessDataPurchase, Type: BusinessSourceMCP, Rows: []map[string]any{{"date": "2026-06-02", "amount": 100}}},
			},
		},
	}

	prompt := PromptCompiler{}.Compile(task, RunManifest{RunID: "run-business", Trigger: TriggerManual})
	for _, want := range []string{
		"## Business Diagnostics",
		"Target start: 2026-06-01T00:00:00Z",
		"Target end: 2026-06-08T00:00:00Z",
		"Region: east",
		"Recipients: ops@example.com",
		"- sales (sales/csv): path=D:/data/sales.csv",
		"- purchase (purchase/mcp): inlineRows=1",
		`{"amount":100,"date":"2026-06-02"}`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("compiled prompt missing %q:\n%s", want, prompt)
		}
	}
}
