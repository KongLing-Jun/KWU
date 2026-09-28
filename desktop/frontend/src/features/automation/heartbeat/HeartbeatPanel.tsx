// Heartbeat Panel — legacy modal now backed by automation task storage.
//
// Renders a list of tasks with add/edit/delete controls, plus a manual
// "run now" button for each. The panel is opened from the sidebar nav item.

import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, RefObject } from "react";
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  ChevronLeft,
  ChevronsUpDown,
  CircleHelp,
  Clock,
  Check,
  Clipboard,
  Download,
  FileImage,
  FileText,
  GraduationCap,
  Hammer,
  Heart,
  MessageSquare,
  Play,
  Plus,
  RefreshCw,
  Search,
  Square,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { app, onFilesDropped, onProjectTreeChanged } from "../../../lib/bridge";
import { useAutomationModels } from "./useAutomationModels";
import { clientErrorCode, reportClientError } from "../../../lib/clientError";
import { expertAvatarSrcForInfo } from "../../../lib/expertAssets";
import { useT } from "../../../lib/i18n";
import { modelProviderLabel } from "../../../lib/modelCatalogOrdering";
import {
  filterModelInfosForSubscriptionAccess,
  resolveModelRouteAccess,
  type SubscriptionAccess,
} from "../../../lib/subscriptionModelAccess";
import { AnchoredPopover } from "../../../components/AnchoredPopover";
import { Markdown } from "../../../components/Markdown";
import {
  heartbeatListTasks,
  heartbeatSaveTasks,
	heartbeatCreateTask,
	heartbeatUpdateTask,
	heartbeatSetTaskEnabled,
	heartbeatDeleteTask,
	automationGetScheduleCatalog,
	automationListDeliveryTargets,
	automationResolveCapabilities,
  heartbeatTaskQuota,
  heartbeatTriggerNow,
  heartbeatGenerateID,
  heartbeatListTemplates,
  heartbeatSaveTaskTemplate,
  heartbeatDeleteTaskTemplate,
  heartbeatRestoreTaskTemplate,
  automationTemplateToHeartbeatTask,
  automationListRuns,
	automationListRunsV2,
  automationReadRun,
  automationExportRunResult,
  automationCancelRun,
	automationRetryDelivery,
	subscribeAutomationChanges,
  decodeAutomationModelRoute,
  encodeAutomationModelRoute,
} from "./heartbeat.bridge";
import type {
  AutomationTaskTemplate,
  AutomationTaskQuota,
  AutomationRunDetail,
  AutomationRunEvent,
  AutomationRunManifest,
  AutomationRunResultExportFormat,
  AutomationRunStatus,
  AutomationRunSummary,
  AutomationDeliveryTargetCatalogEntry,
  AutomationScheduleCatalogEntry,
  BusinessDataKind,
  BusinessDataSource,
  BusinessDiagnosticsConfig,
  BusinessSourceType,
  HeartbeatTask,
} from "./heartbeat.types";
import type { ExpertInfo, ModelInfo, SharedModelCatalogState, SkillView, UserExpertInfo, WorkspaceView } from "../../../lib/types";
import { AutomationWorkspace, type AutomationWorkspaceTab } from "../workspace/AutomationWorkspace";

const BUSINESS_SOURCE_DEFS: {
  kind: BusinessDataKind;
  label: string;
  required: boolean;
  fields: string;
}[] = [
  { kind: "sales", label: "销售", required: true, fields: "date, order_id, sku, quantity, amount, region" },
  { kind: "inventory", label: "库存", required: true, fields: "date, sku, on_hand_qty, reserved_qty, warehouse, region" },
  { kind: "purchase", label: "采购", required: true, fields: "date, purchase_id, sku, quantity, amount, supplier, region" },
  { kind: "finance", label: "财务", required: false, fields: "date, account, amount, category, region" },
  { kind: "returns", label: "退换货", required: false, fields: "date, return_id, order_id, sku, quantity, amount, reason, region" },
];

const BUSINESS_DIAGNOSTICS_PROMPT =
  "只使用 businessDiagnostics 中配置的真实经营数据源执行周度经营诊断：采集销售、采集库存、对比历史、识别风险并生成可审计方案。";
const BUSINESS_DIAGNOSTICS_EXPERT_ROLE = "数据分析专家团\n从指标定义、数据清洗、诊断分析到报告输出形成闭环。";

interface AutomationCloudProjectChoice {
  projectId: string;
  name: string;
}

interface AutomationResourceLoadState {
  loading: boolean;
  error: string;
}

async function listAutomationCloudProjects(): Promise<AutomationCloudProjectChoice[]> {
  if (typeof app.ListCloudProjects !== "function") return [];
  const load = async (joined: boolean) => {
    const items: AutomationCloudProjectChoice[] = [];
    let cursor = "";
    for (let pageNumber = 0; pageNumber < 20; pageNumber += 1) {
      const page = joined
        ? await app.ListJoinedCloudProjects({ pageSize: 100, cursor, status: "active" })
        : await app.ListCloudProjects({ pageSize: 100, cursor, status: "active" });
      for (const project of Array.isArray(page?.items) ? page.items : []) {
        if (project.projectId?.trim()) items.push({ projectId: project.projectId.trim(), name: project.name?.trim() || project.projectId.trim() });
      }
      cursor = page?.nextCursor?.trim() || "";
      if (!cursor) break;
    }
    return items;
  };
  const [owned, joined] = await Promise.all([
    load(false),
    typeof app.ListJoinedCloudProjects === "function" ? load(true) : Promise.resolve([]),
  ]);
  const byID = new Map<string, AutomationCloudProjectChoice>();
  for (const project of [...owned, ...joined]) {
    if (!byID.has(project.projectId)) byID.set(project.projectId, project);
  }
  return [...byID.values()];
}

function automationUserExpertInfo(userExpert: UserExpertInfo): ExpertInfo {
  return {
    id: userExpert.id,
    source: userExpert.source,
    mode: userExpert.mode,
    name: userExpert.name,
    role: userExpert.systemPrompt,
    description: userExpert.description,
    category: userExpert.category,
    categories: userExpert.category ? [userExpert.category] : [],
    tags: userExpert.tags,
    avatarKey: userExpert.avatarKey,
    members: userExpert.members,
    prompts: userExpert.prompts,
    language: userExpert.language,
    enabled: userExpert.enabled,
    createdAt: userExpert.createdAt,
    updatedAt: userExpert.updatedAt,
  };
}

function automationTaskWorkspaceKey(task: Pick<HeartbeatTask, "projectId" | "workspaceRoot">): string {
  return task.projectId?.trim() || task.workspaceRoot?.trim() || "";
}

function localDateInputValue(value: string | undefined): string {
  if (!value) return "";
  const d = new Date(value);
  if (!Number.isFinite(d.getTime())) return "";
  const month = (d.getMonth() + 1).toString().padStart(2, "0");
  const day = d.getDate().toString().padStart(2, "0");
  return `${d.getFullYear()}-${month}-${day}`;
}

function localDateToISO(value: string, endOfDay = false): string {
  const d = new Date(`${value}T00:00:00`);
  if (!Number.isFinite(d.getTime())) return "";
  if (endOfDay) d.setHours(23, 59, 59, 999);
  return d.toISOString();
}

function defaultBusinessTargetDate(daysAgo: number, now = Date.now()): string {
  const d = new Date(now);
  d.setDate(d.getDate() - daysAgo);
  d.setHours(daysAgo === 0 ? 23 : 0, daysAgo === 0 ? 59 : 0, daysAgo === 0 ? 59 : 0, daysAgo === 0 ? 999 : 0);
  return d.toISOString();
}

function defaultBusinessSource(kind: BusinessDataKind): BusinessDataSource {
  return {
    id: kind,
    kind,
    type: kind === "inventory" ? "xlsx" : "csv",
    path: "",
  };
}

function defaultBusinessDiagnosticsConfig(): BusinessDiagnosticsConfig {
  return {
    sources: BUSINESS_SOURCE_DEFS.filter((def) => def.required).map((def) => defaultBusinessSource(def.kind)),
    targetStart: defaultBusinessTargetDate(7),
    targetEnd: defaultBusinessTargetDate(0),
    region: "",
    recipients: [],
  };
}

function createBusinessDiagnosticsTask(id: string, workspaceRoot = ""): HeartbeatTask {
  return {
    id,
    title: "周度经营诊断",
    prompt: BUSINESS_DIAGNOSTICS_PROMPT,
    role: BUSINESS_DIAGNOSTICS_EXPERT_ROLE,
    interval: "168h|weekly:mon@03:30",
    enabled: true,
    scope: workspaceRoot ? "project" : "global",
    workspaceRoot,
    approvalMode: "auto",
    newConversationEachRun: false,
    businessDiagnostics: defaultBusinessDiagnosticsConfig(),
  };
}

const automationModelUnavailableMessage = "当前模型不可用，请选择其他模型或检查模型设置。";

function heartbeatErrorMessage(err: unknown): string {
  return reportClientError("automation.operation", err, {
    knownMessages: {
      AUTOMATION_MODEL_UNAVAILABLE: automationModelUnavailableMessage,
      NETWORK_UNAVAILABLE: "暂时无法连接云端服务。",
    },
    fallbackMessage: "自动化操作失败，请稍后重试。",
  }).userMessage;
}

function backendNextRunLabel(nextRunAt: string | undefined, now: number, t: ReturnType<typeof useT>): string {
	if (!nextRunAt) return "暂无下次运行";
	const next = Date.parse(nextRunAt);
	if (!Number.isFinite(next)) return "暂无下次运行";
	const diff = next - now;
	if (diff <= 0) return t("heartbeat.due" as any);
	if (diff < 60_000) return t("heartbeat.soon" as any);
	if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}${t("heartbeat.minLater" as any)}`;
	if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}${t("heartbeat.hourLater" as any)}`;
	const date = new Date(next);
	return `${date.getMonth() + 1}/${date.getDate()} ${date.getHours().toString().padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
}

function heartbeatIntervalLabel(interval: string | undefined, t: ReturnType<typeof useT>): string {
  if ((interval || "").startsWith("once@")) {
    const runAt = Date.parse((interval || "").slice("once@".length));
    if (Number.isFinite(runAt)) {
      const d = new Date(runAt);
      return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
    }
    return "once";
  }
  const cycleMatch = (interval || "").match(/^(\d+)[smh]\|(daily|weekly|biweekly|monthly|yearly)(?::([^@]*))?(?:@(\d{2}:\d{2}))?$/);
  if (cycleMatch) {
    const [, , type, days, time] = cycleMatch;
    const timeStr = time ? ` ${time}` : "";
    if (type === "daily") return `${t("heartbeat.cycleDaily")}${timeStr}`;
    if (type === "weekly") return `${t("heartbeat.cycleWeekly")}${timeStr}`;
    if (type === "biweekly") return `${t("heartbeat.cycleBiweekly")}${timeStr}`;
    if (type === "monthly") return `${t("heartbeat.cycleMonthly")}${days ? ` ${days}` : ""}${timeStr}`;
    if (type === "yearly") {
      const parts = (days || "").split("-");
      return `${t("heartbeat.cycleYearly")} ${parts[0] || "1"}/${parts[1] || "1"}${timeStr}`;
    }
  }
  const clean = (interval || "").replace(/\|.*$/, "");
  const m = clean.match(/^(\d+)([smh])$/);
  if (!m) return clean;
  const unitLabels: Record<string, string> = {
    s: t("heartbeat.unitSec"),
    m: t("heartbeat.unitMin"),
    h: t("heartbeat.unitHour"),
  };
  return `${t("heartbeat.freqEvery")}${t("heartbeat.everyJoiner")}${m[1]}${unitLabels[m[2]] || m[2]}`;
}

const RUN_STATUS_LABELS: Record<AutomationRunStatus, string> = {
  queued: "排队",
  checking_condition: "检查条件",
  skipped: "跳过",
  running: "运行中",
  succeeded: "成功",
  failed: "失败",
  blocked: "阻塞",
  canceled: "已取消",
};

const DELIVERY_STATUS_LABELS: Record<string, string> = {
  queued: "等待投递",
  sending: "投递中",
  succeeded: "投递成功",
  failed: "投递失败",
  canceled: "投递已取消",
};

const USAGE_LABELS: Record<string, string> = {
  PromptTokens: "输入 Token",
  promptTokens: "输入 Token",
  NormalInputTokens: "非缓存输入 Token",
  normalInputTokens: "非缓存输入 Token",
  CompletionTokens: "输出 Token",
  completionTokens: "输出 Token",
  TotalTokens: "总 Token",
  totalTokens: "总 Token",
  CacheHitTokens: "缓存命中 Token",
  cacheHitTokens: "缓存命中 Token",
  CacheMissTokens: "缓存未命中 Token",
  cacheMissTokens: "缓存未命中 Token",
  CacheReadTokens: "缓存读取 Token",
  cacheReadTokens: "缓存读取 Token",
  CacheWriteTokens: "缓存写入 Token",
  cacheWriteTokens: "缓存写入 Token",
  ReasoningTokens: "推理 Token",
  reasoningTokens: "推理 Token",
  FinishReason: "结束原因",
  finishReason: "结束原因",
};

const RUN_NUMBER_FORMAT = new Intl.NumberFormat("zh-CN");

function automationUsageRows(usage: Record<string, unknown> | undefined): Array<{ label: string; value: string }> {
  if (!usage) return [];
  return Object.entries(usage).flatMap(([key, raw]) => {
    if (typeof raw === "number" && Number.isFinite(raw)) {
      return [{ label: USAGE_LABELS[key] || key, value: RUN_NUMBER_FORMAT.format(raw) }];
    }
    if (typeof raw === "string" && raw.trim()) {
      return [{ label: USAGE_LABELS[key] || key, value: raw.trim() }];
    }
    return [];
  });
}

function automationDeliveryStatusLabel(status: string): string {
  return DELIVERY_STATUS_LABELS[status] || status || "未知状态";
}

export function automationRunStatusTone(status: string | undefined): "neutral" | "running" | "success" | "warning" | "danger" {
  if (status === "running" || status === "queued" || status === "checking_condition") return "running";
  if (status === "succeeded") return "success";
  if (status === "skipped" || status === "blocked") return "warning";
  if (status === "failed" || status === "canceled") return "danger";
  return "neutral";
}

function automationRunStatusLabel(status: string | undefined): string {
  if (!status) return "-";
  return RUN_STATUS_LABELS[status as AutomationRunStatus] || status;
}

const RUN_EVENT_KIND_LABELS: Record<string, string> = {
  "0": "开始",
  "1": "思考",
  "2": "输出",
  "3": "消息",
  "4": "工具开始",
  "5": "工具结果",
  "6": "用量",
  "7": "通知",
  "8": "阶段",
  "9": "审批",
  "10": "提问",
  "11": "结束",
  "12": "压缩开始",
  "13": "压缩完成",
  "14": "工具进度",
  "15": "MCP 就绪",
  "16": "重试",
  "17": "用户补充",
  "18": "已退役事件",
  "19": "安全评估",
  "20": "审计",
  "21": "任务树",
  "22": "任务更新",
  "23": "任务开始",
  "24": "任务输出",
  "25": "任务完成",
  "26": "任务失败",
  "27": "任务树完成",
  "28": "任务摘要",
  message: "消息",
  error: "错误",
  notice: "通知",
  phase: "阶段",
  reasoning: "思考",
  result: "结果",
  text: "输出",
};

const RUN_EVENT_LEVEL_LABELS: Record<string, string> = {
  "0": "信息",
  "1": "警告",
  "2": "错误",
  info: "信息",
  warn: "警告",
  warning: "警告",
  error: "错误",
};

export function automationRunEventLabel(event: Pick<AutomationRunEvent, "status" | "kind" | "level">): string {
  if (event.status) return automationRunStatusLabel(event.status);
  const kind = event.kind == null ? "" : String(event.kind);
  const level = event.level == null ? "" : String(event.level);
  return RUN_EVENT_KIND_LABELS[kind] || RUN_EVENT_LEVEL_LABELS[level] || kind || level || "事件";
}

export function automationRunEventText(event: Pick<AutomationRunEvent, "error" | "text">): string {
  return event.error || event.text || "无事件详情";
}

const COMPACT_RUN_EVENT_KINDS = new Set(["1", "2", "14", "24", "reasoning", "text", "tool_progress", "task_node_output"]);

export function normalizeAutomationRunEvents(events: AutomationRunEvent[] | undefined): AutomationRunEvent[] {
  if (!events?.length) return [];
  const out: AutomationRunEvent[] = [];
  let stream: AutomationRunEvent | null = null;
  const flush = () => {
    if (stream) out.push(stream);
    stream = null;
  };
  for (const event of events) {
    const kind = event.kind == null ? "" : String(event.kind);
    if (!event.status && !event.error && COMPACT_RUN_EVENT_KINDS.has(kind)) {
      if (stream !== null && stream.kind === event.kind && stream.level === event.level) {
        const current = stream as AutomationRunEvent;
        stream = Object.assign({}, current, { text: appendRunEventText(current.text || "", event.text || "") });
      } else {
        flush();
        stream = { ...event, text: event.text || "" };
      }
    } else {
      flush();
      out.push(event);
    }
  }
  flush();
  return out;
}

function appendRunEventText(current: string, next: string): string {
  if (!current) return next.trimStart();
  if (!next) return current;
  const trimmedNext = next.trimStart();
  if (!trimmedNext || next !== trimmedNext || /\s$/.test(current)) return current + next;
  const last = current[current.length - 1];
  const first = trimmedNext[0];
  if (/^[.,:;!?)]}，。！？；：、]/.test(trimmedNext)) return current + trimmedNext;
  if (/["']/.test(first)) return /[A-Za-z0-9]$/.test(current) ? `${current} ${trimmedNext}` : current + trimmedNext;
  if (/["'([{]$/.test(current)) return current + trimmedNext;
  if (isCjk(last) && isCjk(first)) return current + trimmedNext;
  return `${current} ${trimmedNext}`;
}

function isCjk(value: string): boolean {
  return /[\u3400-\u9fff]/.test(value);
}

function automationRunTimeLabel(value: string | undefined): string {
  if (!value) return "-";
  const ms = Date.parse(value);
  if (!Number.isFinite(ms)) return value;
  const d = new Date(ms);
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
}

function automationRunReason(run: Pick<AutomationRunSummary, "errorMessage" | "skipReason" | "conditionReason" | "triggerReason" | "summary">): string {
  return run.errorMessage || run.skipReason || run.conditionReason || run.triggerReason || run.summary || "";
}

function automationDurationLabel(ms: number | undefined): string {
  if (!ms || ms <= 0) return "-";
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${Math.round(ms / 1000)}s`;
  return `${Math.round(ms / 60000)}m`;
}

function automationScheduleSnapshotLabel(manifest: AutomationRunManifest): string {
  const schedule = manifest.schedule;
  if (!schedule?.kind) return "未记录";
  const parts: string[] = [schedule.kind];
  if (schedule.runAt) parts.push(automationRunTimeLabel(schedule.runAt));
  if (schedule.every) parts.push(schedule.every);
  if (schedule.calendar?.frequency) parts.push(schedule.calendar.frequency);
  if (schedule.calendar?.timeOfDay) parts.push(schedule.calendar.timeOfDay);
  if (schedule.weekdays?.length) parts.push(schedule.weekdays.join("、"));
  if (schedule.calendar?.weekdays?.length) parts.push(schedule.calendar.weekdays.join("、"));
  if (schedule.timezone) parts.push(schedule.timezone);
  if (schedule.window?.start || schedule.window?.end) {
    parts.push(`${schedule.window.start || "-"}–${schedule.window.end || "-"}`);
  }
  return parts.join(" · ");
}

function automationEffectiveRangeLabel(manifest: AutomationRunManifest): string {
  const from = manifest.effectiveRange?.from;
  const until = manifest.effectiveRange?.until;
  if (!from && !until) return "永久有效";
  return `${automationRunTimeLabel(from)} → ${automationRunTimeLabel(until)}`;
}

function automationRunUserMessage(manifest: AutomationRunManifest, hasResult: boolean): string {
  if (manifest.errorCode === "data_quality_blocked" || manifest.status === "blocked") {
    return manifest.errorMessage || "必要经营数据缺失或无效，本次自动化任务已停止。";
  }
  if (manifest.status === "failed") {
    if (manifest.errorCode === "snapshot_failed" && hasResult) return "结果已生成，但会话保存失败。";
    if (hasResult) return "结果已生成，但运行收尾失败。";
    return manifest.errorMessage || manifest.error || "自动化任务运行失败。";
  }
  if (manifest.status === "canceled") return "本次运行已取消。";
  if (manifest.status === "skipped") {
    return manifest.skipReason || manifest.conditionReason || "条件未满足，本次未执行。";
  }
  return "";
}

function automationRunTimeRange(manifest: AutomationRunManifest): string {
  const started = automationRunTimeLabel(manifest.startedAt);
  const finished = manifest.finishedAt ? automationRunTimeLabel(manifest.finishedAt) : "";
  return finished && finished !== "-" ? `${started} / ${finished}` : started;
}

interface HeartbeatPanelProps {
  open: boolean;
  onClose: () => void;
  startNew?: boolean;
  embedded?: boolean;
  authenticated: boolean;
  modelCatalog?: SharedModelCatalogState;
  modelAccountScopeKey?: string;
  modelSettingsRevision?: number;
  subscriptionAccess: SubscriptionAccess;
  onConfigureModel?: () => void;
  onOpenTopic: (scope: string, workspaceRoot: string, topicId: string, sessionPath?: string) => void;
}

interface TriggerRequest {
  generation: number;
  startedAt: number;
  settling?: boolean;
}

const TRIGGER_POLL_INTERVAL_MS = 500;
const RUN_HISTORY_PAGE_SIZE = 20;
const unavailableModelCatalog: SharedModelCatalogState = { state: "unavailable" };

