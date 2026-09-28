import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type UIEvent } from "react";
import { ChevronRight } from "lucide-react";
import { AnchoredPopover } from "../../../components/AnchoredPopover";
import type { ProjectGateway } from "../data/projectGateway";
import type { CloudProjectWorkItem } from "../model/projectTypes";
import { WORK_ITEM_STATUS_ICONS } from "./workItemStatusVisual";

const DAY_MS = 24 * 60 * 60 * 1000;
const MAX_MONTHS = 7;

type CalendarFilters = { search: string; assigneeUserId: string; priority: string; labelId: string };
type MonthPage = { items: CloudProjectWorkItem[]; totalCount: number; undatedCount: number; loading: boolean; error: string };
type PendingTarget = { kind: "month"; month: string } | { kind: "day"; month: string; day: string };
type CalendarSegment = {
  item: CloudProjectWorkItem;
  row: number;
  column: number;
  span: number;
  lane: number;
  startDate: string;
  endDate: string;
  continuesBefore: boolean;
  continuesAfter: boolean;
  milestone: boolean;
};
const WORK_ITEM_STATUS_LABELS = { todo: "待开始", in_progress: "进行中", paused: "已暂停", done: "已完成" } as const;
const LUNAR_DAY_LABELS = ["初一", "初二", "初三", "初四", "初五", "初六", "初七", "初八", "初九", "初十", "十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八", "十九", "二十", "廿一", "廿二", "廿三", "廿四", "廿五", "廿六", "廿七", "廿八", "廿九", "三十"] as const;

function dateKey(value: Date) { return value.toISOString().slice(0, 10); }
function calendarDateKey(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return dateKey(date);
}
function monthKey(value: Date) { return value.toISOString().slice(0, 7); }
function monthDate(value: string) {
  const [year, month] = value.split("-").map(Number);
  return new Date(Date.UTC(year, month - 1, 1));
}
function addMonths(value: string, delta: number) {
  const base = monthDate(value);
  return monthKey(new Date(Date.UTC(base.getUTCFullYear(), base.getUTCMonth() + delta, 1)));
}
function monthRange(value: string) {
  const first = monthDate(value);
  const leading = (first.getUTCDay() + 6) % 7;
  const start = new Date(first.getTime() - leading * DAY_MS);
  const cells = Array.from({ length: 42 }, (_, index) => new Date(start.getTime() + index * DAY_MS));
  return { start: dateKey(cells[0]!), end: dateKey(cells[41]!), cells };
}
function lunarDay(value: Date) {
  try {
    const formatter = new Intl.DateTimeFormat("zh-CN-u-ca-chinese", { month: "short", day: "numeric", timeZone: "UTC" });
    const rawDay = formatter.formatToParts(value).find((part) => part.type === "day")?.value ?? "";
    const day = Number.parseInt(rawDay, 10);
    return LUNAR_DAY_LABELS[day - 1] ?? rawDay;
  } catch {
    return "";
  }
}
function monthLabel(value: string) {
  const date = monthDate(value);
  return `${date.getUTCFullYear()}年${date.getUTCMonth() + 1}月`;
}
function emptyPage(): MonthPage { return { items: [], totalCount: 0, undatedCount: 0, loading: true, error: "" }; }

function daysBetween(first: string, last: string) {
  return Math.round((new Date(`${last}T00:00:00.000Z`).getTime() - new Date(`${first}T00:00:00.000Z`).getTime()) / DAY_MS);
}

