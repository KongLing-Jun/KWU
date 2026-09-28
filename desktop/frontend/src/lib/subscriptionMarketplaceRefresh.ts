import {
  claimSubscriptionMarketplaceRefresh,
  settleSubscriptionMarketplaceRefresh,
  subscriptionMarketplaceRefreshSucceeded,
  type SubscriptionMarketplaceRefreshScopeState,
  type SubscriptionMarketplaceView,
} from "./subscriptionModelAccess";

const DEFAULT_RETRY_DELAYS_MS = [0, 1_000, 4_000, 15_000, 60_000] as const;

export type SubscriptionMarketplaceRetryWait = (delayMs: number, signal: AbortSignal) => Promise<boolean>;

function waitForRetry(delayMs: number, signal: AbortSignal): Promise<boolean> {
  if (signal.aborted) return Promise.resolve(false);
  if (delayMs <= 0) return Promise.resolve(true);
  return new Promise((resolve) => {
    const onAbort = () => {
      globalThis.clearTimeout(timer);
      resolve(false);
    };
    const timer = globalThis.setTimeout(() => {
      signal.removeEventListener("abort", onAbort);
      resolve(true);
    }, delayMs);
    signal.addEventListener("abort", onAbort, { once: true });
  });
}

export interface SubscriptionMarketplaceRefreshRequest {
  accountScopeStates: Map<string, SubscriptionMarketplaceRefreshScopeState>;
  accountScopeKey: string;
  refreshMarketplace: () => Promise<SubscriptionMarketplaceView>;
  readMarketplace: () => Promise<SubscriptionMarketplaceView>;
  refreshModelCatalog: () => void | Promise<void>;
  force?: boolean;
  retryDelaysMs?: readonly number[];
  wait?: SubscriptionMarketplaceRetryWait;
}

export async function refreshSubscriptionMarketplaceOnce({
  accountScopeStates,
  accountScopeKey,
  refreshMarketplace,
  readMarketplace,
  refreshModelCatalog,
  force = false,
  retryDelaysMs = DEFAULT_RETRY_DELAYS_MS,
  wait = waitForRetry,
}: SubscriptionMarketplaceRefreshRequest): Promise<boolean> {
  const controller = claimSubscriptionMarketplaceRefresh(accountScopeStates, accountScopeKey, force);
  if (!controller) return false;

  for (let attempt = 0; !controller.signal.aborted; attempt += 1) {
    const delayMs = retryDelaysMs[Math.min(attempt, retryDelaysMs.length - 1)] ?? 60_000;
    if (!await wait(delayMs, controller.signal) || controller.signal.aborted) return false;
    try {
      const refreshed = await refreshMarketplace();
      if (controller.signal.aborted || !subscriptionMarketplaceRefreshSucceeded(refreshed)) continue;
      const cached = await readMarketplace();
      if (controller.signal.aborted || !subscriptionMarketplaceRefreshSucceeded(cached)) continue;
      await refreshModelCatalog();
      if (!settleSubscriptionMarketplaceRefresh(accountScopeStates, accountScopeKey, controller)) return false;
      return true;
    } catch {
      // Refresh, cache verification, and catalog publication failures share the bounded backoff.
    }
  }
  return false;
}
