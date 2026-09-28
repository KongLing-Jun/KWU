// Run: tsx src/__tests__/session-view-snapshot.test.ts

import {
  SESSION_VIEW_SNAPSHOT_VERSION,
  SessionViewSnapshotCache,
  normalizeSessionViewIdentity,
  sessionViewBasicMeta,
  sessionViewIdentityKey,
  type SessionViewIdentityInput,
  type SessionViewSnapshot,
} from "../features/session/sessionViewSnapshot";
import { initialState, type State } from "../features/session/sessionState";

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

function input(index: number, overrides: Partial<SessionViewIdentityInput> = {}): SessionViewIdentityInput {
  return {
    actorId: "actor-a",
    profileId: "profile-a",
    scope: "project",
    workspaceRoot: "D:\\Repo\\Demo",
    sessionPath: `D:\\Repo\\Demo\\session-${index}.jsonl`,
    topicId: `topic-${index}`,
    ...overrides,
  };
}

function snapshot(identityInput: SessionViewIdentityInput, title = ""): SessionViewSnapshot {
  const identity = normalizeSessionViewIdentity(identityInput);
  return {
    version: SESSION_VIEW_SNAPSHOT_VERSION,
    key: sessionViewIdentityKey(identity),
    identity,
    items: [{ kind: "user", id: "h0", text: title || identity.topicId }],
    seq: 1,
    composerSelection: { phase: "pending", seq: 0, modelRef: "" },
    title,
    scroll: { atBottom: true },
  };
}

console.log("\nsession view snapshot");

eq(
  sessionViewIdentityKey(input(1, { workspaceRoot: "D:\\Repo\\Demo\\" })),
  sessionViewIdentityKey(input(1, { workspaceRoot: "d:/repo/demo" })),
  "Windows workspace and session paths normalize separators, trailing slashes, and case",
);
ok(
  sessionViewIdentityKey(input(1, { workspaceRoot: "/Repo/Demo", sessionPath: "/Repo/Demo/S.jsonl" }))
    !== sessionViewIdentityKey(input(1, { workspaceRoot: "/repo/demo", sessionPath: "/repo/demo/S.jsonl" })),
  "Unix workspace paths retain case",
);
ok(
  sessionViewIdentityKey(input(1, { actorId: "actor-b" })) !== sessionViewIdentityKey(input(1)),
  "actor identity isolates snapshots",
);
ok(
  sessionViewIdentityKey(input(1, { profileId: "profile-b" })) !== sessionViewIdentityKey(input(1)),
  "profile identity isolates snapshots",
);
ok(
  sessionViewIdentityKey(input(1, { sessionPath: "" })) !== sessionViewIdentityKey(input(2, { sessionPath: "" })),
  "topic identity isolates sessions before a path is available",
);

const cachedMeta = sessionViewBasicMeta({
  label: "target session",
  modelRef: "kwu/gpt-5.6",
  ready: true,
  eventChannel: "runtime-only",
  cwd: "D:\\Repo\\Demo",
  collaborationMode: "goal",
  toolApprovalMode: "auto",
  tokenMode: "economy",
  goal: "finish target work",
  goalStatus: "running",
});
eq(cachedMeta?.modelRef, "kwu/gpt-5.6", "cached view keeps the target session model identity");
eq(cachedMeta?.collaborationMode, "goal", "cached view keeps the target Composer profile");
ok(!(cachedMeta && "eventChannel" in cachedMeta), "cached view excludes runtime-only event bindings");

const cache = new SessionViewSnapshotCache(8);
for (let index = 0; index < 8; index += 1) cache.set(snapshot(input(index)));
cache.get(input(0));
cache.set(snapshot(input(8)));
eq(cache.keys().length, 8, "idle snapshot cache is capped at eight entries");
ok(cache.get(input(0)) !== undefined, "cache hit refreshes LRU recency");
eq(cache.get(input(1)), undefined, "least recently used idle snapshot is evicted");

const bad = { ...snapshot(input(20)), version: 99 as 2 };
cache.set(bad);
eq(cache.get(input(20)), undefined, "incompatible snapshot versions fail closed");

cache.set(snapshot(input(30), "old title"));
cache.renameSession(input(30).sessionPath ?? "", "new title");
eq(cache.get(input(30))?.title, "new title", "rename updates cached title without dropping history");
cache.invalidateSessionPath(input(30).sessionPath ?? "");
eq(cache.get(input(30)), undefined, "delete and restore invalidation removes the matching session");

const versionedHistory = { ...snapshot(input(40)), sessionTurns: 7 };
cache.set(versionedHistory);
eq(cache.get(input(40))?.sessionTurns, 7, "cached view retains the backend history turn version");

