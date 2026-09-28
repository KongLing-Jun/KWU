// Run: tsx src/__tests__/operational-prompt-projection.test.ts

import {
  applyPromptReplyReceipt,
  beginPromptReply,
  initialPromptProjection,
  PROMPT_SEEN_EVENT_LIMIT,
  projectLegacyPrompt,
  projectOperationalPrompt,
  selectVisiblePrompt,
} from "../features/session/promptProjection";
import type { WireEvent, WireOperationalEnvelope } from "../lib/types";

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

function event(sequence: number, kind: WireOperationalEnvelope["kind"], overrides: Partial<WireOperationalEnvelope> = {}): WireOperationalEnvelope {
  return {
    schemaVersion: 1,
    eventId: `event-${sequence}`,
    operationId: `operation-${sequence}`,
    aggregate: { kind: "prompt", id: "prompt-1", sequence },
    scope: { tabId: "tab-a", sessionId: "session-a", promptId: "prompt-1" },
    kind,
    retryable: true,
    prompt: {
      kind: "ask",
      lifecycle: kind === "prompt.resolved" ? "resolved" : "pending",
      ask: { id: "prompt-1", questions: [{ id: "q1", prompt: "Pick", options: [{ label: "A" }] }] },
    },
    ...overrides,
  };
}

let state = projectOperationalPrompt(initialPromptProjection("tab-a"), event(1, "prompt.registered"));
eq(selectVisiblePrompt(state)?.promptId, "prompt-1", "registered prompt becomes the canonical visible prompt");

const duplicate = projectOperationalPrompt(state, event(1, "prompt.registered"));
eq(duplicate, state, "duplicate eventId is idempotent");

const outOfOrder = projectOperationalPrompt(state, { ...event(0, "prompt.emitted"), eventId: "late-event" });
eq(selectVisiblePrompt(outOfOrder)?.sequence, 1, "out-of-order aggregate sequence cannot replace current state");
eq(outOfOrder.seenEventIds["late-event"], true, "ignored late delivery is remembered and bounded");

const wrongTab = projectOperationalPrompt(state, { ...event(2, "prompt.emitted"), eventId: "wrong-tab", scope: { tabId: "tab-b", promptId: "prompt-1" } });
eq(wrongTab, state, "wrong-tab prompt fact cannot enter another tab projection");

const answers = [{ questionId: "q1", selected: ["A"] }];
state = beginPromptReply(state, "prompt-1", { kind: "ask", answers });
eq(selectVisiblePrompt(state)?.lifecycle, "replying", "reply command moves the prompt to replying without clearing it");
state = applyPromptReplyReceipt(state, { promptId: "prompt-1", status: "failed", occurrenceId: "occ-1", retryable: true });
eq(selectVisiblePrompt(state)?.lastReply, { kind: "ask", answers }, "reply failure retains the exact Ask input for retry");
eq(selectVisiblePrompt(state)?.occurrenceId, "occ-1", "reply failure exposes the metadata-only occurrence ID");

state = beginPromptReply(state, "prompt-1", { kind: "ask", answers });
const accepted = applyPromptReplyReceipt(state, { promptId: "prompt-1", status: "accepted" });
eq(selectVisiblePrompt(accepted)?.lifecycle, "replying", "accepted receipt does not clear before prompt.resolved fact");

state = projectOperationalPrompt(accepted, event(2, "prompt.resolved"));
eq(selectVisiblePrompt(state), undefined, "resolved fact creates a non-visible tombstone");
state = projectOperationalPrompt(state, { ...event(3, "prompt.replayed"), eventId: "replay-after-terminal", causationId: "event-2" });
eq(selectVisiblePrompt(state), undefined, "later non-terminal delivery cannot resurrect a terminal prompt tombstone");

const legacy: WireEvent = {
  kind: "ask_request",
  eventId: "shared-delivery-id",
  eventSequence: 9000,
  tabId: "tab-a",
  ask: { id: "prompt-legacy", questions: [{ id: "q1", prompt: "Pick", options: [{ label: "A" }] }] },
};
let compatibility = projectLegacyPrompt(initialPromptProjection("tab-a"), legacy, "tab-a");
eq(selectVisiblePrompt(compatibility)?.sequence, 0, "legacy transport sequence never enters the aggregate-local sequence domain");
compatibility = projectOperationalPrompt(compatibility, {
  ...event(1, "prompt.emitted"),
  eventId: "shared-delivery-id",
  aggregate: { kind: "prompt", id: "prompt-legacy", sequence: 1 },
  scope: { tabId: "tab-a", promptId: "prompt-legacy" },
  prompt: { kind: "ask", lifecycle: "pending", ask: legacy.ask },
});
eq(compatibility.prompts["prompt-legacy"]?.authority, "operational", "operational fact upgrades a legacy-first projection even when wire event IDs match");
compatibility = projectOperationalPrompt(compatibility, {
  ...event(2, "prompt.resolved"),
  aggregate: { kind: "prompt", id: "prompt-legacy", sequence: 2 },
  scope: { tabId: "tab-a", promptId: "prompt-legacy" },
});
eq(selectVisiblePrompt(compatibility), undefined, "operational terminal fact resolves a legacy-only fallback projection");

let bounded = initialPromptProjection("tab-a");
for (let index = 1; index <= PROMPT_SEEN_EVENT_LIMIT + 1; index += 1) {
  bounded = projectOperationalPrompt(bounded, { ...event(index, "prompt.emitted"), eventId: `bounded-${index}` });
}
eq(bounded.seenEventOrder.length, PROMPT_SEEN_EVENT_LIMIT, "seen event retention is bounded");
eq(bounded.seenEventIds["bounded-1"], undefined, "oldest seen event identity is evicted");
eq(bounded.seenEventIds[`bounded-${PROMPT_SEEN_EVENT_LIMIT + 1}`], true, "newest seen event identity is retained");

let stale = beginPromptReply(projectOperationalPrompt(initialPromptProjection("tab-a"), event(1, "prompt.emitted")), "prompt-1", { kind: "ask", answers });
stale = applyPromptReplyReceipt(stale, { promptId: "prompt-1", status: "already_resolved" });
eq(selectVisiblePrompt(stale)?.lifecycle, "pending", "already-resolved receipt leaves clearing to a fact but exits permanent replying state");
eq(selectVisiblePrompt(stale)?.lastReply, { kind: "ask", answers }, "already-resolved resync retains the submitted input");

process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed) process.exit(1);
