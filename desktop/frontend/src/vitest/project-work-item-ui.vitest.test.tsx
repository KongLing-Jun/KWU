import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "../lib/i18n";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import type { CloudProjectWorkItem, CloudProjectWorkItemStatus } from "../features/projects/model/projectTypes";
import { WorkItemBoard } from "../features/projects/ui/WorkItemBoard";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((next, fail) => { resolve = next; reject = fail; });
  return { promise, resolve, reject };
}

function item(workItemId: string, title: string, status: CloudProjectWorkItemStatus, version = 1): CloudProjectWorkItem {
  return {
    workItemId, projectId: "p-1", title, description: `${title} description`, status, rank: 100,
    version, createdAt: "2026-08-05T04:00:00Z", updatedAt: "2026-08-10T04:00:00Z",
    assigneeUserId: null, assigneeStatus: null, assigneeDisplayName: null, priority: "none",
    startAt: null, dueAt: null, labels: [],
  };
}

function gatewayWith(items: CloudProjectWorkItem[], overrides: Partial<ProjectGateway> = {}) {
  const list = vi.fn(async (input: { status?: string; cursor?: string; scheduledFrom?: string; scheduledTo?: string }) => {
    const matching = input.status === "all" ? items : items.filter((entry) => entry.status === input.status);
    const pageItems = input.scheduledFrom ? matching : input.cursor ? matching.slice(1) : matching.slice(0, 1);
    return { projectScopeKey: "opaque-a", projectId: "p-1", items: pageItems, nextCursor: input.scheduledFrom ? null : matching.length > 1 && !input.cursor ? `${input.status}-next` : null, totalCount: matching.length, undatedCount: input.scheduledFrom ? 3 : 0 };
  });
  return {
    listCloudProjectWorkItems: list,
    listCloudProjectMembers: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", currentRole: "owner", items: [{ userId: "usr-1", displayName: "成员甲", role: "member", status: "active", version: 1 }] }),
    listCloudProjectLabels: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [] }),
    listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [] }),
    listCloudProjectActivities: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [], nextCursor: null, readCursor: null }),
    downloadCloudProjectAsset: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", assetId: "asset-1", filename: "附件.txt", url: "", expiresAt: "", savedPath: "C:\\workspace\\附件.txt" }),
    ...overrides,
  } as unknown as ProjectGateway & { listCloudProjectWorkItems: typeof list };
}

function renderBoard(gateway: ProjectGateway, archived = false) {
  return render(<LocaleProvider><WorkItemBoard accountScopeKey="owner-a" projectScopeKey="opaque-a" projectId="p-1" projectName="Alpha" archived={archived} stale={false} gateway={gateway} /></LocaleProvider>);
}

