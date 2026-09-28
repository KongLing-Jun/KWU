// Run: tsx src/__tests__/composer-responsive-toolbar.test.tsx

import { readFileSync } from "node:fs";
import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { Composer } from "../components/Composer";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ToolApprovalMode } from "../lib/types";
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

function eq(actual: unknown, expected: unknown, label: string) {
  ok(actual === expected, `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
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
  globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
  globalThis.HTMLTextAreaElement = dom.window.HTMLTextAreaElement;
  globalThis.Event = dom.window.Event;
  globalThis.KeyboardEvent = dom.window.KeyboardEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
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

function mockApp(skills: Array<{
  name: string;
  displayName?: string;
  description: string;
  scope: string;
  runAs: string;
  enabled: boolean;
  iconUrl?: string;
}> = []) {
  window.go = {
    main: {
      App: {
        Commands: async () => skills
          .filter((skill) => skill.enabled)
          .map((skill) => ({
            name: skill.name,
            description: skill.description,
            kind: "skill" as const,
          })),
        Models: async () => [],
        ModelsForTab: async () => [],
        ListDir: async () => [],
        SearchFileRefs: async () => [],
        ListSessions: async () => [],
        MCPServers: async () => [],
        SkillsSettings: async () => ({ skills, skillRoots: [] }),
        RefreshSkills: async () => {},
        SetSkillEnabled: async () => {},
        ReconnectMCPServer: async () => {},
        SetMCPServerEnabled: async () => {},
      } as Partial<AppBindings> as AppBindings,
    },
  };
}

async function renderComposer(overrides: Partial<Parameters<typeof Composer>[0]> = {}) {
  const rootElement = document.getElementById("root");
  if (!rootElement) throw new Error("missing root element");
  const root = createRoot(rootElement);
  const props: Parameters<typeof Composer>[0] = {
    running: false,
    collaborationMode: "normal",
    toolApprovalMode: "ask",
    tokenMode: "full",
    modelLabel: "DeepSeek-R1",
    onSend: () => {},
    onCancel: () => undefined,
    onCycleMode: () => {},
    onSetMode: () => {},
    onSetCollaborationMode: () => {},
    onSetToolApprovalMode: () => {},
    onToggleYoloApprovalMode: () => {},
    onClearGoal: () => {},
    onSwitchModel: () => {},
    onSetEffort: () => {},
    onSetTokenMode: () => {},
    ...overrides,
    modelCatalog: overrides.modelCatalog ?? { state: "ready", routes: [] },
  };
  await act(async () => {
    root.render(
      <LocaleProvider>
        <ToastProvider>
          <Composer {...props} />
        </ToastProvider>
      </LocaleProvider>,
    );
    await flushTimers();
  });
  return root;
}

async function openHubSection(label: string) {
  await flushTimers();
  if (!document.querySelector('.composer-add-menu[data-state="open"]')) {
    document.querySelector<HTMLButtonElement>(".composer-action-trigger--add")?.click();
    await flushTimers();
  }
  const entry = Array.from(document.querySelectorAll<HTMLButtonElement>(".composer-add-menu__nav-item"))
    .find((button) => button.textContent?.trim() === label);
  entry?.click();
  await flushTimers();
  await flushTimers();
}

console.log("\ncomposer responsive toolbar");

{
  const dom = installDom();
  mockApp();
  let listDirCalls = 0;
  const fileReferenceApp = window.go?.main?.App;
  if (!fileReferenceApp) throw new Error("missing test app binding");
  fileReferenceApp.ListDir = async () => {
    listDirCalls += 1;
    return [];
  };
  const root = await renderComposer({
    insertRequest: { id: 1, text: "检查当前实现", mode: "replace" },
  });

  eq(
    document.querySelectorAll(".composer-modebar--approval .composer-modebar__item").length,
    0,
    "desktop composer uses the shared approval trigger instead of a separate three-option control",
  );
  ok(
    document.querySelector(".composer-approval-trigger")?.textContent?.includes("需要批准") === true,
    "desktop composer shows the label for the active approval mode",
  );
  ok(
    document.querySelector(".composer-action-trigger--add")?.getAttribute("title") === null,
    "unified add trigger relies on its custom tooltip without a duplicate native title",
  );

  await act(async () => {
    document.querySelector<HTMLButtonElement>(".composer-action-trigger--add")?.click();
    await flushTimers();
    Array.from(document.querySelectorAll<HTMLButtonElement>(".composer-add-menu__nav-item"))
      .find((button) => button.textContent?.trim() === "引用文件或目录")
      ?.click();
    await flushTimers();
    await flushTimers();
  });

  eq(document.getElementById("composer-input")?.textContent, "检查当前实现 @", "file reference entry reuses the existing trailing @ token");
  eq(listDirCalls, 1, "file reference entry reuses the existing workspace directory bridge");
  ok(Boolean(document.querySelector(".slashmenu")), "file reference entry opens the existing workspace picker");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  let listSessionCalls = 0;
  const historyReferenceApp = window.go?.main?.App;
  if (!historyReferenceApp) throw new Error("missing test app binding");
  historyReferenceApp.ListSessions = async () => {
    listSessionCalls += 1;
    return [];
  };
  const root = await renderComposer();

  await act(async () => {
    document.querySelector<HTMLButtonElement>(".composer-action-trigger--add")?.click();
    await flushTimers();
    Array.from(document.querySelectorAll<HTMLButtonElement>(".composer-add-menu__nav-item"))
      .find((button) => button.textContent?.trim() === "引用历史对话")
      ?.click();
    await flushTimers();
    await flushTimers();
  });

  eq(document.getElementById("composer-input")?.textContent, "@", "history reference entry reuses the existing @ reference token");
  eq(listSessionCalls, 1, "history reference entry reuses the existing session bridge");
  ok(document.querySelector(".slashmenu")?.textContent?.includes("暂无历史会话") === true, "history reference entry opens the existing history picker");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([{
    name: "ardot-design-assistant",
    displayName: "Ardot Design Assistant",
    description: "Design assistant",
    scope: "global",
    runAs: "inline",
    enabled: true,
    iconUrl: "https://cdn.example.com/skills/ardot-design-assistant.png",
  }, {
    name: "code-refactor",
    description: "Refactor code",
    scope: "global",
    runAs: "subagent",
    enabled: true,
  }]);
  let submitted: Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null = null;
  const root = await renderComposer({
    onOpenSkills: () => {},
    onSend: (...args) => {
      submitted = args;
    },
  });

  await act(async () => {
    await openHubSection("技能");
  });
  const skillRow = document.querySelector<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__main");
  ok(skillRow?.textContent?.includes("ardot-design-assistant") === true, "skill quick list uses the real skill catalog");
  const skillIcons = document.querySelectorAll<HTMLElement>(".composer-skill-menu__icon");
  eq(
    skillIcons[0]?.querySelector("img")?.getAttribute("src"),
    "https://cdn.example.com/skills/ardot-design-assistant.png",
    "skill quick list renders the skill catalog icon",
  );
  eq(skillIcons[0]?.textContent, "A", "skill icon keeps an initial fallback behind the image");
  eq(skillIcons[1]?.textContent, "C", "skills without an icon URL render a visible initial fallback");
  ok(
    document.querySelectorAll<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__main")[1]?.textContent?.includes("Refactor code") === true,
    "skill quick list displays the skill description",
  );
  ok(
    !document.querySelector(".composer-skill-menu")?.textContent?.includes("技能命令")
      && !document.querySelector(".composer-skill-menu")?.textContent?.includes("主流程"),
    "skill quick list omits internal slash and execution-mode wording",
  );
  await act(async () => {
    skillRow?.click();
    await flushTimers();
  });
  const editor = document.getElementById("composer-input");
  eq(editor?.querySelectorAll(".composer-skill-token").length, 1, "skill selection inserts an inline token at the saved caret");
  ok(Boolean(document.querySelector(".composer-add-menu__nav-item .lucide-hammer")), "unified add menu uses the shared skill icon");
  ok(Boolean(editor?.querySelector(".composer-skill-token .lucide-hammer")), "inline skill token uses the shared skill icon");
  eq(editor?.querySelector(".composer-skill-token")?.getAttribute("title"), "ardot-design-assistant", "inline skill token exposes its full name on hover");
  ok(Boolean(editor?.querySelector(".composer-skill-token__remove")), "inline skill token exposes a real remove control");
  ok(Boolean(document.querySelector(".composer-skill-menu")), "skill list stays open so several skills can be selected");

  await act(async () => {
    const token = editor?.querySelector(".composer-skill-token");
    if (editor && token) {
      const typed = document.createTextNode("你好");
      editor.replaceChildren(token, typed);
      editor.focus();
      const range = document.createRange();
      range.setStart(typed, typed.nodeValue?.length ?? 0);
      range.collapse(true);
      const selection = window.getSelection();
      selection?.removeAllRanges();
      selection?.addRange(range);
    }
    editor?.dispatchEvent(new Event("input", { bubbles: true }));
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 1, "typing directly after a token keeps the token rendered");

  await act(async () => {
    const rows = document.querySelectorAll<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__main");
    rows[1]?.click();
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 2, "a second skill is inserted without replacing the first");

  await act(async () => {
    editor?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flushTimers();
  });
  const submittedResult = submitted as Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null;
  const [displayText, submitText, capabilities] = submittedResult ?? [];
  eq(displayText, "/ardot-design-assistant 你好 /code-refactor", "sent user text preserves every visual slash skill token");
  eq(submitText, "你好", "model-visible task text excludes the visual skill tokens");
  eq(capabilities?.title_text, "你好", "automatic title text excludes every visual skill token");
  eq(capabilities?.skill_names?.join(","), "ardot-design-assistant,code-refactor", "all selected skills use the real structured skill execution path");
  eq(capabilities?.mcp_server_names?.length, 0, "skill-only submission does not claim any connector");
  eq(editor?.querySelectorAll(".composer-skill-token").length, 0, "successful send clears all one-shot skill tokens");
  await act(async () => {
    const paste = new Event("paste", { bubbles: true, cancelable: true });
    Object.defineProperty(paste, "clipboardData", {
      value: {
        files: [],
        items: [],
        types: ["text/plain"],
        getData: (type: string) => type === "text" || type === "text/plain"
          ? "/ardot-design-assistant /code-refactor 再检查一次"
          : "",
        setData: () => {},
      },
    });
    editor?.dispatchEvent(paste);
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 2, "pasting copied slash skill text restores every inline token");
  const pastedToken = editor?.querySelector(".composer-skill-token");
  if (editor && pastedToken) {
    editor.focus();
    const range = document.createRange();
    range.setStartAfter(pastedToken);
    range.collapse(true);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    document.dispatchEvent(new Event("selectionchange"));
  }
  await act(async () => {
    editor?.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Backspace",
      bubbles: true,
      cancelable: true,
    }));
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 1, "Backspace removes only the adjacent skill token atomically");
  await act(async () => {
    editor?.querySelector<SVGPathElement>(".composer-skill-token__remove path")?.dispatchEvent(new MouseEvent("mousedown", {
      bubbles: true,
      cancelable: true,
    }));
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 0, "mouse-down on the remove icon path removes the selected skill token without relying on WebView click delivery");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  let submitted: Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null = null;
  const root = await renderComposer({
    insertRequest: { id: 1, text: "现在呢", mode: "replace" },
    selectedConnectors: [],
    onSend: (...args) => {
      submitted = args;
    },
  });
  await act(async () => {
    document.querySelector<HTMLButtonElement>(".composer__btn--send")?.click();
    await flushTimers();
  });
  const submittedResult = submitted as Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null;
  const capabilities = submittedResult?.[2];
  eq(capabilities?.mcp_server_names?.length, 0, "ordinary turns submit an explicit empty connector scope");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([
    { name: "frontend-design", description: "Design polished interfaces", scope: "global", runAs: "inline", enabled: true },
    { name: "code-refactor", description: "Systematic code cleanup", scope: "global", runAs: "subagent", enabled: true },
    { name: "code-review", description: "Review changed code", scope: "global", runAs: "inline", enabled: true },
    { name: "financial-scan", description: "Search stocks and funds", scope: "global", runAs: "inline", enabled: true },
    { name: "commit", description: "Create a commit", scope: "global", runAs: "inline", enabled: true },
    { name: "explore", description: "Explore the codebase", scope: "global", runAs: "subagent", enabled: true },
    { name: "document", description: "Restructure documents", scope: "global", runAs: "inline", enabled: true },
  ]);
  const root = await renderComposer({ onOpenSkills: () => {} });

  await act(async () => {
    await openHubSection("技能");
  });
  const search = document.querySelector<HTMLInputElement>(".composer-skill-menu__search input");
  ok(Boolean(search), "skill search is shown for a non-trivial catalog");
  await act(async () => {
    if (search) {
      search.value = "stocks";
      search.dispatchEvent(new Event("input", { bubbles: true }));
    }
    await flushTimers();
  });
  const filteredRows = document.querySelectorAll(".composer-skill-menu .composer-connector-menu__row");
  eq(filteredRows.length, 1, "typing in skill search changes the visible result set");
  ok(filteredRows[0]?.textContent?.includes("financial-scan") === true, "skill search matches the real description");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  let enabled = false;
  let refreshCalls = 0;
  const enableCalls: Array<[string, boolean]> = [];
  window.go = {
    main: {
      App: {
        Commands: async () => [],
        Models: async () => [],
        ModelsForTab: async () => [],
        SkillsSettings: async () => ({
          skills: [{
            name: "disabled-skill",
            description: "Disabled fixture",
            scope: "global",
            runAs: "inline",
            enabled,
          }],
          skillRoots: [],
        }),
        RefreshSkills: async () => {
          refreshCalls += 1;
        },
        SetSkillEnabled: async (name: string, next: boolean) => {
          enableCalls.push([name, next]);
          enabled = next;
        },
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const root = await renderComposer({ onOpenSkills: () => {} });

  await act(async () => {
    await openHubSection("技能");
  });
  const enableButton = document.querySelector<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__action");
  ok(Boolean(enableButton), "disabled skill exposes a real enable action");
  await act(async () => {
    enableButton?.click();
    await flushTimers();
    await flushTimers();
  });
  eq(JSON.stringify(enableCalls), JSON.stringify([["disabled-skill", true]]), "enable action calls SetSkillEnabled with the selected skill");
  eq(refreshCalls, 1, "enable action refreshes the backend skill registry");
  eq(document.querySelectorAll(".composer-skill-token").length, 1, "enabled skill is inserted only after backend enable succeeds");

  const refreshButton = document.querySelector<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__refresh");
  await act(async () => {
    refreshButton?.click();
    await flushTimers();
    await flushTimers();
  });
  eq(refreshCalls, 2, "skill list refresh button calls the real RefreshSkills bridge");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([{
    name: "ardot-design-assistant",
    description: "Design assistant",
    scope: "global",
    runAs: "inline",
    enabled: true,
  }]);
  let submitted: Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null = null;
  const root = await renderComposer({
    insertRequest: { id: 2, text: "你好世界", mode: "replace" },
    onOpenSkills: () => {},
    onSend: (...args) => {
      submitted = args;
    },
  });
  await act(async () => {
    await flushTimers();
  });
  const editor = document.getElementById("composer-input");
  const textNode = editor?.firstChild;
  if (editor && textNode) {
    editor.focus();
    const range = document.createRange();
    range.setStart(textNode, 2);
    range.collapse(true);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    document.dispatchEvent(new Event("selectionchange"));
  }
  await act(async () => {
    await openHubSection("技能");
  });
  const caretSkillRow = document.querySelector<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__main");
  ok(Boolean(caretSkillRow), "skill list remains available after preserving the editor caret");
  await act(async () => {
    caretSkillRow?.click();
    await flushTimers();
  });
  await act(async () => {
    editor?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flushTimers();
  });
  const submittedResult = submitted as Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null;
  const [displayText, submitText, capabilities] = submittedResult ?? [];
  eq(displayText, "你好 /ardot-design-assistant 世界", "skill selection inserts the token at the saved caret");
  eq(submitText, "你好 世界", "caret-positioned visual token is removed from model-visible task text");
  eq(capabilities?.skill_names?.join(","), "ardot-design-assistant", "caret-positioned skill still uses structured execution");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  let openedSkills = 0;
  let openedConnectors = 0;
  const root = await renderComposer({
    effort: undefined,
    onOpenSkills: () => {
      openedSkills += 1;
    },
    onOpenConnectors: () => {
      openedConnectors += 1;
    },
  });

  await act(async () => {
    await openHubSection("技能");
  });
  ok(
    ["技能", "连接器"].every((label) =>
      Array.from(document.querySelectorAll(".composer-add-menu__nav-item"))
        .some((entry) => entry.textContent?.trim() === label)),
    "unified add menu exposes both skills and connectors",
  );
  ok(Boolean(document.querySelector(".composer-skill-menu")), "skills entry opens the quick list instead of navigating away");
  eq(openedSkills, 0, "opening the quick list does not open the full skill workspace");
  eq(openedConnectors, 0, "skills entry does not open connectors");
  await act(async () => {
    document.querySelector<HTMLButtonElement>(".composer-skill-menu .composer-connector-menu__more")?.click();
    await flushTimers();
  });
  eq(openedSkills, 1, "only the skill panel management action delegates to the full workspace");

  await act(async () => {
    await openHubSection("连接器");
  });
  ok(Boolean(document.querySelector(".composer-connector-menu")), "connector entry opens the quick list instead of navigating away");
  eq(openedConnectors, 0, "opening the quick list does not open the full connector workspace");
  await act(async () => {
    document.querySelector<HTMLButtonElement>(".composer-connector-menu__more")?.click();
    await flushTimers();
  });
  eq(openedConnectors, 1, "only the connector panel management action delegates to the full workspace");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  for (const [mode, label, iconClass] of [
    ["ask", "需要批准", "lucide-shield"],
    ["auto", "自动批准", "lucide-shield-check"],
    ["yolo", "完全访问", "lucide-shield-alert"],
  ] as const) {
    const root = await renderComposer({ toolApprovalMode: mode });
    const approvalTrigger = document.querySelector(".composer-approval-trigger");
    ok(
      approvalTrigger?.textContent?.includes(label) === true,
      `approval trigger shows ${label} for ${mode} mode`,
    );
    ok(
      approvalTrigger?.classList.contains(`composer-approval-trigger--${mode}`) === true
        && approvalTrigger?.querySelector(`.${iconClass}`) !== null,
      `approval trigger shows the matching icon and semantic class for ${mode} mode`,
    );
    await act(async () => root.unmount());
  }
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  const approvalChanges: ToolApprovalMode[] = [];
  const root = await renderComposer({
    toolApprovalMode: "ask",
    onSetToolApprovalMode: (mode) => { approvalChanges.push(mode); },
  });

  const approvalTrigger = document.querySelector<HTMLButtonElement>(".composer-approval-trigger");
  ok(approvalTrigger?.getAttribute("aria-haspopup") === "menu", "shared approval trigger declares its menu");
  ok(approvalTrigger?.textContent?.includes("需要批准") === true, "approval trigger renders the current ask-mode label");
  ok(approvalTrigger?.getAttribute("aria-label")?.includes("需要批准") === true, "approval trigger names the current state");
  ok(approvalTrigger?.getAttribute("title") === null, "approval trigger relies on its custom tooltip without a duplicate native title");
  await act(async () => {
    approvalTrigger?.click();
    await new Promise((resolve) => setTimeout(resolve, 20));
  });
  const approvalItems = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]'));
  eq(approvalItems.length, 3, "approval menu exposes all three permission choices");
  ok(
    approvalItems.every((item) => item.querySelectorAll("svg").length === 1),
    "approval menu choices render their permission icon without an active checkmark",
  );
  ok(
    ["lucide-shield", "lucide-shield-check", "lucide-shield-alert"].every(
      (iconClass, index) => approvalItems[index]?.querySelector(`.${iconClass}`) !== null,
    ),
    "approval menu choices keep their labels and permission icons in the same mode order",
  );
  ok(
    document.querySelector(".composer-approval-menu")?.classList.contains("composer-approval-menu--compact") === true,
    "approval menu follows the composer container into its compact presentation",
  );
  const approvalDescription = approvalItems[0]?.querySelector(".composer-access-menu__desc")?.textContent ?? "";
  ok(
    approvalDescription.length > 0 && (approvalItems[0]?.getAttribute("aria-label") ?? "").includes(approvalDescription),
    "compact approval choices retain their full accessible explanation",
  );
  eq(approvalItems[0]?.getAttribute("aria-checked"), "true", "current approval choice is announced");
  await act(async () => {
    approvalItems[0]?.focus();
    await flushTimers();
  });
  ok(document.activeElement === approvalItems[0], "current approval choice accepts focus");
  await act(async () => {
    approvalItems[0]?.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
    await flushTimers();
  });
  ok(document.activeElement === approvalItems[1], "arrow keys move focus between approval choices");
  await act(async () => {
    approvalItems[1]?.click();
    await flushTimers();
  });
  eq(approvalChanges.join(","), "auto", "shared approval selection reuses the approval callback");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([{
    name: "review",
    description: "Review changes",
    scope: "global",
    runAs: "inline",
    enabled: true,
  }]);
  let submitted: Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null = null;
  const root = await renderComposer({
    insertRequest: { id: 1, text: "/review ", kind: "skill" },
    modelLabel: "gpt/gpt-5.6-luna",
    modelCatalog: {
      state: "ready",
      routes: [{
        ref: "gpt/gpt-5.6-luna",
        provider: "gpt",
        model: "gpt-5.6-luna",
        billingSource: "newapi_byok",
        current: true,
      }],
    },
    effort: { supported: true, current: "auto", default: "auto", levels: ["auto", "low"] },
    selectedConnectors: ["GitHub", "Notion"],
    onClearConnector: () => {},
    onOpenSkills: () => {},
    onOpenConnectors: () => {},
    activeExpert: {
      id: "review-team",
      name: "Review Team",
      role: "Code review",
      mode: "team",
      prompt: "Review the change.",
    },
    onClearExpert: () => {},
    onSend: (...args) => {
      submitted = args;
    },
  });

  const capabilityChips = document.querySelector<HTMLElement>(".composer-capability-chips");
  const accessibleName = capabilityChips?.getAttribute("aria-label") ?? "";
  ok(accessibleName.includes("当前消息/任务已选择 3 项"), "persistent capability row announces every selected capability");
  ok(
    ["review", "GitHub", "Notion"].every((name) => accessibleName.includes(name)),
    "persistent capability row names every selected item",
  );
  ok(document.querySelector(".composer-capabilities-trigger") === null, "toolbar does not duplicate the selected capability state");
  eq(document.querySelectorAll(".composer-skill-token").length, 1, "the selected slash skill renders inside the editable message");
  eq(document.querySelectorAll(".composer-capability-chip").length, 2, "only persistent connectors render above the input");
  const connectorIcons = Array.from(document.querySelectorAll<HTMLElement>("button.composer-capability-chip .connector-brand-icon"));
  eq(connectorIcons.length, 2, "each enabled connector renders as a compact icon button");
  ok(
    connectorIcons.every((icon) => icon.style.getPropertyValue("--connector-icon-size") === "24px"),
    "enabled connectors use bare 24px brand icons inside the input",
  );
  const modelTrigger = document.querySelector<HTMLButtonElement>(".composer-meta__control--model .modelsw__trigger");
  ok(Boolean(modelTrigger?.getAttribute("aria-label")), "icon-only model trigger retains an accessible name");
  ok(
    modelTrigger?.getAttribute("aria-label") === "gpt/gpt-5.6-luna · 推理强度：自动",
    "model trigger does not repeat the provider already present before the slash",
  );
  ok(modelTrigger?.getAttribute("title") === null, "model trigger relies on its custom tooltip without a duplicate native title");
  ok(Boolean(document.querySelector(".composer-action-trigger__clear")), "extreme expert state retains its clear action");
  const editor = document.getElementById("composer-input");
  await act(async () => {
    editor?.append(document.createTextNode("检查这次修改"));
    editor?.dispatchEvent(new Event("input", { bubbles: true }));
    await flushTimers();
  });
  await act(async () => {
    editor?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flushTimers();
  });
  const submittedResult = submitted as Parameters<NonNullable<Parameters<typeof Composer>[0]["onSend"]>> | null;
  const [displayText, submitText, capabilities] = submittedResult ?? [];
  eq(displayText, "/review 检查这次修改", "skill and connector turn keeps the visible slash token");
  ok(submitText?.includes("检查这次修改") === true && !submitText.includes("/review"), "model-visible text keeps the task and removes the visual token");
  eq(capabilities?.skill_names?.join(","), "review", "selected skill uses the host-validated structured trigger");
  eq(capabilities?.title_text, "检查这次修改", "automatic title text keeps only the visible conversation text");
  eq(capabilities?.mcp_server_names?.join(","), "GitHub,Notion", "selected connectors still use the structured connector scope");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([{
    name: "review",
    description: "Review changes",
    scope: "global",
    runAs: "inline",
    enabled: true,
  }]);
  const root = await renderComposer({
    insertRequest: { id: 1, text: "/review ", kind: "skill" },
    selectedConnectors: ["GitHub", "Notion"],
    onClearConnector: () => {},
    onOpenSkills: () => {},
    onOpenConnectors: () => {},
    onSend: async () => {
      throw new Error("connection refused");
    },
  });
  const editor = document.getElementById("composer-input");
  await act(async () => {
    editor?.append(document.createTextNode("inspect @src/app.ts and continue"));
    editor?.dispatchEvent(new Event("input", { bubbles: true }));
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 1, "structured failure case starts with one selected skill");
  ok(editor?.textContent?.includes("inspect @src/app.ts") === true, "structured failure case starts with text and @ file reference");
  await act(async () => {
    document.querySelector<HTMLButtonElement>(".composer__btn--send")?.click();
    await flushTimers();
  });
  eq(editor?.querySelectorAll(".composer-skill-token").length, 1, "failed structured send preserves one-shot skill tokens");
  ok(editor?.textContent?.includes("inspect @src/app.ts") === true, "failed structured send preserves text and @ file reference");
  eq(document.querySelectorAll(".composer-capability-chip").length, 2, "failed structured send preserves connector selections");
  ok(document.body.textContent?.includes("发送消息失败，请稍后重试。") === true, "failed structured send exposes the safe Chinese error");
  ok(document.body.textContent?.includes("connection refused") === false, "failed structured send hides the backend error");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([{
    name: "grilling",
    description: "Ask hard questions",
    scope: "global",
    runAs: "inline",
    enabled: true,
  }]);
  const root = await renderComposer({
    insertRequest: { id: 1, text: "/grilling ", kind: "skill" },
    onOpenSkills: () => {},
    onOpenConnectors: () => {},
  });
  await act(async () => {
    await flushTimers();
  });

  eq(document.querySelectorAll(".composer-skill-token").length, 1, "external skill insertion renders one inline token");
  const editor = document.getElementById("composer-input");
  const selection = window.getSelection();
  eq(selection?.isCollapsed, true, "external skill insertion leaves a collapsed caret");
  eq(selection?.anchorNode, editor?.lastChild, "external skill insertion places the caret in the trailing text node");
  eq(selection?.anchorOffset, editor?.lastChild?.textContent?.length ?? -1, "external skill insertion places the caret after the token boundary");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp([{
    name: "review",
    description: "Review changes",
    scope: "global",
    runAs: "inline",
    enabled: true,
  }]);
  const root = await renderComposer({
    onOpenSkills: () => {},
  });
  await act(async () => {
    await flushTimers();
    await flushTimers();
  });
  const editor = document.getElementById("composer-input");
  await act(async () => {
    if (editor) editor.textContent = "/rev";
    editor?.dispatchEvent(new Event("input", { bubbles: true }));
    await flushTimers();
  });

  ok(Boolean(document.querySelector(".slashmenu")), "incomplete slash text still opens the real skill autocomplete");
  await act(async () => {
    editor?.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true }));
    await flushTimers();
  });
  eq(document.querySelectorAll(".composer-skill-token").length, 1, "choosing a slash suggestion replaces the incomplete query with one skill token");
  ok(!document.querySelector(".slashmenu"), "skill autocomplete hides after a skill token has been selected");

  await act(async () => root.unmount());
  dom.window.close();
}

{
  const styles = loadStylesSource();
  const projectStyles = readFileSync(new URL("../features/projects/projects.css", import.meta.url), "utf8");
  ok(
    /\.composer-capability-chip\s*\{[^}]*width:\s*32px;[^}]*height:\s*32px;[^}]*padding:\s*4px;[^}]*border:\s*0;[^}]*background:\s*transparent/.test(styles)
      && /\.composer-capability-chip--connector \.connector-brand-icon:has\(> img\)\s*\{[^}]*background:\s*transparent/.test(styles)
      && /\.composer-capability-chip--connector \.connector-brand-icon img\s*\{[^}]*width:\s*100%;[^}]*height:\s*100%/.test(styles)
      && /\.composer-capability-chip:hover:not\(:disabled\),[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 8%, transparent\)/.test(styles),
    "enabled connector controls keep a stable hit area while showing only the full-size brand icon",
  );
  ok(/@container\s*\(max-width:\s*760px\)/.test(styles), "compact layout uses the confirmed 760px container breakpoint");
  ok(/@container\s*\(max-width:\s*520px\)/.test(styles), "extreme layout uses the confirmed 520px container breakpoint");
  ok(
    /\.composer-meta__control--model\s*\{[\s\S]*?flex:\s*0 0 240px;[\s\S]*?width:\s*240px;[\s\S]*?max-width:\s*240px/.test(styles),
    "desktop model control preserves its established fixed width",
  );
  ok(
    /@container\s*\(max-width:\s*760px\)[\s\S]*?\.composer-meta__control--model\s*\{[\s\S]*?flex:\s*0 0 168px;[\s\S]*?width:\s*168px;[\s\S]*?max-width:\s*168px/.test(styles),
    "compact model control has a fixed 168px width",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-meta__control--model\s*\{[\s\S]*?flex:\s*0 0 32px;[\s\S]*?width:\s*32px;[\s\S]*?max-width:\s*32px/.test(styles),
    "extreme model control has a fixed 32px width",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.app--creation \.composer-meta__control--model,[\s\S]*?flex:\s*0 0 32px;[\s\S]*?width:\s*32px;[\s\S]*?min-width:\s*32px;[\s\S]*?max-width:\s*32px/.test(styles),
    "creation theme clears the compact model minimum at the extreme breakpoint",
  );
  ok(
    /\.modelsw__menu--model\s*\{[\s\S]*?width:\s*240px;[\s\S]*?min-width:\s*240px;[\s\S]*?@media\s*\(max-width:\s*760px\)[\s\S]*?\.modelsw__menu--model\.modelsw__menu--portal\s*\{[\s\S]*?width:\s*168px;/.test(styles),
    "popover width is stable across desktop and compact layouts",
  );
  ok(
    !/(?:^|\n)\.modelsw\s*\{[^}]*width:\s*240px/.test(styles)
      && !/(?:^|\n)\.modelsw__trigger\s*\{[^}]*width:\s*240px/.test(styles)
      && !/(?:^|\n)\.modelsw__menu\s*\{[^}]*width:\s*240px/.test(styles),
    "shared model and effort switcher classes do not impose the model selector width",
  );
  ok(
    /\.modelsw--model\s*\{[^}]*width:\s*240px/.test(styles)
      && /\.modelsw__trigger--model\s*\{[^}]*width:\s*240px/.test(styles),
    "model-only modifier classes preserve the fixed width outside the composer",
  );
  ok(
    /\.composer-add-menu\s*\{[\s\S]*?display:\s*flex;[\s\S]*?width:\s*min\(608px,[\s\S]*?height:\s*min\(540px,\s*calc\(100vh - 24px\)\);[\s\S]*?overflow:\s*hidden/.test(styles),
    "unified add menu keeps one stable two-panel height while sections switch",
  );
  ok(
    /\.composer-add-menu__nav\s*\{[\s\S]*?flex:\s*0 0 206px;[\s\S]*?flex-direction:\s*column/.test(styles)
      && /\.composer-add-menu__panel\s*\{[\s\S]*?border-left:\s*1px solid var\(--border-soft\)/.test(styles),
    "unified add menu separates navigation from factual feature panels",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-meta__control--model \.modelsw__label[\s\S]*?display:\s*none/.test(styles),
    "extreme layout reduces the model switcher to its icon",
  );
  ok(
    /@container\s*\(max-width:\s*760px\)[\s\S]*?\.composer-meta__control--intent\s*\{[\s\S]*?flex:\s*0 0 auto;[\s\S]*?max-width:\s*none/.test(styles),
    "compact intent group keeps enough real width for simultaneous intent controls",
  );
  ok(
    /@container\s*\(max-width:\s*760px\)[\s\S]*?\.composer-meta__control--approval\s*\{\s*flex:\s*0 0 112px;\s*width:\s*112px;\s*min-width:\s*112px;\s*max-width:\s*112px;/.test(styles)
      && /@container\s*\(max-width:\s*760px\)[\s\S]*?\.app--creation \.composer-meta__control--approval,[\s\S]*?:root\[data-theme-style\] \.app--creation \.composer-meta__control--approval\s*\{\s*flex:\s*0 0 112px;\s*width:\s*112px;\s*min-width:\s*112px;\s*max-width:\s*112px;/.test(styles),
    "compact approval control reserves enough width for four Chinese label characters",
  );
  ok(
    /@container\s*\(max-width:\s*760px\)[\s\S]*?\n\s*\.composer-action-trigger-shell--expert\s*\{\s*flex:\s*0 1 216px;\s*width:\s*auto;\s*max-width:\s*216px;/.test(styles),
    "compact expert shell uses available width before truncating expert and expert-team names",
  );
  ok(
    /@container\s*\(max-width:\s*760px\)[\s\S]*?\.app--creation \.composer-action-trigger-shell--expert,[\s\S]*?:root\[data-theme-style\] \.app--creation \.composer-action-trigger-shell--expert\s*\{\s*flex:\s*0 1 216px;\s*width:\s*auto;\s*max-width:\s*216px;/.test(styles),
    "creation-theme compact expert shell keeps the full expert-name allowance",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-action-trigger-shell--expert\s*\{[\s\S]*?width:\s*60px/.test(styles),
    "extreme expert shell reserves space for the avatar and clear action",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-action-trigger__label,[\s\S]*?\.composer-mode-chip__label\s*\{[\s\S]*?display:\s*none/.test(styles)
      && !styles.includes(".composer-action-trigger__clear,\n  .composer-action-trigger__label"),
    "extreme layout hides labels without hiding the expert clear action",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.app--creation \.composer-action-trigger--expert,[\s\S]*?width:\s*28px[\s\S]*?\.app--creation \.composer-mode-chip,[\s\S]*?width:\s*28px/.test(styles),
    "creation theme explicitly compresses expert and mode controls at the extreme breakpoint",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-mode-chip\s*\{[\s\S]*?width:\s*32px;[\s\S]*?min-width:\s*32px;[\s\S]*?max-width:\s*32px/.test(styles),
    "extreme mode chips override fixed plan and goal minimum widths",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.app--creation \.composer-mode-chip,[\s\S]*?width:\s*28px;[\s\S]*?min-width:\s*28px;[\s\S]*?max-width:\s*28px/.test(styles),
    "creation theme preserves the extreme mode-chip width overrides",
  );
  ok(
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-meta__control--approval\s*\{[^}]*flex:\s*0 0 32px;[^}]*width:\s*32px/.test(styles)
      && /@container\s*\(max-width:\s*520px\)[\s\S]*?\.composer-meta__control--approval \.composer-approval-trigger > span,[\s\S]*?display:\s*none/.test(styles)
      && /@container\s*\(max-width:\s*520px\)[\s\S]*?\.app--creation \.composer-meta__control--approval,[\s\S]*?width:\s*28px/.test(styles),
    "narrow composers collapse approval to an accessible icon control",
  );
  ok(
    /\.composer-approval-trigger\s*\{[^}]*border:\s*1px solid var\(--border\);[^}]*border-radius:\s*7px;[^}]*background:\s*var\(--surface-2\)/.test(styles)
      && styles.includes(".app--creation .composer-approval-trigger,")
      && styles.includes(":root[data-theme-style] .app--creation .composer-approval-trigger,"),
    "approval trigger uses the same neutral light surface language as the model trigger",
  );
  ok(
    /\.composer-approval-menu\s*\{[^}]*padding:\s*5px;[^}]*border-radius:\s*9px;[^}]*box-shadow:\s*0 12px 32px rgba\(0, 0, 0, 0\.45\)/.test(styles)
      && styles.includes(":root[data-theme-style] .composer-approval-menu,"),
    "approval menu uses the model menu surface shape and themed floating treatment",
  );
  ok(
    /\.composer-approval-trigger--yolo \.composer-approval-trigger__status-icon,[\s\S]*?\.composer-approval-trigger--yolo \.composer-approval-trigger__label\s*\{[^}]*color:\s*var\(--danger\)/.test(styles)
      && /\.composer-approval-menu__item--yolo\.composer-access-menu__item--active,[\s\S]*?background:\s*color-mix\(in srgb, var\(--danger\) 12%, transparent\)/.test(styles),
    "full access keeps a neutral control surface while its icon, label, and active row use danger semantics",
  );
  ok(
    /\.modelsw__selection-effort\s*\{[\s\S]*?flex:\s*0 0 auto/.test(styles)
      && /\.modelsw__selection-effort\s*\{[^}]*color:\s*inherit/.test(styles)
      && /\.modelsw__selection-effort--explicit\s*\{[^}]*color:\s*inherit/.test(styles)
      && styles.includes(".modelsw__nav-item,")
      && styles.includes(".modelsw__panel-header {")
      && /\.modelsw__effort-list\s*\{[\s\S]*?flex-direction:\s*column/.test(styles),
    "compact model selector keeps model and effort available with one text color through the two-level panel",
  );
  ok(
    /\.composer-approval-menu\s*\{[\s\S]*?width:\s*min\(176px,[\s\S]*?\.composer-approval-menu \.composer-access-menu__item\s*\{[\s\S]*?height:\s*40px;[\s\S]*?\.composer-approval-menu \.composer-access-menu__desc\s*\{[\s\S]*?display:\s*none/.test(styles)
      && /\.composer-approval-trigger\s*\{[^}]*font-weight:\s*400/.test(styles)
      && /\.composer-approval-menu \.composer-access-menu__title\s*\{[^}]*font-weight:\s*400/.test(styles),
    "approval menu stays compact and uses the trigger's regular text weight at every composer width",
  );
  ok(
    /\.project-composer-approval-menu \.project-select-menu__list > button:hover,[\s\S]*?\.project-composer-approval-menu \.project-select-menu__list > button:focus-visible\s*\{[^}]*color:\s*var\(--fg\);[^}]*background:\s*var\(--bg-elev-2\)/.test(projectStyles),
    "project permission menu keeps the regular hover and keyboard-focus feedback",
  );
  ok(
    /\.composer-card--resized:has\(\.composer-capability-chips\)\s*\{[\s\S]*?height:\s*var\(--composer-height\);[\s\S]*?grid-template-rows:\s*auto minmax\(0, 1fr\) auto/.test(styles),
    "resized composer keeps the requested card height when capability chips are present",
  );
  ok(
    /\.composer-card--resized \.composer\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*hidden/.test(styles)
      && /\.composer-card--resized \.composer__input\s*\{[\s\S]*?min-height:\s*calc\(var\(--font-content\) \* 1\.55\);[\s\S]*?align-self:\s*stretch;[\s\S]*?overflow-y:\s*auto/.test(styles),
    "resized input keeps one visible line while yielding remaining space to the toolbar",
  );
  ok(
    /\.chat-pane--creation-empty \.composer-card--resized \.composer,[\s\S]*?\.chat-pane--creation-empty \.composer-card--resized \.composer__input\s*\{[\s\S]*?min-height:\s*0/.test(styles),
    "creation theme resets its tall empty-composer minimums after the user resizes the card",
  );
  ok(
    /\.composer-skill-token:hover \.composer-skill-token__remove,[\s\S]*?pointer-events:\s*auto/.test(styles),
    "skill token reveals its remove control on hover",
  );
  ok(
    /\.composer-skill-token\s*\{[^}]*padding:\s*2px 8px 2px 6px;[^}]*border-color:\s*var\(--border\);[^}]*border-radius:\s*999px;[^}]*background:\s*var\(--bg-soft\)/.test(styles)
      && /\.composer-skill-token:hover,[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 6%, var\(--bg-soft\)\)/.test(styles),
    "composer skill tokens use a neutral theme-aware pill treatment",
  );
}

{
  const dom = installDom();
  mockApp();
  const boundApp = window.go?.main?.App;
  if (!boundApp) throw new Error("missing test app binding");
  boundApp.ListProjectTree = async () => [{ key: "alpha", kind: "project", label: "Alpha", root: "D:\\alpha" }];
  let recognition: {
    start: () => void;
    stop: () => void;
    abort: () => void;
    onresult: ((event: unknown) => void) | null;
    onerror: ((event: unknown) => void) | null;
    onend: (() => void) | null;
    lang: string;
    interimResults: boolean;
    continuous: boolean;
  } | null = null;
  let abortCalls = 0;
  class TestSpeechRecognition {
    lang = "";
    interimResults = true;
    continuous = true;
    onresult: ((event: unknown) => void) | null = null;
    onerror: ((event: unknown) => void) | null = null;
    onend: (() => void) | null = null;
    constructor() { recognition = this; }
    start() {}
    stop() { this.onend?.(); }
    abort() { abortCalls += 1; this.onend?.(); }
  }
  Object.defineProperty(dom.window, "SpeechRecognition", { configurable: true, value: TestSpeechRecognition });
  const approvalModes: ToolApprovalMode[] = [];
  const root = await renderComposer({
    modelLabel: "Auto",
    submitDisabled: true,
    submitDisabledReason: "请选择工作空间后再发送",
    projectPresentation: {
      scopeKey: "account-a/project-a",
      workspacePath: "",
      cloudProjectId: "project-a",
      cloudProjectName: "项目 A",
      workspaceLocked: false,
      executionTarget: "local",
      onExecutionTargetChange: () => {},
      onWorkspacePathChange: () => {},
      onCloudProjectIdChange: () => {},
    },
    onSetToolApprovalMode: (mode) => { approvalModes.push(mode); },
  });
  await act(async () => { await flushTimers(); await flushTimers(); await flushTimers(); await flushTimers(); });
  eq(document.querySelector(".composer-wrap")?.classList.contains("composer-wrap--project"), true, "project composer reuses the shared surface with a project variant");
  eq(document.querySelector(".modelsw__selection-model")?.textContent, "Auto", "project composer keeps Auto visible until the user chooses an explicit model");
  eq(document.querySelector("#composer-input")?.getAttribute("data-placeholder"), "今天帮你做些什么？@ 引用资产文件、项目待办或调用技能", "project composer exposes the approved placeholder");
  const projectEditor = document.getElementById("composer-input");
  await act(async () => {
    projectEditor?.replaceChildren(document.createTextNode("保留这份草稿"));
    projectEditor?.dispatchEvent(new Event("input", { bubbles: true }));
    await flushTimers();
  });
  await act(async () => {
    projectEditor?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
    await flushTimers();
    await flushTimers();
  });
  ok(projectEditor?.textContent?.includes("保留这份草稿") === true, "missing local workspace preserves the project draft");
  const projectContextbar = document.querySelector<HTMLElement>(".project-composer-contextbar");
  const taskTarget = document.querySelector<HTMLElement>("[aria-label='任务类型：本地任务']");
  const workspace = document.querySelector<HTMLElement>("[aria-label='工作空间：选择工作空间']");
  eq(taskTarget?.tagName, "SPAN", "project task type is display-only text");
  eq(workspace?.tagName, "SPAN", "project workspace is display-only text");
  eq(projectContextbar?.querySelectorAll("button").length, 1, "only the project permission mode is interactive in the context row");
  eq(projectContextbar?.querySelectorAll(".lucide-chevron-down").length, 1, "project permission mode keeps its dropdown affordance");
  ok(!document.querySelector(".composer-meta__control--approval"), "project toolbar does not duplicate the permission selector");
  const approval = projectContextbar?.querySelector<HTMLButtonElement>(".project-composer-contextbar__approval");
  ok(approval?.textContent?.includes("需要批准") === true, "project permission selector shows the current mode in the context row");
  await act(async () => {
    approval?.click();
    await flushTimers();
  });
  const approvalMenu = document.querySelector<HTMLElement>(".composer-approval-menu");
  eq(approvalMenu?.querySelectorAll("[role='menuitemradio']").length, 3, "project permission selector opens the shared three-option menu");
  ok(Boolean(approvalMenu?.querySelector(".composer-approval-menu__item--yolo")), "project permission menu keeps the full-access warning option");
  await act(async () => {
    Array.from(approvalMenu?.querySelectorAll<HTMLButtonElement>("[role='menuitemradio']") ?? []).find((option) => option.textContent?.includes("自动批准"))?.click();
    await flushTimers();
  });
  eq(approvalModes.at(-1), "auto", "project permission selection uses the authoritative Composer callback");
  await act(async () => {
    document.querySelector<HTMLButtonElement>("[aria-label='开始语音输入']")?.click();
    await flushTimers();
  });
  act(() => {
    recognition?.onresult?.({ results: [{ 0: { transcript: "整理本月计划" }, isFinal: true }] });
  });
  ok(document.querySelector("#composer-input")?.textContent?.includes("整理本月计划") === true, "final speech recognition text is appended to the current draft");

  await act(async () => {
    document.querySelector<HTMLButtonElement>("[aria-label='停止语音输入']")?.click();
    await flushTimers();
  });
  ok(Boolean(document.querySelector("[aria-label='开始语音输入']")), "speech input can be stopped explicitly");
  await act(async () => {
    document.querySelector<HTMLButtonElement>("[aria-label='开始语音输入']")?.click();
    await flushTimers();
    recognition?.onerror?.({ error: "not-allowed" });
    await flushTimers();
  });
  ok(document.body.textContent?.includes("请在系统设置中允许语音权限") === true, "speech permission failure is visible to the user");
  delete (dom.window as typeof dom.window & { SpeechRecognition?: unknown }).SpeechRecognition;
  await act(async () => {
    document.querySelector<HTMLButtonElement>("[aria-label='开始语音输入']")?.click();
    await flushTimers();
  });
  ok(document.body.textContent?.includes("当前 WebView 不支持语音输入") === true, "unsupported speech recognition is not silent");
  Object.defineProperty(dom.window, "SpeechRecognition", { configurable: true, value: TestSpeechRecognition });
  await act(async () => {
    document.querySelector<HTMLButtonElement>("[aria-label='开始语音输入']")?.click();
    await flushTimers();
  });

  await act(async () => root.unmount());
  eq(abortCalls, 1, "unmount aborts an active speech recognition session");
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  const boundApp = window.go?.main?.App;
  if (!boundApp) throw new Error("missing test app binding");
  let projectTreeRequests = 0;
  boundApp.ListProjectTree = async () => { projectTreeRequests += 1; return []; };
  const root = await renderComposer({
    modelLabel: "Auto",
    projectPresentation: {
      scopeKey: "account-a/project-a",
      workspacePath: "D:\\alpha",
      cloudProjectId: "project-a",
      cloudProjectName: "产品设计",
      workspaceLocked: false,
      executionTarget: "cloud",
      onExecutionTargetChange: () => {},
      onWorkspacePathChange: () => {},
      onCloudProjectIdChange: () => {},
    },
  });
  const cloudProjectDisplay = document.querySelector<HTMLElement>(".project-composer-contextbar__workspace");
  eq(cloudProjectDisplay?.tagName, "SPAN", "cloud project context is display-only");
  eq(cloudProjectDisplay?.getAttribute("aria-label"), "云项目：产品设计", "cloud task names the current project");
  ok(!document.body.textContent?.includes("project-a"), "cloud task never exposes the internal project id");
  eq(projectTreeRequests, 0, "display-only project context does not load selectable project options");
  await act(async () => root.unmount());
  dom.window.close();
}

{
  const dom = installDom();
  mockApp();
  const boundApp = window.go?.main?.App;
  if (!boundApp) throw new Error("missing test app binding");
  let projectTreeRequests = 0;
  boundApp.ListProjectTree = async () => { projectTreeRequests += 1; throw new Error("project tree unavailable"); };
  const root = await renderComposer({
    projectPresentation: {
      scopeKey: "account-a/project-unavailable",
      workspacePath: "",
      cloudProjectId: "prj_internal_only",
      cloudProjectName: "",
      workspaceLocked: true,
      executionTarget: "cloud",
      onExecutionTargetChange: () => {},
      onWorkspacePathChange: () => {},
      onCloudProjectIdChange: () => {},
    },
  });
  await act(async () => { await flushTimers(); await flushTimers(); });
  const cloudProjectDisplay = document.querySelector<HTMLElement>(".project-composer-contextbar__workspace");
  eq(cloudProjectDisplay?.getAttribute("aria-label"), "云项目：当前云项目", "missing project names use the safe current-project fallback");
  eq(cloudProjectDisplay?.tagName, "SPAN", "the safe fallback remains display-only");
  ok(!document.body.textContent?.includes("prj_internal_only"), "the fallback never exposes the internal project id");
  eq(projectTreeRequests, 0, "the fallback does not request project selection data");
  await act(async () => root.unmount());
  dom.window.close();
}

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
