package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
)

type CloudProjectMemberView struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	Version     uint64 `json:"version"`
}
type CloudProjectMemberListView struct {
	ProjectScopeKey string                   `json:"projectScopeKey"`
	ProjectID       string                   `json:"projectId"`
	CurrentRole     string                   `json:"currentRole"`
	Items           []CloudProjectMemberView `json:"items"`
}
type CloudProjectInviteView struct {
	InviteID  string `json:"inviteId"`
	Status    string `json:"status"`
	Role      string `json:"role"`
	Version   uint64 `json:"version"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}
type CloudProjectInviteListView struct {
	ProjectScopeKey string                   `json:"projectScopeKey"`
	ProjectID       string                   `json:"projectId"`
	Items           []CloudProjectInviteView `json:"items"`
}
type CloudProjectInviteCreateInput struct {
	ProjectID string `json:"projectId"`
}
type CloudProjectInviteCreateView struct {
	ProjectScopeKey        string `json:"projectScopeKey"`
	ProjectID              string `json:"projectId"`
	InviteID               string `json:"inviteId"`
	Token                  string `json:"token,omitempty"`
	ExpiresAt              string `json:"expiresAt"`
	CommittedWithoutSecret bool   `json:"committedWithoutSecret"`
}
type CloudProjectInviteMutationInput struct {
	ProjectID       string `json:"projectId"`
	InviteID        string `json:"inviteId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type CloudProjectMemberMutationInput struct {
	ProjectID       string `json:"projectId"`
	UserID          string `json:"userId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type JoinCloudProjectInput struct {
	ProjectID string `json:"projectId"`
}
type JoinCloudProjectView struct {
	ProjectScopeKey string `json:"projectScopeKey"`
	ProjectID       string `json:"projectId"`
	UserID          string `json:"userId"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	DisplayName     string `json:"displayName"`
	Version         uint64 `json:"version"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	WorkspacePath   string `json:"workspacePath"`
}
type AcceptCloudProjectInviteInput struct {
	InviteID string `json:"inviteId"`
	Token    string `json:"token"`
}
type AcceptCloudProjectInviteView struct {
	ProjectScopeKey   string `json:"projectScopeKey"`
	Status            string `json:"status"`
	ProjectID         string `json:"projectId,omitempty"`
	MembershipVersion uint64 `json:"membershipVersion,omitempty"`
	WorkspacePath     string `json:"workspacePath,omitempty"`
}
type DeleteCloudProjectInput struct {
	ProjectID       string `json:"projectId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type CloudProjectActivityView struct {
	ActivityID             string                                    `json:"activityId"`
	Kind                   string                                    `json:"kind"`
	ActorUserID            projectNullableStringView                 `json:"actorUserId" ts_type:"string | null"`
	AuthorDisplayName      string                                    `json:"authorDisplayName"`
	ClientMessageID        string                                    `json:"clientMessageId,omitempty"`
	MessageText            string                                    `json:"messageText,omitempty"`
	EventName              string                                    `json:"eventName,omitempty"`
	EventPayload           map[string]any                            `json:"eventPayload,omitempty"`
	MentionUserIDs         []string                                  `json:"mentionUserIds"`
	Assets                 []CloudProjectActivityAssetView           `json:"assets"`
	Unread                 bool                                      `json:"unread"`
	OtherMembersUnread     *bool                                     `json:"otherMembersUnread,omitempty"`
	UnreadOtherMemberCount *int64                                    `json:"unreadOtherMemberCount,omitempty"`
	OtherMemberReadStates  *[]CloudProjectMessageMemberReadStateView `json:"otherMemberReadStates,omitempty"`
	CreatedAt              string                                    `json:"createdAt"`
}
type CloudProjectMessageMemberReadStateView struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
}
type CloudProjectActivityAssetView struct {
	AssetID     string `json:"assetId"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}
type CloudProjectActivityListInput struct {
	ProjectID     string   `json:"projectId"`
	PageSize      int      `json:"pageSize,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
	Filter        string   `json:"filter"`
	MemberUserIDs []string `json:"memberUserIds,omitempty"`
}
type CloudProjectMessageListInput struct {
	ProjectID string `json:"projectId"`
	PageSize  int    `json:"pageSize,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
}
type CloudProjectActivityPageView struct {
	ProjectScopeKey string                     `json:"projectScopeKey"`
	ProjectID       string                     `json:"projectId"`
	Items           []CloudProjectActivityView `json:"items"`
	NextCursor      projectNullableStringView  `json:"nextCursor" ts_type:"string | null"`
	ReadCursor      projectNullableStringView  `json:"readCursor" ts_type:"string | null"`
}
type PostCloudProjectMessageInput struct {
	ProjectID      string   `json:"projectId"`
	Text           string   `json:"text"`
	MentionUserIDs []string `json:"mentionUserIds"`
	AssetIDs       []string `json:"assetIds"`
}
type MarkCloudProjectActivityReadInput struct {
	ProjectID     string `json:"projectId"`
	ThroughCursor string `json:"throughCursor"`
}
type MarkCloudProjectMessageReadInput struct {
	ProjectID     string `json:"projectId"`
	ThroughCursor string `json:"throughCursor"`
}

func projectRandomKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (a *App) projectServicesForV2(feature func(project.CapabilityFeatures) bool, name string) (*cloudProjectServices, error) {
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return nil, err
	}
	if !feature(features) {
		return nil, cloudCapabilityUnavailable(name)
	}
	return services, nil
}

func (a *App) ListCloudProjectMembers(projectID string) (CloudProjectMemberListView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return CloudProjectMemberListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	list, err := services.Gateway.ListMembers(ctx, projectID)
	if err != nil {
		return CloudProjectMemberListView{}, projectProtocolWailsError(err)
	}
	items := make([]CloudProjectMemberView, len(list.Items))
	for i, m := range list.Items {
		items[i] = CloudProjectMemberView{UserID: m.UserID, DisplayName: m.DisplayName, Role: m.Role, Status: m.Status, Version: m.Version}
	}
	currentRole := ""
	for _, member := range list.Items {
		if member.UserID == services.Scope.Binding.CloudUserID {
			currentRole = member.Role
			break
		}
	}
	return CloudProjectMemberListView{ProjectScopeKey: list.ProjectScopeKey, ProjectID: projectID, CurrentRole: currentRole, Items: items}, nil
}

func (a *App) JoinCloudProject(input JoinCloudProjectInput) (JoinCloudProjectView, error) {
	services, err := a.projectServicesForOperation()
	if err != nil {
		return JoinCloudProjectView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	result, err := services.Gateway.JoinProject(ctx, input.ProjectID)
	if err != nil {
		return JoinCloudProjectView{}, projectProtocolWailsError(err)
	}
	member := result.Member
	workspacePath, err := ensureCloudProjectWorkspace(ctx, services, member.ProjectID)
	if err != nil {
		return JoinCloudProjectView{}, projectLocalWailsError(err, "PROJECT_WORKSPACE_UNAVAILABLE", "dedicated project workspace is unavailable")
	}
	return JoinCloudProjectView{
		ProjectScopeKey: result.ProjectScopeKey,
		ProjectID:       member.ProjectID,
		UserID:          member.UserID,
		Role:            member.Role,
		Status:          member.Status,
		DisplayName:     member.DisplayName,
		Version:         member.Version,
		CreatedAt:       member.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:       member.UpdatedAt.UTC().Format(time.RFC3339Nano),
		WorkspacePath:   workspacePath,
	}, nil
}

func (a *App) CreateCloudProjectInvite(input CloudProjectInviteCreateInput) (CloudProjectInviteCreateView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return CloudProjectInviteCreateView{}, projectWailsError(err)
	}
	key, err := projectRandomKey()
	if err != nil {
		return CloudProjectInviteCreateView{}, err
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	result, err := services.Gateway.CreateInvite(ctx, input.ProjectID, key)
	if err != nil {
		return CloudProjectInviteCreateView{}, projectProtocolWailsError(err)
	}
	return CloudProjectInviteCreateView{ProjectScopeKey: result.ProjectScopeKey, ProjectID: input.ProjectID, InviteID: result.InviteID, Token: result.Token, ExpiresAt: result.ExpiresAt.UTC().Format(time.RFC3339Nano), CommittedWithoutSecret: result.CommittedWithoutSecret}, nil
}
func (a *App) ListCloudProjectInvites(projectID string) (CloudProjectInviteListView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return CloudProjectInviteListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scope, rows, err := services.Gateway.ListInvites(ctx, projectID)
	if err != nil {
		return CloudProjectInviteListView{}, projectProtocolWailsError(err)
	}
	items := make([]CloudProjectInviteView, len(rows))
	for i, row := range rows {
		items[i] = CloudProjectInviteView{InviteID: row.InviteID, Status: row.Status, Role: row.Role, Version: row.Version, ExpiresAt: row.ExpiresAt.UTC().Format(time.RFC3339Nano), CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano)}
	}
	return CloudProjectInviteListView{ProjectScopeKey: scope, ProjectID: projectID, Items: items}, nil
}
func (a *App) RevokeCloudProjectInvite(input CloudProjectInviteMutationInput) error {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return projectWailsError(err)
	}
	key, err := projectRandomKey()
	if err != nil {
		return err
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	_, err = services.Gateway.RevokeInvite(ctx, input.ProjectID, input.InviteID, input.ExpectedVersion, key)
	return projectProtocolWailsError(err)
}
func (a *App) DeactivateCloudProjectMember(input CloudProjectMemberMutationInput) error {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return projectWailsError(err)
	}
	key, err := projectRandomKey()
	if err != nil {
		return err
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	_, err = services.Gateway.DeactivateMember(ctx, input.ProjectID, input.UserID, input.ExpectedVersion, key)
	return projectProtocolWailsError(err)
}
func (a *App) AcceptCloudProjectInvite(input AcceptCloudProjectInviteInput) (AcceptCloudProjectInviteView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return AcceptCloudProjectInviteView{}, projectWailsError(err)
	}
	key, err := projectRandomKey()
	if err != nil {
		return AcceptCloudProjectInviteView{}, err
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	result, err := services.Gateway.AcceptInvite(ctx, input.InviteID, input.Token, key)
	if err != nil {
		status, statusErr := services.Gateway.InviteAcceptanceStatus(ctx, input.InviteID, key)
		if statusErr == nil && status.Status != "not_received" {
			result = status
			err = nil
		}
	}
	if err != nil {
		return AcceptCloudProjectInviteView{}, projectProtocolWailsError(err)
	}
	workspacePath := ""
	if result.ProjectID != "" {
		workspacePath, err = ensureCloudProjectWorkspace(ctx, services, result.ProjectID)
		if err != nil {
			return AcceptCloudProjectInviteView{}, projectLocalWailsError(err, "PROJECT_WORKSPACE_UNAVAILABLE", "dedicated project workspace is unavailable")
		}
	}
	return AcceptCloudProjectInviteView{ProjectScopeKey: result.ProjectScopeKey, Status: result.Status, ProjectID: result.ProjectID, MembershipVersion: result.MembershipVersion, WorkspacePath: workspacePath}, nil
}
func (a *App) DeleteCloudProject(input DeleteCloudProjectInput) error {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Members }, "project members")
	if err != nil {
		return projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	detail, err := services.Gateway.GetProject(ctx, input.ProjectID)
	if err != nil {
		return projectProtocolWailsError(err)
	}
	if detail.Project.OwnerUserID != services.Scope.Binding.CloudUserID {
		return projectWailsError(&cloud.Error{Code: cloud.ErrorCode("FORBIDDEN"), Message: "only the project owner can delete the project"})
	}
	key, err := projectRandomKey()
	if err != nil {
		return err
	}
	_, err = services.Gateway.DeleteProject(ctx, input.ProjectID, input.ExpectedVersion, key)
	if err != nil {
		return projectProtocolWailsError(err)
	}
	a.evictDeletedCloudProjectFromWorkspace(ctx, services, input.ProjectID)
	return nil
}

func (a *App) evictDeletedCloudProjectFromWorkspace(ctx context.Context, services *cloudProjectServices, projectID string) bool {
	if err := services.Cache.RemoveProject(ctx, services.Scope, projectID); err != nil {
		slog.Warn("cloud project was deleted but its workspace cache could not be updated", "project_id", projectID, "err", err)
		return false
	}
	a.emitProjectTreeChanged()
	return true
}

func (a *App) ListCloudProjectActivities(input CloudProjectActivityListInput) (CloudProjectActivityPageView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Activity }, "project activity")
	if err != nil {
		return CloudProjectActivityPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if err := a.recoverCloudProjectActivity(ctx, services, input.ProjectID); err != nil {
		recordDesktopBackgroundFailure("project.activity.recover", err)
	}
	page, err := services.Gateway.ListActivities(ctx, input.ProjectID, project.ListActivitiesInput{PageSize: input.PageSize, Cursor: input.Cursor, Filter: project.ActivityFilter(input.Filter), MemberUserIDs: input.MemberUserIDs})
	if err != nil {
		return CloudProjectActivityPageView{}, projectProtocolWailsError(err)
	}
	items := make([]CloudProjectActivityView, len(page.Items))
	for i, row := range page.Items {
		items[i] = cloudProjectActivityView(row)
	}
	return CloudProjectActivityPageView{ProjectScopeKey: services.Scope.Key, ProjectID: input.ProjectID, Items: items, NextCursor: projectNullableStringView{value: cloneProjectString(page.NextCursor)}, ReadCursor: projectNullableStringView{value: cloneProjectString(page.ReadCursor)}}, nil
}
func (a *App) ListCloudProjectMessages(input CloudProjectMessageListInput) (CloudProjectActivityPageView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Activity }, "project messages")
	if err != nil {
		return CloudProjectActivityPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if err := a.recoverCloudProjectActivity(ctx, services, input.ProjectID); err != nil {
		recordDesktopBackgroundFailure("project.activity.recover", err)
	}
	page, err := services.Gateway.ListMessages(ctx, input.ProjectID, project.ListMessagesInput{PageSize: input.PageSize, Cursor: input.Cursor})
	if err != nil {
		return CloudProjectActivityPageView{}, projectProtocolWailsError(err)
	}
	items := make([]CloudProjectActivityView, len(page.Items))
	for i, row := range page.Items {
		items[i] = cloudProjectActivityView(row)
	}
	return CloudProjectActivityPageView{ProjectScopeKey: services.Scope.Key, ProjectID: input.ProjectID, Items: items, NextCursor: projectNullableStringView{value: cloneProjectString(page.NextCursor)}, ReadCursor: projectNullableStringView{value: cloneProjectString(page.ReadCursor)}}, nil
}
func (a *App) MarkCloudProjectActivityRead(input MarkCloudProjectActivityReadInput) error {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Activity }, "project activity")
	if err != nil {
		return projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, _, err := services.Gateway.MarkActivityRead(ctx, input.ProjectID, input.ThroughCursor)
	if err != nil {
		return projectProtocolWailsError(err)
	}
	if scopeKey != services.Scope.Key {
		return projectLocalWailsError(errors.New("project scope changed"), "PROJECT_SCOPE_CHANGED", "project scope changed during activity readback")
	}
	return nil
}
func (a *App) MarkCloudProjectMessageRead(input MarkCloudProjectMessageReadInput) error {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Activity }, "project messages")
	if err != nil {
		return projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, _, err := services.Gateway.MarkMessageRead(ctx, input.ProjectID, input.ThroughCursor)
	if err != nil {
		return projectProtocolWailsError(err)
	}
	if scopeKey != services.Scope.Key {
		return projectLocalWailsError(errors.New("project scope changed"), "PROJECT_SCOPE_CHANGED", "project scope changed during message readback")
	}
	return nil
}
func (a *App) PostCloudProjectMessage(input PostCloudProjectMessageInput) (CloudProjectActivityView, error) {
	services, err := a.projectServicesForV2(func(f project.CapabilityFeatures) bool { return f.Activity }, "project activity")
	if err != nil {
		return CloudProjectActivityView{}, projectWailsError(err)
	}
	if services.ActivityJournal == nil {
		return CloudProjectActivityView{}, projectLocalWailsError(errors.New("activity journal unavailable"), "PROJECT_ACTIVITY_UNAVAILABLE", "activity journal is unavailable")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	record, err := services.ActivityJournal.Prepare(ctx, services.Scope, input.ProjectID, input.Text, input.MentionUserIDs, input.AssetIDs)
	if err != nil {
		return CloudProjectActivityView{}, projectLocalWailsError(err, "PROJECT_ACTIVITY_UNAVAILABLE", "activity message could not be prepared")
	}
	_, err = services.Gateway.PostMessage(ctx, input.ProjectID, record.IdempotencyKey, project.PostMessageInput{ClientMessageID: record.ClientMessageID, Text: record.Text, MentionUserIDs: record.MentionUserIDs, AssetIDs: record.AssetIDs})
	if err != nil {
		return CloudProjectActivityView{}, projectProtocolWailsError(err)
	}
	message, err := readCloudProjectMessage(ctx, services, input.ProjectID, record.ClientMessageID)
	if err != nil {
		return CloudProjectActivityView{}, projectProtocolWailsError(err)
	}
	if err := services.ActivityJournal.Complete(ctx, services.Scope, record.ClientMessageID); err != nil {
		return CloudProjectActivityView{}, projectLocalWailsError(err, "PROJECT_ACTIVITY_UNAVAILABLE", "activity readback could not be completed")
	}
	return cloudProjectActivityView(message), nil
}
func (a *App) recoverCloudProjectActivity(ctx context.Context, services *cloudProjectServices, projectID string) error {
	if services.ActivityJournal == nil {
		return nil
	}
	pending, err := services.ActivityJournal.Pending(ctx, services.Scope, projectID)
	if err != nil {
		return err
	}
	for _, record := range pending {
		_, postErr := services.Gateway.PostMessage(ctx, projectID, record.IdempotencyKey, project.PostMessageInput{ClientMessageID: record.ClientMessageID, Text: record.Text, MentionUserIDs: record.MentionUserIDs, AssetIDs: record.AssetIDs})
		if postErr != nil {
			recordDesktopBackgroundFailure("project.activity.recover.post", postErr)
			continue
		}
		if _, readErr := readCloudProjectMessage(ctx, services, projectID, record.ClientMessageID); readErr != nil {
			recordDesktopBackgroundFailure("project.activity.recover.readback", readErr)
		} else if completeErr := services.ActivityJournal.Complete(ctx, services.Scope, record.ClientMessageID); completeErr != nil {
			recordDesktopBackgroundFailure("project.activity.recover.complete", completeErr)
		}
	}
	return nil
}

func readCloudProjectMessage(ctx context.Context, services *cloudProjectServices, projectID, clientMessageID string) (project.ProjectActivity, error) {
	page, err := services.Gateway.ListMessages(ctx, projectID, project.ListMessagesInput{PageSize: 100})
	if err != nil {
		return project.ProjectActivity{}, err
	}
	for _, message := range page.Items {
		if message.ClientMessageID == clientMessageID {
			return message, nil
		}
	}
	return project.ProjectActivity{}, errors.New("posted project message was not present in authoritative message list")
}

func cloudProjectActivityView(row project.ProjectActivity) CloudProjectActivityView {
	assets := make([]CloudProjectActivityAssetView, len(row.Assets))
	for i, asset := range row.Assets {
		assets[i] = CloudProjectActivityAssetView{AssetID: asset.AssetID, Filename: asset.Filename, ContentType: asset.ContentType, SizeBytes: asset.SizeBytes}
	}
	var readStates *[]CloudProjectMessageMemberReadStateView
	if row.OtherMemberReadStates != nil {
		states := make([]CloudProjectMessageMemberReadStateView, len(*row.OtherMemberReadStates))
		for i, state := range *row.OtherMemberReadStates {
			states[i] = CloudProjectMessageMemberReadStateView{UserID: state.UserID, DisplayName: state.DisplayName, Status: state.Status}
		}
		readStates = &states
	}
	return CloudProjectActivityView{ActivityID: row.ActivityID, Kind: row.Kind, ActorUserID: projectNullableStringView{value: cloneProjectString(row.ActorUserID)}, AuthorDisplayName: row.AuthorDisplayName, ClientMessageID: row.ClientMessageID, MessageText: row.MessageText, EventName: row.EventName, EventPayload: row.EventPayload, MentionUserIDs: append([]string(nil), row.MentionUserIDs...), Assets: assets, Unread: row.Unread, OtherMembersUnread: cloneProjectBool(row.OtherMembersUnread), UnreadOtherMemberCount: cloneProjectInt64(row.UnreadOtherMemberCount), OtherMemberReadStates: readStates, CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano)}
}

func cloneProjectBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneProjectInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
