package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testCapabilityScope() CapabilityTaskScope {
	return CapabilityTaskScope{
		CapabilityID: "crossborder-commerce",
		WorkspaceID:  "workspace-commerce-research",
		TaskID:       "task-portable-blender-opportunity",
		DocumentID:   "primary",
	}
}

func testCapabilityDocument() capabilityDashboardDocument {
	return capabilityDashboardDocument{
		SchemaVersion: capabilityDashboardSchemaVersion,
		DocumentID:    "primary",
		WorkspaceID:   "workspace-commerce-research",
		TaskID:        "task-portable-blender-opportunity",
		Revision:      1,
		UpdatedAt:     "2026-08-21T14:00:00+08:00",
		Widgets: []capabilityDashboardWidget{{
			ID:           "market-summary",
			Type:         "research.summary",
			Version:      "1.0.0",
			Surface:      "conversation-inline",
			Status:       "ready",
			Title:        "Market summary",
			Props:        json.RawMessage(`{"kind":"research.summary","eyebrow":"Validation","conclusion":"PASS","confidence":100,"summary":"mock","metrics":[],"findings":[]}`),
			EvidenceRefs: []string{"ev:seed"},
			Revision:     1,
		}},
		AppliedOperationIDs: []string{"op:seed"},
	}
}

func testCapabilityEvidenceDocument() capabilityEvidenceDocument {
	scope := testCapabilityScope()
	return capabilityEvidenceDocument{
		SchemaVersion: capabilityEvidenceSchemaVersion,
		CapabilityID:  scope.CapabilityID,
		WorkspaceID:   scope.WorkspaceID,
		TaskID:        scope.TaskID,
		Revision:      1,
		UpdatedAt:     "2026-08-21T14:00:00+08:00",
		Artifacts: []capabilityEvidenceArtifact{{
			EvidenceRef: "ev:ebay:active-listing-snapshot",
			Scope: capabilityEvidenceScope{
				CapabilityID: scope.CapabilityID,
				WorkspaceID:  scope.WorkspaceID,
				TaskID:       scope.TaskID,
			},
			Title:   "eBay snapshot",
			Summary: "Authorized task evidence fixture.",
			Facts:   []string{"listing count 1284"},
			Provenance: capabilityEvidenceProvenance{
				Source:     "eBay",
				CapturedAt: "2026-08-21T14:00:00+08:00",
				DataClass:  "mock",
			},
		}},
	}
}

func TestCapabilityDashboardAtomicPersistenceAndAction(t *testing.T) {
	root := t.TempDir()
	scope := testCapabilityScope()
	document := testCapabilityDocument()
	path := capabilityDashboardPath(root, scope)
	if err := writeCapabilityJSONAtomic(path, document); err != nil {
		t.Fatalf("write dashboard: %v", err)
	}

	var loaded capabilityDashboardDocument
	if err := readCapabilityJSON(path, &loaded); err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	if err := validateCapabilityDocument(loaded, scope); err != nil {
		t.Fatalf("validate dashboard: %v", err)
	}
	action := capabilityDashboardAction{
		Type:         "dashboard.attach_evidence",
		OperationID:  "ui:cite:market-summary",
		BaseRevision: loaded.Revision,
		At:           "2026-08-21T15:00:00+08:00",
		WidgetID:     "market-summary",
		EvidenceRefs: []string{"ev:session:citation"},
	}
	if err := applyCapabilityAction(&loaded, action); err != nil {
		t.Fatalf("apply action: %v", err)
	}
	if loaded.Revision != 2 || loaded.Widgets[0].Revision != 2 {
		t.Fatalf("unexpected revisions: document=%d widget=%d", loaded.Revision, loaded.Widgets[0].Revision)
	}
	if !containsCapabilityOperation(loaded.AppliedOperationIDs, action.OperationID) {
		t.Fatal("operation id was not recorded")
	}
	if err := writeCapabilityJSONAtomic(path, loaded); err != nil {
		t.Fatalf("replace dashboard: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat dashboard: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("dashboard permissions too broad: %o", info.Mode().Perm())
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".capability-*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary files remained: %v, %v", matches, err)
	}
}

func TestCapabilityDashboardRejectsInvalidScopeAndWidgetSurface(t *testing.T) {
	scope := testCapabilityScope()
	scope.TaskID = "../escape"
	if err := validateCapabilityScope(scope); err == nil {
		t.Fatal("expected traversal-like task id to be rejected")
	}

	document := testCapabilityDocument()
	document.Widgets[0].Type = "trend.chart"
	if err := validateCapabilityDocument(document, testCapabilityScope()); err == nil {
		t.Fatal("expected unsupported inline trend chart to be rejected")
	}
}

func TestCapabilityDashboardRejectsResearchSummaryWithObjectMetrics(t *testing.T) {
	document := testCapabilityDocument()
	document.Widgets[0].Props = json.RawMessage(`{"kind":"research.summary","eyebrow":"Validation","conclusion":"PASS","confidence":100,"summary":"PASS","metrics":{"status":"PASS"},"findings":[]}`)
	if err := validateCapabilityDocument(document, testCapabilityScope()); err == nil || !strings.Contains(err.Error(), "cannot unmarshal object") {
		t.Fatalf("expected object metrics to be rejected, got %v", err)
	}
}

