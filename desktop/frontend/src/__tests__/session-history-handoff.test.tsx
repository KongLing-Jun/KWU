// Run: tsx src/__tests__/session-history-handoff.test.tsx
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import React, { act, useLayoutEffect } from "react";
import { createRoot } from "react-dom/client";
import { useController, initialState } from "../lib/useController";
import { Transcript } from "../components/Transcript";
import { HistoryLoadErrorBanner } from "../components/HistoryLoadErrorBanner";
import { LocaleProvider } from "../lib/i18n";
import { snapshotSessionView } from "../features/session/sessionViewSnapshot";
import type { AppBindings } from "../lib/bridge";
import type { ExpertTeamSession, HistoryMessage, TabMeta, TranscriptPage, WireEvent } from "../lib/types";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

const tick = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const messages = (topic: string): HistoryMessage[] => [
  { role: "user", content: `history ${topic}` },
  { role: "assistant", content: `answer ${topic}` },
];
const page = (topic: string, baseline = messages(topic)): TranscriptPage => ({
  journalId: `${topic}-journal`, through: 0, next: 0, hasMore: false, entries: [], baseline,
});

async function harness(paginated = false, creationMode = false) {
  const dom = new JSDOM("<!doctype html><div id='root'></div>", { url: "http://localhost/", pretendToBeVisual: true });
  Object.assign(globalThis, {
    IS_REACT_ACT_ENVIRONMENT: true,
    window: dom.window, document: dom.window.document, localStorage: dom.window.localStorage,
    Node: dom.window.Node, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event,
    CustomEvent: dom.window.CustomEvent, MouseEvent: dom.window.MouseEvent,
    requestAnimationFrame: () => 1, cancelAnimationFrame: () => {},
    ResizeObserver: class { observe() {} unobserve() {} disconnect() {} },
  });
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  dom.window.requestAnimationFrame = () => 1;
  dom.window.cancelAnimationFrame = () => {};
  dom.window.matchMedia = (() => ({ matches: false, addEventListener() {}, removeEventListener() {} })) as typeof window.matchMedia;
  dom.window.HTMLElement.prototype.scrollTo = () => {};
  dom.window.HTMLElement.prototype.scrollIntoView = () => {};
  let now = 0;
  let timerId = 0;
  const timers = new Map<number, { at: number; run: () => void }>();
  window.setTimeout = ((run: () => void, delay = 0) => {
    const id = ++timerId;
    timers.set(id, { at: now + delay, run });
    return id;
  }) as typeof window.setTimeout;
  window.clearTimeout = (id) => { timers.delete(id); };
  const eventHandlers = new Set<(event: WireEvent) => void>();
  window.runtime = {
    EventsOn: (name, handler) => {
      if (name === "agent:event") eventHandlers.add(handler);
      return () => { eventHandlers.delete(handler); };
    },
    BrowserOpenURL() {},
  };
  const tab = (id: string, topic: string): TabMeta => ({
    id, topicId: topic, sessionPath: `/diagnostic/${topic}.jsonl`, sessionTurns: 1,
    scope: "global", workspaceRoot: "", cwd: "/diagnostic", workspacePath: "/diagnostic",
    label: "diagnostic", ready: true, running: false, active: true, mode: "normal",
    toolApprovalMode: "ask", tokenMode: "full", readOnly: false,
  });
  let active = tab("original-a", "a");
  const tabs = new Map([[active.id, active]]);
  let serial = 0;
  let activation: ReturnType<typeof deferred<void>> | undefined;
  let read: ReturnType<typeof deferred<TranscriptPage | null>> | undefined;
  let reuseOriginal = false;
  let overridePath: string | undefined;
  let teamRead: ReturnType<typeof deferred<ExpertTeamSession | null>> | undefined;
  const reads: Array<{ id: string; journal: string; after: number }> = [];
  const olderReads: Array<{ id: string; before: number; generation: string }> = [];
  const api: Partial<AppBindings> = {
    SubmitToTab: async () => {}, CloseTab: async () => {}, ClearSession: async () => {},
    ActivateSessionForTab: async (id, path) => { tabs.get(id)!.sessionPath = path; },
    ListTabs: async () => [active],
    MetaForTab: async (id) => ({ ...tabs.get(id)!, eventChannel: "agent:event", autoApproveTools: false, bypass: false, collaborationMode: "normal" }),
    EffortForTab: async () => ({ supported: false, current: "auto", default: "auto", levels: [] }),
    ContextUsageForTab: async () => ({ used: 0, window: 100, sessionTokens: 0 }),
    BalanceForTab: async () => ({ available: false, display: "" }),
    JobsForTab: async () => [], CheckpointsForTab: async () => [], ReplayPendingPrompts: async () => {},
    ExpertTeamForTab: async () => teamRead ? teamRead.promise : null,
    TranscriptPageForTab: async (id, journal, after) => {
      reads.push({ id, journal, after });
      if (read && tabs.get(id)?.topicId === "a") return read.promise;
      if (paginated) return null;
      return page(tabs.get(id)!.topicId!);
    },
    HistoryPageForTab: async (id, before, _limit, generation) => {
      olderReads.push({ id, before, generation });
      const start = before > 0 ? Math.max(0, before - 2) : 4;
      const topic = tabs.get(id)!.topicId!;
      return {
        messages: [start, start + 1].map((index) => ({ role: "user", content: `history ${topic} ${index}` })),
        before: start, hasMore: start > 0, generation: `${id}-generation`,
      };
    },
    ActivateTopic: async (_scope, _root, topic) => {
      const gate = activation;
      if (gate) await gate.promise;
      active = tab(reuseOriginal && topic === "a" ? "original-a" : `new-${++serial}`, topic);
      active.scope = _scope;
      active.workspaceRoot = _root;
      if (overridePath) active.sessionPath = overridePath;
      tabs.set(active.id, active);
      return active;
    },
  };
  window.go = { main: { App: api as AppBindings } };
  let controller!: ReturnType<typeof useController>;
  const commits: Array<{ running: boolean; approval: boolean; loaded: boolean; preview: boolean; items: number }> = [];
  function Probe() {
    controller = useController();
    const state = controller.state;
    useLayoutEffect(() => {
      commits.push({ running: state.running, approval: Boolean(state.approval), loaded: state.hydrateHistoryLoaded === true, preview: Boolean(state.hydratePreview), items: state.items.length });
    });
    return <LocaleProvider>
      <HistoryLoadErrorBanner error={state.hydrateError} cached={Boolean(state.hydratePreview)} onRetry={() => void controller.retryHistoryLoad()} />
      <Transcript items={state.items} live={state.live} viewKey={controller.sessionViewKey}
        creationMode={creationMode} running={state.running}
        activeExpert={teamRead ? { id: "team", name: "Review Team", mode: "team" } : undefined}
        expertSelectionRevision={teamRead ? 1 : 0}
        initialScrollAnchor={controller.sessionViewScroll} onScrollAnchorChange={controller.updateSessionViewScroll}
        tabId={controller.surfaceTransitionActive ? undefined : controller.activeTabId} onPrompt={() => {}}
        hydrating={state.hydrating && !state.hydrateHistoryLoaded}
        existingSessionLoading={state.surfaceIntent === "existing-session" && state.hydrating && state.hydrateHistoryLoaded !== true} />
    </LocaleProvider>;
  }
  const root = createRoot(document.getElementById("root")!);
  await act(async () => { root.render(<Probe />); await tick(); });
  assert.equal(controller.state.hydrateHistoryLoaded, true);
  const open = (topic: string) => controller.activateTopic("global", "", topic, `/diagnostic/${topic}.jsonl`);
  return {
    get controller() { return controller; }, api, reads, olderReads, commits,
    get active() { return active; },
    open,
    holdActivation() { activation = deferred<void>(); return activation; },
    releaseActivation() { activation = undefined; },
    holdRead() { read = deferred<TranscriptPage | null>(); return read; },
    holdTeam() { teamRead = deferred<ExpertTeamSession | null>(); return teamRead; },
    reuse() { reuseOriginal = true; },
    mismatch() { overridePath = "/diagnostic/different.jsonl"; },
    async advance(ms: number) {
      await act(async () => {
        now += ms;
        for (const [id, timer] of [...timers]) if (timer.at <= now) { timers.delete(id); timer.run(); }
        await tick();
      });
    },
    async emit(event: WireEvent) { await act(async () => { for (const handler of eventHandlers) handler(event); await tick(); }); },
    async close() { await act(async () => root.unmount()); dom.window.close(); },
  };
}

