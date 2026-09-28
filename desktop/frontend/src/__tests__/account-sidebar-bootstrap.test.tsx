// Run: tsx src/__tests__/account-sidebar-bootstrap.test.tsx

import { JSDOM } from "jsdom";
import { readFileSync } from "node:fs";
import React, { StrictMode } from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { useAccountSidebar } from "../features/account/controller/useAccountSidebar";
import {
  useAccountStore,
  type CloudBootstrapView,
  type CloudCachedAssetsView,
} from "../features/account/store/accountStore";
import type { AppBindings } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

let passed = 0;
let failed = 0;

function ok(value: boolean, label: string) {
  if (value) {
    process.stdout.write(`  PASS  ${label}\n`);
    passed += 1;
  } else {
    process.stdout.write(`  FAIL  ${label}\n`);
    failed += 1;
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}

function bootstrap(name: string): CloudBootstrapView {
  return {
    available: true,
    account: {
      status: "authenticated",
      profile_id: `profile-${name}`,
      cloud_user_id: `user-${name}`,
      binding_generation: 1,
      display_name: name,
    },
    balance: { total_balance: "0" },
    subscription: {
      plans: [],
      current: { plan: "free", status: "free" },
    },
    payments: [],
  };
}

function refreshedAssets(name: string, totalBalance: string, plan: string, version: number): CloudBootstrapView {
  const view = bootstrap(name);
  view.balance = { total_balance: totalBalance, version };
  view.subscription = {
    plans: [],
    current: { plan, status: plan === "free" ? "free" : "active", version },
  };
  return view;
}

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", { url: "http://localhost/" });
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;

window.localStorage.setItem("clouvia.sidebar-avatar.local", "data:image/png;base64,local");
window.localStorage.setItem("clouvia.sidebar-avatar.user-Account A", "data:image/png;base64,account-a");
window.localStorage.setItem("clouvia.sidebar-avatar.user-Account B", "data:image/png;base64,account-b");
window.localStorage.setItem("clouvia.sidebar-avatar.user-Offline", "data:image/png;base64,offline");
window.localStorage.setItem("clouvia.sidebar-avatar.user-Expired", "data:image/png;base64,expired");

const requests: Array<ReturnType<typeof deferred<CloudBootstrapView>>> = [];
const assetRequests: Array<ReturnType<typeof deferred<CloudBootstrapView>>> = [];
const cachedAssetRequests: Array<ReturnType<typeof deferred<CloudCachedAssetsView>>> = [];
const assetListeners = new Set<(...data: unknown[]) => void>();
let assetListenerRegistrations = 0;
let assetListenerRemovals = 0;
window.runtime = {
  EventsOn: (name, callback) => {
    if (name === "cloud-assets:changed") {
      assetListenerRegistrations += 1;
      assetListeners.add(callback);
    }
    return () => {
      if (name === "cloud-assets:changed" && assetListeners.delete(callback)) {
        assetListenerRemovals += 1;
      }
    };
  },
  BrowserOpenURL: () => {},
};
window.go = {
  main: {
    App: {
      CloudBootstrap: () => {
        const request = deferred<CloudBootstrapView>();
        requests.push(request);
        return request.promise;
      },
      CloudRefreshAssets: () => {
        const request = deferred<CloudBootstrapView>();
        assetRequests.push(request);
        return request.promise;
      },
      CloudCachedAssets: () => {
        const request = deferred<CloudCachedAssetsView>();
        cachedAssetRequests.push(request);
        return request.promise;
      },
    } as Partial<AppBindings> as AppBindings,
  },
};

const avatarRenders: string[] = [];

function Harness({ contextKey }: { contextKey: string }) {
  const sidebar = useAccountSidebar(contextKey);
  avatarRenders.push(sidebar.avatarSrc);
  const label = sidebar.initialized
    ? sidebar.account?.display_name || "signed-out"
    : "loading";
  return (
    <button
      type="button"
      data-avatar={sidebar.avatarSrc}
      data-balance={sidebar.displayBalance ?? "—"}
      data-label={label}
      onClick={sidebar.toggleMenu}
    >
      {label}
    </button>
  );
}

function renderHarness(contextKey: string) {
  return (
    <StrictMode>
      <LocaleProvider>
        <ToastProvider>
          <Harness contextKey={contextKey} />
        </ToastProvider>
      </LocaleProvider>
    </StrictMode>
  );
}

async function flush() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

console.log("\naccount sidebar bootstrap");

useAccountStore.getState().reset();
const host = document.getElementById("root")!;
const root = createRoot(host);
await act(async () => root.render(renderHarness("tab-a")));
await flush();
ok(requests.length === 1, "mount bootstrap is deduplicated under StrictMode");
ok(cachedAssetRequests.length === 1, "mount cached-asset read is deduplicated under StrictMode");
ok(host.textContent === "loading", "uninitialized sidebar renders loading instead of a signed-out label");

await act(async () => {
  cachedAssetRequests[0].resolve({
    profile_id: "profile-Account A",
    cloud_user_id: "user-Account A",
    binding_generation: 1,
    balance: { total_balance: "12.50", version: 1 },
  });
  await cachedAssetRequests[0].promise;
});
await flush();
ok(
  host.querySelector("button")?.getAttribute("data-balance") === "12.50",
  "cached balance renders before the authoritative bootstrap settles",
);

for (const listener of assetListeners) listener({ balanceChanged: true, subscriptionChanged: true });
await flush();
ok(assetRequests.length === 0, "asset wakeup waits for the in-flight bootstrap before refreshing remotely");

await act(async () => {
  host.querySelector<HTMLButtonElement>("button")?.click();
  await Promise.resolve();
});
ok(requests.length === 1, "opening the account menu reuses the in-flight bootstrap");

await act(async () => {
  requests[0].resolve(bootstrap("Account A"));
  await requests[0].promise;
});
await flush();
ok(host.textContent === "Account A", "successful mount bootstrap restores the account without opening the menu");
ok(assetListeners.size === 1, "asset listener is deduplicated under StrictMode");
ok(assetRequests.length === 1, "early asset wakeup refreshes remotely immediately after bootstrap settles");
ok(cachedAssetRequests.length === 1, "asset wakeups do not repeat the initial cached-asset read");

await act(async () => {
  assetRequests[0].resolve(refreshedAssets("Account A", "20.00", "plus", 2));
  await assetRequests[0].promise;
});
await flush();
ok(useAccountStore.getState().balance?.total_balance === "20.00", "early wakeup converges assets newer than the bootstrap");
ok(useAccountStore.getState().subscription?.current.plan === "plus", "early wakeup converges subscription after bootstrap");

for (const listener of assetListeners) listener({ balanceChanged: false, subscriptionChanged: false });
await flush();
ok(assetRequests.length === 1, "empty wakeup payload does not refresh account assets");

for (const listener of assetListeners) listener({ balanceChanged: true, subscriptionChanged: false });
await flush();
ok(assetRequests.length === 2, "first asset wakeup starts one authoritative remote refresh");
for (const listener of assetListeners) {
  listener({ balanceChanged: false, subscriptionChanged: true });
  listener({ balanceChanged: true, subscriptionChanged: true });
}
await flush();
ok(assetRequests.length === 2, "wakeup burst stays single-flight while a remote refresh is pending");

await act(async () => {
  assetRequests[1].resolve(refreshedAssets("Account A", "30.00", "free", 3));
  await assetRequests[1].promise;
});
await flush();
ok(assetRequests.length === 3, "dirty wakeups coalesce into exactly one immediate trailing read");

await act(async () => {
  assetRequests[2].resolve(refreshedAssets("Account A", "40.00", "plus", 4));
  await assetRequests[2].promise;
});
await flush();
ok(useAccountStore.getState().balance?.total_balance === "40.00", "trailing read converges the sidebar balance without opening the menu");
ok(useAccountStore.getState().subscription?.current.plan === "plus", "combined remote refresh converges subscription independently");

for (const listener of assetListeners) listener({ balanceChanged: true, subscriptionChanged: false });
await flush();
await act(async () => {
  assetRequests[3].reject(new Error("remote assets unavailable"));
  try {
    await assetRequests[3].promise;
  } catch {
    // Expected diagnostic-only failure.
  }
});
await flush();
ok(useAccountStore.getState().balance?.total_balance === "40.00", "bridge rejection preserves the last confirmed balance");
ok(
  useAccountStore.getState().assetSyncError === "同步云端账户信息失败，请稍后重试。",
  "remote refresh rejection records a safe Chinese diagnostic",
);

for (const listener of assetListeners) listener({ balanceChanged: true, subscriptionChanged: false });
await flush();
ok(assetRequests.length === 5, "old context starts its final remote refresh before navigation");
await act(async () => root.render(renderHarness("tab-b")));
await flush();
ok(requests.length === 2 && useAccountStore.getState().contextKey === "tab-b", "context change starts one bootstrap for the new context");
ok(assetListeners.size === 1 && assetListenerRemovals > 0, "context change cleans up the previous asset listener before registering its replacement");
await act(async () => {
  assetRequests[4].resolve(refreshedAssets("Account A", "late-old-context", "plus", 100));
  await assetRequests[4].promise;
});
await flush();
ok(
  useAccountStore.getState().contextKey === "tab-b" && useAccountStore.getState().balance === null,
  "late asset response from the previous context cannot pollute the active context",
);

await act(async () => root.render(renderHarness("tab-c")));
await flush();
ok(requests.length === 3 && useAccountStore.getState().contextKey === "tab-c", "a second context change starts only its own bootstrap");

await act(async () => {
  requests[1].resolve(bootstrap("Stale B"));
  await requests[1].promise;
});
await flush();
ok(useAccountStore.getState().contextKey === "tab-c" && host.textContent === "loading", "late bootstrap response cannot pollute the active context");

await act(async () => {
  requests[2].resolve(bootstrap("Account C"));
  await requests[2].promise;
});
await flush();
ok(host.textContent === "Account C", "active context accepts its own bootstrap response");

await act(async () => root.render(renderHarness("tab-a")));
await flush();
ok(requests.length === 3 && host.textContent === "Account A", "returning to an initialized context reuses the cached account without a request");
ok(
  host.querySelector("button")?.getAttribute("data-avatar") === "data:image/png;base64,account-a",
  "recoverable account reads its stable cloud user ID avatar slot",
);

avatarRenders.length = 0;
await act(async () => {
  useAccountStore.getState().applyBootstrap(bootstrap("Account B"));
});
ok(avatarRenders[0] === "", "account switch first render hides the previous account avatar");
ok(
  host.querySelector("button")?.getAttribute("data-avatar") === "data:image/png;base64,account-b",
  "account switch loads only the target account avatar slot",
);

avatarRenders.length = 0;
await act(async () => {
  useAccountStore.getState().applyBootstrap({
    ...bootstrap("Account B"),
    account: {
      ...bootstrap("Account B").account,
      status: "anonymous",
    },
  });
});
ok(avatarRenders[0] === "", "logout first render hides the previous account avatar");
ok(
  host.querySelector("button")?.getAttribute("data-avatar") === "data:image/png;base64,local",
  "anonymous account uses the local avatar slot even when the previous cloud user ID remains bound",
);

await act(async () => {
  useAccountStore.getState().applyBootstrap({
    ...bootstrap("Offline"),
    account: { ...bootstrap("Offline").account, status: "offline" },
  });
});
ok(
  host.querySelector("button")?.getAttribute("data-avatar") === "data:image/png;base64,offline",
  "offline account with a stable ID keeps its account avatar slot",
);

await act(async () => {
  useAccountStore.getState().applyBootstrap({
    ...bootstrap("Expired"),
    account: { ...bootstrap("Expired").account, status: "expired" },
  });
});
ok(
  host.querySelector("button")?.getAttribute("data-avatar") === "data:image/png;base64,local",
  "expired account does not restore the old cloud user avatar slot",
);

await act(async () => {
  useAccountStore.getState().applyBootstrap({
    ...bootstrap("Display Name Only"),
    account: {
      ...bootstrap("Display Name Only").account,
      cloud_user_id: undefined,
    },
  });
});
ok(
  host.querySelector("button")?.getAttribute("data-avatar") === "data:image/png;base64,local",
  "account without a stable cloud user ID does not use its display name as an avatar slot",
);

const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
ok(
  /const sidebarAccountLabel = !sidebarAccountInitialized[\s\S]*?t\("common\.loading"\)[\s\S]*?: sidebarAccountRequiresReauthentication[\s\S]*?t\("sidebar\.reauthenticationRequired"\)[\s\S]*?: sidebarAccountHasSession[\s\S]*?t\("account\.nameUnavailable"\)/.test(appSource),
  "App maps loading and expired sessions to distinct labels before the safe recoverable identity fallback",
);
ok(
  appSource.includes('new URL("./assets/brand-symbol.png", import.meta.url).href')
    && appSource.includes("<img src={src || brandAccountAvatar}")
    && !/sidebarAccountInitial\b/.test(appSource),
  "App uses the solid brand symbol instead of an account-label initial",
);
ok(
  appSource.includes("accountKey: modelCatalogAccountScopeKey")
    && appSource.includes("accountKey: subscriptionRepairAccountKey"),
  "subscription changes locally re-filter stable model facts without changing their backend request key",
);

await act(async () => {
  useAccountStore.getState().applyBootstrap(bootstrap("Unmount"));
});
const balanceBeforeUnmount = useAccountStore.getState().balance?.total_balance;
const assetRequestCountBeforeUnmount = assetRequests.length;
for (const listener of assetListeners) listener({ balanceChanged: true, subscriptionChanged: false });
await flush();
ok(assetRequests.length === assetRequestCountBeforeUnmount + 1, "unmount scenario starts an in-flight remote refresh");
const unmountAssetRequest = assetRequests[assetRequestCountBeforeUnmount];
await act(async () => root.unmount());
ok(assetListeners.size === 0 && assetListenerRemovals === assetListenerRegistrations, "unmount removes every registered asset listener");
await act(async () => {
  unmountAssetRequest.resolve(refreshedAssets("Unmount", "late-after-unmount", "plus", 100));
  await unmountAssetRequest.promise;
});
await flush();
ok(useAccountStore.getState().balance?.total_balance === balanceBeforeUnmount, "late asset response after unmount cannot commit");
useAccountStore.getState().reset();
dom.window.close();
process.stdout.write(`\n${passed} passed, ${failed} failed\n`);
if (failed > 0) process.exit(1);
