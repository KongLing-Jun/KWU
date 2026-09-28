// Run: tsx src/__tests__/workspace-artifact-refresh.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { WorkspacePanel } from "../components/WorkspacePanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import type { DirEntry, WireEvent } from "../lib/types";
import { shouldRefreshWorkspaceForEvent } from "../lib/workspaceArtifactRefresh";

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

function flushPromises(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function eventually(predicate: () => boolean, attempts = 20): Promise<boolean> {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return true;
  }
  return false;
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
  globalThis.Event = dom.window.Event;
  globalThis.CustomEvent = dom.window.CustomEvent;
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.PointerEvent = dom.window.MouseEvent as unknown as typeof PointerEvent;
  globalThis.MutationObserver = dom.window.MutationObserver;
  globalThis.ResizeObserver = TestResizeObserver;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window.HTMLElement.prototype, "scrollIntoView", { configurable: true, value: () => {} });
  Object.defineProperty(dom.window.HTMLElement.prototype, "offsetHeight", {
    configurable: true,
    get() { return this.classList.contains("workspace-tree") ? 600 : 24; },
  });
  Object.defineProperty(dom.window.HTMLElement.prototype, "offsetWidth", { configurable: true, get() { return 800; } });
  return dom;
}

function installWorkspaceBindings(
  listDir: AppBindings["ListDirForTab"],
  searchFileRefsForTab: AppBindings["SearchFileRefsForTab"] = async () => [],
) {
  window.go = {
    main: {
      App: {
        ListDir: async () => {
          throw new Error("active ListDir must not run for an owning tab");
        },
        ListDirForTab: listDir,
        SearchFileRefs: async () => {
          throw new Error("active SearchFileRefs must not run for an owning tab");
        },
        SearchFileRefsForTab: searchFileRefsForTab,
        ReadFile: async () => ({ path: "", body: "", size: 0, truncated: false, binary: false }),
        ReadFileForTab: async () => ({ path: "", body: "", size: 0, truncated: false, binary: false }),
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
}

async function renderPanel(root: Root, refreshKey: number) {
  await act(async () => {
    root.render(
      <LocaleProvider>
        <WorkspacePanel
          open
          tabId="tab-artifacts"
          cwd="/workspace"
          maximized={false}
          initialViewMode="files"
          refreshKey={refreshKey}
          onClose={() => {}}
          onToggleMaximized={() => {}}
        />
      </LocaleProvider>,
    );
    await flushPromises();
  });
}

async function clickWorkspacePath(path: string): Promise<boolean> {
  const found = await eventually(() => Boolean(document.querySelector(`[data-workspace-path="${path}"]`)));
  if (!found) return false;
  const row = document.querySelector(`[data-workspace-path="${path}"]`) as HTMLButtonElement;
  await act(async () => {
    row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  return true;
}

async function setWorkspaceFilter(value: string): Promise<boolean> {
  const input = document.querySelector(".workspace-search input") as HTMLInputElement | null;
  if (!input) return false;
  await act(async () => {
    const setInputValue = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
    if (!setInputValue) throw new Error("missing input value setter");
    setInputValue.call(input, value);
    const reactPropsKey = Object.keys(input).find((key) => key.startsWith("__reactProps$"));
    const reactProps = reactPropsKey
      ? (input as unknown as Record<string, { onChange?: (event: { target: HTMLInputElement }) => void }>)[reactPropsKey]
      : undefined;
    if (!reactProps?.onChange) throw new Error("missing React input change handler");
    reactProps.onChange({ target: input });
    await flushPromises();
  });
  return true;
}

async function triggerManualTreeRefresh(): Promise<boolean> {
  const tree = document.querySelector(".workspace-tree") as HTMLDivElement | null;
  if (!tree) return false;
  await act(async () => {
    tree.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: 20, clientY: 20 }));
    await flushPromises();
  });
  const menuReady = await eventually(() => Array.from(document.querySelectorAll("button")).some((button) => button.textContent?.includes("刷新文件树")));
  if (!menuReady) return false;
  const refreshButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes("刷新文件树")) as HTMLButtonElement | undefined;
  if (!refreshButton) return false;
  await act(async () => {
    refreshButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  return true;
}

async function closeFixture(root: Root, dom: JSDOM) {
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log("\nworkspace artifact refresh events");

const eventCases: Array<{ label: string; event: WireEvent; expected: boolean }> = [
  {
    label: "successful writer result refreshes workspace",
    event: { kind: "tool_result", tool: { name: "write_file", readOnly: false } } as WireEvent,
    expected: true,
  },
  {
    label: "failed writer result refreshes workspace",
    event: { kind: "tool_result", tool: { name: "write_file", readOnly: false, err: "partial write" } } as WireEvent,
    expected: true,
  },
  {
    label: "successful reader result does not refresh workspace",
    event: { kind: "tool_result", tool: { name: "read_file", readOnly: true } } as WireEvent,
    expected: false,
  },
  {
    label: "failed reader result does not refresh workspace",
    event: { kind: "tool_result", tool: { name: "read_file", readOnly: true, err: "missing" } } as WireEvent,
    expected: false,
  },
  {
    label: "tool result without a tool refreshes conservatively",
    event: { kind: "tool_result" } as WireEvent,
    expected: true,
  },
  {
    label: "tool result without readOnly refreshes conservatively",
    event: { kind: "tool_result", tool: { name: "legacy_tool" } } as WireEvent,
    expected: true,
  },
  {
    label: "successful turn completion refreshes workspace",
    event: { kind: "turn_done" } as WireEvent,
    expected: true,
  },
  {
    label: "failed turn completion refreshes workspace",
    event: { kind: "turn_done", err: "turn failed" } as WireEvent,
    expected: true,
  },
  {
    label: "ordinary notice refreshes workspace",
    event: { kind: "notice", text: "background work completed" } as WireEvent,
    expected: true,
  },
  {
    label: "warning notice refreshes workspace",
    event: { kind: "notice", level: "warn", text: "background work failed" } as WireEvent,
    expected: true,
  },
  {
    label: "tool dispatch does not refresh workspace",
    event: { kind: "tool_dispatch", tool: { name: "write_file", readOnly: false } } as WireEvent,
    expected: false,
  },
  {
    label: "assistant text does not refresh workspace",
    event: { kind: "text", text: "working" } as WireEvent,
    expected: false,
  },
];

for (const { label, event, expected } of eventCases) {
  ok(shouldRefreshWorkspaceForEvent(event) === expected, label);
}

console.log("\nworkspace artifact refresh cache invalidation");

{
  const dom = installDom();
  let generated = false;
  let reportsCalls = 0;
  installWorkspaceBindings(async (_tabID: string, dir: string): Promise<DirEntry[]> => {
    if (dir === "") return [{ name: "reports", isDir: true }];
    if (dir === "reports/") {
      reportsCalls += 1;
      return generated ? [{ name: "report.xlsx", isDir: false }] : [];
    }
    return [];
  });
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await clickWorkspacePath("reports/"), "cached-directory fixture expands reports");
  ok(await eventually(() => reportsCalls === 1), "empty reports directory is cached");
  ok(await clickWorkspacePath("reports/"), "cached-directory fixture collapses reports");
  generated = true;
  await renderPanel(root, 1);
  ok(await clickWorkspacePath("reports/"), "cached-directory fixture re-expands reports after refresh");
  ok(await eventually(() => document.body.textContent?.includes("report.xlsx") === true), "refresh invalidates a collapsed directory cache");
  ok(reportsCalls === 2, "re-expanding a collapsed directory after refresh issues a new ListDir");

  await closeFixture(root, dom);
}

{
  const dom = installDom();
  let reportsCalls = 0;
  let resolveStale: ((entries: DirEntry[]) => void) | undefined;
  let resolveFresh: ((entries: DirEntry[]) => void) | undefined;
  installWorkspaceBindings(async (_tabID: string, dir: string): Promise<DirEntry[]> => {
    if (dir === "") return [{ name: "reports", isDir: true }];
    if (dir !== "reports/") return [];
    reportsCalls += 1;
    return new Promise<DirEntry[]>((resolve) => {
      if (reportsCalls === 1) resolveStale = resolve;
      else resolveFresh = resolve;
    });
  });
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await clickWorkspacePath("reports/"), "pending-response fixture expands reports");
  ok(await eventually(() => reportsCalls === 1 && Boolean(resolveStale)), "pre-refresh directory request is pending");
  ok(await clickWorkspacePath("reports/"), "pending-response fixture collapses reports");
  await renderPanel(root, 1);
  await act(async () => {
    resolveStale?.([{ name: "stale-before-refresh.xlsx", isDir: false }]);
    await flushPromises();
  });
  ok(await clickWorkspacePath("reports/"), "pending-response fixture re-expands reports after refresh");
  const freshRequestStarted = await eventually(() => reportsCalls === 2 && Boolean(resolveFresh));
  ok(freshRequestStarted, "post-refresh expansion issues a fresh ListDir");
  ok(document.body.textContent?.includes("stale-before-refresh.xlsx") === false, "pre-refresh pending response is ignored");
  if (resolveFresh) {
    await act(async () => {
      resolveFresh?.([{ name: "fresh-after-refresh.xlsx", isDir: false }]);
      await flushPromises();
    });
  }
  ok(await eventually(() => document.body.textContent?.includes("fresh-after-refresh.xlsx") === true), "fresh post-refresh response populates the directory");

  await closeFixture(root, dom);
}

{
  const dom = installDom();
  let reportsCalls = 0;
  let resolveStale: ((entries: DirEntry[]) => void) | undefined;
  let resolveFresh: ((entries: DirEntry[]) => void) | undefined;
  installWorkspaceBindings(async (_tabID: string, dir: string): Promise<DirEntry[]> => {
    if (dir === "") return [{ name: "reports", isDir: true }];
    if (dir !== "reports/") return [];
    reportsCalls += 1;
    return new Promise<DirEntry[]>((resolve) => {
      if (reportsCalls === 1) resolveStale = resolve;
      else resolveFresh = resolve;
    });
  });
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await clickWorkspacePath("reports/"), "manual-refresh fixture expands reports");
  ok(await eventually(() => reportsCalls === 1 && Boolean(resolveStale)), "manual-refresh pre-refresh request is pending");
  ok(await clickWorkspacePath("reports/"), "manual-refresh fixture collapses reports");
  const tree = document.querySelector(".workspace-tree") as HTMLDivElement | null;
  if (!tree) throw new Error("missing workspace tree");
  await act(async () => {
    tree.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: 20, clientY: 20 }));
    await flushPromises();
  });
  const refreshMenuReady = await eventually(() => Array.from(document.querySelectorAll("button")).some((button) => button.textContent?.includes("刷新文件树")));
  ok(refreshMenuReady, "manual file-tree refresh action is available");
  const refreshButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes("刷新文件树")) as HTMLButtonElement | undefined;
  if (refreshButton) {
    await act(async () => {
      refreshButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await flushPromises();
    });
  }
  await act(async () => {
    resolveStale?.([{ name: "stale-before-manual-refresh.xlsx", isDir: false }]);
    await flushPromises();
  });
  ok(await clickWorkspacePath("reports/"), "manual-refresh fixture re-expands reports");
  const freshRequestStarted = await eventually(() => reportsCalls === 2 && Boolean(resolveFresh));
  ok(freshRequestStarted, "manual refresh advances request generations and forces a fresh ListDir");
  ok(document.body.textContent?.includes("stale-before-manual-refresh.xlsx") === false, "manual refresh ignores its pre-refresh pending response");
  if (resolveFresh) {
    await act(async () => {
      resolveFresh?.([{ name: "fresh-after-manual-refresh.xlsx", isDir: false }]);
      await flushPromises();
    });
  }
  ok(await eventually(() => document.body.textContent?.includes("fresh-after-manual-refresh.xlsx") === true), "manual refresh accepts the fresh response");

  await closeFixture(root, dom);
}

