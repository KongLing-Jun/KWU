package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"kwu/internal/cloud"
)

const projectsPath = "/api/v1/projects"

// ProjectScopeResolver supplies the currently authenticated project scope.
// Gateway operations snapshot it before I/O and revalidate it afterward.
type ProjectScopeResolver interface {
	CurrentProjectScope(context.Context) (Scope, error)
}

// ProjectJSONTransport is the existing Cloud JSON transport surface used after
// AuthenticatedRequestExecutor supplies a current access token.
type ProjectJSONTransport interface {
	DoAuthenticatedJSON(context.Context, string, string, any, string, http.Header, any) error
	DoAuthenticatedJSONQuery(context.Context, string, string, url.Values, any, string, http.Header, any) error
}

// ProjectGateway performs authenticated Project HTTP operations without owning
// account state or serializing unrelated reads behind a global mutex.
type ProjectGateway struct {
	executor  cloud.AuthenticatedRequestExecutor
	transport ProjectJSONTransport
	scopes    ProjectScopeResolver
}

// ListProjectsInput selects one server-ordered Project page. Zero values use
// the contract defaults of page size 20 and active status.
type ListProjectsInput struct {
	PageSize int
	Cursor   string
	Status   ProjectStatus
	Search   string
}

// CreateProjectInput is the mutable Project state accepted by create.
type CreateProjectInput struct {
	Name             string
	Description      NullableStringUpdate
	HumanDescription *string
	AgentInstruction *string
	SkillID          *string
	ExpertID         *string
	ConnectorID      *string
}

// NullableStringUpdate distinguishes an omitted field from an explicit null.
type NullableStringUpdate struct {
	Set   bool
	Value *string
}

// PatchProjectInput applies one optimistic Project mutation.
type PatchProjectInput struct {
	ExpectedVersion uint64
	Name            *string
	Description     NullableStringUpdate
	Status          *ProjectStatus
}

// ProjectResult stamps a validated Project with the scope that produced it.
type ProjectResult struct {
	ProjectScopeKey string
	Project         Project
}

type ProjectOnboardingState string

const (
	ProjectOnboardingStateActive    ProjectOnboardingState = "active"
	ProjectOnboardingStateCompleted ProjectOnboardingState = "completed"
	ProjectOnboardingStateDismissed ProjectOnboardingState = "dismissed"
	ProjectOnboardingStateDeleted   ProjectOnboardingState = "deleted"
)

// ProjectOnboardingClaim is the server-owned, account-scoped guide state.
// Project is present only while the guide remains active.
type ProjectOnboardingClaim struct {
	ProjectScopeKey string
	Project         *Project
	GuideVersion    uint64
	State           ProjectOnboardingState
	Created         bool
}

// CommandResult is the stable mutation receipt returned by projects-v1.
type CommandResult struct {
	ProjectScopeKey  string
	OperationID      string
	ResourceKind     string
	ResourceID       string
	ProjectID        string
	CommittedVersion uint64
}

// NewProjectGateway binds the existing authenticated executor, JSON transport
// and current-scope resolver to the Project API.
func NewProjectGateway(executor cloud.AuthenticatedRequestExecutor, transport ProjectJSONTransport, scopes ProjectScopeResolver) (*ProjectGateway, error) {
	if nilGatewayDependency(executor) || nilGatewayDependency(transport) || nilGatewayDependency(scopes) {
		return nil, fmt.Errorf("projects-v1: gateway dependencies are not configured")
	}
	return &ProjectGateway{executor: executor, transport: transport, scopes: scopes}, nil
}

// ListProjects fetches and strictly validates one page for the scope captured
// before the request. A late response is rejected if the current scope changed.
func (g *ProjectGateway) ListProjects(ctx context.Context, input ListProjectsInput) (ProjectPage, error) {
	return g.listProjectsAt(ctx, projectsPath, input, "", false)
}

// ListOwnedProjects returns only projects whose owner is the current Cloud
// user. Ownership remains an internal route decision so it cannot leak into
// the joined-project endpoint.
func (g *ProjectGateway) ListOwnedProjects(ctx context.Context, input ListProjectsInput) (ProjectPage, error) {
	return g.listProjectsAt(ctx, projectsPath, input, "owned", false)
}

