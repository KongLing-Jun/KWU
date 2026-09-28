// Run: tsx src/__tests__/existing-session-loading-presentation.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import { blankSessionRequestScenePresentation } from "../lib/scenePresentation";
import type { Scene, TabMeta } from "../lib/types";
import { loadStylesSource } from "./helpers/styles";

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

function installWindowFakeClock() {
  const originalSetTimeout = window.setTimeout.bind(window);
  const originalClearTimeout = window.clearTimeout.bind(window);
  let now = 0;
  let nextId = 1;
  const timers = new Map<number, { at: number; callback: () => void }>();
  window.setTimeout = ((handler: TimerHandler, timeout = 0, ...args: unknown[]) => {
    const id = nextId;
    nextId += 1;
    timers.set(id, {
      at: now + Number(timeout || 0),
      callback: () => {
        if (typeof handler === "function") handler(...args);
      },
    });
    return id;
  }) as typeof window.setTimeout;
  window.clearTimeout = ((id: number) => {
    timers.delete(id);
  }) as typeof window.clearTimeout;
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
      window.setTimeout = originalSetTimeout;
      window.clearTimeout = originalClearTimeout;
    },
  };
}

console.log("\nexisting session loading presentation");

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
ok(
  /const blankSessionRequestPending = pendingBlankSessionRequest !== null;/.test(appSource)
    && /const blankSessionPresentation = state\.surfaceIntent === "blank"[\s\S]*?\|\| blankSessionRequestPending;/.test(appSource)
    && /const blankSessionHistoryPending = blankSessionRequestPending[\s\S]*?blankSessionPresentation && state\.hydrateHistoryLoaded !== true/.test(appSource)
    && /const sessionHasContent = !blankSessionHistoryPending[\s\S]*?state\.items\.length > 0/.test(appSource)
    && /const transcriptItems = blankSessionHistoryPending[\s\S]*?\? \[\][\s\S]*?: hydratePlaceholderActive/.test(appSource)
    && /const existingSessionSubmitBlocked = blankSessionHistoryPending[\s\S]*?\|\| \(nonBlankSessionPresentation/.test(appSource),
  "new blank intent hides the previous transcript before backend activation and history hydration",
);

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
const selectedScenes: string[] = [];
const reusableBlankTab = {
  id: "blank-design",
  scope: "global",
  workspaceRoot: "",
  sessionTurns: 0,
  scene: "design",
  sceneState: "draft",
  running: false,
  pendingPrompt: false,
  backgroundJobs: 0,
} satisfies Pick<TabMeta, "id" | "scope" | "workspaceRoot" | "sessionTurns" | "scene" | "sceneState" | "running" | "pendingPrompt" | "backgroundJobs">;
const repeatedBlankPreview = blankSessionRequestScenePresentation({
  targetScope: "global",
  targetWorkspaceRoot: "",
  activeTabId: reusableBlankTab.id,
  activeTab: reusableBlankTab,
  visibleScene: reusableBlankTab.scene,
  surfaceIntent: "blank",
  surfaceTransitionActive: false,
  hasVisibleContent: false,
  allowCurrentBlankSceneReuse: true,
});
ok(repeatedBlankPreview.scene === "design", "repeating New on the reusable blank tab preserves its Design preview");
ok(repeatedBlankPreview.sceneState === "draft", "repeating New preserves the reusable blank scene state");
ok(
  blankSessionRequestScenePresentation({
    targetScope: "global",
    targetWorkspaceRoot: "",
    activeTabId: reusableBlankTab.id,
    activeTab: { ...reusableBlankTab, scene: "coding" },
    visibleScene: "coding",
    surfaceIntent: "blank",
    surfaceTransitionActive: false,
    hasVisibleContent: false,
    allowCurrentBlankSceneReuse: true,
  }).scene === "coding",
  "repeating New preserves a reusable Coding preview",
);
for (const locallyDisplayedScene of ["design", "coding"] as const) {
  ok(
    blankSessionRequestScenePresentation({
      targetScope: "global",
      targetWorkspaceRoot: "",
      activeTabId: reusableBlankTab.id,
      activeTab: { ...reusableBlankTab, scene: "office" },
      visibleScene: locallyDisplayedScene,
      surfaceIntent: "blank",
      surfaceTransitionActive: false,
      hasVisibleContent: false,
      allowCurrentBlankSceneReuse: true,
    }).scene === locallyDisplayedScene,
    `repeating New preserves the locally displayed ${locallyDisplayedScene} draft before controller readiness`,
  );
}
ok(
  blankSessionRequestScenePresentation({
    targetScope: "global",
    targetWorkspaceRoot: "",
    activeTabId: reusableBlankTab.id,
    activeTab: reusableBlankTab,
    visibleScene: "design",
    surfaceIntent: "existing-session",
    surfaceTransitionActive: false,
    hasVisibleContent: true,
    allowCurrentBlankSceneReuse: true,
  }).scene === "office",
  "opening New from an existing conversation still previews Office",
);
for (const [label, overrides] of [
  ["stale tab identity", { activeTabId: "newer-tab" }],
  ["different scope", { targetScope: "project", targetWorkspaceRoot: "D:\\other" }],
  ["unknown turns", { activeTab: { ...reusableBlankTab, sessionTurns: undefined } }],
  ["active transition", { surfaceTransitionActive: true }],
  ["expert or project-tree request", { allowCurrentBlankSceneReuse: false }],
] as const) {
  ok(
    blankSessionRequestScenePresentation({
      targetScope: "global",
      targetWorkspaceRoot: "",
      activeTabId: reusableBlankTab.id,
      activeTab: reusableBlankTab,
      visibleScene: "coding",
      surfaceIntent: "blank",
      surfaceTransitionActive: false,
      hasVisibleContent: false,
      allowCurrentBlankSceneReuse: true,
      ...overrides,
    }).scene === "office",
    `${label} cannot leak the previous scene into a truly new blank preview`,
  );
}

const loadingClock = installWindowFakeClock();
await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript items={[]} hydrating existingSessionLoading onPrompt={() => {}} />
    </LocaleProvider>,
  );
});
ok(document.querySelector(".welcome") === null, "existing-session hydration suppresses Welcome during the quiet window");
ok(document.querySelector(".transcript__session-loading") !== null, "quiet window immediately mounts the stable loading protection surface");
ok(document.querySelector("[role=\"status\"]") === null, "quiet window mounts no loading live region");
ok(!document.body.textContent?.includes("正在加载会话…"), "quiet window shows no loading label");
ok(document.querySelector(".transcript__session-placeholder") === null, "quiet window never mounts transcript-shaped placeholder cards");

