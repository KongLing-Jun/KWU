// Run: tsx src/__tests__/crash-reporting.test.ts

import { readFileSync } from "node:fs";
import { JSDOM } from "jsdom";

import { recordClientError, type AppBindings, type ClientErrorRecord } from "../lib/bridge";
import { installClientErrorLocalSink } from "../lib/clientError";

import {
  buildCrashPayload,
  buildPerformancePayload,
  crashFrameMetadata,
  formatPerformanceContext,
  globalCrashReportReason,
  installPerformancePressureMonitor,
  normalizeCrashError,
  parseReportedPerf,
  paintPerformancePrompt,
  performanceLabelForReason,
  reportCrash,
  installGlobalCrashHandlers,
  serializeReportedPerf,
  shouldRecordEventLoopLagSample,
  shouldPromptForPerformanceLabel,
  shouldReportGlobalCrashEvent,
  shouldRecordLongTaskSample,
  topFrameFromStack,
  type PerformanceSnapshot,
} from "../lib/crash";

let passed = 0;
let failed = 0;

function eq(a: unknown, b: unknown, label: string) {
  if (JSON.stringify(a) === JSON.stringify(b)) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\ncrash reporting");

const err = new TypeError("invalid argument");
err.stack = "TypeError: invalid argument\n    at submit (src/App.tsx:12:3)";
const payload = buildCrashPayload("unhandledrejection", err, "component stack");

eq(normalizeCrashError("boom"), { errorType: "string", errorMessage: "boom" }, "normalizes string reasons");
eq(topFrameFromStack(err.stack), "at submit (src/App.tsx:12:3)", "extracts top app frame");
eq(
  crashFrameMetadata("TypeError: canary message:999:1\n    at submit (src/App.tsx:12:3)\n    at render (node_modules/react-dom/index.js:40:5)\n    at invoke (wails://runtime.js:8:2)"),
  [
    { category: "app", ordinal: 0, line: 12, column: 3 },
    { category: "framework", ordinal: 1, line: 40, column: 5 },
    { category: "runtime", ordinal: 2, line: 8, column: 2 },
  ],
  "crash frame metadata only keeps enum and numeric fields",
);
eq(payload.kind, "exception", "unhandled rejection is a nonfatal exception kind");
eq(payload.source, "frontend.global", "global handler payload identifies source");
eq(payload.errorType, "TypeError", "captures error type");
eq(payload.componentStack, "component stack", "captures component stack");
eq(payload.message.includes("[unhandledrejection]"), true, "keeps human-readable message");
eq(shouldReportGlobalCrashEvent({ defaultPrevented: false }), true, "reports unhandled global events by default");
eq(shouldReportGlobalCrashEvent({ defaultPrevented: true }), false, "ignores global events already handled by a filter");
eq(
  shouldReportGlobalCrashEvent({ defaultPrevented: false, message: "ResizeObserver loop limit exceeded" }),
  false,
  "ignores Chromium ResizeObserver loop limit notices",
);
eq(
  shouldReportGlobalCrashEvent({
    defaultPrevented: false,
    message: "ResizeObserver loop completed with undelivered notifications.",
  }),
  false,
  "ignores Chromium ResizeObserver undelivered notification notices",
);
eq(
  shouldReportGlobalCrashEvent({ defaultPrevented: false, error: new Error("ResizeObserver loop limit exceeded") }),
  false,
  "ignores ResizeObserver notices delivered through ErrorEvent.error",
);
eq(
  shouldReportGlobalCrashEvent({
    defaultPrevented: false,
    message: "",
    error: new Error("ResizeObserver loop limit exceeded"),
  }),
  false,
  "checks ErrorEvent.error when ErrorEvent.message is empty",
);
eq(
  shouldReportGlobalCrashEvent({
    defaultPrevented: false,
    message: "Uncaught Error",
    error: new Error("ResizeObserver loop limit exceeded"),
  }),
  false,
  "checks ErrorEvent.error when ErrorEvent.message is a wrapper",
);
eq(
  globalCrashReportReason({
    defaultPrevented: false,
    message: "Script error.",
    filename: "wails://wails/assets/index-abc123.js",
    lineno: 42,
    colno: 7,
  }),
  "Script error.\nfilename=wails://wails/assets/index-abc123.js lineno=42 colno=7",
  "adds script location to opaque window.error messages",
);
eq(
  globalCrashReportReason({
    defaultPrevented: false,
    message: "Script error.",
  }),
  "Script error.",
  "keeps opaque script errors bare when WebView provides no location",
);

const perf: PerformanceSnapshot = {
  reason: "event loop lag 1300ms",
  uptimeMs: 42_000,
  visibility: "visible",
  focused: true,
  online: true,
  hardwareConcurrency: 10,
  deviceMemoryGb: 16,
  jsHeap: { usedMb: 700, totalMb: 780, limitMb: 900, usagePercent: 77.7 },
  eventLoopLag: { currentMs: 1300, maxMs: 1300, avgMs: 220, samples: 6 },
  longTasks: {
    count: 3,
    totalMs: 1800,
    maxMs: 900,
    recent: [
      { startMs: 40_000, durationMs: 900 },
      { startMs: 41_000, durationMs: 500 },
    ],
  },
  connection: { effectiveType: "4g", rttMs: 50, downlinkMbps: 20, saveData: false },
};
const perfPayload = buildPerformancePayload(perf);
eq(perfPayload.kind, "performance", "performance pressure reports use performance kind");
eq(perfPayload.source, "frontend.performance", "performance pressure reports identify source");
eq(perfPayload.label, "performance.lag", "performance pressure reports partition by stable pressure label");
eq(perfPayload.errorType, "PerformancePressure", "performance pressure reports use a stable error type");
eq(perfPayload.errorMessage.includes("1300"), false, "performance fingerprint message avoids dynamic durations");
eq(perfPayload.label.includes("1300"), false, "performance fingerprint label avoids dynamic durations");
eq(formatPerformanceContext(perf).includes("long tasks: 3"), true, "formats long task context");
eq(perfPayload.message.includes("event loop lag 1300ms"), true, "payload message keeps lag context");
eq(performanceLabelForReason("long task 900ms"), "performance.longtask", "labels long task pressure");
eq(performanceLabelForReason("js heap 87% of limit"), "performance.heap", "labels heap pressure");
eq(shouldRecordLongTaskSample(14_000, 900, 15_000), false, "ignores startup long tasks before grace ends");
eq(shouldRecordLongTaskSample(16_000, 40, 15_000), false, "ignores short long-task observer entries");
eq(shouldRecordLongTaskSample(16_000, 900, 15_000), true, "records post-grace long tasks");
eq(shouldRecordLongTaskSample(60_000, 900, 15_000, true, 20_000), false, "ignores long tasks while the window is hidden");
eq(shouldRecordLongTaskSample(23_000, 900, 15_000, false, 20_000), false, "ignores long tasks immediately after visibility resumes");
eq(shouldRecordLongTaskSample(26_000, 900, 15_000, false, 20_000), true, "records long tasks after the visibility resume grace period");
eq(shouldRecordLongTaskSample(570_000, 92, 15_000, false, 20_000, false), false, "ignores long tasks while unfocused");
eq(shouldRecordEventLoopLagSample(true, 60_000), false, "ignores event-loop lag while the window is hidden");
eq(shouldRecordEventLoopLagSample(false, 3_000), false, "ignores event-loop lag immediately after visibility resumes");
eq(shouldRecordEventLoopLagSample(false, 6_000), true, "records event-loop lag after the visibility resume grace period");
eq(shouldRecordEventLoopLagSample(false, 60_000, false), false, "ignores event-loop lag while unfocused");

eq(shouldPromptForPerformanceLabel(false, 11 * 60_000, false), true, "prompts an unhandled label past cooldown while visible");
eq(shouldPromptForPerformanceLabel(true, 11 * 60_000, false), false, "suppresses an already reported or dismissed label");
eq(shouldPromptForPerformanceLabel(false, 5 * 60_000, false), false, "respects the prompt cooldown window");
eq(shouldPromptForPerformanceLabel(false, 11 * 60_000, true), false, "never prompts while the window is hidden");
eq(shouldPromptForPerformanceLabel(false, 11 * 60_000, false, false), false, "never prompts while unfocused");

const reportedPerf = serializeReportedPerf(new Set(["performance.lag"]), "abc123");
eq([...parseReportedPerf(reportedPerf, "abc123")], ["performance.lag"], "round-trips reported labels for the same build");
eq([...parseReportedPerf(reportedPerf, "def456")], [], "re-surfaces reported labels on a new build");
eq([...parseReportedPerf(null, "abc123")], [], "tolerates missing storage");
eq([...parseReportedPerf("{not json", "abc123")], [], "tolerates corrupt storage");

const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: "http://localhost/" });
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
let reportCalls = 0;
const journalRecords: ClientErrorRecord[] = [];
installClientErrorLocalSink((record) => journalRecords.push(record));
(window as typeof window & { go?: unknown }).go = {
  main: {
    App: {
      ReportCrash: async () => { reportCalls += 1; },
      RecordClientError: async () => {},
    } as Partial<AppBindings> as AppBindings,
  },
};

