import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(testDir, "../App.tsx"), "utf8");
const controllerSource = readFileSync(resolve(testDir, "../features/experts/controller/useExpertCatalog.ts"), "utf8");

assert.doesNotMatch(
  appSource,
  /refreshExpertState\([^)]*\)\.then\(\(\) => \{[\s\S]*?pendingExpertActivationRef\.current = null/,
  "an obsolete refresh completion must not clear the optimistic summoned-expert identity",
);
assert.match(
  controllerSource,
  /pendingExpertActivationIdForTab\(pendingActivation, targetTabId\)/,
  "the current task resolves only the pending activation scoped to that task",
);
assert.match(
  appSource,
  /request\.requestSequence !== expertActivationSequenceRef\.current/,
  "a superseded expert request cannot overwrite the latest optimistic selection",
);

console.log("expert summon identity handoff passed");
