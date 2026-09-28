// Run: tsx src/__tests__/channel-session-revalidation.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { useController } from "../features/session/useController";
import type { AppBindings } from "../lib/bridge";
import type {
  BalanceInfo,
  CheckpointMeta,
  ContextInfo,
  EffortInfo,
  HistoryMessage,
  JobView,
  Meta,
  TabMeta,
} from "../lib/types";

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

const sessionPath = "D:\\repo\\channel.jsonl";
const initialHistory: HistoryMessage[] = [
  { role: "user", content: "first mobile prompt" },
  { role: "assistant", content: "first complete answer" },
];
let previewHistory: HistoryMessage[] = initialHistory;
let previewFailure: Error | undefined;
let previewCalls = 0;

function tabMeta(): TabMeta {
  return {
    id: "tab-channel",
    scope: "project",
    workspaceRoot: "D:\\repo",
    workspaceName: "repo",
    workspacePath: "D:\\repo",
    gitBranch: "main",
    topicId: "",
    topicTitle: "Feishu",
    label: "model",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    active: true,
    readOnly: true,
    sessionPath,
    cwd: "D:\\repo",
  };
}

function meta(): Meta {
  return {
    label: "model",
    ready: true,
    eventChannel: "agent:event",
    cwd: "D:\\repo",
    workspaceRoot: "D:\\repo",
    workspaceName: "repo",
    workspacePath: "D:\\repo",
    gitBranch: "main",
    autoApproveTools: false,
    bypass: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    goal: "",
    goalStatus: "stopped",
    sessionPath,
  };
}

console.log("\nchannel session history revalidation");

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
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

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
      HistoryForTab: async () => initialHistory,
      ReplayPendingPrompts: async () => {},
      OpenChannelSessionForTab: async () => initialHistory,
      PreviewSession: async () => {
        previewCalls += 1;
        if (previewFailure) throw previewFailure;
        return previewHistory;
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

await act(async () => {
  root.render(<Probe />);
  await flushPromises();
});
await waitFor("active channel tab", () => controller?.activeTabId === "tab-channel");

await act(async () => {
  await controller?.openChannelSession(sessionPath, "tab-channel");
  await flushPromises();
});
eq(controller?.state.items.length, 2, "opening the channel renders its initial disk history");

const initialItems = controller?.state.items;
previewHistory = [
  ...initialHistory,
  { role: "user", content: "second mobile prompt" },
  { role: "assistant", content: "second complete answer" },
];
await act(async () => {
  await controller?.revalidateChannelSession("tab-channel", sessionPath);
  await flushPromises();
});
eq(controller?.state.items.length, 4, "revalidation publishes externally appended complete history");
ok(
  controller?.state.items.some((item) => item.kind === "assistant" && item.text === "second complete answer") ?? false,
  "the refreshed transcript includes the completed Agent response",
);
eq(controller?.state.items[0], initialItems?.[0], "append revalidation preserves the existing projected prefix");

const refreshedItems = controller?.state.items;
await act(async () => {
  await controller?.revalidateChannelSession("tab-channel", sessionPath);
  await flushPromises();
});
eq(controller?.state.items, refreshedItems, "an unchanged disk fingerprint does not publish React state");

previewFailure = new Error("disk temporarily unavailable");
await act(async () => {
  await controller?.revalidateChannelSession("tab-channel", sessionPath);
  await flushPromises();
});
eq(controller?.state.items, refreshedItems, "a failed disk preview preserves the current transcript");

previewFailure = undefined;
previewHistory = [
  ...previewHistory,
  { role: "user", content: "recovered prompt" },
  { role: "assistant", content: "recovered answer" },
];
await act(async () => {
  await controller?.revalidateChannelSession("tab-channel", sessionPath);
  await flushPromises();
});
eq(controller?.state.items.length, 6, "a later revision can recover after a failed disk preview");
eq(previewCalls, 4, "each explicit coalesced refresh attempt performs at most one disk preview");

await act(async () => {
  root.unmount();
});

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
