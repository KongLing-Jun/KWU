import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import {
  Archive,
  ArrowDown,
  ArrowUp,
  ChevronRight,
  Download,
  ExternalLink,
  File,
  FileAudio,
  FileImage,
  FileText,
  FileUp,
  FileVideo,
  Folder,
  FolderInput,
  History,
  Info,
  LoaderCircle,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import type { ProjectGateway } from "../data/projectGateway";
import { clientErrorCode, clientErrorMessageWithoutDiagnosticID } from "../../../lib/clientError";
import { useToast } from "../../../lib/toast";
import { formatProjectDateTime } from "../model/projectDateTime";
import type {
  CloudProjectAsset,
  CloudProjectAssetBreadcrumb,
  CloudProjectAssetDirectoryInput,
  CloudProjectAssetDirectoryItem,
  CloudProjectAssetDirectoryPage,
  CloudProjectAssetOperation,
  CloudProjectAssetOperationRecord,
  CloudProjectAssetOperationResourceKind,
  CloudProjectAssetProcessingState,
} from "../model/projectTypes";
import { ProjectModalSurface } from "./ProjectModalSurface";
import { ProjectSelect } from "../../../components/ProjectSelect";

type AssetDialog =
  | { kind: "create-folder"; trigger: HTMLElement | null }
  | { kind: "rename"; item: CloudProjectAssetDirectoryItem; trigger: HTMLElement | null }
  | { kind: "move"; items: CloudProjectAssetDirectoryItem[]; trigger: HTMLElement | null }
  | { kind: "delete"; items: CloudProjectAssetDirectoryItem[]; trigger: HTMLElement | null };

type DirectoryType = "all" | "folder" | "image" | "document" | "pdf" | "markdown" | "audio" | "video" | "archive" | "other";
type DirectorySort = "name" | "updated_at";
type SortDirection = "asc" | "desc";

const DIRECTORY_TYPE_OPTIONS: Array<{ value: DirectoryType; label: string }> = [
  { value: "all", label: "全部类型" },
  { value: "folder", label: "文件夹" },
  { value: "image", label: "图片" },
  { value: "document", label: "文档" },
  { value: "pdf", label: "PDF" },
  { value: "markdown", label: "Markdown" },
  { value: "audio", label: "音频" },
  { value: "video", label: "视频" },
  { value: "archive", label: "压缩包" },
  { value: "other", label: "其他" },
];

const PAGE_SIZE = 50;
const OPERATION_RECORD_LIMIT = 100;

const OPERATION_FILTER_OPTIONS: Array<{ value: "" | CloudProjectAssetOperation; label: string }> = [
  { value: "", label: "全部操作" },
  { value: "create", label: "创建" },
  { value: "read", label: "查看/下载" },
  { value: "update", label: "修改" },
  { value: "delete", label: "删除" },
];

const RESOURCE_KIND_FILTER_OPTIONS: Array<{ value: "" | CloudProjectAssetOperationResourceKind; label: string }> = [
  { value: "", label: "全部资源" },
  { value: "asset", label: "文件" },
  { value: "folder", label: "文件夹" },
  { value: "directory", label: "目录" },
];

interface ProjectAssetsPanelProps {
  projectId: string;
  projectScopeKey: string;
  gateway: ProjectGateway;
  readOnly: boolean;
  onUpgrade?: () => void;
}

export function ProjectAssetsPanel({ projectId, projectScopeKey, gateway, readOnly, onUpgrade }: ProjectAssetsPanelProps) {
  const { showError } = useToast();
  const requestGeneration = useRef(0);
  const activeViewIdentity = useRef("");
  const itemsRef = useRef<CloudProjectAssetDirectoryItem[]>([]);
  const composingRef = useRef(false);
  const nextCursorRef = useRef<string | null>(null);
  const [items, setItems] = useState<CloudProjectAssetDirectoryItem[]>([]);
  const [breadcrumbs, setBreadcrumbs] = useState<CloudProjectAssetBreadcrumb[]>([]);
  const [processingStates, setProcessingStates] = useState<CloudProjectAssetProcessingState[]>([]);
  const [processingUnavailable, setProcessingUnavailable] = useState(false);
  const [folderId, setFolderId] = useState("");
  const [draftSearch, setDraftSearch] = useState("");
  const [appliedSearch, setAppliedSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<DirectoryType>("all");
  const [sort, setSort] = useState<DirectorySort>("updated_at");
  const [direction, setDirection] = useState<SortDirection>("desc");
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [totalCount, setTotalCount] = useState(0);
  const [usageBytes, setUsageBytes] = useState(0);
  const [quotaBytes, setQuotaBytes] = useState<number | null>(null);
  const [quotaPlan, setQuotaPlan] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const [downloadedPath, setDownloadedPath] = useState("");
  const [openMenuId, setOpenMenuId] = useState("");
  const [dialog, setDialog] = useState<AssetDialog | null>(null);
  const [operationRecordsTrigger, setOperationRecordsTrigger] = useState<HTMLElement | null>(null);
  itemsRef.current = items;

  const presentError = useCallback((reason: unknown, safeMessage = assetError(reason)) => {
    setMessage(safeMessage);
    showError("projects.operation", reason, safeMessage);
  }, [showError]);

  const viewIdentity = JSON.stringify([projectScopeKey, projectId, folderId, appliedSearch, typeFilter, sort, direction]);
  activeViewIdentity.current = viewIdentity;

  const accept = useCallback((generation: number, identity: string, scope: string, resultProjectId: string) => (
    generation === requestGeneration.current && identity === activeViewIdentity.current && scope === projectScopeKey && resultProjectId === projectId
  ), [projectId, projectScopeKey]);

  const loadProcessingStates = useCallback(async (generation: number, identity: string, assetIds: string[]) => {
    try {
      if (typeof gateway.listCloudProjectAssetProcessingStates !== "function") throw new Error("processing state bridge is unavailable");
      const result = await gateway.listCloudProjectAssetProcessingStates({ projectId, assetIds });
      if (!result || !accept(generation, identity, result.projectScopeKey, result.projectId)) return;
      setProcessingStates(result.items);
      setProcessingUnavailable(false);
    } catch (reason) {
      if (generation !== requestGeneration.current || identity !== activeViewIdentity.current) return;
      setProcessingStates([]);
      setProcessingUnavailable(true);
      showError("projects.operation", reason, "处理状态暂不可用，文件列表仍可正常使用。");
    }
  }, [accept, gateway, projectId, showError]);

  const directoryInput = useCallback((cursor = ""): CloudProjectAssetDirectoryInput => ({
    projectId,
    folderId: folderId || undefined,
    search: appliedSearch || undefined,
    type: typeFilter === "all" ? undefined : typeFilter,
    sort,
    direction,
    cursor: cursor || undefined,
    limit: PAGE_SIZE,
  }), [appliedSearch, direction, folderId, projectId, sort, typeFilter]);

  const load = useCallback(async (append = false) => {
    const identity = viewIdentity;
    const generation = append ? requestGeneration.current : requestGeneration.current + 1;
    if (!append) requestGeneration.current = generation;
    append ? setLoadingMore(true) : setLoading(true);
    setMessage("");
    try {
      let result: CloudProjectAssetDirectoryPage;
      if (typeof gateway.listCloudProjectAssetDirectory === "function") {
        result = await gateway.listCloudProjectAssetDirectory(directoryInput(append ? nextCursorRef.current || "" : ""));
      } else {
        const legacy = await gateway.listCloudProjectAssets(projectId);
        result = legacyDirectoryPage(legacy.projectScopeKey, legacy.projectId, legacy.items);
      }
      if (!accept(generation, identity, result.projectScopeKey, result.projectId)) return;
      const visibleItems = append ? mergeDirectoryItems(itemsRef.current, result.items) : result.items;
      setItems(visibleItems);
      setBreadcrumbs(result.breadcrumbs || []);
      setNextCursor(result.nextCursor || null);
      nextCursorRef.current = result.nextCursor || null;
      setTotalCount(result.totalCount);
      setUsageBytes(result.usageBytes);
      setQuotaBytes(result.quotaBytes > 0 ? result.quotaBytes : null);
      setQuotaPlan(result.quotaBytes > 0 ? result.quotaPlan : "");
      void loadProcessingStates(generation, identity, visibleItems.filter((item) => item.kind === "asset").map((item) => item.id));
      if (!append) setSelectedIds(new Set());
    } catch (error) {
      if (generation === requestGeneration.current && identity === activeViewIdentity.current) presentError(error);
    } finally {
      if (generation === requestGeneration.current && identity === activeViewIdentity.current) {
        setLoading(false);
        setLoadingMore(false);
      }
    }
  }, [accept, directoryInput, gateway, loadProcessingStates, presentError, projectId, viewIdentity]);

  useEffect(() => {
    requestGeneration.current += 1;
    setFolderId("");
    setDraftSearch("");
    setAppliedSearch("");
    setSelectedIds(new Set());
    setDialog(null);
    setOperationRecordsTrigger(null);
    setBusy("");
    setMessage("");
    setDownloadedPath("");
  }, [projectId, projectScopeKey]);

  useEffect(() => { void load(false); }, [load]);

  useEffect(() => {
    if (!openMenuId) return;
    const close = () => setOpenMenuId("");
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [openMenuId]);

  const commitSearch = () => {
    const normalized = draftSearch.trim();
    if (normalized !== appliedSearch) setAppliedSearch(normalized);
  };

  const upload = async () => {
    const identity = activeViewIdentity.current;
    setBusy("upload"); setMessage("");
    try {
      await gateway.uploadCloudProjectAsset(projectId, folderId || undefined);
      if (identity === activeViewIdentity.current) await load(false);
    } catch (error) { if (identity === activeViewIdentity.current) presentError(error); }
    finally { if (identity === activeViewIdentity.current) setBusy(""); }
  };

  const download = async (item: CloudProjectAssetDirectoryItem) => {
    if (item.kind !== "asset") return;
    const identity = activeViewIdentity.current;
    setBusy(item.id); setMessage(""); setDownloadedPath("");
    try {
      const result = await gateway.downloadCloudProjectAsset(projectId, item.id);
      if (identity === activeViewIdentity.current) setDownloadedPath(result.savedPath);
    }
    catch (error) { if (identity === activeViewIdentity.current) presentError(error); }
    finally { if (identity === activeViewIdentity.current) setBusy(""); }
  };

  const toggleSelection = (id: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  };

  const selectedItems = items.filter((item) => selectedIds.has(item.id));
  const allSelected = items.length > 0 && items.every((item) => selectedIds.has(item.id));
  const processingStateByAsset = useMemo(() => {
    const latest = new Map<string, CloudProjectAssetProcessingState>();
    for (const state of processingStates) {
      const current = latest.get(state.assetId);
      if (!current || state.versionNumber > current.versionNumber) latest.set(state.assetId, state);
    }
    return latest;
  }, [processingStates]);

  const openFolder = (item: CloudProjectAssetDirectoryItem) => {
    if (item.kind !== "folder") return;
    setFolderId(item.id);
    setSelectedIds(new Set());
  };

  const changeSort = (nextSort: DirectorySort) => {
    if (sort === nextSort) setDirection((current) => current === "asc" ? "desc" : "asc");
    else {
      setSort(nextSort);
      setDirection(nextSort === "name" ? "asc" : "desc");
    }
  };

  const runMutation = async (operation: () => Promise<unknown>) => {
    const identity = activeViewIdentity.current;
    setBusy("mutation"); setMessage("");
    try {
      await operation();
      if (identity !== activeViewIdentity.current) return;
      setDialog(null);
      await load(false);
    } catch (error) {
      if (identity !== activeViewIdentity.current) return;
      if (clientErrorCode(error) === "CONFLICT" || /conflict|version/i.test(error instanceof Error ? error.message : String(error))) {
        await load(false);
        if (identity === activeViewIdentity.current) presentError(error, "资产已被其他成员更新，已刷新为云端最新版本。");
      } else presentError(error);
    } finally { if (identity === activeViewIdentity.current) setBusy(""); }
  };

  const runBatchMutation = async (
    targets: CloudProjectAssetDirectoryItem[],
    operation: (item: CloudProjectAssetDirectoryItem) => Promise<unknown>,
  ) => {
    const identity = activeViewIdentity.current;
    setBusy("mutation");
    setMessage("");
    const failures: string[] = [];
    for (const item of targets) {
      if (identity !== activeViewIdentity.current) return;
      try {
        await operation(item);
      } catch (error) {
        const message = `${item.name}：${assetError(error)}`;
        failures.push(message);
        showError("projects.operation", error, message);
      }
    }
    if (identity !== activeViewIdentity.current) return;
    setDialog(null);
    await load(false);
    if (failures.length > 0) setMessage(`部分操作未完成：${failures.join("；")}`);
    setBusy("");
  };

  return (
    <section className="cloud-projects__business-shell project-assets" data-project-assets-panel>
      <div className="project-assets__toolbar">
        <div className="project-assets__toolbar-primary">
          <button type="button" disabled={readOnly || Boolean(busy) || typeof gateway.createCloudProjectAssetFolder !== "function"} onClick={(event) => setDialog({ kind: "create-folder", trigger: event.currentTarget })}>新建文件夹</button>
          <button type="button" disabled={readOnly || Boolean(busy)} onClick={() => void upload()}>{busy === "upload" ? <LoaderCircle className="spin" size={15} aria-hidden="true" /> : null}上传文件</button>
          <button type="button" disabled={typeof gateway.listCloudProjectAssetOperationRecords !== "function"} onClick={(event) => setOperationRecordsTrigger(event.currentTarget)}><History size={15} aria-hidden="true" />操作记录</button>
          <span className="project-assets__quota">存储空间已用 {formatBytes(usageBytes)} / {quotaBytes == null ? "额度暂不可用" : formatBytes(quotaBytes)} {quotaBytes == null ? null : <Info size={14} aria-label={`当前 ${quotaPlan.toUpperCase()} 套餐存储额度`} />}</span>
          <button type="button" className="project-assets__upgrade" disabled={!onUpgrade} aria-label="升级存储空间" onClick={onUpgrade}>升级 <ExternalLink size={12} aria-hidden="true" /></button>
        </div>
        <div className="project-assets__toolbar-secondary">
          <ProjectSelect ariaLabel="文件类型" value={typeFilter} options={DIRECTORY_TYPE_OPTIONS} className="project-assets__type-filter" onChange={setTypeFilter} />
          <label className="project-assets__search">
            <Search size={15} aria-hidden="true" />
            <input
              type="search"
              value={draftSearch}
              placeholder="搜索文件或文件夹..."
              aria-label="搜索文件或文件夹"
              onChange={(event) => setDraftSearch(event.currentTarget.value)}
              onBlur={commitSearch}
              onCompositionStart={() => { composingRef.current = true; }}
              onCompositionEnd={() => { composingRef.current = false; }}
              onKeyDown={(event) => {
                if (event.key !== "Enter" || event.nativeEvent.isComposing || composingRef.current) return;
                event.preventDefault();
                commitSearch();
              }}
            />
          </label>
        </div>
      </div>

      {breadcrumbs.length > 0 ? <nav className="project-assets__breadcrumbs" aria-label="资产目录路径"><button type="button" onClick={() => setFolderId("")}>资产</button>{breadcrumbs.map((crumb) => <span key={crumb.folderId}><ChevronRight size={13} aria-hidden="true" /><button type="button" aria-current={crumb.folderId === folderId ? "page" : undefined} onClick={() => setFolderId(crumb.folderId)}>{crumb.name}</button></span>)}</nav> : null}
      {message ? <p className="cloud-projects__inline-error project-assets__message" role="alert">{message}</p> : null}
      {downloadedPath ? <p className="cloud-projects__inline-notice project-assets__message" role="status">已下载到 {downloadedPath}</p> : null}
      {processingUnavailable ? <p className="cloud-projects__inline-notice project-assets__message" role="status">处理状态暂不可用，文件列表仍可正常使用。</p> : null}

      {selectedItems.length > 0 ? <div className="project-assets__selection-bar" role="status"><span>已选择 {selectedItems.length} 项</span><div><button type="button" disabled={readOnly || Boolean(busy)} onClick={(event) => setDialog({ kind: "move", items: selectedItems, trigger: event.currentTarget })}><FolderInput size={15} aria-hidden="true" />移动</button><button type="button" className="is-danger" disabled={readOnly || Boolean(busy)} onClick={(event) => setDialog({ kind: "delete", items: selectedItems, trigger: event.currentTarget })}><Trash2 size={15} aria-hidden="true" />删除</button><button type="button" aria-label="清除选择" onClick={() => setSelectedIds(new Set())}><X size={15} aria-hidden="true" /></button></div></div> : null}

      <div className="project-assets__table" role="table" aria-label="项目资产">
        <div className="project-assets__table-head" role="row">
          <span role="columnheader"><input type="checkbox" aria-label="选择全部文件" checked={allSelected} onChange={() => setSelectedIds(allSelected ? new Set() : new Set(items.map((item) => item.id)))} /></span>
          <button type="button" role="columnheader" onClick={() => changeSort("name")}>名称 <SortIcon active={sort === "name"} direction={direction} /></button>
          <span role="columnheader">类型</span><span role="columnheader">更新人</span>
          <button type="button" role="columnheader" onClick={() => changeSort("updated_at")}>更新时间 <SortIcon active={sort === "updated_at"} direction={direction} /></button>
          <span role="columnheader">大小</span><span role="columnheader" className="sr-only">操作</span>
        </div>
        {loading ? <div className="project-assets__empty" role="status"><LoaderCircle className="spin" size={20} />正在加载…</div> : items.length === 0 ? <div className="project-assets__empty"><Folder size={30} aria-hidden="true" /><strong>{appliedSearch || typeFilter !== "all" ? "没有匹配的文件" : "这个文件夹为空"}</strong><span>{readOnly ? "归档项目中的资产仅可查看。" : "上传文件或创建文件夹开始整理项目资料。"}</span></div> : <div className="project-assets__table-body" role="rowgroup">{items.map((item) => {
          const selected = selectedIds.has(item.id);
          const state = item.kind === "asset" ? processingStateByAsset.get(item.id) : undefined;
          return <div key={item.id} className={`project-assets__row${selected ? " is-selected" : ""}`} role="row" data-kind={item.kind} onDoubleClick={() => openFolder(item)}>
            <span role="cell" className="project-assets__check"><input type="checkbox" aria-label={`选择 ${item.name}`} checked={selected} onChange={() => toggleSelection(item.id)} /></span>
            <button type="button" role="cell" className="project-assets__name" onClick={() => item.kind === "folder" ? openFolder(item) : toggleSelection(item.id)} title={item.name}><AssetIcon item={item} /><span>{item.name}</span>{item.kind === "folder" ? <ChevronRight size={14} aria-hidden="true" /> : null}</button>
            <span role="cell" data-label="类型">{itemTypeLabel(item)}</span>
            <span role="cell" data-label="更新人" title={item.updatedBy?.displayName || "未知成员"}>{item.updatedBy?.displayName || "未知成员"}</span>
            <span role="cell" data-label="更新时间">{formatProjectDateTime(item.updatedAt, "-")}</span>
            <span role="cell" data-label="大小">{item.kind === "asset" ? formatBytes(item.sizeBytes || 0) : "-"}</span>
            <div role="cell" className="project-assets__row-menu" onPointerDown={(event) => event.stopPropagation()}>
              <button type="button" aria-label={`${item.name} 操作`} aria-haspopup="menu" aria-expanded={openMenuId === item.id} onClick={(event) => { event.stopPropagation(); setOpenMenuId((current) => current === item.id ? "" : item.id); }}><MoreHorizontal size={17} aria-hidden="true" /></button>
              {openMenuId === item.id ? <div role="menu" onPointerDown={(event) => event.stopPropagation()}>
                {item.kind === "asset" ? <button type="button" role="menuitem" onClick={() => { setOpenMenuId(""); void download(item); }}><Download size={15} aria-hidden="true" />下载</button> : null}
                {item.kind === "asset" ? <button type="button" role="menuitem" disabled={readOnly || typeof gateway.replaceCloudProjectAssetContent !== "function"} onClick={() => { setOpenMenuId(""); void runMutation(async () => {
                  if (!gateway.replaceCloudProjectAssetContent) throw new Error("当前 Cloud 暂不支持替换文件内容");
                  await gateway.replaceCloudProjectAssetContent({ projectId, assetId: item.id, expectedVersion: item.version, folderId: folderId || undefined });
                }); }}><FileUp size={15} aria-hidden="true" />替换内容</button> : null}
                <button type="button" role="menuitem" disabled={readOnly} onClick={(event) => { setOpenMenuId(""); setDialog({ kind: "rename", item, trigger: event.currentTarget }); }}><Pencil size={15} aria-hidden="true" />重命名</button>
                <button type="button" role="menuitem" disabled={readOnly} onClick={(event) => { setOpenMenuId(""); setDialog({ kind: "move", items: [item], trigger: event.currentTarget }); }}><FolderInput size={15} aria-hidden="true" />移动</button>
                {state ? <span className="project-assets__processing" role="status">扫描 {statusLabel(state.scanStatus)} · 解析 {statusLabel(state.parseStatus)}</span> : null}
                <button type="button" role="menuitem" className="is-danger" disabled={readOnly} onClick={(event) => { setOpenMenuId(""); setDialog({ kind: "delete", items: [item], trigger: event.currentTarget }); }}><Trash2 size={15} aria-hidden="true" />删除</button>
              </div> : null}
            </div>
          </div>;
        })}</div>}
      </div>
      {!loading && nextCursor ? <button type="button" className="project-assets__load-more" disabled={loadingMore} onClick={() => void load(true)}>{loadingMore ? <LoaderCircle className="spin" size={15} aria-hidden="true" /> : null}加载更多（{items.length}/{totalCount}）</button> : null}

      {dialog?.kind === "create-folder" || dialog?.kind === "rename" ? <AssetNameDialog dialog={dialog} busy={Boolean(busy)} onClose={() => setDialog(null)} onSubmit={(name) => void runMutation(async () => {
        if (dialog.kind === "create-folder") {
          if (!gateway.createCloudProjectAssetFolder) throw new Error("当前 Cloud 暂不支持创建文件夹");
          await gateway.createCloudProjectAssetFolder({ projectId, name, parentFolderId: folderId || undefined });
        } else if (dialog.item.kind === "folder") {
          if (!gateway.patchCloudProjectAssetFolder) throw new Error("当前 Cloud 暂不支持重命名文件夹");
          await gateway.patchCloudProjectAssetFolder({ projectId, folderId: dialog.item.id, expectedVersion: dialog.item.version, name });
        } else {
          if (!gateway.patchCloudProjectAsset) throw new Error("当前 Cloud 暂不支持重命名文件");
          await gateway.patchCloudProjectAsset({ projectId, assetId: dialog.item.id, expectedVersion: dialog.item.version, name });
        }
      })} /> : null}
      {dialog?.kind === "move" ? <AssetFolderPickerDialog projectId={projectId} gateway={gateway} items={dialog.items} busy={Boolean(busy)} returnFocus={dialog.trigger} onClose={() => setDialog(null)} onConfirm={(targetFolderId) => void runBatchMutation(dialog.items, async (item) => {
          if (item.kind === "folder") {
            if (!gateway.patchCloudProjectAssetFolder) throw new Error("当前 Cloud 暂不支持移动文件夹");
            await gateway.patchCloudProjectAssetFolder({ projectId, folderId: item.id, expectedVersion: item.version, parentFolderId: targetFolderId || null });
          } else {
            if (!gateway.patchCloudProjectAsset) throw new Error("当前 Cloud 暂不支持移动文件");
            await gateway.patchCloudProjectAsset({ projectId, assetId: item.id, expectedVersion: item.version, folderId: targetFolderId || null });
          }
      })} /> : null}
      {dialog?.kind === "delete" ? <AssetDeleteDialog items={dialog.items} busy={Boolean(busy)} returnFocus={dialog.trigger} onClose={() => setDialog(null)} onConfirm={() => void runBatchMutation(dialog.items, async (item) => {
          if (item.kind === "folder") {
            if (!gateway.deleteCloudProjectAssetFolder) throw new Error("当前 Cloud 暂不支持删除文件夹");
            await gateway.deleteCloudProjectAssetFolder({ projectId, folderId: item.id, expectedVersion: item.version });
          } else {
            await gateway.deleteCloudProjectAsset(projectId, item.id);
          }
      })} /> : null}
      {operationRecordsTrigger ? <AssetOperationRecordsDialog projectId={projectId} projectScopeKey={projectScopeKey} gateway={gateway} returnFocus={operationRecordsTrigger} onClose={() => setOperationRecordsTrigger(null)} /> : null}
    </section>
  );
}

function AssetOperationRecordsDialog({ projectId, projectScopeKey, gateway, returnFocus, onClose }: { projectId: string; projectScopeKey: string; gateway: ProjectGateway; returnFocus: HTMLElement; onClose: () => void }) {
  const { showError } = useToast();
  const titleId = useId();
  const closeRef = useRef<HTMLButtonElement>(null);
  const requestGeneration = useRef(0);
  const [operation, setOperation] = useState<"" | CloudProjectAssetOperation>("");
  const [resourceKind, setResourceKind] = useState<"" | CloudProjectAssetOperationResourceKind>("");
  const [items, setItems] = useState<CloudProjectAssetOperationRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    const generation = requestGeneration.current + 1;
    requestGeneration.current = generation;
    setLoading(true);
    setError("");
    if (!gateway.listCloudProjectAssetOperationRecords) {
      setItems([]);
      setError("当前 Cloud 暂不支持资产操作记录");
      setLoading(false);
      return;
    }
    void gateway.listCloudProjectAssetOperationRecords({
      projectId,
      operation: operation || undefined,
      resourceKind: resourceKind || undefined,
      limit: OPERATION_RECORD_LIMIT,
    }).then((page) => {
      if (generation !== requestGeneration.current || page.projectScopeKey !== projectScopeKey || page.projectId !== projectId) return;
      setItems(page.items);
    }).catch((reason) => {
      if (generation === requestGeneration.current) {
        const message = assetError(reason);
        setError(message);
        showError("projects.operation", reason, message);
      }
    }).finally(() => {
      if (generation === requestGeneration.current) setLoading(false);
    });
    return () => { requestGeneration.current += 1; };
  }, [gateway, operation, projectId, projectScopeKey, refreshKey, resourceKind, showError]);

  return <ProjectModalSurface backdropClassName="project-assets__dialog-backdrop" className="project-assets__dialog project-assets__dialog--records" titleId={titleId} pending={false} returnFocus={returnFocus} initialFocusRef={closeRef} onClose={onClose}>
    <header><div><h3 id={titleId}>资产操作记录</h3><span>查看文件和文件夹由谁在何时进行了操作</span></div><button ref={closeRef} type="button" aria-label="关闭操作记录" onClick={onClose}><X size={17} aria-hidden="true" /></button></header>
    <div className="project-assets__record-filters">
      <ProjectSelect ariaLabel="操作类型" value={operation} options={OPERATION_FILTER_OPTIONS} onChange={setOperation} />
      <ProjectSelect ariaLabel="资源类型" value={resourceKind} options={RESOURCE_KIND_FILTER_OPTIONS} onChange={setResourceKind} />
      <button type="button" aria-label="刷新操作记录" disabled={loading} onClick={() => setRefreshKey((value) => value + 1)}><RefreshCw className={loading ? "spin" : undefined} size={15} aria-hidden="true" /></button>
    </div>
    <div className="project-assets__record-content">
      {error ? <p role="alert" className="cloud-projects__inline-error">{error}</p> : null}
      {loading ? <p role="status" className="project-assets__record-empty"><LoaderCircle className="spin" size={18} aria-hidden="true" />正在加载操作记录…</p> : null}
      {!loading && !error && items.length === 0 ? <p className="project-assets__record-empty">暂无符合条件的操作记录</p> : null}
      {!loading && !error && items.length > 0 ? <ol className="project-assets__record-list">{items.map((item) => <li key={item.operationId}>
        <span className={`project-assets__record-icon is-${item.operation}`} aria-hidden="true"><AssetOperationIcon item={item} /></span>
        <div><p><strong>{item.actorDisplayName}</strong> <span>{assetOperationLabel(item.operationDetail)}</span> <b title={item.resourceName}>{item.resourceName || assetResourceKindLabel(item.resourceKind)}</b></p><small>{assetResourceKindLabel(item.resourceKind)}{item.resourceVersion ? ` · 版本 ${item.resourceVersion}` : ""}<span aria-hidden="true"> · </span><span title={`用户 ID：${item.actorUserId}`}>{item.actorUserId}</span></small></div>
        <time dateTime={item.createdAt}>{formatProjectDateTime(item.createdAt, "-")}</time>
      </li>)}</ol> : null}
    </div>
  </ProjectModalSurface>;
}

function AssetOperationIcon({ item }: { item: CloudProjectAssetOperationRecord }) {
  if (item.operation === "delete") return <Trash2 size={15} />;
  if (item.operation === "update") return <Pencil size={15} />;
  if (item.operationDetail === "download") return <Download size={15} />;
  if (item.resourceKind === "folder" || item.resourceKind === "directory") return <Folder size={15} />;
  return <FileUp size={15} />;
}

function assetOperationLabel(detail: CloudProjectAssetOperationRecord["operationDetail"]) {
  return ({ upload: "上传了", create_folder: "创建了文件夹", list_directory: "查看了目录", list_assets: "查看了文件列表", download: "下载了", replace_content: "替换了文件内容", update_metadata: "重命名或移动了", update_folder: "重命名或移动了文件夹", delete_asset: "删除了", delete_folder: "删除了文件夹" } as const)[detail];
}

function assetResourceKindLabel(kind: CloudProjectAssetOperationResourceKind) {
  return ({ asset: "文件", folder: "文件夹", directory: "目录" } as const)[kind];
}

function SortIcon({ active, direction }: { active: boolean; direction: SortDirection }) {
  if (!active) return <ArrowDown size={12} className="is-muted" aria-hidden="true" />;
  return direction === "asc" ? <ArrowUp size={12} aria-hidden="true" /> : <ArrowDown size={12} aria-hidden="true" />;
}

function AssetIcon({ item }: { item: CloudProjectAssetDirectoryItem }) {
  if (item.kind === "folder") return <Folder size={18} className="is-folder" aria-hidden="true" />;
  const type = item.contentType || "";
  if (type.startsWith("image/")) return <FileImage size={18} className="is-image" aria-hidden="true" />;
  if (type.startsWith("audio/")) return <FileAudio size={18} className="is-audio" aria-hidden="true" />;
  if (type.startsWith("video/")) return <FileVideo size={18} className="is-video" aria-hidden="true" />;
  if (/zip|gzip|rar|tar|7z/.test(type)) return <Archive size={18} className="is-archive" aria-hidden="true" />;
  if (/pdf|text|markdown|document|sheet|presentation/.test(type)) return <FileText size={18} className="is-document" aria-hidden="true" />;
  return <File size={18} aria-hidden="true" />;
}

function AssetNameDialog({ dialog, busy, onClose, onSubmit }: { dialog: Extract<AssetDialog, { kind: "create-folder" | "rename" }>; busy: boolean; onClose: () => void; onSubmit: (name: string) => void }) {
  const titleId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const [name, setName] = useState(dialog.kind === "rename" ? dialog.item.name : "");
  return <ProjectModalSurface backdropClassName="project-assets__dialog-backdrop" className="project-assets__dialog" titleId={titleId} pending={busy} returnFocus={dialog.trigger} initialFocusRef={inputRef} onClose={onClose}><header><h3 id={titleId}>{dialog.kind === "create-folder" ? "新建文件夹" : "重命名"}</h3></header><form onSubmit={(event) => { event.preventDefault(); const value = name.trim(); if (value) onSubmit(value); }}><label>名称<input ref={inputRef} value={name} maxLength={120} onChange={(event) => setName(event.currentTarget.value)} /></label><footer><button type="button" disabled={busy} onClick={onClose}>取消</button><button type="submit" className="is-primary" disabled={busy || !name.trim()}>{busy ? "处理中…" : "确定"}</button></footer></form></ProjectModalSurface>;
}

function AssetDeleteDialog({ items, busy, returnFocus, onClose, onConfirm }: { items: CloudProjectAssetDirectoryItem[]; busy: boolean; returnFocus: HTMLElement | null; onClose: () => void; onConfirm: () => void }) {
  const titleId = useId();
  const cancelRef = useRef<HTMLButtonElement>(null);
  return <ProjectModalSurface backdropClassName="project-assets__dialog-backdrop" className="project-assets__dialog" role="alertdialog" titleId={titleId} pending={busy} returnFocus={returnFocus} initialFocusRef={cancelRef} onClose={onClose}><header><h3 id={titleId}>删除资产</h3></header><p>确定删除{items.length === 1 ? `“${items[0]?.name}”` : `已选择的 ${items.length} 项`}吗？正在被事项或动态引用的文件，以及非空文件夹不会被删除。</p><footer><button ref={cancelRef} type="button" disabled={busy} onClick={onClose}>取消</button><button type="button" className="is-danger" disabled={busy} onClick={onConfirm}>{busy ? "处理中…" : "删除"}</button></footer></ProjectModalSurface>;
}

function AssetFolderPickerDialog({ projectId, gateway, items, busy, returnFocus, onClose, onConfirm }: { projectId: string; gateway: ProjectGateway; items: CloudProjectAssetDirectoryItem[]; busy: boolean; returnFocus: HTMLElement | null; onClose: () => void; onConfirm: (folderId: string) => void }) {
  const { showError } = useToast();
  const titleId = useId();
  const [currentFolderId, setCurrentFolderId] = useState("");
  const [folders, setFolders] = useState<CloudProjectAssetDirectoryItem[]>([]);
  const [crumbs, setCrumbs] = useState<CloudProjectAssetBreadcrumb[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const cancelRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    let active = true;
    setLoading(true); setError("");
    if (!gateway.listCloudProjectAssetDirectory) { setError("当前 Cloud 暂不支持目录选择"); setLoading(false); return; }
    void gateway.listCloudProjectAssetDirectory({ projectId, folderId: currentFolderId || undefined, type: "folder", sort: "name", direction: "asc", limit: 100 }).then((page) => {
      if (!active) return;
      setFolders(page.items.filter((item) => item.kind === "folder" && !items.some((selected) => selected.kind === "folder" && selected.id === item.id)));
      setCrumbs(page.breadcrumbs || []);
    }).catch((reason) => {
      if (active) {
        const message = assetError(reason);
        setError(message);
        showError("projects.operation", reason, message);
      }
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [currentFolderId, gateway, items, projectId, showError]);
  return <ProjectModalSurface backdropClassName="project-assets__dialog-backdrop" className="project-assets__dialog project-assets__dialog--picker" titleId={titleId} pending={busy} returnFocus={returnFocus} initialFocusRef={cancelRef} onClose={onClose}><header><h3 id={titleId}>移动到文件夹</h3><button type="button" aria-label="关闭" disabled={busy} onClick={onClose}><X size={17} /></button></header><nav aria-label="目标目录"><button type="button" onClick={() => setCurrentFolderId("")}>资产</button>{crumbs.map((crumb) => <span key={crumb.folderId}><ChevronRight size={13} /><button type="button" onClick={() => setCurrentFolderId(crumb.folderId)}>{crumb.name}</button></span>)}</nav>{error ? <p role="alert">{error}</p> : loading ? <p role="status">正在加载…</p> : folders.length === 0 ? <p>当前目录没有子文件夹。</p> : <ul>{folders.map((folder) => <li key={folder.id}><button type="button" onClick={() => setCurrentFolderId(folder.id)}><Folder size={17} className="is-folder" /><span>{folder.name}</span><ChevronRight size={14} /></button></li>)}</ul>}<footer><button ref={cancelRef} type="button" disabled={busy} onClick={onClose}>取消</button><button type="button" className="is-primary" disabled={busy || Boolean(error)} onClick={() => onConfirm(currentFolderId)}>{busy ? "处理中…" : "移动到此处"}</button></footer></ProjectModalSurface>;
}

function legacyDirectoryPage(projectScopeKey: string, projectId: string, assets: CloudProjectAsset[]): CloudProjectAssetDirectoryPage {
  const items: CloudProjectAssetDirectoryItem[] = assets.map((asset) => ({ kind: "asset", id: asset.assetId, name: asset.filename, folderId: null, contentType: asset.contentType, sizeBytes: asset.sizeBytes, version: asset.version, updatedBy: undefined, updatedAt: asset.updatedAt }));
  return { projectScopeKey, projectId, breadcrumbs: [], items, totalCount: items.length, nextCursor: null, usageBytes: items.reduce((sum, item) => sum + (item.sizeBytes || 0), 0), quotaBytes: 0, quotaPlan: "" };
}

function mergeDirectoryItems(current: CloudProjectAssetDirectoryItem[], incoming: CloudProjectAssetDirectoryItem[]) {
  const seen = new Set(current.map((item) => `${item.kind}:${item.id}`));
  return [...current, ...incoming.filter((item) => !seen.has(`${item.kind}:${item.id}`))];
}

function statusLabel(value: string) {
  return ({ pending: "待处理", scanning: "扫描中", clean: "安全", rejected: "已拒绝", failed_retryable: "失败（可重试）", not_started: "未开始", not_applicable: "不适用", processing: "处理中", ready: "就绪", active: "有效", cleanup_pending: "待清理", purged: "已清除" } as Record<string, string>)[value] ?? value;
}

function itemTypeLabel(item: CloudProjectAssetDirectoryItem) {
  if (item.kind === "folder") return "文件夹";
  const type = (item.contentType || "").toLowerCase();
  if (type.includes("markdown")) return "Markdown";
  if (type.includes("pdf")) return "PDF";
  if (type.startsWith("image/")) return "图片";
  if (type.startsWith("audio/")) return "音频";
  if (type.startsWith("video/")) return "视频";
  if (/zip|gzip|rar|tar|7z/.test(type)) return "压缩包";
  if (/text|document|sheet|presentation/.test(type)) return "文档";
  return type || "文件";
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${trimNumber(value / 1024)} KB`;
  if (value < 1024 * 1024 * 1024) return `${trimNumber(value / 1024 / 1024)} MB`;
  return `${trimNumber(value / 1024 / 1024 / 1024)} GB`;
}

function trimNumber(value: number) {
  return value >= 10 ? value.toFixed(1).replace(/\.0$/, "") : value.toFixed(2).replace(/0+$/, "").replace(/\.$/, "");
}

function assetError(error: unknown) {
  const message = clientErrorMessageWithoutDiagnosticID(error);
  if (/quota|storage_limit|too large|413/i.test(message)) return "存储空间不足，请升级套餐或删除不再需要的文件。";
  if (/in use|asset_in_use/i.test(message)) return "该文件仍被事项或动态引用，请先取消关联。";
  if (/folder.*not empty|directory.*not empty/i.test(message)) return "文件夹不为空，请先移动或删除其中内容。";
  if (/conflict|version/i.test(message)) return "内容已被其他成员更新，列表已刷新，请重试。";
  return message.trim() || "项目资产操作失败，请重试。";
}
