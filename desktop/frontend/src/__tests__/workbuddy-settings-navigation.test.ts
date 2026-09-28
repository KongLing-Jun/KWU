import { loadStylesSource } from "./helpers/styles";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

const here = dirname(fileURLToPath(import.meta.url));
const source = (path: string) => readFileSync(resolve(here, path), "utf8");
const settings = source("../features/settings/ui/SettingsPanel.tsx");
const app = source("../App.tsx");
const styles = loadStylesSource();

function styleRule(selector: string) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = styles.match(new RegExp(`^${escaped}\\s*\\{([^}]*)\\}`, "m"));
  ok(match, `styles define ${selector}`);
  return match![1];
}

function mediaRules(maxWidth: number) {
  const marker = `@media (max-width: ${maxWidth}px)`;
  const rules: string[] = [];
  let searchFrom = 0;
  while (true) {
    const start = styles.indexOf(marker, searchFrom);
    if (start < 0) break;
    const open = styles.indexOf("{", start + marker.length);
    ok(open >= 0, `${marker} opens a rule block`);
    let depth = 0;
    let end = open;
    for (; end < styles.length; end += 1) {
      if (styles[end] === "{") depth += 1;
      if (styles[end] === "}") depth -= 1;
      if (depth === 0) break;
    }
    ok(depth === 0, `${marker} closes its rule block`);
    rules.push(styles.slice(open + 1, end));
    searchFrom = end + 1;
  }
  ok(rules.length > 0, `styles define ${marker}`);
  return rules.join("\n");
}

for (const group of ["workspace", "assistant", "system", "support"]) {
  ok(settings.includes(`label: "settings.group.${group}"`), `settings navigation defines the ${group} group`);
}

const expectedTabs = [
  "general", "appearance", "shortcuts", "models", "mcp",
  "memory", "permissions", "sandbox", "updates", "feedback",
];
for (const tab of expectedTabs) {
  const occurrences = settings.match(new RegExp(`\{ id: "${tab}", Icon:`, "g"))?.length ?? 0;
  ok(occurrences === 1, `${tab} remains available in exactly one settings group`);
}
ok(!settings.includes('{ id: "skills", Icon:'), "skills stays out of the settings navigation");
ok(!settings.includes('{ id: "bots", Icon:'), "message channels stay out of settings navigation");
ok(settings.includes('initialTab === "bots" ? "mcp"'), "legacy bot settings targets fall back to connector settings");

