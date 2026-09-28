import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import { CloudProjectsSection } from "../features/projects/ui/CloudProjectsSection";
import { LocaleProvider } from "../lib/i18n";

describe("WorkBuddy Projects v2 collaboration surface", () => {
  it("keeps project messages and the shared Plan on distinct project gateways", async () => {
    const listWorkItems = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", projectId: "p-v2", items: [], nextCursor: null });
    const listLabels = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", projectId: "p-v2", items: [{ labelId: "lbl-1", name: "Release", color: "#3366FF", version: 1 }] });
    const listMessages = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", projectId: "p-v2", items: [
      { activityId: "a-1", kind: "user_message", actorUserId: "owner-1", authorDisplayName: "Owner", messageText: "请确认本周计划", mentionUserIds: [], assets: [], unread: true, createdAt: "2026-08-07T08:00:00Z" },
      { activityId: "a-2", kind: "user_message", actorUserId: "member-1", authorDisplayName: "Member", messageText: "已经确认", mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-07T07:00:00Z" },
    ], nextCursor: null, readCursor: "read-cursor-1" });
    const gateway = {
      listCloudProjects: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", items: [{ projectId: "p-v2", name: "V2 Project", description: null, status: "active", version: 1, createdAt: "2026-08-07T08:00:00Z", updatedAt: "2026-08-07T08:00:00Z" }], nextCursor: null, stale: false, cachedAt: null }),
      getCloudProjectFeatures: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", instructionsAvailable: true, workItemsAvailable: true, membersAvailable: true, activityAvailable: true, workItemFieldsAvailable: true }),
      getCloudProject: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", project: { projectId: "p-v2", name: "V2 Project", description: null, status: "active", version: 1, createdAt: "2026-08-07T08:00:00Z", updatedAt: "2026-08-07T08:00:00Z" }, stale: false, cachedAt: null }),
      getCloudProjectInstruction: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", projectId: "p-v2", humanDescription: "Summary", agentInstruction: "Instruction", contentHash: "a".repeat(64), version: 1, createdAt: "2026-08-07T08:00:00Z", updatedAt: "2026-08-07T08:00:00Z" }),
      listCloudProjectMembers: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", projectId: "p-v2", currentRole: "owner", items: [{ userId: "owner-1", displayName: "Owner", role: "owner", status: "active", version: 1 }, { userId: "member-1", displayName: "Member", role: "member", status: "active", version: 1 }] }),
      listCloudProjectInvites: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-v2", projectId: "p-v2", items: [] }),
      listCloudProjectMessages: listMessages,
      postCloudProjectMessage: vi.fn(),
      listCloudProjectWorkItems: listWorkItems,
      listCloudProjectLabels: listLabels,
      createCloudProjectLabel: vi.fn(),
    } as unknown as ProjectGateway;

    render(<LocaleProvider><CloudProjectsSection authenticated accountScopeKey="account-v2" gateway={gateway} /></LocaleProvider>);
    await userEvent.click(await screen.findByRole("button", { name: /V2 Project/ }));
    expect(await screen.findByRole("heading", { name: "项目动态" })).toBeVisible();
    expect(listMessages).toHaveBeenCalledWith({ projectId: "p-v2", pageSize: 20 });
    expect(screen.getByText("请确认本周计划")).toBeVisible();
    expect(screen.getByText("已经确认")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "邀请" }));
    const drawer = await screen.findByRole("dialog", { name: "成员与邀请" });
    expect(drawer).toHaveAttribute("data-owner-relation", "owner");
    expect(within(drawer).getByRole("heading", { name: "当前成员" })).toBeVisible();
    expect(within(drawer).getByText("项目当前的所有者与活跃成员。")).toBeVisible();
    expect(within(drawer).getByText("Owner")).toBeVisible();
    expect(within(drawer).getByText("Member")).toBeVisible();
    expect(within(drawer).getAllByRole("listitem")).toHaveLength(2);
    expect(gateway.listCloudProjectMembers).toHaveBeenCalledWith("p-v2");
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    await userEvent.click(within(drawer).getByRole("button", { name: "复制项目 ID" }));
    expect(await within(drawer).findByRole("status")).toHaveTextContent("项目 ID 已复制。");
    expect(writeText).toHaveBeenCalledWith("p-v2");
    await userEvent.click(within(drawer).getByRole("button", { name: "关闭" }));

    await userEvent.click(screen.getByRole("button", { name: "事项" }));
    await waitFor(() => expect(listWorkItems).toHaveBeenCalledTimes(4));
    await waitFor(() => expect(listLabels).toHaveBeenCalledWith("p-v2"));
    await userEvent.click(screen.getByRole("button", { name: "标签设置" }));
    expect(screen.getByPlaceholderText("新标签名称")).toBeVisible();
    await userEvent.click(within(screen.getByRole("dialog", { name: "标签设置" })).getByRole("button", { name: "完成" }));
    expect(screen.getByRole("button", { name: "表格" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "看板" })).toBeEnabled();
    for (const name of ["列表", "甘特", "日历"]) expect(screen.getByRole("button", { name })).toBeEnabled();
    expect(listWorkItems).toHaveBeenCalledTimes(4);
  });
});
