package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	managedconnector "kwu/desktop/internal/connector"
	"kwu/desktop/internal/mcpoauth"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/plugin"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

type memoryConnectorCredentialStore struct {
	mu     sync.Mutex
	values map[string]string
}

type flakyConnectorCredentialStore struct {
	delegate *memoryConnectorCredentialStore
	mu       sync.Mutex
	failures map[string]int
	gets     map[string]int
}

func newFlakyConnectorCredentialStore(delegate *memoryConnectorCredentialStore) *flakyConnectorCredentialStore {
	return &flakyConnectorCredentialStore{delegate: delegate, failures: map[string]int{}, gets: map[string]int{}}
}

func (s *flakyConnectorCredentialStore) Set(owner config.CredentialOwner, key, value string) error {
	return s.delegate.Set(owner, key, value)
}

func (s *flakyConnectorCredentialStore) Get(owner config.CredentialOwner, key string) (string, bool, error) {
	fullKey := memoryCredentialKey(owner, key)
	s.mu.Lock()
	s.gets[fullKey]++
	if s.failures[fullKey] > 0 {
		s.failures[fullKey]--
		s.mu.Unlock()
		return "", false, errors.New("temporary keyring read failure")
	}
	s.mu.Unlock()
	return s.delegate.Get(owner, key)
}

func (s *flakyConnectorCredentialStore) Delete(owner config.CredentialOwner, key string) error {
	return s.delegate.Delete(owner, key)
}

func (s *flakyConnectorCredentialStore) fail(owner config.CredentialOwner, key string, count int) {
	s.mu.Lock()
	s.failures[memoryCredentialKey(owner, key)] = count
	s.mu.Unlock()
}

func (s *flakyConnectorCredentialStore) getCount(owner config.CredentialOwner, key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gets[memoryCredentialKey(owner, key)]
}

func newMemoryConnectorCredentialStore() *memoryConnectorCredentialStore {
	return &memoryConnectorCredentialStore{values: map[string]string{}}
}

func memoryCredentialKey(owner config.CredentialOwner, key string) string {
	return config.CredentialKey(owner, key)
}

func (s *memoryConnectorCredentialStore) Set(owner config.CredentialOwner, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[memoryCredentialKey(owner, key)] = value
	return nil
}

func (s *memoryConnectorCredentialStore) Get(owner config.CredentialOwner, key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, found := s.values[memoryCredentialKey(owner, key)]
	return value, found, nil
}

func (s *memoryConnectorCredentialStore) Delete(owner config.CredentialOwner, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, memoryCredentialKey(owner, key))
	return nil
}

func (s *memoryConnectorCredentialStore) raw(owner config.CredentialOwner, key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[memoryCredentialKey(owner, key)]
}

