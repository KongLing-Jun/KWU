// Run: tsx src/__tests__/account-center-switch.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";

import { AccountCenter, type AccountCloudBindings } from "../components/AccountCenter";
import type { AliyunCaptchaSDKOptions } from "../lib/aliyunCaptcha";
import { app, type CloudLoginRequest } from "../lib/bridge";
import { LocaleProvider } from "../lib/i18n";
import {
  type CloudBootstrapView,
  type CloudPaymentView,
  useAccountStore,
} from "../store/account";

type SwitchBindings = AccountCloudBindings & {
  CloudSwitchAccount(request: CloudLoginRequest): Promise<CloudBootstrapView>;
};

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

function eq(actual: unknown, expected: unknown, label: string) {
  ok(JSON.stringify(actual) === JSON.stringify(expected), `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
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
  globalThis.InputEvent = dom.window.InputEvent;
  globalThis.MouseEvent = dom.window.MouseEvent;
  globalThis.localStorage = dom.window.localStorage;
  globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
  globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);
  return dom;
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
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

function invokeReactClick(button: HTMLButtonElement) {
  const reactPropsKey = Object.keys(button).find((key) => key.startsWith("__reactProps$"));
  const reactProps = reactPropsKey
    ? (button as unknown as Record<string, { onClick?: () => void }>)[reactPropsKey]
    : undefined;
  if (!reactProps?.onClick) throw new Error("missing React click handler");
  reactProps.onClick();
}

function required<T extends Element>(container: ParentNode, selector: string): T {
  const element = container.querySelector<T>(selector);
  if (!element) throw new Error(`missing ${selector}`);
  return element;
}

function payment(orderID: string): CloudPaymentView {
  return {
    order_id: orderID,
    order_type: "subscription_purchase",
    plan: "pro",
    amount: "59.90",
    amount_unit: "yuan",
    currency: "CNY",
    payment_channel: "alipay",
    state: "paying",
    payable: true,
    entitlement_granted: false,
    expires_at: "2030-01-01T00:30:00Z",
    payment_entry: {
      entry_type: "qr_code",
      qr_code: `https://qr.test/${orderID}`,
      expires_at: "2030-01-01T00:30:00Z",
    },
  };
}

function authenticated(
  cloudUserID: string,
  generation: number,
  balance: string,
  orderID: string,
): CloudBootstrapView {
  return {
    available: true,
    capabilities: { registration: true, registration_mode: "fixture", payments: true, realtime: true },
    account: {
      status: "authenticated",
      profile_id: "default",
      cloud_user_id: cloudUserID,
      binding_generation: generation,
      display_name: cloudUserID === "usr_01" ? "Current A" : "Target B",
      last_sync_at: "2026-07-12T08:00:00Z",
    },
    balance: { total_balance: balance },
    subscription: {
      plans: [],
      current: {
        plan: cloudUserID === "usr_01" ? "lite" : "pro",
        status: "active",
        updated_at: "2026-07-12T08:00:00Z",
      },
    },
    payments: [payment(orderID)],
  };
}

const viewA = authenticated("usr_01", 1, "100.00", "order-a");
const viewB = authenticated("usr_02", 2, "200.00", "order-b");
const candidateEnvelope: CloudBootstrapView = {
  ...viewA,
  operation_error: { code: "PROFILE_SCOPE_MISMATCH", message: "account belongs to another profile" },
  candidate: { ...viewB.account, status: "offline" },
};
const realCapabilities = { registration: true, registration_mode: "real" as const, payments: false, realtime: false };
const realViewA: CloudBootstrapView = { ...viewA, capabilities: realCapabilities };
const realViewB: CloudBootstrapView = { ...viewB, capabilities: realCapabilities };
const realCandidateEnvelope: CloudBootstrapView = { ...candidateEnvelope, capabilities: realCapabilities };

