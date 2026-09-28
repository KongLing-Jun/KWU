// Run: tsx src/__tests__/expert-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  pendingExpertActivationIdForTab,
  removeDeletedExpertSelection,
  resolveExpertRefreshSelection,
} from "../lib/expertSelection";

let passed = 0;
let failed = 0;

function ok(cond: boolean, label: string) {
  if (cond) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nexpert bridge contract");

const backendTeamSelection = {
  id: "finance-expert-team",
  name: "Finance Expert Team",
  role: "Finance",
  mode: "team",
  prompt: "Diagnose quarterly finance data.",
};
ok(
  resolveExpertRefreshSelection({
    previous: backendTeamSelection,
    next: null,
    canClear: true,
    backendExpertIds: new Set(["finance-expert-team"]),
    pendingActivationId: "finance-expert-team",
  })?.id === "finance-expert-team",
  "pending expert-team activation keeps the composer chip visible across an empty refresh",
);
ok(
  resolveExpertRefreshSelection({
    previous: backendTeamSelection,
    next: null,
    canClear: true,
    backendExpertIds: new Set(["finance-expert-team"]),
    pendingActivationId: null,
  }) === null,
  "confirmed backend expert selections still clear when the active session reports no expert",
);
ok(
  resolveExpertRefreshSelection({
    previous: backendTeamSelection,
    next: { ...backendTeamSelection, id: "architecture-review", name: "Architecture Review", mode: "expert" },
    canClear: true,
    backendExpertIds: new Set(["finance-expert-team", "architecture-review"]),
    pendingActivationId: "finance-expert-team",
  })?.id === "finance-expert-team",
  "pending expert-team activation ignores a stale backend expert echo",
);
ok(
  removeDeletedExpertSelection(backendTeamSelection, "finance-expert-team") === null,
  "deleting the selected user expert clears its frontend selection",
);
ok(
  pendingExpertActivationIdForTab({ expertId: "finance-expert-team", tabId: "tab-a", requestSequence: 3 }, "tab-b") === null,
  "an activation pending in another task cannot mask the current task expert",
);
ok(
  resolveExpertRefreshSelection({
    previous: backendTeamSelection,
    next: { ...backendTeamSelection, id: "architecture-review", name: "Architecture Review", mode: "expert" },
    canClear: true,
    backendExpertIds: new Set(["finance-expert-team", "architecture-review"]),
    pendingActivationId: pendingExpertActivationIdForTab(
      { expertId: "finance-expert-team", tabId: "tab-a", requestSequence: 3 },
      "tab-b",
    ),
  })?.id === "architecture-review",
  "a tab-scoped refresh restores the current task instead of a pending expert from another task",
);
ok(
  resolveExpertRefreshSelection({
    previous: backendTeamSelection,
    next: backendTeamSelection,
    canClear: true,
    backendExpertIds: new Set(["finance-expert-team"]),
    pendingClear: true,
  }) === null,
  "an in-flight clear does not flash the stale expert back into the composer",
);


const here = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(here, "../App.tsx"), "utf8");
const composerSource = readFileSync(resolve(here, "../features/composer/ui/Composer.tsx"), "utf8");
const expertDirectorySource = readFileSync(resolve(here, "../features/experts/ui/ExpertDirectory.tsx"), "utf8");
const transcriptSource = readFileSync(resolve(here, "../components/Transcript.tsx"), "utf8");
const expertAssetsSource = readFileSync(resolve(here, "../lib/expertAssets.ts"), "utf8");
const expertSelectionSource = readFileSync(resolve(here, "../lib/expertSelection.ts"), "utf8");
const expertCatalogControllerSource = readFileSync(resolve(here, "../features/experts/controller/useExpertCatalog.ts"), "utf8");
const expertStateProjectionSource = readFileSync(resolve(here, "../features/experts/model/expertSelection.ts"), "utf8");
const zhSource = readFileSync(resolve(here, "../locales/zh.ts"), "utf8");
const openExpertDirectorySource = appSource.match(/const openExpertDirectory = useCallback[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const openCapabilityWorkspaceSource = appSource.match(/const openCapabilityWorkspace = useCallback[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const handleTabChangeSource = appSource.match(/const handleTabChange[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const refreshExpertStateSource = expertCatalogControllerSource.match(/const refreshExpertState[\s\S]*?\}, \[[^\]]*\]\);/)?.[0] ?? "";
const handleOpenTopicSource = appSource.match(/const handleOpenTopic[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const openSidebarImConnectionSessionSource = appSource.match(/const openSidebarImConnectionSession[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const openBlankSessionSource = appSource.match(/const openBlankSession[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const expertNewTaskTargetSource = appSource.match(/const expertNewTaskTarget[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const waitForExpertTargetReadySource = appSource.match(/const waitForExpertTargetReady[\s\S]*?\}, \[[^\]]*\]\);/)?.[0] ?? "";
const activateExpertSelectionSource = appSource.match(/const activateExpertSelection[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const summonExpertToNewTaskSource = appSource.match(/const summonExpertToNewTask[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const activateExpertForCurrentTaskOrNewTaskSource = appSource.match(/const activateExpertForCurrentTaskOrNewTask[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const handleSummonExpertSource = appSource.match(/const handleSummonExpert[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const handleSelectRecentExpertSource = appSource.match(/const handleSelectRecentExpert[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const confirmExpertTeamSwitchSource = appSource.match(/const confirmExpertTeamSwitch[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const clearActiveExpertSelectionSource = appSource.match(/const clearActiveExpertSelection[\s\S]*?\}, \[[^\]]+\]\);/)?.[0] ?? "";
const projectClearExpertSource = appSource.match(/onClearExpert=\{async \(\) => \{[\s\S]*?\}\}\s*activeExpert=/)?.[0] ?? "";
const deleteUserExpertStart = expertCatalogControllerSource.indexOf("const deleteExpert");
const deleteUserExpertEnd = expertCatalogControllerSource.indexOf("const deleteUserExpert", deleteUserExpertStart);
const deleteUserExpertSource = deleteUserExpertStart >= 0 && deleteUserExpertEnd > deleteUserExpertStart
  ? expertCatalogControllerSource.slice(deleteUserExpertStart, deleteUserExpertEnd)
  : "";
const expertDirectorySummonSource = expertDirectorySource.match(/const summon[\s\S]*?\n  \};/)?.[0] ?? "";
const onResumeSessionStart = appSource.indexOf("const onResumeSession");
const openPaletteStart = appSource.indexOf("const openPalette", onResumeSessionStart);
const onResumeSessionSource =
  onResumeSessionStart >= 0 && openPaletteStart > onResumeSessionStart
    ? appSource.slice(onResumeSessionStart, openPaletteStart)
    : "";

ok(appSource.includes("app.SetExpertForTab(targetTabId, selection.id)"), "App calls tab-scoped backend SetExpert for backend experts");
ok(
  openExpertDirectorySource.includes("void refreshExpertState();") &&
    openExpertDirectorySource.includes("navigateToExpertDirectory();") &&
    openExpertDirectorySource.indexOf("void refreshExpertState();") < openExpertDirectorySource.indexOf("navigateToExpertDirectory();") &&
    openCapabilityWorkspaceSource.includes('if (view === "experts")') &&
    openCapabilityWorkspaceSource.includes("openExpertDirectory();"),
  "every expert navigation click forces a fresh backend expert and expert-team catalog request",
);
ok(
  appSource.includes("projectExpertSelections(capabilityBindings, expertCatalog, userExpertCatalog)") &&
    appSource.includes("recentExperts={projectExperts}") &&
    activateExpertSelectionSource.includes("app.SetExpertForTab(targetTabId, selection.id)"),
  "project tasks offer every bound expert while activating one expert at a time",
);
ok(
  !appSource.includes("reconcileProjectExpertSelection(") &&
    appSource.includes("recentExperts={projectExperts}"),
  "project expert bindings remain selectable without enabling an expert automatically",
);
ok(
  projectClearExpertSource.includes("if (adapter.tabId) await app.ClearExpertForTab(adapter.tabId)") &&
    !projectClearExpertSource.includes("prepareProjectComposerTab("),
  "clearing a project expert does not prepare a tab or navigate away from the project composer",
);
ok(
  appSource.includes("const target = blankSessionTarget()") &&
    appSource.includes("await openBlankSession(target.scope, target.workspaceRoot)") &&
    appSource.includes("activeExpert={displayedActiveExpert}"),
  "App opens a new targeted task before activating an expert and passes the atomic displayed expert state to Composer",
);
ok(
  openBlankSessionSource.includes("const meta =") &&
    openBlankSessionSource.includes("seedActiveTabMeta(meta)") &&
    openBlankSessionSource.indexOf("seedActiveTabMeta(meta)") < openBlankSessionSource.indexOf("await refreshTabMetas("),
  "App seeds blank-session tab metadata before composer insert requests can target the wrong draft",
);
ok(
  summonExpertToNewTaskSource.includes("optimisticExpertSelection(selection)") &&
    summonExpertToNewTaskSource.indexOf("optimisticExpertSelection(selection)") < summonExpertToNewTaskSource.indexOf("await openBlankSession(target.scope, target.workspaceRoot)") &&
    summonExpertToNewTaskSource.includes("await activateExpertSelection(selection, targetTabId, request)"),
  "App shows the selected expert immediately and activates it on the newly opened task",
);
ok(
  waitForExpertTargetReadySource.includes("tab.id === targetTabId") &&
    activateExpertSelectionSource.includes("requestedTabId ?? activeTabIdRef.current") &&
    activateExpertSelectionSource.includes("enqueueExpertMutation(async () =>") &&
    activateExpertSelectionSource.includes("await waitForExpertTargetReady(targetTabId)") &&
    activateExpertSelectionSource.indexOf("await waitForExpertTargetReady(targetTabId)") < activateExpertSelectionSource.indexOf("app.SetExpertForTab(targetTabId, selection.id)"),
  "App serializes expert mutations and waits for the explicit target before calling SetExpert on that task",
);
ok(
  activateExpertForCurrentTaskOrNewTaskSource.includes("app.CurrentExpertForTab(targetTabId)") &&
    activateExpertForCurrentTaskOrNewTaskSource.includes("current.canSwitch || current.selectedId === selection.id") &&
    activateExpertForCurrentTaskOrNewTaskSource.includes("await summonExpertToNewTask(selection)") &&
    handleSummonExpertSource.includes("await activateExpertForCurrentTaskOrNewTask(selection)") &&
    handleSelectRecentExpertSource.includes("await activateExpertForCurrentTaskOrNewTask(selection)"),
  "directory and recent-expert switches open a new task when conversation content makes in-place activation unsafe",
);
ok(
  activateExpertSelectionSource.includes("void refreshExpertState(targetTabId);") &&
    activateExpertSelectionSource.includes("if (activeTabIdRef.current === targetTabId)") &&
    !/refreshExpertState\(\)\.then\([\s\S]*pendingExpertActivationIdRef\.current = null/.test(activateExpertSelectionSource) &&
    expertCatalogControllerSource.includes("pendingExpertActivationIdForTab(pendingActivation, targetTabId)"),
  "App keeps pending activation until confirmation and ignores responses after leaving the target task",
);
ok(
  appSource.includes("activeProjectComposerAdapter.availableExperts : recentComposerExperts") &&
    appSource.includes("onSelectRecentExpert={handleSelectRecentExpert}") &&
    activateExpertSelectionSource.includes('binding === "attach"') &&
    activateExpertSelectionSource.includes("app.AttachExpertForTab(targetTabId, selection.id)"),
  "App passes project and recent expert selections through the scoped activation gate",
);
ok(
  appSource.includes("const transcriptFileTabId = surfaceTransitionActive") &&
    appSource.includes("? state.hydratePlaceholderTabId") &&
    appSource.includes("tabId={transcriptFileTabId}"),
  "hydration placeholder file links keep their source tab workspace owner",
);
ok(
  expertSelectionSource.includes("backendExpertIds.has(previous.id) ? null : previous"),
  "App preserves frontend-only expert selections across backend expert refreshes",
);
ok(
  deleteUserExpertSource.includes("removeDeletedExpertSelection") &&
    deleteUserExpertSource.includes("setRecentExperts") &&
    deleteUserExpertSource.indexOf("removeDeletedExpertSelection") < deleteUserExpertSource.indexOf("await refreshExpertState()"),
  "deleting a user expert clears active and recent frontend selections before refresh",
);
ok(
  appSource.includes("expertRefreshCanClearSelection") &&
    refreshExpertStateSource.includes("resolveExpertRefreshSelection") &&
    refreshExpertStateSource.includes("canClear: canClearSelection") &&
    expertSelectionSource.includes("!canClear") &&
    expertSelectionSource.includes("return previous"),
  "App keeps the composer expert chip visible while a session switch is still hydrating",
);
ok(
  appSource.includes("clearExpertErrorToastOpenRef") &&
    clearActiveExpertSelectionSource.includes("clearExpertErrorToastOpenRef.current") &&
    clearActiveExpertSelectionSource.includes("onDismiss") &&
    clearActiveExpertSelectionSource.includes("enqueueExpertMutation(() => app.ClearExpertForTab(targetTabId))") &&
    clearActiveExpertSelectionSource.includes("activeTabIdRef.current === targetTabId") &&
    clearActiveExpertSelectionSource.indexOf("clearExpertErrorToastOpenRef.current") < clearActiveExpertSelectionSource.indexOf("enqueueExpertMutation(() => app.ClearExpertForTab(targetTabId))"),
  "App suppresses repeated clear failures and does not restore a cleared expert into another task",
);
ok(
  handleOpenTopicSource.includes("showChatView(false)") && !handleOpenTopicSource.includes("showChatView(true)") && !handleOpenTopicSource.includes("showChatView(!openingActiveTopic)"),
  "App keeps the active expert while opening saved topic sessions until the target session expert is restored",
);
ok(
  handleTabChangeSource.includes("showChatView(false)") && !handleTabChangeSource.includes("showChatView(true)"),
  "App keeps the active expert while switching session tabs until the target session expert is restored",
);
ok(
  openSidebarImConnectionSessionSource.includes("showChatView(false)") && !openSidebarImConnectionSessionSource.includes("showChatView(true)") &&
    onResumeSessionSource.includes("showChatView(false)") && !onResumeSessionSource.includes("showChatView(true)"),
  "App keeps the active expert while opening sidebar or history sessions until the target session expert is restored",
);
ok(
  !appSource.includes("expert-active-banner") && composerSource.includes("composer-action-trigger--expert") && !composerSource.includes("composer-expert-chip"),
  "Active expert remains visible as a removable composer status control",
);
ok(
  composerSource.includes("hubSection === \"experts\"") && composerSource.includes("composer-recent-expert-menu") && composerSource.includes("chooseRecentExpert"),
  "Unified composer add menu exposes the recent expert panel",
);
ok(
  appSource.includes("activeExpert?.mode === \"team\"") &&
    appSource.includes("setExpertTeamSwitchConfirm({ kind: \"recent\", selection })") &&
    appSource.includes("setExpertTeamSwitchConfirm({ kind: \"directory\" })") &&
    appSource.includes("expertDirectoryUsesCurrentTaskRef.current = true") &&
    zhSource.includes("切换专家团将开启新对话"),
  "App confirms expert-team switches before opening a new task",
);
ok(
  !appSource.includes("selectedComposerWorkspace") &&
    !appSource.includes("showComposerWorkspacePicker") &&
    !appSource.includes("pickComposerWorkspace") &&
    !appSource.includes("app.ListWorkspaces()"),
  "App no longer owns a composer-only workspace target",
);
ok(
  appSource.includes('const blankSessionTarget = useCallback(() => ({ scope: "global", workspaceRoot: "" }), []);'),
  "App starts composer-created new tasks from the global target after removing the workspace selector",
);
ok(
  expertNewTaskTargetSource.includes('activeTab?.scope === "project"') &&
    expertNewTaskTargetSource.includes("activeTab.workspaceRoot") &&
    expertNewTaskTargetSource.includes('return { scope: "project", workspaceRoot: activeTab.workspaceRoot }') &&
    summonExpertToNewTaskSource.includes("const target = expertNewTaskTarget()") &&
    confirmExpertTeamSwitchSource.includes("const target = expertNewTaskTarget()"),
  "App starts expert-summoned new tasks from the active project workspace when one is active",
);
ok(
  !appSource.includes("showWorkspacePicker=") &&
    !composerSource.includes("composer-workspace-trigger") &&
    !composerSource.includes("composer-workspace-menu"),
  "Composer no longer renders the workspace selector in the prompt bar",
);
ok(
  !appSource.includes("selection.mode === \"expert\"") &&
    appSource.includes("activatableExpertIds.has(selection.id)") &&
    expertCatalogControllerSource.includes("expertActivationIds(experts, userExperts)"),
  "App sends built-in and user expert ids to SetExpert regardless of display mode",
);
ok(
  refreshExpertStateSource.includes("const catalogPromise = expertCatalogBridge.ListExperts()") &&
    refreshExpertStateSource.indexOf("const catalogPromise = expertCatalogBridge.ListExperts()") <
      refreshExpertStateSource.indexOf("if (!targetTabId)") &&
    refreshExpertStateSource.includes("expertCatalogBridge.ListUserExperts().catch(() => [])") &&
    refreshExpertStateSource.includes("setUserExperts(Array.isArray(nextUserExperts) ? nextUserExperts : [])") &&
    refreshExpertStateSource.includes("setExperts(Array.isArray(catalog) ? catalog : [])"),
  "expert and current-user catalogs load from the backend even when no session tab is active",
);
ok(
  refreshExpertStateSource.includes("expertCatalogBridge.ListUserExpertsForTab(targetTabId)") &&
    refreshExpertStateSource.includes("expertCatalogBridge.CurrentExpertForTab(targetTabId)") &&
    refreshExpertStateSource.includes("sequence === refreshSequenceRef.current") &&
    refreshExpertStateSource.includes("activeTabRef.current === targetTabId") &&
    refreshExpertStateSource.includes("pendingExpertActivationIdForTab(pendingActivation, targetTabId)") &&
    refreshExpertStateSource.includes("expertActivationIds(normalizedExperts, normalizedUserExperts)") &&
    refreshExpertStateSource.includes("expertSelectionFromState(expertState, normalizedExperts, normalizedUserExperts)"),
  "expert refresh is tab-scoped, race-guarded, and restores user expert session selection",
);
ok(
  !appSource.includes("systemPrompt: userInfo?.systemPrompt") &&
    activateExpertSelectionSource.includes("systemPrompt: undefined"),
  "backend-activated user experts do not duplicate their system prompt in every hidden user turn",
);
ok(
  appSource.includes("setComposerInsertRequest({ id: Date.now(), text: selection.prompt, mode: \"replace\" })"),
  "App inserts the selected expert display prompt into the composer",
);
ok(
  composerSource.includes("buildExpertSubmitText(submitText, activeExpert)") &&
    expertDirectorySummonSource.includes("members: expertMembersWithAvatars(item.members)") &&
    expertDirectorySummonSource.includes("systemPrompt: item.systemPrompt"),
  "Composer sends hidden expert submit context while ExpertDirectory preserves team members and system prompt",
);
ok(
  appSource.includes("await commitThenSendRef.current(trimmed, `/goal ${trimmed}`)"),
  "Goal-mode first send stores the visible prompt instead of the hidden expert submit wrapper",
);
ok(
  transcriptSource.includes("<AgentWorkspace") &&
    transcriptSource.includes("overviewContent") &&
    transcriptSource.includes("sessionItemsByParent") &&
    transcriptSource.includes("expertWorkspaceItems"),
  "Transcript renders expert-team child sessions through the full-window AgentWorkspace",
);
ok(
  expertDirectorySource.includes("backendExperts?: ExpertInfo[]") && expertDirectorySource.includes("userExperts?: UserExpertInfo[]") && expertDirectorySource.includes("mergeBackendExperts") && expertDirectorySource.includes("avatarSrc: item.avatarSrc"),
  "ExpertDirectory accepts backend and user expert catalogs",
);
ok(
  expertDirectorySource.includes("members: expertMembersWithAvatars(info.members") &&
    expertDirectorySource.includes("draftMembersForSave(draft.mode, draft.members)") &&
    expertDirectorySource.includes("members: draft.mode === \"team\" ? members : undefined"),
  "ExpertDirectory echoes user expert team members through edit/save",
);
ok(
  expertDirectorySource.includes("systemPrompt: draft.systemPrompt") &&
    expertDirectorySource.includes("prompts,") &&
    expertDirectorySource.includes("displayPrompt: prompts[0]"),
  "ExpertDirectory saves optional preset questions separately from the system prompt",
);
ok(
  expertDirectorySource.includes("expertLibraryState?: ExpertLibraryState") && expertDirectorySource.includes("favoriteExpertIds") && expertDirectorySource.includes("recentExpertIds"),
  "ExpertDirectory uses backend library state for favorite and recent filters",
);
ok(
  expertDirectorySource.includes("onImportExpertConfig?:") && expertDirectorySource.includes("parseImportPayload"),
  "ExpertDirectory parses import schema before saving",
);
ok(
  expertDirectorySource.includes("onDeleteUserExpert?:") && expertDirectorySource.includes("deleteUserExpert") && expertDirectorySource.includes("Trash2"),
  "ExpertDirectory exposes delete action for user experts",
);
ok(
  expertCatalogControllerSource.includes("expertCatalogBridge.ResetExperts()") &&
    expertCatalogControllerSource.includes("setRecentExperts([])") &&
    !expertDirectorySource.includes("onResetExperts?:") &&
    !expertDirectorySource.includes("handleResetExperts") &&
    !expertDirectorySource.includes("expert-reset-button"),
  "reset capability remains backend-only and ExpertDirectory omits the reset action",
);
ok(
  expertDirectorySource.includes("deleteConfirmId") &&
    expertDirectorySource.includes("renderDeleteExpertControl") &&
    expertDirectorySource.includes("delete-cancel") &&
    !expertDirectorySource.includes("window.confirm"),
  "ExpertDirectory uses themed delete confirmation instead of the native browser confirm dialog",
);
ok(
    expertStateProjectionSource.includes("expertAvatarSrcForInfo") &&
    expertStateProjectionSource.includes("expertMemberAvatarSrc") &&
    expertDirectorySource.includes("expertMemberWithAvatar") &&
    expertAssetsSource.includes("data-analysis-team") &&
    expertAssetsSource.includes("指标体系专家"),
  "Expert team avatars are restored from shared static assets after window switches",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
