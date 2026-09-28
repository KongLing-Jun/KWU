import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Pencil, Plus, UserRound } from "lucide-react";
import { useT } from "../../../lib/i18n";
import { formatProjectDateTime } from "../model/projectDateTime";
import type { CloudProjectMember, CloudProjectWorkItem, CloudProjectWorkItemStatus } from "../model/projectTypes";
import { ProjectSelect } from "../../../components/ProjectSelect";
import { WORK_ITEM_PRIORITY_OPTIONS, type WorkItemPriority } from "./WorkItemEditorDialog";
import { WORK_ITEM_STATUS_ICONS } from "./workItemStatusVisual";

export type ProjectionColumn = {
  items: CloudProjectWorkItem[];
  nextCursor: string | null;
  totalCount: number;
  loading: boolean;
};

const STATUS_ORDER: CloudProjectWorkItemStatus[] = ["todo", "in_progress", "paused", "done"];
const DAY_MS = 86_400_000;

function statusClass(status: CloudProjectWorkItemStatus) {
  return `work-item-board__status-chip work-item-board__status-chip--${status}`;
}

function assigneeOptions(item: CloudProjectWorkItem, members: CloudProjectMember[]) {
  const options = [{ value: "", label: "未指派" }, ...members.map((member) => ({ value: member.userId, label: member.displayName }))];
  if (item.assigneeUserId && !members.some((member) => member.userId === item.assigneeUserId)) {
    options.splice(1, 0, { value: item.assigneeUserId, label: item.assigneeDisplayName || "当前处理人" });
  }
  return options;
}

export function WorkItemTableProjection({ items, members, readOnly, pending, onOpen, onRename, onPriorityChange, onStatusChange, onAssigneeChange }: {
  items: CloudProjectWorkItem[];
  members: CloudProjectMember[];
  readOnly: boolean;
  pending: boolean;
  onOpen: (item: CloudProjectWorkItem, trigger: HTMLButtonElement) => void;
  onRename: (item: CloudProjectWorkItem, title: string) => Promise<boolean>;
  onPriorityChange: (item: CloudProjectWorkItem, priority: WorkItemPriority) => Promise<boolean>;
  onStatusChange: (item: CloudProjectWorkItem, status: CloudProjectWorkItemStatus) => Promise<boolean>;
  onAssigneeChange: (item: CloudProjectWorkItem, assigneeUserId: string) => Promise<boolean>;
}) {
  const t = useT();
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  const [editingId, setEditingId] = useState("");
  const [titleDraft, setTitleDraft] = useState("");
  const savingRef = useRef(false);
  const allSelected = items.length > 0 && items.every((item) => selected.has(item.workItemId));
  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(items.map((item) => item.workItemId)));
  const toggle = (id: string) => setSelected((current) => {
    const next = new Set(current);
    if (next.has(id)) next.delete(id); else next.add(id);
    return next;
  });
  useEffect(() => {
    if (editingId && !items.some((item) => item.workItemId === editingId)) setEditingId("");
  }, [editingId, items]);
  const beginRename = (item: CloudProjectWorkItem) => {
    if (readOnly || pending) return;
    setEditingId(item.workItemId);
    setTitleDraft(item.title);
  };
  const finishRename = async (item: CloudProjectWorkItem) => {
    if (savingRef.current || editingId !== item.workItemId) return;
    const title = titleDraft.trim();
    if (!title || title === item.title) { setEditingId(""); return; }
    savingRef.current = true;
    const saved = await onRename(item, title);
    savingRef.current = false;
    if (saved) setEditingId("");
  };

  return <div className="work-item-board__table-wrap" data-plan-table>
    <table aria-label="事项表格">
      <thead><tr>
        <th className="work-item-board__select-cell"><input type="checkbox" aria-label="选择全部已加载事项" checked={allSelected} onChange={toggleAll} /></th>
        <th className="work-item-board__index-cell" aria-label="序号" />
        <th>标题</th><th>优先级</th><th>状态</th><th>处理人</th><th>创建时间</th>
      </tr></thead>
      <tbody>{items.map((item, index) => <tr key={item.workItemId} data-selected={selected.has(item.workItemId) || undefined}>
        <td className="work-item-board__select-cell"><input type="checkbox" aria-label={`选择 ${item.title}`} checked={selected.has(item.workItemId)} onChange={() => toggle(item.workItemId)} /></td>
        <td className="work-item-board__index-cell">{index + 1}</td>
        <td>{editingId === item.workItemId ? <input autoFocus className="work-item-board__table-title-input" aria-label={`编辑标题 ${item.title}`} value={titleDraft} maxLength={240} disabled={pending} onChange={(event) => setTitleDraft(event.currentTarget.value)} onBlur={() => void finishRename(item)} onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); void finishRename(item); } else if (event.key === "Escape") { event.preventDefault(); setEditingId(""); } }} /> : <span className="work-item-board__table-title-cell"><button type="button" className="work-item-board__table-title" onClick={(event) => onOpen(item, event.currentTarget)} aria-label={`查看详情 ${item.title}`}>{item.title}</button><button type="button" className="work-item-board__table-rename" disabled={readOnly || pending} onClick={() => beginRename(item)} aria-label={`编辑标题 ${item.title}`}><Pencil size={13} aria-hidden="true" /></button></span>}</td>
        <td><ProjectSelect ariaLabel={`优先级 ${item.title}`} value={(item.priority || "none") as WorkItemPriority} className={`work-item-board__priority-select work-item-board__priority-select--${item.priority || "none"} work-item-board__table-priority-select`} disabled={readOnly || pending} options={WORK_ITEM_PRIORITY_OPTIONS} onChange={(priority) => { if (priority !== (item.priority || "none")) void onPriorityChange(item, priority); }} /></td>
        <td><ProjectSelect ariaLabel={`状态 ${item.title}`} value={item.status} className={`${statusClass(item.status)} work-item-board__table-status-select`} disabled={readOnly || pending} options={STATUS_ORDER.map((status) => ({ value: status, label: t(`workspace.cloudProjects.workItems.status.${status}`) }))} onChange={(status) => { if (status !== item.status) void onStatusChange(item, status); }} /></td>
        <td><ProjectSelect ariaLabel={`处理人 ${item.title}`} value={item.assigneeUserId || ""} className="work-item-board__person-chip work-item-board__table-assignee-select" prefix={<UserRound size={13} aria-hidden="true" />} disabled={readOnly || pending} options={assigneeOptions(item, members)} onChange={(assigneeUserId) => { if (assigneeUserId !== (item.assigneeUserId || "")) void onAssigneeChange(item, assigneeUserId); }} /></td>
        <td><time dateTime={item.createdAt}>{formatProjectDateTime(item.createdAt)}</time></td>
      </tr>)}</tbody>
      <tfoot><tr><td colSpan={7}><span aria-hidden="true">＋</span></td></tr></tfoot>
    </table>
  </div>;
}