function apiWith(overrides: Partial<SwitchBindings> = {}): SwitchBindings {
  return {
    CloudBootstrap: async () => viewA,
    CloudCaptchaConfig: async () => { throw new Error("real CAPTCHA is outside this test"); },
    CloudSendRegisterSMS: async () => undefined,
    CloudSendPasswordResetSMS: async () => undefined,
    CloudResetPassword: async () => undefined,
    CloudRegister: async () => viewA,
    CloudLogin: async () => candidateEnvelope,
    CloudSwitchAccount: async () => viewB,
    CloudLogout: async () => ({ available: true, account: { ...viewA.account, status: "anonymous" } }),
    CloudRefreshAssets: async () => viewA,
    CloudCreateSubscriptionOrder: async () => { throw new Error("payment create is outside this test"); },
    CloudCreateUpgradeOrder: async () => { throw new Error("payment upgrade is outside this test"); },
    CloudConfirmPayment: async () => { throw new Error("payment confirm is outside this test"); },
    CloudRefreshPayment: async () => { throw new Error("payment refresh is outside this test"); },
    ...overrides,
  };
}

async function renderAccount(api: SwitchBindings): Promise<{ container: HTMLElement; root: Root }> {
  useAccountStore.getState().reset();
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AccountCenter api={api} />
      </LocaleProvider>,
    );
    await flush();
  });
  await waitFor("authenticated account", () => useAccountStore.getState().account?.cloud_user_id === "usr_01");
  return { container, root };
}

async function dispose(rendered: { container: HTMLElement; root: Root }) {
  await act(async () => rendered.root.unmount());
  rendered.container.remove();
}

async function openSwitch(container: HTMLElement) {
  await act(async () => {
    required<HTMLButtonElement>(container, '[data-account-action="switch-open"]').click();
    await flush();
  });
  required(container, '[data-account-switch-stage="credentials"]');
}

async function fillAndVerify(container: HTMLElement, password: string) {
  await act(async () => {
    setInput(required(container, 'input[name="switch_phone"]'), "13900000000");
    setInput(required(container, 'input[name="switch_password"]'), password);
    await flush();
  });
  await act(async () => {
    invokeReactClick(required(container, '[data-account-action="switch-verify"]'));
    await flush();
  });
}

console.log("\naccount center explicit account switch");

const dom = installDom();

let browserSwitchError: unknown;
try {
  await (app as SwitchBindings).CloudSwitchAccount({ phone: "13900000000", password: "browser-secret" });
} catch (cause) {
  browserSwitchError = cause;
}
ok(
  browserSwitchError instanceof Error && browserSwitchError.message.includes("browser mock data is disabled"),
  "account switching requires the live Wails desktop binding",
);

console.log("\nfixture account switch requires candidate confirmation");

const fixtureLoginRequests: CloudLoginRequest[] = [];
const fixtureSwitchRequests: CloudLoginRequest[] = [];
const fixtureRender = await renderAccount(apiWith({
  CloudLogin: async (request) => {
    fixtureLoginRequests.push(request);
    return candidateEnvelope;
  },
  CloudSwitchAccount: async (request) => {
    fixtureSwitchRequests.push(request);
    return viewB;
  },
}));
await openSwitch(fixtureRender.container);
await fillAndVerify(fixtureRender.container, "fixture-verify-secret");
await waitFor("fixture switch confirmation", () => fixtureRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null);
await act(async () => {
  setInput(required(fixtureRender.container, 'input[name="switch_confirm_password"]'), "fixture-confirm-secret");
  await flush();
});
await act(async () => {
  invokeReactClick(required(fixtureRender.container, '[data-account-action="switch-confirm"]'));
  await flush();
});
await waitFor("fixture switched account", () => useAccountStore.getState().account?.cloud_user_id === "usr_02");
eq(fixtureLoginRequests, [{ phone: "13900000000", password: "fixture-verify-secret" }], "fixture switch verifies the candidate through CloudLogin");
eq(fixtureSwitchRequests, [{ phone: "13900000000", password: "fixture-confirm-secret" }], "fixture confirmation calls CloudSwitchAccount once");
ok(fixtureRender.container.querySelector('[data-account-switch-stage="confirm"]') === null, "fixture confirmation closes after success");
await dispose(fixtureRender);

