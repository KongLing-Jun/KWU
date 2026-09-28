import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/topicbar-title-clipping.test.ts

const styles = loadStylesSource().replace(/\/\*[\s\S]*?\*\//g, "");

let passed = 0;
let failed = 0;

function ok(value: unknown, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function matchingBlocks(selector: string): string[] {
  const blocks: string[] = [];
  const rule = /([^{}]+)\{([^{}]*)\}/g;
  let match: RegExpExecArray | null;
  while ((match = rule.exec(styles)) !== null) {
    const selectors = match[1].split(",").map((part) => part.trim());
    if (selectors.includes(selector)) blocks.push(match[2]);
  }
  return blocks;
}

function finalDeclaration(selector: string, property: string): string | undefined {
  let value: string | undefined;
  for (const block of matchingBlocks(selector)) {
    const declaration = new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g");
    let match: RegExpExecArray | null;
    while ((match = declaration.exec(block)) !== null) {
      value = match[1].trim();
    }
  }
  return value;
}

const creationTitleRow = ":root[data-theme-style] .app--creation .topicbar__title-row";
const creationTitleButton = ":root[data-theme-style] .app--creation .topicbar__title-button";

console.log("\ntopicbar title clipping contract");

ok(
  finalDeclaration(".topicbar__identity", "overflow") === "hidden" &&
    finalDeclaration(creationTitleRow, "transform") === undefined,
  "creation title row stays within its clipping parent",
);
ok(
  finalDeclaration(creationTitleButton, "overflow") === "hidden" &&
    finalDeclaration(creationTitleButton, "text-overflow") === "ellipsis" &&
    finalDeclaration(creationTitleButton, "white-space") === "nowrap",
  "creation title button retains single-line ellipsis",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
