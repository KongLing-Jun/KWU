package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func decodeV2Payload(payload []byte, target any, fields ...string) error {
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return err
	}
	if len(fields) > 0 {
		var shape map[string]json.RawMessage
		if err := json.Unmarshal(payload, &shape); err != nil {
			return err
		}
		if err := requireExactProjectFields(shape, fields...); err != nil {
			return err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("projects-v2: decode response: %w", err)
	}
	return requireJSONEOF(decoder)
}

func (g *ProjectGateway) CreateAggregate(ctx context.Context, input AggregateCreateInput, key string) (AggregateCreateResult, error) {
	if err := validateProjectName(input.Name); err != nil {
		return AggregateCreateResult{}, err
	}
	if err := validateIdempotencyKey(key); err != nil {
		return AggregateCreateResult{}, err
	}
	body := map[string]any{"name": input.Name, "bindings": input.Bindings}
	if input.Description.Set {
		body["description"] = input.Description.Value
	}
	if input.HumanDescription != "" {
		body["human_description"] = input.HumanDescription
	}
	if input.AgentInstruction != "" {
		body["agent_instruction"] = input.AgentInstruction
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return AggregateCreateResult{}, err
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {key}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, projectsPath+"/aggregates", body, token, headers, &payload)
	})
	if err != nil {
		return AggregateCreateResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return AggregateCreateResult{}, err
	}
	var out AggregateCreateResult
	if err := decodeV2Payload(payload, &out, "project_id", "project_version", "instruction_version", "binding_ids", "created_at"); err != nil {
		return out, err
	}
	out.ProjectScopeKey = scope.Key
	return out, nil
}

func (g *ProjectGateway) ListMembers(ctx context.Context, projectID string) (ProjectMemberList, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ProjectMemberList{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectMemberList{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, projectPath(projectID)+"/members", nil, token, nil, &payload)
	})
	if err != nil {
		return ProjectMemberList{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectMemberList{}, err
	}
	var items []ProjectMember
	if err := decodeV2Payload(payload, &items); err != nil {
		return ProjectMemberList{}, err
	}
	items, err = validateProjectMembers(items, projectID, scope.Binding.CloudUserID)
	if err != nil {
		return ProjectMemberList{}, err
	}
	return ProjectMemberList{ProjectScopeKey: scope.Key, ProjectID: projectID, Items: items}, nil
}

// JoinProject activates the current user's membership for an existing project.
// Identity comes exclusively from the authenticated scope; the request body
// deliberately contains no user_id field.
func (g *ProjectGateway) JoinProject(ctx context.Context, projectID string) (ProjectJoinResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ProjectJoinResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectJoinResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, projectsPath+"/join", map[string]any{"project_id": projectID}, token, nil, &payload)
	})
	if err != nil {
		return ProjectJoinResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectJoinResult{}, err
	}
	var member ProjectMember
	if err := decodeV2Payload(payload, &member, "project_id", "user_id", "role", "status", "display_name", "version", "created_at", "updated_at"); err != nil {
		return ProjectJoinResult{}, err
	}
	if err := validateJoinedProjectMember(member, projectID, scope.Binding.CloudUserID); err != nil {
		return ProjectJoinResult{}, err
	}
	return ProjectJoinResult{ProjectScopeKey: scope.Key, Member: member}, nil
}

func validateJoinedProjectMember(member ProjectMember, projectID, currentUserID string) error {
	if member.ProjectID != projectID || member.UserID != currentUserID {
		return fmt.Errorf("projects-v1: joined member identity mismatch")
	}
	if err := validateJoinedProjectMemberState(member); err != nil {
		return err
	}
	if err := validateProjectTimestamps(member.CreatedAt, member.UpdatedAt); err != nil {
		return fmt.Errorf("projects-v1: invalid joined member timestamps: %w", err)
	}
	return nil
}

