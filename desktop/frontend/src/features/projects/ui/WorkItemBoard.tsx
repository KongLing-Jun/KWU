import { useCallback, useEffect, useId, useMemo, useRef, useState, type FormEvent } from "react";
import { CalendarDays, ChartGantt, Filter, GripVertical, LayoutDashboard, List, MoreHorizontal, Plus, Search, Settings2, Table2, UserRound, X } from "lucide-react";
import { clientErrorCode } from "../../../lib/clientError";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import type { ProjectGateway } from "../data/projectGateway";
import { projectGateway } from "../data/projectGateway";
import { formatProjectDateTime } from "../model/projectDateTime";
import type { CloudProjectActivity, CloudProjectAsset, CloudProjectLabel, CloudProjectMember, CloudProjectWorkItem, CloudProjectWorkItemResult, CloudProjectWorkItemStatus } from "../model/projectTypes";
import { ProjectSelect } from "../../../components/ProjectSelect";
import { WorkItemGanttProjection, WorkItemListProjection, WorkItemTableProjection } from "./WorkItemProjections";
import { ContinuousWorkItemCalendar } from "./ContinuousWorkItemCalendar";
import { WorkItemEditorDialog, WORK_ITEM_PRIORITY_OPTIONS, workItemPriorityLabel, type WorkItemEditorState, type WorkItemPriority } from "./WorkItemEditorDialog";
import { WorkItemDetailDialog } from "./WorkItemDetailDialog";
import { ProjectModalSurface } from "./ProjectModalSurface";
import { WORK_ITEM_STATUS_ICONS } from "./workItemStatusVisual";

const STATUS_ORDER: CloudProjectWorkItemStatus[] = ["todo", "in_progress", "paused", "done"];
const MOVE_TARGETS: Record<CloudProjectWorkItemStatus, CloudProjectWorkItemStatus[]> = {
  todo: STATUS_ORDER,
  in_progress: STATUS_ORDER,
  paused: STATUS_ORDER,
  done: STATUS_ORDER,
};

type PlanDisplayMode = "table" | "board" | "list" | "gantt" | "calendar";
type Priority = WorkItemPriority;
type ColumnState = { items: CloudProjectWorkItem[]; nextCursor: string | null; totalCount: number; loading: boolean };
type PlanFilters = { search: string; assigneeUserId: string; priority: "" | Priority; labelId: string };

const EMPTY_FILTERS: PlanFilters = { search: "", assigneeUserId: "", priority: "", labelId: "" };
const PLAN_VIEW_OPTIONS: Array<{ mode: PlanDisplayMode; label: string; Icon: typeof LayoutDashboard }> = [
  { mode: "board", label: "看板", Icon: LayoutDashboard },
  { mode: "table", label: "表格", Icon: Table2 },
  { mode: "list", label: "列表", Icon: List },
  { mode: "gantt", label: "甘特", Icon: ChartGantt },
  { mode: "calendar", label: "日历", Icon: CalendarDays },
];
function emptyColumns(): Record<CloudProjectWorkItemStatus, ColumnState> {
  return {
    todo: { items: [], nextCursor: null, totalCount: 0, loading: false },
    in_progress: { items: [], nextCursor: null, totalCount: 0, loading: false },
    paused: { items: [], nextCursor: null, totalCount: 0, loading: false },
    done: { items: [], nextCursor: null, totalCount: 0, loading: false },
  };
}

interface WorkItemBoardProps {
  accountScopeKey: string;
  projectScopeKey: string;
  projectId: string;
  projectName: string;
  archived: boolean;
  stale: boolean;
  gateway?: ProjectGateway;
}

interface MoveIntent { workItemId: string; targetStatus: CloudProjectWorkItemStatus; anchor: string }
function editorDraft(editor: WorkItemEditorState): NonNullable<WorkItemEditorState["persistedDraft"]> {
  return {
    title: editor.title.trim(),
    description: editor.description,
    assigneeUserId: editor.assigneeUserId,
    priority: editor.priority,
    startAt: editor.startAt,
    dueAt: editor.dueAt,
  };
}

function editorForCreate(status: CloudProjectWorkItemStatus): WorkItemEditorState {
  return { kind: "create", workItemId: "", title: "", description: "", status, assigneeUserId: "", priority: "none", startAt: "", dueAt: "", labelIds: [], assetIds: [] };
}

