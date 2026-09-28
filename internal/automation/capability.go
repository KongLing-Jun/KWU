package automation

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrCapabilityUnavailable marks a selected runtime capability that can no
// longer be resolved. Callers should block the run instead of silently
// widening or replacing the task's authorization.
var ErrCapabilityUnavailable = errors.New("automation: capability unavailable")

// CapabilityOption is safe catalog metadata for task editors and snapshot
// resolution. It deliberately contains references and tool names only; provider
// keys, connector credentials, prompts, commands, URLs, and headers do not
// belong in this contract.
type CapabilityOption struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Available         bool      `json:"available"`
	UnavailableReason string    `json:"unavailableReason,omitempty"`
	Version           string    `json:"version,omitempty"`
	ContentHash       string    `json:"contentHash,omitempty"`
	ToolAllowlist     []string  `json:"toolAllowlist,omitempty"`
	ReadOnlyTools     []string  `json:"readOnlyTools,omitempty"`
	ServerID          string    `json:"serverId,omitempty"`
	GrantID           string    `json:"grantId,omitempty"`
	GrantPrincipal    string    `json:"grantPrincipal,omitempty"`
	GrantSource       string    `json:"grantSource,omitempty"`
	GrantExpiresAt    time.Time `json:"grantExpiresAt,omitempty"`
}

// CapabilityCatalog is the read-only set of capabilities currently available
// to an automation task.
type CapabilityCatalog struct {
	Models     []CapabilityOption `json:"models"`
	Skills     []CapabilityOption `json:"skills"`
	Experts    []CapabilityOption `json:"experts"`
	Connectors []CapabilityOption `json:"connectors"`
}

// ResolveCapabilitySnapshot freezes a task's selected capability references and
// the selected connectors' current tool allowlists. Missing or unavailable
// references fail closed so a later run cannot acquire a replacement or a
// broader permission set implicitly.
func ResolveCapabilitySnapshot(selection CapabilitySelection, catalog CapabilityCatalog, capturedAt time.Time) (CapabilitySnapshot, error) {
	return resolveCapabilitySnapshot(selection, catalog, CapabilitySnapshotContext{}, capturedAt, false)
}

// CapabilitySnapshotContext supplies run-specific data that cannot be derived
// from a capability catalog. Callers should use this strict entry point for new
// runs so every snapshot is independently auditable.
type CapabilitySnapshotContext struct {
	Prompt    string
	Principal string
}

// ResolveCapabilitySnapshotWithContext freezes verifiable capability and grant
// metadata. Missing identity, artifact version/hash, grant provenance, or a
// non-expired grant fails closed.
func ResolveCapabilitySnapshotWithContext(selection CapabilitySelection, catalog CapabilityCatalog, snapshotContext CapabilitySnapshotContext, capturedAt time.Time) (CapabilitySnapshot, error) {
	return resolveCapabilitySnapshot(selection, catalog, snapshotContext, capturedAt, true)
}

func resolveCapabilitySnapshot(selection CapabilitySelection, catalog CapabilityCatalog, snapshotContext CapabilitySnapshotContext, capturedAt time.Time, strict bool) (CapabilitySnapshot, error) {
	selection.ModelRef = strings.TrimSpace(selection.ModelRef)
	selection.ExpertID = strings.TrimSpace(selection.ExpertID)
	selection.SkillIDs = uniqueTrimmedStrings(selection.SkillIDs)
	selection.ConnectorIDs = uniqueTrimmedStrings(selection.ConnectorIDs)
	snapshotContext.Principal = strings.TrimSpace(snapshotContext.Principal)
	capturedAt = capturedAt.UTC()
	if strict && snapshotContext.Principal == "" {
		return CapabilitySnapshot{}, fmt.Errorf("%w: snapshot principal is required", ErrCapabilityUnavailable)
	}

	indexes, err := newCapabilityIndexes(catalog)
	if err != nil {
		return CapabilitySnapshot{}, err
	}
	if err := validateCapabilitySelectionAvailable(selection, indexes); err != nil {
		return CapabilitySnapshot{}, err
	}
	tools, potentiallyMutating, err := resolveConnectorTools(selection.ConnectorIDs, indexes.connectors)
	if err != nil {
		return CapabilitySnapshot{}, err
	}
	skills, err := resolveCapabilityArtifacts("skill", selection.SkillIDs, indexes.skills, strict)
	if err != nil {
		return CapabilitySnapshot{}, err
	}
	expert, err := resolveExpertArtifact(selection.ExpertID, indexes.experts, strict)
	if err != nil {
		return CapabilitySnapshot{}, err
	}
	connectorGrants, err := resolveConnectorGrants(selection.ConnectorIDs, indexes.connectors, capturedAt, strict)
	if err != nil {
		return CapabilitySnapshot{}, err
	}

	snapshot := CapabilitySnapshot{
		ModelRef:        selection.ModelRef,
		SkillIDs:        append([]string(nil), selection.SkillIDs...),
		ExpertID:        selection.ExpertID,
		ConnectorIDs:    append([]string(nil), selection.ConnectorIDs...),
		ToolAllowlist:   append([]string(nil), tools...),
		Skills:          skills,
		Expert:          expert,
		ConnectorGrants: connectorGrants,
		Principal:       snapshotContext.Principal,
		RiskSummary:     capabilityRiskSummary(len(tools), len(potentiallyMutating)),
		CapturedAt:      capturedAt,
	}
	if strict {
		snapshot.PromptHash = promptDigest(snapshotContext.Prompt)
	}
	return snapshot, nil
}

