// Run: tsx src/__tests__/history-preview-error.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { HistoryPanel } from "../components/HistoryPanel";
import { LocaleProvider } from "../lib/i18n";
import type { HistoryMessage, SessionMeta } from "../lib/types";

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

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", { pretendToBeVisual: true, url: "http://localhost/" });
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

const sessions: SessionMeta[] = [
  { path: "/sessions/good.jsonl", title: "Good", preview: "good", turns: 1, createdAt: 1, lastActivityAt: 2, modTime: 2, current: false, open: false },
  { path: "/sessions/broken.jsonl", title: "Broken", preview: "broken", turns: 1, createdAt: 1, lastActivityAt: 1, modTime: 1, current: false, open: false },
];
let rejectBrokenPreview: ((reason?: unknown) => void) | undefined;
const brokenPreview = new Promise<HistoryMessage[]>((_, reject) => { rejectBrokenPreview = reject; });
const onPreview = async (path: string): Promise<HistoryMessage[]> => {
  if (path.includes("broken")) return brokenPreview;
  return [{ role: "user", content: "previous successful preview" }];
};
const resumedPaths: string[] = [];

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
await act(async () => {
  root.render(
    <LocaleProvider>
      <HistoryPanel
        sessions={sessions}
        trashedSessions={[]}
        running={false}
        onResume={(session) => { resumedPaths.push(session.path); }}
        onPreview={onPreview}
        onDelete={() => {}}
        onRename={() => {}}
        onKindChange={() => {}}
        onClose={() => {}}
      />
    </LocaleProvider>,
  );
  await flush();
  await flush();
});

await act(async () => { await flush(); });
ok(document.body.textContent?.includes("previous successful preview") ?? false, "successful preview is visible");

await act(async () => {
  Array.from(document.querySelectorAll<HTMLButtonElement>(".hist-item__main")).find((button) => button.textContent?.includes("Broken"))?.click();
  await flush();
});
await act(async () => {
  Array.from(document.querySelectorAll<HTMLButtonElement>("button")).find((button) => button.textContent?.includes("打开会话"))?.click();
  await flush();
});
ok(resumedPaths.at(-1) === "/sessions/broken.jsonl", "loading preview immediately owns the newly selected session identity");

await act(async () => {
  rejectBrokenPreview?.(new Error("corrupt jsonl detail"));
  await flush();
});
ok(document.body.textContent?.includes("无法加载预览") ?? false, "preview rejection displays an explicit error");
ok(!(document.body.textContent?.includes("previous successful preview") ?? false), "preview rejection does not show history from another session");
ok(
  document.querySelector<HTMLElement>("[role=alert]")?.title === "加载历史预览失败，请稍后重试。",
  "preview error keeps the safe Chinese detail",
);
ok(document.body.textContent?.includes("corrupt jsonl detail") === false, "preview error hides the raw diagnostic detail");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
