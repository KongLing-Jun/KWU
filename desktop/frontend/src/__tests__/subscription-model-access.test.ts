// Run: tsx src/__tests__/subscription-model-access.test.ts

import {
  exactSavedModelRoute,
  findSubscriptionDefaultRepairCandidate,
  filterModelInfosForSubscriptionAccess,
  filterSettingsRefsForSubscriptionAccess,
  isBlankSessionEligibleForSubscriptionRepair,
  resolveSubscriptionAccess,
  resolveActiveModelRoute,
  resolveModelRouteAccess,
  subscriptionMarketplaceNeedsRefresh,
  subscriptionMarketplaceRefreshSucceeded,
  topicTurnEvidenceKey,
  uniqueCurrentModelRoute,
  type SubscriptionAccess,
  type SubscriptionMarketplaceView,
} from "../lib/subscriptionModelAccess";

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

const planCases = [
  ["free", "free", "Subscribe_Free"],
  ["lite", "active", "Subscribe_Lite"],
  ["plus", "active", "Subscribe_Plus"],
  ["pro", "active", "Subscribe_Pro"],
] as const;

console.log("\nsubscription access policy");

for (const [plan, status, requiredTag] of planCases) {
  eq(
    resolveSubscriptionAccess({ initialized: true, hasBinding: true, allowSubscriptionModels: true, plan, status }),
    { state: "ready", plan, requiredTag },
    `${plan}/${status} resolves its exact subscription tag`,
  );
}

eq(
  resolveSubscriptionAccess({ initialized: false, hasBinding: true, allowSubscriptionModels: true, plan: "pro", status: "active" }),
  { state: "loading" },
  "uninitialized account evidence stays loading",
);

for (const [label, input] of [
  ["missing binding", { initialized: true, hasBinding: false, allowSubscriptionModels: true, plan: "pro", status: "active" }],
  ["existing availability denial", { initialized: true, hasBinding: true, allowSubscriptionModels: false, plan: "pro", status: "active" }],
  ["expired subscription", { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "pro", status: "expired" }],
  ["unknown plan", { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "enterprise", status: "active" }],
  ["wrong plan casing", { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "Pro", status: "active" }],
  ["wrong status casing", { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "pro", status: "Active" }],
  ["free/active mismatch", { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "free", status: "active" }],
  ["lite/free mismatch", { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "lite", status: "free" }],
  ["missing snapshot", { initialized: true, hasBinding: true, allowSubscriptionModels: true }],
] as const) {
  eq(resolveSubscriptionAccess(input), { state: "unavailable" }, `${label} fails closed`);
}

type Route = {
  ref: string;
  model: string;
  billingSource: "subscription_pool" | "newapi_byok";
  disabled?: boolean;
};

const routes: readonly Route[] = [
  { ref: "subscription/shared", model: "shared", billingSource: "subscription_pool" },
  { ref: "byok/shared", model: "shared", billingSource: "newapi_byok" },
  { ref: "subscription/free-only", model: "free-only", billingSource: "subscription_pool" },
  { ref: "subscription/pro-only", model: "pro-only", billingSource: "subscription_pool" },
  { ref: "subscription/approximate", model: "approximate", billingSource: "subscription_pool" },
  { ref: "subscription/lowercase", model: "lowercase", billingSource: "subscription_pool" },
  { ref: "subscription/untagged", model: "untagged", billingSource: "subscription_pool" },
  { ref: "subscription/missing-tags", model: "missing-tags", billingSource: "subscription_pool" },
  { ref: "subscription/missing-model", model: "missing-model", billingSource: "subscription_pool" },
  { ref: "byok/disabled", model: "disabled", billingSource: "newapi_byok", disabled: true },
];

const marketplace: SubscriptionMarketplaceView = {
  sources: [
    {
      id: "international",
      models: [
        { name: "shared", tags: ["Subscribe_Free", "Subscribe_Lite", "Subscribe_Plus", "Subscribe_Pro"] },
        { name: "free-only", tags: ["Subscribe_Free"] },
        { name: "pro-only", tags: ["Subscribe_Pro"] },
        { name: "approximate", tags: ["Subscribe_Pro_Test"] },
        { name: "lowercase", tags: ["subscribe_pro"] },
        { name: "untagged", tags: [] },
        { name: "missing-tags" },
      ],
    },
  ],
};

