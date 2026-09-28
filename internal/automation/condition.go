package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"kwu/internal/config"
)

// ConditionModel evaluates an optional condition_prompt and returns the raw JSON answer.
type ConditionModel interface {
	Decide(context.Context, Task, string) (string, error)
}

// PreflightDisposition decides whether a task may enter the lease and archive
// lifecycle. Deferred tasks have not started and remain eligible for a later tick.
type PreflightDisposition uint8

const (
	PreflightReady PreflightDisposition = iota
	PreflightDefer
)

// PreflightResult is returned by a pure, non-blocking readiness check.
type PreflightResult struct {
	Disposition PreflightDisposition
	Reason      string
}

// Checker evaluates pre-run conditions after schedule due and before runner execution.
type Checker struct {
	ConditionModel     ConditionModel
	ModelResolver      func(Task) error
	TaskResolver       func(Task) (Task, error)
	CapabilityResolver func(Task, time.Time) (CapabilitySnapshot, error)
	Preflight          func(Task) PreflightResult
	RecentRuns         func(taskID string, limit int) ([]RunSummary, error)
}

func (c Checker) Check(ctx context.Context, task Task, due DueResult, now time.Time) ConditionResult {
	now = now.UTC()
	if result, stop := c.checkPreconditions(ctx, task, due, now); stop {
		return result
	}
	if result, stop := c.checkFailureCooldown(task, now); stop {
		return result
	}
	return c.checkConditionPrompt(ctx, task, now)
}

func (c Checker) checkPreconditions(ctx context.Context, task Task, due DueResult, now time.Time) (ConditionResult, bool) {
	if err := ctx.Err(); err != nil {
		return conditionBlocked(now, "condition canceled", err.Error()), true
	}
	if !task.Enabled {
		return conditionSkipped(now, "task disabled"), true
	}
	if !due.Due {
		return conditionSkipped(now, defaultString(due.SkipReason, "schedule not due")), true
	}
	if err := workspaceExists(task.WorkspaceRoot); err != nil {
		return conditionBlocked(now, "workspace unavailable", err.Error()), true
	}
	if err := c.resolveModel(task); err != nil {
		return conditionBlocked(now, "model unavailable", err.Error()), true
	}
	return ConditionResult{}, false
}

func (c Checker) checkFailureCooldown(task Task, now time.Time) (ConditionResult, bool) {
	if blocked, reason, err := c.failureCooldown(task, now); err != nil {
		return conditionBlocked(now, "failure cooldown invalid", err.Error()), true
	} else if blocked {
		return conditionSkipped(now, reason), true
	}
	return ConditionResult{}, false
}

func (c Checker) checkConditionPrompt(ctx context.Context, task Task, now time.Time) ConditionResult {
	prompt := strings.TrimSpace(task.Conditions.ConditionPrompt)
	if prompt == "" {
		return conditionAllowed(now, "conditions passed")
	}
	if c.ConditionModel == nil {
		return conditionBlocked(now, "condition model unavailable", "condition prompt requires a condition model")
	}
	raw, err := c.ConditionModel.Decide(ctx, task, prompt)
	if err != nil {
		return conditionBlocked(now, "condition prompt failed", err.Error())
	}
	return parseConditionPromptResult(now, raw)
}

func (c Checker) resolveModel(task Task) error {
	if c.ModelResolver != nil {
		return c.ModelResolver(task)
	}
	return resolveConfiguredModel(task)
}

func (c Checker) failureCooldown(task Task, now time.Time) (bool, string, error) {
	raw := strings.TrimSpace(task.Conditions.FailureCooldown)
	if raw == "" {
		return false, "", nil
	}
	cooldown, err := time.ParseDuration(raw)
	if err != nil || cooldown <= 0 {
		return false, "", fmt.Errorf("invalid failure cooldown %q", raw)
	}
	if c.RecentRuns == nil {
		return false, "", nil
	}
	runs, err := c.RecentRuns(task.ID, 10)
	if err != nil {
		return false, "", err
	}
	for _, run := range runs {
		if run.Status != RunStatusFailed {
			continue
		}
		at := run.FinishedAt
		if at.IsZero() {
			at = run.StartedAt
		}
		if at.Add(cooldown).After(now) {
			return true, "failure cooldown active", nil
		}
		break
	}
	return false, "", nil
}

func parseConditionPromptResult(now time.Time, raw string) ConditionResult {
	// 条件模型必须返回明确 JSON；解析失败默认阻断，避免误触发有副作用的后台任务。
	var parsed struct {
		ShouldRun *bool  `json:"should_run"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return conditionBlocked(now, "condition prompt invalid JSON", err.Error())
	}
	if parsed.ShouldRun == nil {
		return conditionBlocked(now, "condition prompt missing should_run", "condition prompt JSON must include should_run")
	}
	reason := strings.TrimSpace(parsed.Reason)
	if *parsed.ShouldRun {
		return conditionAllowed(now, defaultString(reason, "condition prompt allowed"))
	}
	return conditionSkipped(now, defaultString(reason, "condition prompt denied"))
}

func resolveConfiguredModel(task Task) error {
	cfg, err := config.LoadForRoot(task.WorkspaceRoot)
	if err != nil {
		return err
	}
	ref := strings.TrimSpace(task.Prompt.Model)
	if ref == "" {
		ref = cfg.DefaultModel
	}
	if _, ok := cfg.ResolveModel(ref); !ok {
		return fmt.Errorf("model %q is not configured", ref)
	}
	return nil
}

func workspaceExists(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}
	return nil
}

func conditionAllowed(now time.Time, reason string) ConditionResult {
	return ConditionResult{ShouldRun: true, Reason: reason, CheckedAt: now}
}

func conditionSkipped(now time.Time, reason string) ConditionResult {
	return ConditionResult{Reason: reason, CheckedAt: now}
}

func conditionBlocked(now time.Time, reason, err string) ConditionResult {
	return ConditionResult{Reason: reason, Error: err, CheckedAt: now}
}
