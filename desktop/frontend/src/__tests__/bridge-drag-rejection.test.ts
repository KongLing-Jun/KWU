// Run: tsx src/__tests__/bridge-drag-rejection.test.ts

import { JSDOM } from "jsdom";
import { installWailsNonFileDragErrorSuppression, isWailsNonFileDragError, isWailsNonFileDragErrorEvent } from "../lib/bridge";

let passed = 0;
let failed = 0;

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\nbridge drag rejection filtering");

eq(
  isWailsNonFileDragError(new Error("additional File object is not a file on the disk")),
  true,
  "suppresses Wails' explicit non-file drag error",
);
eq(isWailsNonFileDragError(new Error("invalid argument")), false, "does not suppress generic invalid argument");
eq(
  isWailsNonFileDragError(new Error("invalid argument"), true),
  true,
  "suppresses invalid argument only after a native file drag",
);
eq(
  isWailsNonFileDragError(new TypeError("invalid argument"), false),
  false,
  "keeps non-drag TypeError invalid argument visible",
);
eq(
  isWailsNonFileDragError("network invalid argument", true),
  false,
  "does not suppress broader messages that merely contain invalid argument",
);
eq(
  isWailsNonFileDragError("Uncaught TypeError: invalid argument", true),
  true,
  "normalizes Chromium's window.error message prefix",
);
eq(
  isWailsNonFileDragErrorEvent({ message: "Uncaught TypeError: invalid argument", error: undefined }, true),
  true,
  "suppresses invalid argument delivered through ErrorEvent.message",
);
eq(
  isWailsNonFileDragErrorEvent({ message: "Uncaught TypeError: invalid argument", error: new TypeError("invalid argument") }, true),
  true,
  "suppresses invalid argument delivered through ErrorEvent.error.message",
);
eq(
  isWailsNonFileDragErrorEvent({ message: "Uncaught TypeError: invalid argument", error: new TypeError("invalid argument") }, false),
  false,
  "keeps ErrorEvent invalid argument visible without a recent native file drag",
);

const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: "http://localhost/" });
globalThis.window = dom.window as unknown as Window & typeof globalThis;
const uninstall = installWailsNonFileDragErrorSuppression();
const fileDrag = new window.Event("dragover", { bubbles: true, cancelable: true });
const fileTransfer = { files: { length: 1 }, types: ["Files"], dropEffect: "none" };
Object.defineProperty(fileDrag, "dataTransfer", { value: fileTransfer });
window.dispatchEvent(fileDrag);
eq(fileDrag.defaultPrevented, true, "prevents browser navigation for native file drags");
eq(fileTransfer.dropEffect, "copy", "marks native file drags as copy operations");

const textDrag = new window.Event("dragover", { bubbles: true, cancelable: true });
Object.defineProperty(textDrag, "dataTransfer", { value: { files: { length: 0 }, types: ["text/plain"], dropEffect: "none" } });
window.dispatchEvent(textDrag);
eq(textDrag.defaultPrevented, false, "does not block non-file drags");
uninstall();
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
