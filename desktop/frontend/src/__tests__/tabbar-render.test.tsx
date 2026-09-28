// Run: tsx src/__tests__/tabbar-render.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { TabBar } from "../components/TabBar";
import { LocaleProvider } from "../lib/i18n";
import type { TabMeta } from "../lib/types";

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
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.Event = dom.window.Event;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });
  return dom;
}

console.log("\ntabbar rendered behavior");

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const closed: string[] = [];
  const tabs: TabMeta[] = [
    {
      id: "tab-a",
      label: "test",
      ready: true,
      running: false,
      active: true,
      scope: "project",
      workspaceRoot: "/repo",
      workspaceName: "Repo",
      topicId: "topic-a",
      topicTitle: "Topic A",
      cwd: "/repo",
      mode: "normal",
    },
  ];

  await act(async () => {
    root.render(
      <LocaleProvider>
        <TabBar
          tabs={tabs}
          activeTabId="tab-a"
          onTabChange={() => {}}
          onTabClose={(tabId) => closed.push(tabId)}
          onTabsClose={() => {}}
          onTabsReorder={() => {}}
          onNewTab={() => {}}
        />
      </LocaleProvider>,
    );
    await flushTimers();
  });

  const closeButton = document.querySelector(".tabbar__tab-close") as HTMLButtonElement | null;
  ok(closeButton?.tagName === "BUTTON", "tab close control renders as a button");
  ok(closeButton?.getAttribute("aria-label") === "关闭标签页", "tab close button exposes an accessible name");
  await act(async () => {
    closeButton?.click();
    await flushTimers();
  });
  ok(closed[0] === "tab-a", "tab close button closes its owning tab");
  await act(async () => root.unmount());
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
