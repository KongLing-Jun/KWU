// Run: tsx src/__tests__/context-panel-expert.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { ContextPanel } from "../components/ContextPanel";
import type { ExpertSelection } from "../components/ExpertDirectory";
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

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLImageElement = dom.window.HTMLImageElement;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  return dom;
}

console.log("\ncontext panel active expert");

const dom = installDom();
const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);
const activeExpert: ExpertSelection = {
  id: "backend-agent-team",
  name: "Backend Agent Team",
  role: "Agent orchestration",
  mode: "team",
  prompt: "Coordinate the backend agent team.",
  tone: "teal",
  avatarSrc: "file:///avatars/office-automation-team.png",
  members: [
    { kind: "lead", name: "Lead Agent", role: "Coordinator", task: "Plan", output: "Plan", tone: "blue" },
    { kind: "sub", name: "Worker Agent", role: "Execution", task: "Do", output: "Result", tone: "green" },
  ],
};

await act(async () => {
  root.render(
    <LocaleProvider>
      <ContextPanel
        activeExpert={activeExpert}
      />
    </LocaleProvider>,
  );
  await flush();
});

const expertCard = document.querySelector<HTMLElement>(".context-panel__expert");
ok(Boolean(expertCard), "context panel renders active expert card");
ok(expertCard?.textContent?.includes("Backend Agent Team") === true, "expert card shows selected expert team name");
ok(expertCard?.textContent?.includes("Agent orchestration") === true, "expert card shows selected expert role");
ok(expertCard?.textContent?.includes("2") === true, "expert card shows team size");
const avatar = document.querySelector<HTMLImageElement>(".context-panel__expert-avatar img");
ok(avatar?.getAttribute("src") === "file:///avatars/office-automation-team.png", "expert card uses selected avatar image");

await act(async () => {
  root.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
