import { StrictMode } from "react";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ProjectGateway } from "../features/projects/data/projectGateway";
import type { CloudProjectWorkItem } from "../features/projects/model/projectTypes";
import { ContinuousWorkItemCalendar } from "../features/projects/ui/ContinuousWorkItemCalendar";

const projectId = "prj_01M0HVG37Z8GTFTJSA7H5HZ99A";
const projectScopeKey = "captured-project-scope";

const capturedCalendarItems: CloudProjectWorkItem[] = [
  {
    workItemId: "wi_01M0HW6AZ60VKYH4MNRAN9ARP7",
    projectId,
    title: "测试",
    description: "测试内容",
    status: "in_progress",
    rank: 1000,
    version: 3,
    createdAt: "2026-08-21T09:59:48.45179Z",
    updatedAt: "2026-08-21T10:04:34.949127Z",
    assigneeUserId: "usr_01KYRYVGJB8GYKPM4K37ANQ2QC",
    assigneeStatus: "active",
    assigneeDisplayName: "185****7918",
    priority: "urgent",
    startAt: "2026-08-21T00:00:00Z",
    dueAt: "2026-08-31T23:59:59.999Z",
    labels: [{ labelId: "lbl_01M0HW8HRDQA2G4KAHHRRQH6TT", name: "1", color: "#3366FF", version: 1 }],
  },
  {
    workItemId: "wi_01M0J7K7S9E8DREBSV5AWV059H",
    projectId,
    title: "添加一个事项。",
    description: "123",
    status: "in_progress",
    rank: 2000,
    version: 2,
    createdAt: "2026-08-21T13:19:05.504784Z",
    updatedAt: "2026-08-21T13:19:05.600921Z",
    assigneeUserId: "usr_01M0CNTP0FF9ZJ3XZN8QET3CHZ",
    assigneeStatus: "active",
    assigneeDisplayName: "198****4585",
    priority: "urgent",
    startAt: "2026-08-21T00:00:00Z",
    dueAt: "2026-08-23T23:59:59.999Z",
    labels: [{ labelId: "lbl_01M0HW8HRDQA2G4KAHHRRQH6TT", name: "1", color: "#3366FF", version: 1 }],
  },
  {
    workItemId: "wi_01M0HW5KRNHYE4SHJ2W2V6Y83R",
    projectId,
    title: "185第一个事项。",
    description: "第一个事项。",
    status: "todo",
    rank: 1000,
    version: 2,
    createdAt: "2026-08-21T09:59:24.691524Z",
    updatedAt: "2026-08-21T14:17:15.270685Z",
    assigneeUserId: null,
    assigneeStatus: null,
    assigneeDisplayName: null,
    priority: "none",
    startAt: "2026-08-22T00:00:00Z",
    dueAt: "2026-08-22T23:59:59.999Z",
    labels: [],
  },
];

function capturedGateway() {
  const listCloudProjectWorkItems = vi.fn(async (input: { scheduledFrom?: string; scheduledTo?: string }) => {
    const items = input.scheduledFrom === "2026-07-27" && input.scheduledTo === "2026-09-06"
      ? capturedCalendarItems
      : [];
    return {
      projectScopeKey,
      projectId,
      items,
      nextCursor: null,
      totalCount: items.length,
      undatedCount: 0,
    };
  });
  return { listCloudProjectWorkItems } as unknown as ProjectGateway & { listCloudProjectWorkItems: typeof listCloudProjectWorkItems };
}

