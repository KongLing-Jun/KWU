import type { ArtifactDelivery, MemoryCitation, TaskLifecycle, TurnMetadata } from "../../lib/types";
import type { ToolFileDiff } from "../../lib/tools";

export type ToolStatus = "running" | "done" | "error" | "stopped";
export type LiveStream = { id: string; text: string; reasoning: string; reasoningComplete: boolean };
export type MessageActionScope = "fork" | "summ-from" | "summ-upto" | "conversation" | "code" | "both";
export type MessageActionState = { turn: number; scope: MessageActionScope };

export type Item =
  | { kind: "user"; id: string; text: string; submitText?: string; failed?: boolean; createdAt?: number; parentId?: string }
  | { kind: "assistant"; id: string; text: string; reasoning: string; streaming: boolean; reasoningComplete?: boolean; memoryCitations?: MemoryCitation[]; finalAnswer?: boolean; artifactDelivery?: ArtifactDelivery; artifactPaths?: string[]; turn?: TurnMetadata; parentId?: string; runtimeTurnId?: string; runtimeCandidateId?: string }
  | { kind: "phase"; id: string; text: string; parentId?: string }
  | { kind: "notice"; id: string; level: "info" | "warn"; text: string; parentId?: string }
  | {
      kind: "compaction";
      id: string;
      pending: boolean;
      trigger: string;
      messages: number;
      summary: string;
      archive: string;
      parentId?: string;
    }
  | {
      kind: "tool";
      id: string;
      name: string;
      args: string;
      readOnly: boolean;
      status: ToolStatus;
      preparing?: boolean;
      output?: string;
      error?: string;
      truncated?: boolean;
      dataArchived?: boolean;
      artifactPaths?: string[];
      durationMs?: number;
      subject?: string;
      summary?: string;
      fileDiff?: ToolFileDiff;
      isShell?: boolean;
      parentId?: string;
      taskTreeId?: string;
      nodeId?: string;
      profile?: { model?: string; effort?: string };
      subagentRef?: string;
      taskLifecycle?: TaskLifecycle;
      taskLifecycleRefs?: string[];
      taskLifecycleReceipts?: TaskLifecycle[];
      sourceToolId?: string;
    };

export type ToolItem = Extract<Item, { kind: "tool" }>;
type NoticeItem = Extract<Item, { kind: "notice" }>;

// MCP startup is best-effort: unavailable servers are omitted from the current
// tool registry and remain inspectable in the connector management UI. Keeping
// these transport details out of the transcript prevents an optional connector
// from presenting as a failed chat turn, including when older history is loaded.
export function isSilentMCPAvailabilityNotice(text: string): boolean {
  const normalized = text.trim();
  return /^mcp \S+: [\s\S]+$/.test(normalized)
    || /^\d+ MCP server\(s\) failed to start: [\s\S]+ — run \/mcp for details$/.test(normalized);
}

const consumedRevealViewsByNotice = new WeakMap<NoticeItem, Set<string>>();

export function markNoticeForReveal(item: NoticeItem): NoticeItem {
  if (!consumedRevealViewsByNotice.has(item)) consumedRevealViewsByNotice.set(item, new Set());
  return item;
}

export function noticeNeedsReveal(item: Item): item is NoticeItem {
  return item.kind === "notice" && consumedRevealViewsByNotice.has(item);
}

export function consumeNoticeReveal(item: NoticeItem, semanticViewKey: string): boolean {
  const consumedViews = consumedRevealViewsByNotice.get(item);
  if (!consumedViews || consumedViews.has(semanticViewKey)) return false;
  consumedViews.add(semanticViewKey);
  return true;
}

/** Mirrors the Go backend's ReadOnly() hints. */
export function isReadOnlyTool(name: string): boolean {
  switch (name) {
    case "read_file":
    case "ls":
    case "grep":
    case "glob":
    case "web_fetch":
    case "code_index":
    case "bash_output":
    case "waitJob":
    case "todo_write":
    case "read_skill":
    case "deliver_files":
      return true;
    default:
      return false;
  }
}
