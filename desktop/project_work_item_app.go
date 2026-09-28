package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
)

type CloudProjectWorkItemListInput struct {
	ProjectID      string `json:"projectId"`
	PageSize       int    `json:"pageSize,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
	Status         string `json:"status,omitempty"`
	Search         string `json:"search,omitempty"`
	AssigneeUserID string `json:"assigneeUserId,omitempty"`
	Priority       string `json:"priority,omitempty"`
	LabelID        string `json:"labelId,omitempty"`
	ScheduledFrom  string `json:"scheduledFrom,omitempty"`
	ScheduledTo    string `json:"scheduledTo,omitempty"`
}
type CreateCloudProjectWorkItemInput struct {
	ProjectID      string  `json:"projectId"`
	Title          string  `json:"title"`
	DescriptionSet bool    `json:"descriptionSet,omitempty"`
	Description    *string `json:"description"`
	Status         string  `json:"status,omitempty"`
	AssigneeUserID *string `json:"assigneeUserId,omitempty"`
	Priority       string  `json:"priority,omitempty"`
	StartAt        *string `json:"startAt,omitempty"`
	DueAt          *string `json:"dueAt,omitempty"`
}
type PatchCloudProjectWorkItemInput struct {
	ProjectID       string  `json:"projectId"`
	WorkItemID      string  `json:"workItemId"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	Title           *string `json:"title,omitempty"`
	DescriptionSet  bool    `json:"descriptionSet,omitempty"`
	Description     *string `json:"description"`
	AssigneeSet     bool    `json:"assigneeSet,omitempty"`
	AssigneeUserID  *string `json:"assigneeUserId"`
	Priority        *string `json:"priority,omitempty"`
	StartAtSet      bool    `json:"startAtSet,omitempty"`
	StartAt         *string `json:"startAt"`
	DueAtSet        bool    `json:"dueAtSet,omitempty"`
	DueAt           *string `json:"dueAt"`
}
type MoveCloudProjectWorkItemInput struct {
	ProjectID        string  `json:"projectId"`
	WorkItemID       string  `json:"workItemId"`
	ExpectedVersion  uint64  `json:"expectedVersion"`
	TargetStatus     string  `json:"targetStatus"`
	BeforeWorkItemID *string `json:"beforeWorkItemId,omitempty"`
	AfterWorkItemID  *string `json:"afterWorkItemId,omitempty"`
}
type DeleteCloudProjectWorkItemInput struct {
	ProjectID       string `json:"projectId"`
	WorkItemID      string `json:"workItemId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type CloudProjectWorkItemView struct {
	WorkItemID          string                    `json:"workItemId"`
	ProjectID           string                    `json:"projectId"`
	Title               string                    `json:"title"`
	Description         projectNullableStringView `json:"description" ts_type:"string | null"`
	Status              string                    `json:"status"`
	Rank                int64                     `json:"rank"`
	Version             uint64                    `json:"version"`
	CreatedAt           string                    `json:"createdAt"`
	UpdatedAt           string                    `json:"updatedAt"`
	AssigneeUserID      projectNullableStringView `json:"assigneeUserId" ts_type:"string | null"`
	AssigneeStatus      projectNullableStringView `json:"assigneeStatus" ts_type:"string | null"`
	AssigneeDisplayName projectNullableStringView `json:"assigneeDisplayName" ts_type:"string | null"`
	Priority            string                    `json:"priority"`
	StartAt             projectNullableStringView `json:"startAt" ts_type:"string | null"`
	DueAt               projectNullableStringView `json:"dueAt" ts_type:"string | null"`
	Labels              []CloudProjectLabelView   `json:"labels"`
}
type CloudProjectLabelView struct {
	LabelID string `json:"labelId"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	Version uint64 `json:"version"`
}
type CloudProjectLabelListView struct {
	ProjectScopeKey string                  `json:"projectScopeKey"`
	ProjectID       string                  `json:"projectId"`
	Items           []CloudProjectLabelView `json:"items"`
}
type CreateCloudProjectLabelInput struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Color     string `json:"color"`
}
type PatchCloudProjectLabelInput struct {
	ProjectID       string  `json:"projectId"`
	LabelID         string  `json:"labelId"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	Name            *string `json:"name,omitempty"`
	Color           *string `json:"color,omitempty"`
}
type DeleteCloudProjectLabelInput struct {
	ProjectID       string `json:"projectId"`
	LabelID         string `json:"labelId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type MutateCloudProjectWorkItemLabelInput struct {
	ProjectID       string `json:"projectId"`
	WorkItemID      string `json:"workItemId"`
	LabelID         string `json:"labelId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type CloudProjectWorkItemResultView struct {
	ProjectScopeKey string                   `json:"projectScopeKey"`
	ProjectID       string                   `json:"projectId"`
	WorkItem        CloudProjectWorkItemView `json:"workItem"`
}
type CloudProjectWorkItemPageView struct {
	ProjectScopeKey string                     `json:"projectScopeKey"`
	ProjectID       string                     `json:"projectId"`
	Items           []CloudProjectWorkItemView `json:"items"`
	NextCursor      projectNullableStringView  `json:"nextCursor" ts_type:"string | null"`
	TotalCount      uint64                     `json:"totalCount"`
	UndatedCount    uint64                     `json:"undatedCount"`
}
type CloudProjectWorkItemDeleteView struct {
	ProjectScopeKey  string `json:"projectScopeKey"`
	ProjectID        string `json:"projectId"`
	WorkItemID       string `json:"workItemId"`
	CommittedVersion uint64 `json:"committedVersion"`
}

func (a *App) ListCloudProjectWorkItems(input CloudProjectWorkItemListInput) (CloudProjectWorkItemPageView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectWorkItemPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	page, err := services.Gateway.ListWorkItems(ctx, input.ProjectID, project.ListWorkItemsInput{PageSize: input.PageSize, Cursor: input.Cursor, Status: project.WorkItemStatus(input.Status), Search: input.Search, AssigneeUserID: input.AssigneeUserID, Priority: input.Priority, LabelID: input.LabelID, ScheduledFrom: input.ScheduledFrom, ScheduledTo: input.ScheduledTo})
	if err != nil {
		return CloudProjectWorkItemPageView{}, projectProtocolWailsError(err)
	}
	items := make([]CloudProjectWorkItemView, len(page.Items))
	for index, item := range page.Items {
		items[index] = cloudProjectWorkItemView(item)
	}
	return CloudProjectWorkItemPageView{ProjectScopeKey: services.Scope.Key, ProjectID: input.ProjectID, Items: items, NextCursor: projectNullableStringView{value: cloneProjectString(page.NextCursor)}, TotalCount: page.TotalCount, UndatedCount: page.UndatedCount}, nil
}

func (a *App) ListCloudProjectLabels(projectID string) (CloudProjectLabelListView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectLabelListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, labels, err := services.Gateway.ListLabels(ctx, projectID)
	if err != nil {
		return CloudProjectLabelListView{}, projectProtocolWailsError(err)
	}
	items := make([]CloudProjectLabelView, len(labels))
	for index, label := range labels {
		items[index] = CloudProjectLabelView{LabelID: label.LabelID, Name: label.Name, Color: label.Color, Version: label.Version}
	}
	return CloudProjectLabelListView{ProjectScopeKey: scopeKey, ProjectID: projectID, Items: items}, nil
}

func (a *App) CreateCloudProjectLabel(input CreateCloudProjectLabelInput) (CloudProjectLabelListView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectLabelListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	key, err := projectRandomKey()
	if err != nil {
		return CloudProjectLabelListView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "label command could not be prepared")
	}
	result, err := services.Gateway.CreateLabel(ctx, input.ProjectID, input.Name, input.Color, key)
	if err != nil {
		return CloudProjectLabelListView{}, projectProtocolWailsError(err)
	}
	scopeKey, labels, err := services.Gateway.ListLabels(ctx, input.ProjectID)
	if err != nil {
		return CloudProjectLabelListView{}, projectProtocolWailsError(err)
	}
	found := false
	items := make([]CloudProjectLabelView, len(labels))
	for index, label := range labels {
		items[index] = CloudProjectLabelView{LabelID: label.LabelID, Name: label.Name, Color: label.Color, Version: label.Version}
		found = found || label.LabelID == result.ResourceID
	}
	if !found {
		return CloudProjectLabelListView{}, projectLocalWailsError(errors.New("label readback mismatch"), "PROJECT_COMMAND_UNAVAILABLE", "label command readback is incomplete")
	}
	return CloudProjectLabelListView{ProjectScopeKey: scopeKey, ProjectID: input.ProjectID, Items: items}, nil
}

func (a *App) PatchCloudProjectLabel(input PatchCloudProjectLabelInput) (CloudProjectLabelListView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectLabelListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	key, err := projectRandomKey()
	if err != nil {
		return CloudProjectLabelListView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "label command could not be prepared")
	}
	result, err := services.Gateway.PatchLabel(ctx, input.ProjectID, input.LabelID, input.ExpectedVersion, input.Name, input.Color, key)
	if err != nil {
		return CloudProjectLabelListView{}, projectProtocolWailsError(err)
	}
	return a.readbackCloudProjectLabels(ctx, services, input.ProjectID, result.ResourceID, true)
}

func (a *App) DeleteCloudProjectLabel(input DeleteCloudProjectLabelInput) (CloudProjectLabelListView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectLabelListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	key, err := projectRandomKey()
	if err != nil {
		return CloudProjectLabelListView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "label command could not be prepared")
	}
	result, err := services.Gateway.DeleteLabel(ctx, input.ProjectID, input.LabelID, input.ExpectedVersion, key)
	if err != nil {
		return CloudProjectLabelListView{}, projectProtocolWailsError(err)
	}
	return a.readbackCloudProjectLabels(ctx, services, input.ProjectID, result.ResourceID, false)
}

func (a *App) readbackCloudProjectLabels(ctx context.Context, services *cloudProjectServices, projectID, labelID string, shouldExist bool) (CloudProjectLabelListView, error) {
	scopeKey, labels, err := services.Gateway.ListLabels(ctx, projectID)
	if err != nil {
		return CloudProjectLabelListView{}, projectProtocolWailsError(err)
	}
	found := false
	items := make([]CloudProjectLabelView, len(labels))
	for index, label := range labels {
		items[index] = CloudProjectLabelView{LabelID: label.LabelID, Name: label.Name, Color: label.Color, Version: label.Version}
		found = found || label.LabelID == labelID
	}
	if found != shouldExist {
		return CloudProjectLabelListView{}, projectLocalWailsError(errors.New("label readback mismatch"), "PROJECT_COMMAND_UNAVAILABLE", "label command readback is incomplete")
	}
	return CloudProjectLabelListView{ProjectScopeKey: scopeKey, ProjectID: projectID, Items: items}, nil
}

func (a *App) PutCloudProjectWorkItemLabel(input MutateCloudProjectWorkItemLabelInput) (CloudProjectWorkItemResultView, error) {
	return a.mutateCloudProjectWorkItemLabel(input, true)
}

func (a *App) DeleteCloudProjectWorkItemLabel(input MutateCloudProjectWorkItemLabelInput) (CloudProjectWorkItemResultView, error) {
	return a.mutateCloudProjectWorkItemLabel(input, false)
}

func (a *App) mutateCloudProjectWorkItemLabel(input MutateCloudProjectWorkItemLabelInput, put bool) (CloudProjectWorkItemResultView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectWorkItemResultView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	key, err := projectRandomKey()
	if err != nil {
		return CloudProjectWorkItemResultView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "work item label command could not be prepared")
	}
	result, err := services.Gateway.MutateWorkItemLabel(ctx, input.ProjectID, input.WorkItemID, input.LabelID, key, input.ExpectedVersion, put)
	if err != nil {
		return CloudProjectWorkItemResultView{}, projectProtocolWailsError(err)
	}
	return cloudProjectWorkItemResultView(result), nil
}

func (a *App) CreateCloudProjectWorkItem(input CreateCloudProjectWorkItemInput) (CloudProjectWorkItemResultView, error) {
	startAt, err := parseOptionalProjectTime(input.StartAt)
	if err != nil {
		return CloudProjectWorkItemResultView{}, err
	}
	dueAt, err := parseOptionalProjectTime(input.DueAt)
	if err != nil {
		return CloudProjectWorkItemResultView{}, err
	}
	domain := project.CreateWorkItemInput{Title: input.Title, Description: project.NullableStringUpdate{Set: input.DescriptionSet, Value: cloneProjectString(input.Description)}, Status: project.WorkItemStatus(input.Status), AssigneeUserID: project.NullableStringUpdate{Set: input.AssigneeUserID != nil, Value: cloneProjectString(input.AssigneeUserID)}, Priority: optionalNonEmpty(input.Priority), StartAt: startAt, DueAt: dueAt}
	payload := map[string]any{"title": input.Title}
	if input.DescriptionSet {
		payload["description"] = input.Description
	}
	if input.Status != "" {
		payload["status"] = input.Status
	}
	if input.AssigneeUserID != nil {
		payload["assignee_user_id"] = input.AssigneeUserID
	}
	if input.Priority != "" {
		payload["priority"] = input.Priority
	}
	if input.StartAt != nil {
		payload["start_at"] = input.StartAt
	}
	if input.DueAt != nil {
		payload["due_at"] = input.DueAt
	}
	return a.runCloudProjectWorkItemMutation(input.ProjectID, "", project.ProjectCommandWorkItemCreate, payload, func(ctx context.Context, services *cloudProjectServices, key string) (project.CommandResult, error) {
		return services.Gateway.CreateWorkItem(ctx, input.ProjectID, domain, key)
	})
}

func (a *App) PatchCloudProjectWorkItem(input PatchCloudProjectWorkItemInput) (CloudProjectWorkItemResultView, error) {
	startAt, err := parseOptionalProjectTime(input.StartAt)
	if err != nil {
		return CloudProjectWorkItemResultView{}, err
	}
	dueAt, err := parseOptionalProjectTime(input.DueAt)
	if err != nil {
		return CloudProjectWorkItemResultView{}, err
	}
	domain := project.PatchWorkItemInput{ExpectedVersion: input.ExpectedVersion, Title: cloneProjectString(input.Title), Description: project.NullableStringUpdate{Set: input.DescriptionSet, Value: cloneProjectString(input.Description)}, AssigneeUserID: project.NullableStringUpdate{Set: input.AssigneeSet, Value: cloneProjectString(input.AssigneeUserID)}, Priority: input.Priority, StartAt: project.NullableTimeUpdate{Set: input.StartAtSet, Value: startAt}, DueAt: project.NullableTimeUpdate{Set: input.DueAtSet, Value: dueAt}}
	payload := map[string]any{"expected_version": input.ExpectedVersion}
	if input.Title != nil {
		payload["title"] = *input.Title
	}
	if input.DescriptionSet {
		payload["description"] = input.Description
	}
	if input.AssigneeSet {
		payload["assignee_user_id"] = input.AssigneeUserID
	}
	if input.Priority != nil {
		payload["priority"] = *input.Priority
	}
	if input.StartAtSet {
		payload["start_at"] = input.StartAt
	}
	if input.DueAtSet {
		payload["due_at"] = input.DueAt
	}
	return a.runCloudProjectWorkItemMutation(input.ProjectID, input.WorkItemID, project.ProjectCommandWorkItemPatch, payload, func(ctx context.Context, services *cloudProjectServices, key string) (project.CommandResult, error) {
		return services.Gateway.PatchWorkItem(ctx, input.ProjectID, input.WorkItemID, domain, key)
	})
}

func (a *App) MoveCloudProjectWorkItem(input MoveCloudProjectWorkItemInput) (CloudProjectWorkItemResultView, error) {
	domain := project.MoveWorkItemInput{ExpectedVersion: input.ExpectedVersion, TargetStatus: project.WorkItemStatus(input.TargetStatus), BeforeWorkItemID: cloneProjectString(input.BeforeWorkItemID), AfterWorkItemID: cloneProjectString(input.AfterWorkItemID)}
	payload := map[string]any{"expected_version": input.ExpectedVersion, "target_status": input.TargetStatus}
	if input.BeforeWorkItemID != nil {
		payload["before_work_item_id"] = *input.BeforeWorkItemID
	}
	if input.AfterWorkItemID != nil {
		payload["after_work_item_id"] = *input.AfterWorkItemID
	}
	return a.runCloudProjectWorkItemMutation(input.ProjectID, input.WorkItemID, project.ProjectCommandWorkItemMove, payload, func(ctx context.Context, services *cloudProjectServices, key string) (project.CommandResult, error) {
		return services.Gateway.MoveWorkItem(ctx, input.ProjectID, input.WorkItemID, domain, key)
	})
}

func (a *App) DeleteCloudProjectWorkItem(input DeleteCloudProjectWorkItemInput) (CloudProjectWorkItemDeleteView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectWorkItemDeleteView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	payload := map[string]any{"expected_version": input.ExpectedVersion}
	if err := waitForCloudProjectCommandReplay(ctx, services); err != nil {
		return CloudProjectWorkItemDeleteView{}, projectCommandPrepareWailsError(err)
	}
	record, err := services.Journal.Prepare(ctx, services.Scope, project.PrepareProjectCommandInput{Operation: project.ProjectCommandWorkItemDelete, ProjectID: input.ProjectID, ResourceID: input.WorkItemID, Payload: payload})
	if err != nil {
		return CloudProjectWorkItemDeleteView{}, projectCommandPrepareWailsError(err)
	}
	workBuddyProjectFaultPoint("journal_durable_before_http", record.IdempotencyKey)
	result, err := services.Gateway.DeleteWorkItem(ctx, input.ProjectID, input.WorkItemID, project.DeleteWorkItemInput{ExpectedVersion: input.ExpectedVersion}, record.IdempotencyKey)
	if err != nil {
		err = settleProjectCommandFailure(ctx, services, record.IdempotencyKey, err)
		return CloudProjectWorkItemDeleteView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("response_before_authoritative_readback", record.IdempotencyKey)
	if err := proveCloudProjectWorkItemAbsent(ctx, services, input.ProjectID, input.WorkItemID); err != nil {
		return CloudProjectWorkItemDeleteView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("readback_before_journal_eviction", record.IdempotencyKey)
	proof := project.ProjectCommandDeletionReadback{ProjectScopeKey: services.Scope.Key, ResourceKind: "work_item", ResourceID: input.WorkItemID, ProjectID: input.ProjectID, Absent: true}
	if err := services.Journal.CompleteAfterDeletion(ctx, services.Scope, record.IdempotencyKey, result, proof); err != nil {
		return CloudProjectWorkItemDeleteView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "work item deletion readback could not be completed")
	}
	return CloudProjectWorkItemDeleteView{ProjectScopeKey: services.Scope.Key, ProjectID: input.ProjectID, WorkItemID: input.WorkItemID, CommittedVersion: result.CommittedVersion}, nil
}

func proveCloudProjectWorkItemAbsent(ctx context.Context, services *cloudProjectServices, projectID, workItemID string) error {
	_, err := services.Gateway.GetWorkItem(ctx, projectID, workItemID)
	if err == nil {
		return errors.New("projects-v1: deleted work item remains present")
	}
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("NOT_FOUND") {
		return err
	}
	return nil
}

func (a *App) runCloudProjectWorkItemMutation(projectID, workItemID string, operation project.ProjectCommandOperation, payload map[string]any, send func(context.Context, *cloudProjectServices, string) (project.CommandResult, error)) (CloudProjectWorkItemResultView, error) {
	services, err := a.projectServicesForWorkItemOperation()
	if err != nil {
		return CloudProjectWorkItemResultView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if err := waitForCloudProjectCommandReplay(ctx, services); err != nil {
		return CloudProjectWorkItemResultView{}, projectCommandPrepareWailsError(err)
	}
	record, err := services.Journal.Prepare(ctx, services.Scope, project.PrepareProjectCommandInput{Operation: operation, ProjectID: projectID, ResourceID: workItemID, Payload: payload})
	if err != nil {
		return CloudProjectWorkItemResultView{}, projectCommandPrepareWailsError(err)
	}
	workBuddyProjectFaultPoint("journal_durable_before_http", record.IdempotencyKey)
	result, err := send(ctx, services, record.IdempotencyKey)
	if err != nil {
		err = settleProjectCommandFailure(ctx, services, record.IdempotencyKey, err)
		if operation == project.ProjectCommandWorkItemMove {
			return CloudProjectWorkItemResultView{}, projectWorkItemMoveWailsError(err)
		}
		return CloudProjectWorkItemResultView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("response_before_authoritative_readback", record.IdempotencyKey)
	readback, err := services.Gateway.GetWorkItem(ctx, projectID, result.ResourceID)
	if err != nil {
		return CloudProjectWorkItemResultView{}, projectProtocolWailsError(err)
	}
	workBuddyProjectFaultPoint("readback_before_journal_eviction", record.IdempotencyKey)
	proof := project.ProjectCommandReadback{ProjectScopeKey: readback.ProjectScopeKey, ResourceKind: "work_item", ResourceID: readback.WorkItem.WorkItemID, ProjectID: readback.WorkItem.ProjectID, Version: readback.WorkItem.Version}
	if err := services.Journal.CompleteAfterReadback(ctx, services.Scope, record.IdempotencyKey, result, proof); err != nil {
		return CloudProjectWorkItemResultView{}, projectLocalWailsError(err, "PROJECT_COMMAND_UNAVAILABLE", "work item command readback could not be completed")
	}
	return cloudProjectWorkItemResultView(readback), nil
}

func projectWorkItemMoveWailsError(err error) error {
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("CONFLICT") {
		return projectProtocolWailsError(err)
	}
	var details struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal(typed.Details, &details) == nil && (details.Reason == "version_mismatch" || details.Reason == "anchor_invalid") {
		safe := cloud.AttachRequestID(&cloud.Error{Code: cloud.ErrorCode("WORK_ITEM_MOVE_RETRYABLE_CONFLICT"), Message: "work item moved concurrently; refresh before retry"}, typed.RequestID)
		return projectDiagnosticWailsError("project.work_item.move", err, safe)
	}
	return projectProtocolWailsError(err)
}

func cloudProjectWorkItemResultView(result project.WorkItemResult) CloudProjectWorkItemResultView {
	return CloudProjectWorkItemResultView{ProjectScopeKey: result.ProjectScopeKey, ProjectID: result.WorkItem.ProjectID, WorkItem: cloudProjectWorkItemView(result.WorkItem)}
}
func cloudProjectWorkItemView(item project.WorkItem) CloudProjectWorkItemView {
	labels := make([]CloudProjectLabelView, len(item.Labels))
	for i, label := range item.Labels {
		labels[i] = CloudProjectLabelView{LabelID: label.LabelID, Name: label.Name, Color: label.Color, Version: label.Version}
	}
	return CloudProjectWorkItemView{WorkItemID: item.WorkItemID, ProjectID: item.ProjectID, Title: item.Title, Description: projectNullableStringView{value: cloneProjectString(item.Description)}, Status: string(item.Status), Rank: item.Rank, Version: item.Version, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano), AssigneeUserID: projectNullableStringView{value: cloneProjectString(item.AssigneeUserID)}, AssigneeStatus: projectNullableStringView{value: cloneProjectString(item.AssigneeStatusSnapshot)}, AssigneeDisplayName: projectNullableStringView{value: cloneProjectString(item.AssigneeDisplaySnapshot)}, Priority: item.Priority, StartAt: projectNullableStringView{value: projectTimeString(item.StartAt)}, DueAt: projectNullableStringView{value: projectTimeString(item.DueAt)}, Labels: labels}
}

func optionalNonEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func parseOptionalProjectTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, &cloud.Error{Code: cloud.ErrorCode("VALIDATION_FAILED"), Message: "project date is invalid"}
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
func projectTimeString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.UTC().Format(time.RFC3339Nano)
	return &text
}
