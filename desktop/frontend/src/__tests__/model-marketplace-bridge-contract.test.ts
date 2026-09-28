// Run: tsx src/__tests__/model-marketplace-bridge-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { app } from "../lib/bridge";

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
const typesSource = readFileSync(resolve(here, "../lib/types.ts"), "utf8");
const bridgeSource = readFileSync(resolve(here, "../lib/bridge.ts"), "utf8");

console.log("\nmodel marketplace bridge contract");

ok(
  /export interface ModelMarketplaceModel \{[\s\S]*id: string;[\s\S]*name: string;[\s\S]*billingType\?: string;[\s\S]*quotaType\?: number;[\s\S]*prices\?: ModelMarketplacePriceFact\[\];[\s\S]*pricingTiers\?: ModelMarketplacePricingTier\[\];[\s\S]*\}/.test(typesSource) &&
    /export interface ModelMarketplacePriceFact \{[\s\S]*key: string;[\s\S]*value: string;[\s\S]*\}/.test(typesSource) &&
    /export interface ModelMarketplaceSource \{[\s\S]*pricingState: string;[\s\S]*pricingAuthenticity\?: string;[\s\S]*statusAuthenticity\?: string;[\s\S]*metadataStatus: string;[\s\S]*partial\?: boolean;[\s\S]*models: ModelMarketplaceModel\[\];[\s\S]*errorCode\?: string;[\s\S]*\}/.test(typesSource) &&
    /export interface ModelMarketplaceView \{[\s\S]*sources: ModelMarketplaceSource\[\];[\s\S]*\}/.test(typesSource),
  "types.ts exposes AirClaude exact-string price facts and structured source state",
);

ok(
  /ModelMarketplaceView,/.test(bridgeSource) &&
    /ModelMarketplace\(\): Promise<ModelMarketplaceView>;/.test(bridgeSource) &&
    /RefreshModelMarketplace\(\): Promise<ModelMarketplaceView>;/.test(bridgeSource),
  "bridge exposes separate cache-only read and explicit refresh methods",
);

const expectedView = { sources: [] };
const globalWithWindow = globalThis as Record<string, unknown>;
const previousWindow = globalWithWindow.window;
globalWithWindow.window = {
  go: {
    main: {
      App: {
        ModelMarketplace: async () => expectedView,
        RefreshModelMarketplace: async () => expectedView,
      },
    },
  },
};

ok(
  (await app.ModelMarketplace()) === expectedView &&
    (await app.RefreshModelMarketplace()) === expectedView,
  "marketplace reads and refreshes forward to the live desktop binding",
);

if (previousWindow === undefined) delete globalWithWindow.window;
else globalWithWindow.window = previousWindow;

ok(
  typeof app.RefreshModelMarketplace === "function" &&
    /ModelsForTab\(tabID: string\): Promise<ModelInfo\[\]>;/.test(bridgeSource) &&
    !/newapi\.suiyuan-ai\.com\/pricing/.test(bridgeSource),
  "desktop bridge keeps the configured-model contract separate from marketplace pricing",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
