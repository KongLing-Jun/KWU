package tasktree

import (
	"fmt"
	"strings"
)

// ExecutorKind names the orchestration path expected to handle a node.
type ExecutorKind string

const (
	ExecutorMain           ExecutorKind = "main"
	ExecutorTask           ExecutorKind = "task"
	ExecutorReadOnlyTask   ExecutorKind = "read_only_task"
	ExecutorParallelTasks  ExecutorKind = "parallel_tasks"
	ExecutorDedicatedAgent ExecutorKind = "dedicated_agent"
)

// ExecutorSpec marks how a node should be executed. Dedicated agents carry a
// stable name so future agent implementations can be referenced without adding
// more fields to TaskNode.
type ExecutorSpec struct {
	Kind  ExecutorKind `json:"kind"`
	Agent string       `json:"agent,omitempty"`
}

func (k ExecutorKind) Valid() bool {
	switch k {
	case ExecutorMain, ExecutorTask, ExecutorReadOnlyTask, ExecutorParallelTasks, ExecutorDedicatedAgent:
		return true
	default:
		return false
	}
}

func (e ExecutorSpec) Valid() bool {
	if !e.Kind.Valid() {
		return false
	}
	return e.Kind != ExecutorDedicatedAgent || strings.TrimSpace(e.Agent) != ""
}

// AssignExecutor validates and stores the executor expected to process a node.
func (n *TaskNode) AssignExecutor(executor ExecutorSpec) error {
	if n == nil {
		return fmt.Errorf("task node is nil")
	}
	if !executor.Valid() {
		return fmt.Errorf("invalid task executor: %+v", executor)
	}
	copied := executor
	n.Executor = &copied
	return nil
}

// NodeResult is the orchestration handoff returned by an executor or sub-agent.
// Summary is suitable for user-facing display; RawOutputRef points at internal
// transcript or artifact storage when the full raw output must be retained.
type NodeResult struct {
	Summary      string         `json:"summary,omitempty"`
	Artifacts    []NodeArtifact `json:"artifacts,omitempty"`
	RawOutputRef string         `json:"raw_output_ref,omitempty"`
	Error        string         `json:"error,omitempty"`
	Blocker      string         `json:"blocker,omitempty"`
}

// NodeArtifact describes structured output produced for a node.
type NodeArtifact struct {
	Type string         `json:"type"`
	Name string         `json:"name,omitempty"`
	Ref  string         `json:"ref,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

func (r NodeResult) UserVisibleSummary() string {
	return r.Summary
}

func (r NodeResult) targetStatus() Status {
	switch {
	case strings.TrimSpace(r.Blocker) != "":
		return StatusBlocked
	case strings.TrimSpace(r.Error) != "":
		return StatusFailed
	default:
		return StatusCompleted
	}
}

// RecordResult validates and writes an executor result back to a task node.
func (t *TaskTree) RecordResult(nodeID string, result NodeResult) error {
	node, ok := t.Node(nodeID)
	if !ok {
		return fmt.Errorf("task node %q not found", nodeID)
	}
	next := result.targetStatus()
	if err := node.TransitionTo(next); err != nil {
		return err
	}
	copied := result
	copied.Artifacts = cloneArtifacts(result.Artifacts)
	node.Result = &copied
	return nil
}

func cloneArtifacts(in []NodeArtifact) []NodeArtifact {
	if len(in) == 0 {
		return nil
	}
	out := make([]NodeArtifact, len(in))
	for i, artifact := range in {
		out[i] = artifact
		if artifact.Data != nil {
			out[i].Data = make(map[string]any, len(artifact.Data))
			for k, v := range artifact.Data {
				out[i].Data[k] = v
			}
		}
	}
	return out
}
