// Run: tsx src/__tests__/transcript-scroll-restore.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import { transcriptAnchorRestoreGeometryStable } from "../lib/transcriptScrollAnchor";
import { questionAnchorId } from "../lib/transcriptGrouping";
import type { Item, SessionScrollAnchor } from "../lib/useController";

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

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}${actual === expected ? "" : `: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`}`);
}

ok(transcriptAnchorRestoreGeometryStable({
  targetOffset: 732.2,
  measuredOffset: 611.6,
  scrollTop: 6_281.6,
  maxScrollTop: 6_281.6,
  layoutStable: true,
  hasMeasuredVirtualGeometry: true,
}), "trusted measured geometry settles at the closest bottom-clamped semantic offset");
ok(transcriptAnchorRestoreGeometryStable({
  targetOffset: -120.6,
  measuredOffset: 0,
  scrollTop: 0,
  maxScrollTop: 6_281.6,
  layoutStable: true,
  hasMeasuredVirtualGeometry: true,
}), "trusted measured geometry settles at the closest top-clamped semantic offset");
ok(!transcriptAnchorRestoreGeometryStable({
  targetOffset: 732.2,
  measuredOffset: 611.6,
  scrollTop: 6_160,
  maxScrollTop: 6_281.6,
  layoutStable: true,
  hasMeasuredVirtualGeometry: true,
}), "a reachable semantic offset near the boundary must continue correcting");
ok(!transcriptAnchorRestoreGeometryStable({
  targetOffset: 732.2,
  measuredOffset: 611.6,
  scrollTop: 6_281.6,
  maxScrollTop: 6_281.6,
  layoutStable: true,
  hasMeasuredVirtualGeometry: false,
}), "an unmeasured boundary estimate cannot complete semantic restore");
ok(!transcriptAnchorRestoreGeometryStable({
  targetOffset: 732.2,
  measuredOffset: 611.6,
  scrollTop: 6_281.6,
  maxScrollTop: 6_281.6,
  layoutStable: false,
  hasMeasuredVirtualGeometry: true,
}), "layout churn cannot complete a clamped semantic restore");

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  const frames = new Map<number, FrameRequestCallback>();
  let nextFrameId = 0;
  const requestFrame = (callback: FrameRequestCallback) => {
    const id = ++nextFrameId;
    frames.set(id, callback);
    return id;
  };
  const cancelFrame = (id: number) => {
    frames.delete(id);
  };

  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.Node = dom.window.Node;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = requestFrame;
  globalThis.cancelAnimationFrame = cancelFrame;
  dom.window.requestAnimationFrame = requestFrame;
  dom.window.cancelAnimationFrame = cancelFrame;
  Object.defineProperty(dom.window, "matchMedia", {
    configurable: true,
    value: () => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} }),
  });
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollTo", {
    configurable: true,
    value: function scrollTo(options: ScrollToOptions | number, y?: number) {
      this.scrollTop = typeof options === "number" ? y ?? 0 : options.top ?? 0;
    },
  });
  const defaultScrollTops = new WeakMap<HTMLElement, number>();
  Object.defineProperties(dom.window.HTMLElement.prototype, {
    clientHeight: { configurable: true, get: () => 400 },
    scrollHeight: { configurable: true, get: () => 1_200 },
    scrollTop: {
      configurable: true,
      get: function getScrollTop(this: HTMLElement) {
        return defaultScrollTops.get(this) ?? 0;
      },
      set: function setScrollTop(this: HTMLElement, value: number) {
        defaultScrollTops.set(this, Number(value));
      },
    },
  });
  globalThis.ResizeObserver = class TestResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as typeof ResizeObserver;

  const flushOneFrame = async () => {
    const callbacks = [...frames.values()];
    frames.clear();
    await act(async () => {
      for (const callback of callbacks) callback(Date.now());
      await flushPromises();
    });
  };
  const flushAllFrames = async () => {
    for (let index = 0; index < 200 && frames.size > 0; index += 1) {
      await flushOneFrame();
    }
  };
  return { dom, flushOneFrame, flushAllFrames };
}

function items(...ids: string[]): Item[] {
  return ids.flatMap((id, index) => [
    { kind: "user", id, text: `prompt ${id}` },
    { kind: "assistant", id: `answer-${id}`, text: `answer ${index}`, reasoning: "", streaming: false },
  ]);
}

function renderTranscript(
  root: Root,
  props: {
    items: Item[];
    viewKey: string;
    initialScrollAnchor: SessionScrollAnchor;
    onScrollAnchorChange?: (key: string, anchor: SessionScrollAnchor) => void;
  },
) {
  root.render(
    <LocaleProvider>
      <Transcript
        {...props}
        onPrompt={() => {}}
        questionNavigator={false}
      />
    </LocaleProvider>,
  );
}

function installScrollLayout(
  container: HTMLElement,
  contentTopByAnchorID: ReadonlyMap<string, number>,
) {
  let scrollTop = 0;
  let emitScroll = false;
  const writes: number[] = [];
  Object.defineProperty(container, "clientHeight", { configurable: true, get: () => 400 });
  Object.defineProperty(container, "scrollHeight", { configurable: true, get: () => 5_000 });
  Object.defineProperty(container, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => {
      scrollTop = Number(value);
      writes.push(scrollTop);
      if (emitScroll) container.dispatchEvent(new window.Event("scroll"));
    },
  });
  container.getBoundingClientRect = () => ({ top: 100 }) as DOMRect;
  for (const anchorID of contentTopByAnchorID.keys()) {
    const node = container.querySelector<HTMLElement>(`#${anchorID}`);
    if (!node) throw new Error(`missing transcript anchor ${anchorID}`);
    node.getBoundingClientRect = () => ({
      top: 100 + (contentTopByAnchorID.get(anchorID) ?? 0) - scrollTop,
    }) as DOMRect;
  }
  return {
    writes,
    get scrollTop() { return scrollTop; },
    set emitScroll(value: boolean) { emitScroll = value; },
  };
}

