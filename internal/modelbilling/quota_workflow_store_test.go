package modelbilling

import (
	"context"
	"testing"
	"time"
)

type quotaWorkflowStoreFixture struct {
	store   *SQLiteQuotaWorkflowStore
	task    ModelTask
	initial time.Time
}

func TestSQLiteQuotaWorkflowStoreRetryStageMatchesSource(t *testing.T) {
	t.Run("final upload", func(t *testing.T) {
		fixture := newQuotaWorkflowStoreFixture(t, "workflow_retry_source_final")
		at := fixture.initial.Add(time.Minute)
		mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowCollecting, QuotaWorkflowFinalUploadPending, nil, nil, at)

		wrongStage := QuotaRetrySettlementQuery
		err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
			Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
			From: QuotaWorkflowFinalUploadPending, To: QuotaWorkflowRetryPending,
			RetryStage: &wrongStage, At: at.Add(time.Minute),
		})
		requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
		requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowFinalUploadPending)

		stage := QuotaRetryFinalUpload
		mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowFinalUploadPending, QuotaWorkflowRetryPending, &stage, nil, at.Add(time.Minute))
	})

	t.Run("settlement query", func(t *testing.T) {
		fixture := newQuotaWorkflowStoreFixture(t, "workflow_retry_source_settlement")
		at := advanceQuotaWorkflowToSettlementPending(t, fixture)

		wrongStage := QuotaRetryFinalUpload
		err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
			Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
			From: QuotaWorkflowSettlementPending, To: QuotaWorkflowRetryPending,
			RetryStage: &wrongStage, At: at.Add(time.Minute),
		})
		requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
		requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowSettlementPending)

		stage := QuotaRetrySettlementQuery
		mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowSettlementPending, QuotaWorkflowRetryPending, &stage, nil, at.Add(time.Minute))
	})
}

func TestSQLiteQuotaWorkflowStoreRetryStageRestrictsRecovery(t *testing.T) {
	t.Run("final upload only resumes uploading", func(t *testing.T) {
		fixture := newQuotaWorkflowStoreFixture(t, "workflow_retry_target_final")
		at := fixture.initial.Add(time.Minute)
		mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowCollecting, QuotaWorkflowFinalUploadPending, nil, nil, at)
		stage := QuotaRetryFinalUpload
		mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowFinalUploadPending, QuotaWorkflowRetryPending, &stage, nil, at.Add(time.Minute))

		for _, target := range []QuotaWorkflowStatus{QuotaWorkflowSettlementPending, QuotaWorkflowFailedManualRequired} {
			err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
				Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
				From: QuotaWorkflowRetryPending, To: target, At: at.Add(2 * time.Minute),
			})
			requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
		}
		requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowRetryPending)
		mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowRetryPending, QuotaWorkflowFinalUploading, nil, nil, at.Add(2*time.Minute))
	})

	allowed := []QuotaWorkflowStatus{
		QuotaWorkflowSettlementPending,
		QuotaWorkflowSettled,
		QuotaWorkflowSettledWithUncovered,
		QuotaWorkflowFailedManualRequired,
	}
	for _, target := range allowed {
		t.Run("settlement query resumes "+string(target), func(t *testing.T) {
			fixture := newQuotaWorkflowStoreFixture(t, "workflow_retry_target_"+string(target))
			at := advanceQuotaWorkflowToSettlementRetry(t, fixture)
			mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowRetryPending, target, nil, nil, at.Add(time.Minute))
			workflow := requireQuotaWorkflowStatus(t, fixture, target)
			if workflow.RetryStage != nil {
				t.Fatalf("recovered workflow retained retry stage: %+v", workflow)
			}
		})
	}

	t.Run("settlement query cannot resume final upload", func(t *testing.T) {
		fixture := newQuotaWorkflowStoreFixture(t, "workflow_retry_target_wrong_final")
		at := advanceQuotaWorkflowToSettlementRetry(t, fixture)
		err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
			Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
			From: QuotaWorkflowRetryPending, To: QuotaWorkflowFinalUploading, At: at.Add(time.Minute),
		})
		requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
		requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowRetryPending)
	})
}

func TestSQLiteQuotaWorkflowStoreSameStateIdempotencyChecksRetryFacts(t *testing.T) {
	fixture := newQuotaWorkflowStoreFixture(t, "workflow_same_state")
	at := fixture.initial.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowCollecting, QuotaWorkflowFinalUploadPending, nil, nil, at)
	stage := QuotaRetryFinalUpload
	lastError := "usage_retryable"
	retryAt := at.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowFinalUploadPending, QuotaWorkflowRetryPending, &stage, &lastError, retryAt)

	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowFinalUploadPending, QuotaWorkflowRetryPending, &stage, &lastError, retryAt.Add(time.Minute))
	workflow := requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowRetryPending)
	if !workflow.UpdatedAt.Equal(retryAt) {
		t.Fatalf("idempotent retry changed updated_at: got %s want %s", workflow.UpdatedAt, retryAt)
	}

	changedError := "different_error"
	err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
		Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
		From: QuotaWorkflowFinalUploadPending, To: QuotaWorkflowRetryPending,
		RetryStage: &stage, LastErrorCode: &changedError, At: retryAt.Add(2 * time.Minute),
	})
	requireQuotaWorkflowStoreCode(t, err, ErrorIdempotencyConflict)

	changedStage := QuotaRetrySettlementQuery
	err = fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
		Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
		From: QuotaWorkflowFinalUploadPending, To: QuotaWorkflowRetryPending,
		RetryStage: &changedStage, LastErrorCode: &lastError, At: retryAt.Add(2 * time.Minute),
	})
	requireQuotaWorkflowStoreCode(t, err, ErrorIdempotencyConflict)
}

