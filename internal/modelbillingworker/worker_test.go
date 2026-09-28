package modelbillingworker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/modelbilling"
	"kwu/internal/userctx"
)

var workerTestNow = time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

type retryMark struct {
	key     string
	attempt int
	next    time.Time
	code    string
}

type scriptedReportStore struct {
	reports            []modelbilling.PendingReport
	claimResults       []coordinatorDelivery
	listErr            error
	markAcceptedErrors []error
	markRetryErr       error
	markBlockedErr     error
	claimCalls         int
	accepted           int
	retried            int
	blocked            int
	retryMarks         []retryMark
	correctionErr      error
	corrections        int
	correctionReason   modelbilling.ErrorCode
}

func (s *scriptedReportStore) EnqueueFinal(context.Context, modelbilling.TaskScope, string) (modelbilling.UsageReportBuildResult, *modelbilling.PendingReport, error) {
	return modelbilling.UsageReportBuildResult{}, nil, nil
}

func (s *scriptedReportStore) EnqueuePartial(context.Context, modelbilling.TaskScope, string) (modelbilling.UsageReportBuildResult, *modelbilling.PendingReport, error) {
	return modelbilling.UsageReportBuildResult{}, nil, nil
}

func (s *scriptedReportStore) GetByIdempotencyKey(_ context.Context, _ modelbilling.TaskScope, key string) (modelbilling.PendingReport, bool, error) {
	for _, report := range s.reports {
		if report.IdempotencyKey == key {
			return report, true, nil
		}
	}
	return modelbilling.PendingReport{}, false, nil
}

