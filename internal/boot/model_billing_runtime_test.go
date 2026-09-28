package boot

import (
	"context"
	"testing"

	"kwu/internal/modelbilling"
	"kwu/internal/provider"
)

type billingRuntimeProvider struct{}

func (billingRuntimeProvider) Name() string { return "runtime-base" }

func (billingRuntimeProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk)
	close(ch)
	return ch, nil
}

func TestBootModelBillingBindsOnlyRoleAndModel(t *testing.T) {
	dispatcher := &modelbilling.MainExecutorDispatcher{}
	billing, err := newBootModelBilling(Options{ModelBillingOptions: ModelBillingOptions{
		MainStreamDispatcher:  dispatcher,
		ModelBillingTaskStore: new(modelbilling.SQLiteTaskStore),
	}})
	if err != nil {
		t.Fatal(err)
	}

	bound, err := billing.roleBoundProvider(billingRuntimeProvider{}, modelbilling.CallRolePlanner, "model_planner")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := bound.(*bootRoleBoundProvider)
	if !ok {
		t.Fatalf("provider = %T, want *bootRoleBoundProvider", bound)
	}
	if got.base.Name() != "runtime-base" {
		t.Fatalf("base = %q, want runtime-base", got.base.Name())
	}
	if got.spec != (modelbilling.CallDispatchSpec{Role: modelbilling.CallRolePlanner, ModelID: "model_planner"}) {
		t.Fatalf("spec = %#v", got.spec)
	}
}

func TestBootModelBillingLegacyRoleBindingReturnsBase(t *testing.T) {
	billing, err := newBootModelBilling(Options{})
	if err != nil {
		t.Fatal(err)
	}
	base := billingRuntimeProvider{}
	bound, err := billing.roleBoundProvider(base, modelbilling.CallRolePlanner, "model_planner")
	if err != nil {
		t.Fatal(err)
	}
	if bound != base {
		t.Fatalf("legacy provider = %T, want original base", bound)
	}
}
