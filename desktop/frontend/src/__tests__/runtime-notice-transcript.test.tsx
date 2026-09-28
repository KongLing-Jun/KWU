// Run: tsx src/__tests__/runtime-notice-transcript.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import type { Item } from "../lib/useController";
import { historyMessagesToItems, initialState, reducer } from "../lib/useController";
import type { WireEvent } from "../lib/types";

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

console.log("\nruntime notice transcript");

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

const rawPhase = "provider/model · executing";
const rawTaskNotice = "background task finished: task-9";
const rawTurnError = "context deadline exceeded";
const rawCanceled = "context canceled";
const rawReadiness = "final-answer readiness blocked: missing project checks";
const rawMCPFailure = "mcp canva--canva: plugin \"canva--canva\": initialize: connector OAuth credential is unavailable";
const rawMCPFailureSummary = "2 MCP server(s) failed to start: canva--canva, tencent-docs--tencent-docs — run /mcp for details";
const unknownNotice = "provider exploded with code E_STRANGE";
const assistantBody = rawTaskNotice;
const items: Item[] = [
  { kind: "user", id: "u1", text: "测试提示展示" },
  { kind: "assistant", id: "a1", text: assistantBody, reasoning: "", streaming: false },
  { kind: "phase", id: "p1", text: rawPhase },
  { kind: "notice", id: "n1", level: "info", text: rawTaskNotice },
  { kind: "notice", id: "n2", level: "warn", text: unknownNotice },
  { kind: "notice", id: "n-mcp", level: "warn", text: rawMCPFailure },
  { kind: "compaction", id: "c1", pending: false, trigger: "auto", messages: 4, summary: "summary", archive: "" },
];

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript items={items} onPrompt={() => {}} questionNavigator={false} />
    </LocaleProvider>,
  );
});

const phase = document.querySelector(".phase");
const noticeLines = Array.from(document.querySelectorAll<HTMLElement>(".notice-line__text"));
const compactionMeta = document.querySelector(".compaction__meta");
const assistant = document.querySelector(".msg--assistant");

ok(phase?.textContent?.includes("provider/model · 正在执行") === true, "known Phase renders localized text");
ok(phase?.textContent?.includes(rawPhase) === false, "known Phase does not expose the raw suffix in the card");
ok(noticeLines.some((line) => line.textContent === "子任务已完成"), "known task lifecycle Notice renders localized text without task ID");
ok(noticeLines.every((line) => line.textContent !== rawTaskNotice), "known task lifecycle Notice hides the raw lifecycle text");
ok(noticeLines.some((line) => line.textContent === unknownNotice), "unknown Notice remains byte-for-byte visible");
ok(noticeLines.every((line) => !line.textContent?.includes("canva--canva")), "persisted MCP startup failures stay hidden from the transcript");
ok(assistant?.textContent?.includes(assistantBody) === true, "assistant body containing the same phrase remains unchanged");
ok(compactionMeta?.textContent?.includes("自动") === true, "known compaction trigger renders localized text");
ok(compactionMeta?.textContent?.includes("auto") === false, "known compaction trigger hides the raw enum in the card");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[
          { kind: "notice", id: "n3", level: "warn", text: rawTurnError },
          { kind: "notice", id: "n4", level: "warn", text: rawCanceled },
          { kind: "notice", id: "n5", level: "info", text: rawReadiness },
        ]}
        onPrompt={() => {}}
        questionNavigator={false}
      />
    </LocaleProvider>,
  );
});
const turnNotices = Array.from(document.querySelectorAll<HTMLElement>(".notice-line__text"));
ok(turnNotices.some((line) => line.textContent === "请求超时，本轮已停止"), "known turn_done timeout renders localized text");
ok(turnNotices.some((line) => line.textContent === "本轮已中断"), "known cancellation renders localized text immediately");
ok(turnNotices.some((line) => line.textContent === "最终回复尚未满足完成条件，正在继续处理"), "known readiness retry remains visible with user-facing text");

const projectedNotice = reducer(
  { ...initialState },
  { type: "event", e: { kind: "notice", level: "info", text: rawTaskNotice } as WireEvent },
);
const noticeItem = projectedNotice.items.at(-1);
ok(noticeItem?.kind === "notice" && noticeItem.text === rawTaskNotice, "notice projection preserves raw Item.text");

const projectedMCPFailure = reducer(
  { ...initialState },
  { type: "event", e: { kind: "notice", level: "warn", text: rawMCPFailure } as WireEvent },
);
ok(projectedMCPFailure.items.length === 0, "live MCP startup failures are omitted from session items");

const projectedMCPFailureSummary = reducer(
  { ...initialState },
  { type: "event", e: { kind: "notice", level: "warn", text: rawMCPFailureSummary } as WireEvent },
);
ok(projectedMCPFailureSummary.items.length === 0, "live MCP startup summaries are omitted from session items");

const restoredMCPFailure = historyMessagesToItems([
  { role: "notice", level: "warn", content: rawMCPFailure },
  { role: "assistant", content: "仍可正常回复" },
], "h");
ok(
  restoredMCPFailure.items.length === 1 && restoredMCPFailure.items[0]?.kind === "assistant",
  "history restore drops MCP startup failures without dropping the assistant response",
);

const projectedTurnError = reducer(
  { ...initialState },
  { type: "event", e: { kind: "turn_done", err: rawTurnError } as WireEvent },
);
const turnErrorItem = projectedTurnError.items.at(-1);
ok(turnErrorItem?.kind === "notice" && turnErrorItem.text === rawTurnError, "turn_done projection preserves raw error text");

const projectedCompaction = reducer(
  { ...initialState },
  { type: "event", e: { kind: "compaction_started", compaction: { trigger: "auto" } } as WireEvent },
);
const compactionItem = projectedCompaction.items.at(-1);
ok(compactionItem?.kind === "compaction" && compactionItem.trigger === "auto", "compaction projection preserves raw trigger enum");

await act(async () => {
  root.unmount();
});

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}

process.stdout.write(`\n${passed} tests passed.\n`);
