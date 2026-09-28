export type TranscriptTraceData = Record<string, string | number | boolean | null | undefined>;

export type TranscriptTraceEvent = {
  seq: number;
  at: number;
  event: string;
  data: Record<string, string | number | boolean | null>;
};

type StorageLike = Pick<Storage, "getItem" | "setItem" | "removeItem">;

type TranscriptTraceOptions = {
  enabled: boolean;
  storage?: StorageLike;
  log?: (line: string) => void;
  now?: () => number;
  maxEntries?: number;
  maxPersistedEntries?: number;
  maxPersistedBytes?: number;
};

type RecordOptions = { persist?: boolean };

export type TranscriptTrace = {
  record: (event: string, data?: TranscriptTraceData, options?: RecordOptions) => void;
  events: () => TranscriptTraceEvent[];
  export: () => string;
  clear: () => void;
};

declare global {
  interface Window {
    __transcriptPerformanceTrace?: TranscriptTrace;
  }
}

export const TRANSCRIPT_TRACE_STORAGE_KEY = "kwu:transcript-performance-trace:v1";
export const TRANSCRIPT_TRACE_MAX_ENTRIES = 240;
export const TRANSCRIPT_TRACE_MAX_PERSISTED_ENTRIES = 120;
export const TRANSCRIPT_TRACE_MAX_PERSISTED_BYTES = 64 * 1024;

const TRACE_PREFIX = "[transcript-performance-trace]";
const FORBIDDEN_DATA_KEY = /(content|text|prompt|reasoning|args|arguments|output|message)/i;

function opaqueTraceHash(value: string): string {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0).toString(16).padStart(8, "0");
}

function safeEvents(raw: string | null): TranscriptTraceEvent[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.flatMap((entry): TranscriptTraceEvent[] => {
      if (!entry || typeof entry !== "object") return [];
      const candidate = entry as Partial<TranscriptTraceEvent>;
      const valid = Number.isFinite(candidate.seq)
        && Number.isFinite(candidate.at)
        && typeof candidate.event === "string"
        && Boolean(candidate.data && typeof candidate.data === "object");
      if (!valid) return [];
      return [{
        seq: candidate.seq as number,
        at: candidate.at as number,
        event: (candidate.event as string).slice(0, 80),
        data: sanitizeData(candidate.data as Record<string, unknown>),
      }];
    });
  } catch {
    return [];
  }
}

function sanitizeData(data: Record<string, unknown>): TranscriptTraceEvent["data"] {
  const clean: TranscriptTraceEvent["data"] = {};
  for (const [key, value] of Object.entries(data)) {
    if (FORBIDDEN_DATA_KEY.test(key) || value === undefined) continue;
    if (typeof value === "string") clean[key] = value.slice(0, 80);
    else if (typeof value === "number") clean[key] = Number.isFinite(value) ? value : 0;
    else if (typeof value === "boolean" || value === null) clean[key] = value;
  }
  return clean;
}

function cloneEvents(events: TranscriptTraceEvent[]): TranscriptTraceEvent[] {
  return events.map((entry) => ({ ...entry, data: { ...entry.data } }));
}

function utf8Bytes(value: string): number {
  return typeof TextEncoder === "undefined" ? value.length * 2 : new TextEncoder().encode(value).byteLength;
}

