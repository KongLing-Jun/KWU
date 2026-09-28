package modelbilling

import "testing"

func TestEmbeddedGatewayFixtureProvidesMainSubscriptionCatalog(t *testing.T) {
	bundle, err := LoadEmbeddedGatewayFixture()
	if err != nil {
		t.Fatal(err)
	}
	models, err := FixtureSubscriptionModels(bundle)
	if err != nil || len(models) != 1 || models[0] != "model_fixture_01" {
		t.Fatalf("models/err = %v/%v", models, err)
	}
	endpoints := map[MaasEndpoint]bool{}
	for _, fixture := range bundle.Cases {
		endpoints[fixture.Endpoint] = true
	}
	if !endpoints[MaasEndpointChatCompletions] || !endpoints[MaasEndpointResponses] {
		t.Fatalf("gateway fixture endpoints = %v", endpoints)
	}
}
