import { lazy, Suspense, useCallback, useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Box,
  Brain,
  CircleHelp,
  Cpu,
  FlaskConical,
  Keyboard,
  Palette,
  PlugZap,
  RefreshCw,
  ShieldCheck,
  SlidersHorizontal,
} from "lucide-react";
import { asArray } from "../../../lib/array";
import { BRAND_PROVIDER_ID } from "../../../lib/brand";
import { normalizeBotSettings } from "../../../lib/botSettings";
import { reportClientError } from "../../../lib/clientError";
import { useDeferredClose } from "../../../lib/useMountTransition";
import { app } from "../../../lib/bridge";
import { useT, type DictKey } from "../../../lib/i18n";
import { providerIsConfigured, providerRequiresKey } from "../../../lib/providerModels";
import { modelProviderLabel } from "../../../lib/providerDisplay";
import {
  applyTheme,
  getTheme,
  getThemeStyle,
  normalizeThemePreference,
  normalizeThemeStyleForTheme,
  type Theme,
  type ThemeStyle,
} from "../../../lib/theme";
import { applyTextSize, getTextSize, type TextSize } from "../../../lib/textSize";
import {
  applyFontFamily,
  applyMonoFontFamily,
  getFontFamily,
  getMonoFontFamily,
  getCustomFontName,
  getCustomMonoFontName,
  setCustomFontName,
  setCustomMonoFontName,
  type FontFamily,
  type MonoFontFamily,
} from "../../../lib/fontFamily";
import { normalizeToolApprovalMode } from "../../../lib/types";
import { normalizeStatusBarItems } from "../../../lib/statusBarItems";
import {
  filterSubscriptionModelCandidates,
  filterModelInfosForSubscriptionAccess,
  type SubscriptionAccountState,
  type SubscriptionAccess,
} from "../../../lib/subscriptionModelAccess";
import type { BotSettingsView, ProviderView, SettingsCoreView, SettingsModelDetailsView, SettingsTab, SettingsView, SharedModelCatalogState } from "../../../lib/types";
import { AppearanceSection } from "./AppearanceSettingsSection";
import { ModalCloseButton } from "../../../components/ModalCloseButton";
import { UpdatesSettingsSection } from "./UpdatesSettingsSection";
import { ShortcutsSettingsSection } from "./ShortcutsSettingsSection";
import { PermissionsSettingsSection } from "./PermissionsSettingsSection";
import { SandboxSettingsSection } from "./SandboxSettingsSection";
import { ExperimentalSettingsSection } from "./ExperimentalSettingsSection";
import { qqBotAdded } from "./BotsSettingsSection";
import { settingsModelMeta } from "./ModelPicker";
import { ModelsSettingsSection, modelRoutesForSettings, normalizeReasoningLanguage, normalizeReasoningProtocol, type SettingsModelRoute } from "./ModelsSettingsSection";
import {
  FeedbackSettingsSection,
  useFeedbackDraft,
  type FeedbackAccountState,
} from "./FeedbackSettingsSection";
import {
  GeneralSettingsSection,
  closeBehaviorLabel,
  normalizeAutoPlan,
  normalizeCloseBehavior,
  normalizeDesktopLayoutStyle,
  normalizeDisplayMode,
  normalizeStatusBarStyle,
} from "./GeneralSettingsSection";
import {
  cancelSettingsScopeLoads,
  invalidateSettingsDetails,
  loadSettingsCore,
  loadSettingsDetails,
  peekSettingsCore,
  peekSettingsDetails,
} from "../data/settingsSnapshotCache";

const SETTINGS_GROUPS: ReadonlyArray<{
  label: DictKey;
  tabs: ReadonlyArray<{ id: SettingsTab; Icon: typeof SlidersHorizontal }>;
}> = [
  {
    label: "settings.group.workspace",
    tabs: [
      { id: "general", Icon: SlidersHorizontal },
      { id: "appearance", Icon: Palette },
      { id: "shortcuts", Icon: Keyboard },
    ],
  },
  {
    label: "settings.group.assistant",
    tabs: [
      { id: "models", Icon: Cpu },
      { id: "mcp", Icon: PlugZap },
      { id: "memory", Icon: Brain },
    ],
  },
  {
    label: "settings.group.system",
    tabs: [
      { id: "permissions", Icon: ShieldCheck },
      { id: "sandbox", Icon: Box },
      { id: "experimental", Icon: FlaskConical },
      { id: "updates", Icon: RefreshCw },
    ],
  },
  {
    label: "settings.group.support",
    tabs: [{ id: "feedback", Icon: CircleHelp }],
  },
];
export type SettingsInitialFocus = { target: "bot-allowlist"; connectionId?: string };
const NOOP = () => {};
const unavailableModelCatalog: SharedModelCatalogState = { state: "unavailable" };