{
  const dom = installDom();
  let rootCalls = 0;
  installWorkspaceBindings(async (_tabID: string, dir: string): Promise<DirEntry[]> => {
    if (dir === "") {
      rootCalls += 1;
      return [{ name: "reports", isDir: true }];
    }
    return [];
  });
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await eventually(() => rootCalls >= 1), "view-mode fixture loads the initial tree");
  await renderPanel(root, 1);
  ok(await eventually(() => rootCalls >= 2), "view-mode fixture applies the new refresh key");
  const callsAfterRefresh = rootCalls;
  const tabs = Array.from(document.querySelectorAll(".workspace-files__tabs button[role='tab']")) as HTMLButtonElement[];
  if (tabs.length !== 2) throw new Error("missing workspace view tabs");
  await act(async () => {
    tabs[1].dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
    await flushPromises();
  });
  ok(rootCalls === callsAfterRefresh, "changing only viewMode does not reapply an unchanged refresh key");

  await closeFixture(root, dom);
}

console.log("\nworkspace artifact search refresh");

{
  const dom = installDom();
  let generated = false;
  let searchCalls = 0;
  installWorkspaceBindings(
    async () => [],
    async (_tabID: string, query: string): Promise<DirEntry[]> => {
      searchCalls += 1;
      return generated
        ? [{ name: `${query}-new-report.xlsx`, isDir: false }]
        : [{ name: `${query}-old-result.txt`, isDir: false }];
    },
  );
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await setWorkspaceFilter("report"), "automatic-search fixture keeps an active filter");
  ok(await eventually(() => searchCalls === 1 && document.body.textContent?.includes("report-old-result.txt") === true), "initial filtered search result is visible");
  generated = true;
  await renderPanel(root, 1);
  ok(await eventually(() => searchCalls === 2), "automatic refresh reruns the active file search");
  ok(await eventually(() => document.body.textContent?.includes("report-new-report.xlsx") === true), "automatic refresh shows the newly generated workbook in filtered results");
  ok(document.body.textContent?.includes("report-old-result.txt") === false, "automatic refresh replaces the stale filtered result");

  await closeFixture(root, dom);
}

