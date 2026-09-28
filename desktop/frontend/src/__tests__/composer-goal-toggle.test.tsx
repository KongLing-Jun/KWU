import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/composer-goal-toggle.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { Composer } from "../components/Composer";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { AppBindings } from "../lib/bridge";
import type { CollaborationMode, ToolApprovalMode, TokenMode } from "../lib/types";

let passed = 0;
let failed = 0;
const here = dirname(fileURLToPath(import.meta.url));
const styles = loadStylesSource();

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
  if (actual === expected) ok(true, label);
  else ok(false, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function flushTimers(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

function installDom() {
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
  globalThis.HTMLTextAreaElement = dom.window.HTMLTextAreaElement;
  globalThis.Event = dom.window.Event;
  globalThis.CustomEvent = dom.window.CustomEvent;
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.InputEvent = dom.window.InputEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.PointerEvent = dom.window.MouseEvent as unknown as typeof PointerEvent;
  globalThis.MutationObserver = dom.window.MutationObserver;
  globalThis.File = dom.window.File;
  globalThis.FileReader = dom.window.FileReader;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  globalThis.ResizeObserver = TestResizeObserver;
  Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
  Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: () => ({
      matches: true,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }),
  });
  return dom;
}

function setReducedMotion(matches: boolean) {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: () => ({
      matches,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }),
  });
}

async function renderComposer(props: Partial<Parameters<typeof Composer>[0]> = {}) {
  if (!window.go?.main?.App) mockApp({});
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const calls: {
    send: string[];
    setCollaborationMode: CollaborationMode[];
  } = {
    send: [],
    setCollaborationMode: [],
  };
  let currentProps: Parameters<typeof Composer>[0] = {
    running: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask" as ToolApprovalMode,
    tokenMode: "full" as TokenMode,
    goal: "",
    cwd: "/repo",
    modelLabel: "DeepSeek-R1",
    onSend: (displayText) => {
      calls.send.push(displayText);
    },
    onCancel: () => undefined,
    onCycleMode: () => {},
    onSetMode: () => {},
    onSetCollaborationMode: (mode) => calls.setCollaborationMode.push(mode),
    onSetToolApprovalMode: () => {},
    onToggleYoloApprovalMode: () => {},
    onClearGoal: () => {},
    onSwitchModel: () => {},
    onSetEffort: () => {},
    onSetTokenMode: () => {},
    ready: true,
    ...props,
    modelCatalog: props.modelCatalog ?? { state: "ready", routes: [] },
  };
  const paint = async (nextProps: Partial<Parameters<typeof Composer>[0]> = {}) => {
    currentProps = { ...currentProps, ...nextProps };
    await act(async () => {
      root.render(
        <LocaleProvider>
          <ToastProvider>
            <Composer {...currentProps} />
          </ToastProvider>
        </LocaleProvider>,
      );
      await flushTimers();
    });
  };
  await paint();
  return { root, calls, rerender: paint };
}

function mockApp(methods: Partial<AppBindings>) {
  window.go = {
    main: {
      App: {
        Commands: async () => [],
        Models: async () => [],
        ModelsForTab: async () => [],
        ...methods,
      } as Partial<AppBindings> as AppBindings,
    },
  };
}

