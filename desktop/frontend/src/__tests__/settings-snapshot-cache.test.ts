import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import {
  cancelSettingsScopeLoads,
  invalidateSettingsDetails,
  invalidateSettingsScope,
  loadSettingsCore,
  loadSettingsDetails,
  peekSettingsCore,
  peekSettingsDetails,
  resetSettingsSnapshotCacheForTests,
  scheduleSettingsCorePrefetch,
  settingsSnapshotScopeKey,
  updateSettingsCore,
} from "../features/settings/data/settingsSnapshotCache";
import type { SettingsCoreView, SettingsModelDetailsView } from "../lib/types";

const core = (model: string) => ({ defaultModel: model } as SettingsCoreView);
const details = (name: string) => ({
  providers: [{ name }],
  officialProviders: [],
  providerKinds: [],
} as unknown as SettingsModelDetailsView);

resetSettingsSnapshotCacheForTests();
console.log("settings snapshot cache deduplicates one scope");
let resolveCore: ((value: SettingsCoreView) => void) | undefined;
const pendingCore = new Promise<SettingsCoreView>((resolve) => {
  resolveCore = resolve;
});
let coreLoads = 0;
const loader = () => {
  coreLoads += 1;
  return pendingCore;
};
const first = loadSettingsCore("profile-a", loader);
const second = loadSettingsCore("profile-a", loader);
assert.equal(coreLoads, 1);
resolveCore?.(core("model-a"));
assert.equal((await first).defaultModel, "model-a");
assert.equal((await second).defaultModel, "model-a");
assert.equal(peekSettingsCore("profile-a")?.defaultModel, "model-a");

console.log("invalidated late response cannot repopulate a scope");
let resolveStale: ((value: SettingsCoreView) => void) | undefined;
const stale = loadSettingsCore("profile-b", () => new Promise((resolve) => {
  resolveStale = resolve;
}), true);
invalidateSettingsScope("profile-b");
resolveStale?.(core("stale-model"));
await stale;
assert.equal(peekSettingsCore("profile-b"), null);
updateSettingsCore("profile-b", core("fresh-model"));
assert.equal(peekSettingsCore("profile-b")?.defaultModel, "fresh-model");

console.log("details invalidate independently from the core snapshot");
await loadSettingsDetails("profile-b", async () => details("provider-a"));
assert.equal(peekSettingsDetails("profile-b")?.providers[0]?.name, "provider-a");
invalidateSettingsDetails("profile-b");
assert.equal(peekSettingsDetails("profile-b"), null);

console.log("idle settings prefetch warms both data and the lazy feature chunk");
const appSource = readFileSync(resolve("src/App.tsx"), "utf8");
assert.match(appSource, /const loadSettingsFeature = \(\) => import\("\.\/features\/settings"\)/);
assert.match(appSource, /lazy\(\(\) => loadSettingsFeature\(\)/);
assert.match(appSource, /scheduleSettingsCorePrefetch[\s\S]*loadSettingsFeature\(\)[\s\S]*app\.SettingsCore\(\)/);

console.log("scope key includes the real account scope and active route");
assert.notEqual(
  settingsSnapshotScopeKey("account-a", { id: "tab-a", scope: "project", workspaceRoot: "a" }),
  settingsSnapshotScopeKey("account-a", { id: "tab-b", scope: "project", workspaceRoot: "a" }),
);
assert.notEqual(
  settingsSnapshotScopeKey("account-a", { id: "tab-a", scope: "project", workspaceRoot: "a" }),
  settingsSnapshotScopeKey("account-b", { id: "tab-a", scope: "project", workspaceRoot: "a" }),
);

console.log("cancelled idle prefetch cannot populate the old scope");
let timerHandler: (() => void) | undefined;
let idleHandler: (() => void) | undefined;
let resolvePrefetch: ((value: SettingsCoreView) => void) | undefined;
const cancelPrefetch = scheduleSettingsCorePrefetch(
  "prefetch-scope",
  () => new Promise((resolve) => {
    resolvePrefetch = resolve;
  }),
  {
    setTimeout: (handler, timeout) => {
      assert.equal(timeout, 1500);
      timerHandler = handler;
      return 1;
    },
    clearTimeout: () => {},
    requestIdleCallback: (handler, options) => {
      assert.equal(options?.timeout, 1000);
      idleHandler = handler;
      return 2;
    },
    cancelIdleCallback: () => {},
  },
);
timerHandler?.();
idleHandler?.();
cancelPrefetch();
resolvePrefetch?.(core("late-prefetch"));
await new Promise((resolve) => setTimeout(resolve, 0));
assert.equal(peekSettingsCore("prefetch-scope"), null);

console.log("cancelled panel request keeps an existing snapshot but rejects late replacement");
updateSettingsCore("panel-scope", core("existing"));
let resolvePanelLoad: ((value: SettingsCoreView) => void) | undefined;
const latePanelLoad = loadSettingsCore("panel-scope", () => new Promise((resolve) => {
  resolvePanelLoad = resolve;
}), true);
cancelSettingsScopeLoads("panel-scope");
resolvePanelLoad?.(core("late-panel"));
await latePanelLoad;
assert.equal(peekSettingsCore("panel-scope")?.defaultModel, "existing");
assert.equal(peekSettingsCore("profile-b")?.defaultModel, "fresh-model");

console.log("invalidated late details cannot restore stale provider data");
let resolveStaleDetails: ((value: SettingsModelDetailsView) => void) | undefined;
const staleDetails = loadSettingsDetails("profile-b", () => new Promise((resolve) => {
  resolveStaleDetails = resolve;
}), true);
invalidateSettingsDetails("profile-b");
resolveStaleDetails?.(details("stale-provider"));
await staleDetails;
assert.equal(peekSettingsDetails("profile-b"), null);
