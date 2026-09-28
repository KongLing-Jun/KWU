package modelbillingworker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
)

type supplementerSpy struct {
	requests []QuotaSupplementRequest
	respond  func(QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error)
}

type quotaSupplementGetErrorStore struct {
	*modelbilling.SQLiteQuotaWorkflowStore
	err error
}

func (s *quotaSupplementGetErrorStore) Get(
	context.Context,
	modelbilling.TaskScope,
	string,
) (modelbilling.QuotaWorkflow, error) {
	return modelbilling.QuotaWorkflow{}, s.err
}

func (s *supplementerSpy) Supplement(
	_ context.Context,
	request QuotaSupplementRequest,
) (modelbilling.QuotaEvidenceRevision, error) {
	s.requests = append(s.requests, request)
	if s.respond != nil {
		return s.respond(request)
	}
	return completeSupplementRevision(request, workerTestNow.Add(time.Minute), "7")
}

func TestQuotaSupplementRunnerAppendsEvidenceHistoryAndEnqueuesOneFinal(t *testing.T) {
	for _, status := range []modelbilling.QuotaEvidenceStatus{
		modelbilling.QuotaEvidenceMissing,
		modelbilling.QuotaEvidenceConflicting,
		modelbilling.QuotaEvidenceInvalid,
	} {
		t.Run(string(status), func(t *testing.T) {
			db := openWorkerSQLite(t)
			scope := workerScope(cloud.BindingScope{
				ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 7,
			})
			taskID := "task_supplement_" + string(status)
			task := persistSQLiteUsagePendingTaskWithQuotaStatus(t, db, scope, taskID, status)
			reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow })
			build, pending, err := reports.EnqueueFinal(context.Background(), scope, task.TaskID)
			if err != nil || build.Disposition != modelbilling.ReportBuildEvidencePending || pending != nil {
				t.Fatalf("initial final build = disposition:%s pending:%+v err:%v", build.Disposition, pending, err)
			}

			fake := &supplementerSpy{}
			runner := newSQLiteQuotaSupplementRunner(t, db, reports, fake)
			warnings, err := runner.RunOnce(context.Background(), scope)
			if err != nil || len(warnings) != 0 {
				t.Fatalf("RunOnce warnings=%v err=%v", warnings, err)
			}
			if len(fake.requests) != 1 {
				t.Fatalf("supplement requests = %d, want 1", len(fake.requests))
			}
			request := fake.requests[0]
			if !request.Scope.Equal(scope) || request.TaskID != task.TaskID || request.CallID != "call-"+task.TaskID ||
				request.Endpoint != modelbilling.MaasEndpointChatCompletions || request.CurrentEvidence.Status != status {
				t.Fatalf("supplement request = %+v", request)
			}
			history, err := modelbilling.NewSQLiteQuotaEvidenceStore(db).History(
				context.Background(), scope, task.TaskID, request.CallID,
			)
			if err != nil || len(history) != 2 || history[0].Status != status ||
				history[1].Status != modelbilling.QuotaEvidenceComplete || history[1].Revision != 2 {
				t.Fatalf("history = %+v, err=%v", history, err)
			}
			assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowFinalUploadPending, "")
			assertSQLiteFinalCount(t, db, task.TaskID, 1)

			warnings, err = runner.RunOnce(context.Background(), scope)
			if err != nil || len(warnings) != 0 || len(fake.requests) != 1 {
				t.Fatalf("idempotent rerun warnings=%v err=%v requests=%d", warnings, err, len(fake.requests))
			}
			assertSQLiteFinalCount(t, db, task.TaskID, 1)
		})
	}
}

