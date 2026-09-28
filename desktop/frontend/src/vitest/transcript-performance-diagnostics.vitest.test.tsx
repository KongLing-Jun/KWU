import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import {
  createTranscriptTrace,
  installTranscriptTraceForTest,
  TRANSCRIPT_TRACE_ENABLED,
} from "../lib/transcriptPerformanceTrace";
import { questionAnchorId } from "../lib/transcriptGrouping";
import type { ExpertTeamSession } from "../lib/types";
import type { Item, SessionScrollAnchor } from "../lib/useController";

vi.mock("gsap", () => {
  const gsap = { to: () => ({ kill() {} }) };
  return { default: gsap, gsap };
});

class MeasuredResizeObserver implements ResizeObserver {
  constructor(private readonly callback: ResizeObserverCallback) {}

  observe(target: Element) {
    const height = target.classList.contains("transcript-virtual-row") ? 180 : 800;
    this.callback([{
      target,
      contentRect: { width: 1_000, height } as DOMRectReadOnly,
      borderBoxSize: [{ inlineSize: 1_000, blockSize: height }],
      contentBoxSize: [{ inlineSize: 1_000, blockSize: height }],
      devicePixelContentBoxSize: [{ inlineSize: 1_000, blockSize: height }],
    }], this);
  }

  disconnect() {}
  unobserve() {}
}

const scrollTops = new WeakMap<HTMLElement, number>();
const originalDescriptors = new Map<string, PropertyDescriptor | undefined>();
let nestedListTop = 240;
let targetRowStart = 0;
let restoreInstalledTrace: (() => void) | undefined;
const privateViewKey = "diagnostic-private-view";
const privateAnchorID = "diagnostic-private-question";
const privateQuestionBody = "diagnostic private question body";

function setPrototypeProperty(name: string, descriptor: PropertyDescriptor) {
  originalDescriptors.set(name, Object.getOwnPropertyDescriptor(HTMLElement.prototype, name));
  Object.defineProperty(HTMLElement.prototype, name, { configurable: true, ...descriptor });
}

function restorePrototypeProperties() {
  for (const [name, descriptor] of originalDescriptors) {
    if (descriptor) Object.defineProperty(HTMLElement.prototype, name, descriptor);
    else delete (HTMLElement.prototype as unknown as Record<string, unknown>)[name];
  }
  originalDescriptors.clear();
}

function waitForLayout(milliseconds = 120) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

function items(): Item[] {
  return [
    { kind: "user", id: privateAnchorID, text: privateQuestionBody },
    ...Array.from({ length: 3 }, (_, index): Item => ({
      kind: "assistant",
      id: `diagnostic-intro-${index}`,
      text: `intro ${index}`,
      reasoning: "",
      streaming: false,
    })),
    { kind: "user", id: "diagnostic-prior-question", text: "prior question" },
    {
      kind: "tool",
      id: "diagnostic-task",
      name: "task",
      args: JSON.stringify({ description: "private tool args" }),
      readOnly: false,
      status: "done",
      output: "private tool output",
    },
    ...Array.from({ length: 32 }, (_, index): Item => ({
      kind: "assistant",
      id: `diagnostic-result-${index}`,
      text: `result ${index}`,
      reasoning: "",
      streaming: false,
    })),
  ];
}

beforeEach(() => {
  nestedListTop = 240;
  targetRowStart = 0;
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => (
    window.setTimeout(() => callback(performance.now()), 0)
  ));
  vi.stubGlobal("cancelAnimationFrame", (handle: number) => window.clearTimeout(handle));
  globalThis.ResizeObserver = MeasuredResizeObserver;
  Object.defineProperty(window, "go", {
    configurable: true,
    value: { main: { App: { ExpertTeamForTab: async () => null } } },
  });
  setPrototypeProperty("clientHeight", { get: () => 800 });
  setPrototypeProperty("scrollHeight", { get: () => 20_000 });
  setPrototypeProperty("scrollTop", {
    get() { return scrollTops.get(this as HTMLElement) ?? 0; },
    set(value) { scrollTops.set(this as HTMLElement, Number(value)); },
  });
  setPrototypeProperty("scrollTo", {
    value: function scrollTo(this: HTMLElement, options: ScrollToOptions | number, y?: number) {
      const top = typeof options === "number" ? y ?? 0 : options.top ?? 0;
      this.scrollTop = Number(top);
      this.dispatchEvent(new Event("scroll", { bubbles: true }));
    },
  });
  setPrototypeProperty("getBoundingClientRect", {
    value: function getBoundingClientRect() {
      const element = this as HTMLElement;
      const transcript = element.classList.contains("transcript")
        ? element
        : element.closest<HTMLElement>(".transcript");
      if (element.classList.contains("transcript")) return { top: 0 } as DOMRect;
      if (element.classList.contains("transcript-virtual-list")) {
        return { top: nestedListTop - (transcript?.scrollTop ?? 0) } as DOMRect;
      }
      if (element.id === questionAnchorId(privateAnchorID)) {
        return { top: nestedListTop - (transcript?.scrollTop ?? 0) + targetRowStart } as DOMRect;
      }
      return { top: 0 } as DOMRect;
    },
  });
});

