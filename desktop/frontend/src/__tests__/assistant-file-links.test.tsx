// Run: tsx src/__tests__/assistant-file-links.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { waitFor } from "@testing-library/react";
import { Markdown } from "../components/Markdown";
import { AssistantMessage } from "../components/Message";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { AppBindings } from "../lib/bridge";
import { resolveWorkspaceFileOwner, visibleWorkspaceFilePathKeys } from "../lib/workspaceFileScope";

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
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
}

function assistantView(text: string, tabId?: string, artifactPaths?: string[]) {
  return (
    <LocaleProvider>
      <ToastProvider>
        <AssistantMessage
          item={{
            kind: "assistant",
            id: "assistant-file-links",
            text,
            reasoning: "",
            streaming: false,
            finalAnswer: true,
            ...(artifactPaths ? { artifactPaths } : {}),
          }}
          {...(tabId ? { tabId } : {})}
        />
      </ToastProvider>
    </LocaleProvider>
  );
}

async function renderAssistant(
  text: string,
  openWorkspacePath: (path: string) => Promise<void> | void,
  readFile: AppBindings["ReadFile"] = async (path) => ({
    path,
    body: "",
    size: 0,
    truncated: false,
    binary: false,
  }),
  options: {
    tabId?: string;
    openWorkspacePathForTab?: (tabID: string, path: string) => Promise<void> | void;
    readFileForTab?: (tabID: string, path: string) => Promise<Awaited<ReturnType<AppBindings["ReadFile"]>>>;
    artifactPaths?: string[];
  } = {},
) {
  const rootEl = document.getElementById("root");
  if (!rootEl) throw new Error("missing root");
  const resolvedTabId = options.tabId === undefined ? "assistant-test-owner" : options.tabId;
  window.go = {
    main: {
      App: {
        OpenWorkspacePath: openWorkspacePath,
        OpenWorkspacePathForTab: options.openWorkspacePathForTab ?? (async (_tabID, path) => openWorkspacePath(path)),
        ReadFile: readFile,
        ReadFileForTab: options.readFileForTab ?? (async (_tabID, path) => readFile(path)),
        AttachmentDataURL: async () => "",
      } as Partial<AppBindings> as AppBindings,
    },
  };
  const root = createRoot(rootEl);
  await act(async () => {
    root.render(assistantView(text, resolvedTabId, options.artifactPaths));
    await flushTimers();
  });
  return root;
}

console.log("\nassistant file links");

installDom();

const oldOwnerPaths = new Set(["reports/shared-name.xlsx"]);
eq(
  visibleWorkspaceFilePathKeys({ ownerKey: "tab:tab-a", paths: oldOwnerPaths }, "tab:tab-b").size,
  0,
  "owner-bound verification hides old paths synchronously on the first render for a new owner",
);
eq(resolveWorkspaceFileOwner(undefined, {}).kind, "none", "missing file owner fails closed");
eq(resolveWorkspaceFileOwner("   ", { workspaceRoot: "  " }).kind, "none", "blank file owner fails closed");
eq(resolveWorkspaceFileOwner(undefined, { allowActiveWorkspace: true }).kind, "active", "active workspace access requires an explicit opt-in");

const opened: string[] = [];
const root = await renderAssistant(
  [
    "已生成交付物：",
    "@reports/finance-summary.xlsx",
    "[经营诊断.docx](D:/project/KWU/reports/经营诊断.docx)",
  ].join("\n"),
  async (path) => {
    opened.push(path);
  },
);

const cards = Array.from(document.querySelectorAll<HTMLElement>(".msg--assistant .msg-attachment"));
ok(cards.length >= 2, "assistant messages render generated file references as attachment cards");
ok(document.body.textContent?.includes("finance-summary.xlsx") === true, "assistant file card shows xlsx name");
ok(document.body.textContent?.includes("经营诊断.docx") === true, "assistant markdown local file card shows docx name");
ok(cards.every((card) => card.textContent?.includes("工作区引用")), "text-derived file cards remain workspace references");

await act(async () => {
  cards[0]?.click();
  await flushTimers();
});
eq(opened[0], "reports/finance-summary.xlsx", "clicking assistant file card opens the workspace file");

