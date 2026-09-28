// Run: tsx src/__tests__/expert-workspace-virtualization.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import gsap from "gsap";
import React, { act, type ReactNode, useLayoutEffect, useState } from "react";
import { flushSync } from "react-dom";
import { createRoot, type Root } from "react-dom/client";

import { AgentWorkspace } from "../components/AgentWorkspace";
import { ExpertTeamSessionView } from "../components/ExpertTeamSession";
import { Transcript } from "../components/Transcript";
import { transcriptVirtualRowMeasured, type TranscriptVirtualRow } from "../components/TranscriptVirtualRows";
import { useExpertCatalog } from "../features/experts/controller/useExpertCatalog";
import type { ExpertSelection } from "../features/experts/ui/ExpertDirectory";
import { LocaleProvider } from "../lib/i18n";
import { markNoticeForReveal } from "../lib/sessionItems";
import type { ExpertTeamSession } from "../lib/types";
import { questionAnchorId } from "../lib/transcriptGrouping";
import type { Item } from "../lib/useController";
import type { SessionScrollAnchor } from "../features/session/sessionViewSnapshot";

type ToolItem = Extract<Item, { kind: "tool" }>;

// The tsx/JSDOM runner exposes GSAP through its CommonJS wrapper. This suite
// asserts semantic scroll state, not animation timing, so keep tweens inert.
Object.defineProperty(gsap, "to", { configurable: true, value: () => ({ kill() {} }) });
Object.defineProperty(gsap, "fromTo", { configurable: true, value: () => ({ kill() {} }) });

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

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

async function settleLayout() {
  await new Promise((resolve) => setTimeout(resolve, 150));
}

async function nextAnimationFrame() {
  await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
}

class TestResizeObserver {
  private readonly callback: ResizeObserverCallback;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
  }

  observe(target: Element) {
    this.callback([{
      target,
      contentRect: { width: 1_000, height: 800 } as DOMRectReadOnly,
      borderBoxSize: [{ inlineSize: 1_000, blockSize: 800 }],
      contentBoxSize: [{ inlineSize: 1_000, blockSize: 800 }],
      devicePixelContentBoxSize: [{ inlineSize: 1_000, blockSize: 800 }],
    }], this as unknown as ResizeObserver);
  }
  unobserve() {}
  disconnect() {}
}

class MeasuredRowResizeObserver {
  private readonly callback: ResizeObserverCallback;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
  }

  observe(target: Element) {
    const height = target.classList.contains("transcript-virtual-row") ? 180 : 800;
    this.callback([{
      target,
      contentRect: { width: 1_000, height } as DOMRectReadOnly,
      borderBoxSize: [{ inlineSize: 1_000, blockSize: height }],
      contentBoxSize: [{ inlineSize: 1_000, blockSize: height }],
      devicePixelContentBoxSize: [{ inlineSize: 1_000, blockSize: height }],
    }], this as unknown as ResizeObserver);
  }
  unobserve() {}
  disconnect() {}
}

class VariableRowResizeObserver {
  private readonly callback: ResizeObserverCallback;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
  }

  observe(target: Element) {
    const height = target.classList.contains("transcript-virtual-row") ? 260 : 800;
    this.callback([{
      target,
      contentRect: { width: 1_000, height } as DOMRectReadOnly,
      borderBoxSize: [{ inlineSize: 1_000, blockSize: height }],
      contentBoxSize: [{ inlineSize: 1_000, blockSize: height }],
      devicePixelContentBoxSize: [{ inlineSize: 1_000, blockSize: height }],
    }], this as unknown as ResizeObserver);
  }
  unobserve() {}
  disconnect() {}
}

class SilentResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

const realPerformanceNow = performance.now.bind(performance);
function installDom() {
  Object.defineProperty(performance, "now", { configurable: true, value: realPerformanceNow });
  const dom = new JSDOM("<!doctype html><html><body><div id=\"scroll\"><div id=\"root\"></div></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.Element = dom.window.Element;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.Node = dom.window.Node;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  globalThis.ResizeObserver = TestResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof TestResizeObserver }).ResizeObserver = TestResizeObserver;
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", { configurable: true, value: () => {} });
  const scrollElement = document.getElementById("scroll") as HTMLDivElement;
  Object.defineProperty(scrollElement, "clientHeight", { configurable: true, value: 800 });
  Object.defineProperty(scrollElement, "scrollHeight", { configurable: true, value: 20_000 });
  Object.defineProperty(scrollElement, "getBoundingClientRect", {
    configurable: true,
    value: () => ({ left: 0, top: 0, right: 1_000, bottom: 800, width: 1_000, height: 800, x: 0, y: 0, toJSON: () => {} }),
  });
  return { dom, scrollElement };
}

function overviewRows(prefix: string, count: number): ReactNode[] {
  return Array.from({ length: count }, (_, index) => <div data-history-row={`${prefix}-${index}`} key={`${prefix}-${index}`}>{prefix} {index}</div>);
}

function task(id: string, label: string): ToolItem {
  return {
    kind: "tool",
    id,
    name: "task",
    args: JSON.stringify({ description: label }),
    readOnly: false,
    status: "done",
    output: `${label} complete`,
  };
}

function persistedTeam(id: string, name: string): ExpertTeamSession {
  return {
    state: "team_active",
    id,
    name,
    lead: { memberKey: "lead", name: `${name} Lead`, role: "lead" },
    members: [],
  };
}

function assistantItems(prefix: string, count: number, parentId?: string): Item[] {
  return Array.from({ length: count }, (_, index) => ({
    kind: "assistant" as const,
    id: `${prefix}-${index}`,
    parentId,
    text: `${prefix} answer ${index}`,
    reasoning: "",
    streaming: false,
  }));
}

function questionItems(prefix: string, count: number): Item[] {
  return Array.from({ length: count }, (_, index) => [
    { kind: "user" as const, id: `${prefix}-question-${index}`, text: `${prefix} question ${index}` },
    { kind: "assistant" as const, id: `${prefix}-answer-${index}`, text: `${prefix} answer ${index}`, reasoning: "", streaming: false },
  ]).flat();
}

function mountedVirtualRows(testId: string): number {
  return document.querySelector(`[data-testid="${testId}"]`)?.querySelectorAll(".transcript-virtual-row").length ?? 0;
}

async function scrollViewport(scrollElement: HTMLDivElement, top: number) {
  scrollElement.scrollTop = top;
  scrollElement.dispatchEvent(new window.Event("scroll"));
  await flush();
}

function render(root: Root, node: ReactNode) {
  root.render(<LocaleProvider>{node}</LocaleProvider>);
}

function AppExpertTranscriptHarness({ items, tabId, viewKey }: { items: Item[]; tabId: string; viewKey: string }) {
  const [, setRecentExperts] = useState<ExpertSelection[]>([]);
  const { activeExpert } = useExpertCatalog({
    activeTabId: tabId,
    viewKey,
    canClearSelection: true,
    setRecentExperts,
  });
  return (
    <div data-testid="app-expert-transcript-harness" data-active-expert-id={activeExpert?.id ?? ""}>
      <Transcript items={items} tabId={tabId} viewKey={viewKey} onPrompt={() => {}} activeExpert={activeExpert} />
    </div>
  );
}

function invokeReactButtonClick(button: HTMLButtonElement | undefined) {
  if (!button) return;
  const propsKey = Object.keys(button).find((key) => key.startsWith("__reactProps$"));
  const onClick = propsKey
    ? (button as unknown as Record<string, { onClick?: () => void }>)[propsKey]?.onClick
    : undefined;
  if (!onClick) throw new Error("missing React button click handler");
  onClick();
}

function TranscriptWithLayoutScroll({
  layoutScrollTop,
  ...props
}: React.ComponentProps<typeof Transcript> & { layoutScrollTop?: number }) {
  useLayoutEffect(() => {
    if (layoutScrollTop === undefined) return;
    const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
    if (!transcript) throw new Error("missing layout-switch transcript scroller");
    for (const anchor of transcript.querySelectorAll<HTMLElement>("[data-question-anchor]")) {
      anchor.removeAttribute("id");
    }
    const virtualList = transcript.querySelector(".transcript-virtual-list") as HTMLDivElement | null;
    if (virtualList) {
      virtualList.getBoundingClientRect = () => ({ top: 240 - transcript.scrollTop }) as DOMRect;
    }
    transcript.scrollTop = layoutScrollTop;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
  }, [layoutScrollTop, props.viewKey]);
  return <Transcript {...props} />;
}

function instrumentTranscriptScroll(transcript: HTMLDivElement, initialScrollTop: number) {
  let scrollTop = initialScrollTop;
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 800 });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, value: 20_000 });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => { scrollTop = Number(value); },
  });
  return () => scrollTop;
}

function installManualAnimationFrames(dom: JSDOM) {
  let now = realPerformanceNow();
  Object.defineProperty(performance, "now", { configurable: true, value: () => now });
  const frames = new Map<number, FrameRequestCallback>();
  let nextFrameID = 0;
  const requestFrame = (callback: FrameRequestCallback) => {
    const id = ++nextFrameID;
    frames.set(id, callback);
    return id;
  };
  const cancelFrame = (id: number) => frames.delete(id);
  globalThis.requestAnimationFrame = requestFrame;
  globalThis.cancelAnimationFrame = cancelFrame;
  dom.window.requestAnimationFrame = requestFrame;
  dom.window.cancelAnimationFrame = cancelFrame;
  return {
    pendingCount: () => frames.size,
    async flushOne() {
      now += 1000 / 60;
      const callbacks = [...frames.values()];
      frames.clear();
      await act(async () => {
        for (const callback of callbacks) callback(now);
        await flush();
      });
    },
    async flushAll(limit = 200) {
      for (let index = 0; index < limit && frames.size > 0; index += 1) await this.flushOne();
    },
  };
}

function installWindowFakeClock(dom: JSDOM) {
  const originalSetTimeout = dom.window.setTimeout.bind(dom.window);
  const originalClearTimeout = dom.window.clearTimeout.bind(dom.window);
  let now = 0;
  let nextID = 0;
  const timers = new Map<number, { at: number; callback: () => void }>();
  dom.window.setTimeout = ((handler: TimerHandler, timeout = 0, ...args: unknown[]) => {
    const id = ++nextID;
    timers.set(id, {
      at: now + Number(timeout || 0),
      callback: () => {
        if (typeof handler === "function") handler(...args);
      },
    });
    return id;
  }) as typeof dom.window.setTimeout;
  dom.window.clearTimeout = ((id: number) => timers.delete(id)) as typeof dom.window.clearTimeout;
  return {
    advance(ms: number) {
      now += ms;
      for (;;) {
        const due = [...timers.entries()]
          .filter(([, timer]) => timer.at <= now)
          .sort((left, right) => left[1].at - right[1].at || left[0] - right[0]);
        if (due.length === 0) return;
        const [id, timer] = due[0];
        timers.delete(id);
        timer.callback();
      }
    },
    restore() {
      timers.clear();
      dom.window.setTimeout = originalSetTimeout;
      dom.window.clearTimeout = originalClearTimeout;
    },
  };
}

console.log("\nexpert workspace virtualization");

{
  const measuredKeys = new Set(["old-question-row"]);
  const oldRows = Array.from({ length: 40 }, (_, index): TranscriptVirtualRow => ({
    key: index === 33 ? "old-question-row" : `old-row-${index}`,
    node: null,
  }));
  const replacementRows = oldRows.map((row, index) => index === 33
    ? { ...row, key: "replacement-question-row" }
    : row);
  ok(transcriptVirtualRowMeasured(measuredKeys, oldRows, 33), "same-view old row key retains its acquired measurement");
  ok(!transcriptVirtualRowMeasured(measuredKeys, replacementRows, 33), "same-view key replacement cannot reuse the old row index measurement");
}

