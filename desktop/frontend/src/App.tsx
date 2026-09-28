import { lazy, Suspense, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent, PointerEvent as ReactPointerEvent, SetStateAction } from "react";
import { ShellExpandProvider, useShellExpand } from "./lib/shellExpand";
import { useWorkspaceNavigation, type CapabilityWorkspaceView, type WorkspaceView } from "./app/useWorkspaceNavigation";
import { useDesktopPreferences } from "./app/useDesktopPreferences";
import gsap from "gsap";
import { useGSAP } from "@gsap/react";
import { Flip } from "gsap/Flip";
import { ScrollToPlugin } from "gsap/ScrollToPlugin";
gsap.registerPlugin(useGSAP, Flip, ScrollToPlugin);
import {
  Activity,
  CircleHelp,
  Command,
  Download,
  SquarePen,
  PanelLeft,
  PanelRight,
  FileDown,
  FileImage,
  FileText,
  FileJson,
  History,
  MessageSquare,
  Settings as SettingsIcon,
  Pencil,
  Trash2,
  AlarmClock,
  Brain,
  Cpu,
  Palette,
  RefreshCw,
  CircleUserRound,
  Users,
  FolderKanban,
  Hammer,
  Link,
  ShieldUser,
  ChevronUp,
  ChevronRight,
  LogOut,
  WalletCards,
  Camera,
  Sparkles,
  Library,
} from "lucide-react";
import { useToast } from "./lib/toast";
import { reportClientError } from "./lib/clientError";
import {
  currentComposerExpertTraceId,
  recordComposerExpertPaint,
  recordComposerExpertTrace,
} from "./lib/composerExpertTrace";
import { composerExpertShouldUseTargetIdentity } from "./lib/composerExpertHandoff";
import type { PendingExpertActivation } from "./lib/expertSelection";
import {
  resolveComposerModelEntitlementPresentation,
} from "./lib/composerModelEntitlement";
import { asArray } from "./lib/array";
import { BRAND_DISPLAY_NAME, formatBrandVersion } from "./lib/brand";
import { clearLegacyLangPref, readLegacyLangPref, useI18n, useT, type Translator } from "./lib/i18n";
import { projectTreeRuntimeKeyForTabs, useController, type Item, type LiveStream } from "./features/session";
import type { SuperpowerComposerSend, SuperpowerSessionAdapter } from "./features/superpowers";
import { channelSessionRefreshTarget } from "./features/session/historyRevalidation";
import { app, onConversationActivity, onEvent, onModelRouteChanged, onProjectTreeChanged, waitForBackendReady } from "./lib/bridge";
import {
  type ComposerCapabilitySelection,
  ensureUsableMCPServer,
  isMCPManagementCommand,
  isMCPPromptInvocation,
  loadEnabledConnectorSelections,
  MCP_SERVER_REMOVED_EVENT,
  mcpConnectTarget,
  removeConnectorFromEnabledSelections,
  saveEnabledConnectorSelections,
} from "./lib/composerCapabilities";
import type { ComposerCapabilitySubmit } from "./lib/types";
import { generativeMusic, isGenerativeMusicEnabled } from "./lib/generative-music";
import { playSuccessChime } from "./lib/sound";
import { getSuperpowersEnabled, setSuperpowersEnabled } from "./lib/experimentalFeatures";
import {
  resolveDisplayedSendAvailability,
  sendAvailabilityAllowsControllerActions,
} from "./lib/sendAvailability";
import {
  resolveSubscriptionModelGateLabel,
  useDelayedSubscriptionModelPending,
} from "./lib/subscriptionModelGatePresentation";
import {
  activeTopicTurnsFromTree,
  cancelSubscriptionMarketplaceRefresh,
  resolveModelRouteAccess,
  resolveSubscriptionAccountState,
  resolveSubscriptionAccess,
  resolveActiveModelRoute,
  resolveSubscriptionDefaultRepair,
  resolveSubscriptionModelInvocationAccess,
  isBlankSessionEligibleForSubscriptionRepair,
  runSubscriptionDefaultRepair,
  settleSubscriptionDefaultRepairRunState,
  shouldBlockSubscriptionDefaultRepair,
  subscriptionDefaultRepairAttemptKey,
  subscriptionDefaultRepairScopeKey,
  subscriptionMarketplaceNeedsRefresh,
  subscriptionRepairFactsKey,
  topicTurnEvidenceKey,
  type SubscriptionModelInvocationAccess,
  type SubscriptionDefaultRepairRunState,
  type SubscriptionMarketplaceRefreshScopeState,
  type SubscriptionMarketplaceView,
  type TopicTurnEvidence,
} from "./lib/subscriptionModelAccess";
import { refreshSubscriptionMarketplaceOnce } from "./lib/subscriptionMarketplaceRefresh";
import { Transcript } from "./components/Transcript";
import { Composer, type ComposerDraftStore } from "./features/composer/ui/Composer";
import { TodoPanel } from "./components/TodoPanel";
import { ApprovalModal } from "./components/ApprovalModal";
import { AskCard } from "./components/AskCard";
import { UndoRewindBanner } from "./components/UndoRewindBanner";
import { ClearContextCard } from "./components/ClearContextCard";
import { StatusBar } from "./components/StatusBar";
import { SendAvailabilityBanner } from "./components/SendAvailabilityBanner";
import { SubscriptionModelGateBanner } from "./components/SubscriptionModelGateBanner";
import { HistoryLoadErrorBanner } from "./components/HistoryLoadErrorBanner";
import { CommandPalette, type PaletteItem } from "./components/CommandPalette";
import type { HistoryPanelKind } from "./components/HistoryPanel";
import { UpdateBanner } from "./components/UpdateBanner";
import { ContextPanel } from "./components/ContextPanel";
import { ProjectTree } from "./features/workspace/ui/ProjectTree";
import { WorkspacePanel } from "./features/workspace/ui/WorkspacePanel";
import { CHAT_MIN_WIDTH, WORKSPACE_RESIZER_WIDTH, useWorkspaceDockLayout } from "./features/workspace/controller/useWorkspaceDockLayout";
import { Tooltip } from "./components/Tooltip";
import { StartupSplash } from "./components/StartupSplash";
import { AppChrome } from "./components/AppChrome";
import { ShortcutsCheatsheet } from "./components/ShortcutsCheatsheet";
import type { ExpertSelection } from "./features/experts";
import type { ProjectCapabilityKind } from "./features/projects/ui/CloudProjectsSectionImpl";
import { ProjectCapabilityDialog, type ProjectCapabilityCatalogItem } from "./features/projects/ui/ProjectCapabilityDialog";
import { projectGateway } from "./features/projects/data/projectGateway";
import type { CloudProjectCapabilityBinding } from "./features/projects/model/projectTypes";
import {
  applyProjectCapabilityBindings,
  asProjectComposerSubmitError,
  availableProjectCapabilitySelection,
  classifyProjectComposerFirstSend,
  ensureProjectComposerAdapter,
  ensureNewProjectConversationAdapter,
  initializeProjectComposerDefaults,
  parseProjectComposerKey,
  projectComposerAdapterForTab,
  projectComposerBlankSessionTarget,
  projectComposerCapabilitiesForTab,
  projectComposerCapabilitySubmit,
  projectComposerFailureMessage,
  planProjectComposerScopeChange,
  planProjectComposerLeave,
  releaseProjectComposerAdapter,
  submitProjectComposerTurn,
  type ProjectComposerAdapter,
} from "./features/projects/model/projectComposerAdapter";
import { createProjectComposerAttempt, type ProjectComposerAttempt } from "./lib/projectComposerDiagnostics";
import { expertSelectionFromState } from "./features/experts/model/expertSelection";
import { projectExpertSelections } from "./features/projects/model/projectExpertSelection";
import { useExpertCatalog } from "./features/experts/controller/useExpertCatalog";
import { expertAvatarSrcForInfo } from "./lib/expertAssets";
import { hasRecoverableCloudIdentity, requiresReauthentication } from "./features/account/store/accountStore";
import { accountBridge } from "./features/account/bridge/accountBridge";
import { useAccountNavigation } from "./features/account/controller/useAccountNavigation";
import { useAccountSidebar } from "./features/account/controller/useAccountSidebar";
import { scheduleSettingsCorePrefetch, settingsSnapshotScopeKey } from "./features/settings/data/settingsSnapshotCache";
import "./features/automation/heartbeat/heartbeat.css";
import "./features/automation/workspace/AutomationWorkspace.css";
import { CopyButton } from "./components/CopyButton";
import { parseTodos } from "./lib/tools";
import { shouldShowTodoPanel, todoDismissalKey } from "./lib/todoVisibility";
import {
  type BotConnectionView,
  type BotRuntimeStatusView,
  type BotSettingsView,
  type CapabilityTaskScope,
  type CollaborationMode,
  type ComposerInsertRequest,
  type ComposerSessionIdentity,
  type DesktopStartupSettingsView,
  type Mode,
	  type ModelInfo,
  type ModelMarketplaceView,
  type SharedModelCatalogState,
  modeHasPlan,
  type Scene,
  type SceneState,
  type SessionMeta,
  type SettingsView,
  type TabMeta,
  type TokenMode,
  type ToolApprovalMode,
} from "./lib/types";
import {
  composerProfileFromMeta,
  composerProfileFromTab,
  composerProfileMode,
  composerProfileWithMode,
  controllerComposerProfileCollaborationMode,
  defaultComposerProfile,
  displayedComposerProfileCollaborationMode,
  hydrateComposerProfileFromMeta,
  hydrateComposerProfilesFromTabs,
  patchComposerProfile,
  pruneUserPlanModeIntents,
  resolvePlanRestoreTabId,
  shouldRestoreUserPlanModeForProfile,
  updateUserPlanModeIntent,
  type ComposerProfile,
  type ComposerProfileField,
  type UserPlanModeIntents,
} from "./lib/composerProfile";
import {
  restorableToolApprovalMode,
  toggleYoloToolApprovalMode,
  type RestorableToolApprovalMode,
} from "./lib/toolApprovalMode";
import {
  CREATION_SIDEBAR_MIN_WIDTH,
  RIGHT_DOCK_MAX_WIDTH,
  RIGHT_DOCK_PREVIEW_DEFAULT_WIDTH,
  RIGHT_DOCK_PREVIEW_MIN_WIDTH,
  RIGHT_DOCK_TREE_MAX_WIDTH,
  RIGHT_DOCK_TREE_MIN_WIDTH,
  DEFAULT_WORKSPACE_PANEL_STATE,
  type RightDockMode,
  type WorkspacePanelState,
  SIDEBAR_MAX_WIDTH,
  SIDEBAR_MIN_WIDTH,
  clampCreationSidebarWidth,
  clampRightDockPreviewWidth,
  clampRightDockTreeWidth,
  clampSidebarWidth,
  defaultRightDockTreeWidth,
  defaultSidebarWidth,
  saveRightDockPreviewWidth,
  saveRightDockTreeWidth,
  saveSidebarCollapsed,
  saveSidebarWidth,
  useLayoutStore,
} from "./store/layout";
import { useOverlayStore } from "./store/overlays";
import { hydrateDisplayMode } from "./lib/displayMode";
import { sessionActivityTime } from "./lib/session";
import {
  blankSessionRequestScenePresentation,
  normalizeScenePresentation,
} from "./lib/scenePresentation";
import { enqueueOpenTopicRequest, supersedeOpenTopicRequests, type PendingOpenTopicRequest } from "./lib/openTopicCoalescing";
import { composerSessionIdentityFromSource } from "./lib/composerSessionIdentity";
import {
  applyTheme,
  clearLegacyThemePreference,
  defaultStyleForTheme,
  getTheme,
  getThemeStyle,
  isThemeStyle,
  readLegacyThemePreference,
  type Theme,
} from "./lib/theme";
import { applyTextSize, DEFAULT_TEXT_SIZE, getTextSize, nextTextSize } from "./lib/textSize";
import { useViewportHeightVar, useWindowStatePersistence } from "./lib/windowState";
import { shouldAutoCollapseSidebarForViewport, workspacePanelAriaMinWidth } from "./lib/workspaceLayout";
import { createRafResizeUpdater } from "./lib/resizeDrag";
import { useGlobalShortcut } from "./lib/keyboardShortcuts";
import { topicShortcutIndexFromEvent, useTopicShortcuts, type TopicShortcutEntry } from "./lib/topicShortcuts";
import { composerDraftKeyForTab } from "./lib/composerDraftKey";
import {
  conversationAttentionAccountScope,
  conversationAttentionIdentity,
  readConversationUnreadCompletions,
  shouldMarkTurnDoneUnread,
  withUnreadCompletion,
  withoutUnreadCompletion,
  writeConversationUnreadCompletions,
  type ConversationUnreadCompletions,
} from "./lib/conversationAttention";
import { shouldRefreshWorkspaceForEvent } from "./lib/workspaceArtifactRefresh";
import { formatSkillToken } from "./lib/skillTokens";
import type { CapabilityWelcomePresentation, CreationScenarioSkill } from "./components/Welcome";
import { findCreationScenarioSkill } from "./features/composer/model/creationScenarioSkills";
import { settingsModelCatalogFingerprint } from "./lib/modelCatalogFingerprint";
import {
  hasReliableComposerSelectionSnapshot,
  resolveModelSwitcherTransition,
  shouldHoldComposerSelectionPresentation,
  type ModelSwitcherTransition,
} from "./lib/modelSwitcherTransition";

type SubscriptionRepairFacts =
  | { key: string; state: "loading" }
  | {
      key: string;
      state: "settled";
      routes?: ModelInfo[];
      settings?: SettingsView;
      marketplace?: ModelMarketplaceView;
    };

function isReadOnlyManagementCommand(command: string): boolean {
  return /^\/(?:help|status|model|skill)(?:\s+(?:list|show)(?:\s+\S+)?)?$/.test(command)
    || isMCPManagementCommand(command);
}

const HistoryPanel = lazy(() => import("./components/HistoryPanel").then((module) => ({ default: module.HistoryPanel })));
const loadSettingsFeature = () => import("./features/settings");
const SettingsPanel = lazy(() => loadSettingsFeature().then((module) => ({ default: module.SettingsPanel })));
const UpdateCenterDialog = lazy(() => import("./components/UpdateCenterDialog").then((module) => ({ default: module.UpdateCenterDialog })));
const SkillsWorkspace = lazy(() => import("./components/CapabilitiesPanel").then((module) => ({ default: module.SkillsSettingsPage })));
const ConnectorsWorkspace = lazy(() => import("./components/CapabilitiesPanel").then((module) => ({ default: module.MCPServersSettingsPage })));
const ProjectDirectory = lazy(() => import("./features/workspace/ui/ProjectDirectory").then((module) => ({ default: module.ProjectDirectory })));
const AccountCenter = lazy(() => import("./features/account/ui/AccountCenter").then((module) => ({ default: module.AccountCenter })));
const ExpertDirectory = lazy(() => import("./features/experts/ui/ExpertDirectory").then((module) => ({ default: module.ExpertDirectory })));
const HeartbeatPanel = lazy(() => import("./features/automation/heartbeat/HeartbeatPanel").then((module) => ({ default: module.HeartbeatPanel })));
const SuperpowerWorkspace = lazy(() => import("./features/superpowers").then((module) => ({ default: module.SuperpowerWorkspace })));
const ArtifactLibraryPage = lazy(() => import("./features/artifacts/ArtifactLibraryPage").then((module) => ({ default: module.ArtifactLibraryPage })));

const SIDEBAR_FLOAT_BREAKPOINT = 820;
const CREATION_SIDEBAR_FLOAT_BREAKPOINT = 1200;
const DESKTOP_ACCOUNT_CONTEXT_KEY = "desktop";
const brandAccountAvatar = new URL("./assets/brand-symbol.png", import.meta.url).href;

function stripGoalResearchFlags(arg: string): string {
  const parts = arg.trim().split(/\s+/).filter(Boolean);
  while (parts.length > 0) {
    const flag = parts[0].toLowerCase();
    if (flag !== "--research" && flag !== "--auto-research" && flag !== "--deep" && flag !== "--simple" && flag !== "--no-research") break;
    parts.shift();
  }
  return parts.join(" ");
}

function hasGoalResearchFlag(arg: string): boolean {
  const first = arg.trim().split(/\s+/, 1)[0]?.toLowerCase();
  return first === "--research" || first === "--auto-research" || first === "--deep" || first === "--simple" || first === "--no-research";
}

function isThemeMode(value: string): value is Theme {
  return value === "auto" || value === "light" || value === "dark";
}

function SidebarAccountAvatar({ src, menu = false }: { src: string; menu?: boolean }) {
  const size = menu ? 28 : 22;
  return (
    <span className={`sidebar-account-avatar${menu ? " sidebar-account-avatar--menu" : ""}`} aria-hidden="true">
      <img src={src || brandAccountAvatar} alt="" width={size} height={size} />
    </span>
  );
}
type ExpertTeamSwitchConfirm =
  | { kind: "recent"; selection: ExpertSelection }
  | { kind: "directory" };

const CAPABILITY_WORKSPACE_NAV_ITEMS = [
  { id: "experts", icon: ShieldUser, labelKey: "sidebar.experts" },
  { id: "skills", icon: Hammer, labelKey: "creation.sidebar.skills" },
  { id: "connectors", icon: Link, labelKey: "workspace.connectors.nav" },
] as const;

function CapabilityWorkspaceNav({
  active,
  onSelect,
}: {
  active: CapabilityWorkspaceView;
  onSelect: (view: CapabilityWorkspaceView) => void;
}) {
  const t = useT();
  return (
    <nav className="capability-workspace-nav" aria-label={t("workspace.capabilities.navLabel")}>
      {CAPABILITY_WORKSPACE_NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const selected = active === item.id;
        return (
          <button
            key={item.id}
            type="button"
            className={`capability-workspace-nav__item${selected ? " capability-workspace-nav__item--active" : ""}`}
            aria-current={selected ? "page" : undefined}
            onClick={() => onSelect(item.id)}
          >
            <Icon size={16} aria-hidden="true" />
            <span>{t(item.labelKey)}</span>
          </button>
        );
      })}
    </nav>
  );
}

function CapabilityWorkspaceLoading({ view }: { view: CapabilityWorkspaceView }) {
  const t = useT();
  const label = view === "experts"
    ? t("sidebar.experts")
    : view === "skills"
      ? t("workspace.skills.title")
      : t("workspace.connectors.title");
  return (
    <div
      className="capability-workspace-loading"
      data-capability-view={view}
      role="status"
      aria-live="polite"
      aria-label={`${label} - ${t("common.loading")}`}
    >
      <strong className="capability-workspace-loading__target">{label}</strong>
      <span className="capability-workspace-loading__status">{t("common.loading")}</span>
    </div>
  );
}

function rememberRecentExpertSelection(list: ExpertSelection[], selection: ExpertSelection): ExpertSelection[] {
  return [
    selection,
    ...list.filter((item) => item.id !== selection.id),
  ].slice(0, 5);
}

const SHOW_CONTEXT_DOCK = false;
type HistoryViewState = {
  kind: HistoryPanelKind;
  sessions: SessionMeta[];
  trashedSessions: SessionMeta[];
  initialWorkspaceRoot?: string;
};
type SidebarImPlatform = "qq" | "feishu" | "lark" | "weixin";
type SidebarImStatus = "connected" | "disabled" | "pending" | "error" | "disconnected";
type SidebarImConnection = {
  id: string;
  connectionId: string;
  platform: SidebarImPlatform;
  title: string;
  platformLabel: string;
  subtitle: string;
  status: SidebarImStatus;
  statusLabel: string;
  remoteId: string;
  sessionId: string;
  sessionSource: string;
  scope: "global" | "project";
  workspaceRoot: string;
  allowAll: boolean;
  allowlistEnabled: boolean;
  allowlistUsers: string[];
  allowlistMatched: boolean;
};
type SidebarImTopicSource = {
  platform: SidebarImPlatform;
  label: string;
  title: string;
  remoteId: string;
  connectionId: string;
};
type SidebarImConnectionDetailProps = {
  connection: SidebarImConnection;
  onClose: () => void;
  onOpenSession: () => void;
  onOpenSettings: () => void;
  onManageAllowlist: () => void;
};

function isSidebarImConnection(connection: BotConnectionView): boolean {
  return connection.provider === "feishu" || connection.provider === "weixin";
}

function sidebarImPlatform(connection: BotConnectionView): SidebarImPlatform {
  if (connection.provider === "weixin") return "weixin";
  return connection.domain === "lark" ? "lark" : "feishu";
}

function sidebarImPlatformLabel(platform: SidebarImPlatform, translate: Translator): string {
  if (platform === "qq") return "QQ";
  if (platform === "lark") return "Lark";
  if (platform === "weixin") return translate("settings.botWeixin");
  return translate("settings.botFeishu");
}

function botMappingScope(mapping: BotConnectionView["sessionMappings"][number] | null | undefined, connectionWorkspaceRoot: string): "global" | "project" {
  if (mapping?.scope === "project") return "project";
  if ((mapping?.workspaceRoot ?? "").trim()) return "project";
  return connectionWorkspaceRoot.trim() ? "project" : "global";
}

function botMappingWorkspaceRoot(
  mapping: BotConnectionView["sessionMappings"][number] | null | undefined,
  connectionWorkspaceRoot: string,
): string {
  const workspaceRoot = (mapping?.workspaceRoot ?? "").trim() || connectionWorkspaceRoot.trim();
  return botMappingScope(mapping, connectionWorkspaceRoot) === "project" ? workspaceRoot : "";
}

function compactRemoteId(value: string): string {
  const trimmed = value.trim();
  if (trimmed.length <= 28) return trimmed;
  return `${trimmed.slice(0, 12)}…${trimmed.slice(-8)}`;
}

function botMappingIdentityLabel(mapping: BotConnectionView["sessionMappings"][number] | null | undefined): string {
  const chatType = (mapping?.chatType ?? "").trim();
  const userId = (mapping?.userId ?? "").trim();
  const threadId = (mapping?.threadId ?? "").trim();
  if (threadId) return compactRemoteId(threadId);
  if ((chatType === "group" || chatType === "guild") && userId) return compactRemoteId(userId);
  return "";
}

function sidebarImStatus(connection: BotConnectionView, botEnabled: boolean): SidebarImStatus {
  if (!botEnabled || !connection.enabled) return "disabled";
  if (connection.status === "connected") return "connected";
  if (connection.status === "pending") return "pending";
  if (connection.status === "error") return "error";
  return "disconnected";
}

function sidebarImStatusLabel(status: SidebarImStatus, translate: Translator): string {
  switch (status) {
    case "connected":
      return translate("sidebar.imConnected");
    case "disabled":
      return translate("sidebar.imDisabled");
    case "pending":
      return translate("sidebar.imPending");
    case "error":
      return translate("sidebar.imError");
    default:
      return translate("sidebar.imDisconnected");
  }
}

function uniqueTrimmedValues(values: string[]): string[] {
  return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)));
}

function sidebarImAllowlistUsers(bot: BotSettingsView, platform: SidebarImPlatform): string[] {
  if (platform === "qq") return uniqueTrimmedValues(asArray(bot.allowlist.qqUsers));
  if (platform === "weixin") return uniqueTrimmedValues(asArray(bot.allowlist.weixinUsers));
  return uniqueTrimmedValues(asArray(bot.allowlist.feishuUsers));
}

function sidebarImQQAdded(qq: BotSettingsView["qq"]): boolean {
  return Boolean(qq.enabled || qq.secretSet || qq.appId.trim());
}

function sidebarImQQStatus(bot: BotSettingsView, runtimeStatus: BotRuntimeStatusView | null | undefined): SidebarImStatus {
  const appId = bot.qq.appId.trim();
  if (!bot.enabled || !bot.qq.enabled) return "disabled";
  if (!appId || !bot.qq.secretSet) return "disconnected";
  if (typeof window !== "undefined" && !window.runtime) return "pending";
  if (!runtimeStatus) return "pending";
  const status = runtimeStatus.status.trim().toLowerCase();
  if (runtimeStatus.running && runtimeStatus.connections > 0 && status === "running") {
    return "connected";
  }
  if (status === "error" || status === "blocked" || status === "degraded") return "error";
  if (status === "stopped") return "disconnected";
  return "pending";
}

async function loadBotRuntimeStatus(): Promise<BotRuntimeStatusView | null> {
  if (typeof window !== "undefined" && !window.runtime) return null;
  try {
    return await app.BotRuntimeStatus();
  } catch (e) {
    console.warn("bot runtime status failed", e);
    return null;
  }
}

function sidebarImQQConnection(bot: BotSettingsView, translate: Translator, runtimeStatus?: BotRuntimeStatusView | null): SidebarImConnection | null {
  if (!sidebarImQQAdded(bot.qq)) return null;
  const remoteId = bot.qq.appId.trim();
  const status = sidebarImQQStatus(bot, runtimeStatus);
  const statusLabel = sidebarImStatusLabel(status, translate);
  const allowlistUsers = sidebarImAllowlistUsers(bot, "qq");
  const subtitleParts = [
    remoteId ? compactRemoteId(remoteId) : "QQ",
    statusLabel,
  ].filter(Boolean);
  return {
    id: "__qq_bot__",
    connectionId: "__qq_bot__",
    platform: "qq",
    title: "QQ Bot",
    platformLabel: "QQ",
    subtitle: subtitleParts.join(" · "),
    status,
    statusLabel,
    remoteId,
    sessionId: "",
    sessionSource: "",
    scope: "global",
    workspaceRoot: "",
    allowAll: bot.allowlist.allowAll,
    allowlistEnabled: bot.allowlist.enabled,
    allowlistUsers,
    allowlistMatched: remoteId ? allowlistUsers.includes(remoteId) : false,
  };
}

function sidebarImConnectionsFromBot(
  bot: BotSettingsView | null | undefined,
  translate: Translator,
  runtimeStatus?: BotRuntimeStatusView | null,
): SidebarImConnection[] {
  if (!bot) return [];
  const qqConnection = sidebarImQQConnection(bot, translate, runtimeStatus);
  const connectionItems: SidebarImConnection[] = [];
  for (const connection of asArray(bot.connections)) {
    if (!isSidebarImConnection(connection)) continue;
    const mappings = connection.sessionMappings.filter((mapping) => mapping.sessionId.trim() || mapping.remoteId.trim());
    const rowMappings = mappings.length > 0 ? mappings : [null];
    rowMappings.forEach((mapping, index) => {
      const platform = sidebarImPlatform(connection);
      const platformLabel = sidebarImPlatformLabel(platform, translate);
      const remoteId = mapping?.remoteId.trim() ?? "";
      const sessionId = mapping?.sessionId.trim() ?? "";
      const sessionSource = mapping?.sessionSource.trim() ?? "";
      const scope = botMappingScope(mapping, connection.workspaceRoot);
      const workspaceRoot = botMappingWorkspaceRoot(mapping, connection.workspaceRoot);
      const status = sidebarImStatus(connection, bot.enabled);
      const title = connection.label.trim() || platformLabel;
      const allowlistUsers = sidebarImAllowlistUsers(bot, platform);
      const identityLabel = botMappingIdentityLabel(mapping);
      const mappedUserId = mapping?.userId.trim() ?? "";
      const subtitleParts = [
        remoteId ? compactRemoteId(remoteId) : platformLabel,
        identityLabel,
        connection.model.trim() || "",
        sidebarImStatusLabel(status, translate),
      ].filter(Boolean);
      connectionItems.push({
        id: mapping ? `${connection.id}:mapping:${index}` : connection.id,
        connectionId: connection.id,
        platform,
        title,
        platformLabel,
        subtitle: subtitleParts.join(" · "),
        status,
        statusLabel: sidebarImStatusLabel(status, translate),
        remoteId,
        sessionId,
        sessionSource,
        scope,
        workspaceRoot,
        allowAll: bot.allowlist.allowAll,
        allowlistEnabled: bot.allowlist.enabled,
        allowlistUsers,
        allowlistMatched: remoteId
          ? allowlistUsers.includes(remoteId) || (mappedUserId ? allowlistUsers.includes(mappedUserId) : false)
          : false,
      });
    });
  }
  return qqConnection ? [qqConnection, ...connectionItems] : connectionItems;
}

function mappedSessionTarget(sessionId: string): { kind: "path" | "topic"; value: string } | null {
  const trimmed = sessionId.trim();
  if (!trimmed) return null;
  const lower = trimmed.toLowerCase();
  if (lower.startsWith("path:")) {
    const value = trimmed.slice(5).trim();
    return value ? { kind: "path", value } : null;
  }
  if (lower.startsWith("topic:")) {
    const value = trimmed.slice(6).trim();
    return value ? { kind: "topic", value } : null;
  }
  if (trimmed.endsWith(".jsonl") || trimmed.includes("/") || trimmed.includes("\\") || trimmed.startsWith("~")) {
    return { kind: "path", value: trimmed };
  }
  return { kind: "topic", value: trimmed };
}

function sidebarImSessionTarget(connection: SidebarImConnection): { kind: "path" | "topic"; value: string } | null {
  return mappedSessionTarget(connection.sessionId);
}

function isChannelSession(session: SessionMeta): boolean {
  return session.kind === "channel" || session.sessionSource === "auto";
}

function sidebarImTopicSourcesFromBot(bot: BotSettingsView | null | undefined, translate: Translator): Record<string, SidebarImTopicSource> {
  if (!bot?.connections?.length) return {};
  const sources: Record<string, SidebarImTopicSource> = {};
  for (const connection of bot.connections) {
    if (!isSidebarImConnection(connection)) continue;
    const platform = sidebarImPlatform(connection);
    const label = sidebarImPlatformLabel(platform, translate);
    const title = connection.label.trim() || label;
    for (const mapping of asArray(connection.sessionMappings)) {
      const scope = botMappingScope(mapping, connection.workspaceRoot);
      if (scope !== "global") continue;
      const target = mappedSessionTarget(mapping.sessionId);
      if (!target || target.kind !== "topic") continue;
      if (sources[target.value]) continue;
      sources[target.value] = {
        platform,
        label,
        title,
        remoteId: mapping.remoteId.trim(),
        connectionId: connection.id,
      };
    }
  }
  return sources;
}

function sidebarImScopeLabel(connection: SidebarImConnection, translate: Translator): string {
  if (connection.scope === "project") return translate("botDetail.scopeProject", { name: connection.workspaceRoot || "Project" });
  return translate("botDetail.scopeGlobal");
}

function sidebarImSessionLabel(connection: SidebarImConnection, translate: Translator): string {
  const target = sidebarImSessionTarget(connection);
  if (!target) {
    return connection.remoteId ? translate("botDetail.readOnlyChannel") : translate("botDetail.noSession");
  }
  if (connection.sessionSource === "auto") return translate("botDetail.readOnlyChannel");
  if (target.kind === "path") return target.value.split(/[\\/]/).pop() || target.value;
  return target.value;
}

function sidebarImAccessModeLabel(connection: SidebarImConnection, translate: Translator): string {
  if (connection.allowAll) return translate("botDetail.accessAllowAll");
  if (connection.allowlistEnabled) return translate("botDetail.accessWhitelist");
  return translate("botDetail.accessDisabled");
}

function sidebarImAccessStatusLabel(connection: SidebarImConnection, translate: Translator): string {
  if (connection.allowAll) return translate("botDetail.accessOpen");
  if (!connection.remoteId) return translate("botDetail.accessUnknown");
  return connection.allowlistMatched ? translate("botDetail.accessMatched") : translate("botDetail.accessMissing");
}

function sidebarImAccessStatusClass(connection: SidebarImConnection): string {
  if (connection.allowAll || connection.allowlistMatched) return "ok";
  if (!connection.remoteId) return "muted";
  return "warn";
}

function SidebarImConnectionDetail({ connection, onClose, onOpenSession, onOpenSettings, onManageAllowlist }: SidebarImConnectionDetailProps) {
  const translate = useT();
  const target = sidebarImSessionTarget(connection);
  const accessStatusClass = sidebarImAccessStatusClass(connection);
  return (
    <div className="bot-detail">
      <section className="bot-detail__summary">
        <div className={`bot-detail__avatar bot-detail__avatar--${connection.platform}`} aria-hidden="true">
          {connection.platform === "qq" ? "Q" : connection.platform === "weixin" ? "微" : connection.platform === "lark" ? "L" : "飞"}
        </div>
        <div className="bot-detail__summary-main">
          <span>{translate("botDetail.subtitle")}</span>
          <h2>{connection.title}</h2>
          <div className="bot-detail__chips">
            <span>{connection.platformLabel}</span>
            <span>{connection.statusLabel}</span>
            <span>{sidebarImScopeLabel(connection, translate)}</span>
          </div>
        </div>
        <div className="bot-detail__summary-actions">
          <button type="button" className="btn btn--primary btn--small bot-detail__primary" disabled={!target} title={target ? undefined : translate("botDetail.openDisabled")} onClick={onOpenSession}>
            <MessageSquare size={14} />
            {translate("botDetail.openSession")}
          </button>
          <button type="button" className="btn btn--secondary btn--small" onClick={onOpenSettings}>
            <SettingsIcon size={14} />
            {translate("botDetail.manage")}
          </button>
          <button type="button" className="btn btn--secondary btn--small" onClick={onClose}>
            {translate("botDetail.close")}
          </button>
        </div>
      </section>

      <section className="bot-detail__panel bot-detail__panel--access" aria-label={translate("botDetail.access")}>
        <div className="bot-detail__section-head">
          <span>{translate("botDetail.access")}</span>
          <div className="bot-detail__section-actions">
            {connection.remoteId ? (
              <CopyButton text={connection.remoteId} label={translate("botDetail.copyRemoteId")} />
            ) : null}
            <button type="button" className="btn btn--secondary btn--small" onClick={onManageAllowlist}>
              {translate("botDetail.manageAllowlist")}
            </button>
          </div>
        </div>
        <div className="bot-detail__access-grid">
          <div>
            <span>{translate("botDetail.accessMode")}</span>
            <strong>{sidebarImAccessModeLabel(connection, translate)}</strong>
          </div>
          <div>
            <span>{translate("botDetail.accessCurrentUser")}</span>
            <code title={connection.remoteId || undefined}>{connection.remoteId || "—"}</code>
          </div>
          <div>
            <span>{translate("botDetail.accessStatus")}</span>
            <strong className={`bot-detail__access-status bot-detail__access-status--${accessStatusClass}`}>
              {sidebarImAccessStatusLabel(connection, translate)}
            </strong>
          </div>
        </div>
        <div className="bot-detail__allowlist">
          <span>{translate("botDetail.channelAllowlistUsers")}</span>
          <div className="bot-detail__id-list">
            {connection.allowlistUsers.length > 0 ? (
              connection.allowlistUsers.map((id) => (
                <code
                  key={id}
                  className={id === connection.remoteId ? "bot-detail__id-list-item--active" : ""}
                  title={id}
                >
                  {id}
                </code>
              ))
            ) : (
              <em>{translate("botDetail.emptyAllowlistUsers")}</em>
            )}
          </div>
        </div>
      </section>

      <section className="bot-detail__panel bot-detail__panel--facts" aria-label={translate("botDetail.summary")}>
        <div className="bot-detail__section-head">
          <span>{translate("botDetail.summary")}</span>
        </div>
        <div className="bot-detail__facts">
          <div>
            <span>{translate("botDetail.remoteId")}</span>
            <code>{connection.remoteId || "—"}</code>
          </div>
          <div>
            <span>{translate("botDetail.localTopic")}</span>
            <strong>{sidebarImSessionLabel(connection, translate)}</strong>
          </div>
          <div>
            <span>{translate("botDetail.scope")}</span>
            <strong>{sidebarImScopeLabel(connection, translate)}</strong>
          </div>
        </div>
      </section>
    </div>
  );
}

function tabWorkspaceTitle(tab?: TabMeta): string {
  if (!tab) return "Global";
  if (tab.scope === "project") return tab.workspaceName || tab.workspaceRoot || "Project";
  if (tab.scope === "global") return tab.workspaceName || "Global";
  return tab.workspaceName || tab.workspaceRoot || "Global";
}

function topicTitle(tab?: TabMeta): string {
  if (!tab) return "Global";
  const workspaceTitle = tabWorkspaceTitle(tab);
  const topic = tab.topicTitle || (tab.scope === "global" ? workspaceTitle : "Untitled");
  return topic === workspaceTitle ? workspaceTitle : `${workspaceTitle} / ${topic}`;
}

function topicDisplayTitle(tab?: TabMeta): string {
  if (!tab) return "全局";
  return tab.topicTitle || (tab.scope === "global" ? tabWorkspaceTitle(tab) : "未命名");
}

function isMissingSessionError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err ?? "");
  return /no such file|cannot find the file|file does not exist|session is pending cleanup|session .*not found/i.test(message);
}

