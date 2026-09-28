// Run: tsx src/__tests__/settings-subscription-model-access.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { SettingsPanel } from "../components/SettingsPanel";
import { derivedModelSelectionState } from "../features/settings/ui/ModelsSettingsSection";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess, SubscriptionAccountState } from "../lib/subscriptionModelAccess";
import type { ModelInfo, ModelMarketplaceView, SettingsView, SharedModelCatalogState } from "../lib/types";

let passed = 0;
let failed = 0;

eq(
  derivedModelSelectionState({
    loading: true,
    inheritsDefault: true,
    selectedRef: "",
    defaultRef: "subscription/default",
    availableRefs: ["subscription/default"],
  }),
  "loading",
  "derived model state reports loading before entitlement evidence settles",
);
eq(
  derivedModelSelectionState({
    loading: false,
    inheritsDefault: true,
    selectedRef: "",
    defaultRef: "subscription/default",
    availableRefs: ["subscription/default"],
  }),
  "inherited_available",
  "derived model state distinguishes a legal inherited default",
);
eq(
  derivedModelSelectionState({
    loading: false,
    inheritsDefault: true,
    selectedRef: "",
    defaultRef: "subscription/old",
    availableRefs: ["subscription/default"],
  }),
  "inherited_unavailable",
  "derived model state distinguishes an unavailable inherited default",
);
eq(
  derivedModelSelectionState({
    loading: false,
    inheritsDefault: false,
    selectedRef: "subscription/explicit",
    defaultRef: "subscription/default",
    availableRefs: ["subscription/explicit"],
  }),
  "explicit_available",
  "derived model state distinguishes a legal explicit value",
);
eq(
  derivedModelSelectionState({
    loading: false,
    inheritsDefault: false,
    selectedRef: "subscription/old",
    defaultRef: "subscription/default",
    availableRefs: ["subscription/explicit"],
  }),
  "explicit_unavailable",
  "derived model state preserves and marks an unauthorized explicit old value",
);

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

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};
Object.defineProperty(globalThis.window, "matchMedia", {
  configurable: true,
  value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
});
window.scrollTo = () => {};
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });

const proAccess: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
const marketplace: ModelMarketplaceView = {
  sources: [{
    id: "international",
    label: "International",
    sourceUrl: "https://pricing.example.invalid/api/pricing",
    pricingUrl: "https://pricing.example.invalid/models",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [
      { id: "shared", name: "shared", tags: ["Subscribe_Free"] },
      { id: "pro-only", name: "pro-only", tags: ["Subscribe_Pro"] },
      { id: "free-only", name: "free-only", tags: ["Subscribe_Free"] },
    ],
  }],
};

function provider(name: string, billingSource: "subscription_pool" | "newapi_byok", models: string[]): SettingsView["providers"][number] {
  return {
    name,
    builtIn: name === "kwu",
    added: true,
    kind: "openai",
    baseUrl: "",
    models,
    chatModels: models,
    visionModels: [],
    visionModelsConfigured: false,
    modelsUrl: "",
    default: models[0] ?? "",
    apiKeyEnv: "",
    keySet: false,
    requiresKey: false,
    configured: true,
    balanceUrl: "",
    contextWindow: 128_000,
    reasoningProtocol: "openai",
    supportedEfforts: [],
    defaultEffort: "",
    billingSource,
    locked: billingSource === "subscription_pool",
  };
}