for (const initialScrollAnchor of [{ atBottom: true } as const, undefined]) {
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>(() => {}) } },
  };
  await act(async () => {
    render(root,
      <Transcript
        items={[task("persisted-authority-task", "Persisted Authority")]}
        tabId="persisted-authority-tab"
        viewKey={`persisted-authority-${initialScrollAnchor ? "bottom" : "initial"}`}
        onPrompt={() => {}}
        activeExpert={{ id: "persisted-authority-team", name: "Persisted Authority Team", mode: "team" }}
        initialScrollAnchor={initialScrollAnchor}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const scenario = initialScrollAnchor ? "true-bottom" : "no-anchor";
  ok(document.querySelector('.transcript__session-loading') === null && document.querySelector('.tool') !== null, `team ${scenario} keeps parent content readable while sidecar is pending`);
  ok(document.querySelector('[data-testid="agent-workspace-overview-virtual-list"]') === null, `team ${scenario} never exposes legacy workspace`);
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const clock = installWindowFakeClock(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team = persistedTeam("continuous-loading-team", "Continuous Loading Team");
  let resolveTeam: ((value: ExpertTeamSession | null) => void) | undefined;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  await act(async () => {
    render(root,
      <Transcript
        items={[task("continuous-loading-task", "Continuous Loading"), ...questionItems("continuous-loading", 12)]}
        tabId="continuous-loading-tab"
        viewKey="continuous-loading-view"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: "continuous-loading-question-8", offset: 24 }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  ok(document.querySelector('.transcript__session-loading:not([role="status"])') !== null, "expert pending starts with one quiet protection surface");
  await act(async () => {
    clock.advance(100);
    resolveTeam?.(team);
    await flush();
  });
  ok(document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === "true" && frames.pendingCount() > 0, "pending-to-persisted keeps the unstable presentation protected");
  await act(async () => {
    clock.advance(79);
  });
  ok(document.querySelector('.transcript__session-loading[role="status"]') === null, "pending-to-persisted remains silent through the original 179ms window");
  await act(async () => {
    clock.advance(1);
  });
  ok(document.querySelectorAll('.transcript__session-loading[role="status"]').length === 1, "pending-to-persisted preserves one timer and reveals one status at the original 180ms boundary");
  await act(async () => root.unmount());
  clock.restore();
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team = persistedTeam("retry-team", "Retry Team");
  let attempts = 0;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => (++attempts === 1 ? null : team) } },
  };
  await act(async () => {
    render(root,
      <Transcript
        items={[task("retry-team-task", "Retry Team"), ...questionItems("retry-team", 12)]}
        tabId="retry-team-tab"
        viewKey="retry-team-view"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: "retry-team-question-8", offset: 24 }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  ok(document.querySelector('.transcript__session-loading[role="alert"]') === null && document.querySelector(".agent-workspace") === null && document.querySelector(".tool") !== null, "missing persisted sidecar keeps the regular parent transcript usable");
  await act(async () => {
    render(root,
      <Transcript
        items={[task("retry-team-task", "Retry Team"), ...questionItems("retry-team", 12)]}
        tabId="retry-team-tab"
        viewKey="retry-team-view"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: "retry-team-question-8", offset: 24 }}
        historyRevalidationSignal={1}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  ok(attempts === 2 && document.querySelector('[data-team-state="team_active"]') !== null, "a later history refresh upgrades the fallback into the persisted team presentation");
  ok(document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === "true" && frames.pendingCount() > 0, "fallback-to-persisted upgrade preserves and restarts the semantic restore");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const teamA = persistedTeam("same-view-team-a", "Same View Team A");
  const teamB = persistedTeam("same-view-team-b", "Same View Team B");
  let selectedID = teamA.id;
  let resolveTeamB: ((team: ExpertTeamSession | null) => void) | undefined;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => selectedID === teamA.id
      ? Promise.resolve(teamA)
      : new Promise<ExpertTeamSession | null>((resolve) => { resolveTeamB = resolve; }) } },
  };
  const transcript = (team: ExpertTeamSession) => (
    <Transcript
      items={[task("same-view-team-task", "Same View Team")]}
      tabId="same-view-team-tab"
      viewKey="same-view-team-session"
      onPrompt={() => {}}
      activeExpert={{ id: team.id, name: team.name, mode: "team" }}
      questionNavigator={false}
    />
  );
  await act(async () => {
    render(root, transcript(teamA));
    await flush();
  });
  ok(document.body.textContent?.includes("Same View Team A Lead") === true, "same-view persisted cache initially renders team A");
  selectedID = teamB.id;
  await act(async () => {
    render(root, transcript(teamB));
    await flush();
  });
  ok(document.body.textContent?.includes("Same View Team A Lead") === true, "same-view current-identity refresh keeps the session team visible until the authoritative view resolves");
  await act(async () => {
    resolveTeamB?.(teamB);
    await flush();
  });
  ok(document.body.textContent?.includes("Same View Team B Lead") === true, "same-view team identity change resolves authoritative team B");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let currentExpertRequests = 0;
  let teamRequests = 0;
  const archivedTeam: ExpertTeamSession = {
    state: "team_archived",
    id: "archived-session-team",
    name: "Archived Session Team",
    lead: { memberKey: "lead", name: "Archived Lead", role: "coordination" },
    members: [{ memberKey: "review", name: "Archived Reviewer", role: "review", subagentRef: "sa_archived_review", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: {
      ListExperts: async () => [],
      CurrentExpertForTab: async () => {
        currentExpertRequests += 1;
        return { selectedId: "", selectedName: "", source: "none", canSwitch: false, requiresNewSession: true };
      },
      ListUserExpertsForTab: async () => [],
      ExpertLibraryStateForTab: async () => ({ favoriteIds: [], recentIds: [], usageCounts: {} }),
      ExpertTeamForTab: async () => {
        teamRequests += 1;
        return archivedTeam;
      },
    } },
  };

  await act(async () => {
    render(root,
      <AppExpertTranscriptHarness
        items={[{ kind: "user", id: "archived-session-question", text: "continue normally" }]}
        tabId="archived-session-tab"
        viewKey="archived-session-view"
      />,
    );
    await flush();
    await flush();
  });

  const harness = document.querySelector('[data-testid="app-expert-transcript-harness"]');
  ok(currentExpertRequests === 1 && harness?.getAttribute("data-active-expert-id") === "", "App expert state resolves archived CurrentExpertForTab none into activeExpert null");
  ok(teamRequests === 1 && document.querySelector('[data-team-state="team_archived"]') !== null, "real Transcript restores the archived session team independently of current expert identity");
  ok(document.body.textContent?.includes("Archived Lead") === true && document.body.textContent?.includes("Archived Reviewer") === true, "archived sidecar members drive the historical workspace without task fallback");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const clock = installWindowFakeClock(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team = persistedTeam("transient-sidecar-team", "Transient Sidecar Team");
  let attempts = 0;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => {
      attempts += 1;
      if (attempts === 1) throw new Error("session path is still being created");
      return team;
    } } },
  };

  await act(async () => {
    render(root,
      <Transcript
        items={[task("transient-sidecar-task", "Transient Sidecar Team")]}
        tabId="transient-sidecar-tab"
        viewKey="transient-sidecar-session"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
      />,
    );
    await flush();
  });
  ok(document.querySelector('.transcript__session-loading[role="alert"]') === null, "a transient expert-sidecar read failure remains recoverable without an immediate error panel");
  await act(async () => {
    clock.advance(250);
    await flush();
  });
  ok(attempts === 2 && document.querySelector('[data-team-state="team_active"]') !== null, "the transient expert-sidecar read automatically retries into the persisted workspace");

  await act(async () => root.unmount());
  clock.restore();
  dom.window.close();
}

{
  const { dom } = installDom();
  const clock = installWindowFakeClock(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let attempts = 0;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => {
      attempts += 1;
      throw new Error("session sidecar remains unavailable");
    } } },
  };

  await act(async () => {
    render(root,
      <Transcript
        items={[task("unavailable-sidecar-task", "Unavailable Sidecar Team")]}
        tabId="unavailable-sidecar-tab"
        viewKey="unavailable-sidecar-session"
        onPrompt={() => {}}
        activeExpert={{ id: "unavailable-sidecar-team", name: "Unavailable Sidecar Team", mode: "team" }}
      />,
    );
    await flush();
  });
  await act(async () => {
    clock.advance(250);
    await flush();
  });
  await act(async () => {
    clock.advance(250);
    await flush();
  });
  ok(attempts === 3 && document.querySelector('.transcript__session-loading[role="alert"]') === null && document.querySelector(".agent-workspace") === null && document.querySelector(".tool") !== null, "a persistently unavailable expert sidecar leaves the regular parent transcript usable after bounded automatic attempts");

  await act(async () => root.unmount());
  clock.restore();
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => null } },
  };

  await act(async () => {
    render(root,
      <Transcript
        items={[]}
        tabId="blank-team-missing-sidecar-tab"
        viewKey="blank-team-missing-sidecar-session"
        onPrompt={() => {}}
        activeExpert={{ id: "blank-team-missing-sidecar", name: "Blank Team", mode: "team" }}
      />,
    );
    await flush();
  });
  ok(document.querySelector('.transcript__session-loading[role="alert"]') === null, "a blank selected team does not show a sidecar-read failure before its first message");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const source = readFileSync(new URL("../components/Transcript.tsx", import.meta.url), "utf8");
  const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
  const tracePathStart = source.indexOf("const tracePath =");
  const firstCommittedEffect = Math.min(
    ...[source.indexOf("useLayoutEffect(() =>", tracePathStart), source.indexOf("useEffect(() =>", tracePathStart)]
      .filter((index) => index >= 0),
  );
  const renderOnlySection = source.slice(tracePathStart, firstCommittedEffect);
  const deferredGateStart = source.indexOf("const deferExpertReturnPresentation =");
  const deferredGateAssignment = source.indexOf("expertReturnPresentationDeferred.current = deferExpertReturnPresentation", deferredGateStart);
  const deferredGateCommitStart = source.lastIndexOf("useLayoutEffect(() =>", deferredGateAssignment);
  const deferredGateEffectStart = source.indexOf("useEffect(() =>", deferredGateCommitStart);
  const deferredGateRenderSection = source.slice(deferredGateStart, deferredGateCommitStart);
  const deferredGateCommitSection = source.slice(deferredGateCommitStart, deferredGateEffectStart);
  const handleScrollStart = source.indexOf("const handleScroll =", tracePathStart);
  const handleScrollSection = source.slice(handleScrollStart, source.indexOf("useLayoutEffect(() =>", handleScrollStart));
  ok(
    tracePathStart >= 0 && firstCommittedEffect > tracePathStart && !renderOnlySection.includes("presentationRestoreSnapshot.current = {"),
    "concurrent render cannot mutate the committed presentation restore snapshot",
  );
  ok(
    deferredGateStart >= 0 && deferredGateAssignment > deferredGateStart && deferredGateCommitStart > deferredGateStart && deferredGateEffectStart > deferredGateCommitStart
      && !deferredGateRenderSection.includes("expertReturnPresentationDeferred.current =")
      && deferredGateCommitSection.includes("expertReturnPresentationDeferred.current = deferExpertReturnPresentation"),
    "concurrent render cannot mutate the committed deferred-return input boundary",
  );
  ok(
    handleScrollStart >= 0 && (handleScrollSection.match(/syncPresentationRestoreSnapshot\(/g)?.length ?? 0) === 3,
    "legacy scroll handler synchronizes protected-bottom, bottom, and exact anchors without clearing a transient missing anchor",
  );
  ok(
    handleScrollSection.includes("if (userInitiated && replacementPresentationHidden.current)")
      && handleScrollSection.includes("virtualLayoutReplacementAnchor.current = null")
      && handleScrollSection.includes("revealSemanticPresentation()"),
    "real user input cancels a hidden layout replacement instead of replaying its programmatic anchor",
  );
  const revealEffectStart = source.indexOf("if (!revealNotice || !consumeNoticeReveal");
  const revealEffectEnd = source.indexOf("const traceRenderSignature", revealEffectStart);
  const revealEffectSource = source.slice(revealEffectStart, revealEffectEnd);
  ok(
    revealEffectStart >= 0 && revealEffectEnd > revealEffectStart
      && revealEffectSource.includes('deferredExpertReturnRevealBottom.current = activeExpert?.mode === "team"'),
    "ordinary hidden reveal cannot arm a deferred expert bottom replay for the same view",
  );
  const messageActionStart = appSource.indexOf("const handleMessageAction =");
  const messageActionEnd = appSource.indexOf("const handleEditPrompt =", messageActionStart);
  const messageActionSource = appSource.slice(messageActionStart, messageActionEnd);
  const codeOnlyStart = messageActionSource.indexOf('if (scope === "code")');
  const summarizeStart = messageActionSource.indexOf('if (scope === "summ-from"', codeOnlyStart);
  const codeOnlySource = messageActionSource.slice(codeOnlyStart, summarizeStart);
  ok(
    messageActionStart >= 0 && messageActionEnd > messageActionStart && codeOnlyStart >= 0 && summarizeStart > codeOnlyStart
      && codeOnlySource.includes("rewind(turn, scope)") && !codeOnlySource.includes("setRewindSignal"),
    "App code-only rewind executes without incrementing the Transcript expert-team reload signal",
  );
}

