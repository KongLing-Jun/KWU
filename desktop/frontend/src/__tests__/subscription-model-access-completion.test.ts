// Run: tsx src/__tests__/subscription-model-access-completion.test.ts

import { readFileSync, readdirSync } from "node:fs";
import {
  filterModelInfosForSubscriptionAccess,
  resolveSubscriptionAccess,
  type SubscriptionAccessInput,
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

type Route = {
  ref: string;
  model: string;
  billingSource: "subscription_pool" | "newapi_byok";
};

const routes: readonly Route[] = [
  { ref: "subscription/shared", model: "shared", billingSource: "subscription_pool" },
  { ref: "byok/shared", model: "shared", billingSource: "newapi_byok" },
  { ref: "subscription/free", model: "free", billingSource: "subscription_pool" },
  { ref: "subscription/lite", model: "lite", billingSource: "subscription_pool" },
  { ref: "subscription/plus", model: "plus", billingSource: "subscription_pool" },
  { ref: "subscription/pro", model: "pro", billingSource: "subscription_pool" },
  { ref: "subscription/multi", model: "multi", billingSource: "subscription_pool" },
];

const marketplace: SubscriptionMarketplaceView = {
  sources: [{
    id: "international",
    models: [
      { name: "shared", tags: ["Subscribe_Free", "Subscribe_Lite", "Subscribe_Plus", "Subscribe_Pro"] },
      { name: "free", tags: ["Subscribe_Free"] },
      { name: "lite", tags: ["Subscribe_Lite"] },
      { name: "plus", tags: ["Subscribe_Plus"] },
      { name: "pro", tags: ["Subscribe_Pro"] },
      { name: "multi", tags: ["Subscribe_Free", "Subscribe_Pro"] },
    ],
  }],
};

const lifecycle: Array<{ label: string; input: SubscriptionAccessInput; expected: string[] }> = [
  {
    label: "login bootstrap",
    input: { initialized: false, hasBinding: false, allowSubscriptionModels: false },
    expected: ["byok/shared"],
  },
  {
    label: "login completion",
    input: { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "free", status: "free" },
    expected: ["subscription/shared", "byok/shared", "subscription/free", "subscription/multi"],
  },
  {
    label: "account switch",
    input: { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "pro", status: "active" },
    expected: ["subscription/shared", "byok/shared", "subscription/pro", "subscription/multi"],
  },
  {
    label: "manual refresh",
    input: { initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "plus", status: "active" },
    expected: ["subscription/shared", "byok/shared", "subscription/plus"],
  },
  {
    label: "logout",
    input: { initialized: true, hasBinding: true, allowSubscriptionModels: false, plan: "plus", status: "active" },
    expected: ["byok/shared"],
  },
];

console.log("\nsubscription model access completion criteria");

const routesBefore = JSON.stringify(routes);
for (const [plan, status, expected] of [
  ["free", "free", ["subscription/shared", "byok/shared", "subscription/free", "subscription/multi"]],
  ["lite", "active", ["subscription/shared", "byok/shared", "subscription/lite"]],
  ["plus", "active", ["subscription/shared", "byok/shared", "subscription/plus"]],
  ["pro", "active", ["subscription/shared", "byok/shared", "subscription/pro", "subscription/multi"]],
] as const) {
  eq(
    filterModelInfosForSubscriptionAccess(
      routes,
      marketplace,
      resolveSubscriptionAccess({ initialized: true, hasBinding: true, allowSubscriptionModels: true, plan, status }),
    ).map((route) => route.ref),
    expected,
    `${plan} uses its exact tag without lower-tier inheritance`,
  );
}
for (const step of lifecycle) {
  const access = resolveSubscriptionAccess(step.input);
  eq(
    filterModelInfosForSubscriptionAccess(routes, marketplace, access).map((route) => route.ref),
    step.expected,
    `${step.label} recomputes exact-tag access and preserves same-name BYOK`,
  );
}
eq(JSON.stringify(routes), routesBefore, "login, switch, refresh, and logout filtering never rewrite current route inputs");

for (const [label, view] of [
  ["marketplace error", undefined],
  ["marketplace missing source", { sources: [] }],
] as const) {
  eq(
    filterModelInfosForSubscriptionAccess(
      routes,
      view,
      resolveSubscriptionAccess({ initialized: true, hasBinding: true, allowSubscriptionModels: true, plan: "pro", status: "active" }),
    ).map((route) => route.ref),
    ["byok/shared"],
    `${label} fails subscription closed without hiding same-name BYOK`,
  );
}

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
const switcherSource = readFileSync(new URL("../components/ModelSwitcher.tsx", import.meta.url), "utf8");
const marketplaceSource = readFileSync(new URL("../features/model-marketplace/ui/ModelMarketplace.tsx", import.meta.url), "utf8");
const settingsSource = readFileSync(new URL("../features/settings/ui/SettingsPanel.tsx", import.meta.url), "utf8");
const heartbeatSource = readFileSync(new URL("../features/automation/heartbeat/HeartbeatPanel.tsx", import.meta.url), "utf8");
const settingsBehaviorSource = readFileSync(new URL("./settings-subscription-model-access.test.tsx", import.meta.url), "utf8");
const heartbeatBehaviorSource = readFileSync(new URL("./heartbeat-subscription-model-access.test.tsx", import.meta.url), "utf8");

ok(
  /const subscriptionAccess = useMemo\(\(\) => resolveSubscriptionAccess/.test(appSource)
    && /sidebarAccountPlan,[\s\S]*sidebarAccountSubscriptionStatus/.test(appSource),
  "App derives shared access from reactive account identity, plan, and status evidence",
);

const entryChecks = [
  ["ModelSwitcher", switcherSource, "filterModelInfosForSubscriptionAccess"],
  ["ModelMarketplace", marketplaceSource, "filterModelInfosForSubscriptionAccess"],
  ["Settings and Bots", settingsSource, "filterSettingsRefsForSubscriptionAccess"],
  ["Heartbeat", heartbeatSource, "filterModelInfosForSubscriptionAccess"],
] as const;
for (const [entry, source, sharedFilter] of entryChecks) {
  ok(source.includes(sharedFilter) && source.includes("subscriptionAccess"), `${entry} consumes shared access through the common filtering policy`);
}
ok(
  (appSource.match(/<SettingsPanel/g) ?? []).length === 2
    && (appSource.match(/<HeartbeatPanel/g) ?? []).length === 2
    && (appSource.match(/subscriptionAccess=\{subscriptionAccess\}/g) ?? []).length >= 6,
  "all production Settings/Bots, Heartbeat, Marketplace, and switcher call sites receive shared access",
);
ok(settingsBehaviorSource.includes("bot picker shares the same authorized refs"), "Bots is covered as a distinct settings entry");
ok(settingsBehaviorSource.includes("perform zero settings writes"), "Settings and Bots lifecycle tests assert zero implicit writes");
ok(heartbeatBehaviorSource.includes("perform zero task writes"), "Heartbeat lifecycle tests assert zero implicit writes");
ok(settingsBehaviorSource.includes("same-name filtering leaves exactly one BYOK option"), "Settings covers same-name BYOK preservation");
ok(heartbeatBehaviorSource.includes("same-name subscription and BYOK automation routes remain distinct"), "Heartbeat covers same-name route identity");

const discoveredTests = readdirSync(new URL(".", import.meta.url))
  .filter((name) => /\.test\.tsx?$/.test(name));
for (const name of [
  "subscription-model-access.test.ts",
  "model-switcher-provider-label.test.tsx",
  "model-marketplace.test.tsx",
  "settings-subscription-model-access.test.tsx",
  "heartbeat-subscription-model-access.test.tsx",
  "subscription-model-access-completion.test.ts",
]) {
  ok(discoveredTests.includes(name), `pnpm test:all discovers ${name}`);
}

process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
