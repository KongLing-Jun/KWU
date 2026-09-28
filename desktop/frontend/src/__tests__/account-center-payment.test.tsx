// Run: tsx src/__tests__/account-center-payment.test.tsx

import { JSDOM } from "jsdom";
import React from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";

import { AccountPayments, type AccountPaymentBindings } from "../components/AccountPayments";
import { LocaleProvider } from "../lib/i18n";
import {
  type CloudAccountView,
  type CloudBootstrapView,
  type CloudPaymentOperationView,
  type CloudPaymentView,
  type CloudSubscriptionPlan,
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

function eq(actual: unknown, expected: unknown, label: string) {
  ok(
    JSON.stringify(actual) === JSON.stringify(expected),
    `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
  );
}

async function flush(milliseconds = 0) {
  await new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function waitFor(label: string, predicate: () => boolean) {
  for (let attempt = 0; attempt < 120; attempt += 1) {
    await act(async () => flush(5));
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

const account: CloudAccountView = {
  status: "authenticated",
  profile_id: "profile-a",
  cloud_user_id: "usr_01",
  binding_generation: 3,
  display_name: "Fixture User",
};
const plans: CloudSubscriptionPlan[] = [
  {
    plan_id: "plan-pro",
    plan: "pro",
    display_name: "Pro DTO",
    price_amount: "59.9000",
    currency: "CNY",
    credit_amount: "120000.0000",
    period_days: 30,
    enabled: true,
    sort_order: 30,
    config_version: "catalog-v1",
  },
  {
    plan_id: "plan-lite",
    plan: "lite",
    display_name: "Lite DTO",
    price_amount: "9.9000",
    currency: "CNY",
    credit_amount: "20000.0000",
    period_days: 30,
    enabled: true,
    sort_order: 10,
    config_version: "catalog-v1",
  },
  {
    plan_id: "plan-plus",
    plan: "plus",
    display_name: "Plus DTO",
    price_amount: "29.9000",
    currency: "CNY",
    credit_amount: "60000.0000",
    period_days: 30,
    enabled: true,
    sort_order: 20,
    config_version: "catalog-v1",
  },
];

function payment(
  orderID: string,
  state: CloudPaymentView["state"],
  overrides: Partial<CloudPaymentView> = {},
): CloudPaymentView {
  const payable = state === "paying";
  return {
    order_id: orderID,
    order_type: "subscription_purchase",
    plan_id: "plan-plus",
    plan: "plus",
    amount: "29.9000",
    currency: "CNY",
    payment_channel: "alipay",
    state,
    payable,
    entitlement_granted: state === "payment_succeeded",
    expires_at: "2030-01-01T00:30:00Z",
    payment_entry: payable
      ? {
        entry_type: "qr_code",
        qr_code_url: `https://pay.test/${orderID}`,
        expires_at: "2030-01-01T00:30:00Z",
      }
      : null,
    version: state === "payment_succeeded" ? 3 : 1,
    ...overrides,
  };
}

function freeView(payments: CloudPaymentView[] = []): CloudBootstrapView {
  return {
    available: true,
    account,
    balance: { total_balance: "500.0000" },
    subscription: {
      plans,
      current: { plan: "free", status: "free", updated_at: "2026-07-12T00:00:00Z" },
    },
    payments,
  };
}

function operationFor(planID: string, channel: string, orderID: string): CloudPaymentOperationView {
  const plan = plans.find((candidate) => (candidate.plan_id ?? candidate.plan) === planID);
  if (!plan || (channel !== "alipay" && channel !== "wechat")) throw new Error("bad test purchase input");
  return {
    account,
    payment: payment(orderID, "paying", {
      plan_id: plan.plan_id,
      plan: plan.plan,
      amount: plan.price_amount,
      payment_channel: channel,
      payment_entry: {
        entry_type: "qr_code",
        qr_code_url: `https://pay.test/${orderID}`,
        expires_at: "2030-01-01T00:30:00Z",
      },
    }),
  };
}

function baseAPI(overrides: Partial<AccountPaymentBindings> = {}): AccountPaymentBindings {
  return {
    CloudCreateSubscriptionOrder: async () => { throw new Error("create is outside this scenario"); },
    CloudCreateUpgradeOrder: async () => { throw new Error("upgrade is outside the supported client scope"); },
    CloudConfirmPayment: async () => { throw new Error("V3 does not use manual confirmation"); },
    CloudRefreshPayment: async () => { throw new Error("poll is outside this scenario"); },
    ...overrides,
  };
}

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("missing root");
const mountedRootElement = rootElement;
let root = createRoot(mountedRootElement);

