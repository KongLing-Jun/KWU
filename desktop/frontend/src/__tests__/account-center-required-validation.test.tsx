// Run: tsx src/__tests__/account-center-required-validation.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";

import { AccountCenter, type AccountCloudBindings } from "../components/AccountCenter";
import type { AliyunCaptchaSDKOptions } from "../lib/aliyunCaptcha";
import type { CloudBootstrapView, CloudCapabilitiesView } from "../store/account";
import { useAccountStore } from "../store/account";
import { LocaleProvider } from "../lib/i18n";

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
  Object.defineProperty(dom.window.navigator, "language", { configurable: true, value: "zh-CN" });
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

function required<T extends Element>(container: HTMLElement, selector: string): T {
  const element = container.querySelector<T>(selector);
  if (!element) throw new Error(`missing ${selector}`);
  return element;
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

const fixtureCapabilities: CloudCapabilitiesView = {
  registration: true,
  registration_mode: "fixture",
  payments: false,
  realtime: false,
};
const realCapabilities: CloudCapabilitiesView = {
  registration: true,
  registration_mode: "real",
  payments: false,
  realtime: false,
};

function anonymous(capabilities: CloudCapabilitiesView): CloudBootstrapView {
  return {
    available: true,
    capabilities,
    account: { status: "anonymous", profile_id: "default" },
  };
}

function apiWith(
  capabilities: CloudCapabilitiesView,
  overrides: Partial<AccountCloudBindings> = {},
): AccountCloudBindings {
  return {
    CloudBootstrap: async () => anonymous(capabilities),
    CloudCaptchaConfig: async () => ({
      provider: "aliyun_captcha_v2",
      region: "cn",
      prefix: "public-prefix",
      scene_id: "account-required-validation",
    }),
    CloudSendRegisterSMS: async () => undefined,
    CloudSendPasswordResetSMS: async () => undefined,
    CloudResetPassword: async () => undefined,
    CloudRegister: async () => anonymous(capabilities),
    CloudLogin: async () => anonymous(capabilities),
    CloudSwitchAccount: async () => anonymous(capabilities),
    CloudLogout: async () => anonymous(capabilities),
    CloudRefreshAssets: async () => anonymous(capabilities),
    CloudCreateSubscriptionOrder: async () => { throw new Error("payment create is outside this test"); },
    CloudCreateUpgradeOrder: async () => { throw new Error("payment upgrade is outside this test"); },
    CloudConfirmPayment: async () => { throw new Error("payment confirm is outside this test"); },
    CloudRefreshPayment: async () => { throw new Error("payment refresh is outside this test"); },
    ...overrides,
  };
}

async function renderAccount(api: AccountCloudBindings) {
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
  await waitFor("account bootstrap", () => container.querySelector('[data-account-state="anonymous"]') !== null);
  return { container, root };
}

async function dispose(root: Root, container: HTMLElement) {
  await act(async () => root.unmount());
  container.remove();
}

console.log("\naccount center required field validation");

const dom = installDom();

let fixtureLoginCalls = 0;
let fixtureRegisterCalls = 0;
let fixtureSMSCalls = 0;
const fixtureRender = await renderAccount(apiWith(fixtureCapabilities, {
  CloudLogin: async () => {
    fixtureLoginCalls += 1;
    throw { code: "LOGIN_FAILED", message: "fixture login failed" };
  },
  CloudRegister: async () => {
    fixtureRegisterCalls += 1;
    throw { code: "REGISTER_FAILED", message: "fixture registration failed" };
  },
  CloudSendRegisterSMS: async () => {
    fixtureSMSCalls += 1;
  },
}));
const fixturePhone = required<HTMLInputElement>(fixtureRender.container, 'input[name="phone"]');
const fixturePassword = required<HTMLInputElement>(fixtureRender.container, 'input[name="password"]');
const fixtureLogin = required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="login"]');
await act(async () => {
  fixtureLogin.click();
  await flush();
});
ok(fixtureLoginCalls === 0, "empty fixture login never calls CloudLogin");
ok(fixtureRender.container.textContent?.includes("手机号不能为空") === true, "empty login shows the phone error");
ok(fixtureRender.container.textContent?.includes("密码不能为空") === true, "empty login shows the password error");
ok(document.activeElement === fixturePhone, "empty login focuses the phone first");
ok(fixturePhone.required && fixturePassword.required, "login inputs expose required semantics");
ok(fixturePhone.getAttribute("aria-invalid") === "true", "invalid phone exposes aria-invalid");
ok(fixturePhone.getAttribute("aria-describedby") === "account-phone-error", "phone error has a stable description link");
ok(fixtureRender.container.querySelectorAll('.account-center__required[aria-hidden="true"]').length === 2, "login renders two decorative required marks");