await act(async () => {
  loadingClock.advance(179);
});
ok(document.querySelector("[role=\"status\"]") === null, "179ms remains silent");
ok(!document.body.textContent?.includes("正在加载会话…"), "179ms still shows no loading label");

await act(async () => {
  loadingClock.advance(1);
});
const loadingStatus = document.querySelector(".transcript__session-loading");
ok(loadingStatus?.textContent === "正在加载会话…", "slow existing-session hydration renders the localized history loading message");
ok(loadingStatus?.getAttribute("role") === "status" && loadingStatus.getAttribute("aria-live") === "polite", "loading message uses polite status semantics");
ok(document.querySelectorAll("[role=\"status\"]").length === 1, "history loading presentation owns the only live status in its surface");
ok(document.querySelector(".transcript__session-placeholder") === null, "slow loading still uses no transcript-shaped placeholder cards");
ok(document.querySelector(".welcome") === null, "loading message never reveals Welcome");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[{ kind: "user", id: "loaded-user", text: "loaded history" }]}
        onPrompt={() => {}}
      />
    </LocaleProvider>,
  );
});
ok(document.querySelector(".transcript__session-loading") === null, "loaded history atomically replaces the loading message");
ok(document.body.textContent?.includes("loaded history") ?? false, "loaded transcript content is visible after hydration");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[]}
        hydrating
        showWelcomeDuringHydration
        welcomeVariant="creation"
        onPrompt={() => {}}
        scene="office"
        sceneState="locked"
        sceneSelectionAvailability="disabled"
        onSelectScene={async (scene) => {
          selectedScenes.push(scene);
        }}
      />
    </LocaleProvider>,
  );
});
const blankHydrationWelcome = document.querySelector(".welcome");
ok(blankHydrationWelcome !== null, "blank hydration first frame mounts the complete Welcome surface");
ok(document.querySelector(".transcript__session-loading") === null, "blank hydration never mounts existing-session loading feedback");
const blankHydrationModes = document.querySelector(".welcome-creation__modes");
const blankHydrationScenarios = document.querySelector(".welcome-creation__scenarios");
ok(document.querySelector(".welcome__brand") !== null, "blank first frame includes the brand");
ok(document.querySelector(".welcome-creation__headline") !== null, "blank first frame includes the creation title");
ok(blankHydrationModes !== null, "blank first frame reserves the creation mode row before TabMeta is seeded");
ok(blankHydrationScenarios !== null, "blank first frame includes creation scenarios");
ok(document.querySelector(".welcome-creation__mode--active")?.textContent?.includes("日常办公") === true, "explicit blank first frame selects Office");
ok(document.querySelector(".welcome-creation__headline")?.textContent === "你的办公超能力", "explicit blank first frame shows the Office headline");
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__mode")).every((button) => button.disabled),
  "blank first-frame mode controls cannot write through stale tab metadata",
);
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__scenario")).every((button) => button.disabled),
  "blank first-frame secondary scenarios cannot write through stale tab metadata",
);

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[]}
        welcomeVariant="creation"
        onPrompt={() => {}}
        scene="office"
        sceneState="draft"
        sceneSelectionAvailability="enabled"
        onSelectScene={async (scene) => {
          selectedScenes.push(scene);
        }}
      />
    </LocaleProvider>,
  );
});
ok(document.querySelector(".welcome") === blankHydrationWelcome, "empty history completion preserves the same Welcome node without remounting");
ok(document.querySelector(".welcome-creation__modes") === blankHydrationModes, "TabMeta seed enables the existing mode row without inserting a node");
ok(document.querySelector(".welcome-creation__scenarios") === blankHydrationScenarios, "TabMeta seed preserves the existing scenario node");
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__mode")).every((button) => !button.disabled),
  "seeded blank TabMeta enables scene selection on the new tab",
);
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__scenario")).every((button) => !button.disabled),
  "seeded blank TabMeta enables the existing secondary scenarios",
);
await act(async () => {
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__mode"))
    .find((button) => button.textContent?.includes("代码开发"))
    ?.click();
});
ok(selectedScenes.at(-1) === "coding", "enabled mode selection targets the seeded blank tab");

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[]}
        welcomeVariant="creation"
        onPrompt={() => {}}
        scene="design"
        sceneState="draft"
        sceneSelectionAvailability="disabled"
        onSelectScene={async (scene) => {
          selectedScenes.push(scene);
        }}
      />
    </LocaleProvider>,
  );
});
const zeroTurnTopicModes = document.querySelector(".welcome-creation__modes");
const zeroTurnTopicScenarios = document.querySelector(".welcome-creation__scenarios");
ok(zeroTurnTopicModes !== null, "zero-turn topic first frame reserves the creation mode row");
ok(
  document.querySelector(".welcome-creation__mode--active")?.textContent?.includes("设计创意") === true,
  "zero-turn topic first frame selects its persisted Design mode",
);
ok(
  document.querySelector(".welcome-creation__headline")?.textContent === "你的设计超能力"
    && document.querySelector(".welcome-creation__headline")?.getAttribute("aria-hidden") !== "true"
    && zeroTurnTopicScenarios?.getAttribute("aria-hidden") !== "true",
  "zero-turn topic first frame shows its persisted headline and secondary scenarios",
);
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__mode")).every((button) => button.disabled),
  "zero-turn topic first-frame modes cannot write through the previous tab",
);
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__scenario")).every((button) => button.disabled),
  "zero-turn topic first-frame secondary scenarios cannot write through the previous tab",
);

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[]}
        welcomeVariant="creation"
        onPrompt={() => {}}
        scene="design"
        sceneState="draft"
        sceneSelectionAvailability="enabled"
        onSelectScene={async (scene) => {
          selectedScenes.push(scene);
        }}
      />
    </LocaleProvider>,
  );
});
ok(document.querySelector(".welcome-creation__modes") === zeroTurnTopicModes, "zero-turn topic metadata enables the existing mode row");
ok(document.querySelector(".welcome-creation__scenarios") === zeroTurnTopicScenarios, "zero-turn topic metadata enables the existing scenario region");
ok(
  document.querySelector(".welcome-creation__mode--active")?.textContent?.includes("设计创意") === true,
  "zero-turn topic metadata keeps its saved non-office scene without a selection flash",
);
ok(
  document.querySelector(".welcome-creation__headline")?.getAttribute("aria-hidden") !== "true"
    && zeroTurnTopicScenarios?.getAttribute("aria-hidden") !== "true",
  "zero-turn topic metadata preserves scene-specific copy in place",
);
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>(".welcome-creation__scenario")).every((button) => !button.disabled),
  "zero-turn topic metadata enables secondary scenarios in place",
);

