import assert from "node:assert/strict";

import { classifyTestAssetPath, collectTestingInventory } from "./testing-inventory.mjs";

assert.equal(classifyTestAssetPath("internal/agent/agent_test.go"), "root-go");
assert.equal(classifyTestAssetPath("desktop/app_test.go"), "desktop-go");
assert.equal(
  classifyTestAssetPath("desktop/frontend/src/__tests__/app.test.tsx"),
  "frontend",
);
assert.equal(
  classifyTestAssetPath("scripts/desktop-actions-draft-test.ps1"),
  "release-contract",
);
assert.equal(classifyTestAssetPath("docs/testing.md"), null);

const inventory = collectTestingInventory();
assert.equal(inventory.schemaVersion, 1);
assert.match(inventory.sourceCommit, /^[0-9a-f]{40}$/);
assert.ok(inventory.surfaces.every((surface) => surface.fileCount > 0));
assert.ok(inventory.capabilities.httptestFiles > 0);
assert.ok(inventory.workflows.includes(".github/workflows/ci.yml"));
assert.ok(inventory.workflows.includes(".github/workflows/nightly-go-regression.yml"));

console.log("testing inventory: all checks passed");