{
  const { dom, scrollElement } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const delegated = task("legacy-expert", "Legacy Expert");
  const childItems = assistantItems("legacy-child", 48, delegated.id);

  await act(async () => {
    render(root,
      <AgentWorkspace
        items={[delegated]}
        subcalls={new Map([[delegated.id, childItems]])}
        overviewContent={overviewRows("legacy-overview", 48)}
        selectionScopeKey="legacy-session"
        scrollElement={scrollElement}
      />,
    );
    await flush();
  });

  const overviewMounted = mountedVirtualRows("agent-workspace-overview-virtual-list");
  ok(overviewMounted > 0 && overviewMounted < 48, "compatibility overview mounts a bounded non-empty row window");
  await act(async () => scrollViewport(scrollElement, 3_600));
  ok(mountedVirtualRows("agent-workspace-overview-virtual-list") > 0, "compatibility overview keeps a mounted window at a deep parent scroll offset");

  await act(async () => {
    Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Legacy Expert"))
      ?.click();
    await flush();
  });
  const focusedMounted = mountedVirtualRows("agent-workspace-focused-virtual-list");
  ok(focusedMounted > 0 && focusedMounted < childItems.length, "compatibility focused child mounts a bounded non-empty row window");
  await act(async () => scrollViewport(scrollElement, 3_600));
  ok(mountedVirtualRows("agent-workspace-focused-virtual-list") > 0, "compatibility focused child keeps a mounted window at a deep parent scroll offset");
  ok(document.querySelector(".agent-workspace__view")?.getAttribute("data-view") === "focused", "compatibility member selection still opens focused history");

  await act(async () => {
    render(root,
      <AgentWorkspace
        items={[delegated]}
        subcalls={new Map([[delegated.id, childItems]])}
        overviewContent={overviewRows("short-overview", 3)}
        selectionScopeKey="another-session"
        scrollElement={scrollElement}
      />,
    );
    await flush();
  });
  ok(document.querySelector('[data-testid="agent-workspace-overview-virtual-list"]') === null, "compatibility short overview keeps direct rendering");
  ok(document.querySelectorAll("[data-history-row^=\"short-overview\"]").length === 3, "scope switch resets the compatibility workspace to the new overview");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom, scrollElement } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const childMessages = Array.from({ length: 48 }, (_, index) => ({ role: "assistant", content: `persisted child answer ${index}` }));
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        SubagentHistoryPageForTab: async () => ({
          messages: childMessages,
          before: 0,
          hasMore: false,
          generation: "long-child-history",
        }),
      },
    },
  };
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "persisted-team",
    name: "Persisted Team",
    lead: { memberKey: "lead", name: "Lead Expert", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Review Expert", role: "review", subagentRef: "sa-long", status: "completed" }],
  };

  await act(async () => {
    render(root,
      <ExpertTeamSessionView
        team={team}
        tabId="parent-tab"
        childItemsByRef={{}}
        overviewContent={overviewRows("persisted-overview", 48)}
        selectionScopeKey="persisted-session"
        scrollElement={scrollElement}
      />,
    );
    await flush();
  });
  const overviewMounted = mountedVirtualRows("expert-team-overview-virtual-list");
  ok(overviewMounted > 0 && overviewMounted < 48, "persisted overview mounts a bounded non-empty row window");
  await act(async () => scrollViewport(scrollElement, 3_600));
  ok(mountedVirtualRows("expert-team-overview-virtual-list") > 0, "persisted overview keeps a mounted window at a deep parent scroll offset");

  await act(async () => {
    Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Review Expert"))
      ?.click();
    await flush();
    await flush();
  });
  const childMounted = mountedVirtualRows("expert-team-child-virtual-list");
  ok(childMounted > 0 && childMounted < childMessages.length, "persisted focused child mounts a bounded non-empty row window");
  await act(async () => scrollViewport(scrollElement, 3_600));
  ok(mountedVirtualRows("expert-team-child-virtual-list") > 0, "persisted focused child keeps a mounted window at a deep parent scroll offset");
  ok(document.querySelector(".msg--assistant") !== null, "persisted focused child keeps restored history content visible");

  await act(async () => {
    Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Lead Expert"))
      ?.click();
    await flush();
  });
  ok(mountedVirtualRows("expert-team-overview-virtual-list") > 0, "returning to the lead restores a non-empty overview window");

  await act(async () => {
    render(root,
      <ExpertTeamSessionView
        team={team}
        tabId="parent-tab"
        childItemsByRef={{}}
        overviewContent={overviewRows("persisted-short", 3)}
        selectionScopeKey="persisted-session-short"
        scrollElement={scrollElement}
      />,
    );
    await flush();
  });
  ok(document.querySelector('[data-testid="expert-team-overview-virtual-list"]') === null, "persisted short overview keeps direct rendering");
  ok(document.querySelectorAll("[data-history-row^=\"persisted-short\"]").length === 3, "persisted session scope switch resets member selection and keeps all short rows");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = VariableRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = VariableRowResizeObserver as unknown as typeof ResizeObserver;
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "offscreen-geometry-team",
    name: "Offscreen Geometry Team",
    lead: { memberKey: "lead", name: "Geometry Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Geometry Reviewer", role: "review", subagentRef: "sa-offscreen-geometry", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  const virtualSeekTops: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
      virtualSeekTops.push(top);
      this.scrollTop = top;
      this.dispatchEvent(new window.Event("scroll"));
    },
  });
  const anchorMessageId = "offscreen-geometry-question-8";
  const items: Item[] = [task("offscreen-geometry-task", "Geometry Reviewer"), ...questionItems("offscreen-geometry", 12)];
  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="offscreen-geometry-tab"
        viewKey="offscreen-geometry-session"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing offscreen geometry transcript");
  let scrollTop = 0;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = Number(value);
      },
    },
  });
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const querySelectorAll = transcript.querySelectorAll.bind(transcript);
  let anchorDOMAvailable = false;
  Object.defineProperty(transcript, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !anchorDOMAvailable ? [] : querySelectorAll(selector),
  });

  virtualSeekTops.length = 0;
  await act(async () => {
    resolveTeam?.(team);
    await flush();
  });
  const virtualList = transcript.querySelector(".transcript-virtual-list") as HTMLElement | null;
  if (!virtualList) throw new Error("missing persisted offscreen virtual list");
  const hiddenPersisted = document.querySelector<HTMLElement>('[data-testid="persisted-expert-presentation"]');
  ok(hiddenPersisted?.getAttribute("aria-hidden") === "true" && hiddenPersisted.style.visibility === "hidden", "persisted semantic restore remains hidden while acquiring target geometry");
  ok(hiddenPersisted?.style.display === "contents", "persisted hidden prelayout uses the final presentation box tree");
  await frames.flushOne();
  const targetNode = Array.from(querySelectorAll<HTMLElement>("[id]")).find((node) => node.id === questionAnchorId(anchorMessageId));
  const targetRow = targetNode?.closest<HTMLElement>(".transcript-virtual-row");
  const targetRowStart = Number(targetRow?.style.transform.match(/translateY\(([-\d.]+)px\)/)?.[1] ?? Number.NaN);
  ok(Boolean(targetNode) && Number.isFinite(targetRowStart), "persisted target row is mounted and measured after its single seek");
  virtualList.getBoundingClientRect = () => ({ top: 1_000 - targetRowStart - scrollTop }) as DOMRect;
  if (targetNode) targetNode.getBoundingClientRect = () => ({ top: 1_000 - scrollTop }) as DOMRect;
  anchorDOMAvailable = true;
  await frames.flushOne();
  anchorDOMAvailable = false;
  await frames.flushAll();
  const semanticSeekTops = virtualSeekTops.filter((top) => Math.abs(top) > 1);
  ok(semanticSeekTops.length >= 1, `persisted single semantic seek reaches the unmeasured target (${semanticSeekTops.length} virtualizer adjustments)`);
  ok(Math.abs(scrollTop - 1_024) <= 1, `persisted offscreen restore converges after target DOM leaves overscan (${scrollTop})`);
  const revealedPersisted = document.querySelector<HTMLElement>('[data-testid="persisted-expert-presentation"]');
  ok(revealedPersisted?.getAttribute("aria-hidden") === null, "persisted semantic presentation reveals once after virtual geometry is stable");
  ok(revealedPersisted?.style.display === "contents", "persisted reveal preserves the prelayout box tree");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = VariableRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = VariableRowResizeObserver as unknown as typeof ResizeObserver;
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "clamped-geometry-team",
    name: "Clamped Geometry Team",
    lead: { memberKey: "lead", name: "Geometry Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Geometry Reviewer", role: "review", subagentRef: "sa-clamped-geometry", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  const virtualSeekTops: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
      virtualSeekTops.push(top);
      this.scrollTop = top;
      this.dispatchEvent(new window.Event("scroll"));
    },
  });
  const anchorMessageId = "clamped-geometry-question-8";
  const anchorOffset = 732.200_195_312_5;
  const finalMaxScrollTop = 6_281.600_097_656_25;
  const closestOffset = 611.600_097_656_25;
  const items: Item[] = [task("clamped-geometry-task", "Geometry Reviewer"), ...questionItems("clamped-geometry", 12)];
  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="clamped-geometry-tab"
        viewKey="clamped-geometry-session"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: anchorOffset }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing clamped geometry transcript");
  let scrollTop = 0;
  let scrollHeight = 20_000;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, get: () => scrollHeight },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = Math.max(0, Math.min(Number(value), scrollHeight - 800));
      },
    },
  });
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const querySelectorAll = transcript.querySelectorAll.bind(transcript);
  let anchorDOMAvailable = false;
  Object.defineProperty(transcript, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !anchorDOMAvailable ? [] : querySelectorAll(selector),
  });

  await act(async () => {
    resolveTeam?.(team);
    await flush();
  });
  const virtualList = transcript.querySelector(".transcript-virtual-list") as HTMLElement | null;
  if (!virtualList) throw new Error("missing persisted clamped virtual list");
  virtualSeekTops.length = 0;
  await frames.flushOne();
  const targetNode = Array.from(querySelectorAll<HTMLElement>("[id]")).find((node) => node.id === questionAnchorId(anchorMessageId));
  const targetRow = targetNode?.closest<HTMLElement>(".transcript-virtual-row");
  const targetRowStart = Number(targetRow?.style.transform.match(/translateY\(([-\d.]+)px\)/)?.[1] ?? Number.NaN);
  ok(Boolean(targetNode) && Number.isFinite(targetRowStart), "deep-offset target is measured after its single virtual seek");
  const referenceTop = finalMaxScrollTop - closestOffset;
  virtualList.getBoundingClientRect = () => ({ top: referenceTop - targetRowStart - scrollTop }) as DOMRect;
  if (targetNode) targetNode.getBoundingClientRect = () => ({ top: referenceTop - scrollTop }) as DOMRect;
  scrollHeight = finalMaxScrollTop + 800;
  anchorDOMAvailable = true;
  await frames.flushOne();
  anchorDOMAvailable = false;
  await frames.flushAll();

  ok(Math.abs(scrollTop - finalMaxScrollTop) <= 0.001, "deep-offset restore reaches the physical bottom clamp after measured heights shrink");
  ok(Math.abs(anchorOffset - closestOffset - 120.600_097_656_25) <= 0.001, "the remaining 120.6px is the unreachable semantic offset beyond the clamp");
  ok(virtualSeekTops.filter((top) => Math.abs(top) > 1).length >= 1, "deep-offset restore retains the bounded single-seek path");
  ok(document.querySelector('.transcript__session-loading[role="alert"]') === null, "a measured deep semantic anchor reveals at its closest physically reachable offset");
  const revealedPersisted = document.querySelector<HTMLElement>('[data-testid="persisted-expert-presentation"]');
  ok(revealedPersisted?.getAttribute("aria-hidden") === null, "a bottom-clamped semantic presentation reveals without exposing correction frames");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = VariableRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = VariableRowResizeObserver as unknown as typeof ResizeObserver;
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const virtualSeekTops: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
      virtualSeekTops.push(top);
      this.scrollTop = top;
      this.dispatchEvent(new window.Event("scroll"));
    },
  });
  const anchorMessageId = "ordinary-offscreen-question-8";
  const committedOrdinaryItems = questionItems("ordinary-offscreen", 12);
  const neverResolve = new Promise<void>(() => {});
  const SuspendAfterTranscript = ({ suspend }: { suspend: boolean }) => {
    if (suspend) throw neverResolve;
    return null;
  };
  const ordinaryRestoreNode = (items: Item[], suspend = false) => (
    <React.Suspense fallback={null}>
      <Transcript
        items={items}
        viewKey="ordinary-offscreen-session"
        onPrompt={() => {}}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />
      <SuspendAfterTranscript suspend={suspend} />
    </React.Suspense>
  );
  await act(async () => {
    render(root, ordinaryRestoreNode(committedOrdinaryItems));
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing ordinary offscreen transcript");
  let scrollTop = 0;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => { scrollTop = Number(value); },
    },
  });
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const querySelectorAll = transcript.querySelectorAll.bind(transcript);
  let anchorDOMAvailable = false;
  Object.defineProperty(transcript, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !anchorDOMAvailable ? [] : querySelectorAll(selector),
  });
  const virtualList = transcript.querySelector(".transcript-virtual-list") as HTMLElement | null;
  if (!virtualList) throw new Error("missing ordinary offscreen virtual list");
  const hiddenOrdinary = document.querySelector<HTMLElement>('[data-testid="ordinary-semantic-presentation"]');
  ok(hiddenOrdinary?.getAttribute("aria-hidden") === "true" && hiddenOrdinary.style.visibility === "hidden", "ordinary long semantic restore remains hidden while acquiring target geometry");
  ok(hiddenOrdinary?.style.display === "contents", "ordinary hidden prelayout uses the final presentation box tree");
  await act(async () => {
    React.startTransition(() => render(root, ordinaryRestoreNode(questionItems("aborted-ordinary", 18), true)));
    await flush();
  });
  virtualSeekTops.length = 0;
  await frames.flushOne();
  const committedSeekCount = virtualSeekTops.filter((top) => Math.abs(top) > 1).length;
  ok(committedSeekCount >= 1, `aborted row-map render cannot redirect the committed semantic seek (${committedSeekCount} virtualizer adjustments)`);
  const updatedCommittedOrdinaryItems = [
    ...committedOrdinaryItems,
    ...questionItems("ordinary-offscreen-update", 1),
  ];
  await act(async () => {
    render(root, ordinaryRestoreNode(updatedCommittedOrdinaryItems));
    await flush();
  });
  let targetNode = Array.from(querySelectorAll<HTMLElement>("[id]")).find((node) => node.id === questionAnchorId(anchorMessageId));
  if (!targetNode) {
    await frames.flushOne();
    targetNode = Array.from(querySelectorAll<HTMLElement>("[id]")).find((node) => node.id === questionAnchorId(anchorMessageId));
  }
  ok(
    virtualSeekTops.filter((top) => Math.abs(top) > 1).length === committedSeekCount,
    "a same-view committed rows update cannot issue a second virtual seek in the active restore generation",
  );
  const targetRow = targetNode?.closest<HTMLElement>(".transcript-virtual-row");
  const targetRowStart = Number(targetRow?.style.transform.match(/translateY\(([-\d.]+)px\)/)?.[1] ?? Number.NaN);
  ok(Boolean(targetNode) && Number.isFinite(targetRowStart), "ordinary target row is mounted and measured after its single seek");
  virtualList.getBoundingClientRect = () => ({ top: 1_000 - targetRowStart - scrollTop }) as DOMRect;
  if (targetNode) targetNode.getBoundingClientRect = () => ({ top: 1_000 - scrollTop }) as DOMRect;
  anchorDOMAvailable = true;
  await frames.flushOne();
  anchorDOMAvailable = false;
  await frames.flushAll();
  const semanticSeekTops = virtualSeekTops.filter((top) => Math.abs(top) > 1);
  ok(document.querySelector(".transcript-virtual-list") !== null, "ordinary long restore uses the virtualized path");
  ok(semanticSeekTops.length >= 1, `ordinary single semantic seek reaches the unmeasured target (${semanticSeekTops.length} virtualizer adjustments)`);
  ok(document.querySelector('.transcript__restore-status') !== null, `ordinary offscreen restore degrades when only estimated geometry remains (${scrollTop})`);
  const revealedOrdinary = document.querySelector<HTMLElement>('[data-testid="ordinary-semantic-presentation"]');
  ok(revealedOrdinary?.getAttribute("aria-hidden") === null, "ordinary long semantic presentation reveals once after virtual geometry is stable");
  ok(revealedOrdinary?.style.display === "contents", "ordinary reveal preserves the prelayout box tree");

  await act(async () => root.unmount());
  dom.window.close();
}

