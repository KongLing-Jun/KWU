import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";

import { LocaleProvider } from "../lib/i18n";
import { ToastProvider, useToast } from "../lib/toast";

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", { url: "http://localhost" });
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.IS_REACT_ACT_ENVIRONMENT = true;

let copied = "";
Object.defineProperty(globalThis.navigator, "clipboard", {
  configurable: true,
  value: { writeText: async (value: string) => { copied = value; } },
});

function Harness() {
  const { showError } = useToast();
  return (
    <button
      type="button"
      onClick={() => showError(
        "account.operation",
        new Error("CLOUD_ERROR: private detail [diagnostic_id=log-toast-001]"),
        "操作失败，请稍后重试。",
        { durationMs: 60_000 },
      )}
    >
      trigger
    </button>
  );
}

const host = document.getElementById("root");
if (!host) throw new Error("missing root");
const root = createRoot(host);
await act(async () => {
  root.render(<LocaleProvider><ToastProvider><Harness /></ToastProvider></LocaleProvider>);
});
await act(async () => {
  host.querySelector<HTMLButtonElement>("button")?.click();
});

const toast = host.querySelector<HTMLElement>(".toast");
if (!toast) throw new Error("missing error toast");
if (toast.querySelector(".toast__text")?.textContent !== "操作失败，请稍后重试。") {
  throw new Error(`toast user message was rewritten: ${toast.textContent}`);
}
if (toast.querySelector(".toast__diagnostic code")?.textContent !== "log-toast-001") {
  throw new Error(`toast Log ID was not rendered separately: ${toast.textContent}`);
}
await act(async () => {
  toast.querySelector<HTMLButtonElement>(".copybtn")?.click();
  await Promise.resolve();
});
if (copied !== "log-toast-001") throw new Error(`copied ${JSON.stringify(copied)}`);

await act(async () => toast.click());
await act(async () => root.unmount());
dom.window.close();
