import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { LocaleProvider } from "../lib/i18n";
import { AccountRecharge } from "../features/account/ui/AccountRecharge";
import {
  cloudScopeKey,
  type CloudBootstrapView,
  type CloudMaaSRechargeOrderView,
  useAccountStore,
} from "../features/account/store/accountStore";

const account = {
  status: "authenticated" as const,
  profile_id: "profile-recharge",
  cloud_user_id: "usr-recharge",
  binding_generation: 2,
};
const scopeKey = cloudScopeKey(account);
const bootstrap: CloudBootstrapView = {
  available: true,
  capabilities: { registration: true, registration_mode: "real", payments: true, realtime: true },
  account,
  balance: { total_balance: "1000.00", available_balance: "1000.00", version: 1 },
  subscription: { plans: [], current: { plan: "free", status: "free", version: 1 } },
  payments: [],
};
const pending: CloudMaaSRechargeOrderView = {
  order_id: "maasr-test",
  amount: "100.00",
  amount_unit: "yuan",
  currency: "CNY",
  payment_method: "alipay",
  recharge_key_name: "usr-rechargecz",
  conversion_rule_version: "maas-quota-credit-v1",
  quota_per_usd: "500000.00000000",
  cny_per_usd: "7.30000000",
  quota_delta: 6849316,
  credit_delta: "200000.0272",
  credit_per_cny: "2000",
  status: "payment_pending",
  qr_code_url: "https://pay.example/maasr-test",
  expires_at: "2030-08-17T12:10:00Z",
  paid: false,
  credited: false,
  version: 2,
};

describe("AccountRecharge", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    useAccountStore.getState().reset();
    useAccountStore.getState().activateContext("desktop-account");
    useAccountStore.getState().applyBootstrap(bootstrap);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("creates an idempotent Alipay order, polls, and finishes only after credit", async () => {
    const create = vi.fn().mockResolvedValue(pending);
    const get = vi.fn().mockResolvedValue({
      ...pending,
      status: "credited",
      paid: true,
      credited: true,
      paid_at: "2026-08-17T12:01:12Z",
      credited_at: "2026-08-17T12:01:13Z",
      remote_quota_after: 7849316,
      remote_credit_after: "229200.0272",
      version: 7,
    } satisfies CloudMaaSRechargeOrderView);
    const refresh = vi.fn().mockResolvedValue({
      ...bootstrap,
      balance: { total_balance: "2000.00", available_balance: "2000.00", version: 2 },
    });
    const rechargeBalance = vi.fn().mockResolvedValue({
      key_name: "usr-rechargecz",
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
    });

    render(
      <LocaleProvider>
        <AccountRecharge
          api={{
            CloudCreateMaaSRechargeOrder: create,
            CloudGetMaaSRechargeOrder: get,
            CloudGetMaaSRechargeKeyBalance: rechargeBalance,
            CloudRefreshAssets: refresh,
          }}
          scopeKey={scopeKey}
          pollIntervalMs={250}
        />
      </LocaleProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "立即充值" }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create.mock.calls[0][0]).toBe("100.00");
    expect(create.mock.calls[0][1]).toBe("alipay");
    expect(create.mock.calls[0][2]).toMatch(/^maas-recharge-/);
    expect(document.querySelector("[data-recharge-qr]")).not.toBeNull();
    expect(screen.getByText("¥1 = 2,000 积分")).toBeTruthy();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(250);
    });
    await waitFor(() => expect(screen.getByText("充值已到账")).toBeTruthy());
    expect(get).toHaveBeenCalledWith("maasr-test");
    expect(refresh).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.getByText("229,200.0272")).toBeTruthy());
    expect(screen.queryByText("¥114.6000")).toBeNull();
    expect(screen.queryByText("usr-rechargecz")).toBeNull();
    expect(rechargeBalance).toHaveBeenCalledTimes(1);
    expect(useAccountStore.getState().balance?.version).toBe(2);
  });

  it("rejects zero and amounts with more than two decimals", () => {
    const create = vi.fn();
    render(
      <LocaleProvider>
        <AccountRecharge
          api={{
            CloudCreateMaaSRechargeOrder: create,
            CloudGetMaaSRechargeOrder: vi.fn(),
            CloudGetMaaSRechargeKeyBalance: vi.fn(),
            CloudRefreshAssets: vi.fn(),
          }}
          scopeKey={scopeKey}
        />
      </LocaleProvider>,
    );
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "1.001" } });
    expect(screen.getByText("请输入大于 0 且最多两位小数的金额。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "立即充值" })).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("highlights custom amount mode immediately", () => {
    render(
      <LocaleProvider>
        <AccountRecharge
          api={{
            CloudCreateMaaSRechargeOrder: vi.fn(),
            CloudGetMaaSRechargeOrder: vi.fn(),
            CloudGetMaaSRechargeKeyBalance: vi.fn(),
            CloudRefreshAssets: vi.fn(),
          }}
          scopeKey={scopeKey}
        />
      </LocaleProvider>,
    );

    const custom = screen.getByRole("button", { name: "自定义" });
    const preset = screen.getByRole("button", { name: "¥100" });
    expect(preset.getAttribute("aria-pressed")).toBe("true");
    fireEvent.click(custom);
    expect(custom.getAttribute("aria-pressed")).toBe("true");
    expect(preset.getAttribute("aria-pressed")).toBe("false");
  });

  it("does not show success when credited is true but paid is false", async () => {
    const malformed = { ...pending, status: "credited", credited: true, paid: false, version: 7 } satisfies CloudMaaSRechargeOrderView;
    render(
      <LocaleProvider>
        <AccountRecharge
          api={{
            CloudCreateMaaSRechargeOrder: vi.fn().mockResolvedValue(malformed),
            CloudGetMaaSRechargeOrder: vi.fn(),
            CloudGetMaaSRechargeKeyBalance: vi.fn(),
            CloudRefreshAssets: vi.fn(),
          }}
          scopeKey={scopeKey}
        />
      </LocaleProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "立即充值" }));
    await waitFor(() => expect(document.querySelector('[data-recharge-stage="result"]')).not.toBeNull());
    expect(document.querySelector('[data-recharge-stage="success"]')).toBeNull();
    expect(document.querySelector('[data-recharge-stage="result"]')).not.toBeNull();
  });

  it("shows a specific message for a payment-provider failure", async () => {
    render(
      <LocaleProvider>
        <AccountRecharge
          api={{
            CloudCreateMaaSRechargeOrder: vi.fn().mockRejectedValue(
              new Error("UPSTREAM_ERROR: payment provider is temporarily unavailable"),
            ),
            CloudGetMaaSRechargeOrder: vi.fn(),
            CloudGetMaaSRechargeKeyBalance: vi.fn(),
            CloudRefreshAssets: vi.fn(),
          }}
          scopeKey={scopeKey}
        />
      </LocaleProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "立即充值" }));
    await waitFor(() => expect(screen.getByText("支付渠道暂时不可用，请稍后重试。")).toBeTruthy());
  });
});
