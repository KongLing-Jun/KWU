import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AlertCircle, BadgeCheck, Blocks, CheckCircle2, ChevronDown, ChevronLeft, ChevronRight, Download, ExternalLink, Library, LoaderCircle, MessageCircle, MoreHorizontal, PackageCheck, Pin, Play, Plus, RefreshCw, Search, Settings, ShieldCheck, ShieldOff, Trash2 } from "lucide-react";
import { asArray } from "../lib/array";
import { app, openExternal, type ConnectorAuthFieldView, type ConnectorCatalogItemView, type ConnectorStateView } from "../lib/bridge";
import { reportClientError } from "../lib/clientError";
import { useT, type Translator } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { mcpServerLifecycleActions, mcpServerRetryableFromAvailableList } from "../lib/mcpServerLifecycle";
import type { ConnectorAvailability } from "../lib/connectorAvailability";
import { announceMCPServerRemoved } from "../lib/composerCapabilities";
import type { CapabilitiesView, MCPConnectionTestResult, MCPServerInput, PublicSkillView, PublicSkillsPageView, ServerView, SkillRootSkillView, SkillRootView, SkillsSettingsView, SkillView, TabMeta } from "../lib/types";
import {
  CONNECTOR_CATALOG,
  registerConnectorCatalog,
  type ConnectorCatalogEntry,
} from "../features/connectors/catalog";
import { hasRecoverableCloudIdentity, useAccountStore } from "../features/account/store/accountStore";
import { isConnectorTargetUnavailable } from "../features/connectors/target";
import { ConnectorBrandIcon } from "../features/connectors/ui/ConnectorBrandIcon";
import { FeishuBotConnector, matchesFeishuConnectorQuery } from "../features/connectors/ui/FeishuBotConnector";
import { InlineConfirmButton } from "./InlineConfirmButton";
import { ResizableDrawer } from "./ResizableDrawer";
import { Tooltip } from "./Tooltip";
import { ModalCloseButton } from "./ModalCloseButton";
import { ContextMenu, type ContextMenuPoint } from "./ContextMenu";
import { useHorizontalDragScroll } from "../hooks/useHorizontalDragScroll";

// CapabilitiesPanel is the desktop MCP & Skills drawer — the GUI counterpart to
// the CLI's /mcp + /skill, aligning with Claude Code's Customize → Connectors:
// each server shows a connected/failed dot, transport, and tool/prompt/resource
// counts, with add / remove / retry; skills list their scope and run mode.
type CapTab = "servers" | "skills";

const PUBLIC_SKILLS_PAGE_SIZE = 20;

type SettingsSnapshot<T> = { key: string; value: T };

let mcpSettingsSnapshot: SettingsSnapshot<ServerView[]> | null = null;

function capabilityErrorMessage(
  scope: string,
  error: unknown,
  fallbackMessage: string,
  knownMessages?: Readonly<Record<string, string>>,
): string {
  return reportClientError(scope, error, { fallbackMessage, knownMessages }).userMessage;
}

let skillsSettingsSnapshot: SettingsSnapshot<SkillsSettingsView> | null = null;

function settingsSnapshotKey(meta: Awaited<ReturnType<typeof app.Meta>> | null | undefined, tabs: TabMeta[] | null | undefined): string {
  const active = tabs?.find((tab) => tab.active);
  const tabID = (active?.id || "").trim();
  const root = (active?.workspaceRoot || active?.workspacePath || active?.cwd || meta?.workspaceRoot || meta?.workspacePath || meta?.cwd || "").trim();
  const channel = (meta?.eventChannel || "").trim();
  return `${channel}|${tabID}|${root}`;
}

