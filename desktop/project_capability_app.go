package main

import (
	"crypto/rand"
	"encoding/base64"

	"kwu/internal/project"
)

type CloudProjectCapabilityBindingInput struct {
	Kind        string         `json:"kind"`
	RegistryRef string         `json:"registryRef"`
	Position    int            `json:"position"`
	Config      map[string]any `json:"config"`
}

type SaveCloudProjectCapabilitiesInput struct {
	ProjectID       string                                `json:"projectId"`
	ExpectedVersion uint64                                `json:"expectedVersion"`
	Bindings        *[]CloudProjectCapabilityBindingInput `json:"bindings,omitempty"`
	SkillIDSet      bool                                  `json:"skillIdSet,omitempty"`
	SkillID         *string                               `json:"skillId"`
	ExpertIDSet     bool                                  `json:"expertIdSet,omitempty"`
	ExpertID        *string                               `json:"expertId"`
	ConnectorIDSet  bool                                  `json:"connectorIdSet,omitempty"`
	ConnectorID     *string                               `json:"connectorId"`
}

type CloudProjectCapabilityBindingView struct {
	BindingID      string         `json:"bindingId"`
	Kind           string         `json:"kind"`
	RegistryRef    string         `json:"registryRef"`
	Position       int            `json:"position"`
	Config         map[string]any `json:"config"`
	PublicDisabled bool           `json:"publicDisabled"`
	Version        uint64         `json:"version"`
}

type CloudProjectCapabilityListView struct {
	ProjectScopeKey string                              `json:"projectScopeKey"`
	ProjectID       string                              `json:"projectId"`
	ProjectVersion  uint64                              `json:"projectVersion"`
	Items           []CloudProjectCapabilityBindingView `json:"items"`
}

func (a *App) ListCloudProjectCapabilities(projectID string) (CloudProjectCapabilityListView, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return CloudProjectCapabilityListView{}, projectWailsError(err)
	}
	if !features.Bindings {
		return CloudProjectCapabilityListView{}, cloudCapabilityUnavailable("project bindings")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	result, err := services.Gateway.ListBindings(ctx, projectID)
	if err != nil {
		return CloudProjectCapabilityListView{}, projectProtocolWailsError(err)
	}
	return cloudProjectCapabilityListView(result), nil
}

func (a *App) SaveCloudProjectCapabilities(input SaveCloudProjectCapabilitiesInput) (CloudProjectCapabilityListView, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return CloudProjectCapabilityListView{}, projectWailsError(err)
	}
	if !features.Bindings {
		return CloudProjectCapabilityListView{}, cloudCapabilityUnavailable("project bindings")
	}
	key, err := newProjectCapabilityKey()
	if err != nil {
		return CloudProjectCapabilityListView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "project capability command could not be prepared")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	var result project.ProjectBindingList
	if input.Bindings != nil {
		bindings := make([]project.ProjectBindingInput, len(*input.Bindings))
		for i, binding := range *input.Bindings {
			bindings[i] = project.ProjectBindingInput{Kind: binding.Kind, RegistryRef: binding.RegistryRef, Position: binding.Position, Config: binding.Config}
		}
		result, err = services.Gateway.PutBindings(ctx, input.ProjectID, input.ExpectedVersion, bindings, key)
	} else {
		result, err = services.Gateway.PatchBindings(ctx, input.ProjectID, project.PatchProjectBindingsInput{
			ExpectedVersion: input.ExpectedVersion,
			SkillID:         project.NullableStringUpdate{Set: input.SkillIDSet, Value: cloneProjectString(input.SkillID)},
			ExpertID:        project.NullableStringUpdate{Set: input.ExpertIDSet, Value: cloneProjectString(input.ExpertID)},
			ConnectorID:     project.NullableStringUpdate{Set: input.ConnectorIDSet, Value: cloneProjectString(input.ConnectorID)},
		}, key)
	}
	if err != nil {
		return CloudProjectCapabilityListView{}, projectProtocolWailsError(err)
	}
	return cloudProjectCapabilityListView(result), nil
}

func cloudProjectCapabilityListView(result project.ProjectBindingList) CloudProjectCapabilityListView {
	items := make([]CloudProjectCapabilityBindingView, len(result.Items))
	for i, item := range result.Items {
		items[i] = CloudProjectCapabilityBindingView{BindingID: item.BindingID, Kind: item.Kind, RegistryRef: item.RegistryRef, Position: item.Position, Config: item.Config, PublicDisabled: item.PublicDisabled, Version: item.Version}
	}
	return CloudProjectCapabilityListView{ProjectScopeKey: result.ProjectScopeKey, ProjectID: result.ProjectID, ProjectVersion: result.ProjectVersion, Items: items}
}

func newProjectCapabilityKey() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "project-capability-" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