for (const scenario of ["ordinary", "expert"] as const) {
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team = persistedTeam("deleted-anchor-team", "Deleted Anchor Team");
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      this.scrollTop = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
    },
  });
  await act(async () => {
    render(root,
      <Transcript
        items={scenario === "expert"
          ? [task("deleted-anchor-task", "Deleted Anchor"), ...questionItems("deleted-anchor", 12)]
          : questionItems("deleted-anchor", 12)}
        tabId={scenario === "expert" ? "deleted-anchor-tab" : undefined}
        viewKey={`deleted-anchor-${scenario}`}
        onPrompt={() => {}}
        activeExpert={scenario === "expert" ? { id: team.id, name: team.name, mode: "team" } : undefined}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: "deleted-question", offset: 24 }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error(`missing ${scenario} deleted-anchor transcript`);
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
  });
  if (scenario === "expert") {
    await act(async () => {
      resolveTeam?.(team);
      await flush();
    });
  }
  await frames.flushAll();
  const presentation = document.querySelector(`[data-testid="${scenario === "expert" ? "persisted-expert" : "ordinary-semantic"}-presentation"]`);
  ok(transcript.scrollTop !== 20_000 && document.querySelector('.transcript__restore-status') !== null, `${scenario} missing anchor degrades without forcing bottom`);
  ok(presentation?.getAttribute("aria-hidden") === null && document.querySelector(".transcript__session-loading--overlay") === null, `${scenario} deleted anchor fallback reveals the presentation`);
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = SilentResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = SilentResizeObserver as unknown as typeof ResizeObserver;
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const anchorMessageId = "unresolved-geometry-question-10";
  const unresolvedItems = questionItems("unresolved-geometry", 35);
  const unresolvedNode = (items: Item[]) => (
    <Transcript
      items={items}
      viewKey="unresolved-geometry-view"
      onPrompt={() => {}}
      initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
      questionNavigator={false}
    />
  );
  await act(async () => {
    render(root, unresolvedNode(unresolvedItems));
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing unresolved geometry transcript");
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
  });
  const querySelectorAll = transcript.querySelectorAll.bind(transcript);
  Object.defineProperty(transcript, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" ? [] : querySelectorAll(selector),
  });
  for (let index = 0; index < 175; index += 1) await frames.flushOne();
  ok(document.querySelector('.transcript__restore-status') === null, "unresolved restore remains within its original elapsed-time budget before the hard limit");
  const streamedItems = unresolvedItems.map((item) => item.kind === "assistant" && item.id === "unresolved-geometry-answer-34"
    ? { ...item, text: `${item.text} streamed` }
    : item);
  await act(async () => {
    render(root, unresolvedNode(streamedItems));
    await flush();
  });
  for (let index = 0; index < 6; index += 1) await frames.flushOne();
  const failure = document.querySelector<HTMLElement>('.transcript__restore-status');
  ok(failure !== null && failure.textContent?.includes("恢复") === true, "same-view assistant streaming cannot reset the active restore generation hard-limit budget");
  const sessionCSS = readFileSync(new URL("../styles/session.css", import.meta.url), "utf8");
  const failureRule = sessionCSS.match(/\.transcript__restore-status\s*\{([^}]*)\}/)?.[1] ?? "";
  ok(
    failure?.getAttribute("role") === "status" && /flex-wrap:\s*wrap/.test(failureRule)
      && document.querySelector('[data-testid="ordinary-semantic-presentation"]')?.getAttribute("aria-hidden") === null,
    "failure status wraps controls while keeping the transcript readable",
  );
  const transcriptSource = readFileSync(new URL("../components/Transcript.tsx", import.meta.url), "utf8");
  ok(
    transcriptSource.includes("!transcriptMounted.current") && transcriptSource.includes("transcriptMounted.current = false"),
    "Transcript unmount synchronously invalidates detached virtual-presentation work before it can start another restore generation",
  );
  ok(
    transcriptSource.includes("virtualQuestionRowByAnchorIDRef.current.get(questionAnchorId(anchor.anchorMessageId))")
      && !transcriptSource.includes("hotQuestionRowByAnchorID, hydrating"),
    "the active restore generation reads layout-committed row geometry without restarting when same-view rows commit",
  );
  const retry = Array.from(failure?.querySelectorAll<HTMLButtonElement>("button") ?? [])
    .find((button) => button.textContent?.includes("重试"));
  ok(Boolean(retry), "incomplete restore offers an explicit retry");
  retry?.focus();
  ok(retry?.tabIndex === 0 && retry.disabled === false && document.activeElement === retry, "failure retry is keyboard focusable as an enabled native button");
  await act(async () => {
    retry?.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    retry?.dispatchEvent(new window.Event("pointerup", { bubbles: true }));
    retry?.click();
    await flush();
  });
  ok(document.querySelector('[data-testid="ordinary-semantic-presentation"]')?.getAttribute("aria-hidden") === null && frames.pendingCount() > 0, "pointer retry starts a new job without hiding readable content");
  await frames.flushAll();
  const retryFailure = document.querySelector<HTMLElement>('.transcript__restore-status');
  const keyboardRetry = Array.from(retryFailure?.querySelectorAll<HTMLButtonElement>("button") ?? [])
    .find((button) => button.textContent?.includes("重试"));
  await act(async () => {
    keyboardRetry?.dispatchEvent(new window.KeyboardEvent("keydown", { key: " ", code: "Space", bubbles: true }));
    keyboardRetry?.click();
    await flush();
  });
  ok(document.querySelector('[data-testid="ordinary-semantic-presentation"]')?.getAttribute("aria-hidden") === null && frames.pendingCount() > 0, "Space retry starts a new job without hiding readable content");
  await frames.flushAll();
  const touchRetryFailure = document.querySelector<HTMLElement>('.transcript__restore-status');
  const touchRetry = Array.from(touchRetryFailure?.querySelectorAll<HTMLButtonElement>("button") ?? [])
    .find((button) => button.textContent?.includes("重试"));
  await act(async () => {
    touchRetry?.dispatchEvent(new window.Event("touchstart", { bubbles: true }));
    touchRetry?.click();
    await flush();
  });
  ok(document.querySelector('[data-testid="ordinary-semantic-presentation"]')?.getAttribute("aria-hidden") === null && frames.pendingCount() > 0, "touch retry starts a new job without hiding readable content");
  await frames.flushAll();
  const touchRetryResult = document.querySelector<HTMLElement>('.transcript__restore-status');
  const retryExit = Array.from(touchRetryResult?.querySelectorAll<HTMLButtonElement>("button") ?? [])
    .find((button) => button.textContent?.includes("显示"));
  ok(Boolean(retryExit), "incomplete restore retry remains recoverable with a show-current-content exit");
  await act(async () => {
    retryExit?.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    retryExit?.dispatchEvent(new window.Event("pointerup", { bubbles: true }));
    retryExit?.click();
    await flush();
  });
  const presentation = document.querySelector('[data-testid="ordinary-semantic-presentation"]');
  ok(presentation?.getAttribute("aria-hidden") === null, "show-current-content reveals the incomplete presentation");
  await frames.flushAll();
  ok(presentation?.getAttribute("aria-hidden") === null && frames.pendingCount() === 0, "the completed old restore RAF cannot reclaim the cancelled presentation");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const scrollToTops: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
      scrollToTops.push(top);
      this.scrollTop = top;
    },
  });
  const anchorMessageId = "ordinary-short-question-1";
  await act(async () => {
    render(root,
      <Transcript
        items={questionItems("ordinary-short", 2)}
        viewKey="ordinary-short-session"
        onPrompt={() => {}}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  const anchorNode = document.getElementById(questionAnchorId(anchorMessageId));
  if (!transcript || !anchorNode) throw new Error("missing ordinary short restore fixture");
  let scrollTop = 0;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 4_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => { scrollTop = Number(value); },
    },
  });
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  anchorNode.getBoundingClientRect = () => ({ top: 1_000 - scrollTop }) as DOMRect;
  await frames.flushAll();
  ok(document.querySelector(".transcript-virtual-list") === null, "ordinary short restore keeps direct rendering");
  ok(scrollToTops.length === 0, "ordinary short restore performs no virtual seek");
  ok(Math.abs(scrollTop - 1_024) <= 1, `ordinary short restore keeps exact DOM correction (${scrollTop})`);

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const restoreAssignments: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
      restoreAssignments.push(top);
      this.scrollTop = top;
    },
  });
  await act(async () => {
    render(root,
      <Transcript
        items={questionItems("ordinary-hidden-cancel", 12)}
        viewKey="ordinary-hidden-cancel-view"
        onPrompt={() => {}}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: "ordinary-hidden-cancel-question-8", offset: 24 }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing ordinary hidden cancellation transcript");
  let scrollTop = 0;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => { scrollTop = Number(value); },
    },
  });
  ok(document.querySelector('[data-testid="ordinary-semantic-presentation"]')?.getAttribute("aria-hidden") === "true", "ordinary long cancellation starts from a committed hidden restore");
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: 240 }));
    transcript.scrollTop = 120;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  ok(document.querySelector('[data-testid="ordinary-semantic-presentation"]')?.getAttribute("aria-hidden") === null, "ordinary user input immediately reveals the hidden presentation");
  restoreAssignments.length = 0;
  await frames.flushAll();
  ok(scrollTop === 120 && restoreAssignments.length === 0, "cancelled ordinary restore RAF cannot replay the old semantic anchor");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  await act(async () => {
    render(root,
      <Transcript
        items={questionItems("ordinary-bottom", 12)}
        viewKey="ordinary-bottom-session"
        onPrompt={() => {}}
        initialScrollAnchor={{ atBottom: true }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing ordinary bottom transcript");
  let scrollTop = 0;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => { scrollTop = Number(value); },
    },
  });
  await frames.flushAll();
  ok(Math.abs(scrollTop - 20_000) <= 1, `ordinary true-bottom restore keeps bottom following (${scrollTop})`);

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => persistedTeam("clamp-team", "Clamp Team") } },
  };
  const clampQuestion: Item = { kind: "user", id: "clamp-question-5", text: "Inspect the clamped expert result" };
  const items: Item[] = [
    { kind: "user", id: "clamp-prior-question", text: "Prepare the clamp investigation" },
    ...assistantItems("clamp-intro", 3),
    clampQuestion,
    task("clamp-task", "Clamp Reviewer"),
    ...assistantItems("clamp-result", 32),
  ];
  const capturedAnchors: SessionScrollAnchor[] = [];
  const capturedByView: Array<{ key: string; anchor: SessionScrollAnchor }> = [];

  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="clamp-tab"
        viewKey="clamp-session"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: clampQuestion.id, offset: 0 }}
        onScrollAnchorChange={(key, anchor) => {
          capturedAnchors.push(anchor);
          capturedByView.push({ key, anchor });
        }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing clamped transcript scroller");
  const virtualList = transcript.querySelector(".transcript-virtual-list") as HTMLDivElement | null;
  if (!virtualList) throw new Error("missing clamped virtual list");
  const targetQuestionNode = document.getElementById(questionAnchorId(clampQuestion.id));
  const targetRowIndex = Number(targetQuestionNode?.closest<HTMLElement>(".transcript-virtual-row")?.dataset.index ?? -1);
  ok(targetRowIndex > 0, "the clamp fixture uses the q5 identity at a measured non-zero virtual row");

  const geometry = { scrollHeight: 7_284, scrollTop: 6_810 };
  const clientHeight = 474;
  const expandedScrollHeight = 7_284;
  const clampedScrollHeight = 6_145;
  const semanticContentTop = 6_415;
  const targetRowStart = targetRowIndex * 180;
  const virtualListOrigin = semanticContentTop - targetRowStart;
  let measuredQuestionID = clampQuestion.id;
  let expandOnRestore = false;
  const maxScrollTop = () => Math.max(0, geometry.scrollHeight - clientHeight);
  Object.defineProperty(transcript, "clientHeight", { configurable: true, get: () => clientHeight });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, get: () => geometry.scrollHeight });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => geometry.scrollTop,
    set: (value) => { geometry.scrollTop = Math.max(0, Math.min(Number(value), maxScrollTop())); },
  });
  Object.defineProperty(transcript, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      if (expandOnRestore) geometry.scrollHeight = expandedScrollHeight;
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      this.scrollTop = Number(top);
      this.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    },
  });
  const originalGetBoundingClientRect = dom.window.HTMLElement.prototype.getBoundingClientRect;
  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", {
    configurable: true,
    value: function getBoundingClientRect() {
      const element = this as HTMLElement;
      if (element.classList.contains("transcript")) return { top: 0 } as DOMRect;
      if (element.classList.contains("transcript-virtual-list")) {
        return { top: virtualListOrigin - geometry.scrollTop } as DOMRect;
      }
      if (element.id === questionAnchorId(measuredQuestionID)) {
        return { top: semanticContentTop - geometry.scrollTop } as DOMRect;
      }
      if (element.hasAttribute("data-question-anchor")) {
        return { top: semanticContentTop - geometry.scrollTop - 1_000 } as DOMRect;
      }
      return originalGetBoundingClientRect.call(element);
    },
  });
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  capturedAnchors.length = 0;

  const scrollAndCapture = async (top: number) => {
    await act(async () => {
      transcript.scrollTop = top;
      transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
      await flush();
    });
  };
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
  });
  await scrollAndCapture(semanticContentTop + 295.4);
  ok(capturedAnchors.at(-1)?.atBottom === false && Math.abs((capturedAnchors.at(-1) as { offset: number }).offset - 295.4) < 0.01, "upward wheel frame one captures question five at offset 295.4");
  await frames.flushOne();
  await scrollAndCapture(semanticContentTop + 155.4);
  ok(capturedAnchors.at(-1)?.atBottom === false && Math.abs((capturedAnchors.at(-1) as { offset: number }).offset - 155.4) < 0.01, "upward wheel frame two updates question five to offset 155.4");
  await frames.flushOne();
  await scrollAndCapture(semanticContentTop + 47.4);
  ok(capturedAnchors.at(-1)?.atBottom === false && Math.abs((capturedAnchors.at(-1) as { offset: number }).offset - 47.4) < 0.01, "upward wheel frame three retains the final question-five offset 47.4");
  await frames.flushOne();
  await frames.flushOne();

  geometry.scrollHeight = clampedScrollHeight;
  geometry.scrollTop = Math.min(geometry.scrollTop, maxScrollTop());
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  ok(geometry.scrollTop === 5_671, "the fourth-frame virtual-height shrink browser-clamps the physical viewport to the new max");
  geometry.scrollHeight = expandedScrollHeight;
  await scrollAndCapture(semanticContentTop + 20);
  ok(capturedAnchors.at(-1)?.atBottom === false && Math.abs((capturedAnchors.at(-1) as { offset: number }).offset - 20) < 0.01, "a later successful capture updates the same active upward batch to question five offset 20");
  expandOnRestore = true;
  await frames.flushOne();
  await frames.flushAll();
  const restoredAnchor = capturedAnchors.at(-1);
  ok(restoredAnchor?.atBottom === false && restoredAnchor.anchorMessageId === clampQuestion.id && Math.abs(restoredAnchor.offset - 20) < 0.01, "the last successful same-batch capture remains the Session cache value through wheel expiry");
  ok(Math.abs(geometry.scrollTop - (semanticContentTop + 20)) < 0.01, `wheel expiry restores the last successful same-batch capture instead of the earlier clamp anchor (${geometry.scrollTop})`);
  ok(document.querySelector(".transcript__jump-bottom") !== null, "the recovery scroll synchronizes the visible away-from-bottom UI state without a second scroll-manager API");

  const resetToBottom = async () => {
    expandOnRestore = false;
    geometry.scrollHeight = expandedScrollHeight;
    geometry.scrollTop = maxScrollTop();
    await act(async () => {
      transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
      await flush();
    });
    await frames.flushAll();
    capturedAnchors.length = 0;
  };
  const stageClampCandidate = async () => {
    await resetToBottom();
    await act(async () => {
      transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
    });
    await scrollAndCapture(semanticContentTop + 47.4);
    await frames.flushOne();
    await frames.flushOne();
    await frames.flushOne();
    await frames.flushOne();
    geometry.scrollHeight = clampedScrollHeight;
    geometry.scrollTop = Math.min(geometry.scrollTop, maxScrollTop());
    await act(async () => {
      transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
      await flush();
    });
  };

  await resetToBottom();
  await scrollAndCapture(semanticContentTop + 47.4);
  capturedAnchors.length = 0;
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
    geometry.scrollHeight = clampedScrollHeight;
    geometry.scrollTop = Math.min(geometry.scrollTop, maxScrollTop());
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  await frames.flushAll();
  ok(capturedAnchors.at(-1)?.atBottom === true && geometry.scrollTop === 5_671, "an upward wheel with no anchor from its own batch keeps the true bottom result and queues no recovery");

  await resetToBottom();
  await scrollAndCapture(semanticContentTop + 47.4);
  capturedAnchors.length = 0;
  geometry.scrollHeight = clampedScrollHeight;
  geometry.scrollTop = Math.min(geometry.scrollTop, maxScrollTop());
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  await frames.flushAll();
  ok(capturedAnchors.at(-1)?.atBottom === true && geometry.scrollTop === 5_671, "a no-wheel virtual-height change keeps the existing bottom semantics without reviving an old anchor");

  await resetToBottom();
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: 600 }));
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  await frames.flushAll();
  ok(capturedAnchors.at(-1)?.atBottom === true && geometry.scrollTop === 6_810, "a positive wheel that truly reaches bottom remains bottom");

  await stageClampCandidate();
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: 600 }));
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  await frames.flushAll();
  ok(capturedAnchors.at(-1)?.atBottom === true && geometry.scrollTop === 5_671, "a newer opposite-direction wheel cancels the clamp candidate");

  for (const input of ["pointerdown", "touchstart"] as const) {
    await stageClampCandidate();
    await act(async () => {
      transcript.dispatchEvent(new window.Event(input, { bubbles: true }));
      transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
      await flush();
    });
    window.dispatchEvent(new window.Event(input === "pointerdown" ? "pointerup" : "touchend"));
    await frames.flushAll();
    ok(capturedAnchors.at(-1)?.atBottom === true && geometry.scrollTop === 5_671, `a newer ${input} gesture cancels the clamp candidate`);
  }

  await stageClampCandidate();
  await act(async () => {
    transcript.dispatchEvent(new window.KeyboardEvent("keydown", { bubbles: true, key: "ArrowDown" }));
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  await frames.flushAll();
  ok(capturedAnchors.at(-1)?.atBottom === true && geometry.scrollTop === 5_671, "a newer recognized scroll key cancels the clamp candidate");

  await stageClampCandidate();
  capturedAnchors.length = 0;
  const remappedClampQuestion: Item = { ...clampQuestion, id: "clamp-question-5-remapped" };
  const remappedItems = items.map((item) => item.id === clampQuestion.id ? remappedClampQuestion : item);
  measuredQuestionID = remappedClampQuestion.id;
  await act(async () => {
    render(root,
      <Transcript
        items={remappedItems}
        tabId="clamp-tab"
        viewKey="clamp-session"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(_, anchor) => capturedAnchors.push(anchor)}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const remappedCacheAnchor = capturedAnchors.at(-1);
  ok(remappedCacheAnchor?.atBottom === false && remappedCacheAnchor.anchorMessageId === remappedClampQuestion.id, "same-view question revalidation remaps the Session cache before wheel expiry");
  geometry.scrollHeight = expandedScrollHeight;
  expandOnRestore = true;
  await frames.flushOne();
  await frames.flushAll();
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  const remappedRestoredAnchor = capturedAnchors.at(-1);
  ok(remappedRestoredAnchor?.atBottom === false && remappedRestoredAnchor.anchorMessageId === remappedClampQuestion.id && Math.abs(remappedRestoredAnchor.offset - 47.4) < 0.01, "wheel expiry keeps the remapped question identity and exact offset in the Session cache");
  ok(Math.abs(geometry.scrollTop - (semanticContentTop + 47.4)) < 0.01, "wheel expiry restores the remapped question identity instead of falling back to bottom");

  measuredQuestionID = clampQuestion.id;
  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="clamp-tab"
        viewKey="clamp-session"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(key, anchor) => {
          capturedAnchors.push(anchor);
          capturedByView.push({ key, anchor });
        }}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();

  await stageClampCandidate();
  const itemsWithoutClampQuestion = items.filter((item) => item.id !== clampQuestion.id);
  await act(async () => {
    render(root,
      <Transcript
        items={itemsWithoutClampQuestion}
        tabId="clamp-tab"
        viewKey="clamp-session"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(key, anchor) => capturedByView.push({ key, anchor })}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();
  ok(geometry.scrollTop === 5_671 && frames.pendingCount() === 0, "a clamp anchor deleted before expiry uses the existing bounded bottom fallback without a recovery loop");

  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="clamp-tab"
        viewKey="clamp-session"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(key, anchor) => capturedByView.push({ key, anchor })}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();
  await stageClampCandidate();
  const clampReveal = markNoticeForReveal({
    kind: "notice", id: "clamp-reveal", level: "warn", text: "本轮已中断",
  });
  await act(async () => {
    render(root,
      <Transcript
        items={[...items, clampReveal]}
        tabId="clamp-tab"
        viewKey="clamp-session"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(key, anchor) => capturedByView.push({ key, anchor })}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();
  ok(geometry.scrollTop === 5_671 && frames.pendingCount() === 0, "an explicit reveal cancels the clamp candidate and remains at bottom");

  await stageClampCandidate();
  capturedByView.length = 0;
  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="clamp-tab-b"
        viewKey="clamp-session-b"
        onPrompt={() => {}}
        activeExpert={{ id: "clamp-team", name: "Clamp Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(key, anchor) => capturedByView.push({ key, anchor })}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();
  ok(!capturedByView.some(({ key, anchor }) => (
    key === "clamp-session-b"
    && anchor.atBottom === false
    && anchor.anchorMessageId === clampQuestion.id
    && Math.abs(anchor.offset - 47.4) < 0.01
  )), "a committed semantic-view change cannot consume the previous view's clamp candidate");

  await stageClampCandidate();
  await act(async () => root.unmount());
  await frames.flushAll();
  ok(geometry.scrollTop === 5_671 && frames.pendingCount() === 0, "unmount synchronously discards a clamp candidate without scheduling recovery");
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => null } },
  };
  const ordinaryQuestion: Item = { kind: "user", id: "ordinary-clamp-question", text: "Inspect the ordinary result" };
  const ordinaryItems: Item[] = [
    { kind: "user", id: "ordinary-clamp-prior", text: "Prepare the ordinary investigation" },
    ...assistantItems("ordinary-clamp-intro", 3),
    ordinaryQuestion,
    ...assistantItems("ordinary-clamp-result", 32),
  ];
  const captured: SessionScrollAnchor[] = [];
  await act(async () => {
    render(root,
      <Transcript
        items={ordinaryItems}
        tabId="ordinary-clamp-tab"
        viewKey="ordinary-clamp-session"
        onPrompt={() => {}}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: ordinaryQuestion.id, offset: 0 }}
        onScrollAnchorChange={(_, anchor) => captured.push(anchor)}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await frames.flushAll();
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing ordinary clamp transcript");
  ok(transcript.querySelector(".transcript-virtual-list") !== null, "the ordinary clamp negative path is independently virtualized");
  const geometry = { scrollHeight: 7_284, scrollTop: 6_810 };
  const clientHeight = 474;
  const semanticContentTop = 6_415;
  const maxScrollTop = () => Math.max(0, geometry.scrollHeight - clientHeight);
  Object.defineProperty(transcript, "clientHeight", { configurable: true, get: () => clientHeight });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, get: () => geometry.scrollHeight });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => geometry.scrollTop,
    set: (value) => { geometry.scrollTop = Math.max(0, Math.min(Number(value), maxScrollTop())); },
  });
  const originalGetBoundingClientRect = dom.window.HTMLElement.prototype.getBoundingClientRect;
  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", {
    configurable: true,
    value: function getBoundingClientRect() {
      const element = this as HTMLElement;
      if (element.classList.contains("transcript")) return { top: 0 } as DOMRect;
      if (element.id === questionAnchorId(ordinaryQuestion.id)) {
        return { top: semanticContentTop - geometry.scrollTop } as DOMRect;
      }
      if (element.hasAttribute("data-question-anchor")) {
        return { top: semanticContentTop - geometry.scrollTop - 1_000 } as DOMRect;
      }
      return originalGetBoundingClientRect.call(element);
    },
  });
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  captured.length = 0;
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
    transcript.scrollTop = semanticContentTop + 47.4;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  ok(captured.length === 0, "the ordinary virtualized DOM-empty scroll keeps its existing non-expert capture behavior");
  await frames.flushOne();
  await frames.flushOne();
  await frames.flushOne();
  await frames.flushOne();
  geometry.scrollHeight = 6_145;
  geometry.scrollTop = Math.min(geometry.scrollTop, maxScrollTop());
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  await frames.flushAll();
  ok(captured.at(-1)?.atBottom === true && geometry.scrollTop === 5_671, "an ordinary virtualized transcript keeps its existing clamp-to-bottom behavior and never queues expert recovery");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let requestCount = 0;
  const boundTeam: ExpertTeamSession = {
    state: "team_active",
    id: "rewind-team",
    name: "Rewind Team",
    lead: { memberKey: "lead", name: "Rewind Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Rewind Old Reviewer", role: "review", subagentRef: "sa-old", status: "completed" }],
  };
  const resetTeam: ExpertTeamSession = { ...boundTeam, members: [] };
  let resolveOptimisticTeam!: (team: ExpertTeamSession | null) => void;
  let resolveCommittedTeam!: (team: ExpertTeamSession | null) => void;
  const optimisticTeam = new Promise<ExpertTeamSession | null>((resolve) => { resolveOptimisticTeam = resolve; });
  const committedTeam = new Promise<ExpertTeamSession | null>((resolve) => { resolveCommittedTeam = resolve; });
  const teamResponses = [Promise.resolve(boundTeam), optimisticTeam, committedTeam];
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        ExpertTeamForTab: async () => {
          requestCount += 1;
          const response = teamResponses.shift();
          if (!response) throw new Error("unexpected expert-team reload");
          return response;
        },
      },
    },
  };
  const activeExpert = { id: "rewind-team", name: "Rewind Team", mode: "team" };
  const retainedItems = assistantItems("rewind-retained-parent", 2);
  const stableStatuses = { "sa-old": "completed" as const };

  await act(async () => {
    render(root, <Transcript items={retainedItems} tabId="rewind-tab" viewKey="rewind-session" onPrompt={() => {}} activeExpert={activeExpert} subagentStatusByRef={stableStatuses} rewindSignal={0} />);
    await flush();
    await flush();
  });
  ok(requestCount === 1, "initial rewind team sidecar is loaded once");
  ok(document.body.textContent?.includes("Rewind Old Reviewer") === true, "the old bound member is initially rendered");

  await act(async () => {
    render(root, <Transcript items={retainedItems} tabId="rewind-tab" viewKey="rewind-session" onPrompt={() => {}} activeExpert={activeExpert} subagentStatusByRef={stableStatuses} rewindSignal={0} />);
    await flush();
  });
  ok(requestCount === 1, "ordinary render and code-only rewind without a signal change do not reload the team");

  await act(async () => {
    render(root, <Transcript items={retainedItems} tabId="rewind-tab" viewKey="rewind-session" onPrompt={() => {}} activeExpert={activeExpert} subagentStatusByRef={stableStatuses} rewindSignal={1} />);
    await flush();
  });
  ok(requestCount === 2, "optimistic conversation rewind signal reloads the still-bound sidecar once");
  ok(document.body.textContent?.includes("Rewind Old Reviewer") === true, "the pending optimistic reload keeps the cached member while the backend sidecar is still bound");

  await act(async () => {
    render(root, <Transcript items={retainedItems} tabId="rewind-tab" viewKey="rewind-session" onPrompt={() => {}} activeExpert={activeExpert} subagentStatusByRef={stableStatuses} rewindSignal={2} />);
    await flush();
  });
  ok(requestCount === 3, "backend commit signal starts a third authoritative team reload");
  await act(async () => {
    resolveCommittedTeam(resetTeam);
    await flush();
  });
  ok(document.body.textContent?.includes("Rewind Old Reviewer") === false, "the committed reset response removes the cached old member");
  await act(async () => {
    resolveOptimisticTeam(boundTeam);
    await flush();
  });
  ok(document.body.textContent?.includes("Rewind Old Reviewer") === false, "a late optimistic bound response cannot overwrite the committed reset team");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "active-guard-team",
    name: "Active Guard Team",
    lead: { memberKey: "lead", name: "Active Guard Lead", role: "lead" },
    members: [],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => team } },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" as const };
  const shortItems = assistantItems("active-guard-parent", 9);

  await act(async () => {
    render(root, <Transcript items={shortItems} tabId="active-guard-tab" viewKey="active-guard-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing active-guard transcript scroller");
  let scrollTop = 100;
  let scrollHeight = 900;
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 800 });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, get: () => scrollHeight });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => { scrollTop = Math.max(0, Math.min(Number(value), scrollHeight - 800)); },
  });
  await act(async () => scrollViewport(transcript, 100));

  const thresholdItems = assistantItems("active-guard-parent", 10);
  scrollHeight = 1_800;
  await act(async () => {
    render(root, <Transcript items={thresholdItems} tabId="active-guard-tab" viewKey="active-guard-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true }));
  await act(async () => scrollViewport(transcript, 0));
  ok(scrollTop === 0, "active-guard user scroll reaches the requested top position");
  await act(async () => scrollViewport(transcript, 0));
  ok(scrollTop === 0, "the next programmatic reset cannot revive a user-cancelled guard");

  const finalItems = [...thresholdItems, {
    kind: "assistant" as const,
    id: "active-guard-later-message",
    text: "later message after active-guard user scroll",
    reasoning: "",
    streaming: false,
  }];
  scrollHeight = 1_980;
  await act(async () => {
    render(root, <Transcript items={finalItems} tabId="active-guard-tab" viewKey="active-guard-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await settleLayout();
  });
  ok(scrollTop === 0, `user intent immediately cancels an active virtualization reset guard before a second reset (scrollTop=${scrollTop})`);

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "settled-result-team",
    name: "Settled Result Team",
    lead: { memberKey: "lead", name: "Settled Result Lead", role: "lead" },
    members: [],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => team } },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" as const };
  const historyItems: Item[] = [
    { kind: "user", id: "settled-result-question", text: "check six task results" },
    ...assistantItems("settled-result-history", 11),
  ];

  await act(async () => {
    render(root, <Transcript items={historyItems} tabId="settled-result-tab" viewKey="settled-result-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing settled-result transcript scroller");
  let scrollHeight = 2_160;
  let scrollTop = 1_360;
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 800 });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, get: () => scrollHeight });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => { scrollTop = Math.max(0, Math.min(Number(value), scrollHeight - 800)); },
  });
  await act(async () => scrollViewport(transcript, scrollTop));

  const streamedItems: Item[] = [...historyItems, {
    kind: "assistant",
    id: "settled-result-stream",
    text: "",
    reasoning: "",
    streaming: true,
  }];
  let liveText = "";
  for (let index = 0; index < 6; index += 1) {
    liveText = Array.from({ length: index + 1 }, (_, result) => `task result ${result + 1}`).join("\n");
    scrollHeight = 2_340 + index * 120;
    await act(async () => {
      render(root, <Transcript items={streamedItems} live={{ id: "settled-result-stream", text: liveText, reasoning: "", reasoningComplete: true }} tabId="settled-result-tab" viewKey="settled-result-session" onPrompt={() => {}} activeExpert={activeExpert} running />);
      if (index < 5) await settleLayout();
      else await new Promise((resolve) => setTimeout(resolve, 55));
    });
  }
  const delayedMeasurementBottom = scrollTop;
  scrollHeight += 360;
  await act(async () => {
    scrollTop = delayedMeasurementBottom;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await settleLayout();
  });
  ok(scrollTop === scrollHeight - 800, "an already-virtualized overview stays pinned through six streamed task results");
  const streamingRowCount = Number(document.querySelector('[data-testid="expert-team-overview-virtual-list"]')?.getAttribute("data-row-count"));

  const oldBottom = scrollTop;
  const acceptedFinalText = "accepted final after six streamed task result lines settle";
  const finalItems = streamedItems.map((item) => item.id === "settled-result-stream" && item.kind === "assistant"
    ? { ...item, text: `${liveText}\n\n${acceptedFinalText}`, streaming: false }
    : item);
  scrollHeight += 960;
  await act(async () => {
    flushSync(() => render(root, <Transcript items={finalItems} tabId="settled-result-tab" viewKey="settled-result-session" onPrompt={() => {}} activeExpert={activeExpert} />));
    scrollTop = oldBottom;
    await Promise.resolve();
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await settleLayout();
  });
  const settledRowCount = Number(document.querySelector('[data-testid="expert-team-overview-virtual-list"]')?.getAttribute("data-row-count"));
  ok(finalItems.length === streamedItems.length && settledRowCount === streamingRowCount + 1, `settling the same assistant keeps item count stable and adds exactly one terminal action row (${streamingRowCount}->${settledRowCount})`);
  ok(scrollTop === scrollHeight - 800, `an already-virtualized result/final measurement keeps the existing bottom intent (scrollTop=${scrollTop})`);
  ok(document.body.textContent?.includes(acceptedFinalText) === true, "the settled accepted final remains mounted after its row adds turn actions");

  transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true }));
  await act(async () => scrollViewport(transcript, 640));
  const middleItems = [...finalItems, {
    kind: "assistant" as const,
    id: "settled-result-after-user",
    text: "",
    reasoning: "",
    streaming: true,
  }];
  scrollHeight += 180;
  await act(async () => {
    flushSync(() => render(root, <Transcript items={middleItems} live={{ id: "settled-result-after-user", text: "later task result", reasoning: "", reasoningComplete: true }} tabId="settled-result-tab" viewKey="settled-result-session" onPrompt={() => {}} activeExpert={activeExpert} running />));
    await Promise.resolve();
    render(root, <Transcript items={middleItems} live={{ id: "settled-result-after-user", text: "later task result grew", reasoning: "", reasoningComplete: true }} tabId="settled-result-tab" viewKey="settled-result-session" onPrompt={() => {}} activeExpert={activeExpert} running />);
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await settleLayout();
  });
  ok(scrollTop === 640, "already-virtualized live-only growth does not steal a real user middle position");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "threshold-team",
    name: "Threshold Team",
    lead: { memberKey: "lead", name: "Threshold Lead", role: "lead" },
    members: [],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => team } },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" as const };
  const shortItems = assistantItems("threshold-parent", 9);

  await act(async () => {
    render(root, <Transcript items={shortItems} tabId="threshold-tab" viewKey="threshold-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing threshold transcript scroller");
  let scrollTop = 100;
  let scrollHeight = 900;
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 800 });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, get: () => scrollHeight });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => { scrollTop = Math.max(0, Math.min(Number(value), scrollHeight - 800)); },
  });
  await act(async () => scrollViewport(transcript, 100));

  const thresholdItems = assistantItems("threshold-parent", 10);
  scrollHeight = 1_800;
  await act(async () => {
    render(root, <Transcript items={thresholdItems} tabId="threshold-tab" viewKey="threshold-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  ok(document.querySelector('[data-testid="expert-team-overview-virtual-list"]') !== null, "persisted overview crosses into virtualization at the inclusive threshold");

  // TanStack Virtual may reset the DOM scroll position while replacing its
  // estimated rows with measured rows, then reset again while remounting the
  // measured window. Neither scroll has a user input signal.
  await act(async () => scrollViewport(transcript, 0));
  await act(async () => scrollViewport(transcript, 0));
  const finalItems = [...thresholdItems, {
    kind: "assistant" as const,
    id: "threshold-accepted-final",
    text: "accepted expert final after virtualization",
    reasoning: "",
    streaming: false,
  }];
  scrollHeight = 1_980;
  await act(async () => {
    render(root, <Transcript items={finalItems} tabId="threshold-tab" viewKey="threshold-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await settleLayout();
  });
  await act(async () => scrollViewport(transcript, scrollTop));
  ok(scrollTop === 1_180, "a programmatic virtualizer reset preserves pinned-bottom following for the accepted final");
  ok(document.body.textContent?.includes("accepted expert final after virtualization") === true, "the accepted final remains mounted after the virtualization transition");

  transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true }));
  await act(async () => scrollViewport(transcript, 0));
  const afterUserScroll = [...finalItems, {
    kind: "assistant" as const,
    id: "threshold-later-message",
    text: "later message after real user scroll",
    reasoning: "",
    streaming: false,
  }];
  scrollHeight = 2_160;
  await act(async () => {
    render(root, <Transcript items={afterUserScroll} tabId="threshold-tab" viewKey="threshold-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await settleLayout();
  });
  ok(scrollTop === 0, "a real user scroll to the top still disables bottom following");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const pending = new Map<string, (team: ExpertTeamSession | null) => void>();
  const requestCount = new Map<string, number>();
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        ExpertTeamForTab: (tabId: string) => {
          requestCount.set(tabId, (requestCount.get(tabId) ?? 0) + 1);
          return new Promise<ExpertTeamSession | null>((resolve) => pending.set(tabId, resolve));
        },
      },
    },
  };
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "cached-team",
    name: "Cached Team",
    lead: { memberKey: "lead", name: "Cached Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Cached Reviewer", role: "review", subagentRef: "sa-cache", status: "completed" }],
  };
  const lateTeam: ExpertTeamSession = {
    ...team,
    id: "late-team",
    name: "Late Team",
    lead: { memberKey: "lead", name: "Late Lead", role: "lead" },
  };
  const otherTeam: ExpertTeamSession = {
    ...team,
    id: "other-team",
    name: "Other Team",
    lead: { memberKey: "lead", name: "Other Lead", role: "lead" },
  };
  const activeExpert = { id: "cached-team", name: "Cached Team", mode: "team" };
  const otherActiveExpert = { id: otherTeam.id, name: otherTeam.name, mode: "team" };
  const parentItems: Item[] = [task("cached-task", "Cached Reviewer"), ...assistantItems("cached-parent", 12)];

  await act(async () => {
    render(root, <Transcript items={parentItems} tabId="team-tab-a" viewKey="team-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  await act(async () => {
    pending.get("team-tab-a")?.(team);
    await flush();
  });
  ok(document.querySelector('[data-team-state="team_active"]') !== null, "resolved team sidecar selects the persisted workspace");

  await act(async () => {
    render(root, <Transcript items={parentItems} viewKey="team-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  ok(document.querySelector('[data-team-state]') === null, "a cached team waits until the target tab id is ready");
  ok(document.querySelector(".agent-workspace") === null, "the cached-team wait does not mount the legacy workspace");
  ok(document.querySelector('.transcript__session-loading') === null && document.querySelector('.msg--assistant') !== null, "the cached-team wait preserves readable parent history");

  await act(async () => {
    render(root, <Transcript items={parentItems} tabId="team-tab-b" viewKey="team-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  ok(document.querySelector('[data-team-state="team_active"]') !== null, "the same semantic session keeps its persisted sidecar when the transient tab id changes");
  ok(document.body.textContent?.includes("Cached Lead") === true, "the reused sidecar stays visible without a legacy workspace gap");
  ok(requestCount.get("team-tab-b") === 1, "the reused sidecar still refreshes through the current tab id");

  await act(async () => {
    render(root, <Transcript items={parentItems} tabId="team-tab-c" viewKey="different-team-session" onPrompt={() => {}} activeExpert={otherActiveExpert} />);
    await flush();
  });
  ok(document.querySelector("[data-team-state]") === null, "a different semantic session does not reuse another session's sidecar");
  ok(
    document.querySelector(".agent-workspace") === null
      && document.querySelector('.msg--assistant') !== null,
    "a cache miss keeps its own parent content until its sidecar resolves",
  );

  await act(async () => {
    pending.get("team-tab-b")?.(lateTeam);
    await flush();
  });
  ok(document.querySelector("[data-team-state]") === null, "a late result from the previous tab does not replace the active semantic session");
  ok(document.body.textContent?.includes("Late Lead") === false, "a late previous-tab sidecar does not leak into the active session");

  await act(async () => {
    pending.get("team-tab-c")?.(otherTeam);
    await flush();
  });
  ok(document.querySelector('[data-team-state="team_active"]') !== null, "the active semantic session accepts its own sidecar result");
  ok(document.body.textContent?.includes("Other Lead") === true, "the active session renders its own resolved team identity");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "input-only-team",
    name: "Input Only Team",
    lead: { memberKey: "lead", name: "Input Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Input Reviewer", role: "review", subagentRef: "sa-input-only", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  const assignedScrollTops: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      assignedScrollTops.push(Number(top));
      this.scrollTop = Number(top);
      this.dispatchEvent(new window.Event("scroll"));
    },
  });
  const items: Item[] = [task("input-only-task", "Input Reviewer"), ...questionItems("input-only", 12)];
  const cachedAnchor: SessionScrollAnchor = {
    atBottom: false,
    anchorMessageId: "input-only-question-1",
    offset: 24,
  };
  const neverResolve = new Promise<void>(() => {});
  const SuspendAfterTranscript = ({ suspend }: { suspend: boolean }) => {
    if (suspend) throw neverResolve;
    return null;
  };
  const transcriptNode = (initialScrollAnchor: SessionScrollAnchor, suspend = false) => (
    <React.Suspense fallback={null}>
      <Transcript
        items={items}
        tabId="input-only-tab"
        viewKey="input-only-session"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={initialScrollAnchor}
        questionNavigator={false}
      />
      <SuspendAfterTranscript suspend={suspend} />
    </React.Suspense>
  );

  await act(async () => {
    render(root, transcriptNode(cachedAnchor));
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing input-only transcript");
  let scrollTop = 0;
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = Number(value);
        assignedScrollTops.push(scrollTop);
      },
    },
  });
  const originalRect = dom.window.HTMLElement.prototype.getBoundingClientRect;
  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", {
    configurable: true,
    value: function getBoundingClientRect(this: HTMLElement) {
      if (this === transcript) return { top: 0 } as DOMRect;
      if (this.id === questionAnchorId("input-only-question-1")) return { top: 1_000 - scrollTop } as DOMRect;
      return originalRect.call(this);
    },
  });
  ok(document.querySelector('.transcript__session-loading') !== null, "input-only cancellation starts while the semantic return is gated");
  await act(async () => {
    React.startTransition(() => render(root, transcriptNode({ atBottom: true }, true)));
    await flush();
  });
  ok(document.querySelector('.transcript__session-loading') !== null, "an aborted non-deferred render leaves the committed semantic return gated");
  assignedScrollTops.length = 0;
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -300 }));
    await flush();
  });
  await act(async () => {
    resolveTeam?.(team);
    await flush();
  });
  await frames.flushAll();
  ok(assignedScrollTops.every((top) => top === 0), `input-only cancellation prevents persisted seek or replay of the old semantic anchor (${assignedScrollTops.join(",")})`);

  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", { configurable: true, value: originalRect });
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "pending-writeback-team",
    name: "Pending Writeback Team",
    lead: { memberKey: "lead", name: "Pending Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Pending Reviewer", role: "review", subagentRef: "sa-pending-writeback", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      this.scrollTop = typeof options === "number" ? y ?? 0 : options.top ?? 0;
    },
  });
  const items: Item[] = [task("pending-writeback-task", "Pending Reviewer"), ...questionItems("pending-writeback", 12)];
  const cachedAnchor: SessionScrollAnchor = {
    atBottom: false,
    anchorMessageId: "pending-writeback-question-1",
    offset: 24,
  };
  const cacheWrites: SessionScrollAnchor[] = [];

  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="pending-writeback-tab"
        viewKey="pending-writeback-session"
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        initialScrollAnchor={cachedAnchor}
        onScrollAnchorChange={(_, anchor) => cacheWrites.push(anchor)}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing pending-writeback transcript");
  let scrollTop = 0;
  let mode: "question-zero" | "bottom" | "wrong-offset" | "normal" = "question-zero";
  let dispatchProgrammaticScrollEvents = true;
  const assignedScrollTops: number[] = [];
  const assignmentHiddenStates: boolean[] = [];
  Object.defineProperties(transcript, {
    clientHeight: { configurable: true, value: 800 },
    scrollHeight: { configurable: true, value: 20_000 },
    scrollTop: {
      configurable: true,
      get: () => scrollTop,
      set: (value: number) => {
        scrollTop = mode === "bottom" ? 19_250 : mode === "wrong-offset" ? 1_000 : Number(value);
        assignedScrollTops.push(scrollTop);
        assignmentHiddenStates.push(document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === "true");
        if (dispatchProgrammaticScrollEvents) transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
      },
    },
  });
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const contentTopByID = new Map([
    [questionAnchorId("pending-writeback-question-0"), 100],
    [questionAnchorId("pending-writeback-question-1"), 1_000],
  ]);
  const originalRect = dom.window.HTMLElement.prototype.getBoundingClientRect;
  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", {
    configurable: true,
    value: function getBoundingClientRect(this: HTMLElement) {
      if (this === transcript) return { top: 0 } as DOMRect;
      if (mode === "question-zero") {
        if (this.id === questionAnchorId("pending-writeback-question-0")) return { top: -40 } as DOMRect;
        if (contentTopByID.has(this.id)) return { top: 200 } as DOMRect;
      }
      if (mode === "wrong-offset" && contentTopByID.has(this.id)) {
        if (this.id === questionAnchorId("pending-writeback-question-0")) return { top: -400 } as DOMRect;
        if (this.id === questionAnchorId("pending-writeback-question-1")) return { top: -100 } as DOMRect;
      }
      const contentTop = contentTopByID.get(this.id);
      if (contentTop !== undefined) return { top: contentTop - scrollTop } as DOMRect;
      return originalRect.call(this);
    },
  });

  mode = "question-zero";
  await frames.flushOne();
  mode = "bottom";
  await frames.flushOne();
  mode = "wrong-offset";
  await frames.flushOne();
  ok(cacheWrites.length === 0, `active same-view restore ignores programmatic question, bottom, and wrong-offset cache writes (${JSON.stringify(cacheWrites)})`);
  const deferredLegacyWorkspace = document.querySelector(".agent-workspace");
  ok(deferredLegacyWorkspace?.closest('[hidden][aria-hidden="true"]') !== null, "a semantic return does not expose the unresolved legacy fallback");
  ok(document.querySelector('.transcript__session-loading') !== null, "a semantic return stays on the lightweight loading presentation until its selected session resolves");
  ok(assignedScrollTops.length === 0, `the unresolved legacy fallback does not visibly seek before the selected session presentation is ready (${assignedScrollTops.join(",")})`);

  mode = "normal";
  dispatchProgrammaticScrollEvents = false;
  assignedScrollTops.length = 0;
  await act(async () => {
    resolveTeam?.(team);
    await flush();
  });
  ok(
    document.querySelector('.transcript__session-loading--overlay') !== null && frames.pendingCount() > 0,
    "virtual handle replacement enters declarative hidden restore without waiting for a scroll event",
  );
  dispatchProgrammaticScrollEvents = true;
  await frames.flushAll();
  ok(assignedScrollTops.includes(1_024), `persisted replacement restores cached question A with exact offset X (${assignedScrollTops.join(",")})`);
  ok(!assignedScrollTops.includes(1_100), "persisted replacement does not restore the programmatic wrong offset");
  ok(cacheWrites.length === 0, "persisted active restore also avoids Session anchor callbacks");

  assignedScrollTops.length = 0;
  assignmentHiddenStates.length = 0;
  await act(async () => {
    invokeReactButtonClick(Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Pending Reviewer")));
    await flush();
  });
  const replacementPresentation = document.querySelector('[data-testid="persisted-expert-presentation"]');
  ok(
    !assignmentHiddenStates.includes(true) && replacementPresentation?.getAttribute("aria-hidden") === null,
    "member selection remains readable without reviving a finished parent restore",
  );
  await act(async () => {
    mode = "wrong-offset";
    transcript.scrollTop = 700;
    await flush();
  });
  ok(cacheWrites.length === 0, "a post-convergence child replacement cannot write a wrong semantic offset");
  await act(async () => {
    mode = "bottom";
    transcript.scrollTop = 19_250;
    await flush();
  });
  ok(cacheWrites.length === 0, "a post-convergence child replacement cannot replace the canonical anchor with bottom");

  mode = "normal";
  assignedScrollTops.length = 0;
  await act(async () => {
    invokeReactButtonClick(Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Pending Lead")));
    await flush();
  });
  await frames.flushAll();
  ok(assignedScrollTops.includes(1_024), "returning from the child replacement restores the same canonical identity and offset");
  ok(cacheWrites.length === 0, "replacement convergence keeps Session cache writes frozen until stable");
  ok(document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === null, "replacement presentation reveals again only after its second stable convergence");
  dispatchProgrammaticScrollEvents = true;

  const writesAfterRestore = cacheWrites.length;
  await act(async () => {
    scrollTop = 600;
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });
  ok(cacheWrites.length === writesAfterRestore + 1, `programmatic semantic capture resumes after the bounded restore completes (writes=${cacheWrites.length}, before=${writesAfterRestore}, hidden=${document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden")}, frames=${frames.pendingCount()})`);

  assignedScrollTops.length = 0;
  const keyboardSelectedMember = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Pending Reviewer"));
  await act(async () => {
    keyboardSelectedMember?.click();
    await flush();
  });
  ok(
    document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === null
      && !assignedScrollTops.includes(1_024),
    "keyboard-equivalent member activation stays visible without replaying the old anchor",
  );
  await frames.flushAll();
  await act(async () => {
    invokeReactButtonClick(Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Pending Lead")));
    await flush();
  });
  await frames.flushAll();

  assignedScrollTops.length = 0;
  const userSelectedMember = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Pending Reviewer"));
  await act(async () => {
    userSelectedMember?.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    userSelectedMember?.click();
    await flush();
  });
  ok(document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === null, "real user member selection reveals an imperatively hidden replacement immediately");
  await frames.flushAll();
  ok(!assignedScrollTops.includes(1_024), "real user member selection is not reclaimed by the old programmatic semantic anchor");
  await act(async () => {
    invokeReactButtonClick(Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Pending Lead")));
    await flush();
  });
  await frames.flushAll();
  await act(async () => {
    scrollTop = 640;
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });

  const activeMember = Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
    .find((button) => button.textContent?.includes("Pending Lead"));
  await act(async () => {
    activeMember?.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    activeMember?.click();
    await flush();
  });
  assignmentHiddenStates.length = 0;
  assignedScrollTops.length = 0;
  await act(async () => {
    invokeReactButtonClick(Array.from(document.querySelectorAll<HTMLButtonElement>(".agent-workspace__tab"))
      .find((button) => button.textContent?.includes("Pending Reviewer")));
    await flush();
  });
  ok(
    !assignmentHiddenStates.includes(true) && document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === null,
    "clicking the active tab cannot make subsequent member selection hide readable content",
  );

  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", { configurable: true, value: originalRect });
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "native-return-team",
    name: "Native Return Team",
    lead: { memberKey: "lead", name: "Native Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Native Reviewer", role: "review", subagentRef: "sa-native-return", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  const items: Item[] = [task("native-return-task", "Native Reviewer"), ...questionItems("native-return", 48)];
  const cachedAnchor: SessionScrollAnchor = { atBottom: false, anchorMessageId: "native-return-question-10", offset: -12.2 };
  const assignedScrollTops: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = Number(typeof options === "number" ? y ?? 0 : options.top ?? 0);
      assignedScrollTops.push(top);
      this.scrollTop = top;
      this.dispatchEvent(new window.Event("scroll"));
    },
  });
  const transcriptNode = (tabId?: string) => (
    <React.StrictMode><Transcript
      items={items}
      tabId={tabId}
      viewKey="native-return-session"
      onPrompt={() => {}}
      activeExpert={{ id: team.id, name: team.name, mode: "team" }}
      initialScrollAnchor={cachedAnchor}
      questionNavigator={false}
    /></React.StrictMode>
  );

  await act(async () => {
    render(root, transcriptNode());
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing native-return transcript");
  let scrollTop = 0;
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 800 });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, value: 20_000 });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => {
      scrollTop = Number(value);
      assignedScrollTops.push(scrollTop);
    },
  });
  assignedScrollTops.length = 0;
  ok(document.querySelector('.transcript__session-loading') !== null, "a semantic team return mounts loading protection while App withholds tabId during its surface transition");
  ok(document.querySelector('[data-testid="agent-workspace-overview-virtual-list"]') === null, "the tabId-less return does not expose the legacy presentation");
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  ok(assignedScrollTops.every((top) => top === 0), `the tabId-less return performs no visible legacy viewport seek (${assignedScrollTops.join(",")})`);

  await act(async () => {
    render(root, transcriptNode("native-return-tab"));
    await flush();
  });
  assignedScrollTops.length = 0;
  await act(async () => {
    resolveTeam?.(team);
    await flush();
  });
  ok(document.querySelector('.transcript__session-loading[role="status"]') !== null, "the final persisted presentation stays loading until its initial semantic restore is visually stable");
  const hiddenPersistedPresentation = document.querySelector('[data-testid="persisted-expert-presentation"]');
  const persistedOverlay = document.querySelector('.transcript__session-loading--overlay');
  const hiddenPersistedList = document.querySelector<HTMLElement>('[data-testid="expert-team-overview-virtual-list"]');
  const hiddenListOrigin = hiddenPersistedList
    ? hiddenPersistedList.getBoundingClientRect().top + scrollTop
    : Number.NaN;
  ok(
    hiddenPersistedPresentation?.getAttribute("aria-hidden") === "true"
      && (hiddenPersistedPresentation as HTMLElement).style.visibility === "hidden"
      && hiddenPersistedList !== null
      && persistedOverlay !== null,
    "the final persisted workspace participates in layout but is hidden during its initial seek and measurement corrections",
  );
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 1_000));
  });
  ok(document.querySelector('[data-testid="expert-team-overview-virtual-list"]') !== null, "the final persisted workspace becomes visible after bounded semantic restore convergence");
  const revealedPersistedList = document.querySelector<HTMLElement>('[data-testid="expert-team-overview-virtual-list"]');
  const revealedListOrigin = revealedPersistedList
    ? revealedPersistedList.getBoundingClientRect().top + scrollTop
    : Number.NaN;
  ok(Math.abs(revealedListOrigin - hiddenListOrigin) <= 1, "overlay removal does not change persisted virtual-list geometry");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const frames = installManualAnimationFrames(dom);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolvePendingUserTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolvePendingUserTeam = resolve; }) } },
  };
  const team = { id: "pending-user-team", name: "Pending User Team", mode: "team" as const };
  const items: Item[] = [task("pending-user-task", "Pending User"), ...questionItems("pending-user", 2)];
  const cachedAnchor: SessionScrollAnchor = { atBottom: false, anchorMessageId: "pending-user-question-1", offset: 24 };
  const cacheWrites: SessionScrollAnchor[] = [];

  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="pending-user-tab"
        viewKey="pending-user-session"
        onPrompt={() => {}}
        activeExpert={team}
        initialScrollAnchor={cachedAnchor}
        onScrollAnchorChange={(_, anchor) => cacheWrites.push(anchor)}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  await act(async () => {
    resolvePendingUserTeam?.(persistedTeam(team.id, team.name));
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing pending-user transcript");
  instrumentTranscriptScroll(transcript, 600);
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  for (const node of transcript.querySelectorAll<HTMLElement>("[data-question-anchor]")) {
    node.getBoundingClientRect = () => ({
      top: node.id === questionAnchorId("pending-user-question-0") ? -40 : 200,
    }) as DOMRect;
  }
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -300 }));
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });
  const userAnchor = cacheWrites.at(-1);
  ok(userAnchor?.atBottom === false && userAnchor.anchorMessageId === "pending-user-question-0" && userAnchor.offset === 40, "real wheel input replaces active restore A/X with the user's exact anchor");
  ok(document.querySelector('[data-testid="persisted-expert-presentation"]')?.getAttribute("aria-hidden") === null, "user input immediately reveals a hidden persisted presentation");
  await frames.flushAll();
  ok(cacheWrites.at(-1) === userAnchor, "user input cancels the old restore RAF before it can overwrite the new anchor");

  await act(async () => root.unmount());
  dom.window.close();
}

