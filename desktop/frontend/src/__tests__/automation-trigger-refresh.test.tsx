// Run: tsx src/__tests__/automation-trigger-refresh.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import type {
  AutomationRunSummary,
  AutomationTask,
} from "../features/automation/heartbeat/heartbeat.types";
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    await flush();
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function task(id: string, name: string): AutomationTask {
  return {
    id,
    name,
    enabled: true,
    scope: "global",
    prompt: { prompt: "run" },
    schedule: { kind: "interval", every: "30m" },
    execution: { approvalMode: "auto" },
    createdAt: "2026-07-28T00:00:00.000Z",
  };
}

function run(taskId: string, runId: string, status: AutomationRunSummary["status"], startedAt: string): AutomationRunSummary {
  return { taskId, runId, status, startedAt, trigger: "manual" };
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

const nativeSetTimeout = globalThis.setTimeout;
const nativeClearTimeout = globalThis.clearTimeout;
let timerID = 0;
const pollTimers = new Map<number, () => void>();
globalThis.setTimeout = ((handler: TimerHandler, timeout?: number, ...args: unknown[]) => {
  if (timeout === 500 && typeof handler === "function") {
    timerID += 1;
    pollTimers.set(timerID, () => handler(...args));
    return timerID;
  }
  return nativeSetTimeout(handler, timeout, ...args);
}) as typeof setTimeout;
globalThis.clearTimeout = ((id: ReturnType<typeof setTimeout>) => {
  if (typeof id === "number" && pollTimers.delete(id)) return;
  nativeClearTimeout(id);
}) as typeof clearTimeout;

const taskIDs = ["task-a", "task-b"] as const;
const listRequests: Record<string, Array<ReturnType<typeof deferred<AutomationRunSummary[]>>>> = {
  "task-a": [],
  "task-b": [],
};
const triggerRequests: Record<string, Array<ReturnType<typeof deferred<void>>>> = {
  "task-a": [],
  "task-b": [],
};

window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => [
        task("task-a", "Automation A"),
        task("task-b", "Automation B"),
      ],
      AutomationTaskQuota: async () => ({
        state: "ready",
        plan: "pro",
        used: 2,
        limit: 0,
        unlimited: true,
        canCreate: true,
        pausedTaskIds: [],
      }),
      AutomationListRuns: (taskID: string) => {
        const request = deferred<AutomationRunSummary[]>();
        listRequests[taskID].push(request);
        return request.promise;
      },
      AutomationListTemplates: async () => [],
      AutomationTriggerNow: (taskID: string) => {
        const request = deferred<void>();
        triggerRequests[taskID].push(request);
        return request.promise;
      },
      ListWorkspaces: async () => [],
    } as Partial<AppBindings> as AppBindings,
  },
};

function card(host: HTMLElement, title: string): HTMLElement {
  const match = [...host.querySelectorAll<HTMLElement>(".heartbeat-card")]
    .find((candidate) => candidate.textContent?.includes(title));
  if (!match) throw new Error(`missing card ${title}`);
  return match;
}

function triggerButton(taskCard: HTMLElement): HTMLButtonElement {
  const button = taskCard.querySelector<HTMLButtonElement>(".heartbeat-card__open-btn--play");
  if (!button) throw new Error("missing trigger button");
  return button;
}

async function click(button: HTMLButtonElement) {
  await act(async () => {
    button.click();
    await Promise.resolve();
  });
}

async function settle<T>(request: ReturnType<typeof deferred<T>>, value: T) {
  await act(async () => {
    request.resolve(value);
    await request.promise;
  });
}

async function fail<T>(request: ReturnType<typeof deferred<T>>, reason: Error) {
  await act(async () => {
    request.reject(reason);
    await request.promise.catch(() => undefined);
  });
}

async function runPollTimers() {
  const callbacks = [...pollTimers.values()];
  pollTimers.clear();
  await act(async () => {
    callbacks.forEach((callback) => callback());
    await Promise.resolve();
  });
}

console.log("\nautomation trigger refresh");

const access: SubscriptionAccess = { state: "unavailable" };
const host = document.getElementById("root")!;
const root = createRoot(host);
await act(async () => {
  root.render(
    <LocaleProvider>
      <HeartbeatPanel
        open
        embedded
        authenticated
        subscriptionAccess={access}
        onClose={() => {}}
        onOpenTopic={() => {}}
      />
    </LocaleProvider>,
  );
});
await waitFor("task cards", () => taskIDs.every((id) => listRequests[id].length === 1));

