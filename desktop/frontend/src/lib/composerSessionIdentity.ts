import type {
  CollaborationMode,
  ComposerSessionIdentity,
  GoalStatus,
  TokenMode,
  ToolApprovalMode,
} from "./types";

type ComposerSessionIdentitySource = Partial<Omit<ComposerSessionIdentity, "expertKnown">>;

function normalizeCollaborationMode(value: unknown): CollaborationMode {
  return value === "plan" || value === "goal" ? value : "normal";
}

function normalizeToolApprovalMode(value: unknown): ToolApprovalMode {
  return value === "auto" || value === "yolo" ? value : "ask";
}

function normalizeTokenMode(value: unknown): TokenMode {
  return value === "economy" ? value : "full";
}

function normalizeGoalStatus(value: unknown): GoalStatus {
  return value === "running" || value === "complete" || value === "blocked" ? value : "stopped";
}

export function composerSessionIdentityFromSource(
  source: ComposerSessionIdentitySource,
  expertKnown = true,
): ComposerSessionIdentity {
  const billingSource = source.billingSource === "subscription_pool" || source.billingSource === "newapi_byok"
    ? source.billingSource
    : "";
  return {
    modelRef: source.modelRef?.trim() ?? "",
    billingSource,
    effort: typeof source.effort === "string" ? source.effort.trim() : undefined,
    expertKnown,
    expertId: source.expertId?.trim() ?? "",
    expertName: source.expertName?.trim() ?? "",
    collaborationMode: normalizeCollaborationMode(source.collaborationMode),
    toolApprovalMode: normalizeToolApprovalMode(source.toolApprovalMode),
    tokenMode: normalizeTokenMode(source.tokenMode),
    goal: source.goal?.trim() ?? "",
    goalStatus: normalizeGoalStatus(source.goalStatus),
  };
}

export function runtimeComposerSessionIdentity(
  source: ComposerSessionIdentitySource,
  previous?: ComposerSessionIdentity,
): ComposerSessionIdentity {
  const runtime = composerSessionIdentityFromSource(source, false);
  if (runtime.expertId) {
    return { ...runtime, expertKnown: true };
  }
  return {
    ...runtime,
    expertKnown: previous?.expertKnown ?? false,
    expertId: previous?.expertId ?? "",
    expertName: previous?.expertName ?? "",
  };
}
