// Run: tsx src/__tests__/task-tree-state.test.ts

import {
  applyTaskTreeEvent,
  initialTaskTreeState,
  taskTreeDisplayModel,
  taskTreeDisplayRows,
} from "../lib/taskTreeState";
import { initialState, reducer, type Item } from "../lib/useController";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq<T>(actual: T, expected: T, label: string) {
  ok(actual === expected, `${label}${actual === expected ? "" : `: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`}`);
}

function toolItems(items: Item[]) {
  return items.filter((item): item is Extract<Item, { kind: "tool" }> => item.kind === "tool");
}

console.log("\ntask tree frontend state contract");

{
  let state = applyTaskTreeEvent(initialTaskTreeState, {
    kind: "task_tree_created",
    taskTree: {
      treeId: "tree-1",
      title: "Implement feature",
      status: "running",
      rootIds: ["root"],
      nodes: [
        { nodeId: "root", title: "Plan", status: "pending" },
        { nodeId: "child", title: "Build", status: "pending", parentId: "root", dependsOn: ["root"] },
      ],
    },
  });
  const tree = state.trees["tree-1"];
  eq(state.activeTreeId, "tree-1", "task tree state is rooted by tree id");
  eq(tree.nodes.child.parentId, "root", "node keeps parent relationship");
  eq(tree.nodes.child.dependsOn[0], "root", "node keeps dependency list");
  const rows = taskTreeDisplayRows(tree);
  eq(rows.length, 2, "display rows include the task tree hierarchy");
  eq(rows[0].nodeId, "root", "root node renders first");
  eq(rows[1].depth, 1, "child node renders one level deeper");

  state = applyTaskTreeEvent(state, {
    kind: "task_node_started",
    taskTree: { treeId: "tree-1", nodeId: "child" },
  });
  eq(state.trees["tree-1"].nodes.child.status, "running", "node status updates by node id");
  eq(state.trees["tree-1"].currentNodeIds[0], "child", "running node is exposed for minimal display");

  state = applyTaskTreeEvent(state, {
    kind: "task_node_output",
    taskTree: { treeId: "tree-1", nodeId: "child", outputDelta: "created files" },
  });
  state = applyTaskTreeEvent(state, {
    kind: "task_node_output",
    taskTree: { treeId: "tree-1", nodeId: "child", outputDelta: "\nran checks" },
  });
  eq(state.trees["tree-1"].nodes.child.output, "created files\nran checks", "node output appends by node id");
  eq(state.trees["tree-1"].nodes.child.outputDeltas.length, 2, "node output deltas remain available");

  state = applyTaskTreeEvent(state, {
    kind: "task_node_failed",
    taskTree: { treeId: "tree-1", nodeId: "child", error: "blocked by missing input" },
  });
  eq(state.trees["tree-1"].nodes.child.error, "blocked by missing input", "node error or blocker reason is stored");
  eq(state.trees["tree-1"].currentNodeIds.length, 0, "failed nodes are no longer reported as running");

  state = applyTaskTreeEvent(state, {
    kind: "task_tree_summary",
    taskTree: { treeId: "tree-1", status: "failed", summary: "Feature is blocked on input" },
  });
  const display = taskTreeDisplayModel(state.trees["tree-1"]);
  eq(display.summary, "Feature is blocked on input", "final summary is exposed for display");
  eq(display.status, "failed", "summary event updates the tree status");
}

{
  let state = applyTaskTreeEvent(initialTaskTreeState, {
    kind: "task_tree_created",
    taskTree: {
      treeId: "tree-tools",
      rootIds: ["node-a"],
      nodes: [{ nodeId: "node-a", title: "Use tools", status: "running" }],
    },
  });
  state = applyTaskTreeEvent(state, {
    kind: "tool_dispatch",
    tool: { id: "tool-1", name: "bash", args: "echo hi", readOnly: false, taskTreeId: "tree-tools", nodeId: "node-a" },
  });
  state = applyTaskTreeEvent(state, {
    kind: "tool_progress",
    tool: { id: "tool-1", name: "bash", readOnly: false, output: "hi\n", taskTreeId: "tree-tools", nodeId: "node-a" },
  });
  const node = state.trees["tree-tools"].nodes["node-a"];
  eq(node.toolIds[0], "tool-1", "tool activity is associated with the owning node");
  eq(node.output, "", "tool output does not overwrite node output");
}

{
  const state = applyTaskTreeEvent(initialTaskTreeState, { kind: "message", text: "normal assistant message" });
  eq(state, initialTaskTreeState, "ordinary message events do not change task tree state");
}

{
  let state = reducer(initialState, { type: "user", text: "please do it", seq: 0 });
  const itemCount = state.items.length;
  state = reducer(state, {
    type: "event",
    e: {
      kind: "task_tree_created",
      taskTree: {
        treeId: "tree-ui",
        rootIds: ["root"],
        nodes: [{ nodeId: "root", title: "UI contract", status: "running" }],
      },
    },
  });
  eq(state.pendingUser, "please do it", "task tree event does not confirm the optimistic user bubble");
  eq(state.items.length, itemCount, "task tree event does not add or replace transcript items");
  eq(state.taskTrees.activeTreeId, "tree-ui", "task tree event still updates the side state");
}

{
  let state = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
  state = reducer(state, {
    type: "event",
    e: {
      kind: "task_tree_created",
      taskTree: {
        treeId: "tree-tool-items",
        rootIds: ["node-a"],
        nodes: [{ nodeId: "node-a", title: "Node A", status: "running" }],
      },
    },
  });
  state = reducer(state, {
    type: "event",
    e: {
      kind: "tool_dispatch",
      tool: { id: "tool-item", name: "bash", args: "pwd", readOnly: false, taskTreeId: "tree-tool-items", nodeId: "node-a" },
    },
  });
  const tools = toolItems(state.items);
  eq(tools[0]?.taskTreeId, "tree-tool-items", "tool item keeps task tree id metadata");
  eq(tools[0]?.nodeId, "node-a", "tool item keeps node id metadata");
  eq(state.taskTrees.trees["tree-tool-items"].nodes["node-a"].toolIds[0], "tool-item", "controller state links tools to nodes");
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
