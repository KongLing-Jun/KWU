package modelbilling

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"kwu/internal/agent"
	"kwu/internal/provider"
)

const subscriptionGatewayProvider = "subscription_gateway_fixture"

const terminalPersistenceTimeout = 5 * time.Second

// CallDispatchSpec is the only per-call input accepted by the billing
// dispatcher. Task ownership and pricing facts are always derived internally.
type CallDispatchSpec struct {
	Role    CallRole
	ModelID string
}

// SubscriptionGatewayEndpointResolver is the pre-dispatch metadata seam used
// by normalized subscription gateways. Subscription calls must persist the
// exact Maas endpoint before dispatch.
type SubscriptionGatewayEndpointResolver interface {
	EndpointFor(GatewayRequest) (MaasEndpoint, error)
}

func (s CallDispatchSpec) Validate() error {
	if err := s.Role.Validate(); err != nil {
		return err
	}
	return requireTrimmed("call dispatch model_id", s.ModelID)
}

// MainExecutorDispatcher owns routing and write-ahead state for main and
// derived calls while preserving the existing main Agent.stream seam.
type MainExecutorDispatcher struct {
	Calls        CallStore
	Tasks        TaskStore
	Gateway      SubscriptionModelGateway
	Evidence     CallEvidenceResolver
	IDs          IDGenerator
	Now          func() time.Time
	ProviderName string
}

var _ agent.StreamDispatcher = (*MainExecutorDispatcher)(nil)

// RoleBoundStreamDispatcher binds one immutable role/model pair for Agent
// paths that use the generic StreamDispatcher seam.
type RoleBoundStreamDispatcher struct {
	dispatcher *MainExecutorDispatcher
	spec       CallDispatchSpec
}

var _ agent.StreamDispatcherWithTerminalResult = (*RoleBoundStreamDispatcher)(nil)