await settle(listRequests["task-b"][0], [
  run("task-b", "b-history", "succeeded", "2026-07-27T00:00:00.000Z"),
]);
const taskACard = card(host, "Automation A");
const taskBCard = card(host, "Automation B");
await click(triggerButton(taskACard));
ok(
  triggerRequests["task-a"].length === 1 && listRequests["task-a"].length === 2,
  "first click starts TriggerNow and the first run refresh concurrently",
);
ok(
  taskACard.textContent?.includes("启动中") === true &&
    triggerButton(taskACard).disabled &&
    triggerButton(taskACard).getAttribute("aria-busy") === "true",
  "pending trigger immediately shows an accessible starting row and disables only its button",
);
await click(triggerButton(taskACard));
ok(triggerRequests["task-a"].length === 1, "disabled pending task cannot trigger twice");
ok(!triggerButton(taskBCard).disabled, "another task remains independently triggerable");

const aRunning = run("task-a", "a-running", "running", "2099-01-01T00:00:00.000Z");
await settle(listRequests["task-a"][1], [aRunning]);
ok(
  taskACard.textContent?.includes("a-running") === true &&
    taskACard.textContent?.includes("正在创建运行记录") === false,
  "a real manifest from this trigger replaces the local starting row",
);
await settle(listRequests["task-a"][0], [
  run("task-a", "a-stale-history", "succeeded", "2026-07-27T00:00:00.000Z"),
]);
ok(
  taskACard.textContent?.includes("a-running") === true &&
    taskACard.textContent?.includes("a-stale-history") === false,
  "late initial run history cannot overwrite the newer trigger refresh",
);

await click(triggerButton(taskBCard));
ok(
  triggerRequests["task-b"].length === 1 &&
    triggerRequests["task-a"].length === 1 &&
    triggerButton(taskACard).disabled &&
    triggerButton(taskBCard).disabled,
  "two tasks can trigger independently without sharing state",
);
await fail(listRequests["task-b"][1], new Error("temporary list failure"));
ok(
  taskBCard.textContent?.includes("b-history") === true &&
    taskBCard.textContent?.includes("启动中") === true,
  "temporary ListRuns failure preserves prior history and pending state",
);
await runPollTimers();
ok(listRequests["task-b"].length === 3, "failed ListRuns schedules the next single-task polling cycle");
const bRunning = run("task-b", "b-running", "running", "2099-01-01T00:00:01.000Z");
await settle(listRequests["task-b"][2], [bRunning]);
ok(
  taskBCard.textContent?.includes("b-running") === true &&
    taskACard.textContent?.includes("a-running") === true,
  "per-task refreshes merge without overwriting the other task",
);

await settle(triggerRequests["task-a"][0], undefined);
await waitFor("task A final refresh", () => listRequests["task-a"].length >= 4);
const aSucceeded = run("task-a", "a-running", "succeeded", "2099-01-01T00:00:00.000Z");
await settle(listRequests["task-a"][3], [aSucceeded]);
ok(
  !triggerButton(taskACard).disabled &&
    taskACard.textContent?.includes("成功") === true,
  "TriggerNow resolution performs a final refresh before restoring the button",
);
await settle(listRequests["task-a"][2], [
  run("task-a", "a-stale-poll", "running", "2099-01-01T00:00:00.000Z"),
]);
ok(
  taskACard.textContent?.includes("成功") === true &&
    taskACard.textContent?.includes("a-stale-poll") === false,
  "an older polling response cannot overwrite the final refresh",
);

await fail(triggerRequests["task-b"][0], new Error("trigger failed"));
await waitFor("task B final refresh", () => listRequests["task-b"].length >= 4);
await settle(listRequests["task-b"][3], [bRunning]);
ok(
  !triggerButton(taskBCard).disabled &&
    host.textContent?.includes("自动化任务立即运行失败：自动化操作失败，请稍后重试。") === true
      && host.textContent?.includes("trigger failed") === false,
  "TriggerNow rejection refreshes, clears pending state, and keeps the safe Chinese error",
);

await click(triggerButton(taskACard));
const unmountListRequest = listRequests["task-a"][4];
const unmountTriggerRequest = triggerRequests["task-a"][1];
const errors: unknown[][] = [];
const originalConsoleError = console.error;
console.error = (...args: unknown[]) => { errors.push(args); };
await act(async () => root.unmount());
ok(pollTimers.size === 0, "unmount clears every scheduled polling timer");
await settle(unmountListRequest, [aRunning]);
await settle(unmountTriggerRequest, undefined);
await flush();
console.error = originalConsoleError;
ok(
  listRequests["task-a"].length === 5 &&
    errors.every((args) => !String(args[0]).includes("unmounted")),
  "late polling and trigger completions neither refresh again nor update the unmounted panel",
);

globalThis.setTimeout = nativeSetTimeout;
globalThis.clearTimeout = nativeClearTimeout;
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
