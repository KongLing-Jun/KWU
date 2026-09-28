// Run: tsx src/__tests__/model-switcher-stable-snapshot.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { ModelSwitcher } from "../components/ModelSwitcher";
import { LocaleProvider } from "../lib/i18n";
import type { EffortInfo, ModelInfo, ModelMarketplaceView, SharedModelCatalogState } from "../lib/types";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
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
Object.defineProperty(globalThis.window, "matchMedia", {
  configurable: true,
  value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
});
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });

const marketplace: ModelMarketplaceView = {
  sources: [{
    id: "international",
    label: "International",
    sourceUrl: "",
    pricingUrl: "",
    pricingState: "fresh",
    metadataStatus: "ready",
    models: [
      { id: "pro", name: "pro-model", tags: ["Subscribe_Pro"] },
      { id: "free", name: "free-model", tags: ["Subscribe_Free"] },
      { id: "duplicate", name: "duplicate", tags: ["Subscribe_Pro"] },
    ],
  }],
};
const routes: ModelInfo[] = [
  { ref: "kwu/pro-model", provider: "kwu", model: "pro-model", billingSource: "subscription_pool", current: true },
  { ref: "kwu/free-model", provider: "kwu", model: "free-model", billingSource: "subscription_pool", current: false },
  { ref: "custom/local", provider: "custom", model: "local", billingSource: "newapi_byok", current: false },
];
const proAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" } as const;
const freeAccess = { state: "ready", plan: "free", requiredTag: "Subscribe_Free" } as const;

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
let picked = "";
const render = async (
  modelCatalog: SharedModelCatalogState,
  access: SubscriptionAccess = proAccess,
  label = "kwu/pro-model",
  selectionPending = false,
  billingSource: ModelInfo["billingSource"] | null = "subscription_pool",
  selectionPresentation: "auto" | "choose" | "pending" = "auto",
  effortSnapshot?: string,
  effort?: EffortInfo,
  entryMode: "root" | "models" = "root",
  unmatchedLabel?: string,
) => {
  await act(async () => {
    root.render(
      <LocaleProvider>
        <ModelSwitcher
          label={label}
          modelCatalog={modelCatalog}
          currentBillingSource={billingSource ?? undefined}
          subscriptionAccess={access}
          selectionPending={selectionPending}
          selectionPresentation={selectionPresentation}
          effort={effort}
          effortSnapshot={effortSnapshot}
          entryMode={entryMode}
          unmatchedLabel={unmatchedLabel}
          onPick={(ref) => { picked = ref; }}
          onPickEffort={() => {}}
        />
      </LocaleProvider>,
    );
  });
};
const trigger = () => rootElement.querySelector(".modelsw__trigger") as HTMLButtonElement;
const openModelPanel = async () => {
  await act(async () => trigger().click());
  const modelNav = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__nav-item")]
    .find((button) => button.textContent?.includes("模型"));
  await act(async () => modelNav?.click());
};

console.log("\nmodel switcher shared catalog state");

const switcherSource = readFileSync(new URL("../components/ModelSwitcher.tsx", import.meta.url), "utf8");
ok(
  !switcherSource.includes("ModelsForTab")
    && !switcherSource.includes("ModelMarketplace()")
    && !switcherSource.includes("catalogRefreshSignal")
    && !switcherSource.includes('from "../lib/bridge"')
    && switcherSource.includes("groupModelInfosByProvider(filtered, providerLabels)"),
  "the switcher is a pure shared-catalog consumer with no bridge or refresh-key fallback",
);

await render({ state: "loading" });
ok(
  trigger().disabled
    && trigger().getAttribute("aria-busy") === "true"
    && trigger().textContent?.includes("pro-model") === true,
  "loading clears candidates while preserving the saved identity in the disabled trigger",
);

await render({ state: "ready", routes, marketplace });
ok(!trigger().disabled && trigger().textContent?.includes("pro-model") === true, "ready exposes the filtered shared routes");
await openModelPanel();
ok(document.body.textContent?.includes("free-model") === false, "Pro access filters the same shared snapshot");
await act(async () => trigger().click());

await render({ state: "ready", routes, marketplace }, freeAccess, "kwu/free-model");
ok(trigger().textContent?.includes("free-model") === true, "an access change re-filters without loading catalog data");

picked = "";
await render({ state: "ready", routes, marketplace }, { state: "unavailable" }, "kwu/pro-model");
ok(
  trigger().textContent?.includes("请选择模型") === true
    && trigger().textContent?.includes("pro-model") === false
    && !trigger().disabled,
  "signed-out access does not expose the current subscription route in the trigger",
);
await openModelPanel();
let visibleItems = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")];
ok(
  visibleItems.every((item) => !item.textContent?.includes("pro-model"))
    && visibleItems.some((item) => item.textContent?.includes("custom/local")),
  "signed-out access hides subscription routes while preserving BYOK",
);
ok(picked === "", "an access change never invokes the model picker implicitly");
await act(async () => trigger().click());