func (s *scriptedReportStore) ListByTask(_ context.Context, _ modelbilling.TaskScope, taskID string) ([]modelbilling.PendingReport, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	var reports []modelbilling.PendingReport
	for _, report := range s.reports {
		if report.TaskID == taskID {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (s *scriptedReportStore) ClaimDue(_ context.Context, _ modelbilling.TaskScope, now, lease time.Time) (modelbilling.PendingReport, bool, error) {
	s.claimCalls++
	if len(s.claimResults) != 0 {
		result := s.claimResults[0]
		s.claimResults = s.claimResults[1:]
		return modelbilling.PendingReport{}, result.found, result.err
	}
	for index := range s.reports {
		report := &s.reports[index]
		if report.State != modelbilling.ReportPending && report.State != modelbilling.ReportRetry && report.State != modelbilling.ReportDelivering {
			continue
		}
		if report.NextAttemptAt == nil || report.NextAttemptAt.After(now) {
			continue
		}
		report.State = modelbilling.ReportDelivering
		report.AttemptCount++
		leaseCopy := lease.UTC()
		report.NextAttemptAt = &leaseCopy
		report.UpdatedAt = now.UTC()
		return *report, true, nil
	}
	return modelbilling.PendingReport{}, false, nil
}

func (s *scriptedReportStore) MarkAccepted(_ context.Context, _ modelbilling.TaskScope, key string, attempt int, at time.Time) error {
	if len(s.markAcceptedErrors) != 0 {
		err := s.markAcceptedErrors[0]
		s.markAcceptedErrors = s.markAcceptedErrors[1:]
		if err != nil {
			return err
		}
	}
	if err := s.mark(key, attempt, modelbilling.ReportAccepted, "", nil, at); err != nil {
		return err
	}
	s.accepted++
	return nil
}

func (s *scriptedReportStore) MarkRetry(_ context.Context, _ modelbilling.TaskScope, key string, attempt int, next time.Time, code string, at time.Time) error {
	if s.markRetryErr != nil {
		return s.markRetryErr
	}
	if err := s.mark(key, attempt, modelbilling.ReportRetry, code, &next, at); err != nil {
		return err
	}
	s.retried++
	s.retryMarks = append(s.retryMarks, retryMark{key: key, attempt: attempt, next: next, code: code})
	return nil
}

func (s *scriptedReportStore) MarkBlocked(_ context.Context, _ modelbilling.TaskScope, key string, attempt int, code string, at time.Time) error {
	if s.markBlockedErr != nil {
		return s.markBlockedErr
	}
	if err := s.mark(key, attempt, modelbilling.ReportBlocked, code, nil, at); err != nil {
		return err
	}
	s.blocked++
	return nil
}

func (s *scriptedReportStore) mark(key string, attempt int, state modelbilling.ReportState, code string, next *time.Time, at time.Time) error {
	for index := range s.reports {
		report := &s.reports[index]
		if report.IdempotencyKey != key {
			continue
		}
		if report.State != modelbilling.ReportDelivering || report.AttemptCount != attempt {
			return &modelbilling.Error{Code: modelbilling.ErrorInvalidStateTransition, Message: "stale delivery attempt"}
		}
		report.State = state
		report.NextAttemptAt = next
		report.UpdatedAt = at.UTC()
		if code == "" {
			report.LastErrorCode = nil
		} else {
			codeCopy := code
			report.LastErrorCode = &codeCopy
		}
		return nil
	}
	return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "report not found"}
}

func (s *scriptedReportStore) CorrectUnacceptedFinal(
	_ context.Context,
	_ modelbilling.TaskScope,
	key string,
	attempt int,
	reason modelbilling.ErrorCode,
	at time.Time,
) (modelbilling.PendingReport, error) {
	if s.correctionErr != nil {
		return modelbilling.PendingReport{}, s.correctionErr
	}
	for index := range s.reports {
		report := &s.reports[index]
		if report.IdempotencyKey != key {
			continue
		}
		if report.Kind != modelbilling.ReportFinal || report.State != modelbilling.ReportDelivering || report.AttemptCount != attempt {
			return modelbilling.PendingReport{}, &modelbilling.Error{
				Code: modelbilling.ErrorInvalidStateTransition, Message: "stale final correction",
			}
		}
		next := at.UTC()
		report.State, report.NextAttemptAt, report.LastErrorCode, report.UpdatedAt =
			modelbilling.ReportPending, &next, nil, next
		s.corrections++
		s.correctionReason = reason
		return *report, nil
	}
	return modelbilling.PendingReport{}, &modelbilling.Error{
		Code: modelbilling.ErrorContractInvalid, Message: "report not found",
	}
}

type scriptedTaskStore struct {
	tasks       map[string]modelbilling.ModelTask
	updates     []modelbilling.TaskStateUpdate
	getErrors   map[string][]error
	updateError func(modelbilling.TaskStateUpdate) error
	listErr     error
	getCalls    int
	events      *[]string
}

func (s *scriptedTaskStore) Get(_ context.Context, scope modelbilling.TaskScope, taskID string) (modelbilling.ModelTask, error) {
	s.getCalls++
	if queued := s.getErrors[taskID]; len(queued) != 0 {
		err := queued[0]
		s.getErrors[taskID] = queued[1:]
		return modelbilling.ModelTask{}, err
	}
	task, ok := s.tasks[taskID]
	if !ok {
		return modelbilling.ModelTask{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "task not found"}
	}
	if !task.Scope.Equal(scope) {
		return modelbilling.ModelTask{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "task scope mismatch"}
	}
	return task, nil
}

func (s *scriptedTaskStore) UpdateState(_ context.Context, update modelbilling.TaskStateUpdate) error {
	if s.updateError != nil {
		if err := s.updateError(update); err != nil {
			return err
		}
	}
	task, ok := s.tasks[update.TaskID]
	if !ok {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "task not found"}
	}
	if task.State != update.From {
		return &modelbilling.Error{Code: modelbilling.ErrorInvalidStateTransition, Message: "stale task state"}
	}
	task.State = update.To
	task.Result = update.Result
	task.LastErrorCode = update.LastErrorCode
	task.UpdatedAt = update.At.UTC()
	s.tasks[update.TaskID] = task
	s.updates = append(s.updates, update)
	emit(s.events, "task")
	return nil
}

func (s *scriptedTaskStore) ListSettling(_ context.Context, scope modelbilling.TaskScope) ([]modelbilling.ModelTask, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	var tasks []modelbilling.ModelTask
	for _, task := range s.tasks {
		if task.State == modelbilling.TaskSettling && task.Scope.Equal(scope) {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

type cloudResult struct {
	result modelbilling.QuotaSettlementResult
	err    error
}

type recordingCloud struct {
	t               *testing.T
	delegate        modelbilling.CloudQuotaClient
	uploadErrors    []error
	finalizeResults []cloudResult
	getResults      []cloudResult
	partials        []modelbilling.PartialQuotaReport
	finals          []modelbilling.FinalQuotaReport
	finalPayloads   [][]byte
	queries         []string
	uploads         int
	finalizes       int
	gets            int
}

func (c *recordingCloud) UploadPartialQuota(ctx context.Context, scope cloud.BindingScope, report modelbilling.PartialQuotaReport) error {
	c.uploads++
	c.partials = append(c.partials, report)
	if c.delegate != nil {
		return c.delegate.UploadPartialQuota(ctx, scope, report)
	}
	if len(c.uploadErrors) == 0 {
		return nil
	}
	err := c.uploadErrors[0]
	c.uploadErrors = c.uploadErrors[1:]
	return err
}

func (c *recordingCloud) FinalizeQuota(ctx context.Context, scope cloud.BindingScope, report modelbilling.FinalQuotaReport) (modelbilling.QuotaSettlementResult, error) {
	c.finalizes++
	c.finals = append(c.finals, report)
	payload, err := modelbilling.CanonicalJSON(report)
	if err != nil {
		return modelbilling.QuotaSettlementResult{}, err
	}
	c.finalPayloads = append(c.finalPayloads, append([]byte(nil), payload...))
	if c.delegate != nil {
		return c.delegate.FinalizeQuota(ctx, scope, report)
	}
	if len(c.finalizeResults) == 0 {
		return modelbilling.QuotaSettlementResult{}, errors.New("unexpected FinalizeQuota call")
	}
	result := c.finalizeResults[0]
	c.finalizeResults = c.finalizeResults[1:]
	return result.result, result.err
}

func (c *recordingCloud) GetQuotaSettlement(ctx context.Context, scope cloud.BindingScope, taskID string) (modelbilling.QuotaSettlementResult, error) {
	c.gets++
	c.queries = append(c.queries, taskID)
	if c.delegate != nil {
		return c.delegate.GetQuotaSettlement(ctx, scope, taskID)
	}
	if len(c.getResults) == 0 {
		return modelbilling.QuotaSettlementResult{}, errors.New("unexpected GetQuotaSettlement call")
	}
	result := c.getResults[0]
	c.getResults = c.getResults[1:]
	return result.result, result.err
}

type observerKey struct {
	scope        cloud.BindingScope
	taskID       string
	settlementID string
}

type observerSpy struct {
	keys     []observerKey
	errors   []error
	attempts int
	events   *[]string
}

func (s *observerSpy) CreditSettlementObserved(_ context.Context, scope cloud.BindingScope, taskID, settlementID string) error {
	s.attempts++
	s.keys = append(s.keys, observerKey{scope: scope, taskID: taskID, settlementID: settlementID})
	emit(s.events, "observer")
	if len(s.errors) == 0 {
		return nil
	}
	err := s.errors[0]
	s.errors = s.errors[1:]
	return err
}

type scriptedWorkflowStore struct {
	workflows         map[string]modelbilling.QuotaWorkflow
	updates           []modelbilling.QuotaWorkflowUpdate
	getErr            error
	updateErr         error
	settlements       map[string]modelbilling.QuotaSettlementResult
	settlementSaveErr error
	settlementSaves   int
	events            *[]string
}

func (s *scriptedWorkflowStore) Get(_ context.Context, scope modelbilling.TaskScope, taskID string) (modelbilling.QuotaWorkflow, error) {
	if s.getErr != nil {
		return modelbilling.QuotaWorkflow{}, s.getErr
	}
	workflow, ok := s.workflows[taskID]
	if !ok {
		return modelbilling.QuotaWorkflow{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "workflow not found"}
	}
	if !workflow.Scope.Equal(scope) {
		return modelbilling.QuotaWorkflow{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "workflow scope mismatch"}
	}
	return workflow, nil
}

func (s *scriptedWorkflowStore) Update(_ context.Context, update modelbilling.QuotaWorkflowUpdate) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	workflow, ok := s.workflows[update.TaskID]
	if !ok || workflow.Status != update.From {
		return &modelbilling.Error{Code: modelbilling.ErrorInvalidStateTransition, Message: "stale workflow state"}
	}
	workflow.Status = update.To
	workflow.RetryStage = cloneRetryStageForTest(update.RetryStage)
	workflow.LastErrorCode = cloneStringForTest(update.LastErrorCode)
	workflow.UpdatedAt = update.At.UTC()
	s.workflows[update.TaskID] = workflow
	s.updates = append(s.updates, update)
	emit(s.events, "workflow")
	return nil
}

func (s *scriptedWorkflowStore) SaveQuotaSettlement(
	_ context.Context,
	_ modelbilling.TaskScope,
	result modelbilling.QuotaSettlementResult,
	_ time.Time,
) error {
	if s.settlementSaveErr != nil {
		return s.settlementSaveErr
	}
	if s.settlements == nil {
		s.settlements = make(map[string]modelbilling.QuotaSettlementResult)
	}
	s.settlements[result.Settlement.TaskID] = result
	s.settlementSaves++
	emit(s.events, "settlement")
	return nil
}

func (s *scriptedWorkflowStore) CurrentQuotaSettlement(
	_ context.Context,
	_ modelbilling.TaskScope,
	taskID string,
) (modelbilling.QuotaSettlementResult, bool, error) {
	result, found := s.settlements[taskID]
	return result, found, nil
}

func emit(events *[]string, event string) {
	if events != nil {
		*events = append(*events, event)
	}
}

func testRetryConfig(now *time.Time) RetryConfig {
	return RetryConfig{
		BaseDelay: 10 * time.Second, MaxDelay: time.Minute, JitterFraction: 0.5,
		LeaseDuration: 30 * time.Second, Clock: func() time.Time { return *now },
		Jitter: func(time.Duration) time.Duration { return 0 },
	}
}

func newWorkerForTest(t *testing.T, reports *scriptedReportStore, tasks *scriptedTaskStore, client *recordingCloud, observer *observerSpy, now *time.Time) *Worker {
	t.Helper()
	worker, err := New(reports, tasks, client, observer, testRetryConfig(now))
	if err != nil {
		t.Fatal(err)
	}
	return worker
}

func newWorkerWithWorkflowForTest(
	t *testing.T,
	reports modelbilling.ReportStore,
	tasks SettlementTaskStore,
	client modelbilling.CloudQuotaClient,
	observer modelbilling.SettlementObserver,
	workflow QuotaWorkflowStore,
	now *time.Time,
) *Worker {
	t.Helper()
	worker, err := New(reports, tasks, client, observer, testRetryConfig(now), workflow)
	if err != nil {
		t.Fatal(err)
	}
	return worker
}

func workerTestScope() modelbilling.TaskScope {
	return workerScope(cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud-worker", BindingGeneration: 7})
}

func workerScope(binding cloud.BindingScope) modelbilling.TaskScope {
	return modelbilling.TaskScope{ActorID: "actor-worker", ProfileID: binding.ProfileID, CloudBinding: &binding}
}

func workerTask(taskID, freezeID string, scope modelbilling.TaskScope, state modelbilling.TaskState, result modelbilling.TaskResult) modelbilling.ModelTask {
	created := workerTestNow.Add(-10 * time.Minute)
	started := created.Add(time.Minute)
	ended := workerTestNow.Add(-time.Minute)
	return modelbilling.ModelTask{
		TaskID: taskID, Scope: scope, BillingSource: modelbilling.BillingSubscriptionPool,
		InitialModelID: "model-worker", ClientTaskType: "chat", ClientEstimatedCost: "10",
		FreezeRequest: &cloud.FreezeRequest{
			Scope: *scope.CloudBinding, TaskID: taskID, ModelID: "model-worker", ClientTaskType: "chat",
			ClientEstimatedCost: "10", IdempotencyKey: "freeze-request-" + taskID,
		},
		Freeze: &cloud.Freeze{
			Scope: *scope.CloudBinding, FreezeID: freezeID, TaskID: taskID, Status: cloud.FreezeActive,
			FreezeAmount: "10", FreeFrozenAmount: "4", SubscriptionFrozenAmount: "6",
			ExpiresAt: workerTestNow.Add(time.Hour),
		},
		Entitlement:       &modelbilling.TaskEntitlementSnapshot{Plan: "pro", CapturedAt: created.Add(2 * time.Minute)},
		PricingEvidenceID: "price-" + taskID, State: state, Result: &result,
		CreatedAt: created, StartedAt: &started, UpdatedAt: ended, EndedAt: &ended,
	}
}

func completeQuotaCall(callID string, occurredAt time.Time) modelbilling.QuotaReportCall {
	requestID := "oneapi-" + callID
	quota := modelbilling.DecimalString("1")
	source := modelbilling.QuotaSourceBilling
	return modelbilling.QuotaReportCall{
		CallID: callID, Endpoint: modelbilling.MaasEndpointResponses, Model: "model-worker",
		OneAPIRequestID: &requestID,
		BillingQuotaRaw: modelbilling.QuotaRawValue{Present: true, Type: modelbilling.QuotaJSONNumber, Value: json.RawMessage("1")},
		UsageQuotaRaw:   modelbilling.QuotaRawValue{}, Quota: &quota, QuotaSource: &source,
		EvidenceStatus: modelbilling.QuotaEvidenceComplete, EvidenceIssues: []modelbilling.QuotaEvidenceIssue{},
		EvidenceRevision: 1, OccurredAt: occurredAt.UTC(),
	}
}

func incompleteQuotaCall(callID string, occurredAt time.Time) modelbilling.QuotaReportCall {
	return modelbilling.QuotaReportCall{
		CallID: callID, Endpoint: modelbilling.MaasEndpointChatCompletions, Model: "model-worker",
		BillingQuotaRaw: modelbilling.QuotaRawValue{}, UsageQuotaRaw: modelbilling.QuotaRawValue{},
		EvidenceStatus:   modelbilling.QuotaEvidenceMissing,
		EvidenceIssues:   []modelbilling.QuotaEvidenceIssue{modelbilling.QuotaIssueRequestIDMissing, modelbilling.QuotaIssueMissing},
		EvidenceRevision: 1, OccurredAt: occurredAt.UTC(),
	}
}

func newPartialQuotaReport(
	t *testing.T,
	scope modelbilling.TaskScope,
	taskID, freezeID string,
	sequence uint64,
	incomplete bool,
) modelbilling.PartialQuotaReport {
	t.Helper()
	captured := workerTestNow.Add(-2 * time.Minute)
	call := completeQuotaCall("call-"+taskID, captured.Add(-time.Minute))
	if incomplete {
		call = incompleteQuotaCall("call-"+taskID, captured.Add(-time.Minute))
	}
	key, err := modelbilling.BuildPartialIdempotencyKey(*scope.CloudBinding, taskID, freezeID, sequence)
	if err != nil {
		t.Fatal(err)
	}
	report := modelbilling.PartialQuotaReport{
		ContractVersion: modelbilling.QuotaContractV1, TaskID: taskID, FreezeID: freezeID,
		Sequence: sequence, ReportMode: modelbilling.ReportModeCumulative, CapturedAt: captured,
		Calls: []modelbilling.QuotaReportCall{call}, Scope: *scope.CloudBinding, IdempotencyKey: key,
	}
	if err := report.Validate(); err != nil {
		t.Fatal(err)
	}
	return report
}

func newFinalQuotaReport(t *testing.T, task modelbilling.ModelTask) modelbilling.FinalQuotaReport {
	t.Helper()
	if task.EndedAt == nil || task.Result == nil || task.Scope.CloudBinding == nil || task.Freeze == nil {
		t.Fatal("final quota task fixture is incomplete")
	}
	key, err := modelbilling.BuildFinalIdempotencyKey(*task.Scope.CloudBinding, task.TaskID, task.Freeze.FreezeID)
	if err != nil {
		t.Fatal(err)
	}
	call := completeQuotaCall("call-"+task.TaskID, task.EndedAt.Add(-time.Minute))
	report := modelbilling.FinalQuotaReport{
		ContractVersion: modelbilling.QuotaContractV1, TaskID: task.TaskID, FreezeID: task.Freeze.FreezeID,
		ClientSummary: modelbilling.QuotaTaskSummary{
			TaskTotalQuota: "1", ModelCallCount: 1, StartedAt: *task.StartedAt, FinishedAt: *task.EndedAt,
		},
		Calls: []modelbilling.QuotaReportCall{call}, Scope: *task.Scope.CloudBinding,
		IdempotencyKey: key, TaskResult: *task.Result, CreatedAt: *task.EndedAt,
	}
	if err := report.ValidateForTask(task); err != nil {
		t.Fatal(err)
	}
	return report
}

func pendingPartial(t *testing.T, report modelbilling.PartialQuotaReport) modelbilling.PendingReport {
	t.Helper()
	payload, err := modelbilling.CanonicalJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := modelbilling.QuotaPayloadHash(payload)
	if err != nil {
		t.Fatal(err)
	}
	next := report.CapturedAt
	return modelbilling.PendingReport{
		Scope: report.Scope, TaskID: report.TaskID, FreezeID: report.FreezeID,
		ContractVersion: modelbilling.QuotaContractV1, PayloadHash: hash,
		Kind: modelbilling.ReportPartial, Sequence: report.Sequence, IdempotencyKey: report.IdempotencyKey,
		CanonicalPayload: payload, State: modelbilling.ReportPending, NextAttemptAt: &next,
		CreatedAt: report.CapturedAt, UpdatedAt: report.CapturedAt,
	}
}

func pendingFinal(t *testing.T, report modelbilling.FinalQuotaReport) modelbilling.PendingReport {
	t.Helper()
	payload, err := modelbilling.CanonicalJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := modelbilling.QuotaPayloadHash(payload)
	if err != nil {
		t.Fatal(err)
	}
	next := report.CreatedAt
	return modelbilling.PendingReport{
		Scope: report.Scope, TaskID: report.TaskID, FreezeID: report.FreezeID,
		ContractVersion: modelbilling.QuotaContractV1, PayloadHash: hash,
		Kind: modelbilling.ReportFinal, IdempotencyKey: report.IdempotencyKey,
		CanonicalPayload: payload, State: modelbilling.ReportPending, NextAttemptAt: &next,
		CreatedAt: report.CreatedAt, UpdatedAt: report.CreatedAt,
	}
}

func withReportState(report modelbilling.PendingReport, state modelbilling.ReportState) modelbilling.PendingReport {
	report.State = state
	if state == modelbilling.ReportAccepted || state == modelbilling.ReportBlocked {
		report.NextAttemptAt = nil
		if report.AttemptCount == 0 {
			report.AttemptCount = 1
		}
	}
	return report
}

func quotaSettlementResult(taskID string, status modelbilling.QuotaSettlementState) modelbilling.QuotaSettlementResult {
	settlement := modelbilling.QuotaSettlement{SettlementID: "settlement-" + taskID, TaskID: taskID, Status: status}
	switch status {
	case modelbilling.QuotaSettlementPending:
		settlement.TaskTotalQuota = decimalPointer("1")
		return modelbilling.QuotaSettlementResult{Settlement: settlement}
	case modelbilling.QuotaSettlementManualRequired:
		code := "MANUAL_REVIEW_REQUIRED"
		settlement.ErrorCode = &code
		return modelbilling.QuotaSettlementResult{Settlement: settlement}
	case modelbilling.QuotaSettlementSettled, modelbilling.QuotaSettlementSettledUncovered:
		policyVersion := "maas-quota-config-v1"
		settlement.TaskTotalQuota = decimalPointer("1")
		settlement.QuotaPolicyVersion = &policyVersion
		settlement.QuotaPerUnit = decimalPointer("500000")
		settlement.USDExchangeRate = decimalPointer("7.3")
		settlement.CreditPerCNY = decimalPointer("2000")
		settlement.AdditionalChargedAmount = decimalPointer("0")
		settlement.AdditionalChargedFreeAmount = decimalPointer("0")
		settlement.AdditionalChargedSubscriptionAmount = decimalPointer("0")
		balance := "20"
		if status == modelbilling.QuotaSettlementSettled {
			settlement.ChargedCredits = decimalPointer("3")
			settlement.ConsumedFrozenAmount = decimalPointer("3")
			settlement.ConsumedFrozenFreeAmount = decimalPointer("3")
			settlement.ConsumedFrozenSubscriptionAmount = decimalPointer("0")
			settlement.ReleasedFrozenAmount = decimalPointer("7")
			settlement.ReleasedFrozenFreeAmount = decimalPointer("1")
			settlement.ReleasedFrozenSubscriptionAmount = decimalPointer("6")
			settlement.UncoveredAmount = decimalPointer("0")
		} else {
			balance = "0"
			settlement.ChargedCredits = decimalPointer("12")
			settlement.ConsumedFrozenAmount = decimalPointer("10")
			settlement.ConsumedFrozenFreeAmount = decimalPointer("4")
			settlement.ConsumedFrozenSubscriptionAmount = decimalPointer("6")
			settlement.ReleasedFrozenAmount = decimalPointer("0")
			settlement.ReleasedFrozenFreeAmount = decimalPointer("0")
			settlement.ReleasedFrozenSubscriptionAmount = decimalPointer("0")
			settlement.UncoveredAmount = decimalPointer("2")
		}
		version := uint64(2)
		settlement.Balance = decimalPointer(balance)
		settlement.BalanceVersion = &version
		return modelbilling.QuotaSettlementResult{
			Settlement: settlement,
			Balance: &cloud.BalanceSnapshot{
				TotalBalance: balance, FreeBalance: "0", SubscriptionBalance: "0", AvailableBalance: "0",
				FreeAvailableBalance: "0", SubscriptionAvailableBalance: "0", FrozenBalance: "0",
				FreeFrozenBalance: "0", SubscriptionFrozenBalance: "0", Version: version,
			},
		}
	default:
		return modelbilling.QuotaSettlementResult{Settlement: settlement}
	}
}

func decimalPointer(value string) *modelbilling.DecimalString {
	decimal := modelbilling.DecimalString(value)
	return &decimal
}

func timePointer(value time.Time) *time.Time { return &value }

func stringPointer(value string) *string { return &value }

func cloneRetryStageForTest(value *modelbilling.QuotaRetryStage) *modelbilling.QuotaRetryStage {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneStringForTest(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func newWorkflowStore(task modelbilling.ModelTask, status modelbilling.QuotaWorkflowStatus) *scriptedWorkflowStore {
	return &scriptedWorkflowStore{workflows: map[string]modelbilling.QuotaWorkflow{
		task.TaskID: {TaskID: task.TaskID, Scope: task.Scope, Status: status, UpdatedAt: workerTestNow},
	}}
}

func acceptedFinalStore(t *testing.T, task modelbilling.ModelTask) *scriptedReportStore {
	t.Helper()
	accepted := withReportState(pendingFinal(t, newFinalQuotaReport(t, task)), modelbilling.ReportAccepted)
	return &scriptedReportStore{reports: []modelbilling.PendingReport{accepted}}
}

func assertTaskFacts(t *testing.T, got, before modelbilling.ModelTask) {
	t.Helper()
	if !reflect.DeepEqual(got, before) {
		t.Fatalf("task changed unexpectedly:\n got: %+v\nwant: %+v", got, before)
	}
}

func assertErrorCode(t *testing.T, err error, code modelbilling.ErrorCode) {
	t.Helper()
	if modelbilling.ErrorCodeOf(err) != code {
		t.Fatalf("error = %v, code = %q, want %q", err, modelbilling.ErrorCodeOf(err), code)
	}
}

func TestWorkerRejectsInvalidConfigurationAndBYOKScope(t *testing.T) {
	now := workerTestNow
	reports := &scriptedReportStore{}
	tasks := &scriptedTaskStore{}
	client := &recordingCloud{t: t}
	observer := &observerSpy{}
	valid := testRetryConfig(&now)

	invalid := []RetryConfig{
		func() RetryConfig { value := valid; value.BaseDelay = 0; return value }(),
		func() RetryConfig { value := valid; value.MaxDelay = time.Second; return value }(),
		func() RetryConfig { value := valid; value.LeaseDuration = 0; return value }(),
		func() RetryConfig { value := valid; value.JitterFraction = math.NaN(); return value }(),
		func() RetryConfig { value := valid; value.JitterFraction = 1.1; return value }(),
		func() RetryConfig { value := valid; value.Clock = nil; return value }(),
		func() RetryConfig { value := valid; value.Jitter = nil; return value }(),
	}
	for index, retry := range invalid {
		if _, err := New(reports, tasks, client, observer, retry); err == nil {
			t.Fatalf("invalid config %d accepted", index)
		}
	}
	workflow := &scriptedWorkflowStore{}
	if _, err := New(reports, tasks, client, observer, valid, workflow, workflow); err == nil {
		t.Fatal("multiple workflow stores accepted")
	}
	if _, err := New(reports, tasks, client, observer, valid, nil); err == nil {
		t.Fatal("nil workflow store accepted")
	}

	worker := newWorkerForTest(t, reports, tasks, client, observer, &now)
	byok := modelbilling.TaskScope{ActorID: "actor-worker", ProfileID: "default"}
	if found, err := worker.DeliverOne(context.Background(), byok); found || modelbilling.ErrorCodeOf(err) != modelbilling.ErrorBillingSourceMismatch {
		t.Fatalf("BYOK DeliverOne = %v, %v", found, err)
	}
	if err := worker.ReconcileTask(context.Background(), byok, "task-byok"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorBillingSourceMismatch {
		t.Fatalf("BYOK ReconcileTask = %v", err)
	}
	if reports.claimCalls != 0 || tasks.getCalls != 0 || client.uploads+client.finalizes+client.gets != 0 {
		t.Fatalf("BYOK caused side effects: claims=%d gets=%d cloud=(%d,%d,%d)", reports.claimCalls, tasks.getCalls, client.uploads, client.finalizes, client.gets)
	}
}

func TestRetryDelayUsesClaimedAttemptAndBoundedJitter(t *testing.T) {
	now := workerTestNow
	worker := &Worker{retry: RetryConfig{
		BaseDelay: 10 * time.Second, MaxDelay: 60 * time.Second, JitterFraction: 0.5,
		Clock: func() time.Time { return now }, LeaseDuration: time.Second,
		Jitter: func(limit time.Duration) time.Duration { return limit },
	}}
	for _, test := range []struct {
		attempt int
		delay   time.Duration
	}{{1, 15 * time.Second}, {2, 30 * time.Second}, {3, 60 * time.Second}, {4, 60 * time.Second}, {100, 60 * time.Second}} {
		next, err := worker.nextRetry(test.attempt, now)
		if err != nil || !next.Equal(now.Add(test.delay)) {
			t.Fatalf("attempt %d next = %v, %v; want %v", test.attempt, next, err, now.Add(test.delay))
		}
	}
	if _, err := worker.nextRetry(0, now); err == nil {
		t.Fatal("zero attempt accepted")
	}
	worker.retry.Jitter = func(limit time.Duration) time.Duration { return limit + time.Nanosecond }
	if _, err := worker.nextRetry(1, now); err == nil {
		t.Fatal("out-of-bound jitter accepted")
	}
}

func TestDeliverOneUploadsQuotaPartialSnapshots(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		name := "complete"
		if incomplete {
			name = "evidence_pending"
		}
		t.Run(name, func(t *testing.T) {
			now := workerTestNow
			scope := workerTestScope()
			partial := newPartialQuotaReport(t, scope, "task-partial-"+name, "freeze-partial-"+name, 1, incomplete)
			reports := &scriptedReportStore{reports: []modelbilling.PendingReport{pendingPartial(t, partial)}}
			client := &recordingCloud{t: t}
			worker := newWorkerForTest(t, reports, &scriptedTaskStore{}, client, &observerSpy{}, &now)

			found, err := worker.DeliverOne(context.Background(), scope)
			if !found || err != nil {
				t.Fatalf("DeliverOne = %v, %v", found, err)
			}
			if client.uploads != 1 || client.finalizes != 0 || !reflect.DeepEqual(client.partials[0], partial) {
				t.Fatalf("cloud calls = uploads:%d finalizes:%d report:%+v", client.uploads, client.finalizes, client.partials)
			}
			stored := reports.reports[0]
			if stored.State != modelbilling.ReportAccepted || stored.AttemptCount != 1 || stored.NextAttemptAt != nil {
				t.Fatalf("stored report = state:%s attempt:%d next:%v", stored.State, stored.AttemptCount, stored.NextAttemptAt)
			}
		})
	}
}

func TestDeliverOneClassifiesQuotaCloudFailures(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantState modelbilling.ReportState
		wantCode  modelbilling.ErrorCode
		continues bool
	}{
		{"retry", &modelbilling.Error{Code: modelbilling.ErrorGatewayUnavailable, Message: "offline", Retryable: true}, modelbilling.ReportRetry, modelbilling.ErrorGatewayUnavailable, true},
		{"block", &modelbilling.Error{Code: modelbilling.ErrorQuotaEvidenceIncomplete, Message: "final evidence incomplete"}, modelbilling.ReportBlocked, modelbilling.ErrorQuotaEvidenceIncomplete, true},
		{"auth", &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "login required"}, modelbilling.ReportDelivering, modelbilling.ErrorAuthRequired, false},
		{"scope", &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "binding changed"}, modelbilling.ReportDelivering, modelbilling.ErrorScopeMismatch, false},
		{"cancel", context.Canceled, modelbilling.ReportDelivering, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := workerTestNow
			scope := workerTestScope()
			partial := newPartialQuotaReport(t, scope, "task-failure-"+test.name, "freeze-failure-"+test.name, 1, false)
			reports := &scriptedReportStore{reports: []modelbilling.PendingReport{pendingPartial(t, partial)}}
			client := &recordingCloud{t: t, uploadErrors: []error{test.err}}
			worker := newWorkerForTest(t, reports, &scriptedTaskStore{}, client, &observerSpy{}, &now)

			found, err := worker.DeliverOne(context.Background(), scope)
			if !found {
				t.Fatal("report was not claimed")
			}
			if test.wantCode != "" {
				assertErrorCode(t, err, test.wantCode)
			} else if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
			if canContinueDeliveryPass(err) != test.continues {
				t.Fatalf("continue = %v, want %v", canContinueDeliveryPass(err), test.continues)
			}
			stored := reports.reports[0]
			if stored.State != test.wantState || stored.AttemptCount != 1 {
				t.Fatalf("report = state:%s attempt:%d", stored.State, stored.AttemptCount)
			}
			switch test.wantState {
			case modelbilling.ReportRetry:
				if stored.NextAttemptAt == nil || !stored.NextAttemptAt.Equal(now.Add(10*time.Second)) || stored.LastErrorCode == nil || *stored.LastErrorCode != string(test.wantCode) {
					t.Fatalf("retry outcome = next:%v code:%v", stored.NextAttemptAt, stored.LastErrorCode)
				}
			case modelbilling.ReportBlocked:
				if stored.NextAttemptAt != nil || stored.LastErrorCode == nil || *stored.LastErrorCode != string(test.wantCode) {
					t.Fatalf("blocked outcome = next:%v code:%v", stored.NextAttemptAt, stored.LastErrorCode)
				}
			case modelbilling.ReportDelivering:
				if stored.NextAttemptAt == nil || !stored.NextAttemptAt.Equal(now.Add(30*time.Second)) {
					t.Fatalf("lease was changed: %v", stored.NextAttemptAt)
				}
			}
		})
	}
}

func TestDeliverOneCorrectsRejectedFinalBeforeIdempotencyBinding(t *testing.T) {
	for _, test := range []struct {
		name          string
		correctionErr error
		wantState     modelbilling.ReportState
		wantCorrected int
	}{
		{name: "corrected", wantState: modelbilling.ReportPending, wantCorrected: 1},
		{
			name: "outside_boundary",
			correctionErr: &modelbilling.Error{
				Code: modelbilling.ErrorQuotaFinalPayloadConflict, Message: "business facts changed",
			},
			wantState: modelbilling.ReportBlocked,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := workerTestNow
			scope := workerTestScope()
			task := workerTask("task-final-correction-"+test.name, "freeze-final-correction-"+test.name,
				scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
			reports := &scriptedReportStore{
				reports:       []modelbilling.PendingReport{pendingFinal(t, newFinalQuotaReport(t, task))},
				correctionErr: test.correctionErr,
			}
			tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			client := &recordingCloud{t: t, finalizeResults: []cloudResult{{err: &modelbilling.Error{
				Code: modelbilling.ErrorQuotaSummaryMismatch, Message: "cloud recomputation rejected summary",
			}}}}
			worker := newWorkerForTest(t, reports, tasks, client, &observerSpy{}, &now)

			found, err := worker.DeliverOne(context.Background(), scope)
			if !found || !canContinueDeliveryPass(err) {
				t.Fatalf("DeliverOne = %v, %v", found, err)
			}
			assertErrorCode(t, err, modelbilling.ErrorQuotaSummaryMismatch)
			stored := reports.reports[0]
			if stored.State != test.wantState || reports.corrections != test.wantCorrected ||
				client.finalizes != 1 || reports.accepted != 0 {
				t.Fatalf("correction = state:%s corrections:%d finalizes:%d accepted:%d",
					stored.State, reports.corrections, client.finalizes, reports.accepted)
			}
			if test.wantCorrected == 1 {
				if reports.correctionReason != modelbilling.ErrorQuotaSummaryMismatch || stored.AttemptCount != 1 ||
					stored.NextAttemptAt == nil || !stored.NextAttemptAt.Equal(now) || stored.LastErrorCode != nil {
					t.Fatalf("corrected report = %+v reason %q", stored, reports.correctionReason)
				}
			}
		})
	}
}

func TestLegacyReportsNeverCallQuotaCloud(t *testing.T) {
	t.Run("delivery", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		partial := newPartialQuotaReport(t, scope, "task-legacy-partial", "freeze-legacy-partial", 1, false)
		legacy := pendingPartial(t, partial)
		legacy.ContractVersion = modelbilling.LegacyTokenContractV1
		legacy.PayloadHash = ""
		reports := &scriptedReportStore{reports: []modelbilling.PendingReport{legacy}}
		client := &recordingCloud{t: t}
		worker := newWorkerForTest(t, reports, &scriptedTaskStore{}, client, &observerSpy{}, &now)

		found, err := worker.DeliverOne(context.Background(), scope)
		if !found || !canContinueDeliveryPass(err) {
			t.Fatalf("DeliverOne = %v, %v", found, err)
		}
		assertErrorCode(t, err, modelbilling.ErrorLegacyReportContract)
		if reports.reports[0].State != modelbilling.ReportBlocked || client.uploads+client.finalizes+client.gets != 0 {
			t.Fatalf("legacy delivery = state:%s cloud:(%d,%d,%d)", reports.reports[0].State, client.uploads, client.finalizes, client.gets)
		}
	})

	t.Run("accepted_final_recovery", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-legacy-final", "freeze-legacy-final", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
		legacy := pendingFinal(t, newFinalQuotaReport(t, task))
		legacy.ContractVersion = modelbilling.LegacyTokenContractV1
		legacy.PayloadHash = ""
		legacy = withReportState(legacy, modelbilling.ReportAccepted)
		reports := &scriptedReportStore{reports: []modelbilling.PendingReport{legacy}}
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		client := &recordingCloud{t: t}
		worker := newWorkerForTest(t, reports, tasks, client, &observerSpy{}, &now)

		err := worker.ReconcileTask(context.Background(), scope, task.TaskID)
		assertErrorCode(t, err, modelbilling.ErrorLegacyReportContract)
		if tasks.tasks[task.TaskID].State != modelbilling.TaskRecoveryRequired || client.uploads+client.finalizes+client.gets != 0 {
			t.Fatalf("legacy reconcile = state:%s cloud:(%d,%d,%d)", tasks.tasks[task.TaskID].State, client.uploads, client.finalizes, client.gets)
		}
	})
}

