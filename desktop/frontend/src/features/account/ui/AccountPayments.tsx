import { useCallback, useEffect, useRef, useState } from "react";
import { Bot, CalendarDays, Coins, Cpu, Folder, History, ShieldCheck } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";

import { reportClientError } from "../../../lib/clientError";
import { type DictKey, useI18n } from "../../../lib/i18n";
import {
  type CloudAccountError,
  type CloudPaymentState,
  type CloudPaymentView,
  type CloudSubscriptionPlan,
  useAccountStore,
} from "../store/accountStore";
import { accountBridge, type AccountPaymentBindings } from "../bridge/accountBridge";

export type { AccountPaymentBindings } from "../bridge/accountBridge";

interface AccountPaymentsProps {
  api?: AccountPaymentBindings;
  paymentsEnabled?: boolean;
  scopeKey: string;
  pollIntervalMs?: number;
}

interface PurchaseIntent {
  action: "purchase" | "upgrade";
  plan: CloudSubscriptionPlan;
  channel: CloudPaymentView["payment_channel"];
}

type PollNotice = "auth" | "retrying" | "stopped";
type PaymentStage = "select" | "scan" | "confirm" | "success" | "result";

interface PlanPresentation {
  displayName: string;
  modelTier: DictKey;
  modelExamples: DictKey;
  automationTasksUnlimited: boolean;
  projects: number;
  projectsUnlimited: boolean;
}

const experiencePlanCode = "pro";

// Only Alipay has completed real-environment acceptance. Keep WeChat visible
// beside it so enabling the channel later does not require another layout pass.
const paymentChannelOptions: Array<{
  channel: CloudPaymentView["payment_channel"];
  enabled: boolean;
}> = [
  { channel: "alipay", enabled: true },
  { channel: "wechat", enabled: false },
];
const planPresentations: Record<string, PlanPresentation> = {
  pro: {
    displayName: "体验套餐",
    modelTier: "account.payment.modelTier.pro",
    modelExamples: "account.payment.modelExamples.pro",
    automationTasksUnlimited: true,
    projects: 1_000,
    projectsUnlimited: true,
  },
};
const pollNoticeKeys: Record<PollNotice, DictKey> = {
  auth: "account.payment.poll.auth",
  retrying: "account.payment.poll.retrying",
  stopped: "account.payment.poll.stopped",
};
const paymentChannelKeys: Record<CloudPaymentView["payment_channel"], DictKey> = {
  alipay: "account.payment.channel.alipay",
  wechat: "account.payment.channel.wechat",
};
const paymentStateKeys: Record<CloudPaymentState, DictKey> = {
  order_created: "account.payment.status.orderCreated",
  paying: "account.payment.status.paying",
  payment_confirming: "account.payment.status.confirming",
  payment_succeeded: "account.payment.status.succeeded",
  payment_closed: "account.payment.status.closed",
  payment_failed: "account.payment.status.failed",
  payment_timeout: "account.payment.status.timeout",
  payment_manual_required: "account.payment.status.manualRequired",
};
const authErrorCodes = new Set([
  "AUTH_REQUIRED",
  "TOKEN_EXPIRED",
  "REFRESH_TOKEN_INVALID",
  "REFRESH_TOKEN_EXPIRED",
  "REFRESH_TOKEN_REUSED",
  "SESSION_EXPIRED",
  "UNAUTHORIZED",
  "DEVICE_MISMATCH",
  "ACCOUNT_DISABLED",
]);
const stoppedPollErrorCodes = new Set([
  "INVALID_RESPONSE",
  "CLOUD_PROTOCOL_ERROR",
  "PROFILE_SCOPE_MISMATCH",
  "NOT_FOUND",
  "VALIDATION_FAILED",
  "IDEMPOTENCY_CONFLICT",
  "CONFLICT",
  "PAYMENT_ORDER_NOT_FOUND",
  "PAYMENT_ORDER_SCOPE_MISMATCH",
  "PAYMENT_ORDER_IDENTITY_MISMATCH",
  "PAYMENT_VERSION_REGRESSION",
]);
const retryablePollErrorCodes = new Set([
  "NETWORK_UNAVAILABLE",
  "UPSTREAM_ERROR",
  "INTERNAL_ERROR",
  "CONCURRENT_OPERATION_RETRYABLE",
]);

function paymentError(cause: unknown): CloudAccountError {
  const presentation = reportClientError("account.payment", cause, {
    fallbackMessage: "支付请求失败，请稍后重试。",
  });
  if (cause && typeof cause === "object") {
    const value = cause as { code?: unknown; message?: unknown; retryable?: unknown; request_id?: unknown; diagnostic_id?: unknown };
    if (typeof value.code === "string" && value.code && typeof value.message === "string" && value.message) {
      return {
        code: value.code,
        message: presentation.userMessage,
        retryable: typeof value.retryable === "boolean" ? value.retryable : undefined,
        request_id: typeof value.request_id === "string" ? value.request_id : undefined,
        diagnostic_id: presentation.diagnosticId,
      };
    }
  }
  const rawMessage = cause instanceof Error ? cause.message : String(cause || "");
  const match = /^([A-Z][A-Z0-9_]+):\s*(.+)$/.exec(rawMessage.trim());
  return match
    ? { code: match[1], message: presentation.userMessage, diagnostic_id: presentation.diagnosticId }
    : { code: presentation.code || "CLOUD_ERROR", message: presentation.userMessage, diagnostic_id: presentation.diagnosticId };
}

