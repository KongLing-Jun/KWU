// Run: tsx src/__tests__/transcript-performance-trace.test.ts

import {
  createTranscriptTrace,
  TRANSCRIPT_TRACE_STORAGE_KEY,
  transcriptTraceOpaqueToken,
  transcriptTraceMemorySnapshot,
  transcriptTraceViewToken,
} from "../lib/transcriptPerformanceTrace";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

class MemoryStorage {
  readonly values = new Map<string, string>();
  reads = 0;
  writes = 0;

  getItem(key: string) {
    this.reads += 1;
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string) {
    this.writes += 1;
    this.values.set(key, value);
  }

  removeItem(key: string) {
    this.values.delete(key);
  }
}

console.log("\ntranscript performance trace");

{
  const storage = new MemoryStorage();
  const lines: string[] = [];
  let now = 100;
  const trace = createTranscriptTrace({
    enabled: true,
    storage,
    log: (line) => lines.push(line),
    now: () => ++now,
    maxEntries: 3,
    maxPersistedEntries: 3,
  });

  trace.record("view.commit", {
    path: "team",
    itemCount: 51,
    content: "private body",
    toolArgs: "private args",
    toolOutput: "private output",
    promptValue: "private prompt",
    reasoning: "private reasoning",
  }, { persist: true });
  trace.record("virtual.range", { first: 0, last: 3 });
  trace.record("raf.fire", { active: 1 });
  trace.record("view.commit", { path: "normal" }, { persist: true });

  const events = trace.events();
  const exported = trace.export();
  ok(events.length === 3 && events[0].event === "virtual.range", "in-memory trace obeys the entry ring limit");
  ok(storage.writes === 2, "only explicitly marked structural events synchronously persist");
  ok(lines.length === 4 && lines.every((line) => line.startsWith("[transcript-performance-trace]")), "each event mirrors to the development log sink");
  ok(!/private body|private args|private output|private prompt|private reasoning/.test(exported), "trace strips transcript text, prompts, reasoning and tool payload fields");
  ok(events.every((entry) => !Object.keys(entry.data).some((key) => /(content|text|prompt|reasoning|args|output|message)/i.test(key))), "forbidden payload keys never enter the event ring");

  const reloaded = createTranscriptTrace({ enabled: true, storage, maxEntries: 3, maxPersistedEntries: 3 });
  ok(reloaded.events().length === 2 && reloaded.events().every((entry) => entry.event === "view.commit"), "only structural events remain readable after a module-style reload");
  reloaded.clear();
  ok(storage.values.has(TRANSCRIPT_TRACE_STORAGE_KEY) === false, "clear removes the persisted diagnostic ring");
}

{
  const storage = new MemoryStorage();
  storage.values.set(TRANSCRIPT_TRACE_STORAGE_KEY, JSON.stringify([{
    seq: 1,
    at: 1,
    event: "view.commit",
    data: { path: "team", content: "poisoned persisted body", nested: { output: "poisoned" } },
  }]));
  const trace = createTranscriptTrace({ enabled: true, storage });
  ok(trace.events()[0]?.data.path === "team" && !trace.export().includes("poisoned"), "reloaded diagnostics re-sanitize persisted data before exposing it");
}

{
  const storage = new MemoryStorage();
  const trace = createTranscriptTrace({ enabled: true, storage, maxEntries: 50, maxPersistedEntries: 50, maxPersistedBytes: 512 });
  for (let index = 0; index < 20; index += 1) {
    trace.record("view.commit", { path: `team-${index}-${"x".repeat(80)}` }, { persist: true });
  }
  const raw = storage.values.get(TRANSCRIPT_TRACE_STORAGE_KEY) ?? "";
  ok(new TextEncoder().encode(raw).byteLength <= 512 && JSON.parse(raw).length < 20, "persisted structural trace obeys its UTF-8 byte ceiling");
}

{
  const storage = new MemoryStorage();
  const lines: string[] = [];
  const trace = createTranscriptTrace({ enabled: false, storage, log: (line) => lines.push(line) });
  trace.record("view.commit", { itemCount: 51 }, { persist: true });
  ok(trace.events().length === 0 && storage.reads === 0 && storage.writes === 0 && lines.length === 0, "production-disabled trace performs no reads, writes or logging");
}

{
  const first = transcriptTraceViewToken("global|private/session/path");
  const second = transcriptTraceViewToken("global|private/session/path");
  ok(first === second && /^v[0-9a-f]{8}$/.test(first) && !first.includes("private"), "view identity is logged only as a stable opaque token");

  const anchor = transcriptTraceOpaqueToken("private-message-id", "a");
  ok(/^a[0-9a-f]{8}$/.test(anchor) && !anchor.includes("private-message-id") && anchor !== first, "anchor identity uses a namespaced opaque token rather than a raw message ID");
}

{
  const previous = Object.getOwnPropertyDescriptor(performance, "memory");
  Object.defineProperty(performance, "memory", {
    configurable: true,
    value: {
      usedJSHeapSize: 10 * 1024 * 1024,
      totalJSHeapSize: 20 * 1024 * 1024,
      jsHeapSizeLimit: 100 * 1024 * 1024,
    },
  });
  const memory = transcriptTraceMemorySnapshot();
  ok(memory.usedHeapMb === 10 && memory.totalHeapMb === 20 && memory.heapLimitMb === 100, "low-frequency snapshots include numeric JS heap usage when Chromium exposes it");
  if (previous) Object.defineProperty(performance, "memory", previous);
  else delete (performance as Performance & { memory?: unknown }).memory;
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
