// Run: tsx src/__tests__/account-center-states.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";

import { AccountCenter, type AccountCloudBindings } from "../components/AccountCenter";
import { LocaleProvider } from "../lib/i18n";
import { zh } from "../locales/zh";
import type { CloudBootstrapView } from "../store/account";
import { useAccountStore } from "../store/account";

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
  const dom = new JSDOM("<!doctype html><html><body></body></html>", {
    pretendToBeVisual: true,
    url: "http://localhost/",
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  globalThis.window = dom.window as unknown as Window & typeof globalThis;
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  globalThis.Node = dom.window.Node;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLInputElement = dom.window.HTMLInputElement;
  globalThis.Event = dom.window.Event;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  Object.defineProperty(dom.window.HTMLElement.prototype, "attachEvent", { configurable: true, value: () => {} });
  Object.defineProperty(dom.window.HTMLElement.prototype, "detachEvent", { configurable: true, value: () => {} });
  return dom;
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 25; attempt += 1) {
    await act(async () => {
      await flush();
    });
    if (predicate()) return;
  }
  throw new Error(`timed out waiting for ${label}`);
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (cause: unknown) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}

function invokeReactClick(button: HTMLButtonElement) {
  const reactPropsKey = Object.keys(button).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (button as unknown as Record<string, { onClick?: () => void }>)[reactPropsKey]
    : undefined;
  if (!reactProps?.onClick) throw new Error("missing React click handler");
  reactProps.onClick();
}

function setInput(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
  if (!setter) throw new Error("missing input value setter");
  setter.call(input, value);
  const reactPropsKey = Object.keys(input).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (input as unknown as Record<string, { onChange?: (event: { currentTarget: HTMLInputElement; target: HTMLInputElement }) => void }>)[reactPropsKey]
    : undefined;
  reactProps?.onChange?.({ currentTarget: input, target: input });
  input.dispatchEvent(new window.Event("input", { bubbles: true }));
  input.dispatchEvent(new window.Event("change", { bubbles: true }));
}

const anonymous: CloudBootstrapView = {
  available: true,
  capabilities: { registration: true, registration_mode: "fixture", payments: true, realtime: true },
  account: { status: "anonymous", profile_id: "default" },
};

function authenticated(
  totalBalance: string,
  generation = 1,
  profileID = "default",
  cloudUserID = "usr_01",
): CloudBootstrapView {
  return {
    available: true,
    capabilities: { registration: true, registration_mode: "fixture", payments: true, realtime: true },
    account: {
      status: "authenticated",
      profile_id: profileID,
      cloud_user_id: cloudUserID,
      binding_generation: generation,
      display_name: "Fixture User",
      role: "user",
      last_sync_at: "2026-07-11T08:00:00Z",
    },
    balance: { total_balance: totalBalance },
    subscription: {
      plans: [
        {
          plan: "starter",
          display_name: "Starter",
          price_amount: "10.00",
          currency: "CNY",
          credit_amount: "100.00",
          period_days: 30,
          enabled: true,
          sort_order: 1,
          config_version: "catalog-v1",
        },
        {
          plan: "pro",
          display_name: "Pro",
          price_amount: "20.00",
          currency: "CNY",
          credit_amount: "240.00",
          period_days: 30,
          enabled: true,
          sort_order: 2,
          config_version: "catalog-v1",
        },
      ],
      current: {
        plan: "starter",
        status: "active",
        period_end: "2026-08-10T08:00:00Z",
        updated_at: "2026-07-11T08:00:00Z",
      },
    },
  };
}

function apiWith(overrides: Partial<AccountCloudBindings>): AccountCloudBindings {
  return {
    CloudBootstrap: async () => anonymous,
    CloudCaptchaConfig: async () => { throw new Error("real CAPTCHA is outside this test"); },
    CloudSendRegisterSMS: async () => undefined,
    CloudSendPasswordResetSMS: async () => undefined,
    CloudResetPassword: async () => undefined,
    CloudRegister: async () => anonymous,
    CloudLogin: async () => anonymous,
    CloudSwitchAccount: async () => anonymous,
    CloudLogout: async () => anonymous,
    CloudRefreshAssets: async () => authenticated("100.00"),
    CloudCreateSubscriptionOrder: async () => { throw new Error("payment create is outside this test"); },
    CloudCreateUpgradeOrder: async () => { throw new Error("payment upgrade is outside this test"); },
    CloudConfirmPayment: async () => { throw new Error("payment confirm is outside this test"); },
    CloudRefreshPayment: async () => { throw new Error("payment refresh is outside this test"); },
    CloudCreateMaaSRechargeOrder: async () => { throw new Error("recharge create is outside this test"); },
    CloudGetMaaSRechargeOrder: async () => { throw new Error("recharge query is outside this test"); },
    CloudGetMaaSRechargeKeyBalance: async () => ({
      key_name: "usr_01cz",
      available_balance: "0.0000",
      remain_quota: 0,
      balance_cny: "0",
      source: "maas_recharge_api_key",
      conversion_rule_version: "maas-quota-credit-v1",
      cny_per_usd: "7.3",
      quota_per_usd: "500000",
      queried_at: "2026-08-22T10:00:00Z",
      available_credits: "0",
      package_available_credits: "100",
      credit_per_cny: "2000",
    }),
    ...overrides,
  };
}

async function renderAccount(
  api: AccountCloudBindings,
  contextKey = "default",
  avatarSrc = "",
): Promise<{ container: HTMLElement; root: Root }> {
  useAccountStore.getState().reset();
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AccountCenter api={api} contextKey={contextKey} avatarSrc={avatarSrc} />
      </LocaleProvider>,
    );
    await flush();
  });
  return { container, root };
}

