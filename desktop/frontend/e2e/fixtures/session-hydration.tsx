import { Profiler, useLayoutEffect } from "react";
import { createRoot } from "react-dom/client";
import gsap from "gsap";
import { ScrollToPlugin } from "gsap/ScrollToPlugin";
import { Transcript } from "../../src/components/Transcript";
import { LocaleProvider } from "../../src/lib/i18n";
import { useController } from "../../src/lib/useController";
import { createTranscriptTrace, installTranscriptTraceForTest } from "../../src/lib/transcriptPerformanceTrace";
import type { AppBindings } from "../../src/lib/bridge";
import type { ExpertTeamSession, TabMeta, TranscriptPage, WireEvent } from "../../src/lib/types";
import "../../src/styles.css";

gsap.registerPlugin(ScrollToPlugin);
// Isolate the existing DEV logger too: this fixture exports numeric summaries only.
installTranscriptTraceForTest(createTranscriptTrace({ enabled: false }));
const params = new URLSearchParams(location.search);
const scope = params.get("scope") === "global" ? "global" : "project";
const expert = params.has("expert");
const terminal = params.has("terminal");
const pendingPrompt = !terminal || params.has("prompt");
const sourceHandoff = params.has("source-handoff");
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}
const history = deferred<TranscriptPage>();
const jobs = deferred<[]>();
const team = deferred<ExpertTeamSession | null>();
const target = "fixture-target";
const sourceOwner = (id: string) => id.startsWith("source-");
const tab = (id: string): TabMeta => ({
  id, topicId: sourceOwner(id) ? target : id, sessionPath: `/fixture/${sourceOwner(id) ? target : id}.jsonl`, sessionTurns: 1,
  scope, workspaceRoot: scope === "project" ? "/fixture/workspace" : "",
  cwd: "/fixture", workspacePath: "/fixture", label: "Synthetic session",
  workspaceName: "Synthetic workspace", topicTitle: "Synthetic session",
  ready: true, running: false, active: true, mode: "normal",
  toolApprovalMode: "ask", tokenMode: "full", readOnly: false,
});
let active = tab(sourceHandoff ? "source-old" : "fixture-seed");
let sourcePrepared = false;
let sourceSerial = 0;
const activationGate = deferred<void>();
const eventHandlers = new Set<(event: WireEvent) => void>();
let controller!: ReturnType<typeof useController>;
let recording = false;
let epoch = 0;
let request = 0;
let frame = 0;
let criticalCount = 0;
let geometryCount = 0;
let disconnect = () => {};
const capacity = 256;
const now = () => Math.round((performance.now() - epoch) * 100) / 100;
const critical = createTranscriptTrace({ enabled: true, now, maxEntries: capacity });
const geometry = createTranscriptTrace({ enabled: true, now, maxEntries: capacity });
type Operation = "activate" | "history" | "jobs" | "checkpoints" | "context" | "balance" | "runtime" | "team";

function commit(source: "controller" | "transcript" | "capture") {
  if (!recording || !controller) return;
  const state = controller.state;
  criticalCount++;
  critical.record(source === "capture" ? "capture.start" : "commit", {
    source,
    target: controller.renderedViewTarget?.topicId === target,
    running: state.running, approval: Boolean(state.approval),
    loaded: state.hydrateHistoryLoaded === true, hydrating: state.hydrating,
    preview: Boolean(state.hydratePreview), transition: controller.surfaceTransitionActive,
    items: state.items.length,
    assistantChars: state.items.reduce((n, item) => n + (item.kind === "assistant" ? item.text.length : 0), 0),
  });
  queueGeometry();
}

