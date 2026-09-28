// Run: tsx src/__tests__/agent-workspace.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { AgentWorkspace } from "../components/AgentWorkspace";
import { LocaleProvider } from "../lib/i18n";
import type { AppBindings } from "../lib/bridge";
import type { Item } from "../lib/useController";

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
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  globalThis.ResizeObserver = TestResizeObserver;
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

function mockApp(methods: Partial<AppBindings>) {
  window.go = {
    main: {
      App: {
        ToolResultForTab: async () => null,
        ...methods,
      } as Partial<AppBindings> as AppBindings,
    },
  };
}

async function renderWorkspace(items: Extract<Item, { kind: "tool" }>[]) {
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AgentWorkspace items={items} subcalls={new Map()} tabId="tab-1" />
      </LocaleProvider>,
    );
    await flushTimers();
  });
  return root;
}

console.log("\nagent workspace");

{
  const dom = installDom();
  mockApp({
    ToolResultForTab: async (_tabID, toolID) =>
      toolID === "task-1"
        ? { args: "{\"description\":\"Audit checkout flow\"}", output: "Archived subagent answer" }
        : null,
  });
  const root = await renderWorkspace([
    {
      kind: "tool",
      id: "task-1",
      name: "task",
      args: "{\"description\":\"Audit checkout flow\"}",
      readOnly: false,
      status: "done",
      subject: "Audit checkout flow",
      summary: "Found two issues",
      dataArchived: true,
    },
  ]);

  const expertTab = document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab")[1];
  if (!expertTab) throw new Error("expert tab did not render");
  await act(async () => {
    expertTab.click();
    await flushTimers();
  });

  const activeView = document.querySelector<HTMLElement>(".agent-workspace__view[data-view=\"focused\"]");
  ok(Boolean(activeView), "clicking a sub-expert avatar opens the focused view");
  ok((activeView?.textContent ?? "").trim().length > 0, "focused view is not blank for completed archived sub-expert tasks");
  ok(activeView?.textContent?.includes("Archived subagent answer") === true, "focused view restores archived sub-expert output as chat content");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