const reactError = new Error("render failed password=secret C:\\Users\\alice\\private.ts");
reactError.stack = "Error: render failed\n    at render (src/App.tsx:12:3)";
reportCrash("react", reactError, "\n    at App (src/App.tsx:20:4)\n    at ReactDOM (node_modules/react-dom/index.js:30:2)");
eq(journalRecords.length, 1, "React crash 使用单次 client error submission");
eq(journalRecords[0]?.scope, "global.react", "React crash 使用固定 scope");
eq(journalRecords[0]?.stackFrames, [{ category: "app", ordinal: 0, line: 12, column: 3 }], "React crash 只传 stack 数值 metadata");
eq(
  journalRecords[0]?.componentFrames,
  [
    { category: "app", ordinal: 0, line: 20, column: 4 },
    { category: "framework", ordinal: 1, line: 30, column: 2 },
  ],
  "React crash 只传 component 数值 metadata",
);
const journalPayload = JSON.stringify(journalRecords[0]);
eq(journalPayload.includes("render failed"), false, "持久化 DTO 不包含 crash message");
eq(journalPayload.includes("private.ts"), false, "持久化 DTO 不包含 stack basename");
eq(document.querySelector(".crash-overlay__copy") !== null, true, "crash overlay keeps local copy action");
eq(document.querySelector(".crash-overlay__reload") !== null, true, "crash overlay offers a recovery action");
eq(document.getElementById("crash-overlay")?.getAttribute("role"), "alert", "crash overlay announces fatal errors");
eq(document.querySelector(".crash-overlay__send"), null, "crash overlay hides unavailable send action");