export function CapabilitiesPanel({
  onClose,
  initialTab = "servers",
}: {
  onClose: () => void;
  initialTab?: CapTab;
}) {
  const t = useT();
  const [view, setView] = useState<CapabilitiesView | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [tab, setTab] = useState<CapTab>(initialTab);
  const [skillQuery, setSkillQuery] = useState("");
  const [expandedErrors, setExpandedErrors] = useState<Set<string>>(() => new Set());
  const [expandedServers, setExpandedServers] = useState<Set<string>>(() => new Set());
  const [expandedServerTools, setExpandedServerTools] = useState<Set<string>>(() => new Set());

  const reload = useCallback(async () => {
    setView(normalizeCapabilitiesView(await app.Capabilities().catch(() => ({ servers: [], skills: [], skillRoots: [] }))));
  }, []);
  useEffect(() => {
    void reload();
  }, [reload]);
  useEffect(() => {
    if (tab !== "servers" || !view?.servers.some((s) => s.status === "initializing" || s.status === "deferred")) return;
    const id = window.setInterval(() => void reload(), 2500);
    return () => window.clearInterval(id);
  }, [reload, tab, view?.servers]);

  // mutate runs an MCP edit, re-reads the snapshot, and surfaces any failure as an
  // inline banner (a connect error, a missing binary, a bad URL).
  const mutate = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      await reload();
      return true;
    } catch (e) {
      const message = capabilityErrorMessage("capabilities.operation", e, "更新能力设置失败，请稍后重试。");
      await reload();
      setErr(message);
      return false;
    } finally {
      setBusy(false);
    }
  };

  const summary = useMemo(() => {
    if (!view) return "";
    return t("caps.summary", {
      connected: view.servers.filter((s) => s.status === "connected").length,
      failed: view.servers.filter((s) => s.status === "failed").length,
      skills: view.skills.length,
    });
  }, [view, t]);

  const filteredSkills = useMemo(() => {
    if (!view) return [];
    const q = skillQuery.trim().toLowerCase();
    if (!q) return view.skills;
    return view.skills.filter((sk) => {
      const text = [sk.name, `/${sk.name}`, sk.description, sk.scope, sk.runAs].join(" ").toLowerCase();
      return text.includes(q);
    });
  }, [view, skillQuery]);
  const skillSummary = useMemo(() => {
    if (!view) return "";
    return skillListSummary(view.skills, filteredSkills, skillQuery.trim().length > 0, t);
  }, [filteredSkills, skillQuery, t, view]);

  const serverGroups = useMemo(() => {
    const servers = sortServersForDisplay(view?.servers ?? []);
    return {
      failed: servers.filter((s) => s.status === "failed"),
      active: servers.filter((s) => s.status !== "failed"),
    };
  }, [view]);
  const retryableActiveServerNames = useMemo(() => retryableAvailableServerNames(serverGroups.active), [serverGroups.active]);
  const toggleError = useCallback((name: string) => {
    setExpandedErrors((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }, []);

  const toggleServer = useCallback((name: string) => {
    setExpandedServers((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }, []);

  const toggleServerTools = useCallback((name: string) => {
    setExpandedServerTools((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  }, []);

  return (
    <ResizableDrawer onClose={onClose} subtle>
        <header className="drawer__head">
          <div>
            <div className="drawer__title">{t("caps.title")}</div>
            {view && <div className="drawer__summary">{summary}</div>}
          </div>
          <div className="drawer__actions">
            <Tooltip label={t("caps.refresh")}>
              <button className="chip" disabled={busy} onClick={() => void reload()}>
                ↻
              </button>
            </Tooltip>
            <ModalCloseButton label={t("common.close")} onClick={onClose} />
          </div>
        </header>

        {!view ? (
          <div className="empty">{t("caps.loading")}</div>
        ) : (
          <div className="drawer__body">
            {err && <div className="banner banner--error">{err}</div>}

            <div className="cap-tabs" role="tablist" aria-label={t("caps.title")}>
              <button
                className={`cap-tab${tab === "servers" ? " cap-tab--active" : ""}`}
                role="tab"
                aria-selected={tab === "servers"}
                onClick={() => setTab("servers")}
              >
                {t("caps.connectorsTab")}
              </button>
              <button
                className={`cap-tab${tab === "skills" ? " cap-tab--active" : ""}`}
                role="tab"
                aria-selected={tab === "skills"}
                onClick={() => setTab("skills")}
              >
                {t("caps.skillsTab")}
              </button>
            </div>

            {tab === "servers" ? (
              <section className="mem-section">
                {serverGroups.failed.length > 0 && (
                  <FailedServersNotice
                    servers={serverGroups.failed}
                    expanded={expandedErrors}
                    onToggle={toggleError}
                    onRetry={(name) => void mutate(() => app.ReconnectMCPServer(name))}
                    onRetryMany={(names) => void mutate(() => Promise.all(names.map((name) => app.ReconnectMCPServer(name))))}
                    onConfirmClearAuth={(name) => void mutate(() => app.ClearMCPServerAuthentication(name))}
                    onConfirm={(name) => void mutate(() => app.RemoveMCPServer(name))}
                    onConfirmMany={(names) => void mutate(() => Promise.all(names.map((name) => app.RemoveMCPServer(name))))}
                    busy={busy}
                  />
                )}
                {view.servers.length === 0 && (
                  <div className="mem-empty">{t("caps.noServers")}</div>
                )}
                {serverGroups.active.length > 0 && (
                  <div className="cap-server-section">
                    <div className="cap-server-section__head">
                      <div className="cap-server-section__title">{t("caps.availableServers")}</div>
                      <button
                        className="btn btn--small"
                        disabled={busy || retryableActiveServerNames.length === 0}
                        type="button"
                        onClick={() => void mutate(() => Promise.all(retryableActiveServerNames.map((name) => app.ReconnectMCPServer(name))))}
                      >
                        {t("caps.retryAll")}
                      </button>
                    </div>
                    <ServerGroup
                      busy={busy}
                      servers={serverGroups.active}
                      expanded={expandedServers}
                      expandedTools={expandedServerTools}
                      editing={editing}
                      onConfirm={(name) => void mutate(() => app.RemoveMCPServer(name))}
                      onEdit={(name) => {
                        setEditing(name);
                      }}
                      onCancelEdit={() => setEditing(null)}
                      onRetry={(name) => void mutate(() => app.ReconnectMCPServer(name))}
                      onReconnect={(name) => void mutate(() => app.ReconnectMCPServer(name))}
                      onConfirmClearAuth={(name) => void mutate(() => app.ClearMCPServerAuthentication(name))}
                      onTrustTool={(name, toolName) => void mutate(() => app.TrustMCPServerTool(name, toolName))}
                      onTrustTools={(name, toolNames) => void mutate(() => app.TrustMCPServerTools(name, toolNames))}
                      onUntrustTool={(name, toolName) => void mutate(() => app.UntrustMCPServerTool(name, toolName))}
                      onToggle={(name, on) => void mutate(() => app.SetMCPServerEnabled(name, on))}
                      onUpdate={(name, input) =>
                        void mutate(() => app.UpdateMCPServer(name, input)).then((ok) => {
                          if (ok) setEditing(null);
                        })
                      }
                      onToggleDetails={toggleServer}
                      onToggleTools={toggleServerTools}
                    />
                  </div>
                )}
              </section>
            ) : (
              <section className="mem-section">
                <div className="cap-search">
                  <input
                    className="mem-input"
                    type="search"
                    placeholder={t("caps.searchSkills")}
                    value={skillQuery}
                    onChange={(e) => setSkillQuery(e.target.value)}
                  />
                </div>
                <SkillSources
                  roots={view.skillRoots ?? []}
                  busy={busy}
                  onAdd={() => mutate(async () => {
                    const path = await app.PickSkillFolder();
                    if (path) await app.AddSkillPath(path);
                  })}
                  onRefresh={() => mutate(() => app.RefreshSkills())}
                  onRemove={(path) => mutate(() => app.RemoveSkillPath(path))}
                />
                <div className="cap-skills-head">
                  <div className="cap-skills-head__copy">
                    <div className="cap-skills-head__title">{t("caps.skills")}</div>
                    <div className="cap-skills-head__summary">{skillSummary}</div>
                  </div>
                </div>
                {view.skills.length === 0 ? (
                  <div className="mem-empty">{t("caps.noSkills")}</div>
                ) : filteredSkills.length === 0 ? (
                  <div className="mem-empty">{t("caps.noSkillMatches")}</div>
                ) : (
                  <div className="cap-skills">
                    {filteredSkills.map((sk) => (
                      <SkillRow
                        key={sk.name}
                        skill={sk}
                        busy={busy}
                        onToggleEnabled={(enabled) => void mutate(() => app.SetSkillEnabled(sk.name, enabled))}
                      />
                    ))}
                  </div>
                )}
              </section>
            )}
          </div>
        )}
    </ResizableDrawer>
  );
}

function normalizeCapabilitiesView(view: CapabilitiesView | null | undefined): CapabilitiesView {
  return {
    servers: normalizeServerViews(view?.servers),
    ...normalizeSkillsSettingsView(view),
  };
}

function normalizeServerViews(servers: ServerView[] | null | undefined): ServerView[] {
  return sortServersForDisplay(
    asArray(servers).map((server) => ({
      ...server,
      args: asArray(server.args),
      envKeys: asArray(server.envKeys),
      headerKeys: asArray(server.headerKeys),
      toolList: asArray(server.toolList),
      trustedReadOnlyTools: asArray(server.trustedReadOnlyTools),
    })),
  );
}

export async function disconnectManagedConnectorSelectionNames(
  connectorID: string,
  servers: readonly ServerView[],
  disconnect: () => Promise<unknown>,
): Promise<string[]> {
  const prefix = `${connectorID}--`;
  const runtimeNames = servers
    .map((server) => server.name.trim())
    .filter((name) => name.startsWith(prefix));
  await disconnect();
  return [...new Set([connectorID, ...runtimeNames])];
}

function normalizeSkillsSettingsView(view: SkillsSettingsView | CapabilitiesView | null | undefined): SkillsSettingsView {
  return {
    skills: asArray(view?.skills),
    skillRoots: asArray(view?.skillRoots).map((root) => ({
      ...root,
      removable: Boolean(root.removable),
      skillItems: asArray(root.skillItems),
    })),
  };
}

function sortServersForDisplay(servers: ServerView[]): ServerView[] {
  return [...servers].sort((a, b) => {
    const priority = serverDisplayPriority(a) - serverDisplayPriority(b);
    if (priority !== 0) return priority;
    return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  });
}

function serverDisplayPriority(server: ServerView): number {
  if (server.status === "failed" || server.authStatus === "required") return 0;
  if (server.builtIn) return 1;
  if (server.status !== "disabled") return 2;
  return 3;
}

function skillListSummary(skills: SkillView[], filtered: SkillView[], searching: boolean, t: ReturnType<typeof useT>): string {
  if (searching) {
    return t("caps.skillsSummaryMatches", { matched: filtered.length, total: skills.length });
  }
  const parts = [t("caps.skillsSummaryAvailable", { skills: skills.length })];
  const scopes = ["project", "custom", "global", "builtin"];
  for (const scope of scopes) {
    const count = skills.filter((skill) => skill.scope === scope).length;
    if (count > 0) parts.push(skillScopeSummary(scope, count, t));
  }
  return parts.join(" · ");
}

function mcpServerSummary(servers: ServerView[], t: ReturnType<typeof useT>): string {
  return t("caps.mcpSummary", {
    connected: servers.filter((s) => s.status === "connected").length,
    failed: servers.filter((s) => s.status === "failed").length,
    tools: servers.reduce((total, server) => total + (server.tools || 0), 0),
  });
}

function skillScopeSummary(scope: string, count: number, t: ReturnType<typeof useT>): string {
  switch (scope) {
    case "builtin":
      return t("caps.skillsSummaryBuiltin", { count });
    case "project":
      return t("caps.skillsSummaryProject", { count });
    case "custom":
      return t("caps.skillsSummaryCustom", { count });
    case "global":
      return t("caps.skillsSummaryGlobal", { count });
    default:
      return `${count} ${scope}`;
  }
}

function skillSourceSummary(active: number, missing: number, empty: number, t: ReturnType<typeof useT>): string {
  const parts: string[] = [];
  if (active > 0) parts.push(t("caps.sourcesSummaryActive", { active }));
  if (missing > 0) parts.push(t("caps.sourcesSummaryMissing", { missing }));
  if (empty > 0) parts.push(t("caps.sourcesSummaryEmpty", { empty }));
  return parts.length > 0 ? parts.join(" · ") : t("caps.sourcesSummaryNone");
}

function SkillSources({
  roots,
  busy,
  onAdd,
  onRefresh,
  onRemove,
}: {
  roots: SkillRootView[];
  busy: boolean;
  onAdd: () => void;
  onRefresh: () => void;
  onRemove: (path: string) => void;
}) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  const [showDiagnostics, setShowDiagnostics] = useState(false);
  const [expandedRootSkills, setExpandedRootSkills] = useState<Set<string>>(() => new Set());
  const [fullRootSkills, setFullRootSkills] = useState<Set<string>>(() => new Set());
  const primaryRoots = roots.filter(isPrimarySkillRoot);
  const diagnosticRoots = roots.filter((root) => !isPrimarySkillRoot(root));
  const diagnosticsVisible = expanded && showDiagnostics;
  const shownRoots = diagnosticsVisible ? [...primaryRoots, ...diagnosticRoots] : primaryRoots;
  const summaryRoots = diagnosticsVisible ? roots : primaryRoots;
  const active = summaryRoots.filter((root) => root.skills > 0).length;
  const missing = summaryRoots.filter((root) => root.status === "missing").length;
  const empty = summaryRoots.filter((root) => root.status === "ok" && root.skills === 0).length;
  const toggleRootSkills = (key: string) => {
    setExpandedRootSkills((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  const toggleRootSkillFull = (key: string) => {
    setFullRootSkills((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  return (
    <div className={`cap-sources${expanded ? " cap-sources--expanded" : ""}`}>
      <div className="cap-sources__head">
        <div className="cap-sources__copy">
          <div className="cap-sources__title">{t("caps.sources")}</div>
          <div className="cap-sources__summary">{skillSourceSummary(active, missing, empty, t)}</div>
        </div>
        {!expanded && (
          <div className="cap-sources__actions">
            <button className="btn btn--small" type="button" onClick={() => setExpanded(true)} aria-expanded={expanded}>
              {t("caps.manageSkillSources")}
            </button>
          </div>
        )}
      </div>
      {expanded && (
        <>
          <div className="cap-sources__manage">
            <div className="cap-sources__manage-actions">
              <button className="btn btn--small" disabled={busy} onClick={onRefresh}>
                {t("caps.refreshSkills")}
              </button>
              <button className="btn btn--small" disabled={busy} onClick={onAdd}>
                {t("caps.addSkillFolder")}
              </button>
            </div>
            <button
              className="btn btn--small"
              type="button"
              onClick={() => {
                setShowDiagnostics(false);
                setExpanded(false);
              }}
              aria-expanded={expanded}
            >
              {t("common.collapse")}
            </button>
          </div>
          {shownRoots.length === 0 ? (
            <div className="mem-empty">{t("caps.noSkillRoots")}</div>
          ) : (
            <div className="cap-source-list">
              {shownRoots.map((root) => {
                const key = skillRootKey(root);
                const rootSkills = root.skillItems ?? [];
                const rootSkillsExpanded = expandedRootSkills.has(key);
                const rootSkillsFull = fullRootSkills.has(key);
                const canShowRootSkills = rootSkills.length > 0;
                const canRemoveRoot = root.removable;
                return (
                  <div className={`cap-source cap-source--${skillRootTone(root)}`} key={key}>
                    <span className={`cap-dot cap-dot--${skillRootDot(root)}`} />
                    <div className="cap-source__text">
                      <div className="cap-source__head">
                        <div className="cap-source__label" title={root.dir}>
                          {skillRootLabel(root)}
                        </div>
                      </div>
                      <div className="cap-source__meta">
                        <span>{skillRootStatus(root, t)}</span>
                        <span>{t("caps.skillRootCount", { skills: root.skills })}</span>
                        {root.configured && <span>{t("caps.skillRootConfigured")}</span>}
                      </div>
                      {(canShowRootSkills || canRemoveRoot) && (
                        <div className="cap-source-actions">
                          <>
                            {canShowRootSkills && (
                              <button
                                className="btn btn--small"
                                disabled={busy}
                                type="button"
                                aria-expanded={rootSkillsExpanded}
                                onClick={() => toggleRootSkills(key)}
                              >
                                {rootSkillsExpanded ? t("caps.hideSkills") : t("caps.showSkills")}
                              </button>
                              )}
                              {canRemoveRoot && (
                                <InlineConfirmButton
                                  label={t("caps.skillRootRemove")}
                                  confirmLabel={t("caps.skillRootConfirmRemove")}
                                  cancelLabel={t("common.cancel")}
                                  disabled={busy}
                                  danger
                                  onConfirm={() => onRemove(root.dir)}
                                />
                              )}
                            </>
                        </div>
                      )}
                      {rootSkillsExpanded && rootSkills.length > 0 && (
                        <SkillRootSkillsList
                          skills={rootSkills}
                          showAll={rootSkillsFull}
                          onToggleAll={() => toggleRootSkillFull(key)}
                        />
                      )}
                      {root.warning && <div className="cap-source__warning">{root.warning}</div>}
                    </div>
                    <div className="cap-source__badges">
                      {skillRootBadges(root, t).map((badge) => (
                        <span className={`cap-source-badge cap-source-badge--${badge.tone}`} key={badge.label}>
                          {badge.label}
                        </span>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          {diagnosticRoots.length > 0 && (
            <button className="cap-diagnostics" type="button" onClick={() => setShowDiagnostics((v) => !v)}>
              {diagnosticsVisible ? t("caps.hideDiagnostics") : t("caps.showDiagnostics", { count: diagnosticRoots.length })}
            </button>
          )}
        </>
      )}
    </div>
  );
}

const skillRootPreviewLimit = 5;

function SkillRootSkillsList({
  skills,
  showAll,
  onToggleAll,
}: {
  skills: SkillRootSkillView[];
  showAll: boolean;
  onToggleAll: () => void;
}) {
  const t = useT();
  const visible = showAll ? skills : skills.slice(0, skillRootPreviewLimit);
  return (
    <div className="cap-source-skills">
      {visible.map((skill) => (
        <div className="cap-source-skill" key={`${skill.scope}:${skill.name}`}>
          <div className="cap-source-skill__head">
            <span className="cap-source-skill__name">/{skill.name}</span>
            <span className="cap-source-skill__badges">
              <span className={`cap-skill-badge cap-skill-badge--${skill.scope}`}>{skillScopeLabel(skill.scope, t)}</span>
              {skill.runAs === "subagent" && <span className="cap-skill-badge cap-skill-badge--run">{t("caps.subagent")}</span>}
            </span>
          </div>
          {skill.description && <div className="cap-source-skill__desc">{skill.description}</div>}
        </div>
      ))}
      {skills.length > skillRootPreviewLimit && (
        <button className="cap-source-skills__more" type="button" onClick={onToggleAll}>
          {showAll ? t("common.collapse") : t("caps.skillRootShowAllSkills", { count: skills.length })}
        </button>
      )}
    </div>
  );
}

function skillRootKey(root: SkillRootView): string {
  return `${root.scope}:${root.priority}:${root.dir}`;
}

function isPrimarySkillRoot(root: SkillRootView): boolean {
  return root.skills > 0 || root.configured || Boolean(root.warning);
}

function skillRootTone(root: SkillRootView): "active" | "empty" | "problem" {
  if (root.warning || root.status === "inactive" || root.status === "unreadable") return "problem";
  if (root.skills > 0) return "active";
  return "empty";
}

function skillRootDot(root: SkillRootView): "connected" | "disabled" | "failed" {
  const tone = skillRootTone(root);
  if (tone === "active") return "connected";
  if (tone === "empty") return "disabled";
  return "failed";
}

function skillRootStatus(root: SkillRootView, t: ReturnType<typeof useT>): string {
  if (root.status === "ok" && root.skills > 0) return t("caps.skillRootActive");
  if (root.status === "ok") return t("caps.skillRootEmpty");
  return root.status;
}

function skillRootLabel(root: SkillRootView): string {
  return root.dir;
}

function skillRootBadges(root: SkillRootView, t: ReturnType<typeof useT>): Array<{ label: string; tone: "scope" | "builtin" | "configured" | "missing" }> {
  const badges: Array<{ label: string; tone: "scope" | "builtin" | "configured" | "missing" }> = [
    { label: skillScopeLabel(root.scope, t), tone: "scope" },
    root.scope === "custom"
      ? { label: root.configured ? t("caps.skillRootUserConfigured") : t("caps.skillRootConfiguredPath"), tone: "configured" }
      : { label: t("caps.skillRootBuiltinPath"), tone: "builtin" },
  ];
  if (root.status === "missing") {
    badges.push({ label: t("caps.skillRootMissing"), tone: "missing" });
  }
  return badges;
}

function ServerGroup({
  servers,
  expanded,
  expandedTools,
  detailsPresentation = "inline",
  busy,
  editing,
  onConfirm,
  onEdit,
  onCancelEdit,
  onRetry,
  onReconnect,
  onConfirmClearAuth,
  onTrustTool,
  onTrustTools,
  onUntrustTool,
  onToggle,
  onUpdate,
  onToggleDetails,
  onToggleTools,
  selectedServers,
  onSelect,
}: {
  servers: ServerView[];
  expanded: Set<string>;
  expandedTools: Set<string>;
  detailsPresentation?: "inline" | "dialog";
  busy: boolean;
  editing: string | null;
  onConfirm: (name: string) => void;
  onEdit: (name: string) => void;
  onCancelEdit: () => void;
  onRetry: (name: string) => void;
  onReconnect: (name: string) => void;
  onConfirmClearAuth: (name: string) => void;
  onTrustTool: (name: string, toolName: string) => void;
  onTrustTools: (name: string, toolNames: string[]) => void;
  onUntrustTool: (name: string, toolName: string) => void;
  onToggle: (name: string, on: boolean) => void;
  onUpdate: (name: string, input: MCPServerInput) => void;
  onToggleDetails: (name: string) => void;
  onToggleTools: (name: string) => void;
  selectedServers?: readonly string[];
  onSelect?: (name: string) => void;
}) {
  const selectedServerNames = useMemo(() => new Set(selectedServers ?? []), [selectedServers]);
  if (servers.length === 0) return null;
  return (
    <div className="cap-server-group">
      {servers.map((s) => (
        <ServerRow
          key={s.name}
          s={s}
          expanded={expanded.has(s.name)}
          toolsExpanded={expandedTools.has(s.name)}
          detailsPresentation={detailsPresentation}
          busy={busy}
          editing={editing === s.name}
          onConfirm={() => onConfirm(s.name)}
          onEdit={() => onEdit(s.name)}
          onCancelEdit={onCancelEdit}
          onRetry={() => onRetry(s.name)}
          onReconnect={() => onReconnect(s.name)}
          onConfirmClearAuth={() => onConfirmClearAuth(s.name)}
          onTrustTool={(toolName) => onTrustTool(s.name, toolName)}
          onTrustTools={(toolNames) => onTrustTools(s.name, toolNames)}
          onUntrustTool={(toolName) => onUntrustTool(s.name, toolName)}
          onToggle={(on) => onToggle(s.name, on)}
          onUpdate={(input) => onUpdate(s.name, input)}
          onToggleDetails={() => onToggleDetails(s.name)}
          onToggleTools={() => onToggleTools(s.name)}
          selected={selectedServerNames.has(s.name)}
          onSelect={onSelect ? () => onSelect(s.name) : undefined}
        />
      ))}
    </div>
  );
}

function FailedServersNotice({
  servers,
  expanded,
  busy,
  onToggle,
  onRetry,
  onRetryMany,
  onConfirmClearAuth,
  onConfirm,
  onConfirmMany,
}: {
  servers: ServerView[];
  expanded: Set<string>;
  busy: boolean;
  onToggle: (name: string) => void;
  onRetry: (name: string) => void;
  onRetryMany: (names: string[]) => void;
  onConfirmClearAuth: (name: string) => void;
  onConfirm: (name: string) => void;
  onConfirmMany: (names: string[]) => void;
}) {
  const t = useT();
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const groups = useMemo(() => failureGroups(servers, t), [servers, t]);
  const removableFailures = useMemo(() => servers.filter(canBulkRemoveFailure), [servers]);
  const retryNames = useMemo(() => servers.map((s) => s.name), [servers]);
  return (
    <div className="cap-failures" role="region" aria-label={t("caps.failureTitle", { failed: servers.length })}>
      <div className="cap-failures__head">
        <div>
          <div className="cap-failures__title">{t("caps.failureTitle", { failed: servers.length })}</div>
          <div className="cap-failures__hint">{t("caps.failureHint")}</div>
        </div>
        <div className="cap-failures__actions">
          <button className="btn btn--small" disabled={busy} type="button" onClick={() => setDetailsOpen((v) => !v)} aria-expanded={detailsOpen}>
            {detailsOpen ? t("caps.hideFailureDetails") : t("caps.showFailureDetails")}
          </button>
          <button className="btn btn--small" disabled={busy || retryNames.length === 0} type="button" onClick={() => onRetryMany(retryNames)}>
            {t("caps.retryAll")}
          </button>
          {removableFailures.length > 0 && (
            <button className="btn btn--small" disabled={busy} type="button" onClick={() => setBulkOpen((v) => !v)} aria-expanded={bulkOpen}>
              {t("caps.bulkActions")}
            </button>
          )}
        </div>
      </div>
      <div className="cap-failures__meta">
        <div className="cap-failures__chips" aria-label={t("caps.failureGroups")}>
          {groups.map((group) => (
            <span className="cap-failure-chip" key={group.kind}>{group.label}</span>
          ))}
        </div>
      </div>
      {bulkOpen && removableFailures.length > 0 && (
        <div className="cap-failures__bulk">
          <InlineConfirmButton
            label={t("caps.removeInvalid", { count: removableFailures.length })}
            confirmLabel={t("caps.confirmRemoveInvalid", { count: removableFailures.length })}
            cancelLabel={t("common.cancel")}
            disabled={busy}
            danger
            onConfirm={() => onConfirmMany(removableFailures.map((s) => s.name))}
          />
        </div>
      )}
      {detailsOpen && <div className="cap-failures__list">
        {servers.map((s) => {
          const open = expanded.has(s.name);
          const error = s.error || t("caps.failed");
          const actionLabel = serverActionLabel(s, t);
          const handlePrimaryAction = () => {
            if (shouldOpenAuth(s)) {
              openExternal((s.authUrl || "").trim());
              return;
            }
            onRetry(s.name);
          };
          return (
            <div className="cap-failure" key={s.name}>
              <div className="cap-failure__main">
                <span className="cap-dot cap-dot--failed" />
                <div className="cap-failure__text">
                  <div className="cap-failure__name">{s.name}</div>
                  <div className="cap-failure__summary">{s.authStatus === "required" ? t("caps.authRequiredSummary") : summarizeServerError(error)}</div>
                </div>
              </div>
              <div className="cap-failure__actions">
                <TestConnectionControl name={s.name} disabled={busy} />
                <button className="btn btn--small" disabled={busy} onClick={handlePrimaryAction}>
                  {actionLabel}
                </button>
                {canClearAuth(s) && (
                  <InlineConfirmButton
                    label={t("caps.clearAuth")}
                    confirmLabel={t("caps.confirmClearAuth")}
                    cancelLabel={t("common.cancel")}
                    disabled={busy}
                    onConfirm={() => onConfirmClearAuth(s.name)}
                  />
                )}
                <button className="btn btn--small" onClick={() => onToggle(s.name)} aria-expanded={open}>
                  {open ? t("common.collapse") : t("caps.showLog")}
                </button>
                {!s.builtIn && (
                  <InlineConfirmButton
                    label={t("caps.remove")}
                    confirmLabel={t("caps.confirmRemove")}
                    cancelLabel={t("common.cancel")}
                    disabled={busy}
                    danger
                    onConfirm={() => onConfirm(s.name)}
                  />
                )}
              </div>
              {open && (
                <div className="cap-failure__logbox">
                  <div className="cap-failure__logbar">
                    <span>{t("caps.rawLog")}</span>
                    <button className="btn btn--small" onClick={() => void navigator.clipboard?.writeText(error)}>
                      {t("caps.copyLog")}
                    </button>
                  </div>
                  <pre className="cap-failure__log">{error}</pre>
                </div>
              )}
            </div>
          );
        })}
      </div>}
    </div>
  );
}

function ServerRow({
  s,
  expanded,
  toolsExpanded,
  detailsPresentation,
  busy,
  editing,
  onConfirm,
  onEdit,
  onCancelEdit,
  onRetry,
  onReconnect,
  onConfirmClearAuth,
  onTrustTool,
  onTrustTools,
  onUntrustTool,
  onToggle,
  onUpdate,
  onToggleDetails,
  onToggleTools,
  selected,
  onSelect,
}: {
  s: ServerView;
  expanded: boolean;
  toolsExpanded: boolean;
  detailsPresentation: "inline" | "dialog";
  busy: boolean;
  editing: boolean;
  onConfirm: () => void;
  onEdit: () => void;
  onCancelEdit: () => void;
  onRetry: () => void;
  onReconnect: () => void;
  onConfirmClearAuth: () => void;
  onTrustTool: (toolName: string) => void;
  onTrustTools: (toolNames: string[]) => void;
  onUntrustTool: (toolName: string) => void;
  onToggle: (on: boolean) => void;
  onUpdate: (input: MCPServerInput) => void;
  onToggleDetails: () => void;
  onToggleTools: () => void;
  selected?: boolean;
  onSelect?: () => void;
}) {
  const t = useT();
  const actionLabel = serverActionLabel(s, t);
  const lifecycle = mcpServerLifecycleActions(s);
  const tools = s.toolList ?? [];
  const detailsInDialog = detailsPresentation === "dialog";
  let sub =
    s.status === "failed"
      ? s.error || t("caps.failed")
      : s.status === "initializing"
        ? t("caps.initializing")
      : s.status === "deferred"
        ? t("caps.deferred")
      : s.status === "disabled"
        ? s.configured && !s.autoStart
          ? t("caps.disabledAutoStart")
          : t("caps.disabled")
        : t("caps.counts", { tools: s.tools, prompts: s.prompts, resources: s.resources });
  if (s.authStatus === "possible" && s.status !== "failed") {
    sub = `${sub} · ${t("caps.authPossibleShort")}`;
  }
  const handlePrimaryAction = () => {
    if (shouldOpenAuth(s)) {
      openExternal((s.authUrl || "").trim());
      return;
    }
    onRetry();
  };
  const summary = (
    <div className="cap-row__text">
      <div className="cap-row__head">
        <span className="cap-row__name">{s.name}</span>
        <span className="cap-row__transport">{s.transport}</span>
        {s.builtIn && <span className="cap-row__builtin">{t("caps.builtIn")}</span>}
      </div>
      <div className="cap-row__sub">{sub}</div>
    </div>
  );
  return (
    <div className={`cap-server-entry${s.status === "disabled" ? " cap-server-entry--disabled" : ""}`}>
      <Tooltip label={s.error} disabled={!s.error} fill block>
        <div className={`cap-row${s.status === "disabled" ? " cap-row--disabled" : ""}`}>
          {detailsInDialog ? (
            <button
              className="cap-row__summary-trigger"
              type="button"
              aria-haspopup="dialog"
              aria-label={`${t("caps.showFailureDetails")} ${s.name}`}
              onClick={onToggleDetails}
            >
              <span className={`cap-dot cap-dot--${s.status}`} />
              {summary}
            </button>
          ) : (
            <>
              <Tooltip label={expanded ? t("caps.collapseDetails") : t("caps.expandDetails")}>
                <button
                  className="cap-disclosure"
                  aria-expanded={expanded}
                  aria-label={expanded ? t("caps.collapseDetails") : t("caps.expandDetails")}
                  onClick={onToggleDetails}
                >
                  {expanded ? <ChevronDown aria-hidden size={15} strokeWidth={2.2} /> : <ChevronRight aria-hidden size={15} strokeWidth={2.2} />}
                </button>
              </Tooltip>
              <span className={`cap-dot cap-dot--${s.status}`} />
              {summary}
            </>
          )}
          <div className="cap-row__actions">
            {detailsInDialog && (
              <button className="btn btn--small cap-detail-action" type="button" aria-haspopup="dialog" onClick={onToggleDetails}>
                {t("caps.details")}
              </button>
            )}
            {onSelect ? (
              <button
                className={`btn btn--small cap-select-action${selected ? " cap-select-action--selected" : ""}`}
                type="button"
                disabled={busy || s.status !== "connected" || s.tools <= 0}
                aria-pressed={selected}
                onClick={onSelect}
              >
                {selected ? t("caps.enabledForCurrentTask") : t("caps.useForCurrentTask")}
              </button>
            ) : null}
            {lifecycle.showRetryInRow ? (
              <button className="btn btn--small" disabled={busy} onClick={handlePrimaryAction}>
                {actionLabel}
              </button>
            ) : (
              <Tooltip label={lifecycle.enabled ? t("caps.disable") : t("caps.enable")}>
                <label className="cap-switch">
                  <input
                    type="checkbox"
                    checked={lifecycle.enabled}
                    disabled={busy}
                    aria-label={`${lifecycle.enabled ? t("caps.disable") : t("caps.enable")} ${s.name}`}
                    onChange={(e) => onToggle(e.target.checked)}
                  />
                  <span className="cap-switch__track" />
                </label>
              </Tooltip>
            )}
          </div>
        </div>
      </Tooltip>
      {expanded && !detailsInDialog && (
        <ServerDetails
          s={s}
          tools={tools}
          busy={busy}
          onConfirm={onConfirm}
          onConnectNow={onRetry}
          onReconnect={onReconnect}
          onConfirmClearAuth={onConfirmClearAuth}
          onTrustTool={onTrustTool}
          onTrustTools={onTrustTools}
          onUntrustTool={onUntrustTool}
          toolsExpanded={toolsExpanded}
          editing={editing}
          onEdit={onEdit}
          onCancelEdit={onCancelEdit}
          onUpdate={onUpdate}
          onToggleTools={onToggleTools}
        />
      )}
    </div>
  );
}

function ServerDetails({
  s,
  tools,
  busy,
  onConfirm,
  onConnectNow,
  onReconnect,
  onConfirmClearAuth,
  onTrustTool,
  onTrustTools,
  onUntrustTool,
  toolsExpanded,
  editing,
  onEdit,
  onCancelEdit,
  onUpdate,
  onToggleTools,
}: {
  s: ServerView;
  tools: ServerView["toolList"];
  busy: boolean;
  onConfirm: () => void;
  onConnectNow: () => void;
  onReconnect: () => void;
  onConfirmClearAuth: () => void;
  onTrustTool: (toolName: string) => void;
  onTrustTools: (toolNames: string[]) => void;
  onUntrustTool: (toolName: string) => void;
  toolsExpanded: boolean;
  editing: boolean;
  onEdit: () => void;
  onCancelEdit: () => void;
  onUpdate: (input: MCPServerInput) => void;
  onToggleTools: () => void;
}) {
  const t = useT();
  const command = serverCommand(s);
  const canEditConfig = s.configured && !s.builtIn;
  const lifecycle = mcpServerLifecycleActions(s);
  const canConnectNow = lifecycle.canConnectNow;
  const canReconnect = lifecycle.canReconnect;
  const canShowTools = s.status === "connected" && ((s.tools ?? 0) > 0 || (tools?.length ?? 0) > 0);
  const showClearAuth = canClearAuth(s);
  const authLabel = serverAuthLabel(s, t);
  const trustedReadOnlyTools = s.trustedReadOnlyTools ?? [];
  const trustedReadOnlyToolNames = new Set(trustedReadOnlyTools);
  const canTrustTool = s.configured && !s.builtIn;
  const reportedReadOnlyToolNames = (tools ?? []).filter((tool) => tool.readOnlyHint).map((tool) => tool.name);
  const bulkTrustToolNames = reportedReadOnlyToolNames.filter((name) => !trustedReadOnlyToolNames.has(name));
  if (editing && canEditConfig) {
    return (
      <div className="cap-server-details">
        <EditServerForm s={s} busy={busy} onCancel={onCancelEdit} onSave={onUpdate} />
      </div>
    );
  }
  return (
    <div className="cap-server-details">
      <div className="cap-detail-grid">
        <div className="cap-detail">
          <span className="cap-detail__label">{t("caps.status")}</span>
          <span className="cap-detail__value">{serverStatusLabel(s, t)}</span>
        </div>
        <div className="cap-detail">
          <span className="cap-detail__label">{t("caps.transport")}</span>
          <span className="cap-detail__value">{s.transport}</span>
        </div>
        {authLabel && (
          <div className="cap-detail">
            <span className="cap-detail__label">{t("caps.auth")}</span>
            <span className="cap-detail__value">{authLabel}</span>
          </div>
        )}
        {command && (
          <div className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{s.transport === "stdio" ? t("caps.command") : t("caps.url")}</span>
            <span className="cap-detail__code">{command}</span>
          </div>
        )}
        {s.envKeys && s.envKeys.length > 0 && (
          <div className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.envKeys")}</span>
            <span className="cap-detail__value">{s.envKeys.join(", ")}</span>
          </div>
        )}
        {s.headerKeys && s.headerKeys.length > 0 && (
          <div className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.headerKeys")}</span>
            <span className="cap-detail__value">{s.headerKeys.join(", ")}</span>
          </div>
        )}
        {trustedReadOnlyTools.length > 0 && (
          <div className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.trustedReadOnlyTools")}</span>
            <span className="cap-detail__code">{trustedReadOnlyTools.join(", ")}</span>
          </div>
        )}
      </div>
      <div className="cap-detail-actions">
        <TestConnectionControl name={s.name} disabled={busy} />
        {canConnectNow && (
          <button className="btn btn--small" disabled={busy} onClick={onConnectNow}>
            {t("caps.connectNow")}
          </button>
        )}
        {canReconnect && (
          <button className="btn btn--small" disabled={busy} onClick={onReconnect}>
            {t("caps.reconnect")}
          </button>
        )}
        {canShowTools && (
          <button className="btn btn--small" disabled={busy} onClick={onToggleTools} aria-expanded={toolsExpanded}>
            {toolsExpanded ? t("caps.hideTools") : t("caps.showTools")}
          </button>
        )}
        {canTrustTool && bulkTrustToolNames.length > 0 && (
          <button
            className="btn btn--small cap-trust-bulk"
            disabled={busy}
            onClick={() => onTrustTools(bulkTrustToolNames)}
            title={t("caps.trustReportedReadOnlyTitle")}
            type="button"
          >
            <ShieldCheck aria-hidden size={13} strokeWidth={2.2} />
            {t("caps.trustReportedReadOnly", { count: bulkTrustToolNames.length })}
          </button>
        )}
        {showClearAuth && (
          <InlineConfirmButton
            label={t("caps.clearAuth")}
            confirmLabel={t("caps.confirmClearAuth")}
            cancelLabel={t("common.cancel")}
            disabled={busy}
            onConfirm={onConfirmClearAuth}
          />
        )}
        {canEditConfig && (
          <>
            <button className="btn btn--small" disabled={busy} onClick={onEdit}>
              {t("caps.editConfig")}
            </button>
            <InlineConfirmButton
              label={t("caps.remove")}
              confirmLabel={t("caps.confirmRemove")}
              cancelLabel={t("common.cancel")}
              disabled={busy}
              danger
              onConfirm={onConfirm}
            />
          </>
        )}
      </div>
      {toolsExpanded && (
        tools && tools.length > 0 ? (
          <div className="cap-tool-list">
            <div className="cap-tool-list__title">{t("caps.tools")}</div>
            {tools.map((tool) => {
              const trusted = trustedReadOnlyToolNames.has(tool.name);
              return (
                <div className="cap-tool" key={tool.name}>
                  <div className="cap-tool__name">{tool.name}</div>
                  <div className="cap-tool__desc">
                    <span>{tool.description}</span>
                    {tool.readOnlyHint && (
                      <span className="cap-tool-hint" title={t("caps.reportedReadOnlyTitle")}>
                        {t("caps.reportedReadOnly")}
                      </span>
                    )}
                  </div>
                  <div className="cap-tool__action">
                    {canTrustTool ? (
                      trusted ? (
                        <div className="cap-tool-trust-stack">
                          <span className="cap-tool-trust cap-tool-trust--trusted" title={t("caps.trustedReadOnlyTitle")}>
                            <ShieldCheck aria-hidden size={12} strokeWidth={2.2} />
                            {t("caps.trustedReadOnly")}
                          </span>
                          <button
                            className="btn btn--small cap-tool-untrust-btn"
                            disabled={busy}
                            onClick={() => onUntrustTool(tool.name)}
                            title={t("caps.untrustReadOnlyTitle")}
                            type="button"
                          >
                            <ShieldOff aria-hidden size={12} strokeWidth={2.2} />
                            {t("caps.untrustReadOnly")}
                          </button>
                        </div>
                      ) : (
                        <button
                          className="btn btn--small cap-tool-trust-btn"
                          disabled={busy}
                          onClick={() => onTrustTool(tool.name)}
                          title={t("caps.trustReadOnlyTitle")}
                          type="button"
                        >
                          <ShieldCheck aria-hidden size={12} strokeWidth={2.2} />
                          {t("caps.trustReadOnly")}
                        </button>
                      )
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="cap-tool-empty">{t("caps.noToolDetails")}</div>
        )
      )}
    </div>
  );
}

function EditServerForm({
  s,
  busy,
  onCancel,
  onSave,
}: {
  s: ServerView;
  busy: boolean;
  onCancel: () => void;
  onSave: (input: MCPServerInput) => void;
}) {
  const t = useT();
  const initialTransport = normalizeTransportValue(s.transport);
  const [transport, setTransport] = useState(initialTransport);
  const [command, setCommand] = useState(initialTransport === "stdio" ? serverCommand(s) : "");
  const [url, setUrl] = useState(initialTransport === "stdio" ? "" : s.url || serverCommand(s));
  const [headers, setHeaders] = useState("");
  const [env, setEnv] = useState("");
  const isStdio = transport === "stdio";
  const ready = isStdio ? command.trim() !== "" : url.trim() !== "";

  const submit = () => {
    const envText = env.trim();
    const headerText = headers.trim();
    onSave({
      name: s.name,
      transport,
      command: isStdio ? command.trim() : "",
      args: [],
      url: isStdio ? "" : url.trim(),
      env: envText === "" ? null : parseKeyValueText(envText),
      headers: isStdio || headerText === "" ? null : parseKeyValueText(headerText),
      trustedReadOnlyTools: s.trustedReadOnlyTools ?? [],
    });
  };

  return (
    <div className="cap-config-edit">
      <div className="cap-detail-grid">
        <div className="cap-detail">
          <span className="cap-detail__label">{t("caps.name")}</span>
          <span className="cap-detail__value">{s.name}</span>
        </div>
        <label className="cap-detail cap-detail--select">
          <span className="cap-detail__label">{t("caps.transport")}</span>
          <select className="mem-select" value={transport} disabled={busy} onChange={(e) => setTransport(e.target.value)}>
            <option value="stdio">stdio</option>
            <option value="http">http</option>
            <option value="sse">sse</option>
          </select>
        </label>
        {isStdio ? (
          <label className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.command")}</span>
            <input className="mem-input" value={command} disabled={busy} onChange={(e) => setCommand(e.target.value)} placeholder={t("caps.commandPlaceholder")} />
          </label>
        ) : (
          <label className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.url")}</span>
            <input className="mem-input" value={url} disabled={busy} onChange={(e) => setUrl(e.target.value)} placeholder={t("caps.urlPlaceholder")} />
          </label>
        )}
        {!isStdio && (
          <label className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.headersLabel")}</span>
            <textarea className="mem-textarea cap-config-edit__env" value={headers} disabled={busy} onChange={(e) => setHeaders(e.target.value)} placeholder={t("caps.headersPlaceholder")} spellCheck={false} />
          </label>
        )}
        {!isStdio && s.headerKeys && s.headerKeys.length > 0 && (
          <div className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.headerKeys")}</span>
            <span className="cap-detail__value">{s.headerKeys.join(", ")}</span>
            <span className="cap-edit-hint">{t("caps.headersPreserveHint")}</span>
          </div>
        )}
        <label className="cap-detail cap-detail--wide">
          <span className="cap-detail__label">{t("caps.envLabel")}</span>
          <textarea className="mem-textarea cap-config-edit__env" value={env} disabled={busy} onChange={(e) => setEnv(e.target.value)} placeholder={t("caps.envPlaceholder")} spellCheck={false} />
        </label>
        {s.envKeys && s.envKeys.length > 0 && (
          <div className="cap-detail cap-detail--wide">
            <span className="cap-detail__label">{t("caps.envKeys")}</span>
            <span className="cap-detail__value">{s.envKeys.join(", ")}</span>
            <span className="cap-edit-hint">{t("caps.envPreserveHint")}</span>
          </div>
        )}
      </div>
      <div className="cap-detail-actions">
        <button className="btn btn--small" disabled={busy} onClick={onCancel}>
          {t("common.cancel")}
        </button>
        <button className="btn btn--primary btn--small" disabled={busy || !ready} onClick={submit}>
          {t("caps.saveConfig")}
        </button>
      </div>
    </div>
  );
}

function serverCommand(s: ServerView): string {
  if (s.transport === "stdio") return [s.command, ...(s.args ?? [])].filter(Boolean).join(" ").trim();
  return (s.url || "").trim();
}

function normalizeTransportValue(transport: string): string {
  return transport === "http" || transport === "sse" ? transport : "stdio";
}

function parseKeyValueText(text: string): Record<string, string> {
  const values: Record<string, string> = {};
  for (const rawLine of text.split("\n")) {
    const line = rawLine.trim();
    if (!line) continue;
    const eq = line.indexOf("=");
    if (eq > 0) values[line.slice(0, eq).trim()] = line.slice(eq + 1).trim();
  }
  return values;
}

function serverStatusLabel(s: ServerView, t: ReturnType<typeof useT>): string {
  switch (s.status) {
    case "connected":
      return t("caps.connected");
    case "deferred":
      return t("caps.deferred");
    case "initializing":
      return t("caps.initializing");
    case "disabled":
      return s.configured && !s.autoStart ? t("caps.disabledAutoStart") : t("caps.disabled");
    case "failed":
      if (s.authStatus === "required") return t("caps.authRequired");
      return t("caps.failed");
    default:
      return s.status;
  }
}

function summarizeServerError(error: string): string {
  const normalized = error.replace(/\s+/g, " ").trim();
  const plugin = normalized.match(/plugin "([^"]+)"/i)?.[1];
  const npmCode = normalized.match(/\bnpm error code ([A-Z0-9_]+)/i)?.[1];
  const errno = normalized.match(/\berrno (-?\d+)/i)?.[1];
  const reason = npmCode
    ? `npm ${npmCode}${errno ? ` (${errno})` : ""}`
    : normalized.split(/(?:\.\s+|\n)/)[0];
  const summary = plugin ? `${plugin}: ${reason}` : reason;
  return summary.length > 180 ? `${summary.slice(0, 176).trim()}…` : summary;
}

type FailureKind = "auth" | "missing-command" | "command-unavailable" | "network" | "other";

function failureKind(server: ServerView): FailureKind {
  if (server.authStatus === "required") return "auth";
  const err = (server.error || "").toLowerCase();
  if (err.includes("command is required")) return "missing-command";
  if (
    err.includes("command not found") ||
    err.includes("executable file not found") ||
    err.includes("no such file") ||
    err.includes("enoent")
  ) {
    return "command-unavailable";
  }
  if (
    err.includes("401") ||
    err.includes("403") ||
    err.includes("unauthorized") ||
    err.includes("forbidden") ||
    err.includes("timeout") ||
    err.includes("network")
  ) {
    return "network";
  }
  return "other";
}

function failureGroups(servers: ServerView[], t: ReturnType<typeof useT>): Array<{ kind: FailureKind; label: string }> {
  const counts = new Map<FailureKind, number>();
  for (const server of servers) {
    const kind = failureKind(server);
    counts.set(kind, (counts.get(kind) ?? 0) + 1);
  }
  const order: FailureKind[] = ["missing-command", "command-unavailable", "auth", "network", "other"];
  return order.flatMap((kind) => {
    const count = counts.get(kind) ?? 0;
    if (count === 0) return [];
    return [{ kind, label: failureGroupLabel(kind, count, t) }];
  });
}

function failureGroupLabel(kind: FailureKind, count: number, t: ReturnType<typeof useT>): string {
  switch (kind) {
    case "auth":
      return t("caps.failureGroupAuth", { count });
    case "missing-command":
      return t("caps.failureGroupMissingCommand", { count });
    case "command-unavailable":
      return t("caps.failureGroupCommandUnavailable", { count });
    case "network":
      return t("caps.failureGroupNetwork", { count });
    default:
      return t("caps.failureGroupOther", { count });
  }
}

function canBulkRemoveFailure(server: ServerView): boolean {
  if (server.builtIn || !server.configured) return false;
  const kind = failureKind(server);
  return kind === "missing-command" || kind === "command-unavailable";
}

function retryableAvailableServerNames(servers: ServerView[]): string[] {
  return servers.filter(mcpServerRetryableFromAvailableList).map((s) => s.name);
}

function serverActionLabel(s: ServerView, t: ReturnType<typeof useT>): string {
  const err = (s.error || "").toLowerCase();
  if (shouldOpenAuth(s)) return t("caps.reauthorize");
  if (
    err.includes("command not found") ||
    err.includes("executable file not found") ||
    err.includes("no such file") ||
    err.includes("enoent")
  ) {
    return t("caps.checkCommand");
  }
  return t("caps.retry");
}

function serverAuthLabel(s: ServerView, t: ReturnType<typeof useT>): string {
  if (s.authStatus === "required") return t("caps.authRequired");
  if (s.authStatus === "possible") return t("caps.authPossible");
  return "";
}

function shouldOpenAuth(s: ServerView): boolean {
  const url = (s.authUrl || "").trim();
  return s.authStatus === "required" && /^https?:\/\//i.test(url);
}

function canClearAuth(s: ServerView): boolean {
  if (!s.configured || s.builtIn) return false;
  return Boolean(s.authConfigured || s.authStatus === "required" || s.authStatus === "possible" || isRemoteTransport(s.transport));
}

function isRemoteTransport(transport?: string): boolean {
  const value = (transport || "").trim().toLowerCase();
  return value === "http" || value === "streamable-http" || value === "sse";
}

function SkillRow({
  skill,
  displayName: displayNameOverride,
  iconUrl,
  busy,
  onToggleEnabled,
  onUninstall,
  selected,
  onSelect,
  batchMode = false,
  batchSelected = false,
  onBatchSelect,
}: {
  skill: SkillView;
  displayName?: string | null;
  iconUrl?: string | null;
  busy: boolean;
  onToggleEnabled: (enabled: boolean) => void;
  onUninstall?: () => Promise<unknown>;
  selected?: boolean;
  onSelect?: () => void;
  batchMode?: boolean;
  batchSelected?: boolean;
  onBatchSelect?: () => void;
}) {
  const t = useT();
  const displayName = displayNameOverride?.trim() || skill.displayName?.trim() || skill.name;
  const summary = chineseSkillDescription(displayName, skill.description);
  const [menuPoint, setMenuPoint] = useState<ContextMenuPoint | null>(null);
  const [uninstallConfirm, setUninstallConfirm] = useState(false);
  const pinned = skill.scope === "custom" || skill.scope === "project";
  const closeMenu = useCallback(() => {
    setMenuPoint(null);
    setUninstallConfirm(false);
  }, []);
  useEffect(() => {
    if (batchMode) closeMenu();
  }, [batchMode, closeMenu]);
  return (
    <div
      className={`cap-skill-card${selected ? " cap-skill-card--selected" : ""}${!skill.enabled ? " cap-skill-card--disabled" : ""}${menuPoint ? " cap-skill-card--menu-open" : ""}${batchMode ? " cap-skill-card--batch-mode" : ""}${batchSelected ? " cap-skill-card--batch-selected" : ""}`}
      role={batchMode ? "checkbox" : undefined}
      aria-checked={batchMode ? batchSelected : undefined}
      tabIndex={batchMode ? 0 : undefined}
      onClick={batchMode ? onBatchSelect : undefined}
      onKeyDown={batchMode ? (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onBatchSelect?.();
        }
      } : undefined}
    >
      {batchMode && (
        <label className="cap-skill-card__batch-checkbox" onClick={(event) => event.stopPropagation()}>
          <input
            type="checkbox"
            checked={batchSelected}
            aria-hidden="true"
            tabIndex={-1}
            onChange={onBatchSelect}
          />
          <span aria-hidden="true" />
        </label>
      )}
      <div className="cap-skill-card__top">
        <div className="cap-skill-card__toggle">
          <span className="cap-skill-card__head">
            <span className="cap-skill-card__icon" aria-hidden="true">
              <span>{displayName.charAt(0).toUpperCase() || "/"}</span>
              {iconUrl && <img src={iconUrl} alt="" onError={(event) => { event.currentTarget.remove(); }} />}
            </span>
            <span className="cap-skill-card__command" title={displayName}>{displayName}</span>
          </span>
        </div>
        {!batchMode && <span className="cap-skill-card__header-actions">
          <button
            className="cap-skill-card__menu-trigger"
            type="button"
            aria-label={`打开技能 ${displayName} 菜单`}
            aria-haspopup="menu"
            aria-expanded={Boolean(menuPoint)}
            onClick={(event) => {
              const rect = event.currentTarget.getBoundingClientRect();
              setUninstallConfirm(false);
              setMenuPoint(menuPoint ? null : { left: rect.left, top: rect.bottom + 5 });
            }}
          >
            <MoreHorizontal size={17} aria-hidden="true" />
          </button>
          {pinned && <Pin className="cap-skill-card__pin" size={16} strokeWidth={1.8} aria-label="已固定" />}
          <Tooltip label={skill.enabled ? t("caps.disableSkill") : t("caps.enableSkill")}>
            <label className="cap-switch">
              <input
                type="checkbox"
                checked={skill.enabled}
                disabled={busy}
                aria-label={`${skill.enabled ? t("caps.disableSkill") : t("caps.enableSkill")} ${displayName}`}
                onChange={(e) => onToggleEnabled(e.target.checked)}
              />
              <span className="cap-switch__track" />
            </label>
          </Tooltip>
        </span>}
      </div>
      <div className="cap-skill-card__desc">{summary}</div>
      <ContextMenu
        open={Boolean(menuPoint)}
        point={menuPoint}
        minWidth={134}
        ariaLabel={`技能 ${displayName} 操作`}
        onClose={closeMenu}
        items={[
          {
            key: "chat",
            icon: <MessageCircle size={15} aria-hidden="true" />,
            label: "去对话",
            disabled: busy || !skill.enabled || !onSelect,
            onSelect: () => { closeMenu(); onSelect?.(); },
          },
          {
            key: "uninstall",
            icon: <Trash2 size={15} aria-hidden="true" />,
            label: uninstallConfirm ? "确认卸载" : "卸载",
            danger: true,
            disabled: busy || !onUninstall,
            onSelect: () => {
              if (!uninstallConfirm) {
                setUninstallConfirm(true);
                return;
              }
              closeMenu();
              void onUninstall?.();
            },
          },
        ]}
      />
    </div>
  );
}

function skillScopeLabel(scope: string, t: ReturnType<typeof useT>): string {
  switch (scope) {
    case "builtin":
      return t("caps.skillScopeBuiltin");
    case "project":
      return t("caps.skillScopeProject");
    case "custom":
      return t("caps.skillScopeCustom");
    case "global":
      return t("caps.skillScopeGlobal");
    default:
      return scope;
  }
}

function chineseSkillDescription(name: string, description: string): string {
  const searchable = `${name} ${description}`.toLowerCase();
  if (/browser|playwright|chrome|网页|浏览器/.test(searchable)) return "用于浏览器操作、网页检查和交互验证。";
  if (/document|docx|word|文档/.test(searchable)) return "用于创建、编辑和检查文档内容。";
  if (/spreadsheet|excel|sheet|表格/.test(searchable)) return "用于创建、分析和校验电子表格。";
  if (/presentation|slides|ppt|演示/.test(searchable)) return "用于创建、编辑和检查演示文稿。";
  if (/pdf/.test(searchable)) return "用于读取、生成和检查 PDF 文件。";
  if (/image|图片|图像/.test(searchable)) return "用于生成、编辑或检查图像内容。";
  if (/figma|design|设计/.test(searchable)) return "用于设计稿分析、设计协作和界面实现。";
  if (/git|worktree|分支/.test(searchable)) return "用于 Git 分支、工作树和版本集成流程。";
  if (/test|debug|review|quality|测试|调试|审查/.test(searchable)) return "用于测试、调试或代码质量检查。";
  if (/security|安全/.test(searchable)) return "用于安全检查和代码加固。";
  if (/data|analytics|数据|分析/.test(searchable)) return "用于数据分析、验证和结果呈现。";
  return "用于扩展智能体能力，可在任务中按需启用。";
}

const CONNECTOR_MATURITY_LABEL_KEYS = {
  official: "workspace.connectors.maturity.official",
  preview: "workspace.connectors.maturity.preview",
  community: "workspace.connectors.maturity.community",
} as const;

function managedConnectorErrorMessage(state: ConnectorStateView | undefined, t: Translator): string {
	if (!state?.error) return "";
	if (state.error.code === "CONNECTOR_OAUTH_TEMPORARILY_UNAVAILABLE") return t("workspace.connectors.temporarilyUnavailable");
	if (state.error.code === "CONNECTOR_OAUTH_REAUTH_REQUIRED") return t("workspace.connectors.reauthorizationRequired");
	if (state.error.code === "CONNECTOR_OAUTH_FAILED") return t("workspace.connectors.oauthNotCompleted");
	return t("workspace.connectors.connectionFailed");
}

function managedConnectorFailureToast(state: ConnectorStateView | undefined, t: Translator): string {
	if (state?.error?.code === "CONNECTOR_OAUTH_TEMPORARILY_UNAVAILABLE") return t("workspace.connectors.temporarilyUnavailableToast");
	if (state?.error?.code === "CONNECTOR_OAUTH_REAUTH_REQUIRED") return t("workspace.connectors.reauthorizationRequiredToast");
	if (state?.error?.code === "CONNECTOR_OAUTH_FAILED") return t("workspace.connectors.oauthNotCompletedToast");
	return t("workspace.connectors.connectionFailedToast");
}

type ManagedConnectorConnectOutcome = {
	status: "connected" | "canceled" | "failed";
	message?: string;
};

const CATALOG_SERVER_NAMES = new Set(
  CONNECTOR_CATALOG.flatMap((entry) => entry.preset ? [entry.preset.name] : []),
);

function ConnectionTestResult({ result }: { result: MCPConnectionTestResult }) {
  const connected = result.status === "connected";
  return (
    <div
      className={`connector-test-result connector-test-result--${connected ? "connected" : "failed"}`}
      role="status"
      aria-live="polite"
    >
      <strong>状态：{connected ? "已连接" : "连接失败"}</strong>
      <span>延迟：{result.latencyMs} ms</span>
      <span>工具：{result.tools}</span>
      <span>资源：{result.resources}</span>
      <span>提示词：{result.prompts}</span>
      {result.error && <span className="connector-test-result__error">原因：{result.error}</span>}
      {result.repairAction && <span>修复：{result.repairAction}</span>}
    </div>
  );
}

function TestConnectionControl({ name, disabled }: { name: string; disabled: boolean }) {
  const t = useT();
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<MCPConnectionTestResult | null>(null);
  const testConnection = async () => {
    setTesting(true);
    try {
      setResult(await app.TestMCPServer(name));
    } catch (error) {
      setResult({
        name,
        status: "failed",
        phase: "request",
        latencyMs: 0,
        tools: 0,
        resources: 0,
        prompts: 0,
        error: capabilityErrorMessage("connectors.test", error, "连接测试失败，请检查配置后重试。"),
        repairAction: "检查连接器配置后重试",
      });
    } finally {
      setTesting(false);
    }
  };
  return (
    <div className="connector-test-control">
      <button
        className="btn btn--small"
        type="button"
        disabled={disabled || testing}
        title={t("workspace.connectors.testConnectionHint")}
        onClick={() => void testConnection()}
      >
        {testing ? t("workspace.connectors.testingConnection") : t("workspace.connectors.testConnection")}
      </button>
      {result && <ConnectionTestResult result={result} />}
    </div>
  );
}

function ConnectorCatalogPanel({
	query,
  onTry,
	managedCatalog,
	managedStates,
	tabID,
	loginRequired,
	busyConnectorIDs,
	onConnectConnector,
	onDisconnectConnector,
	onCancelConnector,
	onBotSettingsChanged,
}: {
	query: string;
  onTry?: (serverName: string, prompt: string) => void;
	managedCatalog: readonly ConnectorCatalogItemView[];
	managedStates: readonly ConnectorStateView[];
	tabID: string;
	loginRequired: boolean;
	busyConnectorIDs: ReadonlySet<string>;
	onConnectConnector: (connectorID: string, tokenValues: Record<string, string>) => Promise<ManagedConnectorConnectOutcome>;
	onDisconnectConnector: (connectorID: string) => Promise<boolean>;
	onCancelConnector: (connectorID: string) => Promise<boolean>;
	onBotSettingsChanged?: () => void;
}) {
  const t = useT();
	const { showToast } = useToast();
	const [preparingIDs, setPreparingIDs] = useState<Set<string>>(() => new Set());
	const [operationNotice, setOperationNotice] = useState("");
	const [cancelingConnectorID, setCancelingConnectorID] = useState("");
	const [authRequest, setAuthRequest] = useState<{ connectorID: string; fields: ConnectorAuthFieldView[]; values: Record<string, string> } | null>(null);
	const [detailConnectorID, setDetailConnectorID] = useState("");
	const managedCatalogByID = useMemo(() => new Map(managedCatalog.map((item) => [item.id, item])), [managedCatalog]);
	const managedStateByID = useMemo(() => new Map(managedStates.map((item) => [item.connector_id, item])), [managedStates]);
  const entries = useMemo(
		() => {
			const normalizedQuery = query.trim().toLowerCase();
			return managedCatalog
				.filter((item) => !normalizedQuery || [item.name, item.provider, item.description, item.category, ...item.capabilities].join(" ").toLowerCase().includes(normalizedQuery))
				.map((item): ConnectorCatalogEntry => ({
					id: item.id, category: item.category, name: item.name, provider: item.provider, description: item.description,
					authMode: "none", setup: "direct", maturity: "official", docsUrl: "", note: "",
					tags: [...new Set([item.kind, ...item.capabilities, item.version])], examplePrompt: item.examples[0],
				}));
		},
		[managedCatalog, query],
	);
	const allEntriesByID = useMemo(() => new Map(managedCatalog.map((item): [string, ConnectorCatalogEntry] => [item.id, {
		id: item.id, category: item.category, name: item.name, provider: item.provider, description: item.description,
		authMode: "none", setup: "direct", maturity: "official", docsUrl: "", note: "",
		tags: [...new Set([item.kind, ...item.capabilities, item.version])], examplePrompt: item.examples[0],
	}])), [managedCatalog]);
	const updatePreparing = (connectorID: string, active: boolean) => setPreparingIDs((previous) => {
		const next = new Set(previous);
		if (active) next.add(connectorID);
		else next.delete(connectorID);
		return next;
	});
	const closeDetails = useCallback(() => {
		setDetailConnectorID("");
		setAuthRequest(null);
		setOperationNotice("");
	}, []);
	useEffect(() => {
		if (!detailConnectorID) return;
		const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") closeDetails(); };
		document.addEventListener("keydown", closeOnEscape);
		return () => document.removeEventListener("keydown", closeOnEscape);
	}, [closeDetails, detailConnectorID]);
	const startConnect = async (connectorID: string) => {
		if (loginRequired) {
			showToast(t("workspace.connectors.loginRequired"), "info");
			return;
		}
		if (!tabID || preparingIDs.has(connectorID) || busyConnectorIDs.has(connectorID)) return;
		const switchingConnectorID = [
			...busyConnectorIDs,
			...managedStates.filter((state) => state.status === "authorizing").map((state) => state.connector_id),
		].find((id) => id !== connectorID);
		const switchingConnector = switchingConnectorID ? allEntriesByID.get(switchingConnectorID) : undefined;
		const nextConnector = allEntriesByID.get(connectorID);
		updatePreparing(connectorID, true);
		setOperationNotice(switchingConnector && nextConnector
			? t("workspace.connectors.authorizationSwitching", { previous: switchingConnector.name, next: nextConnector.name })
			: "");
		try {
			await app.InstallConnector(tabID, connectorID);
			const fields = await app.ConnectorAuthFields(tabID, connectorID);
			if (fields.length > 0) {
				setAuthRequest({ connectorID, fields, values: {} });
				return;
			}
			const outcome = await onConnectConnector(connectorID, {});
			if (outcome.status === "connected") {
				setOperationNotice("");
				showToast(t("workspace.connectors.connectionReady", { name: nextConnector?.name ?? connectorID }), "success");
			}
			else if (outcome.status === "failed") {
				setOperationNotice("");
				showToast(outcome.message || t("workspace.connectors.connectionFailedToast"), "error");
			}
		} catch (error) {
			setOperationNotice("");
			showToast(capabilityErrorMessage("connectors.managed.prepare", error, "连接器准备失败，请重试。"), "error");
		} finally {
			updatePreparing(connectorID, false);
		}
	};
	const submitTokenAuth = async () => {
		if (!authRequest) return;
		const connectorID = authRequest.connectorID;
		updatePreparing(connectorID, true);
		try {
			const outcome = await onConnectConnector(connectorID, authRequest.values);
			if (outcome.status === "connected") {
				setAuthRequest(null);
				setOperationNotice("");
				showToast(t("workspace.connectors.connectionReady", { name: allEntriesByID.get(connectorID)?.name ?? connectorID }), "success");
			}
			else if (outcome.status === "failed") showToast(outcome.message || t("workspace.connectors.connectionFailedToast"), "error");
		} catch (error) {
			showToast(capabilityErrorMessage("connectors.managed.token", error, "连接器凭据保存失败，请重试。"), "error");
		} finally {
			updatePreparing(connectorID, false);
		}
	};
	const cancelConnect = async (connectorID: string) => {
		if (cancelingConnectorID) return;
		setCancelingConnectorID(connectorID);
		setOperationNotice("");
		try {
			if (await onCancelConnector(connectorID)) {
				showToast(t("workspace.connectors.authorizationCanceled", { name: allEntriesByID.get(connectorID)?.name ?? connectorID }), "info");
				closeDetails();
			}
		} finally {
			setCancelingConnectorID("");
		}
	};
	const detailEntry = detailConnectorID ? allEntriesByID.get(detailConnectorID) : undefined;
	const detailCatalogItem = detailConnectorID ? managedCatalogByID.get(detailConnectorID) : undefined;
	const detailState = detailConnectorID ? managedStateByID.get(detailConnectorID) : undefined;
	const detailStatus = detailState?.status ?? "not_installed";
	const detailConnected = detailStatus === "connected";
	const detailConnecting = Boolean(detailConnectorID) && (preparingIDs.has(detailConnectorID) || busyConnectorIDs.has(detailConnectorID) || detailStatus === "installing" || detailStatus === "authorizing");
	const detailFailed = detailStatus === "failed" || detailStatus === "repair_required" || Boolean(detailState?.error);
	const detailOAuthFailed = detailState?.error?.code === "CONNECTOR_OAUTH_FAILED" || detailState?.error?.code === "CONNECTOR_OAUTH_REAUTH_REQUIRED";
	const activeAuthorizationIDs = new Set([
		...preparingIDs,
		...busyConnectorIDs,
		...managedStates.filter((state) => state.status === "authorizing").map((state) => state.connector_id),
	]);
  return (
    <div className="connector-catalog">
			{loginRequired && <div className="connector-catalog__notice" role="status">{t("workspace.connectors.anonymousNotice")}</div>}
			{operationNotice && !detailEntry && <div className="connector-catalog__notice" role="status" aria-live="polite">{operationNotice}</div>}
			{entries.length === 0 && !matchesFeishuConnectorQuery(query) && <div className="connector-catalog__empty">{t("workspace.connectors.searchEmpty")}</div>}
      <div className="connector-catalog__grid" id="connector-catalog-panel">
			<FeishuBotConnector query={query} onChanged={onBotSettingsChanged} />
        {entries.map((entry) => (
				<ManagedConnectorCatalogCard
					key={entry.id}
					entry={entry}
					catalogItem={managedCatalogByID.get(entry.id)!}
					state={managedStateByID.get(entry.id)}
					selectedForTask={false}
					busy={preparingIDs.has(entry.id) || busyConnectorIDs.has(entry.id)}
					loginRequired={loginRequired}
					anotherAuthorizationActive={[...activeAuthorizationIDs].some((id) => id !== entry.id)}
					onOpen={() => setDetailConnectorID(entry.id)}
				/>
        ))}
      </div>
			{detailEntry && detailCatalogItem && (
				<div className="management-modal-backdrop connector-catalog-dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeDetails(); }}>
					<section className="management-modal connector-catalog-dialog" role="dialog" aria-modal="true" aria-labelledby="connector-catalog-dialog-title">
						<ModalCloseButton className="connector-catalog-dialog__close" label={t("common.close")} showTooltipOnFocus={false} autoFocus onClick={closeDetails} />
						<div className="connector-catalog-dialog__content">
							<ConnectorBrandIcon item={detailCatalogItem} name={detailEntry.name} size={56} />
							<div className="connector-catalog-dialog__title-line">
								<h2 id="connector-catalog-dialog-title">{detailEntry.name}</h2>
							</div>
							<p className="connector-catalog-dialog__description">{detailEntry.description}</p>
							{operationNotice && !detailConnected && <div className="connector-catalog-dialog__notice" role="status" aria-live="polite">{operationNotice}</div>}
							{detailConnecting && (
								<div className="connector-catalog-dialog__waiting" role="status">
									<LoaderCircle className="connector-catalog__spinner" aria-hidden size={18} />
									<span>{t("workspace.connectors.authorizationWaiting")}</span>
								</div>
							)}
							{authRequest?.connectorID === detailEntry.id && (
								<div className="connector-catalog-dialog__credentials" role="group" aria-label="Connector token credentials">
									{authRequest.fields[0]?.documentation_url && (
										<button type="button" className="connector-catalog__link" onClick={() => void openExternal(authRequest.fields[0].documentation_url)}>
											<ExternalLink aria-hidden size={13} />{authRequest.fields[0].documentation_label}
										</button>
									)}
									{authRequest.fields.map((field) => (
										<label className="set-label" key={field.key}>
											<span>{field.label}{field.required ? " *" : ""}</span>
											<input className="mem-input" type={field.type === "secret" ? "password" : "text"} autoComplete="off" value={authRequest.values[field.key] ?? ""}
												onChange={(event) => setAuthRequest((current) => current ? { ...current, values: { ...current.values, [field.key]: event.target.value } } : current)} />
										</label>
									))}
								</div>
							)}
						</div>
						<footer className="connector-catalog-dialog__actions">
							{authRequest?.connectorID === detailEntry.id ? (
								<>
									<button className="btn" type="button" onClick={() => setAuthRequest(null)}>{t("common.cancel")}</button>
									<button className="btn btn--primary" type="button" disabled={preparingIDs.has(detailEntry.id) || authRequest.fields.some((field) => field.required && !(authRequest.values[field.key] ?? "").trim())} onClick={() => void submitTokenAuth()}>{t("composer.connectorQuick.connect")}</button>
								</>
							) : detailConnecting ? (
								<button className="btn btn--danger" type="button" disabled={cancelingConnectorID === detailEntry.id} onClick={() => void cancelConnect(detailEntry.id)}>
									{cancelingConnectorID === detailEntry.id ? t("workspace.connectors.authorizationCanceling") : t("workspace.connectors.cancelConnection")}
								</button>
							) : detailConnected ? (
								<>
									<InlineConfirmButton label={t("workspace.connectors.disconnect")} confirmLabel={t("workspace.connectors.confirmDisconnect")} cancelLabel={t("common.cancel")} danger disabled={busyConnectorIDs.has(detailEntry.id)} onConfirm={async () => {
										if (await onDisconnectConnector(detailEntry.id)) closeDetails();
									}} />
									{detailEntry.examplePrompt && onTry && <button className="btn btn--primary" type="button" onClick={() => { onTry(detailEntry.id, detailEntry.examplePrompt!); closeDetails(); }}>{t("workspace.connectors.tryInConversation")}</button>}
								</>
							) : (
								<button className="btn btn--primary" type="button" disabled={!tabID || preparingIDs.has(detailEntry.id) || !detailCatalogItem.compatible} onClick={() => void startConnect(detailEntry.id)}>
									{loginRequired ? t("workspace.connectors.loginToConnect") : detailOAuthFailed ? t("workspace.connectors.reauthorize") : detailFailed ? t("common.retry") : t("composer.connectorQuick.connect")}
								</button>
							)}
						</footer>
					</section>
				</div>
			)}
		</div>
	);
}

function ManagedConnectorCatalogCard({
	entry,
	catalogItem,
	state,
	selectedForTask,
	busy,
	loginRequired,
	anotherAuthorizationActive,
	onOpen,
}: {
	entry: ConnectorCatalogEntry;
	catalogItem: ConnectorCatalogItemView;
	state?: ConnectorStateView;
	selectedForTask: boolean;
	busy: boolean;
	loginRequired: boolean;
	anotherAuthorizationActive: boolean;
	onOpen: () => void;
}) {
	const t = useT();
	const status = state?.status ?? "not_installed";
	const connected = status === "connected";
	const connecting = status === "installing" || status === "authorizing" || busy;
	const failed = status === "failed" || status === "repair_required" || Boolean(state?.error);
	const statusClass = connected ? "connected" : failed ? "configuration_error" : connecting ? "connecting" : "uninstalled";
	const statusLabel = connected
		? t("workspace.connectors.cardConnectedStatus")
		: failed
			? managedConnectorErrorMessage(state, t)
			: status === "authorizing"
				? t("workspace.connectors.cardAuthorizationWaiting")
				: connecting
					? t("workspace.connectors.cardPreparing")
					: anotherAuthorizationActive
						? t("workspace.connectors.cardAuthorizationSwitch")
						: loginRequired
							? t("workspace.connectors.loginToConnect")
							: t("workspace.connectors.cardNotConnected");
	return (
		<article className={`connector-catalog__card connector-catalog__card--${statusClass}${selectedForTask ? " connector-catalog__card--task-enabled" : ""}`} data-connector-id={entry.id}>
			<button className="connector-catalog__card-main" type="button" onClick={onOpen} aria-label={`${entry.name} · ${statusLabel}`}>
				<ConnectorBrandIcon item={catalogItem} name={entry.name} size={40} />
				<span className="connector-catalog__card-copy">
					<span className="connector-catalog__card-title"><strong>{entry.name}</strong>{connected && <span className="connector-catalog__connected-dot" aria-hidden="true" />}</span>
					<span className="connector-catalog__description">{entry.description}</span>
					<span className={`connector-catalog__card-status connector-catalog__card-status--${statusClass}`}>{statusLabel}</span>
				</span>
				<span className={`connector-catalog__card-action connector-catalog__card-action--${statusClass}`} aria-hidden="true">
					{connecting ? <LoaderCircle className="connector-catalog__spinner" size={18} /> : failed || !catalogItem.compatible ? <AlertCircle size={18} /> : connected ? <ChevronRight size={18} /> : <Plus size={20} />}
				</span>
			</button>
		</article>
	);
}

export function ConnectorCatalogCard({
  entry,
  availability,
  selectedForTask,
  busy,
  connectionResult,
  onInstall,
  onRetry,
  onSelect,
  onTry,
  onOpenDetails,
}: {
  entry: ConnectorCatalogEntry;
  availability: ConnectorAvailability;
  selectedForTask: boolean;
  busy: boolean;
  connectionResult?: MCPConnectionTestResult;
  onInstall: () => void;
  onRetry: () => void;
  onSelect?: () => void;
  onTry?: () => void;
  onOpenDetails?: () => void;
}) {
  const t = useT();
  const server = availability.server;
  const auth = server && shouldOpenAuth(server);
  const standaloneResult = server ? undefined : connectionResult;
  const resultFailed = standaloneResult && standaloneResult.status !== "connected";
  const statusClass = standaloneResult?.status === "connected"
    ? "connected"
    : resultFailed
      ? "configuration_error"
      : availability.state;
  const statusLabel = standaloneResult?.status === "connected"
    ? t("workspace.connectors.cardConnected", { count: standaloneResult.tools })
    : resultFailed
      ? standaloneResult.error || t("workspace.connectors.cardFailed")
      : availability.label;
  const primaryAction = () => {
    if (availability.state === "connected") {
      onSelect?.();
      return;
    }
    if (auth) {
      void openExternal((server.authUrl || "").trim());
      return;
    }
    if (availability.state === "configuration_error") {
      onRetry();
      return;
    }
    if (availability.state === "uninstalled") onInstall();
  };
  const primaryLabel = availability.state === "connected"
    ? selectedForTask
      ? t("caps.enabledForCurrentTask")
      : t("caps.useForCurrentTask")
    : resultFailed
      ? t("composer.connectorQuick.retry")
      : auth
      ? t("composer.connectorQuick.authorize")
      : availability.state === "configuration_error"
        ? t("composer.connectorQuick.retry")
        : availability.state === "connecting" || busy
          ? t("composer.connectorQuick.connecting")
          : t("composer.connectorQuick.connect");

  return (
    <article
      className={`connector-catalog__card connector-catalog__card--${statusClass}${selectedForTask ? " connector-catalog__card--task-enabled" : ""}`}
      data-connector-id={entry.id}
    >
      <div className="connector-catalog__card-head">
        <span>
          <strong>{entry.name}</strong>
          <small>{entry.provider}</small>
        </span>
        <span className={`connector-catalog__state connector-catalog__state--${statusClass}`}>
          {busy || availability.state === "connecting"
            ? <LoaderCircle className="connector-catalog__spinner" aria-hidden size={15} />
            : statusClass === "connected"
              ? <CheckCircle2 aria-hidden size={15} />
              : statusClass === "configuration_error"
                ? <AlertCircle aria-hidden size={15} />
                : null}
          {statusLabel}
        </span>
      </div>
      <p className="connector-catalog__description">{entry.description}</p>
      <div className="connector-catalog__tags">
        <span className={`connector-catalog__badge connector-catalog__badge--${entry.maturity}`}>
          {t(CONNECTOR_MATURITY_LABEL_KEYS[entry.maturity])}
        </span>
        {entry.tags.map((tag) => <span className="connector-catalog__tag" key={tag}>{tag}</span>)}
      </div>
      {resultFailed && standaloneResult?.repairAction ? (
        <p className="connector-catalog__repair">{standaloneResult.repairAction}</p>
      ) : null}
      <div className="connector-catalog__card-footer">
        <div className="connector-catalog__secondary-actions">
          <button type="button" className="connector-catalog__link" onClick={() => void openExternal(entry.docsUrl)}>
            <ExternalLink aria-hidden size={13} />
            {t("workspace.connectors.officialDocs")}
          </button>
          {onOpenDetails && (
            <button type="button" className="connector-catalog__link" onClick={onOpenDetails}>
              {t("caps.details")}
            </button>
          )}
          {onTry && availability.state === "connected" && (
            <button type="button" className="connector-catalog__link connector-catalog__try" onClick={onTry}>
              <Play aria-hidden size={13} />
              {t("workspace.connectors.tryInConversation")}
            </button>
          )}
        </div>
        <button
          type="button"
          className={`btn btn--small${selectedForTask ? " btn--primary" : ""}`}
          disabled={busy || availability.state === "connecting" || (!entry.preset && entry.setup !== "guide-only")}
          aria-pressed={availability.state === "connected" ? selectedForTask : undefined}
          onClick={primaryAction}
        >
          {busy && <LoaderCircle className="connector-catalog__spinner" aria-hidden size={14} />}
          {primaryLabel}
        </button>
      </div>
    </article>
  );
}

// MCPServersSettingsPage is a self-contained MCP servers management page
// embedded inside the settings centre.
export function MCPServersSettingsPage({
	workspace = false,
	targetTabID,
	selectedServers,
	onSelectServer,
  onTryConnector,
	onBotSettingsChanged,
}: {
	workspace?: boolean;
	targetTabID?: string;
	selectedServers?: readonly string[];
	onSelectServer?: (name: string) => void;
  onTryConnector?: (name: string, prompt: string) => void;
	onBotSettingsChanged?: () => void;
} = {}) {
	const t = useT();
	const { showToast } = useToast();
	const accountInitialized = useAccountStore((state) => state.initialized);
	const account = useAccountStore((state) => state.account);
	const connectorIdentityReady = hasRecoverableCloudIdentity(account);
	const connectorLoginRequired = accountInitialized && !connectorIdentityReady;
	const [snapshotKey, setSnapshotKey] = useState("");
	const [servers, setServers] = useState<ServerView[] | null>(null);
	const [discoveredActiveTabID, setDiscoveredActiveTabID] = useState("");
	const targetTabProvided = targetTabID !== undefined;
	// Only use a target confirmed by ListTabs. App.tsx can briefly retain the
	// previous tab ID while single-surface navigation replaces its controller.
	const activeTabID = discoveredActiveTabID;
	const managedTargetRef = useRef(activeTabID);
	managedTargetRef.current = activeTabID;
	const [managedCatalog, setManagedCatalog] = useState<ConnectorCatalogItemView[]>([]);
	const [managedStates, setManagedStates] = useState<ConnectorStateView[]>([]);
	const [busyConnectorIDs, setBusyConnectorIDs] = useState<Set<string>>(() => new Set());
	const busyConnectorIDsRef = useRef<Set<string>>(new Set());
	const cancelledConnectorIDsRef = useRef<Set<string>>(new Set());
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState<string | null>(null);
	const [connectorQuery, setConnectorQuery] = useState("");
	const [headerActionsHost, setHeaderActionsHost] = useState<HTMLElement | null>(null);
	const [detailServerName, setDetailServerName] = useState<string | null>(null);
	const [editing, setEditing] = useState<string | null>(null);
	const [expandedErrors, setExpandedErrors] = useState<Set<string>>(() => new Set());
	const [expandedServers, setExpandedServers] = useState<Set<string>>(() => new Set());
	const [expandedServerTools, setExpandedServerTools] = useState<Set<string>>(() => new Set());

	useLayoutEffect(() => {
		setHeaderActionsHost(workspace ? document.getElementById("capability-workspace-header-actions") : null);
	}, [workspace]);
	const setConnectorBusy = useCallback((connectorID: string, active: boolean) => {
		const next = new Set(busyConnectorIDsRef.current);
		if (active) next.add(connectorID);
		else next.delete(connectorID);
		busyConnectorIDsRef.current = next;
		setBusyConnectorIDs(next);
	}, []);
	const reloadManagedConnectors = useCallback(async (tabID: string) => {
		const catalog = await app.ConnectorCatalog(tabID);
		if (tabID !== managedTargetRef.current) return;
		const items = asArray(catalog.items);
		registerConnectorCatalog(items);
		setManagedCatalog(items);
		if (!tabID || !connectorIdentityReady) {
			setManagedStates([]);
			return;
		}
		try {
			const states = asArray(await app.ConnectorStates(tabID));
			if (tabID === managedTargetRef.current) setManagedStates(states);
		} catch (error) {
			if (isConnectorTargetUnavailable(error)) return;
			throw error;
		}
	}, [connectorIdentityReady]);

	const reload = useCallback(async () => {
		const [meta, tabs] = await Promise.all([
			app.Meta().catch(() => null),
			app.ListTabs().catch(() => []),
		]);
		const key = settingsSnapshotKey(meta, tabs);
		const backendActiveTabID = tabs.find((tab) => tab.active)?.id ?? "";
		const nextActiveTabID = targetTabProvided
			? (targetTabID ? (tabs.some((tab) => tab.id === targetTabID) ? targetTabID : backendActiveTabID) : "")
			: backendActiveTabID;
		managedTargetRef.current = nextActiveTabID;
		setDiscoveredActiveTabID(nextActiveTabID);
		setSnapshotKey(key);
		const cached = key ? mcpSettingsSnapshot : null;
		if (cached?.key === key) {
			setServers(cached.value);
		} else {
			setServers(null);
		}
		try {
			const [next] = await Promise.all([
				app.MCPServers().then(normalizeServerViews),
				reloadManagedConnectors(nextActiveTabID),
			]);
			mcpSettingsSnapshot = { key, value: next };
			setServers(next);
		} catch (e) {
			setErr(capabilityErrorMessage("connectors.load", e, "加载连接器失败，请稍后重试。"));
			if (cached?.key !== key) setServers([]);
		}
	}, [reloadManagedConnectors, targetTabID, targetTabProvided]);
	useEffect(() => { void reload(); }, [reload]);
	useEffect(() => {
		if (!servers?.some((s) => s.status === "initializing" || s.status === "deferred")) return;
		const id = window.setInterval(() => void reload(), 2500);
		return () => window.clearInterval(id);
	}, [reload, servers]);
	const closeDetailDialog = useCallback(() => {
		setDetailServerName(null);
		setEditing(null);
	}, []);
	useEffect(() => {
		if (!workspace || !detailServerName) return;
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") closeDetailDialog();
		};
		document.addEventListener("keydown", onKeyDown);
		return () => document.removeEventListener("keydown", onKeyDown);
	}, [closeDetailDialog, detailServerName, workspace]);

	const mutate = async (fn: () => Promise<unknown>) => {
		setBusy(true);
		setErr(null);
		try {
			await fn();
			await reload();
			return true;
		} catch (e) {
			const message = capabilityErrorMessage("connectors.operation", e, "更新连接器失败，请稍后重试。");
			await reload();
			setErr(message);
			return false;
		} finally {
			setBusy(false);
		}
	};
	const connectManagedConnector = useCallback(async (connectorID: string, tokenValues: Record<string, string>): Promise<ManagedConnectorConnectOutcome> => {
		if (!activeTabID || busyConnectorIDsRef.current.has(connectorID)) return { status: "failed", message: t("workspace.connectors.connectionFailedToast") };
		const tabID = activeTabID;
		const previousAuthorizationIDs = new Set([
			...busyConnectorIDsRef.current,
			...managedStates.filter((state) => state.status === "authorizing").map((state) => state.connector_id),
		]);
		previousAuthorizationIDs.delete(connectorID);
		for (const previousConnectorID of previousAuthorizationIDs) {
			cancelledConnectorIDsRef.current.add(previousConnectorID);
			try {
				await app.CancelConnectorAuthorization(tabID, previousConnectorID);
			} finally {
				setConnectorBusy(previousConnectorID, false);
			}
		}
		if (previousAuthorizationIDs.size > 0) await reloadManagedConnectors(tabID);
		setConnectorBusy(connectorID, true);
		setErr(null);
		cancelledConnectorIDsRef.current.delete(connectorID);
		const poll = window.setInterval(() => {
			void reloadManagedConnectors(tabID).catch((error) => {
				if (isConnectorTargetUnavailable(error)) {
					window.clearInterval(poll);
					return;
				}
				setErr(capabilityErrorMessage("connectors.managed.poll", error, "加载连接器状态失败，请稍后重试。"));
			});
		}, 1000);
		try {
			const state = await app.ConnectConnector(tabID, connectorID, tokenValues);
			await reloadManagedConnectors(tabID);
			setServers(normalizeServerViews(await app.MCPServers()));
			return state.status === "connected"
				? { status: "connected" }
				: { status: "failed", message: managedConnectorFailureToast(state, t) };
		} catch (error) {
			const canceled = cancelledConnectorIDsRef.current.has(connectorID);
			await reloadManagedConnectors(tabID).catch((reloadError) => {
				if (!isConnectorTargetUnavailable(reloadError)) {
					setErr(capabilityErrorMessage("connectors.managed.load", reloadError, "加载连接器状态失败，请稍后重试。"));
				}
			});
			return canceled
				? { status: "canceled" }
				: {
					status: "failed",
					message: capabilityErrorMessage(
						"connectors.managed.connect",
						error,
						t("workspace.connectors.connectionFailedToast"),
						{ CONNECTOR_OAUTH_FAILED: t("workspace.connectors.oauthNotCompletedToast") },
					),
				};
		} finally {
			window.clearInterval(poll);
			cancelledConnectorIDsRef.current.delete(connectorID);
			setConnectorBusy(connectorID, false);
		}
	}, [activeTabID, managedStates, reloadManagedConnectors, setConnectorBusy, t]);
	const disconnectManagedConnector = useCallback(async (connectorID: string) => {
		if (!activeTabID || busyConnectorIDsRef.current.has(connectorID)) return false;
		const tabID = activeTabID;
		setConnectorBusy(connectorID, true);
		setErr(null);
		try {
			const removedNames = await disconnectManagedConnectorSelectionNames(
				connectorID,
				servers ?? [],
				() => app.DisconnectConnector(tabID, connectorID),
			);
			removedNames.forEach(announceMCPServerRemoved);
			await reloadManagedConnectors(tabID);
			setServers(normalizeServerViews(await app.MCPServers()));
			return true;
		} catch (error) {
			showToast(capabilityErrorMessage("connectors.managed.disconnect", error, "连接器解绑失败，请重试。"), "error");
			return false;
		} finally {
			setConnectorBusy(connectorID, false);
		}
	}, [activeTabID, reloadManagedConnectors, servers, setConnectorBusy, showToast]);
	const cancelManagedConnector = useCallback(async (connectorID: string) => {
		if (!activeTabID) return false;
		cancelledConnectorIDsRef.current.add(connectorID);
		setConnectorBusy(connectorID, true);
		setErr(null);
		try {
			await app.CancelConnectorAuthorization(activeTabID, connectorID);
			await reloadManagedConnectors(activeTabID);
			return true;
		} catch (error) {
			cancelledConnectorIDsRef.current.delete(connectorID);
			showToast(capabilityErrorMessage("connectors.managed.cancel", error, "取消连接失败，请重试。"), "error");
			return false;
		} finally {
			setConnectorBusy(connectorID, false);
		}
	}, [activeTabID, reloadManagedConnectors, setConnectorBusy, showToast]);
	const removeServers = async (names: readonly string[]): Promise<boolean> => {
		const unique = [...new Set(names.map((name) => name.trim()).filter(Boolean))];
		if (unique.length === 0) return true;
		const removed = await mutate(() => Promise.all(unique.map((name) => app.RemoveMCPServer(name))));
		if (removed) unique.forEach(announceMCPServerRemoved);
		return removed;
	};
	const managedRuntimePrefixes = useMemo(
		() => managedCatalog.map((item) => `${item.id}--`),
		[managedCatalog],
	);
	const visibleServers = useMemo(
		() => workspace
			? (servers ?? []).filter((server) => !managedRuntimePrefixes.some((prefix) => server.name.startsWith(prefix)))
			: (servers ?? []),
		[managedRuntimePrefixes, servers, workspace],
	);
	const serverGroups = useMemo(() => {
		const sorted = sortServersForDisplay(visibleServers)
			.filter((server) => !CATALOG_SERVER_NAMES.has(server.name));
		return {
			configurationError: sorted.filter((s) =>
				s.status === "failed" || (s.status === "connected" && (s.tools ?? 0) <= 0)),
			connecting: sorted.filter((s) => s.status === "initializing" || s.status === "deferred"),
			connected: sorted.filter((s) => s.status === "connected" && (s.tools ?? 0) > 0),
		};
	}, [visibleServers]);
	const detailServer = useMemo(
		() => (detailServerName ? (servers ?? []).find((server) => server.name === detailServerName) ?? null : null),
		[detailServerName, servers],
	);
	useEffect(() => {
		if (workspace && detailServerName && servers && !detailServer) {
			closeDetailDialog();
		}
	}, [closeDetailDialog, detailServer, detailServerName, servers, workspace]);
	const retryableActiveServerNames = useMemo(
		() => retryableAvailableServerNames([...serverGroups.connected, ...serverGroups.connecting]),
		[serverGroups.connected, serverGroups.connecting],
	);
	const toggleError = useCallback((name: string) => {
		setExpandedErrors((prev) => { const next = new Set(prev); if (next.has(name)) next.delete(name); else next.add(name); return next; });
	}, []);
	const toggleServer = useCallback((name: string) => {
		setExpandedServers((prev) => { const next = new Set(prev); if (next.has(name)) next.delete(name); else next.add(name); return next; });
	}, []);
	const toggleServerTools = useCallback((name: string) => {
		setExpandedServerTools((prev) => { const next = new Set(prev); if (next.has(name)) next.delete(name); else next.add(name); return next; });
	}, []);

	const summary = useMemo(() => {
		if (!servers) return "";
		return mcpServerSummary(visibleServers, t);
	}, [servers, t, visibleServers]);
	const loading = !servers;
	const actionBusy = busy || !snapshotKey || loading;
	const connectorTopbar = (
		<div className="connector-workspace-topbar">
			<label className="connector-workspace-topbar__search">
				<Search size={16} aria-hidden="true" />
				<input
					type="search"
					aria-label={t("workspace.connectors.search")}
					placeholder={t("workspace.connectors.search")}
					value={connectorQuery}
					onChange={(event) => setConnectorQuery(Array.from(event.target.value).slice(0, 128).join(""))}
				/>
			</label>
		</div>
	);

	return (
		<section className="mem-section">
			{workspace ? (headerActionsHost ? createPortal(connectorTopbar, headerActionsHost) : null) : null}
			{!workspace && err && serverGroups.configurationError.length === 0 && <div className="banner banner--error">{err}</div>}
			{!workspace && servers && servers.length > 0 && <div className="cap-mcp-toolbar">
				{servers && servers.length > 0 ? <div className="drawer__summary">{summary}</div> : <span />}
			</div>}
			{workspace && (
				<section className="connector-workspace-add connector-workspace-add--catalog" aria-label={t("workspace.connectors.availableTitle")}>
					{err && <div className="banner banner--error">{err}</div>}
					<ConnectorCatalogPanel
						query={connectorQuery}
						managedCatalog={managedCatalog}
						managedStates={managedStates}
						tabID={activeTabID}
						loginRequired={connectorLoginRequired}
						busyConnectorIDs={busyConnectorIDs}
						onConnectConnector={connectManagedConnector}
						onDisconnectConnector={disconnectManagedConnector}
						onCancelConnector={cancelManagedConnector}
						onBotSettingsChanged={onBotSettingsChanged}
						onTry={onTryConnector}
					/>
				</section>
			)}
			{workspace && detailServer && (
				<div className="management-modal-backdrop connector-detail-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeDetailDialog(); }}>
					<section className="management-modal connector-detail-modal" role="dialog" aria-modal="true" aria-labelledby="connector-detail-title">
						<header className="management-modal__head connector-detail-modal__head">
							<div className="connector-detail-modal__identity">
								<span className={`cap-dot cap-dot--${detailServer.status}`} aria-hidden="true" />
								<div>
									<div id="connector-detail-title" className="management-modal__title">{detailServer.name}</div>
									<div className="management-modal__summary">
										{t("caps.serverDetails")} · {serverStatusLabel(detailServer, t)} · {detailServer.transport}
									</div>
								</div>
							</div>
							<ModalCloseButton label={t("common.close")} onClick={closeDetailDialog} />
						</header>
						<div className="connector-detail-modal__body">
							{err && <div className="banner banner--error">{err}</div>}
							<ServerDetails
								s={detailServer}
								tools={detailServer.toolList ?? []}
								busy={actionBusy}
								onConfirm={() => {
									void removeServers([detailServer.name]).then((ok) => {
										if (ok) closeDetailDialog();
									});
								}}
								onConnectNow={() => void mutate(() => app.ReconnectMCPServer(detailServer.name))}
								onReconnect={() => void mutate(() => app.ReconnectMCPServer(detailServer.name))}
								onConfirmClearAuth={() => void mutate(() => app.ClearMCPServerAuthentication(detailServer.name))}
								onTrustTool={(toolName) => void mutate(() => app.TrustMCPServerTool(detailServer.name, toolName))}
								onTrustTools={(toolNames) => void mutate(() => app.TrustMCPServerTools(detailServer.name, toolNames))}
								onUntrustTool={(toolName) => void mutate(() => app.UntrustMCPServerTool(detailServer.name, toolName))}
								toolsExpanded={expandedServerTools.has(detailServer.name)}
								editing={editing === detailServer.name}
								onEdit={() => setEditing(detailServer.name)}
								onCancelEdit={() => setEditing(null)}
								onUpdate={(input) => {
									void mutate(() => app.UpdateMCPServer(detailServer.name, input)).then((ok) => {
										if (ok) setEditing(null);
									});
								}}
								onToggleTools={() => toggleServerTools(detailServer.name)}
							/>
						</div>
					</section>
				</div>
			)}
				{serverGroups.configurationError.length > 0 && (
					<FailedServersNotice
						servers={serverGroups.configurationError}
						expanded={expandedErrors}
						busy={actionBusy}
						onToggle={toggleError}
						onRetry={(name) => void mutate(() => app.ReconnectMCPServer(name))}
						onRetryMany={(names) => void mutate(() => Promise.all(names.map((name) => app.ReconnectMCPServer(name))))}
					onConfirmClearAuth={(name) => void mutate(() => app.ClearMCPServerAuthentication(name))}
					onConfirm={(name) => void removeServers([name])}
					onConfirmMany={(names) => void removeServers(names)}
					/>
			)}
			{loading && (
				<div className="mem-empty">{t("caps.loading")}</div>
			)}
			{!workspace && !loading && servers.length === 0 && !err && (
				<div className="mem-empty">{t("caps.noServers")}</div>
			)}
			{serverGroups.connecting.length > 0 && (
				<div className="cap-server-section" aria-labelledby="connector-group-connecting">
					<div className="cap-server-section__head">
						<div id="connector-group-connecting" className="cap-server-section__title">连接中 · 暂不可选</div>
						<button
							className="btn btn--small"
							disabled={actionBusy || retryableActiveServerNames.length === 0}
							type="button"
							onClick={() => void mutate(() => Promise.all(retryableActiveServerNames.map((name) => app.ReconnectMCPServer(name))))}
						>
							{t("caps.retryAll")}
						</button>
					</div>
					<ServerGroup
						busy
						servers={serverGroups.connecting}
						expanded={expandedServers}
						expandedTools={expandedServerTools}
						detailsPresentation={workspace ? "dialog" : "inline"}
						editing={editing}
						onConfirm={(name) => void removeServers([name])}
						onEdit={setEditing}
						onCancelEdit={() => setEditing(null)}
						onRetry={(name) => void mutate(() => app.ReconnectMCPServer(name))}
						onReconnect={(name) => void mutate(() => app.ReconnectMCPServer(name))}
						onConfirmClearAuth={(name) => void mutate(() => app.ClearMCPServerAuthentication(name))}
						onTrustTool={(name, toolName) => void mutate(() => app.TrustMCPServerTool(name, toolName))}
						onTrustTools={(name, toolNames) => void mutate(() => app.TrustMCPServerTools(name, toolNames))}
						onUntrustTool={(name, toolName) => void mutate(() => app.UntrustMCPServerTool(name, toolName))}
						onToggle={() => undefined}
						onUpdate={() => undefined}
						onToggleDetails={toggleServer}
						onToggleTools={toggleServerTools}
						selectedServers={selectedServers}
						onSelect={onSelectServer}
					/>
				</div>
			)}
			{serverGroups.connected.length > 0 && (
				<div className="cap-server-section">
					{workspace && (
						<div className="cap-server-section__head">
							<div className="cap-server-section__title connector-configured-title">
								{t("workspace.connectors.configuredCount", { count: serverGroups.connected.length })}
							</div>
							<div className="drawer__summary connector-configured-summary">{summary}</div>
						</div>
					)}
					{!workspace && (
						<div className="cap-server-section__head">
							<div className="cap-server-section__title">已连接 · 可用于当前任务</div>
						</div>
					)}
						<ServerGroup
							busy={actionBusy}
							servers={serverGroups.connected}
							expanded={expandedServers}
							expandedTools={expandedServerTools}
							detailsPresentation={workspace ? "dialog" : "inline"}
							editing={editing}
							onConfirm={(name) => void removeServers([name])}
							onEdit={(name) => { setEditing(name); }}
							onCancelEdit={() => setEditing(null)}
							onRetry={(name) => void mutate(() => app.ReconnectMCPServer(name))}
							onReconnect={(name) => void mutate(() => app.ReconnectMCPServer(name))}
							onConfirmClearAuth={(name) => void mutate(() => app.ClearMCPServerAuthentication(name))}
							onTrustTool={(name, toolName) => void mutate(() => app.TrustMCPServerTool(name, toolName))}
							onTrustTools={(name, toolNames) => void mutate(() => app.TrustMCPServerTools(name, toolNames))}
							onUntrustTool={(name, toolName) => void mutate(() => app.UntrustMCPServerTool(name, toolName))}
							onToggle={(name, on) => void mutate(() => app.SetMCPServerEnabled(name, on))}
							onUpdate={(name, input) =>
								void mutate(() => app.UpdateMCPServer(name, input)).then((ok) => {
									if (ok) setEditing(null);
								})
							}
							onToggleDetails={(name) => {
								if (workspace) {
									setEditing(null);
									setDetailServerName(name);
								} else {
									toggleServer(name);
								}
							}}
							onToggleTools={toggleServerTools}
							selectedServers={selectedServers}
							onSelect={onSelectServer}
						/>
				</div>
			)}
		</section>
	);
}

type SkillLibraryTab = "market" | "installed";
const ALL_PUBLIC_SKILLS_CATEGORY = "全部";
const FEATURED_SKILL_COUNT = 4;

function publicSkillCategories(skill: PublicSkillView): string[] {
	const categories = (skill.categories ?? []).map((category) => category.trim()).filter(Boolean);
	if (categories.length > 0) return Array.from(new Set(categories));
	const legacyCategory = skill.category?.trim();
	return legacyCategory ? [legacyCategory] : [];
}

function publicSkillKey(skill: Pick<PublicSkillView, "namespace" | "slug">): string {
	return `${skill.namespace}/${skill.slug}`.toLowerCase();
}

function formatSkillMetric(value: number): string {
	if (!Number.isFinite(value) || value <= 0) return "0";
	if (value >= 10_000) return `${(value / 10_000).toFixed(value >= 100_000 ? 0 : 1).replace(/\.0$/, "")}万`;
	return new Intl.NumberFormat("zh-CN").format(value);
}

function formatSkillPackageSize(bytes: number): string {
	if (!Number.isFinite(bytes) || bytes <= 0) return "—";
	if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
	return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

function PublicSkillCard({
	skill,
	installed,
	installing,
	onOpen,
	onInstall,
}: {
	skill: PublicSkillView;
	installed: boolean;
	installing: boolean;
	onOpen: () => void;
	onInstall: () => void;
}) {
	return (
		<article className="skill-library-card">
			<button className="skill-library-card__main" type="button" onClick={onOpen}>
				<span className="skill-library-card__head">
					<span className="skill-library-card__icon" aria-hidden="true">
						<Blocks size={23} strokeWidth={1.8} />
						{skill.iconUrl && <img src={skill.iconUrl} alt="" onError={(event) => { event.currentTarget.remove(); }} />}
					</span>
					<span className="skill-library-card__identity">
						<span className="skill-library-card__title">
							<span className="skill-library-card__title-text" title={skill.name || skill.slug}>
								{skill.name || skill.slug}
							</span>
							{skill.verified && <BadgeCheck size={16} aria-label="已认证" />}
						</span>
					</span>
				</span>
				<span className="skill-library-card__description">{skill.description || "暂无技能说明"}</span>
			</button>
			<button
				className={`skill-library-card__install${installed ? " skill-library-card__install--done" : ""}`}
				type="button"
				aria-label={installed ? `技能 ${skill.name || skill.slug} 已安装` : `安装技能 ${skill.name || skill.slug}`}
				title={installed ? "已安装" : "安装技能"}
				disabled={installed || installing}
				onClick={onInstall}
			>
				{installing ? <LoaderCircle className="skill-library-spinner" size={17} aria-hidden /> : installed ? <CheckCircle2 size={17} aria-hidden /> : <Plus size={19} aria-hidden />}
			</button>
		</article>
	);
}

function PublicSkillDetailDialog({
	skill,
	loading,
	error,
	installed,
	installing,
	onClose,
	onInstall,
}: {
	skill: PublicSkillView;
	loading: boolean;
	error: string | null;
	installed: boolean;
	installing: boolean;
	onClose: () => void;
	onInstall: () => void;
}) {
	const t = useT();
	useEffect(() => {
		const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
		document.addEventListener("keydown", closeOnEscape);
		return () => document.removeEventListener("keydown", closeOnEscape);
	}, [onClose]);
	return (
		<div className="skill-detail-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
			<section className="skill-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="skill-detail-title">
				<header className="skill-detail-dialog__header">
					<span className="skill-library-card__icon skill-detail-dialog__icon" aria-hidden="true">
						<Blocks size={27} strokeWidth={1.8} />
						{skill.iconUrl && <img src={skill.iconUrl} alt="" onError={(event) => { event.currentTarget.remove(); }} />}
					</span>
					<div>
						<h2 id="skill-detail-title">{skill.name || skill.slug}{skill.verified && <BadgeCheck size={18} aria-label="已认证" />}</h2>
						<p>@{skill.namespace}/{skill.slug} · v{skill.version}</p>
					</div>
					<ModalCloseButton label={t("common.close")} autoFocus onClick={onClose} />
				</header>
				<div className="skill-detail-dialog__body">
					<p className="skill-detail-dialog__description">{skill.description || "暂无技能说明"}</p>
					{error && <div className="banner banner--error skill-detail-dialog__error">{error}</div>}
					{skill.tags?.length > 0 && <div className="skill-detail-dialog__tags">{skill.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
					<div className="skill-detail-dialog__stats">
						<span><strong>{formatSkillMetric(skill.downloads)}</strong>下载</span>
						<span><strong>{formatSkillMetric(skill.install_count)}</strong>安装</span>
						<span><strong>{formatSkillMetric(skill.stars)}</strong>收藏</span>
						<span><strong>{skill.score ? skill.score.toFixed(1) : "—"}</strong>评分</span>
					</div>
					<div className="skill-detail-dialog__package">
						<span><small>发布者</small>{skill.publisher || skill.ownerName || skill.namespace}</span>
						<span><small>安装包</small>{skill.download_filename || `${skill.slug}.zip`}</span>
						<span><small>大小</small>{formatSkillPackageSize(skill.download_size_bytes)}</span>
						<span><small>完整性</small>{skill.download_sha256 ? `SHA-256 · ${skill.download_sha256.slice(0, 12)}…` : "—"}</span>
					</div>
					{loading && <div className="skill-detail-dialog__loading"><LoaderCircle className="skill-library-spinner" size={15} />正在获取最新详情…</div>}
				</div>
				<footer className="skill-detail-dialog__footer">
					<button className="btn btn--primary" type="button" disabled={installed || installing} onClick={onInstall}>
						{installing ? <LoaderCircle className="skill-library-spinner" size={15} /> : installed ? <PackageCheck size={15} /> : <Download size={15} />}
						{installing ? "正在安装" : installed ? "已安装" : "安装技能"}
					</button>
				</footer>
			</section>
		</div>
	);
}

// SkillsSettingsPage combines the public skill catalog with local skill
// management. Catalog browsing is unauthenticated; installation is delegated to
// the desktop backend so archive size and extraction paths stay within safe limits.
export function SkillsSettingsPage({
	workspace = false,
	selectedSkills,
	onSelectSkill,
}: {
	workspace?: boolean;
	selectedSkills?: readonly string[];
	onSelectSkill?: (name: string) => void;
}) {
	const t = useT();
	const [snapshotKey, setSnapshotKey] = useState("");
	const [view, setView] = useState<SkillsSettingsView | null>(null);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState<string | null>(null);
	const [libraryTab, setLibraryTab] = useState<SkillLibraryTab>("market");
	const [skillQuery, setSkillQuery] = useState("");
	const [catalogCategory, setCatalogCategory] = useState(ALL_PUBLIC_SKILLS_CATEGORY);
	const [catalog, setCatalog] = useState<PublicSkillsPageView>({ items: [], next_cursor: "", has_more: false });
	const [catalogLoading, setCatalogLoading] = useState(true);
	const [catalogKeyword, setCatalogKeyword] = useState("");
	const [catalogError, setCatalogError] = useState<string | null>(null);
	const [catalogNotice, setCatalogNotice] = useState<string | null>(null);
	const [featuredOffset, setFeaturedOffset] = useState(0);
	const [detailSkill, setDetailSkill] = useState<PublicSkillView | null>(null);
	const [detailLoading, setDetailLoading] = useState(false);
	const [detailError, setDetailError] = useState<string | null>(null);
	const [installingKey, setInstallingKey] = useState("");
	const [installedCatalogKeys, setInstalledCatalogKeys] = useState<Set<string>>(() => new Set());
	const [headerActionsHost, setHeaderActionsHost] = useState<HTMLElement | null>(null);
	const [batchMode, setBatchMode] = useState(false);
	const [batchSelection, setBatchSelection] = useState<Set<string>>(() => new Set());
	const [batchUninstallArmed, setBatchUninstallArmed] = useState(false);
	const categoryScrollerRef = useHorizontalDragScroll(libraryTab, view);
	const catalogRequestRef = useRef(0);

	useLayoutEffect(() => {
		setHeaderActionsHost(workspace ? document.getElementById("capability-workspace-header-actions") : null);
	}, [workspace]);
	const reload = useCallback(async () => {
		const [meta, tabs] = await Promise.all([
			app.Meta().catch(() => null),
			app.ListTabs().catch(() => []),
		]);
		const key = settingsSnapshotKey(meta, tabs);
		setSnapshotKey(key);
		const cached = key ? skillsSettingsSnapshot : null;
		if (cached?.key === key) {
			setView(cached.value);
		} else {
			setView(null);
		}
		try {
			const next = normalizeSkillsSettingsView(await app.SkillsSettings());
			skillsSettingsSnapshot = { key, value: next };
			setView(next);
			setErr(null);
		} catch (e) {
			setErr(capabilityErrorMessage("skills.load", e, "加载技能失败，请稍后重试。"));
			if (cached?.key !== key) setView(normalizeSkillsSettingsView(null));
		}
	}, []);
	useEffect(() => { void reload(); }, [reload]);
	const loadInitialCatalog = useCallback(async (fallbackToInstalled = false, keyword = "") => {
		keyword = keyword.trim();
		const requestID = ++catalogRequestRef.current;
		setCatalogKeyword(keyword);
		setCatalog({ items: [], next_cursor: "", has_more: false });
		setCatalogLoading(true);
		setCatalogError(null);
		try {
			const itemsByKey = new Map<string, PublicSkillView>();
			let cursor = "";
			for (;;) {
				const next = keyword
					? await app.SearchPublicSkills(keyword, PUBLIC_SKILLS_PAGE_SIZE, cursor)
					: await app.PublicSkills(PUBLIC_SKILLS_PAGE_SIZE, cursor);
				if (requestID !== catalogRequestRef.current) return;
				if (!next || !Array.isArray(next.items)) throw new Error("技能库接口不可用");
				for (const item of next.items) itemsByKey.set(publicSkillKey(item), item);
				const nextCursor = next.next_cursor?.trim() || "";
				const hasMore = Boolean(next.has_more);
				setCatalog({ items: [...itemsByKey.values()], next_cursor: nextCursor, has_more: hasMore });
				if (!hasMore) break;
				if (!nextCursor || nextCursor === cursor) throw new Error("技能库分页游标无效");
				cursor = nextCursor;
			}
		} catch (error) {
			if (requestID !== catalogRequestRef.current) return;
			setCatalogError(capabilityErrorMessage("skills.catalog.load", error, "加载技能库失败，请稍后重试。"));
			if (fallbackToInstalled) setLibraryTab("installed");
		} finally {
			if (requestID !== catalogRequestRef.current) return;
			setCatalogLoading(false);
		}
	}, []);
	useEffect(() => { void loadInitialCatalog(true); }, [loadInitialCatalog]);
	useEffect(() => {
		if (libraryTab !== "market") return;
		const keyword = skillQuery.trim();
		if (keyword === catalogKeyword) return;
		const timer = window.setTimeout(() => void loadInitialCatalog(false, keyword), 250);
		return () => window.clearTimeout(timer);
	}, [catalogKeyword, libraryTab, loadInitialCatalog, skillQuery]);

	const mutate = async (fn: () => Promise<unknown>) => {
		setBusy(true);
		setErr(null);
		try {
			await fn();
			await reload();
			return true;
		} catch (e) {
			const message = capabilityErrorMessage("skills.operation", e, "更新技能设置失败，请稍后重试。");
			await reload();
			setErr(message);
			return false;
		} finally {
			setBusy(false);
		}
	};

	const installedSkills = useMemo(() => {
		return (view?.skills ?? []).filter((skill) => skill.scope !== "builtin");
	}, [view]);
	const filteredSkills = useMemo(() => {
		const q = skillQuery.trim().toLowerCase();
		if (!q) return installedSkills;
		return installedSkills.filter((sk) => {
			const text = [sk.displayName, sk.name, "/" + sk.name, sk.description, sk.scope, sk.runAs].join(" ").toLowerCase();
			return text.includes(q);
		});
	}, [installedSkills, skillQuery]);
	const selectedBatchSkills = useMemo(
		() => installedSkills.filter((skill) => batchSelection.has(skill.name)),
		[batchSelection, installedSkills],
	);
	const uninstallableBatchSkills = useMemo(
		() => selectedBatchSkills.filter((skill) => skill.uninstallable && skill.uninstallId),
		[selectedBatchSkills],
	);
	useEffect(() => {
		const installedNames = new Set(installedSkills.map((skill) => skill.name));
		setBatchSelection((previous) => {
			const next = new Set([...previous].filter((name) => installedNames.has(name)));
			return next.size === previous.size ? previous : next;
		});
	}, [installedSkills]);

	const catalogCategories = useMemo(() => {
		const seen = new Set<string>();
		for (const item of catalog.items) {
			for (const category of publicSkillCategories(item)) seen.add(category);
		}
		return [ALL_PUBLIC_SKILLS_CATEGORY, ...seen];
	}, [catalog.items]);
	useEffect(() => {
		if (!catalogCategories.includes(catalogCategory)) setCatalogCategory(ALL_PUBLIC_SKILLS_CATEGORY);
	}, [catalogCategories, catalogCategory]);
	const filteredCatalog = useMemo(() => {
		const q = skillQuery.trim().toLowerCase();
		return catalog.items.filter((item) => {
			const categories = publicSkillCategories(item);
			if (catalogCategory !== ALL_PUBLIC_SKILLS_CATEGORY && !categories.includes(catalogCategory)) return false;
			if (!q || q === catalogKeyword.toLowerCase()) return true;
			return [item.name, item.slug, item.namespace, item.description, item.category, ...categories, ...(item.tags ?? [])].join(" ").toLowerCase().includes(q);
		});
	}, [catalog.items, catalogCategory, catalogKeyword, skillQuery]);
	const featuredSkills = useMemo(() => {
		if (catalog.items.length < FEATURED_SKILL_COUNT) return [];
		return Array.from({ length: FEATURED_SKILL_COUNT }, (_, index) => (
			catalog.items[(featuredOffset + index) % catalog.items.length]
		)).filter((item): item is PublicSkillView => Boolean(item));
	}, [catalog.items, featuredOffset]);
	const installedNames = useMemo(() => new Set((view?.skills ?? []).map((item) => item.name.toLowerCase())), [view]);
	const catalogIconsBySlug = useMemo(() => {
		const icons = new Map<string, string>();
		for (const item of catalog.items) {
			const iconUrl = item.iconUrl?.trim();
			if (iconUrl) icons.set(item.slug.toLowerCase(), iconUrl);
		}
		return icons;
	}, [catalog.items]);
	const catalogNamesBySlug = useMemo(() => {
		const names = new Map<string, string>();
		for (const item of catalog.items) {
			const name = item.name?.trim();
			if (name) names.set(item.slug.toLowerCase(), name);
		}
		return names;
	}, [catalog.items]);
	const isCatalogSkillInstalled = useCallback((item: PublicSkillView) => (
		installedNames.has(item.slug.toLowerCase()) || installedCatalogKeys.has(publicSkillKey(item))
	), [installedCatalogKeys, installedNames]);

	const openCatalogDetail = async (item: PublicSkillView) => {
		setDetailSkill(item);
		setDetailLoading(true);
		setDetailError(null);
		try {
			const detail = await app.PublicSkill(item.namespace, item.slug);
			if (detail?.skill) setDetailSkill(detail.skill);
		} catch (error) {
			setDetailError(capabilityErrorMessage("skills.catalog.detail", error, "获取技能详情失败，请稍后重试。"));
		} finally {
			setDetailLoading(false);
		}
	};

	const installCatalogSkill = async (item: PublicSkillView) => {
		const key = publicSkillKey(item);
		setInstallingKey(key);
		setCatalogError(null);
		setCatalogNotice(null);
		try {
			const result = await app.InstallPublicSkill(item.namespace, item.slug);
			if (!result) throw new Error("安装结果为空");
			setInstalledCatalogKeys((previous) => new Set(previous).add(key));
			setCatalogNotice(`已安装 ${result.name || item.name}${result.reload_required ? "，将在下次刷新后生效" : ""}`);
			await reload();
		} catch (error) {
			const message = capabilityErrorMessage("skills.catalog.install", error, "安装技能失败，请稍后重试。");
			setCatalogError(message);
			if (detailSkill) setDetailError(message);
		} finally {
			setInstallingKey("");
		}
	};
	const exitBatchMode = () => {
		setBatchMode(false);
		setBatchSelection(new Set());
		setBatchUninstallArmed(false);
	};
	const toggleBatchSkill = (name: string) => {
		setBatchUninstallArmed(false);
		setBatchSelection((previous) => {
			const next = new Set(previous);
			if (next.has(name)) next.delete(name);
			else next.add(name);
			return next;
		});
	};
	const updateBatchEnabled = async (enabled: boolean) => {
		const targets = [...selectedBatchSkills];
		const succeeded = await mutate(async () => {
			for (const skill of targets) await app.SetSkillEnabled(skill.name, enabled);
		});
		if (succeeded) exitBatchMode();
	};
	const uninstallBatchSkills = async () => {
		if (!batchUninstallArmed) {
			setBatchUninstallArmed(true);
			return;
		}
		const targets = [...uninstallableBatchSkills];
		const succeeded = await mutate(async () => {
			for (const skill of targets) await app.UninstallPublicSkill(skill.uninstallId!);
		});
		if (succeeded) exitBatchMode();
	};

	if (!view) return <div className="empty">{t("caps.loading")}</div>;
	const actionBusy = busy || !snapshotKey;
	const showFeaturedSkills = libraryTab === "market" && !skillQuery.trim() && featuredSkills.length > 0;
	const topbar = (
		<div className="skill-library__topbar">
			{libraryTab === "installed" ? (
				<button className="skill-library__market-link" type="button" onClick={() => { exitBatchMode(); setLibraryTab("market"); setSkillQuery(""); }}>
					<ChevronLeft size={16} aria-hidden="true" />技能库
				</button>
			) : <span />}
			<div className="skill-library__actions">
				{libraryTab === "market" && (
					<label className="skill-library__search">
						<Search size={16} aria-hidden="true" />
						<input type="search" aria-label="搜索技能" placeholder="搜索技能" value={skillQuery} onChange={(event) => setSkillQuery(Array.from(event.target.value).slice(0, 128).join(""))} />
					</label>
				)}
				<button
					className={`skill-library__action${libraryTab === "installed" ? " is-active" : ""}`}
					type="button"
					aria-pressed={libraryTab === "installed"}
					onClick={() => { setLibraryTab("installed"); setSkillQuery(""); }}
				>
					<PackageCheck size={17} aria-hidden="true" />我安装的<span>{installedSkills.length}</span>
				</button>
				<button
					className="skill-library__action"
					type="button"
					disabled={actionBusy}
					onClick={() => void mutate(async () => { const path = await app.PickSkillFolder(); if (path) await app.AddSkillPath(path); })}
				>
					<Plus size={17} aria-hidden="true" />添加技能
				</button>
			</div>
		</div>
	);

	return (
		<section className="mem-section skill-library">
			{err && <div className="banner banner--error">{err}</div>}
			{workspace ? (headerActionsHost ? createPortal(topbar, headerActionsHost) : null) : topbar}
			{libraryTab === "market" ? (
				<>
					{showFeaturedSkills && (
						<section className="skill-library__featured" aria-labelledby="featured-skills-title">
							<header className="skill-library__section-head">
								<h2 id="featured-skills-title">精选技能</h2>
								<button type="button" onClick={() => setFeaturedOffset((offset) => (offset + FEATURED_SKILL_COUNT) % catalog.items.length)}>
									<RefreshCw size={15} aria-hidden="true" />换一换
								</button>
							</header>
							<div className="skill-library-grid skill-library__featured-grid">
								{featuredSkills.map((item) => <PublicSkillCard key={`featured-${publicSkillKey(item)}`} skill={item} installed={isCatalogSkillInstalled(item)} installing={installingKey === publicSkillKey(item)} onOpen={() => void openCatalogDetail(item)} onInstall={() => void installCatalogSkill(item)} />)}
							</div>
						</section>
					)}
					<div className="skill-library__section-head skill-library__section-head--catalog"><h2>技能</h2></div>
					<div ref={categoryScrollerRef} className="skill-library__categories" role="tablist" aria-label="技能分类">
						{catalogCategories.map((category) => (
							<button
								className={catalogCategory === category ? "is-active" : ""}
								key={category}
								role="tab"
								aria-selected={catalogCategory === category}
								type="button"
								onClick={() => setCatalogCategory(category)}
							>
								{category}
							</button>
						))}
					</div>
				</>
			) : (
				<>
					<header className="skill-library__installed-heading">
						<div className="skill-library__installed-title">
							<h2>我安装的</h2>
							<span aria-label={`共 ${installedSkills.length} 个技能`}>{installedSkills.length}</span>
						</div>
						{!batchMode && installedSkills.length > 0 && (
							<button className="skill-library__batch-manage" type="button" onClick={() => setBatchMode(true)}>
								批量管理<Settings size={15} strokeWidth={1.8} aria-hidden="true" />
							</button>
						)}
					</header>
					{batchMode && (
						<div className="skill-library__batch-toolbar" aria-label="批量管理技能">
							<div>
								<span>已选择 {selectedBatchSkills.length} 项</span>
								<button type="button" disabled={actionBusy} onClick={() => { setBatchUninstallArmed(false); setBatchSelection(new Set(installedSkills.map((skill) => skill.name))); }}>全选</button>
								<button type="button" disabled={actionBusy || selectedBatchSkills.length === 0} onClick={() => { setBatchUninstallArmed(false); setBatchSelection(new Set()); }}>取消全选</button>
							</div>
							<div>
								<button type="button" disabled={actionBusy || selectedBatchSkills.length === 0} onClick={() => void updateBatchEnabled(true)}>启用</button>
								<button type="button" disabled={actionBusy || selectedBatchSkills.length === 0} onClick={() => void updateBatchEnabled(false)}>停用</button>
								<button type="button" disabled={actionBusy || uninstallableBatchSkills.length === 0} onClick={() => void uninstallBatchSkills()}>{batchUninstallArmed ? "确认卸载" : "卸载"}</button>
								<button className="skill-library__batch-cancel" type="button" disabled={actionBusy} onClick={exitBatchMode}>完成</button>
							</div>
						</div>
					)}
				</>
			)}
			{libraryTab === "market" ? (
				<>
					{catalogError && <div className="banner banner--error skill-library__banner">{catalogError}</div>}
					{catalogNotice && <div className="banner banner--success skill-library__banner">{catalogNotice}</div>}
					{catalogLoading && catalog.items.length === 0 ? (
						<div className="skill-library-grid" aria-label="正在加载技能库">{Array.from({ length: 6 }, (_, index) => <div className="skill-library-card skill-library-card--skeleton" key={index} />)}</div>
					) : filteredCatalog.length === 0 ? (
						<div className="skill-library-empty" aria-live="polite"><Library size={28} /><strong>{catalogKeyword ? "没有匹配的技能" : "技能库暂时没有可用技能"}</strong><span>{catalogKeyword ? "换一个关键词试试。" : "稍后重试，或先管理本地已安装技能。"}</span>{!catalogKeyword && <button className="btn btn--small" type="button" disabled={catalogLoading} onClick={() => void loadInitialCatalog(false)}><RefreshCw size={14} />重试</button>}</div>
					) : (
						<div className="skill-library-grid">
							{filteredCatalog.map((item) => <PublicSkillCard key={publicSkillKey(item)} skill={item} installed={isCatalogSkillInstalled(item)} installing={installingKey === publicSkillKey(item)} onOpen={() => void openCatalogDetail(item)} onInstall={() => void installCatalogSkill(item)} />)}
						</div>
					)}
					{catalogLoading && catalog.items.length > 0 && (
						<div className="skill-library__load-more" role="status" aria-live="polite">
							<LoaderCircle className="skill-library-spinner" size={15} />正在加载全部技能
						</div>
					)}
				</>
			) : err ? null : installedSkills.length === 0 ? (
				workspace ? (
					<div className="primary-workspace__empty skills-workspace-empty" role="status">
						<Blocks size={28} strokeWidth={1.6} aria-hidden="true" />
						<strong>{t("workspace.skills.emptyTitle")}</strong>
						<span>{t("workspace.skills.emptyDescription")}</span>
					</div>
				) : (
					<div className="mem-empty">{t("caps.noSkills")}</div>
				)
			) : filteredSkills.length === 0 ? (
				<div className="mem-empty">{t("caps.noSkillMatches")}</div>
			) : (
				<div className="cap-skills skill-library__installed-grid">
					{filteredSkills.map((sk) => (
						<SkillRow
							key={sk.name}
							skill={sk}
							displayName={catalogNamesBySlug.get((sk.uninstallId || sk.name).toLowerCase())}
							iconUrl={sk.iconUrl || catalogIconsBySlug.get(sk.name.toLowerCase())}
							busy={actionBusy}
							onToggleEnabled={(enabled) => void mutate(() => app.SetSkillEnabled(sk.name, enabled))}
							onUninstall={sk.uninstallable && sk.uninstallId ? () => mutate(() => app.UninstallPublicSkill(sk.uninstallId!)) : undefined}
							selected={selectedSkills?.includes(sk.name)}
							onSelect={onSelectSkill ? () => onSelectSkill(sk.name) : undefined}
							batchMode={batchMode}
							batchSelected={batchSelection.has(sk.name)}
							onBatchSelect={() => toggleBatchSkill(sk.name)}
						/>
					))}
				</div>
			)}
			{detailSkill && <PublicSkillDetailDialog skill={detailSkill} loading={detailLoading} error={detailError} installed={isCatalogSkillInstalled(detailSkill)} installing={installingKey === publicSkillKey(detailSkill)} onClose={() => { setDetailSkill(null); setDetailError(null); }} onInstall={() => void installCatalogSkill(detailSkill)} />}
		</section>
	);
}
