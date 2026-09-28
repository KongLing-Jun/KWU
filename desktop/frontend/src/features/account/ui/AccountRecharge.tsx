import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, CheckCircle2, CircleAlert, Coins, Info, LoaderCircle, RefreshCw, ScanLine, ShieldCheck } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";

import { CopyButton } from "../../../components/CopyButton";
import { clientErrorPresentation, reportClientError } from "../../../lib/clientError";
import { useI18n, useT } from "../../../lib/i18n";
import {
  type CloudAccountError,
  type CloudMaaSRechargeKeyBalanceView,
  type CloudMaaSRechargeOrderView,
  type CloudMaaSRechargeStatus,
  useAccountStore,
} from "../store/accountStore";
import { accountBridge, type AccountRechargeBindings } from "../bridge/accountBridge";

interface AccountRechargeProps {
  api?: AccountRechargeBindings;
  balance?: CloudMaaSRechargeKeyBalanceView | null;
  onBalanceChange?: (balance: CloudMaaSRechargeKeyBalanceView) => void;
  scopeKey: string;
  pollIntervalMs?: number;
}

const amountPattern = /^(?:0|[1-9]\d*)(?:\.\d{1,2})?$/;
const terminalStatuses = new Set<CloudMaaSRechargeStatus>([
  "credited",
  "create_failed",
  "payment_failed",
  "timeout",
  "manual_required",
]);
const stoppedErrorCodes = new Set([
  "AUTH_REQUIRED",
  "TOKEN_EXPIRED",
  "REFRESH_TOKEN_INVALID",
  "REFRESH_TOKEN_EXPIRED",
  "SESSION_EXPIRED",
  "UNAUTHORIZED",
  "NOT_FOUND",
  "VALIDATION_FAILED",
  "CONFLICT",
]);
const rechargeErrorMessages: Readonly<Record<string, string>> = {
  VALIDATION_FAILED: "充值金额或支付方式不正确，请检查后重试。",
  NOT_FOUND: "充值订单不存在或已失效。",
  CONFLICT: "本次充值与已有订单冲突，请修改金额后重试。",
  IDEMPOTENCY_CONFLICT: "本次充值与已有订单冲突，请修改金额后重试。",
  UPSTREAM_ERROR: "支付渠道暂时不可用，请稍后重试。",
  INTERNAL_ERROR: "充值服务暂时不可用，请稍后重试。",
  NETWORK_UNAVAILABLE: "网络连接失败，请检查网络后重试。",
  CLOUD_PROTOCOL_ERROR: "充值订单响应异常，请稍后重试。",
  INVALID_RESPONSE: "充值订单响应异常，请稍后重试。",
};

function rechargeError(cause: unknown): CloudAccountError {
  const value = cause && typeof cause === "object"
    ? cause as { code?: unknown; message?: unknown; retryable?: unknown; request_id?: unknown; diagnostic_id?: unknown }
    : null;
  const presentation = clientErrorPresentation(cause, {
    fallbackMessage: "充值请求失败，请稍后重试。",
    knownMessages: rechargeErrorMessages,
  });
  return {
    code: typeof value?.code === "string" ? value.code : presentation.code || "CLOUD_ERROR",
    message: presentation.userMessage,
    retryable: typeof value?.retryable === "boolean" ? value.retryable : undefined,
    request_id: typeof value?.request_id === "string" ? value.request_id : undefined,
    diagnostic_id: presentation.diagnosticId,
  };
}

function amountError(amount: string): boolean {
  const trimmed = amount.trim();
  return !amountPattern.test(trimmed) || Number(trimmed) <= 0;
}

function rechargeSucceeded(order: CloudMaaSRechargeOrderView): boolean {
  return order.status === "credited" && order.paid === true && order.credited === true;
}

function makeIdempotencyKey(): string {
  const suffix = typeof crypto.randomUUID === "function"
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `maas-recharge-${suffix}`;
}

function formatMoney(value: string, locale: string): string {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return `¥${value}`;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "CNY",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(parsed);
}

function formatCredits(value: string | undefined, locale: string): string | null {
  if (!value) return null;
  const match = /^(\d+)(?:\.(\d+))?$/.exec(value.trim());
  if (!match) return null;
  const formatParts = new Intl.NumberFormat(locale).formatToParts(1000.1);
  const groupSeparator = formatParts.find((part) => part.type === "group")?.value ?? ",";
  const decimalSeparator = formatParts.find((part) => part.type === "decimal")?.value ?? ".";
  const integer = match[1].replace(/\B(?=(\d{3})+(?!\d))/g, groupSeparator);
  const fraction = (match[2] ?? "").replace(/0+$/, "");
  return fraction ? `${integer}${decimalSeparator}${fraction}` : integer;
}

