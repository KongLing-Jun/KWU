import { readFileSync } from "node:fs";
import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import {
  ProjectTree,
  PROJECT_TREE_SHOW_MORE_THRESHOLD,
  projectTreeVisibleNodes,
} from "../components/ProjectTree";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ProjectNode } from "../lib/types";

const baseStyles = readFileSync(new URL("../styles/base.css", import.meta.url), "utf8");

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await act(async () => {
      await flush();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function topic(scope: "global" | "project", index: number, root = ""): ProjectNode {
  const prefix = scope === "global" ? "Global" : root.endsWith("a") ? "Workspace A" : "Workspace B";
  return {
    key: `${scope}-${root}-${index}`,
    kind: scope === "global" ? "global_topic" : "topic",
    label: `${prefix} task ${index}`,
    root,
    topicId: `${scope}-${root}-${index}`,
    sessionPath: `C:\\sessions\\${scope}-${root}-${index}.jsonl`,
    sessionTurns: 1,
    lastActivityAt: 10_000 - index,
  };
}

const globalTopics = Array.from({ length: 7 }, (_, index) => topic("global", index + 1));
const workspaceATopics = Array.from({ length: 7 }, (_, index) => topic("project", index + 1, "C:\\workspace-a"));
const workspaceBTopics = Array.from({ length: 7 }, (_, index) => topic("project", index + 1, "C:\\workspace-b"));
globalTopics[0] = { ...globalTopics[0]!, label: "新的会话", sessionTurns: 0, turns: 0 };
workspaceATopics[0] = { ...workspaceATopics[0]!, label: "新的会话", sessionTurns: 0, turns: 0 };
const tree: ProjectNode[] = [
  { key: "global", kind: "global_folder", label: "任务", children: globalTopics },
  { key: "workspace-a", kind: "project", label: "Workspace A", root: "C:\\workspace-a", children: workspaceATopics },
  { key: "workspace-b", kind: "project", label: "Workspace B", root: "C:\\workspace-b", children: workspaceBTopics },
];

const dom = new JSDOM("<!doctype html><html><body></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
Object.defineProperty(globalThis, "localStorage", { configurable: true, value: dom.window.localStorage });
globalThis.Node = dom.window.Node;
globalThis.Element = dom.window.Element;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.HTMLButtonElement = dom.window.HTMLButtonElement;
globalThis.MouseEvent = dom.window.MouseEvent;
Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });

window.go = {
  main: {
    App: {
      ListProjectTree: async () => tree,
      Platform: async () => "windows",
    } as Partial<AppBindings> as AppBindings,
  },
};

type MountOptions = {
  activeScope?: "global" | "project";
  activeWorkspaceRoot?: string;
  activeTopicId?: string;
  activeSessionPath?: string;
  timeFilter?: "all" | "20";
  creation?: boolean;
};

async function mount(options: MountOptions = {}) {
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  let visibleTopicIds: string[] = [];
  const renderTree = (current: MountOptions) => (
    <LocaleProvider>
      <ToastProvider>
        <ProjectTree
          activeScope={current.activeScope}
          activeWorkspaceRoot={current.activeWorkspaceRoot}
          activeTopicId={current.activeTopicId}
          activeSessionPath={current.activeSessionPath}
          variant={current.creation ? "creation" : "classic"}
          onOpenTopic={() => {}}
          onOpenProjectHistory={() => {}}
          onAddProject={async () => {}}
          timeFilter={current.timeFilter ?? "all"}
          onTimeFilterChange={() => {}}
          showShortcutBadges
          onVisibleTopicsChange={(entries) => {
            visibleTopicIds = entries.map((entry) => entry.topicId);
          }}
        />
      </ToastProvider>
    </LocaleProvider>
  );

  await act(async () => {
    root.render(renderTree(options));
    await flush();
  });
  await waitFor("project tree", () => host.textContent?.includes("工作空间 (2)") === true);

  const section = (modifier: "tasks" | "workspaces") => {
    const element = host.querySelector<HTMLElement>(`.project-tree__task-section--${modifier}`);
    ok(element, `missing ${modifier} section`);
    return element;
  };
  const showMore = (scope: HTMLElement, modifier: "tasks" | "workspace") => {
    const button = scope.querySelector<HTMLButtonElement>(`.project-tree__show-more--${modifier}`);
    ok(button, `missing ${modifier} show-more button`);
    ok(button.classList.contains("project-tree__topic"), `${modifier} show-more must reuse the session row box`);
    ok(button.querySelector(".project-tree__show-more-label"), `${modifier} show-more must keep its hierarchy on the label`);
    return button;
  };
  const workspace = (label: string) => {
    const wrapper = [...host.querySelectorAll<HTMLElement>(".project-tree__project-wrapper")]
      .find((candidate) => candidate.querySelector(".project-tree__folder-label")?.textContent === label);
    ok(wrapper, `missing workspace ${label}`);
    return wrapper;
  };

  return {
    host,
    root,
    section,
    showMore,
    workspace,
    visibleTopicIds: () => visibleTopicIds,
    rerender: async (nextOptions: MountOptions) => {
      await act(async () => {
        root.render(renderTree(nextOptions));
        await flush();
      });
    },
  };
}

async function click(element: HTMLElement) {
  await act(async () => {
    element.click();
    await flush();
  });
}

async function unmount(root: Root, host: HTMLElement) {
  await act(async () => root.unmount());
  host.remove();
}

ok(PROJECT_TREE_SHOW_MORE_THRESHOLD === 5, "WorkBuddy-aligned threshold must stay at five");
ok(projectTreeVisibleNodes(globalTopics, false).length === 5, "collapsed helper returns five nodes");
ok(projectTreeVisibleNodes(globalTopics, true).length === 7, "expanded helper returns all nodes");
ok(projectTreeVisibleNodes(globalTopics, false, true).length === 7, "filters bypass the display limit");
ok(
  /\.app--creation \.project-tree__topic\.project-tree__show-more,[\s\S]*?min-height:\s*26px;/.test(baseStyles),
  "creation show-more matches the rendered session row height",
);
ok(
  /\.sidebar--workbench \.project-tree__topic \{\s*min-height:\s*34px;/.test(baseStyles),
  "workbench session rows keep their 34px height",
);
ok(
  /\.project-tree__show-more:focus-visible,[\s\S]*?background:\s*var\(--sidebar-hover\);[\s\S]*?color:\s*var\(--fg\);[\s\S]*?box-shadow:\s*none;/.test(baseStyles),
  "keyboard focus uses the session-row background without a show-more focus ring",
);
ok(
  /\.project-tree__show-more:focus-within:not\(:focus-visible\):not\(:hover\)[\s\S]*?background:\s*transparent;[\s\S]*?color:\s*var\(--fg-dim\);/.test(baseStyles),
  "pointer focus does not leave a show-more hover background behind",
);

window.localStorage.clear();
{
  const harness = await mount();
  const tasks = harness.section("tasks");
  const workspaces = harness.section("workspaces");
  ok(tasks.textContent?.includes("Global task 5"), "collapsed tasks include the fifth item");
  ok(!tasks.textContent?.includes("Global task 6"), "collapsed tasks hide the sixth item");
  ok(harness.showMore(tasks, "tasks").textContent === "查看更多 (2)", "task button reports the hidden count");
  ok(workspaces.textContent?.includes("工作空间 (2)"), "workspace header reports every workspace");
  ok(workspaces.textContent?.includes("Workspace A") && workspaces.textContent?.includes("Workspace B"), "workspace list itself is never truncated");
  await waitFor("collapsed shortcuts", () => harness.visibleTopicIds().length === 5);
  ok(!harness.visibleTopicIds().includes(globalTopics[5]!.topicId!), "hidden task is excluded from number shortcuts");

  await click(harness.showMore(tasks, "tasks"));
  ok(tasks.textContent?.includes("Global task 7"), "task show-more reveals every task");
  ok(harness.showMore(tasks, "tasks").textContent === "收起", "expanded task button becomes show-less");

  const workspaceA = harness.workspace("Workspace A");
  await click(workspaceA.querySelector<HTMLButtonElement>(".project-tree__folder-main")!);
  ok(workspaceA.textContent?.includes("Workspace A task 5"), "workspace shows five tasks by default");
  ok(!workspaceA.textContent?.includes("Workspace A task 6"), "workspace hides tasks after the threshold");
  await click(harness.showMore(workspaceA, "workspace"));
  ok(workspaceA.textContent?.includes("Workspace A task 7"), "workspace show-more reveals every task");

  const workspaceB = harness.workspace("Workspace B");
  await click(workspaceB.querySelector<HTMLButtonElement>(".project-tree__folder-main")!);
  ok(!workspaceB.textContent?.includes("Workspace B task 6"), "workspace expansion state is independent");
  ok(harness.showMore(workspaceB, "workspace").textContent === "查看更多 (2)", "other workspace remains collapsed");
  await unmount(harness.root, harness.host);
}

{
  const harness = await mount();
  ok(!harness.section("tasks").textContent?.includes("Global task 7"), "task expansion resets after remount");
  ok(harness.showMore(harness.section("tasks"), "tasks").textContent === "查看更多 (2)", "task remount returns to the default five rows");
  const workspaceA = harness.workspace("Workspace A");
  await click(workspaceA.querySelector<HTMLButtonElement>(".project-tree__folder-main")!);
  ok(!workspaceA.textContent?.includes("Workspace A task 7"), "workspace expansion resets after remount");
  ok(harness.showMore(workspaceA, "workspace").textContent === "查看更多 (2)", "workspace remount returns to the default five rows");
  await unmount(harness.root, harness.host);
}

window.localStorage.clear();
{
  const active = workspaceATopics[6]!;
  const harness = await mount({
    activeScope: "project",
    activeWorkspaceRoot: "C:\\workspace-a",
    activeTopicId: active.topicId,
    activeSessionPath: active.sessionPath,
  });
  const workspaceA = harness.workspace("Workspace A");
  await waitFor("active workspace task reveal", () => workspaceA.textContent?.includes("Workspace A task 7") === true);
  ok(harness.showMore(workspaceA, "workspace").textContent === "收起", "active workspace beyond the threshold expands on mount");
  await click(harness.showMore(workspaceA, "workspace"));
  ok(!workspaceA.textContent?.includes("Workspace A task 7"), "user can explicitly hide the active workspace task");
  await harness.rerender({
    activeScope: "project",
    activeWorkspaceRoot: "C:\\workspace-b",
    activeTopicId: workspaceBTopics[6]!.topicId,
    activeSessionPath: workspaceBTopics[6]!.sessionPath,
  });
  ok(!workspaceA.textContent?.includes("Workspace A task 7"), "another workspace reveal does not reopen a manually collapsed workspace");
  await waitFor("other active workspace reveal", () => harness.workspace("Workspace B").textContent?.includes("Workspace B task 7") === true);
  await unmount(harness.root, harness.host);
}

window.localStorage.clear();
{
  const active = globalTopics[6]!;
  const harness = await mount({
    activeScope: "global",
    activeTopicId: active.topicId,
    activeSessionPath: active.sessionPath,
  });
  const tasks = harness.section("tasks");
  await waitFor("active global task reveal", () => tasks.textContent?.includes("Global task 7") === true);
  ok(tasks.querySelectorAll(".project-tree__topic-main").length === 7, "active global task beyond the threshold expands on mount");
  ok(harness.showMore(tasks, "tasks").textContent === "收起", "active global task uses the expanded button state");
  await harness.rerender({
    activeScope: "global",
    activeTopicId: globalTopics[1]!.topicId,
    activeSessionPath: globalTopics[1]!.sessionPath,
  });
  ok(tasks.querySelectorAll(".project-tree__topic-main").length === 7, "switching to a recent task does not automatically collapse the list");
  await click(harness.showMore(tasks, "tasks"));
  ok(tasks.querySelectorAll(".project-tree__topic-main").length === 5, "user can explicitly collapse an automatically expanded task list");
  await harness.rerender({
    activeScope: "global",
    activeTopicId: active.topicId,
    activeSessionPath: active.sessionPath,
  });
  ok(tasks.querySelectorAll(".project-tree__topic-main").length === 5, "manual collapse suppresses automatic task re-expansion until remount");
  await unmount(harness.root, harness.host);
}

window.localStorage.clear();
{
  const active = globalTopics[1]!;
  const harness = await mount({
    activeScope: "global",
    activeTopicId: active.topicId,
    activeSessionPath: active.sessionPath,
  });
  const tasks = harness.section("tasks");
  ok(tasks.querySelectorAll(".project-tree__topic-main").length === 5, "active global task within the first five keeps the list collapsed");
  ok(harness.showMore(tasks, "tasks").textContent === "查看更多 (2)", "recent active task keeps the collapsed button state");
  await unmount(harness.root, harness.host);
}

window.localStorage.clear();
{
  const promotedIndex = 6;
  const originalTopic = globalTopics[promotedIndex]!;
  globalTopics[promotedIndex] = { ...originalTopic, lastActivityAt: 20_000 };
  const harness = await mount({ creation: true });
  const labels = [...harness.section("tasks").querySelectorAll<HTMLElement>(".project-tree__topic-label")]
    .map((element) => element.textContent);
  ok(labels[0] === "Global task 7", "creation sidebar promotes the most recently active old task to the first row");
  await unmount(harness.root, harness.host);
  globalTopics[promotedIndex] = originalTopic;
}

window.localStorage.clear();
{
  const activeDraft = globalTopics[0]!;
  const harness = await mount({
    creation: true,
    activeScope: "global",
    activeTopicId: activeDraft.topicId,
    activeSessionPath: activeDraft.sessionPath,
  });
  const tasks = harness.section("tasks");
  ok(tasks.textContent?.includes("任务 (6)"), "hidden zero-turn default draft is excluded from the task count");
  ok(tasks.querySelectorAll(".project-tree__topic-main").length === 5, "hidden zero-turn default draft does not consume a visible slot");
  ok(harness.showMore(tasks, "tasks").textContent === "查看更多 (1)", "hidden zero-turn default draft is excluded from the remaining count");
  await unmount(harness.root, harness.host);
}

window.localStorage.clear();
{
  const activeDraft = workspaceATopics[0]!;
  const harness = await mount({
    creation: true,
    activeScope: "project",
    activeWorkspaceRoot: "C:\\workspace-a",
    activeTopicId: activeDraft.topicId,
    activeSessionPath: activeDraft.sessionPath,
  });
  const workspaceA = harness.workspace("Workspace A");
  await waitFor("workspace without hidden zero-turn default draft slot", () => workspaceA.querySelectorAll(".project-tree__topic-main").length === 5);
  ok(!workspaceA.textContent?.includes("新的会话"), "workspace hides its zero-turn default draft");
  ok(workspaceA.textContent?.includes("Workspace A task 6"), "workspace hidden zero-turn default draft does not consume a visible slot");
  ok(harness.showMore(workspaceA, "workspace").textContent === "查看更多 (1)", "workspace hidden zero-turn default draft is excluded from the remaining count");
  await unmount(harness.root, harness.host);
}

window.localStorage.clear();
{
  const harness = await mount({ timeFilter: "20" });
  ok(harness.section("tasks").textContent?.includes("Global task 7"), "time-filtered results bypass the display limit");
  ok(!harness.section("tasks").querySelector(".project-tree__show-more"), "time filter does not render a display limit button");
  await unmount(harness.root, harness.host);
}

dom.window.close();
console.log("project tree show-more behavior passed");