// ListJoinedProjects returns projects where the current user has an active
// non-owner member relationship. Cloud may represent an owner as a project
// member as well, so owner-created projects are removed using the validated
// owner_user_id before the page reaches Desktop.
func (g *ProjectGateway) ListJoinedProjects(ctx context.Context, input ListProjectsInput) (ProjectPage, error) {
	return g.listProjectsAt(ctx, projectsPath+"/joined", input, "", true)
}

func (g *ProjectGateway) listProjectsAt(ctx context.Context, path string, input ListProjectsInput, ownership string, excludeCurrentOwner bool) (ProjectPage, error) {
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectPage{}, err
	}
	query, err := projectListQuery(input)
	if err != nil {
		return ProjectPage{}, err
	}
	if ownership != "" {
		query.Set("ownership", ownership)
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, path, query, nil, token, nil, &payload)
	})
	if err != nil {
		return ProjectPage{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectPage{}, err
	}
	page, err := DecodeProjectPage(payload, scope)
	if err != nil {
		return ProjectPage{}, err
	}
	if excludeCurrentOwner {
		items := make([]Project, 0, len(page.Items))
		for _, item := range page.Items {
			if item.OwnerUserID != scope.Binding.CloudUserID {
				items = append(items, item)
			}
		}
		page.Items = items
	}
	return page, nil
}

// ClaimOnboardingProject asks Cloud to return the account's single real guide
// project. The empty request body prevents React from choosing project identity
// or guide content.
func (g *ProjectGateway) ClaimOnboardingProject(ctx context.Context) (ProjectOnboardingClaim, error) {
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectOnboardingClaim{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, projectsPath+"/onboarding/claim", map[string]any{}, token, nil, &payload)
	})
	if err != nil {
		return ProjectOnboardingClaim{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectOnboardingClaim{}, err
	}
	return DecodeProjectOnboardingClaim(payload, scope)
}

// CreateProject creates a Project with a caller-owned durable idempotency key.
func (g *ProjectGateway) CreateProject(ctx context.Context, input CreateProjectInput, idempotencyKey string) (CommandResult, error) {
	if err := validateCreateProjectInput(input); err != nil {
		return CommandResult{}, err
	}
	if err := validateIdempotencyKey(idempotencyKey); err != nil {
		return CommandResult{}, err
	}
	body := map[string]any{"name": input.Name}
	if input.Description.Set {
		body["description"] = input.Description.Value
	}
	if input.HumanDescription != nil {
		body["human_description"] = *input.HumanDescription
	}
	if input.AgentInstruction != nil {
		body["agent_instruction"] = *input.AgentInstruction
	}
	if input.SkillID != nil {
		body["skill_id"] = *input.SkillID
	}
	if input.ExpertID != nil {
		body["expert_id"] = *input.ExpertID
	}
	if input.ConnectorID != nil {
		body["connector_id"] = *input.ConnectorID
	}
	return g.mutateProject(ctx, http.MethodPost, projectsPath, body, idempotencyKey, "")
}

func validateCreateProjectInput(input CreateProjectInput) error {
	if err := validateProjectName(input.Name); err != nil {
		return err
	}
	if input.Description.Set {
		if err := validateProjectDescription(input.Description.Value); err != nil {
			return err
		}
	}
	if input.HumanDescription != nil && !validProjectText(*input.HumanDescription, 12_000) {
		return fmt.Errorf("projects-v1: invalid initial human description")
	}
	if input.AgentInstruction != nil && !validProjectText(*input.AgentInstruction, 48_000) {
		return fmt.Errorf("projects-v1: invalid initial agent instruction")
	}
	return validateCreateProjectBindingIDs(input)
}

func validateCreateProjectBindingIDs(input CreateProjectInput) error {
	bindings := map[string]*string{"skill": input.SkillID, "expert": input.ExpertID, "connector": input.ConnectorID}
	for name, value := range bindings {
		if value != nil && (strings.TrimSpace(*value) == "" || !validProjectText(*value, 500)) {
			return fmt.Errorf("projects-v1: invalid initial %s binding", name)
		}
	}
	return nil
}

