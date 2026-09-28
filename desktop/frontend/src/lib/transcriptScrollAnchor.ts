import { questionAnchorId } from "./transcriptGrouping";

export type TranscriptQuestionAnchor = { id: string; text?: string };

export type CapturedTranscriptAnchor = {
  anchorMessageId: string;
  offset: number;
};

export type TranscriptMiddleAnchor = CapturedTranscriptAnchor & { atBottom: false };
export type TranscriptPresentationPath = "normal" | "team-pending" | "team-legacy" | "team-persisted";
export type TranscriptSemanticPosition = { viewKey: string; anchor: TranscriptMiddleAnchor };
export type TranscriptCommittedPresentation = { viewKey: string; path: TranscriptPresentationPath };
export type TranscriptPresentationRestoreSnapshot = { viewKey: string; anchor: TranscriptMiddleAnchor | null };

export function transcriptAnchorRestoreGeometryStable({
  targetOffset,
  measuredOffset,
  scrollTop,
  maxScrollTop,
  layoutStable,
  hasMeasuredVirtualGeometry,
  tolerance = 1,
}: {
  targetOffset: number;
  measuredOffset: number | undefined;
  scrollTop: number;
  maxScrollTop: number;
  layoutStable: boolean;
  hasMeasuredVirtualGeometry: boolean;
  tolerance?: number;
}): boolean {
  if (!layoutStable || measuredOffset === undefined) return false;
  const correction = targetOffset - measuredOffset;
  if (Math.abs(correction) <= tolerance) return true;
  if (!hasMeasuredVirtualGeometry) return false;
  return (correction < 0 && scrollTop <= tolerance)
    || (correction > 0 && maxScrollTop - scrollTop <= tolerance);
}

function cloneMiddleAnchor(anchor: TranscriptMiddleAnchor): TranscriptMiddleAnchor {
  return { atBottom: false, anchorMessageId: anchor.anchorMessageId, offset: anchor.offset };
}

export function remapTranscriptMiddleAnchor(
  previousQuestions: readonly TranscriptQuestionAnchor[],
  nextQuestions: readonly TranscriptQuestionAnchor[],
  anchor: TranscriptMiddleAnchor,
): TranscriptMiddleAnchor {
  if (nextQuestions.some((question) => question.id === anchor.anchorMessageId)) return anchor;
  const previousIndex = previousQuestions.findIndex((question) => question.id === anchor.anchorMessageId);
  if (previousIndex < 0) return anchor;
  const text = previousQuestions[previousIndex]?.text;
  if (text === undefined) return anchor;
  const occurrence = previousQuestions.slice(0, previousIndex + 1).filter((question) => question.text === text).length - 1;
  const replacement = nextQuestions.filter((question) => question.text === text)[occurrence];
  return replacement
    ? { ...anchor, anchorMessageId: replacement.id }
    : anchor;
}

export function stagePresentationRestoreSnapshot(
  committed: TranscriptCommittedPresentation,
  viewKey: string,
  latest: TranscriptSemanticPosition | null,
  hasPersistedTeam: boolean,
  followingBottom = false,
): TranscriptPresentationRestoreSnapshot | null {
  if (!hasPersistedTeam || committed.viewKey !== viewKey
    || (committed.path !== "team-legacy" && committed.path !== "normal")) return null;
  const anchor = followingBottom ? null : latest?.viewKey === viewKey ? latest.anchor : undefined;
  if (anchor === undefined) return null;
  return {
    viewKey,
    anchor: anchor ? cloneMiddleAnchor(anchor) : null,
  };
}

export function syncPresentationRestoreSnapshot(
  snapshot: TranscriptPresentationRestoreSnapshot | null,
  viewKey: string,
  presentationPath: TranscriptPresentationPath,
  anchor: TranscriptMiddleAnchor | null,
): TranscriptPresentationRestoreSnapshot | null {
  if (!snapshot || snapshot.viewKey !== viewKey
    || (presentationPath !== "team-legacy" && presentationPath !== "normal")) return snapshot;
  return { viewKey, anchor: anchor ? cloneMiddleAnchor(anchor) : null };
}

