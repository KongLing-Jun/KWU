import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function ok(value: unknown, message: string) {
  if (!value) throw new Error(message);
}

const here = dirname(fileURLToPath(import.meta.url));
const source = (path: string) => readFileSync(resolve(here, path), "utf8");
const appSource = source("../App.tsx");
const themeOverrides = source("../styles/theme-overrides.css");
const features = source("../styles/features.css");
const capabilitiesSource = source("../components/CapabilitiesPanel.tsx");

ok(
  appSource.includes("capability-workspace-shell capability-workspace-shell--${workspaceView}"),
  "capability workspace exposes its active view before lazy content mounts",
);
ok(
  themeOverrides.includes(".capability-workspace-shell--skills .capability-workspace-header"),
  "skill workspace sizing uses the stable outer view class",
);
ok(
  !themeOverrides.includes(".capability-workspace-shell:has(.skill-library)"),
  "skill workspace sizing does not depend on lazy child content",
);
ok(
  themeOverrides.includes(".capability-workspace-shell--experts .capability-workspace-header"),
  "expert workspace sizing uses the stable outer view class",
);
ok(
  !themeOverrides.includes(".capability-workspace-shell:has(.capability-workspace-shell__content > .expert-page)"),
  "expert workspace sizing does not depend on lazy child content",
);
for (const view of ["experts", "skills", "connectors"]) {
  ok(
    themeOverrides.includes(`.capability-workspace-shell--${view} .capability-workspace-header`),
    `${view} workspace has an outer route class for its header geometry`,
  );
}
ok(
  !themeOverrides.includes(".capability-workspace-shell:has(.primary-workspace--connectors)"),
  "connector workspace sizing does not depend on its lazy child content",
);
ok(
  appSource.includes('<div key={workspaceView} className="capability-workspace-shell__content">'),
  "capability workspace content is keyed by its active route",
);
ok(
  appSource.includes('className="capability-workspace-loading"') &&
    appSource.includes('data-capability-view={view}') &&
    appSource.includes('role="status"'),
  "capability loading identifies the target route with an accessible status",
);
ok(
  features.includes(".capability-workspace-shell__topbar") &&
    /\.capability-workspace-shell__topbar\s*\{[^}]*display:\s*flex;[^}]*flex:\s*0 0 52px;/.test(features) &&
    /@media screen and \(max-width: 1200px\)[\s\S]*?\.capability-workspace-shell__topbar\s*\{[^}]*flex-basis:\s*104px;[^}]*flex-direction:\s*column;/.test(features) &&
    /\.capability-workspace-shell__toolbar\s*\{[^}]*min-height:\s*52px;/.test(features),
  "capability workspace keeps one wide topbar row and reserves a fixed narrow operation band",
);
ok(
  /<div className="capability-workspace-shell__topbar">\s*<header className="capability-workspace-header">[\s\S]*?<\/header>\s*<div className="capability-workspace-shell__toolbar">\s*<div id="capability-workspace-header-actions"/.test(appSource),
  "capability actions host uses a shared topbar and stays outside the navigation header",
);
ok(
  capabilitiesSource.includes("workspace ? (headerActionsHost ? createPortal(connectorTopbar, headerActionsHost) : null) : null") &&
    capabilitiesSource.includes("workspace ? (headerActionsHost ? createPortal(topbar, headerActionsHost) : null) : topbar"),
  "workspace capability actions wait for the portal host instead of rendering in the content flow",
);
