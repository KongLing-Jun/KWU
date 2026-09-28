// Run: tsx src/__tests__/tab-switch-hydration.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import type { AppBindings } from "../lib/bridge";
import { composerDraftKeyForTab } from "../lib/composerDraftKey";
import { useController } from "../lib/useController";
import type { BalanceInfo, CheckpointMeta, ComposerSessionIdentity, ContextInfo, EffortInfo, HistoryMessage, JobView, Meta, TabMeta, TurnBillingSettled, WireEvent } from "../lib/types";

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
  if (actual === expected) {
    ok(true, label);
  } else {
    ok(false, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
}

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
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

function installWindowFakeClock() {
  const originalSetTimeout = window.setTimeout.bind(window);
  const originalClearTimeout = window.clearTimeout.bind(window);
  let now = 0;
  let nextId = 1;
  const timers = new Map<number, { at: number; callback: () => void }>();
  window.setTimeout = ((handler: TimerHandler, timeout = 0, ...args: unknown[]) => {
    const id = nextId;
    nextId += 1;
    timers.set(id, {
      at: now + Number(timeout || 0),
      callback: () => {
        if (typeof handler === "function") handler(...args);
      },
    });
    return id;
  }) as typeof window.setTimeout;
  window.clearTimeout = ((id: number) => {
    timers.delete(id);
  }) as typeof window.clearTimeout;
  return {
    advance(ms: number) {
      now += ms;
      for (;;) {
        const due = Array.from(timers.entries())
          .filter(([, timer]) => timer.at <= now)
          .sort((left, right) => left[1].at - right[1].at || left[0] - right[0]);
        if (due.length === 0) return;
        const [id, timer] = due[0];
        timers.delete(id);
        timer.callback();
      }
    },
    pending: () => timers.size,
    restore() {
      timers.clear();
      window.setTimeout = originalSetTimeout;
      window.clearTimeout = originalClearTimeout;
    },
  };
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function tabMeta(id: string, overrides: Partial<TabMeta> = {}): TabMeta {
  const workspaceRoot = `/repo/${id}`;
  return {
    id,
    scope: "project",
    workspaceRoot,
    workspaceName: id,
    workspacePath: workspaceRoot,
    gitBranch: "main",
    topicId: `topic-${id}`,
    topicTitle: id,
    label: `model-${id}`,
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    active: false,
    cwd: workspaceRoot,
    ...overrides,
  };
}

function metaFor(tab: TabMeta): Meta {
  return {
    label: tab.label,
    modelRef: tab.modelRef,
    ready: tab.ready,
    startupErr: tab.startupErr,
    eventChannel: "agent:event",
    cwd: tab.cwd || tab.workspaceRoot,
    workspaceRoot: tab.workspaceRoot,
    workspaceName: tab.workspaceName,
    workspacePath: tab.workspacePath,
    gitBranch: tab.gitBranch,
    autoApproveTools: false,
    bypass: false,
    collaborationMode: tab.collaborationMode ?? "normal",
    toolApprovalMode: tab.toolApprovalMode ?? "ask",
    tokenMode: tab.tokenMode ?? "full",
    goal: "",
    goalStatus: "stopped",
  };
}

function userMessage(content: string): HistoryMessage {
  return { role: "user", content };
}

console.log("\ntab switch hydration");

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
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

const context: ContextInfo = { used: 12, window: 100, sessionTokens: 12 };
const effort: EffortInfo = { supported: true, current: "auto", default: "auto", levels: ["auto"] };
const balance: BalanceInfo = { available: false, display: "" };
const jobs: JobView[] = [];
const checkpoints: CheckpointMeta[] = [];
const tabA = tabMeta("tab-a", { active: true });
const tabB = tabMeta("tab-b");
const tabC = tabMeta("tab-c");
const tabD = tabMeta("tab-d");
const tabE = tabMeta("tab-e", {
  ready: false,
  topicId: "topic-history",
  expertId: "delivery-team",
  expertName: "Delivery Team",
});
const tabF = tabMeta("tab-f", { topicId: "" });
const tabG = tabMeta("tab-g", {
  ready: false,
  sessionPath: "/repo/tab-g/restored-zero.jsonl",
  sessionTurns: 0,
  modelRef: "provider/restarted-model",
  billingSource: "newapi_byok",
  effort: "low",
});
let backendActiveId = "tab-a";
const historyB = deferred<HistoryMessage[]>();
const historyD = deferred<HistoryMessage[]>();
const historyEGate = deferred<HistoryMessage[]>();
const historyGGate = deferred<HistoryMessage[]>();
let historyAGate: ReturnType<typeof deferred<HistoryMessage[]>> | undefined;
const effortGGate = deferred<EffortInfo>();
let historyERevalidationGate: ReturnType<typeof deferred<HistoryMessage[]>> | undefined;
let activateTopicGate: ReturnType<typeof deferred<void>> | undefined;
const setActiveBGate = deferred<void>();
const setActiveGateByTab = new Map<string, ReturnType<typeof deferred<void>>>([["tab-b", setActiveBGate]]);
const historyCalls: string[] = [];
let setActiveCalls = 0;
let submitCalls = 0;
let newSessionCalls = 0;
let tabEReady = false;
let contextECalls = 0;
let contextETokens = 78;
let balanceCalls = 0;
let jobsCalls = 0;
let checkpointCalls = 0;
let holdContextE = false;
const contextEGate = deferred<ContextInfo>();
let omitTabMetrics = false;
let holdListTabs = false;
const listTabsGate = deferred<void>();
let holdRaceListTabs = false;
const raceListTabsGate = deferred<void>();
let rejectNextHistoryA = false;
let holdHistoryE = false;
let activateSessionPathOverride = "";
let activateSessionTurnsOverride: number | undefined;
let activateSceneOverride: TabMeta["scene"];
let activateSceneStateOverride: TabMeta["sceneState"];
let resumeHistoryGate: ReturnType<typeof deferred<HistoryMessage[]>> | undefined;
let zeroTurnHistoryGate: ReturnType<typeof deferred<HistoryMessage[]>> | undefined;
const runningTabs = new Set<string>();
const tabsById = new Map([tabA, tabB, tabC, tabD, tabE, tabF, tabG].map((tab) => [tab.id, tab]));
const eventHandlers: Array<(e: WireEvent) => void> = [];
const readyHandlers: Array<() => void> = [];
const turnBillingHandlers: Array<(view: TurnBillingSettled) => void> = [];

function currentTabs(): TabMeta[] {
  return [tabA, tabB, tabC, tabD, { ...tabE, ready: tabEReady }, tabF, tabG].map((tab) => {
    const running = runningTabs.has(tab.id);
    const activeMetrics = tab.id === "tab-e" && running && !omitTabMetrics
      ? { activeTurnStartedAt: 2345, activeTurnCompletionTokens: contextETokens }
      : {};
    return { ...tab, active: tab.id === backendActiveId, running, cancellable: running, ...activeMetrics };
  });
}

window.runtime = {
  EventsOn: (name: string, cb: (...data: unknown[]) => void) => {
    if (name === "agent:event") eventHandlers.push(cb as (e: WireEvent) => void);
    if (name === "agent:ready") readyHandlers.push(cb as () => void);
    if (name === "turn-billing:settled") turnBillingHandlers.push(cb as (view: TurnBillingSettled) => void);
    return () => {};
  },
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      ListTabs: async () => {
        if (holdListTabs) await listTabsGate.promise;
        if (holdRaceListTabs) await raceListTabsGate.promise;
        return currentTabs();
      },
      MetaForTab: async (tabID: string) => metaFor(tabsById.get(tabID) ?? tabA),
      ContextUsageForTab: async (tabID: string) => {
        if (tabID === "tab-d") return { ...context, activeTurnStartedAt: 1234, activeTurnCompletionTokens: 56 };
        if (tabID === "tab-e") {
          contextECalls += 1;
          if (contextECalls === 1) throw new Error("transient context failure");
          if (holdContextE) return contextEGate.promise;
          return { ...context, activeTurnStartedAt: 2345, activeTurnCompletionTokens: contextETokens };
        }
        return context;
      },
      EffortForTab: async (tabID: string) => tabID === "tab-g" ? effortGGate.promise : effort,
      BalanceForTab: async () => {
        balanceCalls += 1;
        return balance;
      },
      JobsForTab: async () => {
        jobsCalls += 1;
        return jobs;
      },
      CheckpointsForTab: async () => {
        checkpointCalls += 1;
        return checkpoints;
      },
      HistoryForTab: async (tabID: string) => {
        historyCalls.push(tabID);
        if (tabID === "tab-a" && rejectNextHistoryA) {
          rejectNextHistoryA = false;
          throw new Error("history disk read failed");
        }
        if (tabID === "tab-a" && historyAGate) return historyAGate.promise;
        if (tabID === "tab-b") return historyB.promise;
        if (tabID === "tab-d") return historyD.promise;
        if (tabID === "tab-e") {
          if (zeroTurnHistoryGate) return zeroTurnHistoryGate.promise;
          if (historyERevalidationGate) return historyERevalidationGate.promise;
          if (holdHistoryE) return historyEGate.promise;
          return [userMessage("restored expert-team history")];
        }
        if (tabID === "tab-g") return historyGGate.promise;
        return [userMessage("cached A")];
      },
      ActivateTopic: async () => {
        if (activateTopicGate) await activateTopicGate.promise;
        backendActiveId = "tab-e";
        return {
          ...tabE,
          sessionPath: activateSessionPathOverride || tabE.sessionPath,
          sessionTurns: activateSessionTurnsOverride,
          scene: activateSceneOverride,
          sceneState: activateSceneStateOverride,
          active: true,
          ready: tabEReady,
          running: runningTabs.has("tab-e"),
          cancellable: runningTabs.has("tab-e"),
        };
      },
      EnsureBlankSurface: async () => {
        backendActiveId = "tab-f";
        return { ...tabF, active: true };
      },
      ResumeSessionForTab: async () => resumeHistoryGate?.promise ?? [],
      OpenProjectTab: async () => {
        backendActiveId = "tab-d";
        return { ...tabD, active: true };
      },
      NewSession: async () => {
        newSessionCalls += 1;
      },
      ReplayPendingPrompts: async () => {},
      SetActiveTab: async (tabID: string) => {
        setActiveCalls += 1;
        const gate = setActiveGateByTab.get(tabID);
        if (gate) await gate.promise;
        backendActiveId = tabID;
      },
      SubmitToTab: async (tabID: string) => {
        submitCalls += 1;
        runningTabs.add(tabID);
      },
      SubmitDisplayToTab: async (tabID: string) => {
        submitCalls += 1;
        runningTabs.add(tabID);
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

type Controller = ReturnType<typeof useController>;
let controller: Controller | undefined;
let observeReactivation = false;
const reactivationMetrics: Array<{ startedAt: number; tokens: number }> = [];

function Probe() {
  controller = useController();
  if (observeReactivation && controller.activeTabId === "tab-e") {
    reactivationMetrics.push({ startedAt: controller.state.turnStartAt, tokens: controller.state.turnTokens });
  }
  return null;
}

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);

await act(async () => {
  root.render(<Probe />);
  await flushPromises();
});
await waitFor("initial active tab", () => controller?.activeTabId === "tab-a" && controller.state.items.length === 1);

await act(async () => {
  for (const handler of eventHandlers) {
    handler({ kind: "turn_started", tabId: "tab-a", turnId: "turn-hydration-race" });
    handler({
      kind: "message",
      tabId: "tab-a",
      turnId: "turn-hydration-race",
      candidateId: "candidate-hydration-race",
      text: "hydration race answer",
    });
  }
  for (const handler of turnBillingHandlers) {
    handler({ taskId: "task-hydration-race", chargedCredits: "5" });
  }
  await flushPromises();
});
historyAGate = deferred<HistoryMessage[]>();
let activeTurnHydration: Promise<unknown> | undefined;
await act(async () => {
  activeTurnHydration = controller?.syncActiveTab(true);
  await flushPromises();
});
await act(async () => {
  historyAGate?.resolve([
    userMessage("cached A"),
    { role: "assistant", content: "hydration race answer" },
  ]);
  await activeTurnHydration;
  await flushPromises();
});
historyAGate = undefined;
eq(controller?.state.runtimeTurnId, "turn-hydration-race", "same-session hydration preserves the active turn identity");
const hydratedLiveAnswer = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "hydration race answer",
);
eq(
  hydratedLiveAnswer?.kind === "assistant" ? hydratedLiveAnswer.runtimeTurnId : undefined,
  "turn-hydration-race",
  "same-session hydration preserves the assistant turn correlation",
);
await act(async () => {
  for (const handler of eventHandlers) {
    handler({
      kind: "turn_done",
      tabId: "tab-a",
      turnId: "turn-hydration-race",
      artifactDelivery: { version: 1, paths: ["hydration-race.pdf"] },
      turn: {
        taskId: "task-hydration-race",
        modelRef: "provider/hydration-race",
        billingSource: "subscription_pool",
      },
    });
  }
  await flushPromises();
});
const completedHydratedAnswer = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "hydration race answer",
);
eq(
  JSON.stringify(completedHydratedAnswer?.kind === "assistant" ? completedHydratedAnswer.artifactDelivery?.paths : undefined),
  JSON.stringify(["hydration-race.pdf"]),
  "a completion after same-session hydration immediately attaches its file delivery contract",
);
eq(
  completedHydratedAnswer?.kind === "assistant" ? completedHydratedAnswer.turn?.modelRef : undefined,
  "provider/hydration-race",
  "a completion after same-session hydration immediately attaches its model",
);
eq(
  completedHydratedAnswer?.kind === "assistant" ? completedHydratedAnswer.turn?.chargedCredits : undefined,
  "5",
  "a completion after same-session hydration immediately attaches settled subscription credits",
);

rejectNextHistoryA = true;
await act(async () => {
  await controller?.syncActiveTab(true);
  await flushPromises();
});
ok(controller?.state.items.some((item) => item.kind === "user" && item.text === "cached A") ?? false, "history rejection keeps the current transcript");
eq(controller?.state.hydrateError, "history disk read failed", "history rejection records an explicit hydration error");

let switchToB: Promise<TabMeta[] | undefined> | undefined;
await act(async () => {
  switchToB = controller?.switchTab("tab-b", tabB);
  await flushPromises();
});

eq(setActiveCalls, 1, "SetActiveTab is called for the selected tab");
eq(controller?.activeTabId, "tab-b", "switchTab updates the active tab before backend activation resolves");
eq(controller?.state.meta?.label, "model-tab-b", "switchTab applies optimistic tab metadata immediately");
eq(controller?.state.items.length, 0, "uncached target tab does not keep the previous transcript visible");
eq(controller?.state.hydrating, true, "target tab shows lightweight hydration state while backend activation is pending");
eq(controller?.state.backendActivationPending, true, "target tab gates unscoped actions while backend activation is pending");
ok(!historyCalls.includes("tab-b"), "HistoryForTab is not requested before SetActiveTab completes");

let newSessionWhileSwitching: Promise<void> | undefined;
await act(async () => {
  newSessionWhileSwitching = controller?.newSession();
  await flushPromises();
});
eq(newSessionCalls, 0, "newSession waits for backend activation before using the unscoped binding");

await act(async () => {
  setActiveBGate.resolve();
  setActiveGateByTab.delete("tab-b");
  await switchToB;
  await newSessionWhileSwitching;
  await flushPromises();
});
eq(newSessionCalls, 1, "newSession runs after the selected tab is active in the backend");
await waitFor("tab-b history request", () => historyCalls.includes("tab-b"));

const historyACallsBeforeSwitchBack = historyCalls.filter((tabID) => tabID === "tab-a").length;
await act(async () => {
  await controller?.switchTab("tab-a", tabA);
  await flushPromises();
});
await waitFor("tab-a restored", () => controller?.activeTabId === "tab-a" && controller.state.items.some((item) => item.kind === "user" && item.text === "cached A"));
eq(
  historyCalls.filter((tabID) => tabID === "tab-a").length,
  historyACallsBeforeSwitchBack,
  "switching back to an already hydrated idle tab reuses its cached transcript",
);

await act(async () => {
  historyB.resolve([userMessage("late B")]);
  await historyB.promise;
  await flushPromises();
});

eq(controller?.activeTabId, "tab-a", "late history for another tab does not change the active tab");
ok(controller?.state.items.some((item) => item.kind === "user" && item.text === "cached A") ?? false, "late history for another tab does not overwrite the active transcript");
ok(!(controller?.state.items.some((item) => item.kind === "user" && item.text === "late B") ?? false), "late history stays scoped to its tab state");

const rapidSwitchBGate = deferred<void>();
setActiveGateByTab.set("tab-b", rapidSwitchBGate);
const setActiveCallsBeforeRapidSwitch = setActiveCalls;
let rapidSwitchToB: Promise<TabMeta[] | undefined> | undefined;
let rapidSwitchBackToA: Promise<TabMeta[] | undefined> | undefined;
await act(async () => {
  rapidSwitchToB = controller?.switchTab("tab-b", tabB);
  await flushPromises();
});
await act(async () => {
  rapidSwitchBackToA = controller?.switchTab("tab-a", tabA);
  await flushPromises();
});
eq(controller?.activeTabId, "tab-a", "the latest rapid switch owns the visible task immediately");
eq(
  setActiveCalls,
  setActiveCallsBeforeRapidSwitch + 1,
  "rapid switches serialize backend activation while the first switch is pending",
);
await act(async () => {
  rapidSwitchBGate.resolve();
  setActiveGateByTab.delete("tab-b");
  await rapidSwitchToB;
  await rapidSwitchBackToA;
  await flushPromises();
});
eq(backendActiveId, "tab-a", "serialized rapid switches leave the backend on the latest task");
eq(controller?.activeTabId, "tab-a", "a superseded switch completion cannot replace the latest visible task");

await act(async () => {
  for (const handler of eventHandlers) handler({ kind: "phase", text: "Planner is thinking", tabId: "tab-a" });
  for (const handler of eventHandlers) handler({ kind: "message", text: "Planner kept", reasoning: "Planner notes", tabId: "tab-a" });
  await flushPromises();
});
await waitFor("cached planner transcript", () =>
  controller?.state.items.some((item) => item.kind === "assistant" && item.text === "Planner kept" && item.reasoning === "Planner notes") ?? false
);
const historyCallsBeforeReady = historyCalls.length;
await act(async () => {
  for (const handler of readyHandlers) handler();
  await flushPromises();
});
await waitFor("ready hydration settled", () => controller?.state.hydrating === false);
eq(historyCalls.length, historyCallsBeforeReady, "agent ready with cached transcript skips executor-only history hydration");
ok(controller?.state.items.some((item) => item.kind === "phase" && item.text === "Planner is thinking") ?? false, "agent ready keeps cached planner phase");
ok(controller?.state.items.some((item) => item.kind === "assistant" && item.text === "Planner kept" && item.reasoning === "Planner notes") ?? false, "agent ready keeps cached planner answer");

await act(async () => {
  controller?.sendToTab("tab-c", "streaming C");
  await flushPromises();
});
await act(async () => {
  await controller?.switchTab("tab-c", tabC);
  await flushPromises();
});
eq(controller?.activeTabId, "tab-c", "switching to a cached running tab still updates the active tab");
ok(controller?.state.items.some((item) => item.kind === "user" && item.text === "streaming C") ?? false, "cached running tab keeps its optimistic transcript");
ok(!historyCalls.includes("tab-c"), "cached running tab skips history hydration");

await act(async () => {
  await controller?.openProjectTab(tabD.workspaceRoot, tabD.topicId || "");
  await flushPromises();
});
eq(controller?.activeTabId, "tab-d", "openProjectTab activates the opened tab");
eq(controller?.state.items.length, 0, "open topic keeps the new tab transcript empty while hydrating");
ok(controller?.state.hydratePlaceholderItems?.some((item) => item.kind === "user" && item.text === "streaming C") ?? false, "open topic stores previous transcript only as a hydration placeholder");
eq(controller?.state.hydratePlaceholderTabId, "tab-c", "hydration placeholder remembers the workspace-owning source tab");

await act(async () => {
  runningTabs.add("tab-d");
  for (const handler of eventHandlers) handler({ kind: "phase", text: "live D while history loads", tabId: "tab-d" });
  historyD.resolve([userMessage("saved D before live output")]);
  await historyD.promise;
  await flushPromises();
});
ok(
  controller?.state.items.some((item) => item.kind === "user" && item.text === "saved D before live output") ?? false,
  "task hydration keeps the saved history that arrived after activation",
);
ok(
  controller?.state.items.some((item) => item.kind === "phase" && item.text === "live D while history loads") ?? false,
  "task hydration preserves live output received while history was loading",
);
eq(controller?.state.hydratePlaceholderItems?.length ?? 0, 0, "empty topic history clears the hydration placeholder");
eq(controller?.state.hydratePlaceholderTabId, undefined, "empty topic history clears the placeholder workspace owner");
await waitFor("open topic restores running runtime status", () => Boolean(controller?.state.running && controller.state.cancellable));
eq(controller?.state.running, true, "open topic hydrate restores backend running status");
eq(controller?.state.cancellable, true, "open topic hydrate restores stop availability");
eq(controller?.state.turnStartAt, 1234, "open topic hydrate restores the backend turn start");
eq(controller?.state.turnTokens, 56, "open topic hydrate restores the backend turn tokens");

await act(async () => {
  runningTabs.add("tab-e");
  omitTabMetrics = true;
  holdHistoryE = true;
  await controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
eq(controller?.activeTabId, "tab-e", "history activation switches to the expert-team topic");
await waitFor("early expert-team history", () => historyCalls.includes("tab-e"));
const historyECallsBeforeReady = historyCalls.filter((tabID) => tabID === "tab-e").length;
await act(async () => {
  for (const handler of readyHandlers) handler();
  await flushPromises();
});
eq(
  historyCalls.filter((tabID) => tabID === "tab-e").length,
  historyECallsBeforeReady,
  "agent ready does not supersede in-flight surface history hydration",
);
eq(controller?.surfaceTransitionActive, true, "surface view remains pending until its original history request settles");
await act(async () => {
  holdHistoryE = false;
  historyEGate.resolve([userMessage("restored expert-team history")]);
  await historyEGate.promise;
  await flushPromises();
});
await waitFor("early restored expert-team transcript", () =>
  controller?.state.items.some((item) => item.kind === "user" && item.text === "restored expert-team history") ?? false
);
eq(controller?.surfaceTransitionActive, false, "original history hydration releases the pending surface after ready");
ok(true, "history activation renders disk-backed history before the target controller is ready");

tabEReady = true;
await waitFor("running expert-team runtime", () => Boolean(controller?.state.running && controller.state.cancellable));
eq(controller?.state.turnStartAt, 0, "failed active snapshot does not invent a turn start");
eq(controller?.state.turnTokens, 0, "failed active snapshot does not invent turn tokens");
await act(async () => {
  await new Promise((resolve) => setTimeout(resolve, 300));
  await flushPromises();
});
await waitFor("retried expert-team metrics", () => controller?.state.turnTokens === 78);
omitTabMetrics = false;
eq(controller?.state.turnStartAt, 2345, "activateTopic restores the backend turn start after deleting cached state");
eq(controller?.state.turnTokens, 78, "activateTopic restores the backend turn tokens after deleting cached state");
ok(contextECalls >= 2, "failed active snapshot is retried automatically");

runningTabs.delete("tab-e");
await act(async () => {
  for (const handler of eventHandlers) handler({ kind: "turn_done", tabId: "tab-e" });
  await flushPromises();
});
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
eq(controller?.activeTabId, "tab-f", "blank surface replaces the visible expert-team state");
eq(controller?.state.running, false, "foreground blank tab starts idle");

await act(async () => {
  await controller?.switchTab("tab-e", { ...tabE, ready: true });
  await flushPromises();
});
eq(controller?.activeTabId, "tab-e", "completed expert-team tab can be selected again");
eq(
  controller?.renderedViewTarget?.composerIdentity?.expertId,
  "delivery-team",
  "switching back to a completed task restores its expert-team identity",
);
eq(
  controller?.renderedViewTarget?.composerIdentity?.expertName,
  "Delivery Team",
  "switching back to a completed task restores its expert-team display name",
);
await act(async () => {
  await controller?.switchTab("tab-f", tabF);
  await flushPromises();
});

await act(async () => {
  for (const handler of eventHandlers) handler({ kind: "turn_started", tabId: "tab-e" });
  for (const handler of eventHandlers) handler({ kind: "turn_done", tabId: "tab-e" });
  await flushPromises();
});
eq(controller?.activeTabId, "tab-f", "background turn_done does not switch the foreground tab");
eq(controller?.state.running, false, "background turn_done leaves the foreground tab idle");

historyERevalidationGate = deferred<HistoryMessage[]>();
activateTopicGate = deferred<void>();
const hotHistoryCallsBefore = historyCalls.filter((tabID) => tabID === "tab-e").length;
let hotActivation: Promise<TabMeta> | undefined;
await act(async () => {
  hotActivation = controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, true, "cached session keeps backend activation pending without hiding its safe projection");
ok(
  controller?.state.items.some((item) => item.kind === "user" && item.text === "restored expert-team history") ?? false,
  "cached session renders its safe history projection before backend activation resolves",
);
eq(
  historyCalls.filter((tabID) => tabID === "tab-e").length,
  hotHistoryCallsBefore,
  "cached session does not start disk revalidation before backend activation resolves",
);
await act(async () => {
  activateTopicGate?.resolve();
  await hotActivation;
  await flushPromises();
});
activateTopicGate = undefined;
eq(controller?.surfaceTransitionActive, false, "cached session binds without entering a visible hydration state");
eq(controller?.state.running, false, "background turn_done settles its originating tab before reactivation");
ok(
  controller?.state.items.some((item) => item.kind === "user" && item.text === "restored expert-team history") ?? false,
  "cached session renders its safe history projection before disk revalidation",
);
eq(
  historyCalls.filter((tabID) => tabID === "tab-e").length,
  hotHistoryCallsBefore + 1,
  "cached session starts one background history revalidation",
);
const hotItems = controller?.state.items;
await act(async () => {
  historyERevalidationGate?.resolve([userMessage("restored expert-team history")]);
  await historyERevalidationGate?.promise;
  await flushPromises();
});
eq(controller?.state.items, hotItems, "equivalent background history does not publish a new items array");
historyERevalidationGate = undefined;

activateSessionTurnsOverride = 1;
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await controller?.activateTopic(
    tabE.scope,
    tabE.workspaceRoot,
    tabE.topicId || "",
    "saved-expert-team.jsonl",
    undefined,
    1,
  );
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
const versionMatchedHistoryCalls = historyCalls.filter((tabID) => tabID === "tab-e").length;
await act(async () => {
  await controller?.activateTopic(
    tabE.scope,
    tabE.workspaceRoot,
    tabE.topicId || "",
    "saved-expert-team.jsonl",
    undefined,
    1,
  );
  await flushPromises();
});
eq(
  historyCalls.filter((tabID) => tabID === "tab-e").length,
  versionMatchedHistoryCalls,
  "cached session skips disk history revalidation when backend turn count is unchanged",
);
activateSessionTurnsOverride = undefined;

await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
historyERevalidationGate = deferred<HistoryMessage[]>();
await act(async () => {
  await controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
await act(async () => {
  for (const handler of eventHandlers) handler({ kind: "turn_started", tabId: "tab-e" });
  for (const handler of eventHandlers) {
    handler({ kind: "message", text: "live during cached revalidation", reasoning: "", tabId: "tab-e" });
  }
  for (const handler of eventHandlers) handler({ kind: "turn_done", tabId: "tab-e" });
  historyERevalidationGate?.resolve([userMessage("structurally changed disk history")]);
  await historyERevalidationGate?.promise;
  await flushPromises();
});
ok(
  controller?.state.items.some((item) => item.kind === "assistant" && item.text === "live during cached revalidation") ?? false,
  "cached history revalidation preserves a live item received while disk history is pending",
);
eq(
  controller?.state.items.filter((item) => item.kind === "user" && item.text === "restored expert-team history").length,
  1,
  "running cached revalidation does not append a duplicate copy of the old cached history",
);
ok(
  !(controller?.state.items.some((item) => item.kind === "user" && item.text === "structurally changed disk history") ?? false),
  "cached revalidation defers a stale structural response even after the intervening live turn completes",
);
historyERevalidationGate = undefined;

controller?.setSessionViewCacheScope({ actorId: "actor-b", profileId: "profile-b" });
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
historyERevalidationGate = deferred<HistoryMessage[]>();
await act(async () => {
  await controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, true, "profile scope change prevents the old view mapping from being re-archived");
eq(controller?.state.items.length, 0, "profile scope change cannot hit the previous profile snapshot");
await act(async () => {
  historyERevalidationGate?.resolve([userMessage("restored expert-team history")]);
  await historyERevalidationGate?.promise;
  await flushPromises();
});
historyERevalidationGate = undefined;

await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
activateSessionPathOverride = "backend-other-session.jsonl";
historyERevalidationGate = deferred<HistoryMessage[]>();
await act(async () => {
  await controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, true, "cache identity mismatch falls back to the cold transition");
eq(controller?.state.items.length, 0, "cache identity mismatch immediately removes the incompatible cached transcript");
await act(async () => {
  historyERevalidationGate?.resolve([userMessage("other session history")]);
  await historyERevalidationGate?.promise;
  await flushPromises();
});
await waitFor("identity mismatch cold history", () =>
  controller?.state.items.some((item) => item.kind === "user" && item.text === "other session history") ?? false
);
activateSessionPathOverride = "";
historyERevalidationGate = undefined;

await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
runningTabs.add("tab-e");
holdContextE = true;
observeReactivation = true;
let reactivateE: Promise<TabMeta> | undefined;
await act(async () => {
  reactivateE = controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await reactivateE;
  await flushPromises();
});
eq(controller?.activeTabId, "tab-e", "reactivating a deleted surface switches with the latest runtime snapshot");
eq(controller?.state.turnStartAt, 2345, "latest runtime snapshot seeds the turn start before rendering");
eq(controller?.state.turnTokens, 78, "latest runtime snapshot seeds tokens before rendering");
await act(async () => {
  contextETokens = 90;
  contextEGate.resolve({ ...context, activeTurnStartedAt: 2345, activeTurnCompletionTokens: 90 });
  await contextEGate.promise;
  await flushPromises();
});
await waitFor("background token merge", () => controller?.state.turnTokens === 90);
observeReactivation = false;
eq(controller?.activeTabId, "tab-e", "reactivating a deleted surface switches after the active snapshot is seeded");
eq(controller?.state.turnStartAt, 2345, "reactivating a deleted surface presents the backend turn start atomically");
eq(controller?.state.turnTokens, 90, "backend hydration advances cached tokens accumulated in the background");
ok(
  reactivationMetrics.length > 0 && reactivationMetrics.every((metric) => metric.startedAt === 2345 && metric.tokens >= 78),
  "every rendered reactivation state preserves the seeded active metrics",
);
holdContextE = false;

await act(async () => {
  for (const handler of eventHandlers) {
    handler({ kind: "turn_started", tabId: "tab-e", turnId: "retained-turn", activeTurnStartedAt: 2345 });
    handler({ kind: "text", tabId: "tab-e", turnId: "retained-turn", text: "before switching" });
    handler({ kind: "phase", tabId: "tab-e", text: "flush stream batch" });
  }
  await flushPromises();
});
const transcriptBeforeSwitch = controller?.state.items;
const historyCallsBeforeRunningSwitch = historyCalls.filter((id) => id === "tab-e").length;
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
historyERevalidationGate = deferred<HistoryMessage[]>();
await act(async () => {
  for (const handler of eventHandlers) {
    handler({ kind: "text", tabId: "tab-e", turnId: "retained-turn", text: " + in background" });
    handler({ kind: "phase", tabId: "tab-e", text: "background stream flushed" });
  }
  await flushPromises();
});
ok(!controller?.state.live?.text.includes("before switching"), "background output stays out of the foreground task");
await act(async () => {
  await controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, false, "running target binds its complete transcript without waiting for disk");
ok(controller?.state.running ?? false, "running target preserves live runtime status during handoff");
ok(
  transcriptBeforeSwitch?.every((previous) => controller?.state.items.some((item) => item.id === previous.id)) ?? false,
  "running handoff preserves every previously displayed item and its identity",
);
eq(controller?.state.runtimeTurnId, "retained-turn", "switching preserves terminal delivery ownership");
eq(controller?.state.hydrateHistoryLoaded, true, "switching keeps the full transcript eligible for subsequent restoration");
eq(controller?.state.live?.text, "before switching + in background", "switching preserves and continues streaming output");
eq(historyCalls.filter((id) => id === "tab-e").length, historyCallsBeforeRunningSwitch, "complete running transcript does not reload older disk history");
await act(async () => {
  for (const handler of eventHandlers) {
    handler({ kind: "phase", text: "live during runtime-tail hydration", tabId: "tab-e" });
  }
  historyERevalidationGate?.resolve([userMessage("restored expert-team history")]);
  await historyERevalidationGate?.promise;
  await flushPromises();
});
ok(
  controller?.state.items.some((item) => item.kind === "phase" && item.text === "live during runtime-tail hydration") ?? false,
  "retained transcript preserves events without depending on a disk history response",
);
historyERevalidationGate = undefined;

await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
eq(controller?.state.live?.text, "before switching + in background", "a second round trip keeps the same stream");
eq(controller?.state.runtimeTurnId, "retained-turn", "a second round trip keeps the turn identity");
eq(historyCalls.filter((id) => id === "tab-e").length, historyCallsBeforeRunningSwitch, "repeated switching never replaces the complete transcript from disk");

await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
holdListTabs = true;
const loadingClock = installWindowFakeClock();
const stoppingComposerIdentity: ComposerSessionIdentity = {
  modelRef: "provider/model-target",
  billingSource: "newapi_byok",
  effort: "high",
  expertKnown: true,
  expertId: "team-target",
  expertName: "Target Team",
  collaborationMode: "goal",
  toolApprovalMode: "auto",
  tokenMode: "economy",
  goal: "ship target",
  goalStatus: "running",
};
let activateStoppingE: Promise<TabMeta> | undefined;
await act(async () => {
  activateStoppingE = controller?.activateTopic(
    tabE.scope,
    tabE.workspaceRoot,
    tabE.topicId || "",
    "uncached-stopping.jsonl",
    undefined,
    undefined,
    undefined,
    undefined,
    stoppingComposerIdentity,
  );
  await flushPromises();
});
eq(controller?.activeTabId, "tab-f", "stopping target remains hidden while its active snapshot is pending");
eq(controller?.state.items.length, 0, "surface transition hides the previous transcript before runtime snapshot resolves");
eq(controller?.state.backendActivationPending, true, "surface transition gates the old runtime before target binding");
eq(controller?.state.meta?.modelRef, "provider/model-target", "surface transition publishes the target model in its first frame");
eq(controller?.state.composerSelection.effortSnapshot, "high", "surface transition publishes the matching target effort in its first frame");
eq(controller?.state.meta?.collaborationMode, "goal", "surface transition publishes the target collaboration mode in its first frame");
eq(controller?.state.meta?.toolApprovalMode, "auto", "surface transition publishes the target approval mode in its first frame");
eq(controller?.state.meta?.tokenMode, "economy", "surface transition publishes the target token mode in its first frame");
eq(controller?.state.meta?.goal, "ship target", "surface transition publishes the target goal in its first frame");
eq(controller?.renderedViewTarget?.composerIdentity?.expertId, "team-target", "surface transition publishes the target expert team in its first frame");
eq(controller?.surfaceLoadingHintVisible, false, "surface transition starts with its loading hint hidden");
const submitsBeforePendingSend = submitCalls;
await act(async () => {
  await controller?.send("must not reach old runtime").catch(() => {});
  await flushPromises();
});
eq(submitCalls, submitsBeforePendingSend, "surface transition rejects sends instead of targeting the previous runtime");
await act(async () => {
  loadingClock.advance(119);
  await flushPromises();
});
eq(controller?.surfaceLoadingHintVisible, false, "surface transition keeps the first 119ms visually quiet");
await act(async () => {
  loadingClock.advance(1);
  await flushPromises();
});
eq(controller?.surfaceLoadingHintVisible, true, "surface transition reveals its loading hint exactly at 120ms");
runningTabs.delete("tab-e");
await act(async () => {
  listTabsGate.resolve();
  await listTabsGate.promise;
  await activateStoppingE;
  await flushPromises();
});
eq(controller?.activeTabId, "tab-e", "stopped target activation still completes");
eq(controller?.state.running, false, "latest runtime status wins when the turn stops during pre-hydration");
eq(controller?.state.turnStartAt, 0, "stopped target cannot revive the stale active turn start");
eq(controller?.state.turnTokens, 0, "stopped target cannot revive stale active turn tokens");
eq(controller?.surfaceLoadingHintVisible, false, "surface completion hides the delayed loading hint");
eq(loadingClock.pending(), 0, "surface completion cancels the target loading timer");
loadingClock.restore();
holdListTabs = false;
runningTabs.add("tab-e");

await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});
let cancelledTransitionId: number | undefined;
await act(async () => {
  cancelledTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "cancelled-topic",
    sessionPath: "/repo/tab-f/cancelled.jsonl",
  });
  await flushPromises();
});
eq(controller?.state.surfaceIntent, "unknown", "path identity without exact turns remains unknown");
await act(async () => {
  if (cancelledTransitionId !== undefined) controller?.cancelSurfaceTransition(cancelledTransitionId);
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, false, "cancelling the prepared path intent restores the source surface");
eq(controller?.state.surfaceIntent, "blank", "cancelled path intent restores the blank source layout");

activateSessionPathOverride = "/repo/tab-f/backend-one-from-old-zero.jsonl";
activateSessionTurnsOverride = 1;
zeroTurnHistoryGate = deferred<HistoryMessage[]>();
let correctedOldZeroTransitionId: number | undefined;
let correctedOldZeroActivation: Promise<TabMeta> | undefined;
let correctedOldZeroMeta: TabMeta | undefined;
await act(async () => {
  correctedOldZeroTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "backend-one-from-old-zero",
    sessionPath: activateSessionPathOverride,
    sessionTurns: 0,
  });
  if (correctedOldZeroTransitionId !== undefined) {
    correctedOldZeroActivation = controller?.activateTopic(
      "project",
      tabF.workspaceRoot,
      "backend-one-from-old-zero",
      activateSessionPathOverride,
      correctedOldZeroTransitionId,
      0,
    );
  }
  correctedOldZeroMeta = await correctedOldZeroActivation;
  await flushPromises();
});
eq(correctedOldZeroMeta?.sessionTurns, 1, "ActivateTopic returns exact one for a stale zero-turn click target");
eq(controller?.renderedViewTarget?.sessionTurns, 1, "backend exact one replaces a stale zero-turn click target");
eq(controller?.state.surfaceIntent, "existing-session", "backend exact one corrects stale blank intent to existing");
await act(async () => {
  zeroTurnHistoryGate?.resolve([userMessage("backend corrected old zero")]);
  await correctedOldZeroActivation;
  await flushPromises();
});
await waitFor("backend corrected old zero hydration", () => controller?.surfaceTransitionActive === false);
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});

activateSessionPathOverride = "/repo/tab-f/backend-one-from-missing.jsonl";
zeroTurnHistoryGate = deferred<HistoryMessage[]>();
let correctedMissingTransitionId: number | undefined;
let correctedMissingActivation: Promise<TabMeta> | undefined;
let correctedMissingMeta: TabMeta | undefined;
await act(async () => {
  correctedMissingTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "backend-one-from-missing",
    sessionPath: activateSessionPathOverride,
  });
  if (correctedMissingTransitionId !== undefined) {
    correctedMissingActivation = controller?.activateTopic(
      "project",
      tabF.workspaceRoot,
      "backend-one-from-missing",
      activateSessionPathOverride,
      correctedMissingTransitionId,
    );
  }
  correctedMissingMeta = await correctedMissingActivation;
  await flushPromises();
});
eq(correctedMissingMeta?.sessionTurns, 1, "ActivateTopic returns exact one for a missing click target summary");
eq(controller?.renderedViewTarget?.sessionTurns, 1, "backend exact one fills a missing click target summary");
eq(controller?.state.surfaceIntent, "existing-session", "backend exact one corrects unknown intent to existing");
await act(async () => {
  zeroTurnHistoryGate?.resolve([userMessage("backend corrected missing turns")]);
  await correctedMissingActivation;
  await flushPromises();
});
await waitFor("backend corrected missing hydration", () => controller?.surfaceTransitionActive === false);
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});

