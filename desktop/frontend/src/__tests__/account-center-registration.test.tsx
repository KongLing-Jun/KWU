// Run: tsx src/__tests__/account-center-registration.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";

import { AccountCenter, type AccountCloudBindings } from "../components/AccountCenter";
import type { AliyunCaptchaSDKOptions } from "../lib/aliyunCaptcha";
import type {
  CloudLoginRequest,
  CloudResetPasswordRequest,
  CloudSendPasswordResetSMSRequest,
  CloudSendRegisterSMSRequest,
} from "../lib/bridge";
import { clientErrorPresentation } from "../lib/clientError";
import { LocaleProvider } from "../lib/i18n";
import type { CloudBootstrapView, CloudCapabilitiesView } from "../store/account";
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

function anonymous(capabilities: CloudCapabilitiesView, profileID = "default"): CloudBootstrapView {
  return {
    available: true,
    capabilities,
    account: { status: "anonymous", profile_id: profileID },
  };
}

function apiWith(
  bootstrap: () => Promise<CloudBootstrapView>,
  overrides: Partial<AccountCloudBindings> = {},
): AccountCloudBindings {
  return {
    CloudBootstrap: bootstrap,
    CloudCaptchaConfig: async () => { throw new Error("real CAPTCHA is outside this test"); },
    CloudSendRegisterSMS: async () => undefined,
    CloudSendPasswordResetSMS: async () => undefined,
    CloudResetPassword: async () => undefined,
    CloudRegister: async () => anonymous(fixtureCapabilities),
    CloudLogin: async () => anonymous(fixtureCapabilities),
    CloudSwitchAccount: async () => anonymous(fixtureCapabilities),
    CloudLogout: async () => anonymous(fixtureCapabilities),
    CloudRefreshAssets: async () => anonymous(fixtureCapabilities),
    CloudCreateSubscriptionOrder: async () => { throw new Error("payment create is outside this test"); },
    CloudCreateUpgradeOrder: async () => { throw new Error("payment upgrade is outside this test"); },
    CloudConfirmPayment: async () => { throw new Error("payment confirm is outside this test"); },
    CloudRefreshPayment: async () => { throw new Error("payment refresh is outside this test"); },
    ...overrides,
  };
}

async function renderAccount(
  api: AccountCloudBindings,
  contextKey = "default",
  onAuthenticated?: (accountScopeKey: string) => void | Promise<void>,
) {
  useAccountStore.getState().reset();
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AccountCenter api={api} contextKey={contextKey} onAuthenticated={onAuthenticated} />
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

const fixtureCapabilities: CloudCapabilitiesView = {
  registration: true,
  registration_mode: "fixture",
  payments: true,
  realtime: true,
};
const developmentCapabilities: CloudCapabilitiesView = {
  registration: true,
  registration_mode: "development",
  payments: false,
  realtime: false,
};
const realCapabilities: CloudCapabilitiesView = {
  registration: true,
  registration_mode: "real",
  payments: false,
  realtime: false,
};
const unavailableCapabilities: CloudCapabilitiesView = {
  registration: false,
  registration_mode: "unavailable",
  payments: false,
  realtime: false,
};

console.log("\naccount center registration modes");

const dom = installDom();

const fixtureRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities)));
await act(async () => {
  fixtureRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
ok(fixtureRender.container.textContent?.includes("完成滑块验证") === true, "fixture registration keeps its existing verification copy");
ok(fixtureRender.container.textContent?.includes("开发环境验证") === false, "fixture registration does not use development copy");
await dispose(fixtureRender.root, fixtureRender.container);

console.log("\naccount center authentication mode normalization");

const fixtureLoginRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities)));
ok(fixtureLoginRender.container.querySelector('[data-account-action="login-captcha"]') === null, "fixture login does not fall back to development verification");
ok(fixtureLoginRender.container.querySelector('[data-account-action="login"]') !== null, "fixture login keeps its direct action");
await dispose(fixtureLoginRender.root, fixtureLoginRender.container);

const legacyModeRender = await renderAccount(apiWith(async () => anonymous(developmentCapabilities)));
ok(legacyModeRender.container.querySelector('[data-account-auth-hint="real"]') !== null, "legacy development metadata is normalized to real authentication");
ok(legacyModeRender.container.querySelector('[data-account-action="login-captcha"]') === null, "legacy development metadata cannot expose a development verification action");
await dispose(legacyModeRender.root, legacyModeRender.container);

const smsRequests: Array<[string, boolean]> = [];
const fixtureSMSAPI = apiWith(async () => anonymous(fixtureCapabilities), {
  CloudSendRegisterSMS: async (request) => {
    smsRequests.push([request.phone, request.captcha_passed === true]);
  },
});
const fixtureSMSRender = await renderAccount(fixtureSMSAPI);
await act(async () => {
  fixtureSMSRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
ok(!/ticket|固定短信码|token/i.test(fixtureSMSRender.container.textContent ?? ""), "fixture registration exposes no credential material");

const fixtureCaptcha = fixtureSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]');
const fixturePhone = fixtureSMSRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
if (!fixtureCaptcha || !fixturePhone) throw new Error("missing fixture registration controls");
ok(fixtureCaptcha.textContent?.includes("完成滑块验证") === true, "fixture registration has an explicit verification action");
await act(async () => {
  setInput(fixturePhone, "13800000000");
  await flush();
});
await act(async () => {
  fixtureCaptcha.click();
  await flush();
});
await act(async () => {
  fixtureSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.click();
  await flush();
});
ok(JSON.stringify(smsRequests[0]) === JSON.stringify(["13800000000", true]), "fixture SMS forwards the phone and passed boolean without a ticket");
ok(fixtureCaptcha.textContent?.includes("完成滑块验证") === true, "successful SMS resets fixture verification");
ok(fixtureSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.disabled === true, "successful SMS requires verification before retry");

await act(async () => {
  fixtureCaptcha.click();
  await flush();
});
const coolingDownSMS = fixtureSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]');
ok(coolingDownSMS?.disabled === true, "successful SMS starts a cooldown that cannot be bypassed by repeating CAPTCHA");
ok(/\d+\s*秒后/.test(coolingDownSMS?.textContent ?? ""), "successful SMS renders the remaining resend cooldown");
await act(async () => {
  invokeReactClick(coolingDownSMS as HTMLButtonElement);
  await flush();
});
ok(smsRequests.length === 1, "cooldown blocks a repeated SMS request");
await dispose(fixtureSMSRender.root, fixtureSMSRender.container);