describe("WorkBuddy cloud plan workspace", () => {
  it("localizes every priority filter option", async () => {
    renderBoard(gatewayWith([]));
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "筛选事项" }));
    await userEvent.click(screen.getByRole("button", { name: "优先级：全部优先级" }));
    for (const label of ["全部优先级", "无优先级", "低", "中", "高", "紧急"]) {
      expect(screen.getByRole("option", { name: label })).toBeVisible();
    }
    for (const internalValue of ["low", "medium", "high", "urgent"]) {
      expect(screen.queryByRole("option", { name: internalValue })).toBeNull();
    }
  });

  it("closes the filter menu when clicking outside", async () => {
    renderBoard(gatewayWith([]));
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "筛选事项" }));
    expect(screen.getByRole("dialog", { name: "筛选事项" })).toBeVisible();

    await userEvent.click(screen.getByRole("region", { name: "Alpha 的事项" }));

    expect(screen.queryByRole("dialog", { name: "筛选事项" })).toBeNull();
    expect(screen.getByRole("button", { name: "筛选事项" })).toHaveAttribute("aria-expanded", "false");
  });

  it("localizes priority values on cards and in item details", async () => {
    const urgent = { ...item("todo-urgent", "紧急事项", "todo"), priority: "urgent" as const };
    renderBoard(gatewayWith([urgent]));
    const title = await screen.findByText("紧急事项");
    const card = title.closest(".work-item-board__card")!;
    expect(card).toHaveTextContent("紧急");
    expect(card).not.toHaveTextContent("urgent");
    await userEvent.click(title.closest("button")!);
    const detail = await screen.findByRole("dialog", { name: "事项详情" });
    expect(within(detail).getByText("优先级").parentElement).toHaveTextContent("紧急");
    expect(detail).not.toHaveTextContent("urgent");
  });

  it("loads four status columns independently and shows cloud total counts", async () => {
    const gateway = gatewayWith([
      item("todo-1", "待开始一", "todo"), item("todo-2", "待开始二", "todo"),
      item("doing-1", "正在进行", "in_progress"), item("done-1", "已经完成", "done"),
    ]);
    renderBoard(gateway);
    expect(await screen.findByText("待开始一")).toBeVisible();
    expect(screen.getByText("待开始一").closest(".work-item-board__card")).toHaveTextContent("待开始");
    expect(screen.getByRole("heading", { name: "待开始" }).parentElement).toHaveTextContent("2");
    expect(screen.getByRole("heading", { name: "进行中" }).parentElement).toHaveTextContent("1");
    expect(screen.getByRole("heading", { name: "已暂停" }).parentElement).toHaveTextContent("0");
    expect(screen.getByRole("heading", { name: "已完成" }).parentElement).toHaveTextContent("1");
    expect(screen.getByRole("heading", { name: "待开始" }).parentElement?.querySelector("svg")).toHaveClass("lucide-circle");
    expect(screen.getByRole("heading", { name: "进行中" }).parentElement?.querySelector("svg")).toHaveClass("lucide-circle-play");
    expect(screen.getByRole("heading", { name: "已暂停" }).parentElement?.querySelector("svg")).toHaveClass("lucide-circle-pause");
    expect(screen.getByRole("heading", { name: "已完成" }).parentElement?.querySelector("svg")).toHaveClass("lucide-circle-check");
    expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(4);
    expect(gateway.listCloudProjectWorkItems.mock.calls.map(([input]) => input.status)).toEqual(["todo", "in_progress", "paused", "done"]);
    await userEvent.click(screen.getByRole("button", { name: "加载更多（1/2）" }));
    expect(await screen.findByText("待开始二")).toBeVisible();
    expect(gateway.listCloudProjectWorkItems).toHaveBeenLastCalledWith(expect.objectContaining({ status: "todo", cursor: "todo-next" }));
  });

  it("waits for blur or Enter before cloud search and sends every filter", async () => {
    const gateway = gatewayWith([item("todo-1", "Alpha", "todo")]);
    renderBoard(gateway);
    await screen.findByText("Alpha");
    await userEvent.click(screen.getByRole("button", { name: "搜索事项" }));
    const search = screen.getByRole("searchbox", { name: "搜索事项" });
    await userEvent.type(search, "release");
    expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(4);
    fireEvent.compositionStart(search);
    fireEvent.keyDown(search, { key: "Enter", isComposing: true });
    expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(4);
    fireEvent.compositionEnd(search);
    fireEvent.blur(search);
    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(8));
    expect(gateway.listCloudProjectWorkItems).toHaveBeenLastCalledWith(expect.objectContaining({ search: "release" }));
    await userEvent.click(screen.getByRole("button", { name: "筛选事项" }));
    await userEvent.click(within(screen.getByRole("dialog", { name: "筛选事项" })).getByRole("button", { name: "负责人：全部负责人" }));
    await userEvent.click(screen.getByRole("option", { name: "成员甲" }));
    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(12));
    expect(gateway.listCloudProjectWorkItems).toHaveBeenLastCalledWith(expect.objectContaining({ search: "release", assigneeUserId: "usr-1" }));
  });

  it("exposes only the six built-in plan views and keeps each view selectable", async () => {
    const gateway = gatewayWith([item("todo-1", "Alpha", "todo")]);
    const view = renderBoard(gateway);
    await screen.findByText("Alpha");
    const toolbar = screen.getByRole("toolbar", { name: "事项视图" });
    expect(within(toolbar).getAllByRole("button").map((button) => button.textContent)).toEqual(["全部", "看板", "表格", "列表", "甘特", "日历"]);
    expect(screen.queryByRole("button", { name: "保存当前自定义视图" })).toBeNull();
    expect(screen.queryByRole("dialog", { name: /(?:保存|重命名)自定义视图/ })).toBeNull();

    const projections = [
      ["看板", "[data-plan-board]"],
      ["表格", "[data-plan-table]"],
      ["列表", "[data-plan-list]"],
      ["甘特", "[data-plan-gantt]"],
      ["日历", "[data-plan-calendar]"],
    ] as const;
    for (const [name, selector] of projections) {
      await userEvent.click(within(toolbar).getByRole("button", { name }));
      expect(within(toolbar).getByRole("button", { name })).toHaveAttribute("aria-pressed", "true");
      expect(view.container.querySelector(selector)).not.toBeNull();
    }
    await userEvent.click(within(toolbar).getByRole("button", { name: "全部" }));
    expect(within(toolbar).getByRole("button", { name: "全部" })).toHaveAttribute("aria-pressed", "true");
  });

  it("keeps five projections and loads the calendar from the authoritative month range", async () => {
    const scheduled = { ...item("todo-1", "排期事项", "todo"), startAt: "2026-08-22T00:00:00Z", dueAt: "2026-08-23T23:59:59Z" };
    const scheduledSecond = { ...item("doing-1", "联调事项", "in_progress"), startAt: "2026-08-14T00:00:00Z", dueAt: "2026-08-18T23:59:59Z" };
    const gateway = gatewayWith([scheduled, scheduledSecond]);
    const view = renderBoard(gateway);
    await screen.findByText("排期事项");
    expect(view.container.querySelector("[data-plan-board]")).not.toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "表格" }));
    expect(view.container.querySelector("[data-plan-table]")).not.toBeNull();
    expect(screen.getByRole("table")).toHaveTextContent("排期事项");
    expect(screen.getByRole("table")).toHaveTextContent("创建时间");
    await userEvent.click(screen.getByRole("checkbox", { name: "选择 排期事项" }));
    expect(screen.getByRole("checkbox", { name: "选择 排期事项" })).toBeChecked();
    await userEvent.click(screen.getByRole("button", { name: "列表" }));
    expect(view.container.querySelector("[data-plan-list]")).toHaveTextContent("待开始1排期事项");
    expect(screen.getByRole("button", { name: "在进行中中新建事项" })).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "甘特" }));
    expect(view.container.querySelector("[data-plan-gantt]")).toHaveTextContent("排期事项");
    const bars = view.container.querySelectorAll<HTMLElement>(".work-item-board__gantt-bar");
    expect(bars).toHaveLength(2);
    expect(bars[0]).toHaveTextContent("排期事项");
    expect(bars[1]).toHaveTextContent("联调事项");
    expect(bars[0]).toHaveClass("work-item-board__gantt-bar--todo");
    expect(bars[0]).toHaveAttribute("data-range-start", "2026-08-22");
    expect(bars[0]).toHaveAttribute("data-range-end", "2026-08-23");
    expect(bars[0].style.width).toBe("96px");
    expect(bars[0].dataset.color).toBeUndefined();
    expect(bars[0].querySelector("svg")).toHaveClass("lucide-circle");
    expect(bars[1]).toHaveClass("work-item-board__gantt-bar--in_progress");
    expect(bars[1].querySelector("svg")).toHaveClass("lucide-circle-play");
    expect(screen.getByLabelText("甘特图横向滚动条")).toBeVisible();
    const scales = screen.getByRole("group", { name: "甘特图时间刻度" });
    for (const label of ["周", "月", "季", "年"]) expect(within(scales).getByRole("button", { name: label })).toBeVisible();
    await userEvent.click(within(scales).getByRole("button", { name: "年" }));
    expect(within(scales).getByRole("button", { name: "年" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "今天" }));
    await userEvent.click(screen.getByRole("button", { name: "日历" }));
    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(7));
    expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({ status: "all", scheduledFrom: "2026-07-27", scheduledTo: "2026-09-06" }));
    const calendarRange = screen.getByRole("button", { name: "查看 排期事项，2026-08-22至2026-08-23" });
    expect(calendarRange).toHaveClass("work-item-board__calendar-range--todo");
    expect(view.container.querySelector("[data-plan-calendar]")).not.toHaveTextContent(/(?:开始|截止) · 排期事项/);
    expect(view.container.querySelector("[data-plan-calendar]")).toHaveTextContent("无日期记录：3");
    expect(view.container.querySelectorAll("[data-calendar-month]")).toHaveLength(3);
    expect(view.container.querySelectorAll(".work-item-board__calendar-day")).toHaveLength(126);
  });

  it("does not add a redundant empty-state message to any plan projection", async () => {
    renderBoard(gatewayWith([]));
    await screen.findAllByText("暂无事项");
    expect(screen.queryByText("没有符合当前条件的事项")).toBeNull();

    for (const view of ["表格", "列表", "甘特", "日历"]) {
      await userEvent.click(screen.getByRole("button", { name: view }));
      expect(screen.queryByText("没有符合当前条件的事项")).toBeNull();
    }
  });

  it("opens a read-only detail with attached assets and item activity before editing", async () => {
    const workItem = item("todo-1", "带附件事项", "todo");
    const download = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", assetId: "asset-1", filename: "验收说明.md", url: "", expiresAt: "", savedPath: "C:\\workspace\\验收说明.md" });
    const gateway = gatewayWith([workItem], {
      listCloudProjectAssets: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [{ assetId: "asset-1", projectId: "p-1", filename: "验收说明.md", contentType: "text/markdown", sizeBytes: 12, sha256: "a".repeat(64), version: 1, workItemIds: ["todo-1"], createdAt: "2026-08-05T04:00:00Z", updatedAt: "2026-08-05T04:00:00Z" }] }),
      listCloudProjectActivities: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [{ activityId: "act-1", kind: "system_event", actorUserId: "usr-1", authorDisplayName: "成员甲", eventName: "work_item.created", eventPayload: { work_item_id: "todo-1" }, mentionUserIds: [], assets: [], unread: false, createdAt: "2026-08-05T04:00:00Z" }], nextCursor: null, readCursor: null }),
      downloadCloudProjectAsset: download,
    });
    renderBoard(gateway);
    const title = await screen.findByText("带附件事项");
    await userEvent.click(title.closest("button")!);
    const detail = await screen.findByRole("dialog", { name: "事项详情" });
    expect(within(detail).getByText("验收说明.md")).toBeVisible();
    expect(within(detail).getByText(/创建了事项/)).toBeVisible();
    expect(screen.queryByRole("dialog", { name: "编辑待办" })).toBeNull();
    await userEvent.click(within(detail).getByRole("button", { name: "验收说明.md" }));
    expect(download).toHaveBeenCalledWith("p-1", "asset-1");
    await userEvent.click(within(detail).getByRole("button", { name: "编辑" }));
    expect(await screen.findByRole("dialog", { name: "编辑待办" })).toBeVisible();
  });

  it("shows an attachment in detail after creating and binding the work item", async () => {
    const created = item("new-1", "新建附件事项", "todo");
    const asset = { assetId: "asset-1", projectId: "p-1", filename: "交付清单.md", contentType: "text/markdown", sizeBytes: 12, sha256: "a".repeat(64), version: 1, workItemIds: ["new-1"], createdAt: "2026-08-05T04:00:00Z", updatedAt: "2026-08-05T04:00:00Z" };
    let createdVisible = false;
    const listWorkItems = vi.fn(async (input: { status?: string }) => ({ projectScopeKey: "opaque-a", projectId: "p-1", items: createdVisible && input.status === "todo" ? [created] : [], nextCursor: null, totalCount: createdVisible && input.status === "todo" ? 1 : 0, undatedCount: 0 }));
    const listAssets = vi.fn()
      .mockResolvedValueOnce({ projectScopeKey: "opaque-a", projectId: "p-1", items: [] })
      .mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [asset] });
    const create = vi.fn(async () => { createdVisible = true; return { projectScopeKey: "opaque-a", projectId: "p-1", workItem: created }; });
    const bind = vi.fn().mockResolvedValue(asset);
    const gateway = gatewayWith([], {
      listCloudProjectWorkItems: listWorkItems as ProjectGateway["listCloudProjectWorkItems"],
      createCloudProjectWorkItem: create,
      putCloudProjectWorkItemAsset: bind,
      deleteCloudProjectWorkItemAsset: vi.fn(),
      listCloudProjectAssets: listAssets,
      listCloudProjectAssetDirectory: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", folderId: null, breadcrumbs: [], items: [{ kind: "asset", id: "asset-1", name: "交付清单.md", contentType: "text/markdown", sizeBytes: 12, updatedAt: "2026-08-05T04:00:00Z" }], nextCursor: null }),
    });
    renderBoard(gateway);
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "添加" }));
    const createDialog = screen.getByRole("dialog", { name: "新建待办" });
    await userEvent.type(within(createDialog).getByLabelText("标题"), "新建附件事项");
    await userEvent.click(within(createDialog).getByRole("button", { name: "添加附件" }));
    await userEvent.click(await screen.findByRole("checkbox", { name: "交付清单.md" }));
    await userEvent.click(within(createDialog).getByRole("button", { name: "创建" }));
    await waitFor(() => expect(bind).toHaveBeenCalledWith({ projectId: "p-1", workItemId: "new-1", assetId: "asset-1" }));
    const title = await screen.findByText("新建附件事项");
    await userEvent.click(title.closest("button")!);
    expect(await screen.findByRole("dialog", { name: "事项详情" })).toHaveTextContent("交付清单.md");
  });

  it("shows the attachment tooltip and gives undated items a visible gantt entry", async () => {
    const gateway = gatewayWith([item("todo-1", "未排期事项", "todo")]);
    const view = renderBoard(gateway);
    await screen.findByText("未排期事项");
    await userEvent.click(screen.getByRole("button", { name: "添加" }));
    const dialog = screen.getByRole("dialog", { name: "新建待办" });
    fireEvent.focus(within(dialog).getByRole("button", { name: "添加附件" }));
    expect(await screen.findByRole("tooltip")).toHaveTextContent("添加附件");
    await userEvent.click(within(dialog).getByRole("button", { name: "取消" }));
    await userEvent.click(screen.getByRole("button", { name: "甘特" }));
    expect(screen.getByRole("button", { name: "查看未排期事项 未排期事项" })).toBeVisible();
    const scroller = screen.getByLabelText("甘特图时间轴");
    const horizontalScrollbar = screen.getByLabelText("甘特图横向滚动条");
    expect(scroller.scrollLeft).toBeGreaterThan(0);
    expect(horizontalScrollbar.scrollLeft).toBe(scroller.scrollLeft);
    Object.defineProperty(scroller, "clientWidth", { configurable: true, value: 400 });
    Object.defineProperty(scroller, "scrollLeft", { configurable: true, writable: true, value: 0 });
    Object.defineProperty(scroller, "scrollTo", { configurable: true, value: vi.fn() });
    Object.defineProperty(horizontalScrollbar, "scrollLeft", { configurable: true, writable: true, value: 0 });
    await userEvent.click(screen.getByRole("button", { name: "今天" }));
    await waitFor(() => expect(scroller.scrollLeft).toBeGreaterThan(0));
    expect(horizontalScrollbar.scrollLeft).toBe(scroller.scrollLeft);
    scroller.scrollLeft = 240;
    fireEvent.scroll(scroller);
    expect(horizontalScrollbar.scrollLeft).toBe(240);
    horizontalScrollbar.scrollLeft = 360;
    fireEvent.scroll(horizontalScrollbar);
    expect(scroller.scrollLeft).toBe(360);
    const scales = screen.getByRole("group", { name: "甘特图时间刻度" });
    await userEvent.click(within(scales).getByRole("button", { name: "季" }));
    expect(view.container.querySelectorAll(".work-item-board__gantt-dates > span")).toHaveLength(3);
    await userEvent.click(within(scales).getByRole("button", { name: "年" }));
    expect(view.container.querySelectorAll(".work-item-board__gantt-dates > span")).toHaveLength(4);
    expect(view.container.querySelector(".work-item-board__gantt-dates")).toHaveTextContent("第1季度1–3月");
  });

  it("creates a one-day work item from a calendar date and names the lunar day", async () => {
    const gateway = gatewayWith([]);
    renderBoard(gateway);
    await userEvent.click(screen.getByRole("button", { name: "日历" }));
    const date = await screen.findByRole("button", { name: "在2026-08-15创建事项，农历初三" });
    await userEvent.click(date);
    const dialog = screen.getByRole("dialog", { name: "新建待办" });
    expect(within(dialog).getByLabelText("开始日期")).toHaveValue("2026-08-15");
    expect(within(dialog).getByLabelText("截止日期")).toHaveValue("2026-08-15");
  });

  it("auto-paginates a month and ignores a late response from the previous month", async () => {
    const august = deferred<{ projectScopeKey: string; projectId: string; items: CloudProjectWorkItem[]; nextCursor: null; totalCount: number; undatedCount: number }>();
    const septemberFirst = { ...item("sep-1", "九月第一项", "todo"), startAt: "2026-09-01T00:00:00Z" };
    const septemberSecond = { ...item("sep-2", "九月第二项", "done"), dueAt: "2026-09-02T23:59:59Z" };
    const list = vi.fn(async (input: { status?: string; cursor?: string; scheduledFrom?: string }) => {
      if (!input.scheduledFrom) return { projectScopeKey: "opaque-a", projectId: "p-1", items: [], nextCursor: null, totalCount: 0, undatedCount: 0 };
      if (input.scheduledFrom === "2026-07-27") return august.promise;
      if (!input.cursor) return { projectScopeKey: "opaque-a", projectId: "p-1", items: [septemberFirst], nextCursor: "sep-next", totalCount: 2, undatedCount: 5 };
      return { projectScopeKey: "opaque-a", projectId: "p-1", items: [septemberSecond], nextCursor: null, totalCount: 2, undatedCount: 5 };
    });
    const gateway = gatewayWith([], { listCloudProjectWorkItems: list as ProjectGateway["listCloudProjectWorkItems"] });
    renderBoard(gateway);
    await userEvent.click(screen.getByRole("button", { name: "日历" }));
    await waitFor(() => expect(list).toHaveBeenCalledWith(expect.objectContaining({ scheduledFrom: "2026-07-27" })));
    await userEvent.click(screen.getByRole("button", { name: "下个月" }));
    expect(await screen.findByRole("button", { name: "查看 九月第一项，2026-09-01" })).toBeVisible();
    expect(await screen.findByRole("button", { name: "查看 九月第二项，2026-09-02" })).toBeVisible();
    expect(screen.getByText("无日期记录：5")).toBeVisible();
    august.resolve({ projectScopeKey: "opaque-a", projectId: "p-1", items: [{ ...item("aug-late", "迟到八月项", "todo"), startAt: "2026-08-12T00:00:00Z" }], nextCursor: null, totalCount: 1, undatedCount: 1 });
    await Promise.resolve();
    expect(screen.getByRole("heading", { name: "2026年9月", level: 2 })).toBeVisible();
    expect(screen.getByText("无日期记录：5")).toBeVisible();
  });

  it("extends the authoritative month window when the calendar is scrolled vertically", async () => {
    const gateway = gatewayWith([]);
    const view = renderBoard(gateway);
    await userEvent.click(screen.getByRole("button", { name: "日历" }));
    await waitFor(() => expect(view.container.querySelectorAll("[data-calendar-month]")).toHaveLength(3));
    const scroller = screen.getByLabelText("连续月份月历，使用滚轮或 PageUp 和 PageDown 浏览");
    Object.defineProperty(scroller, "clientHeight", { configurable: true, value: 400 });
    Object.defineProperty(scroller, "scrollHeight", { configurable: true, value: 1600 });
    Object.defineProperty(scroller, "scrollTop", { configurable: true, writable: true, value: 1180 });
    fireEvent.scroll(scroller);
    await waitFor(() => expect(view.container.querySelectorAll("[data-calendar-month]")).toHaveLength(4));
    expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({ status: "all", scheduledFrom: "2026-09-28", scheduledTo: "2026-11-08" }));
  });

  it("makes later pages reachable from every non-board projection", async () => {
    const gateway = gatewayWith([
      { ...item("todo-1", "第一项", "todo"), startAt: "2026-08-10T00:00:00Z" },
      { ...item("todo-51", "第五十一项", "todo"), dueAt: "2026-08-12T23:59:59Z" },
    ]);
    const view = renderBoard(gateway);
    await screen.findByText("第一项");
    await userEvent.click(screen.getByRole("button", { name: "表格" }));
    await userEvent.click(screen.getByRole("button", { name: /加载剩余事项/ }));
    await waitFor(() => expect(view.container).toHaveTextContent("第五十一项"));
    for (const mode of ["列表", "甘特", "日历"] as const) {
      await userEvent.click(screen.getByRole("button", { name: mode }));
      expect(view.container).toHaveTextContent("第五十一项");
    }
  });

  it("ignores a late load-more response from an obsolete search", async () => {
    const late = deferred<{ projectScopeKey: string; projectId: string; items: CloudProjectWorkItem[]; nextCursor: null; totalCount: number; undatedCount: number }>();
    const list = vi.fn(async (input: { status?: CloudProjectWorkItemStatus; cursor?: string; search?: string }) => {
      if (input.status === "todo" && input.cursor === "old-next") return late.promise;
      const title = input.search === "new" ? "新筛选" : "旧筛选";
      return { projectScopeKey: "opaque-a", projectId: "p-1", items: input.status === "todo" ? [item(`${title}-1`, title, "todo")] : [], nextCursor: input.status === "todo" && !input.search ? "old-next" : null, totalCount: input.status === "todo" ? 2 : 0, undatedCount: 0 };
    });
    const gateway = gatewayWith([], { listCloudProjectWorkItems: list as ProjectGateway["listCloudProjectWorkItems"] });
    renderBoard(gateway);
    await screen.findByText("旧筛选");
    await userEvent.click(screen.getByRole("button", { name: "加载更多（1/2）" }));
    await userEvent.click(screen.getByRole("button", { name: "搜索事项" }));
    const search = screen.getByRole("searchbox", { name: "搜索事项" });
    await userEvent.type(search, "new");
    fireEvent.blur(search);
    await screen.findByText("新筛选");
    late.resolve({ projectScopeKey: "opaque-a", projectId: "p-1", items: [item("late-1", "迟到旧页", "todo")], nextCursor: null, totalCount: 2, undatedCount: 0 });
    await Promise.resolve();
    expect(screen.queryByText("迟到旧页")).toBeNull();
  });

  it("creates in a selected column and moves once through the keyboard dialog", async () => {
    const todo = item("todo-1", "Alpha", "todo");
    const create = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: item("new-1", "新事项", "in_progress") });
    const move = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: { ...todo, status: "in_progress", version: 2 } });
    const gateway = gatewayWith([todo], { createCloudProjectWorkItem: create, moveCloudProjectWorkItem: move });
    renderBoard(gateway);
    await screen.findByText("Alpha");
    await userEvent.click(screen.getByRole("button", { name: "在进行中中新建事项" }));
    const dialog = screen.getByRole("dialog", { name: "新建待办" });
    expect(dialog).toHaveClass("work-item-board__dialog--editor");
    const modalControls = dialog.querySelector(".project-modal-surface__controls");
    expect(modalControls).not.toBeNull();
    expect(within(modalControls as HTMLElement).getAllByRole("button")).toHaveLength(2);
    expect(within(modalControls as HTMLElement).getByRole("button", { name: "放大弹窗" })).toBeVisible();
    expect(within(modalControls as HTMLElement).getByRole("button", { name: "关闭" })).toBeVisible();
    expect(within(dialog).getAllByRole("button", { name: "关闭" })).toHaveLength(1);
    await userEvent.click(within(dialog).getByRole("button", { name: "放大弹窗" }));
    expect(dialog).toHaveClass("project-modal-surface--maximized");
    await userEvent.click(within(dialog).getByRole("button", { name: "还原弹窗" }));
    expect(dialog).not.toHaveClass("project-modal-surface--maximized");
    expect(within(dialog).getByRole("button", { name: "状态：进行中" })).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "处理人：未指派" })).toBeVisible();
    expect(within(dialog).getByLabelText("开始日期")).toBeVisible();
    expect(within(dialog).getByLabelText("截止日期")).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "优先级：无" })).toBeVisible();
    await userEvent.click(within(dialog).getByRole("button", { name: "状态：进行中" }));
    expect(screen.getByRole("listbox", { name: "状态" })).toBeVisible();
    expect(screen.getByRole("option", { name: "待开始" })).toHaveClass("work-item-board__status-option--todo");
    await userEvent.keyboard("{Escape}");
    await userEvent.click(within(dialog).getByRole("button", { name: "优先级：无" }));
    expect(screen.getByRole("option", { name: "紧急" })).toHaveClass("work-item-board__priority-option--urgent");
    await userEvent.keyboard("{Escape}");
    await userEvent.click(within(dialog).getByRole("button", { name: "处理人：未指派" }));
    await userEvent.click(screen.getByRole("option", { name: "成员甲" }));
    expect(within(dialog).getByRole("button", { name: "处理人：成员甲" })).toBeVisible();
    fireEvent.change(within(dialog).getByLabelText("开始日期"), { target: { value: "2026-08-15" } });
    fireEvent.change(within(dialog).getByLabelText("截止日期"), { target: { value: "2026-08-14" } });
    expect(within(dialog).getByText("截止日期不能早于开始日期。")).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "创建" })).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText("截止日期"), { target: { value: "2026-08-16" } });
    await userEvent.type(within(dialog).getByLabelText("标题"), "新事项");
    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));
    await waitFor(() => expect(create).toHaveBeenCalledWith(expect.objectContaining({ status: "in_progress", title: "新事项", assigneeUserId: "usr-1", startAt: "2026-08-15T00:00:00.000Z", dueAt: "2026-08-16T23:59:59.999Z" })));
    await userEvent.click(screen.getByRole("button", { name: "移动 Alpha" }));
    const moveDialog = screen.getByRole("dialog", { name: "移动事项" });
    await userEvent.selectOptions(within(moveDialog).getByLabelText("目标状态"), "in_progress");
    await userEvent.click(within(moveDialog).getByRole("button", { name: "确认移动" }));
    await waitFor(() => expect(move).toHaveBeenCalledTimes(1));
    expect(move).toHaveBeenCalledWith(expect.objectContaining({ workItemId: "todo-1", targetStatus: "in_progress", expectedVersion: 1 }));
  });

  it("keeps description and date interactions focused on the field the user chose", async () => {
    const gateway = gatewayWith([]);
    renderBoard(gateway);
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "添加" }));
    const dialog = screen.getByRole("dialog", { name: "新建待办" });
    const title = within(dialog).getByLabelText("标题");
    const description = within(dialog).getByLabelText("说明");
    const startDate = within(dialog).getByLabelText("开始日期");
    const showPicker = vi.fn();
    Object.defineProperty(startDate, "showPicker", { configurable: true, value: showPicker });

    await userEvent.type(title, "事项标题");
    await userEvent.click(description);
    await userEvent.type(description, "可以连续填写的事项描述");
    expect(description).toHaveFocus();
    expect(title).toHaveValue("事项标题");
    expect(description).toHaveValue("可以连续填写的事项描述");

    await userEvent.click(startDate);
    expect(showPicker).toHaveBeenCalledTimes(1);
    fireEvent.change(startDate, { target: { value: "2026-08-15" } });
    expect(startDate).toHaveFocus();
  });

  it("does not create a duplicate when relationship binding fails after creation", async () => {
    const created = item("new-1", "带标签事项", "todo");
    const create = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: created });
    const attach = vi.fn().mockRejectedValueOnce(new Error("binding failed")).mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: { ...created, version: 2, labels: [{ labelId: "lbl-1", name: "发布", color: "#3366FF", version: 1 }] } });
    const label = { labelId: "lbl-1", name: "发布", color: "#3366FF", version: 1 };
    const gateway = gatewayWith([], {
      createCloudProjectWorkItem: create,
      putCloudProjectWorkItemLabel: attach,
      deleteCloudProjectWorkItemLabel: vi.fn(),
      listCloudProjectLabels: vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [label] }),
    });
    renderBoard(gateway);
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "添加" }));
    const dialog = screen.getByRole("dialog", { name: "新建待办" });
    await userEvent.type(within(dialog).getByLabelText("标题"), "带标签事项");
    await userEvent.click(within(dialog).getByRole("button", { name: "标签：已选择 0 个" }));
    await userEvent.click(screen.getByRole("checkbox", { name: "发布" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));
    const retryDialog = await screen.findByRole("dialog", { name: "编辑待办" });
    await userEvent.click(within(retryDialog).getByRole("button", { name: "标签：已选择 1 个" }));
    expect(screen.getByRole("checkbox", { name: "发布" })).toBeChecked();
    const retry = within(retryDialog).getByRole("button", { name: "保存" });
    await waitFor(() => expect(retry).toBeEnabled());
    await userEvent.click(retry);
    await waitFor(() => expect(attach).toHaveBeenCalledTimes(2));
    expect(create).toHaveBeenCalledTimes(1);
  });

  it("creates and selects a label in a dedicated dialog before creating a work item", async () => {
    const label = { labelId: "lbl-1", name: "发布", color: "#FF3366", version: 1 };
    const createdItem = item("new-1", "带新标签事项", "todo");
    const createLabel = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [label] });
    const createItem = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: createdItem });
    const attachLabel = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: { ...createdItem, version: 2, labels: [label] } });
    const gateway = gatewayWith([], {
      createCloudProjectLabel: createLabel,
      createCloudProjectWorkItem: createItem,
      putCloudProjectWorkItemLabel: attachLabel,
      deleteCloudProjectWorkItemLabel: vi.fn(),
    });
    renderBoard(gateway);
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "添加" }));
    const dialog = screen.getByRole("dialog", { name: "新建待办" });
    await userEvent.type(within(dialog).getByLabelText("标题"), "带新标签事项");
    await userEvent.click(within(dialog).getByRole("button", { name: "标签：已选择 0 个" }));
    expect(screen.getByRole("textbox", { name: "搜索标签" })).toHaveAttribute("placeholder", "搜索标签");
    const addLabel = screen.getByRole("button", { name: "新增标签" });
    expect(addLabel).toBeVisible();
    await userEvent.click(addLabel);
    const labelDialog = screen.getByRole("dialog", { name: "新建标签" });
    await userEvent.type(within(labelDialog).getByRole("textbox", { name: "标签名称" }), "发布");
    fireEvent.change(within(labelDialog).getByLabelText("标签颜色"), { target: { value: "#ff3366" } });
    await userEvent.click(within(labelDialog).getByRole("button", { name: "创建" }));

    await waitFor(() => expect(createLabel).toHaveBeenCalledWith({ projectId: "p-1", name: "发布", color: "#ff3366" }));
    expect(screen.queryByRole("dialog", { name: "新建标签" })).toBeNull();
    expect(within(dialog).getByRole("button", { name: "标签：已选择 1 个" })).toBeVisible();
    await userEvent.click(within(dialog).getByRole("button", { name: "标签：已选择 1 个" }));
    expect(screen.getByRole("checkbox", { name: "发布" })).toBeChecked();
    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));

    await waitFor(() => expect(attachLabel).toHaveBeenCalledWith({ projectId: "p-1", workItemId: "new-1", labelId: "lbl-1", expectedVersion: 1 }));
    expect(createItem).toHaveBeenCalledTimes(1);
  });

  it("edits a table title, priority, status and assignee through real mutations", async () => {
    const todo = item("todo-1", "旧标题", "todo");
    const patch = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: { ...todo, title: "新标题", version: 2 } });
    const move = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", workItem: { ...todo, status: "in_progress", version: 2 } });
    const gateway = gatewayWith([todo], { patchCloudProjectWorkItem: patch, moveCloudProjectWorkItem: move });
    renderBoard(gateway);
    await screen.findByText("旧标题");
    await userEvent.click(screen.getByRole("button", { name: "表格" }));
    await userEvent.click(screen.getByRole("button", { name: "编辑标题 旧标题" }));
    const title = screen.getByRole("textbox", { name: "编辑标题 旧标题" });
    await userEvent.clear(title);
    await userEvent.type(title, "新标题{Enter}");
    await waitFor(() => expect(patch).toHaveBeenCalledWith({ projectId: "p-1", workItemId: "todo-1", expectedVersion: 1, title: "新标题" }));
    await userEvent.click(screen.getByRole("button", { name: "优先级 旧标题：无" }));
    await userEvent.click(screen.getByRole("option", { name: "紧急" }));
    await waitFor(() => expect(patch).toHaveBeenCalledWith({ projectId: "p-1", workItemId: "todo-1", expectedVersion: 1, priority: "urgent" }));
    await userEvent.click(screen.getByRole("button", { name: "状态 旧标题：待开始" }));
    await userEvent.click(screen.getByRole("option", { name: "进行中" }));
    await waitFor(() => expect(move).toHaveBeenCalledWith(expect.objectContaining({ projectId: "p-1", workItemId: "todo-1", expectedVersion: 1, targetStatus: "in_progress" })));
    await userEvent.click(screen.getByRole("button", { name: "处理人 旧标题：未指派" }));
    await userEvent.click(screen.getByRole("option", { name: "成员甲" }));
    await waitFor(() => expect(patch).toHaveBeenCalledWith({ projectId: "p-1", workItemId: "todo-1", expectedVersion: 1, assigneeSet: true, assigneeUserId: "usr-1" }));
  });

  it("creates, updates and deletes labels through Cloud readback", async () => {
    const label = { labelId: "lbl-1", name: "发布", color: "#3366FF", version: 1 };
    const create = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [label] });
    const patch = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [{ ...label, name: "已发布", version: 2 }] });
    const remove = vi.fn().mockResolvedValue({ projectScopeKey: "opaque-a", projectId: "p-1", items: [] });
    const gateway = gatewayWith([], { createCloudProjectLabel: create, patchCloudProjectLabel: patch, deleteCloudProjectLabel: remove });
    renderBoard(gateway);
    await screen.findAllByText("暂无事项");
    await userEvent.click(screen.getByRole("button", { name: "标签设置" }));
    const dialog = screen.getByRole("dialog", { name: "标签设置" });
    await userEvent.type(within(dialog).getByPlaceholderText("新标签名称"), "发布");
    await userEvent.click(within(dialog).getByRole("button", { name: "创建" }));
    await waitFor(() => expect(create).toHaveBeenCalled());
    const name = within(dialog).getByDisplayValue("发布");
    await userEvent.clear(name);
    await userEvent.type(name, "已发布");
    await userEvent.click(within(dialog).getByRole("button", { name: "保存" }));
    await waitFor(() => expect(patch).toHaveBeenCalledWith(expect.objectContaining({ labelId: "lbl-1", expectedVersion: 1, name: "已发布" })));
    await userEvent.click(within(dialog).getByRole("button", { name: "删除" }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith({ projectId: "p-1", labelId: "lbl-1", expectedVersion: 2 }));
  });

  it("keeps archived plans read-only", async () => {
    const gateway = gatewayWith([item("todo-1", "Alpha", "todo")]);
    renderBoard(gateway, true);
    await screen.findByText("Alpha");
    expect(screen.getByRole("button", { name: "添加" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "在待开始中新建事项" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "移动 Alpha" })).toBeDisabled();
  });
});