func TestCapabilityStableSessionBindingRoundTrip(t *testing.T) {
	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "session-001.jsonl")
	identity := capabilityStableSessionIdentity(sessionPath)
	if identity == "" || identity != capabilityStableSessionIdentity(sessionPath) {
		t.Fatalf("unstable session identity %q", identity)
	}

	binding := capabilitySessionBinding{
		SchemaVersion:     capabilityDashboardSchemaVersion,
		SessionHash:       "legacy-tab-hash",
		SessionIdentity:   identity,
		SessionDocumentID: "session-" + identity[:16],
		Scope:             testCapabilityScope(),
		UpdatedAt:         "2026-08-25T16:00:00+08:00",
	}
	path := capabilityStableBindingPath(root, identity)
	if err := writeCapabilityJSONAtomic(path, binding); err != nil {
		t.Fatalf("write stable binding: %v", err)
	}

	var loaded capabilitySessionBinding
	if err := readCapabilityJSON(path, &loaded); err != nil {
		t.Fatalf("read stable binding: %v", err)
	}
	if loaded.SessionIdentity != identity || loaded.Scope != binding.Scope {
		t.Fatalf("unexpected stable binding: %#v", loaded)
	}
	if err := validateCapabilitySessionBinding(loaded); err != nil {
		t.Fatalf("validate stable binding: %v", err)
	}
	sessionScope := capabilitySessionScope(loaded)
	if sessionScope.DocumentID != binding.SessionDocumentID || sessionScope == binding.Scope {
		t.Fatalf("session scope did not remain isolated from aggregate scope: %#v", sessionScope)
	}
}

func TestCapabilitySessionDocumentIDMigratesDeterministically(t *testing.T) {
	binding := capabilitySessionBinding{
		SchemaVersion: capabilityDashboardSchemaVersion,
		SessionHash:   strings.Repeat("a", 64),
		Scope:         testCapabilityScope(),
	}
	active := capabilityActiveContext{sessionHash: binding.SessionHash}
	if !completeCapabilitySessionBinding(active, &binding) {
		t.Fatal("expected legacy binding to be completed")
	}
	if binding.SessionDocumentID != "session-aaaaaaaaaaaaaaaa" {
		t.Fatalf("session document id = %q", binding.SessionDocumentID)
	}
	if completeCapabilitySessionBinding(active, &binding) {
		t.Fatal("completed binding should remain stable")
	}
}

func TestCapabilityDashboardActionRejectsStaleAndIsIdempotentBeforeMutation(t *testing.T) {
	document := testCapabilityDocument()
	action := capabilityDashboardAction{Type: "dashboard.attach_evidence", OperationID: "op:first", BaseRevision: document.Revision, At: time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), WidgetID: "market-summary", EvidenceRefs: []string{"ev:new"}}
	changed, err := mutateCapabilityDocument(&document, action)
	if err != nil || !changed {
		t.Fatalf("first mutation: changed=%v err=%v", changed, err)
	}
	revision := document.Revision
	changed, err = mutateCapabilityDocument(&document, action)
	if err != nil || changed || document.Revision != revision {
		t.Fatalf("idempotent replay changed document: changed=%v revision=%d err=%v", changed, document.Revision, err)
	}
	stale := capabilityDashboardAction{
		Type:         "dashboard.attach_evidence",
		OperationID:  "op:stale",
		BaseRevision: 0,
		At:           "2026-08-21T15:00:00Z",
		WidgetID:     "market-summary",
		EvidenceRefs: []string{"ev:new"},
	}
	if changed, err = mutateCapabilityDocument(&document, stale); err == nil || changed {
		t.Fatalf("expected stale action rejection: changed=%v err=%v", changed, err)
	}
}

func TestCapabilityEvidenceAtomicPersistenceAndValidation(t *testing.T) {
	root := t.TempDir()
	scope := testCapabilityScope()
	document := testCapabilityEvidenceDocument()
	path := capabilityEvidencePath(root, scope)
	if err := validateCapabilityEvidenceDocument(document, scope); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}
	if err := writeCapabilityJSONAtomic(path, document); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	var loaded capabilityEvidenceDocument
	if err := readCapabilityJSON(path, &loaded); err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if loaded.Artifacts[0].EvidenceRef != document.Artifacts[0].EvidenceRef {
		t.Fatalf("evidence reference = %q", loaded.Artifacts[0].EvidenceRef)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat evidence: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("evidence permissions too broad: %o", info.Mode().Perm())
	}
}

func TestCapabilityEvidenceRejectsScopeDuplicatesAndOversizedContent(t *testing.T) {
	scope := testCapabilityScope()
	document := testCapabilityEvidenceDocument()
	document.Artifacts[0].Scope.TaskID = "another-task"
	if err := validateCapabilityEvidenceDocument(document, scope); err == nil {
		t.Fatal("expected artifact scope mismatch")
	}

	document = testCapabilityEvidenceDocument()
	document.Artifacts = append(document.Artifacts, document.Artifacts[0])
	if err := validateCapabilityEvidenceDocument(document, scope); err == nil {
		t.Fatal("expected duplicate evidence reference rejection")
	}

	document = testCapabilityEvidenceDocument()
	document.Artifacts[0].Summary = strings.Repeat("x", 8_001)
	if err := validateCapabilityEvidenceDocument(document, scope); err == nil {
		t.Fatal("expected oversized evidence summary rejection")
	}
}