activateSessionPathOverride = "/repo/tab-f/zero-turn.jsonl";
activateSessionTurnsOverride = 0;
activateSceneOverride = "design";
activateSceneStateOverride = "draft";
zeroTurnHistoryGate = deferred<HistoryMessage[]>();
let zeroTurnTransitionId: number | undefined;
let zeroTurnActivation: Promise<TabMeta> | undefined;
let zeroTurnMeta: TabMeta | undefined;
await act(async () => {
  zeroTurnTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "zero-turn-topic",
    sessionPath: "/repo/tab-f/zero-turn.jsonl",
    sessionTurns: 1,
    scene: "coding",
    sceneState: "draft",
  });
  await flushPromises();
});
eq(controller?.renderedViewTarget?.scene, "coding", "prepared zero-turn target exposes its exact scene before backend activation");
eq(controller?.renderedViewTarget?.sceneState, "draft", "prepared zero-turn target exposes its exact scene state before backend activation");
await act(async () => {
  if (zeroTurnTransitionId !== undefined) {
    zeroTurnActivation = controller?.activateTopic(
      "project",
      tabF.workspaceRoot,
      "zero-turn-topic",
      "/repo/tab-f/zero-turn.jsonl",
      zeroTurnTransitionId,
      1,
    );
  }
  zeroTurnMeta = await zeroTurnActivation;
  await flushPromises();
});
eq(zeroTurnMeta?.sessionTurns, 0, "ActivateTopic returns explicit zero for a stale nonzero click target");
eq(controller?.renderedViewTarget?.sessionTurns, 0, "backend explicit zero replaces a stale nonzero click target");
eq(controller?.renderedViewTarget?.scene, "design", "runtime scene overrides the prepared project-tree summary");
eq(controller?.renderedViewTarget?.sceneState, "draft", "runtime scene state overrides the prepared project-tree summary");
eq(controller?.state.surfaceIntent, "blank", "backend explicit zero corrects stale existing intent to blank");
eq(controller?.state.hydrating, true, "zero-turn activation hydrates without leaving the blank surface");
eq(controller?.surfaceTransitionActive, true, "zero-turn transition remains active until empty history resolves");
await act(async () => {
  zeroTurnHistoryGate?.resolve([]);
  await zeroTurnActivation;
  await flushPromises();
});
eq(controller?.state.surfaceIntent, "blank", "empty history preserves the exact zero-turn blank surface");
eq(controller?.surfaceTransitionActive, false, "empty zero-turn hydration completes the prepared transition");
zeroTurnHistoryGate = undefined;
activateSessionPathOverride = "";
activateSessionTurnsOverride = undefined;
activateSceneOverride = undefined;
activateSceneStateOverride = undefined;
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot);
  await flushPromises();
});

