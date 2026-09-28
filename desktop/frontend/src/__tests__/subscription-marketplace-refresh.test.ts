// Run: tsx src/__tests__/subscription-marketplace-refresh.test.ts

import { refreshSubscriptionMarketplaceOnce } from "../lib/subscriptionMarketplaceRefresh";
import type {
  SubscriptionMarketplaceRefreshScopeState,
  SubscriptionMarketplaceView,
} from "../lib/subscriptionModelAccess";
import { cancelSubscriptionMarketplaceRefresh } from "../lib/subscriptionModelAccess";

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
  ok(
    JSON.stringify(actual) === JSON.stringify(expected),
    `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
  );
}

const unavailableView: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    pricingState: "unavailable",
    metadataStatus: "metadata_unavailable",
    models: [],
    errorCode: "pricing_unavailable",
  }],
};
const readyView: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [{ name: "gpt-5.6-luna" }],
  }],
};
const readyEmptyView: SubscriptionMarketplaceView = {
  sources: [{ id: "international", pricingState: "fresh", metadataStatus: "ready", models: [] }],
};
const partialReadyEmptyView: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [],
    partial: true,
  }],
};
const writeFailedView: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "metadata_write_failed",
    models: [{ name: "gpt-5.6-luna" }],
  }],
};
const cachedErrorView: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    pricingState: "stale_usable",
    metadataStatus: "ready",
    models: [{ name: "gpt-5.6-luna" }],
    errorCode: "pricing_unavailable",
  }],
};
const immediateRetryDelays = [0, 0, 0] as const;
const noWait = async (_delayMs: number, signal: AbortSignal) => !signal.aborted;

console.log("\nsubscription Marketplace refresh lifecycle");

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  const refreshResults = [unavailableView, readyView];
  let refreshCalls = 0;
  let cacheReads = 0;
  let catalogReads = 0;
  const request = () => refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey: "profile-a\u0000user-a\u00001",
    refreshMarketplace: async () => refreshResults[refreshCalls++] ?? readyView,
    readMarketplace: async () => {
      cacheReads += 1;
      return readyView;
    },
    refreshModelCatalog: () => {
      catalogReads += 1;
    },
    retryDelaysMs: immediateRetryDelays,
    wait: noWait,
  });

  const claims = await Promise.all([request(), request(), request()]);
  eq(claims, [true, false, false], "StrictMode-style concurrency owns one retry sequence");
  eq(refreshCalls, 2, "resolved unavailable result retries until ready");
  eq(cacheReads, 1, "only a ready refresh verifies the cache-only view");
  eq(catalogReads, 1, "one settled sequence re-reads shared catalog facts once");
  eq(accountScopeStates.get("profile-a\u0000user-a\u00001")?.state, "settled", "claimed scope settles after recovery");
}

for (const [label, firstResult] of [
  ["rejected request", new Error("network unavailable")],
  ["metadata write failure", writeFailedView],
  ["cached result with error code", cachedErrorView],
] as const) {
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  let calls = 0;
  const claimed = await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey: `profile-a\u0000${label}\u00001`,
    refreshMarketplace: async () => {
      calls += 1;
      if (calls === 1) {
        if (firstResult instanceof Error) throw firstResult;
        return firstResult;
      }
      return readyView;
    },
    readMarketplace: async () => readyView,
    refreshModelCatalog: () => {},
    retryDelaysMs: immediateRetryDelays,
    wait: noWait,
  });
  ok(claimed, `${label} claims recovery`);
  eq(calls, 2, `${label} follows the shared retry path`);
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  let refreshCalls = 0;
  let cacheReads = 0;
  await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey: "profile-a\u0000cache-verification\u00001",
    refreshMarketplace: async () => {
      refreshCalls += 1;
      return readyView;
    },
    readMarketplace: async () => {
      cacheReads += 1;
      return cacheReads === 1 ? unavailableView : readyView;
    },
    refreshModelCatalog: () => {},
    retryDelaysMs: immediateRetryDelays,
    wait: noWait,
  });
  eq(refreshCalls, 2, "a ready response without a ready cache retries");
  eq(cacheReads, 2, "every apparently ready response verifies persisted cache evidence");
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  const accountScopeKey = "profile-a\u0000publish-retry\u00001";
  let refreshCalls = 0;
  let catalogReads = 0;
  let releaseRetryWait!: () => void;
  let markRetryWait!: () => void;
  const retryWait = new Promise<void>((resolve) => {
    releaseRetryWait = resolve;
  });
  const retryWaitStarted = new Promise<void>((resolve) => {
    markRetryWait = resolve;
  });
  let waitCalls = 0;
  const recovering = refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey,
    refreshMarketplace: async () => {
      refreshCalls += 1;
      return readyView;
    },
    readMarketplace: async () => readyView,
    refreshModelCatalog: async () => {
      catalogReads += 1;
      if (catalogReads === 1) throw new Error("catalog publication failed");
    },
    retryDelaysMs: [0, 1],
    wait: async (_delayMs, signal) => {
      waitCalls += 1;
      if (waitCalls === 2) {
        markRetryWait();
        await retryWait;
      }
      return !signal.aborted;
    },
  });
  await retryWaitStarted;
  eq(accountScopeStates.get(accountScopeKey)?.state, "running", "publication failure retains the unique running owner");
  ok(!(await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey,
    force: true,
    refreshMarketplace: async () => readyView,
    readMarketplace: async () => readyView,
    refreshModelCatalog: () => {},
    wait: noWait,
  })), "force cannot create a second loop while publication recovery is pending");
  releaseRetryWait();
  ok(await recovering, "publication rejection retries through the same run");
  eq(refreshCalls, 2, "publication rejection does not leak a parallel refresh request");
  eq(catalogReads, 2, "publication is retried before the run settles");
  eq(accountScopeStates.get(accountScopeKey)?.state, "settled", "run settles only after catalog publication succeeds");
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  const accountScopeKey = "profile-a\u0000publish-cancelled\u00001";
  let refreshCalls = 0;
  let publishCalls = 0;
  let releasePublish!: () => void;
  let markPublishStarted!: () => void;
  const publish = new Promise<void>((resolve) => {
    releasePublish = resolve;
  });
  const publishStarted = new Promise<void>((resolve) => {
    markPublishStarted = resolve;
  });
  const cancelled = refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey,
    refreshMarketplace: async () => {
      refreshCalls += 1;
      return readyView;
    },
    readMarketplace: async () => readyView,
    refreshModelCatalog: async () => {
      publishCalls += 1;
      markPublishStarted();
      await publish;
    },
    wait: noWait,
  });
  await publishStarted;
  ok(!(await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey,
    force: true,
    refreshMarketplace: async () => readyView,
    readMarketplace: async () => readyView,
    refreshModelCatalog: () => {},
    wait: noWait,
  })), "force cannot overlap an in-flight catalog publication");
  cancelSubscriptionMarketplaceRefresh(accountScopeStates);
  releasePublish();
  ok(!(await cancelled), "cancelled publication cannot settle after its late completion");
  eq(refreshCalls, 1, "cancelled publication does not begin another refresh request");
  eq(publishCalls, 1, "cancelled publication has one owner and no leaked retry loop");
  eq(accountScopeStates.size, 0, "cancelled publication removes its scope owner");
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  let refreshCalls = 0;
  await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey: "profile-a\u0000authoritative-empty\u00001",
    refreshMarketplace: async () => {
      refreshCalls += 1;
      return readyEmptyView;
    },
    readMarketplace: async () => readyEmptyView,
    refreshModelCatalog: () => {},
    retryDelaysMs: immediateRetryDelays,
    wait: noWait,
  });
  eq(refreshCalls, 1, "fresh ready empty catalog is authoritative and does not retry");
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  const accountScopeKey = "profile-a\u0000partial-empty\u00001";
  let refreshCalls = 0;
  const recovered = await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey,
    refreshMarketplace: async () => {
      refreshCalls += 1;
      return refreshCalls === 1 ? partialReadyEmptyView : readyEmptyView;
    },
    readMarketplace: async () => readyEmptyView,
    refreshModelCatalog: () => {},
    retryDelaysMs: immediateRetryDelays,
    wait: noWait,
  });
  ok(recovered, "fresh ready empty partial catalog recovers after complete publication");
  eq(refreshCalls, 2, "fresh ready empty partial catalog retries instead of settling");
  eq(accountScopeStates.get(accountScopeKey)?.state, "settled", "complete empty publication settles the scope");
}

{
  const accountScopeKey = "profile-a\u0000long-recovery\u00001";
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  let refreshCalls = 0;
  const observedDelays: number[] = [];
  let releaseRunningRefresh!: () => void;
  const runningRefresh = new Promise<void>((resolve) => {
    releaseRunningRefresh = resolve;
  });
  const baseRequest = {
    accountScopeStates,
    accountScopeKey,
    readMarketplace: async () => unavailableView,
    refreshModelCatalog: () => {},
    retryDelaysMs: [0, 1, 4, 15, 60],
    wait: async (delayMs, signal) => {
      observedDelays.push(delayMs);
      return !signal.aborted;
    },
  };
  const recovered = await refreshSubscriptionMarketplaceOnce({
    ...baseRequest,
    refreshMarketplace: async () => {
      refreshCalls += 1;
      return refreshCalls < 7 ? unavailableView : readyView;
    },
    readMarketplace: async () => readyView,
  });
  ok(recovered, "background recovery continues beyond the former three-attempt limit");
  eq(refreshCalls, 7, "background recovery reaches a later successful attempt");
  eq(observedDelays, [0, 1, 4, 15, 60, 60, 60], "retry delay grows then remains capped");

  const automaticReplay = await refreshSubscriptionMarketplaceOnce({
    ...baseRequest,
    refreshMarketplace: async () => readyView,
  });
  ok(!automaticReplay, "settled scope cannot enter a duplicate effect retry loop");

  const forcedRecoveryPromise = refreshSubscriptionMarketplaceOnce({
    ...baseRequest,
    force: true,
    refreshMarketplace: async () => {
      refreshCalls += 1;
      await runningRefresh;
      return readyView;
    },
    readMarketplace: async () => readyView,
  });
  const overlappingForce = await refreshSubscriptionMarketplaceOnce({
    ...baseRequest,
    force: true,
    refreshMarketplace: async () => readyView,
  });
  ok(!overlappingForce, "force cannot duplicate an in-flight scope");
  releaseRunningRefresh();
  ok(await forcedRecoveryPromise, "explicit account refresh reopens a settled failed scope");
  eq(refreshCalls, 8, "forced recovery adds one successful refresh");
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  const accountScopeKey = "profile-a\u0000cancelled\u00001";
  let refreshCalls = 0;
  let cacheReads = 0;
  let catalogReads = 0;
  let releaseLateResponse!: () => void;
  let markRefreshStarted!: () => void;
  const lateResponse = new Promise<void>((resolve) => {
    releaseLateResponse = resolve;
  });
  const refreshStarted = new Promise<void>((resolve) => {
    markRefreshStarted = resolve;
  });
  const cancelled = refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey,
    refreshMarketplace: async () => {
      refreshCalls += 1;
      markRefreshStarted();
      await lateResponse;
      return readyView;
    },
    readMarketplace: async () => {
      cacheReads += 1;
      return readyView;
    },
    refreshModelCatalog: () => {
      catalogReads += 1;
    },
    wait: noWait,
  });
  await refreshStarted;
  cancelSubscriptionMarketplaceRefresh(accountScopeStates);
  releaseLateResponse();
  ok(!(await cancelled), "aborted scope does not settle from a late RPC response");
  eq(refreshCalls, 1, "abort stops subsequent retry requests");
  eq(cacheReads, 0, "late RPC response does not verify or publish a cache");
  eq(catalogReads, 0, "late RPC response does not refresh the model catalog");
  eq(accountScopeStates.size, 0, "abort removes the obsolete account scope");

  const replacement = await refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey: "profile-a\u0000replacement\u00001",
    refreshMarketplace: async () => readyView,
    readMarketplace: async () => readyView,
    refreshModelCatalog: () => {},
    wait: noWait,
  });
  ok(replacement, "a replacement account scope can recover after cancellation");
}

{
  const accountScopeStates = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
  const scopes: string[] = [];
  await Promise.all(["user-a", "user-b"].map((user) => refreshSubscriptionMarketplaceOnce({
    accountScopeStates,
    accountScopeKey: `profile-a\u0000${user}\u00001`,
    refreshMarketplace: async () => {
      scopes.push(user);
      return readyView;
    },
    readMarketplace: async () => readyView,
    refreshModelCatalog: () => {},
    retryDelaysMs: immediateRetryDelays,
    wait: noWait,
  })));
  eq(scopes.sort(), ["user-a", "user-b"], "different account scopes refresh independently");
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