export function WorkItemListProjection({ columns, readOnly, pending, onEdit, onCreate, onLoadMore }: {
  columns: Record<CloudProjectWorkItemStatus, ProjectionColumn>;
  readOnly: boolean;
  pending: boolean;
  onEdit: (item: CloudProjectWorkItem, trigger: HTMLButtonElement) => void;
  onCreate: (status: CloudProjectWorkItemStatus, trigger: HTMLButtonElement) => void;
  onLoadMore: (status: CloudProjectWorkItemStatus) => void;
}) {
  const t = useT();
  const [collapsed, setCollapsed] = useState<Set<CloudProjectWorkItemStatus>>(() => new Set());
  const toggle = (status: CloudProjectWorkItemStatus) => setCollapsed((current) => {
    const next = new Set(current);
    if (next.has(status)) next.delete(status); else next.add(status);
    return next;
  });

  return <div className="work-item-board__plan-list" data-plan-list>
    {STATUS_ORDER.map((status) => {
      const column = columns[status];
      const StatusIcon = WORK_ITEM_STATUS_ICONS[status];
      const isCollapsed = collapsed.has(status);
      return <section key={status} className={`work-item-board__list-group work-item-board__list-group--${status}`}>
        <header>
          <button type="button" className="work-item-board__list-toggle" onClick={() => toggle(status)} aria-expanded={!isCollapsed}>
            {isCollapsed ? <ChevronDown size={15} aria-hidden="true" /> : <ChevronUp size={15} aria-hidden="true" />}
            <StatusIcon size={16} aria-hidden="true" /><strong>{t(`workspace.cloudProjects.workItems.status.${status}`)}</strong><span>{column.totalCount}</span>
          </button>
          <button type="button" className="work-item-board__list-add" disabled={readOnly || pending} aria-label={`在${t(`workspace.cloudProjects.workItems.status.${status}`)}中新建事项`} onClick={(event) => onCreate(status, event.currentTarget)}><Plus size={17} aria-hidden="true" /></button>
        </header>
        {!isCollapsed ? <>
          <ol>{column.items.map((item) => <li key={item.workItemId}>
            <span className={`work-item-board__task-status work-item-board__task-status--${status}`} aria-hidden="true" />
            <button type="button" className="work-item-board__list-title" onClick={(event) => onEdit(item, event.currentTarget)}>{item.title}</button>
            <span className="work-item-board__list-assignee"><span className="work-item-board__avatar"><UserRound size={14} aria-hidden="true" /></span><span>{item.assigneeDisplayName || "未指派"}</span></span>
            <time dateTime={item.updatedAt}>{formatProjectDateTime(item.updatedAt)}</time>
          </li>)}</ol>
          {!column.items.length && !column.loading ? <button type="button" className="work-item-board__list-empty" disabled={readOnly || pending} onClick={(event) => onCreate(status, event.currentTarget)}>输入待办标题</button> : null}
          {column.nextCursor ? <button type="button" className="work-item-board__list-more" disabled={column.loading || pending} onClick={() => onLoadMore(status)}>{column.loading ? "加载中…" : `加载更多（${column.items.length}/${column.totalCount}）`}</button> : null}
        </> : null}
      </section>;
    })}
  </div>;
}