function settingsFixture(): SettingsView {
  return {
    defaultModel: "kwu/free-only",
    defaultBillingSource: "subscription_pool",
    plannerModel: "kwu/free-only",
    subagentModel: "kwu/free-only",
    subagentEffort: "",
    autoPlan: "off",
    providers: [
      provider("custom", "newapi_byok", ["shared"]),
    ],
    officialProviders: [provider("kwu", "subscription_pool", ["shared", "pro-only", "free-only"])],
    permissions: { mode: "ask", allow: [], ask: [], deny: [] },
    sandbox: { bash: "enforce", network: false, workspaceRoot: "", allowWrite: [], shell: "auto" },
    agent: { temperature: 0, maxSteps: 0, plannerMaxSteps: 0, systemPrompt: "", coldResumePrune: true, reasoningLanguage: "auto" },
    bot: {
      enabled: true,
      model: "kwu/free-only",
      toolApprovalMode: "",
      maxSteps: 0,
      debounceMs: 0,
      allowlist: { enabled: false, allowAll: false, qqUsers: [], feishuUsers: [], weixinUsers: [], qqGroups: [], feishuGroups: [], weixinGroups: [] },
      qq: { enabled: false, appId: "", appSecretEnv: "", secretSet: false, sandbox: false },
      feishu: { enabled: false, domain: "feishu", appId: "", appSecretEnv: "", secretSet: false, verificationToken: "", mode: "webhook", webhookPort: 0, requireMention: false },
      weixin: { enabled: false, accountId: "", tokenEnv: "", tokenSet: false, apiBase: "" },
      connections: [{
        id: "connection-1",
        provider: "feishu",
        domain: "feishu",
        label: "Test bot",
        enabled: true,
        status: "connected",
        model: "kwu/free-only",
        toolApprovalMode: "",
        workspaceRoot: "",
        credential: { appId: "app", appSecretEnv: "BOT_SECRET", accountId: "", tokenEnv: "", secretSet: true },
        sessionMappings: [],
        lastError: "",
        createdAt: "2026-07-22T00:00:00Z",
        updatedAt: "2026-07-22T00:00:00Z",
      }],
    },
    desktopLanguage: "en",
    desktopLayoutStyle: "workbench",
    desktopTheme: "auto",
    desktopThemeStyle: "graphite",
    closeBehavior: "background",
    displayMode: "standard",
    statusBarStyle: "text",
    statusBarItems: ["model"],
    defaultToolApprovalMode: "ask",
    checkUpdates: true,
    telemetry: true,
    metrics: true,
    configPath: "/tmp/kwu/config.toml",
    providerKinds: [],
    autoApproveTools: false,
    bypass: false,
  };
}

function renderSettings(
  root: Root,
  access: SubscriptionAccess,
  modelCatalog: SharedModelCatalogState,
  activeTabId?: string,
  accountState: SubscriptionAccountState = access.state === "ready" ? "ready" : access.state,
) {
  root.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="models"
        initialFocus={{ target: "bot-allowlist", connectionId: "connection-1" }}
        activeTabId={activeTabId}
        modelCatalog={modelCatalog}
        subscriptionAccess={access}
        subscriptionAccountState={accountState}
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
}

async function pickerOptionTexts(container: HTMLElement, index: number): Promise<string[]> {
  const trigger = container.querySelectorAll<HTMLButtonElement>(".settings-model-picker__trigger")[index];
  if (!trigger) throw new Error(`missing model picker ${index}`);
  await act(async () => {
    trigger.click();
    await flushPromises();
  });
  const lists = document.querySelectorAll('[data-anchored-popover="active"][data-state="open"] .settings-model-picker__list');
  const list = lists[lists.length - 1];
  const texts = [...(list?.querySelectorAll<HTMLElement>(".settings-model-picker__option") ?? [])].map((option) => option.textContent ?? "");
  await act(async () => {
    trigger.click();
    await flushPromises();
  });
  return texts;
}

async function pickModelOption(container: HTMLElement, index: number, text: string): Promise<void> {
  const trigger = container.querySelectorAll<HTMLButtonElement>(".settings-model-picker__trigger")[index];
  if (!trigger) throw new Error(`missing model picker ${index}`);
  await act(async () => {
    trigger.click();
    await flushPromises();
  });
  const lists = document.querySelectorAll('[data-anchored-popover="active"][data-state="open"] .settings-model-picker__list');
  const list = lists[lists.length - 1];
  const option = [...(list?.querySelectorAll<HTMLButtonElement>(".settings-model-picker__option") ?? [])]
    .find((candidate) => candidate.textContent?.includes(text));
  if (!option) throw new Error(`missing model option ${text}`);
  await act(async () => {
    option.click();
    await flushPromises();
  });
}

