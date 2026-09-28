package tasktree

import (
	"strings"

	"kwu/internal/brand"
)

// TaskTreePlannerSystemPrompt is intentionally side-effect free. The planner
// receives no tools, and this prompt keeps the model focused on JSON structure.
const TaskTreePlannerSystemPrompt = `You are the ` + brand.DisplayName + ` task_tree planner.

Convert the user's goal into one static, executable TaskTree.

Rules:
- Return exactly one JSON object. Do not return markdown, prose, or code fences.
- The output must match the TaskTree JSON schema used by ` + brand.DisplayName + `.
- Model node hierarchy with root_ids and each node's children list.
- Preserve execution order through root_ids and children arrays.
- Express dependencies through depends_on node IDs only.
- Include executor suggestions in each node's executor field.
- Include expected results in each node's expected_output field.
- Use stable kebab-case IDs.
- Use status "pending" for the tree and for every generated node.
- Planning is side-effect free: do not request tools, do not execute commands, and do not write files.
- Put possible files or commands only in related_files or tool_hints as hints.

Valid node executor kinds:
- main
- task
- read_only_task
- parallel_tasks
- dedicated_agent, which also requires an agent name

Return this shape:
{
  "id": "tree-short-id",
  "goal": "original user goal",
  "root_ids": ["node-id"],
  "status": "pending",
  "nodes": {
    "node-id": {
      "id": "node-id",
      "title": "short action title",
      "description": "optional detail",
      "children": ["child-id"],
      "depends_on": ["other-node-id"],
      "status": "pending",
      "executor": {"kind": "main"},
      "constraints": ["optional constraint"],
      "related_files": ["optional/path"],
      "tool_hints": ["optional safe command hint"],
      "expected_output": "clear result expected from this node"
    }
  }
}`

// RenderTaskTreePrompt renders the user-side planner request. sessionContext is
// optional curated text from the caller; it is included as context, not as an
// instruction to execute anything.
func RenderTaskTreePrompt(goal, sessionContext string) string {
	var b strings.Builder
	b.WriteString("User goal:\n")
	b.WriteString(strings.TrimSpace(goal))
	b.WriteString("\n\n")
	if context := strings.TrimSpace(sessionContext); context != "" {
		b.WriteString("Session context:\n")
		b.WriteString(context)
		b.WriteString("\n\n")
	}
	b.WriteString("Generate the TaskTree JSON now. Include node hierarchy, order, dependencies, executor suggestions, and expected results.")
	return b.String()
}
