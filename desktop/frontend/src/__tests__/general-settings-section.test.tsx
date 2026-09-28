// Run: tsx src/__tests__/general-settings-section.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import {
  GeneralSettingsSection,
  normalizeAutoPlan,
  normalizeCloseBehavior,
  normalizeDesktopLayoutStyle,
  normalizeDisplayMode,
  normalizeStatusBarStyle,
} from "../components/settings/GeneralSettingsSection";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SettingsView } from "../lib/types";

let passed = 0;
let failed = 0;

function equal(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

console.log("\ngeneral settings section");

equal(normalizeCloseBehavior("unknown"), "background", "unknown close behavior normalizes to background");
equal(normalizeDisplayMode("compact"), "compact", "compact display mode is preserved");
equal(normalizeDesktopLayoutStyle("classic"), "creation", "desktop layout remains pinned to creation mode");
equal(normalizeStatusBarStyle("icon"), "icon", "icon status bar style is preserved");
equal(normalizeAutoPlan("ask"), "on", "legacy ask auto-plan mode normalizes to on");

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
window.scrollTo = () => {};

const calls: string[] = [];
window.go = {
  main: {
    App: {
      SetCloseBehavior: async (mode: string) => { calls.push(`close:${mode}`); },
      SetDisplayMode: async (mode: string) => { calls.push(`display:${mode}`); },
      SetDefaultToolApprovalMode: async (mode: string) => { calls.push(`approval:${mode}`); },
      SetAutoPlan: async (mode: string) => { calls.push(`plan:${mode}`); },
    } as Partial<AppBindings> as AppBindings,
  },
};

const settings = {
  closeBehavior: "background",
  displayMode: "standard",
  autoPlan: "off",
  defaultToolApprovalMode: "ask",
  statusBarStyle: "text",
  statusBarItems: ["model", "workspace", "git_branch", "cache", "balance"],
} as SettingsView;

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

await act(async () => {
  root.render(
    <LocaleProvider>
      <GeneralSettingsSection
        s={settings}
        busy={false}
        agentRunning={false}
        apply={async (mutation) => { await mutation(); }}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});

const segmentedControls = Array.from(document.querySelectorAll<HTMLElement>(".set-seg"));
equal(segmentedControls.length >= 6, true, "general settings segmented controls render");
equal(document.body.textContent?.includes("桌面风格"), false, "desktop layout style setting is hidden");
equal(document.body.textContent?.includes("创作"), false, "fixed creation layout value is hidden");

const clickOption = async (groupIndex: number, optionIndex: number) => {
  const button = segmentedControls[groupIndex]?.querySelectorAll<HTMLButtonElement>("button")[optionIndex];
  if (!button) throw new Error(`missing option ${groupIndex}:${optionIndex}`);
  await act(async () => {
    button.click();
    await flushPromises();
  });
};

await clickOption(1, 1);
await clickOption(2, 1);
await clickOption(3, 1);
await clickOption(4, 1);

equal(calls.includes("close:quit"), true, "quit close behavior uses the existing bridge mutation");
equal(calls.includes("display:compact"), true, "compact display mode uses the existing bridge mutation");
equal(calls.includes("approval:auto"), true, "auto approval uses the existing bridge mutation");
equal(calls.includes("plan:on"), true, "auto-plan on uses the existing bridge mutation");

await act(async () => root.unmount());

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
