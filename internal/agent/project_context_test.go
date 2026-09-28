package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/provider"
)

func testProjectContextPin() *ProjectContextPin {
	return &ProjectContextPin{
		Environment:  ProjectEnvironmentIdentity{EnvironmentID: strings.Repeat("a", 64), CloudBaseURL: "https://cloud.example.com"},
		BindingScope: ProjectBindingScope{ProfileID: "default", CloudUserID: "user-1", BindingGeneration: 3},
		ProjectID:    "project-1", InstructionVersion: 7, ContentHash: strings.Repeat("b", 64),
	}
}

func TestBranchMetaProjectContextPinRoundTripAndLegacyIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	legacy := BranchMeta{Scope: "project", WorkspaceRoot: `D:\legacy-workspace`}
	if err := SaveBranchMeta(path, legacy); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta legacy = %#v, %v, %v", got, ok, err)
	}
	if got.IsCloudProjectBound() {
		t.Fatal("legacy LocalWorkspace metadata must not be inferred as CloudProject")
	}

	want := BranchMeta{Scope: "project", WorkspaceRoot: `D:\legacy-workspace`, ProjectContext: testProjectContextPin()}
	if err := SaveBranchMeta(path, want); err != nil {
		t.Fatal(err)
	}
	got, _, err = LoadBranchMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := got.ValidateProjectContextPin(); err != nil || !got.IsCloudProjectBound() {
		t.Fatalf("project pin = %#v, validate=%v, bound=%v", got.ProjectContext, err, got.IsCloudProjectBound())
	}
	got.ProjectContext.ContentHash = "partial"
	if err := got.ValidateProjectContextPin(); err == nil || got.IsCloudProjectBound() {
		t.Fatalf("partial project pin must fail closed: validate=%v bound=%v", err, got.IsCloudProjectBound())
	}
	if err := SaveBranchMeta(path, got); err == nil {
		t.Fatal("partial project pin must not be persisted as an ordinary session")
	}
	partialJSON := []byte(`{"id":"session","project_context":{"environment":{"environment_id":"partial"}}}`)
	if err := os.WriteFile(BranchMetaPath(path), partialJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadBranchMeta(path); err == nil {
		t.Fatal("partial on-disk project pin must fail load closed")
	}
}

func TestSessionProjectContextRoundTripAndPrivacyProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	contextBody := "PRIVATE PROJECT INSTRUCTION"
	s := NewSession("")
	s.Add(provider.Message{Role: provider.RoleUser, Content: contextBody, MessageKind: provider.MessageKindProjectContext, Metadata: &provider.MessageMetadata{Source: "cloud_project_instruction", Version: 2, ContentHash: strings.Repeat("c", 64)}})
	s.Add(provider.Message{Role: provider.RoleUser, Content: "real task"})
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Snapshot(); len(got) != 2 || !got[0].IsProjectContext() || got[0].Content != contextBody {
		t.Fatalf("loaded messages = %#v", got)
	}
	preview, turns := SessionPreviewFromMessages(loaded.Snapshot())
	if preview != "real task" || turns != 1 {
		t.Fatalf("preview/turns = %q/%d, want real task/1", preview, turns)
	}
	inspection := InspectProjectContext(loaded.Snapshot())
	if inspection == nil || inspection.Body != contextBody || inspection.Source != "cloud_project_instruction" || inspection.Version != 2 {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestBranchMetaProjectContextPinSurvivesResumeAndAutomationMetadataUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "automation-topic.jsonl")
	pin := testProjectContextPin()
	want := BranchMeta{TopicID: "automation-topic", TopicTitle: "Daily task", ProjectContext: pin}
	if err := SaveBranchMeta(path, want); err != nil {
		t.Fatal(err)
	}
	resumed, ok, err := LoadBranchMeta(path)
	if err != nil || !ok || resumed.ProjectContext == nil || *resumed.ProjectContext != *pin {
		t.Fatalf("resume metadata = %#v, ok=%v err=%v", resumed, ok, err)
	}
	resumed.TopicTitle = "Daily task updated"
	if err := SaveBranchMetaPreserveUpdated(path, resumed); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadBranchMeta(path)
	if err != nil || !ok || got.TopicTitle != "Daily task updated" || got.ProjectContext == nil || *got.ProjectContext != *pin {
		t.Fatalf("automation metadata round-trip = %#v, ok=%v err=%v", got, ok, err)
	}
}

func TestSessionCatalogRebuildExcludesProjectContextFromLegacyCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "PRIVATE", MessageKind: provider.MessageKindProjectContext, Metadata: &provider.MessageMetadata{Source: provider.ProjectContextSourceCloudInstruction, Version: 1, ContentHash: strings.Repeat("a", 64)}},
		{Role: provider.RoleUser, Content: "real task"},
	}
	if err := NewSessionFromMessages(messages).Save(path); err != nil {
		t.Fatal(err)
	}
	if err := SaveBranchMetaPreserveUpdated(path, BranchMeta{SchemaVersion: 1, Turns: 2, Preview: "PRIVATE"}); err != nil {
		t.Fatal(err)
	}
	listed, err := ListSessions(dir)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListSessions = %#v, %v", listed, err)
	}
	if listed[0].Turns != 1 || listed[0].Preview != "real task" {
		t.Fatalf("rebuilt catalog row = %#v", listed[0])
	}
	meta, _, err := LoadBranchMeta(path)
	if err != nil || meta.SchemaVersion != BranchMetaCountsVersion {
		t.Fatalf("rebuilt sidecar = %#v, %v", meta, err)
	}
}