func TestFinalQuotaDeliveryIsExactAcrossAcceptanceCrashWindow(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-final-crash", "freeze-final-crash", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
	final := newFinalQuotaReport(t, task)
	reports := &scriptedReportStore{
		reports:            []modelbilling.PendingReport{pendingFinal(t, final)},
		markAcceptedErrors: []error{errors.New("acceptance write failed")},
	}
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	pendingResult := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
	client := &recordingCloud{t: t, finalizeResults: []cloudResult{{result: pendingResult}, {result: pendingResult}}}
	worker := newWorkerForTest(t, reports, tasks, client, &observerSpy{}, &now)

	found, err := worker.DeliverOne(context.Background(), scope)
	if !found || err == nil {
		t.Fatalf("first DeliverOne = %v, %v", found, err)
	}
	if reports.reports[0].State != modelbilling.ReportDelivering || reports.reports[0].AttemptCount != 1 || tasks.tasks[task.TaskID].State != modelbilling.TaskUsagePending {
		t.Fatalf("first crash window = report:%s/%d task:%s", reports.reports[0].State, reports.reports[0].AttemptCount, tasks.tasks[task.TaskID].State)
	}
	if found, err := worker.DeliverOne(context.Background(), scope); found || err != nil {
		t.Fatalf("leased DeliverOne = %v, %v", found, err)
	}
	now = now.Add(31 * time.Second)
	found, err = worker.DeliverOne(context.Background(), scope)
	if !found || err != nil {
		t.Fatalf("retry DeliverOne = %v, %v", found, err)
	}
	if client.finalizes != 2 || len(client.finalPayloads) != 2 || !bytes.Equal(client.finalPayloads[0], client.finalPayloads[1]) {
		t.Fatalf("final calls were not exact-idempotent: calls=%d payloads=%q", client.finalizes, client.finalPayloads)
	}
	if reports.reports[0].State != modelbilling.ReportAccepted || reports.reports[0].AttemptCount != 2 || tasks.tasks[task.TaskID].State != modelbilling.TaskSettling {
		t.Fatalf("retry result = report:%s/%d task:%s", reports.reports[0].State, reports.reports[0].AttemptCount, tasks.tasks[task.TaskID].State)
	}
}

