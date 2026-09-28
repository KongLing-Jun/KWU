package modelbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func openFixture(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", path, err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestVersionedNormalizedFixturesDecodeStrictly(t *testing.T) {
	pricing, err := DecodePricingFixture(openFixture(t, "testdata/NEWAPI-PRICING-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodePricingFixture: %v", err)
	}
	if pricing.DependencyID != DependencyNewAPIPricing || len(pricing.Cases) < 4 {
		t.Fatalf("pricing fixture = dependency %q cases %d", pricing.DependencyID, len(pricing.Cases))
	}

	status, err := DecodeStatusFixture(openFixture(t, "testdata/NEWAPI-STATUS-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeStatusFixture: %v", err)
	}
	if status.DependencyID != DependencyNewAPIStatus || len(status.Cases) < 2 {
		t.Fatalf("status fixture = dependency %q cases %d", status.DependencyID, len(status.Cases))
	}

	gateway, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeGatewayFixture: %v", err)
	}
	if gateway.DependencyID != DependencyCloudGateway || len(gateway.Cases) < 4 {
		t.Fatalf("gateway fixture = dependency %q cases %d", gateway.DependencyID, len(gateway.Cases))
	}
	if gateway.Cases[0].Endpoint != MaasEndpointChatCompletions || gateway.Cases[0].ResponseMode != GatewayResponseStreaming {
		t.Fatalf("stream fixture endpoint/mode = %q/%q", gateway.Cases[0].Endpoint, gateway.Cases[0].ResponseMode)
	}
	assertExactSplitUsage(t, gateway.Cases[0].Chunks[1].Usage)
	responses := gatewayFixtureCaseByName(t, gateway, "responses_non_stream_success")
	if responses.Name != "responses_non_stream_success" || responses.Endpoint != MaasEndpointResponses || responses.ResponseMode != GatewayResponseNonStreaming {
		t.Fatalf("responses fixture = %+v", responses)
	}

	usage, err := DecodeUsageFixture(openFixture(t, "testdata/CLOUD-USAGE-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeUsageFixture: %v", err)
	}
	if usage.DependencyID != DependencyCloudUsage || len(usage.Cases) < 4 {
		t.Fatalf("usage fixture = dependency %q cases %d", usage.DependencyID, len(usage.Cases))
	}
	assertExactSplitUsage(t, &usage.Cases[0].Partial.Calls[0].Usage)
	assertExactSplitUsage(t, &usage.Cases[2].Final.Calls[0].Usage)

	settlement, err := DecodeSettlementFixture(openFixture(t, "testdata/CLOUD-SETTLEMENT-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeSettlementFixture: %v", err)
	}
	if settlement.DependencyID != DependencyCloudSettlement || len(settlement.Cases) < 4 {
		t.Fatalf("settlement fixture = dependency %q cases %d", settlement.DependencyID, len(settlement.Cases))
	}
}

func assertExactSplitUsage(t *testing.T, usage *NormalizedUsage) {
	t.Helper()
	if usage == nil || usage.BreakdownStatus != UsageExact {
		t.Fatalf("exact split usage = %+v", usage)
	}
	want := map[string]int64{
		"input": 10, "normal": 8, "cached": 2, "cache_read": 2,
		"cache_write": 0, "output": 4,
	}
	got := map[string]*int64{
		"input": usage.InputTokens, "normal": usage.NormalInputTokens,
		"cached": usage.CachedInputTokens, "cache_read": usage.CacheReadTokens,
		"cache_write": usage.CacheWriteTokens, "output": usage.OutputTokens,
	}
	for name, value := range got {
		if value == nil || *value != want[name] {
			t.Fatalf("exact usage %s = %v, want %d", name, value, want[name])
		}
	}
}

func TestFixtureDecoderRejectsUnknownFieldsAndWrongVersion(t *testing.T) {
	unknown := `{"fixture_version":"b0-v1","dependency_id":"NEWAPI-PRICING-01","cases":[],"unknown":true}`
	if _, err := DecodePricingFixture(strings.NewReader(unknown)); err == nil {
		t.Fatal("DecodePricingFixture accepted an unknown field")
	}
	wrongVersion := `{"fixture_version":"latest","dependency_id":"NEWAPI-PRICING-01","cases":[]}`
	if _, err := DecodePricingFixture(strings.NewReader(wrongVersion)); err == nil {
		t.Fatal("DecodePricingFixture accepted an unversioned alias")
	}
}

