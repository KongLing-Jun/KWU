package tasktree

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetectTaskTreeStateReadsSidecarAndReportsResumeBoundary(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	tree := sampleSnapshotTree()
	tree.Nodes["blocked"] = &TaskNode{
		ID:     "blocked",
		Title:  "Blocked work",
		Status: StatusBlocked,
	}
	tree.Nodes["failed"] = &TaskNode{
		ID:     "failed",
		Title:  "Failed work",
		Status: StatusFailed,
	}

	if err := SaveSnapshotForSession(sessionPath, Snapshot{Tree: tree}); err != nil {
		t.Fatalf("SaveSnapshotForSession: %v", err)
	}

	snapshot, ok, err := DetectTaskTreeState(sessionPath)
	if err != nil {
		t.Fatalf("DetectTaskTreeState: %v", err)
	}
	if !ok || snapshot.Tree == nil || snapshot.Tree.ID != tree.ID {
		t.Fatalf("detected snapshot = %+v, ok=%t", snapshot, ok)
	}

	state, err := DetectTaskTreeResumeState(sessionPath, "")
	if err != nil {
		t.Fatalf("DetectTaskTreeResumeState: %v", err)
	}
	if state.Mode != SnapshotModeTaskTree || !state.HasSnapshot || !state.HasTaskTree {
		t.Fatalf("resume mode/presence = %+v", state)
	}
	if state.TreeComplete || state.TreeStatus != StatusRunning {
		t.Fatalf("resume tree completion/status = %+v", state)
	}
	if !reflect.DeepEqual(state.RunningNodes, []string{"root"}) {
		t.Fatalf("running nodes = %v", state.RunningNodes)
	}
	if !reflect.DeepEqual(state.BlockedNodes, []string{"blocked"}) {
		t.Fatalf("blocked nodes = %v", state.BlockedNodes)
	}
	if !reflect.DeepEqual(state.FailedNodes, []string{"failed"}) {
		t.Fatalf("failed nodes = %v", state.FailedNodes)
	}
	if state.AutoContinue || state.ResumePolicy != ResumePolicyInspectOnly {
		t.Fatalf("resume policy = %+v", state)
	}

	rendered := RenderResumeState(state)
	for _, want := range []string{
		"mode: task_tree",
		"task tree snapshot: present",
		"task tree complete: false",
		"running nodes: root",
		"blocked nodes: blocked",
		"failed nodes: failed",
		"auto-continue: no",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered resume state missing %q:\n%s", want, rendered)
		}
	}
}

func TestDetectTaskTreeResumeStateKeepsLegacySessionNormal(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "legacy.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"old"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy session: %v", err)
	}

	snapshot, ok, err := DetectTaskTreeState(sessionPath)
	if err != nil {
		t.Fatalf("DetectTaskTreeState: %v", err)
	}
	if ok {
		t.Fatalf("legacy session should not have task-tree state: %+v", snapshot)
	}

	state, err := DetectTaskTreeResumeState(sessionPath, "")
	if err != nil {
		t.Fatalf("DetectTaskTreeResumeState: %v", err)
	}
	if state.Mode != ResumeModeNormal || state.HasSnapshot || state.HasTaskTree {
		t.Fatalf("legacy resume state = %+v", state)
	}
	if state.AutoContinue || state.ResumePolicy != ResumePolicyInspectOnly {
		t.Fatalf("legacy resume policy = %+v", state)
	}

	rendered := RenderResumeState(state)
	for _, want := range []string{"mode: normal", "task tree snapshot: absent", "task tree: absent", "auto-continue: no"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("legacy render missing %q:\n%s", want, rendered)
		}
	}
}

func TestDetectTaskTreeResumeStateKeepsExplicitTaskTreeModeWithoutSnapshot(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "missing-snapshot.jsonl")

	state, err := DetectTaskTreeResumeState(sessionPath, SnapshotModeTaskTree)
	if err != nil {
		t.Fatalf("DetectTaskTreeResumeState: %v", err)
	}
	if state.Mode != SnapshotModeTaskTree || state.HasSnapshot || state.HasTaskTree {
		t.Fatalf("task-tree mode without snapshot = %+v", state)
	}
	if state.AutoContinue {
		t.Fatalf("task-tree mode without snapshot must not auto-continue: %+v", state)
	}
}

func TestDetectTaskTreeStateCorruptSidecarReportsPresentError(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "corrupt.jsonl")
	sidecar := SnapshotPathForSession(sessionPath)
	if err := os.WriteFile(sidecar, []byte(`{"version":1,"mode":"task_tree","tree":{"status":"waiting"}}`), 0o644); err != nil {
		t.Fatalf("write corrupt sidecar: %v", err)
	}

	_, ok, err := DetectTaskTreeState(sessionPath)
	if err == nil {
		t.Fatal("DetectTaskTreeState corrupt sidecar error = nil")
	}
	if !ok {
		t.Fatal("corrupt sidecar should still report ok=true so callers can warn")
	}
	if !strings.Contains(err.Error(), "decode task tree snapshot") {
		t.Fatalf("corrupt sidecar error = %v", err)
	}

	state, err := DetectTaskTreeResumeState(sessionPath, "")
	if err == nil {
		t.Fatal("DetectTaskTreeResumeState corrupt sidecar error = nil")
	}
	if state.Mode != SnapshotModeTaskTree || !state.HasSnapshot || state.HasTaskTree {
		t.Fatalf("corrupt resume state = %+v", state)
	}
}
