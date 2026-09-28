// Run: tsx src/__tests__/bundle-contract.test.ts

import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

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
const settingsSource = readFileSync(resolve(here, "../features/settings/ui/SettingsPanel.tsx"), "utf8");
const botsSettingsSource = readFileSync(resolve(here, "../features/settings/ui/BotsSettingsSection.tsx"), "utf8");
const markdownSource = readFileSync(resolve(here, "../components/Markdown.tsx"), "utf8");
const agentWorkspaceSource = readFileSync(resolve(here, "../components/AgentWorkspace.tsx"), "utf8");
const toolCardSource = readFileSync(resolve(here, "../components/ToolCard.tsx"), "utf8");
const bridgeSource = readFileSync(resolve(here, "../lib/bridge.ts"), "utf8");
const i18nSource = readFileSync(resolve(here, "../lib/i18n.tsx"), "utf8");
const viteSource = readFileSync(resolve(here, "../../vite.config.ts"), "utf8");

console.log("\nbundle contract");

ok(
  !/import\s+\{[^}]*\}\s+from\s+["']\.\/lib\/sessionExport["']/.test(appSource),
  "App keeps session export code out of the initial chunk",
);
ok(
  appSource.includes('import("./lib/sessionExport")'),
  "App loads session export code on demand",
);
ok(
  !/import\s+\{[^}]*\}\s+from\s+["']\.\/components\/SettingsPanel["']/.test(appSource) &&
    !/import\s+\{[^}]*\}\s+from\s+["']\.\/components\/HistoryPanel["']/.test(appSource),
  "App keeps secondary drawers out of the initial chunk",
);
ok(
  appSource.includes('import("./features/settings")') &&
    appSource.includes('import("./components/HistoryPanel")'),
  "App loads secondary drawers on demand",
);
ok(
  appSource.includes('import("./features/workspace/ui/ProjectDirectory")')
    && appSource.includes('const loadModelMarketplace = () => import("./features/model-marketplace/ui/ModelMarketplace")')
    && appSource.includes('import("./features/account/ui/AccountCenter")')
    && appSource.includes('import("./features/experts/ui/ExpertDirectory")')
    && appSource.includes('import("./features/automation/heartbeat/HeartbeatPanel")'),
  "App lazy-loads non-chat project, model marketplace, account, expert and automation workspaces",
);
ok(
  (appSource.match(/onPointerEnter=\{\(\) => void loadModelMarketplace\(\)\}/g) ?? []).length >= 3 &&
    (appSource.match(/onFocus=\{\(\) => void loadModelMarketplace\(\)\}/g) ?? []).length >= 3,
  "model marketplace navigation preloads its chunk on pointer and keyboard intent",
);
ok(
  appSource.includes('from "./features/account/store/accountStore"') &&
    !appSource.includes('from "./features/account"'),
  "App does not pull the lazy account workspace through its barrel export",
);
ok(
  !agentWorkspaceSource.includes('import("../lib/bridge")') &&
    !toolCardSource.includes('import("../lib/bridge")'),
  "archived tool results reuse the bridge already present in the initial chunk",
);
ok(
  !/import\s+\{[^}]*\b(?:MCPServersSettingsPage|SkillsSettingsPage)\b[^}]*\}\s+from\s+["']\.\/CapabilitiesPanel["']/.test(settingsSource) &&
    !/import\s+\{[^}]*\bMemorySettingsPage\b[^}]*\}\s+from\s+["']\.\/MemoryPanel["']/.test(settingsSource),
  "SettingsPanel keeps secondary settings pages out of the first settings chunk",
);
ok(
  settingsSource.includes('import("../../../components/CapabilitiesPanel")') &&
    settingsSource.includes('import("../../../components/MemoryPanel")'),
  "SettingsPanel loads secondary settings pages on demand",
);
ok(
  !/from\s+["']qrcode\.react["']/.test(botsSettingsSource),
  "BotsSettingsSection keeps QR rendering code out of its initial chunk",
);
ok(
  botsSettingsSource.includes('import("qrcode.react")'),
  "BotsSettingsSection loads QR rendering code on demand",
);
ok(
  !/from\s+["']react-markdown["']/.test(markdownSource) &&
    !/from\s+["']remark-gfm["']/.test(markdownSource) &&
    !/from\s+["']remark-math["']/.test(markdownSource) &&
    !/from\s+["']rehype-katex["']/.test(markdownSource) &&
    !/katex\/dist\/katex\.min\.css/.test(markdownSource),
  "Markdown wrapper keeps markdown/math vendor code out of the initial chunk",
);
ok(
  markdownSource.includes('import("./MarkdownRenderer")'),
  "Markdown wrapper loads markdown renderer on demand",
);
ok(
  !bridgeSource.includes("bridgeMock") &&
    !viteSource.includes("__ENABLE_BROWSER_MOCK__") &&
    !existsSync(resolve(here, "../platform/wails/bridgeMock.ts")) &&
    !existsSync(resolve(here, "../lib/bridgeMock.ts")),
  "frontend bundle has no browser mock module or Vite-only mock flag",
);
ok(
  i18nSource.includes('from "../locales/zh"') &&
    !i18nSource.includes('locales/en') &&
    !i18nSource.includes('locales/zh-TW') &&
    !existsSync(resolve(here, "../locales/en.ts")) &&
    !existsSync(resolve(here, "../locales/zh-TW.ts")),
  "desktop bundle ships only the Simplified Chinese dictionary",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
