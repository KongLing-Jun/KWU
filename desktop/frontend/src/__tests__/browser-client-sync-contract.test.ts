import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

const here = dirname(fileURLToPath(import.meta.url));
const source = (path: string) => readFileSync(resolve(here, path), "utf8");
const viteSource = source("../../vite.config.ts");
const devSource = source("../../../../dev");
const mainSource = source("../main.tsx");
const runtimeSource = source("../lib/wailsDevRuntime.ts");
const bridgeSource = source("../lib/bridge.ts");
const appSource = source("../App.tsx");
const marketplaceSource = source("../features/model-marketplace/ui/ModelMarketplace.tsx");
const accountSidebarSource = source("../features/account/controller/useAccountSidebar.ts");
const backendSource = source("../../../app.go");
const backendTabsSource = source("../../../tabs.go");

ok(viteSource.includes('"/wails"') && viteSource.includes("ws: true"), "Vite proxies Wails HTTP and WebSocket runtime traffic");
ok(viteSource.includes("KWU_DESKTOP_WAILS_PORT"), "the Wails dev port is configurable for isolated worktrees");
ok(
  viteSource.includes("strictPort: hasConfiguredDevPort"),
  "direct Wails development can fall back from a busy default Vite port while configured worktree ports stay strict",
);
ok(
  devSource.includes('export KWU_DESKTOP_WAILS_PORT="$wails_port"'),
  "the dev launcher passes its resolved Wails port to the Vite child process",
);
ok(
  mainSource.includes("await connectWailsDevRuntime()") && mainSource.indexOf("await connectWailsDevRuntime()") < mainSource.indexOf("createRoot(root).render"),
  "the browser connects to Wails before React requests application data",
);
ok(
  runtimeSource.includes('"/wails/ipc.js"') && runtimeSource.includes('"/wails/runtime.js"') && runtimeSource.includes("import.meta.env.DEV"),
  "runtime injection is development-only and uses the proxied Wails scripts",
);
ok(
    runtimeSource.includes('if (message === "runtime:ready") return;') &&
    runtimeSource.indexOf('await loadScript("/wails/ipc.js")') <
      runtimeSource.indexOf("suppressBrowserRuntimeReady();") &&
    runtimeSource.indexOf("suppressBrowserRuntimeReady();") <
      runtimeSource.indexOf('await loadScript("/wails/runtime.js")'),
  "browser runtime suppresses the desktop-only ready handshake before loading the Wails runtime",
);
ok(runtimeSource.includes("go?.main?.App"), "runtime bootstrap waits for real Go bindings instead of accepting mock state");
ok(
  runtimeSource.includes("throw backendRequiredError") &&
    runtimeSource.includes("browser mock data is disabled") &&
    !runtimeSource.includes("browser preview data"),
  "a missing Wails backend fails closed instead of loading browser preview data",
);
ok(
  bridgeSource.includes('EventsOn("model-route:changed"') &&
    backendTabsSource.includes('emitRuntimeEvent("model-route:changed", tabID)') &&
    backendSource.includes("a.emitModelRouteChanged(tab.ID)"),
  "successful model changes are broadcast through a shared backend runtime event",
);
ok(
  appSource.includes("onModelRouteChanged") &&
    appSource.includes("refreshSubscriptionRepairFacts()") &&
    appSource.includes("modelCatalog={sharedModelCatalog}"),
  "each client invalidates its active model facts when another client changes a route",
);
ok(
  /const resolvedRoutes = routes\.status === "fulfilled" \? asArray\(routes\.value\) : undefined/.test(appSource) &&
    appSource.includes("routes: resolvedRoutes") &&
    /const sharedModelCatalog: SharedModelCatalogState = currentSubscriptionRepairFacts\.state === "loading"[\s\S]*\? \{ state: "unavailable" \}[\s\S]*state: "ready"[\s\S]*routes: currentSubscriptionRepairFacts\.routes/.test(appSource) &&
    (appSource.match(/modelCatalog=\{sharedModelCatalog\}/g) ?? []).length >= 2,
  "App maps the authoritative request to loading, unavailable, or ready and distributes one shared projection",
);
ok(
  appSource.includes("const request = ++subscriptionRepairRequestRef.current;") &&
    appSource.includes("if (subscriptionRepairRequestRef.current !== request) return;"),
  "App keeps the sequence guard that prevents stale shared-catalog requests from committing",
);
ok(
  marketplaceSource.includes("modelCatalog: SharedModelCatalogState") &&
    !marketplaceSource.includes("ModelsForTab") &&
    !marketplaceSource.includes("loadSelectableModels") &&
    !marketplaceSource.includes("refreshSignal"),
  "the model marketplace consumes App's invalidated shared catalog without its own bridge reload",
);
ok(accountSidebarSource.includes("@refresh reset"), "account sidebar hook changes force a safe Fast Refresh remount");

console.log("browser/client data sync contract passed");
