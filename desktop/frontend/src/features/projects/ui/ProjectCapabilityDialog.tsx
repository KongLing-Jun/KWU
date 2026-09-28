import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Search } from "lucide-react";
import { clientErrorCode } from "../../../lib/clientError";
import { useToast } from "../../../lib/toast";
import type { ProjectGateway } from "../data/projectGateway";
import type { CloudProjectCapabilityBinding } from "../model/projectTypes";
import type { ProjectCapabilityKind } from "./CloudProjectsSectionImpl";
import { installMissingBoundProjectSkills, loadProjectCapabilityCatalog, type ProjectCapabilityCatalogItem } from "./projectCapabilityCatalog";
import { ProjectCapabilityIcon } from "./ProjectCapabilityIcon";
import { ProjectModalSurface } from "./ProjectModalSurface";

export type { ProjectCapabilityCatalogItem } from "./projectCapabilityCatalog";

export function projectCapabilityOptionDisabled(item: ProjectCapabilityCatalogItem, checked: boolean): boolean {
  if (item.connectable !== undefined) return item.connectable === false && !checked;
  return item.available === false && item.installable !== true && !checked;
}
const labels: Record<ProjectCapabilityKind, string> = {
  connector: "连接器",
  expert: "专家",
  skill: "技能",
};

const emptyDraftSelection: ProjectCapabilityCatalogItem[] = [];

