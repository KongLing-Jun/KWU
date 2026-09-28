// Run: tsx src/__tests__/automation-error-visibility.test.tsx

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
function changeInput(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
  if (!setter) throw new Error("missing input value setter");
  setter.call(input, value);
  const reactPropsKey = Object.keys(input).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (input as unknown as Record<string, { onChange?: (event: { target: HTMLInputElement }) => void }>)[reactPropsKey]
    : undefined;
  reactProps?.onChange?.({ target: input });
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
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
  id: "task-errors",
  revision: 1,
  name: "错误可见性测试",
  enabled: true,
  scope: "global",
  prompt: { prompt: "run" },
  schedule: { kind: "interval", every: "30m" },
  execution: { approvalMode: "auto" },
  createdAt: "2026-08-01T00:00:00Z",
};
let taskLoadCalls = 0;
let templateSaveCalls = 0;
const events = new Map<string, (...payload: unknown[]) => void>();
window.runtime = {
  EventsOn: (name, callback) => {
    events.set(name, callback);
    return () => { events.delete(name); };
  },
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => {
        taskLoadCalls += 1;
        if (taskLoadCalls === 1) throw new Error("initial load unavailable");
        return [task];
      },
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 1, limit: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
      AutomationListRuns: async () => [{ runId: "run-errors", taskId: task.id, taskName: task.name, status: "running", startedAt: "2026-08-01T00:00:00Z" }],
      AutomationListTemplates: async () => [],
      AutomationGetScheduleCatalog: async () => [],
      AutomationListDeliveryTargets: async () => [],
      AutomationListCapabilities: async () => ({ models: [], skills: [], experts: [], connectors: [] }),
      AutomationValidateDraft: async () => ({ task: {}, nextRuns: [] }),
      AutomationResolveCapabilities: async () => ({ riskSummary: "只读能力" }),
      AutomationModels: async () => [],
      AutomationReadRun: async () => ({ manifest: { runId: "run-errors", taskId: task.id, taskName: task.name, status: "running" } }),
      AutomationCancelRun: async () => { throw new Error("cancel unavailable"); },
      AutomationUpdateTask: async (next: Record<string, unknown>) => ({ ...next, revision: 2 }),
      AutomationSaveTaskTemplate: async () => {
        templateSaveCalls += 1;
        throw new Error("template store unavailable");
      },
      ListWorkspaces: async () => [],
      SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
      ListExperts: async () => [],
      ModelMarketplace: async () => ({ sources: [] }),
    } as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation error visibility");
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

await waitFor("initial load error", () => host.querySelector('[role="alert"]') !== null);
ok(host.querySelector('[role="alert"]')?.textContent?.includes("自动化任务加载失败") === true, "initial load failure is visible");

events.get("automation:task_changed")?.();
await act(() => flush(80));
await waitFor("task after authoritative reload", () => host.querySelector(".heartbeat-card__title-btn") !== null);
await waitFor("running record", () => host.querySelector(".heartbeat-card__run") !== null);
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-card__run")?.click(); await flush(); });
await waitFor("run detail", () => host.textContent?.includes("运行概要") === true);
const cancelButton = [...host.querySelectorAll<HTMLButtonElement>("button")].find((button) => button.textContent?.trim() === "取消");
await act(async () => { cancelButton?.click(); await flush(); });
await waitFor("cancel error", () => host.querySelector('[role="alert"]')?.textContent?.includes("自动化任务取消失败") === true);
ok(true, "cancel failure remains visible in run detail");

await act(async () => { host.querySelector<HTMLButtonElement>('[aria-label="返回自动化任务列表"]')?.click(); await flush(); });
await act(async () => { host.querySelector<HTMLButtonElement>(".heartbeat-card__title-btn")?.click(); await flush(); });
await waitFor("task editor", () => host.querySelector(".heartbeat-editor") !== null);
const titleInput = host.querySelector<HTMLInputElement>(".heartbeat-editor__input");
await act(async () => {
  if (titleInput) changeInput(titleInput, "保留的草稿");
  const templateToggle = [...host.querySelectorAll<HTMLLabelElement>(".heartbeat-editor__template-toggle")]
    .find((label) => label.textContent?.includes("同时保存为模板"));
  templateToggle?.querySelector<HTMLInputElement>('input[type="checkbox"]')?.click();
  await flush();
});
const saveButton = [...host.querySelectorAll<HTMLButtonElement>(".heartbeat-editor__actions button")]
  .find((button) => button.textContent?.trim() === "保存");
await act(async () => { saveButton?.click(); await flush(); });
await waitFor("template save error", () => host.querySelector('[role="alert"]')?.textContent?.includes("任务已保存，但模板保存失败") === true);
ok(templateSaveCalls === 1, "the requested template save is attempted once");
ok(host.querySelector(".heartbeat-editor") !== null, "partial template failure keeps the editor open");
ok(host.querySelector<HTMLInputElement>(".heartbeat-editor__input")?.value === "保留的草稿", "partial template failure keeps the persisted draft");
ok(
  host.querySelector('[role="alert"]')?.textContent?.includes("任务已保存，但模板保存失败") === true,
  "partial template failure reports partial success",
);

await act(async () => root.unmount());
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
