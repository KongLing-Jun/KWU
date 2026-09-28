import type { ProjectNode, TabMeta } from "./types";

export type ConversationUnreadCompletions = Record<string, number>;

type ConversationIdentitySource = {
  kind?: ProjectNode["kind"];
  root?: ProjectNode["root"];
  scope?: TabMeta["scope"];
  workspaceRoot?: TabMeta["workspaceRoot"];
  topicId?: string;
  sessionPath?: string;
};

type AttentionStorage = Pick<Storage, "getItem" | "setItem">;

const STORAGE_PREFIX = "reasonix:conversation-unread-completions:v1:";

function storageKey(accountScope: string): string {
  return `${STORAGE_PREFIX}${encodeURIComponent(accountScope)}`;
}

export function conversationAttentionAccountScope(profileId?: string, cloudUserId?: string): string {
  const profile = profileId?.trim() || "local";
  return JSON.stringify(["profile", profile, cloudUserId?.trim() || "anonymous"]);
}

export function conversationAttentionIdentity(source?: ConversationIdentitySource | null): string {
  if (!source) return "";
  const sessionPath = source.sessionPath?.trim() ?? "";
  if (sessionPath) return JSON.stringify(["session-path", sessionPath]);

  const topicId = source.topicId?.trim() ?? "";
  if (!topicId) return "";
  const sourceWithTabScope = source as Pick<TabMeta, "scope" | "workspaceRoot">;
  const sourceWithNodeKind = source as Pick<ProjectNode, "kind" | "root">;
  const scope = sourceWithTabScope.scope === "project"
    || sourceWithNodeKind.kind === "topic"
    || sourceWithNodeKind.kind === "session"
    ? "project"
    : "global";
  const workspaceRoot = scope === "project"
    ? (sourceWithTabScope.workspaceRoot ?? sourceWithNodeKind.root ?? "").trim()
    : "";
  return JSON.stringify(["topic", scope, workspaceRoot, topicId]);
}

export function readConversationUnreadCompletions(
  accountScope: string,
  storage: AttentionStorage | undefined = typeof localStorage === "undefined" ? undefined : localStorage,
): ConversationUnreadCompletions {
  if (!accountScope || !storage) return {};
  try {
    const parsed = JSON.parse(storage.getItem(storageKey(accountScope)) ?? "{}") as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    const records: ConversationUnreadCompletions = {};
    for (const [identity, completedAt] of Object.entries(parsed)) {
      if (identity && typeof completedAt === "number" && Number.isFinite(completedAt) && completedAt > 0) {
        records[identity] = completedAt;
      }
    }
    return records;
  } catch {
    return {};
  }
}

export function writeConversationUnreadCompletions(
  accountScope: string,
  records: ConversationUnreadCompletions,
  storage: AttentionStorage | undefined = typeof localStorage === "undefined" ? undefined : localStorage,
): void {
  if (!accountScope || !storage) return;
  try {
    storage.setItem(storageKey(accountScope), JSON.stringify(records));
  } catch {
    // Persistence is best-effort when web storage is disabled or full.
  }
}

export function withUnreadCompletion(
  records: ConversationUnreadCompletions,
  identity: string,
  completedAt: number,
): ConversationUnreadCompletions {
  if (!identity || !Number.isFinite(completedAt) || completedAt <= 0) return records;
  if (records[identity] === completedAt) return records;
  return { ...records, [identity]: completedAt };
}

export function withoutUnreadCompletion(
  records: ConversationUnreadCompletions,
  identity: string,
): ConversationUnreadCompletions {
  if (!identity || !Object.prototype.hasOwnProperty.call(records, identity)) return records;
  const next = { ...records };
  delete next[identity];
  return next;
}

export function shouldMarkTurnDoneUnread(
  turnConversationIdentity: string,
  activeConversationIdentity: string,
  error?: string,
  interrupted = false,
): boolean {
  return Boolean(
    turnConversationIdentity
    && !error
    && !interrupted
    && turnConversationIdentity !== activeConversationIdentity
  );
}
