package agent

import (
	"path/filepath"
	"testing"

	"kwu/internal/provider"
	"kwu/internal/scene"
)

func TestBranchMetaSceneStatusRoundTripAndLegacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	meta := BranchMeta{}
	want := scene.Status{Scene: scene.Coding, State: scene.Locked}
	if err := meta.SetSceneStatus(want); err != nil {
		t.Fatalf("SetSceneStatus: %v", err)
	}
	if err := SaveBranchMeta(path, meta); err != nil {
		t.Fatalf("SaveBranchMeta: %v", err)
	}
	loaded, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", loaded, ok, err)
	}
	got, err := loaded.SceneStatus()
	if err != nil || got != want {
		t.Fatalf("SceneStatus = %+v, %v; want %+v", got, err, want)
	}

	legacy, err := (BranchMeta{}).SceneStatus()
	if err != nil || !legacy.IsLegacy() {
		t.Fatalf("missing scene = %+v, %v; want legacy", legacy, err)
	}
	illegal, err := (BranchMeta{Scene: "code", SceneState: "locked"}).SceneStatus()
	if err == nil || !illegal.IsLegacy() {
		t.Fatalf("illegal scene = %+v, %v; want diagnostic legacy", illegal, err)
	}
}

func TestBranchMetaComposerStateRoundTripAndInheritance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	effort := "high"
	approval := "ask"
	want := BranchMeta{Model: "custom/deepseek-v4", BillingSource: "newapi_byok", Effort: &effort, ToolApprovalMode: &approval}
	if err := SaveBranchMeta(path, want); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", loaded, ok, err)
	}
	child := BranchMeta{}
	child.InheritComposerState(loaded)
	if child.Model != want.Model || child.BillingSource != want.BillingSource || child.Effort == nil || *child.Effort != effort || child.ToolApprovalMode == nil || *child.ToolApprovalMode != approval {
		t.Fatalf("inherited composer state = %+v", child)
	}
	*child.Effort = "low"
	*child.ToolApprovalMode = "auto"
	if *loaded.Effort != effort {
		t.Fatal("child effort pointer aliases parent")
	}
	if *loaded.ToolApprovalMode != approval {
		t.Fatal("child tool approval pointer aliases parent")
	}
}

func TestBranchMetaComposerApprovalDistinguishesMissingAndExplicitAsk(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "missing.jsonl")
	if err := SaveBranchMeta(missingPath, BranchMeta{}); err != nil {
		t.Fatal(err)
	}
	missing, ok, err := LoadBranchMeta(missingPath)
	if err != nil || !ok || missing.ToolApprovalMode != nil {
		t.Fatalf("missing approval = %+v, %v, %v", missing.ToolApprovalMode, ok, err)
	}

	ask := "ask"
	explicitPath := filepath.Join(dir, "explicit.jsonl")
	if err := SaveBranchMeta(explicitPath, BranchMeta{ToolApprovalMode: &ask}); err != nil {
		t.Fatal(err)
	}
	explicit, ok, err := LoadBranchMeta(explicitPath)
	if err != nil || !ok || explicit.ToolApprovalMode == nil || *explicit.ToolApprovalMode != ask {
		t.Fatalf("explicit approval = %+v, %v, %v", explicit.ToolApprovalMode, ok, err)
	}
}

func TestBranchMetaRoundTripAndList(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "root.jsonl")
	childPath := filepath.Join(dir, "child.jsonl")

	root := NewSession("sys")
	root.Add(provider.Message{Role: provider.RoleUser, Content: "root prompt"})
	if err := root.Save(rootPath); err != nil {
		t.Fatal(err)
	}
	if err := TouchBranchMeta(rootPath); err != nil {
		t.Fatal(err)
	}

	child := NewSession("sys")
	child.Add(provider.Message{Role: provider.RoleUser, Content: "child prompt"})
	if err := child.Save(childPath); err != nil {
		t.Fatal(err)
	}
	if err := SaveBranchMeta(childPath, BranchMeta{Name: "experiment", ParentID: BranchID(rootPath), ForkTurn: 2}); err != nil {
		t.Fatal(err)
	}

	branches, err := ListBranches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("branches = %d, want 2", len(branches))
	}
	var rootFound, childFound bool
	for _, b := range branches {
		if b.ID == "root" {
			rootFound = true
		}
		if b.ParentID == "root" && b.Name == "experiment" {
			childFound = true
		}
	}
	if !rootFound {
		t.Fatal("root branch not found")
	}
	if !childFound {
		t.Fatalf("child with parent root and name experiment not found among %+v", branches)
	}
}

func TestListBranchesSkipsCleanupPending(t *testing.T) {
	dir := t.TempDir()
	visiblePath := filepath.Join(dir, "visible.jsonl")
	pendingPath := filepath.Join(dir, "pending.jsonl")

	visible := NewSession("sys")
	visible.Add(provider.Message{Role: provider.RoleUser, Content: "visible prompt"})
	if err := visible.Save(visiblePath); err != nil {
		t.Fatal(err)
	}
	if err := TouchBranchMeta(visiblePath); err != nil {
		t.Fatal(err)
	}

	pending := NewSession("sys")
	pending.Add(provider.Message{Role: provider.RoleUser, Content: "pending prompt"})
	if err := pending.Save(pendingPath); err != nil {
		t.Fatal(err)
	}
	if err := SaveBranchMeta(pendingPath, BranchMeta{Name: "pending experiment"}); err != nil {
		t.Fatal(err)
	}
	if err := MarkCleanupPending(pendingPath, "delete"); err != nil {
		t.Fatal(err)
	}

	branches, err := ListBranches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 {
		t.Fatalf("branches = %d, want 1: %+v", len(branches), branches)
	}
	if branches[0].Path != visiblePath {
		t.Fatalf("listed branch path = %q, want %q", branches[0].Path, visiblePath)
	}
}

func TestSessionModelRoundTripPreservesActivity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	session := NewSession("sys")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	if err := session.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadSessionModel(path); ok {
		t.Fatal("fresh session should not have a stored model")
	}
	meta, err := EnsureBranchMeta(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := SetBranchModelPreserveUpdated(path, "openrouter/anthropic/claude-sonnet"); err != nil {
		t.Fatal(err)
	}
	model, ok := LoadSessionModel(path)
	if !ok || model != "openrouter/anthropic/claude-sonnet" {
		t.Fatalf("LoadSessionModel = %q, %v", model, ok)
	}
	updated, ok, err := LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta ok=%v err=%v", ok, err)
	}
	if !updated.UpdatedAt.Equal(meta.UpdatedAt) {
		t.Fatalf("model write refreshed activity: before=%s after=%s", meta.UpdatedAt, updated.UpdatedAt)
	}
}
