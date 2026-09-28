package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kwu/internal/tool"
)

const defaultToolTimeout = 120 * time.Second
const defaultDelegatedTaskTimeoutSteps = 12
const maxDelegatedTaskTimeoutSteps = 30

type toolExecutionResult struct {
	output string
	err    error
}

type toolCallParentContextKey struct{}

// toolTimeoutBudgeter is implemented by trusted orchestration tools whose one
// outer call contains several independently bounded agent/tool rounds. It keeps
// the configured timeout intact for ordinary tools while preventing the outer
// task wrapper from cancelling healthy child work before its finite step budget
// can finish.
type toolTimeoutBudgeter interface {
	toolTimeoutBudget(time.Duration, json.RawMessage) time.Duration
}

type toolTimeoutError struct {
	name    string
	timeout time.Duration
}

func (e toolTimeoutError) Error() string {
	return fmt.Sprintf("tool %q timed out after %s", e.name, e.timeout)
}

func normalizedToolTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultToolTimeout
	}
	return timeout
}

func (a *Agent) executeTool(ctx context.Context, name string, t tool.Tool, args json.RawMessage) (string, error) {
	timeout := a.toolTimeout
	if budgeter, ok := t.(tool.TimeoutBudgeter); ok {
		timeout = budgeter.ToolTimeoutBudget(timeout, args)
	} else if budgeter, ok := t.(toolTimeoutBudgeter); ok {
		timeout = budgeter.toolTimeoutBudget(timeout, args)
	}
	return executeToolWithTimeout(ctx, name, timeout, func(callCtx context.Context) (string, error) {
		return t.Execute(callCtx, args)
	})
}

func delegatedTaskTimeoutBudget(base time.Duration, steps int) time.Duration {
	base = normalizedToolTimeout(base)
	if steps <= 0 {
		steps = defaultDelegatedTaskTimeoutSteps
	}
	if steps > maxDelegatedTaskTimeoutSteps {
		steps = maxDelegatedTaskTimeoutSteps
	}
	maxDuration := time.Duration(1<<63 - 1)
	if base > maxDuration/time.Duration(steps) {
		return maxDuration
	}
	return base * time.Duration(steps)
}

func executeToolWithTimeout(parent context.Context, toolName string, timeout time.Duration, execute func(context.Context) (string, error)) (string, error) {
	if timeout == tool.NoTimeoutBudget {
		return executeToolUntilParentDone(parent, execute)
	}
	timeout = normalizedToolTimeout(timeout)
	timeoutErr := toolTimeoutError{name: toolName, timeout: timeout}
	ctx, cancel := context.WithTimeoutCause(parent, timeout, timeoutErr)
	defer cancel()
	ctx = context.WithValue(ctx, toolCallParentContextKey{}, toolCallParentContext(parent))

	done := make(chan toolExecutionResult, 1)
	go func() {
		output, err := execute(ctx)
		done <- toolExecutionResult{output: output, err: err}
	}()

	select {
	case result := <-done:
		return result.output, result.err
	case <-ctx.Done():
		select {
		case result := <-done:
			return result.output, result.err
		default:
			return "", context.Cause(ctx)
		}
	}
}

func executeToolUntilParentDone(parent context.Context, execute func(context.Context) (string, error)) (string, error) {
	ctx := context.WithValue(parent, toolCallParentContextKey{}, toolCallParentContext(parent))
	done := make(chan toolExecutionResult, 1)
	go func() {
		output, err := execute(ctx)
		done <- toolExecutionResult{output: output, err: err}
	}()
	select {
	case result := <-done:
		return result.output, result.err
	case <-parent.Done():
		select {
		case result := <-done:
			return result.output, result.err
		default:
			return "", context.Cause(parent)
		}
	}
}

func toolCallParentContext(ctx context.Context) context.Context {
	if parent, ok := ctx.Value(toolCallParentContextKey{}).(context.Context); ok {
		return parent
	}
	return ctx
}