func TestFixtureDecoderRejectsEmptyAndUnknownErrorCodes(t *testing.T) {
	decoders := []struct {
		name   string
		path   string
		decode func(io.Reader) error
	}{
		{name: "pricing", path: "testdata/NEWAPI-PRICING-01/normalized-v1.json", decode: func(r io.Reader) error { _, err := DecodePricingFixture(r); return err }},
		{name: "status", path: "testdata/NEWAPI-STATUS-01/normalized-v1.json", decode: func(r io.Reader) error { _, err := DecodeStatusFixture(r); return err }},
		{name: "gateway", path: "testdata/CLOUD-GATEWAY-01/normalized-v1.json", decode: func(r io.Reader) error { _, err := DecodeGatewayFixture(r); return err }},
		{name: "usage", path: "testdata/CLOUD-USAGE-01/normalized-v1.json", decode: func(r io.Reader) error { _, err := DecodeUsageFixture(r); return err }},
		{name: "settlement", path: "testdata/CLOUD-SETTLEMENT-01/normalized-v1.json", decode: func(r io.Reader) error { _, err := DecodeSettlementFixture(r); return err }},
	}
	for _, decoder := range decoders {
		data, err := os.ReadFile(decoder.path)
		if err != nil {
			t.Fatalf("read %s: %v", decoder.path, err)
		}
		for _, code := range []string{"", "unknown_fixture_code"} {
			t.Run(decoder.name+"/"+code, func(t *testing.T) {
				corrupted := replaceFirstFixtureErrorCode(t, data, code)
				if err := decoder.decode(bytes.NewReader(corrupted)); err == nil {
					t.Fatalf("decoder accepted error code %q", code)
				}
			})
		}
	}
}

func replaceFirstFixtureErrorCode(t *testing.T, data []byte, code string) []byte {
	t.Helper()
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if !replaceFixtureErrorCode(root, code) {
		t.Fatal("fixture has no error code")
	}
	corrupted, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return corrupted
}

func replaceFixtureErrorCode(value any, code string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, hasCode := typed["code"]; hasCode {
			typed["code"] = code
			return true
		}
		for _, child := range typed {
			if replaceFixtureErrorCode(child, code) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if replaceFixtureErrorCode(child, code) {
				return true
			}
		}
	}
	return false
}

func TestFixtureGatewayStreamsProviderCompatibleChunks(t *testing.T) {
	bundle, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeGatewayFixture: %v", err)
	}
	gateway, err := NewFixtureGateway(bundle)
	if err != nil {
		t.Fatalf("NewFixtureGateway: %v", err)
	}

	stream, err := gateway.Stream(context.Background(), bundle.Scope, bundle.Cases[0].Request)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var chunks []GatewayChunk
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 3 || chunks[0].Chunk.Type != 0 || chunks[1].Chunk.Usage == nil {
		t.Fatalf("streamed chunks = %+v", chunks)
	}
	if chunks[1].ProviderRequestID != "provider_req_fixture" || !json.Valid(chunks[1].RawUsageRedacted) {
		t.Fatalf("usage chunk metadata = request %q raw %s", chunks[1].ProviderRequestID, chunks[1].RawUsageRedacted)
	}
	if chunks[2].QuotaEvidence == nil || chunks[2].QuotaEvidence.Status != QuotaEvidenceComplete ||
		chunks[2].QuotaEvidence.Quota == nil || *chunks[2].QuotaEvidence.Quota != DecimalString("50") ||
		chunks[2].QuotaEvidence.Source == nil || *chunks[2].QuotaEvidence.Source != QuotaSourceBothConsistent {
		t.Fatalf("stream quota evidence = %+v", chunks[2].QuotaEvidence)
	}
	endpoint, err := gateway.EndpointFor(bundle.Cases[0].Request)
	if err != nil || endpoint != MaasEndpointChatCompletions {
		t.Fatalf("stream endpoint/err = %q/%v", endpoint, err)
	}

	otherScope := bundle.Scope
	otherScope.BindingGeneration++
	if _, err := gateway.Stream(context.Background(), otherScope, bundle.Cases[0].Request); ErrorCodeOf(err) != ErrorScopeMismatch {
		t.Fatalf("scope mismatch error = %v", err)
	}
}