const stalePerformancePrompt = document.createElement("div");
stalePerformancePrompt.id = "performance-report-prompt";
document.body.appendChild(stalePerformancePrompt);
installPerformancePressureMonitor();
eq(document.getElementById("performance-report-prompt"), null, "disabled performance monitor removes a stale prompt");
paintPerformancePrompt(perfPayload, perf);
eq(document.getElementById("performance-report-prompt"), null, "disabled performance prompt never mounts frontend UI");
eq(reportCalls, 0, "rendering local diagnostics never calls the report bridge");

journalRecords.length = 0;
installGlobalCrashHandlers();
const windowError = new Error("window failure password=secret");
windowError.stack = "Error: window failure\n    at onLoad (src/main.tsx:44:9)";
window.dispatchEvent(new dom.window.ErrorEvent("error", {
  error: windowError,
  message: windowError.message,
  filename: "C:\\Users\\alice\\private.js",
  lineno: 44,
  colno: 9,
}));
const rejectionEvent = new dom.window.Event("unhandledrejection");
Object.defineProperty(rejectionEvent, "reason", { value: new TypeError("rejected body=secret"), enumerable: true });
window.dispatchEvent(rejectionEvent);
eq(journalRecords.length, 2, "window.error 与 unhandledrejection 各自只提交一条");
eq(journalRecords[0]?.scope, "global.window.error", "window.error 使用固定 scope");
eq(journalRecords[1]?.scope, "global.unhandledrejection", "unhandledrejection 使用固定 scope");

let unhandledRejections = 0;
const rejectionObserver = () => { unhandledRejections += 1; };
window.addEventListener("unhandledrejection", rejectionObserver);
delete (window as typeof window & { go?: unknown }).go;
recordClientError({ scope: "workspace.preview", code: "NETWORK_ERROR", errorType: "Error", occurredAt: new Date().toISOString() });
await new Promise((resolve) => setTimeout(resolve, 0));
eq(unhandledRejections, 0, "缺失 Wails binding 不产生 unhandled rejection");
(window as typeof window & { go?: unknown }).go = {
  main: {
    App: {
      RecordClientError: async () => { throw new Error("journal unavailable"); },
    } as Partial<AppBindings> as AppBindings,
  },
};
recordClientError({ scope: "workspace.preview", code: "NETWORK_ERROR", errorType: "Error", occurredAt: new Date().toISOString() });
await new Promise((resolve) => setTimeout(resolve, 0));
eq(unhandledRejections, 0, "rejected Wails binding 不产生 unhandled rejection");
window.removeEventListener("unhandledrejection", rejectionObserver);
installClientErrorLocalSink();

const botsSettingsSource = readFileSync(new URL("../features/settings/ui/BotsSettingsSection.tsx", import.meta.url), "utf8");
const updatesSettingsSource = readFileSync(new URL("../features/settings/ui/UpdatesSettingsSection.tsx", import.meta.url), "utf8");
eq(botsSettingsSource.includes("app.ReportCrash"), false, "bot diagnostics do not expose a send path");
eq(botsSettingsSource.includes('t("settings.botCopyDiagnostic")'), true, "bot diagnostics keep the copy action");
eq(botsSettingsSource.includes('t("settings.botSendDiagnostic")'), false, "bot diagnostics hide the send action");
eq(updatesSettingsSource.includes('t("settings.telemetryLabel")'), false, "settings hide unavailable telemetry controls");
eq(updatesSettingsSource.includes('t("settings.metricsLabel")'), false, "settings hide unavailable metrics controls");
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
