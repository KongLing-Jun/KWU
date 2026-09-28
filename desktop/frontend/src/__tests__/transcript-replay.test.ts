import assert from "node:assert/strict";
import { reducer } from "../features/session/useController";
import { initialState } from "../features/session/sessionState";
import { coalesceRoutedStreamEvents } from "../features/session/streamEventBatch";
import { transcriptReplayNeedsFullRestore } from "../features/session/transcriptReplay";
import type { TranscriptReplay, WireEvent } from "../lib/types";

const durable = (sequence: number, event: WireEvent): WireEvent => ({
  ...event, tabId: "task-a", transcriptJournalId: "journal-a", transcriptSequence: sequence,
});
const started = durable(2, { kind: "turn_started", turnId: "turn-a" });
const text = durable(4, { kind: "text", turnId: "turn-a", text: "before switch" });
const late = durable(5, { kind: "text", turnId: "turn-a", text: " + background" });
const replay: TranscriptReplay = {
  journalId: "journal-a", through: 4,
  messages: [{ role: "user", content: "old request" }, { role: "assistant", content: "old output" }],
  events: [
    durable(1, { kind: "notice", transcriptUser: { role: "user", content: "new request" } }),
    started,
    durable(3, { kind: "reasoning", turnId: "turn-a", text: "saved reasoning" }),
    text,
  ],
};

// Cold recovery starts with no in-memory stream. A duplicate from the snapshot
// and an event committed after its watermark arrive while IPC is in progress.
const recovered = reducer(initialState, { type: "transcript_replay", replay, pending: [text, late] });
assert.equal(recovered.live?.text, "before switch + background");
assert.equal(recovered.live?.reasoning, "saved reasoning");
assert.equal(recovered.runtimeTurnId, "turn-a");
assert.equal(recovered.items.filter((item) => item.kind === "user").length, 2);
assert.ok(recovered.items.some((item) => item.kind === "assistant" && item.text === "old output"));
assert.equal(reducer(recovered, { type: "event", e: late }), recovered);
assert.equal(reducer(recovered, { type: "event", e: text }), recovered);

const nextReplay = { ...replay, through: 5, events: [...replay.events, late] };
const switchedAgain = reducer(initialState, { type: "transcript_replay", replay: nextReplay, pending: [] });
assert.equal(switchedAgain.live?.text, recovered.live?.text);
const otherTask = reducer(initialState, { type: "event", e: { kind: "turn_started", turnId: "turn-b" } });
assert.equal(otherTask.live?.text, "");
assert.equal(switchedAgain.runtimeTurnId, "turn-a");

const optimistic = reducer(initialState, { type: "user", seq: 0, text: "new request" });
const accepted = reducer(optimistic, { type: "event", e: replay.events[0] });
assert.equal(accepted.items.filter((item) => item.kind === "user").length, 1);
const batch = coalesceRoutedStreamEvents([text, late, late].map((e) => ({ tabId: "task-a", e })));
assert.equal(batch.length, 2);
assert.equal(batch[1].e.text, " + background");
assert.equal(batch[1].e.transcriptSequence, 5);

// Returning to a cached task applies only the missing suffix. A live event
// received before IPC resolves must not append its delta a second time.
const incremental = reducer(recovered, {
  type: "transcript_replay",
  replay: { journalId: "journal-a", after: 4, through: 5, messages: [], events: [late] },
  pending: [late],
});
assert.equal(incremental.items, recovered.items);
assert.equal(incremental.live, recovered.live);
assert.equal(incremental.live?.text, "before switch + background");
assert.equal(incremental.transcriptThrough, 5);
assert.deepEqual(incremental.transcriptSeen, {});
const reloading = reducer(recovered, { type: "hydrate_start", reason: "startup", surfaceIntent: "existing-session" });
const reloaded = reducer(reloading, {
  type: "transcript_replay",
  replay: { journalId: "journal-a", after: 4, through: 5, messages: [], events: [late] },
  pending: [],
});
assert.equal(reloaded.hydrateHistoryLoaded, true);
const replacedHistory = reducer(recovered, { type: "history", messages: [{ role: "user", content: "rewound" }] });
assert.equal(replacedHistory.transcriptJournalId, undefined);
assert.equal(replacedHistory.transcriptThrough, 0);