const MCPServersSettingsPage = lazy(() => import("../../../components/CapabilitiesPanel").then((module) => ({ default: module.MCPServersSettingsPage })));
const SkillsSettingsPage = lazy(() => import("../../../components/CapabilitiesPanel").then((module) => ({ default: module.SkillsSettingsPage })));
const MemorySettingsPage = lazy(() => import("../../../components/MemoryPanel").then((module) => ({ default: module.MemorySettingsPage })));

function settingsModelProviders(settings: SettingsView): ProviderView[] {
  return [
    ...asArray(settings.officialProviders),
    ...asArray(settings.providers).filter((provider) => !(provider.builtIn && provider.name === BRAND_PROVIDER_ID)),
  ];
}

// SettingsPanel is the desktop settings centre — a centred modal with left
// navigation and a right content area. It hosts all settings pages plus MCP,
// Skills, and Memory management, replacing the old per-feature drawers.
export function SettingsPanel({
  onClose,
  onChanged,
  initialTab,
  activeTabId,
  settingsScopeKey,
  agentRunning = false,
  embedded = false,
  modelCatalog = unavailableModelCatalog,
  subscriptionAccess = { state: "unavailable" },
  subscriptionAccountState = "unavailable",
  feedbackAccountState = "unavailable",
  feedbackAccountScopeKey = "",
  onRequireAccount = NOOP,
  superpowersEnabled = false,
  onSuperpowersEnabledChange = NOOP,
}: {
  onClose: () => void;
  onChanged: (settings?: SettingsView | null) => void;
  initialTab?: SettingsTab;
  initialFocus?: SettingsInitialFocus;
  activeTabId?: string | null;
  settingsScopeKey?: string;
  agentRunning?: boolean;
  embedded?: boolean;
  modelCatalog?: SharedModelCatalogState;
  subscriptionAccess?: SubscriptionAccess;
  subscriptionAccountState?: SubscriptionAccountState;
  feedbackAccountState?: FeedbackAccountState;
  feedbackAccountScopeKey?: string;
  onRequireAccount?: () => void;
  superpowersEnabled?: boolean;
  onSuperpowersEnabledChange?: (enabled: boolean) => void;
}) {
  const panelInstanceID = useId();
  const resolvedSettingsScopeKey = settingsScopeKey ?? (activeTabId ? `tab:${activeTabId}` : `panel:${panelInstanceID}`);
  const t = useT();
  const panelTitle = initialTab === "appearance" ? t("personalization.settingsTitle") : t("settings.title");
  const [s, setS] = useState<SettingsView | null>(() => {
    const cached = peekSettingsCore(resolvedSettingsScopeKey);
    return cached ? normalizeSettingsView(settingsViewFromCore(cached)) : null;
  });
  const [loadingSettings, setLoadingSettings] = useState(() => !peekSettingsCore(resolvedSettingsScopeKey));
  const [settingsLoadFailed, setSettingsLoadFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [warning, setWarning] = useState<string | null>(null);
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const [themeStyle, setThemeStyleState] = useState<ThemeStyle>(() => getThemeStyle(getTheme()));
  const [textSize, setTextSizeState] = useState<TextSize>(getTextSize());
  const [fontFamily, setFontFamilyState] = useState<FontFamily>(getFontFamily());
  const [monoFontFamily, setMonoFontFamilyState] = useState<MonoFontFamily>(getMonoFontFamily());
  const [customFontName, setCustomFontNameState] = useState<string>(getCustomFontName());
  const [customMonoFontName, setCustomMonoFontNameState] = useState<string>(getCustomMonoFontName());
  const contentRef = useRef<HTMLElement | null>(null);
  const settingsRequestRef = useRef(0);
  const detailsRequestRef = useRef(0);
  const [detailsState, setDetailsState] = useState<"idle" | "loading" | "ready" | "unavailable">("idle");
  const [tab, setTab] = useState<SettingsTab>(initialTab === "providers" ? "models" : initialTab === "bots" ? "mcp" : initialTab ?? "general");
  const feedback = useFeedbackDraft({
    accountState: feedbackAccountState,
    accountScopeKey: feedbackAccountScopeKey,
    onRequireAccount,
  });
  // Play the modal exit animation, then let the parent unmount us.
  const { status, requestClose: completeClose } = useDeferredClose(onClose, 240);
  const requestClose = useCallback(() => {
    void feedback.requestClose(completeClose);
  }, [completeClose, feedback.requestClose]);

  const reload = useCallback(async (force = true) => {
    const requestID = settingsRequestRef.current + 1;
    settingsRequestRef.current = requestID;
    setLoadingSettings(!peekSettingsCore(resolvedSettingsScopeKey));
    setSettingsLoadFailed(false);
    try {
      const core = await loadSettingsCore(resolvedSettingsScopeKey, async () => (
        typeof app.SettingsCore === "function"
          ? app.SettingsCore()
          : settingsCoreFromLegacy(await app.Settings())
      ), force);
      const next = normalizeSettingsView(settingsViewFromCore(core));
      if (settingsRequestRef.current !== requestID) return null;
      setS(next);
      return next;
    } catch {
      if (settingsRequestRef.current !== requestID) return null;
      setS(null);
      setSettingsLoadFailed(true);
      return null;
    } finally {
      if (settingsRequestRef.current === requestID) setLoadingSettings(false);
    }
  }, [resolvedSettingsScopeKey]);
  useEffect(() => {
    detailsRequestRef.current += 1;
    setDetailsState("idle");
    const cached = peekSettingsCore(resolvedSettingsScopeKey);
    setS(cached ? normalizeSettingsView(settingsViewFromCore(cached)) : null);
    setLoadingSettings(!cached);
    void reload(true);
    if (initialTab) setTab(initialTab === "providers" ? "models" : initialTab === "bots" ? "mcp" : initialTab);
    return () => cancelSettingsScopeLoads(resolvedSettingsScopeKey);
  }, [initialTab, reload, resolvedSettingsScopeKey]);
  const modelsRelatedTab = tab === "models";
  const reloadDetails = useCallback(async (force = true) => {
    const requestID = detailsRequestRef.current + 1;
    detailsRequestRef.current = requestID;
    setDetailsState("loading");
    try {
      const details = await loadSettingsDetails(resolvedSettingsScopeKey, async () => (
        typeof app.SettingsModelDetails === "function"
          ? app.SettingsModelDetails()
          : settingsDetailsFromLegacy(await app.Settings())
      ), force);
      if (detailsRequestRef.current !== requestID) return null;
      const normalized = normalizeSettingsDetails(details);
      setS((current) => current ? normalizeSettingsView({ ...current, ...normalized }) : current);
      setDetailsState("ready");
      return normalized;
    } catch {
      if (detailsRequestRef.current === requestID) setDetailsState("unavailable");
      return null;
    }
  }, [resolvedSettingsScopeKey]);
  useEffect(() => {
    if (!modelsRelatedTab || detailsState !== "idle") return;
    const cached = peekSettingsDetails(resolvedSettingsScopeKey);
    if (cached) {
      const normalized = normalizeSettingsDetails(cached);
      setS((current) => current ? normalizeSettingsView({ ...current, ...normalized }) : current);
    }
    void reloadDetails(true);
  }, [detailsState, modelsRelatedTab, reloadDetails, resolvedSettingsScopeKey]);
  const modelProviders = useMemo(() => s ? settingsModelProviders(s) : [], [s]);

  const allowedModelRoutes = useMemo(() => {
    if (!s) return [];
    const marketplace = modelCatalog.state === "ready" ? modelCatalog.marketplace : undefined;
    const rawSettingsRoutes = modelRoutesForSettings(s, modelProviders);
    const settingsRoutes = filterSubscriptionModelCandidates(
      rawSettingsRoutes,
      marketplace,
      subscriptionAccess,
      (route) => {
        const separator = route.ref.indexOf("/");
        const providerName = separator > 0 ? route.ref.slice(0, separator) : "";
        const model = separator > 0 ? route.ref.slice(separator + 1) : "";
        return { model, billingSource: route.billingSource, disabled: !providerName || !model };
      },
    );
    const routeModels = modelCatalog.state === "ready" ? modelCatalog.routes : [];
    const remoteRoutes = routeModels.flatMap((model): SettingsModelRoute[] => (
      model.billingSource === "subscription_pool" || model.billingSource === "newapi_byok"
        ? [{ ref: model.ref, billingSource: model.billingSource }]
        : []
    ));
    const remoteSourcesByRef = new Map<string, Set<SettingsModelRoute["billingSource"]>>();
    for (const route of remoteRoutes) {
      const sources = remoteSourcesByRef.get(route.ref) ?? new Set<SettingsModelRoute["billingSource"]>();
      sources.add(route.billingSource);
      remoteSourcesByRef.set(route.ref, sources);
    }
    const ambiguousRefs = new Set(
      [...remoteSourcesByRef.entries()].filter(([, sources]) => sources.size !== 1).map(([ref]) => ref),
    );
    const filteredRouteModels = filterModelInfosForSubscriptionAccess(
      routeModels,
      marketplace,
      subscriptionAccess,
    ).filter((model) => model.billingSource === "subscription_pool" || model.billingSource === "newapi_byok");
    const combined: SettingsModelRoute[] = [
      ...settingsRoutes,
      ...filteredRouteModels
        .filter((model) => !ambiguousRefs.has(model.ref))
        .map((model) => ({ ref: model.ref, billingSource: model.billingSource! })),
    ];
    const seen = new Set<string>();
    return combined.filter((route) => {
      const identity = `${route.ref}\u0000${route.billingSource}`;
      if (seen.has(identity)) return false;
      seen.add(identity);
      return true;
    });
  }, [modelCatalog, modelProviders, s, subscriptionAccess]);
  const modelOptionsLoading = detailsState !== "ready"
    || modelCatalog.state === "loading"
    || subscriptionAccess.state === "loading";
  useEffect(() => {
    if (!s) return;
    const nextTheme = normalizeThemePreference(s.desktopTheme);
    const nextStyle = normalizeThemeStyleForTheme(s.desktopThemeStyle, nextTheme);
    setThemeState(nextTheme);
    setThemeStyleState(nextStyle);
  }, [s?.desktopTheme, s?.desktopThemeStyle]);

  // apply runs a mutation, re-reads settings, and refreshes the topbar/model.
  const apply = useCallback(async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setErr(null);
    setWarning(null);
    try {
      const result = await fn();
      const refreshDetails = detailsState === "ready";
      invalidateSettingsDetails(resolvedSettingsScopeKey);
      const core = await reload(true);
      const details = refreshDetails ? await reloadDetails(true) : null;
      const next = core && details ? normalizeSettingsView({ ...core, ...details }) : core;
      if (next) setS(next);
      onChanged(next);
      if (typeof result === "string" && result.trim()) {
        setWarning(result.trim());
      }
    } catch (e) {
      setErr(settingsMutationErrorMessage(e, t));
    } finally {
      setBusy(false);
    }
  }, [detailsState, reload, reloadDetails, onChanged, resolvedSettingsScopeKey, t]);
  useEffect(() => {
    if (!err && !warning && !settingsLoadFailed) return;
    const content = contentRef.current;
    if (!content) return;
    if (typeof content.scrollTo === "function") {
      content.scrollTo({ top: 0, behavior: "smooth" });
    } else {
      content.scrollTop = 0;
    }
  }, [err, warning, settingsLoadFailed]);

  // Close on Esc
  useEffect(() => {
    if (embedded) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !document.querySelector("[data-anchored-popover='active']")) requestClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [embedded, requestClose]);

  // The settings-reliant pages (general, models, permissions,
  // sandbox, appearance, updates) need SettingsView loaded. MCP, Skills, and Memory
  // load their own data and render regardless.
  const needsSettings = tab === "general" || tab === "models" || tab === "permissions" || tab === "sandbox" || tab === "appearance" || tab === "updates";
  const lazySettingsPageFallback = <div className="empty">{t("settings.loading")}</div>;

  return (
    <div
      className={embedded ? "primary-workspace settings-workspace" : "management-modal-backdrop settings-modal-backdrop"}
      data-state={embedded ? undefined : status}
      onClick={embedded ? undefined : (e) => { if (e.target === e.currentTarget) requestClose(); }}
    >
      <div className={embedded ? "settings-workspace__surface" : "management-modal settings-modal"} data-state={embedded ? undefined : status}>
        {!embedded && <ModalCloseButton className="settings-modal__close" label={t("common.close")} showTooltip={false} onClick={requestClose} disabled={feedback.submitting} />}

        <div className={`settings-center${embedded ? " settings-center--embedded" : ""}`}>
          {!embedded && <nav className="settings-center__nav" aria-label={panelTitle}>
            {SETTINGS_GROUPS.map((group) => (
              <section className="settings-center__navgroup" key={group.label} aria-labelledby={`settings-nav-${group.label}`}>
                <div className="settings-center__navgroup-label" id={`settings-nav-${group.label}`}>{t(group.label)}</div>
                {group.tabs.map(({ id, Icon }) => (
                  <button
                    key={id}
                    type="button"
                    className={`settings-center__navitem${tab === id ? " settings-center__navitem--active" : ""}`}
                    aria-current={tab === id ? "page" : undefined}
                    data-selected={tab === id ? "true" : undefined}
                    style={tab === id ? {
                      backgroundColor: "rgba(0, 0, 0, 0.12)",
                    } : undefined}
                    disabled={feedback.submitting}
                    onClick={() => setTab(id)}
                  >
                    <Icon className="settings-center__navitem-icon" size={16} aria-hidden="true" />
                    <span className="settings-center__navitem-copy">
                      <span>{settingsTabLabel(id, t)}</span>
                      {s && <small>{settingsTabMeta(id, s, t, subscriptionAccountState)}</small>}
                    </span>
                  </button>
                ))}
              </section>
            ))}
          </nav>}
          <main ref={contentRef} className="settings-center__content">
            {needsSettings && settingsLoadFailed && (
              <div className="banner banner--error settings-load-error" role="alert">
                <span>{t("settings.loadFailed")}</span>
                <button className="btn btn--small" type="button" onClick={() => void reload()}>{t("common.retry")}</button>
              </div>
            )}
            {needsSettings && err && <div className="banner banner--error">{err}</div>}
            {needsSettings && warning && <div className="banner banner--warning">{warning}</div>}
            {modelsRelatedTab && detailsState === "unavailable" && (
              <div className="banner banner--error settings-model-details-error" role="alert">
                <span>{t("settings.loadFailed")}</span>
                <button className="btn btn--small" type="button" onClick={() => void reloadDetails()}>{t("common.retry")}</button>
              </div>
            )}
            {needsSettings && !s ? (
              loadingSettings ? <div className="empty">{t("settings.loading")}</div> : null
            ) : (
              <>
                {tab === "general" && s && <SettingsPageShell key={tab} s={s} tab={tab} busy={busy} apply={apply}><GeneralSettingsSection s={s} busy={busy} apply={apply} agentRunning={agentRunning} /></SettingsPageShell>}
                {tab === "models" && s && <SettingsPageShell key={tab} s={s} tab={tab} busy={busy || detailsState !== "ready"} apply={apply}><ModelsSettingsSection s={s} modelProviders={modelProviders} busy={busy || detailsState !== "ready"} apply={apply} modelRoutes={allowedModelRoutes} modelOptionsLoading={modelOptionsLoading} subscriptionAccountState={subscriptionAccountState} /></SettingsPageShell>}
                {tab === "mcp" && <SettingsPageShell key={tab} s={s} tab={tab} busy={false} apply={apply}><Suspense fallback={lazySettingsPageFallback}><MCPServersSettingsPage /></Suspense></SettingsPageShell>}
                {tab === "skills" && <SettingsPageShell key={tab} s={s} tab={tab} busy={false} apply={apply}><Suspense fallback={lazySettingsPageFallback}><SkillsSettingsPage /></Suspense></SettingsPageShell>}
                {tab === "memory" && <SettingsPageShell key={tab} s={s} tab={tab} busy={false} apply={apply}><Suspense fallback={lazySettingsPageFallback}><MemorySettingsPage /></Suspense></SettingsPageShell>}
                {tab === "shortcuts" && <SettingsPageShell key={tab} s={s} tab={tab} busy={false} apply={apply}><ShortcutsSettingsSection /></SettingsPageShell>}
                {tab === "permissions" && s && <SettingsPageShell key={tab} s={s} tab={tab} busy={busy} apply={apply}><PermissionsSettingsSection settings={s} busy={busy} apply={apply} /></SettingsPageShell>}
                {tab === "sandbox" && s && <SettingsPageShell key={tab} s={s} tab={tab} busy={busy} apply={apply}><SandboxSettingsSection settings={s} busy={busy} apply={apply} /></SettingsPageShell>}
                {tab === "experimental" && <SettingsPageShell key={tab} s={s} tab={tab} busy={false} apply={apply}><ExperimentalSettingsSection superpowersEnabled={superpowersEnabled} onSuperpowersEnabledChange={onSuperpowersEnabledChange} /></SettingsPageShell>}
                {tab === "appearance" && s && (
                  <SettingsPageShell key={tab} s={s} tab={tab} busy={busy} apply={apply}>
                    <AppearanceSection
                      theme={theme}
                      textSize={textSize}
                      fontFamily={fontFamily}
                      monoFontFamily={monoFontFamily}
                      customFontName={customFontName}
                      customMonoFontName={customMonoFontName}
                      onTheme={(nextTheme) => {
                        applyTheme(nextTheme, themeStyle, { persist: false });
                        setThemeState(nextTheme);
                        void apply(() => app.SetDesktopAppearance(nextTheme, themeStyle));
                      }}
                      onTextSize={(size) => {
                        applyTextSize(size);
                        setTextSizeState(size);
                      }}
                      onFontFamily={(font) => {
                        applyFontFamily(font);
                        setFontFamilyState(font);
                      }}
                      onMonoFontFamily={(font) => {
                        applyMonoFontFamily(font);
                        setMonoFontFamilyState(font);
                      }}
                      onCustomFontNameChange={(name) => {
                        setCustomFontNameState(name);
                        setCustomFontName(name);
                        applyFontFamily("custom");
                      }}
                      onCustomMonoFontNameChange={(name) => {
                        setCustomMonoFontNameState(name);
                        setCustomMonoFontName(name);
                        applyMonoFontFamily("custom");
                      }}
                    />
                  </SettingsPageShell>
                )}
                {tab === "updates" && s && (
                  <SettingsPageShell key={tab} s={s} tab={tab} busy={busy} apply={apply}>
                    <UpdatesSettingsSection
                      configPath={s.configPath}
                      checkUpdates={s.checkUpdates}
                      settingsBusy={busy}
                      applySettings={apply}
                    />
                  </SettingsPageShell>
                )}
                {tab === "feedback" && (
                  <SettingsPageShell key={tab} s={s} tab={tab} busy={feedback.submitting} apply={apply}>
                    <FeedbackSettingsSection controller={feedback} />
                  </SettingsPageShell>
                )}
              </>
            )}
          </main>
        </div>
      </div>
    </div>
  );
}
function settingsMutationErrorMessage(e: unknown, t: ReturnType<typeof useT>): string {
  const message = String((e as Error)?.message ?? e);
  if (message.includes("finish or cancel the current turn") && message.includes("before changing ")) {
    reportClientError("settings.save", e, { fallbackMessage: t("settings.activeWorkError") });
    return t("settings.activeWorkError");
  }
  return reportClientError("settings.save", e, {
    fallbackMessage: "保存设置失败，请稍后重试。",
  }).userMessage;
}
function SettingsPageShell({ s: _s, tab, hideHeader = false, children }: { s: SettingsView | null; tab: SettingsTab; busy: boolean; apply: (fn: () => Promise<unknown>) => Promise<void>; hideHeader?: boolean; children: ReactNode }) {
  const t = useT();
  const descKey = `settings.pageDesc.${tab}` as keyof typeof import("../../../locales/zh").zh;
  const desc = t(descKey as any);
  return (
    <div className={`settings-page settings-page--${settingsPageKind(tab)} settings-page--${tab}`}>
      {!hideHeader && (
        <div className="settings-page__header">
          <h2 className="settings-page__title">{settingsTabPageTitle(tab, t)}</h2>
          {typeof desc === "string" && desc !== `settings.pageDesc.${tab}` && <p className="settings-page__desc">{desc}</p>}
        </div>
      )}
      {children}
    </div>
  );
}
function settingsPageKind(tab: SettingsTab): "form" | "manager" {
  switch (tab) {
    case "models":
    case "mcp":
    case "skills":
    case "memory":
      return "manager";
    default:
      return "form";
  }
}