function visibleHistory(label: string) {
  assert.match(document.body.textContent ?? "", /history a/, label);
  assert.equal(document.querySelector(".transcript__session-loading"), null, label);
}

const projected = snapshotSessionView({
  ...initialState, running: true, turnActive: true,
  items: [{ kind: "assistant", id: "live", text: "old", reasoning: "", streaming: true }],
  live: { id: "live", text: "latest output", reasoning: "latest reasoning", reasoningComplete: true },
  transcriptJournalId: "old-owner-journal", transcriptThrough: 99,
}, { scope: "global", sessionPath: "/diagnostic/a.jsonl" }, { atBottom: true });
assert.equal(projected.items[0].kind === "assistant" && projected.items[0].text, "latest output");
assert.equal(projected.items[0].kind === "assistant" && projected.items[0].streaming, false);
assert.equal("live" in projected || "running" in projected || "transcriptJournalId" in projected, false);

for (const reuse of [false, true]) {
  const h = await harness();
  await h.emit({ kind: "turn_started", tabId: "original-a", turnId: "retained-old-turn" });
  assert.equal(h.controller.state.hydrateHistoryLoaded, true);
  await act(async () => { await h.open("b"); await tick(); });
  const complete = messages("a").concat({ role: "assistant", content: "Complete accepted answer" });
  const firstRead = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  await act(async () => { firstRead.resolve(page("a", complete)); await tick(); });
  assert.match(document.body.textContent ?? "", /Complete accepted answer/);
  await act(async () => { await h.open("b"); await tick(); });
  if (reuse) h.reuse();
  const activation = h.holdActivation();
  const read = h.holdRead();
  const start = h.commits.length;
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.open("a"); await tick(); });
  assert.match(document.body.textContent ?? "", /Complete accepted answer/, "first selection uses accepted history, not first retained owner");
  assert.equal(h.controller.state.running, false);
  await act(async () => { activation.resolve(); await opening; await tick(); });
  assert.match(document.body.textContent ?? "", /Complete accepted answer/, "canonical retained state cannot displace the selected complete preview");
  await act(async () => { read.resolve(page("a", complete)); await tick(); });
  assert.ok(h.commits.slice(start).every((commit) => commit.items === 3 && !commit.running), "every committed phase keeps the complete idle projection");
  assert.ok(h.reads.filter((request) => request.id === h.active.id).length > 0, "history is still revalidated");
  await h.close();
  console.log(`PASS source selection: stale loaded/running owner, reuse=${reuse}`);
}

