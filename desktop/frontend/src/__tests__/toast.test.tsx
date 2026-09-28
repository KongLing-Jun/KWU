// Run: tsx src/__tests__/toast.test.tsx

import { JSDOM } from "jsdom";
import React, { useEffect } from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { ToastProvider, useToast } from "../lib/toast";

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

async function wait(ms = 0) {
  await new Promise((resolve) => setTimeout(resolve, ms));
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
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  return dom;
}

console.log("\ntoast lifecycle");

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  function Trigger() {
    const { showToast } = useToast();
    useEffect(() => {
      showToast("腾讯文档已连接", "success");
    }, [showToast]);
    return null;
  }

  await act(async () => {
    root.render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    await wait();
  });

  const successToast = document.querySelector<HTMLElement>(".toast--success");
  ok(successToast?.textContent === "腾讯文档已连接", "成功 Toast 使用简短结果文案");
  ok(Boolean(successToast?.querySelector("svg")), "成功 Toast 使用统一矢量勾选图标");
  ok(successToast?.getAttribute("role") === "status", "成功 Toast 使用非打断式状态语义");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const originalConsoleError = console.error;
  console.error = () => {};

  function Trigger() {
    const { showError } = useToast();
    useEffect(() => {
      showError("toast.test", new Error("raw backend detail"), "保存失败，请稍后重试。", { durationMs: 2500 });
    }, [showError]);
    return null;
  }

  await act(async () => {
    root.render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    await wait();
  });

  const errorToast = document.querySelector<HTMLElement>(".toast--error");
  ok(errorToast?.textContent?.includes("保存失败，请稍后重试。") === true, "统一错误 Toast 显示中文兜底文案");
  ok(errorToast?.textContent?.includes("raw backend detail") === false, "统一错误 Toast 不暴露原始异常");
  ok(errorToast?.getAttribute("role") === "alert", "错误 Toast 使用 alert 语义");
  ok(errorToast?.getAttribute("aria-live") === "assertive", "错误 Toast 使用即时播报");

  await act(async () => root.unmount());
  console.error = originalConsoleError;
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let dismissed = 0;

  function Trigger() {
    const { showToast } = useToast();
    useEffect(() => {
      showToast("Clear expert failed", "error", { durationMs: 5, onDismiss: () => { dismissed += 1; } });
    }, [showToast]);
    return null;
  }

  await act(async () => {
    root.render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    await wait();
  });

  ok(dismissed === 0, "toast dismiss callback is not called while the toast is visible");

  await act(async () => {
    await wait(20);
  });

  ok(dismissed === 1, "toast dismiss callback runs after the toast auto-dismisses");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let dismissed = 0;

  function Trigger() {
    const { showToast } = useToast();
    useEffect(() => {
      showToast("Click to dismiss", "warn", { durationMs: 2500, onDismiss: () => { dismissed += 1; } });
    }, [showToast]);
    return null;
  }

  await act(async () => {
    root.render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    await wait();
  });

  const toast = document.querySelector<HTMLElement>(".toast");
  if (!toast) throw new Error("toast did not render");

  await act(async () => {
    toast.click();
    await wait();
  });

  ok(dismissed === 1, "toast dismiss callback runs when the toast is clicked away");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let actions = 0;

  function Trigger() {
    const { showToast } = useToast();
    useEffect(() => {
      showToast("Default model adjusted", "info", {
        durationMs: 2500,
        actionLabel: "Open model settings",
        onAction: () => {
          actions += 1;
        },
      });
    }, [showToast]);
    return null;
  }

  await act(async () => {
    root.render(
      <ToastProvider>
        <Trigger />
      </ToastProvider>,
    );
    await wait();
  });

  const action = document.querySelector<HTMLButtonElement>(".toast__action");
  ok(action?.textContent === "Open model settings", "toast renders its explicit action label");
  await act(async () => {
    action?.click();
    await wait();
  });
  ok(actions === 1, "toast action invokes its callback exactly once");
  ok(document.querySelector(".toast") === null, "toast action dismisses the completed notification");

  await act(async () => root.unmount());
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