function sampleGeometry() {
  frame = 0;
  if (!recording) return;
  // Fixed anchors, not a per-frame walk over the transcript. RAF is not paint evidence.
  const selectors = {
    viewport: ".transcript",
    question: ".msg--user",
    process: ".turn-collapse",
    answer: ".msg--assistant:not(.turn-collapse .msg--assistant)",
    team: '[data-testid="persisted-expert-presentation"]',
  } as const;
  for (const [anchor, selector] of Object.entries(selectors)) {
    const node = document.querySelector<HTMLElement>(selector);
    const rect = node?.getBoundingClientRect();
    const style = node && (anchor === "question" || anchor === "answer") ? getComputedStyle(node) : undefined;
    geometryCount++;
    geometry.record(anchor, {
      present: Boolean(node), x: rect?.x, y: rect?.y, width: rect?.width, height: rect?.height,
      scrollTop: anchor === "viewport" ? node?.scrollTop : undefined,
      scrollHeight: anchor === "viewport" ? node?.scrollHeight : undefined,
      clientHeight: anchor === "viewport" ? node?.clientHeight : undefined,
      layoutAuto: style ? style.contentVisibility === "auto" : undefined,
      intrinsicHeight60: style ? ["60px", "auto 60px"].includes(style.containIntrinsicHeight) : undefined,
      markdownParagraph: node && anchor === "answer" ? Boolean(node.querySelector(".md > p")) : undefined,
    });
  }
}
function queueGeometry() {
  if (recording && !frame) frame = requestAnimationFrame(sampleGeometry);
}
async function measured<T>(operation: Operation, run: () => Promise<T>): Promise<T> {
  const id = ++request;
  const captureEpoch = epoch;
  const captured = recording;
  if (captured) {
    criticalCount++;
    critical.record("request.start", { operation, request: id });
  }
  try {
    const result = await run();
    if (captured && recording && epoch === captureEpoch) {
      criticalCount++;
      critical.record("request.end", { operation, request: id });
      queueGeometry();
    }
    return result;
  } catch (error) {
    if (captured && recording && epoch === captureEpoch) {
      criticalCount++;
      critical.record("request.failure", { operation, request: id });
    }
    throw error;
  }
}
const emptyPage = (): TranscriptPage => ({ journalId: "fixture-seed-journal", through: 0, next: 0, hasMore: false, entries: [], baseline: [] });
const sourcePage = (complete: boolean): TranscriptPage => ({
  journalId: "source-shared-journal", through: 0, next: 0, hasMore: false, entries: [],
  baseline: Array.from({ length: complete ? 54 : 8 }, (_, index) => ({
    role: index === 0 ? "user" : "assistant",
    content: index === 53 ? "Complete accepted answer. " + "Verified report content. ".repeat(30) : `Synthetic source message ${index + 1}`,
  })),
});
const replay = (): TranscriptPage => {
  const entries: TranscriptPage["entries"] = [
    { sequence: 1, payload: { kind: "turn_started", turnId: "fixture-turn", activeTurnStartedAt: 1000 } },
    { sequence: 2, payload: { kind: "message", turnId: "fixture-turn", text: "Synthetic completed report.\n\n" + "A diagnostic paragraph with no private data. ".repeat(80) } },
  ];
  if (pendingPrompt) entries.push({ sequence: entries.length + 1, payload: { kind: "approval_request", approval: { id: "fixture-prompt", tool: "read_file", subject: "synthetic" } } });
  if (terminal) entries.push({ sequence: entries.length + 1, payload: { kind: "turn_done", turnId: "fixture-turn" } });
  return { journalId: "fixture-target-journal", through: entries.length, next: entries.length,
    hasMore: false, baseline: [{ role: "user", content: "Synthetic request" }], entries };
};
const api: Partial<AppBindings> = {
  ListTabs: () => measured("runtime", async () => [active]),
  MetaForTab: async (id) => ({ ...tab(id), eventChannel: "agent:event", autoApproveTools: false, bypass: false, collaborationMode: "normal" }),
  EffortForTab: async () => ({ supported: false, current: "auto", default: "auto", levels: [] }),
  ContextUsageForTab: () => measured("context", async () => ({ used: 0, window: 100, sessionTokens: 0 })),
  BalanceForTab: () => measured("balance", async () => ({ available: false, display: "" })),
  JobsForTab: (id) => measured("jobs", () => id === target ? jobs.promise : Promise.resolve([])),
  CheckpointsForTab: () => measured("checkpoints", async () => []),
  ReplayPendingPrompts: async () => {},
  ExpertTeamForTab: (id) => measured("team", () => id === target && expert ? team.promise : Promise.resolve(null)),
  TranscriptPageForTab: (id) => measured("history", () => sourceHandoff && sourceOwner(id)
    ? sourcePrepared ? history.promise : Promise.resolve(sourcePage(id !== "source-old"))
    : id === target ? history.promise : Promise.resolve(emptyPage())),
  ActivateTopic: (_scope, _root, id) => measured("activate", async () => {
    if (sourceHandoff && id === target) {
      if (sourcePrepared) await activationGate.promise;
      active = tab(sourcePrepared && params.has("reuse") ? "source-old" : `source-${++sourceSerial}`);
    } else active = tab(id);
    return active;
  }),
};
window.go = { main: { App: api as AppBindings } };
window.runtime = { EventsOn: (name, handler) => {
  if (name === "agent:event") eventHandlers.add(handler);
  return () => { eventHandlers.delete(handler); };
}, BrowserOpenURL() {} };