function calendarSegments(items: CloudProjectWorkItem[], month: string, gridStart: string) {
  const monthStart = `${month}-01`;
  const monthEnd = dateKey(new Date(monthDate(addMonths(month, 1)).getTime() - DAY_MS));
  const segments: CalendarSegment[] = [];
  for (const item of items) {
    const start = item.startAt ? calendarDateKey(item.startAt) : null;
    const due = item.dueAt ? calendarDateKey(item.dueAt) : null;
    if (!start && !due) continue;
    const firstDate = start ?? due!;
    const lastDate = due ?? start!;
    const itemStart = firstDate <= lastDate ? firstDate : lastDate;
    const itemEnd = firstDate <= lastDate ? lastDate : firstDate;
    const clippedStart = itemStart < monthStart ? monthStart : itemStart;
    const clippedEnd = itemEnd > monthEnd ? monthEnd : itemEnd;
    if (clippedEnd < clippedStart) continue;
    let cursor = clippedStart;
    while (cursor <= clippedEnd) {
      const offset = daysBetween(gridStart, cursor);
      const row = Math.floor(offset / 7);
      const column = offset % 7;
      const weekEnd = dateKey(new Date(new Date(`${cursor}T00:00:00.000Z`).getTime() + (6 - column) * DAY_MS));
      const endDate = clippedEnd < weekEnd ? clippedEnd : weekEnd;
      const span = daysBetween(cursor, endDate) + 1;
      segments.push({
        item,
        row,
        column,
        span,
        lane: 0,
        startDate: cursor,
        endDate,
        continuesBefore: cursor !== itemStart,
        continuesAfter: endDate !== itemEnd,
        milestone: !(start && due),
      });
      cursor = dateKey(new Date(new Date(`${endDate}T00:00:00.000Z`).getTime() + DAY_MS));
    }
  }
  segments.sort((left, right) => left.row - right.row || left.column - right.column || right.span - left.span || left.item.workItemId.localeCompare(right.item.workItemId));
  const laneEnds = new Map<number, number[]>();
  for (const segment of segments) {
    const rowLanes = laneEnds.get(segment.row) ?? [];
    let lane = rowLanes.findIndex((endColumn) => endColumn < segment.column);
    if (lane < 0) lane = rowLanes.length;
    rowLanes[lane] = segment.column + segment.span - 1;
    laneEnds.set(segment.row, rowLanes);
    segment.lane = lane;
  }
  return segments;
}
function centeredDayScrollTop(scroller: HTMLElement, day: HTMLElement) {
  const scrollerRect = scroller.getBoundingClientRect();
  const dayRect = day.getBoundingClientRect();
  return Math.max(0, scroller.scrollTop + dayRect.top - scrollerRect.top + dayRect.height / 2 - scroller.clientHeight / 2);
}

