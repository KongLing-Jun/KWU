// Run: tsx src/__tests__/automation-run-details.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { HeartbeatPanel } from "../features/automation/heartbeat/HeartbeatPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";

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

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.KeyboardEvent = dom.window.KeyboardEvent;
globalThis.CustomEvent = dom.window.CustomEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};
Object.defineProperty(globalThis.window, "matchMedia", {
  configurable: true,
  value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
});
Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });

window.runtime = { EventsOn: () => () => {}, BrowserOpenURL: () => {} };
window.go = {
  main: {
    App: {
      AutomationReloadTasks: async () => [{
        id: "task-detail",
        name: "每日运营摘要",
        enabled: true,
        prompt: { prompt: "summarize" },
        schedule: { kind: "cron", cron: "0 9 * * *" },
      }],
      AutomationTaskQuota: async () => ({ state: "ready", plan: "pro", used: 1, limit: 0, unlimited: true, canCreate: true, pausedTaskIds: [] }),
      AutomationListRuns: async () => [{
        runId: "run-detail",
        taskId: "task-detail",
        taskName: "每日运营摘要",
        status: "succeeded",
        startedAt: "2026-08-01T01:00:00Z",
      }],
      AutomationListTemplates: async () => [],
      AutomationGetScheduleCatalog: async () => [],
      AutomationListDeliveryTargets: async () => [],
      AutomationReadRun: async () => ({
        manifest: {
          runId: "run-detail",
          taskId: "task-detail",
          taskName: "每日运营摘要",
          status: "succeeded",
          trigger: "schedule",
          startedAt: "2026-08-01T01:00:00Z",
          finishedAt: "2026-08-01T01:01:05Z",
          durationMs: 65000,
          scope: "project",
          workspaceRoot: "D:/project/demo",
          schedule: { kind: "interval", every: "PT24H", weekdays: ["mon", "fri"], timezone: "Asia/Shanghai" },
          effectiveRange: { from: "2026-08-01T00:00:00Z", until: "2026-08-31T23:59:59Z" },
          summary: "摘要生成完成",
          resultPath: "runs/task-detail/run-detail/result.md",
          sessionPath: "runs/task-detail/run-detail/session.jsonl",
          eventsPath: "runs/task-detail/run-detail/events.jsonl",
          usage: { PromptTokens: 1200, CompletionTokens: 340, TotalTokens: 1540 },
          capabilities: {
            modelRef: "subscription/gpt-5",
						approvalMode: "yolo",
            principal: "user-1/profile-a",
            promptHash: "sha256:prompt",
            skillIds: ["daily-report"],
            expertId: "ops-expert",
            connectorIds: ["sales-read"],
            toolAllowlist: ["mcp__sales__read"],
            riskSummary: "只读数据访问",
            capturedAt: "2026-08-01T01:00:00Z",
          },
        },
        resultSummary: "# 今日摘要\n\n销售额稳定增长。",
        availableResultFormats: [{ format: "md", label: "Markdown" }],
        events: [
          { time: "2026-08-01T01:00:00Z", status: "queued", text: "进入队列" },
          { time: "2026-08-01T01:00:05Z", kind: "tool_input", text: "[已脱敏] connector credential" },
          { time: "2026-08-01T01:01:05Z", status: "succeeded", text: "运行完成" },
        ],
        deliveries: [
          { id: "delivery-ok", runId: "run-detail", status: "succeeded", retryable: true, target: { type: "feishu", ref: "ops" } },
          { id: "delivery-permanent", runId: "run-detail", status: "failed", retryable: false, target: { type: "weixin_ilink", ref: "duty" }, failure: { message: "目标已失效" } },
          { id: "delivery-retry", runId: "run-detail", status: "failed", retryable: true, target: { type: "feishu", ref: "alerts" }, failure: { message: "服务暂时不可用" } },
        ],
      }),
      ListWorkspaces: async () => [],
    } as Partial<AppBindings> as AppBindings,
  },
};

console.log("\nautomation run details");
const host = document.getElementById("root")!;
const root = createRoot(host);
const access: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
let openedTopic: [string, string, string, string | undefined] | null = null;
await act(async () => {
  root.render(
    <LocaleProvider>
      <HeartbeatPanel
        open
        embedded
        authenticated
        subscriptionAccess={access}
        onClose={() => {}}
        onOpenTopic={(scope, workspaceRoot, topicId, sessionPath) => {
          openedTopic = [scope, workspaceRoot, topicId, sessionPath];
        }}
      />
    </LocaleProvider>,
  );
});

await waitFor("task run", () => host.querySelector(".heartbeat-card__run") !== null);
await act(async () => {
  host.querySelector<HTMLButtonElement>(".heartbeat-card__run")?.click();
});
await waitFor("run detail", () => host.textContent?.includes("今日摘要") === true);

for (const heading of ["运行概要", "事件时间线", "结果与产物", "使用量", "通知投递", "权限与能力快照"]) {
  ok(host.textContent?.includes(heading) === true, `run detail clearly exposes ${heading}`);
}
const eventTimeline = [...host.querySelectorAll<HTMLDetailsElement>("details")]
  .find((details) => details.querySelector("summary")?.textContent?.trim() === "事件时间线");
ok(eventTimeline?.open === false, "the event timeline is collapsed by default, including redacted tool inputs");
await act(async () => { eventTimeline?.querySelector<HTMLElement>("summary")?.click(); });
ok(
  eventTimeline?.open === true && eventTimeline.textContent?.includes("[已脱敏] connector credential") === true,
  "users can explicitly expand the event timeline to inspect redacted events",
);
ok(
  host.textContent?.includes("1,540") === true && host.textContent?.includes("输入 Token") === true,
  "usage is readable instead of an opaque JSON blob",
);
ok(
  host.textContent?.includes("subscription/gpt-5") === true &&
    host.textContent?.includes("user-1/profile-a") === true &&
		host.textContent?.includes("mcp__sales__read") === true &&
		host.textContent?.includes("yolo") === true,
  "the frozen permission and capability snapshot is visible",
);
ok(
  host.textContent?.includes("result.md") === true && host.textContent?.includes("events.jsonl") === true,
  "result and archive artifacts remain visible",
);
ok(
  host.textContent?.includes("投递成功") === true && host.textContent?.includes("投递失败") === true,
  "notification delivery status is visible in the run detail",
);
const retryButtons = [...host.querySelectorAll<HTMLButtonElement>("button")]
  .filter((button) => button.textContent?.includes("重试投递"));
ok(
  retryButtons.length === 1 && retryButtons[0]?.getAttribute("aria-label")?.includes("alerts") === true,
  "only retryable failed notifications expose a retry action",
);
ok(
  host.textContent?.includes("Markdown") === true,
  "backend-provided export behavior remains available beside the result",
);
ok(
  host.textContent?.includes("PT24H") === true && host.textContent?.includes("Asia/Shanghai") === true && host.textContent?.includes("生效区间") === true,
  "the immutable schedule and effective range are readable in the run detail",
);
const openSessionButton = [...host.querySelectorAll<HTMLButtonElement>("button")]
  .find((button) => button.textContent?.includes("打开关联会话"));
await act(async () => openSessionButton?.click());
ok(
  JSON.stringify(openedTopic) === JSON.stringify(["project", "D:/project/demo", "task-detail", "runs/task-detail/run-detail/session.jsonl"]),
  "an archived run reopens the exact associated session through the existing topic navigation contract",
);

await act(async () => root.unmount());
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