export function HeartbeatPanel({ open, onClose, startNew, embedded = false, authenticated, modelCatalog = unavailableModelCatalog, modelAccountScopeKey = "", modelSettingsRevision = 0, subscriptionAccess, onConfigureModel, onOpenTopic }: HeartbeatPanelProps) {
  const t = useT();
  const [tasks, setTasks] = useState<HeartbeatTask[]>([]);
  const [taskQuota, setTaskQuota] = useState<AutomationTaskQuota>({
    state: "loading",
    used: 0,
    limit: 0,
    unlimited: false,
    canCreate: false,
    pausedTaskIds: [],
  });
  const [templates, setTemplates] = useState<AutomationTaskTemplate[]>([]);
	const [templateLoadState, setTemplateLoadState] = useState<AutomationResourceLoadState>({ loading: false, error: "" });
	const [localWorkspaceMap, setLocalWorkspaceMap] = useState<Record<string, string>>({});
	const [cloudProjects, setCloudProjects] = useState<AutomationCloudProjectChoice[]>([]);
	const [projectLoadState, setProjectLoadState] = useState<AutomationResourceLoadState>({ loading: false, error: "" });
	const [scheduleCatalog, setScheduleCatalog] = useState<AutomationScheduleCatalogEntry[]>([]);
	const [deliveryTargetCatalog, setDeliveryTargetCatalog] = useState<AutomationDeliveryTargetCatalogEntry[]>([]);
  const [runsByTask, setRunsByTask] = useState<Record<string, AutomationRunSummary[]>>({});
	const [workspaceRuns, setWorkspaceRuns] = useState<AutomationRunSummary[]>([]);
	const [runHistoryCursor, setRunHistoryCursor] = useState("");
	const [runHistoryHasMore, setRunHistoryHasMore] = useState(false);
	const [runHistoryLoading, setRunHistoryLoading] = useState(false);
	const [runHistoryLoaded, setRunHistoryLoaded] = useState(false);
	const [workspaceTab, setWorkspaceTab] = useState<AutomationWorkspaceTab>("scheduled");
  const [triggeringByTask, setTriggeringByTask] = useState<Record<string, TriggerRequest>>({});
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<HeartbeatTask | null>(null);
  const [creating, setCreating] = useState(false);
  const [usingTemplates, setUsingTemplates] = useState(false);
  const [viewingRun, setViewingRun] = useState<{ taskID: string; runID: string } | null>(null);
  const [runDetail, setRunDetail] = useState<AutomationRunDetail | null>(null);
  const [runDetailLoading, setRunDetailLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "enabled" | "disabled">("all");
  const [scopeFilter, setScopeFilter] = useState<string>("all");
  const [scopeFilterOpen, setScopeFilterOpen] = useState(false);
  const scopeFilterRef = useRef<HTMLButtonElement>(null);
  const [statusFilterOpen, setStatusFilterOpen] = useState(false);
  const statusFilterRef = useRef<HTMLButtonElement>(null);
  const [saveError, setSaveError] = useState("");
  const [editorSaveError, setEditorSaveError] = useState("");
  const clearModelError = useCallback(() => setEditorSaveError((error) => error === automationModelUnavailableMessage ? "" : error), []);
  const [savePhase, setSavePhase] = useState<"" | "task" | "template">("");
  const saveRequestRef = useRef<object | null>(null);
  const editorGenerationRef = useRef(0);
  const taskLoadGenerationRef = useRef(0);
  const [capabilityRiskSummary, setCapabilityRiskSummary] = useState("");
  const backdropRef = useRef<HTMLDivElement>(null);
  const startedRef = useRef(false);
  const mountedRef = useRef(false);
  const openRef = useRef(open);
  const panelGenerationRef = useRef(0);
  const triggerGenerationRef = useRef(0);
  const triggeringRef = useRef<Record<string, TriggerRequest>>({});
  const runRequestGenerationRef = useRef<Record<string, number>>({});
	const runHistoryRequestGenerationRef = useRef(0);
	const projectRequestGenerationRef = useRef(0);
	const projectRequestsInFlightRef = useRef(0);
	const projectTreeRevisionFloorRef = useRef(0);
	const templateRequestGenerationRef = useRef(0);
  const triggerPollTimersRef = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  openRef.current = open;
  const resetEditorFeedback = useCallback(() => {
    editorGenerationRef.current += 1;
    saveRequestRef.current = null;
    setSavePhase("");
    setEditorSaveError("");
    setSaveError("");
    setCapabilityRiskSummary("");
  }, []);
	const workspaceMap = useMemo(() => {
		const next = { ...localWorkspaceMap };
		for (const project of cloudProjects) next[project.projectId] = project.name;
		return next;
	}, [cloudProjects, localWorkspaceMap]);

  const clearTriggerPoll = useCallback((taskID: string) => {
    const timer = triggerPollTimersRef.current[taskID];
    if (timer !== undefined) {
      clearTimeout(timer);
      delete triggerPollTimersRef.current[taskID];
    }
  }, []);

  const clearAllTriggerPolls = useCallback(() => {
    Object.values(triggerPollTimersRef.current).forEach((timer) => clearTimeout(timer));
    triggerPollTimersRef.current = {};
  }, []);

  const panelRequestIsCurrent = useCallback((panelGeneration: number) => (
    mountedRef.current &&
    openRef.current &&
    panelGenerationRef.current === panelGeneration
  ), []);

  const loadRunsForTask = useCallback(async (taskID: string, panelGeneration: number) => {
    const requestGeneration = (runRequestGenerationRef.current[taskID] || 0) + 1;
    runRequestGenerationRef.current[taskID] = requestGeneration;
    try {
      const runs = await automationListRuns(taskID, 3);
      if (
        !panelRequestIsCurrent(panelGeneration) ||
        runRequestGenerationRef.current[taskID] !== requestGeneration
      ) {
        return null;
      }
      setRunsByTask((previous) => ({ ...previous, [taskID]: runs }));
      return runs;
    } catch {
      return null;
    }
  }, [panelRequestIsCurrent]);

  const loadRuns = useCallback(async (taskList: HeartbeatTask[], panelGeneration = panelGenerationRef.current) => {
    if (taskList.length === 0) {
      if (panelRequestIsCurrent(panelGeneration)) setRunsByTask(() => ({}));
      return;
    }
    await Promise.all(taskList.map((task) => loadRunsForTask(task.id, panelGeneration)));
  }, [loadRunsForTask, panelRequestIsCurrent]);

	const loadWorkspaceRuns = useCallback(async (cursor = "", panelGeneration = panelGenerationRef.current) => {
		const requestGeneration = runHistoryRequestGenerationRef.current + 1;
		runHistoryRequestGenerationRef.current = requestGeneration;
		setRunHistoryLoading(true);
		try {
			const page = await automationListRunsV2({ limit: RUN_HISTORY_PAGE_SIZE }, cursor);
			if (!panelRequestIsCurrent(panelGeneration) || runHistoryRequestGenerationRef.current !== requestGeneration) return;
			setWorkspaceRuns((previous) => cursor ? [...previous, ...page.items] : page.items);
			setRunHistoryCursor(page.nextCursor || "");
			setRunHistoryHasMore(page.hasMore && !!page.nextCursor);
			setRunHistoryLoaded(true);
		} catch (error) {
			if (panelRequestIsCurrent(panelGeneration) && runHistoryRequestGenerationRef.current === requestGeneration) {
				setSaveError(`运行记录加载失败：${heartbeatErrorMessage(error)}`);
			}
		} finally {
			if (panelRequestIsCurrent(panelGeneration) && runHistoryRequestGenerationRef.current === requestGeneration) {
				setRunHistoryLoading(false);
			}
		}
	}, [panelRequestIsCurrent]);

  const loadTasks = useCallback(async (panelGeneration: number) => {
    const requestGeneration = ++taskLoadGenerationRef.current;
    setLoading(true);
    try {
      const taskList = await heartbeatListTasks();
      if (!panelRequestIsCurrent(panelGeneration) || taskLoadGenerationRef.current !== requestGeneration) return;
      setTasks(taskList);
      setSaveError("");
      const taskIDs = new Set(taskList.map((task) => task.id));
      setRunsByTask((previous) => Object.fromEntries(
        Object.entries(previous).filter(([taskID]) => taskIDs.has(taskID)),
      ));
      void loadRuns(taskList, panelGeneration);
    } catch (err) {
      if (panelRequestIsCurrent(panelGeneration) && taskLoadGenerationRef.current === requestGeneration) {
        setSaveError(`自动化任务加载失败：${heartbeatErrorMessage(err)}`);
      }
    } finally {
      if (panelRequestIsCurrent(panelGeneration) && taskLoadGenerationRef.current === requestGeneration) setLoading(false);
    }
  }, [loadRuns, panelRequestIsCurrent]);

	const loadWorkspaces = useCallback(async (panelGeneration: number) => {
		try {
			const workspaces = await app.ListWorkspaces();
			if (!panelRequestIsCurrent(panelGeneration)) return;
			const next: Record<string, string> = {};
			for (const workspace of workspaces || []) {
				if (workspace.path) next[workspace.path] = workspace.name;
			}
			setLocalWorkspaceMap(next);
		} catch (error) {
			if (panelRequestIsCurrent(panelGeneration)) {
				setSaveError(`工作空间加载失败：${heartbeatErrorMessage(error)}`);
			}
		}
	}, [panelRequestIsCurrent]);

	const loadCloudProjects = useCallback(async (
		panelGeneration = panelGenerationRef.current,
		allowRevisionRetry = true,
	): Promise<void> => {
		if (!authenticated) {
			projectRequestGenerationRef.current += 1;
			setCloudProjects([]);
			setProjectLoadState({ loading: false, error: "" });
			return;
		}
		const requestGeneration = projectRequestGenerationRef.current + 1;
		projectRequestGenerationRef.current = requestGeneration;
		projectRequestsInFlightRef.current += 1;
		setProjectLoadState((current) => ({ ...current, loading: true, error: "" }));
		let revisionBefore = projectTreeRevisionFloorRef.current;
		try {
			const revision = await app.ProjectTreeRevision();
			if (Number.isFinite(revision)) revisionBefore = Math.max(revisionBefore, revision);
		} catch {
			// Older runtimes may not expose revisions; the in-flight guard still covers synchronous delivery.
		}
		let projects: AutomationCloudProjectChoice[] | undefined;
		let loadError: unknown;
		try {
			projects = await listAutomationCloudProjects();
		} catch (error) {
			loadError = error;
		} finally {
			let revisionAfter = revisionBefore;
			try {
				const revision = await app.ProjectTreeRevision();
				if (Number.isFinite(revision)) revisionAfter = Math.max(revisionAfter, revision);
			} catch {
				// Keep the last observed revision when the binding is unavailable.
			}
			projectTreeRevisionFloorRef.current = Math.max(projectTreeRevisionFloorRef.current, revisionAfter);
			projectRequestsInFlightRef.current = Math.max(0, projectRequestsInFlightRef.current - 1);
			if (!panelRequestIsCurrent(panelGeneration) || projectRequestGenerationRef.current !== requestGeneration) return;
			if (projects) {
				setCloudProjects(projects);
				setProjectLoadState({ loading: false, error: "" });
				return;
			}
			if (allowRevisionRetry && revisionAfter > revisionBefore) {
				void loadCloudProjects(panelGeneration, false);
				return;
			}
			setProjectLoadState({ loading: false, error: heartbeatErrorMessage(loadError) });
		}
	}, [authenticated, panelRequestIsCurrent]);

	const loadTemplates = useCallback(async (panelGeneration = panelGenerationRef.current) => {
		const requestGeneration = templateRequestGenerationRef.current + 1;
		templateRequestGenerationRef.current = requestGeneration;
		setTemplateLoadState((current) => ({ ...current, loading: true, error: "" }));
		try {
			const next = await heartbeatListTemplates();
			if (!panelRequestIsCurrent(panelGeneration) || templateRequestGenerationRef.current !== requestGeneration) return;
			setTemplates(next);
			setTemplateLoadState({ loading: false, error: "" });
		} catch (error) {
			if (panelRequestIsCurrent(panelGeneration) && templateRequestGenerationRef.current === requestGeneration) {
				setTemplateLoadState({ loading: false, error: heartbeatErrorMessage(error) });
			}
		}
	}, [panelRequestIsCurrent]);

	const loadTaskQuota = useCallback(async (panelGeneration: number) => {
		const quota = await heartbeatTaskQuota().catch(() => null);
		if (quota && panelRequestIsCurrent(panelGeneration)) setTaskQuota(quota);
	}, [panelRequestIsCurrent]);

	const loadCatalogs = useCallback(async (panelGeneration: number) => {
		const [catalog, deliveryTargets] = await Promise.all([
			automationGetScheduleCatalog().catch(() => null),
			automationListDeliveryTargets().catch(() => null),
		]);
		if (!panelRequestIsCurrent(panelGeneration)) return;
		if (catalog) setScheduleCatalog(catalog);
		if (deliveryTargets) setDeliveryTargetCatalog(deliveryTargets);
	}, [panelRequestIsCurrent]);

  const openRunDetail = useCallback(async (taskID: string, runID: string) => {
    setSaveError("");
    setCapabilityRiskSummary("");
    setViewingRun({ taskID, runID });
    setRunDetail(null);
    setRunDetailLoading(true);
    try {
      setRunDetail(await automationReadRun(taskID, runID));
    } catch {
      setRunDetail(null);
    } finally {
      setRunDetailLoading(false);
    }
  }, []);

  const refreshRunDetail = useCallback(async () => {
    if (!viewingRun) return;
    setRunDetailLoading(true);
    try {
      setRunDetail(await automationReadRun(viewingRun.taskID, viewingRun.runID));
    } catch {
      setRunDetail(null);
    } finally {
      setRunDetailLoading(false);
    }
  }, [viewingRun]);

  const handleCancelRun = useCallback(async (taskID: string, runID: string) => {
    try {
      await automationCancelRun(taskID, runID);
      await loadRuns(tasks);
      await refreshRunDetail();
      setSaveError("");
    } catch (err) {
      setSaveError(`自动化任务取消失败：${heartbeatErrorMessage(err)}`);
    }
  }, [loadRuns, refreshRunDetail, tasks]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      panelGenerationRef.current += 1;
      triggeringRef.current = {};
      clearAllTriggerPolls();
    };
  }, [clearAllTriggerPolls]);

  useEffect(() => {
    panelGenerationRef.current += 1;
    resetEditorFeedback();
    const panelGeneration = panelGenerationRef.current;
    triggeringRef.current = {};
    runRequestGenerationRef.current = {};
		runHistoryRequestGenerationRef.current += 1;
    clearAllTriggerPolls();
    setTriggeringByTask({});
		setWorkspaceRuns([]);
		setRunHistoryCursor("");
		setRunHistoryHasMore(false);
		setRunHistoryLoaded(false);
		setRunHistoryLoading(false);
    if (!open) return;
    setEditing(null);
    setViewingRun(null);
    setRunDetail(null);
    setUsingTemplates(false);
    setSearchQuery("");
    setStatusFilter("all");
    setScopeFilter("all");
    startedRef.current = false;
    void loadTasks(panelGeneration);
		void loadWorkspaces(panelGeneration);
		void loadCloudProjects(panelGeneration);
		void loadTemplates(panelGeneration);
		void loadTaskQuota(panelGeneration);
		void loadCatalogs(panelGeneration);
    return () => {
      panelGenerationRef.current += 1;
      triggeringRef.current = {};
      runRequestGenerationRef.current = {};
      clearAllTriggerPolls();
    };
  }, [clearAllTriggerPolls, loadCatalogs, loadCloudProjects, loadTaskQuota, loadTasks, loadTemplates, loadWorkspaces, open, resetEditorFeedback]);

	useEffect(() => {
		if (!open || !authenticated) return;
		return onProjectTreeChanged((revision) => {
			if (typeof revision === "number" && revision <= projectTreeRevisionFloorRef.current) return;
			if (projectRequestsInFlightRef.current > 0) return;
			void loadCloudProjects(panelGenerationRef.current);
		});
	}, [authenticated, loadCloudProjects, open]);

	useEffect(() => {
		if (!open || !embedded || workspaceTab !== "runs" || runHistoryLoaded) return;
		void loadWorkspaceRuns();
	}, [embedded, loadWorkspaceRuns, open, runHistoryLoaded, workspaceTab]);

	useEffect(() => {
		if (!open) return;
		let active = true;
		let debounceTimer: number | undefined;
		const invalidations = new Set<"task" | "run" | "run_event" | "delivery" | "catalog">();
		const flushInvalidations = () => {
			debounceTimer = undefined;
			const pending = new Set(invalidations);
			invalidations.clear();
			const panelGeneration = panelGenerationRef.current;
			void (async () => {
				if (pending.has("task") || pending.has("run")) {
					await loadTasks(panelGeneration);
				} else if (pending.has("catalog")) {
					const [catalog, deliveryTargets] = await Promise.all([
						automationGetScheduleCatalog().catch(() => null),
						automationListDeliveryTargets().catch(() => null),
					]);
					if (active && panelRequestIsCurrent(panelGeneration)) {
						if (catalog) setScheduleCatalog(catalog);
						if (deliveryTargets) setDeliveryTargetCatalog(deliveryTargets);
					}
				}
				if (pending.has("run") && embedded && runHistoryLoaded) {
					await loadWorkspaceRuns("", panelGeneration);
				}
				if ((pending.has("run") || pending.has("run_event") || pending.has("delivery")) && viewingRun) {
					const selectedRun = viewingRun;
					const detail = await automationReadRun(selectedRun.taskID, selectedRun.runID).catch(() => null);
					if (active && detail && panelRequestIsCurrent(panelGeneration)) setRunDetail(detail);
				}
			})();
		};
		const unsubscribe = subscribeAutomationChanges((kind) => {
			invalidations.add(kind);
			if (debounceTimer !== undefined) return;
			debounceTimer = window.setTimeout(flushInvalidations, 50);
		});
		return () => {
			active = false;
			unsubscribe();
			if (debounceTimer !== undefined) window.clearTimeout(debounceTimer);
		};
	}, [embedded, loadTasks, loadWorkspaceRuns, open, panelRequestIsCurrent, runHistoryLoaded, viewingRun]);

  const quotaCanCreate = taskQuota.state === "ready" && taskQuota.canCreate;
  const quotaLimitReached = taskQuota.state === "ready"
    && !taskQuota.unlimited
    && taskQuota.used >= taskQuota.limit;
  const quotaPausedTaskIDs = useMemo(() => new Set(taskQuota.pausedTaskIds || []), [taskQuota.pausedTaskIds]);
  const quotaLabel = taskQuota.state === "ready"
    ? `${taskQuota.used}/${taskQuota.unlimited ? "无限" : taskQuota.limit}`
    : `${tasks.length}/--`;
  const quotaCreateMessage = taskQuota.state === "ready"
    ? taskQuota.unlimited
      ? "当前套餐自动化任务数量无限"
      : quotaLimitReached
        ? `已达到当前套餐自动化任务上限（${taskQuota.used}/${taskQuota.limit}）。请删除不需要的任务后再创建。`
        : `当前套餐最多可创建 ${taskQuota.limit} 个自动化任务`
    : "正在确认当前套餐额度，请稍后再试";

  // Open directly in add mode when startNew is true.
  useEffect(() => {
    if (!open || !startNew || startedRef.current || taskQuota.state !== "ready") return;
    startedRef.current = true;
    if (!quotaCanCreate) {
      setSaveError(quotaCreateMessage);
      return;
    }
    void heartbeatGenerateID().then((id) => {
      setCreating(true);
      setEditing({
        id,
        title: "",
        prompt: "",
        interval: "30m",
        enabled: true,
        approvalMode: "auto",
        newConversationEachRun: false,
      });
    });
  }, [open, quotaCanCreate, quotaCreateMessage, startNew, taskQuota.state]);

  const save = useCallback(
    async (next: HeartbeatTask[]) => {
      try {
        await heartbeatSaveTasks(next);
        setTasks(next);
        const quota = await heartbeatTaskQuota().catch(() => null);
        if (quota) {
          setTaskQuota(quota);
        } else {
          setTaskQuota((current) => current.state === "ready"
            ? { ...current, used: next.length, canCreate: current.unlimited || next.length < current.limit }
            : current);
        }
        setSaveError("");
        return true;
      } catch (err) {
        setSaveError(`自动化任务保存失败：${heartbeatErrorMessage(err)}`);
        return false;
      }
    },
    [],
  );

  const handleAdd = useCallback(async () => {
    if (!quotaCanCreate) {
      setSaveError(quotaCreateMessage);
      return;
    }
    resetEditorFeedback();
    const editorGeneration = editorGenerationRef.current;
    const id = await heartbeatGenerateID();
    if (editorGenerationRef.current !== editorGeneration) return;
    setCreating(true);
    setEditing({
      id,
      title: "",
      prompt: "",
      interval: "30m",
      enabled: true,
      approvalMode: "auto",
      newConversationEachRun: false,
    });
  }, [quotaCanCreate, quotaCreateMessage, resetEditorFeedback]);

  const createBusinessDraft = useCallback(async (id: string) => {
    let workspaceRoot = "";
    try {
      const workspaces = await app.ListWorkspaces();
      workspaceRoot = workspaces?.find((ws) => ws.current)?.path || "";
    } catch {
      // fall back to a global task; the editor still lets the user pick a workspace
    }
    return createBusinessDiagnosticsTask(id, workspaceRoot);
  }, []);

  const handleUseTemplate = useCallback(async (template: AutomationTaskTemplate) => {
    if (!quotaCanCreate) {
      setSaveError(quotaCreateMessage);
      return;
    }
    resetEditorFeedback();
    const editorGeneration = editorGenerationRef.current;
    const id = await heartbeatGenerateID();
    const task = template.kind === "business_diagnostics"
      ? await createBusinessDraft(id)
      : automationTemplateToHeartbeatTask(template, id);
    if (editorGenerationRef.current !== editorGeneration) return;
    setCreating(true);
    setEditing(task);
    setUsingTemplates(false);
  }, [createBusinessDraft, quotaCanCreate, quotaCreateMessage, resetEditorFeedback]);

  const handleDeleteTemplate = useCallback(async (template: AutomationTaskTemplate) => {
    await heartbeatDeleteTaskTemplate(template.id);
    await loadTemplates();
  }, [loadTemplates]);

  const handleRestoreTemplates = useCallback(async () => {
    await heartbeatRestoreTaskTemplate();
    await loadTemplates();
  }, [loadTemplates]);

  const handleEdit = useCallback((task: HeartbeatTask) => {
    resetEditorFeedback();
    setCreating(false);
    setEditing({ ...task });
    setCapabilityRiskSummary("");
  }, [resetEditorFeedback]);

  const handleDelete = useCallback(
    async (id: string) => {
			const task = tasks.find((item) => item.id === id);
			if (!task) return false;
			if (!task.revision) return save(tasks.filter((item) => item.id !== id));
			let saved = false;
			try {
				await heartbeatDeleteTask(task);
				setTasks((current) => current.filter((item) => item.id !== id));
				setSaveError("");
				saved = true;
			} catch (err) {
				setSaveError(`自动化任务删除失败：${heartbeatErrorMessage(err)}`);
			}
      if (saved) {
        clearTriggerPoll(id);
        delete triggeringRef.current[id];
        runRequestGenerationRef.current[id] = (runRequestGenerationRef.current[id] || 0) + 1;
        setTriggeringByTask((previous) => {
          if (!(id in previous)) return previous;
          const nextTriggering = { ...previous };
          delete nextTriggering[id];
          return nextTriggering;
        });
        setRunsByTask((previous) => {
          if (!(id in previous)) return previous;
          const nextRuns = { ...previous };
          delete nextRuns[id];
          return nextRuns;
        });
      }
      return saved;
    },
		[clearTriggerPoll, save, tasks],
  );

	const handleToggle = useCallback(async (task: HeartbeatTask) => {
		try {
			const updated = await heartbeatSetTaskEnabled(task, !task.enabled);
			const refreshedTasks = await heartbeatListTasks().catch(() => null);
			setTasks((current) => refreshedTasks || current.map((item) => item.id === updated.id ? updated : item));
			setSaveError("");
		} catch (err) {
			setSaveError(`自动化任务状态更新失败：${heartbeatErrorMessage(err)}`);
		}
	}, []);

  const startTriggerPolling = useCallback((taskID: string, request: TriggerRequest, panelGeneration: number) => {
    const poll = async () => {
      await loadRunsForTask(taskID, panelGeneration);
      const current = triggeringRef.current[taskID];
      if (
        !panelRequestIsCurrent(panelGeneration) ||
        current?.generation !== request.generation ||
        current.settling
      ) {
        return;
      }
      triggerPollTimersRef.current[taskID] = setTimeout(() => {
        delete triggerPollTimersRef.current[taskID];
        void poll();
      }, TRIGGER_POLL_INTERVAL_MS);
    };
    void poll();
  }, [loadRunsForTask, panelRequestIsCurrent]);

  const handleTrigger = useCallback(
    async (id: string) => {
      if (triggeringRef.current[id]) return;
      const request: TriggerRequest = {
        generation: triggerGenerationRef.current + 1,
        startedAt: Date.now(),
      };
      triggerGenerationRef.current = request.generation;
      triggeringRef.current[id] = request;
      setTriggeringByTask((previous) => ({ ...previous, [id]: request }));
      const panelGeneration = panelGenerationRef.current;
      let triggerPromise: Promise<void>;
      try {
        triggerPromise = heartbeatTriggerNow(id);
      } catch (err) {
        triggerPromise = Promise.reject(err);
      }
      startTriggerPolling(id, request, panelGeneration);
      try {
        await triggerPromise;
        if (triggeringRef.current[id]?.generation === request.generation) {
          setSaveError("");
        }
      } catch (err) {
        if (triggeringRef.current[id]?.generation === request.generation) {
          setSaveError(`自动化任务立即运行失败：${heartbeatErrorMessage(err)}`);
        }
      } finally {
        if (
          !panelRequestIsCurrent(panelGeneration) ||
          triggeringRef.current[id]?.generation !== request.generation
        ) {
          return;
        }
        triggeringRef.current[id] = { ...request, settling: true };
        clearTriggerPoll(id);
        await loadRunsForTask(id, panelGeneration);
		const refreshedTasks = await heartbeatListTasks().catch(() => null);
		if (refreshedTasks && panelRequestIsCurrent(panelGeneration)) setTasks(refreshedTasks);
        if (
          panelRequestIsCurrent(panelGeneration) &&
          triggeringRef.current[id]?.generation === request.generation
        ) {
          clearTriggerPoll(id);
          delete triggeringRef.current[id];
          setTriggeringByTask((previous) => {
            if (previous[id]?.generation !== request.generation) return previous;
            const next = { ...previous };
            delete next[id];
            return next;
          });
        }
      }
    },
    [clearTriggerPoll, loadRunsForTask, panelRequestIsCurrent, startTriggerPolling],
  );

  const handleSaveEdit = useCallback(
    async (task: HeartbeatTask, options?: { saveAsTemplate?: boolean }) => {
      if (saveRequestRef.current) return;
      const request = {};
      saveRequestRef.current = request;
      const panelGeneration = panelGenerationRef.current;
      const isCurrent = () => saveRequestRef.current === request && panelRequestIsCurrent(panelGeneration);
      setSavePhase("task");
      setEditorSaveError("");
      setCapabilityRiskSummary("");
      try {
        try {
          const capabilities = await automationResolveCapabilities(task);
          if (!isCurrent()) return;
          setCapabilityRiskSummary(capabilities.riskSummary?.trim() || "");
        } catch (err) {
          const code = clientErrorCode(err);
          if (isCurrent()) setEditorSaveError(code === "AUTOMATION_MODEL_UNAVAILABLE"
            ? heartbeatErrorMessage(err)
            : `本次更改尚未保存：${heartbeatErrorMessage(err)}${code === "NETWORK_UNAVAILABLE" ? "请检查网络连接后重试。" : ""}`);
          return;
        }
        let savedTask: HeartbeatTask;
        try {
          savedTask = task.revision
            ? await heartbeatUpdateTask(task)
            : await heartbeatCreateTask(task);
        } catch (err) {
          if (isCurrent()) {
            setEditorSaveError(clientErrorCode(err) === "AUTOMATION_MODEL_UNAVAILABLE"
              ? heartbeatErrorMessage(err)
              : `未能确认任务保存结果：${heartbeatErrorMessage(err)} 请返回任务列表核对，避免重复添加。`);
            void loadTasks(panelGeneration);
            void loadTaskQuota(panelGeneration);
          }
          return;
        }
        // Acknowledged persistence must not wait for list or template refreshes.
        if (panelRequestIsCurrent(panelGeneration)) {
          setTasks((current) => {
            const index = current.findIndex((item) => item.id === savedTask.id);
            if (index < 0) return [...current, savedTask];
            if ((current[index].revision || 0) > (savedTask.revision || 0)) return current;
            const next = [...current];
            next[index] = savedTask;
            return next;
          });
          void loadTasks(panelGeneration);
          void loadTaskQuota(panelGeneration);
        }
        if (isCurrent()) {
          setEditing(savedTask);
          setCreating(false);
        }
        if (options?.saveAsTemplate) {
          if (isCurrent()) setSavePhase("template");
          try {
            await heartbeatSaveTaskTemplate(savedTask);
            if (panelRequestIsCurrent(panelGeneration)) void loadTemplates(panelGeneration);
          } catch (err) {
            if (isCurrent()) setEditorSaveError(`任务已保存，但模板保存失败：${heartbeatErrorMessage(err)}`);
            return savedTask;
          }
        }
        if (isCurrent()) {
          setCapabilityRiskSummary("");
          setEditing(null);
          setCreating(false);
        }
        return savedTask;
      } finally {
        if (saveRequestRef.current === request) {
          saveRequestRef.current = null;
          if (panelRequestIsCurrent(panelGeneration)) setSavePhase("");
        }
      }
    },
    [loadTaskQuota, loadTasks, loadTemplates, panelRequestIsCurrent],
  );

  const returnToList = useCallback(() => {
    resetEditorFeedback();
    setEditing(null);
    setCreating(false);
    setViewingRun(null);
    setRunDetail(null);
    setUsingTemplates(false);
  }, [resetEditorFeedback]);

  const backOrClose = useCallback(() => {
    if (editing || viewingRun || usingTemplates) {
      returnToList();
      return;
    }
    onClose();
  }, [editing, onClose, returnToList, usingTemplates, viewingRun]);

  const handleBackdrop = useCallback(
    (e: React.MouseEvent) => {
      if (!embedded && e.target === backdropRef.current) backOrClose();
    },
    [backOrClose, embedded],
  );

  useEffect(() => {
    if (!open) return;
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape") backOrClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [backOrClose, open]);

  if (!open) return null;

  const scopeFilterLabel = (filter: string, map: Record<string, string>): string => {
    if (filter === "all") return "全部项目";
    if (filter === "global") return "全局";
    return map[filter] || filter.split("/").pop() || filter;
  };

  const statusFilterLabel = (filter: string): string => {
    if (filter === "all") return t("heartbeat.filterAll" as any);
    if (filter === "enabled") return t("heartbeat.filterEnabled" as any);
    return t("heartbeat.filterDisabled" as any);
  };
  const isTaskListView = !editing && !viewingRun && !usingTemplates;
  const headerTitle = editing?.businessDiagnostics ? "经营诊断" : editing ? creating ? t("heartbeat.addTask") : t("heartbeat.editTask") : viewingRun ? "运行详情" : usingTemplates ? "模板使用" : t("heartbeat.title");
  const workspaceOptions = Object.entries(workspaceMap)
    .map(([value, label]) => ({ value, label: label || value.split("/").pop() || value }))
    .sort((a, b) => a.label.localeCompare(b.label, "zh-CN"));
  const backdropInteraction = embedded ? {} : { onClick: handleBackdrop };
	const workspaceScopeLabel = (task: HeartbeatTask): string => {
		const key = automationTaskWorkspaceKey(task);
		if (task.scope !== "project" || !key) return t("heartbeat.scopeGlobal");
		return workspaceMap[key] || key.split(/[\\/]/).pop() || key;
	};

  return (
    <div ref={backdropRef} className={`heartbeat-backdrop${embedded ? " heartbeat-backdrop--embedded" : ""}`} {...backdropInteraction}>
      <div className={`heartbeat-modal${embedded ? " heartbeat-modal--embedded" : ""}`}>
        {(!embedded || !isTaskListView) && <header className={`heartbeat-modal__header${embedded && isTaskListView ? " heartbeat-modal__header--workspace" : ""}`}>
          {embedded && isTaskListView ? (
            <div className="heartbeat-page-heading">
              <div className="heartbeat-page-heading__copy">
                <h1>{t("heartbeat.pageTitle" as any)}</h1>
                <p>{t("heartbeat.pageHint" as any)}</p>
              </div>
              <button
                className="heartbeat-toolbar__btn heartbeat-toolbar__btn--primary heartbeat-page-heading__add"
                onClick={handleAdd}
                disabled={!quotaCanCreate}
                title={!quotaCanCreate ? quotaCreateMessage : undefined}
              >
                <Plus size={15} aria-hidden="true" />
                {t("heartbeat.addTask")}
              </button>
            </div>
          ) : (
            <>
              {editing || viewingRun || usingTemplates ? (
                <button
                  className="heartbeat-modal__back"
                  onClick={returnToList}
                  aria-label="返回自动化任务列表"
                  title="返回自动化任务列表"
                >
                  <ChevronLeft size={16} aria-hidden="true" />
                </button>
              ) : (
                <Activity size={16} aria-hidden="true" />
              )}
              {embedded ? <h1>{headerTitle}</h1> : <span>{headerTitle}</span>}
              {!embedded && (
                <button
                  className="heartbeat-modal__close"
                  onClick={onClose}
                  aria-label={t("common.close")}
                >
                  <X size={16} aria-hidden="true" />
                </button>
              )}
            </>
          )}
        </header>}

        {!editing && !viewingRun && !usingTemplates && quotaLimitReached ? (
          <div
            className="heartbeat-run-detail__note heartbeat-run-detail__note--error"
            data-automation-quota-notice
            role="alert"
          >
            {quotaCreateMessage}
          </div>
        ) : null}

		{!viewingRun && projectLoadState.error ? (
			<div className="heartbeat-run-detail__note heartbeat-run-detail__note--error" data-automation-project-error role="alert">
				<span>云项目加载失败：{projectLoadState.error}</span>
				<button className="heartbeat-toolbar__btn" type="button" disabled={projectLoadState.loading} onClick={() => void loadCloudProjects()}>
					<RefreshCw size={13} aria-hidden="true" />
					重试
				</button>
			</div>
		) : !viewingRun && projectLoadState.loading && cloudProjects.length === 0 ? (
			<div className="heartbeat-run-detail__note" data-automation-project-loading role="status">正在加载云项目…</div>
		) : null}

		{!editing && !viewingRun && templateLoadState.error ? (
			<div className="heartbeat-run-detail__note heartbeat-run-detail__note--error" data-automation-template-error role="alert">
				<span>自动化模板加载失败：{templateLoadState.error}</span>
				<button className="heartbeat-toolbar__btn" type="button" disabled={templateLoadState.loading} onClick={() => void loadTemplates()}>
					<RefreshCw size={13} aria-hidden="true" />
					重试
				</button>
			</div>
		) : !editing && !viewingRun && templateLoadState.loading && templates.length === 0 ? (
			<div className="heartbeat-run-detail__note" data-automation-template-loading role="status">正在加载自动化模板…</div>
		) : null}

        {editing ? (
          <>
            {saveError && <div className="heartbeat-run-detail__note heartbeat-run-detail__note--error" role="alert">{saveError}</div>}
            <TaskEditor key={editing.id} task={editing} savePhase={savePhase} saveError={editorSaveError} capabilityRiskSummary={capabilityRiskSummary} scheduleCatalog={scheduleCatalog} deliveryTargetCatalog={deliveryTargetCatalog} cloudProjects={cloudProjects} projectLoadState={projectLoadState} onRetryProjects={loadCloudProjects} authenticated={authenticated} modelCatalog={modelCatalog} modelAccountScopeKey={modelAccountScopeKey} modelSettingsRevision={modelSettingsRevision} subscriptionAccess={subscriptionAccess} onConfigureModel={onConfigureModel} onClearModelError={clearModelError} onSave={handleSaveEdit} onCancel={returnToList} onDelete={() => void handleDelete(editing.id).then((saved) => { if (saved) returnToList(); })} />
          </>
        ) : viewingRun ? (
          <>
            {saveError && <div className="heartbeat-run-detail__note heartbeat-run-detail__note--error" role="alert">{saveError}</div>}
            <RunDetailView
              detail={runDetail}
              loading={runDetailLoading}
              onRefresh={refreshRunDetail}
              onCancelRun={() => void handleCancelRun(viewingRun.taskID, viewingRun.runID)}
              onOpenSession={() => {
                const manifest = runDetail?.manifest;
                if (!manifest?.sessionPath) return;
                const task = tasks.find((item) => item.id === manifest.taskId);
                onClose();
                onOpenTopic(
                  manifest.scope || "global",
                  manifest.workspaceRoot || "",
                  task?.topicId || manifest.taskId,
                  manifest.sessionPath,
                );
              }}
            />
          </>
        ) : usingTemplates ? (
          <div className="heartbeat-modal__body heartbeat-modal__body--templates">
            <div className="heartbeat-template-actions">
              <button className="heartbeat-toolbar__btn" type="button" onClick={handleRestoreTemplates}>
                <RefreshCw size={13} />
                <span>恢复已删除模板</span>
              </button>
            </div>
			{templates.length > 0 || (!templateLoadState.loading && !templateLoadState.error) ? (
				<TemplateGrid templates={templates} onSelect={handleUseTemplate} onDelete={handleDeleteTemplate} disabled={!quotaCanCreate} />
			) : null}
          </div>
		) : embedded ? (
			<>
				{saveError && <div className="heartbeat-run-detail__note heartbeat-run-detail__note--error" role="alert">{saveError}</div>}
				<span className="heartbeat-task-section__count heartbeat-task-section__count--workspace" title={quotaCreateMessage}>{quotaLabel}</span>
				<AutomationWorkspace
				templates={templates}
				tasks={tasks}
				runs={workspaceRuns}
				activeTab={workspaceTab}
				canCreate={quotaCanCreate}
				createDisabledTitle={quotaCreateMessage}
				onTabChange={setWorkspaceTab}
				onAddTask={() => void handleAdd()}
				onSelectTemplate={(template) => void handleUseTemplate(template)}
				renderTask={(task) => (
					<TaskCard
						task={task}
						nextRunAt={task.nextRunAt}
						quotaPaused={quotaPausedTaskIDs.has(task.id)}
						runs={runsByTask[task.id] || []}
						triggering={triggeringByTask[task.id]}
						scopeLabel={workspaceScopeLabel(task)}
						onToggle={() => void handleToggle(task)}
						onEdit={() => handleEdit(task)}
						onTrigger={() => void handleTrigger(task.id)}
						onDelete={() => void handleDelete(task.id)}
						onOpenRun={(runID) => void openRunDetail(task.id, runID)}
						onOpenTopic={onOpenTopic}
						onClose={onClose}
					/>
				)}
				renderRun={(run) => (
					<>
						<button
							className="automation-workspace__run-button"
							type="button"
							onClick={() => void openRunDetail(run.taskId, run.runId)}
						>
							<span>{run.taskName || run.taskId}</span>
							<strong>{automationRunStatusLabel(run.status)}</strong>
							<small>{automationRunReason(run) || automationRunTimeLabel(run.startedAt)}</small>
						</button>
						{runHistoryHasMore && run === workspaceRuns[workspaceRuns.length - 1] ? (
							<button
								className="heartbeat-toolbar__btn"
								type="button"
								disabled={runHistoryLoading}
								onClick={() => void loadWorkspaceRuns(runHistoryCursor)}
							>
								{runHistoryLoading ? "正在加载…" : "加载更多运行记录"}
							</button>
						) : null}
					</>
				)}
				/>
			</>
		) : (
          <div className={`heartbeat-modal__body${embedded ? " heartbeat-modal__body--workspace" : ""}`}>
            {saveError && <div className="heartbeat-run-detail__note heartbeat-run-detail__note--error" role="alert">{saveError}</div>}

            {embedded && templates.length > 0 && (
              <section className="heartbeat-embedded-templates" aria-labelledby="heartbeat-template-starters-title">
                <div className="heartbeat-embedded-templates__head">
                  <div>
                    <h2 id="heartbeat-template-starters-title">{t("heartbeat.templateStarters")}</h2>
                    <p>{t("heartbeat.templateStartersHint")}</p>
                  </div>
                  <button className="heartbeat-toolbar__btn" type="button" onClick={() => setUsingTemplates(true)}>
                    {t("heartbeat.viewAllTemplates")}
                    <ArrowRight size={13} aria-hidden="true" />
                  </button>
                </div>
                <TemplateGrid templates={templates.slice(0, 6)} onSelect={handleUseTemplate} onDelete={handleDeleteTemplate} deletable={false} disabled={!quotaCanCreate} />
              </section>
            )}

            <section className="heartbeat-task-section" aria-labelledby={embedded ? "heartbeat-my-tasks-title" : undefined}>
              {embedded && (
                <div className="heartbeat-task-section__head">
                  <h2 id="heartbeat-my-tasks-title">{t("heartbeat.myTasks" as any)}</h2>
                  <span className="heartbeat-task-section__count" title={quotaCreateMessage}>{quotaLabel}</span>
                </div>
              )}

              <div className="heartbeat-toolbar heartbeat-task-section__toolbar">
                {(tasks.length > 0 || !embedded) && (
                  <>
                    <div className="heartbeat-toolbar__search">
                      <Search size={13} className="heartbeat-toolbar__search-icon" />
                      <input
                        className="heartbeat-toolbar__search-input"
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        placeholder={t("heartbeat.searchPlaceholder" as any)}
                      />
                      {searchQuery && (
                        <button
                          className="heartbeat-toolbar__search-clear"
                          type="button"
                          onClick={() => setSearchQuery("")}
                          aria-label={t("heartbeat.clearSearch" as any)}
                        >
                          <X size={12} />
                        </button>
                      )}
                    </div>
                    <div className="heartbeat-scope-filter">
                      <button
                        ref={statusFilterRef}
                        className="heartbeat-toolbar__btn heartbeat-toolbar__btn--select"
                        type="button"
                        onClick={() => setStatusFilterOpen((v) => !v)}
                      >
                        <span>{statusFilterLabel(statusFilter)}</span>
                        <ChevronsUpDown size={12} />
                      </button>
                      <AnchoredPopover
                        open={statusFilterOpen}
                        anchorRef={statusFilterRef}
                        onClose={() => setStatusFilterOpen(false)}
                        className="heartbeat-filter-menu"
                        placement="bottom"
                      >
                        <div className="heartbeat-filter-menu__list" role="listbox">
                          {(["all", "enabled", "disabled"] as const).map((key) => (
                            <button
                              key={key}
                              className={`heartbeat-filter-menu__option${statusFilter === key ? " heartbeat-filter-menu__option--selected" : ""}`}
                              role="option"
                              aria-selected={statusFilter === key}
                              type="button"
                              onClick={() => { setStatusFilter(key); setStatusFilterOpen(false); }}
                            >
                              <span>{key === "all" ? t("heartbeat.filterAll" as any) : key === "enabled" ? t("heartbeat.filterEnabled" as any) : t("heartbeat.filterDisabled" as any)}</span>
                              {statusFilter === key && <Check size={12} className="heartbeat-filter-menu__check" />}
                            </button>
                          ))}
                        </div>
                      </AnchoredPopover>
                    </div>
                  </>
                )}
                <div className="heartbeat-scope-filter">
                  <button
                    ref={scopeFilterRef}
                    className="heartbeat-toolbar__btn heartbeat-toolbar__btn--select"
                    type="button"
                    onClick={() => setScopeFilterOpen((v) => !v)}
                  >
                    <span>{scopeFilterLabel(scopeFilter, workspaceMap)}</span>
                    <ChevronsUpDown size={12} />
                  </button>
                  <AnchoredPopover
                    open={scopeFilterOpen}
                    anchorRef={scopeFilterRef}
                    onClose={() => setScopeFilterOpen(false)}
                    className="heartbeat-filter-menu"
                    placement="bottom"
                  >
                    <div className="heartbeat-filter-menu__list" role="listbox">
                      <button
                        className={`heartbeat-filter-menu__option${scopeFilter === "all" ? " heartbeat-filter-menu__option--selected" : ""}`}
                        role="option"
                        aria-selected={scopeFilter === "all"}
                        type="button"
                        onClick={() => { setScopeFilter("all"); setScopeFilterOpen(false); }}
                      >
                        <span>{t("heartbeat.filterAllProjects" as any)}</span>
                        {scopeFilter === "all" && <Check size={12} className="heartbeat-filter-menu__check" />}
                      </button>
                      <button
                        className={`heartbeat-filter-menu__option${scopeFilter === "global" ? " heartbeat-filter-menu__option--selected" : ""}`}
                        role="option"
                        aria-selected={scopeFilter === "global"}
                        type="button"
                        onClick={() => { setScopeFilter("global"); setScopeFilterOpen(false); }}
                      >
                        <span>{t("heartbeat.scopeGlobal")}</span>
                        {scopeFilter === "global" && <Check size={12} className="heartbeat-filter-menu__check" />}
                      </button>
                      {workspaceOptions.map((item) => (
                        <button
                          key={item.value}
                          className={`heartbeat-filter-menu__option${scopeFilter === item.value ? " heartbeat-filter-menu__option--selected" : ""}`}
                          role="option"
                          aria-selected={scopeFilter === item.value}
                          type="button"
                          onClick={() => { setScopeFilter(item.value); setScopeFilterOpen(false); }}
                        >
                          <span>{item.label}</span>
                          {scopeFilter === item.value && <Check size={12} className="heartbeat-filter-menu__check" />}
                        </button>
                      ))}
                    </div>
                  </AnchoredPopover>
                </div>
                {!embedded && (
                  <div className="heartbeat-toolbar__actions">
                    <button className="heartbeat-toolbar__btn" onClick={() => setUsingTemplates(true)}>
                      <FileText size={14} />
                      模板使用
                    </button>
                    <button
                      className="heartbeat-toolbar__btn heartbeat-toolbar__btn--primary"
                      onClick={handleAdd}
                      disabled={!quotaCanCreate}
                      title={!quotaCanCreate ? quotaCreateMessage : undefined}
                    >
                      <Plus size={14} />
                      {t("heartbeat.addTask")}
                    </button>
                  </div>
                )}
              </div>

              {(() => {
                const filtered = tasks
                  .filter((task) => {
                    if (statusFilter === "enabled" && !task.enabled) return false;
                    if (statusFilter === "disabled" && task.enabled) return false;
                    if (searchQuery && !task.title.toLowerCase().includes(searchQuery.toLowerCase())) return false;
                    const workspaceKey = automationTaskWorkspaceKey(task);
                    if (scopeFilter === "global" && task.scope === "project" && workspaceKey) return false;
                    if (scopeFilter !== "all" && scopeFilter !== "global") {
                      if (task.scope !== "project" || workspaceKey !== scopeFilter) return false;
                    }
                    return true;
                  })
                  .sort((a, b) => {
                    if (a.enabled && !b.enabled) return -1;
                    if (!a.enabled && b.enabled) return 1;
                    return 0;
                  });

                const scopeLabel = (task: HeartbeatTask): string => {
									const key = automationTaskWorkspaceKey(task);
									if (task.scope !== "project" || !key) return t("heartbeat.scopeGlobal");
									return workspaceMap[key] || key.split(/[\\/]/).pop() || key;
                };

                return loading ? (
                  <div className="heartbeat-empty">
                    <Heart size={24} className="heartbeat-pulse" />
                    <span>{t("workspace.loading")}</span>
                  </div>
                ) : tasks.length === 0 ? (
                  <div className="heartbeat-empty heartbeat-empty--tasks">
                    <span className="heartbeat-empty__icon"><Heart size={20} /></span>
                    <div className="heartbeat-empty__copy">
                      <strong>{t("heartbeat.noTasksTitle" as any)}</strong>
                      <span>{t(scopeFilter === "all" ? "heartbeat.noTasksHint" as any : "heartbeat.noTasksInScope" as any)}</span>
                    </div>
                    {embedded && (
                      <button
                        className="heartbeat-toolbar__btn heartbeat-toolbar__btn--primary"
                        type="button"
                        onClick={handleAdd}
                        disabled={!quotaCanCreate}
                        title={!quotaCanCreate ? quotaCreateMessage : undefined}
                      >
                        <Plus size={14} aria-hidden="true" />
                        {t("heartbeat.addTask")}
                      </button>
                    )}
                  </div>
                ) : filtered.length === 0 ? (
                  <div className="heartbeat-empty">
                    <Search size={22} />
                    <span>{t("heartbeat.noMatches" as any)}</span>
                  </div>
                ) : (
                  <ul className="heartbeat-tasklist">
                    {filtered.map((task) => (
                      <TaskCard
                        key={task.id}
                        task={task}
						nextRunAt={task.nextRunAt}
                        quotaPaused={quotaPausedTaskIDs.has(task.id)}
                        runs={runsByTask[task.id] || []}
                        triggering={triggeringByTask[task.id]}
                        scopeLabel={scopeLabel(task)}
                        onToggle={() => {
									void handleToggle(task);
                        }}
                        onEdit={() => handleEdit(task)}
                        onTrigger={() => void handleTrigger(task.id)}
                        onDelete={() => void handleDelete(task.id)}
                        onOpenRun={(runID) => void openRunDetail(task.id, runID)}
                        onOpenTopic={onOpenTopic}
                        onClose={onClose}
                      />
                    ))}
                  </ul>
                );
              })()}
            </section>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Template Grid ─────────────────────────────────────────────────────────────

const TEMPLATE_ICON_MAP: Record<string, typeof FileText> = {
  business: Activity,
  calendar: Clock,
  chat: MessageSquare,
  checklist: Check,
  clipboard: Clipboard,
  film: Play,
  health: Heart,
  image: FileImage,
  language: FileText,
  message: MessageSquare,
  moon: Heart,
  news: FileText,
  question: CircleHelp,
};

function TemplateGrid({
  templates,
  onSelect,
  onDelete,
  deletable = true,
  disabled = false,
}: {
  templates: AutomationTaskTemplate[];
  onSelect: (template: AutomationTaskTemplate) => void;
  onDelete: (template: AutomationTaskTemplate) => Promise<void>;
  deletable?: boolean;
  disabled?: boolean;
}) {
  const t = useT();
  const [confirmingTemplateDeleteID, setConfirmingTemplateDeleteID] = useState<string | null>(null);

  if (templates.length === 0) {
    return (
      <div className="heartbeat-empty">
        <FileText size={24} />
        <span>暂无可用模板</span>
      </div>
    );
  }

  return (
    <div className="heartbeat-template-grid">
      {templates.map((template) => {
        const Icon = TEMPLATE_ICON_MAP[template.icon || ""] || FileText;
        const confirmingDelete = deletable && confirmingTemplateDeleteID === template.id;
        return (
          <div
            key={template.id}
            className={`heartbeat-template-card-wrap${confirmingDelete ? " heartbeat-template-card-wrap--confirming" : ""}`}
          >
            <button
              className={`heartbeat-template-card${deletable ? " heartbeat-template-card--deletable" : ""}`}
              type="button"
              disabled={disabled}
              onClick={() => onSelect(template)}
            >
              <span className="heartbeat-template-card__icon"><Icon size={16} /></span>
              <span className="heartbeat-template-card__title">{template.title}</span>
              <span className="heartbeat-template-card__desc">{template.description}</span>
            </button>
            {deletable && (confirmingDelete ? (
              <span
                className="heartbeat-template-card__confirm"
                role="group"
                aria-label={`${t("heartbeat.confirmDelete" as any)} ${template.title}`}
                onClick={(e) => e.stopPropagation()}
              >
                <span className="heartbeat-template-card__confirm-label">{t("heartbeat.confirmDelete" as any)}</span>
                <button
                  className="heartbeat-template-card__confirm-btn heartbeat-template-card__confirm-btn--danger"
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setConfirmingTemplateDeleteID(null);
                    void onDelete(template);
                  }}
                >
                  {t("common.delete")}
                </button>
                <button
                  className="heartbeat-template-card__confirm-btn"
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    setConfirmingTemplateDeleteID(null);
                  }}
                >
                  {t("common.cancel")}
                </button>
              </span>
            ) : (
              <button
                className="heartbeat-template-card__delete"
                type="button"
                aria-label={`删除模板 ${template.title}`}
                title="删除模板"
                onClick={(e) => {
                  e.stopPropagation();
                  setConfirmingTemplateDeleteID(template.id);
                }}
              >
                <Trash2 size={13} />
              </button>
            ))}
          </div>
        );
      })}
    </div>
  );
}

// ── Task Card ─────────────────────────────────────────────────────────────────

function TaskCard({
  task,
  nextRunAt,
  quotaPaused,
  runs,
  triggering,
  scopeLabel,
  onToggle,
  onEdit,
  onTrigger,
  onDelete,
  onOpenRun,
  onOpenTopic,
  onClose,
}: {
  task: HeartbeatTask;
  nextRunAt?: string;
  quotaPaused: boolean;
  runs: AutomationRunSummary[];
  triggering?: TriggerRequest;
  scopeLabel: string;
  onToggle: () => void;
  onEdit: () => void;
  onTrigger: () => void;
  onDelete: () => void;
  onOpenRun: (runID: string) => void;
  onOpenTopic: (scope: string, workspaceRoot: string, topicId: string, sessionPath?: string) => void;
  onClose: () => void;
}) {
  const t = useT();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const intervalLabel = heartbeatIntervalLabel(task.interval, t);
  const hasCurrentRun = !!triggering && runs.some((run) => (
    run.taskId === task.id &&
    !!run.startedAt &&
    Date.parse(run.startedAt) >= triggering.startedAt
  ));
  const showStartingRun = !!triggering && !hasCurrentRun;
  const runNowLabel = triggering ? `${t("heartbeat.runNow")}（启动中）` : t("heartbeat.runNow");

  const nextRunLabel = quotaPaused ? "套餐额度暂停" : backendNextRunLabel(nextRunAt, Date.now(), t);

  const lastRunLabel = task.lastRunAt
    ? (() => {
        const d = new Date(task.lastRunAt);
        const now = new Date();
        const diff = now.getTime() - task.lastRunAt;
        if (diff < 60000) return t("heartbeat.justNow" as any);
        if (diff < 3600000) return `${Math.floor(diff / 60000)}${t("heartbeat.minAgo" as any)}`;
        if (diff < 86400000) return `${Math.floor(diff / 3600000)}${t("heartbeat.hourAgo" as any)}`;
        return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`;
      })()
    : t("heartbeat.neverRun");

  return (
    <li className={`heartbeat-card${!task.enabled ? " heartbeat-card--disabled" : ""}${quotaPaused ? " heartbeat-card--quota-paused" : ""}`}>
      <div className="heartbeat-card__head">
        <span className={`heartbeat-card__dot${task.enabled && !quotaPaused ? " heartbeat-card__dot--on" : ""}`} />
        <span className="heartbeat-card__title">
          <button
            type="button"
            className="heartbeat-card__title-btn"
            onClick={onEdit}
          >
            <span className="heartbeat-card__title-text">{task.title || t("heartbeat.untitled")}</span>
            <span className="heartbeat-card__title-scope">{scopeLabel}</span>
            {task.businessDiagnostics && <span className="heartbeat-card__badge">经营诊断</span>}
            {quotaPaused && <span className="heartbeat-card__badge heartbeat-card__badge--quota">套餐额度暂停</span>}
          </button>
        </span>
        <span className="heartbeat-card__meta-item heartbeat-card__meta-item--compact">
          <Clock size={10} />
          {intervalLabel}
          <span className="heartbeat-card__meta-sep">·</span>
          {task.enabled ? nextRunLabel : lastRunLabel}
        </span>
        <span className="heartbeat-card__head-actions">
          <button
            className="heartbeat-card__open-btn heartbeat-card__open-btn--play"
            type="button"
            disabled={!task.enabled || quotaPaused || !!triggering}
            onClick={onTrigger}
            title={quotaPaused ? "升级套餐后自动恢复运行" : runNowLabel}
            aria-label={quotaPaused ? "升级套餐后自动恢复运行" : runNowLabel}
            aria-busy={!!triggering}
          >
            <Play size={12} />
          </button>
          <button
            className="heartbeat-card__open-btn"
            type="button"
            disabled={!task.topicId}
            onClick={() => {
              if (task.topicId) {
                onClose();
                onOpenTopic(task.scope || "global", task.workspaceRoot || "", task.topicId);
              }
            }}
            title={t("heartbeat.openTopic" as any)}
            aria-label={t("heartbeat.openTopic" as any)}
          >
            <MessageSquare size={13} />
          </button>
          {confirmingDelete ? (
            <span className="heartbeat-card__confirm" role="group" aria-label={t("heartbeat.confirmDelete" as any)}>
              <span className="heartbeat-card__confirm-label">{t("heartbeat.confirmDelete" as any)}</span>
              <button
                className="heartbeat-card__btn heartbeat-card__btn--danger"
                type="button"
                onClick={() => {
                  setConfirmingDelete(false);
                  onDelete();
                }}
              >
                {t("common.delete")}
              </button>
              <button className="heartbeat-card__btn" type="button" onClick={() => setConfirmingDelete(false)}>
                {t("common.cancel")}
              </button>
            </span>
          ) : (
            <button
              className="heartbeat-card__open-btn heartbeat-card__open-btn--danger"
              type="button"
              onClick={() => setConfirmingDelete(true)}
              title={t("heartbeat.delete")}
              aria-label={t("heartbeat.delete")}
            >
              <Trash2 size={13} />
            </button>
          )}
          <button
            className={`heartbeat-card__toggle${task.enabled ? " heartbeat-card__toggle--on" : ""}`}
            onClick={onToggle}
            disabled={quotaPaused}
            aria-label={task.enabled ? t("heartbeat.disable") : t("heartbeat.enabled")}
            title={quotaPaused ? "升级套餐后自动恢复运行" : undefined}
          >
            <span className="heartbeat-card__toggle-knob" />
          </button>
        </span>
      </div>
      {(showStartingRun || runs.length > 0) && (
        <div className="heartbeat-card__runs" aria-label="最近运行">
          {showStartingRun && (
            <div className="heartbeat-card__run heartbeat-card__run--starting" role="status" aria-live="polite">
              <span className="heartbeat-run-status heartbeat-run-status--running">启动中</span>
              <span className="heartbeat-card__run-time">刚刚</span>
              <span className="heartbeat-card__run-reason">正在创建运行记录</span>
            </div>
          )}
          {runs.map((run) => (
            <button
              key={run.runId}
              className="heartbeat-card__run"
              type="button"
              onClick={() => onOpenRun(run.runId)}
              title={run.runId}
            >
              <span className={`heartbeat-run-status heartbeat-run-status--${automationRunStatusTone(run.status)}`}>
                {automationRunStatusLabel(run.status)}
              </span>
              <span className="heartbeat-card__run-time">{automationRunTimeLabel(run.startedAt)}</span>
              <span className="heartbeat-card__run-reason">{automationRunReason(run) || run.runId}</span>
            </button>
          ))}
        </div>
      )}
    </li>
  );
}

// ── Run Detail ────────────────────────────────────────────────────────────────

function RunDetailView({
  detail,
  loading,
  onRefresh,
  onCancelRun,
  onOpenSession,
}: {
  detail: AutomationRunDetail | null;
  loading: boolean;
  onRefresh: () => void;
  onCancelRun: () => void;
  onOpenSession: () => void;
}) {
  const [exportingFormat, setExportingFormat] = useState<AutomationRunResultExportFormat | null>(null);
  const [exportError, setExportError] = useState("");
	const [retryingDelivery, setRetryingDelivery] = useState<string | null>(null);
  const manifest = detail?.manifest;
  const status = manifest?.status;
  const owner = manifest?.hostOwner || manifest?.owner || "-";
  const errorText = [manifest?.errorCode, manifest?.error || manifest?.errorMessage].filter(Boolean).join(": ");
  const usageRows = automationUsageRows(manifest?.usage);
  const events = normalizeAutomationRunEvents(detail?.events);
  const resultText = detail?.resultSummary || manifest?.summary || "";
  const hasResult = !!resultText;
  const exportFormats = (detail?.availableResultFormats || []).filter((item) => item?.format && item?.label);
  const hasExportableResult = !!manifest && exportFormats.length > 0;
  const userMessage = manifest ? automationRunUserMessage(manifest, hasResult) : "";
  const capabilities = manifest?.capabilities;
  const deliveryAttempts = detail?.deliveries || [];
  const skillSnapshotText = capabilities?.skills?.length
    ? capabilities.skills.map((skill) => `${skill.id}${skill.version ? `@${skill.version}` : ""}`).join("、")
    : capabilities?.skillIds?.join("、") || "未使用";
  const expertSnapshotText = capabilities?.expert
    ? `${capabilities.expert.id}${capabilities.expert.version ? `@${capabilities.expert.version}` : ""}`
    : capabilities?.expertId || "未使用";
  const connectorGrantText = capabilities?.connectorGrants?.length
    ? capabilities.connectorGrants.map((grant) => `${grant.connectorId} · ${grant.source} · ${grant.grantId}`).join("；")
    : capabilities?.connectorIds?.join("、") || "未使用";

  async function handleExportResult(format: AutomationRunResultExportFormat) {
    if (!manifest) return;
    setExportError("");
    setExportingFormat(format);
    try {
      const exported = await automationExportRunResult(manifest.taskId, manifest.runId, format);
      if (!exported?.payload) throw new Error("导出结果为空");
      const filename = exported.filename || `automation-result-${manifest.runId}.${format}`;
      const path = await app.PickExportFile(filename, exported.mimeType || "application/octet-stream");
      if (path) await app.SaveExportFile(path, exported.payload, exported.base64Encoded !== false);
    } catch (err) {
      setExportError(reportClientError("automation.export", err, {
        fallbackMessage: "导出自动化结果失败，请稍后重试。",
      }).userMessage);
    } finally {
      setExportingFormat(null);
    }
  }

	async function handleRetryDelivery(attemptID: string) {
		if (!manifest) return;
		setRetryingDelivery(attemptID);
		try {
			await automationRetryDelivery(manifest.taskId, manifest.runId, attemptID);
			onRefresh();
		} catch (err) {
			setExportError(heartbeatErrorMessage(err));
		} finally {
			setRetryingDelivery(null);
		}
	}

  return (
    <div className="heartbeat-run-detail">
      <div className="heartbeat-run-detail__toolbar">
        <button className="heartbeat-toolbar__btn" type="button" onClick={onRefresh} disabled={loading}>
          <RefreshCw size={13} />
          刷新
        </button>
        {status === "running" && (
          <button className="heartbeat-toolbar__btn" type="button" onClick={onCancelRun}>
            <Square size={12} />
            取消
          </button>
        )}
      </div>

      {loading ? (
        <div className="heartbeat-empty">
          <FileText size={24} />
          <span>正在读取运行归档</span>
        </div>
      ) : !manifest ? (
        <div className="heartbeat-empty">
          <AlertTriangle size={24} />
          <span>未能读取运行详情</span>
        </div>
      ) : (
        <>
          <section className="heartbeat-run-detail__summary">
            <span className={`heartbeat-run-status heartbeat-run-status--${automationRunStatusTone(status)}`}>
              {automationRunStatusLabel(status)}
            </span>
            <div className="heartbeat-run-detail__summary-main">
              <span className="heartbeat-run-detail__eyebrow">运行概要</span>
              <strong>{manifest.taskName || manifest.taskId}</strong>
              <span>
                <Clock size={12} />
                {automationRunTimeRange(manifest)}
              </span>
              <span>耗时 {automationDurationLabel(manifest.durationMs)}</span>
            </div>
          </section>

          {userMessage && (
            <section
              className={`heartbeat-run-detail__block ${
                status === "failed" || status === "canceled"
                  ? "heartbeat-run-detail__block--error"
                  : "heartbeat-run-detail__block--blocked"
              }`}
            >
              <h3>{status === "failed" ? "失败原因" : "状态说明"}</h3>
              <p>{userMessage}</p>
            </section>
          )}

          <details className="heartbeat-run-detail__technical heartbeat-run-detail__technical--result" open>
            <summary>最终结果</summary>
            <section className="heartbeat-run-detail__result-panel">
              <div className="heartbeat-run-detail__block-head">
                <h3>结果与产物</h3>
                <div className="heartbeat-run-detail__export" aria-label="导出最终结果">
                  {manifest?.sessionPath ? (
                    <button type="button" onClick={onOpenSession} title="打开关联会话">
                      <MessageSquare size={12} />
                      打开关联会话
                    </button>
                  ) : null}
                  {exportFormats.map((item) => (
                    <button
                      key={item.format}
                      type="button"
                      onClick={() => void handleExportResult(item.format)}
                      disabled={!hasExportableResult || !!exportingFormat}
                      title={`导出 ${item.label}`}
                    >
                      <Download size={12} />
                      {exportingFormat === item.format ? "..." : item.label}
                    </button>
                  ))}
                </div>
              </div>
              {resultText ? (
                <>
                  <div className="heartbeat-run-detail__result-markdown">
                    <Markdown text={resultText} />
                  </div>
                  {detail?.resultTruncated && <span className="heartbeat-run-detail__note">结果已截断，导出会读取完整 result.md</span>}
                </>
              ) : (
                <p className="heartbeat-run-detail__empty-result">尚未生成结果</p>
              )}
              {exportError && <span className="heartbeat-run-detail__note heartbeat-run-detail__note--error">{exportError}</span>}
              <section className="heartbeat-run-detail__artifacts" aria-label="运行产物">
                <RunFact label="结果文件" value={manifest.resultPath || "尚未生成"} wide mono />
                <RunFact label="会话归档" value={manifest.sessionPath || "未保留"} wide mono />
                <RunFact label="事件归档" value={manifest.eventsPath || "未保留"} wide mono />
              </section>
            </section>
          </details>

					<details className="heartbeat-run-detail__technical" open>
						<summary>通知投递</summary>
						{deliveryAttempts.length ? (
							<ul className="heartbeat-run-deliveries">
								{deliveryAttempts.map((delivery) => (
									<li key={delivery.id}>
										<div className="heartbeat-run-deliveries__target">
											<strong>{delivery.target.type}</strong>
											<span title={delivery.target.ref}>{delivery.target.ref}</span>
										</div>
										<span className={`heartbeat-run-deliveries__status heartbeat-run-deliveries__status--${delivery.status}`}>
											{automationDeliveryStatusLabel(delivery.status)}
										</span>
										<em>{delivery.failure?.message || "无失败信息"}</em>
										{delivery.status === "failed" && delivery.retryable ? (
											<button
												type="button"
												disabled={retryingDelivery === delivery.id}
												onClick={() => void handleRetryDelivery(delivery.id)}
												aria-label={`重试投递 ${delivery.target.type} ${delivery.target.ref}`}
											>
												{retryingDelivery === delivery.id ? "重试中…" : "重试投递"}
											</button>
										) : null}
									</li>
								))}
							</ul>
						) : (
							<p className="heartbeat-run-detail__empty-section">本次运行没有通知投递记录</p>
						)}
					</details>

          <details className="heartbeat-run-detail__technical">
            <summary>事件时间线</summary>
            {events.length ? (
              <ol className="heartbeat-run-timeline">
                {events.map((event, idx) => (
                  <li key={`${event.time || ""}-${idx}`}>
                    <time dateTime={event.time}>{automationRunTimeLabel(event.time)}</time>
                    <span className="heartbeat-run-timeline__dot" aria-hidden="true" />
                    <div>
                      <strong>{automationRunEventLabel(event)}</strong>
                      <p>{automationRunEventText(event)}</p>
                    </div>
                  </li>
                ))}
              </ol>
            ) : (
              <p className="heartbeat-run-detail__empty-section">本次运行没有可显示的事件</p>
            )}
          </details>

          <details className="heartbeat-run-detail__technical" open>
            <summary>使用量</summary>
            {usageRows.length ? (
              <section className="heartbeat-run-detail__grid" aria-label="模型使用量">
                {usageRows.map((item) => <RunFact key={item.label} label={item.label} value={item.value} />)}
              </section>
            ) : (
              <p className="heartbeat-run-detail__empty-section">后端未返回本次运行的使用量</p>
            )}
          </details>

          <details className="heartbeat-run-detail__technical" open>
            <summary>权限与能力快照</summary>
            <section className="heartbeat-run-detail__grid" aria-label="运行时冻结的权限与能力">
              <RunFact label="执行主体" value={capabilities?.principal || "未记录"} wide mono />
              <RunFact label="模型" value={capabilities?.modelRef || manifest.model || "未记录"} />
								<RunFact label="审批模式" value={capabilities?.approvalMode || "未记录"} />
              <RunFact label="冻结调度" value={automationScheduleSnapshotLabel(manifest)} wide />
              <RunFact label="生效区间" value={automationEffectiveRangeLabel(manifest)} wide />
              <RunFact label="冻结时间" value={automationRunTimeLabel(capabilities?.capturedAt)} />
              <RunFact label="技能" value={skillSnapshotText} wide />
              <RunFact label="专家" value={expertSnapshotText} wide />
              <RunFact label="连接器授权" value={connectorGrantText} wide />
              <RunFact label="允许工具" value={capabilities?.toolAllowlist?.join("、") || "未授权工具"} wide mono />
              <RunFact label="能力风险" value={capabilities?.riskSummary || "未记录"} wide />
              <RunFact label="提示词指纹" value={capabilities?.promptHash || "未记录"} wide mono />
            </section>
          </details>

          <details className="heartbeat-run-detail__technical">
            <summary>技术详情</summary>
            <section className="heartbeat-run-detail__grid" aria-label="manifest">
              <RunFact label="run id" value={manifest.runId || "-"} wide mono />
              <RunFact label="触发" value={manifest.triggerReason || manifest.trigger || "-"} />
              <RunFact label="跳过" value={manifest.skipReason || manifest.conditionReason || "-"} />
              <RunFact label="宿主" value={owner} />
              <RunFact label="租约" value={manifest.leaseId || "-"} />
              <RunFact label="模型" value={manifest.model || "-"} />
								<RunFact label="任务版本" value={String(manifest.taskRevision || "-")} />
              <RunFact label="计费来源" value={manifest.billingSource || "-"} />
              <RunFact label="工作区" value={manifest.workspaceRoot || "-"} wide />
              <RunFact label="开始" value={automationRunTimeLabel(manifest.startedAt)} />
              <RunFact label="结束" value={automationRunTimeLabel(manifest.finishedAt)} />
            </section>

            {errorText && (
              <section className="heartbeat-run-detail__block heartbeat-run-detail__block--technical-error">
                <h3>错误码</h3>
                <pre>{errorText}</pre>
              </section>
            )}

          </details>
        </>
      )}
    </div>
  );
}

function RunFact({
  label,
  value,
  wide,
  mono,
}: {
  label: string;
  value: string;
  wide?: boolean;
  mono?: boolean;
}) {
  return (
    <div className={`heartbeat-run-fact${wide ? " heartbeat-run-fact--wide" : ""}`}>
      <span>{label}</span>
      <strong className={mono ? "heartbeat-run-fact__mono" : ""} title={value}>{value}</strong>
    </div>
  );
}

// ── Cycle Editor ──────────────────────────────────────────────────────────────

const WEEKDAYS = [
  { key: "mon", label: "周一" },
  { key: "tue", label: "周二" },
  { key: "wed", label: "周三" },
  { key: "thu", label: "周四" },
  { key: "fri", label: "周五" },
  { key: "sat", label: "周六" },
  { key: "sun", label: "周日" },
] as const;

const ALL_WEEKDAYS = WEEKDAYS.map(w => w.key);
const DEFAULT_WEEKLY_DAY = "mon";

function defaultHeartbeatCycleDays(cycleType: string): string[] {
  if (cycleType === "daily") return [...ALL_WEEKDAYS];
  if (cycleType === "weekly" || cycleType === "biweekly") return [DEFAULT_WEEKLY_DAY];
  return [];
}

export function heartbeatBuildCycleInterval(cycleType: string, days: string[], time: string): string {
  const base: Record<string, string> = {
    daily: "24h",
    weekly: "168h",
    biweekly: "336h",
    monthly: "720h",
    yearly: "8760h",
  };
  const selectedDays = days.filter(Boolean);
  const isDailyWithSelection = cycleType === "daily" && selectedDays.length > 0 && selectedDays.length < 7;
  const isDailyWithoutSelection = cycleType === "daily" && selectedDays.length === 0;
  const effectiveType = isDailyWithoutSelection || isDailyWithSelection ? "weekly" : cycleType;
  const scheduleDays =
    (effectiveType === "weekly" || effectiveType === "biweekly") && selectedDays.length === 0
      ? defaultHeartbeatCycleDays(effectiveType)
      : selectedDays;

  let suffix = `|${effectiveType}`;
  if (effectiveType === "weekly" || effectiveType === "biweekly") {
    suffix += `:${scheduleDays.join(",")}`;
  } else if (effectiveType === "monthly") {
    suffix += `:${scheduleDays[0] || "1"}`;
  } else if (effectiveType === "yearly") {
    suffix += `:${scheduleDays[0] || "1"}-${scheduleDays[1] || "1"}`;
  }
  suffix += `@${time}`;
  return (base[cycleType] || "24h") + suffix;
}

function CycleEditor({
  draft,
  setDraft,
  scheduleCatalog,
}: {
  draft: HeartbeatTask;
  setDraft: (field: keyof HeartbeatTask, value: string | boolean) => void;
  scheduleCatalog: AutomationScheduleCatalogEntry[];
}) {
  const t = useT();
  const cycleMatch = (draft.interval || "").match(/^(\d+)[smh]\|(daily|weekly|biweekly|monthly|yearly)(?::([^@]*))?(?:@(\d{2}:\d{2}))?$/);
  const [cycleType, setCycleType] = useState<string>(
    cycleMatch ? cycleMatch[2] : "daily"
  );
  const cycleDays = cycleMatch?.[3] || "";
  const cycleTime = cycleMatch?.[4] || "09:00";
  const [selectedDays, setSelectedDays] = useState<string[]>(
    cycleDays ? cycleDays.split(",").filter(Boolean) :
    defaultHeartbeatCycleDays(cycleMatch ? cycleMatch[2] : "daily")
  );
  const [monthDay, setMonthDay] = useState(cycleDays || "1");
  const [yearMonth, setYearMonth] = useState(cycleDays.split("-")[0] || "1");
  const [yearDay, setYearDay] = useState(cycleDays.split("-")[1] || "1");
  const [timeVal, setTimeVal] = useState(cycleTime);

  const hasWeekdays = cycleType === "weekly" || cycleType === "biweekly";

  // Build interval string when config changes
  const buildInterval = useCallback(heartbeatBuildCycleInterval, []);

  const onCycleTypeChange = useCallback((ct: string) => {
    setCycleType(ct);
    const days = defaultHeartbeatCycleDays(ct);
    setSelectedDays(days);
    setMonthDay("1");
    setYearMonth("1");
    setYearDay("1");
    setDraft("interval", buildInterval(ct, days, timeVal));
  }, [buildInterval, setDraft, timeVal]);

  const onDayToggle = useCallback((day: string) => {
    if (selectedDays.includes(day) && selectedDays.length <= 1) return;
    const next = selectedDays.includes(day)
      ? selectedDays.filter((selectedDay) => selectedDay !== day)
      : [...selectedDays, day];
    setSelectedDays(next);
    setDraft("interval", buildInterval(cycleType, next, timeVal));
  }, [buildInterval, cycleType, selectedDays, setDraft, timeVal]);

  const onMonthDayChange = useCallback((d: string) => {
    setMonthDay(d);
    setDraft("interval", buildInterval(cycleType, [d], timeVal));
  }, [buildInterval, cycleType, setDraft, timeVal]);

  const onYearMonthChange = useCallback((m: string) => {
    setYearMonth(m);
    setDraft("interval", buildInterval(cycleType, [m, yearDay], timeVal));
  }, [buildInterval, cycleType, setDraft, timeVal, yearDay]);

  const onYearDayChange = useCallback((d: string) => {
    setYearDay(d);
    setDraft("interval", buildInterval(cycleType, [yearMonth, d], timeVal));
  }, [buildInterval, cycleType, setDraft, timeVal, yearMonth]);

  const onTimeChange = useCallback((tm: string) => {
    setTimeVal(tm);
    const days = hasWeekdays ? selectedDays
      : cycleType === "monthly" ? [monthDay]
      : cycleType === "yearly" ? [yearMonth, yearDay]
      : [];
    setDraft("interval", buildInterval(cycleType, days, tm));
  }, [buildInterval, cycleType, selectedDays, monthDay, yearMonth, yearDay, setDraft]);

  const MONTHS = Array.from({ length: 12 }, (_, i) => ({
    value: String(i + 1),
    label: `${i + 1}月`,
  }));
  const DAYS = Array.from({ length: 31 }, (_, i) => ({
    value: String(i + 1),
    label: `${i + 1}日`,
  }));
  const cycleTypeOptions: HeartbeatSelectOption[] = scheduleCatalog
		.filter((entry) => entry.kind === "calendar" && !!entry.frequency)
		.map((entry) => ({
			value: entry.frequency!,
			label: entry.frequency === "daily" ? t("heartbeat.cycleDaily")
				: entry.frequency === "weekly" ? t("heartbeat.cycleWeekly")
					: entry.frequency === "biweekly" ? "每两周"
						: entry.frequency === "monthly" ? t("heartbeat.cycleMonthly")
							: entry.frequency === "yearly" ? t("heartbeat.cycleYearly")
								: entry.frequency || entry.id,
		}));

  return (
    <div className="heartbeat-editor__cycle-wrap">
      <div className="heartbeat-editor__cycle-row">
        <HeartbeatSelect
          className="heartbeat-editor-select--freq"
          value={cycleType}
          options={cycleTypeOptions}
          ariaLabel={t("heartbeat.freqCycle")}
          onChange={onCycleTypeChange}
        />

        {cycleType === "monthly" && (
          <HeartbeatSelect
            className="heartbeat-editor-select--freq heartbeat-editor-select--freq-day"
            value={monthDay}
            options={DAYS}
            ariaLabel={t("heartbeat.cycleMonthly")}
            onChange={onMonthDayChange}
          />
        )}

        {cycleType === "yearly" && (
          <>
            <HeartbeatSelect
              className="heartbeat-editor-select--freq heartbeat-editor-select--freq-day"
              value={yearMonth}
              options={MONTHS}
              ariaLabel={t("heartbeat.cycleYearly")}
              onChange={onYearMonthChange}
            />
            <HeartbeatSelect
              className="heartbeat-editor-select--freq heartbeat-editor-select--freq-day"
              value={yearDay}
              options={DAYS}
              ariaLabel={t("heartbeat.cycleYearly")}
              onChange={onYearDayChange}
            />
          </>
        )}

        <input
          className="heartbeat-editor__freq-input heartbeat-editor__freq-input--time"
          type="time"
          aria-label="运行时间"
          value={timeVal}
          onChange={(e) => onTimeChange(e.target.value)}
        />

        {hasWeekdays && (
          <div className="set-seg">
            {WEEKDAYS.map((wd) => (
              <button
                key={wd.key}
                type="button"
                className={`set-seg__btn${selectedDays.includes(wd.key) ? " set-seg__btn--on" : ""}`}
                onClick={() => onDayToggle(wd.key)}
                aria-pressed={selectedDays.includes(wd.key)}
              >
                {wd.label}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Editor ─────────────────────────────────────────────────────────────────────

function normalizeMode(mode: "ask" | "auto" | "yolo" | undefined): "ask" | "auto" | "yolo" {
  if (mode === "ask" || mode === "auto" || mode === "yolo") return mode;
  return "auto";
}

function deliveryTargetKey(target: { type: string; ref: string }): string {
	return `${target.type}\u0000${target.ref}`;
}

function sortBusinessSources(sources: BusinessDataSource[]): BusinessDataSource[] {
  const byKind = new Map(sources.map((source) => [source.kind, source]));
  return BUSINESS_SOURCE_DEFS.flatMap((def) => {
    const source = byKind.get(def.kind);
    return source ? [source] : [];
  });
}

function businessRunTime(interval: string | undefined): string {
  return interval?.match(/@(\d{2}:\d{2})$/)?.[1] || "03:30";
}

function businessDiagnosticsValidationErrors(config: BusinessDiagnosticsConfig | undefined): string[] {
  if (!config) return [];
  const errors: string[] = [];
  const start = Date.parse(config.targetStart);
  const end = Date.parse(config.targetEnd);
  if (!Number.isFinite(start) || !Number.isFinite(end) || start >= end) {
    errors.push("请选择有效的目标日期范围。");
  }
  for (const def of BUSINESS_SOURCE_DEFS) {
    const source = config.sources.find((item) => item.kind === def.kind);
    if (def.required && !source) {
      errors.push(`${def.label}数据源必填。`);
    }
  }
  for (const source of config.sources) {
    const def = BUSINESS_SOURCE_DEFS.find((item) => item.kind === source.kind);
    if (!def) {
      errors.push(`未知数据源类型：${source.kind}`);
      continue;
    }
    if (!source.id.trim()) errors.push(`${def.label}数据源 ID 必填。`);
    if (source.type === "csv" || source.type === "xlsx") {
      if (!source.path?.trim()) errors.push(`${def.label}文件路径必填。`);
    } else if (source.type === "http") {
      if (!source.url?.trim()) errors.push(`${def.label} HTTP URL 必填。`);
    } else if (source.type === "mcp") {
      if (!Array.isArray(source.rows)) errors.push(`${def.label} MCP rows 必须是 JSON 数组。`);
    } else {
      errors.push(`${def.label}数据源输入类型无效。`);
    }
  }
  return errors;
}

function parseBusinessRecipients(value: string): string[] {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function parseBusinessRows(text: string): { rows?: Record<string, unknown>[]; error?: string } {
  try {
    const parsed = JSON.parse(text || "[]");
    if (!Array.isArray(parsed)) return { error: "MCP rows 必须是 JSON 数组。" };
    if (parsed.some((row) => !row || typeof row !== "object" || Array.isArray(row))) {
      return { error: "每条 MCP row 必须是 JSON 对象。" };
    }
    return { rows: parsed as Record<string, unknown>[] };
  } catch (err) {
    reportClientError("automation.parseRows", err, {
      fallbackMessage: "JSON 行数据格式无效，请检查后重试。",
    });
    return { error: "JSON 行数据格式无效，请检查后重试。" };
  }
}

function isBusinessDataKind(value: string | undefined): value is BusinessDataKind {
  return BUSINESS_SOURCE_DEFS.some((def) => def.kind === value);
}

function businessSourceKindAtPoint(point: { x: number; y: number } | undefined): BusinessDataKind | null {
  if (!point || typeof document === "undefined") return null;
  const target = document.elementFromPoint(point.x, point.y);
  const raw = target instanceof HTMLElement
    ? target.closest<HTMLElement>("[data-business-source-kind]")?.dataset.businessSourceKind
    : undefined;
  return isBusinessDataKind(raw) ? raw : null;
}

function businessFileTypeFromPath(path: string, fallback: BusinessSourceType): BusinessSourceType {
  const lower = path.toLowerCase();
  if (lower.endsWith(".xlsx")) return "xlsx";
  if (lower.endsWith(".csv")) return "csv";
  return fallback === "xlsx" ? "xlsx" : "csv";
}

function businessFileName(path: string): string {
  return path.replace(/[\\/]+$/, "").split(/[\\/]/).filter(Boolean).pop() || path;
}

function normalizeBusinessFilePath(raw: string): string {
  const text = raw.trim();
  if (!text || /^about:blank(?:#blocked)?/i.test(text)) return "";
  if (text.startsWith("file://")) {
    try {
      const url = new URL(text);
      const pathname = decodeURIComponent(url.pathname).replace(/^\/([A-Za-z]:[\\/])/, "$1");
      return url.hostname ? `//${url.hostname}${pathname}` : pathname;
    } catch {
      return "";
    }
  }
  if (/^[A-Za-z][A-Za-z0-9+.-]*:/.test(text) && !/^[A-Za-z]:[\\/]/.test(text)) return "";
  return text;
}

function filePathFromDataTransfer(dt: DataTransfer): string {
  const file = dt.files?.[0] as (File & { path?: string }) | undefined;
  if (file?.path) return normalizeBusinessFilePath(file.path);
  const text = (dt.getData("text/uri-list") || dt.getData("text/plain") || "").split(/\r?\n/)[0]?.trim();
  return normalizeBusinessFilePath(text || "");
}

function BusinessDiagnosticsEditor({
  config,
  interval,
  onConfigChange,
  onIntervalChange,
  onErrorsChange,
}: {
  config: BusinessDiagnosticsConfig;
  interval: string;
  onConfigChange: (config: BusinessDiagnosticsConfig) => void;
  onIntervalChange: (interval: string) => void;
  onErrorsChange: (errors: string[]) => void;
}) {
  const [rowsTextByID, setRowsTextByID] = useState<Record<string, string>>(() => {
    const entries = config.sources.map((source) => [
      source.id,
      JSON.stringify(source.rows || [], null, 2),
    ]);
    return Object.fromEntries(entries);
  });
  const [rowErrorsByID, setRowErrorsByID] = useState<Record<string, string>>({});
  const [draggingKind, setDraggingKind] = useState<BusinessDataKind | null>(null);
  const draggingKindRef = useRef<BusinessDataKind | null>(null);

  useEffect(() => {
    onErrorsChange(Object.values(rowErrorsByID).filter(Boolean));
  }, [onErrorsChange, rowErrorsByID]);

  const updateConfig = useCallback((patch: Partial<BusinessDiagnosticsConfig>) => {
    onConfigChange({ ...config, ...patch });
  }, [config, onConfigChange]);

  const upsertSource = useCallback((kind: BusinessDataKind, patch: Partial<BusinessDataSource>) => {
    const current = config.sources.find((source) => source.kind === kind) || defaultBusinessSource(kind);
    const nextSource: BusinessDataSource = {
      ...current,
      ...patch,
      id: kind,
      kind,
    };
    const nextSources = sortBusinessSources([
      ...config.sources.filter((source) => source.kind !== kind),
      nextSource,
    ]);
    onConfigChange({ ...config, sources: nextSources });
  }, [config, onConfigChange]);

  const applySourceFile = useCallback((kind: BusinessDataKind, path: string, fallbackType: BusinessSourceType) => {
    const cleanPath = normalizeBusinessFilePath(path);
    if (!cleanPath) return;
    upsertSource(kind, {
      type: businessFileTypeFromPath(cleanPath, fallbackType),
      path: cleanPath,
      url: undefined,
      rows: undefined,
      format: undefined,
    });
    setRowErrorsByID((prev) => {
      const next = { ...prev };
      delete next[kind];
      return next;
    });
  }, [upsertSource]);

  const removeSource = useCallback((kind: BusinessDataKind) => {
    const nextSources = config.sources.filter((source) => source.kind !== kind);
    onConfigChange({ ...config, sources: nextSources });
    setRowErrorsByID((prev) => {
      const next = { ...prev };
      delete next[kind];
      return next;
    });
  }, [config, onConfigChange]);

  useEffect(() => onFilesDropped((paths, point) => {
    const path = normalizeBusinessFilePath(paths[0] || "");
    const kind = businessSourceKindAtPoint(point) || draggingKindRef.current;
    if (!path || !kind) return;
    const source = config.sources.find((item) => item.kind === kind) || defaultBusinessSource(kind);
    if (source.type !== "csv" && source.type !== "xlsx") return;
    applySourceFile(kind, path, source.type);
    draggingKindRef.current = null;
    setDraggingKind(null);
  }), [applySourceFile, config.sources]);

  const pickSourceFile = useCallback(async (kind: BusinessDataKind, sourceType: BusinessSourceType) => {
    try {
      const path = await app.PickAutomationSourceFile(sourceType);
      if (path) applySourceFile(kind, path, sourceType);
    } catch {
      // ignore cancelled or unavailable native dialog
    }
  }, [applySourceFile]);

  const setSourceType = useCallback((kind: BusinessDataKind, type: BusinessSourceType) => {
    const current = config.sources.find((source) => source.kind === kind) || defaultBusinessSource(kind);
    if (type === "http") {
      upsertSource(kind, { type, path: undefined, rows: undefined, url: current.url || "", format: current.format || "csv" });
    } else if (type === "mcp") {
      const rows = current.rows || [];
      setRowsTextByID((prev) => ({ ...prev, [kind]: prev[kind] ?? JSON.stringify(rows, null, 2) }));
      upsertSource(kind, { type, path: undefined, url: undefined, rows, format: undefined });
    } else {
      upsertSource(kind, { type, path: current.path || "", url: undefined, rows: undefined, format: undefined });
    }
    setRowErrorsByID((prev) => {
      const next = { ...prev };
      delete next[kind];
      return next;
    });
  }, [config.sources, upsertSource]);

  const runTime = businessRunTime(interval);
  const sourceTypeOptions: HeartbeatSelectOption[] = [
    { value: "csv", label: "CSV" },
    { value: "xlsx", label: "XLSX" },
    { value: "http", label: "HTTP" },
    { value: "mcp", label: "MCP rows" },
  ];
  const sourceFormatOptions: HeartbeatSelectOption[] = [
    { value: "csv", label: "CSV" },
    { value: "json", label: "JSON" },
    { value: "xlsx", label: "XLSX" },
  ];

  return (
    <section className="heartbeat-business">
      <div className="heartbeat-business__head">
        <div>
          <h3>周度经营诊断</h3>
          <p>上传真实经营数据后，在关联会话中读取数据、展开分析，并归档结果。</p>
        </div>
      </div>

      <div className="heartbeat-business__grid">
        <label>
          目标开始
          <input
            className="heartbeat-editor__input"
            type="date"
            value={localDateInputValue(config.targetStart)}
            onChange={(e) => updateConfig({ targetStart: localDateToISO(e.target.value) })}
          />
        </label>
        <label>
          目标结束
          <input
            className="heartbeat-editor__input"
            type="date"
            value={localDateInputValue(config.targetEnd)}
            onChange={(e) => updateConfig({ targetEnd: localDateToISO(e.target.value, true) })}
          />
        </label>
        <label>
          运行时间
          <input
            className="heartbeat-editor__input"
            type="time"
            value={runTime}
            onChange={(e) => onIntervalChange(`168h|weekly:mon@${e.target.value || "03:30"}`)}
          />
        </label>
        <label>
          业务区域
          <input
            className="heartbeat-editor__input"
            value={config.region || ""}
            onChange={(e) => updateConfig({ region: e.target.value })}
            placeholder="east"
          />
        </label>
      </div>

      <label className="heartbeat-business__label">
        报告收件人
        <input
          className="heartbeat-editor__input"
          value={(config.recipients || []).join(", ")}
          onChange={(e) => updateConfig({ recipients: parseBusinessRecipients(e.target.value) })}
          placeholder="ops@example.com, finance@example.com"
        />
      </label>

      <div className="heartbeat-business__sources">
        {BUSINESS_SOURCE_DEFS.map((def) => {
          const source = config.sources.find((item) => item.kind === def.kind);
          const active = !!source || def.required;
          const current = source || defaultBusinessSource(def.kind);
          if (!active) {
            return (
              <div className="heartbeat-business-source heartbeat-business-source--inactive" key={def.kind}>
                <div className="heartbeat-business-source__title">
                  <strong>{def.label}</strong>
                  <span>{def.fields}</span>
                </div>
                <button className="heartbeat-btn" type="button" onClick={() => upsertSource(def.kind, {})}>
                  添加
                </button>
              </div>
            );
          }
          return (
            <div className="heartbeat-business-source" key={def.kind}>
              <div className="heartbeat-business-source__title">
                <strong>{def.label}{def.required ? " *" : ""}</strong>
                <span>{def.fields}</span>
              </div>
              <div className="heartbeat-business-source__controls">
                <HeartbeatSelect
                  className="heartbeat-editor-select--source-type"
                  value={current.type}
                  options={sourceTypeOptions}
                  ariaLabel={def.label}
                  onChange={(type) => setSourceType(def.kind, type as BusinessSourceType)}
                />
                {current.type === "http" ? (
                  <>
                    <input
                      className="heartbeat-editor__input heartbeat-business-source__location"
                      value={current.url || ""}
                      onChange={(e) => upsertSource(def.kind, { url: e.target.value })}
                      placeholder="https://example.com/data.csv"
                    />
                    <HeartbeatSelect
                      className="heartbeat-editor-select--source-format"
                      value={current.format || "csv"}
                      options={sourceFormatOptions}
                      ariaLabel={def.label}
                      onChange={(format) => upsertSource(def.kind, { format })}
                    />
                  </>
                ) : current.type === "mcp" ? (
                  <textarea
                    className="heartbeat-editor__textarea heartbeat-business-source__rows"
                    value={rowsTextByID[current.id] ?? JSON.stringify(current.rows || [], null, 2)}
                    onChange={(e) => {
                      const text = e.target.value;
                      setRowsTextByID((prev) => ({ ...prev, [current.id]: text }));
                      const parsed = parseBusinessRows(text);
                      if (parsed.error) {
                        setRowErrorsByID((prev) => ({ ...prev, [current.id]: `${def.label}: ${parsed.error}` }));
                        return;
                      }
                      setRowErrorsByID((prev) => {
                        const next = { ...prev };
                        delete next[current.id];
                        return next;
                      });
                      upsertSource(def.kind, { rows: parsed.rows || [] });
                    }}
                    placeholder='[{"date":"2026-06-21","sku":"SKU-1","quantity":10,"amount":120,"region":"east"}]'
                  />
                ) : (
                  <div
                    className={`heartbeat-business-file${draggingKind === def.kind ? " heartbeat-business-file--drag" : ""}`}
                    data-business-source-kind={def.kind}
                    style={{ "--wails-drop-target": "drop" } as CSSProperties}
                    onDragEnter={(e) => {
                      e.preventDefault();
                      draggingKindRef.current = def.kind;
                      setDraggingKind(def.kind);
                    }}
                    onDragOver={(e) => {
                      e.preventDefault();
                      draggingKindRef.current = def.kind;
                      setDraggingKind(def.kind);
                    }}
                    onDragLeave={(e) => {
                      if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
                        draggingKindRef.current = null;
                        setDraggingKind(null);
                      }
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      draggingKindRef.current = null;
                      setDraggingKind(null);
                      const path = filePathFromDataTransfer(e.dataTransfer);
                      if (path) applySourceFile(def.kind, path, current.type);
                    }}
                  >
                    <button
                      className="heartbeat-business-file__pick"
                      type="button"
                      onClick={() => void pickSourceFile(def.kind, current.type)}
                    >
                      <Upload size={13} />
                      选择文件
                    </button>
                    <span className="heartbeat-business-file__name">
                      {current.path ? businessFileName(current.path) : "拖拽 CSV/XLSX 文件到这里"}
                    </span>
                    {current.path && (
                      <span className="heartbeat-business-file__path" title={current.path}>
                        {current.path}
                      </span>
                    )}
                  </div>
                )}
                {!def.required && (
                  <button className="heartbeat-card__open-btn" type="button" onClick={() => removeSource(def.kind)} title="移除数据源">
                    <X size={12} />
                  </button>
                )}
              </div>
              {rowErrorsByID[current.id] && <span className="heartbeat-business-source__error">{rowErrorsByID[current.id]}</span>}
            </div>
          );
        })}
      </div>
    </section>
  );
}