export function ProjectCapabilityDialog({ kind, projectId, gateway, experts, mode = "project", draftSelectedItems = emptyDraftSelection, returnFocus, onClose, onApplied, onCatalogLoaded, onDraftApplied }: {
  kind: ProjectCapabilityKind;
  projectId: string;
  gateway: ProjectGateway;
  experts: ProjectCapabilityCatalogItem[];
  mode?: "project" | "draft";
  draftSelectedItems?: ProjectCapabilityCatalogItem[];
  returnFocus?: HTMLElement | null;
  onClose: () => void;
  onApplied?: (items: CloudProjectCapabilityBinding[]) => void;
  onCatalogLoaded?: (kind: ProjectCapabilityKind, items: ProjectCapabilityCatalogItem[]) => void;
  onDraftApplied?: (items: ProjectCapabilityCatalogItem[]) => void;
}) {
  const { showError, showToast } = useToast();
  const [catalog, setCatalog] = useState<ProjectCapabilityCatalogItem[]>([]);
  const [bindings, setBindings] = useState<CloudProjectCapabilityBinding[]>([]);
  const [projectVersion, setProjectVersion] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const [source, setSource] = useState(kind === "connector" ? "configured" : "recommended");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [loadFailed, setLoadFailed] = useState(false);
  const onCatalogLoadedRef = useRef(onCatalogLoaded);
  const label = labels[kind];

  useEffect(() => {
    onCatalogLoadedRef.current = onCatalogLoaded;
  }, [onCatalogLoaded]);

  useEffect(() => {
    setSource(kind === "connector" ? "configured" : "recommended");
    setCategory("all");
  }, [kind]);

  useEffect(() => {
    let active = true;
    void (async () => {
      setLoading(true);
      setError("");
      setLoadFailed(false);
      try {
        const capability = mode === "draft" ? null : await gateway.listCloudProjectCapabilities(projectId);
        const boundRegistryRefs = capability?.items
          .filter((item) => item.kind === kind)
          .map((item) => item.registryRef) ?? [];
        const nextCatalog = await loadProjectCapabilityCatalog(kind, gateway, experts, boundRegistryRefs);
        if (!active) return;
        if (capability) {
          const current = capability.items.filter((item) => item.kind === kind);
          setBindings(capability.items);
          setProjectVersion(capability.projectVersion);
          setSelected(new Set(current.map((item) => item.registryRef)));
        } else if (mode === "draft") {
          setSelected(new Set(draftSelectedItems.map((item) => item.id)));
        }
        setCatalog(nextCatalog);
        onCatalogLoadedRef.current?.(kind, nextCatalog);
      } catch (reason) {
        if (active) {
          setLoadFailed(true);
          const message = capabilityLoadError(kind, reason);
          setError(message);
          showError("projects.operation", reason, message);
        }
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => { active = false; };
  }, [draftSelectedItems, experts, gateway, kind, mode, projectId, showError]);

  const filtered = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase();
    return catalog.filter((item) => {
      if (item.sources && !item.sources.includes(source)) return false;
      if (kind === "skill" && category !== "all" && item.category !== category) return false;
      return !normalized || `${item.name} ${item.description ?? ""}`.toLocaleLowerCase().includes(normalized);
    });
  }, [catalog, category, kind, query, source]);
  const categories = useMemo(() => [...new Set(catalog.map((item) => item.category).filter((value): value is string => Boolean(value)))].slice(0, 10), [catalog]);

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      if (mode === "draft") {
        onDraftApplied?.(catalog.filter((item) => selected.has(item.id)));
        onClose();
        return;
      }
      const result = await gateway.saveCloudProjectCapabilities({
        projectId,
        expectedVersion: projectVersion,
        bindings: [
          ...bindings.filter((item) => item.kind !== kind).map((item) => ({
            kind: item.kind,
            registryRef: item.registryRef,
            position: item.position,
            config: item.config,
          })),
          ...[...selected].map((registryRef, position) => ({
            kind,
            registryRef,
            position,
            config: bindings.find((item) => item.kind === kind && item.registryRef === registryRef)?.config ?? {},
          })),
        ],
      });
      onApplied?.(result.items);
      if (kind === "skill") {
        const install = await installMissingBoundProjectSkills(result.items, gateway);
        if (install.installed.length > 0) showToast(`已保存到云端并自动安装技能：${install.installed.join("、")}`, "info");
        if (install.failed.length > 0) showToast(`技能已保存到云端，但本机自动安装失败：${install.failed.join("、")}`, "warn", { durationMs: 6_000 });
      }
      onClose();
    } catch (reason) {
      const message = capabilitySaveError(kind, reason);
      setError(message);
      showError("projects.operation", reason, message);
    } finally {
      setSaving(false);
    }
  };

  return <ProjectModalSurface backdropClassName="project-capability-dialog__backdrop" className="project-capability-dialog" titleId="project-capability-dialog-title" pending={saving} showClose initialFocus="close" returnFocus={returnFocus} onClose={onClose}>
      <header>
        <div><h2 id="project-capability-dialog-title">选择{label}</h2><p>{mode === "draft" ? `为新项目选择已有${label}；创建项目后统一保存到云端。` : kind === "connector" ? "为当前项目配置连接器；连接状态和全局启用状态单独管理。" : `为当前项目复用已有${label}；配置保存在云端，实际执行仍遵循本机权限与审批。`}</p></div>
      </header>
      <div className="project-capability-dialog__toolbar">
        <label><Search size={16} /><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={`搜索${label}`} /></label>
        <span>已选择 {selected.size} 项</span>
      </div>
      <div className="project-capability-dialog__tabs" role="tablist" aria-label={`${label}来源`}>
        {kind === "connector" ? <><button type="button" role="tab" aria-selected={source === "configured"} onClick={() => setSource("configured")}>已配置</button><button type="button" role="tab" aria-selected={source === "all"} onClick={() => setSource("all")}>全部连接器</button></> : null}
        {kind === "expert" ? <><button type="button" role="tab" aria-selected={source === "recommended"} onClick={() => setSource("recommended")}>推荐专家</button><button type="button" role="tab" aria-selected={source === "mine"} onClick={() => setSource("mine")}>我的专家</button></> : null}
        {kind === "skill" ? <><button type="button" role="tab" aria-selected={source === "recommended"} onClick={() => setSource("recommended")}>推荐</button><button type="button" role="tab" aria-selected={source === "installed"} onClick={() => setSource("installed")}>已安装</button><button type="button" role="tab" aria-selected={source === "skillhub"} onClick={() => setSource("skillhub")}>SkillHub</button></> : null}
      </div>
      {kind === "skill" && categories.length ? <div className="project-capability-dialog__categories" role="tablist" aria-label="技能分类">
        <button type="button" role="tab" aria-selected={category === "all"} onClick={() => setCategory("all")}>全部</button>
        {categories.map((item) => <button key={item} type="button" role="tab" aria-selected={category === item} onClick={() => setCategory(item)}>{item}</button>)}
      </div> : null}
      {kind === "connector" && [...selected].some((id) => {
        const item = catalog.find((candidate) => candidate.id === id);
        return item?.available === false && item.connectable !== false;
      }) ? <div className="project-capability-dialog__connector-notice" role="status">
        <span>连接器可以先保存到项目；无论是否已连接，都不会自动出现在对话输入框。</span>
      </div> : null}
      <div className="project-capability-dialog__content" aria-busy={loading}>
        {loading ? <p className="project-capability-dialog__empty">正在加载…</p> : loadFailed ? null : filtered.length === 0 ? <p className="project-capability-dialog__empty">{capabilityEmptyMessage(kind, source, label, query)}</p> : <div className="project-capability-dialog__grid">
          {filtered.map((item) => {
            const checked = selected.has(item.id);
            const disabled = projectCapabilityOptionDisabled(item, checked);
            return <button key={item.id} type="button" className={checked ? "is-selected" : ""} aria-pressed={checked} disabled={disabled} title={disabled ? item.availabilityReason : undefined} onClick={() => {
              setSelected((current) => {
                const next = new Set(current);
                if (checked) next.delete(item.id);
                else next.add(item.id);
                return next;
              });
            }}>
              <span className="project-capability-dialog__check">{checked ? <Check size={14} /> : null}</span>
              <span>
                <span className="project-capability-dialog__identity"><ProjectCapabilityIcon kind={kind} iconUrl={item.iconUrl} /><strong>{item.name}</strong><small>{kind === "expert" && item.mode === "team" ? `专家团 · ${item.memberCount ?? 0} 位成员` : item.meta}</small></span>
                <p>{item.description || item.availabilityReason || "暂无说明"}</p>
              </span>
            </button>;
          })}
        </div>}
      </div>
      {error ? <p className="project-capability-dialog__error" role="alert">{error}</p> : null}
      <footer><button type="button" onClick={onClose}>取消</button><button type="button" className="project-capability-dialog__primary" disabled={loading || loadFailed || saving} onClick={() => void save()}>{saving ? "保存中…" : mode === "draft" ? "确定" : "保存"}</button></footer>
  </ProjectModalSurface>;
}