let staleTransitionId: number | undefined;
let latestTransitionId: number | undefined;
await act(async () => {
  staleTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "stale-topic",
    sessionPath: "/repo/tab-f/stale.jsonl",
    sessionTurns: 1,
  });
  latestTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "latest-topic",
    sessionPath: "/repo/tab-f/latest.jsonl",
    sessionTurns: 1,
  });
  if (staleTransitionId !== undefined) controller?.cancelSurfaceTransition(staleTransitionId);
  await flushPromises();
});
eq(controller?.renderedViewTarget?.topicId, "latest-topic", "stale intent cancellation cannot clear the latest target");
await act(async () => {
  if (latestTransitionId !== undefined) controller?.cancelSurfaceTransition(latestTransitionId);
  await flushPromises();
});
eq(controller?.state.surfaceIntent, "blank", "matching latest intent cancellation restores the blank source");

resumeHistoryGate = deferred<HistoryMessage[]>();
let pathTransitionId: number | undefined;
await act(async () => {
  pathTransitionId = controller?.beginSurfaceTransition({
    scope: "project",
    workspaceRoot: tabF.workspaceRoot,
    topicId: "",
    sessionPath: "/repo/tab-f/existing.jsonl",
    sessionTurns: 1,
  });
  await flushPromises();
});
const transitionComposerKey = composerDraftKeyForTab(controller?.renderedViewTarget);
eq(controller?.state.surfaceIntent, "existing-session", "path transition classifies the target as existing from its first rendered frame");
eq(controller?.state.items.length, 0, "path transition does not reveal the source transcript");
await act(async () => {
  await controller?.ensureBlankSurface("project", tabF.workspaceRoot, pathTransitionId);
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, true, "blank runtime creation keeps the target transition visible");
eq(controller?.state.surfaceIntent, "existing-session", "blank runtime creation cannot replace the pending target intent");
let resumeExisting: Promise<void> | undefined;
await act(async () => {
  resumeExisting = controller?.resumeSession("/repo/tab-f/existing.jsonl", "tab-f", pathTransitionId);
  await flushPromises();
});
const submitsBeforeBackgroundSend = submitCalls;
await act(async () => {
  await controller?.sendToTab("tab-c", "background send remains scoped");
  await flushPromises();
});
eq(submitCalls, submitsBeforeBackgroundSend + 1, "explicit background sendToTab remains available during a rendered-surface transition");
const submitsBeforeExistingSend = submitCalls;
await act(async () => {
  await controller?.send("must wait for existing history").catch(() => {});
  await flushPromises();
});
eq(submitCalls, submitsBeforeExistingSend, "current rendered surface cannot send before existing history is ready");
await act(async () => {
  resumeHistoryGate?.resolve([userMessage("loaded existing path")]);
  await resumeExisting;
  await flushPromises();
});
eq(controller?.surfaceTransitionActive, false, "path history completion ends the matching surface transition");
eq(controller?.state.surfaceIntent, "existing-session", "loaded path preserves existing-session layout intent");
eq(controller?.renderedViewTarget?.sessionPath, "/repo/tab-f/existing.jsonl", "path history keeps the target identity after transition completion");
ok(controller?.sessionViewKey.includes("path:/repo/tab-f/existing.jsonl") ?? false, "path history keeps the target session view and scroll-cache identity");
eq(composerDraftKeyForTab(controller?.renderedViewTarget), transitionComposerKey, "Composer target key does not switch when path activation completes");
eq(controller?.state.meta?.sessionPath, "/repo/tab-f/existing.jsonl", "path history immediately publishes the target session path in controller metadata");
ok(
  controller?.state.items.some((item) => item.kind === "user" && item.text === "loaded existing path") ?? false,
  "path history replaces the loading surface",
);
resumeHistoryGate = undefined;

await act(async () => {
  for (const handler of eventHandlers) {
    handler({ kind: "turn_started", tabId: "tab-f", turnId: "turn-previous" });
    handler({
      kind: "message",
      tabId: "tab-f",
      turnId: "turn-previous",
      candidateId: "candidate-previous",
      text: "previous session answer",
    });
    handler({ kind: "turn_started", tabId: "tab-f", turnId: "turn-current" });
    handler({
      kind: "message",
      tabId: "tab-f",
      turnId: "turn-current",
      candidateId: "candidate-current",
      text: "current session answer",
    });
  }
  await flushPromises();
});
eq(controller?.state.running, true, "identified current turn remains active before its terminal event");
await act(async () => {
  for (const handler of turnBillingHandlers) {
    handler({ taskId: "task-shared-terminal-test", chargedCredits: "7" });
  }
  await flushPromises();
});
const terminalRefreshCallsBeforeStale = balanceCalls + jobsCalls + checkpointCalls;
await act(async () => {
  for (const handler of eventHandlers) {
    handler({
      kind: "turn_done",
      tabId: "tab-f",
      conversationSessionPath: "/repo/tab-f/stale.jsonl",
      turnId: "turn-current",
      artifactDelivery: { version: 1, paths: ["stale-session.pdf"] },
    });
    handler({
      kind: "turn_done",
      tabId: "tab-f",
      conversationSessionPath: "/repo/tab-f/existing.jsonl",
      turnId: "turn-stale",
      artifactDelivery: { version: 1, paths: ["stale-turn.pdf"] },
    });
    handler({
      kind: "turn_done",
      tabId: "tab-f",
      conversationSessionPath: "/repo/tab-f/existing.jsonl",
      artifactDelivery: { version: 1, paths: ["identityless-stale.pdf"] },
    });
    handler({
      kind: "turn_done",
      tabId: "tab-f",
      conversationSessionPath: "/repo/tab-f/existing.jsonl",
      turnId: "turn-previous",
      artifactDelivery: { version: 1, paths: ["previous.pdf"] },
      turn: { taskId: "task-shared-terminal-test", modelRef: "provider/previous", billingSource: "newapi_byok" },
    });
  }
  await flushPromises();
});
eq(controller?.state.running, true, "stale session and turn terminals cannot settle the current turn");
eq(
  balanceCalls + jobsCalls + checkpointCalls,
  terminalRefreshCallsBeforeStale,
  "stale terminals do not trigger completion refresh side effects",
);
const previousAnswerAfterLateDone = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "previous session answer",
);
eq(
  JSON.stringify(previousAnswerAfterLateDone?.kind === "assistant" ? previousAnswerAfterLateDone.artifactDelivery?.paths : undefined),
  JSON.stringify(["previous.pdf"]),
  "a late successful terminal binds its contract to the completed historical turn",
);
eq(
  previousAnswerAfterLateDone?.kind === "assistant" ? previousAnswerAfterLateDone.finalAnswer : undefined,
  true,
  "a late successful terminal commits only its historical final answer",
);
eq(
  previousAnswerAfterLateDone?.kind === "assistant" ? previousAnswerAfterLateDone.turn : undefined,
  undefined,
  "a historical terminal does not project billing metadata",
);
await act(async () => {
  for (const handler of eventHandlers) {
    handler({
      kind: "turn_done",
      tabId: "tab-f",
      conversationSessionPath: "/repo/tab-f/existing.jsonl",
      turnId: "turn-previous",
      artifactDelivery: { version: 1, paths: ["conflicting-replay.pdf"] },
    });
  }
  await flushPromises();
});
const previousAnswerAfterReplay = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "previous session answer",
);
eq(
  JSON.stringify(previousAnswerAfterReplay?.kind === "assistant" ? previousAnswerAfterReplay.artifactDelivery?.paths : undefined),
  JSON.stringify(["previous.pdf"]),
  "a replayed historical terminal cannot overwrite the first committed contract",
);
eq(
  balanceCalls + jobsCalls + checkpointCalls,
  terminalRefreshCallsBeforeStale,
  "historical terminal replays remain free of completion refresh side effects",
);
const currentAnswerBeforeDone = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "current session answer",
);
eq(
  currentAnswerBeforeDone?.kind === "assistant" ? currentAnswerBeforeDone.artifactDelivery : undefined,
  undefined,
  "stale terminals cannot bind their artifact contract",
);