afterEach(() => {
  restoreInstalledTrace?.();
  restoreInstalledTrace = undefined;
  restorePrototypeProperties();
  scrollTops.delete(document.documentElement);
  vi.unstubAllGlobals();
});

test("DEV trace correlates wheel capture and cache write without private data", async () => {
  expect(TRANSCRIPT_TRACE_ENABLED).toBe(true);
  let now = 10_000;
  const trace = createTranscriptTrace({ enabled: true, now: () => ++now, maxEntries: 1_000 });
  restoreInstalledTrace = installTranscriptTraceForTest(trace);
  let savedAnchor: SessionScrollAnchor = { atBottom: true };
  const transcriptItems = items();
  const view = render(
    <LocaleProvider>
      <Transcript
        items={transcriptItems}
        viewKey={privateViewKey}
        initialScrollAnchor={{ atBottom: true }}
        onScrollAnchorChange={(_, anchor) => { savedAnchor = anchor; }}
        onPrompt={() => {}}
        questionNavigator={false}
      />
    </LocaleProvider>,
  );
  await act(async () => waitForLayout());

  const transcript = document.querySelector<HTMLDivElement>(".transcript");
  expect(transcript).not.toBeNull();
  if (transcript) {
    await act(async () => {
      transcript.scrollTop = 1_000;
      transcript.dispatchEvent(new Event("scroll", { bubbles: true }));
      await waitForLayout();
    });
  }
  await act(async () => {
    if (!transcript) return;
    transcript.scrollTop = 3_400;
    transcript.dispatchEvent(new Event("scroll", { bubbles: true }));
    await waitForLayout();
  });
  expect(document.getElementById(questionAnchorId(privateAnchorID))).toBeNull();

  await act(async () => {
    if (!transcript) return;
    transcript.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
    transcript.scrollTop = 3_800;
    transcript.dispatchEvent(new Event("scroll", { bubbles: true }));
    window.dispatchEvent(new PointerEvent("pointerup", { bubbles: true }));
    transcript.dispatchEvent(new WheelEvent("wheel", { bubbles: true, deltaY: -1_300, deltaMode: WheelEvent.DOM_DELTA_PIXEL }));
    transcript.dispatchEvent(new WheelEvent("wheel", { bubbles: true, deltaY: -800, deltaMode: WheelEvent.DOM_DELTA_LINE }));
    transcript.scrollTop = 4_200;
    transcript.dispatchEvent(new Event("scroll", { bubbles: true }));
    await waitForLayout();
    transcript.scrollTop = 4_400;
    transcript.dispatchEvent(new Event("scroll", { bubbles: true }));
  });

  expect(savedAnchor.atBottom).toBe(false);
  view.unmount();
  const restoredView = render(
    <LocaleProvider>
      <Transcript
        items={transcriptItems}
        viewKey={privateViewKey}
        initialScrollAnchor={savedAnchor}
        onPrompt={() => {}}
        questionNavigator={false}
      />
    </LocaleProvider>,
  );
  await act(async () => waitForLayout());
  restoredView.unmount();

  const events = trace.events();
  const begin = events.find((entry) => entry.event === "wheel.begin");
  const restart = events.find((entry) => entry.event === "wheel.restart");
  const targetBatch = Number(restart?.data.batch);
  const lateScroll = events.find((entry) => entry.event === "scroll.handle"
    && entry.data.source === "programmatic"
    && entry.data.wheelActive === false
    && Number(entry.data.batch) === targetBatch
    && Number(entry.data.sinceExpireMs) >= 0);
  const semanticCapture = events.find((entry) => entry.event === "anchor.capture"
    && entry.data.found === true);
  const cacheWrite = events.find((entry) => entry.event === "anchor.cache.write"
    && entry.seq > (semanticCapture?.seq ?? Number.MAX_SAFE_INTEGER)
    && entry.data.anchor === semanticCapture?.data.anchor
    && entry.data.offset === semanticCapture?.data.offset);
  const visibleJumps = events.filter((entry) => entry.event === "viewport.visible-jump");
  const visibleJumpSummaries = events.filter((entry) => entry.event === "viewport.visible-jump.summary");

  expect(begin?.data).toMatchObject({ deltaY: -1_300, deltaMode: WheelEvent.DOM_DELTA_PIXEL });
  expect(restart?.data).toMatchObject({
    previousBatch: begin?.data.batch,
    deltaY: -800,
    deltaMode: WheelEvent.DOM_DELTA_LINE,
  });
  expect(targetBatch).toBeGreaterThan(0);
  expect(events.some((entry) => entry.event === "wheel.expire")).toBe(true);
  expect(lateScroll?.data.sinceWheelMs).toEqual(expect.any(Number));
  expect(semanticCapture?.data).toMatchObject({
    found: true,
    questionIndex: 0,
    offset: expect.any(Number),
    anchor: expect.stringMatching(/^a[0-9a-f]{8}$/),
  });
  expect(cacheWrite?.data.questionIndex).toBe(semanticCapture?.data.questionIndex);
  expect(visibleJumps).toHaveLength(0);
  expect(visibleJumpSummaries).toHaveLength(0);
  expect(trace.export()).not.toMatch(/diagnostic-private-view|diagnostic-private-question|diagnostic private question body|private tool args|private tool output/);
});