func TestQuotaSupplementRunnerResumesSupplementingAfterEvidenceAppendCrash(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 3, modelbilling.QuotaEvidenceMissing)
	workflows := modelbilling.NewSQLiteQuotaWorkflowStore(db)
	if err := workflows.Update(context.Background(), modelbilling.QuotaWorkflowUpdate{
		Scope: scope, TaskID: task.TaskID, From: modelbilling.QuotaWorkflowEvidencePending,
		To: modelbilling.QuotaWorkflowSupplementing, At: workerTestNow,
	}); err != nil {
		t.Fatal(err)
	}
	current, found, err := modelbilling.NewSQLiteQuotaEvidenceStore(db).Current(
		context.Background(), scope, task.TaskID, "call-"+task.TaskID,
	)
	if err != nil || !found {
		t.Fatalf("current evidence found=%v err=%v", found, err)
	}
	complete, err := completeSupplementRevision(QuotaSupplementRequest{
		Scope: scope, TaskID: task.TaskID, CallID: "call-" + task.TaskID,
		Endpoint: modelbilling.MaasEndpointChatCompletions, CurrentEvidence: current,
	}, workerTestNow.Add(time.Minute), "9")
	if err != nil {
		t.Fatal(err)
	}
	if err := modelbilling.NewSQLiteQuotaEvidenceStore(db).Append(
		context.Background(), scope, task.TaskID, "call-"+task.TaskID, complete,
	); err != nil {
		t.Fatal(err)
	}

	fake := &supplementerSpy{}
	reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow.Add(2 * time.Minute) })
	runner := newSQLiteQuotaSupplementRunner(t, db, reports, fake)
	warnings, err := runner.RunOnce(context.Background(), scope)
	if err != nil || len(warnings) != 0 || len(fake.requests) != 0 {
		t.Fatalf("resume warnings=%v err=%v supplement calls=%d", warnings, err, len(fake.requests))
	}
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowFinalUploadPending, "")
	assertSQLiteFinalCount(t, db, task.TaskID, 1)
}

func TestQuotaSupplementRunnerClassifiesRetryAndInvalidResponse(t *testing.T) {
	t.Run("retryable returns to evidence pending", func(t *testing.T) {
		db := openWorkerSQLite(t)
		scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 11, modelbilling.QuotaEvidenceMissing)
		fake := &supplementerSpy{respond: func(QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
			return modelbilling.QuotaEvidenceRevision{}, &modelbilling.Error{
				Code: modelbilling.ErrorGatewayUnavailable, Message: "offline", Retryable: true,
			}
		}}
		runner := newSQLiteQuotaSupplementRunner(
			t, db, modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake,
		)
		warnings, err := runner.RunOnce(context.Background(), scope)
		if err != nil || len(warnings) != 1 {
			t.Fatalf("retry warnings=%v err=%v", warnings, err)
		}
		assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowEvidencePending,
			string(modelbilling.ErrorGatewayUnavailable))
		assertSQLiteFinalCount(t, db, task.TaskID, 0)
	})

	t.Run("deterministically invalid response exits hot loop", func(t *testing.T) {
		db := openWorkerSQLite(t)
		scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 12, modelbilling.QuotaEvidenceInvalid)
		fake := &supplementerSpy{respond: func(request QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
			return request.CurrentEvidence, nil
		}}
		runner := newSQLiteQuotaSupplementRunner(
			t, db, modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake,
		)
		warnings, err := runner.RunOnce(context.Background(), scope)
		if err != nil || len(warnings) != 1 {
			t.Fatalf("manual warnings=%v err=%v", warnings, err)
		}
		assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowFailedManualRequired,
			string(modelbilling.ErrorContractInvalid))
		warnings, err = runner.RunOnce(context.Background(), scope)
		if err != nil || len(warnings) != 0 || len(fake.requests) != 1 {
			t.Fatalf("manual rerun warnings=%v err=%v requests=%d", warnings, err, len(fake.requests))
		}
	})
}

func TestQuotaSupplementRunnerWithoutSupplementerLeavesEvidencePending(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 21, modelbilling.QuotaEvidenceMissing)
	runner := newSQLiteQuotaSupplementRunner(t, db,
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), nil)
	warnings, err := runner.RunOnce(context.Background(), scope)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("RunOnce warnings=%v err=%v", warnings, err)
	}
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowEvidencePending, "")
	assertSQLiteFinalCount(t, db, task.TaskID, 0)
}