function cssBlock(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return styles.match(new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`))?.[1] ?? "";
}

function dispatchPasteFile(editor: HTMLElement, file: File) {
  const event = new Event("paste", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "clipboardData", {
    configurable: true,
    value: {
      files: [file],
      items: [],
      types: ["Files"],
      getData: () => "",
    },
  });
  editor.dispatchEvent(event);
}

function composerEditor(): HTMLDivElement {
  const editor = document.querySelector<HTMLDivElement>("#composer-input");
  if (!editor) throw new Error("composer editor did not render");
  return editor;
}

function composerEditorValue(editor: HTMLElement): string {
  return editor.textContent ?? "";
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await act(async () => {
      await flushTimers();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

console.log("\ncomposer goal toggle");

{
  const dom = installDom();
  mockApp({});
  const { root } = await renderComposer({
    onOpenExpertDirectory: () => {},
  });

  const intentTrigger = document.querySelector<HTMLButtonElement>(".composer-meta__control--intent .composer-action-trigger");
  ok(
    Boolean(intentTrigger?.querySelector(".lucide-plus")),
    "idle composer shows one unified add entry in the lower-left composer controls",
  );
  const composerMeta = document.querySelector<HTMLElement>(".composer-meta");
  ok(
    composerMeta?.classList.contains("composer-meta--no-intent-chip") === true,
    "idle composer does not reserve status-chip space before a mode or expert is selected",
  );
  await act(async () => {
    intentTrigger?.click();
    await flushTimers();
  });
  ok(
    document.querySelector(".composer-add-menu")?.textContent?.includes("召唤专家") === true,
    "unified add menu preserves the summon expert action",
  );

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  mockApp({});
  let selectedExpertId = "";
  let directoryOpenCount = 0;
  const { root } = await renderComposer({
    recentExperts: [{ id: "testing", name: "测试专家", role: "测试", mode: "expert", prompt: "" }],
    recentExpertsLabel: "项目预选专家",
    recentExpertsLimit: 1,
    onSelectRecentExpert: (selection) => {
      selectedExpertId = selection.id;
    },
    onOpenExpertDirectory: () => {
      directoryOpenCount += 1;
    },
  });

  const intentTrigger = document.querySelector<HTMLButtonElement>(".composer-meta__control--intent .composer-action-trigger");
  await act(async () => {
    intentTrigger?.click();
    await flushTimers();
  });
  const expertSection = [...document.querySelectorAll<HTMLButtonElement>(".composer-add-menu__nav-item")]
    .find((button) => button.textContent?.includes("召唤专家"));
  await act(async () => {
    expertSection?.click();
    await flushTimers();
  });

  eq(document.querySelector(".composer-recent-expert-menu")?.getAttribute("aria-label"), "项目预选专家", "project conversation labels the bound expert list");
  const projectExpert = document.querySelector<HTMLButtonElement>(".composer-recent-expert-menu__item");
  ok(projectExpert?.textContent?.includes("测试专家") === true, "project conversation exposes its newly bound expert in place");
  await act(async () => {
    projectExpert?.click();
    await flushTimers();
  });
  eq(selectedExpertId, "testing", "selecting a project expert invokes the in-place selection callback");
  eq(directoryOpenCount, 0, "selecting a bound project expert does not navigate to the expert directory");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

for (const scenario of [
  {
    label: "plan",
    props: { collaborationMode: "plan" as CollaborationMode },
    selector: ".composer-mode-chip--plan",
    expectedCall: "normal",
  },
  {
    label: "goal",
    props: { collaborationMode: "goal" as CollaborationMode, goal: "Ship the fix" },
    selector: ".composer-mode-chip--goal",
    expectedCall: "clear-goal",
  },
  {
    label: "token",
    props: { tokenMode: "economy" as TokenMode },
    selector: ".composer-mode-chip--token",
    expectedCall: "full",
  },
]) {
  const dom = installDom();
  setReducedMotion(false);
  const modeCalls: string[] = [];
  const { root } = await renderComposer({
    ...scenario.props,
    onSetCollaborationMode: (mode) => modeCalls.push(mode),
    onClearGoal: () => modeCalls.push("clear-goal"),
    onSetTokenMode: (mode) => modeCalls.push(mode),
  });

  const modeChip = document.querySelector<HTMLButtonElement>(scenario.selector);
  if (!modeChip) throw new Error(`${scenario.label} mode chip did not render`);

  await act(async () => {
    modeChip.click();
  });
  ok(
    document.querySelector(".composer-add-menu") === null,
    `${scenario.label} chip cancellation does not mount the closed intent menu in the current frame`,
  );
  eq(modeCalls.join(","), scenario.expectedCall, `${scenario.label} chip cancellation invokes its callback immediately`);

  await act(async () => {
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
  });
  ok(
    document.querySelector(".composer-add-menu") === null,
    `${scenario.label} chip cancellation keeps the intent menu unmounted after the next animation frame`,
  );

  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 180));
  });
  ok(
    document.querySelector(".composer-add-menu") === null,
    `${scenario.label} chip cancellation keeps the intent menu unmounted after the close duration`,
  );

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  setReducedMotion(false);
  const { root } = await renderComposer();
  const intentTrigger = document.querySelector<HTMLButtonElement>(".composer-action-trigger--add");
  if (!intentTrigger) throw new Error("composer intent trigger did not render");

  await act(async () => {
    intentTrigger.click();
    await flushTimers();
  });
  const planOption = document.querySelector<HTMLButtonElement>(".composer-intent-menu__item");
  if (!planOption) throw new Error("plan mode option did not render");
  await act(async () => {
    planOption.click();
  });

  eq(
    document.querySelector(".composer-add-menu")?.getAttribute("data-state"),
    "closing",
    "choosing a collaboration mode from the open menu preserves its closing phase",
  );
  await act(async () => {
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
  });
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 180));
  });
  ok(document.querySelector(".composer-add-menu") === null, "open collaboration menu unmounts after its close duration");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  setReducedMotion(false);
  mockApp({});
  let clearExpertCount = 0;
  const { root } = await renderComposer({
    activeExpert: {
      id: "finance-team",
      name: "Finance Team",
      role: "Operating analysis",
      mode: "team",
      prompt: "Act as the finance expert team.",
    },
    onOpenExpertDirectory: () => {},
    onClearExpert: () => {
      clearExpertCount += 1;
    },
  });

  const intentTrigger = document.querySelector<HTMLButtonElement>(".composer-action-trigger--expert");
  ok(intentTrigger?.textContent?.includes("Finance Team") === true, "active expert still renders in the composer intent trigger");

  await act(async () => {
    intentTrigger?.click();
    await flushTimers();
  });

  eq(intentTrigger?.getAttribute("aria-expanded"), "true", "active expert trigger keeps the unified menu expanded after its opening click");
  eq(
    document.querySelector(".composer-add-menu")?.getAttribute("data-state"),
    "open",
    "active expert trigger leaves the unified menu in its open phase",
  );
  ok(Boolean(document.querySelector(".composer-recent-expert-menu")), "active expert trigger opens the recent expert panel");
  ok(
    document.querySelector(".composer-recent-expert-menu__item--active")?.textContent?.includes("Finance Team") === true,
    "recent expert panel keeps the active expert selected",
  );

  await act(async () => {
    intentTrigger?.click();
    await flushTimers();
  });
  await act(async () => {
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
  });
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 180));
  });

  eq(intentTrigger?.getAttribute("aria-expanded"), "false", "clicking the active expert trigger again collapses the unified menu");
  ok(document.querySelector(".composer-add-menu") === null, "active expert trigger completes the existing close animation");

  const clearExpertButton = document.querySelector<HTMLButtonElement>(".composer-action-trigger__clear");
  ok(Boolean(clearExpertButton), "active expert trigger exposes a direct clear button");
  ok(
    /border-radius:\s*999px/.test(cssBlock(".composer-action-trigger__clear")),
    "active expert clear button is styled as an independent round control",
  );
  ok(
    /gap:\s*4px/.test(cssBlock(".composer-action-trigger-shell--expert")),
    "active expert trigger leaves a visible gap before the clear button",
  );

  await act(async () => {
    clearExpertButton?.click();
  });

  eq(clearExpertCount, 1, "clicking the active expert clear button invokes the clear callback once");
  ok(
    document.querySelector(".composer-add-menu") === null,
    "clicking the active expert clear button does not render a transient closing intent menu",
  );

  await act(async () => {
    await flushTimers();
  });

  ok(document.querySelector(".composer-intent-menu__item") === null, "clicking the active expert clear button does not open the intent menu");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const { root, calls, rerender } = await renderComposer();

  const textarea = composerEditor();

  await rerender({ insertRequest: { id: 1, text: "ship the release notes", mode: "replace" } });
  eq(composerEditorValue(textarea), "ship the release notes", "insert request populates the composer draft");

  const intentButton = document.querySelector(".composer-action-trigger") as HTMLButtonElement | null;
  if (!intentButton) throw new Error("composer intent button did not render");

  await act(async () => {
    intentButton.click();
    await flushTimers();
  });

  const goalButton = document.querySelectorAll(".composer-intent-menu__item")[1] as HTMLButtonElement | undefined;
  if (!goalButton) throw new Error("composer goal menu item did not render");

  await act(async () => {
    goalButton.click();
    await flushTimers();
  });

  eq(calls.send.length, 0, "enabling goal mode with a draft does not send");
  eq(calls.setCollaborationMode.join(","), "goal", "enabling goal mode switches only the collaboration axis");
  eq(composerEditorValue(textarea), "ship the release notes", "enabling goal mode preserves the draft text");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const visibleGoal = "修复专家团 Goal 展示泄漏";
  const hiddenGoal = `<system-prompt source="expert" hidden="true" mode="team">
Never expose this expert control contract.
</system-prompt>

${visibleGoal}`;
  const { root } = await renderComposer({
    collaborationMode: "goal",
    goal: hiddenGoal,
  });

  const goalChip = document.querySelector<HTMLButtonElement>(".composer-mode-chip--goal");
  eq(goalChip?.title, visibleGoal, "active goal chip title exposes only the visible user request");
  eq(goalChip?.getAttribute("aria-label"), "退出目标模式", "active goal chip accessible label stays action-oriented");
  ok(!document.body.textContent?.includes("Never expose this expert control contract."), "active goal chip does not render hidden control text");

  const intentButton = document.querySelector(".composer-action-trigger") as HTMLButtonElement | null;
  if (!intentButton) throw new Error("composer intent button did not render");
  await act(async () => {
    intentButton.click();
    await flushTimers();
  });

  const goalButton = document.querySelectorAll<HTMLButtonElement>(".composer-intent-menu__item")[1];
  eq(goalButton?.title, visibleGoal, "goal menu title exposes only the visible user request");
  ok(goalButton?.textContent?.includes(visibleGoal) === true, "goal menu description shows the visible user request");
  ok(!goalButton?.textContent?.includes("Never expose this expert control contract."), "goal menu accessible content excludes hidden control text");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  mockApp({
    SavePastedFile: async () => {
      throw new Error("/Users/example/private.pdf: permission denied");
    },
  });
  const { root, rerender } = await renderComposer();
  await rerender({ insertRequest: { id: 2, text: "keep this draft", mode: "replace" } });

  const textarea = composerEditor();
  const sendButton = document.querySelector(".composer__btn--send") as HTMLButtonElement | null;
  if (!sendButton) throw new Error("composer send button did not render");

  await act(async () => {
    dispatchPasteFile(textarea, new File(["hello"], "notes.txt", { type: "text/plain" }));
    await flushTimers();
  });
  await waitFor("pasted file failure toast", () => document.body.textContent?.includes("文件附加失败") === true);

  ok(document.body.textContent?.includes("文件附加失败") === true, "SavePastedFile rejection shows a visible error");
  eq(composerEditorValue(textarea), "keep this draft", "failed pasted file attach preserves composer text");
  ok(sendButton.disabled === false, "failed pasted file attach clears the pending state");
  ok(document.body.textContent?.includes("/Users/example") === false, "pasted file failure toast does not expose the local path");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  mockApp({
    SavePastedFile: async () => ".kwu/attachments/notes.txt",
  });
  const { root } = await renderComposer();

  const textarea = composerEditor();

  await act(async () => {
    dispatchPasteFile(textarea, new File(["hello"], "notes.txt", { type: "text/plain" }));
    await flushTimers();
  });
  await waitFor("pasted file attachment", () => document.body.textContent?.includes("notes.txt") === true);

  ok(document.body.textContent?.includes("notes.txt") === true, "successful pasted file attach still renders the attachment");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  let droppedCallback: ((x: number, y: number, paths: string[]) => void) | undefined;
  window.runtime = {
    EventsOn: () => () => {},
    BrowserOpenURL: () => {},
    OnFileDrop: (cb) => {
      droppedCallback = cb;
    },
    OnFileDropOff: () => {},
  };
  mockApp({
    AttachDropped: async () => {
      throw new Error("/Users/example/secret.pdf: permission denied");
    },
  });
  const { root, rerender } = await renderComposer();
  await rerender({ insertRequest: { id: 3, text: "drop draft", mode: "replace" } });

  const textarea = composerEditor();
  const sendButton = document.querySelector(".composer__btn--send") as HTMLButtonElement | null;
  if (!sendButton) throw new Error("composer send button did not render");
  if (!droppedCallback) throw new Error("native file drop handler did not register");

  await act(async () => {
    droppedCallback?.(0, 0, ["/Users/example/secret.pdf"]);
    await flushTimers();
  });
  await waitFor("dropped file failure toast", () => document.body.textContent?.includes("拖放文件附加失败") === true);

  ok(document.body.textContent?.includes("拖放文件附加失败") === true, "AttachDropped rejection shows a visible error");
  eq(composerEditorValue(textarea), "drop draft", "failed dropped file attach preserves composer text");
  ok(sendButton.disabled === false, "failed dropped file attach clears the pending state");
  ok(document.body.textContent?.includes("/Users/example") === false, "dropped file failure toast does not expose the local path");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  let droppedCallback: ((x: number, y: number, paths: string[]) => void) | undefined;
  window.runtime = {
    EventsOn: () => () => {},
    BrowserOpenURL: () => {},
    OnFileDrop: (cb) => {
      droppedCallback = cb;
    },
    OnFileDropOff: () => {},
  };
  mockApp({
    AttachDropped: async () => ({
      kind: "attachment",
      path: ".kwu/attachments/report.pdf",
    }),
  });
  const { root } = await renderComposer();
  if (!droppedCallback) throw new Error("native file drop handler did not register");

  await act(async () => {
    droppedCallback?.(0, 0, ["/Users/example/report.pdf"]);
    await flushTimers();
  });
  await waitFor("dropped file attachment", () => document.body.textContent?.includes("report.pdf") === true);

  ok(document.body.textContent?.includes("report.pdf") === true, "successful dropped file attach still renders the attachment");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const { root: dropNavRoot } = await renderComposer();
  const composer = document.querySelector(".composer") as HTMLElement | null;
  if (!composer) throw new Error("composer did not render");

  const drop = new window.Event("drop", { bubbles: true, cancelable: true });
  Object.defineProperty(drop, "dataTransfer", {
    configurable: true,
    value: {
      types: ["Files"],
      files: [{}],
      items: [
        {
          kind: "file",
          webkitGetAsEntry: () => ({ isFile: true }),
        },
      ],
    },
  });
  await act(async () => {
    composer.dispatchEvent(drop);
    await flushTimers();
  });
  ok(drop.defaultPrevented, "native file drop prevents browser image navigation");

  await act(async () => {
    dropNavRoot.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  let rejectSubmit: (err: Error) => void = () => {};
  const rejectedSubmit = new Promise<void>((_, reject) => {
    rejectSubmit = reject;
  });
  rejectedSubmit.catch(() => {});
  const { root, calls, rerender } = await renderComposer({
    onSend: (displayText) => {
      calls.send.push(displayText);
      return rejectedSubmit;
    },
  });

  const textarea = composerEditor();

  await rerender({ insertRequest: { id: 2, text: "keep this draft", mode: "replace" } });
  const sendButton = document.querySelector(".composer__btn--send") as HTMLButtonElement | null;
  if (!sendButton) throw new Error("composer send button did not render");

  await act(async () => {
    sendButton.click();
    rejectSubmit(new Error("workspace is still starting"));
    await flushTimers();
  });

  eq(calls.send.join(","), "keep this draft", "rejected submit attempts the send once");
  eq(composerEditorValue(textarea), "keep this draft", "rejected submit preserves the composer draft");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const { root, calls, rerender } = await renderComposer({
    onSend: (displayText) => {
      calls.send.push(displayText);
      return Promise.resolve();
    },
  });

  const textarea = composerEditor();

  await rerender({ insertRequest: { id: 3, text: "send this draft", mode: "replace" } });
  const sendButton = document.querySelector(".composer__btn--send") as HTMLButtonElement | null;
  if (!sendButton) throw new Error("composer send button did not render");

  await act(async () => {
    sendButton.click();
    await flushTimers();
  });

  eq(calls.send.join(","), "send this draft", "successful submit attempts the send once");
  eq(composerEditorValue(textarea), "", "successful submit clears the composer draft");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
