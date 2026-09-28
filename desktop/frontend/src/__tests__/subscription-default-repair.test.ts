// Run: tsx src/__tests__/subscription-default-repair.test.ts

import { readFileSync } from "node:fs";

import {
  activeTopicTurnsFromTree,
  claimSubscriptionMarketplaceRefresh,
  isBlankSessionEligibleForSubscriptionRepair,
  resolveModelRouteAccess,
  resolveSubscriptionAccess,
  resolveSubscriptionAccountState,
  resolveSubscriptionDefaultRepair,
  resolveSubscriptionModelInvocationAccess,
  runSubscriptionDefaultRepair,
  settleSubscriptionDefaultRepairRunState,
  type SubscriptionMarketplaceRefreshScopeState,
  shouldBlockSubscriptionDefaultRepair,
  subscriptionDefaultRepairAttemptKey,
  subscriptionDefaultRepairScopeKey,
  subscriptionMarketplaceNeedsRefresh,
  subscriptionRepairFactsKey,
  topicTurnEvidenceKey,
  type SubscriptionAccess,
  type SubscriptionMarketplaceView,
} from "../lib/subscriptionModelAccess";
import type { ProjectNode } from "../lib/types";

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
  ok(JSON.stringify(actual) === JSON.stringify(expected), `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

type Route = {
  ref: string;
  model: string;
  billingSource: "subscription_pool" | "newapi_byok";
  current?: boolean;
  disabled?: boolean;
};

const deepseekRoute: Route = {
  ref: "subscription/deepseek-v4-flash",
  model: "deepseek-v4-flash",
  billingSource: "subscription_pool",
  current: true,
};
const byokSameName: Route = {
  ref: "byok/deepseek-v4-flash",
  model: "deepseek-v4-flash",
  billingSource: "newapi_byok",
  current: true,
};

function marketplace(tags: readonly string[], models = 1): SubscriptionMarketplaceView {
  return {
    sources: [{
      id: "international",
      models: Array.from({ length: models }, () => ({ name: "deepseek-v4-flash", tags })),
    }],
  };
}

function access(plan: "free" | "lite" | "plus" | "pro"): SubscriptionAccess {
  return resolveSubscriptionAccess({
    initialized: true,
    hasBinding: true,
    allowSubscriptionModels: true,
    plan,
    status: plan === "free" ? "free" : "active",
  });
}

console.log("\nsubscription default repair");

eq(resolveSubscriptionAccountState({
  initialized: false,
  available: true,
  accountStatus: "authenticated",
  hasBinding: true,
  access: { state: "loading" },
}), "loading", "account bootstrap remains loading");
eq(resolveSubscriptionAccountState({
  initialized: true,
  available: true,
  accountStatus: "anonymous",
  hasBinding: false,
  access: { state: "unavailable" },
}), "anonymous", "anonymous is distinct from unavailable");
eq(resolveSubscriptionAccountState({
  initialized: true,
  available: false,
  accountStatus: "offline",
  hasBinding: true,
  access: { state: "unavailable" },
}), "unavailable", "offline account is unavailable");
eq(resolveSubscriptionAccountState({
  initialized: true,
  available: true,
  accountStatus: "authenticated",
  hasBinding: true,
  access: access("pro"),
}), "ready", "authenticated entitled account is ready");

eq(
  resolveModelRouteAccess(byokSameName, undefined, { state: "loading" }),
  { state: "allowed" },
  "BYOK remains usable while account evidence loads",
);
eq(
  resolveModelRouteAccess(deepseekRoute, undefined, { state: "loading" }, true),
  { state: "loading" },
  "subscription route waits for account and marketplace evidence",
);
eq(
  resolveModelRouteAccess(deepseekRoute, undefined, access("free"), false, true),
  { state: "allowed" },
  "universal blank-session default remains usable with a cold marketplace cache",
);
eq(
  resolveModelRouteAccess(
    { ...deepseekRoute, model: "gpt-5.4", ref: "subscription/gpt-5.4" },
    undefined,
    access("free"),
    false,
    true,
  ),
  { state: "unavailable" },
  "cold marketplace cache never grants GPT access",
);
ok(subscriptionMarketplaceNeedsRefresh(undefined), "missing Marketplace metadata requests one recovery refresh");
ok(subscriptionMarketplaceNeedsRefresh({
  sources: [{
    id: "international",
    pricingState: "unavailable",
    metadataStatus: "metadata_unavailable",
    models: [],
  }],
}), "unavailable empty Marketplace metadata requests recovery");
ok(!subscriptionMarketplaceNeedsRefresh({
  sources: [{ id: "international", pricingState: "fresh", metadataStatus: "ready", models: [] }],
}), "an authoritative empty Marketplace result does not create a refresh loop");
const marketplaceRefreshScopes = new Map<string, SubscriptionMarketplaceRefreshScopeState>();
ok(claimSubscriptionMarketplaceRefresh(marketplaceRefreshScopes, "profile-a\u0000user-a\u00001"), "first account scope claims refresh");
ok(!claimSubscriptionMarketplaceRefresh(marketplaceRefreshScopes, "profile-a\u0000user-a\u00001"), "StrictMode replay cannot duplicate an account refresh");
marketplaceRefreshScopes.set("profile-a\u0000user-a\u00001", { state: "settled" });
ok(!claimSubscriptionMarketplaceRefresh(marketplaceRefreshScopes, "profile-a\u0000user-a\u00001"), "automatic replay cannot reopen a settled scope");
ok(claimSubscriptionMarketplaceRefresh(marketplaceRefreshScopes, "profile-a\u0000user-a\u00001", true), "explicit account refresh can reopen a settled scope");
ok(claimSubscriptionMarketplaceRefresh(marketplaceRefreshScopes, "profile-a\u0000user-b\u00001"), "a different account scope can refresh independently");
eq(resolveSubscriptionModelInvocationAccess({
  route: deepseekRoute,
  routeAccess: { state: "loading" },
  factsState: "loading",
  auxiliaryBillingSource: "subscription_pool",
}), { state: "allowed" }, "subscription invocation does not wait for route entitlement evidence");
eq(resolveSubscriptionModelInvocationAccess({
  route: deepseekRoute,
  routeAccess: { state: "unavailable" },
  factsState: "settled",
  auxiliaryBillingSource: "subscription_pool",
}), { state: "allowed" }, "client-side entitlement results do not block sends");
eq(resolveSubscriptionModelInvocationAccess({
  route: deepseekRoute,
  routeAccess: { state: "allowed" },
  factsState: "settled",
  auxiliaryBillingSource: "subscription_pool",
}), { state: "allowed" }, "authorized subscription route remains sendable");
eq(resolveSubscriptionModelInvocationAccess({
  route: null,
  routeAccess: { state: "loading" },
  factsState: "loading",
  auxiliaryBillingSource: "newapi_byok",
}), { state: "allowed" }, "BYOK auxiliary evidence remains sendable while facts load");
ok(!shouldBlockSubscriptionDefaultRepair({
  hasByokRoute: false,
  relevant: true,
  classificationPending: true,
  blankEligible: false,
  accountState: "loading",
  factsSettled: false,
  decisionState: "blocked",
}), "loading subscription context does not gate blank/history classification");
ok(!shouldBlockSubscriptionDefaultRepair({
  hasByokRoute: true,
  relevant: true,
  classificationPending: true,
  blankEligible: false,
  accountState: "loading",
  factsSettled: false,
  decisionState: "blocked",
}), "loading account never gates an authoritative BYOK route");
ok(!shouldBlockSubscriptionDefaultRepair({
  hasByokRoute: false,
  relevant: true,
  classificationPending: false,
  blankEligible: true,
  accountState: "anonymous",
  factsSettled: true,
  decisionState: "blocked",
}), "anonymous blank subscription context remains sendable during repair");
ok(!shouldBlockSubscriptionDefaultRepair({
  hasByokRoute: false,
  relevant: true,
  classificationPending: false,
  blankEligible: false,
  accountState: "unavailable",
  factsSettled: true,
  decisionState: "blocked",
}), "confirmed historical context is outside the phase-two blank-session gate");

for (const [plan, tag] of [
  ["free", "Subscribe_Free"],
  ["lite", "Subscribe_Lite"],
  ["plus", "Subscribe_Plus"],
  ["pro", "Subscribe_Pro"],
] as const) {
  eq(resolveSubscriptionDefaultRepair(
    { defaultModel: deepseekRoute.ref, defaultBillingSource: "subscription_pool" },
    [deepseekRoute],
    marketplace([tag]),
    access(plan),
  ), { state: "keep" }, `${plan} legal saved default requires no write`);
}

eq(resolveSubscriptionDefaultRepair(
  { defaultModel: byokSameName.ref, defaultBillingSource: "newapi_byok" },
  [byokSameName, { ...deepseekRoute, current: false }],
  marketplace(["Subscribe_Pro"]),
  access("pro"),
), { state: "keep" }, "same-name BYOK default is preserved exactly");

const repair = resolveSubscriptionDefaultRepair(
  { defaultModel: "", defaultBillingSource: "" },
  [deepseekRoute],
  marketplace(["Subscribe_Pro"]),
  access("pro"),
);
ok(repair.state === "repair" && repair.candidate.ref === deepseekRoute.ref, "one exact entitled DeepSeek route is repairable");

for (const [label, routes, view] of [
  ["wrong tag", [deepseekRoute], marketplace(["Subscribe_Plus"])],
  ["missing tags", [deepseekRoute], marketplace([])],
  ["duplicate marketplace model", [deepseekRoute], marketplace(["Subscribe_Pro"], 2)],
  ["disabled route", [{ ...deepseekRoute, disabled: true }], marketplace(["Subscribe_Pro"])],
] as const) {
  eq(resolveSubscriptionDefaultRepair(
    { defaultModel: "", defaultBillingSource: "" },
    routes,
    view,
    access("pro"),
  ), { state: "blocked" }, `${label} fails closed`);
}

eq(resolveSubscriptionDefaultRepair(
  {
    defaultModel: "",
    defaultBillingSource: "",
    plannerModel: "byok/planner",
  },
  [deepseekRoute, { ref: "byok/planner", model: "planner", billingSource: "newapi_byok" }],
  marketplace(["Subscribe_Pro"]),
  access("pro"),
), { state: "blocked" }, "cross-source derived model conflict causes zero write");

const attempted = new Set<string>();
let writes = 0;
let claims = 0;
const runInput = {
  attemptedKeys: attempted,
  attemptKey: "account-v7/tab/topic/facts",
  isTargetCurrent: () => true,
  isScopeCurrent: () => true,
  onClaim: () => {
    claims += 1;
  },
  apply: async () => {
    writes += 1;
    return true;
  },
};
eq(await runSubscriptionDefaultRepair(runInput), "applied", "first StrictMode effect applies");
eq(await runSubscriptionDefaultRepair(runInput), "duplicate", "StrictMode replay is deduplicated");
eq(writes, 1, "StrictMode and rerender write at most once");
eq(claims, 1, "applied-state rerender cannot put a duplicate run back into loading");
eq(await runSubscriptionDefaultRepair({
  attemptedKeys: new Set(),
  attemptKey: "old-account/old-tab",
  isTargetCurrent: () => false,
  isScopeCurrent: () => false,
  apply: async () => {
    writes += 1;
    return true;
  },
}), "stale", "stale tab or account context cannot write");
eq(writes, 1, "stale context leaves write count unchanged");
let failedWrites = 0;
let failedNotifications = 0;
const failedAttempted = new Set<string>();
const failedRun = {
  attemptedKeys: failedAttempted,
  attemptKey: "same-failed-facts",
  isTargetCurrent: () => true,
  isScopeCurrent: () => true,
  apply: async () => {
    failedWrites += 1;
    throw new Error("setter failed");
  },
  onApplied: () => {
    failedNotifications += 1;
  },
};
eq(await runSubscriptionDefaultRepair(failedRun), "failed", "setter failure reaches a terminal result");
eq(await runSubscriptionDefaultRepair(failedRun), "duplicate", "setter failure does not retry the same fact key");
eq(failedWrites, 1, "failed setter is called once for one fact key");
eq(failedNotifications, 0, "failed setter emits no success action");
let successNotifications = 0;
eq(await runSubscriptionDefaultRepair({
  attemptedKeys: new Set(),
  attemptKey: "successful-facts",
  isTargetCurrent: () => true,
  isScopeCurrent: () => true,
  apply: async () => true,
  onApplied: () => {
    successNotifications += 1;
  },
}), "applied", "matching context completes repair");
eq(successNotifications, 1, "successful matching repair emits one completion action");
let targetCurrent = true;
let selfInvalidatingNotifications = 0;
eq(await runSubscriptionDefaultRepair({
  attemptedKeys: new Set(),
  attemptKey: "self-invalidating-facts",
  isTargetCurrent: () => targetCurrent,
  isScopeCurrent: () => true,
  apply: async () => {
    targetCurrent = false;
    return true;
  },
  onApplied: () => {
    selfInvalidatingNotifications += 1;
  },
}), "applied", "successful writes complete when only mutable repair facts change");
eq(selfInvalidatingNotifications, 1, "self-invalidating facts still emit one completion action");

let scopeCurrent = true;
let scopedWrites = 0;
let staleNotifications = 0;
eq(await runSubscriptionDefaultRepair({
  attemptedKeys: new Set(),
  attemptKey: "switched-scope",
  isTargetCurrent: () => true,
  isScopeCurrent: () => scopeCurrent,
  apply: async (isScopeCurrent) => {
    scopedWrites += 1;
    scopeCurrent = false;
    if (!isScopeCurrent()) return false;
    scopedWrites += 1;
    return true;
  },
  onApplied: () => {
    staleNotifications += 1;
  },
}), "stale", "account or tab switch during setters remains stale");
eq(scopedWrites, 1, "scope switch prevents the later tab write");
eq(staleNotifications, 0, "scope switch emits no success action");

eq(await runSubscriptionDefaultRepair({
  attemptedKeys: new Set(),
  attemptKey: "post-write-refresh-failed",
  isTargetCurrent: () => true,
  isScopeCurrent: () => true,
  apply: async () => true,
  onApplied: () => {
    throw new Error("facts refresh failed");
  },
}), "applied", "post-write facts refresh failure cannot reclassify successful setters");

const oldRunningState = {
  scopeKey: "account/tab/session",
  attemptKey: "mutable-facts",
  state: "running" as const,
};
eq(settleSubscriptionDefaultRepairRunState(oldRunningState, {
  scopeKey: oldRunningState.scopeKey,
  attemptKey: oldRunningState.attemptKey,
  result: "stale",
  currentScopeKey: "other-account/tab/session",
}), null, "switching away clears the completed old-scope running state");
eq(settleSubscriptionDefaultRepairRunState(oldRunningState, {
  scopeKey: oldRunningState.scopeKey,
  attemptKey: oldRunningState.attemptKey,
  result: "applied",
  currentScopeKey: oldRunningState.scopeKey,
}), null, "applied completion cannot leak running state into a later rerender");
eq(settleSubscriptionDefaultRepairRunState(oldRunningState, {
  scopeKey: oldRunningState.scopeKey,
  attemptKey: oldRunningState.attemptKey,
  result: "failed",
  currentScopeKey: "other-account/tab/session",
}), null, "failed state from an old scope cannot pollute a switch back");
eq(settleSubscriptionDefaultRepairRunState(oldRunningState, {
  scopeKey: oldRunningState.scopeKey,
  attemptKey: oldRunningState.attemptKey,
  result: "failed",
  currentScopeKey: oldRunningState.scopeKey,
}), { ...oldRunningState, state: "failed" }, "setter failure remains terminal only in its current scope");

const globalTab = { id: "tab-g", scope: "global", workspaceRoot: "", topicId: "topic-g" };
const projectTab = { id: "tab-p", scope: "project", workspaceRoot: "D:/repo", topicId: "topic-p" };
const globalTopicWithoutTurns: ProjectNode = {
  key: "global-topic-g",
  kind: "global_topic",
  label: "Global topic",
  topicId: globalTab.topicId,
};
const projectTopicWithoutTurns: ProjectNode = {
  key: "project-topic-p",
  kind: "topic",
  label: "Project topic",
  root: projectTab.workspaceRoot,
  topicId: projectTab.topicId,
};
eq(activeTopicTurnsFromTree([globalTopicWithoutTurns], globalTab), 0, "matched global topic without turns is blank");
eq(activeTopicTurnsFromTree([projectTopicWithoutTurns], projectTab), 0, "matched project topic without turns is blank");
eq(activeTopicTurnsFromTree([{ ...projectTopicWithoutTurns, turns: 2 }], projectTab), 2, "explicit project turns preserve history");
eq(activeTopicTurnsFromTree([projectTopicWithoutTurns], { ...projectTab, topicId: "missing-topic" }), undefined, "missing target topic remains unavailable");

const omittedTurns = activeTopicTurnsFromTree([projectTopicWithoutTurns], projectTab);
const omittedTurnsKey = topicTurnEvidenceKey(projectTab);
ok(isBlankSessionEligibleForSubscriptionRepair({
  tabKey: omittedTurnsKey,
  turnEvidence: omittedTurns === undefined
    ? { key: omittedTurnsKey, state: "unavailable" }
    : { key: omittedTurnsKey, state: "ready", turns: omittedTurns },
  historyLoaded: true,
  hydrating: false,
  backendActivationPending: false,
  readOnly: false,
  running: false,
  hasHistory: false,
  hasCheckpoints: false,
  hasLiveTurn: false,
}), "matched omitted turns reaches blank-session repair eligibility");

for (const [label, tab] of [["global", globalTab], ["project", projectTab]] as const) {
  const key = topicTurnEvidenceKey(tab);
  ok(isBlankSessionEligibleForSubscriptionRepair({
    tabKey: key,
    turnEvidence: { key, state: "ready", turns: 0 },
    historyLoaded: true,
    hydrating: false,
    backendActivationPending: false,
    readOnly: false,
    running: false,
    hasHistory: false,
    hasCheckpoints: false,
    hasLiveTurn: false,
  }), `${label} blank session is eligible`);
}
ok(!isBlankSessionEligibleForSubscriptionRepair({
  tabKey: topicTurnEvidenceKey(projectTab),
  turnEvidence: { key: topicTurnEvidenceKey(globalTab), state: "ready", turns: 0 },
  historyLoaded: true,
  hydrating: false,
  backendActivationPending: false,
  readOnly: false,
  running: false,
  hasHistory: false,
  hasCheckpoints: false,
  hasLiveTurn: false,
}), "stale turn evidence cannot misclassify history as blank");
ok(!isBlankSessionEligibleForSubscriptionRepair({
  tabKey: topicTurnEvidenceKey(projectTab),
  turnEvidence: { key: topicTurnEvidenceKey(projectTab), state: "ready", turns: 1 },
  historyLoaded: true,
  hydrating: false,
  backendActivationPending: false,
  readOnly: false,
  running: false,
  hasHistory: false,
  hasCheckpoints: false,
  hasLiveTurn: false,
}), "project-tree history prevents blank repair");

const firstKey = subscriptionDefaultRepairAttemptKey({
  accountKey: "profile/user/1/subscription-v1",
  tabKey: "tab/topic",
  hydrationGeneration: 3,
  settings: { defaultModel: "", defaultBillingSource: "" },
  routes: [deepseekRoute],
  marketplace: marketplace(["Subscribe_Pro"]),
});
const nextAccountKey = subscriptionDefaultRepairAttemptKey({
  accountKey: "profile/user/2/subscription-v2",
  tabKey: "tab/topic",
  hydrationGeneration: 3,
  settings: { defaultModel: "", defaultBillingSource: "" },
  routes: [deepseekRoute],
  marketplace: marketplace(["Subscribe_Pro"]),
});
ok(firstKey !== nextAccountKey, "account and subscription generation changes invalidate attempt context");
const nextMarketplaceKey = subscriptionDefaultRepairAttemptKey({
  accountKey: "profile/user/1/subscription-v1",
  tabKey: "tab/topic",
  hydrationGeneration: 3,
  settings: { defaultModel: "", defaultBillingSource: "" },
  routes: [deepseekRoute],
  marketplace: marketplace(["Subscribe_Plus"]),
});
ok(firstKey !== nextMarketplaceKey, "marketplace entitlement tag changes invalidate attempt context");
const stableScopeKey = subscriptionDefaultRepairScopeKey({
  accountKey: "profile/user/1/subscription-v1",
  tabKey: "tab/topic",
  hydrationGeneration: 3,
});
eq(stableScopeKey, subscriptionDefaultRepairScopeKey({
  accountKey: "profile/user/1/subscription-v1",
  tabKey: "tab/topic",
  hydrationGeneration: 3,
}), "mutable settings and route facts do not participate in the stable repair scope");
ok(stableScopeKey !== subscriptionDefaultRepairScopeKey({
  accountKey: "profile/user/2/subscription-v2",
  tabKey: "tab/topic",
  hydrationGeneration: 3,
}), "real account changes invalidate the stable repair scope");
ok(stableScopeKey !== subscriptionDefaultRepairScopeKey({
  accountKey: "profile/user/1/subscription-v1",
  tabKey: "other-tab/topic",
  hydrationGeneration: 3,
}), "real tab changes invalidate the stable repair scope");
ok(stableScopeKey !== subscriptionDefaultRepairScopeKey({
  accountKey: "profile/user/1/subscription-v1",
  tabKey: "tab/topic",
  hydrationGeneration: 4,
}), "session generation changes invalidate the stable repair scope");

const staleFactsKey = subscriptionRepairFactsKey({
  accountKey: "profile/user/1",
  tabID: "tab",
  revision: 1,
  activeLabel: "gpt-5.4",
  activeBillingSource: "subscription_pool",
});
const selectedFactsKey = subscriptionRepairFactsKey({
  accountKey: "profile/user/1",
  tabID: "tab",
  revision: 2,
  activeLabel: "deepseek-v4-flash",
  activeBillingSource: "subscription_pool",
});
ok(staleFactsKey !== selectedFactsKey, "manual legal model selection invalidates old gate facts");

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
const subscriptionAccessSource = readFileSync(new URL("../lib/subscriptionModelAccess.ts", import.meta.url), "utf8");
const marketplaceRefreshSource = readFileSync(new URL("../lib/subscriptionMarketplaceRefresh.ts", import.meta.url), "utf8");
ok(appSource.includes("app.ModelsForTab(tabID)"), "App reads tab-specific routes");
ok(appSource.includes("resolveActiveModelRoute("), "App resolves current route from strict active metadata fallback");
ok(
    /const savedModelRef = activeTab\s*\?\s*activeTab\.modelRef\?\.trim\(\) \?\? ""\s*:\s*state\.meta\?\.modelRef\?\.trim\(\) \?\? ""/.test(appSource)
    && /const resolvedModelSwitcherCurrentRef = currentModelRoute\?\.ref\?\.trim\(\)\s*\|\| savedModelRef\s*\|\| ""/.test(appSource)
    && /const composerModelLabel = composerModelEntitlementPresentation\?\.state === "resolved"[\s\S]*?: modelSwitcherPresentation\.ref/.test(appSource)
    && /modelLabel=\{composerModelLabel\}/.test(appSource),
  "ModelSwitcher uses the active tab saved model identity before Meta and never parses human labels",
);
ok(
  appSource.includes("activeLabel: savedModelRef")
    && /resolveActiveModelRoute\(\s*subscriptionRepairRoutes,\s*savedModelRef,/.test(appSource),
  "controller label changes do not invalidate facts or participate in current route resolution",
);
ok(
  /const resolvedModelSwitcherCurrentBillingSource = currentModelRoute\?\.billingSource\s*\?\? \(activeTab\?\.billingSource/.test(appSource)
    && /const composerModelBillingSource = composerModelEntitlementPresentation\?\.state === "resolved"[\s\S]*?: modelSwitcherPresentation\.billingSource/.test(appSource)
    && /currentModelBillingSource=\{composerModelBillingSource\}/.test(appSource),
  "ModelSwitcher pairs the authoritative route ref with its billing source before tab fallback",
);
ok(
  appSource.includes("startedFactsKey: currentSubscriptionRepairFactsKey")
    && appSource.includes("previousRef: resolvedModelSwitcherCurrentRef")
    && appSource.includes("&& !modelSwitcherTransition")
    && appSource.includes("transitionPending: modelSwitcherPresentation.pending")
    && appSource.includes("const sessionComposerSelectionBusy = sessionComposerSelectionPending")
    && appSource.includes("modelSwitchPending={sessionComposerSelectionBusy}"),
  "explicit model switching preserves the prior identity and disables only the selector until new facts settle",
);
ok(appSource.includes("app.Settings()"), "App reads persisted settings");
ok(appSource.includes("app.ModelMarketplace()"), "App uses cache-only marketplace read");
ok(
  appSource.includes("tabID ? app.ModelsForTab(tabID) : app.Models()")
    && /Promise\.allSettled\(\[\s*tabID \? app\.ModelsForTab\(tabID\) : app\.Models\(\),\s*app\.Settings\(\),\s*app\.ModelMarketplace\(\),/.test(appSource),
  "active conversations and project-only surfaces share one parallel catalog loader",
);
ok(
  appSource.includes("refreshSubscriptionMarketplaceOnce({")
    && appSource.includes("refreshMarketplace: () => app.RefreshModelMarketplace()")
    && appSource.includes("readMarketplace: () => app.ModelMarketplace()")
    && appSource.includes("ensureModelMarketplaceRefresh(accountScopeKey, true)")
    && appSource.includes("subscriptionMarketplaceNeedsRefresh(subscriptionRepairMarketplace)")
    && appSource.includes("cancelSubscriptionMarketplaceRefresh(marketplaceRefreshScopeStatesRef.current)")
    && appSource.includes('sidebarAccount?.status === "authenticated"')
    && marketplaceRefreshSource.includes("claimSubscriptionMarketplaceRefresh(")
    && marketplaceRefreshSource.includes("subscriptionMarketplaceRefreshSucceeded(")
    && marketplaceRefreshSource.includes("await refreshModelCatalog()"),
  "App owns one authenticated background recovery, cancels obsolete scopes, and re-reads shared facts after settlement",
);
const defaultRepairWrite = appSource.indexOf("app.SetDefaultModelRoute(candidateRef, \"subscription_pool\")");
const tabRepairWrite = appSource.indexOf("app.SetModelRouteForTab(tabID, candidateRef, \"subscription_pool\")");
const repairClaim = appSource.indexOf("onClaim: () =>", defaultRepairWrite - 1000);
const repairRunning = appSource.indexOf('state: "running"', repairClaim);
const repairCompletion = appSource.indexOf("onApplied: () =>", tabRepairWrite);
ok(defaultRepairWrite >= 0, "repair persists the validated ref as the global default");
ok(tabRepairWrite > defaultRepairWrite, "repair updates the explicit tab only after persisting the global default");
ok(repairCompletion > tabRepairWrite, "repair completion actions require both default and tab writes");
ok(appSource.includes("refreshSubscriptionRepairFacts();"), "successful repair re-reads real backend settings and routes");
ok(subscriptionAccessSource.includes('return { ...current, state: "failed" }'), "setter failure records a terminal UI state");
ok(appSource.includes("!subscriptionRepairRunFailed"), "failed repair leaves the confirming state");
ok(repairClaim >= 0 && repairRunning > repairClaim && repairRunning < defaultRepairWrite, "running state begins only after the attempt is claimed");
ok(
  appSource.includes("onAuthenticated={refreshAccountAfterAuthentication}")
    && /const refreshModelCatalog = useCallback\(\(\) => \{\s*refreshSubscriptionRepairFacts\(\);/.test(appSource)
    && appSource.includes("modelCatalog={sharedModelCatalog}"),
  "explicit authentication enters the same shared model-catalog refresh path",
);
ok(appSource.includes("if (subscriptionModelInvocationBlocked) throw new Error(\"current model is unavailable\")"), "central commitThenSend rejects unavailable model invocation");
ok(appSource.includes("if (!pendingPlanRevision || state.running || subscriptionModelInvocationBlocked) return"), "pending plan revision waits behind the model gate");
ok(appSource.includes("if (!controllerReady || subscriptionModelInvocationBlocked) return"), "goal and transcript prompt paths honor the model gate");
ok(appSource.includes("subscriptionModelInvocationBlocked || hydratePlaceholderActive"), "edit and rewind resend paths honor the model gate");
ok(appSource.includes("await send(trimmed, submitText.trim());"), "read-only management commands bypass the model gate without committing rewind");
ok(appSource.includes("actionLabel: t(\"subscriptionDefaultRepair.openModels\")"), "successful repair toast exposes the model-settings action");
ok(appSource.indexOf("showToast(t(\"subscriptionDefaultRepair.adjusted\")") > appSource.indexOf("onApplied: async () =>"), "success toast is emitted only from the post-setter completion branch");

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
