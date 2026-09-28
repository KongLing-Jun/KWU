// Run: tsx src/__tests__/composer-selection-atomic.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import type { AppBindings } from "../lib/bridge";
import { useController } from "../lib/useController";
import type { ContextInfo, EffortInfo, Meta, TabMeta } from "../lib/types";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
}

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
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

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
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

function tab(modelRef: string): TabMeta {
  return {
    id: "tab-a",
    scope: "project",
    workspaceRoot: "/repo",
    workspaceName: "repo",
    workspacePath: "/repo",
    gitBranch: "main",
    topicId: "topic-a",
    topicTitle: "Atomic selection",
    label: modelRef,
    modelRef,
    billingSource: "newapi_byok",
    ready: true,
    running: false,
    mode: "normal",
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    active: true,
    cwd: "/repo",
  };
}

function meta(modelRef: string): Meta {
  return {
    label: modelRef,
    modelRef,
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

function effort(current: string, supported = true): EffortInfo {
  return { supported, current, default: "auto", levels: supported ? ["auto", "low", "high"] : [] };
}

console.log("\ncomposer selection atomic presentation");

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
ok(
  appSource.includes("shouldHoldComposerSelectionPresentation({")
    && appSource.includes("selectionModelRef: state.composerSelection.modelRef")
    && appSource.includes("presentedModelRef: composerModelLabel")
    && appSource.includes("modelSwitchPending={sessionComposerSelectionBusy}")
    && appSource.includes("modelSelectionPresentation={sessionComposerModelSelectionPresentation}")
    && appSource.includes("preserveComposerSnapshotDuringEntitlementPending")
    && appSource.includes("effortSnapshot={sessionComposerModelSelectionPresentation")
    && appSource.includes("previousEffort: state.effort")
    && appSource.includes("modelSwitcherPresentation.effort")
    && appSource.includes('state.composerSelection.phase === "updating-effort"'),
  "App preserves reliable existing-session snapshots while incomplete selections and explicit switches remain guarded",
);

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

const startupMeta = deferred<Meta>();
const startupEffort = deferred<EffortInfo>();
const startupContext = deferred<ContextInfo>();
const metaQueue: Array<Promise<Meta>> = [startupMeta.promise];
const effortQueue: Array<Promise<EffortInfo>> = [startupEffort.promise];
let currentModel = "provider/a";
let currentEffort = "high";
let modelSetGate: ReturnType<typeof deferred<void>> | undefined;
let effortSetGate: ReturnType<typeof deferred<void>> | undefined;
let failNextModelSet = false;
let failNextEffortSet = false;
let contextPending = true;

window.runtime = {
  EventsOn: () => () => {},
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      ListTabs: async () => [tab(currentModel)],
      MetaForTab: async () => metaQueue.shift() ?? meta(currentModel),
      EffortForTab: async () => effortQueue.shift() ?? effort(currentEffort),
      ContextUsageForTab: async () => contextPending
        ? startupContext.promise
        : { used: 12, window: 100, sessionTokens: 12 },
      BalanceForTab: async () => ({ available: false, display: "" }),
      JobsForTab: async () => [],
      CheckpointsForTab: async () => [],
      HistoryForTab: async () => [],
      ReplayPendingPrompts: async () => {},
      SetModelForTab: async (_tabId: string, modelRef: string) => {
        if (failNextModelSet) {
          failNextModelSet = false;
          throw new Error("model switch failed");
        }
        if (modelSetGate) await modelSetGate.promise;
        currentModel = modelRef;
        currentEffort = modelRef === "provider/d" ? "high" : "auto";
      },
      SetEffortForTab: async (_tabId: string, level: string) => {
        if (failNextEffortSet) {
          failNextEffortSet = false;
          throw new Error("effort switch failed");
        }
        if (effortSetGate) await effortSetGate.promise;
        currentEffort = level;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

type Controller = ReturnType<typeof useController>;
let controller: Controller | undefined;
const renderSnapshots: Array<{ phase: string; modelRef: string; effort?: string }> = [];

function Probe() {
  controller = useController();
  renderSnapshots.push({
    phase: controller.state.composerSelection.phase,
    modelRef: controller.state.meta?.modelRef ?? "",
    effort: controller.state.effort?.current,
  });
  return null;
}

function currentController(): Controller {
  if (!controller) throw new Error("controller did not render");
  return controller;
}

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

await act(async () => {
  root.render(<Probe />);
  await flushPromises();
});
await waitFor("active tab", () => currentController().activeTabId === "tab-a");

await act(async () => {
  startupMeta.resolve(meta("provider/a"));
  await flushPromises();
});
eq(currentController().state.composerSelection.phase, "pending", "model metadata alone keeps the combined selection pending");
eq(currentController().state.effort, undefined, "model metadata alone never exposes a missing effort");

await act(async () => {
  startupEffort.resolve(effort("high"));
  await startupEffort.promise;
  await flushPromises();
});
await waitFor("ready startup selection", () => currentController().state.composerSelection.phase === "ready");
eq(currentController().state.meta?.modelRef, "provider/a", "startup publishes the model after effort settles");
eq(currentController().state.effort?.current, "high", "startup publishes effort in the same ready selection");
eq(currentController().state.context.used, 0, "slow ancillary context does not delay the combined selection");
ok(
  !renderSnapshots.some((snapshot) => snapshot.phase === "ready" && snapshot.modelRef === "provider/a" && snapshot.effort === undefined),
  "no ready render exposes the new model without its effort",
);

contextPending = false;
startupContext.resolve({ used: 12, window: 100, sessionTokens: 12 });
await act(async () => {
  await startupContext.promise;
  await flushPromises();
});

const staleMeta = deferred<Meta>();
const staleEffort = deferred<EffortInfo>();
const latestMeta = deferred<Meta>();
const latestEffort = deferred<EffortInfo>();
metaQueue.push(staleMeta.promise, latestMeta.promise);
effortQueue.push(staleEffort.promise, latestEffort.promise);
let staleRefresh!: Promise<void>;
let latestRefresh!: Promise<void>;
await act(async () => {
  staleRefresh = currentController().refreshMeta();
  latestRefresh = currentController().refreshMeta();
  await flushPromises();
});
currentModel = "provider/b";
await act(async () => {
  latestMeta.resolve(meta("provider/b"));
  latestEffort.resolve(effort("low"));
  await latestRefresh;
});
eq(currentController().state.meta?.modelRef, "provider/b", "latest refresh publishes its model");
eq(currentController().state.effort?.current, "low", "latest refresh publishes its matching effort");
await act(async () => {
  staleMeta.resolve(meta("provider/a"));
  staleEffort.resolve(effort("high"));
  await staleRefresh;
});
eq(currentController().state.meta?.modelRef, "provider/b", "late older metadata cannot overwrite the latest selection");
eq(currentController().state.effort?.current, "low", "late older effort cannot overwrite the latest selection");

const degradedMeta = deferred<Meta>();
const degradedEffort = deferred<EffortInfo>();
metaQueue.push(degradedMeta.promise);
effortQueue.push(degradedEffort.promise);
let degradedRefresh!: Promise<void>;
await act(async () => {
  degradedRefresh = currentController().refreshMeta();
  await flushPromises();
  currentModel = "provider/c";
  degradedMeta.resolve(meta("provider/c"));
  degradedEffort.reject(new Error("effort unavailable"));
  await degradedRefresh;
});
eq(currentController().state.composerSelection.phase, "degraded", "effort failure releases a model-only degraded selection");
eq(currentController().state.meta?.modelRef, "provider/c", "degraded selection keeps the current model visible");
eq(currentController().state.effort, undefined, "degraded selection clears the older model effort");

const unsupportedMeta = deferred<Meta>();
const unsupportedEffort = deferred<EffortInfo>();
metaQueue.push(unsupportedMeta.promise);
effortQueue.push(unsupportedEffort.promise);
let unsupportedRefresh!: Promise<void>;
await act(async () => {
  unsupportedRefresh = currentController().refreshMeta();
  await flushPromises();
  currentModel = "provider/plain";
  unsupportedMeta.resolve(meta("provider/plain"));
  unsupportedEffort.resolve(effort("auto", false));
  await unsupportedRefresh;
});
eq(currentController().state.composerSelection.phase, "ready", "unsupported effort is a complete model-only selection");
eq(currentController().state.effort?.supported, false, "unsupported model retains authoritative capability evidence");

modelSetGate = deferred<void>();
let modelSwitch!: Promise<void>;
await act(async () => {
  modelSwitch = currentController().setModel("provider/d");
  await flushPromises();
});
eq(currentController().state.composerSelection.phase, "pending", "direct model switch invalidates the prior combined selection immediately");
eq(currentController().state.effort, undefined, "direct model switch cannot show the previous model effort");
modelSetGate.resolve();
await act(async () => {
  await modelSwitch;
});
modelSetGate = undefined;
eq(currentController().state.meta?.modelRef, "provider/d", "successful model switch publishes the selected model");
eq(currentController().state.effort?.current, "high", "successful model switch publishes the selected model effort atomically");

effortSetGate = deferred<void>();
let effortSwitch!: Promise<void>;
await act(async () => {
  effortSwitch = currentController().setEffort("low");
  await flushPromises();
});
eq(currentController().state.composerSelection.phase, "updating-effort", "direct effort switch keeps a visible updating phase until confirmation");
eq(currentController().state.meta?.modelRef, "provider/d", "optimistic effort switch keeps the confirmed model visible");
eq(currentController().state.effort?.current, "low", "optimistic effort switch displays the selected level immediately");
effortSetGate.resolve();
await act(async () => {
  await effortSwitch;
});
effortSetGate = undefined;
eq(currentController().state.composerSelection.phase, "ready", "confirmed effort switch returns the visible pair to ready");
eq(currentController().state.meta?.modelRef, "provider/d", "effort switch preserves the confirmed model");
eq(currentController().state.effort?.current, "low", "effort switch publishes the confirmed new level");

failNextEffortSet = true;
await act(async () => {
  await currentController().setEffort("high");
});
eq(currentController().state.composerSelection.phase, "ready", "failed effort switch restores the prior ready phase");
eq(currentController().state.meta?.modelRef, "provider/d", "failed effort switch keeps the prior model");
eq(currentController().state.effort?.current, "low", "failed effort switch restores the prior level");
ok(
  currentController().state.items.some((item) => item.kind === "notice" && item.text.includes("切换推理强度失败")),
  "failed effort switch reports a visible warning",
);

failNextModelSet = true;
await act(async () => {
  await currentController().setModel("provider/failing");
});
eq(currentController().state.composerSelection.phase, "ready", "failed model switch restores the prior ready phase");
eq(currentController().state.meta?.modelRef, "provider/d", "failed model switch restores the prior model");
eq(currentController().state.effort?.current, "low", "failed model switch restores the prior effort");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
