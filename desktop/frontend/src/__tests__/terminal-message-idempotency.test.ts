// Run: tsx src/__tests__/terminal-message-idempotency.test.ts

import { reducer, reducerEntryTraceMessage } from "../features/session/useController";
import type { Item } from "../features/session/sessionItems";
import { initialState, type State } from "../features/session/sessionState";
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

function rootAssistants(state: State): Array<Extract<Item, { kind: "assistant" }>> {
  return state.items.filter((item): item is Extract<Item, { kind: "assistant" }> => item.kind === "assistant" && !item.parentId);
}

function apply(state: State, event: WireEvent): State {
  return reducer(state, { type: "event", e: event });
}

function startTurn(state: State, turnId: string, startedAt: number): State {
  return apply(state, { kind: "turn_started", turnId, eventId: `${turnId}:start`, eventSequence: 1, activeTurnStartedAt: startedAt });
}

function terminal(turnId: string, candidateId: string, overrides: Partial<WireEvent> = {}): WireEvent {
  return {
    kind: "message",
    eventId: `${turnId}:${candidateId}:message`,
    turnId,
    candidateId,
    eventSequence: 4,
    text: "same final",
    reasoning: "same reasoning",
    ...overrides,
  };
}

console.log("\nterminal message idempotency");

let legacyDuplicate = apply(initialState, { kind: "turn_started", activeTurnStartedAt: 500 });
legacyDuplicate = apply(legacyDuplicate, { kind: "message", text: "legacy final", reasoning: "legacy reasoning" });
legacyDuplicate = apply(legacyDuplicate, { kind: "message", text: "legacy final", reasoning: "legacy reasoning" });
eq(rootAssistants(legacyDuplicate).length, 1, "same live turn legacy terminal delivery is idempotent without wire identity");

let duplicate = startTurn(initialState, "turn-1", 1_000);
duplicate = apply(duplicate, { kind: "reasoning", turnId: "turn-1", candidateId: "candidate-1", reasoning: "same reasoning" });
duplicate = apply(duplicate, { kind: "text", turnId: "turn-1", candidateId: "candidate-1", text: "same final" });
const beforeRepeatedStart = duplicate;
duplicate = startTurn(duplicate, "turn-1", 1_000);
eq(duplicate.live, beforeRepeatedStart.live, "repeated identified turn start preserves live reasoning and text");
eq(duplicate.items, beforeRepeatedStart.items, "repeated identified turn start preserves assistant identity");
duplicate = apply(duplicate, terminal("turn-1", "candidate-1"));
duplicate = apply(duplicate, terminal("turn-1", "candidate-1"));
eq(rootAssistants(duplicate).length, 1, "same turn and candidate terminal delivery is idempotent");
duplicate = apply(duplicate, { kind: "turn_done", eventId: "turn-1:done", turnId: "turn-1", candidateId: "candidate-1", eventSequence: 5 });
duplicate = apply(duplicate, terminal("turn-1", "candidate-1", { eventSequence: 6 }));
eq(rootAssistants(duplicate).length, 1, "late terminal replay after TurnDone reconciles the accepted candidate");
duplicate = startTurn(duplicate, "turn-1", 1_000);
eq(rootAssistants(duplicate).length, 1, "late identified start does not recreate a completed assistant");
eq(duplicate.running, false, "late identified start does not restart a completed turn");

let delayedStart = apply(initialState, { kind: "reasoning", turnId: "delayed-turn", candidateId: "delayed-candidate", text: "thinking before start delivery" });
delayedStart = apply(delayedStart, { kind: "text", turnId: "delayed-turn", candidateId: "delayed-candidate", text: "answer before start delivery" });
const beforeDelayedStart = delayedStart;
delayedStart = startTurn(delayedStart, "delayed-turn", 1_500);
eq(delayedStart.live, beforeDelayedStart.live, "delayed start preserves deltas already attributed to its turn");
eq(delayedStart.runtimeTurnId, "delayed-turn", "delayed start still installs the runtime turn identity");
eq(delayedStart.turnStartAt, 1_500, "delayed start still installs the authoritative start time");

let enriched = startTurn(initialState, "turn-2", 2_000);
enriched = apply(enriched, terminal("turn-2", "candidate-1", { memoryCitations: [] }));
enriched = apply(enriched, terminal("turn-2", "candidate-1", {
  memoryCitations: [{ id: "memory-1", source: "memory.md", lineStart: 3 }],
}));
eq(rootAssistants(enriched).length, 1, "authoritative terminal enrichment does not append a candidate");
eq(rootAssistants(enriched)[0]?.memoryCitations?.[0]?.id, "memory-1", "later terminal delivery enriches the existing candidate");

let rejected = startTurn(initialState, "turn-3", 3_000);
rejected = apply(rejected, terminal("turn-3", "candidate-1"));
rejected = apply(rejected, {
  kind: "final_candidate_rejected",
  eventId: "turn-3:candidate-1:rejected",
  turnId: "turn-3",
  candidateId: "candidate-1",
  eventSequence: 5,
});
rejected = apply(rejected, terminal("turn-3", "candidate-1", { eventSequence: 6 }));
eq(rootAssistants(rejected).length, 0, "late replay of a rejected candidate stays retracted");
rejected = apply(rejected, terminal("turn-3", "candidate-2", { eventSequence: 8, text: "replacement final" }));
eq(rootAssistants(rejected).length, 1, "rejection permits one replacement candidate");
eq(rootAssistants(rejected)[0]?.text, "replacement final", "replacement candidate remains visible after rejection");

let separateTurns = startTurn(initialState, "turn-4", 4_000);
separateTurns = apply(separateTurns, terminal("turn-4", "candidate-1"));
separateTurns = apply(separateTurns, { kind: "turn_done", eventId: "turn-4:done", turnId: "turn-4", eventSequence: 5 });
separateTurns = startTurn(separateTurns, "turn-5", 5_000);
separateTurns = apply(separateTurns, terminal("turn-5", "candidate-1"));
eq(rootAssistants(separateTurns).length, 2, "different turns preserve identical assistant text");

let batched = startTurn(initialState, "turn-6", 6_000);
batched = reducer(batched, {
  type: "stream_batch",
  events: [
    { kind: "reasoning", turnId: "turn-6", candidateId: "candidate-1", reasoning: "same reasoning" },
    { kind: "text", turnId: "turn-6", candidateId: "candidate-1", text: "same final" },
  ],
});
batched = apply(batched, terminal("turn-6", "candidate-1"));
batched = apply(batched, terminal("turn-6", "candidate-1"));
eq(rootAssistants(batched).length, 1, "stream batch projection keeps terminal idempotency");

const trace = reducerEntryTraceMessage("tab-fixture", terminal("turn-7", "candidate-1", { text: "SECRET_BODY" }), 2) ?? "";
ok(trace.includes("tab=tab-fixture") && trace.includes("turn=turn-7") && trace.includes("candidate=candidate-1") && trace.includes("arrival=2"), "reducer entry trace records only delivery identity and order");
ok(!trace.includes("SECRET_BODY") && !trace.includes("same reasoning"), "reducer entry trace excludes message and reasoning content");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
