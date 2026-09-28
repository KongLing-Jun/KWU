import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ProjectTree } from "../components/ProjectTree";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { ProjectNode, TopicMeta } from "../lib/types";
import { conversationAttentionIdentity } from "../lib/conversationAttention";

const workspaceRoot = "C:\\workspace";
const tree: ProjectNode[] = [
  {
    key: "global",
    kind: "global_folder",
    label: "任务",
    children: [],
  },
  {
    key: "workspace",
    kind: "project",
    label: "Workspace",
    root: workspaceRoot,
    children: [],
  },
];

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function installBridge(overrides: Partial<AppBindings> = {}) {
  window.go = {
    main: {
      App: {
        ListProjectTree: vi.fn(async () => tree),
        Platform: vi.fn(async () => "windows"),
        ...overrides,
      } as Partial<AppBindings> as AppBindings,
    },
  };
}

type RenderTreeOptions = {
  onCreateTopic?: (scope: string, workspaceRoot: string) => Promise<void> | void;
  onOpenTopic?: (scope: string, workspaceRoot: string, topicId: string) => Promise<void> | void;
  onCreateCloudProjectTopic?: (projectId: string) => Promise<void> | void;
  onTopicsChanged?: () => Promise<void> | void;
  refreshSignal?: number;
  variant?: "classic" | "workbench" | "creation";
  unreadCompletionIds?: ReadonlySet<string>;
};

function treeElement(options: RenderTreeOptions = {}) {
  return (
    <LocaleProvider>
      <ToastProvider>
        <ProjectTree
          onOpenTopic={options.onOpenTopic ?? vi.fn()}
          onCreateCloudProjectTopic={options.onCreateCloudProjectTopic}
          onOpenProjectHistory={vi.fn()}
          onAddProject={vi.fn(async () => {})}
          onCreateTopic={options.onCreateTopic}
          onTopicsChanged={options.onTopicsChanged}
          refreshSignal={options.refreshSignal}
          variant={options.variant}
          unreadCompletionIds={options.unreadCompletionIds}
          timeFilter="all"
          onTimeFilterChange={vi.fn()}
        />
      </ToastProvider>
    </LocaleProvider>
  );
}

function renderTree(options: RenderTreeOptions = {}) {
  return render(treeElement(options));
}

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});