function settingsTabPageTitle(id: SettingsTab, t: ReturnType<typeof useT>): string {
  switch (id) {
    case "general": return t("settings.title");
    case "mcp": return t("settings.tab.mcp");
    case "skills": return t("settings.tab.skills");
    case "memory": return t("settings.tab.memory");
    case "shortcuts": return t("settings.tab.shortcuts");
    default: return settingsTabLabel(id, t);
  }
}

function settingsTabLabel(id: SettingsTab, t: ReturnType<typeof useT>): string {
  switch (id) {
    case "general":
      return t("settings.tab.general");
    case "models":
      return t("settings.tab.models");
    case "providers":
      return t("settings.tab.providers");
    case "bots":
      return t("settings.tab.bots");
    case "mcp":
      return t("settings.tab.mcp");
    case "skills":
      return t("settings.tab.skills");
    case "memory":
      return t("settings.tab.memory");
    case "shortcuts":
      return t("settings.tab.shortcuts");
    case "permissions":
      return t("settings.tab.permissions");
    case "sandbox":
      return t("settings.tab.sandbox");
    case "appearance":
      return t("settings.tab.appearance");
    case "updates":
      return t("settings.tab.updates");
    case "experimental":
      return t("settings.tab.experimental");
    case "feedback":
      return t("settings.tab.feedback");
  }
}

