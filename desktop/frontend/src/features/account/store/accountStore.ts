import { create } from "zustand";

export type CloudAccountStatus =
  | "anonymous"
  | "authenticating"
  | "authenticated"
  | "refreshing"
  | "expired"
  | "offline";

export interface CloudAccountView {
  status: CloudAccountStatus;
  profile_id: string;
  cloud_user_id?: string;
  binding_generation?: number;
  display_name?: string;
  role?: string;
  last_sync_at?: string;
}

export interface CloudBalanceView {
  total_balance: string;
  available_balance?: string;
  frozen_balance?: string;
  free_available_balance?: string;
  subscription_available_balance?: string;
  credit_per_cny?: string;
  version?: number;
}

export interface CloudSubscriptionPlan {
  plan_id?: string;
  plan: string;
  display_name: string;
  price_amount: string;
  currency: string;
  credit_amount: string;
  period_days: number;
  enabled: boolean;
  sort_order: number;
  config_version: string;
}

export interface CloudCurrentSubscriptionView {
  plan: string;
  status: "free" | "active" | "expired";
  period_end?: string;
  version?: number;
  updated_at?: string;
}

export interface CloudSubscriptionView {
  plans: CloudSubscriptionPlan[];
  current: CloudCurrentSubscriptionView;
}

export type CloudPaymentState =
  | "order_created"
  | "paying"
  | "payment_confirming"
  | "payment_succeeded"
  | "payment_closed"
  | "payment_failed"
  | "payment_timeout"
  | "payment_manual_required";

export interface CloudPaymentEntryView {
  entry_type: "qr_code" | "native_qr";
  qr_code?: string;
  code_url?: string;
  qr_code_url?: string;
  expires_at: string;
}

export interface CloudPaymentView {
  order_id: string;
  plan_id?: string;
  order_type: "subscription_purchase" | "subscription_upgrade";
  plan: string;
  from_plan?: string;
  amount: string;
  amount_unit?: "yuan" | "fen";
  currency: string;
  payment_channel: "alipay" | "wechat";
  state: CloudPaymentState;
  payable: boolean;
  entitlement_granted: boolean;
  expires_at: string;
  payment_entry?: CloudPaymentEntryView | null;
  version?: number;
}

export interface CloudPaymentOperationView {
  account: CloudAccountView;
  payment: CloudPaymentView;
  balance?: CloudBalanceView | null;
  subscription?: CloudSubscriptionView | null;
}

export type CloudMaaSRechargeStatus =
  | "creating"
  | "payment_pending"
  | "polling"
  | "paid_verified"
  | "credited"
  | "create_failed"
  | "payment_failed"
  | "timeout"
  | "manual_required";

export interface CloudMaaSRechargeOrderView {
  order_id: string;
  amount: string;
  amount_unit: "yuan";
  currency: "CNY";
  payment_method: "alipay";
  recharge_key_name: string;
  conversion_rule_version: string;
  quota_per_usd: string;
  cny_per_usd: string;
  quota_delta: number;
  credit_delta?: string;
  credit_per_cny?: string;
  status: CloudMaaSRechargeStatus;
  qr_code_url?: string;
  expires_at: string;
  paid: boolean;
  credited: boolean;
  paid_at?: string;
  credited_at?: string;
  remote_quota_after?: number;
  remote_credit_after?: string;
  version: number;
}

export interface CloudMaaSRechargeKeyBalanceView {
  key_name: string;
  available_balance: string;
  remain_quota: number;
  balance_cny: string;
  source: "maas_recharge_api_key" | "";
  conversion_rule_version: string;
  cny_per_usd: string;
  quota_per_usd: string;
  queried_at?: string;
  available_credits?: string;
  package_available_credits?: string;
  credit_per_cny?: string;
}

export interface CloudBootstrapView {
  available: boolean;
  capabilities?: CloudCapabilitiesView;
  account: CloudAccountView;
  balance?: CloudBalanceView | null;
  subscription?: CloudSubscriptionView | null;
  operation_error?: CloudAccountError | null;
  candidate?: CloudAccountView | null;
  payments?: CloudPaymentView[] | null;
}