func newConnectorOAuthRefreshTestApp(t *testing.T) (*App, userctx.ActorContext, managedconnector.OwnerScope, *memoryConnectorCredentialStore) {
	t.Helper()
	isolateDesktopUserDirs(t)
	t.Setenv("KWU_CLOUD_BASE_URL", "")
	t.Setenv("KWU_CLOUD_FAKE", "1")
	app := NewApp()
	if app.cloudRuntime == nil || app.cloudRuntime.bindings == nil {
		t.Fatal("test cloud binding repository is unavailable")
	}
	actor := userctx.Default("")
	if _, err := app.cloudRuntime.bindings.ConfirmBinding(context.Background(), actor, "oauth-user"); err != nil {
		t.Fatal(err)
	}
	scope, err := app.connectorOwnerScopeForActor(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	credentials := newMemoryConnectorCredentialStore()
	app.connectorCredentials = credentials
	return app, actor, scope, credentials
}

func oauthTestCredential(serverURL string) connectorOAuthCredential {
	return connectorOAuthCredential{
		Version: 1, AccessToken: "old-access", RefreshToken: "old-refresh", TokenType: "Bearer",
		Scope: "docs.read", ExpiresAt: time.Now().UTC().Add(-time.Minute), ClientID: "public-client",
		Issuer: serverURL, Resource: serverURL + "/mcp", TokenEndpoint: serverURL + "/token",
	}
}

func oauthTestServer(id, serverURL string) managedconnector.MCPServer {
	return managedconnector.MCPServer{
		ID: id, Transport: "streamable-http", URL: serverURL + "/mcp",
		Auth: managedconnector.MCPAuth{Type: "oauth", Resource: serverURL + "/mcp"},
	}
}

type flakyConnectorBindingResolver struct {
	delegate cloud.BindingResolver
	mu       sync.Mutex
	failures int
	calls    int
}

func (r *flakyConnectorBindingResolver) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (cloud.ProfileBinding, error) {
	r.mu.Lock()
	r.calls++
	if r.failures > 0 {
		r.failures--
		r.mu.Unlock()
		return cloud.ProfileBinding{}, errors.New("temporary binding read failure")
	}
	r.mu.Unlock()
	return r.delegate.ResolveBinding(ctx, actor)
}

func TestConnectorOAuthRuntimeHeadersRetryTransientKeyringReadDuringPluginInitialize(t *testing.T) {
	app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
	flaky := newFlakyConnectorCredentialStore(credentials)
	app.connectorCredentials = flaky
	app.connectorOAuthRetryDelays = []time.Duration{0, 0}
	credential := oauthTestCredential("https://oauth.example.com")
	credential.ExpiresAt = time.Now().UTC().Add(time.Hour)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
		t.Fatal(err)
	}
	owner := connectorCredentialOwner(scope)
	key := connectorOAuthKey("docs", "main")
	flaky.fail(owner, key, 1)

	mcpServer := desktopMCPHTTPServer(t)
	defer mcpServer.Close()
	host := plugin.NewHost()
	defer host.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tools, err := host.Add(ctx, plugin.Spec{
		Name: "docs--main", Type: "http", URL: mcpServer.URL,
		RuntimeHeaders: app.connectorOAuthRuntimeHeaders(actor, scope, "docs", oauthTestServer("main", "https://oauth.example.com")),
	})
	if err != nil || len(tools) == 0 {
		t.Fatalf("plugin initialize after transient read: tools=%d err=%v", len(tools), err)
	}
	if got := flaky.getCount(owner, key); got < 2 {
		t.Fatalf("keyring gets = %d, want retry", got)
	}
	if credentials.raw(owner, key) == "" {
		t.Fatal("transient read removed the credential")
	}
}

func TestConnectorOAuthRuntimeHeadersRetryTransientBindingRead(t *testing.T) {
	app, actor, scope, _ := newConnectorOAuthRefreshTestApp(t)
	resolver := &flakyConnectorBindingResolver{delegate: app.cloudRuntime.bindings}
	app.connectorBindingResolver = resolver
	app.connectorOAuthRetryDelays = []time.Duration{0, 0}
	credential := oauthTestCredential("https://oauth.example.com")
	credential.ExpiresAt = time.Now().UTC().Add(time.Hour)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
		t.Fatal(err)
	}
	resolver.mu.Lock()
	resolver.failures = 1
	resolver.calls = 0
	resolver.mu.Unlock()

	snapshot, err := app.connectorOAuthRuntimeHeaders(actor, scope, "docs", oauthTestServer("main", "https://oauth.example.com"))(context.Background())
	if err != nil || snapshot.Headers["Authorization"] != "Bearer old-access" {
		t.Fatalf("binding retry snapshot=%v err=%v", snapshot.Headers, err)
	}
	resolver.mu.Lock()
	calls := resolver.calls
	resolver.mu.Unlock()
	if calls < 2 {
		t.Fatalf("binding reads = %d, want retry", calls)
	}
}

