export type ComposerExpertTraceData = Record<string, unknown>;

export type ComposerExpertTraceEvent = {
  traceId: string;
  event: string;
  elapsedMs: number;
  at: number;
  data: ComposerExpertTraceData;
};

type ComposerExpertTraceWindowApi = {
  events: () => ComposerExpertTraceEvent[];
  export: () => string;
  clear: () => void;
  currentTraceId: () => string | undefined;
};

declare global {
  interface Window {
    __composerExpertTrace?: ComposerExpertTraceWindowApi;
  }
}

const TRACE_PREFIX = "[composer-expert-trace]";
const TRACE_EVENT_LIMIT = 1_000;
const traceEnabled = Boolean(import.meta.env?.DEV);
const traceEvents: ComposerExpertTraceEvent[] = [];
const traceStartedAt = new Map<string, number>();
let activeTraceId: string | undefined;
let traceSequence = 0;

function monotonicNow(): number {
  return typeof performance === "undefined" ? Date.now() : performance.now();
}

function roundMilliseconds(value: number): number {
  return Math.round(value * 100) / 100;
}

function publishTraceEvent(entry: ComposerExpertTraceEvent): void {
  traceEvents.push(entry);
  if (traceEvents.length > TRACE_EVENT_LIMIT) traceEvents.splice(0, traceEvents.length - TRACE_EVENT_LIMIT);
  const line = `${TRACE_PREFIX} ${JSON.stringify(entry)}`;
  console.debug(line);
  if (typeof window !== "undefined") {
    const runtime = window.runtime as (typeof window.runtime & { LogInfo?: (message: string) => void }) | undefined;
    runtime?.LogInfo?.(line);
  }
}

function installWindowApi(): void {
  if (!traceEnabled || typeof window === "undefined" || window.__composerExpertTrace) return;
  window.__composerExpertTrace = {
    events: () => traceEvents.map((entry) => ({ ...entry, data: { ...entry.data } })),
    export: () => JSON.stringify(traceEvents, null, 2),
    clear: () => {
      traceEvents.length = 0;
      traceStartedAt.clear();
      activeTraceId = undefined;
    },
    currentTraceId: () => activeTraceId,
  };
}

export function beginComposerExpertTrace(data: ComposerExpertTraceData): string | undefined {
  if (!traceEnabled) return undefined;
  installWindowApi();
  const startedAt = monotonicNow();
  const traceId = `${Date.now()}-${++traceSequence}`;
  activeTraceId = traceId;
  traceStartedAt.set(traceId, startedAt);
  publishTraceEvent({ traceId, event: "project-tree.click", elapsedMs: 0, at: startedAt, data });
  return traceId;
}

export function currentComposerExpertTraceId(): string | undefined {
  return traceEnabled ? activeTraceId : undefined;
}

export function recordComposerExpertTrace(
  event: string,
  data: ComposerExpertTraceData = {},
  traceId = activeTraceId,
): void {
  if (!traceEnabled || !traceId) return;
  installWindowApi();
  const at = monotonicNow();
  const startedAt = traceStartedAt.get(traceId) ?? at;
  publishTraceEvent({
    traceId,
    event,
    elapsedMs: roundMilliseconds(at - startedAt),
    at,
    data,
  });
}

export function recordComposerExpertPaint(
  data: ComposerExpertTraceData,
  traceId = activeTraceId,
): void {
  if (!traceEnabled || !traceId || typeof window === "undefined") return;
  window.requestAnimationFrame(() => {
    recordComposerExpertTrace("app.raf-before-paint", data, traceId);
    window.setTimeout(() => recordComposerExpertTrace("app.after-paint", data, traceId), 0);
  });
}
