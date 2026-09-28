import assert from "node:assert/strict";
import { parseTaskDispatchOutcome } from "../lib/taskDispatch";

for (const status of ["dispatch_decision_required", "dependency_not_satisfied", "capacity_wait_required"] as const) {
  const parsed = parseTaskDispatchOutcome(JSON.stringify({
    status,
    running_task_runs: ["task-1"],
    depends_on: ["task-0"],
    message: "internal scheduling guidance",
  }));
  assert.equal(parsed?.status, status);
  assert.deepEqual(parsed?.runningTaskRuns, ["task-1"]);
  assert.deepEqual(parsed?.dependsOn, ["task-0"]);
  assert.equal("subagentRef" in (parsed ?? {}), false, "non-dispatch outcome cannot create child navigation identity");
}

assert.equal(parseTaskDispatchOutcome(JSON.stringify({ subagentRef: "sa_real", turnStatus: "completed" })), null);
assert.equal(parseTaskDispatchOutcome("not json"), null);

console.log("task dispatch presentation contract passed");