await act(async () => {
  setInput(fixturePhone, "  ");
  setInput(fixturePassword, " ");
  await flush();
});
ok(fixturePhone.getAttribute("aria-invalid") === "true", "whitespace phone keeps its error");
ok(fixturePassword.getAttribute("aria-invalid") === "false", "non-empty whitespace password clears its error");
await act(async () => {
  setInput(fixturePhone, "13800000000");
  await flush();
});
ok(fixturePhone.getAttribute("aria-invalid") === "false", "non-empty trimmed phone clears its error");

await act(async () => {
  fixtureLogin.click();
  await flush();
});
await waitFor("fixture login failure", () => fixtureLoginCalls === 1 && useAccountStore.getState().error !== null);
await act(async () => {
  setInput(fixturePassword, "preserved-across-mode-switch");
  await flush();
});
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-mode="register"]').click();
  await flush();
});
const registerPhone = required<HTMLInputElement>(fixtureRender.container, 'input[name="phone"]');
const registerPassword = required<HTMLInputElement>(fixtureRender.container, 'input[name="password"]');
const registerSMSCode = required<HTMLInputElement>(fixtureRender.container, 'input[name="sms_code"]');
ok(useAccountStore.getState().error === null, "switching to registration clears the login banner");
ok(registerPhone.value === "13800000000" && registerPassword.value === "preserved-across-mode-switch", "switching modes preserves entered credentials");
ok(fixtureRender.container.querySelectorAll('.account-center__required[aria-hidden="true"]').length === 3, "registration renders three decorative required marks");

await act(async () => {
  setInput(registerPhone, "");
  setInput(registerPassword, "");
  setInput(registerSMSCode, " ");
  await flush();
});
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="register"]').click();
  await flush();
});
ok(fixtureRegisterCalls === 0, "empty registration never calls CloudRegister");
ok(fixtureRender.container.textContent?.includes("手机号不能为空") === true
  && fixtureRender.container.textContent?.includes("密码不能为空") === true
  && fixtureRender.container.textContent?.includes("短信验证码不能为空") === true, "registration reports every missing field");
ok(document.activeElement === registerPhone, "empty registration focuses the phone first");

const fixtureCaptcha = required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="captcha"]');
await act(async () => {
  fixtureCaptcha.click();
  await flush();
});
ok(fixtureCaptcha.textContent?.includes("滑块验证已通过") === false, "empty phone cannot advance fixture verification");
ok(required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="sms"]').disabled, "empty phone keeps fixture SMS disabled");
await act(async () => {
  invokeReactClick(required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="sms"]'));
  await flush();
});
ok(fixtureSMSCalls === 0, "handler-level SMS defense blocks an empty phone");

await act(async () => {
  setInput(registerPhone, "13800000000");
  setInput(registerSMSCode, "123456");
  await flush();
});
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="register"]').click();
  await flush();
});
ok(fixtureRegisterCalls === 0, "registration with only a missing password never calls CloudRegister");
ok(fixtureRender.container.textContent?.includes("密码不能为空") === true, "registration with only a missing password shows its error");
ok(fixtureRender.container.textContent?.includes("手机号不能为空") === false
  && fixtureRender.container.textContent?.includes("短信验证码不能为空") === false, "non-empty registration fields do not retain empty errors");