func NewRoleBoundStreamDispatcher(dispatcher *MainExecutorDispatcher, spec CallDispatchSpec) (*RoleBoundStreamDispatcher, error) {
	if dispatcher == nil {
		return nil, newError(ErrorContractInvalid, "call dispatcher is required")
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &RoleBoundStreamDispatcher{dispatcher: dispatcher, spec: spec}, nil
}

func (d *RoleBoundStreamDispatcher) Stream(ctx context.Context, base provider.Provider, request provider.Request) (<-chan provider.Chunk, error) {
	chunks, _, err := d.StreamWithTerminalResult(ctx, base, request)
	return chunks, err
}

func (d *RoleBoundStreamDispatcher) StreamWithTerminalResult(ctx context.Context, base provider.Provider, request provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	if d == nil || d.dispatcher == nil {
		return nil, nil, newError(ErrorContractInvalid, "role-bound call dispatcher is required")
	}
	return d.dispatcher.StreamCallWithTerminalResult(ctx, base, request, d.spec)
}

func (d *MainExecutorDispatcher) Stream(ctx context.Context, byok provider.Provider, request provider.Request) (<-chan provider.Chunk, error) {
	chunks, _, err := d.StreamWithTerminalResult(ctx, byok, request)
	return chunks, err
}

// StreamWithTerminalResult keeps the terminal persistence outcome scoped to
// this one dispatch. Agent uses it after cancellation because a caller that has
// stopped reading chunks must still receive a failed terminal write rather than
// seeing a misleading successful cancellation.
func (d *MainExecutorDispatcher) StreamWithTerminalResult(ctx context.Context, byok provider.Provider, request provider.Request) (<-chan provider.Chunk, <-chan error, error) {
	task, ok := TaskContextFrom(ctx)
	if !ok {
		return nil, nil, newError(ErrorContractInvalid, "main executor requires task context")
	}
	return d.streamCallWithTerminalResult(ctx, byok, request, CallDispatchSpec{Role: CallRoleMain, ModelID: task.InitialModelID}, false)
}

func (d *MainExecutorDispatcher) StreamCall(ctx context.Context, base provider.Provider, request provider.Request, spec CallDispatchSpec) (<-chan provider.Chunk, error) {
	chunks, _, err := d.StreamCallWithTerminalResult(ctx, base, request, spec)
	return chunks, err
}

func (d *MainExecutorDispatcher) StreamCallWithTerminalResult(ctx context.Context, base provider.Provider, request provider.Request, spec CallDispatchSpec) (<-chan provider.Chunk, <-chan error, error) {
	return d.streamCallWithTerminalResult(ctx, base, request, spec, true)
}

func (d *MainExecutorDispatcher) streamCallWithTerminalResult(ctx context.Context, byok provider.Provider, request provider.Request, spec CallDispatchSpec, requireBarrier bool) (<-chan provider.Chunk, <-chan error, error) {
	task, ok := TaskContextFrom(ctx)
	if !ok {
		return nil, nil, newError(ErrorContractInvalid, "model call dispatcher requires task context")
	}
	if err := d.validateCallDispatch(spec, requireBarrier); err != nil {
		return nil, nil, err
	}
	registration, err := d.registerForeground(ctx, task, requireBarrier)
	if err != nil {
		return nil, nil, err
	}
	handedOff := false
	var barrierErr error
	defer func() {
		if !handedOff && registration != nil {
			registration.Complete(barrierErr)
		}
	}()
	call, err := d.prepare(ctx, task, byok, request, spec)
	if err != nil {
		barrierErr = err
		return nil, nil, err
	}
	terminalResult := make(chan error, 1)
	chunks, foregroundErr, err := d.dispatch(ctx, task, call, byok, request, terminalResult, registration)
	if err != nil {
		barrierErr = foregroundErr
		return nil, nil, err
	}
	if chunks == nil {
		barrierErr = foregroundErr
		if barrierErr == nil {
			barrierErr = newError(ErrorContractInvalid, "model call dispatcher returned a nil stream")
		}
		return nil, nil, barrierErr
	}
	handedOff = true
	return chunks, terminalResult, nil
}

func (d *MainExecutorDispatcher) validateCallDispatch(spec CallDispatchSpec, requireBarrier bool) error {
	if d == nil || d.Calls == nil || d.IDs == nil {
		return newError(ErrorContractInvalid, "model call dispatcher dependencies are required")
	}
	if err := spec.Validate(); err != nil {
		return err
	}
	if requireBarrier && d.Evidence == nil {
		return newError(ErrorContractInvalid, "derived model call requires pricing evidence resolver")
	}
	return nil
}

func (d *MainExecutorDispatcher) registerForeground(ctx context.Context, task TaskContext, required bool) (*foregroundCallRegistration, error) {
	if _, ok := foregroundCallBarrierFrom(ctx); !ok && !required {
		return nil, nil
	}
	return registerForegroundCall(ctx, task.TaskID)
}

func (d *MainExecutorDispatcher) prepare(ctx context.Context, task TaskContext, byok provider.Provider, request provider.Request, spec CallDispatchSpec) (ModelCall, error) {
	callID, err := d.IDs.NewID()
	if err != nil {
		return ModelCall{}, err
	}
	evidenceID, err := d.resolveCallEvidenceID(ctx, task, spec)
	if err != nil {
		return ModelCall{}, err
	}
	providerName := d.providerForCall(task, byok)
	call := ModelCall{CallID: callID, TaskID: task.TaskID, Scope: task.Scope, BillingSource: task.BillingSource, Role: spec.Role, ModelID: spec.ModelID, Provider: providerName, PricingEvidenceID: evidenceID, State: CallPrepared}
	if task.BillingSource == BillingSubscriptionPool {
		endpoint, err := d.subscriptionEndpoint(GatewayRequest{
			TaskID: task.TaskID, CallID: callID, ModelID: spec.ModelID, Role: spec.Role, ProviderRequest: request,
		})
		if err != nil {
			return ModelCall{}, err
		}
		call.Endpoint = endpoint
	}
	if err := d.Calls.Prepare(ctx, task, call); err != nil {
		return ModelCall{}, err
	}
	emitModelCallLifecycle(ctx, call, "", CallPrepared)
	return call, nil
}

func (d *MainExecutorDispatcher) subscriptionEndpoint(request GatewayRequest) (MaasEndpoint, error) {
	resolver, ok := d.Gateway.(SubscriptionGatewayEndpointResolver)
	if !ok {
		return "", newError(ErrorContractInvalid, "subscription gateway must resolve its Maas endpoint")
	}
	endpoint, err := resolver.EndpointFor(request)
	if err != nil {
		return "", err
	}
	if err := endpoint.Validate(); err != nil {
		return "", err
	}
	return endpoint, nil
}

func (d *MainExecutorDispatcher) resolveCallEvidenceID(ctx context.Context, task TaskContext, spec CallDispatchSpec) (string, error) {
	if d.Evidence == nil {
		if spec.Role != CallRoleMain || spec.ModelID != task.InitialModelID {
			return "", newError(ErrorContractInvalid, "derived model call requires pricing evidence resolver")
		}
		return task.PricingEvidenceID, nil
	}
	evidence, err := d.Evidence.ResolveForCall(ctx, task, spec.ModelID)
	if err != nil {
		return "", err
	}
	if err := validateResolvedEvidenceForSpec(task, spec, evidence); err != nil {
		return "", err
	}
	return evidence.EvidenceID, nil
}

func (d *MainExecutorDispatcher) providerForCall(task TaskContext, byok provider.Provider) string {
	if task.BillingSource == BillingSubscriptionPool {
		return subscriptionGatewayProvider
	}
	if task.BillingSource == BillingNewAPIByok && byok != nil {
		return byok.Name()
	}
	return d.ProviderName
}

func validateResolvedEvidenceForSpec(task TaskContext, spec CallDispatchSpec, evidence PricingEvidence) error {
	if err := evidence.Validate(task.BillingSource); err != nil {
		return err
	}
	if evidence.TaskID != task.TaskID || !evidence.Scope.Equal(task.Scope) {
		return newError(ErrorScopeMismatch, "resolved call evidence does not belong to task scope")
	}
	if evidence.ModelID != spec.ModelID {
		return newError(ErrorContractInvalid, "resolved call evidence does not match dispatch model")
	}
	return nil
}

func (d *MainExecutorDispatcher) dispatch(ctx context.Context, task TaskContext, call ModelCall, byok provider.Provider, request provider.Request, terminalResult chan<- error, registration *foregroundCallRegistration) (<-chan provider.Chunk, error, error) {
	if task.BillingSource == BillingSubscriptionPool {
		return d.streamSubscription(ctx, task, call, request, terminalResult, registration)
	}
	if task.BillingSource != BillingNewAPIByok || byok == nil {
		err := newError(ErrorBillingSourceMismatch, "model call route has no matching dispatcher")
		return nil, err, err
	}
	return d.streamBYOK(ctx, task, call, byok, request, terminalResult, registration)
}

func (d *MainExecutorDispatcher) streamSubscription(ctx context.Context, task TaskContext, call ModelCall, request provider.Request, terminalResult chan<- error, registration *foregroundCallRegistration) (<-chan provider.Chunk, error, error) {
	if d.Gateway == nil || task.Scope.CloudBinding == nil {
		err := newError(ErrorContractInvalid, "subscription gateway and binding are required")
		return nil, err, err
	}
	if err := d.begin(ctx, task, call); err != nil {
		return nil, err, err
	}
	chunks, err := d.Gateway.Stream(ctx, *task.Scope.CloudBinding, GatewayRequest{TaskID: task.TaskID, CallID: call.CallID, ModelID: call.ModelID, Role: call.Role, ProviderRequest: request})
	if err != nil {
		if persistErr := d.completeInitialError(ctx, task, call, err); persistErr != nil {
			return nil, persistErr, persistErr
		}
		return nil, err, err
	}
	return d.pipeSubscription(ctx, task, call, chunks, terminalResult, registration), nil, nil
}

func (d *MainExecutorDispatcher) streamBYOK(ctx context.Context, task TaskContext, call ModelCall, byok provider.Provider, request provider.Request, terminalResult chan<- error, registration *foregroundCallRegistration) (<-chan provider.Chunk, error, error) {
	attemptStarted := false
	var beginErr error
	ctx = provider.WithBeforeDispatch(ctx, func() error {
		if err := d.begin(ctx, task, call); err != nil {
			beginErr = err
			return err
		}
		attemptStarted = true
		beginErr = nil
		return nil
	})
	chunks, err := byok.Stream(ctx, request)
	if err != nil {
		if beginErr != nil {
			return nil, err, beginErr
		}
		if !attemptStarted {
			return nil, err, err
		}
		if persistErr := d.completeInitialError(ctx, task, call, err); persistErr != nil {
			return nil, persistErr, persistErr
		}
		return nil, err, err
	}
	return d.pipeBYOK(ctx, task, call, chunks, terminalResult, registration), nil, nil
}

func (d *MainExecutorDispatcher) begin(ctx context.Context, task TaskContext, call ModelCall) error {
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	if err := d.Calls.BeginAttempt(ctx, task, call.CallID, now); err != nil {
		return err
	}
	emitModelCallLifecycle(ctx, call, CallPrepared, CallRunning)
	return nil
}

func (d *MainExecutorDispatcher) pipeBYOK(ctx context.Context, task TaskContext, call ModelCall, chunks <-chan provider.Chunk, terminalResult chan<- error, registration *foregroundCallRegistration) <-chan provider.Chunk {
	out := make(chan provider.Chunk)
	go d.runBYOKPipe(ctx, task, call, chunks, out, terminalResult, registration)
	return out
}

func (d *MainExecutorDispatcher) pipeSubscription(ctx context.Context, task TaskContext, call ModelCall, chunks <-chan GatewayChunk, terminalResult chan<- error, registration *foregroundCallRegistration) <-chan provider.Chunk {
	out := make(chan provider.Chunk)
	go d.runSubscriptionPipe(ctx, task, call, chunks, out, terminalResult, registration)
	return out
}

func (d *MainExecutorDispatcher) runBYOKPipe(ctx context.Context, task TaskContext, call ModelCall, chunks <-chan provider.Chunk, out chan<- provider.Chunk, terminalResult chan<- error, registration *foregroundCallRegistration) {
	var persistErr error
	defer finishPipe(out, terminalResult, registration, &persistErr)
	var usage *provider.Usage
	for {
		chunk, ok, err := receiveProviderChunk(ctx, chunks)
		if err != nil {
			persistErr = d.forwardTerminalError(out, ctx, task, call, err)
			return
		}
		if !ok {
			persistErr = d.finishBYOK(out, ctx, task, call, usage, false)
			return
		}
		if terminal, err := d.handleBYOKChunk(out, ctx, task, call, chunk, &usage); terminal {
			persistErr = err
			return
		}
		if err := forwardLiveChunk(ctx, out, chunk); err != nil {
			persistErr = d.forwardTerminalError(out, ctx, task, call, err)
			return
		}
	}
}

func (d *MainExecutorDispatcher) runSubscriptionPipe(ctx context.Context, task TaskContext, call ModelCall, chunks <-chan GatewayChunk, out chan<- provider.Chunk, terminalResult chan<- error, registration *foregroundCallRegistration) {
	var persistErr error
	defer finishPipe(out, terminalResult, registration, &persistErr)
	var terminal subscriptionTerminal
	for {
		gatewayChunk, ok, err := receiveGatewayChunk(ctx, chunks)
		if err != nil {
			persistErr = d.forwardTerminalError(out, ctx, task, terminal.apply(call), err)
			return
		}
		if !ok {
			persistErr = d.finishSubscription(out, ctx, task, terminal.apply(call), terminal.usage, false)
			return
		}
		if err := terminal.accept(gatewayChunk); err != nil {
			persistErr = d.forwardTerminalError(out, ctx, task, terminal.apply(call), err)
			return
		}
		if finished, err := d.handleSubscriptionChunk(out, ctx, task, terminal.apply(call), terminal.usage, gatewayChunk.Chunk); finished {
			persistErr = err
			return
		}
		if err := forwardLiveChunk(ctx, out, gatewayChunk.Chunk); err != nil {
			persistErr = d.forwardTerminalError(out, ctx, task, terminal.apply(call), err)
			return
		}
	}
}

func (d *MainExecutorDispatcher) handleBYOKChunk(out chan<- provider.Chunk, ctx context.Context, task TaskContext, call ModelCall, chunk provider.Chunk, usage **provider.Usage) (bool, error) {
	if chunk.Type == provider.ChunkUsage {
		*usage = chunk.Usage
	}
	if chunk.Type == provider.ChunkError {
		return true, d.forwardTerminalError(out, ctx, task, call, chunk.Err)
	}
	if chunk.Type == provider.ChunkDone {
		return true, d.finishBYOK(out, ctx, task, call, *usage, true)
	}
	return false, nil
}

func (d *MainExecutorDispatcher) handleSubscriptionChunk(out chan<- provider.Chunk, ctx context.Context, task TaskContext, call ModelCall, usage *NormalizedUsage, chunk provider.Chunk) (bool, error) {
	if chunk.Type == provider.ChunkError {
		return true, d.forwardTerminalError(out, ctx, task, call, chunk.Err)
	}
	if chunk.Type == provider.ChunkDone {
		return true, d.finishSubscription(out, ctx, task, call, usage, true)
	}
	return false, nil
}

func finishPipe(out chan<- provider.Chunk, terminalResult chan<- error, registration *foregroundCallRegistration, persistErr *error) {
	if terminalResult != nil {
		terminalResult <- *persistErr
		close(terminalResult)
	}
	close(out)
	if registration != nil {
		registration.Complete(*persistErr)
	}
}

func receiveProviderChunk(ctx context.Context, chunks <-chan provider.Chunk) (provider.Chunk, bool, error) {
	select {
	case <-ctx.Done():
		return provider.Chunk{}, false, ctx.Err()
	case chunk, ok := <-chunks:
		if err := ctx.Err(); err != nil {
			return provider.Chunk{}, false, err
		}
		return chunk, ok, nil
	}
}

func receiveGatewayChunk(ctx context.Context, chunks <-chan GatewayChunk) (GatewayChunk, bool, error) {
	select {
	case <-ctx.Done():
		return GatewayChunk{}, false, ctx.Err()
	case chunk, ok := <-chunks:
		if err := ctx.Err(); err != nil {
			return GatewayChunk{}, false, err
		}
		return chunk, ok, nil
	}
}

func forwardLiveChunk(ctx context.Context, out chan<- provider.Chunk, chunk provider.Chunk) error {
	select {
	case out <- chunk:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type subscriptionTerminal struct {
	usage             *NormalizedUsage
	providerRequestID *string
	rawUsage          json.RawMessage
	quotaEvidence     *QuotaEvidenceRevision
	requestFacts      []quotaRequestFact
}

type quotaRequestFact struct {
	requestID string
	evidence  *QuotaEvidenceRevision
}

func (t *subscriptionTerminal) accept(chunk GatewayChunk) error {
	if err := chunk.Validate(); err != nil {
		return err
	}
	t.acceptUsageFacts(chunk)
	t.acceptRequestFacts(chunk)
	if err := t.acceptRequestIDs(gatewayChunkRequestIDs(chunk)); err != nil {
		return err
	}
	return t.acceptQuotaEvidenceFacts(gatewayChunkEvidenceFacts(chunk))
}

func (t *subscriptionTerminal) acceptUsageFacts(chunk GatewayChunk) {
	if chunk.Chunk.Type == provider.ChunkUsage {
		t.usage = chunk.Usage
	}
	if len(chunk.RawUsageRedacted) != 0 {
		t.rawUsage = append(t.rawUsage[:0], chunk.RawUsageRedacted...)
	}
}

func (t *subscriptionTerminal) acceptRequestFacts(chunk GatewayChunk) {
	if chunk.ProviderRequestID != "" {
		t.recordRequestFact(chunk.ProviderRequestID, gatewayRequestFactEvidence(chunk))
	}
	if chunk.QuotaEvidence != nil && chunk.QuotaEvidence.OneAPIRequestID != nil {
		t.recordRequestFact(*chunk.QuotaEvidence.OneAPIRequestID, chunk.QuotaEvidence)
	}
	for i := range chunk.QuotaEvidenceRevisions {
		revision := &chunk.QuotaEvidenceRevisions[i]
		t.recordRequestFact(*revision.OneAPIRequestID, revision)
	}
}

func gatewayRequestFactEvidence(chunk GatewayChunk) *QuotaEvidenceRevision {
	if chunk.QuotaEvidence == nil {
		return nil
	}
	if chunk.QuotaEvidence.OneAPIRequestID == nil || *chunk.QuotaEvidence.OneAPIRequestID == chunk.ProviderRequestID {
		return chunk.QuotaEvidence
	}
	return nil
}

func gatewayChunkRequestIDs(chunk GatewayChunk) []string {
	requestIDs := make([]string, 0, 2+len(chunk.QuotaEvidenceRevisions))
	if chunk.ProviderRequestID != "" {
		requestIDs = append(requestIDs, chunk.ProviderRequestID)
	}
	if chunk.QuotaEvidence != nil && chunk.QuotaEvidence.OneAPIRequestID != nil {
		requestIDs = append(requestIDs, *chunk.QuotaEvidence.OneAPIRequestID)
	}
	for i := range chunk.QuotaEvidenceRevisions {
		requestIDs = append(requestIDs, *chunk.QuotaEvidenceRevisions[i].OneAPIRequestID)
	}
	return requestIDs
}

func (t *subscriptionTerminal) acceptRequestIDs(requestIDs []string) error {
	for _, requestID := range requestIDs {
		merged, err := mergeGatewayRequestID(t.providerRequestID, requestID)
		if err != nil {
			return err
		}
		t.providerRequestID = merged
	}
	return nil
}

func gatewayChunkEvidenceFacts(chunk GatewayChunk) []*QuotaEvidenceRevision {
	evidenceFacts := make([]*QuotaEvidenceRevision, 0, 1+len(chunk.QuotaEvidenceRevisions))
	if chunk.QuotaEvidence != nil {
		evidenceFacts = append(evidenceFacts, chunk.QuotaEvidence)
	}
	for i := range chunk.QuotaEvidenceRevisions {
		evidenceFacts = append(evidenceFacts, &chunk.QuotaEvidenceRevisions[i])
	}
	return evidenceFacts
}

func (t *subscriptionTerminal) acceptQuotaEvidenceFacts(evidenceFacts []*QuotaEvidenceRevision) error {
	for _, evidence := range evidenceFacts {
		if t.quotaEvidence == nil {
			t.quotaEvidence = cloneQuotaEvidence(evidence)
			continue
		}
		if !quotaEvidenceProjectionEqual(t.quotaEvidence, evidence) {
			return newError(ErrorContractInvalid, "subscription gateway returned conflicting quota evidence")
		}
	}
	return nil
}

func (t *subscriptionTerminal) recordRequestFact(requestID string, evidence *QuotaEvidenceRevision) {
	if requestID == "" {
		return
	}
	for i := range t.requestFacts {
		if t.requestFacts[i].requestID != requestID {
			continue
		}
		if t.requestFacts[i].evidence == nil && evidence != nil {
			t.requestFacts[i].evidence = cloneQuotaEvidence(evidence)
			return
		}
		if evidence == nil || quotaEvidenceEqual(t.requestFacts[i].evidence, evidence) {
			return
		}
	}
	t.requestFacts = append(t.requestFacts, quotaRequestFact{
		requestID: requestID,
		evidence:  cloneQuotaEvidence(evidence),
	})
}

func mergeGatewayRequestID(existing *string, candidate string) (*string, error) {
	if candidate == "" {
		return cloneOptionalString(existing), nil
	}
	if err := requireTrimmed("Maas request ID", candidate); err != nil {
		return nil, err
	}
	if existing != nil && *existing != candidate {
		return nil, newError(ErrorContractInvalid, "subscription gateway returned multiple request IDs")
	}
	value := candidate
	return &value, nil
}

func quotaEvidenceProjectionEqual(left, right *QuotaEvidenceRevision) bool {
	if left == nil || right == nil {
		return left == right
	}
	return optionalStringEqual(left.OneAPIRequestID, right.OneAPIRequestID) &&
		quotaRawEqual(left.BillingQuotaRaw, right.BillingQuotaRaw) &&
		quotaRawEqual(left.UsageQuotaRaw, right.UsageQuotaRaw) &&
		optionalDecimalEqual(left.Quota, right.Quota) && optionalQuotaSourceEqual(left.Source, right.Source) &&
		left.Status == right.Status && quotaEvidenceIssuesEqual(left.Issues, right.Issues)
}

func (t subscriptionTerminal) apply(call ModelCall) ModelCall {
	call.ProviderRequestID = cloneOptionalString(t.providerRequestID)
	call.RawUsageRedacted = append(json.RawMessage(nil), t.rawUsage...)
	call.QuotaEvidence = cloneQuotaEvidence(t.quotaEvidence)
	if t.quotaEvidence != nil {
		occurredAt := t.quotaEvidence.ObservedAt.UTC()
		call.OccurredAt = &occurredAt
	}
	for _, fact := range t.requestFacts {
		call.quotaRequestIDs = append(call.quotaRequestIDs, fact.requestID)
	}
	if len(t.requestFacts) > 1 {
		var fallback *QuotaEvidenceRevision
		if t.quotaEvidence != nil {
			fallback = t.quotaEvidence
		} else {
			for _, fact := range t.requestFacts {
				if fact.evidence != nil {
					fallback = fact.evidence
					break
				}
			}
		}
		if fallback != nil {
			call.quotaEvidenceRevisions = make([]QuotaEvidenceRevision, len(t.requestFacts))
			for i, fact := range t.requestFacts {
				source := fact.evidence
				var revision QuotaEvidenceRevision
				if source == nil {
					requestID := fact.requestID
					revision = missingQuotaEvidence(&requestID, fallback.ObservedAt)
				} else {
					revision = deriveQuotaEvidence(&fact.requestID, source.BillingQuotaRaw, source.UsageQuotaRaw)
					revision.ObservedAt = source.ObservedAt
				}
				revision.Revision = uint64(i + 1)
				call.quotaEvidenceRevisions[i] = revision
			}
			call.QuotaEvidence = cloneQuotaEvidence(&call.quotaEvidenceRevisions[0])
			occurredAt := call.QuotaEvidence.ObservedAt.UTC()
			call.OccurredAt = &occurredAt
		}
	}
	return call
}

func (d *MainExecutorDispatcher) finishBYOK(out chan<- provider.Chunk, ctx context.Context, task TaskContext, call ModelCall, usage *provider.Usage, forwardDone bool) error {
	call.FinishedAt = timePtr(d.now())
	if usage == nil {
		call.State = CallUsageMissing
		call.Usage = NormalizedUsage{BreakdownStatus: UsageMissing}
	} else {
		call.State = CallCompleted
		call.Usage = normalizeBYOKUsage(usage)
	}
	return d.forwardCompleted(out, ctx, task, call, forwardDone)
}

func (d *MainExecutorDispatcher) finishSubscription(out chan<- provider.Chunk, ctx context.Context, task TaskContext, call ModelCall, usage *NormalizedUsage, forwardDone bool) error {
	call.FinishedAt = timePtr(d.now())
	if usage == nil {
		call.State = CallUsageMissing
		call.Usage = NormalizedUsage{BreakdownStatus: UsageMissing}
	} else {
		call.State = CallCompleted
		call.Usage = *usage
	}
	return d.forwardCompleted(out, ctx, task, call, forwardDone)
}

func (d *MainExecutorDispatcher) forwardCompleted(out chan<- provider.Chunk, ctx context.Context, task TaskContext, call ModelCall, forwardDone bool) error {
	if err := d.persistTerminal(ctx, task, call); err != nil {
		sendTerminalChunk(ctx, out, provider.Chunk{Type: provider.ChunkError, Err: err})
		return err
	}
	if forwardDone {
		sendTerminalChunk(ctx, out, provider.Chunk{Type: provider.ChunkDone})
	}
	return nil
}

func (d *MainExecutorDispatcher) forwardTerminalError(out chan<- provider.Chunk, ctx context.Context, task TaskContext, call ModelCall, streamErr error) error {
	if streamErr == nil {
		streamErr = newError(ErrorGatewayUnavailable, "model stream ended with an empty error")
	}
	call.FinishedAt = timePtr(d.now())
	call.State = CallFailed
	if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
		call.State = CallCancelled
	} else if provider.IsStreamInterrupted(streamErr) {
		call.State = CallInterrupted
	}
	code := ErrorGatewayUnavailable
	if task.BillingSource == BillingNewAPIByok {
		code = ErrorByokCredentialInvalid
	}
	if domain := ErrorCodeOf(streamErr); domain != "" {
		code = domain
	}
	text := string(code)
	call.LastErrorCode = &text
	call.Usage = NormalizedUsage{BreakdownStatus: UsageUnavailable}
	if err := d.persistTerminal(ctx, task, call); err != nil {
		sendTerminalChunk(ctx, out, provider.Chunk{Type: provider.ChunkError, Err: err})
		return err
	}
	sendTerminalChunk(ctx, out, provider.Chunk{Type: provider.ChunkError, Err: streamErr})
	return nil
}

func (d *MainExecutorDispatcher) completeInitialError(ctx context.Context, task TaskContext, call ModelCall, streamErr error) error {
	call.FinishedAt = timePtr(d.now())
	call.State = CallFailed
	if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) {
		call.State = CallCancelled
	} else if provider.IsStreamInterrupted(streamErr) {
		call.State = CallInterrupted
	}
	code := ErrorGatewayUnavailable
	if task.BillingSource == BillingNewAPIByok {
		code = ErrorByokCredentialInvalid
	}
	if domain := ErrorCodeOf(streamErr); domain != "" {
		code = domain
	}
	codeText := string(code)
	call.LastErrorCode = &codeText
	call.Usage = NormalizedUsage{BreakdownStatus: UsageUnavailable}
	if err := d.persistTerminal(ctx, task, call); err != nil {
		return err
	}
	return nil
}

// sendTerminalChunk keeps terminal delivery cancel-aware. Normal consumers
// still receive terminal chunks; once the caller cancels and stops consuming,
// the dispatcher can close its output after persistence instead of blocking.
func sendTerminalChunk(ctx context.Context, out chan<- provider.Chunk, chunk provider.Chunk) {
	select {
	case out <- chunk:
	case <-ctx.Done():
	}
}

func (d *MainExecutorDispatcher) persistTerminal(ctx context.Context, task TaskContext, call ModelCall) error {
	if completeErr := d.complete(ctx, task, call); completeErr != nil {
		recoveryErr := d.markRecovery(ctx, task)
		return newTerminalPersistenceError(completeErr, recoveryErr)
	}
	return nil
}

func (d *MainExecutorDispatcher) complete(ctx context.Context, task TaskContext, call ModelCall) error {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistenceTimeout)
	defer cancel()
	if err := d.Calls.Complete(persistCtx, task, call); err != nil {
		return err
	}
	emitModelCallLifecycle(ctx, call, CallRunning, call.State)
	return nil
}