await render(
  { state: "ready", routes: [], marketplace },
  { state: "unavailable" },
  "kwu/pro-model",
  false,
  "subscription_pool",
  "auto",
  undefined,
  undefined,
  "root",
  "Auto",
);
ok(
  trigger().disabled
    && trigger().textContent?.includes("暂无可选模型") === true
    && trigger().textContent?.includes("pro-model") === false
    && trigger().textContent?.includes("Auto") === false,
  "a clean signed-out empty catalog exposes no saved subscription label",
);

await render(
  { state: "ready", routes: [], marketplace },
  { state: "unavailable" },
  "kwu/deepseek-v4-flash",
  false,
  null,
);
ok(
  trigger().disabled
    && trigger().textContent?.includes("暂无可选模型") === true
    && trigger().textContent?.includes("deepseek-v4-flash") === false,
  "a clean signed-out catalog hides a saved official ref when its billing source is undefined",
);

await render(
  { state: "ready", routes, marketplace },
  { state: "unavailable" },
  "kwu/not-in-catalog",
  false,
  "subscription_pool",
  "auto",
  undefined,
  undefined,
  "root",
  "Auto",
);
ok(
  trigger().textContent?.includes("请选择模型") === true
    && trigger().textContent?.includes("not-in-catalog") === false
    && trigger().textContent?.includes("Auto") === false,
  "signed-out access does not expose an unresolved saved subscription ref",
);

await render(
  { state: "ready", routes, marketplace },
  { state: "unavailable" },
  "custom/local",
  false,
  "newapi_byok",
);
ok(
  trigger().textContent?.includes("custom/local") === true && !trigger().disabled,
  "signed-out access preserves an allowed BYOK current route",
);

await render({ state: "ready", routes }, proAccess, "kwu/pro-model");
await openModelPanel();
visibleItems = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")];
ok(
  visibleItems.every((item) => !item.textContent?.includes("pro-model"))
    && visibleItems.some((item) => item.textContent?.includes("custom/local")),
  "a ready catalog without marketplace evidence fails subscription routes closed and preserves BYOK",
);
await act(async () => trigger().click());

await render({ state: "ready", routes: [], marketplace }, proAccess, "");
ok(trigger().disabled && trigger().textContent?.includes("暂无可选模型") === true, "a successful empty catalog has the normal empty state");

await render({ state: "unavailable" }, proAccess, "");
ok(
  trigger().disabled
    && trigger().getAttribute("aria-busy") === null
    && trigger().textContent?.includes("模型配置暂不可用") === true,
  "an unavailable catalog is distinct from loading and a successful empty list",
);

const identityRoutes: ModelInfo[] = [
  { ref: "zeta/z", provider: "zeta", model: "z", billingSource: "newapi_byok", current: true },
  { ref: "duplicate", provider: "custom", model: "duplicate", billingSource: "newapi_byok", current: false },
  { ref: "duplicate", provider: "kwu", model: "duplicate", billingSource: "subscription_pool", current: false },
  { ref: "kwu/pro-model", provider: "kwu", model: "pro-model", billingSource: "subscription_pool", current: false },
  { ref: "alpha/a", provider: "alpha", model: "a", billingSource: "newapi_byok", current: false },
];
const identityCatalog: SharedModelCatalogState = { state: "ready", routes: identityRoutes, marketplace };

await render(identityCatalog, proAccess, "zeta/z", false, "newapi_byok");
ok(trigger().textContent?.includes("zeta/z") === true, "a unique authoritative current ref resolves without parsing human labels");
await render(identityCatalog, proAccess, "duplicate", false, "newapi_byok");
ok(trigger().textContent?.includes("custom/duplicate") === true, "duplicate refs resolve the BYOK route by billing source");
await render(identityCatalog, proAccess, "duplicate", false, "subscription_pool");
ok(trigger().textContent?.includes("云智企办/duplicate") === true, "duplicate refs resolve the subscription route by billing source");
await render(identityCatalog, proAccess, "missing/model", false, "subscription_pool", "auto", undefined, undefined, "root", "Auto");
ok(
  trigger().textContent?.includes("missing/model") === true
    && trigger().textContent?.includes("Auto") === false,
  "a nonempty unresolved ref wins over unmatched Auto and is not replaced by an unrelated current flag",
);
await render(identityCatalog, proAccess, "", false, "subscription_pool", "auto", undefined, undefined, "root", "Auto");
ok(trigger().textContent?.includes("Auto") === true, "an empty ref displays the explicit unmatched Auto label");
await openModelPanel();
const groupLabels = [...document.body.querySelectorAll(".modelsw__group-label")].map((node) => node.textContent ?? "");
ok(groupLabels[0]?.includes("云智企办") === true, "a provider containing subscription routes sorts first");
const remainingGroups = groupLabels.slice(1).join("|");
ok(
  remainingGroups.indexOf("alpha") < remainingGroups.indexOf("custom")
    && remainingGroups.indexOf("custom") < remainingGroups.indexOf("zeta"),
  "remaining providers use stable label ordering",
);
const itemOrder = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")]
  .map((item) => item.getAttribute("aria-label"));
