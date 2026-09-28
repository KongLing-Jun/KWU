import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Clock3,
  ExternalLink,
  File,
  FileAudio2,
  FileCode2,
  FileImage,
  FileSpreadsheet,
  FileText,
  FileVideo2,
  Folder,
  Library,
  MessageSquare,
  PanelLeftClose,
  PanelLeftOpen,
  Presentation,
  RefreshCw,
  Search,
} from "lucide-react";
import { CommandPalette, type PaletteItem } from "../../components/CommandPalette";
import { app } from "../../lib/bridge";
import { useT } from "../../lib/i18n";
import type { ArtifactView, SessionMeta } from "../../lib/types";
import { openWorkspacePathForOwner, resolveWorkspaceFileOwner } from "../../lib/workspaceFileScope";
import "./artifact-library.css";

type ArtifactSection = "recent" | "local";

interface ArtifactLibraryPageProps {
  onOpenSource: (session: SessionMeta) => void | Promise<void>;
  accountScopeKey: string;
  refreshSignal: number;
}

interface ArtifactLoadState {
  queryKey: string;
  artifacts: ArtifactView[];
  loading: boolean;
  error: string;
}

const sectionIcons = {
  recent: Clock3,
  local: Folder,
} as const;

function sourceTitle(artifact: ArtifactView): string {
  return artifact.sessionTitle?.trim()
    || artifact.session.title?.trim()
    || artifact.session.topicTitle?.trim()
    || artifact.session.preview?.trim()
    || "未命名会话";
}

function normalizedType(artifact: ArtifactView): string {
  return artifact.type?.trim().toLowerCase() || "other";
}

function typeLabel(type: string): string {
  const labels: Record<string, string> = {
    document: "文档",
    spreadsheet: "表格",
    presentation: "幻灯片",
    pdf: "PDF",
    image: "图片",
    video: "视频",
    audio: "音频",
    markdown: "Markdown",
    code: "代码",
    other: "其他",
  };
  return labels[type] || type;
}

function ArtifactIcon({ artifact }: { artifact: ArtifactView }) {
  const type = normalizedType(artifact);
  const Icon = type === "image"
    ? FileImage
    : type === "video"
      ? FileVideo2
      : type === "audio"
        ? FileAudio2
        : type === "spreadsheet"
          ? FileSpreadsheet
          : type === "presentation"
            ? Presentation
            : type === "code"
              ? FileCode2
              : type === "document" || type === "markdown" || type === "pdf"
                ? FileText
                : File;
  return <Icon size={19} aria-hidden="true" />;
}

function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

function formatTime(timestamp: number): string {
  if (!timestamp) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(timestamp);
}

function ArtifactRow({
  artifact,
  onOpenFile,
  onOpenSource,
}: {
  artifact: ArtifactView;
  onOpenFile: (artifact: ArtifactView) => void;
  onOpenSource: (session: SessionMeta) => void | Promise<void>;
}) {
  const t = useT();
  return (
    <article className="artifact-library__row">
      <button
        type="button"
        className="artifact-library__file-icon"
        aria-label={`${t("artifacts.openFile")}：${artifact.name}`}
        onClick={() => onOpenFile(artifact)}
      >
        <ArtifactIcon artifact={artifact} />
      </button>
      <div className="artifact-library__file-copy">
        <button type="button" className="artifact-library__file-name" onClick={() => onOpenFile(artifact)}>
          {artifact.name}
        </button>
        <span className="artifact-library__file-path" title={artifact.path}>{artifact.path}</span>
      </div>
      <button
        type="button"
        className="artifact-library__source"
        title={t("artifacts.openSource")}
        onClick={() => void onOpenSource(artifact.session)}
      >
        <MessageSquare size={14} aria-hidden="true" />
        <span>{sourceTitle(artifact)}</span>
      </button>
      <span className="artifact-library__time">{formatTime(artifact.deliveredAt)}</span>
      <span className="artifact-library__size">{formatSize(artifact.size)}</span>
      <button
        type="button"
        className="artifact-library__open"
        aria-label={`${t("artifacts.openFile")}：${artifact.name}`}
        title={t("artifacts.openFile")}
        onClick={() => onOpenFile(artifact)}
      >
        <ExternalLink size={15} aria-hidden="true" />
      </button>
    </article>
  );
}

