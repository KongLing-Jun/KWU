// Run: tsx src/__tests__/automation-pagination.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import type { AutomationRunPage } from "../features/automation/heartbeat/heartbeat.types";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";

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

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
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
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};
Object.defineProperty(globalThis.window, "matchMedia", {
  configurable: true,
  value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
});
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });

const requests: Array<{ filter: Record<string, unknown>; cursor: string }> = [];
const automationEvents = new Map<string, (...payload: unknown[]) => void>();
let taskListCalls = 0;
let catalogCalls = 0;
let deliveryTargetCalls = 0;
const pages: Record<string, AutomationRunPage> = {
  "": {
    items: [{ runId: "run-new", taskId: "task-a", taskName: "最新记录", status: "succeeded", startedAt: "2026-08-01T10:00:00Z" }],
    nextCursor: "cursor-2",
    hasMore: true,
  },
  "cursor-2": {
    items: [{ runId: "run-old", taskId: "task-b", taskName: "较早记录", status: "failed", startedAt: "2026-08-01T09:00:00Z" }],
    hasMore: false,
  },
};

window.runtime = {
  EventsOn: (name, callback) => {
    automationEvents.set(name, callback);
    return () => { automationEvents.delete(name); };
  },
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => {
        taskListCalls += 1;
        return [{
        id: "task-next",
        name: "后端排期任务",
        enabled: true,
        prompt: { prompt: "run" },
        schedule: { kind: "interval", every: "30m" },
        nextRunAt: "2099-01-02T01:00:00Z",
        }];
      },
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 1, limit: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
      AutomationListRuns: async () => [],
      AutomationListTemplates: async () => [],
      AutomationGetScheduleCatalog: async () => {
        catalogCalls += 1;
        return [];
      },
      AutomationListDeliveryTargets: async () => {
        deliveryTargetCalls += 1;
        return [];
      },
      AutomationListRunsV2: async (filter: Record<string, unknown>, cursor: string) => {
        requests.push({ filter, cursor });
        return pages[cursor] || { items: [], hasMore: false };
      },
      ListWorkspaces: async () => [],
    } as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation run pagination");
const host = document.getElementById("root")!;
const root = createRoot(host);
const access: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
await act(async () => {
  root.render(
    <LocaleProvider>
      <HeartbeatPanel open embedded authenticated subscriptionAccess={access} onClose={() => {}} onOpenTopic={() => {}} />
    </LocaleProvider>,
  );
});

await waitFor("run history tab", () => host.querySelectorAll<HTMLButtonElement>('[role="tab"]').length === 2);
await waitFor("backend next run", () => host.textContent?.includes("1/2") === true);
ok(host.textContent?.includes("后端排期任务") === true, "scheduled task renders the backend-enriched task DTO");
await act(async () => {
  host.querySelectorAll<HTMLButtonElement>('[role="tab"]')[1]?.click();
});
await waitFor("first cursor page", () => host.textContent?.includes("最新记录") === true);

ok(
  requests.length === 1 && requests[0]?.cursor === "" && requests[0]?.filter.limit === 20 && !("taskId" in requests[0].filter),
  "run history starts with one cross-task cursor request",
);
const more = [...host.querySelectorAll<HTMLButtonElement>("button")]
  .find((button) => button.textContent?.includes("加载更多运行记录"));
ok(Boolean(more), "a backend nextCursor exposes load-more");

await act(async () => {
  more?.click();
});
await waitFor("second cursor page", () => host.textContent?.includes("较早记录") === true);
ok(
  requests.length === 2 && requests[1]?.cursor === "cursor-2" &&
    host.textContent?.indexOf("最新记录")! < host.textContent?.indexOf("较早记录")!,
  "load-more appends the next global page in backend order",
);
ok(
  ![...host.querySelectorAll<HTMLButtonElement>("button")].some((button) => button.textContent?.includes("加载更多运行记录")),
  "the load-more action disappears when the backend reports no more records",
);

const taskListCallsBeforeEvents = taskListCalls;
const catalogCallsBeforeEvents = catalogCalls;
const deliveryTargetCallsBeforeEvents = deliveryTargetCalls;
const runPageCallsBeforeEvents = requests.length;
automationEvents.get("automation:task_changed")?.({ taskId: "do-not-trust" });
automationEvents.get("automation:run_changed")?.({ items: [{ runId: "forged" }] });
automationEvents.get("automation:catalog_changed")?.({ frequencies: ["forged"] });
await act(async () => {
  await new Promise((resolve) => setTimeout(resolve, 80));
});
await waitFor("coalesced invalidation refresh", () => (
  taskListCalls === taskListCallsBeforeEvents + 1 &&
  catalogCalls === catalogCallsBeforeEvents + 1 &&
  deliveryTargetCalls === deliveryTargetCallsBeforeEvents + 1 &&
  requests.length === runPageCallsBeforeEvents + 1
));
ok(
  requests[requests.length - 1]?.cursor === "" && !host.textContent?.includes("forged"),
  "bursty runtime events trigger one authoritative task, catalog, and global-run re-read",
);

await act(async () => root.unmount());
console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
