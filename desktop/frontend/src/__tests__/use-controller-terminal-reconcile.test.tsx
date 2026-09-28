// Run: tsx src/__tests__/use-controller-terminal-reconcile.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { snapshotBreadcrumbs } from "../lib/breadcrumbs";
import type { AppBindings } from "../lib/bridge";
import type { ContextInfo, EffortInfo, HistoryMessage, Meta, TabMeta, WireEvent } from "../lib/types";
import { useController } from "../lib/useController";

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

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}${actual === expected ? "" : `: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`}`);
}

type Timer = { at: number; callback: () => void };

class FakeClock {
  now = 1_000;
  nextId = 1;
  timers = new Map<number, Timer>();

  setTimeout = (callback: TimerHandler, delay = 0): number => {
    const id = this.nextId;
    this.nextId += 1;
    this.timers.set(id, { at: this.now + Math.max(0, Number(delay)), callback: callback as () => void });
    return id;
  };

  clearTimeout = (id?: number) => {
    if (id !== undefined) this.timers.delete(id);
  };

  async advance(ms: number) {
    const target = this.now + ms;
    for (;;) {
      const due = [...this.timers.entries()]
        .filter(([, timer]) => timer.at <= target)
        .sort((a, b) => a[1].at - b[1].at || a[0] - b[0])[0];
      if (!due) break;
      const [id, timer] = due;
      this.timers.delete(id);
      this.now = timer.at;
      await act(async () => {
        timer.callback();
        await flushMicrotasks();
      });
    }
    this.now = target;
    await act(flushMicrotasks);
  }
}

async function flushMicrotasks() {
  for (let index = 0; index < 8; index += 1) await Promise.resolve();
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function tabMeta(overrides: Partial<TabMeta> = {}): TabMeta {
  return {
    id: "tab-a",
    scope: "project",
    workspaceRoot: "/repo",
    workspaceName: "repo",
    workspacePath: "/repo",
    topicId: "topic-a",
    topicTitle: "General",
    label: "model",
    ready: true,
    running: false,
    cancellable: false,
    mode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    active: true,
    cwd: "/repo",
    ...overrides,
  };
}

function meta(): Meta {
  return {
    label: "model",
    ready: true,
    eventChannel: "agent:event",
    cwd: "/repo",
    workspaceRoot: "/repo",
    workspaceName: "repo",
    workspacePath: "/repo",
    autoApproveTools: false,
    bypass: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    goal: "",
    goalStatus: "stopped",
  };
}

console.log("\nuse controller terminal reconcile");

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
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;

const clock = new FakeClock();
const realDateNow = Date.now;
Date.now = () => clock.now;
window.setTimeout = clock.setTimeout as typeof window.setTimeout;
window.clearTimeout = clock.clearTimeout as typeof window.clearTimeout;

const eventHandlers: Array<(event: WireEvent) => void> = [];
let listTabsCalls = 0;
let listTabsCallTimes: number[] = [];
let historyCalls = 0;
let replayPendingPromptCalls = 0;
let backendRunning = false;
let historyResponse: HistoryMessage[] = [];
let listTabsResponse: () => Promise<TabMeta[]> = async () => [
  tabMeta({ running: backendRunning, cancellable: backendRunning }),
];
const context: ContextInfo = { used: 0, window: 100, sessionTokens: 0 };
const effort: EffortInfo = { supported: true, current: "auto", default: "auto", levels: ["auto"] };

window.runtime = {
  EventsOn: (name: string, callback: (payload: unknown) => void) => {
    if (name === "agent:event") eventHandlers.push(callback as (event: WireEvent) => void);
    return () => {};
  },
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      ListTabs: async () => {
        listTabsCalls += 1;
        listTabsCallTimes.push(clock.now);
        return listTabsResponse();
      },
      MetaForTab: async () => meta(),
      ContextUsageForTab: async () => context,
      EffortForTab: async () => effort,
      BalanceForTab: async () => ({ available: false, display: "" }),
      JobsForTab: async () => [],
      CheckpointsForTab: async () => [],
      HistoryForTab: async () => {
        historyCalls += 1;
        return historyResponse;
      },
      ReplayPendingPrompts: async () => {
        replayPendingPromptCalls += 1;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

type Controller = ReturnType<typeof useController>;
let controller: Controller | undefined;

function Probe() {
  controller = useController();
  return null;
}

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

async function emit(event: WireEvent) {
  await act(async () => {
    for (const handler of eventHandlers) handler({ tabId: "tab-a", ...event });
    await flushMicrotasks();
  });
}

let turnIdentity = 10_000;
async function startTurn() {
  turnIdentity += 1;
  backendRunning = true;
  await emit({ kind: "turn_started", activeTurnStartedAt: turnIdentity });
}

async function finishTurn() {
  backendRunning = false;
  await emit({ kind: "turn_done" });
}

await act(async () => {
  root.render(<Probe />);
  await flushMicrotasks();
});
eq(controller?.activeTabId, "tab-a", "mount resolves the active tab");
listTabsCalls = 0;
historyCalls = 0;
replayPendingPromptCalls = 0;

await startTurn();
await emit({ kind: "message", text: "normal final" });
await clock.advance(99);
await finishTurn();
await clock.advance(1);
eq(listTabsCalls, 0, "turn_done inside the grace period cancels the terminal query");
eq(controller?.state.running, false, "normal turn_done keeps the existing immediate idle path");

await startTurn();
const duplicateTerminal: WireEvent = {
  kind: "message",
  eventId: "duplicate-terminal-event",
  turnId: `turn-${turnIdentity}`,
  candidateId: "candidate-duplicate",
  eventSequence: 4,
  text: "duplicate controller final",
};
await emit(duplicateTerminal);
await emit(duplicateTerminal);
eq(
  controller?.state.items.filter((item) => item.kind === "assistant" && item.text === "duplicate controller final").length,
  1,
  "duplicate Wails delivery enters useController twice but projects one assistant item",
);
const duplicateTrace = snapshotBreadcrumbs().filter((entry) => entry.cat === "event.delivery" && entry.msg.includes("candidate=candidate-duplicate"));
eq(duplicateTrace.length, 2, "useController records both content-free duplicate arrivals");
ok(duplicateTrace.every((entry) => !entry.msg.includes("duplicate controller final")), "delivery breadcrumbs exclude assistant text");
await finishTurn();

await startTurn();
await emit({ kind: "message", text: "lost terminal event" });
backendRunning = false;
await clock.advance(99);
eq(controller?.state.running, true, "terminal reconciliation waits for the 100ms grace period");
await clock.advance(1);
eq(controller?.state.running, false, "backend idle recovers a lost turn_done at 100ms");
eq(historyCalls, 0, "terminal recovery does not hydrate history when the final is already visible");

listTabsCalls = 0;
await startTurn();
await emit({ kind: "message", text: "backend still running" });
await clock.advance(100);
eq(controller?.state.running, true, "a running backend cannot unlock the composer");
backendRunning = false;
await clock.advance(299);
eq(controller?.state.running, true, "the second terminal query respects the 300ms backoff");
await clock.advance(1);
eq(controller?.state.running, false, "the second terminal query recovers after backend idle");
eq(listTabsCalls, 2, "running then idle uses one terminal query per backoff point");

listTabsCalls = 0;
await startTurn();
await emit({ kind: "message", text: "long running final" });
await clock.advance(100 + 300 + 1_000 + 1_000);
eq(controller?.state.running, true, "a continuously running backend remains locked");
eq(listTabsCalls, 4, "terminal retries continue at the one-second cap");
await finishTurn();

listTabsCalls = 0;
let unavailableAttempt = 0;
listTabsResponse = async () => {
  unavailableAttempt += 1;
  if (unavailableAttempt === 1) throw new Error("SECRET_TERMINAL_ERROR");
  if (unavailableAttempt === 2) return [];
  if (unavailableAttempt === 3) return [tabMeta({ id: "other-tab" })];
  return [tabMeta({ running: false, cancellable: false })];
};
await startTurn();
await emit({ kind: "message", text: "SECRET_TERMINAL_MESSAGE" });
await clock.advance(100 + 300 + 1_000);
eq(controller?.state.running, true, "errors, empty results, and missing tabs remain unknown");
await clock.advance(1_000);
eq(controller?.state.running, false, "terminal reconciliation keeps retrying after unavailable status");
eq(listTabsCalls, 4, "each unavailable terminal result advances one backoff step");
listTabsResponse = async () => [tabMeta({ running: backendRunning, cancellable: backendRunning })];

listTabsCalls = 0;
await startTurn();
await emit({ kind: "message", text: "child final", parentId: "parent-1" });
await emit({ kind: "message", text: "subagent final", subagentRef: "agent-1" });
await clock.advance(2_000);
eq(listTabsCalls, 0, "child and subagent messages do not start terminal reconciliation");
await finishTurn();

listTabsCalls = 0;
const oldQuery = deferred<TabMeta[]>();
listTabsResponse = () => oldQuery.promise;
await startTurn();
await emit({ kind: "message", text: "old turn final" });
await clock.advance(100);
await startTurn();
oldQuery.resolve([tabMeta({ running: false, cancellable: false })]);
await act(flushMicrotasks);
eq(controller?.state.running, true, "an old idle query cannot clear a newly started turn");
eq(listTabsCalls, 1, "the old terminal chain issued only its original query");
await finishTurn();
listTabsResponse = async () => [tabMeta({ running: backendRunning, cancellable: backendRunning })];

listTabsCalls = 0;
historyCalls = 0;
historyResponse = [
  { role: "user", content: "persisted watchdog prompt" },
  { role: "assistant", content: "persisted watchdog final" },
];
await startTurn();
await clock.advance(10_000);
await startTurn();
backendRunning = false;
await clock.advance(29_999);
eq(listTabsCalls, 0, "a new turn identity replaces the old watchdog without an early query");
eq(controller?.state.running, true, "the replacement watchdog uses the new turn activity time");
await clock.advance(1);
eq(controller?.state.running, false, "the replacement watchdog recovers the new turn after it becomes stale");
eq(historyCalls, 1, "watchdog missed-turn recovery reloads persisted history");
ok(
  controller?.state.items.some((item) => item.kind === "assistant" && item.text === "persisted watchdog final") === true,
  "watchdog recovery projects the persisted final into the transcript",
);
historyResponse = [];

listTabsCalls = 0;
await startTurn();
await clock.advance(29_999);
await emit({ kind: "tool_progress", tool: { id: "tool-1", name: "test", args: "{}", readOnly: true } });
await clock.advance(1);
eq(listTabsCalls, 0, "fresh activity at the watchdog boundary reschedules without querying");
backendRunning = false;
await clock.advance(29_998);
eq(controller?.state.running, true, "rescheduled watchdog waits for the latest activity to become stale");
await clock.advance(1);
eq(controller?.state.running, false, "a rescheduled watchdog recovers once the backend is idle");

listTabsCalls = 0;
listTabsCallTimes = [];
let watchdogAttempt = 0;
listTabsResponse = async () => {
  watchdogAttempt += 1;
  if (watchdogAttempt === 1) return [tabMeta({ running: true, cancellable: true })];
  if (watchdogAttempt === 2) throw new Error("SECRET_WATCHDOG_ERROR");
  return [tabMeta({ running: false, cancellable: false })];
};
await startTurn();
await clock.advance(30_000);
eq(controller?.state.running, true, "watchdog keeps running state after an authoritative running result");
await clock.advance(29_999);
eq(listTabsCalls, 1, "watchdog running result does not create a tight query loop");
await clock.advance(1);
eq(controller?.state.running, true, "watchdog failure does not guess that the backend is idle");
eq(listTabsCalls, 2, "watchdog failure re-arms without an immediate retry");
await clock.advance(30_000);
eq(controller?.state.running, false, "watchdog re-arms after failure and later recovers");
eq(
  listTabsCallTimes.slice(0, 3).map((at, index) => index === 0 ? 0 : at - listTabsCallTimes[index - 1]).join(","),
  "0,30000,30000",
  "watchdog polling queries remain at least 30 seconds apart",
);
eq(listTabsCalls, 4, "idle watchdog recovery performs one expected post-hydration status refresh");

const terminalBreadcrumbs = snapshotBreadcrumbs().filter((entry) => entry.cat === "terminal" || entry.cat === "watchdog");
const breadcrumbText = terminalBreadcrumbs.map((entry) => entry.msg).join("\n");
ok(terminalBreadcrumbs.length > 0, "terminal and watchdog reconciliation emit diagnostic breadcrumbs");
ok(!breadcrumbText.includes("SECRET_TERMINAL_MESSAGE"), "breadcrumbs exclude message content");
ok(!breadcrumbText.includes("SECRET_TERMINAL_ERROR"), "breadcrumbs exclude terminal error details");
ok(!breadcrumbText.includes("SECRET_WATCHDOG_ERROR"), "breadcrumbs exclude watchdog error details");

const unmountQuery = deferred<TabMeta[]>();
listTabsResponse = () => unmountQuery.promise;
await startTurn();
await emit({ kind: "message", text: "unmount final" });
await clock.advance(100);
replayPendingPromptCalls = 0;
await act(async () => root.unmount());
unmountQuery.resolve([tabMeta({ running: false, cancellable: false, pendingPrompt: true })]);
await flushMicrotasks();
eq(replayPendingPromptCalls, 0, "an in-flight terminal result cannot apply runtime status after unmount");
eq(clock.timers.size, 0, "unmount clears reconciliation timers and does not re-arm in-flight work");

Date.now = realDateNow;
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
