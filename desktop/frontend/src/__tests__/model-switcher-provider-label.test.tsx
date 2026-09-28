// Run: tsx src/__tests__/model-switcher-provider-label.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { ModelSwitcher } from "../components/ModelSwitcher";
import { LocaleProvider } from "../lib/i18n";
import type { SharedModelCatalogState } from "../lib/types";

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

const catalog: SharedModelCatalogState = {
  state: "ready",
  routes: [
    { ref: "kwu/gpt-5.5", provider: "kwu", model: "gpt-5.5", billingSource: "subscription_pool", current: true },
    { ref: "kwu/gpt-5.4-mini", provider: "airclaude_subscription", model: "gpt-5.4-mini", billingSource: "subscription_pool", current: false },
    { ref: "custom/gpt-image-2", provider: "custom", model: "gpt-image-2", billingSource: "newapi_byok", current: false },
  ],
  marketplace: {
    sources: [{
      id: "international",
      label: "International",
      sourceUrl: "",
      pricingUrl: "",
      pricingState: "fresh",
      metadataStatus: "ready",
      models: [
        { id: "gpt-5.5", name: "gpt-5.5", tags: ["Subscribe_Pro"] },
        { id: "gpt-5.4-mini", name: "gpt-5.4-mini", tags: ["Subscribe_Pro"] },
      ],
    }],
  },
};

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);
let picked = "";
let pickedEffort = "";
await act(async () => {
  root.render(
    <LocaleProvider>
      <ModelSwitcher
        label="kwu/gpt-5.5"
        modelCatalog={catalog}
        currentBillingSource="subscription_pool"
        subscriptionAccess={{ state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" }}
        effort={{ supported: true, current: "auto", default: "auto", levels: ["auto", "low", "medium", "high"] }}
        onPick={(ref) => { picked = ref; }}
        onPickEffort={(level) => { pickedEffort = level; }}
      />
    </LocaleProvider>,
  );
});

const trigger = rootEl.querySelector(".modelsw__trigger") as HTMLButtonElement;
ok(trigger.textContent?.includes("自动") === true, "compact model trigger includes the localized current effort");
await act(async () => trigger.click());
let text = document.body.textContent ?? "";
ok(text.includes("模型") && text.includes("推理强度") && text.includes("自动"), "first panel keeps model and effort as two compact navigation rows");
const effortNav = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__nav-item")]
  .find((button) => button.textContent?.includes("推理强度"));
await act(async () => effortNav?.click());
text = document.body.textContent ?? "";
ok(text.includes("自动") && text.includes("低") && text.includes("中") && text.includes("高"), "effort subpanel exposes localized choices");
const autoEffortButton = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__effort-item")]
  .find((button) => button.textContent?.trim() === "自动");
ok(autoEffortButton?.title === "使用模型或服务商的默认推理强度", "automatic effort explains its provider-default semantics without expanding the composer");
const highEffortButton = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__effort-item")]
  .find((button) => button.textContent?.trim() === "高");
await act(async () => highEffortButton?.click());
ok(pickedEffort === "high", "effort subpanel reuses the existing session effort callback");

await act(async () => trigger.click());
const modelNav = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__nav-item")]
  .find((button) => button.textContent?.includes("模型"));
await act(async () => modelNav?.click());
text = document.body.textContent ?? "";
ok(text.includes("云智企办") && !text.includes("Clouvia Official"), "KWU routes use the localized product provider label");
ok(text.includes("云智企办/gpt-5.5") && text.includes("云智企办/gpt-5.4-mini"), "platform route labels keep their short provider prefix");
ok(text.includes("custom/gpt-image-2"), "configured custom routes remain visible");
ok(text.includes("平台积分"), "subscription routes retain their billing-source metadata");
const imageButton = [...document.body.querySelectorAll<HTMLButtonElement>(".modelsw__item")]
  .find((button) => button.textContent?.includes("gpt-image-2"));
await act(async () => imageButton?.click());
ok(picked === "custom/gpt-image-2", "provider presentation does not change selection behavior");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
