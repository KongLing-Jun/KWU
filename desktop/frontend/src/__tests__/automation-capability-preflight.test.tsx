// Run: tsx src/__tests__/automation-capability-preflight.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";

let passed = 0;
let failed = 0;
function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
}
async function flush(delay = 0) {
  await new Promise((resolve) => setTimeout(resolve, delay));
}
async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    await act(() => flush());
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
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });

const task = {
  id: "task-preflight",
  revision: 1,
  name: "能力预检任务",
  enabled: true,
  scope: "global",
  prompt: { prompt: "summarize" },
  schedule: { kind: "interval", every: "30m" },
  execution: { approvalMode: "auto" },
  createdAt: "2026-08-01T00:00:00Z",
};
let resolveCalls = 0;
let updateCalls = 0;
window.runtime = { EventsOn: () => () => {}, BrowserOpenURL: () => {} };
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => [task],
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 1, limit: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
      AutomationListRuns: async () => [],
      AutomationListTemplates: async () => [],
      AutomationGetScheduleCatalog: async () => [],
      AutomationListDeliveryTargets: async () => [],
      AutomationListCapabilities: async () => ({ models: [], skills: [], experts: [], connectors: [] }),
      AutomationValidateDraft: async () => ({ task: {}, nextRuns: [] }),
      AutomationResolveCapabilities: async () => {
        resolveCalls += 1;
        if (resolveCalls === 1) throw new Error("connector authorization expired");
        return { riskSummary: "连接器只读访问；高风险操作需要审批" };
      },
      AutomationUpdateTask: async (next: Record<string, unknown>) => {
        updateCalls += 1;
        return { ...next, revision: 2 };
      },
      AutomationModels: async () => [],
      ListWorkspaces: async () => [],
      SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
      ListExperts: async () => [],
      ModelMarketplace: async () => ({ sources: [] }),
    } as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation capability preflight");
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

await waitFor("task", () => host.querySelector(".heartbeat-card__title-btn") !== null);
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-card__title-btn")?.click(); await flush(); });
await waitFor("editor", () => host.querySelector(".heartbeat-editor") !== null);
ok(host.querySelector(".heartbeat-editor__top-actions") === null, "editor does not duplicate actions above the form");
const saveButton = [...host.querySelectorAll<HTMLButtonElement>(".heartbeat-editor__actions button")]
  .find((button) => button.textContent?.trim() === "保存");

await act(async () => { saveButton?.click(); await flush(); });
await waitFor("capability failure", () => host.querySelector('[role="alert"]')?.textContent?.includes("能力检查失败") === true);
ok(resolveCalls === 1, "save resolves authoritative capabilities first");
ok(updateCalls === 0, "failed capability resolution blocks update persistence");
ok(host.querySelector(".heartbeat-editor") !== null, "failed capability resolution keeps the draft editor mounted");

await act(async () => { saveButton?.click(); await flush(); });
await waitFor("successful update", () => updateCalls === 1 && host.querySelector(".heartbeat-editor") === null);
ok(resolveCalls === 2 && updateCalls === 1, "successful capability resolution allows exactly one update");
ok(
  host.querySelector('[role="status"]')?.textContent?.includes("连接器只读访问；高风险操作需要审批") === true,
  "the authoritative risk summary remains visible after save",
);

await act(async () => root.unmount());
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
