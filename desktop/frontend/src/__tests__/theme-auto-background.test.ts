// Run: tsx src/__tests__/theme-auto-background.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { WORKSPACE_REF_DRAG_TYPE } from "../lib/workspaceDrag";

const testDir = dirname(fileURLToPath(import.meta.url));
const themeSource = readFileSync(resolve(testDir, "../lib/theme.ts"), "utf8");
const startupSplashSource = readFileSync(resolve(testDir, "../components/StartupSplash.tsx"), "utf8");
const i18nSource = readFileSync(resolve(testDir, "../lib/i18n.tsx"), "utf8");
const displayModeSource = readFileSync(resolve(testDir, "../lib/displayMode.ts"), "utf8");
const fontFamilySource = readFileSync(resolve(testDir, "../lib/fontFamily.ts"), "utf8");
const keyboardShortcutsSource = readFileSync(resolve(testDir, "../lib/keyboardShortcuts.ts"), "utf8");
const layoutPreferencesSource = readFileSync(resolve(testDir, "../lib/layoutPreferences.ts"), "utf8");
const layoutStoreSource = readFileSync(resolve(testDir, "../store/layout.ts"), "utf8");
const memoryPanelSource = readFileSync(resolve(testDir, "../components/MemoryPanel.tsx"), "utf8");
const crashSource = readFileSync(resolve(testDir, "../lib/crash.ts"), "utf8");
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");

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

console.log("\ntheme auto native background contract");

ok(themeSource.includes('const DEFAULT_THEME: Theme = "light";'), "unset theme defaults to light");
ok(themeSource.includes('const THEME_KEY = "kwu-theme";'), "theme storage key uses kwu namespace");
ok(themeSource.includes('const STYLE_KEY = "kwu-theme-style";'), "theme style storage key uses kwu namespace");
ok(
  themeSource.includes("localStorage.removeItem(THEME_KEY)") && themeSource.includes("localStorage.removeItem(STYLE_KEY)"),
  "legacy theme migration clears both old storage keys",
);
ok(
  appSource.includes('app.MigrateDesktopPreferences(legacyLanguage, legacyTheme.theme, "")'),
  "legacy WebView migration does not send a visual style",
);
ok(
  appSource.includes("const style = defaultStyleForTheme(next)") &&
    appSource.includes("const style = defaultStyleForTheme(cur)") &&
    !appSource.includes("applyTheme(cur, arg)"),
  "legacy theme-style commands keep the desktop on its default visual style",
);
ok(startupSplashSource.includes('const SPLASH_FLAG = "kwu.splash.shown";'), "startup splash session key uses kwu namespace");
ok(WORKSPACE_REF_DRAG_TYPE === "application/x-kwu-workspace-ref", "workspace drag MIME type uses kwu namespace");
ok(i18nSource.includes('const STORAGE_KEY = "kwu-lang";'), "language storage key uses kwu namespace");
ok(displayModeSource.includes('const DISPLAY_MODE_KEY = "kwu-display-mode";'), "display mode storage key uses kwu namespace");
ok(displayModeSource.includes('const DISPLAY_MODE_EVENT = "kwu:display-mode";'), "display mode event uses kwu namespace");
ok(fontFamilySource.includes('const FONT_FAMILY_KEY = "kwu-font-family";'), "font family storage key uses kwu namespace");
ok(fontFamilySource.includes('const CUSTOM_FONT_KEY = "kwu-font-family-custom";'), "custom font storage key uses kwu namespace");
ok(fontFamilySource.includes('const MONO_FONT_FAMILY_KEY = "kwu-mono-font-family";'), "mono font storage key uses kwu namespace");
ok(fontFamilySource.includes('const CUSTOM_MONO_FONT_KEY = "kwu-mono-font-family-custom";'), "custom mono font storage key uses kwu namespace");
ok(keyboardShortcutsSource.includes('const SHORTCUTS_STORAGE_KEY = "kwu.customShortcuts";'), "custom shortcut storage key uses kwu namespace");
ok(keyboardShortcutsSource.includes('const SHORTCUTS_CHANGED_EVENT = "kwu:shortcuts-changed";'), "shortcut change event uses kwu namespace");
ok(layoutPreferencesSource.includes('const STORAGE_KEY = "kwu.layoutPreferences.v1";'), "layout preferences storage key uses kwu namespace");
ok(!layoutPreferencesSource.includes("LEGACY_SIZE_KEYS"), "layout preferences no longer keep legacy size key aliases");
ok(layoutStoreSource.includes('const SIDEBAR_COLLAPSED_KEY = "kwu.sidebar.collapsed";'), "sidebar collapsed storage key uses kwu namespace");
ok(memoryPanelSource.includes('const AUTO_MEMORY_SUGGESTIONS_KEY = "kwu.memory.autoSuggestions";'), "auto memory suggestions storage key uses kwu namespace");
ok(crashSource.includes('const PERF_REPORTED_STORAGE_KEY = "kwu:perf-reported";'), "performance report storage key uses kwu namespace");
ok(
  themeSource.includes('const AUTO_THEME_MEDIA_QUERY = "(prefers-color-scheme: light)";'),
  "auto theme uses one shared light color-scheme media query",
);
ok(
  themeSource.includes("window.matchMedia?.(AUTO_THEME_MEDIA_QUERY).matches"),
  "resolved auto theme reads the shared color-scheme query",
);
ok(
  themeSource.includes("syncAutoThemeBackgroundListener(theme);"),
  "applyTheme updates the native auto-theme listener",
);
ok(
  themeSource.includes('autoThemeMediaQuery.addEventListener("change", syncAutoThemeBackground)'),
  "auto mode listens for color-scheme changes",
);
ok(
  themeSource.includes('autoThemeMediaQuery.removeEventListener("change", syncAutoThemeBackground)'),
  "explicit modes remove the color-scheme listener",
);
ok(
  themeSource.includes('if (theme !== "auto")') && themeSource.includes("clearAutoThemeBackgroundListener();"),
  "non-auto themes clear the auto listener",
);
ok(
  themeSource.includes('if (currentTheme === "auto"') && themeSource.includes('syncNativeWindowBackground("auto")'),
  "color-scheme changes only sync native background while auto is active",
);
ok(
  themeSource.includes("if (autoThemeMediaQuery || typeof window"),
  "reapplying auto does not register duplicate listeners",
);
ok(
  themeSource.includes('getComputedStyle(document.documentElement).getPropertyValue("--bg")') &&
    themeSource.includes("parseHexColor"),
  "native window background resolves from the active visual-style background token",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
