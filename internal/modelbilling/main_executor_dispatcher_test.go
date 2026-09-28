package modelbilling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

type recordingCallStore struct {
	prepared         []ModelCall
	begun            []string
	finished         []ModelCall
	completeCtxErrs  []error
	completeErr      error
	completeStarted  chan struct{}
	completeRelease  chan struct{}
	completeFinished chan struct{}
	completeOnce     sync.Once
}

func (s *recordingCallStore) Prepare(_ context.Context, _ TaskContext, call ModelCall) error {
	s.prepared = append(s.prepared, call)
	return nil
}
func (s *recordingCallStore) BeginAttempt(_ context.Context, _ TaskContext, callID string, _ time.Time) error {
	s.begun = append(s.begun, callID)
	return nil
}
func (s *recordingCallStore) Complete(ctx context.Context, _ TaskContext, call ModelCall) error {
	s.completeCtxErrs = append(s.completeCtxErrs, ctx.Err())
	if s.completeStarted != nil {
		close(s.completeStarted)
	}
	if s.completeRelease != nil {
		<-s.completeRelease
	}
	s.finished = append(s.finished, call)
	if s.completeFinished != nil {
		s.completeOnce.Do(func() { close(s.completeFinished) })
	}
	return s.completeErr
}

type recordingTaskStore struct {
	updates      []TaskStateUpdate
	updateErr    error
	failUpdateTo TaskState
	getErr       error
	getCalls     int
}

func (*recordingTaskStore) CreateTaskWithEvidence(context.Context, ModelTask, PricingEvidence) error {
	return nil
}
func (*recordingTaskStore) RecordFreezeRequest(context.Context, TaskContext, *cloud.FreezeRequest, time.Time) error {
	return nil
}
func (*recordingTaskStore) RecordFreezeResult(context.Context, TaskContext, *cloud.Freeze, *TaskEntitlementSnapshot, time.Time) error {
	return nil
}
func (s *recordingTaskStore) UpdateState(_ context.Context, update TaskStateUpdate) error {
	s.updates = append(s.updates, update)
	if s.updateErr != nil && (s.failUpdateTo == "" || s.failUpdateTo == update.To) {
		return s.updateErr
	}
	return nil
}
func (s *recordingTaskStore) Get(context.Context, TaskScope, string) (ModelTask, error) {
	s.getCalls++
	return ModelTask{}, s.getErr
}
func (*recordingTaskStore) ListRecoverable(context.Context, TaskScope) ([]ModelTask, error) {
	return nil, nil
}
func (*recordingCallStore) ListByTask(context.Context, TaskScope, string) ([]ModelCall, error) {
	return nil, nil
}

type dispatcherIDGenerator struct{ id string }

func (g dispatcherIDGenerator) NewID() (string, error) { return g.id, nil }

type recordingGateway struct {
	calls     int
	request   GatewayRequest
	chunks    []GatewayChunk
	endpoint  MaasEndpoint
	streamErr error
}

type endpointlessGateway struct{ calls int }

func (g *endpointlessGateway) Stream(context.Context, cloud.BindingScope, GatewayRequest) (<-chan GatewayChunk, error) {
	g.calls++
	return nil, newError(ErrorContractInvalid, "must not dispatch")
}

func (g *recordingGateway) EndpointFor(GatewayRequest) (MaasEndpoint, error) {
	if g.endpoint == "" {
		return "", newError(ErrorContractInvalid, "test gateway endpoint is required")
	}
	return g.endpoint, nil
}

func (g *recordingGateway) Stream(_ context.Context, _ cloud.BindingScope, request GatewayRequest) (<-chan GatewayChunk, error) {
	g.calls++
	g.request = request
	if g.streamErr != nil {
		return nil, g.streamErr
	}
	out := make(chan GatewayChunk, len(g.chunks))
	for _, chunk := range g.chunks {
		out <- chunk
	}
	close(out)
	return out, nil
}

type forbiddenProvider struct{ calls int }

func (*forbiddenProvider) Name() string { return "byok" }
func (p *forbiddenProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	p.calls++
	return nil, newError(ErrorContractInvalid, "BYOK provider must not be used")
}

type synchronousErrorProvider struct {
	calls int
	err   error
}

func (*synchronousErrorProvider) Name() string { return "synchronous-error" }
func (p *synchronousErrorProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	p.calls++
	return nil, p.err
}

func TestMainExecutorDispatcherSubscriptionUsesGatewayAndPreservesFixtureUsage(t *testing.T) {
	usage := NormalizedUsage{InputTokens: int64Pointer(4), OutputTokens: int64Pointer(2), ProviderTotalTokens: int64Pointer(6), RequestCount: int64Pointer(1), BreakdownStatus: UsageExact}
	evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:   MaasEndpointResponses,
		Headers:    map[string][]string{"X-Oneapi-Request-Id": {"gateway_request_01"}},
		FinalJSON:  json.RawMessage(`{"billing":{"quota":1.25},"usage":{"quota":1.250}}`),
		ObservedAt: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointResponses, chunks: []GatewayChunk{
		{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "ok"}},
		{Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: normalizedProviderUsage(usage)}, Usage: &usage, ProviderRequestID: "gateway_request_01", RawUsageRedacted: []byte(`{"safe":true}`)},
		{Chunk: provider.Chunk{Type: provider.ChunkDone}, QuotaEvidence: &evidence},
	}}
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{TaskID: "task_runtime", Scope: TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}, BillingSource: BillingSubscriptionPool, InitialModelID: "model_fixture_01", PricingEvidenceID: "evidence_01"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_runtime"}, Now: func() time.Time { return time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC) }}
	base := &forbiddenProvider{}
	chunks, err := d.Stream(ctx, base, provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "fixture prompt"}}})
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	if gateway.calls != 1 || base.calls != 0 || len(store.prepared) != 1 || len(store.begun) != 1 || len(store.finished) != 1 {
		t.Fatalf("gateway/base/prepared/begun/finished = %d/%d/%d/%d/%d", gateway.calls, base.calls, len(store.prepared), len(store.begun), len(store.finished))
	}
	finished := store.finished[0]
	if store.prepared[0].Endpoint != MaasEndpointResponses || finished.Endpoint != MaasEndpointResponses ||
		finished.Provider != subscriptionGatewayProvider || finished.ProviderRequestID == nil || *finished.ProviderRequestID != "gateway_request_01" ||
		finished.QuotaEvidence == nil || finished.QuotaEvidence.Status != QuotaEvidenceComplete || finished.QuotaEvidence.Source == nil || *finished.QuotaEvidence.Source != QuotaSourceBothConsistent ||
		finished.Usage.BreakdownStatus != UsageExact || *finished.Usage.InputTokens != 4 {
		t.Fatalf("terminal subscription call = %+v", finished)
	}
	if gateway.request.TaskID != "task_runtime" || gateway.request.CallID != "call_runtime" || gateway.request.ModelID != "model_fixture_01" {
		t.Fatalf("gateway request = %+v", gateway.request)
	}
}