// GetProject fetches and strictly validates one Project for the current scope.
func (g *ProjectGateway) GetProject(ctx context.Context, projectID string) (ProjectResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ProjectResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, projectPath(projectID), nil, token, nil, &payload)
	})
	if err != nil {
		return ProjectResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectResult{}, err
	}
	project, err := DecodeProject(payload, scope)
	if err != nil {
		return ProjectResult{}, err
	}
	if project.ProjectID != projectID {
		return ProjectResult{}, fmt.Errorf("projects-v1: project response ID does not match request")
	}
	return ProjectResult{ProjectScopeKey: scope.Key, Project: project}, nil
}

// GetProjectInstruction always performs an authenticated online read. The
// Instruction body is never stored in the ordinary Project cache.
func (g *ProjectGateway) GetProjectInstruction(ctx context.Context, projectID string) (InstructionResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return InstructionResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return InstructionResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, instructionPath(projectID), nil, token, nil, &payload)
	})
	if err != nil {
		return InstructionResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return InstructionResult{}, err
	}
	instruction, err := DecodeProjectInstruction(payload, scope, projectID)
	if err != nil {
		return InstructionResult{}, err
	}
	return InstructionResult{ProjectScopeKey: scope.Key, Instruction: instruction}, nil
}

// PutProjectInstruction updates both required strings using the Instruction's
// independent expected version and a caller-owned durable idempotency key.
func (g *ProjectGateway) PutProjectInstruction(ctx context.Context, projectID string, input PutProjectInstructionInput, idempotencyKey string) (CommandResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return CommandResult{}, err
	}
	if err := ValidatePutProjectInstructionInput(input); err != nil {
		return CommandResult{}, err
	}
	if err := validateIdempotencyKey(idempotencyKey); err != nil {
		return CommandResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	body := map[string]any{
		"expected_version": input.ExpectedVersion, "human_description": input.HumanDescription,
		"agent_instruction": input.AgentInstruction,
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {idempotencyKey}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPut, instructionPath(projectID), body, token, headers, &payload)
	})
	if err != nil {
		return CommandResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return CommandResult{}, err
	}
	return DecodeInstructionCommandResult(payload, scope, projectID)
}

func ValidatePutProjectInstructionInput(input PutProjectInstructionInput) error {
	if input.ExpectedVersion == 0 || input.ExpectedVersion > MaxExpectedVersion {
		return fmt.Errorf("projects-v1: instruction expected_version must be between 1 and %d", MaxExpectedVersion)
	}
	if !validProjectText(input.HumanDescription, 12_000) || !validProjectText(input.AgentInstruction, 48_000) {
		return fmt.Errorf("projects-v1: invalid instruction text")
	}
	return nil
}

// PatchProject updates one Project using optimistic concurrency and a durable
// idempotency key.
func (g *ProjectGateway) PatchProject(ctx context.Context, projectID string, input PatchProjectInput, idempotencyKey string) (CommandResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return CommandResult{}, err
	}
	if err := validatePatchProjectInput(input, idempotencyKey); err != nil {
		return CommandResult{}, err
	}
	return g.mutateProject(ctx, http.MethodPatch, projectPath(projectID), projectPatchBody(input), idempotencyKey, projectID)
}

func validatePatchProjectInput(input PatchProjectInput, idempotencyKey string) error {
	if input.ExpectedVersion == 0 || input.ExpectedVersion > MaxExpectedVersion {
		return fmt.Errorf("projects-v1: expected_version must be between 1 and %d", MaxExpectedVersion)
	}
	if err := validateIdempotencyKey(idempotencyKey); err != nil {
		return err
	}
	if input.Name == nil && !input.Description.Set && input.Status == nil {
		return fmt.Errorf("projects-v1: patch requires a mutation")
	}
	if err := validatePatchProjectFields(input); err != nil {
		return err
	}
	return validatePatchProjectStatus(input)
}

func validatePatchProjectFields(input PatchProjectInput) error {
	if input.Name != nil {
		if err := validateProjectName(*input.Name); err != nil {
			return err
		}
	}
	if input.Description.Set {
		if err := validateProjectDescription(input.Description.Value); err != nil {
			return err
		}
	}
	return nil
}

