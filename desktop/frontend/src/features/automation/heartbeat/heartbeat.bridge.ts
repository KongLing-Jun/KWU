// Heartbeat panel bridge — typed wrappers around automation bindings.
// Custom components should import from here instead of calling app.* directly
// so that the legacy panel can migrate without duplicating automation UI.

import { app } from "../../../lib/bridge";
import { getWailsRuntime } from "../../../platform/wails/wailsRuntime";
import type {
  AutomationCancelRunResponse,
	AutomationCapabilitySnapshot,
	AutomationDeliveryAttempt,
  AutomationDeliveryTargetCatalogEntry,
  AutomationRunDetail,
  AutomationRunFilter,
  AutomationRunPage,
  AutomationRunResultExport,
  AutomationRunResultExportFormat,
  AutomationRunSummary,
  AutomationSchedule,
  AutomationScheduleCatalogEntry,
  AutomationTask,
  AutomationTaskQuota,
  AutomationTaskTemplate,
  HeartbeatTask,
} from "./heartbeat.types";

type AutomationV2Bindings = {
	AutomationGetScheduleCatalog(): Promise<unknown>;
	AutomationListDeliveryTargets(): Promise<unknown>;
	AutomationListRunsV2(filter: AutomationRunFilter, cursor: string): Promise<unknown>;
};

const automationV2 = app as typeof app & AutomationV2Bindings;

export type AutomationChangeKind = "task" | "run" | "run_event" | "delivery" | "catalog";

const AUTOMATION_CHANGE_EVENTS: ReadonlyArray<readonly [string, AutomationChangeKind]> = [
	["automation:task_changed", "task"],
	["automation:run_changed", "run"],
	["automation:run_event", "run_event"],
	["automation:delivery_changed", "delivery"],
	["automation:catalog_changed", "catalog"],
];

export function subscribeAutomationChanges(onChange: (kind: AutomationChangeKind) => void): () => void {
	const runtime = getWailsRuntime();
	if (!runtime) return () => {};
	const unsubscribe: Array<() => void> = [];
	try {
		for (const [eventName, kind] of AUTOMATION_CHANGE_EVENTS) {
			unsubscribe.push(runtime.EventsOn(eventName, () => onChange(kind)));
		}
	} catch {
		unsubscribe.forEach((off) => off());
		return () => {};
	}
	let active = true;
	return () => {
		if (!active) return;
		active = false;
		unsubscribe.forEach((off) => off());
	};
}

export function heartbeatListTasks(): Promise<HeartbeatTask[]> {
  return app.AutomationReloadTasks().then((v) => ((v ?? []) as AutomationTask[]).map(automationTaskToHeartbeatTask));
}

export function heartbeatSaveTasks(tasks: HeartbeatTask[]): Promise<void> {
  return app.AutomationSaveTasks(tasks.map(heartbeatTaskToAutomationTask) as unknown);
}

export function heartbeatTaskQuota(): Promise<AutomationTaskQuota> {
  return app.AutomationTaskQuota().then((value) => value as AutomationTaskQuota);
}

export function heartbeatTriggerNow(id: string): Promise<void> {
  return app.AutomationTriggerNow(id).then(() => undefined);
}

export function heartbeatGenerateID(): Promise<string> {
  return app.AutomationGenerateID();
}

export function encodeAutomationModelRoute(model: string, billingSource?: HeartbeatTask["billingSource"]): string {
  return billingSource ? `${billingSource}::${model}` : model;
}

export function decodeAutomationModelRoute(value: string): Pick<HeartbeatTask, "model" | "billingSource"> {
  const splitAt = value.indexOf("::");
  if (splitAt < 0) {
    return { model: value || undefined, billingSource: undefined };
  }
  return {
    model: value.slice(splitAt + "::".length) || undefined,
    billingSource: value.slice(0, splitAt) as HeartbeatTask["billingSource"],
  };
}

export function heartbeatListTemplates(): Promise<AutomationTaskTemplate[]> {
  return app.AutomationListTemplates().then((v) => Array.isArray(v) ? (v as AutomationTaskTemplate[]) : []);
}

export function heartbeatSaveTaskTemplate(task: HeartbeatTask): Promise<AutomationTaskTemplate> {
  return app.AutomationSaveTaskTemplate(heartbeatTaskToAutomationTask(task) as unknown).then((v) => v as AutomationTaskTemplate);
}

