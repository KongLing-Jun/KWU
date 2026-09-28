export type GeneratedMediaReference = {
  type: "image" | "video";
  path?: string;
  url?: string;
  mimeType?: string;
};

export type VideoGenerationProgress = {
  taskId: string;
  status: string;
  progressPercent: number;
};

type MediaTranscriptItem =
  | { kind: "user"; id: string }
  | { kind: "assistant"; id: string; text: string; streaming?: boolean }
  | { kind: "tool"; id: string; name: string; output?: string }
  | { kind: string; id: string };

const noRestoredMediaOutputs: ReadonlyMap<string, string> = new Map();

export function isGeneratedMediaTool(name: string): boolean {
  return name === "generate_image" || name === "generate_video";
}

function safeRemoteURL(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  try {
    const parsed = new URL(value.trim());
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.toString() : undefined;
  } catch {
    return undefined;
  }
}

export function parseGeneratedMediaResult(output?: string): GeneratedMediaReference[] {
  if (!output?.trim()) return [];
  let decoded: unknown;
  try {
    decoded = JSON.parse(output);
  } catch {
    return [];
  }
  if (!decoded || typeof decoded !== "object") return [];
  const rawMedia = (decoded as { media?: unknown }).media;
  if (!Array.isArray(rawMedia)) return [];
  const results: GeneratedMediaReference[] = [];
  const seen = new Set<string>();
  for (const raw of rawMedia) {
    if (!raw || typeof raw !== "object") continue;
    const entry = raw as Record<string, unknown>;
    const type = entry.type === "image" || entry.type === "video" ? entry.type : null;
    const path = typeof entry.path === "string" && entry.path.trim() ? entry.path.trim() : undefined;
    const url = safeRemoteURL(entry.url);
    const mimeType = typeof entry.mime_type === "string" ? entry.mime_type.trim() : undefined;
    if (!type || (!path && !url)) continue;
    const key = `${type}\0${path ?? ""}\0${url ?? ""}`;
    if (seen.has(key)) continue;
    seen.add(key);
    results.push({ type, path, url, mimeType });
  }
  return results;
}

/**
 * Associates generated media with the final assistant message in each user
 * turn. A later media call clears an earlier assistant candidate so paths are
 * always rendered after the last generation step, at the end of the answer.
 */
export function generatedMediaByAssistant(
  items: readonly MediaTranscriptItem[],
  restoredOutputs: ReadonlyMap<string, string> = noRestoredMediaOutputs,
): ReadonlyMap<string, GeneratedMediaReference[]> {
  const byAssistant = new Map<string, GeneratedMediaReference[]>();
  let media: GeneratedMediaReference[] = [];
  let mediaKeys = new Set<string>();
  let finalAssistantId = "";

  const finishTurn = () => {
    if (finalAssistantId && media.length > 0) {
      byAssistant.set(finalAssistantId, media);
    }
    media = [];
    mediaKeys = new Set<string>();
    finalAssistantId = "";
  };

  for (const item of items) {
    if (item.kind === "user") {
      finishTurn();
      continue;
    }
    if (item.kind === "tool" && "name" in item && isGeneratedMediaTool(item.name)) {
      for (const reference of parseGeneratedMediaResult(item.output ?? restoredOutputs.get(item.id))) {
        const key = `${reference.type}\0${reference.path ?? ""}\0${reference.url ?? ""}`;
        if (mediaKeys.has(key)) continue;
        mediaKeys.add(key);
        media.push(reference);
      }
      finalAssistantId = "";
      continue;
    }
    if (
      item.kind === "assistant"
      && "text" in item
      && (item.streaming === true || item.text.trim() !== "")
      && media.length > 0
    ) {
      finalAssistantId = item.id;
    }
  }
  finishTurn();
  return byAssistant;
}

export function parseVideoGenerationProgress(output?: string): VideoGenerationProgress | null {
  if (!output?.trim()) return null;
  const lines = output.trim().split(/\r?\n/);
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    try {
      const decoded: unknown = JSON.parse(lines[index]);
      if (!decoded || typeof decoded !== "object") continue;
      const progress = decoded as Record<string, unknown>;
      if (progress.kind !== "video_generation_progress") continue;
      const taskId = typeof progress.task_id === "string" ? progress.task_id.trim() : "";
      const status = typeof progress.status === "string" ? progress.status.trim() : "";
      const rawPercent = typeof progress.progress_percent === "number" ? progress.progress_percent : Number.NaN;
      if (!taskId || !Number.isFinite(rawPercent)) return null;
      return {
        taskId,
        status,
        progressPercent: Math.min(100, Math.max(0, Math.round(rawPercent))),
      };
    } catch {
      continue;
    }
  }
  return null;
}
