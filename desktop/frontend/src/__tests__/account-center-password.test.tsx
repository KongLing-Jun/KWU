// Run: tsx src/__tests__/account-center-password.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";

import { AccountCenter, type AccountCloudBindings } from "../components/AccountCenter";
import { LocaleProvider } from "../lib/i18n";
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

const authenticated: CloudBootstrapView = {
  available: true,
  capabilities: { registration: true, registration_mode: "fixture", payments: true, realtime: true },
  account: {
    status: "authenticated",
    profile_id: "default",
    cloud_user_id: "usr_01",
    binding_generation: 1,
    display_name: "Fixture User",
  },
  balance: { total_balance: "100.00" },
  subscription: {
    plans: [],
    current: { plan: "free", status: "free", updated_at: "2026-07-11T08:00:00Z" },
  },
};

console.log("\naccount center password lifetime");

const loginPasswords: string[] = [];
const registerPasswords: string[] = [];
let registerAttempts = 0;
const api: AccountCloudBindings = {
  CloudBootstrap: async () => anonymous,
  CloudCaptchaConfig: async () => { throw new Error("real CAPTCHA is outside this test"); },
  CloudSendRegisterSMS: async () => undefined,
  CloudSendPasswordResetSMS: async () => undefined,
  CloudResetPassword: async () => undefined,
  CloudRegister: async (request) => {
    registerPasswords.push(request.password);
    registerAttempts += 1;
    if (registerAttempts === 1) {
      throw { code: "AUTH_INVALID", message: `registration rejected: ${JSON.stringify(request)}` };
    }
    return anonymous;
  },
  CloudLogin: async (request) => {
    loginPasswords.push(request.password);
    return anonymous;
  },
  CloudSwitchAccount: async () => authenticated,
  CloudLogout: async () => anonymous,
  CloudRefreshAssets: async () => authenticated,
  CloudCreateSubscriptionOrder: async () => { throw new Error("payment create is outside this test"); },
  CloudCreateUpgradeOrder: async () => { throw new Error("payment upgrade is outside this test"); },
  CloudConfirmPayment: async () => { throw new Error("payment confirm is outside this test"); },
  CloudRefreshPayment: async () => { throw new Error("payment refresh is outside this test"); },
};

const dom = installDom();
const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);

await act(async () => {
  root.render(
    <LocaleProvider>
      <AccountCenter api={api} />
    </LocaleProvider>,
  );
  await flush();
});

const phone = document.querySelector<HTMLInputElement>('input[name="phone"]');
const password = document.querySelector<HTMLInputElement>('input[name="password"]');
if (!phone || !password) throw new Error("missing account inputs");

const loginSecret = "test-only-login-secret";
await act(async () => {
  setInput(phone, "13800000000");
  setInput(password, loginSecret);
  await flush();
});
await act(async () => {
  document.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});

ok(loginPasswords[0] === loginSecret, `login forwards the local password to the bridge once (captured length ${loginPasswords[0]?.length ?? -1})`);
ok(password.value === "", "successful login clears the password in finally");
ok(!JSON.stringify(useAccountStore.getState()).includes(loginSecret), "login password never enters the Zustand store");
ok(!JSON.stringify({ ...localStorage }).includes(loginSecret), "login password never enters localStorage");

await act(async () => {
  document.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});

const registerPassword = document.querySelector<HTMLInputElement>('input[name="password"]');
const smsCode = document.querySelector<HTMLInputElement>('input[name="sms_code"]');
if (!registerPassword || !smsCode) throw new Error("missing registration inputs");
const registerSecret = "test-only-register-secret";

await act(async () => {
  setInput(registerPassword, registerSecret);
  setInput(smsCode, "123456");
  await flush();
});
await act(async () => {
  document.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
  await flush();
});

ok(registerPasswords[0] === registerSecret, `registration forwards the local password to the bridge once (captured length ${registerPasswords[0]?.length ?? -1})`);
ok(registerPassword.value === "", "failed registration clears the password in finally");
ok(phone.value === "13800000000", "failed registration preserves the phone for correction");
ok(smsCode.value === "123456", "failed registration preserves the SMS code for correction");
const registrationError = document.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(!registrationError.includes(JSON.stringify({ phone: "13800000000", password: registerSecret, sms_code: "123456" })), "registration error never renders the request payload");
ok(!registrationError.includes(registerSecret), "registration error never renders the submitted password");
ok(!JSON.stringify(useAccountStore.getState()).includes(registerSecret), "registration password never enters the Zustand store");
ok(!JSON.stringify({ ...localStorage }).includes(registerSecret), "registration password never enters localStorage");

const successfulSecret = "test-only-success-secret";
await act(async () => {
  setInput(registerPassword, successfulSecret);
  await flush();
});
await act(async () => {
  document.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
  await flush();
});

ok(registerPasswords[1] === successfulSecret, `successful registration forwards the local password once (captured length ${registerPasswords[1]?.length ?? -1})`);
ok(registerPassword.value === "", "successful registration clears the password in finally");
ok(smsCode.value === "", "successful registration clears the SMS code");
ok(phone.value === "13800000000", "successful registration does not retain extra secret state beyond the editable phone");
ok(!JSON.stringify(useAccountStore.getState()).includes(successfulSecret), "successful registration password never enters the Zustand store");

await act(async () => {
  root.unmount();
});
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
