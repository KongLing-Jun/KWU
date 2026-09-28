// Run: tsx src/__tests__/settings-appearance-accessibility.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { AppearanceSection } from "../components/settings/AppearanceSettingsSection";
import { LocaleProvider } from "../lib/i18n";
import type { Theme } from "../lib/theme";

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

console.log("\nsettings appearance accessibility");

const dom = new JSDOM("<!doctype html><html data-platform=\"windows\"><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.HTMLCanvasElement = dom.window.HTMLCanvasElement;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
dom.window.HTMLCanvasElement.prototype.getContext = (() => ({
  font: "",
  measureText: () => ({ width: 100 }),
})) as unknown as typeof dom.window.HTMLCanvasElement.prototype.getContext;

let selectedTheme: Theme | null = null;
const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

await act(async () => {
  root.render(
    <LocaleProvider>
      <AppearanceSection
        theme="auto"
        textSize="default"
        fontFamily="custom"
        monoFontFamily="custom"
        customFontName="Example Sans"
        customMonoFontName="Example Mono"
        onTheme={(theme) => { selectedTheme = theme; }}
        onTextSize={() => {}}
        onFontFamily={() => {}}
        onMonoFontFamily={() => {}}
        onCustomFontNameChange={() => {}}
        onCustomMonoFontNameChange={() => {}}
      />
    </LocaleProvider>,
  );
});

const groups = Array.from(document.querySelectorAll<HTMLElement>("[role=\"radiogroup\"]"));
ok(groups.length === 4, "visible appearance choices expose radiogroup semantics");
ok(document.querySelectorAll(".appearance-panel__section").length === 4, "appearance choices use the renderer-style section layout");
ok(groups.every((group) => Boolean(group.getAttribute("aria-label"))), "each radiogroup has an accessible name");
ok(groups.every((group) => group.querySelectorAll('[role="radio"][aria-checked="true"]').length === 1), "each radiogroup exposes one selected option");
ok(groups.every((group) => group.querySelectorAll('[role="radio"][tabindex="0"]').length === 1), "each radiogroup exposes one keyboard tab stop");

const radioButtons = Array.from(document.querySelectorAll<HTMLButtonElement>('button[role="radio"]'));
ok(radioButtons.length > 0 && radioButtons.every((button) => button.type === "button"), "appearance choice buttons cannot submit a parent form");
ok(document.querySelectorAll('[data-theme-style-card]').length === 0, "visual style choices stay hidden");

const textareas = Array.from(document.querySelectorAll<HTMLTextAreaElement>("textarea"));
ok(textareas.length === 2, "custom font choices render both name inputs");
ok(textareas.every((textarea) => Boolean(textarea.name) && Boolean(textarea.getAttribute("aria-label"))), "custom font inputs expose stable names and accessible labels");
ok(textareas.every((textarea) => textarea.autocomplete === "off" && textarea.getAttribute("spellcheck") === "false"), "custom font inputs disable irrelevant autocomplete and spellcheck");

const themeButtons = groups[0]?.querySelectorAll<HTMLButtonElement>('button[role="radio"]');
await act(async () => {
  themeButtons?.[0]?.dispatchEvent(new dom.window.KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true, cancelable: true }));
});
ok(selectedTheme === "light", "arrow keys move selection within a theme radiogroup");

await act(async () => {
  themeButtons?.[2]?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
});
ok(selectedTheme === "dark", "theme option keeps its existing click callback behavior");

await act(async () => root.unmount());

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exitCode = 1;