await act(async () => {
  for (const handler of eventHandlers) {
    handler({
      kind: "turn_done",
      tabId: "tab-f",
      conversationSessionPath: "/repo/tab-f/existing.jsonl",
      turnId: "turn-current",
      artifactDelivery: { version: 1, paths: ["current.pdf"] },
      turn: { taskId: "task-shared-terminal-test", modelRef: "provider/current", billingSource: "newapi_byok" },
    });
  }
  await flushPromises();
});
eq(controller?.state.running, false, "matching session and turn terminal settles the current turn");
ok(
  balanceCalls + jobsCalls + checkpointCalls > terminalRefreshCallsBeforeStale,
  "matching terminal triggers completion refresh side effects",
);
const currentAnswerAfterDone = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "current session answer",
);
eq(
  JSON.stringify(currentAnswerAfterDone?.kind === "assistant" ? currentAnswerAfterDone.artifactDelivery?.paths : undefined),
  JSON.stringify(["current.pdf"]),
  "matching terminal binds the artifact contract",
);
eq(
  currentAnswerAfterDone?.kind === "assistant" ? currentAnswerAfterDone.turn?.chargedCredits : undefined,
  "7",
  "a historical terminal cannot consume pending billing for the matching current terminal",
);

await act(async () => {
  for (const handler of eventHandlers) {
    handler({ kind: "turn_started", tabId: "tab-f" });
    handler({
      kind: "message",
      tabId: "tab-f",
      text: "legacy terminal answer",
    });
    handler({ kind: "turn_done", tabId: "tab-f", turnId: "foreign-turn", artifactDelivery: { version: 1, paths: ["foreign.pdf"] } });
  }
  await flushPromises();
});
eq(controller?.state.running, true, "a contract terminal cannot settle an unidentified active turn");
const legacyAnswer = controller?.state.items.find(
  (item) => item.kind === "assistant" && item.text === "legacy terminal answer",
);
eq(
  legacyAnswer?.kind === "assistant" ? legacyAnswer.artifactDelivery : undefined,
  undefined,
  "an unmatched contract terminal cannot bind to an unidentified assistant",
);
await act(async () => {
  for (const handler of eventHandlers) handler({ kind: "turn_done", tabId: "tab-f" });
  await flushPromises();
});
eq(controller?.state.running, false, "an identity-free legacy terminal without a contract remains backward compatible");

