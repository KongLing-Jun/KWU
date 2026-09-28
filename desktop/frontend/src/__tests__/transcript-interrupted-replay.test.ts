import assert from "node:assert/strict";
import { reducer } from "../features/session/useController";
import { initialState, type State } from "../features/session/sessionState";
import type { WireEvent } from "../lib/types";

const durable = (events: WireEvent[]) => events.map((event, index) => ({
  ...event, transcriptJournalId: "interrupted", transcriptSequence: index + 1,
}));
const user = (content: string): WireEvent => ({ kind: "notice", transcriptUser: { role: "user", content } });
const content = (state: State) => state.items.filter((item) => item.kind !== "assistant" || item.text || item.reasoning)
  .map((item) => item.kind === "assistant" ? [item.kind, item.text, item.reasoning, item.runtimeTurnId] : [item.kind, "text" in item ? item.text : item.id]);

for (const partial of [false, true]) {
  const events = durable([
    user("automation"),
    { kind: "turn_started", turnId: "old", candidateId: "old-answer" },
    ...(partial ? [
      { kind: "reasoning", turnId: "old", candidateId: "old-answer", text: "partial thought" },
      { kind: "text", turnId: "old", candidateId: "old-answer", text: "partial output" },
      { kind: "tool_dispatch", turnId: "old", tool: { id: "old-tool", name: "read", args: "{}" } },
    ] as WireEvent[] : []),
    user("continue"),
    { kind: "turn_started", turnId: "new", candidateId: "new-answer" },
    { kind: "text", turnId: "new", candidateId: "new-answer", text: "new output" },
    { kind: "message", turnId: "new", candidateId: "new-answer", text: "new output" },
    { kind: "turn_done", turnId: "new" },
  ]);
  const replay = { journalId: "interrupted", through: events.length, messages: [], events };
  const full = reducer(initialState, { type: "transcript_replay", replay, pending: [] });
  const userIndex = full.items.findIndex((item) => item.kind === "user" && item.text === "continue");
  const answerIndex = full.items.findIndex((item) => item.kind === "assistant" && item.text === "new output");
  assert.ok(answerIndex > userIndex, "new answer follows its accepted user after cold restart");
  const old = full.items.find((item) => item.kind === "assistant" && item.runtimeTurnId === "old");
  assert.ok(old?.kind === "assistant" && !old.streaming && !old.finalAnswer);
  if (partial && old?.kind === "assistant") {
    assert.equal(old.text, "partial output");
    assert.equal(old.reasoning, "partial thought");
  }
  for (let split = 1; split < events.length; split++) {
    const first = reducer(initialState, { type: "transcript_replay", replay: { ...replay, through: split, events: events.slice(0, split) }, pending: [] });
    const paged = reducer(first, { type: "transcript_replay", replay: { ...replay, after: split, events: events.slice(split) }, pending: [] });
    assert.deepEqual(content(paged), content(full), `page boundary ${split}`);
  }
  const live = events.reduce((state, e) => reducer(state, { type: "event", e }), initialState);
  assert.deepEqual(content(live), content(full));
  const duplicate = reducer(full, { type: "event", e: { kind: "turn_started", turnId: "old" } });
  assert.equal(duplicate.live, undefined);
  assert.equal(duplicate.runtimeTurnId, "new");
}

let active = durable([user("automation"), { kind: "turn_started", turnId: "old", candidateId: "old-answer" }, user("continue")])
  .reduce((state, e) => reducer(state, { type: "event", e }), initialState);
active = reducer(active, { type: "event", e: { kind: "turn_done", turnId: "old", interrupted: true } });
assert.equal(active.running, true, "late old terminal before new start must not stop the accepted user");
active = reducer(active, { type: "event", e: { kind: "turn_started", turnId: "new", candidateId: "new-answer" } });
active = reducer(active, { type: "event", e: { kind: "text", turnId: "new", candidateId: "new-answer", text: "new stream" } });
const newLive = active.live;
for (const e of [
  { kind: "turn_started", turnId: "new" },
  { kind: "turn_done", turnId: "old", interrupted: true },
  { kind: "message", turnId: "old", candidateId: "old-answer", text: "late old output" },
  { kind: "text", turnId: "old", candidateId: "old-answer", text: "late old fragment" },
] as WireEvent[]) {
  active = reducer(active, { type: "event", e });
  assert.equal(active.live, newLive);
  assert.equal(active.running, true);
  assert.equal(active.currentAssistant, newLive?.id);
}
console.log("interrupted transcript replay contracts passed");

active = reducer(active, { type: "event", e: { kind: "tool_dispatch", turnId: "new", tool: { id: "new-tool", name: "read", args: "{}" } } });
for (const e of [
  { kind: "tool_result", turnId: "old", tool: { id: "missing-old-tool", name: "read", output: "old result" } },
  { kind: "tool_dispatch", turnId: "old", tool: { id: "old-dispatch", name: "read", args: "{}" } },
  { kind: "final_candidate_rejected", turnId: "old", candidateId: "missing-old-candidate" },
] as WireEvent[]) {
  const prior = active;
  active = reducer(active, { type: "event", e });
  assert.equal(active.items, prior.items, "unattributable old event must not change the new turn");
  assert.equal(active.live, prior.live);
}
