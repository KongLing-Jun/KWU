import { asArray } from "./array";
import type { BotConnectionView, BotSettingsView } from "./types";

export const BOT_TOOL_APPROVAL_MODES = ["", "ask", "auto", "yolo"] as const;

export type BotConnectionToolApprovalMode = (typeof BOT_TOOL_APPROVAL_MODES)[number];

export function defaultBotSettings(): BotSettingsView {
  return {
    enabled: false,
    model: "",
    toolApprovalMode: "ask",
    maxSteps: 0,
    debounceMs: 1500,
    allowlist: {
      enabled: true,
      allowAll: false,
      qqUsers: [],
      feishuUsers: [],
      weixinUsers: [],
      qqGroups: [],
      feishuGroups: [],
      weixinGroups: [],
    },
    qq: { enabled: false, appId: "", appSecretEnv: "QQ_BOT_APP_SECRET", secretSet: false, sandbox: false },
    feishu: {
      enabled: false,
      domain: "feishu",
      appId: "",
      appSecretEnv: "FEISHU_BOT_APP_SECRET",
      secretSet: false,
      verificationToken: "",
      mode: "webhook",
      webhookPort: 8080,
      requireMention: true,
    },
    weixin: {
      enabled: false,
      accountId: "default",
      tokenEnv: "WEIXIN_BOT_TOKEN",
      tokenSet: false,
      apiBase: "https://ilinkai.weixin.qq.com",
    },
    connections: [],
  };
}

export function normalizeBotSettings(bot: BotSettingsView | null | undefined): BotSettingsView {
  const fallback = defaultBotSettings();
  const allowlist = bot?.allowlist ?? fallback.allowlist;
  const mode = bot?.feishu?.mode === "websocket" ? "websocket" : "webhook";
  return {
    ...fallback,
    ...bot,
    toolApprovalMode: normalizeBotToolApprovalMode(bot?.toolApprovalMode),
    maxSteps: Math.max(0, Number(bot?.maxSteps ?? fallback.maxSteps) || 0),
    debounceMs: Number(bot?.debounceMs) || fallback.debounceMs,
    allowlist: {
      ...fallback.allowlist,
      ...allowlist,
      qqUsers: asArray(allowlist.qqUsers),
      feishuUsers: asArray(allowlist.feishuUsers),
      weixinUsers: asArray(allowlist.weixinUsers),
      qqGroups: asArray(allowlist.qqGroups),
      feishuGroups: asArray(allowlist.feishuGroups),
      weixinGroups: asArray(allowlist.weixinGroups),
    },
    qq: { ...fallback.qq, ...bot?.qq },
    feishu: { ...fallback.feishu, ...bot?.feishu, domain: bot?.feishu?.domain === "lark" ? "lark" : "feishu", mode },
    weixin: { ...fallback.weixin, ...bot?.weixin },
    connections: asArray(bot?.connections).map(normalizeBotConnection),
  };
}

export function normalizeBotConnection(raw: unknown): BotConnectionView {
  const source = raw && typeof raw === "object" ? raw as Record<string, unknown> : {};
  const credential = source.credential && typeof source.credential === "object"
    ? source.credential as Record<string, unknown>
    : {};
  const workspaceRoot = String(source.workspaceRoot ?? "").trim();
  return {
    id: String(source.id ?? "").trim(),
    provider: String(source.provider ?? "").trim(),
    domain: String(source.domain ?? "").trim(),
    label: String(source.label ?? "").trim(),
    enabled: source.enabled !== false,
    status: String(source.status ?? "disconnected").trim(),
    model: String(source.model ?? "").trim(),
    toolApprovalMode: normalizeBotToolApprovalMode(source.toolApprovalMode, true),
    workspaceRoot,
    credential: {
      appId: String(credential.appId ?? "").trim(),
      appSecretEnv: String(credential.appSecretEnv ?? "").trim(),
      accountId: String(credential.accountId ?? "").trim(),
      tokenEnv: String(credential.tokenEnv ?? "").trim(),
      secretSet: Boolean(credential.secretSet),
    },
    sessionMappings: asArray(source.sessionMappings as BotConnectionView["sessionMappings"] | undefined).map((item) => ({
      remoteId: String(item?.remoteId ?? "").trim(),
      sessionId: String(item?.sessionId ?? "").trim(),
      sessionSource: String(item?.sessionSource ?? "").trim(),
      chatType: String(item?.chatType ?? "").trim(),
      userId: String(item?.userId ?? "").trim(),
      threadId: String(item?.threadId ?? "").trim(),
      scope: normalizeBotMappingScope(item?.scope, item?.workspaceRoot ?? workspaceRoot),
      workspaceRoot: normalizeBotMappingScope(item?.scope, item?.workspaceRoot ?? workspaceRoot) === "project"
        ? String(item?.workspaceRoot ?? workspaceRoot).trim()
        : "",
      updatedAt: String(item?.updatedAt ?? "").trim(),
    })),
    lastError: String(source.lastError ?? "").trim(),
    createdAt: String(source.createdAt ?? "").trim(),
    updatedAt: String(source.updatedAt ?? "").trim(),
  };
}

export function normalizeBotToolApprovalMode(mode: unknown, allowEmpty = false): "ask" | "auto" | "yolo" | "" {
  const raw = String(mode ?? "").trim().toLowerCase();
  if (raw === "") return allowEmpty ? "" : "ask";
  if (raw === "ask") return "ask";
  if (raw === "auto") return "auto";
  if (raw === "yolo" || raw === "full" || raw === "full-access" || raw === "bypass") return "yolo";
  return allowEmpty ? "" : "ask";
}

export function normalizeBotMappingScope(scope: unknown, workspaceRoot: unknown): "global" | "project" {
  if (String(scope ?? "").trim() === "project") return "project";
  return String(workspaceRoot ?? "").trim() ? "project" : "global";
}
