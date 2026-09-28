package modelbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"sync"

	"kwu/internal/cloud"
)

type MaterializedQuotaFixtureSelection struct {
	FinalCaseName      string
	SettlementCaseName string
}

type acceptedQuotaFinal struct {
	key    string
	hash   string
	total  DecimalString
	result QuotaSettlementResult
}

// AcceptedQuotaRequestRegistry models the cloud-user-wide authority for
// request IDs already accepted by a final report. Local task scans remain
// binding-scoped; only this accepted identity index crosses profile/generation.
type AcceptedQuotaRequestRegistry interface {
	AcceptQuotaRequestIDs(context.Context, string, string, []string) error
}

// MaterializedFixtureCloudQuotaClient is a strict normalized fake. It never
// performs network I/O and does not emulate real Maas/cloud wire details.
type MaterializedFixtureCloudQuotaClient struct {
	mu               sync.Mutex
	scope            cloud.BindingScope
	final            QuotaSettlementFixtureCase
	settlement       QuotaSettlementFixtureCase
	partials         map[string]string
	sequences        map[string]string
	snapshots        map[string]PartialQuotaReport
	finals           map[string]acceptedQuotaFinal
	finalKeys        map[string]string
	taskFreeze       map[string]string
	freezeTask       map[string]string
	requestTask      map[string]string
	acceptedRequests AcceptedQuotaRequestRegistry
}

func NewMaterializedFixtureCloudQuotaClient(
	bundle QuotaFixtureBundle,
	scope cloud.BindingScope,
	selection MaterializedQuotaFixtureSelection,
) (*MaterializedFixtureCloudQuotaClient, error) {
	return NewMaterializedFixtureCloudQuotaClientWithAcceptedRequests(bundle, scope, selection, nil)
}