await act(async () => {
  root.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const bareFileRoot = await renderAssistant(
  "已生成工作簿：quarterly_business_diagnosis.xlsx",
  async (path) => {
    opened.push(path);
  },
);
const bareFileLink = await waitFor(() => {
  const link = document.querySelector<HTMLAnchorElement>(".msg--assistant a");
  if (link?.textContent !== "quarterly_business_diagnosis.xlsx") throw new Error("bare file link has not settled");
  return link;
});
ok(bareFileLink?.textContent === "quarterly_business_diagnosis.xlsx", "existing bare generated filename renders as an inline link");
await act(async () => {
  bareFileLink?.click();
  await flushTimers();
});
eq(opened.at(-1), "quarterly_business_diagnosis.xlsx", "clicking a bare generated filename link opens the workspace file");

await act(async () => {
  bareFileRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const htmlArtifactRoot = await renderAssistant(
  ["文件已创建并验证通过:", "`/workspace/snake/index.html` (13,111 字节)"].join("\n"),
  async (path) => {
    opened.push(path);
  },
);
await act(async () => {
  await flushTimers();
});
const htmlArtifactCard = document.querySelector<HTMLButtonElement>(".msg--assistant button.msg-attachment");
ok(htmlArtifactCard?.getAttribute("title") === "/workspace/snake/index.html", "verified HTML artifacts render as a file card");
await act(async () => {
  htmlArtifactCard?.click();
  await flushTimers();
});
eq(opened.at(-1), "/workspace/snake/index.html", "clicking a verified HTML artifact card opens the workspace file");

await act(async () => {
  htmlArtifactRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const missingHtmlArtifactRoot = await renderAssistant(
  "尚未生成：`/workspace/snake/missing.html`",
  async (path) => {
    opened.push(path);
  },
  async (path) => ({ path, body: "", size: 0, truncated: false, binary: false, err: "file does not exist" }),
);
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") === null, "missing HTML artifacts do not render a clickable file card");
ok(document.querySelector<HTMLAnchorElement>(".msg--assistant a") === null, "missing HTML artifacts do not render a broken link");
ok(document.body.textContent?.includes("/workspace/snake/missing.html") === true, "missing HTML artifact remains visible");

await act(async () => {
  missingHtmlArtifactRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const chineseBareFileRoot = await renderAssistant(
  "文件路径：季度经营诊断与决策方案.xlsx",
  async (path) => {
    opened.push(path);
  },
);
const chineseBareFileLink = await waitFor(() => {
  const link = document.querySelector<HTMLAnchorElement>(".msg--assistant a");
  if (link?.textContent !== "季度经营诊断与决策方案.xlsx") throw new Error("Chinese bare file link has not settled");
  return link;
});
ok(chineseBareFileLink?.textContent === "季度经营诊断与决策方案.xlsx", "Chinese bare generated filename renders as an inline link");
await act(async () => {
  chineseBareFileLink?.click();
  await flushTimers();
});
eq(opened.at(-1), "季度经营诊断与决策方案.xlsx", "clicking a Chinese generated filename opens the decoded workspace path");

await act(async () => {
  chineseBareFileRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const missingBareRoot = await renderAssistant(
  "尚未生成：missing_quarterly_business_diagnosis.xlsx",
  async (path) => {
    opened.push(path);
  },
  async (path) => ({ path, body: "", size: 0, truncated: false, binary: false, err: "file does not exist" }),
);
ok(document.querySelector<HTMLAnchorElement>(".msg--assistant a") === null, "missing bare generated filename stays plain text");
ok(document.body.textContent?.includes("missing_quarterly_business_diagnosis.xlsx") === true, "missing bare generated filename remains visible");

await act(async () => {
  missingBareRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const duplicateRoot = await renderAssistant(
  "@D:/reports/quarter.xlsx @D:\\reports\\quarter.xlsx @D:/reports/quarter.xlsx",
  async (path) => {
    opened.push(path);
  },
);
const duplicateCards = Array.from(document.querySelectorAll<HTMLButtonElement>(".msg--assistant button.msg-attachment"));
eq(duplicateCards.length, 1, "duplicate generated file references render as one clickable card");
await act(async () => {
  duplicateCards[0]?.click();
  await flushTimers();
});
eq(opened.at(-1), "D:/reports/quarter.xlsx", "deduplicated generated file card keeps a working open target");

await act(async () => {
  duplicateRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const aliasedPathRoot = await renderAssistant(
  "`/home/user/workspace/snake-game.html` `snake-game.html`",
  async (path) => {
    opened.push(path);
  },
);
const aliasedPathCards = Array.from(document.querySelectorAll<HTMLButtonElement>(".msg--assistant button.msg-attachment"));
eq(aliasedPathCards.length, 1, "absolute and workspace-relative aliases render as one generated file card");
await act(async () => {
  aliasedPathCards[0]?.click();
  await flushTimers();
});
eq(opened.at(-1), "/home/user/workspace/snake-game.html", "the deduplicated generated file card keeps the first verified path");

await act(async () => {
  aliasedPathRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const fallbackAliasReads: string[] = [];
const fallbackAliasRoot = await renderAssistant(
  "`C:\\workspace\\snake.html` `snake.html`",
  async (path) => {
    opened.push(path);
  },
  async (path) => {
    fallbackAliasReads.push(path);
    if (path !== "snake.html") {
      return { path, body: "", size: 0, truncated: false, binary: false, err: "file does not exist" };
    }
    return { path, body: "", size: 0, truncated: false, binary: false };
  },
);
eq(JSON.stringify(fallbackAliasReads), JSON.stringify(["C:\\workspace\\snake.html", "snake.html"]), "invalid absolute and valid relative aliases are both verified");
const fallbackAliasCards = Array.from(document.querySelectorAll<HTMLButtonElement>(".msg--assistant button.msg-attachment"));
eq(fallbackAliasCards.length, 1, "valid relative alias renders one file card after the absolute alias fails");
eq(fallbackAliasCards[0]?.getAttribute("title"), "snake.html", "file card keeps the actually verified relative alias");
await act(async () => {
  fallbackAliasCards[0]?.click();
  await flushTimers();
});
eq(opened.at(-1), "snake.html", "fallback relative alias opens successfully");
await act(async () => {
  fallbackAliasRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const successfulAliasReads: string[] = [];
const successfulAliasRoot = await renderAssistant(
  "`C:\\workspace\\both-exist.html` `both-exist.html`",
  async (path) => {
    opened.push(path);
  },
  async (path) => {
    successfulAliasReads.push(path);
    return { path, body: "", size: 0, truncated: false, binary: false };
  },
);
eq(JSON.stringify(successfulAliasReads), JSON.stringify(["C:\\workspace\\both-exist.html", "both-exist.html"]), "two existing aliases are both verified");
eq(document.querySelectorAll(".msg--assistant .msg-attachment").length, 1, "two verified aliases still render one file card");
await act(async () => {
  successfulAliasRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const structuredReads: string[] = [];
const structuredCanonicalPath = "C:\\workspace\\exports\\structured-deliverable.html";
const structuredRoot = await renderAssistant(
  "交付完成；文本别名为 `structured-deliverable.html`。",
  async (path) => {
    opened.push(path);
  },
  async (path) => {
    structuredReads.push(path);
    return { path, body: "", size: 1, truncated: false, binary: false };
  },
  { artifactPaths: [structuredCanonicalPath] },
);
eq(JSON.stringify(structuredReads), JSON.stringify([structuredCanonicalPath, "structured-deliverable.html"]), "structured and text-derived exact candidates are both verified");
const structuredCards = Array.from(document.querySelectorAll<HTMLButtonElement>(".msg--assistant button.msg-attachment"));
eq(structuredCards.length, 1, "structured canonical path and text alias render one card after verification");
eq(structuredCards[0]?.getAttribute("title"), structuredCanonicalPath, "structured host path is the authoritative card target");
ok(structuredCards[0]?.textContent?.includes("已交付产物") === true, "structured artifact path renders as a delivered artifact");
ok(structuredCards[0]?.textContent?.includes("工作区引用") === false, "structured artifact wins over an equivalent text-derived workspace reference");
await act(async () => {
  structuredCards[0]?.click();
  await flushTimers();
});
eq(opened.at(-1), structuredCanonicalPath, "structured artifact card opens the host path");
await act(async () => {
  structuredRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const explicitHiddenPath = "C:/workspace/.playwright-cli/page-private.yml";
const explicitHiddenRoot = await renderAssistant(
  `[显式测试证据](${explicitHiddenPath})`,
  async (path) => {
    opened.push(path);
  },
);
const explicitHiddenCards = Array.from(document.querySelectorAll<HTMLButtonElement>(".msg--assistant button.msg-attachment"));
eq(explicitHiddenCards.length, 1, "an explicitly linked hidden support path remains available as a text-derived card");
eq(explicitHiddenCards[0]?.getAttribute("title"), explicitHiddenPath, "the explicit hidden support card keeps its referenced path");
await act(async () => {
  explicitHiddenCards[0]?.click();
  await flushTimers();
});
eq(opened.at(-1), explicitHiddenPath, "the explicit hidden support card opens its referenced path");
await act(async () => {
  explicitHiddenRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const missingStructuredRoot = await renderAssistant(
  "交付未确认。",
  async (path) => {
    opened.push(path);
  },
  async (path) => ({ path, body: "", size: 0, truncated: false, binary: false, err: "file does not exist" }),
  { artifactPaths: ["C:\\workspace\\exports\\missing-structured.html"] },
);
eq(document.querySelectorAll(".msg--assistant .msg-attachment").length, 0, "missing structured artifact path renders no card");
await act(async () => {
  missingStructuredRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const missingAliasReads: string[] = [];
const missingAliasRoot = await renderAssistant(
  "`C:\\workspace\\both-missing.html` `both-missing.html`",
  async (path) => {
    opened.push(path);
  },
  async (path) => {
    missingAliasReads.push(path);
    return { path, body: "", size: 0, truncated: false, binary: false, err: "file does not exist" };
  },
);
eq(JSON.stringify(missingAliasReads), JSON.stringify(["C:\\workspace\\both-missing.html", "both-missing.html"]), "two missing aliases are both verified");
eq(document.querySelectorAll(".msg--assistant .msg-attachment").length, 0, "two missing aliases render no file card");
eq(document.querySelectorAll(".msg--assistant a").length, 0, "two missing aliases render no broken link");
await act(async () => {
  missingAliasRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const mixedAliasRoot = await renderAssistant(
  "[/home/user/workspace/snake-game.html](/home/user/workspace/snake-game.html) `snake-game.html`",
  async (path) => {
    opened.push(path);
  },
);
const mixedAliasCards = Array.from(document.querySelectorAll<HTMLButtonElement>(".msg--assistant button.msg-attachment"));
eq(mixedAliasCards.length, 1, "absolute Markdown links and relative inline artifacts render as one file card");

await act(async () => {
  mixedAliasRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const missingInlineRoot = await renderAssistant(
  ["已为你生成 Excel 文件:", "`不存在的表格.xlsx`"].join("\n"),
  async (path) => {
    opened.push(path);
  },
  async (path) => ({
    path,
    body: "",
    size: 0,
    truncated: false,
    binary: false,
    err: "file does not exist",
  }),
);
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") === null, "assistant inline generated filenames require an existing workspace file");
ok(document.body.textContent?.includes("不存在的表格.xlsx") === true, "missing inline generated filenames stay visible as text");

await act(async () => {
  missingInlineRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const missingLinkedRoot = await renderAssistant(
  ["交付文件：", "@missing/report.xlsx", "[决策方案.xlsx](missing/决策方案.xlsx)"].join("\n"),
  async (path) => {
    opened.push(path);
  },
  async (path) => ({ path, body: "", size: 0, truncated: false, binary: false, err: "file does not exist" }),
);
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") === null, "missing reference and markdown files do not render fake attachment cards");
ok(document.body.textContent?.includes("missing/report.xlsx") === true, "missing workspace reference remains visible as plain text");
ok(document.body.textContent?.includes("决策方案.xlsx") === true, "missing markdown file remains visible as plain text");
ok(document.querySelector<HTMLAnchorElement>(".msg--assistant a") === null, "missing markdown file is not left as a broken clickable link");

await act(async () => {
  missingLinkedRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const failingRoot = await renderAssistant(
  "@reports/missing-file.txt",
  async () => {
    throw new Error("forced open failure");
  },
);
const failingCard = document.querySelector<HTMLElement>(".msg--assistant .msg-attachment");
if (!failingCard) throw new Error("failing assistant file card did not render");

await act(async () => {
  failingCard.click();
  await flushTimers();
});

ok(document.body.textContent?.includes("无法打开文件。") === true, "failed assistant file opens show a visible toast");

await act(async () => {
  failingRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const openedForTab: Array<[string, string]> = [];
const tabScopedRoot = await renderAssistant(
  ["已生成专家团文件:", "`reports/team-output.xlsx`"].join("\n"),
  async () => {
    throw new Error("global workspace should not be used for tab-scoped assistant files");
  },
  async (path) => ({
    path,
    body: "",
    size: 0,
    truncated: false,
    binary: false,
    err: "wrong workspace",
  }),
  {
    tabId: "tab-expert-team",
    readFileForTab: async (tabID, path) => ({
      path,
      body: `${tabID}:${path}`,
      size: 1,
      truncated: false,
      binary: false,
    }),
    openWorkspacePathForTab: async (tabID, path) => {
      openedForTab.push([tabID, path]);
    },
  },
);
const tabScopedCard = document.querySelector<HTMLElement>(".msg--assistant .msg-attachment");
ok(tabScopedCard !== null, "tab-scoped assistant generated files verify against the owning tab workspace");

await act(async () => {
  tabScopedCard?.click();
  await flushTimers();
});

eq(JSON.stringify(openedForTab[0]), JSON.stringify(["tab-expert-team", "reports/team-output.xlsx"]), "clicking a tab-scoped generated file opens it in the owning tab workspace");

await act(async () => {
  tabScopedRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
const scopedText = "`reports/shared-name.xlsx`";
const changingTabRoot = await renderAssistant(
  scopedText,
  async () => {},
  async (path) => ({ path, body: "", size: 0, truncated: false, binary: false, err: "global workspace must not be used" }),
  {
    tabId: "tab-a",
    readFileForTab: async (tabID, path) => ({
      path,
      body: "",
      size: 0,
      truncated: false,
      binary: false,
      ...(tabID === "tab-a" ? {} : { err: "missing in tab-b" }),
    }),
    openWorkspacePathForTab: async () => {},
  },
);
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") !== null, "tab A verifies its generated file");
await act(async () => {
  changingTabRoot.render(assistantView(scopedText, "tab-b"));
  await flushTimers();
  await flushTimers();
});
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") === null, "switching to tab B drops tab A's cached file verification");
await act(async () => {
  changingTabRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
let ownerlessActiveReads = 0;
let ownerlessActiveOpens = 0;
let ownerlessTabReads = 0;
const ownerlessRoot = await renderAssistant(
  "`reports/ownerless.xlsx`",
  async () => {
    ownerlessActiveOpens += 1;
  },
  async (path) => {
    ownerlessActiveReads += 1;
    return { path, body: "", size: 0, truncated: false, binary: false };
  },
  {
    tabId: "",
    readFileForTab: async (_tabID, path) => {
      ownerlessTabReads += 1;
      return { path, body: "", size: 0, truncated: false, binary: false };
    },
  },
);
eq(ownerlessActiveReads, 0, "ownerless assistant verification never reads the active workspace");
eq(ownerlessTabReads, 0, "ownerless assistant verification never invents a tab owner");
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") === null, "ownerless assistant files remain non-interactive");
eq(ownerlessActiveOpens, 0, "ownerless assistant files never open the active workspace");
await act(async () => {
  ownerlessRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
let blankTabReads = 0;
const blankOwnerRoot = await renderAssistant(
  "`reports/blank-owner.xlsx`",
  async () => {},
  async (path) => ({ path, body: "", size: 0, truncated: false, binary: false }),
  {
    tabId: "   ",
    readFileForTab: async (_tabID, path) => {
      blankTabReads += 1;
      return { path, body: "", size: 0, truncated: false, binary: false };
    },
  },
);
eq(blankTabReads, 0, "blank assistant tab IDs fail closed instead of reaching tab-scoped APIs");
ok(document.querySelector<HTMLElement>(".msg--assistant .msg-attachment") === null, "blank-owner assistant files remain non-interactive");
await act(async () => {
  blankOwnerRoot.unmount();
});

document.body.innerHTML = "<div id=\"root\"></div>";
let markdownActiveOpens = 0;
let markdownTabOpens = 0;
window.go = {
  main: {
    App: {
      OpenWorkspacePath: async () => {
        markdownActiveOpens += 1;
      },
      OpenWorkspacePathForTab: async () => {
        markdownTabOpens += 1;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};
const markdownRootElement = document.getElementById("root");
if (!markdownRootElement) throw new Error("missing markdown root");
const markdownRoot = createRoot(markdownRootElement);
await act(async () => {
  markdownRoot.render(
    <LocaleProvider>
      <ToastProvider>
        <Markdown text="[ownerless.xlsx](reports/ownerless.xlsx)" />
      </ToastProvider>
    </LocaleProvider>,
  );
  await flushTimers();
  await flushTimers();
});
const ownerlessMarkdownLink = document.querySelector<HTMLAnchorElement>("a");
ok(ownerlessMarkdownLink !== null, "ownerless local Markdown links still render visibly");
await act(async () => {
  ownerlessMarkdownLink?.click();
  await flushTimers();
});
eq(markdownActiveOpens, 0, "ownerless local Markdown links never open the active workspace");
eq(markdownTabOpens, 0, "ownerless local Markdown links never invent a tab owner");
ok(document.body.textContent?.includes("无法打开文件。") === true, "ownerless local Markdown links use the existing failure toast");
await act(async () => {
  markdownRoot.unmount();
});

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