function formatConversionRate(creditPerCNY: string | undefined, locale: string): string | null {
  const credits = formatCredits(creditPerCNY, locale);
  if (!credits || /^0(?:[.,]0+)?$/.test(credits)) return null;
  return `¥1 = ${credits} 积分`;
}

function formatCountdown(expiresAt: string, now: number): string {
  const milliseconds = Math.max(0, Date.parse(expiresAt) - now);
  const totalSeconds = Math.ceil(milliseconds / 1_000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

export function AccountRecharge({
  api = accountBridge,
  balance: balanceFromParent = null,
  onBalanceChange,
  scopeKey,
  pollIntervalMs = 1_000,
}: AccountRechargeProps) {
  const { locale, t } = useI18n();
  const [amount, setAmount] = useState("100.00");
  const [amountSelection, setAmountSelection] = useState<"preset" | "custom">("preset");
  const [order, setOrder] = useState<CloudMaaSRechargeOrderView | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<CloudAccountError | null>(null);
  const [pollRetrying, setPollRetrying] = useState(false);
  const [pollStopped, setPollStopped] = useState(false);
  const [rechargeBalance, setRechargeBalance] = useState<CloudMaaSRechargeKeyBalanceView | null>(balanceFromParent);
  const [rechargeBalanceLoading, setRechargeBalanceLoading] = useState(false);
  const [rechargeBalanceUnavailable, setRechargeBalanceUnavailable] = useState(false);
  const [clock, setClock] = useState(() => Date.now());
  const lifecycle = useRef(0);
  const idempotency = useRef<{ amount: string; key: string } | null>(null);
  const creditedRefresh = useRef("");
  const creditedBalance = useRef("");
  const amountInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    lifecycle.current += 1;
    setAmount("100.00");
    setAmountSelection("preset");
    setOrder(null);
    setCreating(false);
    setError(null);
    setPollRetrying(false);
    setPollStopped(false);
    setRechargeBalance(balanceFromParent);
    setRechargeBalanceLoading(false);
    setRechargeBalanceUnavailable(false);
    idempotency.current = null;
    creditedRefresh.current = "";
    creditedBalance.current = "";
    return () => {
      lifecycle.current += 1;
    };
  }, [scopeKey]);

  useEffect(() => {
    if (balanceFromParent) setRechargeBalance(balanceFromParent);
  }, [balanceFromParent]);

  useEffect(() => {
    if (!order || terminalStatuses.has(order.status)) return;
    setClock(Date.now());
    const timer = setInterval(() => setClock(Date.now()), 1_000);
    return () => clearInterval(timer);
  }, [order?.order_id, order?.status]);

  const refreshAssetsAfterCredit = useCallback(async (creditedOrder: CloudMaaSRechargeOrderView) => {
    if (creditedRefresh.current === creditedOrder.order_id) return;
    creditedRefresh.current = creditedOrder.order_id;
    try {
      const view = await api.CloudRefreshAssets();
      const state = useAccountStore.getState();
      if (state.scopeKey === scopeKey) state.applyBootstrap(view);
    } catch {
      // The recharge result is authoritative even if the optional balance refresh fails.
    }
  }, [api, scopeKey]);

  const refreshRechargeBalanceAfterCredit = useCallback(async (creditedOrder: CloudMaaSRechargeOrderView) => {
    if (creditedBalance.current === creditedOrder.order_id) return;
    creditedBalance.current = creditedOrder.order_id;
    const expectedScope = scopeKey;
    const epoch = lifecycle.current;
    setRechargeBalance(null);
    setRechargeBalanceUnavailable(false);
    setRechargeBalanceLoading(true);
    try {
      const balance = await api.CloudGetMaaSRechargeKeyBalance();
      if (lifecycle.current !== epoch || useAccountStore.getState().scopeKey !== expectedScope) return;
      setRechargeBalance(balance);
      onBalanceChange?.(balance);
    } catch (cause: unknown) {
      if (lifecycle.current === epoch && useAccountStore.getState().scopeKey === expectedScope) {
        reportClientError("account.maasRechargeBalance", cause, { fallbackMessage: t("account.recharge.balanceUnavailable") });
        setRechargeBalanceUnavailable(true);
      }
    } finally {
      if (lifecycle.current === epoch && useAccountStore.getState().scopeKey === expectedScope) {
        setRechargeBalanceLoading(false);
      }
    }
  }, [api, onBalanceChange, scopeKey, t]);

  const acceptOrder = useCallback((incoming: CloudMaaSRechargeOrderView) => {
    setOrder((current) => {
      if (current && incoming.order_id === current.order_id && incoming.version < current.version) return current;
      return incoming;
    });
    if (rechargeSucceeded(incoming)) {
      void refreshAssetsAfterCredit(incoming);
      void refreshRechargeBalanceAfterCredit(incoming);
    }
  }, [refreshAssetsAfterCredit, refreshRechargeBalanceAfterCredit]);

  useEffect(() => {
    if (!order || terminalStatuses.has(order.status)) return;
    const expectedScope = scopeKey;
    const expectedOrderID = order.order_id;
    const epoch = lifecycle.current;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let failures = 0;

    const schedule = (delay: number) => {
      timer = setTimeout(() => void poll(), delay);
    };
    const poll = async () => {
      if (cancelled || lifecycle.current !== epoch || useAccountStore.getState().scopeKey !== expectedScope) return;
      try {
        const incoming = await api.CloudGetMaaSRechargeOrder(expectedOrderID);
        if (cancelled || lifecycle.current !== epoch) return;
        failures = 0;
        setPollRetrying(false);
        setPollStopped(false);
        setError(null);
        acceptOrder(incoming);
        if (!terminalStatuses.has(incoming.status)) schedule(Math.max(250, pollIntervalMs));
      } catch (cause) {
        if (cancelled || lifecycle.current !== epoch) return;
        const parsed = rechargeError(cause);
        setError(parsed);
        if (parsed.retryable === false || stoppedErrorCodes.has(parsed.code.toUpperCase())) {
          setPollStopped(true);
          return;
        }
        failures += 1;
        setPollRetrying(true);
        schedule(Math.min(30_000, Math.max(1_000, pollIntervalMs) * (2 ** Math.min(failures, 4))));
      }
    };

    schedule(Math.max(250, pollIntervalMs));
    return () => {
      cancelled = true;
      if (timer !== null) clearTimeout(timer);
    };
  }, [acceptOrder, api, order?.order_id, order?.status, pollIntervalMs, scopeKey]);

  const validationError = amountError(amount);
  const createOrder = async () => {
    const normalizedAmount = amount.trim();
    if (creating || amountError(normalizedAmount)) return;
    const expectedScope = scopeKey;
    const epoch = lifecycle.current;
    if (idempotency.current?.amount !== normalizedAmount) {
      idempotency.current = { amount: normalizedAmount, key: makeIdempotencyKey() };
    }
    setCreating(true);
    setError(null);
    setPollRetrying(false);
    setPollStopped(false);
    try {
      const created = await api.CloudCreateMaaSRechargeOrder(
        normalizedAmount,
        "alipay",
        idempotency.current.key,
      );
      if (lifecycle.current !== epoch || useAccountStore.getState().scopeKey !== expectedScope) return;
      acceptOrder(created);
    } catch (cause) {
      if (lifecycle.current === epoch) setError(rechargeError(cause));
    } finally {
      if (lifecycle.current === epoch) setCreating(false);
    }
  };

  const startAnother = () => {
    setOrder(null);
    setError(null);
    setPollRetrying(false);
    setPollStopped(false);
    setRechargeBalance(null);
    setRechargeBalanceLoading(false);
    setRechargeBalanceUnavailable(false);
    idempotency.current = null;
    creditedRefresh.current = "";
    creditedBalance.current = "";
  };

  const statusLabel = useMemo(() => {
    if (!order) return "";
    return t(`account.recharge.status.${order.status}`);
  }, [order?.status, t]);
  const successful = order ? rechargeSucceeded(order) : false;
  const failed = order ? terminalStatuses.has(order.status) && !successful : false;
  const balanceConversionRate = formatConversionRate(rechargeBalance?.credit_per_cny, locale);
  const orderConversionRate = order
    ? formatConversionRate(order.credit_per_cny, locale)
    : null;
  const normalizedAmount = amount.trim();
  const selectedPreset = amountSelection === "preset" && ["50.00", "100.00", "200.00", "500.00"].includes(normalizedAmount)
    ? normalizedAmount
    : "custom";

  if (!order) {
    return (
      <div className="account-recharge" data-recharge-stage="amount">
        <div className="account-recharge__entry-grid">
          <section className="account-recharge__amount-card">
            <div className="account-recharge__amount-heading">
              <span aria-hidden="true"><Coins size={20} /></span>
              <div>
                <h3>{t("account.recharge.amountTitle")}</h3>
                <p>{t("account.recharge.amountHint")}</p>
              </div>
            </div>
            <div className="account-recharge__rate" data-recharge-rate>
              <span aria-hidden="true"><Coins size={17} /></span>
              <div><small>{t("account.recharge.rateLabel")}</small><strong>{balanceConversionRate ?? t("account.recharge.rateUnavailable")}</strong></div>
              <small>{t("account.recharge.rateHint")}</small>
            </div>
            <div className="account-recharge__amount-field">
              <span>{t("account.recharge.quickAmounts")}</span>
              <div className="account-recharge__presets" aria-label={t("account.recharge.quickAmounts")}>
                {[50, 100, 200, 500].map((preset) => {
                  const value = `${preset}.00`;
                  const selected = selectedPreset === value;
                  return (
                    <button
                      aria-pressed={selected}
                      className={selected ? "is-selected" : ""}
                      key={preset}
                      type="button"
                      onClick={() => {
                        setAmount(value);
                        setAmountSelection("preset");
                      }}
                    >
                      ¥{preset}{selected ? <Check size={13} aria-hidden="true" /> : null}
                    </button>
                  );
                })}
                <button
                  aria-pressed={selectedPreset === "custom"}
                  className={selectedPreset === "custom" ? "is-selected" : ""}
                  type="button"
                  onClick={() => {
                    setAmountSelection("custom");
                    amountInputRef.current?.focus();
                  }}
                >
                  {t("account.recharge.customAmount")}
                </button>
              </div>
            </div>
            <label className="account-recharge__amount-field" htmlFor="account-recharge-amount">
              <span>{t("account.recharge.amountLabel")}</span>
              <span className="account-recharge__amount-input">
                <b aria-hidden="true">¥</b>
                <input
                  ref={amountInputRef}
                  id="account-recharge-amount"
                  autoFocus
                  inputMode="decimal"
                  value={amount}
                  aria-invalid={validationError}
                  onChange={(event) => {
                    setAmount(event.currentTarget.value);
                    setAmountSelection("custom");
                  }}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") void createOrder();
                  }}
                  data-recharge-amount
                />
                <small>{t("account.recharge.currency")}</small>
              </span>
            </label>
            <p
              className={validationError && amount.trim() !== ""
                ? "account-recharge__amount-help account-recharge__amount-help--error"
                : "account-recharge__amount-help"}
              role={validationError && amount.trim() !== "" ? "alert" : undefined}
            >
              <Info size={13} aria-hidden="true" />{t("account.recharge.amountError")}
            </p>
            <div className="account-recharge__payment-method">
              <span>{t("account.recharge.paymentMethod")}</span>
              <div><b aria-hidden="true">支</b><span><strong>{t("account.payment.channel.alipay")}</strong><small>{t("account.recharge.alipayHint")}</small></span><CheckCircle2 size={18} aria-hidden="true" /></div>
            </div>
            {error ? <RechargeError error={error} /> : null}
          </section>
          <aside className="account-recharge__summary-card">
            <h3>{t("account.recharge.orderSummary")}</h3>
            <dl>
              <div><dt>{t("account.recharge.orderAmount")}</dt><dd>{validationError ? "—" : formatMoney(normalizedAmount, locale)}</dd></div>
              <div><dt>{t("account.recharge.rateLabel")}</dt><dd>{balanceConversionRate ?? "—"}</dd></div>
              <div><dt>{t("account.recharge.paymentMethod")}</dt><dd>{t("account.payment.channel.alipay")}</dd></div>
              <div><dt>{t("account.recharge.creditDestination")}</dt><dd>{t("account.billing.rechargeCredits")}</dd></div>
            </dl>
            <div className="account-recharge__payable"><span>{t("account.recharge.payable")}</span><strong>{validationError ? "—" : formatMoney(normalizedAmount, locale)}</strong></div>
            <button
              className="account-recharge__primary"
              type="button"
              disabled={creating || validationError}
              onClick={() => void createOrder()}
              data-recharge-action="create"
            >
              {creating ? <LoaderCircle className="account-recharge__spin" size={16} aria-hidden="true" /> : <ScanLine size={16} aria-hidden="true" />}
              {t(creating ? "account.recharge.creating" : "account.recharge.create")}
            </button>
            <p className="account-recharge__secure-hint"><ShieldCheck size={13} aria-hidden="true" />{t("account.recharge.secureHint")}</p>
          </aside>
        </div>
      </div>
    );
  }

  return (
    <div className="account-recharge" data-recharge-stage={successful ? "success" : failed ? "result" : "payment"} data-recharge-order={order.order_id}>
      {successful ? (
        <section className="account-recharge__result account-recharge__result--success" role="status">
          <span className="account-recharge__result-icon" aria-hidden="true"><CheckCircle2 size={34} /></span>
          <h3>{t("account.recharge.successTitle")}</h3>
          <p>{t("account.recharge.successHint")}</p>
          {formatCredits(order.credit_delta, locale) ? (
            <strong>{t("account.recharge.quotaAdded", { quota: formatCredits(order.credit_delta, locale) ?? "" })}</strong>
          ) : (
            <small>{t("account.recharge.balanceUnavailable")}</small>
          )}
          {formatCredits(order.remote_credit_after, locale) ? (
            <small>{t("account.recharge.quotaAfter", { quota: formatCredits(order.remote_credit_after, locale) ?? "" })}</small>
          ) : null}
          <div className="account-recharge__available-balance" data-recharge-balance>
            <span>{t("account.recharge.availableBalance")}</span>
            {rechargeBalanceLoading ? (
              <small>{t("account.recharge.balanceLoading")}</small>
            ) : formatCredits(rechargeBalance?.available_credits, locale) ? (
              <>
                <strong>{formatCredits(rechargeBalance?.available_credits, locale)}</strong>
                <small>{t("account.recharge.creditsUnit")}</small>
              </>
            ) : rechargeBalanceUnavailable ? (
              <small>{t("account.recharge.balanceUnavailable")}</small>
            ) : null}
          </div>
          <button className="account-recharge__secondary" type="button" onClick={startAnother}>
            {t("account.recharge.again")}
          </button>
        </section>
      ) : failed ? (
        <section className="account-recharge__result account-recharge__result--failed" role="status">
          <span className="account-recharge__result-icon" aria-hidden="true"><CircleAlert size={32} /></span>
          <h3>{statusLabel}</h3>
          <p>{t(order.status === "manual_required" ? "account.recharge.manualHint" : "account.recharge.failedHint")}</p>
          <button className="account-recharge__secondary" type="button" onClick={startAnother} disabled={order.status === "manual_required"}>
            {t("account.recharge.retry")}
          </button>
        </section>
      ) : (
        <div className="account-recharge__payment">
          <section className="account-recharge__qr-card">
            <div className="account-recharge__qr">
              {order.qr_code_url ? (
                <QRCodeSVG value={order.qr_code_url} size={188} level="M" marginSize={2} data-recharge-qr />
              ) : (
                <LoaderCircle className="account-recharge__spin" size={32} aria-label={t("common.loading")} />
              )}
            </div>
            <strong>{t("account.recharge.scanTitle")}</strong>
            <p>{t("account.recharge.scanHint")}</p>
            <small>{t("account.recharge.expiresIn", { time: formatCountdown(order.expires_at, clock) })}</small>
          </section>
          <section className="account-recharge__order-card">
            <span className="account-recharge__status" data-recharge-status={order.status}>
              <i aria-hidden="true" />
              {statusLabel}
            </span>
            <dl>
              <div><dt>{t("account.recharge.orderAmount")}</dt><dd>{formatMoney(order.amount, locale)}</dd></div>
              <div><dt>{t("account.recharge.rateLabel")}</dt><dd>{orderConversionRate ?? "—"}</dd></div>
              <div><dt>{t("account.recharge.paymentMethod")}</dt><dd>{t("account.payment.channel.alipay")}</dd></div>
              <div><dt>{t("account.recharge.orderID")}</dt><dd title={order.order_id}>{order.order_id}</dd></div>
            </dl>
            <p className="account-recharge__auto-hint">
              <RefreshCw className={!pollRetrying && !pollStopped ? "account-recharge__spin" : ""} size={14} aria-hidden="true" />
              {t(pollStopped
                ? "account.recharge.pollStopped"
                : pollRetrying
                  ? "account.recharge.pollRetrying"
                  : "account.recharge.autoPolling")}
            </p>
            {error ? <RechargeError error={error} /> : null}
          </section>
        </div>
      )}
    </div>
  );
}

function RechargeError({ error }: { error: CloudAccountError }) {
  const t = useT();
  return (
    <div className="account-recharge__error" role="alert">
      <CircleAlert size={15} aria-hidden="true" />
      <span>{error.message}</span>
      {error.request_id ? <small translate="no">Request ID: {error.request_id}</small> : null}
      {error.diagnostic_id ? (
        <small className="account-recharge__error-log-id" data-account-error-diagnostic-id translate="no">
          <span>{t("account.logID")}</span>
          <code>{error.diagnostic_id}</code>
          <CopyButton text={error.diagnostic_id} showInlineLabel={false} label={t("common.copy")} />
        </small>
      ) : null}
    </div>
  );
}