func TestSQLiteQuotaWorkflowStoreRejectsUpdatedAtRegression(t *testing.T) {
	fixture := newQuotaWorkflowStoreFixture(t, "workflow_time_regression")
	err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
		Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
		From: QuotaWorkflowCollecting, To: QuotaWorkflowEvidencePending,
		At: fixture.initial.Add(-time.Nanosecond),
	})
	requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
	requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowCollecting)

	updatedAt := fixture.initial.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowCollecting, QuotaWorkflowEvidencePending, nil, nil, updatedAt)
	err = fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
		Scope: fixture.task.Scope, TaskID: fixture.task.TaskID,
		From: QuotaWorkflowCollecting, To: QuotaWorkflowEvidencePending,
		At: fixture.initial,
	})
	requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
	workflow := requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowEvidencePending)
	if !workflow.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("regressed update changed updated_at: got %s want %s", workflow.UpdatedAt, updatedAt)
	}
}

func TestSQLiteQuotaWorkflowStoreManualSupplementRecoveryRejectsAdvancedState(t *testing.T) {
	fixture := newQuotaWorkflowStoreFixture(t, "workflow_manual_supplement_advanced")
	at := fixture.initial.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowCollecting, QuotaWorkflowFinalUploadPending, nil, nil, at)
	err := fixture.store.MarkSupplementFailedManualRequired(
		context.Background(), fixture.task.Scope, fixture.task.TaskID, at.Add(time.Minute), string(ErrorContractInvalid),
	)
	requireQuotaWorkflowStoreCode(t, err, ErrorInvalidStateTransition)
	requireQuotaWorkflowStatus(t, fixture, QuotaWorkflowFinalUploadPending)
}

func newQuotaWorkflowStoreFixture(t *testing.T, taskID string) quotaWorkflowStoreFixture {
	t.Helper()
	db := openSchemaTestDB(t)
	task, evidence := storeTask(BillingSubscriptionPool, taskID)
	if err := NewSQLiteTaskStore(db).CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteQuotaWorkflowStore(db)
	workflow, err := store.Get(context.Background(), task.Scope, task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	return quotaWorkflowStoreFixture{store: store, task: task, initial: workflow.UpdatedAt}
}

func advanceQuotaWorkflowToSettlementPending(t *testing.T, fixture quotaWorkflowStoreFixture) time.Time {
	t.Helper()
	at := fixture.initial.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowCollecting, QuotaWorkflowFinalUploadPending, nil, nil, at)
	at = at.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowFinalUploadPending, QuotaWorkflowFinalUploading, nil, nil, at)
	at = at.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowFinalUploading, QuotaWorkflowSettlementPending, nil, nil, at)
	return at
}

func advanceQuotaWorkflowToSettlementRetry(t *testing.T, fixture quotaWorkflowStoreFixture) time.Time {
	t.Helper()
	at := advanceQuotaWorkflowToSettlementPending(t, fixture)
	stage := QuotaRetrySettlementQuery
	at = at.Add(time.Minute)
	mustUpdateQuotaWorkflow(t, fixture, QuotaWorkflowSettlementPending, QuotaWorkflowRetryPending, &stage, nil, at)
	return at
}

func mustUpdateQuotaWorkflow(
	t *testing.T,
	fixture quotaWorkflowStoreFixture,
	from, to QuotaWorkflowStatus,
	stage *QuotaRetryStage,
	lastError *string,
	at time.Time,
) {
	t.Helper()
	if err := fixture.store.Update(context.Background(), QuotaWorkflowUpdate{
		Scope: fixture.task.Scope, TaskID: fixture.task.TaskID, From: from, To: to,
		RetryStage: stage, LastErrorCode: lastError, At: at,
	}); err != nil {
		t.Fatalf("update %s -> %s: %v", from, to, err)
	}
}

func requireQuotaWorkflowStatus(
	t *testing.T,
	fixture quotaWorkflowStoreFixture,
	want QuotaWorkflowStatus,
) QuotaWorkflow {
	t.Helper()
	workflow, err := fixture.store.Get(context.Background(), fixture.task.Scope, fixture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Status != want {
		t.Fatalf("workflow status = %s, want %s", workflow.Status, want)
	}
	return workflow
}

func requireQuotaWorkflowStoreCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	if got := ErrorCodeOf(err); got != want {
		t.Fatalf("error code = %q, want %q: %v", got, want, err)
	}
}