type HeartbeatSelectOption = {
  value: string;
  label: string;
  description?: string;
  visual?: {
    kind: "skill" | "expert";
    src?: string;
    fallback: string;
  };
};

function automationExpertRole(expert: ExpertInfo): string {
  return [expert.name, expert.role, expert.description].map((value) => value?.trim() || "").filter(Boolean).join("\n");
}

function automationPickerFallback(label: string): string {
  return label.trim().charAt(0).toUpperCase() || "?";
}

function HeartbeatSelectVisual({ visual }: { visual?: HeartbeatSelectOption["visual"] }) {
  if (!visual) return null;
  return (
    <span
      className={`heartbeat-editor-select__visual heartbeat-editor-select__visual--${visual.kind}`}
      aria-hidden="true"
    >
      <span className="heartbeat-editor-select__visual-fallback">{visual.fallback}</span>
      {visual.src && (
        <img
          src={visual.src}
          alt=""
          loading="lazy"
          decoding="async"
          onError={(event) => { event.currentTarget.remove(); }}
        />
      )}
    </span>
  );
}

function HeartbeatSelect({
  value,
  options,
  ariaLabel,
  describedBy,
  controlRef,
  invalid,
  busy,
  className,
  menuMinWidth,
  onChange,
}: {
  value: string;
  options: HeartbeatSelectOption[];
  ariaLabel: string;
  describedBy?: string;
  controlRef?: RefObject<HTMLButtonElement | null>;
  invalid?: boolean;
  busy?: boolean;
  className?: string;
  menuMinWidth?: number;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState<number>();
  const [activeIndex, setActiveIndex] = useState(-1);
  const localTriggerRef = useRef<HTMLButtonElement>(null);
  const triggerRef = controlRef ?? localTriggerRef;
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const selected = options.find((option) => option.value === value) ?? options[0];
  const selectedIndex = options.findIndex((option) => option.value === selected?.value);

  useEffect(() => {
    if (!open || options.length === 0) return;
    const nextIndex = activeIndex >= 0 && activeIndex < options.length
      ? activeIndex
      : Math.max(0, selectedIndex);
    if (nextIndex !== activeIndex) {
      setActiveIndex(nextIndex);
      return;
    }
    optionRefs.current[nextIndex]?.focus();
  }, [activeIndex, open, options.length, selectedIndex]);

  const openMenu = (trigger: HTMLButtonElement) => {
    setMenuWidth(trigger.getBoundingClientRect().width);
    setActiveIndex(Math.max(0, selectedIndex));
    setOpen(true);
  };

  const moveActive = (nextIndex: number) => {
    if (options.length === 0) return;
    const wrappedIndex = (nextIndex + options.length) % options.length;
    setActiveIndex(wrappedIndex);
  };

  const pick = (next: string) => {
    onChange(next);
    setOpen(false);
    triggerRef.current?.focus();
  };

  const closeFromKeyboard = () => {
    setOpen(false);
    triggerRef.current?.focus();
  };

  const handleTriggerKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!open) {
        openMenu(event.currentTarget);
        return;
      }
      moveActive(activeIndex + (event.key === "ArrowDown" ? 1 : -1));
      return;
    }
    if (event.key === "Home" && open) {
      event.preventDefault();
      moveActive(0);
      return;
    }
    if (event.key === "End" && open) {
      event.preventDefault();
      moveActive(options.length - 1);
      return;
    }
    if (event.key === "Escape" && open) {
      event.preventDefault();
      closeFromKeyboard();
    }
  };

  const handleOptionKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, index: number) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      moveActive(index + (event.key === "ArrowDown" ? 1 : -1));
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      moveActive(event.key === "Home" ? 0 : options.length - 1);
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      event.stopPropagation();
      pick(options[index].value);
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      closeFromKeyboard();
    }
  };

  return (
    <div className={`heartbeat-editor-select${className ? ` ${className}` : ""}`}>
      <button
        ref={triggerRef}
        type="button"
        className="heartbeat-editor-select__trigger"
        aria-label={ariaLabel}
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        aria-busy={busy || undefined}
        aria-haspopup="listbox"
        aria-expanded={open}
        title={selected?.description || selected?.label}
        onClick={(event) => {
          if (open) {
            setOpen(false);
            return;
          }
          openMenu(event.currentTarget);
        }}
        onKeyDown={handleTriggerKeyDown}
      >
        <span className="heartbeat-editor-select__value">
          <HeartbeatSelectVisual visual={selected?.visual} />
          <span className="heartbeat-editor-select__label">{selected?.label || ""}</span>
        </span>
        <ChevronsUpDown size={12} />
      </button>
      <AnchoredPopover
        open={open}
        anchorRef={triggerRef}
        onClose={() => setOpen(false)}
        className="heartbeat-editor-select__menu"
        placement="bottom"
        style={{ width: menuWidth, minWidth: menuMinWidth, maxWidth: "min(90vw, 420px)" }}
      >
        <div className="heartbeat-editor-select__list" role="listbox" aria-label={ariaLabel}>
          {options.map((option, index) => (
            <button
              key={option.value || "__empty"}
              ref={(element) => { optionRefs.current[index] = element; }}
              type="button"
              role="option"
              aria-selected={option.value === value}
              tabIndex={index === activeIndex ? 0 : -1}
              className={`heartbeat-editor-select__option${option.value === value ? " heartbeat-editor-select__option--selected" : ""}`}
              title={option.description || option.label}
              onFocus={() => setActiveIndex(index)}
              onKeyDown={(event) => handleOptionKeyDown(event, index)}
              onClick={() => pick(option.value)}
            >
              <span className="heartbeat-editor-select__option-content">
                <HeartbeatSelectVisual visual={option.visual} />
                <span className="heartbeat-editor-select__label">{option.label}</span>
              </span>
              {option.value === value && <Check size={13} className="heartbeat-editor-select__check" />}
            </button>
          ))}
        </div>
      </AnchoredPopover>
    </div>
  );
}

