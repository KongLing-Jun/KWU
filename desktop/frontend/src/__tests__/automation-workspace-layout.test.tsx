// Run: tsx src/__tests__/automation-workspace-layout.test.tsx

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import {
  AutomationWorkspace,
  orderAutomationWorkspaceTasks,
} from "../features/automation/workspace/AutomationWorkspace";
import type {
  AutomationRunSummary,
  AutomationTaskTemplate,
  HeartbeatTask,
} from "../features/automation/heartbeat/heartbeat.types";

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

const templates: AutomationTaskTemplate[] = Array.from({ length: 12 }, (_, index) => ({
  id: `template-${index + 1}`,
  title: `模板 ${index + 1}`,
  description: `模板 ${index + 1} 的说明`,
  prompt: `执行模板 ${index + 1}`,
  icon: index === 0 ? "news" : undefined,
  source: "builtin",
}));

const tasks: HeartbeatTask[] = [];
const runs: AutomationRunSummary[] = [];
const selectedTemplates: string[] = [];
const selectedTabs: string[] = [];
let addCount = 0;

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;

console.log("\nautomation workspace layout");

const host = document.getElementById("root")!;
const root = createRoot(host);
await act(async () => {
  root.render(
    <AutomationWorkspace
      templates={templates}
      tasks={tasks}
      runs={runs}
      onAddTask={() => { addCount += 1; }}
      onSelectTemplate={(template) => { selectedTemplates.push(template.id); }}
      onTabChange={(tab) => { selectedTabs.push(tab); }}
    />,
  );
});

const tabs = [...host.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
ok(
  tabs.length === 2 && tabs[0].textContent === "定时任务" && tabs[1].textContent === "运行记录",
  "renders scheduled-task and run-history tabs",
);
ok(
  tabs[0].getAttribute("aria-selected") === "true" && tabs[1].getAttribute("aria-selected") === "false",
  "selects scheduled tasks by default",
);
ok(
  host.textContent?.includes("开启你的第一个自动化任务吧") === true &&
    host.querySelector('[data-testid="automation-empty-state"]') !== null,
  "renders the scheduled-task empty state when there are no tasks",
);

const addButton = host.querySelector<HTMLButtonElement>('[data-testid="automation-add-task"]');
await act(async () => { addButton?.click(); });
ok(addCount === 1, "forwards the empty-state add action");

const templateCards = [...host.querySelectorAll<HTMLButtonElement>('[data-testid="automation-template-card"]')];
ok(templateCards.length === 12, "renders all twelve templates without truncation");
await act(async () => { templateCards[11]?.click(); });
ok(selectedTemplates[0] === "template-12", "forwards the selected template object");

await act(async () => { tabs[1]?.click(); });
ok(
  selectedTabs[0] === "runs" &&
    tabs[1].getAttribute("aria-selected") === "true" &&
    host.querySelector('[data-testid="automation-run-history"]') !== null,
  "switches to run history and reports the tab change",
);

await act(async () => {
	tabs[1]?.dispatchEvent(new dom.window.KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
});
ok(
	tabs[0].getAttribute("aria-selected") === "true" && document.activeElement === tabs[0],
	"supports arrow-key tab selection and focus movement",
);

const unorderedTasks: HeartbeatTask[] = [
  { id: "disabled-zulu", title: "Zulu", prompt: "", interval: "1h", enabled: false },
  { id: "same-b", title: "Same", prompt: "", interval: "1h", enabled: true, nextRunAt: "2026-08-20T09:00:00Z" },
  { id: "missing-zulu", title: "Zulu", prompt: "", interval: "1h", enabled: true },
  { id: "later", title: "Later", prompt: "", interval: "1h", enabled: true, nextRunAt: "2026-08-20T10:00:00Z" },
  { id: "same-a", title: "Same", prompt: "", interval: "1h", enabled: true, nextRunAt: "2026-08-20T09:00:00Z" },
  { id: "soon", title: "Soon", prompt: "", interval: "1h", enabled: true, nextRunAt: "2026-08-20T08:00:00Z" },
  { id: "missing-alpha", title: "Alpha", prompt: "", interval: "1h", enabled: true, nextRunAt: "not-a-date" },
  { id: "disabled-alpha", title: "Alpha", prompt: "", interval: "1h", enabled: false },
];
const originalTaskOrder = unorderedTasks.map((task) => task.id);
const expectedTaskOrder = [
  "soon",
  "same-a",
  "same-b",
  "later",
  "missing-alpha",
  "missing-zulu",
  "disabled-alpha",
  "disabled-zulu",
];
ok(
  orderAutomationWorkspaceTasks(unorderedTasks).map((task) => task.id).join(",") === expectedTaskOrder.join(","),
  "orders enabled tasks by next run, then missing schedules and disabled tasks by identity",
);
ok(
  unorderedTasks.map((task) => task.id).join(",") === originalTaskOrder.join(","),
  "does not mutate the task array supplied by the caller",
);

await act(async () => {
  root.render(
    <AutomationWorkspace
      templates={templates}
      tasks={unorderedTasks}
      runs={runs}
      onAddTask={() => { addCount += 1; }}
      onSelectTemplate={(template) => { selectedTemplates.push(template.id); }}
      renderTask={(task) => <span data-testid="ordered-task">{task.id}</span>}
    />,
  );
});
ok(
  [...host.querySelectorAll<HTMLElement>('[data-testid="ordered-task"]')]
    .map((item) => item.textContent)
    .join(",") === expectedTaskOrder.join(","),
  "renders the agreed task order in My Automations",
);

const testDir = dirname(fileURLToPath(import.meta.url));
const css = readFileSync(resolve(testDir, "../features/automation/workspace/AutomationWorkspace.css"), "utf8");
ok(
  /\.automation-workspace__scroll\s*\{[^}]*overflow-y:\s*auto;/s.test(css) &&
    /\.automation-workspace__template-grid\s*\{[^}]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/s.test(css),
  "keeps scrolling inside the content area and uses a three-column desktop template grid",
);
ok(
  /@media\s*\(max-width:\s*840px\)[\s\S]*repeat\(2,\s*minmax\(0,\s*1fr\)\)/.test(css) &&
    /@media\s*\(max-width:\s*560px\)[\s\S]*grid-template-columns:\s*1fr/.test(css),
  "adapts the template grid to two and one columns",
);

await act(async () => root.unmount());
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
