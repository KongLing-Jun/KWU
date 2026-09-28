// Package scene defines the Desktop conversation scene contract.
package scene

import "fmt"

// Scene identifies the work emphasis injected into a Desktop system prompt.
// Unset is internal-only and preserves the pre-scene prompt for hosts and
// legacy sessions that have not adopted Desktop scenes.
type Scene string

const (
	Unset  Scene = ""
	Office Scene = "office"
	Coding Scene = "coding"
	Design Scene = "design"
)

// State is the persisted lifecycle for a Desktop scene. Legacy is runtime-only:
// old metadata omits both scene fields and must never be rewritten as a fourth
// public scene. The zero state belongs to hosts that have not adopted scenes.
type State string

const (
	StateUnset State = ""
	Draft      State = "draft"
	Locked     State = "locked"
	Legacy     State = "legacy"
)

// Status pairs a scene with its lifecycle state.
type Status struct {
	Scene Scene
	State State
}

const (
	OfficePromptFragment = "Scene: Office. Produce concise, directly usable documents, spreadsheets, communications, and organized information with minimal disruption. Preserve source facts, reconcile conflicts, distinguish facts from assumptions, verify calculations and labels, and confirm the requested file contents and paths before delivery."
	CodingPromptFragment = "Scene: Coding. Understand the relevant code, contracts, and behavior that must be preserved before editing. Make the smallest coherent change. Validate the requested behavior, not merely command success, with focused tests that cover the critical normal path, boundaries, and failure cases; read the actual results and inspect regression risk before delivery."
	DesignPromptFragment = "Scene: Design. Clarify the audience and goal, then produce a concrete, locally inspectable design artifact using capabilities actually available. Verify file validity, render-relevant structure, information hierarchy, responsive behavior, and requested accessibility states such as visible keyboard focus; the presence of an attribute alone is not proof that a state is visible. Use a real preview or render when available, otherwise state the validation limit. Never claim access to canvases, layers, cloud sync, exports, or generated files unless that action actually occurred."
)

// Parse accepts only the three exact user-facing protocol values.
func Parse(value string) (Scene, error) {
	s := Scene(value)
	if s.IsSelectable() {
		return s, nil
	}
	return Unset, fmt.Errorf("invalid scene %q", value)
}

// IsSelectable reports whether s is one of the three public Desktop scenes.
func (s Scene) IsSelectable() bool {
	switch s {
	case Office, Coding, Design:
		return true
	default:
		return false
	}
}

// Fragment returns the frozen prompt fragment for s. Unset intentionally
// returns an empty fragment; every other unknown value is rejected.
func Fragment(s Scene) (string, error) {
	switch s {
	case Unset:
		return "", nil
	case Office:
		return OfficePromptFragment, nil
	case Coding:
		return CodingPromptFragment, nil
	case Design:
		return DesignPromptFragment, nil
	default:
		return "", fmt.Errorf("invalid scene %q", s)
	}
}

// NewStatus validates a runtime status without normalizing unknown values.
func NewStatus(s Scene, state State) (Status, error) {
	status := Status{Scene: s, State: state}
	if err := status.Validate(); err != nil {
		return Status{}, err
	}
	return status, nil
}

// RestoreStatus decodes optional persisted values. Missing values are legacy;
// partial or invalid values return legacy plus an error for diagnostics.
func RestoreStatus(sceneValue, stateValue string) (Status, error) {
	if sceneValue == "" && stateValue == "" {
		return Status{Scene: Unset, State: Legacy}, nil
	}
	s, err := Parse(sceneValue)
	if err != nil {
		return Status{Scene: Unset, State: Legacy}, err
	}
	state := State(stateValue)
	if state != Draft && state != Locked {
		return Status{Scene: Unset, State: Legacy}, fmt.Errorf("invalid scene state %q", stateValue)
	}
	return Status{Scene: s, State: state}, nil
}

func (s Status) Validate() error {
	switch s.State {
	case StateUnset, Legacy:
		if s.Scene == Unset {
			return nil
		}
	case Draft, Locked:
		if s.Scene.IsSelectable() {
			return nil
		}
	}
	return fmt.Errorf("invalid scene status %q/%q", s.Scene, s.State)
}

func (s Status) Persisted() (sceneValue, stateValue string, ok bool) {
	if s.Validate() != nil || (s.State != Draft && s.State != Locked) {
		return "", "", false
	}
	return string(s.Scene), string(s.State), true
}

func (s Status) IsLegacy() bool { return s.Scene == Unset && s.State == Legacy }