function projectedPaymentState(payment: CloudPaymentView): CloudPaymentState {
  if (payment.state === "payment_succeeded" && !payment.entitlement_granted) return "payment_confirming";
  return payment.state;
}

function paymentEntry(
  payment: CloudPaymentView,
  now: number,
): { field: "qr_code" | "code_url" | "qr_code_url"; value: string; expiresAt: number } | null {
  const entry = payment.payment_entry;
  if (!payment.payable || !entry) return null;
  const entryExpiry = Date.parse(entry.expires_at);
  const orderExpiry = Date.parse(payment.expires_at);
  if (!Number.isFinite(entryExpiry) || !Number.isFinite(orderExpiry)) return null;
  const expiresAt = Math.min(entryExpiry, orderExpiry);
  if (expiresAt <= now) return null;
  if (
    typeof entry.qr_code_url === "string"
    && entry.qr_code_url.trim() !== ""
    && !entry.qr_code
    && !entry.code_url
  ) {
    return { field: "qr_code_url", value: entry.qr_code_url, expiresAt };
  }
  if (
    payment.payment_channel === "alipay"
    && entry.entry_type === "qr_code"
    && typeof entry.qr_code === "string"
    && entry.qr_code.trim() !== ""
    && !entry.code_url
  ) {
    return { field: "qr_code", value: entry.qr_code, expiresAt };
  }
  if (
    payment.payment_channel === "wechat"
    && entry.entry_type === "native_qr"
    && typeof entry.code_url === "string"
    && entry.code_url.trim() !== ""
    && !entry.qr_code
  ) {
    return { field: "code_url", value: entry.code_url, expiresAt };
  }
  return null;
}

function paymentTerminal(payment: CloudPaymentView): boolean {
  if (payment.state === "payment_succeeded") return payment.entitlement_granted;
  return payment.state === "payment_closed"
    || payment.state === "payment_failed"
    || payment.state === "payment_timeout"
    || payment.state === "payment_manual_required";
}

function paymentCanReorder(payment: CloudPaymentView): boolean {
  return payment.state === "payment_closed"
    || payment.state === "payment_failed"
    || payment.state === "payment_timeout";
}

function paymentStage(payment: CloudPaymentView | null): PaymentStage {
  if (!payment) return "select";
  if (payment.state === "payment_succeeded" && payment.entitlement_granted) return "success";
  if (payment.state === "payment_confirming" || payment.state === "payment_succeeded") return "confirm";
  if (paymentTerminal(payment)) return "result";
  return "scan";
}

function paymentStep(payment: CloudPaymentView | null): 1 | 2 | 3 {
  if (!payment) return 1;
  if (
    payment.state === "payment_confirming"
    || payment.state === "payment_succeeded"
    || payment.state === "payment_manual_required"
  ) {
    return 3;
  }
  return 2;
}

function formatDecimal(value: number, locale: string, maximumFractionDigits = 4): string {
  return new Intl.NumberFormat(locale, {
    minimumFractionDigits: 0,
    maximumFractionDigits,
  }).format(value);
}

function formatDecimalString(value: string, locale: string): string {
  const match = /^(\d+)(?:\.(\d+))?$/.exec(value.trim());
  if (!match) return value;
  const formatParts = new Intl.NumberFormat(locale).formatToParts(1000.1);
  const groupSeparator = formatParts.find((part) => part.type === "group")?.value ?? ",";
  const decimalSeparator = formatParts.find((part) => part.type === "decimal")?.value ?? ".";
  const integer = match[1].replace(/\B(?=(\d{3})+(?!\d))/g, groupSeparator);
  const fraction = (match[2] ?? "").replace(/0+$/, "");
  return fraction ? `${integer}${decimalSeparator}${fraction}` : integer;
}