func TestConnectorOAuthPersistentReadFailureIsTemporaryAndDoesNotBlockAnotherConnector(t *testing.T) {
	app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
	flaky := newFlakyConnectorCredentialStore(credentials)
	app.connectorCredentials = flaky
	app.connectorOAuthRetryDelays = []time.Duration{0, 0}
	runtimeService := &oauthRefreshRuntime{}
	app.connectorService = runtimeService
	owner := connectorCredentialOwner(scope)
	for _, connectorID := range []string{"canva", "tencent-docs"} {
		credential := oauthTestCredential("https://oauth.example.com")
		credential.ExpiresAt = time.Now().UTC().Add(time.Hour)
		if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, connectorID, "main", credential); err != nil {
			t.Fatal(err)
		}
	}
	canvaKey := connectorOAuthKey("canva", "main")
	flaky.fail(owner, canvaKey, 3)
	_, canvaErr := app.connectorOAuthRuntimeHeaders(actor, scope, "canva", oauthTestServer("main", "https://oauth.example.com"))(context.Background())
	if canvaErr == nil || strings.Contains(strings.ToLower(canvaErr.Error()), "credential is unavailable") {
		t.Fatalf("temporary failure message = %v", canvaErr)
	}
	if runtimeService.currentStatus() != managedconnector.StatusRepairNeeded {
		t.Fatalf("temporary failure status = %s", runtimeService.currentStatus())
	}
	if credentials.raw(owner, canvaKey) == "" {
		t.Fatal("persistent temporary failure removed the credential")
	}
	if _, found := app.connectorOAuthRuntimeFailure(scope, "canva"); !found {
		t.Fatal("temporary runtime failure was not exposed to connector state views")
	}
	tencent, err := app.connectorOAuthRuntimeHeaders(actor, scope, "tencent-docs", oauthTestServer("main", "https://oauth.example.com"))(context.Background())
	if err != nil || tencent.Headers["Authorization"] != "Bearer old-access" {
		t.Fatalf("independent connector failed: headers=%v err=%v", tencent.Headers, err)
	}
	canva, err := app.connectorOAuthRuntimeHeaders(actor, scope, "canva", oauthTestServer("main", "https://oauth.example.com"))(context.Background())
	if err != nil || canva.Headers["Authorization"] != "Bearer old-access" {
		t.Fatalf("connector retry failed: headers=%v err=%v", canva.Headers, err)
	}
	if _, found := app.connectorOAuthRuntimeFailure(scope, "canva"); found || runtimeService.currentStatus() != managedconnector.StatusConnected {
		t.Fatalf("successful retry did not restore connected state: unavailable=%t status=%s", found, runtimeService.currentStatus())
	}
}

func TestConnectorOAuthDiagnosticLogContainsOnlySafeClassification(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	failure := &connectorOAuthFailure{
		Stage: connectorOAuthStageKeyring, Class: connectorOAuthClassKeyringRead, Retryable: true,
		cause: errors.New("access_token=secret-access refresh_token=secret-refresh Authorization=Bearer-secret"),
	}
	logConnectorOAuthFailure("canva", "main", failure, 1, 3, true)
	logged := output.String()
	for _, want := range []string{"stage=keyring", "class=keyring_read", "attempt=1", "max_attempts=3", "retrying=true"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("safe diagnostic log missing %q: %s", want, logged)
		}
	}
	for _, forbidden := range []string{"secret-access", "secret-refresh", "Bearer-secret", "Authorization"} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("diagnostic log contains sensitive value %q: %s", forbidden, logged)
		}
	}
}

