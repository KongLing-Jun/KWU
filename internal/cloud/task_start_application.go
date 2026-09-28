package cloud

import (
	"context"
	"fmt"
	"sync"

	"kwu/internal/userctx"
)

// TaskPriceStatus is the local CLOUD-07 mock gate. It is not a declaration of
// the production cloud pricing protocol.
type TaskPriceStatus string

const (
	TaskPriceFresh         TaskPriceStatus = "fresh"
	TaskPriceStaleUsable   TaskPriceStatus = "stale_usable"
	TaskPriceStaleBlocked  TaskPriceStatus = "stale_blocked"
	TaskPriceRefreshFailed TaskPriceStatus = "refresh_failed"
	TaskPriceUnsettleable  TaskPriceStatus = "unsettleable"
)

// TaskPriceGate carries only the conservative local decision. A refresh
// failure can proceed solely when a cached price remains within its allowed
// maximum age. CLOUD-07 owns the eventual cloud-side rule.
type TaskPriceGate struct {
	Status             TaskPriceStatus
	CachedWithinMaxAge bool
}

func (g TaskPriceGate) allowed() bool {
	switch g.Status {
	case TaskPriceFresh, TaskPriceStaleUsable:
		return true
	case TaskPriceRefreshFailed:
		return g.CachedWithinMaxAge
	default:
		return false
	}
}

// TaskStartInput is the small adapter boundary between the task domain and
// the frozen cloud contracts. ClientEstimatedCost is sent for cloud freeze
// evaluation only; this application never derives or mutates a local ledger.
type TaskStartInput struct {
	Binding             ProfileBinding
	Session             Session
	Price               TaskPriceGate
	TaskID              string
	ModelID             string
	ClientTaskType      string
	ClientEstimatedCost string
	Start               TaskStartCallback
}

// FreezeFunding is retained for later usage/reporting coordination. It is not
// a UI balance or frozen-amount view; CLOUD-10 keeps the ledger split cloud
// authoritative.
type FreezeFunding struct {
	FreeFrozenAmount         string
	SubscriptionFrozenAmount string
}

// TaskStartAuthorization is supplied to the model-task callback only after an
// active freeze has been confirmed for the current binding scope.
type TaskStartAuthorization struct {
	Scope          BindingScope
	TaskID         string
	FreezeID       string
	Funding        FreezeFunding
	LowBalanceMode bool
}

type TaskStartCallback func(context.Context, TaskStartAuthorization) error

// TaskStartResult is intentionally UI-neutral. In particular, it excludes
// the freeze amount while retaining the identifiers and funding source needed
// by later recovery or usage handling.
type TaskStartResult struct {
	Scope          BindingScope
	TaskID         string
	FreezeID       string
	IdempotencyKey string
	Funding        FreezeFunding
	LowBalanceMode bool
	Started        bool
}

// TaskStartApplication creates one freeze per scope/task and starts a model
// only after a matching active result. CLOUD-09 remains limited to this
// process's retained records: it cannot discover work missing from local
// state, reinstall, or another device.
type TaskStartApplication struct {
	credit   CloudCreditClient
	bindings BindingResolver
	mu       sync.Mutex
	records  map[string]*taskStartRecord
}

type taskStartRecord struct {
	ready  chan struct{}
	result TaskStartResult
	err    error
}

func NewTaskStartApplication(credit CloudCreditClient, bindings BindingResolver) *TaskStartApplication {
	return &TaskStartApplication{credit: credit, bindings: bindings, records: make(map[string]*taskStartRecord)}
}

func (a *TaskStartApplication) Start(ctx context.Context, actor userctx.ActorContext, input TaskStartInput) (TaskStartResult, error) {
	scope, err := taskStartScopeFor(ctx, a.bindings, actor, input.Binding, input.Session)
	if err != nil {
		return TaskStartResult{}, err
	}
	if err := taskStartRequired(input); err != nil {
		return TaskStartResult{}, err
	}
	key := scope.key() + "\x00" + input.TaskID
	if existing := a.existing(key); existing != nil {
		<-existing.ready
		return existing.result, existing.err
	}
	if !input.Price.allowed() {
		return TaskStartResult{}, &Error{Code: ErrorModelPricing, Message: "cloud: task price is not settleable"}
	}
	idempotencyKey, err := BuildIdempotencyKey(IdempotencyActionCreateFreeze, scope, input.TaskID)
	if err != nil {
		return TaskStartResult{}, err
	}
	record, existing := a.reserve(key)
	if existing != nil {
		<-existing.ready
		return existing.result, existing.err
	}
	return a.startReserved(ctx, key, record, scope, input, idempotencyKey)
}

