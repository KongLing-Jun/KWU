// Run: tsx src/__tests__/bot-settings-contract.test.ts

import {
  defaultBotSettings,
  normalizeBotConnection,
  normalizeBotMappingScope,
  normalizeBotSettings,
  normalizeBotToolApprovalMode,
} from "../lib/botSettings";

let passed = 0;
let failed = 0;

function equal(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}\n`);
    failed += 1;
  }
}

console.log("\nbot settings normalization contract");

const defaults = defaultBotSettings();
equal(defaults.toolApprovalMode, "ask", "default bot approval mode remains ask");
equal(defaults.allowlist.enabled, true, "allowlist protection remains enabled by default");
equal(defaults.qq.appSecretEnv, "QQ_BOT_APP_SECRET", "QQ keeps its established secret environment name");

const normalized = normalizeBotSettings({
  ...defaults,
  toolApprovalMode: "full-access",
  maxSteps: -3,
  debounceMs: 0,
  feishu: { ...defaults.feishu, domain: "lark", mode: "legacy" as "webhook" },
  allowlist: { ...defaults.allowlist, qqUsers: null as unknown as string[] },
});
equal(normalized.toolApprovalMode, "yolo", "legacy full-access approval normalizes to yolo");
equal(normalized.maxSteps, 0, "negative maximum steps clamp to zero");
equal(normalized.debounceMs, 1500, "zero debounce falls back to the existing default");
equal(normalized.feishu.domain, "lark", "Lark domain remains distinct from Feishu");
equal(normalized.feishu.mode, "webhook", "unknown Feishu mode normalizes to webhook");
equal(normalized.allowlist.qqUsers.length, 0, "missing allowlist arrays normalize to empty arrays");

const connection = normalizeBotConnection({
  id: "  bot-1 ",
  provider: " feishu ",
  enabled: undefined,
  toolApprovalMode: "bypass",
  workspaceRoot: " D:/project/demo ",
  sessionMappings: [{ remoteId: " room-1 ", workspaceRoot: " D:/project/demo " }],
});
equal(connection.id, "bot-1", "connection identifiers are trimmed");
equal(connection.enabled, true, "connections remain enabled unless explicitly disabled");
equal(connection.toolApprovalMode, "yolo", "connection bypass mode normalizes to yolo");
equal(connection.sessionMappings[0]?.scope, "project", "workspace mappings infer project scope");
equal(connection.sessionMappings[0]?.workspaceRoot, "D:/project/demo", "project workspace roots are trimmed");

equal(normalizeBotToolApprovalMode("", false), "ask", "empty global approval mode falls back to ask");
equal(normalizeBotToolApprovalMode("", true), "", "empty per-connection approval mode remains inherited");
equal(normalizeBotMappingScope("global", ""), "global", "empty workspace remains global");

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
