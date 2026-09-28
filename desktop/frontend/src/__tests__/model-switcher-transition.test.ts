// Run: tsx src/__tests__/model-switcher-transition.test.ts

import {
  hasReliableComposerSelectionSnapshot,
  resolveModelSwitcherTransition,
  shouldHoldComposerSelectionPresentation,
  type ModelSwitcherTransition,
} from "../lib/modelSwitcherTransition";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
}

const transition: ModelSwitcherTransition = {
  tabId: "tab-a",
  startedFactsKey: "facts-old",
  previousRef: "kwu/deepseek-v4-flash",
  previousBillingSource: "subscription_pool",
  previousEffort: { supported: true, current: "high", default: "auto", levels: ["auto", "high"] },
};
const resolve = (
  currentFactsKey: string,
  currentFactsState: "loading" | "settled",
  currentRef: string,
  activeTabId = "tab-a",
  selectionPhase: "pending" | "ready" | "degraded" | "updating-effort" = "ready",
  selectionModelRef = currentRef,
) => resolveModelSwitcherTransition({
  transition,
  activeTabId,
  currentFactsKey,
  currentFactsState,
  currentRef,
  currentBillingSource: "subscription_pool",
  selectionPhase,
  selectionModelRef,
});

console.log("\nmodel switcher explicit transition");

const oldSettled = resolve("facts-old", "settled", "");
ok(oldSettled.pending && oldSettled.ref.endsWith("flash"), "the pre-switch settled facts cannot clear explicit pending");
ok(oldSettled.effort?.current === "high", "pending preserves the previous confirmed effort with the model");

const nextLoading = resolve("facts-next", "loading", "");
ok(nextLoading.pending && nextLoading.ref.endsWith("flash"), "a new loading facts generation preserves the old confirmed identity");

const success = resolve("facts-next", "settled", "kwu/deepseek-v4-pro");
ok(!success.pending && success.ref.endsWith("pro"), "new settled facts replace the old identity after success");

const selectionStillPending = resolve(
  "facts-next",
  "settled",
  "kwu/deepseek-v4-pro",
  "tab-a",
  "pending",
  "kwu/deepseek-v4-pro",
);
ok(
  selectionStillPending.pending && selectionStillPending.ref.endsWith("flash") && selectionStillPending.effort?.current === "high",
  "settled catalog facts keep the previous pair until the replacement effort is ready",
);

const mismatchedSelection = resolve(
  "facts-next",
  "settled",
  "kwu/deepseek-v4-pro",
  "tab-a",
  "ready",
  "kwu/deepseek-v4-flash",
);
ok(mismatchedSelection.pending && mismatchedSelection.ref.endsWith("flash"), "a mismatched ready selection cannot release the previous pair");

const failure = resolve("facts-next", "settled", "kwu/deepseek-v4-flash");
ok(!failure.pending && failure.ref.endsWith("flash"), "new settled facts restore the old identity after failure");

const otherTab = resolve("facts-old", "settled", "kwu/other-tab-model", "tab-b");
ok(!otherTab.pending && otherTab.ref === "kwu/other-tab-model", "a tab switch cannot inherit another tab's pending identity");

const sameRefByok = resolveModelSwitcherTransition({
  transition: {
    ...transition,
    previousRef: "shared/model",
    previousBillingSource: "newapi_byok",
  },
  activeTabId: "tab-a",
  currentFactsKey: "facts-old",
  currentFactsState: "settled",
  currentRef: "shared/model",
  currentBillingSource: "subscription_pool",
  selectionPhase: "ready",
  selectionModelRef: "shared/model",
});
ok(sameRefByok.billingSource === "newapi_byok", "pending preserves billing source when refs collide across sources");

ok(
  shouldHoldComposerSelectionPresentation({
    selectionPhase: "ready",
    selectionModelRef: "custom/next",
    presentedModelRef: "",
    transitionPending: false,
    surfaceTransitionActive: false,
  }),
  "a ready combined pair remains hidden while catalog facts still expose an empty identity",
);
ok(
  shouldHoldComposerSelectionPresentation({
    selectionPhase: "ready",
    selectionModelRef: "custom/next",
    presentedModelRef: "custom/previous",
    transitionPending: false,
    surfaceTransitionActive: false,
  }),
  "a ready combined pair remains hidden while catalog facts expose another identity",
);
ok(
  !shouldHoldComposerSelectionPresentation({
    selectionPhase: "ready",
    selectionModelRef: "custom/next",
    presentedModelRef: "custom/next",
    transitionPending: false,
    surfaceTransitionActive: false,
  }),
  "the confirmed model and effort are released when the presented identity catches up",
);
ok(
  !shouldHoldComposerSelectionPresentation({
    selectionPhase: "updating-effort",
    selectionModelRef: "custom/next",
    presentedModelRef: "custom/next",
    transitionPending: false,
    surfaceTransitionActive: false,
  }),
  "an effort-only update keeps the matching model and optimistic effort visible",
);
ok(
  shouldHoldComposerSelectionPresentation({
    selectionPhase: "updating-effort",
    selectionModelRef: "custom/next",
    presentedModelRef: "custom/previous",
    transitionPending: false,
    surfaceTransitionActive: false,
  }),
  "an effort-only update still guards against a mismatched presented model",
);
ok(
  !shouldHoldComposerSelectionPresentation({
    selectionPhase: "degraded",
    selectionModelRef: "custom/next",
    presentedModelRef: "custom/previous",
    transitionPending: false,
    surfaceTransitionActive: false,
  }),
  "an explicit degraded result remains available instead of waiting forever for effort",
);

ok(
  hasReliableComposerSelectionSnapshot({
    selectionPhase: "pending",
    selectionModelRef: "custom/target",
    effortSnapshot: "high",
  }),
  "a persisted target model and effort form a reliable busy snapshot",
);
ok(
  hasReliableComposerSelectionSnapshot({
    selectionPhase: "ready",
    selectionModelRef: "custom/no-effort",
    effort: { supported: false, current: "auto", default: "auto", levels: [] },
  }),
  "authoritative unsupported effort is a reliable model-only snapshot",
);
ok(
  !hasReliableComposerSelectionSnapshot({
    selectionPhase: "pending",
    selectionModelRef: "custom/legacy",
  }),
  "a legacy identity without effort capability evidence remains unknown",
);
ok(
  !shouldHoldComposerSelectionPresentation({
    selectionPhase: "pending",
    selectionModelRef: "custom/target",
    presentedModelRef: "custom/target",
    transitionPending: false,
    surfaceTransitionActive: true,
    hasReliableSnapshot: true,
  }),
  "a reliable target snapshot remains visible throughout a surface transition",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
