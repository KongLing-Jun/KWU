// Run: tsx src/__tests__/model-marketplace-app-contract.test.ts

import { readFileSync } from "node:fs";
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
const navigationSource = readFileSync(resolve(here, "../app/useWorkspaceNavigation.ts"), "utf8");
const showcaseSource = readFileSync(resolve(here, "../features/account/ui/AccountModelShowcase.tsx"), "utf8");
const accessSource = readFileSync(resolve(here, "../lib/subscriptionModelAccess.ts"), "utf8");
const refreshSource = readFileSync(resolve(here, "../lib/subscriptionMarketplaceRefresh.ts"), "utf8");

console.log("\naccount model showcase app contract");

ok(
  !appSource.includes('import("./features/model-marketplace/ui/ModelMarketplace")') &&
    !navigationSource.includes('"modelMarketplace"'),
  "App removes the standalone model marketplace workspace",
);

ok(
  /const prepare = useCallback\(\(closeSidebar: boolean\) => \{[\s\S]*closeTransientOverlays\(\);[\s\S]*clearSidebarDetail\(\);[\s\S]*clearSettingsTarget\(\);/.test(navigationSource) &&
    /const openModelMarketplace = useCallback\(\(\) => \{[\s\S]*prepare\(false\);[\s\S]*setWorkspaceView\("account"\);/.test(navigationSource),
  "legacy model entry closes transient surfaces and opens the combined account page",
);

ok(
  !appSource.includes('t("sidebar.modelMarketplace")') &&
    !appSource.includes('workspaceView === "modelMarketplace"'),
  "all sidebar variants remove the standalone model marketplace entry",
);

ok(
  /workspaceView === "account" \? \([\s\S]*<AccountCenter[\s\S]*modelCatalog=\{sharedModelCatalog\}/.test(appSource),
  "combined account page receives the shared model catalog",
);

ok(
  showcaseSource.includes("KNOWN_PROVIDERS") &&
    showcaseSource.includes("provider.models.slice(0, 3)") &&
    showcaseSource.includes("data-account-provider-dialog") &&
    !showcaseSource.includes("onSelectModel"),
  "account showcase previews three models and exposes a browse-only provider dialog",
);

ok(
  /const topicbarVisible = workspaceView === "chat" \|\| Boolean\(sidebarImDetailConnection\);/.test(appSource) &&
    /\{topicbarVisible && \(\s*<header className="topicbar">/.test(appSource),
  "non-chat workspace views hide the chat topicbar while preserving detail pages",
);

ok(
    !appSource.includes("modelMarketplaceAutoRefreshClaimedRef") &&
    !appSource.includes("claimModelMarketplaceAutoRefresh") &&
    appSource.includes("ensureModelMarketplaceRefresh") &&
    appSource.includes("refreshSubscriptionMarketplaceOnce") &&
    refreshSource.includes("claimSubscriptionMarketplaceRefresh") &&
    refreshSource.includes("subscriptionMarketplaceRefreshSucceeded") &&
    refreshSource.includes("readMarketplace") &&
    accessSource.includes("subscriptionMarketplaceNeedsRefresh"),
  "App owns the scope-aware bounded refresh while the former standalone workspace ownership stays removed",
);

ok(
  !/SetModel\(/.test(navigationSource.match(/const openModelMarketplace[\s\S]*?\}, \[prepare, setWorkspaceView\]\);/)?.[0] ?? ""),
  "model marketplace entry does not change the active chat model",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