export function heartbeatDeleteTaskTemplate(id: string): Promise<void> {
  return app.AutomationDeleteTaskTemplate(id).then(() => undefined);
}

export function heartbeatRestoreTaskTemplate(id = ""): Promise<void> {
  return app.AutomationRestoreTaskTemplate(id).then(() => undefined);
}

export function automationTemplateToHeartbeatTask(template: AutomationTaskTemplate, id: string): HeartbeatTask {
  return {
    id,
    title: template.title,
    prompt: template.prompt,
    interval: template.interval || "30m",
    enabled: true,
    approvalMode: "auto",
    newConversationEachRun: false,
  };
}

export function automationListRuns(taskID: string, limit = 3): Promise<AutomationRunSummary[]> {
  return app.AutomationListRuns(taskID, limit).then((v) => Array.isArray(v) ? (v as AutomationRunSummary[]) : []);
}

export function automationReadRun(taskID: string, runID: string): Promise<AutomationRunDetail> {
  return app.AutomationReadRun(taskID, runID).then((v) => (v ?? {}) as AutomationRunDetail);
}

export function automationExportRunResult(taskID: string, runID: string, format: AutomationRunResultExportFormat): Promise<AutomationRunResultExport> {
  return app.AutomationExportRunResult(taskID, runID, format).then((v) => (v ?? {}) as AutomationRunResultExport);
}

export function automationCancelRun(taskID: string, runID: string): Promise<AutomationCancelRunResponse> {
  return app.AutomationCancelRun(taskID, runID).then((v) => (v ?? { taskId: taskID, runId: runID, canceled: false, status: "not_cancelable" }) as AutomationCancelRunResponse);
}

export function automationListRunsV2(filter: AutomationRunFilter, cursor = ""): Promise<AutomationRunPage> {
	return Promise.resolve().then(() => automationV2.AutomationListRunsV2(filter, cursor)).then((value) => {
		const page = (value ?? {}) as Partial<AutomationRunPage>;
		return {
			items: Array.isArray(page.items) ? page.items : [],
			nextCursor: typeof page.nextCursor === "string" && page.nextCursor ? page.nextCursor : undefined,
			hasMore: page.hasMore === true,
		};
	});
}

export function automationGetScheduleCatalog(): Promise<AutomationScheduleCatalogEntry[]> {
	return Promise.resolve().then(() => automationV2.AutomationGetScheduleCatalog()).then((value) => (
		Array.isArray(value) ? value as AutomationScheduleCatalogEntry[] : []
	));
}

export function automationListDeliveryTargets(): Promise<AutomationDeliveryTargetCatalogEntry[]> {
	return Promise.resolve().then(() => automationV2.AutomationListDeliveryTargets()).then((value) => (
		Array.isArray(value) ? value as AutomationDeliveryTargetCatalogEntry[] : []
	));
}

export function automationResolveCapabilities(task: HeartbeatTask): Promise<AutomationCapabilitySnapshot> {
	return app.AutomationResolveCapabilities(heartbeatTaskToAutomationTask(task) as unknown)
		.then((value) => value as AutomationCapabilitySnapshot);
}

export function heartbeatCreateTask(task: HeartbeatTask): Promise<HeartbeatTask> {
	return app.AutomationCreateTask(heartbeatTaskToAutomationTask(task) as unknown)
		.then((value) => automationTaskToHeartbeatTask(value as AutomationTask));
}

export function heartbeatUpdateTask(task: HeartbeatTask): Promise<HeartbeatTask> {
	if (!task.revision) return Promise.reject(new Error("task revision is required"));
	return app.AutomationUpdateTask(heartbeatTaskToAutomationTask(task) as unknown, task.revision)
		.then((value) => automationTaskToHeartbeatTask(value as AutomationTask));
}

export function heartbeatSetTaskEnabled(task: HeartbeatTask, enabled: boolean): Promise<HeartbeatTask> {
	if (!task.revision) return Promise.reject(new Error("task revision is required"));
	return app.AutomationSetTaskEnabled(task.id, enabled, task.revision)
		.then((value) => automationTaskToHeartbeatTask(value as AutomationTask));
}

export function heartbeatDeleteTask(task: HeartbeatTask): Promise<void> {
	if (!task.revision) return Promise.reject(new Error("task revision is required"));
	return app.AutomationDeleteTask(task.id, task.revision);
}