async function render(
  view: CloudBootstrapView,
  api: AccountPaymentBindings,
  pollIntervalMs = 60_000,
) {
  await act(async () => root.unmount());
  root = createRoot(mountedRootElement);
  useAccountStore.getState().reset();
  useAccountStore.getState().applyBootstrap(view);
  const scopeKey = useAccountStore.getState().scopeKey;
  await act(async () => {
    root.render(
      <LocaleProvider>
        <AccountPayments api={api} scopeKey={scopeKey} pollIntervalMs={pollIntervalMs} />
      </LocaleProvider>,
    );
    await flush();
  });
}

async function choosePurchase(plan: string, channel: CloudPaymentView["payment_channel"]) {
  await act(async () => {
    invokeReactClick(
      document.querySelector<HTMLButtonElement>(`[data-payment-channel-option="${channel}"]`)!,
    );
    await flush();
  });
  await act(async () => {
    invokeReactClick(
      document.querySelector<HTMLButtonElement>(`[data-payment-create-plan="${plan}"]`)!,
    );
    await flush();
  });
}

console.log("\n套餐展示、创建忙状态与订单返回");
const firstCreate = deferred<CloudPaymentOperationView>();
const purchaseCalls: Array<[string, string]> = [];
let createdOrderSequence = 1;
const purchaseAPI = baseAPI({
  CloudCreateSubscriptionOrder: async (planID, channel) => {
    purchaseCalls.push([planID, channel]);
    if (purchaseCalls.length === 1) return firstCreate.promise;
    createdOrderSequence += 1;
    return operationFor(planID, channel, `created-order-${createdOrderSequence}`);
  },
});
await render(freeView(), purchaseAPI);
eq(
  Array.from(document.querySelectorAll<HTMLElement>("[data-payment-plan]")).map((card) => card.dataset.paymentPlan),
  ["pro"],
  "前端只展示体验套餐",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-price]')?.textContent?.trim(),
  "¥59.90/ 30 天",
  "体验套餐仍使用云端价格与周期",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-credit] small')?.textContent?.trim(),
  "每月积分",
  "体验套餐展示每月积分标签",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-credit] strong')?.textContent?.trim(),
  "120,000",
  "体验套餐展示云端返回的每月总积分",
);
ok(document.querySelector('[data-payment-plan="pro"] .account-payments__plan-summary') !== null, "套餐左侧复用价格摘要");
ok(document.querySelector('[data-payment-plan="pro"] .account-payments__plan-details') !== null, "套餐右侧复用权益和购买入口");
ok(document.querySelector("[data-payment-plan-base-credit]") === null, "不再拆分基础积分");
ok(document.querySelector("[data-payment-plan-bonus-credit]") === null, "不再拆分赠送积分");
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-models] small')?.textContent,
  "Claude、GPT、Gemini、DeepSeek、GLM、Qwen",
  "Pro 展示参考套餐的模型示例",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-automation-tasks] small')?.textContent?.trim(),
  "自动任务",
  "体验套餐展示自动化任务标签",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-automation-tasks] strong')?.textContent?.trim(),
  "无限",
  "体验套餐展示无限自动化任务",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-projects] small')?.textContent?.trim(),
  "创建项目",
  "体验套餐展示项目标签",
);
eq(
  document.querySelector('[data-payment-plan="pro"] [data-payment-plan-projects] strong')?.textContent?.trim(),
  "无限",
  "体验套餐将真实 1000 项目上限展示为无限",
);
ok(
  document.querySelector('[data-payment-plan="pro"] .account-payments__plan-heading strong')?.textContent === "体验套餐",
  "套餐标题使用前端体验套餐名称",
);
ok(document.querySelector('[data-payment-create-plan="pro"]') !== null, "Free 用户可以直接购买体验套餐");
ok(document.querySelector('[data-payment-plan="lite"]') === null && document.querySelector('[data-payment-plan="plus"]') === null, "Lite 和 Plus 不展示");
ok(document.querySelector("[data-payment-plan-period-end]") === null, "Free 用户不显示伪造的套餐到期时间");
eq(
  Array.from(document.querySelectorAll<HTMLElement>("[data-payment-channel-option]"))
    .map((action) => action.dataset.paymentChannelOption),
  ["alipay", "wechat"],
  "支付方式为支付宝和微信预留稳定的并排位置",
);
ok(
  document.querySelector<HTMLButtonElement>('[data-payment-channel-option="alipay"]')?.disabled === false
    && document.querySelector<HTMLButtonElement>('[data-payment-channel-option="wechat"]')?.disabled === true,
  "真实验收范围仅开放支付宝，微信明确标记为即将开放",
);
const catalogTerms = document.querySelector("[data-payment-catalog-notes]")?.textContent ?? "";
ok(catalogTerms.includes("套餐有效期 30 天，支付成功后自动生效。"), "套餐目录展示统一有效期说明");
ok(catalogTerms.includes("订阅到期后套餐权益失效，积分清零"), "套餐目录展示到期积分清零说明");
ok(catalogTerms.includes("未登录：自动任务 5 个、项目 0 个；Free：自动任务 10 个、项目 5 个"), "套餐目录展示未登录与 Free 权益说明");
eq(document.querySelectorAll("[data-payment-create]").length, 1, "只有体验套餐提供下单按钮");

