package modelbilling

import (
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestSubscriptionTaskCannotRunBeforeFreezeReady(t *testing.T) {
	if err := ValidateTaskTransition(BillingSubscriptionPool, TaskCreated, TaskRunning); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("created -> running error = %v, want %s", err, ErrorInvalidStateTransition)
	}
	steps := [][2]TaskState{
		{TaskCreated, TaskFreezePending},
		{TaskFreezePending, TaskReady},
		{TaskReady, TaskRunning},
		{TaskRunning, TaskUsagePending},
		{TaskUsagePending, TaskSettling},
		{TaskSettling, TaskCompleted},
	}
	for _, step := range steps {
		if err := ValidateTaskTransition(BillingSubscriptionPool, step[0], step[1]); err != nil {
			t.Fatalf("subscription %s -> %s: %v", step[0], step[1], err)
		}
	}
}

func TestBYOKTaskCannotEnterCloudOnlyStates(t *testing.T) {
	for _, state := range []TaskState{TaskFreezePending, TaskUsagePending, TaskSettling} {
		if err := ValidateTaskTransition(BillingNewAPIByok, TaskCreated, state); ErrorCodeOf(err) != ErrorInvalidStateTransition {
			t.Fatalf("BYOK created -> %s error = %v", state, err)
		}
	}
	steps := [][2]TaskState{{TaskCreated, TaskReady}, {TaskReady, TaskRunning}, {TaskRunning, TaskCompleted}}
	for _, step := range steps {
		if err := ValidateTaskTransition(BillingNewAPIByok, step[0], step[1]); err != nil {
			t.Fatalf("BYOK %s -> %s: %v", step[0], step[1], err)
		}
	}
}

func TestCallAndReportTransitionsRejectIllegalJumps(t *testing.T) {
	if err := ValidateCallTransition(CallPrepared, CallCompleted); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("prepared -> completed error = %v", err)
	}
	if err := ValidateCallTransition(CallPrepared, CallRunning); err != nil {
		t.Fatalf("prepared -> running: %v", err)
	}
	if err := ValidateReportTransition(ReportPending, ReportAccepted); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("pending -> accepted error = %v", err)
	}
	if err := ValidateReportTransition(ReportPending, ReportDelivering); err != nil {
		t.Fatalf("pending -> delivering: %v", err)
	}
}

func TestSubscriptionFailedOrCancelledTaskCanContinueFinalization(t *testing.T) {
	for _, state := range []TaskState{TaskFailed, TaskCancelled} {
		if err := ValidateTaskTransition(BillingSubscriptionPool, state, TaskUsagePending); err != nil {
			t.Fatalf("subscription %s -> usage_pending: %v", state, err)
		}
	}
}

