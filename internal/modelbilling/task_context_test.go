package modelbilling

import (
	"context"
	"testing"

	"kwu/internal/cloud"
)

func TestTaskContextDeepCopiesAndPropagatesScope(t *testing.T) {
	binding := cloud.BindingScope{ProfileID: "profile_01", CloudUserID: "cloud_01", BindingGeneration: 2}
	input := TaskContextInput{
		TaskID: "task_01", Scope: TaskScope{ActorID: "actor_01", ProfileID: "profile_01", CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model_01", PricingEvidenceID: "evidence_01",
	}
	task, err := NewTaskContext(input)
	if err != nil {
		t.Fatalf("NewTaskContext: %v", err)
	}
	binding.CloudUserID = "changed"
	if got := task.Scope.CloudBinding.CloudUserID; got != "cloud_01" {
		t.Fatalf("task binding = %q, want immutable copy", got)
	}

	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := TaskContextFrom(ctx)
	if !ok || stored.TaskID != task.TaskID || !stored.Scope.Equal(task.Scope) {
		t.Fatalf("TaskContextFrom = %#v, %v", stored, ok)
	}
}

func TestRandomIDGeneratorProducesRFC4122V4UUID(t *testing.T) {
	value, err := NewRandomIDGenerator().NewID()
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if len(value) != 36 || value[14] != '4' || value[19] != '8' && value[19] != '9' && value[19] != 'a' && value[19] != 'b' {
		t.Fatalf("id %q is not an RFC 4122 v4 UUID", value)
	}
}

type fixedIDGenerator struct{ value string }

func (f fixedIDGenerator) NewID() (string, error) { return f.value, nil }

func TestIDGeneratorAcceptsDeterministicFake(t *testing.T) {
	var generator IDGenerator = fixedIDGenerator{value: "task-fixed-01"}
	got, err := generator.NewID()
	if err != nil || got != "task-fixed-01" {
		t.Fatalf("NewID = %q, %v", got, err)
	}
}

func TestWithTaskContextRejectsDifferentExistingValue(t *testing.T) {
	first, _ := NewTaskContext(TaskContextInput{TaskID: "task_01", Scope: TaskScope{ActorID: "actor", ProfileID: "profile"}, BillingSource: BillingNewAPIByok, InitialModelID: "model", PricingEvidenceID: "evidence"})
	ctx, err := WithTaskContext(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WithTaskContext(ctx, first); err != nil {
		t.Fatalf("same context: %v", err)
	}
	for name, change := range map[string]func(*TaskContext){
		"task":  func(v *TaskContext) { v.TaskID = "task_02" },
		"scope": func(v *TaskContext) { v.Scope.ActorID = "other_actor" },
		"billing_source": func(v *TaskContext) {
			v.BillingSource = BillingSubscriptionPool
			v.Scope.CloudBinding = &cloud.BindingScope{ProfileID: v.Scope.ProfileID, CloudUserID: "cloud", BindingGeneration: 1}
		},
		"initial_model": func(v *TaskContext) { v.InitialModelID = "other_model" },
		"evidence":      func(v *TaskContext) { v.PricingEvidenceID = "other_evidence" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := first
			change(&candidate)
			if _, err := WithTaskContext(ctx, candidate); ErrorCodeOf(err) != ErrorIdempotencyConflict {
				t.Fatalf("overwrite error = %v", err)
			}
		})
	}
}

func TestWithNewTaskContextStartsFreshTopLevelLifecycle(t *testing.T) {
	stale, _ := NewTaskContext(TaskContextInput{TaskID: "task_stale", Scope: TaskScope{ActorID: "actor", ProfileID: "profile"}, BillingSource: BillingNewAPIByok, InitialModelID: "model", PricingEvidenceID: "evidence_stale"})
	fresh, _ := NewTaskContext(TaskContextInput{TaskID: "task_fresh", Scope: TaskScope{ActorID: "actor", ProfileID: "profile"}, BillingSource: BillingNewAPIByok, InitialModelID: "model", PricingEvidenceID: "evidence_fresh"})
	ctx, err := WithTaskContext(context.Background(), stale)
	if err != nil {
		t.Fatal(err)
	}

	ctx, err = WithNewTaskContext(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := TaskContextFrom(ctx)
	if !ok || got.TaskID != fresh.TaskID || got.PricingEvidenceID != fresh.PricingEvidenceID {
		t.Fatalf("TaskContextFrom = %+v, %v; want fresh top-level task", got, ok)
	}
	if _, err := WithTaskContext(ctx, stale); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("derived replacement after fresh bind error = %v", err)
	}
}

func TestTaskContextRejectsInvalidInputAndCopiesReadValue(t *testing.T) {
	valid := TaskContextInput{TaskID: "task", Scope: TaskScope{ActorID: "actor", ProfileID: "profile"}, BillingSource: BillingNewAPIByok, InitialModelID: "model", PricingEvidenceID: "evidence"}
	for name, change := range map[string]func(*TaskContextInput){"source": func(v *TaskContextInput) { v.BillingSource = "bad" }, "task": func(v *TaskContextInput) { v.TaskID = "" }, "model": func(v *TaskContextInput) { v.InitialModelID = " " }, "evidence": func(v *TaskContextInput) { v.PricingEvidenceID = "" }, "scope": func(v *TaskContextInput) { v.Scope.ProfileID = "" }} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			change(&candidate)
			if _, err := NewTaskContext(candidate); err == nil {
				t.Fatal("accepted invalid context")
			}
		})
	}
	binding := cloud.BindingScope{ProfileID: "profile", CloudUserID: "cloud", BindingGeneration: 1}
	valid.BillingSource = BillingSubscriptionPool
	valid.Scope.CloudBinding = &binding
	task, err := NewTaskContext(valid)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(nil, task)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := TaskContextFrom(ctx)
	first.Scope.CloudBinding.CloudUserID = "mutated"
	second, _ := TaskContextFrom(ctx)
	if second.Scope.CloudBinding.CloudUserID != "cloud" {
		t.Fatalf("context leaked mutable binding: %+v", second)
	}
}

func TestBillingRouteSelectionRequiresExplicitBillingSource(t *testing.T) {
	_, err := NewBillingRouteSelection(BillingRouteSelection{ModelRef: "provider/model"})
	if ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("missing billing source error = %v", err)
	}
	selection, err := NewBillingRouteSelection(BillingRouteSelection{ModelRef: "provider/model", BillingSource: BillingNewAPIByok})
	if err != nil {
		t.Fatalf("NewBillingRouteSelection: %v", err)
	}
	if selection.ModelRef != "provider/model" || selection.BillingSource != BillingNewAPIByok {
		t.Fatalf("selection = %+v", selection)
	}
}
