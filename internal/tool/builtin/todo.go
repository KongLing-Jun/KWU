package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kwu/internal/evidence"
	"kwu/internal/tool"
)

func init() { tool.RegisterBuiltin(todoWrite{}) }

// todoWrite records the agent's running task list. It has no host side effects —
// the full list lives in the call's args (the model re-sends it whole on every
// update), which a frontend renders as a checklist. Execute just validates the
// shape and acks with a count, so the model gets a stable confirmation. The agent
// keeps one item in_progress at a time and flips each to completed as it finishes.
type todoWrite struct{}

type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm,omitempty"`
	Level      int    `json:"level,omitempty"`
}

func (todoWrite) Name() string { return "todo_write" }

func (todoWrite) Description() string {
	return "Record and update a structured task list for the current work. Send the COMPLETE list every call — it replaces the previous one only when accepted. Keep exactly one item in_progress at a time. When an item is done, successfully call complete_step immediately; the host marks that item completed and advances the list automatically, so use another todo_write only for other list changes. A rejected todo_write applies NONE of its statuses. Skip it for trivial single-step tasks. The list is two-level: a `level` 0 item is a PHASE (a milestone) and the `level` 1 items after it are its concrete sub-steps; omit `level` (0) for a flat list. Each item has `content` (imperative, e.g. \"Add the parser\"), `status` (pending|in_progress|completed), `activeForm` (present-continuous shown while in progress, e.g. \"Adding the parser\"), and optional `level` (0 phase | 1 sub-step)."
}

func (todoWrite) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "todos":{
    "type":"array",
    "description":"The complete task list, in order. Replaces the previous list only when accepted; if rejected, none of its statuses apply. Successfully call complete_step before newly completing an item; the host advances signed-off items automatically.",
    "items":{
      "type":"object",
      "properties":{
        "content":{"type":"string","description":"Imperative description of the task."},
        "status":{"type":"string","enum":["pending","in_progress","completed"],"description":"Task state. Keep at most one in_progress."},
        "activeForm":{"type":"string","description":"Present-continuous form shown while the task is in progress (e.g. \"Running tests\")."},
        "level":{"type":"integer","minimum":0,"maximum":1,"description":"Nesting level: 0 = phase/milestone, 1 = a sub-step of the phase above it. Omit for a flat list."}
      },
      "required":["content","status"]
    }
  }
},
"required":["todos"]
}`)
}

// ReadOnly is true: todo_write only records a list (no filesystem or process
// effect), so it never needs approval and stays available in plan mode — where
// laying out a plan as todos is exactly the point.
func (todoWrite) ReadOnly() bool { return true }

func (todoWrite) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Todos []todoItem `json:"todos"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	var done, active, pending int
	for i, t := range p.Todos {
		if t.Content == "" {
			return "", fmt.Errorf("todo %d: content is required", i+1)
		}
		if t.Level < 0 || t.Level > 1 {
			return "", fmt.Errorf("todo %d: invalid level %d (want 0 phase | 1 sub-step)", i+1, t.Level)
		}
		switch t.Status {
		case "completed":
			done++
		case "in_progress":
			active++
		case "pending", "":
			pending++
		default:
			return "", fmt.Errorf("todo %d: invalid status %q (want pending|in_progress|completed)", i+1, t.Status)
		}
	}
	if err := verifyTodoCompletionTransitions(ctx, p.Todos); err != nil {
		return "", err
	}
	return fmt.Sprintf("Todos updated: %d total — %d completed, %d in progress, %d pending.",
		len(p.Todos), done, active, pending), nil
}

func verifyTodoCompletionTransitions(ctx context.Context, todos []todoItem) error {
	ledger, ok := evidence.FromContext(ctx)
	if !ok {
		return nil
	}
	missing, hasBaseline := ledger.UnverifiedCompletedTodos(toEvidenceTodos(todos))
	if !hasBaseline || len(missing) == 0 {
		return nil
	}
	previous, _ := ledger.LatestTodos()
	return fmt.Errorf("todo_write rejected: this complete update was not applied; latest successful todo state remains %s. Missing successful complete_step receipt(s): %s. For each item you actually finished and attempted to mark completed in this rejected update, successfully call complete_step while it remains unfinished in that unchanged state; do not sign off future work. The host advances signed-off items automatically, so re-send the complete todo list only if other list changes are still needed",
		todoStateSummary(previous, 8), missingTodoSummary(missing, 8))
}

func todoStateSummary(todos []evidence.TodoItem, limit int) string {
	if len(todos) == 0 {
		return "[]"
	}
	parts := make([]string, 0, min(len(todos), limit)+1)
	for i, todo := range todos {
		if i >= limit {
			parts = append(parts, fmt.Sprintf("+%d more", len(todos)-limit))
			break
		}
		status := strings.TrimSpace(todo.Status)
		if status == "" {
			status = "pending"
		}
		parts = append(parts, fmt.Sprintf("%d %q=%s", i+1, boundedTodoContent(todo.Content), status))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func missingTodoSummary(missing []evidence.TodoStepMatch, limit int) string {
	parts := make([]string, 0, min(len(missing), limit)+1)
	for i, todo := range missing {
		if i >= limit {
			parts = append(parts, fmt.Sprintf("+%d more", len(missing)-limit))
			break
		}
		parts = append(parts, fmt.Sprintf("%d %q", todo.Index, boundedTodoContent(todo.Content)))
	}
	return strings.Join(parts, ", ")
}

func boundedTodoContent(content string) string {
	const limit = 80
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	return string(runes[:limit]) + "…"
}

func toEvidenceTodos(todos []todoItem) []evidence.TodoItem {
	out := make([]evidence.TodoItem, 0, len(todos))
	for _, t := range todos {
		out = append(out, evidence.TodoItem{
			Content:    t.Content,
			Status:     t.Status,
			ActiveForm: t.ActiveForm,
			Level:      t.Level,
		})
	}
	return out
}
