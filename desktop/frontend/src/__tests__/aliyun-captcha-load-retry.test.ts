// Run: tsx src/__tests__/aliyun-captcha-load-retry.test.ts

import { JSDOM } from "jsdom";

import {
  ALIYUN_CAPTCHA_SCRIPT_URL,
  preloadAliyunCaptcha,
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

console.log("\nAliyun CAPTCHA script retry");

const dom = new JSDOM("<!doctype html><html><head></head><body></body></html>", {
  url: "http://localhost/",
});
const sdkWindow = dom.window as unknown as Window & typeof globalThis;
const sdkDocument = dom.window.document;

const failedLoad = preloadAliyunCaptcha(sdkWindow, sdkDocument);
const failedScript = sdkDocument.querySelector<HTMLScriptElement>(`script[src="${ALIYUN_CAPTCHA_SCRIPT_URL}"]`);
if (!failedScript) throw new Error("missing initial CAPTCHA script");
failedScript.dispatchEvent(new dom.window.Event("error"));
let rejected = false;
try {
  await failedLoad;
} catch {
  rejected = true;
}
ok(rejected, "rejects the failed CDN load");
ok(!failedScript.isConnected, "removes the failed official script node");

const retryLoad = preloadAliyunCaptcha(sdkWindow, sdkDocument);
const retryScript = sdkDocument.querySelector<HTMLScriptElement>(`script[src="${ALIYUN_CAPTCHA_SCRIPT_URL}"]`);
ok(retryScript !== null && retryScript !== failedScript, "retry creates a new official script node");
retryScript?.dispatchEvent(new dom.window.Event("load"));
let missingInitRejected = false;
try {
  await retryLoad;
} catch {
  missingInitRejected = true;
}
ok(missingInitRejected && retryScript?.isConnected === false, "removes a loaded script when the SDK init function is missing");

const finalLoad = preloadAliyunCaptcha(sdkWindow, sdkDocument);
const finalScript = sdkDocument.querySelector<HTMLScriptElement>(`script[src="${ALIYUN_CAPTCHA_SCRIPT_URL}"]`);
ok(finalScript !== null && finalScript !== retryScript, "missing SDK init can retry with another new script node");
(sdkWindow as typeof sdkWindow & { initAliyunCaptcha?: (options: AliyunCaptchaSDKOptions) => void }).initAliyunCaptcha = () => undefined;
finalScript?.dispatchEvent(new dom.window.Event("load"));
await finalLoad;
ok(true, "new script can load successfully after a failure");

dom.window.close();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
