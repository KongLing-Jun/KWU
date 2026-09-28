// Run: tsx src/__tests__/automation-bridge.test.ts

import {
  automationGetScheduleCatalog,
  automationListRunsV2,
  automationListDeliveryTargets,
  automationResolveCapabilities,
  automationReadRun,
  automationExportRunResult,
  heartbeatDeleteTaskTemplate,
  heartbeatListTemplates,
  heartbeatRestoreTaskTemplate,
  heartbeatSaveTaskTemplate,
  automationTemplateToHeartbeatTask,
  automationTaskToHeartbeatTask,
  decodeAutomationModelRoute,
  encodeAutomationModelRoute,
  heartbeatTaskToAutomationTask,
  subscribeAutomationChanges,
} from "../features/automation/heartbeat/heartbeat.bridge";

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

console.log("\nautomation bridge");

for (const route of [
  { model: "kwu/claude-fable-5", billingSource: "subscription_pool" as const },
  { model: "kwu/claude-opus-4-6", billingSource: "subscription_pool" as const },
  { model: "custom/chat-model", billingSource: "newapi_byok" as const },
]) {
  eq(
    decodeAutomationModelRoute(encodeAutomationModelRoute(route.model, route.billingSource)),
    route,
    `model route round trip keeps ${route.billingSource}/${route.model}`,
  );
}
eq(
  decodeAutomationModelRoute("legacy/chat-model"),
  { model: "legacy/chat-model", billingSource: undefined },
  "legacy model route remains source-less",
);
eq(
  decodeAutomationModelRoute(""),
  { model: undefined, billingSource: undefined },
  "empty model route clears both model and billing source",
);
for (const model of [":custom/chat-model", "custom/chat::variant"]) {
  eq(
    decodeAutomationModelRoute(encodeAutomationModelRoute(model, "newapi_byok")),
    { model, billingSource: "newapi_byok" },
    `model route round trip preserves model punctuation for ${model}`,
  );
}

const task = heartbeatTaskToAutomationTask({
  id: "task-1",
  title: "Task 1",
  prompt: "check status",
  interval: "30m",
  enabled: true,
  model: "deepseek/deepseek-v4-pro",
  billingSource: "newapi_byok",
  skills: ["review", "tdd"],
  role: "Reviewer",
	expertId: "expert-reviewer",
	effectiveFrom: "2026-08-01T09:30",
  scope: "project",
  workspaceRoot: "D:/project/KWU",
  approvalMode: "auto",
  timeWindowStart: "09:00",
  timeWindowEnd: "17:00",
	intervalWeekdays: ["mon", "wed", "fri"],
	timezone: "Asia/Shanghai",
});

eq(task.schedule, {
  kind: "interval",
  every: "30m",
  timezone: "Asia/Shanghai",
	weekdays: ["mon", "wed", "fri"],
  window: { start: "09:00", end: "17:00" },
}, "interval schedule maps to automation task");
eq(task.prompt, {
  prompt: "check status",
  model: "deepseek/deepseek-v4-pro",
  billingSource: "newapi_byok",
  skills: ["review", "tdd"],
  role: "Reviewer",
}, "prompt metadata maps to automation prompt");
eq(task.execution?.approvalMode, "auto", "approval mode maps to execution policy");
eq(task.capabilities?.expertId, "expert-reviewer", "expert capability stores the catalog ID instead of role text");
eq(task.effectiveRange?.from, new Date("2026-08-01T09:30").toISOString(), "datetime-local values serialize as RFC3339");

const cloudProjectTask = heartbeatTaskToAutomationTask({
	id: "cloud-project-task",
	title: "Cloud project task",
	prompt: "run with project context",
	interval: "1h",
	enabled: true,
	scope: "project",
	projectId: "cloud-project-1",
	workspaceRoot: "",
});
eq(cloudProjectTask.projectId, "cloud-project-1", "Cloud Project ownership survives task serialization");
eq(automationTaskToHeartbeatTask(cloudProjectTask).projectId, "cloud-project-1", "Cloud Project ownership survives the backend round trip");

const businessDiagnostics = {
  targetStart: "2026-06-21T00:00:00Z",
  targetEnd: "2026-06-28T00:00:00Z",
  region: "east",
  recipients: ["ops@example.com"],
  sources: [
    { id: "sales", kind: "sales" as const, type: "csv" as const, path: "D:/data/sales.csv" },
    { id: "inventory", kind: "inventory" as const, type: "xlsx" as const, path: "D:/data/inventory.xlsx" },
    {
      id: "purchase",
      kind: "purchase" as const,
      type: "mcp" as const,
      rows: [{ date: "2026-06-24", purchase_id: "P-1", sku: "SKU-1", quantity: 10, amount: 50, supplier: "A", region: "east" }],
    },
  ],
};

