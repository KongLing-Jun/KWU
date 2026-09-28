import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/workspace-layout.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  availableWorkspacePanelWidth,
  resolveLiveWorkspacePanelWidth,
  resolveWorkspacePanelWidth,
  workspacePanelCanOpen,
  workspacePanelAriaMinWidth,
} from "../lib/workspaceLayout";
import * as workspaceLayout from "../lib/workspaceLayout";

let passed = 0;
let failed = 0;
const testDir = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");
const workspaceDockSource = readFileSync(resolve(testDir, "../features/workspace/controller/useWorkspaceDockLayout.ts"), "utf8");
const appChromeSource = readFileSync(resolve(testDir, "../components/AppChrome.tsx"), "utf8");
const workspacePanelSource = readFileSync(resolve(testDir, "../features/workspace/ui/WorkspacePanel.tsx"), "utf8");
const tooltipSource = readFileSync(resolve(testDir, "../components/Tooltip.tsx"), "utf8");
const styles = loadStylesSource();
const desktopMainSource = readFileSync(resolve(testDir, "../../../main.go"), "utf8");
const zhLocaleSource = readFileSync(resolve(testDir, "../locales/zh.ts"), "utf8");

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

const CHAT_MIN_WIDTH = 400;
const SIDEBAR_WIDTH = 264;
const RESIZER_WIDTH = 8;
const PREVIEW_MIN_WIDTH = 420;
const PREVIEW_DEFAULT_WIDTH = 660;
const CHAT_COMFORT_MIN_WIDTH = 560;
const RIGHT_DOCK_MIN_RENDER_WIDTH = 280;

console.log("\nworkspace dock layout");

const expandedAvailable = availableWorkspacePanelWidth({
  viewportWidth: 1280,
  sidebarCollapsed: false,
  sidebarWidth: SIDEBAR_WIDTH,
  chatMinWidth: CHAT_MIN_WIDTH,
  resizerWidth: RESIZER_WIDTH,
});
eq(expandedAvailable, 608, "1280px viewport leaves room for an expanded-sidebar dock");
eq(
  resolveWorkspacePanelWidth({
    open: true,
    maximized: false,
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
    availableWidth: expandedAvailable,
  }),
  608,
  "expanded-sidebar preview clamps to available width instead of overflowing",
);

const collapsedAvailable = availableWorkspacePanelWidth({
  viewportWidth: 1280,
  sidebarCollapsed: true,
  sidebarWidth: SIDEBAR_WIDTH,
  chatMinWidth: CHAT_MIN_WIDTH,
  resizerWidth: RESIZER_WIDTH,
});
eq(collapsedAvailable, 872, "collapsed sidebar restores workspace room");
eq(
  resolveWorkspacePanelWidth({
    open: true,
    maximized: false,
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
    availableWidth: collapsedAvailable,
  }),
  PREVIEW_DEFAULT_WIDTH,
  "wide-enough collapsed layout keeps the preferred preview width",
);

const narrowAvailable = availableWorkspacePanelWidth({
  viewportWidth: 900,
  sidebarCollapsed: false,
  sidebarWidth: SIDEBAR_WIDTH,
  chatMinWidth: CHAT_MIN_WIDTH,
  resizerWidth: RESIZER_WIDTH,
});
const narrowRendered = resolveWorkspacePanelWidth({
  open: true,
  maximized: false,
  preferredWidth: PREVIEW_DEFAULT_WIDTH,
  minWidth: PREVIEW_MIN_WIDTH,
  availableWidth: narrowAvailable,
});
eq(narrowAvailable, 228, "very narrow viewports may leave less than the nominal dock minimum");
eq(narrowRendered, 228, "very narrow dock still stays inside the viewport");
eq(workspacePanelAriaMinWidth(PREVIEW_MIN_WIDTH, narrowRendered), 228, "ARIA minimum follows constrained rendered width");

