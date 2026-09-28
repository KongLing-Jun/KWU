import { JSDOM } from "jsdom";
import React, { act, useRef, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { ProjectTree } from "../components/ProjectTree";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ComposerSessionIdentity, ProjectNode, Scene, SceneState } from "../lib/types";
import { loadStylesSource } from "./helpers/styles";

type OpenRecord = {
  topicId: string;
  sessionPath?: string;
  preparedOpenToken?: number;
  sessionTurns?: number;
  scene?: Scene;
  sceneState?: SceneState;
  composerIdentity?: ComposerSessionIdentity;
};

type IntentRecord = {
  topicId: string;
  sessionPath: string;
  transitionId: number;
  sessionTurns?: number;
  scene?: Scene;
  sceneState?: SceneState;
  composerIdentity?: ComposerSessionIdentity;
};

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

function count(values: readonly unknown[]): number {
  return values.length;
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await act(async () => {
      await flush();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function installFakeClock() {
  type Timer = { at: number; callback: () => void };
  const originalGlobalSetTimeout = globalThis.setTimeout;
  const originalGlobalClearTimeout = globalThis.clearTimeout;
  const originalWindowSetTimeout = window.setTimeout;
  const originalWindowClearTimeout = window.clearTimeout;
  const timers = new Map<number, Timer>();
  let now = 0;
  let nextId = 1;

  const fakeSetTimeout = ((handler: TimerHandler, timeout = 0) => {
    if (typeof handler !== "function") throw new Error("string timers are unsupported");
    const id = nextId;
    nextId += 1;
    timers.set(id, { at: now + timeout, callback: () => handler() });
    return id;
  }) as typeof window.setTimeout;
  const fakeClearTimeout = ((id?: number) => {
    if (id !== undefined) timers.delete(id);
  }) as typeof window.clearTimeout;

  globalThis.setTimeout = fakeSetTimeout as unknown as typeof globalThis.setTimeout;
  globalThis.clearTimeout = fakeClearTimeout as unknown as typeof globalThis.clearTimeout;
  window.setTimeout = fakeSetTimeout;
  window.clearTimeout = fakeClearTimeout;

  return {
    advance(ms: number) {
      const target = now + ms;
      while (true) {
        const due = [...timers.entries()]
          .filter(([, timer]) => timer.at <= target)
          .sort((left, right) => left[1].at - right[1].at || left[0] - right[0])[0];
        if (!due) break;
        const [id, timer] = due;
        timers.delete(id);
        now = timer.at;
        timer.callback();
      }
      now = target;
    },
    restore() {
      globalThis.setTimeout = originalGlobalSetTimeout;
      globalThis.clearTimeout = originalGlobalClearTimeout;
      window.setTimeout = originalWindowSetTimeout;
      window.clearTimeout = originalWindowClearTimeout;
    },
  };
}

const tree: ProjectNode[] = [
  {
    key: "topic-a",
    kind: "topic",
    label: "Existing A",
    root: "C:\\repo",
    topicId: "topic-a",
    sessionPath: "C:\\sessions\\a.jsonl",
    sessionTurns: 1,
    scene: "coding",
    sceneState: "locked",
    modelRef: "provider/model-a",
    billingSource: "newapi_byok",
    expertId: "team-a",
    expertName: "Team A",
    collaborationMode: "plan",
    toolApprovalMode: "ask",
    tokenMode: "economy",
    goalStatus: "stopped",
  },
  {
    key: "topic-b",
    kind: "topic",
    label: "Existing B",
    root: "C:\\repo",
    topicId: "topic-b",
    sessionPath: "C:\\sessions\\b.jsonl",
    sessionTurns: 1,
    scene: "design",
    sceneState: "draft",
    modelRef: "provider/model-b",
    billingSource: "newapi_byok",
    expertId: "team-b",
    expertName: "Team B",
    collaborationMode: "goal",
    toolApprovalMode: "auto",
    tokenMode: "full",
    goal: "ship B",
    goalStatus: "running",
  },
  {
    key: "topic-blank",
    kind: "topic",
    label: "新的会话",
    root: "C:\\repo",
    topicId: "topic-blank",
    sessionPath: "C:\\sessions\\blank.jsonl",
    sessionTurns: 0,
    scene: "coding",
    sceneState: "draft",
  },
  {
    key: "topic-pathless",
    kind: "topic",
    label: "Pathless Topic",
    root: "C:\\repo",
    topicId: "topic-pathless",
  },
];

const dom = new JSDOM("<!doctype html><html><body></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.Element = dom.window.Element;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
globalThis.MouseEvent = dom.window.MouseEvent;
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", {
  configurable: true,
  value: () => {},
});
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", {
  configurable: true,
  value: () => {},
});

