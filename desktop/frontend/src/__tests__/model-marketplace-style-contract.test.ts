import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/model-marketplace-style-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

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

function hasDeclaration(selector: string, property: string, expected: RegExp): boolean {
  return matchingBlocks(selector).some((block) => {
    const declaration = new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g");
    let match: RegExpExecArray | null;
    while ((match = declaration.exec(block)) !== null) {
      if (expected.test(match[1].trim())) return true;
    }
    return false;
  });
}

function blockText(selector: string): string {
  return matchingBlocks(selector).join("\n");
}

function avoidsExpensiveEffects(selectors: string[]): boolean {
  const declarations = selectors.map(blockText).join("\n");
  return !/(?:^|;)\s*(?:transform|filter|box-shadow|opacity|will-change|animation|transition)\s*:/.test(declarations);
}

console.log("\nmodel marketplace style contract");

ok(finalDeclaration(".model-marketplace", "background") === "var(--bg)", "page uses the app background token");
ok(
  hasDeclaration(".model-marketplace__header", "padding-top", /^22px$/),
  "page top spacing belongs to the header so sticky controls can reach the scroller top",
);
ok(
  hasDeclaration(".model-marketplace", "container-type", /^inline-size$/) &&
    hasDeclaration(".model-marketplace__header", "max-width", /^1280px$/) &&
    hasDeclaration(".model-marketplace-selectable", "max-width", /^1280px$/) &&
    hasDeclaration(".model-marketplace-catalog", "max-width", /^1280px$/),
  "header, selectable models, and model catalog share one constrained content axis",
);
ok(
  matchingBlocks(".model-marketplace-grid").some((block) =>
    /grid-template-columns:\s*repeat\(auto-fill,\s*minmax\(min\(100%,\s*220px\),\s*1fr\)\)/.test(block),
  ) && hasDeclaration(".model-marketplace-grid", "grid-auto-rows", /^minmax\(88px,\s*auto\)$/),
  "desktop grid packs compact model identity cards on one visual rhythm",
);
ok(
  hasDeclaration(".model-marketplace-controls", "position", /^sticky$/) &&
    hasDeclaration(".model-marketplace-controls", "top", /^0$/) &&
    hasDeclaration(".model-marketplace-controls", "z-index", /^var\(--z-inline-sticky\)$/) &&
    hasDeclaration(".model-marketplace-controls", "background", /var\(--bg\)/),
  "source tabs and filtering controls stay sticky inside the model marketplace scroller",
);
ok(finalDeclaration(".model-marketplace-card", "border-radius") === "8px", "model cards use the reference catalog radius");
ok(
  finalDeclaration(".model-marketplace-card", "content-visibility") === "auto" &&
    hasDeclaration(".model-marketplace-card", "contain-intrinsic-size", /88px/) &&
    hasDeclaration(".model-marketplace-card", "min-height", /^88px$/),
  "off-screen model cards defer layout and paint work",
);
ok(
  finalDeclaration(".model-marketplace-card", "background") === "var(--surface)" &&
    !/linear-gradient|radial-gradient|blur\(|backdrop-filter|bokeh|orb/i.test(blockText(".model-marketplace-card")),
  "cards stay quiet and avoid decorative gradients or blur effects",
);
ok(
  hasDeclaration(".model-marketplace-card__name", "display", /^block$/) &&
    hasDeclaration(".model-marketplace-card__name", "min-width", /^0$/) &&
    hasDeclaration(".model-marketplace-card__name", "overflow", /^hidden$/) &&
    hasDeclaration(".model-marketplace-card__name", "text-overflow", /^ellipsis$/) &&
    hasDeclaration(".model-marketplace-card__name", "white-space", /^nowrap$/) &&
    hasDeclaration(".model-marketplace-card__vendor", "overflow-wrap", /^anywhere$/),
  "long model names stay on one line with a trailing ellipsis",
);
ok(
  hasDeclaration(".model-marketplace-card__top", "flex", /^1 1 auto$/) &&
    hasDeclaration(".model-marketplace-card__top", "width", /^100%$/) &&
    hasDeclaration(".model-marketplace-card", "cursor", /^default$/) &&
    !styles.includes(".model-marketplace-card__summary") &&
    !styles.includes(".model-marketplace-card__tags") &&
    !styles.includes(".model-marketplace-card__details") &&
    !styles.includes(".model-marketplace-card__select-state"),
  "catalog cards only style the static model and vendor identity",
);
ok(
  matchingBlocks(".model-marketplace-card:hover:not(:disabled)").length === 0 &&
    matchingBlocks(".model-marketplace-card:focus-visible:not(:disabled)").length === 0,
  "static catalog cards expose no interactive hover or focus affordance",
);
ok(
  avoidsExpensiveEffects([
    ".model-marketplace-card",
  ]),
  "catalog cards avoid whole-card compositing effects and broad transitions",
);
ok(
  finalDeclaration(".model-marketplace-selectable__card", "background") === "var(--surface)" &&
    hasDeclaration(
      ".model-marketplace-selectable__card:hover:not(:disabled):not(.model-marketplace-selectable__card--current)",
      "border-color",
      /^var\(--model-market-border-hover\)$/,
    ) &&
    hasDeclaration(
      ".model-marketplace-selectable__card:focus-visible:not(:disabled):not(.model-marketplace-selectable__card--current)",
      "border-color",
      /^var\(--model-market-border-hover\)$/,
    ) &&
    hasDeclaration(
      ".model-marketplace-selectable__card:hover:not(:disabled):not(.model-marketplace-selectable__card--current)",
      "background",
      /var\(--surface\)/,
    ) &&
    hasDeclaration(
      ".model-marketplace-selectable__card:focus-visible:not(:disabled):not(.model-marketplace-selectable__card--current)",
      "background",
      /var\(--surface\)/,
    ),
  "ordinary selectable cards use lightweight theme hover and keyboard focus states",
);
ok(
  hasDeclaration(".model-marketplace-selectable__card--current", "border-color", /^var\(--model-market-solid\)$/) &&
    hasDeclaration(".model-marketplace-selectable__card--current", "background", /^var\(--model-market-solid\)$/) &&
    hasDeclaration(
      ".model-marketplace-selectable__card:disabled:not(.model-marketplace-selectable__card--current)",
      "background",
      /var\(--model-market-soft-strong\)/,
    ) &&
    !blockText(".model-marketplace-selectable__card:disabled:not(.model-marketplace-selectable__card--current)").includes("opacity"),
  "current stays themed even when disabled while ordinary busy cards use a neutral non-opacity state",
);
ok(
  avoidsExpensiveEffects([
    ".model-marketplace-selectable__card",
    ".model-marketplace-selectable__card:hover:not(:disabled):not(.model-marketplace-selectable__card--current)",
    ".model-marketplace-selectable__card:focus-visible:not(:disabled):not(.model-marketplace-selectable__card--current)",
    ".model-marketplace-selectable__card--current",
    ".model-marketplace-selectable__card:disabled:not(.model-marketplace-selectable__card--current)",
  ]),
  "selectable cards avoid whole-card compositing effects and broad transitions",
);
ok(
  /@container\s*\(max-width:\s*760px\)[\s\S]*?\.model-marketplace-grid\s*\{[\s\S]*?grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/.test(styles) &&
    /@container\s*\(max-width:\s*520px\)[\s\S]*?\.model-marketplace-grid\s*\{[\s\S]*?grid-template-columns:\s*1fr/.test(styles),
  "compact container queries use two columns before falling back to one",
);
ok(
  hasDeclaration(".model-marketplace-empty", "border-radius", /^8px$/) &&
    hasDeclaration(".model-marketplace-empty", "background", /var\(--model-market-soft\)/),
  "empty and failure state uses the same restrained surface language",
);
ok(
  hasDeclaration(".model-marketplace-selectable__scroller", "max-height", /^280px$/) &&
    hasDeclaration(".model-marketplace-selectable__scroller", "overflow-y", /^auto$/) &&
    hasDeclaration(".model-marketplace-selectable__scroller", "overscroll-behavior", /^contain$/),
  "configured models use a bounded, independently scrollable region",
);
ok(
  hasDeclaration(".model-marketplace-toolbar", "grid-template-columns", /minmax\(240px,\s*1fr\) minmax\(136px,\s*164px\) minmax\(136px,\s*164px\)/) &&
    !styles.includes(".model-marketplace__open") &&
    !styles.includes(".model-marketplace-card__price-grid"),
  "toolbar fits search and two filters while external and price affordances stay removed",
);
ok(
  hasDeclaration(".model-marketplace-filter", "grid-template-columns", /auto minmax\(0, 1fr\) auto/) &&
    finalDeclaration(".model-marketplace-filter", "background") === "var(--model-market-soft)" &&
    hasDeclaration(".model-marketplace-filter--open", "border-color", /^var\(--model-market-border-hover\)$/) &&
    hasDeclaration(".model-marketplace-filter--open", "background", /var\(--model-market-soft-strong\)/) &&
    hasDeclaration(".model-marketplace-filter-menu", "border", /var\(--model-market-border-hover\)/) &&
    finalDeclaration(".model-marketplace-filter-menu", "background") === "var(--surface)",
  "vendor and plan filter triggers and popovers use a lightweight theme surface",
);
ok(
  hasDeclaration(
    ".model-marketplace-filter-option:hover:not(.model-marketplace-filter-option--selected)",
    "border-color",
    /^transparent$/,
  ) &&
    hasDeclaration(
      ".model-marketplace-filter-option:focus-visible:not(.model-marketplace-filter-option--selected)",
      "background",
      /var\(--model-market-soft\)/,
    ) &&
    hasDeclaration(".model-marketplace-filter-option--selected", "border-color", /^var\(--model-market-solid\)$/) &&
    hasDeclaration(".model-marketplace-filter-option--selected", "background", /var\(--model-market-solid\)/) &&
    hasDeclaration(".model-marketplace-filter-option--selected", "color", /^var\(--model-market-solid-fg\)$/),
  "ordinary filter options stay lightweight while the selected option remains themed",
);
ok(
  avoidsExpensiveEffects([
    ".model-marketplace-filter",
    ".model-marketplace-filter__chevron",
    ".model-marketplace-filter:hover",
    ".model-marketplace-filter:focus-visible",
    ".model-marketplace-filter--open",
    ".model-marketplace-filter-menu",
    ".model-marketplace-filter-option",
    ".model-marketplace-filter-option:hover:not(.model-marketplace-filter-option--selected)",
    ".model-marketplace-filter-option:focus-visible:not(.model-marketplace-filter-option--selected)",
    ".model-marketplace-filter-option--selected",
  ]) &&
    finalDeclaration(
      '.anchored-popover.model-marketplace-filter-menu[data-ready="true"][data-state="open"]',
      "animation",
    ) === "none" &&
    finalDeclaration(
      '.anchored-popover.model-marketplace-filter-menu[data-state="closing"]',
      "animation",
    ) === "none",
  "filter controls avoid transforms, filters, shadows, opacity effects, and animated transitions",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
