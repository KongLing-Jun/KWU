package modelbillingworker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

func TestB4LocalL1AcceptedFinalRestartConvergesEventRESTAndCheckpoint(t *testing.T) {
	ctx := context.Background()
	db := openWorkerSQLite(t)
	scope := workerTestScope()
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}.Normalize()
	tasks := modelbilling.NewSQLiteTaskStore(db)
	reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow })
	workflows := modelbilling.NewSQLiteQuotaWorkflowStore(db)
	task := persistSQLiteUsagePendingTask(
		t, db, tasks, scope, "task-b4-local-l1", "freeze-b4-local-l1", true,
	)
	build, pending, err := reports.EnqueueFinal(ctx, scope, task.TaskID)
	if err != nil || build.Disposition != modelbilling.ReportBuildDeliverable || pending == nil {
		t.Fatalf("EnqueueFinal = disposition:%s pending:%+v err:%v", build.Disposition, pending, err)
	}

	pendingSettlement := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
	terminalSettlement := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	client := &recordingCloud{
		t:               t,
		finalizeResults: []cloudResult{{result: pendingSettlement}},
		getResults:      []cloudResult{{result: terminalSettlement}},
	}
	trace := []string{}
	observer := &b4LocalL1Observer{
		db: db, trace: &trace, scope: *scope.CloudBinding,
		taskID: task.TaskID, settlementID: terminalSettlement.Settlement.SettlementID,
	}
	now := workerTestNow
	beforeRestart := newWorkerWithWorkflowForTest(t, reports, tasks, client, observer, workflows, &now)
	if found, err := beforeRestart.DeliverOne(ctx, scope); err != nil || !found {
		t.Fatalf("DeliverOne before restart = found:%t err:%v", found, err)
	}
	if client.finalizes != 1 || client.gets != 0 || observer.calls != 0 {
		t.Fatalf("before restart calls = finalize:%d get:%d observer:%d", client.finalizes, client.gets, observer.calls)
	}

	// Recreate every worker-owned store around the same SQLite database. The
	// accepted final is durable, so event reconciliation may only query it.
	restarted := newWorkerWithWorkflowForTest(
		t,
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }),
		modelbilling.NewSQLiteTaskStore(db), client, observer,
		modelbilling.NewSQLiteQuotaWorkflowStore(db), &now,
	)
	binding := cloud.ProfileBinding{
		ActorID: actor.ActorID, ProfileID: scope.ProfileID, CloudUserID: scope.CloudBinding.CloudUserID,
		Status: cloud.BindingBound, BindingGeneration: scope.CloudBinding.BindingGeneration,
	}
	bindings, err := cloud.NewStaticBindingResolver(binding)
	if err != nil {
		t.Fatal(err)
	}
	state := cloud.NewMemoryEventStateRepository()
	otherGeneration := *scope.CloudBinding
	otherGeneration.BindingGeneration++
	if err := state.CommitApplied(ctx, otherGeneration, b4LocalL1Event(
		"other-generation", otherGeneration.CloudUserID, 99, task.TaskID,
		terminalSettlement.Settlement.SettlementID,
	)); err != nil {
		t.Fatal(err)
	}
	balance := &b4LocalL1Balance{trace: &trace}
	tracedState := &b4LocalL1EventState{EventStateRepository: state, trace: &trace}
	deps := cloud.EventSyncApplicationDependencies{
		Bindings: bindings, State: tracedState, Balance: balance, Tasks: &b4LocalL1Tasks{},
		Settlements: restarted, Subscription: b4LocalL1Subscription{},
		Payment: b4LocalL1Payment{}, Sync: b4LocalL1Sync{},
	}
	settledEvent := b4LocalL1Event(
		"settlement-v1", scope.CloudBinding.CloudUserID, 1, task.TaskID,
		terminalSettlement.Settlement.SettlementID,
	)
	if err := runB4LocalL1Event(ctx, deps, settledEvent); err != nil {
		t.Fatal(err)
	}
	if got, want := trace[:3], []string{"observer", "balance", "checkpoint"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event convergence prefix = %v, want %v; full trace=%v", got, want, trace)
	}
	assertB4LocalL1Settlement(t, db, scope, task.TaskID, 2, modelbilling.QuotaSettlementSettled)
	if client.finalizes != 1 || client.gets != 1 || observer.calls != 1 {
		t.Fatalf("restart calls = finalize:%d get:%d observer:%d", client.finalizes, client.gets, observer.calls)
	}
	storedTask, err := tasks.Get(ctx, scope, task.TaskID)
	if err != nil || storedTask.State != modelbilling.TaskCompleted {
		t.Fatalf("settled task = state:%s err:%v", storedTask.State, err)
	}
	assertB4LocalL1Checkpoint(t, state, *scope.CloudBinding, settledEvent.EventID, 1)
	assertB4LocalL1Checkpoint(t, state, otherGeneration, "other-generation", 99)

	// A later signal against the already-completed task still performs the
	// authoritative REST refresh before checkpointing. A failed refresh leaves
	// the event replayable without repeating settlement persistence/observation.
	retryEvent := b4LocalL1Event(
		"settlement-v2", scope.CloudBinding.CloudUserID, 2, task.TaskID,
		terminalSettlement.Settlement.SettlementID,
	)
	balance.failNext = errors.New("normalized balance refresh unavailable")
	if err := runB4LocalL1Event(ctx, deps, retryEvent); err == nil {
		t.Fatal("event with failed balance refresh unexpectedly succeeded")
	}
	assertB4LocalL1Checkpoint(t, state, *scope.CloudBinding, settledEvent.EventID, 1)
	if err := runB4LocalL1Event(ctx, deps, retryEvent); err != nil {
		t.Fatalf("event replay after balance recovery: %v", err)
	}
	assertB4LocalL1Checkpoint(t, state, *scope.CloudBinding, retryEvent.EventID, 2)
	assertB4LocalL1Settlement(t, db, scope, task.TaskID, 2, modelbilling.QuotaSettlementSettled)
	if client.finalizes != 1 || client.gets != 1 || observer.calls != 1 {
		t.Fatalf("REST replay repeated settlement effects = finalize:%d get:%d observer:%d",
			client.finalizes, client.gets, observer.calls)
	}
	var accepted int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM usage_reports WHERE task_id=? AND state=?`, task.TaskID, modelbilling.ReportAccepted,
	).Scan(&accepted); err != nil || accepted != 1 {
		t.Fatalf("accepted finals = %d, err=%v", accepted, err)
	}
}

func runB4LocalL1Event(ctx context.Context, deps cloud.EventSyncApplicationDependencies, event cloud.Event) error {
	source, err := cloud.NewFixtureEventSource([]cloud.Event{event})
	if err != nil {
		return err
	}
	deps.Source = source
	app, err := cloud.NewEventSyncApplication(deps)
	if err != nil {
		return err
	}
	return app.Run(ctx, userctx.ActorContext{ActorID: "actor-worker", ProfileID: "default"})
}

func b4LocalL1Event(eventID, cloudUserID string, version uint64, taskID, settlementID string) cloud.Event {
	data, _ := json.Marshal(map[string]string{"task_id": taskID, "settlement_id": settlementID})
	return cloud.Event{
		EventID: eventID, EventType: cloud.EventSettlementCompleted, CloudUserID: cloudUserID,
		OccurredAt: workerTestNow, Version: version, Data: data,
	}
}

type b4LocalL1Observer struct {
	db           *sql.DB
	trace        *[]string
	scope        cloud.BindingScope
	taskID       string
	settlementID string
	calls        int
}

func (o *b4LocalL1Observer) CreditSettlementObserved(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID, settlementID string,
) error {
	if scope != o.scope || taskID != o.taskID || settlementID != o.settlementID {
		return fmt.Errorf("observer identity mismatch")
	}
	var revisions int
	if err := o.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM model_task_quota_settlement_revisions WHERE task_id=?`, taskID,
	).Scan(&revisions); err != nil {
		return err
	}
	if revisions != 2 {
		return fmt.Errorf("observer ran before terminal settlement persistence: revisions=%d", revisions)
	}
	o.calls++
	*o.trace = append(*o.trace, "observer")
	return nil
}