func TestMainExecutorDispatcherRejectsConflictingGatewayRequestIDs(t *testing.T) {
	revisions, err := NormalizeQuotaEvidenceRevisions(QuotaObservationInput{
		Endpoint: MaasEndpointChatCompletions,
		Headers: map[string][]string{
			"X-Oneapi-Request-Id": {"request-a"},
			"x-oneapi-request-id": {"request-b"},
		},
		FinalJSON:  json.RawMessage(`{"billing":{"quota":1}}`),
		ObservedAt: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	usage := NormalizedUsage{RequestCount: int64Pointer(1), BreakdownStatus: UsageExact}
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{
		{Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: normalizedProviderUsage(usage)}, Usage: &usage},
		{Chunk: provider.Chunk{Type: provider.ChunkDone}, QuotaEvidenceRevisions: revisions},
	}}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_request_conflict"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_request_conflict"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var terminal error
	for chunk := range out {
		if chunk.Type == provider.ChunkError {
			terminal = chunk.Err
		}
	}
	if ErrorCodeOf(terminal) != ErrorContractInvalid || len(store.finished) != 1 {
		t.Fatalf("terminal/finished = %v/%+v", terminal, store.finished)
	}
	finished := store.finished[0]
	if finished.State != CallFailed || finished.ProviderRequestID == nil || *finished.ProviderRequestID != "request-a" ||
		finished.QuotaEvidence == nil || finished.QuotaEvidence.OneAPIRequestID == nil || *finished.QuotaEvidence.OneAPIRequestID != "request-a" ||
		len(finished.quotaEvidenceRevisions) != 2 || finished.quotaEvidenceRevisions[0].OneAPIRequestID == nil ||
		finished.quotaEvidenceRevisions[1].OneAPIRequestID == nil ||
		*finished.quotaEvidenceRevisions[0].OneAPIRequestID != "request-a" ||
		*finished.quotaEvidenceRevisions[1].OneAPIRequestID != "request-b" {
		t.Fatalf("failed call facts = %+v", finished)
	}
}

func TestMainExecutorDispatcherDoesNotCopyQuotaAcrossConflictingRequestIDs(t *testing.T) {
	evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint: MaasEndpointChatCompletions,
		Headers: map[string][]string{
			"X-Oneapi-Request-Id": {"request-b"},
		},
		FinalJSON:  json.RawMessage(`{"billing":{"quota":50}}`),
		ObservedAt: time.Date(2026, 7, 13, 0, 0, 1, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	usage := NormalizedUsage{RequestCount: int64Pointer(1), BreakdownStatus: UsageExact}
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{
		{
			Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: normalizedProviderUsage(usage)},
			Usage: &usage, ProviderRequestID: "request-a",
		},
		{Chunk: provider.Chunk{Type: provider.ChunkDone}, QuotaEvidence: &evidence},
	}}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_request_evidence_ownership"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_request_evidence_ownership"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var terminal error
	for chunk := range out {
		if chunk.Type == provider.ChunkError {
			terminal = chunk.Err
		}
	}
	if ErrorCodeOf(terminal) != ErrorContractInvalid || len(store.finished) != 1 {
		t.Fatalf("terminal/finished = %v/%+v", terminal, store.finished)
	}
	revisions := store.finished[0].quotaEvidenceRevisions
	if len(revisions) != 2 || revisions[0].OneAPIRequestID == nil || *revisions[0].OneAPIRequestID != "request-a" ||
		revisions[0].Status != QuotaEvidenceMissing || revisions[0].BillingQuotaRaw.Present || revisions[0].UsageQuotaRaw.Present ||
		revisions[0].Quota != nil || revisions[1].OneAPIRequestID == nil || *revisions[1].OneAPIRequestID != "request-b" ||
		revisions[1].Status != QuotaEvidenceComplete || revisions[1].Quota == nil || *revisions[1].Quota != "50" {
		t.Fatalf("request-owned evidence revisions = %+v", revisions)
	}
}

func TestMainExecutorDispatcherPreservesSameRequestIDDifferingEvidence(t *testing.T) {
	first := dispatcherQuotaEvidence(t, MaasEndpointChatCompletions, "request-same")
	second, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint:   MaasEndpointChatCompletions,
		Headers:    map[string][]string{"X-Oneapi-Request-Id": {"request-same"}},
		FinalJSON:  json.RawMessage(`{"usage":{"quota":2}}`),
		ObservedAt: first.ObservedAt.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	second.Revision = 2
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{{
		Chunk: provider.Chunk{Type: provider.ChunkDone}, QuotaEvidenceRevisions: []QuotaEvidenceRevision{first, second},
	}}}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_evidence_conflict"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_evidence_conflict"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var terminal error
	for chunk := range out {
		if chunk.Type == provider.ChunkError {
			terminal = chunk.Err
		}
	}
	if ErrorCodeOf(terminal) != ErrorContractInvalid || len(store.finished) != 1 {
		t.Fatalf("terminal/finished = %v/%+v", terminal, store.finished)
	}
	got := store.finished[0].quotaEvidenceRevisions
	if len(got) != 2 || got[0].Quota == nil || *got[0].Quota != "1" || got[1].Quota == nil || *got[1].Quota != "2" {
		t.Fatalf("audited revisions = %+v", got)
	}
}

func TestGatewayChunkRejectsDuplicateEvidenceProjectionAcrossRevisions(t *testing.T) {
	first := dispatcherQuotaEvidence(t, MaasEndpointChatCompletions, "request-duplicate")
	second := *cloneQuotaEvidence(&first)
	second.Revision = 2
	second.ObservedAt = second.ObservedAt.Add(time.Second)
	chunk := GatewayChunk{
		Chunk:                  provider.Chunk{Type: provider.ChunkDone},
		QuotaEvidenceRevisions: []QuotaEvidenceRevision{first, second},
	}
	if err := chunk.Validate(); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("duplicate projection error = %v", err)
	}
}

func TestMainExecutorDispatcherRejectsQuotaEvidenceBeforeDone(t *testing.T) {
	evidence := dispatcherQuotaEvidence(t, MaasEndpointChatCompletions, "request-early")
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{{
		Chunk: provider.Chunk{Type: provider.ChunkText, Text: "partial"}, QuotaEvidence: &evidence,
	}}}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_early_quota"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_early_quota"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var terminal error
	for chunk := range out {
		if chunk.Type == provider.ChunkError {
			terminal = chunk.Err
		}
	}
	if ErrorCodeOf(terminal) != ErrorContractInvalid || len(store.finished) != 1 || store.finished[0].State != CallFailed {
		t.Fatalf("terminal/finished = %v/%+v", terminal, store.finished)
	}
}