eq(
  resolveWorkspacePanelWidth({
    open: false,
    maximized: false,
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
    availableWidth: 0,
  }),
  PREVIEW_DEFAULT_WIDTH,
  "closed panel preserves the saved preferred width",
);
eq(
  resolveWorkspacePanelWidth({
    open: true,
    maximized: true,
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
    availableWidth: 228,
  }),
  PREVIEW_DEFAULT_WIDTH,
  "maximized panel preserves the saved preferred width",
);
eq(
  workspacePanelCanOpen({
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
    availableWidth: 228,
    minRenderWidth: RIGHT_DOCK_MIN_RENDER_WIDTH,
  }),
  false,
  "workspace expand control is disabled when opening cannot reach render width",
);
eq(
  workspacePanelCanOpen({
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
    availableWidth: 300,
    minRenderWidth: RIGHT_DOCK_MIN_RENDER_WIDTH,
  }),
  true,
  "workspace expand control is enabled once opening can reach render width",
);

eq(
  resolveLiveWorkspacePanelWidth({
    viewportWidth: 1268,
    sidebarCollapsed: false,
    sidebarWidth: 400,
    chatMinWidth: CHAT_COMFORT_MIN_WIDTH,
    resizerWidth: RESIZER_WIDTH,
    open: true,
    maximized: false,
    preferredWidth: PREVIEW_MIN_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
  }),
  300,
  "live dock drag clamps the hard minimum to the available dock width",
);

