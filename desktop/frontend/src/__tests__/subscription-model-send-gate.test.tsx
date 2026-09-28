// Run: tsx src/__tests__/subscription-model-send-gate.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";

import { Composer } from "../components/Composer";
import { SubscriptionModelGateBanner } from "../components/SubscriptionModelGateBanner";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionModelInvocationAccess } from "../lib/subscriptionModelAccess";
import { ToastProvider } from "../lib/toast";

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

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

function installDom() {
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
  globalThis.HTMLTextAreaElement = dom.window.HTMLTextAreaElement;
  globalThis.Event = dom.window.Event;
  globalThis.CustomEvent = dom.window.CustomEvent;
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.InputEvent = dom.window.InputEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.File = dom.window.File;
  globalThis.FileReader = dom.window.FileReader;
  globalThis.PointerEvent = dom.window.MouseEvent as unknown as typeof PointerEvent;
  globalThis.MutationObserver = dom.window.MutationObserver;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  globalThis.ResizeObserver = TestResizeObserver;
  Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
  Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
  Object.defineProperty(dom.window, "matchMedia", {
    configurable: true,
    value: () => ({
      matches: true,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }),
  });
  (dom.window as unknown as { go: { main: { App: Record<string, unknown> } } }).go = {
    main: {
      App: {
        Commands: async () => [],
        Models: async () => [],
        ModelsForTab: async () => [
          {
            ref: "custom/test-model",
            provider: "custom",
            model: "test-model",
            billingSource: "newapi_byok",
            current: true,
            selectable: true,
          },
          {
            ref: "kwu/qwen3.7-plus",
            provider: "airclaude_subscription",
            model: "qwen3.7-plus",
            billingSource: "subscription_pool",
            current: false,
            selectable: true,
          },
        ],
        ModelMarketplace: async () => ({
          sources: [{
            id: "international",
            models: [{ name: "qwen3.7-plus", tags: ["Subscribe_Free"] }],
          }],
        }),
      },
    },
  };
}

function setEditorValue(node: HTMLDivElement, value: string) {
  node.replaceChildren(document.createTextNode(value));
  node.dispatchEvent(new Event("input", { bubbles: true }));
}

function pressEnter(node: HTMLDivElement) {
  node.dispatchEvent(new KeyboardEvent("keydown", {
    key: "Enter",
    code: "Enter",
    keyCode: 13,
    which: 13,
    bubbles: true,
    cancelable: true,
  }));
}

console.log("\nsubscription model send gate interaction");

installDom();
const container = document.getElementById("root")!;
const root = createRoot(container);
let sends = 0;
let switches = 0;

async function render(access: SubscriptionModelInvocationAccess) {
  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToastProvider>
          <SubscriptionModelGateBanner
            access={access}
            modelCatalog={{
              state: "ready",
              routes: [{
                ref: "custom/test-model",
                provider: "custom",
                model: "test-model",
                billingSource: "newapi_byok",
                current: false,
              }],
            }}
            subscriptionAccess={{ state: "ready", plan: "free", requiredTag: "Subscribe_Free" }}
            onSwitchModel={() => { switches += 1; }}
          />
          <Composer
            running={false}
            collaborationMode="normal"
            toolApprovalMode="ask"
            tokenMode="full"
            goal=""
            cwd="/repo"
            modelLabel={access.state === "allowed" ? "BYOK model" : "Subscription model unavailable"}
            tabId="gate-tab"
            modelCatalog={{
              state: "ready",
              routes: [{
                ref: "custom/test-model",
                provider: "custom",
                model: "test-model",
                billingSource: "newapi_byok",
                current: true,
              }],
            }}
            sessionKey="gate-session"
            onSend={() => { sends += 1; }}
            onCancel={() => undefined}
            onCycleMode={() => undefined}
            onSetMode={() => undefined}
            onSetCollaborationMode={() => undefined}
            onSetToolApprovalMode={() => undefined}
            onToggleYoloApprovalMode={() => undefined}
            onClearGoal={() => undefined}
            onSwitchModel={() => undefined}
            onSetEffort={() => undefined}
            onSetTokenMode={() => undefined}
            submitDisabled={access.state !== "allowed"}
            submitDisabledReason={access.state === "loading" ? "Verifying model access. Please wait." : undefined}
            ready={access.state === "allowed"}
          />
        </ToastProvider>
      </LocaleProvider>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

await render({ state: "blocked" });
const blockedEditor = container.querySelector("#composer-input") as HTMLDivElement;
const blockedSend = container.querySelector(".composer__btn--send") as HTMLButtonElement;
ok(blockedSend.disabled, "unauthorized subscription disables the real Composer send button");
await act(async () => {
  setEditorValue(blockedEditor, "must not send");
  pressEnter(blockedEditor);
  blockedSend.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
ok(sends === 0, "unauthorized subscription blocks both Enter and button submission");
const switchButton = container.querySelector("[data-subscription-model-gate] button") as HTMLButtonElement;
await act(async () => switchButton.click());
ok(switches === 1, "gate banner exposes an interactive switch-model recovery action");

await render({ state: "loading" });
const loadingEditor = container.querySelector("#composer-input") as HTMLDivElement;
const loadingSend = container.querySelector(".composer__btn--send") as HTMLButtonElement;
const modelTrigger = container.querySelector(".modelsw__trigger") as HTMLButtonElement;
ok(loadingEditor.getAttribute("contenteditable") === "true", "permission verification keeps the composer input editable");
ok(loadingSend.disabled, "permission verification keeps sending blocked");
ok(!modelTrigger.disabled, "permission verification keeps the model switcher interactive");
ok(container.querySelector("[data-subscription-model-gate]") === null, "permission verification does not mount the model gate banner");
ok(
  container.textContent?.includes("Checking whether this subscription model is available") === false,
  "permission verification keeps the loading banner copy out of the DOM",
);
const sendTooltipTrigger = loadingSend.closest(".tooltip-trigger") as HTMLElement;
await act(async () => {
  sendTooltipTrigger.focus();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
ok(document.body.textContent?.includes("Verifying model access. Please wait.") === true, "disabled send control explains permission verification on focus");
await act(async () => {
  setEditorValue(loadingEditor, "still must not send");
  pressEnter(loadingEditor);
  loadingSend.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
});
ok(sends === 0, "permission verification blocks both Enter and button submission");

await render({ state: "allowed" });
const byokEditor = container.querySelector("#composer-input") as HTMLDivElement;
const allowedSend = container.querySelector(".composer__btn--send") as HTMLButtonElement;
const allowedSendTooltipTrigger = allowedSend.closest(".tooltip-trigger") as HTMLElement;
ok(allowedSendTooltipTrigger.getAttribute("tabindex") === null, "ordinary send tooltip does not add a duplicate tab stop");
ok(document.body.textContent?.includes("Verifying model access. Please wait.") === false, "settled access immediately restores the ordinary send tooltip");
await act(async () => {
  setEditorValue(byokEditor, "BYOK remains available");
  pressEnter(byokEditor);
  await new Promise((resolve) => setTimeout(resolve, 0));
});
ok(sends === 1, "allowed BYOK route still submits through the real Composer Enter interaction");
ok(container.querySelector("[data-subscription-model-gate]") === null, "allowed route removes the gate banner");

await act(async () => root.unmount());

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
