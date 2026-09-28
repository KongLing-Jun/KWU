package main

import (
	"context"
	"testing"

	managedconnector "kwu/desktop/internal/connector"
)

type projectConnectorCatalogRuntime struct {
	catalog   managedconnector.Catalog
	installed string
}

func (*projectConnectorCatalogRuntime) Start(context.Context) error { return nil }
func (*projectConnectorCatalogRuntime) Close() error                { return nil }
func (r *projectConnectorCatalogRuntime) Catalog() (managedconnector.Catalog, error) {
	return r.catalog, nil
}
func (r *projectConnectorCatalogRuntime) RefreshCatalog(context.Context) (managedconnector.Catalog, error) {
	return r.catalog, nil
}
func (r *projectConnectorCatalogRuntime) Install(_ context.Context, id string, _ managedconnector.OwnerScope, _ managedconnector.ScopeConfirmation) (managedconnector.Snapshot, error) {
	r.installed = id
	return managedconnector.Snapshot{ConnectorID: id, Status: managedconnector.StatusAuthorizationNeeded}, nil
}
func (*projectConnectorCatalogRuntime) Snapshot(id string, _ managedconnector.OwnerScope) (managedconnector.Snapshot, error) {
	return managedconnector.Snapshot{ConnectorID: id, Status: managedconnector.StatusConnected}, nil
}
func (*projectConnectorCatalogRuntime) SetOwnerStatus(managedconnector.OwnerScope, string, managedconnector.Status, *managedconnector.SafeError) (managedconnector.Snapshot, error) {
	return managedconnector.Snapshot{}, nil
}
func (*projectConnectorCatalogRuntime) Manifest(string) (managedconnector.ActiveVersion, string, managedconnector.Manifest, error) {
	return managedconnector.ActiveVersion{}, "", managedconnector.Manifest{}, nil
}
func (*projectConnectorCatalogRuntime) CatalogItem(string) (managedconnector.CatalogItem, error) {
	return managedconnector.CatalogItem{}, nil
}

func TestProjectConnectorAvailabilityUsesDynamicCatalogIdentity(t *testing.T) {
	catalog := ConnectorCatalogView{Items: []ConnectorCatalogItemView{
		{ID: "lexiang", Name: "乐享知识库", Description: "知识库", Compatible: true},
		{ID: "canva", Name: "Canva可画", Description: "设计", Compatible: true},
	}}
	states := []ConnectorStateView{
		{ConnectorID: "lexiang", Status: managedconnector.StatusConnected},
		{ConnectorID: "canva", Status: managedconnector.StatusAuthorizationNeeded},
	}

	items := projectConnectorAvailability(catalog, states)
	if len(items) != 2 {
		t.Fatalf("availability items = %d, want 2", len(items))
	}
	if items[0].RegistryRef != "lexiang" || !items[0].Available || items[0].LocalServerName != "lexiang" || items[0].LocalStatus != "connected" {
		t.Fatalf("connected dynamic connector = %+v", items[0])
	}
	if items[1].RegistryRef != "canva" || items[1].Available || items[1].LocalStatus != "authorization_required" || items[1].AvailabilityReason != "尚未连接" {
		t.Fatalf("unconnected dynamic connector = %+v", items[1])
	}
}

func TestProjectConnectorAvailabilityReportsCompatibilityAndRepair(t *testing.T) {
	catalog := ConnectorCatalogView{Items: []ConnectorCatalogItemView{
		{ID: "blocked", Name: "Blocked", Compatible: false, BlockedReason: "需要更新客户端"},
		{ID: "repair", Name: "Repair", Compatible: true},
	}}
	states := []ConnectorStateView{{ConnectorID: "repair", Status: managedconnector.StatusRepairNeeded}}
	items := projectConnectorAvailability(catalog, states)
	if items[0].AvailabilityReason != "需要更新客户端" {
		t.Fatalf("blocked reason = %q", items[0].AvailabilityReason)
	}
	if items[1].AvailabilityReason != "需要重新连接" {
		t.Fatalf("repair reason = %q", items[1].AvailabilityReason)
	}
}

func TestProjectConnectorAvailabilityKeepsCatalogUsableWithoutOwnerState(t *testing.T) {
	catalog := ConnectorCatalogView{Items: []ConnectorCatalogItemView{
		{ID: "github", Name: "GitHub", Description: "代码仓库", Compatible: true},
	}}
	items := projectConnectorAvailability(catalog, nil)
	if len(items) != 1 || items[0].RegistryRef != "github" {
		t.Fatalf("catalog-only items = %+v", items)
	}
	if items[0].Available || items[0].AvailabilityReason != "尚未连接" {
		t.Fatalf("catalog-only availability = %+v", items[0])
	}
}

func TestListProjectConnectorAvailabilityDoesNotDependOnAnActiveConversationTab(t *testing.T) {
	app, actor, _, _ := newConnectorOAuthRefreshTestApp(t)
	app.setCurrentActorContext(actor)
	app.connectorService = &projectConnectorCatalogRuntime{catalog: managedconnector.Catalog{Items: []managedconnector.CatalogItem{{ID: "github", Name: "GitHub", Compatible: true}}}}
	app.activeTabID = "stale-conversation"
	app.tabRuntimes = map[string]*WorkspaceTab{}

	items, err := app.ListProjectConnectorAvailability()
	if err != nil {
		t.Fatalf("project connector catalog depends on an active conversation: %v", err)
	}
	if len(items) != 1 || items[0].RegistryRef != "github" || !items[0].Available {
		t.Fatalf("project connector availability = %+v", items)
	}
}

func TestInstallProjectConnectorUsesAuthenticatedOwnerWithoutConversationTab(t *testing.T) {
	app, actor, _, _ := newConnectorOAuthRefreshTestApp(t)
	app.setCurrentActorContext(actor)
	runtimeService := &projectConnectorCatalogRuntime{catalog: managedconnector.Catalog{Items: []managedconnector.CatalogItem{{ID: "notion", Name: "Notion", Compatible: true}}}}
	app.connectorService = runtimeService
	app.activeTabID = ""
	app.tabRuntimes = map[string]*WorkspaceTab{}

	state, err := app.InstallProjectConnector("notion")
	if err != nil {
		t.Fatalf("InstallProjectConnector: %v", err)
	}
	if runtimeService.installed != "notion" || state.ConnectorID != "notion" || state.Status != managedconnector.StatusAuthorizationNeeded {
		t.Fatalf("installed state = %+v, runtime id = %q", state, runtimeService.installed)
	}
}
