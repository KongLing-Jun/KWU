import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

function cssBlock(source: string, selector: string): string {
  const start = source.indexOf(selector);
  assert.notEqual(start, -1, `missing CSS selector ${selector}`);
  const open = source.indexOf("{", start);
  assert.notEqual(open, -1, `missing opening brace for ${selector}`);
  let depth = 0;
  for (let index = open; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] !== "}") continue;
    depth -= 1;
    if (depth === 0) return source.slice(open + 1, index);
  }
  assert.fail(`missing closing brace for ${selector}`);
}

const settingsCSS = readFileSync(resolve("src/styles/settings.css"), "utf8");
const themeOverridesCSS = readFileSync(resolve("src/styles/theme-overrides.css"), "utf8");

console.log("settings modal avoids full-surface scale animation");
for (const name of ["@keyframes settings-modal-in", "@keyframes settings-modal-out"]) {
  const keyframes = cssBlock(settingsCSS, name);
  assert.doesNotMatch(keyframes, /\bscale\s*\(/);
}

console.log("creation settings surfaces avoid backdrop blur");
for (const selector of [
  ".app--creation .settings-modal-backdrop,",
  ".app--creation .settings-modal,",
]) {
  const block = cssBlock(themeOverridesCSS, selector);
  assert.match(block, /backdrop-filter:\s*none/);
  assert.doesNotMatch(block, /backdrop-filter:\s*blur\s*\(/);
}

console.log("general settings preserve renderer-style alignment and focus treatment");
const navigationItem = cssBlock(themeOverridesCSS, ".app--creation .settings-center__navitem,");
assert.match(navigationItem, /border-radius:\s*4px/);

const generalAction = cssBlock(
  themeOverridesCSS,
  ".app--creation .settings-page--general .settings-field:not(.settings-field--stacked) .settings-field__control,",
);
assert.match(generalAction, /min-width:\s*128px/);

const segmentFocus = cssBlock(
  themeOverridesCSS,
  ".app--creation .settings-center .set-seg:focus-within,",
);
assert.match(segmentFocus, /border-color:\s*color-mix/);
assert.match(segmentFocus, /box-shadow:\s*0 0 0 2px/);
