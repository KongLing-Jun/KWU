// Run: tsx src/__tests__/new-session-load-race.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { initialState, reducer, useController, type Item } from "../lib/useController";
import type { AppBindings } from "../lib/bridge";
import type { BalanceInfo, CheckpointMeta, ContextInfo, EffortInfo, HistoryMessage, JobView, Meta, TabMeta } from "../lib/types";

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

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function tabMeta(overrides: Partial<TabMeta> = {}): TabMeta {
  return {
    id: "tab-a",
    scope: "project",
    workspaceRoot: "/repo",
    workspaceName: "repo",
    workspacePath: "/repo",
    gitBranch: "main",
    topicId: "topic-a",
    topicTitle: "General",
    label: "model",
    ready: true,
    running: false,
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
    gitBranch: "main",
    autoApproveTools: false,
    bypass: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    goal: "",
    goalStatus: "stopped",
  };
}

console.log("\nnew session load race");

const resetSourceItems: Item[] = [{ kind: "user", id: "old-user", text: "old prompt" }];
const resetPlaceholderItems: Item[] = [{ kind: "user", id: "placeholder-user", text: "placeholder prompt" }];
const resetState = reducer(
  {
    ...initialState,
    items: resetSourceItems,
    hydrating: true,
    surfaceIntent: "existing-session",
    hydrateReason: "open-topic",
    hydratePlaceholderItems: resetPlaceholderItems,
  },
  { type: "reset" },
);
eq(resetState.items.length, 0, "reset clears real transcript items");
eq(resetState.hydratePlaceholderItems?.length, 1, "reset preserves hydration placeholder separately");
eq(resetState.surfaceIntent, "existing-session", "reset preserves the current surface intent");

const emptyHistoryState = reducer(resetState, { type: "history", messages: [] });
eq(emptyHistoryState.items.length, 0, "empty history keeps the real transcript empty");
eq(emptyHistoryState.hydrateHistoryLoaded, true, "empty history marks transcript hydration loaded");
eq(emptyHistoryState.hydratePlaceholderItems?.length ?? 0, 0, "empty history clears hydration placeholder items");

const hydrateDoneState = reducer(emptyHistoryState, { type: "hydrate_done" });
eq(hydrateDoneState.hydrateHistoryLoaded, true, "hydrate_done preserves successful history evidence");
eq(hydrateDoneState.surfaceIntent, "existing-session", "history completion preserves the target surface intent");
const nextHydrateState = reducer(hydrateDoneState, { type: "hydrate_start", reason: "switch-tab", surfaceIntent: "blank" });
eq(nextHydrateState.hydrateHistoryLoaded, false, "the next hydration invalidates prior history evidence");
eq(nextHydrateState.surfaceIntent, "blank", "a new navigation atomically replaces the old surface intent");
const failedExistingState = reducer(
  nextHydrateState,
  { type: "hydrate_start", reason: "resume-session", surfaceIntent: "existing-session" },
);
eq(
  reducer(failedExistingState, { type: "hydrate_error", reason: "resume-session", error: "disk" }).surfaceIntent,
  "existing-session",
  "history failure preserves existing-session layout semantics",
);
const firstTurnState = reducer(
  { ...initialState, surfaceIntent: "blank" },
  { type: "user", text: "first cloud project turn", seq: 0 },
);
eq(firstTurnState.surfaceIntent, "existing-session", "the first visible user turn atomically promotes a blank surface");
eq(firstTurnState.items.length, 1, "surface promotion preserves the optimistic first user turn");

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

const staleHistory = deferred<HistoryMessage[]>();
let newSessionCalls = 0;
let clearSessionCalls = 0;
const context: ContextInfo = { used: 12, window: 100, sessionTokens: 12 };
const effort: EffortInfo = { supported: true, current: "auto", default: "auto", levels: ["auto"] };
const balance: BalanceInfo = { available: false, display: "" };
const jobs: JobView[] = [];
const checkpoints: CheckpointMeta[] = [];

