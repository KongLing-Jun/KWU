// Run: tsx src/__tests__/file-drop-multiplex.test.ts

import { JSDOM } from "jsdom";

let passed = 0;
let failed = 0;

function ok(value: unknown, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) ok(true, label);
  else ok(false, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

console.log("\nfile drop multiplex");

const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: "http://localhost/" });
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
globalThis.DataTransfer = dom.window.DataTransfer;

let nativeDrop: ((x: number, y: number, paths: string[]) => void) | undefined;
let registerCount = 0;
let offCount = 0;
let throwOnOff = false;

(window as unknown as {
  runtime: {
    OnFileDrop: (cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void;
    OnFileDropOff: () => void;
  };
}).runtime = {
  OnFileDrop: (cb) => {
    nativeDrop = cb;
    registerCount += 1;
  },
  OnFileDropOff: () => {
    offCount += 1;
    if (throwOnOff) {
      throw new DOMException(
        "Failed to execute 'send' on 'WebSocket': Still in CONNECTING state.",
        "InvalidStateError",
      );
    }
  },
};

const { onFilesDropped } = await import("../lib/bridge");
const calls: string[] = [];

const offA = onFilesDropped((paths, point) => calls.push(`a:${paths[0]}:${point?.x}:${point?.y}`));
const offB = onFilesDropped((paths, point) => calls.push(`b:${paths[0]}:${point?.x}:${point?.y}`));

eq(registerCount, 1, "native drop registers once for multiple listeners");
nativeDrop?.(12, 34, ["C:/data/sales.csv"]);
eq(calls.join("|"), "a:C:/data/sales.csv:12:34|b:C:/data/sales.csv:12:34", "native drop fans out with coordinates");

offA();
nativeDrop?.(1, 2, ["C:/data/inventory.xlsx"]);
eq(calls[calls.length - 1], "b:C:/data/inventory.xlsx:1:2", "removing one listener keeps the other active");
eq(offCount, 0, "native drop remains active while one listener remains");

offB();
eq(offCount, 1, "native drop unregisters after the last listener");

throwOnOff = true;
const offC = onFilesDropped(() => {});
let cleanupThrew = false;
try {
  offC();
} catch {
  cleanupThrew = true;
}
eq(cleanupThrew, false, "transient Wails disconnect does not escape file-drop cleanup");

throwOnOff = false;
const offD = onFilesDropped(() => {});
eq(registerCount, 3, "file drop can register again after disconnected cleanup");
offD();
eq(offCount, 3, "reconnected file drop unregisters normally");

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
