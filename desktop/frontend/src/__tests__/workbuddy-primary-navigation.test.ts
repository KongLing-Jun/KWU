import { loadStylesSource } from "./helpers/styles";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

const here = dirname(fileURLToPath(import.meta.url));
const source = (path: string) => readFileSync(resolve(here, path), "utf8");
const appSource = source("../App.tsx");
const workspaceNavigationSource = source("../app/useWorkspaceNavigation.ts");
const appNavigationSource = `${appSource}\n${workspaceNavigationSource}`;
const accountSidebarSource = source("../features/account/controller/useAccountSidebar.ts");
const sessionControllerSource = source("../features/session/useController.ts");
const composerSource = source("../features/composer/ui/Composer.tsx");
const projectSource = `${source("../features/workspace/ui/ProjectDirectory.tsx")}\n${source("../features/workspace/ui/ProjectDirectoryImpl.tsx")}`;
const cloudProjectSource = source("../features/projects/ui/CloudProjectsSectionImpl.tsx");
const projectTreeSource = source("../features/workspace/ui/ProjectTree.tsx");
const capabilitiesSource = source("../components/CapabilitiesPanel.tsx");
const feishuConnectorSource = source("../features/connectors/ui/FeishuBotConnector.tsx");
const automationSource = source("../features/automation/heartbeat/HeartbeatPanel.tsx");
const automationStyles = source("../features/automation/heartbeat/heartbeat.css");
const settingsPanelSource = source("../features/settings/ui/SettingsPanel.tsx");
const botsSettingsSource = source("../features/settings/ui/BotsSettingsSection.tsx");
const zhSource = source("../locales/zh.ts");
const styles = loadStylesSource();
const creationAccountMenuStart = appSource.indexOf('<div className="sidebar-account-menu"');
const creationAccountMenuSource = appSource.slice(
  creationAccountMenuStart,
  appSource.indexOf("{sidebarWorkbench ? (", creationAccountMenuStart),
);
const creationBrandStart = appSource.indexOf('className="sidebar__brand-entry"');
const creationBrandSource = appSource.slice(
  creationBrandStart,
  appSource.indexOf('className={`sidebar__new', creationBrandStart),
);

for (const view of ["projects", "automation"]) {
  ok(new RegExp(`type WorkspaceView = [^;]*"${view}"`).test(workspaceNavigationSource), `workspace route includes ${view}`);
  ok(appNavigationSource.includes(`setWorkspaceView("${view}")`), `sidebar handler opens ${view}`);
  ok(appSource.includes(`workspaceView === "${view}" ? (`), `main area renders ${view}`);
  ok(appSource.includes(`aria-pressed={workspaceView === "${view}"}`), `${view} navigation exposes its selected state`);
}

for (const view of ["experts", "skills", "connectors"]) {
  ok(new RegExp(`type WorkspaceView = [^;]*"${view}"`).test(workspaceNavigationSource), `workspace route includes ${view}`);
  ok(appNavigationSource.includes(`setWorkspaceView("${view}")`), `capability navigation opens ${view}`);
}

ok(
  appSource.includes("CAPABILITY_WORKSPACE_NAV_ITEMS") &&
    appSource.includes('className="capability-workspace-nav"') &&
    appSource.includes('aria-current={selected ? "page" : undefined}') &&
    styles.includes(".capability-workspace-nav__item:focus-visible") &&
    styles.includes(".capability-workspace-nav__item:not(.capability-workspace-nav__item--active):hover") &&
    !styles.includes(".capability-workspace-nav__item:hover"),
  "experts, skills, and connectors share an accessible header navigation",
);