await choosePurchase("pro", "alipay");
ok(
  Array.from(document.querySelectorAll<HTMLButtonElement>("[data-payment-create]")).every((button) => button.disabled),
  "仅创建请求进行中时禁用下单入口",
);
await act(async () => {
  firstCreate.resolve(operationFor("plan-pro", "alipay", "created-order-1"));
  await flush();
});
eq(purchaseCalls, [["plan-pro", "alipay"]], "创建订单传真实 plan_id 与支付方式");
ok(
  document.querySelector('[data-payment-stage-view="scan"]') !== null
    && document.querySelector("[data-payment-plans]") === null,
  "二维码创建后进入独立扫码阶段",
);
ok(
  document.querySelector('[data-payment-order="created-order-1"] [data-payment-entry-field="qr_code_url"]') !== null,
  "展示云端 qr_code_url",
);
eq(
  document.querySelector('[data-payment-order="created-order-1"] svg')?.getAttribute("width"),
  "192",
  "二维码使用适合扫码的尺寸",
);
ok(document.querySelector("[data-payment-countdown]") !== null, "二维码展示到期倒计时");
ok(document.querySelector("[data-payment-confirm]") === null, "V3 不显示手动确认入口");

await act(async () => {
  invokeReactClick(document.querySelector<HTMLButtonElement>("[data-payment-collapse]")!);
  await flush();
});
ok(document.querySelector("[data-payment-order]") === null, "返回选择套餐会收起当前订单");
ok(
  document.querySelector('[data-payment-order-summary="created-order-1"]') !== null,
  "收起只改变展示，不伪装取消云端订单",
);
await act(async () => {
  invokeReactClick(
    document.querySelector<HTMLButtonElement>('[data-payment-select-order="created-order-1"]')!,
  );
  await flush();
});
ok(document.querySelector('[data-payment-order="created-order-1"]') !== null, "可从进行中订单重新打开二维码");

console.log("\n重复选择体验套餐复用现有订单");
await act(async () => {
  invokeReactClick(document.querySelector<HTMLButtonElement>("[data-payment-collapse]")!);
  await flush();
});
await choosePurchase("pro", "alipay");
eq(purchaseCalls.length, 1, "重复点击体验套餐不会创建第二笔订单");
ok(document.querySelector("[data-payment-switch-confirm]") === null, "查看相同订单不弹切换确认");
await act(async () => {
  invokeReactClick(document.querySelector<HTMLButtonElement>("[data-payment-collapse]")!);
  await flush();
});
await choosePurchase("pro", "alipay");
eq(purchaseCalls.length, 1, "再次点击同套餐同渠道仍复用现有订单");
ok(document.querySelector("[data-payment-switch-confirm]") === null, "查看相同订单不弹切换确认");