func TestQuotaSupplementRunnerDoesNotTurnSessionScopeOrLocalFailuresIntoManualState(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode modelbilling.ErrorCode
	}{
		{name: "auth", err: &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "login required"}, wantCode: modelbilling.ErrorAuthRequired},
		{name: "session", err: &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "session expired"}, wantCode: modelbilling.ErrorSessionExpired},
		{name: "scope", err: &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "binding changed"}, wantCode: modelbilling.ErrorScopeMismatch},
		{name: "local transient", err: errors.New("sqlite temporarily busy"), wantCode: modelbilling.ErrorUsageRetryable},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openWorkerSQLite(t)
			scope, task := prepareSQLiteIncompleteQuotaTask(
				t, db, uint64(60+index), modelbilling.QuotaEvidenceMissing,
			)
			fake := &supplementerSpy{respond: func(QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
				return modelbilling.QuotaEvidenceRevision{}, test.err
			}}
			runner := newSQLiteQuotaSupplementRunner(t, db,
				modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake)
			warnings, err := runner.RunOnce(context.Background(), scope)
			if err != nil || len(warnings) != 1 {
				t.Fatalf("RunOnce warnings=%v err=%v", warnings, err)
			}
			assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowEvidencePending,
				string(test.wantCode))
		})
	}
}

func TestQuotaSupplementRunnerQuarantinesDeterministicSQLiteHydrationDamage(t *testing.T) {
	tests := []struct {
		name     string
		corrupt  func(*testing.T, *sql.DB, string)
		wantCode modelbilling.ErrorCode
	}{
		{
			name: "task identity",
			corrupt: func(t *testing.T, db *sql.DB, taskID string) {
				t.Helper()
				if _, err := db.Exec(`UPDATE model_tasks SET actor_id=? WHERE task_id=?`, "other_actor", taskID); err != nil {
					t.Fatal(err)
				}
			},
			wantCode: modelbilling.ErrorScopeMismatch,
		},
		{
			name: "workflow hydration",
			corrupt: func(t *testing.T, db *sql.DB, taskID string) {
				t.Helper()
				if _, err := db.Exec(`UPDATE model_task_quota_workflow SET updated_at=? WHERE task_id=?`, "damaged", taskID); err != nil {
					t.Fatal(err)
				}
			},
			wantCode: modelbilling.ErrorContractInvalid,
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openWorkerSQLite(t)
			scope, task := prepareSQLiteIncompleteQuotaTask(
				t, db, uint64(90+index), modelbilling.QuotaEvidenceMissing,
			)
			test.corrupt(t, db, task.TaskID)
			fake := &supplementerSpy{}
			runner := newSQLiteQuotaSupplementRunner(t, db,
				modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake)

			warnings, err := runner.RunOnce(context.Background(), scope)
			if err != nil || len(warnings) != 1 {
				t.Fatalf("first RunOnce warnings=%v err=%v", warnings, err)
			}
			var status modelbilling.QuotaWorkflowStatus
			var retryStage, lastError sql.NullString
			if err := db.QueryRow(`SELECT status,retry_stage,last_error_code
				FROM model_task_quota_workflow WHERE task_id=?`, task.TaskID).
				Scan(&status, &retryStage, &lastError); err != nil {
				t.Fatal(err)
			}
			if status != modelbilling.QuotaWorkflowFailedManualRequired || retryStage.Valid ||
				!lastError.Valid || lastError.String != string(test.wantCode) {
				t.Fatalf("quarantined workflow status=%s retry=%v error=%v, want failed_manual_required/%s",
					status, retryStage, lastError, test.wantCode)
			}
			warnings, err = runner.RunOnce(context.Background(), scope)
			if err != nil || len(warnings) != 0 || len(fake.requests) != 0 {
				t.Fatalf("second RunOnce warnings=%v err=%v supplement requests=%d", warnings, err, len(fake.requests))
			}
		})
	}
}

func TestQuotaSupplementRunnerDoesNotQuarantineTransientWorkflowGetError(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 92, modelbilling.QuotaEvidenceMissing)
	workflowStore := modelbilling.NewSQLiteQuotaWorkflowStore(db)
	runner, err := NewQuotaSupplementRunner(
		&quotaSupplementGetErrorStore{SQLiteQuotaWorkflowStore: workflowStore, err: errors.New("sqlite temporarily busy")},
		modelbilling.NewSQLiteSubscriptionUsageReader(db), modelbilling.NewSQLiteQuotaEvidenceStore(db),
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }),
		&supplementerSpy{}, func() time.Time { return workerTestNow.Add(3 * time.Minute) },
	)
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := runner.RunOnce(context.Background(), scope)
	if err != nil || len(warnings) != 1 {
		t.Fatalf("RunOnce warnings=%v err=%v", warnings, err)
	}
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowEvidencePending, "")
}