async function dispose(root: Root, container: HTMLElement) {
  await act(async () => root.unmount());
  container.remove();
}

console.log("\naccount center anonymous state");

const dom = installDom();
const smsCaptchaValues: boolean[] = [];
const anonymousRender = await renderAccount(apiWith({
  CloudSendRegisterSMS: async (request) => {
    smsCaptchaValues.push(request.captcha_passed === true);
  },
}));
await waitFor("anonymous bootstrap", () => anonymousRender.container.querySelector('[data-account-state="anonymous"]') !== null);
const anonymousRoot = anonymousRender.container.querySelector<HTMLElement>('[data-account-state="anonymous"]');
ok(anonymousRoot?.classList.contains("account-center--auth") === true, "anonymous state uses the dedicated login experience");
ok(anonymousRender.container.querySelector(".account-center__auth-shell") !== null, "anonymous state renders the split auth shell");
ok(
  anonymousRender.container.querySelector<HTMLImageElement>(".account-center__auth-wordmark")?.alt === "云智企办",
  "login experience uses the canonical product name",
);
ok(
  anonymousRender.container.querySelector("#account-auth-title")?.textContent === zh["account.loginTitle"],
  "login experience starts with factual login copy",
);
ok(
  anonymousRender.container.querySelector('label[for="password"]')?.textContent === `${zh["account.password"]}*`,
  "login mode keeps the password label",
);
ok(anonymousRender.container.querySelector('[data-account-action="login"]') !== null, "anonymous state exposes login");
ok(anonymousRender.container.querySelector('[data-account-mode="register"]') !== null, "anonymous state exposes registration");
ok(anonymousRender.container.querySelector("[data-account-reauthentication]") === null, "anonymous state does not show the expired-session notice");
ok(anonymousRender.container.querySelector(".account-center__identity-card") === null, "anonymous state does not add an identity card");

await act(async () => {
  anonymousRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
ok(
  anonymousRender.container.querySelector("#account-auth-title")?.textContent === zh["account.registerTitle"],
  "registration mode updates the account heading",
);
ok(
  anonymousRender.container.querySelector('label[for="password"]')?.textContent === `${zh["account.setPassword"]}*`,
  "registration mode uses the set-password label",
);
const anonymousPhone = anonymousRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
if (!anonymousPhone) throw new Error("missing anonymous phone");
await act(async () => {
  setInput(anonymousPhone, "13800000000");
  await flush();
});
await act(async () => {
  anonymousRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]')?.click();
  await flush();
});
await act(async () => {
  anonymousRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.click();
  await flush();
});
ok(smsCaptchaValues[0] === true, "fake captcha gates the SMS bridge call");
await dispose(anonymousRender.root, anonymousRender.container);

console.log("\naccount center capability normalization");

const limitedAnonymousRender = await renderAccount(apiWith({
  CloudBootstrap: async () => ({
    ...anonymous,
    capabilities: { registration: false, registration_mode: "unavailable", payments: false, realtime: false },
  }),
}));
await waitFor("limited anonymous bootstrap", () => limitedAnonymousRender.container.querySelector('[data-account-state="anonymous"]') !== null);
const disabledRegistration = limitedAnonymousRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]');
ok(disabledRegistration?.disabled === false, "connected cloud keeps registration enabled despite stale capability metadata");
ok(disabledRegistration?.dataset.capabilityUnavailable === undefined, "connected cloud removes the obsolete future-capability marker");
const realAuthHintElement = limitedAnonymousRender.container.querySelector<HTMLElement>(".account-center__auth-hint");
const realAuthHint = realAuthHintElement?.textContent ?? "";
ok(realAuthHintElement?.dataset.accountAuthHint === "real", "stale unavailable metadata is normalized to the real-cloud account hint");
ok(!/fake mode|模拟模式/i.test(realAuthHint), "real authentication never displays the fixture-mode hint");
await dispose(limitedAnonymousRender.root, limitedAnonymousRender.container);

const limitedAuthenticatedRender = await renderAccount(apiWith({
  CloudBootstrap: async () => ({
    ...authenticated("1000.0000"),
    capabilities: { registration: false, registration_mode: "unavailable", payments: false, realtime: false },
  }),
}));
await waitFor("limited authenticated bootstrap", () => limitedAuthenticatedRender.container.querySelector('[data-account-state="authenticated"]') !== null);
const disabledPayments = limitedAuthenticatedRender.container.querySelector<HTMLButtonElement>('[data-account-page="payments"]');
ok(disabledPayments?.disabled === false, "plans page remains available when payment actions are disabled");
ok(disabledPayments?.dataset.capabilityUnavailable === "true", "plans page marks payment actions as a future capability");
ok(limitedAuthenticatedRender.container.querySelector('[data-payment-plan]') === null, "overview does not duplicate the plan catalog");
await act(async () => {
  disabledPayments?.click();
  await flush();
});
ok(limitedAuthenticatedRender.container.querySelector('[data-account-page-panel="payments"]') !== null, "plans page opens without payment capability");
ok(limitedAuthenticatedRender.container.querySelector('[data-payment-disabled-notice]') !== null, "plans page explains that payment actions are unavailable");
ok(limitedAuthenticatedRender.container.querySelector('[data-payment-plan="starter"]') === null, "plans page hides non-Pro catalog entries");
ok(limitedAuthenticatedRender.container.querySelector('[data-payment-plan="pro"]') !== null, "plans page keeps the experience plan visible");
const disabledPaymentActions = Array.from(limitedAuthenticatedRender.container.querySelectorAll<HTMLButtonElement>("[data-payment-create]"));
ok(disabledPaymentActions.length === 0, "subscribed users expose no purchase or upgrade actions");
await dispose(limitedAuthenticatedRender.root, limitedAuthenticatedRender.container);

