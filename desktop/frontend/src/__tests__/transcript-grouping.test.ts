// Run: tsx src/__tests__/transcript-grouping.test.ts

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { buildTurnGroups, scrollVersion } from "../lib/transcriptGrouping";
import { useScrollManager } from "../lib/useScrollManager";
import type { Item } from "../lib/useController";

let passed = 0;
let failed = 0;

function ok(cond: boolean, label: string) {
  if (cond) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq<T>(actual: T, expected: T, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function installTranscriptDom() {
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
    for (let i = 0; i < 20 && frames.size > 0; i += 1) {
      await flushOneFrame();
    }
  };
  return { dom, flushOneFrame, flushAllFrames };
}

function installScrollMetrics(el: HTMLElement, readScrollHeight: () => number, clientHeight = 640) {
  let scrollTop = 0;
  Object.defineProperty(el, "clientHeight", { configurable: true, get: () => clientHeight });
  Object.defineProperty(el, "scrollHeight", { configurable: true, get: readScrollHeight });
  Object.defineProperty(el, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => {
      scrollTop = Number(value);
    },
  });
  return {
    get scrollTop() { return scrollTop; },
    set scrollTop(value: number) { scrollTop = value; },
  };
}

function syntheticTranscriptItems(turns: number, toolsPerTurn: number): Item[] {
  const items: Item[] = [];
  let seq = 0;
  for (let turn = 0; turn < turns; turn += 1) {
    items.push({ kind: "user", id: `u${seq++}`, text: `prompt ${turn}` });
    items.push({ kind: "assistant", id: `a${seq++}`, text: `answer ${turn}`, reasoning: "", streaming: false });
    for (let tool = 0; tool < toolsPerTurn; tool += 1) {
      items.push({
        kind: "tool",
        id: `t${seq++}`,
        name: "bash",
        args: "",
        readOnly: false,
        status: "done",
        dataArchived: true,
      });
    }
  }
  return items;
}

type ScrollManagerForTest = ReturnType<typeof useScrollManager> & {
  pinToBottomAfterLayout?: (frames?: number) => void;
};

function ScrollProbe({ onManager, viewKey }: {
  onManager: (manager: ScrollManagerForTest) => void;
  viewKey: string;
}) {
  const manager = useScrollManager("", viewKey) as ScrollManagerForTest;
  onManager(manager);
  return React.createElement("div", { id: "scroll-probe", ref: manager.scrollRef });
}

function renderScrollProbe(root: Root, onManager: (manager: ScrollManagerForTest) => void, viewKey = "") {
  root.render(React.createElement(ScrollProbe, { onManager, viewKey }));
}

console.log("\ntranscript grouping contract");

{
  const here = dirname(fileURLToPath(import.meta.url));
  const groupingPath = resolve(here, "../lib/transcriptGrouping.ts");
  const transcriptPath = resolve(here, "../components/Transcript.tsx");
  const source = readFileSync(groupingPath, "utf8");
  const transcriptSource = readFileSync(transcriptPath, "utf8");
  ok(!source.includes(".findIndex("), "turn grouping does not scan a second collection for each item");
  ok(/pinToBottomAfterLayout\(\d+\)/.test(transcriptSource), "streaming transcript auto-scroll uses pinned multi-frame layout follow-up");
}

{
  const groups = buildTurnGroups(syntheticTranscriptItems(3, 2));
  eq(groups.length, 3, "creates one group per user turn");
  eq(groups[0].startIdx, 0, "first group start index");
  eq(groups[0].endIdx, 4, "first group end index");
  eq(groups[0].toolCount, 2, "counts top-level tools in a turn");
  eq(groups[2].assistantPreview, "answer 2", "keeps latest assistant preview for each turn");
}

{
  const groups = buildTurnGroups([
    { kind: "user", id: "u-preview", text: "coordinate experts" },
    { kind: "assistant", id: "lead-preview", text: "lead expert synthesis", reasoning: "", streaming: false },
    { kind: "tool", id: "delegate-task", name: "task", args: "{}", readOnly: false, status: "done" },
    { kind: "assistant", id: "child-preview", parentId: "delegate-task", text: "child expert private reply", reasoning: "", streaming: false },
  ]);
  eq(groups[0].assistantPreview, "lead expert synthesis", "turn preview ignores nested child assistant replies");
}

