import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, it, vi } from "vitest";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import { ProjectActivityPanel } from "../features/projects/ui/ProjectActivityPanel";

beforeAll(() => {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
});

describe("ProjectActivityPanel", () => {
  it("paginates messages without letting an older page replace a refresh", async () => {
    let resolveOlderPage!: (value: any) => void;
    const olderPage = new Promise((resolve) => { resolveOlderPage = resolve; });
    let firstPageRequests = 0;
    const listCloudProjectMessages = vi.fn((input: { cursor?: string }) => {
      if (input.cursor === "page-2") return olderPage;
      firstPageRequests += 1;
      return Promise.resolve({
        projectScopeKey: "scope-a", projectId: "p-1", items: [{ activityId: firstPageRequests === 1 ? "first" : "refreshed", kind: "user_message", actorUserId: "user-1", authorDisplayName: "负责人", messageText: firstPageRequests === 1 ? "第一页留言" : "刷新后的留言", mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-10T11:00:00Z" }], nextCursor: firstPageRequests === 1 ? "page-2" : null, readCursor: null,
      });
    });
    const gateway = { listCloudProjectMessages } as unknown as ProjectGateway;
    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    expect(await screen.findByText("第一页留言")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "加载更多留言" }));
    await userEvent.click(screen.getByRole("button", { name: "刷新留言" }));
    expect(await screen.findByText("刷新后的留言")).toBeVisible();
    await act(async () => resolveOlderPage({
      projectScopeKey: "scope-a", projectId: "p-1", items: [{ activityId: "older", kind: "user_message", actorUserId: "user-3", authorDisplayName: "旧成员", messageText: "迟到的旧分页", mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-10T10:00:00Z" }], nextCursor: null, readCursor: null,
    }));

    expect(screen.queryByText("迟到的旧分页")).not.toBeInTheDocument();
    expect(screen.getByText("刷新后的留言")).toBeVisible();
  });

  it("marks displayed messages read with their isolated cursor on entry and refresh", async () => {
    let resolveFirstRead!: () => void;
    const firstRead = new Promise<void>((resolve) => { resolveFirstRead = resolve; });
    const markCloudProjectActivityRead = vi.fn();
    const markCloudProjectMessageRead = vi.fn().mockImplementationOnce(() => firstRead).mockResolvedValue(undefined);
    const listCloudProjectMessages = vi.fn().mockResolvedValue({
      projectScopeKey: "scope-a", projectId: "p-1", items: [{ activityId: "unread", kind: "user_message", actorUserId: "user-1", authorDisplayName: "负责人", messageText: "旧筛选未读动态", mentionUserIds: [], assets: [], unread: true, otherMembersUnread: true, unreadOtherMemberCount: 1, otherMemberReadStates: [{ userId: "user-2", displayName: "李四", status: "read" }, { userId: "user-3", displayName: "王五", status: "unread" }], createdAt: "2026-08-10T11:00:00Z" }], nextCursor: null, readCursor: "cursor-1",
    });
    const gateway = { listCloudProjectMessages, markCloudProjectActivityRead, markCloudProjectMessageRead } as unknown as ProjectGateway;
    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    const message = await screen.findByText("旧筛选未读动态");
    expect(message).toBeVisible();
    expect(message.closest("li")).toHaveAttribute("data-unread", "true");
    expect(screen.getByLabelText("未读")).toBeVisible();
    expect(screen.getByLabelText("其他成员阅读状态")).toHaveTextContent("李四 · 已读");
    expect(screen.getByLabelText("其他成员阅读状态")).toHaveTextContent("王五 · 未读");
    await waitFor(() => expect(markCloudProjectMessageRead).toHaveBeenCalledWith({ projectId: "p-1", throughCursor: "cursor-1" }));
    expect(markCloudProjectActivityRead).not.toHaveBeenCalled();

    await act(async () => resolveFirstRead());
    await waitFor(() => expect(screen.queryByLabelText("未读")).not.toBeInTheDocument());
    expect(message.closest("li")).not.toHaveAttribute("data-unread");

    await userEvent.click(screen.getByRole("button", { name: "刷新留言" }));
    await waitFor(() => expect(markCloudProjectMessageRead).toHaveBeenCalledTimes(2));
    expect(markCloudProjectMessageRead).toHaveBeenLastCalledWith({ projectId: "p-1", throughCursor: "cursor-1" });
    expect(markCloudProjectActivityRead).not.toHaveBeenCalled();
  });

  it("refreshes per-member read receipts on the author's own message", async () => {
    let requests = 0;
    const markCloudProjectMessageRead = vi.fn();
    const listCloudProjectMessages = vi.fn().mockImplementation(() => {
      requests += 1;
      const wangStatus = requests === 1 ? "unread" : "read";
      return Promise.resolve({
        projectScopeKey: "scope-a",
        projectId: "p-1",
        items: [{
          activityId: "own-message",
          kind: "user_message",
          actorUserId: "current-user",
          authorDisplayName: "我",
          messageText: "请确认方案",
          mentionUserIds: [],
          assets: [],
          unread: false,
          otherMembersUnread: wangStatus === "unread",
          unreadOtherMemberCount: wangStatus === "unread" ? 1 : 0,
          otherMemberReadStates: [
            { userId: "user-2", displayName: "李四", status: "read" },
            { userId: "user-3", displayName: "王五", status: wangStatus },
          ],
          createdAt: "2026-08-22T11:00:00Z",
        }],
        nextCursor: null,
        readCursor: "own-message-cursor",
      });
    });
    const gateway = { listCloudProjectMessages, markCloudProjectMessageRead } as unknown as ProjectGateway;
    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    expect(await screen.findByText("王五 · 未读")).toBeVisible();
    expect(screen.getByText("李四 · 已读")).toBeVisible();
    expect(markCloudProjectMessageRead).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "刷新留言" }));
    expect(await screen.findByText("王五 · 已读")).toBeVisible();
    expect(screen.queryByText("王五 · 未读")).not.toBeInTheDocument();
    expect(markCloudProjectMessageRead).not.toHaveBeenCalled();
  });

  it("keeps unread UI when the message read request fails", async () => {
    const gateway = {
      listCloudProjectMessages: vi.fn().mockResolvedValue({
        projectScopeKey: "scope-a", projectId: "p-1", items: [{ activityId: "unread", kind: "user_message", actorUserId: "user-1", authorDisplayName: "负责人", messageText: "仍需查看", mentionUserIds: [], assets: [], unread: true, createdAt: "2026-08-10T11:00:00Z" }], nextCursor: null, readCursor: "message-cursor",
      }),
      markCloudProjectMessageRead: vi.fn().mockRejectedValue(new Error("network unavailable")),
    } as unknown as ProjectGateway;
    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    const message = await screen.findByText("仍需查看");
    expect(await screen.findByText("留言已加载，但标记已读失败")).toBeVisible();
    expect(message.closest("li")).toHaveAttribute("data-unread", "true");
    expect(screen.getByLabelText("未读")).toBeVisible();
  });

  it("does not let a message readback from the previous project clear the current project", async () => {
    let resolvePreviousRead!: () => void;
    const previousRead = new Promise<void>((resolve) => { resolvePreviousRead = resolve; });
    const listCloudProjectMessages = vi.fn((input: { projectId: string }) => Promise.resolve({
      projectScopeKey: input.projectId === "p-1" ? "scope-a" : "scope-b",
      projectId: input.projectId,
      items: [{ activityId: `${input.projectId}-unread`, kind: "user_message", actorUserId: "user-1", authorDisplayName: "负责人", messageText: input.projectId === "p-1" ? "第一个项目留言" : "第二个项目未读留言", mentionUserIds: [], assets: [], unread: true, createdAt: "2026-08-10T11:00:00Z" }],
      nextCursor: null,
      readCursor: input.projectId === "p-1" ? "p1-message-cursor" : null,
    }));
    const markCloudProjectMessageRead = vi.fn().mockReturnValue(previousRead);
    const gateway = { listCloudProjectMessages, markCloudProjectMessageRead } as unknown as ProjectGateway;
    const view = render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    await waitFor(() => expect(markCloudProjectMessageRead).toHaveBeenCalledWith({ projectId: "p-1", throughCursor: "p1-message-cursor" }));
    view.rerender(<ProjectActivityPanel projectId="p-2" projectScopeKey="scope-b" gateway={gateway} members={[]} readOnly={false} />);
    const currentMessage = await screen.findByText("第二个项目未读留言");
    expect(currentMessage.closest("li")).toHaveAttribute("data-unread", "true");

    await act(async () => resolvePreviousRead());
    expect(currentMessage.closest("li")).toHaveAttribute("data-unread", "true");
    expect(screen.getByLabelText("未读")).toBeVisible();
  });

  it("ignores a posted message that resolves after switching projects", async () => {
    let resolvePost!: (value: any) => void;
    const delayedPost = new Promise((resolve) => { resolvePost = resolve; });
    const listCloudProjectMessages = vi.fn((input: { projectId: string }) => Promise.resolve({
      projectScopeKey: input.projectId === "p-1" ? "scope-a" : "scope-b",
      projectId: input.projectId,
      items: input.projectId === "p-1" ? [] : [{ activityId: "p2-current", kind: "user_message", actorUserId: "user-2", authorDisplayName: "产品同学", messageText: "第二个项目动态", mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-10T12:00:00Z" }],
      nextCursor: null,
      readCursor: null,
    }));
    const gateway = {
      listCloudProjectMessages,
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-a", projectId: "p-1", items: [] }),
      postCloudProjectMessage: vi.fn().mockReturnValue(delayedPost),
    } as unknown as ProjectGateway;
    const view = render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    await userEvent.click(screen.getByRole("button", { name: "发布留言" }));
    await userEvent.type(screen.getByRole("textbox", { name: "发布项目留言" }), "第一个项目留言");
    await userEvent.click(screen.getByRole("button", { name: "确认发布" }));
    view.rerender(<ProjectActivityPanel projectId="p-2" projectScopeKey="scope-b" gateway={gateway} members={[]} readOnly={false} />);
    expect(await screen.findByText("第二个项目动态")).toBeVisible();
    await act(async () => resolvePost({
      activityId: "late-post", projectId: "p-1", kind: "user_message", eventName: null, messageText: "迟到的第一个项目留言", authorUserId: "user-1", authorDisplayName: "负责人", mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-10T13:00:00Z",
    }));

    expect(screen.queryByText("迟到的第一个项目留言")).not.toBeInTheDocument();
    expect(screen.getByText("第二个项目动态")).toBeVisible();
  });

  it("publishes a text-only message with structured mentions", async () => {
    const existingCreatedAt = new Date(2026, 7, 10, 11, 22, 33).toISOString();
    const listCloudProjectMessages = vi.fn().mockResolvedValue({
      projectScopeKey: "scope-a",
      projectId: "p-1",
      items: [{ activityId: "message-existing", kind: "user_message", actorUserId: "user-2", authorDisplayName: "产品同学", messageText: "已有留言", mentionUserIds: [], assets: [], unread: false, createdAt: existingCreatedAt }],
      nextCursor: null,
      readCursor: null,
    });
    const postCloudProjectMessage = vi.fn().mockResolvedValue({
      activityId: "activity-1",
      projectId: "p-1",
      kind: "user_message",
      eventName: null,
      messageText: "交付风险已更新",
      authorUserId: "user-1",
      authorDisplayName: "项目负责人",
      mentionUserIds: ["user-2"],
      assets: [],
      unread: false,
      createdAt: "2026-08-10T12:00:00Z",
    });
    const gateway = { listCloudProjectMessages, postCloudProjectMessage } as unknown as ProjectGateway;

    render(<ProjectActivityPanel
      projectId="p-1"
      projectScopeKey="scope-a"
      gateway={gateway}
      members={[{ userId: "user-2", displayName: "产品同学", role: "member", status: "active", version: 1 }]}
      readOnly={false}
    />);

    const publish = screen.getByRole("button", { name: "发布留言" });
    expect(screen.queryByRole("textbox", { name: "发布项目留言" })).not.toBeInTheDocument();
    await waitFor(() => expect(listCloudProjectMessages).toHaveBeenLastCalledWith({ projectId: "p-1", pageSize: 20 }));
    expect(screen.queryByRole("button", { name: "与我相关" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "成员动态" })).not.toBeInTheDocument();
    const activityLine = await screen.findByText("已有留言");
    expect(activityLine).toBeVisible();
    const activityRow = activityLine.closest("li");
    expect(activityRow?.querySelector(".project-activity__actor")).toHaveTextContent("产品同学");
    expect(activityRow?.querySelector(".project-activity__action")).toHaveTextContent("已有留言");
    expect(activityRow?.querySelector("time")).toHaveTextContent("留言时间：2026-08-10 11:22:33");

    await userEvent.click(publish);
    const message = screen.getByRole("textbox", { name: "发布项目留言" });
    expect(message).toHaveFocus();
    await userEvent.type(message, "交付风险已更新 @产");
    const mentionSuggestions = screen.getByRole("listbox", { name: "提及成员建议" });
    expect(mentionSuggestions).toBeVisible();
    expect(screen.getByRole("option", { name: "@产品同学" })).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{Enter}");
    expect(message).toHaveValue("交付风险已更新 @产品同学 ");
    expect(screen.queryByRole("button", { name: "添加附件" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "确认发布" }));
    expect(postCloudProjectMessage).toHaveBeenCalledWith({ projectId: "p-1", text: "交付风险已更新 @产品同学 ", mentionUserIds: ["user-2"], assetIds: [] });
    await waitFor(() => expect(screen.queryByRole("textbox", { name: "发布项目留言" })).not.toBeInTheDocument());
    expect(screen.queryByText("留言已完成权威回读")).not.toBeInTheDocument();
    await userEvent.click(publish);
    const nextMessage = screen.getByRole("textbox", { name: "发布项目留言" });
    expect(nextMessage).toHaveValue("");
    await userEvent.type(nextMessage, "下一条留言");
    await userEvent.click(screen.getByRole("button", { name: "确认发布" }));
    expect(postCloudProjectMessage).toHaveBeenLastCalledWith({ projectId: "p-1", text: "下一条留言", mentionUserIds: [], assetIds: [] });
  });

  it("supports keyboard mention navigation and preserves cancelled text without duplicate mentions", async () => {
    const listCloudProjectMessages = vi.fn().mockResolvedValue({
      projectScopeKey: "scope-a",
      projectId: "p-1",
      items: [],
      nextCursor: null,
      readCursor: null,
    });
    const postCloudProjectMessage = vi.fn().mockResolvedValue({
      activityId: "activity-2",
      projectId: "p-1",
      kind: "user_message",
      eventName: null,
      messageText: "请确认 @设计同学 @设计同学 ",
      authorUserId: "user-1",
      authorDisplayName: "项目负责人",
      mentionUserIds: ["user-3"],
      assets: [],
      unread: false,
      createdAt: "2026-08-10T12:00:00Z",
    });
    const gateway = { listCloudProjectMessages, postCloudProjectMessage } as unknown as ProjectGateway;

    render(<ProjectActivityPanel
      projectId="p-1"
      projectScopeKey="scope-a"
      gateway={gateway}
      members={[
        { userId: "user-2", displayName: "产品同学", role: "member", status: "active", version: 1 },
        { userId: "user-3", displayName: "设计同学", role: "member", status: "active", version: 1 },
      ]}
      readOnly={false}
    />);

    await userEvent.click(screen.getByRole("button", { name: "发布留言" }));
    const message = screen.getByRole("textbox", { name: "发布项目留言" });
    await userEvent.type(message, "请确认 @");
    expect(screen.getByRole("option", { name: "@产品同学" })).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowDown}");
    expect(screen.getByRole("option", { name: "@设计同学" })).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowUp}");
    expect(screen.getByRole("option", { name: "@产品同学" })).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("listbox", { name: "提及成员建议" })).not.toBeInTheDocument();

    await userEvent.clear(message);
    await userEvent.type(message, "请确认 @设{Enter}@设{Enter}");
    expect(message).toHaveValue("请确认 @设计同学 @设计同学 ");

    await userEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(screen.queryByRole("textbox", { name: "发布项目留言" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "发布留言" }));
    expect(screen.getByRole("textbox", { name: "发布项目留言" })).toHaveValue("请确认 @设计同学 @设计同学 ");
    await userEvent.click(screen.getByRole("button", { name: "确认发布" }));
    expect(postCloudProjectMessage).toHaveBeenCalledWith({
      projectId: "p-1",
      text: "请确认 @设计同学 @设计同学 ",
      mentionUserIds: ["user-3"],
      assetIds: [],
    });
  });

  it("removes a structured mention id when its inserted text is deleted", async () => {
    const postCloudProjectMessage = vi.fn().mockResolvedValue({
      activityId: "activity-3", projectId: "p-1", kind: "user_message", eventName: null, messageText: "请确认", authorUserId: "user-1", authorDisplayName: "负责人", mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-10T12:00:00Z",
    });
    const gateway = {
      listCloudProjectMessages: vi.fn().mockResolvedValue({ projectScopeKey: "scope-a", projectId: "p-1", items: [], nextCursor: null, readCursor: null }),
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "scope-a", projectId: "p-1", items: [] }),
      postCloudProjectMessage,
    } as unknown as ProjectGateway;
    render(<ProjectActivityPanel
      projectId="p-1"
      projectScopeKey="scope-a"
      gateway={gateway}
      members={[{ userId: "user-3", displayName: "设计同学", role: "member", status: "active", version: 1 }]}
      readOnly={false}
    />);

    await userEvent.click(screen.getByRole("button", { name: "发布留言" }));
    const message = screen.getByRole("textbox", { name: "发布项目留言" });
    await userEvent.type(message, "请确认 @设{Enter}");
    expect(message).toHaveValue("请确认 @设计同学 ");
    fireEvent.change(message, { target: { value: "请确认 ", selectionStart: 4 } });
    await userEvent.click(screen.getByRole("button", { name: "确认发布" }));

    expect(postCloudProjectMessage).toHaveBeenCalledWith({ projectId: "p-1", text: "请确认 ", mentionUserIds: [], assetIds: [] });
  });

  it("rejects a message that exceeds the backend UTF-8 byte limit", async () => {
    const postCloudProjectMessage = vi.fn();
    const gateway = {
      listCloudProjectMessages: vi.fn().mockResolvedValue({ projectScopeKey: "scope-a", projectId: "p-1", items: [], nextCursor: null, readCursor: null }),
      postCloudProjectMessage,
    } as unknown as ProjectGateway;
    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    await userEvent.click(screen.getByRole("button", { name: "发布留言" }));
    fireEvent.change(screen.getByRole("textbox", { name: "发布项目留言" }), { target: { value: "中".repeat(6_827) } });
    await userEvent.click(screen.getByRole("button", { name: "确认发布" }));

    expect(await screen.findByText("留言不能超过 20,480 个 UTF-8 字节。")).toBeVisible();
    expect(postCloudProjectMessage).not.toHaveBeenCalled();
  });

  it("keeps activity read-only when the project is archived", async () => {
    const gateway = {
      listCloudProjectMessages: vi.fn().mockResolvedValue({ projectScopeKey: "scope-a", projectId: "p-1", items: [], nextCursor: null, readCursor: null }),
    } as unknown as ProjectGateway;

    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly />);

    expect(screen.getByRole("button", { name: "发布留言" })).toBeDisabled();
    expect(screen.queryByRole("textbox", { name: "发布项目留言" })).not.toBeInTheDocument();
  });

  it("opens a published activity attachment and reports download failures", async () => {
    const downloadCloudProjectAsset = vi.fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("download unavailable"));
    const gateway = {
      listCloudProjectMessages: vi.fn().mockResolvedValue({
        projectScopeKey: "scope-a",
        projectId: "p-1",
        items: [{
          activityId: "message-with-assets",
          kind: "user_message",
          actorUserId: "user-1",
          authorDisplayName: "负责人",
          messageText: "请查看附件",
          mentionUserIds: [],
          assets: [
            { assetId: "asset-1", filename: "验收记录.md" },
            { assetId: "asset-2", filename: "风险清单.pdf" },
          ],
          unread: false,
          createdAt: "2026-08-10T12:00:00Z",
        }],
        nextCursor: null,
        readCursor: null,
      }),
      downloadCloudProjectAsset,
    } as unknown as ProjectGateway;

    render(<ProjectActivityPanel projectId="p-1" projectScopeKey="scope-a" gateway={gateway} members={[]} readOnly={false} />);

    await userEvent.click(await screen.findByRole("button", { name: "打开附件 验收记录.md" }));
    expect(downloadCloudProjectAsset).toHaveBeenCalledWith("p-1", "asset-1");
    await userEvent.click(screen.getByRole("button", { name: "打开附件 风险清单.pdf" }));
    expect(await screen.findByText("附件打开失败，请稍后重试。")).toBeVisible();
  });
});
