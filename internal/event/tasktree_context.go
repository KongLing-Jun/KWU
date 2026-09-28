package event

import (
	"context"
	"strings"
)

type taskTreeContextKey struct{}

// TaskTreeContext identifies the task-tree node currently owning nested agent
// output. It lets sinks associate sub-agent text and tool activity with a node
// without changing ordinary event flow.
type TaskTreeContext struct {
	TreeID string
	NodeID string
}

// WithTaskTreeContext carries task-tree ownership through tool and sub-agent
// calls. Blank tree or node IDs leave ctx unchanged.
func WithTaskTreeContext(ctx context.Context, treeID, nodeID string) context.Context {
	treeID = strings.TrimSpace(treeID)
	nodeID = strings.TrimSpace(nodeID)
	if treeID == "" || nodeID == "" {
		return ctx
	}
	return context.WithValue(ctx, taskTreeContextKey{}, TaskTreeContext{TreeID: treeID, NodeID: nodeID})
}

// TaskTreeContextFromContext returns the current task-tree ownership context.
func TaskTreeContextFromContext(ctx context.Context) (TaskTreeContext, bool) {
	tc, ok := ctx.Value(taskTreeContextKey{}).(TaskTreeContext)
	if !ok || strings.TrimSpace(tc.TreeID) == "" || strings.TrimSpace(tc.NodeID) == "" {
		return TaskTreeContext{}, false
	}
	tc.TreeID = strings.TrimSpace(tc.TreeID)
	tc.NodeID = strings.TrimSpace(tc.NodeID)
	return tc, true
}
