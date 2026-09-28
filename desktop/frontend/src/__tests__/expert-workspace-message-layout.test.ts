import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/expert-workspace-message-layout.test.ts

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

console.log("\nexpert workspace message layout");

ok(
  /\.agent-workspace__view\s*>\s*\.msg\s*\{[^}]*align-self:\s*stretch\s*;[^}]*width:\s*100%\s*;[^}]*margin-inline:\s*0\s*;/s.test(styles),
  "every direct expert workspace message row spans the shared content track",
);

ok(
  /\.msg--user\s*\{[^}]*align-items:\s*flex-end\s*;/s.test(styles),
  "the full-width user row keeps its bubble aligned to the right edge",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
