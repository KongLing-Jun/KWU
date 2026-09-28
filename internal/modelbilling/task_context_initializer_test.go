package modelbilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/userctx"
)

type initializerRouteResolver struct {
	route ResolvedBillingRoute
	err   error
	calls int
}

func (r *initializerRouteResolver) ResolveBillingRoute(_ context.Context, _ userctx.ActorContext, _ BillingRouteSelection) (ResolvedBillingRoute, error) {
	r.calls++
	return r.route, r.err
}

type initializerSubscriptionResolver struct {
	bootstrap SubscriptionBootstrap
	calls     int
}

func (r *initializerSubscriptionResolver) ResolveSubscriptionBootstrap(context.Context, userctx.ActorContext, ResolvedBillingRoute) (SubscriptionBootstrap, error) {
	r.calls++
	return r.bootstrap, nil
}

type initializerCreditConfig struct{ calls int }

func (c *initializerCreditConfig) GetCreditConfig(context.Context, cloud.BindingScope) (CreditConfig, error) {
	c.calls++
	version := "fixture-v1"
	return CreditConfig{CreditPerCNY: "2000", PricingGroup: "default", Version: &version, FetchedAt: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)}, nil
}

type initializerFreezeClient struct{ calls int }

func (c *initializerFreezeClient) CreateFreeze(_ context.Context, request cloud.FreezeRequest) (cloud.FreezeResult, error) {
	c.calls++
	return cloud.FreezeResult{Freeze: cloud.Freeze{Scope: request.Scope, FreezeID: "freeze-1", TaskID: request.TaskID, Status: cloud.FreezeActive, FreezeAmount: "1", FreeFrozenAmount: "1", SubscriptionFrozenAmount: "0", ExpiresAt: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)}, Balance: cloud.BalanceSnapshot{TotalBalance: "1", FreeBalance: "1", SubscriptionBalance: "0", AvailableBalance: "0", FreeAvailableBalance: "0", SubscriptionAvailableBalance: "0", FrozenBalance: "1", FreeFrozenBalance: "1", SubscriptionFrozenBalance: "0", Version: 1}}, nil
}
func (*initializerFreezeClient) GetFreeze(context.Context, cloud.BindingScope, string) (cloud.FreezeResult, error) {
	return cloud.FreezeResult{}, errors.New("not used")
}

type initializerTaskStore struct {
	created        []ModelTask
	evidence       []PricingEvidence
	updates        []TaskStateUpdate
	freezeRequests int
	freezeResults  int
}

func (s *initializerTaskStore) CreateTaskWithEvidence(_ context.Context, task ModelTask, evidence PricingEvidence) error {
	s.created, s.evidence = append(s.created, task), append(s.evidence, evidence)
	return nil
}
func (s *initializerTaskStore) RecordFreezeRequest(context.Context, TaskContext, *cloud.FreezeRequest, time.Time) error {
	s.freezeRequests++
	return nil
}
func (s *initializerTaskStore) RecordFreezeResult(context.Context, TaskContext, *cloud.Freeze, *TaskEntitlementSnapshot, time.Time) error {
	s.freezeResults++
	return nil
}
func (s *initializerTaskStore) UpdateState(_ context.Context, update TaskStateUpdate) error {
	s.updates = append(s.updates, update)
	return nil
}
func (*initializerTaskStore) Get(context.Context, TaskScope, string) (ModelTask, error) {
	return ModelTask{}, errors.New("not used")
}
func (*initializerTaskStore) ListRecoverable(context.Context, TaskScope) ([]ModelTask, error) {
	return nil, errors.New("not used")
}

type initializerIDs struct{ values []string }

func (g *initializerIDs) NewID() (string, error) {
	next := g.values[0]
	g.values = g.values[1:]
	return next, nil
}

func TestTaskContextInitializerSubscriptionPersistsBeforeFreeze(t *testing.T) {
	scope := testBindingScope()
	routes := &initializerRouteResolver{route: ResolvedBillingRoute{ModelRef: "fixture/model", ModelID: "model_fixture_01", Provider: "subscription_gateway_fixture", BillingSource: BillingSubscriptionPool}}
	subscription := &initializerSubscriptionResolver{bootstrap: SubscriptionBootstrap{Scope: scope, Plan: "plus"}}
	credit, freeze, tasks := &initializerCreditConfig{}, &initializerFreezeClient{}, &initializerTaskStore{}
	recovery := cloud.NewMemoryTaskRecoveryStore()
	initializer := TaskContextInitializer{Routes: routes, Subscriptions: subscription, Tasks: tasks, CreditConfig: credit, Freeze: freeze, TaskRecovery: recovery, IDs: &initializerIDs{values: []string{"task-1", "evidence-1"}}, Now: func() time.Time { return time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC) }}
	task, err := initializer.InitializeTaskContext(context.Background(), userctx.ActorContext{ActorID: "actor", ProfileID: "default"}, BillingRouteSelection{ModelRef: "fixture/model", BillingSource: BillingSubscriptionPool})
	if err != nil {
		t.Fatal(err)
	}
	if task.BillingSource != BillingSubscriptionPool || task.Scope.CloudBinding == nil || *task.Scope.CloudBinding != scope {
		t.Fatalf("task = %+v", task)
	}
	if len(tasks.created) != 1 || tasks.created[0].State != TaskCreated || len(tasks.evidence) != 1 || tasks.evidence[0].PricingGroup != "default" {
		t.Fatalf("bootstrap = %#v %#v", tasks.created, tasks.evidence)
	}
	if credit.calls != 1 || freeze.calls != 1 || tasks.freezeRequests != 1 || tasks.freezeResults != 1 || len(tasks.updates) != 0 {
		t.Fatalf("calls credit/freeze/request/result/updates = %d/%d/%d/%d/%d", credit.calls, freeze.calls, tasks.freezeRequests, tasks.freezeResults, len(tasks.updates))
	}
	records, err := recovery.Load(context.Background(), scope)
	if err != nil || len(records) != 1 || records[0].TaskID != task.TaskID || records[0].FreezeID != "freeze-1" || records[0].IdempotencyKey == "" || records[0].HasUsage {
		t.Fatalf("task recovery record = %#v, %v", records, err)
	}
}