export function ContinuousWorkItemCalendar({
  projectScopeKey,
  projectId,
  projectName,
  filters,
  gateway,
  refreshRevision,
  onEdit,
  onCreate,
  readOnly,
}: {
  projectScopeKey: string;
  projectId: string;
  projectName: string;
  filters: CalendarFilters;
  gateway: ProjectGateway;
  refreshRevision: number;
  onEdit: (item: CloudProjectWorkItem, trigger?: HTMLButtonElement) => void;
  onCreate: (date: string, trigger: HTMLButtonElement) => void;
  readOnly: boolean;
}) {
  const today = dateKey(new Date());
  const todayMonth = today.slice(0, 7);
  const [months, setMonths] = useState(() => [addMonths(todayMonth, -1), todayMonth, addMonths(todayMonth, 1)]);
  const [visibleMonth, setVisibleMonth] = useState(todayMonth);
  const [pages, setPages] = useState<Record<string, MonthPage>>({});
  const [expandedDay, setExpandedDay] = useState<{ day: string; items: CloudProjectWorkItem[] } | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const moreAnchorRef = useRef<HTMLButtonElement>(null);
  const monthRefs = useRef(new Map<string, HTMLElement>());
  const dayRefs = useRef(new Map<string, HTMLElement>());
  const generationRef = useRef(0);
  const monthRequestRef = useRef(new Map<string, number>());
  const requestSequenceRef = useRef(0);
  const extendLockRef = useRef(false);
  const pendingAdjustmentRef = useRef<{ kind: "prepend" } | { kind: "drop-first"; height: number } | null>(null);
  const pendingTargetRef = useRef<PendingTarget | null>({ kind: "day", month: todayMonth, day: today });
  const projectIdentityRef = useRef(`${projectScopeKey}\u0000${projectId}`);
  const filterIdentity = JSON.stringify(filters);

  const loadMonth = useCallback(async (month: string, force = false) => {
    const generation = generationRef.current;
    if (!force && pages[month] && !pages[month].error) return;
    const request = ++requestSequenceRef.current;
    monthRequestRef.current.set(month, request);
    setPages((current) => ({ ...current, [month]: { ...(current[month] ?? emptyPage()), loading: true, error: "" } }));
    const range = monthRange(month);
    try {
      const items: CloudProjectWorkItem[] = [];
      let cursor: string | undefined;
      let totalCount = 0;
      let undatedCount = 0;
      do {
        const page = await gateway.listCloudProjectWorkItems({
          projectId,
          pageSize: 100,
          status: "all",
          scheduledFrom: range.start,
          scheduledTo: range.end,
          ...(cursor ? { cursor } : {}),
          ...(filters.search ? { search: filters.search } : {}),
          ...(filters.assigneeUserId ? { assigneeUserId: filters.assigneeUserId } : {}),
          ...(filters.priority ? { priority: filters.priority as "none" | "low" | "medium" | "high" | "urgent" } : {}),
          ...(filters.labelId ? { labelId: filters.labelId } : {}),
        });
        if (generationRef.current !== generation || monthRequestRef.current.get(month) !== request || page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return;
        items.push(...page.items);
        totalCount = page.totalCount ?? items.length;
        undatedCount = page.undatedCount ?? 0;
        cursor = page.nextCursor ?? undefined;
      } while (cursor);
      if (generationRef.current !== generation || monthRequestRef.current.get(month) !== request) return;
      const seen = new Set<string>();
      const uniqueItems = items.filter((item) => !seen.has(item.workItemId) && Boolean(seen.add(item.workItemId)));
      setPages((current) => ({ ...current, [month]: { items: uniqueItems, totalCount, undatedCount, loading: false, error: "" } }));
    } catch {
      if (generationRef.current !== generation || monthRequestRef.current.get(month) !== request) return;
      setPages((current) => ({ ...current, [month]: { ...(current[month] ?? emptyPage()), loading: false, error: "月历加载失败，请重试。" } }));
    }
  }, [filterIdentity, filters, gateway, pages, projectId, projectScopeKey]);

  useEffect(() => {
    generationRef.current += 1;
    monthRequestRef.current.clear();
    setPages({});
    setExpandedDay(null);
    const projectIdentity = `${projectScopeKey}\u0000${projectId}`;
    if (projectIdentityRef.current !== projectIdentity) {
      projectIdentityRef.current = projectIdentity;
      const currentDay = dateKey(new Date());
      const currentMonth = currentDay.slice(0, 7);
      setVisibleMonth(currentMonth);
      pendingTargetRef.current = { kind: "day", month: currentMonth, day: currentDay };
      setMonths([addMonths(currentMonth, -1), currentMonth, addMonths(currentMonth, 1)]);
    }
  }, [filterIdentity, projectId, projectScopeKey, refreshRevision]);

  useEffect(() => {
    for (const month of months) {
      if (!pages[month]) void loadMonth(month);
    }
  }, [loadMonth, months, pages]);

  useLayoutEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    const adjustment = pendingAdjustmentRef.current;
    if (adjustment?.kind === "prepend") {
      const first = monthRefs.current.get(months[0]!);
      if (first) scroller.scrollTop += first.offsetHeight;
    } else if (adjustment?.kind === "drop-first") {
      scroller.scrollTop = Math.max(0, scroller.scrollTop - adjustment.height);
    }
    pendingAdjustmentRef.current = null;
    const target = pendingTargetRef.current;
    if (target?.kind === "month") {
      const node = monthRefs.current.get(target.month);
      if (node) {
        scroller.scrollTop = Math.max(0, node.offsetTop - 52);
        pendingTargetRef.current = null;
      }
    } else if (target?.kind === "day") {
      const node = dayRefs.current.get(`${target.month}:${target.day}`);
      if (node) {
        scroller.scrollTop = centeredDayScrollTop(scroller, node);
        pendingTargetRef.current = null;
      }
    }
    requestAnimationFrame(() => { extendLockRef.current = false; });
  }, [months]);

  const extend = useCallback((direction: -1 | 1) => {
    if (extendLockRef.current) return;
    extendLockRef.current = true;
    setMonths((current) => {
      if (direction < 0) {
        pendingAdjustmentRef.current = { kind: "prepend" };
        const next = [addMonths(current[0]!, -1), ...current];
        if (next.length > MAX_MONTHS) next.pop();
        return next;
      }
      const firstHeight = monthRefs.current.get(current[0]!)?.offsetHeight ?? 0;
      const next = [...current, addMonths(current[current.length - 1]!, 1)];
      if (next.length > MAX_MONTHS) {
        next.shift();
        pendingAdjustmentRef.current = { kind: "drop-first", height: firstHeight };
      }
      return next;
    });
  }, []);

  const updateVisibleMonth = useCallback(() => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    const marker = scroller.scrollTop + 96;
    let closest = months[0]!;
    for (const month of months) {
      const node = monthRefs.current.get(month);
      if (node && node.offsetTop <= marker) closest = month;
    }
    setVisibleMonth(closest);
  }, [months]);

  const onScroll = (event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget;
    updateVisibleMonth();
    if (target.scrollTop < 80) extend(-1);
    else if (target.scrollHeight - target.clientHeight - target.scrollTop < 80) extend(1);
  };

  const scrollToMonth = (target: string) => {
    const node = monthRefs.current.get(target);
    if (node && scrollRef.current) {
      const top = Math.max(0, node.offsetTop - 52);
      if (typeof scrollRef.current.scrollTo === "function") scrollRef.current.scrollTo({ top, behavior: "smooth" });
      else scrollRef.current.scrollTop = top;
      setVisibleMonth(target);
      return;
    }
    pendingTargetRef.current = { kind: "month", month: target };
    setVisibleMonth(target);
    setMonths([addMonths(target, -1), target, addMonths(target, 1)]);
  };

  const positionToday = useCallback((behavior: ScrollBehavior) => {
    const currentDay = dateKey(new Date());
    const currentMonth = currentDay.slice(0, 7);
    const node = dayRefs.current.get(`${currentMonth}:${currentDay}`);
    const scroller = scrollRef.current;
    setVisibleMonth(currentMonth);
    if (node && scroller) {
      const top = centeredDayScrollTop(scroller, node);
      if (typeof scroller.scrollTo === "function") scroller.scrollTo({ top, behavior });
      else scroller.scrollTop = top;
      return;
    }
    pendingTargetRef.current = { kind: "day", month: currentMonth, day: currentDay };
    setMonths([addMonths(currentMonth, -1), currentMonth, addMonths(currentMonth, 1)]);
  }, []);

  useEffect(() => {
    // The calendar mounts conditionally, so its scroll viewport may not have its final size during the layout pass.
    const frame = requestAnimationFrame(() => positionToday("instant"));
    return () => cancelAnimationFrame(frame);
  }, [positionToday]);

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "PageUp" && event.key !== "PageDown") return;
    event.preventDefault();
    scrollToMonth(addMonths(visibleMonth, event.key === "PageUp" ? -1 : 1));
  };

  const visiblePage = pages[visibleMonth];
  return <section className="work-item-board__calendar" data-plan-calendar aria-label={`${projectName} ${monthLabel(visibleMonth)}月历`}>
    <header className="work-item-board__calendar-header">
      <h2>{monthLabel(visibleMonth)}</h2>
      <div>
        <button type="button" aria-label="上个月" onClick={() => scrollToMonth(addMonths(visibleMonth, -1))}>‹</button>
        <button type="button" onClick={() => positionToday("smooth")}>今天</button>
        <button type="button" aria-label="下个月" onClick={() => scrollToMonth(addMonths(visibleMonth, 1))}>›</button>
        <span>无日期记录：{visiblePage?.undatedCount ?? 0}</span>
      </div>
    </header>
    <div className="work-item-board__calendar-weekdays" aria-hidden="true">{["周一", "周二", "周三", "周四", "周五", "周六", "周日"].map((day) => <span key={day}>{day}</span>)}</div>
    <div ref={scrollRef} className="work-item-board__calendar-scroll" tabIndex={0} onScroll={onScroll} onKeyDown={handleKeyDown} aria-label="连续月份月历，使用滚轮或 PageUp 和 PageDown 浏览">
      <div className="work-item-board__calendar-sentinel" data-calendar-sentinel="previous" />
      {months.map((month) => {
        const page = pages[month] ?? emptyPage();
        const range = monthRange(month);
        const segments = calendarSegments(page.items, month, range.start);
        return <section ref={(node) => { if (node) monthRefs.current.set(month, node); else monthRefs.current.delete(month); }} className="work-item-board__calendar-month" data-calendar-month={month} key={month} aria-label={monthLabel(month)}>
          <h3>{monthLabel(month)}</h3>
          {page.loading ? <progress aria-label={`正在加载${monthLabel(month)}`} /> : null}
          {page.error ? <div className="work-item-board__calendar-error" role="alert"><span>{page.error}</span><button type="button" onClick={() => void loadMonth(month, true)}>重试</button></div> : null}
          <div className="work-item-board__calendar-grid">{range.cells.map((cell, cellIndex) => {
            const day = dateKey(cell);
            const outside = monthKey(cell) !== month;
            const isToday = day === dateKey(new Date());
            const lunar = lunarDay(cell);
            const row = Math.floor(cellIndex / 7);
            const column = cellIndex % 7;
            const daySegments = segments.filter((segment) => segment.row === row && segment.column <= column && segment.column + segment.span > column);
            const dayItems = [...new Map(daySegments.map((segment) => [segment.item.workItemId, segment.item])).values()];
            const hiddenCount = new Set(daySegments.filter((segment) => segment.lane >= 2).map((segment) => segment.item.workItemId)).size;
            return <div ref={(node) => { const key = `${month}:${day}`; if (node) dayRefs.current.set(key, node); else dayRefs.current.delete(key); }} className="work-item-board__calendar-day" key={day} data-calendar-day={day} data-outside={outside || undefined} data-today={isToday || undefined}>
              <button type="button" className="work-item-board__calendar-create" aria-label={`在${day}创建事项${lunar ? `，农历${lunar}` : ""}`} disabled={readOnly} onClick={(event) => onCreate(day, event.currentTarget)}><span className="work-item-board__calendar-date"><time dateTime={day}>{Number(day.slice(8))}</time><small>{lunar}</small></span></button>
              {hiddenCount > 0 ? <button type="button" className="work-item-board__calendar-more-trigger" aria-label={`${day}还有${hiddenCount}项，展开全部`} onClick={(event) => { moreAnchorRef.current = event.currentTarget; setExpandedDay({ day, items: dayItems }); }}><span>还有 {hiddenCount} 项</span><ChevronRight size={11} aria-hidden="true" /></button> : null}
            </div>;
          })}<div className="work-item-board__calendar-ranges">{segments.filter((segment) => segment.lane < 2).map((segment) => {
            const StatusIcon = WORK_ITEM_STATUS_ICONS[segment.item.status];
            const dateLabel = segment.startDate === segment.endDate ? segment.startDate : `${segment.startDate}至${segment.endDate}`;
            return <button
              type="button"
              className={`work-item-board__calendar-range work-item-board__calendar-range--${segment.item.status}`}
              key={`${segment.item.workItemId}:${segment.startDate}:${segment.endDate}`}
              data-work-item-status={segment.item.status}
              data-range-start={segment.startDate}
              data-range-end={segment.endDate}
              data-continues-before={segment.continuesBefore || undefined}
              data-continues-after={segment.continuesAfter || undefined}
              data-milestone={segment.milestone || undefined}
              style={{ gridColumn: `${segment.column + 1} / span ${segment.span}`, gridRow: String(segment.row + 1), "--calendar-lane-offset": `${38 + segment.lane * 26}px` } as CSSProperties}
              aria-label={`查看 ${segment.item.title}，${dateLabel}`}
              onClick={(event) => onEdit(segment.item, event.currentTarget)}
            ><StatusIcon size={12} aria-hidden="true" /><span>{segment.item.title}</span></button>;
          })}</div></div>
        </section>;
      })}
      <div className="work-item-board__calendar-sentinel" data-calendar-sentinel="next" />
    </div>
    <AnchoredPopover open={Boolean(expandedDay)} anchorRef={moreAnchorRef} onClose={() => setExpandedDay(null)} className="work-item-board__calendar-more-popover" contentKey={expandedDay?.day}>
      {expandedDay ? <div role="dialog" aria-label={`${expandedDay.day}全部事项`}>
        <header><strong>{expandedDay.day}</strong><span>{expandedDay.items.length} 项</span></header>
        <ul>{expandedDay.items.map((item) => <li key={item.workItemId}><button type="button" aria-label={`查看 ${item.title}`} onClick={(event) => { setExpandedDay(null); onEdit(item, event.currentTarget); }}><span>{WORK_ITEM_STATUS_LABELS[item.status]}</span><strong>{item.title}</strong></button></li>)}</ul>
      </div> : null}
    </AnchoredPopover>
  </section>;
}
