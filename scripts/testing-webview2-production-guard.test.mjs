import assert from "node:assert/strict";
import { productionDebugFlagFindings } from "./testing-webview2-production-guard.mjs";

assert.deepEqual(
  productionDebugFlagFindings([
    { path: "desktop/main.go", content: "normal production source" },
    { path: "desktop/frontend/e2e/probe.mjs", content: "connectOverCDP(endpoint)" },
    { path: "scripts/testing-webview2-cdp.ps1", content: "--remote-debugging-port=9222" },
  ]),
  [],
);

assert.deepEqual(
  productionDebugFlagFindings([
    { path: "desktop/main.go", content: `os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", value)` },
  ]),
  [{ path: "desktop/main.go", marker: "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS" }],
);

assert.deepEqual(
  productionDebugFlagFindings([
    { path: "desktop/main.go", content: `os.Getenv("KWU_TEST_WEBVIEW2_CDP_PORT")` },
  ]),
  [{ path: "desktop/main.go", marker: "KWU_TEST_WEBVIEW2_CDP_PORT" }],
);

assert.deepEqual(
  productionDebugFlagFindings([
    { path: "desktop/build/windows/installer/project.nsi", content: "--remote-debugging-port=9222" },
  ]),
  [{ path: "desktop/build/windows/installer/project.nsi", marker: "--remote-debugging-port" }],
);

console.log("production WebView2 debug guard tests: passed");