console.log("\n无二维码的进行中订单也必须经过切换确认");
const missingEntryCalls: Array<[string, string]> = [];
const missingEntryAPI = baseAPI({
  CloudCreateSubscriptionOrder: async (planID, channel) => {
    missingEntryCalls.push([planID, channel]);
    return operationFor(planID, channel, "created-after-missing-entry");
  },
});
await render(
  freeView([
    payment("entry-missing", "order_created", {
      payable: false,
      payment_entry: null,
    }),
  ]),
  missingEntryAPI,
);
ok(document.querySelector("[data-payment-entry-unavailable]") !== null, "无二维码订单使用紧凑等待状态");
await act(async () => {
  invokeReactClick(document.querySelector<HTMLButtonElement>("[data-payment-collapse]")!);
  await flush();
});
const missingEntryTarget = document.querySelector<HTMLButtonElement>('[data-payment-create-plan="pro"]')!;
ok(!missingEntryTarget.disabled, "非确认中的订单仍允许发起套餐切换");
await act(async () => {
  invokeReactClick(missingEntryTarget);
  await flush();
});
ok(document.querySelector("[data-payment-switch-confirm]") !== null, "无二维码进行中订单不会直接创建新订单");
eq(missingEntryCalls.length, 0, "用户确认切换前不创建新订单");
await act(async () => {
  invokeReactClick(document.querySelector<HTMLButtonElement>("[data-payment-switch-continue]")!);
  await flush();
});
eq(missingEntryCalls, [["plan-pro", "alipay"]], "无二维码进行中订单经明确确认后才切换");

console.log("\n支付确认中禁止创建任何新订单");
const confirmingCreateCalls: Array<[string, string]> = [];
await render(
  freeView([
    payment("confirming-order", "payment_confirming", {
      payable: false,
      payment_entry: null,
      entitlement_granted: false,
    }),
    payment("previous-timeout", "payment_timeout", {
      payable: false,
      payment_entry: null,
    }),
  ]),
  baseAPI({
    CloudCreateSubscriptionOrder: async (planID, channel) => {
      confirmingCreateCalls.push([planID, channel]);
      return operationFor(planID, channel, "should-not-create");
    },
  }),
);
ok(
  document.querySelector('[data-payment-stage-view="confirm"] [data-payment-progress]') !== null
    && document.querySelector("[data-payment-entry]") === null,
  "支付确认阶段移除二维码并展示真实进度",
);
await act(async () => {
  invokeReactClick(document.querySelector<HTMLButtonElement>("[data-payment-collapse]")!);
  await flush();
});
ok(document.querySelector("[data-payment-confirming-guard]") !== null, "返回套餐后展示明确的新购阻止提示");
ok(
  Array.from(
    document.querySelectorAll<HTMLButtonElement>("[data-payment-create], [data-payment-reorder]"),
  ).every((button) => button.disabled),
  "支付确认中禁用购买和重新下单入口",
);
const disabledCreate = document.querySelector<HTMLButtonElement>("[data-payment-create]")!;
await act(async () => {
  invokeReactClick(disabledCreate);
  await flush();
});
eq(confirmingCreateCalls.length, 0, "即使旧事件被触发也不会绕过支付确认保护");
ok(document.querySelector('[data-payment-order="confirming-order"]') !== null, "阻止新购时回到确认中的订单");

console.log("\n支付确认完成后按权益发放事实推进");
await render(
  freeView([
    payment("granting-order", "payment_succeeded", {
      payable: false,
      payment_entry: null,
      entitlement_granted: false,
    }),
  ]),
  baseAPI(),
);
ok(
  document.querySelector('[data-payment-stage-view="confirm"]') !== null
    && document.querySelector("[data-payment-entry]") === null,
  "支付已确认但权益未发放时停留在云端确认阶段",
);
eq(
  document.querySelectorAll('[data-payment-progress-state="complete"]').length,
  2,
  "支付已确认时只完成订单与支付两步",
);

await render(
  freeView([
    payment("succeeded-order", "payment_succeeded", {
      payable: false,
      payment_entry: null,
      entitlement_granted: true,
    }),
  ]),
  baseAPI(),
);
await act(async () => {
  invokeReactClick(
    document.querySelector<HTMLButtonElement>('[data-payment-select-order="succeeded-order"]')!,
  );
  await flush();
});
ok(
  document.querySelector('[data-payment-stage-view="success"]') !== null
    && document.querySelector("[data-payment-entry]") === null,
  "权益已发放后展示成功结果且不保留二维码",
);
eq(
  document.querySelectorAll('[data-payment-progress-state="complete"]').length,
  3,
  "成功结果完成全部三步",
);