const offlineRender = await renderAccount(apiWith({
  CloudBootstrap: async () => ({
    ...authenticated("900.00"),
    available: false,
    account: {
      ...authenticated("900.00").account,
      status: "offline",
    },
    operation_error: { code: "NETWORK_UNAVAILABLE", message: "cloud network is unavailable" },
  }),
}));
await waitFor("offline account bootstrap", () => offlineRender.container.querySelector('[data-account-state="offline"]') !== null);
ok(offlineRender.container.querySelector('[data-account-page-panel="overview"]') !== null, "offline bound session renders its recoverable account view");
ok(offlineRender.container.textContent?.includes("900.00") === true, "offline bound session keeps cached assets visible");
ok(offlineRender.container.querySelector('[data-account-action="register"]') === null, "offline bound session never falls back to registration");
await dispose(offlineRender.root, offlineRender.container);

console.log("\naccount center cross-executor mixed authenticated baseline");

const mixedAuthenticatedBaseline: CloudBootstrapView = {
  available: true,
  capabilities: { registration: true, registration_mode: "development", payments: false, realtime: false },
  account: {
    status: "authenticated",
    profile_id: "cross-executor",
    cloud_user_id: "usr_cross_executor",
    binding_generation: 7,
  },
  operation_error: {
    code: "REFRESH_TOKEN_REUSED",
    message: "cloud session expired; sign in again",
    request_id: "req-cross-refresh-reused",
		diagnostic_id: "diag-cross-refresh-reused",
  },
};
const mixedAuthenticatedRender = await renderAccount(apiWith({
  CloudBootstrap: async () => mixedAuthenticatedBaseline,
}), "cross-executor-mixed");
await waitFor("mixed authenticated bootstrap", () => mixedAuthenticatedRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(mixedAuthenticatedRender.container.querySelector('[data-account-identity]')?.textContent === zh["account.nameUnavailable"], "mixed authenticated state uses the safe unavailable-name label");
ok(!mixedAuthenticatedRender.container.textContent?.includes("usr_cross_executor"), "mixed authenticated state does not render the cloud user ID as a name");
ok(mixedAuthenticatedRender.container.querySelector('[role="alert"]') !== null, "mixed authenticated state renders the ordinary operation error alert");
ok(mixedAuthenticatedRender.container.querySelector('[data-account-error-code]')?.textContent?.includes("REFRESH_TOKEN_REUSED") === true, "mixed authenticated state retains the safe operation error code");
ok(mixedAuthenticatedRender.container.querySelector('[data-account-error-request-id]')?.textContent?.includes("req-cross-refresh-reused") === true, "mixed authenticated state retains the safe operation request ID");
ok(mixedAuthenticatedRender.container.querySelector('[data-account-error-diagnostic-id]')?.textContent?.includes("diag-cross-refresh-reused") === true, "mixed authenticated state retains the local diagnostic ID");
ok(mixedAuthenticatedRender.container.querySelector('[data-account-error-diagnostic-id] .copybtn') !== null, "mixed authenticated state renders a dedicated Log ID copy control");
ok(mixedAuthenticatedRender.container.querySelector('[data-account-reauthentication]') === null, "mixed authenticated state does not show the expired-session notice");
await dispose(mixedAuthenticatedRender.root, mixedAuthenticatedRender.container);

console.log("\naccount center expired state");

const expiredBootstrap: CloudBootstrapView = {
  ...authenticated("secret-balance"),
  account: {
    ...authenticated("secret-balance").account,
    status: "expired",
    cloud_user_id: "expired-user-id",
    display_name: undefined,
    role: undefined,
  },
  operation_error: {
    code: "AUTH_REQUIRED",
    message: "authentication is required",
    request_id: "req-cross-current-auth",
  },
};
const expiredRender = await renderAccount(apiWith({
  CloudBootstrap: async () => expiredBootstrap,
  CloudLogin: async () => authenticated("321.00", 2, "default", "reauthenticated-user"),
}), "expired-context", "data:image/png;base64,expired-avatar");
await waitFor("expired account bootstrap", () => expiredRender.container.querySelector('[data-account-state="expired"]') !== null);
ok(expiredRender.container.querySelector("[data-account-reauthentication]") !== null, "expired state shows the inline reauthentication notice");
ok(expiredRender.container.querySelector('[role="alert"]') !== null, "expired state retains the operation error alert");
ok(expiredRender.container.querySelector('[data-account-error-code]')?.textContent?.includes("AUTH_REQUIRED") === true, "expired state retains the safe operation error code");
ok(expiredRender.container.querySelector('[data-account-action="login"]') !== null, "expired state reuses the login form");
ok(expiredRender.container.querySelector('[data-account-mode="register"]') !== null, "expired state reuses the registration tab");
ok(expiredRender.container.querySelector(".account-center__identity-card") === null, "expired state hides the identity card");
ok(expiredRender.container.querySelector('[data-account-page="payments"]') === null, "expired state hides plans and payments");
ok(expiredRender.container.querySelector('[data-account-action="logout"]') === null, "expired state hides logout");
ok(!expiredRender.container.textContent?.includes("expired-user-id"), "expired state does not render the retained ordering UID");
ok(!expiredRender.container.textContent?.includes("secret-balance"), "expired state does not render stale assets");
const expiredPhone = expiredRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const expiredPassword = expiredRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!expiredPhone || !expiredPassword) throw new Error("missing expired login inputs");
await act(async () => {
  setInput(expiredPhone, "13900000000");
  setInput(expiredPassword, "reauthentication-secret");
  await flush();
});
await act(async () => {
  expiredRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("expired account reauthentication", () => expiredRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(expiredRender.container.querySelector("[data-account-reauthentication]") === null, "successful login removes the expired notice");
ok(expiredRender.container.textContent?.includes("321.00") === true, "successful login restores the authenticated overview");
await dispose(expiredRender.root, expiredRender.container);

console.log("\naccount center candidate envelope");

const boundAnonymous: CloudBootstrapView = {
  available: true,
  capabilities: { registration: true, registration_mode: "fixture", payments: false, realtime: false },
  account: {
    status: "anonymous",
    profile_id: "default",
    cloud_user_id: "usr_01",
    binding_generation: 1,
  },
};
let candidateSwitchRequest: { phone: string; password: string } | undefined;
const candidateRender = await renderAccount(apiWith({
  CloudBootstrap: async () => boundAnonymous,
  CloudLogin: async () => ({
    ...boundAnonymous,
    operation_error: {
      code: "PROFILE_SCOPE_MISMATCH",
      message: "account belongs to another profile",
    },
    candidate: {
      status: "offline",
      profile_id: "default",
      cloud_user_id: "usr_02",
      binding_generation: 2,
      display_name: "Candidate User",
    },
  }),
  CloudSwitchAccount: async (request) => {
    candidateSwitchRequest = request;
    return authenticated("0.00", 2, "default", "usr_02");
  },
}));
await waitFor("candidate login form", () => candidateRender.container.querySelector('[data-account-action="login"]') !== null);
await act(async () => {
  setInput(candidateRender.container.querySelector<HTMLInputElement>('input[name="phone"]')!, "13900000002");
  setInput(candidateRender.container.querySelector<HTMLInputElement>('input[name="password"]')!, "candidate-secret");
  await flush();
});
await act(async () => {
  candidateRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("anonymous candidate response", () => useAccountStore.getState().account?.cloud_user_id === "usr_01");
const anonymousConfirmation = candidateRender.container.querySelector('[data-account-switch-stage="confirm"]');
ok(anonymousConfirmation !== null, "anonymous profile mismatch opens the switch confirmation");
ok(candidateRender.container.querySelector('[data-account-switch-candidate="usr_02"]') !== null, "anonymous confirmation displays the candidate identity");
ok(candidateRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent?.includes("PROFILE_SCOPE_MISMATCH") !== true, "anonymous profile mismatch never exposes the internal error code");
ok(useAccountStore.getState().account?.cloud_user_id === "usr_01", "candidate identity never replaces the current account scope");
await act(async () => {
  setInput(candidateRender.container.querySelector<HTMLInputElement>('input[name="switch_confirm_password"]')!, "candidate-secret");
  await flush();
});
await act(async () => {
  candidateRender.container.querySelector<HTMLButtonElement>('[data-account-action="switch-confirm"]')?.click();
  await flush();
});
await waitFor("anonymous candidate switch", () => useAccountStore.getState().account?.cloud_user_id === "usr_02");
ok(candidateSwitchRequest?.phone === "13900000002", "confirmed switch reuses the verified candidate phone");
ok(candidateSwitchRequest?.password === "candidate-secret", "confirmed switch requires target password re-entry");
await dispose(candidateRender.root, candidateRender.container);

console.log("\naccount center authenticated state");

let refreshCount = 0;
const authRender = await renderAccount(apiWith({
  CloudBootstrap: async () => authenticated("100.00"),
  CloudLogout: async () => ({
    available: true,
    account: {
      status: "anonymous",
      profile_id: "default",
      cloud_user_id: "usr_01",
      binding_generation: 1,
    },
  }),
  CloudRefreshAssets: async () => {
    refreshCount += 1;
    return authenticated("125.00");
  },
}), "default", "data:image/png;base64,account-avatar");
await waitFor("authenticated bootstrap", () => authRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(authRender.container.textContent?.includes("100.00") === true, "recovered account shows total balance");
ok(authRender.container.querySelector(".account-center--authenticated") !== null, "recovered account uses the authenticated account console");
ok(
  authRender.container.querySelector(".account-showcase__hero-copy > p")?.textContent === "一个办公 Agent，协同多家领先模型持续完成工作",
  "authenticated account uses the office Agent guidance",
);
ok(authRender.container.querySelector('[data-account-identity]')?.textContent === "Fixture User", "account identity displays the cloud display name instead of the user role");
ok(
  authRender.container.querySelector<HTMLImageElement>(".account-center__account-identity img")?.src === "data:image/png;base64,account-avatar",
  "account identity displays the local sidebar avatar",
);
ok(zh["account.identity"] === "用户名", "Simplified Chinese localizes the user name label");
ok(
  authRender.container.querySelector(".account-center__action-menu-profile i")?.getAttribute("aria-label") === "已连接",
  "account console maps the backend account status",
);
ok(authRender.container.querySelector('[data-account-current-plan]')?.textContent === "Starter", "account console resolves the current plan from the backend catalog");
ok(authRender.container.querySelector('[data-payment-plan]') === null, "overview does not duplicate the plan catalog");
await act(async () => {
  authRender.container.querySelector<HTMLButtonElement>('[data-account-page="payments"]')?.click();
  await flush();
});
ok(authRender.container.querySelector('[data-payment-plan="starter"]') === null, "plans page hides the current non-Pro plan");
const experiencePlan = authRender.container.querySelector<HTMLElement>('[data-payment-plan="pro"]');
ok(experiencePlan?.querySelector(".account-payments__plan-heading strong")?.textContent === "体验套餐", "plans page uses the experience plan name");
ok(experiencePlan?.querySelector('[data-payment-plan-price]')?.textContent === "¥20.00/ 30 天", "plans page keeps the cloud price and period");
ok(experiencePlan?.querySelector('[data-payment-plan-credit]')?.textContent?.includes("240") === true, "plans page uses the cloud experience credit display");
ok(experiencePlan?.querySelector('[data-payment-plan-period]')?.textContent === "30", "plans page shows the cloud period");
ok(experiencePlan?.dataset.paymentCurrent !== "true", "hidden current plan does not mark the experience plan as current");
await waitFor("never-recharged balance", () => (
  authRender.container.querySelector(".account-billing-modal__balance--subscription strong")?.textContent === "100"
  && authRender.container.querySelector(".account-billing-modal__balance--recharge strong")?.textContent === "0"
));
ok(
  authRender.container.querySelector(".account-billing-modal__balance--subscription strong")?.textContent === "100"
    && authRender.container.querySelector(".account-billing-modal__balance--recharge strong")?.textContent === "0",
  "never-recharged account shows zero recharge credits and all credits as package credits",
);

await act(async () => {
  authRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')?.click();
  await flush();
});
ok(
  refreshCount === 1 && useAccountStore.getState().balance?.total_balance === "125.00",
  "refresh replaces authenticated assets while the plans page is open",
);

await act(async () => {
  authRender.container.querySelector<HTMLButtonElement>('[data-account-action="logout"]')?.click();
  await flush();
});
ok(authRender.container.querySelector('[data-account-state="anonymous"]') !== null, "logout returns to anonymous state");
ok(!authRender.container.textContent?.includes("125.00"), "logout clears scoped assets");
await dispose(authRender.root, authRender.container);

console.log("\naccount billing balance states");

const creditedBalanceRender = await renderAccount(apiWith({
  CloudBootstrap: async () => authenticated("349200.0272"),
  CloudGetMaaSRechargeKeyBalance: async () => ({
    key_name: "usr_01cz",
    available_balance: "229200.0272",
    remain_quota: 7849316,
    balance_cny: "114.6000",
    source: "maas_recharge_api_key",
    conversion_rule_version: "maas-quota-credit-v1",
    cny_per_usd: "7.3",
    quota_per_usd: "500000",
    queried_at: "2026-08-22T10:00:00Z",
    available_credits: "229200.0272",
    package_available_credits: "120000.0000",
    credit_per_cny: "2000",
  }),
}));
await waitFor("credited balance account", () => creditedBalanceRender.container.querySelector('[data-account-state="authenticated"]') !== null);
await act(async () => {
  creditedBalanceRender.container.querySelector<HTMLButtonElement>('[data-account-page="payments"]')?.click();
  await flush();
});
await waitFor("credited balances", () => (
  creditedBalanceRender.container.querySelector(".account-billing-modal__balance--subscription strong")?.textContent === "120,000"
  && creditedBalanceRender.container.querySelector(".account-billing-modal__balance--recharge strong")?.textContent === "229,200.0272"
));
ok(
  creditedBalanceRender.container.querySelector(".account-billing-modal__balance--subscription strong")?.textContent === "120,000"
    && creditedBalanceRender.container.querySelector(".account-billing-modal__balance--recharge strong")?.textContent === "229,200.0272",
  "credited account displays independent package and recharge balances",
);
await dispose(creditedBalanceRender.root, creditedBalanceRender.container);

let retryBalanceCalls = 0;
const retryBalanceRender = await renderAccount(apiWith({
  CloudBootstrap: async () => authenticated("100.0000"),
  CloudGetMaaSRechargeKeyBalance: async () => {
    retryBalanceCalls += 1;
    if (retryBalanceCalls === 1) throw new Error("simulated balance failure");
    return {
      key_name: "",
      available_balance: "0.0000",
      remain_quota: 0,
      balance_cny: "",
      source: "",
      conversion_rule_version: "",
      cny_per_usd: "",
      quota_per_usd: "",
      available_credits: "0.0000",
      package_available_credits: "100.0000",
      credit_per_cny: "2000",
    };
  },
}));
await waitFor("retry balance account", () => retryBalanceRender.container.querySelector('[data-account-state="authenticated"]') !== null);
await act(async () => {
  retryBalanceRender.container.querySelector<HTMLButtonElement>('[data-account-page="payments"]')?.click();
  await flush();
});
await waitFor("combined balance failure", () => retryBalanceRender.container.querySelectorAll(".account-billing-modal__balance strong").length === 2
  && Array.from(retryBalanceRender.container.querySelectorAll(".account-billing-modal__balance strong"))
    .every((node) => node.textContent === zh["account.billing.balanceUnavailable"]));
ok(
  Array.from(retryBalanceRender.container.querySelectorAll(".account-billing-modal__balance strong"))
    .every((node) => node.textContent === zh["account.billing.balanceUnavailable"]),
  "real combined-query failure marks both balances unavailable",
);
await act(async () => {
  retryBalanceRender.container.querySelector<HTMLButtonElement>(".account-billing-modal .modal-close-button")?.click();
  await flush();
  retryBalanceRender.container.querySelector<HTMLButtonElement>('[data-account-page="payments"]')?.click();
  await flush();
});
await waitFor("retried balances", () => retryBalanceCalls === 2
  && retryBalanceRender.container.querySelector(".account-billing-modal__balance--subscription strong")?.textContent === "100"
  && retryBalanceRender.container.querySelector(".account-billing-modal__balance--recharge strong")?.textContent === "0");
ok(retryBalanceCalls === 2, "reopening the billing modal retries a failed balance query");
await dispose(retryBalanceRender.root, retryBalanceRender.container);

const lowBalanceView = authenticated("250.0000");
lowBalanceView.balance = {
  total_balance: "250.0000",
  available_balance: "250.0000",
  frozen_balance: "0.0000",
};
const lowBalanceRender = await renderAccount(apiWith({
  CloudBootstrap: async () => lowBalanceView,
}));
await waitFor("low balance account", () => lowBalanceRender.container.querySelector('[data-account-balance]')?.textContent?.includes("250.0000") === true);
const lowBalanceRecharge = lowBalanceRender.container.querySelector<HTMLButtonElement>('[data-account-page="payments"]');
ok(lowBalanceRecharge !== null, "low balance keeps the plans entry available");
await act(async () => {
  lowBalanceRecharge?.click();
  await flush();
});
ok(lowBalanceRender.container.querySelector('[data-subscription-modal][role="dialog"]') !== null, "low balance recharge opens the plans and payments modal");
await dispose(lowBalanceRender.root, lowBalanceRender.container);

const depletedBalanceView = authenticated("0.0000");
depletedBalanceView.balance = {
  total_balance: "0.0000",
  available_balance: "0.0000",
  frozen_balance: "0.0000",
};
const depletedBalanceRender = await renderAccount(apiWith({
  CloudBootstrap: async () => depletedBalanceView,
}));
await waitFor("depleted balance account", () => depletedBalanceRender.container.querySelector('[data-account-balance]')?.textContent?.includes("0.0000") === true);
ok(depletedBalanceRender.container.querySelector('[data-account-page="payments"]') !== null, "depleted balance keeps the plans entry available");
await dispose(depletedBalanceRender.root, depletedBalanceRender.container);

const defaultAvatarRender = await renderAccount(apiWith({
  CloudBootstrap: async () => authenticated("100.00"),
}), "default-avatar");
await waitFor("default avatar bootstrap", () => defaultAvatarRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(
  defaultAvatarRender.container.querySelector<HTMLImageElement>(".account-center__account-identity img")?.src.includes("brand-symbol.png") === true,
  "account identity uses the solid brand symbol when the local avatar is empty",
);
ok(
  defaultAvatarRender.container.querySelector(".account-center__account-identity svg") === null,
  "account identity no longer falls back to the generic user icon",
);
await dispose(defaultAvatarRender.root, defaultAvatarRender.container);

console.log("\naccount center identity fallbacks");

const adminIdentityRender = await renderAccount(apiWith({
  CloudBootstrap: async () => ({
    ...authenticated("100.00"),
    account: {
      ...authenticated("100.00").account,
      display_name: "Admin Display Name",
      role: "admin",
    },
  }),
}), "admin-identity");
await waitFor("admin identity bootstrap", () => adminIdentityRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(adminIdentityRender.container.querySelector('[data-account-identity]')?.textContent === "Admin Display Name", "admin role does not override the cloud display name");
await dispose(adminIdentityRender.root, adminIdentityRender.container);

const cloudIDIdentityRender = await renderAccount(apiWith({
  CloudBootstrap: async () => ({
    ...authenticated("100.00"),
    account: {
      ...authenticated("100.00").account,
      display_name: undefined,
      role: "admin",
    },
  }),
}), "cloud-id-identity");
await waitFor("cloud user ID identity bootstrap", () => cloudIDIdentityRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(cloudIDIdentityRender.container.querySelector('[data-account-identity]')?.textContent === zh["account.nameUnavailable"], "missing display name uses the safe unavailable-name label");
ok(!cloudIDIdentityRender.container.textContent?.includes("usr_01"), "missing display name does not render the cloud user ID as a name");
await dispose(cloudIDIdentityRender.root, cloudIDIdentityRender.container);

const emptyIdentityRender = await renderAccount(apiWith({
  CloudBootstrap: async () => ({
    ...authenticated("100.00"),
    account: {
      ...authenticated("100.00").account,
      cloud_user_id: undefined,
      display_name: undefined,
      role: "user",
    },
  }),
}), "empty-identity");
await waitFor("empty identity bootstrap", () => emptyIdentityRender.container.querySelector('[data-account-state="authenticated"]') !== null);
ok(emptyIdentityRender.container.querySelector('[data-account-identity]')?.textContent === zh["account.nameUnavailable"], "missing display name and cloud user ID use the safe unavailable-name label");
await dispose(emptyIdentityRender.root, emptyIdentityRender.container);

console.log("\naccount center operation epoch");

const refreshes = [
  deferred<CloudBootstrapView>(),
  deferred<CloudBootstrapView>(),
  deferred<CloudBootstrapView>(),
  deferred<CloudBootstrapView>(),
  deferred<CloudBootstrapView>(),
];
let refreshIndex = 0;
const raceRender = await renderAccount(apiWith({
  CloudBootstrap: async () => authenticated("150.00", 1),
  CloudRefreshAssets: async () => refreshes[refreshIndex++].promise,
}), "tab-race");
await waitFor("race bootstrap", () => raceRender.container.textContent?.includes("150.00") === true);

await act(async () => {
  invokeReactClick(raceRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')!);
  await flush();
});
await act(async () => {
  invokeReactClick(raceRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')!);
  await flush();
});
await act(async () => {
  refreshes[1].resolve(authenticated("250.00", 2));
  await flush();
});
await waitFor("newer generation first", () => raceRender.container.textContent?.includes("250.00") === true);

await act(async () => {
  invokeReactClick(raceRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')!);
  await flush();
});
ok(useAccountStore.getState().busy, "latest operation is busy before stale completion");
await act(async () => {
  refreshes[0].resolve(authenticated("99.00", 1));
  await flush();
});
ok(useAccountStore.getState().account?.binding_generation === 2 && useAccountStore.getState().balance?.total_balance === "250.00", "generation 1 completion cannot overwrite generation 2");
ok(useAccountStore.getState().busy, "stale finally cannot clear the latest operation busy state");
await act(async () => {
  refreshes[2].resolve(authenticated("275.00", 2));
  await flush();
});
await waitFor("latest refresh completion", () => raceRender.container.textContent?.includes("275.00") === true);
await act(async () => {
  invokeReactClick(raceRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')!);
  await flush();
});
await act(async () => {
  invokeReactClick(raceRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')!);
  await flush();
});
await act(async () => {
  refreshes[4].resolve(authenticated("300.00", 2));
  await flush();
});
await waitFor("newer refresh before stale error", () => raceRender.container.textContent?.includes("300.00") === true);
await act(async () => {
  refreshes[3].reject(new Error("AUTH_REQUIRED: stale request detail"));
  await flush();
});
ok(useAccountStore.getState().error === null, "stale rejection cannot replace the latest operation error state");
await dispose(raceRender.root, raceRender.container);

console.log("\naccount center context invalidation");

const profileARefresh = deferred<CloudBootstrapView>();
const profileBBootstrap = deferred<CloudBootstrapView>();
const contextRender = await renderAccount(apiWith({
  CloudBootstrap: async () => authenticated("310.00", 1, "profile-a"),
  CloudRefreshAssets: async () => profileARefresh.promise,
}), "tab-a");
await waitFor("profile A assets", () => contextRender.container.textContent?.includes("310.00") === true);
await act(async () => {
  contextRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')?.click();
  await flush();
});
await act(async () => {
  contextRender.root.render(
    <LocaleProvider>
      <AccountCenter
        key="tab-b"
        api={apiWith({ CloudBootstrap: async () => profileBBootstrap.promise })}
        contextKey="tab-b"
      />
    </LocaleProvider>,
  );
  await flush();
});
ok(contextRender.container.textContent?.includes("310.00") === false, "profile remount hides old assets before the new bootstrap resolves");
ok(useAccountStore.getState().account === null && useAccountStore.getState().balance === null, "profile remount clears the old Zustand scope before bootstrap");
await act(async () => {
  profileARefresh.resolve(authenticated("999.00", 1, "profile-a"));
  await flush();
});
ok(contextRender.container.textContent?.includes("999.00") === false, "unmounted profile completion cannot publish into the new context");
ok(useAccountStore.getState().busy, "unmounted profile finally cannot clear the new bootstrap busy state");
await act(async () => {
  profileBBootstrap.resolve(authenticated("420.00", 1, "profile-b"));
  await flush();
});
await waitFor("profile B assets", () => contextRender.container.textContent?.includes("420.00") === true);
await dispose(contextRender.root, contextRender.container);

console.log("\naccount center session cache");

useAccountStore.getState().reset();
const cachedContainer = document.createElement("div");
document.body.append(cachedContainer);
const cachedRoot = createRoot(cachedContainer);
await act(async () => {
  cachedRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => authenticated("510.00", 1, "profile-cache") })} contextKey="tab-cache" />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("cached account bootstrap", () => cachedContainer.textContent?.includes("510.00") === true);
await dispose(cachedRoot, cachedContainer);

const cachedRefresh = deferred<CloudBootstrapView>();
const restoredContainer = document.createElement("div");
document.body.append(restoredContainer);
const restoredRoot = createRoot(restoredContainer);
await act(async () => {
  restoredRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => cachedRefresh.promise })} contextKey="tab-cache" />
    </LocaleProvider>,
  );
  await flush();
});
ok(restoredContainer.textContent?.includes("510.00") === true, "same context renders cached assets before bootstrap resolves");
ok(restoredContainer.querySelector('[data-account-state="authenticated"]') !== null, "same context immediately restores the authenticated account");
ok(restoredContainer.textContent?.includes("当前构建未启用云功能") === false, "cached account never flashes the unavailable message");
await act(async () => {
  cachedRefresh.resolve(authenticated("525.00", 1, "profile-cache"));
  await flush();
});
await waitFor("cached account revalidation", () => restoredContainer.textContent?.includes("525.00") === true);
await dispose(restoredRoot, restoredContainer);

const failedRefresh = deferred<CloudBootstrapView>();
const failedContainer = document.createElement("div");
document.body.append(failedContainer);
const failedRoot = createRoot(failedContainer);
await act(async () => {
  failedRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => failedRefresh.promise })} contextKey="tab-cache" />
    </LocaleProvider>,
  );
  await flush();
});
ok(failedContainer.textContent?.includes("525.00") === true, "cached assets remain visible during a failing revalidation");
await act(async () => {
  failedRefresh.reject(new Error("CLOUD_UNAVAILABLE: cached refresh failed"));
  await flush();
});
await waitFor("cached refresh error", () => failedContainer.querySelector('[role="alert"]') !== null);
ok(failedContainer.querySelector('[data-account-state="authenticated"]') !== null, "cached account remains recoverable after revalidation fails");
ok(failedContainer.textContent?.includes("525.00") === true, "failed revalidation preserves cached assets");
await dispose(failedRoot, failedContainer);

const partialRefresh = deferred<CloudBootstrapView>();
const partialContainer = document.createElement("div");
document.body.append(partialContainer);
const partialRoot = createRoot(partialContainer);
await act(async () => {
  partialRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => partialRefresh.promise })} contextKey="tab-cache" />
    </LocaleProvider>,
  );
  await flush();
});
ok(partialContainer.textContent?.includes("525.00") === true, "resolved partial revalidation starts from the stable cached assets");
await act(async () => {
  partialRefresh.resolve({
    available: true,
    capabilities: { registration: true, registration_mode: "fixture", payments: true, realtime: true },
    account: { ...authenticated("525.00", 1, "profile-cache").account, status: "offline" },
    balance: { total_balance: "530.00" },
    operation_error: { code: "NETWORK_UNAVAILABLE", message: "cloud network is unavailable" },
  });
  await flush();
});
await waitFor("resolved partial refresh error", () => partialContainer.querySelector('[role="alert"]') !== null);
ok(partialContainer.textContent?.includes("530.00") === true, "resolved partial revalidation merges non-empty assets live");
ok(partialContainer.textContent?.includes("Starter") === true, "resolved partial revalidation preserves the stable subscription");
await dispose(partialRoot, partialContainer);

const partialRestore = deferred<CloudBootstrapView>();
const partialRestoreContainer = document.createElement("div");
document.body.append(partialRestoreContainer);
const partialRestoreRoot = createRoot(partialRestoreContainer);
await act(async () => {
  partialRestoreRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => partialRestore.promise })} contextKey="tab-cache" />
    </LocaleProvider>,
  );
  await flush();
});
ok(partialRestoreContainer.textContent?.includes("525.00") === true, "re-entry restores the last stable balance instead of partial live data");
ok(partialRestoreContainer.querySelector('[role="alert"]') === null, "re-entry does not restore the transient partial error");
await dispose(partialRestoreRoot, partialRestoreContainer);