ok(
  /\.capability-workspace-shell\s*\{[^}]*background:\s*var\(--chat-bg\);/.test(styles) &&
    /\.capability-workspace-nav\s*\{[^}]*min-height:\s*68px;[^}]*padding:\s*14px 24px;[^}]*border-bottom:\s*0;[^}]*background:\s*transparent;/.test(styles) &&
    /\.capability-workspace-header\s*\{[^}]*min-height:\s*76px;[^}]*padding:\s*16px 15px;[^}]*border-radius:\s*14px 14px 0 0;[^}]*background:\s*var\(--bg-elev\);/.test(styles) &&
    /\.capability-workspace-header\s*>\s*\.capability-workspace-nav\s*\{[^}]*min-height:\s*0;[^}]*padding:\s*0;[^}]*background:\s*transparent;/.test(styles) &&
    appSource.includes('className="capability-workspace-header"') &&
    appSource.includes('id="capability-workspace-header-actions"') &&
    appSource.includes('workspaceHeaderActionsId="capability-workspace-header-actions"') &&
    /\.primary-workspace--capabilities\s*\{[^}]*width:\s*100%;[^}]*margin:\s*0;[^}]*padding:\s*20px 24px 72px;[^}]*background:\s*transparent;/.test(styles) &&
    /\.primary-workspace--skills\s+\.cap-search\s+\.mem-input\s*\{[^}]*background:\s*color-mix\(in srgb, var\(--bg-elev\) 86%, var\(--chat-bg\)\);/.test(styles) &&
    !/\.primary-workspace--skills\s*,\s*\.primary-workspace--connectors\s*\{[^}]*padding-top:/.test(styles),
  "experts, skills, and connectors keep one persistent reference header while the skill search matches the skill card surface",
);

ok(
  !workspaceNavigationSource.includes('"channels"') &&
    !appSource.includes('creation.sidebar.messageChannels') &&
    !appSource.includes('initialTab="bots"') &&
    !settingsPanelSource.includes('{ id: "bots", Icon:'),
  "message channels no longer expose a standalone workspace or settings destination",
);
ok(
  capabilitiesSource.includes("<FeishuBotConnector") &&
    feishuConnectorSource.includes('catalogTargets={["feishu"]}') &&
    feishuConnectorSource.includes("<BotsSettingsSection") &&
    botsSettingsSource.includes('app.StartBotConnectionInstall(provider, domain)') &&
    botsSettingsSource.includes('app.PollBotConnectionInstall(current.result.installId)'),
  "the connector catalog owns one Feishu card while reusing the existing message-channel install flow",
);
ok(
  !appSource.includes("addProjectFromDirectory") && !appSource.includes("openProjectFromDirectory"),
  "project workspace no longer exposes the removed local-directory callbacks",
);
ok(
  /const pickWorkspace[\s\S]*?app\.PickWorkspace\(\)[\s\S]*?app\.SwitchWorkspace\(picked\)/.test(sessionControllerSource),
  "chat-side workspace picking explicitly switches after the folder has been registered",
);
ok(!appSource.includes("const useProjectStarter"), "project templates no longer start a prepared chat");