func TestFinalQuotaDeliveryBlocksDeterministicAcceptanceConflict(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-final-accept-conflict", "freeze-final-accept-conflict", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
	final := newFinalQuotaReport(t, task)
	conflict := &modelbilling.Error{Code: modelbilling.ErrorQuotaRequestIDReused, Message: "request was accepted for another task"}
	reports := &scriptedReportStore{
		reports:            []modelbilling.PendingReport{pendingFinal(t, final)},
		markAcceptedErrors: []error{conflict},
	}
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	client := &recordingCloud{t: t, finalizeResults: []cloudResult{{result: quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)}}}
	worker := newWorkerForTest(t, reports, tasks, client, &observerSpy{}, &now)

	found, err := worker.DeliverOne(context.Background(), scope)
	if !found || !canContinueDeliveryPass(err) || modelbilling.ErrorCodeOf(err) != modelbilling.ErrorQuotaRequestIDReused {
		t.Fatalf("DeliverOne = %v, %v", found, err)
	}
	if reports.reports[0].State != modelbilling.ReportBlocked || reports.blocked != 1 || client.finalizes != 1 {
		t.Fatalf("acceptance conflict = report:%s blocked:%d finalizes:%d",
			reports.reports[0].State, reports.blocked, client.finalizes)
	}
}

