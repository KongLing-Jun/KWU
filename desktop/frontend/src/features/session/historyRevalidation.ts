import type { HistoryMessage } from "../../lib/types";
import type { Item } from "./sessionItems";
import { preserveTurnBilling } from "./turnBilling";

const FNV_OFFSET = 0x811c9dc5;
const FNV_PRIME = 0x01000193;

export type HistoryRevalidationRequestGuard = {
  tabId: string;
  activationSeq: number;
  identityKey: string;
  loadSeq: number;
  items: Item[] | undefined;
};

export type HistoryRevalidationCurrentGuard = {
  activeTabId: string | undefined;
  activationSeq: number;
  identityKey: string | undefined;
  loadSeq: number | undefined;
  items: Item[] | undefined;
  hasActiveRuntimeTail: boolean;
};

type ChannelSessionRefreshTab = {
  id: string;
  readOnly?: boolean;
  sessionPath?: string;
};

export function channelSessionRefreshTarget(
  tab: ChannelSessionRefreshTab | undefined,
): { tabId: string; sessionPath: string } | undefined {
  const sessionPath = tab?.sessionPath?.trim() ?? "";
  if (!tab?.id || tab.readOnly !== true || !sessionPath) return undefined;
  return { tabId: tab.id, sessionPath };
}

function hashText(hash: number, value: string): number {
  let next = hash;
  for (let index = 0; index < value.length; index += 1) {
    next ^= value.charCodeAt(index);
    next = Math.imul(next, FNV_PRIME);
  }
  return next >>> 0;
}

function hashValue(hash: number, value: unknown): number {
  if (value === null) return hashText(hash, "null;");
  if (value === undefined) return hashText(hash, "undefined;");
  if (typeof value === "string") return hashText(hashText(hash, "s:"), `${value};`);
  if (typeof value === "number") return hashText(hashText(hash, "n:"), `${value};`);
  if (typeof value === "boolean") return hashText(hash, value ? "b:1;" : "b:0;");
  if (Array.isArray(value)) {
    let next = hashText(hash, "[");
    for (const item of value) next = hashValue(next, item);
    return hashText(next, "]");
  }
  if (typeof value === "object") {
    let next = hashText(hash, "{");
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      next = hashText(next, key);
      next = hashValue(next, item);
    }
    return hashText(next, "}");
  }
  return hashText(hash, `${typeof value};`);
}

export function historyMessagesFingerprint(messages: HistoryMessage[]): string {
  let textLength = 0;
  for (const message of messages) {
    textLength += message.content.length;
    textLength += message.submitText?.length ?? 0;
    textLength += message.reasoning?.length ?? 0;
    textLength += message.summary?.length ?? 0;
  }
  return `${messages.length}:${textLength}:${hashValue(FNV_OFFSET, messages).toString(16)}`;
}

export function prepareHistoryRevalidation(currentFingerprint: string | undefined, messages: HistoryMessage[]) {
  const fingerprint = historyMessagesFingerprint(messages);
  return {
    changed: currentFingerprint !== fingerprint,
    fingerprint,
    messages,
  };
}

export function historyRevalidationRequestIsCurrent(
  request: HistoryRevalidationRequestGuard,
  current: HistoryRevalidationCurrentGuard,
): boolean {
  return current.activeTabId === request.tabId
    && current.activationSeq === request.activationSeq
    && current.identityKey === request.identityKey
    && current.loadSeq === request.loadSeq
    && current.items === request.items
    && !current.hasActiveRuntimeTail;
}

export function mergeRevalidatedHistoryItems(previous: Item[], next: Item[]): Item[] {
  next = preserveTurnBilling(previous, next);
  const shared = Math.min(previous.length, next.length);
  let prefix = 0;
  while (
    prefix < shared
    && hashValue(FNV_OFFSET, previous[prefix]) === hashValue(FNV_OFFSET, next[prefix])
  ) {
    prefix += 1;
  }
  if (prefix === previous.length && prefix === next.length) return previous;
  if (prefix === previous.length) return [...previous, ...next.slice(prefix)];
  return carryRuntimeAssistantIdentity(previous, next);
}

function semanticallyEqualItem(left: Item, right: Item): boolean {
  if (left.kind !== right.kind) return false;
  if (left.kind === "user" && right.kind === "user") {
    return left.text === right.text && left.submitText === right.submitText;
  }
  if (left.kind === "assistant" && right.kind === "assistant") {
    return left.streaming === right.streaming
      && left.text === right.text
      && left.reasoning === right.reasoning;
  }
  if (left.kind === "tool" && right.kind === "tool") {
    return left.name === right.name
      && left.args === right.args
      && left.output === right.output
      && left.error === right.error
      && left.status === right.status;
  }
  const { id: _leftID, ...leftValue } = left;
  const { id: _rightID, ...rightValue } = right;
  return hashValue(FNV_OFFSET, leftValue) === hashValue(FNV_OFFSET, rightValue);
}

function carryRuntimeAssistantIdentity(previous: Item[], next: Item[]): Item[] {
  let changed = false;
  const carried = next.map((item, index) => {
    const prior = previous[index];
    if (
      item.kind !== "assistant"
      || prior?.kind !== "assistant"
      || !semanticallyEqualItem(prior, item)
      || (!prior.runtimeTurnId && !prior.runtimeCandidateId)
    ) return item;
    changed = true;
    return {
      ...item,
      runtimeTurnId: prior.runtimeTurnId,
      runtimeCandidateId: prior.runtimeCandidateId,
    };
  });
  return changed ? carried : next;
}

export function mergeRuntimeTailItems(history: Item[], tail: Item[]): Item[] {
  const maxOverlap = Math.min(history.length, tail.length);
  let overlap = maxOverlap;
  while (overlap > 0) {
    const historyStart = history.length - overlap;
    let matches = true;
    for (let index = 0; index < overlap; index += 1) {
      if (!semanticallyEqualItem(history[historyStart + index], tail[index])) {
        matches = false;
        break;
      }
    }
    if (matches) break;
    overlap -= 1;
  }
  if (overlap > 0) {
    const historyStart = history.length - overlap;
    history = history.map((item, index) => {
      if (index < historyStart) return item;
      const runtime = tail[index - historyStart];
      if (
        item.kind !== "assistant"
        || runtime?.kind !== "assistant"
        || (!runtime.runtimeTurnId && !runtime.runtimeCandidateId)
      ) return item;
      return {
        ...item,
        id: runtime.id,
        runtimeTurnId: runtime.runtimeTurnId,
        runtimeCandidateId: runtime.runtimeCandidateId,
      };
    });
  }
  if (overlap === tail.length) return history;
  return [...history, ...tail.slice(overlap)];
}
