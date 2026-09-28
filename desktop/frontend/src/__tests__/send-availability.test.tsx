// Run: tsx src/__tests__/send-availability.test.tsx

import { JSDOM } from "jsdom";
import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { SendAvailabilityBanner } from "../components/SendAvailabilityBanner";
import { HistoryLoadErrorBanner } from "../components/HistoryLoadErrorBanner";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import {
  resolveDisplayedSendAvailability,
  sendAvailabilityAllowsControllerActions,
} from "../lib/sendAvailability";
import type { TabSendAvailability } from "../lib/types";

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

const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", { pretendToBeVisual: true, url: "http://localhost/" });
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
globalThis.window = dom.window as unknown as Window & typeof globalThis;
globalThis.document = dom.window.document;
Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
globalThis.Node = dom.window.Node;
globalThis.HTMLElement = dom.window.HTMLElement;
globalThis.Event = dom.window.Event;
globalThis.MouseEvent = dom.window.MouseEvent;
globalThis.localStorage = dom.window.localStorage;
globalThis.requestAnimationFrame = dom.window.requestAnimationFrame.bind(dom.window);
globalThis.cancelAnimationFrame = dom.window.cancelAnimationFrame.bind(dom.window);

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const root = createRoot(rootElement);
const actions: string[] = [];
const renderBanner = async (availability: TabSendAvailability, hasHistory: boolean) => {
  await act(async () => {
    root.render(
      <LocaleProvider>
        <SendAvailabilityBanner
          availability={availability}
          hasHistory={hasHistory}
          onLogin={() => { actions.push("login"); }}
          onOpenAccount={() => { actions.push("account"); }}
          onOpenPayments={() => { actions.push("payments"); }}
          onConfigureBYOK={() => { actions.push("configure"); }}
          onRetry={() => { actions.push("retry"); }}
          onSwitchModel={() => { actions.push("model"); }}
        />
      </LocaleProvider>,
    );
  });
};

console.log("\nsend availability");

ok(sendAvailabilityAllowsControllerActions({ state: "ready", retryable: false }), "ready availability enables controller actions");
for (const state of ["starting", "subscription_auth_required", "subscription_account_action_required", "subscription_balance_depleted", "byok_configuration_required", "service_unavailable", "startup_failed"] as const) {
  ok(!sendAvailabilityAllowsControllerActions({ state, retryable: true }), `${state} disables controller actions`);
}

const staleReadyAfterLogout = resolveDisplayedSendAvailability({
  availability: { state: "ready", retryable: false },
  billingSource: "subscription_pool",
  accountState: "anonymous",
});
ok(
  staleReadyAfterLogout?.state === "subscription_auth_required"
    && staleReadyAfterLogout.action === "login"
    && staleReadyAfterLogout.retryable,
  "anonymous subscription route overrides stale ready tab state with sign-in recovery",
);
ok(
  resolveDisplayedSendAvailability({
    availability: { state: "ready", retryable: false },
    billingSource: "newapi_byok",
    accountState: "anonymous",
  })?.state === "ready",
  "anonymous account does not override a ready custom model route",
);
ok(
  resolveDisplayedSendAvailability({
    availability: { state: "ready", retryable: false },
    billingSource: "subscription_pool",
    accountState: "ready",
  })?.state === "ready",
  "authenticated subscription route preserves backend readiness",
);
ok(
  resolveDisplayedSendAvailability({
    availability: { state: "ready", retryable: false },
    billingSource: "subscription_pool",
    accountState: "ready",
    totalBalance: "0.0000",
  })?.state === "subscription_balance_depleted",
  "zero subscription balance stops controller actions",
);
ok(
  resolveDisplayedSendAvailability({
    availability: { state: "ready", retryable: false },
    billingSource: "subscription_pool",
    accountState: "ready",
    totalBalance: "0.0001",
  })?.state === "ready",
  "positive subscription balance remains usable below the reminder threshold",
);
ok(
  resolveDisplayedSendAvailability({
    availability: { state: "ready", retryable: false },
    billingSource: "newapi_byok",
    accountState: "ready",
    totalBalance: "0",
  })?.state === "ready",
  "zero cloud balance does not block BYOK sending",
);
ok(
  resolveDisplayedSendAvailability({
    availability: { state: "service_unavailable", retryable: true, action: "retry" },
    billingSource: "subscription_pool",
    accountState: "ready",
    totalBalance: "0",
  })?.state === "service_unavailable",
  "cached balance never masks a backend startup or service failure",
);

