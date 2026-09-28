// Run: tsx src/__tests__/history-file-links.test.tsx

import { JSDOM } from "jsdom";
import React, { type ComponentProps } from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { HistoryPanel } from "../components/HistoryPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

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
  ok(actual === expected, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function flushTimers(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
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
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

console.log("\nhistory assistant file links");

let globalReadCount = 0;
const tabReads: Array<[string, string]> = [];
const tabOpens: Array<[string, string]> = [];
const workspaceReads: Array<[string, string]> = [];
const workspaceOpens: Array<[string, string]> = [];
window.go = {
  main: {
    App: {
      ReadFile: async (path: string) => {
        globalReadCount += 1;
        return { path, body: "", size: 0, truncated: false, binary: false, err: "wrong active workspace" };
      },
      ReadFileForTab: async (tabId: string, path: string) => {
        tabReads.push([tabId, path]);
        return { path, body: "", size: 0, truncated: false, binary: false };
      },
      ReadFileForWorkspace: async (workspaceRoot: string, path: string) => {
        workspaceReads.push([workspaceRoot, path]);
        return { path, body: "", size: 0, truncated: false, binary: false };
      },
      OpenWorkspacePath: async () => {
        throw new Error("history preview must not open against the active workspace");
      },
      OpenWorkspacePathForTab: async (tabId: string, path: string) => {
        tabOpens.push([tabId, path]);
      },
      OpenWorkspacePathForWorkspace: async (workspaceRoot: string, path: string) => {
        workspaceOpens.push([workspaceRoot, path]);
      },
      AttachmentDataURL: async () => "",
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);
const historyProps = {
  kind: "history" as const,
  sessions: [
    {
      path: "D:/sessions/history-owner.jsonl",
      preview: "create the historical report",
      turns: 1,
      createdAt: Date.now() - 1_000,
      lastActivityAt: Date.now(),
      modTime: Date.now(),
      current: false,
      open: false,
      scope: "project",
      workspaceRoot: "D:/projects/history-owner",
    },
  ],
  trashedSessions: [],
  running: false,
  onResume: () => {},
  onPreview: async () => [
    { role: "user", content: "create the historical report" },
    { role: "assistant", content: "Historical output: `reports/history-output.xlsx`" },
  ],
  onDelete: () => {},
  onRename: () => {},
  onKindChange: () => {},
  onClose: () => {},
} as ComponentProps<typeof HistoryPanel>;

await act(async () => {
  root.render(
    <LocaleProvider>
      <ToastProvider>
        {React.createElement(HistoryPanel, historyProps)}
      </ToastProvider>
    </LocaleProvider>,
  );
  await flushTimers();
  await flushTimers();
  await flushTimers();
});

eq(globalReadCount, 0, "history preview does not verify generated files against the active workspace");
eq(tabReads.length, 0, "history preview does not bind generated files to a live tab");
eq(JSON.stringify(workspaceReads), JSON.stringify([["D:/projects/history-owner", "reports/history-output.xlsx"]]), "history preview verifies generated files against the selected session workspace");

const fileCard = document.querySelector<HTMLButtonElement>(".history-preview .msg-attachment");
ok(fileCard !== null, "history preview renders an existing generated file");
await act(async () => {
  fileCard?.click();
  await flushTimers();
});
eq(tabOpens.length, 0, "history preview does not open generated files against a live tab");
eq(JSON.stringify(workspaceOpens), JSON.stringify([["D:/projects/history-owner", "reports/history-output.xlsx"]]), "history preview opens generated files against the selected session workspace");

await act(async () => {
  root.unmount();
});

async function verifyOwnerlessHistoryFailsClosed(workspaceRoot: string | undefined, label: string) {
  document.body.innerHTML = "<div id=\"root\"></div>";
  const nextRootElement = document.getElementById("root");
  if (!nextRootElement) throw new Error("missing ownerless history root");
  const nextRoot = createRoot(nextRootElement);
  const beforeGlobalReads = globalReadCount;
  const beforeTabReads = tabReads.length;
  const beforeWorkspaceReads = workspaceReads.length;
  const nextProps: ComponentProps<typeof HistoryPanel> = {
    ...historyProps,
    sessions: [
      {
        ...historyProps.sessions[0],
        path: `D:/sessions/${label}.jsonl`,
        workspaceRoot,
      },
    ],
  };
  await act(async () => {
    nextRoot.render(
      <LocaleProvider>
        <ToastProvider>
          <HistoryPanel {...nextProps} />
        </ToastProvider>
      </LocaleProvider>,
    );
    await flushTimers();
    await flushTimers();
    await flushTimers();
  });
  eq(globalReadCount, beforeGlobalReads, `${label} history never falls back to the active workspace`);
  eq(tabReads.length, beforeTabReads, `${label} history never borrows a live tab owner`);
  eq(workspaceReads.length, beforeWorkspaceReads, `${label} history never calls a workspace-scoped API without a nonblank root`);
  ok(document.querySelector<HTMLButtonElement>(".history-preview .msg-attachment") === null, `${label} history files remain non-interactive`);
  await act(async () => {
    nextRoot.unmount();
  });
}

await verifyOwnerlessHistoryFailsClosed(undefined, "missing-root");
await verifyOwnerlessHistoryFailsClosed("   ", "blank-root");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
