// Run: tsx src/__tests__/operational-prompt-reducer.test.ts

import { initialPromptProjection } from "../features/session/promptProjection";
import { reducer } from "../features/session/useController";
import { initialState } from "../features/session/sessionState";
import type { PromptLifecycle, WireEvent, WireOperationalEnvelope } from "../lib/types";

let passed = 0;
let failed = 0;
function eq(actual: unknown, expected: unknown, label: string) {
  if (JSON.stringify(actual) === JSON.stringify(expected)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

function operational(sequence: number, kind: WireOperationalEnvelope["kind"], lifecycle: PromptLifecycle): WireEvent {
  return {
    kind: "operational",
    tabId: "tab-dashboard",
    operational: {
      schemaVersion: 1,
      eventId: `event-${sequence}`,
      operationId: "operation-dashboard",
      causationId: sequence > 1 ? `event-${sequence - 1}` : undefined,
      aggregate: { kind: "prompt", id: "prompt-dashboard", sequence },
      scope: { tabId: "tab-dashboard", sessionId: "session-dashboard", promptId: "prompt-dashboard" },
      kind,
      prompt: {
        kind: "approval",
        lifecycle,
        approval: { id: "prompt-dashboard", tool: "dashboard_add_widget", subject: "add widget" },
      },
    },
  };
}

let state = { ...initialState, prompts: initialPromptProjection("tab-dashboard") };
state = reducer(state, { type: "event", e: operational(1, "prompt.registered", "registered") });
state = reducer(state, { type: "event", e: operational(2, "prompt.emitted", "pending") });
state = reducer(state, {
  type: "event",
  e: {
    kind: "approval_request",
    tabId: "tab-dashboard",
    eventId: "event-2",
    approval: { id: "prompt-dashboard", tool: "dashboard_add_widget", subject: "add widget" },
  },
});
eq(state.approval?.id, "prompt-dashboard", "paired operational and legacy events expose one ApprovalModal input");
eq(Object.keys(state.prompts.prompts), ["prompt-dashboard"], "paired delivery keeps one canonical prompt identity");
eq(state.prompts.prompts["prompt-dashboard"]?.authority, "operational", "legacy delivery cannot replace operational authority");

state = reducer(state, { type: "prompt_reply_started", promptId: "prompt-dashboard", input: { kind: "approval", allow: true, session: false, persist: false } });
state = reducer(state, { type: "prompt_reply_receipt", receipt: { promptId: "prompt-dashboard", status: "accepted" } });
eq(state.approval?.id, "prompt-dashboard", "accepted receipt keeps prompt visible until terminal fact");
eq(state.prompts.prompts["prompt-dashboard"]?.lifecycle, "replying", "accepted receipt preserves replying state");

state = reducer(state, { type: "event", e: operational(3, "prompt.resolved", "resolved") });
eq(state.approval, undefined, "terminal operational fact clears ApprovalModal input");
eq(state.pendingPrompt, false, "terminal operational fact clears pending prompt state");

state = reducer(state, { type: "event", e: operational(4, "prompt.replayed", "pending") });
eq(state.approval, undefined, "replay after terminal cannot resurrect prompt");

process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed) process.exit(1);
