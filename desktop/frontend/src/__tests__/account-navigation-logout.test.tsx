// Run: tsx src/__tests__/account-navigation-logout.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";

import { useAccountNavigation } from "../features/account/controller/useAccountNavigation";
import {
  type CloudBootstrapView,
  useAccountStore,
} from "../features/account/store/accountStore";

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

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.Node = dom.window.Node;
  globalThis.HTMLElement = dom.window.HTMLElement;
  return dom;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (cause: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

const authenticated: CloudBootstrapView = {
  available: true,
  account: {
    status: "authenticated",
    profile_id: "profile-a",
    cloud_user_id: "user-a",
    binding_generation: 1,
  },
  balance: { total_balance: "100.00" },
};
const anonymous: CloudBootstrapView = {
  available: true,
  account: {
    status: "anonymous",
    profile_id: "profile-a",
    cloud_user_id: "user-a",
    binding_generation: 1,
  },
};

interface NavigationHandle {
  accountLogoutPending: boolean;
  requestAccountLogout: () => Promise<void>;
}

let navigation!: NavigationHandle;

function Harness({
  logout,
  beforeLogout,
  beforeOpen,
  openSurface,
  onLogoutError,
}: {
  logout: () => Promise<CloudBootstrapView>;
  beforeLogout: () => void;
  beforeOpen: () => void;
  openSurface: () => void;
  onLogoutError: (cause: unknown) => void;
}) {
  navigation = useAccountNavigation(beforeOpen, openSurface, { logout, beforeLogout, onLogoutError });
  return <span data-logout-pending={navigation.accountLogoutPending ? "true" : "false"} />;
}

console.log("\naccount navigation logout sequencing");

const dom = installDom();
const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

useAccountStore.getState().reset();
useAccountStore.getState().activateContext("desktop");
useAccountStore.getState().applyBootstrap(authenticated);

const success = deferred<CloudBootstrapView>();
let logoutCalls = 0;
const successEvents: string[] = [];
await act(async () => {
  root.render(
    <Harness
      logout={() => {
        logoutCalls += 1;
        return success.promise;
      }}
      beforeLogout={() => successEvents.push("beforeLogout")}
      beforeOpen={() => successEvents.push("beforeOpen")}
      openSurface={() => successEvents.push(`open:${useAccountStore.getState().account?.status}`)}
      onLogoutError={() => successEvents.push("error")}
    />,
  );
});

let firstRequest!: Promise<void>;
let duplicateRequest!: Promise<void>;
await act(async () => {
  firstRequest = navigation!.requestAccountLogout();
  duplicateRequest = navigation!.requestAccountLogout();
});
ok(firstRequest === duplicateRequest && logoutCalls === 1, "duplicate clicks share one logout request");
ok(
  successEvents.join(",") === "beforeLogout"
    && useAccountStore.getState().account?.status === "authenticated",
  "pending logout stays on the authenticated workspace without opening account",
);
ok(navigation.accountLogoutPending === true, "pending state disables repeated sidebar logout interaction");

await act(async () => {
  success.resolve(anonymous);
  await firstRequest;
});
ok(
  successEvents.join(",") === "beforeLogout,beforeOpen,open:anonymous",
  "successful logout applies anonymous state before account navigation",
);
ok(useAccountStore.getState().account?.status === "anonymous", "successful logout updates the shared sidebar account state");
ok(navigation.accountLogoutPending === false, "successful logout clears the pending state");

useAccountStore.getState().applyBootstrap(authenticated);
const failure = deferred<CloudBootstrapView>();
const failureEvents: string[] = [];
let reportedFailure: unknown;
await act(async () => {
  root.render(
    <Harness
      logout={() => failure.promise}
      beforeLogout={() => failureEvents.push("beforeLogout")}
      beforeOpen={() => failureEvents.push("beforeOpen")}
      openSurface={() => failureEvents.push("open")}
      onLogoutError={(cause) => {
        reportedFailure = cause;
        failureEvents.push("error");
      }}
    />,
  );
});

const logoutFailure = new Error("logout unavailable");
let failedRequest!: Promise<void>;
await act(async () => {
  failedRequest = navigation!.requestAccountLogout();
  failure.reject(logoutFailure);
  await failedRequest;
});
ok(failureEvents.join(",") === "beforeLogout,error", "failed logout reports an error without account navigation");
ok(reportedFailure === logoutFailure, "failed logout forwards the original cause to the visible error handler");
ok(
  useAccountStore.getState().account?.status === "authenticated"
    && useAccountStore.getState().balance?.total_balance === "100.00",
  "failed logout preserves the authenticated account and assets",
);
ok(navigation.accountLogoutPending === false, "failed logout allows a later retry");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