func TestMainExecutorDispatcherClosedStreamHasNoCompleteQuotaEvidence(t *testing.T) {
	usage := NormalizedUsage{RequestCount: int64Pointer(1), BreakdownStatus: UsageExact}
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: normalizedProviderUsage(usage)}, Usage: &usage, ProviderRequestID: "request-without-done",
	}}}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_without_done"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_without_done"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	for range out {
	}
	if len(store.finished) != 1 || store.finished[0].State != CallCompleted || store.finished[0].QuotaEvidence != nil {
		t.Fatalf("closed stream terminal facts = %+v", store.finished)
	}
}

func TestMainExecutorDispatcherRequiresExplicitSubscriptionEndpoint(t *testing.T) {
	store := &recordingCallStore{}
	gateway := &endpointlessGateway{}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_endpoint_required"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_endpoint_required"}}
	if _, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{}); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("missing endpoint error = %v", err)
	}
	if gateway.calls != 0 || len(store.prepared) != 0 {
		t.Fatalf("gateway/prepared = %d/%+v", gateway.calls, store.prepared)
	}
}

func TestMainExecutorDispatcherAcceptsRepeatedIdenticalGatewayIdentity(t *testing.T) {
	evidence := dispatcherQuotaEvidence(t, MaasEndpointChatCompletions, "request-repeat")
	usage := NormalizedUsage{RequestCount: int64Pointer(1), BreakdownStatus: UsageExact}
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{
		{Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: normalizedProviderUsage(usage)}, Usage: &usage, ProviderRequestID: "request-repeat"},
		{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "ok"}, ProviderRequestID: "request-repeat"},
		{Chunk: provider.Chunk{Type: provider.ChunkDone}, QuotaEvidence: &evidence},
	}}
	ctx, err := WithTaskContext(context.Background(), dispatcherSubscriptionTask(t, "task_request_repeat"))
	if err != nil {
		t.Fatal(err)
	}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_request_repeat"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	for range out {
	}
	if len(store.finished) != 1 || store.finished[0].State != CallCompleted ||
		store.finished[0].ProviderRequestID == nil || *store.finished[0].ProviderRequestID != "request-repeat" ||
		store.finished[0].QuotaEvidence == nil || store.finished[0].QuotaEvidence.Status != QuotaEvidenceComplete {
		t.Fatalf("completed call facts = %+v", store.finished)
	}
}

func TestMainExecutorDispatcherBYOKNeverAcceptsMaasQuotaFacts(t *testing.T) {
	store := &recordingCallStore{}
	ctx, err := WithTaskContext(context.Background(), dispatcherBYOKTask(t))
	if err != nil {
		t.Fatal(err)
	}
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{
		{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}},
		{Type: provider.ChunkDone},
	}}
	d := MainExecutorDispatcher{Calls: store, Gateway: &recordingGateway{endpoint: MaasEndpointResponses}, IDs: dispatcherIDGenerator{id: "call_byok_no_quota"}}
	out, err := d.Stream(ctx, base, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	for range out {
	}
	if len(store.finished) != 1 || store.finished[0].Endpoint != "" || store.finished[0].QuotaEvidence != nil {
		t.Fatalf("BYOK terminal call contains Maas quota facts: %+v", store.finished)
	}
}

func TestNormalizeBYOKUsageDoesNotReuseCacheMissForCacheWrite(t *testing.T) {
	usage := &provider.Usage{
		PromptTokens: 13, NormalInputTokens: 8, CacheWriteTokens: 2, CacheReadTokens: 3,
		Presence: provider.UsagePresence{Input: true, NormalInput: true, CacheWrite: true, CacheRead: true},
	}
	got := normalizeBYOKUsage(usage)
	if got.NormalInputTokens == nil || *got.NormalInputTokens != 8 || got.CacheWriteTokens == nil || *got.CacheWriteTokens != 2 || got.CacheReadTokens == nil || *got.CacheReadTokens != 3 || got.InputTokens == nil || *got.InputTokens != 13 {
		t.Fatalf("normalized usage = %+v", got)
	}
}

func TestMainExecutorDispatcherPersistsBeforeForwardingDone(t *testing.T) {
	store := &recordingCallStore{completeStarted: make(chan struct{}), completeRelease: make(chan struct{})}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}}, {Type: provider.ChunkDone}}}
	d := MainExecutorDispatcher{Calls: store, IDs: dispatcherIDGenerator{id: "call_done"}}
	out, err := d.Stream(ctx, base, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if first := <-out; first.Type != provider.ChunkUsage {
		t.Fatalf("first chunk = %#v", first)
	}
	select {
	case <-store.completeStarted:
	case <-time.After(time.Second):
		t.Fatal("terminal persistence did not start")
	}
	select {
	case chunk := <-out:
		t.Fatalf("Done escaped before terminal persistence: %#v", chunk)
	default:
	}
	close(store.completeRelease)
	if done := <-out; done.Type != provider.ChunkDone {
		t.Fatalf("chunk after persistence = %#v", done)
	}
}

func TestMainExecutorDispatcherCompleteFailureReturnsChunkErrorAndMarksRecovery(t *testing.T) {
	want := errors.New("disk full")
	store := &recordingCallStore{completeErr: want}
	tasks := &recordingTaskStore{}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkDone}}}
	d := MainExecutorDispatcher{Calls: store, Tasks: tasks, IDs: dispatcherIDGenerator{id: "call_failed_complete"}}
	out, err := d.Stream(ctx, base, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	chunk := <-out
	if chunk.Type != provider.ChunkError || !errors.Is(chunk.Err, want) {
		t.Fatalf("terminal chunk = %#v", chunk)
	}
	if code := ErrorCodeOf(chunk.Err); code != ErrorUsagePersistFailed {
		t.Fatalf("terminal error code = %q, want %q", code, ErrorUsagePersistFailed)
	}
	var typed *terminalPersistenceError
	if !errors.As(chunk.Err, &typed) {
		t.Fatalf("terminal error type = %T, want terminalPersistenceError", chunk.Err)
	}
	if len(tasks.updates) != 1 || tasks.updates[0].To != TaskRecoveryRequired || tasks.updates[0].LastErrorCode == nil || *tasks.updates[0].LastErrorCode != string(ErrorUsagePersistFailed) {
		t.Fatalf("task updates = %#v", tasks.updates)
	}
	if chunk = <-out; chunk.Type == provider.ChunkDone {
		t.Fatal("Done must not follow failed terminal persistence")
	}
}

