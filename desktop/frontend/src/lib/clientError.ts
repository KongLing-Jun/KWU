import type { ClientErrorFrame, ClientErrorRecord } from "./bridge";
import { safeProjectComposerDiagnostic } from "./projectComposerDiagnostics";

export interface ClientErrorLog {
  source: "client";
  scope: string;
  code?: string;
  errorType: string;
  summary: string;
  diagnosticId?: string;
  occurredAt: string;
}

export interface ClientErrorPresentation {
  userMessage: string;
  code?: string;
  diagnosticId?: string;
}

export interface ClientErrorOptions {
  fallbackMessage?: string;
  knownMessages?: Readonly<Record<string, string>>;
  log?: (entry: ClientErrorLog) => void;
  stackFrames?: readonly ClientErrorFrame[];
  componentFrames?: readonly ClientErrorFrame[];
}

export type ClientErrorLocalSink = (entry: ClientErrorRecord) => void;

const DEFAULT_USER_MESSAGE = "操作失败，请稍后重试。";
const MAX_LOG_SUMMARY_LENGTH = 500;

const ERROR_CODE_MESSAGES: Readonly<Record<string, string>> = {
	DURABLE_CREDENTIAL_STATE_INVALID: "当前登录状态异常，请重新登录后重试。",
	TENCENT_MCP_TASK_UNAVAILABLE: "当前任务暂时无法挂载腾讯文档工具，请重试。",
	TENCENT_MCP_OWNER_BINDING_FAILED: "腾讯文档账号绑定已变化，请重新连接。",
	TENCENT_MCP_STATE_UNAVAILABLE: "腾讯文档连接状态不可用，请重新连接。",
	TENCENT_MCP_CREDENTIAL_UNAVAILABLE: "腾讯文档本地授权凭据不可用，请重新连接。",
	TENCENT_MCP_RUNTIME_CONNECT_FAILED: "腾讯文档 MCP 无法注册到当前任务，请重试。",
  SLIDER_CAPTCHA_REQUIRED: "请先完成滑块验证后重试。",
  AUTH_INVALID: "账号或密码不正确，请检查后重试。",
  AUTH_REQUIRED: "登录状态已失效，请重新登录。",
  UNAUTHORIZED: "登录状态已失效，请重新登录。",
  FORBIDDEN: "当前账号没有执行此操作的权限。",
  PERMISSION_DENIED: "当前账号没有执行此操作的权限。",
  NOT_FOUND: "未找到所需内容，它可能已被删除或移动。",
  RATE_LIMITED: "操作过于频繁，请稍后再试。",
  TOO_MANY_REQUESTS: "操作过于频繁，请稍后再试。",
  NETWORK_ERROR: "网络连接失败，请检查网络后重试。",
  TIMEOUT: "请求超时，请稍后重试。",
  SERVICE_UNAVAILABLE: "服务暂时不可用，请稍后重试。",
  SKILL_ALREADY_INSTALLED: "该技能已经安装，无需重复安装。",
  SKILL_CATALOG_UNAVAILABLE: "获取技能详情失败，请稍后重试。",
  SKILL_DOWNLOAD_FAILED: "技能包下载失败，请检查网络后重试。",
  SKILL_INSTALL_PERMISSION: "无法写入技能目录，请检查目录权限后重试。",
  SKILL_INSTALL_BUSY: "技能文件正被其他程序占用，请稍后重试。",
  SKILL_INSTALL_FAILED: "保存技能失败，请稍后重试。",
};

