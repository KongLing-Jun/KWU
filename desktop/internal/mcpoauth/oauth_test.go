package mcpoauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"kwu/internal/brand"
)

func TestGenericMCPMetadataDCRPKCEAndTokenSeam(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+server.URL+`/.well-known/oauth-protected-resource"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/.well-known/oauth-protected-resource":
			writeJSON(t, w, map[string]any{
				"resource": server.URL + "/mcp", "authorization_servers": []string{server.URL},
				"scopes_supported": []string{"docs.read"}, "bearer_methods_supported": []string{"header"},
			})
		case "/.well-known/oauth-authorization-server":
			writeJSON(t, w, map[string]any{
				"issuer": server.URL + "/oauth", "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token", "registration_endpoint": server.URL + "/register",
				"scopes_supported": []string{"docs.read"}, "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
				"token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"},
			})
		case "/register":
			var request struct {
				RedirectURIs []string `json:"redirect_uris"`
				Application  string   `json:"application_type"`
				AuthMethod   string   `json:"token_endpoint_auth_method"`
				ClientName   string   `json:"client_name"`
				Scope        string   `json:"scope"`
			}
			if json.NewDecoder(r.Body).Decode(&request) != nil || request.Application != "native" || request.AuthMethod != "none" || request.ClientName != brand.DisplayName {
				t.Fatal("invalid public-client registration request")
			}
			writeJSON(t, w, map[string]any{"client_id": "public-client", "redirect_uris": request.RedirectURIs, "scope": request.Scope})
		case "/token":
			if r.ParseForm() != nil || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code_verifier") != "verifier" {
				t.Fatal("invalid token request")
			}
			writeJSON(t, w, map[string]any{"access_token": "access", "refresh_token": "refresh", "token_type": "Bearer", "scope": "docs.read", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	config := Config{ResourceURL: server.URL + "/mcp", Resource: server.URL + "/mcp", Scopes: []string{"docs.read"}}
	metadata, err := client.Discover(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Issuer != server.URL+"/oauth" {
		t.Fatalf("issuer = %q", metadata.Issuer)
	}
	if !containsAll(metadata.GrantTypesSupported, []string{"authorization_code", "refresh_token"}) {
		t.Fatalf("grant types = %v", metadata.GrantTypesSupported)
	}
	redirect := "http://127.0.0.1:34567" + callbackPath
	registration, err := client.Register(context.Background(), config, metadata, redirect)
	if err != nil || registration.ClientID != "public-client" {
		t.Fatalf("registration = %+v, %v", registration, err)
	}
	authorization, err := AuthorizationURL(config, metadata, registration, redirect, "state", "challenge")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorization)
	if parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("resource") != config.Resource {
		t.Fatalf("authorization URL = %s", authorization)
	}
	before := time.Now()
	token, err := client.Exchange(context.Background(), config, metadata, registration, redirect, "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" || !token.ExpiresAt.After(before.Add(59*time.Minute)) {
		t.Fatalf("token = %+v", token)
	}
}

func TestDiscoveryFallsBackWhenAdvertisedMetadataIsUnavailable(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp/tongzhou-research":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+server.URL+`/.well-known/oauth-protected-resource/mcp/tongzhou/research", scope="research:read"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/.well-known/oauth-protected-resource":
			writeJSON(t, w, map[string]any{
				"resource": server.URL + "/mcp/tongzhou-research", "authorization_servers": []string{server.URL},
				"scopes_supported": []string{"research:read"}, "bearer_methods_supported": []string{"header"},
			})
		case "/.well-known/oauth-authorization-server":
			writeJSON(t, w, map[string]any{
				"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token", "registration_endpoint": server.URL + "/register",
				"scopes_supported": []string{"research:read"}, "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
				"token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	config := Config{ResourceURL: server.URL + "/mcp/tongzhou-research", Resource: server.URL + "/mcp/tongzhou-research", Scopes: []string{"research:read"}}
	metadata, err := NewClient(server.Client()).Discover(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Issuer != server.URL || metadata.RegistrationEndpoint != server.URL+"/register" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestStaticPublicClientDoesNotRequireDCR(t *testing.T) {
	client := NewClient(&http.Client{})
	registration, err := client.Register(context.Background(), Config{ClientID: "declared-client"}, Metadata{}, "http://127.0.0.1:34567"+callbackPath)
	if err != nil || registration.ClientID != "declared-client" {
		t.Fatalf("registration = %+v, %v", registration, err)
	}
}

func TestDCRUsesAuthorizationServerGrantTypes(t *testing.T) {
	redirect := "http://127.0.0.1:34567" + callbackPath
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			RedirectURIs []string `json:"redirect_uris"`
			GrantTypes   []string `json:"grant_types"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.GrantTypes) != 1 || request.GrantTypes[0] != "authorization_code" {
			t.Fatalf("registration grant types = %v", request.GrantTypes)
		}
		writeJSON(t, w, map[string]any{
			"client_id": "authorization-code-client", "redirect_uris": request.RedirectURIs,
			"grant_types": request.GrantTypes, "response_types": []string{"code"}, "token_endpoint_auth_method": "none",
		})
	}))
	defer server.Close()

	registration, err := NewClient(server.Client()).Register(context.Background(), Config{}, Metadata{
		RegistrationEndpoint: server.URL, GrantTypesSupported: []string{"authorization_code"},
	}, redirect)
	if err != nil || registration.ClientID != "authorization-code-client" {
		t.Fatalf("registration = %+v, %v", registration, err)
	}
}

func TestRefreshUsesPublicClientFormAndParsesRotation(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q", got)
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatal("refresh request used HTTP authentication")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		want := url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {"old-refresh"},
			"client_id":     {"public-client"},
			"resource":      {server.URL + "/mcp"},
		}
		if r.Form.Encode() != want.Encode() {
			t.Fatalf("refresh form = %q, want %q", r.Form.Encode(), want.Encode())
		}
		writeJSON(t, w, map[string]any{
			"access_token": "new-access", "refresh_token": "new-refresh",
			"token_type": "Bearer", "scope": "docs.read", "expires_in": 120,
		})
	}))
	defer server.Close()

	before := time.Now().UTC()
	token, err := NewClient(server.Client()).Refresh(
		context.Background(),
		Config{Resource: server.URL + "/mcp"},
		Metadata{TokenEndpoint: server.URL + "/token"},
		Registration{ClientID: "public-client"},
		"old-refresh",
	)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" || token.Scope != "docs.read" || !token.ExpiresAt.After(before.Add(119*time.Second)) {
		t.Fatalf("unexpected refreshed token fields: access=%t refresh=%t scope=%q expiry=%v", token.AccessToken != "", token.RefreshToken != "", token.Scope, token.ExpiresAt)
	}
}

func TestRefreshAllowsOmittedOptionalFields(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Has("resource") {
			t.Fatalf("empty resource was sent: %q", r.Form.Get("resource"))
		}
		writeJSON(t, w, map[string]any{"access_token": "new-access", "expires_in": 60})
	}))
	defer server.Close()

	token, err := NewClient(server.Client()).Refresh(context.Background(), Config{}, Metadata{TokenEndpoint: server.URL}, Registration{ClientID: "public-client"}, "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken != "" || token.Scope != "" || token.TokenType != "Bearer" || token.ExpiresAt.IsZero() {
		t.Fatalf("optional refresh fields were synthesized incorrectly: refresh=%t scope=%q type=%q expiry=%v", token.RefreshToken != "", token.Scope, token.TokenType, token.ExpiresAt)
	}
}

func TestRefreshClassifiesInvalidGrantWithoutExposingResponse(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    map[string]any
		invalid bool
	}{
		{name: "invalid grant", status: http.StatusBadRequest, body: map[string]any{"error": "invalid_grant", "error_description": "sensitive detail"}, invalid: true},
		{name: "ordinary server error", status: http.StatusServiceUnavailable, body: map[string]any{"error": "temporarily_unavailable", "error_description": "sensitive detail"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				writeJSON(t, w, tt.body)
			}))
			defer server.Close()
			_, err := NewClient(server.Client()).Refresh(context.Background(), Config{}, Metadata{TokenEndpoint: server.URL}, Registration{ClientID: "public-client"}, "old-refresh")
			if errors.Is(err, ErrInvalidGrant) != tt.invalid {
				t.Fatalf("Refresh error = %v, invalid_grant = %t", err, errors.Is(err, ErrInvalidGrant))
			}
			if err == nil || strings.Contains(err.Error(), "sensitive detail") {
				t.Fatalf("unsafe refresh error = %v", err)
			}
		})
	}
}

func TestOAuthDiscoveryAllowsEmptyDeclaredScopes(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+server.URL+`/.well-known/oauth-protected-resource"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/.well-known/oauth-protected-resource":
			writeJSON(t, w, map[string]any{"resource": server.URL + "/mcp", "authorization_servers": []string{server.URL}})
		case "/.well-known/oauth-authorization-server":
			writeJSON(t, w, map[string]any{
				"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token",
				"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code"},
				"token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	config := Config{ResourceURL: server.URL + "/mcp", Scopes: []string{}, ClientID: "public-client"}
	metadata, err := NewClient(server.Client()).Discover(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := AuthorizationURL(config, metadata, Registration{ClientID: config.ClientID}, "http://127.0.0.1:34567"+callbackPath, "state", "challenge")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorization)
	if parsed.Query().Has("scope") || parsed.Query().Has("resource") {
		t.Fatalf("optional OAuth parameters were synthesized: %s", authorization)
	}
}

func TestCallbackRejectsWrongStateAndConsumesCodeOnce(t *testing.T) {
	callback, err := StartCallbackListener("expected")
	if err != nil {
		t.Fatal(err)
	}
	defer callback.Close()
	wrong, _ := http.Get(callback.RedirectURI + "?state=wrong&code=bad")
	if wrong.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong-state status = %d", wrong.StatusCode)
	}
	_ = wrong.Body.Close()
	good, _ := http.Get(callback.RedirectURI + "?state=expected&code=ok")
	_ = good.Body.Close()
	code, err := callback.Wait(context.Background())
	if err != nil || code != "ok" {
		t.Fatalf("callback = %q, %v", code, err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrationRejectsConfidentialClientResponse(t *testing.T) {
	raw := `{"client_id":"id","client_secret":"forbidden","redirect_uris":["http://127.0.0.1:1/oauth/connector/callback"],"token_endpoint_auth_method":"none","scope":"a"}`
	if _, err := parseRegistration([]byte(raw), "http://127.0.0.1:1"+callbackPath, []string{"a"}, []string{"authorization_code"}); err == nil || !strings.Contains(err.Error(), "confidential") {
		t.Fatalf("confidential client response was accepted: %v", err)
	}
}

func TestRegistrationAllowsOmittedOptionalScope(t *testing.T) {
	redirect := "http://127.0.0.1:34567" + callbackPath
	raw := `{"client_id":"public-client","redirect_uris":["` + redirect + `"],"token_endpoint_auth_method":"none","grant_types":["authorization_code","refresh_token"],"response_types":["code"]}`
	if clientID, err := parseRegistration([]byte(raw), redirect, []string{"design:meta:read", "design:content:write"}, []string{"authorization_code", "refresh_token"}); err != nil || clientID != "public-client" {
		t.Fatalf("registration = %q, %v", clientID, err)
	}
}

func TestRegistrationRejectsIncompleteReturnedScope(t *testing.T) {
	redirect := "http://127.0.0.1:34567" + callbackPath
	raw := `{"client_id":"public-client","redirect_uris":["` + redirect + `"],"token_endpoint_auth_method":"none","scope":"design:meta:read"}`
	if _, err := parseRegistration([]byte(raw), redirect, []string{"design:meta:read", "design:content:write"}, []string{"authorization_code"}); err == nil {
		t.Fatal("registration with incomplete returned scope was accepted")
	}
}
