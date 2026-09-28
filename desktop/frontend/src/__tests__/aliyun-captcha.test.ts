// Run: tsx src/__tests__/aliyun-captcha.test.ts

import { JSDOM } from "jsdom";

import {
  ALIYUN_CAPTCHA_PROVIDER,
  ALIYUN_CAPTCHA_SCRIPT_URL,
  AliyunCaptchaAdapter,
  preloadAliyunCaptcha,
  validateAliyunCaptchaConfig,
  type AliyunCaptchaProof,
  type AliyunCaptchaSDKOptions,
} from "../lib/aliyunCaptcha";

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

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((nextResolve) => { resolve = nextResolve; });
  return { promise, resolve };
}

console.log("\nAliyun CAPTCHA adapter");

const config = validateAliyunCaptchaConfig({
  provider: "aliyun_captcha_v2",
  region: "cn",
  prefix: "public-prefix",
  scene_id: "register-scene",
});
ok(config.provider === ALIYUN_CAPTCHA_PROVIDER && config.scene_id === "register-scene", "accepts the discovered public config");

for (const invalid of [
  { ...config, provider: "other" },
  { ...config, region: "us" },
  { ...config, prefix: " reasonix " },
  { ...config, scene_id: "reasonix-register-dev" },
  { ...config, secret: "must-not-cross-the-bridge" },
]) {
  let rejected = false;
  try {
    validateAliyunCaptchaConfig(invalid);
  } catch {
    rejected = true;
  }
  ok(rejected, "rejects malformed, development, or secret-bearing config");
}

const dom = new JSDOM("<!doctype html><html><head></head><body><div id='captcha'></div><button id='captcha-button'>verify</button></body></html>", {
  pretendToBeVisual: true,
  url: "http://localhost/",
});
const sdkWindow = dom.window as unknown as Window & typeof globalThis;
const sdkDocument = dom.window.document;
let sdkOptions: AliyunCaptchaSDKOptions | undefined;
let initCalls = 0;
let destroyCalls = 0;
const instances = Array.from({ length: 5 }, () => ({ destroy: () => { destroyCalls += 1; } }));

const firstLoad = preloadAliyunCaptcha(sdkWindow, sdkDocument);
const secondLoad = preloadAliyunCaptcha(sdkWindow, sdkDocument);
const scripts = sdkDocument.querySelectorAll<HTMLScriptElement>(`script[src="${ALIYUN_CAPTCHA_SCRIPT_URL}"]`);
ok(scripts.length === 1, "loads the official CDN script once");
(sdkWindow as typeof sdkWindow & { initAliyunCaptcha?: (options: AliyunCaptchaSDKOptions) => void }).initAliyunCaptcha = (options) => {
  sdkOptions = options;
  options.getInstance?.(instances[initCalls]);
  initCalls += 1;
};
scripts[0]?.dispatchEvent(new dom.window.Event("load"));
await Promise.all([firstLoad, secondLoad]);

const proofs: AliyunCaptchaProof[] = [];
const errors: unknown[] = [];
const proofCompletion = deferred<void>();
const element = sdkDocument.querySelector<HTMLElement>("#captcha");
const button = sdkDocument.querySelector<HTMLButtonElement>("#captcha-button");
if (!element || !button) throw new Error("missing CAPTCHA test elements");
const adapter = new AliyunCaptchaAdapter({
  config,
  window: sdkWindow,
  document: sdkDocument,
  element,
  button,
  onProof: (proof) => {
    proofs.push(proof);
    return proofCompletion.promise;
  },
  onError: (error) => { errors.push(error); },
});
await adapter.init();
ok(initCalls === 1, "initializes popup mode from discovered config");
ok(sdkOptions?.SceneId === config.scene_id && sdkOptions.mode === "popup", "passes the discovered scene to the SDK");
ok((sdkWindow as typeof sdkWindow & { AliyunCaptchaConfig?: unknown }).AliyunCaptchaConfig !== undefined, "sets the public SDK region and prefix config");

const rawTicket = `{"sceneId":"register-scene","token":"opaque"}`;
sdkOptions?.success?.(rawTicket);
await flush();
ok(proofs.length === 0, "ignores SDK success without a user intent");

button.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
sdkOptions?.success?.(rawTicket);
button.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
sdkOptions?.success?.("concurrent-ticket");
await flush();
ok(proofs.length === 1 && destroyCalls === 0, "rejects a second intent and success while proof consumption is pending");
proofCompletion.resolve();
await flush();
ok(proofs[0]?.ticket === rawTicket && proofs[0]?.provider === ALIYUN_CAPTCHA_PROVIDER, "forwards the first raw proof unchanged");
ok(destroyCalls === 1 && initCalls === 2, "destroys and rebuilds after success");

button.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
sdkOptions?.success?.("fresh-ticket");
await flush();
ok(proofs.length === 2, "accepts a fresh user proof after consumption rebuild completes");

button.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
sdkOptions?.fail?.({ code: "CAPTCHA_FAILED" });
await flush();
ok(errors.length === 1 && destroyCalls === 3 && initCalls === 4, "destroys and rebuilds after SDK failure");

adapter.destroy();
button.dispatchEvent(new dom.window.MouseEvent("click", { bubbles: true }));
sdkOptions?.success?.("after-destroy");
await flush();
ok(proofs.length === 2 && destroyCalls === 4, "cleans up listeners and SDK state on destroy");

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