const FRONTEND_SCOPE_ALLOWLIST = new Set([
  "account.assets.refresh",
  "account.logout",
  "account.maasBalance",
  "account.maasRechargeBalance",
  "account.operation",
  "account.payment",
  "automation.export",
  "automation.operation",
  "automation.parseRows",
  "bots.copyDiagnostic",
  "bots.install",
  "capabilities.operation",
  "connectors.load",
  "connectors.managed.cancel",
  "connectors.managed.connect",
  "connectors.managed.disconnect",
  "connectors.managed.load",
  "connectors.managed.poll",
  "connectors.managed.prepare",
  "connectors.managed.token",
  "connectors.operation",
  "connectors.quick.load",
  "connectors.quick.poll",
  "connectors.quick.retry",
  "connectors.test",
  "composer.send",
  "composer.toolApprovalMode",
  "creation.skill.resolve",
  "experts.activate",
  "experts.clear",
  "experts.newTask",
  "experts.refresh",
  "experts.save",
  "experts.switch",
  "global.react",
  "global.unhandledrejection",
  "global.window.error",
  "history.openSession",
  "history.preview",
  "memory.operation",
  "models.fetch",
  "models.fetchAfterKey",
  "models.load",
  "onboarding.fetchModels",
  "onboarding.save",
  "projectComposer.send",
  "projectComposer.toolApprovalMode",
  "projects.operation",
  "projectTree.createCloudProjectTopic",
  "projectTree.createTopic",
  "projectTree.pinProject",
  "projectTree.pinProjectSession",
  "projectTree.pinTopic",
  "projectTree.removeWorkspace",
  "projectTree.renameProject",
  "projectTree.renameTopic",
  "projectTree.trashProjectSession",
  "projectTree.trashTopic",
  "scene.restore",
  "scene.save",
  "session.cancel",
  "session.clearContext",
  "session.command",
  "session.new",
  "session.openTopic",
  "session.refresh",
  "session.resume",
  "session.send",
  "session.steer",
  "session.switchEffort",
  "session.switchModel",
  "session.switchTab",
  "session.switchTokenMode",
  "settings.appearance.style",
  "settings.appearance.theme",
  "settings.save",
  "skills.catalog.detail",
  "skills.catalog.install",
  "skills.catalog.load",
  "skills.load",
  "skills.operation",
  "skills.quick.disable",
  "skills.quick.enable",
  "skills.quick.load",
  "topics.rename",
  "updater.operation",
  "workspace.preview",
]);

const FRONTEND_CODE_ALLOWLIST = new Set([
  "ACCOUNT_DISABLED",
  "AUTH_CRYPTO_CONFIG_UNAVAILABLE",
  "AUTH_INVALID",
  "AUTH_RATE_LIMITED",
  "AUTH_REQUIRED",
  "CLOUD_PROTOCOL_ERROR",
  "CLOUD_ERROR",
  "CONNECTOR_OAUTH_FAILED",
  "CONFLICT",
  "DURABLE_CREDENTIAL_STATE_INVALID",
  "FORBIDDEN",
  "IDEMPOTENCY_CONFLICT",
  "INTERNAL_ERROR",
  "INVALID_CREDENTIALS",
  "INVALID_RESPONSE",
  "NETWORK_ERROR",
  "NETWORK_UNAVAILABLE",
  "NOT_FOUND",
  "PASSWORD_DECRYPT_FAILED",
  "PASSWORD_INVALID",
  "PASSWORD_MISMATCH",
  "PERMISSION_DENIED",
  "PHONE_ALREADY_REGISTERED",
  "PHONE_INVALID",
  "RATE_LIMITED",
  "SERVICE_UNAVAILABLE",
  "SKILL_ALREADY_INSTALLED",
  "SKILL_CATALOG_UNAVAILABLE",
  "SKILL_DOWNLOAD_FAILED",
  "SKILL_INSTALL_BUSY",
  "SKILL_INSTALL_FAILED",
  "SKILL_INSTALL_PERMISSION",
  "SLIDER_CAPTCHA_EXPIRED",
  "SLIDER_CAPTCHA_REUSED",
  "SLIDER_CAPTCHA_REQUIRED",
  "SMS_CODE_ATTEMPTS_EXCEEDED",
  "SMS_CODE_EXPIRED",
  "SMS_CODE_INVALID",
  "SMS_PROVIDER_UNAVAILABLE",
  "SMS_RATE_LIMITED",
  "TENCENT_MCP_CREDENTIAL_UNAVAILABLE",
  "TENCENT_MCP_OWNER_BINDING_FAILED",
  "TENCENT_MCP_RUNTIME_CONNECT_FAILED",
  "TENCENT_MCP_STATE_UNAVAILABLE",
  "TENCENT_MCP_TASK_UNAVAILABLE",
  "TIMEOUT",
  "TOO_MANY_REQUESTS",
  "UNAUTHORIZED",
  "UPSTREAM_ERROR",
  "VALIDATION_FAILED",
]);

const CLIENT_ERROR_TYPES = new Set(["Error", "Panic", "Warning", "RuntimeError", "TypeError", "unknown"]);