useAccountStore.getState().reset();
const firstPartialContainer = document.createElement("div");
document.body.append(firstPartialContainer);
const firstPartialRoot = createRoot(firstPartialContainer);
await act(async () => {
  firstPartialRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({
        CloudBootstrap: async () => ({
          available: true,
          capabilities: { registration: true, registration_mode: "fixture", payments: true, realtime: true },
          account: authenticated("42.00", 1, "profile-partial", "usr_partial").account,
          balance: { total_balance: "42.00" },
          operation_error: { code: "NETWORK_UNAVAILABLE", message: "cloud network is unavailable" },
        }),
      })} contextKey="tab-partial" />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("first partial bootstrap", () => firstPartialContainer.querySelector('[role="alert"]') !== null);
ok(firstPartialContainer.textContent?.includes("42.00") === true, "first partial bootstrap exposes its available balance live");
await dispose(firstPartialRoot, firstPartialContainer);

const firstPartialRetry = deferred<CloudBootstrapView>();
const firstPartialRetryContainer = document.createElement("div");
document.body.append(firstPartialRetryContainer);
const firstPartialRetryRoot = createRoot(firstPartialRetryContainer);
await act(async () => {
  firstPartialRetryRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => firstPartialRetry.promise })} contextKey="tab-partial" />
    </LocaleProvider>,
  );
  await flush();
});
ok(firstPartialRetryContainer.querySelector("[data-account-loading]") !== null, "re-entry after a first partial bootstrap starts from loading");
ok(firstPartialRetryContainer.textContent?.includes("usr_partial") === false, "re-entry does not restore a partial identity shell");
await dispose(firstPartialRetryRoot, firstPartialRetryContainer);

