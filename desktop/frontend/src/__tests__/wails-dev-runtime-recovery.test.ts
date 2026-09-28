// Run: tsx src/__tests__/wails-dev-runtime-recovery.test.ts

import {
  isTransientWailsConnectionError,
  waitForWailsTransport,
} from "../lib/wailsDevRuntime";

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

console.log("\nwails dev runtime recovery");

ok(
  isTransientWailsConnectionError(
    new DOMException("Failed to execute 'send' on 'WebSocket': Still in CONNECTING state.", "InvalidStateError"),
  ),
  "classifies WebSocket CONNECTING errors as transient",
);
ok(
  isTransientWailsConnectionError(new TypeError("Cannot read properties of null (reading 'send')")),
  "classifies Wails null socket errors as transient",
);
ok(!isTransientWailsConnectionError(new Error("permission denied")), "does not hide unrelated runtime errors");

let attempts = 0;
const connected = await waitForWailsTransport(
  () => {
    attempts += 1;
    if (attempts === 1) {
      throw new DOMException("Still in CONNECTING state.", "InvalidStateError");
    }
  },
  {
    timeoutMs: 50,
    retryMs: 0,
    sleep: async () => {},
  },
);
ok(connected && attempts === 2, "waits through a transient connection race");

let unrelatedError = "";
try {
  await waitForWailsTransport(
    () => {
      throw new Error("permission denied");
    },
    {
      timeoutMs: 50,
      retryMs: 0,
      sleep: async () => {},
    },
  );
} catch (error) {
  unrelatedError = error instanceof Error ? error.message : String(error);
}
ok(unrelatedError === "permission denied", "surfaces non-connection failures");

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