const failedSMSRequests: Array<[string, boolean]> = [];
const failedSMSRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudSendRegisterSMS: async (request) => {
    failedSMSRequests.push([request.phone, request.captcha_passed === true]);
    throw {
      code: "SMS_FAILED",
      message: `sms failed for ${request.phone}`,
      request_id: "req-register-sms-01",
    };
  },
}));
await act(async () => {
  failedSMSRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const failedSMSPhone = failedSMSRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const failedSMSCaptcha = failedSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]');
if (!failedSMSPhone || !failedSMSCaptcha) throw new Error("missing failed SMS registration controls");
await act(async () => {
  setInput(failedSMSPhone, "13800000000");
  await flush();
});
await act(async () => {
  failedSMSCaptcha.click();
  await flush();
});
await act(async () => {
  failedSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.click();
  await flush();
});
ok(JSON.stringify(failedSMSRequests[0]) === JSON.stringify(["13800000000", true]), "failed fixture SMS still forwards the phone and passed boolean once");
ok(failedSMSCaptcha.textContent?.includes("完成滑块验证") === true, "failed SMS resets fixture verification");
ok(failedSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.disabled === true, "failed SMS requires verification before retry without starting a cooldown");
ok(!(failedSMSRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "").includes("13800000000"), "SMS error never renders its request payload");
ok(failedSMSRender.container.querySelector(".account-center__error-icon")?.getAttribute("aria-hidden") === "true", "cloud errors render a decorative status icon");
ok(failedSMSRender.container.querySelector('[data-account-error-code][translate="no"]')?.textContent?.includes("SMS_FAILED") === true, "cloud error codes retain their technical identifier");
ok(failedSMSRender.container.querySelector('[data-account-error-request-id][translate="no"]')?.textContent?.includes("req-register-sms-01") === true, "safe cloud errors render their request ID");
await act(async () => {
  failedSMSCaptcha.click();
  await flush();
});
ok(failedSMSRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.disabled === false, "failed SMS can be retried immediately after a new CAPTCHA");
await dispose(failedSMSRender.root, failedSMSRender.container);

const wailsStringErrorRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudSendRegisterSMS: async () => {
		throw new Error("SMS_RATE_LIMITED: SMS requests are too frequent [request_id=req-wails-string-01] [diagnostic_id=diag-wails-string-01]");
  },
}));
await act(async () => {
  wailsStringErrorRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const wailsStringCaptcha = wailsStringErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]');
const wailsStringPhone = wailsStringErrorRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
if (!wailsStringCaptcha || !wailsStringPhone) throw new Error("missing Wails string error controls");
await act(async () => {
  setInput(wailsStringPhone, "13800000000");
  await flush();
});
await act(async () => {
  wailsStringCaptcha.click();
  await flush();
});
await act(async () => {
  wailsStringErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.click();
  await flush();
});
ok(wailsStringErrorRender.container.querySelector('[data-account-error-request-id]')?.textContent?.includes("req-wails-string-01") === true, "Wails string errors expose the safe request ID separately");
ok(wailsStringErrorRender.container.querySelector('[data-account-error-diagnostic-id]')?.textContent?.includes("diag-wails-string-01") === true, "Wails string errors expose the local diagnostic ID separately");
ok(wailsStringErrorRender.container.querySelector('[data-account-error-diagnostic-id] .copybtn') !== null, "Wails string errors expose a dedicated Log ID copy control");
ok(!(wailsStringErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "").includes("[request_id="), "Wails request ID metadata is removed from the user message");
ok(!(wailsStringErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "").includes("[diagnostic_id="), "Wails diagnostic metadata is removed from the user message");
ok(wailsStringErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent?.includes("操作过于频繁，请稍后再试") === true, "SMS rate limits use the account-specific Chinese message");
await dispose(wailsStringErrorRender.root, wailsStringErrorRender.container);

console.log("\naccount center common error messages");

const invalidCredentialsRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudLogin: async () => {
    throw {
      code: "INVALID_CREDENTIALS",
      message: "unsafe credential diagnostic",
      request_id: "req-invalid-credentials-01",
    };
  },
}));
const invalidCredentialsPhone = invalidCredentialsRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const invalidCredentialsPassword = invalidCredentialsRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!invalidCredentialsPhone || !invalidCredentialsPassword) throw new Error("missing invalid credential inputs");
await act(async () => {
  setInput(invalidCredentialsPhone, "13900000000");
  setInput(invalidCredentialsPassword, "invalid-credential-secret");
  await flush();
});
await act(async () => {
  invalidCredentialsRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("invalid credentials message", () => useAccountStore.getState().error?.code === "INVALID_CREDENTIALS");
const invalidCredentialsAlert = invalidCredentialsRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(invalidCredentialsAlert.includes("手机号未注册或密码错误"), "invalid credentials use the deterministic Chinese account message");
ok(!invalidCredentialsAlert.includes("unsafe credential diagnostic"), "invalid credentials never expose the cloud message");
await dispose(invalidCredentialsRender.root, invalidCredentialsRender.container);

const registrationErrors = [
  { code: "SMS_CODE_INVALID", message: "短信验证码错误，请重新输入" },
  { code: "SMS_CODE_EXPIRED", message: "短信验证码已过期，请重新获取" },
  { code: "SMS_CODE_ATTEMPTS_EXCEEDED", message: "验证码错误次数过多，请重新获取" },
  { code: "PHONE_INVALID", message: "手机号格式不正确" },
  { code: "PHONE_ALREADY_REGISTERED", message: "该手机号已注册，请直接登录" },
  { code: "PASSWORD_INVALID", message: "密码格式不正确，请输入 8–20 个字符" },
  { code: "SLIDER_CAPTCHA_REQUIRED", message: "请重新完成滑块验证" },
  { code: "SLIDER_CAPTCHA_EXPIRED", message: "滑块验证已过期，请重新完成验证" },
  { code: "SLIDER_CAPTCHA_REUSED", message: "请重新完成滑块验证" },
  { code: "AUTH_RATE_LIMITED", message: "操作过于频繁，请稍后再试" },
  { code: "SMS_PROVIDER_UNAVAILABLE", message: "短信服务暂不可用，请稍后重试" },
  { code: "ACCOUNT_DISABLED", message: "账号已被禁用，请联系管理员" },
  { code: "AUTH_CRYPTO_CONFIG_UNAVAILABLE", message: "注册服务暂不可用，请稍后重试" },
  { code: "PASSWORD_DECRYPT_FAILED", message: "密码校验失败，请重新输入后再试" },
] as const;
let registrationErrorAttempt = 0;
const registrationErrorRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudRegister: async () => {
    const errorCase = registrationErrors[registrationErrorAttempt] ?? registrationErrors[registrationErrors.length - 1];
    registrationErrorAttempt += 1;
    throw { code: errorCase.code, message: `unsafe ${errorCase.code} diagnostic` };
  },
}));
await act(async () => {
  registrationErrorRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const registrationErrorPhone = registrationErrorRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const registrationErrorPassword = registrationErrorRender.container.querySelector<HTMLInputElement>('input[name="password"]');
const registrationErrorSMS = registrationErrorRender.container.querySelector<HTMLInputElement>('input[name="sms_code"]');
if (!registrationErrorPhone || !registrationErrorPassword || !registrationErrorSMS) throw new Error("missing registration error inputs");
await act(async () => {
  setInput(registrationErrorPhone, "13800000000");
  setInput(registrationErrorPassword, "registration-error-secret");
  setInput(registrationErrorSMS, "111111");
  await flush();
});
await act(async () => {
  registrationErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
  await flush();
});
await waitFor("invalid SMS code message", () => useAccountStore.getState().error?.code === "SMS_CODE_INVALID");
ok(registrationErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent?.includes("短信验证码错误，请重新输入") === true, "invalid SMS codes use the deterministic Chinese message");
await act(async () => {
  setInput(registrationErrorPassword, "registration-expired-secret");
  await flush();
});
await act(async () => {
  registrationErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
  await flush();
});
await waitFor("expired SMS code message", () => useAccountStore.getState().error?.code === "SMS_CODE_EXPIRED");
const expiredSMSAlert = registrationErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(expiredSMSAlert.includes("短信验证码已过期，请重新获取"), "expired SMS codes use the deterministic Chinese message");
ok(!expiredSMSAlert.includes("unsafe SMS_CODE_EXPIRED diagnostic"), "registration errors never expose the cloud message");
for (const errorCase of registrationErrors.slice(2)) {
  await act(async () => {
    setInput(registrationErrorPassword, `registration-${errorCase.code.toLowerCase()}-secret`);
    await flush();
  });
  await act(async () => {
    registrationErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
    await flush();
  });
  await waitFor(`${errorCase.code} message`, () => useAccountStore.getState().error?.code === errorCase.code);
  const errorAlert = registrationErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
  ok(errorAlert.includes(errorCase.message), `${errorCase.code} uses its deterministic Chinese account message`);
  ok(!errorAlert.includes(`unsafe ${errorCase.code} diagnostic`), `${errorCase.code} never exposes the cloud message`);
}
await dispose(registrationErrorRender.root, registrationErrorRender.container);

let smsErrorAttempt = 0;
const commonSMSErrorRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudSendRegisterSMS: async () => {
    smsErrorAttempt += 1;
    const code = smsErrorAttempt === 1 ? "SMS_RATE_LIMITED" : "NETWORK_UNAVAILABLE";
    throw { code, message: `unsafe ${code} diagnostic` };
  },
}));
await act(async () => {
  commonSMSErrorRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const commonSMSErrorPhone = commonSMSErrorRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const commonSMSErrorCaptcha = commonSMSErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]');
if (!commonSMSErrorPhone || !commonSMSErrorCaptcha) throw new Error("missing common SMS error controls");
await act(async () => {
  setInput(commonSMSErrorPhone, "13800000000");
  await flush();
});
await act(async () => {
  commonSMSErrorCaptcha.click();
  await flush();
});
await act(async () => {
  commonSMSErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.click();
  await flush();
});
await waitFor("structured SMS rate limit message", () => useAccountStore.getState().error?.code === "SMS_RATE_LIMITED");
ok(commonSMSErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent?.includes("操作过于频繁，请稍后再试") === true, "structured SMS rate limits use the deterministic Chinese message");
await act(async () => {
  commonSMSErrorCaptcha.click();
  await flush();
});
await act(async () => {
  commonSMSErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="sms"]')?.click();
  await flush();
});
await waitFor("SMS network error message", () => useAccountStore.getState().error?.code === "NETWORK_UNAVAILABLE");
const networkErrorAlert = commonSMSErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(networkErrorAlert.includes("网络连接失败，请检查网络后重试"), "SMS network failures use the deterministic Chinese message");
ok(!networkErrorAlert.includes("unsafe NETWORK_UNAVAILABLE diagnostic"), "SMS errors never expose the cloud message");
await dispose(commonSMSErrorRender.root, commonSMSErrorRender.container);

const unknownErrorRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudLogin: async () => {
    throw {
      code: "UNEXPECTED_ACCOUNT_FAILURE",
      message: "unsafe unknown account diagnostic",
      request_id: "req-unknown-account-01",
    };
  },
}));
const unknownErrorPhone = unknownErrorRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const unknownErrorPassword = unknownErrorRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!unknownErrorPhone || !unknownErrorPassword) throw new Error("missing unknown error inputs");
await act(async () => {
  setInput(unknownErrorPhone, "13900000000");
  setInput(unknownErrorPassword, "unknown-error-secret");
  await flush();
});
await act(async () => {
  unknownErrorRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("unknown account error", () => useAccountStore.getState().error?.code === "UNEXPECTED_ACCOUNT_FAILURE");
const unknownErrorAlert = unknownErrorRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(unknownErrorAlert.includes("云请求失败"), "unknown account codes use the controlled fallback");
ok(!unknownErrorAlert.includes("unsafe unknown account diagnostic"), "unknown account codes never expose the cloud message");
ok(unknownErrorRender.container.querySelector('[data-account-error-code]')?.textContent?.includes("UNEXPECTED_ACCOUNT_FAILURE") === true, "unknown account codes preserve technical details");
ok(unknownErrorRender.container.querySelector('[data-account-error-request-id]')?.textContent?.includes("req-unknown-account-01") === true, "unknown account errors preserve request IDs");
await dispose(unknownErrorRender.root, unknownErrorRender.container);

ok(
  clientErrorPresentation(
    { code: "INVALID_CREDENTIALS", message: "raw account error" },
    { fallbackMessage: "其他页面回退提示" },
  ).userMessage === "其他页面回退提示",
  "account-specific mappings do not change the global client error table",
);

console.log("\naccount center real Aliyun CAPTCHA");

let realSDKOptions: AliyunCaptchaSDKOptions | undefined;
let realSDKInitCalls = 0;
let realSDKDestroyCalls = 0;
(window as typeof window & { initAliyunCaptcha?: (options: AliyunCaptchaSDKOptions) => void }).initAliyunCaptcha = (options) => {
  realSDKOptions = options;
  realSDKInitCalls += 1;
  options.getInstance?.({ destroy: () => { realSDKDestroyCalls += 1; } });
};
const discoveredConfig = async () => ({
  provider: "aliyun_captcha_v2",
  region: "cn",
  prefix: "public-prefix",
  scene_id: "register-scene",
});
const rawTicket = `{"sceneId":"register-scene","token":"opaque"}`;
const realSMSRequests: CloudSendRegisterSMSRequest[] = [];
const realRegistrationRender = await renderAccount(apiWith(async () => anonymous(realCapabilities), {
  CloudCaptchaConfig: discoveredConfig,
  CloudSendRegisterSMS: async (request) => { realSMSRequests.push(request); },
}));
await act(async () => {
  realRegistrationRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const realPhone = realRegistrationRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
if (!realPhone) throw new Error("missing real registration phone");
await act(async () => {
  setInput(realPhone, "13800000000");
  await flush();
});
await waitFor("real registration CAPTCHA ready", () => realRegistrationRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
await act(async () => {
  realRegistrationRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]')?.click();
  realSDKOptions?.success?.(rawTicket);
  await flush();
});
await waitFor("single real SMS", () => realSMSRequests.length === 1);
ok(JSON.stringify(realSMSRequests[0]) === JSON.stringify({
  phone: "13800000000",
  captcha_ticket: rawTicket,
  captcha_provider: "aliyun_captcha_v2",
}), "real slider success sends one SMS with the raw proof unchanged");
ok(realRegistrationRender.container.querySelector('[data-account-action="sms"]') === null, "real registration has no second SMS action after slider success");
ok(realSDKDestroyCalls >= 1 && realSDKInitCalls >= 2, "real SMS completion destroys and rebuilds the slider");
ok(!JSON.stringify(useAccountStore.getState()).includes(rawTicket), "real SMS proof never enters the account store");
ok(!JSON.stringify({ ...localStorage }).includes(rawTicket), "real SMS proof is not persisted");
await dispose(realRegistrationRender.root, realRegistrationRender.container);

let realSMSAttempts = 0;
const retryRender = await renderAccount(apiWith(async () => anonymous(realCapabilities), {
  CloudCaptchaConfig: discoveredConfig,
  CloudSendRegisterSMS: async () => {
    realSMSAttempts += 1;
    if (realSMSAttempts === 1) throw { code: "SLIDER_CAPTCHA_EXPIRED", message: "verify again" };
  },
}));
await act(async () => {
  retryRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const retryPhone = retryRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
if (!retryPhone) throw new Error("missing retry phone");
await act(async () => {
  setInput(retryPhone, "13800000000");
  await flush();
});
await waitFor("retry CAPTCHA ready", () => retryRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
const initBeforeFailure = realSDKInitCalls;
await act(async () => {
  retryRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]')?.click();
  realSDKOptions?.success?.(`{"token":"expired-proof"}`);
  await flush();
});
await waitFor("expired proof rebuild", () => realSMSAttempts === 1 && realSDKInitCalls > initBeforeFailure);
ok(retryRender.container.querySelector('[data-account-error-request-id]') === null, "expired proof failure exposes no sensitive proof metadata");
const secondProof = `{"token":"fresh-proof"}`;
await act(async () => {
  retryRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]')?.click();
  realSDKOptions?.success?.(secondProof);
  await flush();
});
await waitFor("fresh proof retry", () => realSMSAttempts === 2);
ok(realSMSAttempts === 2, "expired real proof rebuilds and requires a fresh user verification");
await dispose(retryRender.root, retryRender.container);

const realLoginRequests: CloudLoginRequest[] = [];
const realLoginRender = await renderAccount(apiWith(async () => anonymous(realCapabilities), {
  CloudCaptchaConfig: discoveredConfig,
  CloudLogin: async (request) => {
    realLoginRequests.push(request);
    return anonymous(realCapabilities);
  },
}));
const realLoginPhone = realLoginRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const realLoginPassword = realLoginRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!realLoginPhone || !realLoginPassword) throw new Error("missing real login inputs");
await act(async () => {
  setInput(realLoginPhone, "13900000000");
  setInput(realLoginPassword, "local-secret");
  await flush();
});
await waitFor("real login CAPTCHA ready", () => realLoginRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
await act(async () => {
  realLoginRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  realSDKOptions?.success?.(rawTicket);
  await flush();
});
await waitFor("single real login", () => realLoginRequests.length === 1);
ok(JSON.stringify(realLoginRequests[0]) === JSON.stringify({
  phone: "13900000000",
  password: "local-secret",
  captcha_ticket: rawTicket,
  captcha_provider: "aliyun_captcha_v2",
}), "real slider success submits login once with the same proof");
ok(realLoginPassword.value === "", "real login clears the password after submission");
await dispose(realLoginRender.root, realLoginRender.container);

console.log("\naccount center password reset");

const resetSMSRequests: CloudSendPasswordResetSMSRequest[] = [];
const resetPasswordRequests: CloudResetPasswordRequest[] = [];
let resetPasswordAttempt = 0;
const resetRender = await renderAccount(apiWith(async () => anonymous(realCapabilities), {
  CloudCaptchaConfig: discoveredConfig,
  CloudSendPasswordResetSMS: async (request) => {
    resetSMSRequests.push(request);
  },
  CloudResetPassword: async (request) => {
    resetPasswordRequests.push(request);
    resetPasswordAttempt += 1;
    if (resetPasswordAttempt === 1) {
      throw { code: "PASSWORD_INVALID", message: "unsafe password reset diagnostic" };
    }
    if (resetPasswordAttempt === 2) {
      throw { code: "INVALID_CREDENTIALS", message: "unsafe reset credential diagnostic" };
    }
    if (resetPasswordAttempt === 3) {
      throw { code: "SMS_CODE_EXPIRED", message: "unsafe reset SMS diagnostic" };
    }
  },
}));
const forgotPassword = resetRender.container.querySelector<HTMLButtonElement>('[data-account-action="password-reset-open"]');
if (!forgotPassword) throw new Error("missing forgot-password entry");
await act(async () => {
  invokeReactClick(forgotPassword);
  await flush();
});
const resetPhone = resetRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
if (!resetPhone) throw new Error("missing password-reset phone");
await act(async () => {
  setInput(resetPhone, "13700000000");
  await flush();
});
await waitFor("password-reset CAPTCHA ready", () => resetRender.container.querySelector('[data-aliyun-captcha-status="ready"]') !== null);
const resetTicket = `{"sceneId":"register-scene","token":"reset-proof"}`;
await act(async () => {
  resetRender.container.querySelector<HTMLButtonElement>('[data-account-action="password-reset-send"]')?.click();
  realSDKOptions?.success?.(resetTicket);
  await flush();
});
await waitFor("password-reset SMS request", () => resetSMSRequests.length === 1);
ok(JSON.stringify(resetSMSRequests[0]) === JSON.stringify({
  phone: "13700000000",
  captcha_ticket: resetTicket,
  captcha_provider: "aliyun_captcha_v2",
}), "forgot-password slider forwards the raw proof to the reset SMS endpoint");

const resetSMSCode = resetRender.container.querySelector<HTMLInputElement>('input[name="password_reset_sms_code"]');
const resetNewPassword = resetRender.container.querySelector<HTMLInputElement>('input[name="password_reset_new_password"]');
const resetConfirmPassword = resetRender.container.querySelector<HTMLInputElement>('input[name="password_reset_confirm_password"]');
const resetSubmit = resetRender.container.querySelector<HTMLButtonElement>('[data-account-action="password-reset-submit"]');
if (!resetSMSCode || !resetNewPassword || !resetConfirmPassword || !resetSubmit) {
  throw new Error("missing password-reset confirmation inputs");
}
ok(
  resetRender.container.querySelectorAll('.account-center__required[aria-hidden="true"]').length === 4,
  "password reset renders required marks for phone and all confirmation fields",
);
ok(
  resetSMSCode.required && resetNewPassword.required && resetConfirmPassword.required,
  "password-reset confirmation inputs expose required semantics",
);
ok(!resetSubmit.disabled, "password-reset submit remains available for local required validation");
await act(async () => {
  resetSubmit.click();
  await flush();
});
ok(resetPasswordRequests.length === 0, "empty password reset never calls the cloud bridge");
ok(
  resetRender.container.textContent?.includes("短信验证码不能为空") === true
    && resetRender.container.textContent?.includes("新密码不能为空") === true
    && resetRender.container.textContent?.includes("确认新密码不能为空") === true,
  "password reset reports every missing confirmation field",
);
ok(document.activeElement === resetSMSCode, "empty password reset focuses the SMS code first");
ok(
  resetRender.container.querySelector('[role="alert"]') === null,
  "password-reset required errors stay below fields instead of creating a top banner",
);
await act(async () => {
  setInput(resetSMSCode, "654321");
  setInput(resetNewPassword, "new-password-a12");
  setInput(resetConfirmPassword, "different-password");
  await flush();
});
ok(
  resetSMSCode.getAttribute("aria-invalid") === "false"
    && resetNewPassword.getAttribute("aria-invalid") === "false"
    && resetConfirmPassword.getAttribute("aria-invalid") === "false",
  "entering password-reset values clears their required errors",
);
await act(async () => {
  resetSubmit.click();
  await flush();
});
ok(resetPasswordRequests.length === 0, "mismatched reset passwords never call the cloud bridge");
const resetMismatchAlert = resetRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(resetMismatchAlert.includes("两次输入的新密码不一致"), "password mismatch uses the reset-specific Chinese message");
ok(!resetMismatchAlert.includes("云请求失败"), "password mismatch never falls back to the generic cloud message");

await act(async () => {
  setInput(resetConfirmPassword, "new-password-a12");
  await flush();
});
await act(async () => {
  resetSubmit.click();
  await flush();
});
await waitFor("reset password invalid message", () => useAccountStore.getState().error?.code === "PASSWORD_INVALID");
const resetPasswordInvalidAlert = resetRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(resetPasswordInvalidAlert.includes("新密码需包含 8～20 个字符"), "cloud password errors use the reset-specific Chinese message");
ok(!resetPasswordInvalidAlert.includes("unsafe password reset diagnostic"), "reset password errors never expose the cloud message");

await act(async () => {
  setInput(resetNewPassword, "new-password-a12");
  setInput(resetConfirmPassword, "new-password-a12");
  await flush();
});
await act(async () => {
  resetSubmit.click();
  await flush();
});
await waitFor("reset rejected message", () => useAccountStore.getState().error?.code === "INVALID_CREDENTIALS");
const resetRejectedAlert = resetRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(resetRejectedAlert.includes("无法完成密码重置，请确认手机号后重试"), "reset rejection uses a non-enumerating Chinese message");
ok(!resetRejectedAlert.includes("手机号未注册或密码错误"), "reset rejection never reuses the login credential message");
ok(!resetRejectedAlert.includes("unsafe reset credential diagnostic"), "reset rejection never exposes the cloud message");

await act(async () => {
  setInput(resetNewPassword, "new-password-a12");
  setInput(resetConfirmPassword, "new-password-a12");
  await flush();
});
await act(async () => {
  resetSubmit.click();
  await flush();
});
await waitFor("reset expired SMS message", () => useAccountStore.getState().error?.code === "SMS_CODE_EXPIRED");
const resetExpiredSMSAlert = resetRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent ?? "";
ok(resetExpiredSMSAlert.includes("短信验证码已过期，请重新获取"), "reset reuses the deterministic expired SMS message");
ok(!resetExpiredSMSAlert.includes("unsafe reset SMS diagnostic"), "reset SMS errors never expose the cloud message");

await act(async () => {
  setInput(resetNewPassword, "new-password-a12");
  setInput(resetConfirmPassword, "new-password-a12");
  await flush();
});
await act(async () => {
  resetSubmit.click();
  await flush();
});
await waitFor("password-reset request", () => resetPasswordRequests.length === 4);
ok(JSON.stringify(resetPasswordRequests[3]) === JSON.stringify({
  phone: "13700000000",
  new_password: "new-password-a12",
  sms_code: "654321",
}), "password reset submits the documented phone, new password, and SMS code");
ok(resetRender.container.querySelector('[data-account-password-reset-success]') !== null, "successful password reset returns to login with a success notice");
ok(resetRender.container.querySelector<HTMLInputElement>('input[name="phone"]')?.value === "13700000000", "successful password reset retains the phone for login");
ok(resetRender.container.querySelector('input[name^="password_reset_"]') === null, "successful password reset removes reset secrets from the DOM");
ok(!JSON.stringify(useAccountStore.getState()).includes("new-password-a12"), "password-reset secrets never enter the account store");
ok(!JSON.stringify({ ...localStorage }).includes(resetTicket), "password-reset proof is not persisted");
await dispose(resetRender.root, resetRender.container);

console.log("\naccount center delegates post-authentication refresh ownership");

const authenticatedView: CloudBootstrapView = {
  available: true,
  capabilities: fixtureCapabilities,
  account: {
    status: "authenticated",
    profile_id: "default",
    cloud_user_id: "cloud-user",
    binding_generation: 1,
  },
};
const loginAuthenticatedScopes: string[] = [];
const loginRefreshRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudLogin: async () => authenticatedView,
}), "default", (accountScopeKey) => {
  loginAuthenticatedScopes.push(accountScopeKey);
  throw new Error("downstream refresh failure must not affect login");
});
const loginRefreshPhone = loginRefreshRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const loginRefreshPassword = loginRefreshRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!loginRefreshPhone || !loginRefreshPassword) throw new Error("missing marketplace refresh login inputs");
await act(async () => {
  setInput(loginRefreshPhone, "13900000002");
  setInput(loginRefreshPassword, "local-secret");
  await flush();
});
await act(async () => {
  loginRefreshRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("login authentication notification", () => loginAuthenticatedScopes.length === 1);
ok(loginAuthenticatedScopes[0] === "default\u0000cloud-user\u00001", "login forwards the exact authenticated account scope");
ok(loginRefreshRender.container.querySelector('[data-account-state="authenticated"]') !== null, "downstream refresh failure does not affect successful login");
await dispose(loginRefreshRender.root, loginRefreshRender.container);

let resolveDeferredAuthentication!: () => void;
const deferredAuthentication = new Promise<void>((resolve) => {
  resolveDeferredAuthentication = resolve;
});
let deferredAuthenticationStarted = 0;
let deferredAuthenticationCompleted = 0;
const deferredRefreshRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudLogin: async () => authenticatedView,
}), "default", async () => {
  deferredAuthenticationStarted += 1;
  await deferredAuthentication;
  deferredAuthenticationCompleted += 1;
});
const deferredRefreshPhone = deferredRefreshRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const deferredRefreshPassword = deferredRefreshRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!deferredRefreshPhone || !deferredRefreshPassword) throw new Error("missing deferred marketplace refresh login inputs");
await act(async () => {
  setInput(deferredRefreshPhone, "13900000004");
  setInput(deferredRefreshPassword, "local-secret");
  await flush();
});
await act(async () => {
  deferredRefreshRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("deferred authentication notification", () => deferredAuthenticationStarted === 1);
ok(deferredAuthenticationCompleted === 0, "authentication waits for the shared refresh coordinator to settle");
await act(async () => {
  resolveDeferredAuthentication();
  await deferredAuthentication;
  await flush();
});
await waitFor("deferred authentication completion", () => deferredAuthenticationCompleted === 1);
ok(deferredAuthenticationStarted === 1, "authentication notifies the shared refresh coordinator once");
await dispose(deferredRefreshRender.root, deferredRefreshRender.container);

const registrationAuthenticatedScopes: string[] = [];
const registrationRefreshRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudRegister: async () => authenticatedView,
}), "default", (accountScopeKey) => {
  registrationAuthenticatedScopes.push(accountScopeKey);
});
await act(async () => {
  registrationRefreshRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const registrationRefreshPhone = registrationRefreshRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const registrationRefreshPassword = registrationRefreshRender.container.querySelector<HTMLInputElement>('input[name="password"]');
const registrationRefreshSMS = registrationRefreshRender.container.querySelector<HTMLInputElement>('input[name="sms_code"]');
if (!registrationRefreshPhone || !registrationRefreshPassword || !registrationRefreshSMS) throw new Error("missing marketplace refresh registration inputs");
await act(async () => {
  setInput(registrationRefreshPhone, "13900000003");
  setInput(registrationRefreshPassword, "registration-password");
  setInput(registrationRefreshSMS, "123456");
  await flush();
});
await act(async () => {
  registrationRefreshRender.container.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
  await flush();
});
await waitFor("registration authentication notification", () => registrationAuthenticatedScopes.length === 1);
ok(registrationAuthenticatedScopes[0] === "default\u0000cloud-user\u00001", "successful registration forwards the authenticated account scope once");
await dispose(registrationRefreshRender.root, registrationRefreshRender.container);

const assetRefreshAuthenticatedScopes: string[] = [];
const assetRefreshRender = await renderAccount(apiWith(async () => anonymous(fixtureCapabilities), {
  CloudLogin: async () => authenticatedView,
  CloudRefreshAssets: async () => authenticatedView,
}), "default", (accountScopeKey) => {
  assetRefreshAuthenticatedScopes.push(accountScopeKey);
});
const assetRefreshPhone = assetRefreshRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const assetRefreshPassword = assetRefreshRender.container.querySelector<HTMLInputElement>('input[name="password"]');
if (!assetRefreshPhone || !assetRefreshPassword) throw new Error("missing asset refresh login inputs");
await act(async () => {
  setInput(assetRefreshPhone, "13900000005");
  setInput(assetRefreshPassword, "local-secret");
  await flush();
});
await act(async () => {
  assetRefreshRender.container.querySelector<HTMLButtonElement>('[data-account-action="login"]')?.click();
  await flush();
});
await waitFor("asset refresh authenticated state", () => assetRefreshAuthenticatedScopes.length === 1);
await act(async () => {
  assetRefreshRender.container.querySelector<HTMLButtonElement>('[data-account-action="refresh"]')?.click();
  await flush();
});
await waitFor("asset refresh authentication notification", () => assetRefreshAuthenticatedScopes.length === 2);
ok(
  assetRefreshAuthenticatedScopes.every((scope) => scope === "default\u0000cloud-user\u00001"),
  "authenticated asset refresh reopens the same shared model-catalog scope",
);
await dispose(assetRefreshRender.root, assetRefreshRender.container);

console.log("\naccount center registration mismatch handling");

const registrationCandidate = {
  status: "authenticated" as const,
  profile_id: "default",
  cloud_user_id: "cloud-new-user",
  binding_generation: 2,
  display_name: "New cloud user",
};
const switchRequests: CloudLoginRequest[] = [];
const mismatchAPI = apiWith(async () => anonymous(fixtureCapabilities), {
  CloudRegister: async () => ({
    ...anonymous(fixtureCapabilities),
    operation_error: { code: "PROFILE_SCOPE_MISMATCH", message: "confirm account switch" },
    candidate: registrationCandidate,
  }),
  CloudSwitchAccount: async (request) => {
    switchRequests.push(request);
    return {
      available: true,
      capabilities: fixtureCapabilities,
      account: registrationCandidate,
    };
  },
});
const mismatchRender = await renderAccount(mismatchAPI);
await act(async () => {
  mismatchRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const mismatchPhone = mismatchRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const mismatchPassword = mismatchRender.container.querySelector<HTMLInputElement>('input[name="password"]');
const mismatchSMS = mismatchRender.container.querySelector<HTMLInputElement>('input[name="sms_code"]');
if (!mismatchPhone || !mismatchPassword || !mismatchSMS) throw new Error("missing registration mismatch inputs");
await act(async () => {
  setInput(mismatchPhone, "13900000001");
  setInput(mismatchPassword, "registration-password");
  setInput(mismatchSMS, "123456");
  await flush();
});
await act(async () => {
  mismatchRender.container.querySelector<HTMLButtonElement>('[data-account-action="register"]')?.click();
  await flush();
});
ok(mismatchRender.container.querySelector('[data-account-switch-stage="confirm"]') !== null, "registration mismatch opens the explicit confirmation stage");
ok(mismatchRender.container.querySelector('[data-account-switch-candidate="cloud-new-user"]') !== null, "registration mismatch displays the staged account candidate");
ok(mismatchRender.container.querySelector<HTMLElement>('[role="alert"]')?.textContent?.includes("PROFILE_SCOPE_MISMATCH") !== true, "registration mismatch hides the internal operation code");
ok(switchRequests.length === 0, "registration mismatch never triggers a second account-switch request");
await dispose(mismatchRender.root, mismatchRender.container);

const unavailableRender = await renderAccount(apiWith(async () => anonymous(unavailableCapabilities)));
const unavailableTab = unavailableRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]');
if (!unavailableTab) throw new Error("missing unavailable registration tab");
ok(!unavailableTab.disabled, "connected cloud keeps registration enabled despite stale unavailable metadata");
ok(unavailableTab.textContent?.includes("后续版本开放") === false, "connected cloud never renders the obsolete future-version copy");
await act(async () => {
  invokeReactClick(unavailableTab);
  await flush();
});
ok(unavailableRender.container.querySelector('[data-account-auth-hint="real"]') !== null, "stale unavailable metadata is normalized to real authentication");
ok(unavailableRender.container.querySelector('[data-account-action="register"]') !== null, "connected cloud exposes registration after normalization");
ok(unavailableRender.container.querySelector('[data-account-action="captcha"]') !== null, "normalized registration requires the real CAPTCHA path");
await dispose(unavailableRender.root, unavailableRender.container);

console.log("\naccount center registration local-state lifetime");

let currentBootstrap = anonymous(fixtureCapabilities);
const lifecycleAPI = apiWith(async () => currentBootstrap);
const lifecycleRender = await renderAccount(lifecycleAPI, "context-a");
await act(async () => {
  lifecycleRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const lifecyclePhone = lifecycleRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const lifecycleSMS = lifecycleRender.container.querySelector<HTMLInputElement>('input[name="sms_code"]');
const lifecycleCaptcha = lifecycleRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]');
if (!lifecyclePhone || !lifecycleSMS || !lifecycleCaptcha) throw new Error("missing registration lifecycle controls");
await act(async () => {
  setInput(lifecyclePhone, "13800000000");
  setInput(lifecycleSMS, "654321");
  lifecycleCaptcha.click();
  await flush();
});

currentBootstrap = anonymous(fixtureCapabilities, "other-profile");
await act(async () => {
  useAccountStore.getState().applyBootstrap(currentBootstrap);
  await flush();
});
ok(lifecycleCaptcha.textContent?.includes("完成滑块验证") === true, "scope switch clears CAPTCHA state");
ok(lifecyclePhone.value === "13800000000" && lifecycleSMS.value === "654321", "scope switch preserves correctable phone and SMS inputs");

await act(async () => {
  lifecycleCaptcha.click();
  lifecycleRender.root.render(
    <LocaleProvider>
      <AccountCenter api={lifecycleAPI} contextKey="context-b" />
    </LocaleProvider>,
  );
  await flush();
});
await waitFor("new context bootstrap", () => useAccountStore.getState().scopeKey.startsWith("other-profile"));
await act(async () => {
  lifecycleRender.container.querySelector<HTMLButtonElement>('[data-account-mode="register"]')?.click();
  await flush();
});
const contextPhone = lifecycleRender.container.querySelector<HTMLInputElement>('input[name="phone"]');
const contextSMS = lifecycleRender.container.querySelector<HTMLInputElement>('input[name="sms_code"]');
const contextCaptcha = lifecycleRender.container.querySelector<HTMLButtonElement>('[data-account-action="captcha"]');
ok(contextPhone?.value === "" && contextSMS?.value === "", "context switch clears phone and SMS inputs");
ok(contextCaptcha?.textContent?.includes("完成滑块验证") === true, "context switch clears CAPTCHA state");
await dispose(lifecycleRender.root, lifecycleRender.container);

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
