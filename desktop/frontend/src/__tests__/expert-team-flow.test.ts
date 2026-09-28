// Run: tsx src/__tests__/expert-team-flow.test.ts

import { buildExpertTeamFlowModel, isExpertDelegationTool } from "../lib/expertTeamFlow";
import { applyTaskTreeEvent, initialTaskTreeState } from "../lib/taskTreeState";
import type { Item } from "../lib/useController";

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

console.log("\nexpert team flow contract");

{
  const model = buildExpertTeamFlowModel({
    taskTrees: initialTaskTreeState,
    items: [],
    activeExpert: null,
  });
  eq(model, null, "empty task tree state does not create a fake expert team flow");
}

{
  const model = buildExpertTeamFlowModel({
    taskTrees: initialTaskTreeState,
    items: [{ kind: "user", id: "u1", text: "Please turn customer feedback into service and growth actions.", createdAt: 100 }],
    activeExpert: null,
  });
  eq(model, null, "plain chat without an expert team does not create pending flow");
}

{
  const model = buildExpertTeamFlowModel({
    taskTrees: initialTaskTreeState,
    items: [{ kind: "user", id: "u1", text: "Please turn customer feedback into service and growth actions.", createdAt: 100 }],
    activeExpert: { id: "ops-team", name: "Customer Ops Team", role: "customer service", mode: "team", selectedAt: 200 },
  });
  eq(model, null, "team selection does not create pending flow from older history");
}

{
  const model = buildExpertTeamFlowModel({
    taskTrees: initialTaskTreeState,
    items: [{ kind: "user", id: "u1", text: "Please turn customer feedback into service and growth actions.", createdAt: 300 }],
    activeExpert: { id: "ops-team", name: "Customer Ops Team", role: "customer service", mode: "team", selectedAt: 200 },
  });
  eq(model, null, "team mode without backend task tree does not create a frontend static planning tree");
}

{
  const model = buildExpertTeamFlowModel({
    taskTrees: initialTaskTreeState,
    items: [{ kind: "user", id: "u1", text: "Please review this implementation.", createdAt: 300 }],
    activeExpert: { id: "reviewer", name: "Reviewer", role: "engineering", mode: "expert", selectedAt: 200 },
  });
  eq(model, null, "single expert mode does not show expert-team planning flow");
}

{
  const taskTrees = applyTaskTreeEvent(initialTaskTreeState, {
    kind: "task_tree_created",
    taskTree: {
      treeId: "tree-default-team",
      title: "Default team",
      status: "running",
      rootIds: ["root"],
      nodes: [{ nodeId: "root", title: "Root", status: "running" }],
    },
  });
  const model = buildExpertTeamFlowModel({ taskTrees, items: [], activeExpert: null });
  eq(model?.teamName, "AI 专家团", "missing active expert falls back to a generic expert team name");
}

{
  let taskTrees = applyTaskTreeEvent(initialTaskTreeState, {
    kind: "task_tree_created",
    taskTree: {
      treeId: "tree-experts",
      title: "Launch plan",
      status: "running",
      rootIds: ["goal"],
      nodes: [
        { nodeId: "goal", title: "Break down goal", status: "completed" },
        {
          nodeId: "research",
          title: "Research constraints",
          status: "running",
          parentId: "goal",
          dependsOn: ["goal"],
          goal: "Find launch risks",
          expertName: "Backend elected expert",
          expertRole: "critic",
          contextExpectation: "Use repo constraints",
          outputExpectation: "Return risk list",
          completionPercent: 62,
          confidencePercent: 71,
        },
        { nodeId: "summary", title: "Summarize result", status: "pending", parentId: "goal", dependsOn: ["research"] },
      ],
    },
  });
  taskTrees = applyTaskTreeEvent(taskTrees, {
    kind: "tool_dispatch",
    tool: {
      id: "delegate-1",
      name: "task",
      args: "Research constraints",
      readOnly: false,
      taskTreeId: "tree-experts",
      nodeId: "research",
      profile: { model: "subagent-model", effort: "high" },
    },
  });

  const items: Item[] = [
    {
      kind: "tool",
      id: "delegate-1",
      name: "task",
      args: "Research constraints",
      readOnly: false,
      status: "running",
      subject: "research agent",
      taskTreeId: "tree-experts",
      nodeId: "research",
      profile: { model: "subagent-model", effort: "high" },
    },
    {
      kind: "tool",
      id: "nested-1",
      name: "bash",
      args: "pwd",
      readOnly: true,
      status: "done",
      parentId: "delegate-1",
      taskTreeId: "tree-experts",
      nodeId: "research",
    },
  ];

  const model = buildExpertTeamFlowModel({
    taskTrees,
    items,
    activeExpert: { id: "team-1", name: "Product Team", role: "planner", mode: "team" },
  });

  ok(Boolean(model), "active task tree creates an expert team flow");
  eq(model?.teamName, "Product Team", "team mode keeps the selected team name");
  eq(model?.stage, "executing", "running node maps to executing stage");
  eq(model?.activeNodeIds[0], "research", "active node ids come from task tree display state");
  eq(model?.nodes.length, 3, "task nodes keep the task tree hierarchy");
  eq(model?.nodes[1]?.depth, 1, "child task node keeps depth");
  eq(model?.nodes[1]?.goal, "Find launch risks", "backend task goal is preserved");
  eq(model?.nodes[1]?.expertName, "Backend elected expert", "backend expert election wins over tool subject");
  eq(model?.nodes[1]?.expertRole, "critic", "backend expert role is preserved");
  eq(model?.nodes[1]?.contextExpectation, "Use repo constraints", "backend context expectation is preserved");
  eq(model?.nodes[1]?.outputExpectation, "Return risk list", "backend output expectation is preserved");
  eq(model?.nodes[1]?.tools.length, 2, "node keeps both delegate and nested tool activity");
  eq(model?.nodes[1]?.tools[0]?.delegated, true, "sub-agent task tool is marked as delegation");
  eq(model?.nodes[1]?.tools[0]?.model, "subagent-model", "sub-agent profile is preserved for future display");
  eq(model?.nodes[0]?.verification.completionPercent, 100, "completed task reports full completion");
  eq(model?.nodes[1]?.verification.completionPercent, 62, "backend completion percent wins over status heuristic");
  eq(model?.nodes[1]?.verification.confidencePercent, 71, "backend confidence percent is preserved");
  eq(model?.nodes[1]?.verification.confidenceSource, "backend", "confidence source marks backend-provided confidence");
  eq(model?.nodes[2]?.verification.confidenceSource, "not_available", "confidence is not fabricated before backend provides it");
}

{
  ok(isExpertDelegationTool("review"), "review is treated as delegation tooling");
  ok(isExpertDelegationTool("parallel_tasks"), "parallel_tasks is treated as delegation tooling");
  ok(!isExpertDelegationTool("bash"), "ordinary tools are not treated as delegation tooling");
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
