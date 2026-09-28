// Run: tsx src/__tests__/bridge-runtime-required.test.ts

import { app, onEvent, onModelRouteChanged, onProjectTreeChanged, onReady, onUpdaterProgress, waitForBackendReady } from "../lib/bridge";

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

function ok(value: unknown, label: string) {
  eq(Boolean(value), true, label);
}

console.log("\nbridge runtime required");

const globalWithWindow = globalThis as Record<string, unknown>;
const previousWindow = globalWithWindow.window;
const eventNames: string[] = [];
const eventCallbacks = new Map<string, (...args: unknown[]) => void>();
let readySignalUnsubscribed = false;
globalWithWindow.window = {
  setTimeout(handler: () => void, timeout: number) {
    return globalThis.setTimeout(handler, timeout);
  },
  clearTimeout(timer: ReturnType<typeof globalThis.setTimeout>) {
    globalThis.clearTimeout(timer);
  },
  runtime: {
    EventsOn(name: string, callback: (...args: unknown[]) => void) {
      eventNames.push(name);
      eventCallbacks.set(name, callback);
      return () => {
        if (name === "agent:ready") readySignalUnsubscribed = true;
      };
    },
    BrowserOpenURL() {},
  },
};

let missingBindingError = "";
try {
  await app.Platform();
} catch (err) {
  missingBindingError = err instanceof Error ? err.message : String(err);
}

ok(
  missingBindingError.includes("bindings are missing") && missingBindingError.includes("browser mock data is disabled"),
  "desktop runtime without App bindings fails closed",
);

onEvent(() => {});
onUpdaterProgress(() => {});
onReady(() => {});
onProjectTreeChanged(() => {});
let changedModelTab = "";
onModelRouteChanged((tabID) => { changedModelTab = tabID; });
ok(eventNames.includes("agent:event"), "desktop runtime uses real event channel without App binding");
ok(eventNames.includes("updater:progress"), "desktop runtime uses real updater channel without App binding");
ok(eventNames.includes("agent:ready"), "desktop runtime uses real ready channel without App binding");
ok(eventNames.includes("project-tree:changed"), "desktop runtime uses real project-tree channel without App binding");
ok(eventNames.includes("model-route:changed"), "desktop runtime uses the cross-client model-route channel");
eventCallbacks.get("model-route:changed")?.("tab-two");
eq(changedModelTab, "tab-two", "model-route events preserve the affected tab ID");

let ready = false;
let readyChecks = 0;
const readySignal = waitForBackendReady(async () => {
  readyChecks += 1;
  return ready;
});
await Promise.resolve();
await Promise.resolve();
ready = true;
eventCallbacks.get("agent:ready")?.();
await readySignal;
ok(readySignalUnsubscribed, "ready signal wakes immediately and removes its temporary listener");
eq(readyChecks, 2, "ready waiter re-checks once after the backend event");

if (previousWindow === undefined) {
  delete globalWithWindow.window;
} else {
  globalWithWindow.window = previousWindow;
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