func validateJoinedProjectMemberState(member ProjectMember) error {
	if member.Role != "member" && member.Role != "owner" {
		return fmt.Errorf("projects-v1: invalid joined member role")
	}
	if member.Status != "active" || member.Version == 0 || member.Version > MaxSafeJSONInteger {
		return fmt.Errorf("projects-v1: invalid joined member state")
	}
	if strings.TrimSpace(member.DisplayName) == "" || !validProjectText(member.DisplayName, 255) {
		return fmt.Errorf("projects-v1: invalid joined member display name")
	}
	return nil
}

func (g *ProjectGateway) ListBindings(ctx context.Context, projectID string) (ProjectBindingList, error) {
	return g.projectBindingsRequest(ctx, http.MethodGet, projectID, nil, "")
}

func (g *ProjectGateway) PutBindings(ctx context.Context, projectID string, expectedVersion uint64, bindings []ProjectBindingInput, key string) (ProjectBindingList, error) {
	if expectedVersion == 0 || expectedVersion > MaxExpectedVersion {
		return ProjectBindingList{}, fmt.Errorf("projects-v2: invalid project version")
	}
	if err := validateIdempotencyKey(key); err != nil {
		return ProjectBindingList{}, err
	}
	return g.projectBindingsRequest(ctx, http.MethodPut, projectID, map[string]any{"expected_version": expectedVersion, "bindings": bindings}, key)
}

func (g *ProjectGateway) PatchBindings(ctx context.Context, projectID string, input PatchProjectBindingsInput, key string) (ProjectBindingList, error) {
	if input.ExpectedVersion == 0 || input.ExpectedVersion > MaxExpectedVersion {
		return ProjectBindingList{}, fmt.Errorf("projects-v2: invalid project version")
	}
	if err := validateIdempotencyKey(key); err != nil {
		return ProjectBindingList{}, err
	}
	body := map[string]any{"expected_version": input.ExpectedVersion}
	for field, update := range map[string]NullableStringUpdate{
		"skill_id": input.SkillID, "expert_id": input.ExpertID, "connector_id": input.ConnectorID,
	} {
		if !update.Set {
			continue
		}
		if update.Value != nil && (strings.TrimSpace(*update.Value) == "" || !validProjectText(*update.Value, 500)) {
			return ProjectBindingList{}, fmt.Errorf("projects-v2: invalid %s", field)
		}
		body[field] = update.Value
	}
	if len(body) == 1 {
		return ProjectBindingList{}, fmt.Errorf("projects-v2: binding patch requires a mutation")
	}
	return g.projectBindingsRequest(ctx, http.MethodPatch, projectID, body, key)
}

func (g *ProjectGateway) projectBindingsRequest(ctx context.Context, method, projectID string, body any, key string) (ProjectBindingList, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ProjectBindingList{}, err
	}
	headers := http.Header{}
	if key != "" {
		headers.Set("X-Idempotency-Key", key)
	}
	scope, payload, err := g.scopedJSONRequest(ctx, method, projectPath(projectID)+"/bindings", body, headers)
	if err != nil {
		return ProjectBindingList{}, err
	}
	var out ProjectBindingList
	if err := decodeV2Payload(payload, &out, "project_id", "project_version", "items"); err != nil {
		return out, err
	}
	if err := validateProjectBindingList(out, projectID); err != nil {
		return ProjectBindingList{}, err
	}
	out.ProjectScopeKey = scope
	return out, nil
}

func (g *ProjectGateway) scopedJSONRequest(ctx context.Context, method, path string, body any, headers http.Header) (string, json.RawMessage, error) {
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", nil, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, method, path, body, token, headers, &payload)
	})
	if err != nil {
		return "", nil, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", nil, err
	}
	return scope.Key, payload, nil
}

func validateProjectBindingList(out ProjectBindingList, projectID string) error {
	if out.ProjectID != projectID || out.ProjectVersion == 0 || out.Items == nil {
		return fmt.Errorf("projects-v2: invalid binding response")
	}
	for _, item := range out.Items {
		if !validProjectBinding(item, projectID) {
			return fmt.Errorf("projects-v2: invalid binding response")
		}
	}
	return nil
}