ok(
  appSource.includes('import("./components/CapabilitiesPanel")') &&
    appSource.includes("module.SkillsSettingsPage") &&
    appSource.includes("module.MCPServersSettingsPage"),
  "skills and connectors reuse existing capability pages through lazy imports",
);
ok(!/import \{[^}]*SkillsSettingsPage|import \{[^}]*MCPServersSettingsPage/.test(appSource), "capability pages stay out of the initial app chunk");
ok(
  /workspaceView === "automation" \? \([\s\S]*?<HeartbeatPanel[\s\S]*?open[\s\S]*?embedded/.test(appSource),
  "automation reuses the existing heartbeat panel in embedded workspace mode",
);
ok(
  source("../features/automation/heartbeat/HeartbeatPanel.tsx").includes("embedded?: boolean") &&
    source("../features/automation/heartbeat/heartbeat.css").includes(".heartbeat-backdrop--embedded"),
  "heartbeat keeps its modal API while offering a non-overlay workspace presentation",
);
ok(!projectSource.includes("ListWorkspaces"), "project directory does not load local workspaces for the cloud-only project surface");
ok(!projectSource.includes("PickWorkspace") && !projectSource.includes("SwitchWorkspace"), "project directory does not expose local-workspace mutations");
ok(!projectSource.includes("onUseStarter") && !appSource.includes("useProjectStarter"), "project templates do not route into new conversation composition");
ok(cloudProjectSource.includes("projectTemplates.map") && cloudProjectSource.includes('openDialog("template"'), "project templates open the CloudProject preparation dialog");
ok(styles.includes(".primary-workspace") && styles.includes(".project-directory__grid"), "primary workspaces share a responsive WorkBuddy layout");
const projectWorkspaceStyle = styles.match(/\.primary-workspace--projects\s*\{([^}]*)\}/)?.[1] ?? "";
ok(
  [
    "flex: 1 1 auto",
    "min-height: 0",
    "overflow-x: hidden",
    "overflow-y: auto",
    "overscroll-behavior-y: contain",
  ].every((declaration) => projectWorkspaceStyle.includes(declaration)),
  "project directory owns a shrinkable vertical scroll surface",
);
ok(
  styles.includes(".primary-workspace--capabilities .cap-skills") &&
    styles.includes("repeat(auto-fill, minmax(min(100%, 280px), 1fr))"),
  "skills keep readable cards across responsive workspace widths",
);
ok(
  /\.connector-catalog__card\s*\{[^}]*background:\s*color-mix\(in srgb, var\(--bg-elev\) 86%, var\(--chat-bg\)\);[^}]*box-shadow:\s*none;[^}]*transition:\s*none;/.test(styles) &&
    /\.connector-catalog__card:not\(\.connector-catalog__card--task-enabled\):not\(\.connector-catalog__card--configuration_error\):hover,[\s\S]*?:focus-within\s*\{[^}]*border-color:\s*var\(--accent\);/.test(styles) &&
    /\.connector-catalog__card--task-enabled\s*\{[^}]*border-color:[^}]*var\(--ok\)[^}]*background:[^}]*var\(--ok\)[^}]*box-shadow:\s*none;/.test(styles) &&
    /\.connector-catalog__card--configuration_error\s*\{[^}]*border-color:[^}]*var\(--danger\)/.test(styles) &&
    /\.primary-workspace--capabilities \.cap-server-entry\s*\{[^}]*background:\s*color-mix\(in srgb, var\(--bg-elev\) 86%, var\(--chat-bg\)\);[^}]*box-shadow:\s*none;/.test(styles) &&
    /\.primary-workspace--capabilities \.cap-server-entry:hover,[\s\S]*?\.primary-workspace--capabilities \.cap-server-entry:focus-within\s*\{[^}]*border-color:\s*var\(--accent\);/.test(styles),
  "connector catalog and installed MCP cards use lightweight surfaces, theme focus borders, and preserved status colors",
);
ok(
  capabilitiesSource.includes('className="cap-skill-card__top"') &&
    capabilitiesSource.includes('className="cap-skill-card__header-actions"') &&
    capabilitiesSource.includes('className="cap-skill-card__desc"') &&
    /\.skill-library__installed-grid \.cap-skill-card__top\s*\{[^}]*display:\s*flex;[^}]*min-width:\s*0;/.test(styles) &&
    /\.skill-library__installed-grid \.cap-skill-card__command\s*\{[^}]*overflow:\s*hidden;[^}]*text-overflow:\s*ellipsis;[^}]*white-space:\s*nowrap;/.test(styles) &&
    /\.skill-library__installed-grid \.cap-skill-card__header-actions\s*\{[^}]*flex:\s*0 0 auto;/.test(styles),
  "skill names, descriptions, and actions use separate overflow-safe layout regions",
);
ok(styles.includes("scrollbar-gutter: stable") && styles.includes("overscroll-behavior-y: contain"), "long capability catalogs keep a visible, contained scroll surface");
ok(!zhSource.includes("WorkBuddy 中使用"), "project empty state names the project workspace instead of WorkBuddy");
ok(
    appSource.includes("<ConnectorsWorkspace") &&
    appSource.includes("selectedServers={activeComposerCapabilities.connectors}") &&
    appSource.includes("onSelectServer={selectWorkspaceConnector}") &&
    appSource.includes("onTryConnector={tryWorkspaceConnector}") &&
    appSource.includes('showToast(t("workspace.connectors.tryReadyToast", { name }), "info")') &&
    !capabilitiesSource.includes("workspaceCustomOpen") &&
    !capabilitiesSource.includes("workspaceAddMode") &&
    capabilitiesSource.includes("connector-workspace-add--catalog") &&
    !capabilitiesSource.includes("connector-workspace-add--custom") &&
    capabilitiesSource.includes("<ConnectorCatalogPanel") &&
    capabilitiesSource.includes("CONNECTOR_CATALOG") &&
    capabilitiesSource.includes("connector-catalog__grid") &&
    styles.includes(".connector-workspace-add") &&
    styles.includes(".connector-catalog__card") &&
    styles.includes(".connector-catalog__card button:focus-visible"),
  "the connector workspace renders the approved catalog inline without a custom MCP creation entry",
);
ok(
  appSource.includes('t("settings.appearance")') &&
    appSource.includes('setSettingsTarget("appearance")') &&
    source("../features/settings/ui/SettingsPanel.tsx").includes('initialTab === "appearance" ? t("personalization.settingsTitle")'),
  "the creation account menu opens the appearance destination directly",
);
ok(
  !appSource.includes('t("sidebar.modelMarketplace")') &&
    !appSource.includes('aria-pressed={workspaceView === "modelMarketplace"}') &&
    appSource.includes('t("creation.sidebar.capabilitiesHint")') &&
    appSource.includes('className={`sidebar-feature-zone__item sidebar-feature-zone__item--capabilities') &&
    !appSource.includes('id="creation-more-capabilities"') &&
    appSource.includes("sidebar-account-entry") &&
    /\.app--creation \.sidebar-account-entry,[\s\S]*?border: 1px solid color-mix\(in srgb, var\(--border\) 60%, transparent\);[\s\S]*?background: var\(--sidebar-bg\);/.test(styles) &&
    styles.includes(".app--creation .project-tree__header") &&
    styles.includes("display: none"),
  "creation navigation groups capabilities under experts and removes the standalone model marketplace destination",
);
ok(
  appSource.includes("sidebar__brand-actions") &&
    !appSource.includes("sidebar__brand-action--search") &&
    appSource.includes("sidebar__brand-action--history") &&
    /sidebar__brand-action--history[\s\S]*?openAllHistory\(\)/.test(appSource) &&
    appSource.includes("sidebar__brand-action--collapse") &&
    /sidebar__brand-action--collapse[\s\S]*?toggleSidebar/.test(appSource) &&
    appSource.includes('searchExpanded={!sidebarCreation}') &&
    appSource.includes('t("history.centerTitle")'),
  "creation navigation keeps the global conversation center as the single history entry and places the sidebar collapse action beside it",
);
ok(
  creationBrandStart >= 0 &&
    /sidebar__brand-entry[\s\S]*?void handleNewTab\(\);/.test(creationBrandSource) &&
    creationBrandSource.includes("{BRAND_DISPLAY_NAME}") &&
    creationBrandSource.includes("{sidebarAppVersion ?") &&
    creationBrandSource.includes('className="sidebar__brand-version"') &&
    /sidebar__brand-entry[\s\S]*?<\/button>[\s\S]*?sidebar__brand-actions/.test(creationBrandSource) &&
    /sidebar__brand-action--history[\s\S]*?openAllHistory\(\)/.test(creationBrandSource) &&
    /sidebar__brand-action--collapse[\s\S]*?toggleSidebar/.test(creationBrandSource),
  "creation brand is an independent new-chat button while history and collapse remain sibling tools",
);
ok(
  appSource.includes("app.Version()") &&
    appSource.includes("formatBrandVersion(version)") &&
    /app\.Version\(\)[\s\S]*?\.catch\(\(\) => \{[\s\S]*?setSidebarAppVersion\(""\)/.test(appSource),
  "creation brand reads the dynamic app version and hides failures instead of rendering an error placeholder",
);
ok(
  /\.app--creation \.sidebar,[\s\S]*?padding:\s*38px 8px 14px;/.test(styles) &&
    /\.app--windows\.app--creation \.sidebar,[\s\S]*?padding-top:\s*0;/.test(styles) &&
    /\.app--creation \.sidebar__brand,[\s\S]*?\.app--windows\.app--creation \.sidebar__brand,[\s\S]*?flex-basis:\s*56px;[\s\S]*?height:\s*56px;[\s\S]*?min-height:\s*56px;/.test(styles) &&
    /\.app--creation \.sidebar__brand-entry,[\s\S]*?\.app--windows\.app--creation \.sidebar__brand-entry,[\s\S]*?height:\s*56px;[\s\S]*?min-height:\s*56px;/.test(styles) &&
    /\.app--windows\.app--creation \.sidebar__brand-entry \.sidebar__brand-name,[\s\S]*?font-size:\s*16px;[\s\S]*?line-height:\s*18px;/.test(styles) &&
    /\.app--windows\.app--creation \.sidebar__brand-version,[\s\S]*?line-height:\s*14px;/.test(styles) &&
    /\.app--windows\.app--creation \.sidebar__brand-actions,[\s\S]*?align-self:\s*center;/.test(styles),
  "only Windows removes the creation sidebar top inset while keeping the shared compact brand row",
);
const creationBrandStyles = styles.slice(
  styles.indexOf(".app--creation .sidebar__brand-entry,"),
  styles.indexOf(".app--creation .sidebar__brand-actions,"),
);
ok(
  creationBrandStyles.includes(".sidebar__brand-entry:focus-visible") &&
    /\.app--creation \.sidebar__brand-entry,[\s\S]*?background:\s*transparent;/.test(creationBrandStyles) &&
    /sidebar__brand-entry:hover,[\s\S]*?sidebar__brand-entry:active,[\s\S]*?background:\s*transparent;/.test(creationBrandStyles) &&
    !/(?:transform|filter|box-shadow|animation|transition)\s*:/.test(creationBrandStyles),
  "creation brand entry stays transparent while keeping accessible focus without high-cost visual effects",
);
ok(
  /!sidebarCreation && \(\s*<Tooltip label=\{t\("topicBar\.command"\)\}>/.test(appSource) &&
    /if \(sidebarCreation\) \{\s*void openAllHistory\(\);/.test(appSource),
  "creation mode hides the duplicate command button and routes its shortcut to the conversation center",
);
ok(
  creationAccountMenuSource.includes("sidebar-account-menu__identity") &&
    creationAccountMenuSource.includes('role="menu"') &&
    appSource.includes('aria-expanded={creationAccountMenuOpen}') &&
    creationAccountMenuSource.includes('t("account.paymentsTab")') &&
    creationAccountMenuSource.includes('t("settings.title")') &&
    creationAccountMenuSource.includes('setSettingsTarget("general")') &&
    creationAccountMenuSource.includes('t("settings.tab.updates")') &&
    creationAccountMenuSource.includes("setUpdateCenterOpen(true)") &&
    appSource.includes("<UpdateCenterDialog") &&
    creationAccountMenuSource.includes('t("settings.appearance")') &&
    creationAccountMenuSource.includes('setSettingsTarget("appearance")') &&
    creationAccountMenuSource.includes('t("account.logout")') &&
    creationAccountMenuSource.includes('t("account.changeAvatar")') &&
    creationAccountMenuSource.includes('accept="image/png,image/jpeg,image/webp"') &&
    creationAccountMenuSource.includes("openAccountPayments") &&
    creationAccountMenuSource.includes("requestAccountLogout") &&
    creationAccountMenuSource.includes("{sidebarAccountHasSession ? (") &&
    appSource.includes("const sidebarAccountHasSession = hasRecoverableCloudIdentity(sidebarAccount)") &&
    appSource.includes("const sidebarAccountRequiresReauthentication = requiresReauthentication(sidebarAccount)") &&
    /const sidebarAccountLabel = !sidebarAccountInitialized[\s\S]*?: sidebarAccountRequiresReauthentication[\s\S]*?t\("sidebar\.reauthenticationRequired"\)[\s\S]*?: sidebarAccountHasSession[\s\S]*?: t\("sidebar\.signInOrRegister"\)/.test(appSource) &&
    !creationAccountMenuSource.includes('data-account-menu-action="reauthenticate"') &&
    creationAccountMenuSource.includes('sidebarAccountRequiresReauthentication ? t("account.reauthenticationTitle") : sidebarAccountLabel') &&
    creationAccountMenuSource.includes('? t("account.reauthenticationAction")') &&
    creationAccountMenuSource.match(/sidebarAccountRequiresReauthentication/g)?.length === 2 &&
    !creationAccountMenuSource.includes("{sidebarAccountRequiresReauthentication ? (") &&
    appSource.includes("toggleCreationAccountMenu") &&
    /useEffect\(\(\) => \{[\s\S]*?ensureBootstrap\(\)/.test(accountSidebarSource) &&
    /toggleMenu[\s\S]*?ensureBootstrap\(\)/.test(accountSidebarSource) &&
    !creationAccountMenuSource.includes("openTrash()") &&
    !creationAccountMenuSource.includes('t("sidebar.trash")'),
  "expired sessions reuse the anonymous account menu structure while changing only identity copy and preserving the no-session logout gate",
);
ok(
  appSource.includes("selectComposerSkill") &&
    appSource.includes("selectComposerConnector") &&
    !appSource.includes("await app.SetMCPServerEnabled(name, true)") &&
    appSource.includes("loadEnabledConnectorSelections") &&
    appSource.includes("saveEnabledConnectorSelections") &&
    appSource.includes("enabledConnectorsByScope[sidebarAccountScopeKey]") &&
    appSource.includes("connectComposerConnector(selectedMCPServer)") &&
    !capabilitiesSource.includes("if (!selectable.has(name)) onSelectServer(name)") &&
    appSource.includes("formatSkillToken(name)") &&
    appSource.includes("toggleComposerConnector") &&
    !appSource.includes("selectedSkills={activeComposerCapabilities.skills}") &&
    appSource.includes("selectedConnectors={activeComposerCapabilities.connectors}") &&
    composerSource.includes("<SkillTokenEditor") &&
    composerSource.includes("<SkillQuickMenu") &&
    composerSource.includes("parseSkillTokens(expandedText, knownSkillNames)") &&
    composerSource.includes("stripSkillTokens(expandedText, knownSkillNames)") &&
    composerSource.includes("const executableSkillNames = parsedSkillInput.skillNames.filter") &&
    composerSource.includes("skills: executableSkillNames") &&
    composerSource.includes("connectors: useConnectorScope ? selectedConnectors : []"),
  "skills use one-shot inline slash tokens while connector enablement persists globally per owner",
);
ok(
  /\) : !sidebarCreation \? \(\s*<nav className="sidebar__nav">/.test(appSource),
  "creation mode removes the obsolete four-button footer navigation",
);
ok(
  /\.app--creation \.sidebar__section--projects,[\s\S]*?flex: 1 1 auto;/.test(styles) &&
    /\.app--creation \.project-tree,[\s\S]*?flex: 1 1 auto;[\s\S]*?min-height: 0;/.test(styles) &&
    /\.app--creation \.project-tree__list,[\s\S]*?flex: 1 1 auto;[\s\S]*?max-height: none;/.test(styles) &&
    appSource.includes('sidebar-feature-zone__item sidebar-feature-zone__item--new-chat') &&
    styles.includes(".app--creation .sidebar-feature-zone__item--new-chat"),
  "creation tasks and workspaces fill the space above the fixed account entry",
);
ok(
  projectTreeSource.includes("ChevronDown") &&
    projectTreeSource.includes("project-tree__task-section-arrow-shell") &&
    projectTreeSource.includes('className="project-tree__topic-actions"') &&
    projectTreeSource.includes('aria-label={pinLabel}') &&
    projectTreeSource.includes('aria-label={archiveLabel}') &&
    styles.includes(".app--creation .project-tree__topic-actions") &&
    styles.includes(".app--creation .project-tree__topic-action:focus-visible"),
  "creation task groups use a polished disclosure control and expose accessible pin and trash actions on every topic row",
);
const taskSectionToggleHoverStyle = styles.match(/\.project-tree__task-section-toggle:hover\s*\{([^}]*)\}/)?.[1] ?? "";
const taskSectionToggleFocusStyle = styles.match(/\.project-tree__task-section-toggle:focus-visible\s*\{([^}]*)\}/)?.[1] ?? "";
const taskSectionArrowFeedbackStyle =
  styles.match(
    /\.project-tree__task-section-toggle:hover \.project-tree__task-section-arrow-shell,\s*\.project-tree__task-section-toggle:focus-visible \.project-tree__task-section-arrow-shell\s*\{([^}]*)\}/,
  )?.[1] ?? "";
ok(
  taskSectionToggleHoverStyle.includes("background: var(--sidebar-hover)") &&
    taskSectionToggleFocusStyle.includes("outline: 2px solid var(--accent-soft)") &&
    taskSectionArrowFeedbackStyle.includes("color: var(--fg)") &&
    !taskSectionArrowFeedbackStyle.includes("background:"),
  "task section headers use one row hover surface while preserving arrow color and keyboard focus feedback",
);
ok(
  projectTreeSource.includes('aria-label={t("projectTree.searchPlaceholder")}') &&
    projectTreeSource.includes('name="project-tree-search"') &&
    projectTreeSource.includes('autoComplete="off"'),
  "the moved project search keeps an explicit accessible field name",
);
ok(
  automationSource.includes("heartbeat-embedded-templates") &&
    automationSource.includes("deletable={false}") &&
    automationSource.includes("templates.slice(0, 6)") &&
    automationSource.includes("heartbeat-task-section"),
  "automation exposes compact non-destructive starter templates and a distinct task workspace",
);
ok(
  automationSource.includes("Object.entries(workspaceMap)") &&
    automationSource.includes("workspaceOptions.map") &&
    automationSource.includes("tasks.length > 0 || !embedded") &&
    automationStyles.includes("width: min(100%, 1440px)") &&
    automationStyles.includes("@media screen and (max-width: 1180px)"),
  "automation uses real workspace data and adapts its wider task layout while keeping empty-state controls purposeful",
);
ok(
  appSource.includes('href={skipToProjectComposerAvailable || skipToComposerAvailable ? "#composer-input" : "#main-workspace"}') &&
    /<main[^>]*id="main-workspace"[^>]*className="main"[^>]*tabIndex=\{-1\}/.test(appSource),
  "skip navigation targets the visible workspace when the composer is absent",
);
ok(
  appSource.includes("sidebarManualExpandedRef.current = true") &&
    appSource.includes("viewportTooNarrowForSidebar && sidebarManualExpandedRef.current") &&
    appSource.includes("sidebarCreation && sidebarCollapsed") &&
    appSource.includes("sidebar-expand-toggle") &&
    styles.includes(".app--creation .sidebar-expand-toggle") &&
    !styles.includes(".sidebar-collapse-toggle"),
  "narrow workspaces expose a discoverable sidebar toggle and preserve an explicit user expansion",
);
ok(
  /const closeNarrowSidebar = useCallback[\s\S]*?setSidebarCollapsed\(true\)/.test(appSource) &&
    /if \(closeSidebar\) closeNarrowSidebar\(\)/.test(workspaceNavigationSource) &&
    /const openProjectDirectory = useCallback[\s\S]*?prepare\(true\)[\s\S]*?setSelectedCloudProjectId\(""\)[\s\S]*?setWorkspaceView\("projects"\)/.test(workspaceNavigationSource) &&
    /const openCloudProject = useCallback\(\(projectId: string\)[\s\S]*?setSelectedCloudProjectId\(projectId\)[\s\S]*?setWorkspaceView\("projects"\)/.test(workspaceNavigationSource),
  "primary workspace navigation closes the narrow sidebar after a destination is selected",
);
ok(
  appSource.includes("mainWorkspaceRef.current?.scrollTo({ top: 0, left: 0 })") &&
    appSource.includes("ref={mainWorkspaceRef}"),
  "switching primary workspaces resets the shared scroll container",
);

for (const locale of ["zh"]) {
  const localeSource = source(`../locales/${locale}.ts`);
  for (const key of ["workspace.projects.title", "workspace.projects.starters", "workspace.skills.title", "workspace.connectors.title", "personalization.title", "personalization.settingsTitle", "heartbeat.pageTitle", "heartbeat.templateStarters", "heartbeat.myTasks"]) {
    ok(localeSource.includes(`"${key}"`), `${locale} localizes ${key}`);
  }
}

console.log("WorkBuddy primary navigation contract passed");
