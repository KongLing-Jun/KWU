// Run: tsx src/__tests__/transcript-process-fold.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import type { TaskTreeState } from "../lib/taskTreeState";
import type { Item } from "../lib/useController";

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

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

console.log("\ntranscript process fold");

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
globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", { configurable: true, value: () => {} });
localStorage.setItem("kwu-display-mode", "standard");

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

const completedItems: Item[] = [
  { kind: "user", id: "user-1", text: "执行一个真实的长任务" },
  { kind: "assistant", id: "progress-1", text: "我先检查相关文件。", reasoning: "分析入口", streaming: false },
  { kind: "phase", id: "phase-1", text: "provider/model · executing" },
  { kind: "tool", id: "read-1", name: "read_file", args: "{}", readOnly: true, status: "done", output: "source", durationMs: 1200 },
  { kind: "tool", id: "todo-1", name: "todo_write", args: "{}", readOnly: true, status: "done", output: "updated" },
  { kind: "notice", id: "info-1", level: "info", text: "background task finished: task-9" },
  { kind: "compaction", id: "compact-1", pending: false, trigger: "auto", messages: 6, summary: "summary", archive: "" },
  { kind: "tool", id: "task-1", name: "task", args: "{}", readOnly: false, status: "done", output: "agent result", durationMs: 800, taskTreeId: "tree-1", nodeId: "node-1" },
  { kind: "notice", id: "warn-1", level: "warn", text: "需要关注的真实警告" },
  { kind: "assistant", id: "report-1", text: `这是完整调研报告。${"关键事实与验证结论。".repeat(30)}`, reasoning: "最终回答前的推理", streaming: false },
  { kind: "assistant", id: "final-1", text: "所有步骤已完成，完整报告见上方。", reasoning: "收尾确认", streaming: false, finalAnswer: true },
  { kind: "user", id: "user-2", text: "继续检查" },
  { kind: "phase", id: "phase-2", text: "provider/model · executing" },
  { kind: "assistant", id: "final-2", text: "第二轮最终回答。", reasoning: "", streaming: false, finalAnswer: true },
];

const taskTrees: TaskTreeState = {
  trees: {
    "tree-1": {
      treeId: "tree-1",
      title: "专家团队执行",
      status: "completed",
      rootIds: ["node-1"],
      nodes: {
        "node-1": {
          nodeId: "node-1",
          title: "检查实现",
          status: "completed",
          dependsOn: [],
          childIds: [],
          output: "检查完成",
          outputDeltas: [],
          toolIds: ["task-1"],
          completionPercent: 100,
          updatedAt: 1,
        },
      },
      currentNodeIds: [],
      summary: "团队任务完成",
      updatedAt: 1,
    },
  },
  activeTreeId: "tree-1",
  seq: 1,
};

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={completedItems}
        taskTrees={taskTrees}
        viewKey="process-fold-acceptance"
        onPrompt={() => {}}
        questionNavigator={false}
        creationMode
      />
    </LocaleProvider>,
  );
  await flush();
});

const completedFolds = Array.from(document.querySelectorAll<HTMLElement>(".turn-collapse"));
const completedFold = completedFolds[0];
const completedHeader = completedFold?.querySelector<HTMLButtonElement>(".reasoning__head");
const outsideAssistantMessages = Array.from(document.querySelectorAll<HTMLElement>(".msg--assistant"))
  .filter((message) => !message.closest(".turn-collapse"));