func TestConnectorOAuthMissingAndInvalidCredentialRequireReauthorization(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
	}{
		{name: "missing"},
		{name: "invalid", raw: "{invalid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
			app.connectorOAuthRetryDelays = []time.Duration{0, 0}
			manifest := managedconnector.Manifest{
				Connector: managedconnector.ConnectorManifest{ID: "docs", Kind: "mcp", Entrypoints: managedconnector.ManifestEntrypoints{MCP: "mcp.json"}},
				MCP:       &managedconnector.MCPManifest{Servers: []managedconnector.MCPServer{{ID: "main", Transport: "streamable-http", URL: "https://mcp.example.com", Auth: managedconnector.MCPAuth{Type: "oauth"}}}},
			}
			runtimeService := &oauthRefreshRuntime{root: t.TempDir(), manifest: manifest}
			app.connectorService = runtimeService
			owner := connectorCredentialOwner(scope)
			key := connectorOAuthKey("docs", "main")
			if tt.raw != "" {
				if err := credentials.Set(owner, key, tt.raw); err != nil {
					t.Fatal(err)
				}
			}
			_, err := app.connectorOAuthRuntimeHeaders(actor, scope, "docs", oauthTestServer("main", "https://oauth.example.com"))(context.Background())
			if err == nil || !strings.Contains(err.Error(), "authorization is required") {
				t.Fatalf("reauthorization error = %v", err)
			}
			if runtimeService.currentStatus() != managedconnector.StatusAuthorizationNeeded {
				t.Fatalf("reauthorization status = %s", runtimeService.currentStatus())
			}
			if tt.raw != "" && credentials.raw(owner, key) != tt.raw {
				t.Fatal("invalid credential was deleted before explicit reauthorization")
			}
		})
	}
}

func TestConnectorOAuthKnownExpiryRefreshesAndMergesCredential(t *testing.T) {
	tests := []struct {
		name            string
		responseRefresh string
		responseScope   string
		wantRefresh     string
		wantScope       string
	}{
		{name: "rotates refresh token and scope", responseRefresh: "rotated-refresh", responseScope: "docs.read docs.list", wantRefresh: "rotated-refresh", wantScope: "docs.read docs.list"},
		{name: "retains omitted refresh token and scope", wantRefresh: "old-refresh", wantScope: "docs.read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, actor, scope, _ := newConnectorOAuthRefreshTestApp(t)
			var refreshRequests atomic.Int32
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				refreshRequests.Add(1)
				response := map[string]any{"access_token": "new-access", "expires_in": 120}
				if tt.responseRefresh != "" {
					response["refresh_token"] = tt.responseRefresh
				}
				if tt.responseScope != "" {
					response["scope"] = tt.responseScope
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }
			credential := oauthTestCredential(server.URL)
			if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
				t.Fatal(err)
			}

			snapshot, err := app.connectorOAuthRuntimeHeaders(actor, scope, "docs", oauthTestServer("main", server.URL))(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Headers["Authorization"] != "Bearer new-access" || snapshot.Observation == connectorOAuthCredentialObservation(credential) {
				t.Fatal("known-expiry provider did not return the refreshed credential")
			}
			saved, found, err := app.loadConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main")
			if err != nil || !found {
				t.Fatalf("saved credential found=%t err=%v", found, err)
			}
			if saved.AccessToken != "new-access" || saved.RefreshToken != tt.wantRefresh || saved.Scope != tt.wantScope || !saved.ExpiresAt.After(time.Now().UTC().Add(110*time.Second)) || saved.TokenEndpoint != server.URL+"/token" {
				t.Fatalf("unexpected merged credential: access=%t refresh_rotated=%t scope=%q expiry=%v endpoint_saved=%t", saved.AccessToken != "", saved.RefreshToken != "old-refresh", saved.Scope, saved.ExpiresAt, saved.TokenEndpoint != "")
			}
			if refreshRequests.Load() != 1 {
				t.Fatalf("refresh requests = %d", refreshRequests.Load())
			}
		})
	}
}

func TestConnectorOAuthConcurrentRefreshIsSingleFlightPerCredential(t *testing.T) {
	app, actor, scope, _ := newConnectorOAuthRefreshTestApp(t)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(30 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "expires_in": 60})
	}))
	defer server.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }
	credential := oauthTestCredential(server.URL)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
		t.Fatal(err)
	}
	observed := connectorOAuthCredentialObservation(credential)
	serverSpec := oauthTestServer("main", server.URL)

	var wait sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := app.refreshConnectorOAuthCredential(context.Background(), actor, scope, "docs", serverSpec, observed)
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("refresh requests = %d, want 1", requests.Load())
	}
}