window.go = {
  main: {
    App: {
      ListProjectTree: async () => tree,
      Platform: async () => "windows",
    } as Partial<AppBindings> as AppBindings,
  },
};

async function mountHarness({ creation = false, holdOpen = false }: { creation?: boolean; holdOpen?: boolean } = {}) {
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  const opens: OpenRecord[] = [];
  const intents: IntentRecord[] = [];
  const cancellations: number[] = [];
  const renames: Array<{ topicId: string; title: string }> = [];
  const openGate = deferred<void>();
  let nextTransitionId = 0;

  function Harness() {
    const [layout, setLayout] = useState<"center" | "bottom">("center");
    const [presentationScene, setPresentationScene] = useState<Scene>("office");
    const transitionRef = useRef<number | undefined>(undefined);
    return (
      <LocaleProvider>
        <ToastProvider>
          <div data-testid="composer-layout" data-layout={layout} />
          <div data-testid="presentation-scene" data-scene={presentationScene} />
          <ProjectTree
            activeScope={creation ? "project" : undefined}
            activeWorkspaceRoot={creation ? "C:\\repo" : undefined}
            activeTopicId={creation ? "topic-blank" : undefined}
            activeSessionPath={creation ? "C:\\sessions\\blank.jsonl" : undefined}
            variant={creation ? "creation" : "classic"}
            onBeginOpenTopicIntent={(_scope, _workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity) => {
              nextTransitionId += 1;
              if (sessionPath?.trim()) {
                transitionRef.current = nextTransitionId;
                intents.push({ topicId, sessionPath, transitionId: nextTransitionId, sessionTurns, scene, sceneState, composerIdentity });
                setLayout(sessionTurns === 0 ? "center" : "bottom");
                setPresentationScene(scene ?? "office");
              }
              return nextTransitionId;
            }}
            onCancelOpenTopicIntent={(transitionId) => {
              cancellations.push(transitionId);
              if (transitionRef.current !== transitionId) return;
              transitionRef.current = undefined;
              setLayout("center");
            }}
            onOpenTopic={async (_scope, _workspaceRoot, topicId, sessionPath, preparedOpenToken, sessionTurns, scene, sceneState, composerIdentity) => {
              opens.push({ topicId, sessionPath, preparedOpenToken, sessionTurns, scene, sceneState, composerIdentity });
              if (holdOpen) await openGate.promise;
            }}
            onOpenProjectHistory={() => {}}
            onAddProject={async () => {}}
            onRenameTopic={(topicId, title) => {
              renames.push({ topicId, title });
            }}
            timeFilter="all"
            onTimeFilterChange={() => {}}
          />
        </ToastProvider>
      </LocaleProvider>
    );
  }

  await act(async () => {
    root.render(<Harness />);
    await flush();
  });
  await waitFor(
    "project tree topics",
    () => host.querySelectorAll(".project-tree__topic-main").length === (creation ? 3 : 4),
  );

  return {
    host,
    root,
    opens,
    intents,
    cancellations,
    renames,
    resolveOpen() {
      openGate.resolve();
    },
    rejectOpen(reason: unknown) {
      openGate.reject(reason);
    },
    topic(label: string) {
      const button = [...host.querySelectorAll<HTMLButtonElement>(".project-tree__topic-main")]
        .find((candidate) => candidate.textContent?.includes(label));
      ok(button, `missing topic button ${label}`);
      return button;
    },
    layout() {
      return host.querySelector("[data-testid=\"composer-layout\"]")?.getAttribute("data-layout");
    },
    presentationScene() {
      return host.querySelector("[data-testid=\"presentation-scene\"]")?.getAttribute("data-scene");
    },
  };
}

async function unmount(root: Root, host: HTMLElement) {
  await act(async () => root.unmount());
  host.remove();
}

