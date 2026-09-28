// Run: tsx src/__tests__/event-handling-contract.test.ts

import { EVENT_HANDLING_OWNER, eventHandlingOwner } from "../lib/eventHandling";
import type { EventKind, WireEvent } from "../lib/types";
import { initialState, reducer } from "../lib/useController";

let passed = 0;
let failed = 0;

function eq(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

console.log("\nevent handling contract");

eq(eventHandlingOwner("text"), "controller", "conversation events belong to the controller");
eq(eventHandlingOwner("final_candidate_rejected"), "controller", "final candidate rejection belongs to the conversation controller");
eq(eventHandlingOwner("task_node_updated"), "task-tree", "task payload events belong to task-tree state");
eq(eventHandlingOwner("mcp_surface_ready"), "background", "MCP readiness is background metadata");
eq(eventHandlingOwner("audit"), "background", "audit events are background metadata");

const sent = reducer({ ...initialState }, { type: "user", text: "hello", seq: 0 });
const afterAudit = reducer(sent, { type: "event", e: { kind: "audit" } as WireEvent });
eq(afterAudit, sent, "background audit metadata does not confirm an optimistic submit");

const afterStatus = reducer(sent, {
  type: "event",
  e: { kind: "subagent_status_changed", subagentRef: "sa-fast", subagentStatus: "completed" },
});
eq(afterStatus.subagentStatusByRef["sa-fast"], "completed", "status signal is retained before a parent task card exists");
eq(afterStatus.pendingLocalUser, sent.pendingLocalUser, "status signal does not confirm an optimistic submit");
const afterRedispatch = reducer(afterStatus, {
  type: "event",
  e: { kind: "subagent_status_changed", subagentRef: "sa-fast", subagentStatus: "running" },
});
eq(afterRedispatch.subagentStatusByRef["sa-fast"], "running", "redispatch replaces the previous terminal signal with running");
const afterStoppedDiscard = reducer({ ...initialState, discardTurn: true }, {
  type: "event",
  e: { kind: "subagent_status_changed", subagentRef: "sa-stopped", subagentStatus: "interrupted" },
});
eq(afterStoppedDiscard.subagentStatusByRef["sa-stopped"], "interrupted", "stopped child status survives a discarded parent turn");

const afterParallel = reducer(afterStatus, {
  type: "event",
  e: { kind: "tool_dispatch", tool: { id: "parallel", name: "parallel_tasks", args: "{}", readOnly: false } },
});
eq(afterParallel.subagentStatusByRef, afterStatus.subagentStatusByRef, "parallel task projection does not create a persistent member status");

const allKinds: EventKind[] = Object.keys(EVENT_HANDLING_OWNER) as EventKind[];
eq(new Set(allKinds).size, allKinds.length, "event handling registry has no duplicate keys");

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}

process.stdout.write(`\n${passed} tests passed.\n`);
