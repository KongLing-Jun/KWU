// Run: tsx src/__tests__/update-release-notes-dialog.test.tsx

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { UpdateReleaseNotesDialog } from "../components/UpdateReleaseNotesDialog";
import { LocaleProvider } from "../lib/i18n";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
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
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

console.log("\nupdate release notes dialog");

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
let closes = 0;
let outerCloses = 0;

function Harness() {
  const [open, setOpen] = React.useState(false);
  return (
    <>
      <button id="release-notes-trigger" type="button" onClick={() => setOpen(true)}>View release notes</button>
      {open && (
        <UpdateReleaseNotesDialog
          version="v1.0.11"
          notes={"# Highlights\n\n- **Faster** startup\n- [Documentation](https://example.com)"}
          onClose={() => {
            closes += 1;
            setOpen(false);
          }}
        />
      )}
    </>
  );
}

await act(async () => {
  root.render(
    <React.StrictMode>
      <LocaleProvider>
        <Harness />
      </LocaleProvider>
    </React.StrictMode>,
  );
  await flush();
});

const trigger = document.getElementById("release-notes-trigger") as HTMLButtonElement | null;
if (!trigger) throw new Error("missing release notes trigger");
trigger.focus();
await act(async () => {
  trigger.click();
  await flush();
});

ok(document.querySelector('[role="dialog"]')?.getAttribute("aria-modal") === "true", "renders an accessible modal dialog");
ok(document.body.textContent?.includes("v1.0.11") === true, "shows the release version");
ok(document.body.textContent?.includes("Faster startup") === true, "renders release note content");
ok(document.querySelector(".update-notes-modal__body h1")?.textContent === "Highlights", "renders Markdown headings");
ok(document.querySelector(".update-notes-modal__body strong")?.textContent === "Faster", "renders Markdown emphasis");
ok(document.querySelector(".update-notes-modal__body a")?.getAttribute("href") === "https://example.com", "renders Markdown links");
ok(document.activeElement?.classList.contains("modal-close-button") === true, "focuses the dialog close button");
ok(document.querySelector('[role="tooltip"]') === null, "does not show the close tooltip from automatic focus");

const outerEscape = (event: KeyboardEvent) => {
  if (event.key === "Escape") outerCloses += 1;
};
document.addEventListener("keydown", outerEscape);
await act(async () => {
  document.activeElement?.dispatchEvent(new dom.window.KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  await flush();
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));
});
ok(closes === 1, "Escape requests dialog close");
ok(outerCloses === 0, "Escape does not reach the parent Settings listener");
ok(document.querySelector('[role="dialog"]') === null, "Escape closes only the release notes dialog");
ok(document.activeElement === trigger, "closing restores focus to the release notes trigger");
document.removeEventListener("keydown", outerEscape);

await act(async () => {
  root.render(
    <LocaleProvider>
      <UpdateReleaseNotesDialog version="v1.0.11" notes="" onClose={() => {}} />
    </LocaleProvider>,
  );
  await flush();
});
ok(document.body.textContent?.includes("暂无版本说明") === true, "shows the empty-notes placeholder");

const here = dirname(fileURLToPath(import.meta.url));
const bannerSource = readFileSync(resolve(here, "../components/UpdateBanner.tsx"), "utf8");
const updateCenterSource = readFileSync(resolve(here, "../components/UpdateCenterDialog.tsx"), "utf8");
const settingsSource = readFileSync(resolve(here, "../features/settings/ui/UpdatesSettingsSection.tsx"), "utf8");
ok(bannerSource.includes('t("updater.viewReleaseNotes")'), "the update banner exposes the notes action");
ok(updateCenterSource.includes('t("updater.viewReleaseNotes")'), "the account update center exposes the notes action");
ok(updateCenterSource.includes("app.Version()") && updateCenterSource.includes("void check()"), "the update center shows the current version and checks the backend on open");
ok(
  /<ModalCloseButton[\s\S]*?autoFocus[\s\S]*?showTooltipOnFocus=\{false\}/.test(updateCenterSource),
  "the update center suppresses the close tooltip from automatic focus",
);
ok(!settingsSource.includes("useUpdater") && !settingsSource.includes("app.Version()"), "update settings remain configuration-only and perform no manual updater I/O");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