func validatePatchProjectStatus(input PatchProjectInput) error {
	if input.Status != nil {
		if *input.Status != ProjectActive && *input.Status != ProjectArchived {
			return fmt.Errorf("projects-v1: invalid project status")
		}
		if input.Name != nil || input.Description.Set {
			return fmt.Errorf("projects-v1: status transition cannot include field edits")
		}
	}
	return nil
}

func projectPatchBody(input PatchProjectInput) map[string]any {
	body := map[string]any{"expected_version": input.ExpectedVersion}
	if input.Name != nil {
		body["name"] = *input.Name
	}
	if input.Description.Set {
		body["description"] = input.Description.Value
	}
	if input.Status != nil {
		body["status"] = *input.Status
	}
	return body
}

func (g *ProjectGateway) mutateProject(ctx context.Context, method, path string, body any, idempotencyKey, expectedProjectID string) (CommandResult, error) {
	scope, err := g.currentScope(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {idempotencyKey}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, method, path, body, token, headers, &payload)
	})
	if err != nil {
		return CommandResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return CommandResult{}, err
	}
	return DecodeCommandResult(payload, scope, expectedProjectID)
}

func (g *ProjectGateway) currentScope(ctx context.Context) (Scope, error) {
	scope, err := g.scopes.CurrentProjectScope(ctx)
	if err != nil {
		return Scope{}, err
	}
	if err := scope.Validate(); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

func (g *ProjectGateway) revalidateScope(ctx context.Context, expected Scope) error {
	current, err := g.currentScope(ctx)
	if err != nil {
		return err
	}
	if current.Key != expected.Key {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "project response belongs to a stale binding scope"}
	}
	return nil
}

func projectListQuery(input ListProjectsInput) (url.Values, error) {
	pageSize, err := projectListPageSize(input.PageSize)
	if err != nil {
		return nil, err
	}
	status, err := projectListStatus(input.Status)
	if err != nil {
		return nil, err
	}
	if err := validateProjectListCursor(input.Cursor); err != nil {
		return nil, err
	}
	if !validProjectText(input.Search, 200) {
		return nil, fmt.Errorf("projects-v1: project search exceeds 200 bytes")
	}
	query := url.Values{"page_size": {strconv.Itoa(pageSize)}, "status": {string(status)}}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	if input.Search != "" {
		query.Set("search", input.Search)
	}
	return query, nil
}

func projectListPageSize(value int) (int, error) {
	if value == 0 {
		value = 20
	}
	if value < 1 || value > 100 {
		return 0, fmt.Errorf("projects-v1: page_size must be between 1 and 100")
	}
	return value, nil
}

func projectListStatus(value ProjectStatus) (ProjectStatus, error) {
	if value == "" {
		value = ProjectActive
	}
	if value != ProjectActive && value != ProjectArchived && value != ProjectStatus("all") {
		return "", fmt.Errorf("projects-v1: invalid project list status")
	}
	return value, nil
}

func validateProjectListCursor(value string) error {
	if value != "" && (len([]byte(value)) > 2048 || strings.TrimSpace(value) != value) {
		return fmt.Errorf("projects-v1: invalid project cursor")
	}
	return nil
}

func projectPath(projectID string) string {
	return projectsPath + "/" + url.PathEscape(projectID)
}

func instructionPath(projectID string) string {
	return projectPath(projectID) + "/instructions"
}

func validateProjectName(value string) error {
	if strings.TrimSpace(value) == "" || !validProjectText(value, 200) || utf8.RuneCountInString(value) > 15 {
		return fmt.Errorf("projects-v1: invalid project name")
	}
	return nil
}

func validateProjectDescription(value *string) error {
	if value != nil && !validProjectText(*value, 4000) {
		return fmt.Errorf("projects-v1: project description exceeds 4000 bytes")
	}
	return nil
}

func validateIdempotencyKey(value string) error {
	if value == "" || strings.TrimSpace(value) != value || !validProjectText(value, 200) {
		return fmt.Errorf("projects-v1: invalid idempotency key")
	}
	return nil
}

func validProjectText(value string, maximumBytes int) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00') && len([]byte(value)) <= maximumBytes
}

func nilGatewayDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
