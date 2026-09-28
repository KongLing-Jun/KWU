package tasktree

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	// ResumeModeNormal is the default for legacy sessions without mode metadata.
	ResumeModeNormal = "normal"
	// ResumePolicyInspectOnly records the first restore boundary: detect and
	// report task-tree state, but do not automatically continue execution.
	ResumePolicyInspectOnly = "inspect_only"
)

// DetectTaskTreeState reads the task-tree sidecar for a session. ok reports
// whether the sidecar exists, even when it cannot be decoded, so resume callers
// can distinguish legacy sessions from corrupt task-tree state.
func DetectTaskTreeState(sessionPath string) (Snapshot, bool, error) {
	path := SnapshotPathForSession(sessionPath)
	if path == "" {
		return Snapshot{}, false, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, true, fmt.Errorf("read task tree snapshot %s: %w", path, err)
	}
	snap, err := UnmarshalSnapshot(b)
	if err != nil {
		return Snapshot{}, true, fmt.Errorf("decode task tree snapshot %s: %w", path, err)
	}
	return snap, true, nil
}

// DetectTaskTreeResumeState returns the restore-time status a CLI, server, or
// controller can display before later versions decide whether to continue work.
func DetectTaskTreeResumeState(sessionPath, sessionMode string) (ResumeState, error) {
	snapshot, ok, err := DetectTaskTreeState(sessionPath)
	if err != nil {
		state := ResumeStateFromSnapshot(sessionMode, Snapshot{}, false)
		state.HasSnapshot = ok
		if ok && strings.TrimSpace(state.Mode) == ResumeModeNormal && strings.TrimSpace(sessionMode) == "" {
			state.Mode = SnapshotModeTaskTree
		}
		return state, err
	}
	return ResumeStateFromSnapshot(sessionMode, snapshot, ok), nil
}

// RenderResumeState provides a compact human-readable status line block for
// first-version resume surfaces.
func RenderResumeState(state ResumeState) string {
	mode := strings.TrimSpace(state.Mode)
	if mode == "" {
		mode = ResumeModeNormal
	}

	var b strings.Builder
	fmt.Fprintf(&b, "mode: %s\n", mode)
	if state.HasSnapshot {
		b.WriteString("task tree snapshot: present\n")
	} else {
		b.WriteString("task tree snapshot: absent\n")
	}

	if state.HasTaskTree {
		fmt.Fprintf(&b, "task tree: %s (%s)\n", state.TreeID, state.TreeStatus)
		fmt.Fprintf(&b, "task tree complete: %t\n", state.TreeComplete)
		writeResumeNodeLine(&b, "running nodes", state.RunningNodes)
		writeResumeNodeLine(&b, "blocked nodes", state.BlockedNodes)
		writeResumeNodeLine(&b, "failed nodes", state.FailedNodes)
	} else {
		b.WriteString("task tree: absent\n")
	}

	if state.AutoContinue {
		b.WriteString("auto-continue: yes\n")
	} else {
		b.WriteString("auto-continue: no; this version only reports resume state\n")
	}
	if state.ResumePolicy != "" {
		fmt.Fprintf(&b, "resume policy: %s\n", state.ResumePolicy)
	}
	return strings.TrimSpace(b.String())
}

func enrichResumeState(state ResumeState, tree *TaskTree) ResumeState {
	state.AutoContinue = false
	state.ResumePolicy = ResumePolicyInspectOnly
	if tree == nil {
		return state
	}
	state.RunningNodes = nodeIDsWithStatus(tree, StatusRunning)
	state.BlockedNodes = nodeIDsWithStatus(tree, StatusBlocked)
	state.FailedNodes = nodeIDsWithStatus(tree, StatusFailed)
	return state
}

func nodeIDsWithStatus(tree *TaskTree, status Status) []string {
	if tree == nil || len(tree.Nodes) == 0 {
		return nil
	}
	ids := make([]string, 0)
	for id, node := range tree.Nodes {
		if node != nil && node.Status == status {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func writeResumeNodeLine(b *strings.Builder, label string, ids []string) {
	if len(ids) == 0 {
		fmt.Fprintf(b, "%s: none\n", label)
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, strings.Join(ids, ", "))
}
