package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type commandResultWire struct {
	OperationID      string `json:"operation_id"`
	ResourceKind     string `json:"resource_kind"`
	ResourceID       string `json:"resource_id"`
	ProjectID        string `json:"project_id"`
	CommittedVersion uint64 `json:"committed_version"`
}

type projectOnboardingClaimWire struct {
	Project      json.RawMessage `json:"project"`
	GuideVersion uint64          `json:"guide_version"`
	State        string          `json:"state"`
	Created      bool            `json:"created"`
}

// DecodeProjectOnboardingClaim strictly validates the Cloud extension so a
// malformed guide response cannot be merged into the normal project list.
func DecodeProjectOnboardingClaim(payload []byte, scope Scope) (ProjectOnboardingClaim, error) {
	if err := scope.Validate(); err != nil {
		return ProjectOnboardingClaim{}, err
	}
	if err := validateProjectOnboardingClaimShape(payload); err != nil {
		return ProjectOnboardingClaim{}, err
	}
	wire, err := decodeProjectOnboardingClaimWire(payload)
	if err != nil {
		return ProjectOnboardingClaim{}, err
	}
	state, err := validateProjectOnboardingClaimWire(wire)
	if err != nil {
		return ProjectOnboardingClaim{}, err
	}
	result := ProjectOnboardingClaim{ProjectScopeKey: scope.Key, GuideVersion: wire.GuideVersion, State: state, Created: wire.Created}
	if err := attachOnboardingProject(&result, wire.Project, scope); err != nil {
		return ProjectOnboardingClaim{}, err
	}
	return result, nil
}

func validateProjectOnboardingClaimShape(payload []byte) error {
	if !utf8.Valid(payload) {
		return fmt.Errorf("projects-v1: onboarding claim is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("projects-v1: decode onboarding claim shape: %w", err)
	}
	if err := requireExactProjectFields(fields, "project", "guide_version", "state", "created"); err != nil {
		return fmt.Errorf("projects-v1: onboarding claim: %w", err)
	}
	for _, field := range [...]string{"project", "guide_version", "state", "created"} {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("projects-v1: onboarding claim field %s must be present", field)
		}
	}
	return nil
}

func decodeProjectOnboardingClaimWire(payload []byte) (projectOnboardingClaimWire, error) {
	var wire projectOnboardingClaimWire
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return projectOnboardingClaimWire{}, fmt.Errorf("projects-v1: decode onboarding claim: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return projectOnboardingClaimWire{}, err
	}
	return wire, nil
}

func validateProjectOnboardingClaimWire(wire projectOnboardingClaimWire) (ProjectOnboardingState, error) {
	state := ProjectOnboardingState(wire.State)
	if wire.GuideVersion == 0 || wire.GuideVersion > MaxSafeJSONInteger || !validProjectOnboardingState(state) {
		return "", fmt.Errorf("projects-v1: invalid onboarding claim state")
	}
	if string(wire.Project) == "null" {
		if state == ProjectOnboardingStateActive || wire.Created {
			return "", fmt.Errorf("projects-v1: active onboarding claim requires a project")
		}
		return state, nil
	}
	if state != ProjectOnboardingStateActive {
		return "", fmt.Errorf("projects-v1: inactive onboarding claim must not include a project")
	}
	return state, nil
}

func attachOnboardingProject(result *ProjectOnboardingClaim, raw json.RawMessage, scope Scope) error {
	if string(raw) == "null" {
		return nil
	}
	project, err := DecodeProject(raw, scope)
	if err != nil {
		return err
	}
	result.Project = &project
	return nil
}

func validProjectOnboardingState(state ProjectOnboardingState) bool {
	return state == ProjectOnboardingStateActive || state == ProjectOnboardingStateCompleted || state == ProjectOnboardingStateDismissed || state == ProjectOnboardingStateDeleted
}

