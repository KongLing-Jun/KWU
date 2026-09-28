package tasktree

import "fmt"

// NodeContext is the isolated execution brief prepared for one task node.
type NodeContext struct {
	Goal              string             `json:"goal"`
	NodeID            string             `json:"node_id"`
	NodeTitle         string             `json:"node_title"`
	NodeDescription   string             `json:"node_description,omitempty"`
	NodePath          []string           `json:"node_path"`
	ParentConstraints []string           `json:"parent_constraints,omitempty"`
	DependencyResults []DependencyResult `json:"dependency_results,omitempty"`
	RelatedFiles      []string           `json:"related_files,omitempty"`
	ToolHints         []string           `json:"tool_hints,omitempty"`
	ExpectedOutput    string             `json:"expected_output,omitempty"`
}

// DependencyResult keeps the dependency node identity attached to its returned
// output so prompts can cite where the evidence came from.
type DependencyResult struct {
	NodeID       string         `json:"node_id"`
	NodeTitle    string         `json:"node_title"`
	Status       Status         `json:"status"`
	Summary      string         `json:"summary,omitempty"`
	Artifacts    []NodeArtifact `json:"artifacts,omitempty"`
	RawOutputRef string         `json:"raw_output_ref,omitempty"`
	Error        string         `json:"error,omitempty"`
	Blocker      string         `json:"blocker,omitempty"`
}

// BuildNodeContext returns a fresh context value for nodeID without mutating the
// task tree. Missing parent or dependency IDs are treated as invalid tree state.
func (t *TaskTree) BuildNodeContext(nodeID string) (NodeContext, error) {
	node, ok := t.Node(nodeID)
	if !ok {
		return NodeContext{}, fmt.Errorf("task node %q not found", nodeID)
	}
	path, constraints, err := t.nodePathAndParentConstraints(node)
	if err != nil {
		return NodeContext{}, err
	}
	deps, err := t.completedDependencyResults(node)
	if err != nil {
		return NodeContext{}, err
	}
	return NodeContext{
		Goal:              t.Goal,
		NodeID:            node.ID,
		NodeTitle:         node.Title,
		NodeDescription:   node.Description,
		NodePath:          path,
		ParentConstraints: constraints,
		DependencyResults: deps,
		RelatedFiles:      cloneStrings(node.RelatedFiles),
		ToolHints:         cloneStrings(node.ToolHints),
		ExpectedOutput:    node.ExpectedOutput,
	}, nil
}

func (t *TaskTree) nodePathAndParentConstraints(node *TaskNode) ([]string, []string, error) {
	var chain []*TaskNode
	seen := map[string]bool{}
	for cur := node; cur != nil; {
		if seen[cur.ID] {
			return nil, nil, fmt.Errorf("task node parent cycle at %q", cur.ID)
		}
		seen[cur.ID] = true
		chain = append(chain, cur)
		if cur.ParentID == "" {
			break
		}
		parent, ok := t.Node(cur.ParentID)
		if !ok {
			return nil, nil, fmt.Errorf("task parent %q for node %q not found", cur.ParentID, cur.ID)
		}
		cur = parent
	}

	path := make([]string, 0, len(chain))
	constraints := []string{}
	for i := len(chain) - 1; i >= 0; i-- {
		cur := chain[i]
		path = append(path, cur.Title)
		if cur.ID != node.ID {
			constraints = append(constraints, cur.Constraints...)
		}
	}
	return path, constraints, nil
}

func (t *TaskTree) completedDependencyResults(node *TaskNode) ([]DependencyResult, error) {
	out := []DependencyResult{}
	for _, depID := range node.DependsOn {
		dep, ok := t.Node(depID)
		if !ok {
			return nil, fmt.Errorf("task dependency %q for node %q not found", depID, node.ID)
		}
		if dep.Status != StatusCompleted || dep.Result == nil {
			continue
		}
		out = append(out, DependencyResult{
			NodeID:       dep.ID,
			NodeTitle:    dep.Title,
			Status:       dep.Status,
			Summary:      dep.Result.Summary,
			Artifacts:    cloneArtifacts(dep.Result.Artifacts),
			RawOutputRef: dep.Result.RawOutputRef,
			Error:        dep.Result.Error,
			Blocker:      dep.Result.Blocker,
		})
	}
	return out, nil
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