function ready(plan: "free" | "lite" | "plus" | "pro", status: "free" | "active"): SubscriptionAccess {
  return resolveSubscriptionAccess({ initialized: true, hasBinding: true, allowSubscriptionModels: true, plan, status });
}

function refs(access: SubscriptionAccess, view: SubscriptionMarketplaceView | null | undefined = marketplace): string[] {
  return filterModelInfosForSubscriptionAccess(routes, view, access).map((route) => route.ref);
}

console.log("\nsubscription model candidate filtering");

eq(refs(ready("free", "free")), ["subscription/shared", "byok/shared", "subscription/free-only"], "Free uses only the exact Free tag and preserves BYOK");
eq(refs(ready("lite", "active")), ["subscription/shared", "byok/shared"], "Lite does not inherit Free models");
eq(refs(ready("plus", "active")), ["subscription/shared", "byok/shared"], "Plus does not inherit lower-plan models");
eq(refs(ready("pro", "active")), ["subscription/shared", "byok/shared", "subscription/pro-only"], "Pro uses only the exact Pro tag");

for (const [label, access, view] of [
  ["account loading", { state: "loading" } as SubscriptionAccess, marketplace],
  ["account unavailable", { state: "unavailable" } as SubscriptionAccess, marketplace],
  ["missing marketplace", ready("pro", "active"), null],
  ["missing international source", ready("pro", "active"), { sources: [{ id: "domestic", models: [{ name: "pro-only", tags: ["Subscribe_Pro"] }] }] }],
  ["duplicate international source", ready("pro", "active"), { sources: [{ id: "international", models: [] }, { id: "international", models: [] }] }],
  ["missing source models", ready("pro", "active"), { sources: [{ id: "international" }] }],
] as const) {
  eq(refs(access, view), ["byok/shared"], `${label} hides subscription routes but preserves BYOK`);
}
eq(
  filterModelInfosForSubscriptionAccess(routes, undefined, ready("pro", "active")).map((route) => route.ref),
  ["byok/shared"],
  "undefined marketplace evidence hides subscription routes but preserves BYOK",
);

console.log("\nsubscription marketplace refresh policy");

for (const [label, view] of [
  ["missing marketplace", null],
  ["missing sources", {}],
  ["missing international source", { sources: [{ id: "domestic", models: [{ name: "domestic-ready" }] }] }],
  ["duplicate international sources", { sources: [
    { id: "international", models: [{ name: "first" }] },
    { id: "international", models: [{ name: "second" }] },
  ] }],
  ["fresh domestic source cannot mask unavailable international pricing", { sources: [
    { id: "domestic", pricingState: "ready", metadataStatus: "ready", models: [{ name: "domestic-ready" }] },
    { id: "international", pricingState: "unavailable", metadataStatus: "ready", models: [] },
  ] }],
  ["fresh domestic source cannot mask unavailable international metadata", { sources: [
    { id: "domestic", pricingState: "fresh", metadataStatus: "ready", models: [{ name: "domestic-ready" }] },
    { id: "international", pricingState: "fresh", metadataStatus: "metadata_unavailable", models: [] },
  ] }],
  ["stale populated cache", { sources: [{
    id: "international",
    pricingState: "stale_usable",
    metadataStatus: "ready",
    models: [{ name: "cached-model" }],
  }] }],
  ["populated cache with refresh error", { sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [{ name: "cached-model" }],
    errorCode: "pricing_unavailable",
  }] }],
  ["metadata write failure", { sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "metadata_write_failed",
    models: [{ name: "cached-model" }],
  }] }],
] as const) {
  ok(subscriptionMarketplaceNeedsRefresh(view), `${label} requires refresh`);
  ok(!subscriptionMarketplaceRefreshSucceeded(view), `${label} is not a successful refresh result`);
}

for (const [label, view] of [
  ["fresh empty international source", { sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [],
  }] }],
  ["fresh populated international source", { sources: [{
    id: "international",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [{ name: "cached-model" }],
  }] }],
] as const) {
  ok(!subscriptionMarketplaceNeedsRefresh(view), `${label} does not refresh-loop`);
  ok(subscriptionMarketplaceRefreshSucceeded(view), `${label} is an authoritative refresh result`);
}

