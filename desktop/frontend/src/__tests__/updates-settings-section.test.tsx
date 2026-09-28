// Run: tsx src/__tests__/updates-settings-section.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { UpdatesSettingsSection } from "../components/settings/UpdatesSettingsSection";
import type { AppBindings } from "../lib/bridge";
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

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

console.log("\nupdates settings section");

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
globalThis.MouseEvent = dom.window.MouseEvent;

const saved: boolean[] = [];
let mutationCount = 0;
window.go = {
  main: {
    App: {
      SetDesktopCheckUpdates: async (enabled: boolean) => {
        saved.push(enabled);
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

await act(async () => {
  root.render(
    <LocaleProvider>
      <UpdatesSettingsSection
        configPath="C:\\Users\\example\\.reasonix\\config.json"
        checkUpdates
        settingsBusy={false}
        applySettings={async (mutation) => {
          mutationCount += 1;
          await mutation();
        }}
      />
    </LocaleProvider>,
  );
  await flushPromises();
});

const toggle = document.querySelector<HTMLInputElement>('.updates-settings-panel__switch input[type="checkbox"]');
ok(Boolean(toggle), "renderer-style update switch renders");
ok(toggle?.checked === true, "switch reflects the saved update preference");
ok(Boolean(toggle?.getAttribute("aria-label")), "switch keeps an accessible name");
ok(document.querySelectorAll(".updates-settings-panel__card").length === 1, "update preference uses the reference card layout");
ok(document.body.textContent?.includes("config.json") === true, "configuration path remains visible");

await act(async () => {
  toggle?.click();
  await flushPromises();
});

ok(mutationCount === 1, "toggle still runs through the settings mutation wrapper");
ok(saved.length === 1 && saved[0] === false, "toggle preserves the update preference bridge call");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exitCode = 1;