const selectionSnapshot = {
  ...snapshot(input(41)),
  effort: { supported: true, current: "high", default: "auto", levels: ["auto", "high"] },
  composerSelection: { phase: "ready" as const, seq: 7, modelRef: "provider/model-b" },
};
cache.set(selectionSnapshot);
const restoredSelection = cache.get(input(41));
eq(restoredSelection?.composerSelection.modelRef, "provider/model-b", "cached view keeps the confirmed target model");
eq(restoredSelection?.effort?.current, "high", "cached view keeps the matching authoritative effort");
if (restoredSelection?.effort) restoredSelection.effort.levels.push("max");
eq(cache.get(input(41))?.effort?.levels.length, 2, "cached effort capability arrays are isolated from consumers");

const sources = new SessionViewSnapshotCache(2);
const view = input(50);
const oldWriter = sources.claimWriter(view, "old");
sources.acceptHistory(oldWriter);
const writer = sources.claimWriter(view, "complete");
sources.acceptHistory(writer);
const completeState = { ...initialState, items: snapshot(view).items, hydrateHistoryLoaded: true };
const staleState = { ...completeState, running: true, turnActive: true };
for (const owners of [["old", "complete"], ["complete", "old"]]) {
  const states = new Map<string, State>(owners.map((id) => [id, id === "old" ? staleState : completeState]));
  eq(sources.select(view, (id) => states.get(id)).state, completeState, "source selection is independent of retained Map order");
}
const accepted = sources.source(view)!;
sources.set({ ...snapshot(view), source: accepted });
ok(sources.canCarry({ ...snapshot(view), source: accepted }), "transition may carry the accepted source in its lifecycle");
sources.get(view);
eq(sources.source(view), accepted, "LRU reads and saves cannot create source acceptance");
const newWriter = sources.claimWriter(view, "new");
eq(sources.select(view, () => staleState).state, undefined, "writer transfer freezes old source instead of dereferencing it");
eq(sources.select(view, () => staleState).snapshot?.source, accepted, "writer transfer keeps the accepted display provenance");
sources.acceptHistory(oldWriter);
sources.acceptIntent(oldWriter);
eq(sources.source(view), accepted, "superseded history and live events cannot publish");
sources.releaseOwner("old");
eq(sources.writer(view, "new"), newWriter, "closing an unrelated retained owner preserves the writer");
sources.acceptIntent(newWriter, true);
const optimistic = sources.source(view)!;
eq(optimistic.basis, "intent", "optimistic user qualifies before a turn starts");
sources.acceptIntent(newWriter);
eq(sources.source(view), optimistic, "stream chunks keep one intent identity");
sources.acceptHistory(newWriter);
eq(sources.source(view), optimistic, "reconciliation preserves the current intent identity");
ok(optimistic.reconciled, "actual applied history qualifies future idle snapshots");
sources.set({ ...snapshot(view), source: accepted });
eq(sources.get(view), undefined, "borrowed snapshots cannot overwrite a newer source");
sources.set({ ...snapshot(view), source: optimistic });
sources.releaseOwner("new");
eq(sources.select(view, () => undefined).snapshot?.source, optimistic, "owner eviction retains a matching frozen display");
const reactivated = sources.claimWriter(view, "new");
ok(reactivated !== newWriter, "reusing an owner after revocation creates a fresh grant");
sources.acceptHistory(newWriter);
eq(sources.source(view), optimistic, "old tickets stay revoked even if owner ID is reused");
sources.claimWriter(input(51), "other");
sources.claimWriter(input(52), "third");
eq(sources.keys().length, 2, "source metadata and snapshots share the same bounded lifecycle");
sources.acceptHistory(reactivated);
eq(sources.source(view), undefined, "evicted callbacks cannot recreate source entries");
ok(!sources.canCarry({ ...snapshot(view), source: accepted }), "eviction cannot revive an old carried snapshot");
const rebound = sources.claimWriter(view, "new");
sources.invalidateSessionPath(view.sessionPath!);
sources.acceptHistory(rebound);
eq(sources.source(view), undefined, "path invalidation revokes in-flight acceptance");
const resetWriter = sources.claimWriter(view, "new");
sources.invalidate(view);
sources.claimWriter(view, "new");
sources.acceptHistory(resetWriter);
eq(sources.source(view), undefined, "reset separates same-key lifecycles");
const scopedWriter = sources.writer(view, "new");
sources.clear();
sources.acceptHistory(scopedWriter);
eq(sources.keys().length, 0, "scope changes prevent late acceptance from recreating entries");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