func TestTaskContextInitializerBYOKDoesNotTouchCloud(t *testing.T) {
	routes := &initializerRouteResolver{route: ResolvedBillingRoute{ModelRef: "byok/model", ModelID: "model", Provider: "byok", BillingSource: BillingNewAPIByok}}
	credit, freeze, tasks := &initializerCreditConfig{}, &initializerFreezeClient{}, &initializerTaskStore{}
	initializer := TaskContextInitializer{Routes: routes, Tasks: tasks, CreditConfig: credit, Freeze: freeze, IDs: &initializerIDs{values: []string{"task-1", "evidence-1"}}}
	task, err := initializer.InitializeTaskContext(context.Background(), userctx.ActorContext{ActorID: "actor", ProfileID: "default"}, BillingRouteSelection{ModelRef: "byok/model", BillingSource: BillingNewAPIByok})
	if err != nil {
		t.Fatal(err)
	}
	if task.Scope.CloudBinding != nil || credit.calls != 0 || freeze.calls != 0 || tasks.freezeRequests != 0 || tasks.freezeResults != 0 {
		t.Fatalf("BYOK touched cloud: task=%+v credit=%d freeze=%d", task, credit.calls, freeze.calls)
	}
	if len(tasks.updates) != 1 || tasks.updates[0].From != TaskCreated || tasks.updates[0].To != TaskReady {
		t.Fatalf("updates = %#v", tasks.updates)
	}
	if len(tasks.evidence) != 1 || tasks.evidence[0].SnapshotID != "pricing-unavailable-v2:newapi_byok:group_unavailable" || tasks.evidence[0].PricingGroup != "__kwu_unavailable__" {
		t.Fatalf("BYOK group-unavailable evidence = %#v", tasks.evidence)
	}
}

func TestTaskContextInitializerRejectsRouteIdentityMismatchBeforeExternalWork(t *testing.T) {
	cases := []struct {
		name      string
		selection BillingRouteSelection
		route     ResolvedBillingRoute
	}{
		{
			name:      "billing_source",
			selection: BillingRouteSelection{ModelRef: "byok/model", BillingSource: BillingNewAPIByok},
			route:     ResolvedBillingRoute{ModelRef: "byok/model", ModelID: "model", Provider: "subscription_gateway_fixture", BillingSource: BillingSubscriptionPool},
		},
		{
			name:      "model_ref",
			selection: BillingRouteSelection{ModelRef: "byok/model", BillingSource: BillingNewAPIByok},
			route:     ResolvedBillingRoute{ModelRef: "byok/other", ModelID: "other", Provider: "byok", BillingSource: BillingNewAPIByok},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			routes := &initializerRouteResolver{route: tc.route}
			subscription := &initializerSubscriptionResolver{bootstrap: SubscriptionBootstrap{Scope: testBindingScope(), Plan: "plus"}}
			credit, freeze, tasks := &initializerCreditConfig{}, &initializerFreezeClient{}, &initializerTaskStore{}
			initializer := TaskContextInitializer{Routes: routes, Subscriptions: subscription, Tasks: tasks, CreditConfig: credit, Freeze: freeze, IDs: &initializerIDs{values: []string{"task", "evidence"}}}
			if _, err := initializer.InitializeTaskContext(context.Background(), userctx.ActorContext{ActorID: "actor", ProfileID: "default"}, tc.selection); ErrorCodeOf(err) != ErrorBillingSourceMismatch {
				t.Fatalf("identity mismatch error = %v", err)
			}
			if subscription.calls != 0 || credit.calls != 0 || freeze.calls != 0 || len(tasks.created) != 0 {
				t.Fatalf("identity mismatch reached external work: bootstrap/credit/freeze/tasks = %d/%d/%d/%d", subscription.calls, credit.calls, freeze.calls, len(tasks.created))
			}
		})
	}
}
