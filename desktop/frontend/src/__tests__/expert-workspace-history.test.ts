import assert from "node:assert/strict";

import { parallelTaskOutput, restoredParallelExpertItems } from "../lib/expertWorkspaceHistory";
import type { ToolItem } from "../features/session/sessionItems";

const parent: ToolItem = {
  kind: "tool",
  id: "parallel-history",
  name: "parallel_tasks",
  args: "",
  readOnly: false,
  status: "done",
  dataArchived: true,
};

const output = [
  "Completed 2 parallel tasks:",
  "── task-1 ──",
  "research answer",
  "── task-2 ──",
  "review answer",
  "[FAILED] review failed",
].join("\n");

assert.equal(parallelTaskOutput(output, 0), "research answer");
assert.equal(parallelTaskOutput(output, 1), "review answer\n[FAILED] review failed");

const restored = restoredParallelExpertItems(parent, {
  args: JSON.stringify({
    tasks: [
      { member_name: "Research Expert", description: "Collect evidence", prompt: "private research prompt" },
      { member_name: "Review Expert", description: "Review evidence", prompt: "private review prompt" },
    ],
  }),
  output,
});

assert.equal(restored.length, 2);
assert.equal(restored[0]?.id, "parallel-history/sub-1");
assert.equal(restored[0]?.parentId, "parallel-history");
assert.equal(JSON.parse(restored[0]?.args ?? "{}").member_name, "Research Expert");
assert.equal((restored[0]?.args ?? "").includes("private research prompt"), false, "restored UI metadata omits private prompts");
assert.equal(restored[0]?.output, "research answer");
assert.equal(restored[1]?.status, "error");

console.log("expert workspace history passed");