await renderBanner({ state: "subscription_auth_required", retryable: true, action: "login", detail: "auth_required" }, true);
ok(document.body.textContent?.includes("继续当前会话") ?? false, "signed-out history explains how to continue the same conversation");
ok(document.body.textContent?.includes("本地历史") ?? false, "signed-out history states that local history remains visible");
ok(document.querySelectorAll("button").length === 2, "signed-out recovery only offers login and model switch");
ok(!document.body.textContent?.includes("重试"), "signed-out recovery does not offer an ineffective retry");
ok(document.querySelector(".send-availability--auth .send-availability__title")?.textContent === "登录后使用官方订阅", "signed-out guidance uses a clear recovery title");
ok(document.querySelector(".send-availability__content .banner__details") !== null, "diagnostic detail stays grouped with the status copy");
ok(document.querySelector(".send-availability__icon") !== null, "signed-out guidance uses the themed compact status treatment");
ok(document.querySelector(".send-availability__actions .btn--primary") !== null, "the primary recovery action is visually prioritized");
ok(document.querySelector("[role=\"status\"]")?.getAttribute("aria-live") === "polite", "availability changes are announced without interrupting the user");

await renderBanner({ state: "subscription_auth_required", retryable: true, action: "login" }, false);
const blankAuthText = document.body.textContent ?? "";
ok(!blankAuthText.includes("本地历史") && !blankAuthText.includes("继续当前会话"), "blank signed-out session makes no preserved-history claim");

await renderBanner({ state: "byok_configuration_required", retryable: true, action: "configure_byok" }, false);
const byokText = document.body.textContent ?? "";
ok(byokText.includes("API 地址") && byokText.includes("访问密钥"), "custom model service failure explains the required configuration");
ok(!byokText.includes("登录"), "BYOK failure does not ask the user to sign in");
const configureButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes("配置模型服务"));
await act(async () => configureButton?.click());
ok(actions.includes("configure"), "BYOK settings action remains available while sending is disabled");
ok(!document.body.textContent?.includes("重试"), "BYOK configuration recovery does not offer retry before configuration");

await renderBanner({ state: "subscription_balance_depleted", retryable: false, action: "account" }, true);
ok(document.body.textContent?.includes("积分已用完") ?? false, "depleted subscription banner explains why sending stopped");
const rechargeButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes("充值获取更多积分"));
await act(async () => rechargeButton?.click());
ok(actions.includes("payments"), "depleted subscription banner opens the recharge flow");

await renderBanner({ state: "service_unavailable", retryable: true, action: "retry" }, false);
const retryButton = Array.from(document.querySelectorAll("button")).find((button) => button.textContent?.includes("重试"));
ok(Boolean(retryButton), "transient service failures still offer retry");
await act(async () => retryButton?.click());
ok(actions.includes("retry"), "retryable service failure invokes the retry action");

await act(async () => {
  root.render(
    <LocaleProvider>
      <HistoryLoadErrorBanner error="disk detail" />
      <Transcript items={[]} hydrating onPrompt={() => {}} />
    </LocaleProvider>,
  );
});
const blankHistoryErrorText = document.body.textContent ?? "";
ok(blankHistoryErrorText.includes("未切换为空白会话"), "blank history error uses neutral fail-closed wording");
ok(!blankHistoryErrorText.includes("保留") && !blankHistoryErrorText.includes("保存"), "blank history error makes no unsupported preserved-content claim");
ok(document.querySelector(".welcome") === null, "blank history error suppresses the Welcome page");
ok(document.querySelector("[data-history-load-error] code")?.textContent === "disk detail", "history error preserves diagnostic detail");

await act(async () => root.unmount());
dom.window.close();

console.log(`\n${passed} passed, ${failed} failed, ${passed + failed} total`);
if (failed > 0) process.exit(1);