console.log("\nsettings subscription model access");

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
const settingsPanelCalls = appSource.split("<SettingsPanel").slice(1);
ok(
  settingsPanelCalls.length === 1 && settingsPanelCalls.every((call) => (
    call.slice(0, 500).includes("modelCatalog={sharedModelCatalog}")
    && call.slice(0, 500).includes("subscriptionAccess={subscriptionAccess}")
    && call.slice(0, 500).includes("subscriptionAccountState={subscriptionAccountState}")
  )),
  "the production SettingsPanel receives the shared model catalog and account access",
);

let marketplaceCalls = 0;
let mutationCalls = 0;
window.go = {
  main: {
    App: {
      Settings: async () => settingsFixture(),
      Meta: async () => ({}),
      ListTabs: async () => [],
      MCPServers: async () => [],
      ConnectorCatalog: async () => ({ items: [] }),
      ModelMarketplace: async () => {
        marketplaceCalls += 1;
        throw new Error("SettingsPanel must not load Marketplace independently");
      },
      SetDefaultModelRoute: async () => { mutationCalls += 1; },
      SetPlannerModel: async () => { mutationCalls += 1; },
      SetSubagentModel: async () => { mutationCalls += 1; },
      SetBotSettings: async () => { mutationCalls += 1; },
    } as Partial<AppBindings> as AppBindings,
  },
};

const container = document.getElementById("root")!;
const root = createRoot(container);
await act(async () => renderSettings(root, proAccess, { state: "loading" }));
await waitFor("model settings", () => container.querySelectorAll(".settings-model-picker__trigger").length === 3);
ok(marketplaceCalls === 0, "models page does not start an independent marketplace request");
ok(
    container.querySelector(".settings-model-picker__selected")?.textContent?.includes("正在确认可用订阅模型") === true,
  "a filtered current value remains visible with a loading hint while marketplace is pending",
);
ok(
  [...container.querySelectorAll<HTMLElement>("[data-model-selection-state]")].slice(0, 2)
    .every((node) => node.dataset.modelSelectionState === "loading"),
  "planner and subagent inheritance state remains loading while marketplace is pending",
);
ok(
    container.querySelector(".settings-model-picker__selected")?.textContent?.includes("不可重新选择") === false,
  "marketplace loading is not reported as a settled unavailable current value",
);
const loadingDefaultOptions = await pickerOptionTexts(container, 0);
ok(loadingDefaultOptions.some((text) => text.includes("shared")), "marketplace loading preserves BYOK options");
ok(!loadingDefaultOptions.some((text) => text.includes("pro-only")), "marketplace loading fails subscription options closed");
await act(async () => {
  container.querySelectorAll<HTMLButtonElement>(".settings-subtab")[1]?.click();
  await flushPromises();
});
const loadingOfficialCard = container.querySelector<HTMLElement>(".provider-access-card--builtin");
ok(
  loadingOfficialCard?.textContent?.includes("正在确认可用订阅模型") === true,
  "official access card reports subscription model loading",
);
ok(
  !loadingOfficialCard?.textContent?.includes("pro-only") && !loadingOfficialCard?.textContent?.includes("free-only"),
  "official access card does not expose the full catalog while plan access is loading",
);
await act(async () => {
  container.querySelectorAll<HTMLButtonElement>(".settings-subtab")[0]?.click();
  await flushPromises();
});