export function consumePresentationRestoreSnapshot(
  snapshot: TranscriptPresentationRestoreSnapshot | null,
  viewKey: string,
  presentationPath: TranscriptPresentationPath,
): { anchor: TranscriptMiddleAnchor | null } | undefined {
  if (!snapshot || snapshot.viewKey !== viewKey || presentationPath !== "team-persisted") return undefined;
  return { anchor: snapshot.anchor ? cloneMiddleAnchor(snapshot.anchor) : null };
}

export function shouldAutoScrollTranscriptQuestions(
  previousViewKey: string | undefined,
  currentViewKey: string,
  previousRevalidationSignal: number,
  currentRevalidationSignal: number,
): boolean {
  return previousViewKey === currentViewKey
    && previousRevalidationSignal === currentRevalidationSignal;
}

export function findTranscriptAnchorNode(container: HTMLElement, messageID: string): HTMLElement | null {
  const expectedID = questionAnchorId(messageID);
  for (const candidate of container.querySelectorAll<HTMLElement>("[id]")) {
    if (candidate.id === expectedID) return candidate;
  }
  return null;
}

export function captureTranscriptAnchor(
  container: HTMLElement,
  questions: readonly TranscriptQuestionAnchor[],
  threshold = 12,
): CapturedTranscriptAnchor | undefined {
  const questionsByDOMID = new Map(questions.map((question) => [questionAnchorId(question.id), question]));
  const nodesByMessageID = new Map<string, HTMLElement>();
  for (const candidate of container.querySelectorAll<HTMLElement>("[id]")) {
    const question = questionsByDOMID.get(candidate.id);
    if (question) nodesByMessageID.set(question.id, candidate);
  }

  const containerTop = container.getBoundingClientRect().top;
  let captured: CapturedTranscriptAnchor | undefined;
  let first: CapturedTranscriptAnchor | undefined;
  for (const question of questions) {
    const node = nodesByMessageID.get(question.id);
    if (!node) continue;
    const relativeTop = node.getBoundingClientRect().top - containerTop;
    first ??= { anchorMessageId: question.id, offset: -relativeTop };
    if (relativeTop > threshold) break;
    captured = { anchorMessageId: question.id, offset: -relativeTop };
  }
  return captured ?? first;
}

export function captureVirtualTranscriptAnchor(
  questions: readonly TranscriptQuestionAnchor[],
  rowByAnchorID: ReadonlyMap<string, number>,
  visualOffsetForRow: (row: number) => number | undefined,
  threshold = 12,
): CapturedTranscriptAnchor | undefined {
  let captured: CapturedTranscriptAnchor | undefined;
  let first: CapturedTranscriptAnchor | undefined;
  for (const question of questions) {
    const row = rowByAnchorID.get(questionAnchorId(question.id));
    if (row === undefined) continue;
    const offset = visualOffsetForRow(row);
    if (offset === undefined) continue;
    const candidate = { anchorMessageId: question.id, offset };
    first ??= candidate;
    if (-offset > threshold) break;
    captured = candidate;
  }
  return captured ?? first;
}

export function scrollTopForTranscriptNode(
  container: HTMLElement,
  node: HTMLElement,
  visualOffset = 0,
): number {
  const relativeTop = node.getBoundingClientRect().top - container.getBoundingClientRect().top;
  return Math.max(0, container.scrollTop + relativeTop - visualOffset);
}

export function restoreTranscriptAnchor(
  container: HTMLElement,
  messageID: string,
  offset: number,
): boolean {
  const node = findTranscriptAnchorNode(container, messageID);
  if (!node) return false;
  container.scrollTop = scrollTopForTranscriptNode(container, node, -offset);
  return true;
}