for (const resolution of ["persisted", "null"] as const) {
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  let latestUserAnchor = "";
  const team: ExpertTeamSession = {
    state: "team_active",
    id: `user-${resolution}-team`,
    name: `User ${resolution} Team`,
    lead: { memberKey: "lead", name: "User Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "User Reviewer", role: "review", subagentRef: `sa-user-${resolution}`, status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" };
  const parentItems: Item[] = [task(`user-${resolution}-task`, "User Reviewer"), ...questionItems(`user-${resolution}`, 48)];
  const initialAnchorMessageId = `user-${resolution}-question-10`;
  const userRestoreSeeks: number[] = [];
  let dispatchProgrammaticScroll = true;
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      userRestoreSeeks.push(Number(top));
      this.scrollTop = Number(top);
      if (dispatchProgrammaticScroll) this.dispatchEvent(new window.Event("scroll"));
    },
  });

  await act(async () => {
    render(root,
      <React.StrictMode><Transcript
        items={parentItems}
        tabId={`user-${resolution}-tab`}
        viewKey={`user-${resolution}-session`}
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: false, anchorMessageId: initialAnchorMessageId, offset: 24 }}
        onScrollAnchorChange={(_, anchor) => {
          if (anchor.atBottom === false) latestUserAnchor = anchor.anchorMessageId;
        }}
        questionNavigator={false}
      /></React.StrictMode>,
    );
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  await act(async () => {
    resolveTeam?.(team);
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing transcript scroller");
  const currentScrollTop = instrumentTranscriptScroll(transcript, 0);
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const mountedAnchors = Array.from(transcript.querySelectorAll<HTMLElement>("[data-question-anchor]"));
  if (mountedAnchors.length < 2) throw new Error("missing mounted user anchors");
  for (const [index, anchor] of mountedAnchors.entries()) {
    anchor.getBoundingClientRect = () => ({ top: index === 0 ? 40 : 200 }) as DOMRect;
  }
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true }));
    transcript.scrollTop = 120;
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });
  const firstGestureAnchor = latestUserAnchor;
  for (const [index, anchor] of mountedAnchors.entries()) {
    anchor.getBoundingClientRect = () => ({ top: index === 0 ? -100 : index === 1 ? 10 : 200 }) as DOMRect;
  }
  await act(async () => {
    transcript.scrollTop = 160;
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });
  ok(latestUserAnchor !== "" && latestUserAnchor !== initialAnchorMessageId, `real user scrolling records a newer semantic anchor before ${resolution} settlement`);
  ok(latestUserAnchor !== firstGestureAnchor, `continuous scrolling keeps the latest ${resolution} semantic anchor after user intent is consumed`);
  const lastValidAnchor = latestUserAnchor;
  await act(async () => settleLayout());
  let expectedPersistedAnchor = lastValidAnchor;
  if (resolution === "persisted") {
    for (const anchor of mountedAnchors) anchor.removeAttribute("id");
    await act(async () => {
      transcript.scrollTop = 180;
      transcript.dispatchEvent(new window.Event("scroll"));
      await flush();
    });
    const revalidatedItems = parentItems.map((item) => ({ ...item, id: `history-${item.id}` }));
    expectedPersistedAnchor = `history-${lastValidAnchor}`;
    dispatchProgrammaticScroll = false;
    await act(async () => {
      render(root,
        <React.StrictMode><Transcript
          items={revalidatedItems}
          tabId={`user-${resolution}-tab`}
          viewKey={`user-${resolution}-session`}
          onPrompt={() => {}}
          activeExpert={activeExpert}
          initialScrollAnchor={{ atBottom: false, anchorMessageId: initialAnchorMessageId, offset: 24 }}
          historyRevalidationSignal={1}
          onScrollAnchorChange={(_, anchor) => {
            if (anchor.atBottom === false) latestUserAnchor = anchor.anchorMessageId;
          }}
          questionNavigator={false}
        /></React.StrictMode>,
      );
      await flush();
    });
    ok(latestUserAnchor === expectedPersistedAnchor, "history revalidation remaps the latest semantic anchor to the replacement user-message identity");
    dispatchProgrammaticScroll = true;
  }
  userRestoreSeeks.length = 0;

  await act(async () => settleLayout());
  ok(latestUserAnchor === expectedPersistedAnchor && currentScrollTop() < 20_000, `${resolution} persisted presentation retains the latest user identity through bounded revalidation restore`);

  await act(async () => root.unmount());
  dom.window.close();
}