await act(async () => {
  renderSettings(root, proAccess, { state: "ready", routes: [], marketplace });
  await flushPromises();
});
const defaultOptions = await pickerOptionTexts(container, 0);
const plannerOptions = await pickerOptionTexts(container, 1);
const subagentOptions = await pickerOptionTexts(container, 2);
ok(
  container.querySelector(".settings-model-picker__selected")?.textContent?.includes("不可重新选择") === true,
  "an unauthorized current value uses the unavailable hint after marketplace settles",
);
ok(defaultOptions.some((text) => text.includes("pro-only")), "default picker includes the authorized subscription model");
ok(defaultOptions.filter((text) => text.includes("shared")).length === 1, "same-name filtering leaves exactly one BYOK option");
ok(defaultOptions.find((text) => text.includes("shared"))?.includes("custom") === true, "ModelPicker deduplication preserves the same-name BYOK provider");
ok(!defaultOptions.some((text) => text.includes("free-only")), "default picker excludes the unauthorized current subscription model from new options");
ok(plannerOptions.some((text) => text.includes("pro-only")) && !plannerOptions.some((text) => text.includes("shared")), "planner keeps its original subscription billing-source constraint after entitlement filtering");
ok(subagentOptions.some((text) => text.includes("pro-only")) && !subagentOptions.some((text) => text.includes("shared")), "subagent keeps its original subscription billing-source constraint after entitlement filtering");
await act(async () => {
  container.querySelectorAll<HTMLButtonElement>(".settings-subtab")[1]?.click();
  await flushPromises();
});
const settledOfficialCard = container.querySelector<HTMLElement>(".provider-access-card--builtin");
ok(
  settledOfficialCard?.textContent?.includes("当前套餐可用模型") === true,
  "official access card identifies its list as current-plan models",
);
ok(
  settledOfficialCard?.textContent?.includes("pro-only") === true
    && settledOfficialCard?.textContent?.includes("free-only") === false
    && settledOfficialCard?.textContent?.includes("shared") === false,
  "official access card only displays models authorized for the current plan",
);
await act(async () => {
  container.querySelectorAll<HTMLButtonElement>(".settings-subtab")[0]?.click();
  await flushPromises();
});
const settledDerivedTriggers = [...container.querySelectorAll<HTMLElement>("[data-model-selection-state]")];
ok(
  settledDerivedTriggers[0]?.dataset.modelSelectionState === "explicit_unavailable"
    && settledDerivedTriggers[1]?.dataset.modelSelectionState === "explicit_unavailable",
  "an explicit planner equal to the default remains explicit when both are unavailable",
);
ok(
  container.querySelectorAll(".settings-model-picker__selected")[1]?.textContent?.includes("free-only") === true,
  "an explicit planner equal to the default displays the actual model instead of inheritance",
);

await act(async () => renderSettings(root, { state: "ready", plan: "free", requiredTag: "Subscribe_Free" }, { state: "ready", routes: [], marketplace }));
const freeDerivedTriggers = [...container.querySelectorAll<HTMLElement>("[data-model-selection-state]")];
ok(
  freeDerivedTriggers[0]?.dataset.modelSelectionState === "explicit_available"
    && freeDerivedTriggers[1]?.dataset.modelSelectionState === "explicit_available",
  "an explicit planner equal to the default remains explicit when both are available",
);
ok(mutationCalls === 0, "derived model state transitions perform zero automatic writes");
await act(async () => renderSettings(root, proAccess, { state: "ready", routes: [], marketplace }));

await act(async () => renderSettings(root, { state: "loading" }, { state: "ready", routes: [], marketplace }));
ok(
  container.querySelector(".settings-model-picker__selected")?.textContent?.includes("正在确认可用订阅模型") === true,
  "access loading uses the loading hint after marketplace data is cached",
);
const accessLoadingOptions = await pickerOptionTexts(container, 0);
ok(accessLoadingOptions.some((text) => text.includes("shared")), "access loading keeps existing BYOK refs selectable");
ok(marketplaceCalls === 0, "access loading does not request marketplace directly");