type b4LocalL1Balance struct {
	trace    *[]string
	failNext error
}

func (b *b4LocalL1Balance) Refresh(context.Context, userctx.ActorContext) (cloud.BalanceView, error) {
	*b.trace = append(*b.trace, "balance")
	if b.failNext != nil {
		err := b.failNext
		b.failNext = nil
		return cloud.BalanceView{}, err
	}
	return cloud.BalanceView{}, nil
}

type b4LocalL1EventState struct {
	cloud.EventStateRepository
	trace *[]string
}

func (s *b4LocalL1EventState) CommitApplied(ctx context.Context, scope cloud.BindingScope, event cloud.Event) error {
	if err := s.EventStateRepository.CommitApplied(ctx, scope, event); err != nil {
		return err
	}
	*s.trace = append(*s.trace, "checkpoint")
	return nil
}

type b4LocalL1Tasks struct{}

func (*b4LocalL1Tasks) RecoverPendingForEventSync(
	context.Context, userctx.ActorContext, cloud.TaskRecoveryExclusion,
) ([]cloud.TaskRecoveryDecision, error) {
	return nil, nil
}

func (*b4LocalL1Tasks) RecoverExact(
	context.Context, userctx.ActorContext, cloud.BindingScope, string, string,
) (cloud.TaskRecoveryDecision, bool, error) {
	return cloud.TaskRecoveryDecision{}, false, nil
}

