import { readFileSync } from "node:fs";
import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/project-directory-rendering-contract.test.ts

const projectStyles = readFileSync(
  new URL("../features/projects/projects.css", import.meta.url),
  "utf8",
);
const styles = `${loadStylesSource()}\n${projectStyles}`.replace(/\/\*[\s\S]*?\*\//g, "");
const cloudProjectsSource = readFileSync(
  new URL("../features/projects/ui/CloudProjectsSectionImpl.tsx", import.meta.url),
  "utf8",
);
const workItemSource = readFileSync(
  new URL("../features/projects/ui/WorkItemBoard.tsx", import.meta.url),
  "utf8",
);
const instructionDialogSource = readFileSync(
  new URL("../features/projects/ui/ProjectInstructionDialog.tsx", import.meta.url),
  "utf8",
);
const capabilityDialogSource = readFileSync(
  new URL("../features/projects/ui/ProjectCapabilityDialog.tsx", import.meta.url),
  "utf8",
);

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

function blockText(selector: string): string {
  return matchingBlocks(selector).join("\n");
}

console.log("\nproject directory rendering contract");

for (const selector of [".project-directory__card", ".project-directory__starter"]) {
  ok(finalDeclaration(selector, "transition") === "none", `${selector} does not animate paint or compositing properties`);
}

for (const selector of [".project-directory__card:hover:not(:disabled)", ".project-directory__starter:hover"]) {
  const hover = blockText(selector);
  ok(/(?:^|;)\s*border-color\s*:/.test(hover), `${selector} keeps a lightweight border cue`);
  ok(
    !/(?:^|;)\s*(?:background|background-color|transform|box-shadow|filter|opacity|transition)\s*:/.test(hover),
    `${selector} avoids large-area hover repaint and compositor effects`,
  );
}

for (const selector of [".project-directory__card:focus-visible", ".project-directory__starter:focus-visible"]) {
  ok(finalDeclaration(selector, "outline")?.startsWith("3px solid") === true, `${selector} preserves keyboard focus without a shadow`);
  ok(finalDeclaration(selector, "box-shadow") === undefined, `${selector} does not use a focus shadow`);
}

ok(
  finalDeclaration(".project-directory__card--active", "background")?.includes("var(--bg-elev)") === true,
  "the current project keeps its static selected background",
);

ok(
  /\{dialog\s*&&\s*authenticated\s*&&\s*state\.ownerScope\s*===\s*accountScopeKey\s*\?\s*\(/.test(cloudProjectsSource),
  "a project dialog is gated by authentication and the real state owner scope",
);

ok(
  /className="cloud-projects__directory-tools"[\s\S]*?cloud-projects__search[\s\S]*?cloud-projects__quiet-action/.test(cloudProjectsSource),
  "directory search and refresh are one grouped control surface",
);
ok(
  /setCapabilityBindingsStatus\("error"\)/.test(cloudProjectsSource)
    && /capabilityBindingsStatus === "ready"/.test(cloudProjectsSource)
    && /项目能力加载失败/.test(cloudProjectsSource),
  "project Composer fails closed when authoritative capability bindings cannot be loaded",
);
ok(
  /className="cloud-projects__card-copy"[\s\S]*?cloud-projects__card-meta/.test(cloudProjectsSource),
  "project-card title and metadata have explicit semantic layout hooks",
);
ok(
  /min-height\s*:\s*clamp\(190px,\s*21vw,\s*244px\)/.test(blockText(".cloud-projects__hero")),
  "wide directory hero keeps the project list in the first viewport",
);
ok(
  finalDeclaration(".cloud-projects", "container-type") === "inline-size"
    && !/\.cloud-projects__hero img\s*\{[^}]*width\s*:\s*min\(50vw/s.test(styles)
    && /@container\s*\(max-width:\s*760px\)[\s\S]*?\.cloud-projects__hero\s*\{/.test(projectStyles),
  "project hero responds to its content area instead of overflowing from viewport-sized artwork",
);
ok(
  /display\s*:\s*flex/.test(blockText(".cloud-projects__card-copy"))
    && /flex-direction\s*:\s*column/.test(blockText(".cloud-projects__card-copy")),
  "project-card copy stacks title and metadata instead of joining their baselines",
);
ok(
  /repeat\(2,\s*minmax\(0,\s*1fr\)\)/.test(blockText(".cloud-projects__list")),
  "wide project cards match the two-column reference composition",
);
ok(
  /min-height\s*:\s*clamp\(180px,\s*32vh,\s*300px\)/.test(blockText(".cloud-projects__unavailable")),
  "wide unavailable states use an intentional centered composition",
);
ok(
  /repeat\(4,\s*max\(260px,\s*calc\(\(100%\s*-\s*10px\)\s*\/\s*2\)\)\)/.test(blockText(".work-item-board__columns")),
  "plan viewport ends after two complete wide columns",
);
ok(
  finalDeclaration(".work-item-board__board-scroller", "padding-bottom") === "calc(var(--scrollbar-size) + 4px)",
  "plan board reserves the horizontal scrollbar height below its final cards",
);
ok(
  finalDeclaration(".work-item-board .work-item-board__columns", "height") === "100%"
    && finalDeclaration(".work-item-board .work-item-board__columns", "min-height") === "0"
    && finalDeclaration(".work-item-board .work-item-board__column ol", "overflow-y") === "auto",
  "short plan viewports shrink the columns and keep their card lists vertically scrollable",
);
ok(
  !/\.cloud-projects__workspace-center\s*\{[^}]*min-height\s*:\s*520px/s.test(styles),
  "narrow workspace does not reserve a fixed 520px blank center",
);
ok(
  /@media\s*\(max-width:\s*680px\)[\s\S]*?\.cloud-projects__hero\s*\{[^}]*padding\s*:\s*14px\s+0\s+24px\s+40px/s.test(styles),
  "narrow directory reserves the shell-toggle safe area",
);

ok(
  /repeat\(2,\s*minmax\(0,\s*1fr\)\)/.test(blockText(".project-directory__starter-grid")),
  "Goal 11 wide template collection uses two equivalent columns",
);
ok(
  /grid-template-columns\s*:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/.test(blockText(".app--creation .primary-workspace--projects :is(.project-directory__starter-grid)")),
  "Goal 11 theme override preserves the two-column template hierarchy",
);
ok(
  finalDeclaration(".cloud-projects__primary-action", "background") === "var(--fg)",
  "Goal 11 create action is the solid visual primary",
);
ok(
  /assets\/projects\/project-team-collaboration\.png/.test(cloudProjectsSource),
  "the hero uses the supplied project collaboration artwork",
);
ok(
  finalDeclaration(".cloud-projects__hero img", "aspect-ratio") === "629 / 210"
    && finalDeclaration(".cloud-projects__hero img", "object-fit") === "contain"
    && finalDeclaration(".cloud-projects__hero img", "border-radius") === "0",
  "project hero preserves the supplied wide illustration without circular cropping",
);
ok(
  /function ProjectCardMenu/.test(cloudProjectsSource)
    && /aria-haspopup="menu"/.test(cloudProjectsSource)
    && /role="menu"/.test(cloudProjectsSource)
    && /event\.key === "Escape"/.test(cloudProjectsSource)
    && /event\.key === "ArrowDown"/.test(cloudProjectsSource),
  "project cards expose an accessible action menu with Escape and arrow-key navigation",
);
ok(
  /className="cloud-projects__card-menu-action"/.test(cloudProjectsSource)
    && /className="cloud-projects__card-menu-action cloud-projects__card-menu-action--danger"/.test(cloudProjectsSource),
  "project-card rename and destructive delete actions have distinct hooks",
);
ok(
  /const selected = actionProject \|\| visible\.selected/.test(cloudProjectsSource)
    && /acceptResult\(ownerScope, result, selectedProjectRef\.current === selected\.projectId\)/.test(cloudProjectsSource),
  "a card rename targets that card without implicitly entering its workspace",
);
ok(
  /if \(selectedProjectRef\.current === deletedProjectId\) leaveWorkspace\(\)/.test(cloudProjectsSource),
  "deleting a directory card leaves a workspace only when that project is actually open",
);
ok(
  finalDeclaration('.cloud-projects__composer[data-project-composer-reused="true"]', "border-top") === "0"
    && finalDeclaration('.cloud-projects__composer[data-project-composer-reused="true"]', "background") === "var(--chat-bg)",
  "project composer has a contained project-only footer instead of a clipped full-width seam",
);
ok(
  finalDeclaration(".project-composer-contextbar__approval-shell", "margin-left") === "auto",
  "project permission selector aligns to the far right of the composer context row",
);
ok(
  finalDeclaration('.cloud-projects__composer[data-project-composer-reused="false"] > div > span', "color") === "var(--fg-dim)"
    && finalDeclaration(".cloud-projects__composer span", "color") === undefined
    && finalDeclaration(":root[data-theme-style] .composer__btn--send:disabled", "color") === "var(--control-disabled-fg)",
  "project fallback copy cannot override the reusable Composer disabled icon color",
);
ok(
  finalDeclaration(".project-select-menu__list", "max-height") === "min(280px, 46vh)"
    && finalDeclaration(".project-select-menu__list", "overflow-y") === "auto"
    && finalDeclaration(".project-select-menu__list", "overscroll-behavior") === "contain",
  "long project selectors stay inside the viewport and scroll independently",
);
ok(
  finalDeclaration(".project-composer-contextbar .project-select-menu--attached .project-select-menu__list", "max-height") === "none"
    && finalDeclaration(".project-composer-contextbar .project-select-menu--attached .project-select-menu__list", "overflow") === "visible",
  "the attached project tray keeps its existing parent-owned scrolling layout",
);
ok(
  finalDeclaration(".work-item-board__editor-assets", "overflow") === "hidden"
    && finalDeclaration(".work-item-board__editor-assets .project-activity__asset-picker", "border") === "0"
    && finalDeclaration('.project-activity__asset-picker label input[type="checkbox"]', "width") === "16px"
    && finalDeclaration(".project-asset-selection__items", "display") === "grid",
  "work-item attachments stay inside a compact picker without stretched checkboxes",
);
ok(
  finalDeclaration(".project-capability-icon__image", "width") === "100%"
    && finalDeclaration(".project-capability-icon__image", "height") === "100%"
    && finalDeclaration(".project-capability-icon__image", "object-fit") === "cover",
  "project capability images stay inside their icon container",
);
ok(
  finalDeclaration(".project-capability-dialog__grid > button > span:last-child", "min-width") === "0"
    && finalDeclaration(".project-capability-dialog__identity", "display") === "grid"
    && finalDeclaration(".project-capability-dialog__identity", "grid-template-columns") === "24px minmax(0, 1fr) max-content"
    && finalDeclaration(".project-capability-dialog__identity small", "white-space") === "nowrap",
  "project capability card titles truncate without squeezing install status onto multiple lines",
);
ok(
  finalDeclaration(".cloud-project-dialog__capability > header", "pointer-events") === "none",
  "new-project capability headings do not intercept the overlaid add button",
);
ok(
  finalDeclaration(".project-capability-dialog", "width") === "min(960px, calc(100vw - 48px))"
    && finalDeclaration(".project-capability-dialog", "height") === "min(680px, calc(100vh - 48px))"
    && /width:\s*calc\(100vw - 24px\)/.test(blockText(".project-modal-surface--maximized"))
    && /height:\s*calc\(100vh - 24px\)/.test(blockText(".project-modal-surface--maximized")),
  "project capability dialogs have visibly distinct restored and maximized sizes",
);
ok(
  /<ProjectModalSurface[\s\S]*?showClose/.test(instructionDialogSource)
    && /<ProjectModalSurface[\s\S]*?showClose/.test(capabilityDialogSource)
    && /connector:[\s\S]*?expert:[\s\S]*?skill:/.test(capabilityDialogSource),
  "project instructions and all three capability kinds share one resize and close control surface",
);
ok(
  !/data-project-onboarding-guide/.test(cloudProjectsSource) && /claimCloudProjectOnboarding/.test(cloudProjectsSource),
  "the server-owned onboarding project uses the normal project workspace",
);
ok(
  /data-project-activity-shell/.test(cloudProjectsSource) && /data-project-assets-shell/.test(cloudProjectsSource),
  "Goal 11 Activity and Assets render explicit structural unavailable shells",
);
ok(
  /data-project-member-drawer/.test(cloudProjectsSource) && /currentAccountOwner/.test(cloudProjectsSource),
  "Goal 11 member drawer exposes only the current-account owner relationship",
);
ok(
  !/owner_user_id|cloud_user_id/.test(cloudProjectsSource),
  "Goal 11 member UI source contains no opaque owner or cloud-user field",
);
ok(
  /gateway\.listCloudProjectWorkItems/.test(workItemSource)
    && /assigneeUserId/.test(workItemSource)
    && /labelId/.test(workItemSource)
    && /totalCount/.test(workItemSource),
  "Goal 11 Plan filtering and complete counts come from the Cloud query",
);
ok(
  /className="work-item-board__plan-header"/.test(workItemSource)
    && /role="toolbar"/.test(workItemSource)
    && /data-plan-board/.test(workItemSource),
  "Goal 11 Plan keeps the compact reference toolbar above its board",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