export function automationListDeliveries(taskID: string, runID: string): Promise<AutomationDeliveryAttempt[]> {
	return app.AutomationListDeliveries(taskID, runID).then((value) => Array.isArray(value) ? value as AutomationDeliveryAttempt[] : []);
}

export function automationRetryDelivery(taskID: string, runID: string, attemptID: string): Promise<AutomationDeliveryAttempt> {
	return app.AutomationRetryDelivery(taskID, runID, attemptID).then((value) => value as AutomationDeliveryAttempt);
}

// 过渡期前端仍复用 HeartbeatPanel；这里把旧表单字段收敛成新的 automation Task JSON。
export function heartbeatTaskToAutomationTask(task: HeartbeatTask, now = Date.now()): AutomationTask {
  const createdAt = task.createdAt ? new Date(task.createdAt).toISOString() : new Date(now).toISOString();
  return {
		schemaVersion: 2,
		revision: task.revision,
    id: task.id,
    name: task.title.trim() || "未命名任务",
    enabled: task.enabled,
    scope: task.scope || "global",
		projectId: emptyToUndefined(task.projectId),
    workspaceRoot: task.scope === "project" ? task.workspaceRoot || "" : "",
    topicId: task.topicId,
    newConversationEachRun: !!task.newConversationEachRun,
    prompt: {
      prompt: task.prompt,
      model: emptyToUndefined(task.model),
      billingSource: task.billingSource,
      skills: nonEmptyStrings(task.skills),
      role: emptyToUndefined(task.role),
    },
    schedule: heartbeatIntervalToAutomationSchedule(task),
    conditions: {
      conditionPrompt: emptyToUndefined(task.conditionPrompt),
      failureCooldown: emptyToUndefined(task.failureCooldown),
    },
    execution: {
      approvalMode: task.approvalMode || "auto",
    },
    archive: {},
		capabilities: {
			modelRef: emptyToUndefined(task.model),
			skillIds: nonEmptyStrings(task.skills),
			expertId: emptyToUndefined(task.expertId),
			connectorIds: nonEmptyStrings(task.connectorIds),
		},
		effectiveRange: {
			from: localDateTimeToRFC3339(task.effectiveFrom),
			until: localDateTimeToRFC3339(task.effectiveUntil),
		},
		missedRunPolicy: task.missedRunPolicy || "latest_once",
		delivery: {
			enabled: !!task.deliveryEnabled,
			targets: task.deliveryEnabled ? task.deliveryTargets : undefined,
		},
    createdAt,
    updatedAt: new Date(now).toISOString(),
    lastRunAt: task.lastRunAt ? new Date(task.lastRunAt).toISOString() : undefined,
    lastRunId: task.lastRunId,
    businessDiagnostics: task.businessDiagnostics,
  };
}

// 后端只保存 automation Task；列表渲染前转回 HeartbeatTask，减少 UI 改名带来的大 diff。
export function automationTaskToHeartbeatTask(task: AutomationTask): HeartbeatTask {
  return {
    id: task.id,
		revision: task.revision,
    title: task.name,
    prompt: task.prompt?.prompt || "",
    interval: automationScheduleToHeartbeatInterval(task.schedule),
    enabled: task.enabled,
    scope: task.scope,
		projectId: task.projectId,
    workspaceRoot: task.workspaceRoot,
    topicId: task.topicId,
    lastRunAt: millis(task.lastRunAt),
    lastRunId: task.lastRunId,
		nextRunAt: task.nextRunAt,
    newConversationEachRun: !!task.newConversationEachRun || !!task.archive?.visible || !!task.archive?.keepSession,
    createdAt: millis(task.createdAt),
    approvalMode: task.execution?.approvalMode,
    timeWindowStart: task.schedule?.window?.start,
    timeWindowEnd: task.schedule?.window?.end,
		intervalWeekdays: nonEmptyStrings(task.schedule?.weekdays),
		timezone: task.schedule?.timezone,
    model: task.prompt?.model,
    billingSource: task.prompt?.billingSource,
    skills: nonEmptyStrings(task.prompt?.skills),
		role: task.prompt?.role,
		expertId: task.capabilities?.expertId,
		connectorIds: nonEmptyStrings(task.capabilities?.connectorIds),
		effectiveFrom: rfc3339ToLocalDateTime(task.effectiveRange?.from),
		effectiveUntil: rfc3339ToLocalDateTime(task.effectiveRange?.until),
		missedRunPolicy: task.missedRunPolicy || "latest_once",
		deliveryEnabled: !!task.delivery?.enabled,
		deliveryTargets: task.delivery?.targets,
    conditionPrompt: task.conditions?.conditionPrompt,
    failureCooldown: task.conditions?.failureCooldown,
    businessDiagnostics: task.businessDiagnostics,
  };
}

