// Run: tsx src/__tests__/history-revalidation.test.ts

import { reducer } from "../features/session/useController";
import {
  channelSessionRefreshTarget,
  historyRevalidationRequestIsCurrent,
  mergeRuntimeTailItems,
  prepareHistoryRevalidation,
} from "../features/session/historyRevalidation";
import type { Item } from "../features/session/sessionItems";
import { initialState } from "../features/session/sessionState";
import type { HistoryMessage } from "../lib/types";

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

const baseMessages: HistoryMessage[] = [
  { role: "user", content: "one" },
  { role: "assistant", content: "answer one" },
];

console.log("\nhistory revalidation");

const base = reducer(initialState, { type: "history", messages: baseMessages });
const unchanged = prepareHistoryRevalidation(base.historyFingerprint, baseMessages.map((message) => ({ ...message })));
eq(unchanged.changed, false, "equivalent history skips publishing a reducer action");

let latestRefreshTab = { id: "channel-a", readOnly: true, sessionPath: " D:\\repo\\a.jsonl " };
eq(channelSessionRefreshTarget(latestRefreshTab)?.tabId, "channel-a", "a refresh iteration selects the current channel tab");
latestRefreshTab = { id: "channel-b", readOnly: true, sessionPath: "D:\\repo\\b.jsonl" };
eq(channelSessionRefreshTarget(latestRefreshTab)?.tabId, "channel-b", "a queued iteration selects the latest channel tab");
eq(channelSessionRefreshTarget(latestRefreshTab)?.sessionPath, "D:\\repo\\b.jsonl", "queued refresh uses the latest normalized path");
eq(channelSessionRefreshTarget({ ...latestRefreshTab, readOnly: false }), undefined, "editable tabs are excluded from channel refresh");
eq(channelSessionRefreshTarget({ ...latestRefreshTab, sessionPath: " " }), undefined, "tabs without a session path are excluded");

const expectedItems = base.items;
const currentRequest = {
  tabId: "channel-tab",
  activationSeq: 4,
  identityKey: "profile|channel-session",
  loadSeq: 9,
  items: expectedItems,
};
const currentGuard = {
  activeTabId: "channel-tab",
  activationSeq: 4,
  identityKey: "profile|channel-session",
  loadSeq: 9,
  items: expectedItems,
  hasActiveRuntimeTail: false,
};
ok(historyRevalidationRequestIsCurrent(currentRequest, currentGuard), "current channel history request may publish");
ok(
  !historyRevalidationRequestIsCurrent(currentRequest, { ...currentGuard, activeTabId: "other-tab" }),
  "tab switch rejects a deferred channel history response",
);
ok(
  !historyRevalidationRequestIsCurrent(currentRequest, { ...currentGuard, activationSeq: 5 }),
  "surface activation change rejects a deferred channel history response",
);
ok(
  !historyRevalidationRequestIsCurrent(currentRequest, { ...currentGuard, identityKey: "profile|other-session" }),
  "session path identity change rejects a deferred channel history response",
);
ok(
  !historyRevalidationRequestIsCurrent(currentRequest, { ...currentGuard, loadSeq: 10 }),
  "newer history load rejects an older channel history response",
);
ok(
  !historyRevalidationRequestIsCurrent(currentRequest, { ...currentGuard, items: [...expectedItems] }),
  "concurrent transcript change rejects a deferred channel history response",
);
ok(
  !historyRevalidationRequestIsCurrent(currentRequest, { ...currentGuard, hasActiveRuntimeTail: true }),
  "active runtime tail rejects a disk snapshot replacement",
);

const appendedMessages = [...baseMessages, { role: "user", content: "two" }];
const appended = prepareHistoryRevalidation(base.historyFingerprint, appendedMessages);
eq(appended.changed, true, "appended history requests a revalidation publish");
const appendedState = reducer(base, {
  type: "history_revalidated",
  messages: appendedMessages,
  fingerprint: appended.fingerprint,
});
eq(appendedState.items[0], base.items[0], "prefix append preserves the first projected item identity");
eq(appendedState.items[1], base.items[1], "prefix append preserves the second projected item identity");
eq(appendedState.items.length, base.items.length + 1, "prefix append adds only the new projected item");

const modifiedMessages = [
  { role: "user", content: "changed" },
  { role: "assistant", content: "answer one" },
];
const modified = prepareHistoryRevalidation(base.historyFingerprint, modifiedMessages);
const modifiedState = reducer(base, {
  type: "history_revalidated",
  messages: modifiedMessages,
  fingerprint: modified.fingerprint,
});
ok(modifiedState.items !== base.items, "structural history change atomically replaces the projection");
eq(modifiedState.items[0]?.id, "h0", "structural replacement keeps stable projection ids");

