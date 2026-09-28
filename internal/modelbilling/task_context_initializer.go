package modelbilling

import (
	"context"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/userctx"
)

// BillingRouteResolver resolves only an already explicit user selection. It
// must never infer a billing source from a model, key, plan, or fallback.
type BillingRouteResolver interface {
	ResolveBillingRoute(context.Context, userctx.ActorContext, BillingRouteSelection) (ResolvedBillingRoute, error)
}

// SubscriptionBootstrap is the authenticated subscription snapshot needed to
// create a subscription task. PlanConfigVersion remains unavailable until its
// real adapter exists, so the stored entitlement version is deliberately nil.
type SubscriptionBootstrap struct {
	Scope cloud.BindingScope
	Plan  string
}

// SubscriptionBootstrapResolver is only called for subscription_pool routes.
type SubscriptionBootstrapResolver interface {
	ResolveSubscriptionBootstrap(context.Context, userctx.ActorContext, ResolvedBillingRoute) (SubscriptionBootstrap, error)
}

// TaskContextInitializer performs the write-ahead task start protocol. The
// BYOK route never touches cloud dependencies; the subscription route uses the
// injected MOCK_ONLY credit/freeze adapters in B2B desktop composition.
type TaskContextInitializer struct {
	Routes        BillingRouteResolver
	Subscriptions SubscriptionBootstrapResolver
	Tasks         TaskStore
	CreditConfig  CreditConfigProvider
	Freeze        cloud.CloudCreditClient
	TaskRecovery  cloud.TaskRecoveryStore
	IDs           IDGenerator
	Now           func() time.Time
}

func (i TaskContextInitializer) InitializeTaskContext(ctx context.Context, actor userctx.ActorContext, selection BillingRouteSelection) (TaskContext, error) {
	if i.Routes == nil || i.Tasks == nil || i.IDs == nil {
		return TaskContext{}, newError(ErrorContractInvalid, "task context initializer dependencies are required")
	}
	selection, err := NewBillingRouteSelection(selection)
	if err != nil {
		return TaskContext{}, err
	}
	actor = actor.Normalize()
	route, err := i.Routes.ResolveBillingRoute(ctx, actor, selection)
	if err != nil {
		return TaskContext{}, err
	}
	if err := route.Validate(); err != nil {
		return TaskContext{}, err
	}
	if route.ModelRef != selection.ModelRef || route.BillingSource != selection.BillingSource {
		return TaskContext{}, newError(ErrorBillingSourceMismatch, "resolved route does not match explicit selection")
	}
	if route.BillingSource == BillingSubscriptionPool {
		return i.initializeSubscription(ctx, actor, route)
	}
	return i.initializeBYOK(ctx, actor, route)
}

func (i TaskContextInitializer) initializeBYOK(ctx context.Context, actor userctx.ActorContext, route ResolvedBillingRoute) (TaskContext, error) {
	taskID, evidenceID, err := i.identities()
	if err != nil {
		return TaskContext{}, err
	}
	scope := TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID}
	now := i.now()
	task := ModelTask{TaskID: taskID, Scope: scope, BillingSource: BillingNewAPIByok, InitialModelID: route.ModelID, ClientTaskType: "chat", ClientEstimatedCost: "0.0000", PricingEvidenceID: evidenceID, State: TaskCreated, CreatedAt: now, UpdatedAt: now}
	// The local profile provider is authoritative for dispatch but has no B1
	// pricing group contract yet. Persist that fact rather than fabricate one.
	evidence := PricingEvidence{EvidenceID: evidenceID, SnapshotID: "pricing-unavailable-v2:newapi_byok:group_unavailable", Scope: scope, TaskID: taskID, ModelID: route.ModelID, PricingGroup: "__kwu_unavailable__", State: PricingUnavailable, CapturedAt: now}
	if err := i.Tasks.CreateTaskWithEvidence(ctx, task, evidence); err != nil {
		return TaskContext{}, err
	}
	emitTaskLifecycle(ctx, taskID, "", TaskCreated)
	if err := i.Tasks.UpdateState(ctx, TaskStateUpdate{Scope: scope, TaskID: taskID, From: TaskCreated, To: TaskReady, At: now}); err != nil {
		return TaskContext{}, err
	}
	emitTaskLifecycle(ctx, taskID, TaskCreated, TaskReady)
	return NewTaskContext(TaskContextInput{TaskID: taskID, Scope: scope, BillingSource: BillingNewAPIByok, InitialModelID: route.ModelID, PricingEvidenceID: evidenceID})
}

