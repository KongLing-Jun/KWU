// Run: tsx src/__tests__/tool-approval-default-preference.test.ts

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

let passed = 0;
let failed = 0;

function ok(condition: boolean, label: string) {
  if (condition) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\ntool approval default preference");

const appSource = readFileSync(fileURLToPath(new URL("../App.tsx", import.meta.url)), "utf8");
const controllerSource = readFileSync(fileURLToPath(new URL("../features/session/useController.ts", import.meta.url)), "utf8");
const settingsSource = readFileSync(fileURLToPath(new URL("../features/settings/ui/GeneralSettingsSection.tsx", import.meta.url)), "utf8");

ok(
  !/toolApprovalDefaultPreference|SetDefaultToolApprovalMode\(m\)/.test(appSource),
  "composer approval selection does not update the new-session default",
);
ok(
  /await setControllerToolApprovalMode\(m\);[\s\S]*?patchComposerProfile/.test(appSource),
  "composer profile updates only after backend persistence succeeds",
);
ok(
  /SetToolApprovalModeForTab\(activeTabId, mode\);/.test(controllerSource)
    && !/SetToolApprovalModeForTab\(activeTabId, mode\)\.catch/.test(controllerSource),
  "session controller propagates approval persistence errors",
);
ok(
  /SyncToolApprovalModeForTab\(activeTabId, mode\)/.test(controllerSource)
    && /if \(!controllerReady\) return;[\s\S]*?syncControllerToolApprovalMode\(toolApprovalMode\)/.test(appSource),
  "controller hydration syncs runtime posture without persisting legacy fallback",
);
ok(
  /app\.SetDefaultToolApprovalMode\(mode\)/.test(settingsSource),
  "settings remains the owner of the ordinary new-session default",
);
ok(
  /const setProjectToolApprovalMode = async[\s\S]*?await app\.SetToolApprovalModeForTab\(targetTabId, mode\);[\s\S]*?patchProfile\(\{ toolApprovalMode: mode \}\)/.test(appSource),
  "prepared project composer persists before updating its profile",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
