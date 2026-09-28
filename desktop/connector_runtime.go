package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	managedconnector "kwu/desktop/internal/connector"
	"kwu/desktop/internal/diagnostics"
	"kwu/desktop/internal/mcpoauth"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/plugin"
	"kwu/internal/userctx"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const connectorOAuthBrokerPollInterval = 1500 * time.Millisecond

var defaultConnectorOAuthCredentialRetryDelays = []time.Duration{50 * time.Millisecond, 100 * time.Millisecond}

type connectorOAuthFailureStage string
type connectorOAuthFailureClass string

const (
	connectorOAuthStageBinding    connectorOAuthFailureStage = "binding"
	connectorOAuthStageKeyring    connectorOAuthFailureStage = "keyring"
	connectorOAuthStageCredential connectorOAuthFailureStage = "credential"
	connectorOAuthStageRefresh    connectorOAuthFailureStage = "refresh"

	connectorOAuthClassBindingRead  connectorOAuthFailureClass = "binding_read"
	connectorOAuthClassOwnerChanged connectorOAuthFailureClass = "owner_changed"
	connectorOAuthClassKeyringRead  connectorOAuthFailureClass = "keyring_read"
	connectorOAuthClassNotFound     connectorOAuthFailureClass = "keyring_not_found"
	connectorOAuthClassInvalid      connectorOAuthFailureClass = "credential_invalid"
	connectorOAuthClassRefreshRead  connectorOAuthFailureClass = "refresh_failed"
	connectorOAuthClassInvalidGrant connectorOAuthFailureClass = "refresh_invalid_grant"
)

type connectorOAuthFailure struct {
	Stage     connectorOAuthFailureStage
	Class     connectorOAuthFailureClass
	Retryable bool
	Reauth    bool
	cause     error
}

type connectorOAuthRuntimeFailureKey struct {
	ProfileID         string
	CloudUserID       string
	BindingGeneration uint64
	ConnectorID       string
	ServerID          string
}

func (e *connectorOAuthFailure) Error() string {
	if e != nil && e.Reauth {
		return "Connector authorization is required. Reconnect it from the Connectors page."
	}
	return "Connector is temporarily unavailable. Please retry the current operation later."
}

