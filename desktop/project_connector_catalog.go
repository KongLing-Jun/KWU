package main

import (
	"strings"

	managedconnector "kwu/desktop/internal/connector"
)

type ProjectConnectorAvailabilityView struct {
	RegistryRef        string `json:"registryRef"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	IconURL            string `json:"iconUrl,omitempty"`
	Compatible         bool   `json:"compatible"`
	LocalServerName    string `json:"localServerName"`
	LocalStatus        string `json:"localStatus"`
	Available          bool   `json:"available"`
	AvailabilityReason string `json:"availabilityReason"`
}

// InstallProjectConnector installs a Cloud Catalog connector for the current
// authenticated desktop owner without depending on an ordinary conversation tab.
func (a *App) InstallProjectConnector(connectorID string) (ConnectorStateView, error) {
	ctx := a.bootContext()
	actor := a.cloudActorContext()
	owner, err := a.connectorOwnerScopeForActor(ctx, actor)
	if err != nil {
		return ConnectorStateView{}, err
	}
	return a.installConnectorForOwner(ctx, owner, actor, connectorID)
}

// ListProjectConnectorAvailability projects the Cloud Catalog through the
// current owner's local connector state. Project bindings keep the catalog ID;
// credentials and runtime names remain device-local.
func (a *App) ListProjectConnectorAvailability() ([]ProjectConnectorAvailabilityView, error) {
	catalog, err := a.ConnectorCatalog("")
	if err != nil {
		return nil, err
	}
	// Project capability configuration belongs to the authenticated desktop
	// owner, not to whichever ordinary conversation happens to be active. A
	// project can be opened with no tab or with a stale tab after navigation.
	owner, err := a.connectorOwnerScopeForActor(a.bootContext(), a.cloudActorContext())
	if err != nil {
		// Keep the Cloud Catalog usable while the owner state is unavailable;
		// connection side effects still require an exact owner scope elsewhere.
		return projectConnectorAvailability(catalog, nil), nil
	}
	runtimeService, err := a.connectorRuntime()
	if err != nil {
		return projectConnectorAvailability(catalog, nil), nil
	}
	states := make([]ConnectorStateView, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		snapshot, snapshotErr := runtimeService.Snapshot(item.ID, owner)
		if snapshotErr != nil {
			continue
		}
		states = append(states, connectorStateView(snapshot))
	}
	return projectConnectorAvailability(catalog, states), nil
}

func projectConnectorAvailability(catalog ConnectorCatalogView, states []ConnectorStateView) []ProjectConnectorAvailabilityView {
	stateByID := make(map[string]ConnectorStateView, len(states))
	for _, state := range states {
		stateByID[state.ConnectorID] = state
	}
	items := make([]ProjectConnectorAvailabilityView, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		state := stateByID[item.ID]
		localStatus := state.Status
		if localStatus == "" {
			localStatus = managedconnector.StatusNotInstalled
		}
		available := item.Compatible && state.Status == managedconnector.StatusConnected
		view := ProjectConnectorAvailabilityView{
			RegistryRef:        item.ID,
			Name:               item.Name,
			Description:        item.Description,
			IconURL:            item.IconURL,
			Compatible:         item.Compatible,
			LocalStatus:        string(localStatus),
			Available:          available,
			AvailabilityReason: projectConnectorAvailabilityReason(item, state),
		}
		if available {
			// The catalog ID is the stable selection consumed by
			// expandConnectorSelections; it is not a raw MCP server name.
			view.LocalServerName = item.ID
		}
		items = append(items, view)
	}
	return items
}

func projectConnectorAvailabilityReason(item ConnectorCatalogItemView, state ConnectorStateView) string {
	if !item.Compatible {
		if reason := strings.TrimSpace(item.BlockedReason); reason != "" {
			return reason
		}
		return "当前客户端版本不兼容"
	}
	switch state.Status {
	case managedconnector.StatusConnected:
		return ""
	case managedconnector.StatusAuthorizing:
		return "正在授权"
	case managedconnector.StatusRepairNeeded:
		return "需要重新连接"
	case managedconnector.StatusFailed:
		if state.Error != nil && strings.TrimSpace(state.Error.Message) != "" {
			return state.Error.Message
		}
		return "连接失败，请重试"
	default:
		return "尚未连接"
	}
}