const removedMessages = [baseMessages[0]];
const removed = prepareHistoryRevalidation(base.historyFingerprint, removedMessages);
const removedState = reducer(base, {
  type: "history_revalidated",
  messages: removedMessages,
  fingerprint: removed.fingerprint,
});
eq(removedState.items.length, 1, "history deletion replaces the projection");
eq(removedState.items[0]?.id, "h0", "history deletion retains stable ids from the fixed projection origin");

const repeatedEarlyHistory: Item[] = [
  { kind: "user", id: "h0", text: "continue" },
  { kind: "assistant", id: "h1", text: "earlier answer", reasoning: "", streaming: false },
];
const repeatedTail: Item[] = [
  { kind: "user", id: "u20", text: "continue" },
  { kind: "assistant", id: "a21", text: "latest answer", reasoning: "", streaming: false },
];
const repeatedMerged = mergeRuntimeTailItems(repeatedEarlyHistory, repeatedTail);
eq(repeatedMerged.length, 4, "an early duplicate does not discard an unpersisted ordered tail");
eq(repeatedMerged[2], repeatedTail[0], "the repeated user message remains at the runtime boundary");
eq(repeatedMerged[3], repeatedTail[1], "the repeated user message keeps its following assistant response");

const overlappingHistory: Item[] = [
  { kind: "user", id: "h0", text: "one" },
  { kind: "assistant", id: "h1", text: "persisted", reasoning: "", streaming: false },
];
const overlappingTail: Item[] = [
  { kind: "assistant", id: "a9", text: "persisted", reasoning: "", streaming: false },
  { kind: "user", id: "u10", text: "new tail" },
];
const overlappingMerged = mergeRuntimeTailItems(overlappingHistory, overlappingTail);
eq(overlappingMerged.length, 3, "a true history-suffix and tail-prefix overlap is emitted only once");
eq(overlappingMerged[1], overlappingHistory[1], "the persisted side owns the overlapped item");
eq(overlappingMerged[2], overlappingTail[1], "only the unpersisted remainder is appended");

const identifiedTail: Item[] = [
  { kind: "assistant", id: "a-runtime", text: "persisted", reasoning: "", streaming: false, runtimeTurnId: "turn-runtime", runtimeCandidateId: "candidate-runtime" },
];
const identifiedMerged = mergeRuntimeTailItems(overlappingHistory, identifiedTail);
const identifiedOverlap = identifiedMerged.find((item) => item.kind === "assistant" && item.text === "persisted");
eq(identifiedOverlap?.id, "a-runtime", "runtime-tail overlap keeps the live item identity for active lifecycle references");
eq(identifiedOverlap?.kind === "assistant" ? identifiedOverlap.runtimeTurnId : undefined, "turn-runtime", "runtime-tail overlap carries turn identity onto the history-owned item");
eq(identifiedOverlap?.kind === "assistant" ? identifiedOverlap.runtimeCandidateId : undefined, "candidate-runtime", "runtime-tail overlap carries candidate identity onto the history-owned item");

let completedRuntime = reducer(initialState, { type: "event", e: { kind: "turn_started", turnId: "turn-revalidated" } });
completedRuntime = reducer(completedRuntime, {
  type: "event",
  e: { kind: "message", turnId: "turn-revalidated", candidateId: "candidate-revalidated", text: "persisted after completion", reasoning: "" },
});
const revalidatedRuntime = reducer(completedRuntime, {
  type: "history_revalidated",
  messages: [{ role: "assistant", content: "persisted after completion" }],
  fingerprint: "fixture-revalidated",
});
const afterLateReplay = reducer(revalidatedRuntime, {
  type: "event",
  e: { kind: "message", turnId: "turn-revalidated", candidateId: "candidate-revalidated", text: "persisted after completion", reasoning: "" },
});
eq(afterLateReplay.items.filter((item) => item.kind === "assistant").length, 1, "history revalidation keeps enough runtime identity to absorb a late terminal replay");

const liveBase = reducer(base, { type: "event", e: { kind: "turn_started" } });
const liveWithMessage = reducer(liveBase, {
  type: "event",
  e: { kind: "message", text: "arrived while history was pending", reasoning: "" },
});
const runtimeTailHydrated = reducer(liveWithMessage, {
  type: "history_with_runtime_tail",
  messages: baseMessages,
  fingerprint: prepareHistoryRevalidation(undefined, baseMessages).fingerprint,
});
ok(
  runtimeTailHydrated.items.some((item) => item.kind === "assistant" && item.text === "arrived while history was pending"),
  "runtime-tail hydration merges against reducer-time items instead of a captured tail",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