function item(workItemId: string, title: string, startAt: string | null, dueAt: string | null): CloudProjectWorkItem {
  return {
    ...capturedCalendarItems[2],
    workItemId,
    title,
    startAt,
    dueAt,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function calendar(props: Partial<Parameters<typeof ContinuousWorkItemCalendar>[0]> = {}) {
  return <ContinuousWorkItemCalendar
    projectScopeKey={projectScopeKey}
    projectId={projectId}
    projectName="185新建第一个云项目"
    filters={{ search: "", assigneeUserId: "", priority: "", labelId: "" }}
    gateway={capturedGateway()}
    refreshRevision={0}
    onEdit={vi.fn()}
    onCreate={vi.fn()}
    readOnly={false}
    {...props}
  />;
}

describe("ContinuousWorkItemCalendar", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-08-21T04:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("centers today's week on entry and from Today without resetting a refreshed view", async () => {
    const animationFrames: FrameRequestCallback[] = [];
    const requestAnimationFrameSpy = vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      animationFrames.push(callback);
      return animationFrames.length;
    });
    const originalClientHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientHeight");
    Object.defineProperty(HTMLElement.prototype, "clientHeight", {
      configurable: true,
      get: function clientHeight(this: HTMLElement) { return this.classList.contains("work-item-board__calendar-scroll") ? 400 : 0; },
    });
    const rect = (top: number, height: number) => ({ x: 0, y: top, top, right: 100, bottom: top + height, left: 0, width: 100, height, toJSON: () => ({}) }) as DOMRect;
    const rectSpy = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function mockRect(this: HTMLElement) {
      const element = this;
      if (element.classList.contains("work-item-board__calendar-scroll")) return rect(100, 400);
      if (element.dataset.calendarDay === "2026-08-21" && element.closest('[data-calendar-month="2026-08"]')) {
        const scroller = element.closest("[data-plan-calendar]")?.querySelector<HTMLElement>(".work-item-board__calendar-scroll");
        return rect(700 - (scroller?.scrollTop ?? 0), 80);
      }
      return rect(0, 0);
    });

    try {
      const gateway = capturedGateway();
      const view = render(calendar({ gateway }));
      const scroller = screen.getByLabelText("连续月份月历，使用滚轮或 PageUp 和 PageDown 浏览");
      expect(scroller.scrollTop).toBe(440);

      scroller.scrollTop = 0;
      animationFrames.splice(0).forEach((callback) => callback(performance.now()));
      expect(scroller.scrollTop).toBe(440);

      const scrollTo = vi.fn(({ top }: ScrollToOptions) => { scroller.scrollTop = top ?? scroller.scrollTop; });
      Object.defineProperty(scroller, "scrollTo", { configurable: true, value: scrollTo });
      scroller.scrollTop = 900;
      view.rerender(calendar({ gateway, refreshRevision: 1 }));
      await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledTimes(6));
      expect(scroller.scrollTop).toBe(900);

      await userEvent.click(screen.getByRole("button", { name: "今天" }));
      expect(scrollTo).toHaveBeenCalledWith({ top: 440, behavior: "smooth" });
      expect(scroller.scrollTop).toBe(440);
    } finally {
      requestAnimationFrameSpy.mockRestore();
      rectSpy.mockRestore();
      if (originalClientHeight) Object.defineProperty(HTMLElement.prototype, "clientHeight", originalClientHeight);
      else delete (HTMLElement.prototype as { clientHeight?: number }).clientHeight;
    }
  });

  it("renders the captured authoritative response under the app StrictMode boundary", async () => {
    const gateway = capturedGateway();
    const onEdit = vi.fn();
    render(
      <StrictMode>
        {calendar({ gateway, onEdit })}
      </StrictMode>,
    );

    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({
      projectId,
      status: "all",
      pageSize: 100,
      scheduledFrom: "2026-07-27",
      scheduledTo: "2026-09-06",
    })));
    const testSegments = await screen.findAllByRole("button", { name: /^查看 测试，/ });
    expect(testSegments).toHaveLength(3);
    expect(testSegments.map((segment) => [segment.dataset.rangeStart, segment.dataset.rangeEnd])).toEqual([
      ["2026-08-21", "2026-08-23"],
      ["2026-08-24", "2026-08-30"],
      ["2026-08-31", "2026-08-31"],
    ]);
    expect(testSegments[0]).toHaveAttribute("data-continues-after", "true");
    expect(testSegments[1]).toHaveAttribute("data-continues-before", "true");
    expect(testSegments[1]).toHaveAttribute("data-continues-after", "true");
    expect(testSegments[2]).toHaveAttribute("data-continues-before", "true");
    await userEvent.click(screen.getByRole("button", { name: /^2026-08-22还有\d+项，展开全部$/ }));
    const dayDialog = screen.getByRole("dialog", { name: "2026-08-22全部事项" });
    await userEvent.click(within(dayDialog).getByRole("button", { name: "查看 185第一个事项。" }));
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ title: "185第一个事项。" }), expect.any(HTMLButtonElement));
    expect(screen.queryByText(/(?:开始|截止) ·/)).toBeNull();
    await userEvent.click(testSegments[1]);
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ workItemId: "wi_01M0HW6AZ60VKYH4MNRAN9ARP7" }), expect.any(HTMLButtonElement));
    await userEvent.click(screen.getByRole("button", { name: "上个月" }));
    expect(screen.getByRole("heading", { name: "2026年7月", level: 2 })).toBeVisible();
    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({
      scheduledFrom: "2026-06-29",
      scheduledTo: "2026-08-09",
    })));
    await userEvent.click(screen.getByRole("button", { name: "今天" }));
    expect(screen.getByRole("heading", { name: "2026年8月", level: 2 })).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "下个月" }));
    expect(screen.getByRole("heading", { name: "2026年9月", level: 2 })).toBeVisible();
    await waitFor(() => expect(gateway.listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({
      scheduledFrom: "2026-08-31",
      scheduledTo: "2026-10-11",
    })));
    await userEvent.click(screen.getByRole("button", { name: "今天" }));
    expect(screen.getByRole("heading", { name: "2026年8月", level: 2 })).toBeVisible();
  });

  it("expands all items for a busy day without replacing blank-day creation", async () => {
    const busyDayItems = Array.from({ length: 5 }, (_, index) => item(
      `busy-${index + 1}`,
      `事项 ${index + 1}`,
      "2026-08-22T00:00:00Z",
      null,
    ));
    const listCloudProjectWorkItems = vi.fn(async (input: { scheduledFrom?: string }) => {
      const items = input.scheduledFrom === "2026-07-27" ? busyDayItems : [];
      return { projectScopeKey, projectId, items, nextCursor: null, totalCount: items.length, undatedCount: 0 };
    });
    const onEdit = vi.fn();
    const onCreate = vi.fn();
    render(calendar({ gateway: { listCloudProjectWorkItems } as unknown as ProjectGateway, onEdit, onCreate }));

    expect(await screen.findByRole("button", { name: "查看 事项 1，2026-08-22" })).toBeVisible();
    expect(screen.getByRole("button", { name: "查看 事项 2，2026-08-22" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "查看 事项 5，2026-08-22" })).toBeNull();
    const moreButton = screen.getByRole("button", { name: "2026-08-22还有3项，展开全部" });
    expect(moreButton.querySelector("svg")).not.toBeNull();
    await userEvent.click(moreButton);
    const dialog = screen.getByRole("dialog", { name: "2026-08-22全部事项" });
    expect(within(dialog).getAllByRole("button")).toHaveLength(5);
    await userEvent.click(within(dialog).getByRole("button", { name: "查看 事项 5" }));
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ workItemId: "busy-5" }), expect.any(HTMLButtonElement));

    await userEvent.click(screen.getByRole("button", { name: /在2026-08-22创建事项/ }));
    expect(onCreate).toHaveBeenCalledWith("2026-08-22", expect.any(HTMLButtonElement));
  });

  it("keeps filters on every cursor page and renders start-only, due-only, same-day and undated semantics", async () => {
    const startOnly = item("start-only", "仅开始", "2026-08-10T00:00:00Z", null);
    const dueOnly = item("due-only", "仅截止", null, "2026-08-10T23:59:59.999Z");
    const sameDay = item("same-day", "同日", "2026-08-12T00:00:00Z", "2026-08-12T15:59:59.999Z");
    const listCloudProjectWorkItems = vi.fn(async (input: { scheduledFrom?: string; cursor?: string }) => {
      if (input.scheduledFrom !== "2026-07-27") return { projectScopeKey, projectId, items: [], nextCursor: null, totalCount: 0, undatedCount: 1 };
      if (!input.cursor) return { projectScopeKey, projectId, items: [startOnly, sameDay], nextCursor: "page-2", totalCount: 3, undatedCount: 1 };
      return { projectScopeKey, projectId, items: [dueOnly, sameDay], nextCursor: null, totalCount: 3, undatedCount: 1 };
    });
    const gateway = { listCloudProjectWorkItems } as unknown as ProjectGateway;
    render(<StrictMode>{calendar({
      gateway,
      filters: { search: "发布", assigneeUserId: "usr-1", priority: "urgent", labelId: "label-1" },
    })}</StrictMode>);

    const startNode = await screen.findByRole("button", { name: "查看 仅开始，2026-08-10" });
    const dueNode = screen.getByRole("button", { name: "查看 仅截止，2026-08-10" });
    expect(startNode).toHaveAttribute("data-milestone", "true");
    expect(dueNode).toHaveAttribute("data-milestone", "true");
    const sameDayRange = screen.getByRole("button", { name: "查看 同日，2026-08-12" });
    expect(sameDayRange).not.toHaveAttribute("data-milestone");
    expect(sameDayRange).toHaveAttribute("data-range-start", "2026-08-12");
    expect(sameDayRange).toHaveAttribute("data-range-end", "2026-08-12");
    expect(screen.getByText("无日期记录：1")).toBeVisible();
    expect(screen.getAllByRole("button", { name: "查看 同日，2026-08-12" })).toHaveLength(1);
    expect(listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({
      cursor: "page-2",
      search: "发布",
      assigneeUserId: "usr-1",
      priority: "urgent",
      labelId: "label-1",
      scheduledFrom: "2026-07-27",
      scheduledTo: "2026-09-06",
    }));
  });

  it("renders closed ranges across days, weeks and months with shared status visuals", async () => {
    const items: CloudProjectWorkItem[] = [
      { ...item("two-day", "两日待开始", "2026-08-22T00:00:00Z", "2026-08-23T23:59:59.999Z"), status: "todo" },
      { ...item("cross-week", "跨周进行中", "2026-08-23T00:00:00Z", "2026-08-25T23:59:59.999Z"), status: "in_progress" },
      { ...item("cross-month", "跨月已暂停", "2026-08-31T00:00:00Z", "2026-09-02T23:59:59.999Z"), status: "paused" },
      { ...item("done", "单日已完成", "2026-08-26T00:00:00Z", "2026-08-26T23:59:59.999Z"), status: "done" },
    ];
    const gateway = {
      listCloudProjectWorkItems: vi.fn().mockResolvedValue({ projectScopeKey, projectId, items, nextCursor: null, totalCount: items.length, undatedCount: 0 }),
    } as unknown as ProjectGateway;
    const onEdit = vi.fn();
    render(calendar({ gateway, onEdit }));

    const twoDay = await screen.findByRole("button", { name: "查看 两日待开始，2026-08-22至2026-08-23" });
    expect(twoDay).toHaveStyle({ gridColumn: "6 / span 2" });
    expect(twoDay).toHaveClass("work-item-board__calendar-range--todo");
    expect(twoDay.querySelector("svg")).toHaveClass("lucide-circle");

    const crossWeek = screen.getAllByRole("button", { name: /^查看 跨周进行中，/ });
    expect(crossWeek.map((segment) => [segment.dataset.rangeStart, segment.dataset.rangeEnd])).toEqual([
      ["2026-08-23", "2026-08-23"],
      ["2026-08-24", "2026-08-25"],
    ]);
    expect(crossWeek[0]).toHaveClass("work-item-board__calendar-range--in_progress");
    expect(crossWeek[0].querySelector("svg")).toHaveClass("lucide-circle-play");

    const crossMonth = screen.getAllByRole("button", { name: /^查看 跨月已暂停，/ });
    expect(crossMonth.map((segment) => [segment.dataset.rangeStart, segment.dataset.rangeEnd])).toEqual([
      ["2026-08-31", "2026-08-31"],
      ["2026-09-01", "2026-09-02"],
    ]);
    expect(crossMonth[0]).toHaveAttribute("data-continues-after", "true");
    expect(crossMonth[1]).toHaveAttribute("data-continues-before", "true");
    expect(crossMonth[0]).toHaveClass("work-item-board__calendar-range--paused");
    expect(crossMonth[0].querySelector("svg")).toHaveClass("lucide-circle-pause");
    const done = screen.getByRole("button", { name: "查看 单日已完成，2026-08-26" });
    expect(done).toHaveClass("work-item-board__calendar-range--done");
    expect(done.querySelector("svg")).toHaveClass("lucide-circle-check");

    await userEvent.click(crossMonth[1]);
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ workItemId: "cross-month" }), crossMonth[1]);
  });

  it("shows the existing error state and retries the failed month", async () => {
    let failAugust = true;
    const listCloudProjectWorkItems = vi.fn(async (input: { scheduledFrom?: string }) => {
      if (input.scheduledFrom === "2026-07-27" && failAugust) throw new Error("cloud unavailable");
      const items = input.scheduledFrom === "2026-07-27" ? [capturedCalendarItems[0]] : [];
      return { projectScopeKey, projectId, items, nextCursor: null, totalCount: items.length, undatedCount: 0 };
    });
    render(calendar({ gateway: { listCloudProjectWorkItems } as unknown as ProjectGateway }));

    expect(await screen.findByRole("alert")).toHaveTextContent("月历加载失败，请重试。");
    expect(screen.queryByRole("button", { name: /^查看 测试，/ })).toBeNull();
    failAugust = false;
    await userEvent.click(screen.getByRole("button", { name: "重试" }));
    expect((await screen.findAllByRole("button", { name: /^查看 测试，/ }))[0]).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("does not let an obsolete project response overwrite the current project", async () => {
    const oldPage = deferred<{ projectScopeKey: string; projectId: string; items: CloudProjectWorkItem[]; nextCursor: null; totalCount: number; undatedCount: number }>();
    const newProjectId = "prj-new";
    const newScope = "scope-new";
    const current = { ...item("current", "当前项目事项", "2026-08-15T00:00:00Z", null), projectId: newProjectId };
    const obsolete = item("obsolete", "旧项目迟到事项", "2026-08-16T00:00:00Z", null);
    const listCloudProjectWorkItems = vi.fn(async (input: { projectId: string; scheduledFrom?: string }) => {
      if (input.projectId === projectId && input.scheduledFrom === "2026-07-27") return oldPage.promise;
      const isCurrentMonth = input.scheduledFrom === "2026-07-27";
      return {
        projectScopeKey: input.projectId === newProjectId ? newScope : projectScopeKey,
        projectId: input.projectId,
        items: input.projectId === newProjectId && isCurrentMonth ? [current] : [],
        nextCursor: null,
        totalCount: input.projectId === newProjectId && isCurrentMonth ? 1 : 0,
        undatedCount: 0,
      };
    });
    const gateway = { listCloudProjectWorkItems } as unknown as ProjectGateway;
    const view = render(calendar({ gateway }));
    await waitFor(() => expect(listCloudProjectWorkItems).toHaveBeenCalledWith(expect.objectContaining({ projectId, scheduledFrom: "2026-07-27" })));
    view.rerender(calendar({ gateway, projectId: newProjectId, projectScopeKey: newScope, projectName: "新项目" }));
    expect(await screen.findByRole("button", { name: "查看 当前项目事项，2026-08-15" })).toBeVisible();
    oldPage.resolve({ projectScopeKey, projectId, items: [obsolete], nextCursor: null, totalCount: 1, undatedCount: 0 });
    await waitFor(() => expect(screen.queryByRole("button", { name: "查看 旧项目迟到事项，2026-08-16" })).toBeNull());
  });
});
