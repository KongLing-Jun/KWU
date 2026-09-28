package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
)

// CloudProjectListInput selects one server-ordered CloudProject page.
type CloudProjectListInput struct {
	PageSize int    `json:"pageSize,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Status   string `json:"status,omitempty"`
	Search   string `json:"search,omitempty"`
}

// CreateCloudProjectInput contains only user intent. Go owns the durable
// idempotency key and never accepts one from React.
type CreateCloudProjectInput struct {
	Name             string                               `json:"name"`
	DescriptionSet   bool                                 `json:"descriptionSet,omitempty"`
	Description      *string                              `json:"description"`
	HumanDescription *string                              `json:"humanDescription,omitempty"`
	AgentInstruction *string                              `json:"agentInstruction,omitempty"`
	SkillID          *string                              `json:"skillId,omitempty"`
	ExpertID         *string                              `json:"expertId,omitempty"`
	ConnectorID      *string                              `json:"connectorId,omitempty"`
	Bindings         []CloudProjectCapabilityBindingInput `json:"bindings,omitempty"`
}

// PatchCloudProjectInput distinguishes an omitted description from an explicit
// null while carrying the authoritative version used for optimistic locking.
type PatchCloudProjectInput struct {
	ProjectID       string  `json:"projectId"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	Name            *string `json:"name,omitempty"`
	DescriptionSet  bool    `json:"descriptionSet,omitempty"`
	Description     *string `json:"description"`
}