{
  const dom = installDom();
  let searchCalls = 0;
  let resolveStale: ((entries: DirEntry[]) => void) | undefined;
  let resolveFresh: ((entries: DirEntry[]) => void) | undefined;
  installWorkspaceBindings(
    async () => [],
    async (): Promise<DirEntry[]> => {
      searchCalls += 1;
      return new Promise<DirEntry[]>((resolve) => {
        if (searchCalls === 1) resolveStale = resolve;
        else resolveFresh = resolve;
      });
    },
  );
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await setWorkspaceFilter("pending"), "pending-search fixture keeps an active filter");
  ok(await eventually(() => searchCalls === 1 && Boolean(resolveStale)), "pre-refresh search request is pending");
  await renderPanel(root, 1);
  const freshSearchStarted = await eventually(() => searchCalls === 2 && Boolean(resolveFresh));
  ok(freshSearchStarted, "automatic refresh starts a fresh filtered search");
  await act(async () => {
    resolveStale?.([{ name: "pending-stale-before-refresh.xlsx", isDir: false }]);
    await flushPromises();
  });
  ok(document.body.textContent?.includes("pending-stale-before-refresh.xlsx") === false, "pre-refresh pending search response is ignored");
  if (resolveFresh) {
    await act(async () => {
      resolveFresh?.([{ name: "pending-fresh-after-refresh.xlsx", isDir: false }]);
      await flushPromises();
    });
  }
  ok(await eventually(() => document.body.textContent?.includes("pending-fresh-after-refresh.xlsx") === true), "fresh post-refresh search response is accepted");

  await closeFixture(root, dom);
}