console.log("\n多个非终态订单都会轮询并分别结束");
const multiRefreshCalls: string[] = [];
const multiPollAPI = baseAPI({
  CloudRefreshPayment: async (orderID) => {
    multiRefreshCalls.push(orderID);
    const original = useAccountStore.getState().payments.find((candidate) => candidate.order_id === orderID);
    if (!original) throw new Error("missing order");
    return {
      account,
      payment: {
        ...original,
        state: "payment_timeout",
        payable: false,
        payment_entry: null,
        version: (original.version ?? 1) + 1,
      },
    };
  },
});
await render(
  freeView([
    payment("poll-a", "paying"),
    payment("poll-b", "paying", { payment_channel: "wechat" }),
  ]),
  multiPollAPI,
  5,
);
await waitFor(
  "all pending orders reach terminal states",
  () => useAccountStore.getState().payments.every((candidate) => candidate.state === "payment_timeout"),
);
eq([...new Set(multiRefreshCalls)].sort(), ["poll-a", "poll-b"], "轮询覆盖所有非终态本地订单");
const callsAfterTerminal = multiRefreshCalls.length;
await act(async () => flush(25));
eq(multiRefreshCalls.length, callsAfterTerminal, "每个终态订单都停止轮询");

console.log("\n鉴权错误停止轮询，临时错误退避后恢复");
let authRefreshCalls = 0;
const authPollAPI = baseAPI({
  CloudRefreshPayment: async () => {
    authRefreshCalls += 1;
    throw { code: "TOKEN_EXPIRED", message: "expired" };
  },
});
await render(freeView([payment("auth-a", "paying"), payment("auth-b", "paying")]), authPollAPI, 5);
await waitFor("auth polling notice", () => document.querySelector('[data-payment-poll-notice="auth"]') !== null);
const callsWhenAuthStopped = authRefreshCalls;
await act(async () => flush(35));
eq(authRefreshCalls, callsWhenAuthStopped, "鉴权终止错误会停止当前 scope 的全部轮询");

let transientRefreshCalls = 0;
const transientRetry = deferred<CloudPaymentOperationView>();
const transientPollAPI = baseAPI({
  CloudRefreshPayment: async () => {
    transientRefreshCalls += 1;
    if (transientRefreshCalls === 1) throw { code: "NETWORK_UNAVAILABLE", message: "offline" };
    return transientRetry.promise;
  },
});
await render(freeView([payment("retry-order", "paying")]), transientPollAPI, 5);
await waitFor(
  "transient retry notice",
  () => document.querySelector('[data-payment-poll-notice="retrying"]') !== null,
);
await waitFor("transient retry request", () => transientRefreshCalls === 2);
const retryOriginal = useAccountStore.getState().payments[0]!;
await act(async () => {
  transientRetry.resolve({
    account,
    payment: {
      ...retryOriginal,
      state: "payment_timeout",
      payable: false,
      payment_entry: null,
      version: (retryOriginal.version ?? 1) + 1,
    },
  });
  await flush();
});
await waitFor(
  "transient retry succeeds",
  () => useAccountStore.getState().payments[0]?.state === "payment_timeout",
);
eq(transientRefreshCalls, 2, "临时错误退避后自动重试");
ok(document.querySelector("[data-payment-poll-notice]") === null, "刷新恢复后清除临时错误提示");

console.log("\n真实支付错误码按云端 retryable 契约停止或退避");
let notFoundRefreshCalls = 0;
const notFoundPollAPI = baseAPI({
  CloudRefreshPayment: async () => {
    notFoundRefreshCalls += 1;
    throw { code: "NOT_FOUND", message: "payment order was not found", retryable: false };
  },
});
await render(freeView([payment("missing-order", "paying")]), notFoundPollAPI, 5);
await waitFor(
  "not found polling stop",
  () => document.querySelector('[data-payment-poll-notice="stopped"]') !== null,
);
await act(async () => flush(35));
eq(notFoundRefreshCalls, 1, "NOT_FOUND 是非重试错误，当前订单停止轮询");

let realRetryableCalls = 0;
const realRetryablePollAPI = baseAPI({
  CloudRefreshPayment: async (orderID) => {
    realRetryableCalls += 1;
    if (realRetryableCalls === 1) {
      throw new Error("UPSTREAM_ERROR: payment provider is temporarily unavailable [request_id=req-payment]");
    }
    if (realRetryableCalls === 2) {
      throw {
        code: "CONCURRENT_OPERATION_RETRYABLE",
        message: "another payment operation is in progress",
        retryable: true,
      };
    }
    const original = useAccountStore.getState().payments.find((candidate) => candidate.order_id === orderID);
    if (!original) throw new Error("missing order");
    return {
      account,
      payment: {
        ...original,
        state: "payment_timeout",
        payable: false,
        payment_entry: null,
        version: (original.version ?? 1) + 1,
      },
    };
  },
});
await render(freeView([payment("real-retryable-order", "paying")]), realRetryablePollAPI, 5);
await waitFor(
  "real retryable payment errors recover",
  () => useAccountStore.getState().payments[0]?.state === "payment_timeout",
);
eq(realRetryableCalls, 3, "UPSTREAM_ERROR 和 CONCURRENT_OPERATION_RETRYABLE 均退避重试");
ok(document.querySelector("[data-payment-poll-notice]") === null, "真实临时错误恢复后清除轮询提示");