const coldBootstrap = deferred<CloudBootstrapView>();
const coldContainer = document.createElement("div");
document.body.append(coldContainer);
const coldRoot = createRoot(coldContainer);
await act(async () => {
  coldRoot.render(
    <LocaleProvider>
      <AccountCenter api={apiWith({ CloudBootstrap: async () => coldBootstrap.promise })} contextKey="tab-cold" />
    </LocaleProvider>,
  );
  await flush();
});
ok(coldContainer.querySelector("[data-account-loading]") !== null, "uncached context renders a loading state while bootstrap is pending");
ok(coldContainer.textContent?.includes("当前构建未启用云功能") === false, "uncached context does not treat pending bootstrap as unavailable");
await act(async () => {
  coldBootstrap.resolve(anonymous);
  await flush();
});
await waitFor("cold account bootstrap", () => coldContainer.querySelector('[data-account-state="anonymous"]') !== null);
await dispose(coldRoot, coldContainer);

console.log("\naccount center unavailable state");

const errorRender = await renderAccount(apiWith({
  CloudBootstrap: async () => {
    throw new Error("CLOUD_UNAVAILABLE: cloud features are unavailable");
  },
}));
await waitFor("unavailable error", () => errorRender.container.querySelector('[role="alert"]') !== null);
ok(errorRender.container.textContent?.includes("CLOUD_UNAVAILABLE") === true, "typed unavailable code is visible");
ok(errorRender.container.querySelector('[data-account-action="login"]') === null, "unavailable state does not expose a dead login form");
await dispose(errorRender.root, errorRender.container);
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
