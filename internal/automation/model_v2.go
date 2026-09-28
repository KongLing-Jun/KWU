package automation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const desktopBotDeliveryTargetRefPrefix = "bot_target_v1_"

const CurrentTaskSchemaVersion = 2

// MissedRunPolicy controls what happens when the desktop was offline at a
// scheduled occurrence.
type MissedRunPolicy string

const (
	MissedRunSkip       MissedRunPolicy = "skip"
	MissedRunLatestOnce MissedRunPolicy = "latest_once"
	MissedRunCatchUp    MissedRunPolicy = "catch_up"
)

type EffectiveRange struct {
	From  time.Time `json:"from,omitempty"`
	Until time.Time `json:"until,omitempty"`
}

// CapabilitySelection stores references only. Credentials remain owned by the
// existing connector and provider configuration stores.
type CapabilitySelection struct {
	ModelRef     string   `json:"modelRef,omitempty"`
	SkillIDs     []string `json:"skillIds,omitempty"`
	ExpertID     string   `json:"expertId,omitempty"`
	ConnectorIDs []string `json:"connectorIds,omitempty"`
}

type CapabilitySnapshot struct {
	ModelRef        string                   `json:"modelRef,omitempty"`
	ApprovalMode    ApprovalMode             `json:"approvalMode,omitempty"`
	SkillIDs        []string                 `json:"skillIds,omitempty"`
	ExpertID        string                   `json:"expertId,omitempty"`
	ConnectorIDs    []string                 `json:"connectorIds,omitempty"`
	ToolAllowlist   []string                 `json:"toolAllowlist,omitempty"`
	PromptHash      string                   `json:"promptHash,omitempty"`
	Skills          []CapabilityArtifact     `json:"skills,omitempty"`
	Expert          *CapabilityArtifact      `json:"expert,omitempty"`
	ConnectorGrants []ConnectorGrantSnapshot `json:"connectorGrants,omitempty"`
	Principal       string                   `json:"principal,omitempty"`
	RiskSummary     string                   `json:"riskSummary,omitempty"`
	CapturedAt      time.Time                `json:"capturedAt,omitempty"`
}

// CapabilityArtifact identifies the exact skill or expert definition used by
// a run. At least one of Version or ContentHash is required by strict capture.
type CapabilityArtifact struct {
	ID          string `json:"id"`
	Version     string `json:"version,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
}

// ConnectorGrantSnapshot contains only auditable authorization metadata. It
// never stores connector credentials or secret configuration values.
type ConnectorGrantSnapshot struct {
	ConnectorID   string    `json:"connectorId"`
	ServerID      string    `json:"serverId"`
	ToolAllowlist []string  `json:"toolAllowlist,omitempty"`
	GrantID       string    `json:"grantId"`
	Principal     string    `json:"principal"`
	Source        string    `json:"source"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

type DeliveryPolicy struct {
	Enabled bool             `json:"enabled"`
	Targets []DeliveryTarget `json:"targets,omitempty"`
}

type DeliveryTarget struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

func normalizeMissedRunPolicy(policy MissedRunPolicy) MissedRunPolicy {
	policy = MissedRunPolicy(strings.ToLower(strings.TrimSpace(string(policy))))
	if policy == "" {
		return MissedRunLatestOnce
	}
	return policy
}

func validMissedRunPolicy(policy MissedRunPolicy) bool {
	switch MissedRunPolicy(strings.ToLower(strings.TrimSpace(string(policy)))) {
	case "", MissedRunSkip, MissedRunLatestOnce, MissedRunCatchUp:
		return true
	default:
		return false
	}
}

func normalizeCapabilitySelection(selection CapabilitySelection, prompt PromptConfig) CapabilitySelection {
	selection.ModelRef = strings.TrimSpace(selection.ModelRef)
	if selection.ModelRef == "" {
		selection.ModelRef = strings.TrimSpace(prompt.Model)
	}
	selection.ExpertID = strings.TrimSpace(selection.ExpertID)
	selection.SkillIDs = uniqueTrimmedStrings(selection.SkillIDs)
	if len(selection.SkillIDs) == 0 {
		selection.SkillIDs = uniqueTrimmedStrings(prompt.Skills)
	}
	selection.ConnectorIDs = uniqueTrimmedStrings(selection.ConnectorIDs)
	return selection
}

func validateCapabilitySelection(selection CapabilitySelection) error {
	for _, values := range [][]string{selection.SkillIDs, selection.ConnectorIDs} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%w: capability references must not be empty", ErrInvalidTask)
			}
		}
	}
	return nil
}

func normalizeDeliveryPolicy(policy DeliveryPolicy) DeliveryPolicy {
	seen := make(map[string]struct{}, len(policy.Targets))
	targets := make([]DeliveryTarget, 0, len(policy.Targets))
	for _, target := range policy.Targets {
		target.Type = strings.ToLower(strings.TrimSpace(target.Type))
		target.Ref = strings.TrimSpace(target.Ref)
		if target.Type == "" && target.Ref == "" {
			continue
		}
		key := target.Type + "\x00" + target.Ref
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, target)
	}
	policy.Targets = targets
	return policy
}

func validateDeliveryPolicy(policy DeliveryPolicy) error {
	if policy.Enabled && len(policy.Targets) == 0 {
		return fmt.Errorf("%w: enabled delivery requires at least one target", ErrInvalidTask)
	}
	for _, target := range policy.Targets {
		if strings.TrimSpace(target.Type) == "" || strings.TrimSpace(target.Ref) == "" {
			return fmt.Errorf("%w: delivery target type and ref are required", ErrInvalidTask)
		}
		if !deliveryTargetAllowsPlainTestRef(target.Type) && !validDesktopBotDeliveryTargetRef(target.Ref) {
			return fmt.Errorf("%w: %s delivery target must use an opaque reference", ErrInvalidTask, target.Type)
		}
	}
	return nil
}

func deliveryTargetAllowsPlainTestRef(targetType string) bool {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "local_test", "retry_test":
		return true
	default:
		return false
	}
}

func validDesktopBotDeliveryTargetRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, desktopBotDeliveryTargetRefPrefix) {
		return false
	}
	digest, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ref, desktopBotDeliveryTargetRefPrefix))
	return err == nil && len(digest) == 32
}

func uniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

type taskJSON Task

func (t *Task) UnmarshalJSON(data []byte) error {
	var decoded taskJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range knownTaskJSONFields {
		delete(fields, name)
	}
	*t = Task(decoded)
	if len(fields) > 0 {
		t.UnknownFields = fields
	}
	return nil
}

func (t Task) MarshalJSON() ([]byte, error) {
	known, err := json.Marshal(taskJSON(t))
	if err != nil {
		return nil, err
	}
	if len(t.UnknownFields) == 0 {
		return known, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(known, &fields); err != nil {
		return nil, err
	}
	for name, value := range t.UnknownFields {
		if _, exists := fields[name]; exists || len(value) == 0 || !json.Valid(value) {
			continue
		}
		fields[name] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(fields)
}

var knownTaskJSONFields = []string{
	"schemaVersion", "revision", "id", "name", "enabled", "scope",
	"projectId", "workspaceRoot", "topicId", "newConversationEachRun", "prompt", "schedule",
	"effectiveRange", "missedRunPolicy", "capabilities", "delivery", "conditions",
	"execution", "archive", "businessDiagnostics", "createdAt", "updatedAt",
	"lastRunAt", "lastRunId",
}