func validProjectBinding(item ProjectBinding, projectID string) bool {
	validKind := item.Kind == "connector" || item.Kind == "expert" || item.Kind == "skill"
	return item.ProjectID == projectID && item.BindingID != "" && item.Version > 0 && validKind
}

func (g *ProjectGateway) CreateInvite(ctx context.Context, projectID, key string) (ProjectInviteCreateResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ProjectInviteCreateResult{}, err
	}
	if err := validateIdempotencyKey(key); err != nil {
		return ProjectInviteCreateResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectInviteCreateResult{}, err
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {key}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, projectPath(projectID)+"/invites", map[string]any{"expires_in_seconds": 86400}, token, headers, &payload)
	})
	if err != nil {
		return ProjectInviteCreateResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectInviteCreateResult{}, err
	}
	var out ProjectInviteCreateResult
	if err := decodeV2Payload(payload, &out); err != nil {
		return out, err
	}
	if out.InviteID == "" || (out.Token == "" && !out.CommittedWithoutSecret) {
		return out, fmt.Errorf("projects-v2: invalid invite create response")
	}
	out.ProjectScopeKey = scope.Key
	return out, nil
}

func (g *ProjectGateway) ListInvites(ctx context.Context, projectID string) (string, []ProjectInvite, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", nil, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", nil, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, projectPath(projectID)+"/invites", nil, token, nil, &payload)
	})
	if err != nil {
		return "", nil, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", nil, err
	}
	var items []ProjectInvite
	if err := decodeV2Payload(payload, &items); err != nil {
		return "", nil, err
	}
	if items == nil {
		items = []ProjectInvite{}
	}
	return scope.Key, items, nil
}

func (g *ProjectGateway) RevokeInvite(ctx context.Context, projectID, inviteID string, version uint64, key string) (CommandResult, error) {
	return g.mutateV2Command(ctx, http.MethodPost, projectPath(projectID)+"/invites/"+url.PathEscape(inviteID)+"/revoke", map[string]any{"expected_version": version}, key, projectID, "invite", inviteID)
}

func (g *ProjectGateway) AcceptInvite(ctx context.Context, inviteID, token, key string) (ProjectInviteAcceptResult, error) {
	if err := validateOpaqueProjectID(inviteID); err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	if token == "" || strings.TrimSpace(token) != token {
		return ProjectInviteAcceptResult{}, fmt.Errorf("projects-v2: invalid invite token")
	}
	if err := validateIdempotencyKey(key); err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {key}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, authToken string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, "/api/v1/project-invites/"+url.PathEscape(inviteID)+"/accept", map[string]any{"token": token}, authToken, headers, &payload)
	})
	if err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	var out ProjectInviteAcceptResult
	if err := decodeV2Payload(payload, &out); err != nil {
		return out, err
	}
	out.ProjectScopeKey = scope.Key
	return out, nil
}

func (g *ProjectGateway) InviteAcceptanceStatus(ctx context.Context, inviteID, attemptKey string) (ProjectInviteAcceptResult, error) {
	if err := validateOpaqueProjectID(inviteID); err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	if err := validateIdempotencyKey(attemptKey); err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPost, "/api/v1/project-invites/"+url.PathEscape(inviteID)+"/acceptance-status", map[string]any{"attempt_key": attemptKey}, token, nil, &payload)
	})
	if err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ProjectInviteAcceptResult{}, err
	}
	var out ProjectInviteAcceptResult
	if err := decodeV2Payload(payload, &out); err != nil {
		return out, err
	}
	out.ProjectScopeKey = scope.Key
	return out, nil
}

func (g *ProjectGateway) DeactivateMember(ctx context.Context, projectID, userID string, version uint64, key string) (CommandResult, error) {
	return g.mutateV2Command(ctx, http.MethodPost, projectPath(projectID)+"/members/"+url.PathEscape(userID)+"/deactivate", map[string]any{"expected_version": version}, key, projectID, "membership", userID)
}

