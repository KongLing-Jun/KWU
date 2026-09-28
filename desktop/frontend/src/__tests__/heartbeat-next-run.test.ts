// Run: tsx src/__tests__/heartbeat-next-run.test.ts

import { automationRunStatusTone, heartbeatBuildCycleInterval } from "../features/automation/heartbeat/HeartbeatPanel";

let passed = 0;
let failed = 0;

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\nheartbeat schedule helpers");

eq(
  heartbeatBuildCycleInterval("daily", [], "09:00"),
  "24h|weekly:mon@09:00",
  "empty daily day selection does not save as every day",
);

eq(
  heartbeatBuildCycleInterval("weekly", [], "09:00"),
  "168h|weekly:mon@09:00",
  "weekly default uses one weekday",
);

eq(automationRunStatusTone("running"), "running", "running status uses running tone");
eq(automationRunStatusTone("succeeded"), "success", "succeeded status uses success tone");
eq(automationRunStatusTone("blocked"), "warning", "blocked status uses warning tone");
eq(automationRunStatusTone("failed"), "danger", "failed status uses danger tone");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