const businessTask = heartbeatTaskToAutomationTask({
  id: "task-business",
  title: "Weekly business diagnostics",
  prompt: "diagnose",
  interval: "168h|weekly:mon@03:30",
  enabled: false,
  businessDiagnostics,
});

eq(businessTask.businessDiagnostics, businessDiagnostics, "business diagnostics config maps to automation task");
eq(businessTask.schedule?.timezone, Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC", "calendar schedule keeps local timezone");

const heartbeat = automationTaskToHeartbeatTask({
  id: "task-2",
  name: "Calendar Task",
  enabled: false,
  prompt: { prompt: "report", model: "deepseek/deepseek-v4-flash", billingSource: "newapi_byok", skills: ["review"], role: "Analyst" },
  schedule: {
    kind: "calendar",
		timezone: "America/New_York",
    calendar: { frequency: "weekly", timeOfDay: "10:30", weekdays: ["mon", "fri"] },
  },
  execution: { approvalMode: "ask" },
  archive: { visible: true },
  createdAt: "2026-06-27T09:00:00Z",
  updatedAt: "2026-06-27T09:00:00Z",
  lastRunAt: "2026-06-28T09:00:00Z",
	nextRunAt: "2026-06-29T09:00:00Z",
	capabilities: { expertId: "expert-analyst" },
});

eq(heartbeat.interval, "168h|weekly:mon,fri@10:30", "calendar schedule maps to heartbeat interval");
eq(heartbeat.createdAt, Date.parse("2026-06-27T09:00:00Z"), "createdAt maps to millis");
eq(heartbeat.lastRunAt, Date.parse("2026-06-28T09:00:00Z"), "lastRunAt maps to millis");
eq(heartbeat.nextRunAt, "2026-06-29T09:00:00Z", "backend nextRunAt remains the card's authoritative RFC3339 value");
eq(heartbeat.newConversationEachRun, true, "archive visible maps to conversation flag");
eq(heartbeat.model, "deepseek/deepseek-v4-flash", "automation prompt model maps to heartbeat model");
eq(heartbeat.billingSource, "newapi_byok", "automation prompt billing source maps to heartbeat route");
eq(heartbeat.skills, ["review"], "automation prompt skills map to heartbeat skills");
eq(heartbeat.role, "Analyst", "automation prompt role maps to heartbeat role");
eq(heartbeat.expertId, "expert-analyst", "expert catalog ID survives the backend round trip");
eq(heartbeat.timezone, "America/New_York", "stored IANA timezone survives editor round trip");

const undatedHeartbeat = automationTaskToHeartbeatTask({
  id: "task-undated",
  name: "Long-lived automation",
  enabled: true,
  prompt: { prompt: "run forever" },
  schedule: { kind: "interval", every: "30m" },
  effectiveRange: {
    from: "0001-01-01T00:00:00Z",
    until: "0001-01-01T00:00:00Z",
  },
});
eq(undatedHeartbeat.effectiveFrom, undefined, "backend zero start time remains an empty effective range");
eq(undatedHeartbeat.effectiveUntil, undefined, "backend zero end time remains an empty effective range");

const intervalHeartbeat = automationTaskToHeartbeatTask({
  id: "task-interval-weekdays",
  name: "Weekday interval",
  enabled: true,
  prompt: { prompt: "poll" },
  schedule: { kind: "interval", every: "2h", weekdays: ["tue", "thu"] },
});
eq(intervalHeartbeat.intervalWeekdays, ["tue", "thu"], "interval weekday filters survive the backend round trip");

const businessHeartbeat = automationTaskToHeartbeatTask({
  id: "task-business-2",
  name: "Business Task",
  enabled: true,
  prompt: { prompt: "diagnose" },
  schedule: {
    kind: "calendar",
    calendar: { frequency: "weekly", timeOfDay: "03:30", weekdays: ["mon"] },
  },
  businessDiagnostics,
});

eq(businessHeartbeat.businessDiagnostics, businessDiagnostics, "business diagnostics config maps to heartbeat task");

const templateTask = automationTemplateToHeartbeatTask({
  id: "daily-ai-news",
  title: "每日 AI 新闻推送",
  description: "关注当天 AI 动向",
  prompt: "summarize AI news",
  interval: "24h|daily@09:00",
  icon: "news",
}, "task-from-template");

eq(templateTask, {
  id: "task-from-template",
  title: "每日 AI 新闻推送",
  prompt: "summarize AI news",
  interval: "24h|daily@09:00",
  enabled: true,
  approvalMode: "auto",
  newConversationEachRun: false,
}, "automation template seeds an unsaved add-task draft with its default interval");

const builtinTemplate = {
  id: "daily-ai-news",
  title: "Daily AI news",
  description: "Daily AI news fixture",
  prompt: "summarize AI news",
  interval: "24h|daily@09:00",
  icon: "news",
  kind: "task",
  source: "builtin",
};
const deletedTemplates = new Map<string, Record<string, unknown>>();
let templateFixture: Record<string, unknown>[] = [builtinTemplate];
const globalWithWindow = globalThis as Record<string, unknown>;
const previousWindow = globalWithWindow.window;
const automationEventCallbacks = new Map<string, (...payload: unknown[]) => void>();
const unsubscribedEvents: string[] = [];
let resolvedCapabilityTask: Record<string, unknown> | undefined;
globalWithWindow.window = {
  runtime: {
    EventsOn: (name: string, callback: (...payload: unknown[]) => void) => {
      automationEventCallbacks.set(name, callback);
      return () => { unsubscribedEvents.push(name); };
    },
    BrowserOpenURL: () => {},
  },
  go: {
    main: {
      App: {
        AutomationSaveTaskTemplate: async (savedTask: Record<string, unknown>) => {
          const prompt = savedTask.prompt as Record<string, unknown>;
          const saved = {
            id: `user-${savedTask.id}`,
            title: savedTask.name,
            description: "Saved task fixture",
            prompt: prompt.prompt,
            interval: (savedTask.schedule as Record<string, unknown>).every,
            icon: "task",
            kind: "task",
            source: "user",
          };
          templateFixture.push(saved);
          return saved;
        },
        AutomationListTemplates: async () => templateFixture,
        AutomationDeleteTaskTemplate: async (id: string) => {
          const deleted = templateFixture.find((template) => template.id === id);
          if (deleted) deletedTemplates.set(id, deleted);
          templateFixture = templateFixture.filter((template) => template.id !== id);
        },
        AutomationRestoreTaskTemplate: async (id: string) => {
          const restored = deletedTemplates.get(id);
          if (restored) templateFixture.push(restored);
        },
        AutomationReadRun: async () => ({
          availableResultFormats: ["md", "docx", "txt", "json", "xlsx", "pptx"].map((format) => ({ format })),
        }),
        AutomationGetScheduleCatalog: async () => [
          { id: "once", kind: "once", requiresRunAt: true },
          { id: "interval", kind: "interval", requiresEvery: true, requiresWeekdays: true },
          { id: "weekly", kind: "calendar", frequency: "weekly", requiresWeekdays: true },
        ],
        AutomationListDeliveryTargets: async () => [
          { type: "feishu", ref: "bot_target_v1_safe", label: "运营群", provider: "飞书", available: true, scope: "profile" },
          { type: "weixin_ilink", ref: "bot_target_v1_revoked", label: "值班群", provider: "微信", available: false, unavailableReason: "授权已撤销", scope: "workspace" },
        ],
        AutomationResolveCapabilities: async (candidate: Record<string, unknown>) => {
          resolvedCapabilityTask = candidate;
          return {
            modelRef: "custom/chat-model",
            skillIds: ["review"],
            connectorIds: ["github"],
            promptHash: "sha256:preflight",
            principal: "automation-user/work",
            capturedAt: "2026-08-01T12:00:00Z",
          };
        },
        AutomationListRunsV2: async (filter: Record<string, unknown>, cursor: string) => ({
          items: [{ runId: cursor ? "run-1" : "run-2", taskId: String(filter.taskId || "task-1"), status: "succeeded" }],
          nextCursor: cursor ? undefined : "cursor-2",
          hasMore: !cursor,
        }),
        AutomationExportRunResult: async (_taskID: string, runID: string, format: string) => ({
          filename: `automation-result-${runID}.${format}`,
          base64Encoded: true,
        }),
      },
    },
  },
};

const savedTemplate = await heartbeatSaveTaskTemplate({
  id: "task-manual",
  title: "Manual template",
  prompt: "manual prompt",
  interval: "45m",
  enabled: false,
  approvalMode: "auto",
  newConversationEachRun: false,
});

eq(savedTemplate, {
  id: "user-task-manual",
  title: "Manual template",
  description: "Saved task fixture",
  prompt: "manual prompt",
  interval: "45m",
  icon: "task",
  kind: "task",
  source: "user",
}, "manual task can be saved through the backend template binding");

eq((await heartbeatListTemplates()).some((template) => template.id === "user-task-manual"), true, "saved manual template appears in template list");
await heartbeatDeleteTaskTemplate("user-task-manual");
eq((await heartbeatListTemplates()).some((template) => template.id === "user-task-manual"), false, "manual template can be deleted through the backend template binding");
await heartbeatRestoreTaskTemplate("user-task-manual");
eq((await heartbeatListTemplates()).some((template) => template.id === "user-task-manual"), true, "manual template can be restored through the backend template binding");
eq((await heartbeatListTemplates()).some((template) => template.id === "daily-ai-news"), true, "built-in template appears before deletion");
await heartbeatDeleteTaskTemplate("daily-ai-news");
eq((await heartbeatListTemplates()).some((template) => template.id === "daily-ai-news"), false, "built-in template can be deleted through the backend template binding");
await heartbeatRestoreTaskTemplate("daily-ai-news");
eq((await heartbeatListTemplates()).some((template) => template.id === "daily-ai-news"), true, "built-in template can be restored through the backend template binding");

const runDetail = await automationReadRun("task-1", "run-1");
eq(
  runDetail.availableResultFormats?.map((format) => format.format),
  ["md", "docx", "txt", "json", "xlsx", "pptx"],
  "run detail exposes backend-provided export formats",
);

const exported = await automationExportRunResult("task-1", "run-1", "docx");

eq(exported.filename, "automation-result-run-1.docx", "run result export returns the selected filename");
eq(exported.base64Encoded, true, "run result export payload is marked base64");

const scheduleCatalog = await automationGetScheduleCatalog();
eq(scheduleCatalog.map((entry) => entry.id), ["once", "interval", "weekly"], "schedule editor options preserve backend catalog order");
eq(scheduleCatalog[1]?.requiresWeekdays, true, "interval weekday support comes from the backend catalog");

const deliveryTargetCatalog = await automationListDeliveryTargets();
eq(deliveryTargetCatalog, [
  { type: "feishu", ref: "bot_target_v1_safe", label: "运营群", provider: "飞书", available: true, scope: "profile" },
  { type: "weixin_ilink", ref: "bot_target_v1_revoked", label: "值班群", provider: "微信", available: false, unavailableReason: "授权已撤销", scope: "workspace" },
], "delivery target catalog preserves only the backend safe projection and opaque refs");

const capabilitySnapshot = await automationResolveCapabilities({
  id: "task-preflight",
  title: "Preflight task",
  prompt: "review this change",
  interval: "1h",
  enabled: true,
  model: "custom/chat-model",
  skills: ["review"],
  connectorIds: ["github"],
});
eq(resolvedCapabilityTask?.capabilities, {
  modelRef: "custom/chat-model",
  skillIds: ["review"],
  connectorIds: ["github"],
}, "capability preflight forwards the editor task through the canonical automation task contract");
eq(capabilitySnapshot, {
  modelRef: "custom/chat-model",
  skillIds: ["review"],
  connectorIds: ["github"],
  promptHash: "sha256:preflight",
  principal: "automation-user/work",
  capturedAt: "2026-08-01T12:00:00Z",
}, "capability preflight returns the strict backend snapshot without widening it");

const firstRunPage = await automationListRunsV2({ limit: 20 });
eq(firstRunPage, {
  items: [{ runId: "run-2", taskId: "task-1", status: "succeeded" }],
  nextCursor: "cursor-2",
  hasMore: true,
}, "global run history uses the cursor page contract");
const secondRunPage = await automationListRunsV2({ taskId: "task-1", limit: 20 }, firstRunPage.nextCursor);
eq(secondRunPage.items.map((run) => run.runId), ["run-1"], "run history forwards the next cursor instead of flattening per-task limits");

const changeKinds: string[] = [];
const unsubscribeAutomation = subscribeAutomationChanges((kind) => changeKinds.push(kind));
automationEventCallbacks.get("automation:task_changed")?.({ untrusted: "payload" });
automationEventCallbacks.get("automation:run_changed")?.("ignored");
automationEventCallbacks.get("automation:run_event")?.({ taskId: "task-1", runId: "run-1", sequence: 3, raw: "ignored" });
automationEventCallbacks.get("automation:delivery_changed")?.({ status: "forged" });
automationEventCallbacks.get("automation:catalog_changed")?.();
eq(changeKinds, ["task", "run", "run_event", "delivery", "catalog"], "automation events are reduced to invalidation kinds without trusting payloads");
unsubscribeAutomation();
unsubscribeAutomation();
eq(unsubscribedEvents.sort(), [
  "automation:catalog_changed",
  "automation:delivery_changed",
  "automation:run_changed",
  "automation:run_event",
  "automation:task_changed",
], "one unified unsubscribe detaches every automation invalidation listener exactly once");

if (previousWindow === undefined) delete globalWithWindow.window;
else globalWithWindow.window = previousWindow;

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
