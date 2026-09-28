import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/expert-directory-rendering-contract.test.ts

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
    while ((match = declaration.exec(block)) !== null) value = match[1].trim();
  }
  return value;
}

function hasDeclaration(blocks: string[], property: string, value: RegExp): boolean {
  return blocks.some((block) => new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*${value.source}`).test(block));
}

console.log("\nexpert directory rendering contract");

const expertWorkspaceHeader = matchingBlocks(".expert-page__workspace-bar");
ok(
  hasDeclaration(expertWorkspaceHeader, "min-height", /76px/) &&
    hasDeclaration(expertWorkspaceHeader, "padding", /16px 15px/) &&
    finalDeclaration(".expert-page__workspace-bar", "border-bottom") === "0" &&
    finalDeclaration(".expert-page__workspace-bar", "border-radius") === "14px 14px 0 0" &&
    finalDeclaration(".expert-page__workspace-bar", "background") === "var(--bg-elev)",
  "expert workspace header matches the compact elevated reference bar",
);

const sharedCapabilityHeader = matchingBlocks(".capability-workspace-header");
ok(
  hasDeclaration(sharedCapabilityHeader, "min-height", /76px/) &&
    hasDeclaration(sharedCapabilityHeader, "padding", /16px 15px/) &&
    finalDeclaration(".capability-workspace-header", "border-radius") === "14px 14px 0 0" &&
    finalDeclaration(".capability-workspace-header", "background") === "var(--bg-elev)",
  "all capability tabs reuse the expert reference header geometry",
);

ok(
  finalDeclaration(".expert-page__workspace-bar > .capability-workspace-nav", "min-height") === "0",
  "expert navigation does not stretch the compact reference header",
);

ok(
  finalDeclaration(".capability-workspace-nav__item--active", "border-color") === "transparent" &&
    finalDeclaration(".capability-workspace-nav__item--active", "background") === "color-mix(in srgb, var(--fg) 78%, var(--bg-elev))" &&
    finalDeclaration(".capability-workspace-nav__item--active", "color") === "var(--bg-elev)",
  "capability navigation uses the dark filled active pill from the reference",
);

const expertGrid = matchingBlocks(".expert-grid");
ok(
  hasDeclaration(expertGrid, "grid-template-columns", /repeat\(auto-fit,\s*minmax\(min\(100%,\s*288px\),\s*1fr\)\)/) &&
    finalDeclaration(".expert-grid", "gap") === "12px",
  "expert grid uses compact content-driven 4/3/2/1 columns",
);

ok(
  finalDeclaration(".expert-card-shell", "min-height") === "154px" &&
    finalDeclaration(".expert-card", "min-height") === "154px" &&
    finalDeclaration(".expert-card", "border-radius") === "16px" &&
    finalDeclaration(".expert-card", "background") === "color-mix(in srgb, var(--bg-elev) 86%, var(--chat-bg))",
  "expert cards match the compact WorkBuddy geometry and lightweight skill-card surface",
);

ok(
  finalDeclaration(".expert-card", "box-shadow") === "none" &&
    finalDeclaration(".expert-card", "transition") === "none",
  "expert cards do not keep static shadows or hover transitions",
);

ok(
  finalDeclaration(".expert-section h2", "font-size") === "20px" &&
    finalDeclaration(".expert-section h2", "font-weight") === "600" &&
    finalDeclaration(".expert-section h2", "line-height") === "28px" &&
    finalDeclaration(".expert-mode-switch--catalog .expert-mode-switch__item", "font-size") === "20px" &&
    finalDeclaration(".expert-mode-switch--catalog .expert-mode-switch__item", "font-weight") === "600" &&
    finalDeclaration(".expert-mode-switch--catalog .expert-mode-switch__item", "line-height") === "28px",
  "expert section headings and catalog tabs use the WorkBuddy type scale",
);

ok(
  finalDeclaration(".expert-card__title strong", "font-size") === "14px" &&
    finalDeclaration(".expert-card__title strong", "font-weight") === "600" &&
    finalDeclaration(".expert-card__title strong", "line-height") === "20px" &&
    finalDeclaration(".expert-card__title small", "font-size") === "11px" &&
    finalDeclaration(".expert-card__title small", "font-weight") === "400" &&
    finalDeclaration(".expert-card__title small", "line-height") === "16px" &&
    finalDeclaration(".expert-card__title small", "color") === "var(--fg)" &&
    matchingBlocks(".expert-card small").length === 0,
  "expert card identity typography matches WorkBuddy without a generic small-text override",
);

ok(
  finalDeclaration(".expert-card p", "color") === "color-mix(in srgb, var(--fg) 78%, transparent)" &&
    finalDeclaration(".expert-tags span", "color") === "color-mix(in srgb, var(--fg) 78%, transparent)",
  "expert card descriptions and tags use the stronger WorkBuddy secondary text level",
);

ok(
  finalDeclaration(':root[data-platform="windows"] .expert-page', "font-family") ===
    '"Segoe WPC", "Segoe UI", "Microsoft YaHei", sans-serif' &&
    finalDeclaration(".expert-scene-person", "font-size") === "14px" &&
    finalDeclaration(".expert-scene-person", "font-weight") === "600" &&
    finalDeclaration(".expert-scene-person", "line-height") === "20px" &&
    finalDeclaration(".expert-scene-person", "color") === "var(--fg)",
  "Windows expert typography and featured expert rows use the approved title weight",
);

const expertHover = matchingBlocks(".expert-card:hover");
ok(
  finalDeclaration(".expert-card:hover", "background") === undefined &&
    finalDeclaration(".expert-card:hover", "border-color") === "var(--accent)" &&
    finalDeclaration(".expert-card:focus-visible", "border-color") === "var(--accent)",
  "expert card hover and keyboard focus change only to the active theme border",
);
ok(
  expertHover.every((block) => !/(?:^|;)\s*(?:box-shadow|transform|filter|opacity|transition)\s*:/.test(block)),
  "expert card hover avoids compositor-heavy properties",
);
ok(
  finalDeclaration(".expert-card:active", "transform") === "none" &&
    finalDeclaration(".expert-card:active", "box-shadow") === "none",
  "expert card press feedback does not move or scale the card",
);
ok(
  finalDeclaration(".expert-card:focus-visible", "outline")?.startsWith("2px solid") === true,
  "expert cards keep a visible keyboard focus cue",
);

ok(
  finalDeclaration(".expert-scenes", "display") === "flex" &&
    finalDeclaration(".expert-scene-card", "flex") === "1 0 246px" &&
    finalDeclaration(".expert-scene-card", "min-width") === "246px" &&
    finalDeclaration(".expert-scene-card", "width") === "auto" &&
    finalDeclaration(".expert-scene-card", "height") === "224px",
  "featured scenes fill wide rows while preserving the compact minimum width",
);
ok(matchingBlocks(".expert-scene-card:hover").length === 0, "featured scene outer cards have no hover repaint rule");
ok(finalDeclaration(".expert-scene-card__background", "filter") === undefined, "featured scene images avoid CSS filters");
ok(
  finalDeclaration(".expert-scene-person:hover", "background")?.includes("var(--fg)") === true,
  "featured scene expert rows keep their small-area neutral hover feedback",
);

const summonButton = matchingBlocks(".expert-card__summon");
ok(
  finalDeclaration(".expert-card__summon", "transition") === "none" &&
    summonButton.every((block) => !/(?:^|;)\s*opacity\s*:/.test(block)),
  "summon button visibility does not animate opacity",
);
ok(
  /@media\s*\(hover:\s*hover\)\s*and\s*\(pointer:\s*fine\)[\s\S]*?\.expert-card__summon\s*\{\s*visibility:\s*hidden;[\s\S]*?\.expert-card-shell:focus-within\s+\.expert-card__summon[\s\S]*?visibility:\s*visible;/.test(styles),
  "fine pointers reveal summon buttons on hover or keyboard focus",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
