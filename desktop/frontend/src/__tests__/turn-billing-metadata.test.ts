// Run: tsx src/__tests__/turn-billing-metadata.test.ts

import { historyMessagesToItems, initialState, reducer } from "../lib/useController";
import type { HistoryMessage, TurnMetadata } from "../lib/types";
import { applyTurnBilling } from "../features/session/turnBilling";

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

function assistantTurn(items: typeof initialState.items, id: string): TurnMetadata | undefined {
  const item = items.find((candidate) => candidate.kind === "assistant" && candidate.id === id);
  return item?.kind === "assistant" ? item.turn : undefined;
}

console.log("\nturn billing metadata");

const metadata: TurnMetadata = {
  taskId: "task-current",
  modelRef: "provider/glm-5.2",
  modelId: "glm-5.2",
  billingSource: "subscription_pool",
};
const state = {
  ...initialState,
  running: true,
  turnActive: true,
  items: [
    { kind: "user", id: "u-old", text: "old prompt" } as const,
    { kind: "assistant", id: "a-old", text: "old answer", reasoning: "", streaming: false } as const,
    { kind: "user", id: "u-current", text: "new prompt" } as const,
    { kind: "assistant", id: "a-tool-call", text: "working", reasoning: "", streaming: false } as const,
    { kind: "tool", id: "tool-1", name: "read_file", args: "{}", readOnly: true, status: "done" } as const,
    { kind: "assistant", id: "a-final", text: "final answer", reasoning: "", streaming: true } as const,
  ],
};
const done = reducer(state, { type: "event", e: { kind: "turn_done", turn: metadata } });
eq(assistantTurn(done.items, "a-old"), undefined, "earlier turn remains untouched");
eq(assistantTurn(done.items, "a-tool-call"), undefined, "intermediate assistant remains untouched");
eq(assistantTurn(done.items, "a-final")?.taskId, "task-current", "only final assistant owns turn metadata");
eq(done.items.find((item) => item.id === "a-final")?.kind === "assistant" && done.items.find((item) => item.id === "a-final")?.streaming, false, "existing turn finalization is preserved");

const settled = reducer(done, { type: "turn_billing_settled", taskId: "task-current", chargedCredits: "7.9000" });
eq(assistantTurn(settled.items, "a-final")?.chargedCredits, "7.9000", "settlement keeps the authoritative decimal string");
const zeroSettled = reducer(done, { type: "turn_billing_settled", taskId: "task-current", chargedCredits: "0" });
eq(assistantTurn(zeroSettled.items, "a-final")?.chargedCredits, "0", "legal zero credit settlement is retained");

const restored = historyMessagesToItems([
  { role: "user", content: "prompt" },
  { role: "assistant", content: "answer", turn: { ...metadata, chargedCredits: "3.2500" } },
] as HistoryMessage[], "h").items;
eq(assistantTurn(restored, "h1")?.modelId, "glm-5.2", "history restores the model identity");
eq(assistantTurn(restored, "h1")?.chargedCredits, "3.2500", "history restores exact charged credits");

const localFinal = applyTurnBilling(done.items, [{ taskId: metadata.taskId, status: "settled", chargedCredits: "0.0000" }], "scope-a");
eq(assistantTurn(localFinal, "a-final")?.chargedCredits, "0.0000", "local batch keeps final zero");
eq(applyTurnBilling(localFinal, [{ taskId: metadata.taskId, status: "pending" }], "scope-a"), localFinal, "late pending cannot replace a known final");
eq(applyTurnBilling(localFinal, [{ taskId: metadata.taskId, status: "error" }], "scope-a"), localFinal, "read failure cannot replace a known final");
eq(applyTurnBilling(localFinal, [{ taskId: metadata.taskId, status: "settled", chargedCredits: "0.0000" }], "scope-a"), localFinal, "duplicate result is idempotent");
const otherScope = applyTurnBilling(localFinal, [{ taskId: metadata.taskId, status: "unavailable" }], "scope-b");
eq(assistantTurn(otherScope, "a-final")?.chargedCredits, undefined, "a different binding does not inherit final credits");

const replayedFinal = reducer({ ...done, items: localFinal, transcriptJournalId: "journal-a" }, {
  type: "transcript_replay", pending: [],
  replay: { journalId: "journal-a", through: 10, events: [], messages: [{ role: "assistant", content: "final answer", turn: metadata }] },
});
eq(assistantTurn(replayedFinal.items, "h0")?.chargedCredits, "0.0000", "full replay without credits preserves the newer final");
eq(replayedFinal.transcriptThrough, 10, "billing does not change replay watermark");
const staleBatch = reducer(replayedFinal, {
  type: "turn_billing_results", scopeKey: "scope-b", sessionPath: "other-session", sessionGen: replayedFinal.sessionGen,
  results: [{ taskId: metadata.taskId, status: "settled", chargedCredits: "999" }],
});
eq(staleBatch, replayedFinal, "response for a previous session is discarded");

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}
process.stdout.write(`\n${passed} tests passed.\n`);