func (a *TaskStartApplication) startReserved(ctx context.Context, key string, record *taskStartRecord, scope BindingScope, input TaskStartInput, idempotencyKey string) (TaskStartResult, error) {
	request := FreezeRequest{
		Scope: scope, TaskID: input.TaskID, ModelID: input.ModelID,
		ClientTaskType: input.ClientTaskType, ClientEstimatedCost: input.ClientEstimatedCost,
		IdempotencyKey: idempotencyKey,
	}
	freezeResult, err := a.createFreeze(ctx, request)
	if err != nil {
		a.finish(key, record, TaskStartResult{}, err, true)
		return TaskStartResult{}, err
	}
	if err := freezeResult.Validate(); err != nil {
		a.finish(key, record, TaskStartResult{}, err, true)
		return TaskStartResult{}, err
	}
	if freezeResult.Freeze.Scope != scope || freezeResult.Freeze.TaskID != input.TaskID {
		err := &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: freeze result belongs to another task scope"}
		a.finish(key, record, TaskStartResult{}, err, true)
		return TaskStartResult{}, err
	}

	result := TaskStartResult{
		Scope: scope, TaskID: input.TaskID, FreezeID: freezeResult.Freeze.FreezeID,
		IdempotencyKey: idempotencyKey,
		Funding: FreezeFunding{
			FreeFrozenAmount:         freezeResult.Freeze.FreeFrozenAmount,
			SubscriptionFrozenAmount: freezeResult.Freeze.SubscriptionFrozenAmount,
		},
		LowBalanceMode: freezeResult.Freeze.LowBalanceMode,
	}
	if freezeResult.Freeze.Status != FreezeActive {
		err := fmt.Errorf("cloud: freeze %q is %s, not active", result.FreezeID, freezeResult.Freeze.Status)
		a.finish(key, record, result, err, false)
		return result, err
	}
	return a.startWithActiveFreeze(ctx, key, record, input.Start, result)
}

func (a *TaskStartApplication) createFreeze(ctx context.Context, request FreezeRequest) (FreezeResult, error) {
	if isNilDependency(a.credit) {
		return FreezeResult{}, fmt.Errorf("cloud: task start credit client is not configured")
	}
	return a.credit.CreateFreeze(ctx, request)
}

func (a *TaskStartApplication) startWithActiveFreeze(ctx context.Context, key string, record *taskStartRecord, start TaskStartCallback, result TaskStartResult) (TaskStartResult, error) {
	if err := start(ctx, TaskStartAuthorization{
		Scope: result.Scope, TaskID: result.TaskID, FreezeID: result.FreezeID,
		Funding: result.Funding, LowBalanceMode: result.LowBalanceMode,
	}); err != nil {
		a.finish(key, record, result, err, false)
		return result, err
	}
	result.Started = true
	a.finish(key, record, result, nil, false)
	return result, nil
}

func taskStartScopeFor(ctx context.Context, resolver BindingResolver, actor userctx.ActorContext, binding ProfileBinding, session Session) (BindingScope, error) {
	actor = actor.Normalize()
	if isNilDependency(resolver) {
		return BindingScope{}, fmt.Errorf("cloud: task start binding resolver is not configured")
	}
	current, err := resolver.ResolveBinding(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	return taskStartCurrentScope(actor, current, binding, session)
}

func taskStartCurrentScope(actor userctx.ActorContext, current ProfileBinding, binding ProfileBinding, session Session) (BindingScope, error) {
	if err := current.Validate(); err != nil {
		return BindingScope{}, err
	}
	if err := binding.Validate(); err != nil {
		return BindingScope{}, err
	}
	if err := taskStartBindingMatches(actor, current, binding); err != nil {
		return BindingScope{}, err
	}
	return taskStartSessionScope(current, session)
}

func taskStartBindingMatches(actor userctx.ActorContext, current ProfileBinding, binding ProfileBinding) error {
	if current != binding {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task start binding does not belong to actor"}
	}
	if current.Status != BindingBound || current.ActorID != actor.ActorID || current.ProfileID != actor.ProfileID {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: task start binding does not belong to actor"}
	}
	return nil
}

func taskStartSessionScope(current ProfileBinding, session Session) (BindingScope, error) {
	scope, err := current.Scope()
	if err != nil {
		return BindingScope{}, err
	}
	if err := session.Validate(); err != nil {
		return BindingScope{}, err
	}
	if session.State != SessionAuthenticated || session.Scope != scope {
		return BindingScope{}, &Error{Code: ErrorAuthRequired, Message: "cloud: task start requires authenticated session for binding"}
	}
	return scope, nil
}

func taskStartRequired(input TaskStartInput) error {
	for _, field := range []struct{ name, value string }{
		{"task_id", input.TaskID}, {"model_id", input.ModelID},
		{"client_task_type", input.ClientTaskType}, {"client_estimated_cost", input.ClientEstimatedCost},
	} {
		if err := requireTrimmed(field.name, field.value); err != nil {
			return err
		}
	}
	if input.Start == nil {
		return fmt.Errorf("cloud: task start callback is required")
	}
	return nil
}

func (a *TaskStartApplication) existing(key string) *taskStartRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.records[key]
}

func (a *TaskStartApplication) reserve(key string) (*taskStartRecord, *taskStartRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if existing := a.records[key]; existing != nil {
		return nil, existing
	}
	record := &taskStartRecord{ready: make(chan struct{})}
	a.records[key] = record
	return record, nil
}

func (a *TaskStartApplication) finish(key string, record *taskStartRecord, result TaskStartResult, err error, remove bool) {
	a.mu.Lock()
	record.result, record.err = result, err
	if remove && a.records[key] == record {
		delete(a.records, key)
	}
	close(record.ready)
	a.mu.Unlock()
}