func TestMainExecutorDispatcherPreservesRecoveryFailureWithoutNormalTaskTerminal(t *testing.T) {
	completeErr := errors.New("terminal write failed")
	recoveryErr := errors.New("recovery write failed")
	calls := &recordingCallStore{completeErr: completeErr}
	tasks := &recordingTaskStore{updateErr: recoveryErr, failUpdateTo: TaskRecoveryRequired}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := MainExecutorDispatcher{Calls: calls, Tasks: tasks, IDs: dispatcherIDGenerator{id: "call_recovery_failure"}}
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkDone}}}
	lifecycle := TaskLifecycleRunner{Tasks: tasks}

	err = lifecycle.RunTask(ctx, func(ctx context.Context) error {
		out, streamErr := dispatcher.Stream(ctx, base, provider.Request{})
		if streamErr != nil {
			return streamErr
		}
		for chunk := range out {
			if chunk.Type == provider.ChunkError {
				return chunk.Err
			}
		}
		return errors.New("missing terminal persistence error")
	})
	if !errors.Is(err, completeErr) || !errors.Is(err, recoveryErr) {
		t.Fatalf("terminal error = %v, want complete and recovery causes", err)
	}
	var typed *terminalPersistenceError
	if !errors.As(err, &typed) {
		t.Fatalf("terminal error type = %T, want terminalPersistenceError", err)
	}
	if code := ErrorCodeOf(err); code != ErrorUsagePersistFailed {
		t.Fatalf("terminal error code = %q, want %q", code, ErrorUsagePersistFailed)
	}
	if tasks.getCalls != 0 {
		t.Fatalf("task Get calls = %d, want 0", tasks.getCalls)
	}
	if len(tasks.updates) != 2 || tasks.updates[0].To != TaskRunning || tasks.updates[1].To != TaskRecoveryRequired || tasks.updates[1].LastErrorCode == nil || *tasks.updates[1].LastErrorCode != string(ErrorUsagePersistFailed) {
		t.Fatalf("task updates = %#v, want begin and recovery attempt only", tasks.updates)
	}
}

func TestTaskLifecycleTerminalPersistenceFailureDoesNotDependOnTaskGet(t *testing.T) {
	completeErr := errors.New("terminal write failed")
	getErr := errors.New("task read failed")
	calls := &recordingCallStore{completeErr: completeErr}
	tasks := &recordingTaskStore{getErr: getErr}
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := MainExecutorDispatcher{Calls: calls, Tasks: tasks, IDs: dispatcherIDGenerator{id: "call_get_failure"}}
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkDone}}}
	lifecycle := TaskLifecycleRunner{Tasks: tasks}

	err = lifecycle.RunTask(ctx, func(ctx context.Context) error {
		out, streamErr := dispatcher.Stream(ctx, base, provider.Request{})
		if streamErr != nil {
			return streamErr
		}
		for chunk := range out {
			if chunk.Type == provider.ChunkError {
				return chunk.Err
			}
		}
		return errors.New("missing terminal persistence error")
	})
	if !errors.Is(err, completeErr) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("terminal error = %v, code=%q", err, ErrorCodeOf(err))
	}
	if tasks.getCalls != 0 {
		t.Fatalf("task Get calls = %d, want 0", tasks.getCalls)
	}
	if len(tasks.updates) != 2 || tasks.updates[0].To != TaskRunning || tasks.updates[1].To != TaskRecoveryRequired {
		t.Fatalf("task updates = %#v, want begin and recovery only", tasks.updates)
	}
}

func TestMainExecutorDispatcherCancellationPersistsCancelledWithRecoveryContext(t *testing.T) {
	store := &recordingCallStore{}
	task := dispatcherBYOKTask(t)
	ctx, cancel := context.WithCancel(context.Background())
	ctx, err := WithTaskContext(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	defer close(release)
	base := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkText, Text: "partial"}}, hold: release}
	d := MainExecutorDispatcher{Calls: store, IDs: dispatcherIDGenerator{id: "call_cancel"}}
	out, err := d.Stream(ctx, base, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	<-out
	cancel()
	for range out {
	}
	if len(store.finished) != 1 || store.finished[0].State != CallCancelled {
		t.Fatalf("terminal calls = %#v", store.finished)
	}
	if len(store.completeCtxErrs) != 1 || store.completeCtxErrs[0] != nil {
		t.Fatalf("terminal persistence inherited cancellation: %#v", store.completeCtxErrs)
	}
}

func TestMainExecutorDispatcherSubscriptionErrorPreservesAcceptedGatewayTerminalFacts(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{TaskID: "task_subscription_error_facts", Scope: TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}, BillingSource: BillingSubscriptionPool, InitialModelID: "model_fixture_01", PricingEvidenceID: "evidence_01"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, chunks: []GatewayChunk{
		{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "partial"}, ProviderRequestID: "gateway-error-request", RawUsageRedacted: []byte(`{"input_tokens":1}`)},
		{Chunk: provider.Chunk{Type: provider.ChunkError, Err: errors.New("fixture stream failed")}},
	}}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_subscription_error_facts"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	for range out {
	}
	if len(store.finished) != 1 {
		t.Fatalf("terminal calls = %#v", store.finished)
	}
	got := store.finished[0]
	if got.State != CallFailed || got.ProviderRequestID == nil || *got.ProviderRequestID != "gateway-error-request" || string(got.RawUsageRedacted) != `{"input_tokens":1}` {
		t.Fatalf("failed subscription terminal facts = %+v", got)
	}
}

func TestMainExecutorDispatcherSubscriptionSynchronousStreamErrorReturnsImmediately(t *testing.T) {
	want := errors.New("subscription stream failed before handoff")
	task := dispatcherSubscriptionTask(t, "task_subscription_synchronous_error")
	ctx, barrier := dispatcherContextWithForegroundBarrier(t, task)
	store := &recordingCallStore{}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions, streamErr: want}
	dispatcher := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_subscription_synchronous_error"}}

	result := runSynchronousDispatch(t, func() (<-chan provider.Chunk, <-chan error, error) {
		return dispatcher.StreamWithTerminalResult(ctx, &forbiddenProvider{}, provider.Request{})
	})
	assertForegroundDispatchFailure(t, barrier, result.err)

	if !errors.Is(result.err, want) {
		t.Fatalf("dispatch error = %v, want synchronous gateway error", result.err)
	}
	if gateway.calls != 1 || len(store.prepared) != 1 || len(store.begun) != 1 || len(store.finished) != 1 {
		t.Fatalf("gateway/prepared/begun/finished = %d/%d/%d/%d", gateway.calls, len(store.prepared), len(store.begun), len(store.finished))
	}
	finished := store.finished[0]
	if finished.State != CallFailed || finished.LastErrorCode == nil || *finished.LastErrorCode != string(ErrorGatewayUnavailable) ||
		finished.Usage.BreakdownStatus != UsageUnavailable {
		t.Fatalf("terminal subscription call = %+v", finished)
	}
}