export function createTranscriptTrace(options: TranscriptTraceOptions): TranscriptTrace {
  const maxEntries = Math.max(1, options.maxEntries ?? TRANSCRIPT_TRACE_MAX_ENTRIES);
  const maxPersistedEntries = Math.max(1, options.maxPersistedEntries ?? TRANSCRIPT_TRACE_MAX_PERSISTED_ENTRIES);
  const maxPersistedBytes = Math.max(512, options.maxPersistedBytes ?? TRANSCRIPT_TRACE_MAX_PERSISTED_BYTES);
  const storage = options.storage;
  const persistedEvents = options.enabled ? safeEvents(storage?.getItem(TRANSCRIPT_TRACE_STORAGE_KEY) ?? null) : [];
  const events = [...persistedEvents];
  if (events.length > maxEntries) events.splice(0, events.length - maxEntries);
  let sequence = events.reduce((max, entry) => Math.max(max, entry.seq), 0);

  const persist = () => {
    if (!storage) return;
    let persisted = persistedEvents.slice(-maxPersistedEntries);
    let serialized = JSON.stringify(persisted);
    while (persisted.length > 1 && utf8Bytes(serialized) > maxPersistedBytes) {
      persisted = persisted.slice(Math.max(1, Math.ceil(persisted.length / 8)));
      serialized = JSON.stringify(persisted);
    }
    try {
      storage.setItem(TRANSCRIPT_TRACE_STORAGE_KEY, serialized);
    } catch {
      // Diagnostics must never affect the transcript path.
    }
  };

  return {
    record(event, data = {}, recordOptions = {}) {
      if (!options.enabled) return;
      const entry: TranscriptTraceEvent = {
        seq: ++sequence,
        at: (options.now ?? Date.now)(),
        event: event.slice(0, 80),
        data: sanitizeData(data),
      };
      events.push(entry);
      if (events.length > maxEntries) events.splice(0, events.length - maxEntries);
      const line = `${TRACE_PREFIX} ${JSON.stringify(entry)}`;
      options.log?.(line);
      if (recordOptions.persist) {
        persistedEvents.push(entry);
        if (persistedEvents.length > maxPersistedEntries) {
          persistedEvents.splice(0, persistedEvents.length - maxPersistedEntries);
        }
        persist();
      }
    },
    events: () => cloneEvents(events),
    export: () => JSON.stringify(events, null, 2),
    clear() {
      events.length = 0;
      persistedEvents.length = 0;
      sequence = 0;
      try {
        storage?.removeItem(TRANSCRIPT_TRACE_STORAGE_KEY);
      } catch {
        // Diagnostics must never affect the transcript path.
      }
    },
  };
}

function browserStorage(): StorageLike | undefined {
  try {
    return typeof localStorage === "undefined" ? undefined : localStorage;
  } catch {
    return undefined;
  }
}

export const TRANSCRIPT_TRACE_ENABLED = import.meta.env?.DEV === true;
const transcriptTrace = createTranscriptTrace({
  enabled: TRANSCRIPT_TRACE_ENABLED,
  storage: browserStorage(),
  log: (line) => {
    console.debug(line);
    const runtime = typeof window === "undefined"
      ? undefined
      : window.runtime as (typeof window.runtime & { LogInfo?: (message: string) => void }) | undefined;
    runtime?.LogInfo?.(line);
  },
});
let transcriptTraceTestOverride: TranscriptTrace | undefined;

if (TRANSCRIPT_TRACE_ENABLED && typeof window !== "undefined") {
  window.__transcriptPerformanceTrace = transcriptTrace;
}

export function recordTranscriptTrace(event: string, data: TranscriptTraceData = {}, options: RecordOptions = {}): void {
  if (TRANSCRIPT_TRACE_ENABLED && transcriptTraceTestOverride) {
    transcriptTraceTestOverride.record(event, data, options);
    return;
  }
  transcriptTrace.record(event, data, options);
}

export function transcriptTraceViewToken(viewKey: string): string {
  return `v${opaqueTraceHash(viewKey)}`;
}

export function transcriptTraceOpaqueToken(value: string, namespace = "o"): string {
  const prefix = /^[a-z]$/i.test(namespace) ? namespace.toLocaleLowerCase() : "o";
  return `${prefix}${opaqueTraceHash(`${prefix}\0${value}`)}`;
}

/** Installs an isolated recorder for component tests without enabling production diagnostics. */
export function installTranscriptTraceForTest(trace: TranscriptTrace): () => void {
  const previous = transcriptTraceTestOverride;
  transcriptTraceTestOverride = trace;
  return () => {
    if (transcriptTraceTestOverride === trace) transcriptTraceTestOverride = previous;
  };
}

export function transcriptTraceMemorySnapshot(): TranscriptTraceData {
  if (typeof performance === "undefined") return {};
  const memory = (performance as Performance & {
    memory?: { usedJSHeapSize?: number; totalJSHeapSize?: number; jsHeapSizeLimit?: number };
  }).memory;
  if (!memory) return {};
  const megabytes = (bytes?: number) => bytes && Number.isFinite(bytes)
    ? Math.round((bytes / 1024 / 1024) * 100) / 100
    : undefined;
  return {
    usedHeapMb: megabytes(memory.usedJSHeapSize),
    totalHeapMb: megabytes(memory.totalJSHeapSize),
    heapLimitMb: megabytes(memory.jsHeapSizeLimit),
  };
}
