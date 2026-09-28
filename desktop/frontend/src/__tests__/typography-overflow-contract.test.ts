import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/typography-overflow-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { TEXT_SIZES } from "../lib/textSize";

const testDir = dirname(fileURLToPath(import.meta.url));
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

function eq(a: unknown, b: unknown, label: string) {
  if (a === b) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}: expected ${JSON.stringify(b)}, got ${JSON.stringify(a)}\n`);
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

function hasDeclaration(selector: string, property: string, expected: string): boolean {
  return matchingBlocks(selector).some((block) => {
    const declaration = new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g");
    let match: RegExpExecArray | null;
    while ((match = declaration.exec(block)) !== null) {
      if (match[1].trim() === expected) return true;
    }
    return false;
  });
}

function clipsSingleLine(selector: string) {
  eq(finalDeclaration(selector, "overflow"), "hidden", `${selector} clips long text`);
  eq(finalDeclaration(selector, "text-overflow"), "ellipsis", `${selector} uses ellipsis`);
  eq(finalDeclaration(selector, "white-space"), "nowrap", `${selector} stays on one line`);
}

console.log("\ntypography overflow contract");

eq(
  JSON.stringify(TEXT_SIZES),
  JSON.stringify(["small", "default", "large", "xlarge", "xxlarge"]),
  "text-size presets include the large accessibility step",
);
eq(finalDeclaration(":root", "--sans"), "var(--font-ui)", "legacy sans alias stays synced with UI font");
eq(finalDeclaration(':root[data-text-size="xxlarge"]', "--font-scale"), "1.32", "xxlarge has a real scale bump");
ok(
  (finalDeclaration(":root", "--statusbar-dock-height") ?? "").includes("var(--font-scale)"),
  "status bar dock height scales with interface text size",
);
ok(
  hasDeclaration(".layout", "--statusbar-height", "var(--statusbar-dock-height)"),
  "layout reserves scaled status bar height",
);
eq(
  finalDeclaration(".app", "height"),
  "var(--app-viewport-height, 100%)",
  "app height follows the live viewport height variable",
);
eq(finalDeclaration(".transcript--empty", "overflow-y"), "auto", "empty transcript can scroll instead of clipping");
eq(finalDeclaration(".welcome", "overflow"), "visible", "welcome empty state is not clipped by its own box");
ok(
  hasDeclaration(".transcript--empty > .welcome", "margin-block", "auto"),
  "empty-state auto margins apply only to the welcome content",
);
ok(
  finalDeclaration(".transcript--empty > *", "margin-block") === undefined,
  "empty-state generic children do not receive auto margins",
);
eq(
  finalDeclaration(":root[data-theme-style] .statusbar", "height"),
  "var(--statusbar-dock-height)",
  "fixed status bar height follows the scaled dock token",
);
eq(
  finalDeclaration(":root[data-theme-style] .statusbar", "min-height"),
  "var(--statusbar-dock-height)",
  "status bar min-height follows the scaled dock token",
);

eq(finalDeclaration(".statusbar", "white-space"), "nowrap", "status bar keeps metrics on one row");
eq(finalDeclaration(".statusbar", "overflow"), "hidden", "status bar clips instead of overflowing");
clipsSingleLine(".statusbar__model");

for (const selector of [
  ".sidebar-im__summary-label",
  ".sidebar-im__summary-status",
  ".workbench-dock__tab-label",
  ".workspace-files__scope-title",
  ".workspace-files__scope-meta",
  ".context-panel__section-head span",
  ".context-panel__metric span",
  ".context-panel__metric strong",
  ".app--creation .context-panel__mini-stat span",
  ".app--creation .context-panel__mini-stat strong",
  ".topbar__model",
  ".composer-modebar__item span",
  ".composer-more-menu__item span",
]) {
  clipsSingleLine(selector);
}

eq(
  finalDeclaration(".app--creation .layout.layout--workspace-open", "transition"),
  "grid-template-columns 0s, min-width 0s",
  "creation dock skips zero-width grid interpolation on open",
);
eq(
  finalDeclaration(".app--creation .context-panel__usage", "animation"),
  "none",
  "creation overview usage card disables inherited entrance animation",
);
ok(
  finalDeclaration(".app--creation .context-panel__mini-stat", "justify-content") !== "space-between",
  "creation overview rows avoid edge-pinned value alignment",
);
ok(
  finalDeclaration(".app--creation .context-panel__mini-stat", "grid-template-columns") !== "minmax(0, 1fr) auto",
  "creation overview rows avoid the spacer grid that pushes values to the edge",
);
ok(
  finalDeclaration(".app--creation .context-panel__mini-stat strong", "max-width") !== "14ch",
  "creation overview values are not capped to a fixed 14ch width",
);

eq(finalDeclaration(".composer-modebar", "overflow"), "hidden", "chat mode switcher contains enlarged labels");
eq(finalDeclaration(".agent-workspace__tabs", "overflow-x"), "auto", "expert workspace tabs scroll horizontally");
eq(finalDeclaration(".agent-workspace__tab", "flex"), "0 0 auto", "expert workspace tabs keep their content width instead of shrinking names");
eq(finalDeclaration(".agent-workspace__view", "justify-content"), "safe flex-end", "expert workspace content falls back to top alignment instead of clipping tall titles");
eq(
  finalDeclaration(".todobar__item", "grid-template-columns"),
  "var(--todobar-status-width) minmax(0, 1fr)",
  "todo rows keep a fixed status column so expert-team task text aligns",
);
eq(finalDeclaration(".todobar__status", "width"), "100%", "todo status pills fill the fixed status column");
eq(finalDeclaration(".todobar__item--sub", "padding-left"), "0", "todo subtask indentation does not shift the text column");
for (const selector of [
  ".agent-workspace__view > .msg--assistant",
  ".agent-workspace__view > .tool",
  ".agent-workspace__view > .tool-group",
  ".agent-workspace__view > .turn-collapse",
  ".agent-workspace__view > .readonly-batch",
  ".agent-workspace__view > .expert-team-flow-card",
]) {
  eq(finalDeclaration(selector, "align-self"), "stretch", `${selector} stretches to the expert workspace text column`);
  eq(finalDeclaration(selector, "width"), "100%", `${selector} uses the full expert workspace text column`);
  eq(finalDeclaration(selector, "margin-left"), "0", `${selector} does not center itself in the expert workspace`);
  eq(finalDeclaration(selector, "margin-right"), "0", `${selector} keeps the same left edge as neighboring text`);
}
eq(finalDeclaration(".agent-workspace__view .tool__subject", "white-space"), "normal", "expert workspace tool subjects wrap instead of being clipped");
eq(finalDeclaration(".agent-workspace__view .tool__subject", "text-overflow"), "clip", "expert workspace tool subjects avoid ellipsis clipping");
ok(
  /@container\s*\(max-width:\s*760px\)[\s\S]*?\.composer-meta--has-intent-chip\s+\.composer-meta__control--model\s*\{[\s\S]*?flex\s*:\s*1 1 160px[\s\S]*?\.composer-meta__control--effort\s*\{[\s\S]*?display\s*:\s*none[\s\S]*?\.composer-meta__control--more\s*\{[\s\S]*?display\s*:\s*inline-flex/.test(styles),
  "composer compact controls activate at the capped theme width",
);
eq(finalDeclaration(".md table", "overflow-x"), "auto", "markdown tables scroll horizontally");
eq(finalDeclaration(".code", "overflow"), "auto", "code blocks scroll instead of widening the layout");
ok(
  /@media\s*\(max-width:\s*900px\)[\s\S]*?\.settings-center\s*\{[\s\S]*?grid-template-columns\s*:\s*1fr/.test(styles),
  "settings center stacks navigation before the modal is too narrow",
);
ok(
  /@media\s*\(max-width:\s*900px\)[\s\S]*?\.settings-field\s*\{[\s\S]*?grid-template-columns\s*:\s*1fr/.test(styles),
  "settings fields collapse to one column at the mid-width breakpoint",
);
eq(
  finalDeclaration(".app--creation .settings-center .set-seg__btn", "overflow"),
  "hidden",
  "themed settings segments contain labels within their own grid track",
);
eq(
  finalDeclaration(".app--creation .settings-center .set-seg__btn", "text-overflow"),
  "ellipsis",
  "themed settings segments use an ellipsis instead of covering adjacent labels",
);
eq(
  finalDeclaration(".app--creation .settings-page--models .subagent-effort-control .set-seg__btn:first-child", "white-space"),
  "normal",
  "the long automatic subagent effort label wraps inside its segment",
);
eq(
  finalDeclaration(".app--creation .settings-page--models .subagent-effort-control .set-seg__btn:first-child", "overflow-wrap"),
  "anywhere",
  "localized automatic effort text can wrap at constrained widths",
);
ok(
  /@media\s*\(max-width:\s*760px\)[\s\S]*?\.settings-modal\s*\{[\s\S]*?width\s*:\s*100vw[\s\S]*?height\s*:\s*100vh/.test(styles),
  "settings modal only becomes fullscreen at the narrow breakpoint",
);

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