func (d *MainExecutorDispatcher) markRecovery(ctx context.Context, task TaskContext) error {
	if d.Tasks == nil {
		return newError(ErrorContractInvalid, "task store is required to mark terminal persistence recovery")
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalPersistenceTimeout)
	defer cancel()
	code := string(ErrorUsagePersistFailed)
	if err := d.Tasks.UpdateState(persistCtx, TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskRunning, To: TaskRecoveryRequired, LastErrorCode: &code, At: d.now()}); err != nil {
		return err
	}
	emitTaskLifecycle(ctx, task.TaskID, TaskRunning, TaskRecoveryRequired)
	return nil
}

func (d *MainExecutorDispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

func timePtr(value time.Time) *time.Time { return &value }

func normalizeBYOKUsage(usage *provider.Usage) NormalizedUsage {
	value := func(present bool, tokens int) *int64 {
		if !present {
			return nil
		}
		v := int64(tokens)
		return &v
	}
	requests := int64(1)
	return NormalizedUsage{
		InputTokens:         value(usage.Presence.Input, usage.PromptTokens),
		NormalInputTokens:   value(usage.Presence.NormalInput, usage.NormalInputTokens),
		CachedInputTokens:   value(usage.Presence.CachedInput, usage.CacheHitTokens),
		CacheReadTokens:     value(usage.Presence.CacheRead, usage.CacheReadTokens),
		CacheWriteTokens:    value(usage.Presence.CacheWrite, usage.CacheWriteTokens),
		OutputTokens:        value(usage.Presence.Output, usage.CompletionTokens),
		ProviderTotalTokens: value(usage.Presence.ProviderTotal, usage.TotalTokens),
		RequestCount:        &requests,
		BreakdownStatus:     UsagePartial,
	}
}