func TestFixtureGatewayNormalizesResponsesRawQuotaAndLegalZero(t *testing.T) {
	bundle, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := gatewayFixtureCaseByName(t, bundle, "responses_non_stream_success")
	gateway, err := NewFixtureGateway(bundle)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := gateway.EndpointFor(fixture.Request)
	if err != nil || endpoint != MaasEndpointResponses {
		t.Fatalf("responses endpoint/err = %q/%v", endpoint, err)
	}
	stream, err := gateway.Stream(context.Background(), bundle.Scope, fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	var evidence *QuotaEvidenceRevision
	for chunk := range stream {
		if chunk.QuotaEvidence != nil {
			evidence = chunk.QuotaEvidence
		}
	}
	if evidence == nil || evidence.Status != QuotaEvidenceComplete || evidence.Quota == nil ||
		*evidence.Quota != DecimalString("0") || evidence.OneAPIRequestID == nil || *evidence.OneAPIRequestID != "provider_req_responses" {
		t.Fatalf("responses quota evidence = %+v", evidence)
	}
}

func TestFixtureGatewayCoversQuotaEvidenceMatrixForBothEndpoints(t *testing.T) {
	bundle, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := NewFixtureGateway(bundle)
	if err != nil {
		t.Fatal(err)
	}
	modes := map[MaasEndpoint]map[GatewayResponseMode]bool{}
	for _, fixture := range bundle.Cases {
		if modes[fixture.Endpoint] == nil {
			modes[fixture.Endpoint] = map[GatewayResponseMode]bool{}
		}
		modes[fixture.Endpoint][fixture.ResponseMode] = true
	}
	for _, endpoint := range []MaasEndpoint{MaasEndpointChatCompletions, MaasEndpointResponses} {
		if !modes[endpoint][GatewayResponseStreaming] || !modes[endpoint][GatewayResponseNonStreaming] {
			t.Fatalf("fixture response modes for %s = %v", endpoint, modes[endpoint])
		}
	}
	type expectedEvidence struct {
		endpoint MaasEndpoint
		status   QuotaEvidenceStatus
		source   *QuotaSource
		issue    *QuotaEvidenceIssue
	}
	ptrSource := func(value QuotaSource) *QuotaSource { return &value }
	ptrIssue := func(value QuotaEvidenceIssue) *QuotaEvidenceIssue { return &value }
	expected := map[string]expectedEvidence{}
	for _, endpoint := range []struct {
		prefix string
		value  MaasEndpoint
	}{{"chat", MaasEndpointChatCompletions}, {"responses", MaasEndpointResponses}} {
		expected[endpoint.prefix+"_billing_only"] = expectedEvidence{endpoint.value, QuotaEvidenceComplete, ptrSource(QuotaSourceBilling), nil}
		expected[endpoint.prefix+"_usage_only"] = expectedEvidence{endpoint.value, QuotaEvidenceComplete, ptrSource(QuotaSourceUsage), nil}
		expected[endpoint.prefix+"_quota_missing"] = expectedEvidence{endpoint.value, QuotaEvidenceMissing, nil, ptrIssue(QuotaIssueMissing)}
		expected[endpoint.prefix+"_quota_invalid"] = expectedEvidence{endpoint.value, QuotaEvidenceInvalid, nil, ptrIssue(QuotaIssueInvalid)}
		expected[endpoint.prefix+"_quota_conflicting"] = expectedEvidence{endpoint.value, QuotaEvidenceConflicting, nil, ptrIssue(QuotaIssueConflicting)}
		expected[endpoint.prefix+"_request_id_missing"] = expectedEvidence{endpoint.value, QuotaEvidenceMissing, nil, ptrIssue(QuotaIssueRequestIDMissing)}
	}
	for name, want := range expected {
		fixture := gatewayFixtureCaseByName(t, bundle, name)
		if fixture.Endpoint != want.endpoint {
			t.Fatalf("fixture %s endpoint = %q", name, fixture.Endpoint)
		}
		stream, err := gateway.Stream(context.Background(), bundle.Scope, fixture.Request)
		if err != nil {
			t.Fatalf("fixture %s stream: %v", name, err)
		}
		var evidence *QuotaEvidenceRevision
		for chunk := range stream {
			if chunk.QuotaEvidence != nil {
				evidence = chunk.QuotaEvidence
			}
		}
		if evidence == nil || evidence.Status != want.status {
			t.Fatalf("fixture %s evidence = %+v", name, evidence)
		}
		if want.source != nil && (evidence.Source == nil || *evidence.Source != *want.source) {
			t.Fatalf("fixture %s source = %v", name, evidence.Source)
		}
		if want.issue != nil && !fixtureContainsQuotaIssue(evidence.Issues, *want.issue) {
			t.Fatalf("fixture %s issues = %v", name, evidence.Issues)
		}
	}
}

func fixtureContainsQuotaIssue(issues []QuotaEvidenceIssue, want QuotaEvidenceIssue) bool {
	for _, issue := range issues {
		if issue == want {
			return true
		}
	}
	return false
}

func TestFixtureGatewayPreservesConflictingHeaderRequestIDs(t *testing.T) {
	bundle, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := gatewayFixtureCaseByName(t, bundle, "chat_duplicate_header_request_ids")
	gateway, err := NewFixtureGateway(bundle)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := gateway.Stream(context.Background(), bundle.Scope, fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	chunk := <-stream
	if chunk.QuotaEvidence != nil || len(chunk.QuotaEvidenceRevisions) != 2 ||
		chunk.QuotaEvidenceRevisions[0].OneAPIRequestID == nil || *chunk.QuotaEvidenceRevisions[0].OneAPIRequestID != "req_header_a" ||
		chunk.QuotaEvidenceRevisions[1].OneAPIRequestID == nil || *chunk.QuotaEvidenceRevisions[1].OneAPIRequestID != "req_header_b" {
		t.Fatalf("header request ID revisions = %+v", chunk.QuotaEvidenceRevisions)
	}
}

func gatewayFixtureCaseByName(t *testing.T, bundle GatewayFixtureBundle, name string) GatewayFixtureCase {
	t.Helper()
	for _, fixture := range bundle.Cases {
		if fixture.Name == name {
			return fixture
		}
	}
	t.Fatalf("gateway fixture %q not found", name)
	return GatewayFixtureCase{}
}

func TestFixtureGatewayPreservesRetryableChunkErrorsAndCancellation(t *testing.T) {
	bundle, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeGatewayFixture: %v", err)
	}
	gateway, err := NewFixtureGateway(bundle)
	if err != nil {
		t.Fatalf("NewFixtureGateway: %v", err)
	}
	stream, err := gateway.Stream(context.Background(), bundle.Scope, bundle.Cases[1].Request)
	if err != nil {
		t.Fatalf("Stream retryable case: %v", err)
	}
	chunk := <-stream
	var domainErr *Error
	if chunk.Chunk.Err == nil || !errors.As(chunk.Chunk.Err, &domainErr) || !domainErr.Retryable {
		t.Fatalf("retryable chunk = %+v", chunk)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := gateway.Stream(ctx, bundle.Scope, bundle.Cases[0].Request); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Stream error = %v", err)
	}
}

func TestFixtureFakesCloneTheirInputBundles(t *testing.T) {
	gatewayBundle, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeGatewayFixture: %v", err)
	}
	gateway, err := NewFixtureGateway(gatewayBundle)
	if err != nil {
		t.Fatalf("NewFixtureGateway: %v", err)
	}
	request := gatewayBundle.Cases[0].Request
	gatewayBundle.Cases[0].Chunks[0].Text = "mutated"
	stream, err := gateway.Stream(context.Background(), gatewayBundle.Scope, request)
	if err != nil {
		t.Fatalf("Stream after caller mutation: %v", err)
	}
	if chunk := <-stream; chunk.Chunk.Text != "fixture response" {
		t.Fatalf("fixture gateway retained caller slice: %q", chunk.Chunk.Text)
	}

	usageBundle, err := DecodeUsageFixture(openFixture(t, "testdata/CLOUD-USAGE-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeUsageFixture: %v", err)
	}
	settlements, err := DecodeSettlementFixture(openFixture(t, "testdata/CLOUD-SETTLEMENT-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeSettlementFixture: %v", err)
	}
	client, err := NewFixtureCloudUsageClient(usageBundle, settlements)
	if err != nil {
		t.Fatalf("NewFixtureCloudUsageClient: %v", err)
	}
	report := *usageBundle.Cases[0].Partial
	report.Calls = append([]ModelCallUsage(nil), report.Calls...)
	usageBundle.Cases[0].Partial.Calls[0].ModelID = "mutated"
	if err := client.UploadPartial(context.Background(), usageBundle.Scope, report); err != nil {
		t.Fatalf("UploadPartial after caller mutation: %v", err)
	}
}