func TestQuotaSupplementRunnerIsolatesBindingGeneration(t *testing.T) {
	db := openWorkerSQLite(t)
	firstScope, first := prepareSQLiteIncompleteQuotaTask(t, db, 31, modelbilling.QuotaEvidenceMissing)
	secondScope, second := prepareSQLiteIncompleteQuotaTask(t, db, 32, modelbilling.QuotaEvidenceMissing)
	if firstScope.CloudBinding.BindingGeneration == secondScope.CloudBinding.BindingGeneration {
		t.Fatal("test scopes unexpectedly share binding generation")
	}
	fake := &supplementerSpy{}
	runner := newSQLiteQuotaSupplementRunner(t, db,
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake)
	warnings, err := runner.RunOnce(context.Background(), firstScope)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("RunOnce warnings=%v err=%v", warnings, err)
	}
	if len(fake.requests) != 1 || fake.requests[0].TaskID != first.TaskID {
		t.Fatalf("requests = %+v", fake.requests)
	}
	assertQuotaWorkflow(t, db, firstScope, first.TaskID, modelbilling.QuotaWorkflowFinalUploadPending, "")
	assertQuotaWorkflow(t, db, secondScope, second.TaskID, modelbilling.QuotaWorkflowEvidencePending, "")
}

type concurrentSupplementer struct {
	calls      atomic.Int32
	secondCall chan struct{}
}

func (s *concurrentSupplementer) Supplement(
	_ context.Context,
	request QuotaSupplementRequest,
) (modelbilling.QuotaEvidenceRevision, error) {
	if s.calls.Add(1) == 1 {
		select {
		case <-s.secondCall:
		case <-time.After(150 * time.Millisecond):
		}
	} else {
		select {
		case <-s.secondCall:
		default:
			close(s.secondCall)
		}
	}
	return completeSupplementRevision(request, workerTestNow.Add(time.Minute), "7")
}

func TestQuotaSupplementRunnerSerializesConcurrentRunOnceCalls(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 70, modelbilling.QuotaEvidenceMissing)
	fake := &concurrentSupplementer{secondCall: make(chan struct{})}
	reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow })
	runner := newSQLiteQuotaSupplementRunner(t, db, reports, fake)
	results := make(chan struct {
		warnings []error
		err      error
	}, 2)
	for range 2 {
		go func() {
			warnings, err := runner.RunOnce(context.Background(), scope)
			results <- struct {
				warnings []error
				err      error
			}{warnings: warnings, err: err}
		}()
	}
	for range 2 {
		result := <-results
		if result.err != nil || len(result.warnings) != 0 {
			t.Fatalf("concurrent RunOnce warnings=%v err=%v", result.warnings, result.err)
		}
	}
	if calls := fake.calls.Load(); calls != 1 {
		t.Fatalf("concurrent supplement calls = %d, want 1", calls)
	}
	assertSQLiteFinalCount(t, db, task.TaskID, 1)
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowFinalUploadPending, "")
}

func TestCoordinatorRunsSupplementAfterUsageRecoveryBeforeDeliveryAndSkipsZeroCallFake(t *testing.T) {
	db := openWorkerSQLite(t)
	scope := workerScope(cloud.BindingScope{
		ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 41,
	})
	task := persistSQLiteUsagePendingTask(
		t, db, modelbilling.NewSQLiteTaskStore(db), scope, "task_zero_supplement", "freeze_zero_supplement", false,
	)
	reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow })
	fake := &supplementerSpy{}
	runner := newSQLiteQuotaSupplementRunner(t, db, reports, fake)
	var events []string
	config := coordinatorConfig(nil)
	config.Supplement = &recordingSupplementPhase{delegate: runner, events: &events}
	coordinator, err := NewCoordinator(
		modelbilling.NewSQLiteSubscriptionUsageReader(db), reports, modelbilling.NewSQLiteTaskStore(db),
		&coordinatorWorkerStub{reconcileErrors: map[string]error{}, events: &events}, config,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.RunOnce(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != 0 {
		t.Fatalf("zero-call supplement requests = %d", len(fake.requests))
	}
	if len(events) < 2 || events[len(events)-2] != "supplement" || events[len(events)-1] != "deliver" {
		t.Fatalf("phase order = %v", events)
	}
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowCollecting, "")
}

