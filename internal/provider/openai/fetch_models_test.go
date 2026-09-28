package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestFetchModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]string{
				{"id": "z-model", "object": "model"},
				{"id": "gpt-image-1", "object": "model"},
				{"id": "a-model", "object": "model"},
			},
		})
	}))
	defer srv.Close()

	models, err := FetchModels(context.Background(), srv.URL, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("want 3 models, got %d", len(models))
	}
	want := []string{"z-model", "gpt-image-1", "a-model"}
	for i := range want {
		if models[i] != want[i] {
			t.Errorf("want provider order %v, got %v", want, models)
			break
		}
	}
}

func TestFetchModelCatalogPreservesEndpointCapabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
            {"id":" chat-model ","supported_endpoint_types":[" openai ","anthropic","openai"],"unknown":"ignored"},
            {"id":"image-model","supported_endpoint_types":["image-generation","gemini"]},
            {"id":"unknown-model"}
        ]}`))
	}))
	defer srv.Close()

	catalog, err := FetchModelCatalogWithClient(context.Background(), srv.Client(), srv.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	want := []ModelCatalogEntry{
		{ID: "chat-model", SupportedEndpointTypes: []string{"openai", "anthropic"}},
		{ID: "image-model", SupportedEndpointTypes: []string{"image-generation", "gemini"}},
		{ID: "unknown-model", SupportedEndpointTypes: []string{}},
	}
	if !reflect.DeepEqual(catalog, want) {
		t.Fatalf("catalog = %#v, want %#v", catalog, want)
	}
}

func TestFetchModelsIgnoresNonstandardCapabilityMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
            {"id":"string-capability","supported_endpoint_types":"openai"},
            {"id":"object-capability","supported_endpoint_types":{"chat":true}}
        ]}`))
	}))
	defer srv.Close()

	models, err := FetchModelsWithClient(context.Background(), srv.Client(), srv.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"string-capability", "object-capability"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("legacy model IDs = %v, want %v", models, want)
	}
}

func TestFetchModelsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"invalid key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := FetchModels(context.Background(), srv.URL, "bad-key")
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}

func TestFetchModelsEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": nil})
	}))
	defer srv.Close()

	models, err := FetchModels(context.Background(), srv.URL, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("want empty list, got %v", models)
	}
}
