import { readFileSync } from "node:fs";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

const source = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
const composerSource = readFileSync(new URL("../features/composer/ui/Composer.tsx", import.meta.url), "utf8");
const projectComposerStart = source.indexOf("const renderProjectComposer");
const projectComposerEnd = source.indexOf("const openProjectCapability", projectComposerStart);
const projectComposer = source.slice(projectComposerStart, projectComposerEnd);
const prepareProjectComposerStart = source.indexOf("const prepareProjectComposerTab");
const prepareProjectComposerEnd = source.indexOf("const activateProjectComposerSession", prepareProjectComposerStart);
const prepareProjectComposer = source.slice(prepareProjectComposerStart, prepareProjectComposerEnd);
const activateExpertSelectionStart = source.indexOf("const activateExpertSelection");
const activateExpertSelectionEnd = source.indexOf("const optimisticExpertSelection", activateExpertSelectionStart);
const activateExpertSelection = source.slice(activateExpertSelectionStart, activateExpertSelectionEnd);

ok(
  projectComposerStart >= 0 && projectComposerEnd > projectComposerStart,
  "production Project Composer adapter is present",
);
ok(
  projectComposer.indexOf("if (adapter.sessionPath)") < projectComposer.indexOf("classifyProjectComposerFirstSend(displayText, submitText)"),
  "an existing pinned session restores ordinary target-Tab semantics before applying first-send classification",
);
ok(projectComposer.includes("return handleSend(displayText, submitText, capabilities)"), "resumed project sessions preserve ordinary app-local command behavior");
ok(projectComposer.includes("await sendToTab(adapter.tabId, displayText, submitText, capabilities)"), "resumed project turns submit to the exact project Tab");
ok(projectComposer.includes("submitProjectComposerTurn(adapter") && projectComposer.includes("setTokenMode: (tabId, mode) => app.SetTokenModeForTab(tabId, mode)"), "project turns use the executable target-Tab runtime including token mode");
ok(projectComposer.includes("await app.SteerForTab(adapter.tabId"), "running project sessions steer the exact project Tab");
ok(projectComposer.includes("return cancelForTab(adapter.tabId)"), "project Stop restores and cancels the exact retained project Tab");
ok(source.includes("projectComposerBlankSessionTarget(workspacePath, adapter.executionTarget)"), "project task target determines whether the draft uses a local workspace or cloud-only project scope");
ok(
  source.includes('tabID ? app.ModelsForTab(tabID) : app.Models()')
    && source.includes('app.Settings()')
    && source.includes('app.ModelMarketplace()')
    && /!activeTab\s*\?\s*\{ state: "allowed" \}/.test(source),
  "project Composer loads the same routes, Settings, and Marketplace facts without an ordinary conversation tab",
);
ok(
  projectComposer.includes('onCreateSession(tabId, adapter.executionTarget === "local" ? adapter.workspacePath : "", adapter.executionTarget)'),
  "project session creation forwards both the selected local workspace and the explicit execution target",
);
const promoteProjectComposerStart = source.indexOf("const promoteProjectComposerAdapter");
const promoteProjectComposerEnd = source.indexOf("const provisionNewCloudProjectConversation", promoteProjectComposerStart);
const promoteProjectComposer = source.slice(promoteProjectComposerStart, promoteProjectComposerEnd);
const provisionNewProjectComposerEnd = source.indexOf("const startNewCloudProjectConversation", promoteProjectComposerEnd);
const provisionNewProjectComposer = source.slice(promoteProjectComposerEnd, provisionNewProjectComposerEnd);
const startNewProjectComposerEnd = source.indexOf("const leaveEmptyProjectComposer", provisionNewProjectComposerEnd);
const startNewProjectComposer = source.slice(provisionNewProjectComposerEnd, startNewProjectComposerEnd);
const submittedDraftClearedStart = projectComposer.indexOf("onSubmittedDraftCleared");
const submittedDraftClearedEnd = projectComposer.indexOf("onCancel=", submittedDraftClearedStart);
const submittedDraftCleared = projectComposer.slice(submittedDraftClearedStart, submittedDraftClearedEnd);
ok(
  !promoteProjectComposer.includes("ensureNewProjectConversationAdapter")
    && provisionNewProjectComposer.includes("ensureNewProjectConversationAdapter")
    && !provisionNewProjectComposer.includes("openProjectDirectory")
    && !provisionNewProjectComposer.includes("openCloudProject")
    && startNewProjectComposer.includes("provisionNewCloudProjectConversation(projectId)")
    && startNewProjectComposer.includes("openCloudProject(normalizedProjectId)")
    && submittedDraftCleared.includes("provisionNewCloudProjectConversation(projectId)")
    && !submittedDraftCleared.includes("startNewCloudProjectConversation")
    && !submittedDraftCleared.includes("openProjectDirectory")
    && !submittedDraftCleared.includes("openCloudProject"),
  "accepted project turns provision the next draft after clearing without reopening the project workspace",
);
ok(
  projectComposer.includes("selectedConnectors={enabledComposerConnectors}"),
  "project Composer renders the same account-wide connector switches as ordinary tasks",
);
ok(
  source.includes("projectComposerCapabilitiesForTab(projectComposerAdaptersRef.current, activeTabId)")
    && source.includes("resolveAvailableProjectCapabilities(activeProjectCapabilities)")
    && source.includes("projectComposerCapabilitySubmit(submitText, availableProjectCapabilities, capabilities, activeProjectCapabilities)"),
  "later turns on a promoted project Tab keep the selected project capability scope",
);
ok(
  prepareProjectComposer.includes("ensureBlankSurface") &&
    prepareProjectComposer.includes("ensureBlankTab") &&
    !prepareProjectComposer.includes("openBlankSession"),
  "the first project send prepares its runtime Tab without opening the ordinary blank-session preview",
);
ok(
  prepareProjectComposer.indexOf("openCloudProject(adapter.projectId)") < prepareProjectComposer.indexOf("await ensureBlankSurface"),
  "the project workspace remains visible while its runtime Tab is prepared",
);
ok(
  activateExpertSelection.includes('navigation: "chat" | "preserve" = "chat"')
    && activateExpertSelection.includes('if (navigation === "chat") setWorkspaceView("chat")')
    && prepareProjectComposer.includes('activateExpertSelection(adapter.activeExpert, adapter.tabId, undefined, "preserve")')
    && prepareProjectComposer.includes('activateExpertSelection(desiredExpert, tabId, undefined, "preserve")'),
  "preparing a project expert preserves the project surface until the accepted turn promotes once to chat",
);
ok(projectComposer.includes('adapter.executionTarget === "local" && !adapter.workspacePath.trim()'), "local project tasks require an explicit workspace binding");
ok(projectComposer.includes('localWorkspaceMissing ? "请选择工作空间后再发送"'), "the missing-workspace gate explains how to continue");
ok(
  projectComposer.includes("onCloudProjectIdChange: (nextProjectId)") &&
    projectComposer.includes('if (!nextAdapter.sessionPath) nextAdapter.executionTarget = "cloud"') &&
    projectComposer.includes("openCloudProject(normalizedProjectId)"),
  "cloud task project selection opens the selected cloud project with a cloud-scoped draft",
);
ok(!projectComposer.includes('if (executionTarget === "cloud") current.workspacePath = ""'), "switching to cloud execution preserves the local workspace draft for switching back");
ok(projectComposer.includes("initializeProjectComposerDefaults(adapter, subscriptionRepairSettings)"), "project drafts initialize from the same settings snapshot as new conversations");
ok(
  projectComposer.includes("modelLabel={adapter.modelRef}")
    && composerSource.includes('unmatchedLabel={projectPresentation ? "Auto" : undefined}'),
  "project model passes its real ref and reserves Auto for a genuinely empty project selection",
);
ok(
  /onSwitchModel=\{async \(modelRef, billingSource\) => \{[\s\S]*?if \(!adapter\.tabId\) \{[\s\S]*?current\.modelRef = modelRef;[\s\S]*?return;[\s\S]*?\}[\s\S]*?SetModel/.test(projectComposer),
  "choosing a model in an unstarted project draft stays local and does not prepare a global blank Tab",
);
ok(projectComposer.includes("effort={adapter.effort}"), "project effort never falls back to ordinary active Tab state");
ok(!projectComposer.includes("adapter.effort || state.effort"), "ordinary effort fallback is absent");
ok(
  source.includes("app.EffortForDraftModel(target.scope, target.workspaceRoot, modelRef")
    && source.includes("if (current.tabId || current.effortRouteKey !== routeKey) return"),
  "project drafts load model effort capability without creating a Tab and ignore obsolete results",
);
ok(
  /onSetEffort=\{async \(level\) => \{[\s\S]*?if \(!adapter\.tabId\) \{[\s\S]*?current\.effort = \{ \.\.\.current\.effort, current: level \};[\s\S]*?return;[\s\S]*?SetEffortForTab/.test(projectComposer),
  "choosing project draft effort stays local until the real Tab exists",
);
ok(projectComposer.includes("activeExpert={adapter.activeExpert ?? null}"), "project expert state is rendered from the project adapter");
ok(
  projectComposer.includes("const projectInsertRequest = projectComposerInsertRequests[sessionKey] ?? composerInsertRequest")
    && projectComposer.includes("insertRequest={projectInsertRequest}")
    && projectComposer.includes("handleComposerInsertRequestConsumed(id)"),
  "project Composer consumes the global expert trial prompt while preserving scoped skill and connector insertions",
);
ok(
  projectComposer.includes('await activateExpertSelection(selection, tabId, undefined, "preserve")')
    && !projectComposer.includes("reconcileProjectExpertSelection("),
  "project experts are enabled only after an explicit Composer selection",
);
ok(
  projectComposer.includes("if (adapter.tabId) await app.ClearExpertForTab(adapter.tabId)")
    && !projectComposer.includes("await prepareProjectComposerTab(adapter, adapter.workspacePath);\n            await app.ClearExpertForTab"),
  "clearing a project expert stays local until a project Tab has already been prepared",
);
ok(projectComposer.includes("重新打开") && projectComposer.includes("discardProjectComposerAdapter"), "the ninth-draft gate exposes reopen and discard actions");
ok(source.includes('href={skipToProjectComposerAvailable || skipToComposerAvailable ? "#composer-input"'), "the project skip link targets the real production editor");
ok(source.includes("planProjectComposerScopeChange") && source.includes("for (const tabId of closeTabIds)"), "scope change consumes the phase-aware cleanup plan and closes prepared drafts");
ok(
  source.includes('const projectAccountScopeKey = hasRecoverableCloudIdentity(sidebarAccount) ? sidebarAccountScopeKey : ""')
    && source.includes('setWorkspaceView((current) => current === "projects" ? "chat" : current)')
    && source.includes("setProjectRevision((revision) => revision + 1)"),
  "sign-out invalidates the authenticated project scope, exits the project surface, and refreshes the sidebar tree",
);
ok(!source.includes("const renderProjectConversation"), "project sessions are not rendered as a project workspace tab");
ok(promoteProjectComposer.includes("showChatView(false)") && !source.includes("renderProjectConversation={"), "accepted project turns remain on the ordinary task selected in the sidebar");
ok(
  projectComposer.includes('session-composer-region--history') &&
    projectComposer.includes('inert={expertMemberHistoryFocused ? true : undefined}') &&
    projectComposer.includes('aria-hidden={expertMemberHistoryFocused || undefined}'),
  "focused project expert history makes the existing Composer content inert and hidden",
);
ok(
  /disabled=\{expertMemberHistoryFocused \|\|/.test(projectComposer),
  "focused project expert history disables project Composer submission",
);
ok(
  projectComposer.includes("{expertMemberHistoryFocused && (") &&
    projectComposer.includes('className="session-composer-history-overlay"') &&
    projectComposer.includes("onClick={() => expertMemberHistoryReturnRef.current()}") &&
    projectComposer.includes('t("agentWorkspace.returnMain")'),
  "focused project expert history exposes the shared return-to-lead action",
);
ok(
  source.includes("setExpertMemberHistoryFocused(focused)") &&
    source.includes("expertMemberHistoryReturnRef.current = focused && returnToLead ? returnToLead : () => {}"),
  "returning to the lead clears shared focus and restores the project Composer through the existing handler",
);
ok(
  projectComposer.includes("onSubmitError={(error) =>") &&
    projectComposer.includes('asProjectComposerSubmitError("prepare-tab", error)'),
  "unexpected project submit failures retain safe project diagnostics instead of using the generic Composer error",
);

console.log("project Composer App production contract passed");