function workspaceDisplayName(path?: string): string {
  if (!path) return "";
  const parts = path.split(/[/\\]/).filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

function materializeLiveItems(items: Item[], live?: LiveStream): Item[] {
  if (!live) return items;
  return items.map((item) => {
    if (item.kind !== "assistant" || item.id !== live.id) return item;
    return { ...item, text: live.text, reasoning: live.reasoning, streaming: true };
  });
}

function fence(label: string, value: string): string {
  if (!value.trim()) return "";
  const fenceToken = value.includes("```") ? "````" : "```";
  return `${label}\n${fenceToken}\n${value.trim()}\n${fenceToken}`;
}

function sessionItemsToMarkdown(title: string, items: Item[], live?: LiveStream): string {
  const lines: string[] = [`# ${title.trim() || `${BRAND_DISPLAY_NAME} session`}`, ""];
  for (const item of materializeLiveItems(items, live)) {
    switch (item.kind) {
      case "user":
        lines.push("## User", "", item.text.trim(), "");
        break;
      case "assistant":
        lines.push("## Assistant");
        if (item.reasoning.trim()) {
          lines.push("", "### Reasoning", "", item.reasoning.trim());
        }
        if (item.text.trim()) {
          lines.push("", item.text.trim());
        }
        lines.push("");
        break;
      case "tool":
        lines.push(`### Tool: ${item.name}`);
        if (item.args.trim()) lines.push("", fence("Args", item.args));
        if (item.output?.trim()) lines.push("", fence("Output", item.output));
        if (item.error?.trim()) lines.push("", fence("Error", item.error));
        lines.push("");
        break;
      case "phase":
        lines.push(`### Phase`, "", item.text.trim(), "");
        break;
      case "notice":
        lines.push(`### ${item.level === "warn" ? "Warning" : "Notice"}`, "", item.text.trim(), "");
        break;
      case "compaction":
        lines.push("### Context Compaction", "");
        if (item.pending) {
          lines.push("Compaction pending.");
        } else {
          lines.push(`Messages: ${item.messages}`);
          if (item.trigger) lines.push(`Trigger: ${item.trigger}`);
          if (item.summary.trim()) lines.push("", item.summary.trim());
        }
        lines.push("");
        break;
    }
  }
  return lines.join("\n").replace(/\n{3,}/g, "\n\n").trimEnd() + "\n";
}

function sessionItemsToJson(title: string, items: Item[], live?: LiveStream): string {
  const exportItems = materializeLiveItems(items, live).map((item) => {
    const legacyItem = item as Item & { forceReveal?: unknown };
    if (!Object.prototype.hasOwnProperty.call(legacyItem, "forceReveal")) return item;
    const exportItem = { ...legacyItem };
    delete exportItem.forceReveal;
    return exportItem;
  });
  return JSON.stringify(
    {
      title,
      exportedAt: new Date().toISOString(),
      items: exportItems,
    },
    null,
    2,
  );
}

function safeFilename(name: string): string {
  const cleaned = name.trim().replace(/[\\/:*?"<>|]+/g, "-").replace(/\s+/g, " ").slice(0, 80);
  return cleaned || "云智企办-会话";
}

/** Global hotkey handler for shell-expand toggle (Ctrl/Cmd+B). */
function ShellHotkeys() {
  const shellExpand = useShellExpand();
  useGlobalShortcut("shell.toggle", () => shellExpand?.toggleLast(), [shellExpand], Boolean(shellExpand));
  return null;
}

/** Global hotkey handler for text-size shortcuts (Ctrl/Cmd + Plus/Minus/0). */
function TextSizeHotkeys() {
  useGlobalShortcut("textSize.increase", () => applyTextSize(nextTextSize(getTextSize(), 1)));
  useGlobalShortcut("textSize.decrease", () => applyTextSize(nextTextSize(getTextSize(), -1)));
  useGlobalShortcut("textSize.reset", () => applyTextSize(DEFAULT_TEXT_SIZE));
  return null;
}

export default function App() {
  const {
    state,
    activeTabId,
    send,
    sendToTab,
    beforeSendRef,
    runShell,
    steer,
    notice,
    cancel,
    cancelForTab,
    approve,
    answerQuestion,
    setControllerMode,
    setCollaborationMode: setControllerCollaborationMode,
    setToolApprovalMode: setControllerToolApprovalMode,
    syncToolApprovalMode: syncControllerToolApprovalMode,
    setGoal: setControllerGoal,
    clearGoal: clearControllerGoal,
    clearSession,
    listSessions,
    resumeSession,
    openChannelSession,
    revalidateChannelSession,
    previewSession,
    deleteSession,
    restoreSession,
    purgeTrashedSession,
    renameSession,
    loadOlderHistory,
    refreshMeta,
    pickWorkspace,
    switchWorkspace,
    rewind,
    setModel,
    retryTabStartup,
    retryHistoryLoad,
    setEffort,
    setTokenMode,
    switchTab,
    openProjectTab,
    openGlobalTab,
    closeTab,
    reorderTabs,
    openTopicSession,
    beginSurfaceTransition,
    cancelSurfaceTransition,
    setSessionViewCacheScope,
    activateTopic,
    surfaceTransitionActive,
    surfaceLoadingHintVisible,
    renderedViewTarget,
    renderedViewTitle,
    sessionViewKey,
    sessionViewScroll,
    updateSessionViewScroll,
    syncActiveTab,
    ensureBlankTab,
    ensureBlankSurface,
  } = useController();
  const { locale, setPref: setLocalePref } = useI18n();
  const t = useT();
  useEffect(() => {
    const syncTimezone = () => {
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      if (timezone && typeof app.SetClientTimezone === "function") {
        void app.SetClientTimezone(timezone).catch(() => {});
      }
    };
    const syncVisibleTimezone = () => {
      if (document.visibilityState === "visible") syncTimezone();
    };
    syncTimezone();
    window.addEventListener("focus", syncTimezone);
    document.addEventListener("visibilitychange", syncVisibleTimezone);
    return () => {
      window.removeEventListener("focus", syncTimezone);
      document.removeEventListener("visibilitychange", syncVisibleTimezone);
    };
  }, []);
  const [composerProfilesByTab, setComposerProfilesByTab] = useState<Record<string, ComposerProfile>>({});
  const yoloRestoreToolApprovalModesRef = useRef<Record<string, RestorableToolApprovalMode>>({});
  const userPlanModeByTabRef = useRef<UserPlanModeIntents>({});
  const [tabMetas, setTabMetas] = useState<TabMeta[]>([]);
  const tabMetasRef = useRef(tabMetas);
  tabMetasRef.current = tabMetas;
  const tabSwitchRequestSeqRef = useRef(0);
  const blankSessionRequestSeqRef = useRef(0);
  const [pendingBlankSessionRequest, setPendingBlankSessionRequest] = useState<{
    seq: number;
    scope: string;
    workspaceRoot: string;
    previewScene: Scene;
    previewSceneState: SceneState;
  } | null>(null);
  const surfaceNavigationSeqRef = useRef(0);
  const preparedTopicNavigationRef = useRef<{ token: number; transitionId?: number; navigationSeq: number } | null>(null);
  const openTopicSeqRef = useRef(0);
  const openTopicRunningRef = useRef(false);
  const openTopicPendingRef = useRef<PendingOpenTopicRequest | null>(null);
  const activeTab = useMemo(
    () => tabMetas.find((tab) => tab.id === activeTabId) ?? tabMetas.find((tab) => tab.active),
    [activeTabId, tabMetas],
  );
  const activeTabRef = useRef(activeTab);
  activeTabRef.current = activeTab;
  const conversationStateKey = useMemo(
    () => composerDraftKeyForTab(activeTab, activeTabId) || "__draft__",
    [activeTab, activeTabId],
  );
  const [draftScenesByTab, setDraftScenesByTab] = useState<Record<string, Scene>>({});
  const [tabOrderIds, setTabOrderIds] = useState<string[]>([]);
  const [tabRevealSignal, setTabRevealSignal] = useState(0);
  const [transcriptRevealSignal, setTranscriptRevealSignal] = useState(0);
  const startupSplashVisible = useOverlayStore((s) => s.startupSplashVisible);
  const setStartupSplashVisible = useOverlayStore((s) => s.setStartupSplashVisible);
  const settingsTarget = useOverlayStore((s) => s.settingsTarget);
  const setSettingsTarget = useOverlayStore((s) => s.setSettingsTarget);
  const settingsFocus = useOverlayStore((s) => s.settingsFocus);
  const setSettingsFocus = useOverlayStore((s) => s.setSettingsFocus);
  const {
    desktopLayoutStyle,
    singleSurfaceLayout,
    startupUpdateChecksEnabled,
    desktopPlatform,
    statusBarStyle,
    statusBarItems,
    applyDesktopPreferences,
    enableStartupUpdateChecks,
  } = useDesktopPreferences(setLocalePref);
  const sidebarCreation = desktopLayoutStyle === "creation";
  const [sidebarAppVersion, setSidebarAppVersion] = useState("");
  useEffect(() => {
    if (!sidebarCreation) {
      setSidebarAppVersion("");
      return;
    }

    let cancelled = false;
    void app.Version()
      .then((version) => {
        if (!cancelled) setSidebarAppVersion(formatBrandVersion(version));
      })
      .catch(() => {
        if (!cancelled) setSidebarAppVersion("");
      });
    return () => {
      cancelled = true;
    };
  }, [sidebarCreation]);
  const [histView, setHistView] = useState<HistoryViewState | null>(null);
  const histViewOpenRef = useRef(false);
  histViewOpenRef.current = histView !== null;
  const paletteOpen = useOverlayStore((s) => s.paletteOpen);
  const setPaletteOpen = useOverlayStore((s) => s.setPaletteOpen);
  const shortcutsOpen = useOverlayStore((s) => s.shortcutsOpen);
  const setShortcutsOpen = useOverlayStore((s) => s.setShortcutsOpen);
  const paletteSessions = useOverlayStore((s) => s.paletteSessions);
  const setPaletteSessions = useOverlayStore((s) => s.setPaletteSessions);
  const { showToast, showError } = useToast();
  const [sidebarImConnections, setSidebarImConnections] = useState<SidebarImConnection[]>([]);
  const [imTopicSources, setImTopicSources] = useState<Record<string, SidebarImTopicSource>>({});
  const [sidebarImDetailConnectionId, setSidebarImDetailConnectionId] = useState("");
  const {
    menuOpen: creationAccountMenuOpen,
    menuRef: creationAccountMenuRef,
    avatarInputRef: accountAvatarInputRef,
    avatarSrc: sidebarAccountAvatarSrc,
    initialized: sidebarAccountInitialized,
    account: sidebarAccount,
    scopeKey: sidebarAccountScopeKey,
    available: sidebarAccountAvailable,
    balance: sidebarAccountBalance,
    displayBalance: sidebarAccountDisplayBalance,
    plan: sidebarAccountPlan,
    status: sidebarAccountSubscriptionStatus,
    subscriptionVersion: sidebarAccountSubscriptionVersion,
    closeMenu: closeCreationAccountMenu,
    toggleMenu: toggleCreationAccountMenu,
    handleAvatarChange: handleAccountAvatarChange,
  } = useAccountSidebar(DESKTOP_ACCOUNT_CONTEXT_KEY);
  const projectAccountScopeKey = hasRecoverableCloudIdentity(sidebarAccount) ? sidebarAccountScopeKey : "";
  const conversationAttentionScope = useMemo(
    () => conversationAttentionAccountScope(sidebarAccount?.profile_id, sidebarAccount?.cloud_user_id),
    [sidebarAccount?.cloud_user_id, sidebarAccount?.profile_id],
  );
  const conversationAttentionScopeRef = useRef(conversationAttentionScope);
  conversationAttentionScopeRef.current = conversationAttentionScope;
  const conversationRunAccountScopesRef = useRef(new Map<string, string>());
  const [conversationUnreadState, setConversationUnreadState] = useState<{
    scope: string;
    records: ConversationUnreadCompletions;
  }>(() => ({
    scope: conversationAttentionScope,
    records: readConversationUnreadCompletions(conversationAttentionScope),
  }));
  useEffect(() => {
    setConversationUnreadState({
      scope: conversationAttentionScope,
      records: readConversationUnreadCompletions(conversationAttentionScope),
    });
  }, [conversationAttentionScope]);
  const updateConversationUnreadCompletion = useCallback((
    accountScope: string,
    identity: string,
    completedAt: number | null,
  ) => {
    if (!accountScope || !identity) return;
    setConversationUnreadState((currentState) => {
      const currentRecords = currentState.scope === accountScope
        ? currentState.records
        : readConversationUnreadCompletions(accountScope);
      const nextRecords = completedAt === null
        ? withoutUnreadCompletion(currentRecords, identity)
        : withUnreadCompletion(currentRecords, identity, completedAt);
      if (nextRecords !== currentRecords) writeConversationUnreadCompletions(accountScope, nextRecords);
      if (conversationAttentionScopeRef.current !== accountScope) return currentState;
      return nextRecords === currentState.records
        ? currentState
        : { scope: accountScope, records: nextRecords };
    });
  }, []);
  const conversationUnreadCompletionIds = useMemo(
    () => new Set(
      conversationUnreadState.scope === conversationAttentionScope
        ? Object.keys(conversationUnreadState.records)
        : [],
    ),
    [conversationAttentionScope, conversationUnreadState],
  );
  useEffect(() => {
    const previousScope = projectComposerVisibleScopeRef.current;
    projectComposerVisibleScopeRef.current = projectAccountScopeKey;
    const scopeChanged = previousScope !== projectAccountScopeKey;
    const { removedKeys, closeTabIds } = planProjectComposerScopeChange(projectComposerAdaptersRef.current, projectAccountScopeKey);
    for (const tabId of closeTabIds) void closeTab(tabId).catch(() => {});
    if (removedKeys.length === 0 && !scopeChanged) return;
    for (const key of removedKeys) delete projectComposerDraftStore.current[key];
    setProjectComposerInsertRequests((current) => {
      const next = { ...current };
      for (const key of removedKeys) delete next[key];
      return next;
    });
    setSelectedCloudProjectId("");
    setProjectCapabilityOrigin(null);
    setRestoreProjectCapabilityFocus(null);
    setWorkspaceView((current) => current === "projects" ? "chat" : current);
    setProjectRevision((revision) => revision + 1);
    setProjectComposerRevision((revision) => revision + 1);
  }, [closeTab, projectAccountScopeKey]);
  useEffect(() => {
    setSessionViewCacheScope({
      actorId: hasRecoverableCloudIdentity(sidebarAccount) ? sidebarAccount?.cloud_user_id : undefined,
      profileId: sidebarAccount?.profile_id,
      bindingGeneration: sidebarAccount?.binding_generation,
    });
  }, [
    setSessionViewCacheScope,
    sidebarAccount?.cloud_user_id,
    sidebarAccount?.profile_id,
    sidebarAccount?.binding_generation,
    sidebarAccount?.status,
  ]);
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed);
  const setSidebarCollapsed = useLayoutStore((s) => s.setSidebarCollapsed);
  const heartbeatOpen = useOverlayStore((s) => s.heartbeatOpen);
  const setHeartbeatOpen = useOverlayStore((s) => s.setHeartbeatOpen);
  type TimeFilter = "all" | "10" | "20" | "1h" | "3h" | "5h" | "1d";
  const [topicTimeFilter, setTopicTimeFilter] = useState<TimeFilter>(() => {
    try {
      const saved = localStorage.getItem("projectTree:timeFilter");
      if (saved === "all" || saved === "10" || saved === "20" || saved === "1h" || saved === "3h" || saved === "5h" || saved === "1d") return saved;
    } catch { /* localStorage unavailable */ }
    return "all";
  });
  useEffect(() => {
    try { localStorage.setItem("projectTree:timeFilter", topicTimeFilter); } catch { /* ignore */ }
  }, [topicTimeFilter]);
  const sidebarWidth = useLayoutStore((s) => s.sidebarWidth);
  const setSidebarWidth = useLayoutStore((s) => s.setSidebarWidth);
  const [sidebarResizing, setSidebarResizing] = useState(false);
  const [liveSidebarWidth, setLiveSidebarWidth] = useState<number | null>(null);
  const [viewportWidth, setViewportWidth] = useState(() => (typeof window === "undefined" ? 1440 : window.innerWidth));
  const rightDockTreeWidth = useLayoutStore((s) => s.rightDockTreeWidth);
  const setRightDockTreeWidth = useLayoutStore((s) => s.setRightDockTreeWidth);
  const rightDockPreviewWidth = useLayoutStore((s) => s.rightDockPreviewWidth);
  const setRightDockPreviewWidth = useLayoutStore((s) => s.setRightDockPreviewWidth);
  const workspacePanelState = useLayoutStore(
    (s) => s.workspacePanelsByConversation[conversationStateKey] ?? DEFAULT_WORKSPACE_PANEL_STATE,
  );
  const setWorkspacePanelStateForConversation = useLayoutStore((s) => s.setWorkspacePanelState);
  const updateWorkspacePanelState = useCallback(
    (update: SetStateAction<WorkspacePanelState>) => {
      setWorkspacePanelStateForConversation(conversationStateKey, update);
    },
    [conversationStateKey, setWorkspacePanelStateForConversation],
  );
  const workspacePanelOpen = workspacePanelState.open;
  const workspacePanelMaximized = workspacePanelState.maximized;
  const workspacePreviewActive = workspacePanelState.previewActive;
  const rightDockMode = workspacePanelState.mode === "changed" ? "files" : workspacePanelState.mode;
  // Refresh workspace views when a turn, notice, or potentially mutating tool
  // result can have changed generated artifacts.
  useEffect(() => {
    const unsub = onEvent((e) => {
      if (shouldRefreshWorkspaceForEvent(e)) {
        setDockRefreshKey((v) => v + 1);
      }
      if (e.kind === "turn_done" && !e.err) playSuccessChime();
    });
    return unsub;
  }, []);

  const [workspacePanelResizing, setWorkspacePanelResizing] = useState(false);
  const [liveWorkspacePanelRenderWidth, setLiveWorkspacePanelRenderWidth] = useState<number | null>(null);
  const [dockRefreshKey, setDockRefreshKey] = useState(0);
  const [projectRevision, setProjectRevision] = useState(0);
  const projectTreeRuntimeKey = useMemo(() => projectTreeRuntimeKeyForTabs(tabMetas), [tabMetas]);
  const projectTreeRuntimeKeyRef = useRef<string | null>(null);
  const hasProjectTreeRuntimeBaselineRef = useRef(false);
  useEffect(() => {
    if (!hasProjectTreeRuntimeBaselineRef.current) {
      if (tabMetas.length === 0) return;
      hasProjectTreeRuntimeBaselineRef.current = true;
      projectTreeRuntimeKeyRef.current = projectTreeRuntimeKey;
      return;
    }
    if (projectTreeRuntimeKeyRef.current === projectTreeRuntimeKey) return;
    projectTreeRuntimeKeyRef.current = projectTreeRuntimeKey;
    setProjectRevision((value) => value + 1);
  }, [projectTreeRuntimeKey, tabMetas.length]);
  const lastSeenProjectTreeRevisionRef = useRef(0);
  const refreshInFlightRef = useRef(false);
  const refreshQueuedRef = useRef(false);
  const [activeTopicTurnEvidence, setActiveTopicTurnEvidence] = useState<TopicTurnEvidence>({
    key: "",
    state: "unavailable",
  });
  const [subscriptionRepairFacts, setSubscriptionRepairFacts] = useState<SubscriptionRepairFacts>({
    key: "",
    state: "loading",
  });
  const [subscriptionRepairFactsRevision, setSubscriptionRepairFactsRevision] = useState(0);
  const [automationModelSettingsRevision, setAutomationModelSettingsRevision] = useState(0);
  const [modelSwitcherTransition, setModelSwitcherTransition] = useState<ModelSwitcherTransition | null>(null);
  const refreshSubscriptionRepairFacts = useCallback(() => {
    setSubscriptionRepairFactsRevision((revision) => revision + 1);
  }, []);
  const refreshModelCatalog = useCallback(() => {
    refreshSubscriptionRepairFacts();
  }, [refreshSubscriptionRepairFacts]);
  const marketplaceRefreshScopeStatesRef = useRef(new Map<string, SubscriptionMarketplaceRefreshScopeState>());
  const ensureModelMarketplaceRefresh = useCallback((accountScopeKey: string, force = false) => {
    void refreshSubscriptionMarketplaceOnce({
      accountScopeStates: marketplaceRefreshScopeStatesRef.current,
      accountScopeKey,
      refreshMarketplace: () => app.RefreshModelMarketplace(),
      readMarketplace: () => app.ModelMarketplace(),
      refreshModelCatalog,
      force,
    }).catch(() => {});
  }, [refreshModelCatalog]);
  const refreshAccountAfterAuthentication = useCallback(async (accountScopeKey: string) => {
    ensureModelMarketplaceRefresh(accountScopeKey, true);
    await refreshMeta();
  }, [ensureModelMarketplaceRefresh, refreshMeta]);
  const subscriptionRepairRequestRef = useRef(0);
  const modelEntitlementSnapshotRef = useRef<{
    accountKey: string;
    routes: ModelInfo[];
    marketplace?: ModelMarketplaceView;
  } | null>(null);
  const subscriptionRepairAttemptedKeysRef = useRef(new Set<string>());
  const subscriptionRepairContextRef = useRef("");
  const subscriptionRepairScopeRef = useRef("");
  const [subscriptionRepairRunState, setSubscriptionRepairRunState] = useState<SubscriptionDefaultRepairRunState | null>(null);
  const [composerInsertRequest, setComposerInsertRequest] = useState<ComposerInsertRequest | null>(null);
  const projectComposerDraftStore = useRef<ComposerDraftStore["current"]>({});
  const projectComposerAdaptersRef = useRef(new Map<string, ProjectComposerAdapter>());
  const projectComposerVisibleScopeRef = useRef("");
  const [projectComposerRevision, setProjectComposerRevision] = useState(0);
  const [selectedCloudProjectId, setSelectedCloudProjectId] = useState("");
  const [projectCapabilityOrigin, setProjectCapabilityOrigin] = useState<{
    sessionKey: string;
    projectId: string;
    kind: ProjectCapabilityKind;
    onCapabilitiesChanged?: (items: CloudProjectCapabilityBinding[]) => void;
    onCapabilityCatalogChanged?: (kind: ProjectCapabilityKind, items: ProjectCapabilityCatalogItem[]) => void;
  } | null>(null);
  const [restoreProjectCapabilityFocus, setRestoreProjectCapabilityFocus] = useState<ProjectCapabilityKind | null>(null);
  const [projectComposerInsertRequests, setProjectComposerInsertRequests] = useState<Record<string, ComposerInsertRequest>>({});
  const handleComposerInsertRequestConsumed = useCallback((id: number) => {
    setComposerInsertRequest((current) => current?.id === id ? null : current);
  }, []);
  const [workspaceView, setWorkspaceView] = useState<WorkspaceView>("chat");
  const workspaceViewRef = useRef(workspaceView);
  workspaceViewRef.current = workspaceView;
  const lastViewedConversationRef = useRef({ scope: "", identity: "", visible: false });
  useEffect(() => {
    const identity = conversationAttentionIdentity(activeTab);
    const previous = lastViewedConversationRef.current;
    const conversationVisible = workspaceView === "chat" && histView === null;
    const enteredConversation = Boolean(identity && identity !== previous.identity);
    const returnedToVisibleConversation = Boolean(identity && conversationVisible && !previous.visible && previous.identity === identity && previous.scope === conversationAttentionScope);
    const initializedScopeForVisibleConversation = Boolean(identity && conversationAttentionScope && !previous.scope);
    if (conversationVisible && (enteredConversation || returnedToVisibleConversation || initializedScopeForVisibleConversation)) {
      updateConversationUnreadCompletion(conversationAttentionScope, identity, null);
    }
    lastViewedConversationRef.current = { scope: conversationAttentionScope, identity, visible: conversationVisible };
  }, [
    activeTab?.scope,
    activeTab?.workspaceRoot,
    activeTab?.topicId,
    activeTab?.sessionPath,
    conversationAttentionScope,
    histView,
    updateConversationUnreadCompletion,
    workspaceView,
  ]);
  const [superpowerEntry, setSuperpowerEntry] = useState<{ screen: "catalog" | "ecommerce"; key: number }>({ screen: "catalog", key: 0 });
  const [superpowersEnabled, setSuperpowersEnabledState] = useState(getSuperpowersEnabled);
  const [updateCenterOpen, setUpdateCenterOpen] = useState(false);
  const [enabledConnectorsByScope, setEnabledConnectorsByScope] = useState(loadEnabledConnectorSelections);
  const [recentComposerExperts, setRecentComposerExperts] = useState<ExpertSelection[]>([]);
  const expertRefreshCanClearSelection = Boolean(activeTabId && state.meta?.ready === true && !state.backendActivationPending && !state.hydrating);
  const composerExpertViewKey = sessionViewKey || (activeTabId ? `tab:${activeTabId}` : "");
  const {
    expertCatalog,
    userExpertCatalog,
    expertLibraryState,
    activeExpert,
    activeExpertResolution,
    setActiveExpert,
    publishExpertSelection,
    pendingExpertActivationRef,
    expertActivationSequenceRef,
    activatableExpertIds,
    refreshExpertState,
    saveUserExpert,
    saveCatalogExpert,
    duplicateExpert,
    toggleExpertFavorite,
    importExpertConfig,
    deleteUserExpert,
    deleteCatalogExpert,
  } = useExpertCatalog({
    activeTabId: activeTabId ?? null,
    viewKey: composerExpertViewKey,
    canClearSelection: expertRefreshCanClearSelection,
    setRecentExperts: setRecentComposerExperts,
  });
  const projectCapabilityExperts = useMemo(() => [...expertCatalog, ...userExpertCatalog].map((expert: any) => ({
    id: String(expert.id ?? ""),
    name: String(expert.name ?? expert.id ?? "未命名专家"),
    description: String(expert.description ?? expert.summary ?? ""),
    meta: expert.source === "user" ? "我的专家" : "专家库",
    mode: expert.mode === "team" ? "team" as const : "expert" as const,
    memberCount: Array.isArray(expert.members) ? expert.members.length : 0,
    iconUrl: expertAvatarSrcForInfo(expert),
  })).filter((expert) => expert.id), [expertCatalog, userExpertCatalog]);
  const renderedComposerIdentity = renderedViewTarget?.composerIdentity;
  const renderedTargetActiveExpert = useMemo<ExpertSelection | null>(() => {
    if (!renderedComposerIdentity?.expertKnown || !renderedComposerIdentity.expertId) return null;
    const catalogSelection = expertSelectionFromState({
      selectedId: renderedComposerIdentity.expertId,
      selectedName: renderedComposerIdentity.expertName,
      source: "session",
      canSwitch: false,
      requiresNewSession: false,
    }, expertCatalog, userExpertCatalog);
    if (catalogSelection) return catalogSelection;
    const recentSelection = recentComposerExperts.find((expert) => expert.id === renderedComposerIdentity.expertId);
    if (recentSelection) {
      return {
        ...recentSelection,
        name: renderedComposerIdentity.expertName || recentSelection.name,
        systemPrompt: undefined,
      };
    }
    return {
      id: renderedComposerIdentity.expertId,
      name: renderedComposerIdentity.expertName || renderedComposerIdentity.expertId,
      role: "expert",
      mode: "expert",
      prompt: "",
      systemPrompt: undefined,
    };
  }, [expertCatalog, recentComposerExperts, renderedComposerIdentity, userExpertCatalog]);
  const composerExpertUsesTargetIdentity = composerExpertShouldUseTargetIdentity({
    hasRenderedTarget: Boolean(renderedViewTarget),
    surfaceTransitionActive,
    optimisticExpertActivation: Boolean(
      activeExpert?.id
      && pendingExpertActivationRef.current?.expertId === activeExpert.id
    ),
    renderedViewKey: composerExpertViewKey,
    activeTabId,
    resolution: activeExpertResolution,
  });
  const displayedActiveExpert = composerExpertUsesTargetIdentity
    ? renderedTargetActiveExpert
    : activeExpert;
  useLayoutEffect(() => {
    const traceId = currentComposerExpertTraceId();
    if (!traceId) return;
    const data = {
      activeTabId: activeTabId ?? "",
      surfaceTransitionActive,
      displayedSource: composerExpertUsesTargetIdentity ? "target-identity" : "active-expert",
      displayedExpertId: displayedActiveExpert?.id ?? "",
      displayedExpertName: displayedActiveExpert?.name ?? "",
      transitionExpertKnown: renderedComposerIdentity?.expertKnown ?? false,
      transitionExpertId: renderedComposerIdentity?.expertId ?? "",
      transitionExpertName: renderedComposerIdentity?.expertName ?? "",
      activeExpertId: activeExpert?.id ?? "",
      activeExpertName: activeExpert?.name ?? "",
      expertResolutionViewKey: activeExpertResolution?.viewKey ?? "",
      expertResolutionTabId: activeExpertResolution?.tabId ?? "",
      expertResolutionSequence: activeExpertResolution?.refreshSequence ?? 0,
      expertResolutionAuthoritative: activeExpertResolution?.authoritative ?? false,
    };
    recordComposerExpertTrace("app.commit", data, traceId);
    recordComposerExpertPaint(data, traceId);
  }, [
    activeExpert?.id,
    activeExpert?.name,
    activeExpertResolution?.authoritative,
    activeExpertResolution?.refreshSequence,
    activeExpertResolution?.tabId,
    activeExpertResolution?.viewKey,
    activeTabId,
    composerExpertUsesTargetIdentity,
    displayedActiveExpert?.id,
    displayedActiveExpert?.name,
    surfaceTransitionActive,
    renderedComposerIdentity?.expertId,
    renderedComposerIdentity?.expertKnown,
    renderedComposerIdentity?.expertName,
  ]);
  const [expertTeamSwitchConfirm, setExpertTeamSwitchConfirm] = useState<ExpertTeamSwitchConfirm | null>(null);
  const [expertMemberHistoryFocused, setExpertMemberHistoryFocused] = useState(false);
  const expertMemberHistoryReturnRef = useRef<() => void>(() => {});
  const handleExpertMemberHistoryFocusChange = useCallback((focused: boolean, returnToLead?: () => void) => {
    expertMemberHistoryReturnRef.current = focused && returnToLead ? returnToLead : () => {};
    setExpertMemberHistoryFocused(focused);
  }, []);
  const transientOverlayDismissSignal = useOverlayStore((s) => s.transientOverlayDismissSignal);
  const setTransientOverlayDismissSignal = useOverlayStore((s) => s.setTransientOverlayDismissSignal);
  const [renamingTopicId, setRenamingTopicId] = useState<string | null>(null);
  const [topicTitleDraft, setTopicTitleDraft] = useState("");
  const topicExportOpen = useOverlayStore((s) => s.topicExportOpen);
  const setTopicExportOpen = useOverlayStore((s) => s.setTopicExportOpen);
  const [sidebarTogglePressed, setSidebarTogglePressed] = useState(false);
  const [workspaceTogglePressed, setWorkspaceTogglePressed] = useState(false);
  const [clearContextPending, setClearContextPending] = useState(false);
  const topicRenameSkipCommitRef = useRef(false);
  const topicRenameCommitHandledRef = useRef(false);
  const expertDirectoryUsesCurrentTaskRef = useRef(false);
  const clearExpertErrorToastOpenRef = useRef(false);
  const expertMutationQueueRef = useRef<Promise<unknown>>(Promise.resolve());
  const expertMutationResultsRef = useRef(new Map<string, { sessionPath: string | undefined; result: Promise<boolean> }>());
  const [expertSelectionRevisions, setExpertSelectionRevisions] = useState<Record<string, number>>({});
  const invalidateExpertTeam = useCallback((tabId: string) => {
    setExpertSelectionRevisions((current) => ({ ...current, [tabId]: (current[tabId] ?? 0) + 1 }));
  }, []);
  const enqueueExpertMutation = useCallback(<T,>(operation: () => Promise<T>, targetTabId: string): Promise<T> => {
    const sessionPath = tabMetasRef.current.find((tab) => tab.id === targetTabId)?.sessionPath;
    const result = expertMutationQueueRef.current.catch(() => undefined).then(() => {
      if (tabMetasRef.current.find((tab) => tab.id === targetTabId)?.sessionPath !== sessionPath) {
        throw new Error("conversation changed while updating expert selection");
      }
      return operation();
    });
    expertMutationQueueRef.current = result.then(() => undefined, () => undefined);
    expertMutationResultsRef.current.set(targetTabId, { sessionPath, result: result.then((value) => value !== null, () => false) });
    return result;
  }, []);
  beforeSendRef.current = async (tabId) => {
    const sequence = expertActivationSequenceRef.current;
    const sessionPath = tabMetasRef.current.find((tab) => tab.id === tabId)?.sessionPath;
    if (expertMutationResultsRef.current.get(tabId)?.sessionPath !== sessionPath) expertMutationResultsRef.current.delete(tabId);
    const operation = expertMutationResultsRef.current.get(tabId);
    if (operation && !await operation.result) throw new Error("expert selection failed; retry the selection before sending");
    await expertMutationQueueRef.current;
    if (sequence !== expertActivationSequenceRef.current || operation !== expertMutationResultsRef.current.get(tabId)) {
      throw new Error("expert selection changed while preparing to send");
    }
  };
  const appRef = useRef<HTMLDivElement>(null);
  const layoutRef = useRef<HTMLDivElement>(null);
  const mainWorkspaceRef = useRef<HTMLElement>(null);
  const sidebarTogglePressTimerRef = useRef<number | null>(null);
  const workspaceTogglePressTimerRef = useRef<number | null>(null);
  const sidebarAutoCollapsedRef = useRef(false);
  const sidebarManualExpandedRef = useRef(false);
  const creationSidebarRestoreCollapsedRef = useRef<boolean | null>(null);

  // Persist window geometry across launches.
  useWindowStatePersistence();
  useViewportHeightVar();
  useEffect(() => {
    mainWorkspaceRef.current?.scrollTo({ top: 0, left: 0 });
  }, [workspaceView]);

  const closeTransientOverlays = useCallback(() => {
    setTransientOverlayDismissSignal((signal) => signal + 1);
    closeCreationAccountMenu();
  }, [closeCreationAccountMenu]);

  const prepareAccountSurface = useCallback(() => {
    closeTransientOverlays();
    setSidebarImDetailConnectionId("");
    setSettingsFocus(null);
    setSettingsTarget(null);
  }, [closeTransientOverlays, setSettingsFocus, setSettingsTarget]);
  const showAccountSurface = useCallback(() => setWorkspaceView("account"), []);
  const {
    accountCenterRequest,
    accountLogoutPending,
    openAccountCenter,
    openAccountPayments,
    requestAccountLogout,
  } = useAccountNavigation(prepareAccountSurface, showAccountSurface, {
    beforeLogout: closeCreationAccountMenu,
    logout: () => accountBridge.CloudLogout(),
    onLogoutError: (cause) => showError("account.logout", cause, t("account.logoutFailed")),
  });

  const showChatView = useCallback((clearExpert = false) => {
    setWorkspaceView("chat");
    if (clearExpert) setActiveExpert(null);
  }, []);

  const closeNarrowSidebar = useCallback(() => {
    const floatingBreakpoint = sidebarCreation ? CREATION_SIDEBAR_FLOAT_BREAKPOINT : SIDEBAR_FLOAT_BREAKPOINT;
    if (viewportWidth > floatingBreakpoint) return;
    sidebarManualExpandedRef.current = false;
    setSidebarCollapsed(true);
  }, [setSidebarCollapsed, sidebarCreation, viewportWidth]);

  const clearWorkspaceSidebarDetail = useCallback(() => setSidebarImDetailConnectionId(""), []);
  const clearWorkspaceSettingsTarget = useCallback(() => {
    setSettingsFocus(null);
    setSettingsTarget(null);
  }, [setSettingsFocus, setSettingsTarget]);
  const closeHeartbeatOverlay = useCallback(() => setHeartbeatOpen(false), [setHeartbeatOpen]);
  const {
    openExpertDirectory: navigateToExpertDirectory,
    openModelMarketplace,
    openProjectDirectory,
    openCloudProject,
    openSkillsWorkspace,
    openConnectorsWorkspace,
    openAutomationWorkspace,
    openArtifactLibrary,
    openSuperpowerWorkspace: navigateToSuperpowerWorkspace,
    openCapabilityWorkspace: navigateToCapabilityWorkspace,
  } = useWorkspaceNavigation({
    closeTransientOverlays,
    closeNarrowSidebar,
    clearSidebarDetail: clearWorkspaceSidebarDetail,
    clearSettingsTarget: clearWorkspaceSettingsTarget,
    closeHeartbeatOverlay,
    setSelectedCloudProjectId,
    setWorkspaceView,
  });
  const openSuperpowerWorkspace = useCallback(() => {
    setSuperpowerEntry((current) => ({ screen: "catalog", key: current.key + 1 }));
    navigateToSuperpowerWorkspace();
  }, [navigateToSuperpowerWorkspace]);
  const openExpertDirectory = useCallback(() => {
    void refreshExpertState();
    navigateToExpertDirectory();
  }, [navigateToExpertDirectory, refreshExpertState]);
  const openCapabilityWorkspace = useCallback((view: CapabilityWorkspaceView) => {
    if (view === "experts") {
      openExpertDirectory();
      return;
    }
    navigateToCapabilityWorkspace(view);
  }, [navigateToCapabilityWorkspace, openExpertDirectory]);

  const enabledComposerConnectors = enabledConnectorsByScope[sidebarAccountScopeKey] ?? [];
  const activeProjectComposerAdapter = projectComposerAdapterForTab(projectComposerAdaptersRef.current, activeTabId);
  const activeProjectCapabilities = projectComposerCapabilitiesForTab(projectComposerAdaptersRef.current, activeTabId);
  const activeComposerCapabilities = activeProjectCapabilities
    ? { ...activeProjectCapabilities, connectors: enabledComposerConnectors }
    : { connectors: enabledComposerConnectors };

  const resolveAvailableProjectCapabilities = async (selection: ComposerCapabilitySelection) => {
    const [installedSkills, connectorAvailability] = await Promise.all([
      (selection.skills?.length ?? 0) > 0
        ? app.SkillsSettings().then((settings) => settings.skills)
        : Promise.resolve([]),
      (selection.connectors?.length ?? 0) > 0
        ? app.ListProjectConnectorAvailability()
        : Promise.resolve([]),
    ]);
    return availableProjectCapabilitySelection(selection, installedSkills, connectorAvailability);
  };

  useEffect(() => {
    saveEnabledConnectorSelections(enabledConnectorsByScope);
  }, [enabledConnectorsByScope]);

  useEffect(() => {
    const handleRemoved = (event: Event) => {
      const name = (event as CustomEvent<string>).detail;
      if (!name) return;
      setEnabledConnectorsByScope((current) => removeConnectorFromEnabledSelections(current, name));
      for (const adapter of projectComposerAdaptersRef.current.values()) {
        const connectors = adapter.capabilities.connectors ?? [];
        if (connectors.includes(name)) {
          adapter.capabilities = { ...adapter.capabilities, connectors: connectors.filter((connector) => connector !== name) };
        }
      }
      setProjectComposerRevision((revision) => revision + 1);
    };
    window.addEventListener(MCP_SERVER_REMOVED_EVENT, handleRemoved);
    return () => window.removeEventListener(MCP_SERVER_REMOVED_EVENT, handleRemoved);
  }, []);

  const toggleComposerConnector = useCallback((name: string) => {
    setEnabledConnectorsByScope((current) => {
      const names = current[sidebarAccountScopeKey] ?? [];
      const next = names.includes(name) ? names.filter((item) => item !== name) : [...names, name];
      return { ...current, [sidebarAccountScopeKey]: next };
    });
  }, [sidebarAccountScopeKey]);

  const selectComposerSkill = useCallback((name: string) => {
    closeTransientOverlays();
    closeNarrowSidebar();
    showChatView(false);
    setComposerInsertRequest({ id: Date.now(), text: `${formatSkillToken(name)} `, kind: "skill" });
    showToast(t("workspace.skills.insertedToast", { name }), "info");
  }, [closeNarrowSidebar, closeTransientOverlays, showChatView, showToast, t]);

  const enableComposerConnector = useCallback((name: string) => {
    const target = name.trim();
    if (!target) return;
    setEnabledConnectorsByScope((current) => {
      const connectors = current[sidebarAccountScopeKey] ?? [];
      if (connectors.includes(target)) return current;
      return { ...current, [sidebarAccountScopeKey]: [...connectors, target] };
    });
  }, [sidebarAccountScopeKey]);

  const connectComposerConnector = useCallback(async (name: string): Promise<void> => {
    const target = name.trim();
    const server = await ensureUsableMCPServer(
      target,
      () => app.MCPServers(),
      (serverName) => app.ReconnectMCPServer(serverName),
    );
    notice(`已连接 ${target} · ${server.tools} 个工具，请在连接器菜单中启用`);
  }, [notice]);

  const selectComposerConnector = useCallback(async (name: string) => {
    if (activeComposerCapabilities.connectors?.includes(name)) {
      toggleComposerConnector(name);
      return;
    }
    toggleComposerConnector(name);
    showToast(t("workspace.connectors.useReminderToast", { name }), "info");
  }, [activeComposerCapabilities.connectors, showToast, t, toggleComposerConnector]);

  const tryComposerConnector = useCallback((name: string, prompt: string) => {
    enableComposerConnector(name);
    closeTransientOverlays();
    closeNarrowSidebar();
    showChatView(false);
    setComposerInsertRequest({ id: Date.now(), text: prompt, mode: "replace" });
    showToast(t("workspace.connectors.tryReadyToast", { name }), "info");
  }, [closeNarrowSidebar, closeTransientOverlays, enableComposerConnector, showChatView, showToast, t]);

  const reloadSidebarImConnections = useCallback(async () => {
    const [settings, runtimeStatus] = await Promise.all([
      app.DesktopStartupSettings(),
      loadBotRuntimeStatus(),
    ]);
    setSidebarImConnections(sidebarImConnectionsFromBot(settings.bot, t, runtimeStatus));
    setImTopicSources(sidebarImTopicSourcesFromBot(settings.bot, t));
  }, [t]);

  const refreshSidebarImConnectionsFromSettings = useCallback(async (settings: Pick<SettingsView | DesktopStartupSettingsView, "bot">) => {
    const runtimeStatus = await loadBotRuntimeStatus();
    setSidebarImConnections(sidebarImConnectionsFromBot(settings.bot, t, runtimeStatus));
    setImTopicSources(sidebarImTopicSourcesFromBot(settings.bot, t));
  }, [t]);

  const openBotSettings = useCallback(() => {
    setSidebarImDetailConnectionId("");
    setSettingsFocus(null);
    openConnectorsWorkspace();
  }, [openConnectorsWorkspace, setSettingsFocus]);

  const openBotAllowlistSettings = useCallback((_connectionId: string) => {
    setSidebarImDetailConnectionId("");
    setSettingsFocus(null);
    openConnectorsWorkspace();
  }, [openConnectorsWorkspace, setSettingsFocus]);

  const pulseSidebarToggle = useCallback(() => {
    if (typeof window === "undefined") return;
    if (sidebarTogglePressTimerRef.current !== null) {
      window.clearTimeout(sidebarTogglePressTimerRef.current);
    }
    setSidebarTogglePressed(true);
    sidebarTogglePressTimerRef.current = window.setTimeout(() => {
      sidebarTogglePressTimerRef.current = null;
      setSidebarTogglePressed(false);
    }, 260);
  }, []);

  const pulseWorkspaceToggle = useCallback(() => {
    if (typeof window === "undefined") return;
    if (workspaceTogglePressTimerRef.current !== null) {
      window.clearTimeout(workspaceTogglePressTimerRef.current);
    }
    setWorkspaceTogglePressed(true);
    workspaceTogglePressTimerRef.current = window.setTimeout(() => {
      workspaceTogglePressTimerRef.current = null;
      setWorkspaceTogglePressed(false);
    }, 260);
  }, []);

  const anchorAppScrollToChat = useCallback(() => {
    if (typeof window === "undefined") return;
    const el = appRef.current;
    if (!el) return;
    const pin = () => {
      el.scrollLeft = 0;
    };
    pin();
    window.requestAnimationFrame(pin);
    window.setTimeout(pin, 300);
  }, []);

  useEffect(() => {
    return () => {
      if (sidebarTogglePressTimerRef.current !== null) {
        window.clearTimeout(sidebarTogglePressTimerRef.current);
      }
      if (workspaceTogglePressTimerRef.current !== null) {
        window.clearTimeout(workspaceTogglePressTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const syncDesktopPreferences = async () => {
      const legacyLanguage = readLegacyLangPref();
      const legacyTheme = readLegacyThemePreference();
      if (legacyLanguage || legacyTheme.hasValue) {
        await app.MigrateDesktopPreferences(legacyLanguage, legacyTheme.theme, "");
        clearLegacyLangPref();
        clearLegacyThemePreference();
      }
      const [settings, runtimeStatus] = await Promise.all([
        app.DesktopStartupSettings(),
        loadBotRuntimeStatus(),
      ]);
      if (cancelled) return;
      applyDesktopPreferences(settings);
      hydrateDisplayMode(settings.displayMode);
      setSidebarImConnections(sidebarImConnectionsFromBot(settings.bot, t, runtimeStatus));
      setImTopicSources(sidebarImTopicSourcesFromBot(settings.bot, t));
    };
    void syncDesktopPreferences().catch((e) => {
      console.warn("desktop preferences sync failed", e);
      enableStartupUpdateChecks();
    });
    return () => {
      cancelled = true;
    };
  }, [applyDesktopPreferences, enableStartupUpdateChecks, t]);

  useEffect(() => {
    setSidebarImDetailConnectionId((current) => {
      if (!current) return "";
      return sidebarImConnections.some((connection) => connection.id === current) ? current : "";
    });
  }, [sidebarImConnections]);

  // Open settings when the native menu item (CmdOrCtrl+,) is activated.
  useEffect(() => {
    if (typeof window === "undefined" || !window.runtime) return;
    return window.runtime.EventsOn("app:open-settings", () => {
      closeTransientOverlays();
      setSettingsTarget("general");
    });
  }, [closeTransientOverlays]);
  useEffect(() => {
    if (typeof window === "undefined") return;
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const creationSidebarFloating = sidebarCreation && viewportWidth <= CREATION_SIDEBAR_FLOAT_BREAKPOINT;
  useEffect(() => {
    const floatingBreakpoint = sidebarCreation ? CREATION_SIDEBAR_FLOAT_BREAKPOINT : SIDEBAR_FLOAT_BREAKPOINT;
    if (creationSidebarFloating && creationSidebarRestoreCollapsedRef.current === null) {
      creationSidebarRestoreCollapsedRef.current = sidebarCollapsed;
    } else if (!creationSidebarFloating && creationSidebarRestoreCollapsedRef.current !== null) {
      const restoreCollapsed = creationSidebarRestoreCollapsedRef.current;
      creationSidebarRestoreCollapsedRef.current = null;
      sidebarManualExpandedRef.current = false;
      sidebarAutoCollapsedRef.current = false;
      if (sidebarCollapsed !== restoreCollapsed) setSidebarCollapsed(restoreCollapsed);
      return;
    }
    const minViewportForSidebar = sidebarWidth + CHAT_MIN_WIDTH;
    const viewportTooNarrowForSidebar = viewportWidth < minViewportForSidebar || viewportWidth <= floatingBreakpoint;
    if (viewportTooNarrowForSidebar && sidebarManualExpandedRef.current) return;
    if (
      shouldAutoCollapseSidebarForViewport({
        viewportWidth,
        sidebarCollapsed,
        sidebarWidth,
        chatMinWidth: CHAT_MIN_WIDTH,
        floatingSidebarBreakpoint: floatingBreakpoint,
      })
    ) {
      sidebarAutoCollapsedRef.current = true;
      setSidebarCollapsed(true);
      return;
    }
    if (!viewportTooNarrowForSidebar && sidebarAutoCollapsedRef.current && sidebarCollapsed) {
      sidebarManualExpandedRef.current = false;
      sidebarAutoCollapsedRef.current = false;
      setSidebarCollapsed(false);
    }
    if (!sidebarCollapsed) {
      if (!viewportTooNarrowForSidebar) sidebarManualExpandedRef.current = false;
      sidebarAutoCollapsedRef.current = false;
    }
  }, [creationSidebarFloating, setSidebarCollapsed, sidebarCollapsed, sidebarCreation, sidebarWidth, viewportWidth]);

  const [pendingPlanRevision, setPendingPlanRevision] = useState<string | null>(null);
  const [footerHeight, setFooterHeight] = useState(0);
  const footerHeightRef = useRef(0);
  const footerRef = useRef<HTMLElement>(null);
  const runningRef = useRef(state.running);
  const activeTabIdRef = useRef(activeTabId);
  activeTabIdRef.current = activeTabId;
  const commitThenSendRef = useRef<(displayText: string, submitText?: string, capabilities?: ComposerCapabilitySubmit) => Promise<void>>(async () => {});
  const workspacePanelAvailable = workspaceView === "chat";
  const workspacePanelLayoutOpen = workspacePanelAvailable && workspacePanelOpen;
  const workspaceLayoutSidebarCollapsed = sidebarCollapsed || creationSidebarFloating;
  const {
    rightDockDetailActive,
    preferredWorkspacePanelWidth,
    workspacePanelMinWidth,
    chatReservedWidth,
    workspacePanelRenderWidth,
    workspacePanelRenderable,
    workspacePanelGridOpen,
    workspacePanelExpandDisabled,
    resolveLiveWorkspacePanelRenderWidth,
  } = useWorkspaceDockLayout({
    rightDockMode,
    workspacePreviewActive,
    rightDockPreviewWidth,
    rightDockTreeWidth,
    workspacePanelOpen: workspacePanelLayoutOpen,
    workspacePanelMaximized,
    liveWorkspacePanelRenderWidth,
    viewportWidth,
    sidebarCollapsed: workspaceLayoutSidebarCollapsed,
    sidebarWidth,
  });
  const workspacePanelLabel = workspacePanelRenderable ? t("rightDock.collapse") : t("rightDock.expand");
  const workspacePanelTooltipLabel = workspacePanelExpandDisabled ? t("rightDock.expandUnavailableNarrow") : workspacePanelLabel;
  const displayedScene = activeTab?.id
    ? draftScenesByTab[activeTab.id] ?? activeTab.scene
    : activeTab?.scene;
  const activeTopicTurnEvidenceKey = topicTurnEvidenceKey(activeTab);
  const composerSessionKey = useMemo(
    () => composerDraftKeyForTab(
      surfaceTransitionActive ? renderedViewTarget : activeTab,
      surfaceTransitionActive ? undefined : activeTabId,
    ) || "__draft__",
    [activeTab, activeTabId, renderedViewTarget, surfaceTransitionActive],
  );
  const sidebarImDetailConnection = useMemo(
    () => sidebarImConnections.find((connection) => connection.id === sidebarImDetailConnectionId) ?? null,
    [sidebarImConnections, sidebarImDetailConnectionId],
  );
  useEffect(() => {
    let cancelled = false;
    const evidenceKey = topicTurnEvidenceKey(activeTab);
    if (!evidenceKey) {
      setActiveTopicTurnEvidence({ key: "", state: "unavailable" });
      return () => {
        cancelled = true;
      };
    }
    setActiveTopicTurnEvidence({ key: evidenceKey, state: "loading" });
    void app.ListProjectTree()
      .then((tree) => {
        if (cancelled) return;
        const turns = activeTopicTurnsFromTree(asArray(tree), activeTab);
        setActiveTopicTurnEvidence(turns === undefined
          ? { key: evidenceKey, state: "unavailable" }
          : { key: evidenceKey, state: "ready", turns });
      })
      .catch(() => {
        if (!cancelled) setActiveTopicTurnEvidence({ key: evidenceKey, state: "unavailable" });
      });
    return () => {
      cancelled = true;
    };
  }, [activeTab?.id, activeTab?.scope, activeTab?.topicId, activeTab?.workspaceRoot, projectRevision]);
  const activeTopicTurns = activeTopicTurnEvidence.key === activeTopicTurnEvidenceKey
    && activeTopicTurnEvidence.state === "ready"
    ? activeTopicTurnEvidence.turns
    : undefined;
  const sessionTurns = useMemo(() => {
    const visibleUserTurns = state.items.reduce((count, item) => (item.kind === "user" ? count + 1 : count), 0);
    const currentTabTurns = Math.max(state.checkpoints.length, visibleUserTurns);
    return currentTabTurns > 0 ? currentTabTurns : activeTopicTurns ?? 0;
  }, [activeTopicTurns, state.checkpoints.length, state.items]);
  const startupSplashHold = !activeTabId && state.meta?.ready !== true && !state.meta?.startupErr;
  const renderedSessionPath = renderedViewTarget?.sessionPath ?? state.meta?.sessionPath;
  const blankSessionRequestPending = pendingBlankSessionRequest !== null;
  const blankSessionPresentation = state.surfaceIntent === "blank"
    || (state.surfaceIntent === "unknown" && !renderedSessionPath?.trim())
    || blankSessionRequestPending;
  const blankSessionHistoryPending = blankSessionRequestPending
    || (blankSessionPresentation && state.hydrateHistoryLoaded !== true);
  const blankSceneSelectionPending = Boolean(pendingBlankSessionRequest && blankSessionPresentation);
  const zeroTurnTopicBindingPending = surfaceTransitionActive
    && renderedViewTarget?.sessionTurns === 0
    && conversationStateKey !== composerSessionKey;
  const preparedScenePresentation = normalizeScenePresentation(
    renderedViewTarget?.scene,
    renderedViewTarget?.sceneState,
  );
  const activeScenePresentation = normalizeScenePresentation(displayedScene, activeTab?.sceneState);
  const welcomeScenePresentation = blankSceneSelectionPending
    ? normalizeScenePresentation(
      pendingBlankSessionRequest?.previewScene,
      pendingBlankSessionRequest?.previewSceneState,
    )
    : zeroTurnTopicBindingPending
      ? preparedScenePresentation
      : activeScenePresentation;
  const blankSceneIdentityUnbound = blankSessionPresentation
    && (!activeTab || activeTab.id !== activeTabId);
  const welcomeSceneSelectionAvailability = blankSceneSelectionPending
    || zeroTurnTopicBindingPending
    || blankSceneIdentityUnbound
    ? "disabled"
    : activeTab?.id === activeTabId && blankSessionPresentation
      ? welcomeScenePresentation.sceneState === "draft" ? "enabled" : "disabled"
      : "hidden";
  const nonBlankSessionPresentation = !blankSessionRequestPending && (
    state.surfaceIntent === "existing-session"
    || (state.surfaceIntent === "unknown" && Boolean(renderedSessionPath?.trim()))
  );
  const existingSessionHydrating = nonBlankSessionPresentation
    && state.hydrating
    && state.hydrateHistoryLoaded !== true;
  const existingSessionSubmitBlocked = blankSessionHistoryPending
    || (nonBlankSessionPresentation && (
      state.hydrating
      || state.hydrateHistoryLoaded !== true
      || state.backendActivationPending
      || state.meta?.ready !== true
    ));
  const hydrateStatusLabel = state.hydrating
    && state.surfaceIntent !== "blank"
    && !existingSessionHydrating
    && (!surfaceTransitionActive || surfaceLoadingHintVisible)
    ? state.hydrateReason === "switch-tab"
      ? t("status.hydrateSwitch")
      : state.hydrateReason === "resume-session"
        ? t("status.hydrateResume")
        : state.hydrateReason === "new-session"
          ? t("status.hydrateNewSession")
          : t("status.hydrateSync")
    : undefined;
  const activeComposerProfile = !surfaceTransitionActive && activeTabId
    ? composerProfilesByTab[activeTabId]
    : undefined;
  const backendActiveComposerProfile = useMemo(() => {
    if (surfaceTransitionActive) {
      return state.meta ? composerProfileFromMeta(state.meta) : defaultComposerProfile;
    }
    if (state.meta) {
      return composerProfileFromMeta(
        state.meta,
        activeTab ? composerProfileMode(composerProfileFromTab(activeTab, activeComposerProfile?.toolApprovalMode)) : undefined,
        activeComposerProfile?.toolApprovalMode,
      );
    }
    return composerProfileFromTab(activeTab, activeComposerProfile?.toolApprovalMode);
  }, [activeComposerProfile?.toolApprovalMode, activeTab, state.meta, surfaceTransitionActive]);
  const composerProfile = !surfaceTransitionActive && activeTabId
    ? activeComposerProfile ?? backendActiveComposerProfile
    : backendActiveComposerProfile;
  const goal = composerProfile.goal;
  const collaborationMode = displayedComposerProfileCollaborationMode(composerProfile);
  const toolApprovalMode = composerProfile.toolApprovalMode;
  const tokenMode: TokenMode = composerProfile.tokenMode;
  const sendAvailability = state.meta?.sendAvailability ?? activeTab?.sendAvailability;
  const accountHasRecoverableSubscription = hasRecoverableCloudIdentity(sidebarAccount);
  const existingAllowSubscriptionModels = accountHasRecoverableSubscription
    && sendAvailability?.state !== "subscription_auth_required"
    && sendAvailability?.state !== "subscription_account_action_required";
  const subscriptionAccess = useMemo(() => resolveSubscriptionAccess({
    initialized: sidebarAccountInitialized,
    hasBinding: Boolean(
      sidebarAccount?.profile_id?.trim()
      && sidebarAccount.cloud_user_id?.trim()
      && (sidebarAccount.binding_generation ?? 0) > 0
    ),
    allowSubscriptionModels: existingAllowSubscriptionModels,
    plan: sidebarAccountPlan,
    status: sidebarAccountSubscriptionStatus,
  }), [
    existingAllowSubscriptionModels,
    sidebarAccount?.binding_generation,
    sidebarAccount?.cloud_user_id,
    sidebarAccount?.profile_id,
    sidebarAccountInitialized,
    sidebarAccountPlan,
    sidebarAccountSubscriptionStatus,
  ]);
  const subscriptionAccountState = resolveSubscriptionAccountState({
    initialized: sidebarAccountInitialized,
    available: sidebarAccountAvailable,
    accountStatus: sidebarAccount?.status,
    hasBinding: accountHasRecoverableSubscription,
    access: subscriptionAccess,
  });
  const settingsScopeKey = useMemo(
    () => settingsSnapshotScopeKey(sidebarAccountScopeKey, activeTab),
    [activeTab, sidebarAccountScopeKey],
  );
  useEffect(() => {
    if (typeof app.SettingsCore !== "function") return;
    return scheduleSettingsCorePrefetch(settingsScopeKey, () => {
      void loadSettingsFeature().catch(() => {});
      return app.SettingsCore();
    });
  }, [settingsScopeKey]);
  const displayedSendAvailability = resolveDisplayedSendAvailability({
    availability: sendAvailability,
    billingSource: activeTab?.billingSource,
    accountState: subscriptionAccountState,
    totalBalance: sidebarAccountBalance,
  });
  const subscriptionRepairAccountKey = [
    sidebarAccount?.profile_id?.trim() ?? "",
    sidebarAccount?.cloud_user_id?.trim() ?? "",
    String(sidebarAccount?.binding_generation ?? 0),
    String(sidebarAccountSubscriptionVersion ?? ""),
    sidebarAccountPlan ?? "",
    sidebarAccountSubscriptionStatus ?? "",
    sidebarAccount?.status ?? "",
  ].join("\u0000");
  const modelCatalogAccountScopeKey = [
    sidebarAccount?.profile_id?.trim() ?? "",
    sidebarAccount?.cloud_user_id?.trim() ?? "",
    String(sidebarAccount?.binding_generation ?? 0),
  ].join("\u0000");
  const modelCatalogRefreshEligible = Boolean(
    sidebarAccountInitialized
    && sidebarAccount?.status === "authenticated"
    && sidebarAccount.profile_id?.trim()
    && sidebarAccount.cloud_user_id?.trim()
    && (sidebarAccount.binding_generation ?? 0) > 0,
  );
  useEffect(() => {
    if (!modelCatalogRefreshEligible) {
      cancelSubscriptionMarketplaceRefresh(marketplaceRefreshScopeStatesRef.current);
      return;
    }
    return () => cancelSubscriptionMarketplaceRefresh(marketplaceRefreshScopeStatesRef.current);
  }, [modelCatalogAccountScopeKey, modelCatalogRefreshEligible]);
  const savedModelRef = activeTab
    ? activeTab.modelRef?.trim() ?? ""
    : state.meta?.modelRef?.trim() ?? "";
  const currentSubscriptionRepairFactsKey = subscriptionRepairFactsKey({
    accountKey: modelCatalogAccountScopeKey,
    tabID: activeTab?.id ?? "",
    revision: subscriptionRepairFactsRevision,
    activeLabel: savedModelRef,
    activeBillingSource: activeTab?.billingSource,
  });
  useEffect(() => {
    const tabID = activeTab?.id;
    const request = ++subscriptionRepairRequestRef.current;
    setSubscriptionRepairFacts({ key: currentSubscriptionRepairFactsKey, state: "loading" });
    void Promise.allSettled([
      tabID ? app.ModelsForTab(tabID) : app.Models(),
      app.Settings(),
      app.ModelMarketplace(),
    ]).then(([routes, settings, marketplace]) => {
      if (subscriptionRepairRequestRef.current !== request) return;
      const resolvedRoutes = routes.status === "fulfilled" ? asArray(routes.value) : undefined;
      const resolvedSettings = settings.status === "fulfilled" ? settings.value : undefined;
      const resolvedMarketplace = marketplace.status === "fulfilled" ? marketplace.value : undefined;
      if (resolvedRoutes) {
        modelEntitlementSnapshotRef.current = {
          accountKey: modelCatalogAccountScopeKey,
          routes: resolvedRoutes,
          marketplace: resolvedMarketplace,
        };
      }
      setSubscriptionRepairFacts({
        key: currentSubscriptionRepairFactsKey,
        state: "settled",
        routes: resolvedRoutes,
        settings: resolvedSettings,
        marketplace: resolvedMarketplace,
      });
    });
  }, [activeTab?.id, currentSubscriptionRepairFactsKey, modelCatalogAccountScopeKey]);
  const currentSubscriptionRepairFacts = subscriptionRepairFacts.key === currentSubscriptionRepairFactsKey
    ? subscriptionRepairFacts
    : { key: currentSubscriptionRepairFactsKey, state: "loading" as const };
  const subscriptionRepairRoutes = currentSubscriptionRepairFacts.state === "settled"
    ? currentSubscriptionRepairFacts.routes
    : undefined;
  const subscriptionRepairSettings = currentSubscriptionRepairFacts.state === "settled"
    ? currentSubscriptionRepairFacts.settings
    : undefined;
  const subscriptionRepairMarketplace = currentSubscriptionRepairFacts.state === "settled"
    ? currentSubscriptionRepairFacts.marketplace
    : undefined;
  useEffect(() => {
    if (
      !modelCatalogRefreshEligible
      || currentSubscriptionRepairFacts.state !== "settled"
      || !subscriptionMarketplaceNeedsRefresh(subscriptionRepairMarketplace)
    ) return;
    void ensureModelMarketplaceRefresh(modelCatalogAccountScopeKey);
  }, [
    currentSubscriptionRepairFacts.state,
    ensureModelMarketplaceRefresh,
    modelCatalogAccountScopeKey,
    modelCatalogRefreshEligible,
    subscriptionRepairMarketplace,
  ]);
  const sharedModelCatalog = useMemo<SharedModelCatalogState>(() => currentSubscriptionRepairFacts.state === "loading"
    ? { state: "loading" }
    : subscriptionRepairRoutes === undefined
      ? { state: "unavailable" }
      : {
          state: "ready",
          routes: subscriptionRepairRoutes,
          marketplace: subscriptionRepairMarketplace,
        }, [currentSubscriptionRepairFacts.state, subscriptionRepairRoutes, subscriptionRepairMarketplace]);
  // Chat-only fact refreshes do not discard this binding's last known marketplace.
  const automationCatalogSnapshot = modelEntitlementSnapshotRef.current;
  const automationModelCatalog: SharedModelCatalogState = sharedModelCatalog.state === "loading"
    && automationCatalogSnapshot?.accountKey === modelCatalogAccountScopeKey
    ? { state: "ready", routes: automationCatalogSnapshot.routes, marketplace: automationCatalogSnapshot.marketplace }
    : sharedModelCatalog;
  const currentModelRoute = subscriptionRepairRoutes
    ? resolveActiveModelRoute(
        subscriptionRepairRoutes,
        savedModelRef,
        activeTab?.billingSource,
      )
    : null;
  const currentModelRouteAccess = resolveModelRouteAccess(
    currentModelRoute,
    subscriptionRepairMarketplace as SubscriptionMarketplaceView | undefined,
    subscriptionAccess,
    currentSubscriptionRepairFacts.state === "loading",
    // Every authenticated Desktop plan includes the blank-session default.
    // Do not turn a cold marketplace cache into a false entitlement denial.
    true,
  );
  const subscriptionModelInvocationBaseAccess = resolveSubscriptionModelInvocationAccess({
    route: currentModelRoute,
    routeAccess: currentModelRouteAccess,
    factsState: currentSubscriptionRepairFacts.state,
    auxiliaryBillingSource: activeTab?.billingSource,
  });
  const subscriptionDefaultRepairDecision = resolveSubscriptionDefaultRepair(
    subscriptionRepairSettings,
    subscriptionRepairRoutes,
    subscriptionRepairMarketplace as SubscriptionMarketplaceView | undefined,
    subscriptionAccess,
  );
  const controllerReady = sendAvailabilityAllowsControllerActions(displayedSendAvailability) && !state.backendActivationPending && !state.hydratePreview;
  const sessionRuntimeBound = !state.backendActivationPending && state.meta?.ready === true;
  const hasLocalHistory = state.items.some((item) => item.kind !== "notice" && item.kind !== "compaction");
  const runtimeConnectionWaiting = Boolean(
    hasLocalHistory
    && !sessionRuntimeBound
    && !state.hydrateError,
  );
  const [runtimeConnectionStage, setRuntimeConnectionStage] = useState<"hidden" | "stalled">("hidden");
  useEffect(() => {
    if (!runtimeConnectionWaiting) {
      setRuntimeConnectionStage("hidden");
      return;
    }
    if (state.meta?.startupErr || displayedSendAvailability?.state === "startup_failed") {
      setRuntimeConnectionStage("stalled");
      return;
    }
    setRuntimeConnectionStage("hidden");
    const stalledTimer = window.setTimeout(() => setRuntimeConnectionStage("stalled"), 3_000);
    return () => {
      window.clearTimeout(stalledTimer);
    };
  }, [
    composerSessionKey,
    displayedSendAvailability?.state,
    runtimeConnectionWaiting,
    state.meta?.startupErr,
  ]);
  const blankSessionEligibleForSubscriptionRepair = isBlankSessionEligibleForSubscriptionRepair({
    tabKey: activeTopicTurnEvidenceKey,
    turnEvidence: activeTopicTurnEvidence,
    historyLoaded: state.hydrateHistoryLoaded === true,
    hydrating: state.hydrating,
    hydrateError: state.hydrateError,
    backendActivationPending: state.backendActivationPending,
    readOnly: Boolean(activeTab?.readOnly),
    running: state.running || Boolean(activeTab?.running),
    hasHistory: hasLocalHistory,
    hasCheckpoints: state.checkpoints.length > 0,
    hasLiveTurn: Boolean(state.live || state.turnActive),
  });
  const currentRouteIsByok = currentModelRouteAccess.state === "allowed"
    && currentModelRoute?.billingSource === "newapi_byok";
  const auxiliaryRouteIsByok = currentSubscriptionRepairFacts.state === "loading"
    && activeTab?.billingSource === "newapi_byok";
  const subscriptionRepairHasByokRoute = currentRouteIsByok || auxiliaryRouteIsByok;
  const subscriptionRepairRelevant = currentModelRoute?.billingSource === "subscription_pool"
    || activeTab?.billingSource === "subscription_pool"
    || Boolean(subscriptionRepairSettings && (
      !subscriptionRepairSettings.defaultModel.trim()
      || subscriptionRepairSettings.defaultBillingSource === "subscription_pool"
    ));
  const subscriptionRepairHasHistoricalEvidence = hasLocalHistory
    || state.checkpoints.length > 0
    || (
      activeTopicTurnEvidence.key === activeTopicTurnEvidenceKey
      && activeTopicTurnEvidence.state === "ready"
      && (activeTopicTurnEvidence.turns ?? 0) > 0
    );
  const subscriptionRepairClassificationPending = !subscriptionRepairHasHistoricalEvidence && Boolean(
    activeTopicTurnEvidenceKey
    && (
      activeTopicTurnEvidence.key !== activeTopicTurnEvidenceKey
      || activeTopicTurnEvidence.state !== "ready"
      || state.hydrating
      || state.hydrateHistoryLoaded !== true
    )
  );
  const subscriptionDefaultRepairGateBlocks = shouldBlockSubscriptionDefaultRepair({
    hasByokRoute: subscriptionRepairHasByokRoute,
    relevant: subscriptionRepairRelevant,
    classificationPending: subscriptionRepairClassificationPending,
    blankEligible: blankSessionEligibleForSubscriptionRepair,
    accountState: subscriptionAccountState,
    factsSettled: currentSubscriptionRepairFacts.state === "settled",
    decisionState: subscriptionDefaultRepairDecision.state,
  });
  const subscriptionRepairAttemptKey = blankSessionEligibleForSubscriptionRepair
    && subscriptionRepairSettings
    && subscriptionRepairRoutes
    && subscriptionRepairMarketplace
    ? subscriptionDefaultRepairAttemptKey({
        accountKey: subscriptionRepairAccountKey,
        tabKey: activeTopicTurnEvidenceKey,
        hydrationGeneration: state.sessionGen,
        settings: subscriptionRepairSettings,
        routes: subscriptionRepairRoutes,
        marketplace: subscriptionRepairMarketplace,
      })
    : "";
  const subscriptionRepairScopeKey = subscriptionDefaultRepairScopeKey({
    accountKey: subscriptionRepairAccountKey,
    tabKey: activeTopicTurnEvidenceKey,
    hydrationGeneration: state.sessionGen,
  });
  const subscriptionRepairContextKey = !subscriptionRepairHasByokRoute
    && subscriptionAccountState === "ready"
    && subscriptionDefaultRepairDecision.state === "repair"
    ? subscriptionRepairAttemptKey
    : "";
  subscriptionRepairContextRef.current = subscriptionRepairContextKey;
  subscriptionRepairScopeRef.current = subscriptionRepairScopeKey;
  const subscriptionRepairRunFailed = subscriptionRepairRunState?.scopeKey === subscriptionRepairScopeKey
    && subscriptionRepairRunState.attemptKey === subscriptionRepairAttemptKey
    && subscriptionRepairRunState.state === "failed";
  const subscriptionModelInvocationAccess: SubscriptionModelInvocationAccess =
    !activeTab
      ? { state: "allowed" }
      : subscriptionModelInvocationBaseAccess.state === "allowed" && subscriptionDefaultRepairGateBlocks
      ? {
          state: (subscriptionRepairContextKey && !subscriptionRepairRunFailed)
            || subscriptionRepairClassificationPending
            || currentSubscriptionRepairFacts.state === "loading"
            ? "loading"
            : "blocked",
        }
      : subscriptionModelInvocationBaseAccess;
  const subscriptionModelInvocationBlocked = subscriptionModelInvocationAccess.state !== "allowed";
  const subscriptionModelPendingVisible = useDelayedSubscriptionModelPending(
    currentSubscriptionRepairFactsKey,
    subscriptionModelInvocationAccess.state === "loading",
  );
  const displayedModelLabel = resolveSubscriptionModelGateLabel({
    state: subscriptionModelInvocationAccess.state,
    transitioning: surfaceTransitionActive,
    routeDisplayName: currentModelRoute?.displayName,
    routeModel: currentModelRoute?.model,
    sessionMetaLabel: state.meta?.label,
    tabLabel: activeTab?.label,
    fallbackLabel: t("subscriptionModelGate.selectModel"),
    blockedLabel: t("subscriptionModelGate.currentModelUnavailable"),
  });
  const resolvedModelSwitcherCurrentRef = currentModelRoute?.ref?.trim()
    || savedModelRef
    || "";
  const resolvedModelSwitcherCurrentBillingSource = currentModelRoute?.billingSource
    ?? (activeTab?.billingSource === "subscription_pool" || activeTab?.billingSource === "newapi_byok"
      ? activeTab.billingSource
      : undefined);
  const modelSwitcherPresentation = resolveModelSwitcherTransition({
    transition: modelSwitcherTransition,
    activeTabId: activeTab?.id ?? "",
    currentFactsKey: currentSubscriptionRepairFactsKey,
    currentFactsState: currentSubscriptionRepairFacts.state,
    currentRef: resolvedModelSwitcherCurrentRef,
    currentBillingSource: resolvedModelSwitcherCurrentBillingSource,
    selectionPhase: state.composerSelection.phase,
    selectionModelRef: state.composerSelection.modelRef,
  });
  const composerSelectionHasReliableSnapshot = hasReliableComposerSelectionSnapshot({
    selectionPhase: state.composerSelection.phase,
    selectionModelRef: state.composerSelection.modelRef,
    effort: state.effort,
    effortSnapshot: state.composerSelection.effortSnapshot,
  });
  const composerModelEntitlementHandoffActive = Boolean(
    renderedComposerIdentity?.billingSource === "subscription_pool"
    && !modelSwitcherTransition
    && (surfaceTransitionActive || currentSubscriptionRepairFacts.state === "loading"),
  );
  const composerModelEntitlementPresentation = composerModelEntitlementHandoffActive
    ? resolveComposerModelEntitlementPresentation({
        modelRef: renderedComposerIdentity?.modelRef ?? "",
        billingSource: renderedComposerIdentity?.billingSource,
        accountKey: modelCatalogAccountScopeKey,
        access: subscriptionAccess,
        snapshot: modelEntitlementSnapshotRef.current,
      })
    : null;
  const preserveComposerSnapshotDuringEntitlementPending = Boolean(
    composerModelEntitlementPresentation?.state === "pending"
    && composerSelectionHasReliableSnapshot,
  );
  const composerModelLabel = composerModelEntitlementPresentation?.state === "resolved"
    ? composerModelEntitlementPresentation.ref
    : composerModelEntitlementPresentation?.state === "choose"
      ? ""
      : composerModelEntitlementPresentation?.state === "pending"
        ? preserveComposerSnapshotDuringEntitlementPending
          ? renderedComposerIdentity?.modelRef?.trim() || state.meta?.modelRef?.trim() || ""
          : ""
        : surfaceTransitionActive
          ? state.meta?.modelRef?.trim() ?? ""
          : modelSwitcherPresentation.ref;
  const composerModelBillingSource = composerModelEntitlementPresentation?.state === "resolved"
    ? composerModelEntitlementPresentation.billingSource
    : composerModelEntitlementPresentation
      ? renderedComposerIdentity?.billingSource || undefined
      : surfaceTransitionActive
        ? renderedComposerIdentity?.billingSource || undefined
        : modelSwitcherPresentation.billingSource;
  const composerModelSelectionPresentation = composerModelEntitlementPresentation?.state === "choose"
    ? "choose" as const
    : composerModelEntitlementPresentation?.state === "pending" && !preserveComposerSnapshotDuringEntitlementPending
      ? "pending" as const
      : "auto" as const;
  // The combined controller selection can settle before the shared catalog and
  // active-tab facts publish the same identity. Existing-session handoffs keep a
  // complete target snapshot visible while busy; an explicit model switch keeps
  // its previous confirmed pair until the replacement is fully ready.
  const sessionComposerSelectionPending = shouldHoldComposerSelectionPresentation({
    selectionPhase: state.composerSelection.phase,
    selectionModelRef: state.composerSelection.modelRef,
    presentedModelRef: composerModelLabel,
    transitionPending: modelSwitcherPresentation.pending,
    surfaceTransitionActive,
    hasReliableSnapshot: composerSelectionHasReliableSnapshot,
  });
  const sessionComposerModelSelectionPresentation = composerModelSelectionPresentation === "choose"
    ? "choose" as const
    : composerModelSelectionPresentation === "pending"
      || (sessionComposerSelectionPending && !modelSwitcherPresentation.pending)
      ? "pending" as const
      : "auto" as const;
  const sessionComposerSelectionBusy = sessionComposerSelectionPending
    || surfaceTransitionActive
    || composerModelEntitlementPresentation?.state === "pending"
    || state.composerSelection.phase === "pending"
    || state.composerSelection.phase === "updating-effort";
  useEffect(() => {
    if (!modelSwitcherTransition || modelSwitcherPresentation.pending) return;
    setModelSwitcherTransition((current) => current === modelSwitcherTransition ? null : current);
  }, [modelSwitcherPresentation.pending, modelSwitcherTransition]);
  const refreshSubscriptionRepairFactsForSettings = useCallback((settings?: SettingsView | null) => {
    if (!settings) return;
    const defaultChanged = (
      settings.defaultModel !== subscriptionRepairSettings?.defaultModel
      || settings.defaultBillingSource !== subscriptionRepairSettings?.defaultBillingSource
    );
    const previousCatalog = settingsModelCatalogFingerprint(subscriptionRepairSettings);
    const nextCatalog = settingsModelCatalogFingerprint(settings);
    const catalogChanged = nextCatalog !== null && previousCatalog !== nextCatalog;
    if (catalogChanged || defaultChanged) setAutomationModelSettingsRevision((revision) => revision + 1);
    if (catalogChanged) refreshModelCatalog();
    else if (defaultChanged) refreshSubscriptionRepairFacts();
  }, [
    refreshModelCatalog,
    refreshSubscriptionRepairFacts,
    subscriptionRepairSettings,
  ]);
  const patchActiveComposerProfile = useCallback(
    (patch: Partial<Omit<ComposerProfile, "pending">>, pendingFields: ComposerProfileField[]) => {
      if (!activeTabId) return;
      setComposerProfilesByTab((current) => patchComposerProfile(current, activeTabId, composerProfile, patch, pendingFields));
    },
    [activeTabId, composerProfile],
  );
  const topicbarEditing = Boolean(activeTab?.topicId && activeTab.topicId === renamingTopicId);
  const visibleTabId = activeTabId;
  const visibleTabs = useMemo(() => {
    const byId = new Map(tabMetas.map((tab) => [tab.id, tab]));
    const ordered = tabOrderIds.map((id) => byId.get(id)).filter((tab): tab is TabMeta => Boolean(tab));
    const missing = tabMetas.filter((tab) => !tabOrderIds.includes(tab.id));
    return [...ordered, ...missing].map((tab) => {
      const profile = composerProfilesByTab[tab.id] ?? composerProfileFromTab(tab);
      return {
        ...tab,
        running: tab.id === visibleTabId ? tab.running || state.running : tab.running,
        mode: composerProfileMode(profile),
        collaborationMode: displayedComposerProfileCollaborationMode(profile),
        toolApprovalMode: profile.toolApprovalMode,
        tokenMode: profile.tokenMode,
        goal: profile.goal,
        active: tab.id === visibleTabId,
      };
    });
  }, [composerProfilesByTab, state.running, tabMetas, tabOrderIds, visibleTabId]);

  useEffect(() => {
    const ids = tabMetas.map((tab) => tab.id);
    setTabOrderIds((current) => {
      const next = current.filter((id) => ids.includes(id));
      for (const id of ids) {
        if (!next.includes(id)) next.push(id);
      }
      return next.join("\u0000") === current.join("\u0000") ? current : next;
    });
  }, [tabMetas]);

  useEffect(() => {
    const ids = new Set(tabMetas.map((tab) => tab.id));
    for (const id of Object.keys(yoloRestoreToolApprovalModesRef.current)) {
      if (!ids.has(id)) delete yoloRestoreToolApprovalModesRef.current[id];
    }
    userPlanModeByTabRef.current = pruneUserPlanModeIntents(userPlanModeByTabRef.current, ids);
    setDraftScenesByTab((current) => {
      const next = Object.fromEntries(Object.entries(current).filter(([id]) => ids.has(id)));
      return Object.keys(next).length === Object.keys(current).length ? current : next;
    });
    setComposerProfilesByTab((current) => hydrateComposerProfilesFromTabs(current, tabMetas));
  }, [tabMetas]);

  useEffect(() => {
    if (!renamingTopicId || activeTab?.topicId === renamingTopicId) return;
    topicRenameSkipCommitRef.current = false;
    topicRenameCommitHandledRef.current = false;
    setRenamingTopicId(null);
    setTopicTitleDraft("");
  }, [activeTab?.topicId, renamingTopicId]);

  useEffect(() => {
    if (!activeTabId || !state.meta) return;
    setComposerProfilesByTab((current) => hydrateComposerProfileFromMeta(current, activeTabId, state.meta!));
  }, [activeTabId, state.meta]);

  const syncModeToController = useCallback((m: Mode) => setControllerMode(m), [setControllerMode]);

  useEffect(() => {
    void app.SetTrayLocale(locale).catch(() => {});
  }, [locale]);

  // applyMode is the single source of truth for the input mode: it updates the
  // local pill and pushes the matching gate state to the controller (plan = read
  // only; yolo = auto-approve approval-gated tools while user decisions still wait).
  // normal clears both.
  const applyMode = useCallback(
    (m: Mode) => {
      userPlanModeByTabRef.current = updateUserPlanModeIntent(userPlanModeByTabRef.current, activeTabId, modeHasPlan(m));
      patchActiveComposerProfile(composerProfileWithMode(m), ["collaborationMode", "toolApprovalMode", "goal"]);
      void syncModeToController(m);
    },
    [activeTabId, patchActiveComposerProfile, syncModeToController],
  );
  const applyCollaborationMode = useCallback(
    (m: CollaborationMode, options: { rememberUserIntent?: boolean } = {}): Promise<void> => {
      if (options.rememberUserIntent !== false) {
        userPlanModeByTabRef.current = updateUserPlanModeIntent(userPlanModeByTabRef.current, activeTabId, m === "plan");
      }
      if (m === "goal") {
        patchActiveComposerProfile({ collaborationMode: "normal", goalDraftMode: true, goal: "" }, ["collaborationMode", "goal"]);
        return setControllerCollaborationMode("normal");
      }
      patchActiveComposerProfile({ collaborationMode: m, goalDraftMode: false, goal: "" }, ["collaborationMode", "goal"]);
      return setControllerCollaborationMode(m);
    },
    [activeTabId, patchActiveComposerProfile, setControllerCollaborationMode],
  );
  const applyToolApprovalMode = useCallback(
    async (m: ToolApprovalMode): Promise<void> => {
      if (!activeTabId) return;
      const targetTabId = activeTabId;
      const previousMode = toolApprovalMode;
      try {
        await setControllerToolApprovalMode(m);
        if (m === "yolo") {
          if (previousMode !== "yolo") {
            yoloRestoreToolApprovalModesRef.current[targetTabId] = restorableToolApprovalMode(previousMode);
          }
        } else {
          yoloRestoreToolApprovalModesRef.current[targetTabId] = restorableToolApprovalMode(m);
        }
        setComposerProfilesByTab((current) => patchComposerProfile(
          current,
          targetTabId,
          current[targetTabId] ?? composerProfile,
          { toolApprovalMode: m },
          ["toolApprovalMode"],
        ));
      } catch (error) {
        showError("composer.toolApprovalMode", error, "保存会话审批姿态失败，请稍后重试。");
      }
    },
    [activeTabId, composerProfile, setControllerToolApprovalMode, showError, toolApprovalMode],
  );
  const toggleYoloApprovalMode = useCallback(() => {
    if (!activeTabId) return;
    const next = toggleYoloToolApprovalMode(
      toolApprovalMode,
      yoloRestoreToolApprovalModesRef.current[activeTabId],
    );
    void applyToolApprovalMode(next.mode);
  }, [activeTabId, applyToolApprovalMode, toolApprovalMode]);
  const applyGoal = useCallback(
    (nextGoal: string) => {
      userPlanModeByTabRef.current = updateUserPlanModeIntent(userPlanModeByTabRef.current, activeTabId, false);
      const trimmed = nextGoal.trim();
      patchActiveComposerProfile({
        collaborationMode: trimmed ? "goal" : "normal",
        goalDraftMode: false,
        goal: trimmed,
      }, ["collaborationMode", "goal"]);
      void (trimmed ? setControllerGoal(trimmed) : clearControllerGoal());
    },
    [activeTabId, clearControllerGoal, patchActiveComposerProfile, setControllerGoal],
  );
  const applyTokenMode = useCallback(
    (m: TokenMode) => {
      patchActiveComposerProfile({ tokenMode: m }, ["tokenMode"]);
      void setTokenMode(m);
    },
    [patchActiveComposerProfile, setTokenMode],
  );
  // Shift+Tab toggles only the collaboration axis; Ctrl/Cmd+Y toggles YOLO on the
  // tool-permission axis while preserving the Ask/Auto base mode.
  const cycleMode = useCallback(() => {
    applyCollaborationMode(collaborationMode === "plan" ? "normal" : "plan");
  }, [applyCollaborationMode, collaborationMode]);

  // Switching models rebuilds the controller, which starts in normal mode — so
  // re-apply the current mode, or the pill would say plan/YOLO while the fresh
  // controller silently uses normal gating.
  const switchModel = useCallback(
    async (ref: string, billingSource?: ModelInfo["billingSource"]) => {
      const tabId = activeTab?.id;
      if (tabId) {
        setModelSwitcherTransition((current) => current?.tabId === tabId
          ? current
          : {
              tabId,
              startedFactsKey: currentSubscriptionRepairFactsKey,
              previousRef: resolvedModelSwitcherCurrentRef,
              previousBillingSource: resolvedModelSwitcherCurrentBillingSource,
              previousEffort: state.effort,
            });
      }
      await setModel(ref, billingSource);
      refreshSubscriptionRepairFacts();
      await setControllerCollaborationMode(controllerComposerProfileCollaborationMode(composerProfile));
      await syncControllerToolApprovalMode(toolApprovalMode);
      if (goal.trim()) await setControllerGoal(goal);
    },
    [
      activeTab?.id,
      composerProfile,
      currentSubscriptionRepairFactsKey,
      goal,
      refreshSubscriptionRepairFacts,
      resolvedModelSwitcherCurrentBillingSource,
      resolvedModelSwitcherCurrentRef,
      setControllerCollaborationMode,
      setControllerGoal,
      syncControllerToolApprovalMode,
      setModel,
      state.effort,
      toolApprovalMode,
    ],
  );

  // Startup and workspace/model rebuilds create a fresh controller in normal
  // mode. Re-apply the UI mode once the controller is ready, including the case
  // where the user picked YOLO while boot was still loading and the legacy
  // SetBypass binding was a harmless no-op.
  useEffect(() => {
    if (!controllerReady) return;
    void setControllerCollaborationMode(controllerComposerProfileCollaborationMode(composerProfile));
    void syncControllerToolApprovalMode(toolApprovalMode);
    if (goal.trim()) void setControllerGoal(goal);
  }, [composerProfile, controllerReady, goal, setControllerCollaborationMode, setControllerGoal, syncControllerToolApprovalMode, toolApprovalMode]);

  // The live task list pinned above the composer comes from the most recent
  // successful top-level todo_write result; failed or still-running attempts do
  // not advance the canonical panel state. Incomplete lists are always shown so
  // a stale local dismissal cannot hide work that still blocks final readiness;
  // completed lists collapse automatically and can then be dismissed. The
  // dismissal key is still based on stable todo content/state so history reloads
  // do not resurrect the same finished list under a different event id.
  const todoEntry = useMemo(() => {
    for (let i = state.items.length - 1; i >= 0; i--) {
      const it = state.items[i];
      if (it.kind === "tool" && it.name === "todo_write" && !it.parentId && it.status === "done" && !it.error) {
        return { item: it, index: i };
      }
    }
    return null;
  }, [state.items]);
  const todoItem = todoEntry?.item ?? null;
  const todos = useMemo(() => (todoItem ? parseTodos(todoItem.args) : []), [todoItem]);
  const todoBelongsToActiveTurn = useMemo(() => {
    if (!todoEntry) return false;
    for (let i = todoEntry.index + 1; i < state.items.length; i++) {
      if (state.items[i]?.kind === "user") return false;
    }
    return true;
  }, [state.items, todoEntry]);
  const [dismissedTodo, setDismissedTodo] = useState<string | null>(null);
  const todoKey = useMemo(() => todoDismissalKey(todos), [todos]);
  const todoActive = (state.running || state.turnActive) && todoBelongsToActiveTurn;
  const showTodos = shouldShowTodoPanel(todoKey, dismissedTodo, todos, todoActive);

  const sessionTitle = surfaceTransitionActive
    ? renderedViewTitle || t("transcript.loadingSession")
    : topicTitle(activeTab);
  const sessionHasContent = !blankSessionHistoryPending
    && (state.items.length > 0 || Boolean(state.live?.text || state.live?.reasoning));
  const getSessionMarkdown = useCallback(
    () => sessionItemsToMarkdown(sessionTitle, state.items, state.live),
    [sessionTitle, state.items, state.live],
  );
  const getSessionJson = useCallback(
    () => sessionItemsToJson(sessionTitle, state.items, state.live),
    [sessionTitle, state.items, state.live],
  );

  useEffect(() => {
    if (!topicExportOpen) return;
    const onDown = (event: MouseEvent) => {
      const target = event.target as Element | null;
      if (!target?.closest(".topicbar__export")) setTopicExportOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [topicExportOpen]);

  const exportSession = useCallback(
    async (format: "markdown" | "json" | "pdf" | "image") => {
      const base = safeFilename(sessionTitle);
      setTopicExportOpen(false);
      try {
        if (format === "json") {
          const path = await app.PickExportFile(`${base}.json`, "application/json");
          if (path) await app.SaveExportFile(path, getSessionJson(), false);
        } else if (format === "pdf") {
          const path = await app.PickExportFile(`${base}.pdf`, "application/pdf");
          if (!path) return;
          const { blobToBase64, renderSessionPdfBlob } = await import("./lib/sessionExport");
          const blob = await renderSessionPdfBlob(getSessionMarkdown(), sessionTitle);
          await app.SaveExportFile(path, await blobToBase64(blob), true);
        } else if (format === "image") {
          const path = await app.PickExportFile(`${base}.png`, "image/png");
          if (!path) return;
          const { blobToBase64, renderSessionImageBlob } = await import("./lib/sessionExport");
          const blob = await renderSessionImageBlob(getSessionMarkdown());
          await app.SaveExportFile(path, await blobToBase64(blob), true);
        } else {
          const path = await app.PickExportFile(`${base}.md`, "text/markdown");
          if (path) await app.SaveExportFile(path, getSessionMarkdown(), false);
        }
      } catch (err) {
        showError("session.export", err, "导出会话失败，请稍后重试。");
      }
    },
    [getSessionJson, getSessionMarkdown, sessionTitle, showError],
  );

  useEffect(() => {
    if (!pendingPlanRevision || state.running || subscriptionModelInvocationBlocked) return;
    const text = pendingPlanRevision;
    setPendingPlanRevision(null);
    void commitThenSendRef.current(text).catch((err) => {
      console.warn("Failed to submit pending plan revision", err);
    });
  }, [pendingPlanRevision, state.running, subscriptionModelInvocationBlocked]);

  useEffect(() => {
    setClearContextPending(false);
  }, [activeTabId]);

  const cancelClearContext = useCallback(() => {
    setClearContextPending(false);
  }, []);

  const confirmClearContext = useCallback(async () => {
    setClearContextPending(false);
    try {
      await clearSession();
      setDockRefreshKey((v) => v + 1);
      notice(t("clearContext.done"));
    } catch (err) {
      const message = reportClientError("session.clearContext", err, {
        fallbackMessage: t("clearContext.failed"),
      }).userMessage;
      notice(message, "warn");
    }
  }, [clearSession, notice, t]);

  // Keep runningRef in sync so handleSend sees the latest running value
  // even inside a stale closure.
  useEffect(() => {
    runningRef.current = state.running;
  }, [state.running]);

  // handleSend intercepts slash commands that need a desktop-native action before
  // they reach the backend: "/model <ref>" rebuilds on that model, "/memory"
  // opens Settings, and "/clear" shows an in-app confirmation card. Everything else — skills (/init, …),
  // custom commands, bare /model and the other read-only management verbs
  // (/skill, /mcp) — goes straight to Submit, which the controller
  // resolves (a turn, or a listing Notice).
  const handleSend = useCallback(
    async (displayText: string, submitText = displayText, capabilities?: ComposerCapabilitySubmit): Promise<import("./features/projects/model/projectComposerAdapter").ProjectComposerSendOutcome> => {
      const trimmed = displayText.trim();
      // "!<cmd>" runs a shell command directly, bypassing the model.
      if (trimmed.startsWith("!")) {
        const cmd = trimmed.slice(1).trim();
        if (!cmd) {
          notice("usage: !<command>  (e.g. !ls -la)");
          return "blocked";
        }
        await runShell(cmd);
        return "handled-no-turn";
      }
      const model = /^\/model\s+(\S+)$/.exec(trimmed);
      if (model) {
		notice("请在模型选择器中同时选择模型和计费通道");
        return "handled-no-turn";
      }
      if (trimmed === "/memory") {
        closeTransientOverlays();
        setSettingsTarget("memory");
        return "handled-no-turn";
      }
      if (trimmed === "/clear") {
        setClearContextPending(true);
        return "handled-no-turn";
      }
      const goalCommand = /^\/goal(?:\s+(.*))?$/.exec(trimmed);
      if (goalCommand) {
        if (subscriptionModelInvocationBlocked) return "blocked";
        const arg = (goalCommand[1] ?? "").trim();
        const displayGoal = stripGoalResearchFlags(arg);
        if (displayGoal && !["status", "clear", "off", "stop", "done"].includes(displayGoal.toLowerCase())) {
          if (hasGoalResearchFlag(arg)) {
            userPlanModeByTabRef.current = updateUserPlanModeIntent(userPlanModeByTabRef.current, activeTabId, false);
            patchActiveComposerProfile({
              collaborationMode: "goal",
              goalDraftMode: false,
              goal: displayGoal,
            }, ["collaborationMode", "goal"]);
          } else {
            applyGoal(displayGoal);
          }
        } else if (["clear", "off", "stop", "done"].includes(displayGoal.toLowerCase())) {
          applyGoal("");
        }
        if (!controllerReady) return "blocked";
        await commitThenSendRef.current(trimmed, submitText.trim());
        return "submitted";
      }
      if (collaborationMode === "goal" && !goal.trim()) {
        if (!controllerReady || subscriptionModelInvocationBlocked) return "blocked";
        applyGoal(trimmed);
        await commitThenSendRef.current(trimmed, `/goal ${trimmed}`);
        return "submitted";
      }
      const theme = /^\/theme(?:\s+(\S+))?$/.exec(trimmed);
      if (theme) {
        const arg = theme[1]?.toLowerCase();
        if (!arg) {
          const cur = getTheme();
          notice(t("settings.themeCurrent", { theme: cur, style: getThemeStyle(cur) }));
          return "handled-no-turn";
        }
        if (isThemeMode(arg)) {
          const next = arg;
          const style = defaultStyleForTheme(next);
          try {
            await app.SetDesktopAppearance(next, style);
            applyTheme(next, style);
            notice(t("settings.themeChanged", { theme: next, style }));
          } catch (err) {
            showError("settings.appearance.theme", err, "主题切换失败，请稍后重试。");
          }
          return "handled-no-turn";
        }
        if (isThemeStyle(arg)) {
          const cur = getTheme();
          const style = defaultStyleForTheme(cur);
          try {
            await app.SetDesktopAppearance(cur, arg);
            applyTheme(cur, style);
            notice(t("settings.themeChanged", { theme: cur, style }));
          } catch (err) {
            showError("settings.appearance.style", err, "界面风格切换失败，请稍后重试。");
          }
          return "handled-no-turn";
        }
        notice(t("settings.themeUnknown", { name: arg }), "warn");
        return "handled-no-turn";
      }
      const readOnlyManagementCommand = isReadOnlyManagementCommand(trimmed);
      const mcpPromptInvocation = isMCPPromptInvocation(trimmed);
      const selectedMCPServer = mcpConnectTarget(trimmed);
      if (runningRef.current) {
        if (subscriptionModelInvocationBlocked) return "blocked";
        await steer(submitText.trim());
        return "submitted";
      }
      if (!controllerReady || (subscriptionModelInvocationBlocked && !readOnlyManagementCommand)) return "blocked";
      if (selectedMCPServer) {
        await connectComposerConnector(selectedMCPServer);
        return "handled-no-turn";
      }
      if (readOnlyManagementCommand) {
        await send(trimmed, submitText.trim());
        return "handled-no-turn";
      }
      if (mcpPromptInvocation) {
        await send(trimmed, submitText.trim());
        return "submitted";
      }
      await setControllerCollaborationMode(controllerComposerProfileCollaborationMode(composerProfile));
      await syncControllerToolApprovalMode(toolApprovalMode);
      if (goal.trim()) await setControllerGoal(goal);
      const availableProjectCapabilities = activeProjectCapabilities
        ? await resolveAvailableProjectCapabilities(activeProjectCapabilities)
        : undefined;
      await commitThenSendRef.current(
        trimmed,
        submitText.trim(),
        availableProjectCapabilities
          ? projectComposerCapabilitySubmit(submitText, availableProjectCapabilities, capabilities, activeProjectCapabilities)
          : capabilities,
      );
      return "submitted";
    },
    [activeProjectCapabilities, activeTabId, applyGoal, closeTransientOverlays, collaborationMode, composerProfile, connectComposerConnector, controllerReady, goal, send, runShell, notice, setControllerCollaborationMode, setControllerGoal, syncControllerToolApprovalMode, steer, subscriptionModelInvocationBlocked, switchModel, t, toolApprovalMode, showError, showToast],
  );

  const refreshTabMetas = useCallback(async (
    shouldApply: () => boolean = () => true,
    observeListTabs?: (outcome: ProjectComposerAttempt["listTabsOutcome"]) => void,
  ): Promise<TabMeta[]> => {
    observeListTabs?.("not-called");
    let listTabsFailed = false;
    const tabs = asArray(await app.ListTabs().catch(() => {
      listTabsFailed = true;
      return [] as TabMeta[];
    }));
    observeListTabs?.(listTabsFailed ? "failed" : "succeeded");
    if (shouldApply()) {
      setTabMetas((current) => JSON.stringify(current) === JSON.stringify(tabs) ? current : tabs);
    }
    return tabs;
  }, []);
  useEffect(() => onModelRouteChanged((tabID) => {
    void refreshTabMetas();
    if (tabID && tabID !== activeTabId) return;
    refreshSubscriptionRepairFacts();
    void refreshMeta();
  }), [activeTabId, refreshMeta, refreshSubscriptionRepairFacts, refreshTabMetas]);
  useEffect(() => {
    if (
      !activeTab?.id
      || !subscriptionRepairAttemptKey
      || subscriptionRepairContextKey !== subscriptionRepairAttemptKey
      || subscriptionDefaultRepairDecision.state !== "repair"
    ) {
      return;
    }
    const tabID = activeTab.id;
    const candidate = subscriptionDefaultRepairDecision.candidate;
    const candidateRef = candidate.ref?.trim();
    if (!candidateRef) return;
    void runSubscriptionDefaultRepair({
      attemptedKeys: subscriptionRepairAttemptedKeysRef.current,
      attemptKey: subscriptionRepairAttemptKey,
      isTargetCurrent: () => subscriptionRepairContextRef.current === subscriptionRepairAttemptKey,
      isScopeCurrent: () => subscriptionRepairScopeRef.current === subscriptionRepairScopeKey,
      onClaim: () => {
        setSubscriptionRepairRunState({
          scopeKey: subscriptionRepairScopeKey,
          attemptKey: subscriptionRepairAttemptKey,
          state: "running",
        });
      },
      apply: async (isScopeCurrent) => {
        await app.SetDefaultModelRoute(candidateRef, "subscription_pool");
        if (!isScopeCurrent()) return false;
        await app.SetModelRouteForTab(tabID, candidateRef, "subscription_pool");
        return true;
      },
      onApplied: () => {
        refreshSubscriptionRepairFacts();
        showToast(t("subscriptionDefaultRepair.adjusted"), "info", {
          durationMs: 6000,
          actionLabel: t("subscriptionDefaultRepair.openModels"),
          onAction: () => {
            closeTransientOverlays();
            setSettingsTarget("models");
          },
        });
        void Promise.allSettled([refreshTabMetas(), refreshMeta()]);
      },
    }).then((result) => {
      if (result === "duplicate") return;
      setSubscriptionRepairRunState((current) => settleSubscriptionDefaultRepairRunState(current, {
        scopeKey: subscriptionRepairScopeKey,
        attemptKey: subscriptionRepairAttemptKey,
        result,
        currentScopeKey: subscriptionRepairScopeRef.current,
      }));
    });
  }, [
    activeTab?.id,
    closeTransientOverlays,
    refreshMeta,
    refreshSubscriptionRepairFacts,
    refreshTabMetas,
    showToast,
    subscriptionDefaultRepairDecision,
    subscriptionRepairAttemptKey,
    subscriptionRepairContextKey,
    subscriptionRepairScopeKey,
    t,
  ]);
  const seedActiveTabMeta = useCallback((tab: TabMeta): void => {
    setTabMetas((current) => {
      const seeded = { ...tab, active: true };
      let found = false;
      const next = current.map((existing) => {
        if (existing.id === tab.id) {
          found = true;
          return { ...existing, ...seeded };
        }
        return existing.active ? { ...existing, active: false } : existing;
      });
      return found ? next : [...next, seeded];
    });
    setTabOrderIds((current) => current.includes(tab.id) ? current : [...current, tab.id]);
  }, []);

  useEffect(() => {
    const unsub = onConversationActivity((e) => {
      const turnTabId = resolvePlanRestoreTabId(e.tabId, activeTabIdRef.current);
      const currentAccountScope = conversationAttentionScopeRef.current;
      const eventTab = turnTabId ? tabMetasRef.current.find((item) => item.id === turnTabId) : undefined;
      const eventConversationIdentity = conversationAttentionIdentity({
        scope: e.conversationScope,
        workspaceRoot: e.conversationWorkspaceRoot,
        topicId: e.conversationTopicId,
        sessionPath: e.conversationSessionPath,
      });
      const turnConversationIdentity = eventConversationIdentity || conversationAttentionIdentity(eventTab);
      if (e.kind === "turn_started") {
        if (turnConversationIdentity) {
          conversationRunAccountScopesRef.current.set(turnConversationIdentity, currentAccountScope);
        }
        updateConversationUnreadCompletion(
          currentAccountScope,
          turnConversationIdentity,
          null,
        );
        window.setTimeout(() => setProjectRevision((value) => value + 1), 0);
        return;
      }
      if (e.kind !== "turn_done") return;
      const eventAccountScope = conversationRunAccountScopesRef.current.get(turnConversationIdentity)
        ?? currentAccountScope;
      conversationRunAccountScopesRef.current.delete(turnConversationIdentity);
      const activeConversationIdentityAtCompletion = conversationAttentionScopeRef.current === eventAccountScope
        && workspaceViewRef.current === "chat"
        && !histViewOpenRef.current
        ? conversationAttentionIdentity(
            tabMetasRef.current.find((item) => item.id === activeTabIdRef.current),
          )
        : "";
      const completedSuccessfully = !e.err && !e.interrupted;
      if (completedSuccessfully && shouldMarkTurnDoneUnread(
        turnConversationIdentity,
        activeConversationIdentityAtCompletion,
        e.err,
        e.interrupted,
      )) {
        updateConversationUnreadCompletion(eventAccountScope, turnConversationIdentity, Date.now());
      } else if (completedSuccessfully) {
        updateConversationUnreadCompletion(eventAccountScope, turnConversationIdentity, null);
      }
      window.setTimeout(() => {
        setProjectRevision((value) => value + 1);
        refreshTabMetas().then((tabs) => {
          if (!turnTabId) return;
          const tab = tabs.find((item) => item.id === turnTabId) ?? eventTab;
          const baseProfile = tab ? composerProfileFromTab(tab) : defaultComposerProfile;
          if (!shouldRestoreUserPlanModeForProfile(userPlanModeByTabRef.current, turnTabId, baseProfile)) {
            if (baseProfile.goal.trim()) {
              userPlanModeByTabRef.current = updateUserPlanModeIntent(userPlanModeByTabRef.current, turnTabId, false);
            }
            return;
          }
          setComposerProfilesByTab((current) => patchComposerProfile(
            current,
            turnTabId,
            current[turnTabId] ?? baseProfile,
            { collaborationMode: "plan", goalDraftMode: false, goal: "" },
            ["collaborationMode", "goal"],
          ));
          if (activeTabIdRef.current === turnTabId) {
            void setControllerCollaborationMode("plan");
          }
        });
      }, 250);
    });
    return unsub;
  }, [refreshTabMetas, setControllerCollaborationMode, updateConversationUnreadCompletion]);

  const blankSessionTarget = useCallback(() => ({ scope: "global", workspaceRoot: "" }), []);
  const expertNewTaskTarget = useCallback(() => {
    if (activeTab?.scope === "project" && activeTab.workspaceRoot) {
      return { scope: "project", workspaceRoot: activeTab.workspaceRoot };
    }
    return blankSessionTarget();
  }, [activeTab?.scope, activeTab?.workspaceRoot, blankSessionTarget]);

  const supersedeBlankSessionRequest = useCallback(() => {
    blankSessionRequestSeqRef.current += 1;
    setPendingBlankSessionRequest(null);
  }, []);

  const openBlankSession = useCallback(async (
    scope: string,
    workspaceRoot: string,
    allowCurrentBlankSceneReuse = false,
  ) => {
    surfaceNavigationSeqRef.current += 1;
    supersedeOpenTopicRequests({ seqRef: openTopicSeqRef, runningRef: openTopicRunningRef, pendingRef: openTopicPendingRef });
    const resolvedWorkspaceRoot = scope === "project" ? workspaceRoot : "";
    const previewScenePresentation = blankSessionRequestScenePresentation({
      targetScope: scope,
      targetWorkspaceRoot: resolvedWorkspaceRoot,
      activeTabId,
      activeTab,
      visibleScene: displayedScene,
      surfaceIntent: state.surfaceIntent,
      surfaceTransitionActive,
      hasVisibleContent: sessionHasContent,
      allowCurrentBlankSceneReuse,
    });
    const request = {
      seq: blankSessionRequestSeqRef.current + 1,
      scope,
      workspaceRoot: resolvedWorkspaceRoot,
      previewScene: previewScenePresentation.scene,
      previewSceneState: previewScenePresentation.sceneState,
    };
    blankSessionRequestSeqRef.current = request.seq;
    setPendingBlankSessionRequest(request);
    try {
      const meta = singleSurfaceLayout
        ? await ensureBlankSurface(scope, resolvedWorkspaceRoot)
        : await ensureBlankTab(scope, resolvedWorkspaceRoot);
      if (blankSessionRequestSeqRef.current !== request.seq) return false;
      seedActiveTabMeta(meta);
      setPendingBlankSessionRequest((current) => current?.seq === request.seq ? null : current);
      setProjectRevision((value) => value + 1);
      await refreshTabMetas(() => blankSessionRequestSeqRef.current === request.seq);
      if (blankSessionRequestSeqRef.current !== request.seq) return false;
      setTabRevealSignal((signal) => signal + 1);
      setTranscriptRevealSignal((signal) => signal + 1);
      return meta.id;
    } catch (error) {
      setPendingBlankSessionRequest((current) => current?.seq === request.seq ? null : current);
      if (blankSessionRequestSeqRef.current !== request.seq) return false;
      throw error;
    }
  }, [
    activeTab,
    activeTabId,
    displayedScene,
    ensureBlankSurface,
    ensureBlankTab,
    refreshTabMetas,
    seedActiveTabMeta,
    sessionHasContent,
    singleSurfaceLayout,
    state.surfaceIntent,
    surfaceTransitionActive,
  ]);

  const waitForExpertTargetReady = useCallback(async (targetTabId: string) => {
    await waitForBackendReady(async () => {
      const tabs = asArray<TabMeta>(await app.ListTabs().catch(() => [] as TabMeta[]));
      const target = tabs.find((tab) => tab.id === targetTabId);
      return !target || target.ready || Boolean(target.startupErr);
    });
  }, []);

  const activateExpertSelection = useCallback(async (
    selection: ExpertSelection,
    requestedTabId?: string,
    existingRequest?: PendingExpertActivation,
    navigation: "chat" | "preserve" = "chat",
    binding: "switch" | "attach" = "switch",
  ): Promise<boolean> => {
    const selectedAt = Date.now();
    if (navigation === "chat") setWorkspaceView("chat");
    const isBackendExpert = activatableExpertIds.has(selection.id);
    let activationAppliedToCurrentTab = !isBackendExpert;
    if (isBackendExpert) {
      const targetTabId = requestedTabId ?? activeTabIdRef.current;
      if (!targetTabId) {
        showError("experts.activate", new Error("expert target tab is unavailable"), "启用专家失败，请稍后重试。");
        return false;
      }
      const request = existingRequest ?? {
        expertId: selection.id,
        tabId: targetTabId,
        requestSequence: ++expertActivationSequenceRef.current,
      };
      if (request.requestSequence !== expertActivationSequenceRef.current) return false;
      request.tabId = targetTabId;
      pendingExpertActivationRef.current = request;
      const sessionPath = tabMetasRef.current.find((tab) => tab.id === targetTabId)?.sessionPath;
      const targetIsCurrent = () => request.requestSequence === expertActivationSequenceRef.current
        && tabMetasRef.current.find((tab) => tab.id === targetTabId)?.sessionPath === sessionPath;
      try {
        const next = await enqueueExpertMutation(async () => {
          await waitForExpertTargetReady(targetTabId);
          if (!targetIsCurrent()) return null;
          return binding === "attach"
            ? app.AttachExpertForTab(targetTabId, selection.id)
            : app.SetExpertForTab(targetTabId, selection.id);
        }, targetTabId);
        if (!targetIsCurrent()) return false;
        if (!next) return false;
        const normalized = expertSelectionFromState(next, expertCatalog, userExpertCatalog) ?? selection;
        const nextExpert = {
          ...normalized,
          role: selection.role || normalized.role,
          mode: selection.mode,
          prompt: selection.prompt || normalized.prompt,
          systemPrompt: undefined,
          avatarSrc: selection.avatarSrc || normalized.avatarSrc,
          members: selection.members || normalized.members,
          selectedAt,
        };
        setRecentComposerExperts((current) => rememberRecentExpertSelection(current, nextExpert));
        invalidateExpertTeam(targetTabId);
        if (activeTabIdRef.current === targetTabId) {
          activationAppliedToCurrentTab = true;
          setActiveExpert(nextExpert);
          // The tab-scoped refresh clears the pending marker only after the
          // current view confirms this expert. An obsolete refresh may resolve
          // normally during a new-task handoff and must not hide the optimistic
          // composer identity.
          void refreshExpertState(targetTabId);
        } else if (pendingExpertActivationRef.current?.requestSequence === request.requestSequence) {
          pendingExpertActivationRef.current = null;
        }
      } catch (err) {
        if (request.requestSequence !== expertActivationSequenceRef.current) return false;
        if (pendingExpertActivationRef.current?.requestSequence === request.requestSequence) {
          pendingExpertActivationRef.current = null;
        }
        showError("experts.activate", err, "启用专家失败，请稍后重试。");
        return false;
      }
    } else {
      const nextExpert = { ...selection, selectedAt };
      setActiveExpert(nextExpert);
      setRecentComposerExperts((current) => rememberRecentExpertSelection(current, nextExpert));
    }
    if (selection.prompt.trim() && activationAppliedToCurrentTab) {
      setComposerInsertRequest({ id: Date.now(), text: selection.prompt, mode: "replace" });
    }
    return true;
  }, [activatableExpertIds, enqueueExpertMutation, expertCatalog, invalidateExpertTeam, refreshExpertState, showError, userExpertCatalog, waitForExpertTargetReady]);

  const optimisticExpertSelection = useCallback((selection: ExpertSelection) => {
    const nextExpert = { ...selection, systemPrompt: undefined, selectedAt: Date.now() };
    setWorkspaceView("chat");
    setActiveExpert(nextExpert);
    setRecentComposerExperts((current) => rememberRecentExpertSelection(current, nextExpert));
  }, []);

  const summonExpertToNewTask = useCallback(async (selection: ExpertSelection) => {
    closeTransientOverlays();
    setSidebarImDetailConnectionId("");
    const previousExpert = activeExpert;
    let request: PendingExpertActivation | undefined;
    if (activatableExpertIds.has(selection.id)) {
      request = {
        expertId: selection.id,
        tabId: "",
        requestSequence: ++expertActivationSequenceRef.current,
      };
      pendingExpertActivationRef.current = request;
    }
    optimisticExpertSelection(selection);
    try {
      const target = expertNewTaskTarget();
      const targetTabId = await openBlankSession(target.scope, target.workspaceRoot);
      if (!targetTabId) {
        if (pendingExpertActivationRef.current?.requestSequence === request?.requestSequence) {
          pendingExpertActivationRef.current = null;
        }
        setActiveExpert(previousExpert);
        return;
      }
      if (request && request.requestSequence !== expertActivationSequenceRef.current) return;
      const activated = await activateExpertSelection(selection, targetTabId, request);
      if (!activated && (!request || request.requestSequence === expertActivationSequenceRef.current)) {
        setActiveExpert(previousExpert);
      }
    } catch (err) {
      if (request && request.requestSequence !== expertActivationSequenceRef.current) return;
      if (pendingExpertActivationRef.current?.requestSequence === request?.requestSequence) {
        pendingExpertActivationRef.current = null;
      }
      setActiveExpert(previousExpert);
      showError("experts.switch", err, "切换专家失败，请稍后重试。");
      return;
    }
  }, [activateExpertSelection, activeExpert, activatableExpertIds, closeTransientOverlays, expertNewTaskTarget, openBlankSession, optimisticExpertSelection, showError]);

  const activateExpertForCurrentTaskOrNewTask = useCallback(async (selection: ExpertSelection) => {
    const targetTabId = activeTabIdRef.current;
    if (targetTabId) {
      const current = await app.CurrentExpertForTab(targetTabId).catch(() => null);
      if (current && (current.canSwitch || current.selectedId === selection.id)) {
        await activateExpertSelection(selection, targetTabId);
        return;
      }
      const projectAdapter = projectComposerAdapterForTab(projectComposerAdaptersRef.current, targetTabId);
      if (current && !current.selectedId && projectAdapter) {
        await activateExpertSelection(selection, targetTabId, undefined, "chat", "attach");
        return;
      }
    }
    await summonExpertToNewTask(selection);
  }, [activateExpertSelection, summonExpertToNewTask]);

  const handleSummonExpert = useCallback(async (selection: ExpertSelection) => {
    const useCurrentTask = expertDirectoryUsesCurrentTaskRef.current;
    expertDirectoryUsesCurrentTaskRef.current = false;
    closeTransientOverlays();
    setSidebarImDetailConnectionId("");
    if (useCurrentTask) {
      await activateExpertForCurrentTaskOrNewTask(selection);
      return;
    }
    await summonExpertToNewTask(selection);
  }, [activateExpertForCurrentTaskOrNewTask, closeTransientOverlays, summonExpertToNewTask]);

  const handleSelectRecentExpert = useCallback(async (selection: ExpertSelection) => {
    closeTransientOverlays();
    if (activeExpert?.mode === "team") {
      setExpertTeamSwitchConfirm({ kind: "recent", selection });
      return;
    }
    await activateExpertForCurrentTaskOrNewTask(selection);
  }, [activeExpert?.mode, activateExpertForCurrentTaskOrNewTask, closeTransientOverlays]);

  const clearExpertSelectionForTab = useCallback((targetTabId: string, adapter?: ProjectComposerAdapter): Promise<boolean> => {
    if (clearExpertErrorToastOpenRef.current) return Promise.resolve(false);
    const requestSequence = ++expertActivationSequenceRef.current;
    const previousExpert = adapter ? adapter.activeExpert ?? null : activeExpert;
    const sessionPath = tabMetasRef.current.find((tab) => tab.id === targetTabId)?.sessionPath;
    const targetIsCurrent = () => requestSequence === expertActivationSequenceRef.current
      && tabMetasRef.current.find((tab) => tab.id === targetTabId)?.sessionPath === sessionPath
      && (!adapter || projectComposerAdaptersRef.current.get(adapter.key) === adapter);
    pendingExpertActivationRef.current = { expertId: "", tabId: targetTabId, requestSequence };
    publishExpertSelection(targetTabId, null);
    if (adapter) {
      adapter.expertRequestSequence = requestSequence;
      adapter.activeExpert = undefined;
      setProjectComposerRevision((value) => value + 1);
    }
    const operation = (async () => {
      try {
        if (adapter?.tabPreparation) await adapter.tabPreparation;
        if (targetTabId) {
          await enqueueExpertMutation(() => app.ClearExpertForTab(targetTabId), targetTabId);
          invalidateExpertTeam(targetTabId);
        }
        if (targetIsCurrent()) publishExpertSelection(targetTabId, null);
        return true;
      } catch (err) {
        if (targetIsCurrent()) {
          publishExpertSelection(targetTabId, previousExpert);
          if (adapter) {
            adapter.activeExpert = previousExpert ?? undefined;
            setProjectComposerRevision((value) => value + 1);
          }
          clearExpertErrorToastOpenRef.current = true;
          showError("experts.clear", err, "退出专家模式失败，请稍后重试。", {
            onDismiss: () => { clearExpertErrorToastOpenRef.current = false; },
          });
        }
        return false;
      } finally {
        if (pendingExpertActivationRef.current?.requestSequence === requestSequence) {
          pendingExpertActivationRef.current = null;
          if (targetTabId && targetIsCurrent()) void refreshExpertState(targetTabId);
        }
      }
    })();
    if (targetTabId) expertMutationResultsRef.current.set(targetTabId, { sessionPath, result: operation });
    if (adapter) adapter.expertMutation = operation;
    return operation;
  }, [activeExpert, enqueueExpertMutation, invalidateExpertTeam, publishExpertSelection, refreshExpertState, showError]);

  const clearActiveExpertSelection = useCallback(async () => {
    closeTransientOverlays();
    const targetTabId = activeTabIdRef.current;
    if (targetTabId) await clearExpertSelectionForTab(targetTabId);
  }, [clearExpertSelectionForTab, closeTransientOverlays]);

  const handleComposerOpenExpertDirectory = useCallback(() => {
    closeTransientOverlays();
    if (displayedActiveExpert?.mode === "team") {
      setExpertTeamSwitchConfirm({ kind: "directory" });
      return;
    }
    const projectAdapter = projectComposerAdapterForTab(projectComposerAdaptersRef.current, activeTabIdRef.current);
    if (!displayedActiveExpert && projectAdapter) {
      setProjectCapabilityOrigin({
        sessionKey: projectAdapter.key,
        projectId: projectAdapter.projectId,
        kind: "expert",
        onCapabilitiesChanged: (items) => {
          projectAdapter.availableExperts = projectExpertSelections(items, expertCatalog, userExpertCatalog);
          setProjectComposerRevision((revision) => revision + 1);
        },
      });
      setRestoreProjectCapabilityFocus(null);
      return;
    }
    expertDirectoryUsesCurrentTaskRef.current = Boolean(displayedActiveExpert);
    openExpertDirectory();
  }, [closeTransientOverlays, displayedActiveExpert, expertCatalog, openExpertDirectory, userExpertCatalog]);

  const cancelExpertTeamSwitch = useCallback(() => {
    setExpertTeamSwitchConfirm(null);
  }, []);

  const confirmExpertTeamSwitch = useCallback(async () => {
    const pending = expertTeamSwitchConfirm;
    if (!pending) return;
    setExpertTeamSwitchConfirm(null);
    if (pending.kind === "recent") {
      await summonExpertToNewTask(pending.selection);
      return;
    }
    try {
      const target = expertNewTaskTarget();
      const opened = await openBlankSession(target.scope, target.workspaceRoot);
      if (!opened) return;
    } catch (err) {
      showError("experts.newTask", err, "创建专家任务失败，请稍后重试。");
      return;
    }
    setActiveExpert(null);
    expertDirectoryUsesCurrentTaskRef.current = true;
    openExpertDirectory();
  }, [expertNewTaskTarget, expertTeamSwitchConfirm, openBlankSession, openExpertDirectory, showError, summonExpertToNewTask]);

  useEffect(() => {
    if (!expertTeamSwitchConfirm) return;
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        cancelExpertTeamSwitch();
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        void confirmExpertTeamSwitch();
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [cancelExpertTeamSwitch, confirmExpertTeamSwitch, expertTeamSwitchConfirm]);

  useEffect(() => {
    void refreshTabMetas();
    const id = window.setInterval(() => void refreshTabMetas(), 2000);
    return () => window.clearInterval(id);
  }, [refreshTabMetas]);

  useEffect(() => {
    const el = footerRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    let frame = 0;
    const update = () => {
      if (frame) window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(() => {
        frame = 0;
        const next = Math.round(el.getBoundingClientRect().height);
        if (Math.abs(footerHeightRef.current - next) < 2) return;
        footerHeightRef.current = next;
        setFooterHeight(next);
      });
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => {
      if (frame) window.cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, []);

  // Run the ambient engine only while the agent is generating.
  useEffect(() => {
    if (state.running && isGenerativeMusicEnabled()) {
      generativeMusic.start();
    } else {
      generativeMusic.stop();
    }
    return () => generativeMusic.stop();
  }, [state.running]);

  // playTokenNote no-ops unless the engine is running, so subscribe unconditionally.
  useEffect(() => {
    const unsub = onEvent((e) => {
      if (e.kind === "text" || e.kind === "reasoning" || e.kind === "tool_dispatch") {
        generativeMusic.playTokenNote();
      }
    });
    return unsub;
  }, []);

  const toggleSidebar = useCallback(() => {
    closeTransientOverlays();
    pulseSidebarToggle();
    anchorAppScrollToChat();
    const nextCollapsed = !sidebarCollapsed;
    const floatingBreakpoint = sidebarCreation ? CREATION_SIDEBAR_FLOAT_BREAKPOINT : SIDEBAR_FLOAT_BREAKPOINT;
    const temporaryCreationOverlay = sidebarCreation && viewportWidth <= CREATION_SIDEBAR_FLOAT_BREAKPOINT;
    if (nextCollapsed) {
      sidebarManualExpandedRef.current = false;
    } else if (viewportWidth <= floatingBreakpoint) {
      sidebarManualExpandedRef.current = true;
    }
    setSidebarCollapsed(nextCollapsed);
    if (!temporaryCreationOverlay) saveSidebarCollapsed(nextCollapsed);
  }, [anchorAppScrollToChat, closeTransientOverlays, pulseSidebarToggle, sidebarCollapsed, sidebarCreation, viewportWidth]);

  const sidebarWidthClamp = desktopLayoutStyle === "creation" ? clampCreationSidebarWidth : clampSidebarWidth;
  const sidebarRenderWidth = liveSidebarWidth ?? sidebarWidth;
  const sidebarResizeMinWidth = desktopLayoutStyle === "creation" ? CREATION_SIDEBAR_MIN_WIDTH : SIDEBAR_MIN_WIDTH;

  useEffect(() => {
    if (desktopLayoutStyle === "creation" || sidebarWidth >= SIDEBAR_MIN_WIDTH) return;
    setSidebarWidth(SIDEBAR_MIN_WIDTH);
    saveSidebarWidth(SIDEBAR_MIN_WIDTH);
  }, [desktopLayoutStyle, sidebarWidth]);

  const setExpandedSidebarWidth = useCallback((width: number) => {
    closeTransientOverlays();
    const next = sidebarWidthClamp(width);
    setSidebarWidth(next);
    saveSidebarWidth(next);
  }, [closeTransientOverlays, sidebarWidthClamp]);

  const startSidebarResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (sidebarCollapsed) return;
      const layout = layoutRef.current;
      if (!layout) return;
      event.preventDefault();
      closeTransientOverlays();
      setSidebarResizing(true);
      let nextWidth = sidebarWidth;
      const liveResize = createRafResizeUpdater({
        target: layout,
        separator: event.currentTarget,
        cssVar: "--sidebar-expanded-width",
        onApply: setLiveSidebarWidth,
      });
      const dockLiveResize = createRafResizeUpdater({
        target: layout,
        cssVar: "--workspace-width",
        onApply: setLiveWorkspacePanelRenderWidth,
      });
      const onMove = (moveEvent: PointerEvent) => {
        nextWidth = sidebarWidthClamp(moveEvent.clientX);
        liveResize.schedule(nextWidth);
        dockLiveResize.schedule(resolveLiveWorkspacePanelRenderWidth(preferredWorkspacePanelWidth, nextWidth));
      };
      const onDone = () => {
        liveResize.flush();
        dockLiveResize.flush();
        setSidebarWidth(nextWidth);
        saveSidebarWidth(nextWidth);
        setLiveSidebarWidth(null);
        setLiveWorkspacePanelRenderWidth(null);
        setSidebarResizing(false);
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onDone);
        window.removeEventListener("pointercancel", onDone);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onDone);
      window.addEventListener("pointercancel", onDone);
    },
    [closeTransientOverlays, preferredWorkspacePanelWidth, resolveLiveWorkspacePanelRenderWidth, sidebarCollapsed, sidebarWidth, sidebarWidthClamp],
  );

  const resizeSidebarWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (sidebarCollapsed) return;
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setExpandedSidebarWidth(sidebarWidth + (event.key === "ArrowRight" ? 16 : -16));
      } else if (event.key === "Home") {
        event.preventDefault();
        setExpandedSidebarWidth(sidebarResizeMinWidth);
      } else if (event.key === "End") {
        event.preventDefault();
        setExpandedSidebarWidth(SIDEBAR_MAX_WIDTH);
      }
    },
    [setExpandedSidebarWidth, sidebarCollapsed, sidebarWidth, sidebarResizeMinWidth],
  );

  const setSavedWorkspacePanelWidth = useCallback(
    (width: number) => {
      closeTransientOverlays();
      if (rightDockDetailActive) {
        const next = clampRightDockPreviewWidth(width);
        setRightDockPreviewWidth(next);
        saveRightDockPreviewWidth(next);
        return;
      }
      const next = clampRightDockTreeWidth(width);
      setRightDockTreeWidth(next);
      saveRightDockTreeWidth(next);
    },
    [closeTransientOverlays, rightDockDetailActive],
  );

  const ensureWorkspacePanelWidth = useCallback(
    (width: number) => {
      closeTransientOverlays();
      if (rightDockMode === "context") return;
      const next = clampRightDockPreviewWidth(width);
      setRightDockPreviewWidth(next);
      saveRightDockPreviewWidth(next);
    },
    [closeTransientOverlays, rightDockMode],
  );

  const startWorkspacePanelResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!workspacePanelOpen) return;
      const layout = layoutRef.current;
      if (!layout) return;
      event.preventDefault();
      closeTransientOverlays();
      setWorkspacePanelResizing(true);
      const startX = event.clientX;
      const startDockWidth = workspacePanelRenderWidth;
      let nextDockWidth = startDockWidth;
      const liveResize = createRafResizeUpdater({
        target: layout,
        separator: event.currentTarget,
        cssVar: "--workspace-width",
        onApply: setLiveWorkspacePanelRenderWidth,
      });
      const onMove = (moveEvent: PointerEvent) => {
        const delta = moveEvent.clientX - startX;
        nextDockWidth = startDockWidth - delta;
        if (rightDockDetailActive) {
          nextDockWidth = clampRightDockPreviewWidth(nextDockWidth);
        } else {
          nextDockWidth = clampRightDockTreeWidth(nextDockWidth);
        }
        liveResize.schedule(resolveLiveWorkspacePanelRenderWidth(nextDockWidth));
      };
      const onDone = () => {
        liveResize.flush();
        setSavedWorkspacePanelWidth(nextDockWidth);
        setLiveWorkspacePanelRenderWidth(null);
        setWorkspacePanelResizing(false);
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onDone);
        window.removeEventListener("pointercancel", onDone);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onDone);
      window.addEventListener("pointercancel", onDone);
    },
    [closeTransientOverlays, resolveLiveWorkspacePanelRenderWidth, rightDockDetailActive, setSavedWorkspacePanelWidth, workspacePanelOpen, workspacePanelRenderWidth],
  );

  const resizeWorkspacePanelWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setSavedWorkspacePanelWidth(workspacePanelRenderWidth + (event.key === "ArrowLeft" ? 16 : -16));
      } else if (event.key === "Home") {
        event.preventDefault();
        setSavedWorkspacePanelWidth(rightDockDetailActive ? RIGHT_DOCK_PREVIEW_MIN_WIDTH : RIGHT_DOCK_TREE_MIN_WIDTH);
      } else if (event.key === "End") {
        event.preventDefault();
        setSavedWorkspacePanelWidth(rightDockDetailActive ? RIGHT_DOCK_MAX_WIDTH : RIGHT_DOCK_TREE_MAX_WIDTH);
      }
    },
    [rightDockDetailActive, setSavedWorkspacePanelWidth, workspacePanelRenderWidth],
  );

  const openWorkspacePanel = useCallback(
    (mode: RightDockMode = rightDockMode) => {
      closeTransientOverlays();
      updateWorkspacePanelState((current) => ({
        ...current,
        open: true,
        maximized: false,
        previewActive: mode === "context" || mode !== current.mode ? false : current.previewActive,
        mode,
      }));
    },
    [closeTransientOverlays, rightDockMode, updateWorkspacePanelState],
  );

  const closeWorkspacePanel = useCallback(() => {
    closeTransientOverlays();
    if (!workspacePanelOpen) {
      return;
    }
    setLiveWorkspacePanelRenderWidth(null);
    updateWorkspacePanelState((current) => ({
      ...current,
      open: false,
      maximized: false,
    }));
  }, [closeTransientOverlays, updateWorkspacePanelState, workspacePanelOpen]);

  const toggleWorkspacePanel = useCallback(() => {
    if (workspacePanelExpandDisabled) {
      return;
    }
    pulseWorkspaceToggle();
    if (workspacePanelRenderable) {
      closeWorkspacePanel();
      return;
    }
    openWorkspacePanel("files");
  }, [closeWorkspacePanel, openWorkspacePanel, pulseWorkspaceToggle, workspacePanelExpandDisabled, workspacePanelRenderable]);

  const openRightDockMode = useCallback(
    (mode: RightDockMode) => {
      openWorkspacePanel(mode);
    },
    [openWorkspacePanel],
  );

  const handleWorkspacePreviewModeChange = useCallback(
    (active: boolean) => {
      if (workspacePreviewActive === active) return;
      closeTransientOverlays();
      updateWorkspacePanelState((current) => ({
        ...current,
        previewActive: active,
      }));
    },
    [closeTransientOverlays, updateWorkspacePanelState, workspacePreviewActive],
  );

  const layoutStyle = useMemo(
    () =>
      ({
        "--sidebar-expanded-width": `${sidebarRenderWidth}px`,
        "--chat-min-width": `${chatReservedWidth}px`,
        "--workspace-width": `${workspacePanelRenderWidth}px`,
        "--workspace-resizer-width": `${WORKSPACE_RESIZER_WIDTH}px`,
      }) as CSSProperties,
    [chatReservedWidth, sidebarRenderWidth, workspacePanelRenderWidth],
  );

  const setWorkspacePanel = useCallback((open: boolean) => {
    if (open) {
      openWorkspacePanel();
    } else {
      closeWorkspacePanel();
    }
  }, [closeWorkspacePanel, openWorkspacePanel]);

  const addWorkspaceTextToComposer = useCallback((text: string) => {
    setComposerInsertRequest({ id: Date.now(), text });
  }, []);

  const handleTabChange = useCallback((id: string) => {
    const requestSeq = tabSwitchRequestSeqRef.current + 1;
    tabSwitchRequestSeqRef.current = requestSeq;
    closeTransientOverlays();
    showChatView(false);
    const selected = tabMetas.find((tab) => tab.id === id);
    setTabMetas((current) => current.map((tab) => ({ ...tab, active: tab.id === id })));
    void switchTab(id, selected).then(() => refreshTabMetas(
      () => tabSwitchRequestSeqRef.current === requestSeq,
    ));
    setTabRevealSignal((signal) => signal + 1);
  }, [closeTransientOverlays, refreshTabMetas, showChatView, switchTab, tabMetas]);

  const handleTabClose = useCallback(async (id: string) => {
    closeTransientOverlays();
    setComposerProfilesByTab((current) => {
      if (!(id in current)) return current;
      const next = { ...current };
      delete next[id];
      return next;
    });
    setTabMetas((current) => {
      if (current.length <= 1) return current;
      const closingIndex = current.findIndex((tab) => tab.id === id);
      if (closingIndex < 0) return current;
      const closingTab = current[closingIndex];
      const remaining = current.filter((tab) => tab.id !== id);
      if (!closingTab.active && closingTab.id !== activeTabId) return remaining;
      const nextIndex = Math.min(closingIndex, remaining.length - 1);
      const nextActiveId = remaining[nextIndex]?.id;
      return remaining.map((tab) => ({ ...tab, active: tab.id === nextActiveId }));
    });
    await closeTab(id);
    await refreshTabMetas();
    setTabRevealSignal((signal) => signal + 1);
  }, [activeTabId, closeTab, closeTransientOverlays, refreshTabMetas]);

  const handleTabsClose = useCallback(async (ids: string[], nextActiveTabId?: string) => {
    closeTransientOverlays();
    const currentIds = tabMetas.map((tab) => tab.id);
    const targets = ids.filter((id, index) => currentIds.includes(id) && ids.indexOf(id) === index);
    if (targets.length === 0) return;
    for (const id of targets) {
      await closeTab(id);
    }
    if (nextActiveTabId && currentIds.includes(nextActiveTabId)) {
      const selected = tabMetas.find((tab) => tab.id === nextActiveTabId);
      setTabMetas((current) => current.map((tab) => ({ ...tab, active: tab.id === nextActiveTabId })));
      void switchTab(nextActiveTabId, selected);
    }
    await refreshTabMetas();
    setTabRevealSignal((signal) => signal + 1);
  }, [closeTab, closeTransientOverlays, refreshTabMetas, switchTab, tabMetas]);

  const handleTabsReorder = useCallback(async (ids: string[]) => {
    setTabOrderIds(ids);
    setTabMetas((current) => {
      const byId = new Map(current.map((tab) => [tab.id, tab]));
      const ordered = ids.map((id) => byId.get(id)).filter((tab): tab is TabMeta => Boolean(tab));
      return ordered.length === current.length ? ordered : current;
    });
    await reorderTabs(ids);
    await refreshTabMetas();
    setTabRevealSignal((signal) => signal + 1);
  }, [refreshTabMetas, reorderTabs]);

  const handleNewTab = useCallback(async () => {
    closeTransientOverlays();
    closeNarrowSidebar();
    setSidebarImDetailConnectionId("");
    showChatView(true);
    const target = blankSessionTarget();
    await openBlankSession(target.scope, target.workspaceRoot, true);
  }, [blankSessionTarget, closeNarrowSidebar, closeTransientOverlays, openBlankSession, showChatView]);

  const handleSuperpowersEnabledChange = useCallback((enabled: boolean) => {
    setSuperpowersEnabled(enabled);
    setSuperpowersEnabledState(enabled);
    setSettingsFocus(null);
    setSettingsTarget(null);
    void handleNewTab();
  }, [handleNewTab, setSettingsFocus, setSettingsTarget]);

  const [rewindSignal, setRewindSignal] = useState(0);

  // ── Optimistic rewind ─────────────────────────────────────────────────
  // Rewind is optimistic: the UI immediately truncates, scrolls to the
  // target, fills the composer, and shows an undo banner.  The real Go
  // Rewind is deferred until the user SENDS a new message.  Undo simply
  // restores the full items list — no Go call needed.
  type RewindState = {
    turn: number;
    scope: string;
    fullItems: Item[];     // pre-truncation items (for undo)
    boundaryIdx: number;   // first item index of the rewound-to turn
    turnDiff: number;      // turns rolled back
    prompt: string;        // user message text for composer fill
  };
  const [rewindState, setRewindState] = useState<RewindState | null>(null);
  const [rewindCommitting, setRewindCommitting] = useState(false);
  const rewindStateRef = useRef(rewindState);
  rewindStateRef.current = rewindState;

  const hydratePlaceholderActive = Boolean(
    activeTabId &&
    !surfaceTransitionActive &&
    state.hydratePlaceholderTabId === activeTabId &&
    state.surfaceIntent !== "existing-session" &&
    state.hydrating &&
    state.items.length === 0 &&
    state.hydratePlaceholderItems?.length,
  );
  const transcriptHydrating = state.hydrating && !state.hydrateHistoryLoaded;
  const showWelcomeDuringHydration = blankSessionPresentation
    && transcriptHydrating
    && !state.hydrateError;
  const transcriptItems = blankSessionHistoryPending
    ? []
    : hydratePlaceholderActive
      ? state.hydratePlaceholderItems!
      : state.items;
  const transcriptFileTabId = surfaceTransitionActive
    ? undefined
    : hydratePlaceholderActive
      ? state.hydratePlaceholderTabId
      : activeTabId;

  // Display items: truncated when an optimistic rewind is pending.
  const displayItems = useMemo(() => {
    if (!rewindState) return transcriptItems;
    return transcriptItems.slice(0, rewindState.boundaryIdx).filter((it) => it.kind !== "compaction");
  }, [transcriptItems, rewindState]);

  // send wrapper: commits any pending optimistic rewind before sending.
  const commitThenSend = useCallback(async (displayText: string, submitText?: string, capabilities?: ComposerCapabilitySubmit) => {
    if (activeTab?.readOnly) throw new Error("channel session is read-only");
    if (!controllerReady || existingSessionSubmitBlocked) throw new Error("workspace is still starting");
    if (subscriptionModelInvocationBlocked) throw new Error("current model is unavailable");
    const rs = rewindStateRef.current;
    if (rs) {
      rewindStateRef.current = null;
      setRewindState(null);
      setRewindCommitting(true);
      let ok = false;
      try {
        ok = await rewind(rs.turn, rs.scope);
      } finally {
        setRewindCommitting(false);
      }
      if (!ok) {
        // Rewind failed: the Go conversation is intact. Do not send; the
        // controller emits a notice with the reason.
        setRewindState(null);
        throw new Error("rewind failed");
      }
      setRewindSignal((v) => v + 1);
      if (rs.scope === "both") {
        // Code was only reverted now (deferred), so refresh the dock here.
        setDockRefreshKey((v) => v + 1);
        setProjectRevision((v) => v + 1);
      }
    }
    await send(displayText, submitText, capabilities);
  }, [activeTab?.readOnly, controllerReady, existingSessionSubmitBlocked, send, rewind, subscriptionModelInvocationBlocked]);

  const handleTranscriptPrompt = useCallback((text: string) => {
    if (!controllerReady || existingSessionSubmitBlocked || subscriptionModelInvocationBlocked) return;
    void commitThenSend(text).catch((err) => {
      console.warn("Failed to submit transcript prompt", err);
    });
  }, [commitThenSend, controllerReady, existingSessionSubmitBlocked, subscriptionModelInvocationBlocked]);

  const handleCreationScenario = useCallback(async (text: string, skill?: CreationScenarioSkill) => {
    if (activeTab?.readOnly) return;
    if (skill?.name) {
      try {
        const settings = await app.SkillsSettings();
        const matchedSkill = findCreationScenarioSkill(skill, settings.skills ?? []);
        if (!matchedSkill) {
          setComposerInsertRequest({ id: Date.now(), text, mode: "replace" });
          showToast(t("welcome.creation.skillNotFound", { label: skill.label }), "warn");
          return;
        }
        if (!matchedSkill.enabled) {
          await app.SetSkillEnabled(matchedSkill.name, true);
          await app.RefreshSkills();
        }
        setComposerInsertRequest({
          id: Date.now(),
          text: `${formatSkillToken(matchedSkill.name)} `,
          kind: "skill",
          skillName: matchedSkill.name,
          skillLabel: skill.label,
        });
      } catch (err) {
        setComposerInsertRequest({ id: Date.now(), text, mode: "replace" });
        showError("creation.skill.resolve", err, t("welcome.creation.skillResolveFailed", { label: skill.label }));
      }
      return;
    }
    setComposerInsertRequest({ id: Date.now(), text, mode: "replace" });
  }, [activeTab?.readOnly, showError, showToast, t]);
  const handleSelectScene = useCallback(async (scene: Scene): Promise<void> => {
    if (!activeTab || activeTab.readOnly) return;
    if (!controllerReady) {
      setDraftScenesByTab((current) => ({ ...current, [activeTab.id]: scene }));
      return;
    }
    try {
      const updated = await app.SetSceneForTab(activeTab.id, scene);
      seedActiveTabMeta(updated);
    } catch (err) {
      showError("scene.save", err, "保存场景设置失败，请稍后重试。");
    }
  }, [activeTab, controllerReady, seedActiveTabMeta, showError]);
  useEffect(() => {
    if (!controllerReady || !activeTab?.id) return;
    const pendingScene = draftScenesByTab[activeTab.id];
    if (!pendingScene) return;
    let cancelled = false;
    void app.SetSceneForTab(activeTab.id, pendingScene)
      .then((updated) => {
        if (cancelled) return;
        seedActiveTabMeta(updated);
        setDraftScenesByTab((current) => {
          if (current[activeTab.id] !== pendingScene) return current;
          const next = { ...current };
          delete next[activeTab.id];
          return next;
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setDraftScenesByTab((current) => {
          if (current[activeTab.id] !== pendingScene) return current;
          const next = { ...current };
          delete next[activeTab.id];
          return next;
        });
        showError("scene.restore", err, "恢复场景设置失败，请稍后重试。");
      });
    return () => {
      cancelled = true;
    };
  }, [activeTab?.id, controllerReady, draftScenesByTab, seedActiveTabMeta, showError]);
  commitThenSendRef.current = commitThenSend;

  const handleMessageAction = useCallback((turn: number, scope: string) => {
    if (activeTab?.readOnly) return;
    if (hydratePlaceholderActive) return;
    if (subscriptionModelInvocationBlocked && scope !== "code") return;
    if (scope === "fork") {
      // Fork still goes through the controller (not optimistic).
      rewind(turn, scope).then((ok) => {
        if (!ok) return;
        refreshTabMetas();
        setProjectRevision((v) => v + 1);
      });
      return;
    }

    // Code-only rewind only affects files — no message truncation,
    // no optimistic UI needed.  Execute immediately.
    if (scope === "code") {
      rewind(turn, scope).then((ok) => {
        if (!ok) return;
        setDockRefreshKey((v) => v + 1);
        setProjectRevision((v) => v + 1);
      });
      return;
    }

    // Summarize only compresses the conversation log — no files touched,
    // no optimistic UI needed. Execute immediately like code-only rewind.
    if (scope === "summ-from" || scope === "summ-upto") {
      rewind(turn, scope).then((ok) => {
        if (!ok) return;
        setDockRefreshKey((v) => v + 1);
        setProjectRevision((v) => v + 1);
      });
      return;
    }

    const items = state.items;
    // Find the boundary: index of the Nth user message where N = turn.
    let boundaryIdx = 0;
    let userCount = 0;
    for (let i = 0; i < items.length; i++) {
      if (items[i].kind === "user") {
        if (userCount === turn) { boundaryIdx = i; break; }
        userCount++;
      }
    }

    const prevUserCount = items.filter((it) => it.kind === "user").length;
    const turnDiff = prevUserCount - userCount;

    // Save full items for undo.
    const userItem = items[boundaryIdx]?.kind === "user" ? items[boundaryIdx] as Extract<Item, { kind: "user" }> : undefined;
    const prompt = userItem?.text ?? "";
    setRewindState({
      turn,
      scope,
      fullItems: items,
      boundaryIdx,
      turnDiff,
      prompt,
    });

    // Fill composer with the rewound-to user message.
    const insertId = Date.now();
    setComposerInsertRequest({ id: insertId, text: prompt, mode: "replace" });

    setRewindSignal((v) => v + 1);
  }, [activeTab?.readOnly, hydratePlaceholderActive, state.items, rewind, refreshTabMetas, setComposerInsertRequest, subscriptionModelInvocationBlocked]);

  const handleEditPrompt = useCallback(async (turn: number, displayText: string, submitText?: string): Promise<boolean> => {
    const sourceTabId = activeTabId;
    if (!sourceTabId || activeTab?.readOnly || !controllerReady || subscriptionModelInvocationBlocked || hydratePlaceholderActive || rewindStateRef.current || state.running || state.messageAction != null || state.approval != null || state.ask != null || clearContextPending) return false;
    const next = displayText.trim();
    if (!next) return false;
    const submit = (submitText ?? displayText).trim();
    const ok = await rewind(turn, "conversation");
    if (!ok) return false;
    setRewindSignal((v) => v + 1);
    try {
      await sendToTab(sourceTabId, next, submit);
      return true;
    } catch {
      return false;
    }
  }, [activeTab?.readOnly, activeTabId, clearContextPending, controllerReady, hydratePlaceholderActive, sendToTab, state.approval, state.ask, state.messageAction, state.running, subscriptionModelInvocationBlocked, rewind]);

  const runOpenTopicRequest = useCallback(async (request: PendingOpenTopicRequest) => {
    try {
      let openedTab: TabMeta;
      if (singleSurfaceLayout) {
        openedTab = await activateTopic(
          request.scope,
          request.workspaceRoot,
          request.topicId,
          request.sessionPath || "",
          request.transitionId,
          request.sessionTurns,
          request.scene,
          request.sceneState,
          request.composerIdentity,
        );
      } else if (request.sessionPath) {
        openedTab = await openTopicSession(request.scope, request.workspaceRoot, request.topicId, request.sessionPath);
      } else if (request.scope === "global") {
        openedTab = await openGlobalTab(request.topicId);
      } else {
        openedTab = await openProjectTab(request.workspaceRoot, request.topicId);
      }
      if (request.seq !== openTopicSeqRef.current) return;
      seedActiveTabMeta(openedTab);
      // Fire refreshTabMetas in background — transcript data loads independently.
      void refreshTabMetas(() => request.seq === openTopicSeqRef.current);
      setTabRevealSignal((signal) => signal + 1);
      setTranscriptRevealSignal((signal) => signal + 1);
      if (request.capability?.capabilityId === "crossborder-commerce") {
        setSuperpowerEntry((current) => ({ screen: "ecommerce", key: current.key + 1 }));
        setWorkspaceView("superpowers");
      }
    } catch (err) {
      if (request.seq !== openTopicSeqRef.current) return;
      console.warn("topic open failed", err);
      showToast(t("history.failedOpenSession"), "error");
      void refreshTabMetas(() => request.seq === openTopicSeqRef.current);
    }
  }, [activateTopic, openGlobalTab, openProjectTab, openTopicSession, refreshTabMetas, seedActiveTabMeta, showToast, singleSurfaceLayout, t]);
  const beginProjectTreeTopicOpenIntent = useCallback((
    scope: string,
    workspaceRoot: string,
    topicId: string,
    sessionPath?: string,
    sessionTurns?: number,
    scene?: Scene,
    sceneState?: SceneState,
    composerIdentity?: ComposerSessionIdentity,
    _capability?: CapabilityTaskScope,
  ): number | undefined => {
    if (!singleSurfaceLayout) return undefined;
    supersedeBlankSessionRequest();
    supersedeOpenTopicRequests({ seqRef: openTopicSeqRef, runningRef: openTopicRunningRef, pendingRef: openTopicPendingRef });
    const navigationSeq = surfaceNavigationSeqRef.current + 1;
    surfaceNavigationSeqRef.current = navigationSeq;
    const token = navigationSeq;
    closeTransientOverlays();
    setSidebarImDetailConnectionId("");
    showChatView(false);
    const transitionId = sessionPath?.trim()
      ? beginSurfaceTransition({ scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity })
      : undefined;
    preparedTopicNavigationRef.current = { token, transitionId, navigationSeq };
    return token;
  }, [beginSurfaceTransition, closeTransientOverlays, showChatView, singleSurfaceLayout, supersedeBlankSessionRequest]);
  const cancelPreparedTopicNavigation = useCallback((token: number) => {
    const prepared = preparedTopicNavigationRef.current;
    if (prepared?.token !== token) return;
    preparedTopicNavigationRef.current = null;
    if (prepared.transitionId !== undefined) {
      cancelSurfaceTransition(prepared.transitionId);
    }
  }, [cancelSurfaceTransition]);
  const handleOpenTopic = useCallback((
    scope: string,
    workspaceRoot: string,
    topicId: string,
    sessionPath?: string,
    preparedOpenToken?: number,
    sessionTurns?: number,
    scene?: Scene,
    sceneState?: SceneState,
    composerIdentity?: ComposerSessionIdentity,
    capability?: CapabilityTaskScope,
  ): Promise<void> => {
    let preparedTransitionId: number | undefined;
    if (preparedOpenToken !== undefined) {
      const prepared = preparedTopicNavigationRef.current;
      if (
        prepared?.token !== preparedOpenToken
        || prepared.navigationSeq !== surfaceNavigationSeqRef.current
      ) {
        cancelPreparedTopicNavigation(preparedOpenToken);
        return Promise.resolve();
      }
      preparedTopicNavigationRef.current = null;
      preparedTransitionId = prepared.transitionId;
    } else {
      supersedeBlankSessionRequest();
      surfaceNavigationSeqRef.current += 1;
    }
    closeTransientOverlays();
    closeNarrowSidebar();
    setSidebarImDetailConnectionId("");
    showChatView(false);
    const transitionId = singleSurfaceLayout
      ? preparedTransitionId ?? beginSurfaceTransition({ scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity })
      : undefined;
    return enqueueOpenTopicRequest(
      { seqRef: openTopicSeqRef, runningRef: openTopicRunningRef, pendingRef: openTopicPendingRef },
      { scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity, capability, transitionId },
      runOpenTopicRequest,
    );
  }, [beginSurfaceTransition, cancelPreparedTopicNavigation, closeNarrowSidebar, closeTransientOverlays, runOpenTopicRequest, showChatView, singleSurfaceLayout, supersedeBlankSessionRequest]);

  const openSidebarImConnectionSession = useCallback(async (connection: SidebarImConnection) => {
    const target = sidebarImSessionTarget(connection);
    if (!target) {
      showToast(t("sidebar.imWaiting", { name: connection.title }));
      return;
    }
    closeNarrowSidebar();
    setSidebarImDetailConnectionId("");
    showChatView(false);
    try {
      let openedTab: TabMeta | undefined;
      if (connection.sessionSource === "auto" && target.kind === "path") {
        const transitionId = singleSurfaceLayout
          ? beginSurfaceTransition({
            scope: connection.scope,
            workspaceRoot: connection.scope === "project" ? connection.workspaceRoot : "",
            topicId: "",
            sessionPath: target.value,
          })
          : undefined;
        const tab = singleSurfaceLayout
          ? await ensureBlankSurface(connection.scope, connection.scope === "project" ? connection.workspaceRoot : "", transitionId)
          : await ensureBlankTab(connection.scope, connection.scope === "project" ? connection.workspaceRoot : "");
        await openChannelSession(target.value, tab.id, transitionId);
        openedTab = { ...tab, sessionPath: target.value, readOnly: true };
      } else if (target.kind === "path") {
        const transitionId = singleSurfaceLayout
          ? beginSurfaceTransition({
            scope: connection.scope,
            workspaceRoot: connection.scope === "project" ? connection.workspaceRoot : "",
            topicId: "",
            sessionPath: target.value,
          })
          : undefined;
        const tab = singleSurfaceLayout
          ? await ensureBlankSurface(connection.scope, connection.scope === "project" ? connection.workspaceRoot : "", transitionId)
          : await ensureBlankTab(connection.scope, connection.scope === "project" ? connection.workspaceRoot : "");
        await resumeSession(target.value, tab.id, transitionId);
        openedTab = { ...tab, sessionPath: target.value, readOnly: false };
      } else if (connection.scope === "project") {
        openedTab = singleSurfaceLayout
          ? await activateTopic("project", connection.workspaceRoot, target.value)
          : await openProjectTab(connection.workspaceRoot, target.value);
      } else {
        openedTab = singleSurfaceLayout
          ? await activateTopic("global", "", target.value)
          : await openGlobalTab(target.value);
      }
      if (openedTab) seedActiveTabMeta(openedTab);
      await refreshTabMetas();
      setTabRevealSignal((value) => value + 1);
      setTranscriptRevealSignal((value) => value + 1);
      setProjectRevision((value) => value + 1);
    } catch (err) {
      console.warn("bot sidebar open failed", err);
      showToast(t("sidebar.imOpenFailed", { name: connection.title }));
    }
  }, [activateTopic, beginSurfaceTransition, closeNarrowSidebar, ensureBlankSurface, ensureBlankTab, openChannelSession, openGlobalTab, openProjectTab, refreshTabMetas, resumeSession, seedActiveTabMeta, showChatView, showToast, singleSurfaceLayout, t]);

  // The conversation center is profile-wide rather than owned by the active
  // tab. Both datasets are loaded together so the History / Trash tabs have
  // accurate counts and switching tabs is instant.
  const loadConversationCenterData = useCallback(async () => {
    const [sessions, trashedSessions] = await Promise.all([
      app.ListAllSessions().catch(() => []),
      app.ListAllTrashedSessions().catch(() => []),
    ]);
    return {
      sessions: asArray<SessionMeta>(sessions),
      trashedSessions: asArray<SessionMeta>(trashedSessions),
    };
  }, []);
  const openConversationCenter = useCallback(async (
    kind: HistoryPanelKind,
    initialWorkspaceRoot?: string,
  ) => {
    closeTransientOverlays();
    const data = await loadConversationCenterData();
    setHistView({ kind, ...data, initialWorkspaceRoot });
  }, [closeTransientOverlays, loadConversationCenterData]);
  const openProjectHistory = useCallback(async (scope: "global" | "project", workspaceRoot: string) => {
    await openConversationCenter("history", scope === "global" ? "" : workspaceRoot);
  }, [openConversationCenter]);
  const openAllHistory = useCallback(async () => {
    await openConversationCenter("history");
  }, [openConversationCenter]);
  const openTrash = useCallback(async () => {
    await openConversationCenter("trash");
  }, [openConversationCenter]);
  const closeHistory = useCallback(() => {
    closeTransientOverlays();
    setHistView(null);
  }, [closeTransientOverlays]);
  const refreshHistoryView = useCallback(async () => {
    if (!histViewOpenRef.current) return;
    const data = await loadConversationCenterData();
    setHistView((current) => current === null ? null : { ...current, ...data });
  }, [loadConversationCenterData]);

  const refreshSessionSurfaces = useCallback(async () => {
    if (refreshInFlightRef.current) {
      refreshQueuedRef.current = true;
      return;
    }
    refreshInFlightRef.current = true;
    try {
      do {
        refreshQueuedRef.current = false;
        setProjectRevision((value) => value + 1);
        const channelTarget = channelSessionRefreshTarget(activeTabRef.current);
        const channelHistoryRefresh = channelTarget
          ? revalidateChannelSession(channelTarget.tabId, channelTarget.sessionPath)
          : Promise.resolve();
        await Promise.allSettled([
          refreshTabMetas(),
          refreshHistoryView(),
          reloadSidebarImConnections(),
          channelHistoryRefresh,
        ]);
      } while (refreshQueuedRef.current);
    } finally {
      refreshInFlightRef.current = false;
    }
  }, [refreshHistoryView, refreshTabMetas, reloadSidebarImConnections, revalidateChannelSession]);

  const catchUpProjectTreeRevision = useCallback(async () => {
    const rev = await app.ProjectTreeRevision().catch(() => 0);
    if (rev <= lastSeenProjectTreeRevisionRef.current) return;
    lastSeenProjectTreeRevisionRef.current = rev;
    void refreshSessionSurfaces();
  }, [refreshSessionSurfaces]);

  useEffect(() => {
    return onProjectTreeChanged((rev) => {
      if (typeof rev === "number") {
        if (rev <= lastSeenProjectTreeRevisionRef.current) return;
        lastSeenProjectTreeRevisionRef.current = rev;
      }
      void refreshSessionSurfaces();
    });
  }, [refreshSessionSurfaces]);

  useEffect(() => {
    void catchUpProjectTreeRevision();
    const id = window.setInterval(() => {
      if (document.visibilityState === "hidden") return;
      void catchUpProjectTreeRevision();
    }, 10000);
    const onFocus = () => void catchUpProjectTreeRevision();
    const onVisibility = () => {
      if (document.visibilityState === "visible") void catchUpProjectTreeRevision();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.clearInterval(id);
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [catchUpProjectTreeRevision]);

  const onResumeSession = useCallback(
    async (session: SessionMeta) => {
      if (state.running && !singleSurfaceLayout) return;
      showChatView(false);
      const scope = session.scope || (session.workspaceRoot ? "project" : "global");
      const composerIdentity = composerSessionIdentityFromSource(session);
      try {
        let targetTab: TabMeta;
        if (isChannelSession(session)) {
          const targetScope = scope === "project" ? "project" : "global";
          const targetWorkspaceRoot = scope === "project" ? session.workspaceRoot || "" : "";
          const transitionId = singleSurfaceLayout
            ? beginSurfaceTransition({
              scope: targetScope,
              workspaceRoot: targetWorkspaceRoot,
              topicId: session.topicId || "",
              sessionPath: session.path,
              composerIdentity,
            })
            : undefined;
          targetTab = singleSurfaceLayout
            ? await ensureBlankSurface(targetScope, targetWorkspaceRoot, transitionId)
            : await ensureBlankTab(targetScope, targetWorkspaceRoot);
          await openChannelSession(session.path, targetTab.id, transitionId);
          targetTab = { ...targetTab, sessionPath: session.path, readOnly: true };
        } else if (scope === "project" && session.workspaceRoot && session.topicId) {
          targetTab = singleSurfaceLayout
            ? await activateTopic("project", session.workspaceRoot, session.topicId, session.path, undefined, undefined, undefined, undefined, composerIdentity)
            : await openProjectTab(session.workspaceRoot, session.topicId);
        } else if (scope === "global" && session.topicId) {
          targetTab = singleSurfaceLayout
            ? await activateTopic("global", "", session.topicId, session.path, undefined, undefined, undefined, undefined, composerIdentity)
            : await openGlobalTab(session.topicId);
        } else {
          throw new Error(scope === "global" && !session.topicId
            ? t("history.failedOpenSession")
            : (session.topicId ? "Missing workspaceRoot" : t("history.failedOpenSession")));
        }
        if (!isChannelSession(session) && !singleSurfaceLayout) {
          await resumeSession(session.path, targetTab.id);
        }
        if (!isChannelSession(session)) {
          targetTab = { ...targetTab, sessionPath: session.path, readOnly: false };
        }
        seedActiveTabMeta(targetTab);
        setHistView(null);
        // Fire refreshTabMetas in background — transcript data loads independently.
        void refreshTabMetas();
        setTabRevealSignal((value) => value + 1);
        setTranscriptRevealSignal((value) => value + 1);
      } catch (err: any) {
        await refreshHistoryView();
        if (isMissingSessionError(err)) return;
        setHistView(null);
        if (scope === "project" && session.workspaceRoot) {
          const name = workspaceDisplayName(session.workspaceRoot);
          showToast(t("history.failedOpenProject", { name, path: session.workspaceRoot }));
        } else {
          showError("history.openSession", err, "打开历史会话失败，请稍后重试。");
        }
      }
    },
    [activateTopic, beginSurfaceTransition, ensureBlankSurface, ensureBlankTab, openChannelSession, openGlobalTab, openProjectTab, refreshHistoryView, refreshTabMetas, state.running, resumeSession, seedActiveTabMeta, showChatView, showError, showToast, singleSurfaceLayout, t],
  );

  // Command palette: ⌘K / Ctrl+K opens a fuzzy navigator over commands and
  // recent sessions. Sessions are snapshotted on open so the list is stable
  // while the palette is up.
  const openPalette = useCallback(async () => {
    closeTransientOverlays();
    setPaletteOpen(true);
    setPaletteSessions(await listSessions().catch(() => []));
  }, [closeTransientOverlays, listSessions]);
  useGlobalShortcut("commandPalette.open", () => {
    if (sidebarCreation) {
      void openAllHistory();
      return;
    }
    setPaletteOpen((current) => {
      if (!current) void openPalette();
      return !current; // ← fix: toggle the state so the palette actually opens/closes
    });
  }, [openAllHistory, openPalette, sidebarCreation]);
  useGlobalShortcut("app.newSession", () => void handleNewTab(), [handleNewTab]);
  useGlobalShortcut("settings.open", () => {
    closeTransientOverlays();
    setSettingsTarget("general");
  }, [closeTransientOverlays]);
  useGlobalShortcut("shortcuts.show", () => setShortcutsOpen(true));
  useGlobalShortcut("sidebar.toggle", toggleSidebar, [toggleSidebar]);

  // --- Topic shortcut navigation (Cmd/Ctrl+1-9) ---
  const visibleTopicsRef = useRef<TopicShortcutEntry[]>([]);
  const handleVisibleTopicsChange = useCallback((topics: TopicShortcutEntry[]) => {
    visibleTopicsRef.current = topics;
  }, []);
  const handleNavigateTopic = useCallback((entry: TopicShortcutEntry) => {
    void handleOpenTopic(entry.scope, entry.workspaceRoot, entry.topicId, entry.sessionPath);
  }, [handleOpenTopic]);
  const { showBadges: showTopicBadges } = useTopicShortcuts(!sidebarCollapsed, desktopPlatform);

  // Register Cmd/Ctrl+1-9 shortcuts for topic navigation
  useEffect(() => {
    if (sidebarCollapsed) return;
    const onKeydown = (event: globalThis.KeyboardEvent) => {
      const idx = topicShortcutIndexFromEvent(event, desktopPlatform);
      if (idx === null) return;
      event.preventDefault();
      const topics = visibleTopicsRef.current;
      if (idx < topics.length) {
        handleNavigateTopic(topics[idx]);
      }
    };
    document.addEventListener("keydown", onKeydown);
    return () => document.removeEventListener("keydown", onKeydown);
  }, [sidebarCollapsed, desktopPlatform, handleNavigateTopic]);

  const paletteItems = useMemo<PaletteItem[]>(() => {
    const cmds: PaletteItem[] = [
      { id: "cmd-new", group: t("palette.group.commands"), title: t("palette.cmd.newSession"), icon: <SquarePen size={15} />, compact: true, keywords: ["new", "新建"], run: () => void handleNewTab() },
      { id: "cmd-history", group: t("palette.group.commands"), title: t("palette.cmd.history"), icon: <History size={15} />, compact: true, keywords: ["history", "历史"], run: () => void openAllHistory() },
      { id: "cmd-trash", group: t("palette.group.commands"), title: t("palette.cmd.trash"), icon: <Trash2 size={15} />, compact: true, keywords: ["trash", "回收站"], run: () => void openTrash() },
      { id: "cmd-settings", group: t("palette.group.commands"), title: t("palette.cmd.settings"), icon: <SettingsIcon size={15} />, compact: true, keywords: ["settings", "设置"], run: () => setSettingsTarget("general") },
      { id: "cmd-appearance", group: t("palette.group.commands"), title: t("palette.cmd.appearance"), icon: <Palette size={15} />, compact: true, keywords: ["theme", "appearance", "外观", "主题"], run: () => setSettingsTarget("appearance") },
      { id: "cmd-memory", group: t("palette.group.commands"), title: t("palette.cmd.memory"), icon: <Brain size={15} />, compact: true, keywords: ["memory", "记忆"], run: () => setSettingsTarget("memory") },
      { id: "cmd-models", group: t("palette.group.commands"), title: t("palette.cmd.models"), icon: <Cpu size={15} />, compact: true, keywords: ["model", "模型"], run: () => setSettingsTarget("models") },
    ];
    const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
    const dayLabel = (ms: number) => {
      const days = Math.round((startOfDay(new Date()) - startOfDay(new Date(ms))) / 86_400_000);
      if (days <= 0) return t("history.today");
      if (days === 1) return t("history.yesterday");
      return new Date(ms).toLocaleDateString();
    };
    const sessionItems: PaletteItem[] = paletteSessions.slice(0, 12).map((s) => ({
      id: `sess-${s.path}`,
      group: t("palette.group.sessions"),
      title: s.title?.trim() || s.preview || t("history.emptySession"),
      hint: s.workspaceRoot || undefined,
      meta: dayLabel(sessionActivityTime(s)),
      badge: t(s.turns === 1 ? "history.turnOne" : "history.turnOther", { n: s.turns }),
      run: () => void onResumeSession(s),
    }));
    return [...cmds, ...sessionItems];
  }, [t, paletteSessions, handleNewTab, openAllHistory, openTrash, onResumeSession]);
  // Delete / rename act on disk, then re-fetch so the panel reflects the change.
  const onDeleteSession = useCallback(
    async (path: string) => {
      if (state.running) return;
      try {
        await deleteSession(path);
        await refreshHistoryView();
      } catch {
        await refreshHistoryView();
      }
    },
    [state.running, deleteSession, refreshHistoryView],
  );
  const onRenameSession = useCallback(
    async (path: string, title: string) => {
      if (state.running) return;
      await renameSession(path, title);
      await refreshHistoryView();
    },
    [state.running, renameSession, refreshHistoryView],
  );
  const onRestoreTrashedSession = useCallback(
    async (path: string) => {
      await restoreSession(path);
      await refreshHistoryView();
    },
    [refreshHistoryView, restoreSession],
  );
  const onPurgeTrashedSession = useCallback(
    async (path: string) => {
      await purgeTrashedSession(path);
      updateConversationUnreadCompletion(
        conversationAttentionScope,
        conversationAttentionIdentity({ kind: "global_session", sessionPath: path }),
        null,
      );
      await refreshHistoryView();
    },
    [conversationAttentionScope, purgeTrashedSession, refreshHistoryView, updateConversationUnreadCompletion],
  );
  const onPurgeAllTrashedSessions = useCallback(
    async (paths: string[]) => {
      const uniquePaths = Array.from(new Set(paths));
      for (const path of uniquePaths) {
        await purgeTrashedSession(path);
        updateConversationUnreadCompletion(
          conversationAttentionScope,
          conversationAttentionIdentity({ kind: "global_session", sessionPath: path }),
          null,
        );
      }
      await refreshHistoryView();
    },
    [conversationAttentionScope, purgeTrashedSession, refreshHistoryView, updateConversationUnreadCompletion],
  );

  // Workspace: open the folder chooser and switch projects. The hook resets the
  // transcript and refreshes meta on a pick. A cancel is a no-op.
  const switchFolder = useCallback(async (path?: string) => {
    const picked = path === undefined ? await pickWorkspace() : await switchWorkspace(path);
    if (picked) {
      setProjectRevision((value) => value + 1);
      await refreshTabMetas();
    }
    return picked;
  }, [pickWorkspace, switchWorkspace, refreshTabMetas]);

  const refreshProjectsAndTabs = useCallback(async () => {
    setProjectRevision((value) => value + 1);
    const tabs = await refreshTabMetas();
    if (activeTabId && !tabs.some((tab) => tab.id === activeTabId)) {
      await syncActiveTab(true);
    }
  }, [activeTabId, refreshTabMetas, syncActiveTab]);

  const renameTopic = useCallback(async (topicId: string, title: string) => {
    const nextTitle = title.trim();
    if (!topicId || !nextTitle) return;
    try {
      await app.RenameTopic(topicId, nextTitle);
      await refreshProjectsAndTabs();
    } catch (err) {
      showError("topics.rename", err, "重命名任务失败，请稍后重试。");
    }
  }, [refreshProjectsAndTabs, showError]);

  const startActiveTopicRename = useCallback(() => {
    if (!activeTab?.topicId) return;
    topicRenameSkipCommitRef.current = false;
    topicRenameCommitHandledRef.current = false;
    setRenamingTopicId(activeTab.topicId);
    setTopicTitleDraft(activeTab.topicTitle || "");
  }, [activeTab?.topicId, activeTab?.topicTitle]);

  const cancelActiveTopicRename = useCallback(() => {
    topicRenameSkipCommitRef.current = true;
    topicRenameCommitHandledRef.current = true;
    setRenamingTopicId(null);
    setTopicTitleDraft("");
  }, []);

  const commitActiveTopicRename = useCallback(async () => {
    if (topicRenameSkipCommitRef.current) {
      topicRenameSkipCommitRef.current = false;
      topicRenameCommitHandledRef.current = false;
      setRenamingTopicId(null);
      return;
    }
    if (topicRenameCommitHandledRef.current) return;
    topicRenameCommitHandledRef.current = true;
    const topicId = renamingTopicId;
    setRenamingTopicId(null);
    if (!topicId) return;
    const nextTitle = topicTitleDraft.trim();
    if (!nextTitle) return;
    try {
      await renameTopic(topicId, nextTitle);
    } catch {
      /* keep the app usable if a stale topic cannot be renamed */
    }
  }, [renameTopic, renamingTopicId, topicTitleDraft]);

  const sidebarExpandBlocked = false;
  const sidebarToggleTitle = sidebarCollapsed
      ? t("sidebar.expand")
      : t("sidebar.collapse");
  const sidebarNavTooltipDisabled = !sidebarCollapsed;
  const browserPreviewChrome = typeof window !== "undefined" && !window.runtime;
  const workspacePanelResetWidth = rightDockDetailActive
    ? RIGHT_DOCK_PREVIEW_DEFAULT_WIDTH
    : defaultRightDockTreeWidth();
  const workspacePanelResizeMinWidth = workspacePanelAriaMinWidth(workspacePanelMinWidth, workspacePanelRenderWidth);
  const workspacePanelMaxWidth = rightDockDetailActive ? RIGHT_DOCK_MAX_WIDTH : RIGHT_DOCK_TREE_MAX_WIDTH;
  const sidebarAccountHasSession = hasRecoverableCloudIdentity(sidebarAccount);
  const sidebarAccountRequiresReauthentication = requiresReauthentication(sidebarAccount);
  const sidebarFeedbackHasIdentity = sidebarAccountHasSession
    && Boolean(sidebarAccount?.profile_id?.trim())
    && Boolean(sidebarAccount?.cloud_user_id?.trim())
    && (sidebarAccount?.binding_generation ?? 0) > 0;
  const feedbackAccountState: "loading" | "ready" | "anonymous" | "expired" | "unavailable" =
    !sidebarAccountInitialized
      ? "loading"
      : !sidebarAccountAvailable
        ? "unavailable"
        : sidebarAccountRequiresReauthentication
          ? "expired"
          : sidebarFeedbackHasIdentity
            ? "ready"
            : "anonymous";
  const feedbackAccountScopeKey = feedbackAccountState === "ready" ? sidebarAccountScopeKey : "";
  const sidebarAccountLabel = !sidebarAccountInitialized
    ? t("common.loading")
    : sidebarAccountRequiresReauthentication
      ? t("sidebar.reauthenticationRequired")
      : sidebarAccountHasSession
      ? sidebarAccount?.display_name?.trim() || t("account.nameUnavailable")
      : t("sidebar.signInOrRegister");
  const sidebarAccountPlanLabel = sidebarAccountPlan === "pro" ? "体验套餐" : sidebarAccountPlan;
  const topicbarVisible = workspaceView === "chat" || Boolean(sidebarImDetailConnection);
  const skipToComposerAvailable = workspaceView === "chat" && !sidebarImDetailConnection;
  const skipToProjectComposerAvailable = workspaceView === "projects" && Boolean(selectedCloudProjectId);
  const topicbarTitle = sidebarImDetailConnection
    ? t("botDetail.title", { name: sidebarImDetailConnection.title })
    : surfaceTransitionActive
      ? renderedViewTitle || t("transcript.loadingSession")
      : topicDisplayTitle(activeTab);
  const topicbarWorkspaceLabel = sidebarImDetailConnection ? t("botDetail.subtitle") : activeTab ? tabWorkspaceTitle(activeTab) : "";
  const topicbarWorkspacePath = activeTab?.scope === "project" ? activeTab.workspaceRoot || state.meta?.cwd : "";
  const topicbarImSource = activeTab?.scope === "global" && activeTab.topicId ? imTopicSources[activeTab.topicId] : undefined;
  const topicbarImSourceLabel = sidebarImDetailConnection
    ? sidebarImDetailConnection.platformLabel
    : topicbarImSource ? t("msg.fromIm", { source: topicbarImSource.label }) : "";
  const topicbarImSourcePlatform = sidebarImDetailConnection?.platform ?? topicbarImSource?.platform;
  const topicbarSubtitleVisible = !sidebarCreation && Boolean(topicbarWorkspaceLabel || topicbarImSourceLabel);
  const topicbarSubtitleTitle = sidebarImDetailConnection
    ? [topicbarWorkspaceLabel, topicbarImSourceLabel, sidebarImScopeLabel(sidebarImDetailConnection, t)].filter(Boolean).join(" · ")
    : [topicbarWorkspacePath || topicbarWorkspaceLabel, topicbarImSourceLabel].filter(Boolean).join(" · ");
  const topicbarCanRename = !sidebarImDetailConnection && Boolean(activeTab?.topicId);
  const hideCreationTopicTitle = sidebarCreation
    && !sidebarImDetailConnection
    && (
      pendingBlankSessionRequest !== null
      || (blankSessionPresentation && sessionTurns === 0)
    );
  const topicbarTitleEditSize = Math.min(56, Math.max(4, topicTitleDraft.length || topicbarTitle.length || 1));
  const sidebarWorkbench = desktopLayoutStyle === "workbench";
  // Creation keeps the classic sidebar/chat structure while gating chrome tweaks
  // behind its own style flag so classic/workbench remain unchanged.
  const appChromeHidden = sidebarWorkbench || sidebarCreation;
  const workbenchChromeHidden = sidebarWorkbench;
  const sidebarClassName = [
    "sidebar",
    sidebarCollapsed ? "sidebar--collapsed" : "",
    sidebarWorkbench ? "sidebar--workbench" : "",
  ].filter(Boolean).join(" ");

  const updateProjectComposerAdapter = (sessionKey: string, update: (adapter: ProjectComposerAdapter) => void) => {
    const adapter = projectComposerAdaptersRef.current.get(sessionKey);
    if (!adapter) return;
    update(adapter);
    setProjectComposerRevision((revision) => revision + 1);
  };

  const discardProjectComposerAdapter = (adapter: ProjectComposerAdapter) => {
    releaseProjectComposerAdapter(projectComposerAdaptersRef.current, adapter.key);
    delete projectComposerDraftStore.current[adapter.key];
    setProjectComposerInsertRequests((current) => {
      if (!(adapter.key in current)) return current;
      const next = { ...current };
      delete next[adapter.key];
      return next;
    });
    if (adapter.tabId && adapter.phase !== "sent") void closeTab(adapter.tabId).catch(() => {});
    setProjectComposerRevision((revision) => revision + 1);
  };

  const promoteProjectComposerAdapter = async (adapter: ProjectComposerAdapter) => {
    setProjectComposerInsertRequests((current) => {
      if (!(adapter.key in current)) return current;
      const next = { ...current };
      delete next[adapter.key];
      return next;
    });
    setSelectedCloudProjectId(adapter.projectId);
    showChatView(false);
    setProjectComposerRevision((revision) => revision + 1);
  };

  const provisionNewCloudProjectConversation = (projectId: string): ProjectComposerAdapter | null => {
    const normalizedProjectId = projectId.trim();
    if (!normalizedProjectId || !sidebarAccountScopeKey) return null;
    const identity = { accountScopeKey: sidebarAccountScopeKey, projectId: normalizedProjectId };
    const { adapter, replacedSentKey } = ensureNewProjectConversationAdapter(projectComposerAdaptersRef.current, identity);
    if (!adapter) return null;
    if (replacedSentKey) {
      delete projectComposerDraftStore.current[replacedSentKey];
      setProjectComposerInsertRequests((requests) => {
        if (!(replacedSentKey in requests)) return requests;
        const next = { ...requests };
        delete next[replacedSentKey];
        return next;
      });
    }
    setProjectComposerRevision((revision) => revision + 1);
    return adapter;
  };

  const startNewCloudProjectConversation = (projectId: string) => {
    const adapter = provisionNewCloudProjectConversation(projectId);
    if (!adapter) return;
    const normalizedProjectId = adapter.projectId;
    openCloudProject(normalizedProjectId);
  };

  const leaveEmptyProjectComposer = (sessionKey: string) => {
    window.setTimeout(() => {
      const adapter = projectComposerAdaptersRef.current.get(sessionKey);
      const draft = projectComposerDraftStore.current[sessionKey];
      const plan = planProjectComposerLeave(adapter, draft);
      if (!plan) return;
      releaseProjectComposerAdapter(projectComposerAdaptersRef.current, plan.removeKey);
      delete projectComposerDraftStore.current[sessionKey];
      setProjectComposerInsertRequests((current) => {
        if (!(sessionKey in current)) return current;
        const next = { ...current };
        delete next[sessionKey];
        return next;
      });
      if (plan.closeTabId) void closeTab(plan.closeTabId).catch(() => {});
      setProjectComposerRevision((revision) => revision + 1);
    }, 0);
  };

  const createProjectComposerTab = async (
    adapter: ProjectComposerAdapter,
    workspacePath: string,
    attempt: ProjectComposerAttempt = createProjectComposerAttempt(adapter),
  ): Promise<string> => {
    attempt.step = "prepare-tab";
    attempt.hasTab = Boolean(adapter.tabId);
    attempt.hasSession = Boolean(adapter.sessionPath);
    if (adapter.tabId) {
      attempt.step = "prepare-tab";
      openCloudProject(adapter.projectId);
      return adapter.tabId;
    }
    updateProjectComposerAdapter(adapter.key, (current) => { current.phase = "preparing-tab"; });
    const target = projectComposerBlankSessionTarget(workspacePath, adapter.executionTarget);
    const desiredModelRef = adapter.modelRef.trim();
    const desiredBillingSource = adapter.modelBillingSource;
    const desiredEffort = adapter.effort?.current;
    openCloudProject(adapter.projectId);
    attempt.step = "ensure-blank-tab";
    const preparedTab = singleSurfaceLayout
      ? await ensureBlankSurface(target.scope, target.workspaceRoot)
      : await ensureBlankTab(target.scope, target.workspaceRoot);
    const tabId = preparedTab.id;
    seedActiveTabMeta(preparedTab);
    setProjectRevision((value) => value + 1);
    attempt.step = "refresh-tabs";
    await refreshTabMetas(undefined, (outcome) => { attempt.listTabsOutcome = outcome; });
    attempt.step = "verify-adapter";
    attempt.adapterCurrent = projectComposerAdaptersRef.current.get(adapter.key) === adapter;
    if (!attempt.adapterCurrent || projectComposerVisibleScopeRef.current !== adapter.accountScopeKey) {
      attempt.reason = "scope-changed";
      await closeTab(tabId);
      throw new Error("project composer scope changed while preparing the tab");
    }
    if (desiredModelRef) {
      attempt.step = "set-model";
      if (desiredBillingSource) await app.SetModelRouteForTab(tabId, desiredModelRef, desiredBillingSource);
      else await app.SetModelForTab(tabId, desiredModelRef);
    }
    if (desiredEffort) {
      attempt.step = "set-effort";
      await app.SetEffortForTab(tabId, desiredEffort);
    }
    attempt.step = "load-tab-state";
    attempt.listTabsOutcome = "not-called";
    let tabStateListFailed = false;
    const [tabs, effort, expertState] = await Promise.all([
      app.ListTabs().catch(() => {
        tabStateListFailed = true;
        return [] as TabMeta[];
      }),
      app.EffortForTab(tabId).catch(() => undefined),
      app.CurrentExpertForTab(tabId).catch(() => null),
    ]);
    attempt.listTabsOutcome = tabStateListFailed ? "failed" : "succeeded";
    const preparedMeta = asArray<TabMeta>(tabs).find((tab) => tab.id === tabId);
    if (projectComposerAdaptersRef.current.get(adapter.key) !== adapter
      || projectComposerVisibleScopeRef.current !== adapter.accountScopeKey) {
      await closeTab(tabId);
      throw new Error("project composer scope changed while preparing the tab");
    }
    attempt.step = "load-tab-state";
    const preparedExpert = expertSelectionFromState(expertState, expertCatalog, userExpertCatalog);
    updateProjectComposerAdapter(adapter.key, (current) => {
      current.tabId = tabId;
      current.modelRef = preparedMeta?.modelRef?.trim() ?? "";
      current.modelBillingSource = preparedMeta?.billingSource || undefined;
      current.effort = effort;
      if (current.expertRequestSequence === undefined) current.activeExpert = preparedExpert || undefined;
      current.phase = "draft";
    });
    openCloudProject(adapter.projectId);
    return tabId;
  };

  const prepareProjectComposerTab = (
    adapter: ProjectComposerAdapter,
    workspacePath: string,
    attempt: ProjectComposerAttempt = createProjectComposerAttempt(adapter),
  ): Promise<string> => {
    if (adapter.tabPreparation) return adapter.tabPreparation;
    const preparation = createProjectComposerTab(adapter, workspacePath, attempt).finally(() => {
      if (adapter.tabPreparation === preparation) adapter.tabPreparation = undefined;
    });
    adapter.tabPreparation = preparation;
    return preparation;
  };

  const selectProjectComposerExpert = (adapter: ProjectComposerAdapter, selection: ExpertSelection): Promise<boolean> => {
    const requestSequence = ++expertActivationSequenceRef.current;
    const previous = adapter.activeExpert;
    adapter.expertRequestSequence = requestSequence;
    adapter.activeExpert = selection;
    setProjectComposerRevision((value) => value + 1);
    const isCurrent = () => projectComposerAdaptersRef.current.get(adapter.key) === adapter
      && adapter.expertRequestSequence === requestSequence
      && expertActivationSequenceRef.current === requestSequence;
    const operation = (async () => {
      try {
        const tabId = await prepareProjectComposerTab(adapter, adapter.workspacePath);
        if (!isCurrent()) return false;
        const applied = await activateExpertSelection(selection, tabId, { tabId, expertId: selection.id, requestSequence }, "preserve");
        if (!applied && isCurrent()) adapter.activeExpert = previous;
        return applied;
      } catch (error) {
        if (isCurrent()) {
          adapter.activeExpert = previous;
          showError("experts.activate", error, "启用专家失败，请稍后重试。");
        }
        return false;
      } finally {
        setProjectComposerRevision((value) => value + 1);
      }
    })();
    adapter.expertMutation = operation;
    return operation;
  };

  const loadProjectComposerDraftEffort = (adapter: ProjectComposerAdapter) => {
    if (adapter.tabId) return;
    const modelRef = adapter.modelRef.trim();
    if (!modelRef) return;
    const target = projectComposerBlankSessionTarget(adapter.workspacePath, adapter.executionTarget);
    const routeKey = JSON.stringify([target.scope, target.workspaceRoot, modelRef, adapter.modelBillingSource ?? ""]);
    if (adapter.effortRouteKey === routeKey) return;
    const currentEffort = adapter.effort?.current || "auto";
    adapter.effortRouteKey = routeKey;
    adapter.effort = undefined;
    void app.EffortForDraftModel(target.scope, target.workspaceRoot, modelRef, adapter.modelBillingSource ?? "", currentEffort)
      .then((effort) => {
        updateProjectComposerAdapter(adapter.key, (current) => {
          if (current.tabId || current.effortRouteKey !== routeKey) return;
          current.effort = effort;
        });
      })
      .catch(() => {});
  };

  const activateProjectComposerSession = async (
    adapter: ProjectComposerAdapter,
    attempt: ProjectComposerAttempt = createProjectComposerAttempt(adapter),
  ): Promise<TabMeta> => {
    attempt.step = "verify-pinned";
    attempt.hasTab = Boolean(adapter.tabId);
    attempt.hasSession = Boolean(adapter.sessionPath);
    if (!adapter.tabId || !adapter.sessionPath) {
      attempt.reason = "not-pinned";
      throw new Error("project session is not pinned");
    }
    attempt.step = "refresh-tabs";
    delete attempt.tabFound;
    delete attempt.pathMatches;
    const tabs = await refreshTabMetas(undefined, (outcome) => { attempt.listTabsOutcome = outcome; });
    attempt.step = "verify-target";
    const target = tabs.find((tab) => tab.id === adapter.tabId);
    if (attempt.listTabsOutcome === "succeeded") {
      attempt.tabFound = Boolean(target);
      if (target) attempt.pathMatches = target.sessionPath === adapter.sessionPath;
    }
    if (!target || target.sessionPath !== adapter.sessionPath) {
      if (attempt.listTabsOutcome === "succeeded") attempt.reason = target ? "path-mismatch" : "tab-missing";
      throw new Error("project session tab/path reconciliation failed");
    }
    attempt.step = "switch-tab";
    await switchTab(adapter.tabId, target);
    return target;
  };

  const renderProjectComposer = ({ sessionKey, projectId, projectName, workspacePath, capabilityBindings, instructionVersion, contentHash, projectWritable, projectDisabledReason, onCreateSession, onCapabilitiesChanged }: {
    sessionKey: string;
    projectId: string;
    projectName: string;
    workspacePath: string;
    capabilityBindings: CloudProjectCapabilityBinding[];
    instructionVersion: number;
    contentHash: string;
    projectWritable: boolean;
    projectDisabledReason?: string;
    onCreateSession: (tabId: string, workspacePath?: string, executionTarget?: "local" | "cloud") => Promise<import("./features/projects/model/projectTypes").CloudProjectSession>;
    onCapabilitiesChanged: (items: CloudProjectCapabilityBinding[]) => void;
  }) => {
    void projectComposerRevision;
    const identity = parseProjectComposerKey(sessionKey);
    const adapter = identity ? ensureProjectComposerAdapter(projectComposerAdaptersRef.current, identity) : null;
    if (!adapter) {
      const retained = [...projectComposerAdaptersRef.current.values()].filter((candidate) => candidate.accountScopeKey === identity?.accountScopeKey && candidate.phase !== "sent");
      return <section className="primary-workspace__notice project-composer-capacity" role="alert">
        <strong>当前账号最多保留 8 个项目草稿</strong>
        <span>请重新打开或丢弃一个已有草稿后继续。</span>
        <div className="project-composer-capacity__actions">
          {retained.map((candidate) => <div key={candidate.key}>
            <button type="button" onClick={() => setSelectedCloudProjectId(candidate.projectId)}>重新打开 {candidate.projectId}</button>
            <button type="button" onClick={() => discardProjectComposerAdapter(candidate)}>丢弃</button>
          </div>)}
        </div>
      </section>;
    }
    if (subscriptionRepairSettings) initializeProjectComposerDefaults(adapter, subscriptionRepairSettings);
    if (!adapter.workspacePath && workspacePath.trim() && !adapter.sessionPath) {
      adapter.workspacePath = workspacePath.trim();
      adapter.executionTarget = "local";
    }
    loadProjectComposerDraftEffort(adapter);
    applyProjectCapabilityBindings(adapter, capabilityBindings);
    const projectExperts = projectExpertSelections(capabilityBindings, expertCatalog, userExpertCatalog);
    adapter.availableExperts = projectExperts;
    const projectInsertRequest = projectComposerInsertRequests[sessionKey] ?? composerInsertRequest;
    const targetMeta = tabMetas.find((tab) => tab.id === adapter.tabId);
    const projectProfile = adapter.profile;
    const patchProfile = (patch: Partial<Omit<ComposerProfile, "pending">>) => updateProjectComposerAdapter(sessionKey, (current) => {
      current.profile = { ...current.profile, ...patch, pending: {} };
    });
    const setProjectMode = (mode: Mode) => patchProfile(composerProfileWithMode(mode));
    const setProjectCollaborationMode = (mode: CollaborationMode) => patchProfile(mode === "goal"
      ? { collaborationMode: "normal", goalDraftMode: true, goal: "" }
      : { collaborationMode: mode, goalDraftMode: false, goal: "" });
    const setProjectToolApprovalMode = async (mode: ToolApprovalMode): Promise<void> => {
      const targetTabId = adapter.tabId;
      if (!targetTabId) {
        patchProfile({ toolApprovalMode: mode });
        return;
      }
      try {
        await app.SetToolApprovalModeForTab(targetTabId, mode);
        if (projectComposerAdaptersRef.current.get(sessionKey) !== adapter || adapter.tabId !== targetTabId) return;
        patchProfile({ toolApprovalMode: mode });
      } catch (error) {
        showError("projectComposer.toolApprovalMode", error, "保存项目任务审批设置失败，请稍后重试。");
      }
    };
    const toggleProjectConnector = (name: string) => toggleComposerConnector(name);
    const openCapability = (kind: ProjectCapabilityKind) => openProjectCapability(kind, document.activeElement instanceof HTMLButtonElement ? document.activeElement : document.createElement("button"), { sessionKey, projectId, onCapabilitiesChanged });
    const localWorkspaceMissing = adapter.executionTarget === "local" && !adapter.workspacePath.trim();

    return <div className={`session-composer-region session-composer-region--project${expertMemberHistoryFocused ? " session-composer-region--history" : ""}`} id="project-composer-input">
      <div className="session-composer-slot">
        <div className="session-composer-content" inert={expertMemberHistoryFocused ? true : undefined} aria-hidden={expertMemberHistoryFocused || undefined}>
          <Composer
          running={Boolean(targetMeta?.running)}
          collaborationMode={displayedComposerProfileCollaborationMode(projectProfile)}
          toolApprovalMode={projectProfile.toolApprovalMode}
          tokenMode={projectProfile.tokenMode}
          goal={projectProfile.goal}
          cwd={targetMeta?.workspaceRoot}
          modelLabel={adapter.modelRef}
          tabId={adapter.tabId || undefined}
          connectorOwnerTabId={adapter.tabId || activeTabId || undefined}
          modelCatalog={sharedModelCatalog}
          currentModelBillingSource={adapter.modelBillingSource}
          modelSwitchPending={false}
          modelSelectionPresentation={composerModelSelectionPresentation}
          subscriptionAccess={subscriptionAccess}
          effort={adapter.effort}
          onSend={async (displayText, submitText = displayText, capabilities) => {
            const expertOperation = adapter.expertMutation;
            if (expertOperation && !await expertOperation) return "blocked";
            if (projectComposerAdaptersRef.current.get(sessionKey) !== adapter || expertOperation !== adapter.expertMutation) return "blocked";
            const attempt = createProjectComposerAttempt(adapter);
            if (["preparing-tab", "creating-session", "sending"].includes(adapter.phase)) return "blocked";
            const submitTurn = async () => {
              try {
                return await submitProjectComposerTurn(adapter, { projectId, instructionVersion, contentHash }, {
                  createSession: (tabId) => onCreateSession(tabId, adapter.executionTarget === "local" ? adapter.workspacePath : "", adapter.executionTarget),
                  refreshTabs: () => refreshTabMetas(undefined, (outcome) => { attempt.listTabsOutcome = outcome; }),
                  activateTab: async (tabId, target) => { await switchTab(tabId, target); },
                  setCollaborationMode: (tabId) => app.SetCollaborationModeForTab(tabId, controllerComposerProfileCollaborationMode(projectProfile)),
                  setToolApprovalMode: (tabId, mode) => app.SetToolApprovalModeForTab(tabId, mode),
                  setTokenMode: (tabId, mode) => app.SetTokenModeForTab(tabId, mode),
                  setGoal: (tabId, goal) => goal.trim() ? app.SetGoalForTab(tabId, goal) : app.ClearGoalForTab(tabId),
                  send: async (tabId) => {
                    let projectCapabilities: ReturnType<typeof availableProjectCapabilitySelection>;
                    try {
                      attempt.step = "resolve-capabilities";
                      projectCapabilities = await resolveAvailableProjectCapabilities(adapter.capabilities);
                    } catch (error) {
                      throw asProjectComposerSubmitError("resolve-capabilities", error, attempt);
                    }
                    try {
                      attempt.step = "send-message";
                      await sendToTab(tabId, displayText, submitText, projectComposerCapabilitySubmit(submitText, projectCapabilities, capabilities, adapter.capabilities));
                    } catch (error) {
                      throw asProjectComposerSubmitError("send-message", error, attempt);
                    }
                  },
                  promote: () => promoteProjectComposerAdapter(adapter),
                }, attempt);
              } finally {
                setProjectComposerRevision((revision) => revision + 1);
              }
            };
            if (adapter.sessionPath) {
              const previousPhase = adapter.phase;
              try {
                const target = await activateProjectComposerSession(adapter, attempt);
                if (target.running) {
                  if (subscriptionModelInvocationBlocked) return "blocked";
                  attempt.step = "steer";
                  await app.SteerForTab(adapter.tabId, submitText.trim());
                  return "submitted";
                }
                const trimmed = displayText.trim();
                if (trimmed.startsWith("!")) {
                  const command = trimmed.slice(1).trim();
                  if (!command) return "blocked";
                  attempt.step = "run-shell";
                  await app.RunShellForTab(adapter.tabId, command);
                  return "handled-no-turn";
                }
                const connectorName = mcpConnectTarget(trimmed);
                if (connectorName) {
                  attempt.step = "connect-command";
                  const server = await ensureUsableMCPServer(connectorName, () => app.MCPServers(), (name) => app.ReconnectMCPServer(name));
                  showToast(`已连接 ${connectorName} · ${server.tools} 个工具，请在连接器菜单中启用`, "info");
                  return "handled-no-turn";
                }
                if (/^\/(?:memory|clear|theme|model)(?:\s|$)/.test(trimmed)) {
                  attempt.step = "management-command";
                  return handleSend(displayText, submitText, capabilities);
                }
                if (trimmed.startsWith("/")) {
                  if (isReadOnlyManagementCommand(trimmed)) {
                    attempt.step = "management-command";
                    await sendToTab(adapter.tabId, displayText, submitText, capabilities);
                    return "handled-no-turn";
                  }
                  return submitTurn();
                }
                return submitTurn();
              } catch (error) {
                updateProjectComposerAdapter(sessionKey, (current) => { current.phase = previousPhase === "sent" ? "sent" : "pinned-unsent"; });
                const failure = asProjectComposerSubmitError("activate-session", error, attempt);
                showError("projectComposer.send", failure, projectComposerFailureMessage(failure), { durationMs: 6000 });
                return "blocked";
              }
            }
            const disposition = classifyProjectComposerFirstSend(displayText, submitText);
            if (disposition !== "submitted") return disposition;
            if (subscriptionModelInvocationBlocked) {
              attempt.step = "model-admission";
              attempt.reason = "model-blocked";
              const failure = asProjectComposerSubmitError("prepare-tab", new Error("project model invocation is blocked"), attempt);
              showError("projectComposer.send", failure, projectComposerFailureMessage(failure), { durationMs: 6000 });
              return "blocked";
            }
            try {
              await prepareProjectComposerTab(adapter, adapter.workspacePath, attempt);
              return submitTurn();
            } catch (error) {
              updateProjectComposerAdapter(sessionKey, (current) => { current.phase = current.sessionPath ? "pinned-unsent" : "draft"; });
              const failure = asProjectComposerSubmitError("prepare-tab", error, attempt);
              showError("projectComposer.send", failure, projectComposerFailureMessage(failure), { durationMs: 6000 });
              return "blocked";
            }
          }}
          onSubmitError={(error) => {
            const failure = asProjectComposerSubmitError("prepare-tab", error);
            showError("projectComposer.send", failure, projectComposerFailureMessage(failure), { durationMs: 6000 });
          }}
          onSubmittedDraftCleared={(clearedDraftKey) => {
            // Prepare the next project draft without reopening the project
            // workspace: the accepted turn has already promoted this surface
            // to the ordinary chat selected in the sidebar.
            if (clearedDraftKey === sessionKey) provisionNewCloudProjectConversation(projectId);
          }}
          onCancel={() => {
            if (!adapter.tabId) return undefined;
            return cancelForTab(adapter.tabId);
          }}
          onCycleMode={() => setProjectCollaborationMode(projectProfile.collaborationMode === "plan" ? "normal" : "plan")}
          onSetMode={setProjectMode}
          onSetCollaborationMode={setProjectCollaborationMode}
          onSetToolApprovalMode={setProjectToolApprovalMode}
          onToggleYoloApprovalMode={() => setProjectToolApprovalMode(projectProfile.toolApprovalMode === "yolo" ? "ask" : "yolo")}
          onClearGoal={() => patchProfile({ collaborationMode: "normal", goalDraftMode: false, goal: "" })}
          onSwitchModel={async (modelRef, billingSource) => {
            if (!adapter.tabId) {
              updateProjectComposerAdapter(sessionKey, (current) => {
                current.modelRef = modelRef;
                current.modelBillingSource = billingSource;
                current.defaultsInitialized = true;
              });
              return;
            }
            const tabId = adapter.tabId;
            if (billingSource) await app.SetModelRouteForTab(tabId, modelRef, billingSource);
            else await app.SetModelForTab(tabId, modelRef);
            const effort = await app.EffortForTab(tabId).catch(() => undefined);
            updateProjectComposerAdapter(sessionKey, (current) => {
              current.modelRef = modelRef;
              current.modelBillingSource = billingSource;
              current.defaultsInitialized = true;
              current.effort = effort;
              if (!current.sessionPath) current.phase = "draft";
            });
          }}
          onSetEffort={async (level) => {
            if (!adapter.tabId) {
              updateProjectComposerAdapter(sessionKey, (current) => {
                if (current.effort?.supported) current.effort = { ...current.effort, current: level };
              });
              return;
            }
            await app.SetEffortForTab(adapter.tabId, level);
            const effort = await app.EffortForTab(adapter.tabId).catch(() => undefined);
            updateProjectComposerAdapter(sessionKey, (current) => {
              current.effort = effort;
              if (!current.sessionPath) current.phase = "draft";
            });
          }}
          onSetTokenMode={(mode) => patchProfile({ tokenMode: mode })}
          onOpenExpertDirectory={() => openCapability("expert")}
          onOpenSkills={() => openCapability("skill")}
          onOpenConnectors={() => {
            setProjectCapabilityOrigin({ sessionKey, projectId, kind: "connector", onCapabilitiesChanged });
            openCapabilityWorkspace("connectors");
          }}
          selectedConnectors={enabledComposerConnectors}
          onToggleConnector={toggleProjectConnector}
          onClearConnector={toggleProjectConnector}
          onClearExpert={async () => {
            await clearExpertSelectionForTab(adapter.tabId, adapter);
          }}
          activeExpert={adapter.activeExpert ?? null}
          recentExperts={projectExperts}
          recentExpertsLabel="项目预选专家"
          recentExpertsLimit={projectExperts.length}
          onSelectRecentExpert={async (selection) => {
            await selectProjectComposerExpert(adapter, selection);
          }}
          insertRequest={projectInsertRequest}
          onInsertRequestConsumed={(id) => {
            if (projectComposerInsertRequests[sessionKey]?.id === id) {
              setProjectComposerInsertRequests((current) => current[sessionKey]?.id === id ? Object.fromEntries(Object.entries(current).filter(([key]) => key !== sessionKey)) : current);
              return;
            }
            handleComposerInsertRequestConsumed(id);
          }}
          readOnly={Boolean(targetMeta?.readOnly)}
          disabled={expertMemberHistoryFocused || adapter.phase === "preparing-tab" || adapter.phase === "creating-session" || adapter.phase === "sending" || (activeTabId === adapter.tabId && (rewindCommitting || state.messageAction != null || state.approval != null || state.ask != null || clearContextPending))}
          submitDisabled={!projectWritable || localWorkspaceMissing || subscriptionModelInvocationBlocked || instructionVersion <= 0 || !contentHash}
          submitDisabledReason={projectDisabledReason
            || (localWorkspaceMissing ? "请选择工作空间后再发送" : undefined)
            || (instructionVersion <= 0 || !contentHash ? "项目指令尚未准备完成" : undefined)
            || (subscriptionModelInvocationAccess.state === "loading" ? t("subscriptionModelGate.verifyingTooltip") : undefined)}
          decisionPending={adapter.phase === "preparing-tab" || adapter.phase === "creating-session" || adapter.phase === "sending"}
          ready={projectWritable && !localWorkspaceMissing && !subscriptionModelInvocationBlocked && instructionVersion > 0 && Boolean(contentHash)}
		  turnStartAt={activeTabId === adapter.tabId ? state.turnStartAt : undefined}
		  turnTokens={activeTabId === adapter.tabId ? state.turnTokens : undefined}
		  retry={activeTabId === adapter.tabId ? state.retry : undefined}
          transientDismissSignal={transientOverlayDismissSignal}
          sessionKey={sessionKey}
          surfaceTransitioning={false}
          draftStore={projectComposerDraftStore}
          projectPresentation={{
            scopeKey: sessionKey,
            executionTarget: adapter.executionTarget,
            workspacePath: adapter.workspacePath,
            cloudProjectId: projectId,
            cloudProjectName: projectName,
            workspaceLocked: Boolean(adapter.sessionPath),
            workspaceSelectionLocked: Boolean(adapter.workspacePath),
            onExecutionTargetChange: (executionTarget) => updateProjectComposerAdapter(sessionKey, (current) => {
              if (current.sessionPath) return;
              current.executionTarget = executionTarget;
            }),
            onWorkspacePathChange: (path) => updateProjectComposerAdapter(sessionKey, (current) => {
              if (!current.sessionPath && current.executionTarget === "local") current.workspacePath = path;
            }),
            onCloudProjectIdChange: (nextProjectId) => {
              const normalizedProjectId = nextProjectId.trim();
              if (!normalizedProjectId || normalizedProjectId === projectId || adapter.sessionPath) return;
              const nextAdapter = ensureProjectComposerAdapter(projectComposerAdaptersRef.current, {
                accountScopeKey: adapter.accountScopeKey,
                projectId: normalizedProjectId,
              });
              if (!nextAdapter) {
                showToast("当前账号最多保留 8 个项目草稿，请先丢弃一个草稿。", "info");
                return;
              }
              if (!nextAdapter.sessionPath) nextAdapter.executionTarget = "cloud";
              openCloudProject(normalizedProjectId);
              setProjectComposerRevision((revision) => revision + 1);
            },
          }}
          />
        </div>
        {expertMemberHistoryFocused && (
          <div className="session-composer-history-overlay">
            <button type="button" className="agent-workspace__return" onClick={() => expertMemberHistoryReturnRef.current()}>
              <span aria-hidden="true">↩</span>
              <span>{t("agentWorkspace.returnMain")}</span>
            </button>
          </div>
        )}
      </div>
    </div>;
  };

  const openProjectCapability = (kind: ProjectCapabilityKind, _trigger: HTMLButtonElement, context: {
    sessionKey: string;
    projectId: string;
    onCapabilitiesChanged?: (items: CloudProjectCapabilityBinding[]) => void;
    onCapabilityCatalogChanged?: (kind: ProjectCapabilityKind, items: ProjectCapabilityCatalogItem[]) => void;
  }) => {
    setProjectCapabilityOrigin({ ...context, kind });
    setRestoreProjectCapabilityFocus(null);
  };

  const returnToProjectCapabilityOrigin = (focus = true) => {
    if (!projectCapabilityOrigin) return;
    setRestoreProjectCapabilityFocus(focus ? projectCapabilityOrigin.kind : null);
    setProjectCapabilityOrigin(null);
    openCloudProject(projectCapabilityOrigin.projectId);
  };

  const selectWorkspaceSkill = (name: string) => {
    if (!projectCapabilityOrigin) {
      selectComposerSkill(name);
      return;
    }
    setProjectComposerInsertRequests((current) => ({ ...current, [projectCapabilityOrigin.sessionKey]: { id: Date.now(), text: `${formatSkillToken(name)} `, kind: "skill" } }));
    returnToProjectCapabilityOrigin();
  };

  const selectWorkspaceConnector = async (name: string) => {
    if (!projectCapabilityOrigin) {
      await selectComposerConnector(name);
      return;
    }
    toggleComposerConnector(name);
    returnToProjectCapabilityOrigin();
  };

  const tryWorkspaceConnector = (name: string, prompt: string) => {
    if (!projectCapabilityOrigin) {
      tryComposerConnector(name, prompt);
      return;
    }
    enableComposerConnector(name);
    setProjectComposerInsertRequests((current) => ({ ...current, [projectCapabilityOrigin.sessionKey]: { id: Date.now(), text: prompt, mode: "replace" } }));
    returnToProjectCapabilityOrigin();
  };

  const summonWorkspaceExpert = async (selection: ExpertSelection) => {
    if (!projectCapabilityOrigin) {
      await handleSummonExpert(selection);
      return;
    }
    const adapter = projectComposerAdaptersRef.current.get(projectCapabilityOrigin.sessionKey);
    if (!adapter) throw new Error("project composer adapter is unavailable");
    const origin = projectCapabilityOrigin;
    if (await selectProjectComposerExpert(adapter, selection)) {
      if (projectComposerAdaptersRef.current.get(origin.sessionKey) === adapter) returnToProjectCapabilityOrigin();
    }
  };

  const superpowerSessionAdapter = useMemo<SuperpowerSessionAdapter | undefined>(() => {
    const desktopSessionAvailable = typeof window.go?.main?.App?.SubmitDisplay === "function";
    if (!desktopSessionAvailable) return undefined;
    const unavailableReason = activeTab?.readOnly
      ? "当前会话为只读会话，无法提交研究追问。"
      : state.running
        ? "当前会话正在执行其他任务，请完成后再发送。"
        : displayedSendAvailability?.state === "subscription_auth_required"
          ? "当前模型需要登录，请登录后再发送。"
          : displayedSendAvailability?.state === "subscription_account_action_required"
            ? "订阅账户需要处理，请恢复账户后再发送。"
            : displayedSendAvailability?.state === "subscription_balance_depleted"
              ? "当前订阅余额不足，请充值或切换模型。"
              : displayedSendAvailability?.state === "byok_configuration_required"
                ? "当前模型尚未配置 API Key，请先完成模型配置。"
              : displayedSendAvailability?.state === "service_unavailable"
                  ? "模型服务暂不可用，请恢复服务或切换模型后再发送。"
                  : displayedSendAvailability?.state === "startup_failed"
                    ? displayedSendAvailability.detail || "当前会话启动失败，请重试会话。"
                    : subscriptionModelInvocationBlocked
                      ? "当前模型不可用，请在模型选择器中切换可用模型。"
                      : existingSessionSubmitBlocked
                        ? "正在恢复当前会话，请准备完成后再发送…"
                        : "正在准备当前会话，请准备完成后再发送…";
    return {
      ready: controllerReady
        && !existingSessionSubmitBlocked
        && !subscriptionModelInvocationBlocked
        && !state.running
        && !activeTab?.readOnly,
      running: state.running,
      unavailableReason,
      messages: state.items.reduce<SuperpowerSessionAdapter["messages"]>((messages, item) => {
        if (item.kind === "user") messages.push({ id: item.id, role: "user", text: item.text, streaming: false });
        if (item.kind === "assistant") messages.push({ id: item.id, role: "assistant", text: item.text, streaming: item.streaming });
        return messages;
      }, []),
      assistantMessages: state.items.flatMap((item) => item.kind === "assistant"
        ? [{ id: item.id, text: item.text, streaming: item.streaming }]
        : []),
      submit: handleSend,
    };
  }, [activeTab?.readOnly, controllerReady, displayedSendAvailability?.detail, displayedSendAvailability?.state, existingSessionSubmitBlocked, handleSend, state.items, state.running, subscriptionModelInvocationBlocked]);

  const handleCapabilityBindingChanged = useCallback(() => {
    setProjectRevision((value) => value + 1);
  }, []);

  const renderSessionComposer = (
    onSend: SuperpowerComposerSend = handleSend,
    selectedConnectors: string[] = activeComposerCapabilities.connectors,
  ) => (
    <Composer
      running={state.running || rewindCommitting}
      collaborationMode={collaborationMode}
      toolApprovalMode={toolApprovalMode}
      tokenMode={tokenMode}
      goal={goal}
      cwd={state.meta?.cwd}
      modelLabel={composerModelLabel}
      tabId={activeTabId}
      modelCatalog={sharedModelCatalog}
      currentModelBillingSource={composerModelBillingSource}
      modelSwitchPending={sessionComposerSelectionBusy}
      modelSelectionPresentation={sessionComposerModelSelectionPresentation}
      subscriptionAccess={subscriptionAccess}
      effort={sessionComposerModelSelectionPresentation === "auto"
        ? modelSwitcherPresentation.pending
          ? modelSwitcherPresentation.effort
          : state.composerSelection.phase === "ready" || state.composerSelection.phase === "updating-effort"
            ? state.effort
            : undefined
        : undefined}
      effortSnapshot={sessionComposerModelSelectionPresentation === "auto" && sessionComposerSelectionBusy
        ? state.composerSelection.effortSnapshot
        : undefined}
      onSend={onSend}
      onCancel={cancel}
      onCycleMode={cycleMode}
      onSetMode={applyMode}
      onSetCollaborationMode={applyCollaborationMode}
      onSetToolApprovalMode={applyToolApprovalMode}
      onToggleYoloApprovalMode={toggleYoloApprovalMode}
      onClearGoal={() => applyGoal("")}
      onSwitchModel={switchModel}
      onSetEffort={setEffort}
      onSetTokenMode={applyTokenMode}
      onOpenExpertDirectory={handleComposerOpenExpertDirectory}
      onOpenSkills={openSkillsWorkspace}
      onOpenConnectors={openConnectorsWorkspace}
      connectorOwnerTabId={activeTabId}
      selectedConnectors={selectedConnectors}
      onToggleConnector={selectComposerConnector}
      onClearConnector={toggleComposerConnector}
      onClearExpert={clearActiveExpertSelection}
      activeExpert={displayedActiveExpert}
      recentExperts={activeProjectComposerAdapter ? activeProjectComposerAdapter.availableExperts : recentComposerExperts}
      recentExpertsLabel={activeProjectComposerAdapter ? "项目预选专家" : undefined}
      recentExpertsLimit={activeProjectComposerAdapter ? activeProjectComposerAdapter.availableExperts.length : undefined}
      onSelectRecentExpert={handleSelectRecentExpert}
      insertRequest={composerInsertRequest}
      onInsertRequestConsumed={handleComposerInsertRequestConsumed}
      readOnly={Boolean(activeTab?.readOnly)}
      disabled={expertMemberHistoryFocused || surfaceTransitionActive || rewindCommitting || state.messageAction != null || state.approval != null || state.ask != null || clearContextPending}
      submitDisabled={!controllerReady || existingSessionSubmitBlocked || subscriptionModelInvocationBlocked}
      submitDisabledReason={subscriptionModelInvocationAccess.state === "loading" ? t("subscriptionModelGate.verifyingTooltip") : undefined}
      decisionPending={rewindCommitting || state.messageAction != null || state.approval != null || state.ask != null || clearContextPending}
      ready={controllerReady && !existingSessionSubmitBlocked && !subscriptionModelInvocationBlocked}
      turnStartAt={state.turnStartAt}
      turnTokens={state.turnTokens}
      retry={state.retry}
      transientDismissSignal={transientOverlayDismissSignal}
      sessionKey={composerSessionKey}
      surfaceTransitioning={surfaceTransitionActive}
    />
  );

  const renderSessionOperationalPrompt = () => (
    <>
      {state.approval && (
        <ApprovalModal
          key={state.approval.id}
          approval={state.approval}
          onAnswer={async (allow, session, persist) => {
            // Approving an exit_plan_mode plan leaves plan mode; await the
            // mode switch before sending the approval so the controller
            // observes the updated state before it unblocks.
            if (state.approval!.tool === "exit_plan_mode" && allow) await applyCollaborationMode("normal", { rememberUserIntent: false });
            approve(state.approval!.id, allow, session, persist);
          }}
          onRevisePlan={(text) => {
            setPendingPlanRevision(text);
            approve(state.approval!.id, false, false, false);
          }}
          onExitPlan={async () => {
            await applyCollaborationMode("normal");
            approve(state.approval!.id, false, false, false);
          }}
          onStop={cancel}
        />
      )}
      {state.ask && (
        <AskCard
          ask={state.ask}
          onAnswer={answerQuestion}
          onDismiss={() => answerQuestion(state.ask!.id, [])}
          onStop={cancel}
        />
      )}
    </>
  );

  const renderSessionTranscript = (options?: {
    welcomePresentation?: CapabilityWelcomePresentation;
    onWelcomePrompt?: (text: string) => void;
  }) => (
    <Transcript
      items={displayItems}
      parallelItemsByCall={state.parallelItemsByCall}
      live={state.live}
      tabId={transcriptFileTabId}
      viewKey={sessionViewKey}
      initialScrollAnchor={sessionViewScroll}
      onScrollAnchorChange={updateSessionViewScroll}
      historyRevalidationSignal={state.historyRevalidationSeq}
      hasOlderHistory={state.historyHasMore}
      loadingOlderHistory={state.historyLoadingOlder}
      onLoadOlderHistory={loadOlderHistory}
      footerHeight={footerHeight}
      onPrompt={options?.onWelcomePrompt ?? handleTranscriptPrompt}
      onSelectScenario={options?.onWelcomePrompt ?? handleCreationScenario}
      scene={welcomeScenePresentation.scene}
      sceneState={welcomeScenePresentation.sceneState}
      sceneSelectionAvailability={welcomeSceneSelectionAvailability}
      onSelectScene={handleSelectScene}
      onEditPrompt={handleEditPrompt}
      onRewind={handleMessageAction}
      checkpoints={state.checkpoints}
      actionPending={state.messageAction != null}
      rewindDisabled={state.historyHasMore || Boolean(activeTab?.readOnly) || !controllerReady || subscriptionModelInvocationBlocked || hydratePlaceholderActive || rewindState != null || rewindCommitting || state.running || state.messageAction != null || state.approval != null || state.ask != null || clearContextPending}
      running={state.running || rewindCommitting}
      welcomeVariant={sidebarCreation ? "creation" : "default"}
      capabilityWelcomePresentation={options?.welcomePresentation}
      creationMode={sidebarCreation}
      actionHoverMenus={sidebarCreation && !hydratePlaceholderActive}
      rewindSignal={rewindSignal}
      revealSignal={transcriptRevealSignal}
      hydrating={transcriptHydrating || Boolean(state.hydrateError)}
      showWelcomeDuringHydration={showWelcomeDuringHydration}
      existingSessionLoading={existingSessionHydrating}
      taskTrees={state.taskTrees}
      activeExpert={displayedActiveExpert}
      childItemsByRef={state.childItemsByRef}
      expertSelectionRevision={expertSelectionRevisions[activeTabId ?? ""] ?? 0}
      subagentStatusByRef={state.subagentStatusByRef}
      onExpertMemberHistoryFocusChange={handleExpertMemberHistoryFocusChange}
    />
  );

  const renderSessionStatusBar = () => (
    <StatusBar
      context={state.context}
      usage={state.usage}
      balance={state.balance}
      jobs={state.jobs}
      running={state.running || rewindCommitting}
      collaborationMode={collaborationMode}
      toolApprovalMode={toolApprovalMode}
      sessionTurns={sessionTurns}
      sessionTokens={state.sessionTokens}
      turnTokens={state.turnTotalTokens}
      turnCost={state.turnCost}
      cost={state.sessionCost}
      currency={state.sessionCurrency}
      modelLabel={displayedModelLabel}
      modelPending={subscriptionModelPendingVisible}
      labelStyle={statusBarStyle}
      items={statusBarItems}
      workspacePath={state.meta?.workspacePath || state.meta?.workspaceRoot || state.meta?.cwd}
      workspaceName={state.meta?.workspaceName}
      gitBranch={state.meta?.gitBranch}
      hydrationLabel={hydrateStatusLabel}
    />
  );

  return (
    <ShellExpandProvider>
    <ShellHotkeys />
    <TextSizeHotkeys />
    <div
      ref={appRef}
      className={[
        "app",
        `app--${desktopPlatform}`,
        browserPreviewChrome ? "app--browser-preview" : "",
        sidebarWorkbench ? "app--workbench" : "",
        sidebarCreation ? "app--creation" : "",
      ].filter(Boolean).join(" ")}
    >
      <div
        ref={layoutRef}
        className={[
          "layout",
          sidebarWorkbench ? "layout--workbench" : "",
          workbenchChromeHidden ? "layout--workbench-chrome-hidden" : "",
          sidebarCreation ? "layout--creation-chrome-hidden" : "",
          sidebarCollapsed ? "layout--sidebar-collapsed" : "",
          sidebarResizing ? "layout--resizing layout--sidebar-resizing" : "",
          workspacePanelGridOpen ? "layout--workspace-open" : "",
          workspacePanelRenderable && workspacePanelMaximized ? "layout--workspace-maximized" : "",
          workspacePanelResizing ? "layout--resizing layout--workspace-resizing" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        style={layoutStyle}
      >
        {!appChromeHidden && (
          <AppChrome
            platform={desktopPlatform}
            browserPreviewChrome={browserPreviewChrome}
            workbenchChrome={sidebarWorkbench}
            tabs={visibleTabs}
            activeTabId={visibleTabId}
            revealActiveSignal={tabRevealSignal}
            commandCompact={true}
            sidebarTogglePressed={sidebarTogglePressed}
            sidebarExpandBlocked={sidebarExpandBlocked}
            sidebarCollapsed={sidebarCollapsed}
            sidebarToggleTitle={sidebarToggleTitle}
            workspacePanelAvailable={workspacePanelAvailable}
            workspacePanelMaximized={workspacePanelMaximized}
            workspacePanelRenderable={workspacePanelRenderable}
            workspacePanelToggleDisabled={workspacePanelExpandDisabled}
            workspaceTogglePressed={workspaceTogglePressed}
            workspacePanelLabel={workspacePanelLabel}
            workspacePanelTooltipLabel={workspacePanelTooltipLabel}
            onToggleSidebar={toggleSidebar}
            onToggleWorkspacePanel={toggleWorkspacePanel}
            onTabChange={(id) => void handleTabChange(id)}
            onTabClose={(id) => void handleTabClose(id)}
            onTabsClose={(ids, nextActiveTabId) => void handleTabsClose(ids, nextActiveTabId)}
            onTabsReorder={(ids) => void handleTabsReorder(ids)}
            onNewTab={() => void handleNewTab()}
            onOpenPalette={() => void openPalette()}
          />
        )}
        <a className="skip-to-composer" href={skipToProjectComposerAvailable || skipToComposerAvailable ? "#composer-input" : "#main-workspace"}>
          {t(skipToComposerAvailable || skipToProjectComposerAvailable ? "shortcuts.skipToComposer" : "shortcuts.skipToWorkspace")}
        </a>

        <aside className={sidebarClassName} aria-label={t("sidebar.navigation")}>
          {sidebarWorkbench ? (
            <>
              <div className="sidebar__head" aria-hidden={sidebarCollapsed}>
                <div className="sidebar__brand sidebar__brand--workbench">
                  <span className="sidebar__brand-name">{BRAND_DISPLAY_NAME}</span>
                </div>
              </div>

              <div className="sidebar__quick-actions">
                <button
                  className="sidebar__quick-action"
                  type="button"
                  onClick={() => {
                    void handleNewTab();
                  }}
                >
                  <MessageSquare size={18} aria-hidden="true" />
                  <span>{t("topbar.newSession")}</span>
                </button>
              </div>
            </>
          ) : (
            <>
              <div className="sidebar__brand" aria-hidden={sidebarCollapsed}>
                {sidebarCreation && !sidebarCollapsed ? (
                  <button
                    className="sidebar__brand-entry"
                    type="button"
                    onClick={() => {
                      void handleNewTab();
                    }}
                  >
                    <span className="sidebar__brand-name">{BRAND_DISPLAY_NAME}</span>
                    {sidebarAppVersion ? <span className="sidebar__brand-version">{sidebarAppVersion}</span> : null}
                  </button>
                ) : (
                  <span className="sidebar__brand-name">{BRAND_DISPLAY_NAME}</span>
                )}
                {sidebarCreation && !sidebarCollapsed ? (
                  <div className="sidebar__brand-actions" aria-label={t("sidebar.utilityActions")}>
                    <Tooltip label={t("history.centerTitle")} fill side="bottom">
                      <button
                        className="sidebar__brand-action sidebar__brand-action--history"
                        type="button"
                        aria-label={t("history.centerTitle")}
                        onClick={() => void openAllHistory()}
                      >
                        <History size={15} aria-hidden="true" />
                      </button>
                    </Tooltip>
                    <Tooltip label={sidebarToggleTitle} fill side="bottom">
                      <button
                        className={`sidebar__brand-action sidebar__brand-action--collapse${sidebarTogglePressed ? " sidebar__brand-action--pressed" : ""}`}
                        type="button"
                        onClick={toggleSidebar}
                        aria-label={sidebarToggleTitle}
                        aria-pressed={!sidebarCollapsed}
                      >
                        <PanelLeft size={15} aria-hidden="true" />
                      </button>
                    </Tooltip>
                  </div>
                ) : null}
              </div>

              <button
                className={`sidebar__new${sidebarCreation ? ` sidebar-feature-zone__item sidebar-feature-zone__item--new-chat${workspaceView === "chat" && !sessionHasContent && blankSessionPresentation ? " sidebar-feature-zone__item--active" : ""}` : ""}`}
                type="button"
                aria-pressed={sidebarCreation && workspaceView === "chat" && !sessionHasContent && blankSessionPresentation}
                onClick={() => {
                  void handleNewTab();
                }}
              >
                <SquarePen size={18} aria-hidden="true" />
                <span>{sidebarCreation ? t("creation.sidebar.newChat") : t("topbar.newSession")}</span>
              </button>
            </>
          )}

          {sidebarCreation && (
            <section className="sidebar-feature-zone" aria-label={t("settings.title")}>
              <div className="sidebar-feature-zone__items">
                {superpowersEnabled && <button
                  className={`sidebar-feature-zone__item${workspaceView === "superpowers" ? " sidebar-feature-zone__item--active" : ""}`}
                  type="button"
                  aria-pressed={workspaceView === "superpowers"}
                  onClick={openSuperpowerWorkspace}
                >
                  <Sparkles size={14} aria-hidden="true" />
                  <span>{t("sidebar.superpowers")}</span>
                </button>}
                <button
                  className={`sidebar-feature-zone__item${workspaceView === "projects" ? " sidebar-feature-zone__item--active" : ""}`}
                  type="button"
                  aria-pressed={workspaceView === "projects"}
                  onClick={openProjectDirectory}
                >
                  <FolderKanban size={14} aria-hidden="true" />
                  <span>{t("workspace.projects.nav")}</span>
                </button>
                <button
                  className={`sidebar-feature-zone__item sidebar-feature-zone__item--capabilities${["experts", "skills", "connectors"].includes(workspaceView) ? " sidebar-feature-zone__item--active" : ""}`}
                  type="button"
                  aria-pressed={["experts", "skills", "connectors"].includes(workspaceView)}
                  onClick={openExpertDirectory}
                >
                  <Users size={14} aria-hidden="true" />
                  <span className="sidebar-feature-zone__item-copy">
                    <span>{t("sidebar.experts")}</span>
                    <small>{t("creation.sidebar.capabilitiesHint")}</small>
                  </span>
                </button>
                <button
                  className={`sidebar-feature-zone__item${workspaceView === "automation" ? " sidebar-feature-zone__item--active" : ""}`}
                  type="button"
                  aria-pressed={workspaceView === "automation"}
                  onClick={openAutomationWorkspace}
                >
                  <AlarmClock size={14} aria-hidden="true" />
                  <span>{t("sidebar.automation")}</span>
                </button>
                <button
                  className={`sidebar-feature-zone__item${workspaceView === "artifacts" ? " sidebar-feature-zone__item--active" : ""}`}
                  type="button"
                  aria-pressed={workspaceView === "artifacts"}
                  onClick={openArtifactLibrary}
                >
                  <Library size={14} aria-hidden="true" />
                  <span>{t("sidebar.artifacts")}</span>
                </button>
              </div>
            </section>
          )}

          <section className="sidebar__section sidebar__section--projects">
            <ProjectTree
              activeScope={activeTab?.scope}
              activeWorkspaceRoot={activeTab?.workspaceRoot}
              activeTopicId={activeTab?.topicId}
              activeSessionPath={activeTab?.sessionPath}
              unreadCompletionIds={conversationUnreadCompletionIds}
              showSuperpowerSessions={superpowersEnabled}
              imTopicSources={imTopicSources}
              onBeginOpenTopicIntent={beginProjectTreeTopicOpenIntent}
              onCancelOpenTopicIntent={cancelPreparedTopicNavigation}
              onOpenTopic={handleOpenTopic}
              onCreateCloudProjectTopic={startNewCloudProjectConversation}
              onOpenProjectHistory={openProjectHistory}
              onCreateTopic={async (scope, workspaceRoot) => {
                showChatView(true);
                await openBlankSession(scope, scope === "project" ? workspaceRoot : "");
              }}
              onTopicsChanged={refreshProjectsAndTabs}
              onRenameTopic={renameTopic}
              refreshSignal={projectRevision}
              onAddProject={async () => {
                await switchFolder();
              }}
              timeFilter={topicTimeFilter}
              onTimeFilterChange={setTopicTimeFilter}
              variant={sidebarWorkbench ? "workbench" : sidebarCreation ? "creation" : "classic"}
              searchExpanded={!sidebarCreation}
              showShortcutBadges={showTopicBadges}
              shortcutPlatform={desktopPlatform}
              onVisibleTopicsChange={handleVisibleTopicsChange}
            />
          </section>

          {sidebarCreation && (
            <div className="sidebar-account" data-account-session={sidebarAccountHasSession ? "true" : "false"} ref={creationAccountMenuRef}>
              <button
                className={`sidebar-account-entry${workspaceView === "account" ? " sidebar-account-entry--active" : ""}`}
                type="button"
                aria-label={t("sidebar.account")}
                aria-haspopup="menu"
                aria-controls={creationAccountMenuOpen ? "creation-account-menu" : undefined}
                aria-expanded={creationAccountMenuOpen}
                onClick={toggleCreationAccountMenu}
              >
                <SidebarAccountAvatar src={sidebarAccountAvatarSrc} />
                <span className="sidebar-account-entry__label" title={sidebarAccountLabel} aria-live="polite">{sidebarAccountLabel}</span>
                <ChevronUp className="sidebar-account-entry__chevron" size={14} aria-hidden="true" />
              </button>
              {creationAccountMenuOpen ? (
                <div className="sidebar-account-menu" id="creation-account-menu" role="menu" aria-label={t("sidebar.account")}>
                  <div className="sidebar-account-menu__identity-wrap">
                    <button
                      className="sidebar-account-menu__identity"
                      type="button"
                      role="menuitem"
                      aria-current={workspaceView === "account" ? "page" : undefined}
                      onClick={openAccountCenter}
                    >
                      <SidebarAccountAvatar src={sidebarAccountAvatarSrc} menu />
                      <span className="sidebar-account-menu__identity-copy">
                        <strong title={sidebarAccountLabel}>
                          {sidebarAccountRequiresReauthentication ? t("account.reauthenticationTitle") : sidebarAccountLabel}
                        </strong>
                        <small>
                          {sidebarAccountRequiresReauthentication
                            ? t("account.reauthenticationAction")
                            : sidebarAccountPlanLabel || t("sidebar.account")}
                        </small>
                      </span>
                      <ChevronRight size={14} aria-hidden="true" />
                    </button>
                    <button
                      className="sidebar-account-menu__avatar-upload"
                      type="button"
                      role="menuitem"
                      aria-label={t("account.changeAvatar")}
                      onClick={() => accountAvatarInputRef.current?.click()}
                    >
                      <Camera size={13} aria-hidden="true" />
                    </button>
                    <input
                      ref={accountAvatarInputRef}
                      className="sr-only"
                      type="file"
                      accept="image/png,image/jpeg,image/webp"
                      tabIndex={-1}
                      onChange={handleAccountAvatarChange}
                    />
                  </div>
                  <div className="sidebar-account-menu__separator" role="separator" />
                  <button
                    className="sidebar-account-menu__item"
                    type="button"
                    role="menuitem"
                    onClick={openAccountCenter}
                  >
                    <Activity size={16} aria-hidden="true" />
                    <span className="sidebar-account-menu__item-copy">
                      <span>{t("account.totalBalance")}</span>
                      <small>{sidebarAccountDisplayBalance ?? "—"}</small>
                    </span>
                    <ChevronRight size={14} aria-hidden="true" />
                  </button>
                  <button
                    className="sidebar-account-menu__item"
                    type="button"
                    role="menuitem"
                    onClick={openAccountPayments}
                  >
                    <WalletCards size={16} aria-hidden="true" />
                    <span className="sidebar-account-menu__item-copy">
                      <span>{t("account.paymentsTab")}</span>
                      <small>{sidebarAccountPlanLabel || t("account.currentSubscription")}</small>
                    </span>
                    <ChevronRight size={14} aria-hidden="true" />
                  </button>
                  <div className="sidebar-account-menu__separator" role="separator" />
                  <button
                    className="sidebar-account-menu__item"
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      closeTransientOverlays();
                      setSettingsTarget("general");
                    }}
                  >
                    <SettingsIcon size={16} aria-hidden="true" />
                    <span>{t("settings.title")}</span>
                  </button>
                  <button
                    className="sidebar-account-menu__item"
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      closeTransientOverlays();
                      setSettingsTarget("appearance");
                    }}
                  >
                    <Palette size={16} aria-hidden="true" />
                    <span>{t("settings.appearance")}</span>
                  </button>
                  <button
                    className="sidebar-account-menu__item"
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      closeTransientOverlays();
                      setSettingsTarget("feedback");
                    }}
                  >
                    <CircleHelp size={16} aria-hidden="true" />
                    <span>{t("settings.tab.feedback")}</span>
                  </button>
                  <button
                    className="sidebar-account-menu__item"
                    type="button"
                    role="menuitem"
                    onClick={() => {
                      closeTransientOverlays();
                      setUpdateCenterOpen(true);
                    }}
                  >
                    <RefreshCw size={16} aria-hidden="true" />
                    <span>{t("settings.tab.updates")}</span>
                  </button>
                  {sidebarAccountHasSession ? (
                    <>
                      <div className="sidebar-account-menu__separator" role="separator" />
                      <button
                        className="sidebar-account-menu__item sidebar-account-menu__item--logout"
                        type="button"
                        role="menuitem"
                        data-account-menu-action="logout"
                        disabled={accountLogoutPending}
                        onClick={() => void requestAccountLogout()}
                      >
                        <LogOut size={16} aria-hidden="true" />
                        <span>{t("account.logout")}</span>
                      </button>
                    </>
                  ) : null}
                </div>
              ) : null}
            </div>
          )}

          {sidebarWorkbench ? (
            <nav className="sidebar__nav sidebar__nav--footer">
              <div className="sidebar__utility-row" aria-label={t("sidebar.utilityActions")}>
                <Tooltip label={t("sidebar.allHistory")} fill side="top">
                  <button
                    className="sidebar__utility-button"
                    type="button"
                    onClick={() => void openAllHistory()}
                  >
                    <History size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.allHistory")}</span>
                  </button>
                </Tooltip>
                <Tooltip label={t("sidebar.trash")} fill side="top">
                  <button
                    className="sidebar__utility-button"
                    type="button"
                    onClick={() => void openTrash()}
                  >
                    <Trash2 size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.trash")}</span>
                  </button>
                </Tooltip>
                <Tooltip label={t("heartbeat.scheduler")} fill side="top">
                  <button
                    className="sidebar__utility-button"
                    type="button"
                    onClick={() => setHeartbeatOpen(true)}
                  >
                    <AlarmClock size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.automation")}</span>
                  </button>
                </Tooltip>
                <Tooltip label={t("sidebar.artifacts")} fill side="top">
                  <button
                    className={`sidebar__utility-button${workspaceView === "artifacts" ? " sidebar__utility-button--active" : ""}`}
                    type="button"
                    aria-pressed={workspaceView === "artifacts"}
                    onClick={openArtifactLibrary}
                  >
                    <Library size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.artifacts")}</span>
                  </button>
                </Tooltip>
                <Tooltip label={t("sidebar.experts")} fill side="top">
                  <button
                    className={`sidebar__utility-button${workspaceView === "experts" ? " sidebar__utility-button--active" : ""}`}
                    type="button"
                    aria-pressed={workspaceView === "experts"}
                    onClick={openExpertDirectory}
                  >
                    <Users size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.experts")}</span>
                  </button>
                </Tooltip>
                {superpowersEnabled && <Tooltip label={t("sidebar.superpowers")} fill side="top">
                  <button
                    className={`sidebar__utility-button${workspaceView === "superpowers" ? " sidebar__utility-button--active" : ""}`}
                    type="button"
                    aria-pressed={workspaceView === "superpowers"}
                    onClick={openSuperpowerWorkspace}
                  >
                    <Sparkles size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.superpowers")}</span>
                  </button>
                </Tooltip>}
                <Tooltip label={t("sidebar.account")} fill side="top">
                  <button
                    className={`sidebar__utility-button${workspaceView === "account" ? " sidebar__utility-button--active" : ""}`}
                    type="button"
                    aria-pressed={workspaceView === "account"}
                    onClick={openAccountCenter}
                  >
                    <CircleUserRound size={16} aria-hidden="true" />
                    <span className="sr-only">{t("sidebar.account")}</span>
                  </button>
                </Tooltip>
                <Tooltip label={t("topbar.settings")} fill side="top">
                  <button
                    className="sidebar__utility-button"
                    type="button"
                    onClick={() => {
                      closeTransientOverlays();
                      setSettingsTarget("general");
                    }}
                  >
                    <SettingsIcon size={16} aria-hidden="true" />
                    <span className="sr-only">{t("topbar.settings")}</span>
                  </button>
                </Tooltip>
              </div>
            </nav>
          ) : !sidebarCreation ? (
            <nav className="sidebar__nav">
              <Tooltip label={t("sidebar.allHistory")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                <button
                  className="sidebar__navitem"
                  type="button"
                  aria-label={t("sidebar.allHistory")}
                  onClick={() => void openAllHistory()}
                >
                  <History size={15} aria-hidden="true" />
                  <span>{t("sidebar.allHistory")}</span>
                </button>
              </Tooltip>
              <Tooltip label={t("sidebar.trash")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                <button
                  className="sidebar__navitem"
                  type="button"
                  aria-label={t("sidebar.trash")}
                  onClick={() => void openTrash()}
                >
                  <Trash2 size={15} aria-hidden="true" />
                  <span>{t("sidebar.trash")}</span>
                </button>
              </Tooltip>
              <Tooltip label={t("sidebar.artifacts")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                <button
                  className={`sidebar__navitem${workspaceView === "artifacts" ? " sidebar__navitem--active" : ""}`}
                  type="button"
                  aria-label={t("sidebar.artifacts")}
                  aria-pressed={workspaceView === "artifacts"}
                  onClick={openArtifactLibrary}
                >
                  <Library size={15} aria-hidden="true" />
                  <span>{t("sidebar.artifacts")}</span>
                </button>
              </Tooltip>
              {!sidebarCreation && (
                <Tooltip label={t("heartbeat.scheduler")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                  <button
                    className="sidebar__navitem"
                    onClick={() => setHeartbeatOpen(true)}
                  >
                    <AlarmClock size={15} />
                    <span>{t("sidebar.automation")}</span>
                  </button>
                </Tooltip>
              )}
              {!sidebarCreation && superpowersEnabled && (
                <Tooltip label={t("sidebar.superpowers")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                  <button
                    className={`sidebar__navitem${workspaceView === "superpowers" ? " sidebar__navitem--active" : ""}`}
                    aria-pressed={workspaceView === "superpowers"}
                    onClick={openSuperpowerWorkspace}
                  >
                    <Sparkles size={15} />
                    <span>{t("sidebar.superpowers")}</span>
                  </button>
                </Tooltip>
              )}
              {!sidebarCreation && (
                <Tooltip label={t("sidebar.experts")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                  <button
                    className={`sidebar__navitem${workspaceView === "experts" ? " sidebar__navitem--active" : ""}`}
                    aria-pressed={workspaceView === "experts"}
                    onClick={openExpertDirectory}
                  >
                    <Users size={15} />
                    <span>{t("sidebar.experts")}</span>
                  </button>
                </Tooltip>
              )}
              {!sidebarCreation && (
                <Tooltip label={t("sidebar.account")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                  <button
                    className={`sidebar__navitem${workspaceView === "account" ? " sidebar__navitem--active" : ""}`}
                    aria-pressed={workspaceView === "account"}
                    onClick={openAccountCenter}
                  >
                    <CircleUserRound size={15} />
                    <span>{t("sidebar.account")}</span>
                  </button>
                </Tooltip>
              )}
              <Tooltip label={t("topbar.settings")} fill side="right" disabled={sidebarNavTooltipDisabled}>
                <button
                  className="sidebar__navitem"
                  type="button"
                  aria-label={t("topbar.settings")}
                  onClick={() => {
                    closeTransientOverlays();
                    setSettingsTarget("general");
                  }}
                >
                  <SettingsIcon size={15} aria-hidden="true" />
                  <span>{t("topbar.settings")}</span>
                </button>
              </Tooltip>
            </nav>
          ) : null}

        </aside>
        {creationSidebarFloating && !sidebarCollapsed && (
          <div className="sidebar-compact-backdrop" role="presentation" onClick={closeNarrowSidebar} />
        )}
        <button
          className="sidebar-resizer"
          type="button"
          role="separator"
          aria-orientation="vertical"
          aria-label={t("sidebar.resize")}
          aria-valuemin={sidebarResizeMinWidth}
          aria-valuemax={SIDEBAR_MAX_WIDTH}
          aria-valuenow={sidebarRenderWidth}
          onPointerDown={startSidebarResize}
          onKeyDown={resizeSidebarWithKeyboard}
          onDoubleClick={() => setExpandedSidebarWidth(defaultSidebarWidth())}
        />
        {sidebarCreation && sidebarCollapsed && (
          <button
            className={`sidebar-expand-toggle${sidebarTogglePressed ? " sidebar-expand-toggle--pressed" : ""}`}
            type="button"
            onClick={toggleSidebar}
            aria-label={sidebarToggleTitle}
            aria-pressed={!sidebarCollapsed}
            title={sidebarToggleTitle}
          >
            <PanelRight size={15} aria-hidden="true" />
          </button>
        )}

        <section className={`chat-pane${sidebarCreation && workspaceView === "chat" && !sessionHasContent && blankSessionPresentation ? " chat-pane--creation-empty" : ""}`}>
          <>
          {topicbarVisible && (
          <header className="topicbar">
            {workbenchChromeHidden && (
              <Tooltip label={sidebarToggleTitle}>
                <button
                  className={[
                    "topicbar__chrome-btn",
                    sidebarExpandBlocked ? "topicbar__chrome-btn--blocked" : "",
                    sidebarTogglePressed ? "topicbar__chrome-btn--pressed" : "",
                  ].filter(Boolean).join(" ")}
                  type="button"
                  onClick={sidebarExpandBlocked ? undefined : toggleSidebar}
                  aria-label={sidebarToggleTitle}
                  aria-pressed={!sidebarCollapsed}
                  aria-disabled={sidebarExpandBlocked}
                >
                  <PanelLeft size={15} />
                </button>
              </Tooltip>
            )}
            <div className="topicbar__identity">
              {hideCreationTopicTitle ? null : (
                <div className="topicbar__title-row">
                {topicbarEditing ? (
                  <div className="topicbar__title-edit">
                    <input
                      autoFocus
                      className="topicbar__title-input"
                      aria-label={t("topicBar.renameSession")}
                      size={sidebarCreation ? topicbarTitleEditSize : undefined}
                      value={topicTitleDraft}
                      onChange={(event) => setTopicTitleDraft(event.target.value)}
                      onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
                        if (event.key === "Enter") {
                          event.preventDefault();
                          void commitActiveTopicRename();
                        }
                        if (event.key === "Escape") {
                          event.preventDefault();
                          cancelActiveTopicRename();
                        }
                      }}
                      onBlur={() => void commitActiveTopicRename()}
                    />
                  </div>
                ) : sidebarCreation && topicbarCanRename ? (
                  <h1 title={topicTitle(activeTab)}>
                    <button
                      className="topicbar__title-button"
                      type="button"
                      onClick={startActiveTopicRename}
                      aria-label={t("topicBar.renameSession")}
                    >
                      {topicbarTitle}
                    </button>
                  </h1>
                ) : (
                  <h1 title={sidebarImDetailConnection ? topicbarTitle : topicTitle(activeTab)}>{topicbarTitle}</h1>
                )}
                {!sidebarCreation && (
                  <Tooltip label={t("topicBar.renameSession")}>
                    <button
                      className="topicbar__icon-btn"
                      type="button"
                      disabled={!topicbarCanRename || topicbarEditing}
                      onClick={startActiveTopicRename}
                      aria-label={t("topicBar.renameSession")}
                    >
                      <Pencil size={14} />
                    </button>
                  </Tooltip>
                )}
                </div>
              )}
              {topicbarSubtitleVisible && (
                <div className="topicbar__subtitle" title={topicbarSubtitleTitle}>
                  {topicbarWorkspaceLabel && <span>{topicbarWorkspaceLabel}</span>}
                  {topicbarImSourcePlatform && (
                    <span className={`topicbar__source-chip topicbar__source-chip--${topicbarImSourcePlatform}`}>
                      {topicbarImSourceLabel}
                    </span>
                  )}
                </div>
              )}
            </div>
            <div className="topicbar__spacer" />
            <div className="topicbar__actions">
              {workspacePanelAvailable && workbenchChromeHidden && (
                <Tooltip label={workspacePanelTooltipLabel} showOnFocus={false}>
                  <button
                    className={[
                      "topicbar__chrome-btn",
                      "topicbar__chrome-btn--workspace",
                      workspacePanelRenderable ? "topicbar__chrome-btn--active" : "",
                      workspacePanelExpandDisabled ? "topicbar__chrome-btn--blocked" : "",
                      workspaceTogglePressed ? "topicbar__chrome-btn--pressed" : "",
                    ].filter(Boolean).join(" ")}
                    type="button"
                    disabled={workspacePanelExpandDisabled}
                    onClick={workspacePanelExpandDisabled ? undefined : toggleWorkspacePanel}
                    aria-label={workspacePanelLabel}
                    aria-pressed={workspacePanelRenderable}
                  >
                    <PanelRight size={15} />
                  </button>
                </Tooltip>
              )}
              {!sidebarImDetailConnection && (
              <>
              <Tooltip label={t("topicBar.copyAll")}>
                <CopyButton
                  getText={getSessionMarkdown}
                  label={t("topicBar.copyAll")}
                  className="topicbar__action-btn topicbar__action-btn--icon topicbar__action-btn--utility"
                  showInlineLabel={false}
                />
              </Tooltip>
              <div className={`topicbar__export${topicExportOpen ? " topicbar__export--open" : ""}`}>
                <Tooltip label={t("topicBar.export")}>
                  <button
                    className="topicbar__action-btn topicbar__action-btn--icon topicbar__action-btn--utility"
                    type="button"
                    disabled={!sessionHasContent}
                    aria-label={t("topicBar.export")}
                    aria-haspopup="menu"
                    aria-expanded={topicExportOpen}
                    onClick={() => setTopicExportOpen((open) => !open)}
                  >
                    <Download size={14} />
                  </button>
                </Tooltip>
                {topicExportOpen && (
                  <div className="topicbar__export-menu" role="menu">
                    <button type="button" role="menuitem" onClick={() => void exportSession("markdown")}>
                      <FileText size={13} />
                      <span>{t("topicBar.exportMarkdown")}</span>
                    </button>
                    <button type="button" role="menuitem" onClick={() => void exportSession("json")}>
                      <FileJson size={13} />
                      <span>{t("topicBar.exportJson")}</span>
                    </button>
                    <button type="button" role="menuitem" onClick={() => void exportSession("pdf")}>
                      <FileDown size={13} />
                      <span>{t("topicBar.exportPdf")}</span>
                    </button>
                    <button type="button" role="menuitem" onClick={() => void exportSession("image")}>
                      <FileImage size={13} />
                      <span>{t("topicBar.exportImage")}</span>
                    </button>
                  </div>
                )}
              </div>
              </>
              )}
              <Tooltip label={t("shortcuts.cheatsheetTitle")}>
                <button
                  className="topicbar__action-btn topicbar__action-btn--icon topicbar__action-btn--utility"
                  type="button"
                  aria-label={t("shortcuts.cheatsheetTitle")}
                  onClick={() => {
                    closeTransientOverlays();
                    setShortcutsOpen(true);
                  }}
                >
                  <CircleHelp size={14} />
                </button>
              </Tooltip>
              {!sidebarCreation && (
                <Tooltip label={t("topicBar.command")}>
                  <button
                    className="topicbar__action-btn topicbar__action-btn--label topicbar__action-btn--accent"
                    type="button"
                    aria-label={t("topicBar.command")}
                    onClick={() => void openPalette()}
                  >
                    <Command size={14} />
                    <span>{t("topicBar.command")}</span>
                  </button>
                </Tooltip>
              )}
              {workspacePanelAvailable && sidebarCreation && (
                <Tooltip label={workspacePanelTooltipLabel} showOnFocus={false}>
                  <button
                    className={[
                      "topicbar__chrome-btn",
                      "topicbar__chrome-btn--workspace",
                      workspacePanelRenderable ? "topicbar__chrome-btn--active" : "",
                      workspacePanelExpandDisabled ? "topicbar__chrome-btn--blocked" : "",
                      workspaceTogglePressed ? "topicbar__chrome-btn--pressed" : "",
                    ].filter(Boolean).join(" ")}
                    type="button"
                    disabled={workspacePanelExpandDisabled}
                    onClick={workspacePanelExpandDisabled ? undefined : toggleWorkspacePanel}
                    aria-label={workspacePanelLabel}
                    aria-pressed={workspacePanelRenderable}
                  >
                    <PanelRight size={15} />
                  </button>
                </Tooltip>
              )}
            </div>
          </header>
          )}

          {workspaceView === "chat" && (
            <>
              <SendAvailabilityBanner
                availability={displayedSendAvailability}
                hasHistory={hasLocalHistory}
                onLogin={openAccountCenter}
                onOpenAccount={openAccountCenter}
                onOpenPayments={openAccountPayments}
                onConfigureBYOK={() => setSettingsTarget("models")}
                onRetry={async () => {
                  await retryTabStartup();
                  await refreshTabMetas();
                }}
                onSwitchModel={openModelMarketplace}
              />
              {displayedSendAvailability?.state === "ready" && (
                <SubscriptionModelGateBanner
                  access={subscriptionModelInvocationAccess}
                  modelCatalog={sharedModelCatalog}
                  subscriptionAccess={subscriptionAccess}
                  onSwitchModel={switchModel}
                  onOpenModelPicker={openModelMarketplace}
                />
              )}

              <HistoryLoadErrorBanner error={state.hydrateError} cached={Boolean(state.hydratePreview)} onRetry={surfaceTransitionActive ? undefined : () => void retryHistoryLoad()} />
            </>
          )}

          <UpdateBanner enabled={startupUpdateChecksEnabled === true} />

          <main ref={mainWorkspaceRef} id="main-workspace" className="main" tabIndex={-1}>
            {sidebarImDetailConnection ? (
              <SidebarImConnectionDetail
                connection={sidebarImDetailConnection}
                onClose={() => setSidebarImDetailConnectionId("")}
                onOpenSettings={openBotSettings}
                onManageAllowlist={() => openBotAllowlistSettings(sidebarImDetailConnection.connectionId)}
                onOpenSession={() => void openSidebarImConnectionSession(sidebarImDetailConnection)}
              />
            ) : workspaceView === "superpowers" ? (
              <Suspense fallback={<div className="primary-workspace__empty">{t("common.loading")}</div>}>
                <SuperpowerWorkspace
                  sessionAdapter={superpowerSessionAdapter}
                  renderSessionTranscript={renderSessionTranscript}
                  renderOperationalPrompt={renderSessionOperationalPrompt}
                  renderComposer={(onSend) => renderSessionComposer(onSend, activeComposerCapabilities.connectors.filter((name) => name !== "shopify-global-catalog"))}
                  renderSessionStatusBar={renderSessionStatusBar}
                  initialScreen={superpowerEntry.screen}
                  navigationKey={superpowerEntry.key}
                  onCapabilityBindingChanged={handleCapabilityBindingChanged}
                />
              </Suspense>
            ) : workspaceView === "automation" ? (
              <div className="capability-workspace-shell capability-workspace-shell--automation">
                {projectCapabilityOrigin ? <header className="capability-workspace-header"><button type="button" className="capability-workspace-header__project-return" onClick={() => returnToProjectCapabilityOrigin()}><FolderKanban size={15} aria-hidden="true" />返回项目</button></header> : null}
                <Suspense fallback={<div className="primary-workspace__empty">{t("common.loading")}</div>}>
                  <HeartbeatPanel
                    open
                    embedded
                    authenticated={sidebarAccountHasSession}
                    onConfigureModel={() => setSettingsTarget("models")}
                    modelCatalog={automationModelCatalog}
                    modelAccountScopeKey={modelCatalogAccountScopeKey}
                    modelSettingsRevision={automationModelSettingsRevision}
                    subscriptionAccess={subscriptionAccess}
                    onClose={() => projectCapabilityOrigin ? returnToProjectCapabilityOrigin() : showChatView(false)}
                    onOpenTopic={(scope, workspaceRoot, topicId, sessionPath) => {
                      void handleOpenTopic(scope, workspaceRoot, topicId, sessionPath);
                    }}
                  />
                </Suspense>
              </div>
            ) : workspaceView === "artifacts" ? (
              <Suspense fallback={<div className="primary-workspace__empty">{t("common.loading")}</div>}>
                <ArtifactLibraryPage onOpenSource={onResumeSession} accountScopeKey={projectAccountScopeKey} refreshSignal={projectRevision} />
              </Suspense>
            ) : workspaceView === "projects" ? (
              <Suspense fallback={<div className="primary-workspace__empty">{t("common.loading")}</div>}>
                <ProjectDirectory
                  renderProjectComposer={renderProjectComposer}
                  onOpenProjectCapability={openProjectCapability}
                  projectExperts={projectCapabilityExperts}
                  selectedCloudProjectId={selectedCloudProjectId}
                  onSelectedCloudProjectChange={setSelectedCloudProjectId}
                  restoreProjectCapabilityFocus={restoreProjectCapabilityFocus}
                  onRestoreProjectCapabilityFocusConsumed={() => setRestoreProjectCapabilityFocus(null)}
                  onProjectComposerLeave={leaveEmptyProjectComposer}
                  onOpenPayments={openAccountPayments}
                />
              </Suspense>
            ) : workspaceView === "skills" || workspaceView === "connectors" || workspaceView === "experts" ? (
              <div className={`capability-workspace-shell capability-workspace-shell--${workspaceView}`}>
                <div className="capability-workspace-shell__topbar">
                  <header className="capability-workspace-header">
                    <CapabilityWorkspaceNav active={workspaceView} onSelect={openCapabilityWorkspace} />
                    {projectCapabilityOrigin ? <button type="button" className="capability-workspace-header__project-return" onClick={() => returnToProjectCapabilityOrigin()}><FolderKanban size={15} aria-hidden="true" />返回项目</button> : null}
                  </header>
                  <div className="capability-workspace-shell__toolbar">
                    <div id="capability-workspace-header-actions" className="capability-workspace-header__actions" />
                  </div>
                </div>
                <div key={workspaceView} className="capability-workspace-shell__content">
                  {workspaceView === "skills" ? (
                    <section className="primary-workspace primary-workspace--capabilities primary-workspace--skills" aria-labelledby="skills-workspace-title">
                      <h1 id="skills-workspace-title" className="sr-only">{t("workspace.skills.title")}</h1>
                      <Suspense fallback={<CapabilityWorkspaceLoading view={workspaceView} />}>
                        <SkillsWorkspace workspace onSelectSkill={selectWorkspaceSkill} />
                      </Suspense>
                    </section>
                  ) : workspaceView === "connectors" ? (
                    <section className="primary-workspace primary-workspace--capabilities primary-workspace--connectors" aria-labelledby="connectors-workspace-title">
                      <h1 id="connectors-workspace-title" className="sr-only">{t("workspace.connectors.title")}</h1>
                      <Suspense fallback={<CapabilityWorkspaceLoading view={workspaceView} />}>
                        <ConnectorsWorkspace
                          workspace
                          targetTabID={activeTabId}
                          selectedServers={activeComposerCapabilities.connectors}
                          onSelectServer={selectWorkspaceConnector}
                          onTryConnector={tryWorkspaceConnector}
                          onBotSettingsChanged={() => {
                            void reloadSidebarImConnections().catch((e) => console.warn("bot sidebar refresh failed", e));
                          }}
                        />
                      </Suspense>
                    </section>
                  ) : (
                    <Suspense fallback={<CapabilityWorkspaceLoading view={workspaceView} />}>
                      <ExpertDirectory
                        onSummon={summonWorkspaceExpert}
                        authenticated={sidebarAccountHasSession}
                        workspaceHeaderActionsId="capability-workspace-header-actions"
                        backendExperts={expertCatalog}
                        userExperts={userExpertCatalog}
                        expertLibraryState={expertLibraryState}
                        onSaveUserExpert={saveUserExpert}
                        onSaveCatalogExpert={saveCatalogExpert}
                        onDuplicateExpert={duplicateExpert}
                        onToggleFavorite={toggleExpertFavorite}
                        onImportExpertConfig={importExpertConfig}
                        onDeleteUserExpert={deleteUserExpert}
                        onDeleteCatalogExpert={deleteCatalogExpert}
                      />
                    </Suspense>
                  )}
                </div>
              </div>
            ) : workspaceView === "account" ? (
              <Suspense fallback={<div className="primary-workspace__empty">{t("common.loading")}</div>}>
                <AccountCenter
                  avatarSrc={sidebarAccountAvatarSrc}
                  contextKey={DESKTOP_ACCOUNT_CONTEXT_KEY}
                  modelCatalog={sharedModelCatalog}
                  request={accountCenterRequest}
                  onAuthenticated={refreshAccountAfterAuthentication}
                />
              </Suspense>
            ) : renderSessionTranscript()}
          </main>

          {!sidebarImDetailConnection && workspaceView === "chat" && (
          <footer className="footer" ref={footerRef}>
            {showTodos ? <TodoPanel todos={todos} active={todoActive} onDismiss={() => setDismissedTodo(todoKey)} /> : null}
            {rewindState && (
              <UndoRewindBanner
                meta={{
                  turns: rewindState.turnDiff,
                  filesRestored: [], // optimistic: files haven't changed yet
                  filesRemoved: [],
                  onUndo: () => {
                    setRewindState(null);
                    setComposerInsertRequest({ id: Date.now(), text: "", mode: "replace" });
                  },
                }}
              />
            )}
            {renderSessionOperationalPrompt()}
            {clearContextPending && (
              <ClearContextCard
                onCancel={cancelClearContext}
                onConfirm={() => {
                  void confirmClearContext();
                }}
              />
            )}
            {expertTeamSwitchConfirm && (
              <div className="expert-switch-confirm-backdrop" role="presentation" onMouseDown={cancelExpertTeamSwitch}>
                <div
                  className="expert-switch-confirm"
                  role="dialog"
                  aria-modal="true"
                  aria-labelledby="expert-switch-confirm-title"
                  onMouseDown={(event) => event.stopPropagation()}
                >
                  <h2 id="expert-switch-confirm-title">{t("expertSwitchConfirm.title")}</h2>
                  <p>{t("expertSwitchConfirm.body")}</p>
                  <div className="expert-switch-confirm__actions">
                    <button type="button" className="expert-switch-confirm__btn expert-switch-confirm__btn--cancel" onClick={cancelExpertTeamSwitch}>
                      {t("common.cancel")}
                    </button>
                    <button type="button" className="expert-switch-confirm__btn expert-switch-confirm__btn--primary" onClick={() => void confirmExpertTeamSwitch()}>
                      {t("expertSwitchConfirm.confirm")}
                    </button>
                  </div>
                </div>
              </div>
            )}
            <div className={`session-composer-region${expertMemberHistoryFocused ? " session-composer-region--history" : ""}`}>
              <div className="session-composer-slot">
                <div className="session-composer-content" inert={expertMemberHistoryFocused ? true : undefined} aria-hidden={expertMemberHistoryFocused || undefined}>
                  {renderSessionComposer()}
                </div>
                {expertMemberHistoryFocused && (
                  <div className="session-composer-history-overlay">
                    <button type="button" className="agent-workspace__return" onClick={() => expertMemberHistoryReturnRef.current()}>
                      <span aria-hidden="true">↩</span>
                      <span>{t("agentWorkspace.returnMain")}</span>
                    </button>
                  </div>
                )}
              </div>
              {runtimeConnectionStage === "stalled" && (
                <div
                  className="session-runtime-notice session-runtime-notice--stalled"
                  role="status"
                  aria-live="polite"
                >
                  <span>{t("session.runtimeStalled")}</span>
                  {(displayedSendAvailability?.retryable || Boolean(state.meta?.startupErr)) && (
                    <button
                      className="btn btn--small"
                      type="button"
                      onClick={() => void retryTabStartup()}
                    >
                      {t("common.retry")}
                    </button>
                  )}
                </div>
              )}
            </div>
            {renderSessionStatusBar()}
          </footer>
          )}
          </>
        </section>

        {workspacePanelGridOpen && (
          <button
            className="workspace-panel-resizer"
            type="button"
            role="separator"
            aria-orientation="vertical"
            aria-label={t("rightDock.resize")}
            aria-valuemin={workspacePanelResizeMinWidth}
            aria-valuemax={Math.max(workspacePanelMaxWidth, workspacePanelRenderWidth)}
            aria-valuenow={workspacePanelRenderWidth}
            onPointerDown={startWorkspacePanelResize}
            onKeyDown={resizeWorkspacePanelWithKeyboard}
            onDoubleClick={() => setSavedWorkspacePanelWidth(workspacePanelResetWidth)}
          />
        )}

        {workspacePanelRenderable && (
          <aside
            className={[
              "workbench-dock",
              `workbench-dock--${rightDockMode}`,
            ].join(" ")}
            aria-label={t("rightDock.workbench")}
          >
            <div className="workbench-dock__tools">
              <div className="workbench-dock__tabs" role="tablist" aria-label={t("rightDock.views")}>
                {SHOW_CONTEXT_DOCK && (
                  <button
                    type="button"
                    role="tab"
                    aria-selected={rightDockMode === "context"}
                    className={`workbench-dock__tab${rightDockMode === "context" ? " workbench-dock__tab--active" : ""}`}
                    onClick={() => openRightDockMode("context")}
                  >
                    <Activity size={13} />
                    <span className="workbench-dock__tab-label">{t("rightDock.overview")}</span>
                  </button>
                )}
                <button
                  type="button"
                  role="tab"
                  aria-selected={rightDockMode === "files"}
                  className={`workbench-dock__tab${rightDockMode === "files" ? " workbench-dock__tab--active" : ""}`}
                  onClick={() => openRightDockMode("files")}
                >
                  <FileText size={13} />
                  <span className="workbench-dock__tab-label">{t("workspace.filesTab")}</span>
                </button>
              </div>
            </div>
            <div className="workbench-dock__body">
              {rightDockMode === "context" ? (
                <ContextPanel
                  tabId={activeTabId}
                  context={state.context}
                  usage={state.usage}
                  sessionTokens={state.sessionTokens}
                  sessionCost={state.sessionCost}
                  sessionCurrency={state.sessionCurrency}
                  sessionTurns={sessionTurns}
                  turnTokens={state.turnTotalTokens}
                  turnCost={state.turnCost}
                  balance={state.balance}
                  sessionGen={state.sessionGen}
                  refreshKey={dockRefreshKey}
                  activeExpert={displayedActiveExpert}
                />
              ) : (
                <WorkspacePanel
                  open={workspacePanelRenderable}
                  tabId={activeTabId}
                  cwd={state.meta?.cwd}
                  maximized={workspacePanelMaximized}
                  panelWidth={workspacePanelRenderWidth}
                  onClose={() => setWorkspacePanel(false)}
                  onToggleMaximized={() => {
                    closeTransientOverlays();
                    updateWorkspacePanelState((current) => ({
                      ...current,
                      maximized: !current.maximized,
                    }));
                  }}
                  onPreviewModeChange={handleWorkspacePreviewModeChange}
                  onAddToChat={addWorkspaceTextToComposer}
                  onRequestPanelWidth={ensureWorkspacePanelWidth}
                  refreshKey={dockRefreshKey}
                  initialViewMode="files"
                  showViewTabs={false}
                />
              )}
            </div>
          </aside>
        )}
      </div>

      {histView !== null && (
        <Suspense fallback={null}>
          <HistoryPanel
            kind={histView.kind}
            sessions={histView.sessions}
            trashedSessions={histView.trashedSessions}
            initialWorkspaceRoot={histView.initialWorkspaceRoot}
            running={state.running}
            onResume={onResumeSession}
            onPreview={previewSession}
            onDelete={onDeleteSession}
            onRename={onRenameSession}
            onRestore={onRestoreTrashedSession}
            onPurge={onPurgeTrashedSession}
            onPurgeAll={onPurgeAllTrashedSessions}
            onKindChange={(kind) => setHistView((current) => current === null ? null : { ...current, kind })}
            onClose={closeHistory}
          />
        </Suspense>
      )}

      {settingsTarget !== null && (
        <Suspense fallback={null}>
          <SettingsPanel
            initialTab={settingsTarget}
            modelCatalog={sharedModelCatalog}
            subscriptionAccess={subscriptionAccess}
            subscriptionAccountState={subscriptionAccountState}
            feedbackAccountState={feedbackAccountState}
            feedbackAccountScopeKey={feedbackAccountScopeKey}
            onRequireAccount={openAccountCenter}
            initialFocus={settingsFocus ?? undefined}
            activeTabId={activeTabId}
            settingsScopeKey={settingsScopeKey}
            agentRunning={state.running}
            superpowersEnabled={superpowersEnabled}
            onSuperpowersEnabledChange={handleSuperpowersEnabledChange}
            onClose={() => {
              setSettingsFocus(null);
              setSettingsTarget(null);
            }}
            onChanged={(settings) => {
              void refreshMeta();
              refreshSubscriptionRepairFactsForSettings(settings);
              if (settings) {
                applyDesktopPreferences(settings);
                void refreshSidebarImConnectionsFromSettings(settings).catch((e) => console.warn("bot sidebar refresh failed", e));
                return;
              }
              void reloadSidebarImConnections().catch((e) => console.warn("bot sidebar refresh failed", e));
              void app.DesktopStartupSettings()
                .then(applyDesktopPreferences)
                .catch((e) => console.warn("desktop preferences refresh failed", e));
            }}
          />
        </Suspense>
      )}

      {projectCapabilityOrigin && workspaceView !== "connectors" && (
        <ProjectCapabilityDialog
          kind={projectCapabilityOrigin.kind}
          projectId={projectCapabilityOrigin.projectId}
          gateway={projectGateway}
          experts={projectCapabilityExperts}
          onClose={() => setProjectCapabilityOrigin(null)}
          onApplied={(items) => {
            const origin = projectCapabilityOrigin;
            if (!origin) return;
            origin.onCapabilitiesChanged?.(items);
            updateProjectComposerAdapter(origin.sessionKey, (adapter) => {
              applyProjectCapabilityBindings(adapter, items);
            });
          }}
          onCatalogLoaded={(kind, items) => projectCapabilityOrigin.onCapabilityCatalogChanged?.(kind, items)}
        />
      )}

      {updateCenterOpen && (
        <Suspense fallback={null}>
          <UpdateCenterDialog onClose={() => setUpdateCenterOpen(false)} />
        </Suspense>
      )}

      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        items={paletteItems}
        placeholder={t("palette.placeholder")}
        emptyText={t("palette.empty")}
      />

      <ShortcutsCheatsheet
        open={shortcutsOpen}
        platform={desktopPlatform}
        onClose={() => setShortcutsOpen(false)}
        t={t}
      />

      {startupSplashVisible && (
        <StartupSplash hold={startupSplashHold} onDone={() => setStartupSplashVisible(false)} />
      )}

      <Suspense fallback={null}>
        <HeartbeatPanel open={heartbeatOpen} authenticated={sidebarAccountHasSession} onConfigureModel={() => setSettingsTarget("models")} modelCatalog={automationModelCatalog} modelAccountScopeKey={modelCatalogAccountScopeKey} modelSettingsRevision={automationModelSettingsRevision} subscriptionAccess={subscriptionAccess} onClose={() => setHeartbeatOpen(false)} onOpenTopic={(scope, workspaceRoot, topicId, sessionPath) => {
          void handleOpenTopic(scope, workspaceRoot, topicId, sessionPath);
        }} />
      </Suspense>
    </div>
    </ShellExpandProvider>
  );
}