// Input formatting and an unrelated runtime event cannot create a second
// accepted user bubble or erase the identity of the optimistic one.
const padded = reducer(initialState, { type: "user", seq: 0, text: "  new request\n" });
const notified = reducer(padded, { type: "event", e: { kind: "notice", text: "connecting" } });
const paddedAccepted = reducer(notified, { type: "event", e: replay.events[0] });
assert.equal(paddedAccepted.items.filter((item) => item.kind === "user").length, 1);
assert.equal(paddedAccepted.items[0].id, padded.items[0].id);

// A snapshot taken before acceptance must retain the pending user. An older
// identical request in that journal must not consume the new submission.
const pendingSnapshot = reducer(padded, {
  type: "transcript_replay",
  replay: { journalId: "journal-a", through: 0, messages: [], events: [] },
  pending: [],
});
assert.equal(pendingSnapshot.items[0], padded.items[0]);
const repeated = reducer(padded, {
  type: "transcript_replay",
  replay: {
    journalId: "journal-a", through: 1, messages: [],
    events: [durable(1, { kind: "notice", transcriptUser: { role: "user", content: "new request", createdAt: 1 } })],
  },
  pending: [],
});
assert.equal(repeated.items.filter((item) => item.kind === "user").length, 2);

// Detachment can drop a message boundary while later reasoning is already
// delivered. Filling that gap on top of the later projection would close its
// stream and then dedup away the reasoning. Recover the journal in order.
const firstThinking = durable(3, { kind: "reasoning", turnId: "turn-a", candidateId: "first", text: "first thought" });
const thinkingReplay: TranscriptReplay = {
  journalId: "journal-a", through: 3, messages: [],
  events: [replay.events[0], durable(2, { kind: "turn_started", turnId: "turn-a", candidateId: "first" }), firstThinking],
};
const thinking = reducer(initialState, { type: "transcript_replay", replay: thinkingReplay, pending: [] });
const missedBoundary = durable(4, { kind: "message", turnId: "turn-a", candidateId: "first", text: "first step", reasoning: "first thought" });
const newerThinking = durable(7, { kind: "reasoning", turnId: "turn-a", candidateId: "second", text: "second thought" });
const gapped = reducer(thinking, { type: "event", e: newerThinking });
const gapReplay: TranscriptReplay = {
  journalId: "journal-a", after: 3, through: 7, messages: [], events: [missedBoundary, newerThinking],
};
assert.equal(transcriptReplayNeedsFullRestore(gapReplay, gapped), true);
assert.equal(transcriptReplayNeedsFullRestore(gapReplay, thinking), false, "an unseen ordered suffix can still use incremental replay");
assert.equal(transcriptReplayNeedsFullRestore({ ...gapReplay, events: [newerThinking] }, gapped), false, "database-global sequence gaps alone do not trigger reconstruction");
const duringRecovery = durable(9, { kind: "reasoning", turnId: "turn-a", candidateId: "second", text: " + during recovery" });
const ordered = reducer(gapped, {
  type: "transcript_replay",
  replay: { ...thinkingReplay, through: 7, events: [...thinkingReplay.events, ...gapReplay.events] },
  pending: [newerThinking, duringRecovery],
});
assert.ok(ordered.items.some((item) => item.kind === "assistant" && item.text === "first step" && item.reasoning === "first thought"));
assert.equal(ordered.live?.reasoning, "second thought + during recovery");
assert.equal(reducer(ordered, { type: "event", e: duringRecovery }).live, ordered.live);
console.log("durable transcript recovery contracts passed");