async function prepareSource() {
  const settle = async () => {
    for (let attempt = 0; attempt < 500; attempt++) {
      await new Promise((resolve) => setTimeout(resolve, 20));
      if (controller.state.hydrateHistoryLoaded && !controller.state.hydrating && !controller.surfaceTransitionActive) return;
    }
    throw new Error("Source fixture did not settle");
  };
  await settle();
  for (const handler of eventHandlers) handler({ kind: "turn_started", tabId: "source-old", turnId: "stale-retained-turn" });
  const open = (id: string) => controller.activateTopic(scope, scope === "project" ? "/fixture/workspace" : "", id, `/fixture/${id}.jsonl`);
  await open("fixture-away"); await settle();
  await open(target); await settle();
  await open("fixture-away"); await settle();
  sourcePrepared = true;
}

function stop() {
  recording = false;
  cancelAnimationFrame(frame);
  frame = 0;
  disconnect();
  disconnect = () => {};
}
const capture = {
  start() {
    stop();
    critical.clear(); geometry.clear();
    criticalCount = 0; geometryCount = 0; request = 0;
    epoch = performance.now(); recording = true;
    const surface = document.getElementById("surface")!;
    const resize = new ResizeObserver(queueGeometry);
    resize.observe(surface);
    const viewport = surface.querySelector(".transcript");
    if (viewport) resize.observe(viewport);
    const mutation = new MutationObserver(queueGeometry);
    mutation.observe(surface, { childList: true, subtree: true, attributes: true, attributeFilter: ["class", "style", "aria-hidden", "data-state"] });
    surface.addEventListener("scroll", queueGeometry, true);
    disconnect = () => { resize.disconnect(); mutation.disconnect(); surface.removeEventListener("scroll", queueGeometry, true); };
    commit("capture");
  },
  stop,
  export() {
    return { schema: 1, scope, expert, terminal, pendingPrompt, sourceHandoff, visibleControl: params.has("visible-control"), active: recording, capacity,
      droppedCritical: Math.max(0, criticalCount - capacity), droppedGeometry: Math.max(0, geometryCount - capacity),
      critical: critical.events(), geometry: geometry.events() };
  },
  get state() {
    return { ready: controller?.state.hydrateHistoryLoaded === true, target: controller?.renderedViewTarget?.topicId === target, sourcePrepared,
      running: controller?.state.running, approval: Boolean(controller?.state.approval),
      items: controller?.state.items.length };
  },
};
declare global { interface Window { __hydrationFixture: typeof capture } }
window.__hydrationFixture = capture;

function Fixture() {
  controller = useController();
  const state = controller.state;
  useLayoutEffect(() => commit("controller"));
  return <LocaleProvider>
    <div style={{ height: "100dvh", display: "flex", flexDirection: "column" }}>
      <div style={{ display: "flex", gap: 8, flexShrink: 0, flexWrap: "wrap" }}>
        <button onClick={() => capture.start()}>Start capture</button>
        {sourceHandoff && <button onClick={() => void prepareSource()}>Prepare accepted source</button>}
        <button onClick={() => void controller.activateTopic(scope, scope === "project" ? "/fixture/workspace" : "", target, `/fixture/${target}.jsonl`)}>Open session</button>
        {sourceHandoff && <button onClick={() => activationGate.resolve()}>Release activation</button>}
        <button onClick={() => history.resolve(sourceHandoff ? sourcePage(true) : replay())}>Release history</button>
        <button onClick={() => team.resolve({ state: "team_archived", id: "fixture-team", name: "Synthetic team", lead: { memberKey: "lead", name: "Lead", role: "coordination" }, members: [] })}>Release team</button>
        <button onClick={() => jobs.resolve([])}>Release jobs</button>
        <button onClick={stop}>Stop capture</button>
      </div>
      <div id="surface" style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
        <Profiler id="transcript" onRender={() => commit("transcript")}>
          <Transcript items={state.items} live={state.live} viewKey={controller.sessionViewKey}
            creationMode running={state.running} questionNavigator={false}
            activeExpert={expert && controller.activeTabId === target ? { id: "fixture-team", name: "Synthetic team", mode: "team" } : null}
            initialScrollAnchor={controller.sessionViewScroll} onScrollAnchorChange={controller.updateSessionViewScroll}
            tabId={controller.surfaceTransitionActive ? undefined : controller.activeTabId} onPrompt={() => {}}
            hydrating={state.hydrating && !state.hydrateHistoryLoaded}
            existingSessionLoading={state.surfaceIntent === "existing-session" && state.hydrating && state.hydrateHistoryLoaded !== true} />
        </Profiler>
      </div>
    </div>
  </LocaleProvider>;
}
createRoot(document.getElementById("root")!).render(<Fixture />);
window.addEventListener("pagehide", stop, { once: true });
