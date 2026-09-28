import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { SettingsPanel } from "../components/SettingsPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SettingsCoreView, SettingsModelDetailsView } from "../lib/types";

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
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

const core: SettingsCoreView = {
  defaultModel: "kwu/gpt-5.4",
  defaultBillingSource: "subscription_pool",
  selectedModelSummary: {
    ref: "kwu/gpt-5.4",
    provider: "kwu",
    model: "gpt-5.4",
    displayName: "gpt-5.4",
    billingSource: "subscription_pool",
    availability: "ready",
    accountState: "ready",
  },
  plannerModel: "",
  subagentModel: "",
  subagentEffort: "",
  autoPlan: "off",
  permissions: { mode: "ask", allow: [], ask: [], deny: [] },
  sandbox: { bash: "enforce", network: false, workspaceRoot: "", allowWrite: [], shell: "auto" },
  agent: { temperature: 0, maxSteps: 0, plannerMaxSteps: 0, systemPrompt: "", coldResumePrune: true, reasoningLanguage: "auto" },
  bot: {
    enabled: false,
    model: "",
    toolApprovalMode: "ask",
    maxSteps: 0,
    debounceMs: 0,
    allowlist: { enabled: false, allowAll: false, qqUsers: [], feishuUsers: [], weixinUsers: [], qqGroups: [], feishuGroups: [], weixinGroups: [] },
    qq: { enabled: false, appId: "", appSecretEnv: "", secretSet: false, sandbox: false },
    feishu: { enabled: false, domain: "feishu", appId: "", appSecretEnv: "", secretSet: false, verificationToken: "", mode: "webhook", webhookPort: 0, requireMention: false },
    weixin: { enabled: false, accountId: "", tokenEnv: "", tokenSet: false, apiBase: "" },
    connections: [],
  },
  desktopLanguage: "zh",
  desktopLayoutStyle: "creation",
  desktopTheme: "auto",
  desktopThemeStyle: "graphite",
  closeBehavior: "background",
  displayMode: "standard",
  statusBarStyle: "text",
  statusBarItems: [],
  defaultToolApprovalMode: "ask",
  checkUpdates: true,
  telemetry: false,
  metrics: false,
  configPath: "config.toml",
  autoApproveTools: false,
  bypass: false,
};

const officialProvider: SettingsModelDetailsView["providers"][number] = {
  name: "kwu",
  builtIn: true,
  added: true,
  kind: "openai",
  baseUrl: "",
  models: ["gpt-5.4"],
  chatModels: ["gpt-5.4"],
  visionModels: [],
  visionModelsConfigured: false,
  modelsUrl: "",
  default: "gpt-5.4",
  apiKeyEnv: "",
  keySet: true,
  requiresKey: false,
  configured: true,
  keySource: "",
  keySourcePath: "",
  balanceUrl: "",
  contextWindow: 1_000_000,
  reasoningProtocol: "openai",
  supportedEfforts: ["low", "medium", "high"],
  defaultEffort: "",
  billingSource: "subscription_pool",
  locked: true,
  availability: "ready",
};