console.log("\nreal account switch verification");

let realSDKOptions: AliyunCaptchaSDKOptions | undefined;
(window as typeof window & { initAliyunCaptcha?: (options: AliyunCaptchaSDKOptions) => void }).initAliyunCaptcha = (options) => {
  realSDKOptions = options;
  options.getInstance?.({ destroy: () => undefined });
};
const realSwitchRequests: CloudLoginRequest[] = [];
const realLoginRequests: CloudLoginRequest[] = [];
const realRender = await renderAccount(apiWith({
  CloudBootstrap: async () => realViewA,
  CloudCaptchaConfig: async () => ({ provider: "aliyun_captcha_v2", region: "cn", prefix: "public-prefix", scene_id: "register-scene" }),
  CloudLogin: async (request) => {
    realLoginRequests.push(request);
    return realCandidateEnvelope;
  },
  CloudSwitchAccount: async (request) => {
    realSwitchRequests.push(request);
    return realViewB;
  },
}));
await openSwitch(realRender.container);
await act(async () => {
  setInput(required(realRender.container, 'input[name="switch_phone"]'), "13900000000");
  setInput(required(realRender.container, 'input[name="switch_password"]'), "real-verify-secret");
  await flush();
});
await waitFor("real switch CAPTCHA ready", () => realRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
const switchProof = `{"token":"switch-proof"}`;
await act(async () => {
  required<HTMLButtonElement>(realRender.container, '[data-account-action="switch-verify"]').click();
  realSDKOptions?.success?.(switchProof);
  await flush();
});
await waitFor("real switch confirmation", () => realRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null);
await act(async () => {
  setInput(required(realRender.container, 'input[name="switch_confirm_password"]'), "real-confirm-secret");
  await flush();
});
await waitFor("real confirmation CAPTCHA ready", () => realRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
const confirmProof = `{"token":"confirm-proof"}`;
await act(async () => {
  required<HTMLButtonElement>(realRender.container, '[data-account-action="switch-confirm"]').click();
  realSDKOptions?.success?.(confirmProof);
  await flush();
});
await waitFor("real switched account", () => useAccountStore.getState().account?.cloud_user_id === "usr_02");
eq(realLoginRequests, [{
  phone: "13900000000",
  password: "real-verify-secret",
  captcha_ticket: switchProof,
  captcha_provider: "aliyun_captcha_v2",
}], "real switch verifies the candidate with one raw proof");
eq(realSwitchRequests, [{
  phone: "13900000000",
  password: "real-confirm-secret",
  captcha_ticket: confirmProof,
  captcha_provider: "aliyun_captcha_v2",
}], "real confirmation reauthenticates through CloudSwitchAccount");
ok(!JSON.stringify(useAccountStore.getState()).includes(switchProof), "real switch proof never enters Zustand");
ok(!JSON.stringify({ ...localStorage }).includes(switchProof), "real switch proof never enters localStorage");
ok(!JSON.stringify(useAccountStore.getState()).includes(confirmProof), "real confirmation proof never enters Zustand");
ok(!JSON.stringify({ ...localStorage }).includes(confirmProof), "real confirmation proof never enters localStorage");
await dispose(realRender);

console.log("\ncancel closes switch UI without changing A");

let cancelSwitchCalls = 0;
const cancelRender = await renderAccount(apiWith({
  CloudSwitchAccount: async () => {
    cancelSwitchCalls += 1;
    return viewB;
  },
}));
await openSwitch(cancelRender.container);
await act(async () => {
  required<HTMLButtonElement>(cancelRender.container, '[data-account-action="switch-cancel"]').click();
  await flush();
});
ok(cancelSwitchCalls === 0, "cancel never calls CloudSwitchAccount");
ok(cancelRender.container.querySelector("[data-account-switch-stage]") === null, "cancel closes and clears switch UI");
ok(useAccountStore.getState().balance?.total_balance === "100.00", "cancel preserves A balance");
eq(useAccountStore.getState().payments.map((item) => item.order_id), ["order-a"], "cancel preserves A orders");
await dispose(cancelRender);

console.log("\nconfirmed switch is atomic and isolates late A operations");

const lateAView = deferred<CloudBootstrapView>();
const switchCompletion = deferred<CloudBootstrapView>();
const switchRequests: CloudLoginRequest[] = [];
const successRender = await renderAccount(apiWith({
  CloudRefreshAssets: async () => lateAView.promise,
  CloudSwitchAccount: async (request) => {
    switchRequests.push(request);
    return switchCompletion.promise;
  },
}));
await act(async () => {
  useAccountStore.getState().setError({ code: "OLD_A_ERROR", message: "old A error" });
  await flush();
});
await act(async () => {
  required<HTMLButtonElement>(successRender.container, '[data-account-page="payments"]').click();
  await flush();
});
ok(successRender.container.querySelector('[data-account-page-panel="payments"]') !== null, "payment page is mounted before switch flow opens");
await act(async () => {
  invokeReactClick(required(successRender.container, '[data-account-action="refresh"]'));
  await flush();
});
await act(async () => {
  invokeReactClick(required(successRender.container, '[data-account-action="switch-open"]'));
  await flush();
});
ok(successRender.container.querySelector('[data-account-page-panel="payments"]') === null, "opening switch flow unmounts scoped payment polling UI");
const verifyPassword = "verify-switch-secret";
await fillAndVerify(successRender.container, verifyPassword);
await waitFor("success switch confirmation", () => successRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null);
const switchPassword = "confirm-switch-secret";
await act(async () => {
  setInput(required(successRender.container, 'input[name="switch_confirm_password"]'), switchPassword);
  await flush();
});
await act(async () => {
  invokeReactClick(required(successRender.container, '[data-account-action="switch-confirm"]'));
  await flush();
});
eq(switchRequests, [{ phone: "13900000000", password: switchPassword }], "confirmation calls CloudSwitchAccount exactly once");
ok(required<HTMLInputElement>(successRender.container, 'input[name="switch_confirm_password"]').value === "", "confirmation password clears immediately while request is pending");
ok(!JSON.stringify(useAccountStore.getState()).includes(switchPassword), "confirmation password never enters Zustand");
ok(!JSON.stringify({ ...localStorage }).includes(switchPassword), "confirmation password never enters localStorage");
ok(required<HTMLButtonElement>(successRender.container, '[data-account-action="switch-cancel"]').disabled, "switch request cannot be canceled after backend mutation starts");
ok(required<HTMLButtonElement>(successRender.container, '[data-account-action="refresh"]').disabled, "switch request disables refresh");
ok(required<HTMLButtonElement>(successRender.container, '[data-account-action="logout"]').disabled, "switch request disables logout");
ok(
  [...successRender.container.querySelectorAll<HTMLButtonElement>("[data-account-page]")].every((button) => button.disabled),
  "switch request disables account page navigation",
);
await act(async () => {
  switchCompletion.resolve(viewB);
  await flush();
});
await waitFor("authoritative B scope", () => useAccountStore.getState().account?.cloud_user_id === "usr_02");
eq(useAccountStore.getState().scopeKey, "default\u0000usr_02\u00002", "successful switch installs B scope");
ok(useAccountStore.getState().balance?.total_balance === "200.00", "successful switch replaces A balance with B balance");
eq(useAccountStore.getState().payments.map((item) => item.order_id), ["order-b"], "successful switch replaces A orders with B orders");
ok(useAccountStore.getState().error === null, "successful switch hides the old A error");
ok(successRender.container.querySelector("[data-account-switch-stage]") === null, "scope change clears switch-local UI");
await act(async () => {
  lateAView.resolve({ ...viewA, operation_error: { code: "LATE_A_VIEW", message: "late A view" } });
  await flush();
});
ok(useAccountStore.getState().account?.cloud_user_id === "usr_02", "late A view cannot replace B account");
ok(useAccountStore.getState().balance?.total_balance === "200.00", "late A view cannot replace B assets");
eq(useAccountStore.getState().payments.map((item) => item.order_id), ["order-b"], "late A view cannot restore A orders");
ok(useAccountStore.getState().error === null, "late A completion cannot write into B error state");
await dispose(successRender);

console.log("\npost-commit partial B view replaces A without retrying switch");

const partialB: CloudBootstrapView = {
  available: true,
  account: viewB.account,
  balance: null,
  subscription: null,
  payments: [],
  operation_error: {
    code: "NETWORK_UNAVAILABLE",
    message: "cloud network is unavailable",
  },
};
let partialSwitchCalls = 0;
const partialRender = await renderAccount(apiWith({
  CloudSwitchAccount: async () => {
    partialSwitchCalls += 1;
    return partialB;
  },
}));
await openSwitch(partialRender.container);
await fillAndVerify(partialRender.container, "partial-verify-secret");
await waitFor("partial switch confirmation", () => partialRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null);
await act(async () => {
  setInput(required(partialRender.container, 'input[name="switch_confirm_password"]'), "partial-confirm-secret");
  await flush();
});
await act(async () => {
  invokeReactClick(required(partialRender.container, '[data-account-action="switch-confirm"]'));
  await flush();
});
await waitFor("partial authoritative B", () => (
  useAccountStore.getState().account?.cloud_user_id === "usr_02"
  && !useAccountStore.getState().busy
));
ok(partialSwitchCalls === 1, "partial authoritative view does not retry CloudSwitchAccount");
ok(partialRender.container.querySelector("[data-account-switch-stage]") === null, "partial authoritative B closes local switch flow");
ok(useAccountStore.getState().balance === null && useAccountStore.getState().subscription === null, "partial B clears A assets");
eq(useAccountStore.getState().payments, [], "partial B clears A orders");
eq(useAccountStore.getState().error, partialB.operation_error, "partial B preserves the safe hydration error");
ok(!partialRender.container.textContent?.includes("100.00"), "partial B no longer renders A balance");
await dispose(partialRender);

console.log("\nfailed switch retains candidate confirmation for retry");

let retryCalls = 0;
const retryRender = await renderAccount(apiWith({
  CloudSwitchAccount: async () => {
    retryCalls += 1;
    if (retryCalls === 1) throw { code: "PROFILE_SCOPE_MISMATCH", message: "binding changed after preparation" };
    return viewB;
  },
}));
await openSwitch(retryRender.container);
await fillAndVerify(retryRender.container, "retry-verify-secret");
await waitFor("retry switch confirmation", () => retryRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null);
await act(async () => {
  setInput(required(retryRender.container, 'input[name="switch_confirm_password"]'), "retry-failed-secret");
  await flush();
});
await act(async () => {
  invokeReactClick(required(retryRender.container, '[data-account-action="switch-confirm"]'));
  await flush();
});
await waitFor("switch failure", () => useAccountStore.getState().error?.code === "PROFILE_SCOPE_MISMATCH");
ok(retryRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null, "failed switch retains the candidate confirmation");
ok(required<HTMLInputElement>(retryRender.container, 'input[name="switch_confirm_password"]').value === "", "failed switch clears the confirmation password");
ok(retryRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent?.includes("PROFILE_SCOPE_MISMATCH") !== true, "stale switch hides the internal mismatch code");
ok(useAccountStore.getState().error?.message !== "binding changed after preparation", "stale switch replaces the backend message with recovery guidance");
await act(async () => {
  setInput(required(retryRender.container, 'input[name="switch_confirm_password"]'), "retry-success-secret");
  await flush();
});
await act(async () => {
  invokeReactClick(required(retryRender.container, '[data-account-action="switch-confirm"]'));
  await flush();
});
await waitFor("retry B scope", () => useAccountStore.getState().account?.cloud_user_id === "usr_02");
await waitFor("retry switch UI completion", () => (
  retryRender.container.querySelector("[data-account-switch-stage]") === null
  && !useAccountStore.getState().busy
));
ok(retryCalls === 2, "failed confirmation can be retried without re-verifying the candidate");
await dispose(retryRender);

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