func TestAcceptedFinalRestartOnlyQueriesSettlement(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-final-restart", "freeze-final-restart", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
	accepted := withReportState(pendingFinal(t, newFinalQuotaReport(t, task)), modelbilling.ReportAccepted)
	reports := &scriptedReportStore{reports: []modelbilling.PendingReport{accepted}}
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)}}}
	worker := newWorkerForTest(t, reports, tasks, client, &observerSpy{}, &now)

	if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
		t.Fatal(err)
	}
	if client.finalizes != 0 || client.gets != 1 || tasks.tasks[task.TaskID].State != modelbilling.TaskSettling {
		t.Fatalf("restart = finalizes:%d gets:%d task:%s", client.finalizes, client.gets, tasks.tasks[task.TaskID].State)
	}
}

func TestReconcileSettlementEventUsesWorkerSettlementOrderWithoutFinalize(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-event-settlement", "freeze-event-settlement", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	events := []string{}
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}, events: &events}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	workflow.events = &events
	observer := &observerSpy{events: &events}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: result}}}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

	if err := worker.ReconcileSettlementEvent(
		context.Background(), actor, *scope.CloudBinding, task.TaskID, result.Settlement.SettlementID,
	); err != nil {
		t.Fatal(err)
	}
	if client.gets != 1 || client.finalizes != 0 {
		t.Fatalf("cloud calls = gets:%d finalizes:%d", client.gets, client.finalizes)
	}
	if want := []string{"settlement", "observer", "workflow", "task"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestReconcileSettlementEventRejectsWrongRESTSettlementIDBeforePersistence(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-event-wrong-rest", "freeze-event-wrong-rest", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	observer := &observerSpy{}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: result}}}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

	err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-from-event")
	assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
	if workflow.settlementSaves != 0 || observer.attempts != 0 || len(workflow.updates) != 0 || len(tasks.updates) != 0 {
		t.Fatalf("wrong settlement ID effects = saves:%d observer:%d workflow:%d task:%d",
			workflow.settlementSaves, observer.attempts, len(workflow.updates), len(tasks.updates))
	}
	if client.gets != 1 || client.finalizes != 0 {
		t.Fatalf("cloud calls = gets:%d finalizes:%d", client.gets, client.finalizes)
	}
}

func TestReconcileSettlementEventRejectsWrongCachedSettlementIDBeforeObserver(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-event-wrong-cache", "freeze-event-wrong-cache", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	workflow.settlements = map[string]modelbilling.QuotaSettlementResult{task.TaskID: result}
	observer := &observerSpy{}
	client := &recordingCloud{t: t}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

	err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-from-event")
	assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
	if client.gets+client.finalizes != 0 || workflow.settlementSaves != 0 || observer.attempts != 0 || len(workflow.updates) != 0 || len(tasks.updates) != 0 {
		t.Fatalf("wrong cached ID effects = cloud:%d saves:%d observer:%d workflow:%d task:%d",
			client.gets+client.finalizes, workflow.settlementSaves, observer.attempts, len(workflow.updates), len(tasks.updates))
	}
}

func TestReconcileSettlementEventChecksPendingCachedSettlementIDBeforeGET(t *testing.T) {
	t.Run("wrong event identity", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-wrong-pending-cache", "freeze-event-wrong-pending-cache", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		pending := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
		workflow.settlements = map[string]modelbilling.QuotaSettlementResult{task.TaskID: pending}
		client := &recordingCloud{t: t}
		observer := &observerSpy{}
		worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

		err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-from-event")
		assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
		if client.gets != 0 || client.finalizes != 0 || observer.attempts != 0 || len(workflow.updates) != 0 || len(tasks.updates) != 0 {
			t.Fatalf("wrong pending cache effects = gets:%d finalizes:%d observer:%d workflow:%d task:%d",
				client.gets, client.finalizes, observer.attempts, len(workflow.updates), len(tasks.updates))
		}
	})

	t.Run("matching event identity continues GET", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-matching-pending-cache", "freeze-event-matching-pending-cache", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		pending := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
		terminal := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
		workflow.settlements = map[string]modelbilling.QuotaSettlementResult{task.TaskID: pending}
		client := &recordingCloud{t: t, getResults: []cloudResult{{result: terminal}}}
		observer := &observerSpy{}
		worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

		if err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, pending.Settlement.SettlementID); err != nil {
			t.Fatal(err)
		}
		if client.gets != 1 || client.finalizes != 0 || observer.attempts != 1 || tasks.tasks[task.TaskID].State != modelbilling.TaskCompleted {
			t.Fatalf("matching pending cache = gets:%d finalizes:%d observer:%d task:%s",
				client.gets, client.finalizes, observer.attempts, tasks.tasks[task.TaskID].State)
		}
	})

	t.Run("ordinary reconcile continues GET", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-ordinary-pending-cache", "freeze-ordinary-pending-cache", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		pending := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
		workflow.settlements = map[string]modelbilling.QuotaSettlementResult{task.TaskID: pending}
		client := &recordingCloud{t: t, getResults: []cloudResult{{result: pending}}}
		worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, &observerSpy{}, workflow, &now)

		if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
			t.Fatal(err)
		}
		if client.gets != 1 || client.finalizes != 0 || tasks.tasks[task.TaskID].State != modelbilling.TaskSettling {
			t.Fatalf("ordinary pending cache = gets:%d finalizes:%d task:%s", client.gets, client.finalizes, tasks.tasks[task.TaskID].State)
		}
	})
}

func TestReconcileSettlementEventCompletedTaskRequiresMatchingPersistedTerminalSettlement(t *testing.T) {
	for _, test := range []struct {
		name       string
		settlement map[string]modelbilling.QuotaSettlementResult
	}{
		{name: "missing"},
		{name: "wrong_id", settlement: map[string]modelbilling.QuotaSettlementResult{
			"task-event-completed-wrong_id": quotaSettlementResult("task-event-completed-wrong_id", modelbilling.QuotaSettlementSettled),
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := workerTestNow
			scope := workerTestScope()
			taskID := "task-event-completed-" + test.name
			task := workerTask(taskID, "freeze-"+taskID, scope, modelbilling.TaskCompleted, modelbilling.TaskResultCompleted)
			tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettled)
			workflow.settlements = test.settlement
			observer := &observerSpy{}
			client := &recordingCloud{t: t}
			worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
			actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

			err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-from-event")
			assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
			if client.gets+client.finalizes != 0 || workflow.settlementSaves != 0 || observer.attempts != 0 || len(tasks.updates) != 0 {
				t.Fatalf("completed identity effects = gets:%d finalizes:%d saves:%d observer:%d task:%d",
					client.gets, client.finalizes, workflow.settlementSaves, observer.attempts, len(tasks.updates))
			}
		})
	}
}

func TestReconcileSettlementEventReusesCachedTerminalAndVerifiesCompletedTask(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-event-cached", "freeze-event-cached", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	workflow.settlements = map[string]modelbilling.QuotaSettlementResult{task.TaskID: result}
	observer := &observerSpy{}
	client := &recordingCloud{t: t}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

	if err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, result.Settlement.SettlementID); err != nil {
		t.Fatal(err)
	}
	if client.gets+client.finalizes != 0 || workflow.settlementSaves != 1 || observer.attempts != 1 || tasks.tasks[task.TaskID].State != modelbilling.TaskCompleted {
		t.Fatalf("cached reconcile = gets:%d finalizes:%d saves:%d observer:%d task:%s",
			client.gets, client.finalizes, workflow.settlementSaves, observer.attempts, tasks.tasks[task.TaskID].State)
	}
	if err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, result.Settlement.SettlementID); err != nil {
		t.Fatal(err)
	}
	if client.gets+client.finalizes != 0 || workflow.settlementSaves != 1 || observer.attempts != 1 {
		t.Fatalf("completed verification repeated side effects = gets:%d finalizes:%d saves:%d observer:%d",
			client.gets, client.finalizes, workflow.settlementSaves, observer.attempts)
	}
}

func TestReconcileSettlementEventAcceptedFinalOnlyGetsSettlement(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-event-accepted", "freeze-event-accepted", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementPending)
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: result}}}
	worker := newWorkerForTest(t, acceptedFinalStore(t, task), tasks, client, &observerSpy{}, &now)
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

	if err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, result.Settlement.SettlementID); err != nil {
		t.Fatal(err)
	}
	if client.gets != 1 || client.finalizes != 0 || tasks.tasks[task.TaskID].State != modelbilling.TaskSettling {
		t.Fatalf("accepted final event = gets:%d finalizes:%d task:%s", client.gets, client.finalizes, tasks.tasks[task.TaskID].State)
	}
}