function settingsTabMeta(
  id: SettingsTab,
  s: SettingsView,
  t: ReturnType<typeof useT>,
  subscriptionAccountState: SubscriptionAccountState,
): string {
  switch (id) {
    case "models":
      if (
        subscriptionAccountState === "anonymous"
        && (s.selectedModelSummary?.billingSource || s.defaultBillingSource) === "subscription_pool"
      ) {
        return t("settings.anonymousDefaultModel");
      }
      if (s.selectedModelSummary) {
        const summary = s.selectedModelSummary;
        if (!summary.ref) return t("common.none");
        if (!summary.provider) return summary.displayName || summary.model || summary.ref;
        return `${modelProviderLabel(summary.provider, undefined, t)} · ${summary.displayName || summary.model}`;
      }
      return settingsModelMeta(s, t);
    case "general":
      return closeBehaviorLabel(normalizeCloseBehavior(s.closeBehavior), t);
    case "providers":
      return t("settings.providerCount", { n: s.providers.length });
    case "bots":
      return botSettingsMeta(s.bot, t);
    case "mcp":
      return t("caps.connectorsTab");
    case "skills":
      return t("caps.skillsTab");
    case "memory":
      return t("settings.tabSub.memory");
    case "shortcuts":
      return t("settings.tabSub.shortcuts");
    case "permissions":
      return permissionModeLabel(s.permissions.mode, t);
    case "sandbox":
      return sandboxModeLabel(s.sandbox.bash, t);
    case "appearance":
      return t("settings.appearanceMeta");
    case "updates":
      return t("settings.updatesMeta");
    case "experimental":
      return t("settings.experimentalMeta");
    case "feedback":
      return t("settings.feedbackMeta");
  }
}

