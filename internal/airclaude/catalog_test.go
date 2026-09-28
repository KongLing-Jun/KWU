package airclaude

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestFetchChatModelsUsesCapabilitiesAndPreservesProviderOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-product-key" {
			t.Fatal("model catalog did not use the product credential")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
            {"id":"image-model","supported_endpoint_types":["image-generation","gemini"]},
            {"id":"model-b","supported_endpoint_types":["anthropic","openai"]},
            {"id":" model-a ","supported_endpoint_types":[" openai "]},
			{"id":"mixed-model","supported_endpoint_types":["image-generation","openai"]},
            {"id":"model-b","supported_endpoint_types":["openai"]},
            {"id":"response-only","supported_endpoint_types":["openai-response"]},
            {"id":"unknown-model"},
            {"id":"","supported_endpoint_types":["openai"]}
        ]}`))
	}))
	defer server.Close()

	models, err := fetchChatModels(context.Background(), server.Client(), server.URL+"/v1", "test-product-key")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"model-b", "model-a", "mixed-model"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
}

func TestFetchChatModelsRejectsCatalogWithoutOpenAIEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
            {"id":"image-model","supported_endpoint_types":["image-generation"]},
            {"id":"embedding-model","supported_endpoint_types":["embeddings"]},
            {"id":"unknown-model"}
        ]}`))
	}))
	defer server.Close()

	if _, err := fetchChatModels(context.Background(), server.Client(), server.URL, "test-product-key"); err == nil {
		t.Fatal("catalog without an OpenAI chat model was accepted")
	}
}
