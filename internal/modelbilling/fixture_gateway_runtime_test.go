package modelbilling

import (
	"context"
	"testing"

	"kwu/internal/provider"
)

func TestFixtureSubscriptionModelsUsesOnlyMainFixtureCases(t *testing.T) {
	bundle := GatewayFixtureBundle{Version: FixtureVersionB0V1, DependencyID: DependencyCloudGateway, Scope: testBindingScope(), Cases: []GatewayFixtureCase{
		{Name: "main", Endpoint: MaasEndpointChatCompletions, ResponseMode: GatewayResponseStreaming, Request: GatewayRequest{TaskID: "task", CallID: "call", ModelID: "model_main", Role: CallRoleMain}, Chunks: []GatewayFixtureChunk{{Type: "done"}}},
		{Name: "planner", Endpoint: MaasEndpointResponses, ResponseMode: GatewayResponseNonStreaming, Request: GatewayRequest{TaskID: "task2", CallID: "call2", ModelID: "model_planner", Role: CallRolePlanner}, Chunks: []GatewayFixtureChunk{{Type: "done"}}},
	}}
	models, err := FixtureSubscriptionModels(bundle)
	if err != nil || len(models) != 1 || models[0] != "model_main" {
		t.Fatalf("models/err = %v/%v", models, err)
	}
}

func TestMaterializedFixtureGatewayRequiresStrictNonIdentityRequest(t *testing.T) {
	bundle := GatewayFixtureBundle{Version: FixtureVersionB0V1, DependencyID: DependencyCloudGateway, Scope: testBindingScope(), Cases: []GatewayFixtureCase{{
		Name: "main", Endpoint: MaasEndpointResponses, ResponseMode: GatewayResponseNonStreaming, Request: GatewayRequest{TaskID: "fixture_task", CallID: "fixture_call", ModelID: "model_main", Role: CallRoleMain, ProviderRequest: provider.Request{MaxTokens: 16}}, Chunks: []GatewayFixtureChunk{{Type: "done"}},
	}}}
	gateway, err := NewMaterializedFixtureGateway(bundle, GatewayFixtureSelection{FixtureVersion: FixtureVersionB0V1, CaseName: "main"})
	if err != nil {
		t.Fatal(err)
	}
	request := GatewayRequest{TaskID: "task_runtime", CallID: "call_runtime", ModelID: "model_main", Role: CallRoleMain, ProviderRequest: provider.Request{MaxTokens: 16}}
	endpoint, err := gateway.EndpointFor(request)
	if err != nil || endpoint != MaasEndpointResponses {
		t.Fatalf("endpoint/err = %q/%v", endpoint, err)
	}
	chunks, err := gateway.Stream(context.Background(), testBindingScope(), request)
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	request.ModelID = "other"
	if _, err := gateway.Stream(context.Background(), testBindingScope(), request); ErrorCodeOf(err) != ErrorContractInvalid {
		t.Fatalf("error = %v", err)
	}
}