func TestReconcileSettlementEventUsagePendingRequiresFinalWhileReconcileTaskStillWaits(t *testing.T) {
	t.Run("event fails closed", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-usage-no-final", "freeze-event-usage-no-final", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		client := &recordingCloud{t: t}
		observer := &observerSpy{}
		worker := newWorkerForTest(t, &scriptedReportStore{}, tasks, client, observer, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

		err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-event")
		assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
		if client.gets != 0 || client.finalizes != 0 || observer.attempts != 0 || len(tasks.updates) != 0 {
			t.Fatalf("missing final effects = gets:%d finalizes:%d observer:%d task:%d",
				client.gets, client.finalizes, observer.attempts, len(tasks.updates))
		}
		assertTaskFacts(t, tasks.tasks[task.TaskID], task)
	})

	t.Run("ordinary reconcile keeps waiting", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-ordinary-usage-no-final", "freeze-ordinary-usage-no-final", scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		client := &recordingCloud{t: t}
		observer := &observerSpy{}
		worker := newWorkerForTest(t, &scriptedReportStore{}, tasks, client, observer, &now)

		if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
			t.Fatalf("ReconcileTask = %v, want wait", err)
		}
		if client.gets != 0 || client.finalizes != 0 || observer.attempts != 0 || len(tasks.updates) != 0 {
			t.Fatalf("ordinary reconcile effects = gets:%d finalizes:%d observer:%d task:%d",
				client.gets, client.finalizes, observer.attempts, len(tasks.updates))
		}
		assertTaskFacts(t, tasks.tasks[task.TaskID], task)
	})
}

func TestReconcileSettlementEventUsagePendingRejectsNonAcceptedFinalWhileReconcileTaskKeepsExistingSemantics(t *testing.T) {
	for _, test := range []struct {
		name              string
		state             modelbilling.ReportState
		wantOrdinaryState modelbilling.TaskState
		wantUpdates       int
	}{
		{name: "blocked", state: modelbilling.ReportBlocked, wantOrdinaryState: modelbilling.TaskRecoveryRequired, wantUpdates: 1},
		{name: "pending", state: modelbilling.ReportPending, wantOrdinaryState: modelbilling.TaskUsagePending},
		{name: "retry", state: modelbilling.ReportRetry, wantOrdinaryState: modelbilling.TaskUsagePending},
		{name: "delivering", state: modelbilling.ReportDelivering, wantOrdinaryState: modelbilling.TaskUsagePending},
	} {
		t.Run(test.name, func(t *testing.T) {
			newFixture := func(taskID string) (modelbilling.TaskScope, modelbilling.ModelTask, *scriptedReportStore, *scriptedTaskStore, *recordingCloud, *observerSpy, *Worker) {
				now := workerTestNow
				scope := workerTestScope()
				task := workerTask(taskID, "freeze-"+taskID, scope, modelbilling.TaskUsagePending, modelbilling.TaskResultCompleted)
				final := pendingFinal(t, newFinalQuotaReport(t, task))
				final.State = test.state
				reports := &scriptedReportStore{reports: []modelbilling.PendingReport{final}}
				tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
				client := &recordingCloud{t: t}
				observer := &observerSpy{}
				worker := newWorkerForTest(t, reports, tasks, client, observer, &now)
				return scope, task, reports, tasks, client, observer, worker
			}

			scope, task, _, tasks, client, observer, worker := newFixture("task-event-non-accepted-" + test.name)
			actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}
			err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-event")
			assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
			if client.gets != 0 || client.finalizes != 0 || observer.attempts != 0 || len(tasks.updates) != 0 {
				t.Fatalf("event effects = gets:%d finalizes:%d observer:%d task:%d",
					client.gets, client.finalizes, observer.attempts, len(tasks.updates))
			}
			assertTaskFacts(t, tasks.tasks[task.TaskID], task)

			scope, task, _, tasks, client, observer, worker = newFixture("task-ordinary-non-accepted-" + test.name)
			if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
				t.Fatalf("ReconcileTask = %v", err)
			}
			if client.gets != 0 || client.finalizes != 0 || observer.attempts != 0 || len(tasks.updates) != test.wantUpdates || tasks.tasks[task.TaskID].State != test.wantOrdinaryState {
				t.Fatalf("ordinary effects = gets:%d finalizes:%d observer:%d updates:%d task:%s",
					client.gets, client.finalizes, observer.attempts, len(tasks.updates), tasks.tasks[task.TaskID].State)
			}
		})
	}
}

func TestReconcileSettlementEventFailsClosedWithoutAcceptedFinalOrExactScope(t *testing.T) {
	t.Run("no accepted final", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-no-final", "freeze-event-no-final", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		client := &recordingCloud{t: t}
		observer := &observerSpy{}
		worker := newWorkerForTest(t, &scriptedReportStore{}, tasks, client, observer, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

		err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-event")
		assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
		if client.gets+client.finalizes != 0 || observer.attempts != 0 || len(tasks.updates) != 0 {
			t.Fatalf("missing final effects = gets:%d finalizes:%d observer:%d task:%d", client.gets, client.finalizes, observer.attempts, len(tasks.updates))
		}
	})

	t.Run("binding generation", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-wrong-scope", "freeze-event-wrong-scope", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		client := &recordingCloud{t: t}
		observer := &observerSpy{}
		worker := newWorkerForTest(t, acceptedFinalStore(t, task), tasks, client, observer, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}
		wrong := *scope.CloudBinding
		wrong.BindingGeneration++

		err := worker.ReconcileSettlementEvent(context.Background(), actor, wrong, task.TaskID, "settlement-event")
		assertErrorCode(t, err, modelbilling.ErrorScopeMismatch)
		if client.gets+client.finalizes != 0 || observer.attempts != 0 || len(tasks.updates) != 0 {
			t.Fatalf("wrong scope effects = gets:%d finalizes:%d observer:%d task:%d", client.gets, client.finalizes, observer.attempts, len(tasks.updates))
		}
	})

	t.Run("actor profile", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-wrong-profile", "freeze-event-wrong-profile", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		client := &recordingCloud{t: t}
		observer := &observerSpy{}
		worker := newWorkerForTest(t, acceptedFinalStore(t, task), tasks, client, observer, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: "another-profile"}

		err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, "settlement-event")
		assertErrorCode(t, err, modelbilling.ErrorScopeMismatch)
		if tasks.getCalls != 0 || client.gets+client.finalizes != 0 || observer.attempts != 0 {
			t.Fatalf("wrong profile effects = task gets:%d cloud:%d observer:%d", tasks.getCalls, client.gets+client.finalizes, observer.attempts)
		}
	})

	t.Run("empty expected settlement", func(t *testing.T) {
		now := workerTestNow
		scope := workerTestScope()
		task := workerTask("task-event-empty-settlement", "freeze-event-empty-settlement", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
		tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
		worker := newWorkerForTest(t, acceptedFinalStore(t, task), tasks, &recordingCloud{t: t}, &observerSpy{}, &now)
		actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}

		err := worker.ReconcileSettlementEvent(context.Background(), actor, *scope.CloudBinding, task.TaskID, " ")
		assertErrorCode(t, err, modelbilling.ErrorContractInvalid)
		if tasks.getCalls != 0 {
			t.Fatalf("empty settlement reached task store %d times", tasks.getCalls)
		}
	})
}

func TestReconcileQuotaSettlementStates(t *testing.T) {
	tests := []struct {
		name         string
		status       modelbilling.QuotaSettlementState
		wantTask     modelbilling.TaskState
		wantWorkflow modelbilling.QuotaWorkflowStatus
		wantObserver int
	}{
		{"pending", modelbilling.QuotaSettlementPending, modelbilling.TaskSettling, modelbilling.QuotaWorkflowSettlementPending, 0},
		{"settled", modelbilling.QuotaSettlementSettled, modelbilling.TaskCompleted, modelbilling.QuotaWorkflowSettled, 1},
		{"uncovered", modelbilling.QuotaSettlementSettledUncovered, modelbilling.TaskCompleted, modelbilling.QuotaWorkflowSettledWithUncovered, 1},
		{"manual", modelbilling.QuotaSettlementManualRequired, modelbilling.TaskRecoveryRequired, modelbilling.QuotaWorkflowFailedManualRequired, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := workerTestNow
			scope := workerTestScope()
			task := workerTask("task-settlement-"+test.name, "freeze-settlement-"+test.name, scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
			tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
			client := &recordingCloud{t: t, getResults: []cloudResult{{result: quotaSettlementResult(task.TaskID, test.status)}}}
			observer := &observerSpy{}
			worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)

			if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
				t.Fatal(err)
			}
			if tasks.tasks[task.TaskID].State != test.wantTask || workflow.workflows[task.TaskID].Status != test.wantWorkflow || observer.attempts != test.wantObserver {
				t.Fatalf("result = task:%s workflow:%s observer:%d", tasks.tasks[task.TaskID].State, workflow.workflows[task.TaskID].Status, observer.attempts)
			}
			if client.gets != 1 || client.finalizes != 0 {
				t.Fatalf("cloud calls = gets:%d finalizes:%d", client.gets, client.finalizes)
			}
		})
	}
}

func TestInvalidSettlementConservationReturnsToRESTRecovery(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*modelbilling.QuotaSettlementResult)
	}{
		{
			name: "freeze_conservation",
			mutate: func(result *modelbilling.QuotaSettlementResult) {
				result.Settlement.ReleasedFrozenAmount = decimalPointer("8")
				result.Settlement.ReleasedFrozenFreeAmount = decimalPointer("4")
				result.Settlement.ReleasedFrozenSubscriptionAmount = decimalPointer("4")
			},
		},
		{
			name: "balance_snapshot",
			mutate: func(result *modelbilling.QuotaSettlementResult) {
				result.Balance.TotalBalance = "21"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := workerTestNow
			scope := workerTestScope()
			task := workerTask("task-invalid-"+test.name, "freeze-invalid-"+test.name, scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
			result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
			test.mutate(&result)
			tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
			workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
			observer := &observerSpy{}
			client := &recordingCloud{t: t, getResults: []cloudResult{{result: result}}}
			worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)

			err := worker.ReconcileTask(context.Background(), scope, task.TaskID)
			assertErrorCode(t, err, modelbilling.ErrorSettlementBalanceInconsistent)
			storedWorkflow := workflow.workflows[task.TaskID]
			if tasks.tasks[task.TaskID].State != modelbilling.TaskSettling || storedWorkflow.Status != modelbilling.QuotaWorkflowRetryPending ||
				storedWorkflow.RetryStage == nil || *storedWorkflow.RetryStage != modelbilling.QuotaRetrySettlementQuery || observer.attempts != 0 {
				t.Fatalf("recovery = task:%s workflow:%+v observer:%d", tasks.tasks[task.TaskID].State, storedWorkflow, observer.attempts)
			}
		})
	}
}