{
  const h = await harness();
  await act(async () => { await h.open("b"); await tick(); });
  const olderRead = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  const supersededId = h.active.id;
  await act(async () => { await h.open("b"); await tick(); });
  const latestRead = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  const complete = messages("a").concat({ role: "assistant", content: "Latest accepted source" });
  await act(async () => { latestRead.resolve(page("a", complete)); await tick(); });
  await h.emit({ kind: "turn_started", tabId: supersededId, turnId: "late-old-event" });
  await act(async () => { olderRead.resolve(page("a")); await tick(); });
  await act(async () => { await h.open("b"); await tick(); });
  const activation = h.holdActivation();
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.open("a"); await tick(); });
  assert.match(document.body.textContent ?? "", /Latest accepted source/, "late old events and loads cannot replace accepted source");
  assert.equal(h.controller.state.running, false);
  await act(async () => { activation.resolve(); await opening; await tick(); });
  await h.close();
  console.log("PASS source selection: borrowed preview and superseded background results");
}

for (const intent of ["user", "approval", "live"] as const) {
  const h = await harness();
  await act(async () => { await h.open("b"); await tick(); });
  const read = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  const owner = h.active.id;
  assert.ok(h.controller.state.hydratePreview);
  if (intent === "user") {
    await act(async () => { await h.controller.sendToTab(owner, "Optimistic current user"); await tick(); });
    assert.equal(h.controller.state.turnActive, false, "protection begins before turn_started");
    assert.match(document.body.textContent ?? "", /Optimistic current user/);
  } else if (intent === "approval") {
    await h.emit({ kind: "approval_request", tabId: owner, approval: { id: "current-prompt", tool: "read_file", subject: "synthetic" } });
    assert.equal(h.controller.state.approval?.id, "current-prompt");
  } else {
    await h.emit({ kind: "turn_started", tabId: owner, turnId: "current-turn" });
    await h.emit({ kind: "text", tabId: owner, text: "Current live text" });
    assert.equal(h.controller.state.running, true);
  }
  assert.equal(h.controller.state.hydratePreview, undefined, "current qualified intent exits borrowed preview");
  await act(async () => { read.resolve(page("a")); await tick(); });
  if (intent === "user") assert.match(document.body.textContent ?? "", /Optimistic current user/, "replay preserves optimistic user");
  await act(async () => { await h.open("b"); await tick(); });
  const activation = h.holdActivation();
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.open("a"); await tick(); });
  if (intent === "user") assert.match(document.body.textContent ?? "", /Optimistic current user/, "new intent remains the accepted source on return");
  assert.equal(h.controller.state.approval, undefined, "transition never exposes another owner's actionable prompt");
  await act(async () => { activation.resolve(); await opening; await tick(); });
  await h.close();
  console.log(`PASS source selection: qualified ${intent}`);
}

