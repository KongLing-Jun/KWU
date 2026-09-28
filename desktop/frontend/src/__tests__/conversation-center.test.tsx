// Run: tsx src/__tests__/conversation-center.test.tsx

import { JSDOM } from "jsdom";
import React, { act, useState } from "react";
import { createRoot } from "react-dom/client";
import { HistoryPanel, type HistoryPanelKind } from "../components/HistoryPanel";
import { LocaleProvider } from "../lib/i18n";
import type { SessionMeta } from "../lib/types";

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

const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const now = Date.now();
const session = (path: string, title: string, workspaceRoot?: string): SessionMeta => ({
  path,
  title,
  preview: title,
  turns: 2,
  createdAt: now - 1_000,
  lastActivityAt: now,
  modTime: now,
  current: false,
  open: false,
  scope: workspaceRoot ? "project" : "global",
  workspaceRoot,
  topicTitle: `${title} topic`,
});

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
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });

const sessions = [
  session("/sessions/global.jsonl", "Global conversation"),
  session("/sessions/alpha.jsonl", "Alpha conversation", "D:/workspaces/alpha"),
  session("/sessions/beta.jsonl", "Beta conversation", "D:/workspaces/beta"),
];
const trashedSessions = [
  { ...session("/trash/alpha.jsonl", "Deleted alpha", "D:/workspaces/alpha"), deletedAt: now },
];

function Harness() {
  const [kind, setKind] = useState<HistoryPanelKind>("history");
  return (
    <LocaleProvider>
      <HistoryPanel
        kind={kind}
        sessions={sessions}
        trashedSessions={trashedSessions}
        running={false}
        onResume={() => {}}
        onPreview={async () => []}
        onDelete={() => {}}
        onRename={() => {}}
        onRestore={() => {}}
        onPurge={() => {}}
        onPurgeAll={() => {}}
        onKindChange={setKind}
        onClose={() => {}}
      />
    </LocaleProvider>
  );
}

console.log("\nconversation center");

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(<Harness />);
  await flush();
  await flush();
});

const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
ok(tabs.length === 2, "conversation center exposes conversations and trash tabs");
ok(tabs[0]?.textContent?.includes("3") ?? false, "conversation tab shows the global session count");
ok(tabs[1]?.textContent?.includes("1") ?? false, "trash tab shows the global trash count");

const workspaceSelect = document.querySelector<HTMLSelectElement>(".history-workspace-filter select");
ok(Boolean(workspaceSelect), "workspace filter is visible");
const optionText = Array.from(workspaceSelect?.options ?? []).map((option) => option.textContent ?? "").join("\n");
ok(optionText.includes("alpha") && optionText.includes("beta") && optionText.includes("全局"), "workspace filter distinguishes global and project conversations");

await act(async () => {
  if (workspaceSelect) {
    workspaceSelect.value = "D:/workspaces/alpha";
    workspaceSelect.dispatchEvent(new Event("change", { bubbles: true }));
  }
  await flush();
});
const historyRows = Array.from(document.querySelectorAll<HTMLElement>(".hist-item"));
ok(historyRows.length === 1 && historyRows[0]?.textContent?.includes("Alpha conversation"), "workspace filter limits the conversation list");

await act(async () => {
  tabs[1]?.click();
  await flush();
  await flush();
});
ok(document.querySelector('[role="tab"][aria-selected="true"]')?.textContent?.includes("回收站") ?? false, "trash opens inside the same conversation center");
ok(document.body.textContent?.includes("Deleted alpha") ?? false, "trash keeps the active workspace filter and renders matching deleted sessions");

await act(async () => {
  root.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
