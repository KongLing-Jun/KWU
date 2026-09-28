// Run: tsx src/__tests__/todo-panel-runtime-state.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";

import { TodoPanel } from "../components/TodoPanel";
import { LocaleProvider } from "../lib/i18n";

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

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.HTMLLIElement = dom.window.HTMLLIElement;
globalThis.localStorage = dom.window.localStorage;
dom.window.HTMLElement.prototype.scrollIntoView = () => {};

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
const todos = [
  { content: "Inspect the document", status: "completed" as const },
  { content: "Append a new joke", activeForm: "Appending a new joke", status: "in_progress" as const },
  { content: "Verify the document", status: "pending" as const },
];

console.log("\ntodo panel runtime state");

await act(async () => {
  root.render(
    <LocaleProvider>
      <TodoPanel todos={todos} active={false} onDismiss={() => {}} />
    </LocaleProvider>,
  );
});
ok(rootElement.querySelector(".todobar__status--unfinished") !== null, "idle in-progress snapshot is presented as unfinished");
ok(rootElement.querySelector(".todobar__status--in_progress") === null, "idle panel does not claim that a task is running");
ok(rootElement.textContent?.includes("Append a new joke") === true, "idle panel uses stable task content instead of active-form copy");
ok(rootElement.querySelectorAll(".prompt-shelf__header-button").length === 2, "idle unfinished panel can be dismissed");

await act(async () => {
  root.render(
    <LocaleProvider>
      <TodoPanel todos={todos} active onDismiss={() => {}} />
    </LocaleProvider>,
  );
});
ok(rootElement.querySelector(".todobar__status--in_progress") !== null, "active turn presents the current task as running");
ok(rootElement.textContent?.includes("Appending a new joke") === true, "active turn uses active-form copy");
ok(rootElement.querySelectorAll(".prompt-shelf__header-button").length === 1, "active unfinished panel stays visible until the turn settles");

const completedTodos = todos.map((todo) => ({ ...todo, status: "completed" as const }));
await act(async () => {
  root.render(
    <LocaleProvider>
      <TodoPanel todos={completedTodos} active={false} onDismiss={() => {}} />
    </LocaleProvider>,
  );
});
ok(rootElement.querySelector(".todobar__list") === null, "completed todo panel collapses automatically");
const expandButton = Array.from(rootElement.querySelectorAll<HTMLButtonElement>(".prompt-shelf__header-button"))
  .find((button) => button.textContent?.includes("展开"));
await act(async () => expandButton?.click());
ok(rootElement.querySelector(".todobar__list") !== null, "completed todo details remain available on demand");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
