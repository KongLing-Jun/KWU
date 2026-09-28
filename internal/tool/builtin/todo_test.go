package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"kwu/internal/evidence"
)

func TestTodoWriteAcceptsLevels(t *testing.T) {
	args := json.RawMessage(`{"todos":[` +
		`{"content":"Phase","status":"in_progress","level":0},` +
		`{"content":"sub","status":"pending","level":1}]}`)
	if _, err := (todoWrite{}).Execute(context.Background(), args); err != nil {
		t.Fatalf("levels 0/1 should be accepted: %v", err)
	}
}

func TestTodoWriteSchemaUsesPortableIntegerBounds(t *testing.T) {
	var schema struct {
		Properties map[string]struct {
			Items struct {
				Properties map[string]struct {
					Type    string   `json:"type"`
					Enum    []any    `json:"enum"`
					Minimum *float64 `json:"minimum"`
					Maximum *float64 `json:"maximum"`
				} `json:"properties"`
			} `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal((todoWrite{}).Schema(), &schema); err != nil {
		t.Fatalf("decode todo schema: %v", err)
	}

	level := schema.Properties["todos"].Items.Properties["level"]
	if level.Type != "integer" {
		t.Fatalf("level type = %q, want integer", level.Type)
	}
	if len(level.Enum) != 0 {
		t.Fatalf("level enum = %v, numeric enums are not portable across tool APIs", level.Enum)
	}
	if level.Minimum == nil || *level.Minimum != 0 {
		t.Fatalf("level minimum = %v, want 0", level.Minimum)
	}
	if level.Maximum == nil || *level.Maximum != 1 {
		t.Fatalf("level maximum = %v, want 1", level.Maximum)
	}
}

func TestTodoWriteRejectsBadLevel(t *testing.T) {
	args := json.RawMessage(`{"todos":[{"content":"x","status":"pending","level":2}]}`)
	_, err := (todoWrite{}).Execute(context.Background(), args)
	if err == nil || !strings.Contains(err.Error(), "level") {
		t.Fatalf("level 2 should be rejected with a level error, got %v", err)
	}
}

func TestTodoWriteRejectsNewCompletedWithoutCompleteStepReceipt(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Add parser", Status: "in_progress"}},
	})
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Add parser","status":"completed"}]}`)

	_, err := (todoWrite{}).Execute(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "complete_step") {
		t.Fatalf("new completion without complete_step should be rejected, got %v", err)
	}
	for _, want := range []string{"complete update was not applied", `1 "Add parser"=in_progress`, `1 "Add parser"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("atomic rejection %q missing %q", err, want)
		}
	}
}

func TestTodoWriteAtomicRejectionListsEveryMissingSignoff(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{
		{Content: "S1", Status: "in_progress"},
		{Content: "S2", Status: "pending"},
		{Content: "S3", Status: "pending"},
	}})
	ctx := evidence.WithLedger(context.Background(), ledger)
	_, err := (todoWrite{}).Execute(ctx, json.RawMessage(`{"todos":[
		{"content":"S1","status":"completed"},
		{"content":"S2","status":"completed"},
		{"content":"S3","status":"in_progress"}]}`))
	if err == nil {
		t.Fatal("unsigned batch completion should be rejected")
	}
	for _, want := range []string{`1 "S1"`, `2 "S2"`, `3 "S3"=pending`, "actually finished and attempted to mark completed", "do not sign off future work", "host advances signed-off items automatically"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("batch rejection %q missing %q", err, want)
		}
	}
}

func TestTodoWriteAtomicRejectionBoundsUnicodeContent(t *testing.T) {
	long := strings.Repeat("界", 100) + "UNBOUNDED_TAIL"
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{ToolName: "todo_write", Success: true, Todos: []evidence.TodoItem{{Content: long, Status: "in_progress"}}})
	ctx := evidence.WithLedger(context.Background(), ledger)
	args, err := json.Marshal(map[string]any{"todos": []todoItem{{Content: long, Status: "completed"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = (todoWrite{}).Execute(ctx, args)
	if err == nil {
		t.Fatal("unsigned long todo completion should be rejected")
	}
	message := err.Error()
	if !utf8.ValidString(message) || !strings.Contains(message, "…") || strings.Contains(message, "UNBOUNDED_TAIL") {
		t.Fatalf("bounded rejection is not rune-safe or retained the tail: %q", message)
	}
	if len([]rune(message)) > 700 {
		t.Fatalf("bounded rejection unexpectedly long: %d runes", len([]rune(message)))
	}
}

func TestTodoWriteAcceptsNewCompletedWithCompleteStepReceipt(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Add parser", Status: "in_progress"}},
	})
	ledger.Record(evidence.Receipt{ToolName: "complete_step", Success: true, Step: "Add parser"})
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Add parser","status":"completed"}]}`)

	if _, err := (todoWrite{}).Execute(ctx, args); err != nil {
		t.Fatalf("matching complete_step should authorize new completion: %v", err)
	}
}

func TestTodoWriteAllowsInitialCompletedWithoutBaseline(t *testing.T) {
	ctx := evidence.WithLedger(context.Background(), evidence.NewLedger())
	args := json.RawMessage(`{"todos":[{"content":"Add parser","status":"completed"}]}`)

	if _, err := (todoWrite{}).Execute(ctx, args); err != nil {
		t.Fatalf("initial completed todo without baseline should preserve existing behavior: %v", err)
	}
}

func TestTodoWriteRejectsFailedCompleteStepReceipt(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Add parser", Status: "in_progress"}},
	})
	ledger.Record(evidence.Receipt{ToolName: "complete_step", Success: false, Step: "Add parser"})
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Add parser","status":"completed"}]}`)

	_, err := (todoWrite{}).Execute(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "complete_step") {
		t.Fatalf("failed complete_step without proof-bearing recovery should not authorize completion, got %v", err)
	}
}

