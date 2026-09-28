import type { WireEvent } from "../../lib/types";

export interface RoutedStreamEvent {
  tabId: string;
  e: WireEvent;
}

interface CoalescedEvent {
  routed: RoutedStreamEvent;
  lastIndex: number;
}

export const STREAM_BATCH_DISABLE_STORAGE_KEY = "kwu.performance.disableStreamBatch";

interface StorageReader {
  getItem(key: string): string | null;
}

function reasoningDelta(event: WireEvent): string {
  return event.text ?? event.reasoning ?? "";
}

function reasoningDestination(event: WireEvent): "text" | "reasoning" {
  if (typeof event.text === "string") return "text";
  if (typeof event.reasoning === "string") return "reasoning";
  return "text";
}

export function isStreamBatchingEnabled(storage?: StorageReader): boolean {
  const source = storage ?? (typeof localStorage !== "undefined" ? localStorage : undefined);
  if (!source) return true;
  try {
    return source.getItem(STREAM_BATCH_DISABLE_STORAGE_KEY) !== "1";
  } catch {
    return true;
  }
}

// Coalesce every text/reasoning key to one event while ordering the result by
// each key's last occurrence. That preserves the final reasoningComplete
// semantics for interleaved reasoning/text without dispatching every delta.
export function coalesceRoutedStreamEvents(batch: RoutedStreamEvent[]): RoutedStreamEvent[] {
  const byKey = new Map<string, CoalescedEvent>();
  batch.forEach((routed, index) => {
    const { e, tabId } = routed;
    const parentId = e.parentId?.trim() ?? "";
    const subagentRef = e.subagentRef?.trim() ?? "";
    const turnId = e.turnId?.trim() ?? "";
    const candidateId = e.candidateId?.trim() ?? "";
    // Durable deltas retain their individual cursor for replay deduplication.
    // They still share one animation-frame dispatch, without merging payloads.
    const cursor = e.transcriptJournalId && e.transcriptSequence
      ? `\u0000${e.transcriptJournalId}\u0000${e.transcriptSequence}`
      : "";
    const key = `${tabId}\u0000${turnId}\u0000${candidateId}\u0000${parentId}\u0000${subagentRef}\u0000${e.kind}${cursor}`;
    const existing = byKey.get(key);
    if (!existing) {
      byKey.set(key, {
        routed: { tabId, e: { ...e } },
        lastIndex: index,
      });
      return;
    }
    if (cursor) return;
    const field = e.kind === "reasoning" ? reasoningDestination(existing.routed.e) : "text";
    const delta = e.kind === "reasoning" ? reasoningDelta(e) : (e.text ?? "");
    existing.routed.e[field] = (existing.routed.e[field] ?? "") + delta;
    existing.lastIndex = index;
  });
  return [...byKey.values()]
    .sort((left, right) => left.lastIndex - right.lastIndex)
    .map(({ routed }) => routed);
}