test("DEV hidden persisted restore records one semantic seek without visible jump residue", async () => {
  expect(TRANSCRIPT_TRACE_ENABLED).toBe(true);
  const trace = createTranscriptTrace({ enabled: true, maxEntries: 1_000 });
  restoreInstalledTrace = installTranscriptTraceForTest(trace);
  let resolveTeam: ((team: ExpertTeamSession | null) => void) | undefined;
  Object.defineProperty(window, "go", {
    configurable: true,
    value: {
      main: {
        App: {
          ExpertTeamForTab: () => new Promise<ExpertTeamSession | null>((resolve) => { resolveTeam = resolve; }),
        },
      },
    },
  });
  const team = {
    state: "team_active" as const,
    id: "diagnostic-persisted-team",
    name: "Diagnostic Persisted Team",
    lead: { memberKey: "lead", name: "Lead", role: "lead" as const },
    members: [] as [],
  };
  const persistedItems: Item[] = [
    ...Array.from({ length: 12 }, (_, index): Item[] => ([
      { kind: "user", id: `persisted-prior-question-${index}`, text: `prior ${index}` },
      { kind: "assistant", id: `persisted-prior-answer-${index}`, text: `answer ${index}`, reasoning: "", streaming: false },
    ])).flat(),
    ...items(),
  ];
  render(
    <LocaleProvider>
      <Transcript
        items={persistedItems}
        tabId="diagnostic-persisted-tab"
        viewKey="diagnostic-persisted-view"
        initialScrollAnchor={{ atBottom: false, anchorMessageId: privateAnchorID, offset: -12.2 }}
        onPrompt={() => {}}
        activeExpert={{ id: team.id, name: team.name, mode: "team" }}
        questionNavigator={false}
      />
    </LocaleProvider>,
  );
  await act(async () => waitForLayout());
  expect(document.querySelector('.transcript__session-loading')).not.toBeNull();
  expect(document.querySelector('.transcript__session-loading[role="status"]')).toBeNull();
  expect(trace.events().filter((entry) => entry.event === "viewport.visible-jump")).toHaveLength(0);

  await act(async () => {
    resolveTeam?.(team);
    await Promise.resolve();
  });
  await act(async () => {
    await waitForLayout(600);
  });
  const persistedJumps = trace.events().filter((entry) => entry.event === "viewport.visible-jump");
  const persistedSummaries = trace.events().filter((entry) => entry.event === "viewport.visible-jump.summary");
  const persistedSeeks = trace.events().filter((entry) => entry.event === "anchor.virtual-seek");
  expect(persistedSeeks).toHaveLength(1);
  expect(persistedJumps).toHaveLength(0);
  expect(persistedSummaries).toHaveLength(0);
  const presentation = document.querySelector<HTMLElement>('[data-testid="persisted-expert-presentation"]');
  expect(presentation?.getAttribute("aria-hidden")).toBeNull();
  expect(presentation?.style.visibility).not.toBe("hidden");
  await act(async () => waitForLayout());
  expect(trace.events().filter((entry) => entry.event === "viewport.visible-jump")).toHaveLength(0);
  expect(trace.events().filter((entry) => entry.event === "viewport.visible-jump.summary")).toHaveLength(0);
});