func (g *ProjectGateway) DeleteProject(ctx context.Context, projectID string, version uint64, key string) (CommandResult, error) {
	return g.mutateV2Command(ctx, http.MethodDelete, projectPath(projectID), map[string]any{"expected_version": version}, key, projectID, "project", projectID)
}

func (g *ProjectGateway) ListActivities(ctx context.Context, projectID string, input ListActivitiesInput) (ActivityPage, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return ActivityPage{}, err
	}
	query, err := activityListQuery(input)
	if err != nil {
		return ActivityPage{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return ActivityPage{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, projectPath(projectID)+"/activities", query, nil, token, nil, &payload)
	})
	if err != nil {
		return ActivityPage{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return ActivityPage{}, err
	}
	var page ActivityPage
	if err := decodeV2Payload(payload, &page, "items", "next_cursor", "read_cursor"); err != nil {
		return page, err
	}
	if page.Items == nil {
		page.Items = []ProjectActivity{}
	}
	page.scopeKey, page.projectID = scope.Key, projectID
	return page, nil
}

func activityListQuery(input ListActivitiesInput) (url.Values, error) {
	if input.PageSize == 0 {
		input.PageSize = 20
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		return nil, fmt.Errorf("projects-v2: invalid activity page size")
	}
	if input.Filter == "" {
		input.Filter = ActivityAllMembers
	}
	query := url.Values{"page_size": {strconv.Itoa(input.PageSize)}, "filter": {string(input.Filter)}}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	if len(input.MemberUserIDs) > 0 {
		query.Set("member_user_ids", strings.Join(input.MemberUserIDs, ","))
	}
	return query, nil
}

func (g *ProjectGateway) MarkActivityRead(ctx context.Context, projectID, throughCursor string) (string, ActivityReadResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", ActivityReadResult{}, err
	}
	if throughCursor == "" || len([]byte(throughCursor)) > 4096 {
		return "", ActivityReadResult{}, fmt.Errorf("projects-v2: invalid activity read cursor")
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ActivityReadResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodPut, projectPath(projectID)+"/activity-read", map[string]any{"through_cursor": throughCursor}, token, nil, &payload)
	})
	if err != nil {
		return "", ActivityReadResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ActivityReadResult{}, err
	}
	var out ActivityReadResult
	if err := decodeV2Payload(payload, &out, "through_activity_id", "updated_at"); err != nil {
		return "", out, err
	}
	return scope.Key, out, nil
}

func (g *ProjectGateway) ListLabels(ctx context.Context, projectID string) (string, []ProjectLabel, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", nil, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", nil, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, projectPath(projectID)+"/labels", nil, token, nil, &payload)
	})
	if err != nil {
		return "", nil, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", nil, err
	}
	var labels []projectLabelWire
	if err := decodeV2Payload(payload, &labels); err != nil {
		return "", nil, err
	}
	result, err := projectLabelsFromWire(labels, projectID)
	if err != nil {
		return "", nil, err
	}
	return scope.Key, result, nil
}

func (g *ProjectGateway) CreateLabel(ctx context.Context, projectID, name, color, key string) (CommandResult, error) {
	if strings.TrimSpace(name) == "" {
		return CommandResult{}, fmt.Errorf("projects-v2: invalid label name")
	}
	return g.mutateV2Command(ctx, http.MethodPost, projectPath(projectID)+"/labels", map[string]any{"name": name, "color": color}, key, projectID, "label", "")
}

func (g *ProjectGateway) PatchLabel(ctx context.Context, projectID, labelID string, expectedVersion uint64, name, color *string, key string) (CommandResult, error) {
	if err := validateOpaqueProjectID(labelID); err != nil {
		return CommandResult{}, err
	}
	if expectedVersion == 0 || (name == nil && color == nil) {
		return CommandResult{}, fmt.Errorf("projects-v2: invalid label patch")
	}
	body := map[string]any{"expected_version": expectedVersion}
	if name != nil {
		if strings.TrimSpace(*name) == "" {
			return CommandResult{}, fmt.Errorf("projects-v2: invalid label name")
		}
		body["name"] = *name
	}
	if color != nil {
		body["color"] = *color
	}
	return g.mutateV2Command(ctx, http.MethodPatch, projectPath(projectID)+"/labels/"+url.PathEscape(labelID), body, key, projectID, "label", labelID)
}