function formatMoney(
  amount: string,
  amountUnit: CloudPaymentView["amount_unit"],
  currency: string,
  locale: string,
): string {
  let value = Number(amount);
  if (!Number.isFinite(value)) return `${amount} ${currency}`.trim();
  if (amountUnit === "fen") value /= 100;
  const decimal = new Intl.NumberFormat(locale, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
  if (currency.toUpperCase() === "CNY") return `¥${decimal}`;
  return `${currency.toUpperCase()} ${decimal}`;
}

function formatCountdown(milliseconds: number): string {
  const totalSeconds = Math.max(0, Math.ceil(milliseconds / 1_000));
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  const seconds = totalSeconds % 60;
  const paddedMinutes = String(minutes).padStart(2, "0");
  const paddedSeconds = String(seconds).padStart(2, "0");
  return hours > 0 ? `${hours}:${paddedMinutes}:${paddedSeconds}` : `${paddedMinutes}:${paddedSeconds}`;
}

function purchaseKey(intent: PurchaseIntent): string {
  return `${intent.action}\u0000${intent.plan.plan_id ?? ""}\u0000${intent.channel}`;
}

function formatSubscriptionDate(value: string | undefined, locale: string): string | null {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

function isAuthenticationError(error: CloudAccountError): boolean {
  return authErrorCodes.has(error.code.toUpperCase());
}

function shouldStopPolling(error: CloudAccountError): boolean {
  const code = error.code.toUpperCase();
  if (retryablePollErrorCodes.has(code) || error.retryable === true) return false;
  return stoppedPollErrorCodes.has(code) || code.startsWith("CONTRACT_") || error.retryable === false;
}

export function AccountPayments({
  api = accountBridge,
  paymentsEnabled = true,
  scopeKey,
  pollIntervalMs = 3_000,
}: AccountPaymentsProps) {
  const { locale, t } = useI18n();
  const subscription = useAccountStore((state) => state.subscription);
  const payments = useAccountStore((state) => state.payments);
  const applyPaymentOperation = useAccountStore((state) => state.applyPaymentOperation);
  const setError = useAccountStore((state) => state.setError);
  const [creatingAction, setCreatingAction] = useState("");
  const [selectedOrderID, setSelectedOrderID] = useState<string | null>(null);
  const [selectedChannel, setSelectedChannel] = useState<CloudPaymentView["payment_channel"]>("alipay");
  const [pendingPurchase, setPendingPurchase] = useState<PurchaseIntent | null>(null);
  const [expiryClock, setExpiryClock] = useState(() => Date.now());
  const [pollNotice, setPollNotice] = useState<PollNotice | null>(null);
  const lifecycleEpoch = useRef(0);
  const selectionScope = useRef("");
  const pollTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollInFlight = useRef(new Set<string>());
  const pollNextAt = useRef(new Map<string, number>());
  const pollFailures = useRef(new Map<string, number>());
  const pollStopped = useRef(new Set<string>());
  const pollAuthBlocked = useRef(false);

  const clearPollTimer = useCallback(() => {
    if (pollTimer.current !== null) {
      clearInterval(pollTimer.current);
      pollTimer.current = null;
    }
  }, []);

  const resetPollState = useCallback(() => {
    clearPollTimer();
    pollInFlight.current.clear();
    pollNextAt.current.clear();
    pollFailures.current.clear();
    pollStopped.current.clear();
    pollAuthBlocked.current = false;
  }, [clearPollTimer]);

  const isCurrent = useCallback((epoch: number, expectedScope: string) => (
    lifecycleEpoch.current === epoch
    && expectedScope === scopeKey
    && useAccountStore.getState().scopeKey === expectedScope
  ), [scopeKey]);

  useEffect(() => {
    lifecycleEpoch.current += 1;
    resetPollState();
    selectionScope.current = "";
    setCreatingAction("");
    setSelectedOrderID(null);
    setSelectedChannel("alipay");
    setPendingPurchase(null);
    setPollNotice(null);
    return () => {
      lifecycleEpoch.current += 1;
      resetPollState();
    };
  }, [resetPollState, scopeKey]);

  useEffect(() => {
    if (selectionScope.current !== scopeKey) {
      selectionScope.current = scopeKey;
      const recovered = [...payments].reverse().find((payment) => !paymentTerminal(payment));
      setSelectedOrderID(recovered?.order_id ?? null);
      return;
    }
    if (selectedOrderID && !payments.some((payment) => payment.order_id === selectedOrderID)) {
      setSelectedOrderID(null);
    }
  }, [payments, scopeKey, selectedOrderID]);

  useEffect(() => {
    const hasLiveEntry = payments.some((payment) => payment.payable && payment.payment_entry);
    if (!hasLiveEntry) return;
    setExpiryClock(Date.now());
    const timer = setInterval(() => setExpiryClock(Date.now()), 1_000);
    return () => clearInterval(timer);
  }, [payments, scopeKey]);

  useEffect(() => {
    clearPollTimer();
    if (!paymentsEnabled) return;
    const interval = Math.max(1, pollIntervalMs);
    const expectedScope = scopeKey;
    const epoch = lifecycleEpoch.current;
    const initialPollAt = Date.now() + interval;
    for (const payment of useAccountStore.getState().payments) {
      if (!paymentTerminal(payment) && !pollNextAt.current.has(payment.order_id)) {
        pollNextAt.current.set(payment.order_id, initialPollAt);
      }
    }

    const pollAll = () => {
      if (!isCurrent(epoch, expectedScope) || pollAuthBlocked.current) return;
      const now = Date.now();
      const currentPayments = useAccountStore.getState().payments;
      const activeOrderIDs = new Set(
        currentPayments.filter((payment) => !paymentTerminal(payment)).map((payment) => payment.order_id),
      );
      for (const orderID of pollNextAt.current.keys()) {
        if (!activeOrderIDs.has(orderID)) {
          pollNextAt.current.delete(orderID);
          pollFailures.current.delete(orderID);
          pollStopped.current.delete(orderID);
        }
      }
      for (const payment of currentPayments) {
        const orderID = payment.order_id;
        if (!paymentTerminal(payment) && !pollNextAt.current.has(orderID)) {
          pollNextAt.current.set(orderID, now + interval);
          continue;
        }
        if (
          paymentTerminal(payment)
          || pollStopped.current.has(orderID)
          || pollInFlight.current.has(orderID)
          || (pollNextAt.current.get(orderID) ?? 0) > now
        ) {
          continue;
        }
        pollInFlight.current.add(orderID);
        pollNextAt.current.set(orderID, now + interval);
        void api.CloudRefreshPayment(orderID)
          .then((view) => {
            if (!isCurrent(epoch, expectedScope)) return;
            applyPaymentOperation(view);
            pollFailures.current.delete(orderID);
            pollNextAt.current.set(orderID, Date.now() + interval);
            if (pollFailures.current.size === 0 && pollStopped.current.size === 0) {
              setPollNotice((current) => (current === "retrying" ? null : current));
            }
          })
          .catch((cause) => {
            if (!isCurrent(epoch, expectedScope)) return;
            const error = paymentError(cause);
            if (isAuthenticationError(error)) {
              pollAuthBlocked.current = true;
              setPollNotice("auth");
              return;
            }
            if (shouldStopPolling(error)) {
              pollStopped.current.add(orderID);
              setPollNotice("stopped");
              return;
            }
            const failures = (pollFailures.current.get(orderID) ?? 0) + 1;
            pollFailures.current.set(orderID, failures);
            const retryDelay = Math.min(30_000, interval * (2 ** Math.min(failures, 4)));
            pollNextAt.current.set(orderID, Date.now() + retryDelay);
            setPollNotice("retrying");
          })
          .finally(() => {
            pollInFlight.current.delete(orderID);
          });
      }
    };

    pollTimer.current = setInterval(pollAll, Math.min(interval, 1_000));
    return clearPollTimer;
  }, [api, applyPaymentOperation, clearPollTimer, isCurrent, paymentsEnabled, pollIntervalMs, scopeKey]);

  const plans = [...(subscription?.plans ?? [])].sort((left, right) => (
    left.sort_order - right.sort_order || left.plan.localeCompare(right.plan)
  ));
  const visiblePlans = plans.filter((plan) => plan.plan === experiencePlanCode);
  const enabledPlans = visiblePlans.filter((plan) => plan.enabled);
  const recommendedPlan = enabledPlans[Math.floor(enabledPlans.length / 2)]?.plan ?? "";
  const catalogPeriodDays = visiblePlans.length > 0 && visiblePlans.every((plan) => plan.period_days === visiblePlans[0].period_days)
    ? visiblePlans[0].period_days
    : null;
  const currentPlan = plans.find((plan) => plan.plan === subscription?.current.plan) ?? null;
  const subscriptionPeriodEnd = formatSubscriptionDate(subscription?.current.period_end, locale);
  const actionForPlan = (plan: CloudSubscriptionPlan): PurchaseIntent["action"] | null => {
    if (!plan.enabled || !plan.plan_id) return null;
    if (subscription?.current.status === "free" || subscription?.current.status === "expired") return "purchase";
    if (subscription?.current.status !== "active" || !currentPlan) return null;
    return plan.sort_order > currentPlan.sort_order ? "upgrade" : null;
  };
  const activePayments = [...payments].filter((payment) => !paymentTerminal(payment)).reverse();
  const terminalPayments = [...payments].filter(paymentTerminal).reverse();
  const confirmingPayment = activePayments.find((payment) => (
    projectedPaymentState(payment) === "payment_confirming"
  )) ?? null;
  const selectedPayment = selectedOrderID
    ? payments.find((payment) => payment.order_id === selectedOrderID) ?? null
    : null;
  const stage = paymentStage(selectedPayment);
  const currentStep = paymentStep(selectedPayment);
  const completed = stage === "success";

  useEffect(() => {
    if (confirmingPayment) setPendingPurchase(null);
  }, [confirmingPayment?.order_id]);

  const planName = useCallback((planCode: string) => (
    planPresentations[planCode]?.displayName
      ?? plans.find((plan) => plan.plan === planCode)?.display_name
      ?? planCode
  ), [plans]);

  const performCreate = async (intent: PurchaseIntent) => {
    const planID = intent.plan.plan_id;
    if (!paymentsEnabled || creatingAction !== "" || !planID) return;
    const blockingPayment = useAccountStore.getState().payments.find((payment) => (
      !paymentTerminal(payment) && projectedPaymentState(payment) === "payment_confirming"
    ));
    if (blockingPayment) {
      setPendingPurchase(null);
      setSelectedOrderID(blockingPayment.order_id);
      return;
    }
    const expectedScope = scopeKey;
    const epoch = lifecycleEpoch.current;
    const key = purchaseKey(intent);
    setCreatingAction(key);
    setPendingPurchase(null);
    setError(null);
    try {
      const view = await api.CloudCreateSubscriptionOrder(planID, intent.channel);
      if (!isCurrent(epoch, expectedScope)) return;
      applyPaymentOperation(view);
      pollNextAt.current.set(view.payment.order_id, Date.now() + Math.max(1, pollIntervalMs));
      setSelectedOrderID(view.payment.order_id);
    } catch (cause) {
      if (isCurrent(epoch, expectedScope)) setError(paymentError(cause));
    } finally {
      if (isCurrent(epoch, expectedScope)) setCreatingAction("");
    }
  };

  const requestCreate = (intent: PurchaseIntent) => {
    if (!paymentsEnabled || creatingAction !== "") return;
    const activeOrders = payments.filter((payment) => !paymentTerminal(payment));
    const blockingPayment = activeOrders.find((payment) => (
      projectedPaymentState(payment) === "payment_confirming"
    ));
    if (blockingPayment) {
      setPendingPurchase(null);
      setSelectedOrderID(blockingPayment.order_id);
      return;
    }
    const sameOrder = activeOrders.find((payment) => (
      payment.plan_id === intent.plan.plan_id
      && payment.payment_channel === intent.channel
      && payment.order_type === "subscription_purchase"
    ));
    if (sameOrder) {
      setPendingPurchase(null);
      setSelectedOrderID(sameOrder.order_id);
      return;
    }
    if (activeOrders.length > 0) {
      setPendingPurchase(intent);
      return;
    }
    void performCreate(intent);
  };

  const renderOrderSummary = (payment: CloudPaymentView, terminal: boolean) => {
    const state = projectedPaymentState(payment);
    const retryPlan = plans.find((plan) => plan.plan === payment.plan);
    const retryAction = retryPlan ? actionForPlan(retryPlan) : null;
    const canReorder = terminal
      && paymentCanReorder(payment)
      && retryPlan !== undefined
      && retryAction !== null;
    return (
      <article
        className="account-payments__order-summary"
        data-payment-order-summary={payment.order_id}
        data-payment-selected={selectedOrderID === payment.order_id ? "true" : undefined}
        data-payment-status={state}
        key={payment.order_id}
      >
        <div className="account-payments__order-summary-main">
          <strong>{planName(payment.plan)}</strong>
          <span>{formatMoney(payment.amount, payment.amount_unit, payment.currency, locale)}</span>
          <span>{t(paymentChannelKeys[payment.payment_channel])}</span>
        </div>
        <span className="account-payments__status" data-payment-status-badge={state}>
          {t(paymentStateKeys[state])}
        </span>
        <div className="account-payments__order-summary-actions">
          <button
            data-payment-select-order={payment.order_id}
            onClick={() => setSelectedOrderID(payment.order_id)}
            type="button"
          >
            {t("account.payment.action.viewOrder")}
          </button>
          {canReorder ? (
            <button
              data-payment-reorder={payment.order_id}
              disabled={creatingAction !== "" || confirmingPayment !== null}
              onClick={() => {
                if (retryPlan && retryAction) requestCreate({ action: retryAction, plan: retryPlan, channel: payment.payment_channel });
              }}
              type="button"
            >
              {t("account.payment.action.reorder")}
            </button>
          ) : null}
        </div>
      </article>
    );
  };

  const returnToPlans = () => {
    if (selectedPayment) {
      const restoredChannelEnabled = paymentChannelOptions.some((option) => (
        option.channel === selectedPayment.payment_channel && option.enabled
      ));
      setSelectedChannel(restoredChannelEnabled ? selectedPayment.payment_channel : "alipay");
    }
    setSelectedOrderID(null);
  };

  const steps: Array<{ number: 1 | 2 | 3; label: DictKey }> = [
    { number: 1, label: "account.payment.step.select" },
    { number: 2, label: "account.payment.step.scan" },
    { number: 3, label: "account.payment.step.confirm" },
  ];

  return (
    <section
      className="account-payments"
      data-account-payments
      data-payment-actions-enabled={paymentsEnabled ? "true" : "false"}
    >
      {!paymentsEnabled ? (
        <p className="account-payments__notice" data-payment-disabled-notice>
          {t("account.payment.disabledNotice")}
        </p>
      ) : null}

      {pollNotice ? (
        <p
          className={`account-payments__notice account-payments__notice--${pollNotice}`}
          data-payment-poll-notice={pollNotice}
          role={pollNotice === "auth" ? "alert" : "status"}
        >
          {t(pollNoticeKeys[pollNotice])}
        </p>
      ) : null}

      {confirmingPayment && selectedPayment?.order_id !== confirmingPayment.order_id ? (
        <p className="account-payments__notice" data-payment-confirming-guard role="status">
          {t("account.payment.confirmingGuard", { plan: planName(confirmingPayment.plan) })}
        </p>
      ) : null}

      {selectedPayment ? (
        <ol className="account-payments__steps" data-payment-stage={stage}>
          {steps.map((step) => {
            const state = completed || step.number < currentStep
              ? "complete"
              : step.number === currentStep ? "current" : "upcoming";
            return (
              <li
                aria-current={state === "current" ? "step" : undefined}
                data-payment-step={step.number}
                data-payment-step-state={state}
                key={step.number}
              >
                <span aria-hidden="true">{state === "complete" ? "✓" : step.number}</span>
                <strong>{t(step.label)}</strong>
              </li>
            );
          })}
        </ol>
      ) : null}

      {!selectedPayment ? <section className="account-payments__section account-payments__catalog">
        <h2 className="sr-only">{t("account.payment.plansTitle")}</h2>
        <div className="account-payments__catalog-toolbar">
          <details className="account-payments__history account-payments__history--catalog" data-payment-history>
            <summary>
              <History size={15} aria-hidden="true" />
              {t("account.payment.historyTitle")}
              <span>{terminalPayments.length}</span>
            </summary>
            {terminalPayments.length > 0 ? (
              <div className="account-payments__order-list">
                {terminalPayments.map((payment) => renderOrderSummary(payment, true))}
              </div>
            ) : (
              <p className="account-payments__empty" data-payment-empty="history">
                {t("account.payment.noHistory")}
              </p>
            )}
          </details>
        </div>

        {pendingPurchase ? (
          <div className="account-payments__switch-confirm" data-payment-switch-confirm role="alert">
            <div>
              <strong>{t("account.payment.switchTitle")}</strong>
              <p>{t("account.payment.switchWarning", { plan: planName(pendingPurchase.plan.plan) })}</p>
            </div>
            <div className="account-payments__switch-actions">
              <button
                data-payment-switch-keep
                onClick={() => {
                  setPendingPurchase(null);
                  const current = [...payments].reverse().find((payment) => !paymentTerminal(payment));
                  if (current) setSelectedOrderID(current.order_id);
                }}
                type="button"
              >
                {t("account.payment.action.keepOrder")}
              </button>
              <button
                data-payment-switch-continue
                disabled={creatingAction !== "" || confirmingPayment !== null}
                onClick={() => void performCreate(pendingPurchase)}
                type="button"
              >
                {t("account.payment.action.continuePurchase")}
              </button>
            </div>
          </div>
        ) : null}

        <fieldset
          aria-label={t("account.payment.preferredChannel")}
          className="account-payments__channel-selector"
          data-payment-channel-selector
        >
          <legend>{t("account.payment.channelTitle")}</legend>
          <div>
            {paymentChannelOptions.map(({ channel, enabled }) => (
              <button
                aria-pressed={enabled && selectedChannel === channel}
                className={enabled && selectedChannel === channel ? "account-payments__channel account-payments__channel--active" : "account-payments__channel"}
                data-payment-channel-option={channel}
                disabled={!enabled || creatingAction !== "" || confirmingPayment !== null}
                key={channel}
                onClick={() => {
                  if (!enabled) return;
                  setSelectedChannel(channel);
                  setPendingPurchase(null);
                }}
                type="button"
              >
                <span>{t(paymentChannelKeys[channel])}</span>
                {!enabled ? <small>{t("account.payment.channel.comingSoon")}</small> : null}
              </button>
            ))}
          </div>
        </fieldset>

        <div className="account-payments__plans" data-payment-plans>
          {visiblePlans.length > 0 ? visiblePlans.map((plan) => {
            const current = plan.plan === subscription?.current.plan;
            const pending = pendingPurchase?.plan.plan === plan.plan;
            const action = actionForPlan(plan);
            const canPurchase = action !== null;
            const intent = action ? { action, plan, channel: selectedChannel } : null;
            const presentation = planPresentations[plan.plan];
            const totalCredit = formatDecimalString(plan.credit_amount, locale);
            return (
              <article
                className={[
                  "account-payments__plan",
                  plan.plan === recommendedPlan ? "account-payments__plan--recommended" : "",
                  current ? "account-payments__plan--current" : "",
                ].filter(Boolean).join(" ")}
                data-payment-current={current ? "true" : undefined}
                data-payment-plan={plan.plan}
                data-payment-recommended={plan.plan === recommendedPlan ? "true" : undefined}
                data-payment-selected={pending ? "true" : undefined}
                key={plan.plan}
              >
                <div className="account-payments__plan-content">
                  <div className="account-payments__plan-summary">
                    <span className="account-payments__plan-heading">
                      <strong>{presentation?.displayName ?? plan.display_name}</strong>
                      <span className="account-payments__plan-badges">
                        {plan.plan === recommendedPlan ? <span className="account-payments__recommended">{t("account.payment.recommended")}</span> : null}
                        {current ? <span className="account-payments__current">{t("account.payment.currentPlan")}</span> : null}
                      </span>
                    </span>
                    <span className="account-payments__plan-price" data-payment-plan-price>
                      {formatMoney(plan.price_amount, undefined, plan.currency, locale)}
                      <span>{t("account.payment.periodValue", { days: plan.period_days })}</span>
                    </span>
                    {presentation ? <p>{t("account.payment.experienceSummary")}</p> : null}
                  </div>
                  <div className="account-payments__plan-details">
                    <div className="account-payments__plan-benefits">
                      {presentation ? (
                        <>
                          <span data-payment-plan-credit>
                            <Coins size={18} aria-hidden="true" />
                            <span className="account-payments__plan-benefit-copy">
                              <small>{t("account.payment.monthlyCreditLabel")}</small>
                              <strong>{totalCredit}</strong>
                            </span>
                          </span>
                          <span className="account-payments__plan-benefit--models" data-payment-plan-models>
                            <Cpu size={18} aria-hidden="true" />
                            <span className="account-payments__plan-benefit-copy">
                              <span>{t(presentation.modelTier)}</span>
                              <small>{t(presentation.modelExamples)}</small>
                            </span>
                          </span>
                          <span data-payment-plan-automation-tasks>
                            <Bot size={18} aria-hidden="true" />
                            <span className="account-payments__plan-benefit-copy">
                              <small>{t("account.payment.automationTasksLabel")}</small>
                              <strong>{presentation.automationTasksUnlimited ? t("account.payment.unlimited") : null}</strong>
                            </span>
                          </span>
                          <span data-payment-plan-projects>
                            <Folder size={18} aria-hidden="true" />
                            <span className="account-payments__plan-benefit-copy">
                              <small>{t("account.payment.projectsLabel")}</small>
                              <strong>{presentation.projectsUnlimited
                                ? t("account.payment.unlimited")
                                : formatDecimal(presentation.projects, locale)}</strong>
                            </span>
                          </span>
                        </>
                      ) : (
                        <>
                          <span data-payment-plan-credit>
                            <Coins size={17} aria-hidden="true" />
                            {t("account.payment.creditPerPeriod", {
                              credit: totalCredit,
                            })}
                          </span>
                          <span>
                            <CalendarDays size={17} aria-hidden="true" />
                            {t("account.payment.validity", { days: plan.period_days })}
                          </span>
                          <span>
                            <ShieldCheck size={17} aria-hidden="true" />
                            {t("account.payment.autoEffective")}
                          </span>
                        </>
                      )}
                    </div>
                    {current && subscriptionPeriodEnd ? (
                      <span className="account-payments__plan-expiry" data-payment-plan-period-end>
                        {t("account.subscriptionValidUntil")} <time dateTime={subscription?.current.period_end}>{subscriptionPeriodEnd}</time>
                      </span>
                    ) : null}
                    <span hidden data-payment-plan-period>{plan.period_days}</span>
                    <button
                      className={`account-payments__plan-action${plan.plan === recommendedPlan ? " account-payments__plan-action--recommended" : ""}`}
                      data-payment-create={action ?? undefined}
                      data-payment-create-plan={action ? plan.plan : undefined}
                      data-payment-channel={selectedChannel}
                      data-payment-plan-action={plan.plan}
                      disabled={!paymentsEnabled || !canPurchase || creatingAction !== "" || confirmingPayment !== null}
                      onClick={() => {
                        if (intent) requestCreate(intent);
                      }}
                      type="button"
                    >
                      {intent && creatingAction === purchaseKey(intent)
                        ? t("account.payment.action.creating")
                        : current
                          ? t("account.payment.currentPlan")
                          : action === "upgrade"
                            ? t("account.payment.action.upgrade")
                            : action === "purchase"
                              ? t("account.payment.action.purchase")
                              : t("account.payment.action.unavailable")}
                    </button>
                  </div>
                </div>
              </article>
            );
          }) : (
            <p className="account-payments__empty" data-payment-empty="plans">{t("account.payment.noPlans")}</p>
          )}
        </div>

        {visiblePlans.length > 0 ? (
          <aside className="account-payments__catalog-notes" data-payment-catalog-notes>
            <span>
              <ShieldCheck size={17} aria-hidden="true" />
              {catalogPeriodDays !== null
                ? t("account.payment.catalogTermValidity", { days: catalogPeriodDays })
                : t("account.payment.catalogTermValidityByPlan")}
            </span>
            <span>
              <ShieldCheck size={17} aria-hidden="true" />
              {t("account.payment.catalogTermExpiry")}
            </span>
            <span>
              <ShieldCheck size={17} aria-hidden="true" />
              {t("account.payment.catalogTermFree")}
            </span>
          </aside>
        ) : null}
      </section> : null}

      {selectedPayment ? (() => {
        const state = projectedPaymentState(selectedPayment);
        const entry = paymentEntry(selectedPayment, expiryClock);
        const succeeded = state === "payment_succeeded" && selectedPayment.entitlement_granted;
        const paymentConfirmed = selectedPayment.state === "payment_succeeded";
        const retryPlan = plans.find((plan) => plan.plan === selectedPayment.plan);
        const retryAction = retryPlan ? actionForPlan(retryPlan) : null;
        const canReorder = paymentCanReorder(selectedPayment)
          && retryPlan !== undefined
          && retryAction !== null;
        const title = stage === "scan"
          ? t("account.payment.checkoutTitle")
          : stage === "confirm"
            ? t("account.payment.confirmationTitle")
            : stage === "success"
              ? t("account.payment.successTitle")
              : t(paymentStateKeys[state]);
        const resultStatusLabel = paymentConfirmed && !selectedPayment.entitlement_granted
          ? t("account.payment.progress.entitlementPending")
          : t(paymentStateKeys[state]);
        const resultHint = stage === "confirm"
          ? t(paymentConfirmed
            ? "account.payment.entitlementHint"
            : "account.payment.confirmingHint")
          : stage === "success"
            ? t("account.payment.successHint")
            : selectedPayment.state === "payment_manual_required"
              ? t("account.payment.manualHint")
              : t("account.payment.terminalHint");
        return (
          <section
            className={`account-payments__section account-payments__checkout account-payments__checkout--${stage}`}
            data-payment-order={selectedPayment.order_id}
            data-payment-stage-view={stage}
            data-payment-status={state}
            data-payment-success={succeeded ? "true" : undefined}
          >
            <div className="account-payments__section-heading">
              <div>
                <h2>{title}</h2>
                <p>{t("account.payment.orderStillValid")}</p>
              </div>
              <button
                className="account-payments__back"
                data-payment-collapse
                onClick={returnToPlans}
                type="button"
              >
                {t("account.payment.action.backToPlans")}
              </button>
            </div>
            {stage === "scan" ? (
              <div className="account-payments__checkout-body">
                <div className="account-payments__checkout-info">
                  <div className="account-payments__checkout-title">
                    <strong>{planName(selectedPayment.plan)}</strong>
                    <span className="account-payments__status" data-payment-status-badge={state}>
                      {t(paymentStateKeys[state])}
                    </span>
                  </div>
                  <strong className="account-payments__checkout-amount">
                    {formatMoney(
                      selectedPayment.amount,
                      selectedPayment.amount_unit,
                      selectedPayment.currency,
                      locale,
                    )}
                  </strong>
                  <dl className="account-payments__facts account-payments__order-facts">
                    <div>
                      <dt>{t("account.payment.channelLabel")}</dt>
                      <dd>{t(paymentChannelKeys[selectedPayment.payment_channel])}</dd>
                    </div>
                    <div>
                      <dt>{t("account.payment.statusLabel")}</dt>
                      <dd>{t(paymentStateKeys[state])}</dd>
                    </div>
                  </dl>
                </div>
                {entry ? (
                  <div
                    className="account-payments__entry"
                    data-payment-entry
                    data-payment-entry-field={entry.field}
                    data-payment-entry-value={entry.value}
                  >
                    <span className="account-payments__entry-label">{t("account.payment.entryLabel")}</span>
                    <QRCodeSVG aria-label={t("account.payment.entryLabel")} size={192} value={entry.value} />
                    <strong className="account-payments__countdown" data-payment-countdown>
                      {t("account.payment.expiresIn", {
                        time: formatCountdown(entry.expiresAt - expiryClock),
                      })}
                    </strong>
                    <span className="account-payments__scan-hint">{t("account.payment.scanHint")}</span>
                  </div>
                ) : (
                  <div
                    className="account-payments__entry-waiting"
                    data-payment-entry-unavailable
                    role="status"
                  >
                    <span aria-hidden="true">…</span>
                    <strong>{t(paymentStateKeys[state])}</strong>
                    <p>{t("account.payment.entryUnavailable")}</p>
                  </div>
                )}
              </div>
            ) : (
              <div
                aria-live="polite"
                className="account-payments__result"
                data-payment-confirmation={stage === "confirm" ? "true" : undefined}
                role="status"
              >
                <div
                  className="account-payments__result-mark"
                  data-payment-result-mark={stage}
                  aria-hidden="true"
                >
                  {stage === "success" ? "✓" : stage === "confirm" ? "" : "!"}
                </div>
                <div className="account-payments__result-heading">
                  <strong>{title}</strong>
                  <p>{resultHint}</p>
                  <span className="account-payments__status" data-payment-status-badge={state}>
                    {resultStatusLabel}
                  </span>
                </div>
                <dl className="account-payments__result-summary">
                  <div>
                    <dt>{t("account.payment.orderPlan")}</dt>
                    <dd>{planName(selectedPayment.plan)}</dd>
                  </div>
                  <div>
                    <dt>{t("account.payment.orderAmount")}</dt>
                    <dd>
                      {formatMoney(
                        selectedPayment.amount,
                        selectedPayment.amount_unit,
                        selectedPayment.currency,
                        locale,
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>{t("account.payment.channelLabel")}</dt>
                    <dd>{t(paymentChannelKeys[selectedPayment.payment_channel])}</dd>
                  </div>
                </dl>
                {stage === "confirm" || stage === "success" ? (
                  <div className="account-payments__progress" data-payment-progress>
                    <div data-payment-progress-state="complete">
                      <span aria-hidden="true">✓</span>
                      <strong>{t("account.payment.progress.orderCreated")}</strong>
                    </div>
                    <div data-payment-progress-state={paymentConfirmed ? "complete" : "current"}>
                      <span aria-hidden="true">{paymentConfirmed ? "✓" : "2"}</span>
                      <strong>
                        {t(paymentConfirmed
                          ? "account.payment.progress.paymentConfirmed"
                          : "account.payment.status.confirming")}
                      </strong>
                    </div>
                    <div
                      data-payment-progress-state={
                        succeeded ? "complete" : paymentConfirmed ? "current" : "upcoming"
                      }
                    >
                      <span aria-hidden="true">{succeeded ? "✓" : "3"}</span>
                      <strong>
                        {t(succeeded
                          ? "account.payment.progress.entitlementGranted"
                          : "account.payment.progress.entitlementPending")}
                      </strong>
                    </div>
                  </div>
                ) : null}
                {stage === "confirm" ? (
                  <p className="account-payments__auto-update">{t("account.payment.autoUpdateHint")}</p>
                ) : null}
                {canReorder ? (
                  <button
                    className="account-payments__primary-action"
                    data-payment-reorder={selectedPayment.order_id}
                    disabled={creatingAction !== "" || confirmingPayment !== null}
                    onClick={() => requestCreate({
                      action: retryAction!,
                      plan: retryPlan!,
                      channel: selectedPayment.payment_channel,
                    })}
                    type="button"
                  >
                    {t("account.payment.action.reorder")}
                  </button>
                ) : null}
              </div>
            )}
          </section>
        );
      })() : null}

      {activePayments.length > 0 ? <section className="account-payments__section account-payments__orders">
        <div className="account-payments__section-heading">
          <div>
            <h2>{t("account.payment.ordersTitle")}</h2>
            <p>{t("account.payment.ordersHint")}</p>
          </div>
        </div>
        {activePayments.length > 0 ? (
          <div className="account-payments__order-group" data-payment-active-orders>
            <h3>{t("account.payment.activeOrdersTitle")}</h3>
            <div className="account-payments__order-list">
              {activePayments.map((payment) => renderOrderSummary(payment, false))}
            </div>
          </div>
        ) : null}
      </section> : null}
    </section>
  );
}