type eventQuotaCloud struct {
	delegate *recordingCloud
	events   *[]string
}

func (c *eventQuotaCloud) UploadPartialQuota(
	ctx context.Context,
	scope cloud.BindingScope,
	report modelbilling.PartialQuotaReport,
) error {
	return c.delegate.UploadPartialQuota(ctx, scope, report)
}

func (c *eventQuotaCloud) FinalizeQuota(
	ctx context.Context,
	scope cloud.BindingScope,
	report modelbilling.FinalQuotaReport,
) (modelbilling.QuotaSettlementResult, error) {
	emit(c.events, "deliver_final")
	return c.delegate.FinalizeQuota(ctx, scope, report)
}

func (c *eventQuotaCloud) GetQuotaSettlement(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID string,
) (modelbilling.QuotaSettlementResult, error) {
	return c.delegate.GetQuotaSettlement(ctx, scope, taskID)
}

func TestCoordinatorConvergesEvidencePendingThroughSupplementAndFinalDelivery(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 42, modelbilling.QuotaEvidenceMissing)
	var events []string
	fake := &supplementerSpy{respond: func(request QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
		emit(&events, "supplement_call")
		return completeSupplementRevision(request, workerTestNow.Add(time.Minute), "7")
	}}
	reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow })
	runner := newSQLiteQuotaSupplementRunner(t, db, reports, fake)
	pending := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
	pending.Settlement.TaskTotalQuota = decimalPointer("7")
	cloudSpy := &recordingCloud{t: t, finalizeResults: []cloudResult{{result: pending}}}
	tasks := modelbilling.NewSQLiteTaskStore(db)
	workflows := modelbilling.NewSQLiteQuotaWorkflowStore(db)
	now := workerTestNow.Add(4 * time.Minute)
	worker, err := New(
		reports, tasks, &eventQuotaCloud{delegate: cloudSpy, events: &events}, &observerSpy{},
		testRetryConfig(&now), workflows,
	)
	if err != nil {
		t.Fatal(err)
	}
	config := coordinatorConfig(nil)
	config.Supplement = runner
	coordinator, err := NewCoordinator(
		modelbilling.NewSQLiteSubscriptionUsageReader(db), reports, tasks, worker, config,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.RunOnce(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != 1 || cloudSpy.finalizes != 1 {
		t.Fatalf("calls supplement=%d finalize=%d", len(fake.requests), cloudSpy.finalizes)
	}
	if len(events) != 2 || events[0] != "supplement_call" || events[1] != "deliver_final" {
		t.Fatalf("phase events = %v", events)
	}
	assertSQLiteFinalCount(t, db, task.TaskID, 1)
	var accepted int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage_reports
		WHERE task_id=? AND report_kind=? AND state=?`,
		task.TaskID, modelbilling.ReportFinal, modelbilling.ReportAccepted).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted != 1 {
		t.Fatalf("accepted final count = %d, want 1", accepted)
	}
	history, err := modelbilling.NewSQLiteQuotaEvidenceStore(db).History(
		context.Background(), scope, task.TaskID, "call-"+task.TaskID,
	)
	if err != nil || len(history) != 2 || history[1].Status != modelbilling.QuotaEvidenceComplete {
		t.Fatalf("evidence history=%+v err=%v", history, err)
	}
}

type recordingSupplementPhase struct {
	delegate *QuotaSupplementRunner
	events   *[]string
}

func (p *recordingSupplementPhase) RunOnce(
	ctx context.Context,
	scope modelbilling.TaskScope,
) ([]error, error) {
	emit(p.events, "supplement")
	return p.delegate.RunOnce(ctx, scope)
}

func newSQLiteQuotaSupplementRunner(
	t *testing.T,
	db *sql.DB,
	reports FinalReportEnqueuer,
	supplementer NormalizedQuotaSupplementer,
) *QuotaSupplementRunner {
	t.Helper()
	runner, err := NewQuotaSupplementRunner(
		modelbilling.NewSQLiteQuotaWorkflowStore(db),
		modelbilling.NewSQLiteSubscriptionUsageReader(db),
		modelbilling.NewSQLiteQuotaEvidenceStore(db),
		reports,
		supplementer,
		func() time.Time { return workerTestNow.Add(3 * time.Minute) },
	)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func prepareSQLiteIncompleteQuotaTask(
	t *testing.T,
	db *sql.DB,
	generation uint64,
	status modelbilling.QuotaEvidenceStatus,
) (modelbilling.TaskScope, modelbilling.ModelTask) {
	t.Helper()
	scope := workerScope(cloud.BindingScope{
		ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: generation,
	})
	taskID := fmt.Sprintf("task_incomplete_%d", generation)
	return prepareSQLiteIncompleteQuotaTaskInScope(t, db, scope, taskID, status)
}

func prepareSQLiteIncompleteQuotaTaskInScope(
	t *testing.T,
	db *sql.DB,
	scope modelbilling.TaskScope,
	taskID string,
	status modelbilling.QuotaEvidenceStatus,
) (modelbilling.TaskScope, modelbilling.ModelTask) {
	t.Helper()
	task := persistSQLiteUsagePendingTaskWithQuotaStatus(t, db, scope, taskID, status)
	reports := modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow })
	build, pending, err := reports.EnqueueFinal(context.Background(), scope, task.TaskID)
	if err != nil || build.Disposition != modelbilling.ReportBuildEvidencePending || pending != nil {
		t.Fatalf("prepare final build = disposition:%s pending:%+v err:%v", build.Disposition, pending, err)
	}
	return scope, task
}

func persistSQLiteUsagePendingTaskWithQuotaStatus(
	t *testing.T,
	db *sql.DB,
	scope modelbilling.TaskScope,
	taskID string,
	status modelbilling.QuotaEvidenceStatus,
) modelbilling.ModelTask {
	t.Helper()
	created := workerTestNow.Add(-10 * time.Minute)
	task := modelbilling.ModelTask{
		TaskID: taskID, Scope: scope, BillingSource: modelbilling.BillingSubscriptionPool,
		InitialModelID: "model-worker", ClientTaskType: "chat", ClientEstimatedCost: "10",
		PricingEvidenceID: "price-" + taskID, State: modelbilling.TaskCreated,
		CreatedAt: created, UpdatedAt: created,
	}
	pricing := modelbilling.PricingEvidence{
		EvidenceID: task.PricingEvidenceID, SnapshotID: "snapshot-worker", Scope: scope, TaskID: task.TaskID,
		ModelID: task.InitialModelID, PricingGroup: "default", State: modelbilling.PricingFresh, CapturedAt: created,
	}
	tasks := modelbilling.NewSQLiteTaskStore(db)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, pricing); err != nil {
		t.Fatal(err)
	}
	taskContext, err := modelbilling.NewTaskContext(modelbilling.TaskContextInput{
		TaskID: task.TaskID, Scope: scope, BillingSource: task.BillingSource,
		InitialModelID: task.InitialModelID, PricingEvidenceID: task.PricingEvidenceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := &cloud.FreezeRequest{
		Scope: *scope.CloudBinding, TaskID: task.TaskID, ModelID: task.InitialModelID,
		ClientTaskType: task.ClientTaskType, ClientEstimatedCost: task.ClientEstimatedCost,
		IdempotencyKey: "freeze-request-" + task.TaskID,
	}
	if err := tasks.RecordFreezeRequest(context.Background(), taskContext, request, created.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	freeze := &cloud.Freeze{
		Scope: *scope.CloudBinding, FreezeID: "freeze_" + task.TaskID, TaskID: task.TaskID,
		Status: cloud.FreezeActive, FreezeAmount: "10", FreeFrozenAmount: "4", SubscriptionFrozenAmount: "6",
		ExpiresAt: workerTestNow.Add(time.Hour),
	}
	if err := tasks.RecordFreezeResult(context.Background(), taskContext, freeze,
		&modelbilling.TaskEntitlementSnapshot{Plan: "pro", CapturedAt: created.Add(2 * time.Minute)},
		created.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), modelbilling.TaskStateUpdate{
		Scope: scope, TaskID: task.TaskID, From: modelbilling.TaskReady, To: modelbilling.TaskRunning,
		At: created.Add(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	call := modelbilling.ModelCall{
		CallID: "call-" + task.TaskID, TaskID: task.TaskID, Scope: scope,
		BillingSource: modelbilling.BillingSubscriptionPool, Role: modelbilling.CallRoleMain,
		ModelID: task.InitialModelID, Provider: "fixture", Endpoint: modelbilling.MaasEndpointChatCompletions,
		PricingEvidenceID: task.PricingEvidenceID, State: modelbilling.CallPrepared,
	}
	calls := modelbilling.NewSQLiteCallStore(db)
	if err := calls.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	startedAt := created.Add(4 * time.Minute)
	if err := calls.BeginAttempt(context.Background(), taskContext, call.CallID, startedAt); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	payload := json.RawMessage(`{}`)
	switch status {
	case modelbilling.QuotaEvidenceMissing:
		payload = json.RawMessage(`{}`)
	case modelbilling.QuotaEvidenceConflicting:
		payload = json.RawMessage(`{"billing":{"quota":1},"usage":{"quota":2}}`)
	case modelbilling.QuotaEvidenceInvalid:
		payload = json.RawMessage(`{"billing":{"quota":"invalid"}}`)
	default:
		t.Fatalf("unsupported initial evidence status %q", status)
	}
	quota, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint:  modelbilling.MaasEndpointChatCompletions,
		Headers:   map[string][]string{"X-Oneapi-Request-Id": {"request-" + task.TaskID}},
		FinalJSON: payload, ObservedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if quota.Status != status {
		t.Fatalf("normalized initial evidence status = %s, want %s", quota.Status, status)
	}
	requestCount := int64(1)
	call.State, call.FinishedAt, call.QuotaEvidence = modelbilling.CallCompleted, &finishedAt, &quota
	call.Usage = modelbilling.NormalizedUsage{RequestCount: &requestCount, BreakdownStatus: modelbilling.UsageExact}
	if err := calls.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	result := modelbilling.TaskResultCompleted
	if err := tasks.UpdateState(context.Background(), modelbilling.TaskStateUpdate{
		Scope: scope, TaskID: task.TaskID, From: modelbilling.TaskRunning, To: modelbilling.TaskUsagePending,
		Result: &result, At: workerTestNow.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := tasks.Get(context.Background(), scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func completeSupplementRevision(
	request QuotaSupplementRequest,
	observedAt time.Time,
	quota string,
) (modelbilling.QuotaEvidenceRevision, error) {
	requestID := "supplement-" + request.CallID
	if request.CurrentEvidence.OneAPIRequestID != nil {
		requestID = *request.CurrentEvidence.OneAPIRequestID
	}
	evidence, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint:   request.Endpoint,
		Headers:    map[string][]string{"X-Oneapi-Request-Id": {requestID}},
		FinalJSON:  json.RawMessage(fmt.Sprintf(`{"billing":{"quota":%s}}`, quota)),
		ObservedAt: observedAt,
	})
	if err != nil {
		return modelbilling.QuotaEvidenceRevision{}, err
	}
	evidence.Revision = request.CurrentEvidence.Revision + 1
	return evidence, nil
}

func assertQuotaWorkflow(
	t *testing.T,
	db *sql.DB,
	scope modelbilling.TaskScope,
	taskID string,
	want modelbilling.QuotaWorkflowStatus,
	wantCode string,
) {
	t.Helper()
	workflow, err := modelbilling.NewSQLiteQuotaWorkflowStore(db).Get(context.Background(), scope, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Status != want {
		t.Fatalf("workflow status = %s, want %s", workflow.Status, want)
	}
	if wantCode == "" {
		if workflow.LastErrorCode != nil {
			t.Fatalf("workflow last error = %q", *workflow.LastErrorCode)
		}
	} else if workflow.LastErrorCode == nil || *workflow.LastErrorCode != wantCode {
		t.Fatalf("workflow last error = %v, want %q", workflow.LastErrorCode, wantCode)
	}
}

func assertSQLiteFinalCount(t *testing.T, db *sql.DB, taskID string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM usage_reports WHERE task_id=? AND report_kind=?`,
		taskID, modelbilling.ReportFinal).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("final report count = %d, want %d", count, want)
	}
}

