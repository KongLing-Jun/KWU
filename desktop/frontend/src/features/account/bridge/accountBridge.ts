import { app, type AppBindings } from "../../../lib/bridge";

export type AccountPaymentBindings = Pick<
  AppBindings,
  | "CloudCreateSubscriptionOrder"
  | "CloudCreateUpgradeOrder"
  | "CloudConfirmPayment"
  | "CloudRefreshPayment"
>;

export type AccountRechargeBindings = Pick<
  AppBindings,
  | "CloudCreateMaaSRechargeOrder"
  | "CloudGetMaaSRechargeKeyBalance"
  | "CloudGetMaaSRechargeOrder"
  | "CloudRefreshAssets"
>;

export type AccountCloudBindings = Pick<
  AppBindings,
  | "CloudBootstrap"
  | "CloudCaptchaConfig"
  | "CloudSendRegisterSMS"
  | "CloudSendPasswordResetSMS"
  | "CloudResetPassword"
  | "CloudRegister"
  | "CloudLogin"
  | "CloudSwitchAccount"
  | "CloudLogout"
  | "CloudRefreshAssets"
  | "CloudCreateSubscriptionOrder"
  | "CloudCreateUpgradeOrder"
  | "CloudConfirmPayment"
  | "CloudRefreshPayment"
  | "CloudCreateMaaSRechargeOrder"
  | "CloudGetMaaSRechargeKeyBalance"
  | "CloudGetMaaSRechargeOrder"
> & Partial<Pick<AppBindings, "CloudCachedAssets">>;

export const accountBridge: AccountCloudBindings = app;