const recordedObjects = new WeakSet<object>();
const recentFingerprints = new Map<string, number>();
const FINGERPRINT_TTL_MS = 2_000;
let localSink: ClientErrorLocalSink | undefined;

export function installClientErrorLocalSink(sink?: ClientErrorLocalSink): void {
  localSink = sink;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === "object" ? value as Record<string, unknown> : null;
}

function stringField(record: Record<string, unknown> | null, key: string): string {
  const value = record?.[key];
  return typeof value === "string" ? value.trim() : "";
}

function normalizeCode(value: string): string | undefined {
  const code = value.trim().toUpperCase();
  return /^[A-Z][A-Z0-9_]{2,80}$/.test(code) ? code : undefined;
}

function normalizeDiagnosticID(value: string): string | undefined {
  const diagnosticID = value.trim();
  return /^[A-Za-z0-9_-]{1,64}$/.test(diagnosticID) ? diagnosticID : undefined;
}

export function clientErrorCode(error: unknown): string | undefined {
  const record = asRecord(error);
  const direct = normalizeCode(stringField(record, "code"));
  if (direct) return direct;

  const message = error instanceof Error ? error.message : typeof error === "string" ? error : stringField(record, "message");
  const bracketed = message.match(/(?:^|\b)([A-Z][A-Z0-9_]{2,80})(?=[:\s,\]])/);
  return bracketed ? normalizeCode(bracketed[1]) : undefined;
}

export function clientErrorDiagnosticID(error: unknown): string | undefined {
  const record = asRecord(error);
  const direct = normalizeDiagnosticID(stringField(record, "diagnostic_id") || stringField(record, "diagnosticId"));
  if (direct) return direct;
  const message = error instanceof Error ? error.message : typeof error === "string" ? error : stringField(record, "message");
  const match = /\[diagnostic_id=([A-Za-z0-9_-]{1,64})\]/.exec(message);
  return match ? normalizeDiagnosticID(match[1]) : undefined;
}

export function clientErrorMessageWithoutDiagnosticID(error: unknown): string {
  const record = asRecord(error);
  const message = error instanceof Error ? error.message : typeof error === "string" ? error : stringField(record, "message");
  return message.replace(/\s*\[diagnostic_id=[A-Za-z0-9_-]{1,64}\]/g, "").trim();
}