func (g *ProjectGateway) DeleteLabel(ctx context.Context, projectID, labelID string, expectedVersion uint64, key string) (CommandResult, error) {
	if err := validateOpaqueProjectID(labelID); err != nil {
		return CommandResult{}, err
	}
	if expectedVersion == 0 {
		return CommandResult{}, fmt.Errorf("projects-v2: invalid label version")
	}
	return g.mutateV2Command(ctx, http.MethodDelete, projectPath(projectID)+"/labels/"+url.PathEscape(labelID), map[string]any{"expected_version": expectedVersion}, key, projectID, "label", labelID)
}

func (g *ProjectGateway) MutateWorkItemLabel(ctx context.Context, projectID, workItemID, labelID, key string, expectedVersion uint64, put bool) (WorkItemResult, error) {
	if err := validateWorkItemMutation(projectID, workItemID, expectedVersion, key); err != nil {
		return WorkItemResult{}, err
	}
	if err := validateOpaqueProjectID(labelID); err != nil {
		return WorkItemResult{}, err
	}
	method := http.MethodDelete
	if put {
		method = http.MethodPut
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return WorkItemResult{}, err
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {key}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		path := projectPath(projectID) + "/work-items/" + url.PathEscape(workItemID) + "/labels/" + url.PathEscape(labelID)
		return g.transport.DoAuthenticatedJSON(callCtx, method, path, map[string]any{"expected_version": expectedVersion}, token, headers, &payload)
	})
	if err != nil {
		return WorkItemResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return WorkItemResult{}, err
	}
	item, err := DecodeWorkItem(payload, scope, projectID, workItemID)
	if err != nil {
		return WorkItemResult{}, err
	}
	return WorkItemResult{ProjectScopeKey: scope.Key, WorkItem: item}, nil
}

func (g *ProjectGateway) PostMessage(ctx context.Context, projectID, key string, input PostMessageInput) (CommandResult, error) {
	if strings.TrimSpace(input.ClientMessageID) == "" || !validProjectText(input.ClientMessageID, 200) {
		return CommandResult{}, fmt.Errorf("projects-v2: invalid client message id")
	}
	if strings.TrimSpace(input.Text) == "" || !validProjectText(input.Text, 20_480) {
		return CommandResult{}, fmt.Errorf("projects-v2: invalid message text")
	}
	mentionUserIDs := input.MentionUserIDs
	if mentionUserIDs == nil {
		mentionUserIDs = []string{}
	}
	assetIDs := input.AssetIDs
	if assetIDs == nil {
		assetIDs = []string{}
	}
	body := map[string]any{"client_message_id": input.ClientMessageID, "text": input.Text, "mention_user_ids": mentionUserIDs, "asset_ids": assetIDs}
	return g.mutateV2Command(ctx, http.MethodPost, projectPath(projectID)+"/messages", body, key, projectID, "activity", "")
}

func (g *ProjectGateway) mutateV2Command(ctx context.Context, method, path string, body any, key, projectID, kind, resourceID string) (CommandResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return CommandResult{}, err
	}
	if err := validateIdempotencyKey(key); err != nil {
		return CommandResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return CommandResult{}, err
	}
	var payload json.RawMessage
	headers := http.Header{"X-Idempotency-Key": {key}}
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, method, path, body, token, headers, &payload)
	})
	if err != nil {
		return CommandResult{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return CommandResult{}, err
	}
	result, err := decodeCommandResult(payload, scope, projectID, kind)
	if err != nil {
		return result, err
	}
	if resourceID != "" && result.ResourceID != resourceID {
		return CommandResult{}, fmt.Errorf("projects-v2: command resource mismatch")
	}
	return result, nil
}
