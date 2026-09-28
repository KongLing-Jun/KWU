import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import userEvent from "@testing-library/user-event";
import { MCPServersSettingsPage, SkillsSettingsPage } from "../components/CapabilitiesPanel";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { mcpServerLifecycleActions, mcpServerRetryableFromAvailableList } from "../lib/mcpServerLifecycle";
import type { Meta, PublicSkillView, ServerView, TabMeta } from "../lib/types";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

function server(status: ServerView["status"]): ServerView {
  return {
    name: "codegraph",
    transport: "stdio",
    status,
    configured: true,
    autoStart: true,
    tier: "background",
    tools: 0,
    prompts: 0,
    resources: 0,
  };
}

function publicSkill(name: string, categories: string[]): PublicSkillView {
  const slug = name.toLowerCase().replace(/\s+/g, "-");
  return {
    category: "",
    categories,
    claim_state: "",
    claimable: false,
    created_at: "",
    description: `${name} description`,
    downloads: 0,
    homepage: "",
    installs: 0,
    install_count: 0,
    labels: [],
    name,
    namespace: "test",
    ownerName: "Test",
    publisher: "Test",
    score: 0,
    slug,
    source: "SkillHub",
    type: "user",
    stars: 0,
    subCategories: [],
    tags: [],
    updated_at: "",
    verified: false,
    version: "1.0.0",
    download_url: "",
    download_filename: "",
    download_size_bytes: 0,
    download_sha256: "",
  };
}

const initializing = mcpServerLifecycleActions(server("initializing"));
ok(initializing.enabled, "initializing server should still be treated as enabled");
ok(!initializing.showRetryInRow, "initializing server should not expose retry until it fails");
ok(!initializing.canReconnect, "initializing server should not expose reconnect while already connecting");
ok(!initializing.canConnectNow, "initializing server should not use the deferred connect-now action");

const connected = mcpServerLifecycleActions(server("connected"));
ok(!connected.showRetryInRow, "connected server row should keep the toggle UI");
ok(connected.canReconnect, "connected server details should expose reconnect");

const manuallyConnected = mcpServerLifecycleActions({ ...server("connected"), autoStart: false, startIntent: "off", runtimeState: "ready" });
ok(manuallyConnected.enabled, "connected manual server should still render as enabled");
ok(!manuallyConnected.canConnectNow, "connected manual server should not expose connect-now");
ok(manuallyConnected.canReconnect, "connected manual server should expose reconnect");

const automaticIdle = mcpServerLifecycleActions({ ...server("deferred"), startIntent: "automatic" });
ok(!automaticIdle.canConnectNow, "automatic idle server should not look like a manual connector");
ok(!automaticIdle.canReconnect, "automatic idle server should wait for background connection or failure");

const failed = mcpServerLifecycleActions({ ...server("failed"), runtimeState: "issue" });
ok(failed.showRetryInRow, "failed server row should expose retry");

ok(mcpServerRetryableFromAvailableList(server("initializing")), "connecting server should be included in available-list retry all");
ok(mcpServerRetryableFromAvailableList({ ...server("deferred"), startIntent: "automatic" }), "automatic idle server should be included in available-list retry all");
ok(!mcpServerRetryableFromAvailableList(server("connected")), "connected server should be excluded from available-list retry all");
ok(!mcpServerRetryableFromAvailableList({ ...server("disabled"), startIntent: "off" }), "disabled server should be excluded from available-list retry all");
ok(!mcpServerRetryableFromAvailableList({ ...server("failed"), runtimeState: "issue" }), "failed server is handled by the failure banner retry all");

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
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

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><header class=\"capability-workspace-header\"><div id=\"capability-workspace-header-actions\" class=\"capability-workspace-header__actions\"></div></header><div id=\"root\"></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  Object.defineProperties(dom.window.HTMLElement.prototype, {
    attachEvent: { configurable: true, value: () => undefined },
    detachEvent: { configurable: true, value: () => undefined },
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.Node = dom.window.Node;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.Event = dom.window.Event;
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
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

function findButton(label: string): HTMLButtonElement | undefined {
  return Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.trim() === label) as HTMLButtonElement | undefined;
}