await act(async () => renderSettings(root, { state: "unavailable" }, { state: "ready", routes: [], marketplace }));
ok(
  container.querySelector(".settings-model-picker__selected")?.textContent?.includes("不可重新选择") === true,
  "settled unavailable access uses the unavailable-current hint",
);
const unavailableOptions = await pickerOptionTexts(container, 0);
ok(!unavailableOptions.some((text) => text.includes("pro-only")) && unavailableOptions.some((text) => text.includes("shared")), "access changes recompute model options and preserve BYOK");
ok(marketplaceCalls === 0, "access changes do not request marketplace directly");
ok(mutationCalls === 0, "loading, marketplace resolution, and access changes perform zero settings writes");
await act(async () => root.unmount());

const emptyHost = document.createElement("div");
document.body.appendChild(emptyHost);
const emptyRoot = createRoot(emptyHost);
const emptySettings = settingsFixture();
emptySettings.defaultModel = "";
emptySettings.plannerModel = "";
emptySettings.subagentModel = "";
emptySettings.providers = [provider("kwu", "subscription_pool", ["pro-only"])];
let emptyScenarioMarketplaceCalls = 0;
let emptyScenarioMutations = 0;
window.go = {
  main: {
    App: {
      Settings: async () => emptySettings,
      ModelMarketplace: async () => {
        emptyScenarioMarketplaceCalls += 1;
        return new Promise<ModelMarketplaceView>(() => {});
      },
      SetDefaultModelRoute: async () => { emptyScenarioMutations += 1; },
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => renderSettings(emptyRoot, proAccess, { state: "loading" }));
await waitFor("empty loading model settings", () => emptyHost.querySelectorAll(".settings-model-picker__trigger").length === 3);
const emptySelectionText = emptyHost.querySelector(".settings-model-picker__selected")?.textContent ?? "";
ok(emptySelectionText.includes("正在确认可用订阅模型"), "loading with no current value or refs shows a minimal loading hint");
ok(!emptySelectionText.includes("未配置模型"), "loading with no current value or refs does not report missing provider configuration");
ok(
  emptyHost.querySelectorAll(".settings-model-picker__selected")[1]?.textContent?.includes("使用当前模型") === true,
  "only an empty raw planner value displays the single-model inheritance option",
);
ok(emptyScenarioMarketplaceCalls === 0, "empty loading scenario does not request marketplace directly");
ok(emptyScenarioMutations === 0, "empty loading scenario performs zero settings writes");
await act(async () => emptyRoot.unmount());
emptyHost.remove();

async function verifyUnavailableMarketplace(
  label: string,
  modelCatalog: SharedModelCatalogState,
) {
  const host = document.createElement("div");
  document.body.appendChild(host);
  const scenarioRoot = createRoot(host);
  let scenarioMutations = 0;
  let scenarioMarketplaceCalls = 0;
  window.go = {
    main: {
      App: {
        Settings: async () => settingsFixture(),
        ModelMarketplace: async () => {
          scenarioMarketplaceCalls += 1;
          throw new Error("SettingsPanel must not load Marketplace independently");
        },
        SetDefaultModelRoute: async () => { scenarioMutations += 1; },
      } as Partial<AppBindings> as AppBindings,
    },
  };
  await act(async () => renderSettings(scenarioRoot, proAccess, modelCatalog));
  await waitFor(label, () => host.querySelectorAll(".settings-model-picker__trigger").length === 3);
  await waitFor(`${label} settled`, () => host.querySelector(".settings-model-picker__selected")?.textContent?.includes("不可重新选择") === true);
  ok(host.querySelector(".settings-model-picker__selected")?.textContent?.includes("不可重新选择") === true, `${label} shows the settled unavailable-current hint`);
  const options = await pickerOptionTexts(host, 0);
  ok(options.some((text) => text.includes("shared")), `${label} preserves BYOK refs`);
  ok(!options.some((text) => text.includes("pro-only")), `${label} hides subscription refs`);
  ok(scenarioMarketplaceCalls === 0, `${label} does not request marketplace directly`);
  ok(scenarioMutations === 0, `${label} performs zero settings writes`);
  await act(async () => scenarioRoot.unmount());
  host.remove();
}

await verifyUnavailableMarketplace("shared catalog unavailable", { state: "unavailable" });
await verifyUnavailableMarketplace("missing international source", { state: "ready", routes: [], marketplace: { sources: [] } });

const realHost = document.createElement("div");
document.body.appendChild(realHost);
const realRoot = createRoot(realHost);
const realSettings = settingsFixture();
realSettings.defaultModel = "kwu/gpt-5.4";
realSettings.plannerModel = "subscription/free-route";
realSettings.subagentModel = "kwu/gpt-5.4";
realSettings.providers = [provider("custom", "newapi_byok", ["shared"])];
realSettings.bot = {
  ...realSettings.bot!,
  model: "kwu/gpt-5.4",
  connections: realSettings.bot!.connections.map((connection) => ({ ...connection, model: "kwu/gpt-5.4" })),
};
const realRouteModels: ModelInfo[] = [
  { ref: "kwu/gpt-5.4", provider: "kwu", model: "gpt-5.4", billingSource: "subscription_pool", current: true },
  { ref: "subscription/free-route", provider: "subscription", model: "free-route", billingSource: "subscription_pool", current: false },
  { ref: "subscription/shared", provider: "subscription", model: "shared", billingSource: "subscription_pool", current: false },
  { ref: "custom/shared", provider: "custom", model: "shared", billingSource: "newapi_byok", current: false },
  { ref: "collision/ambiguous", provider: "collision", model: "ambiguous", billingSource: "subscription_pool", current: false },
  { ref: "collision/ambiguous", provider: "collision", model: "ambiguous", billingSource: "newapi_byok", current: false },
];
const freeMarketplace: ModelMarketplaceView = {
  sources: [{
    id: "international",
    label: "International",
    sourceUrl: "https://pricing.example.invalid/api/pricing",
    pricingUrl: "https://pricing.example.invalid/models",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [
      { id: "gpt-5.4", name: "gpt-5.4", tags: ["Subscribe_Pro"] },
      { id: "free-route", name: "free-route", tags: ["Subscribe_Free"] },
      { id: "shared", name: "shared", tags: ["Subscribe_Free"] },
      { id: "ambiguous", name: "ambiguous", tags: ["Subscribe_Pro"] },
    ],
  }],
};
let realModelsCalls = 0;
let realMarketplaceCalls = 0;
const defaultRouteWrites: Array<[string, string]> = [];
const plannerWrites: string[] = [];
const subagentWrites: string[] = [];
const effortWrites: string[] = [];
let realOtherWrites = 0;
window.go = {
  main: {
    App: {
      Settings: async () => realSettings,
      ModelsForTab: async (tabID: string) => {
        if (tabID !== "tab-real") throw new Error(`unexpected tab ${tabID}`);
        realModelsCalls += 1;
        return realRouteModels;
      },
      ModelMarketplace: async () => {
        realMarketplaceCalls += 1;
        return freeMarketplace;
      },
      SetDefaultModelRoute: async (ref: string, billingSource: string) => { defaultRouteWrites.push([ref, billingSource]); },
      SetPlannerModel: async (ref: string) => { plannerWrites.push(ref); },
      SetSubagentModel: async (ref: string) => { subagentWrites.push(ref); },
      SetSubagentEffort: async (level: string) => { effortWrites.push(level); },
      SetBotSettings: async () => { realOtherWrites += 1; },
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => renderSettings(
  realRoot,
  { state: "ready", plan: "free", requiredTag: "Subscribe_Free" },
  { state: "ready", routes: realRouteModels, marketplace: freeMarketplace },
  "tab-real",
));
await waitFor("real route settings", () => realHost.querySelectorAll(".settings-model-picker__trigger").length === 3);
await waitFor("real route loading settled", () => realHost.textContent?.includes("不可重新选择") === true);
const realDefaultOptions = await pickerOptionTexts(realHost, 0);
const realPlannerOptions = await pickerOptionTexts(realHost, 1);
const effortControl = realHost.querySelector<HTMLElement>(".subagent-effort-control");
ok(realModelsCalls === 0, "settings reuses App's active-tab route catalog");
ok(realMarketplaceCalls === 0, "settings reuses App's marketplace snapshot");
ok(effortControl?.tagName === "DIV" && effortControl.getAttribute("role") === "group", "subagent effort uses the themed settings control instead of a native select");
ok(effortControl?.querySelectorAll("button").length === 6, "subagent effort exposes auto and every supported effort level");
ok(effortControl?.querySelector("select") === null, "subagent effort does not render a native select");
const mediumEffort = Array.from(effortControl?.querySelectorAll<HTMLButtonElement>("button") ?? []).find((button) => button.textContent === "medium");
await act(async () => {
  mediumEffort?.click();
  await flushPromises();
});
eq(effortWrites, ["medium"], "subagent effort forwards the exact selected level");
ok(realHost.querySelector(".settings-model-picker__selected")?.textContent?.includes("gpt-5.4") === true, "old Pro default remains visible");
ok(realHost.querySelector(".settings-model-picker__selected")?.textContent?.includes("不可重新选择") === true, "old Pro default is marked unavailable for new selection");
ok(realDefaultOptions.some((text) => text.includes("free-route")), "default settings exposes Free routes returned only by ModelsForTab");
ok(!realDefaultOptions.some((text) => text.includes("gpt-5.4")), "old Pro route is not offered as a new option");
ok(
  realDefaultOptions.some((text) => text.includes("shared") && text.includes("custom"))
    && realDefaultOptions.some((text) => text.includes("shared") && text.includes("subscription")),
  "same-name subscription and BYOK routes both remain selectable",
);
ok(!realDefaultOptions.some((text) => text.includes("ambiguous")), "same ref with conflicting billing sources fails closed");
ok(realPlannerOptions.some((text) => text.includes("free-route")) && !realPlannerOptions.some((text) => text.includes("custom")), "planner keeps the default subscription billing source constraint for route-only models");
ok(
  realHost.querySelectorAll(".settings-model-picker__selected")[1]?.textContent?.includes("free-route") === true,
  "an explicit planner different from the default displays its actual model",
);
ok(defaultRouteWrites.length === 0 && plannerWrites.length === 0 && subagentWrites.length === 0 && realOtherWrites === 0, "route and marketplace loading perform zero settings writes");

await pickModelOption(realHost, 0, "free-route");
await waitFor("default route write", () => defaultRouteWrites.length === 1);
eq(defaultRouteWrites, [["subscription/free-route", "subscription_pool"]], "default model selection forwards the exact route billing source");
ok(realOtherWrites === 0, "selecting a default route does not write planner, subagent, or bot settings");
await pickModelOption(realHost, 1, "使用当前模型");
await waitFor("planner route write", () => plannerWrites.length === 1);
await pickModelOption(realHost, 2, "free-route");
await waitFor("subagent route write", () => subagentWrites.length === 1);
eq(plannerWrites, [""], "choosing single-model inheritance persists an empty planner value");
eq(subagentWrites, ["subscription/free-route"], "subagent model selection forwards the exact route ref");
await act(async () => realRoot.unmount());
realHost.remove();

const routeOnlyHost = document.createElement("div");
document.body.appendChild(routeOnlyHost);
const routeOnlyRoot = createRoot(routeOnlyHost);
const routeOnlySettings = settingsFixture();
routeOnlySettings.defaultModel = "kwu/deepseek-v4-flash";
routeOnlySettings.defaultBillingSource = "subscription_pool";
routeOnlySettings.selectedModelSummary = {
  ref: "kwu/deepseek-v4-flash",
  provider: "kwu",
  model: "deepseek-v4-flash",
  displayName: "deepseek-v4-flash",
  billingSource: "subscription_pool",
  availability: "ready",
  accountState: "ready",
};
routeOnlySettings.providers = routeOnlySettings.providers.filter((item) => item.name !== "kwu");
window.go = {
  main: {
    App: {
      Settings: async () => routeOnlySettings,
      Models: async () => [{
        ref: "kwu/deepseek-v4-flash",
        provider: "kwu",
        model: "deepseek-v4-flash",
        billingSource: "subscription_pool",
        current: true,
      }],
      ModelMarketplace: async () => ({
        sources: [{
          id: "international",
          label: "International",
          sourceUrl: "https://pricing.example.invalid/api/pricing",
          pricingUrl: "https://pricing.example.invalid/models",
          pricingState: "fresh",
          metadataStatus: "ready",
          models: [{ id: "deepseek-v4-flash", name: "deepseek-v4-flash", tags: ["Subscribe_Free"] }],
        }],
      }),
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => renderSettings(
  routeOnlyRoot,
  { state: "ready", plan: "free", requiredTag: "Subscribe_Free" },
  {
    state: "ready",
    routes: [{
      ref: "kwu/deepseek-v4-flash",
      provider: "kwu",
      model: "deepseek-v4-flash",
      billingSource: "subscription_pool",
      current: true,
    }],
    marketplace: {
      sources: [{
        id: "international",
        label: "International",
        sourceUrl: "https://pricing.example.invalid/api/pricing",
        pricingUrl: "https://pricing.example.invalid/models",
        pricingState: "fresh",
        metadataStatus: "ready",
        models: [{ id: "deepseek-v4-flash", name: "deepseek-v4-flash", tags: ["Subscribe_Free"] }],
      }],
    },
  },
));
await waitFor("route-only subscription default", () => (
  routeOnlyHost.querySelector(".settings-model-picker__selected")?.textContent?.includes("deepseek-v4-flash") === true
  && routeOnlyHost.querySelector(".settings-model-picker__selected")?.textContent?.includes("Confirming") === false
));
ok(
  routeOnlyHost.querySelector(".provider-fetch-banner--warn") === null,
  "an authorized subscription route does not require a matching BYOK provider",
);
await act(async () => routeOnlyRoot.unmount());
routeOnlyHost.remove();

const anonymousHost = document.createElement("div");
document.body.appendChild(anonymousHost);
const anonymousRoot = createRoot(anonymousHost);
const anonymousSettings = settingsFixture();
anonymousSettings.providers = anonymousSettings.providers.filter((item) => item.name !== "kwu");
window.go = {
  main: {
    App: {
      Settings: async () => anonymousSettings,
      ModelMarketplace: async () => marketplace,
      Models: async () => [],
    } as Partial<AppBindings> as AppBindings,
  },
};
await act(async () => renderSettings(
  anonymousRoot,
  { state: "unavailable" },
  { state: "ready", routes: [], marketplace },
  undefined,
  "anonymous",
));
await waitFor("anonymous model settings", () => anonymousHost.querySelectorAll(".settings-model-picker__trigger").length === 3);
const anonymousPickerText = [...anonymousHost.querySelectorAll<HTMLElement>(".settings-model-picker__selected")]
  .map((node) => node.textContent ?? "")
  .join(" ");
ok(
  anonymousPickerText.includes("未登录") && !anonymousPickerText.includes("free-only"),
  "signed-out model pickers hide the saved subscription model without rewriting it",
);
const anonymousModelsNav = [...anonymousHost.querySelectorAll<HTMLElement>(".settings-center__navitem")]
  .find((node) => node.textContent?.includes("模型"));
ok(
  anonymousModelsNav?.textContent?.includes("未登录 · 暂无可用默认模型") === true
    && anonymousModelsNav?.textContent?.includes("free-only") === false,
  "signed-out settings navigation does not summarize the stale subscription default",
);
ok(
  anonymousHost.querySelector(".provider-fetch-banner--warn") === null,
  "signed-out model settings do not report the preserved subscription default as unavailable",
);
await act(async () => anonymousRoot.unmount());
anonymousHost.remove();

dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
