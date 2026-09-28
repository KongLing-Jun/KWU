// Run: tsx src/__tests__/brand-contract.test.ts

import { readFileSync } from "node:fs";

import {
  BRAND_DISPLAY_NAME,
  BRAND_ENGLISH_NAME,
  BRAND_SLUG,
  BRAND_PROVIDER_ID,
  LEGACY_BRAND_SLUG,
  LEGACY_PROVIDER_ID,
  OFFICIAL_API_KEY_ENV,
  LEGACY_OFFICIAL_API_KEY_ENV,
  PROJECT_CONFIG_NAME,
  LEGACY_PROJECT_CONFIG_NAME,
} from "../lib/brand";

const accountSidebarSource = readFileSync(
  new URL("../features/account/controller/useAccountSidebar.ts", import.meta.url),
  "utf8",
);

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

console.log("\nbrand contract");

ok(BRAND_DISPLAY_NAME === "云智企办", "display name is the Chinese product name");
ok(BRAND_ENGLISH_NAME === "Clouvia", "English brand is Clouvia");
ok(BRAND_SLUG === "clouvia", "new visible slug is lowercase clouvia");
ok(LEGACY_BRAND_SLUG === "kwu", "legacy technical slug remains readable");
ok(BRAND_PROVIDER_ID === "kwu", "new provider id is lowercase kwu");
ok(LEGACY_PROVIDER_ID === "kwu", "legacy provider id aliases the current provider id");
ok(OFFICIAL_API_KEY_ENV === "KWU_API_KEY", "official API key env uses the KWU prefix");
ok(LEGACY_OFFICIAL_API_KEY_ENV === "KWU_API_KEY", "legacy API key env remains available for compatibility");
ok(PROJECT_CONFIG_NAME === "clouvia.toml", "new project config name is clouvia.toml");
ok(LEGACY_PROJECT_CONFIG_NAME === "kwu.toml", "legacy project config name remains readable");
ok(
  accountSidebarSource.includes('import { BRAND_SLUG } from "../../../lib/brand"')
    && accountSidebarSource.includes("`${BRAND_SLUG}.sidebar-avatar.${accountID}`")
    && accountSidebarSource.includes("hasRecoverableCloudIdentity(account)")
    && !accountSidebarSource.includes("displayName?.trim()")
    && !accountSidebarSource.includes("reasonix.sidebar-avatar"),
  "avatar storage uses the current brand slug and stable recoverable identity",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
