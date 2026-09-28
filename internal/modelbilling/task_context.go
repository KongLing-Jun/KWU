package modelbilling

import (
	"context"
	"crypto/rand"
	"fmt"
)

// IDGenerator creates opaque client-owned task and logical-call identities.
type IDGenerator interface {
	NewID() (string, error)
}

type randomIDGenerator struct{}

func NewRandomIDGenerator() IDGenerator { return randomIDGenerator{} }

func (randomIDGenerator) NewID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("modelbilling: generate id: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

type TaskContextInput struct {
	TaskID            string
	Scope             TaskScope
	BillingSource     BillingSource
	InitialModelID    string
	PricingEvidenceID string
}

// TaskContext is immutable identity carried by one user task and all of its turns.
type TaskContext struct {
	TaskID            string
	Scope             TaskScope
	BillingSource     BillingSource
	InitialModelID    string
	PricingEvidenceID string
}

func NewTaskContext(input TaskContextInput) (TaskContext, error) {
	ctx := TaskContext{
		TaskID: input.TaskID, Scope: cloneTaskScope(input.Scope), BillingSource: input.BillingSource,
		InitialModelID: input.InitialModelID, PricingEvidenceID: input.PricingEvidenceID,
	}
	for name, value := range map[string]string{
		"task_id": ctx.TaskID, "initial_model_id": ctx.InitialModelID, "pricing_evidence_id": ctx.PricingEvidenceID,
	} {
		if err := requireTrimmed(name, value); err != nil {
			return TaskContext{}, err
		}
	}
	if err := ctx.Scope.Validate(ctx.BillingSource); err != nil {
		return TaskContext{}, err
	}
	return ctx, nil
}

type taskContextKey struct{}

func WithTaskContext(ctx context.Context, task TaskContext) (context.Context, error) {
	validated, err := validatedTaskContext(task)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if existing, ok := TaskContextFrom(ctx); ok {
		if existing.TaskID == task.TaskID && existing.Scope.Equal(task.Scope) && existing.BillingSource == task.BillingSource && existing.InitialModelID == task.InitialModelID && existing.PricingEvidenceID == task.PricingEvidenceID {
			return ctx, nil
		}
		return nil, newError(ErrorIdempotencyConflict, "task context cannot be replaced")
	}
	return context.WithValue(ctx, taskContextKey{}, validated), nil
}

// WithNewTaskContext binds a freshly-created top-level task even when the
// caller context still carries a completed task. Derived calls must use
// WithTaskContext so they cannot replace their parent task accidentally.
func WithNewTaskContext(ctx context.Context, task TaskContext) (context.Context, error) {
	validated, err := validatedTaskContext(task)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, taskContextKey{}, validated), nil
}

func validatedTaskContext(task TaskContext) (TaskContext, error) {
	return NewTaskContext(TaskContextInput{
		TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	})
}

func TaskContextFrom(ctx context.Context) (TaskContext, bool) {
	if ctx == nil {
		return TaskContext{}, false
	}
	task, ok := ctx.Value(taskContextKey{}).(TaskContext)
	if !ok {
		return TaskContext{}, false
	}
	return TaskContext{
		TaskID: task.TaskID, Scope: cloneTaskScope(task.Scope), BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	}, true
}

func cloneTaskScope(scope TaskScope) TaskScope {
	cloned := scope
	if scope.CloudBinding != nil {
		binding := *scope.CloudBinding
		cloned.CloudBinding = &binding
	}
	return cloned
}