{
  const h = await harness();
  h.api.SubmitToTab = async () => { throw new Error("synthetic submit failure"); };
  await act(async () => { await h.controller.send("Failed current user").catch(() => {}); await tick(); });
  await act(async () => { await h.open("b"); await tick(); });
  const activation = h.holdActivation();
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.open("a"); await tick(); });
  assert.match(document.body.textContent ?? "", /Failed current user/, "a failed optimistic send remains the current source");
  assert.ok(h.controller.state.items.some((item) => item.kind === "user" && item.failed));
  await act(async () => { activation.resolve(); await opening; await tick(); });
  await h.close();
  console.log("PASS source selection: failed optimistic send remains visible on return");
}

{
  const h = await harness();
  await act(async () => { await h.controller.resumeSession("/diagnostic/rebound.jsonl"); await tick(); });
  assert.equal(h.controller.state.hydrateHistoryLoaded, true, "path binding does not invalidate the new history request");
  assert.equal(h.controller.state.meta?.sessionPath, "/diagnostic/rebound.jsonl");
  await act(async () => { await h.controller.clearSession(); await tick(); });
  assert.equal(h.controller.state.items.length, 0);
  await act(async () => { await h.open("b"); await tick(); });
  const activation = h.holdActivation();
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.controller.activateTopic("global", "", "a", "/diagnostic/rebound.jsonl"); await tick(); });
  assert.equal(h.controller.state.items.length, 0, "reset cannot resurrect pre-clear content");
  await act(async () => { activation.resolve(); await opening; await tick(); });
  await h.close();
  console.log("PASS source selection: path binding and reset lifecycle");
}

for (const outcome of ["success", "empty", "failure", "reuse", "mismatch", "scope-change"] as const) {
  const h = await harness();
  await act(async () => { await h.open("b"); await tick(); });
  if (outcome === "reuse") h.reuse();
  if (outcome === "mismatch") h.mismatch();
  const gate = h.holdActivation();
  const read = h.holdRead();
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.open("a"); await tick(); });
  visibleHistory(`${outcome}: before activation`);
  if (outcome === "scope-change") {
    await act(async () => h.controller.setSessionViewCacheScope({ actorId: "another-account" }));
  }
  await act(async () => { gate.resolve(); await opening; await tick(); });
  h.releaseActivation();
  if (outcome === "scope-change") {
    assert.notEqual(h.controller.activeTabId, h.active.id, "invalidated activation cannot bind to the new account");
    assert.doesNotMatch(document.body.textContent ?? "", /history a/);
  } else if (outcome === "mismatch") {
    assert.equal(h.controller.state.items.length, 0, "changed identity drops the incompatible preview");
  } else {
    visibleHistory(`${outcome}: after activation`);
    await h.advance(250);
    visibleHistory(`${outcome}: delayed history`);
    if (outcome !== "reuse") {
      assert.equal(h.controller.state.hydrateHistoryLoaded, false, "display does not authorize runtime actions");
      assert.equal(h.controller.state.transcriptJournalId, undefined, "new runtime does not inherit replay cursor");
      assert.equal(h.reads.at(-1)?.journal, "");
      assert.equal(h.controller.state.live, undefined);
      assert.equal(h.controller.state.approval, undefined);
    }
  }
  await act(async () => {
    if (outcome === "failure") read.reject(new Error("synthetic read failure"));
    else read.resolve(page("a", outcome === "empty" ? [] : messages("a")));
    await tick();
  });
  if (outcome === "failure") {
    visibleHistory("read failure preserves current preview");
    assert.equal(h.controller.state.hydrating, false);
    assert.ok(document.querySelector("[data-history-load-error]"));
    const retry = h.holdRead();
    await act(async () => { document.querySelector<HTMLButtonElement>("[data-history-load-error] button")!.click(); await tick(); });
    visibleHistory("retry preserves current preview");
    await act(async () => { retry.resolve(page("a")); await tick(); });
    assert.equal(h.controller.state.hydratePreview, undefined);
    assert.equal(h.controller.state.hydrateError, undefined);
  } else if (outcome === "empty") {
    assert.equal(h.controller.state.items.length, 0, "authoritative empty history replaces old preview");
    assert.equal(h.controller.state.hydratePreview, undefined);
  } else if (outcome !== "scope-change" && outcome !== "mismatch") {
    visibleHistory("successful replay publishes history");
    assert.equal(h.controller.state.hydratePreview, undefined);
    assert.equal(h.controller.state.hydrateHistoryLoaded, true);
  }
  await h.close();
  console.log(`PASS history handoff: ${outcome}`);
}

