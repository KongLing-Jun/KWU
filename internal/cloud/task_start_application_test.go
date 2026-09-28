package cloud

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/userctx"
)

func TestTaskStartApplicationStartsOnceAfterActiveFreeze(t *testing.T) {
	t.Parallel()
	scope := taskStartScope(t)
	credit := &taskStartCreditClient{result: taskStartFreezeResult(scope, FreezeActive, false)}
	app := taskStartApplication(t, scope, credit)
	starts := 0
	input := taskStartInput(scope, func(context.Context, TaskStartAuthorization) error {
		starts++
		return nil
	})

	first, err := app.Start(context.Background(), taskStartActor(), input)
	if err != nil {
		t.Fatalf("Start first: %v", err)
	}
	second, err := app.Start(context.Background(), taskStartActor(), input)
	if err != nil {
		t.Fatalf("Start retry: %v", err)
	}
	if credit.calls != 1 || starts != 1 {
		t.Fatalf("calls = freeze %d, start %d; want 1, 1", credit.calls, starts)
	}
	if first != second || first.FreezeID != "freeze-1" || first.LowBalanceMode {
		t.Fatalf("result = %#v, %#v", first, second)
	}
	key, err := BuildIdempotencyKey(IdempotencyActionCreateFreeze, scope, "task-1")
	if err != nil || first.IdempotencyKey != key || credit.request.IdempotencyKey != key {
		t.Fatalf("idempotency keys = result %q, request %q, error %v; want %q", first.IdempotencyKey, credit.request.IdempotencyKey, err, key)
	}
}

func TestTaskStartApplicationReturnsLowBalanceFundingWithoutFreezeAmount(t *testing.T) {
	t.Parallel()
	scope := taskStartScope(t)
	credit := &taskStartCreditClient{result: taskStartFreezeResult(scope, FreezeActive, true)}
	result, err := taskStartApplication(t, scope, credit).Start(context.Background(), taskStartActor(), taskStartInput(scope, func(context.Context, TaskStartAuthorization) error { return nil }))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !result.LowBalanceMode || result.Funding.FreeFrozenAmount != "3" || result.Funding.SubscriptionFrozenAmount != "2" {
		t.Fatalf("result funding = %#v", result)
	}
}

func TestTaskStartApplicationDoesNotStartWhenFreezeIsRejected(t *testing.T) {
	t.Parallel()
	scope := taskStartScope(t)
	cases := []struct {
		name   string
		credit *taskStartCreditClient
		input  func(TaskStartInput) TaskStartInput
	}{
		{name: "insufficient balance", credit: &taskStartCreditClient{err: &Error{Code: ErrorInsufficientBalance}}},
		{name: "network failure", credit: &taskStartCreditClient{err: &Error{Code: ErrorNetworkUnavailable, Retryable: true}}},
		{name: "non active", credit: &taskStartCreditClient{result: taskStartFreezeResult(scope, FreezeCreated, false)}},
		{name: "price blocked", credit: &taskStartCreditClient{result: taskStartFreezeResult(scope, FreezeActive, false)}, input: func(in TaskStartInput) TaskStartInput {
			in.Price = TaskPriceGate{Status: TaskPriceStaleBlocked}
			return in
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			starts := 0
			input := taskStartInput(scope, func(context.Context, TaskStartAuthorization) error { starts++; return nil })
			if tc.input != nil {
				input = tc.input(input)
			}
			_, err := taskStartApplication(t, scope, tc.credit).Start(context.Background(), taskStartActor(), input)
			if err == nil {
				t.Fatal("Start error = nil")
			}
			if starts != 0 {
				t.Fatalf("start callback calls = %d, want 0", starts)
			}
			if tc.name == "price blocked" && tc.credit.calls != 0 {
				t.Fatalf("freeze calls = %d, want 0", tc.credit.calls)
			}
		})
	}
}

func TestTaskStartApplicationAllowsUsablePricesOnly(t *testing.T) {
	t.Parallel()
	scope := taskStartScope(t)
	cases := []struct {
		name  string
		gate  TaskPriceGate
		allow bool
	}{
		{name: "fresh", gate: TaskPriceGate{Status: TaskPriceFresh}, allow: true},
		{name: "stale usable", gate: TaskPriceGate{Status: TaskPriceStaleUsable}, allow: true},
		{name: "refresh failure within cache limit", gate: TaskPriceGate{Status: TaskPriceRefreshFailed, CachedWithinMaxAge: true}, allow: true},
		{name: "refresh failure outside cache limit", gate: TaskPriceGate{Status: TaskPriceRefreshFailed}, allow: false},
		{name: "dynamic not settleable", gate: TaskPriceGate{Status: TaskPriceUnsettleable}, allow: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			credit := &taskStartCreditClient{result: taskStartFreezeResult(scope, FreezeActive, false)}
			input := taskStartInput(scope, func(context.Context, TaskStartAuthorization) error { return nil })
			input.Price = tc.gate
			_, err := taskStartApplication(t, scope, credit).Start(context.Background(), taskStartActor(), input)
			if (err == nil) != tc.allow {
				t.Fatalf("Start error = %v, allow = %v", err, tc.allow)
			}
		})
	}
}