holdRaceListTabs = true;
const switchClock = installWindowFakeClock();
let staleActivation: Promise<TabMeta> | undefined;
await act(async () => {
  staleActivation = controller?.activateTopic(tabE.scope, tabE.workspaceRoot, tabE.topicId || "", "saved-expert-team.jsonl");
  await flushPromises();
});
await act(async () => {
  await controller?.openProjectTab(tabD.workspaceRoot, tabD.topicId || "");
  await flushPromises();
});
eq(controller?.activeTabId, "tab-d", "a newer openProjectTab supersedes a pending surface activation");
await act(async () => {
  switchClock.advance(120);
  await flushPromises();
});
eq(controller?.surfaceLoadingHintVisible, false, "switching targets cancels the stale loading hint before its threshold");
eq(switchClock.pending(), 0, "switching targets clears the stale target timer");
await act(async () => {
  raceListTabsGate.resolve();
  await raceListTabsGate.promise;
  await staleActivation;
  await flushPromises();
});
eq(controller?.activeTabId, "tab-d", "released stale surface activation cannot overwrite the newer active tab");
switchClock.restore();
holdRaceListTabs = false;
backendActiveId = "tab-e";

await act(async () => {
  root.unmount();
});
controller = undefined;
const reloadedRoot = createRoot(rootEl);
await act(async () => {
  reloadedRoot.render(<Probe />);
  await flushPromises();
});
await waitFor("reloaded active metrics", () => controller?.activeTabId === "tab-e" && controller.state.turnTokens === 90);
eq((controller as Controller | undefined)?.state.turnStartAt, 2345, "controller rebuild restores the backend turn start");
eq((controller as Controller | undefined)?.state.turnTokens, 90, "controller rebuild restores the backend turn tokens");
await act(async () => {
  reloadedRoot.unmount();
});