func TestSubscriptionTerminalValidationMatchesFreezeLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	for _, state := range []TaskState{TaskFailed, TaskCancelled} {
		result := TaskResultFailed
		if state == TaskCancelled {
			result = TaskResultCancelled
		}
		task := ModelTask{
			TaskID: "task_terminal", Scope: testTaskScope(BillingSubscriptionPool),
			BillingSource: BillingSubscriptionPool, PricingEvidenceID: "price_01",
			InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0",
			State: state, Result: &result, CreatedAt: now, UpdatedAt: now,
		}
		if err := task.Validate(); err != nil {
			t.Fatalf("created -> %s without freeze request: %v", state, err)
		}
	}

	freezePending := ModelTask{
		TaskID: "task_pending", Scope: testTaskScope(BillingSubscriptionPool),
		BillingSource: BillingSubscriptionPool, PricingEvidenceID: "price_01",
		State: TaskFreezePending, CreatedAt: now, UpdatedAt: now,
	}
	if err := freezePending.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("freeze_pending without request error = %v, want %s", err, ErrorContractInvalid)
	}

	task := ModelTask{
		TaskID: "task_bad_freeze", Scope: testTaskScope(BillingSubscriptionPool),
		BillingSource: BillingSubscriptionPool, PricingEvidenceID: "price_01",
		InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0",
		State: TaskFailed, Result: taskResultPointer(TaskResultFailed), CreatedAt: now, UpdatedAt: now,
	}
	other := testBindingScope()
	other.BindingGeneration++
	task.FreezeRequest = &cloud.FreezeRequest{
		Scope: other, TaskID: task.TaskID, ModelID: "model_01", ClientTaskType: "chat",
		ClientEstimatedCost: "0", IdempotencyKey: "freeze_key",
	}
	if err := task.Validate(); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("terminal mismatched freeze request error = %v, want %s", err, ErrorScopeMismatch)
	}

	binding := testBindingScope()
	terminalWithFreeze := ModelTask{
		TaskID: "task_with_freeze", Scope: testTaskScope(BillingSubscriptionPool),
		BillingSource: BillingSubscriptionPool, PricingEvidenceID: "price_01",
		InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0",
		State: TaskCancelled, Result: taskResultPointer(TaskResultCancelled), CreatedAt: now, UpdatedAt: now,
		FreezeRequest: &cloud.FreezeRequest{
			Scope: binding, TaskID: "task_with_freeze", ModelID: "model_01",
			ClientTaskType: "chat", ClientEstimatedCost: "0", IdempotencyKey: "freeze_key",
		},
		Freeze: &cloud.Freeze{
			Scope: binding, FreezeID: "frz_01", TaskID: "task_with_freeze", Status: cloud.FreezeExpiredReleased,
			FreezeAmount: "0", FreeFrozenAmount: "0", SubscriptionFrozenAmount: "0", ExpiresAt: now.Add(time.Hour),
		},
	}
	if err := terminalWithFreeze.Validate(); err != nil {
		t.Fatalf("cancelled task with consistent freeze: %v", err)
	}
	terminalWithFreeze.Freeze.TaskID = "other_task"
	if err := terminalWithFreeze.Validate(); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("terminal mismatched freeze error = %v, want %s", err, ErrorScopeMismatch)
	}
}

func TestTaskResultPersistsAcrossSubscriptionAndBYOKRecovery(t *testing.T) {
	for _, result := range []TaskResult{TaskResultCompleted, TaskResultFailed, TaskResultCancelled} {
		if err := ValidateTaskResultTransition(nil, taskResultPointer(result)); err != nil {
			t.Fatalf("set terminal result %s: %v", result, err)
		}
		if err := ValidateTaskResultTransition(taskResultPointer(result), taskResultPointer(result)); err != nil {
			t.Fatalf("preserve result %s: %v", result, err)
		}
		if err := ValidateTaskResultTransition(taskResultPointer(result), taskResultPointer(TaskResultCompleted)); result != TaskResultCompleted && ErrorCodeOf(err) != ErrorInvalidStateTransition {
			t.Fatalf("overwrite result %s error = %v", result, err)
		}
	}
	if err := ValidateTaskUpdate(
		BillingSubscriptionPool, TaskFailed, TaskUsagePending,
		taskResultPointer(TaskResultFailed), taskResultPointer(TaskResultFailed),
	); err != nil {
		t.Fatalf("subscription failed result recovery: %v", err)
	}
	if err := ValidateTaskUpdate(
		BillingNewAPIByok, TaskRunning, TaskCompleted, nil, taskResultPointer(TaskResultCompleted),
	); err != nil {
		t.Fatalf("BYOK completed result recovery: %v", err)
	}

	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	byok := ModelTask{
		TaskID: "task_byok", Scope: testTaskScope(BillingNewAPIByok), BillingSource: BillingNewAPIByok,
		PricingEvidenceID: "price_01", State: TaskCompleted, Result: taskResultPointer(TaskResultCompleted),
		InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := byok.Validate(); err != nil {
		t.Fatalf("completed BYOK task: %v", err)
	}
	byok.Result = taskResultPointer(TaskResultFailed)
	if err := byok.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("BYOK completed with failed result error = %v", err)
	}
}