func TestMainExecutorDispatcherBYOKSynchronousErrorBeforeDispatchReturnsImmediately(t *testing.T) {
	want := errors.New("BYOK stream failed before dispatch")
	task := dispatcherBYOKTask(t)
	ctx, barrier := dispatcherContextWithForegroundBarrier(t, task)
	store := &recordingCallStore{}
	base := &synchronousErrorProvider{err: want}
	dispatcher := MainExecutorDispatcher{Calls: store, IDs: dispatcherIDGenerator{id: "call_byok_synchronous_before_dispatch"}}

	result := runSynchronousDispatch(t, func() (<-chan provider.Chunk, <-chan error, error) {
		return dispatcher.StreamWithTerminalResult(ctx, base, provider.Request{})
	})
	assertForegroundDispatchFailure(t, barrier, result.err)

	if !errors.Is(result.err, want) {
		t.Fatalf("dispatch error = %v, want synchronous BYOK error", result.err)
	}
	if base.calls != 1 || len(store.prepared) != 1 || len(store.begun) != 0 || len(store.finished) != 0 {
		t.Fatalf("provider/prepared/begun/finished = %d/%d/%d/%d", base.calls, len(store.prepared), len(store.begun), len(store.finished))
	}
	if store.prepared[0].State != CallPrepared {
		t.Fatalf("prepared call = %+v", store.prepared[0])
	}
}

func TestMainExecutorDispatcherBYOKSynchronousErrorAfterDispatchReturnsImmediately(t *testing.T) {
	task := dispatcherBYOKTask(t)
	ctx, barrier := dispatcherContextWithForegroundBarrier(t, task)
	store := &recordingCallStore{}
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverCalls++
		http.Error(w, "rejected", http.StatusBadRequest)
	}))
	defer server.Close()
	base := httpDispatcherProvider{client: server.Client(), url: server.URL}
	dispatcher := MainExecutorDispatcher{Calls: store, IDs: dispatcherIDGenerator{id: "call_byok_synchronous_after_dispatch"}}

	result := runSynchronousDispatch(t, func() (<-chan provider.Chunk, <-chan error, error) {
		return dispatcher.StreamWithTerminalResult(ctx, base, provider.Request{})
	})
	assertForegroundDispatchFailure(t, barrier, result.err)

	if result.err == nil {
		t.Fatal("dispatcher accepted synchronous BYOK transport error")
	}
	if serverCalls != 1 || len(store.prepared) != 1 || len(store.begun) != 1 || len(store.finished) != 1 {
		t.Fatalf("network/prepared/begun/finished = %d/%d/%d/%d", serverCalls, len(store.prepared), len(store.begun), len(store.finished))
	}
	finished := store.finished[0]
	if finished.State != CallFailed || finished.LastErrorCode == nil || *finished.LastErrorCode != string(ErrorByokCredentialInvalid) ||
		finished.Usage.BreakdownStatus != UsageUnavailable {
		t.Fatalf("terminal BYOK call = %+v", finished)
	}
}

func TestMainExecutorDispatcherSubscriptionCancellationPreservesAcceptedGatewayTerminalFacts(t *testing.T) {
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{TaskID: "task_subscription_cancel_facts", Scope: TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}, BillingSource: BillingSubscriptionPool, InitialModelID: "model_fixture_01", PricingEvidenceID: "evidence_01"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, err = WithTaskContext(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	store := &recordingCallStore{}
	gateway := &cancellationAwareGateway{endpoint: MaasEndpointChatCompletions, started: make(chan struct{}), done: make(chan struct{}), chunk: GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "partial"}, ProviderRequestID: "gateway-cancel-request", RawUsageRedacted: []byte(`{"input_tokens":1}`)}}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_subscription_cancel_facts"}}
	out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-gateway.started:
	case <-time.After(time.Second):
		t.Fatal("gateway did not start")
	}
	if chunk := <-out; chunk.Type != provider.ChunkText {
		t.Fatalf("first chunk = %#v", chunk)
	}
	cancel()
	for range out {
	}
	select {
	case <-gateway.done:
	case <-time.After(time.Second):
		t.Fatal("ctx-aware gateway producer did not exit")
	}
	if len(store.finished) != 1 {
		t.Fatalf("terminal calls = %#v", store.finished)
	}
	got := store.finished[0]
	if got.State != CallCancelled || got.ProviderRequestID == nil || *got.ProviderRequestID != "gateway-cancel-request" || string(got.RawUsageRedacted) != `{"input_tokens":1}` {
		t.Fatalf("cancelled subscription terminal facts = %+v", got)
	}
}

func TestMainExecutorDispatcherCancellationClosesWithoutTerminalConsumer(t *testing.T) {
	for _, tc := range []struct {
		name string
		new  func(*testing.T, *recordingCallStore) (context.CancelFunc, <-chan provider.Chunk, <-chan struct{})
	}{
		{
			name: "byok",
			new: func(t *testing.T, store *recordingCallStore) (context.CancelFunc, <-chan provider.Chunk, <-chan struct{}) {
				t.Helper()
				ctx, cancel := context.WithCancel(context.Background())
				ctx, err := WithTaskContext(ctx, dispatcherBYOKTask(t))
				if err != nil {
					t.Fatal(err)
				}
				d := MainExecutorDispatcher{Calls: store, IDs: dispatcherIDGenerator{id: "call_cancel_without_consumer_byok"}}
				hold := make(chan struct{})
				producer := &scriptedDispatcherProvider{chunks: []provider.Chunk{{Type: provider.ChunkText, Text: "partial"}}, hold: hold, done: make(chan struct{})}
				out, err := d.Stream(ctx, producer, provider.Request{})
				if err != nil {
					t.Fatal(err)
				}
				return cancel, out, producer.done
			},
		},
		{
			name: "subscription",
			new: func(t *testing.T, store *recordingCallStore) (context.CancelFunc, <-chan provider.Chunk, <-chan struct{}) {
				t.Helper()
				binding := testBindingScope()
				task, err := NewTaskContext(TaskContextInput{TaskID: "task_subscription_cancel_without_consumer", Scope: TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}, BillingSource: BillingSubscriptionPool, InitialModelID: "model_fixture_01", PricingEvidenceID: "evidence_01"})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				ctx, err = WithTaskContext(ctx, task)
				if err != nil {
					t.Fatal(err)
				}
				gateway := &cancellationAwareGateway{endpoint: MaasEndpointChatCompletions, started: make(chan struct{}), done: make(chan struct{}), chunk: GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "partial"}}}
				d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_cancel_without_consumer_subscription"}}
				out, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{})
				if err != nil {
					t.Fatal(err)
				}
				return cancel, out, gateway.done
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &recordingCallStore{completeFinished: make(chan struct{})}
			cancel, out, producerDone := tc.new(t, store)
			cancel()
			select {
			case <-store.completeFinished:
			case <-time.After(time.Second):
				t.Fatal("terminal call persistence did not finish")
			}
			select {
			case chunk, ok := <-out:
				if ok {
					t.Fatalf("cancelled dispatcher forwarded terminal chunk without a consumer: %#v", chunk)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled dispatcher did not close its output")
			}
			if len(store.finished) != 1 || store.finished[0].State != CallCancelled {
				t.Fatalf("terminal calls = %#v", store.finished)
			}
			select {
			case <-producerDone:
			case <-time.After(time.Second):
				t.Fatal("ctx-aware producer did not exit after cancellation")
			}
		})
	}
}