func (e *connectorOAuthFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func connectorOAuthFailureDetails(err error) (*connectorOAuthFailure, bool) {
	var failure *connectorOAuthFailure
	return failure, errors.As(err, &failure)
}

type connectorOAuthCredential struct {
	Version       int       `json:"version"`
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	TokenType     string    `json:"token_type"`
	Scope         string    `json:"scope,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	ClientID      string    `json:"client_id"`
	Issuer        string    `json:"issuer"`
	Resource      string    `json:"resource"`
	TokenEndpoint string    `json:"token_endpoint,omitempty"`
}

type connectorCredentialStore interface {
	Set(config.CredentialOwner, string, string) error
	Get(config.CredentialOwner, string) (string, bool, error)
	Delete(config.CredentialOwner, string) error
}

type systemConnectorCredentialStore struct{}

func (systemConnectorCredentialStore) Set(owner config.CredentialOwner, key, value string) error {
	return config.SetKeyringOnlyCredential(owner, key, value)
}

func (systemConnectorCredentialStore) Get(owner config.CredentialOwner, key string) (string, bool, error) {
	return config.KeyringOnlyCredential(owner, key)
}

func (systemConnectorCredentialStore) Delete(owner config.CredentialOwner, key string) error {
	return config.DeleteKeyringOnlyCredential(owner, key)
}

type connectorOAuthClientFactory func(string) (*mcpoauth.Client, error)

func (a *App) managedConnectorCredentials() connectorCredentialStore {
	if a.connectorCredentials != nil {
		return a.connectorCredentials
	}
	return systemConnectorCredentialStore{}
}

func (a *App) newConnectorOAuthClient(endpoint string) (*mcpoauth.Client, error) {
	if a.connectorOAuthClient != nil {
		return a.connectorOAuthClient(endpoint)
	}
	httpClient, err := newPublicCloudHTTPClient(endpoint)
	if err != nil {
		return nil, err
	}
	return mcpoauth.NewClient(httpClient), nil
}

type ConnectorAuthFieldView struct {
	Key                string `json:"key"`
	ServerID           string `json:"server_id"`
	ID                 string `json:"id"`
	Label              string `json:"label"`
	Type               string `json:"type"`
	Required           bool   `json:"required"`
	DocumentationURL   string `json:"documentation_url"`
	DocumentationLabel string `json:"documentation_label"`
}

type managedMCPRegistrar interface {
	ConnectManagedMCPServer(context.Context, plugin.Spec) (int, error)
}

func registerManagedMCPSpec(ctx context.Context, registrar managedMCPRegistrar, spec plugin.Spec) (int, error) {
	if registrar == nil {
		return 0, errors.New("managed MCP registration is unavailable")
	}
	return registrar.ConnectManagedMCPServer(ctx, spec)
}

func probeManagedMCPSpec(ctx context.Context, host *plugin.Host, spec plugin.Spec) (int, error) {
	if host == nil {
		return 0, errors.New("managed MCP probe is unavailable")
	}
	tools, err := host.Add(ctx, spec)
	return len(tools), err
}

func (a *App) ConnectorAuthFields(_ string, connectorID string) ([]ConnectorAuthFieldView, error) {
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return nil, err
	}
	_, _, manifest, err := runtimeService.Manifest(strings.TrimSpace(connectorID))
	if err != nil {
		return nil, err
	}
	if manifest.Token == nil || manifest.MCP == nil {
		return []ConnectorAuthFieldView{}, nil
	}
	views := []ConnectorAuthFieldView{}
	for _, server := range manifest.MCP.Servers {
		if server.Auth.Type != "token" {
			continue
		}
		for _, field := range manifest.Token.Fields {
			views = append(views, ConnectorAuthFieldView{
				Key: server.ID + "." + field.ID, ServerID: server.ID, ID: field.ID,
				Label: field.Label, Type: field.Type, Required: field.Required,
				DocumentationURL: manifest.Token.DocumentationURL, DocumentationLabel: manifest.Token.DocumentationLabel,
			})
		}
	}
	return views, nil
}

// ConnectConnector installs/upgrades from the current Catalog, completes every
// declared schema 2 auth/lifecycle entrypoint, and marks the aggregate connected
// only after the remote MCP and any optional package Skills are ready.
func (a *App) ConnectConnector(tabID, connectorID string, tokenValues map[string]string) (ConnectorStateView, error) {
	connectorID = strings.TrimSpace(connectorID)
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab == nil {
		return ConnectorStateView{}, fmt.Errorf("connector target tab %q is unavailable", tabID)
	}
	// Browser authorization and credential registration are one global desktop
	// operation, matching WorkBuddy's connector lifecycle.
	a.connectorConnectMu.Lock()
	defer a.connectorConnectMu.Unlock()
	if ctrl != nil && controllerHasActiveRuntimeWork(ctrl) {
		return ConnectorStateView{}, rebuildControllerActiveWorkError("connector")
	}
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return ConnectorStateView{}, err
	}
	if _, refreshErr := runtimeService.RefreshCatalog(a.bootContext()); refreshErr != nil {
		if _, catalogErr := runtimeService.Catalog(); catalogErr != nil {
			return ConnectorStateView{}, refreshErr
		}
	}
	ctx := a.bootContext()
	scope, actor, err := a.connectorOwnerScope(ctx, tabID)
	if err != nil {
		return ConnectorStateView{}, err
	}
	snapshot, err := runtimeService.Install(ctx, connectorID, scope, func(ctx context.Context, _ managedconnector.OwnerScope) error {
		return a.confirmConnectorOwnerScope(ctx, actor, scope)
	})
	if err != nil {
		return ConnectorStateView{}, err
	}
	if snapshot.Status == managedconnector.StatusFailed {
		return connectorStateView(snapshot), errors.New("connector package installation failed")
	}
	_, packageRoot, manifest, err := runtimeService.Manifest(connectorID)
	if err != nil {
		return ConnectorStateView{}, err
	}
	plan, err := managedconnector.BuildExecutionPlan(manifest, packageRoot, desktopConnectorRuntimePolicy().OS)
	if err != nil {
		return ConnectorStateView{}, err
	}
	flowCtx, cancel, err := a.beginConnectorAuthorization(scope, connectorID)
	if err != nil {
		return ConnectorStateView{}, err
	}
	defer a.finishConnectorAuthorization(scope, connectorID, cancel)
	if _, err := runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusAuthorizing, nil); err != nil {
		return ConnectorStateView{}, err
	}
	fail := func(status managedconnector.Status, code, message string) (ConnectorStateView, error) {
		failed, stateErr := runtimeService.SetOwnerStatus(scope, connectorID, status, &managedconnector.SafeError{Code: code, Message: message, Retryable: true})
		if stateErr != nil {
			return ConnectorStateView{}, stateErr
		}
		return connectorStateView(failed), errors.New(message)
	}

	if err := a.storeConnectorTokenValues(flowCtx, actor, scope, manifest, tokenValues); err != nil {
		return fail(managedconnector.StatusAuthorizationNeeded, "CONNECTOR_TOKEN_REQUIRED", err.Error())
	}
	if err := a.authorizeConnectorOAuthServers(flowCtx, actor, scope, manifest); err != nil {
		if errors.Is(flowCtx.Err(), context.Canceled) {
			cancelled, stateErr := runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusAuthorizationNeeded, nil)
			if stateErr != nil {
				return ConnectorStateView{}, stateErr
			}
			return connectorStateView(cancelled), context.Canceled
		}
		if _, ok := connectorOAuthFailureDetails(err); ok {
			a.recordConnectorOAuthRuntimeFailure(flowCtx, actor, scope, connectorID, "", err)
			snapshot, snapshotErr := runtimeService.Snapshot(connectorID, scope)
			if snapshotErr != nil {
				return ConnectorStateView{}, snapshotErr
			}
			return connectorStateView(snapshot), publicConnectorOAuthError(err)
		}
		return fail(managedconnector.StatusAuthorizationNeeded, "CONNECTOR_OAUTH_FAILED", "未完成授权，请重试。")
	}

	// OAuth account setup is independent of the current model controller. If the
	// controller is unavailable, use the same generic MCP Host for a temporary
	// initialize/tools/list probe; a later controller rebuild restores the
	// connected package through applyConnectorCapabilities.
	if _, currentCtrl := a.tabAndCtrlByID(tabID); currentCtrl != nil {
		ctrl = currentCtrl
	}
	var probeHost *plugin.Host
	if ctrl == nil {
		probeHost = plugin.NewHost()
		defer probeHost.Close()
	}
	disconnectConnected := func(names []string) {
		if ctrl != nil {
			a.disconnectManagedMCPs(ctrl, names)
		}
	}
	connectedNames := []string{}
	tools := 0
	for _, execution := range plan.MCP {
		spec, err := a.connectorMCPSpec(flowCtx, actor, scope, manifest, execution)
		if err != nil {
			disconnectConnected(connectedNames)
			if _, ok := connectorOAuthFailureDetails(err); ok {
				a.recordConnectorOAuthRuntimeFailure(flowCtx, actor, scope, connectorID, execution.ServerID, err)
				snapshot, snapshotErr := runtimeService.Snapshot(connectorID, scope)
				if snapshotErr != nil {
					return ConnectorStateView{}, snapshotErr
				}
				return connectorStateView(snapshot), publicConnectorOAuthError(err)
			}
			return fail(managedconnector.StatusAuthorizationNeeded, "CONNECTOR_MCP_AUTH_UNAVAILABLE", "connector MCP credentials are unavailable")
		}
		var count int
		if ctrl == nil {
			count, err = probeManagedMCPSpec(flowCtx, probeHost, spec)
		} else {
			ctrl.DisconnectMCPServer(execution.RuntimeName)
			if host := ctrl.Host(); host != nil {
				host.ClearFailure(execution.RuntimeName)
			}
			managedCtrl, ok := ctrl.(managedMCPRegistrar)
			if !ok {
				disconnectConnected(connectedNames)
				return fail(managedconnector.StatusRepairNeeded, "CONNECTOR_MCP_UNSUPPORTED", "current task cannot register managed MCP servers")
			}
			count, err = registerManagedMCPSpec(flowCtx, managedCtrl, spec)
		}
		if err != nil || count <= 0 {
			disconnectConnected(append(connectedNames, execution.RuntimeName))
			return fail(managedconnector.StatusRepairNeeded, "CONNECTOR_MCP_CONNECT_FAILED", "connector MCP server did not expose tools")
		}
		connectedNames = append(connectedNames, execution.RuntimeName)
		tools += count
	}
	if err := a.confirmConnectorOwnerScope(flowCtx, actor, scope); err != nil {
		disconnectConnected(connectedNames)
		return ConnectorStateView{}, err
	}
	connected, err := runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusConnected, nil)
	if err != nil {
		disconnectConnected(connectedNames)
		return ConnectorStateView{}, err
	}
	a.clearConnectorOAuthRuntimeFailures(scope, connectorID, "")
	if plan.Skills != nil {
		if err := a.RefreshSkills(); err != nil {
			_, _ = runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusRepairNeeded, &managedconnector.SafeError{Code: "CONNECTOR_SKILLS_RELOAD_FAILED", Message: "connector Skills could not be registered", Retryable: true})
			return ConnectorStateView{}, err
		}
	}
	view := connectorStateView(connected)
	view.Tools = tools
	return view, nil
}

func (a *App) DisconnectConnector(tabID, connectorID string) (ConnectorStateView, error) {
	connectorID = strings.TrimSpace(connectorID)
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab == nil {
		return ConnectorStateView{}, fmt.Errorf("connector target tab %q is unavailable", tabID)
	}
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return ConnectorStateView{}, err
	}
	ctx := a.bootContext()
	scope, actor, err := a.connectorOwnerScope(ctx, tabID)
	if err != nil {
		return ConnectorStateView{}, err
	}
	a.cancelConnectorAuthorization(scope, connectorID)
	_, packageRoot, manifest, err := runtimeService.Manifest(connectorID)
	if err != nil {
		return ConnectorStateView{}, err
	}
	plan, err := managedconnector.BuildExecutionPlan(manifest, packageRoot, desktopConnectorRuntimePolicy().OS)
	if err != nil {
		return ConnectorStateView{}, err
	}
	for _, execution := range plan.MCP {
		if ctrl != nil {
			ctrl.DisconnectMCPServer(execution.RuntimeName)
			if host := ctrl.Host(); host != nil {
				host.ClearFailure(execution.RuntimeName)
			}
		}
		var tokenSchema *managedconnector.TokenSchema
		for _, server := range manifest.MCP.Servers {
			if server.ID == execution.ServerID && server.Auth.Type == "token" {
				tokenSchema = manifest.Token
				break
			}
		}
		if err := a.deleteConnectorServerCredentials(actor, scope, connectorID, execution.ServerID, tokenSchema); err != nil {
			return ConnectorStateView{}, err
		}
	}
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return ConnectorStateView{}, err
	}
	disconnected, err := runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusAuthorizationNeeded, nil)
	if err != nil {
		return ConnectorStateView{}, err
	}
	a.clearConnectorOAuthRuntimeFailures(scope, connectorID, "")
	if plan.Skills != nil {
		if err := a.RefreshSkills(); err != nil {
			return ConnectorStateView{}, err
		}
	}
	return connectorStateView(disconnected), nil
}

// CancelConnectorAuthorization stops only the active browser authorization
// wait for this connector. It deliberately preserves any existing Keyring
// credential; unlink remains the responsibility of DisconnectConnector.
func (a *App) CancelConnectorAuthorization(tabID, connectorID string) error {
	connectorID = strings.TrimSpace(connectorID)
	tab, _ := a.tabAndCtrlByID(tabID)
	if tab == nil {
		return fmt.Errorf("connector target tab %q is unavailable", tabID)
	}
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return err
	}
	scope, _, err := a.connectorOwnerScope(a.bootContext(), tabID)
	if err != nil {
		return err
	}
	a.cancelConnectorAuthorization(scope, connectorID)
	_, err = runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusAuthorizationNeeded, nil)
	return err
}

func (a *App) authorizeConnectorOAuthServers(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, manifest managedconnector.Manifest) error {
	if manifest.MCP == nil {
		return nil
	}
	for _, server := range manifest.MCP.Servers {
		if server.Auth.Type != "oauth" {
			continue
		}
		_, found, loadErr := a.loadConnectorOAuthCredential(ctx, actor, scope, manifest.Connector.ID, server.ID)
		if loadErr == nil && found {
			continue
		}
		if failure, ok := connectorOAuthFailureDetails(loadErr); ok && !failure.Reauth {
			return loadErr
		}
		if server.Auth.Provider != "" {
			if err := a.authorizeConnectorBrokerOAuthServer(ctx, actor, scope, manifest.Connector.ID, server); err != nil {
				return err
			}
			continue
		}
		resource := server.Auth.Resource
		config := mcpoauth.Config{ResourceURL: server.URL, Resource: resource, Scopes: append([]string(nil), (*server.Auth.Scopes)...), ClientID: server.Auth.ClientID}
		client, err := a.newConnectorOAuthClient(server.URL)
		if err != nil {
			return err
		}
		metadata, err := client.Discover(ctx, config)
		if err != nil {
			return err
		}
		state, verifier, challenge, err := mcpoauth.NewPKCE()
		if err != nil {
			return err
		}
		callback, err := mcpoauth.StartCallbackListener(state)
		if err != nil {
			return err
		}
		registration, err := client.Register(ctx, config, metadata, callback.RedirectURI)
		if err != nil {
			callback.Close()
			return err
		}
		authorizationURL, err := mcpoauth.AuthorizationURL(config, metadata, registration, callback.RedirectURI, state, challenge)
		if err != nil {
			callback.Close()
			return err
		}
		wruntime.BrowserOpenURL(a.ctx, authorizationURL)
		code, err := callback.Wait(ctx)
		callback.Close()
		if err != nil {
			return err
		}
		token, err := client.Exchange(ctx, config, metadata, registration, callback.RedirectURI, code, verifier)
		if err != nil {
			return err
		}
		credential := connectorOAuthCredential{Version: 1, AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, TokenType: token.TokenType, Scope: token.Scope, ExpiresAt: token.ExpiresAt, ClientID: registration.ClientID, Issuer: metadata.Issuer, Resource: resource, TokenEndpoint: metadata.TokenEndpoint}
		if err := a.storeConnectorOAuthCredential(ctx, actor, scope, manifest.Connector.ID, server.ID, credential); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) authorizeConnectorBrokerOAuthServer(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID string, server managedconnector.MCPServer) error {
	if server.Auth.Provider != "github" || a.cloudRuntime == nil || a.cloudRuntime.connectorOAuth == nil {
		return errors.New("connector OAuth broker is unavailable")
	}
	cloudScope := cloud.BindingScope{
		ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration,
	}
	started, err := a.cloudRuntime.connectorOAuth.StartGitHub(ctx, cloudScope)
	if err != nil {
		return err
	}
	wruntime.BrowserOpenURL(a.ctx, started.AuthorizationURL)
	completed, err := waitForGitHubConnectorOAuth(ctx, a.cloudRuntime.connectorOAuth, cloudScope, started.FlowID, connectorOAuthBrokerPollInterval)
	if err != nil {
		return err
	}
	credential := connectorOAuthCredential{
		Version: 1, AccessToken: completed.AccessToken, TokenType: completed.TokenType,
		Scope: completed.Scope, Resource: server.URL, Issuer: "github-cloud-broker",
	}
	return a.storeConnectorOAuthCredential(ctx, actor, scope, connectorID, server.ID, credential)
}

func waitForGitHubConnectorOAuth(ctx context.Context, client cloud.ConnectorOAuthClient, scope cloud.BindingScope, flowID string, interval time.Duration) (cloud.ConnectorOAuthCompletion, error) {
	if client == nil || interval <= 0 {
		return cloud.ConnectorOAuthCompletion{}, errors.New("connector OAuth broker polling is unavailable")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return cloud.ConnectorOAuthCompletion{}, errors.New("connector OAuth authorization timed out or was cancelled")
		case <-ticker.C:
			result, err := client.CompleteGitHub(ctx, scope, flowID)
			if err != nil {
				return cloud.ConnectorOAuthCompletion{}, err
			}
			if result.Status == "completed" {
				return result, nil
			}
		}
	}
}

func (a *App) storeConnectorTokenValues(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, manifest managedconnector.Manifest, values map[string]string) error {
	if manifest.Token == nil || manifest.MCP == nil {
		return nil
	}
	tokenServers := 0
	for _, server := range manifest.MCP.Servers {
		if server.Auth.Type == "token" {
			tokenServers++
		}
	}
	for _, server := range manifest.MCP.Servers {
		if server.Auth.Type != "token" {
			continue
		}
		for _, field := range manifest.Token.Fields {
			value := strings.TrimSpace(values[server.ID+"."+field.ID])
			if value == "" && tokenServers == 1 {
				value = strings.TrimSpace(values[field.ID])
			}
			key := connectorTokenKey(manifest.Connector.ID, server.ID, field.ID)
			if value != "" {
				if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
					return err
				}
				if err := a.managedConnectorCredentials().Set(connectorCredentialOwner(scope), key, value); err != nil {
					return errors.New("could not store connector token in Keyring")
				}
			}
			_, found, err := a.managedConnectorCredentials().Get(connectorCredentialOwner(scope), key)
			if err != nil || field.Required && !found {
				return fmt.Errorf("connector token field %q is required", field.Label)
			}
		}
	}
	return nil
}

func (a *App) connectorMCPSpec(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, manifest managedconnector.Manifest, execution managedconnector.MCPExecution) (plugin.Spec, error) {
	spec := plugin.Spec{Name: execution.RuntimeName, Headers: cloneStringMap(execution.Headers)}
	if execution.Transport != "streamable-http" || !managedconnector.IsPublicMCPHTTPSURL(execution.URL) {
		return plugin.Spec{}, errors.New("connector MCP transport is not public HTTPS streamable HTTP")
	}
	spec.Type = "streamable-http"
	spec.URL = execution.URL
	switch execution.AuthType {
	case "oauth":
		server, foundServer := connectorMCPServer(manifest, execution.ServerID)
		if !foundServer {
			return plugin.Spec{}, errors.New("connector OAuth server is unavailable")
		}
		if _, found, err := a.loadConnectorOAuthCredential(ctx, actor, scope, manifest.Connector.ID, execution.ServerID); err != nil || !found {
			if err == nil {
				err = newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassNotFound, false, true, nil)
			}
			return plugin.Spec{}, err
		}
		spec.RuntimeHeaders = a.connectorOAuthRuntimeHeaders(actor, scope, manifest.Connector.ID, server)
		spec.RuntimeHeaderRefresh = a.connectorOAuthRuntimeRefresher(actor, scope, manifest.Connector.ID, server)
	case "token":
		if manifest.Token == nil {
			return plugin.Spec{}, errors.New("connector token schema is unavailable")
		}
		spec.RuntimeHeaders = a.connectorTokenRuntimeHeaders(actor, scope, manifest.Connector.ID, execution.ServerID, *manifest.Token)
	}
	return spec, nil
}

func connectorMCPServer(manifest managedconnector.Manifest, serverID string) (managedconnector.MCPServer, bool) {
	if manifest.MCP != nil {
		for _, server := range manifest.MCP.Servers {
			if server.ID == serverID {
				return server, true
			}
		}
	}
	return managedconnector.MCPServer{}, false
}

func (a *App) connectorOAuthRuntimeHeaders(actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID string, server managedconnector.MCPServer) plugin.RuntimeHeaderProvider {
	return func(ctx context.Context) (plugin.RuntimeHeaderSnapshot, error) {
		credential, found, err := a.loadConnectorOAuthCredential(ctx, actor, scope, connectorID, server.ID)
		if err != nil || !found {
			if err == nil {
				err = newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassNotFound, false, true, nil)
			}
			a.recordConnectorOAuthRuntimeFailure(ctx, actor, scope, connectorID, server.ID, err)
			return plugin.RuntimeHeaderSnapshot{}, publicConnectorOAuthError(err)
		}
		observation := connectorOAuthCredentialObservation(credential)
		if !credential.ExpiresAt.IsZero() && !credential.ExpiresAt.After(time.Now().UTC()) {
			credential, err = a.refreshConnectorOAuthCredential(ctx, actor, scope, connectorID, server, observation)
			if err != nil {
				a.recordConnectorOAuthRuntimeFailure(ctx, actor, scope, connectorID, server.ID, err)
				return plugin.RuntimeHeaderSnapshot{}, publicConnectorOAuthError(err)
			}
			observation = connectorOAuthCredentialObservation(credential)
		}
		a.recordConnectorOAuthRuntimeSuccess(ctx, actor, scope, connectorID, server.ID)
		return connectorOAuthHeaderSnapshot(credential, observation), nil
	}
}

func (a *App) connectorOAuthRuntimeRefresher(actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID string, server managedconnector.MCPServer) plugin.RuntimeHeaderRefresher {
	return func(ctx context.Context, observation string) (plugin.RuntimeHeaderSnapshot, error) {
		credential, err := a.refreshConnectorOAuthCredential(ctx, actor, scope, connectorID, server, observation)
		if err != nil {
			a.recordConnectorOAuthRuntimeFailure(ctx, actor, scope, connectorID, server.ID, err)
			return plugin.RuntimeHeaderSnapshot{}, publicConnectorOAuthError(err)
		}
		a.recordConnectorOAuthRuntimeSuccess(ctx, actor, scope, connectorID, server.ID)
		return connectorOAuthHeaderSnapshot(credential, connectorOAuthCredentialObservation(credential)), nil
	}
}

func connectorOAuthHeaderSnapshot(credential connectorOAuthCredential, observation string) plugin.RuntimeHeaderSnapshot {
	return plugin.RuntimeHeaderSnapshot{Headers: map[string]string{"Authorization": "Bearer " + credential.AccessToken}, Observation: observation}
}

func (a *App) connectorTokenRuntimeHeaders(actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string, schema managedconnector.TokenSchema) plugin.RuntimeHeaderProvider {
	return func(ctx context.Context) (plugin.RuntimeHeaderSnapshot, error) {
		if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
			return plugin.RuntimeHeaderSnapshot{}, err
		}
		values := map[string]string{}
		for _, field := range schema.Fields {
			value, found, err := a.managedConnectorCredentials().Get(connectorCredentialOwner(scope), connectorTokenKey(connectorID, serverID, field.ID))
			if err != nil || field.Required && !found {
				return plugin.RuntimeHeaderSnapshot{}, errors.New("connector token credential is unavailable")
			}
			values[field.ID] = value
		}
		return plugin.RuntimeHeaderSnapshot{Headers: renderConnectorTokenHeaders(schema, values)}, nil
	}
}

func renderConnectorTokenHeaders(schema managedconnector.TokenSchema, values map[string]string) map[string]string {
	headers := make(map[string]string, len(schema.Headers))
	for _, header := range schema.Headers {
		value := header.Template
		for field, replacement := range values {
			value = strings.ReplaceAll(value, "{"+field+"}", replacement)
		}
		headers[header.Name] = value
	}
	return headers
}

func (a *App) storeConnectorOAuthCredential(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string, credential connectorOAuthCredential) error {
	lock := a.connectorOAuthLock(scope, connectorID, serverID)
	lock.Lock()
	defer lock.Unlock()
	return a.storeConnectorOAuthCredentialLocked(ctx, actor, scope, connectorID, serverID, credential)
}

func (a *App) storeConnectorOAuthCredentialLocked(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string, credential connectorOAuthCredential) error {
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return err
	}
	raw, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	return a.managedConnectorCredentials().Set(connectorCredentialOwner(scope), connectorOAuthKey(connectorID, serverID), string(raw))
}

func (a *App) loadConnectorOAuthCredential(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string) (connectorOAuthCredential, bool, error) {
	delays := a.connectorOAuthRetryDelays
	if delays == nil {
		delays = defaultConnectorOAuthCredentialRetryDelays
	}
	maxAttempts := len(delays) + 1
	for attempt := 1; ; attempt++ {
		credential, found, err := a.loadConnectorOAuthCredentialOnce(ctx, actor, scope, connectorID, serverID)
		if err == nil {
			return credential, found, nil
		}
		failure, classified := connectorOAuthFailureDetails(err)
		if !classified || !failure.Retryable || attempt >= maxAttempts || ctx.Err() != nil {
			logConnectorOAuthFailure(connectorID, serverID, failure, attempt, maxAttempts, false)
			return connectorOAuthCredential{}, false, err
		}
		logConnectorOAuthFailure(connectorID, serverID, failure, attempt, maxAttempts, true)
		if err := waitConnectorOAuthRetry(ctx, delays[attempt-1]); err != nil {
			return connectorOAuthCredential{}, false, err
		}
	}
}

func (a *App) loadConnectorOAuthCredentialOnce(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string) (connectorOAuthCredential, bool, error) {
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		class := connectorOAuthClassBindingRead
		retryable := true
		var cloudErr *cloud.Error
		if errors.As(err, &cloudErr) && cloudErr.Code == cloud.ErrorProfileScopeMismatch {
			class = connectorOAuthClassOwnerChanged
			retryable = false
		}
		return connectorOAuthCredential{}, false, newConnectorOAuthFailure(connectorOAuthStageBinding, class, retryable, false, err)
	}
	raw, found, err := a.managedConnectorCredentials().Get(connectorCredentialOwner(scope), connectorOAuthKey(connectorID, serverID))
	if err != nil {
		return connectorOAuthCredential{}, false, newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassKeyringRead, true, false, err)
	}
	if !found {
		return connectorOAuthCredential{}, false, newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassNotFound, false, true, nil)
	}
	var credential connectorOAuthCredential
	if json.Unmarshal([]byte(raw), &credential) != nil || credential.Version != 1 || strings.TrimSpace(credential.AccessToken) == "" || !strings.EqualFold(credential.TokenType, "Bearer") {
		return connectorOAuthCredential{}, false, newConnectorOAuthFailure(connectorOAuthStageCredential, connectorOAuthClassInvalid, false, true, nil)
	}
	return credential, true, nil
}

func newConnectorOAuthFailure(stage connectorOAuthFailureStage, class connectorOAuthFailureClass, retryable, reauth bool, cause error) error {
	return &connectorOAuthFailure{Stage: stage, Class: class, Retryable: retryable, Reauth: reauth, cause: cause}
}

func publicConnectorOAuthError(err error) error {
	if failure, ok := connectorOAuthFailureDetails(err); ok {
		return &connectorOAuthFailure{Stage: failure.Stage, Class: failure.Class, Retryable: failure.Retryable, Reauth: failure.Reauth}
	}
	return &connectorOAuthFailure{Stage: connectorOAuthStageRefresh, Class: connectorOAuthClassRefreshRead, Retryable: false}
}

func waitConnectorOAuthRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func logConnectorOAuthFailure(connectorID, serverID string, failure *connectorOAuthFailure, attempt, maxAttempts int, retrying bool) {
	stage := connectorOAuthFailureStage("unknown")
	class := connectorOAuthFailureClass("unknown")
	if failure != nil {
		stage = failure.Stage
		class = failure.Class
	}
	slog.Warn("connector OAuth credential operation failed",
		"connector_id", connectorID,
		"server_id", serverID,
		"stage", stage,
		"class", class,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"retrying", retrying,
	)
}

func (a *App) recordConnectorOAuthRuntimeFailure(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string, err error) {
	recordDesktopBackgroundFailure("connector.oauth.runtime", err)
	failure, ok := connectorOAuthFailureDetails(err)
	if !ok {
		failure = &connectorOAuthFailure{Stage: connectorOAuthStageRefresh, Class: connectorOAuthClassRefreshRead}
	}
	logConnectorOAuthFailure(connectorID, serverID, failure, 1, 1, false)
	if ctx.Err() != nil || strings.TrimSpace(connectorID) == "" {
		return
	}
	var stateErr error
	if failure.Reauth {
		a.clearConnectorOAuthRuntimeFailures(scope, connectorID, "")
		stateErr = a.disableConnectorOAuthCapabilities(ctx, actor, scope, connectorID, managedconnector.StatusAuthorizationNeeded, &managedconnector.SafeError{
			Code: "CONNECTOR_OAUTH_REAUTH_REQUIRED", Message: "连接器需要重新授权，请重新连接。", Retryable: true,
		})
	} else {
		safeErr := &managedconnector.SafeError{Code: "CONNECTOR_OAUTH_TEMPORARILY_UNAVAILABLE", Message: "连接器暂时不可用，请重试。", Retryable: true}
		a.connectorOAuthRuntimeFailures.Store(connectorOAuthRuntimeFailureMapKey(scope, connectorID, serverID), *safeErr)
		if confirmErr := a.confirmConnectorOwnerScope(ctx, actor, scope); confirmErr != nil {
			return
		}
		runtimeService, runtimeErr := a.connectorRuntime()
		if runtimeErr != nil {
			stateErr = runtimeErr
		} else {
			_, stateErr = runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusRepairNeeded, safeErr)
		}
	}
	if stateErr != nil {
		slog.Warn("connector OAuth runtime state update failed", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "connector.oauth.runtime.state", "connector_id", connectorID, "server_id", serverID, "stage", failure.Stage, "class", failure.Class, "err", stateErr)
	}
}

func connectorOAuthRuntimeFailureMapKey(scope managedconnector.OwnerScope, connectorID, serverID string) connectorOAuthRuntimeFailureKey {
	return connectorOAuthRuntimeFailureKey{
		ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration,
		ConnectorID: connectorID, ServerID: serverID,
	}
}

func (a *App) connectorOAuthRuntimeFailure(scope managedconnector.OwnerScope, connectorID string) (*managedconnector.SafeError, bool) {
	var result *managedconnector.SafeError
	a.connectorOAuthRuntimeFailures.Range(func(key, value any) bool {
		typedKey, ok := key.(connectorOAuthRuntimeFailureKey)
		if !ok || typedKey.ProfileID != scope.ProfileID || typedKey.CloudUserID != scope.CloudUserID || typedKey.BindingGeneration != scope.BindingGeneration || typedKey.ConnectorID != connectorID {
			return true
		}
		typedValue, ok := value.(managedconnector.SafeError)
		if ok {
			copy := typedValue
			result = &copy
		}
		return false
	})
	return result, result != nil
}

func (a *App) clearConnectorOAuthRuntimeFailures(scope managedconnector.OwnerScope, connectorID, serverID string) {
	a.connectorOAuthRuntimeFailures.Range(func(key, _ any) bool {
		typedKey, ok := key.(connectorOAuthRuntimeFailureKey)
		if ok && typedKey.ProfileID == scope.ProfileID && typedKey.CloudUserID == scope.CloudUserID && typedKey.BindingGeneration == scope.BindingGeneration && typedKey.ConnectorID == connectorID && (serverID == "" || typedKey.ServerID == serverID) {
			a.connectorOAuthRuntimeFailures.Delete(key)
		}
		return true
	})
}

func (a *App) recordConnectorOAuthRuntimeSuccess(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string) {
	key := connectorOAuthRuntimeFailureMapKey(scope, connectorID, serverID)
	if _, found := a.connectorOAuthRuntimeFailures.LoadAndDelete(key); !found {
		return
	}
	if _, stillUnavailable := a.connectorOAuthRuntimeFailure(scope, connectorID); stillUnavailable {
		return
	}
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return
	}
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return
	}
	snapshot, err := runtimeService.Snapshot(connectorID, scope)
	if err == nil && snapshot.Status == managedconnector.StatusRepairNeeded && snapshot.Error != nil && snapshot.Error.Code == "CONNECTOR_OAUTH_TEMPORARILY_UNAVAILABLE" {
		_, _ = runtimeService.SetOwnerStatus(scope, connectorID, managedconnector.StatusConnected, nil)
	}
}

func connectorOAuthCredentialObservation(credential connectorOAuthCredential) string {
	raw, _ := json.Marshal(credential)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func connectorOAuthLockKey(scope managedconnector.OwnerScope, connectorID, serverID string) string {
	owner := connectorCredentialOwner(scope)
	digest := sha256.Sum256([]byte(owner.Kind + "\x00" + owner.ID + "\x00" + fmt.Sprint(scope.BindingGeneration) + "\x00" + connectorID + "\x00" + serverID))
	return hex.EncodeToString(digest[:])
}

func (a *App) connectorOAuthLock(scope managedconnector.OwnerScope, connectorID, serverID string) *sync.Mutex {
	lock, _ := a.connectorOAuthLocks.LoadOrStore(connectorOAuthLockKey(scope, connectorID, serverID), &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (a *App) refreshConnectorOAuthCredential(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID string, server managedconnector.MCPServer, observed string) (connectorOAuthCredential, error) {
	if strings.TrimSpace(observed) == "" {
		return connectorOAuthCredential{}, errors.New("connector OAuth credential observation is unavailable")
	}
	lock := a.connectorOAuthLock(scope, connectorID, server.ID)
	lock.Lock()
	defer lock.Unlock()

	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return connectorOAuthCredential{}, err
	}
	credential, found, err := a.loadConnectorOAuthCredential(ctx, actor, scope, connectorID, server.ID)
	if err != nil || !found {
		if err == nil {
			err = newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassNotFound, false, true, nil)
		}
		return connectorOAuthCredential{}, err
	}
	currentObservation := connectorOAuthCredentialObservation(credential)
	if currentObservation != observed {
		return credential, nil
	}
	if strings.TrimSpace(credential.RefreshToken) == "" || strings.TrimSpace(credential.ClientID) == "" || strings.TrimSpace(credential.Issuer) == "" {
		return connectorOAuthCredential{}, newConnectorOAuthFailure(connectorOAuthStageCredential, connectorOAuthClassInvalid, false, true, nil)
	}

	config := mcpoauth.Config{
		ResourceURL: server.URL,
		Resource:    credential.Resource,
		Scopes:      strings.Fields(credential.Scope),
		ClientID:    credential.ClientID,
	}
	metadata := mcpoauth.Metadata{Issuer: credential.Issuer, TokenEndpoint: strings.TrimSpace(credential.TokenEndpoint)}
	if metadata.TokenEndpoint == "" {
		discoveryClient, clientErr := a.newConnectorOAuthClient(server.URL)
		if clientErr != nil {
			return connectorOAuthCredential{}, newConnectorOAuthFailure(connectorOAuthStageRefresh, connectorOAuthClassRefreshRead, false, false, clientErr)
		}
		discovered, discoverErr := discoveryClient.Discover(ctx, config)
		if discoverErr != nil {
			return connectorOAuthCredential{}, newConnectorOAuthFailure(connectorOAuthStageRefresh, connectorOAuthClassRefreshRead, false, false, discoverErr)
		}
		if discovered.Issuer != credential.Issuer {
			return connectorOAuthCredential{}, errors.New("connector OAuth issuer changed during refresh discovery")
		}
		metadata = discovered
	}
	client, err := a.newConnectorOAuthClient(metadata.TokenEndpoint)
	if err != nil {
		return connectorOAuthCredential{}, newConnectorOAuthFailure(connectorOAuthStageRefresh, connectorOAuthClassRefreshRead, false, false, err)
	}
	refreshed, err := client.Refresh(ctx, config, metadata, mcpoauth.Registration{ClientID: credential.ClientID}, credential.RefreshToken)
	if err != nil {
		if errors.Is(err, mcpoauth.ErrInvalidGrant) {
			latest, stillCurrent, verifyErr := a.connectorOAuthCredentialStillCurrent(ctx, actor, scope, connectorID, server.ID, currentObservation)
			if verifyErr != nil {
				return connectorOAuthCredential{}, verifyErr
			}
			if !stillCurrent {
				return latest, nil
			}
			invalidateErr := a.invalidateConnectorOAuthCredentialLocked(ctx, actor, scope, connectorID, server.ID, currentObservation)
			if invalidateErr != nil {
				return connectorOAuthCredential{}, errors.Join(mcpoauth.ErrInvalidGrant, invalidateErr)
			}
			return connectorOAuthCredential{}, newConnectorOAuthFailure(connectorOAuthStageRefresh, connectorOAuthClassInvalidGrant, false, true, mcpoauth.ErrInvalidGrant)
		}
		return connectorOAuthCredential{}, newConnectorOAuthFailure(connectorOAuthStageRefresh, connectorOAuthClassRefreshRead, false, false, err)
	}

	latest, stillCurrent, err := a.connectorOAuthCredentialStillCurrent(ctx, actor, scope, connectorID, server.ID, currentObservation)
	if err != nil {
		return connectorOAuthCredential{}, err
	}
	if !stillCurrent {
		return latest, nil
	}
	credential.AccessToken = refreshed.AccessToken
	credential.ExpiresAt = refreshed.ExpiresAt
	credential.TokenEndpoint = metadata.TokenEndpoint
	if refreshed.RefreshToken != "" {
		credential.RefreshToken = refreshed.RefreshToken
	}
	if refreshed.Scope != "" {
		credential.Scope = refreshed.Scope
	}
	if refreshed.TokenType != "" {
		credential.TokenType = refreshed.TokenType
	}
	if err := a.storeConnectorOAuthCredentialLocked(ctx, actor, scope, connectorID, server.ID, credential); err != nil {
		return connectorOAuthCredential{}, err
	}
	return credential, nil
}

func (a *App) connectorOAuthCredentialStillCurrent(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID, observed string) (connectorOAuthCredential, bool, error) {
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return connectorOAuthCredential{}, false, err
	}
	credential, found, err := a.loadConnectorOAuthCredential(ctx, actor, scope, connectorID, serverID)
	if err != nil || !found {
		if err == nil {
			err = newConnectorOAuthFailure(connectorOAuthStageKeyring, connectorOAuthClassNotFound, false, true, nil)
		}
		return connectorOAuthCredential{}, false, err
	}
	return credential, connectorOAuthCredentialObservation(credential) == observed, nil
}

func (a *App) invalidateConnectorOAuthCredentialLocked(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID, observed string) error {
	if _, current, err := a.connectorOAuthCredentialStillCurrent(ctx, actor, scope, connectorID, serverID, observed); err != nil || !current {
		if err != nil {
			return err
		}
		return nil
	}
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return err
	}
	if _, current, err := a.connectorOAuthCredentialStillCurrent(ctx, actor, scope, connectorID, serverID, observed); err != nil || !current {
		if err != nil {
			return err
		}
		return nil
	}
	deleteErr := a.managedConnectorCredentials().Delete(connectorCredentialOwner(scope), connectorOAuthKey(connectorID, serverID))
	disableErr := a.disableConnectorOAuthCapabilities(ctx, actor, scope, connectorID, managedconnector.StatusAuthorizationNeeded, &managedconnector.SafeError{
		Code: "CONNECTOR_OAUTH_REAUTH_REQUIRED", Message: "连接器需要重新授权，请重新连接。", Retryable: true,
	})
	return errors.Join(deleteErr, disableErr)
}

func (a *App) disableConnectorOAuthCapabilities(ctx context.Context, actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID string, status managedconnector.Status, safeErr *managedconnector.SafeError) error {
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return err
	}
	_, packageRoot, manifest, err := runtimeService.Manifest(connectorID)
	if err != nil {
		return err
	}
	plan, err := managedconnector.BuildExecutionPlan(manifest, packageRoot, desktopConnectorRuntimePolicy().OS)
	if err != nil {
		return err
	}
	runtimeNames := make([]string, 0, len(plan.MCP))
	for _, execution := range plan.MCP {
		runtimeNames = append(runtimeNames, execution.RuntimeName)
	}
	if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
		return err
	}
	_, statusErr := runtimeService.SetOwnerStatus(scope, connectorID, status, safeErr)

	a.mu.RLock()
	controllers := make([]interface {
		ActorContext() userctx.ActorContext
		Host() *plugin.Host
		UnregisterMCPServerTools(string) bool
	}, 0, len(a.tabRuntimes)+len(a.detachedSessions))
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil || tab.Ctrl == nil {
			continue
		}
		if controller, ok := tab.Ctrl.(interface {
			ActorContext() userctx.ActorContext
			Host() *plugin.Host
			UnregisterMCPServerTools(string) bool
		}); ok {
			controllers = append(controllers, controller)
		}
	}
	a.mu.RUnlock()

	hosts := map[*plugin.Host]struct{}{}
	for _, controller := range controllers {
		controllerScope, resolveErr := a.connectorOwnerScopeForActor(ctx, controller.ActorContext())
		if resolveErr != nil || !sameConnectorOwnerScope(controllerScope, scope) {
			continue
		}
		for _, runtimeName := range runtimeNames {
			controller.UnregisterMCPServerTools(runtimeName)
		}
		if host := controller.Host(); host != nil {
			hosts[host] = struct{}{}
		}
	}
	for host := range hosts {
		for _, runtimeName := range runtimeNames {
			host.Remove(runtimeName)
		}
	}
	var skillsErr error
	if plan.Skills != nil {
		skillsErr = a.RefreshSkills()
	}
	return errors.Join(statusErr, skillsErr)
}

func sameConnectorOwnerScope(left, right managedconnector.OwnerScope) bool {
	return left.ProfileID == right.ProfileID && left.CloudUserID == right.CloudUserID && left.BindingGeneration == right.BindingGeneration
}

func (a *App) deleteConnectorServerCredentials(actor userctx.ActorContext, scope managedconnector.OwnerScope, connectorID, serverID string, token *managedconnector.TokenSchema) error {
	if err := a.confirmConnectorOwnerScope(a.bootContext(), actor, scope); err != nil {
		return err
	}
	owner := connectorCredentialOwner(scope)
	lock := a.connectorOAuthLock(scope, connectorID, serverID)
	lock.Lock()
	defer lock.Unlock()
	if err := a.managedConnectorCredentials().Delete(owner, connectorOAuthKey(connectorID, serverID)); err != nil {
		return err
	}
	if token != nil {
		for _, field := range token.Fields {
			if err := a.managedConnectorCredentials().Delete(owner, connectorTokenKey(connectorID, serverID, field.ID)); err != nil {
				return err
			}
		}
	}
	return nil
}

func connectorCredentialOwner(scope managedconnector.OwnerScope) config.CredentialOwner {
	digest := sha256.Sum256([]byte(scope.ProfileID + "\x00" + scope.CloudUserID))
	return config.CredentialOwner{Kind: config.CredentialOwnerProfile, ID: "connector-" + hex.EncodeToString(digest[:])}
}

func connectorOAuthKey(connectorID, serverID string) string {
	return "connector:" + connectorID + ":mcp:" + serverID + ":oauth:v1"
}

func connectorTokenKey(connectorID, serverID, fieldID string) string {
	return "connector:" + connectorID + ":mcp:" + serverID + ":token:" + fieldID + ":v1"
}

func connectorFlowKey(scope managedconnector.OwnerScope, connectorID string) string {
	digest := sha256.Sum256([]byte(scope.ProfileID + "\x00" + scope.CloudUserID + "\x00" + connectorID))
	return hex.EncodeToString(digest[:])
}

func (a *App) beginConnectorAuthorization(scope managedconnector.OwnerScope, connectorID string) (context.Context, context.CancelFunc, error) {
	key := connectorFlowKey(scope, connectorID)
	a.connectorAuthMu.Lock()
	defer a.connectorAuthMu.Unlock()
	if a.connectorAuthFlows == nil {
		a.connectorAuthFlows = map[string]context.CancelFunc{}
	}
	if _, exists := a.connectorAuthFlows[key]; exists {
		return nil, nil, errors.New("connector authorization is already in progress")
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 10*time.Minute)
	a.connectorAuthFlows[key] = cancel
	return ctx, cancel, nil
}

func (a *App) connectorAuthorizationActive(scope managedconnector.OwnerScope, connectorID string) bool {
	key := connectorFlowKey(scope, connectorID)
	a.connectorAuthMu.Lock()
	defer a.connectorAuthMu.Unlock()
	_, active := a.connectorAuthFlows[key]
	return active
}

func (a *App) finishConnectorAuthorization(scope managedconnector.OwnerScope, connectorID string, cancel context.CancelFunc) {
	key := connectorFlowKey(scope, connectorID)
	a.connectorAuthMu.Lock()
	delete(a.connectorAuthFlows, key)
	a.connectorAuthMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) cancelConnectorAuthorization(scope managedconnector.OwnerScope, connectorID string) {
	key := connectorFlowKey(scope, connectorID)
	a.connectorAuthMu.Lock()
	cancel := a.connectorAuthFlows[key]
	delete(a.connectorAuthFlows, key)
	a.connectorAuthMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) cancelAllConnectorAuthorization() {
	a.connectorAuthMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.connectorAuthFlows))
	for key, cancel := range a.connectorAuthFlows {
		cancels = append(cancels, cancel)
		delete(a.connectorAuthFlows, key)
	}
	a.connectorAuthMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (a *App) disconnectManagedMCPs(ctrl interface{ DisconnectMCPServer(string) bool }, names []string) {
	for _, name := range names {
		ctrl.DisconnectMCPServer(name)
	}
}