const coldMarketplace: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    pricingState: "unavailable",
    metadataStatus: "metadata_unavailable",
    models: [],
  }],
};
const coldRoutes: readonly Route[] = [
  { ref: "subscription/deepseek-v4-flash", model: "deepseek-v4-flash", billingSource: "subscription_pool" },
  { ref: "subscription/gpt-5.6-luna", model: "gpt-5.6-luna", billingSource: "subscription_pool" },
  { ref: "subscription/other", model: "other", billingSource: "subscription_pool" },
  { ref: "byok/gpt-5.6-luna", model: "gpt-5.6-luna", billingSource: "newapi_byok" },
];
const coldProRefs = filterModelInfosForSubscriptionAccess(
  coldRoutes,
  coldMarketplace,
  ready("pro", "active"),
).map((route) => route.ref);
eq(
  coldProRefs,
  ["subscription/deepseek-v4-flash", "byok/gpt-5.6-luna"],
  "cold marketplace permits only the authenticated universal default and BYOK",
);
eq(
  coldProRefs,
  coldRoutes
    .filter((route) => resolveModelRouteAccess(
      route,
      coldMarketplace,
      ready("pro", "active"),
      false,
      true,
    ).state === "allowed")
    .map((route) => route.ref),
  "candidate filtering stays in parity with the canonical route access decision",
);
for (const [label, access] of [
  ["loading", { state: "loading" } as SubscriptionAccess],
  ["unavailable", { state: "unavailable" } as SubscriptionAccess],
] as const) {
  eq(
    filterModelInfosForSubscriptionAccess(coldRoutes, coldMarketplace, access).map((route) => route.ref),
    ["byok/gpt-5.6-luna"],
    `${label} access hides the universal subscription route while preserving BYOK`,
  );
}

const duplicateModelMarketplace: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    models: [
      { name: "shared", tags: ["Subscribe_Pro"] },
      { name: "shared", tags: ["Subscribe_Pro"] },
      { name: "pro-only", tags: ["Subscribe_Pro"] },
    ],
  }],
};
eq(
  refs(ready("pro", "active"), duplicateModelMarketplace),
  ["byok/shared", "subscription/pro-only"],
  "duplicate marketplace model IDs fail closed without hiding the same-name BYOK route",
);

const routesBefore = JSON.stringify(routes);
const marketplaceBefore = JSON.stringify(marketplace);
const filteredRoutes = filterModelInfosForSubscriptionAccess(routes, marketplace, ready("pro", "active"));
ok(filteredRoutes !== routes, "filtering returns a new candidate array");
ok(filteredRoutes.every((route) => routes.includes(route)), "filtering preserves original candidate objects");
eq(JSON.stringify(routes), routesBefore, "filtering does not mutate candidate input");
eq(JSON.stringify(marketplace), marketplaceBefore, "filtering does not mutate marketplace input");
ok(!filteredRoutes.some((route) => route.disabled), "disabled routes remain excluded");

console.log("\nsettings ref adapter");

type Provider = { name: string; billingSource: "subscription_pool" | "newapi_byok" };
const providers: readonly Provider[] = [
  { name: "subscription", billingSource: "subscription_pool" },
  { name: "byok", billingSource: "newapi_byok" },
];
const settingsRefs = ["subscription/pro-only", "subscription/free-only", "byok/pro-only"] as const;
const filteredSettingsRefs = filterSettingsRefsForSubscriptionAccess(
  settingsRefs,
  providers,
  marketplace,
  ready("pro", "active"),
  (ref, availableProviders) => {
    const [providerName, ...modelParts] = ref.split("/");
    return {
      model: modelParts.join("/"),
      billingSource: availableProviders.find((provider) => provider.name === providerName)?.billingSource,
    };
  },
);
eq(filteredSettingsRefs, ["subscription/pro-only", "byok/pro-only"], "settings refs use the same policy through a thin ProviderView adapter");
eq(settingsRefs, ["subscription/pro-only", "subscription/free-only", "byok/pro-only"], "settings ref filtering leaves the input array unchanged");

console.log("\nsubscription default repair policy");

