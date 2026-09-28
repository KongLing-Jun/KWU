import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/expert-workspace-layout.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

let passed = 0;
let failed = 0;
const testDir = dirname(fileURLToPath(import.meta.url));
const styles = loadStylesSource().replace(/\/\*[\s\S]*?\*\//g, "");

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

console.log("\nexpert workspace layout contract");

ok(
  /\.agent-workspace__view\s*>\s*\.msg--user\s*,[\s\S]*?\{[\s\S]*?align-self:\s*stretch\s*;[\s\S]*?width:\s*100%\s*;[\s\S]*?margin-left:\s*0\s*;[\s\S]*?margin-right:\s*0\s*;/.test(styles),
  "expert workspace stretches the user message row so its bubble reaches the shared right content edge",
);

ok(
  /\.msg--user\s*\{[\s\S]*?align-items:\s*flex-end\s*;/.test(styles),
  "the stretched user message row keeps the bubble right-aligned",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