func TestFixtureCloudUsageClientIsIdempotentAndScopeBound(t *testing.T) {
	usage, err := DecodeUsageFixture(openFixture(t, "testdata/CLOUD-USAGE-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeUsageFixture: %v", err)
	}
	settlements, err := DecodeSettlementFixture(openFixture(t, "testdata/CLOUD-SETTLEMENT-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeSettlementFixture: %v", err)
	}
	client, err := NewFixtureCloudUsageClient(usage, settlements)
	if err != nil {
		t.Fatalf("NewFixtureCloudUsageClient: %v", err)
	}

	partial := *usage.Cases[0].Partial
	if err := client.UploadPartial(context.Background(), usage.Scope, partial); err != nil {
		t.Fatalf("UploadPartial: %v", err)
	}
	if err := client.UploadPartial(context.Background(), usage.Scope, partial); err != nil {
		t.Fatalf("UploadPartial repeat: %v", err)
	}
	changed := partial
	changed.Calls = append([]ModelCallUsage(nil), partial.Calls...)
	changed.Calls[0].ModelID = "changed"
	if err := client.UploadPartial(context.Background(), usage.Scope, changed); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("changed partial error = %v", err)
	}
	wrongKey := partial
	wrongKey.IdempotencyKey = "kwu_00000000000000000000000000000000"
	if err := client.UploadPartial(context.Background(), usage.Scope, wrongKey); ErrorCodeOf(err) != ErrorIdempotencyConflict {
		t.Fatalf("wrong partial idempotency key error = %v", err)
	}

	final := *usage.Cases[2].Final
	got, err := client.FinalizeUsage(context.Background(), usage.Scope, final)
	if err != nil || got.State != SettlementSettled {
		t.Fatalf("FinalizeUsage = %+v, %v", got, err)
	}
	observed, err := client.GetSettlement(context.Background(), usage.Scope, got.TaskID)
	if err != nil || observed.SettlementID != got.SettlementID {
		t.Fatalf("GetSettlement = %+v, %v", observed, err)
	}
}