function botSettingsMeta(bot: BotSettingsView, t: ReturnType<typeof useT>): string {
  const normalized = normalizeBotSettings(bot);
  const connections = normalized.connections.length + (qqBotAdded(normalized.qq) ? 1 : 0);
  if (connections === 0) return t("settings.botNoConnections");
  if (!normalized.enabled) return t("settings.botDisabledWithConnections", { n: connections });
  return t("settings.botConnectionCount", { n: connections });
}

function normalizeProviderView(p: ProviderView): ProviderView {
  const visionModels = asArray(p.visionModels);
  const requiresKey = providerRequiresKey(p);
  return {
    ...p,
    builtIn: Boolean(p.builtIn),
    added: Boolean(p.added),
    models: asArray(p.models),
    chatModels: asArray(p.chatModels ?? p.models),
    visionModels,
    visionModelsConfigured: Boolean(p.visionModelsConfigured ?? visionModels.length > 0),
    modelsUrl: p.modelsUrl ?? "",
    reasoningProtocol: normalizeReasoningProtocol(p.reasoningProtocol),
    supportedEfforts: asArray(p.supportedEfforts),
    requiresKey,
    configured: providerIsConfigured({ ...p, requiresKey }),
    keySource: p.keySource ?? "",
    keySourcePath: p.keySourcePath ?? "",
  };
}

