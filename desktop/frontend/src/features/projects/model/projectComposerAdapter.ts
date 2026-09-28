import { defaultComposerProfile, type ComposerProfile } from "../../../lib/composerProfile";
import type { ComposerCapabilitySelection } from "../../../lib/composerCapabilities";
import { buildComposerCapabilitySubmit } from "../../../lib/composerCapabilities";
import { normalizeToolApprovalMode, type ComposerCapabilitySubmit, type SettingsView, type SkillView } from "../../../lib/types";
import type { EffortInfo, ModelInfo } from "../../../lib/types";
import type { ExpertSelection } from "../../experts/ui/ExpertDirectory";
import { isMCPManagementCommand } from "../../../lib/composerCapabilities";
import type { CloudProjectCapabilityBinding, CloudProjectSession, ProjectConnectorAvailability } from "./projectTypes";
import {
  createProjectComposerAttempt,
  PROJECT_DIAGNOSTIC_CODES,
  snapshotProjectComposerDiagnostic,
  type ProjectComposerAttempt,
  type ProjectComposerDiagnostic,
} from "../../../lib/projectComposerDiagnostics";

export const PROJECT_COMPOSER_ADAPTER_LIMIT = 8;

export type ProjectComposerPhase = "draft" | "preparing-tab" | "creating-session" | "pinned-unsent" | "sending" | "sent";

export interface ProjectComposerIdentity {
  accountScopeKey: string;
  projectId: string;
}

export interface ProjectComposerAdapter extends ProjectComposerIdentity {
  key: string;
  phase: ProjectComposerPhase;
  tabId: string;
  sessionPath: string;
  workspacePath: string;
  executionTarget: "local" | "cloud";
  modelRef: string;
  modelBillingSource?: ModelInfo["billingSource"];
  effort?: EffortInfo;
  effortRouteKey: string;
  activeExpert?: ExpertSelection;
  expertRequestSequence?: number;
  expertMutation?: Promise<boolean>;
  tabPreparation?: Promise<string>;
  availableExperts: ExpertSelection[];
  profile: ComposerProfile;
  capabilities: ComposerCapabilitySelection;
  defaultsInitialized: boolean;
}

export type ProjectComposerSendOutcome = "submitted" | "handled-no-turn" | "blocked";

export type ProjectComposerFailureStage =
  | "prepare-tab"
  | "create-session"
  | "reconcile-session"
  | "activate-session"
  | "apply-profile"
  | "resolve-capabilities"
  | "send-message";

const PROJECT_COMPOSER_FAILURE_CODES: Record<ProjectComposerFailureStage, string> = PROJECT_DIAGNOSTIC_CODES;

const PROJECT_COMPOSER_FAILURE_MESSAGES: Record<ProjectComposerFailureStage, string> = {
  "prepare-tab": "任务准备失败，草稿已保留，请重试。",
  "create-session": "项目任务创建失败，草稿已保留，请重试。",
  "reconcile-session": "项目任务状态校验失败，草稿已保留，请重试。",
  "activate-session": "任务已创建，但聊天页面暂时无法打开。草稿已保留，请重试。",
  "apply-profile": "任务已创建，但运行设置未能应用。草稿已保留，请重试。",
  "resolve-capabilities": "任务已创建，但项目能力准备失败。草稿已保留，请重试。",
  "send-message": "消息未送达，草稿已保留，请重试。",
};