type HeartbeatMultiSelectOption = {
  value: string;
  label: string;
  description?: string;
  disabled?: boolean;
};

function HeartbeatMultiSelect({
  value,
  options,
  ariaLabel,
  placeholder,
  emptyOptionsLabel,
  onChange,
}: {
  value: string[];
  options: HeartbeatMultiSelectOption[];
  ariaLabel: string;
  placeholder: string;
  emptyOptionsLabel: string;
  onChange: (value: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState<number>();
  const [activeIndex, setActiveIndex] = useState(-1);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const optionRefs = useRef<Array<HTMLInputElement | null>>([]);
  const selectedOptions = value.flatMap((selectedValue) => {
    const option = options.find((candidate) => candidate.value === selectedValue);
    return option ? [option] : [];
  });
  const optionFocusKey = options.map((option) => `${option.value}:${option.disabled ? "disabled" : "enabled"}`).join("\u0000");
  const selectedValueKey = value.join("\u0000");

  const firstEnabledIndex = () => options.findIndex((option) => !option.disabled);
  const selectedEnabledIndex = () => options.findIndex((option) => value.includes(option.value) && !option.disabled);

  useEffect(() => {
    if (!open || options.length === 0) return;
    const nextIndex = activeIndex >= 0 && activeIndex < options.length && !options[activeIndex].disabled
      ? activeIndex
      : Math.max(0, selectedEnabledIndex() >= 0 ? selectedEnabledIndex() : firstEnabledIndex());
    if (options[nextIndex]?.disabled) return;
    if (nextIndex !== activeIndex) {
      setActiveIndex(nextIndex);
      return;
    }
    optionRefs.current[nextIndex]?.focus();
  }, [activeIndex, open, optionFocusKey, selectedValueKey]);

  const openMenu = (trigger: HTMLButtonElement) => {
    setMenuWidth(trigger.getBoundingClientRect().width);
    const selectedIndex = selectedEnabledIndex();
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : firstEnabledIndex());
    setOpen(true);
  };

  const moveActive = (startIndex: number, direction: 1 | -1) => {
    if (options.length === 0) return;
    for (let offset = 0; offset < options.length; offset += 1) {
      const index = (startIndex + direction * offset + options.length) % options.length;
      if (!options[index].disabled) {
        setActiveIndex(index);
        return;
      }
    }
  };

  const closeFromKeyboard = () => {
    setOpen(false);
    triggerRef.current?.focus();
  };

  const handleTriggerKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!open) openMenu(event.currentTarget);
      return;
    }
    if (event.key === "Escape" && open) {
      event.preventDefault();
      closeFromKeyboard();
    }
  };

  const handleOptionKeyDown = (event: React.KeyboardEvent<HTMLInputElement>, index: number) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      moveActive(index + (event.key === "ArrowDown" ? 1 : -1), event.key === "ArrowDown" ? 1 : -1);
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      moveActive(event.key === "Home" ? 0 : options.length - 1, event.key === "Home" ? 1 : -1);
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      closeFromKeyboard();
    }
  };

  const toggle = (optionValue: string, checked: boolean) => {
    onChange(checked
      ? [...value.filter((current) => current !== optionValue), optionValue]
      : value.filter((current) => current !== optionValue));
  };

  return (
    <div className="heartbeat-editor-select heartbeat-editor-multiselect">
      <button
        ref={triggerRef}
        type="button"
        className="heartbeat-editor-select__trigger heartbeat-editor-multiselect__trigger"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={(event) => {
          if (open) {
            setOpen(false);
            return;
          }
          openMenu(event.currentTarget);
        }}
        onKeyDown={handleTriggerKeyDown}
      >
        <span className="heartbeat-editor-multiselect__values">
          {selectedOptions.length === 0 ? (
            <span className="heartbeat-editor-multiselect__placeholder">{placeholder}</span>
          ) : selectedOptions.map((option) => (
            <span key={option.value} className="heartbeat-editor-multiselect__tag" title={option.label}>
              <span>{option.label}</span>
            </span>
          ))}
        </span>
        <ChevronsUpDown size={12} aria-hidden="true" />
      </button>
      <AnchoredPopover
        open={open}
        anchorRef={triggerRef}
        onClose={() => setOpen(false)}
        className="heartbeat-editor-select__menu heartbeat-editor-multiselect__menu"
        placement="bottom"
        style={{ width: menuWidth, maxWidth: "min(90vw, 420px)" }}
        contentKey={value.join("\u0000")}
      >
        <div className="heartbeat-editor-select__list" role="listbox" aria-label={ariaLabel} aria-multiselectable="true">
          {options.length === 0 ? (
            <span className="heartbeat-editor-multiselect__empty">{emptyOptionsLabel}</span>
          ) : options.map((option, index) => {
            const selected = value.includes(option.value);
            return (
              <label
                key={option.value}
                role="option"
                aria-selected={selected}
                className={`heartbeat-editor-select__option heartbeat-editor-multiselect__option${selected ? " heartbeat-editor-select__option--selected" : ""}${option.disabled ? " heartbeat-editor-multiselect__option--disabled" : ""}`}
                title={option.description || option.label}
              >
                <input
                  ref={(element) => { optionRefs.current[index] = element; }}
                  type="checkbox"
                  checked={selected}
                  disabled={option.disabled}
                  tabIndex={index === activeIndex ? 0 : -1}
                  onFocus={() => setActiveIndex(index)}
                  onKeyDown={(event) => handleOptionKeyDown(event, index)}
                  onChange={(event) => toggle(option.value, event.target.checked)}
                />
                <span className="heartbeat-editor-select__option-content heartbeat-editor-multiselect__option-content">
                  <span className="heartbeat-editor-select__label">{option.label}</span>
                  {option.description ? <small>{option.description}</small> : null}
                </span>
              </label>
            );
          })}
        </div>
      </AnchoredPopover>
    </div>
  );
}

