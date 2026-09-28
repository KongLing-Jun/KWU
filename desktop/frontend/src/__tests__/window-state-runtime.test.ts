// Run: tsx src/__tests__/window-state-runtime.test.ts

import { isNativeWailsWindow } from "../lib/windowState";

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

console.log("\nwindow state runtime");

ok(
  !isNativeWailsWindow({ runtime: { WindowGetSize() {} } }),
  "plain browser runtime proxy is not treated as a native window",
);
ok(
  isNativeWailsWindow({
    runtime: { WindowGetSize() {} },
    chrome: { webview: { postMessage() {} } },
  }),
  "WebView2 host is treated as a native window",
);
ok(
  isNativeWailsWindow({
    runtime: { WindowGetSize() {} },
    webkit: { messageHandlers: { external: { postMessage() {} } } },
  }),
  "WebKit host is treated as a native window",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
