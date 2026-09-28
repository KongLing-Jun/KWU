package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type connectorOAuthTestExecutor struct{}

func (connectorOAuthTestExecutor) Do(ctx context.Context, _ BindingScope, request func(context.Context, string) error) error {
	return request(ctx, "reasonix-access")
}

func TestHTTPConnectorOAuthClientStartAndComplete(t *testing.T) {
	flowID := "coaf_01KZXTEST"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer reasonix-access" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request auth or method: %s / %s", request.Header.Get("Authorization"), request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		data := any(nil)
		switch request.URL.Path {
		case githubConnectorOAuthStartPath:
			data = ConnectorOAuthStart{FlowID: flowID, AuthorizationURL: "https://github.com/login/oauth/authorize?state=test", ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}
		case "/api/v1/connectors/oauth/github/flows/" + flowID + "/complete":
			data = ConnectorOAuthCompletion{Status: "completed", AccessToken: "github-access", TokenType: "Bearer", Scope: "repo,read:user"}
		default:
			http.NotFound(writer, request)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": data, "error": nil, "request_id": "req_connector_oauth", "server_time": time.Now().UTC(),
		})
	}))
	defer server.Close()
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-connector-oauth", ClientVersion: "dev"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewHTTPConnectorOAuthClient(connectorOAuthTestExecutor{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	scope := BindingScope{ProfileID: "default", CloudUserID: "usr_one", BindingGeneration: 1}
	started, err := client.StartGitHub(context.Background(), scope)
	if err != nil || started.FlowID != flowID {
		t.Fatalf("start = %+v, err = %v", started, err)
	}
	completed, err := client.CompleteGitHub(context.Background(), scope, flowID)
	if err != nil || completed.Status != "completed" || completed.AccessToken != "github-access" || completed.TokenType != "bearer" {
		t.Fatalf("complete = %+v, err = %v", completed, err)
	}
}

func TestHTTPConnectorOAuthClientRejectsCredentialInPendingResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data":  map[string]any{"status": "pending", "access_token": "must-not-appear"},
			"error": nil, "request_id": "req_pending", "server_time": time.Now().UTC(),
		})
	}))
	defer server.Close()
	transport, _ := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-connector-oauth", ClientVersion: "dev"}, nil)
	client, _ := NewHTTPConnectorOAuthClient(connectorOAuthTestExecutor{}, transport)
	_, err := client.CompleteGitHub(context.Background(), BindingScope{ProfileID: "default", CloudUserID: "usr_one", BindingGeneration: 1}, "coaf_test")
	if err == nil || !strings.Contains(err.Error(), "contains credentials") {
		t.Fatalf("pending response error = %v", err)
	}
}