describe("ProjectTree create topic errors", () => {
  it("renders collapsed child activity and unread completion on the visible Creation row", async () => {
    const runningPath = "C:\\sessions\\running.jsonl";
    const idlePath = "C:\\sessions\\idle.jsonl";
    const creationTree: ProjectNode[] = [{
      key: "global",
      kind: "global_folder",
      label: "任务",
      children: [{
        key: "topic",
        kind: "global_topic",
        label: "后台任务",
        topicId: "topic",
        turns: 2,
        children: [
          { key: "idle", kind: "global_session", label: "Idle", topicId: "topic", sessionPath: idlePath, turns: 1 },
          { key: "running", kind: "global_session", label: "Running", topicId: "topic", sessionPath: runningPath, turns: 1, status: "streaming", running: true },
        ],
      }],
    }];
    const listProjectTree = vi.fn(async () => creationTree);
    installBridge({ ListProjectTree: listProjectTree });
    const view = renderTree({ variant: "creation" });

    const topicButton = await screen.findByRole("button", { name: /后台任务/ });
    const runningIndicator = within(topicButton).getByRole("img", { name: "运行中" });
    expect(runningIndicator).toHaveClass("project-tree__topic-state--running");

    creationTree[0].children![0].children![1] = {
      ...creationTree[0].children![0].children![1],
      status: undefined,
      running: false,
    };
    const unreadCompletionIds = new Set([
      conversationAttentionIdentity({ kind: "global_session", sessionPath: runningPath }),
    ]);
    view.rerender(treeElement({ variant: "creation", unreadCompletionIds, refreshSignal: 1 }));

    const completedIndicator = await waitFor(() => within(topicButton).getByRole("img", { name: "已完成，尚未查看" }));
    expect(completedIndicator).toHaveClass("project-tree__topic-state--completed_unread");
  });

  it("keeps the newest cloud-project order when an older tree refresh resolves last", async () => {
    const olderRefresh = deferred<ProjectNode[]>();
    const newerRefresh = deferred<ProjectNode[]>();
    const localNodes: ProjectNode[] = [
      { key: "local-a", kind: "project", label: "Local A", root: "C:\\local-a" },
      { key: "local-b", kind: "project", label: "Local B", root: "C:\\local-b" },
    ];
    const cloudNode = (id: string, label: string): ProjectNode => ({
      key: `cloud-${id}`, kind: "cloud_project", label, cloudProjectId: id,
    });
    const newProject = cloudNode("project-new", "Cloud New");
    const oldOne = cloudNode("project-old-1", "Cloud Old 1");
    const oldTwo = cloudNode("project-old-2", "Cloud Old 2");
    const listProjectTree = vi.fn()
      .mockReturnValueOnce(olderRefresh.promise)
      .mockReturnValueOnce(newerRefresh.promise);
    installBridge({ ListProjectTree: listProjectTree });
    const view = renderTree({ refreshSignal: 0 });
    await waitFor(() => expect(listProjectTree).toHaveBeenCalledOnce());
    view.rerender(treeElement({ refreshSignal: 1 }));
    await waitFor(() => expect(listProjectTree).toHaveBeenCalledTimes(2));

    await act(async () => {
      newerRefresh.resolve([tree[0], ...localNodes, newProject, oldOne, oldTwo]);
    });
    const labels = () => Array.from(view.container.querySelectorAll(".project-tree__folder-label"), (node) => node.textContent);
    await waitFor(() => expect(labels()).toEqual(["Local A", "Local B", "Cloud New", "Cloud Old 1", "Cloud Old 2"]));

    await act(async () => {
      olderRefresh.resolve([tree[0], ...localNodes, oldOne, oldTwo, newProject]);
    });
    expect(labels()).toEqual(["Local A", "Local B", "Cloud New", "Cloud Old 1", "Cloud Old 2"]);
  });

  it("only toggles a cloud-project parent and creates one project conversation from its plus button", async () => {
    installBridge({
      ListProjectTree: vi.fn(async () => [
        tree[0],
        {
          key: "cloud-project-shared",
          kind: "cloud_project",
          label: "共享云项目",
          cloudProjectId: "project-shared",
          children: [{
            key: "cloud-session-shared",
            kind: "global_session",
            label: "共享项目会话",
            cloudProjectId: "project-shared",
            topicId: "topic-shared",
            sessionPath: "C:\\sessions\\shared.jsonl",
          }],
        } as ProjectNode,
      ]),
    });
    const onOpenTopic = vi.fn();
    const onCreateTopic = vi.fn();
    const onCreateCloudProjectTopic = vi.fn(async () => {});
    renderTree({ onOpenTopic, onCreateTopic, onCreateCloudProjectTopic });

    const projectButton = await screen.findByRole("button", { name: "共享云项目" });
    expect(projectButton).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(projectButton);
    expect(projectButton).toHaveAttribute("aria-expanded", "true");
    expect(onOpenTopic).not.toHaveBeenCalled();
    await userEvent.click(projectButton);
    expect(projectButton).toHaveAttribute("aria-expanded", "false");
    expect(onOpenTopic).not.toHaveBeenCalled();
    const createButton = projectButton.closest(".project-tree__folder")?.querySelector<HTMLButtonElement>('[aria-label="新建会话"]');
    expect(createButton).not.toBeNull();
    await userEvent.click(createButton!);
    expect(onCreateCloudProjectTopic).toHaveBeenCalledOnce();
    expect(onCreateCloudProjectTopic).toHaveBeenCalledWith("project-shared");
    expect(onCreateTopic).not.toHaveBeenCalled();
    expect(onOpenTopic).not.toHaveBeenCalled();

    await userEvent.click(projectButton);
    await userEvent.click(await screen.findByRole("button", { name: /共享项目会话/ }));
    await waitFor(() => expect(onOpenTopic).toHaveBeenCalledWith(
      "global",
      "",
      "topic-shared",
      "C:\\sessions\\shared.jsonl",
      undefined,
      undefined,
      undefined,
      undefined,
      expect.any(Object),
    ));
  });

  it("offers pin and recoverable delete actions for a cloud project session", async () => {
    const setPinned = vi.fn(async () => {});
    const deleteSession = vi.fn(async () => {});
    installBridge({
      ListProjectTree: vi.fn(async () => [
        tree[0],
        {
          key: "cloud-project-shared",
          kind: "cloud_project",
          label: "共享云项目",
          cloudProjectId: "project-shared",
          children: [{
            key: "cloud-session-1",
            kind: "global_session",
            label: "项目会话",
            cloudProjectId: "project-shared",
            topicId: "topic-1",
            sessionPath: "C:\\sessions\\project-session.jsonl",
          }],
        } as ProjectNode,
      ]),
      SetCloudProjectSessionPinned: setPinned,
      DeleteSession: deleteSession,
    });
    renderTree();

    await userEvent.click(await screen.findByRole("button", { name: "共享云项目" }));
    const sessionButton = await screen.findByRole("button", { name: /项目会话/ });
    fireEvent.contextMenu(sessionButton.closest(".project-tree__topic")!);
    await userEvent.click(await screen.findByRole("menuitem", { name: "置顶对话" }));
    expect(setPinned).toHaveBeenCalledWith("C:\\sessions\\project-session.jsonl", true);

    fireEvent.contextMenu(sessionButton.closest(".project-tree__topic")!);
    await userEvent.click(await screen.findByRole("menuitem", { name: "移到回收站" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "确认移入回收站" }));
    expect(deleteSession).toHaveBeenCalledWith("C:\\sessions\\project-session.jsonl");
  });

  it("surfaces one callback error, resets creating state, and permits a successful retry", async () => {
    installBridge();
    const firstAttempt = deferred<void>();
    const onCreateTopic = vi.fn()
      .mockReturnValueOnce(firstAttempt.promise)
      .mockResolvedValueOnce(undefined);
    const onTopicsChanged = vi.fn(async () => {});
    const user = userEvent.setup();
    renderTree({ onCreateTopic, onTopicsChanged });

    const createButton = await screen.findByRole("button", { name: "新建会话" });
    await user.click(createButton);
    expect(createButton).toBeDisabled();

    firstAttempt.reject(new Error("callback create failed"));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("创建新会话失败，请稍后重试。");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(console.error).toHaveBeenCalledWith(
      "[客户端错误]",
      expect.objectContaining({ scope: "projectTree.createTopic" }),
    );
    await waitFor(() => expect(createButton).toBeEnabled());

    await user.click(alert);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    await user.click(createButton);
    await waitFor(() => expect(onCreateTopic).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(createButton).toBeEnabled());
    expect(onTopicsChanged).toHaveBeenCalledOnce();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces one fallback error, resets creating state, and preserves retry success ordering", async () => {
    const firstAttempt = deferred<TopicMeta>();
    const topic = { id: "topic-created" } as TopicMeta;
    const createTopic = vi.fn()
      .mockReturnValueOnce(firstAttempt.promise)
      .mockResolvedValueOnce(topic);
    const order: string[] = [];
    installBridge({
      CreateTopic: createTopic,
      ListProjectTree: vi.fn(async () => {
        order.push("refresh");
        return tree;
      }),
    });
    const onTopicsChanged = vi.fn(async () => {
      order.push("changed");
    });
    const onOpenTopic = vi.fn(async () => {
      order.push("open");
    });
    const user = userEvent.setup();
    renderTree({ onTopicsChanged, onOpenTopic });

    const createButton = await screen.findByRole("button", { name: "新建会话" });
    await user.click(createButton);
    expect(createButton).toBeDisabled();

    firstAttempt.reject(new Error("fallback create failed"));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("创建新会话失败，请稍后重试。");
    expect(screen.getAllByRole("alert")).toHaveLength(1);
    await waitFor(() => expect(createButton).toBeEnabled());

    await user.click(alert);
    order.length = 0;
    await user.click(createButton);
    await waitFor(() => expect(onOpenTopic).toHaveBeenCalledOnce());
    expect(createTopic).toHaveBeenCalledTimes(2);
    expect(createTopic).toHaveBeenLastCalledWith("project", workspaceRoot, "");
    expect(onOpenTopic).toHaveBeenCalledWith("project", workspaceRoot, topic.id);
    expect(order).toEqual(["refresh", "changed", "open"]);
    expect(createButton).toBeEnabled();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
