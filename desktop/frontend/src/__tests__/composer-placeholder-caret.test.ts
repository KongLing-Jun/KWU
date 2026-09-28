// Run: tsx src/__tests__/composer-placeholder-caret.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const css = readFileSync(resolve(here, "../styles/session.css"), "utf8");

let failed = 0;
function ok(condition: boolean, label: string) {
  process.stdout.write(`  ${condition ? "PASS" : "FAIL"}  ${label}\n`);
  if (!condition) failed += 1;
}

console.log("\ncomposer placeholder caret");

ok(
  /\.composer__input\s*\{[^}]*position:\s*relative;/s.test(css),
  "composer input establishes the placeholder positioning context",
);
ok(
  /\.composer__input\[data-empty="true"\]::before\s*\{[^}]*position:\s*absolute;[^}]*inset-block-start:\s*0;[^}]*inset-inline-start:\s*0;/s.test(css),
  "empty placeholder stays out of the editable text flow so the caret starts at offset zero",
);

console.log(`\n${2 - failed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