function TaskEditor({
  task,
  savePhase,
  saveError,
  capabilityRiskSummary,
  scheduleCatalog,
  deliveryTargetCatalog,
	cloudProjects,
	projectLoadState,
	onRetryProjects,
	authenticated,
  modelCatalog,
  modelAccountScopeKey,
  modelSettingsRevision,
  subscriptionAccess,
  onConfigureModel,
  onClearModelError,
  fullPage = false,
  onSave,
  onCancel,
  onDelete,
}: {
  task: HeartbeatTask;
  savePhase: "" | "task" | "template";
  saveError: string;
  capabilityRiskSummary: string;
  scheduleCatalog: AutomationScheduleCatalogEntry[];
  deliveryTargetCatalog: AutomationDeliveryTargetCatalogEntry[];
	cloudProjects: AutomationCloudProjectChoice[];
	projectLoadState: AutomationResourceLoadState;
	onRetryProjects: () => Promise<void>;
	authenticated: boolean;
  modelCatalog: SharedModelCatalogState;
  modelAccountScopeKey: string;
  modelSettingsRevision: number;
  subscriptionAccess: SubscriptionAccess;
  onConfigureModel?: () => void;
  onClearModelError: () => void;
  fullPage?: boolean;
  onSave: (t: HeartbeatTask, options?: { saveAsTemplate?: boolean }) => Promise<HeartbeatTask | undefined>;
  onCancel: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  const titleRef = useRef<HTMLInputElement>(null);
  const modelRef = useRef<HTMLButtonElement>(null);
  const businessValidationRef = useRef<HTMLDivElement>(null);
  const validationFocusRef = useRef<HTMLElement | null>(null);
  const [submitAttempt, setSubmitAttempt] = useState(0);
  const [workspaces, setWorkspaces] = useState<WorkspaceView[]>([]);
  const modelStatusId = useId();
  const [skills, setSkills] = useState<SkillView[]>([]);
  const [experts, setExperts] = useState<ExpertInfo[]>([]);
	const [connectors, setConnectors] = useState<Array<{ id: string; name: string; available: boolean; unavailableReason?: string }>>([]);
  const [projectOpen, setProjectOpen] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [noticeDismissed, setNoticeDismissed] = useState(false);
  const projectRef = useRef<HTMLDivElement>(null);
	const [effectiveRangeOpen, setEffectiveRangeOpen] = useState(false);
	const [effectiveRangeMenuWidth, setEffectiveRangeMenuWidth] = useState<number>();
	const effectiveRangeTriggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    titleRef.current?.focus();
    app.ListWorkspaces().then((list) => setWorkspaces(list ?? [])).catch(() => {});
    app.SkillsSettings().then((view) => setSkills(Array.isArray(view?.skills) ? view.skills : [])).catch(() => {});
		Promise.all([
			app.ListExperts().catch(() => []),
			typeof app.ListUserExperts === "function" ? app.ListUserExperts().catch(() => []) : Promise.resolve([]),
		]).then(([catalog, userCatalog]) => {
			const byID = new Map<string, ExpertInfo>();
			for (const expert of Array.isArray(catalog) ? catalog : []) byID.set(expert.id, expert);
			for (const userExpert of Array.isArray(userCatalog) ? userCatalog : []) byID.set(userExpert.id, automationUserExpertInfo(userExpert));
			setExperts([...byID.values()]);
		}).catch(() => setExperts([]));
		app.AutomationListCapabilities().then((catalog) => {
			const value = catalog as { connectors?: Array<{ id: string; name: string; available: boolean; unavailableReason?: string }> };
			setConnectors(Array.isArray(value?.connectors) ? value.connectors : []);
		}).catch(() => setConnectors([]));
  }, []);

  useEffect(() => {
    if (!projectOpen) return;
    const close = (e: MouseEvent) => {
      if (projectRef.current && !projectRef.current.contains(e.target as Node)) {
        setProjectOpen(false);
      }
    };
    document.addEventListener("click", close);
    return () => document.removeEventListener("click", close);
  }, [projectOpen]);

  const [draft, setDraft] = useState(task);
  useEffect(() => {
    setDraft((previous) => ({ ...previous, revision: task.revision, createdAt: task.createdAt }));
  }, [task.revision, task.createdAt]);
  const [saveAsTemplate, setSaveAsTemplate] = useState(false);
  const [businessEditorErrors, setBusinessEditorErrors] = useState<string[]>([]);
  const intervalBeforeCycle = useRef<string | null>(null);
  const promptRef = useRef<HTMLTextAreaElement>(null);

  const modelRoot = draft.scope === "project" ? draft.workspaceRoot || "" : "";
  const modelScopeKey = JSON.stringify([
    modelAccountScopeKey, draft.scope, modelRoot, draft.projectId || "",
    authenticated, subscriptionAccess.state,
  ]);
  const modelSaveError = saveError === automationModelUnavailableMessage;
  useEffect(() => { onClearModelError(); }, [modelScopeKey, onClearModelError]);
  const currentModels = useAutomationModels(
    modelRoot, modelScopeKey, modelSettingsRevision, savePhase !== "", modelSaveError,
  );
  const { models, loading: modelsLoading, initialLoading: modelsInitialLoading } = currentModels;

  const availableModels = useMemo(() => filterModelInfosForSubscriptionAccess(
    models,
    modelCatalog.state === "ready" ? modelCatalog.marketplace : undefined,
    subscriptionAccess,
  ), [modelCatalog, models, subscriptionAccess]);

  // Auto-grow prompt textarea: shrink-to-fit then cap at 180px
  const autoGrowPrompt = useCallback(() => {
    const el = promptRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 180) + "px";
  }, []);

  useLayoutEffect(() => {
    autoGrowPrompt();
  }, [draft.prompt, autoGrowPrompt]);

  const set = useCallback((field: keyof HeartbeatTask, value: string | boolean) => {
    setDraft((prev) => ({ ...prev, [field]: value }));
  }, []);

  // Detect frequency type from interval value
  const [freqType, setFreqType] = useState<"cycle" | "interval" | "once">(
		task.interval?.startsWith("once@") ? "once" : (task.interval && task.interval.includes("|")) ? "cycle" : "interval"
  );

  const isNew = !task.createdAt;
  const selectedWorkspace = draft.scope === "project" && !draft.projectId && draft.workspaceRoot
    ? workspaces.find((w) => w.path === draft.workspaceRoot)
    : null;
	const selectedCloudProject = draft.scope === "project" && draft.projectId
		? cloudProjects.find((project) => project.projectId === draft.projectId)
		: null;
  const selectedWorkspaceLabel = draft.scope === "project"
		? selectedCloudProject?.name || selectedWorkspace?.name || draft.projectId || draft.workspaceRoot?.split(/[\\/]/).pop() || t("heartbeat.scopeProject")
    : t("heartbeat.scopeGlobal");
  const availableSkills = skills.filter((skill) => skill.enabled);
  const selectedSkills = draft.skills ?? [];
  const selectedSkill = selectedSkills[0] || "";
  const availableSkillNames = new Set(availableSkills.map((skill) => skill.name));
  const selectedSkillInList = !selectedSkill || availableSkillNames.has(selectedSkill);
  const expertOptions = experts.filter((expert) => expert.enabled);
  const expertSelectionKnown = !draft.expertId || expertOptions.some((expert) => expert.id === draft.expertId);
  const modelRouteValue = (model: ModelInfo) => encodeAutomationModelRoute(
    model.ref,
    model.billingSource as HeartbeatTask["billingSource"],
  );
  const modelMatches = models.filter((model) => draft.model
    ? model.ref === draft.model && (!draft.billingSource || model.billingSource === draft.billingSource)
    : model.current);
  const selectedModel = modelMatches.length === 1 ? modelMatches[0] : undefined;
  const modelAccess = resolveModelRouteAccess(selectedModel,
    modelCatalog.state === "ready" ? modelCatalog.marketplace : undefined,
    subscriptionAccess, modelsInitialLoading || (authenticated && modelCatalog.state === "loading"), true);
  const selectedModelValue = !draft.model ? "" : selectedModel
    ? modelRouteValue(selectedModel)
    : encodeAutomationModelRoute(draft.model || "", draft.billingSource);
  const modelInList = !modelsLoading && !currentModels?.error && modelAccess.state === "allowed";
  const modelStatus = modelSaveError ? automationModelUnavailableMessage
    : currentModels?.error || (modelsInitialLoading || modelAccess.state === "loading"
      ? "正在加载模型…" : modelAccess.state !== "allowed" ? automationModelUnavailableMessage : "");
  const skillSelectOptions: HeartbeatSelectOption[] = [
    { value: "", label: fullPage ? "技能" : "不使用技能" },
    ...(!selectedSkillInList && selectedSkill ? [{
      value: selectedSkill,
      label: selectedSkill,
      visual: { kind: "skill" as const, fallback: automationPickerFallback(selectedSkill) },
    }] : []),
    ...availableSkills.map((skill) => ({
      value: skill.name,
      label: skill.name,
      description: skill.description,
      visual: {
        kind: "skill" as const,
        src: skill.iconUrl?.trim() || undefined,
        fallback: automationPickerFallback(skill.displayName || skill.name),
      },
    })),
  ];
  const expertSelectOptions: HeartbeatSelectOption[] = [
    { value: "", label: fullPage ? "召唤专家" : "不使用专家" },
    ...(!expertSelectionKnown && draft.expertId ? [{
      value: draft.expertId,
      label: draft.role?.split(/\r?\n/)[0] || draft.expertId,
      visual: {
        kind: "expert" as const,
        fallback: automationPickerFallback(draft.role?.split(/\r?\n/)[0] || draft.expertId),
      },
    }] : []),
    ...expertOptions.map((expert) => ({
      value: expert.id,
      label: expert.category ? `${expert.name} · ${expert.category}` : expert.name,
      description: expert.description,
      visual: {
        kind: "expert" as const,
        src: expertAvatarSrcForInfo(expert),
        fallback: automationPickerFallback(expert.name),
      },
    })),
  ];
  const modelSelectOptions: HeartbeatSelectOption[] = [
    { value: "", label: t("heartbeat.modelPlaceholder") },
    ...(!availableModels.some((model) => modelRouteValue(model) === selectedModelValue) && draft.model
      ? [{ value: selectedModelValue, label: modelsInitialLoading ? "正在加载模型…" : "模型不可用" }] : []),
    ...availableModels.map((model) => ({
      value: modelRouteValue(model),
      label: `${model.displayName || model.model} · ${modelProviderLabel(model.provider, {
        deepseek: t("settings.providerLabel.deepseek"), product: t("settings.providerLabel.kwu"),
      })}`,
    })),
  ];
  const intervalUnitOptions: HeartbeatSelectOption[] = [
    { value: "m", label: t("heartbeat.unitMin") },
    { value: "h", label: t("heartbeat.unitHour") },
  ];
  const approvalSelectOptions: HeartbeatSelectOption[] = [
    { value: "ask", label: t("heartbeat.approvalModeAsk") },
    { value: "auto", label: t("heartbeat.approvalModeAuto") },
    { value: "yolo", label: "完全访问权限" },
  ];
  const businessErrors = draft.businessDiagnostics
    ? [...businessDiagnosticsValidationErrors(draft.businessDiagnostics), ...businessEditorErrors]
    : [];
	const selectedDeliveryKeys = new Set((draft.deliveryTargets || []).map(deliveryTargetKey));
	const connectorOptions = useMemo(() => {
		const options = [...connectors];
		const catalogIDs = new Set(options.map((connector) => connector.id));
		for (const id of draft.connectorIds || []) {
			if (catalogIDs.has(id)) continue;
			options.push({ id, name: id, available: false, unavailableReason: "连接器当前不可用" });
		}
		return options;
	}, [connectors, draft.connectorIds]);
	const deliveryTargetOptions = useMemo(() => {
		const options = [...deliveryTargetCatalog];
		const catalogKeys = new Set(options.map(deliveryTargetKey));
		for (const saved of draft.deliveryTargets || []) {
			const savedKey = deliveryTargetKey(saved);
			if (catalogKeys.has(savedKey)) continue;
			catalogKeys.add(savedKey);
			options.push({
				...saved,
				label: "已保存目标",
				provider: "",
				available: false,
				unavailableReason: "目标已从目录移除或授权已撤销",
				scope: "已保存",
			});
		}
		return options;
	}, [deliveryTargetCatalog, draft.deliveryTargets]);
	const deliverySelectionInvalid = !!draft.deliveryEnabled && selectedDeliveryKeys.size === 0;
  const setSkillSelection = useCallback((name: string) => {
    setDraft((prev) => ({ ...prev, skills: name ? [name] : undefined }));
  }, []);
  const saving = savePhase !== "";
  const saveStatus = savePhase === "template" ? "任务已保存，正在保存模板…" : saving ? "正在保存…" : "";
  const titleInvalid = submitAttempt > 0 && !draft.title.trim();
  const promptInvalid = submitAttempt > 0 && !draft.prompt.trim();
  const modelInvalid = modelSaveError || (submitAttempt > 0
    && Boolean(currentModels.error || modelAccess.state === "unavailable"));
  // Re-focus after each rejected submission, once its inline messages are rendered.
  useLayoutEffect(() => {
    if (!submitAttempt) return;
    const target = validationFocusRef.current;
    target?.focus({ preventScroll: true });
    target?.scrollIntoView({ block: "center", inline: "nearest" });
  }, [submitAttempt]);
  const saveDraft = () => {
    if (saving) return;
    if (!draft.title.trim() || !draft.prompt.trim() || !modelInList || !selectedModel || businessErrors.length > 0) {
      validationFocusRef.current = !draft.title.trim() ? titleRef.current
        : !draft.prompt.trim() ? promptRef.current
        : !modelInList || !selectedModel ? modelRef.current
        : businessValidationRef.current;
      setSubmitAttempt((attempt) => attempt + 1);
      return;
    }
    void onSave({ ...draft, model: selectedModel.ref, billingSource: selectedModel.billingSource }, { saveAsTemplate });
  };
  const pickModel = (value: string) => {
    onClearModelError();
    setDraft((previous) => ({ ...previous, ...decodeAutomationModelRoute(value) }));
  };
  const modelFeedback = modelStatus && (
    <div className={`heartbeat-editor__optional${modelInvalid ? " heartbeat-editor__field-error" : ""}`} id={modelStatusId} role={modelInvalid ? "alert" : "status"}>
      {modelInvalid && <AlertTriangle size={13} aria-hidden="true" />}
      <span>{modelStatus}</span>
      {currentModels?.error ? <button type="button" className="heartbeat-toolbar__btn" disabled={saving} onClick={() => {
        onClearModelError(); currentModels.refresh();
      }}>重试</button> : modelAccess.state === "unavailable" || modelSaveError ? onConfigureModel && (
        <button type="button" className="heartbeat-toolbar__btn" disabled={saving} onClick={() => {
          onClearModelError(); onConfigureModel();
        }}>模型设置</button>
      ) : null}
    </div>
  );
  const formatEffectiveDate = (value?: string) => value
    ? value.slice(0, 16).replace("T", " ").replace(/-/g, "/")
    : "";
  const effectiveFromLabel = formatEffectiveDate(draft.effectiveFrom);
  const effectiveUntilLabel = formatEffectiveDate(draft.effectiveUntil);
  const effectiveRangeLabel = effectiveFromLabel && effectiveUntilLabel
    ? `${effectiveFromLabel} 至 ${effectiveUntilLabel}`
    : effectiveFromLabel
      ? `${effectiveFromLabel} 起`
      : effectiveUntilLabel
        ? `截至 ${effectiveUntilLabel}`
        : "长期有效";
  return (
    <div className={`heartbeat-editor${fullPage ? " heartbeat-editor--full-page" : ""}`}>
      {fullPage && (
        <div className="heartbeat-editor__topbar">
          <div className="heartbeat-editor__breadcrumb" aria-label="自动化任务位置">
            <Clock size={17} aria-hidden="true" />
            <button type="button" onClick={onCancel}>自动化</button>
            <span aria-hidden="true">/</span>
            <h1>{isNew ? "添加自动化任务" : "编辑自动化任务"}</h1>
          </div>
          <div className="heartbeat-editor__topbar-actions">
            <button type="button" className="heartbeat-btn heartbeat-editor__topbar-cancel" onClick={onCancel}>
              {t("common.cancel")}
            </button>
            <button
              type="button"
              className="heartbeat-btn heartbeat-btn--primary heartbeat-editor__topbar-save"
              onClick={saveDraft}
              disabled={saving}
            >
              {saving ? "正在保存…" : "保存"}
            </button>
          </div>
        </div>
      )}
		{!noticeDismissed && <div className="heartbeat-editor__online-notice" role="note">
			<strong>提示</strong>
			<span>自动化任务执行时，请勿关闭电脑或退出客户端，否则任务将无法正常执行。</span>
        {fullPage && (
          <button type="button" className="heartbeat-editor__online-notice-close" onClick={() => setNoticeDismissed(true)} aria-label="关闭提示">
            <X size={15} aria-hidden="true" />
          </button>
        )}
		</div>}
      <div className="heartbeat-editor__fields" inert={saving}>
        {/* Title */}
        <div className="heartbeat-editor__field heartbeat-editor__field--title">
		<label htmlFor={`${modelStatusId}-title`}>名称</label>
        <input
          ref={titleRef}
          id={`${modelStatusId}-title`}
          className="heartbeat-editor__input"
          aria-required="true"
          aria-invalid={titleInvalid || undefined}
          aria-describedby={titleInvalid ? `${modelStatusId}-title-error` : undefined}
          value={draft.title}
          onChange={(e) => set("title", e.target.value)}
          placeholder={t("heartbeat.titlePlaceholder")}
        />
        {titleInvalid && <div className="heartbeat-editor__field-error" id={`${modelStatusId}-title-error`} role="alert">
          <AlertTriangle size={13} aria-hidden="true" /><span>请输入任务名称。</span>
        </div>}
      </div>

      {/* Scope */}
      <div className="heartbeat-editor__field heartbeat-editor__field--workspace">
		<label>工作空间 <span className="heartbeat-editor__optional">{t("heartbeat.optional")}</span></label>
        <div className="heartbeat-editor__scope-row" ref={projectRef}>
          <button
            type="button"
            className="heartbeat-scope-btn heartbeat-scope-btn--active"
            aria-haspopup="listbox"
            aria-expanded={projectOpen}
            onClick={() => setProjectOpen((value) => !value)}
          >
            {selectedWorkspaceLabel}
            <ChevronsUpDown size={12} aria-hidden="true" />
          </button>
          {projectOpen && (
            <div className="heartbeat-project-menu" role="listbox" aria-label="工作空间">
              <button
                type="button"
                role="option"
                aria-selected={draft.scope !== "project"}
                className={`heartbeat-project-menu__item${draft.scope !== "project" ? " heartbeat-project-menu__item--active" : ""}`}
                onClick={() => {
                  setDraft((prev) => ({ ...prev, scope: "global", projectId: undefined, workspaceRoot: "", topicId: undefined }));
                  setProjectOpen(false);
                }}
              >
                {t("heartbeat.scopeGlobal")}
                {draft.scope !== "project" && <Check size={12} aria-hidden="true" />}
              </button>
				{!authenticated ? (
					<div className="heartbeat-project-menu__empty">{t("workspace.cloudProjects.signIn")}</div>
				) : projectLoadState.error ? (
					<button
						type="button"
						className="heartbeat-project-menu__item"
						disabled={projectLoadState.loading}
						onClick={() => void onRetryProjects()}
					>
						<RefreshCw size={12} aria-hidden="true" />
						云项目加载失败，点击重试
					</button>
				) : projectLoadState.loading && cloudProjects.length === 0 ? (
					<div className="heartbeat-project-menu__empty" role="status">正在加载云项目…</div>
				) : workspaces.length === 0 && cloudProjects.length === 0 ? (
                <div className="heartbeat-project-menu__empty">{t("heartbeat.noProjects")}</div>
				) : null}
				{workspaces.map((ws) => (
                <button
                  key={`local:${ws.path}`}
                  type="button"
                  role="option"
                  aria-selected={draft.scope === "project" && !draft.projectId && draft.workspaceRoot === ws.path}
                  className={`heartbeat-project-menu__item${draft.scope === "project" && !draft.projectId && draft.workspaceRoot === ws.path ? " heartbeat-project-menu__item--active" : ""}`}
                  onClick={() => {
                    setDraft((prev) => ({ ...prev, scope: "project", projectId: undefined, workspaceRoot: ws.path, topicId: undefined }));
                    setProjectOpen(false);
                  }}
                >
                  {ws.name}
                  {draft.scope === "project" && !draft.projectId && draft.workspaceRoot === ws.path
                    ? <Check size={12} aria-hidden="true" />
                    : ws.current && <span className="heartbeat-project-menu__current">{t("heartbeat.currentWorkspace")}</span>}
                </button>
              ))}
							{cloudProjects.map((project) => (
								<button
									key={`cloud:${project.projectId}`}
									type="button"
									role="option"
									aria-selected={draft.scope === "project" && draft.projectId === project.projectId}
									className={`heartbeat-project-menu__item${draft.scope === "project" && draft.projectId === project.projectId ? " heartbeat-project-menu__item--active" : ""}`}
									onClick={() => {
										setDraft((prev) => ({ ...prev, scope: "project", projectId: project.projectId, workspaceRoot: "", topicId: undefined }));
										setProjectOpen(false);
									}}
								>
									{project.name}
									{draft.scope === "project" && draft.projectId === project.projectId
										? <Check size={12} aria-hidden="true" />
										: <span className="heartbeat-project-menu__current">云项目</span>}
								</button>
							))}
            </div>
          )}
        </div>
      </div>

      {/* Prompt */}
      <div className="heartbeat-editor__field heartbeat-editor__field--prompt">
        <label htmlFor={`${modelStatusId}-prompt`}>{t("heartbeat.fieldPrompt")}</label>
        <div className="heartbeat-editor__prompt-box">
          <textarea
            ref={promptRef}
            id={`${modelStatusId}-prompt`}
            className="heartbeat-editor__textarea heartbeat-editor__textarea--prompt"
            aria-required="true"
            aria-invalid={promptInvalid || undefined}
            aria-describedby={promptInvalid ? `${modelStatusId}-prompt-error` : undefined}
            value={draft.prompt}
            onChange={(e) => {
              set("prompt", e.target.value);
              // autoGrowPrompt is called via useEffect watching draft.prompt
            }}
            placeholder={t("heartbeat.promptPlaceholder")}
          />
          <div className="heartbeat-editor__prompt-tools">
            {fullPage && (
              <div className="heartbeat-editor__tool-block heartbeat-editor__tool-block--model">
                <Activity size={17} aria-hidden="true" />
                <HeartbeatSelect
                  menuMinWidth={320}
                  value={selectedModelValue}
                  options={modelSelectOptions}
                  ariaLabel={t("heartbeat.fieldModel")}
                  describedBy={modelStatus ? modelStatusId : undefined}
                  controlRef={modelRef}
                  invalid={modelInvalid}
                  busy={modelsLoading}
                  onChange={pickModel}
                />
              </div>
            )}
            <div className="heartbeat-editor__tool-block">
              {fullPage && <Hammer size={17} aria-hidden="true" />}
              {!fullPage && <span className="heartbeat-editor__tool-label">技能</span>}
              <HeartbeatSelect
                menuMinWidth={260}
                value={selectedSkill}
                options={skillSelectOptions}
                ariaLabel="技能"
                onChange={setSkillSelection}
              />
            </div>
            <div className="heartbeat-editor__tool-block heartbeat-editor__tool-block--expert">
              {fullPage && <GraduationCap size={17} aria-hidden="true" />}
              {!fullPage && <span className="heartbeat-editor__tool-label">专家</span>}
              <HeartbeatSelect
                menuMinWidth={260}
                value={draft.expertId || ""}
                options={expertSelectOptions}
                ariaLabel="专家"
                onChange={(expertId) => {
					const expert = expertOptions.find((option) => option.id === expertId);
					setDraft((prev) => ({
						...prev,
						expertId: expertId || undefined,
						role: expert ? automationExpertRole(expert) : undefined,
					}));
				}}
              />
            </div>
            {fullPage && (
              <div className="heartbeat-editor__tool-block heartbeat-editor__tool-block--approval">
                <AlertTriangle size={17} aria-hidden="true" />
                <HeartbeatSelect
                  menuMinWidth={180}
                  value={normalizeMode(draft.approvalMode)}
                  options={approvalSelectOptions}
                  ariaLabel={t("heartbeat.fieldApprovalMode")}
                  onChange={(value) => setDraft((previous) => ({
                    ...previous,
                    approvalMode: value as HeartbeatTask["approvalMode"],
                  }))}
                />
              </div>
            )}
          </div>
        </div>
        {promptInvalid && <div className="heartbeat-editor__field-error" id={`${modelStatusId}-prompt-error`} role="alert">
          <AlertTriangle size={13} aria-hidden="true" /><span>请输入提示词。</span>
        </div>}
        {fullPage && modelFeedback}
      </div>

      {!fullPage && <div className="heartbeat-editor__field">
        <label>{t("heartbeat.fieldModel")}</label>
        <HeartbeatSelect
          value={selectedModelValue}
          options={modelSelectOptions}
          ariaLabel={t("heartbeat.fieldModel")}
          describedBy={modelStatus ? modelStatusId : undefined}
          controlRef={modelRef}
          invalid={modelInvalid}
          busy={modelsLoading}
          onChange={pickModel}
        />
        {modelFeedback}
      </div>}

      <div className="heartbeat-editor__field heartbeat-editor__field--failure-cooldown">
        <label>{t("heartbeat.fieldFailureCooldown")} <span className="heartbeat-editor__optional">{t("heartbeat.optional")}</span></label>
        <input
          className="heartbeat-editor__input"
          value={draft.failureCooldown || ""}
          onChange={(e) => set("failureCooldown", e.target.value)}
          placeholder="30m"
        />
      </div>

		<div className="heartbeat-editor__field heartbeat-editor__field--connectors">
			<label>连接器 <span className="heartbeat-editor__optional">仅冻结已连接工具的授权快照</span></label>
			<HeartbeatMultiSelect
				value={draft.connectorIds || []}
				options={connectorOptions.map((connector) => {
					const selected = draft.connectorIds?.includes(connector.id) ?? false;
					return {
						value: connector.id,
						label: connector.name || connector.id,
						description: connector.available
							? undefined
							: selected
								? `${connector.unavailableReason || "当前不可用"}；可取消选择`
								: connector.unavailableReason || "当前不可用",
						disabled: !connector.available && !selected,
					};
				})}
				ariaLabel="连接器"
				placeholder="不使用连接器"
				emptyOptionsLabel="暂无可用连接器"
				onChange={(connectorIds) => setDraft((previous) => ({
					...previous,
					connectorIds: connectorIds.length ? connectorIds : undefined,
				}))}
			/>
			{(draft.connectorIds?.length || normalizeMode(draft.approvalMode) === "yolo") ? (
				<span className="heartbeat-editor__mode-hint">运行时仅允许已冻结的连接器工具；高风险操作仍受审批门禁约束。</span>
			) : null}
		</div>

      {/* Approval Mode */}
      {!fullPage && <div className="heartbeat-editor__field">
        <label>{t("heartbeat.fieldApprovalMode")}</label>
        <div className="set-seg" style={{ alignSelf: "flex-start" }}>
          <button
            className={`set-seg__btn${normalizeMode(draft.approvalMode) === "ask" ? " set-seg__btn--on" : ""}`}
            onClick={() => setDraft((prev) => ({ ...prev, approvalMode: "ask" }))}
            title={t("heartbeat.approvalModeAskTooltip")}
          >
            {t("heartbeat.approvalModeAsk")}
          </button>
          <button
            className={`set-seg__btn${normalizeMode(draft.approvalMode) === "auto" ? " set-seg__btn--on" : ""}`}
            onClick={() => setDraft((prev) => ({ ...prev, approvalMode: "auto" }))}
            title={t("heartbeat.approvalModeAutoTooltip")}
          >
            {t("heartbeat.approvalModeAuto")}
          </button>
          <button
            className={`set-seg__btn${normalizeMode(draft.approvalMode) === "yolo" ? " set-seg__btn--on" : ""}`}
            onClick={() => setDraft((prev) => ({ ...prev, approvalMode: "yolo" }))}
            title={t("heartbeat.approvalModeYoloTooltip")}
          >
            {t("heartbeat.approvalModeYolo")}
          </button>
        </div>
        <span className="heartbeat-editor__mode-hint">
          {normalizeMode(draft.approvalMode) === "yolo" ? t("heartbeat.approvalModeYoloHint") :
           normalizeMode(draft.approvalMode) === "auto" ? t("heartbeat.approvalModeAutoHint") :
           t("heartbeat.approvalModeAskHint")}
        </span>
      </div>}

      {/* New conversation per run */}
      <div className="heartbeat-editor__field heartbeat-editor__field--conversation">
        <label>{t("heartbeat.fieldNewConversation")}</label>
        <div className="set-seg" style={{ alignSelf: "flex-start" }}>
          <button
            className={`set-seg__btn${!draft.newConversationEachRun ? " set-seg__btn--on" : ""}`}
            onClick={() => setDraft((prev) => ({ ...prev, newConversationEachRun: false }))}
          >
            {t("heartbeat.newConversationEachRunOff")}
          </button>
          <button
            className={`set-seg__btn${draft.newConversationEachRun ? " set-seg__btn--on" : ""}`}
            onClick={() => setDraft((prev) => ({ ...prev, newConversationEachRun: true }))}
          >
            {t("heartbeat.newConversationEachRunOn")}
          </button>
        </div>
      </div>

      {/* Frequency */}
      <div className="heartbeat-editor__field heartbeat-editor__field--frequency">
        <label>
          {t("heartbeat.fieldInterval")}
          {fullPage && <span className="heartbeat-editor__optional">（建议避开上午高峰时段，高峰期容易排队等待；选择非高峰期执行更稳定）</span>}
        </label>
        <div className="set-seg" style={{ alignSelf: "flex-start" }}>
          <button
            className={`set-seg__btn${freqType === "cycle" ? " set-seg__btn--on" : ""}`}
            onClick={() => {
              setFreqType("cycle");
              // Save the original interval so switching back can restore it
              const cur = draft.interval || "";
              const nextInterval = cur.includes("|") ? cur : "24h|daily@09:00";
              if (!cur.includes("|")) {
                intervalBeforeCycle.current = cur;
              }
              setDraft((prev) => ({ ...prev, interval: nextInterval, timeWindowStart: undefined, timeWindowEnd: undefined }));
            }}
          >
            {t("heartbeat.freqCycle")}
          </button>
          <button
            className={`set-seg__btn${freqType === "interval" ? " set-seg__btn--on" : ""}`}
            onClick={() => {
              setFreqType("interval");
              // Restore original interval if user toggled cycle and back without saving
              if (intervalBeforeCycle.current !== null) {
                setDraft((prev) => ({ ...prev, interval: intervalBeforeCycle.current! }));
                intervalBeforeCycle.current = null;
              } else if ((draft.interval || "").includes("|")) {
                // Fallback: strip cycle suffix
                setDraft((prev) => ({ ...prev, interval: (prev.interval || "").replace(/\|.*$/, "") }));
              }
            }}
          >
            {t("heartbeat.freqInterval")}
          </button>
			<button
				className={`set-seg__btn${freqType === "once" ? " set-seg__btn--on" : ""}`}
				type="button"
				onClick={() => {
					setFreqType("once");
					const runAt = draft.interval?.startsWith("once@")
						? draft.interval.slice("once@".length)
						: new Date(Date.now() + 3_600_000).toISOString().slice(0, 16);
					setDraft((previous) => ({ ...previous, interval: `once@${runAt}`, timeWindowStart: undefined, timeWindowEnd: undefined }));
				}}
			>
				单次
			</button>
        </div>

		{freqType === "once" ? (
			<input
				className="heartbeat-editor__input"
				type="datetime-local"
				value={(draft.interval || "").replace(/^once@/, "").slice(0, 16)}
				onChange={(event) => setDraft((previous) => ({ ...previous, interval: `once@${event.target.value}` }))}
			/>
		) : freqType === "cycle" ? <CycleEditor draft={draft} setDraft={set} scheduleCatalog={scheduleCatalog} /> : (
			<>
          <div className="heartbeat-editor__freq-interval">
            <span className="heartbeat-editor__freq-label">{t("heartbeat.freqEvery")}</span>
            <input
              className="heartbeat-editor__freq-input"
              value={(() => {
                const m = (draft.interval || "").match(/^(\d+)/);
                return m ? m[1] : "1";
              })()}
              onChange={(e) => {
                const num = e.target.value.replace(/\D/g, "");
                const mUnit = (draft.interval || "").match(/^(\d+)([smh])/);
                const unit = mUnit ? mUnit[2] : "h";
                // Guard: never save a bare unit string like "h" or "m"
                setDraft((prev) => ({ ...prev, interval: num ? num + unit : "1" + unit }));
              }}
              placeholder="1"
            />
            <HeartbeatSelect
              className="heartbeat-editor-select--freq-unit"
              value={(() => {
                const m = (draft.interval || "").match(/^(\d+)([smh])/);
                return m ? m[2] : "h";
              })()}
              options={intervalUnitOptions}
              ariaLabel={t("heartbeat.freqInterval")}
              onChange={(unit) => {
                const num = (draft.interval || "").match(/^(\d+)/)?.[1] || "1";
                setDraft((prev) => ({ ...prev, interval: num + unit }));
              }}
            />
            <span className="heartbeat-editor__freq-label" style={{ marginLeft: "6px" }}>
              {draft.timeWindowStart || draft.timeWindowEnd ? (
                <>{t("heartbeat.timeWindow")}</>
              ) : (
                <button
                  className="heartbeat-editor__tw-add"
                  type="button"
                  onClick={() => setDraft((prev) => ({ ...prev, timeWindowStart: "09:00", timeWindowEnd: "17:00" }))}
                  style={{ border: "none", padding: 0, background: "none", font: "inherit" }}
                >
                  + {t("heartbeat.timeWindow")}
                </button>
              )}
            </span>
            {(draft.timeWindowStart || draft.timeWindowEnd) && (
              <>
                <input
                  className="heartbeat-editor__freq-input heartbeat-editor__freq-input--time"
                  type="time"
                  value={draft.timeWindowStart || ""}
                  onChange={(e) => setDraft((prev) => ({ ...prev, timeWindowStart: e.target.value || undefined }))}
                  placeholder="09:00"
                />
                <span className="heartbeat-editor__freq-label heartbeat-editor__tw-sep">—</span>
                <input
                  className="heartbeat-editor__freq-input heartbeat-editor__freq-input--time"
                  type="time"
                  value={draft.timeWindowEnd || ""}
                  onChange={(e) => setDraft((prev) => ({ ...prev, timeWindowEnd: e.target.value || undefined }))}
                  placeholder="17:00"
                />
                <button
                  className="heartbeat-card__open-btn heartbeat-editor__tw-clear"
                  onClick={() => setDraft((prev) => ({ ...prev, timeWindowStart: undefined, timeWindowEnd: undefined }))}
                  title={t("heartbeat.clearTimeWindow")}
                >
                  ×
                </button>
              </>
            )}
          </div>
			{scheduleCatalog.some((entry) => entry.id === "interval" && entry.requiresWeekdays) ? (
				<div className="heartbeat-editor__weekdays" aria-label="可选工作日">
					<span className="heartbeat-editor__optional">可选工作日（留空表示每天）</span>
					{WEEKDAYS.map((weekday) => {
						const selected = draft.intervalWeekdays?.includes(weekday.key) ?? false;
						return (
							<button
								key={weekday.key}
								type="button"
								className={`heartbeat-editor__weekday-btn${selected ? " heartbeat-editor__weekday-btn--on" : ""}`}
								aria-pressed={selected}
								onClick={() => setDraft((previous) => {
									const current = previous.intervalWeekdays || [];
									const intervalWeekdays = current.includes(weekday.key)
										? current.filter((day) => day !== weekday.key)
										: [...current, weekday.key];
									return { ...previous, intervalWeekdays: intervalWeekdays.length ? intervalWeekdays : undefined };
								})}
							>
								{weekday.label}
							</button>
						);
					})}
				</div>
			) : null}
			</>
        )}
      </div>

		<div className="heartbeat-editor__field heartbeat-editor__field--effective-range">
			<label>生效日期区间 <span className="heartbeat-editor__optional">（可选，留空表示始终生效。）</span></label>
			<div className="heartbeat-editor__date-picker">
				<button
					ref={effectiveRangeTriggerRef}
					type="button"
					className="heartbeat-editor__date-picker-trigger"
					aria-label="选择生效日期区间"
					aria-haspopup="dialog"
					aria-expanded={effectiveRangeOpen}
					onClick={(event) => {
						setEffectiveRangeMenuWidth(event.currentTarget.getBoundingClientRect().width);
						setEffectiveRangeOpen((open) => !open);
					}}
				>
					<span className="heartbeat-editor__date-picker-label">{effectiveRangeLabel}</span>
					<ChevronsUpDown size={12} aria-hidden="true" />
				</button>
				<AnchoredPopover
					open={effectiveRangeOpen}
					anchorRef={effectiveRangeTriggerRef}
					onClose={() => setEffectiveRangeOpen(false)}
					className="heartbeat-editor__date-picker-menu"
					placement="bottom"
					style={{ width: effectiveRangeMenuWidth }}
				>
					<div
						className="heartbeat-editor__effective-range"
						onKeyDown={(event) => {
							if (event.key !== "Escape") return;
							event.preventDefault();
							event.stopPropagation();
							setEffectiveRangeOpen(false);
							effectiveRangeTriggerRef.current?.focus();
						}}
					>
						<label className="heartbeat-editor__datetime-input">
							<input aria-label="生效开始时间" className={`heartbeat-editor__input${draft.effectiveFrom ? "" : " heartbeat-editor__input--empty-datetime"}`} lang="zh-CN" type="datetime-local" value={(draft.effectiveFrom || "").slice(0, 16)} onChange={(event) => set("effectiveFrom", event.target.value)} />
							{!draft.effectiveFrom && <span aria-hidden="true">开始日期和时间</span>}
						</label>
						<span>至</span>
						<label className="heartbeat-editor__datetime-input">
							<input aria-label="生效结束时间" className={`heartbeat-editor__input${draft.effectiveUntil ? "" : " heartbeat-editor__input--empty-datetime"}`} lang="zh-CN" type="datetime-local" value={(draft.effectiveUntil || "").slice(0, 16)} onChange={(event) => set("effectiveUntil", event.target.value)} />
							{!draft.effectiveUntil && <span aria-hidden="true">结束日期和时间</span>}
						</label>
					</div>
				</AnchoredPopover>
			</div>
		</div>

		<div className="heartbeat-editor__field heartbeat-editor__field--missed-policy">
			<label>错过执行策略</label>
			<HeartbeatSelect
				value={draft.missedRunPolicy || "latest_once"}
				ariaLabel="错过执行策略"
				options={[
					{ value: "skip", label: "跳过" },
					{ value: "latest_once", label: "仅补最近一次" },
					{ value: "catch_up", label: "逐次补跑", description: "项目管理员场景才应启用" },
				].filter((option) => draft.scope !== "project" || option.value !== "catch_up")}
				onChange={(value) => setDraft((previous) => ({ ...previous, missedRunPolicy: value as HeartbeatTask["missedRunPolicy"] }))}
			/>
			<span className="heartbeat-editor__mode-hint">自动化依赖本机在线；客户端关闭期间不会伪装成云端已执行。</span>
		</div>

		<div className="heartbeat-editor__field heartbeat-editor__field--notifications">
			<label>结果通知</label>
			<label className="heartbeat-editor__template-toggle">
				<input type="checkbox" checked={!!draft.deliveryEnabled} onChange={(event) => setDraft((previous) => ({
					...previous,
					deliveryEnabled: event.target.checked,
				}))} />
				<span>任务完成后推送结果</span>
			</label>
			{draft.deliveryEnabled ? (
				<>
					<HeartbeatMultiSelect
						value={[...selectedDeliveryKeys]}
						options={deliveryTargetOptions.map((target) => {
							const key = deliveryTargetKey(target);
							const selected = selectedDeliveryKeys.has(key);
							const status = target.available
								? [target.provider, target.scope].filter(Boolean).join(" · ")
								: target.unavailableReason || "当前不可用";
							return {
								value: key,
								label: target.label === "已保存目标" ? `${target.label} · ${target.ref}` : target.label,
								description: !target.available && selected ? `${status}；可取消选择` : status,
								disabled: !target.available && !selected,
							};
						})}
						ariaLabel="通知目标"
						placeholder="请选择通知目标"
						emptyOptionsLabel="暂无已授权通知目标"
						onChange={(keys) => setDraft((previous) => ({
							...previous,
							deliveryTargets: keys.flatMap((key) => {
								const target = deliveryTargetOptions.find((option) => deliveryTargetKey(option) === key);
								return target ? [{ type: target.type, ref: target.ref }] : [];
							}),
						}))}
					/>
					{deliverySelectionInvalid ? <span className="heartbeat-editor__mode-hint">请至少选择一个当前可用的通知目标</span> : null}
					<span className="heartbeat-editor__optional">仅保存后端提供的目标引用，不保存账号、远端 ID 或凭据。</span>
				</>
			) : null}
		</div>

      {draft.businessDiagnostics && (
        <BusinessDiagnosticsEditor
          config={draft.businessDiagnostics}
          interval={draft.interval}
          onConfigChange={(businessDiagnostics) => setDraft((prev) => ({ ...prev, businessDiagnostics }))}
          onIntervalChange={(interval) => {
            setFreqType("cycle");
            setDraft((prev) => ({ ...prev, interval, timeWindowStart: undefined, timeWindowEnd: undefined }));
          }}
          onErrorsChange={setBusinessEditorErrors}
        />
      )}

      </div>

      {businessErrors.length > 0 && (
        <div className="heartbeat-editor__validation" ref={businessValidationRef} tabIndex={-1} role="alert">
          <AlertTriangle size={13} />
          <span>{businessErrors[0]}</span>
        </div>
      )}

      {/* Actions */}
      <div className="heartbeat-editor__actions">
        {saveError && !modelSaveError ? (
          <div className="banner banner--error heartbeat-editor__save-feedback heartbeat-editor__save-feedback--error" role="alert">
            <AlertTriangle size={16} aria-hidden="true" focusable="false" />
            <span className="banner__msg">{saveError}</span>
          </div>
        ) : saveStatus ? (
          <div className="heartbeat-editor__save-feedback" role="status">{saveStatus}</div>
        ) : null}
        {capabilityRiskSummary && (
          <div className="heartbeat-editor__save-feedback" role="status">能力提示：{capabilityRiskSummary}</div>
        )}
        {!isNew && !confirmingDelete && (
          <button className="heartbeat-btn heartbeat-btn--danger" disabled={saving} onClick={() => setConfirmingDelete(true)} style={{ marginRight: "auto" }}>
            <Trash2 size={13} />
            {t("heartbeat.delete")}
          </button>
        )}
        {!isNew && confirmingDelete && (
          <span className="heartbeat-editor__confirm-del" style={{ marginRight: "auto" }}>
            <span>{t("heartbeat.confirmDelete")}</span>
            <button className="heartbeat-btn heartbeat-btn--danger" disabled={saving} onClick={onDelete}>
              {t("common.delete")}
            </button>
            <button className="heartbeat-btn" onClick={() => setConfirmingDelete(false)}>
              {t("common.cancel")}
            </button>
          </span>
        )}
        <label className="heartbeat-editor__template-toggle">
          <input
            type="checkbox"
            checked={saveAsTemplate}
            disabled={saving}
            onChange={(e) => setSaveAsTemplate(e.target.checked)}
          />
          <span>同时保存为模板</span>
        </label>
        <button
          className="heartbeat-btn heartbeat-btn--primary"
          onClick={saveDraft}
          disabled={saving}
        >
          {saving ? "正在保存…" : isNew ? t("heartbeat.add") : t("heartbeat.save")}
        </button>
        <button className="heartbeat-btn" onClick={onCancel}>
          {t("common.cancel")}
        </button>
      </div>
    </div>
  );
}
