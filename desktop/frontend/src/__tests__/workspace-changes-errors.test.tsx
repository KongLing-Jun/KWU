// Run: tsx src/__tests__/workspace-changes-errors.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { WorkspacePanel } from "../components/WorkspacePanel";
import { LocaleProvider } from "../lib/i18n";
import { recordClientError, type AppBindings, type ClientErrorRecord } from "../lib/bridge";
import type { DirEntry, FilePreview, GitCommitView, WorkspaceChangesView } from "../lib/types";
import { installClientErrorLocalSink } from "../lib/clientError";
import { resolveWorkspaceFileOwner } from "../lib/workspaceFileScope";

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

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await act(async () => {
      await flushPromises();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
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

async function renderWorkspace(
  changes: WorkspaceChangesView,
  history: AppBindings["WorkspaceGitHistory"] = async (): Promise<GitCommitView[]> => [],
  listDir: AppBindings["ListDir"] = async () => [],
  initialViewMode: "files" | "changed" = "changed",
) {
  const dom = installDom();
  window.go = {
    main: {
      App: {
        ListDir: listDir,
        ListDirForTab: async (_tabID: string, rel: string) => listDir(rel),
        ReadFile: async () => ({ path: "src/app.ts", body: "", size: 0, truncated: false, binary: false }),
        ReadFileForTab: async () => ({ path: "src/app.ts", body: "", size: 0, truncated: false, binary: false }),
        WorkspaceGitHistory: history,
        WorkspaceChanges: async () => changes,
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <WorkspacePanel
          open
          tabId="tab-a"
          cwd="/repo"
          maximized={false}
          initialViewMode={initialViewMode}
          onClose={() => {}}
          onToggleMaximized={() => {}}
        />
      </LocaleProvider>,
    );
    await flushPromises();
  });
  await waitFor("workspace shell", () => Boolean(document.querySelector(initialViewMode === "files" ? ".workspace-tree" : ".workspace-preview__body")));
  return { dom, root };
}

console.log("\nworkspace changes git errors");

ok(resolveWorkspaceFileOwner(undefined, {}).kind === "none", "workspace ownership resolver fails closed by default");
ok(
  resolveWorkspaceFileOwner(undefined, { allowActiveWorkspace: true }).kind === "active",
  "legacy workspace-panel access opts into the active workspace explicitly",
);

{
  const { dom, root } = await renderWorkspace({ files: [], gitAvailable: false });
await waitFor("git unavailable warning", () => document.body.textContent?.includes("当前工作区不可读取 Git 状态。") === true);
ok(document.body.textContent?.includes("当前工作区不可读取 Git 状态。") === true, "gitAvailable=false renders a warning");
ok(document.body.textContent?.includes("暂无改动文件") === false, "gitAvailable=false is not shown as a clean workspace");
  const viewTabs = Array.from(document.querySelectorAll(".workspace-files__tabs button[role='tab']")) as HTMLButtonElement[];
  ok(viewTabs.length === 2, "workspace view tabs render real tab buttons");
  ok(viewTabs.filter((button) => button.getAttribute("aria-selected") === "true").length === 1, "workspace view tabs expose a single selected state");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const { dom, root } = await renderWorkspace({
    files: [],
    gitAvailable: true,
    gitErr: "git status timed out",
  });
await waitFor("git error warning without files", () => document.body.textContent?.includes("当前工作区不可读取 Git 状态。") === true);
ok(document.body.textContent?.includes("当前工作区不可读取 Git 状态。") === true, "gitErr without files renders a warning");
ok(document.body.textContent?.includes("暂无改动文件") === false, "empty files plus gitErr is not shown as a clean workspace");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const { dom, root } = await renderWorkspace({
    files: [
      {
        path: "src/app.ts",
        sources: ["session"],
        gitStatus: "modified",
        latestPrompt: "edit app",
      },
    ],
    gitAvailable: true,
    gitErr: "git status timed out",
  });
  await waitFor("git error warning with files", () => document.body.textContent?.includes("app.ts") === true);
ok(document.body.textContent?.includes("当前工作区不可读取 Git 状态。") === true, "gitErr renders a warning");
  ok(document.body.textContent?.includes("app.ts") === true, "files still render when gitErr is present");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const { dom, root } = await renderWorkspace(
    {
      files: [
        {
          path: "src/app.ts",
          sources: ["session"],
          gitStatus: "modified",
        },
      ],
      gitAvailable: true,
    },
    async () => {
      throw new Error("git log timed out");
    },
  );
  await waitFor("changed file row", () => document.body.textContent?.includes("app.ts") === true);
  const fileButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes("app.ts"));
  if (!fileButton) throw new Error("missing changed file button");
  await act(async () => {
    fileButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
await waitFor("git history error", () => document.body.textContent?.includes("无法确认当前改动文件。") === true);
ok(document.body.textContent?.includes("暂无改动文件") === false, "selected file git history errors are not shown as clean changes");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const { dom, root } = await renderWorkspace(
    { files: [], gitAvailable: true },
    async () => [],
    async () => {
      throw new Error("list dir failed");
    },
    "files",
  );
await waitFor("file tree error", () => document.body.textContent?.includes("无法加载文件树。") === true);
ok(document.body.textContent?.includes("无法加载文件树。") === true, "ListDir failures render a file tree error");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const listCalls: Array<{ tabID: string; dir: string; resolve: (entries: DirEntry[]) => void }> = [];
  window.go = {
    main: {
      App: {
        ListDir: async () => {
          throw new Error("active ListDir must not run for an owning tab");
        },
        ListDirForTab: async (tabID: string, dir: string) => new Promise<DirEntry[]>((resolve) => listCalls.push({ tabID, dir, resolve })),
        ReadFile: async () => ({ path: "ignored.ts", body: "", size: 0, truncated: false, binary: false }),
        ReadFileForTab: async () => ({ path: "ignored.ts", body: "", size: 0, truncated: false, binary: false }),
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const renderPanel = async (tabId: string) => {
    await act(async () => {
      root.render(
        <LocaleProvider>
          <WorkspacePanel
            open
            tabId={tabId}
            cwd="/same-workspace"
            maximized={false}
            initialViewMode="files"
            onClose={() => {}}
            onToggleMaximized={() => {}}
          />
        </LocaleProvider>,
      );
      await flushPromises();
    });
  };

  await renderPanel("tab-a");
  await waitFor("tab-a list request", () => listCalls.length === 1);
  await renderPanel("tab-b");
  await waitFor("tab-b list request", () => listCalls.length >= 2);
  const currentListCall = listCalls[listCalls.length - 1];
  await act(async () => {
    listCalls[0].resolve([{ name: "old-project.ts", isDir: false }]);
    await flushPromises();
  });
  ok(document.querySelector(".workspace-tree__sizer") === null, "late ListDir from old tab is ignored");
  await act(async () => {
    currentListCall.resolve([
      { name: "new-project-a.ts", isDir: false },
      { name: "new-project-b.ts", isDir: false },
    ]);
    await flushPromises();
  });
  await waitFor("new tab tree rows", () => (document.querySelector(".workspace-tree__sizer") as HTMLElement | null)?.style.height === "48px");
  ok((document.querySelector(".workspace-tree__sizer") as HTMLElement | null)?.style.height === "48px", "current tab ListDir result is rendered");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const readCalls: Array<{ tabID: string; path: string; resolve: (preview: FilePreview) => void }> = [];
  window.go = {
    main: {
      App: {
        ListDir: async () => {
          throw new Error("active ListDir must not run for an owning tab");
        },
        ListDirForTab: async () => [{ name: "shared.ts", isDir: false }],
        ReadFile: async () => {
          throw new Error("active ReadFile must not run for an owning tab");
        },
        ReadFileForTab: async (tabID: string, path: string) => new Promise<FilePreview>((resolve) => readCalls.push({ tabID, path, resolve })),
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const renderPanel = async (tabId: string) => {
    await act(async () => {
      root.render(
        <LocaleProvider>
          <WorkspacePanel
            open
            tabId={tabId}
            cwd="/same-workspace"
            maximized={false}
            initialViewMode="files"
            revealPathRequest={{ id: 1, path: "shared.ts" }}
            onClose={() => {}}
            onToggleMaximized={() => {}}
          />
        </LocaleProvider>,
      );
      await flushPromises();
    });
  };

  await renderPanel("tab-a");
  await waitFor("tab-a read request", () => readCalls.length === 1);
  await renderPanel("tab-b");
  await waitFor("tab-b read request", () => readCalls.length >= 2);
  const currentReadCall = readCalls[readCalls.length - 1];
  await act(async () => {
    readCalls[0].resolve({ path: "shared.ts", body: "OLD TAB PREVIEW", size: 15, truncated: false, binary: false });
    await flushPromises();
  });
  ok(document.body.textContent?.includes("OLD TAB PREVIEW") === false, "late ReadFile from old tab is ignored");
  await act(async () => {
    currentReadCall.resolve({ path: "shared.ts", body: "NEW TAB PREVIEW", size: 15, truncated: false, binary: false });
    await flushPromises();
  });
  await waitFor("new tab preview", () => document.body.textContent?.includes("NEW TAB PREVIEW") === true);
  ok(document.body.textContent?.includes("NEW TAB PREVIEW") === true, "current tab ReadFile result is rendered");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const listCalls: Array<[string, string]> = [];
  const searchCalls: Array<[string, string]> = [];
  const readCalls: Array<[string, string]> = [];
  const revealCalls: Array<[string, string]> = [];
  const chatAdds: string[] = [];
  window.go = {
    main: {
      App: {
        ListDir: async () => {
          throw new Error("active ListDir must not run for an owning tab");
        },
        ListDirForTab: async (tabID: string, dir: string) => {
          listCalls.push([tabID, dir]);
          return [{ name: "owned.xlsx", isDir: false }];
        },
        SearchFileRefs: async () => {
          throw new Error("active SearchFileRefs must not run for an owning tab");
        },
        SearchFileRefsForTab: async (tabID: string, query: string) => {
          searchCalls.push([tabID, query]);
          return [{ name: "owned.xlsx", isDir: false }];
        },
        ReadFile: async () => {
          throw new Error("active ReadFile must not run for an owning tab");
        },
        ReadFileForTab: async (tabID: string, path: string) => {
          readCalls.push([tabID, path]);
          return { path, body: "owned tab body", size: 14, truncated: false, binary: false };
        },
        RevealWorkspacePath: async () => {
          throw new Error("active RevealWorkspacePath must not run for an owning tab");
        },
        RevealWorkspacePathForTab: async (tabID: string, path: string) => {
          revealCalls.push([tabID, path]);
        },
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <WorkspacePanel
          open
          tabId="tab-b"
          cwd="/same-workspace"
          maximized={false}
          initialViewMode="files"
          onClose={() => {}}
          onToggleMaximized={() => {}}
          onAddToChat={(text) => chatAdds.push(text)}
        />
      </LocaleProvider>,
    );
    await flushPromises();
  });
  await waitFor("owning tab list call", () => listCalls.length > 0);
  ok(JSON.stringify(listCalls) === JSON.stringify([["tab-b", ""]]), "file tree lists the owning tab workspace instead of the active workspace");
  await waitFor("owning tab file tree", () => document.body.textContent?.includes("owned.xlsx") === true);

  const filterInput = document.querySelector(".workspace-search input") as HTMLInputElement | null;
  if (!filterInput) throw new Error("missing workspace filter input");
  await act(async () => {
    const setInputValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, "value")?.set;
    if (!setInputValue) throw new Error("missing input value setter");
    setInputValue.call(filterInput, "owned");
    const reactPropsKey = Object.keys(filterInput).find((key) => key.startsWith("__reactProps$"));
    const reactProps = reactPropsKey
      ? (filterInput as unknown as Record<string, { onChange?: (event: { target: HTMLInputElement }) => void }>)[reactPropsKey]
      : undefined;
    if (!reactProps?.onChange) throw new Error("missing React input change handler");
    reactProps.onChange({ target: filterInput });
    await flushPromises();
  });
  await waitFor("owning tab search", () => searchCalls.length === 1);
  ok(JSON.stringify(searchCalls[0]) === JSON.stringify(["tab-b", "owned"]), "file search uses the owning tab workspace");

  const ownedRow = document.querySelector('[data-workspace-path="owned.xlsx"]') as HTMLButtonElement | null;
  if (!ownedRow) throw new Error("missing owned file row");
  await act(async () => {
    ownedRow.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("owning tab preview", () => document.body.textContent?.includes("owned tab body") === true);
  ok(JSON.stringify(readCalls[0]) === JSON.stringify(["tab-b", "owned.xlsx"]), "file preview reads from the owning tab workspace");

  const openOwnedMenu = async () => {
    await act(async () => {
      ownedRow.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: 10, clientY: 10 }));
      await flushPromises();
    });
    await waitFor("owned file context menu", () => document.querySelectorAll(".workspace-tree-menu button").length === 3);
    return Array.from(document.querySelectorAll(".workspace-tree-menu button")) as HTMLButtonElement[];
  };
  let menuButtons = await openOwnedMenu();
const addContentsButton = menuButtons.find((button) => button.textContent?.trim() === "添加文件内容");
  if (!addContentsButton) throw new Error("missing Add file contents menu item");
  await act(async () => {
    addContentsButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("owning tab file content", () => chatAdds.some((text) => text.includes("owned tab body")));
  ok(JSON.stringify(readCalls[1]) === JSON.stringify(["tab-b", "owned.xlsx"]), "add-file-content reads from the owning tab workspace");

  menuButtons = await openOwnedMenu();
const revealButton = menuButtons.find((button) => button.textContent?.trim() === "在文件管理器中显示");
  if (!revealButton) throw new Error("missing Show in file manager menu item");
  await act(async () => {
    revealButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("owning tab reveal", () => revealCalls.length === 1);
  ok(JSON.stringify(revealCalls[0]) === JSON.stringify(["tab-b", "owned.xlsx"]), "reveal uses the owning tab workspace");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const chatAdds: string[] = [];
  const readCalls: Array<[string, string]> = [];
  let resolveDelayedRead: ((preview: FilePreview) => void) | undefined;
  const delayedRead = new Promise<FilePreview>((resolve) => {
    resolveDelayedRead = resolve;
  });
  window.go = {
    main: {
      App: {
        ListDir: async () => {
          throw new Error("active ListDir must not run for an owning tab");
        },
        ListDirForTab: async () => [{ name: "owned.xlsx", isDir: false }],
        ReadFile: async () => {
          throw new Error("active ReadFile must not run for an owning tab");
        },
        ReadFileForTab: async (tabID: string, path: string) => {
          readCalls.push([tabID, path]);
          if (readCalls.length === 1) {
            return { path, body: "tab-a preview", size: 13, truncated: false, binary: false };
          }
          if (readCalls.length === 2) return delayedRead;
          return { path, body: "tab-b preview", size: 13, truncated: false, binary: false };
        },
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const renderPanel = async (tabId: string) => {
    await act(async () => {
      root.render(
        <LocaleProvider>
          <WorkspacePanel
            open
            tabId={tabId}
            cwd="/same-workspace"
            maximized={false}
            initialViewMode="files"
            onClose={() => {}}
            onToggleMaximized={() => {}}
            onAddToChat={(text) => chatAdds.push(text)}
          />
        </LocaleProvider>,
      );
      await flushPromises();
    });
  };

  await renderPanel("tab-a");
  await waitFor("stale-add file row", () => Boolean(document.querySelector('[data-workspace-path="owned.xlsx"]')));
  const ownedRow = document.querySelector('[data-workspace-path="owned.xlsx"]') as HTMLButtonElement;
  await act(async () => {
    ownedRow.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("tab-a preview", () => document.body.textContent?.includes("tab-a preview") === true);
  await act(async () => {
    ownedRow.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: 10, clientY: 10 }));
    await flushPromises();
  });
  await waitFor("stale-add context menu", () => document.querySelectorAll(".workspace-tree-menu button").length === 3);
  const addContentsButton = Array.from(document.querySelectorAll(".workspace-tree-menu button"))
  .find((button) => button.textContent?.trim() === "添加文件内容") as HTMLButtonElement | undefined;
  if (!addContentsButton) throw new Error("missing Add file contents menu item");
  await act(async () => {
    addContentsButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("delayed add-file read", () => readCalls.length === 2);

  await renderPanel("tab-b");
  await waitFor("tab-b preview request", () => readCalls.some(([tabID]) => tabID === "tab-b"));
  await act(async () => {
    resolveDelayedRead?.({ path: "owned.xlsx", body: "STALE TAB-A CONTENT", size: 19, truncated: false, binary: false });
    await flushPromises();
    await flushPromises();
  });
  ok(chatAdds.length === 0, "late add-file-content result from the old tab is ignored");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const journalRecords: ClientErrorRecord[] = [];
  const readPaths: string[] = [];
  const invalidPreviewError = "invalid path C:\\secret\\preview-canary.txt";
  window.go = {
    main: {
      App: {
        ListDir: async () => [{ name: "preview-canary.txt", isDir: false }],
        ListDirForTab: async () => [{ name: "preview-canary.txt", isDir: false }],
        ReadFile: async () => ({ path: "preview-canary.txt", body: "", size: 0, truncated: false, binary: false }),
        ReadFileForTab: async (_tabID: string, path: string) => {
          readPaths.push(path);
          if (readPaths.length === 1) {
            return { path, body: "", size: 0, truncated: false, binary: false, err: invalidPreviewError };
          }
          throw new Error(invalidPreviewError);
        },
        RecordClientError: async (record: ClientErrorRecord) => {
          journalRecords.push(record);
        },
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
  installClientErrorLocalSink(recordClientError);
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing preview error root");
  const root = createRoot(rootElement);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <WorkspacePanel
          open
          tabId="tab-preview"
          cwd="/same-workspace"
          maximized={false}
          initialViewMode="files"
          onClose={() => {}}
          onToggleMaximized={() => {}}
        />
      </LocaleProvider>,
    );
    await flushPromises();
  });
  await waitFor("preview canary row", () => Boolean(document.querySelector('[data-workspace-path="preview-canary.txt"]')));
  const row = document.querySelector('[data-workspace-path="preview-canary.txt"]') as HTMLButtonElement | null;
  if (!row) throw new Error("missing preview canary row");
  await act(async () => {
    row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("returned preview error report", () => journalRecords.length === 1 && document.body.textContent?.includes(invalidPreviewError) === true);
  ok(journalRecords.length === 1, "successful FilePreview.err is reported once");
  ok(document.body.textContent?.includes(invalidPreviewError) === true, "returned preview error keeps the existing UI message");
  ok(journalRecords[0]?.scope === "workspace.preview", "returned preview error uses the workspace preview scope");
  ok(!JSON.stringify(journalRecords[0]).includes("preview-canary.txt"), "returned preview error journal DTO omits the raw path");

  await act(async () => {
    row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
    row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("rejected preview error report", () => journalRecords.length === 2);
  ok(journalRecords.length === 2, "rejected preview read is not double-reported");
  await act(async () => {
    root.unmount();
  });
  installClientErrorLocalSink();
  dom.window.close();
}

{
  const dom = installDom();
  let activeListCalls = 0;
  let activeReadCalls = 0;
  window.go = {
    main: {
      App: {
        ListDir: async () => {
          activeListCalls += 1;
          return [{ name: "legacy-active.txt", isDir: false }];
        },
        ListDirForTab: async () => {
          throw new Error("tab-scoped ListDir must not run without a tab owner");
        },
        ReadFile: async (path: string) => {
          activeReadCalls += 1;
          return { path, body: "legacy active preview", size: 21, truncated: false, binary: false };
        },
        ReadFileForTab: async () => {
          throw new Error("tab-scoped ReadFile must not run without a tab owner");
        },
        WorkspaceGitHistory: async () => [],
        WorkspaceChanges: async () => ({ files: [], gitAvailable: true }),
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing legacy workspace root");
  const root = createRoot(rootElement);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <WorkspacePanel
          open
          cwd="/legacy-active"
          maximized={false}
          initialViewMode="files"
          onClose={() => {}}
          onToggleMaximized={() => {}}
        />
      </LocaleProvider>,
    );
    await flushPromises();
  });
  await waitFor("legacy active row", () => Boolean(document.querySelector('[data-workspace-path="legacy-active.txt"]')));
  const row = document.querySelector('[data-workspace-path="legacy-active.txt"]') as HTMLButtonElement;
  await act(async () => {
    row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();
  });
  await waitFor("legacy active preview", () => document.body.textContent?.includes("legacy active preview") === true);
  ok(activeListCalls > 0, "ownerless legacy workspace panel lists the explicitly opted-in active workspace");
  ok(activeReadCalls > 0, "ownerless legacy workspace panel previews from the explicitly opted-in active workspace");
  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