export interface CloudCachedAssetsView {
  profile_id: string;
  cloud_user_id: string;
  binding_generation: number;
  balance?: CloudBalanceView | null;
  subscription?: CloudSubscriptionView | null;
}

export interface AccountAssetRequest {
  contextKey: string;
  scopeKey: string;
  generation: number;
}

export type CloudRegistrationMode = "fixture" | "development" | "real" | "unavailable";

export interface CloudCapabilitiesView {
  registration: boolean;
  registration_mode: CloudRegistrationMode;
  payments: boolean;
  realtime: boolean;
}

export interface CloudAccountError {
  code: string;
  message: string;
  retryable?: boolean;
  request_id?: string;
  diagnostic_id?: string;
}

interface AccountStateData {
  contextKey: string;
  initialized: boolean;
  available: boolean;
  capabilities: CloudCapabilitiesView;
  scopeKey: string;
  account: CloudAccountView | null;
  balance: CloudBalanceView | null;
  subscription: CloudSubscriptionView | null;
  payments: CloudPaymentView[];
  error: CloudAccountError | null;
  assetSyncError: string | null;
  busy: boolean;
  assetRequestGeneration: number;
}

interface AccountState extends AccountStateData {
  activateContext(contextKey: string): void;
  applyBootstrap(view: CloudBootstrapView): void;
  applyPaymentOperation(view: CloudPaymentOperationView): void;
  beginAssetRequest(contextKey: string): AccountAssetRequest | null;
  applyCachedAssets(view: CloudCachedAssetsView, request: AccountAssetRequest): boolean;
  recordAssetSyncFailure(request: AccountAssetRequest, message: string): void;
  finishBootstrap(): void;
  setBusy(busy: boolean): void;
  setError(error: CloudAccountError | null): void;
  reset(): void;
}

export function emptyAccountState(): AccountStateData {
  return {
    contextKey: "",
    initialized: false,
    available: false,
    capabilities: unavailableCapabilities(),
    scopeKey: "",
    account: null,
    balance: null,
    subscription: null,
    payments: [],
    error: null,
    assetSyncError: null,
    busy: false,
    assetRequestGeneration: 0,
  };
}

function unavailableCapabilities(): CloudCapabilitiesView {
  return { registration: false, registration_mode: "unavailable", payments: false, realtime: false };
}

function normalizeCapabilities(capabilities?: CloudCapabilitiesView): CloudCapabilitiesView {
  if (!capabilities) return unavailableCapabilities();
  const registrationMode = capabilities.registration_mode === "fixture"
    || capabilities.registration_mode === "development"
    || capabilities.registration_mode === "real"
    ? capabilities.registration_mode
    : "unavailable";
  return {
    registration: capabilities.registration === true && registrationMode !== "unavailable",
    registration_mode: registrationMode,
    payments: capabilities.payments === true,
    realtime: capabilities.realtime === true,
  };
}

type AccountSnapshot = Pick<
  AccountStateData,
  "available" | "capabilities" | "scopeKey" | "account" | "balance" | "subscription" | "payments"
>;

const accountSnapshots = new Map<string, AccountSnapshot>();
let assetRequestSequence = 0;

function nextAssetRequestGeneration(): number {
  assetRequestSequence += 1;
  return assetRequestSequence;
}

export function hasRecoverableCloudIdentity(account: CloudAccountView | null): boolean {
  return account?.status === "authenticated"
    || (account?.status === "offline" && Boolean(account.cloud_user_id));
}

export function requiresReauthentication(account: CloudAccountView | null): boolean {
  return account?.status === "expired";
}

function cacheAccountState(state: AccountStateData, allowRecoverableSnapshot = true): void {
  if (!state.contextKey || !state.initialized) return;
  const current = accountSnapshots.get(state.contextKey);
  if (
    hasRecoverableCloudIdentity(state.account)
    && (!allowRecoverableSnapshot || state.error !== null || state.balance === null || state.subscription === null)
  ) {
    // A response with no authoritative assets invalidates the old snapshot.
    // When at least one live asset advanced, keep the last complete snapshot
    // for re-entry instead of replacing it with incomplete data.
    if (current && state.balance === null && state.subscription === null) {
      accountSnapshots.delete(state.contextKey);
    }
    return;
  }
  accountSnapshots.set(state.contextKey, {
    available: state.available,
    capabilities: state.capabilities,
    scopeKey: state.scopeKey,
    account: state.account,
    balance: state.balance,
    subscription: state.subscription,
    payments: state.payments,
  });
}