export function redactClientErrorText(value: string): string {
  return value
    .replace(/([?&](?:token|access_token|refresh_token|api[_-]?key|password|secret)=)[^&#\s]+/gi, "$1[已脱敏]")
    .replace(/\b((?:token|access[_-]?token|refresh[_-]?token|api[_-]?key|password|secret)\s*[=:]\s*)[^\s,;]+/gi, "$1[已脱敏]")
    .replace(/\b(Bearer\s+)[A-Za-z0-9._~+/=-]+/gi, "$1[已脱敏]")
    .replace(/("(?:token|accessToken|refreshToken|apiKey|password|secret)"\s*:\s*")[^"]*"/gi, "$1[已脱敏]\"")
    .replace(/\b1[3-9]\d{9}\b/g, "[手机号已脱敏]")
    .replace(/\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/gi, "[邮箱已脱敏]")
    .replace(/\b[A-Za-z]:\\(?:[^\\\r\n]+\\)*[^\\\r\n]*/g, "[本地路径已脱敏]")
    .replace(/(?:^|\s)\/(?:Users|home|var|tmp)\/[^\s]*/g, " [本地路径已脱敏]")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, MAX_LOG_SUMMARY_LENGTH);
}

function errorSummary(error: unknown): { errorType: string; summary: string } {
  if (error instanceof Error) {
    return {
      errorType: error.name || "Error",
      summary: redactClientErrorText(error.message || "未提供异常信息"),
    };
  }
  if (typeof error === "string") {
    return { errorType: "StringError", summary: redactClientErrorText(error) };
  }
  const record = asRecord(error);
  if (record) {
    const type = stringField(record, "name") || "ObjectError";
    const message = stringField(record, "message") || stringField(record, "error") || "未提供异常信息";
    return { errorType: type, summary: redactClientErrorText(message) };
  }
  return {
    errorType: error === null ? "NullError" : typeof error,
    summary: redactClientErrorText(String(error ?? "未提供异常信息")),
  };
}

function shouldRecord(error: unknown, fingerprint: string, now: number): boolean {
  if (error !== null && typeof error === "object") {
    if (recordedObjects.has(error)) return false;
    recordedObjects.add(error);
    return true;
  }
  const previous = recentFingerprints.get(fingerprint);
  recentFingerprints.set(fingerprint, now);
  if (recentFingerprints.size > 100) {
    for (const [key, timestamp] of recentFingerprints) {
      if (now - timestamp > FINGERPRINT_TTL_MS) recentFingerprints.delete(key);
    }
  }
  return previous === undefined || now - previous > FINGERPRINT_TTL_MS;
}

function safeJournalFrames(frames: readonly ClientErrorFrame[] | undefined): ClientErrorFrame[] | undefined {
  if (!frames?.length) return undefined;
  const safe: ClientErrorFrame[] = [];
  for (const frame of frames) {
    if (!frame || typeof frame !== "object") continue;
    const category = frame.category === "app" || frame.category === "framework" || frame.category === "runtime" || frame.category === "unknown"
      ? frame.category
      : "unknown";
    const numbers = [frame.ordinal, frame.line, frame.column];
    if (numbers.some((value) => !Number.isFinite(value) || !Number.isInteger(value) || value < 0)) continue;
    safe.push({ category, ordinal: frame.ordinal, line: frame.line, column: frame.column });
    if (safe.length >= 64) break;
  }
  return safe.length ? safe : undefined;
}

function journalRecord(entry: ClientErrorLog, options: ClientErrorOptions, error: unknown): ClientErrorRecord {
  const scope = FRONTEND_SCOPE_ALLOWLIST.has(entry.scope) ? entry.scope : "frontend.unknown";
  const projectComposer = scope === "projectComposer.send"
    ? safeProjectComposerDiagnostic(entry.code, asRecord(error)?.projectComposer)
    : undefined;
  const code = projectComposer ? entry.code! : entry.code && FRONTEND_CODE_ALLOWLIST.has(entry.code) ? entry.code : "UNKNOWN";
  const errorType = CLIENT_ERROR_TYPES.has(entry.errorType) ? entry.errorType : "Error";
  const crashScope = entry.scope === "global.react" || entry.scope === "global.window.error" || entry.scope === "global.unhandledrejection";
  const stackFrames = crashScope ? safeJournalFrames(options.stackFrames) : undefined;
  const componentFrames = crashScope ? safeJournalFrames(options.componentFrames) : undefined;
  return {
    scope,
    code,
    errorType,
    occurredAt: entry.occurredAt,
    ...(entry.diagnosticId ? { diagnosticId: entry.diagnosticId } : {}),
    ...(stackFrames ? { stackFrames } : {}),
    ...(componentFrames ? { componentFrames } : {}),
    ...(projectComposer ? { projectComposer } : {}),
  };
}

export function clientErrorPresentation(
  error: unknown,
  options: Pick<ClientErrorOptions, "fallbackMessage" | "knownMessages"> = {},
): ClientErrorPresentation {
  const code = clientErrorCode(error);
  const diagnosticId = clientErrorDiagnosticID(error);
  const baseMessage = (code && options.knownMessages?.[code])
    || (code && ERROR_CODE_MESSAGES[code])
    || options.fallbackMessage
    || DEFAULT_USER_MESSAGE;
  return { userMessage: baseMessage, code, diagnosticId };
}

export function reportClientError(
  scopeName: string,
  error: unknown,
  options: ClientErrorOptions = {},
): ClientErrorPresentation {
  const presentation = clientErrorPresentation(error, options);
  const normalized = errorSummary(error);
  const scope = typeof scopeName === "string" ? scopeName.trim() : "";
  const entry: ClientErrorLog = {
    source: "client",
    scope: scope || "unknown",
    code: presentation.code,
    errorType: normalized.errorType,
    summary: normalized.summary,
    diagnosticId: presentation.diagnosticId,
    occurredAt: new Date().toISOString(),
  };
  const fingerprint = `${entry.scope}|${entry.code || ""}|${entry.errorType}|${entry.summary}`;
  if (localSink) {
    try {
      localSink(journalRecord(entry, options, error));
    } catch {
      // Local persistence is best-effort and must not create a new failure.
    }
  }
  if (shouldRecord(error, fingerprint, Date.now())) {
    const log = options.log || ((record: ClientErrorLog) => console.error("[客户端错误]", record));
    log(entry);
  }
  return presentation;
}
