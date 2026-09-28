// Run: tsx src/__tests__/account-center-payment-integration.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";

import { AccountCenter, type AccountCloudBindings } from "../components/AccountCenter";
import { LocaleProvider } from "../lib/i18n";
import {
  type CloudAccountView,
  type CloudBootstrapView,
  type CloudPaymentView,
  useAccountStore,
} from "../store/account";

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
  globalThis.SVGElement = dom.window.SVGElement;
  globalThis.Event = dom.window.Event;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  return dom;
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    await act(async () => {
      await flush();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function invokeReactClick(button: HTMLButtonElement) {
  const reactPropsKey = Object.keys(button).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (button as unknown as Record<string, { onClick?: () => void }>)[reactPropsKey]
    : undefined;
  if (!reactProps?.onClick) throw new Error("missing React click handler");
  reactProps.onClick();
}

function account(profileID: string, userID: string, generation: number): CloudAccountView {
  return {
    status: "authenticated",
    profile_id: profileID,
    cloud_user_id: userID,
    binding_generation: generation,
    display_name: `${profileID} user`,
  };
}

function pendingPayment(orderID: string): CloudPaymentView {
  return {
    order_id: orderID,
    order_type: "subscription_purchase",
    plan: "plus",
    amount: "29.90",
    amount_unit: "yuan",
    currency: "CNY",
    payment_channel: "alipay",
    state: "paying",
    payable: true,
    entitlement_granted: false,
    expires_at: "2030-01-01T00:30:00Z",
    payment_entry: {
      entry_type: "qr_code",
      qr_code: `https://qr.alipay.test/${orderID}`,
      expires_at: "2030-01-01T00:30:00Z",
    },
  };
}

function authenticatedView(
  identity: CloudAccountView,
  balance: string,
  payments: CloudPaymentView[] = [],
): CloudBootstrapView {
  return {
    available: true,
    account: identity,
    balance: { total_balance: balance },
    subscription: {
      plans: [
        {
          plan: "lite",
          display_name: "Lite DTO",
          price_amount: "9.90",
          currency: "CNY",
          credit_amount: "20000.0000",
          period_days: 30,
          enabled: true,
          sort_order: 10,
          config_version: "catalog-v1",
        },
        {
          plan: "plus",
          display_name: "Plus DTO",
          price_amount: "29.90",
          currency: "CNY",
          credit_amount: "60000.0000",
          period_days: 30,
          enabled: true,
          sort_order: 20,
          config_version: "catalog-v1",
        },
        {
          plan_id: "plan-pro",
          plan: "pro",
          display_name: "Pro DTO",
          price_amount: "59.90",
          currency: "CNY",
          credit_amount: "120000.0000",
          period_days: 30,
          enabled: true,
          sort_order: 30,
          config_version: "catalog-v1",
        },
      ],
      current: { plan: "lite", status: "active", updated_at: "2026-07-12T00:00:00Z" },
    },
    payments,
  };
}

const profileA = account("profile-a", "user-a", 1);
const profileB = account("profile-a", "user-b", 2);
const profileC = account("profile-c", "user-c", 1);
const orderA = pendingPayment("order-a");
const viewA = authenticatedView(profileA, "100.00", [orderA]);
const viewB = authenticatedView(profileB, "200.00");
const viewC = authenticatedView(profileC, "300.00");
function apiWith(bootstrap: CloudBootstrapView): AccountCloudBindings {
  return {
    CloudBootstrap: async () => bootstrap,
    CloudCaptchaConfig: async () => { throw new Error("real CAPTCHA is outside this test"); },
    CloudSendRegisterSMS: async () => undefined,
    CloudSendPasswordResetSMS: async () => undefined,
    CloudResetPassword: async () => undefined,
    CloudRegister: async () => bootstrap,
    CloudLogin: async () => bootstrap,
    CloudSwitchAccount: async () => bootstrap,
    CloudLogout: async () => ({ available: true, account: { status: "anonymous", profile_id: bootstrap.account.profile_id } }),
    CloudRefreshAssets: async () => viewB,
    CloudCreateSubscriptionOrder: async () => { throw new Error("create is outside this integration test"); },
    CloudCreateUpgradeOrder: async () => { throw new Error("upgrade is outside this integration test"); },
    CloudConfirmPayment: async () => { throw new Error("V3 does not use manual confirmation"); },
    CloudRefreshPayment: async () => { throw new Error("poll is outside this integration test"); },
  };
}

console.log("\naccount center payment page integration");

const dom = installDom();
useAccountStore.getState().reset();
const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
const apiA = apiWith(viewA);

await act(async () => {
  root.render(
    <LocaleProvider>
      <AccountCenter api={apiA} contextKey="tab-a" />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("profile A bootstrap", () => rootElement.textContent?.includes("100.00") === true);

ok(rootElement.querySelector(".account-center__pages") === null, "authenticated center avoids a redundant overview tab strip");
ok(rootElement.querySelector('[data-account-page="payments"]') !== null, "authenticated overview exposes the plans and payments entry");
ok(rootElement.querySelector('[data-account-page-panel="overview"]') !== null, "overview is the default authenticated page");
ok(rootElement.querySelector("[data-account-payments]") === null, "payment polling component is not mounted on the overview page");
ok(rootElement.textContent?.includes("100.00") === true && rootElement.textContent?.includes("Lite DTO") === true, "overview renders backend balance and plan display name");

await act(async () => {
  invokeReactClick(rootElement.querySelector<HTMLButtonElement>('[data-account-page="payments"]')!);
  await flush();
});
ok(rootElement.querySelector("[data-account-payments]") !== null, "payment page mounts the scoped AccountPayments component");
ok(rootElement.querySelector('[data-account-page-panel="overview"]') === null, "payment page hides the overview panel");
ok(rootElement.querySelector('[data-subscription-modal][role="dialog"]') !== null, "payment page opens as a subscription modal");
ok(rootElement.querySelector('[data-payment-stage-view="scan"]') !== null, "subscription modal resumes the active payment before offering another plan");

await act(async () => {
  invokeReactClick(rootElement.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')!);
  await flush();
});
await waitFor("profile B scope", () => useAccountStore.getState().account?.cloud_user_id === "user-b");
ok(rootElement.querySelector('[data-account-page-panel="overview"]') !== null, "scope change returns to account overview");
ok(rootElement.querySelector("[data-account-payments]") === null, "scope change unmounts the old payment lifecycle");
ok(rootElement.textContent?.includes("200.00") === true && rootElement.textContent?.includes("100.00") === false, "scope change atomically replaces overview assets");

await act(async () => {
  invokeReactClick(rootElement.querySelector<HTMLButtonElement>('[data-account-page="payments"]')!);
  await flush();
});
ok(rootElement.querySelector("[data-account-payments]") !== null, "payment page can be reopened for the current scope");
ok(rootElement.querySelector('[data-payment-channel-selector]') !== null, "subscription modal exposes the real payment channels as a segmented control");
ok(
  rootElement.querySelector<HTMLButtonElement>('[data-payment-channel-option="alipay"]')?.disabled === false
    && rootElement.querySelector<HTMLButtonElement>('[data-payment-channel-option="wechat"]')?.disabled === true,
  "subscription modal reserves an adjacent disabled WeChat option beside active Alipay",
);
ok(rootElement.querySelector('[data-payment-plan="pro"][data-payment-recommended="true"]') !== null, "the experience plan is the only visible recommended plan");

let apiCLogoutCalls = 0;
const apiC: AccountCloudBindings = {
  ...apiWith(viewC),
  CloudLogout: async () => {
    apiCLogoutCalls += 1;
    return { available: true, account: { ...viewC.account, status: "anonymous" } };
  },
};
await act(async () => {
  root.render(
    <LocaleProvider>
      <AccountCenter api={apiC} contextKey="tab-c" />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("profile C context", () => useAccountStore.getState().account?.cloud_user_id === "user-c");
ok(rootElement.querySelector('[data-account-page-panel="overview"]') !== null, "context change returns to account overview");
ok(rootElement.querySelector("[data-account-payments]") === null, "context change unmounts the previous payment lifecycle");

await act(async () => {
  root.render(
    <LocaleProvider>
      <AccountCenter api={apiC} contextKey="tab-c" request={{ id: 1, page: "payments" }} />
    </LocaleProvider>,
  );
  await flush();
});
ok(rootElement.querySelector('[data-subscription-modal][role="dialog"]') !== null, "sidebar request opens plans and payments directly");

await act(async () => {
  root.render(
    <LocaleProvider>
      <AccountCenter api={apiC} contextKey="tab-c" request={{ id: 2, page: "overview", action: "logout" }} />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("sidebar logout request", () => useAccountStore.getState().account?.status === "anonymous");
ok(apiCLogoutCalls === 1, "sidebar logout request uses the existing cloud logout action exactly once");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
