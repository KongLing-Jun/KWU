// Run: tsx src/__tests__/stream-event-batch.test.ts

import { reducer } from "../features/session/useController";
import {
  coalesceRoutedStreamEvents,
  isStreamBatchingEnabled,
  STREAM_BATCH_DISABLE_STORAGE_KEY,
  type RoutedStreamEvent,
} from "../features/session/streamEventBatch";
import { initialState } from "../features/session/sessionState";
import { createRafBatch } from "../lib/rafBatch";
import type { WireEvent } from "../lib/types";

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

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}${actual === expected ? "" : `: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`}`);
}

const routed = (tabId: string, e: WireEvent) => ({ tabId, e });

console.log("\nstream event batch");

const coalesced = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "text", text: "hel" }),
  routed("tab-a", { kind: "reasoning", text: "think " }),
  routed("tab-a", { kind: "text", text: "lo" }),
  routed("tab-a", { kind: "reasoning", text: "again" }),
]);
eq(coalesced.length, 2, "one frame emits one event per tab, parent, and kind");
eq(coalesced[0]?.e.text, "hello", "text deltas concatenate in arrival order");
eq(coalesced[1]?.e.text, "think again", "production-wire reasoning text deltas concatenate in arrival order");

const legacyReasoning = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "reasoning", reasoning: "legacy " }),
  routed("tab-a", { kind: "reasoning", reasoning: "shape" }),
]);
eq(legacyReasoning[0]?.e.reasoning, "legacy shape", "legacy reasoning-field batches preserve their shape");

const productionFirstMixedReasoning = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "reasoning", text: "A" }),
  routed("tab-a", { kind: "reasoning", reasoning: "B" }),
  routed("tab-a", { kind: "reasoning", text: "C" }),
]);
eq(productionFirstMixedReasoning[0]?.e.text, "ABC", "production-first mixed reasoning stays in text");

const legacyFirstMixedReasoning = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "reasoning", reasoning: "A" }),
  routed("tab-a", { kind: "reasoning", text: "B" }),
  routed("tab-a", { kind: "reasoning", reasoning: "C" }),
]);
eq(legacyFirstMixedReasoning[0]?.e.reasoning, "ABC", "legacy-first mixed reasoning stays in reasoning");

const nullishPrecedence = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "reasoning", text: "", reasoning: "must-not-be-used" }),
  routed("tab-a", { kind: "reasoning", text: "kept" }),
]);
eq(nullishPrecedence[0]?.e.text, "kept", "empty text remains authoritative over the compatibility field");

const isolated = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "text", text: "root" }),
  routed("tab-a", { kind: "text", text: "child-a", parentId: "task-a" }),
  routed("tab-b", { kind: "text", text: "other-tab" }),
  routed("tab-a", { kind: "text", text: "+child-a", parentId: "task-a" }),
  routed("tab-a", { kind: "text", text: "child-b", parentId: "task-b" }),
]);
eq(isolated.length, 4, "tab and parent keys remain isolated");
eq(isolated.find(({ e }) => e.parentId === "task-a")?.e.text, "child-a+child-a", "child deltas merge only within their parent");

const candidateIsolated = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "text", turnId: "turn-1", candidateId: "candidate-1", text: "first" }),
  routed("tab-a", { kind: "text", turnId: "turn-1", candidateId: "candidate-2", text: "second" }),
]);
eq(candidateIsolated.length, 2, "stream batching never coalesces different root candidates");

const refIsolated = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "text", text: "one", parentId: "task-a", subagentRef: "child-1" }),
  routed("tab-a", { kind: "text", text: "two", parentId: "task-a", subagentRef: "child-2" }),
]);
eq(refIsolated.length, 2, "persisted child refs remain isolated even when a parent id is reused");

const interleaved = coalesceRoutedStreamEvents([
  routed("tab-a", { kind: "reasoning", text: "r" }),
  routed("tab-a", { kind: "text", text: "answer" }),
]);
const reduced = reducer(
  reducer(initialState, { type: "event", e: { kind: "turn_started" } }),
  { type: "stream_batch", events: interleaved.map(({ e }) => e) },
);
eq(reduced.live?.reasoning, "r", "batch reducer retains reasoning content");
eq(reduced.live?.text, "answer", "batch reducer retains text content");
eq(reduced.live?.reasoningComplete, true, "last text event preserves reasoning completion");

const globals = globalThis as Record<string, unknown>;
const previousRaf = globals.requestAnimationFrame;
const previousCancelRaf = globals.cancelAnimationFrame;
let pendingRaf: ((timestamp: number) => void) | undefined;
globals.requestAnimationFrame = (callback: (timestamp: number) => void) => {
  pendingRaf = callback;
  return 7;
};
globals.cancelAnimationFrame = () => {};
let liveState = reducer(initialState, { type: "event", e: { kind: "turn_started" } });
const liveBatch = createRafBatch<RoutedStreamEvent>((batch) => {
  liveState = reducer(liveState, {
    type: "stream_batch",
    events: coalesceRoutedStreamEvents(batch).map(({ e }) => e),
  });
});
liveBatch.push(routed("tab-a", { kind: "reasoning", text: "one" }));
liveBatch.push(routed("tab-a", { kind: "reasoning", text: " two" }));
liveBatch.push(routed("tab-a", { kind: "reasoning", text: " three" }));
eq(liveState.live?.reasoning ?? "", "", "reasoning waits for the scheduled frame");
pendingRaf?.(16);
eq(liveState.live?.reasoning, "one two three", "one RAF preserves every production-wire reasoning delta");
liveState = reducer(liveState, {
  type: "event",
  e: { kind: "message", text: "", reasoning: "one two three" },
});
const terminalAssistant = liveState.items.at(-1);
eq(terminalAssistant?.kind === "assistant" ? terminalAssistant.reasoning : "", "one two three", "terminal message reveals no previously lost reasoning");
if (previousRaf === undefined) delete globals.requestAnimationFrame;
else globals.requestAnimationFrame = previousRaf;
if (previousCancelRaf === undefined) delete globals.cancelAnimationFrame;
else globals.cancelAnimationFrame = previousCancelRaf;

eq(
  isStreamBatchingEnabled({ getItem: (key) => (key === STREAM_BATCH_DISABLE_STORAGE_KEY ? "1" : null) }),
  false,
  "runtime rollback switch restores direct event dispatch",
);

const causalOrder: string[] = [];
const causalBatch = createRafBatch<string>((items) => causalOrder.push(...items));
causalBatch.push("text-before-tool");
causalBatch.drain();
causalOrder.push("tool");
causalBatch.push("text-before-done");
causalBatch.drain();
causalOrder.push("turn_done");
eq(
  causalOrder.join(","),
  "text-before-tool,tool,text-before-done,turn_done",
  "non-stream events drain pending deltas before tool and terminal events",
);

const pagedTail = reducer(initialState, {
  type: "history",
  messages: [{ role: "assistant", content: "new" }],
  before: 200,
  hasMore: true,
  generation: "gen-1",
});
const pagedOlder = reducer(pagedTail, {
  type: "history_prepend",
  messages: [{ role: "user", content: "old" }],
  before: 100,
  hasMore: false,
  generation: "gen-1",
});
eq(pagedTail.historyGeneration, "gen-1", "tail page retains its generation guard");
eq(pagedOlder.historyBefore, 100, "older page advances the bounded cursor");
eq(pagedOlder.historyGeneration, "gen-1", "older page preserves the generation guard");
eq(pagedOlder.items.length, 2, "older history prepends without replacing the tail");

delete globals.requestAnimationFrame;
const fallbackFlushes: string[][] = [];
const fallbackBatch = createRafBatch<string>((items) => fallbackFlushes.push(items));
fallbackBatch.push("microtask");
await Promise.resolve();
await Promise.resolve();
eq(fallbackFlushes[0]?.[0], "microtask", "missing requestAnimationFrame falls back to a microtask flush");
if (previousRaf === undefined) delete globals.requestAnimationFrame;
else globals.requestAnimationFrame = previousRaf;

if (failed > 0) {
  console.error(`\n${failed} failed, ${passed} passed`);
  process.exit(1);
}
console.log(`\n${passed} passed`);