console.log("\ntranscript scroll restore lifecycle");

{
  const { dom, flushOneFrame, flushAllFrames } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const anchorMessageId = "drift-q1";
  const canonicalOffset = -606.2;
  let sessionCache: SessionScrollAnchor = { atBottom: false, anchorMessageId, offset: canonicalOffset };

  await act(async () => {
    root.render(
      <LocaleProvider>
        <Transcript
          items={items("drift-q0", anchorMessageId)}
          viewKey="anchor-drift-session"
          initialScrollAnchor={{ atBottom: false, anchorMessageId, offset: canonicalOffset }}
          onScrollAnchorChange={(_, anchor) => {
            sessionCache = anchor;
          }}
          onPrompt={() => {}}
          questionNavigator={false}
        />
      </LocaleProvider>,
    );
    await flushPromises();
  });

  const container = document.querySelector<HTMLElement>(".transcript");
  if (!container) throw new Error("missing transcript");
  const metrics = installScrollLayout(container, new Map([
    [questionAnchorId("drift-q0"), 100],
    [questionAnchorId(anchorMessageId), 1_000],
  ]));
  const querySelectorAll = container.querySelectorAll.bind(container);
  let anchorMounted = false;
  Object.defineProperty(container, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !anchorMounted ? [] : querySelectorAll(selector),
  });
  metrics.emitScroll = true;

  for (let frame = 0; frame < 3; frame += 1) await flushOneFrame();
  anchorMounted = true;
  await flushOneFrame();
  await flushOneFrame();
  ok(Math.abs(metrics.scrollTop - 393.8) <= 0.001, "restore reaches the canonical -606.2 offset after three missing and two mounted frames");

  await act(async () => {
    container.querySelector<HTMLElement>(`#${questionAnchorId("drift-q0")}`)?.removeAttribute("id");
    container.scrollTop = 957.8;
    await flushPromises();
  });
  ok(
    sessionCache.atBottom === false
      && sessionCache.anchorMessageId === anchorMessageId
      && Math.abs(sessionCache.offset - canonicalOffset) <= 0.001,
    `late programmatic scroll cannot replace canonical ${canonicalOffset} with ${sessionCache.atBottom === false ? sessionCache.offset : "bottom"}`,
  );
  await flushAllFrames();
  ok(Math.abs(metrics.scrollTop - 393.8) <= 0.001, "active convergence corrects the late -42.2 viewport drift");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom, flushOneFrame } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const canonical: SessionScrollAnchor = { atBottom: false, anchorMessageId: "missing-q", offset: -606.2 };
  let sessionCache: SessionScrollAnchor = canonical;

  await act(async () => {
    renderTranscript(root, {
      items: items("missing-q"),
      viewKey: "incomplete-session",
      initialScrollAnchor: canonical,
      onScrollAnchorChange: (_, anchor) => { sessionCache = anchor; },
    });
    await flushPromises();
  });
  const container = document.querySelector<HTMLElement>(".transcript");
  if (!container) throw new Error("missing transcript");
  const metrics = installScrollLayout(container, new Map([[questionAnchorId("missing-q"), 1_000]]));
  const querySelectorAll = container.querySelectorAll.bind(container);
  let mounted = false;
  Object.defineProperty(container, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !mounted ? [] : querySelectorAll(selector),
  });
  metrics.emitScroll = true;

  for (let frame = 0; frame < 180; frame += 1) await flushOneFrame();
  mounted = true;
  await act(async () => {
    container.scrollTop = 957.8;
    await flushPromises();
  });
  ok(
    sessionCache.atBottom === false && sessionCache.anchorMessageId === "missing-q" && sessionCache.offset === -606.2,
    "the 180-frame hard limit keeps the canonical cache frozen after incomplete restore",
  );
  eq(metrics.scrollTop, 957.8, "the incomplete restore stops scheduling correction frames at its hard limit");

  const anchor = container.querySelector<HTMLElement>(`#${questionAnchorId("missing-q")}`);
  if (!anchor) throw new Error("missing mounted incomplete anchor");
  anchor.getBoundingClientRect = () => ({ top: 140 }) as DOMRect;
  await act(async () => {
    container.dispatchEvent(new window.WheelEvent("wheel", { bubbles: true, deltaY: -120 }));
    container.dispatchEvent(new window.Event("scroll"));
    await flushPromises();
  });
  ok(
    sessionCache.atBottom === false && sessionCache.anchorMessageId === "missing-q" && sessionCache.offset === -40,
    "real user wheel input cancels an incomplete restore and replaces the canonical cache",
  );

  const userScrollTop = metrics.scrollTop;
  await flushOneFrame();
  eq(metrics.scrollTop, userScrollTop, "an old restore RAF cannot reclaim the viewport after user cancellation");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const { dom, flushOneFrame, flushAllFrames } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);

  await act(async () => {
    renderTranscript(root, {
      items: items("u1"),
      viewKey: "session-a",
      initialScrollAnchor: { atBottom: false, anchorMessageId: "u1", offset: 40 },
    });
    await flushPromises();
  });

  const container = document.querySelector<HTMLElement>(".transcript");
  if (!container) throw new Error("missing transcript");
  const contentTopByAnchorID = new Map([[questionAnchorId("u1"), 150]]);
  const metrics = installScrollLayout(container, contentTopByAnchorID);
  const querySelectorAll = container.querySelectorAll.bind(container);
  let anchorMounted = false;
  Object.defineProperty(container, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !anchorMounted ? [] : querySelectorAll(selector),
  });

  await flushOneFrame();
  eq(metrics.scrollTop, 0, "the unavailable first layout frame does not invent a fallback position");
  anchorMounted = true;
  await flushOneFrame();
  eq(metrics.scrollTop, 190, "a semantic anchor appearing on the next layout frame restores its visual offset");
  contentTopByAnchorID.set(questionAnchorId("u1"), 190);
  await flushOneFrame();
  eq(metrics.scrollTop, 230, "the bounded restore window corrects later layout growth above the anchor");

  await act(async () => {
    root.unmount();
  });
  await flushAllFrames();
  dom.window.close();
}