func TestTodoWriteRejectsFailedCompleteStepWithoutProof(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Run project script", Status: "in_progress"}},
	})
	ledger.Record(evidence.Receipt{
		ToolName: "bash",
		Success:  true,
		Command:  `python "script.py"`,
	})
	ledger.Record(evidence.ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Run project script",
		"result":"script ran",
		"evidence":[]
	}`), false, true))
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Run project script","status":"completed"}]}`)

	_, err := (todoWrite{}).Execute(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "complete_step") {
		t.Fatalf("failed complete_step without proof should not authorize completion, got %v", err)
	}
}

func TestTodoWriteRejectsFailedCompleteStepMissingResult(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Run project script", Status: "in_progress"}},
	})
	ledger.Record(evidence.Receipt{
		ToolName: "bash",
		Success:  true,
		Command:  `python "script.py"`,
	})
	ledger.Record(evidence.ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Run project script",
		"evidence":[{"kind":"manual","summary":"checked manually"}]
	}`), false, true))
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Run project script","status":"completed"}]}`)

	_, err := (todoWrite{}).Execute(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "complete_step") {
		t.Fatalf("failed complete_step without result should not authorize completion, got %v", err)
	}
}

func TestTodoWriteRecoversAfterFailedCompleteStepWithProgressReceipt(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Run project script", Status: "in_progress"}},
	})
	ledger.Record(evidence.Receipt{
		ToolName: "bash",
		Success:  true,
		Command:  `python "script.py"`,
	})
	ledger.Record(evidence.ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Run project script",
		"result":"script ran",
		"evidence":[{"kind":"verification","summary":"script completed","command":"python script.py"}]
	}`), false, true))
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Run project script","status":"completed"}]}`)

	if _, err := (todoWrite{}).Execute(ctx, args); err != nil {
		t.Fatalf("matching failed complete_step with progress receipt should recover todo completion: %v", err)
	}
}

func TestTodoWriteRejectsRecoveryWhenProgressIsAfterFailedCompleteStep(t *testing.T) {
	ledger := evidence.NewLedger()
	ledger.Record(evidence.Receipt{
		ToolName: "todo_write",
		Success:  true,
		Todos:    []evidence.TodoItem{{Content: "Run project script", Status: "in_progress"}},
	})
	ledger.Record(evidence.ReceiptFromToolCall("complete_step", json.RawMessage(`{
		"step":"Run project script",
		"result":"script ran",
		"evidence":[{"kind":"verification","summary":"script completed","command":"python other.py"}]
	}`), false, true))
	ledger.Record(evidence.Receipt{
		ToolName: "write_file",
		Success:  true,
		Paths:    []string{"docs/notes.md"},
		Write:    true,
	})
	ctx := evidence.WithLedger(context.Background(), ledger)
	args := json.RawMessage(`{"todos":[{"content":"Run project script","status":"completed"}]}`)

	_, err := (todoWrite{}).Execute(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "complete_step") {
		t.Fatalf("progress after a failed complete_step should not authorize recovery, got %v", err)
	}
}