type GanttScale = "week" | "month" | "quarter" | "year";

function utcDay(value: Date) { return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth(), value.getUTCDate())); }
function addDays(value: Date, days: number) { return new Date(value.getTime() + days * DAY_MS); }
function startOfWeek(value: Date) { const day = (value.getUTCDay() + 6) % 7; return addDays(utcDay(value), -day); }
function startOfScale(anchor: Date, scale: GanttScale) {
  if (scale === "week") return startOfWeek(anchor);
  if (scale === "month") return new Date(Date.UTC(anchor.getUTCFullYear(), anchor.getUTCMonth(), 1));
  if (scale === "quarter") return new Date(Date.UTC(anchor.getUTCFullYear(), Math.floor(anchor.getUTCMonth() / 3) * 3, 1));
  return new Date(Date.UTC(anchor.getUTCFullYear(), 0, 1));
}
function endOfScale(start: Date, scale: GanttScale) {
  if (scale === "week") return addDays(start, 13);
  if (scale === "month") return addDays(new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + 1, 1)), -1);
  if (scale === "quarter") return addDays(new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + 3, 1)), -1);
  return addDays(new Date(Date.UTC(start.getUTCFullYear() + 1, 0, 1)), -1);
}
function moveAnchor(anchor: Date, scale: GanttScale, direction: -1 | 1) {
  if (scale === "week") return addDays(anchor, direction * 14);
  if (scale === "month") return new Date(Date.UTC(anchor.getUTCFullYear(), anchor.getUTCMonth() + direction, 1));
  if (scale === "quarter") return new Date(Date.UTC(anchor.getUTCFullYear(), anchor.getUTCMonth() + direction * 3, 1));
  return new Date(Date.UTC(anchor.getUTCFullYear() + direction, anchor.getUTCMonth(), 1));
}
function ganttTitle(start: Date, scale: GanttScale) {
  if (scale === "year") return `${start.getUTCFullYear()}年`;
  if (scale === "quarter") return `${start.getUTCFullYear()}年第${Math.floor(start.getUTCMonth() / 3) + 1}季度`;
  return `${start.getUTCFullYear()}年${start.getUTCMonth() + 1}月`;
}

function ganttHeaderUnits(start: Date, end: Date, scale: GanttScale) {
  const totalDays = Math.round((end.getTime() - start.getTime()) / DAY_MS) + 1;
  if (scale === "week" || scale === "month") return Array.from({ length: totalDays }, (_, index) => {
    const date = addDays(start, index);
    return { key: date.toISOString(), label: String(date.getUTCDate()), secondary: ["日", "一", "二", "三", "四", "五", "六"][date.getUTCDay()], days: 1, weekend: date.getUTCDay() === 0 || date.getUTCDay() === 6 };
  });
  if (scale === "quarter") return Array.from({ length: 3 }, (_, index) => {
    const unitStart = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + index, 1));
    const unitEnd = addDays(new Date(Date.UTC(unitStart.getUTCFullYear(), unitStart.getUTCMonth() + 1, 1)), -1);
    return { key: unitStart.toISOString(), label: `${unitStart.getUTCMonth() + 1}月`, secondary: `${Math.round((unitEnd.getTime() - unitStart.getTime()) / DAY_MS) + 1}天`, days: Math.round((unitEnd.getTime() - unitStart.getTime()) / DAY_MS) + 1, weekend: false };
  });
  return Array.from({ length: 4 }, (_, index) => {
    const unitStart = new Date(Date.UTC(start.getUTCFullYear(), index * 3, 1));
    const unitEnd = addDays(new Date(Date.UTC(start.getUTCFullYear(), index * 3 + 3, 1)), -1);
    return { key: unitStart.toISOString(), label: `第${index + 1}季度`, secondary: `${index * 3 + 1}–${index * 3 + 3}月`, days: Math.round((unitEnd.getTime() - unitStart.getTime()) / DAY_MS) + 1, weekend: false };
  });
}