eq(
  resolveLiveWorkspacePanelWidth({
    viewportWidth: 1280,
    sidebarCollapsed: false,
    sidebarWidth: 500,
    chatMinWidth: CHAT_COMFORT_MIN_WIDTH,
    resizerWidth: RESIZER_WIDTH,
    open: true,
    maximized: false,
    preferredWidth: PREVIEW_DEFAULT_WIDTH,
    minWidth: PREVIEW_MIN_WIDTH,
  }),
  212,
  "live sidebar drag recomputes dock width from the dragged sidebar width",
);
eq(
  /const closeWorkspacePanel = useCallback\(\(\) => \{[\s\S]*?setLiveWorkspacePanelRenderWidth\(null\);[\s\S]*?open:\s*false,[\s\S]*?maximized:\s*false,/.test(appSource),
  true,
  "closing the dock clears the transient render width before hiding the panel",
);
eq(
  /workspacePanelExpandAvailableWidth[\s\S]*?chatMinWidth:\s*CHAT_COMFORT_MIN_WIDTH/.test(workspaceDockSource),
  true,
  "workspace expand availability reserves the open-state chat comfort width",
);
eq(
  /const workspacePanelExpandDisabled =[\s\S]*?!workspacePanelCanOpen/.test(workspaceDockSource),
  true,
  "App computes a disabled state before the workspace panel can render",
);
eq(
  /if\s*\(workspacePanelExpandDisabled\)\s*\{\s*return;\s*\}[\s\S]*?pulseWorkspaceToggle\(\);/.test(appSource),
  true,
  "workspace toggle handler ignores blocked expands without a click tooltip or pulse",
);
eq(
  /workspacePanelToggleDisabled=\{workspacePanelExpandDisabled\}/.test(appSource),
  true,
  "top chrome receives the workspace expand disabled state",
);
eq(
  /<Tooltip label=\{workspacePanelTooltipLabel\} showOnFocus=\{false\}>[\s\S]*?disabled=\{workspacePanelExpandDisabled\}[\s\S]*?onClick=\{workspacePanelExpandDisabled \? undefined : toggleWorkspacePanel\}/.test(appSource),
  true,
  "topicbar workspace button shows hover tooltip while disabled",
);
eq(
  (appSource.match(/<Tooltip label=\{workspacePanelTooltipLabel\} showOnFocus=\{false\}>/g) ?? []).length,
  2,
  "topicbar workspace tooltips do not open from click-retained focus",
);
eq(
  /workspacePanelToggleDisabled:\s*boolean;/.test(appChromeSource),
  true,
  "AppChrome accepts the workspace expand disabled state",
);
eq(
  /<Tooltip[\s\S]*?label=\{workspacePanelTooltipLabel\}[\s\S]*?showOnFocus=\{false\}[\s\S]*?app-chrome__panel-toggle-tooltip--right[\s\S]*?disabled=\{workspacePanelToggleDisabled\}[\s\S]*?onClick=\{workspacePanelToggleDisabled \? undefined : onToggleWorkspacePanel\}/.test(appChromeSource),
  true,
  "AppChrome wraps the disabled workspace button in a hover tooltip",
);
eq(
  /showOnFocus = true/.test(tooltipSource) &&
    /showOnFocus\?:\s*boolean;/.test(tooltipSource) &&
    /onFocus:\s*\(\) => \{\s*if \(showOnFocus\) \{\s*show\(0\);\s*return;\s*\}\s*hide\(\);\s*\}/.test(tooltipSource),
  true,
  "Tooltip keeps focus support by default but can be made hover-only",
);
eq(
  /app-chrome__panel-toggle-tooltip--right/.test(styles) &&
    /app-chrome__panel-toggle-tooltip[\s\S]*?width:\s*var\(--chrome-toggle-size\)[\s\S]*?height:\s*var\(--app-chrome-height\)/.test(styles),
  true,
  "top chrome tooltip trigger matches the right workspace button hit area",
);
eq(
  /"rightDock\.expandUnavailableNarrow":\s*"窗口太窄，拉宽后可展开"/.test(zhLocaleSource),
  true,
  "Chinese locale explains why narrow workspace expand is disabled",
);

const shouldAutoCollapseSidebarForViewport = (
  workspaceLayout as typeof workspaceLayout & {
    shouldAutoCollapseSidebarForViewport?: (args: {
      viewportWidth: number;
      sidebarCollapsed: boolean;
      sidebarWidth: number;
      chatMinWidth: number;
      floatingSidebarBreakpoint?: number;
    }) => boolean;
  }
).shouldAutoCollapseSidebarForViewport;
eq(typeof shouldAutoCollapseSidebarForViewport, "function", "sidebar auto-collapse threshold helper is exported");
if (shouldAutoCollapseSidebarForViewport) {
  eq(
    shouldAutoCollapseSidebarForViewport({
      viewportWidth: SIDEBAR_WIDTH + CHAT_MIN_WIDTH - 1,
      sidebarCollapsed: false,
      sidebarWidth: SIDEBAR_WIDTH,
      chatMinWidth: CHAT_MIN_WIDTH,
    }),
    true,
    "expanded sidebar auto-collapses before chat drops below its minimum width",
  );
  eq(
    shouldAutoCollapseSidebarForViewport({
      viewportWidth: SIDEBAR_WIDTH + CHAT_MIN_WIDTH,
      sidebarCollapsed: false,
      sidebarWidth: SIDEBAR_WIDTH,
      chatMinWidth: CHAT_MIN_WIDTH,
    }),
    false,
    "expanded sidebar stays open when the chat minimum still fits",
  );
  eq(
    shouldAutoCollapseSidebarForViewport({
      viewportWidth: CHAT_MIN_WIDTH - 1,
      sidebarCollapsed: true,
      sidebarWidth: SIDEBAR_WIDTH,
      chatMinWidth: CHAT_MIN_WIDTH,
      floatingSidebarBreakpoint: 820,
    }),
    false,
    "already-collapsed sidebars do not trigger another auto-collapse",
  );
  eq(
    shouldAutoCollapseSidebarForViewport({
      viewportWidth: 820,
      sidebarCollapsed: false,
      sidebarWidth: SIDEBAR_WIDTH,
      chatMinWidth: CHAT_MIN_WIDTH,
      floatingSidebarBreakpoint: 820,
    }),
    true,
    "sidebar auto-collapses before the theme turns it into a floating overlay",
  );
  eq(
    shouldAutoCollapseSidebarForViewport({
      viewportWidth: 821,
      sidebarCollapsed: false,
      sidebarWidth: SIDEBAR_WIDTH,
      chatMinWidth: CHAT_MIN_WIDTH,
      floatingSidebarBreakpoint: 820,
    }),
    false,
    "sidebar can stay expanded above the floating overlay breakpoint",
  );
}
eq(
  /const SIDEBAR_FLOAT_BREAKPOINT = 820;/.test(appSource) &&
    /const CREATION_SIDEBAR_FLOAT_BREAKPOINT = 1200;/.test(appSource),
  true,
  "creation layout extends the floating breakpoint without changing other layouts",
);
eq(
  /floatingSidebarBreakpoint:\s*floatingBreakpoint/.test(appSource),
  true,
  "App auto-collapses sidebars before they become floating overlays",
);
eq(
  /viewportWidth < minViewportForSidebar \|\| viewportWidth <= floatingBreakpoint/.test(appSource),
  true,
  "App keeps auto-collapsed sidebars collapsed until the floating overlay breakpoint clears",
);
eq(
  /creationSidebarRestoreCollapsedRef\.current = sidebarCollapsed/.test(appSource) &&
    /setSidebarCollapsed\(restoreCollapsed\)/.test(appSource),
  true,
  "creation compact mode restores the sidebar state that existed before entering",
);
const collapsedWorkspaceHeaderInset = styles.match(
  /\.app--creation \.layout--sidebar-collapsed \.main :is\(([\s\S]*?)\)\s*\{([^}]*)\}/,
);
eq(
  Boolean(
    styles.includes("--sidebar-expand-safe-inset: 52px") &&
      styles.includes("--sidebar-expand-safe-inset: 132px") &&
      collapsedWorkspaceHeaderInset?.[1].includes(".cloud-projects__workspace-header") &&
      collapsedWorkspaceHeaderInset[1].includes(".capability-workspace-header") &&
      collapsedWorkspaceHeaderInset[1].includes(".heartbeat-modal--embedded > .heartbeat-modal__header") &&
      collapsedWorkspaceHeaderInset[1].includes(".superpower-conversation__header") &&
      collapsedWorkspaceHeaderInset[1].includes(".account-center__header") &&
      collapsedWorkspaceHeaderInset[2].includes("padding-left: var(--sidebar-expand-safe-inset)") &&
      /\.layout--sidebar-collapsed \.main \.automation-workspace__tab-bar\s*\{[^}]*margin-left:\s*var\(--sidebar-expand-safe-inset\)/.test(styles),
  ),
  true,
  "collapsed creation workspaces reserve the sidebar expand control's leading safe area",
);
eq(
  /if \(!temporaryCreationOverlay\) saveSidebarCollapsed\(nextCollapsed\)/.test(appSource),
  true,
  "temporary creation sidebar overlays do not overwrite the persisted preference",
);
eq(
  /const handleNewTab = useCallback\(async \(\) => \{[\s\S]*?closeNarrowSidebar\(\)/.test(appSource) &&
    /const handleOpenTopic = useCallback\([\s\S]*?closeNarrowSidebar\(\)/.test(appSource) &&
    /const openSidebarImConnectionSession = useCallback\([\s\S]*?closeNarrowSidebar\(\)/.test(appSource),
  true,
  "creation sidebar overlays close after opening a new chat, topic, or message connection",
);
eq(
  /creationSidebarFloating && !sidebarCollapsed && \([\s\S]*?className="sidebar-compact-backdrop"[\s\S]*?onClick=\{closeNarrowSidebar\}/.test(appSource),
  true,
  "expanded compact creation sidebars close when their backdrop is clicked",
);
eq(
  /\.app--creation\s+\.sidebar-compact-backdrop,[\s\S]*?position:\s*absolute;[\s\S]*?inset:\s*0\s+0\s+0\s+var\(--sidebar-expanded-width\);[\s\S]*?z-index:\s*var\(--z-local-backdrop\);[\s\S]*?background:\s*rgba\(0,\s*0,\s*0,\s*0\.16\);[\s\S]*?animation:\s*backdrop-in\s+300ms\s+ease-out\s+both;/.test(styles),
  true,
  "compact creation sidebar backdrop fades over and dims only the underlying workspace",
);
eq(
  /@media\s*\(max-width:\s*1200px\)[\s\S]*?\.app--creation\s+\.sidebar,[\s\S]*?position:\s*absolute/.test(styles),
  true,
  "creation sidebars float throughout the minimum-window width",
);
eq(
  /@media\s*\(max-width:\s*1200px\)[\s\S]*?\.app--creation\s+\.layout:not\(\.layout--sidebar-collapsed\):not\(\.layout--workspace-open\):not\(\.layout--workspace-maximized\)[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)/.test(styles),
  true,
  "creation floating sidebars do not consume a layout column",
);
eq(
  /@media\s*\(max-width:\s*1200px\)[\s\S]*?\.app--creation\s+\.chat-pane,[\s\S]*?grid-column:\s*1\s*\/\s*-1/.test(styles),
  true,
  "creation chat spans the compact single-column layout",
);
eq(
  /@media\s*\(max-width:\s*1200px\)[\s\S]*?\.app--creation\s+\.layout--workspace-open,[\s\S]*?grid-template-columns:[\s\S]*?minmax\(var\(--chat-min-width\),\s*1fr\)[\s\S]*?var\(--workspace-resizer-width\)[\s\S]*?minmax\(0,\s*var\(--workspace-width\)\)/.test(styles),
  true,
  "creation workspace keeps chat, resizer, and dock columns at the minimum window width",
);
eq(
  /\.app--creation\s+\.layout--workspace-open\s+\.chat-pane,[\s\S]*?grid-column:\s*1;[\s\S]*?\.app--creation\s+\.layout--workspace-open\s+\.workspace-panel-resizer,[\s\S]*?grid-column:\s*2;[\s\S]*?\.app--creation\s+\.layout--workspace-open\s+\.workbench-dock,[\s\S]*?grid-column:\s*3;/.test(styles),
  true,
  "creation workspace elements stay in the visible compact grid columns",
);
eq(
  /\.app--creation\s+\.layout--workspace-maximized\s+\.workbench-dock,[\s\S]*?grid-column:\s*1;/.test(styles),
  true,
  "maximized creation workspace occupies the compact main column",
);
eq(
  /const workspaceLayoutSidebarCollapsed = sidebarCollapsed \|\| creationSidebarFloating;/.test(appSource) &&
    /sidebarCollapsed:\s*workspaceLayoutSidebarCollapsed/.test(appSource),
  true,
  "floating creation sidebars do not reduce the right workspace width budget",
);
eq(
  /@media\s*\(max-width:\s*820px\)[\s\S]*?:root\[data-theme-style\]\s+\.sidebar\s*\{[\s\S]*?position:\s*absolute/.test(styles),
  true,
  "theme-style narrow sidebars float at the breakpoint guarded by App",
);
eq(
  /@media\s*\(max-width:\s*820px\)[\s\S]*?\.layout:not\(\.layout--sidebar-collapsed\):not\(\.layout--workspace-open\):not\(\.layout--workspace-maximized\)[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)/.test(styles),
  true,
  "narrow layout overrides the higher-specificity expanded grid rule",
);
eq(
  /@media\s*\(max-width:\s*820px\)[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\)/.test(styles),
  true,
  "narrow layout allows the chat track to shrink to the viewport width",
);
eq(
  /@media\s*\(max-width:\s*820px\)[\s\S]*?\.chat-pane\s*\{[\s\S]*?grid-column:\s*1\s*\/\s*-1/.test(styles),
  true,
  "narrow chat pane spans the full single-column layout",
);
eq(
  /--chat-min-width:\s*400px;/.test(styles),
  true,
  "CSS chat minimum matches the session width guarded by App",
);
eq(
  /grid-template-columns:\s*var\(--sidebar-width\)\s+minmax\(var\(--chat-min-width\),\s*1fr\);/.test(styles),
  true,
  "expanded layout prevents the chat pane from being compressed to zero",
);
eq(
  /grid-template-columns:\s*0px\s+minmax\(var\(--chat-min-width\),\s*1fr\);/.test(styles),
  true,
  "collapsed layout keeps the chat pane at the guarded minimum width",
);
eq(
  /minimumWindowWidth\s+=\s+1200/.test(desktopMainSource) &&
    /MinWidth:\s+minimumWindowWidth,/.test(desktopMainSource),
  true,
  "desktop shell minimum width matches the compact creation layout boundary",
);
eq(
  /<div className="workspace-files__tabs" role="tablist"[\s\S]*?<button[\s\S]*?role="tab"[\s\S]*?aria-selected=\{viewMode === "files"\}[\s\S]*?<button[\s\S]*?role="tab"[\s\S]*?aria-selected=\{viewMode === "changed"\}/.test(workspacePanelSource),
  true,
  "workspace view tabs expose semantic tab roles and selected state",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
