import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/app-chrome-tabs.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");
const desktopPreferencesSource = readFileSync(resolve(testDir, "../app/useDesktopPreferences.ts"), "utf8");
const appChromeSource = readFileSync(resolve(testDir, "../components/AppChrome.tsx"), "utf8");
const commandPaletteSource = readFileSync(resolve(testDir, "../components/CommandPalette.tsx"), "utf8");
const projectTreeSource = readFileSync(resolve(testDir, "../features/workspace/ui/ProjectTree.tsx"), "utf8");
const sessionControllerSource = readFileSync(resolve(testDir, "../features/session/useController.ts"), "utf8");
const tabBarSource = readFileSync(resolve(testDir, "../components/TabBar.tsx"), "utf8");
const topicShortcutsSource = readFileSync(resolve(testDir, "../lib/topicShortcuts.ts"), "utf8");
const openTopicCoalescingSource = readFileSync(resolve(testDir, "../lib/openTopicCoalescing.ts"), "utf8");
const transcriptSource = readFileSync(resolve(testDir, "../components/Transcript.tsx"), "utf8");
const layoutStoreSource = readFileSync(resolve(testDir, "../store/layout.ts"), "utf8");
const stylesSource = loadStylesSource().replace(/\/\*[\s\S]*?\*\//g, "");
const openSidebarImConnectionSessionStart = appSource.indexOf("const openSidebarImConnectionSession");
const loadConversationCenterDataStart = appSource.indexOf("const loadConversationCenterData", openSidebarImConnectionSessionStart);
const openSidebarImConnectionSessionSource = openSidebarImConnectionSessionStart >= 0 && loadConversationCenterDataStart > openSidebarImConnectionSessionStart
  ? appSource.slice(openSidebarImConnectionSessionStart, loadConversationCenterDataStart)
  : "";
const onResumeSessionStart = appSource.indexOf("const onResumeSession");
const openPaletteStart = appSource.indexOf("const openPalette", onResumeSessionStart);
const onResumeSessionSource = onResumeSessionStart >= 0 && openPaletteStart > onResumeSessionStart
  ? appSource.slice(onResumeSessionStart, openPaletteStart)
  : "";
const topicbarActionsStart = appSource.indexOf('<div className="topicbar__actions">');
const topicbarActionsEnd = appSource.indexOf("</header>", topicbarActionsStart);
const topicbarActionsSource = topicbarActionsStart >= 0 && topicbarActionsEnd > topicbarActionsStart
  ? appSource.slice(topicbarActionsStart, topicbarActionsEnd)
  : "";

let passed = 0;
let failed = 0;

function ok(value: unknown, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function matchingBlocks(selector: string): string[] {
  const blocks: string[] = [];
  const rule = /([^{}]+)\{([^{}]*)\}/g;
  let match: RegExpExecArray | null;
  while ((match = rule.exec(stylesSource)) !== null) {
    const selectors = match[1].split(",").map((part) => part.trim());
    if (selectors.includes(selector)) blocks.push(match[2]);
  }
  return blocks;
}

function finalDeclaration(selector: string, property: string): string | undefined {
  let value: string | undefined;
  for (const block of matchingBlocks(selector)) {
    const declaration = new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g");
    let match: RegExpExecArray | null;
    while ((match = declaration.exec(block)) !== null) {
      value = match[1].trim();
    }
  }
  return value;
}

console.log("\napp chrome tabs");

ok(
  /import \{ TabBar \} from "\.\/TabBar";/.test(appChromeSource),
  "AppChrome keeps the classic top session tab strip implementation",
);

for (const propName of ["onTabChange", "onTabClose", "onTabsClose", "onTabsReorder", "onNewTab"]) {
  ok(
    new RegExp(`\\b${propName}\\b`).test(appChromeSource),
    `AppChrome exposes ${propName} for classic tabs`,
  );
}

ok(
  /app-chrome__tab-strip/.test(appChromeSource),
  "AppChrome markup includes classic tab strip containers",
);

ok(
    tabBarSource.includes('className="tabbar__tab-close"') &&
    tabBarSource.includes('type="button"') &&
    tabBarSource.includes('aria-label={t("tabBar.closeTab")}') &&
    !/<span[^>]*className="tabbar__tab-close"/.test(tabBarSource),
  "tab close control is a real labeled button",
);

ok(
  /open:\s*WORKSPACE_PANEL_DEFAULT_OPEN/.test(layoutStoreSource) &&
    /workspacePanelsByConversation:\s*\{\}/.test(layoutStoreSource),
  "each conversation right dock starts collapsed",
);

ok(
  /const conversationStateKey = useMemo\([\s\S]*?composerDraftKeyForTab\(activeTab, activeTabId\)/.test(appSource) &&
    /workspacePanelsByConversation\[conversationStateKey\]/.test(appSource),
  "right dock state is selected by the stable conversation identity",
);

ok(
  /const workspacePanelAvailable = workspaceView === "chat";/.test(appSource) &&
    /workspacePanelOpen:\s*workspacePanelLayoutOpen/.test(appSource),
  "non-chat pages exclude the right dock from layout",
);

ok(
  /workspacePanelAvailable:\s*boolean;/.test(appChromeSource) &&
    /\{workspacePanelAvailable && !workspacePanelMaximized && \(/.test(appChromeSource),
  "non-chat pages hide the classic right dock entry point",
);

ok(
  /const SHOW_CONTEXT_DOCK = false;/.test(appSource) &&
    appSource.includes("{SHOW_CONTEXT_DOCK && ("),
  "right dock hides the context overview tab",
);

ok(
  /mode:\s*"files"/.test(layoutStoreSource) &&
    /const toggleWorkspacePanel = useCallback\([\s\S]*?openWorkspacePanel\("files"\);/.test(appSource),
  "right dock opens the files view by default",
);

ok(
  appSource.includes('onClick={() => openRightDockMode("files")}') &&
    !appSource.includes('onClick={() => openRightDockMode("changed")}') &&
    !appSource.includes('t("workspace.changedTab")'),
  "right dock exposes only the files view",
);

ok(
  topicbarActionsSource.includes("<CopyButton") &&
    topicbarActionsSource.includes("<Download") &&
    topicbarActionsSource.includes("<CircleHelp") &&
    topicbarActionsSource.includes("<PanelRight") &&
    !topicbarActionsSource.includes("<GitBranch") &&
    !topicbarActionsSource.includes('t("workspace.changedTab")'),
  "topic bar omits the redundant Git changes shortcut while retaining its utility actions",
);

ok(
  finalDeclaration(".app-chrome__tab-strip", "overflow") === "hidden",
  "AppChrome tab strip clips tabs to the available chrome width",
);

ok(
  finalDeclaration(".app-chrome__tab-strip", "min-width") === "0",
  "AppChrome tab strip can shrink beside the right dock",
);

ok(
  finalDeclaration(":root[data-theme-style] .app-chrome--tabs .tabbar__tabs", "max-width")?.includes("--chrome-panel-control-size"),
  "themed AppChrome tab lists reserve a flowing new-tab button slot",
);

ok(
  finalDeclaration(":root[data-theme-style] .app-chrome--tabs .tabbar__tabs", "flex") === "0 1 auto",
  "themed AppChrome tab lists size to tab content before shrinking",
);

ok(
  finalDeclaration(":root[data-theme-style] .app-chrome--tabs .tabbar__tabs", "width") === "max-content",
  "themed AppChrome tab lists keep the new-tab button next to the last tab",
);

ok(
  finalDeclaration(":root[data-theme-style] .app-chrome--tabs .tabbar > .tooltip-trigger:has(.tabbar__new)", "flex")?.includes("--chrome-panel-control-size"),
  "themed AppChrome new-tab button keeps a stable slot beside the tabs",
);

ok(
  /workbenchChrome \? \(\s*<span className="app-chrome__spacer" aria-hidden="true" \/>/s.test(appChromeSource),
  "AppChrome workbench branch skips the tab strip",
);

ok(
  /app-chrome__tools--fixed/.test(appChromeSource),
  "AppChrome renders the command search as a fixed chrome tool",
);

ok(
  /workbenchChromeHidden\s*=\s*sidebarWorkbench/.test(appSource),
  "workbench chrome is hidden for every desktop platform",
);

ok(
  /useState<DesktopLayoutStyle>\("creation"\)/.test(desktopPreferencesSource),
  "desktop defaults to creation layout",
);

const creationFeatureStart = appSource.indexOf('<section className="sidebar-feature-zone"');
const creationFeatureEnd = appSource.indexOf('<section className="sidebar__section sidebar__section--projects">', creationFeatureStart);
const creationFeatureZone = creationFeatureStart >= 0 && creationFeatureEnd > creationFeatureStart
  ? appSource.slice(creationFeatureStart, creationFeatureEnd)
  : "";
ok(
  !creationFeatureZone.includes('t("creation.sidebar.messageChannels")') &&
    creationFeatureZone.includes('t("sidebar.automation")') &&
    creationFeatureZone.includes('t("sidebar.experts")') &&
    !creationFeatureZone.includes('t("sidebar.skills")') &&
    !creationFeatureZone.includes('t("sidebar.memory")'),
  "creation sidebar keeps message channels, skills, and memory out of the desktop left feature zone",
);

ok(
  /\{!appChromeHidden && \(/.test(appSource),
  "workbench skips rendering the top AppChrome row",
);

ok(
  /topicbar__chrome-btn/.test(appSource),
  "workbench keeps chrome controls in the topic bar",
);

const shortcutsHelpBlock = appSource.match(/<Tooltip label=\{t\("shortcuts\.cheatsheetTitle"\)\}>[\s\S]*?<\/Tooltip>/)?.[0] ?? "";
ok(
  /setShortcutsOpen\(true\)/.test(shortcutsHelpBlock) &&
    !/setSettingsTarget\("shortcuts"\)/.test(shortcutsHelpBlock),
  "topic bar shortcut help button opens the visible cheatsheet dialog",
);

ok(
  /const \[transcriptRevealSignal, setTranscriptRevealSignal\] = useState\(0\);/.test(appSource) &&
    /revealActiveSignal=\{tabRevealSignal\}/.test(appSource) &&
    /revealSignal=\{transcriptRevealSignal\}/.test(appSource),
  "transcript bottom reveal is decoupled from tab-strip reveal",
);

const tabsReorderBlock = appSource.match(/const handleTabsReorder = useCallback\([\s\S]*?\n  \}, \[refreshTabMetas, reorderTabs\]\);/)?.[0] ?? "";
ok(
  /setTabRevealSignal/.test(tabsReorderBlock) && !/setTranscriptRevealSignal/.test(tabsReorderBlock),
  "tab reordering refreshes the tab strip without snapping the transcript",
);

ok(
  /setTabMetas\(\(current\) => JSON\.stringify\(current\) === JSON\.stringify\(tabs\) \? current : tabs\)/.test(appSource),
  "unchanged tab metadata polling preserves state identity and avoids root rerenders",
);

ok(
  /aria-label=\{t\("transcript\.jumpToBottom"\)\}/.test(transcriptSource) &&
    /title=\{t\("transcript\.jumpToBottom"\)\}/.test(transcriptSource),
  "jump-to-bottom affordance uses localized transcript text",
);

ok(
  /setActive\(items\.length > 0 \? 0 : -1\)/.test(commandPaletteSource),
  "command palette highlights the first item when opened with an empty query",
);

ok(
  /topicShortcutIndexFromEvent\(event, desktopPlatform\)/.test(appSource) &&
    /useTopicShortcuts\(!sidebarCollapsed, desktopPlatform\)/.test(appSource),
  "topic shortcuts use the resolved desktop platform",
);

ok(
  /topicShortcutLabel\(shortcutIndex, shortcutPlatform\)/.test(projectTreeSource),
  "topic shortcut badges render the platform-specific modifier",
);

ok(
  /if \(!enabled\) hideBadges\(\);/.test(topicShortcutsSource) &&
    /if \(heldRef\.current\) hideBadges\(\);/.test(topicShortcutsSource) &&
    /window\.removeEventListener\("blur", onBlur\);\s*hideBadges\(\);/.test(topicShortcutsSource),
  "topic shortcut badge state is cleared when disabled, interrupted, or cleaned up",
);

ok(
  /const \[rewindCommitting, setRewindCommitting\] = useState\(false\);/.test(appSource) &&
    /rewindStateRef\.current = null;/.test(appSource) &&
    /setRewindCommitting\(true\);/.test(appSource),
  "committing optimistic rewind clears undo state before awaiting the backend",
);

ok(
  /const controllerReady = sendAvailabilityAllowsControllerActions\(displayedSendAvailability\) && !state\.backendActivationPending;/.test(appSource) &&
    /if \(!controllerReady \|\| existingSessionSubmitBlocked \|\| subscriptionModelInvocationBlocked\) return;\s*void commitThenSend\(text\)\.catch/.test(appSource) &&
    /onPrompt=\{handleTranscriptPrompt\}/.test(appSource) &&
    /submitDisabled=\{!controllerReady \|\| existingSessionSubmitBlocked \|\| subscriptionModelInvocationBlocked\}/.test(appSource) &&
    /disabled=\{expertMemberHistoryFocused \|\| surfaceTransitionActive \|\| rewindCommitting/.test(appSource),
  "welcome prompts and composer submit share controller readiness and model access gates",
);

ok(
  /<div className="session-composer-slot">\s*<div className="session-composer-content"[^>]*>\s*<Composer/.test(appSource) &&
    !appSource.includes("session-composer-slot--runtime-pending") &&
    !appSource.includes("session.runtimeConnecting") &&
    !appSource.includes("connectingTimer") &&
    /window\.setTimeout\(\(\) => setRuntimeConnectionStage\("stalled"\), 3_000\)/.test(appSource) &&
    /submitDisabled=\{!controllerReady \|\| existingSessionSubmitBlocked \|\| subscriptionModelInvocationBlocked\}/.test(appSource),
  "runtime connection waits keep the composer visible and gated without a connecting placeholder",
);

ok(
  /const composerSessionKey = useMemo\([\s\S]*?composerDraftKeyForTab\([\s\S]*?surfaceTransitionActive \? renderedViewTarget : activeTab/.test(appSource) &&
    /const conversationStateKey = useMemo\([\s\S]*?composerDraftKeyForTab\(activeTab, activeTabId\)/.test(appSource) &&
    /if \(surfaceTransitionActive\) \{\s*return state\.meta \? composerProfileFromMeta\(state\.meta\) : defaultComposerProfile;/.test(appSource) &&
    /const composerModelEntitlementHandoffActive = Boolean\([\s\S]*?renderedComposerIdentity\?\.billingSource === "subscription_pool"[\s\S]*?!modelSwitcherTransition[\s\S]*?surfaceTransitionActive \|\| currentSubscriptionRepairFacts\.state === "loading"/.test(appSource) &&
    /modelLabel=\{composerModelLabel\}/.test(appSource) &&
    /modelSelectionPresentation=\{composerModelSelectionPresentation\}/.test(appSource) &&
    /const renderedComposerIdentity = renderedViewTarget\?\.composerIdentity/.test(appSource) &&
    /composerExpertShouldUseTargetIdentity\(\{[\s\S]*?renderedViewKey: composerExpertViewKey,[\s\S]*?resolution: activeExpertResolution/.test(appSource) &&
    /activeExpert=\{displayedActiveExpert\}/.test(appSource),
  "surface transitions restore the target draft and cached Composer identity without exposing source-session controls",
);

ok(
  /const topicbarTitle =[\s\S]*?surfaceTransitionActive[\s\S]*?renderedViewTitle \|\| t\("transcript\.loadingSession"\)[\s\S]*?: topicDisplayTitle\(activeTab\)/.test(appSource) &&
    /surfaceTransitioning=\{surfaceTransitionActive\}/.test(appSource) &&
    !/session-composer-slot--transitioning/.test(appSource) &&
    /const sessionComposerSelectionBusy = sessionComposerSelectionPending[\s\S]*?\|\| surfaceTransitionActive[\s\S]*?\|\| state\.composerSelection\.phase === "updating-effort";/.test(appSource) &&
    /modelSwitchPending=\{sessionComposerSelectionBusy\}/.test(appSource) &&
    /const composerModelSelectionPresentation = composerModelEntitlementPresentation\?\.state === "choose"[\s\S]*?"pending" as const[\s\S]*?"auto" as const/.test(appSource) &&
    /disabled=\{expertMemberHistoryFocused \|\| surfaceTransitionActive \|\| rewindCommitting/.test(appSource),
  "session transitions present the target title while keeping a busy Composer shell and disabling stale controls",
);

ok(
  /beginSurfaceTransition\(\{[\s\S]*?sessionPath: target\.value[\s\S]*?ensureBlankSurface\([^;]*transitionId\)[\s\S]*?(?:openChannelSession|resumeSession)\(target\.value, tab\.id, transitionId\)/.test(appSource) &&
    /beginSurfaceTransition\(\{[\s\S]*?sessionPath: session\.path[\s\S]*?ensureBlankSurface\([^;]*transitionId\)[\s\S]*?openChannelSession\(session\.path, targetTab\.id, transitionId\)/.test(appSource),
  "path and channel restores establish the existing-session transition before creating a blank runtime surface",
);

ok(
  /await openChannelSession\(target\.value, tab\.id, transitionId\);\s*openedTab = \{ \.\.\.tab, sessionPath: target\.value, readOnly: true \};/.test(openSidebarImConnectionSessionSource) &&
    /await resumeSession\(target\.value, tab\.id, transitionId\);\s*openedTab = \{ \.\.\.tab, sessionPath: target\.value, readOnly: false \};/.test(openSidebarImConnectionSessionSource) &&
    /if \(openedTab\) seedActiveTabMeta\(openedTab\);/.test(openSidebarImConnectionSessionSource),
  "sidebar path restores seed the final session identity and channel read-only contract before metadata refresh",
);

ok(
  /await openChannelSession\(session\.path, targetTab\.id, transitionId\);\s*targetTab = \{ \.\.\.targetTab, sessionPath: session\.path, readOnly: true \};/.test(onResumeSessionSource) &&
    /await resumeSession\(session\.path, targetTab\.id\);[\s\S]*?targetTab = \{ \.\.\.targetTab, sessionPath: session\.path, readOnly: false \};[\s\S]*?seedActiveTabMeta\(targetTab\);/.test(onResumeSessionSource),
  "conversation-center restores seed channel and ordinary session identity before background tab refresh",
);

ok(
  /const \[draftScenesByTab, setDraftScenesByTab\] = useState<Record<string, Scene>>\(\{\}\);/.test(appSource) &&
    /if \(!controllerReady\) \{\s*setDraftScenesByTab\(\(current\) => \(\{ \.\.\.current, \[activeTab\.id\]: scene \}\)\);\s*return;/.test(appSource) &&
    /const pendingScene = draftScenesByTab\[activeTab\.id\];/.test(appSource) &&
    /const activeScenePresentation = normalizeScenePresentation\(displayedScene, activeTab\?\.sceneState\);/.test(appSource) &&
    /scene=\{welcomeScenePresentation\.scene\}/.test(appSource),
  "signed-out creation modes remain local and synchronize after the controller recovers",
);

ok(
  /const transcriptHydrating = state\.hydrating && !state\.hydrateHistoryLoaded;/.test(appSource) &&
    /const blankSessionPresentation = state\.surfaceIntent === "blank"[\s\S]*?state\.surfaceIntent === "unknown" && !renderedSessionPath\?\.trim\(\)/.test(appSource) &&
    /const showWelcomeDuringHydration = blankSessionPresentation[\s\S]*?&& transcriptHydrating[\s\S]*?&& !state\.hydrateError;/.test(appSource) &&
    /showWelcomeDuringHydration=\{showWelcomeDuringHydration\}/.test(appSource) &&
    /!sessionHasContent && blankSessionPresentation \? " chat-pane--creation-empty"/.test(appSource),
  "explicit blank hydration mounts Welcome in the stable creation layout while errors remain suppressed",
);

const openBlankSessionBlock = appSource.match(
  /const openBlankSession = useCallback\([\s\S]*?\n  \}, \[[^\]]*singleSurfaceLayout[^\]]*\]\);/,
)?.[0] ?? "";
ok(
  /blankSessionRequestSeqRef\.current \+ 1/.test(openBlankSessionBlock) &&
    /setPendingBlankSessionRequest\(request\)/.test(openBlankSessionBlock) &&
    openBlankSessionBlock.indexOf("setPendingBlankSessionRequest(request)") < openBlankSessionBlock.indexOf("await ensureBlank") &&
    /if \(blankSessionRequestSeqRef\.current !== request\.seq\) return false;/.test(openBlankSessionBlock) &&
    /setPendingBlankSessionRequest\(\(current\) => current\?\.seq === request\.seq \? null : current\)/.test(openBlankSessionBlock) &&
    /refreshTabMetas\(\(\) => blankSessionRequestSeqRef\.current === request\.seq\)/.test(openBlankSessionBlock) &&
    /\} catch \(error\) \{[\s\S]*?if \(blankSessionRequestSeqRef\.current !== request\.seq\) return false;[\s\S]*?throw error;/.test(openBlankSessionBlock),
  "shared blank opening publishes presentation metadata before activation and ignores stale completion",
);
ok(
  /const refreshTabMetas = useCallback\(async \(shouldApply: \(\) => boolean = \(\) => true\)/.test(appSource) &&
    /if \(shouldApply\(\)\) \{\s*setTabMetas/.test(appSource),
  "late blank metadata refreshes cannot replace the newer tab list",
);
ok(
  /const tabSwitchRequestSeqRef = useRef\(0\);/.test(appSource) &&
    /const handleTabChange[\s\S]*?tabSwitchRequestSeqRef\.current = requestSeq[\s\S]*?refreshTabMetas\([\s\S]*?tabSwitchRequestSeqRef\.current === requestSeq/.test(appSource),
  "late task-switch metadata refreshes cannot replace the latest active tab",
);
ok(
  /const blankSceneSelectionPending = Boolean\(pendingBlankSessionRequest && blankSessionPresentation\);/.test(appSource) &&
    /const previewScenePresentation = blankSessionRequestScenePresentation\(\{[\s\S]*?targetScope: scope,[\s\S]*?activeTabId,[\s\S]*?visibleScene: displayedScene,[\s\S]*?surfaceIntent: state\.surfaceIntent,[\s\S]*?hasVisibleContent: sessionHasContent/.test(openBlankSessionBlock) &&
    /previewScene: previewScenePresentation\.scene,[\s\S]*?previewSceneState: previewScenePresentation\.sceneState/.test(openBlankSessionBlock) &&
    /const welcomeScenePresentation = blankSceneSelectionPending[\s\S]*?pendingBlankSessionRequest\?\.previewScene,[\s\S]*?pendingBlankSessionRequest\?\.previewSceneState/.test(appSource) &&
    /const blankSceneIdentityUnbound = blankSessionPresentation[\s\S]*?!activeTab \|\| activeTab\.id !== activeTabId/.test(appSource) &&
    /blankSceneSelectionPending[\s\S]*?\|\| zeroTurnTopicBindingPending[\s\S]*?\|\| blankSceneIdentityUnbound[\s\S]*?\? "disabled"/.test(appSource) &&
    /scene=\{welcomeScenePresentation\.scene\}/.test(appSource) &&
    /sceneSelectionAvailability=\{welcomeSceneSelectionAvailability\}/.test(appSource),
  "blank requests capture a safe preview while keeping controls disabled until the TabMeta is seeded",
);
ok(
  /const zeroTurnTopicBindingPending = surfaceTransitionActive[\s\S]*?renderedViewTarget\?\.sessionTurns === 0[\s\S]*?conversationStateKey !== composerSessionKey;/.test(appSource) &&
    /const preparedScenePresentation = normalizeScenePresentation\(\s*renderedViewTarget\?\.scene,\s*renderedViewTarget\?\.sceneState,\s*\);/.test(appSource) &&
    /zeroTurnTopicBindingPending[\s\S]*?\? preparedScenePresentation/.test(appSource) &&
    !/scenePresentationPending=\{/.test(appSource),
  "zero-turn topic first frame renders its prepared scene without hiding active, headline, or secondary scenarios",
);
ok(
  /const hideCreationTopicTitle = sidebarCreation[\s\S]*?pendingBlankSessionRequest !== null[\s\S]*?blankSessionPresentation && sessionTurns === 0/.test(appSource) &&
    /<div className="topicbar__identity">\s*\{hideCreationTopicTitle \? null : \(/.test(appSource),
  "creation hides only the topic title content while preserving the topicbar identity layout shell",
);
ok(
  !appSource.includes("hideActiveDraftTopic") &&
    /const hidesCreationBlankNode = \(node: ProjectNode\) => creationTopics[\s\S]*?!isRuntimeSessionNode\(node\)[\s\S]*?node\.label\?\.trim\(\) === "新的会话"[\s\S]*?node\.turns \?\? 0[\s\S]*?node\.sessionTurns \?\? 0[\s\S]*?if \(hidesCreationBlankNode\(node\)\) return null/.test(projectTreeSource),
  "creation hides zero-turn default-title topics regardless of active state without changing project-tree data",
);
ok(
  /const handleNewTab = useCallback\([\s\S]*?openBlankSession\(target\.scope, target\.workspaceRoot, true\)/.test(appSource) &&
    /onCreateTopic=\{async \(scope, workspaceRoot\) => \{[\s\S]*?openBlankSession\(scope, scope === "project" \? workspaceRoot : ""\)/.test(appSource) &&
    /const summonExpertToNewTask = useCallback\([\s\S]*?openBlankSession\(target\.scope, target\.workspaceRoot\)/.test(appSource) &&
    /const confirmExpertTeamSwitch = useCallback\([\s\S]*?openBlankSession\(target\.scope, target\.workspaceRoot\)/.test(appSource),
  "top, project-tree, and expert new-task entries share the guarded blank opening path",
);
ok(
  /if \(blankSessionRequestSeqRef\.current !== request\.seq\) return false;/.test(openBlankSessionBlock) &&
    /return meta\.id;[\s\S]*?\} catch/.test(openBlankSessionBlock) &&
    /const targetTabId = await openBlankSession\(target\.scope, target\.workspaceRoot\);[\s\S]*?if \(!targetTabId\)[\s\S]*?return;[\s\S]*?await activateExpertSelection\(selection, targetTabId, request\)/.test(appSource) &&
    /const opened = await openBlankSession\(target\.scope, target\.workspaceRoot\);\s*if \(!opened\) return;[\s\S]*?setActiveExpert\(null\)/.test(appSource),
  "stale blank completions stop expert side effects and successful opens bind activation to the returned tab",
);
ok(
  /supersedeOpenTopicRequests\(\{ seqRef: openTopicSeqRef, runningRef: openTopicRunningRef, pendingRef: openTopicPendingRef \}\);/.test(openBlankSessionBlock) &&
    /const supersedeBlankSessionRequest = useCallback\(\(\) => \{[\s\S]*?blankSessionRequestSeqRef\.current \+= 1;[\s\S]*?setPendingBlankSessionRequest\(null\)/.test(appSource) &&
    /const beginProjectTreeTopicOpenIntent = useCallback\([\s\S]*?supersedeBlankSessionRequest\(\);/.test(appSource) &&
    /const handleOpenTopic = useCallback\([\s\S]*?supersedeBlankSessionRequest\(\);/.test(appSource),
  "blank creation and existing-topic navigation supersede each other in both directions",
);
ok(
  /const surfaceNavigationSeqRef = useRef\(0\);/.test(appSource) &&
    /preparedTopicNavigationRef\.current = \{ token, transitionId, navigationSeq \};/.test(appSource) &&
    /prepared\.navigationSeq !== surfaceNavigationSeqRef\.current[\s\S]*?cancelPreparedTopicNavigation\(preparedOpenToken\);[\s\S]*?return Promise\.resolve\(\);/.test(appSource),
  "a later blank intent cancels the delayed project-tree topic callback before it can reopen the old target",
);
ok(
  /export function supersedeOpenTopicRequests\(/.test(openTopicCoalescingSource) &&
    /supersedeOpenTopicRequests\(\{ seqRef: openTopicSeqRef, runningRef: openTopicRunningRef, pendingRef: openTopicPendingRef \}\);/.test(appSource) &&
    /if \(!singleSurfaceLayout\) return undefined;/.test(appSource) &&
    !/if \(!singleSurfaceLayout \|\| !sessionPath\?\.trim\(\)\) return undefined;/.test(appSource),
  "project-tree click intent immediately stales running and pending topics, including pathless rows",
);

const openTopicBlock = appSource.match(/const handleOpenTopic = useCallback\([\s\S]*?\n  \}, \[[^\]]*runOpenTopicRequest[^\]]*\]\);/)?.[0] ?? "";
const beginProjectTreeTopicOpenIntentBlock = appSource.match(/const beginProjectTreeTopicOpenIntent = useCallback\([\s\S]*?\n  \}, \[[^\]]*singleSurfaceLayout[^\]]*\]\);/)?.[0] ?? "";
ok(
  /const openTopicRunningRef = useRef\(false\);/.test(appSource) &&
    /const openTopicPendingRef = useRef<PendingOpenTopicRequest \| null>\(null\);/.test(appSource) &&
    /const runOpenTopicRequest = useCallback\(async \(request: PendingOpenTopicRequest\)/.test(appSource) &&
    /openedTab = await activateTopic\([\s\S]*?request\.sessionTurns/.test(appSource) &&
    /openedTab = await openTopicSession\(request\.scope/.test(appSource) &&
    /openedTab = await openGlobalTab\(request\.topicId\)/.test(appSource) &&
    /openedTab = await openProjectTab\(request\.workspaceRoot, request\.topicId\)/.test(appSource) &&
    /request\.seq !== openTopicSeqRef\.current/.test(appSource) &&
    /enqueueOpenTopicRequest\([\s\S]*runningRef: openTopicRunningRef, pendingRef: openTopicPendingRef/.test(openTopicBlock) &&
    !/openTopicQueueRef\.current\.catch\(\(\) => \{\}\)\.then/.test(appSource) &&
    /seedActiveTabMeta\(openedTab\);[\s\S]*void refreshTabMetas\(\(\) => request\.seq === openTopicSeqRef\.current\);/.test(appSource) &&
    /showToast\(t\("history\.failedOpenSession"\), "error"\);[\s\S]*void refreshTabMetas\(\(\) => request\.seq === openTopicSeqRef\.current\);/.test(appSource),
  "opening topics coalesces pending navigation, ignores stale results, and seeds active tab metadata before background refresh",
);

ok(
  /if \(!singleSurfaceLayout\) return undefined;/.test(beginProjectTreeTopicOpenIntentBlock) &&
    /const transitionId = sessionPath\?\.trim\(\)[\s\S]*?beginSurfaceTransition\(\{ scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity \}\)[\s\S]*?: undefined;/.test(beginProjectTreeTopicOpenIntentBlock) &&
    /return token;/.test(beginProjectTreeTopicOpenIntentBlock) &&
    /preparedTransitionId \?\? beginSurfaceTransition\(\{ scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity \}\)/.test(openTopicBlock) &&
    /\{ scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity, transitionId \}/.test(openTopicBlock) &&
    /onBeginOpenTopicIntent=\{beginProjectTreeTopicOpenIntent\}/.test(appSource) &&
    /onCancelOpenTopicIntent=\{cancelPreparedTopicNavigation\}/.test(appSource) &&
    /if \(surfaceTransitionRef\.current\?\.seq !== seq\) return;/.test(sessionControllerSource),
  "project-tree path clicks prepare existing-session intent immediately, reuse it after click delay, and cancel only the matching transition",
);

ok(
  /<HeartbeatPanel[\s\S]*onOpenTopic=\{\(scope, workspaceRoot, topicId, sessionPath\) => \{[\s\S]*void handleOpenTopic\(scope, workspaceRoot, topicId, sessionPath\);[\s\S]*\}\}/.test(appSource),
  "heartbeat topic and archived-session navigation use the guarded open-topic path",
);

for (const selector of [
  ".app--darwin .app-chrome--tabs",
  ":root[data-theme-style] .app--darwin .app-chrome--tabs",
]) {
  const rightSpace = finalDeclaration(selector, "padding-right") ?? finalDeclaration(selector, "padding") ?? "";
  ok(
    rightSpace.includes("--chrome-toggle-size") && !rightSpace.includes("--chrome-right-toggle-offset"),
    `${selector} reserves fixed chrome tool width without shrinking for the right dock`,
  );
}

for (const selector of [
  ".app--windows .app-chrome--native-tabs",
  ".app--linux .app-chrome--native-tabs",
  ":root[data-theme-style] .app--windows .app-chrome--native-tabs",
  ":root[data-theme-style] .app--linux .app-chrome--native-tabs",
]) {
  const rightSpace = finalDeclaration(selector, "padding-right") ?? finalDeclaration(selector, "padding") ?? "";
  ok(
    rightSpace.includes("--chrome-right-toggle-offset"),
    `${selector} reserves right-dock width before rendering tabs`,
  );
}

for (const selector of [
  ".layout--workbench-chrome-hidden",
  ":root[data-theme-style] .layout--workbench-chrome-hidden",
]) {
  ok(
    finalDeclaration(selector, "--app-chrome-height") === "0px" &&
      finalDeclaration(selector, "grid-template-rows") === "minmax(0, 1fr)" &&
      finalDeclaration(selector, "background") === "var(--bg)",
    `${selector} removes the workbench chrome row`,
  );
}

ok(
  finalDeclaration(":root[data-theme-style] .app--darwin .layout--workbench-chrome-hidden", "--app-chrome-height") === "0px" &&
    finalDeclaration(".app--darwin .layout--workbench-chrome-hidden .sidebar--workbench", "padding-top") === "46px" &&
    finalDeclaration(".app--darwin .layout--workbench-chrome-hidden.layout--sidebar-collapsed .topicbar", "padding-left") === "96px",
  "macOS workbench leaves safe space for inset window controls",
);

ok(
  finalDeclaration(".app--darwin .layout--workbench-chrome-hidden.layout--workspace-maximized .workbench-dock__tools", "padding-left") === "96px",
  "macOS maximized workbench dock leaves safe space for inset window controls",
);

ok(
  /@media \(max-width: 820px\) \{[\s\S]*\.app--darwin \.layout--workbench-chrome-hidden \.topicbar\s*\{[\s\S]*padding-left:\s*96px;/.test(stylesSource) &&
    /@media \(max-width: 820px\) \{[\s\S]*\.app--darwin \.layout--workbench-chrome-hidden\.layout--workspace-maximized \.workbench-dock__tools\s*\{[\s\S]*padding-left:\s*96px;/.test(stylesSource),
  "macOS workbench keeps safe space when responsive CSS hides the sidebar",
);

ok(
  finalDeclaration(".workbench-dock__tools", "--wails-draggable") === "drag" &&
    finalDeclaration(".workbench-dock__tabs", "--wails-draggable") === "no-drag" &&
    finalDeclaration(".workbench-dock__tab", "--wails-draggable") === "no-drag",
  "maximized workbench dock keeps a draggable title region while tabs remain clickable",
);

for (const selector of [
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__chrome-btn",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__icon-btn",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__action-btn",
]) {
  ok(
    finalDeclaration(selector, "box-shadow") === "none",
    `${selector} stays flat after removing the workbench chrome row`,
  );
}

ok(
  finalDeclaration(":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar", "background") === "var(--bg-elev)",
  "workbench topic bar uses elevated background for light-mode white",
);

for (const selector of [
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__identity",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__title-row",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__title-row h1",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .tooltip-trigger:has(.topicbar__icon-btn)",
]) {
  ok(
    finalDeclaration(selector, "background") === "transparent" &&
      finalDeclaration(selector, "box-shadow") === "none" &&
      finalDeclaration(selector, "filter") === "none",
    `${selector} cannot paint residual title-row shadows in workbench mode`,
  );
}

for (const selector of [
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__icon-btn",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__chrome-btn",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__icon-btn:hover",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__icon-btn:focus-visible",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__chrome-btn:hover:not(.topicbar__chrome-btn--blocked)",
  ":root[data-theme-style] .layout--workbench-chrome-hidden .topicbar__chrome-btn:focus-visible:not(.topicbar__chrome-btn--blocked)",
]) {
  ok(
    finalDeclaration(selector, "background") === "transparent",
    `${selector} does not paint a hover block in workbench mode`,
  );
}

ok(
  finalDeclaration(".skip-to-composer", "box-shadow") === "none" &&
    finalDeclaration(".skip-to-composer:focus-visible", "box-shadow")?.includes("0 12px 28px"),
  "offscreen skip link does not leak its focus shadow into the workbench title area",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