type observedTaskStore struct {
	*SQLiteTaskStore
	cancelled     chan struct{}
	cancelledOnce sync.Once
}

func (s *observedTaskStore) UpdateState(ctx context.Context, update TaskStateUpdate) error {
	if update.To == TaskCancelled && s.cancelled != nil {
		s.cancelledOnce.Do(func() { close(s.cancelled) })
	}
	return s.SQLiteTaskStore.UpdateState(ctx, update)
}

type gatedCompleteCallStore struct {
	*SQLiteCallStore
	started chan struct{}
	release <-chan struct{}
	err     error
	once    sync.Once
}

func (s *gatedCompleteCallStore) Complete(ctx context.Context, task TaskContext, call ModelCall) error {
	s.once.Do(func() { close(s.started) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if s.err != nil {
		return s.err
	}
	return s.SQLiteCallStore.Complete(ctx, task, call)
}

type cancellationAwareDispatcherProvider struct {
	started chan struct{}
	client  *http.Client
	url     string
}

func (*cancellationAwareDispatcherProvider) Name() string { return "byok" }

func (p *cancellationAwareDispatcherProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	response, err := provider.SendWithRetry(ctx, p.client, provider.SendOptions{Provider: p.Name()}, func(requestCtx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(requestCtx, http.MethodPost, p.url, nil)
	})
	if err != nil {
		return nil, err
	}
	_ = response.Body.Close()
	out := make(chan provider.Chunk)
	go func() {
		defer close(out)
		close(p.started)
		select {
		case out <- provider.Chunk{Type: provider.ChunkText, Text: "partial"}:
		case <-ctx.Done():
			return
		}
		<-ctx.Done()
	}()
	return out, nil
}

func newCancellationAwareDispatcherProvider(t *testing.T) *cancellationAwareDispatcherProvider {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(server.Close)
	return &cancellationAwareDispatcherProvider{started: make(chan struct{}), client: server.Client(), url: server.URL}
}

func readyDispatcherTask(t *testing.T, id string) (*observedTaskStore, *SQLiteCallStore, TaskContext) {
	t.Helper()
	db := openSchemaTestDB(t)
	tasks := &observedTaskStore{SQLiteTaskStore: NewSQLiteTaskStore(db), cancelled: make(chan struct{})}
	calls := NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, id)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskCreated, To: TaskReady, At: storeNow()}); err != nil {
		t.Fatal(err)
	}
	return tasks, calls, storeContext(t, task)
}

func TestTaskLifecycleRunnerWaitsForCancelledCallPersistence(t *testing.T) {
	tasks, calls, task := readyDispatcherTask(t, "task_cancel_waits_for_call")
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	gatedCalls := &gatedCompleteCallStore{SQLiteCallStore: calls, started: make(chan struct{}), release: release}
	provider := newCancellationAwareDispatcherProvider(t)
	dispatcher := &MainExecutorDispatcher{Calls: gatedCalls, Tasks: tasks, IDs: dispatcherIDGenerator{id: "call_cancel_waits_for_call"}}
	executor := agent.New(provider, tool.NewRegistry(), agent.NewSession("system"), agent.Options{StreamDispatcher: dispatcher}, event.Discard)
	runner := TaskLifecycleRunner{Tasks: tasks}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, err := WithTaskContext(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		result <- runner.RunTask(ctx, func(ctx context.Context) error { return executor.Run(ctx, "cancel me") })
	}()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	cancel()
	select {
	case <-gatedCalls.started:
	case <-time.After(time.Second):
		t.Fatal("terminal persistence did not start")
	}
	select {
	case <-result:
		t.Fatal("task runner returned before cancelled call persistence completed")
	case <-tasks.cancelled:
		t.Fatal("task runner marked task cancelled before call persistence completed")
	case <-time.After(time.Second):
	}
	if persisted, err := tasks.Get(context.Background(), task.Scope, task.TaskID); err != nil || persisted.State != TaskRunning {
		t.Fatalf("task while call persistence is blocked = %+v, %v", persisted, err)
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("runner error = %v, want context cancellation", err)
	}
	if persisted, err := tasks.Get(context.Background(), task.Scope, task.TaskID); err != nil || persisted.State != TaskCancelled {
		code := ""
		if persisted.LastErrorCode != nil {
			code = *persisted.LastErrorCode
		}
		persistedCalls, callErr := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
		t.Fatalf("task after cancellation = %+v, %v; code=%q calls=%#v callErr=%v", persisted, err, code, persistedCalls, callErr)
	}
	persistedCalls, err := calls.ListByTask(context.Background(), task.Scope, task.TaskID)
	if err != nil || len(persistedCalls) != 1 || persistedCalls[0].State != CallCancelled {
		t.Fatalf("calls after cancellation = %#v, %v", persistedCalls, err)
	}
}

