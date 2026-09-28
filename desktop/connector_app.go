package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	managedconnector "kwu/desktop/internal/connector"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/userctx"
)

type desktopConnectorRuntime interface {
	Start(context.Context) error
	Close() error
	Catalog() (managedconnector.Catalog, error)
	RefreshCatalog(context.Context) (managedconnector.Catalog, error)
	Install(context.Context, string, managedconnector.OwnerScope, managedconnector.ScopeConfirmation) (managedconnector.Snapshot, error)
	Snapshot(string, managedconnector.OwnerScope) (managedconnector.Snapshot, error)
	SetOwnerStatus(managedconnector.OwnerScope, string, managedconnector.Status, *managedconnector.SafeError) (managedconnector.Snapshot, error)
	Manifest(string) (managedconnector.ActiveVersion, string, managedconnector.Manifest, error)
	CatalogItem(string) (managedconnector.CatalogItem, error)
}

type ConnectorCatalogView struct {
	CatalogVersion string                     `json:"catalog_version"`
	Items          []ConnectorCatalogItemView `json:"items"`
}

type ConnectorCatalogItemView struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	IconURL       string   `json:"icon_url,omitempty"`
	Examples      []string `json:"examples"`
	Kind          string   `json:"kind"`
	Version       string   `json:"version"`
	Compatible    bool     `json:"compatible"`
	BlockedReason string   `json:"blocked_reason,omitempty"`
	Capabilities  []string `json:"capabilities"`
}

type ConnectorStateView struct {
	ConnectorID string                      `json:"connector_id"`
	Status      managedconnector.Status     `json:"status"`
	Version     string                      `json:"version,omitempty"`
	SHA256      string                      `json:"sha256,omitempty"`
	Error       *managedconnector.SafeError `json:"error,omitempty"`
	Tools       int                         `json:"tools,omitempty"`
}

func newDesktopConnectorRuntime() (desktopConnectorRuntime, error) {
	return &desktopConnectorRuntimeHost{}, nil
}

type desktopConnectorRuntimeHost struct {
	mu      sync.Mutex
	service *managedconnector.Service
}

func (h *desktopConnectorRuntimeHost) Start(ctx context.Context) error {
	h.mu.Lock()
	err := h.initialize()
	service := h.service
	h.mu.Unlock()
	if err != nil {
		return err
	}
	_, err = service.RecoverCatalog(ctx)
	return err
}

// initialize runs under the existing host mutex, without network operations.
func (h *desktopConnectorRuntimeHost) initialize() error {
	if h.service != nil {
		return nil
	}
	policy, err := managedconnector.NormalizeRuntimePolicy(desktopConnectorRuntimePolicy())
	if err != nil {
		return err
	}
	baseURL, err := resolveCloudBaseURL(version, os.Getenv(cloudBaseURLEnv), cloudDevelopmentBaseURL, cloudProductionBaseURL)
	if err != nil {
		return err
	}
	catalogURL, err := managedconnector.CatalogURL(baseURL, policy)
	if err != nil {
		return err
	}
	client, err := newPublicCloudHTTPClient(baseURL)
	if err != nil {
		return err
	}
	packageRoot, tempRoot := managedconnector.DefaultRoots(config.CacheDir())
	service, err := managedconnector.NewService(managedconnector.ServiceOptions{
		Mode:   managedconnector.CatalogModeCloud,
		Source: managedconnector.HTTPSource{URL: catalogURL, Client: client}, Downloader: managedconnector.Downloader{Client: client},
		Installer:  managedconnector.Installer{Root: packageRoot, Policy: policy},
		StateStore: managedconnector.StateStore{SharedRoot: packageRoot}, TempRoot: tempRoot,
	})
	if err != nil {
		return err
	}
	h.service = service
	// No persistent Catalog cache is introduced in P2. A failed first fetch is
	// retried by ConnectorCatalog; already-installed active packages remain on
	// disk but are never upgraded without a fresh Catalog.
	return nil
}

func (h *desktopConnectorRuntimeHost) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.service == nil {
		return nil
	}
	err := h.service.Close()
	h.service = nil
	return err
}

func (h *desktopConnectorRuntimeHost) Catalog() (managedconnector.Catalog, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.service == nil {
		return managedconnector.Catalog{}, errors.New("connector service is not started")
	}
	return h.service.Catalog()
}

func (h *desktopConnectorRuntimeHost) RefreshCatalog(ctx context.Context) (managedconnector.Catalog, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.Catalog{}, errors.New("connector service is not started")
	}
	return service.RefreshCatalog(ctx)
}

func (h *desktopConnectorRuntimeHost) RecoverCatalog(ctx context.Context) (managedconnector.Catalog, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.Catalog{}, errors.New("connector service is not started")
	}
	return service.RecoverCatalog(ctx)
}

