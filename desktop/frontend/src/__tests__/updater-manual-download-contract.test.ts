// Run: tsx src/__tests__/updater-manual-download-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const updater = readFileSync(resolve(here, "../lib/useUpdater.ts"), "utf8");
const bridge = readFileSync(resolve(here, "../lib/bridge.ts"), "utf8");
const updateCenter = readFileSync(resolve(here, "../components/UpdateCenterDialog.tsx"), "utf8");

let failed = 0;
function ok(condition: boolean, label: string) {
  process.stdout.write(`  ${condition ? "PASS" : "FAIL"}  ${label}\n`);
  if (!condition) failed += 1;
}

console.log("\nupdater manual download contract");

ok(
  /if \(!info\.canSelfUpdate\) \{\s*void app\.OpenUpdateDownload\(info\.downloadUrl\)/.test(updater),
  "manual updates open the release-specific cloud URL",
);
ok(
  /OpenUpdateDownload\(url: string\): Promise<void>/.test(bridge),
  "the Wails bridge exposes the release-specific download method",
);
ok(
  /manualUpdateHintKey\(status\.info\.manualKind\)/.test(updateCenter),
  "the update center shows a platform-specific manual update hint",
);

console.log(`\n${3 - failed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