let resolveDetails: ((value: SettingsModelDetailsView) => void) | undefined;
const detailsPromise = new Promise<SettingsModelDetailsView>((resolve) => {
  resolveDetails = resolve;
});
let initialCoreCalls = 0;
let detailsCalls = 0;
let providerModelFetchCalls = 0;
window.go = {
  main: {
    App: {
      SettingsCore: async () => {
        initialCoreCalls += 1;
        return core;
      },
      SettingsModelDetails: async () => {
        detailsCalls += 1;
        return detailsPromise;
      },
      Models: async () => [{
        ref: "kwu/gpt-5.4",
        provider: "kwu",
        model: "gpt-5.4",
        billingSource: "subscription_pool",
        current: true,
      }],
      ModelMarketplace: async () => ({ sources: [] }),
      FetchProviderModels: async () => {
        providerModelFetchCalls += 1;
        return ["gpt-5.4"];
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootElement = document.getElementById("root");
assert.ok(rootElement);
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <SettingsPanel initialTab="models" onClose={() => {}} onChanged={() => {}} />
    </LocaleProvider>,
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
});

console.log("settings shell and selected model render before details");
assert.ok(document.querySelector(".settings-center"), "settings shell should render from SettingsCore");
const initialNavText = document.querySelector(".settings-center__nav")?.textContent ?? "";
assert.match(initialNavText, /云智企办 · gpt-5\.4/);
assert.doesNotMatch(initialNavText, /\bkwu\b/);
assert.equal(detailsCalls, 1);
assert.equal(document.querySelectorAll(".settings-model-details-error").length, 0);
assert.equal(
  document.querySelector(".provider-fetch-banner--warn"),
  null,
  "pending provider details must not report the selected model as unavailable",
);
const pendingDefaultModel = document.querySelector(".settings-page--models .settings-model-picker__trigger")?.textContent ?? "";
assert.match(pendingDefaultModel, /正在确认可用订阅模型/);
assert.doesNotMatch(pendingDefaultModel, /\bkwu\b|未配置密钥/);

await act(async () => {
  resolveDetails?.({ providers: [officialProvider], officialProviders: [officialProvider], providerKinds: ["openai"] });
  await detailsPromise;
  await new Promise((resolve) => setTimeout(resolve, 0));
});
assert.equal(initialCoreCalls, 1, "opening the models page must not reload core settings in the background");
assert.equal(detailsCalls, 1, "opening the models page must not reload provider details in the background");
assert.equal(providerModelFetchCalls, 0, "opening the models page must not probe remote provider catalogs");

const settingsNavItems = [...document.querySelectorAll<HTMLButtonElement>(".settings-center__navitem")];
const generalNav = settingsNavItems.find((item) => /General|通用/.test(item.textContent ?? ""));
const modelsNav = settingsNavItems.find((item) => /Models|模型/.test(item.textContent ?? ""));
assert.ok(generalNav && modelsNav, "settings navigation should include general and models");
await act(async () => {
  generalNav.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
assert.equal(generalNav.getAttribute("data-selected"), "true", "clicked settings destination should retain its selected background state");
assert.equal(modelsNav.hasAttribute("data-selected"), false, "previous settings destination should release its selected background state");
await act(async () => {
  modelsNav.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
assert.equal(modelsNav.getAttribute("data-selected"), "true", "selected background state should move to the next clicked destination");
assert.equal(generalNav.hasAttribute("data-selected"), false, "selected background state should leave the prior destination");
assert.equal(initialCoreCalls, 1, "returning to the models page must reuse the settings snapshot");
assert.equal(detailsCalls, 1, "returning to the models page must reuse provider details");
assert.equal(providerModelFetchCalls, 0, "returning to the models page must not probe remote provider catalogs");
await act(async () => {
  root.unmount();
});

console.log("details failure is local, preserves summary, and retries");
const failureRootElement = document.createElement("div");
document.body.appendChild(failureRootElement);
const failureRoot = createRoot(failureRootElement);
let failureDetailsCalls = 0;
window.go = {
  main: {
    App: {
      SettingsCore: async () => core,
      SettingsModelDetails: async () => {
        failureDetailsCalls += 1;
        if (failureDetailsCalls === 1) throw new Error("details unavailable");
        return { providers: [], officialProviders: [], providerKinds: [] };
      },
      Models: async () => [],
      ModelMarketplace: async () => ({ sources: [] }),
    } as Partial<AppBindings> as AppBindings,
  },
};
const failureCore: SettingsCoreView = {
  ...core,
  bot: {
    ...core.bot,
    connections: [{
      id: "connection-a",
      provider: "feishu",
      domain: "feishu",
      label: "Test connection",
      enabled: true,
      status: "connected",
      model: "",
      toolApprovalMode: "ask",
      workspaceRoot: "",
      credential: { appId: "app-a", appSecretEnv: "TEST_SECRET", accountId: "", tokenEnv: "", secretSet: true },
      sessionMappings: [],
      lastError: "",
      createdAt: "",
      updatedAt: "",
    }],
  },
};
window.go!.main!.App!.SettingsCore = async () => failureCore;
await act(async () => {
  failureRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="models"
        activeTabId="failure-tab"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
});
for (let attempt = 0; attempt < 20 && !failureRootElement.querySelector(".settings-model-details-error"); attempt += 1) {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}
assert.ok(failureRootElement.querySelector(".settings-model-details-error"), "details failure should stay in the model area");
assert.match(failureRootElement.querySelector(".settings-center__nav")?.textContent ?? "", /gpt-5\.4/);
const modelPicker = failureRootElement.querySelector<HTMLButtonElement>(".settings-page--models .settings-model-picker__trigger");
assert.ok(modelPicker, "model picker should render");
assert.equal(modelPicker.disabled, true, "model picker must stay disabled until details are verified");
const retry = failureRootElement.querySelector<HTMLButtonElement>(".settings-model-details-error button");
assert.ok(retry);
await act(async () => {
  retry.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
assert.equal(failureDetailsCalls, 2);
assert.equal(failureRootElement.querySelector(".settings-model-details-error"), null);
await act(async () => {
  failureRoot.unmount();
});

console.log("late core response from an old tab cannot replace the active scope");
const staleRootElement = document.createElement("div");
document.body.appendChild(staleRootElement);
const staleRoot = createRoot(staleRootElement);
let resolveOldCore: ((value: SettingsCoreView) => void) | undefined;
let resolveNewCore: ((value: SettingsCoreView) => void) | undefined;
const oldCorePromise = new Promise<SettingsCoreView>((resolve) => {
  resolveOldCore = resolve;
});
const newCorePromise = new Promise<SettingsCoreView>((resolve) => {
  resolveNewCore = resolve;
});
let coreCalls = 0;
window.go = {
  main: {
    App: {
      SettingsCore: async () => {
        coreCalls += 1;
        return coreCalls === 1 ? oldCorePromise : newCorePromise;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};
const renderForTab = (activeTabId: string) => (
  <LocaleProvider>
    <SettingsPanel initialTab="general" activeTabId={activeTabId} onClose={() => {}} onChanged={() => {}} />
  </LocaleProvider>
);
await act(async () => {
  staleRoot.render(renderForTab("tab-old"));
  await new Promise((resolve) => setTimeout(resolve, 0));
});
await act(async () => {
  staleRoot.render(renderForTab("tab-new"));
  await new Promise((resolve) => setTimeout(resolve, 0));
});
const newCore: SettingsCoreView = {
  ...core,
  defaultModel: "kwu/new-model",
  selectedModelSummary: { ...core.selectedModelSummary, ref: "kwu/new-model", model: "new-model", displayName: "new-model" },
};
await act(async () => {
  resolveNewCore?.(newCore);
  await newCorePromise;
});
assert.match(staleRootElement.querySelector(".settings-center__nav")?.textContent ?? "", /new-model/);
await act(async () => {
  resolveOldCore?.(core);
  await oldCorePromise;
});
assert.doesNotMatch(staleRootElement.querySelector(".settings-center__nav")?.textContent ?? "", /gpt-5\.4/);
await act(async () => {
  staleRoot.unmount();
});