await act(async () => {
  root.render(
    <LocaleProvider>
      <Transcript
        items={[]}
        welcomeVariant="creation"
        onPrompt={() => {}}
        scene={"broken" as Scene}
        sceneState="legacy"
        onSelectScene={async (scene) => {
          selectedScenes.push(scene);
        }}
      />
    </LocaleProvider>,
  );
});
ok(document.querySelector(".welcome-creation__mode--active")?.textContent?.includes("日常办公") === true, "invalid legacy scene falls back to Office presentation");
ok(document.querySelector(".welcome-creation__headline")?.textContent === "你的办公超能力", "legacy fallback shows the complete Office headline");
ok(document.querySelector(".welcome-creation__scenarios") !== null, "legacy fallback shows Office secondary scenarios");

const styles = loadStylesSource();
ok(
  /@media \(prefers-reduced-motion: no-preference\) \{[\s\S]*?\.transcript__session-loading svg \{[\s\S]*?animation: spin/.test(styles),
  "continuous loading rotation only runs when reduced motion is not requested",
);
ok(
  !styles.includes("transcript-session-placeholder-shimmer"),
  "placeholder shimmer is removed rather than maintaining a second loading animation",
);
ok(
  !styles.includes("transcript__session-placeholder"),
  "placeholder card styles are removed",
);

await act(async () => root.unmount());
loadingClock.restore();
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