func TestSettlementObserverIsAtLeastOnceBeforeCompletion(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-observer-retry", "freeze-observer-retry", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: result}}}
	observerErr := errors.New("observer unavailable")
	observer := &observerSpy{errors: []error{observerErr, nil}}
	reports := acceptedFinalStore(t, task)
	worker := newWorkerWithWorkflowForTest(t, reports, tasks, client, observer, workflow, &now)

	if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); !errors.Is(err, observerErr) {
		t.Fatalf("first reconcile = %v", err)
	}
	if tasks.tasks[task.TaskID].State != modelbilling.TaskSettling || workflow.workflows[task.TaskID].Status != modelbilling.QuotaWorkflowSettlementPending {
		t.Fatalf("observer failure committed state: task=%s workflow=%s", tasks.tasks[task.TaskID].State, workflow.workflows[task.TaskID].Status)
	}
	restartedCloud := &recordingCloud{t: t}
	restarted := newWorkerWithWorkflowForTest(t, reports, tasks, restartedCloud, observer, workflow, &now)
	if err := restarted.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
		t.Fatal(err)
	}
	if observer.attempts != 2 || client.gets != 1 || restartedCloud.gets != 0 || tasks.tasks[task.TaskID].State != modelbilling.TaskCompleted {
		t.Fatalf("restart = observer:%d cloud gets:%d/%d task:%s", observer.attempts, client.gets, restartedCloud.gets, tasks.tasks[task.TaskID].State)
	}
}

func TestSettlementIsPersistedBeforeObserverWorkflowAndTask(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-settlement-order", "freeze-settlement-order", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	events := []string{}
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}, events: &events}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	workflow.events = &events
	observer := &observerSpy{events: &events}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)}}}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)

	if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err != nil {
		t.Fatal(err)
	}
	if want := []string{"settlement", "observer", "workflow", "task"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSettlementPersistenceFailureDoesNotAdvanceState(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-settlement-persist-fail", "freeze-settlement-persist-fail", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	workflow.settlementSaveErr = errors.New("disk unavailable")
	observer := &observerSpy{}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)}}}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)

	if err := worker.ReconcileTask(context.Background(), scope, task.TaskID); err == nil {
		t.Fatal("persistence failure was accepted")
	}
	if tasks.tasks[task.TaskID].State != modelbilling.TaskSettling ||
		workflow.workflows[task.TaskID].Status != modelbilling.QuotaWorkflowSettlementPending ||
		observer.attempts != 0 || len(workflow.updates) != 0 {
		t.Fatalf("advanced after persistence failure: task=%s workflow=%s observer=%d updates=%d",
			tasks.tasks[task.TaskID].State, workflow.workflows[task.TaskID].Status, observer.attempts, len(workflow.updates))
	}
}

func TestSettlementTaskTotalMustMatchAcceptedFinal(t *testing.T) {
	now := workerTestNow
	scope := workerTestScope()
	task := workerTask("task-settlement-total", "freeze-settlement-total", scope, modelbilling.TaskSettling, modelbilling.TaskResultCompleted)
	result := quotaSettlementResult(task.TaskID, modelbilling.QuotaSettlementSettled)
	result.Settlement.TaskTotalQuota = decimalPointer("2")
	tasks := &scriptedTaskStore{tasks: map[string]modelbilling.ModelTask{task.TaskID: task}}
	workflow := newWorkflowStore(task, modelbilling.QuotaWorkflowSettlementPending)
	observer := &observerSpy{}
	client := &recordingCloud{t: t, getResults: []cloudResult{{result: result}}}
	worker := newWorkerWithWorkflowForTest(t, acceptedFinalStore(t, task), tasks, client, observer, workflow, &now)

	err := worker.ReconcileTask(context.Background(), scope, task.TaskID)
	assertErrorCode(t, err, modelbilling.ErrorQuotaSummaryMismatch)
	if workflow.settlementSaves != 0 || observer.attempts != 0 ||
		workflow.workflows[task.TaskID].Status != modelbilling.QuotaWorkflowRetryPending {
		t.Fatalf("mismatch effects: saves=%d observer=%d workflow=%s", workflow.settlementSaves, observer.attempts, workflow.workflows[task.TaskID].Status)
	}
}