{
  const harness = await mountHarness();
  const clock = installFakeClock();
  await act(async () => harness.topic("Existing A").click());
  ok(harness.topic("Existing A").getAttribute("aria-busy") === "true", "single click immediately exposes pending semantics on the selected row");
  ok(Boolean(harness.topic("Existing A").closest(".project-tree__topic--pending")), "single click immediately paints the selected row as pending");
  ok(harness.layout() === "bottom", "path topic click must synchronously render bottom composer intent");
  ok(harness.presentationScene() === "coding", "path topic click must synchronously prepare its persisted Coding scene");
  ok(count(harness.intents) === 1, "path topic click must synchronously prepare one existing-session transition");
  ok(harness.intents[0]?.composerIdentity?.modelRef === "provider/model-a", "first click frame must carry the target model");
  ok(harness.intents[0]?.composerIdentity?.expertId === "team-a", "first click frame must carry the target expert team");
  ok(harness.intents[0]?.composerIdentity?.collaborationMode === "plan", "first click frame must carry the target collaboration mode");
  ok(count(harness.opens) === 0, "backend open must remain delayed after the first click event");

  await act(async () => clock.advance(199));
  ok(harness.topic("Existing A").getAttribute("aria-busy") === "true", "pending row feedback remains visible during click disambiguation");
  ok(harness.layout() === "bottom", "0-199ms must never restore the centered composer");
  ok(count(harness.opens) === 0, "backend open must not run before 200ms");

  await act(async () => clock.advance(1));
  ok(count(harness.opens) === 1, "backend open must run at 200ms");
  ok(harness.topic("Existing A").getAttribute("aria-busy") === null, "a synchronously settled open clears pending row feedback");
  ok(
    harness.opens[0]?.preparedOpenToken === harness.intents[0]?.transitionId,
    "delayed backend activation must reuse the synchronously prepared transition",
  );
  ok(harness.opens[0]?.scene === "coding" && harness.opens[0]?.sceneState === "locked", "200ms activation retains the prepared scene summary");
  ok(harness.opens[0]?.composerIdentity === harness.intents[0]?.composerIdentity, "backend activation must reuse the exact prepared Composer identity");
  clock.restore();
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness({ creation: true });
  ok(!harness.host.textContent?.includes("新的会话"), "creation must hide the zero-turn default-title topic row");
  ok(harness.host.textContent?.includes("Existing A"), "creation must keep existing topic rows visible");

  await act(async () => {
    harness.topic("Existing A").dispatchEvent(new dom.window.MouseEvent("dblclick", { bubbles: true }));
  });
  const dialog = document.querySelector<HTMLFormElement>(".rename-topic-modal");
  ok(dialog, "creation double-click must open the rename dialog");
  const input = dialog.querySelector<HTMLInputElement>(".rename-topic-modal__input");
  ok(input?.value === "Existing A", "rename dialog must start with the current topic title");
  await act(async () => {
    dialog.dispatchEvent(new dom.window.Event("submit", { bubbles: true, cancelable: true }));
    await flush();
  });
  ok(count(harness.renames) === 1, "rename dialog save must submit exactly once");
  ok(harness.renames[0]?.title === "Existing A", "rename dialog must submit its current title");
  await act(async () => {
    harness.topic("Existing A").dispatchEvent(new dom.window.MouseEvent("dblclick", { bubbles: true }));
  });
  const reopenedDialog = document.querySelector<HTMLFormElement>(".rename-topic-modal");
  ok(reopenedDialog, "creation double-click must reopen the rename dialog");
  await act(async () => {
    reopenedDialog.dispatchEvent(new dom.window.KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  });
  ok(!document.querySelector(".rename-topic-modal"), "Escape must close the rename dialog");
  ok(count(harness.renames) === 1, "Escape must not submit another rename");
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness();
  const clock = installFakeClock();
  await act(async () => harness.topic("Existing A").click());
  ok(harness.layout() === "bottom", "first click must prepare existing-session intent before rename detection");
  await act(async () => harness.topic("Existing A").click());
  ok(harness.topic("Existing A").getAttribute("aria-busy") === null, "rename disambiguation immediately clears pending row feedback");
  ok(harness.layout() === "center", "same-row second click must cancel intent and restore the blank source");
  await act(async () => {
    harness.topic("Existing A").dispatchEvent(new dom.window.MouseEvent("dblclick", { bubbles: true }));
  });
  ok(Boolean(harness.host.querySelector(".project-tree__topic--editing")), "double-click must still enter inline rename");
  await act(async () => clock.advance(200));
  ok(count(harness.opens) === 0, "rename double-click must cancel the delayed backend open");
  ok(count(harness.cancellations) === 1, "rename double-click sequence must cancel exactly one prepared transition");
  clock.restore();
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness();
  const clock = installFakeClock();
  await act(async () => harness.topic("Existing A").click());
  await act(async () => harness.topic("Existing B").click());
  ok(harness.topic("Existing A").getAttribute("aria-busy") === null, "quick target replacement clears the superseded row feedback");
  ok(harness.topic("Existing B").getAttribute("aria-busy") === "true", "quick target replacement keeps feedback only on the latest row");
  ok(harness.layout() === "bottom", "quick different-target click must keep existing-session intent");
  ok(harness.presentationScene() === "design", "quick different-target click must replace the prepared scene immediately");
  ok(count(harness.intents) === 2, "quick different-target click must prepare the latest target");
  ok(harness.intents[1]?.composerIdentity?.modelRef === "provider/model-b", "quick target replacement must synchronously replace the model identity");
  ok(harness.intents[1]?.composerIdentity?.expertId === "team-b", "quick target replacement must synchronously replace the expert identity");
  ok(harness.intents[1]?.composerIdentity?.collaborationMode === "goal", "quick target replacement must synchronously replace the collaboration mode");
  ok(harness.cancellations[0] === harness.intents[0]?.transitionId, "quick different-target click must cancel the old intent");
  await act(async () => clock.advance(200));
  ok(count(harness.opens) === 1 && harness.opens[0]?.topicId === "topic-b", "only the latest quick-click target may activate");
  ok(
    harness.opens[0]?.preparedOpenToken === harness.intents[1]?.transitionId,
    "latest backend activation must retain the latest synchronous transition",
  );
  clock.restore();
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness({ holdOpen: true });
  const clock = installFakeClock();
  await act(async () => harness.topic("Existing A").click());
  await act(async () => clock.advance(200));
  ok(count(harness.opens) === 1, "held navigation starts at the unchanged 200ms boundary");
  ok(harness.topic("Existing A").getAttribute("aria-busy") === "true", "pending feedback remains while asynchronous navigation is unsettled");
  await act(async () => {
    harness.resolveOpen();
    await Promise.resolve();
  });
  ok(harness.topic("Existing A").getAttribute("aria-busy") === null, "asynchronous navigation settlement clears pending feedback");
  clock.restore();
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness({ holdOpen: true });
  const clock = installFakeClock();
  await act(async () => harness.topic("Existing A").click());
  await act(async () => clock.advance(200));
  ok(harness.topic("Existing A").getAttribute("aria-busy") === "true", "failed navigation remains pending until its asynchronous result settles");
  await act(async () => {
    harness.rejectOpen(new Error("open failed"));
    await Promise.resolve();
  });
  ok(harness.topic("Existing A").getAttribute("aria-busy") === null, "asynchronous navigation failure clears pending feedback without an unhandled rejection");
  clock.restore();
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness();
  const clock = installFakeClock();
  await act(async () => harness.topic("新的会话").click());
  ok(harness.layout() === "center", "zero-turn persisted topic must synchronously retain the centered composer");
  ok(count(harness.intents) === 1 && harness.intents[0]?.sessionTurns === 0, "zero-turn persisted topic must prepare explicit blank intent");
  ok(harness.presentationScene() === "coding", "zero-turn persisted topic must expose its saved scene on the first click frame");
  await act(async () => clock.advance(200));
  ok(
    count(harness.opens) === 1
      && harness.opens[0]?.topicId === "topic-blank"
      && harness.opens[0]?.sessionTurns === 0
      && harness.opens[0]?.scene === "coding",
    "zero-turn persisted topic activates with exact blank and scene evidence after click delay",
  );
  clock.restore();
  await unmount(harness.root, harness.host);
}

{
  const harness = await mountHarness();
  const clock = installFakeClock();
  await act(async () => harness.topic("Pathless Topic").click());
  ok(harness.layout() === "center", "pathless topic must retain the centered composer");
  ok(count(harness.intents) === 0, "pathless topic must not prepare a path transition");
  await act(async () => clock.advance(200));
  ok(count(harness.opens) === 1 && harness.opens[0]?.topicId === "topic-pathless", "pathless topic still activates after click delay");
  ok(harness.opens[0]?.preparedOpenToken === 1, "pathless topic must carry an opaque prepared token through the click delay");
  clock.restore();
  await unmount(harness.root, harness.host);
}

const styleSource = loadStylesSource();
ok(
  /@media \(prefers-reduced-motion: no-preference\) \{[\s\S]*?\.project-tree__topic-pending \{[\s\S]*?animation: spin/.test(styleSource),
  "pending row animation only runs when reduced motion is not requested",
);
ok(
  /\.project-tree__topic-pending \{[\s\S]*?position: absolute[\s\S]*?pointer-events: none/.test(styleSource),
  "pending row feedback does not participate in layout or intercept interaction",
);

dom.window.close();
console.log("project tree immediate navigation intent passed");
