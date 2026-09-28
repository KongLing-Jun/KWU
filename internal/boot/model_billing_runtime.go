package boot

import (
	"context"
	"fmt"

	"kwu/internal/agent"
	"kwu/internal/modelbilling"
	"kwu/internal/provider"
)

type bootModelBilling struct {
	dispatcher agent.StreamDispatcher
	calls      *modelbilling.MainExecutorDispatcher
	tasks      modelbilling.TaskStore
	cleanup    func()
	handedOff  bool
}

// bootRoleBoundProvider adapts provider-only derived components to the single
// model-billing dispatcher. The only call-owned facts stored here are the role
// and actual model ID; task, scope, billing source, freeze, and evidence remain
// dispatcher-owned facts derived from the current TaskContext.
type bootRoleBoundProvider struct {
	base       provider.Provider
	dispatcher *modelbilling.MainExecutorDispatcher
	spec       modelbilling.CallDispatchSpec
}

func (p *bootRoleBoundProvider) Name() string { return p.base.Name() }

func (p *bootRoleBoundProvider) Stream(ctx context.Context, request provider.Request) (<-chan provider.Chunk, error) {
	return p.dispatcher.StreamCall(ctx, p.base, request, p.spec)
}

func usesSubscriptionBillingRoute(opts Options) bool {
	return opts.RequireTaskContext && opts.BillingRouteSelection.BillingSource == modelbilling.BillingSubscriptionPool
}

func newBootModelBilling(opts Options) (bootModelBilling, error) {
	if opts.MainStreamDispatcher == nil {
		return bootModelBilling{}, nil
	}
	if opts.ModelBillingTaskStore == nil {
		return bootModelBilling{}, fmt.Errorf("boot: task store is required when main stream dispatcher is enabled")
	}
	calls, ok := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	if !ok {
		return bootModelBilling{}, fmt.Errorf("boot: model billing dispatcher must support role-bound calls")
	}
	return bootModelBilling{dispatcher: opts.MainStreamDispatcher, calls: calls, tasks: opts.ModelBillingTaskStore, cleanup: opts.ModelBillingCleanup}, nil
}

func (b bootModelBilling) roleBoundProvider(base provider.Provider, role modelbilling.CallRole, modelID string) (provider.Provider, error) {
	if b.calls == nil {
		return base, nil
	}
	spec := modelbilling.CallDispatchSpec{Role: role, ModelID: modelID}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &bootRoleBoundProvider{base: base, dispatcher: b.calls, spec: spec}, nil
}

func (b bootModelBilling) roleBoundStreamDispatcher(role modelbilling.CallRole, modelID string) (agent.StreamDispatcher, error) {
	if b.calls == nil {
		return nil, nil
	}
	return modelbilling.NewRoleBoundStreamDispatcher(b.calls, modelbilling.CallDispatchSpec{Role: role, ModelID: modelID})
}

func (b bootModelBilling) chain(cleanup func()) func() {
	if b.cleanup == nil {
		return cleanup
	}
	return func() {
		if cleanup != nil {
			cleanup()
		}
		b.cleanup()
	}
}

func (b *bootModelBilling) closeOnBuildError() {
	if !b.handedOff && b.cleanup != nil {
		b.cleanup()
	}
}