function settingsCoreFromLegacy(view: SettingsView): SettingsCoreView {
  const { providers: _providers, officialProviders: _officialProviders, providerKinds: _providerKinds, ...core } = view;
  const ref = view.defaultModel ?? "";
  const separator = ref.indexOf("/");
  const provider = separator > 0 ? ref.slice(0, separator) : "";
  const model = separator > 0 ? ref.slice(separator + 1) : ref;
  return {
    ...core,
    selectedModelSummary: view.selectedModelSummary ?? {
      ref,
      provider,
      model,
      displayName: model,
      billingSource: view.defaultBillingSource ?? "",
      availability: "unverified",
      accountState: "unknown",
    },
  };
}

function settingsDetailsFromLegacy(view: SettingsView): SettingsModelDetailsView {
  return {
    providers: view.providers,
    officialProviders: view.officialProviders,
    providerKinds: view.providerKinds,
  };
}

function settingsViewFromCore(core: SettingsCoreView): SettingsView {
  return {
    ...core,
    providers: [],
    officialProviders: [],
    providerKinds: [],
  };
}

function normalizeSettingsDetails(view: SettingsModelDetailsView): SettingsModelDetailsView {
  return {
    providers: asArray(view.providers).map(normalizeProviderView),
    officialProviders: asArray(view.officialProviders).map(normalizeProviderView),
    providerKinds: asArray(view.providerKinds),
  };
}

