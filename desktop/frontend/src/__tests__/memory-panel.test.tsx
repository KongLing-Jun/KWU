// Run: tsx src/__tests__/memory-panel.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { MemoryPanel, MemorySettingsPage } from "../components/MemoryPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { MemoryView } from "../lib/types";

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

function flushTimers(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
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
  globalThis.HTMLInputElement = dom.window.HTMLInputElement;
  globalThis.Event = dom.window.Event;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.PointerEvent = dom.window.MouseEvent as unknown as typeof PointerEvent;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });
  Object.defineProperty(window, "matchMedia", {
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
  return dom;
}

const memoryView: MemoryView = {
  available: true,
  docs: [{ path: "/repo/KWU.md", scope: "project", body: "project instructions" }],
  facts: [
    {
      name: "source",
      title: "Source memory",
      description: "source description",
      type: "project",
      body: "source body links to [[target]]",
    },
    {
      name: "target",
      title: "Target memory",
      description: "target description",
      type: "reference",
      body: "target body",
    },
  ],
  archives: [],
  scopes: [{ scope: "project", path: "/repo/KWU.md" }],
  storeDir: "/repo/.kwu/memory",
};

async function renderMemoryPanel() {
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <MemoryPanel
          view={memoryView}
          onClose={() => {}}
          onRemember={() => {}}
          onForget={() => {}}
          onSaveDoc={() => {}}
        />
      </LocaleProvider>,
    );
    await flushTimers();
  });
  return root;
}

async function flushAsyncWork(): Promise<void> {
  await flushTimers();
  await flushTimers();
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(input.ownerDocument.defaultView?.HTMLInputElement.prototype ?? HTMLInputElement.prototype, "value")?.set;
  if (setter) setter.call(input, value);
  else input.value = value;
  const reactPropsKey = Object.keys(input).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (input as unknown as Record<string, { onChange?: (event: { target: HTMLInputElement; currentTarget: HTMLInputElement }) => void }>)[reactPropsKey]
    : undefined;
  reactProps?.onChange?.({ target: input, currentTarget: input });
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

console.log("\nmemory panel");

{
  const dom = installDom();
  const root = await renderMemoryPanel();
  const savedSections = Array.from(document.querySelectorAll(".mem-section__title"))
    .filter((node) => node.textContent === "已保存的记忆");
  ok(savedSections.length === 1, "memory drawer renders saved memories once");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  const root = await renderMemoryPanel();
  const filter = Array.from(document.querySelectorAll("input"))
    .find((input) => input.getAttribute("aria-label") === "筛选指令文件与记忆…") as HTMLInputElement | undefined;
  if (!filter) throw new Error("memory filter input did not render");
  await act(async () => {
    setInputValue(filter, "source");
    await flushTimers();
  });
  ok(!document.body.textContent?.includes("Target memory"), "filter hides target memory before link jump");

  const sourceSummary = Array.from(document.querySelectorAll(".mem-fact__summary"))
    .find((button) => button.textContent?.includes("Source memory")) as HTMLButtonElement | undefined;
  if (!sourceSummary) throw new Error("source memory summary did not render");
  await act(async () => {
    sourceSummary.click();
    await flushTimers();
  });
  const targetLink = Array.from(document.querySelectorAll(".mem-link"))
    .find((button) => button.textContent === "target") as HTMLButtonElement | undefined;
  if (!targetLink) throw new Error("target memory link did not render");
  await act(async () => {
    targetLink.click();
    await flushTimers();
  });
  ok(document.body.textContent?.includes("Target memory") === true, "[[link]] jump clears filters that hide the target memory");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  let memoryCalls = 0;
  window.go = {
    main: {
      App: {
        ListTabs: async () => [],
        Memory: async () => {
          memoryCalls += 1;
          if (memoryCalls === 1) throw new Error("temporary memory load failure");
          return memoryView;
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
        <MemorySettingsPage />
      </LocaleProvider>,
    );
    await flushAsyncWork();
  });
  ok(document.body.textContent?.includes("记忆操作失败，请稍后重试。") === true, "memory settings exposes the Chinese recovery message");
  ok(document.body.textContent?.includes("temporary memory load failure") === false, "memory settings hides the raw backend failure");
  const retry = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.trim() === "重试") as HTMLButtonElement | undefined;
  if (!retry) throw new Error("memory retry button did not render");
  await act(async () => {
    retry.click();
    await flushAsyncWork();
  });
  ok(document.body.textContent?.includes("Source memory") === true && memoryCalls === 2, "memory settings retries a failed load");
  await act(async () => root.unmount());
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