func NewMaterializedFixtureCloudQuotaClientWithAcceptedRequests(
	bundle QuotaFixtureBundle,
	scope cloud.BindingScope,
	selection MaterializedQuotaFixtureSelection,
	acceptedRequests AcceptedQuotaRequestRegistry,
) (*MaterializedFixtureCloudQuotaClient, error) {
	payload, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	validated, err := DecodeQuotaFixture(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if validated.Scope != scope {
		return nil, newError(ErrorScopeMismatch, "quota fixture scope mismatch")
	}
	final, err := quotaFixtureCase(validated, selection.FinalCaseName)
	if err != nil {
		return nil, err
	}
	settlement, err := quotaFixtureCase(validated, selection.SettlementCaseName)
	if err != nil {
		return nil, err
	}
	return &MaterializedFixtureCloudQuotaClient{
		scope: scope, final: final, settlement: settlement,
		partials: make(map[string]string), sequences: make(map[string]string), snapshots: make(map[string]PartialQuotaReport),
		finals: make(map[string]acceptedQuotaFinal), finalKeys: make(map[string]string),
		taskFreeze: make(map[string]string), freezeTask: make(map[string]string), requestTask: make(map[string]string),
		acceptedRequests: acceptedRequests,
	}, nil
}

func (c *MaterializedFixtureCloudQuotaClient) UploadPartialQuota(
	ctx context.Context,
	scope cloud.BindingScope,
	report PartialQuotaReport,
) error {
	if err := quotaFixtureRequestValid(ctx, c, scope); err != nil {
		return err
	}
	if report.Scope != scope {
		return newError(ErrorScopeMismatch, "partial quota report scope mismatch")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	payload, err := CanonicalJSON(report)
	if err != nil {
		return err
	}
	hash, err := QuotaPayloadHash(payload)
	if err != nil {
		return err
	}
	sequenceIdentity := report.TaskID + "\x00" + jsonSequence(report.Sequence)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.uploadPartialLocked(report, hash, sequenceIdentity)
}

func (c *MaterializedFixtureCloudQuotaClient) uploadPartialLocked(report PartialQuotaReport, hash, sequenceIdentity string) error {
	if err := c.validateTaskFreezeBinding(report.TaskID, report.FreezeID); err != nil {
		return err
	}
	if priorKey, found := c.sequences[sequenceIdentity]; found && priorKey != report.IdempotencyKey {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial sequence is already bound to another key")
	}
	if prior, found := c.partials[report.IdempotencyKey]; found {
		if prior != hash {
			return newError(ErrorQuotaPartialSnapshotConflict, "partial key is already bound to another payload")
		}
		return nil
	}
	if previous, found := c.snapshots[report.TaskID]; found {
		if err := validateFixtureCumulativePartial(previous, report); err != nil {
			return err
		}
	}
	c.bindTaskFreeze(report.TaskID, report.FreezeID)
	c.sequences[sequenceIdentity] = report.IdempotencyKey
	c.partials[report.IdempotencyKey] = hash
	c.snapshots[report.TaskID] = clonePartialQuotaReport(report)
	return nil
}

func (c *MaterializedFixtureCloudQuotaClient) FinalizeQuota(
	ctx context.Context,
	scope cloud.BindingScope,
	report FinalQuotaReport,
) (QuotaSettlementResult, error) {
	if err := quotaFixtureRequestValid(ctx, c, scope); err != nil {
		return QuotaSettlementResult{}, err
	}
	if report.Scope != scope {
		return QuotaSettlementResult{}, newError(ErrorScopeMismatch, "final quota report scope mismatch")
	}
	// The cloud ingress folds byte-identical duplicate calls before checking the
	// client summary. Local builders stay stricter and only emit canonical calls.
	normalized, err := normalizeQuotaFinalIngress(report)
	if err != nil {
		return QuotaSettlementResult{}, err
	}
	payload, err := CanonicalJSON(normalized)
	if err != nil {
		return QuotaSettlementResult{}, err
	}
	hash, err := QuotaPayloadHash(payload)
	if err != nil {
		return QuotaSettlementResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.finalizeQuotaLocked(ctx, normalized, hash)
}

func (c *MaterializedFixtureCloudQuotaClient) finalizeQuotaLocked(ctx context.Context, normalized FinalQuotaReport, hash string) (QuotaSettlementResult, error) {
	if err := c.validateTaskFreezeBinding(normalized.TaskID, normalized.FreezeID); err != nil {
		return QuotaSettlementResult{}, err
	}
	if accepted, found := c.finals[normalized.TaskID]; found {
		return validateAcceptedQuotaFinal(accepted, normalized.IdempotencyKey, hash)
	}
	if err := c.validateNewQuotaFinalIdentity(normalized); err != nil {
		return QuotaSettlementResult{}, err
	}
	return c.acceptQuotaFinal(ctx, normalized, hash)
}

func validateAcceptedQuotaFinal(accepted acceptedQuotaFinal, key, hash string) (QuotaSettlementResult, error) {
	if accepted.key != key {
		return QuotaSettlementResult{}, newError(ErrorQuotaFinalIdempotencyConflict, "task final is already bound to another key")
	}
	if accepted.hash != hash {
		return QuotaSettlementResult{}, newError(ErrorQuotaFinalPayloadConflict, "final key is already bound to another payload")
	}
	return cloneQuotaSettlementResult(accepted.result), nil
}

func (c *MaterializedFixtureCloudQuotaClient) validateNewQuotaFinalIdentity(normalized FinalQuotaReport) error {
	if owner, found := c.finalKeys[normalized.IdempotencyKey]; found && owner != normalized.TaskID {
		return newError(ErrorQuotaFinalIdempotencyConflict, "final key belongs to another task")
	}
	for _, call := range normalized.Calls {
		if call.OneAPIRequestID == nil {
			continue
		}
		if owner, found := c.requestTask[*call.OneAPIRequestID]; found && owner != normalized.TaskID {
			return newError(ErrorQuotaRequestIDReused, "oneapi request id was accepted for another task")
		}
	}
	return nil
}

func (c *MaterializedFixtureCloudQuotaClient) acceptQuotaFinal(ctx context.Context, normalized FinalQuotaReport, hash string) (QuotaSettlementResult, error) {
	if c.final.Error != nil {
		return QuotaSettlementResult{}, c.final.Error.domainError()
	}
	result, err := materializeQuotaFixtureResult(c.final, c.scope, normalized.TaskID)
	if err != nil {
		return QuotaSettlementResult{}, err
	}
	if err := validateFixtureSettlementTotal(result, normalized.ClientSummary.TaskTotalQuota); err != nil {
		return QuotaSettlementResult{}, err
	}
	if c.acceptedRequests != nil {
		requestIDs := quotaFinalAcceptedRequestIDs(normalized.Calls)
		if err := c.acceptedRequests.AcceptQuotaRequestIDs(ctx, c.scope.CloudUserID, normalized.TaskID, requestIDs); err != nil {
			return QuotaSettlementResult{}, err
		}
	}
	c.finals[normalized.TaskID] = acceptedQuotaFinal{
		key: normalized.IdempotencyKey, hash: hash, total: normalized.ClientSummary.TaskTotalQuota,
		result: cloneQuotaSettlementResult(result),
	}
	c.finalKeys[normalized.IdempotencyKey] = normalized.TaskID
	c.bindTaskFreeze(normalized.TaskID, normalized.FreezeID)
	for _, call := range normalized.Calls {
		c.requestTask[*call.OneAPIRequestID] = normalized.TaskID
	}
	return result, nil
}

func quotaFinalAcceptedRequestIDs(calls []QuotaReportCall) []string {
	ids := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		if call.OneAPIRequestID != nil {
			ids[*call.OneAPIRequestID] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(ids))
	for requestID := range ids {
		ordered = append(ordered, requestID)
	}
	sort.Strings(ordered)
	return ordered
}

// validateTaskFreezeBinding requires c.mu to be held.
func (c *MaterializedFixtureCloudQuotaClient) validateTaskFreezeBinding(taskID, freezeID string) error {
	if boundFreeze, found := c.taskFreeze[taskID]; found && boundFreeze != freezeID {
		return newError(ErrorScopeMismatch, "task id belongs to another freeze")
	}
	if boundTask, found := c.freezeTask[freezeID]; found && boundTask != taskID {
		return newError(ErrorScopeMismatch, "freeze id belongs to another task")
	}
	return nil
}

// bindTaskFreeze requires c.mu to be held and prior validation to have passed.
func (c *MaterializedFixtureCloudQuotaClient) bindTaskFreeze(taskID, freezeID string) {
	c.taskFreeze[taskID] = freezeID
	c.freezeTask[freezeID] = taskID
}

func (c *MaterializedFixtureCloudQuotaClient) GetQuotaSettlement(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID string,
) (QuotaSettlementResult, error) {
	if err := quotaFixtureRequestValid(ctx, c, scope); err != nil {
		return QuotaSettlementResult{}, err
	}
	if err := requireTrimmed("task_id", taskID); err != nil {
		return QuotaSettlementResult{}, err
	}
	result, err := materializeQuotaFixtureResult(c.settlement, c.scope, taskID)
	if err != nil {
		return QuotaSettlementResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getQuotaSettlementLocked(taskID, result)
}

func (c *MaterializedFixtureCloudQuotaClient) getQuotaSettlementLocked(taskID string, result QuotaSettlementResult) (QuotaSettlementResult, error) {
	accepted, found := c.finals[taskID]
	if !found {
		return result, nil
	}
	if err := validateFixtureSettlementTotal(result, accepted.total); err != nil {
		return QuotaSettlementResult{}, err
	}
	if accepted.result.Settlement.Status != QuotaSettlementPending &&
		result.Settlement.Status == accepted.result.Settlement.Status {
		left, leftErr := CanonicalJSON(accepted.result)
		right, rightErr := CanonicalJSON(result)
		if leftErr != nil || rightErr != nil || !bytes.Equal(left, right) {
			return QuotaSettlementResult{}, newError(ErrorIdempotencyConflict, "terminal quota settlement facts changed")
		}
		return cloneQuotaSettlementResult(accepted.result), nil
	}
	if !fixtureSettlementCanAdvance(accepted.result.Settlement.Status, result.Settlement.Status) {
		return cloneQuotaSettlementResult(accepted.result), nil
	}
	accepted.result = cloneQuotaSettlementResult(result)
	c.finals[taskID] = accepted
	return result, nil
}

func normalizeQuotaFinalIngress(report FinalQuotaReport) (FinalQuotaReport, error) {
	folded := make([]QuotaReportCall, 0, len(report.Calls))
	seen := make(map[string]QuotaReportCall, len(report.Calls))
	for _, call := range report.Calls {
		if err := call.ValidateForFinal(); err != nil {
			return FinalQuotaReport{}, err
		}
		if previous, found := seen[call.CallID]; found {
			if quotaReportCallsEqual([]QuotaReportCall{previous}, []QuotaReportCall{call}) {
				continue
			}
			return FinalQuotaReport{}, newError(ErrorQuotaCallIDConflict, "same call_id has different quota fingerprint")
		}
		seen[call.CallID] = call
		folded = append(folded, call)
	}
	canonical, err := CanonicalizeQuotaReportCalls(report.Calls)
	if err != nil {
		return FinalQuotaReport{}, err
	}
	if !quotaReportCallsEqual(canonical, folded) {
		return FinalQuotaReport{}, newError(ErrorQuotaCallIDConflict, "final quota calls are not canonical after duplicate folding")
	}
	normalized := report
	normalized.Calls = canonical
	if err := normalized.Validate(); err != nil {
		return FinalQuotaReport{}, err
	}
	return normalized, nil
}

func validateFixtureSettlementTotal(result QuotaSettlementResult, expected DecimalString) error {
	if result.Settlement.TaskTotalQuota != nil && *result.Settlement.TaskTotalQuota != expected {
		return newError(ErrorQuotaSummaryMismatch, "quota settlement total does not match accepted final")
	}
	return nil
}

func fixtureSettlementCanAdvance(from, to QuotaSettlementState) bool {
	if from == QuotaSettlementPending {
		return to == QuotaSettlementPending || to == QuotaSettlementSettled ||
			to == QuotaSettlementSettledUncovered || to == QuotaSettlementManualRequired
	}
	return from == to
}

func validateFixtureCumulativePartial(previous, candidate PartialQuotaReport) error {
	if candidate.Sequence <= previous.Sequence || candidate.CapturedAt.Before(previous.CapturedAt) {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota snapshot sequence or captured_at regressed")
	}
	current := make(map[string]QuotaReportCall, len(candidate.Calls))
	for _, call := range candidate.Calls {
		current[call.CallID] = call
	}
	for _, oldCall := range previous.Calls {
		newCall, ok := current[oldCall.CallID]
		if !ok {
			return newError(ErrorQuotaPartialSnapshotConflict, "partial quota snapshot removed a prior call")
		}
		if err := validateFixtureCumulativeCall(oldCall, newCall); err != nil {
			return err
		}
	}
	return nil
}

func validateFixtureCumulativeCall(previous, candidate QuotaReportCall) error {
	if !sameFixturePartialCallIdentity(previous, candidate) {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota snapshot rewrote immutable call identity")
	}
	if candidate.EvidenceRevision < previous.EvidenceRevision {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota evidence revision regressed")
	}
	same := quotaReportCallsEqual([]QuotaReportCall{previous}, []QuotaReportCall{candidate})
	if candidate.EvidenceRevision == previous.EvidenceRevision && !same {
		return newError(ErrorQuotaPartialSnapshotConflict, "partial quota evidence changed without a new revision")
	}
	if previous.EvidenceStatus == QuotaEvidenceComplete && candidate.EvidenceStatus != QuotaEvidenceComplete {
		return newError(ErrorQuotaPartialSnapshotConflict, "complete partial quota evidence regressed")
	}
	return nil
}

func sameFixturePartialCallIdentity(previous, candidate QuotaReportCall) bool {
	return candidate.Endpoint == previous.Endpoint && candidate.Model == previous.Model &&
		candidate.OccurredAt.Equal(previous.OccurredAt)
}

func clonePartialQuotaReport(input PartialQuotaReport) PartialQuotaReport {
	output := input
	output.Calls = make([]QuotaReportCall, len(input.Calls))
	for i := range input.Calls {
		output.Calls[i] = cloneQuotaReportCall(input.Calls[i])
	}
	return output
}

func quotaFixtureRequestValid(ctx context.Context, client *MaterializedFixtureCloudQuotaClient, scope cloud.BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if client == nil || scope != client.scope {
		return newError(ErrorScopeMismatch, "quota fixture request scope mismatch")
	}
	return nil
}

func materializeQuotaFixtureResult(
	fixture QuotaSettlementFixtureCase,
	scope cloud.BindingScope,
	taskID string,
) (QuotaSettlementResult, error) {
	if fixture.Error != nil {
		return QuotaSettlementResult{}, fixture.Error.domainError()
	}
	if fixture.Result == nil {
		return QuotaSettlementResult{}, newError(ErrorContractInvalid, "quota fixture has no result")
	}
	result := cloneQuotaSettlementResult(*fixture.Result)
	result.Settlement.TaskID = taskID
	settlementID, err := materializedSettlementID(scope, taskID)
	if err != nil {
		return QuotaSettlementResult{}, err
	}
	result.Settlement.SettlementID = settlementID
	if err := result.Validate(); err != nil {
		return QuotaSettlementResult{}, err
	}
	return result, nil
}

func cloneQuotaSettlementResult(input QuotaSettlementResult) QuotaSettlementResult {
	output := input
	clone := func(value *DecimalString) *DecimalString { return cloneOptionalDecimal(value) }
	output.Settlement.TaskTotalQuota = clone(input.Settlement.TaskTotalQuota)
	output.Settlement.QuotaPerUnit = clone(input.Settlement.QuotaPerUnit)
	output.Settlement.USDExchangeRate = clone(input.Settlement.USDExchangeRate)
	output.Settlement.CreditPerCNY = clone(input.Settlement.CreditPerCNY)
	output.Settlement.ChargedCredits = clone(input.Settlement.ChargedCredits)
	output.Settlement.ConsumedFrozenAmount = clone(input.Settlement.ConsumedFrozenAmount)
	output.Settlement.ConsumedFrozenFreeAmount = clone(input.Settlement.ConsumedFrozenFreeAmount)
	output.Settlement.ConsumedFrozenSubscriptionAmount = clone(input.Settlement.ConsumedFrozenSubscriptionAmount)
	output.Settlement.AdditionalChargedAmount = clone(input.Settlement.AdditionalChargedAmount)
	output.Settlement.AdditionalChargedFreeAmount = clone(input.Settlement.AdditionalChargedFreeAmount)
	output.Settlement.AdditionalChargedSubscriptionAmount = clone(input.Settlement.AdditionalChargedSubscriptionAmount)
	output.Settlement.ReleasedFrozenAmount = clone(input.Settlement.ReleasedFrozenAmount)
	output.Settlement.ReleasedFrozenFreeAmount = clone(input.Settlement.ReleasedFrozenFreeAmount)
	output.Settlement.ReleasedFrozenSubscriptionAmount = clone(input.Settlement.ReleasedFrozenSubscriptionAmount)
	output.Settlement.UncoveredAmount = clone(input.Settlement.UncoveredAmount)
	output.Settlement.Balance = clone(input.Settlement.Balance)
	if input.Settlement.BalanceVersion != nil {
		value := *input.Settlement.BalanceVersion
		output.Settlement.BalanceVersion = &value
	}
	if input.Balance != nil {
		value := *input.Balance
		output.Balance = &value
	}
	return output
}

func jsonSequence(sequence uint64) string {
	payload, _ := json.Marshal(sequence)
	return string(payload)
}

var _ CloudQuotaClient = (*MaterializedFixtureCloudQuotaClient)(nil)