function normalizeSettingsView(view: SettingsView | null | undefined): SettingsView | null {
  if (!view) return null;
  const permissions = view.permissions ?? { mode: "ask", allow: [], ask: [], deny: [] };
  const sandbox = view.sandbox ?? { bash: "enforce", network: false, workspaceRoot: "", allowWrite: [], shell: "auto" };
  const agent = view.agent ?? { temperature: 0, maxSteps: 0, plannerMaxSteps: 0, systemPrompt: "", coldResumePrune: true, reasoningLanguage: "auto" };
  agent.plannerMaxSteps = Number.isFinite(agent.plannerMaxSteps) ? Math.max(0, Math.trunc(agent.plannerMaxSteps)) : 0;
  agent.maxSteps = Number.isFinite(agent.maxSteps) ? Math.max(0, Math.trunc(agent.maxSteps)) : 0;
  agent.reasoningLanguage = normalizeReasoningLanguage(agent.reasoningLanguage);
  return {
    ...view,
    providers: asArray(view.providers).map(normalizeProviderView),
    officialProviders: asArray(view.officialProviders).map(normalizeProviderView),
    providerKinds: asArray(view.providerKinds),
    permissions: {
      ...permissions,
      allow: asArray(permissions.allow),
      ask: asArray(permissions.ask),
      deny: asArray(permissions.deny),
    },
    sandbox: {
      ...sandbox,
      allowWrite: asArray(sandbox.allowWrite),
    },
    agent,
    bot: normalizeBotSettings(view.bot),
    autoPlan: normalizeAutoPlan(view.autoPlan),
    defaultToolApprovalMode: normalizeToolApprovalMode(view.defaultToolApprovalMode),
    autoApproveTools: Boolean(view.autoApproveTools ?? view.bypass),
    bypass: Boolean(view.autoApproveTools ?? view.bypass),
    desktopLanguage: "zh",
    desktopLayoutStyle: normalizeDesktopLayoutStyle(view.desktopLayoutStyle),
    desktopTheme: normalizeThemePreference(view.desktopTheme),
    desktopThemeStyle: normalizeThemeStyleForTheme(view.desktopThemeStyle, normalizeThemePreference(view.desktopTheme)),
    closeBehavior: normalizeCloseBehavior(view.closeBehavior),
    displayMode: normalizeDisplayMode(view.displayMode),
    statusBarStyle: normalizeStatusBarStyle(view.statusBarStyle),
    statusBarItems: normalizeStatusBarItems(view.statusBarItems),
    checkUpdates: view.checkUpdates !== false,
  };
}


function permissionModeLabel(mode: string, t: ReturnType<typeof useT>): string {
  switch (mode) {
    case "allow":
      return t("settings.modeAllowShort");
    case "deny":
      return t("settings.modeDenyShort");
    default:
      return t("settings.modeAskShort");
  }
}

function sandboxModeLabel(mode: string, t: ReturnType<typeof useT>): string {
  return mode === "off" ? t("settings.bashOffShort") : t("settings.bashEnforceShort");
}