export function ArtifactLibraryPage({ onOpenSource, accountScopeKey, refreshSignal }: ArtifactLibraryPageProps) {
  const t = useT();
  const queryKey = JSON.stringify([accountScopeKey, refreshSignal]);
  const [section, setSection] = useState<ArtifactSection>("recent");
  const [loadState, setLoadState] = useState<ArtifactLoadState>({ queryKey, artifacts: [], loading: true, error: "" });
  const [typeFilter, setTypeFilter] = useState("all");
  const [searchOpen, setSearchOpen] = useState(false);
  const [railCollapsed, setRailCollapsed] = useState(false);
  const searchButtonRef = useRef<HTMLButtonElement>(null);
  const requestSequenceRef = useRef(0);

  // Hide the previous scope immediately, including search results, before the reload effect runs.
  const currentLoadState = loadState.queryKey === queryKey ? loadState : undefined;
  const artifacts = currentLoadState?.artifacts ?? [];
  const loading = currentLoadState?.loading ?? true;
  const error = currentLoadState?.error ?? "";

  const loadArtifacts = useCallback(async () => {
    const requestSequence = ++requestSequenceRef.current;
    setLoadState({ queryKey, artifacts: [], loading: true, error: "" });
    try {
      const result = await app.ListArtifacts();
      if (requestSequence === requestSequenceRef.current) {
        setLoadState({ queryKey, artifacts: Array.isArray(result) ? result : [], loading: false, error: "" });
      }
    } catch {
      if (requestSequence === requestSequenceRef.current) {
        setLoadState({ queryKey, artifacts: [], loading: false, error: t("artifacts.loadFailed") });
      }
    }
  }, [queryKey, t]);

  useEffect(() => {
    void loadArtifacts();
    return () => { requestSequenceRef.current += 1; };
  }, [loadArtifacts]);

  const sortedArtifacts = useMemo(
    () => [...artifacts].sort((left, right) => right.deliveredAt - left.deliveredAt),
    [artifacts],
  );

  const availableTypes = useMemo(
    () => [...new Set(artifacts.map(normalizedType))].sort((left, right) => typeLabel(left).localeCompare(typeLabel(right), "zh-CN")),
    [artifacts],
  );

  const localArtifacts = useMemo(
    () => typeFilter === "all" ? sortedArtifacts : sortedArtifacts.filter((artifact) => normalizedType(artifact) === typeFilter),
    [sortedArtifacts, typeFilter],
  );

  const groupedArtifacts = useMemo(() => {
    const workspaces = new Map<string, Map<string, { title: string; artifacts: ArtifactView[] }>>();
    for (const artifact of localArtifacts) {
      const workspace = artifact.workspaceName?.trim() || t("artifacts.unknownWorkspace");
      const sessions = workspaces.get(workspace) ?? new Map<string, { title: string; artifacts: ArtifactView[] }>();
      const sessionKey = artifact.session.path;
      const session = sessions.get(sessionKey) ?? { title: sourceTitle(artifact), artifacts: [] };
      session.artifacts.push(artifact);
      sessions.set(sessionKey, session);
      workspaces.set(workspace, sessions);
    }
    return [...workspaces.entries()];
  }, [localArtifacts, t]);

  const openFile = useCallback((artifact: ArtifactView) => {
    setLoadState((current) => current.queryKey === queryKey ? { ...current, error: "" } : current);
    const owner = resolveWorkspaceFileOwner(undefined, { workspaceRoot: artifact.session.workspaceRoot });
    void openWorkspacePathForOwner(owner, artifact.path).catch(() => {
      setLoadState((current) => current.queryKey === queryKey ? { ...current, error: t("artifacts.openFailed") } : current);
    });
  }, [queryKey, t]);

  const searchItems = useMemo<PaletteItem[]>(() => sortedArtifacts.map((artifact) => ({
    id: `${artifact.session.path}:${artifact.path}`,
    title: artifact.name,
    hint: [sourceTitle(artifact), artifact.workspaceName].filter(Boolean).join(" · "),
    meta: formatTime(artifact.deliveredAt),
    icon: <ArtifactIcon artifact={artifact} />,
    group: t("artifacts.recent"),
    keywords: [sourceTitle(artifact), artifact.workspaceName],
    run: () => openFile(artifact),
  })), [openFile, sortedArtifacts, t]);

  const closeSearch = useCallback(() => {
    setSearchOpen(false);
    requestAnimationFrame(() => searchButtonRef.current?.focus());
  }, []);

  const sectionTitle = section === "recent" ? t("artifacts.recent") : t("artifacts.local");
  const visibleCount = section === "local" ? localArtifacts.length : sortedArtifacts.length;

  return (
    <section className={`artifact-library${railCollapsed ? " artifact-library--rail-collapsed" : ""}`} aria-label={t("artifacts.title")}>
      <aside id="artifact-library-navigation" className="artifact-library__rail" aria-hidden={railCollapsed}>
        <div className="artifact-library__brand">
          <Library size={21} aria-hidden="true" />
          <h1>{t("artifacts.title")}</h1>
        </div>
        <nav className="artifact-library__nav" aria-label={t("artifacts.navigation") }>
          <button
            ref={searchButtonRef}
            type="button"
            className="artifact-library__nav-item"
            aria-haspopup="dialog"
            aria-expanded={searchOpen}
            onClick={() => setSearchOpen(true)}
          >
            <Search size={17} aria-hidden="true" />
            <span>{t("artifacts.search")}</span>
          </button>
          {(["recent", "local"] as const).map((item) => {
            const Icon = sectionIcons[item];
            const selected = section === item;
            const label = item === "recent" ? t("artifacts.recent") : t("artifacts.local");
            return (
              <button
                key={item}
                type="button"
                className={`artifact-library__nav-item${selected ? " artifact-library__nav-item--active" : ""}`}
                aria-current={selected ? "page" : undefined}
                onClick={() => setSection(item)}
              >
                <Icon size={17} aria-hidden="true" />
                <span>{label}</span>
              </button>
            );
          })}
        </nav>
      </aside>

      <div className="artifact-library__content">
        <div className="artifact-library__rail-toggle-row">
          <button
            type="button"
            className="artifact-library__rail-toggle"
            aria-label={railCollapsed ? t("artifacts.expandNavigation") : t("artifacts.collapseNavigation")}
            aria-expanded={!railCollapsed}
            aria-controls="artifact-library-navigation"
            onClick={() => setRailCollapsed((collapsed) => !collapsed)}
          >
            {railCollapsed ? <PanelLeftOpen size={17} aria-hidden="true" /> : <PanelLeftClose size={17} aria-hidden="true" />}
          </button>
        </div>
        <header className="artifact-library__header">
          <div>
            <h2>{sectionTitle}</h2>
            <p>{t("artifacts.count", { count: visibleCount })}</p>
          </div>
          <button type="button" className="artifact-library__refresh" disabled={loading} onClick={() => void loadArtifacts()}>
            <RefreshCw size={15} className={loading ? "artifact-library__spin" : ""} aria-hidden="true" />
            <span>{t("artifacts.refresh")}</span>
          </button>
        </header>

        {section === "local" ? (
          <div className="artifact-library__toolbar">
            <label htmlFor="artifact-type-filter">{t("artifacts.type")}</label>
            <select id="artifact-type-filter" value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)}>
              <option value="all">{t("artifacts.typeAll")}</option>
              {availableTypes.map((type) => <option key={type} value={type}>{typeLabel(type)}</option>)}
            </select>
          </div>
        ) : null}

        {error ? <div className="artifact-library__notice" role="alert">{error}</div> : null}

        <div className="artifact-library__body">
          {loading ? (
            <div className="artifact-library__empty" role="status">{t("common.loading")}</div>
          ) : section === "local" ? (
            groupedArtifacts.length > 0 ? groupedArtifacts.map(([workspace, sessions]) => (
              <section key={workspace} className="artifact-library__workspace">
                <h3><Folder size={16} aria-hidden="true" />{workspace}</h3>
                {[...sessions.entries()].map(([sessionPath, group]) => (
                  <div key={sessionPath} className="artifact-library__session-group">
                    <div className="artifact-library__session-heading">
                      <MessageSquare size={14} aria-hidden="true" />
                      <span>{group.title}</span>
                      <small>{t("artifacts.count", { count: group.artifacts.length })}</small>
                    </div>
                    <div className="artifact-library__list">
                      {group.artifacts.map((artifact) => (
                        <ArtifactRow key={`${artifact.session.path}:${artifact.path}`} artifact={artifact} onOpenFile={openFile} onOpenSource={onOpenSource} />
                      ))}
                    </div>
                  </div>
                ))}
              </section>
            )) : <div className="artifact-library__empty">{t("artifacts.empty")}</div>
          ) : (
            sortedArtifacts.length > 0 ? (
              <div className="artifact-library__list">
                {sortedArtifacts.map((artifact) => (
                  <ArtifactRow key={`${artifact.session.path}:${artifact.path}`} artifact={artifact} onOpenFile={openFile} onOpenSource={onOpenSource} />
                ))}
              </div>
            ) : <div className="artifact-library__empty">{t("artifacts.empty")}</div>
          )}
        </div>
      </div>
      <CommandPalette
        open={searchOpen}
        onClose={closeSearch}
        items={searchItems}
        placeholder={t("artifacts.searchPlaceholder")}
        emptyText={t("artifacts.noSearchResults")}
        footerLabels={{
          navigate: t("artifacts.searchNavigate"),
          run: t("artifacts.searchRun"),
          close: t("artifacts.searchClose"),
        }}
      />
    </section>
  );
}

export default ArtifactLibraryPage;
