package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ListWorkItemsInput struct {
	PageSize       int
	Cursor         string
	Status         WorkItemStatus
	Search         string
	AssigneeUserID string
	Priority       string
	LabelID        string
	ScheduledFrom  string
	ScheduledTo    string
}
type CreateWorkItemInput struct {
	Title          string
	Description    NullableStringUpdate
	Status         WorkItemStatus
	AssigneeUserID NullableStringUpdate
	Priority       *string
	StartAt        *time.Time
	DueAt          *time.Time
}
type PatchWorkItemInput struct {
	ExpectedVersion uint64
	Title           *string
	Description     NullableStringUpdate
	AssigneeUserID  NullableStringUpdate
	Priority        *string
	StartAt         NullableTimeUpdate
	DueAt           NullableTimeUpdate
}
type NullableTimeUpdate struct {
	Set   bool
	Value *time.Time
}
type MoveWorkItemInput struct {
	ExpectedVersion  uint64
	TargetStatus     WorkItemStatus
	BeforeWorkItemID *string
	AfterWorkItemID  *string
}
type DeleteWorkItemInput struct {
	ExpectedVersion uint64
}
type WorkItemResult struct {
	ProjectScopeKey string
	WorkItem        WorkItem
}

func (g *ProjectGateway) ListWorkItems(ctx context.Context, projectID string, input ListWorkItemsInput) (WorkItemPage, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return WorkItemPage{}, err
	}
	query, err := workItemListQuery(input)
	if err != nil {
		return WorkItemPage{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return WorkItemPage{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, workItemsPath(projectID), query, nil, token, nil, &payload)
	})
	if err != nil {
		return WorkItemPage{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return WorkItemPage{}, err
	}
	return DecodeWorkItemPage(payload, scope, projectID)
}

