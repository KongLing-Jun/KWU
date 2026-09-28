import {
  resolveModelRouteAccess,
  SUBSCRIPTION_BILLING_SOURCE,
  type SubscriptionAccess,
  type SubscriptionMarketplaceView,
  type SubscriptionModelCandidate,
} from "./subscriptionModelAccess";
import type { ModelInfo } from "./types";

export type ComposerModelEntitlementSnapshot = {
  accountKey: string;
  routes: readonly SubscriptionModelCandidate[];
  marketplace?: SubscriptionMarketplaceView;
};

export type ComposerModelEntitlementPresentation =
  | { state: "resolved"; ref: string; billingSource?: ModelInfo["billingSource"] }
  | { state: "choose" }
  | { state: "pending" };

function exactTargetRoute(
  routes: readonly SubscriptionModelCandidate[],
  modelRef: string,
): SubscriptionModelCandidate | null {
  const matches = routes.filter((route) => (
    route.billingSource === SUBSCRIPTION_BILLING_SOURCE
    && (route.ref === modelRef || route.model === modelRef)
  ));
  return matches.length === 1 ? matches[0] : null;
}

export function resolveComposerModelEntitlementPresentation(input: {
  modelRef: string;
  billingSource?: ModelInfo["billingSource"] | "";
  accountKey: string;
  access: SubscriptionAccess;
  snapshot: ComposerModelEntitlementSnapshot | null;
}): ComposerModelEntitlementPresentation {
  const modelRef = input.modelRef.trim();
  if (input.billingSource !== SUBSCRIPTION_BILLING_SOURCE) {
    return {
      state: "resolved",
      ref: modelRef,
      billingSource: input.billingSource === "newapi_byok" ? input.billingSource : undefined,
    };
  }
  if (!modelRef) return { state: "choose" };
  if (input.access.state === "loading") return { state: "pending" };
  if (input.access.state !== "ready") return { state: "choose" };
  if (!input.snapshot || input.snapshot.accountKey !== input.accountKey) {
    return { state: "pending" };
  }

  const route = exactTargetRoute(input.snapshot.routes, modelRef);
  const routeAccess = resolveModelRouteAccess(
    route,
    input.snapshot.marketplace,
    input.access,
    false,
    true,
  );
  if (routeAccess.state === "loading") return { state: "pending" };
  if (routeAccess.state !== "allowed" || !route) return { state: "choose" };
  return {
    state: "resolved",
    ref: route.ref?.trim() || route.model,
    billingSource: SUBSCRIPTION_BILLING_SOURCE,
  };
}