function capabilityEmptyMessage(kind: ProjectCapabilityKind, source: string, label: string, query: string): string {
  const normalizedQuery = query.trim();
  if (normalizedQuery) return `没有匹配“${normalizedQuery}”的${label}，请调整搜索关键词。`;
  if (kind === "connector") {
    return source === "configured"
      ? "当前项目还没有配置连接器。可切换到“全部连接器”添加。"
      : "Cloud Catalog 当前没有可用连接器，请稍后重试。";
  }
  if (kind === "expert") {
    return source === "mine"
      ? "你还没有可用于项目的个人专家。请先在专家中心添加专家。"
      : "当前没有推荐专家。请稍后重试。";
  }
  if (kind === "skill") {
    if (source === "installed") return "当前设备没有已安装的技能。请先安装技能，再返回此处选择。";
    if (source === "skillhub") return "SkillHub 当前没有可用技能。请检查网络后重试。";
    return "当前没有推荐技能。请稍后重试。";
  }
  return `当前没有可用${label}。`;
}

function capabilityLoadError(kind: ProjectCapabilityKind, error: unknown): string {
  if (clientErrorCode(error) === "PERMISSION_DENIED") return `你没有查看或配置当前项目${labels[kind]}的权限。`;
  return `${labels[kind]}加载失败。请关闭窗口后重试。`;
}

function capabilitySaveError(kind: ProjectCapabilityKind, error: unknown): string {
  const label = labels[kind];
  switch (clientErrorCode(error)) {
    case "CONFLICT": return `项目${label}已被其他成员更新。当前选择已保留，请刷新项目后重试。`;
    case "PERMISSION_DENIED": return `你当前没有配置项目${label}的权限。当前选择已保留。`;
    case "VALIDATION_FAILED":
    case "INVALID_ARGUMENT": return `部分${label}已失效或不可用于云项目。当前选择已保留，请重新选择。`;
    case "NETWORK_UNAVAILABLE": return `网络不可用，未能保存项目${label}。当前选择已保留，请联网后重试。`;
    default: return error instanceof Error && error.message.trim() ? error.message : `保存项目${label}失败，当前选择已保留。`;
  }
}