func (*b4LocalL1Tasks) StageEventSync(
	context.Context, cloud.BindingScope, string, []cloud.TaskRecoveryDecision,
) error {
	return nil
}

func (*b4LocalL1Tasks) AcknowledgeEventSync(context.Context, cloud.BindingScope, string) error {
	return nil
}

type b4LocalL1Subscription struct{}

func (b4LocalL1Subscription) Refresh(context.Context, userctx.ActorContext) (cloud.SubscriptionView, error) {
	return cloud.SubscriptionView{}, nil
}

type b4LocalL1Payment struct{}

func (b4LocalL1Payment) Restore(context.Context, userctx.ActorContext) ([]cloud.PaymentRecoveryResult, error) {
	return nil, nil
}

type b4LocalL1Sync struct{}

func (b4LocalL1Sync) ApplyThirdPartySync(context.Context, userctx.ActorContext, cloud.ThirdPartySync) error {
	return nil
}

func assertB4LocalL1Checkpoint(
	t *testing.T,
	state cloud.EventStateRepository,
	scope cloud.BindingScope,
	eventID string,
	version uint64,
) {
	t.Helper()
	checkpoint, found, err := state.Load(context.Background(), scope)
	if err != nil || !found || checkpoint.Cursor != eventID || checkpoint.Version(cloud.EventSettlementCompleted) != version {
		t.Fatalf("checkpoint = %+v found:%t err:%v, want event:%s version:%d", checkpoint, found, err, eventID, version)
	}
}

func assertB4LocalL1Settlement(
	t *testing.T,
	db *sql.DB,
	scope modelbilling.TaskScope,
	taskID string,
	wantRevisions int,
	wantStatus modelbilling.QuotaSettlementState,
) {
	t.Helper()
	var revisions int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM model_task_quota_settlement_revisions WHERE task_id=?`, taskID,
	).Scan(&revisions); err != nil || revisions != wantRevisions {
		t.Fatalf("settlement revisions = %d, err=%v, want %d", revisions, err, wantRevisions)
	}
	current, found, err := modelbilling.NewSQLiteQuotaWorkflowStore(db).CurrentQuotaSettlement(
		context.Background(), scope, taskID,
	)
	if err != nil || !found || current.Settlement.Status != wantStatus {
		t.Fatalf("current settlement = %+v found:%t err:%v", current, found, err)
	}
}