func TestSQLiteQuotaReportLeaseRejectsStaleAttempt(t *testing.T) {
	db := openWorkerSQLite(t)
	scope := workerTestScope()
	task := persistSQLiteUsagePendingTask(t, db, modelbilling.NewSQLiteTaskStore(db), scope,
		"task-sqlite-lease", "freeze-sqlite-lease", false)
	partial := newPartialQuotaReport(t, scope, task.TaskID, task.Freeze.FreezeID, 1, false)
	pending := pendingPartial(t, partial)
	insertSQLitePendingReport(t, db, pending, pending.CreatedAt)
	store := modelbilling.NewSQLiteReportStore(db)
	now := workerTestNow

	first, found, err := store.ClaimDue(context.Background(), scope, now, now.Add(30*time.Second))
	if err != nil || !found || first.AttemptCount != 1 {
		t.Fatalf("first claim = %+v, %v, %v", first, found, err)
	}
	if _, found, err := store.ClaimDue(context.Background(), scope, now.Add(29*time.Second), now.Add(59*time.Second)); err != nil || found {
		t.Fatalf("claim before lease expiry = %v, %v", found, err)
	}
	secondNow := now.Add(31 * time.Second)
	second, found, err := store.ClaimDue(context.Background(), scope, secondNow, secondNow.Add(30*time.Second))
	if err != nil || !found || second.AttemptCount != 2 {
		t.Fatalf("second claim = %+v, %v, %v", second, found, err)
	}
	if err := store.MarkAccepted(context.Background(), scope, pending.IdempotencyKey, first.AttemptCount, secondNow); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorInvalidStateTransition {
		t.Fatalf("stale acceptance = %v", err)
	}
	if err := store.MarkAccepted(context.Background(), scope, pending.IdempotencyKey, second.AttemptCount, secondNow.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	stored, found, err := store.GetByIdempotencyKey(context.Background(), scope, pending.IdempotencyKey)
	if err != nil || !found || stored.State != modelbilling.ReportAccepted || stored.AttemptCount != 2 {
		t.Fatalf("stored report = %+v, %v, %v", stored, found, err)
	}
}

func TestSQLiteWorkerQuarantinesPoisonQuotaRowAndContinues(t *testing.T) {
	db := openWorkerSQLite(t)
	scope := workerTestScope()
	task := persistSQLiteUsagePendingTask(t, db, modelbilling.NewSQLiteTaskStore(db), scope,
		"task-sqlite-poison", "freeze-sqlite-poison", false)
	poison := pendingPartial(t, newPartialQuotaReport(t, scope, task.TaskID, task.Freeze.FreezeID, 1, false))
	valid := pendingPartial(t, newPartialQuotaReport(t, scope, task.TaskID, task.Freeze.FreezeID, 2, true))
	insertSQLitePendingReport(t, db, poison, workerTestNow.Add(-2*time.Minute))
	insertSQLitePendingReport(t, db, valid, workerTestNow.Add(-time.Minute))
	if _, err := db.Exec(`UPDATE usage_reports SET canonical_payload=? WHERE idempotency_key=?`, []byte(`{}`), poison.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	client := &recordingCloud{t: t}
	now := workerTestNow
	worker, err := New(modelbilling.NewSQLiteReportStore(db), modelbilling.NewSQLiteTaskStore(db), client, &observerSpy{}, testRetryConfig(&now))
	if err != nil {
		t.Fatal(err)
	}

	found, deliveryErr := worker.DeliverOne(context.Background(), scope)
	if !found || !canContinueDeliveryPass(deliveryErr) {
		t.Fatalf("poison DeliverOne = %v, %v", found, deliveryErr)
	}
	assertErrorCode(t, deliveryErr, modelbilling.ErrorIdempotencyConflict)
	assertSQLiteReportState(t, db, poison.IdempotencyKey, modelbilling.ReportBlocked, 1, modelbilling.ErrorIdempotencyConflict)
	found, deliveryErr = worker.DeliverOne(context.Background(), scope)
	if !found || deliveryErr != nil {
		t.Fatalf("successor DeliverOne = %v, %v", found, deliveryErr)
	}
	if client.uploads != 1 || client.finalizes+client.gets != 0 {
		t.Fatalf("cloud calls = uploads:%d finalizes:%d gets:%d", client.uploads, client.finalizes, client.gets)
	}
	assertSQLiteReportState(t, db, valid.IdempotencyKey, modelbilling.ReportAccepted, 1, "")
	if found, err := worker.DeliverOne(context.Background(), scope); found || err != nil {
		t.Fatalf("drained DeliverOne = %v, %v", found, err)
	}
}

type trackingSQLiteReportStore struct {
	*modelbilling.SQLiteReportStore
	correctionCalls  int
	correctionReason modelbilling.ErrorCode
	correctionErr    error
}

func (s *trackingSQLiteReportStore) CorrectUnacceptedFinal(
	ctx context.Context,
	scope modelbilling.TaskScope,
	key string,
	expectedAttempt int,
	reason modelbilling.ErrorCode,
	at time.Time,
) (modelbilling.PendingReport, error) {
	s.correctionCalls++
	s.correctionReason = reason
	corrected, err := s.SQLiteReportStore.CorrectUnacceptedFinal(ctx, scope, key, expectedAttempt, reason, at)
	s.correctionErr = err
	return corrected, err
}

func TestSQLiteWorkerFailsClosedWhenCanonicalFinalNeedsNoSafeCorrection(t *testing.T) {
	tests := []struct {
		name string
		code modelbilling.ErrorCode
	}{
		{name: "summary_mismatch", code: modelbilling.ErrorQuotaSummaryMismatch},
		{name: "time_invalid", code: modelbilling.ErrorQuotaTimeInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openWorkerSQLite(t)
			scope := workerTestScope()
			taskID := "task-sqlite-correction-" + test.name
			tasks := modelbilling.NewSQLiteTaskStore(db)
			task := persistSQLiteUsagePendingTask(t, db, tasks, scope, taskID,
				"freeze-sqlite-correction-"+test.name, true)
			now := workerTestNow
			reports := &trackingSQLiteReportStore{
				SQLiteReportStore: modelbilling.NewSQLiteReportStoreWithClock(db, func() time.Time { return now }),
			}
			build, pending, err := reports.EnqueueFinal(context.Background(), scope, task.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			if build.Disposition != modelbilling.ReportBuildDeliverable || build.Final == nil || pending == nil {
				t.Fatalf("final enqueue = %+v / %+v", build, pending)
			}
			if err := build.Final.ValidateForTask(task); err != nil {
				t.Fatalf("enqueued final is not legal: %v", err)
			}
			canonical, err := modelbilling.CanonicalJSON(*build.Final)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canonical, pending.CanonicalPayload) {
				t.Fatal("enqueued final payload is not canonical builder output")
			}
			originalKey := pending.IdempotencyKey
			originalHash := pending.PayloadHash
			originalPayload := append([]byte(nil), pending.CanonicalPayload...)

			client := &recordingCloud{t: t, finalizeResults: []cloudResult{{err: &modelbilling.Error{
				Code: test.code, Message: "cloud rejected canonical final",
			}}}}
			observer := &observerSpy{}
			workflows := modelbilling.NewSQLiteQuotaWorkflowStore(db)
			worker, err := New(reports, tasks, client, observer, testRetryConfig(&now), workflows)
			if err != nil {
				t.Fatal(err)
			}

			found, deliveryErr := worker.DeliverOne(context.Background(), scope)
			if !found || !canContinueDeliveryPass(deliveryErr) {
				t.Fatalf("DeliverOne = %v, %v", found, deliveryErr)
			}
			assertErrorCode(t, deliveryErr, test.code)
			stored, found, err := reports.GetByIdempotencyKey(context.Background(), scope, originalKey)
			if err != nil || !found {
				t.Fatalf("stored final = %+v, %v, %v", stored, found, err)
			}
			if stored.IdempotencyKey != originalKey || stored.PayloadHash != originalHash ||
				!bytes.Equal(stored.CanonicalPayload, originalPayload) {
				t.Fatalf("fail-closed correction changed identity or payload: %+v", stored)
			}
			if stored.State != modelbilling.ReportBlocked || stored.AttemptCount != 1 || stored.NextAttemptAt != nil ||
				stored.LastErrorCode == nil || *stored.LastErrorCode != string(test.code) {
				t.Fatalf("blocked final = %+v", stored)
			}
			if reports.correctionCalls != 1 || reports.correctionReason != test.code ||
				modelbilling.ErrorCodeOf(reports.correctionErr) != modelbilling.ErrorQuotaFinalPayloadConflict {
				t.Fatalf("correction = calls:%d reason:%q err:%v",
					reports.correctionCalls, reports.correctionReason, reports.correctionErr)
			}
			if client.finalizes != 1 || len(client.finalPayloads) != 1 ||
				!bytes.Equal(client.finalPayloads[0], originalPayload) {
				t.Fatalf("cloud final delivery = calls:%d payloads:%d", client.finalizes, len(client.finalPayloads))
			}

			workflow, err := workflows.Get(context.Background(), scope, task.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			if workflow.Status != modelbilling.QuotaWorkflowFailedManualRequired || workflow.RetryStage != nil ||
				workflow.LastErrorCode == nil || *workflow.LastErrorCode != string(test.code) {
				t.Fatalf("workflow = %+v", workflow)
			}
			storedTask, err := tasks.Get(context.Background(), scope, task.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			var accepted, settlements int
			if err := db.QueryRow(`SELECT COUNT(*) FROM usage_reports WHERE task_id=? AND state=?`,
				task.TaskID, modelbilling.ReportAccepted).Scan(&accepted); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM model_task_quota_settlement_revisions WHERE task_id=?`,
				task.TaskID).Scan(&settlements); err != nil {
				t.Fatal(err)
			}
			if accepted != 0 || settlements != 0 || storedTask.State != modelbilling.TaskUsagePending ||
				client.gets != 0 || observer.attempts != 0 {
				t.Fatalf("unexpected settlement advance: accepted=%d settlements=%d task=%s gets=%d observer=%d",
					accepted, settlements, storedTask.State, client.gets, observer.attempts)
			}
		})
	}
}

func openWorkerSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := modelbilling.OpenSQLite(t.TempDir() + "/worker.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := modelbilling.ApplyMigrations(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func persistSQLiteUsagePendingTask(
	t *testing.T,
	db *sql.DB,
	store *modelbilling.SQLiteTaskStore,
	scope modelbilling.TaskScope,
	taskID, freezeID string,
	withCompleteQuotaCall bool,
) modelbilling.ModelTask {
	t.Helper()
	created := workerTestNow.Add(-10 * time.Minute)
	task := modelbilling.ModelTask{
		TaskID: taskID, Scope: scope, BillingSource: modelbilling.BillingSubscriptionPool,
		InitialModelID: "model-worker", ClientTaskType: "chat", ClientEstimatedCost: "10",
		PricingEvidenceID: "price-" + taskID, State: modelbilling.TaskCreated,
		CreatedAt: created, UpdatedAt: created,
	}
	evidence := modelbilling.PricingEvidence{
		EvidenceID: task.PricingEvidenceID, SnapshotID: "snapshot-worker", Scope: scope, TaskID: task.TaskID,
		ModelID: task.InitialModelID, PricingGroup: "default", State: modelbilling.PricingFresh, CapturedAt: created,
	}
	if err := store.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
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
	if err := store.RecordFreezeRequest(context.Background(), taskContext, request, created.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	freeze := &cloud.Freeze{
		Scope: *scope.CloudBinding, FreezeID: freezeID, TaskID: task.TaskID, Status: cloud.FreezeActive,
		FreezeAmount: "10", FreeFrozenAmount: "4", SubscriptionFrozenAmount: "6",
		ExpiresAt: workerTestNow.Add(time.Hour),
	}
	entitlement := &modelbilling.TaskEntitlementSnapshot{Plan: "pro", CapturedAt: created.Add(2 * time.Minute)}
	if err := store.RecordFreezeResult(context.Background(), taskContext, freeze, entitlement, created.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateState(context.Background(), modelbilling.TaskStateUpdate{
		Scope: scope, TaskID: task.TaskID, From: modelbilling.TaskReady, To: modelbilling.TaskRunning, At: created.Add(3 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if withCompleteQuotaCall {
		persistSQLiteCompleteQuotaCall(t, db, taskContext, task, created.Add(4*time.Minute))
	}
	result := modelbilling.TaskResultCompleted
	if err := store.UpdateState(context.Background(), modelbilling.TaskStateUpdate{
		Scope: scope, TaskID: task.TaskID, From: modelbilling.TaskRunning, To: modelbilling.TaskUsagePending,
		Result: &result, At: workerTestNow.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.Get(context.Background(), scope, taskID)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func persistSQLiteCompleteQuotaCall(
	t *testing.T,
	db *sql.DB,
	taskContext modelbilling.TaskContext,
	task modelbilling.ModelTask,
	startedAt time.Time,
) {
	t.Helper()
	store := modelbilling.NewSQLiteCallStore(db)
	call := modelbilling.ModelCall{
		CallID: "call-" + task.TaskID, TaskID: task.TaskID, Scope: task.Scope,
		BillingSource: modelbilling.BillingSubscriptionPool, Role: modelbilling.CallRoleMain,
		ModelID: task.InitialModelID, Provider: "fixture", Endpoint: modelbilling.MaasEndpointChatCompletions,
		PricingEvidenceID: task.PricingEvidenceID, State: modelbilling.CallPrepared,
	}
	if err := store.Prepare(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginAttempt(context.Background(), taskContext, call.CallID, startedAt); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	evidence, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint:   modelbilling.MaasEndpointChatCompletions,
		Headers:    map[string][]string{"X-Oneapi-Request-Id": {"request-" + task.TaskID}},
		FinalJSON:  json.RawMessage(`{"billing":{"quota":1}}`),
		ObservedAt: finishedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestCount := int64(1)
	call.State = modelbilling.CallCompleted
	call.FinishedAt = &finishedAt
	call.QuotaEvidence = &evidence
	call.Usage = modelbilling.NormalizedUsage{
		RequestCount: &requestCount, BreakdownStatus: modelbilling.UsageExact,
	}
	if err := store.Complete(context.Background(), taskContext, call); err != nil {
		t.Fatal(err)
	}
}

func insertSQLitePendingReport(t *testing.T, db *sql.DB, report modelbilling.PendingReport, due time.Time) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO usage_reports (
		idempotency_key,task_id,report_kind,sequence,canonical_payload,state,attempt_count,
		next_attempt_at,last_error_code,created_at,updated_at,contract_version,payload_hash
	) VALUES (?,?,?,?,?,?,0,?,NULL,?,?,?,?)`,
		report.IdempotencyKey, report.TaskID, report.Kind, report.Sequence, []byte(report.CanonicalPayload),
		modelbilling.ReportPending, workerQueueStamp(due), workerQueueStamp(report.CreatedAt), workerQueueStamp(report.UpdatedAt),
		report.ContractVersion, report.PayloadHash)
	if err != nil {
		t.Fatal(err)
	}
}

func assertSQLiteReportState(
	t *testing.T,
	db *sql.DB,
	key string,
	wantState modelbilling.ReportState,
	wantAttempt int,
	wantCode modelbilling.ErrorCode,
) {
	t.Helper()
	var state string
	var attempt int
	var next, code sql.NullString
	if err := db.QueryRow(`SELECT state,attempt_count,next_attempt_at,last_error_code FROM usage_reports WHERE idempotency_key=?`, key).
		Scan(&state, &attempt, &next, &code); err != nil {
		t.Fatal(err)
	}
	if state != string(wantState) || attempt != wantAttempt || (wantState == modelbilling.ReportAccepted || wantState == modelbilling.ReportBlocked) && next.Valid {
		t.Fatalf("report = state:%s attempt:%d next:%+v", state, attempt, next)
	}
	if wantCode == "" {
		if code.Valid {
			t.Fatalf("last_error_code = %q", code.String)
		}
	} else if !code.Valid || code.String != string(wantCode) {
		t.Fatalf("last_error_code = %+v, want %q", code, wantCode)
	}
}

func workerQueueStamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000Z")
}
