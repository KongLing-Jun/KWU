package modelbilling

import (
	"encoding/json"
	"testing"
	"time"

	"kwu/internal/provider"
)

func TestMaterializeGatewayFixtureReplacesExplicitResponseRequestIDPlaceholder(t *testing.T) {
	const placeholder = "${runtime.call_id}"
	bundle := GatewayFixtureBundle{Version: FixtureVersionB0V1, DependencyID: DependencyCloudGateway, Scope: testBindingScope(), Cases: []GatewayFixtureCase{{
		Name: "runtime_request_id", Endpoint: MaasEndpointChatCompletions, ResponseMode: GatewayResponseStreaming,
		Request: GatewayRequest{TaskID: "fixture_task", CallID: "fixture_call", ModelID: "model_fixture_01", Role: CallRoleMain},
		Chunks: []GatewayFixtureChunk{{Type: "done", QuotaObservation: &GatewayQuotaObservation{
			Headers:    map[string][]string{"X-Oneapi-Request-Id": {placeholder}},
			FinalJSON:  json.RawMessage(`{"billing":{"quota":0},"usage":{"quota":0}}`),
			ObservedAt: time.Date(2026, time.July, 11, 0, 9, 30, 0, time.UTC),
		}}},
	}}}

	materialized, err := MaterializeGatewayFixture(
		bundle,
		GatewayFixtureSelection{FixtureVersion: FixtureVersionB0V1, CaseName: "runtime_request_id"},
		GatewayFixtureIdentity{TaskID: "task_runtime", CallID: "call_runtime"},
	)
	if err != nil {
		t.Fatal(err)
	}
	requestIDs := materialized.Chunks[0].QuotaObservation.Headers["X-Oneapi-Request-Id"]
	if len(requestIDs) != 1 || requestIDs[0] != "call_runtime" {
		t.Fatalf("materialized response request IDs = %v", requestIDs)
	}
	if got := bundle.Cases[0].Chunks[0].QuotaObservation.Headers["X-Oneapi-Request-Id"][0]; got != placeholder {
		t.Fatalf("template request ID was mutated: %q", got)
	}
}

func TestMaterializeGatewayFixtureReplacesOnlyRuntimeIdentity(t *testing.T) {
	bundle := GatewayFixtureBundle{Version: FixtureVersionB0V1, DependencyID: DependencyCloudGateway, Scope: testBindingScope(), Cases: []GatewayFixtureCase{{
		Name: "success", Endpoint: MaasEndpointResponses, ResponseMode: GatewayResponseNonStreaming, Request: GatewayRequest{TaskID: "fixture_task", CallID: "fixture_call", ModelID: "model_fixture_01", Role: CallRoleMain}, Chunks: []GatewayFixtureChunk{{Type: "done"}},
	}}}
	runtimeRequest := provider.Request{MaxTokens: 99}
	materialized, err := MaterializeGatewayFixture(bundle, GatewayFixtureSelection{FixtureVersion: FixtureVersionB0V1, CaseName: "success"}, GatewayFixtureIdentity{TaskID: "task_runtime", CallID: "call_runtime", ProviderRequest: runtimeRequest})
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Endpoint != MaasEndpointResponses || materialized.ResponseMode != GatewayResponseNonStreaming || materialized.Request.TaskID != "task_runtime" || materialized.Request.CallID != "call_runtime" || materialized.Request.ModelID != "model_fixture_01" || materialized.Request.Role != CallRoleMain {
		t.Fatalf("materialized request = %+v", materialized.Request)
	}
	if bundle.Cases[0].Request.TaskID != "fixture_task" || bundle.Cases[0].Request.CallID != "fixture_call" {
		t.Fatalf("template was mutated: %+v", bundle.Cases[0].Request)
	}
	if materialized.Request.ProviderRequest.MaxTokens != 99 {
		t.Fatalf("runtime request was not injected")
	}
}

func TestMaterializeGatewayFixtureRejectsUnknownSelection(t *testing.T) {
	bundle := GatewayFixtureBundle{Version: FixtureVersionB0V1, DependencyID: DependencyCloudGateway, Scope: testBindingScope(), Cases: []GatewayFixtureCase{{
		Name: "success", Endpoint: MaasEndpointChatCompletions, ResponseMode: GatewayResponseStreaming, Request: GatewayRequest{TaskID: "fixture_task", CallID: "fixture_call", ModelID: "model_fixture_01", Role: CallRoleMain}, Chunks: []GatewayFixtureChunk{{Type: "done"}},
	}}}
	_, err := MaterializeGatewayFixture(bundle, GatewayFixtureSelection{FixtureVersion: FixtureVersionB0V1, CaseName: "missing"}, GatewayFixtureIdentity{TaskID: "task_runtime", CallID: "call_runtime"})
	if ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("error = %v", err)
	}
}