function accountStateForContext(contextKey: string): AccountStateData {
  const snapshot = accountSnapshots.get(contextKey);
  if (!snapshot) {
    return {
      ...emptyAccountState(),
      contextKey,
      assetRequestGeneration: nextAssetRequestGeneration(),
    };
  }
  return {
    contextKey,
    initialized: true,
    ...snapshot,
    error: null,
    assetSyncError: null,
    busy: false,
    assetRequestGeneration: nextAssetRequestGeneration(),
  };
}

function bootstrapProvidesStableSnapshot(view: CloudBootstrapView): boolean {
  return !hasRecoverableCloudIdentity(view.account)
    || (
      view.account.status === "authenticated"
      && Boolean(view.account.cloud_user_id)
      && view.operation_error == null
      && view.balance != null
      && view.subscription != null
    );
}

export function cloudScopeKey(account: CloudAccountView): string {
  return [account.profile_id, account.cloud_user_id ?? "", account.binding_generation ?? 0].join("\u0000");
}

function samePaymentIdentity(current: CloudPaymentView, incoming: CloudPaymentView): boolean {
  return current.order_id === incoming.order_id
    && (current.plan_id ?? "") === (incoming.plan_id ?? "")
    && current.order_type === incoming.order_type
    && current.plan === incoming.plan
    && (current.from_plan ?? "") === (incoming.from_plan ?? "")
    && current.amount === incoming.amount
    && current.amount_unit === incoming.amount_unit
    && current.currency === incoming.currency
    && current.payment_channel === incoming.payment_channel
    && current.expires_at === incoming.expires_at;
}

function paymentTerminal(payment: CloudPaymentView): CloudPaymentState | null {
  if (payment.state === "payment_succeeded") {
    return payment.entitlement_granted ? payment.state : null;
  }
  return payment.state === "payment_closed"
    || payment.state === "payment_failed"
    || payment.state === "payment_timeout"
    || payment.state === "payment_manual_required"
    ? payment.state
    : null;
}

function paymentProgress(payment: CloudPaymentView): number {
  switch (payment.state) {
    case "order_created": return 0;
    case "paying": return 1;
    case "payment_confirming": return 2;
    case "payment_succeeded": return payment.entitlement_granted ? 4 : 3;
    default: return 4;
  }
}

function mergePayment(
  current: CloudPaymentView,
  incoming: CloudPaymentView,
): { accepted: boolean; payment: CloudPaymentView } {
  if (!samePaymentIdentity(current, incoming)) return { accepted: false, payment: current };
  const currentVersion = current.version ?? 0;
  const incomingVersion = incoming.version ?? 0;
  if (currentVersion > 0 && incomingVersion < currentVersion) {
    return { accepted: false, payment: current };
  }
  if (
    currentVersion > 0
    && incomingVersion === currentVersion
    && (
      incoming.state !== current.state
      || incoming.entitlement_granted !== current.entitlement_granted
    )
  ) {
    return { accepted: false, payment: current };
  }
  const currentTerminal = paymentTerminal(current);
  const incomingTerminal = paymentTerminal(incoming);
  if (currentTerminal !== null && currentTerminal !== incomingTerminal) {
    return { accepted: false, payment: current };
  }
  if (currentTerminal === null && incomingTerminal === null && paymentProgress(incoming) < paymentProgress(current)) {
    return { accepted: false, payment: current };
  }
  return { accepted: true, payment: incoming };
}

function mergePayments(current: CloudPaymentView[], incoming: CloudPaymentView[]): CloudPaymentView[] {
  const merged = [...current];
  for (const payment of incoming) {
    const index = merged.findIndex((candidate) => candidate.order_id === payment.order_id);
    if (index < 0) {
      merged.push(payment);
      continue;
    }
    merged[index] = mergePayment(merged[index], payment).payment;
  }
  return merged;
}

