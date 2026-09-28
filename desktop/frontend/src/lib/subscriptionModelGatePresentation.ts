import { useEffect, useState } from "react";

export const SUBSCRIPTION_MODEL_PENDING_DELAY_MS = 2_000;

type ModelGateLabelInput = {
  state: "loading" | "allowed" | "blocked";
  transitioning?: boolean;
  routeDisplayName?: string | null;
  routeModel?: string | null;
  sessionMetaLabel?: string | null;
  tabLabel?: string | null;
  fallbackLabel: string;
  blockedLabel: string;
};

function firstLabel(...labels: Array<string | null | undefined>): string {
  for (const label of labels) {
    const normalized = label?.trim();
    if (normalized) return normalized;
  }
  return "";
}

export function resolveSubscriptionModelGateLabel({
  state,
  transitioning = false,
  routeDisplayName,
  routeModel,
  sessionMetaLabel,
  tabLabel,
  fallbackLabel,
  blockedLabel,
}: ModelGateLabelInput): string {
  if (transitioning) {
    return firstLabel(sessionMetaLabel, fallbackLabel);
  }
  if (state === "blocked") return blockedLabel;
  if (state === "loading") {
    return firstLabel(sessionMetaLabel, tabLabel, fallbackLabel);
  }
  return firstLabel(routeDisplayName, routeModel, sessionMetaLabel, tabLabel, fallbackLabel);
}

export function useDelayedSubscriptionModelPending(
  factsKey: string,
  loading: boolean,
  delayMs = SUBSCRIPTION_MODEL_PENDING_DELAY_MS,
): boolean {
  const [pendingState, setPendingState] = useState({ factsKey, visible: false });
  const visible = loading && pendingState.factsKey === factsKey && pendingState.visible;

  useEffect(() => {
    setPendingState({ factsKey, visible: false });
    if (!loading) return;

    const timer = window.setTimeout(() => {
      setPendingState({ factsKey, visible: true });
    }, delayMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [delayMs, factsKey, loading]);

  return visible;
}
