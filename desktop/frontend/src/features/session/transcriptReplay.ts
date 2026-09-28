import { app } from "../../lib/bridge";
import type { TranscriptReplay } from "../../lib/types";
import type { State } from "./sessionState";

export function transcriptReplayNeedsFullRestore(
  replay: TranscriptReplay,
  state: Pick<State, "transcriptJournalId" | "transcriptThrough" | "transcriptSeen"> | undefined,
): boolean {
  if (replay.after === undefined) return false;
  if (!state || replay.journalId !== state.transcriptJournalId || state.transcriptThrough < replay.after) return true;
  const projectedThrough = Object.keys(state.transcriptSeen)
    .reduce((latest, sequence) => Math.max(latest, Number(sequence)), state.transcriptThrough);
  // A missed start/message/tool boundary cannot be applied after newer deltas:
  // it would reset their live stream, then dedup would skip restoring them.
  // Rebuild in journal order only when the delta fills such an earlier gap.
  return replay.events.some((event) => {
    const sequence = event.transcriptSequence ?? 0;
    return sequence > state.transcriptThrough
      && sequence <= projectedThrough
      && !state.transcriptSeen[sequence];
  });
}

export async function readTranscriptReplay(
  tabId: string,
  isCurrent: () => boolean,
  cursor?: { journalId: string; through: number },
): Promise<TranscriptReplay | undefined> {
  if (typeof app.TranscriptPageForTab !== "function") return undefined;
  let page;
  try {
    page = await app.TranscriptPageForTab(tabId, cursor?.journalId ?? "", cursor?.through ?? 0, 0);
  } catch (error) {
    if (!cursor || !isCurrent()) throw error;
    // Rewind/summarize may replace the journal without changing the task.
    cursor = undefined;
    page = await app.TranscriptPageForTab(tabId, "", 0, 0);
  }
  if (!page || !isCurrent()) return undefined;
  const replay: TranscriptReplay = {
    journalId: page.journalId,
    after: cursor?.through,
    through: page.through,
    messages: page.baseline ?? [],
    events: [],
  };
  for (;;) {
    for (const entry of page.entries) {
      replay.events.push({
        ...entry.payload,
        tabId,
        operational: entry.payload.operational
          ? { ...entry.payload.operational, scope: { ...entry.payload.operational.scope, tabId } }
          : undefined,
        transcriptJournalId: replay.journalId,
        transcriptSequence: entry.sequence,
      });
    }
    if (!page.hasMore) return replay;
    const after = page.next;
    page = await app.TranscriptPageForTab(tabId, replay.journalId, after, replay.through);
    if (!isCurrent()) return undefined;
    if (!page || page.journalId !== replay.journalId || page.through !== replay.through || page.next <= after) {
      throw new Error("任务回显在恢复期间发生变化，请重新打开任务。");
    }
  }
}