window.runtime = {
  EventsOn: () => () => {},
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      ListTabs: async () => [tabMeta()],
      MetaForTab: async () => meta(),
      ContextUsageForTab: async () => context,
      EffortForTab: async () => effort,
      BalanceForTab: async () => balance,
      JobsForTab: async () => jobs,
      CheckpointsForTab: async () => checkpoints,
      HistoryForTab: async () => staleHistory.promise,
      ReplayPendingPrompts: async () => {},
      ResumeSessionForTab: async () => [{ role: "user", content: "existing prompt" }],
      ClearSession: async () => {
        clearSessionCalls += 1;
      },
      NewSession: async () => {
        newSessionCalls += 1;
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

function currentController(): Controller {
  if (!controller) throw new Error("controller did not render");
  return controller;
}

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);

await act(async () => {
  root.render(<Probe />);
  await flushPromises();
});
await waitFor("active tab", () => controller?.activeTabId === "tab-a");

await act(async () => {
  await controller?.resumeSession("/repo/existing.jsonl", "tab-a");
  await flushPromises();
});
eq(controller?.state.surfaceIntent, "existing-session", "resumed history establishes existing-session intent before clear");
eq(controller?.state.items.length, 1, "resumed history is visible before clear");
await act(async () => {
  await controller?.clearSession();
  await flushPromises();
});
eq(clearSessionCalls, 1, "ClearSession is called once");
eq(controller?.state.surfaceIntent, "blank", "clearing an existing session restores blank intent");
eq(controller?.state.items.length, 0, "clearing an existing session removes its transcript");
ok(!controller?.state.hydrating && !controller?.state.hydrateError, "cleared blank state remains eligible for the Welcome layout");

await act(async () => {
  await controller?.newSession();
  await flushPromises();
});
eq(newSessionCalls, 1, "NewSession is called once");
eq(controller?.state.items.length, 0, "new session clears the visible transcript");
eq(controller?.state.surfaceIntent, "blank", "new session explicitly restores blank surface intent");

await act(async () => {
  staleHistory.resolve([{ role: "user", content: "old prompt" }]);
  await staleHistory.promise;
  await flushPromises();
});

eq(controller?.state.items.length, 0, "stale history load cannot repopulate a new blank session");

await act(async () => {
  root.unmount();
});

const guardedStartupTabs = deferred<TabMeta[]>();
let ensureBlankSurfaceCalls = 0;
window.go = {
  main: {
    App: {
      ListTabs: async () => guardedStartupTabs.promise,
      MetaForTab: async () => meta(),
      ContextUsageForTab: async () => context,
      EffortForTab: async () => effort,
      BalanceForTab: async () => balance,
      JobsForTab: async () => jobs,
      CheckpointsForTab: async () => checkpoints,
      HistoryForTab: async () => [],
      ReplayPendingPrompts: async () => {},
      EnsureBlankSurface: async () => {
        ensureBlankSurfaceCalls += 1;
        return tabMeta({ id: "tab-new", topicId: "topic-new", topicTitle: "New session" });
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

controller = undefined;
const guardRoot = createRoot(rootEl);

await act(async () => {
  guardRoot.render(<Probe />);
  await flushPromises();
});

await act(async () => {
  await controller?.ensureBlankSurface("project", "/repo");
  await flushPromises();
});

eq(ensureBlankSurfaceCalls, 1, "EnsureBlankSurface is called once");
eq(currentController().activeTabId, "tab-new", "blank surface becomes active before startup sync resolves");
eq(currentController().state.surfaceIntent, "blank", "blank surface hydration remains classified as blank");

await act(async () => {
  guardedStartupTabs.resolve([tabMeta({ id: "tab-old", topicId: "topic-old", topicTitle: "Old session" })]);
  await guardedStartupTabs.promise;
  await flushPromises();
});

eq(currentController().activeTabId, "tab-new", "guarded startup sync cannot restore an older active tab");

await act(async () => {
  guardRoot.unmount();
});

let cloudProjectSubmitCalls = 0;
let cloudProjectActivationCalls = 0;
const cloudProjectHistory = deferred<HistoryMessage[]>();
const cloudProjectTab = tabMeta({
  id: "tab-cloud-project",
  topicId: "topic-cloud-project",
  topicTitle: "Joined cloud project task",
  sessionPath: "/cloud/joined-project/session.jsonl",
  sessionTurns: 0,
});
window.go = {
  main: {
    App: {
      ListTabs: async () => [cloudProjectTab],
      SetActiveTab: async () => {
        cloudProjectActivationCalls += 1;
      },
      MetaForTab: async () => ({ ...meta(), sessionPath: cloudProjectTab.sessionPath }),
      ContextUsageForTab: async () => context,
      EffortForTab: async () => effort,
      BalanceForTab: async () => balance,
      JobsForTab: async () => jobs,
      CheckpointsForTab: async () => checkpoints,
      HistoryForTab: async () => cloudProjectHistory.promise,
      ReplayPendingPrompts: async () => {},
      SubmitToTab: async () => {
        cloudProjectSubmitCalls += 1;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

controller = undefined;
const cloudProjectRoot = createRoot(rootEl);
await act(async () => {
  cloudProjectRoot.render(<Probe />);
  await flushPromises();
});
await waitFor("joined cloud project tab", () => controller?.activeTabId === cloudProjectTab.id);
await act(async () => {
  await currentController().switchTab(cloudProjectTab.id, cloudProjectTab);
  await flushPromises();
});
eq(cloudProjectActivationCalls, 1, "the reconciled cloud project Tab is activated once before its first send");
eq(currentController().state.surfaceIntent, "blank", "activating a newly created zero-turn cloud project session starts hydrating on the blank surface");
eq(currentController().state.hydrating, true, "the cloud project history is still loading when the first send becomes available");

await act(async () => {
  await currentController().sendToTab(cloudProjectTab.id, "joined project first turn");
  await flushPromises();
});
eq(currentController().state.surfaceIntent, "existing-session", "the first cloud project send leaves the blank presentation during hydration");
eq(currentController().state.items[0]?.kind, "user", "the promoted session immediately renders the submitted user turn before history resolves");
eq(currentController().activeTabId, cloudProjectTab.id, "surface promotion keeps the created cloud project tab active");
eq(currentController().state.meta?.sessionPath, cloudProjectTab.sessionPath, "surface promotion reuses the created cloud project session identity");
eq(cloudProjectSubmitCalls, 1, "surface promotion submits the cloud project turn exactly once");
eq(cloudProjectActivationCalls, 1, "surface promotion does not reactivate the cloud project Tab");

await act(async () => {
  cloudProjectHistory.resolve([]);
  await cloudProjectHistory.promise;
  await flushPromises();
});
await waitFor("cloud project hydration", () => !currentController().state.hydrating);
eq(currentController().state.surfaceIntent, "existing-session", "late zero-turn history cannot restore the Welcome presentation");
eq(currentController().state.items.filter((item) => item.kind === "user").length, 1, "late history preserves exactly one optimistic user turn");
eq(cloudProjectSubmitCalls, 1, "history reconciliation does not resubmit the cloud project turn");
eq(cloudProjectActivationCalls, 1, "history reconciliation does not reactivate the cloud project Tab");

await act(async () => {
  cloudProjectRoot.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
