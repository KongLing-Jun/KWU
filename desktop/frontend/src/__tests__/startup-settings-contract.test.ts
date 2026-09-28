// Run: tsx src/__tests__/startup-settings-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { zh, type DictKey } from "../locales/zh";

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

const here = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(here, "../App.tsx"), "utf8");
const desktopPreferencesSource = readFileSync(resolve(here, "../app/useDesktopPreferences.ts"), "utf8");
const bridgeSource = readFileSync(resolve(here, "../lib/bridge.ts"), "utf8");
const settingsSource = readFileSync(resolve(here, "../features/settings/ui/SettingsPanel.tsx"), "utf8");
const generalSettingsSource = readFileSync(resolve(here, "../features/settings/ui/GeneralSettingsSection.tsx"), "utf8");
const onboardingSource = readFileSync(resolve(here, "../components/OnboardingOverlay.tsx"), "utf8");
const syncDesktopPreferencesSource =
  appSource.match(/const\s+syncDesktopPreferences\s*=\s*async\s*\(\)\s*=>\s*\{[\s\S]*?\n\s{4}\};/)?.[0] ?? "";
const credentialCopyKeys: DictKey[] = [
  "settings.setKey",
  "settings.botInstallManualQQ",
  "settings.providerKeyPlaceholder",
  "settings.updateKey",
  "onboarding.tagline",
  "onboarding.privacy",
];
const genericSettingsCopyKeys: DictKey[] = [
  "settings.providerApiKeyEnv",
  "settings.balanceUrlPlaceholder",
  "settings.supportedEffortsHint",
  "settings.providerNamePlaceholder",
];
const credentialCopyDictionaries = [
  { locale: "zh", dict: zh, forbidden: /全局保存|全局\s*\.env|KWU 全局 \.env/i, required: /当前用户配置|本地凭据|当前配置/ },
];
const genericSettingsCopyDictionaries = [
  { locale: "zh", dict: zh },
];

console.log("\nstartup settings contract");

ok(
  bridgeSource.includes("DesktopStartupSettings()"),
  "bridge exposes a lightweight desktop startup settings call",
);
ok(
  appSource.includes("app.DesktopStartupSettings()"),
  "App loads startup chrome preferences through the lightweight settings call",
);
ok(
  /typeof\s+app\.SetClientTimezone\s*===\s*["']function["']/.test(appSource),
  "timezone sync tolerates a stale Wails backend binding during frontend hot reload",
);
ok(!appSource.includes('components/OnboardingOverlay'), "App does not render the legacy API key onboarding overlay");
ok(!appSource.includes("app.NeedsOnboarding()"), "App does not probe for an API key before account registration");
ok(
  !/const\s+reloadSidebarImConnections[\s\S]*?app\.Settings\(\)[\s\S]*?\}, \[t\]\);/.test(appSource),
  "sidebar IM refresh avoids rebuilding the full Settings payload",
);
ok(
  syncDesktopPreferencesSource.length > 0 && !syncDesktopPreferencesSource.includes("app.Settings()"),
  "startup preference sync avoids rebuilding the full Settings payload",
);
ok(desktopPreferencesSource.includes('setLocalePreference("zh")'), "desktop startup always selects simplified Chinese");
ok(
  /function normalizeDesktopLayoutStyle\([\s\S]*?return "creation";[\s\S]*?\}/.test(desktopPreferencesSource),
  "desktop startup always normalizes layout to creation mode",
);
ok(
  !`${settingsSource}\n${generalSettingsSource}`.includes("app.SetDesktopLanguage") &&
    !`${settingsSource}\n${generalSettingsSource}`.includes("app.SetDesktopLayoutStyle"),
  "general settings no longer exposes language or layout mutations",
);
ok(
  generalSettingsSource.includes('>中文</span>') &&
    !generalSettingsSource.includes('label={t("settings.desktopLayoutStyle")}') &&
    !settingsSource.includes("desktopLayoutStyleLabel"),
  "general settings hides the fixed creation layout from its page and navigation summary",
);

ok(appSource.includes('return cleaned || "云智企办-会话";'), "browser export fallback filename uses the product name");
ok(!onboardingSource.includes("fallbackWarning"), "onboarding no longer exposes unreachable fallback warning copy");
ok(!onboardingSource.includes("usedFallback"), "onboarding does not branch on a backend fallback that is not implemented");

for (const { locale, dict, forbidden, required } of credentialCopyDictionaries) {
  for (const key of credentialCopyKeys) {
    const value = dict[key];
    ok(!forbidden.test(value), `${locale} ${key} avoids stale global credential storage wording`);
    ok(required.test(value), `${locale} ${key} describes current profile or local credentials storage`);
  }
}

for (const { locale, dict } of genericSettingsCopyDictionaries) {
  for (const key of genericSettingsCopyKeys) {
    const value = dict[key];
    ok(!/DeepSeek|DEEPSEEK_API_KEY|api\.deepseek\.com|deepseek-proxy/i.test(value), `${locale} ${key} avoids DeepSeek-specific generic settings copy`);
  }
}

for (const { locale, dict } of genericSettingsCopyDictionaries) {
  ok(!new RegExp("sui" + "yuan binary", "i").test(dict["mock.askQ2Prompt"]), `${locale} mock action copy avoids the legacy binary name`);
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
