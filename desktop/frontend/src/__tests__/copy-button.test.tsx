// Run: tsx src/__tests__/copy-button.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { CopyButton } from "../components/CopyButton";
import { LocaleProvider } from "../lib/i18n";
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

function flushTimers(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

console.log("\ncopy button");

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
Object.defineProperty(dom.window.navigator, "clipboard", {
  configurable: true,
  value: { writeText: async () => { throw new Error("clipboard denied"); } },
});
Object.defineProperty(dom.window, "runtime", {
  configurable: true,
  value: { ClipboardSetText: async () => false },
});
Object.defineProperty(dom.window.document, "execCommand", {
  configurable: true,
  value: () => false,
});

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <ToastProvider>
        <CopyButton text="copy me" />
      </ToastProvider>
    </LocaleProvider>,
  );
  await flushTimers();
});

const button = document.querySelector<HTMLButtonElement>(".copybtn");
if (!button) throw new Error("copy button did not render");
await act(async () => {
  button.click();
  await flushTimers();
  await flushTimers();
});

ok(!button.classList.contains("copybtn--copied"), "failed clipboard write does not show copied state");
ok(button.getAttribute("aria-label") === "复制", "failed clipboard write keeps the copy action label");
ok(document.querySelector('[role="alert"]')?.textContent?.includes("复制失败") === true, "failed clipboard write shows a visible error");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