function projectComposerDiagnosticId(): string {
  const uuid = globalThis.crypto?.randomUUID?.().replace(/-/g, "");
  if (uuid) return uuid.slice(0, 10).toUpperCase();
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`.slice(-10).toUpperCase();
}

export class ProjectComposerSubmitError extends Error {
  readonly code: string;
  readonly diagnosticId: string;
  readonly originalError: unknown;
  projectComposer?: ProjectComposerDiagnostic;

  constructor(readonly stage: ProjectComposerFailureStage, cause: unknown, diagnosticId = projectComposerDiagnosticId()) {
    const detail = cause instanceof Error ? cause.message : String(cause ?? "unknown error");
    const code = PROJECT_COMPOSER_FAILURE_CODES[stage];
    super(`${code}: ${detail}`);
    this.name = "ProjectComposerSubmitError";
    this.code = code;
    this.diagnosticId = diagnosticId;
    this.originalError = cause;
  }
}

export function asProjectComposerSubmitError(stage: ProjectComposerFailureStage, error: unknown, attempt?: ProjectComposerAttempt): ProjectComposerSubmitError {
  const failure = error instanceof ProjectComposerSubmitError ? error : new ProjectComposerSubmitError(stage, error);
  if (!failure.projectComposer && attempt) {
    failure.projectComposer = snapshotProjectComposerDiagnostic(failure.stage, failure.originalError, attempt);
  }
  return failure;
}

export function projectComposerFailureMessage(error: unknown): string {
  if (!(error instanceof ProjectComposerSubmitError)) return "项目任务发送失败，草稿已保留，请重试。";
  return `${PROJECT_COMPOSER_FAILURE_MESSAGES[error.stage]}（诊断编号：${error.diagnosticId}）`;
}

export function projectComposerBlankSessionTarget(workspacePath: string, executionTarget: "local" | "cloud" = "local"): { scope: "project" | "global"; workspaceRoot: string } {
  if (executionTarget === "cloud") return { scope: "global", workspaceRoot: "" };
  const workspaceRoot = workspacePath.trim();
  return workspaceRoot
    ? { scope: "project", workspaceRoot }
    : { scope: "global", workspaceRoot: "" };
}

export function projectComposerKey(identity: ProjectComposerIdentity): string {
  return JSON.stringify(["project-composer-v1", identity.accountScopeKey, identity.projectId]);
}

export function parseProjectComposerKey(key: string): ProjectComposerIdentity | null {
  try {
    const value = JSON.parse(key);
    if (!Array.isArray(value) || value.length !== 3 || value[0] !== "project-composer-v1") return null;
    const accountScopeKey = typeof value[1] === "string" ? value[1] : "";
    const projectId = typeof value[2] === "string" ? value[2] : "";
    return accountScopeKey && projectId ? { accountScopeKey, projectId } : null;
  } catch {
    return null;
  }
}

export function createProjectComposerAdapter(identity: ProjectComposerIdentity): ProjectComposerAdapter {
  return {
    ...identity,
    key: projectComposerKey(identity),
    phase: "draft",
    tabId: "",
    sessionPath: "",
    workspacePath: "",
    executionTarget: "cloud",
    modelRef: "",
    effortRouteKey: "",
    availableExperts: [],
    profile: { ...defaultComposerProfile, pending: {} },
    capabilities: {},
    defaultsInitialized: false,
  };
}

export function initializeProjectComposerDefaults(
  adapter: ProjectComposerAdapter,
  settings: Pick<SettingsView, "defaultModel" | "defaultBillingSource" | "defaultToolApprovalMode">,
): ProjectComposerAdapter {
  if (adapter.defaultsInitialized || adapter.tabId || adapter.sessionPath) return adapter;
  adapter.modelRef = settings.defaultModel.trim();
  adapter.modelBillingSource = settings.defaultBillingSource?.trim() as ModelInfo["billingSource"] || undefined;
  adapter.profile = {
    ...adapter.profile,
    toolApprovalMode: normalizeToolApprovalMode(settings.defaultToolApprovalMode),
    pending: {},
  };
  adapter.defaultsInitialized = true;
  return adapter;
}

function projectSkillRuntimeName(registryRef: string): string {
  const normalized = registryRef.trim();
  const separator = normalized.lastIndexOf("/");
  return (separator >= 0 ? normalized.slice(separator + 1) : normalized).trim();
}

export function applyProjectCapabilityBindings(
  adapter: ProjectComposerAdapter,
  bindings: readonly CloudProjectCapabilityBinding[],
): ProjectComposerAdapter {
  const enabled = bindings.filter((binding) => !binding.publicDisabled);
  const skills = enabled
    .filter((binding) => binding.kind === "skill")
    .map((binding) => projectSkillRuntimeName(binding.registryRef))
    .filter(Boolean);
  adapter.capabilities = {
    ...adapter.capabilities,
    skills: [...new Set(skills)],
    // Project connector bindings describe the project. They must not become
    // account-wide Composer switches merely because the project is open.
    connectors: [],
  };
  return adapter;
}

export function projectComposerCapabilitySubmit(
  text: string,
  projectSelection: ComposerCapabilitySelection,
  turn?: ComposerCapabilitySubmit,
  configuredProjectSelection: ComposerCapabilitySelection = projectSelection,
): ComposerCapabilitySubmit | undefined {
  // Project skill bindings describe what is available in the project. A skill
  // only runs when the user explicitly adds its token to the current turn.
  const configuredConnectors = new Set(configuredProjectSelection.connectors ?? []);
  const turnConnectors = (turn?.mcp_server_names ?? []).filter((name) => !configuredConnectors.has(name));
  const skills = [...new Set(turn?.skill_names ?? [])];
  const connectors = [...new Set([...(projectSelection.connectors ?? []), ...turnConnectors])];
  if (skills.length === 0 && connectors.length === 0 && !turn) return undefined;
  return buildComposerCapabilitySubmit(turn?.text ?? text, { skills, connectors }, turn?.title_text);
}

export function projectComposerCapabilitiesForTab(
  registry: ReadonlyMap<string, ProjectComposerAdapter>,
  tabId: string | undefined,
): ComposerCapabilitySelection | undefined {
  if (!tabId) return undefined;
  for (const adapter of registry.values()) {
    if (adapter.tabId === tabId && adapter.sessionPath) return adapter.capabilities;
  }
  return undefined;
}

export function projectComposerAdapterForTab(
  registry: ReadonlyMap<string, ProjectComposerAdapter>,
  tabId: string | undefined,
): ProjectComposerAdapter | undefined {
  if (!tabId) return undefined;
  for (const adapter of registry.values()) {
    if (adapter.tabId === tabId && adapter.sessionPath) return adapter;
  }
  return undefined;
}

export function availableProjectCapabilitySelection(
  selection: ComposerCapabilitySelection,
  installedSkills: readonly Pick<SkillView, "name" | "uninstallId" | "enabled">[],
  connectorAvailability?: readonly Pick<ProjectConnectorAvailability, "registryRef" | "localServerName" | "available">[],
): ComposerCapabilitySelection {
  const runtimeNameByRef = new Map<string, string>();
  for (const skill of installedSkills) {
    const runtimeName = skill.name.trim();
    if (!skill.enabled || !runtimeName) continue;
    const uninstallId = (skill.uninstallId || "").trim();
    for (const ref of [runtimeName, uninstallId, projectSkillRuntimeName(uninstallId)]) {
      if (ref && !runtimeNameByRef.has(ref)) runtimeNameByRef.set(ref, runtimeName);
    }
  }
  const skills = (selection.skills ?? [])
    .map((name) => runtimeNameByRef.get(name.trim()) || "")
    .filter((name, index, values) => Boolean(name) && values.indexOf(name) === index);
  const availableConnectorRefs = connectorAvailability === undefined
    ? null
    : new Set(connectorAvailability
      .filter((connector) => connector.available)
      .flatMap((connector) => [connector.registryRef.trim(), connector.localServerName.trim()])
      .filter(Boolean));
  const connectors = (selection.connectors ?? [])
    .map((name) => name.trim())
    .filter((name, index, values) => Boolean(name)
      && values.indexOf(name) === index
      && (availableConnectorRefs === null || availableConnectorRefs.has(name)));
  return {
    ...selection,
    skills,
    connectors,
  };
}

export function ensureProjectComposerAdapter(
  registry: Map<string, ProjectComposerAdapter>,
  identity: ProjectComposerIdentity,
  limit = PROJECT_COMPOSER_ADAPTER_LIMIT,
): ProjectComposerAdapter | null {
  const key = projectComposerKey(identity);
  const current = registry.get(key);
  if (current) return current;
  let count = 0;
  for (const adapter of registry.values()) {
    if (adapter.accountScopeKey === identity.accountScopeKey && adapter.phase !== "sent") count += 1;
  }
  if (count >= limit) return null;
  const adapter = createProjectComposerAdapter(identity);
  registry.set(key, adapter);
  return adapter;
}

export function ensureNewProjectConversationAdapter(
  registry: Map<string, ProjectComposerAdapter>,
  identity: ProjectComposerIdentity,
  limit = PROJECT_COMPOSER_ADAPTER_LIMIT,
): { adapter: ProjectComposerAdapter | null; replacedSentKey?: string } {
  const key = projectComposerKey(identity);
  const current = registry.get(key);
  if (!current || current.phase !== "sent") {
    return { adapter: ensureProjectComposerAdapter(registry, identity, limit) };
  }
  registry.delete(key);
  const retainedSentKey = JSON.stringify([
    "project-composer-sent-v1",
    current.accountScopeKey,
    current.projectId,
    current.tabId,
    current.sessionPath,
  ]);
  current.key = retainedSentKey;
  registry.set(retainedSentKey, current);
  return {
    adapter: ensureProjectComposerAdapter(registry, identity, limit),
    replacedSentKey: key,
  };
}

export function releaseProjectComposerAdapter(
  registry: Map<string, ProjectComposerAdapter>,
  key: string,
): ProjectComposerAdapter | null {
  const adapter = registry.get(key) ?? null;
  if (adapter) registry.delete(key);
  return adapter;
}

type ProjectComposerDraftContent = {
  text: string;
  attachments: unknown[];
  workspaceRefs: unknown[];
  pastedBlocks: unknown[];
  sessionRefs: unknown[];
};

export function planProjectComposerLeave(
  adapter: ProjectComposerAdapter | undefined,
  draft: ProjectComposerDraftContent | undefined,
): { removeKey: string; closeTabId?: string } | null {
  if (!adapter || adapter.sessionPath || ["preparing-tab", "creating-session", "sending"].includes(adapter.phase)) return null;
  const hasDraft = Boolean(draft && (
    draft.text.trim()
    || draft.attachments.length
    || draft.workspaceRefs.length
    || draft.pastedBlocks.length
    || draft.sessionRefs.length
  ));
  if (hasDraft) return null;
  return { removeKey: adapter.key, closeTabId: adapter.tabId || undefined };
}

export interface ProjectComposerScopeChangePlan {
  removedKeys: string[];
  closeTabIds: string[];
  retainedKeys: string[];
}

export function planProjectComposerScopeChange(
  registry: Map<string, ProjectComposerAdapter>,
  accountScopeKey: string,
): ProjectComposerScopeChangePlan {
  const removed: string[] = [];
  const closeTabIds: string[] = [];
  const retainedKeys: string[] = [];
  for (const [key, adapter] of registry) {
    if (adapter.accountScopeKey === accountScopeKey) continue;
    registry.delete(key);
    removed.push(key);
    if (adapter.tabId) closeTabIds.push(adapter.tabId);
  }
  return { removedKeys: removed, closeTabIds, retainedKeys };
}

export function classifyProjectComposerFirstSend(text: string, submitText = text): ProjectComposerSendOutcome {
  const command = text.trim();
  if (!command) return submitText.trim() ? "submitted" : "blocked";
  if (command.startsWith("!")) return "handled-no-turn";
  if (/^\/(?:memory|profile|clear|theme|model)(?:\s|$)/i.test(command)) return "handled-no-turn";
  if (/^\/(?:help|status)(?:\s|$)/i.test(command)) return "handled-no-turn";
  if (/^\/skill(?:\s+(?:list|show)(?:\s+\S+)?)?$/i.test(command)) return "handled-no-turn";
  if (isMCPManagementCommand(command)) return "handled-no-turn";
  return "submitted";
}

type ProjectComposerRuntimeTab = { id: string; sessionPath?: string };

export interface ProjectComposerTurnDependencies<TTab extends ProjectComposerRuntimeTab> {
  createSession: (tabId: string) => Promise<CloudProjectSession>;
  refreshTabs: () => Promise<TTab[]>;
  activateTab: (tabId: string, tab: TTab) => Promise<void>;
  setCollaborationMode: (tabId: string, mode: ComposerProfile["collaborationMode"]) => Promise<void>;
  setToolApprovalMode: (tabId: string, mode: ComposerProfile["toolApprovalMode"]) => Promise<void>;
  setTokenMode: (tabId: string, mode: ComposerProfile["tokenMode"]) => Promise<void>;
  setGoal: (tabId: string, goal: string) => Promise<void>;
  send: (tabId: string) => Promise<void>;
  promote: (tabId: string) => Promise<void>;
}

export async function submitProjectComposerTurn<TTab extends ProjectComposerRuntimeTab>(
  adapter: ProjectComposerAdapter,
  expected: { projectId: string; instructionVersion: number; contentHash: string },
  dependencies: ProjectComposerTurnDependencies<TTab>,
  attempt: ProjectComposerAttempt = createProjectComposerAttempt(adapter),
): Promise<ProjectComposerSendOutcome> {
  if (["preparing-tab", "creating-session", "sending"].includes(adapter.phase)) return "blocked";
  attempt.hasTab = Boolean(adapter.tabId);
  attempt.hasSession = Boolean(adapter.sessionPath);
  if (!adapter.tabId) throw new Error("project composer requires a prepared tab");
  try {
    if (!adapter.sessionPath) {
      adapter.phase = "creating-session";
      let session: CloudProjectSession;
      try {
        attempt.step = "create-session";
        session = await dependencies.createSession(adapter.tabId);
      } catch (error) {
        throw asProjectComposerSubmitError("create-session", error, attempt);
      }
      try {
        attempt.step = "verify-session";
        reconcileProjectSession(adapter, session, { ...expected, tabId: adapter.tabId }, attempt);
      } catch (error) {
        throw asProjectComposerSubmitError("reconcile-session", error, attempt);
      }
    }
    let target: TTab;
    try {
      attempt.step = "refresh-tabs";
      attempt.listTabsOutcome = "not-called";
      delete attempt.tabFound;
      delete attempt.pathMatches;
      const tabs = await dependencies.refreshTabs();
      if (attempt.listTabsOutcome === "not-called") attempt.listTabsOutcome = "succeeded";
      attempt.step = "verify-target";
      attempt.hasTab = Boolean(adapter.tabId);
      attempt.hasSession = Boolean(adapter.sessionPath);
      const refreshed = tabs.find((tab) => tab.id === adapter.tabId);
      if (attempt.listTabsOutcome === "succeeded") {
        attempt.tabFound = Boolean(refreshed);
        if (refreshed) attempt.pathMatches = refreshed.sessionPath === adapter.sessionPath;
      }
      if (!refreshed || refreshed.sessionPath !== adapter.sessionPath) {
        if (attempt.listTabsOutcome === "succeeded") attempt.reason = refreshed ? "path-mismatch" : "tab-missing";
        throw new Error("project session tab/path reconciliation failed");
      }
      target = refreshed;
    } catch (error) {
      if (attempt.step === "refresh-tabs" && attempt.listTabsOutcome === "not-called") attempt.listTabsOutcome = "failed";
      throw asProjectComposerSubmitError("reconcile-session", error, attempt);
    }
    try {
      attempt.step = "switch-tab";
      await dependencies.activateTab(adapter.tabId, target);
    } catch (error) {
      throw asProjectComposerSubmitError("activate-session", error, attempt);
    }
    try {
      attempt.step = "set-collaboration-mode";
      await dependencies.setCollaborationMode(adapter.tabId, adapter.profile.collaborationMode);
      attempt.step = "set-tool-approval-mode";
      await dependencies.setToolApprovalMode(adapter.tabId, adapter.profile.toolApprovalMode);
      attempt.step = "set-token-mode";
      await dependencies.setTokenMode(adapter.tabId, adapter.profile.tokenMode);
      attempt.step = "set-goal";
      await dependencies.setGoal(adapter.tabId, adapter.profile.goal);
    } catch (error) {
      throw asProjectComposerSubmitError("apply-profile", error, attempt);
    }
    adapter.phase = "sending";
    try {
      attempt.step = "send-message";
      await dependencies.send(adapter.tabId);
    } catch (error) {
      throw asProjectComposerSubmitError("send-message", error, attempt);
    }
    adapter.phase = "sent";
    // A submitted receipt is irreversible. Promotion is local presentation
    // cleanup and must never turn an accepted user turn into a retryable draft.
    await dependencies.promote(adapter.tabId).catch(() => {});
    return "submitted";
  } catch (error) {
    if (adapter.phase !== "sent") adapter.phase = adapter.sessionPath ? "pinned-unsent" : "draft";
    throw error;
  }
}

export function reconcileProjectSession(
  adapter: ProjectComposerAdapter,
  session: CloudProjectSession,
  expected: { projectId: string; tabId: string; instructionVersion: number; contentHash: string },
  attempt?: ProjectComposerAttempt,
): ProjectComposerAdapter {
  if (session.projectId !== expected.projectId
    || session.tabId !== expected.tabId
    || session.instructionVersion !== expected.instructionVersion
    || session.contentHash !== expected.contentHash
    || !session.sessionPath.trim()) {
    if (attempt) attempt.reason = "snapshot-mismatch";
    throw new Error("project session did not match the prepared tab and instruction snapshot");
  }
  adapter.tabId = session.tabId;
  adapter.sessionPath = session.sessionPath;
  adapter.phase = "pinned-unsent";
  return adapter;
}
