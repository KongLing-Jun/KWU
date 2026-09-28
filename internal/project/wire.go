package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

type projectPageWire struct {
	Items      []projectWire `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

type projectWire struct {
	ProjectID   string    `json:"project_id"`
	OwnerUserID string    `json:"owner_user_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	Version     uint64    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DecodeProjectPage strictly decodes a list response for the supplied scope.
// Cloud authorizes both owned and joined projects; the opaque scope still
// prevents a late response from crossing account boundaries.
func DecodeProjectPage(payload []byte, scope Scope) (ProjectPage, error) {
	if err := scope.Validate(); err != nil {
		return ProjectPage{}, err
	}
	if !utf8.Valid(payload) {
		return ProjectPage{}, fmt.Errorf("projects-v1: response is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return ProjectPage{}, err
	}
	if err := validateProjectPageShape(payload); err != nil {
		return ProjectPage{}, err
	}
	wire, err := decodeProjectPageWire(payload)
	if err != nil {
		return ProjectPage{}, err
	}
	items, err := projectWireItemsDomain(wire.Items, scope)
	if err != nil {
		return ProjectPage{}, err
	}
	if err := validateProjectNextCursor(wire.NextCursor); err != nil {
		return ProjectPage{}, err
	}
	return ProjectPage{Items: items, NextCursor: cloneString(wire.NextCursor), scopeKey: scope.Key}, nil
}

func decodeProjectPageWire(payload []byte) (projectPageWire, error) {
	var wire projectPageWire
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return projectPageWire{}, fmt.Errorf("projects-v1: decode project page: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return projectPageWire{}, err
	}
	return wire, nil
}

func projectWireItemsDomain(wires []projectWire, scope Scope) ([]Project, error) {
	items := make([]Project, len(wires))
	seenIDs := make(map[string]struct{}, len(wires))
	for i, item := range wires {
		project, err := item.domain(scope)
		if err != nil {
			return nil, fmt.Errorf("projects-v1: item %d: %w", i, err)
		}
		if _, exists := seenIDs[project.ProjectID]; exists {
			return nil, fmt.Errorf("projects-v1: duplicate project_id %q", project.ProjectID)
		}
		seenIDs[project.ProjectID] = struct{}{}
		items[i] = project
	}
	return items, nil
}

func validateProjectNextCursor(cursor *string) error {
	if cursor != nil && (*cursor == "" || strings.TrimSpace(*cursor) != *cursor || len([]byte(*cursor)) > 2048) {
		return fmt.Errorf("projects-v1: invalid next cursor")
	}
	return nil
}

func validateProjectPageShape(payload []byte) error {
	var page map[string]json.RawMessage
	if err := json.Unmarshal(payload, &page); err != nil {
		return fmt.Errorf("projects-v1: decode project page shape: %w", err)
	}
	if err := requireExactProjectFields(page, "items", "next_cursor"); err != nil {
		return err
	}
	itemsPayload, ok := page["items"]
	if !ok || bytes.Equal(bytes.TrimSpace(itemsPayload), []byte("null")) {
		return fmt.Errorf("projects-v1: items must be present and non-null")
	}
	if _, ok := page["next_cursor"]; !ok {
		return fmt.Errorf("projects-v1: next_cursor must be present")
	}
	var items []json.RawMessage
	if err := json.Unmarshal(itemsPayload, &items); err != nil {
		return fmt.Errorf("projects-v1: items must be an array")
	}
	required := [...]string{
		"project_id", "owner_user_id", "name", "description",
		"status", "version", "created_at", "updated_at",
	}
	for i, itemPayload := range items {
		if err := validateProjectItemShape(itemPayload, required[:]); err != nil {
			return fmt.Errorf("projects-v1: item %d: %w", i, err)
		}
	}
	return nil
}

func validateProjectItemShape(payload []byte, required []string) error {
	var item map[string]json.RawMessage
	if err := json.Unmarshal(payload, &item); err != nil {
		return fmt.Errorf("must be an object")
	}
	if err := requireExactProjectFields(item, required...); err != nil {
		return err
	}
	for _, field := range required {
		if _, ok := item[field]; !ok {
			return fmt.Errorf("field %s must be present", field)
		}
	}
	return nil
}

func (w projectWire) domain(scope Scope) (Project, error) {
	if err := w.validateIdentity(scope); err != nil {
		return Project{}, err
	}
	status, err := w.validateMutableFields()
	if err != nil {
		return Project{}, err
	}
	if err := validateProjectTimestamps(w.CreatedAt, w.UpdatedAt); err != nil {
		return Project{}, err
	}
	return Project{
		ProjectID: w.ProjectID, OwnerUserID: w.OwnerUserID, Name: w.Name,
		Description: cloneString(w.Description), Status: status, Version: w.Version,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}, nil
}

func (w projectWire) validateIdentity(scope Scope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := validateOpaqueProjectID(w.ProjectID); err != nil {
		return err
	}
	if err := validateOpaqueProjectID(w.OwnerUserID); err != nil {
		return fmt.Errorf("invalid project owner: %w", err)
	}
	return nil
}

func (w projectWire) validateMutableFields() (ProjectStatus, error) {
	if strings.TrimSpace(w.Name) == "" || !validProjectText(w.Name, 200) {
		return "", fmt.Errorf("invalid project name")
	}
	if w.Description != nil && !validProjectText(*w.Description, 4000) {
		return "", fmt.Errorf("invalid project description")
	}
	status := ProjectStatus(w.Status)
	if status != ProjectActive && status != ProjectArchived {
		return "", fmt.Errorf("invalid project status %q", w.Status)
	}
	if w.Version == 0 || w.Version > MaxSafeJSONInteger {
		return "", fmt.Errorf("invalid project version")
	}
	return status, nil
}

func validateProjectTimestamps(createdAt, updatedAt time.Time) error {
	if createdAt.IsZero() || updatedAt.IsZero() {
		return fmt.Errorf("invalid project timestamp")
	}
	_, createdOffset := createdAt.Zone()
	_, updatedOffset := updatedAt.Zone()
	if createdOffset != 0 || updatedOffset != 0 || createdAt.Location() != time.UTC || updatedAt.Location() != time.UTC {
		return fmt.Errorf("project timestamps must use UTC")
	}
	if updatedAt.Before(createdAt) {
		return fmt.Errorf("invalid project timestamp chronology")
	}
	return nil
}

func validateOpaqueProjectID(value string) error {
	if value == "" || strings.TrimSpace(value) != value || !validProjectText(value, len([]byte(value))) {
		return fmt.Errorf("invalid project_id")
	}
	return nil
}

func requireExactProjectFields(fields map[string]json.RawMessage, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		set[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := set[field]; !ok {
			return fmt.Errorf("unknown field %q", field)
		}
	}
	return nil
}

func rejectDuplicateJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := scanProjectJSONValue(decoder); err != nil {
		return fmt.Errorf("projects-v1: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("projects-v1: multiple JSON values")
		}
		return fmt.Errorf("projects-v1: decode trailing JSON: %w", err)
	}
	return nil
}

func scanProjectJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		if err := scanProjectJSONObject(decoder); err != nil {
			return err
		}
	case '[':
		if err := scanProjectJSONArray(decoder); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	_, err = decoder.Token()
	return err
}

func scanProjectJSONObject(decoder *json.Decoder) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("object key is not a string")
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate JSON key %q", key)
		}
		seen[key] = struct{}{}
		if err := scanProjectJSONValue(decoder); err != nil {
			return err
		}
	}
	return nil
}

func scanProjectJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		if err := scanProjectJSONValue(decoder); err != nil {
			return err
		}
	}
	return nil
}

func requireProjectJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("projects-v1: multiple JSON values")
		}
		return fmt.Errorf("projects-v1: decode trailing JSON: %w", err)
	}
	return nil
}