func TestConnectorOAuthDifferentCredentialsDoNotBlockEachOther(t *testing.T) {
	app, actor, scope, _ := newConnectorOAuthRefreshTestApp(t)
	var started atomic.Int32
	bothStarted := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if started.Add(1) == 2 {
			close(bothStarted)
		}
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "expires_in": 60})
	}))
	defer server.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }

	errCh := make(chan error, 2)
	for _, serverID := range []string{"one", "two"} {
		credential := oauthTestCredential(server.URL)
		if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", serverID, credential); err != nil {
			t.Fatal(err)
		}
		go func(id, observed string) {
			_, err := app.refreshConnectorOAuthCredential(context.Background(), actor, scope, "docs", oauthTestServer(id, server.URL), observed)
			errCh <- err
		}(serverID, connectorOAuthCredentialObservation(credential))
	}
	select {
	case <-bothStarted:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("different credential refreshes were serialized")
	}
	close(release)
	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

func TestConnectorOAuth401ObservationReusesAlreadyRefreshedCredential(t *testing.T) {
	app, actor, scope, _ := newConnectorOAuthRefreshTestApp(t)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "unexpected", "expires_in": 60})
	}))
	defer server.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }
	oldCredential := oauthTestCredential(server.URL)
	oldCredential.ExpiresAt = time.Now().UTC().Add(time.Hour)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", oldCredential); err != nil {
		t.Fatal(err)
	}
	first, err := app.connectorOAuthRuntimeHeaders(actor, scope, "docs", oauthTestServer("main", server.URL))(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	newCredential := oldCredential
	newCredential.AccessToken = "already-refreshed"
	newCredential.ExpiresAt = time.Now().UTC().Add(2 * time.Hour)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", newCredential); err != nil {
		t.Fatal(err)
	}
	replayed, err := app.connectorOAuthRuntimeRefresher(actor, scope, "docs", oauthTestServer("main", server.URL))(context.Background(), first.Observation)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Headers["Authorization"] != "Bearer already-refreshed" || requests.Load() != 0 {
		t.Fatalf("already-refreshed credential was not reused; endpoint requests=%d", requests.Load())
	}
}

func TestConnectorOAuthRefreshErrorsLeaveCredentialUnchanged(t *testing.T) {
	tests := []struct {
		name        string
		closeServer bool
	}{
		{name: "ordinary service error"},
		{name: "network error", closeServer: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "temporarily_unavailable"})
			}))
			client := server.Client()
			serverURL := server.URL
			if tt.closeServer {
				server.Close()
			} else {
				defer server.Close()
			}
			app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(client), nil }
			credential := oauthTestCredential(serverURL)
			if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
				t.Fatal(err)
			}
			owner := connectorCredentialOwner(scope)
			key := connectorOAuthKey("docs", "main")
			before := credentials.raw(owner, key)
			_, err := app.refreshConnectorOAuthCredential(context.Background(), actor, scope, "docs", oauthTestServer("main", serverURL), connectorOAuthCredentialObservation(credential))
			if err == nil {
				t.Fatal("refresh error was treated as success")
			}
			if after := credentials.raw(owner, key); after != before {
				t.Fatal("refresh error changed the stored credential")
			}
		})
	}
}

func TestConnectorOAuthOldGenerationCannotOverwriteOrDeleteNewFlow(t *testing.T) {
	for _, invalidGrant := range []bool{false, true} {
		name := "successful stale response"
		if invalidGrant {
			name = "stale invalid grant"
		}
		t.Run(name, func(t *testing.T) {
			testConnectorOAuthGenerationChange(t, invalidGrant)
		})
	}
}