func TestQuotaSupplementRunnerStandaloneCancellationRetriesWithoutManualFailure(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 51, modelbilling.QuotaEvidenceMissing)
	fake := &supplementerSpy{respond: func(QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
		return modelbilling.QuotaEvidenceRevision{}, context.Canceled
	}}
	runner := newSQLiteQuotaSupplementRunner(t, db,
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake)
	warnings, err := runner.RunOnce(context.Background(), scope)
	if err != nil || len(warnings) != 1 || !errors.Is(warnings[0], context.Canceled) {
		t.Fatalf("cancellation warnings=%v err=%v", warnings, err)
	}
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowEvidencePending,
		string(modelbilling.ErrorUsageRetryable))
}

func TestQuotaSupplementRunnerParentCancellationStopsWithoutFabricatingWorkflowState(t *testing.T) {
	db := openWorkerSQLite(t)
	scope, task := prepareSQLiteIncompleteQuotaTask(t, db, 52, modelbilling.QuotaEvidenceMissing)
	ctx, cancel := context.WithCancel(context.Background())
	fake := &supplementerSpy{respond: func(QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
		cancel()
		return modelbilling.QuotaEvidenceRevision{}, context.Canceled
	}}
	runner := newSQLiteQuotaSupplementRunner(t, db,
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake)
	warnings, err := runner.RunOnce(ctx, scope)
	if !errors.Is(err, context.Canceled) || len(warnings) != 0 {
		t.Fatalf("parent cancellation warnings=%v err=%v", warnings, err)
	}
	assertQuotaWorkflow(t, db, scope, task.TaskID, modelbilling.QuotaWorkflowSupplementing, "")
}