func (g *ProjectGateway) GetWorkItem(ctx context.Context, projectID, workItemID string) (WorkItemResult, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return WorkItemResult{}, err
	}
	if err := validateOpaqueProjectID(workItemID); err != nil {
		return WorkItemResult{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return WorkItemResult{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, workItemPath(workItemID), nil, token, nil, &payload)
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

func (g *ProjectGateway) CreateWorkItem(ctx context.Context, projectID string, input CreateWorkItemInput, key string) (CommandResult, error) {
	if err := validateCreateWorkItemInput(projectID, input, key); err != nil {
		return CommandResult{}, err
	}
	return g.mutateWorkItem(ctx, http.MethodPost, workItemsPath(projectID), createWorkItemBody(input), key, projectID, "")
}

func validateCreateWorkItemInput(projectID string, input CreateWorkItemInput, key string) error {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return err
	}
	if err := validateWorkItemTitle(input.Title); err != nil {
		return err
	}
	if input.Description.Set && !validNullableWorkItemDescription(input.Description.Value) {
		return fmt.Errorf("projects-v1: invalid work item description")
	}
	if input.Status != "" && !validWorkItemStatus(input.Status) {
		return fmt.Errorf("projects-v1: invalid work item status")
	}
	return validateIdempotencyKey(key)
}

func createWorkItemBody(input CreateWorkItemInput) map[string]any {
	body := map[string]any{"title": input.Title}
	if input.Description.Set {
		body["description"] = input.Description.Value
	}
	if input.Status != "" {
		body["status"] = input.Status
	}
	if input.AssigneeUserID.Set {
		body["assignee_user_id"] = input.AssigneeUserID.Value
	}
	if input.Priority != nil {
		body["priority"] = *input.Priority
	}
	if input.StartAt != nil {
		body["start_at"] = input.StartAt.UTC()
	}
	if input.DueAt != nil {
		body["due_at"] = input.DueAt.UTC()
	}
	return body
}

func (g *ProjectGateway) PatchWorkItem(ctx context.Context, projectID, workItemID string, input PatchWorkItemInput, key string) (CommandResult, error) {
	if err := validatePatchWorkItemInput(projectID, workItemID, input, key); err != nil {
		return CommandResult{}, err
	}
	return g.mutateWorkItem(ctx, http.MethodPatch, workItemPath(workItemID), patchWorkItemBody(input), key, projectID, workItemID)
}

func validatePatchWorkItemInput(projectID, workItemID string, input PatchWorkItemInput, key string) error {
	if err := validateWorkItemMutation(projectID, workItemID, input.ExpectedVersion, key); err != nil {
		return err
	}
	if !patchWorkItemHasMutation(input) {
		return fmt.Errorf("projects-v1: work item patch requires a mutation")
	}
	return validatePatchWorkItemText(input)
}

func patchWorkItemHasMutation(input PatchWorkItemInput) bool {
	return input.Title != nil || input.Description.Set || input.AssigneeUserID.Set || input.Priority != nil || input.StartAt.Set || input.DueAt.Set
}

func validatePatchWorkItemText(input PatchWorkItemInput) error {
	if input.Title != nil {
		if err := validateWorkItemTitle(*input.Title); err != nil {
			return err
		}
	}
	if input.Description.Set && !validNullableWorkItemDescription(input.Description.Value) {
		return fmt.Errorf("projects-v1: invalid work item description")
	}
	return nil
}

func patchWorkItemBody(input PatchWorkItemInput) map[string]any {
	body := map[string]any{"expected_version": input.ExpectedVersion}
	if input.Title != nil {
		body["title"] = *input.Title
	}
	if input.Description.Set {
		body["description"] = input.Description.Value
	}
	if input.AssigneeUserID.Set {
		body["assignee_user_id"] = input.AssigneeUserID.Value
	}
	if input.Priority != nil {
		body["priority"] = *input.Priority
	}
	if input.StartAt.Set {
		body["start_at"] = input.StartAt.Value
	}
	if input.DueAt.Set {
		body["due_at"] = input.DueAt.Value
	}
	return body
}

func (g *ProjectGateway) MoveWorkItem(ctx context.Context, projectID, workItemID string, input MoveWorkItemInput, key string) (CommandResult, error) {
	if err := validateWorkItemMutation(projectID, workItemID, input.ExpectedVersion, key); err != nil {
		return CommandResult{}, err
	}
	if !validWorkItemStatus(input.TargetStatus) {
		return CommandResult{}, fmt.Errorf("projects-v1: invalid target_status")
	}
	if input.BeforeWorkItemID != nil && input.AfterWorkItemID != nil {
		return CommandResult{}, fmt.Errorf("projects-v1: work item move accepts at most one anchor")
	}
	body := map[string]any{"expected_version": input.ExpectedVersion, "target_status": input.TargetStatus}
	if input.BeforeWorkItemID != nil {
		if err := validateMoveAnchor(*input.BeforeWorkItemID, workItemID); err != nil {
			return CommandResult{}, err
		}
		body["before_work_item_id"] = *input.BeforeWorkItemID
	}
	if input.AfterWorkItemID != nil {
		if err := validateMoveAnchor(*input.AfterWorkItemID, workItemID); err != nil {
			return CommandResult{}, err
		}
		body["after_work_item_id"] = *input.AfterWorkItemID
	}
	return g.mutateWorkItem(ctx, http.MethodPost, workItemPath(workItemID)+"/move", body, key, projectID, workItemID)
}

func (g *ProjectGateway) DeleteWorkItem(ctx context.Context, projectID, workItemID string, input DeleteWorkItemInput, key string) (CommandResult, error) {
	if err := validateWorkItemMutation(projectID, workItemID, input.ExpectedVersion, key); err != nil {
		return CommandResult{}, err
	}
	return g.mutateWorkItem(ctx, http.MethodDelete, workItemPath(workItemID), map[string]any{"expected_version": input.ExpectedVersion}, key, projectID, workItemID)
}

func (g *ProjectGateway) mutateWorkItem(ctx context.Context, method, path string, body any, key, projectID, workItemID string) (CommandResult, error) {
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
	result, err := decodeCommandResult(payload, scope, projectID, "work_item")
	if err != nil {
		return CommandResult{}, err
	}
	if workItemID != "" && result.ResourceID != workItemID {
		return CommandResult{}, fmt.Errorf("projects-v1: command result work item ID mismatch")
	}
	return result, nil
}

func workItemListQuery(input ListWorkItemsInput) (url.Values, error) {
	pageSize, err := projectListPageSize(input.PageSize)
	if err != nil {
		return nil, err
	}
	status, err := workItemListStatus(input.Status)
	if err != nil {
		return nil, err
	}
	if err := validateWorkItemListCursor(input.Cursor); err != nil {
		return nil, err
	}
	query := url.Values{"page_size": {strconv.Itoa(pageSize)}, "status": {string(status)}}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	if err := setWorkItemSearchQuery(query, input.Search); err != nil {
		return nil, err
	}
	if err := setWorkItemOpaqueQuery(query, "assignee_user_id", input.AssigneeUserID); err != nil {
		return nil, err
	}
	if err := setWorkItemPriorityQuery(query, input.Priority); err != nil {
		return nil, err
	}
	if err := setWorkItemOpaqueQuery(query, "label_id", input.LabelID); err != nil {
		return nil, err
	}
	if err := setWorkItemScheduledRangeQuery(query, input.ScheduledFrom, input.ScheduledTo); err != nil {
		return nil, err
	}
	return query, nil
}

func setWorkItemScheduledRangeQuery(query url.Values, from, to string) error {
	if from == "" && to == "" {
		return nil
	}
	if from == "" || to == "" {
		return fmt.Errorf("projects-v1: scheduled range must be paired")
	}
	fromDate, fromErr := parseWorkItemScheduledDate(from)
	toDate, toErr := parseWorkItemScheduledDate(to)
	if fromErr != nil || toErr != nil || toDate.Before(fromDate) || toDate.Sub(fromDate) > 41*24*time.Hour {
		return fmt.Errorf("projects-v1: invalid scheduled range")
	}
	query.Set("scheduled_from", from)
	query.Set("scheduled_to", to)
	return nil
}

func parseWorkItemScheduledDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("projects-v1: invalid scheduled date")
	}
	return parsed, nil
}

