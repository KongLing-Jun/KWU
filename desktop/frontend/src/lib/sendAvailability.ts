import type { TabSendAvailability, TabSendAvailabilityState } from "./types";
import type { SubscriptionAccountState } from "./subscriptionModelAccess";

const SUBSCRIPTION_BILLING_SOURCE = "subscription_pool";

export function resolveDisplayedSendAvailability(input: {
  availability?: TabSendAvailability;
  billingSource?: string | null;
  accountState: SubscriptionAccountState;
  totalBalance?: string;
}): TabSendAvailability | undefined {
  const subscriptionRoute = input.billingSource?.trim() === SUBSCRIPTION_BILLING_SOURCE;
  if (
    input.accountState === "anonymous"
    && subscriptionRoute
  ) {
    return {
      state: "subscription_auth_required",
      retryable: true,
      action: "login",
      detail: input.availability?.detail,
    };
  }
  const totalBalance = input.totalBalance?.trim();
  if (
    input.accountState === "ready"
    && subscriptionRoute
    && input.availability?.state === "ready"
    && totalBalance
    && Number(totalBalance) <= 0
  ) {
    return {
      state: "subscription_balance_depleted",
      retryable: false,
      action: "account",
    };
  }
  return input.availability;
}

export function sendAvailabilityAllowsControllerActions(availability?: TabSendAvailability): boolean {
  return availability?.state === "ready";
}

export function sendAvailabilityMessageKey(state: TabSendAvailabilityState, hasHistory: boolean) {
  switch (state) {
    case "subscription_auth_required":
      return hasHistory ? "sendAvailability.subscriptionAuthHistory" : "sendAvailability.subscriptionAuthBlank";
    case "subscription_account_action_required":
      return hasHistory ? "sendAvailability.accountHistory" : "sendAvailability.accountBlank";
    case "subscription_balance_depleted":
      return hasHistory ? "sendAvailability.balanceDepletedHistory" : "sendAvailability.balanceDepletedBlank";
    case "byok_configuration_required":
      return hasHistory ? "sendAvailability.byokHistory" : "sendAvailability.byokBlank";
    case "service_unavailable":
      return hasHistory ? "sendAvailability.serviceHistory" : "sendAvailability.serviceBlank";
    case "startup_failed":
      return hasHistory ? "sendAvailability.startupHistory" : "sendAvailability.startupBlank";
    case "starting":
      return "sendAvailability.starting";
    case "ready":
      return undefined;
  }
}