func testConnectorOAuthGenerationChange(t *testing.T, invalidGrant bool) {
	t.Helper()
	app, actor, oldScope, _ := newConnectorOAuthRefreshTestApp(t)
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		if invalidGrant {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "stale-response", "refresh_token": "stale-rotation", "expires_in": 60})
	}))
	defer server.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }
	oldCredential := oauthTestCredential(server.URL)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, oldScope, "docs", "main", oldCredential); err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() {
		_, err := app.refreshConnectorOAuthCredential(context.Background(), actor, oldScope, "docs", oauthTestServer("main", server.URL), connectorOAuthCredentialObservation(oldCredential))
		errCh <- err
	}()
	<-started
	if _, err := app.cloudRuntime.bindings.Rebind(context.Background(), actor, "other-user"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.cloudRuntime.bindings.Rebind(context.Background(), actor, oldScope.CloudUserID); err != nil {
		t.Fatal(err)
	}
	newScope, err := app.connectorOwnerScopeForActor(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	newCredential := oauthTestCredential(server.URL)
	newCredential.AccessToken = "new-flow-access"
	newCredential.RefreshToken = "new-flow-refresh"
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, newScope, "docs", "main", newCredential); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-errCh; err == nil {
		t.Fatal("old generation refresh unexpectedly succeeded")
	}
	saved, found, err := app.loadConnectorOAuthCredential(context.Background(), actor, newScope, "docs", "main")
	if err != nil || !found || saved.AccessToken != "new-flow-access" || saved.RefreshToken != "new-flow-refresh" {
		t.Fatalf("new flow credential was overwritten: found=%t err=%v access_ok=%t refresh_ok=%t", found, err, saved.AccessToken == "new-flow-access", saved.RefreshToken == "new-flow-refresh")
	}
}

type oauthRefreshRuntime struct {
	root     string
	manifest managedconnector.Manifest
	mu       sync.Mutex
	status   managedconnector.Status
	safeErr  *managedconnector.SafeError
}