export function WorkItemGanttProjection({ items, onEdit }: {
  items: CloudProjectWorkItem[];
  onEdit: (item: CloudProjectWorkItem, trigger: HTMLButtonElement) => void;
}) {
  const [scale, setScale] = useState<GanttScale>("month");
  const [anchor, setAnchor] = useState(() => utcDay(new Date()));
  const scrollerRef = useRef<HTMLDivElement>(null);
  const horizontalScrollbarRef = useRef<HTMLDivElement>(null);
  const pendingHorizontalScrollRef = useRef(new WeakMap<HTMLDivElement, number>());
  const todayFocusFrameRef = useRef<number | null>(null);
  const hasInitiallyFocusedTodayRef = useRef(false);
  const start = startOfScale(anchor, scale);
  const end = endOfScale(start, scale);
  const days = Math.round((end.getTime() - start.getTime()) / DAY_MS) + 1;
  const cellWidth = { week: 56, month: 48, quarter: 10, year: 3 }[scale];
  const timelineWidth = days * cellWidth;
  const headerUnits = useMemo(() => ganttHeaderUnits(start, end, scale), [end.getTime(), scale, start.getTime()]);
  const today = utcDay(new Date());
  const todayIndex = Math.round((today.getTime() - start.getTime()) / DAY_MS);

  const synchronizeHorizontalScroll = (left: number, targets: Array<HTMLDivElement | null>) => {
    for (const target of targets) {
      if (!target || target.scrollLeft === left) continue;
      pendingHorizontalScrollRef.current.set(target, left);
      target.scrollLeft = left;
    }
  };

  const scrollTodayIntoView = (index: number, width: number, totalDays: number) => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    const scrollbar = horizontalScrollbarRef.current;
    const target = 286 + index * width + width / 2 - scroller.clientWidth / 2;
    const commonMaximum = Math.max(0, 286 + totalDays * width - Math.max(scroller.clientWidth, scrollbar?.clientWidth ?? 0));
    synchronizeHorizontalScroll(Math.min(commonMaximum, Math.max(0, target)), [scroller, scrollbar]);
  };

  const focusToday = () => {
    const nextAnchor = utcDay(new Date());
    const nextStart = startOfScale(nextAnchor, scale);
    const nextEnd = endOfScale(nextStart, scale);
    const nextDays = Math.round((nextEnd.getTime() - nextStart.getTime()) / DAY_MS) + 1;
    const nextTodayIndex = Math.round((nextAnchor.getTime() - nextStart.getTime()) / DAY_MS);
    setAnchor(nextAnchor);
    if (todayFocusFrameRef.current !== null) window.cancelAnimationFrame(todayFocusFrameRef.current);
    todayFocusFrameRef.current = window.requestAnimationFrame(() => {
      scrollTodayIntoView(nextTodayIndex, cellWidth, nextDays);
      todayFocusFrameRef.current = null;
    });
  };

  useLayoutEffect(() => {
    if (hasInitiallyFocusedTodayRef.current) return;
    hasInitiallyFocusedTodayRef.current = true;
    scrollTodayIntoView(todayIndex, cellWidth, days);
  }, []);

  useEffect(() => () => {
    if (todayFocusFrameRef.current !== null) window.cancelAnimationFrame(todayFocusFrameRef.current);
  }, []);

  const syncHorizontalScroll = (source: HTMLDivElement, target: HTMLDivElement | null) => {
    const pendingLeft = pendingHorizontalScrollRef.current.get(source);
    pendingHorizontalScrollRef.current.delete(source);
    if (pendingLeft === source.scrollLeft) return;
    const left = source.scrollLeft;
    synchronizeHorizontalScroll(left, [target]);
    if (target && target.scrollLeft !== left) synchronizeHorizontalScroll(target.scrollLeft, [source]);
  };

  const barFor = (item: CloudProjectWorkItem) => {
    const rawStart = item.startAt ? utcDay(new Date(item.startAt)) : null;
    const rawEnd = item.dueAt ? utcDay(new Date(item.dueAt)) : null;
    if (!rawStart && !rawEnd) return null;
    const first = rawStart ?? rawEnd!;
    const last = rawEnd ?? rawStart!;
    if (last < start || first > end) return null;
    const clippedStart = first < start ? start : first;
    const clippedEnd = last > end ? end : last;
    const dayOffset = Math.round((clippedStart.getTime() - start.getTime()) / DAY_MS);
    const span = Math.max(1, Math.round((clippedEnd.getTime() - clippedStart.getTime()) / DAY_MS) + 1);
    const milestone = !(rawStart && rawEnd);
    return {
      left: dayOffset * cellWidth + (milestone ? cellWidth / 2 - 8 : 0),
      width: milestone ? 16 : span * cellWidth,
      milestone,
      startDate: clippedStart.toISOString().slice(0, 10),
      endDate: clippedEnd.toISOString().slice(0, 10),
    };
  };

  return <section className="work-item-board__gantt" data-plan-gantt style={{ "--gantt-width": `${timelineWidth}px`, "--gantt-cell": `${cellWidth}px`, "--gantt-days": days } as CSSProperties}>
    <header className="work-item-board__gantt-toolbar">
      <div className="work-item-board__gantt-navigation">
        <button type="button" aria-label="上一个时间范围" onClick={() => setAnchor((current) => moveAnchor(current, scale, -1))}><ChevronLeft size={17} /></button>
        <strong>{ganttTitle(start, scale)}</strong>
        <button type="button" aria-label="下一个时间范围" onClick={() => setAnchor((current) => moveAnchor(current, scale, 1))}><ChevronRight size={17} /></button>
      </div>
      <div className="work-item-board__gantt-actions">
        <button type="button" onClick={focusToday}>今天</button>
        <div role="group" aria-label="甘特图时间刻度">{(["week", "month", "quarter", "year"] as const).map((value) => <button type="button" key={value} aria-pressed={scale === value} onClick={() => setScale(value)}>{{ week: "周", month: "月", quarter: "季", year: "年" }[value]}</button>)}</div>
      </div>
    </header>
    <div ref={horizontalScrollbarRef} className="work-item-board__gantt-horizontal-scrollbar" aria-label="甘特图横向滚动条" onScroll={(event) => syncHorizontalScroll(event.currentTarget, scrollerRef.current)}>
      <span style={{ width: 286 + timelineWidth }} />
    </div>
    <div ref={scrollerRef} className="work-item-board__gantt-scroller" aria-label="甘特图时间轴" onScroll={(event) => syncHorizontalScroll(event.currentTarget, horizontalScrollbarRef.current)}>
      <div className="work-item-board__gantt-grid" style={{ width: `${286 + timelineWidth}px` }}>
        <div className="work-item-board__gantt-corner"><span className="work-item-board__index-cell">#</span><strong>标题</strong></div>
        <div className="work-item-board__gantt-dates" aria-hidden="true">{headerUnits.map((unit) => <span key={unit.key} data-weekend={unit.weekend || undefined} style={{ width: unit.days * cellWidth }}>{unit.label}<small>{unit.secondary}</small></span>)}</div>
        {items.map((item, index) => {
          const bar = barFor(item);
          const StatusIcon = WORK_ITEM_STATUS_ICONS[item.status];
          return <div className="work-item-board__gantt-row" key={item.workItemId}>
            <button type="button" className="work-item-board__gantt-name" onClick={(event) => onEdit(item, event.currentTarget)}><span>{index + 1}</span><strong>{item.title}</strong></button>
            <div className="work-item-board__gantt-track"><span className="work-item-board__gantt-track-grid" aria-hidden="true">{headerUnits.map((unit) => <i key={unit.key} data-weekend={unit.weekend || undefined} style={{ width: unit.days * cellWidth }} />)}</span>{bar ? <button type="button" className={`work-item-board__gantt-bar work-item-board__gantt-bar--${item.status}`} data-work-item-status={item.status} data-range-start={bar.startDate} data-range-end={bar.endDate} data-milestone={bar.milestone || undefined} style={{ left: bar.left, width: bar.width }} aria-label={`查看 ${item.title}`} onClick={(event) => onEdit(item, event.currentTarget)}><StatusIcon size={12} aria-hidden="true" />{bar.milestone ? null : <span>{item.title}</span>}</button> : <button type="button" className="work-item-board__gantt-unscheduled" onClick={(event) => onEdit(item, event.currentTarget)} aria-label={`查看未排期事项 ${item.title}`}>未排期</button>}</div>
          </div>;
        })}
        {todayIndex >= 0 && todayIndex < days ? <span className="work-item-board__gantt-today" aria-label="今天" style={{ left: 286 + todayIndex * cellWidth + cellWidth / 2 }} /> : null}
      </div>
    </div>
  </section>;
}
