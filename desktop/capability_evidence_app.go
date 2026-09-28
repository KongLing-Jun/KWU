package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	capabilityEvidenceSchemaVersion = 1
	capabilityEvidenceMaxArtifacts  = 128
	capabilityEvidenceMaxFacts      = 32
)

var capabilityEvidenceRef = regexp.MustCompile(`^ev:[A-Za-z0-9][A-Za-z0-9._:-]{0,235}$`)

type capabilityEvidenceDocument struct {
	SchemaVersion int                          `json:"schemaVersion"`
	CapabilityID  string                       `json:"capabilityId"`
	WorkspaceID   string                       `json:"workspaceId"`
	TaskID        string                       `json:"taskId"`
	Revision      int                          `json:"revision"`
	UpdatedAt     string                       `json:"updatedAt"`
	Artifacts     []capabilityEvidenceArtifact `json:"artifacts"`
}

type capabilityEvidenceArtifact struct {
	EvidenceRef string                       `json:"evidenceRef"`
	Scope       capabilityEvidenceScope      `json:"scope"`
	Title       string                       `json:"title"`
	Summary     string                       `json:"summary"`
	Facts       []string                     `json:"facts"`
	Provenance  capabilityEvidenceProvenance `json:"provenance"`
}

type capabilityEvidenceScope struct {
	CapabilityID string `json:"capabilityId"`
	WorkspaceID  string `json:"workspaceId"`
	TaskID       string `json:"taskId"`
}

type capabilityEvidenceProvenance struct {
	Source     string `json:"source"`
	CapturedAt string `json:"capturedAt"`
	DataClass  string `json:"dataClass"`
}

