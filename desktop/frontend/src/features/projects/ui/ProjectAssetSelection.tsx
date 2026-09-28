import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronRight, Folder, Search } from "lucide-react";
import { useToast } from "../../../lib/toast";
import type { ProjectGateway } from "../data/projectGateway";
import type { CloudProjectAssetDirectoryBreadcrumb, CloudProjectAssetDirectoryItem } from "../model/projectTypes";

interface Props {
  projectId: string;
  projectScopeKey: string;
  gateway: ProjectGateway;
  selectedIds: string[];
  maxSelected?: number;
  onChange: (ids: string[]) => void;
  onAssetSeen?: (asset: { assetId: string; filename: string }) => void;
}

export function ProjectAssetSelection({ projectId, projectScopeKey, gateway, selectedIds, maxSelected, onChange, onAssetSeen }: Props) {
  const { showError } = useToast();
  const requestRef = useRef(0);
  const composingRef = useRef(false);
  const [folderId, setFolderId] = useState("");
  const [breadcrumbs, setBreadcrumbs] = useState<CloudProjectAssetDirectoryBreadcrumb[]>([]);
  const [items, setItems] = useState<CloudProjectAssetDirectoryItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [searchDraft, setSearchDraft] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async (cursor = "", append = false) => {
    if (!gateway.listCloudProjectAssetDirectory) {
      setError("项目资产目录暂不可用");
      return;
    }
    const request = ++requestRef.current;
    setLoading(true);
    try {
      const page = await gateway.listCloudProjectAssetDirectory({
        projectId,
        ...(folderId ? { folderId } : {}),
        ...(search ? { search } : {}),
        sort: "name",
        direction: "asc",
        ...(cursor ? { cursor } : {}),
        limit: 50,
      });
      if (requestRef.current !== request || page.projectId !== projectId || page.projectScopeKey !== projectScopeKey) return;
      setBreadcrumbs(page.breadcrumbs);
      setItems((current) => append ? mergeDirectoryItems(current, page.items) : page.items);
      setNextCursor(page.nextCursor);
      setError("");
      for (const item of page.items) if (item.kind === "asset") onAssetSeen?.({ assetId: item.id, filename: item.name });
    } catch (reason) {
      if (requestRef.current === request) {
        setError("项目资产加载失败");
        showError("projects.operation", reason, "项目资产加载失败");
      }
    } finally {
      if (requestRef.current === request) setLoading(false);
    }
  }, [folderId, gateway, onAssetSeen, projectId, projectScopeKey, search, showError]);

  useEffect(() => {
    setFolderId("");
    setSearchDraft("");
    setSearch("");
    setItems([]);
    setNextCursor(null);
    return () => { requestRef.current += 1; };
  }, [projectId, projectScopeKey]);

  useEffect(() => { void load(); }, [load]);

  const applySearch = () => {
    const normalized = searchDraft.trim();
    if (normalized !== search) setSearch(normalized);
  };
  const chooseFolder = (nextFolderId: string) => {
    setFolderId(nextFolderId);
    setSearchDraft("");
    setSearch("");
  };

  return <fieldset className="project-activity__asset-picker project-asset-selection">
    <legend>选择项目资产{maxSelected ? `（最多 ${maxSelected} 个）` : ""}</legend>
    <nav aria-label="项目资产目录"><button type="button" onClick={() => chooseFolder("")}>资产</button>{breadcrumbs.map((crumb) => <span key={crumb.folderId}><ChevronRight size={13} aria-hidden="true" /><button type="button" onClick={() => chooseFolder(crumb.folderId)}>{crumb.name}</button></span>)}</nav>
    <div role="search"><Search size={14} aria-hidden="true" /><input type="search" value={searchDraft} onChange={(event) => setSearchDraft(event.currentTarget.value)} onBlur={applySearch} onCompositionStart={() => { composingRef.current = true; }} onCompositionEnd={() => { composingRef.current = false; }} onKeyDown={(event) => { if (event.key === "Enter" && !composingRef.current) { event.preventDefault(); applySearch(); } }} placeholder="搜索当前目录资产" aria-label="搜索项目资产" /></div>
    {error ? <p role="alert">{error}</p> : null}
    {!error && !loading && items.length === 0 ? <p>当前目录没有资产。</p> : null}
    <div className="project-asset-selection__items">{items.map((item) => item.kind === "folder"
      ? <button key={item.id} type="button" className="project-asset-selection__folder" onClick={() => chooseFolder(item.id)}><Folder size={15} aria-hidden="true" />{item.name}<ChevronRight size={13} aria-hidden="true" /></button>
      : <label key={item.id}><input type="checkbox" checked={selectedIds.includes(item.id)} disabled={!selectedIds.includes(item.id) && maxSelected !== undefined && selectedIds.length >= maxSelected} onChange={(event) => onChange(event.currentTarget.checked ? [...selectedIds, item.id] : selectedIds.filter((id) => id !== item.id))} /><span>{item.name}</span></label>)}</div>
    {nextCursor ? <button type="button" disabled={loading} onClick={() => void load(nextCursor, true)}>{loading ? "加载中…" : "加载更多资产"}</button> : null}
  </fieldset>;
}

function mergeDirectoryItems(current: CloudProjectAssetDirectoryItem[], incoming: CloudProjectAssetDirectoryItem[]) {
  const byID = new Map(current.map((item) => [`${item.kind}:${item.id}`, item]));
  for (const item of incoming) byID.set(`${item.kind}:${item.id}`, item);
  return [...byID.values()];
}
