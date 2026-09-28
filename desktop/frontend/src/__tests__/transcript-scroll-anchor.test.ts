// Run: tsx src/__tests__/transcript-scroll-anchor.test.ts

import {
  captureTranscriptAnchor,
  captureVirtualTranscriptAnchor,
  consumePresentationRestoreSnapshot,
  remapTranscriptMiddleAnchor,
  restoreTranscriptAnchor,
  stagePresentationRestoreSnapshot,
  shouldAutoScrollTranscriptQuestions,
  syncPresentationRestoreSnapshot,
} from "../lib/transcriptScrollAnchor";

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

console.log("\ntranscript semantic scroll anchors");

let queryCount = 0;
let contentTop = 160;
const container = {
  scrollTop: 200,
  offsetTop: 9_000,
  getBoundingClientRect: () => ({ top: 100 }),
  querySelectorAll: () => {
    queryCount += 1;
    return [
      {
        id: "question-anchor-u1",
        offsetTop: -4_000,
        getBoundingClientRect: () => ({ top: 100 + contentTop - container.scrollTop }),
      },
      {
        id: "unrelated",
        offsetTop: 0,
        getBoundingClientRect: () => ({ top: 0 }),
      },
    ];
  },
} as unknown as HTMLElement;

const questions = Array.from({ length: 100 }, (_, index) => ({ id: index === 0 ? "u1" : `missing-${index}` }));
const captured = captureTranscriptAnchor(container, questions);
eq(queryCount, 1, "capture scans the transcript DOM once regardless of question count");
eq(captured?.anchorMessageId, "u1", "capture selects the scoped visible question");
eq(captured?.offset, 40, "capture stores the anchor's viewport offset relative to the container");

container.scrollTop = 50;
const restored = restoreTranscriptAnchor(container, "u1", captured?.offset ?? 0);
ok(restored, "restore finds the anchor only within its transcript container");
eq(container.scrollTop, 200, "nested anchor restoration preserves the captured visual offset");

contentTop = 20;
container.scrollTop = 0;
const topCaptured = captureTranscriptAnchor(container, questions);
eq(topCaptured?.anchorMessageId, "u1", "capture falls back to the first question below the top threshold");
eq(topCaptured?.offset, -20, "top fallback records the first question's negative visual offset");
container.scrollTop = 70;
const topRestored = restoreTranscriptAnchor(container, "u1", topCaptured?.offset ?? 0);
ok(topRestored, "restore resolves the top fallback anchor");
eq(container.scrollTop, 0, "top fallback round trip restores scrollTop zero");

const virtualCaptured = captureVirtualTranscriptAnchor(
  [{ id: "old" }, { id: "nearest" }, { id: "below" }],
  new Map([
    ["question-anchor-old", 1],
    ["question-anchor-nearest", 5],
    ["question-anchor-below", 9],
  ]),
  (row) => new Map([[1, 900], [5, 120], [9, -300]]).get(row),
);
eq(virtualCaptured?.anchorMessageId, "nearest", "virtual capture selects the nearest preceding question when no question DOM is mounted");
eq(virtualCaptured?.offset, 120, "virtual capture preserves the measured visual offset for the selected question");

ok(
  !shouldAutoScrollTranscriptQuestions("session-a", "session-b", 0, 0),
  "switching from fewer-question A to cached scroll-up B never schedules forced-bottom",
);
ok(
  !shouldAutoScrollTranscriptQuestions(undefined, "session-b", 0, 0),
  "the first cached view render does not race semantic anchor restoration",
);
ok(
  !shouldAutoScrollTranscriptQuestions("session-b", "session-b", 0, 1),
  "background history revalidation does not force a scrolled-up view to the bottom",
);
ok(
  shouldAutoScrollTranscriptQuestions("session-b", "session-b", 1, 1),
  "a real question count change within the same stable view may follow the bottom",
);

const revalidatedAnchor = remapTranscriptMiddleAnchor(
  [{ id: "u1", text: "same" }, { id: "u2", text: "same" }, { id: "u3", text: "latest" }],
  [{ id: "h1", text: "same" }, { id: "h2", text: "same" }, { id: "h3", text: "latest" }],
  { atBottom: false, anchorMessageId: "u2", offset: 37 },
);
eq(revalidatedAnchor.anchorMessageId, "h2", "history revalidation remaps the same duplicate-question occurrence");
eq(revalidatedAnchor.offset, 37, "history revalidation preserves the exact viewport offset");
eq(
  remapTranscriptMiddleAnchor(
    [{ id: "u1", text: "removed" }],
    [{ id: "h1", text: "different" }],
    { atBottom: false, anchorMessageId: "u1", offset: 12 },
  ).anchorMessageId,
  "u1",
  "semantic mismatch does not guess a replacement anchor",
);