func setWorkItemSearchQuery(query url.Values, value string) error {
	search := strings.TrimSpace(value)
	if len([]byte(search)) > 200 {
		return fmt.Errorf("projects-v1: invalid work item search")
	}
	if search != "" {
		query.Set("search", search)
	}
	return nil
}

func setWorkItemOpaqueQuery(query url.Values, name, value string) error {
	if value == "" {
		return nil
	}
	if err := validateOpaqueProjectID(value); err != nil {
		return err
	}
	query.Set(name, value)
	return nil
}

func setWorkItemPriorityQuery(query url.Values, priority string) error {
	if priority == "" {
		return nil
	}
	if priority != "none" && priority != "low" && priority != "medium" && priority != "high" && priority != "urgent" {
		return fmt.Errorf("projects-v1: invalid work item priority")
	}
	query.Set("priority", priority)
	return nil
}
func workItemListStatus(status WorkItemStatus) (WorkItemStatus, error) {
	if status == "" {
		status = "all"
	}
	if status != "all" && !validWorkItemStatus(status) {
		return "", fmt.Errorf("projects-v1: invalid work item list status")
	}
	return status, nil
}
func validateWorkItemListCursor(cursor string) error {
	if cursor != "" && (strings.TrimSpace(cursor) != cursor || len([]byte(cursor)) > 2048) {
		return fmt.Errorf("projects-v1: invalid work item cursor")
	}
	return nil
}
func workItemsPath(projectID string) string { return projectPath(projectID) + "/work-items" }
func workItemPath(workItemID string) string {
	return "/api/v1/work-items/" + url.PathEscape(workItemID)
}
func validateWorkItemTitle(value string) error {
	if strings.TrimSpace(value) == "" || !validProjectText(value, 300) {
		return fmt.Errorf("projects-v1: invalid work item title")
	}
	return nil
}
func validNullableWorkItemDescription(value *string) bool {
	return value == nil || validProjectText(*value, 8000)
}
func validateWorkItemMutation(projectID, workItemID string, version uint64, key string) error {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return err
	}
	if err := validateOpaqueProjectID(workItemID); err != nil {
		return err
	}
	if version == 0 || version > MaxExpectedVersion {
		return fmt.Errorf("projects-v1: work item expected_version must be between 1 and %d", MaxExpectedVersion)
	}
	return validateIdempotencyKey(key)
}
func validateMoveAnchor(anchor, itemID string) error {
	if err := validateOpaqueProjectID(anchor); err != nil {
		return fmt.Errorf("projects-v1: invalid work item anchor")
	}
	if anchor == itemID {
		return fmt.Errorf("projects-v1: work item anchor cannot reference itself")
	}
	return nil
}
