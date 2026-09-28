// Run: tsx src/__tests__/heartbeat-subscription-model-access.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import type { AutomationTask, AutomationTaskQuota } from "../features/automation/heartbeat/heartbeat.types";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";
import type { ExpertInfo, ModelInfo, ModelMarketplaceView, SharedModelCatalogState, SkillView } from "../lib/types";

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

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
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
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.CustomEvent = dom.window.CustomEvent;
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
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });
window.runtime = { EventsOn: () => () => {}, BrowserOpenURL: () => {} };

const proAccess: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
const oldTask: AutomationTask = {
  id: "task-1",
  name: "Existing automation",
  enabled: true,
  scope: "global",
  prompt: { prompt: "run", model: "subscription/free-only", billingSource: "subscription_pool" },
  schedule: { kind: "interval", every: "30m" },
  effectiveRange: {
    from: "0001-01-01T00:00:00Z",
    until: "0001-01-01T00:00:00Z",
  },
  execution: { approvalMode: "auto" },
  delivery: { enabled: true, targets: [{ type: "feishu", ref: "bot_target_v1_missing" }] },
  createdAt: "2026-07-22T00:00:00Z",
};

function model(ref: string, provider: string, billingSource: ModelInfo["billingSource"]): ModelInfo {
  return {
    ref,
    provider,
    model: ref.slice(ref.indexOf("/") + 1),
    billingSource,
    current: false,
  };
}

const globalModels: ModelInfo[] = [
  model("subscription/shared", "subscription", "subscription_pool"),
  model("byok/shared", "byok", "newapi_byok"),
  model("subscription/pro-only", "subscription", "subscription_pool"),
  model("subscription/free-only", "subscription", "subscription_pool"),
];
const projectModels: ModelInfo[] = [
  model("subscription/project-pro", "subscription", "subscription_pool"),
  model("byok/project-byok", "byok", "newapi_byok"),
];
const catalogSkills: SkillView[] = [
  {
    name: "research",
    displayName: "Research",
    description: "Research a topic.",
    scope: "global",
    runAs: "skill",
    enabled: true,
    iconUrl: "https://assets.example.com/skills/research.png",
  },
  {
    name: "review",
    description: "Review a change.",
    scope: "global",
    runAs: "skill",
    enabled: true,
  },
];
const catalogExperts: ExpertInfo[] = [
  {
    id: "research-expert",
    name: "Research Expert",
    description: "Investigates complex topics.",
    enabled: true,
    avatarUrl: "https://assets.example.com/experts/research.png",
  },
  {
    id: "plain-expert",
    name: "Plain Expert",
    description: "Has no configured avatar.",
    enabled: true,
  },
];
const marketplace: ModelMarketplaceView = {
  sources: [{
    id: "international",
    label: "International",
    sourceUrl: "https://pricing.example.invalid/api/pricing",
    pricingUrl: "https://pricing.example.invalid/models",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [
      { id: "shared", name: "shared", tags: ["Subscribe_Pro"] },
      { id: "pro-only", name: "pro-only", tags: ["Subscribe_Pro"] },
      { id: "free-only", name: "free-only", tags: ["Subscribe_Free"] },
      { id: "project-pro", name: "project-pro", tags: ["Subscribe_Pro"] },
    ],
  }],
};

function renderPanel(root: Root, access: SubscriptionAccess, modelCatalog: SharedModelCatalogState) {
  root.render(
    <LocaleProvider>
      <HeartbeatPanel
        open
        embedded
        authenticated
        modelCatalog={modelCatalog}
        subscriptionAccess={access}
        onClose={() => {}}
        onOpenTopic={() => {}}
      />
    </LocaleProvider>,
  );
}

async function openExistingEditor(host: HTMLElement) {
  await waitFor("automation task", () => host.querySelector(".heartbeat-card__title-btn") !== null);
  await act(async () => {
    host.querySelector<HTMLButtonElement>(".heartbeat-card__title-btn")?.click();
    await flushPromises();
  });
  await waitFor("automation model editor", () => host.querySelectorAll(".heartbeat-editor-select__trigger").length >= 3);
}

