import { CircleAlert, LogIn } from "lucide-react";
import type { TabSendAvailability } from "../lib/types";
import { useT } from "../lib/i18n";
import { sendAvailabilityMessageKey } from "../lib/sendAvailability";

export function SendAvailabilityBanner({
  availability,
  hasHistory,
  onLogin,
  onOpenAccount,
  onOpenPayments,
  onConfigureBYOK,
  onRetry,
  onSwitchModel,
}: {
  availability?: TabSendAvailability;
  hasHistory: boolean;
  onLogin: () => void;
  onOpenAccount: () => void;
  onOpenPayments: () => void;
  onConfigureBYOK: () => void;
  onRetry: () => void | Promise<void>;
  onSwitchModel: () => void;
}) {
  const t = useT();
  if (!availability || availability.state === "ready" || availability.state === "starting") return null;
  const messageKey = sendAvailabilityMessageKey(availability.state, hasHistory);
  if (!messageKey) return null;
  const isSubscriptionAuth = availability.state === "subscription_auth_required";
  const isBalanceDepleted = availability.state === "subscription_balance_depleted";
  return (
    <div
      className={`send-availability banner banner--warning${isSubscriptionAuth ? " send-availability--auth" : ""}`}
      role="status"
      aria-live="polite"
      aria-atomic="true"
      data-send-availability={availability.state}
    >
      <div className="send-availability__copy">
        <span className="send-availability__icon" aria-hidden="true">
          {isSubscriptionAuth ? <LogIn size={17} /> : <CircleAlert size={17} />}
        </span>
        <div className="send-availability__content">
          {isSubscriptionAuth || isBalanceDepleted ? (
            <strong className="send-availability__title">
              {t(isBalanceDepleted ? "sendAvailability.balanceDepletedTitle" : "sendAvailability.subscriptionAuthTitle")}
            </strong>
          ) : null}
          <span className="banner__msg">{t(messageKey)}</span>
          {availability.detail ? (
            <details className="banner__details">
              <summary>{t("sendAvailability.details")}</summary>
              <code>{availability.detail}</code>
            </details>
          ) : null}
        </div>
      </div>
      <div className="send-availability__actions">
        {availability.action === "login" && (
          <button className="btn btn--small btn--primary" type="button" onClick={onLogin}>{t("sendAvailability.login")}</button>
        )}
        {availability.action === "account" && (
          <button
            className="btn btn--small btn--primary"
            type="button"
            onClick={isBalanceDepleted ? onOpenPayments : onOpenAccount}
          >
            {t(isBalanceDepleted ? "account.rechargeCredits" : "sendAvailability.account")}
          </button>
        )}
        {availability.action === "configure_byok" && (
          <button className="btn btn--small btn--primary" type="button" onClick={onConfigureBYOK}>{t("sendAvailability.configureByok")}</button>
        )}
        {availability.retryable && availability.action === "retry" && (
          <button className="btn btn--small" type="button" onClick={() => void onRetry()}>{t("common.retry")}</button>
        )}
        <button className="btn btn--small" type="button" onClick={onSwitchModel}>{t("sendAvailability.switchModel")}</button>
      </div>
    </div>
  );
}
