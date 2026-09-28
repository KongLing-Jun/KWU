package event

const (
	TaskTreeStatusPending   = "pending"
	TaskTreeStatusReady     = "ready"
	TaskTreeStatusRunning   = "running"
	TaskTreeStatusCompleted = "completed"
	TaskTreeStatusFailed    = "failed"
	TaskTreeStatusBlocked   = "blocked"
	TaskTreeStatusSkipped   = "skipped"
)

// TaskTreeNodePayload describes a task tree node in a frontend-friendly shape.
// It is intentionally string-based so the event package stays independent from
// the task tree executor package while preserving stable wire values.
type TaskTreeNodePayload struct {
	NodeID    string   `json:"nodeId"`
	Status    string   `json:"status,omitempty"`
	Title     string   `json:"title,omitempty"`
	ParentID  string   `json:"parentId,omitempty"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

// TaskTreePayload is the structured payload for all task-tree event kinds.
// Node lifecycle events populate TreeID and NodeID, while tree-level events may
// also include RootIDs, Nodes, Status, and Summary.
type TaskTreePayload struct {
	TreeID      string                `json:"treeId"`
	NodeID      string                `json:"nodeId,omitempty"`
	Status      string                `json:"status,omitempty"`
	Title       string                `json:"title,omitempty"`
	ParentID    string                `json:"parentId,omitempty"`
	Result      string                `json:"result,omitempty"`
	Error       string                `json:"error,omitempty"`
	OutputDelta string                `json:"outputDelta,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	RootIDs     []string              `json:"rootIds,omitempty"`
	Nodes       []TaskTreeNodePayload `json:"nodes,omitempty"`
}