ok(
  JSON.stringify(itemOrder) === JSON.stringify([
    "云智企办/duplicate",
    "云智企办/pro-model",
    "alpha/a",
    "custom/duplicate",
    "zeta/z",
  ]),
  "the switcher preserves shared provider ordering and original order within each provider",
);
const duplicateItem = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")]
  .find((item) => item.textContent?.includes("custom/duplicate"));
ok(
  duplicateItem?.getAttribute("title")?.includes("custom/duplicate") === true
    && duplicateItem?.getAttribute("aria-label")?.includes("custom/duplicate") === true,
  "model rows retain their full title and accessible label",
);
const alphaItem = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")]
  .find((item) => item.textContent?.includes("alpha/a"));
picked = "";
await act(async () => alphaItem?.click());
ok(picked === "alpha/a", "an explicit selection invokes only the picker callback in a bridge-free environment");

await render({ state: "ready", routes, marketplace }, proAccess, "kwu/pro-model", true);
ok(
  trigger().disabled
    && trigger().getAttribute("aria-busy") === "true"
    && trigger().textContent?.includes("pro-model") === true,
  "selection pending preserves the confirmed identity while disabling the selector",
);

await render({ state: "loading" }, proAccess, "kwu/pro-model", false, "subscription_pool", "pending");
const pendingLabel = trigger().querySelector(".modelsw__label");
ok(
  trigger().disabled
    && trigger().getAttribute("aria-busy") === "true"
    && pendingLabel?.classList.contains("modelsw__label--pending") === true
    && pendingLabel.textContent === "",
  "an unknown target entitlement keeps the fixed selector shell without exposing any provisional label",
);

await render({ state: "loading" }, proAccess, "kwu/pro-model", true, "subscription_pool", "auto", "high");
ok(
  trigger().disabled
    && trigger().getAttribute("aria-busy") === "true"
    && trigger().textContent?.includes("pro-model") === true
    && trigger().textContent?.includes("高") === true
    && document.body.querySelectorAll(".modelsw__effort-item").length === 0,
  "a reliable existing-session snapshot stays visible while busy without exposing effort controls",
);

await render({ state: "loading" }, proAccess, "kwu/pro-model", true, "subscription_pool", "auto", "");
ok(
  trigger().textContent?.includes("自动") === true,
  "a known empty persisted effort remains distinct from an unknown snapshot and displays auto",
);

await render(
  { state: "loading" },
  proAccess,
  "kwu/pro-model",
  true,
  "subscription_pool",
  "auto",
  "high",
  { supported: true, current: "", default: "", levels: ["low", "high"] },
);
ok(
  trigger().textContent?.includes("自动") === true
    && trigger().textContent?.includes("高") === false,
  "authoritative empty effort displays auto instead of a stale persisted snapshot while busy",
);

await render({ state: "loading" }, proAccess, "kwu/pro-model", true, "subscription_pool", "choose");
ok(
  trigger().disabled
    && trigger().textContent?.includes("请选择模型") === true
    && trigger().textContent?.includes("pro-model") === false,
  "a definitive entitlement denial shows only the final choose-model label while facts settle",
);

picked = "";
await render(
  { state: "ready", routes, marketplace },
  proAccess,
  "kwu/pro-model",
  false,
  "subscription_pool",
  "auto",
  undefined,
  undefined,
  "models",
);
await act(async () => trigger().click());
ok(Boolean(document.body.querySelector(".modelsw__search-input")), "direct model entry opens the searchable model list on the first click");
ok(document.body.querySelectorAll(".modelsw__root-panel").length === 0, "direct model entry skips the intermediate root menu");
const directModel = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")]
  .find((item) => item.textContent?.includes("custom/local"));
await act(async () => directModel?.click());
ok(picked === "custom/local", "direct model entry completes selection without a second navigation step");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