console.log("capabilities panel MCP actions");

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "test-channel", cwd: "/tmp/kwu-test", workspaceRoot: "/tmp/kwu-test" };
  const tabs: TabMeta[] = [{
    id: "tab-1",
    scope: "project",
    workspaceRoot: "/tmp/kwu-test",
    workspaceName: "kwu-test",
    topicId: "topic-1",
    topicTitle: "Test",
    label: "Test",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/kwu-test",
  }];
  let trustCalls = 0;
  let bulkTrustCalls = 0;
  let untrustCalls = 0;
  let servers: ServerView[] = [{
    name: "github",
    transport: "stdio",
    status: "connected",
    configured: true,
    autoStart: true,
    tools: 2,
    prompts: 0,
    resources: 0,
    toolList: [
      { name: "issue_read", description: "Read issues.", readOnlyHint: true },
      { name: "issue_write", description: "Write issues." },
    ],
    trustedReadOnlyTools: [],
  }];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        MCPServers: async () => servers.map((s) => ({
          ...s,
          toolList: s.toolList?.map((tool) => ({ ...tool })),
          trustedReadOnlyTools: [...(s.trustedReadOnlyTools ?? [])],
        })),
        ConnectorCatalog: async () => ({ catalog_version: "test", items: [] }),
        ConnectorStates: async () => [],
        TrustMCPServerTool: async (name: string, toolName: string) => {
          trustCalls += 1;
          servers = servers.map((s) => s.name === name ? { ...s, trustedReadOnlyTools: [...(s.trustedReadOnlyTools ?? []), toolName] } : s);
        },
        TrustMCPServerTools: async (name: string, toolNames: string[]) => {
          bulkTrustCalls += 1;
          servers = servers.map((s) => {
            if (s.name !== name) return s;
            const trusted = Array.from(new Set([...(s.trustedReadOnlyTools ?? []), ...toolNames]));
            return { ...s, trustedReadOnlyTools: trusted };
          });
        },
        UntrustMCPServerTool: async (name: string, toolName: string) => {
          untrustCalls += 1;
          servers = servers.map((s) => s.name === name ? { ...s, trustedReadOnlyTools: (s.trustedReadOnlyTools ?? []).filter((tool) => tool !== toolName) } : s);
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(MCPServersSettingsPage)));
    await flush();
  });
  await waitFor("github server row", () => Boolean(document.querySelector(".cap-row__name")?.textContent?.includes("github")));

  const disclosure = document.querySelector<HTMLButtonElement>(".cap-disclosure");
  if (!disclosure) throw new Error("missing MCP disclosure button");
  ok(disclosure.getAttribute("aria-label") === "展开详情", "MCP disclosure button has an accessible name");
  const serverToggle = document.querySelector<HTMLInputElement>(".cap-row__actions .cap-switch input[type='checkbox']");
  if (!serverToggle) throw new Error("missing MCP server toggle checkbox");
  ok(serverToggle.getAttribute("aria-label") === "本次会话停用 github", "MCP server toggle checkbox has an accessible name");
  await act(async () => {
    disclosure.click();
    await flush();
  });

  const trustReadOnly = findButton("信任只读（1）");
  if (!trustReadOnly) throw new Error("missing bulk Trust read-only button");
  await act(async () => {
    trustReadOnly.click();
    await flush();
  });
  await waitFor("bulk trusted tool", () => servers[0]?.trustedReadOnlyTools?.includes("issue_read") ?? false);

  const viewTools = findButton("查看工具");
  if (!viewTools) throw new Error("missing View tools button");
  await act(async () => {
    viewTools.click();
    await flush();
  });

  await waitFor("trusted badge", () => Boolean(document.querySelector(".cap-tool-trust")?.textContent?.includes("已信任")));
  const untrust = findButton("取消信任");
  if (!untrust) throw new Error("missing Untrust button");
  await act(async () => {
    untrust.click();
    await flush();
  });
  await waitFor("untrusted tool", () => !(servers[0]?.trustedReadOnlyTools?.includes("issue_read") ?? false));

  await waitFor("Trust button", () => Boolean(findButton("信任")));
  const trust = findButton("信任");
  if (!trust) throw new Error("missing Trust button");
  await act(async () => {
    trust.click();
    await flush();
  });

  await waitFor("trusted badge", () => Boolean(document.querySelector(".cap-tool-trust")?.textContent?.includes("已信任")));
  ok(bulkTrustCalls === 1, "clicking Trust read-only invokes the MCP bulk trust action once");
  ok(untrustCalls === 1, "clicking Untrust invokes the MCP untrust action once");
  ok(trustCalls === 1, "clicking Trust invokes the MCP trust action once");
  ok(servers[0]?.trustedReadOnlyTools?.includes("issue_read") ?? false, "trusted raw tool name is added to the server snapshot");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  rootEl.className = "settings-center__content";
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "skills-pagination-channel", cwd: "/tmp/suiyuan-skills-pagination", workspaceRoot: "/tmp/suiyuan-skills-pagination" };
  const tabs: TabMeta[] = [{
    id: "tab-skills-pagination",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-skills-pagination",
    workspaceName: "suiyuan-skills-pagination",
    topicId: "topic-skills-pagination",
    topicTitle: "Skills pagination",
    label: "Skills pagination",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-skills-pagination",
  }];
  const publicSkillsCalls: Array<[number, string]> = [];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
        PublicSkills: async (limit: number, cursor: string) => {
          publicSkillsCalls.push([limit, cursor]);
          if (!cursor) return { items: [publicSkill("First Page", [])], next_cursor: "@test/first-page", has_more: true };
          return { items: [publicSkill("Second Page", [])], next_cursor: "", has_more: false };
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(SkillsSettingsPage)));
    await flush();
  });
  await waitFor("all public skill pages", () => publicSkillsCalls.length === 2 && Boolean(document.body.textContent?.includes("Second Page")));
  ok(publicSkillsCalls[0]?.[0] === 20 && publicSkillsCalls[0]?.[1] === "", "catalog initially requests exactly 20 public skills without a cursor");
  ok(publicSkillsCalls[1]?.[0] === 20 && publicSkillsCalls[1]?.[1] === "@test/first-page", "catalog automatically requests the next page with its cursor");
  const catalogText = Array.from(document.querySelectorAll<HTMLElement>(".skill-library-grid:not(.skill-library__featured-grid) .skill-library-card")).map((card) => card.textContent ?? "").join("\n");
  ok(catalogText.includes("First Page") && catalogText.includes("Second Page"), "catalog automatically appends every page to the existing skills");
  ok(!document.querySelector(".skill-library__load-more"), "catalog does not display a scroll-to-load control after all pages finish loading");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  rootEl.className = "primary-workspace primary-workspace--skills";
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "skills-workspace-pagination-channel", cwd: "/tmp/suiyuan-skills-workspace-pagination", workspaceRoot: "/tmp/suiyuan-skills-workspace-pagination" };
  const tabs: TabMeta[] = [{
    id: "tab-skills-workspace-pagination",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-skills-workspace-pagination",
    workspaceName: "suiyuan-skills-workspace-pagination",
    topicId: "topic-skills-workspace-pagination",
    topicTitle: "Skills workspace pagination",
    label: "Skills workspace pagination",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-skills-workspace-pagination",
  }];
  const publicSkillsCalls: Array<[number, string]> = [];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
        PublicSkills: async (limit: number, cursor: string) => {
          publicSkillsCalls.push([limit, cursor]);
          if (!cursor) return { items: [publicSkill("First Page", [])], next_cursor: "@test/first-page", has_more: true };
          return { items: [publicSkill("Second Page", [])], next_cursor: "", has_more: false };
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(SkillsSettingsPage, { workspace: true })));
    await flush();
  });
  await waitFor("workspace automatically loads all public skill pages", () => publicSkillsCalls.length === 2 && Boolean(document.body.textContent?.includes("Second Page")));
  ok(publicSkillsCalls[1]?.[1] === "@test/first-page", "workspace catalog automatically requests the next cursor without wheel input");

  ok(!document.querySelector(".skill-library__load-more"), "workspace catalog has no scroll-to-load control after all pages finish loading");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "skills-categories-channel", cwd: "/tmp/suiyuan-skills-categories", workspaceRoot: "/tmp/suiyuan-skills-categories" };
  const tabs: TabMeta[] = [{
    id: "tab-skills-categories",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-skills-categories",
    workspaceName: "suiyuan-skills-categories",
    topicId: "topic-skills-categories",
    topicTitle: "Skills categories",
    label: "Skills categories",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-skills-categories",
  }];
  const searchCalls: Array<[string, number, string]> = [];
  const remoteSearchMatch = publicSkill("Remote Match", ["远程分类"]);
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        SkillsSettings: async () => ({ skills: [], skillRoots: [] }),
        PublicSkills: async () => ({
          items: [
            publicSkill("Spreadsheet", ["表格处理", "数据分析", "办公自动化"]),
            publicSkill("Writer", ["内容创作"]),
            publicSkill("Analytics", ["数据分析"]),
            publicSkill("Planner", ["办公自动化"]),
            publicSkill("Translator", ["内容创作"]),
          ],
          next_cursor: "",
          has_more: false,
        }),
        SearchPublicSkills: async (keyword: string, limit: number, cursor: string) => {
          searchCalls.push([keyword, limit, cursor]);
          return { items: [remoteSearchMatch], next_cursor: "", has_more: false };
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(SkillsSettingsPage, { workspace: true })));
    await flush();
  });
  await waitFor("public skill categories", () => document.querySelectorAll(".skill-library__categories button").length === 5);

  ok(document.querySelector("#featured-skills-title")?.textContent === "精选技能", "the default catalog view adds a featured skills section above the full catalog");
  ok(document.querySelectorAll(".skill-library__featured-grid .skill-library-card").length === 4, "featured skills use the reference four-card row");
  ok(document.querySelector(".skill-library__actions")?.textContent?.includes("我安装的"), "the installed shortcut is placed in the top-right action group");
  ok(document.querySelector("#capability-workspace-header-actions > .skill-library__topbar"), "workspace actions are portaled into the shared capability header");
  const categoryScroller = document.querySelector<HTMLDivElement>(".skill-library__categories");
  if (!categoryScroller) throw new Error("missing skill category scroller");
  Object.defineProperties(categoryScroller, {
    clientWidth: { configurable: true, value: 200 },
    scrollWidth: { configurable: true, value: 600 },
    scrollLeft: { configurable: true, value: 0, writable: true },
  });
  let capturedCategoryPointer: number | null = null;
  Object.defineProperties(categoryScroller, {
    setPointerCapture: { configurable: true, value: (pointerId: number) => { capturedCategoryPointer = pointerId; } },
    hasPointerCapture: { configurable: true, value: (pointerId: number) => capturedCategoryPointer === pointerId },
    releasePointerCapture: { configurable: true, value: (pointerId: number) => { if (capturedCategoryPointer === pointerId) capturedCategoryPointer = null; } },
  });
  const categoryWheel = new window.WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
  categoryScroller.dispatchEvent(categoryWheel);
  ok(categoryScroller.scrollLeft === 80 && categoryWheel.defaultPrevented, "vertical wheel input reveals hidden skill categories horizontally");
  categoryScroller.scrollLeft = 400;
  const categoryEdgeWheel = new window.WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 80 });
  categoryScroller.dispatchEvent(categoryEdgeWheel);
  ok(!categoryEdgeWheel.defaultPrevented, "wheel input returns to page scrolling at the category track boundary");
  const categoryDragTarget = document.querySelectorAll<HTMLButtonElement>(".skill-library__categories button")[2];
  if (!categoryDragTarget) throw new Error("missing skill category drag target");
  const dispatchCategoryPointer = (target: Element, type: string, clientX: number) => {
    const event = new window.Event(type, { bubbles: true, cancelable: true });
    Object.defineProperties(event, {
      button: { configurable: true, value: 0 },
      clientX: { configurable: true, value: clientX },
      pointerId: { configurable: true, value: 1 },
      pointerType: { configurable: true, value: "mouse" },
    });
    target.dispatchEvent(event);
    return event;
  };
  categoryScroller.scrollLeft = 100;
  dispatchCategoryPointer(categoryDragTarget, "pointerdown", 160);
  ok(capturedCategoryPointer === null, "pressing a skill category keeps pointer ownership on the button so a normal click can complete");
  const categoryDragLeft = dispatchCategoryPointer(categoryDragTarget, "pointermove", 80);
  ok(capturedCategoryPointer === 1, "pointer capture starts only after horizontal category dragging begins");
  dispatchCategoryPointer(categoryDragTarget, "pointerup", 80);
  ok(categoryScroller.scrollLeft === 180 && categoryDragLeft.defaultPrevented, "dragging left reveals later skill categories");
  categoryDragTarget.click();
  ok(categoryDragTarget.getAttribute("aria-selected") === "false", "finishing a category drag does not accidentally select its button");
  dispatchCategoryPointer(categoryScroller, "pointerdown", 80);
  dispatchCategoryPointer(categoryScroller, "pointermove", 160);
  dispatchCategoryPointer(categoryScroller, "pointerup", 160);
  ok(categoryScroller.scrollLeft === 100, "dragging right returns to earlier skill categories");
  ok(!categoryScroller.classList.contains("is-dragging"), "the grabbing state clears when category dragging ends");
  const firstFeaturedBefore = document.querySelector(".skill-library__featured-grid .skill-library-card")?.textContent;
  const shuffleFeatured = Array.from(document.querySelectorAll<HTMLButtonElement>(".skill-library__section-head button")).find((button) => button.textContent?.includes("换一换"));
  if (!shuffleFeatured) throw new Error("missing featured shuffle button");
  await act(async () => {
    shuffleFeatured.click();
    await flush();
  });
  ok(document.querySelector(".skill-library__featured-grid .skill-library-card")?.textContent !== firstFeaturedBefore, "featured skills can be rotated from the section header");

  const categoryLabels = Array.from(document.querySelectorAll<HTMLButtonElement>(".skill-library__categories button")).map((button) => button.textContent);
  ok(categoryLabels.join(",") === "全部,表格处理,数据分析,办公自动化,内容创作", "skill categories follow the API categories arrays in first-seen order");
  const dataAnalysis = Array.from(document.querySelectorAll<HTMLButtonElement>(".skill-library__categories button")).find((button) => button.textContent === "数据分析");
  if (!dataAnalysis) throw new Error("missing 数据分析 category");

  await act(async () => {
    dataAnalysis.click();
    await flush();
  });
  await waitFor("data analysis skill cards", () => document.querySelectorAll(".skill-library-grid:not(.skill-library__featured-grid) .skill-library-card").length === 2);

  ok(document.querySelector("#featured-skills-title")?.textContent === "精选技能", "featured skills remain visible after switching categories");
  ok(document.querySelectorAll(".skill-library__featured-grid .skill-library-card").length === 4, "category switches preserve the featured skill cards");
  const visibleCards = Array.from(document.querySelectorAll<HTMLElement>(".skill-library-grid:not(.skill-library__featured-grid) .skill-library-card")).map((card) => card.textContent ?? "").join("\n");
  ok(visibleCards.includes("Spreadsheet"), "a skill appears in each category listed by the API");
  ok(visibleCards.includes("Analytics"), "matching category skills remain visible");
  ok(!visibleCards.includes("Writer"), "non-matching category skills are hidden");
  ok(dataAnalysis.getAttribute("aria-selected") === "true", "selected skill category exposes its tab state");

  const searchInput = document.querySelector<HTMLInputElement>('input[aria-label="搜索技能"]');
  if (!searchInput) throw new Error("missing public skill search input");
  const user = userEvent.setup({ document });
  await act(async () => {
    await user.type(searchInput, "文档");
  });
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 300));
    await flush();
  });
  await waitFor("server-side public skill search request", () => searchCalls.length === 1);
  await waitFor("server-side public skill search result", () => document.body.textContent?.includes("Remote Match") === true);
  ok(searchCalls.length === 1 && searchCalls[0]?.[0] === "文档" && searchCalls[0]?.[1] === 20 && searchCalls[0]?.[2] === "", "catalog search requests the first 20 public skills without an ownership filter");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const item = publicSkill("Writer", ["内容创作"]);
  const meta: Meta = { label: "test", ready: true, eventChannel: "skills-direct-install-channel", cwd: "/tmp/skills-direct-install", workspaceRoot: "/tmp/skills-direct-install" };
  const tabs: TabMeta[] = [{
    id: "tab-skills-direct-install",
    scope: "project",
    workspaceRoot: "/tmp/skills-direct-install",
    workspaceName: "skills-direct-install",
    topicId: "topic-skills-direct-install",
    topicTitle: "Skills direct install",
    label: "Skills direct install",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/skills-direct-install",
  }];
  let installed = false;
  const installCalls: Array<[string, string]> = [];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        SkillsSettings: async () => ({
          skills: installed ? [{ name: item.slug, description: item.description, scope: "global", runAs: "skill", enabled: true }] : [],
          skillRoots: [],
        }),
        PublicSkills: async () => ({ items: [item], next_cursor: "", has_more: false }),
        InstallPublicSkill: async (namespace: string, slug: string) => {
          installCalls.push([namespace, slug]);
          installed = true;
          return { name: slug, namespace, slug, version: item.version, path: `/tmp/${slug}/SKILL.md`, reload_required: false };
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(SkillsSettingsPage)));
    await flush();
  });
  await waitFor("direct install button", () => Boolean(document.querySelector('button[aria-label="安装技能 Writer"]')));

  const installButton = document.querySelector<HTMLButtonElement>('button[aria-label="安装技能 Writer"]');
  if (!installButton) throw new Error("missing direct install button");
  await act(async () => {
    installButton.click();
    await flush();
  });
  await waitFor("installed skill state", () => Boolean(document.querySelector('button[aria-label="技能 Writer 已安装"]')));

  ok(installCalls.length === 1, "clicking the plus button installs the public skill once");
  ok(installCalls[0]?.[0] === "test" && installCalls[0]?.[1] === "writer", "the plus button installs the card's namespace and slug");
  ok(document.querySelector('[role="dialog"]') === null, "direct install does not open the skill detail dialog");
  ok(document.querySelector<HTMLButtonElement>('button[aria-label="技能 Writer 已安装"]')?.disabled, "successful direct install changes the plus button to a disabled installed state");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const meta: Meta = {
    label: "test",
    ready: true,
    eventChannel: "connector-dialog-channel",
    cwd: "/tmp/connector-dialog",
    workspaceRoot: "/tmp/connector-dialog",
  };
  const tabs: TabMeta[] = [{
    id: "tab-connector-dialog",
    scope: "project",
    workspaceRoot: "/tmp/connector-dialog",
    workspaceName: "connector-dialog",
    topicId: "topic-connector-dialog",
    topicTitle: "Connector dialog",
    label: "Connector dialog",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/connector-dialog",
  }];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        MCPServers: async () => [{
          name: "openaiDeveloperDocs",
          transport: "http",
          status: "connected",
          configured: true,
          autoStart: true,
          tools: 5,
          prompts: 0,
          resources: 0,
          url: "https://developers.openai.com/mcp",
        }],
        ConnectorCatalog: async () => ({ catalog_version: "test", items: [] }),
        ConnectorStates: async () => [],
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(
      LocaleProvider,
      null,
      React.createElement(MCPServersSettingsPage as React.ComponentType<{ workspace?: boolean }>, { workspace: true }),
    ));
    await flush();
  });
  await waitFor("workspace connector card", () => Boolean(document.querySelector(".cap-row__name")?.textContent?.includes("openaiDeveloperDocs")));

  ok(document.querySelector(".connector-configured-title")?.textContent?.includes("1"), "workspace connector count comes from the MCP server snapshot");
  ok(document.querySelector(".connector-configured-summary")?.textContent?.includes("5"), "workspace connector summary uses the MCP tool count");
  ok(!document.querySelector(".cap-server-entry .cap-server-details"), "workspace connector details should not expand inside the card");
  const details = findButton("详情");
  if (!details) throw new Error("missing connector detail button");
  await act(async () => {
    details.click();
    await flush();
  });

  await waitFor("connector detail dialog", () => Boolean(document.querySelector(".connector-detail-modal")));
  ok(document.querySelector(".connector-detail-modal")?.textContent?.includes("openaiDeveloperDocs"), "connector dialog shows the selected server");
  ok(Boolean(document.querySelector(".connector-detail-modal .cap-server-details")), "connector dialog reuses the existing server detail content");
  ok(!document.querySelector(".cap-server-entry .cap-server-details"), "opening the dialog must not grow the connector card");

  await act(async () => {
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    await flush();
  });
  await waitFor("connector detail dialog close", () => !document.querySelector(".connector-detail-modal"));

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  let chatCalls = 0;
  let uninstallCalls = 0;
  const backendInstalledSkill = { ...publicSkill("审计问题修复（后端）", ["开发工具"]), slug: "audit-fix" };
  const meta: Meta = { label: "test", ready: true, eventChannel: "skills-a11y-channel", cwd: "/tmp/suiyuan-skills-a11y", workspaceRoot: "/tmp/suiyuan-skills-a11y" };
  const tabs: TabMeta[] = [{
    id: "tab-skills-a11y",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-skills-a11y",
    workspaceName: "suiyuan-skills-a11y",
    topicId: "topic-skills-a11y",
    topicTitle: "Skills a11y",
    label: "Skills a11y",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-skills-a11y",
  }];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        SkillsSettings: async () => ({
          skills: [
            { name: "explore", description: "Explore the codebase.", scope: "builtin", runAs: "subagent", enabled: true },
            { name: "audit-fix", displayName: "审计问题修复", description: "Fix audit findings.", scope: "global", runAs: "skill", enabled: true, iconUrl: "https://cdn.example.com/skills/audit-fix.png", uninstallId: "audit-fix", uninstallable: true },
            { name: "removed-from-catalog", description: "No longer exists remotely.", scope: "global", runAs: "skill", enabled: false },
          ],
          skillRoots: [],
        }),
        PublicSkills: async () => ({ items: [backendInstalledSkill], next_cursor: "", has_more: false }),
        UninstallPublicSkill: async (slug: string) => { if (slug === "audit-fix") uninstallCalls += 1; },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(SkillsSettingsPage, { workspace: true, onSelectSkill: () => { chatCalls += 1; } })));
    await flush();
  });
  const installedTab = Array.from(document.querySelectorAll<HTMLButtonElement>(".skill-library__actions button"))
    .find((button) => button.textContent?.includes("我安装的"));
  if (!installedTab) throw new Error("missing installed skills tab");
  await act(async () => {
    installedTab.click();
    await flush();
  });
  await waitFor("skill card", () => document.querySelector(".cap-skill-card__command")?.textContent === "审计问题修复（后端）");
  ok(installedTab?.textContent?.includes("2"), "installed skill count excludes built-in skills without requiring a catalog match");
  ok(document.body.textContent?.includes("explore") === false, "installed skill cards exclude built-in skills");
  ok(document.body.textContent?.includes("removed-from-catalog") === true, "installed skill cards retain local entries missing from the complete backend catalog");
  ok(document.querySelector(".cap-skill-card__command")?.textContent === "审计问题修复（后端）", "installed skill cards prefer the current backend Chinese name");
  ok(
    document.querySelector<HTMLImageElement>(".cap-skill-card__icon img")?.src === "https://cdn.example.com/skills/audit-fix.png",
    "installed skill cards render their persisted marketplace icon",
  );
  ok(!document.querySelector(".primary-workspace__header"), "workspace skill page does not repeat the visible page header");
  const toggle = document.querySelector<HTMLInputElement>(".cap-switch input[type='checkbox']");
  if (!toggle) throw new Error("missing skill toggle checkbox");
  ok(toggle.getAttribute("aria-label") === "停用技能 审计问题修复（后端）", "skill toggle checkbox uses the displayed backend name");

  const batchManage = findButton("批量管理");
  if (!batchManage) throw new Error("missing installed skill batch management button");
  await act(async () => {
    batchManage.click();
    await flush();
  });
  ok(document.querySelector('.cap-skill-card[role="checkbox"]')?.getAttribute("aria-checked") === "false", "batch management adds accessible skill selection controls");
  ok(!document.querySelector('button[aria-label="打开技能 审计问题修复（后端） 菜单"]'), "batch management temporarily hides per-skill actions");
  const finishBatch = findButton("完成");
  if (!finishBatch) throw new Error("missing batch management completion button");
  await act(async () => {
    finishBatch.click();
    await flush();
  });

  const skillMenuTrigger = document.querySelector<HTMLButtonElement>('button[aria-label="打开技能 审计问题修复（后端） 菜单"]');
  if (!skillMenuTrigger) throw new Error("missing installed skill menu trigger");
  await act(async () => {
    skillMenuTrigger.click();
    await flush();
  });
  ok(document.querySelectorAll('[role="menuitem"]').length === 2, "installed skill menu only shows chat and uninstall");
  ok(document.body.textContent?.includes("编辑") === false, "installed skill menu omits the edit action");
  const chatAction = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((button) => button.textContent?.includes("去对话"));
  if (!chatAction) throw new Error("missing installed skill chat action");
  await act(async () => {
    chatAction.click();
    await flush();
  });
  ok(chatCalls === 1, "installed skill chat action delegates to the composer");

  await act(async () => {
    skillMenuTrigger.click();
    await flush();
  });
  const uninstallAction = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((button) => button.textContent?.trim() === "卸载");
  if (!uninstallAction) throw new Error("missing installed skill uninstall action");
  await act(async () => {
    uninstallAction.click();
    await flush();
  });
  const confirmUninstall = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((button) => button.textContent?.trim() === "确认卸载");
  if (!confirmUninstall) throw new Error("missing installed skill uninstall confirmation");
  await act(async () => {
    confirmUninstall.click();
    await flush();
  });
  ok(uninstallCalls === 1, "installed skill uninstall action calls the backend after confirmation");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "mcp-bulk-failure-channel", cwd: "/tmp/suiyuan-mcp-bulk-failure", workspaceRoot: "/tmp/suiyuan-mcp-bulk-failure" };
  const tabs: TabMeta[] = [{
    id: "tab-mcp-bulk-failure",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-mcp-bulk-failure",
    workspaceName: "suiyuan-mcp-bulk-failure",
    topicId: "topic-mcp-bulk-failure",
    topicTitle: "MCP bulk failure",
    label: "MCP bulk failure",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-mcp-bulk-failure",
  }];
  let reconnectCalls = 0;
  const servers: ServerView[] = [
    { ...server("initializing"), name: "healthy" },
    { ...server("initializing"), name: "broken" },
  ];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        MCPServers: async () => servers,
        ConnectorCatalog: async () => ({ catalog_version: "test", items: [] }),
        ConnectorStates: async () => [],
        ReconnectMCPServer: async (name: string) => {
          reconnectCalls += 1;
          if (name === "broken") throw new Error("broken reconnect failure");
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(MCPServersSettingsPage)));
    await flush();
  });
  await waitFor("bulk retry button", () => Boolean(findButton("全部重试")));

  const retryAll = findButton("全部重试");
  if (!retryAll) throw new Error("missing Retry all button");
  await act(async () => {
    retryAll.click();
    await flush();
  });

  await waitFor("bulk retry error banner", () => document.querySelector(".banner--error")?.textContent?.includes("更新连接器失败，请稍后重试。") === true);
  ok(document.querySelector(".banner--error")?.textContent?.includes("broken reconnect failure") === false, "Retry all hides the raw connector failure");
  ok(reconnectCalls === 2, "Retry all attempts each retryable MCP server");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "mcp-failure-channel", cwd: "/tmp/suiyuan-mcp-failure", workspaceRoot: "/tmp/suiyuan-mcp-failure" };
  const tabs: TabMeta[] = [{
    id: "tab-mcp-failure",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-mcp-failure",
    workspaceName: "suiyuan-mcp-failure",
    topicId: "topic-mcp-failure",
    topicTitle: "MCP failure",
    label: "MCP failure",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-mcp-failure",
  }];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        MCPServers: async () => {
          throw new Error("forced MCP load failure");
        },
        ConnectorCatalog: async () => ({ catalog_version: "test", items: [] }),
        ConnectorStates: async () => [],
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(MCPServersSettingsPage)));
    await flush();
  });
  await waitFor("MCP load error banner", () => document.querySelector(".banner--error")?.textContent?.includes("加载连接器失败，请稍后重试。") === true);
  ok(document.querySelector(".banner--error")?.textContent?.includes("forced MCP load failure") === false, "MCP load hides the raw backend failure");

  ok(document.body.textContent?.includes("No MCP servers configured.") === false, "failed MCP load does not render the empty server state");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}