func (h *desktopConnectorRuntimeHost) Install(ctx context.Context, id string, scope managedconnector.OwnerScope, confirm managedconnector.ScopeConfirmation) (managedconnector.Snapshot, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.Snapshot{}, errors.New("connector service is not started")
	}
	return service.Install(ctx, id, scope, confirm)
}

func (h *desktopConnectorRuntimeHost) Snapshot(id string, scope managedconnector.OwnerScope) (managedconnector.Snapshot, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.Snapshot{}, errors.New("connector service is not started")
	}
	return service.Snapshot(id, scope)
}

func (h *desktopConnectorRuntimeHost) SetOwnerStatus(scope managedconnector.OwnerScope, id string, status managedconnector.Status, safeErr *managedconnector.SafeError) (managedconnector.Snapshot, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.Snapshot{}, errors.New("connector service is not started")
	}
	return service.SetOwnerStatus(scope, id, status, safeErr)
}

func (h *desktopConnectorRuntimeHost) Manifest(id string) (managedconnector.ActiveVersion, string, managedconnector.Manifest, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.ActiveVersion{}, "", managedconnector.Manifest{}, errors.New("connector service is not started")
	}
	return service.Manifest(id)
}

func (h *desktopConnectorRuntimeHost) CatalogItem(id string) (managedconnector.CatalogItem, error) {
	h.mu.Lock()
	service := h.service
	h.mu.Unlock()
	if service == nil {
		return managedconnector.CatalogItem{}, errors.New("connector service is not started")
	}
	return service.CatalogItem(id)
}

func desktopConnectorRuntimePolicy() managedconnector.RuntimePolicy {
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	architecture := runtime.GOARCH
	if architecture == "amd64" {
		architecture = "x64"
	}
	return managedconnector.RuntimePolicy{
		OS: osName, Architecture: architecture, ClientVersion: version,
	}
}

func (a *App) managedConnectorConfigName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || a.connectorService == nil {
		return false
	}
	catalog, err := a.connectorService.Catalog()
	if err != nil {
		return false
	}
	for _, item := range catalog.Items {
		if name == item.ID || strings.HasPrefix(name, item.ID+"--") {
			return true
		}
	}
	return false
}

func (a *App) ConnectorCatalog(_ string) (ConnectorCatalogView, error) {
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return ConnectorCatalogView{}, err
	}
	catalog, err := runtimeService.Catalog()
	if err != nil {
		catalog, err = runtimeService.RefreshCatalog(a.bootContext())
		if err != nil {
			return ConnectorCatalogView{}, err
		}
	}
	view := ConnectorCatalogView{CatalogVersion: catalog.Version, Items: make([]ConnectorCatalogItemView, 0, len(catalog.Items))}
	for _, item := range catalog.Items {
		view.Items = append(view.Items, ConnectorCatalogItemView{
			ID: item.ID, Name: item.Name, Provider: item.Provider, Description: item.Description, Category: item.Category,
			IconURL: item.IconURL, Examples: append([]string(nil), item.Examples...), Kind: item.Kind,
			Version: item.Version, Compatible: item.Compatible, BlockedReason: item.BlockedReason,
			Capabilities: append([]string(nil), item.Capabilities...),
		})
	}
	return view, nil
}