const repairRoutes: readonly Route[] = [
  { ref: "subscription/deepseek-v4-flash", model: "deepseek-v4-flash", billingSource: "subscription_pool" },
  { ref: "byok/deepseek-v4-flash", model: "deepseek-v4-flash", billingSource: "newapi_byok" },
  { ref: "subscription/other", model: "other", billingSource: "subscription_pool" },
];
const repairMarketplace: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    models: [
      { name: "deepseek-v4-flash", tags: ["Subscribe_Free", "Subscribe_Lite", "Subscribe_Plus", "Subscribe_Pro"] },
      { name: "other", tags: ["Subscribe_Free"] },
    ],
  }],
};

eq(
  findSubscriptionDefaultRepairCandidate(repairRoutes, repairMarketplace, ready("free", "free"))?.ref,
  "subscription/deepseek-v4-flash",
  "repair selects the unique exact DeepSeek subscription route",
);
eq(
  findSubscriptionDefaultRepairCandidate(
    [...repairRoutes, { ref: "subscription-alt/deepseek-v4-flash", model: "deepseek-v4-flash", billingSource: "subscription_pool" }],
    repairMarketplace,
    ready("free", "free"),
  ),
  null,
  "duplicate DeepSeek subscription routes fail closed",
);
eq(
  findSubscriptionDefaultRepairCandidate(
    repairRoutes,
    { sources: [{ id: "international", models: [{ name: "deepseek-v4-flash", tags: ["Subscribe_Free"] }, { name: "deepseek-v4-flash", tags: ["Subscribe_Free"] }] }] },
    ready("free", "free"),
  ),
  null,
  "duplicate marketplace model IDs fail closed for repair",
);
for (const [label, view] of [
  ["missing marketplace", undefined],
  ["wrong tag", { sources: [{ id: "international", models: [{ name: "deepseek-v4-flash", tags: ["Subscribe_Pro"] }] }] }],
  ["approximate tag", { sources: [{ id: "international", models: [{ name: "deepseek-v4-flash", tags: ["Subscribe_Free_Test"] }] }] }],
] as const) {
  eq(findSubscriptionDefaultRepairCandidate(repairRoutes, view, ready("free", "free")), null, `${label} rejects repair`);
}
eq(
  findSubscriptionDefaultRepairCandidate(
    repairRoutes.map((route) => route.billingSource === "subscription_pool" && route.model === "deepseek-v4-flash" ? { ...route, disabled: true } : route),
    repairMarketplace,
    ready("free", "free"),
  ),
  null,
  "disabled DeepSeek route rejects repair",
);
eq(
  findSubscriptionDefaultRepairCandidate(
    [{ ref: "   ", model: "deepseek-v4-flash", billingSource: "subscription_pool" }],
    repairMarketplace,
    ready("free", "free"),
  ),
  null,
  "blank repair candidate ref is rejected",
);
eq(
  findSubscriptionDefaultRepairCandidate(
    [{ model: "deepseek-v4-flash", billingSource: "subscription_pool" }],
    repairMarketplace,
    ready("free", "free"),
  ),
  null,
  "missing repair candidate ref is rejected",
);
eq(
  resolveModelRouteAccess(repairRoutes[1], undefined, { state: "loading" }),
  { state: "allowed" },
  "BYOK remains allowed while account and marketplace evidence load",
);
for (const [label, route] of [
  ["missing billing source", { ref: "legacy/deepseek", model: "deepseek-v4-flash" }],
  ["empty billing source", { ref: "legacy/deepseek", model: "deepseek-v4-flash", billingSource: "" }],
  ["unknown billing source", { ref: "legacy/deepseek", model: "deepseek-v4-flash", billingSource: "legacy" }],
] as const) {
  eq(resolveModelRouteAccess(route, repairMarketplace, ready("free", "free")), { state: "unavailable" }, `${label} is not treated as BYOK`);
}
eq(
  resolveModelRouteAccess(repairRoutes[0], repairMarketplace, { state: "loading" }),
  { state: "loading" },
  "subscription route waits for account evidence",
);
eq(uniqueCurrentModelRoute([{ ...repairRoutes[0], current: true }, repairRoutes[1]])?.ref, repairRoutes[0].ref, "one current route is accepted");
eq(uniqueCurrentModelRoute([{ ...repairRoutes[0], current: true }, { ...repairRoutes[1], current: true }]), null, "ambiguous current routes fail closed");
eq(resolveActiveModelRoute(repairRoutes, repairRoutes[0].ref, "subscription_pool")?.ref, repairRoutes[0].ref, "full ref resolves one active route");
eq(resolveActiveModelRoute(repairRoutes, repairRoutes[0].model, "subscription_pool")?.ref, repairRoutes[0].ref, "model ID resolves one active route");
eq(resolveActiveModelRoute(repairRoutes, repairRoutes[1].model, "newapi_byok")?.ref, repairRoutes[1].ref, "billing source isolates same-name BYOK route");
eq(resolveActiveModelRoute(repairRoutes, "missing", "subscription_pool"), null, "zero active label matches fail closed");
eq(resolveActiveModelRoute(repairRoutes, "", "subscription_pool"), null, "missing active label fails closed");
eq(resolveActiveModelRoute(repairRoutes, repairRoutes[0].model, ""), null, "missing active billing source fails closed");
eq(
  resolveActiveModelRoute(
    [...repairRoutes, { ref: "subscription/deepseek-duplicate", model: repairRoutes[0].model, billingSource: "subscription_pool" }],
    repairRoutes[0].model,
    "subscription_pool",
  ),
  null,
  "same-model same-source ambiguity fails closed",
);
eq(
  resolveActiveModelRoute([{ ...repairRoutes[0], current: true }, repairRoutes[2]], repairRoutes[2].ref, "subscription_pool")?.ref,
  repairRoutes[0].ref,
  "one explicit current route takes priority over active metadata",
);
eq(
  resolveActiveModelRoute(
    [{ ...repairRoutes[0], current: true }, { ...repairRoutes[1], current: true }, repairRoutes[2]],
    repairRoutes[2].ref,
    "subscription_pool",
  ),
  null,
  "multiple explicit current routes never use metadata fallback",
);
eq(exactSavedModelRoute(repairRoutes, repairRoutes[1].ref, "newapi_byok")?.ref, repairRoutes[1].ref, "saved route identity includes billing source");
eq(exactSavedModelRoute(repairRoutes, repairRoutes[1].ref, "subscription_pool"), null, "same ref with a different billing source does not match");