func resolveCapabilityArtifacts(kind string, ids []string, options map[string]CapabilityOption, strict bool) ([]CapabilityArtifact, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	artifacts := make([]CapabilityArtifact, 0, len(ids))
	for _, id := range ids {
		option := options[id]
		artifact := CapabilityArtifact{ID: id, Version: strings.TrimSpace(option.Version), ContentHash: strings.TrimSpace(option.ContentHash)}
		if strict && artifact.Version == "" && artifact.ContentHash == "" {
			return nil, fmt.Errorf("%w: %s %q has no version or content hash", ErrCapabilityUnavailable, kind, id)
		}
		if artifact.Version != "" || artifact.ContentHash != "" {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, nil
}

func resolveExpertArtifact(id string, options map[string]CapabilityOption, strict bool) (*CapabilityArtifact, error) {
	if id == "" {
		return nil, nil
	}
	artifacts, err := resolveCapabilityArtifacts("expert", []string{id}, options, strict)
	if err != nil || len(artifacts) == 0 {
		return nil, err
	}
	return &artifacts[0], nil
}

func resolveConnectorGrants(ids []string, connectors map[string]CapabilityOption, capturedAt time.Time, strict bool) ([]ConnectorGrantSnapshot, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	grants := make([]ConnectorGrantSnapshot, 0, len(ids))
	for _, id := range ids {
		grant := connectorGrantSnapshot(id, connectors[id])
		complete, err := validateConnectorGrantSnapshot(grant, capturedAt, strict)
		if err != nil {
			return nil, err
		}
		if complete {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

func connectorGrantSnapshot(id string, option CapabilityOption) ConnectorGrantSnapshot {
	return ConnectorGrantSnapshot{
		ConnectorID: id, ServerID: strings.TrimSpace(option.ServerID),
		ToolAllowlist: append([]string(nil), option.ToolAllowlist...),
		GrantID:       strings.TrimSpace(option.GrantID), Principal: strings.TrimSpace(option.GrantPrincipal),
		Source: strings.TrimSpace(option.GrantSource), ExpiresAt: option.GrantExpiresAt.UTC(),
	}
}

func validateConnectorGrantSnapshot(grant ConnectorGrantSnapshot, capturedAt time.Time, strict bool) (bool, error) {
	complete := grant.ServerID != "" && grant.GrantID != "" && grant.Principal != "" && grant.Source != ""
	if strict && !complete {
		return false, fmt.Errorf("%w: connector %q has incomplete grant metadata", ErrCapabilityUnavailable, grant.ConnectorID)
	}
	if strict && !grant.ExpiresAt.IsZero() && !grant.ExpiresAt.After(capturedAt) {
		return false, fmt.Errorf("%w: connector %q grant is expired", ErrCapabilityUnavailable, grant.ConnectorID)
	}
	return complete, nil
}

func promptDigest(prompt string) string {
	digest := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("sha256:%x", digest)
}

type capabilityIndexes struct {
	models     map[string]CapabilityOption
	skills     map[string]CapabilityOption
	experts    map[string]CapabilityOption
	connectors map[string]CapabilityOption
}

func newCapabilityIndexes(catalog CapabilityCatalog) (capabilityIndexes, error) {
	var indexes capabilityIndexes
	var err error
	if indexes.models, err = capabilityIndex("model", catalog.Models); err != nil {
		return capabilityIndexes{}, err
	}
	if indexes.skills, err = capabilityIndex("skill", catalog.Skills); err != nil {
		return capabilityIndexes{}, err
	}
	if indexes.experts, err = capabilityIndex("expert", catalog.Experts); err != nil {
		return capabilityIndexes{}, err
	}
	if indexes.connectors, err = capabilityIndex("connector", catalog.Connectors); err != nil {
		return capabilityIndexes{}, err
	}
	return indexes, nil
}

func validateCapabilitySelectionAvailable(selection CapabilitySelection, indexes capabilityIndexes) error {
	if err := requireCapability("model", selection.ModelRef, indexes.models); err != nil {
		return err
	}
	for _, id := range selection.SkillIDs {
		if err := requireCapability("skill", id, indexes.skills); err != nil {
			return err
		}
	}
	return requireCapability("expert", selection.ExpertID, indexes.experts)
}

func resolveConnectorTools(ids []string, connectors map[string]CapabilityOption) ([]string, map[string]struct{}, error) {
	tools := make([]string, 0)
	seenTools := make(map[string]struct{})
	potentiallyMutating := make(map[string]struct{})
	for _, id := range ids {
		if err := requireCapability("connector", id, connectors); err != nil {
			return nil, nil, err
		}
		appendConnectorTools(connectors[id], &tools, seenTools, potentiallyMutating)
	}
	return tools, potentiallyMutating, nil
}

func appendConnectorTools(connector CapabilityOption, tools *[]string, seen, potentiallyMutating map[string]struct{}) {
	readOnly := stringSet(connector.ReadOnlyTools)
	for _, tool := range uniqueTrimmedStrings(connector.ToolAllowlist) {
		if _, duplicate := seen[tool]; !duplicate {
			seen[tool] = struct{}{}
			*tools = append(*tools, tool)
		}
		if _, ok := readOnly[tool]; !ok {
			potentiallyMutating[tool] = struct{}{}
		}
	}
}

func capabilityIndex(kind string, options []CapabilityOption) (map[string]CapabilityOption, error) {
	out := make(map[string]CapabilityOption, len(options))
	for _, option := range options {
		option.ID = strings.TrimSpace(option.ID)
		if option.ID == "" {
			return nil, fmt.Errorf("%w: %s id is empty", ErrCapabilityUnavailable, kind)
		}
		if _, duplicate := out[option.ID]; duplicate {
			return nil, fmt.Errorf("%w: duplicate %s %q", ErrCapabilityUnavailable, kind, option.ID)
		}
		option.ToolAllowlist = uniqueTrimmedStrings(option.ToolAllowlist)
		option.ReadOnlyTools = uniqueTrimmedStrings(option.ReadOnlyTools)
		option.Version = strings.TrimSpace(option.Version)
		option.ContentHash = strings.TrimSpace(option.ContentHash)
		option.ServerID = strings.TrimSpace(option.ServerID)
		option.GrantID = strings.TrimSpace(option.GrantID)
		option.GrantPrincipal = strings.TrimSpace(option.GrantPrincipal)
		option.GrantSource = strings.TrimSpace(option.GrantSource)
		out[option.ID] = option
	}
	return out, nil
}

func requireCapability(kind, id string, options map[string]CapabilityOption) error {
	if id == "" {
		return nil
	}
	option, ok := options[id]
	if !ok {
		return fmt.Errorf("%w: %s %q was not found", ErrCapabilityUnavailable, kind, id)
	}
	if !option.Available {
		reason := strings.TrimSpace(option.UnavailableReason)
		if reason == "" {
			reason = "unavailable"
		}
		return fmt.Errorf("%w: %s %q is %s", ErrCapabilityUnavailable, kind, id, reason)
	}
	return nil
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range uniqueTrimmedStrings(values) {
		out[value] = struct{}{}
	}
	return out
}

func capabilityRiskSummary(toolCount, potentiallyMutating int) string {
	switch {
	case toolCount == 0:
		return "no connector tools selected"
	case potentiallyMutating == 0:
		return fmt.Sprintf("all %d connector tools are read-only", toolCount)
	default:
		return fmt.Sprintf("%d of %d connector tools may modify data", potentiallyMutating, toolCount)
	}
}