ok(settings.includes('aria-current={tab === id ? "page" : undefined}'), "active settings destination exposes aria-current");
ok(settings.includes('className="settings-center__navitem-icon"'), "settings destinations have recognizable icons");
ok(app.includes('setSettingsTarget("appearance")'), "personalization footer opens the appearance destination");
ok(app.includes('setSettingsTarget("feedback")'), "account menu opens the feedback destination");
ok(
  /setSettingsTarget\("appearance"\)[\s\S]*?setSettingsTarget\("feedback"\)[\s\S]*?sidebarAccountHasSession/.test(app),
  "feedback remains after appearance and before authenticated logout actions",
);
ok(
  /\.app--creation \.settings-center__navgroup-label,[\s\S]*?display: none;/.test(styles),
  "settings navigation keeps destinations in one flat visual list",
);
ok(styles.includes("width: min(1040px, calc(100vw - 64px));"), "creation settings center matches the renderer modal footprint");
ok(styles.includes("height: min(720px, calc(100vh - 64px));"), "creation settings center matches the renderer modal height");
ok(styles.includes("grid-template-columns: 236px minmax(0, 1fr);"), "settings center matches the renderer navigation width");
ok(styles.includes("background: var(--bg-elev-2);"), "settings rows use the renderer-style flat surface");
ok(styles.includes("transform: translateY(1px) scale(0.985);"), "settings buttons retain a renderer-style pressed state");
ok(styles.includes("--settings-seg-thumb-shadow"), "settings segmented controls expose a raised moving thumb shadow");
ok(styles.includes("transform 0.25s cubic-bezier(0.4, 0, 0.2, 1)"), "settings segmented thumb uses the renderer sliding motion");
ok(
  /\.settings-center \.set-seg:has\(> \.set-seg__btn\)[\s\S]*?min-height: 44px;[\s\S]*?border-radius: 14px;[\s\S]*?background: var\(--bg-elev-2\);/.test(styles),
  "all settings segmented controls share the theme selector surface and height",
);
ok(
  /\.settings-center \.set-seg__btn--on[\s\S]*?color: var\(--fg\);[\s\S]*?font-weight: 600;/.test(styles),
  "all settings selected choices keep the reference neutral foreground",
);
ok(
  /\.app--creation \.settings-page--appearance \.set-seg,\s*:root\[data-theme-style\] \.app--creation \.settings-page--appearance \.set-seg\s*\{[^}]*width:\s*420px;/.test(styles),
  "personalization groups retain one readable aligned width",
);
ok(
  /\.app--creation \.settings-page--appearance \.set-seg__btn,[\s\S]*?padding:\s*6px;/.test(styles),
  "personalization segment padding leaves enough room for complete option labels",
);
ok(
  /\.app--creation \.settings-page--appearance \.appearance-panel__control,[\s\S]*?flex-shrink:\s*0;/.test(styles),
  "personalization controls keep their readable width in the desktop row",
);
ok(!/\.settings-page--models \.set-seg__btn[\s\S]*?min-height: 28px;/.test(styles), "model controls no longer restore the compact mismatched treatment");
ok(styles.includes("--settings-nav-active-bg: var(--sidebar-hover);"), "settings navigation selected state follows the active visual style");
ok(
  styles.includes("--settings-nav-surface: color-mix(in srgb, var(--sidebar-bg) 96%, var(--fg) 4%);"),
  "settings navigation surface follows the active visual style",
);
ok(
  /\.app--creation \.settings-center,[\s\S]*?--accent: var\(--fg\);[\s\S]*?--control-primary-bg: var\(--fg\);/.test(styles),
  "settings interactions use the neutral foreground instead of the workspace theme accent",
);
ok(
  styles.includes('.settings-center__navitem[aria-current="page"]'),
  "selected background remains attached to the current settings destination after click",
);
ok(
  settings.includes('data-selected={tab === id ? "true" : undefined}'),
  "settings navigation exposes a persistent selected-state marker",
);
ok(
  settings.includes('backgroundColor: "rgba(0, 0, 0, 0.12)"'),
  "current settings destination owns a persistent inline fill independent of theme cascade order",
);
ok(
  styles.includes('.settings-center__navitem[data-selected="true"]'),
  "persistent selected-state styling wins over transient interaction styles",
);
ok(
  /\.app--creation \.settings-page__header,[\s\S]*?display: block;/.test(styles),
  "creation settings pages retain their visible title hierarchy",
);
const settingsPageHeaderRule = styleRule(".settings-page__header");
ok(/\bposition:\s*sticky;/.test(settingsPageHeaderRule), "settings page header remains sticky while content scrolls");
ok(
  /\btop:\s*0;/.test(settingsPageHeaderRule),
  "settings page header anchors to the scroll container padding edge",
);
ok(/\bbackground:\s*var\(--bg-elev\);/.test(settingsPageHeaderRule), "sticky settings page header remains opaque");
ok(
  /\bz-index:\s*var\(--z-local-raised\);/.test(settingsPageHeaderRule),
  "sticky settings page header uses the local raised layer",
);
ok(
  !/^  \.settings-page__header\s*\{[^}]*?\btop:/m.test(styles),
  "responsive settings styles do not add the content padding to the sticky offset",
);
const settingsContentRule = styleRule(".settings-center__content");
ok(
  /--settings-content-padding-top:\s*clamp\(16px,\s*4vh,\s*34px\);/.test(settingsContentRule),
  "settings content exposes its default top padding to the sticky mask",
);
const settingsContentMaskRule = styleRule(".settings-center__content::before");
for (const declaration of [
  "position: sticky;",
  "top: 0;",
  "z-index: var(--z-local-raised);",
  "height: var(--settings-content-padding-top);",
  "margin-bottom: calc(-1 * var(--settings-content-padding-top));",
  "transform: translateY(calc(-1 * var(--settings-content-padding-top)));",
  "background: var(--bg-elev);",
  "pointer-events: auto;",
]) {
  ok(settingsContentMaskRule.includes(declaration), `settings content top mask keeps ${declaration}`);
}
const settings900Rules = mediaRules(900);
ok(
  settings900Rules.includes("--settings-content-padding-top: 18px;")
    && settings900Rules.includes("--settings-content-padding-top: 24px;"),
  "900px settings masks track standard and creation content padding",
);
const settings760Rules = mediaRules(760);
ok(
  settings760Rules.includes("--settings-content-padding-top: 14px;")
    && settings760Rules.includes("--settings-content-padding-top: 18px;"),
  "760px settings masks track standard and creation content padding",
);
ok(
  mediaRules(980).includes("--settings-content-padding-top: 20px;"),
  "980px settings mask tracks the final standard content padding override",
);
ok(
  /@media \(max-width: 900px\)[\s\S]*?\.settings-center__navgroup-label \{[\s\S]*?display: none;/.test(styles),
  "compact settings navigation hides group headings while retaining all destinations",
);

for (const locale of ["zh"]) {
  const localeSource = source(`../locales/${locale}.ts`);
  for (const group of ["workspace", "assistant", "system", "support"]) {
    ok(localeSource.includes(`"settings.group.${group}"`), `${locale} localizes the ${group} settings group`);
  }
  ok(localeSource.includes('"personalization.title"'), `${locale} localizes the personalization footer`);
  ok(localeSource.includes('"personalization.settingsTitle"'), `${locale} localizes the personalized settings shell`);
}

console.log("WorkBuddy settings navigation contract passed");
