import {
  modeHasAutoApproveTools,
  normalizeMode,
  normalizeToolApprovalMode,
  type Meta,
  type TabMeta,
} from "../../lib/types";

type RuntimeMetaSnapshot = {
  running: boolean;
  pendingPrompt?: boolean;
  backgroundJobs?: number;
  cancellable?: boolean;
};

export function projectTreeRuntimeKeyForTabs(tabs: readonly TabMeta[]): string {
  return JSON.stringify(tabs.map((tab) => [
    tab.id,
    tab.sessionPath ?? "",
    Boolean(tab.running),
    Boolean(tab.pendingPrompt),
    tab.backgroundJobs ?? 0,
  ]));
}

export function foregroundRunningFromRuntimeMeta(meta: RuntimeMetaSnapshot): boolean {
  if (typeof meta.cancellable === "boolean") return meta.cancellable;
  if ((meta.backgroundJobs ?? 0) > 0 && !meta.pendingPrompt) return false;
  return Boolean(meta.running);
}

export function metaFromTab(tab: TabMeta, existing?: Meta): Meta {
  const cwd = tab.cwd || tab.workspaceRoot || existing?.cwd || "";
  const toolApprovalMode = normalizeToolApprovalMode(
    tab.toolApprovalMode,
    normalizeMode(tab.mode),
    modeHasAutoApproveTools(tab.mode),
    (tab.toolApprovalMode ?? "").trim() === "" ? existing?.toolApprovalMode : undefined,
  );
  const autoApproveTools = toolApprovalMode === "yolo";
  return {
    label: tab.label || existing?.label || "",
    modelRef: tab.modelRef?.trim() || existing?.modelRef,
    ready: tab.ready,
    startupErr: tab.startupErr,
    sendAvailability: tab.sendAvailability,
    eventChannel: existing?.eventChannel ?? "agent:event",
    cwd,
    workspaceRoot: tab.workspaceRoot || existing?.workspaceRoot || cwd,
    workspaceName: tab.workspaceName || existing?.workspaceName,
    workspacePath: tab.workspacePath || tab.workspaceRoot || existing?.workspacePath,
    sessionPath: tab.sessionPath !== undefined ? tab.sessionPath : existing?.sessionPath,
    gitBranch: tab.gitBranch || existing?.gitBranch,
    autoApproveTools,
    bypass: autoApproveTools,
    collaborationMode: tab.collaborationMode ?? existing?.collaborationMode ?? "normal",
    toolApprovalMode,
    tokenMode: tab.tokenMode ?? existing?.tokenMode ?? "full",
    goal: tab.goal ?? existing?.goal,
    goalStatus: tab.goalStatus ?? existing?.goalStatus,
  };
}

export function sameMeta(a?: Meta, b?: Meta): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  return (
    a.label === b.label &&
    a.ready === b.ready &&
    a.startupErr === b.startupErr &&
    a.sendAvailability?.state === b.sendAvailability?.state &&
    a.sendAvailability?.retryable === b.sendAvailability?.retryable &&
    a.sendAvailability?.detail === b.sendAvailability?.detail &&
    a.sendAvailability?.action === b.sendAvailability?.action &&
    a.eventChannel === b.eventChannel &&
    a.cwd === b.cwd &&
    a.workspaceRoot === b.workspaceRoot &&
    a.workspaceName === b.workspaceName &&
    a.workspacePath === b.workspacePath &&
    a.sessionPath === b.sessionPath &&
    a.gitBranch === b.gitBranch &&
    a.autoApproveTools === b.autoApproveTools &&
    a.bypass === b.bypass &&
    a.collaborationMode === b.collaborationMode &&
    a.toolApprovalMode === b.toolApprovalMode &&
    a.tokenMode === b.tokenMode &&
    a.goal === b.goal &&
    a.goalStatus === b.goalStatus
  );
}

export const STALE_TURN_RECONCILE_MS = 30_000;

export function shouldReconcileStaleTurn(
  state: { running: boolean; turnActive: boolean } | undefined,
  lastTurnActivityAt: number,
  now = Date.now(),
  timeoutMs = STALE_TURN_RECONCILE_MS,
): boolean {
  if (!state?.running || !state.turnActive || lastTurnActivityAt <= 0) return false;
  return Math.max(0, now - lastTurnActivityAt) >= timeoutMs;
}