// ProjectVersionMutationInput is used by archive and restore transitions.
type ProjectVersionMutationInput struct {
	ProjectID       string `json:"projectId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

// CloudProjectView is the camelCase Wails representation of one authoritative
// Cloud project. Owner identity and local paths are deliberately absent.
type CloudProjectView struct {
	ProjectID   string                    `json:"projectId"`
	Name        string                    `json:"name"`
	Description projectNullableStringView `json:"description" ts_type:"string | null"`
	Status      string                    `json:"status"`
	Version     uint64                    `json:"version"`
	CreatedAt   string                    `json:"createdAt"`
	UpdatedAt   string                    `json:"updatedAt"`
}

// CloudProjectPageView carries the decode-time opaque scope identity used by
// React to reject late results after an account switch.
type CloudProjectPageView struct {
	ProjectScopeKey string                    `json:"projectScopeKey"`
	Items           []CloudProjectView        `json:"items"`
	NextCursor      projectNullableStringView `json:"nextCursor" ts_type:"string | null"`
	Stale           bool                      `json:"stale"`
	CachedAt        projectNullableStringView `json:"cachedAt" ts_type:"string | null"`
}

// CloudProjectOnboardingView exposes Cloud's real guide project without
// allowing React to choose its identity or durable content.
type CloudProjectOnboardingView struct {
	ProjectScopeKey string            `json:"projectScopeKey"`
	Project         *CloudProjectView `json:"project"`
	GuideVersion    uint64            `json:"guideVersion"`
	State           string            `json:"state"`
	Created         bool              `json:"created"`
}

// Wails v2 makes pointer fields optional in generated TypeScript. This value
// wrapper keeps contract-required string-or-null fields required.
type projectNullableStringView struct{ value *string }

func (value projectNullableStringView) MarshalJSON() ([]byte, error) {
	return json.Marshal(value.value)
}

// CloudProjectResultView stamps one project read or mutation readback.
type CloudProjectResultView struct {
	ProjectScopeKey string                    `json:"projectScopeKey"`
	Project         CloudProjectView          `json:"project"`
	WorkspacePath   string                    `json:"workspacePath"`
	Stale           bool                      `json:"stale"`
	CachedAt        projectNullableStringView `json:"cachedAt" ts_type:"string | null"`
}

type CloudProjectFeatureView struct {
	ProjectScopeKey         string `json:"projectScopeKey"`
	InstructionsAvailable   bool   `json:"instructionsAvailable"`
	WorkItemsAvailable      bool   `json:"workItemsAvailable"`
	MembersAvailable        bool   `json:"membersAvailable"`
	ActivityAvailable       bool   `json:"activityAvailable"`
	WorkItemFieldsAvailable bool   `json:"workItemFieldsAvailable"`
	BindingsAvailable       bool   `json:"bindingsAvailable"`
	AssetsAvailable         bool   `json:"assetsAvailable"`
}

func (a *App) GetCloudProjectFeatures() (CloudProjectFeatureView, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return CloudProjectFeatureView{}, projectWailsError(err)
	}
	return CloudProjectFeatureView{ProjectScopeKey: services.Scope.Key, InstructionsAvailable: features.Instructions, WorkItemsAvailable: features.WorkItems, MembersAvailable: features.Members, ActivityAvailable: features.Activity, WorkItemFieldsAvailable: features.WorkItemFields, BindingsAvailable: features.Bindings, AssetsAvailable: features.Assets}, nil
}

// ListCloudProjects returns the authoritative owner-created project directory
// from Cloud. The directory must never fall back to a device-local snapshot:
// callers use an error state to distinguish an unavailable server from a
// current server result.
func (a *App) ListCloudProjects(input CloudProjectListInput) (CloudProjectPageView, error) {
	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectPageView{}, projectWailsError(err)
	}
	defaultCacheQuery := isDefaultProjectCacheQuery(input)
	var directoryRequest uint64
	if defaultCacheQuery {
		directoryRequest = services.beginProjectDirectoryRefresh()
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	page, err := services.Gateway.ListOwnedProjects(ctx, project.ListProjectsInput{
		PageSize: input.PageSize, Cursor: input.Cursor, Status: project.ProjectStatus(input.Status), Search: input.Search,
	})
	if err != nil {
		return CloudProjectPageView{}, projectProtocolWailsError(err)
	}
	if defaultCacheQuery {
		if stored, cacheErr := services.storeProjectDirectoryRefresh(ctx, directoryRequest, page); cacheErr == nil && stored {
			a.emitProjectTreeChanged()
		}
	}
	return cloudProjectPageView(page, false, nil), nil
}

func (s *cloudProjectServices) beginProjectDirectoryRefresh() uint64 {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	s.directoryNext++
	return s.directoryNext
}

func (s *cloudProjectServices) storeProjectDirectoryRefresh(ctx context.Context, request uint64, page project.ProjectPage) (bool, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	if request < s.directoryMinimum || request < s.directoryApplied {
		return false, nil
	}
	if err := s.Cache.StoreSummaries(ctx, s.Scope, page); err != nil {
		return false, err
	}
	s.directoryApplied = request
	return true, nil
}

func (s *cloudProjectServices) storeCreatedProject(ctx context.Context, created project.Project) error {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	// Any directory request already in flight may describe the state before
	// this create. Require a request started after the create to replace the
	// optimistic, server-compatible head ordering.
	s.directoryMinimum = s.directoryNext + 1
	return s.Cache.StoreCreatedProject(ctx, s.Scope, created)
}

// ListJoinedCloudProjects reads the current user's member-only project list.
// Its rebuildable cache directory remains separate from owner-created projects
// so ownership and member permissions cannot be conflated by workspace views.
func (a *App) ListJoinedCloudProjects(input CloudProjectListInput) (CloudProjectPageView, error) {
	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectPageView{}, projectWailsError(err)
	}
	defaultCacheQuery := isDefaultProjectCacheQuery(input)
	var directoryRequest uint64
	if defaultCacheQuery {
		directoryRequest = services.beginJoinedProjectDirectoryRefresh()
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	page, err := services.Gateway.ListJoinedProjects(ctx, project.ListProjectsInput{
		PageSize: input.PageSize, Cursor: input.Cursor, Status: project.ProjectStatus(input.Status), Search: input.Search,
	})
	if err != nil {
		return CloudProjectPageView{}, projectProtocolWailsError(err)
	}
	if defaultCacheQuery {
		if stored, cacheErr := services.storeJoinedProjectDirectoryRefresh(ctx, directoryRequest, page); cacheErr == nil && stored {
			a.emitProjectTreeChanged()
		}
	}
	return cloudProjectPageView(page, false, nil), nil
}

// refreshCloudProjectDirectoriesAfterAuthentication synchronously primes both
// workspace directories before a successful login/bootstrap is returned to
// the UI. Project-directory failures stay best-effort so an otherwise valid
// account login is not converted into an authentication failure.
func (a *App) refreshCloudProjectDirectoriesAfterAuthentication() {
	if _, err := a.projectServicesForOperation(); err != nil {
		return
	}
	input := CloudProjectListInput{PageSize: 100, Status: string(project.ProjectActive)}
	refreshes := []struct {
		name string
		run  func() error
	}{
		{name: "owned", run: func() error { _, err := a.ListCloudProjects(input); return err }},
		{name: "joined", run: func() error { _, err := a.ListJoinedCloudProjects(input); return err }},
	}
	var wait sync.WaitGroup
	for _, refresh := range refreshes {
		refresh := refresh
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := refresh.run(); err != nil {
				slog.Warn("cloud project workspace directory refresh after authentication failed", "directory", refresh.name, "err", err)
			}
		}()
	}
	wait.Wait()
}

func (s *cloudProjectServices) beginJoinedProjectDirectoryRefresh() uint64 {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	s.joinedDirectoryNext++
	return s.joinedDirectoryNext
}

func (s *cloudProjectServices) storeJoinedProjectDirectoryRefresh(ctx context.Context, request uint64, page project.ProjectPage) (bool, error) {
	s.directoryMu.Lock()
	defer s.directoryMu.Unlock()
	if request < s.joinedDirectoryApplied {
		return false, nil
	}
	if err := s.Cache.StoreJoinedSummaries(ctx, s.Scope, page); err != nil {
		return false, err
	}
	s.joinedDirectoryApplied = request
	return true, nil
}

func (a *App) ClaimCloudProjectOnboarding() (CloudProjectOnboardingView, error) {
	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectOnboardingView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	claim, err := services.Gateway.ClaimOnboardingProject(ctx)
	if err != nil {
		return CloudProjectOnboardingView{}, projectProtocolWailsError(err)
	}
	view := CloudProjectOnboardingView{
		ProjectScopeKey: claim.ProjectScopeKey,
		GuideVersion:    claim.GuideVersion,
		State:           string(claim.State),
		Created:         claim.Created,
	}
	if claim.Project != nil {
		projectView := cloudProjectView(*claim.Project)
		view.Project = &projectView
		_ = services.Cache.StoreDetail(ctx, services.Scope, *claim.Project)
	}
	return view, nil
}

func isDefaultProjectCacheQuery(input CloudProjectListInput) bool {
	status := project.ProjectStatus(input.Status)
	return input.Cursor == "" && input.Search == "" && (status == "" || status == project.ProjectActive)
}

func cloudProjectPageView(page project.ProjectPage, stale bool, cachedAt *time.Time) CloudProjectPageView {
	view := page.View()
	items := make([]CloudProjectView, len(view.Items))
	for index, item := range view.Items {
		items[index] = cloudProjectViewFromDomainView(item)
	}
	var cachedAtText *string
	if cachedAt != nil && !cachedAt.IsZero() {
		value := cachedAt.UTC().Format(time.RFC3339Nano)
		cachedAtText = &value
	}
	return CloudProjectPageView{
		ProjectScopeKey: view.ProjectScopeKey,
		Items:           items,
		NextCursor:      projectNullableStringView{value: cloneProjectString(view.NextCursor)},
		Stale:           stale,
		CachedAt:        projectNullableStringView{value: cachedAtText},
	}
}

// GetCloudProject returns one authoritative Cloud read stamped with its scope.
func (a *App) GetCloudProject(projectID string) (CloudProjectResultView, error) {
	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectResultView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	result, err := services.Gateway.GetProject(ctx, projectID)
	if err != nil {
		cached, found, cacheErr := services.Cache.ResolveFallback(ctx, services.Scope, err)
		if cacheErr != nil {
			return CloudProjectResultView{}, projectLocalWailsError(cacheErr, "PROJECT_CACHE_UNAVAILABLE", "cached project is unavailable")
		}
		if found {
			for _, detail := range cached.Details {
				if detail.ProjectID == projectID {
					view := cloudProjectResultView(project.ProjectResult{ProjectScopeKey: services.Scope.Key, Project: detail}, true, &cached.CachedAt)
					workspacePath, workspaceErr := ensureCloudProjectWorkspace(ctx, services, detail.ProjectID)
					if workspaceErr != nil {
						return CloudProjectResultView{}, projectLocalWailsError(workspaceErr, "PROJECT_WORKSPACE_UNAVAILABLE", "dedicated project workspace is unavailable")
					}
					view.WorkspacePath = workspacePath
					return view, nil
				}
			}
		}
		return CloudProjectResultView{}, projectProtocolWailsError(err)
	}
	if cacheErr := services.Cache.StoreDetail(ctx, services.Scope, result.Project); cacheErr == nil {
		a.emitProjectTreeChanged()
	}
	view := cloudProjectResultView(result, false, nil)
	workspacePath, err := ensureCloudProjectWorkspace(ctx, services, result.Project.ProjectID)
	if err != nil {
		return CloudProjectResultView{}, projectLocalWailsError(err, "PROJECT_WORKSPACE_UNAVAILABLE", "dedicated project workspace is unavailable")
	}
	view.WorkspacePath = workspacePath
	return view, nil
}

// CreateCloudProject durably prepares a Go-generated command before HTTP and
// completes it only after an authoritative GET agrees with the receipt.
func (a *App) CreateCloudProject(input CreateCloudProjectInput) (CloudProjectResultView, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return CloudProjectResultView{}, projectWailsError(err)
	}
	domainInput := project.CreateProjectInput{
		Name: input.Name, Description: project.NullableStringUpdate{Set: input.DescriptionSet, Value: cloneProjectString(input.Description)},
		HumanDescription: cloneProjectString(input.HumanDescription), AgentInstruction: cloneProjectString(input.AgentInstruction),
		SkillID: cloneProjectString(input.SkillID), ExpertID: cloneProjectString(input.ExpertID), ConnectorID: cloneProjectString(input.ConnectorID),
	}
	payload := map[string]any{"name": input.Name}
	if input.DescriptionSet {
		payload["description"] = input.Description
	}
	if input.HumanDescription != nil {
		payload["human_description"] = input.HumanDescription
	}
	if input.AgentInstruction != nil {
		payload["agent_instruction"] = input.AgentInstruction
	}
	if input.SkillID != nil {
		payload["skill_id"] = input.SkillID
	}
	if input.ExpertID != nil {
		payload["expert_id"] = input.ExpertID
	}
	if input.ConnectorID != nil {
		payload["connector_id"] = input.ConnectorID
	}
	if len(input.Bindings) > 0 {
		payload["bindings"] = cloudProjectBindingInputs(input.Bindings)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	var created CloudProjectResultView
	if len(input.Bindings) > 0 && features.Members {
		created, err = runCloudProjectAggregateCreate(ctx, services, input, payload)
	} else {
		if len(input.Bindings) > 0 {
			return CloudProjectResultView{}, cloudCapabilityUnavailable("atomic project bindings")
		}
		created, err = runCloudProjectMutation(ctx, services, project.ProjectCommandCreate, "", payload, func(key string) (project.CommandResult, error) {
			return services.Gateway.CreateProject(ctx, domainInput, key)
		}, func(projectID string) error {
			if input.HumanDescription == nil && input.AgentInstruction == nil {
				return nil
			}
			return verifyInitialCloudProjectInstruction(ctx, services, projectID, input.HumanDescription, input.AgentInstruction)
		})
	}
	if err != nil {
		return CloudProjectResultView{}, err
	}
	workspacePath, workspaceErr := ensureCloudProjectWorkspace(ctx, services, created.Project.ProjectID)
	if workspaceErr != nil {
		return CloudProjectResultView{}, projectLocalWailsError(workspaceErr, "PROJECT_WORKSPACE_UNAVAILABLE", "dedicated project workspace is unavailable")
	}
	created.WorkspacePath = workspacePath
	if cacheErr := services.storeCreatedProject(ctx, cloudProjectFromView(created.Project, services.Scope.Binding.CloudUserID)); cacheErr != nil {
		slog.Warn("cloud project was created but its workspace cache could not be updated", "project_id", created.Project.ProjectID, "err", cacheErr)
		return created, nil
	}
	a.emitProjectTreeChanged()
	return created, nil
}

func cloudProjectFromView(view CloudProjectView, ownerUserID string) project.Project {
	createdAt, _ := time.Parse(time.RFC3339Nano, view.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, view.UpdatedAt)
	return project.Project{
		ProjectID: view.ProjectID, OwnerUserID: ownerUserID, Name: view.Name, Description: cloneProjectString(view.Description.value),
		Status: project.ProjectStatus(view.Status), Version: view.Version, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

func runCloudProjectAggregateCreate(ctx context.Context, services *cloudProjectServices, input CreateCloudProjectInput, payload map[string]any) (CloudProjectResultView, error) {
	if err := waitForCloudProjectCommandReplay(ctx, services); err != nil {
		return CloudProjectResultView{}, projectCommandPrepareWailsError(err)
	}
	record, err := services.Journal.Prepare(ctx, services.Scope, project.PrepareProjectCommandInput{Operation: project.ProjectCommandCreate, Payload: payload})
	if err != nil {
		return CloudProjectResultView{}, projectCommandPrepareWailsError(err)
	}
	workBuddyProjectFaultPoint("journal_durable_before_http", record.IdempotencyKey)
	human, agentInstruction := "", ""
	if input.HumanDescription != nil {
		human = *input.HumanDescription
	}
	if input.AgentInstruction != nil {
		agentInstruction = *input.AgentInstruction
	}
	bindings := cloudProjectBindingInputs(input.Bindings)
	created, err := services.Gateway.CreateAggregate(ctx, project.AggregateCreateInput{
		Name: input.Name, Description: project.NullableStringUpdate{Set: input.DescriptionSet, Value: cloneProjectString(input.Description)},
		HumanDescription: human, AgentInstruction: agentInstruction, Bindings: bindings,
	}, record.IdempotencyKey)
	if err != nil {
		err = settleProjectCommandFailure(ctx, services, record.IdempotencyKey, err)
		return CloudProjectResultView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("response_before_authoritative_readback", record.IdempotencyKey)
	readback, err := services.Gateway.GetProject(ctx, created.ProjectID)
	if err != nil {
		return CloudProjectResultView{}, projectProtocolWailsError(err)
	}
	if input.DescriptionSet && !equalProjectDescription(readback.Project.Description, input.Description) {
		return CloudProjectResultView{}, projectProtocolWailsError(fmt.Errorf("projects-v2: initial project description readback mismatch"))
	}
	if err := verifyInitialCloudProjectInstruction(ctx, services, created.ProjectID, &human, &agentInstruction); err != nil {
		return CloudProjectResultView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("readback_before_journal_eviction", record.IdempotencyKey)
	operationID := "op_local_" + record.IdempotencyKey[:32]
	result := project.CommandResult{ProjectScopeKey: services.Scope.Key, OperationID: operationID, ResourceKind: "project", ResourceID: created.ProjectID, ProjectID: created.ProjectID, CommittedVersion: created.ProjectVersion}
	proof := project.ProjectCommandReadback{ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "project", ResourceID: readback.Project.ProjectID, ProjectID: readback.Project.ProjectID, Version: readback.Project.Version}
	if err := services.Journal.CompleteAfterReadback(ctx, services.Scope, record.IdempotencyKey, result, proof); err != nil {
		return CloudProjectResultView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "project aggregate readback could not be completed")
	}
	return cloudProjectResultView(readback, false, nil), nil
}

func cloudProjectBindingInputs(bindings []CloudProjectCapabilityBindingInput) []project.ProjectBindingInput {
	result := make([]project.ProjectBindingInput, len(bindings))
	for i, binding := range bindings {
		result[i] = project.ProjectBindingInput{Kind: binding.Kind, RegistryRef: binding.RegistryRef, Position: binding.Position, Config: binding.Config}
	}
	return result
}

// PatchCloudProject applies editable fields with optimistic concurrency.
func (a *App) PatchCloudProject(input PatchCloudProjectInput) (CloudProjectResultView, error) {
	if err := validateCloudProjectExpectedVersion(input.ExpectedVersion); err != nil {
		return CloudProjectResultView{}, err
	}
	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectResultView{}, projectWailsError(err)
	}
	domainInput := project.PatchProjectInput{
		ExpectedVersion: input.ExpectedVersion, Name: cloneProjectString(input.Name),
		Description: project.NullableStringUpdate{Set: input.DescriptionSet, Value: cloneProjectString(input.Description)},
	}
	payload := map[string]any{"expected_version": input.ExpectedVersion}
	if input.Name != nil {
		payload["name"] = *input.Name
	}
	if input.DescriptionSet {
		payload["description"] = input.Description
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	return runCloudProjectMutation(ctx, services, project.ProjectCommandPatch, input.ProjectID, payload, func(key string) (project.CommandResult, error) {
		return services.Gateway.PatchProject(ctx, input.ProjectID, domainInput, key)
	}, nil)
}

// ArchiveCloudProject makes the CloudProject read-only without touching a
// LocalWorkspace binding or local files.
func (a *App) ArchiveCloudProject(input ProjectVersionMutationInput) (CloudProjectResultView, error) {
	status := project.ProjectArchived
	return a.transitionCloudProject(input, status)
}

// RestoreCloudProject returns an archived CloudProject to active state.
func (a *App) RestoreCloudProject(input ProjectVersionMutationInput) (CloudProjectResultView, error) {
	status := project.ProjectActive
	return a.transitionCloudProject(input, status)
}

func (a *App) transitionCloudProject(input ProjectVersionMutationInput, status project.ProjectStatus) (CloudProjectResultView, error) {
	if err := validateCloudProjectExpectedVersion(input.ExpectedVersion); err != nil {
		return CloudProjectResultView{}, err
	}
	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectResultView{}, projectWailsError(err)
	}
	domainInput := project.PatchProjectInput{ExpectedVersion: input.ExpectedVersion, Status: &status}
	payload := map[string]any{"expected_version": input.ExpectedVersion, "status": status}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	return runCloudProjectMutation(ctx, services, project.ProjectCommandPatch, input.ProjectID, payload, func(key string) (project.CommandResult, error) {
		return services.Gateway.PatchProject(ctx, input.ProjectID, domainInput, key)
	}, nil)
}

func validateCloudProjectExpectedVersion(version uint64) error {
	if version == 0 || version > project.MaxExpectedVersion {
		return &cloud.Error{Code: cloud.ErrorCode("VALIDATION_FAILED"), Message: "project expected version is invalid"}
	}
	return nil
}

func (a *App) projectServicesForOperation() (*cloudProjectServices, error) {
	services, _, err := a.projectServicesAndFeaturesForOperation()
	return services, err
}

func (a *App) projectServicesAndFeaturesForOperation() (*cloudProjectServices, project.CapabilityFeatures, error) {
	runtimeService, err := a.cloudRuntimeOrUnavailable()
	if err != nil {
		return nil, project.CapabilityFeatures{}, err
	}
	runtimeService.projectMu.RLock()
	status := runtimeService.projectCapability
	services := runtimeService.projects
	features := runtimeService.projectFeatures
	runtimeService.projectMu.RUnlock()
	switch status {
	case project.CapabilityAvailable:
		if services == nil || services.Gateway == nil || services.Cache == nil || services.Bindings == nil || services.Journal == nil {
			return nil, project.CapabilityFeatures{}, &cloud.Error{Code: cloud.ErrorCode("INTERNAL_ERROR"), Message: "cloud project services are not initialized"}
		}
		return services, features, nil
	default:
		return nil, project.CapabilityFeatures{}, cloudCapabilityUnavailable("projects")
	}
}

func (a *App) projectServicesForWorkItemOperation() (*cloudProjectServices, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return nil, err
	}
	if !features.WorkItems {
		return nil, cloudCapabilityUnavailable("project work items")
	}
	return services, nil
}

func (a *App) projectServicesForAssetOperation() (*cloudProjectServices, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return nil, err
	}
	if !features.Assets {
		return nil, cloudCapabilityUnavailable("project assets")
	}
	return services, nil
}

func (a *App) projectServicesForInstructionOperation() (*cloudProjectServices, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return nil, err
	}
	if !features.Instructions {
		return nil, cloudCapabilityUnavailable("project instructions")
	}
	return services, nil
}

func runCloudProjectMutation(ctx context.Context, services *cloudProjectServices, operation project.ProjectCommandOperation, projectID string, payload map[string]any, send func(string) (project.CommandResult, error), verify func(string) error) (CloudProjectResultView, error) {
	if err := waitForCloudProjectCommandReplay(ctx, services); err != nil {
		return CloudProjectResultView{}, projectCommandPrepareWailsError(err)
	}
	record, err := services.Journal.Prepare(ctx, services.Scope, project.PrepareProjectCommandInput{
		Operation: operation, ProjectID: projectID, Payload: payload,
	})
	if err != nil {
		return CloudProjectResultView{}, projectCommandPrepareWailsError(err)
	}
	workBuddyProjectFaultPoint("journal_durable_before_http", record.IdempotencyKey)
	result, err := send(record.IdempotencyKey)
	if err != nil {
		err = settleProjectCommandFailure(ctx, services, record.IdempotencyKey, err)
		return CloudProjectResultView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("response_before_authoritative_readback", record.IdempotencyKey)
	readback, err := services.Gateway.GetProject(ctx, result.ProjectID)
	if err != nil {
		return CloudProjectResultView{}, projectProtocolWailsError(err)
	}
	if verify != nil {
		if err := verify(result.ProjectID); err != nil {
			return CloudProjectResultView{}, projectProtocolWailsError(err)
		}
	}
	workBuddyProjectFaultPoint("readback_before_journal_eviction", record.IdempotencyKey)
	projectReadback := project.ProjectCommandReadback{
		ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "project", ResourceID: readback.Project.ProjectID,
		ProjectID: readback.Project.ProjectID, Version: readback.Project.Version,
	}
	if err := services.Journal.CompleteAfterReadback(ctx, services.Scope, record.IdempotencyKey, result, projectReadback); err != nil {
		return CloudProjectResultView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "project command readback could not be completed")
	}
	return cloudProjectResultView(readback, false, nil), nil
}

func equalProjectDescription(actual, expected *string) bool {
	if actual == nil || expected == nil {
		return actual == nil && expected == nil
	}
	return *actual == *expected
}

func verifyInitialCloudProjectInstruction(ctx context.Context, services *cloudProjectServices, projectID string, humanDescription, agentInstruction *string) error {
	expectedHuman := ""
	if humanDescription != nil {
		expectedHuman = *humanDescription
	}
	expectedAgent := ""
	if agentInstruction != nil {
		expectedAgent = *agentInstruction
	}
	readback, err := services.Gateway.GetProjectInstruction(ctx, projectID)
	if err != nil {
		return err
	}
	got := readback.Instruction
	if got.Version != 1 || got.HumanDescription != expectedHuman || got.AgentInstruction != expectedAgent || got.ContentHash != project.InstructionContentSHA256(expectedHuman, expectedAgent) {
		return fmt.Errorf("projects-v1: initial project instruction readback mismatch")
	}
	return nil
}

func projectMutationResultUnknown(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var typed *cloud.Error
	return errors.As(err, &typed) && typed.Code == cloud.ErrorNetworkUnavailable
}

func cloudProjectResultView(result project.ProjectResult, stale bool, cachedAt *time.Time) CloudProjectResultView {
	var cachedAtText *string
	if cachedAt != nil && !cachedAt.IsZero() {
		value := cachedAt.UTC().Format(time.RFC3339Nano)
		cachedAtText = &value
	}
	return CloudProjectResultView{
		ProjectScopeKey: result.ProjectScopeKey,
		Project:         cloudProjectView(result.Project),
		Stale:           stale,
		CachedAt:        projectNullableStringView{value: cachedAtText},
	}
}

func cloudProjectView(value project.Project) CloudProjectView {
	return CloudProjectView{
		ProjectID: value.ProjectID, Name: value.Name, Description: projectNullableStringView{value: cloneProjectString(value.Description)}, Status: string(value.Status), Version: value.Version,
		CreatedAt: value.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func cloudProjectViewFromDomainView(value project.ProjectView) CloudProjectView {
	return CloudProjectView{
		ProjectID: value.ProjectID, Name: value.Name, Description: projectNullableStringView{value: cloneProjectString(value.Description)}, Status: value.Status,
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func cloneProjectString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func projectProtocolWailsError(err error) error {
	if err == nil {
		return nil
	}
	var typed *cloud.Error
	if errors.As(err, &typed) {
		return projectWailsErrorForOperation("project.protocol", err)
	}
	if projectRequestValidationError(err) {
		return projectDiagnosticWailsError("project.protocol", err, &cloud.Error{Code: cloud.ErrorCode("VALIDATION_FAILED"), Message: "project request is invalid"})
	}
	return projectLocalWailsErrorForOperation("project.protocol", err, "CLOUD_PROTOCOL_ERROR", "cloud project response is invalid")
}

func projectCommandPrepareWailsError(err error) error {
	if projectRequestValidationError(err) {
		return projectDiagnosticWailsError("project.command.prepare", err, &cloud.Error{Code: cloud.ErrorCode("VALIDATION_FAILED"), Message: "project request is invalid"})
	}
	return projectLocalWailsErrorForOperation("project.command.prepare", err, "PROJECT_COMMAND_UNAVAILABLE", "project command could not be prepared")
}

func projectRequestValidationError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	validationFragments := []string{
		" is required", " must be ", " must not ", "invalid project.", "invalid project ",
		"invalid project_", "invalid command operation", "requires a mutation", "cannot include field edits",
		"canonical command payload must be an object",
	}
	for _, fragment := range validationFragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func projectLocalWailsError(err error, code, message string) error {
	return projectLocalWailsErrorForOperation("project.local", err, code, message)
}

func projectLocalWailsErrorForOperation(operation string, err error, code, message string) error {
	if err == nil {
		return nil
	}
	var typed *cloud.Error
	if errors.As(err, &typed) {
		return projectWailsErrorForOperation(operation, err)
	}
	return projectDiagnosticWailsError(operation, err, &cloud.Error{Code: cloud.ErrorCode(code), Message: message})
}

func projectWailsError(err error) error {
	return projectWailsErrorForOperation("project.operation", err)
}

func projectWailsErrorForOperation(operation string, err error) error {
	if err == nil {
		return nil
	}
	var typed *cloud.Error
	if !errors.As(err, &typed) {
		return projectDiagnosticWailsError(operation, err, &cloud.Error{Code: cloud.ErrorCode("PROJECT_ERROR"), Message: "project operation failed"})
	}
	message, ok := projectWailsSafeMessage(typed.Code)
	if !ok {
		return projectDiagnosticWailsError(operation, err, cloud.AttachRequestID(&cloud.Error{Code: cloud.ErrorCode("PROJECT_ERROR"), Message: "project operation failed"}, typed.RequestID))
	}
	return projectDiagnosticWailsError(operation, err, cloud.AttachRequestID(&cloud.Error{Code: typed.Code, Message: message, Retryable: typed.Retryable}, typed.RequestID))
}

func projectDiagnosticWailsError(operation string, original, safe error) error {
	return &cloudDiagnosticError{err: safe, diagnosticID: recordCloudRequestFailure(operation, original)}
}

func projectWailsSafeMessage(code cloud.ErrorCode) (string, bool) {
	messages := map[cloud.ErrorCode]string{
		cloud.ErrorCode("CLOUD_UNAVAILABLE"):      "cloud features are unavailable",
		cloud.ErrorCode("CAPABILITY_UNAVAILABLE"): "cloud projects are unavailable in this version",
		cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"):   "cloud project response is invalid",
		cloud.ErrorAuthRequired:                   "authentication is required",
		cloud.ErrorProfileScopeMismatch:           "project response belongs to another account scope",
		cloud.ErrorNetworkUnavailable:             "cloud network is unavailable",
		cloud.ErrorCode("VALIDATION_FAILED"):      "project request is invalid",
		cloud.ErrorCode("NOT_FOUND"):              "project was not found",
		cloud.ErrorCode("FORBIDDEN"):              "only the project owner can perform this operation",
		cloud.ErrorCode("CONFLICT"):               "project changed or cannot perform this operation",
		cloud.ErrorCode("IDEMPOTENCY_CONFLICT"):   "project command conflicts with an existing command",
		cloud.ErrorCode("QUOTA_EXCEEDED"):         "project quota is exceeded",
		cloud.ErrorCode("RATE_LIMITED"):           "too many project requests; try again later",
		cloud.ErrorCode("SERVICE_UNAVAILABLE"):    "cloud project service is temporarily unavailable",
		cloud.ErrorCode("INTERNAL_ERROR"):         "cloud project service is temporarily unavailable",
	}
	message, ok := messages[code]
	return message, ok
}
