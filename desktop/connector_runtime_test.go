package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	managedconnector "kwu/desktop/internal/connector"
	"kwu/internal/cloud"
	"kwu/internal/plugin"
)

type recordingManagedRegistrar struct {
	spec plugin.Spec
}

func TestEmptyConnectorSelectionDoesNotReadCatalog(t *testing.T) {
	for _, selection := range [][]string{nil, {}, {"", "  "}} {
		runtime := &retryableCatalogRuntime{}
		app := &App{connectorService: runtime}
		got, err := app.expandConnectorSelections("missing-tab", nil, selection, selection, "unchanged input")
		if err != nil || got.Input != "unchanged input" || len(got.MCPNames) != 0 || len(got.CommandEnv) != 0 {
			t.Fatalf("empty selection = %+v, %v", got, err)
		}
		if runtime.catalogs != 0 || runtime.refreshes != 0 {
			t.Fatalf("empty selection accessed catalog: %+v", runtime)
		}
	}
	app := &App{connectorService: &desktopConnectorRuntimeHost{}}
	if _, err := app.expandConnectorSelections("", nil, nil, nil, "input"); err != nil {
		t.Fatalf("uninitialized connector blocked empty selection: %v", err)
	}
}

type turnCatalogRuntime struct {
	retryableCatalogRuntime
	readErr    error
	refreshErr error
}

type desktopCatalogSourceFunc func(context.Context) ([]byte, error)

func (f desktopCatalogSourceFunc) Fetch(ctx context.Context) ([]byte, error) { return f(ctx) }