function incomingPaymentsCoverCurrentProgress(
  current: CloudPaymentView[],
  incoming: CloudPaymentView[],
): boolean {
  const incomingByOrder = new Map<string, CloudPaymentView | null>();
  for (const payment of incoming) {
    incomingByOrder.set(payment.order_id, incomingByOrder.has(payment.order_id) ? null : payment);
  }
  return current.every((payment) => {
    const candidate = incomingByOrder.get(payment.order_id);
    return candidate !== undefined && candidate !== null && mergePayment(payment, candidate).accepted;
  });
}

function mergeVersionedAsset<T>(
  current: T | null,
  incoming: T | null | undefined,
  version: (asset: T) => number,
  allowUnversioned = true,
): T | null {
  if (incoming == null) return current;
  const incomingVersion = version(incoming);
  const currentVersion = current == null ? 0 : version(current);
  if (incomingVersion > 0) {
    if (currentVersion > incomingVersion) return current;
    if (currentVersion === incomingVersion) return current;
    return incoming;
  }
  if (!allowUnversioned) return current;
  if (currentVersion > 0) return current;
  return incoming;
}

function mergeAccountAssets(
  state: Pick<AccountStateData, "balance" | "subscription">,
  incoming: Pick<CloudCachedAssetsView, "balance" | "subscription">,
  allowUnversioned = true,
): Pick<AccountStateData, "balance" | "subscription"> {
  return {
    balance: mergeVersionedAsset(
      state.balance,
      incoming.balance,
      (balance) => balance.version ?? 0,
      allowUnversioned,
    ),
    subscription: mergeVersionedAsset(
      state.subscription,
      incoming.subscription,
      (subscription) => subscription.current.version ?? 0,
      allowUnversioned,
    ),
  };
}

function cachedAssetsScopeKey(view: CloudCachedAssetsView): string {
  return [view.profile_id, view.cloud_user_id, view.binding_generation].join("\u0000");
}