func (*oauthRefreshRuntime) Start(context.Context) error { return nil }
func (*oauthRefreshRuntime) Close() error                { return nil }
func (*oauthRefreshRuntime) Catalog() (managedconnector.Catalog, error) {
	return managedconnector.Catalog{}, nil
}
func (*oauthRefreshRuntime) RefreshCatalog(context.Context) (managedconnector.Catalog, error) {
	return managedconnector.Catalog{}, nil
}
func (*oauthRefreshRuntime) Install(context.Context, string, managedconnector.OwnerScope, managedconnector.ScopeConfirmation) (managedconnector.Snapshot, error) {
	return managedconnector.Snapshot{}, nil
}
func (r *oauthRefreshRuntime) Snapshot(id string, _ managedconnector.OwnerScope) (managedconnector.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return managedconnector.Snapshot{ConnectorID: id, Status: r.status, Error: r.safeErr}, nil
}
func (r *oauthRefreshRuntime) SetOwnerStatus(_ managedconnector.OwnerScope, id string, status managedconnector.Status, safeErr *managedconnector.SafeError) (managedconnector.Snapshot, error) {
	r.mu.Lock()
	r.status = status
	r.safeErr = safeErr
	r.mu.Unlock()
	return managedconnector.Snapshot{ConnectorID: id, Status: status, Error: safeErr}, nil
}
func (r *oauthRefreshRuntime) Manifest(string) (managedconnector.ActiveVersion, string, managedconnector.Manifest, error) {
	return managedconnector.ActiveVersion{}, r.root, r.manifest, nil
}
func (*oauthRefreshRuntime) CatalogItem(string) (managedconnector.CatalogItem, error) {
	return managedconnector.CatalogItem{}, nil
}
func (r *oauthRefreshRuntime) currentStatus() managedconnector.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func TestConnectorOAuthInvalidGrantRevokesExactOwnerConnectorCapabilities(t *testing.T) {
	app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
	invalidServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
	}))
	defer invalidServer.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(invalidServer.Client()), nil }

	mcpServer := desktopMCPHTTPServer(t)
	defer mcpServer.Close()
	connectorID := "docs"
	runtimeNames := []string{managedconnector.RuntimeMCPName(connectorID, "one"), managedconnector.RuntimeMCPName(connectorID, "two")}
	manifest := managedconnector.Manifest{
		Connector: managedconnector.ConnectorManifest{ID: connectorID, Kind: "mcp", Entrypoints: managedconnector.ManifestEntrypoints{MCP: "mcp.json"}},
		MCP: &managedconnector.MCPManifest{Servers: []managedconnector.MCPServer{
			{ID: "one", Transport: "streamable-http", URL: "https://mcp.example.com/one", Auth: managedconnector.MCPAuth{Type: "oauth"}},
			{ID: "two", Transport: "streamable-http", URL: "https://mcp.example.com/two", Auth: managedconnector.MCPAuth{Type: "oauth"}},
		}},
	}
	runtimeService := &oauthRefreshRuntime{root: filepath.Clean(t.TempDir()), manifest: manifest}
	app.connectorService = runtimeService

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	targetHost := plugin.NewHost()
	defer targetHost.Close()
	targetTools := []tool.Tool{}
	for _, runtimeName := range runtimeNames {
		toolsForServer, err := targetHost.Add(ctx, plugin.Spec{Name: runtimeName, Type: "http", URL: mcpServer.URL})
		if err != nil {
			t.Fatal(err)
		}
		targetTools = append(targetTools, toolsForServer...)
	}
	visibleRegistry := tool.NewRegistry()
	detachedRegistry := tool.NewRegistry()
	for _, registered := range targetTools {
		visibleRegistry.Add(registered)
		detachedRegistry.Add(registered)
	}
	visibleController := control.New(control.Options{Actor: actor, Host: targetHost, Registry: visibleRegistry, PluginCtx: context.Background()})
	detachedController := control.New(control.Options{Actor: actor, Host: targetHost, Registry: detachedRegistry, PluginCtx: context.Background()})

	otherActor := userctx.ActorContext{ActorID: "other-actor", ProfileID: "other-profile"}.Normalize()
	if _, err := app.cloudRuntime.bindings.ConfirmBinding(context.Background(), otherActor, "other-oauth-user"); err != nil {
		t.Fatal(err)
	}
	otherScope, err := app.connectorOwnerScopeForActor(context.Background(), otherActor)
	if err != nil {
		t.Fatal(err)
	}
	otherHost := plugin.NewHost()
	defer otherHost.Close()
	otherRegistry := tool.NewRegistry()
	for _, runtimeName := range runtimeNames {
		otherTools, err := otherHost.Add(ctx, plugin.Spec{Name: runtimeName, Type: "http", URL: mcpServer.URL})
		if err != nil {
			t.Fatal(err)
		}
		for _, registered := range otherTools {
			otherRegistry.Add(registered)
		}
	}
	otherController := control.New(control.Options{Actor: otherActor, Host: otherHost, Registry: otherRegistry, PluginCtx: context.Background()})

	app.tabRuntimes = map[string]*WorkspaceTab{
		"visible": {ID: "visible", Ctrl: visibleController},
		"other":   {ID: "other", Ctrl: otherController},
	}
	app.detachedSessions = map[string]*WorkspaceTab{"detached": {ID: "detached", Ctrl: detachedController}}

	failingCredential := oauthTestCredential(invalidServer.URL)
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, connectorID, "one", failingCredential); err != nil {
		t.Fatal(err)
	}
	siblingCredential := failingCredential
	siblingCredential.AccessToken = "sibling-access"
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, connectorID, "two", siblingCredential); err != nil {
		t.Fatal(err)
	}
	otherCredential := failingCredential
	otherCredential.AccessToken = "other-owner-access"
	if err := app.storeConnectorOAuthCredential(context.Background(), otherActor, otherScope, connectorID, "one", otherCredential); err != nil {
		t.Fatal(err)
	}

	_, err = app.refreshConnectorOAuthCredential(context.Background(), actor, scope, connectorID, oauthTestServer("one", invalidServer.URL), connectorOAuthCredentialObservation(failingCredential))
	if !errors.Is(err, mcpoauth.ErrInvalidGrant) {
		t.Fatalf("refresh error = %v, want invalid_grant", err)
	}
	if runtimeService.currentStatus() != managedconnector.StatusAuthorizationNeeded {
		t.Fatalf("connector status = %q", runtimeService.currentStatus())
	}
	if _, found, _ := credentials.Get(connectorCredentialOwner(scope), connectorOAuthKey(connectorID, "one")); found {
		t.Fatal("invalid credential remains stored")
	}
	if _, found, _ := credentials.Get(connectorCredentialOwner(scope), connectorOAuthKey(connectorID, "two")); !found {
		t.Fatal("sibling server credential was deleted")
	}
	if _, found, _ := credentials.Get(connectorCredentialOwner(otherScope), connectorOAuthKey(connectorID, "one")); !found {
		t.Fatal("other owner credential was deleted")
	}
	if visibleRegistry.Len() != 0 || detachedRegistry.Len() != 0 {
		t.Fatalf("target owner tools remain: visible=%v detached=%v", visibleRegistry.Names(), detachedRegistry.Names())
	}
	for _, runtimeName := range runtimeNames {
		if targetHost.HasClient(runtimeName) {
			t.Fatalf("target shared host still has %q", runtimeName)
		}
		if !otherHost.HasClient(runtimeName) {
			t.Fatalf("other owner shared host lost %q", runtimeName)
		}
	}
	if len(otherRegistry.Names()) != len(runtimeNames) {
		t.Fatalf("other owner registry changed: %v", otherRegistry.Names())
	}
}

