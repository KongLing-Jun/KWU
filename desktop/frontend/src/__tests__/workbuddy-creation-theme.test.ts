import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/workbuddy-creation-theme.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defaultStyleForTheme, normalizeThemeStyleForTheme } from "../lib/theme";

const testDir = dirname(fileURLToPath(import.meta.url));
const styles = loadStylesSource();
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");
const composerSource = readFileSync(resolve(testDir, "../features/composer/ui/Composer.tsx"), "utf8");
const anchoredPopoverSource = readFileSync(resolve(testDir, "../components/AnchoredPopover.tsx"), "utf8");

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nWorkbuddy creation theme contract");

ok(
  defaultStyleForTheme("auto") === "carbon" && normalizeThemeStyleForTheme(undefined) === "carbon",
  "new desktop users resolve to the reusable Carbon direction",
);
ok(
  normalizeThemeStyleForTheme("graphite") === "graphite",
  "an explicit existing theme choice remains unchanged",
);
ok(
  /:root\[data-theme-style\] \.app--creation \{[\s\S]*?--creation-canvas-bg: var\(--chat-bg\);/.test(styles) &&
    !/:root\[data-theme="light"\]\[data-theme-style\] \.app--creation \{/.test(styles),
  "non-Carbon creation themes inherit their complete visual-style token system",
);
ok(
  /:root\[data-theme="light"\]\[data-theme-style="carbon"\] \.app--creation \{[\s\S]*?--creation-canvas-bg: #ffffff;[\s\S]*?--sidebar-bg: #f3f4f4;[\s\S]*?--border: #e4e5e5;[\s\S]*?--accent: #18181b;[\s\S]*?--accent-soft: rgba\(24, 24, 27, 0\.08\);[\s\S]*?--control-disabled-bg: var\(--fg-faint\);[\s\S]*?--control-disabled-fg: var\(--bg-elev\);[\s\S]*?--control-disabled-border: transparent;/.test(styles),
  "light Carbon owns a black interaction accent instead of the legacy teal",
);
ok(
  /@media \(prefers-color-scheme: light\) \{[\s\S]*?:root\[data-theme-style="carbon"\]:not\(\[data-theme\]\) \.app--creation \{[\s\S]*?--creation-canvas-bg: #ffffff;[\s\S]*?--accent: #18181b;[\s\S]*?--control-disabled-bg: var\(--fg-faint\);[\s\S]*?--control-disabled-fg: var\(--bg-elev\);[\s\S]*?--control-disabled-border: transparent;/.test(styles),
  "automatic light Carbon resolves to the same Workbuddy token system",
);
ok(
  styles.includes("--shell-accent: var(--fg);") &&
    /\.composer-mode-chip--token[\s\S]*?border-color: color-mix\(in srgb, var\(--fg\) 24%, var\(--border\)\);/.test(styles) &&
    /composer-modebar--approval\[data-mode="auto"\][\s\S]*?border-color: color-mix\(in srgb, var\(--fg\) 22%, transparent\);/.test(styles),
  "shell, token, and automatic-mode controls use the current black-or-white foreground",
);
ok(
  /\.statusbar__balance,[\s\S]*?\.statusbar__cost \{[\s\S]*?color: var\(--fg-dim\);/.test(styles) &&
    /\.statusbar__cost b,[\s\S]*?\.statusbar__balance b \{[\s\S]*?color: var\(--fg\);/.test(styles),
  "cost and balance metrics no longer present as green success states",
);
ok(
  /\.app--creation \.sidebar,[\s\S]*?border-right: 1px solid var\(--border-soft\);[\s\S]*?background: var\(--sidebar-bg\);/.test(styles),
  "creation shell keeps a stable low-contrast sidebar surface",
);
ok(
  /\.app--creation \.sidebar-feature-zone__item--new-chat,[\s\S]*?color:\s*var\(--fg\);[\s\S]*?font-size:\s*13px;[\s\S]*?font-weight:\s*400;/.test(styles) &&
    /\.app--creation \.sidebar-feature-zone__item,[\s\S]*?color:\s*var\(--fg\);[\s\S]*?font-size:\s*13px;[\s\S]*?font-weight:\s*400;/.test(styles) &&
    /\.app--creation \.sidebar-feature-zone__item svg,[\s\S]*?color:\s*color-mix\(in srgb, var\(--accent\) 72%, var\(--fg-faint\)\);/.test(styles) &&
    /\.app--creation \.sidebar-feature-zone__item--active,[\s\S]*?font-weight:\s*600;/.test(styles) &&
    /\.app--creation \.sidebar-feature-zone__item-copy small,[\s\S]*?color:\s*var\(--fg-dim\);/.test(styles) &&
    /sidebar-feature-zone__item--new-chat\$\{workspaceView === "chat" && !sessionHasContent && blankSessionPresentation \? " sidebar-feature-zone__item--active" : ""\}/.test(appSource),
  "creation navigation follows WorkBuddy's regular and selected weights while preserving themed icons",
);
ok(
  /\.app--creation \.sidebar-feature-zone__item,[\s\S]*?box-shadow 150ms ease,[\s\S]*?transform 90ms cubic-bezier\(0\.2, 0\.8, 0\.2, 1\);/.test(styles) &&
    /\.app--creation \.sidebar-feature-zone__item:hover:not\(:disabled\),[\s\S]*?0 4px 10px -7px[\s\S]*?inset 0 0 0 1px/.test(styles) &&
    /\.app--creation \.sidebar-feature-zone__item:active:not\(:disabled\),[\s\S]*?inset 0 1px 3px[\s\S]*?transform:\s*translateY\(1px\) scale\(0\.985\);[\s\S]*?transition-duration:\s*60ms;/.test(styles) &&
    /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.app--creation \.sidebar-feature-zone__item:active:not\(:disabled\),[\s\S]*?transform:\s*none;/.test(styles),
  "creation navigation provides reference-matched hover shadow and press feedback",
);
ok(
  /\.app--creation \.project-tree__task-section-head,[\s\S]*?color:\s*var\(--fg-dim\);/.test(styles) &&
    /\.app--creation \.project-tree__section-empty,[\s\S]*?color:\s*var\(--fg-dim\);/.test(styles) &&
    /\.app--creation \.project-tree__topic-label,[\s\S]*?color:\s*var\(--fg\);[\s\S]*?font-size:\s*13px;[\s\S]*?font-weight:\s*400;/.test(styles) &&
    /\.app--creation \.project-tree__topic--active \.project-tree__topic-label,[\s\S]*?font-weight:\s*600;/.test(styles) &&
    /\.app--creation \.project-tree__topic-side,[\s\S]*?color:\s*color-mix\(in srgb, var\(--fg-faint\) 76%, transparent\);/.test(styles),
  "creation task titles use WorkBuddy's regular and selected weight hierarchy",
);
ok(
  /\.app--creation \.sidebar-feature-zone__item,[\s\S]*?height:\s*36px;[\s\S]*?font-size:\s*13px;[\s\S]*?font-weight:\s*430;/.test(styles) &&
    /\.app--creation \.project-tree__topic-label,[\s\S]*?font-size:\s*13px;[\s\S]*?font-weight:\s*430;[\s\S]*?line-height:\s*20px;/.test(styles),
  "creation sidebar uses the current compact type scale and row rhythm",
);
ok(
  /\.app--creation \.project-tree__topic:hover,[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 3\.5%, transparent\);/.test(styles) &&
    /\.app--creation \.project-tree__topic--active,[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 5\.5%, transparent\);/.test(styles) &&
    /\.app--creation \.project-tree__topic--active \.project-tree__topic-label,[\s\S]*?font-weight:\s*520;/.test(styles),
  "creation conversations use restrained neutral hover and active emphasis",
);
ok(
  /\.app--creation \.project-tree__topic-action,[\s\S]*?border:\s*1px solid transparent;[\s\S]*?background:\s*transparent;/.test(styles) &&
    /\.app--creation \.project-tree__topic-action:hover,[\s\S]*?border-color:\s*transparent;[\s\S]*?background:\s*var\(--sidebar-hover\);/.test(styles) &&
    /\.app--creation \.project-tree__topic-action--pinned,[\s\S]*?color:\s*var\(--accent\);/.test(styles) &&
    /\.app--creation \.project-tree__topic-action--archive:hover,[\s\S]*?color:\s*var\(--err\);/.test(styles),
  "creation topic actions stay transparent until directly hovered",
);
ok(
  /\.app--creation \.chat-pane--creation-empty \.topicbar__title-row h1,[\s\S]*?color:\s*var\(--fg-dim\);/.test(styles) &&
    /\.startup-splash__sub\s*\{[\s\S]*?color:\s*var\(--fg-dim\);/.test(styles) &&
    /\.app--creation \.welcome-creation__mode--active\s*\{[\s\S]*?color:\s*var\(--bg-elev\);/.test(styles),
  "creation titles, startup copy, and active mode text meet readable contrast",
);
ok(
  /\.app--creation \.welcome-creation__modes\s*\{[\s\S]*?height:\s*36px;[\s\S]*?border:\s*0;[\s\S]*?border-radius:\s*100px;[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 5%, transparent\);/.test(styles) &&
    /\.app--creation \.welcome-creation__mode\s*\{[\s\S]*?height:\s*32px;[\s\S]*?border-radius:\s*100px;[\s\S]*?background:\s*transparent;[\s\S]*?transition:\s*background-color 0\.14s ease, color 0\.14s ease;/.test(styles) &&
    /\.app--creation \.welcome-creation__mode:hover:not\(\.welcome-creation__mode--active\),[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 7%, transparent\);/.test(styles) &&
    /\.app--creation \.welcome-creation__mode--active\s*\{[\s\S]*?border-color:\s*transparent;[\s\S]*?background:\s*color-mix\(in srgb, var\(--fg\) 94%, #111\);[\s\S]*?color:\s*var\(--bg-elev\);[\s\S]*?box-shadow:\s*none;/.test(styles),
  "creation mode selector matches the compact monochrome reference control",
);
ok(
  /\.app--creation \.welcome-creation__mode:active:not\(:disabled\)\s*\{[\s\S]*?transform:\s*translateY\(1px\) scale\(0\.98\);[\s\S]*?box-shadow:\s*inset 0 1px 3px/.test(styles) &&
    /\.app--creation \.welcome-creation__scenario\s*\{[\s\S]*?box-shadow:\s*0 1px 2px/.test(styles) &&
    /\.app--creation \.welcome-creation__scenario:active:not\(:disabled\)\s*\{[\s\S]*?transform:\s*translateY\(1px\) scale\(0\.985\);[\s\S]*?box-shadow:\s*inset 0 1px 3px/.test(styles) &&
    /@media \(prefers-reduced-motion: reduce\)\s*\{[\s\S]*?\.app--creation \.welcome-creation__mode:active:not\(:disabled\),[\s\S]*?transform:\s*none;/.test(styles),
  "creation controls provide restrained press feedback",
);
ok(
  anchoredPopoverSource.includes("readThemeVariables(anchorRef.current)") &&
    anchoredPopoverSource.includes("...themeVariables") &&
    !styles.includes(':root[data-theme-style="graphite"] .modelsw__menu--model'),
  "portaled controls inherit every visual style instead of carrying a Graphite-only patch",
);
ok(
  /\.app--creation \.composer-card,[\s\S]*?border: 1px solid var\(--border\);[\s\S]*?0 10px 28px color-mix\(in srgb, #000 7%, transparent\);/.test(styles),
  "composer uses a weak border and restrained elevation",
);
ok(
  /sidebarCreation && workspaceView === "chat" && !sessionHasContent && blankSessionPresentation \? " chat-pane--creation-empty"/.test(appSource) &&
    /\.app--creation \.chat-pane--creation-empty \.main,[\s\S]*?flex: 0 0 auto;[\s\S]*?margin-top: 0;/.test(styles) &&
    /\.app--creation \.chat-pane--creation-empty \.transcript--empty,[\s\S]*?container-type: inline-size;[\s\S]*?padding: 14px 32px 0;/.test(styles) &&
    /\.app--creation \.chat-pane--creation-empty \.footer,[\s\S]*?margin-bottom: 0;/.test(styles),
  "blank-compatible empty chat follows the reference's top-led composition",
);
ok(
  /\.app--creation \.chat-pane--creation-empty \.agent-workspace,[\s\S]*?\.app--creation \.chat-pane--creation-empty \.agent-workspace__body,[\s\S]*?min-height: 0;/.test(styles),
  "an expert team selected in an empty chat cannot push the composer below the viewport",
);
ok(
  /\.app--creation \.welcome-creation__scenario-list \{[\s\S]*?display: flex;[\s\S]*?flex-wrap: wrap;/.test(styles) &&
    /@container \(max-width: 520px\) \{[\s\S]*?\.app--creation \.welcome-creation__scenario-list \{[\s\S]*?display: grid;[\s\S]*?grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/.test(styles),
  "business scenarios form a compact row and adapt to two columns on narrow layouts",
);
ok(
  /\.app--creation \.welcome-creation__scenario\s*\{[^}]*height:\s*32px;[^}]*border-radius:\s*100px;[^}]*\}/.test(styles),
  "primary, more, and expanded business scenarios share the same compact pill outline",
);
ok(
  /\.app--creation \.chat-pane--creation-empty \.composer-card,[\s\S]*?min-height: 178px;/.test(styles) &&
    /\.app--creation \.chat-pane--creation-empty \.composer,[\s\S]*?min-height: 128px;/.test(styles) &&
    /\.app--creation \.chat-pane--creation-empty \.composer-meta,[\s\S]*?border-top: 1px solid var\(--border-soft\);/.test(styles),
  "empty chat uses the taller WorkBuddy composer with a distinct controls row",
);
ok(
  appSource.includes("onSelectScenario={handleCreationScenario}") &&
    appSource.includes('setComposerInsertRequest({ id: Date.now(), text, mode: "replace" })') &&
    !appSource.includes("welcome.creation.skillInserted"),
  "business scenarios fill the existing composer draft without showing a redundant success toast",
);
ok(
  appSource.includes("onOpenSkills={openSkillsWorkspace}") &&
    appSource.includes("onOpenConnectors={openConnectorsWorkspace}") &&
    composerSource.includes('hubSection === "skills"') &&
    composerSource.includes('hubSection === "connectors"'),
  "creation composer exposes real skills and connectors through the unified add menu",
);
const composerBodyStart = composerSource.indexOf('className={`composer${dragOver');
const composerMetaStart = composerSource.indexOf("<div className={composerMetaClass}", composerBodyStart);
const composerBodySource = composerSource.slice(composerBodyStart, composerMetaStart);
ok(
  !composerBodySource.includes("composer__btn--send") &&
    composerSource.indexOf('className="composer-meta__send"', composerMetaStart) > composerMetaStart &&
    styles.includes(".composer-meta__send"),
  "send action is pinned to the right edge of the composer controls row",
);
ok(
  /:root\[data-theme-style\] \.composer__btn--send:disabled \{[\s\S]*?background: var\(--control-disabled-bg\);[\s\S]*?color: var\(--control-disabled-fg\);/.test(styles) &&
    /\.app--creation \.composer__btn--send:disabled,[\s\S]*?border-color: var\(--control-disabled-border, color-mix\(in srgb, var\(--control-disabled-fg\) 24%, transparent\)\);[\s\S]*?background: var\(--control-disabled-bg\);[\s\S]*?color: var\(--control-disabled-fg\);/.test(styles),
  "empty or blocked composers keep the send action visually distinct from the enabled primary action",
);
ok(
  composerSource.includes("composer-action-trigger--add") &&
    composerSource.includes('t("composer.addMenuTitle")') &&
    styles.includes(".composer-action-trigger--add"),
  "creation composer presents one unified add entry while keeping the real feature menu",
);
ok(
  /@media \(prefers-reduced-motion: reduce\)/.test(styles),
  "existing reduced-motion protection remains present",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