{
  const groups = buildTurnGroups([
    { kind: "user", id: "u-read-only-preview", text: "inspect with an expert" },
    { kind: "assistant", id: "lead-read-only-preview", text: "lead expert conclusion", reasoning: "", streaming: false },
    { kind: "tool", id: "delegate-read-only-task", name: "read_only_task", args: "{}", readOnly: true, status: "done" },
    { kind: "assistant", id: "child-read-only-preview", parentId: "delegate-read-only-task", text: "private read-only reply", reasoning: "", streaming: false },
  ]);
  eq(groups[0].assistantPreview, "lead expert conclusion", "turn preview ignores read-only child expert replies");
}

{
  const items: Item[] = [
    { kind: "user", id: "u1", text: "prompt" },
    { kind: "tool", id: "tool1", name: "task", args: "", readOnly: false, status: "running" },
    { kind: "assistant", id: "child-a1", parentId: "tool1", text: "short", reasoning: "", streaming: true },
  ];
  const before = scrollVersion(items);
  const after = scrollVersion([
    items[0],
    items[1],
    { kind: "assistant", id: "child-a1", parentId: "tool1", text: "short answer still streaming", reasoning: "", streaming: true },
  ]);
  ok(before !== after, "scroll version changes when nested streaming assistant text grows");
}

{
  const { dom, flushOneFrame, flushAllFrames } = installTranscriptDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let manager: ScrollManagerForTest | undefined;
  let scrollHeight = 1000;

  await act(async () => {
    renderScrollProbe(root, (next) => {
      manager = next;
    });
    await flushPromises();
  });
  const probe = document.getElementById("scroll-probe");
  if (!probe) throw new Error("missing scroll probe");
  const metrics = installScrollMetrics(probe, () => scrollHeight);
  await flushAllFrames();

  ok(typeof manager?.pinToBottomAfterLayout === "function", "scroll manager exposes pinned multi-frame layout follow-up");
  scrollHeight = 1200;
  manager?.pinToBottomAfterLayout?.(3);
  await flushOneFrame();
  eq(metrics.scrollTop, 1200, "pinned layout follow-up scrolls to the first measured bottom");

  scrollHeight = 1600;
  await flushOneFrame();
  eq(metrics.scrollTop, 1600, "pinned layout follow-up follows delayed layout growth");

  metrics.scrollTop = 400;
  manager!.stick.current = false;
  scrollHeight = 2200;
  manager?.pinToBottomAfterLayout?.(2);
  await flushOneFrame();
  eq(metrics.scrollTop, 400, "pinned layout follow-up respects manual scroll unpin");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const { dom, flushAllFrames } = installTranscriptDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let manager: ScrollManagerForTest | undefined;
  const receiveManager = (next: ScrollManagerForTest) => { manager = next; };
  await act(async () => {
    renderScrollProbe(root, receiveManager, "task-a");
    await flushPromises();
  });
  const probe = document.getElementById("scroll-probe");
  if (!probe) throw new Error("missing scroll probe");
  const metrics = installScrollMetrics(probe, () => 2_000);
  await act(async () => manager!.scrollToBottomAfterLayout(5));
  await act(async () => {
    renderScrollProbe(root, receiveManager, "task-b");
    await flushPromises();
  });
  ok(document.getElementById("scroll-probe") === probe, "task switching reuses the transcript scroll element");
  metrics.scrollTop = 320;
  manager!.stick.current = false;
  await flushAllFrames();
  eq(metrics.scrollTop, 320, "task A's pending forced scroll cannot overwrite task B's restored anchor");
  eq(manager!.stick.current, false, "task A's pending frames cannot repin task B to the bottom");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const items = syntheticTranscriptItems(10_000, 1);
  const start = performance.now();
  const groups = buildTurnGroups(items);
  const elapsed = performance.now() - start;
  eq(groups.length, 10_000, "large transcript grouping keeps every turn");
  ok(elapsed < 50, `groups 10k turns in ${elapsed.toFixed(2)}ms`);
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