func TestBYOKRecoveryRestoresOnlyMatchingPersistedResult(t *testing.T) {
	completed := taskResultPointer(TaskResultCompleted)
	if err := ValidateTaskUpdate(BillingNewAPIByok, TaskRunning, TaskRecoveryRequired, nil, completed); err != nil {
		t.Fatalf("running -> recovery_required(completed): %v", err)
	}
	if err := ValidateTaskUpdate(BillingNewAPIByok, TaskRecoveryRequired, TaskCompleted, completed, completed); err != nil {
		t.Fatalf("recovery_required(completed) -> completed: %v", err)
	}
	for _, result := range []TaskResult{TaskResultFailed, TaskResultCancelled} {
		terminal := TaskFailed
		if result == TaskResultCancelled {
			terminal = TaskCancelled
		}
		persisted := taskResultPointer(result)
		if err := ValidateTaskUpdate(BillingNewAPIByok, TaskRunning, TaskRecoveryRequired, nil, persisted); err != nil {
			t.Fatalf("running -> recovery_required(%s): %v", result, err)
		}
		if err := ValidateTaskUpdate(BillingNewAPIByok, TaskRecoveryRequired, terminal, persisted, persisted); err != nil {
			t.Fatalf("recovery_required(%s) -> %s: %v", result, terminal, err)
		}
		wrongTerminal := TaskCancelled
		if terminal == TaskCancelled {
			wrongTerminal = TaskFailed
		}
		if err := ValidateTaskUpdate(BillingNewAPIByok, TaskRecoveryRequired, wrongTerminal, persisted, persisted); ErrorCodeOf(err) != ErrorContractInvalid {
			t.Fatalf("recovery result %s to wrong terminal error = %v", result, err)
		}
	}
	if err := ValidateTaskUpdate(BillingNewAPIByok, TaskRecoveryRequired, TaskCompleted, completed, nil); ErrorCodeOf(err) != ErrorInvalidStateTransition {
		t.Fatalf("cleared recovery result error = %v", err)
	}
}

func TestFinalUsageReportMatchesPersistedSubscriptionTaskResult(t *testing.T) {
	now := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	binding := testBindingScope()
	task := ModelTask{
		TaskID: "task_01", Scope: testTaskScope(BillingSubscriptionPool), BillingSource: BillingSubscriptionPool,
		PricingEvidenceID: "price_01", State: TaskFailed, Result: taskResultPointer(TaskResultFailed),
		InitialModelID: "model_01", ClientTaskType: "chat", ClientEstimatedCost: "0",
		CreatedAt: now, UpdatedAt: now,
		FreezeRequest: &cloud.FreezeRequest{
			Scope: binding, TaskID: "task_01", ModelID: "model_01", ClientTaskType: "chat",
			ClientEstimatedCost: "0", IdempotencyKey: "freeze_key",
		},
		Freeze: &cloud.Freeze{
			Scope: binding, FreezeID: "frz_01", TaskID: "task_01", Status: cloud.FreezeActive,
			FreezeAmount: "0", FreeFrozenAmount: "0", SubscriptionFrozenAmount: "0", ExpiresAt: now.Add(time.Hour),
		},
		Entitlement: &TaskEntitlementSnapshot{Plan: "starter", CapturedAt: now},
	}
	report := validFinalReport(t, TaskResultFailed)
	if err := report.ValidateForTask(task); err != nil {
		t.Fatalf("matching final report: %v", err)
	}
	for _, state := range []TaskState{TaskUsagePending, TaskSettling, TaskCompleted} {
		persisted := task
		persisted.State = state
		if err := persisted.Validate(); err != nil {
			t.Fatalf("subscription %s did not preserve failed result: %v", state, err)
		}
	}
	report.TaskResult = TaskResultCancelled
	if err := report.ValidateForTask(task); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("mismatched final result error = %v", err)
	}
	byok := task
	byok.Scope = testTaskScope(BillingNewAPIByok)
	byok.BillingSource = BillingNewAPIByok
	byok.FreezeRequest, byok.Freeze, byok.Entitlement = nil, nil, nil
	if err := report.ValidateForTask(byok); ErrorCodeOf(err) != ErrorBillingSourceMismatch {
		t.Fatalf("BYOK final report error = %v", err)
	}
}

func taskResultPointer(result TaskResult) *TaskResult {
	return &result
}
