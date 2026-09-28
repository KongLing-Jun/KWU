// Run: tsx src/__tests__/im-session-refresh-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

let passed = 0;
let failed = 0;

function ok(cond: boolean, label: string) {
  if (cond) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

const here = dirname(fileURLToPath(import.meta.url));
const appSource = readFileSync(resolve(here, "../App.tsx"), "utf8");
const bridgeSource = readFileSync(resolve(here, "../lib/bridge.ts"), "utf8");
const controllerSource = readFileSync(resolve(here, "../features/session/useController.ts"), "utf8");

console.log("\nIM session refresh contract");

ok(
  bridgeSource.includes("ProjectTreeRevision(): Promise<number>") &&
    bridgeSource.includes("onProjectTreeChanged(cb: (revision?: number)") &&
    bridgeSource.includes("EventsOn(\"project-tree:changed\"") &&
    bridgeSource.includes("ProjectTreeRevision()"),
  "bridge exposes project tree revision and passes revision payloads from project-tree changed events",
);

ok(
  appSource.includes("lastSeenProjectTreeRevisionRef") &&
    appSource.includes("refreshInFlightRef") &&
    appSource.includes("refreshQueuedRef") &&
    appSource.includes("refreshSessionSurfaces"),
  "App coalesces session surface refreshes behind one refresh function",
);

ok(
  appSource.includes("const refreshSessionSurfaces = useCallback") &&
    appSource.includes("refreshTabMetas()") &&
    appSource.includes("refreshHistoryView()") &&
    appSource.includes("reloadSidebarImConnections()") &&
    appSource.includes("channelSessionRefreshTarget(activeTabRef.current)") &&
    appSource.includes("revalidateChannelSession(channelTarget.tabId, channelTarget.sessionPath)"),
  "App refreshes tabs, history, IM sidebar, and the current read-only channel transcript together",
);

ok(
  appSource.includes("activeTabRef.current = activeTab") &&
    appSource.includes("const channelTarget = channelSessionRefreshTarget(activeTabRef.current)") &&
    !appSource.includes("[activeTab, refreshHistoryView"),
  "each queued refresh loop reads the latest active tab instead of a captured render",
);

ok(
  controllerSource.includes("const revalidateChannelSession = useCallback") &&
    controllerSource.includes("app.PreviewSession(sessionPath)") &&
    controllerSource.includes("revalidateChannelSession = useCallback(async (tabId: string, path: string)") &&
    controllerSource.includes("historyRevalidationRequestIsCurrent") &&
    controllerSource.includes('type: "history_revalidated"'),
  "channel transcript revalidation reads disk history and publishes only a guarded changed snapshot",
);

ok(
  appSource.includes("app.ProjectTreeRevision()") &&
    appSource.includes("window.setInterval") &&
    appSource.includes("catchUpProjectTreeRevision"),
  "App polls lightweight project tree revision as a background fallback",
);

ok(
  appSource.includes("window.addEventListener(\"focus\"") &&
    appSource.includes("document.addEventListener(\"visibilitychange\"") &&
    appSource.includes("document.visibilityState === \"visible\"") &&
    appSource.includes("refreshSessionSurfaces"),
  "App catches up project tree surfaces on focus and visibility restore",
);

ok(
  appSource.includes("window.setInterval(() => {") &&
    appSource.includes("document.visibilityState === \"hidden\"") &&
    appSource.includes("10000") &&
    !appSource.includes("catchUpProjectTreeRevision(), 3000"),
  "App uses a conservative visible-only revision interval",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
