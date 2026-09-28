// Run: tsx src/__tests__/readiness-candidate-presentation.test.ts

import { reducer } from "../features/session/useController";
import { initialState, type State } from "../features/session/sessionState";
import type { Item } from "../features/session/sessionItems";

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

console.log("\nreadiness candidate presentation");

const previous: Item = { kind: "assistant", id: "accepted-previous", text: "previous answer", reasoning: "", streaming: false };
const tool: Item = { kind: "tool", id: "tool-1", name: "write_file", args: "{}", readOnly: false, status: "done" };
const child: Item = { kind: "assistant", id: "child-1", text: "child result", reasoning: "", streaming: false };
let state: State = {
  ...initialState,
  items: [previous, tool],
  childItemsByRef: { sa_child: [child] },
};
state = reducer(state, { type: "event", e: { kind: "turn_started" } });
state = reducer(state, { type: "event", e: { kind: "text", text: "premature candidate" } });
state = reducer(state, { type: "event", e: { kind: "message", text: "premature candidate" } });
ok(state.items.some((item) => item.kind === "assistant" && item.text === "premature candidate"), "candidate may be visible before readiness rejection");

state = reducer(state, { type: "event", e: { kind: "final_candidate_rejected" } });
ok(!state.items.some((item) => item.kind === "assistant" && item.text === "premature candidate"), "rejection retracts current parent candidate");
ok(state.items.some((item) => item.id === "accepted-previous"), "retraction preserves prior accepted assistant replies");
ok(state.items.some((item) => item.id === "tool-1"), "retraction preserves tool cards");
ok(state.childItemsByRef.sa_child?.[0]?.id === "child-1", "retraction preserves isolated child history");
ok(state.running && state.turnActive, "rejection keeps the parent turn active");

state = reducer(state, { type: "event", e: { kind: "notice", level: "warn", text: "final-answer readiness blocked: missing check" } });
state = reducer(state, { type: "event", e: { kind: "text", text: "accepted final" } });
state = reducer(state, { type: "event", e: { kind: "message", text: "accepted final" } });
state = reducer(state, { type: "event", e: { kind: "turn_done" } });
ok(state.items.filter((item) => item.kind === "assistant" && item.text === "accepted final").length === 1, "accepted final remains exactly once");
ok(!JSON.stringify(state.items).includes("premature candidate"), "copy and user-visible export input excludes rejected candidate");

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}

process.stdout.write(`\n${passed} tests passed.\n`);