func TestQuotaSupplementRunnerStandaloneDeadlineRetriesTaskAndContinues(t *testing.T) {
	db := openWorkerSQLite(t)
	scope := workerScope(cloud.BindingScope{
		ProfileID: "default", CloudUserID: "cloud_user", BindingGeneration: 81,
	})
	_, first := prepareSQLiteIncompleteQuotaTaskInScope(
		t, db, scope, "task_a_deadline", modelbilling.QuotaEvidenceMissing,
	)
	_, second := prepareSQLiteIncompleteQuotaTaskInScope(
		t, db, scope, "task_b_complete", modelbilling.QuotaEvidenceMissing,
	)
	fake := &supplementerSpy{respond: func(request QuotaSupplementRequest) (modelbilling.QuotaEvidenceRevision, error) {
		if request.TaskID == first.TaskID {
			return modelbilling.QuotaEvidenceRevision{}, context.DeadlineExceeded
		}
		return completeSupplementRevision(request, workerTestNow.Add(time.Minute), "7")
	}}
	runner := newSQLiteQuotaSupplementRunner(t, db,
		modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return workerTestNow }), fake)
	warnings, err := runner.RunOnce(context.Background(), scope)
	if err != nil || len(warnings) != 1 || !errors.Is(warnings[0], context.DeadlineExceeded) {
		t.Fatalf("RunOnce warnings=%v err=%v", warnings, err)
	}
	if len(fake.requests) != 2 {
		t.Fatalf("supplement requests = %d, want 2", len(fake.requests))
	}
	assertQuotaWorkflow(t, db, scope, first.TaskID, modelbilling.QuotaWorkflowEvidencePending,
		string(modelbilling.ErrorUsageRetryable))
	assertQuotaWorkflow(t, db, scope, second.TaskID, modelbilling.QuotaWorkflowFinalUploadPending, "")
	assertSQLiteFinalCount(t, db, second.TaskID, 1)
}