export function WorkItemBoard({ accountScopeKey, projectScopeKey, projectId, projectName, archived, stale, gateway = projectGateway }: WorkItemBoardProps) {
  const t = useT();
  const { showError } = useToast();
  const requestRef = useRef(0);
  const detailActivityRequestRef = useRef(0);
  const editorTitleRef = useRef<HTMLInputElement>(null);
  const dialogTriggerRef = useRef<HTMLButtonElement | null>(null);
  const dragItemRef = useRef<string>("");
  const filterMenuRef = useRef<HTMLSpanElement>(null);
  const filterTriggerRef = useRef<HTMLButtonElement>(null);
  const searchComposingRef = useRef(false);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const editorDateErrorId = useId();
  const [columns, setColumns] = useState(emptyColumns);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [moveIntent, setMoveIntent] = useState<MoveIntent | null>(null);
  const [editor, setEditor] = useState<WorkItemEditorState | null>(null);
  const [detailItem, setDetailItem] = useState<CloudProjectWorkItem | null>(null);
  const [detailActivities, setDetailActivities] = useState<CloudProjectActivity[]>([]);
  const [detailActivityLoading, setDetailActivityLoading] = useState(false);
  const [members, setMembers] = useState<CloudProjectMember[]>([]);
  const [labels, setLabels] = useState<CloudProjectLabel[]>([]);
  const [assets, setAssets] = useState<CloudProjectAsset[]>([]);
  const [labelSettingsOpen, setLabelSettingsOpen] = useState(false);
  const [labelName, setLabelName] = useState("");
  const [labelColor, setLabelColor] = useState("#3366FF");
  const [labelDrafts, setLabelDrafts] = useState<Record<string, { name: string; color: string }>>({});
  const [displayMode, setDisplayMode] = useState<PlanDisplayMode>("board");
  const [calendarRefreshRevision, setCalendarRefreshRevision] = useState(0);
  const [searchDraft, setSearchDraft] = useState("");
  const [filters, setFilters] = useState<PlanFilters>(EMPTY_FILTERS);
  const [filterMenuOpen, setFilterMenuOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);

  const presentError = useCallback((reason: unknown, message: string) => {
    setError(message);
    showError("projects.operation", reason, message);
  }, [showError]);

  const readOnly = archived || stale;
  const items = useMemo(() => STATUS_ORDER.flatMap((status) => columns[status].items), [columns]);
  const totalCount = STATUS_ORDER.reduce((sum, status) => sum + columns[status].totalCount, 0);
  const hasMoreItems = STATUS_ORDER.some((status) => Boolean(columns[status].nextCursor));
  const activeFilterCount = Number(Boolean(filters.search)) + Number(Boolean(filters.assigneeUserId)) + Number(Boolean(filters.priority)) + Number(Boolean(filters.labelId));
  const editorDateInvalid = Boolean(editor?.startAt && editor?.dueAt && editor.dueAt < editor.startAt);
  const movingItem = moveIntent ? items.find((item) => item.workItemId === moveIntent.workItemId) : undefined;
  const loadColumns = useCallback(async (nextFilters: PlanFilters) => {
    const request = requestRef.current + 1;
    requestRef.current = request;
    setLoading(true);
    setColumns((current) => Object.fromEntries(STATUS_ORDER.map((status) => [status, { ...current[status], loading: true }])) as Record<CloudProjectWorkItemStatus, ColumnState>);
    try {
      const pages = await Promise.all(STATUS_ORDER.map((status) => gateway.listCloudProjectWorkItems({
        projectId,
        pageSize: 50,
        status,
        ...(nextFilters.search ? { search: nextFilters.search } : {}),
        ...(nextFilters.assigneeUserId ? { assigneeUserId: nextFilters.assigneeUserId } : {}),
        ...(nextFilters.priority ? { priority: nextFilters.priority } : {}),
        ...(nextFilters.labelId ? { labelId: nextFilters.labelId } : {}),
      })));
      if (requestRef.current !== request || pages.some((page) => page.projectScopeKey !== projectScopeKey || page.projectId !== projectId)) return false;
      setColumns(Object.fromEntries(STATUS_ORDER.map((status, index) => [status, { items: pages[index].items.filter((item) => item.status === status), nextCursor: pages[index].nextCursor, totalCount: pages[index].totalCount ?? pages[index].items.filter((item) => item.status === status).length, loading: false }])) as Record<CloudProjectWorkItemStatus, ColumnState>);
      setError("");
      return true;
    } catch (reason) {
      if (requestRef.current === request) {
        setColumns((current) => Object.fromEntries(STATUS_ORDER.map((status) => [status, { ...current[status], loading: false }])) as Record<CloudProjectWorkItemStatus, ColumnState>);
        presentError(reason, t("workspace.cloudProjects.workItems.loadFailed"));
      }
      return false;
    } finally {
      if (requestRef.current === request) setLoading(false);
    }
  }, [gateway, presentError, projectId, projectScopeKey, t]);

  const loadMore = async (status: CloudProjectWorkItemStatus) => {
    const column = columns[status];
    if (!column.nextCursor || column.loading) return;
    const request = requestRef.current;
    const cursor = column.nextCursor;
    const requestFilters = { ...filters };
    setColumns((current) => ({ ...current, [status]: { ...current[status], loading: true } }));
    try {
      const page = await gateway.listCloudProjectWorkItems({ projectId, pageSize: 50, status, cursor, ...(requestFilters.search ? { search: requestFilters.search } : {}), ...(requestFilters.assigneeUserId ? { assigneeUserId: requestFilters.assigneeUserId } : {}), ...(requestFilters.priority ? { priority: requestFilters.priority } : {}), ...(requestFilters.labelId ? { labelId: requestFilters.labelId } : {}) });
      if (requestRef.current !== request || page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return;
      setColumns((current) => {
        if (requestRef.current !== request || current[status].nextCursor !== cursor) return current;
        const seen = new Set(current[status].items.map((item) => item.workItemId));
        const appended = page.items.filter((item) => item.status === status && !seen.has(item.workItemId));
        return { ...current, [status]: { items: [...current[status].items, ...appended], nextCursor: page.nextCursor, totalCount: page.totalCount ?? current[status].totalCount, loading: false } };
      });
    } catch (reason) {
      if (requestRef.current === request) {
        setColumns((current) => current[status].nextCursor === cursor ? ({ ...current, [status]: { ...current[status], loading: false } }) : current);
        presentError(reason, t("workspace.cloudProjects.workItems.loadFailed"));
      }
    }
  };

  const loadRemainingProjectionItems = async () => {
    await Promise.all(STATUS_ORDER.filter((status) => columns[status].nextCursor && !columns[status].loading).map((status) => loadMore(status)));
  };

  useEffect(() => {
    requestRef.current += 1;
    setColumns(emptyColumns());
    setError("");
    setNotice("");
    setEditor(null);
    setDetailItem(null);
    setDetailActivities([]);
    setDetailActivityLoading(false);
    detailActivityRequestRef.current += 1;
    setMoveIntent(null);
    setFilters(EMPTY_FILTERS);
    setSearchDraft("");
    setCalendarRefreshRevision(0);
    setDisplayMode("board");
    setFilterMenuOpen(false);
    setSearchOpen(false);
    setLabels([]);
    setAssets([]);
    if (stale) { setLoading(false); return () => { requestRef.current += 1; }; }
    void loadColumns(EMPTY_FILTERS);
    if (typeof gateway.listCloudProjectMembers === "function") void gateway.listCloudProjectMembers(projectId).then((page) => {
      if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) setMembers(page.items.filter((member) => member.status === "active"));
    }).catch(() => setMembers([]));
    if (gateway.listCloudProjectLabels) void gateway.listCloudProjectLabels(projectId).then((page) => {
      if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) setLabels(page.items);
    }).catch(() => setLabels([]));
    if (gateway.listCloudProjectAssets) void gateway.listCloudProjectAssets(projectId).then((page) => {
      if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) setAssets(page.items);
    }).catch(() => setAssets([]));
    return () => { requestRef.current += 1; };
  }, [accountScopeKey, gateway, loadColumns, projectId, projectScopeKey, stale]);

  useEffect(() => {
    setLabelDrafts(Object.fromEntries(labels.map((label) => [label.labelId, { name: label.name, color: label.color }])));
  }, [labels]);

  useEffect(() => {
    if (searchOpen) searchInputRef.current?.focus();
  }, [searchOpen]);

  useEffect(() => {
    if (!filterMenuOpen) return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      const target = event.target;
      if (target instanceof Node && !filterMenuRef.current?.contains(target)) setFilterMenuOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || document.querySelector('[data-anchored-popover="active"][data-state="open"]')) return;
      setFilterMenuOpen(false);
      requestAnimationFrame(() => filterTriggerRef.current?.focus());
    };
    document.addEventListener("click", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("click", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [filterMenuOpen]);

  useEffect(() => {
    if (!editor && !detailItem && !moveIntent && !labelSettingsOpen) return;
    const onEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      if (document.querySelector('[data-anchored-popover="active"][data-state="open"]')) return;
      setEditor(null);
      setDetailItem(null);
      setMoveIntent(null);
      setLabelSettingsOpen(false);
      requestAnimationFrame(() => dialogTriggerRef.current?.focus());
    };
    window.addEventListener("keydown", onEscape);
    return () => window.removeEventListener("keydown", onEscape);
  }, [detailItem, editor, labelSettingsOpen, moveIntent]);

  const openCreate = (status: CloudProjectWorkItemStatus, trigger: HTMLButtonElement) => {
    dialogTriggerRef.current = trigger;
    setEditor(editorForCreate(status));
    setMoveIntent(null);
    setError("");
    setNotice("");
  };

  const openCreateForDate = (date: string, trigger: HTMLButtonElement) => {
    dialogTriggerRef.current = trigger;
    setEditor({ ...editorForCreate("todo"), startAt: date, dueAt: date });
    setMoveIntent(null);
    setError("");
    setNotice("");
  };

  const beginEdit = (item: CloudProjectWorkItem, trigger?: HTMLButtonElement) => {
    if (trigger) dialogTriggerRef.current = trigger;
    setEditor({ kind: "edit", workItemId: item.workItemId, title: item.title, description: item.description || "", status: item.status, assigneeUserId: item.assigneeUserId || "", priority: item.priority || "none", startAt: item.startAt?.slice(0, 10) || "", dueAt: item.dueAt?.slice(0, 10) || "", labelIds: item.labels.map((label) => label.labelId), assetIds: assets.filter((asset) => asset.workItemIds.includes(item.workItemId)).map((asset) => asset.assetId), persisted: item });
    setDetailItem(null);
    setMoveIntent(null);
    setError("");
  };

  const openDetail = (item: CloudProjectWorkItem, trigger?: HTMLButtonElement) => {
    if (trigger) dialogTriggerRef.current = trigger;
    setDetailItem(item);
    setDetailActivities([]);
    setDetailActivityLoading(true);
    setEditor(null);
    setMoveIntent(null);
    setError("");
    const request = ++detailActivityRequestRef.current;
    void (async () => {
      const activities: CloudProjectActivity[] = [];
      let cursor = "";
      do {
        const page = await gateway.listCloudProjectActivities({ projectId, pageSize: 100, filter: "all_members", ...(cursor ? { cursor } : {}) });
        if (detailActivityRequestRef.current !== request || page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return;
        activities.push(...page.items.filter((activity) => String(activity.eventPayload?.work_item_id ?? "") === item.workItemId));
        cursor = page.nextCursor || "";
      } while (cursor);
      if (detailActivityRequestRef.current === request) setDetailActivities(activities);
    })().catch(() => {
      if (detailActivityRequestRef.current === request) setDetailActivities([]);
    }).finally(() => {
      if (detailActivityRequestRef.current === request) setDetailActivityLoading(false);
    });
  };

  const beginMove = (item: CloudProjectWorkItem, trigger?: HTMLButtonElement) => {
    if (trigger) dialogTriggerRef.current = trigger;
    setMoveIntent({ workItemId: item.workItemId, targetStatus: item.status, anchor: "append" });
    setEditor(null);
    setError("");
  };

  const refreshAfterMutation = async () => {
    const refreshed = await loadColumns(filters);
    if (displayMode === "calendar") setCalendarRefreshRevision((current) => current + 1);
    return refreshed;
  };

  const submitEditor = async () => {
    if (!editor || readOnly || !editor.title.trim() || editorDateInvalid) return;
    const current = items.find((item) => item.workItemId === editor.workItemId) ?? editor.persisted;
    setPending(true); setError(""); setNotice("");
    try {
      let saved: CloudProjectWorkItemResult | undefined;
      if (editor.kind === "create") {
        saved = await gateway.createCloudProjectWorkItem({ projectId, title: editor.title.trim(), descriptionSet: true, description: editor.description || null, status: editor.status, ...(editor.assigneeUserId ? { assigneeUserId: editor.assigneeUserId } : {}), priority: editor.priority, ...(editor.startAt ? { startAt: new Date(`${editor.startAt}T00:00:00.000Z`).toISOString() } : {}), ...(editor.dueAt ? { dueAt: new Date(`${editor.dueAt}T23:59:59.999Z`).toISOString() } : {}) });
        const persisted = saved.workItem;
        const persistedDraft = editorDraft(editor);
        setEditor((draft) => draft ? { ...draft, kind: "edit", workItemId: persisted.workItemId, status: persisted.status, persisted, persistedDraft } : draft);
        setColumns((currentColumns) => ({ ...currentColumns, [persisted.status]: { ...currentColumns[persisted.status], items: [persisted, ...currentColumns[persisted.status].items.filter((item) => item.workItemId !== persisted.workItemId)], totalCount: Math.max(currentColumns[persisted.status].totalCount, currentColumns[persisted.status].items.length + 1) } }));
      } else if (current) {
        const draft = editorDraft(editor);
        const baseline = editor.persistedDraft;
        const unchanged = baseline
          ? Object.entries(draft).every(([key, value]) => baseline[key as keyof typeof baseline] === value)
          : editor.title.trim() === current.title && editor.description === (current.description || "") && editor.assigneeUserId === (current.assigneeUserId || "") && editor.priority === (current.priority || "none") && editor.startAt === (current.startAt?.slice(0, 10) || "") && editor.dueAt === (current.dueAt?.slice(0, 10) || "");
        saved = unchanged
          ? { projectScopeKey, projectId, workItem: current }
          : await gateway.patchCloudProjectWorkItem({ projectId, workItemId: current.workItemId, expectedVersion: current.version, title: editor.title.trim(), descriptionSet: true, description: editor.description || null, assigneeSet: true, assigneeUserId: editor.assigneeUserId || null, priority: editor.priority, startAtSet: true, startAt: editor.startAt ? new Date(`${editor.startAt}T00:00:00.000Z`).toISOString() : null, dueAtSet: true, dueAt: editor.dueAt ? new Date(`${editor.dueAt}T23:59:59.999Z`).toISOString() : null });
        if (!unchanged) setEditor((state) => state ? { ...state, persistedDraft: draft } : state);
      }
      if (saved && saved.workItem.status !== editor.status) {
        saved = await gateway.moveCloudProjectWorkItem({ projectId, workItemId: saved.workItem.workItemId, expectedVersion: saved.workItem.version, targetStatus: editor.status });
      }
      if (saved && gateway.putCloudProjectWorkItemLabel && gateway.deleteCloudProjectWorkItemLabel) {
        let readback = saved.workItem;
        const currentIds = new Set(readback.labels.map((label) => label.labelId));
        const targetIds = new Set(editor.labelIds);
        for (const labelId of currentIds) if (!targetIds.has(labelId)) readback = (await gateway.deleteCloudProjectWorkItemLabel({ projectId, workItemId: readback.workItemId, labelId, expectedVersion: readback.version })).workItem;
        for (const labelId of targetIds) if (!currentIds.has(labelId)) readback = (await gateway.putCloudProjectWorkItemLabel({ projectId, workItemId: readback.workItemId, labelId, expectedVersion: readback.version })).workItem;
        saved = { ...saved, workItem: readback };
        setEditor((draft) => draft ? { ...draft, kind: "edit", workItemId: readback.workItemId, persisted: readback } : draft);
      }
      if (saved && gateway.listCloudProjectAssets && gateway.putCloudProjectWorkItemAsset && gateway.deleteCloudProjectWorkItemAsset) {
        const workItemId = saved.workItem.workItemId;
        const currentIds = new Set(assets.filter((asset) => asset.workItemIds.includes(workItemId)).map((asset) => asset.assetId));
        const targetIds = new Set(editor.assetIds);
        for (const assetId of currentIds) if (!targetIds.has(assetId)) await gateway.deleteCloudProjectWorkItemAsset({ projectId, workItemId, assetId });
        for (const assetId of targetIds) if (!currentIds.has(assetId)) await gateway.putCloudProjectWorkItemAsset({ projectId, workItemId, assetId });
        const page = await gateway.listCloudProjectAssets(projectId);
        if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) setAssets(page.items);
      }
      if (await refreshAfterMutation()) { setEditor(null); setNotice(t("workspace.cloudProjects.workItems.saved")); }
    } catch (mutationError) {
      const message = clientErrorCode(mutationError) === "CONFLICT"
        ? t("workspace.cloudProjects.workItems.editConflict")
        : t("workspace.cloudProjects.workItems.saveFailed");
      if (clientErrorCode(mutationError) === "CONFLICT") await refreshAfterMutation();
      presentError(mutationError, message);
    } finally { setPending(false); }
  };

  const moveItem = async (item: CloudProjectWorkItem, targetStatus: CloudProjectWorkItemStatus, beforeWorkItemId?: string): Promise<boolean> => {
    if (targetStatus === item.status) return true;
    if (readOnly || pending || !MOVE_TARGETS[item.status].includes(targetStatus)) return false;
    setPending(true); setError(""); setNotice("");
    try {
      await gateway.moveCloudProjectWorkItem({ projectId, workItemId: item.workItemId, expectedVersion: item.version, targetStatus, ...(beforeWorkItemId ? { beforeWorkItemId } : {}) });
      if (await refreshAfterMutation()) { setMoveIntent(null); setNotice(t("workspace.cloudProjects.workItems.moveDone")); return true; }
    } catch (moveError) {
      const retryable = clientErrorCode(moveError) === "WORK_ITEM_MOVE_RETRYABLE_CONFLICT";
      const message = retryable ? t("workspace.cloudProjects.workItems.moveConflict") : t("workspace.cloudProjects.workItems.moveUnknown");
      if (retryable) await refreshAfterMutation();
      else setMoveIntent(null);
      presentError(moveError, message);
    } finally { setPending(false); }
    return false;
  };

  const renameTableItem = async (item: CloudProjectWorkItem, title: string): Promise<boolean> => {
    if (readOnly || pending || !title.trim()) return false;
    if (title.trim() === item.title) return true;
    setPending(true); setError(""); setNotice("");
    try {
      await gateway.patchCloudProjectWorkItem({ projectId, workItemId: item.workItemId, expectedVersion: item.version, title: title.trim() });
      if (await refreshAfterMutation()) { setNotice(t("workspace.cloudProjects.workItems.saved")); return true; }
    } catch (renameError) {
      const message = clientErrorCode(renameError) === "CONFLICT" ? t("workspace.cloudProjects.workItems.editConflict") : t("workspace.cloudProjects.workItems.saveFailed");
      if (clientErrorCode(renameError) === "CONFLICT") await refreshAfterMutation();
      presentError(renameError, message);
    } finally { setPending(false); }
    return false;
  };

  const patchTablePriority = async (item: CloudProjectWorkItem, priority: WorkItemPriority): Promise<boolean> => {
    if (readOnly || pending) return false;
    if (priority === (item.priority || "none")) return true;
    setPending(true); setError(""); setNotice("");
    try {
      await gateway.patchCloudProjectWorkItem({ projectId, workItemId: item.workItemId, expectedVersion: item.version, priority });
      if (await refreshAfterMutation()) { setNotice(t("workspace.cloudProjects.workItems.saved")); return true; }
    } catch (priorityError) {
      const message = clientErrorCode(priorityError) === "CONFLICT" ? t("workspace.cloudProjects.workItems.editConflict") : t("workspace.cloudProjects.workItems.saveFailed");
      if (clientErrorCode(priorityError) === "CONFLICT") await refreshAfterMutation();
      presentError(priorityError, message);
    } finally { setPending(false); }
    return false;
  };

  const patchTableAssignee = async (item: CloudProjectWorkItem, assigneeUserId: string): Promise<boolean> => {
    if (readOnly || pending) return false;
    if (assigneeUserId === (item.assigneeUserId || "")) return true;
    setPending(true); setError(""); setNotice("");
    try {
      await gateway.patchCloudProjectWorkItem({ projectId, workItemId: item.workItemId, expectedVersion: item.version, assigneeSet: true, assigneeUserId: assigneeUserId || null });
      if (await refreshAfterMutation()) { setNotice(t("workspace.cloudProjects.workItems.saved")); return true; }
    } catch (assigneeError) {
      const message = clientErrorCode(assigneeError) === "CONFLICT" ? t("workspace.cloudProjects.workItems.editConflict") : t("workspace.cloudProjects.workItems.saveFailed");
      if (clientErrorCode(assigneeError) === "CONFLICT") await refreshAfterMutation();
      presentError(assigneeError, message);
    } finally { setPending(false); }
    return false;
  };

  const submitMove = async () => {
    if (!moveIntent) return;
    const item = items.find((candidate) => candidate.workItemId === moveIntent.workItemId);
    if (!item) return;
    await moveItem(item, moveIntent.targetStatus, moveIntent.anchor.startsWith("before:") ? moveIntent.anchor.slice(7) : undefined);
  };

  const deleteWorkItem = async () => {
    if (!editor || editor.kind !== "edit" || readOnly || pending) return;
    const current = items.find((item) => item.workItemId === editor.workItemId);
    if (!current) return;
    setPending(true); setError("");
    try {
      await gateway.deleteCloudProjectWorkItem({ projectId, workItemId: current.workItemId, expectedVersion: current.version });
      if (await refreshAfterMutation()) { setEditor(null); setNotice(t("workspace.cloudProjects.workItems.deleted")); }
    } catch (deleteError) {
      const message = clientErrorCode(deleteError) === "CONFLICT" ? t("workspace.cloudProjects.workItems.deleteConflict") : t("workspace.cloudProjects.workItems.deleteFailed");
      if (clientErrorCode(deleteError) === "CONFLICT") await refreshAfterMutation();
      presentError(deleteError, message);
    } finally { setPending(false); }
  };

  const createLabel = async (event: FormEvent) => {
    event.preventDefault();
    if (!gateway.createCloudProjectLabel || !labelName.trim() || readOnly || pending) return;
    setPending(true); setError("");
    try {
      const page = await gateway.createCloudProjectLabel({ projectId, name: labelName.trim(), color: labelColor });
      if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) { setLabels(page.items); setLabelName(""); setNotice("标签已保存"); }
    } catch (reason) { presentError(reason, "标签保存失败，请刷新后重试。"); }
    finally { setPending(false); }
  };

  const createEditorLabel = async (name: string, color: string): Promise<string | null> => {
    if (!gateway.createCloudProjectLabel || !name.trim() || readOnly || pending) return null;
    const normalizedName = name.trim();
    setPending(true); setError("");
    try {
      const page = await gateway.createCloudProjectLabel({ projectId, name: normalizedName, color });
      if (page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return null;
      setLabels(page.items);
      const created = page.items.find((label) => label.name.localeCompare(normalizedName, undefined, { sensitivity: "accent" }) === 0);
      if (!created) { setError("标签保存失败，请刷新后重试。"); return null; }
      setNotice("标签已创建并选中");
      return created.labelId;
    } catch (reason) {
      presentError(reason, "标签保存失败，请刷新后重试。");
      return null;
    } finally { setPending(false); }
  };

  const saveLabel = async (label: CloudProjectLabel) => {
    const draft = labelDrafts[label.labelId];
    if (!gateway.patchCloudProjectLabel || !draft?.name.trim()) return;
    setPending(true); setError("");
    try {
      const page = await gateway.patchCloudProjectLabel({ projectId, labelId: label.labelId, expectedVersion: label.version, ...(draft.name.trim() !== label.name ? { name: draft.name.trim() } : {}), ...(draft.color !== label.color ? { color: draft.color } : {}) });
      if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) setLabels(page.items);
    } catch (reason) { presentError(reason, "标签更新失败，可能已被其他成员修改，请刷新后重试。"); }
    finally { setPending(false); }
  };

  const deleteLabel = async (label: CloudProjectLabel) => {
    if (!gateway.deleteCloudProjectLabel) return;
    setPending(true); setError("");
    try {
      const page = await gateway.deleteCloudProjectLabel({ projectId, labelId: label.labelId, expectedVersion: label.version });
      if (page.projectScopeKey === projectScopeKey && page.projectId === projectId) setLabels(page.items);
    } catch (labelError) {
      presentError(labelError, clientErrorCode(labelError) === "CONFLICT" ? "标签正在被事项使用，请先移除事项上的该标签。" : "标签删除失败，请刷新后重试。");
    } finally { setPending(false); }
  };

  const applySearch = () => {
    const search = searchDraft.trim();
    setSearchOpen(false);
    if (search === filters.search) return;
    const next = { ...filters, search };
    setFilters(next);
    void loadColumns(next);
  };

  const updateFilter = (patch: Partial<PlanFilters>) => {
    const next = { ...filters, ...patch };
    setFilters(next);
    void loadColumns(next);
  };

  const clearFilters = () => {
    setSearchDraft("");
    setFilterMenuOpen(false);
    if (Object.values(filters).every((value) => value === "")) return;
    setFilters(EMPTY_FILTERS);
    void loadColumns(EMPTY_FILTERS);
  };

  return (
    <section className="work-item-board" aria-label={`${projectName} 的事项`} data-readonly={readOnly || undefined} data-work-item-view="plan">
        <header className="work-item-board__plan-header">
          <div className="work-item-board__views" role="toolbar" aria-label={t("workspace.cloudProjects.workItems.planViews")}>
            <button type="button" className="work-item-board__all-view" aria-pressed={activeFilterCount === 0} onClick={clearFilters}><LayoutDashboard size={14} aria-hidden="true" />全部</button>
            {PLAN_VIEW_OPTIONS.map(({ mode, label, Icon }) => <button key={mode} type="button" aria-pressed={displayMode === mode} onClick={() => setDisplayMode(mode)}><Icon size={14} aria-hidden="true" />{label}</button>)}
          </div>
          <div className="work-item-board__actions">
            <span ref={filterMenuRef} className="work-item-board__action-popover">
              <button ref={filterTriggerRef} type="button" aria-label="筛选事项" aria-expanded={filterMenuOpen} onClick={() => { setFilterMenuOpen((open) => !open); setSearchOpen(false); }}><Filter size={16} aria-hidden="true" />{activeFilterCount ? <small aria-label={`${activeFilterCount} 个筛选条件`}>{activeFilterCount}</small> : null}</button>
              {filterMenuOpen ? <span className="work-item-board__filter-menu" role="dialog" aria-label="筛选事项">
                <label><span>负责人</span><ProjectSelect ariaLabel="负责人" value={filters.assigneeUserId} onChange={(assigneeUserId) => updateFilter({ assigneeUserId })} options={[{ value: "", label: "全部负责人" }, ...members.map((member) => ({ value: member.userId, label: member.displayName }))]} /></label>
                <label><span>优先级</span><ProjectSelect ariaLabel="优先级" value={filters.priority} onChange={(priority) => updateFilter({ priority })} options={[{ value: "" as const, label: "全部优先级" }, ...WORK_ITEM_PRIORITY_OPTIONS.map((option) => ({ ...option, label: option.value === "none" ? "无优先级" : option.label }))]} /></label>
                <label><span>标签</span><ProjectSelect ariaLabel="标签" value={filters.labelId} onChange={(labelId) => updateFilter({ labelId })} options={[{ value: "", label: "全部标签" }, ...labels.map((label) => ({ value: label.labelId, label: label.name }))]} /></label>
                <button type="button" disabled={!activeFilterCount} onClick={clearFilters}>清除筛选</button>
              </span> : null}
            </span>
            <span className="work-item-board__action-popover work-item-board__search-control">
              <button type="button" aria-label="搜索事项" aria-expanded={searchOpen} onClick={() => { setSearchOpen((open) => !open); setFilterMenuOpen(false); }}><Search size={16} aria-hidden="true" /></button>
              {searchOpen ? <form role="search" onSubmit={(event) => { event.preventDefault(); if (!searchComposingRef.current) applySearch(); }}><Search size={15} aria-hidden="true" /><input ref={searchInputRef} type="search" value={searchDraft} onChange={(event) => setSearchDraft(event.currentTarget.value)} onBlur={applySearch} onCompositionStart={() => { searchComposingRef.current = true; }} onCompositionEnd={() => { searchComposingRef.current = false; }} onKeyDown={(event) => { if (event.key === "Escape") { event.preventDefault(); setSearchDraft(filters.search); setSearchOpen(false); } }} placeholder="搜索标题或说明" aria-label="搜索事项" /></form> : null}
            </span>
            <button type="button" aria-label="标签设置" onClick={(event) => { dialogTriggerRef.current = event.currentTarget; setLabelSettingsOpen(true); }}><Settings2 size={16} aria-hidden="true" /></button>
            <span className="work-item-board__action-divider" aria-hidden="true" />
            <button type="button" className="work-item-board__add" disabled={readOnly || pending} onClick={(event) => openCreate("todo", event.currentTarget)}><Plus size={15} aria-hidden="true" />添加</button>
          </div>
        </header>
        {error ? <p className="cloud-projects__inline-error" role="alert">{error}</p> : null}
        {notice ? <p className="work-item-board__notice" role="status">{notice}</p> : null}
        {displayMode !== "calendar" && loading && !items.length ? <p role="status">{t("common.loading")}</p> : null}
        {displayMode === "board" ? <div className="work-item-board__board-scroller"><div className="work-item-board__columns" data-plan-board>
          {STATUS_ORDER.map((status) => {
            const column = columns[status];
            const StatusIcon = WORK_ITEM_STATUS_ICONS[status];
            return <section className={`work-item-board__column work-item-board__column--${status}`} key={status} aria-labelledby={`work-item-status-${status}`} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const item = items.find((candidate) => candidate.workItemId === dragItemRef.current); if (item) void moveItem(item, status); }}>
              <header><div><StatusIcon className="work-item-board__status-icon" size={16} aria-hidden="true" /><h3 id={`work-item-status-${status}`}>{t(`workspace.cloudProjects.workItems.status.${status}`)}</h3><small>{column.totalCount}</small></div><button type="button" disabled={readOnly || pending} aria-label={`在${t(`workspace.cloudProjects.workItems.status.${status}`)}中新建事项`} onClick={(event) => openCreate(status, event.currentTarget)}><Plus size={16} aria-hidden="true" /></button></header>
              <ol aria-label={t("workspace.cloudProjects.workItems.listLabel", { status: t(`workspace.cloudProjects.workItems.status.${status}`) })}>
                {column.items.map((item) => <li key={item.workItemId} className="work-item-board__card" draggable={!readOnly && !pending} onDragStart={() => { dragItemRef.current = item.workItemId; }} onDragEnd={() => { dragItemRef.current = ""; }} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); event.stopPropagation(); const moving = items.find((candidate) => candidate.workItemId === dragItemRef.current); if (moving && moving.workItemId !== item.workItemId) void moveItem(moving, status, item.workItemId); }}>
                  <button type="button" className="work-item-board__card-main" onClick={(event) => openDetail(item, event.currentTarget)}>
                    <span className="work-item-board__card-title"><GripVertical size={14} aria-hidden="true" /><strong>{item.title}</strong></span>
                    <span className="work-item-board__card-meta"><span className={`work-item-board__card-status work-item-board__card-status--${item.status}`}>{t(`workspace.cloudProjects.workItems.status.${item.status}`)}</span><span className={`work-item-board__priority work-item-board__priority--${item.priority || "none"}`}>{workItemPriorityLabel(item.priority as WorkItemPriority | null, "无优先级")}</span>{item.labels.map((label) => <i key={label.labelId} style={{ borderColor: label.color }}>{label.name}</i>)}</span>
                    <span className="work-item-board__card-footer"><span className="work-item-board__assignee"><UserRound size={13} aria-hidden="true" />{item.assigneeDisplayName || "未指派"}</span><time dateTime={item.updatedAt}>{formatProjectDateTime(item.updatedAt)}</time></span>
                  </button>
                  <button type="button" className="work-item-board__card-menu" disabled={readOnly || pending} onClick={(event) => beginMove(item, event.currentTarget)} aria-label={`移动 ${item.title}`}><MoreHorizontal size={16} aria-hidden="true" /></button>
                </li>)}
                {!column.items.length && !column.loading ? <li className="work-item-board__column-empty">暂无事项</li> : null}
              </ol>
              {column.nextCursor ? <button type="button" className="work-item-board__column-more" disabled={column.loading || pending} onClick={() => void loadMore(status)}>{column.loading ? t("common.loading") : `加载更多（${column.items.length}/${column.totalCount}）`}</button> : null}
            </section>;
          })}
        </div></div> : null}
        {displayMode === "table" ? <WorkItemTableProjection items={items} members={members} readOnly={readOnly} pending={pending} onOpen={openDetail} onRename={renameTableItem} onPriorityChange={patchTablePriority} onStatusChange={moveItem} onAssigneeChange={patchTableAssignee} /> : null}
        {displayMode === "list" ? <WorkItemListProjection columns={columns} readOnly={readOnly} pending={pending} onEdit={openDetail} onCreate={openCreate} onLoadMore={(status) => void loadMore(status)} /> : null}
        {displayMode === "gantt" ? <WorkItemGanttProjection items={items} onEdit={openDetail} /> : null}
        {displayMode === "calendar" ? <ContinuousWorkItemCalendar projectScopeKey={projectScopeKey} projectId={projectId} projectName={projectName} filters={filters} gateway={gateway} refreshRevision={calendarRefreshRevision} onEdit={openDetail} onCreate={openCreateForDate} readOnly={readOnly} /> : null}
        {displayMode !== "board" && displayMode !== "calendar" && hasMoreItems ? <div className="work-item-board__projection-more"><button type="button" disabled={pending || STATUS_ORDER.some((status) => columns[status].loading)} aria-label="加载剩余事项" onClick={() => void loadRemainingProjectionItems()}>加载更多（{items.length}/{totalCount}）</button></div> : null}

      {moveIntent ? <ProjectModalSurface backdropClassName="work-item-board__dialog-backdrop" className="work-item-board__dialog" titleId="work-item-move-title" pending={pending} returnFocus={dialogTriggerRef.current} onClose={() => setMoveIntent(null)}><header><h3 id="work-item-move-title">移动事项</h3><button type="button" onClick={() => setMoveIntent(null)} aria-label="关闭"><X size={17} /></button></header><label>目标状态<select value={moveIntent.targetStatus} onChange={(event) => setMoveIntent({ ...moveIntent, targetStatus: event.currentTarget.value as CloudProjectWorkItemStatus, anchor: "append" })}>{(movingItem ? MOVE_TARGETS[movingItem.status] : []).map((status) => <option key={status} value={status}>{t(`workspace.cloudProjects.workItems.status.${status}`)}</option>)}</select></label><label>目标位置<select value={moveIntent.anchor} onChange={(event) => setMoveIntent({ ...moveIntent, anchor: event.currentTarget.value })}><option value="append">列尾</option>{columns[moveIntent.targetStatus].items.filter((item) => item.workItemId !== moveIntent.workItemId).map((item) => <option key={item.workItemId} value={`before:${item.workItemId}`}>在“{item.title}”之前</option>)}</select></label><footer><button type="button" onClick={() => void submitMove()} disabled={pending}>确认移动</button><button type="button" onClick={() => setMoveIntent(null)}>取消</button></footer></ProjectModalSurface> : null}

      {editor ? <WorkItemEditorDialog editor={editor} setEditor={setEditor} titleRef={editorTitleRef} members={members} labels={labels} statusOptions={editor.kind === "create" ? STATUS_ORDER : MOVE_TARGETS[editor.persisted?.status ?? editor.status]} pending={pending} readOnly={readOnly} dateInvalid={editorDateInvalid} dateErrorId={editorDateErrorId} projectId={projectId} projectScopeKey={projectScopeKey} gateway={gateway} onCreateLabel={gateway.createCloudProjectLabel ? createEditorLabel : undefined} onSubmit={() => void submitEditor()} onDelete={() => void deleteWorkItem()} onClose={() => setEditor(null)} /> : null}

      {detailItem ? <WorkItemDetailDialog item={detailItem} assets={assets.filter((asset) => asset.workItemIds.includes(detailItem.workItemId))} activities={detailActivities} activityLoading={detailActivityLoading} gateway={gateway} returnFocus={dialogTriggerRef.current} onEdit={() => beginEdit(detailItem)} onClose={() => { detailActivityRequestRef.current += 1; setDetailItem(null); }} /> : null}

      {labelSettingsOpen ? <ProjectModalSurface backdropClassName="work-item-board__dialog-backdrop" className="work-item-board__dialog work-item-board__dialog--labels" titleId="label-settings-title" pending={pending} returnFocus={dialogTriggerRef.current} onClose={() => setLabelSettingsOpen(false)}><header><h3 id="label-settings-title">标签设置</h3><button type="button" onClick={() => setLabelSettingsOpen(false)} aria-label="关闭"><X size={17} /></button></header><div className="work-item-board__dialog-body"><form className="work-item-board__label-create" onSubmit={createLabel}><input value={labelName} onChange={(event) => setLabelName(event.currentTarget.value)} placeholder="新标签名称" maxLength={80} /><input type="color" value={labelColor} onChange={(event) => setLabelColor(event.currentTarget.value)} /><button type="submit" disabled={!labelName.trim() || pending}>创建</button></form><div className="work-item-board__label-rows">{labels.map((label) => { const draft = labelDrafts[label.labelId] || { name: label.name, color: label.color }; return <div key={label.labelId}><input value={draft.name} onChange={(event) => { const name = event.currentTarget.value; setLabelDrafts((current) => ({ ...current, [label.labelId]: { ...draft, name } })); }} /><input type="color" value={draft.color} onChange={(event) => { const color = event.currentTarget.value; setLabelDrafts((current) => ({ ...current, [label.labelId]: { ...draft, color } })); }} /><button type="button" onClick={() => void saveLabel(label)} disabled={pending || (draft.name === label.name && draft.color === label.color)}>保存</button><button type="button" onClick={() => void deleteLabel(label)} disabled={pending}>删除</button></div>; })}</div></div><footer><button type="button" onClick={() => setLabelSettingsOpen(false)}>完成</button></footer></ProjectModalSurface> : null}
    </section>
  );
}
