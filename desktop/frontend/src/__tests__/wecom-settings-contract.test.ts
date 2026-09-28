// Run: tsx src/__tests__/wecom-settings-contract.test.ts

import { readFileSync } from "node:fs";

const source = readFileSync(new URL("../features/settings/ui/BotsSettingsSection.tsx", import.meta.url), "utf8");

const checks: Array<[boolean, string]> = [
  [source.includes('{ target: "wecom", icon: weixinIcon, tone: "weixin" }'), "WeCom is exposed in the channel catalog with a real bundled asset"],
  [source.includes('provider: "wecom"') && source.includes('domain: "wecom"'), "WeCom setup creates an explicit provider/domain connection"],
  [source.includes('tokenEnv: DEFAULT_WECOM_SECRET_ENV'), "WeCom setup stores only a credential reference"],
  [source.includes('type="password"') && source.includes("saveConnectionSecret(selectedConnection)"), "WeCom secret entry uses the existing secure password flow"],
  [source.includes('selectedConnection.provider !== "wecom" && !selectedConnectionRemote'), "WeCom test delivery does not require a chat identifier"],
  [source.includes('connection.provider === "wecom" && diag.status === "ok"') && source.includes('{ enabled: true, status: "connected" }'), "A successful explicit WeCom test activates the catalog-eligible state"],
  [source.includes('weComReferenceChanged') && source.includes('{ enabled: false, status: "disconnected" }'), "Changing or replacing WeCom credentials revokes the connected state until retested"],
  [!source.includes('webhookUrl:'), "The settings surface does not persist a raw WeCom webhook URL"],
];

let failed = 0;
for (const [ok, label] of checks) {
  process.stdout.write(`  ${ok ? "PASS" : "FAIL"}  ${label}\n`);
  if (!ok) failed += 1;
}

if (failed > 0) process.exit(1);
