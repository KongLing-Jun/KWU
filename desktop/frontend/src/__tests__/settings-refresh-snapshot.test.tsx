// Run: tsx src/__tests__/settings-refresh-snapshot.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { SettingsPanel } from "../components/SettingsPanel";
import { ModelPicker } from "../features/settings/ui/ModelPicker";
import { modelRoutesForSettings } from "../features/settings/ui/ModelsSettingsSection";
import { LocaleProvider, useI18n, type LangPref } from "../lib/i18n";
import type { AppBindings } from "../lib/bridge";
import type { SettingsView } from "../lib/types";
import {
  loadSettingsDetails,
  peekSettingsCore,
  peekSettingsDetails,
} from "../features/settings/data/settingsSnapshotCache";

let passed = 0;
let failed = 0;
const deprecatedKWUOfficialLabel = "Clouvia " + "Official";

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
  if (actual === expected) {
    ok(true, label);
  } else {
    ok(false, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
}

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function setInputValue(input: HTMLInputElement, value: string) {
  const win = input.ownerDocument.defaultView;
  const valueSetter = win ? Object.getOwnPropertyDescriptor(win.HTMLInputElement.prototype, "value")?.set : undefined;
  if (valueSetter) valueSetter.call(input, value);
  else input.value = value;
  // This test file has no user-event helper; call React's tracked handler so controlled inputs update in jsdom.
  const reactPropsKey = Object.keys(input).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (input as unknown as Record<string, { onChange?: (event: { target: HTMLInputElement; currentTarget: HTMLInputElement }) => void }>)[reactPropsKey]
    : undefined;
  reactProps?.onChange?.({ target: input, currentTarget: input });
  const EventCtor = win?.Event ?? Event;
  const InputEventCtor = win?.InputEvent;
  input.dispatchEvent(InputEventCtor
    ? new InputEventCtor("input", { bubbles: true, data: value, inputType: "insertText" })
    : new EventCtor("input", { bubbles: true }));
  input.dispatchEvent(new EventCtor("change", { bubbles: true }));
}

function LocaleHarness({ pref, children }: { pref: LangPref; children: React.ReactNode }) {
  const { setPref } = useI18n();
  React.useEffect(() => {
    setPref(pref);
  }, [pref, setPref]);
  return <>{children}</>;
}

function baseSettings(displayMode: "standard" | "compact" = "standard"): SettingsView {
  return {
    defaultModel: "",
    plannerModel: "",
    subagentModel: "",
    subagentEffort: "",
    autoPlan: "off",
    providers: [],
    officialProviders: [],
    permissions: { mode: "ask", allow: [], ask: [], deny: [] },
    sandbox: { bash: "enforce", network: false, workspaceRoot: "", allowWrite: [], shell: "auto" },
    agent: { temperature: 0, maxSteps: 0, plannerMaxSteps: 0, systemPrompt: "", coldResumePrune: true, reasoningLanguage: "auto" },
    bot: {
      enabled: false,
      model: "",
      toolApprovalMode: "",
      maxSteps: 0,
      debounceMs: 0,
      allowlist: { enabled: false, allowAll: false, qqUsers: [], feishuUsers: [], weixinUsers: [], qqGroups: [], feishuGroups: [], weixinGroups: [] },
      qq: { enabled: false, appId: "", appSecretEnv: "", secretSet: false, sandbox: false },
      feishu: { enabled: false, domain: "feishu", appId: "", appSecretEnv: "", secretSet: false, verificationToken: "", mode: "webhook", webhookPort: 0, requireMention: false },
      weixin: { enabled: false, accountId: "", tokenEnv: "", tokenSet: false, apiBase: "" },
      connections: [],
    },
    desktopLanguage: "en",
    desktopLayoutStyle: "workbench",
    desktopTheme: "auto",
    desktopThemeStyle: "graphite",
    closeBehavior: "background",
    displayMode,
    statusBarStyle: "text",
    statusBarItems: ["model", "workspace", "git_branch", "cache", "balance"],
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

function kwuProvider(): SettingsView["providers"][number] {
  return {
    name: "kwu",
    builtIn: true,
    added: true,
    billingSource: "subscription_pool",
    kind: "openai",
    baseUrl: "",
    modelsUrl: "",
    models: ["gpt-5.5", "gpt-5.4"],
    visionModels: [],
    visionModelsConfigured: false,
    default: "gpt-5.5",
    apiKeyEnv: "",
    keySet: false,
    requiresKey: false,
    configured: true,
    locked: true,
    balanceUrl: "",
    contextWindow: 1_000_000,
    reasoningProtocol: "openai",
    supportedEfforts: ["low", "medium", "high"],
    defaultEffort: "",
  };
}

console.log("\nsettings refresh snapshot");

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
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
window.scrollTo = () => {};

const settingsSnapshots = [baseSettings("standard"), baseSettings("compact")];
const mutationScopeKey = "settings-refresh-mutation";
await loadSettingsDetails(mutationScopeKey, async () => ({
  providers: [],
  officialProviders: [],
  providerKinds: [],
}));
let settingsCalls = 0;
let setDisplayModeCalls = 0;
let onChangedSettings: SettingsView | undefined;

window.go = {
  main: {
    App: {
      Settings: async () => settingsSnapshots[Math.min(settingsCalls++, settingsSnapshots.length - 1)],
      SetDisplayMode: async () => {
        setDisplayModeCalls += 1;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);

await act(async () => {
  root.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="general"
        settingsScopeKey={mutationScopeKey}
        onClose={() => {}}
        onChanged={(settings?: SettingsView | null) => {
          onChangedSettings = settings ?? undefined;
        }}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});

const compactButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.trim() === "紧凑") as HTMLButtonElement | undefined;
if (!compactButton) throw new Error("compact display mode button did not render");

await act(async () => {
  compactButton.click();
  await flushPromises();
});

eq(setDisplayModeCalls, 1, "display mode mutation is invoked once");
eq(settingsCalls, 2, "settings panel reads Settings only for initial load and post-save reload");
ok(onChangedSettings?.displayMode === "compact", "onChanged receives the post-save SettingsView snapshot");
ok(peekSettingsCore(mutationScopeKey)?.displayMode === "compact", "post-save core snapshot updates the current cache");
ok(peekSettingsDetails(mutationScopeKey) === null, "post-save mutation invalidates cached model details");

await act(async () => {
  root.unmount();
});

const scrollRootEl = document.createElement("div");
document.body.appendChild(scrollRootEl);
const scrollRoot = createRoot(scrollRootEl);
const scrollCalls: ScrollToOptions[] = [];
window.go = {
  main: {
    App: {
      Settings: async () => baseSettings("standard"),
      SetDisplayMode: async () => {
        throw new Error("forced display mode failure");
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  scrollRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="general"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
await waitFor("scroll test display mode controls", () => Boolean(Array.from(scrollRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "紧凑")));

const scrollContent = scrollRootEl.querySelector(".settings-center__content") as HTMLElement | null;
if (!scrollContent) throw new Error("settings content did not render");
Object.defineProperty(scrollContent, "scrollTo", {
  configurable: true,
  value: (options: ScrollToOptions) => {
    scrollCalls.push(options);
  },
});
const failingCompactButton = Array.from(scrollRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "紧凑") as HTMLButtonElement | undefined;
if (!failingCompactButton) throw new Error("failing compact display mode button did not render");

await act(async () => {
  failingCompactButton.click();
  await flushPromises();
});
await waitFor("display mode mutation error", () => Boolean(scrollRootEl.querySelector(".banner--error")));

ok(scrollCalls.some((options) => options.top === 0), "settings mutation error scrolls content to the top");

await act(async () => {
  scrollRoot.unmount();
});

const zhActiveWorkRootEl = document.createElement("div");
document.body.appendChild(zhActiveWorkRootEl);
const zhActiveWorkRoot = createRoot(zhActiveWorkRootEl);
const activeWorkSettingsError = "finish or cancel the current turn, answer pending prompts, and stop background jobs before changing effort";
window.go = {
  main: {
    App: {
      Settings: async () => baseSettings("standard"),
      SetDisplayMode: async () => {
        throw new Error(activeWorkSettingsError);
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  zhActiveWorkRoot.render(
    <LocaleProvider>
      <LocaleHarness pref="zh">
        <SettingsPanel
          initialTab="general"
          onClose={() => {}}
          onChanged={() => {}}
        />
      </LocaleHarness>
    </LocaleProvider>,
  );
  await flushPromises();
});
await waitFor("zh locale", () => document.documentElement.lang === "zh-CN");
await waitFor("zh display mode controls", () => Boolean(Array.from(zhActiveWorkRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "紧凑")));

const zhFailingCompactButton = Array.from(zhActiveWorkRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "紧凑") as HTMLButtonElement | undefined;
if (!zhFailingCompactButton) throw new Error("zh failing compact display mode button did not render");

await act(async () => {
  zhFailingCompactButton.click();
  await flushPromises();
});
await waitFor("zh active-work settings error", () => Boolean(zhActiveWorkRootEl.querySelector(".banner--error")));

ok(zhActiveWorkRootEl.textContent?.includes("检测到当前会话仍有任务运行中，暂时不能修改设置。") === true, "active-work settings error follows Chinese locale");
ok(zhActiveWorkRootEl.textContent?.includes(activeWorkSettingsError) === false, "raw active-work settings error is hidden");

await act(async () => {
  zhActiveWorkRoot.unmount();
});

const retryRootEl = document.createElement("div");
document.body.appendChild(retryRootEl);
const retryRoot = createRoot(retryRootEl);
let failingSettingsCalls = 0;
window.go = {
  main: {
    App: {
      Settings: async () => {
        failingSettingsCalls += 1;
        if (failingSettingsCalls === 1) throw new Error("/Users/example/.kwu/settings.toml: permission denied");
        return baseSettings("standard");
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  retryRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="general"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
await waitFor("settings load failure", () => Boolean(document.querySelector(".banner--error")));

ok(document.body.textContent?.includes("设置加载失败。") === true, "failed initial settings load shows a visible error");
ok(document.body.textContent?.includes("加载中…") === false, "failed initial settings load stops showing the loading state");

const retryButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.trim() === "重试") as HTMLButtonElement | undefined;
if (!retryButton) throw new Error("settings retry button did not render");

await act(async () => {
  retryButton.click();
  await flushPromises();
});
await waitFor("settings retry success", () => Boolean(Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.trim() === "紧凑")));

eq(failingSettingsCalls, 2, "settings retry calls Settings again");
ok(document.body.textContent?.includes("设置加载失败。") === false, "settings retry clears the load error");

await act(async () => {
  retryRoot.unmount();
});

const updatesRootEl = document.createElement("div");
document.body.appendChild(updatesRootEl);
const updatesRoot = createRoot(updatesRootEl);
window.go = {
  main: {
    App: {
      Settings: async () => baseSettings("standard"),
      Version: async () => "dev",
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  updatesRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="updates"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});

await waitFor("settings updates page", () => Boolean(updatesRootEl.textContent?.includes("启动时检测新版本")));
ok(document.body.textContent?.includes("启动时检测新版本") === true, "startup update check control is visible");
ok(document.body.textContent?.includes("软件更新") === true, "manual update check is visible");
ok(updatesRootEl.textContent?.includes("匿名启动统计") === false, "telemetry setting is hidden");
ok(updatesRootEl.textContent?.includes("共享聚合质量指标") === false, "metrics setting is hidden");

await act(async () => {
  updatesRoot.unmount();
});

const kwuRootEl = document.createElement("div");
document.body.appendChild(kwuRootEl);
const kwuRoot = createRoot(kwuRootEl);
const provider = kwuProvider();
const customProvider = {
  ...provider,
  builtIn: false,
  billingSource: "newapi_byok",
  locked: false,
  baseUrl: "https://custom.example.invalid/v1",
  apiKeyEnv: "CUSTOM_KWU_API_KEY",
};
const kwuSettings = baseSettings("standard");
kwuSettings.defaultModel = "kwu/gpt-5.5";
kwuSettings.providers = [customProvider];
kwuSettings.officialProviders = [provider];
kwuSettings.providerKinds = ["openai"];
window.go = {
  main: {
    App: {
      Settings: async () => kwuSettings,
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  kwuRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="providers"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
await waitFor("provider access subtab", () => Boolean(Array.from(kwuRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "接入")));
const kwuAccessButton = Array.from(kwuRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "接入") as HTMLButtonElement | undefined;
if (!kwuAccessButton) throw new Error("provider access subtab did not render");
await act(async () => {
  kwuAccessButton.click();
  await flushPromises();
});
await waitFor("kwu provider access card", () => kwuRootEl.textContent?.includes("云智企办") === true);
const officialCard = kwuRootEl.querySelector<HTMLElement>(".provider-access-card--builtin");
const customCard = [...kwuRootEl.querySelectorAll<HTMLElement>(".provider-access-card")]
  .find((card) => !card.classList.contains("provider-access-card--builtin"));

ok(kwuRootEl.textContent?.includes("云智企办") === true, "KWU built-in provider card uses the localized label");
ok(kwuRootEl.querySelectorAll(".provider-access-card").length === 2, "official and same-name custom KWU cards render together");
ok(officialCard?.textContent?.includes("https://custom.example.invalid/v1") === false, "official KWU card does not expose an address");
ok(customCard?.textContent?.includes("https://custom.example.invalid/v1") === true, "same-name custom KWU card keeps its address visible");
ok(kwuRootEl.textContent?.includes("官方积分渠道；登录后自动使用当前账户的专用模型凭据。") === true, "KWU built-in provider card uses the localized points-channel description");
ok(kwuRootEl.textContent?.includes(deprecatedKWUOfficialLabel) === false, "KWU built-in provider card does not append Official to the product name");
ok(kwuRootEl.textContent?.includes("DeepSeek Official") === false, "KWU built-in provider card does not show the DeepSeek label");
ok(kwuRootEl.textContent?.includes("积分渠道") === true, "KWU built-in provider card identifies the points channel");
ok(
  [...(officialCard?.querySelectorAll<HTMLButtonElement>("button") ?? [])].some((button) => button.textContent?.trim() === "刷新模型" && !button.disabled),
  "KWU built-in provider card keeps model refresh enabled without a displayed endpoint",
);
ok(kwuRootEl.querySelector("input[type='password']") === null, "KWU built-in provider card exposes no API key input");
ok(
  !Array.from(officialCard?.querySelectorAll("button") ?? []).some((button) => ["Configure", "Remove access"].includes(button.textContent?.trim() ?? "")),
  "KWU built-in provider card exposes no configure or remove action",
);

const mergedSettings = { ...kwuSettings, providers: [provider, customProvider] };
const mergedRoutes = modelRoutesForSettings(mergedSettings, mergedSettings.providers);
ok(
  mergedRoutes.some((route) => route.ref === "kwu/gpt-5.5" && route.billingSource === "subscription_pool")
    && mergedRoutes.some((route) => route.ref === "kwu/gpt-5.5" && route.billingSource === "newapi_byok"),
  "same kwu/model retains both subscription and BYOK routes",
);
const pickerHost = document.createElement("div");
document.body.appendChild(pickerHost);
const pickerRoot = createRoot(pickerHost);
const pickedRoutes: Array<[string, string | undefined]> = [];
await act(async () => {
  pickerRoot.render(
    <LocaleProvider>
      <ModelPicker
        s={mergedSettings}
        routes={mergedRoutes}
        value="kwu/gpt-5.5"
        selectedBillingSource="subscription_pool"
        disabled={false}
        onPick={(ref, billingSource) => pickedRoutes.push([ref, billingSource])}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
const pickerTrigger = pickerHost.querySelector<HTMLButtonElement>(".settings-model-picker__trigger");
if (!pickerTrigger) throw new Error("same-name model picker did not render");
await act(async () => {
  pickerTrigger.click();
  await flushPromises();
});
const pickerOptions = [...document.querySelectorAll<HTMLButtonElement>("[data-anchored-popover='active'][data-state='open'] .settings-model-picker__option")];
ok(pickerOptions.filter((option) => option.textContent?.includes("gpt-5.5")).length === 2, "ModelPicker keeps both same-name source options in one view");
const customOption = pickerOptions.find((option) => option.textContent?.includes("kwu") && !option.textContent?.includes("云智企办"));
if (!customOption) throw new Error("same-name custom model option did not render");
await act(async () => {
  customOption.click();
  await flushPromises();
});
ok(JSON.stringify(pickedRoutes[0]) === JSON.stringify(["kwu/gpt-5.5", "newapi_byok"]), "ModelPicker selection preserves the custom BYOK identity");
await act(async () => {
  pickerRoot.render(
    <LocaleProvider>
      <ModelPicker
        s={mergedSettings}
        routes={mergedRoutes}
        value="kwu/gpt-5.5"
        selectedBillingSource="newapi_byok"
        disabled={false}
        onPick={(ref, billingSource) => pickedRoutes.push([ref, billingSource])}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
const rerenderedPickerTrigger = pickerHost.querySelector<HTMLButtonElement>(".settings-model-picker__trigger");
if (!rerenderedPickerTrigger) throw new Error("same-name model picker did not rerender");
await act(async () => {
  rerenderedPickerTrigger.click();
  await flushPromises();
});
const officialOption = [...document.querySelectorAll<HTMLButtonElement>("[data-anchored-popover='active'][data-state='open'] .settings-model-picker__option")]
  .find((option) => option.textContent?.includes("云智企办"));
if (!officialOption) throw new Error("same-name official model option did not render");
await act(async () => {
  officialOption.click();
  await flushPromises();
});
ok(JSON.stringify(pickedRoutes[1]) === JSON.stringify(["kwu/gpt-5.5", "subscription_pool"]), "ModelPicker selection preserves the official subscription identity");
await act(async () => pickerRoot.unmount());
pickerHost.remove();

await act(async () => {
  kwuRoot.unmount();
});

const addProviderRootEl = document.createElement("div");
document.body.appendChild(addProviderRootEl);
const addProviderRoot = createRoot(addProviderRootEl);
window.go = {
  main: {
    App: {
      Settings: async () => baseSettings("standard"),
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  addProviderRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="providers"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
await waitFor("add provider access subtab", () => Boolean(Array.from(addProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "接入")));
const addProviderAccessButton = Array.from(addProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "接入") as HTMLButtonElement | undefined;
if (!addProviderAccessButton) throw new Error("add provider access subtab did not render");
await act(async () => {
  addProviderAccessButton.click();
  await flushPromises();
});
await waitFor("custom provider button", () => Boolean(Array.from(addProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "自定义供应商")));

await act(async () => {
  const customProviderButton = Array.from(addProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "自定义供应商") as HTMLButtonElement;
  customProviderButton.click();
  await flushPromises();
});
await waitFor("custom provider editor", () => Boolean(addProviderRootEl.querySelector(".provider-editor--wizard")));

const addProviderText = addProviderRootEl.textContent ?? "";
ok(addProviderText.includes("官方供应商") === false, "add provider panel exposes no official provider mode");
ok(addProviderText.includes("KWU_API_KEY") === false, "add provider panel exposes no KWU_API_KEY input");
ok(addProviderText.includes("自定义供应商名称") === true, "add provider panel opens the custom provider editor directly");

await act(async () => {
  addProviderRoot.unmount();
});

const duplicateProviderRootEl = document.createElement("div");
document.body.appendChild(duplicateProviderRootEl);
const duplicateProviderRoot = createRoot(duplicateProviderRootEl);
let saveProviderCalls = 0;
const duplicateProviderSettings = baseSettings("standard");
duplicateProviderSettings.providers = [kwuProvider()];
duplicateProviderSettings.providerKinds = ["openai"];
window.go = {
  main: {
    App: {
      Settings: async () => duplicateProviderSettings,
      SaveProvider: async () => {
        saveProviderCalls += 1;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

await act(async () => {
  duplicateProviderRoot.render(
    <LocaleProvider>
      <SettingsPanel
        initialTab="providers"
        onClose={() => {}}
        onChanged={() => {}}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});
await waitFor("duplicate provider access subtab", () => Boolean(Array.from(duplicateProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "接入")));
const duplicateAccessButton = Array.from(duplicateProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim() === "接入") as HTMLButtonElement | undefined;
if (!duplicateAccessButton) throw new Error("duplicate provider access subtab did not render");
await act(async () => {
  duplicateAccessButton.click();
  await flushPromises();
});
await waitFor("duplicate add provider button", () => Boolean(Array.from(duplicateProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim().includes("添加模型服务"))));
const duplicateAddButton = Array.from(duplicateProviderRootEl.querySelectorAll("button")).find((button) => button.textContent?.trim().includes("添加模型服务")) as HTMLButtonElement | undefined;
if (!duplicateAddButton) throw new Error("duplicate add provider button did not render");
await act(async () => {
  duplicateAddButton.click();
  await flushPromises();
});
await waitFor("custom provider editor", () => Boolean(duplicateProviderRootEl.querySelector(".provider-editor--wizard")));
const duplicateEditor = duplicateProviderRootEl.querySelector<HTMLElement>(".provider-editor--wizard");
const duplicateInputs = Array.from(duplicateEditor?.querySelectorAll<HTMLInputElement>("input.mem-input") ?? []);
if (duplicateInputs.length < 4) throw new Error("custom provider editor inputs did not render");
await act(async () => {
  setInputValue(duplicateInputs[0], "kwu");
  setInputValue(duplicateInputs[1], "https://example.test/v1");
  setInputValue(duplicateInputs[3], "model-a");
  await flushPromises();
});
const duplicateSaveButton = Array.from(duplicateEditor?.querySelectorAll("button") ?? []).find((button) => button.textContent?.trim() === "保存") as HTMLButtonElement | undefined;
if (!duplicateSaveButton) throw new Error("custom provider save button did not render");
await waitFor("custom provider save button enabled", () => !duplicateSaveButton.disabled);
await act(async () => {
  duplicateSaveButton.click();
  await flushPromises();
});

eq(saveProviderCalls, 0, "duplicate custom provider name does not call SaveProvider");
await waitFor("duplicate custom provider name inline error", () => duplicateProviderRootEl.textContent?.includes("供应商“kwu”已存在。") === true);
ok(duplicateProviderRootEl.textContent?.includes("供应商“kwu”已存在。") === true, "duplicate custom provider name shows an inline error");

await act(async () => {
  duplicateProviderRoot.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