{
  const dom = installDom();
  let generated = false;
  let searchCalls = 0;
  installWorkspaceBindings(
    async () => [],
    async (_tabID: string, query: string): Promise<DirEntry[]> => {
      searchCalls += 1;
      return generated
        ? [{ name: `${query}-manual-new.xlsx`, isDir: false }]
        : [{ name: `${query}-manual-old.txt`, isDir: false }];
    },
  );
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await setWorkspaceFilter("manual"), "manual-search fixture keeps an active filter");
  ok(await eventually(() => searchCalls === 1 && document.body.textContent?.includes("manual-manual-old.txt") === true), "manual-search fixture shows its initial result");
  generated = true;
  ok(await triggerManualTreeRefresh(), "manual search refresh action is available");
  ok(await eventually(() => searchCalls === 2), "manual file-tree refresh reruns the active search");
  ok(await eventually(() => document.body.textContent?.includes("manual-manual-new.xlsx") === true), "manual file-tree refresh shows the new filtered workbook");

  await closeFixture(root, dom);
}

{
  const dom = installDom();
  let searchCalls = 0;
  let resolveStale: ((entries: DirEntry[]) => void) | undefined;
  let resolveFresh: ((entries: DirEntry[]) => void) | undefined;
  installWorkspaceBindings(
    async () => [],
    async (): Promise<DirEntry[]> => {
      searchCalls += 1;
      return new Promise<DirEntry[]>((resolve) => {
        if (searchCalls === 1) resolveStale = resolve;
        else resolveFresh = resolve;
      });
    },
  );
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);

  await renderPanel(root, 0);
  ok(await setWorkspaceFilter("manual-pending"), "manual pending-search fixture keeps an active filter");
  ok(await eventually(() => searchCalls === 1 && Boolean(resolveStale)), "manual pre-refresh search request is pending");
  ok(await triggerManualTreeRefresh(), "manual pending-search refresh action is available");
  const freshSearchStarted = await eventually(() => searchCalls === 2 && Boolean(resolveFresh));
  ok(freshSearchStarted, "manual refresh starts a fresh filtered search");
  await act(async () => {
    resolveStale?.([{ name: "manual-pending-stale-before-refresh.xlsx", isDir: false }]);
    await flushPromises();
  });
  ok(document.body.textContent?.includes("manual-pending-stale-before-refresh.xlsx") === false, "manual refresh ignores its pre-refresh pending search response");
  if (resolveFresh) {
    await act(async () => {
      resolveFresh?.([{ name: "manual-pending-fresh-after-refresh.xlsx", isDir: false }]);
      await flushPromises();
    });
  }
  ok(await eventually(() => document.body.textContent?.includes("manual-pending-fresh-after-refresh.xlsx") === true), "manual refresh accepts the fresh search response");

  await closeFixture(root, dom);
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