func TestConnectorOAuthLegacyCredentialRediscoversMatchingIssuer(t *testing.T) {
	app, actor, scope, _ := newConnectorOAuthRefreshTestApp(t)
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+server.URL+`/.well-known/oauth-protected-resource"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/.well-known/oauth-protected-resource":
			_ = json.NewEncoder(w).Encode(map[string]any{"resource": server.URL + "/mcp", "authorization_servers": []string{server.URL}})
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token",
				"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
				"token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"},
			})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "expires_in": 60})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }
	credential := oauthTestCredential(server.URL)
	credential.TokenEndpoint = ""
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
		t.Fatal(err)
	}
	refreshed, err := app.refreshConnectorOAuthCredential(context.Background(), actor, scope, "docs", oauthTestServer("main", server.URL), connectorOAuthCredentialObservation(credential))
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.TokenEndpoint != server.URL+"/token" || refreshed.AccessToken != "new-access" {
		t.Fatalf("legacy discovery result: endpoint_saved=%t access_updated=%t", refreshed.TokenEndpoint != "", refreshed.AccessToken == "new-access")
	}
}

func TestConnectorOAuthLegacyCredentialRejectsIssuerChange(t *testing.T) {
	app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcp":
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+server.URL+`/.well-known/oauth-protected-resource"`)
			w.WriteHeader(http.StatusUnauthorized)
		case "/.well-known/oauth-protected-resource":
			_ = json.NewEncoder(w).Encode(map[string]any{"resource": server.URL + "/mcp", "authorization_servers": []string{server.URL}})
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": server.URL, "authorization_endpoint": server.URL + "/authorize", "token_endpoint": server.URL + "/token",
				"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
				"token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	app.connectorOAuthClient = func(string) (*mcpoauth.Client, error) { return mcpoauth.NewClient(server.Client()), nil }
	credential := oauthTestCredential(server.URL)
	credential.TokenEndpoint = ""
	credential.Issuer = "https://old-issuer.example"
	if err := app.storeConnectorOAuthCredential(context.Background(), actor, scope, "docs", "main", credential); err != nil {
		t.Fatal(err)
	}
	owner := connectorCredentialOwner(scope)
	key := connectorOAuthKey("docs", "main")
	before := credentials.raw(owner, key)
	_, err := app.refreshConnectorOAuthCredential(context.Background(), actor, scope, "docs", oauthTestServer("main", server.URL), connectorOAuthCredentialObservation(credential))
	if err == nil || !strings.Contains(err.Error(), "issuer changed") {
		t.Fatalf("issuer mismatch error = %v", err)
	}
	if credentials.raw(owner, key) != before {
		t.Fatal("issuer mismatch changed the stored credential")
	}
}
