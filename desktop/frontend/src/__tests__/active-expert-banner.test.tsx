// Run: tsx src/__tests__/active-expert-banner.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { ActiveExpertBanner } from "../components/ActiveExpertBanner";

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
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  return dom;
}

console.log("\nactive expert banner");

const dom = installDom();
const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("missing root");
const root = createRoot(rootEl);
let clearCount = 0;

await act(async () => {
  root.render(
    <ActiveExpertBanner
      expert={{
        id: "pixel-agent",
        name: "Pixel Agent",
        role: "UI specialist",
        mode: "expert",
        prompt: "Answer as a UI specialist.",
        tone: "teal",
        avatarSrc: "file:///avatars/pixel-agent.png",
      }}
      clearLabel="Clear current expert"
      onClear={() => {
        clearCount += 1;
      }}
    />,
  );
  await flush();
});

const banner = document.querySelector<HTMLElement>(".expert-active-banner");
ok(Boolean(banner), "renders the active expert banner");
ok(banner?.textContent?.includes("Pixel Agent") === true, "shows the active expert name");
ok(!document.querySelector(".expert-active-banner__estimate"), "does not render an estimate label");
const avatar = document.querySelector<HTMLImageElement>(".expert-active-banner__avatar img");
ok(avatar?.getAttribute("src") === "file:///avatars/pixel-agent.png", "uses the expert avatar image");

const clearButton = document.querySelector<HTMLButtonElement>(".expert-active-banner__clear");
await act(async () => {
  clearButton?.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
  await flush();
});
ok(clearCount === 1, "clear button invokes the callback");

await act(async () => {
  root.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