console.log("\n终态订单可以真正重新下单");
const reorderCalls: Array<[string, string]> = [];
const reorderAPI = baseAPI({
  CloudCreateSubscriptionOrder: async (planID, channel) => {
    reorderCalls.push([planID, channel]);
    return operationFor(planID, channel, "new-attempt-order");
  },
});
await render(
  freeView([
    payment("timed-out-order", "payment_timeout", {
      payable: false,
      payment_entry: null,
    }),
  ]),
  reorderAPI,
);
await act(async () => {
  invokeReactClick(
    document.querySelector<HTMLButtonElement>('[data-payment-reorder="timed-out-order"]')!,
  );
  await flush();
});
eq(reorderCalls, [["plan-plus", "alipay"]], "终态重新下单复用套餐和渠道");
ok(document.querySelector('[data-payment-order="new-attempt-order"]') !== null, "新 attempt 订单替换为当前展开订单");

console.log("\n已订阅用户只允许向上升级，套餐卡片状态与真实接口一致");
const upgradeCalls: Array<[string, string]> = [];
const upgradeAPI = baseAPI({
  CloudCreateSubscriptionOrder: async (planID, channel) => {
    upgradeCalls.push([planID, channel]);
    return operationFor(planID, channel, "upgrade-order");
  },
  CloudCreateUpgradeOrder: async () => { throw new Error("升级不应调用旧 subscription-upgrade-orders 接口"); },
});
await render({
  available: true,
  account,
  subscription: {
    plans,
    current: {
      plan: "lite",
      status: "active",
      period_end: "2026-08-11T00:00:00Z",
      updated_at: "2026-07-12T00:00:00Z",
    },
  },
  payments: [],
}, upgradeAPI);
ok(document.querySelector('[data-payment-plan-action="lite"]') === null, "当前 Lite 套餐卡片隐藏");
ok(document.querySelector('[data-payment-plan-action="plus"]') === null, "Plus 套餐卡片隐藏");
ok(document.querySelector<HTMLButtonElement>('[data-payment-plan-action="pro"]')?.disabled === false, "Lite 可升级到 Pro");
eq(document.querySelector('[data-payment-create]')?.getAttribute("data-payment-create"), "upgrade", "主按钮进入升级流程");
ok(document.querySelector('[data-payment-plan="pro"][data-payment-recommended="true"]') !== null, "唯一体验套餐有明确推荐标识");
const history = document.querySelector("[data-payment-history]");
const planGrid = document.querySelector("[data-payment-plans]");
ok(Boolean(history && planGrid && (history.compareDocumentPosition(planGrid) & Node.DOCUMENT_POSITION_FOLLOWING)), "历史订单入口位于套餐卡片之前");
await choosePurchase("pro", "alipay");
eq(upgradeCalls, [["plan-pro", "alipay"]], "升级复用 payment-v3 购买接口并提交 plan_id");
eq(useAccountStore.getState().payments[0]?.order_type, "subscription_purchase", "统一链路保留 v3 订单类型");

await render({
  available: true,
  account,
  subscription: {
    plans,
    current: { plan: "plus", status: "active", updated_at: "2026-07-12T00:00:00Z" },
  },
  payments: [],
}, baseAPI());
ok(document.querySelector('[data-payment-plan-action="lite"]') === null, "Plus 用户不显示 Lite");
ok(document.querySelector('[data-payment-plan-action="plus"]') === null, "Plus 用户不显示 Plus");
ok(document.querySelector<HTMLButtonElement>('[data-payment-plan-action="pro"]')?.disabled === false, "Plus 可升级到 Pro");

await render({
  available: true,
  account,
  subscription: {
    plans,
    current: { plan: "pro", status: "active", updated_at: "2026-07-12T00:00:00Z" },
  },
  payments: [],
}, baseAPI());
eq(document.querySelectorAll("[data-payment-create]").length, 0, "最高套餐没有升级目标");

await act(async () => root.unmount());
dom.window.close();
console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