// DecodeProject strictly decodes one Project response for the supplied scope.
func DecodeProject(payload []byte, scope Scope) (Project, error) {
	wrapped := make([]byte, 0, len(payload)+31)
	wrapped = append(wrapped, `{"items":[`...)
	wrapped = append(wrapped, payload...)
	wrapped = append(wrapped, `],"next_cursor":null}`...)
	page, err := DecodeProjectPage(wrapped, scope)
	if err != nil {
		return Project{}, err
	}
	if len(page.Items) != 1 {
		return Project{}, fmt.Errorf("projects-v1: project response must contain one project")
	}
	return page.Items[0], nil
}

// DecodeCommandResult strictly decodes a Project mutation receipt and stamps
// it with the scope that produced it.
func DecodeCommandResult(payload []byte, scope Scope, expectedProjectID string) (CommandResult, error) {
	return decodeCommandResult(payload, scope, expectedProjectID, "project")
}

func DecodeInstructionCommandResult(payload []byte, scope Scope, expectedProjectID string) (CommandResult, error) {
	return decodeCommandResult(payload, scope, expectedProjectID, "instruction")
}

func decodeCommandResult(payload []byte, scope Scope, expectedProjectID, expectedResourceKind string) (CommandResult, error) {
	if err := scope.Validate(); err != nil {
		return CommandResult{}, err
	}
	if err := validateCommandResultShape(payload); err != nil {
		return CommandResult{}, err
	}
	wire, err := decodeCommandResultWire(payload)
	if err != nil {
		return CommandResult{}, err
	}
	if err := validateCommandResultWire(wire, expectedProjectID, expectedResourceKind); err != nil {
		return CommandResult{}, err
	}
	return CommandResult{
		ProjectScopeKey: scope.Key,
		OperationID:     wire.OperationID, ResourceKind: wire.ResourceKind,
		ResourceID: wire.ResourceID, ProjectID: wire.ProjectID,
		CommittedVersion: wire.CommittedVersion,
	}, nil
}

func validateCommandResultShape(payload []byte) error {
	if !utf8.Valid(payload) {
		return fmt.Errorf("projects-v1: command result is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("projects-v1: decode command result shape: %w", err)
	}
	if err := requireExactProjectFields(fields, "operation_id", "resource_kind", "resource_id", "project_id", "committed_version"); err != nil {
		return fmt.Errorf("projects-v1: command result: %w", err)
	}
	for _, field := range [...]string{"operation_id", "resource_kind", "resource_id", "project_id", "committed_version"} {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("projects-v1: command result field %s must be present", field)
		}
	}
	return nil
}

func decodeCommandResultWire(payload []byte) (commandResultWire, error) {
	var wire commandResultWire
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return commandResultWire{}, fmt.Errorf("projects-v1: decode command result: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return commandResultWire{}, err
	}
	return wire, nil
}

func validateCommandResultWire(wire commandResultWire, expectedProjectID, expectedResourceKind string) error {
	if strings.TrimSpace(wire.OperationID) != wire.OperationID || wire.OperationID == "" || !validProjectText(wire.OperationID, len([]byte(wire.OperationID))) {
		return fmt.Errorf("projects-v1: invalid operation_id")
	}
	if wire.ResourceKind != expectedResourceKind {
		return fmt.Errorf("projects-v1: invalid resource_kind")
	}
	if err := validateOpaqueProjectID(wire.ResourceID); err != nil {
		return fmt.Errorf("projects-v1: invalid resource_id")
	}
	if err := validateOpaqueProjectID(wire.ProjectID); err != nil {
		return err
	}
	if err := validateCommandResultRoute(wire, expectedProjectID, expectedResourceKind); err != nil {
		return err
	}
	if wire.CommittedVersion == 0 || wire.CommittedVersion > MaxSafeJSONInteger {
		return fmt.Errorf("projects-v1: invalid committed_version")
	}
	return nil
}

func validateCommandResultRoute(wire commandResultWire, expectedProjectID, expectedResourceKind string) error {
	if (expectedResourceKind == "project" && wire.ResourceID != wire.ProjectID) ||
		(expectedResourceKind == "instruction" && (wire.ResourceID != wire.ProjectID || wire.ProjectID != expectedProjectID)) ||
		(expectedProjectID != "" && wire.ProjectID != expectedProjectID) {
		return fmt.Errorf("projects-v1: command result project ID mismatch")
	}
	return nil
}
