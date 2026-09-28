package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"
)

// ProjectInstruction is one authoritative online Instruction snapshot.
// It is intentionally not part of ProjectCache.
type ProjectInstruction struct {
	ProjectID        string
	HumanDescription string
	AgentInstruction string
	ContentHash      string
	Version          uint64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type InstructionResult struct {
	ProjectScopeKey string
	Instruction     ProjectInstruction
}

type PutProjectInstructionInput struct {
	ExpectedVersion  uint64
	HumanDescription string
	AgentInstruction string
}

type projectInstructionWire struct {
	ProjectID        string    `json:"project_id"`
	HumanDescription string    `json:"human_description"`
	AgentInstruction string    `json:"agent_instruction"`
	ContentHash      string    `json:"content_hash"`
	Version          uint64    `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DecodeProjectInstruction strictly validates an authoritative Instruction
// response, including its exact body-derived projects-v1 content hash.
func DecodeProjectInstruction(payload []byte, scope Scope, expectedProjectID string) (ProjectInstruction, error) {
	if err := scope.Validate(); err != nil {
		return ProjectInstruction{}, err
	}
	if err := validateProjectInstructionShape(payload); err != nil {
		return ProjectInstruction{}, err
	}
	wire, err := decodeProjectInstructionWire(payload)
	if err != nil {
		return ProjectInstruction{}, err
	}
	if err := validateProjectInstructionWire(wire, expectedProjectID); err != nil {
		return ProjectInstruction{}, err
	}
	return ProjectInstruction{
		ProjectID: wire.ProjectID, HumanDescription: wire.HumanDescription,
		AgentInstruction: wire.AgentInstruction, ContentHash: wire.ContentHash,
		Version: wire.Version, CreatedAt: wire.CreatedAt, UpdatedAt: wire.UpdatedAt,
	}, nil
}

func validateProjectInstructionShape(payload []byte) error {
	if !utf8.Valid(payload) {
		return fmt.Errorf("projects-v1: instruction response is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return fmt.Errorf("projects-v1: decode instruction shape: %w", err)
	}
	required := []string{"project_id", "human_description", "agent_instruction", "content_hash", "version", "created_at", "updated_at"}
	if err := requireExactProjectFields(fields, required...); err != nil {
		return fmt.Errorf("projects-v1: instruction response: %w", err)
	}
	for _, field := range required {
		if value, ok := fields[field]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("projects-v1: instruction field %s must be present and non-null", field)
		}
	}
	return nil
}

func decodeProjectInstructionWire(payload []byte) (projectInstructionWire, error) {
	var wire projectInstructionWire
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return projectInstructionWire{}, fmt.Errorf("projects-v1: decode instruction: %w", err)
	}
	if err := requireProjectJSONEOF(decoder); err != nil {
		return projectInstructionWire{}, err
	}
	return wire, nil
}

func validateProjectInstructionWire(wire projectInstructionWire, expectedProjectID string) error {
	if err := validateOpaqueProjectID(wire.ProjectID); err != nil || wire.ProjectID != expectedProjectID {
		return fmt.Errorf("projects-v1: instruction project ID mismatch")
	}
	if !validProjectText(wire.HumanDescription, 12_000) || !validProjectText(wire.AgentInstruction, 48_000) {
		return fmt.Errorf("projects-v1: invalid instruction text")
	}
	if wire.Version == 0 || wire.Version > MaxSafeJSONInteger {
		return fmt.Errorf("projects-v1: invalid instruction version")
	}
	if err := validateProjectInstructionTimestamps(wire.CreatedAt, wire.UpdatedAt); err != nil {
		return err
	}
	if wire.ContentHash != InstructionContentSHA256(wire.HumanDescription, wire.AgentInstruction) {
		return fmt.Errorf("projects-v1: instruction content hash mismatch")
	}
	return nil
}

func validateProjectInstructionTimestamps(createdAt, updatedAt time.Time) error {
	if createdAt.IsZero() || updatedAt.IsZero() {
		return fmt.Errorf("projects-v1: instruction timestamps must use UTC")
	}
	if updatedAt.Before(createdAt) {
		return fmt.Errorf("projects-v1: instruction timestamps must use UTC")
	}
	if createdAt.Location() != time.UTC || updatedAt.Location() != time.UTC {
		return fmt.Errorf("projects-v1: instruction timestamps must use UTC")
	}
	return nil
}