func (a *App) ConnectorStates(tabID string) ([]ConnectorStateView, error) {
	ctx := a.bootContext()
	scope, actor, err := a.connectorOwnerScope(ctx, tabID)
	if err != nil {
		return nil, err
	}
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return nil, err
	}
	catalog, err := runtimeService.Catalog()
	if err != nil {
		return nil, err
	}
	views := make([]ConnectorStateView, 0, len(catalog.Items))
	_, ctrl := a.tabAndCtrlByID(tabID)
	toolCounts := connectedMCPToolCounts(ctrl)
	for _, item := range catalog.Items {
		snapshot, err := runtimeService.Snapshot(item.ID, scope)
		if err != nil {
			return nil, err
		}
		if snapshot.Status == managedconnector.StatusAuthorizing && !a.connectorAuthorizationActive(scope, item.ID) {
			if err := a.confirmConnectorOwnerScope(ctx, actor, scope); err != nil {
				return nil, err
			}
			snapshot, err = runtimeService.SetOwnerStatus(scope, item.ID, managedconnector.StatusAuthorizationNeeded, nil)
			if err != nil {
				return nil, err
			}
		}
		if runtimeErr, unavailable := a.connectorOAuthRuntimeFailure(scope, item.ID); unavailable && snapshot.Status == managedconnector.StatusConnected {
			snapshot.Status = managedconnector.StatusRepairNeeded
			snapshot.Error = runtimeErr
		}
		view := connectorStateView(snapshot)
		if snapshot.Status == managedconnector.StatusConnected {
			if _, packageRoot, manifest, manifestErr := runtimeService.Manifest(item.ID); manifestErr == nil {
				if plan, planErr := managedconnector.BuildExecutionPlan(manifest, packageRoot, desktopConnectorRuntimePolicy().OS); planErr == nil {
					for _, execution := range plan.MCP {
						view.Tools += toolCounts[execution.RuntimeName]
					}
				}
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (a *App) InstallConnector(tabID, connectorID string) (ConnectorStateView, error) {
	ctx := a.bootContext()
	scope, actor, err := a.connectorOwnerScope(ctx, tabID)
	if err != nil {
		return ConnectorStateView{}, err
	}
	return a.installConnectorForOwner(ctx, scope, actor, connectorID)
}

func (a *App) installConnectorForOwner(ctx context.Context, scope managedconnector.OwnerScope, actor userctx.ActorContext, connectorID string) (ConnectorStateView, error) {
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return ConnectorStateView{}, err
	}
	if _, refreshErr := runtimeService.RefreshCatalog(a.bootContext()); refreshErr != nil {
		if _, catalogErr := runtimeService.Catalog(); catalogErr != nil {
			return ConnectorStateView{}, refreshErr
		}
	}
	expected := scope
	confirm := func(ctx context.Context, _ managedconnector.OwnerScope) error {
		return a.confirmConnectorOwnerScope(ctx, actor, expected)
	}
	snapshot, err := runtimeService.Install(ctx, strings.TrimSpace(connectorID), scope, confirm)
	if err != nil {
		return ConnectorStateView{}, err
	}
	if err := confirm(ctx, scope); err != nil {
		return ConnectorStateView{}, err
	}
	return connectorStateView(snapshot), nil
}

func (a *App) connectorRuntime() (desktopConnectorRuntime, error) {
	if a.connectorInitErr != nil {
		return nil, fmt.Errorf("connector service is unavailable: %w", a.connectorInitErr)
	}
	if a.connectorService == nil {
		return nil, errors.New("connector service is unavailable")
	}
	return a.connectorService, nil
}

func (a *App) connectorOwnerScope(ctx context.Context, tabID string) (managedconnector.OwnerScope, userctx.ActorContext, error) {
	if _, err := a.connectorRuntime(); err != nil {
		return managedconnector.OwnerScope{}, userctx.ActorContext{}, err
	}
	a.mu.RLock()
	tab := a.tabRuntimes[strings.TrimSpace(tabID)]
	actor := tabRuntimeActorContext(tab)
	a.mu.RUnlock()
	if tab == nil {
		return managedconnector.OwnerScope{}, userctx.ActorContext{}, fmt.Errorf("connector target tab %q is unavailable", tabID)
	}
	owner, err := a.connectorOwnerScopeForActor(ctx, actor)
	if err != nil {
		return managedconnector.OwnerScope{}, userctx.ActorContext{}, err
	}
	return owner, actor, nil
}

func (a *App) connectorOwnerScopeForActor(ctx context.Context, actor userctx.ActorContext) (managedconnector.OwnerScope, error) {
	scope, err := a.currentConnectorBindingScope(ctx, actor)
	if err != nil {
		return managedconnector.OwnerScope{}, err
	}
	stateRoot := config.NewProfilePathResolver(actor).ConnectorStateDir()
	owner := managedconnector.OwnerScope{
		ProfileID: scope.ProfileID, CloudUserID: scope.CloudUserID, BindingGeneration: scope.BindingGeneration, StateRoot: stateRoot,
	}
	if err := owner.Validate(); err != nil {
		return managedconnector.OwnerScope{}, err
	}
	return owner, nil
}

func (a *App) currentConnectorBindingScope(ctx context.Context, actor userctx.ActorContext) (cloud.BindingScope, error) {
	bindings := a.connectorBindingResolver
	if bindings == nil && a.cloudRuntime != nil {
		bindings = a.cloudRuntime.bindings
	}
	if bindings == nil {
		return cloud.BindingScope{}, errors.New("connector binding resolver is unavailable")
	}
	return currentCloudAssetScope(ctx, bindings, actor)
}

func (a *App) confirmConnectorOwnerScope(ctx context.Context, actor userctx.ActorContext, expected managedconnector.OwnerScope) error {
	current, err := a.currentConnectorBindingScope(ctx, actor)
	if err != nil {
		return err
	}
	if current.ProfileID != expected.ProfileID || current.CloudUserID != expected.CloudUserID || current.BindingGeneration != expected.BindingGeneration {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "cloud: connector binding changed before state write"}
	}
	return nil
}

func connectorStateView(snapshot managedconnector.Snapshot) ConnectorStateView {
	return ConnectorStateView{
		ConnectorID: snapshot.ConnectorID, Status: snapshot.Status, Version: snapshot.Version, SHA256: snapshot.SHA256, Error: snapshot.Error,
	}
}
