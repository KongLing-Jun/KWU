// Run: tsx src/__tests__/text-size.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { DEFAULT_TEXT_SIZE, nextTextSize } from "../lib/textSize";

const testDir = dirname(fileURLToPath(import.meta.url));
const textSizeSource = readFileSync(resolve(testDir, "../lib/textSize.ts"), "utf8");

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
    failed += 1;
  }
}

console.log("\ntext size shortcuts");
eq(nextTextSize("small", 1), DEFAULT_TEXT_SIZE, "increase from small");
eq(nextTextSize("default", 1), "large", "increase from default");
eq(nextTextSize("large", 1), "xlarge", "increase from large");
eq(nextTextSize("xlarge", 1), "xxlarge", "increase from xlarge");
eq(nextTextSize("xxlarge", 1), "xxlarge", "increase clamps at largest");
eq(nextTextSize("xxlarge", -1), "xlarge", "decrease from xxlarge");
eq(nextTextSize("xlarge", -1), "large", "decrease from xlarge");
eq(nextTextSize("large", -1), DEFAULT_TEXT_SIZE, "decrease from large");
eq(nextTextSize("default", -1), "small", "decrease from default");
eq(nextTextSize("small", -1), "small", "decrease clamps at smallest");

console.log("\ntext size branding storage contract");
ok(textSizeSource.includes('const TEXT_SIZE_KEY = "kwu-text-size";'), "text size storage key uses kwu namespace");

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