export const useAccountStore = create<AccountState>()((set, get) => ({
  ...emptyAccountState(),
  activateContext: (contextKey) => set(accountStateForContext(contextKey)),
  applyBootstrap: (view) => {
    set((state) => {
      const scopeKey = cloudScopeKey(view.account);
      const capabilities = normalizeCapabilities(view.capabilities);
      if (requiresReauthentication(view.account)) {
        if (state.contextKey) accountSnapshots.delete(state.contextKey);
        return {
          ...state,
          initialized: true,
          available: view.available,
          capabilities,
          scopeKey,
          account: view.account,
          balance: null,
          subscription: null,
          payments: [],
          error: view.operation_error ?? null,
          assetSyncError: null,
          assetRequestGeneration: nextAssetRequestGeneration(),
        };
      }
      const currentGeneration = state.account?.binding_generation ?? 0;
      const incomingGeneration = view.account.binding_generation ?? 0;
      if (
        state.account !== null
        && state.account.profile_id === view.account.profile_id
        && currentGeneration > 0
        && (
          incomingGeneration < currentGeneration
          || (
            incomingGeneration === currentGeneration
            && (view.account.cloud_user_id ?? "") !== (state.account.cloud_user_id ?? "")
          )
        )
      ) {
        return state;
      }
      const sameRecoverableScope = scopeKey === state.scopeKey
        && hasRecoverableCloudIdentity(state.account)
        && hasRecoverableCloudIdentity(view.account);
      const stableSnapshot = bootstrapProvidesStableSnapshot(view);
      if (!sameRecoverableScope) {
        const next = {
          ...state,
          initialized: true,
          available: view.available,
          capabilities,
          scopeKey,
          account: view.account,
          balance: view.balance ?? null,
          subscription: view.subscription ?? null,
          payments: view.payments ?? [],
          error: view.operation_error ?? null,
          assetSyncError: null,
          assetRequestGeneration: nextAssetRequestGeneration(),
        };
        cacheAccountState(next, stableSnapshot);
        return next;
      }
      const incomingPayments = view.payments ?? [];
      const payments = mergePayments(state.payments, incomingPayments);
      if (!incomingPaymentsCoverCurrentProgress(state.payments, incomingPayments)) {
        const assets = view.operation_error != null && view.balance == null && view.subscription == null
          ? { balance: null, subscription: null }
          : mergeAccountAssets(state, view, false);
        const assetsAdvanced = assets.balance !== state.balance || assets.subscription !== state.subscription;
        const next = {
          ...state,
          initialized: true,
          balance: assets.balance,
          subscription: assets.subscription,
          payments,
          error: view.operation_error ?? state.error,
          assetSyncError: assetsAdvanced ? null : state.assetSyncError,
          assetRequestGeneration: assetsAdvanced
            ? nextAssetRequestGeneration()
            : state.assetRequestGeneration,
        };
        cacheAccountState(next, stableSnapshot);
        return next;
      }
      const assets = view.operation_error != null && view.balance == null && view.subscription == null
        ? { balance: null, subscription: null }
        : mergeAccountAssets(state, view);
      const next = {
        ...state,
        initialized: true,
        available: view.available,
        capabilities,
        scopeKey,
        account: view.account,
        balance: assets.balance,
        subscription: assets.subscription,
        payments,
        error: view.operation_error ?? null,
        assetSyncError: null,
        assetRequestGeneration: nextAssetRequestGeneration(),
      };
      cacheAccountState(next, stableSnapshot);
      return next;
    });
  },
  applyPaymentOperation: (view) => {
    set((state) => {
      if (state.account?.status !== "authenticated" || view.account.status !== "authenticated") return state;
      const scopeKey = cloudScopeKey(view.account);
      if (scopeKey !== state.scopeKey || scopeKey !== cloudScopeKey(state.account)) return state;
      const index = state.payments.findIndex((payment) => payment.order_id === view.payment.order_id);
      let payments: CloudPaymentView[];
      if (index < 0) {
        payments = [...state.payments, view.payment];
      } else {
        const merged = mergePayment(state.payments[index], view.payment);
        if (!merged.accepted) return state;
        payments = state.payments.map((payment, paymentIndex) => (paymentIndex === index ? merged.payment : payment));
      }
      const entitlementGranted = view.payment.state === "payment_succeeded" && view.payment.entitlement_granted;
      const assets = entitlementGranted ? mergeAccountAssets(state, view) : state;
      const next = {
        ...state,
        account: view.account,
        balance: assets.balance,
        subscription: assets.subscription,
        payments,
        error: null,
        assetSyncError: entitlementGranted ? null : state.assetSyncError,
        assetRequestGeneration: entitlementGranted
          ? nextAssetRequestGeneration()
          : state.assetRequestGeneration,
      };
      cacheAccountState(next);
      return next;
    });
  },
  beginAssetRequest: (contextKey) => {
    const state = get();
    if (
      state.contextKey !== contextKey
      || state.account === null
      || !hasRecoverableCloudIdentity(state.account)
      || state.scopeKey !== cloudScopeKey(state.account)
    ) {
      return null;
    }
    const request = {
      contextKey,
      scopeKey: state.scopeKey,
      generation: nextAssetRequestGeneration(),
    };
    set({ assetRequestGeneration: request.generation });
    return request;
  },
  applyCachedAssets: (view, request) => {
    let accepted = false;
    set((state) => {
      if (
        state.contextKey !== request.contextKey
        || state.scopeKey !== request.scopeKey
        || state.assetRequestGeneration !== request.generation
        || state.account === null
        || !hasRecoverableCloudIdentity(state.account)
        || cloudScopeKey(state.account) !== request.scopeKey
        || cachedAssetsScopeKey(view) !== request.scopeKey
      ) {
        return state;
      }
      accepted = true;
      const assets = mergeAccountAssets(state, view);
      if (
        assets.balance === state.balance
        && assets.subscription === state.subscription
        && state.assetSyncError === null
      ) {
        return state;
      }
      const next = { ...state, ...assets, assetSyncError: null };
      cacheAccountState(next);
      return next;
    });
    return accepted;
  },
  recordAssetSyncFailure: (request, message) => {
    set((state) => {
      if (
        state.contextKey !== request.contextKey
        || state.scopeKey !== request.scopeKey
        || state.assetRequestGeneration !== request.generation
      ) {
        return state;
      }
      return { ...state, assetSyncError: message };
    });
  },
  finishBootstrap: () => set({ initialized: true, busy: false }),
  setBusy: (busy) => set({ busy }),
  setError: (error) => set({ error }),
  reset: () => {
    accountSnapshots.clear();
    set(emptyAccountState());
  },
}));