func TestTaskLifecycleRunnerDoesNotOverwriteRecoveryAfterCancelledCallPersistenceFails(t *testing.T) {
	tasks, calls, task := readyDispatcherTask(t, "task_cancel_recovery")
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	want := errors.New("terminal persistence failed")
	gatedCalls := &gatedCompleteCallStore{SQLiteCallStore: calls, started: make(chan struct{}), release: release, err: want}
	provider := newCancellationAwareDispatcherProvider(t)
	dispatcher := &MainExecutorDispatcher{Calls: gatedCalls, Tasks: tasks, IDs: dispatcherIDGenerator{id: "call_cancel_recovery"}}
	executor := agent.New(provider, tool.NewRegistry(), agent.NewSession("system"), agent.Options{StreamDispatcher: dispatcher}, event.Discard)
	runner := TaskLifecycleRunner{Tasks: tasks}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, err := WithTaskContext(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		result <- runner.RunTask(ctx, func(ctx context.Context) error { return executor.Run(ctx, "cancel me") })
	}()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	cancel()
	select {
	case <-gatedCalls.started:
	case <-time.After(time.Second):
		t.Fatal("terminal persistence did not start")
	}
	select {
	case <-result:
		t.Fatal("task runner returned before recovery was recorded")
	case <-tasks.cancelled:
		t.Fatal("task runner marked task cancelled before recovery was recorded")
	case <-time.After(time.Second):
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-result; !errors.Is(err, want) || ErrorCodeOf(err) != ErrorUsagePersistFailed {
		t.Fatalf("runner error = %v, want original terminal persistence error", err)
	}
	if persisted, err := tasks.Get(context.Background(), task.Scope, task.TaskID); err != nil || persisted.State != TaskRecoveryRequired {
		t.Fatalf("task after failed cancellation persistence = %+v, %v", persisted, err)
	}
}

func TestTaskLifecycleRunnerCancellationPreservesBothTerminalPersistenceFailures(t *testing.T) {
	completeErr := errors.New("terminal persistence failed")
	recoveryErr := errors.New("recovery update failed")
	calls := &recordingCallStore{completeErr: completeErr}
	tasks := &recordingTaskStore{updateErr: recoveryErr, failUpdateTo: TaskRecoveryRequired}
	task := dispatcherBYOKTask(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, err := WithTaskContext(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	provider := newCancellationAwareDispatcherProvider(t)
	dispatcher := &MainExecutorDispatcher{Calls: calls, Tasks: tasks, IDs: dispatcherIDGenerator{id: "call_cancel_both_persistence_failures"}}
	executor := agent.New(provider, tool.NewRegistry(), agent.NewSession("system"), agent.Options{StreamDispatcher: dispatcher}, event.Discard)
	runner := TaskLifecycleRunner{Tasks: tasks}
	result := make(chan error, 1)
	go func() {
		result <- runner.RunTask(ctx, func(ctx context.Context) error { return executor.Run(ctx, "cancel me") })
	}()
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, completeErr) || !errors.Is(err, recoveryErr) {
			t.Fatalf("terminal error = %v, want complete and recovery causes", err)
		}
		if code := ErrorCodeOf(err); code != ErrorUsagePersistFailed {
			t.Fatalf("terminal error code = %q, want %q", code, ErrorUsagePersistFailed)
		}
	case <-time.After(time.Second):
		t.Fatal("task runner did not return after terminal persistence failures")
	}
	if len(tasks.updates) != 2 || tasks.updates[0].To != TaskRunning || tasks.updates[1].To != TaskRecoveryRequired {
		t.Fatalf("task updates = %#v, want running and recovery attempt only", tasks.updates)
	}
	for _, update := range tasks.updates {
		if update.To == TaskCancelled || update.To == TaskFailed {
			t.Fatalf("lifecycle wrote ordinary terminal update: %#v", update)
		}
	}
}

func TestMainExecutorDispatcherBeginAttemptFailurePreventsSubscriptionDispatch(t *testing.T) {
	store := &failingBeginCallStore{recordingCallStore: recordingCallStore{}, err: errors.New("attempt write failed")}
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{TaskID: "task_begin_failure", Scope: TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}, BillingSource: BillingSubscriptionPool, InitialModelID: "model_fixture_01", PricingEvidenceID: "evidence"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &recordingGateway{endpoint: MaasEndpointChatCompletions}
	d := MainExecutorDispatcher{Calls: store, Gateway: gateway, IDs: dispatcherIDGenerator{id: "call_begin_failure"}}
	if _, err := d.Stream(ctx, &forbiddenProvider{}, provider.Request{}); err == nil || gateway.calls != 0 {
		t.Fatalf("err/gateway calls = %v/%d", err, gateway.calls)
	}
}

func TestMainExecutorDispatcherPreparedOrAttemptFailurePreventsBYOKNetwork(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { serverCalls++ }))
	defer server.Close()
	task := dispatcherBYOKTask(t)
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	base := httpDispatcherProvider{client: server.Client(), url: server.URL}
	for _, tc := range []struct {
		name  string
		calls CallStore
	}{
		{"prepare", failingPrepareCallStore{err: errors.New("prepare write failed")}},
		{"attempt", &failingBeginCallStore{recordingCallStore: recordingCallStore{}, err: errors.New("attempt write failed")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := MainExecutorDispatcher{Calls: tc.calls, IDs: dispatcherIDGenerator{id: "call_" + tc.name}}
			if _, err := d.Stream(ctx, base, provider.Request{}); err == nil {
				t.Fatal("dispatcher accepted failed write-ahead state")
			}
			if serverCalls != 0 {
				t.Fatalf("network calls = %d, want 0", serverCalls)
			}
		})
	}
}

func TestMainExecutorDispatcherCompleteFailureMarksPersistedRecoveryAndBlocksNextNetwork(t *testing.T) {
	db := openSchemaTestDB(t)
	tasks := NewSQLiteTaskStore(db)
	storedCalls := NewSQLiteCallStore(db)
	task, evidence := storeTask(BillingNewAPIByok, "task_complete_recovery")
	taskContext := storeContext(t, task)
	if err := tasks.CreateTaskWithEvidence(context.Background(), task, evidence); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskCreated, To: TaskReady, At: storeNow()}); err != nil {
		t.Fatal(err)
	}
	if err := tasks.UpdateState(context.Background(), TaskStateUpdate{Scope: task.Scope, TaskID: task.TaskID, From: TaskReady, To: TaskRunning, At: storeNow()}); err != nil {
		t.Fatal(err)
	}

	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { serverCalls++; w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	d := MainExecutorDispatcher{
		Calls: failingCompleteCallStore{SQLiteCallStore: storedCalls, err: errors.New("terminal write failed")},
		Tasks: tasks,
		IDs:   &sequenceIDGenerator{values: []string{"call_complete_failure", "call_after_recovery"}},
	}
	base := httpDispatcherProvider{client: server.Client(), url: server.URL}
	runCtx, err := WithTaskContext(context.Background(), taskContext)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Stream(runCtx, base, providerRequestForDispatcher())
	if err != nil {
		t.Fatal(err)
	}
	var terminal provider.Chunk
	for chunk := range out {
		terminal = chunk
	}
	if terminal.Type != provider.ChunkError || terminal.Err == nil {
		t.Fatalf("terminal chunk = %#v", terminal)
	}
	if got, err := tasks.Get(context.Background(), task.Scope, task.TaskID); err != nil || got.State != TaskRecoveryRequired {
		t.Fatalf("recovery task = %+v, %v", got, err)
	}
	if serverCalls != 1 {
		t.Fatalf("first network calls = %d", serverCalls)
	}
	if _, err := d.Stream(runCtx, base, providerRequestForDispatcher()); err == nil {
		t.Fatal("recovery task accepted a second call")
	}
	if serverCalls != 1 {
		t.Fatalf("recovery task reached network: %d", serverCalls)
	}
}

