package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type WorkItemStatus string

const (
	WorkItemTodo       WorkItemStatus = "todo"
	WorkItemInProgress WorkItemStatus = "in_progress"
	WorkItemPaused     WorkItemStatus = "paused"
	WorkItemDone       WorkItemStatus = "done"
)

type WorkItem struct {
	WorkItemID              string
	ProjectID               string
	Title                   string
	Description             *string
	Status                  WorkItemStatus
	Rank                    int64
	Version                 uint64
	CreatedAt               time.Time
	UpdatedAt               time.Time
	AssigneeUserID          *string
	AssigneeStatusSnapshot  *string
	AssigneeDisplaySnapshot *string
	Priority                string
	StartAt                 *time.Time
	DueAt                   *time.Time
	Labels                  []ProjectLabel
}

type ProjectLabel struct {
	LabelID   string
	ProjectID string
	Name      string
	Color     string
	Version   uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WorkItemPage struct {
	Items        []WorkItem
	NextCursor   *string
	TotalCount   uint64
	UndatedCount uint64
	scopeKey     string
	projectID    string
}

type workItemWire struct {
	WorkItemID              string             `json:"work_item_id"`
	ProjectID               string             `json:"project_id"`
	Title                   string             `json:"title"`
	Description             *string            `json:"description"`
	Status                  string             `json:"status"`
	Rank                    int64              `json:"rank"`
	Version                 uint64             `json:"version"`
	CreatedAt               time.Time          `json:"created_at"`
	UpdatedAt               time.Time          `json:"updated_at"`
	AssigneeUserID          *string            `json:"assignee_user_id"`
	AssigneeStatusSnapshot  *string            `json:"assignee_status_snapshot"`
	AssigneeDisplaySnapshot *string            `json:"assignee_display_snapshot"`
	Priority                string             `json:"priority"`
	StartAt                 *time.Time         `json:"start_at"`
	DueAt                   *time.Time         `json:"due_at"`
	Labels                  []projectLabelWire `json:"labels"`
}

type projectLabelWire struct {
	LabelID   string    `json:"label_id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Version   uint64    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type workItemPageWire struct {
	Items        []workItemWire `json:"items"`
	NextCursor   *string        `json:"next_cursor"`
	TotalCount   uint64         `json:"total_count"`
	UndatedCount uint64         `json:"undated_count"`
}

func DecodeWorkItemPage(payload []byte, scope Scope, expectedProjectID string) (WorkItemPage, error) {
	if err := validateWorkItemPageEnvelope(payload, scope, expectedProjectID); err != nil {
		return WorkItemPage{}, err
	}
	if err := validateWorkItemPageShape(payload); err != nil {
		return WorkItemPage{}, err
	}
	wire, err := decodeWorkItemPageWire(payload)
	if err != nil {
		return WorkItemPage{}, err
	}
	items, err := workItemWiresDomain(wire.Items, expectedProjectID)
	if err != nil {
		return WorkItemPage{}, err
	}
	if err := validateWorkItemNextCursor(wire.NextCursor); err != nil {
		return WorkItemPage{}, err
	}
	if wire.TotalCount > 10_000 || wire.UndatedCount > 10_000 || uint64(len(items)) > wire.TotalCount {
		return WorkItemPage{}, fmt.Errorf("projects-v1: invalid work item total_count")
	}
	return WorkItemPage{Items: items, NextCursor: cloneString(wire.NextCursor), TotalCount: wire.TotalCount, UndatedCount: wire.UndatedCount, scopeKey: scope.Key, projectID: expectedProjectID}, nil
}

func validateWorkItemPageEnvelope(payload []byte, scope Scope, expectedProjectID string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := validateOpaqueProjectID(expectedProjectID); err != nil {
		return err
	}
	if !utf8.Valid(payload) {
		return fmt.Errorf("projects-v1: work item response is not valid UTF-8")
	}
	return rejectDuplicateJSONKeys(payload)
}

func validateWorkItemPageShape(payload []byte) error {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(payload, &shape); err != nil {
		return fmt.Errorf("projects-v1: decode work item page shape: %w", err)
	}
	if err := requireExactProjectFields(shape, "items", "next_cursor", "total_count", "undated_count"); err != nil {
		return err
	}
	if err := validateWorkItemPageRequiredFields(shape); err != nil {
		return err
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(shape["items"], &rawItems); err != nil {
		return fmt.Errorf("projects-v1: work item items must be an array")
	}
	return validateWorkItemShapes(rawItems)
}

func validateWorkItemPageRequiredFields(shape map[string]json.RawMessage) error {
	if raw, ok := shape["items"]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("projects-v1: work item items must be present and non-null")
	}
	if _, ok := shape["next_cursor"]; !ok {
		return fmt.Errorf("projects-v1: work item next_cursor must be present")
	}
	if raw, ok := shape["total_count"]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("projects-v1: work item total_count must be present and non-null")
	}
	if raw, ok := shape["undated_count"]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("projects-v1: work item undated_count must be present and non-null")
	}
	return nil
}

func validateWorkItemShapes(rawItems []json.RawMessage) error {
	required := []string{"work_item_id", "project_id", "title", "description", "status", "rank", "version", "created_at", "updated_at"}
	for index, raw := range rawItems {
		if err := validateWorkItemShape(raw, required); err != nil {
			return fmt.Errorf("projects-v1: work item %d: %w", index, err)
		}
	}
	return nil
}

func validateWorkItemShape(payload json.RawMessage, required []string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("must be an object")
	}
	if len(fields) != len(required) {
		v2 := append(append([]string{}, required...), "assignee_user_id", "assignee_status_snapshot", "assignee_display_snapshot", "priority", "start_at", "due_at", "labels")
		if err := requireExactProjectFields(fields, v2...); err != nil {
			return err
		}
	} else if err := requireExactProjectFields(fields, required...); err != nil {
		return err
	}
	for _, field := range required {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("field %s must be present", field)
		}
	}
	if bytes.Equal(bytes.TrimSpace(fields["rank"]), []byte("null")) {
		return fmt.Errorf("rank must be non-null")
	}
	return nil
}

func decodeWorkItemPageWire(payload []byte) (workItemPageWire, error) {
	var wire workItemPageWire
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return workItemPageWire{}, fmt.Errorf("projects-v1: decode work item page: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return workItemPageWire{}, err
	}
	return wire, nil
}

func workItemWiresDomain(wires []workItemWire, expectedProjectID string) ([]WorkItem, error) {
	items := make([]WorkItem, len(wires))
	seen := make(map[string]struct{}, len(wires))
	for index, itemWire := range wires {
		item, err := itemWire.domain(expectedProjectID)
		if err != nil {
			return nil, fmt.Errorf("projects-v1: work item %d: %w", index, err)
		}
		if _, ok := seen[item.WorkItemID]; ok {
			return nil, fmt.Errorf("projects-v1: duplicate work_item_id")
		}
		seen[item.WorkItemID] = struct{}{}
		items[index] = item
	}
	return items, nil
}

func validateWorkItemNextCursor(cursor *string) error {
	if cursor != nil && (*cursor == "" || strings.TrimSpace(*cursor) != *cursor || len([]byte(*cursor)) > 2048) {
		return fmt.Errorf("projects-v1: invalid work item cursor")
	}
	return nil
}

func DecodeWorkItem(payload []byte, scope Scope, expectedProjectID, expectedWorkItemID string) (WorkItem, error) {
	wrapped := append([]byte(`{"items":[`), payload...)
	wrapped = append(wrapped, []byte(`],"next_cursor":null,"total_count":1,"undated_count":0}`)...)
	page, err := DecodeWorkItemPage(wrapped, scope, expectedProjectID)
	if err != nil {
		return WorkItem{}, err
	}
	if len(page.Items) != 1 || page.Items[0].WorkItemID != expectedWorkItemID {
		return WorkItem{}, fmt.Errorf("projects-v1: work item response ID does not match request")
	}
	return page.Items[0], nil
}

func (wire workItemWire) domain(expectedProjectID string) (WorkItem, error) {
	if err := wire.validateIdentity(expectedProjectID); err != nil {
		return WorkItem{}, err
	}
	if err := wire.validateText(); err != nil {
		return WorkItem{}, err
	}
	status, err := wire.validateState()
	if err != nil {
		return WorkItem{}, err
	}
	if err := validateWorkItemTimestamps(wire.CreatedAt, wire.UpdatedAt); err != nil {
		return WorkItem{}, err
	}
	labels, err := projectLabelsFromWire(wire.Labels, expectedProjectID)
	if err != nil {
		return WorkItem{}, err
	}
	return WorkItem{WorkItemID: wire.WorkItemID, ProjectID: wire.ProjectID, Title: wire.Title, Description: cloneString(wire.Description), Status: status, Rank: wire.Rank, Version: wire.Version, CreatedAt: wire.CreatedAt, UpdatedAt: wire.UpdatedAt,
		AssigneeUserID: cloneString(wire.AssigneeUserID), AssigneeStatusSnapshot: cloneString(wire.AssigneeStatusSnapshot), AssigneeDisplaySnapshot: cloneString(wire.AssigneeDisplaySnapshot), Priority: wire.Priority, StartAt: cloneTime(wire.StartAt), DueAt: cloneTime(wire.DueAt), Labels: labels}, nil
}

func projectLabelsFromWire(wireLabels []projectLabelWire, expectedProjectID string) ([]ProjectLabel, error) {
	labels := make([]ProjectLabel, len(wireLabels))
	for index, label := range wireLabels {
		if label.ProjectID != expectedProjectID || label.LabelID == "" || label.Name == "" || label.Version == 0 || label.CreatedAt.IsZero() || label.UpdatedAt.IsZero() {
			return nil, fmt.Errorf("invalid work item label")
		}
		labels[index] = ProjectLabel{LabelID: label.LabelID, ProjectID: label.ProjectID, Name: label.Name, Color: label.Color, Version: label.Version, CreatedAt: label.CreatedAt, UpdatedAt: label.UpdatedAt}
	}
	return labels, nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (wire workItemWire) validateIdentity(expectedProjectID string) error {
	if err := validateOpaqueProjectID(wire.WorkItemID); err != nil {
		return fmt.Errorf("invalid work_item_id")
	}
	if wire.ProjectID != expectedProjectID {
		return fmt.Errorf("work item project ID mismatch")
	}
	return nil
}

func (wire workItemWire) validateText() error {
	if strings.TrimSpace(wire.Title) == "" || !validProjectText(wire.Title, 300) {
		return fmt.Errorf("invalid work item title")
	}
	if wire.Description != nil && !validProjectText(*wire.Description, 8000) {
		return fmt.Errorf("invalid work item description")
	}
	return nil
}

func (wire workItemWire) validateState() (WorkItemStatus, error) {
	status := WorkItemStatus(wire.Status)
	if !validWorkItemStatus(status) {
		return "", fmt.Errorf("invalid work item status")
	}
	if wire.Rank < -int64(MaxSafeJSONInteger) || wire.Rank > int64(MaxSafeJSONInteger) {
		return "", fmt.Errorf("invalid work item rank")
	}
	if wire.Version == 0 || wire.Version > MaxSafeJSONInteger {
		return "", fmt.Errorf("invalid work item version")
	}
	return status, nil
}

func validateWorkItemTimestamps(createdAt, updatedAt time.Time) error {
	if createdAt.IsZero() || updatedAt.IsZero() {
		return fmt.Errorf("invalid work item timestamps")
	}
	if !isUTCProjectTimestamp(createdAt) || !isUTCProjectTimestamp(updatedAt) {
		return fmt.Errorf("invalid work item timestamps")
	}
	if updatedAt.Before(createdAt) {
		return fmt.Errorf("invalid work item timestamps")
	}
	return nil
}

func isUTCProjectTimestamp(value time.Time) bool {
	_, offset := value.Zone()
	return offset == 0 && value.Location() == time.UTC
}

func validWorkItemStatus(status WorkItemStatus) bool {
	return status == WorkItemTodo || status == WorkItemInProgress || status == WorkItemPaused || status == WorkItemDone
}