function heartbeatIntervalToAutomationSchedule(task: HeartbeatTask): AutomationSchedule {
  const interval = task.interval || "30m";
  if (interval.startsWith("once@")) {
    return { kind: "once", runAt: localDateTimeToRFC3339(interval.slice("once@".length)) };
  }
  const cycle = interval.match(/^(\d+)[smh]\|(daily|weekly|biweekly|monthly|yearly)(?::([^@]*))?(?:@(\d{2}:\d{2}))?$/);
  if (cycle) {
    const [, , rawFrequency, rawDays = "", timeOfDay = "09:00"] = cycle;
    const days = rawDays.split(",").filter(Boolean);
    const timezone = task.timezone || localTimezone();
    if (rawFrequency === "monthly") {
      return { kind: "calendar", timezone, calendar: { frequency: "monthly", timeOfDay, monthDays: numberList(days, [1]) } };
    }
    if (rawFrequency === "yearly") {
      const [month = "1", day = "1"] = rawDays.split("-");
      return { kind: "calendar", timezone, calendar: { frequency: "yearly", timeOfDay, months: numberList([month], [1]), monthDays: numberList([day], [1]) } };
    }
    if (rawFrequency === "weekly" || rawFrequency === "biweekly") {
      return { kind: "calendar", timezone, calendar: { frequency: rawFrequency, timeOfDay, weekdays: days.length ? days : ["mon"] } };
    }
    return { kind: "calendar", timezone, calendar: { frequency: "daily", timeOfDay } };
  }
  const schedule: AutomationSchedule = {
    kind: "interval",
    every: interval.replace(/\|.*$/, "") || "30m",
    timezone: task.timezone || localTimezone(),
		weekdays: nonEmptyStrings(task.intervalWeekdays),
  };
  if (task.timeWindowStart || task.timeWindowEnd) {
    schedule.window = { start: task.timeWindowStart, end: task.timeWindowEnd };
  }
  return schedule;
}

function localDateTimeToRFC3339(value?: string): string | undefined {
	const trimmed = value?.trim();
	if (!trimmed) return undefined;
	const parsed = new Date(trimmed);
	return Number.isNaN(parsed.getTime()) ? trimmed : parsed.toISOString();
}

function rfc3339ToLocalDateTime(value?: string): string | undefined {
	if (!value || value === "0001-01-01T00:00:00Z") return undefined;
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return value;
	const local = new Date(parsed.getTime() - parsed.getTimezoneOffset() * 60_000);
	return local.toISOString().slice(0, 16);
}

function automationScheduleToHeartbeatInterval(schedule: AutomationSchedule | undefined): string {
  if (!schedule) return "30m";
  if (schedule.kind === "once") return `once@${rfc3339ToLocalDateTime(schedule.runAt) || ""}`;
  if (schedule.kind === "interval") return schedule.every || "30m";
  const cal = schedule.calendar || {};
  const time = cal.timeOfDay || "09:00";
  if (cal.frequency === "weekly") return `168h|weekly:${(cal.weekdays || ["mon"]).join(",")}@${time}`;
	if (cal.frequency === "biweekly") return `336h|biweekly:${(cal.weekdays || ["mon"]).join(",")}@${time}`;
  if (cal.frequency === "monthly") return `720h|monthly:${(cal.monthDays || [1]).join(",")}@${time}`;
  if (cal.frequency === "yearly") return `8760h|yearly:${(cal.months || [1])[0]}-${(cal.monthDays || [1])[0]}@${time}`;
  return `24h|daily@${time}`;
}

function numberList(values: string[], fallback: number[]): number[] {
  const out = values.map((v) => Number.parseInt(v, 10)).filter((v) => Number.isFinite(v) && v > 0);
  return out.length ? out : fallback;
}

function millis(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
}

function emptyToUndefined(value: string | undefined): string | undefined {
  const trimmed = (value || "").trim();
  return trimmed || undefined;
}

function nonEmptyStrings(values: string[] | undefined): string[] | undefined {
  const out = (values || []).map((value) => value.trim()).filter(Boolean);
  return out.length ? out : undefined;
}

function localTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}