type failingBeginCallStore struct {
	recordingCallStore
	err error
}

type failingPrepareCallStore struct{ err error }

func (s failingPrepareCallStore) Prepare(context.Context, TaskContext, ModelCall) error { return s.err }
func (failingPrepareCallStore) BeginAttempt(context.Context, TaskContext, string, time.Time) error {
	return errors.New("unexpected begin")
}
func (failingPrepareCallStore) Complete(context.Context, TaskContext, ModelCall) error {
	return errors.New("unexpected complete")
}
func (failingPrepareCallStore) ListByTask(context.Context, TaskScope, string) ([]ModelCall, error) {
	return nil, nil
}

type failingCompleteCallStore struct {
	*SQLiteCallStore
	err error
}

func (s failingCompleteCallStore) Complete(context.Context, TaskContext, ModelCall) error {
	return s.err
}

type sequenceIDGenerator struct{ values []string }

func (g *sequenceIDGenerator) NewID() (string, error) {
	value := g.values[0]
	g.values = g.values[1:]
	return value, nil
}

type httpDispatcherProvider struct {
	client *http.Client
	url    string
}

func (p httpDispatcherProvider) Name() string { return "http-dispatcher" }
func (p httpDispatcherProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	response, err := provider.SendWithRetry(ctx, p.client, provider.SendOptions{Provider: p.Name()}, func(requestCtx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(requestCtx, http.MethodPost, p.url, nil)
	})
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	out := make(chan provider.Chunk, 2)
	out <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2, Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true}}}
	out <- provider.Chunk{Type: provider.ChunkDone}
	close(out)
	return out, nil
}

func providerRequestForDispatcher() provider.Request { return provider.Request{} }

func (s *failingBeginCallStore) BeginAttempt(context.Context, TaskContext, string, time.Time) error {
	return s.err
}

type scriptedDispatcherProvider struct {
	chunks []provider.Chunk
	hold   <-chan struct{}
	done   chan struct{}
}

func (*scriptedDispatcherProvider) Name() string { return "byok" }
func (p *scriptedDispatcherProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	out := make(chan provider.Chunk, len(p.chunks))
	go func() {
		defer close(out)
		if p.done != nil {
			defer close(p.done)
		}
		for _, chunk := range p.chunks {
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}
		if p.hold != nil {
			select {
			case <-p.hold:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

type cancellationAwareGateway struct {
	endpoint MaasEndpoint
	started  chan struct{}
	done     chan struct{}
	chunk    GatewayChunk
}

func (g *cancellationAwareGateway) EndpointFor(GatewayRequest) (MaasEndpoint, error) {
	if err := g.endpoint.Validate(); err != nil {
		return "", err
	}
	return g.endpoint, nil
}

func (g *cancellationAwareGateway) Stream(ctx context.Context, _ cloud.BindingScope, _ GatewayRequest) (<-chan GatewayChunk, error) {
	out := make(chan GatewayChunk)
	go func() {
		defer close(out)
		defer close(g.done)
		close(g.started)
		select {
		case out <- g.chunk:
		case <-ctx.Done():
			return
		}
		<-ctx.Done()
	}()
	return out, nil
}

func dispatcherBYOKTask(t *testing.T) TaskContext {
	t.Helper()
	task, err := NewTaskContext(TaskContextInput{TaskID: "task_byok", Scope: TaskScope{ActorID: "actor", ProfileID: "profile"}, BillingSource: BillingNewAPIByok, InitialModelID: "model", PricingEvidenceID: "evidence"})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func dispatcherSubscriptionTask(t *testing.T, taskID string) TaskContext {
	t.Helper()
	binding := testBindingScope()
	task, err := NewTaskContext(TaskContextInput{
		TaskID: taskID, Scope: TaskScope{ActorID: "actor", ProfileID: binding.ProfileID, CloudBinding: &binding},
		BillingSource: BillingSubscriptionPool, InitialModelID: "model_fixture_01", PricingEvidenceID: "evidence",
	})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

type synchronousDispatchResult struct {
	chunks         <-chan provider.Chunk
	terminalResult <-chan error
	err            error
}

func runSynchronousDispatch(t *testing.T, dispatch func() (<-chan provider.Chunk, <-chan error, error)) synchronousDispatchResult {
	t.Helper()
	done := make(chan synchronousDispatchResult, 1)
	go func() {
		chunks, terminalResult, err := dispatch()
		done <- synchronousDispatchResult{chunks: chunks, terminalResult: terminalResult, err: err}
	}()
	select {
	case result := <-done:
		return result
	case <-time.After(time.Second):
		t.Fatal("synchronous stream error was not returned immediately")
		return synchronousDispatchResult{}
	}
}

func dispatcherContextWithForegroundBarrier(t *testing.T, task TaskContext) (context.Context, *foregroundCallBarrier) {
	t.Helper()
	ctx, err := WithTaskContext(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	barrier := newForegroundCallBarrier(task.TaskID)
	return withForegroundCallBarrier(ctx, barrier), barrier
}

func assertForegroundDispatchFailure(t *testing.T, barrier *foregroundCallBarrier, dispatchErr error) {
	t.Helper()
	barrier.seal()
	barrier.mu.Lock()
	pending := barrier.pending
	closed := barrier.closed
	barrierErr := barrier.err
	barrier.mu.Unlock()
	if dispatchErr == nil {
		t.Fatal("synchronous stream error was not returned")
	}
	if pending != 0 || !closed {
		t.Fatalf("foreground registration remains pending: pending=%d closed=%t", pending, closed)
	}
	if !errors.Is(barrierErr, dispatchErr) {
		t.Fatalf("foreground barrier error = %v, want dispatch error %v", barrierErr, dispatchErr)
	}
}

func dispatcherQuotaEvidence(t *testing.T, endpoint MaasEndpoint, requestID string) QuotaEvidenceRevision {
	t.Helper()
	evidence, err := NormalizeQuotaEvidence(QuotaObservationInput{
		Endpoint: endpoint, Headers: map[string][]string{"X-Oneapi-Request-Id": {requestID}},
		FinalJSON: json.RawMessage(`{"usage":{"quota":1}}`), ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}