{
  const dom = installDom();
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const root = createRoot(rootEl);
  const meta: Meta = { label: "test", ready: true, eventChannel: "skills-failure-channel", cwd: "/tmp/suiyuan-skills-failure", workspaceRoot: "/tmp/suiyuan-skills-failure" };
  const tabs: TabMeta[] = [{
    id: "tab-skills-failure",
    scope: "project",
    workspaceRoot: "/tmp/suiyuan-skills-failure",
    workspaceName: "suiyuan-skills-failure",
    topicId: "topic-skills-failure",
    topicTitle: "Skills failure",
    label: "Skills failure",
    ready: true,
    running: false,
    mode: "normal",
    toolApprovalMode: "auto",
    active: true,
    cwd: "/tmp/suiyuan-skills-failure",
  }];
  window.go = {
    main: {
      App: {
        Meta: async () => meta,
        ListTabs: async () => tabs,
        SkillsSettings: async () => {
          throw new Error("forced skills load failure");
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };

  await act(async () => {
    root.render(React.createElement(LocaleProvider, null, React.createElement(SkillsSettingsPage)));
    await flush();
  });
  await waitFor("skills load error banner", () => document.querySelector(".banner--error")?.textContent?.includes("加载技能失败，请稍后重试。") === true);
  ok(document.querySelector(".banner--error")?.textContent?.includes("forced skills load failure") === false, "skills load hides the raw backend failure");

  ok(document.body.textContent?.includes("No skills found.") === false, "failed skills load does not render the empty skills state");

  await act(async () => {
    root.unmount();
  });
  dom.window.close();
}