func TestConnectorHostReportsStartupFailureAndRecovers(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"data":  map[string]any{"catalog_version": "sha256:" + strings.Repeat("1", 64), "generated_at": "2026-09-07T00:00:00Z", "items": []any{}},
		"error": nil, "request_id": "fixture", "server_time": "2026-09-07T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	fail := true
	root := t.TempDir()
	service, err := managedconnector.NewService(managedconnector.ServiceOptions{
		Mode: managedconnector.CatalogModeCloud,
		Source: desktopCatalogSourceFunc(func(context.Context) ([]byte, error) {
			if fail {
				return nil, errors.New("fixture unavailable")
			}
			return raw, nil
		}),
		Installer: managedconnector.Installer{Root: root}, StateStore: managedconnector.StateStore{SharedRoot: root}, TempRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	host := &desktopConnectorRuntimeHost{service: service}
	if err := host.Start(context.Background()); err == nil {
		t.Fatal("host swallowed startup error")
	}
	if host.service != service {
		t.Fatal("startup discarded recoverable service")
	}
	fail = false
	if _, err := connectorCatalogForTurn(context.Background(), host); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Catalog(); err != nil {
		t.Fatal(err)
	}
}

func (r *turnCatalogRuntime) Catalog() (managedconnector.Catalog, error) {
	r.catalogs++
	return managedconnector.Catalog{Version: "fixture"}, r.readErr
}

func (r *turnCatalogRuntime) RefreshCatalog(ctx context.Context) (managedconnector.Catalog, error) {
	r.refreshes++
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
		return managedconnector.Catalog{}, errors.New("missing deadline")
	}
	return managedconnector.Catalog{Version: "recovered"}, r.refreshErr
}

func TestConnectorCatalogForTurnRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                string
		readErr, refreshErr error
		calls               int
		want                string
	}{
		{"ready", nil, nil, 0, "fixture"},
		{"recovered", errors.New("offline"), nil, 1, "recovered"},
		{"failed", errors.New("offline"), errors.New("sensitive-fixture-detail"), 1, "CONNECTOR_CATALOG_UNAVAILABLE"},
		{"busy", errors.New("offline"), managedconnector.ErrCatalogRefreshInProgress, 1, "CONNECTOR_CATALOG_REFRESHING"},
		{"timeout", errors.New("offline"), context.DeadlineExceeded, 1, "CONNECTOR_CATALOG_TIMEOUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &turnCatalogRuntime{readErr: tc.readErr, refreshErr: tc.refreshErr}
			catalog, err := connectorCatalogForTurn(context.Background(), runtime)
			if runtime.refreshes != tc.calls {
				t.Fatalf("refreshes = %d", runtime.refreshes)
			}
			if tc.refreshErr == nil {
				if err != nil || catalog.Version != tc.want {
					t.Fatalf("catalog = %+v, %v", catalog, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "sensitive-fixture-detail") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := &turnCatalogRuntime{}
	if _, err := connectorCatalogForTurn(ctx, runtime); !errors.Is(err, context.Canceled) || runtime.catalogs != 0 {
		t.Fatalf("canceled read = %v, calls=%d", err, runtime.catalogs)
	}
}

func TestNonemptyConnectorSelectionStillRequiresCatalog(t *testing.T) {
	for _, selection := range []struct{ mcp, skills []string }{
		{mcp: []string{"selected"}}, {skills: []string{"selected"}}, {mcp: []string{"!invalid"}},
	} {
		runtime := &turnCatalogRuntime{readErr: errors.New("offline"), refreshErr: errors.New("offline")}
		app := &App{connectorService: runtime}
		if _, err := app.expandConnectorSelections("missing", nil, selection.mcp, selection.skills, "input"); err == nil {
			t.Fatal("selected capability silently bypassed catalog")
		}
		if runtime.refreshes != 1 {
			t.Fatalf("refreshes = %d", runtime.refreshes)
		}
	}
}

type retryableCatalogRuntime struct {
	refreshes int
	catalogs  int
}

func (*retryableCatalogRuntime) Start(context.Context) error {
	return errors.New("temporary catalog failure")
}

func (*retryableCatalogRuntime) Close() error { return nil }

func (r *retryableCatalogRuntime) Catalog() (managedconnector.Catalog, error) {
	r.catalogs++
	return managedconnector.Catalog{}, errors.New("catalog is not ready")
}

func (r *retryableCatalogRuntime) RefreshCatalog(context.Context) (managedconnector.Catalog, error) {
	r.refreshes++
	return managedconnector.Catalog{Version: "fresh-catalog"}, nil
}

func (*retryableCatalogRuntime) Install(context.Context, string, managedconnector.OwnerScope, managedconnector.ScopeConfirmation) (managedconnector.Snapshot, error) {
	return managedconnector.Snapshot{}, nil
}

func (*retryableCatalogRuntime) Snapshot(string, managedconnector.OwnerScope) (managedconnector.Snapshot, error) {
	return managedconnector.Snapshot{}, nil
}

func (*retryableCatalogRuntime) SetOwnerStatus(managedconnector.OwnerScope, string, managedconnector.Status, *managedconnector.SafeError) (managedconnector.Snapshot, error) {
	return managedconnector.Snapshot{}, nil
}

func (*retryableCatalogRuntime) Manifest(string) (managedconnector.ActiveVersion, string, managedconnector.Manifest, error) {
	return managedconnector.ActiveVersion{}, "", managedconnector.Manifest{}, nil
}

func (*retryableCatalogRuntime) CatalogItem(string) (managedconnector.CatalogItem, error) {
	return managedconnector.CatalogItem{}, nil
}

func TestConnectorCatalogRetriesTransientStartupFailure(t *testing.T) {
	runtime := &retryableCatalogRuntime{}
	app := &App{connectorService: runtime}
	app.startConnectorService(context.Background())

	view, err := app.ConnectorCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	if view.CatalogVersion != "fresh-catalog" || runtime.refreshes != 1 {
		t.Fatalf("catalog = %+v, refreshes = %d", view, runtime.refreshes)
	}
}

func TestMissingConnectorTargetAllowsCatalogButBlocksSideEffects(t *testing.T) {
	runtime := &retryableCatalogRuntime{}
	app := &App{connectorService: runtime, tabRuntimes: map[string]*WorkspaceTab{}}
	app.startConnectorService(context.Background())

	if _, err := app.ConnectorCatalog(""); err != nil {
		t.Fatal(err)
	}
	refreshesAfterCatalog := runtime.refreshes
	catalogsAfterCatalog := runtime.catalogs
	if _, err := app.ConnectorStates("destroyed"); err == nil {
		t.Fatal("states accepted a destroyed target tab")
	}
	if _, err := app.InstallConnector("destroyed", "fixture"); err == nil {
		t.Fatal("install accepted a destroyed target tab")
	}
	if runtime.refreshes != refreshesAfterCatalog || runtime.catalogs != catalogsAfterCatalog {
		t.Fatalf("destroyed target reached catalog side effects: refreshes=%d catalogs=%d", runtime.refreshes, runtime.catalogs)
	}
}

func TestConnectorOwnerScopeDoesNotRequireBuiltTabController(t *testing.T) {
	app, actor, wantScope, _ := newConnectorOAuthRefreshTestApp(t)
	app.connectorService = &retryableCatalogRuntime{}
	app.tabRuntimes = map[string]*WorkspaceTab{
		"restoring": {ID: "restoring", Scope: "global", actor: actor},
	}

	gotScope, gotActor, err := app.connectorOwnerScope(context.Background(), "restoring")
	if err != nil {
		t.Fatal(err)
	}
	if gotScope.ProfileID != wantScope.ProfileID || gotScope.CloudUserID != wantScope.CloudUserID || gotScope.BindingGeneration != wantScope.BindingGeneration {
		t.Fatalf("scope = %+v, want owner identity %+v", gotScope, wantScope)
	}
	if gotActor.ProfileID != actor.Normalize().ProfileID {
		t.Fatalf("actor = %+v, want profile %q", gotActor, actor.Normalize().ProfileID)
	}
	if _, err := app.InstallConnector("restoring", "fixture"); err != nil {
		t.Fatalf("install rejected a restoring tab with known owner: %v", err)
	}
}

func (r *recordingManagedRegistrar) ConnectManagedMCPServer(_ context.Context, spec plugin.Spec) (int, error) {
	r.spec = spec
	return 3, nil
}

func TestUnknownNonProductConnectorRegistersExpandedRuntimeName(t *testing.T) {
	registrar := &recordingManagedRegistrar{}
	runtimeName := managedconnector.RuntimeMCPName("unknown-lab-connector", "documents")
	spec := plugin.Spec{Name: runtimeName, Type: "streamable-http", URL: "https://mcp.example.test", Headers: map[string]string{"X-Client": "desktop"}}
	count, err := registerManagedMCPSpec(context.Background(), registrar, spec)
	if err != nil || count != 3 {
		t.Fatalf("register = %d, %v", count, err)
	}
	if registrar.spec.Name != "unknown-lab-connector--documents" || registrar.spec.URL != spec.URL || registrar.spec.Headers["X-Client"] != "desktop" {
		t.Fatalf("registered spec = %+v", registrar.spec)
	}
}

func TestManagedMCPProbeInitializesAndListsToolsWithoutController(t *testing.T) {
	server := desktopMCPHTTPServer(t)
	defer server.Close()
	host := plugin.NewHost()
	defer host.Close()

	count, err := probeManagedMCPSpec(context.Background(), host, plugin.Spec{
		Name: "restoring--remote", Type: "streamable-http", URL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count <= 0 {
		t.Fatalf("probe tools = %d, want at least one", count)
	}
}

func TestConnectorSkillsRequireLiveSamePackageMCP(t *testing.T) {
	server := desktopMCPHTTPServer(t)
	defer server.Close()
	host := plugin.NewHost()
	defer host.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !connectorMCPsConnectedForSkills(ctx, host, []plugin.Spec{{Name: "fixture--remote", Type: "streamable-http", URL: server.URL}}) {
		t.Fatal("connected same-package MCP did not enable its Skills")
	}
	if !host.HasClient("fixture--remote") {
		t.Fatal("MCP readiness check did not preserve the live remote client")
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	missingHost := plugin.NewHost()
	defer missingHost.Close()
	if connectorMCPsConnectedForSkills(cancelled, missingHost, []plugin.Spec{{Name: "missing--remote", Type: "streamable-http", URL: server.URL}}) {
		t.Fatal("unavailable same-package MCP enabled its Skills")
	}
}

func TestTokenHeaderTemplateOnlyEmitsDeclaredHeaders(t *testing.T) {
	schema := managedconnector.TokenSchema{Fields: []managedconnector.TokenField{{ID: "api_token"}}, Headers: []managedconnector.TokenHeader{{Name: "Authorization", Template: "Token {api_token}"}}}
	headers := renderConnectorTokenHeaders(schema, map[string]string{"api_token": "secret", "undeclared": "must-not-emit"})
	if len(headers) != 1 || headers["Authorization"] != "Token secret" {
		t.Fatalf("rendered headers = %+v", headers)
	}
}

func TestConnectorTokenValuesStayInOwnerKeyringAndRenderAtRequestTime(t *testing.T) {
	app, actor, scope, credentials := newConnectorOAuthRefreshTestApp(t)
	schema := managedconnector.TokenSchema{
		Fields: []managedconnector.TokenField{
			{ID: "account", Label: "Account", Type: "text", Required: true},
			{ID: "api_key", Label: "API key", Type: "secret", Required: true},
		},
		Headers: []managedconnector.TokenHeader{
			{Name: "X-Account", Template: "{account}"},
			{Name: "Authorization", Template: "Bearer {api_key}"},
		},
	}
	manifest := managedconnector.Manifest{
		Connector: managedconnector.ConnectorManifest{ID: "fixture-token"},
		MCP: &managedconnector.MCPManifest{Servers: []managedconnector.MCPServer{{
			ID: "remote", Transport: "streamable-http", URL: "https://mcp.example.com/mcp",
			Auth: managedconnector.MCPAuth{Type: "token", Schema: "token-schema.json"},
		}}},
		Token: &schema,
	}
	if err := app.storeConnectorTokenValues(context.Background(), actor, scope, manifest, map[string]string{
		"account": "current-owner", "api_key": "owner-secret", "undeclared": "must-not-persist",
	}); err != nil {
		t.Fatal(err)
	}
	owner := connectorCredentialOwner(scope)
	if got := credentials.raw(owner, connectorTokenKey("fixture-token", "remote", "account")); got != "current-owner" {
		t.Fatalf("stored account = %q", got)
	}
	if got := credentials.raw(owner, connectorTokenKey("fixture-token", "remote", "api_key")); got != "owner-secret" {
		t.Fatalf("stored API key = %q", got)
	}
	if got := credentials.raw(owner, connectorTokenKey("fixture-token", "remote", "undeclared")); got != "" {
		t.Fatalf("undeclared value persisted = %q", got)
	}
	snapshot, err := app.connectorTokenRuntimeHeaders(actor, scope, "fixture-token", "remote", schema)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Headers) != 2 || snapshot.Headers["X-Account"] != "current-owner" || snapshot.Headers["Authorization"] != "Bearer owner-secret" {
		t.Fatalf("runtime headers = %+v", snapshot.Headers)
	}
	if err := app.deleteConnectorServerCredentials(actor, scope, "fixture-token", "remote", &schema); err != nil {
		t.Fatal(err)
	}
	for _, field := range schema.Fields {
		if _, found, _ := credentials.Get(owner, connectorTokenKey("fixture-token", "remote", field.ID)); found {
			t.Fatalf("token field %q remains after unlink", field.ID)
		}
	}
}

func TestConnectorAuthFieldsExposeOnlyDeclaredFieldsAndDocumentation(t *testing.T) {
	manifest := managedconnector.Manifest{
		Connector: managedconnector.ConnectorManifest{ID: "fixture-token"},
		MCP: &managedconnector.MCPManifest{Servers: []managedconnector.MCPServer{{
			ID: "remote", Auth: managedconnector.MCPAuth{Type: "token"},
		}}},
		Token: &managedconnector.TokenSchema{
			DocumentationLabel: "Get an API key", DocumentationURL: "https://docs.example.com/tokens",
			Fields: []managedconnector.TokenField{{ID: "api_key", Label: "API key", Type: "secret", Required: true}},
		},
	}
	app := &App{connectorService: &oauthRefreshRuntime{manifest: manifest}}
	fields, err := app.ConnectorAuthFields("tab", "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].ID != "api_key" || fields[0].Type != "secret" ||
		fields[0].DocumentationLabel != "Get an API key" || fields[0].DocumentationURL != "https://docs.example.com/tokens" {
		t.Fatalf("auth fields = %+v", fields)
	}
}

func TestConnectorAuthorizationActiveIsProcessLocal(t *testing.T) {
	app := &App{connectorAuthFlows: map[string]context.CancelFunc{}}
	scope := managedconnector.OwnerScope{
		ProfileID:         "profile",
		CloudUserID:       "cloud-user",
		BindingGeneration: 1,
		StateRoot:         t.TempDir(),
	}
	if app.connectorAuthorizationActive(scope, "unknown-lab-connector") {
		t.Fatal("authorization unexpectedly active before this process starts a flow")
	}
	app.connectorAuthFlows[connectorFlowKey(scope, "unknown-lab-connector")] = func() {}
	if !app.connectorAuthorizationActive(scope, "unknown-lab-connector") {
		t.Fatal("authorization started in this process was not recognized")
	}
}

func TestConnectorAuthorizationFlowsAreIndependentAndCancelable(t *testing.T) {
	app := &App{connectorAuthFlows: map[string]context.CancelFunc{}}
	scope := managedconnector.OwnerScope{
		ProfileID:         "profile",
		CloudUserID:       "cloud-user",
		BindingGeneration: 1,
		StateRoot:         t.TempDir(),
	}
	firstCtx, firstCancel, err := app.beginConnectorAuthorization(scope, "first-connector")
	if err != nil {
		t.Fatal(err)
	}
	secondCtx, secondCancel, err := app.beginConnectorAuthorization(scope, "second-connector")
	if err != nil {
		t.Fatal(err)
	}
	defer app.finishConnectorAuthorization(scope, "first-connector", firstCancel)
	defer app.finishConnectorAuthorization(scope, "second-connector", secondCancel)

	app.cancelConnectorAuthorization(scope, "first-connector")
	select {
	case <-firstCtx.Done():
	default:
		t.Fatal("cancel did not stop the selected connector flow")
	}
	select {
	case <-secondCtx.Done():
		t.Fatal("cancel stopped an unrelated connector flow")
	default:
	}
	if app.connectorAuthorizationActive(scope, "first-connector") {
		t.Fatal("cancelled connector remained active")
	}
	if !app.connectorAuthorizationActive(scope, "second-connector") {
		t.Fatal("unrelated connector was removed from the active flow map")
	}
}

type brokerOAuthPollClient struct {
	completions []cloud.ConnectorOAuthCompletion
	calls       int
}

func (*brokerOAuthPollClient) StartGitHub(context.Context, cloud.BindingScope) (cloud.ConnectorOAuthStart, error) {
	return cloud.ConnectorOAuthStart{}, errors.New("not used")
}

func (c *brokerOAuthPollClient) CompleteGitHub(context.Context, cloud.BindingScope, string) (cloud.ConnectorOAuthCompletion, error) {
	result := c.completions[c.calls]
	c.calls++
	return result, nil
}

func TestGitHubConnectorOAuthPollingWaitsUntilCompleted(t *testing.T) {
	client := &brokerOAuthPollClient{completions: []cloud.ConnectorOAuthCompletion{
		{Status: "pending"},
		{Status: "completed", AccessToken: "github-access", TokenType: "bearer"},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := waitForGitHubConnectorOAuth(ctx, client, cloud.BindingScope{}, "coaf_test", time.Millisecond)
	if err != nil || result.AccessToken != "github-access" || client.calls != 2 {
		t.Fatalf("poll result = %+v, calls = %d, err = %v", result, client.calls, err)
	}
}