func TestTaskStartApplicationRejectsBindingAndFreezeScopeMismatch(t *testing.T) {
	t.Parallel()
	scope := taskStartScope(t)
	for _, tc := range []struct {
		name   string
		input  func(TaskStartInput) TaskStartInput
		freeze BindingScope
	}{
		{name: "binding actor", input: func(in TaskStartInput) TaskStartInput { in.Binding.ActorID = "another"; return in }, freeze: scope},
		{name: "binding profile", input: func(in TaskStartInput) TaskStartInput { in.Binding.ProfileID = "another"; return in }, freeze: scope},
		{name: "session scope", input: func(in TaskStartInput) TaskStartInput { in.Session.Scope.BindingGeneration++; return in }, freeze: scope},
		{name: "freeze scope", freeze: BindingScope{ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration + 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credit := &taskStartCreditClient{result: taskStartFreezeResult(tc.freeze, FreezeActive, false)}
			starts := 0
			input := taskStartInput(scope, func(context.Context, TaskStartAuthorization) error { starts++; return nil })
			if tc.input != nil {
				input = tc.input(input)
			}
			_, err := taskStartApplication(t, scope, credit).Start(context.Background(), taskStartActor(), input)
			if err == nil || starts != 0 {
				t.Fatalf("Start error = %v, callback calls = %d", err, starts)
			}
		})
	}
}

func TestTaskStartApplicationRejectsOldGenerationAfterRebind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	actor := taskStartActor()
	bindings := NewBindingRepository()
	oldBinding, err := bindings.ConfirmBinding(ctx, actor, "cloud-1")
	if err != nil {
		t.Fatalf("ConfirmBinding: %v", err)
	}
	if _, err := bindings.Rebind(ctx, actor, "cloud-2"); err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	oldScope, err := oldBinding.Scope()
	if err != nil {
		t.Fatalf("old Binding.Scope: %v", err)
	}
	credit := &taskStartCreditClient{result: taskStartFreezeResult(oldScope, FreezeActive, false)}
	starts := 0
	input := taskStartInput(oldScope, func(context.Context, TaskStartAuthorization) error { starts++; return nil })
	input.Binding = oldBinding
	app := NewTaskStartApplication(credit, bindings)

	_, err = app.Start(ctx, actor, input)
	if err == nil {
		t.Fatal("Start error = nil, want old generation rejection")
	}
	assertProfileScopeMismatch(t, err)
	if credit.calls != 0 || starts != 0 {
		t.Fatalf("calls = freeze %d, start %d; want 0, 0", credit.calls, starts)
	}
}

func TestTaskStartApplicationRetainsFailedOrCanceledStartWithoutRetrying(t *testing.T) {
	t.Parallel()
	scope := taskStartScope(t)
	for _, want := range []error{errors.New("model failed"), context.Canceled} {
		t.Run(want.Error(), func(t *testing.T) {
			credit := &taskStartCreditClient{result: taskStartFreezeResult(scope, FreezeActive, false)}
			app := taskStartApplication(t, scope, credit)
			starts := 0
			input := taskStartInput(scope, func(context.Context, TaskStartAuthorization) error { starts++; return want })
			first, err := app.Start(context.Background(), taskStartActor(), input)
			if !errors.Is(err, want) || first.FreezeID != "freeze-1" {
				t.Fatalf("first = %#v, %v", first, err)
			}
			second, err := app.Start(context.Background(), taskStartActor(), input)
			if !errors.Is(err, want) || second != first || credit.calls != 1 || starts != 1 {
				t.Fatalf("retry = %#v, %v; freeze %d, starts %d", second, err, credit.calls, starts)
			}
		})
	}
}

func taskStartActor() userctx.ActorContext {
	return userctx.ActorContext{ActorID: "actor-1", ProfileID: "profile-1"}
}

func taskStartScope(t *testing.T) BindingScope {
	t.Helper()
	scope, err := NewBindingScope(taskStartActor(), "cloud-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func taskStartInput(scope BindingScope, start TaskStartCallback) TaskStartInput {
	return TaskStartInput{
		Binding: ProfileBinding{ActorID: "actor-1", ProfileID: "profile-1", CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration, Status: BindingBound},
		Session: Session{Scope: scope, State: SessionAuthenticated, AccessToken: "access"},
		Price:   TaskPriceGate{Status: TaskPriceFresh},
		TaskID:  "task-1", ModelID: "model-1", ClientTaskType: "chat", ClientEstimatedCost: "5", Start: start,
	}
}

func taskStartApplication(t *testing.T, scope BindingScope, credit CloudCreditClient) *TaskStartApplication {
	t.Helper()
	resolver, err := NewStaticBindingResolver(taskStartInput(scope, nil).Binding)
	if err != nil {
		t.Fatal(err)
	}
	return NewTaskStartApplication(credit, resolver)
}

type taskStartCreditClient struct {
	result  FreezeResult
	err     error
	calls   int
	request FreezeRequest
}

func (c *taskStartCreditClient) CreateFreeze(_ context.Context, request FreezeRequest) (FreezeResult, error) {
	c.calls++
	c.request = request
	return c.result, c.err
}
func (c *taskStartCreditClient) GetFreeze(context.Context, BindingScope, string) (FreezeResult, error) {
	return FreezeResult{}, errors.New("not used")
}

func taskStartFreezeResult(scope BindingScope, status FreezeStatus, lowBalance bool) FreezeResult {
	return FreezeResult{
		Freeze:  Freeze{Scope: scope, FreezeID: "freeze-1", TaskID: "task-1", Status: status, FreezeAmount: "5", FreeFrozenAmount: "3", SubscriptionFrozenAmount: "2", LowBalanceMode: lowBalance, ExpiresAt: time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)},
		Balance: BalanceSnapshot{TotalBalance: "10", FreeBalance: "5", SubscriptionBalance: "5", AvailableBalance: "5", FreeAvailableBalance: "3", SubscriptionAvailableBalance: "2", FrozenBalance: "5", FreeFrozenBalance: "3", SubscriptionFrozenBalance: "2", Version: 1},
	}
}
