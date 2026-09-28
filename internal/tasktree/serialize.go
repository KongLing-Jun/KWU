package tasktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/fileutil"
)

const (
	// SnapshotVersion is the first stable JSON contract for task-tree state.
	SnapshotVersion = 1
	// SnapshotModeTaskTree mirrors the session agent mode that owns this state.
	SnapshotModeTaskTree = "task_tree"
	// SnapshotSidecarSuffix is the first storage entry point beside session JSONL.
	SnapshotSidecarSuffix = ".tasktree.json"
)

// Snapshot is the mode-specific state stored outside the normal session message
// log. Session messages remain newline-delimited provider.Message values managed
// by agent.Session; this snapshot is the structured state a task_tree resume
// path can load from session metadata or a sidecar file.
type Snapshot struct {
	Version int       `json:"version"`
	Mode    string    `json:"mode"`
	Tree    *TaskTree `json:"tree,omitempty"`
}

// ResumeState is the small restore-time summary needed before an orchestrator
// decides whether to continue, rebuild, or finish a task tree.
type ResumeState struct {
	Mode         string   `json:"mode"`
	HasSnapshot  bool     `json:"has_snapshot"`
	HasTaskTree  bool     `json:"has_task_tree"`
	TreeID       string   `json:"tree_id,omitempty"`
	TreeStatus   Status   `json:"tree_status,omitempty"`
	TreeComplete bool     `json:"tree_complete"`
	RunningNodes []string `json:"running_nodes,omitempty"`
	BlockedNodes []string `json:"blocked_nodes,omitempty"`
	FailedNodes  []string `json:"failed_nodes,omitempty"`
	AutoContinue bool     `json:"auto_continue"`
	ResumePolicy string   `json:"resume_policy,omitempty"`
}

// MarshalSnapshot serializes a task-tree snapshot as stable, indented JSON.
func MarshalSnapshot(s Snapshot) ([]byte, error) {
	s = normalizeSnapshot(s)
	if err := validateSnapshot(s); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// UnmarshalSnapshot decodes and validates a task-tree snapshot.
func UnmarshalSnapshot(b []byte) (Snapshot, error) {
	if len(strings.TrimSpace(string(b))) == 0 {
		return Snapshot{}, fmt.Errorf("empty task tree snapshot")
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, err
	}
	s = normalizeSnapshot(s)
	if err := validateSnapshot(s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

// SnapshotPathForSession returns the first-version sidecar path for a session
// transcript. A future implementation may store the same Snapshot shape inside
// session metadata instead; callers should treat this as the current boundary,
// not as the only possible storage backend.
func SnapshotPathForSession(sessionPath string) string {
	sessionPath = strings.TrimSpace(sessionPath)
	if sessionPath == "" {
		return ""
	}
	return sessionPath + SnapshotSidecarSuffix
}

// SaveSnapshotForSession persists mode-specific task-tree state beside the
// session transcript without changing the message log.
func SaveSnapshotForSession(sessionPath string, s Snapshot) error {
	path := SnapshotPathForSession(sessionPath)
	if path == "" {
		return fmt.Errorf("empty session path")
	}
	b, err := MarshalSnapshot(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tasktree.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := fileutil.ReplaceFile(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// LoadSnapshotForSession loads the task-tree state sidecar. ok is false for
// legacy sessions or normal-mode sessions that have no task-tree state.
func LoadSnapshotForSession(sessionPath string) (Snapshot, bool, error) {
	s, ok, err := DetectTaskTreeState(sessionPath)
	if err != nil {
		return Snapshot{}, false, err
	}
	return s, ok, nil
}

// ResumeStateFromSnapshot summarizes a loaded snapshot. ok should be the boolean
// returned by LoadSnapshotForSession; when ok is false, mode can come from
// session metadata or request/config resolution and HasTaskTree stays false.
func ResumeStateFromSnapshot(mode string, s Snapshot, ok bool) ResumeState {
	mode = strings.TrimSpace(mode)
	if ok && strings.TrimSpace(s.Mode) != "" {
		mode = strings.TrimSpace(s.Mode)
	}
	if mode == "" {
		mode = ResumeModeNormal
	}
	state := ResumeState{
		Mode:         mode,
		HasSnapshot:  ok,
		AutoContinue: false,
		ResumePolicy: ResumePolicyInspectOnly,
	}
	if !ok || s.Tree == nil {
		return state
	}
	state.HasTaskTree = true
	state.TreeID = s.Tree.ID
	state.TreeStatus = s.Tree.Status
	state.TreeComplete = s.Tree.Status == StatusCompleted
	return enrichResumeState(state, s.Tree)
}

func normalizeSnapshot(s Snapshot) Snapshot {
	if s.Version == 0 {
		s.Version = SnapshotVersion
	}
	s.Mode = strings.TrimSpace(s.Mode)
	if s.Mode == "" {
		s.Mode = SnapshotModeTaskTree
	}
	return s
}

func validateSnapshot(s Snapshot) error {
	if s.Version != SnapshotVersion {
		return fmt.Errorf("unsupported task tree snapshot version %d", s.Version)
	}
	if strings.TrimSpace(s.Mode) == "" {
		return fmt.Errorf("task tree snapshot mode is required")
	}
	if s.Tree != nil {
		if !s.Tree.Status.Valid() {
			return fmt.Errorf("%w: tree %q has %q", ErrInvalidStatus, s.Tree.ID, s.Tree.Status)
		}
		for id, node := range s.Tree.Nodes {
			if node == nil {
				return fmt.Errorf("task node %q is nil", id)
			}
			if !node.Status.Valid() {
				return fmt.Errorf("%w: node %q has %q", ErrInvalidStatus, id, node.Status)
			}
			if node.Executor != nil && !node.Executor.Valid() {
				return fmt.Errorf("invalid task executor for node %q: %+v", id, node.Executor)
			}
		}
	}
	return nil
}
