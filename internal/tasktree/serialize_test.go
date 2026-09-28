package tasktree

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMarshalSnapshotRoundTripsStableJSON(t *testing.T) {
	tree := sampleSnapshotTree()
	snapshot := Snapshot{Tree: tree}

	first, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}
	second, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatalf("MarshalSnapshot second: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("snapshot JSON should be stable across repeated marshals")
	}
	text := string(first)
	for _, want := range []string{`"version": 1`, `"mode": "task_tree"`, `"tree": {`, `"raw_output_ref": "subagent://inspect/raw"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("snapshot JSON missing %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, `"messages"`) {
		t.Fatalf("snapshot must not embed session messages:\n%s", text)
	}

	roundTrip, err := UnmarshalSnapshot(first)
	if err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	if roundTrip.Version != SnapshotVersion || roundTrip.Mode != SnapshotModeTaskTree {
		t.Fatalf("round trip version/mode = %d/%q", roundTrip.Version, roundTrip.Mode)
	}
	if roundTrip.Tree == nil {
		t.Fatal("round trip tree is nil")
	}
	if roundTrip.Tree.ID != tree.ID || roundTrip.Tree.Nodes["inspect"].Result.RawOutputRef != "subagent://inspect/raw" {
		t.Fatalf("round trip tree lost data: %+v", roundTrip.Tree)
	}
}

func TestSaveLoadSnapshotForSessionUsesSidecarPath(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.jsonl")
	sessionBytes := []byte(`{"role":"user","content":"hello"}` + "\n")
	if err := os.WriteFile(sessionPath, sessionBytes, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	snapshot := Snapshot{Mode: SnapshotModeTaskTree, Tree: sampleSnapshotTree()}
	if err := SaveSnapshotForSession(sessionPath, snapshot); err != nil {
		t.Fatalf("SaveSnapshotForSession: %v", err)
	}

	sidecar := SnapshotPathForSession(sessionPath)
	if sidecar != sessionPath+SnapshotSidecarSuffix {
		t.Fatalf("sidecar path = %q", sidecar)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	afterSessionBytes, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read session after snapshot save: %v", err)
	}
	if !bytes.Equal(afterSessionBytes, sessionBytes) {
		t.Fatalf("snapshot save changed session messages:\n%s", string(afterSessionBytes))
	}

	loaded, ok, err := LoadSnapshotForSession(sessionPath)
	if err != nil {
		t.Fatalf("LoadSnapshotForSession: %v", err)
	}
	if !ok {
		t.Fatal("LoadSnapshotForSession ok = false")
	}
	if loaded.Tree == nil || loaded.Tree.ID != snapshot.Tree.ID {
		t.Fatalf("loaded snapshot = %+v", loaded)
	}
}

func TestLoadSnapshotForSessionMissingSidecarKeepsLegacySessionCompatible(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "legacy.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	snapshot, ok, err := LoadSnapshotForSession(sessionPath)
	if err != nil {
		t.Fatalf("LoadSnapshotForSession: %v", err)
	}
	if ok {
		t.Fatalf("legacy session should not report task-tree state: %+v", snapshot)
	}

	resume := ResumeStateFromSnapshot("normal", snapshot, ok)
	if resume.Mode != "normal" || resume.HasTaskTree || resume.TreeComplete {
		t.Fatalf("legacy resume state = %+v", resume)
	}
}

func TestResumeStateFromSnapshotReportsModeTreePresenceAndCompletion(t *testing.T) {
	tree := sampleSnapshotTree()
	snapshot := Snapshot{Version: SnapshotVersion, Mode: SnapshotModeTaskTree, Tree: tree}

	resume := ResumeStateFromSnapshot("", snapshot, true)
	if resume.Mode != SnapshotModeTaskTree || !resume.HasTaskTree || resume.TreeComplete {
		t.Fatalf("running resume state = %+v", resume)
	}
	if resume.TreeID != tree.ID || resume.TreeStatus != StatusRunning {
		t.Fatalf("running tree identity/status = %+v", resume)
	}

	tree.Status = StatusCompleted
	resume = ResumeStateFromSnapshot("", snapshot, true)
	if !resume.TreeComplete || resume.TreeStatus != StatusCompleted {
		t.Fatalf("completed resume state = %+v", resume)
	}

	resume = ResumeStateFromSnapshot("task_tree", Snapshot{}, false)
	if resume.Mode != "task_tree" || resume.HasTaskTree || resume.TreeComplete {
		t.Fatalf("missing snapshot resume state = %+v", resume)
	}
}

func TestUnmarshalSnapshotRejectsInvalidBoundaryState(t *testing.T) {
	if _, err := UnmarshalSnapshot([]byte(`{"version":2,"mode":"task_tree"}`)); err == nil {
		t.Fatal("unsupported snapshot version should fail")
	}

	badStatus := Snapshot{
		Version: SnapshotVersion,
		Mode:    SnapshotModeTaskTree,
		Tree: &TaskTree{
			ID:     "tree-bad",
			Goal:   "bad",
			Status: Status("waiting"),
			Nodes:  map[string]*TaskNode{},
		},
	}
	b, err := jsonForTest(badStatus)
	if err != nil {
		t.Fatalf("jsonForTest: %v", err)
	}
	if _, err := UnmarshalSnapshot(b); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("bad status error = %v, want ErrInvalidStatus", err)
	}

	badExecutor := Snapshot{
		Version: SnapshotVersion,
		Mode:    SnapshotModeTaskTree,
		Tree: &TaskTree{
			ID:     "tree-bad-exec",
			Goal:   "bad executor",
			Status: StatusRunning,
			Nodes: map[string]*TaskNode{
				"n1": {
					ID:       "n1",
					Title:    "n1",
					Status:   StatusRunning,
					Executor: &ExecutorSpec{Kind: ExecutorDedicatedAgent},
				},
			},
		},
	}
	b, err = jsonForTest(badExecutor)
	if err != nil {
		t.Fatalf("jsonForTest bad executor: %v", err)
	}
	if _, err := UnmarshalSnapshot(b); err == nil {
		t.Fatal("invalid executor should fail")
	}
}

func TestSnapshotPreservesSessionMessageBoundary(t *testing.T) {
	tree := sampleSnapshotTree()
	snapshotBytes, err := MarshalSnapshot(Snapshot{Tree: tree})
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}

	sessionMessages := []byte(`{"role":"user","content":"hello task tree"}` + "\n")
	if bytes.Contains(snapshotBytes, sessionMessages) {
		t.Fatal("snapshot should not contain JSONL session messages")
	}
	if reflect.DeepEqual(snapshotBytes, sessionMessages) {
		t.Fatal("snapshot JSON should be distinct from message JSONL storage")
	}
}

func sampleSnapshotTree() *TaskTree {
	when := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	tree := NewTaskTree("tree-snapshot", "Persist task tree mode state", when)
	tree.Status = StatusRunning
	tree.RootIDs = []string{"root"}
	tree.Nodes["root"] = &TaskNode{
		ID:             "root",
		Title:          "Persist state",
		Description:    "Keep task tree state outside messages",
		Children:       []string{"inspect"},
		Status:         StatusRunning,
		Executor:       &ExecutorSpec{Kind: ExecutorTask},
		Constraints:    []string{"Do not mutate session messages"},
		RelatedFiles:   []string{"internal/tasktree/serialize.go"},
		ToolHints:      []string{"go test ./internal/tasktree ./internal/agent"},
		ExpectedOutput: "Stable JSON snapshot",
	}
	tree.Nodes["inspect"] = &TaskNode{
		ID:        "inspect",
		ParentID:  "root",
		Title:     "Inspect session boundary",
		DependsOn: []string{"root"},
		Status:    StatusCompleted,
		Result: &NodeResult{
			Summary:      "Messages live in session JSONL; tree state lives in sidecar JSON",
			RawOutputRef: "subagent://inspect/raw",
			Artifacts: []NodeArtifact{{
				Type: "note",
				Name: "boundary",
				Data: map[string]any{"version": float64(1)},
			}},
		},
	}
	return tree
}

func jsonForTest(s Snapshot) ([]byte, error) {
	return json.Marshal(s)
}
