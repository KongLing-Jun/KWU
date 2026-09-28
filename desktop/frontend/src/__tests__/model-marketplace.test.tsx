// Run: tsx src/__tests__/model-marketplace.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { ModelMarketplace } from "../components/ModelMarketplace";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";
import type { ModelInfo, ModelMarketplaceView, SharedModelCatalogState } from "../lib/types";

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
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function setupDom() {
  const dom = new JSDOM('<!doctype html><html><body><div id="root"></div></body></html>', {
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
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window, "matchMedia", {
    configurable: true,
    value: (query: string) => ({
      matches: true,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }),
  });
  return document.getElementById("root")!;
}

async function renderMarketplace(
  initial: ModelMarketplaceView | Error,
  refreshed: ModelMarketplaceView | Error | Promise<ModelMarketplaceView> = initial instanceof Error ? { sources: [] } : initial,
  selectable: ModelInfo[] = [],
  initialAccess: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" },
  claimAutoRefresh?: () => boolean,
) {
  const container = setupDom();
  const root: Root = createRoot(container);
  const opened: string[] = [];
  let loadCalls = 0;
  let refreshCalls = 0;
  const selected: Array<{ ref: string; billingSource?: ModelInfo["billingSource"] }> = [];
  let configureCalls = 0;
  let accountCalls = 0;
  let chatCalls = 0;
  let marketplaceChangedCalls = 0;
  let autoRefreshClaimed = false;
  const claim = claimAutoRefresh ?? (() => {
    if (autoRefreshClaimed) return false;
    autoRefreshClaimed = true;
    return true;
  });
  const loadMarketplace = async () => {
    loadCalls += 1;
    if (initial instanceof Error) throw initial;
    return initial;
  };
  const refreshMarketplace = async () => {
    refreshCalls += 1;
    if (refreshed instanceof Error) throw refreshed;
    return await refreshed;
  };
  const initialCatalog: SharedModelCatalogState = {
    state: "ready",
    routes: selectable,
    marketplace: initial instanceof Error ? undefined : initial,
  };
  const render = (subscriptionAccess: SubscriptionAccess, modelCatalog: SharedModelCatalogState) => (
    <LocaleProvider>
      <ModelMarketplace
        loadMarketplace={loadMarketplace}
        refreshMarketplace={refreshMarketplace}
        claimAutoRefresh={claim}
        modelCatalog={modelCatalog}
        subscriptionAccess={subscriptionAccess}
        onSelectModel={async (ref, billingSource) => { selected.push({ ref, billingSource }); }}
        onConfigureModels={() => { configureCalls += 1; }}
        onRequireAccount={() => { accountCalls += 1; }}
        onModelSelected={() => { chatCalls += 1; }}
        onMarketplaceChanged={() => { marketplaceChangedCalls += 1; }}
      />
    </LocaleProvider>
  );
  await act(async () => {
    root.render(render(initialAccess, initialCatalog));
  });
  return {
    container,
    root,
    opened,
    selected,
    rerender: async (subscriptionAccess: SubscriptionAccess) => {
      await act(async () => root.render(render(subscriptionAccess, initialCatalog)));
    },
    rerenderCatalog: async (modelCatalog: SharedModelCatalogState) => {
      await act(async () => root.render(render(initialAccess, modelCatalog)));
    },
    calls: () => ({ loadCalls, refreshCalls, configureCalls, accountCalls, chatCalls, marketplaceChangedCalls }),
  };
}

const freshView: ModelMarketplaceView = {
  sources: [
    {
      id: "international",
      label: "国际模型",
      sourceUrl: "https://pricing.example.invalid/api/pricing",
      pricingUrl: "https://pricing.example.invalid/models",
      pricingVersion: "airclaude-version",
      pricingState: "fresh",
      metadataStatus: "ready",
      models: [
        {
          id: "claude-test",
          name: "claude-test",
          icon: "Claude.Color",
          vendor: "Anthropic",
          vendorIcon: "Claude.Color",
          tags: ["Claude", "Chat", "Subscribe_Free", "Subscribe_Lite", "Subscribe_Plus", "Subscribe_Pro"],
          groups: ["default"],
          endpoints: ["anthropic"],
          billingType: "per_token",
          quotaType: 0,
          prices: [
            { key: "input_price", value: "0.500000000000000000" },
            { key: "output_price", value: "0" },
            { key: "search_price", value: "0.01" },
            { key: "image_price", value: "2" },
          ],
          pricingTiers: [
            {
              maxTokens: "-1",
              prices: [{ key: "cache_read_price", value: "0.6000000000000001" }],
            },
          ],
        },
      ],
    },
  ],
};

console.log("\nmodel marketplace component");

{
  const sharedRoute: ModelInfo = {
    ref: "custom/shared-model",
    provider: "custom",
    model: "shared-model",
    billingSource: "newapi_byok",
    current: false,
  };
  const { container, root, rerenderCatalog, calls } = await renderMarketplace(freshView);
  await waitFor("ready empty shared catalog", () => container.textContent?.includes("当前套餐暂无可切换模型") === true);
  ok(!container.textContent?.includes("模型配置暂不可用"), "a ready empty shared catalog renders the normal empty state");
  await rerenderCatalog({ state: "loading" });
  ok(
    container.textContent?.includes("正在读取模型配置…") === true
      && container.querySelectorAll("[data-selectable-model]").length === 0,
    "shared catalog loading clears the previous selectable list",
  );
  await rerenderCatalog({ state: "unavailable" });
  ok(container.textContent?.includes("模型配置暂不可用") === true, "shared catalog failure has a distinct unavailable state");
  await rerenderCatalog({ state: "ready", routes: [sharedRoute], marketplace: freshView });
  ok(
    container.querySelector('[data-selectable-model="custom/shared-model"]') !== null
      && calls().loadCalls === 1,
    "shared catalog updates render without reloading the price cache",
  );
  await act(async () => root.unmount());
}

{
  const subscriptionOnly: ModelInfo[] = [
    {
      ref: "kwu/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: true,
    },
  ];
  const { container, root, opened, selected, calls } = await renderMarketplace(
    freshView,
    freshView,
    subscriptionOnly,
    { state: "unavailable" },
  );
  await waitFor("signed-out marketplace", () => Boolean(container.querySelector('[data-model-name="claude-test"]')));
  ok(container.querySelectorAll("[data-selectable-model]").length === 0, "signed-out marketplace hides subscription-only routes");
  const emptyStatus = container.querySelector(".model-marketplace-selectable__status")?.textContent ?? "";
  ok(
    (emptyStatus.includes("subscription") || emptyStatus.includes("订阅")) && !emptyStatus.includes("提供商"),
    "signed-out marketplace explains the empty subscription list without blaming provider configuration",
  );
  await act(async () => container.querySelector<HTMLElement>('[data-model-name="claude-test"]')?.click());
  ok(opened.length === 0, "signed-out catalog cards do not open an external page");
  ok(
    calls().accountCalls === 0 && calls().configureCalls === 0 && calls().chatCalls === 0 && selected.length === 0,
    "signed-out catalog cards do not trigger account, configuration, chat, or model selection actions",
  );
  await act(async () => root.unmount());
}

{
  const mixedRoutes: ModelInfo[] = [
    {
      ref: "airclaude_subscription/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: false,
    },
    {
      ref: "custom/local-model",
      provider: "custom",
      model: "local-model",
      billingSource: "newapi_byok",
      current: false,
    },
  ];
  const missingSourceView: ModelMarketplaceView = { sources: [] };
  const { container, root } = await renderMarketplace(missingSourceView, missingSourceView, mixedRoutes);
  await waitFor("missing marketplace source BYOK route", () => Boolean(container.querySelector('[data-selectable-model="custom/local-model"]')));
  ok(container.querySelector('[data-selectable-model="airclaude_subscription/claude-test"]') === null, "missing marketplace source fails subscription routes closed");
  ok(container.textContent?.includes("需要联网获取模型数据") === true, "missing marketplace source retains the catalog recovery message");
  await act(async () => root.unmount());
}

{
  const sameNameRoutes: ModelInfo[] = [
    {
      ref: "airclaude_subscription/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: false,
    },
    {
      ref: "custom/claude-test",
      provider: "custom",
      model: "claude-test",
      billingSource: "newapi_byok",
      current: false,
    },
  ];
  const { container, root, selected, rerender, calls } = await renderMarketplace(freshView, freshView, sameNameRoutes);
  await waitFor("authorized marketplace routes", () => container.querySelectorAll("[data-selectable-model]").length === 2);
  await rerender({ state: "unavailable" });
  await waitFor("updated marketplace access", () => container.querySelectorAll("[data-selectable-model]").length === 1);
  ok(
    container.querySelector('[data-selectable-model="airclaude_subscription/claude-test"]') === null
      && container.querySelector('[data-selectable-model="custom/claude-test"]') !== null,
    "access changes remove only the same-name subscription route",
  );
  ok(calls().loadCalls === 1, "access changes recompute the selectable list without another marketplace request");
  ok(selected.length === 0, "access changes do not invoke a model setter");
  ok(container.querySelector('[data-model-name="claude-test"]') !== null, "account filtering leaves the complete marketplace browser visible");
  await act(async () => root.unmount());
}

{
  const mixedRoutes: ModelInfo[] = [
    {
      ref: "airclaude_subscription/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: false,
    },
    {
      ref: "custom/local-model",
      provider: "custom",
      model: "local-model",
      billingSource: "newapi_byok",
      current: false,
    },
  ];
  const { container, root, selected } = await renderMarketplace(new Error("cache unavailable"), undefined, mixedRoutes);
  await waitFor("marketplace failure BYOK route", () => Boolean(container.querySelector('[data-selectable-model="custom/local-model"]')));
  ok(container.querySelector('[data-selectable-model="airclaude_subscription/claude-test"]') === null, "marketplace failure hides subscription routes");
  ok(container.textContent?.includes("需要联网获取模型数据") === true, "marketplace failure keeps the catalog error visible below selectable BYOK routes");
  await act(async () => container.querySelector<HTMLButtonElement>('[data-selectable-model="custom/local-model"]')?.click());
  ok(selected.length === 1 && selected[0].billingSource === "newapi_byok", "marketplace failure preserves a selectable BYOK route");
  await act(async () => root.unmount());
}

{
  const { container, root, calls } = await renderMarketplace(freshView);
  await waitFor("fresh source", () => container.textContent?.includes("claude-test") ?? false);
  const text = container.textContent ?? "";
  const card = container.querySelector<HTMLElement>('[data-model-name="claude-test"]');
  ok(calls().loadCalls === 1 && calls().refreshCalls === 0, "initial entry performs one cache-only load and no refresh");
  ok(
    card?.tagName === "ARTICLE" &&
      card.querySelector(".model-marketplace-card__name")?.textContent === "claude-test" &&
      card.querySelector(".model-marketplace-card__vendor")?.textContent === "Anthropic",
    "catalog cards are static identity summaries with model and vendor",
  );
  ok(
    text.includes("浏览适合办公任务的国际模型") &&
      text.includes("本地缓存优先 · 无缓存时自动获取") &&
      container.querySelectorAll("[data-model-marketplace-refresh]").length === 1,
    "catalog header explains the cache boundary and exposes one primary refresh action",
  );
  ok(
    container.querySelector("[data-model-price]") === null &&
      card?.querySelector(".model-marketplace-card__summary") === null &&
      card?.querySelector(".model-marketplace-card__tags") === null &&
      card?.querySelector(".model-marketplace-card__details") === null &&
      !text.includes("按 token 计费") &&
      !text.includes("查看详情"),
    "catalog cards omit pricing, tags, summaries, details, and actions",
  );
  ok(!container.querySelector("[role=tablist]") && !text.includes("国内模型"), "single international source hides the source switcher");
  ok(
    container.querySelector(".model-marketplace-card__avatar--claude") !== null,
    "model icon metadata renders the matching local brand logo",
  );
  ok(container.querySelector(".model-marketplace__open") === null, "model marketplace has no external models-page button");
  await act(async () => root.unmount());
}

{
  const pricingShapesView: ModelMarketplaceView = {
    sources: [{
      ...freshView.sources[0],
      models: [
        {
          id: "request-only",
          name: "request-only",
          summary: "按次调用的视频模型",
          tags: ["Video"],
          billingType: "per_request",
          quotaType: 1,
          prices: [{ key: "request_price", value: "1.5" }],
        },
        {
          id: "tier-only",
          name: "tier-only",
          tags: ["Chat"],
          billingType: "per_token",
          quotaType: 0,
          pricingTiers: [
            {
              maxTokens: "200000",
              prices: [
                { key: "input_price", value: "3" },
                { key: "output_price", value: "15" },
              ],
            },
            {
              maxTokens: "-1",
              prices: [
                { key: "input_price", value: "6" },
                { key: "output_price", value: "22.5" },
              ],
            },
          ],
        },
        {
          id: "special-prices",
          name: "special-prices",
          billingType: "per_token",
          quotaType: 0,
          prices: [
            { key: "input_price", value: "0.12" },
            { key: "image_price", value: "0.47" },
          ],
        },
        {
          id: "identity-only",
          name: "identity-only",
          summary: " ",
          tags: [" "],
          prices: [{ key: "", value: "" }],
          pricingTiers: [{ prices: [{ key: "", value: "" }] }],
        },
      ],
    }],
  };
  const { container, root } = await renderMarketplace(pricingShapesView);
  await waitFor("all pricing shapes", () => Boolean(container.querySelector('[data-model-name="identity-only"]')));
  const requestCard = container.querySelector('[data-model-name="request-only"]');
  const tierCard = container.querySelector('[data-model-name="tier-only"]');
  const specialCard = container.querySelector('[data-model-name="special-prices"]');
  const identityCard = container.querySelector('[data-model-name="identity-only"]');
  const pricingShapeCards = [requestCard, tierCard, specialCard, identityCard];
  ok(
    pricingShapeCards.every((card) => card?.tagName === "ARTICLE") &&
      pricingShapeCards.every((card) => card?.querySelector("[data-model-price], .model-marketplace-card__summary, .model-marketplace-card__tags, .model-marketplace-card__details") === null),
    "all pricing shapes render the same static identity-only card",
  );
  await act(async () => root.unmount());
}

{
  const configuredModels: ModelInfo[] = [
    {
      ref: "airclaude_subscription/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: false,
    },
  ];
  const { container, root, opened, selected, calls } = await renderMarketplace(freshView, freshView, configuredModels);
  await waitFor("catalog marketplace card", () => Boolean(container.querySelector('[data-model-name="claude-test"]')));
  await act(async () => container.querySelector<HTMLElement>('[data-model-name="claude-test"]')?.click());
  ok(
    opened.length === 0 && selected.length === 0 && calls().chatCalls === 0,
    "signed-in catalog cards remain static without switching models or returning to chat",
  );
  ok(
    container.querySelector('[data-model-name="claude-test"]')?.getAttribute("aria-pressed") === null &&
      container.querySelector('[data-model-name="claude-test"]')?.tagName === "ARTICLE" &&
      container.querySelector('[data-model-name="claude-test"]')?.textContent?.includes("查看详情") === false,
    "catalog cards expose no selection state or navigation action",
  );
  await act(async () => root.unmount());
}

{
  const configuredModels: ModelInfo[] = [
    {
      ref: "airclaude_subscription/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: false,
    },
  ];
  const { container, root, opened, selected, calls } = await renderMarketplace(freshView, freshView, configuredModels);
  await waitFor("selectable model card", () => Boolean(container.querySelector('[data-selectable-model="airclaude_subscription/claude-test"]')));
  await act(async () => container.querySelector<HTMLButtonElement>('[data-selectable-model="airclaude_subscription/claude-test"]')?.click());
  ok(
    selected[0]?.ref === "airclaude_subscription/claude-test" && selected[0]?.billingSource === "subscription_pool",
    "selectable model cards keep switching through the configured subscription route",
  );
  ok(calls().chatCalls === 1 && opened.length === 0, "selectable model cards return to chat without opening AirClaude");
  await act(async () => root.unmount());
}

{
  const selectableModels = Array.from({ length: 12 }, (_, index): ModelInfo => ({
    ref: `custom/model-${index}`,
    provider: "custom",
    model: `model-${index}`,
    billingSource: "newapi_byok",
    current: false,
  }));
  const { container, root } = await renderMarketplace(freshView, freshView, selectableModels);
  await waitFor("scrollable selectable models", () => container.querySelectorAll("[data-selectable-model]").length === 12);
  ok(
    container.querySelector(".model-marketplace-selectable__scroller")?.getAttribute("aria-label") === "可选择模型列表",
    "configured models render inside a labelled scroll region",
  );
  ok(
    container.querySelectorAll("button.model-marketplace-filter[aria-haspopup=listbox]").length === 2 &&
      container.querySelectorAll("select").length === 0,
    "vendor and plan filters use full-width listbox triggers instead of partial native select hit areas",
  );
  await act(async () => root.unmount());
}

{
  const deliberatelyUnorderedRoutes: ModelInfo[] = [
    { ref: "zeta/z", provider: "zeta", model: "z", billingSource: "newapi_byok", current: false },
    { ref: "beta/b", provider: "beta", model: "b", billingSource: "newapi_byok", current: false },
    {
      ref: "airclaude_subscription/claude-test",
      provider: "airclaude_subscription",
      model: "claude-test",
      billingSource: "subscription_pool",
      current: false,
    },
    { ref: "alpha/a", provider: "alpha", model: "a", billingSource: "newapi_byok", current: false },
  ];
  const { container, root } = await renderMarketplace(freshView, freshView, deliberatelyUnorderedRoutes);
  await waitFor("shared selectable order", () => container.querySelectorAll("[data-selectable-model]").length === 4);
  const actualOrder = [...container.querySelectorAll<HTMLElement>("[data-selectable-model]")]
    .map((item) => item.dataset.selectableModel);
  ok(
    JSON.stringify(actualOrder) === JSON.stringify([
      "airclaude_subscription/claude-test",
      "alpha/a",
      "beta/b",
      "zeta/z",
    ]),
    "marketplace flattens the same subscription-first and stable provider order as the switcher",
  );
  await act(async () => root.unmount());
}

{
  const filterView: ModelMarketplaceView = {
    sources: [
      {
        ...freshView.sources[0],
        models: [
          freshView.sources[0].models[0],
          { id: "lite-test", name: "lite-test", vendor: "OpenAI", tags: ["Subscribe_Lite", "Subscribe_Plus", "Subscribe_Pro"], endpoints: ["openai"] },
          { id: "plus-test", name: "plus-test", vendor: "OpenAI", tags: ["Subscribe_Plus", "Subscribe_Pro"], endpoints: ["openai"] },
          { id: "pro-test", name: "pro-test", vendor: "OpenAI", tags: ["Subscribe_Pro"], endpoints: ["openai"] },
          { id: "metered-test", name: "metered-test", vendor: "OpenAI", tags: ["Chat"], endpoints: ["openai"] },
        ],
      },
    ],
  };
  const { container, root } = await renderMarketplace(filterView);
  await waitFor("filterable marketplace", () => container.querySelectorAll(".model-marketplace-card").length === 5);

  const vendorTrigger = container.querySelector<HTMLButtonElement>("[data-model-marketplace-vendor-filter]");
  vendorTrigger?.style.setProperty("--accent", "rgb(201, 154, 58)");
  vendorTrigger?.style.setProperty("--surface", "rgb(255, 253, 246)");
  vendorTrigger?.style.setProperty("--border", "rgb(222, 205, 158)");
  vendorTrigger?.style.setProperty("--fg", "rgb(72, 57, 26)");
  vendorTrigger?.style.setProperty("--fg-faint", "rgb(128, 108, 62)");
  await act(async () => vendorTrigger?.click());
  await waitFor("vendor listbox", () => Boolean(document.querySelector('[role="listbox"][aria-label="厂商筛选"]')));
  const vendorPopover = document.querySelector<HTMLElement>(".model-marketplace-filter-menu");
  ok(
    vendorTrigger?.getAttribute("aria-expanded") === "true" &&
      document.querySelector('[data-model-marketplace-filter-option="OpenAI"]') !== null &&
      vendorPopover?.style.getPropertyValue("--accent") === "rgb(201, 154, 58)" &&
      vendorPopover.style.getPropertyValue("--surface") === "rgb(255, 253, 246)" &&
      vendorPopover.style.getPropertyValue("--border") === "rgb(222, 205, 158)" &&
      vendorPopover.style.getPropertyValue("--fg") === "rgb(72, 57, 26)" &&
      vendorPopover.style.getPropertyValue("--fg-faint") === "rgb(128, 108, 62)",
    "clicking the compound vendor trigger opens a listbox with the trigger theme forwarded through the portal",
  );
  await act(async () => document.querySelector<HTMLButtonElement>('[data-model-marketplace-filter-option="OpenAI"]')?.click());
  await waitFor("vendor-filtered models", () => container.querySelectorAll(".model-marketplace-card").length === 4);
  ok(
    container.querySelector('[data-model-name="lite-test"]') !== null &&
      container.querySelector('[data-model-name="claude-test"]') === null &&
      container.querySelector('[data-model-marketplace-result-count]')?.textContent?.includes("4 个结果") === true,
    "selecting a vendor filters the marketplace cards and updates the visible result summary",
  );
  await act(async () => vendorTrigger?.click());
  await act(async () => document.querySelector<HTMLButtonElement>('[data-model-marketplace-filter-option="all"]')?.click());
  await waitFor("vendor filter reset", () => container.querySelectorAll(".model-marketplace-card").length === 5);

  const searchInput = container.querySelector<HTMLInputElement>("[data-model-marketplace-search]");
  ok(
    searchInput?.getAttribute("aria-label") === "搜索模型或厂商" && searchInput.placeholder === "搜索模型或厂商…",
    "marketplace search describes its model-and-vendor scope",
  );
  await act(async () => {
    if (!searchInput) return;
    searchInput.value = "OpenAI";
    searchInput.dispatchEvent(new window.InputEvent("input", { bubbles: true }));
  });
  await waitFor("vendor text matched by search", () => container.querySelectorAll(".model-marketplace-card").length === 4);
  await act(async () => {
    if (!searchInput) return;
    searchInput.value = "LITE";
    searchInput.dispatchEvent(new window.InputEvent("input", { bubbles: true }));
  });
  await waitFor("case-insensitive model name search", () => container.querySelectorAll(".model-marketplace-card").length === 1);
  ok(
    container.querySelector('[data-model-name="lite-test"]') !== null &&
      container.querySelector('[data-model-name="claude-test"]') === null,
    "marketplace search matches model names and vendors case-insensitively without matching tags",
  );
  await act(async () => {
    if (!searchInput) return;
    searchInput.value = "";
    searchInput.dispatchEvent(new window.InputEvent("input", { bubbles: true }));
  });
  await waitFor("search reset", () => container.querySelectorAll(".model-marketplace-card").length === 5);

  const planTrigger = container.querySelector<HTMLButtonElement>("[data-model-marketplace-plan-filter]");
  await act(async () => {
    planTrigger?.dispatchEvent(new window.KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
  });
  await waitFor("keyboard-focused plan listbox", () => (
    Boolean(document.querySelector('[role="listbox"][aria-label="套餐筛选"]')) &&
    document.activeElement?.getAttribute("role") === "option"
  ));
  ok(
    planTrigger?.getAttribute("aria-expanded") === "true" && document.activeElement?.getAttribute("role") === "option",
    "keyboard interaction opens the plan listbox and moves focus to an option",
  );
  ok(
    document.querySelector('[data-model-marketplace-filter-option="Free"]')?.textContent?.includes("Free（1）") === true &&
      document.querySelector('[data-model-marketplace-filter-option="experience"]')?.textContent?.includes("体验套餐（4）") === true &&
      document.querySelector('[data-model-marketplace-filter-option="Lite"]') === null &&
      document.querySelector('[data-model-marketplace-filter-option="Plus"]') === null &&
      document.querySelector('[data-model-marketplace-filter-option="Pro"]') === null,
    "plan options only expose Free and the Pro-backed experience plan",
  );
  await act(async () => document.querySelector<HTMLButtonElement>('[data-model-marketplace-filter-option="experience"]')?.click());
  await waitFor("experience plan selection", () => container.querySelectorAll(".model-marketplace-card").length === 4);
  ok(
    container.querySelector('[data-model-name="claude-test"]') !== null &&
      container.querySelector('[data-model-name="lite-test"]') !== null &&
      container.querySelector('[data-model-name="plus-test"]') !== null &&
      container.querySelector('[data-model-name="pro-test"]') !== null &&
      container.querySelector('[data-model-name="metered-test"]') === null &&
      container.querySelector("#model-marketplace-catalog-title")?.textContent === "体验套餐模型" &&
      container.querySelector('[data-model-marketplace-result-count]')?.textContent?.includes("4 个结果") === true,
    "experience plan reuses the Pro tag and labels the filtered catalog",
  );
  await waitFor("plan selection and trigger focus", () => (
    (planTrigger?.textContent?.includes("体验套餐（4）") ?? false) &&
    planTrigger?.getAttribute("aria-expanded") === "false" &&
    document.activeElement === planTrigger
  ));
  ok(
    planTrigger?.getAttribute("aria-expanded") === "false" && document.activeElement === planTrigger,
    "choosing a plan closes the listbox and restores trigger focus",
  );
  await act(async () => root.unmount());
}

{
  const inferredLogoView: ModelMarketplaceView = {
    sources: [
      {
        ...freshView.sources[0],
        models: [
          { id: "glm-5.2", name: "glm-5.2", vendor: "GLM" },
          { id: "deepseek-v4", name: "deepseek-v4", vendor: "Deepseek" },
        ],
      },
    ],
  };
  const { container, root } = await renderMarketplace(inferredLogoView);
  await waitFor("inferred model logos", () => container.querySelectorAll(".model-marketplace-card").length === 2);
  ok(
    container.querySelector(".model-marketplace-card__avatar--zhipu") !== null &&
    container.querySelector(".model-marketplace-card__avatar--deepseek") !== null,
    "missing vendor icon metadata falls back to the model and vendor names",
  );
  await act(async () => root.unmount());
}

{
  const emptyView: ModelMarketplaceView = {
    sources: [
      {
        id: "international",
        label: "国际模型",
        sourceUrl: "https://pricing.example.invalid/api/pricing",
        pricingUrl: "https://pricing.example.invalid/models",
        pricingState: "unavailable",
        metadataStatus: "metadata_unavailable",
        models: [],
        errorCode: "pricing_unavailable",
      },
      {
        id: "secondary",
        label: "备用模型",
        sourceUrl: "https://pricing.example.invalid/api/pricing-secondary",
        pricingUrl: "https://pricing.example.invalid/models",
        pricingState: "unavailable",
        metadataStatus: "metadata_unavailable",
        models: [],
      },
    ],
  };
  const { container, root, calls } = await renderMarketplace(emptyView, freshView);
  await waitFor("automatically refreshed models", () => container.textContent?.includes("claude-test") ?? false);
  ok(calls().loadCalls === 1 && calls().refreshCalls === 1, "an unavailable empty cache automatically refreshes once");
  ok(calls().marketplaceChangedCalls === 1, "successful automatic refresh invalidates parent entitlement facts once");
  await act(async () => root.unmount());
}

{
  const missingSourceView: ModelMarketplaceView = { sources: [] };
  const unavailableView: ModelMarketplaceView = {
    sources: [{
      ...freshView.sources[0],
      pricingState: "unavailable",
      metadataStatus: "metadata_unavailable",
      models: [],
    }],
  };
  const { container, root, calls } = await renderMarketplace(missingSourceView, unavailableView);
  await waitFor("unavailable automatic refresh result", () => (
    calls().refreshCalls === 1 &&
    container.textContent?.includes("需要联网获取模型数据") === true &&
    container.querySelector<HTMLButtonElement>(".model-marketplace__refresh")?.disabled === false
  ));
  await act(async () => { await flushPromises(); });
  ok(calls().refreshCalls === 1, "an unavailable automatic refresh result does not trigger another automatic request");
  await act(async () => root.unmount());
}

{
  const missingSourceView: ModelMarketplaceView = { sources: [] };
  const { container, root, calls } = await renderMarketplace(missingSourceView, freshView);
  await waitFor("missing-source automatic refresh", () => container.textContent?.includes("claude-test") ?? false);
  ok(calls().loadCalls === 1 && calls().refreshCalls === 1, "a missing source automatically refreshes once");
  await act(async () => root.unmount());
}

{
  const freshEmptyView: ModelMarketplaceView = {
    sources: [{ ...freshView.sources[0], pricingState: "fresh", models: [] }],
  };
  let claims = 0;
  const { container, root, calls } = await renderMarketplace(
    freshEmptyView,
    freshView,
    [],
    { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" },
    () => { claims += 1; return true; },
  );
  await waitFor("fresh empty catalog", () => container.textContent?.includes("暂无模型数据") ?? false);
  ok(calls().refreshCalls === 0 && claims === 0, "a fresh empty catalog does not claim or start automatic refresh");
  await act(async () => root.unmount());
}

{
  const missingSourceView: ModelMarketplaceView = { sources: [] };
  const claim = (() => {
    let claimed = false;
    return () => {
      if (claimed) return false;
      claimed = true;
      return true;
    };
  })();
  const first = await renderMarketplace(missingSourceView, new Error("offline"), [], undefined, claim);
  await waitFor("failed automatic refresh", () => first.calls().refreshCalls === 1 && first.container.textContent?.includes("需要联网获取模型数据") === true);
  const retry = first.container.querySelectorAll<HTMLButtonElement>("[data-model-marketplace-refresh]").item(1);
  await act(async () => retry.click());
  await waitFor("manual retry after automatic failure", () => first.calls().refreshCalls === 2);
  ok(first.calls().refreshCalls === 2, "automatic failure does not loop and manual refresh remains available");
  await act(async () => first.root.unmount());

  const second = await renderMarketplace(missingSourceView, freshView, [], undefined, claim);
  await waitFor("second mount cache read", () => second.calls().loadCalls === 1);
  ok(second.calls().refreshCalls === 0, "the same application-level claim prevents automatic refresh after remount");
  await act(async () => second.root.unmount());
}

{
  const missingSourceView: ModelMarketplaceView = { sources: [] };
  let resolveRefresh!: (view: ModelMarketplaceView) => void;
  const deferredRefresh = new Promise<ModelMarketplaceView>((resolve) => { resolveRefresh = resolve; });
  const { container, root, calls } = await renderMarketplace(missingSourceView, deferredRefresh);
  await waitFor("automatic refresh in flight", () => calls().refreshCalls === 1);
  const retry = container.querySelectorAll<HTMLButtonElement>("[data-model-marketplace-refresh]").item(1);
  await act(async () => retry.click());
  ok(calls().refreshCalls === 1, "automatic and manual refresh share a synchronous in-flight guard");
  resolveRefresh(freshView);
  await waitFor("deferred refresh result", () => container.textContent?.includes("claude-test") ?? false);
  await act(async () => root.unmount());
}

{
  const staleUsableView: ModelMarketplaceView = {
    sources: [
      {
        ...freshView.sources[0],
        pricingState: "stale_usable",
        partial: false,
      },
    ],
  };
  const { container, root, calls } = await renderMarketplace(staleUsableView);
  await waitFor("stale usable state", () => container.textContent?.includes("模型数据缓存已过期，暂可使用") ?? false);
  ok(
    container.querySelector('[data-model-name="claude-test"]') !== null && container.querySelector("[data-model-price]") === null,
    "stale usable is explicit and still displays cached model identity without prices",
  );
  ok(calls().refreshCalls === 1, "stale usable cached models trigger exactly one automatic recovery refresh");
  await act(async () => root.unmount());
}

{
  const blockedView: ModelMarketplaceView = {
    sources: [
      {
        id: "international",
        label: "国际模型",
        sourceUrl: "https://pricing.example.invalid/api/pricing",
        pricingUrl: "https://pricing.example.invalid/models",
        pricingState: "stale_blocked",
        metadataStatus: "ready",
        errorCode: "pricing_unavailable",
        models: [{ id: "model-a", name: "model-a", vendor: "OpenAI" }],
      },
    ],
  };
  const { container, root, calls } = await renderMarketplace(blockedView);
  await waitFor("blocked state", () => container.textContent?.includes("模型数据已过期") ?? false);
  const text = container.textContent ?? "";
  ok(
    text.includes("model-a") && text.includes("OpenAI") && text.includes("刷新失败，已使用缓存") && !text.includes("输入价格"),
    "fallback keeps returned metadata, reports refresh failure, and does not invent price facts",
  );
  ok(calls().refreshCalls === 1, "blocked stale metadata triggers exactly one automatic recovery refresh");
  await act(async () => root.unmount());
}

{
  const writeFailedView: ModelMarketplaceView = {
    sources: [
      {
        ...freshView.sources[0],
        metadataStatus: "metadata_write_failed",
      },
    ],
  };
  const { container, root } = await renderMarketplace(writeFailedView);
  await waitFor("metadata warning", () => container.textContent?.includes("元数据缓存写入失败") ?? false);
  ok((container.textContent ?? "").includes("claude-test"), "metadata write failure still displays the current in-memory catalog");
  await act(async () => root.unmount());
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