func TestUsageFixtureContainsOnlyDeliverableReports(t *testing.T) {
	usage, err := DecodeUsageFixture(openFixture(t, "testdata/CLOUD-USAGE-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeUsageFixture: %v", err)
	}
	for _, fixture := range usage.Cases {
		calls := []ModelCallUsage(nil)
		if fixture.Partial != nil {
			calls = fixture.Partial.Calls
		} else {
			calls = fixture.Final.Calls
		}
		if len(calls) == 0 {
			t.Fatalf("fixture %s contains a zero-call report", fixture.Name)
		}
		for _, call := range calls {
			if call.OccurredAt.IsZero() || call.OccurredAt.Location() != time.UTC ||
				call.Usage.BreakdownStatus != UsageExact || call.Usage.RequestCount == nil || *call.Usage.RequestCount <= 0 {
				t.Fatalf("fixture %s contains a non-deliverable call: %+v", fixture.Name, call)
			}
		}
	}
}

func TestFixtureConstructorsRejectInvalidBundlesDirectly(t *testing.T) {
	gateway, err := DecodeGatewayFixture(openFixture(t, "testdata/CLOUD-GATEWAY-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeGatewayFixture: %v", err)
	}
	wrongHeader := gateway
	wrongHeader.Version = "wrong"
	if _, err := NewFixtureGateway(wrongHeader); err == nil {
		t.Fatal("NewFixtureGateway accepted a wrong bundle header")
	}
	duplicateCall := gateway
	duplicateCall.Cases = append(append([]GatewayFixtureCase(nil), gateway.Cases...), gateway.Cases[0])
	if _, err := NewFixtureGateway(duplicateCall); err == nil {
		t.Fatal("NewFixtureGateway accepted duplicate call_id")
	}
	duplicateName := gateway
	duplicateName.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	duplicateName.Cases[1].Name = duplicateName.Cases[0].Name
	if _, err := NewFixtureGateway(duplicateName); err == nil {
		t.Fatal("NewFixtureGateway accepted duplicate case name")
	}
	untrimmedGatewayName := gateway
	untrimmedGatewayName.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	untrimmedGatewayName.Cases[0].Name = " stream_success "
	if _, err := NewFixtureGateway(untrimmedGatewayName); err == nil {
		t.Fatal("NewFixtureGateway accepted an untrimmed case name")
	}
	invalidEndpoint := gateway
	invalidEndpoint.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	invalidEndpoint.Cases[0].Endpoint = "/v1/guessed"
	if _, err := NewFixtureGateway(invalidEndpoint); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("invalid gateway endpoint error = %v", err)
	}
	conflictingRequestID := gateway
	conflictingRequestID.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	conflictingRequestID.Cases[0].Chunks = append([]GatewayFixtureChunk(nil), gateway.Cases[0].Chunks...)
	conflictingRequestID.Cases[0].Chunks[2].ProviderRequestID = "another-request"
	if _, err := NewFixtureGateway(conflictingRequestID); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("conflicting gateway request ID error = %v", err)
	}
	nonTerminalQuota := gateway
	nonTerminalQuota.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	nonTerminalQuota.Cases[0].Chunks = append([]GatewayFixtureChunk(nil), gateway.Cases[0].Chunks...)
	nonTerminalQuota.Cases[0].Chunks[0].QuotaObservation = nonTerminalQuota.Cases[0].Chunks[2].QuotaObservation
	if _, err := NewFixtureGateway(nonTerminalQuota); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("non-terminal quota observation error = %v", err)
	}
	chunkAfterTerminal := gateway
	chunkAfterTerminal.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	chunkAfterTerminal.Cases[0].Chunks = append(append([]GatewayFixtureChunk(nil), gateway.Cases[0].Chunks...), GatewayFixtureChunk{Type: "text", Text: "late"})
	if _, err := NewFixtureGateway(chunkAfterTerminal); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("chunk after terminal error = %v", err)
	}
	withoutTerminal := gateway
	withoutTerminal.Cases = append([]GatewayFixtureCase(nil), gateway.Cases...)
	withoutTerminal.Cases[0].Chunks = append([]GatewayFixtureChunk(nil), gateway.Cases[0].Chunks[:2]...)
	if _, err := NewFixtureGateway(withoutTerminal); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("missing terminal error = %v", err)
	}

	usage, err := DecodeUsageFixture(openFixture(t, "testdata/CLOUD-USAGE-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeUsageFixture: %v", err)
	}
	settlements, err := DecodeSettlementFixture(openFixture(t, "testdata/CLOUD-SETTLEMENT-01/normalized-v1.json"))
	if err != nil {
		t.Fatalf("DecodeSettlementFixture: %v", err)
	}
	duplicateUsage := usage
	duplicateUsage.Cases = append(append([]UsageFixtureCase(nil), usage.Cases...), usage.Cases[0])
	if _, err := NewFixtureCloudUsageClient(duplicateUsage, settlements); err == nil {
		t.Fatal("NewFixtureCloudUsageClient accepted duplicate idempotency key")
	}
	emptyUsageName := usage
	emptyUsageName.Cases = append([]UsageFixtureCase(nil), usage.Cases...)
	emptyUsageName.Cases[0].Name = ""
	if _, err := NewFixtureCloudUsageClient(emptyUsageName, settlements); err == nil {
		t.Fatal("NewFixtureCloudUsageClient accepted an empty usage case name")
	}
	missingOutcome := usage
	missingOutcome.Cases = append([]UsageFixtureCase(nil), usage.Cases...)
	missingOutcome.Cases[2].Settlement = nil
	missingOutcome.Cases[2].Error = nil
	if _, err := NewFixtureCloudUsageClient(missingOutcome, settlements); err == nil {
		t.Fatal("NewFixtureCloudUsageClient accepted final without settlement/error")
	}
	mismatchedSettlement := usage
	mismatchedSettlement.Cases = append([]UsageFixtureCase(nil), usage.Cases...)
	settlementValue := *mismatchedSettlement.Cases[2].Settlement
	settlementValue.TaskID = "other_task"
	mismatchedSettlement.Cases[2].Settlement = &settlementValue
	if _, err := NewFixtureCloudUsageClient(mismatchedSettlement, settlements); err == nil {
		t.Fatal("NewFixtureCloudUsageClient accepted settlement for another final task")
	}
	duplicateSettlement := settlements
	duplicateSettlement.Cases = append(append([]SettlementFixtureCase(nil), settlements.Cases...), settlements.Cases[0])
	if _, err := NewFixtureCloudUsageClient(usage, duplicateSettlement); err == nil {
		t.Fatal("NewFixtureCloudUsageClient accepted duplicate settlement task_id")
	}
	nilSettlement := settlements
	nilSettlement.Cases = append([]SettlementFixtureCase(nil), settlements.Cases...)
	nilSettlement.Cases[0].Settlement = nil
	if _, err := NewFixtureCloudUsageClient(usage, nilSettlement); err == nil {
		t.Fatal("NewFixtureCloudUsageClient accepted nil settlement without error")
	}
}
