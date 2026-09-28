// Run: tsx src/__tests__/model-catalog-fingerprint.test.ts

import { settingsModelCatalogFingerprint } from "../lib/modelCatalogFingerprint";
import type { SettingsView } from "../lib/types";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  process.stdout.write(`  ${value ? "PASS" : "FAIL"}  ${label}\n`);
  if (value) passed += 1;
  else failed += 1;
}

function settings(overrides: Partial<SettingsView> = {}): SettingsView {
  return {
    providers: [],
    officialProviders: [],
    providerKinds: [],
    ...overrides,
  } as SettingsView;
}

console.log("\nmodel catalog fingerprint");

const empty = settingsModelCatalogFingerprint(settings());
ok(empty !== null, "an empty settled catalog has a stable non-null fingerprint");
ok(
  settingsModelCatalogFingerprint(settings({
    providers: [{
      name: "custom",
      kind: "openai",
      models: ["model-a"],
    }] as SettingsView["providers"],
  })) !== empty,
  "a nonempty to empty provider transition changes the catalog fingerprint",
);
ok(
  settingsModelCatalogFingerprint(settings({ providerKinds: ["openai"] })) !== empty,
  "provider kind registration participates in the catalog fingerprint",
);
ok(
  settingsModelCatalogFingerprint(settings({ providerKinds: ["anthropic", "openai"] }))
    === settingsModelCatalogFingerprint(settings({ providerKinds: ["openai", "anthropic"] })),
  "provider kind ordering does not create a false catalog change",
);
ok(settingsModelCatalogFingerprint(null) === null, "an absent settings snapshot remains distinguishable from an empty settled catalog");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