const stagedAnchor = { atBottom: false as const, anchorMessageId: "question-a", offset: -17 };
const finalAnchor = { atBottom: false as const, anchorMessageId: "question-b", offset: 43 };
let presentationSnapshot = stagePresentationRestoreSnapshot(
  { viewKey: "session-b", path: "team-legacy" },
  "session-b",
  { viewKey: "session-b", anchor: stagedAnchor },
  true,
);
eq(presentationSnapshot?.anchor?.anchorMessageId, stagedAnchor.anchorMessageId, "sidecar resolution stages anchor A");
eq(presentationSnapshot?.anchor?.offset, stagedAnchor.offset, "sidecar resolution stages anchor A offset");

presentationSnapshot = syncPresentationRestoreSnapshot(presentationSnapshot, "session-b", "team-legacy", finalAnchor);
eq(presentationSnapshot?.anchor?.anchorMessageId, finalAnchor.anchorMessageId, "legacy pre-commit scroll replaces A with exact anchor B");
eq(presentationSnapshot?.anchor?.offset, finalAnchor.offset, "legacy pre-commit scroll replaces A with exact offset B");

presentationSnapshot = syncPresentationRestoreSnapshot(presentationSnapshot, "session-b", "team-persisted", null);
eq(presentationSnapshot?.anchor?.anchorMessageId, finalAnchor.anchorMessageId, "persisted synchronous top scroll cannot overwrite anchor B");
eq(presentationSnapshot?.anchor?.offset, finalAnchor.offset, "persisted synchronous top scroll cannot overwrite offset B");

const consumedSnapshot = consumePresentationRestoreSnapshot(presentationSnapshot, "session-b", "team-persisted");
eq(consumedSnapshot?.anchor?.anchorMessageId, finalAnchor.anchorMessageId, "persisted commit consumes exact anchor B");
eq(consumedSnapshot?.anchor?.offset, finalAnchor.offset, "persisted commit consumes exact offset B");

const normalPresentation = { viewKey: "normal-session", path: "normal" as const };
eq(stagePresentationRestoreSnapshot(normalPresentation, "normal-session", null, true), null, "unknown normal position does not invent bottom intent");
let normalSnapshot = stagePresentationRestoreSnapshot(normalPresentation, "normal-session", null, true, true);
ok(normalSnapshot !== null && normalSnapshot.anchor === null, "explicit bottom intent stages a normal layout handoff");
normalSnapshot = syncPresentationRestoreSnapshot(normalSnapshot, "normal-session", "normal", finalAnchor);
eq(normalSnapshot?.anchor?.offset, finalAnchor.offset, "scroll during pending replaces bottom with the newest normal anchor");
normalSnapshot = syncPresentationRestoreSnapshot(normalSnapshot, "normal-session", "normal", null);
eq(normalSnapshot?.anchor, null, "returning to bottom replaces the pending middle anchor");
eq(consumePresentationRestoreSnapshot(normalSnapshot, "another-session", "team-persisted"), undefined, "another session cannot consume normal layout handoff");
eq(stagePresentationRestoreSnapshot(normalPresentation, "normal-session", null, false, true), null, "missing team result cannot trigger normal layout handoff");
eq(stagePresentationRestoreSnapshot({ ...normalPresentation, path: "team-pending" }, "normal-session", null, true, true), null, "blocking loading surface is not a readable handoff source");

for (const offset of [37, 0, -606.2]) {
  const canonical = { atBottom: false as const, anchorMessageId: "question-row-independent", offset };
  const row33Snapshot = stagePresentationRestoreSnapshot(
    { viewKey: "row-remap-session", path: "team-legacy" },
    "row-remap-session",
    { viewKey: "row-remap-session", anchor: canonical },
    true,
  );
  const row39Anchor = consumePresentationRestoreSnapshot(row33Snapshot, "row-remap-session", "team-persisted")?.anchor;
  eq(row39Anchor?.anchorMessageId, canonical.anchorMessageId, `presentation row 33->39 preserves canonical identity at offset ${offset}`);
  eq(row39Anchor?.offset, offset, `presentation row 33->39 preserves canonical offset ${offset}`);
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
