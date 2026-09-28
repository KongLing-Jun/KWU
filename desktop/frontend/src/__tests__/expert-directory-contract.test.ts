import { loadStylesSource } from "./helpers/styles";
// Run: tsx src/__tests__/expert-directory-contract.test.ts
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testDir = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(testDir, "../features/experts/ui/ExpertDirectory.tsx"), "utf8");
const styles = loadStylesSource().replace(/\/\*[\s\S]*?\*\//g, "");

function ok(condition: unknown, message: string): void {
  if (!condition) throw new Error(message);
}

function blockFor(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`).exec(styles);
  return match?.[1] ?? "";
}

function blocksForSelector(selector: string): string[] {
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
  for (const block of blocksForSelector(selector)) {
    const declaration = new RegExp(`(?:^|;)\\s*${property}\\s*:\\s*([^;]+)`, "g");
    let match: RegExpExecArray | null;
    while ((match = declaration.exec(block)) !== null) value = match[1].trim();
  }
  return value;
}

const expertGrid = blockFor(".expert-grid");
ok(
  /grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*288px\),\s*1fr\)\)/.test(expertGrid) &&
    /gap:\s*12px/.test(expertGrid),
  "expert catalog grid must use the compact WorkBuddy width and content-driven columns",
);
const expertScenes = blockFor(".expert-scenes");
ok(
  /display:\s*flex/.test(expertScenes) &&
    /overflow-x:\s*auto/.test(expertScenes),
  "featured expert scenes must stay in a compact horizontal row like the WorkBuddy reference",
);
ok(
  source.includes('className="expert-page__workspace-bar"') &&
    source.includes("workspaceNavigation") &&
    source.includes("workspaceHeaderActionsId") &&
    source.includes("createPortal(catalogHeaderActions, workspaceHeaderActionsHost)") &&
    source.includes('name="experts-search"') &&
    source.includes('autoComplete="off"'),
  "expert catalog portals its real search controls into the persistent shared workspace header",
);
ok(
  !/repeat\(4,\s*minmax\(220px,\s*1fr\)\)/.test(expertGrid),
  "expert catalog grid must not regress to fixed four-column cards",
);
const mediumBreakpointStart = styles.lastIndexOf("@media (max-width: 1180px)");
const compactBreakpointStart = styles.lastIndexOf("@media (max-width: 760px)");
const mediumBreakpoint = mediumBreakpointStart >= 0 && compactBreakpointStart > mediumBreakpointStart
  ? styles.slice(mediumBreakpointStart, compactBreakpointStart)
  : "";
ok(
  mediumBreakpoint.length > 0 && !/\.expert-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,/.test(mediumBreakpoint),
  "expert catalog grid must not use viewport-only two-column overrides",
);

const expertPage = blockFor(".expert-page");
const expertPageContent = blockFor(".expert-page__content");
ok(
  !/container-type:\s*inline-size/.test(expertPage) && /container-type:\s*inline-size/.test(expertPageContent),
  "expert page content must expose an inline-size container without containing modal overlays",
);
ok(
  source.includes('<div className="expert-page__content">'),
  "expert page must keep responsive content in a wrapper so modals stay outside the container",
);
const catalogNavIndex = source.indexOf('className="expert-catalog-nav"');
const scenesIndex = source.indexOf('className="expert-section" aria-labelledby="expert-scenes-title"');
const catalogSectionIndex = source.indexOf('className="expert-section expert-section--catalog"');
ok(
  scenesIndex >= 0 && catalogNavIndex > scenesIndex && catalogSectionIndex > catalogNavIndex &&
    source.includes('className="expert-mode-switch expert-mode-switch--catalog"'),
  "featured scenes must lead into centered expert navigation before catalog cards",
);
const catalogModeSwitch = blockFor(".expert-mode-switch.expert-mode-switch--catalog");
const catalogModeItem = blockFor(".expert-mode-switch--catalog .expert-mode-switch__item");
const expertCategory = blockFor(".expert-catalog-nav .expert-category");
const activeExpertCategory = blockFor(".expert-catalog-nav .expert-category--active");
const catalogPrimary = blockFor(".expert-catalog-nav__primary");
const expertSortSwitch = blockFor(".expert-sort-switch");
const expertSortItem = blockFor(".expert-sort-switch__item");
const activeExpertSortItem = blockFor(".expert-sort-switch__item--active");
ok(
  /gap:\s*26px/.test(catalogModeSwitch) &&
    /font-size:\s*20px/.test(catalogModeItem) &&
    /font-size:\s*15px/.test(expertCategory) &&
    /background:\s*color-mix\(in srgb, var\(--fg\) 6%, var\(--bg-elev\)\)/.test(activeExpertCategory),
  "expert catalog navigation must match the neutral two-row reference hierarchy",
);
ok(
  source.includes('type ExpertSort = "hot" | "latest"') &&
    source.includes('expertLibraryState?.usageCounts') &&
    source.includes('(right.createdAt ?? 0) - (left.createdAt ?? 0)') &&
    source.includes('(usageCounts[right.id] ?? 0) - (usageCounts[left.id] ?? 0)'),
  "expert catalog must sort latest by creation time and hot by persisted usage count",
);
ok(
  /justify-content:\s*space-between/.test(catalogPrimary) &&
    /padding:\s*3px/.test(expertSortSwitch) &&
    /height:\s*28px/.test(expertSortItem) &&
    /background:\s*var\(--bg-elev\)/.test(activeExpertSortItem),
  "hot and latest controls must match the compact segmented reference on the right side",
);
ok(expertPage.includes("background: var(--chat-bg)"), "expert workspace uses the active appearance canvas token");
ok(styles.includes('color-mix(in srgb, var(--expert-tone) 52%, var(--accent))'), "expert category accents blend with the selected appearance color");
ok(
  styles.includes(".expert-page__workspace-bar") &&
    styles.includes(".expert-page__workspace-bar .expert-page__actions"),
  "expert catalog must keep navigation and real search actions in one shared workspace bar",
);

const requiredAvatarBindings = [
  "architectAvatar",
  "codeReviewAvatar",
  "securityReviewAvatar",
  "automationFlowAvatar",
  "dataReportAvatar",
  "docReportAvatar",
  "researchAvatar",
  "opsGrowthAvatar",
  "codeDeliveryTeamAvatar",
  "officeAutomationTeamAvatar",
  "dataAnalysisTeamAvatar",
  "customerOpsTeamAvatar",
  "projectDeliveryTeamAvatar",
];

for (const binding of requiredAvatarBindings) {
  ok(source.includes(`avatarSrc: ${binding}`), `built-in expert card must bind PNG avatar ${binding}`);
}

ok(
  /src=\{item\.avatarSrc\}/.test(source),
  "expert catalog cards must pass avatarSrc into ExpertAvatar",
);
ok(
  /<img className="expert-avatar__image" src=\{src\}/.test(source),
  "ExpertAvatar must render supplied PNG avatars as images",
);
ok(
  source.includes("avatarSrc: expertMemberAvatarSrc({ ...member, avatarKey })"),
  "backend expert team members must resolve remote or static avatars before render",
);
ok(
  source.includes("src={member.avatarSrc || uploadedAvatarSrc(member.avatarKey) || AVATAR_BY_EXPERT_NAME[member.name]}"),
  "expert team member rows must render resolved member PNG avatars",
);

ok(
  source.includes('className="expert-card" onClick={() => setSelected(item)}') &&
  !source.includes('className="expert-card" onClick={() => summon(item)}'),
  "expert and expert-team catalog cards must open details instead of summoning immediately",
);
ok(
  source.includes('className="expert-card__summon"') &&
    source.includes('onClick={() => summon(item)}') &&
    !source.includes('className="expert-card__details"'),
  "catalog cards must expose the reference-style summon action while preserving card detail navigation",
);
ok(
  source.includes("selected.prompts.slice(0, MAX_EXPERT_PROMPTS).map") &&
  source.includes("onClick={() => summon(selected, prompt)}"),
  "expert details must render preset questions that summon with the clicked prompt",
);
ok(
  source.includes('{item.source === "user" && (') &&
    source.includes('if (item.source !== "user" || isCloudUserAssetID(item.id)) return null;'),
  "edit, duplicate, and delete controls must be gated to user-owned experts",
);

const manageActions = blockFor(".expert-manage-card__actions");
const manageActionButton = blockFor(".expert-manage-card__actions button");
ok(
  /flex-wrap:\s*wrap/.test(manageActions) && /white-space:\s*nowrap/.test(manageActionButton),
  "narrow expert card actions must wrap as whole buttons without vertical Chinese text",
);

ok(
  /\.expert-card\s*\{[^}]*cursor:\s*pointer/.test(styles) &&
    finalDeclaration(".expert-scene-card", "transition") === "none" &&
    finalDeclaration(".expert-card", "transition") === "none" &&
    finalDeclaration(".expert-card-shell", "min-height") === "154px" &&
    finalDeclaration(".expert-card", "min-height") === "154px" &&
    finalDeclaration(".expert-card", "border-radius") === "16px" &&
    finalDeclaration(".expert-card", "background") === "color-mix(in srgb, var(--bg-elev) 86%, var(--chat-bg))" &&
    finalDeclaration(".expert-scene-person:hover", "background")?.includes("var(--fg)") === true &&
    finalDeclaration(".expert-card__summon", "min-width") === "46px" &&
    finalDeclaration(".expert-card__summon", "transition") === "none",
  "expert scene and catalog cards must use the compact low-compositing reference while retaining inner feedback",
);