backendActiveId = "tab-g";
controller = undefined;
const zeroTurnReloadRoot = createRoot(rootEl);
await act(async () => {
  zeroTurnReloadRoot.render(<Probe />);
  await flushPromises();
});
await waitFor(
  "restored zero-turn first frame",
  () => controller?.activeTabId === "tab-g" && controller.state.hydrating,
);
eq(
  (controller as Controller | undefined)?.state.surfaceIntent,
  "blank",
  "restored exact zero-turn tab renders the blank surface before history resolves",
);
eq(
  (controller as Controller | undefined)?.renderedViewTarget?.sessionTurns,
  0,
  "restored zero-turn tab preserves explicit zero in the first ListTabs snapshot",
);
eq(
  (controller as Controller | undefined)?.state.meta?.modelRef,
  "provider/restarted-model",
  "restored tab metadata publishes the persisted model in its first frame",
);
eq(
  (controller as Controller | undefined)?.state.composerSelection.effortSnapshot,
  "low",
  "restored tab metadata publishes the matching persisted effort in the same frame",
);
await act(async () => {
  effortGGate.resolve({ ...effort, current: "low" });
  historyGGate.resolve([]);
  await historyGGate.promise;
  await flushPromises();
});
await waitFor(
  "restored zero-turn hydration",
  () => controller?.activeTabId === "tab-g" && controller.state.hydrating === false,
);
eq(
  (controller as Controller | undefined)?.state.surfaceIntent,
  "blank",
  "restored zero-turn tab remains blank after empty history hydration",
);
await act(async () => {
  zeroTurnReloadRoot.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