func (i TaskContextInitializer) initializeSubscription(ctx context.Context, actor userctx.ActorContext, route ResolvedBillingRoute) (TaskContext, error) {
	bootstrap, creditConfig, err := i.subscriptionPrerequisites(ctx, actor, route)
	if err != nil {
		return TaskContext{}, err
	}
	taskID, evidenceID, err := i.identities()
	if err != nil {
		return TaskContext{}, err
	}
	now := i.now()
	scope := TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &bootstrap.Scope}
	task := ModelTask{TaskID: taskID, Scope: scope, BillingSource: BillingSubscriptionPool, InitialModelID: route.ModelID, ClientTaskType: "chat", ClientEstimatedCost: "0.0000", PricingEvidenceID: evidenceID, State: TaskCreated, CreatedAt: now, UpdatedAt: now}
	evidence := PricingEvidence{EvidenceID: evidenceID, SnapshotID: "credit-config-" + taskID, Scope: scope, TaskID: taskID, ModelID: route.ModelID, PricingGroup: creditConfig.PricingGroup, PricingVersion: cloneOptionalString(creditConfig.Version), State: PricingFresh, CapturedAt: now}
	if err := i.Tasks.CreateTaskWithEvidence(ctx, task, evidence); err != nil {
		return TaskContext{}, err
	}
	emitTaskLifecycle(ctx, taskID, "", TaskCreated)
	taskContext, err := NewTaskContext(TaskContextInput{TaskID: taskID, Scope: scope, BillingSource: BillingSubscriptionPool, InitialModelID: route.ModelID, PricingEvidenceID: evidenceID})
	if err != nil {
		return TaskContext{}, err
	}
	if err := i.recordSubscriptionFreeze(ctx, bootstrap, taskContext, route.ModelID, now); err != nil {
		return TaskContext{}, err
	}
	return taskContext, nil
}

func (i TaskContextInitializer) subscriptionPrerequisites(ctx context.Context, actor userctx.ActorContext, route ResolvedBillingRoute) (SubscriptionBootstrap, CreditConfig, error) {
	if i.Subscriptions == nil || i.CreditConfig == nil || i.Freeze == nil {
		return SubscriptionBootstrap{}, CreditConfig{}, newError(ErrorContractInvalid, "subscription task initializer dependencies are required")
	}
	bootstrap, err := i.Subscriptions.ResolveSubscriptionBootstrap(ctx, actor, route)
	if err != nil {
		return SubscriptionBootstrap{}, CreditConfig{}, err
	}
	if err := validateSubscriptionBootstrap(actor, bootstrap); err != nil {
		return SubscriptionBootstrap{}, CreditConfig{}, err
	}
	creditConfig, err := i.CreditConfig.GetCreditConfig(ctx, bootstrap.Scope)
	if err != nil {
		return SubscriptionBootstrap{}, CreditConfig{}, err
	}
	if err := creditConfig.Validate(); err != nil {
		return SubscriptionBootstrap{}, CreditConfig{}, err
	}
	return bootstrap, creditConfig, nil
}

func validateSubscriptionBootstrap(actor userctx.ActorContext, bootstrap SubscriptionBootstrap) error {
	if err := bootstrap.Scope.Validate(); err != nil {
		return newError(ErrorScopeMismatch, err.Error())
	}
	if bootstrap.Scope.ProfileID != actor.ProfileID {
		return newError(ErrorScopeMismatch, "subscription binding profile does not match actor")
	}
	if bootstrap.Plan == "" {
		return newError(ErrorContractInvalid, "subscription plan is required")
	}
	return nil
}

func (i TaskContextInitializer) recordSubscriptionFreeze(ctx context.Context, bootstrap SubscriptionBootstrap, task TaskContext, modelID string, now time.Time) error {
	key, err := cloud.BuildIdempotencyKey(cloud.IdempotencyActionCreateFreeze, bootstrap.Scope, task.TaskID)
	if err != nil {
		return err
	}
	request := &cloud.FreezeRequest{Scope: bootstrap.Scope, TaskID: task.TaskID, ModelID: modelID, ClientTaskType: "chat", ClientEstimatedCost: "0.0000", IdempotencyKey: key}
	if err := i.Tasks.RecordFreezeRequest(ctx, task, request, now); err != nil {
		return err
	}
	emitTaskLifecycle(ctx, task.TaskID, TaskCreated, TaskFreezePending)
	result, err := i.Freeze.CreateFreeze(ctx, *request)
	if err != nil {
		return err
	}
	if err := result.Validate(); err != nil {
		return err
	}
	if result.Freeze.Scope != bootstrap.Scope || result.Freeze.TaskID != task.TaskID || result.Freeze.Status != cloud.FreezeActive {
		return newError(ErrorContractInvalid, "subscription freeze is not an active task freeze")
	}
	if err := i.saveTaskRecoveryRecord(ctx, bootstrap.Scope, task.TaskID, result.Freeze.FreezeID, key); err != nil {
		return err
	}
	entitlement := &TaskEntitlementSnapshot{Plan: bootstrap.Plan, CapturedAt: now}
	if err := i.Tasks.RecordFreezeResult(ctx, task, &result.Freeze, entitlement, now); err != nil {
		return err
	}
	emitTaskLifecycle(ctx, task.TaskID, TaskFreezePending, TaskReady)
	return nil
}

func (i TaskContextInitializer) saveTaskRecoveryRecord(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID, freezeID, idempotencyKey string,
) error {
	if i.TaskRecovery == nil {
		return nil
	}
	return i.TaskRecovery.Save(ctx, cloud.TaskRecoveryRecord{
		Scope: scope, TaskID: taskID, FreezeID: freezeID, IdempotencyKey: idempotencyKey,
	})
}

func (i TaskContextInitializer) identities() (string, string, error) {
	taskID, err := i.IDs.NewID()
	if err != nil {
		return "", "", err
	}
	evidenceID, err := i.IDs.NewID()
	if err != nil {
		return "", "", err
	}
	return taskID, evidenceID, nil
}

func (i TaskContextInitializer) now() time.Time {
	if i.Now != nil {
		return i.Now().UTC()
	}
	return time.Now().UTC()
}
