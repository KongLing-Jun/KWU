// Run: tsx src/__tests__/composer-session-draft.test.tsx

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { Composer, type ComposerDraftStore } from "../components/Composer";
import { composerDraftKeyForTab } from "../lib/composerDraftKey";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { CollaborationMode, ComposerInsertRequest, TokenMode, ToolApprovalMode } from "../lib/types";

let passed = 0;
let failed = 0;
const testDir = dirname(fileURLToPath(import.meta.url));
const composerSource = readFileSync(resolve(testDir, "../features/composer/ui/Composer.tsx"), "utf8");
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

ok(!composerSource.includes("entryMode="), "project and ordinary composers share the same root model menu");

function eq(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) ok(true, label);
  else ok(false, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function flushTimers(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
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
  globalThis.File = dom.window.File;
  globalThis.FileReader = dom.window.FileReader;
  globalThis.PointerEvent = dom.window.MouseEvent as unknown as typeof PointerEvent;
  globalThis.MutationObserver = dom.window.MutationObserver;
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

function installBridgeApp(methods: Record<string, unknown>) {
  (window as unknown as { go: { main: { App: Record<string, unknown> } } }).go = {
    main: {
      App: {
        Commands: async () => [],
        Models: async () => [],
        ModelsForTab: async () => [],
        ...methods,
      },
    },
  };
}

async function renderComposer(props: Partial<Parameters<typeof Composer>[0]> = {}) {
  if (!window.go?.main?.App) installBridgeApp({});
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let currentProps: Parameters<typeof Composer>[0] = {
    running: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask" as ToolApprovalMode,
    tokenMode: "full" as TokenMode,
    goal: "",
    cwd: "/repo",
    modelLabel: "DeepSeek-R1",
    tabId: "single-surface-tab",
    sessionKey: "session:project:/repo:topic-a:session-a",
    onSend: () => {},
    onCancel: () => undefined,
    onCycleMode: () => {},
    onSetMode: () => {},
    onSetCollaborationMode: (_mode: CollaborationMode) => {},
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
  const unmountComposer = async () => {
    await act(async () => {
      root.render(null);
      await flushTimers();
    });
  };
  return { root, rerender: paint, unmountComposer };
}

function textarea(): HTMLDivElement {
  const node = document.querySelector<HTMLDivElement>("#composer-input");
  if (!node) throw new Error("composer editor did not render");
  return node;
}

function contextItemCount(): number {
  return document.querySelectorAll(".composer-context__item").length;
}

function sendButton(): HTMLButtonElement {
  const node = document.querySelector(".composer__btn--send") as HTMLButtonElement | null;
  if (!node) throw new Error("composer send button did not render");
  return node;
}

function setTextareaValue(value: string) {
  const node = textarea();
  node.replaceChildren(document.createTextNode(value));
  node.dispatchEvent(new Event("input", { bubbles: true }));
}

function pressTextareaKey(key: string) {
  const node = textarea();
  node.dispatchEvent(new KeyboardEvent("keydown", {
    key,
    code: key,
    keyCode: key === "Enter" ? 13 : 0,
    which: key === "Enter" ? 13 : 0,
    bubbles: true,
    cancelable: true,
  }));
}

function dispatchPasteFile(node: HTMLElement, file: File) {
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
  node.dispatchEvent(event);
}

console.log("\ncomposer session draft");

ok(
  /composer-context__item--session[\s\S]*<button[\s\S]*aria-label=\{t\("composer\.removeReference"\)\}/.test(composerSource),
  "session context remove button has an accessible name",
);

ok(
  /if \(!trimmedText && attachments\.length === 0 && workspaceRefs\.length === 0 && sessionRefs\.length === 0\)/.test(composerSource) &&
    /disabled=\{[\s\S]*!text\.trim\(\) && attachments\.length === 0 && workspaceRefs\.length === 0 && sessionRefs\.length === 0/.test(composerSource),
  "session refs can be submitted without extra prompt text",
);

ok(
  /setComposerInsertRequest\(\(current\) => current\?\.id === id \? null : current\)/.test(appSource) &&
    appSource.includes("onInsertRequestConsumed={handleComposerInsertRequestConsumed}"),
  "App clears only the insert request acknowledged by Composer",
);

{
  const withoutPath = composerDraftKeyForTab({
    id: "tab-a",
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "topic-a",
    sessionPath: "",
  }, "tab-a");
  const withPath = composerDraftKeyForTab({
    id: "tab-a",
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "topic-a",
    sessionPath: "/repo/.kwu/sessions/topic-a.jsonl",
  }, "tab-a");
  ok(withPath !== withoutPath, "session path draft key isolates sessions under the same topic");
  eq(withPath, ["session-path", "project", "/repo", "/repo/.kwu/sessions/topic-a.jsonl"].join("\u0000"), "session path draft key uses session path when available");
  eq(
    composerDraftKeyForTab({
      scope: "project",
      workspaceRoot: "/repo",
      topicId: "topic-a",
      sessionPath: "/repo/.kwu/sessions/topic-a.jsonl",
    }),
    withPath,
    "transition target and activated tab metadata resolve to the same draft key",
  );
}

{
  const dom = installDom();
  const sends: string[] = [];
  installBridgeApp({
    Commands: async () => [{ name: "mcp", description: "MCP servers", kind: "builtin" }],
    SlashArgs: async () => ({ items: [{ label: "connect", insert: "connect ", hint: "connect", descend: true }], from: 5 }),
  });
  const { root } = await renderComposer({
    onSend: (displayText) => {
      sends.push(displayText);
    },
  });

  await act(async () => {
    await flushTimers();
  });
  await act(async () => {
    setTextareaValue("/mcp");
    await flushTimers();
  });
  ok(Boolean(document.querySelector(".slashmenu")), "exact slash command keeps its completion menu visible before submit");
  await act(async () => {
    pressTextareaKey("Enter");
    await flushTimers();
  });
  eq(sends[0], "/mcp", "Enter submits an exact slash command instead of accepting the same completion");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  const { root, rerender } = await renderComposer();

  await rerender({ insertRequest: { id: 1, text: "draft for A", mode: "replace" } });
  await rerender({ insertRequest: { id: 2, text: "@/repo/src/app.ts", mode: "insert" } });
  eq(textarea().textContent, "draft for A", "session A text is visible before switching");
  eq(contextItemCount(), 1, "session A workspace ref is visible before switching");

  await rerender({ sessionKey: "session:project:/repo:topic-b:session-b" });
  eq(textarea().textContent, "", "session B does not inherit session A text");
  eq(contextItemCount(), 0, "session B does not inherit session A context refs");

  await rerender({ insertRequest: { id: 3, text: "draft for B", mode: "replace" } });
  eq(textarea().textContent, "draft for B", "session B keeps its own text draft");

  await rerender({ sessionKey: "session:project:/repo:topic-a:session-a" });
  eq(textarea().textContent, "draft for A", "session A text is restored when switching back");
  eq(contextItemCount(), 1, "session A context refs are restored when switching back");

  await rerender({ sessionKey: "session:project:/repo:topic-b:session-b" });
  eq(textarea().textContent, "draft for B", "session B text is restored independently");
  eq(contextItemCount(), 0, "session B context refs stay isolated");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const sharedDraftStore = { current: {} };
  const projectKey = JSON.stringify(["project-composer-v1", "scope-a", "project-a"]);
  const { root, rerender, unmountComposer } = await renderComposer({ sessionKey: projectKey, draftStore: sharedDraftStore });

  await rerender({ insertRequest: { id: 901, text: "project draft survives capability navigation", mode: "replace" } });
  eq(textarea().textContent, "project draft survives capability navigation", "project draft is visible before unmount");
  await unmountComposer();
  await rerender({ insertRequest: null });
  eq(textarea().textContent, "project draft survives capability navigation", "App-owned draft store restores the project draft after remount");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  let activeRequest: ComposerInsertRequest | null = { id: 20, text: "expert starter", mode: "replace" };
  const consumedIds: number[] = [];
  const onInsertRequestConsumed = (id: number) => {
    consumedIds.push(id);
    if (activeRequest?.id === id) activeRequest = null;
  };
  const { root, rerender, unmountComposer } = await renderComposer({
    insertRequest: activeRequest,
    onInsertRequestConsumed,
  });

  eq(textarea().textContent, "expert starter", "insert request fills the composer on first consumption");
  eq(consumedIds.join(","), "20", "Composer acknowledges the first insert request once");
  await act(async () => {
    setTextareaValue("");
    await flushTimers();
  });
  await unmountComposer();
  await rerender({ insertRequest: activeRequest });
  eq(textarea().textContent, "", "consumed insert request does not replay after Composer remount");
  eq(consumedIds.join(","), "20", "remount does not acknowledge the consumed request again");

  activeRequest = { id: 21, text: "new starter", mode: "replace" };
  await rerender({ insertRequest: activeRequest });
  eq(textarea().textContent, "new starter", "a later insert request still fills the remounted Composer");
  eq(consumedIds.join(","), "20,21", "a later insert request receives its own acknowledgement");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  const sourceKey = composerDraftKeyForTab({
    id: "source-tab",
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "source-topic",
    sessionPath: "/repo/source.jsonl",
  });
  const transitionTarget = {
    scope: "project",
    workspaceRoot: "/repo",
    topicId: "target-topic",
    sessionPath: "/repo/target.jsonl",
  };
  const transitionKey = composerDraftKeyForTab(transitionTarget);
  const activatedKey = composerDraftKeyForTab({ id: "target-tab", ...transitionTarget }, "target-tab");
  const { root, rerender } = await renderComposer({ sessionKey: sourceKey });

  await rerender({ insertRequest: { id: 10, text: "source draft", mode: "replace" } });
  await rerender({ sessionKey: transitionKey, insertRequest: { id: 11, text: "typed while activating", mode: "replace" } });
  eq(textarea().textContent, "typed while activating", "transition input is written directly to the target draft");
  await rerender({ sessionKey: activatedKey });
  eq(textarea().textContent, "typed while activating", "target draft survives activation without an identity switch");
  await rerender({ sessionKey: sourceKey });
  eq(textarea().textContent, "source draft", "transition input does not pollute the source draft");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  const saveStarted = deferred<void>();
  const savePastedFile = deferred<string>();
  installBridgeApp({
    SavePastedFile: async () => {
      saveStarted.resolve();
      return savePastedFile.promise;
    },
  });
  const { root, rerender } = await renderComposer();
  const file = new File(["draft attachment"], "draft.txt", { type: "text/plain", lastModified: 1 });
  const event = new Event("paste", { bubbles: true, cancelable: true });
  Object.defineProperty(event, "clipboardData", {
    configurable: true,
    value: {
      files: [file],
      items: [],
      types: [],
      getData: () => "",
    },
  });

  await act(async () => {
    textarea().dispatchEvent(event);
    await saveStarted.promise;
  });
  await rerender({ sessionKey: "session:project:/repo:topic-b:session-b" });
  await act(async () => {
    savePastedFile.resolve("/tmp/kwu/draft.txt");
    await flushTimers();
  });
  eq(contextItemCount(), 0, "async attachment does not land in the switched-to session");

  await rerender({ sessionKey: "session:project:/repo:topic-a:session-a" });
  eq(contextItemCount(), 1, "async attachment returns to the source session draft");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const saveStarted = deferred<void>();
  const savePastedFile = deferred<string>();
  const sends: string[] = [];
  installBridgeApp({
    SavePastedFile: async () => {
      saveStarted.resolve();
      return savePastedFile.promise;
    },
  });
  const { root } = await renderComposer({
    onSend: (displayText) => {
      sends.push(displayText);
    },
  });
  const file = new File(["pending attachment"], "pending.txt", { type: "text/plain", lastModified: 2 });

  await act(async () => {
    setTextareaValue("send with pending attachment");
    dispatchPasteFile(textarea(), file);
    sendButton().click();
    await saveStarted.promise;
    await flushTimers();
  });
  eq(sends.length, 0, "submit is blocked while pasted attachment save is pending");

  await act(async () => {
    savePastedFile.resolve(".kwu/attachments/pending.txt");
    await flushTimers();
  });

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const outcomes = ["handled-no-turn", "blocked", "submitted"] as const;
  const sharedDraftStore: ComposerDraftStore = { current: {} };
  const sessionKey = "project:receipt";
  const clearedDraftKeys: string[] = [];
  const { root, rerender, unmountComposer } = await renderComposer({
    sessionKey,
    draftStore: sharedDraftStore,
    onSend: async () => outcomes[0],
    onSubmittedDraftCleared: (draftKey) => { clearedDraftKeys.push(draftKey); },
  });

  await act(async () => {
    setTextareaValue("keep project draft");
    sendButton().click();
    await flushTimers();
  });
  eq(textarea().textContent, "keep project draft", "handled-no-turn keeps the production Composer draft");

  await rerender({ onSend: async () => outcomes[1] });
  await act(async () => {
    sendButton().click();
    await flushTimers();
  });
  eq(textarea().textContent, "keep project draft", "blocked keeps the production Composer draft");

  await rerender({ onSend: async () => outcomes[2] });
  await act(async () => {
    sendButton().click();
    await flushTimers();
  });
  eq(textarea().textContent, "", "submitted clears the production Composer draft exactly once");
  eq(sharedDraftStore.current[sessionKey]?.text, "", "submitted synchronously clears the shared project draft store");
  eq(clearedDraftKeys.join(","), sessionKey, "submitted reports the cleared project draft after its store is empty");

  await unmountComposer();
  await rerender({ onSend: async () => outcomes[2] });
  eq(textarea().textContent, "", "submitted project draft stays empty after unmount and remount");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  let cancelCalls = 0;
  const { root } = await renderComposer({
    running: true,
    turnStartAt: Date.now(),
    onCancel: () => {
      cancelCalls += 1;
      return undefined;
    },
  });

  await act(async () => {
    const stop = document.querySelector<HTMLButtonElement>(".composer-runstatus__stop");
    if (!stop) throw new Error("composer stop button did not render");
    stop.click();
    await flushTimers();
  });
  eq(cancelCalls, 1, "running production Composer delegates Stop to the scoped cancel callback");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  const { root, rerender } = await renderComposer({
    disabled: true,
    surfaceTransitioning: true,
  });

  const composerWrap = document.querySelector<HTMLElement>(".composer-wrap");
  eq(composerWrap?.getAttribute("aria-busy"), "true", "surface transition exposes the Composer busy state");
  eq(document.querySelectorAll(".composer-transition-shell").length, 0, "surface transition does not replace the real Composer with a skeleton");
  eq(document.querySelectorAll("#composer-input").length, 1, "surface transition keeps the real Composer input mounted");
  ok(document.querySelector(".composer")?.classList.contains("composer--disabled") === false, "surface transition does not visually dim the real input surface");
  eq(textarea().getAttribute("contenteditable"), "false", "surface transition leaves the real input visible but disabled");
  ok(sendButton().disabled, "surface transition keeps the real send action disabled");

  await rerender({ disabled: false, surfaceTransitioning: false });
  eq(composerWrap?.getAttribute("aria-busy"), null, "activated Composer clears the busy state");
  eq(document.querySelectorAll("#composer-input").length, 1, "activated Composer still reuses the original input instance");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