for (const resolution of ["persisted", "null"] as const) {
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  const team: ExpertTeamSession = {
    state: "team_active",
    id: `reveal-${resolution}-team`,
    name: `Reveal ${resolution} Team`,
    lead: { memberKey: "lead", name: "Reveal Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Reveal Reviewer", role: "review", subagentRef: `sa-reveal-${resolution}`, status: "interrupted" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }) } },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" };
  const baseItems: Item[] = [task(`reveal-${resolution}-task`, "Reveal Reviewer"), ...questionItems(`reveal-${resolution}`, 48)];
  const anchorMessageId = `reveal-${resolution}-question-10`;
  const restoreSeeks: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      restoreSeeks.push(Number(top));
      this.scrollTop = Number(top);
      this.dispatchEvent(new window.Event("scroll"));
    },
  });
  const interruptedNotice = markNoticeForReveal({
    kind: "notice",
    id: `reveal-${resolution}-notice`,
    level: "warn",
    text: "本轮已中断",
  });

  await act(async () => {
    render(root,
      <Transcript
        items={baseItems}
        tabId={`reveal-${resolution}-tab`}
        viewKey={`reveal-${resolution}-session`}
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />,
    );
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing transcript scroller");
  const currentScrollTop = instrumentTranscriptScroll(transcript, 3_600);

  await act(async () => {
    render(root,
      <Transcript
        items={[...baseItems, interruptedNotice]}
        tabId={`reveal-${resolution}-tab`}
        viewKey={`reveal-${resolution}-session`}
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />,
    );
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  ok(currentScrollTop() === 20_000, `explicit interruption reveal reaches the bottom before ${resolution} sidecar settlement`);
  restoreSeeks.length = 0;
  globalThis.ResizeObserver = SilentResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = SilentResizeObserver as unknown as typeof ResizeObserver;

  await act(async () => {
    resolveTeam?.(resolution === "persisted" ? team : null);
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  await act(async () => settleLayout());
  await act(async () => {
    render(root,
      <Transcript
        items={[...baseItems, interruptedNotice, { kind: "notice", id: `reveal-${resolution}-settled`, level: "info", text: "后台状态已更新" }]}
        tabId={`reveal-${resolution}-tab`}
        viewKey={`reveal-${resolution}-session`}
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />,
    );
    await new Promise((resolve) => setTimeout(resolve, 600));
  });
  ok(currentScrollTop() === 20_000, `${resolution} sidecar settlement cannot undo the explicit interruption reveal`);
  if (resolution === "persisted") {
    ok(!restoreSeeks.includes(0), "persisted metadata does not remount the parent virtualizer or dispatch a top seek");
  }
  ok(!restoreSeeks.some((top) => top > 0), `${resolution} sidecar settlement cannot replay a cleared semantic restore job`);

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const resolveTeams: Array<(team: ExpertTeamSession | null) => void> = [];
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "anchor-team",
    name: "Anchor Team",
    lead: { memberKey: "lead", name: "Anchor Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Anchor Reviewer", role: "review", subagentRef: "sa-anchor", status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeams.push(resolve); }),
      },
    },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" };
  const parentItems: Item[] = [task("anchor-task", "Anchor Reviewer"), ...questionItems("anchor", 24)];
  const anchorMessageId = "anchor-question-10";
  const persistedSeeks: number[] = [];
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      if (this.querySelector?.('[data-testid="expert-team-overview-virtual-list"]')) persistedSeeks.push(Number(top));
      this.scrollTop = Number(top);
      this.dispatchEvent(new window.Event("scroll"));
    },
  });

  await act(async () => {
    render(root,
      <Transcript
        items={parentItems}
        tabId="anchor-tab"
        viewKey="anchor-session"
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
      />,
    );
    await settleLayout();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing transcript scroller");
  const currentScrollTop = instrumentTranscriptScroll(transcript, 0);
  await act(async () => {
    transcript.scrollTop = 0;
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });

  await act(async () => {
    resolveTeams[0]?.(null);
    await flush();
  });
  await act(async () => new Promise((resolve) => setTimeout(resolve, 600)));
  await act(async () => {
    render(root,
      <Transcript
        items={parentItems}
        tabId="anchor-tab"
        viewKey="anchor-session"
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: 24 }}
        questionNavigator={false}
        subagentStatusByRef={{ "sa-anchor": "running" }}
      />,
    );
    await flush();
  });
  await act(async () => new Promise((resolve) => setTimeout(resolve, 600)));
  await act(async () => {
    transcript.scrollTop = 0;
    transcript.dispatchEvent(new window.Event("scroll"));
    resolveTeams[1]?.(team);
    await flush();
  });
  await act(async () => new Promise((resolve) => setTimeout(resolve, 600)));
  await act(async () => settleLayout());
  await act(async () => settleLayout());
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll"));
    await flush();
  });
  ok(document.querySelector('[data-team-state="team_active"]') !== null, "late sidecar replaces the legacy expert virtualizer");
  ok(persistedSeeks.some((top) => top > 0), "the replacement persisted virtualizer receives a semantic anchor seek");
  ok(currentScrollTop() > 0, "the replacement virtualizer does not leave the semantic view at the top");
  ok(document.getElementById(questionAnchorId(anchorMessageId)) !== null, "the replacement virtualizer mounts the target semantic anchor");
  await act(async () => settleLayout());

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let requestCount = 0;
  const runningTeam: ExpertTeamSession = {
    state: "team_active",
    id: "fast-team",
    name: "Fast Team",
    lead: { memberKey: "lead", name: "Fast Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Fast Reviewer", role: "review", subagentRef: "sa-fast", status: "running" }],
  };
  const completedTeam: ExpertTeamSession = {
    ...runningTeam,
    members: [{ ...runningTeam.members[0], status: "completed" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: {
      App: {
        ExpertTeamForTab: async () => {
          requestCount += 1;
          return requestCount === 1 ? runningTeam : completedTeam;
        },
      },
    },
  };
  const activeExpert = { id: "fast-team", name: "Fast Team", mode: "team" };
  const parentWithoutTaskCard = assistantItems("fast-parent", 2);

  await act(async () => {
    render(root, <Transcript items={parentWithoutTaskCard} tabId="fast-tab" viewKey="fast-session" onPrompt={() => {}} activeExpert={activeExpert} subagentStatusByRef={{}} />);
    await flush();
    await flush();
  });
  ok(requestCount === 1, "initial team sidecar is loaded once");
  ok(document.querySelector(".agent-workspace__status--running") !== null, "initial persisted running state is rendered");

  await act(async () => {
    render(root, <Transcript items={parentWithoutTaskCard} tabId="fast-tab" viewKey="fast-session" onPrompt={() => {}} activeExpert={activeExpert} subagentStatusByRef={{ "sa-fast": "completed" }} />);
    await flush();
    await flush();
  });
  ok(requestCount === 2, "a status signal triggers exactly one authoritative team reload");
  ok(document.querySelector(".agent-workspace__status--done") !== null, "terminal status refresh works before a parent task card exists");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "interrupt-team",
    name: "Interrupt Team",
    lead: { memberKey: "lead", name: "Interrupt Lead", role: "lead" },
    members: [{ memberKey: "reviewer", name: "Interrupt Reviewer", role: "review", subagentRef: "sa-interrupted", status: "interrupted" }],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => team } },
  };
  const baseItems = assistantItems("interrupt-parent", 48);
  const normalNotice: Item = { kind: "notice", id: "normal-notice", level: "info", text: "普通后台更新" };
  const interruptedNotice = markNoticeForReveal({
    kind: "notice", id: "interrupted-notice", level: "warn", text: "本轮已中断",
  });

  await act(async () => {
    render(root, <Transcript items={baseItems} tabId="interrupt-tab" viewKey="interrupt-session" onPrompt={() => {}} activeExpert={{ id: team.id, name: team.name, mode: "team" }} />);
    await flush();
    await flush();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing transcript scroller");
  const currentScrollTop = instrumentTranscriptScroll(transcript, 3_600);
  await act(async () => settleLayout());
  await act(async () => scrollViewport(transcript, 3_600));

  await act(async () => {
    render(root, <Transcript items={[...baseItems, normalNotice]} tabId="interrupt-tab" viewKey="interrupt-session" onPrompt={() => {}} activeExpert={{ id: team.id, name: team.name, mode: "team" }} />);
    await settleLayout();
  });
  ok(currentScrollTop() === 3_600, "an ordinary notice preserves the user's middle-of-history anchor");

  await act(async () => {
    render(root, <Transcript items={[...baseItems, normalNotice, interruptedNotice]} tabId="interrupt-tab" viewKey="interrupt-session" onPrompt={() => {}} activeExpert={{ id: team.id, name: team.name, mode: "team" }} />);
    await settleLayout();
  });
  ok(currentScrollTop() === 20_000, "an explicit parent interruption reveals the bottom of a long persisted expert overview");
  await act(async () => {
    transcript.dispatchEvent(new window.Event("scroll"));
    await settleLayout();
  });
  ok(document.body.textContent?.includes("本轮已中断") === true, "the revealed expert interruption remains in the parent transcript");

  await act(async () => scrollViewport(transcript, 4_200));
  const ordinaryFailure: Item = { kind: "notice", id: "ordinary-failure", level: "warn", text: "provider settlement failed" };
  const ordinaryMessage: Item = { kind: "assistant", id: "ordinary-message", text: "稍后补充", reasoning: "", streaming: false };
  await act(async () => {
    render(root, <Transcript items={[...baseItems, normalNotice, interruptedNotice, ordinaryFailure, ordinaryMessage]} tabId="interrupt-tab" viewKey="interrupt-session" onPrompt={() => {}} activeExpert={{ id: team.id, name: team.name, mode: "team" }} />);
    await settleLayout();
  });
  ok(currentScrollTop() === 4_200, "the consumed interruption reveal and later ordinary errors or messages do not steal the user's anchor");

  await act(async () => root.unmount());
  globalThis.ResizeObserver = SilentResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = SilentResizeObserver as unknown as typeof ResizeObserver;
  Object.defineProperty(dom.window.HTMLElement.prototype, "clientHeight", { configurable: true, get: () => 800 });
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollHeight", { configurable: true, get: () => 20_000 });
  const sameViewRoot = createRoot(rootElement);
  await act(async () => {
    render(sameViewRoot, <Transcript items={[...baseItems, normalNotice, interruptedNotice]} tabId="interrupt-tab-remount" viewKey="interrupt-session" initialScrollAnchor={{ atBottom: false, anchorMessageId: "unavailable", offset: 0 }} hydrating onPrompt={() => {}} />);
    await flush();
  });
  const remountedTranscript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!remountedTranscript) throw new Error("missing remounted transcript scroller");
  const remountedScrollTop = instrumentTranscriptScroll(remountedTranscript, 4_600);
  await act(async () => settleLayout());
  ok(remountedScrollTop() === 4_600, "remounting the same semantic view does not reveal the same interruption again");

  await act(async () => sameViewRoot.unmount());
  const otherViewRoot = createRoot(rootElement);
  await act(async () => {
    render(otherViewRoot, <Transcript items={[...baseItems, normalNotice, interruptedNotice]} tabId="interrupt-tab-other" viewKey="interrupt-session-b" initialScrollAnchor={{ atBottom: false, anchorMessageId: "unavailable", offset: 0 }} hydrating onPrompt={() => {}} />);
    await flush();
  });
  const otherViewTranscript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!otherViewTranscript) throw new Error("missing other-view transcript scroller");
  const otherViewScrollTop = instrumentTranscriptScroll(otherViewTranscript, 4_800);
  await act(async () => settleLayout());
  ok(otherViewScrollTop() === 20_000, "a different semantic view can reveal the same interruption once");

  await act(async () => otherViewRoot.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => persistedTeam("tall-team", "Tall Team") } },
  };
  const priorQuestion: Item = { kind: "user", id: "tall-prior-question", text: "Prepare the expert investigation" };
  const question: Item = { kind: "user", id: "tall-question", text: "Inspect the tall expert result" };
  const items: Item[] = [priorQuestion, ...assistantItems("tall-intro", 3), question, task("tall-task", "Tall Reviewer"), ...assistantItems("tall-result", 32)];
  let captured: SessionScrollAnchor | undefined;
  const capturedAnchor = () => captured;
  const capturedAnchors: SessionScrollAnchor[] = [];

  await act(async () => {
    render(root,
      <Transcript
        items={items}
        tabId="tall-tab"
        viewKey="tall-session"
        onPrompt={() => {}}
        activeExpert={{ id: "tall-team", name: "Tall Team", mode: "team" }}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(_, anchor) => {
          captured = anchor;
          capturedAnchors.push(anchor);
        }}
        questionNavigator={false}
      />,
    );
    await settleLayout();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing tall transcript scroller");
  const currentScrollTop = instrumentTranscriptScroll(transcript, 19_200);
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  const virtualList = transcript.querySelector(".transcript-virtual-list") as HTMLDivElement | null;
  if (!virtualList) throw new Error("missing tall virtual list");
  const targetQuestionNode = document.getElementById(questionAnchorId("tall-question"));
  const targetRowIndex = Number(targetQuestionNode?.closest<HTMLElement>(".transcript-virtual-row")?.dataset.index ?? -1);
  const nestedListTop = 240;
  const targetRowStart = targetRowIndex * 180;
  ok(targetRowIndex > 0, "the exact fallback fixture places its question at a non-zero virtual row start");
  virtualList.getBoundingClientRect = () => ({ top: nestedListTop - currentScrollTop() }) as DOMRect;

  await act(async () => {
    transcript.scrollTop = 3_400;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await settleLayout();
  });
  ok(document.getElementById(questionAnchorId("tall-question")) === null, "a tall expert middle window can contain no mounted question DOM");
  captured = undefined;
  capturedAnchors.length = 0;
  await act(async () => {
    transcript.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    transcript.scrollTop = 3_600;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  const expectedOffsetA = 3_600 - nestedListTop - targetRowStart;
  const capturedA = capturedAnchor();
  ok(capturedA?.atBottom === false && capturedA.offset === expectedOffsetA, "gesture scroll A captures the DOM-empty virtual offset");
  await act(async () => {
    transcript.scrollTop = 3_800;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  const expectedOffsetB = 3_800 - nestedListTop - targetRowStart;
  const capturedB = capturedAnchor();
  ok(capturedB?.atBottom === false && capturedB.anchorMessageId === "tall-question", "a DOM-empty expert middle scroll captures the nearest preceding question identity");
  ok(capturedB?.atBottom === false && capturedB.offset === expectedOffsetB, "one gesture keeps the exact latest offset with non-zero nested-list and question-row contributions");

  await act(async () => {
    document.body.dispatchEvent(new window.Event("pointerup", { bubbles: true }));
    transcript.scrollTop = 4_000;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  ok(capturedAnchors.at(-1)?.atBottom === false && (capturedAnchors.at(-1) as { offset: number }).offset === expectedOffsetB, "an external pointerup ends the gesture before programmatic scroll C");

  await act(async () => {
    transcript.dispatchEvent(new window.Event("pointerdown", { bubbles: true }));
    transcript.scrollTop = 19_200;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    document.body.dispatchEvent(new window.Event("pointerup", { bubbles: true }));
    await flush();
  });
  ok(capturedAnchors.at(-1)?.atBottom === true, "the wheel-burst regression starts from the pinned bottom state");

  captured = undefined;
  capturedAnchors.length = 0;
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
    await nextAnimationFrame();
    transcript.scrollTop = 4_200;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await nextAnimationFrame();
    transcript.scrollTop = 4_400;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await nextAnimationFrame();
    transcript.scrollTop = 4_600;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await nextAnimationFrame();
    transcript.scrollTop = 4_800;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await nextAnimationFrame();
    transcript.scrollTop = 5_000;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  const expectedWheelOffset = 4_800 - nestedListTop - targetRowStart;
  const capturedWheel = capturedAnchor();
  ok(capturedWheel?.atBottom === false && capturedWheel.anchorMessageId === "tall-question", "one wheel burst keeps the nearest preceding question through a DOM-empty high row");
  ok(capturedWheel?.atBottom === false && capturedWheel.offset === expectedWheelOffset, "one wheel input covers through the fourth-frame measurement and expires before the fifth-frame scroll");
  const savedAnchor: SessionScrollAnchor = capturedAnchor() ?? { atBottom: true };
  ok(capturedAnchors.at(-1)?.atBottom === false && (capturedAnchors.at(-1) as { offset: number }).offset === expectedWheelOffset, "a post-window programmatic scroll cannot overwrite the final wheel-burst anchor");
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
    await settleLayout();
    transcript.scrollTop = 5_400;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  ok(capturedAnchors.at(-1)?.atBottom === false && (capturedAnchors.at(-1) as { offset: number }).offset === expectedWheelOffset, "a wheel with no resulting scroll leaves no stale intent after its bounded window");

  captured = undefined;
  await act(async () => {
    transcript.scrollTop = 19_200;
    transcript.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    await flush();
  });
  const capturedBottom = capturedAnchor();
  ok(capturedBottom?.atBottom === true, "the expert virtual fallback preserves explicit bottom semantics");
  await act(async () => root.unmount());
  const remountRoot = createRoot(rootElement);
  const remountScrollTops = new WeakMap<HTMLElement, number>();
  Object.defineProperty(dom.window.HTMLElement.prototype, "clientHeight", { configurable: true, get: () => 800 });
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollHeight", { configurable: true, get: () => 20_000 });
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTop", {
    configurable: true,
    get() { return remountScrollTops.get(this) ?? 0; },
    set(value) { remountScrollTops.set(this, Number(value)); },
  });
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      this.scrollTop = Number(top);
      this.dispatchEvent(new window.Event("scroll", { bubbles: true }));
    },
  });
  const originalGetBoundingClientRect = dom.window.HTMLElement.prototype.getBoundingClientRect;
  Object.defineProperty(dom.window.HTMLElement.prototype, "getBoundingClientRect", {
    configurable: true,
    value: function getBoundingClientRect() {
      const element = this as HTMLElement;
      const transcriptElement = element.classList.contains("transcript")
        ? element
        : element.closest<HTMLElement>(".transcript");
      if (element.classList.contains("transcript")) return { top: 0 } as DOMRect;
      if (element.classList.contains("transcript-virtual-list")) {
        return { top: nestedListTop - (transcriptElement?.scrollTop ?? 0) } as DOMRect;
      }
      if (element.id === questionAnchorId("tall-question")) {
        return { top: nestedListTop - (transcriptElement?.scrollTop ?? 0) + targetRowStart } as DOMRect;
      }
      return originalGetBoundingClientRect.call(element);
    },
  });
  await act(async () => {
    render(remountRoot,
      <Transcript
        items={items}
        tabId="tall-tab-remount"
        viewKey="tall-session"
        onPrompt={() => {}}
        activeExpert={{ id: "tall-team", name: "Tall Team", mode: "team" }}
        initialScrollAnchor={savedAnchor}
        questionNavigator={false}
      />,
    );
    await flush();
  });
  const remountedTranscript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!remountedTranscript) throw new Error("missing remounted tall transcript scroller");
  await act(async () => settleLayout());
  ok(savedAnchor.atBottom === false && remountedTranscript.scrollTop === 4_800, "remount restores the exact stable wheel-burst offset instead of the stale bottom");

  await act(async () => remountRoot.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  globalThis.ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  (dom.window as unknown as { ResizeObserver: typeof ResizeObserver }).ResizeObserver = MeasuredRowResizeObserver as unknown as typeof ResizeObserver;
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => persistedTeam("wheel-origin-team", "Wheel Origin Team") } },
  };
  const activeExpert = { id: "wheel-origin-team", name: "Wheel Origin Team", mode: "team" as const };
  const itemsFor = (view: string): Item[] => [
    { kind: "user", id: `${view}-question`, text: `${view} question` },
    task(`${view}-task`, `${view} reviewer`),
    ...assistantItems(`${view}-result`, 32),
  ];
  const capturedByView: Array<{ key: string; anchor: SessionScrollAnchor }> = [];
  const onScrollAnchorChange = (key: string, anchor: SessionScrollAnchor) => {
    capturedByView.push({ key, anchor });
  };
  await act(async () => {
    render(root,
      <TranscriptWithLayoutScroll
        items={itemsFor("wheel-origin-a")}
        tabId="wheel-origin-tab"
        viewKey="wheel-origin-a"
        onPrompt={() => {}}
        activeExpert={activeExpert}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={onScrollAnchorChange}
        questionNavigator={false}
      />,
    );
    await settleLayout();
  });
  const transcript = document.querySelector(".transcript") as HTMLDivElement | null;
  if (!transcript) throw new Error("missing wheel-origin transcript scroller");
  instrumentTranscriptScroll(transcript, 19_200);
  transcript.getBoundingClientRect = () => ({ top: 0 }) as DOMRect;
  capturedByView.length = 0;
  await act(async () => {
    transcript.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -1_300 }));
    flushSync(() => {
      render(root,
        <TranscriptWithLayoutScroll
          items={itemsFor("wheel-origin-b")}
          tabId="wheel-origin-tab"
          viewKey="wheel-origin-b"
          layoutScrollTop={4_400}
          onPrompt={() => {}}
          activeExpert={activeExpert}
          initialScrollAnchor={{ atBottom: true }}
          onScrollAnchorChange={onScrollAnchorChange}
          questionNavigator={false}
        />,
      );
    });
  });
  ok(
    !capturedByView.some(({ key, anchor }) => key === "wheel-origin-b" && anchor.atBottom === false),
    "a new semantic view layout scroll cannot consume the previous view's active wheel window before passive cleanup",
  );

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const team: ExpertTeamSession = {
    state: "team_active",
    id: "blank-team",
    name: "Blank Team",
    lead: { memberKey: "lead", name: "Blank Lead", role: "lead" },
    members: [],
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => team } },
  };
  const activeExpert = { id: team.id, name: team.name, mode: "team" as const };

  await act(async () => {
    render(root, <Transcript items={[]} tabId="blank-team-tab" viewKey="blank-team-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  ok(document.querySelector(".agent-workspace") === null && document.body.textContent?.includes("Blank Lead") !== true, "a selected team does not show its workspace avatar before the first message");

  await act(async () => {
    render(root, <Transcript items={[{ kind: "user", id: "blank-team-question", text: "Start the team" }]} tabId="blank-team-tab" viewKey="blank-team-session" onPrompt={() => {}} activeExpert={activeExpert} />);
    await flush();
  });
  ok(document.querySelector('[data-team-state="team_active"]') !== null && document.body.textContent?.includes("Blank Lead") === true, "the team workspace appears after the first message is sent");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  let requests = 0;
  const teamA: ExpertTeamSession = {
    state: "team_active",
    id: "team-a",
    name: "Team A",
    lead: { memberKey: "lead", name: "Lead A", role: "lead" },
    members: [],
  };
  const teamB: ExpertTeamSession = {
    ...teamA,
    id: "team-b",
    name: "Team B",
    lead: { memberKey: "lead", name: "Lead B", role: "lead" },
  };
  (dom.window as unknown as { go: unknown }).go = {
    main: { App: { ExpertTeamForTab: async () => requests++ === 0 ? teamA : teamB } },
  };
  const items: Item[] = [{ kind: "user", id: "team-switch-question", text: "switch team" }];

  await act(async () => {
    render(root, <Transcript items={items} tabId="team-switch-tab" viewKey="team-switch-session" onPrompt={() => {}} activeExpert={{ id: "team-a", name: "Team A", mode: "team" }} />);
    await flush();
  });
  ok(requests === 1 && document.body.textContent?.includes("Lead A") === true, "expert team workspace loads the initial sidecar");

  await act(async () => {
    render(root, <Transcript items={items} tabId="team-switch-tab" viewKey="team-switch-session" onPrompt={() => {}} activeExpert={{ id: "team-b", name: "Team B", mode: "team" }} />);
    await flush();
  });
  ok(requests === 2 && document.body.textContent?.includes("Lead B") === true, "changing the expert team id refreshes the sidecar instead of reusing stale member identity");

  await act(async () => root.unmount());
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