// EnsureCapabilityEvidence installs the task evidence catalog exactly once.
// Future connector/MCP ingestion should use a separate controlled mutation
// surface; callers cannot replace an existing catalog with a seed.
func (a *App) EnsureCapabilityEvidence(scope CapabilityTaskScope, seedJSON string) (string, error) {
	if len(seedJSON) > capabilityDashboardMaxPayload {
		return "", errors.New("evidence seed exceeds payload limit")
	}
	active, err := a.capabilityActiveContext()
	if err != nil {
		return "", err
	}
	a.capabilityDashboardMu.Lock()
	defer a.capabilityDashboardMu.Unlock()
	if err := validateCapabilityBinding(active, scope); err != nil {
		return "", err
	}

	path := capabilityEvidencePath(active.stateRoot, scope)
	var document capabilityEvidenceDocument
	if err := readCapabilityJSON(path, &document); err == nil {
		if err := validateCapabilityEvidenceDocument(document, scope); err != nil {
			return "", err
		}
		kept := document.Artifacts[:0]
		for _, artifact := range document.Artifacts {
			if artifact.Provenance.DataClass == "mock" || artifact.Provenance.DataClass == "official-research" || isCapabilityPrototypeEvidenceRef(artifact.EvidenceRef) {
				continue
			}
			kept = append(kept, artifact)
		}
		if len(kept) != len(document.Artifacts) {
			document.Artifacts = kept
			document.Revision++
			document.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := writeCapabilityJSONAtomic(path, document); err != nil {
				return "", err
			}
		}
		return marshalCapabilityEvidenceDocument(document)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := decodeCapabilityJSON([]byte(seedJSON), &document); err != nil {
		return "", fmt.Errorf("invalid evidence seed: %w", err)
	}
	if err := validateCapabilityEvidenceDocument(document, scope); err != nil {
		return "", err
	}
	if err := writeCapabilityJSONAtomic(path, document); err != nil {
		return "", err
	}
	return marshalCapabilityEvidenceDocument(document)
}

// LoadCapabilityEvidence reads evidence only from the active bound task.
func (a *App) LoadCapabilityEvidence(scope CapabilityTaskScope) (string, error) {
	active, err := a.capabilityActiveContext()
	if err != nil {
		return "", err
	}
	a.capabilityDashboardMu.Lock()
	defer a.capabilityDashboardMu.Unlock()
	if err := validateCapabilityBinding(active, scope); err != nil {
		return "", err
	}
	var document capabilityEvidenceDocument
	if err := readCapabilityJSON(capabilityEvidencePath(active.stateRoot, scope), &document); err != nil {
		return "", err
	}
	if err := validateCapabilityEvidenceDocument(document, scope); err != nil {
		return "", err
	}
	return marshalCapabilityEvidenceDocument(document)
}

func validateCapabilityEvidenceDocument(document capabilityEvidenceDocument, scope CapabilityTaskScope) error {
	if document.SchemaVersion != capabilityEvidenceSchemaVersion {
		return errors.New("unsupported evidence schema version")
	}
	if document.CapabilityID != scope.CapabilityID || document.WorkspaceID != scope.WorkspaceID || document.TaskID != scope.TaskID {
		return errors.New("evidence document does not match bound scope")
	}
	if document.Revision < 1 || len(document.Artifacts) > capabilityEvidenceMaxArtifacts {
		return errors.New("invalid evidence document limits")
	}
	if _, err := time.Parse(time.RFC3339Nano, document.UpdatedAt); err != nil {
		return errors.New("invalid evidence document timestamp")
	}
	seen := make(map[string]struct{}, len(document.Artifacts))
	for _, artifact := range document.Artifacts {
		if _, exists := seen[artifact.EvidenceRef]; exists {
			return fmt.Errorf("duplicate evidence reference: %s", artifact.EvidenceRef)
		}
		seen[artifact.EvidenceRef] = struct{}{}
		if err := validateCapabilityEvidenceArtifact(artifact, scope); err != nil {
			return err
		}
	}
	return nil
}

func validateCapabilityEvidenceArtifact(artifact capabilityEvidenceArtifact, scope CapabilityTaskScope) error {
	if !capabilityEvidenceRef.MatchString(artifact.EvidenceRef) {
		return errors.New("invalid evidence reference")
	}
	if artifact.Scope.CapabilityID != scope.CapabilityID || artifact.Scope.WorkspaceID != scope.WorkspaceID || artifact.Scope.TaskID != scope.TaskID {
		return fmt.Errorf("evidence artifact scope mismatch: %s", artifact.EvidenceRef)
	}
	title := strings.TrimSpace(artifact.Title)
	summary := strings.TrimSpace(artifact.Summary)
	if title == "" || len(title) > 240 || summary == "" || len(summary) > 8_000 || len(artifact.Facts) > capabilityEvidenceMaxFacts {
		return fmt.Errorf("invalid evidence artifact content: %s", artifact.EvidenceRef)
	}
	for _, fact := range artifact.Facts {
		if strings.TrimSpace(fact) == "" || len(fact) > 2_000 {
			return fmt.Errorf("invalid evidence fact: %s", artifact.EvidenceRef)
		}
	}
	validSources := map[string]bool{"1688": true, "eBay": true, "Shopify": true, "research": true}
	validClasses := map[string]bool{"mock": true, "official-research": true, "live-authorized": true, "live-public": true}
	if !validSources[artifact.Provenance.Source] || !validClasses[artifact.Provenance.DataClass] {
		return fmt.Errorf("invalid evidence provenance: %s", artifact.EvidenceRef)
	}
	if _, err := time.Parse(time.RFC3339Nano, artifact.Provenance.CapturedAt); err != nil {
		return fmt.Errorf("invalid evidence capture timestamp: %s", artifact.EvidenceRef)
	}
	return nil
}

func capabilityEvidencePath(root string, scope CapabilityTaskScope) string {
	return filepath.Join(root, "superpowers", "runtime", scope.WorkspaceID, "tasks", scope.TaskID, "artifacts", "evidence.json")
}

func marshalCapabilityEvidenceDocument(document capabilityEvidenceDocument) (string, error) {
	payload, err := json.Marshal(document)
	return string(payload), err
}
