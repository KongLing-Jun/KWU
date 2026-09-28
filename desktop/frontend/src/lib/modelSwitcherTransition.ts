import type { EffortInfo, ModelInfo } from "./types";

export type ModelSwitcherTransition = {
  tabId: string;
  startedFactsKey: string;
  previousRef: string;
  previousBillingSource?: ModelInfo["billingSource"];
  previousEffort?: EffortInfo;
};

export function hasReliableComposerSelectionSnapshot(input: {
  selectionPhase: "pending" | "ready" | "degraded" | "updating-effort";
  selectionModelRef: string;
  effort?: EffortInfo;
  effortSnapshot?: string;
}): boolean {
  if (!input.selectionModelRef.trim()) return false;
  if (input.selectionPhase === "ready" || input.selectionPhase === "updating-effort") {
    return input.effort !== undefined;
  }
  return input.selectionPhase === "pending" && input.effortSnapshot !== undefined;
}

export function shouldHoldComposerSelectionPresentation(input: {
  selectionPhase: "pending" | "ready" | "degraded" | "updating-effort";
  selectionModelRef: string;
  presentedModelRef: string;
  transitionPending: boolean;
  surfaceTransitionActive: boolean;
  hasReliableSnapshot?: boolean;
}): boolean {
  if (
    input.transitionPending
    || (input.selectionPhase === "pending" && !input.hasReliableSnapshot)
    || (input.surfaceTransitionActive && !input.hasReliableSnapshot)
  ) {
    return true;
  }
  const confirmedModelRef = input.selectionPhase === "ready" || input.selectionPhase === "updating-effort"
    ? input.selectionModelRef.trim()
    : "";
  return Boolean(
    confirmedModelRef
    && confirmedModelRef !== input.presentedModelRef.trim()
  );
}

export function resolveModelSwitcherTransition(input: {
  transition: ModelSwitcherTransition | null;
  activeTabId: string;
  currentFactsKey: string;
  currentFactsState: "loading" | "settled";
  currentRef: string;
  currentBillingSource?: ModelInfo["billingSource"];
  selectionPhase: "pending" | "ready" | "degraded" | "updating-effort";
  selectionModelRef: string;
}): {
  ref: string;
  billingSource?: ModelInfo["billingSource"];
  effort?: EffortInfo;
  pending: boolean;
} {
  const transitionMatches = input.transition?.tabId === input.activeTabId;
  const transitionFactsPending = transitionMatches
    && (
      input.currentFactsKey === input.transition?.startedFactsKey
      || input.currentFactsState !== "settled"
    );
  const replacementReady = input.selectionPhase !== "pending"
    && input.selectionModelRef.trim() === input.currentRef.trim();
  const transitionPending = transitionMatches
    && (transitionFactsPending || !replacementReady);
  if (transitionPending && input.transition) {
    return {
      ref: input.transition.previousRef,
      billingSource: input.transition.previousBillingSource,
      effort: input.transition.previousEffort,
      pending: true,
    };
  }
  return {
    ref: input.currentRef,
    billingSource: input.currentBillingSource,
    pending: false,
  };
}