{
  const h = await harness();
  await act(async () => { await h.open("b"); await tick(); });
  const read = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  const oldTabId = h.controller.activeTabId;
  await act(async () => { await h.open("c"); await tick(); });
  await act(async () => { read.resolve(page("a")); await tick(); });
  assert.notEqual(h.controller.activeTabId, oldTabId);
  assert.match(document.body.textContent ?? "", /history c/);
  assert.doesNotMatch(document.body.textContent ?? "", /history a/);
  await h.close();
  console.log("PASS history handoff: late replay stays with its runtime");
}

{
  const h = await harness();
  await act(async () => { await h.open("b"); await tick(); });
  const read = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  const tabId = h.controller.activeTabId!;
  const event: WireEvent = {
    kind: "notice", tabId, text: "new owner event", level: "info",
    transcriptJournalId: "a-journal", transcriptSequence: 1,
  };
  await h.emit(event);
  visibleHistory("live events do not expose an incomplete new runtime");
  await act(async () => { read.resolve(page("a")); await tick(); });
  assert.equal(h.controller.state.items.filter((item) => item.kind === "notice" && item.text === "new owner event").length, 1);
  await h.close();
  console.log("PASS history handoff: current-owner events merge once");
}

{
  const h = await harness(true);
  await act(async () => { await h.controller.loadOlderHistory(); await tick(); });
  assert.equal(h.controller.state.historyBefore, 2);
  await act(async () => { await h.open("b"); await tick(); });
  const read = h.holdRead();
  await act(async () => { await h.open("a"); await tick(); });
  assert.match(document.body.textContent ?? "", /history a 2/);
  await act(async () => { read.resolve(null); await tick(); });
  const id = h.controller.activeTabId!;
  assert.equal(h.controller.state.items.length, 4, "handoff preserves previously expanded history range");
  assert.equal(h.controller.state.historyBefore, 2);
  assert.deepEqual(h.olderReads.filter((request) => request.id === id), [
    { id, before: 0, generation: "" },
    { id, before: 4, generation: `${id}-generation` },
  ], "range restoration starts with a fresh cursor from the new runtime");
  await act(async () => { await h.controller.loadOlderHistory(); await tick(); });
  assert.equal(h.controller.state.items.length, 6);
  assert.equal(h.controller.state.historyHasMore, false);
  await h.close();
  console.log("PASS history handoff: expanded history uses new-owner pagination");
}

for (const outcome of ["team", "missing", "failure"] as const) {
  const h = await harness();
  await act(async () => { await h.open("b"); await tick(); });
  const team = h.holdTeam();
  const gate = h.holdActivation();
  let opening!: Promise<TabMeta>;
  await act(async () => { opening = h.open("a"); await tick(); });
  visibleHistory("team preview before runtime binding");
  await act(async () => { gate.resolve(); await opening; await tick(); });
  await h.advance(250);
  visibleHistory("team sidecar must not replace readable history with loading");
  await act(async () => {
    if (outcome === "failure") team.reject(new Error("synthetic sidecar failure"));
    else team.resolve(outcome === "missing" ? null : {
      state: "team_archived", id: "team", name: "Review Team",
      lead: { memberKey: "lead", name: "Lead", role: "coordination" }, members: [],
    });
    await tick();
  });
  visibleHistory("settled team sidecar keeps the parent transcript readable");
  if (outcome === "team") assert.ok(document.querySelector("[data-testid='persisted-expert-presentation']"));
  await h.close();
  console.log(`PASS history handoff: delayed expert sidecar ${outcome}`);
}