{
  const { dom, flushOneFrame, flushAllFrames } = installDom();
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root");
  const root = createRoot(rootElement);
  const captured: Array<{ key: string; anchor: SessionScrollAnchor }> = [];

  await act(async () => {
    renderTranscript(root, {
      items: items("u1"),
      viewKey: "session-a",
      initialScrollAnchor: { atBottom: false, anchorMessageId: "u1", offset: 40 },
      onScrollAnchorChange: (key, anchor) => captured.push({ key, anchor }),
    });
    await flushPromises();
  });

  const container = document.querySelector<HTMLElement>(".transcript");
  if (!container) throw new Error("missing transcript");
  const querySelectorAll = container.querySelectorAll.bind(container);
  let anchorMounted = false;
  Object.defineProperty(container, "querySelectorAll", {
    configurable: true,
    value: (selector: string) => selector === "[id]" && !anchorMounted ? [] : querySelectorAll(selector),
  });
  const metrics = installScrollLayout(container, new Map([[questionAnchorId("u1"), 150]]));

  await flushOneFrame();
  eq(metrics.scrollTop, 0, "session A leaves one retry pending after its first missing-anchor frame");

  await act(async () => {
    renderTranscript(root, {
      items: items("u1", "u2"),
      viewKey: "session-b",
      initialScrollAnchor: { atBottom: false, anchorMessageId: "u2", offset: 20 },
      onScrollAnchorChange: (key, anchor) => captured.push({ key, anchor }),
    });
    await flushPromises();
  });
  const u2 = container.querySelector<HTMLElement>(`#${questionAnchorId("u2")}`);
  if (!u2) throw new Error("missing session B anchor");
  u2.getBoundingClientRect = () => ({ top: 100 + 300 - metrics.scrollTop }) as DOMRect;
  metrics.writes.length = 0;
  metrics.emitScroll = true;
  anchorMounted = true;

  await flushOneFrame();
  eq(metrics.scrollTop, 320, "session B restores its own anchor after cancelling session A");
  ok(!metrics.writes.includes(190), "session A's cancelled retry never writes its stale scroll position into session B");
  eq(captured.length, 0, "active programmatic restore does not snapshot even the foreground session identity");

  await flushAllFrames();
  eq(metrics.scrollTop, 320, "later frames cannot overwrite session B with session A's anchor");
  await act(async () => {
    container.dispatchEvent(new window.Event("scroll"));
    await flushPromises();
  });
  ok(
    captured.length === 1
      && captured[0]?.key === "session-b"
      && captured[0].anchor.atBottom === false
      && captured[0].anchor.anchorMessageId === "u2"
      && captured[0].anchor.offset === 20,
    "programmatic capture resumes for the foreground session after restore completion",
  );

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