async function modelOptionTexts(host: HTMLElement): Promise<string[]> {
  const field = [...host.querySelectorAll<HTMLElement>(".heartbeat-editor__field")]
    .find((candidate) => candidate.querySelector("label")?.textContent?.startsWith("模型"));
  const trigger = field?.querySelector<HTMLButtonElement>(".heartbeat-editor-select__trigger");
  if (!trigger) throw new Error("missing automation model picker");
  await act(async () => {
    trigger.click();
    await flushPromises();
  });
  const lists = document.querySelectorAll(".heartbeat-editor-select__list");
  const list = lists[lists.length - 1];
  const texts = [...(list?.querySelectorAll<HTMLElement>(".heartbeat-editor-select__option") ?? [])]
    .map((option) => option.textContent ?? "");
  await act(async () => {
    trigger.click();
    await flushPromises();
  });
  return texts;
}

function installBindings(
  loadMarketplace: () => Promise<ModelMarketplaceView>,
  quota: AutomationTaskQuota = {
    state: "ready",
    plan: "pro",
    used: 1,
    limit: 0,
    unlimited: true,
    canCreate: true,
    pausedTaskIds: [],
  },
  tasks: AutomationTask[] = [oldTask],
) {
  const roots: string[] = [];
  let marketplaceCalls = 0;
  let saveCalls = 0;
  window.go = {
    main: {
      App: {
        AutomationReloadTasks: async () => tasks,
        AutomationTaskQuota: async () => quota,
        AutomationListRuns: async () => [],
        AutomationListTemplates: async () => [],
        AutomationGenerateID: async () => "new-task-id",
        AutomationValidateDraft: async () => ({ task: {}, nextRuns: [] }),
        AutomationListCapabilities: async () => ({ models: [], skills: [], experts: [], connectors: [] }),
        AutomationListDeliveryTargets: async () => [
          { type: "feishu", ref: "bot_target_v1_available", label: "运营通知", provider: "飞书", available: true, scope: "profile" },
          { type: "weixin_ilink", ref: "bot_target_v1_disabled", label: "值班通知", provider: "微信", available: false, unavailableReason: "连接已停用", scope: "workspace" },
        ],
        AutomationModels: async (root: string) => {
          roots.push(root);
          return root === "/project" ? projectModels : globalModels;
        },
        AutomationSaveTasks: async () => { saveCalls += 1; },
        ListWorkspaces: async () => [{ name: "Project One", path: "/project", current: true }],
        SkillsSettings: async () => ({ skills: catalogSkills, skillRoots: [] }),
        ListExperts: async () => catalogExperts,
        ModelMarketplace: async () => {
          marketplaceCalls += 1;
          return loadMarketplace();
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };
  return {
    roots,
    marketplaceCalls: () => marketplaceCalls,
    saveCalls: () => saveCalls,
  };
}

console.log("\nheartbeat subscription model access");

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
const heartbeatCalls = appSource.split("<HeartbeatPanel").slice(1);
ok(
  heartbeatCalls.length === 2 && heartbeatCalls.every((call) => call.slice(0, 350).includes("subscriptionAccess={subscriptionAccess}")),
  "both production HeartbeatPanel entry points receive the shared subscription access",
);
ok(
  heartbeatCalls.length === 2 && heartbeatCalls.every((call) => call.slice(0, 350).includes("authenticated={sidebarAccountHasSession}")),
  "both production HeartbeatPanel entry points reuse the shared authenticated account state",
);
ok(
  heartbeatCalls.length === 2 && heartbeatCalls.every((call) => call.slice(0, 350).includes("modelCatalog={sharedModelCatalog}")),
  "both production HeartbeatPanel entry points receive the shared model catalog",
);

const main = installBindings(async () => {
  throw new Error("HeartbeatPanel must not load Marketplace independently");
});
const host = document.getElementById("root")!;
const root = createRoot(host);
await act(async () => renderPanel(root, proAccess, { state: "loading" }));
await waitFor("automation quota", () => host.textContent?.includes("1/无限") === true);
ok(host.textContent?.includes("1/无限") === true, "automation list shows current local task usage and unlimited Pro quota");
await openExistingEditor(host);
ok(
  host.querySelector(".heartbeat-editor--full-page") === null &&
    host.querySelector(".heartbeat-modal__header")?.textContent?.includes("编辑任务") === true,
  "existing automation reuses the sidebar-compatible add-task editor layout",
);
const effectiveRangeTrigger = host.querySelector<HTMLButtonElement>(".heartbeat-editor__date-picker-trigger");
ok(effectiveRangeTrigger?.textContent?.trim() === "长期有效", "an empty effective range is described as long-term");
ok(
  effectiveRangeTrigger?.querySelector(".lucide-chevrons-up-down")?.getAttribute("aria-hidden") === "true",
  "effective range exposes the same decorative expand indicator as existing editor selects",
);
await act(async () => {
  effectiveRangeTrigger?.click();
  await flushPromises();
});
const effectiveRangeInputs = document.querySelectorAll<HTMLInputElement>(".heartbeat-editor__effective-range input");
ok(
  effectiveRangeInputs.length === 2 && [...effectiveRangeInputs].every((input) => input.value === ""),
  "backend zero times keep both effective-range inputs empty",
);
await act(async () => {
  effectiveRangeInputs[0]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
});
ok(effectiveRangeTrigger?.getAttribute("aria-expanded") === "true", "clicking inside the effective-range picker keeps it open");
await act(async () => {
  document.body.dispatchEvent(new MouseEvent("click", { bubbles: true }));
});
ok(effectiveRangeTrigger?.getAttribute("aria-expanded") === "false", "clicking outside the effective-range picker closes it");
await act(async () => {
  effectiveRangeTrigger?.click();
  await flushPromises();
});
await act(async () => {
  effectiveRangeInputs[0]?.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
});
ok(effectiveRangeTrigger?.getAttribute("aria-expanded") === "false", "Escape closes the effective-range picker");
const deliveryField = [...host.querySelectorAll<HTMLElement>(".heartbeat-editor__field")]
  .find((field) => field.querySelector(":scope > label")?.textContent?.startsWith("结果通知"));
const deliveryTrigger = deliveryField?.querySelector<HTMLButtonElement>(".heartbeat-editor-multiselect__trigger");
await act(async () => {
  deliveryTrigger?.click();
  await flushPromises();
});
await waitFor("delivery target catalog", () => document.body.textContent?.includes("运营通知") === true);
const deliveryOptions = [...document.querySelectorAll<HTMLLabelElement>('[data-state="open"] .heartbeat-editor-multiselect__option')];
const availableDeliveryTarget = deliveryOptions.find((option) => option.textContent?.includes("运营通知"));
const disabledDeliveryTarget = deliveryOptions.find((option) => option.textContent?.includes("值班通知"));
const missingDeliveryTarget = deliveryOptions.find((option) => option.textContent?.includes("bot_target_v1_missing"));
const availableDeliveryCheckbox = availableDeliveryTarget?.querySelector<HTMLInputElement>('input[type="checkbox"]');
ok(availableDeliveryCheckbox?.disabled === false, "available backend delivery targets are selectable");
ok(disabledDeliveryTarget?.querySelector<HTMLInputElement>('input[type="checkbox"]')?.disabled === true && disabledDeliveryTarget.textContent?.includes("连接已停用") === true, "unavailable delivery targets expose the safe reason and cannot be selected");
ok(missingDeliveryTarget?.querySelector<HTMLInputElement>('input[type="checkbox"]')?.disabled === false && missingDeliveryTarget.textContent?.includes("授权已撤销") === true, "a saved missing opaque target remains visible and removable");
await act(async () => {
  availableDeliveryCheckbox?.click();
  await flushPromises();
});
ok(availableDeliveryCheckbox?.checked === true, "delivery selection stores the catalog target without a task write");
await waitFor("global automation models", () => main.roots.includes(""));
ok(
  host.querySelector<HTMLButtonElement>(".heartbeat-editor__actions .heartbeat-btn--primary")?.textContent?.trim() === "保存",
  "existing automation editor renders the save action label",
);
const promptPickers = host.querySelectorAll<HTMLElement>(".heartbeat-editor__prompt-tools .heartbeat-editor-select");
const skillTrigger = promptPickers[0]?.querySelector<HTMLButtonElement>(".heartbeat-editor-select__trigger");
const expertTrigger = promptPickers[1]?.querySelector<HTMLButtonElement>(".heartbeat-editor-select__trigger");
await act(async () => {
  skillTrigger?.click();
  await flushPromises();
});
await waitFor("automation skill catalog", () =>
  [...document.querySelectorAll<HTMLElement>(".heartbeat-editor-select__list")]
    .some((list) => list.textContent?.includes("research")),
);
const skillList = [...document.querySelectorAll<HTMLElement>(".heartbeat-editor-select__list")]
  .find((list) => list.textContent?.includes("research"));
const skillOption = [...(skillList?.querySelectorAll<HTMLButtonElement>(".heartbeat-editor-select__option") ?? [])]
  .find((option) => option.textContent?.includes("research"));
const skillFallbackOption = [...(skillList?.querySelectorAll<HTMLButtonElement>(".heartbeat-editor-select__option") ?? [])]
  .find((option) => option.textContent?.includes("review"));
ok(
  skillOption?.querySelector<HTMLImageElement>(".heartbeat-editor-select__visual--skill img")?.getAttribute("src") === catalogSkills[0].iconUrl,
  "automation skill menu renders the catalog icon",
);
ok(
  skillOption?.querySelector(".heartbeat-editor-select__visual-fallback")?.textContent === "R" &&
    skillFallbackOption?.querySelector(".heartbeat-editor-select__visual-fallback")?.textContent === "R" &&
    skillFallbackOption?.querySelector("img") === null,
  "automation skill menu keeps initial fallbacks for missing or failed icons",
);
await act(async () => {
  skillOption?.click();
  await flushPromises();
});
const selectedSkillImage = skillTrigger?.querySelector<HTMLImageElement>(".heartbeat-editor-select__visual--skill img");
ok(selectedSkillImage?.getAttribute("src") === catalogSkills[0].iconUrl, "selected automation skill keeps its icon in the trigger");
await act(async () => {
  selectedSkillImage?.dispatchEvent(new dom.window.Event("error", { bubbles: true }));
  await flushPromises();
});
ok(
  skillTrigger?.querySelector(".heartbeat-editor-select__visual--skill img") === null &&
    skillTrigger?.querySelector(".heartbeat-editor-select__visual-fallback")?.textContent === "R",
  "failed selected skill images reveal the stable fallback",
);

await act(async () => {
  expertTrigger?.click();
  await flushPromises();
});
const expertList = [...document.querySelectorAll<HTMLElement>(".heartbeat-editor-select__list")]
  .find((list) => list.textContent?.includes("Research Expert"));
const expertOption = [...(expertList?.querySelectorAll<HTMLButtonElement>(".heartbeat-editor-select__option") ?? [])]
  .find((option) => option.textContent?.includes("Research Expert"));
const expertFallbackOption = [...(expertList?.querySelectorAll<HTMLButtonElement>(".heartbeat-editor-select__option") ?? [])]
  .find((option) => option.textContent?.includes("Plain Expert"));
ok(
  expertOption?.querySelector<HTMLImageElement>(".heartbeat-editor-select__visual--expert img")?.getAttribute("src") === catalogExperts[0].avatarUrl,
  "automation expert menu renders the resolved avatar",
);
ok(
  expertFallbackOption?.querySelector(".heartbeat-editor-select__visual-fallback")?.textContent === "P" &&
    expertFallbackOption?.querySelector("img") === null,
  "automation expert menu renders a circular fallback when no avatar resolves",
);
await act(async () => {
  expertOption?.click();
  await flushPromises();
});
ok(
  expertTrigger?.querySelector<HTMLImageElement>(".heartbeat-editor-select__visual--expert img")?.getAttribute("src") === catalogExperts[0].avatarUrl,
  "selected automation expert keeps its avatar in the trigger",
);
const modelTrigger = [...host.querySelectorAll<HTMLButtonElement>(".heartbeat-editor-select__trigger")]
  .find((trigger) => trigger.textContent?.includes("subscription/free-only"));
ok(modelTrigger?.querySelector(".heartbeat-editor-select__visual") === null, "automation model picker remains text-only");
ok(main.marketplaceCalls() === 0, "task editor does not request marketplace directly");
ok(host.textContent?.includes("正在确认可用订阅模型") === true, "pending marketplace shows the minimal confirmation hint");
ok(host.textContent?.includes("不可重新选择") === false, "pending marketplace is not reported as settled unavailable");
const pendingOptions = await modelOptionTexts(host);
ok(pendingOptions.some((text) => text.includes("shared · byok")), "pending marketplace preserves selectable BYOK models");
ok(!pendingOptions.some((text) => text.includes("pro-only")), "pending marketplace hides new subscription options");
ok(pendingOptions.some((text) => text.includes("subscription/free-only")), "pending marketplace preserves the old draft model");

await act(async () => {
  renderPanel(root, proAccess, { state: "ready", routes: [], marketplace });
  await flushPromises();
});
const readyOptions = await modelOptionTexts(host);
ok(readyOptions.some((text) => text.includes("pro-only")), "ready Pro access exposes exact-tag subscription models");
ok(readyOptions.filter((text) => text.includes("shared")).length === 2, "same-name subscription and BYOK automation routes remain distinct");
ok(host.textContent?.includes("不可重新选择") === true, "unauthorized old draft is marked unavailable after marketplace settles");

await act(async () => renderPanel(root, { state: "loading" }, { state: "ready", routes: [], marketplace }));
ok(host.textContent?.includes("正在确认可用订阅模型") === true, "access loading recomputes the editor hint without another request");
const accessLoadingOptions = await modelOptionTexts(host);
ok(accessLoadingOptions.some((text) => text.includes("shared · byok")), "access loading keeps BYOK selectable");
ok(main.marketplaceCalls() === 0, "access changes do not request marketplace directly");

await act(async () => renderPanel(root, { state: "unavailable" }, { state: "ready", routes: [], marketplace }));
const unavailableOptions = await modelOptionTexts(host);
ok(!unavailableOptions.some((text) => text.includes("pro-only")) && unavailableOptions.some((text) => text.includes("shared · byok")), "settled unavailable access hides subscription choices and preserves BYOK");
ok(host.textContent?.includes("不可重新选择") === true, "settled unavailable access marks the old subscription draft unavailable");

const workspaceField = [...host.querySelectorAll<HTMLElement>(".heartbeat-editor__field")]
  .find((field) => field.querySelector(":scope > label")?.textContent?.startsWith("工作空间"));
const projectButton = workspaceField?.querySelector<HTMLButtonElement>(".heartbeat-scope-btn");
await act(async () => {
  projectButton?.click();
  await flushPromises();
});
const projectOption = [...host.querySelectorAll<HTMLButtonElement>(".heartbeat-project-menu__item")]
  .find((button) => button.textContent?.includes("Project One"));
await act(async () => {
  projectOption?.click();
  await flushPromises();
});
await waitFor("project automation models", () => main.roots.includes("/project"));
const projectOptions = await modelOptionTexts(host);
ok(projectOptions.some((text) => text.includes("project-byok")) && !projectOptions.some((text) => text.includes("project-pro")), "project scope reloads AutomationModels and applies current access");
ok(main.marketplaceCalls() === 0, "scope changes do not request marketplace directly");
ok(main.saveCalls() === 0, "marketplace, access, and scope changes perform zero task writes");
await act(async () => root.unmount());

async function verifySettledMarketplace(label: string, modelCatalog: SharedModelCatalogState) {
  const scenarioHost = document.createElement("div");
  document.body.appendChild(scenarioHost);
  const scenarioRoot = createRoot(scenarioHost);
  const scenario = installBindings(async () => {
    throw new Error("HeartbeatPanel must not load Marketplace independently");
  });
  await act(async () => renderPanel(scenarioRoot, proAccess, modelCatalog));
  await openExistingEditor(scenarioHost);
  await waitFor(`${label} settled`, () => scenarioHost.textContent?.includes("不可重新选择") === true);
  const options = await modelOptionTexts(scenarioHost);
  ok(options.some((text) => text.includes("shared · byok")), `${label} preserves BYOK models`);
  ok(!options.some((text) => text.includes("pro-only")), `${label} hides subscription models`);
  ok(scenario.marketplaceCalls() === 0, `${label} does not request marketplace directly`);
  ok(scenario.saveCalls() === 0, `${label} performs zero task writes`);
  await act(async () => scenarioRoot.unmount());
  scenarioHost.remove();
}

await verifySettledMarketplace("shared catalog unavailable", { state: "unavailable" });
await verifySettledMarketplace("missing international source", { state: "ready", routes: [], marketplace: { sources: [] } });

const quotaHost = document.createElement("div");
document.body.appendChild(quotaHost);
const quotaRoot = createRoot(quotaHost);
installBindings(async () => marketplace, {
  state: "ready",
  plan: "free",
  used: 1,
  limit: 1,
  unlimited: false,
  canCreate: false,
  pausedTaskIds: [],
});
await act(async () => renderPanel(quotaRoot, proAccess, { state: "ready", routes: [], marketplace }));
await waitFor("full automation quota", () => quotaHost.textContent?.includes("1/1") === true);
ok(
  quotaHost.querySelector<HTMLElement>("[data-automation-quota-notice]")?.textContent?.includes("已达到当前套餐自动化任务上限（1/1）") === true,
  "full local task quota shows a visible limit message",
);
ok(
  quotaHost.querySelector<HTMLButtonElement>(".heartbeat-page-heading__add")?.disabled === true,
  "full local task quota disables the add-task entry",
);
await act(async () => quotaRoot.unmount());
quotaHost.remove();

const newTaskHost = document.createElement("div");
document.body.appendChild(newTaskHost);
const newTaskRoot = createRoot(newTaskHost);
installBindings(async () => marketplace, undefined, []);
await act(async () => renderPanel(newTaskRoot, proAccess, { state: "ready", routes: [], marketplace }));
await waitFor("new automation action", () => newTaskHost.querySelector(".heartbeat-page-heading__add") !== null);
await act(async () => {
  newTaskHost.querySelector<HTMLButtonElement>(".heartbeat-page-heading__add")?.click();
  await flushPromises();
});
await waitFor("new automation editor", () => newTaskHost.querySelector(".heartbeat-editor__actions") !== null);
const addAction = newTaskHost.querySelector<HTMLButtonElement>(".heartbeat-editor__actions .heartbeat-btn--primary");
ok(addAction?.textContent?.trim() === "添加", "new automation editor renders the add action label");
ok(addAction?.disabled === true, "incomplete new automation keeps its labeled add action disabled");
await act(async () => newTaskRoot.unmount());
newTaskHost.remove();

dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