// Characterization, not acceptance of stale runtime UI. These controlled reads
// identify which production await gates delay reconciliation without changing it.
for (const scope of ["global", "project"] as const) {
  for (const operation of ["JobsForTab", "CheckpointsForTab", "ContextUsageForTab", "BalanceForTab"] as const) {
    const h = await harness(false, true);
    const gate = deferred<never>();
    let listCalls = 0;
    h.api.ListTabs = async () => { listCalls += 1; return [h.active]; };
    h.api[operation] = () => gate.promise;
    h.api.TranscriptPageForTab = async () => ({
      ...page("b", [{ role: "user", content: "Synthetic request" }]), through: 3, next: 3,
      entries: [
        { sequence: 1, payload: { kind: "turn_started", turnId: "old-turn", activeTurnStartedAt: 1000 } },
        { sequence: 2, payload: { kind: "message", turnId: "old-turn", text: "Synthetic completed body" } },
        { sequence: 3, payload: { kind: "approval_request", approval: { id: "old-prompt", tool: "read_file", subject: "synthetic" } } },
      ],
    });
    await act(async () => {
      await h.controller.activateTopic(scope, scope === "project" ? "/diagnostic/workspace" : "", "b", "/diagnostic/b.jsonl");
      await tick();
    });
    assert.equal(h.controller.state.running, true, "unterminated replay currently reconstructs activity despite initial idle runtime metadata");
    assert.equal(h.controller.state.hydrating, false);
    assert.ok(h.controller.state.approval);
    assert.ok(h.commits.some((commit) => commit.loaded && commit.running && commit.approval), "the reconstructed state reached a real React commit");
    const heldCalls = listCalls;
    await act(async () => { await tick(); await tick(); });
    assert.equal(listCalls, heldCalls, "runtime refresh does not start while the ancillary promise stays pending");
    await act(async () => { gate.reject(new Error("synthetic ancillary rejection")); await tick(); await tick(); });
    assert.ok(listCalls > heldCalls);
    assert.equal(h.controller.state.running, false);
    assert.equal(h.controller.state.approval, undefined);
    console.log(`OBSERVED ${scope}/${operation}: committed replay activity; ancillary rejection released idle reconciliation`);
    await h.close();
  }
}

{
  const h = await harness();
  const gate = deferred<never>();
  h.api.JobsForTab = () => gate.promise;
  h.api.TranscriptPageForTab = async () => ({
    ...page("b"), through: 2, next: 2,
    entries: [
      { sequence: 1, payload: { kind: "turn_started", turnId: "finished-turn" } },
      { sequence: 2, payload: { kind: "turn_done", turnId: "finished-turn" } },
    ],
  });
  await act(async () => { await h.open("b"); await tick(); });
  assert.equal(h.controller.state.running, false, "a terminal replay stays idle even while ancillary data is pending");
  await act(async () => { gate.reject(new Error("synthetic release")); await tick(); });
  await h.close();
  console.log("PASS hydration control: terminal journal does not reconstruct activity");
}

{
  const h = await harness();
  const ancillary = deferred<never>();
  h.api.JobsForTab = () => ancillary.promise;
  await act(async () => { await h.open("b"); await tick(); });
  const idleSnapshot = { ...h.active };
  const status = deferred<TabMeta[]>();
  let statusStarted = false;
  h.api.ListTabs = () => { statusStarted = true; return status.promise; };
  await act(async () => { ancillary.reject(new Error("synthetic release")); await tick(); });
  assert.equal(statusStarted, true);
  await h.emit({ kind: "turn_started", tabId: h.active.id, turnId: "new-live-turn", activeTurnStartedAt: 2000 });
  await h.emit({ kind: "approval_request", tabId: h.active.id, approval: { id: "live-prompt", tool: "read_file", subject: "synthetic" } });
  assert.equal(h.controller.state.running, true);
  assert.ok(h.controller.state.approval, "the actual event subscription receives a new valid prompt during the query");
  await act(async () => { status.resolve([idleSnapshot]); await tick(); });
  assert.equal(h.controller.state.running, false, "characterizes the current late-idle race, not desired acceptance behavior");
  assert.equal(h.controller.state.approval, undefined);
  await h.close();
  console.log("OBSERVED stale query: an idle snapshot captured before new live events clears their runtime presentation");
}

console.log("Session history handoff and hydration characterization checks passed.");