console.log("\nsubscription repair blank-session evidence");

const tabAKey = topicTurnEvidenceKey({ id: "tab-a", scope: "project", workspaceRoot: "/repo", topicId: "topic-a" });
const tabBKey = topicTurnEvidenceKey({ id: "tab-b", scope: "global", workspaceRoot: "", topicId: "topic-b" });
const blankInput = {
  tabKey: tabAKey,
  turnEvidence: { key: tabAKey, state: "ready" as const, turns: 0 },
  historyLoaded: true,
  hydrating: false,
  backendActivationPending: false,
  readOnly: false,
  running: false,
  hasHistory: false,
  hasCheckpoints: false,
  hasLiveTurn: false,
};
ok(isBlankSessionEligibleForSubscriptionRepair(blankInput), "hydrated zero-turn tab is eligible for repair");
for (const [label, patch] of [
  ["history hydration pending", { historyLoaded: false }],
  ["history currently restoring", { hydrating: true }],
  ["history hydration failed", { hydrateError: "failed" }],
  ["topic turns loading", { turnEvidence: { key: tabAKey, state: "loading" as const } }],
  ["topic turns unavailable", { turnEvidence: { key: tabAKey, state: "unavailable" as const } }],
  ["historical topic turns", { turnEvidence: { key: tabAKey, state: "ready" as const, turns: 1 } }],
  ["stale prior-tab turns", { tabKey: tabBKey }],
  ["local history", { hasHistory: true }],
  ["checkpoint history", { hasCheckpoints: true }],
  ["live turn", { hasLiveTurn: true }],
  ["read only tab", { readOnly: true }],
  ["running tab", { running: true }],
  ["activation pending", { backendActivationPending: true }],
] as const) {
  ok(!isBlankSessionEligibleForSubscriptionRepair({ ...blankInput, ...patch }), `${label} fails closed`);
}

process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