ok(completedFolds.length === 2, "creation mode renders one process entry per turn even when the saved display mode is standard");
ok(completedHeader?.textContent?.includes("已完成") === true, "completed process entry has a settled summary");
ok(completedHeader?.getAttribute("aria-expanded") === "false", "completed process entry is collapsed by default");
ok(completedFold?.querySelector<HTMLElement>(".turn-collapse__body")?.style.height === "0px", "collapsed process details stay visually hidden when content arrives after mount");
ok(outsideAssistantMessages.length === 2 && outsideAssistantMessages[0]?.textContent?.includes("这是完整调研报告。") === true, "a substantive report remains visible when the accepted final candidate is only a short post-Todo receipt");
ok(outsideAssistantMessages[0]?.textContent?.includes("完整报告见上方") === false, "the short post-Todo receipt stays in process details instead of replacing the report");
ok(outsideAssistantMessages[0]?.querySelector(".reasoning") === null, "final-answer reasoning is moved into process details");
ok(completedFold?.textContent?.includes("我先检查相关文件。") === true, "intermediate assistant prose remains available in process details");
ok(Boolean(completedFold?.querySelector(".expert-team-flow-card")), "expert team flow is contained by the same process entry");
ok(completedFolds[1]?.querySelector(".expert-team-flow-card") === null, "expert team flow stays with the turn that owns its task tree");
ok(completedFold?.querySelector(".notice-line__text")?.textContent === "子任务已完成", "informational notice is contained by process details");
ok(document.body.textContent?.includes("需要关注的真实警告") === true && !completedFold?.textContent?.includes("需要关注的真实警告"), "warning remains visible outside the process fold");

await act(async () => {
  completedHeader?.click();
  await flush();
});
ok(completedHeader?.getAttribute("aria-expanded") === "true", "process details open with one click");

await act(async () => {
  for (const header of completedFold?.querySelectorAll<HTMLButtonElement>(".msg--assistant .reasoning__head") ?? []) {
    if (header.getAttribute("aria-expanded") === "false") header.click();
  }
  await flush();
});
ok(completedFold?.textContent?.includes("最终回答前的推理") === true, "final-answer reasoning remains available in process details");

const runningItems: Item[] = [
  { kind: "user", id: "user-running", text: "继续执行" },
];

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript items={runningItems} running viewKey="process-fold-acceptance" onPrompt={() => {}} questionNavigator={false} creationMode />
    </LocaleProvider>,
  );
  await flush();
});

let runningFold = document.querySelector<HTMLElement>(".turn-collapse");
let runningHeader = runningFold?.querySelector<HTMLButtonElement>(".reasoning__head");
ok(document.querySelectorAll(".turn-collapse").length === 1, "running turn renders a process entry before the first streamed item");
ok(runningHeader?.getAttribute("aria-expanded") === "true", "running process entry is expanded by default");

const streamingItems: Item[] = [
  ...runningItems,
  { kind: "assistant", id: "assistant-running", text: "", reasoning: "", streaming: true },
];
await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={streamingItems}
        live={{ id: "assistant-running", text: "", reasoning: "正在分析入口", reasoningComplete: false }}
        running
        viewKey="process-fold-acceptance"
        onPrompt={() => {}}
        questionNavigator={false}
        creationMode
      />
    </LocaleProvider>,
  );
  await flush();
});

runningFold = document.querySelector<HTMLElement>(".turn-collapse");
ok(runningFold?.textContent?.includes("正在分析入口") === true, "live reasoning is visible before the terminal assistant item arrives");

const settledRunningItems: Item[] = [
  { kind: "user", id: "user-running", text: "继续执行" },
  { kind: "assistant", id: "assistant-running", text: "已检查入口。", reasoning: "正在分析入口", streaming: false },
  { kind: "assistant", id: "assistant-final", text: "检查完成。", reasoning: "", streaming: false, finalAnswer: true },
];

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript items={settledRunningItems} viewKey="process-fold-acceptance" onPrompt={() => {}} questionNavigator={false} creationMode />
    </LocaleProvider>,
  );
  await flush();
});

const settledFold = document.querySelector<HTMLElement>(".turn-collapse");
const settledHeader = settledFold?.querySelector<HTMLButtonElement>(":scope > .reasoning__head");
ok(settledHeader?.getAttribute("aria-expanded") === "false", "process entry collapses when the running turn settles");

await act(async () => {
  settledHeader?.click();
  await flush();
});

const nestedReasoningHeader = settledFold?.querySelector<HTMLButtonElement>(".msg--assistant .reasoning__head");
ok(nestedReasoningHeader?.getAttribute("aria-expanded") === "false", "reopening process details keeps completed reasoning collapsed");

await act(async () => {
  root.unmount();
});
dom.window.close();

if (failed > 0) {
  process.stderr.write(`\n${failed} test(s) failed, ${passed} passed.\n`);
  process.exit(1);
}

process.stdout.write(`\n${passed} tests passed.\n`);