ok(document.activeElement === registerPassword, "registration with only a missing password focuses the password");

await act(async () => {
  setInput(registerPhone, "13800000000");
  setInput(registerPassword, "register-secret");
  setInput(registerSMSCode, "123456");
  await flush();
});
await act(async () => {
  fixtureCaptcha.click();
  await flush();
});
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="sms"]').click();
  await flush();
});
ok(fixtureSMSCalls === 1, "valid fixture SMS still calls its bridge once");
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-mode="login"]').click();
  await flush();
});
ok(required<HTMLInputElement>(fixtureRender.container, 'input[name="phone"]').value === "13800000000", "mode switching preserves the phone");
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-mode="register"]').click();
  await flush();
});
ok(/秒后/.test(required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="sms"]').textContent ?? ""), "mode switching preserves the SMS cooldown");
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-action="register"]').click();
  await flush();
});
await waitFor("fixture registration failure", () => fixtureRegisterCalls === 1 && useAccountStore.getState().error !== null);
await act(async () => {
  required<HTMLButtonElement>(fixtureRender.container, '[data-account-mode="login"]').click();
  await flush();
});
ok(useAccountStore.getState().error === null, "switching to login clears the registration banner");
await dispose(fixtureRender.root, fixtureRender.container);

console.log("\naccount center real CAPTCHA validation");

let sdkOptions: AliyunCaptchaSDKOptions | undefined;
(window as typeof window & { initAliyunCaptcha?: (options: AliyunCaptchaSDKOptions) => void }).initAliyunCaptcha = (options) => {
  sdkOptions = options;
  options.getInstance?.({ destroy: () => undefined });
};
let realLoginCalls = 0;
let realSMSCalls = 0;
const realRender = await renderAccount(apiWith(realCapabilities, {
  CloudLogin: async () => {
    realLoginCalls += 1;
    return anonymous(realCapabilities);
  },
  CloudSendRegisterSMS: async () => {
    realSMSCalls += 1;
  },
}));
await waitFor("real login CAPTCHA", () => realRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
await act(async () => {
  required<HTMLButtonElement>(realRender.container, '[data-account-action="login"]').click();
  sdkOptions?.success?.('{"token":"must-not-be-consumed"}');
  await flush();
});
ok(realLoginCalls === 0, "empty real login is blocked before the SDK records intent");
ok(document.activeElement === required<HTMLInputElement>(realRender.container, 'input[name="phone"]'), "empty real login focuses the phone");
await act(async () => {
  (document.activeElement as HTMLElement | null)?.blur();
  setInput(required<HTMLInputElement>(realRender.container, 'input[name="phone"]'), "13900000000");
  setInput(required<HTMLInputElement>(realRender.container, 'input[name="password"]'), "real-secret");
  await flush();
});
await act(async () => {
  required<HTMLButtonElement>(realRender.container, '[data-account-action="login"]').click();
  sdkOptions?.success?.('{"token":"valid-login-proof"}');
  await flush();
});
await waitFor("valid real login", () => realLoginCalls === 1);
ok(realLoginCalls === 1, "valid real login still reaches the bridge once");

await act(async () => {
  required<HTMLButtonElement>(realRender.container, '[data-account-mode="register"]').click();
  setInput(required<HTMLInputElement>(realRender.container, 'input[name="phone"]'), "");
  await flush();
});
await waitFor("real registration CAPTCHA", () => realRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
await act(async () => {
  required<HTMLButtonElement>(realRender.container, '[data-account-action="captcha"]').click();
  sdkOptions?.success?.('{"token":"must-not-send-sms"}');
  await flush();
});
ok(realSMSCalls === 0, "empty real registration phone is blocked before CAPTCHA intent and SMS");
await dispose(realRender.root, realRender.container);

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
