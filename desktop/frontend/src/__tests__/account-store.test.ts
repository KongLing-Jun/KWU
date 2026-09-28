// Run: tsx src/__tests__/account-store.test.ts

import {
  emptyAccountState,
  hasRecoverableCloudIdentity,
  requiresReauthentication,
  type CloudBootstrapView,
  type CloudPaymentOperationView,
  type CloudPaymentView,
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
  ok(JSON.stringify(actual) === JSON.stringify(expected), `${label}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
}

function logMixedBootstrapDiagnostic(stage: string) {
  const state = useAccountStore.getState();
  const account = state.account;
  console.log(JSON.stringify({
    diagnostic: "cross-executor-mixed-bootstrap",
    stage,
    account_status: account?.status ?? null,
    error_code: state.error?.code ?? null,
    display_name_present: Boolean(account?.display_name),
    cloud_user_id_present: Boolean(account?.cloud_user_id),
    generation: account?.binding_generation ?? null,
  }));
}

console.log("\naccount store capability defaults");

ok(
  !hasRecoverableCloudIdentity({ status: "anonymous", profile_id: "default", cloud_user_id: "remembered-user" }),
  "an anonymous binding is not treated as a recoverable signed-in identity",
);
ok(
  hasRecoverableCloudIdentity({ status: "authenticated", profile_id: "default", cloud_user_id: "signed-in-user" }),
  "an authenticated account exposes signed-in actions",
);
ok(
  hasRecoverableCloudIdentity({ status: "offline", profile_id: "default", cloud_user_id: "offline-user" }),
  "an offline account with persisted identity remains recoverable",
);
ok(
  !hasRecoverableCloudIdentity({ status: "offline", profile_id: "default" }),
  "an offline account without identity is not recoverable",
);
ok(
  !hasRecoverableCloudIdentity({ status: "expired", profile_id: "default", cloud_user_id: "expired-user" }),
  "an expired account never exposes signed-in actions",
);
ok(
  requiresReauthentication({ status: "expired", profile_id: "default", cloud_user_id: "expired-user" }),
  "only an expired account requires reauthentication",
);
ok(
  !requiresReauthentication({ status: "anonymous", profile_id: "default" }),
  "anonymous remains the ordinary login state",
);

useAccountStore.setState(emptyAccountState());
eq(
  useAccountStore.getState().capabilities,
  { registration: false, registration_mode: "unavailable", payments: false, realtime: false },
  "empty account state fails closed",
);
useAccountStore.getState().applyBootstrap({
  available: true,
  capabilities: { registration: true, registration_mode: "development", payments: false, realtime: false },
  account: { status: "anonymous", profile_id: "development" },
});
eq(useAccountStore.getState().capabilities.registration_mode, "development", "explicit development capability is accepted");

useAccountStore.getState().applyBootstrap({
  available: true,
  capabilities: { registration: true, registration_mode: "real", payments: false, realtime: false },
  account: { status: "anonymous", profile_id: "default" },
});
eq(useAccountStore.getState().capabilities.registration_mode, "real", "explicit real capability is accepted");
useAccountStore.getState().applyBootstrap({
  available: true,
  account: { status: "anonymous", profile_id: "missing-capabilities" },
});
eq(
  useAccountStore.getState().capabilities,
  { registration: false, registration_mode: "unavailable", payments: false, realtime: false },
  "missing bootstrap capabilities fail closed instead of retaining an earlier mode",
);

console.log("\naccount store cross-executor mixed authenticated baseline");

const mixedAuthenticatedBaseline: CloudBootstrapView = {
  available: true,
  capabilities: { registration: true, registration_mode: "development", payments: false, realtime: false },
  account: {
    status: "authenticated",
    profile_id: "cross-executor",
    cloud_user_id: "usr_cross_executor",
    binding_generation: 7,
  },
  operation_error: {
    code: "REFRESH_TOKEN_REUSED",
    message: "cloud session expired; sign in again",
    request_id: "req-cross-refresh-reused",
  },
};
useAccountStore.getState().reset();
logMixedBootstrapDiagnostic("before_applyBootstrap");
useAccountStore.getState().applyBootstrap(mixedAuthenticatedBaseline);
logMixedBootstrapDiagnostic("after_applyBootstrap");
eq(useAccountStore.getState().account?.status, "authenticated", "mixed refresh failure keeps the account authenticated in the baseline contract");
eq(useAccountStore.getState().error?.code, "REFRESH_TOKEN_REUSED", "mixed refresh failure stores the operation error code");
eq(useAccountStore.getState().error?.request_id, "req-cross-refresh-reused", "mixed refresh failure stores the operation request ID");
ok(useAccountStore.getState().account?.display_name === undefined, "restart-shaped mixed state has no display name");
ok(!requiresReauthentication(useAccountStore.getState().account), "authenticated mixed state does not require reauthentication");

useAccountStore.getState().applyBootstrap({
  ...mixedAuthenticatedBaseline,
  account: {
    ...mixedAuthenticatedBaseline.account,
    status: "expired",
  },
  balance: { total_balance: "stale-balance" },
  subscription: {
    plans: [],
    current: { plan: "free", status: "free" },
  },
});
eq(useAccountStore.getState().account?.status, "expired", "current AUTH_REQUIRED transition enters the expired account state");
eq(useAccountStore.getState().balance, null, "expired transition clears stale balance assets");
eq(useAccountStore.getState().subscription, null, "expired transition clears stale subscription assets");
eq(useAccountStore.getState().payments, [], "expired transition clears stale payment assets");
ok(requiresReauthentication(useAccountStore.getState().account), "expired transition requires reauthentication");

function bootstrap(
  profileID: string,
  cloudUserID: string,
  generation: number,
  totalBalance: string,
  plan: string,
  payments: CloudPaymentView[] = [],
): CloudBootstrapView {
  return {
    available: true,
    account: {
      status: "authenticated",
      profile_id: profileID,
      cloud_user_id: cloudUserID,
      binding_generation: generation,
      display_name: `User ${cloudUserID}`,
      last_sync_at: "2026-07-11T08:00:00Z",
    },
    balance: { total_balance: totalBalance },
    subscription: {
      plans: [
        {
          plan,
          display_name: plan.toUpperCase(),
          price_amount: "10.00",
          currency: "CNY",
          credit_amount: "100.00",
          period_days: 30,
          enabled: true,
          sort_order: 1,
          config_version: "catalog-v1",
        },
      ],
      current: {
        plan,
        status: "active",
        updated_at: "2026-07-11T08:00:00Z",
      },
    },
    payments,
  };
}

function payment(orderID: string, state: CloudPaymentView["state"], entitlementGranted = false): CloudPaymentView {
  return {
    order_id: orderID,
    order_type: "subscription_purchase",
    plan: "pro",
    amount: "59.90",
    amount_unit: "yuan",
    currency: "CNY",
    payment_channel: "alipay",
    state,
    payable: state === "paying",
    entitlement_granted: entitlementGranted,
    expires_at: "2030-01-01T00:30:00Z",
    payment_entry: state === "paying"
      ? { entry_type: "qr_code", qr_code: "https://qr.alipay.test/pro", expires_at: "2030-01-01T00:30:00Z" }
      : null,
  };
}

console.log("\naccount store scope replacement");

useAccountStore.setState(emptyAccountState());
const accountA = bootstrap("profile-a", "user-a", 1, "88.00", "starter", [payment("order-a", "paying")]);
useAccountStore.getState().applyBootstrap(accountA);
useAccountStore.getState().setError({ code: "STALE_A", message: "old scope error" });

const accountB = bootstrap("profile-a", "user-b", 2, "188.00", "pro", [payment("order-b", "paying")]);
const scopeChangeEmissions: ReturnType<typeof useAccountStore.getState>[] = [];
const unsubscribeScope = useAccountStore.subscribe((state) => {
  scopeChangeEmissions.push(state);
});
useAccountStore.getState().applyBootstrap(accountB);
unsubscribeScope();

eq(scopeChangeEmissions.length, 1, "scope replacement publishes one atomic state");
const switched = useAccountStore.getState();
eq(switched.scopeKey, "profile-a\u0000user-b\u00002", "scope key follows the authoritative binding");
eq(switched.balance?.total_balance, "188.00", "new scope receives only its own balance");
eq(switched.subscription?.current.plan, "pro", "new scope receives only its own subscription");
eq(switched.payments.map((item) => item.order_id), ["order-b"], "new scope receives only its own payments");
eq(switched.error, null, "scope replacement clears the previous scope error");
ok(
  scopeChangeEmissions.every((state) => state.balance?.total_balance !== "88.00" && state.subscription?.current.plan !== "starter" && !state.payments.some((item) => item.order_id === "order-a")),
  "no emitted state mixes the new scope with old scoped assets or payments",
);

console.log("\naccount store expired fail-closed projection");

useAccountStore.getState().activateContext("expired-fail-closed");
useAccountStore.getState().applyBootstrap(accountB);
useAccountStore.getState().applyBootstrap({
  available: true,
  account: {
    status: "expired",
    profile_id: "profile-a",
    binding_generation: 0,
  },
  balance: { total_balance: "leaked-balance" },
  subscription: accountA.subscription,
  payments: [payment("leaked-payment", "paying")],
});
eq(useAccountStore.getState().account?.status, "expired", "lower-generation expired bootstrap bypasses the stale guard");
eq(useAccountStore.getState().balance, null, "lower-generation expired bootstrap clears balance");
eq(useAccountStore.getState().subscription, null, "lower-generation expired bootstrap clears subscription");
eq(useAccountStore.getState().payments, [], "lower-generation expired bootstrap clears payments");
useAccountStore.getState().activateContext("another-context");
useAccountStore.getState().activateContext("expired-fail-closed");
eq(useAccountStore.getState().account, null, "expired transition clears the recoverable in-memory context snapshot");

useAccountStore.getState().applyBootstrap(accountB);
useAccountStore.getState().applyBootstrap({
  available: true,
  account: {
    status: "expired",
    profile_id: "profile-a",
    cloud_user_id: "changed-or-sanitized-user",
    binding_generation: 2,
  },
  balance: { total_balance: "leaked-balance" },
  subscription: accountA.subscription,
  payments: [payment("leaked-payment", "paying")],
});
eq(useAccountStore.getState().account?.status, "expired", "same-generation expired identity drift bypasses the stale guard");
eq(useAccountStore.getState().balance, null, "same-generation expired identity drift clears balance");
eq(useAccountStore.getState().subscription, null, "same-generation expired identity drift clears subscription");
eq(useAccountStore.getState().payments, [], "same-generation expired identity drift clears payments");

useAccountStore.setState(emptyAccountState());
useAccountStore.getState().applyBootstrap(accountB);

console.log("\naccount store same-scope refresh");

const refreshedB = bootstrap("profile-a", "user-b", 2, "199.00", "pro", [payment("order-b", "paying")]);
const sameScopeEmissions: ReturnType<typeof useAccountStore.getState>[] = [];
const unsubscribeRefresh = useAccountStore.subscribe((state) => {
  sameScopeEmissions.push(state);
});
useAccountStore.getState().applyBootstrap(refreshedB);
unsubscribeRefresh();

eq(sameScopeEmissions.length, 1, "same-scope refresh publishes one state");
eq(useAccountStore.getState().balance?.total_balance, "199.00", "same-scope refresh replaces the balance");
ok(sameScopeEmissions.every((state) => state.balance !== null), "same-scope refresh never flashes an empty balance");

console.log("\naccount store scoped payment operation merge");

const confirming = payment("order-b", "payment_confirming");
useAccountStore.getState().applyPaymentOperation({
  account: accountB.account,
  payment: confirming,
  balance: { total_balance: "999999.00" },
  subscription: accountA.subscription,
});
eq(useAccountStore.getState().payments, [confirming], "confirming operation still merges its payment snapshot");
eq(useAccountStore.getState().balance?.total_balance, "199.00", "confirming operation cannot merge an ungranted balance");
eq(useAccountStore.getState().subscription?.current.plan, "pro", "confirming operation cannot merge an ungranted subscription");

const granted = payment("order-b", "payment_succeeded", true);
const operation: CloudPaymentOperationView = {
  account: accountB.account,
  payment: granted,
  balance: { total_balance: "259.00" },
  subscription: accountB.subscription,
};
useAccountStore.getState().applyPaymentOperation(operation);
eq(useAccountStore.getState().payments, [granted], "same-scope operation replaces its order snapshot");
eq(useAccountStore.getState().balance?.total_balance, "259.00", "granted operation merges authoritative balance");

const beforeStaleOperation = JSON.stringify(useAccountStore.getState());
useAccountStore.getState().applyPaymentOperation({
  ...operation,
  account: { ...operation.account, binding_generation: 1 },
  payment: payment("stale-generation", "payment_succeeded", true),
  balance: { total_balance: "1.00" },
});
eq(JSON.stringify(useAccountStore.getState()), beforeStaleOperation, "old-generation payment operation cannot write any scoped state");

useAccountStore.getState().applyPaymentOperation({
  ...operation,
  account: accountA.account,
  payment: payment("old-scope", "payment_failed"),
});
eq(JSON.stringify(useAccountStore.getState()), beforeStaleOperation, "old-account payment operation cannot write any scoped state");

console.log("\naccount store generation rollback guard");

useAccountStore.getState().setError({ code: "CURRENT_B_ERROR", message: "current B scope error" });
const stableBState = useAccountStore.getState();
const beforeLateA = JSON.stringify(useAccountStore.getState());
const zeroGenerationCases: Array<{ label: string; view: CloudBootstrapView }> = [
  {
    label: "missing generation anonymous",
    view: {
      available: true,
      account: { status: "anonymous", profile_id: "profile-a" },
      operation_error: { code: "LATE_MISSING_GENERATION", message: "missing generation" },
    },
  },
  {
    label: "explicit generation zero offline",
    view: {
      available: true,
      account: {
        status: "offline",
        profile_id: "profile-a",
        cloud_user_id: "user-a",
        binding_generation: 0,
      },
      operation_error: { code: "LATE_ZERO_GENERATION", message: "zero generation" },
    },
  },
];
for (const testCase of zeroGenerationCases) {
  useAccountStore.setState(stableBState);
  useAccountStore.getState().applyBootstrap(testCase.view);
  eq(
    JSON.stringify(useAccountStore.getState()),
    beforeLateA,
    `${testCase.label} cannot replace B/gen2 account, assets, orders, or error`,
  );
}
useAccountStore.setState(stableBState);
for (const status of ["anonymous", "offline", "authenticated"] as const) {
  const lateA = bootstrap("profile-a", "user-a", 1, "1.00", "starter", [payment(`late-a-${status}`, "payment_failed")]);
  lateA.account.status = status;
  lateA.operation_error = { code: `LATE_A_${status.toUpperCase()}`, message: "late A response" };
  useAccountStore.getState().applyBootstrap(lateA);
  eq(
    JSON.stringify(useAccountStore.getState()),
    beforeLateA,
    `late A/gen1 ${status} response cannot replace B/gen2 account, assets, orders, or error`,
  );
}
const sameGenerationOtherUser = bootstrap("profile-a", "user-a", 2, "2.00", "starter", [payment("same-generation-other-user", "payment_failed")]);
sameGenerationOtherUser.operation_error = { code: "SAME_GENERATION_OTHER_USER", message: "identity drift" };
useAccountStore.getState().applyBootstrap(sameGenerationOtherUser);
eq(
  JSON.stringify(useAccountStore.getState()),
  beforeLateA,
  "same positive generation with a different cloud user cannot replace B account, assets, orders, or error",
);

useAccountStore.getState().applyBootstrap({
  available: true,
  account: { ...accountB.account, status: "anonymous" },
});
eq(useAccountStore.getState().account?.status, "anonymous", "authoritative logout retains the current binding identity");
eq(useAccountStore.getState().account?.cloud_user_id, "user-b", "logout DTO retains the current cloud user");
eq(useAccountStore.getState().account?.binding_generation, 2, "logout DTO retains the current binding generation");
eq(useAccountStore.getState().balance, null, "anonymous logout clears the authenticated balance");
eq(useAccountStore.getState().subscription, null, "anonymous logout clears the authenticated subscription");
eq(useAccountStore.getState().payments, [], "anonymous logout clears authenticated payments");

console.log("\naccount store same-scope payment coverage");

useAccountStore.setState(emptyAccountState());
const newest = bootstrap("profile-monotonic", "user-monotonic", 4, "900.00", "pro", [payment("order-granted", "payment_succeeded", true)]);
newest.account.last_sync_at = "2026-07-12T10:00:00Z";
newest.operation_error = { code: "LATEST_ERROR", message: "latest snapshot error" };
useAccountStore.getState().applyBootstrap(newest);

const stalePending = bootstrap(
  "profile-monotonic",
  "user-monotonic",
  4,
  "100.00",
  "starter",
  [payment("order-granted", "paying"), payment("order-new", "paying")],
);
stalePending.account.last_sync_at = "2026-07-12T09:00:00Z";
stalePending.operation_error = { code: "STALE_ERROR", message: "stale snapshot error" };
useAccountStore.getState().applyBootstrap(stalePending);

eq(useAccountStore.getState().account?.last_sync_at, "2026-07-12T10:00:00Z", "regressive payment coverage cannot replace the current account snapshot");
eq(useAccountStore.getState().balance?.total_balance, "900.00", "regressive payment coverage cannot replace balance");
eq(useAccountStore.getState().subscription?.current.plan, "pro", "regressive payment coverage cannot replace subscription");
eq(useAccountStore.getState().error?.code, "STALE_ERROR", "regressive payment coverage still publishes the incoming operation error");
eq(
  useAccountStore.getState().payments.map((item) => [item.order_id, item.state, item.entitlement_granted]),
  [["order-granted", "payment_succeeded", true], ["order-new", "paying", false]],
  "regressive payment coverage preserves terminal progress while unioning a new order",
);

const staleEmpty = bootstrap("profile-monotonic", "user-monotonic", 4, "1.00", "starter", []);
staleEmpty.account.last_sync_at = "2026-07-12T08:00:00Z";
staleEmpty.operation_error = { code: "OLDER_ERROR", message: "even older snapshot error" };
useAccountStore.getState().applyBootstrap(staleEmpty);
eq(
  useAccountStore.getState().payments.map((item) => item.order_id),
  ["order-granted", "order-new"],
  "same-scope bootstrap with an empty payment list never deletes L1 orders",
);
eq(useAccountStore.getState().balance?.total_balance, "900.00", "bootstrap missing current orders cannot replace assets");

const coveredEarlier = bootstrap(
  "profile-monotonic",
  "user-monotonic",
  4,
  "950.00",
  "pro",
  [payment("order-granted", "payment_succeeded", true), payment("order-new", "payment_confirming")],
);
coveredEarlier.account.last_sync_at = "2026-07-12T07:00:00Z";
coveredEarlier.operation_error = { code: "COVERED_ERROR", message: "covered snapshot error" };
useAccountStore.getState().applyBootstrap(coveredEarlier);
eq(useAccountStore.getState().account?.last_sync_at, "2026-07-12T07:00:00Z", "complete forward payment coverage accepts an earlier account timestamp");
eq(useAccountStore.getState().balance?.total_balance, "950.00", "complete forward payment coverage replaces assets regardless of timestamp order");
eq(useAccountStore.getState().error?.code, "COVERED_ERROR", "complete payment coverage accepts the incoming error envelope");
eq(
  useAccountStore.getState().payments.map((item) => [item.order_id, item.state]),
  [["order-granted", "payment_succeeded"], ["order-new", "payment_confirming"]],
  "complete payment coverage advances every current order",
);

const identityDrift = bootstrap(
  "profile-monotonic",
  "user-monotonic",
  4,
  "2.00",
  "starter",
  [
    payment("order-granted", "payment_succeeded", true),
    { ...payment("order-new", "payment_confirming"), amount: "0.01" },
    payment("order-safe-new", "paying"),
  ],
);
identityDrift.account.last_sync_at = "2026-07-12T11:00:00Z";
identityDrift.operation_error = { code: "DRIFT_ERROR", message: "identity drift snapshot" };
useAccountStore.getState().applyBootstrap(identityDrift);
eq(useAccountStore.getState().balance?.total_balance, "950.00", "identity drift in any current order blocks account and asset replacement");
eq(useAccountStore.getState().error?.code, "DRIFT_ERROR", "identity drift still publishes the incoming operation error");
eq(
  useAccountStore.getState().payments.map((item) => [item.order_id, item.amount, item.state]),
  [
    ["order-granted", "59.90", "payment_succeeded"],
    ["order-new", "59.90", "payment_confirming"],
    ["order-safe-new", "59.90", "paying"],
  ],
  "identity drift is rejected while a safe new order still unions into the scope",
);

console.log("\naccount store payment progress rejection");

const monotonicAccount = useAccountStore.getState().account!;
const failedOrder = payment("order-terminal", "payment_failed");
useAccountStore.getState().applyPaymentOperation({ account: monotonicAccount, payment: failedOrder });
eq(
  useAccountStore.getState().payments.find((item) => item.order_id === "order-terminal")?.state,
  "payment_failed",
  "new terminal order is added to the current scope",
);

const balanceBeforeRejectedGrant = useAccountStore.getState().balance?.total_balance;
useAccountStore.getState().applyPaymentOperation({
  account: monotonicAccount,
  payment: payment("order-terminal", "payment_succeeded", true),
  balance: { total_balance: "999999.00" },
  subscription: stalePending.subscription,
});
eq(
  useAccountStore.getState().payments.find((item) => item.order_id === "order-terminal"),
  failedOrder,
  "one terminal outcome cannot be replaced by a different terminal outcome",
);
eq(useAccountStore.getState().balance?.total_balance, balanceBeforeRejectedGrant, "rejected fake succeeded result cannot update balance");
eq(useAccountStore.getState().subscription?.current.plan, "pro", "rejected fake succeeded result cannot update subscription");

const identityOrder = payment("order-identity", "paying");
useAccountStore.getState().applyPaymentOperation({ account: monotonicAccount, payment: identityOrder });
useAccountStore.getState().applyPaymentOperation({
  account: monotonicAccount,
  payment: { ...payment("order-identity", "payment_succeeded", true), amount: "0.01" },
  balance: { total_balance: "0.01" },
  subscription: stalePending.subscription,
});
eq(
  useAccountStore.getState().payments.find((item) => item.order_id === "order-identity"),
  identityOrder,
  "same order id with changed immutable identity is rejected",
);
eq(useAccountStore.getState().balance?.total_balance, balanceBeforeRejectedGrant, "identity-rejected succeeded result cannot update assets");

const confirmingOrder = payment("order-progress", "payment_confirming");
useAccountStore.getState().applyPaymentOperation({ account: monotonicAccount, payment: confirmingOrder });
useAccountStore.getState().applyPaymentOperation({ account: monotonicAccount, payment: payment("order-progress", "paying") });
eq(
  useAccountStore.getState().payments.find((item) => item.order_id === "order-progress")?.state,
  "payment_confirming",
  "payment operation cannot move UI progress backward",
);

const awaitingEntitlement = payment("order-awaiting-entitlement", "payment_succeeded", false);
useAccountStore.getState().applyPaymentOperation({ account: monotonicAccount, payment: awaitingEntitlement });
const grantedAfterAwait = payment("order-awaiting-entitlement", "payment_succeeded", true);
useAccountStore.getState().applyPaymentOperation({
  account: monotonicAccount,
  payment: grantedAfterAwait,
  balance: { total_balance: "960.00" },
  subscription: newest.subscription,
});
eq(
  useAccountStore.getState().payments.find((item) => item.order_id === "order-awaiting-entitlement"),
  grantedAfterAwait,
  "succeeded without entitlement remains non-terminal and can advance to granted",
);
eq(useAccountStore.getState().balance?.total_balance, "960.00", "accepted granted transition updates authoritative assets");

console.log("\ncanonical granted snapshots ignore wall-clock ordering");

useAccountStore.setState(emptyAccountState());
const userOnePending = bootstrap(
  "default",
  "usr_01",
  1,
  "81000.0000",
  "plus",
  [payment("usr01-plus-pro", "payment_confirming")],
);
userOnePending.account.last_sync_at = "2026-07-12T04:01:00Z";
useAccountStore.getState().applyBootstrap(userOnePending);
const userOneGrantedAccount = { ...userOnePending.account, last_sync_at: "2026-07-12T01:06:00Z" };
const userOneProAssets = bootstrap("default", "usr_01", 1, "141000.0000", "pro");
useAccountStore.getState().applyPaymentOperation({
  account: userOneGrantedAccount,
  payment: payment("usr01-plus-pro", "payment_succeeded", true),
  balance: userOneProAssets.balance,
  subscription: userOneProAssets.subscription,
});
eq(useAccountStore.getState().account?.last_sync_at, "2026-07-12T01:06:00Z", "usr01 Plus-to-Pro accepts forward grant with an earlier wall-clock timestamp");
eq(useAccountStore.getState().balance?.total_balance, "141000.0000", "usr01 Plus-to-Pro granted balance is accepted");
eq(useAccountStore.getState().subscription?.current.plan, "pro", "usr01 Plus-to-Pro granted subscription is accepted");

useAccountStore.setState(emptyAccountState());
const userTwoConfirming = { ...payment("usr02-lite-plus", "payment_confirming"), plan: "plus", amount: "29.90" };
const userTwoPending = bootstrap("default", "usr_02", 1, "20000.0000", "lite", [userTwoConfirming]);
userTwoPending.account.last_sync_at = "2026-07-12T03:01:00Z";
useAccountStore.getState().applyBootstrap(userTwoPending);
const userTwoGranted = { ...payment("usr02-lite-plus", "payment_succeeded", true), plan: "plus", amount: "29.90" };
const userTwoPlusAssets = bootstrap("default", "usr_02", 1, "80000.0000", "plus");
useAccountStore.getState().applyPaymentOperation({
  account: { ...userTwoPending.account, last_sync_at: "2026-07-12T02:01:00Z" },
  payment: userTwoGranted,
  balance: userTwoPlusAssets.balance,
  subscription: userTwoPlusAssets.subscription,
});
eq(useAccountStore.getState().account?.last_sync_at, "2026-07-12T02:01:00Z", "usr02 Lite-to-Plus accepts forward grant with an earlier wall-clock timestamp");
eq(useAccountStore.getState().balance?.total_balance, "80000.0000", "usr02 Lite-to-Plus granted balance is accepted");
eq(useAccountStore.getState().subscription?.current.plan, "plus", "usr02 Lite-to-Plus granted subscription is accepted");

console.log("\naccount store context cache");

useAccountStore.getState().reset();
useAccountStore.getState().activateContext("tab-cache");
ok(!useAccountStore.getState().initialized, "uncached context starts uninitialized");

const cachedAccount = bootstrap("profile-cache", "user-cache", 3, "500.00", "pro", [payment("cache-order", "paying")]);
useAccountStore.getState().applyBootstrap(cachedAccount);
useAccountStore.getState().setBusy(true);
useAccountStore.getState().setError({ code: "TRANSIENT", message: "transient error" });
useAccountStore.getState().activateContext("tab-other");
ok(useAccountStore.getState().account === null, "uncached context does not expose the previous account");
ok(!useAccountStore.getState().initialized, "uncached context remains pending bootstrap");

useAccountStore.getState().activateContext("tab-cache");
ok(useAccountStore.getState().initialized, "cached context restores as initialized");
eq(useAccountStore.getState().balance?.total_balance, "500.00", "cached context restores account assets");
eq(useAccountStore.getState().payments.map((item) => item.order_id), ["cache-order"], "cached context restores payments");
ok(!useAccountStore.getState().busy && useAccountStore.getState().error === null, "cached context excludes transient request state");

useAccountStore.getState().applyBootstrap(bootstrap("profile-cache", "user-cache", 2, "1.00", "starter"));
useAccountStore.getState().activateContext("tab-other");
useAccountStore.getState().activateContext("tab-cache");
eq(useAccountStore.getState().balance?.total_balance, "500.00", "rejected older generation does not poison the cache");

const cachedGrant = payment("cache-order", "payment_succeeded", true);
const cachedGrantAssets = bootstrap("profile-cache", "user-cache", 3, "650.00", "pro");
useAccountStore.getState().applyPaymentOperation({
  account: cachedAccount.account,
  payment: cachedGrant,
  balance: cachedGrantAssets.balance,
  subscription: cachedGrantAssets.subscription,
});
useAccountStore.getState().activateContext("tab-other");
useAccountStore.getState().activateContext("tab-cache");
eq(useAccountStore.getState().balance?.total_balance, "650.00", "payment operation updates the cached assets");
eq(useAccountStore.getState().payments[0], cachedGrant, "payment operation updates the cached payment");

const partialError = { code: "NETWORK_UNAVAILABLE", message: "cloud network is unavailable" };
useAccountStore.getState().applyBootstrap({
  available: true,
  account: { ...cachedAccount.account, status: "offline" },
  operation_error: partialError,
  payments: [],
});
ok(useAccountStore.getState().balance === null, "failed authoritative refresh clears the last balance");
ok(useAccountStore.getState().subscription === null, "failed authoritative refresh clears the last subscription");
eq(useAccountStore.getState().payments[0], cachedGrant, "partial refresh preserves terminal payment progress");
eq(useAccountStore.getState().error, partialError, "partial refresh publishes its operation error");
useAccountStore.getState().activateContext("tab-other");
useAccountStore.getState().activateContext("tab-cache");
ok(!useAccountStore.getState().initialized && useAccountStore.getState().account === null, "failed authoritative refresh removes the stable account snapshot");

useAccountStore.getState().activateContext("tab-partial");
const firstPartial = bootstrap("profile-partial", "user-partial", 1, "42.00", "starter");
useAccountStore.getState().applyBootstrap({
  available: true,
  account: firstPartial.account,
  balance: firstPartial.balance,
  operation_error: partialError,
});
eq(useAccountStore.getState().balance?.total_balance, "42.00", "first partial response exposes its available balance live");
eq(useAccountStore.getState().error, partialError, "first partial response exposes its error live");
useAccountStore.getState().activateContext("tab-other");
useAccountStore.getState().activateContext("tab-partial");
ok(!useAccountStore.getState().initialized && useAccountStore.getState().account === null, "first partial response does not create an empty asset cache");

useAccountStore.getState().activateContext("tab-cache");
const switchedPartial = bootstrap("profile-cache", "user-next", 4, "1.00", "starter");
useAccountStore.getState().applyBootstrap({
  available: true,
  account: switchedPartial.account,
  operation_error: partialError,
});
eq(useAccountStore.getState().account?.cloud_user_id, "user-next", "partial account switch exposes only the new identity live");
ok(useAccountStore.getState().balance === null && useAccountStore.getState().subscription === null, "partial account switch never borrows old account assets");
useAccountStore.getState().activateContext("tab-other");
useAccountStore.getState().activateContext("tab-cache");
ok(!useAccountStore.getState().initialized && useAccountStore.getState().account === null, "partial account switch clears the previous identity cache");

useAccountStore.getState().applyBootstrap(cachedAccount);

useAccountStore.getState().applyBootstrap({
  available: true,
  account: { ...cachedAccount.account, status: "anonymous" },
});
useAccountStore.getState().activateContext("tab-other");
useAccountStore.getState().activateContext("tab-cache");
eq(useAccountStore.getState().account?.status, "anonymous", "logout replaces the authenticated cache");
ok(useAccountStore.getState().balance === null && useAccountStore.getState().payments.length === 0, "logout cache excludes authenticated assets");

useAccountStore.getState().reset();
useAccountStore.getState().activateContext("tab-cache");
ok(!useAccountStore.getState().initialized && useAccountStore.getState().account === null, "reset clears all context snapshots");

console.log("\naccount store versioned asset convergence");

useAccountStore.getState().reset();
useAccountStore.getState().activateContext("asset-context");
const versioned = bootstrap("profile-assets", "user-assets", 7, "100.00", "pro");
versioned.balance = { ...versioned.balance!, version: 5 };
versioned.subscription = {
  ...versioned.subscription!,
  current: { ...versioned.subscription!.current, version: 8 },
};
useAccountStore.getState().applyBootstrap(versioned);

const interleavedRefresh = bootstrap("profile-assets", "user-assets", 7, "10.00", "plus");
interleavedRefresh.balance = { ...interleavedRefresh.balance!, version: 4 };
interleavedRefresh.subscription = {
  ...interleavedRefresh.subscription!,
  current: { ...interleavedRefresh.subscription!.current, version: 9 },
};
useAccountStore.getState().applyBootstrap(interleavedRefresh);
eq(useAccountStore.getState().balance?.total_balance, "100.00", "lower balance version cannot overwrite a newer bootstrap asset");
eq(useAccountStore.getState().subscription?.current.plan, "plus", "subscription advances independently of an older balance");

const terminalVersionedPayment = payment("versioned-terminal", "payment_succeeded", true);
useAccountStore.getState().applyPaymentOperation({
  account: versioned.account,
  payment: terminalVersionedPayment,
});
const uncoveredVersionedRefresh = bootstrap(
  "profile-assets",
  "user-assets",
  7,
  "125.00",
  "starter",
  [payment("versioned-terminal", "paying")],
);
uncoveredVersionedRefresh.balance = { ...uncoveredVersionedRefresh.balance!, version: 6 };
uncoveredVersionedRefresh.subscription = {
  ...uncoveredVersionedRefresh.subscription!,
  current: { ...uncoveredVersionedRefresh.subscription!.current, version: 8 },
};
useAccountStore.getState().applyBootstrap(uncoveredVersionedRefresh);
eq(
  useAccountStore.getState().payments.find((item) => item.order_id === "versioned-terminal"),
  terminalVersionedPayment,
  "regressive payment coverage still preserves terminal payment progress",
);
eq(
  useAccountStore.getState().balance?.total_balance,
  "125.00",
  "payment coverage failure does not block a higher balance version",
);
eq(
  useAccountStore.getState().subscription?.current.plan,
  "plus",
  "payment coverage failure cannot replace a lower subscription version",
);

const uncoveredUnversionedRefresh = bootstrap(
  "profile-assets",
  "user-assets",
  7,
  "unversioned-balance",
  "unversioned-plan",
  [payment("versioned-terminal", "paying")],
);
useAccountStore.getState().applyBootstrap(uncoveredUnversionedRefresh);
eq(
  useAccountStore.getState().balance?.total_balance,
  "125.00",
  "payment coverage failure keeps missing-version balance behind payment safety",
);
eq(
  useAccountStore.getState().subscription?.current.plan,
  "plus",
  "payment coverage failure keeps missing-version subscription behind payment safety",
);

const versionedGrant = payment("asset-grant", "payment_succeeded", true);
useAccountStore.getState().applyPaymentOperation({
  account: versioned.account,
  payment: versionedGrant,
  balance: { total_balance: "150.00", version: 7 },
  subscription: {
    ...versioned.subscription!,
    current: { ...versioned.subscription!.current, plan: "starter", version: 8 },
  },
});
eq(useAccountStore.getState().balance?.total_balance, "150.00", "granted payment shares the monotonic balance merge");
eq(useAccountStore.getState().subscription?.current.plan, "plus", "granted payment cannot regress subscription independently");

const eventRequest = useAccountStore.getState().beginAssetRequest("asset-context");
ok(eventRequest !== null, "current complete scope can begin an event cache request");
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-assets",
  cloud_user_id: "user-assets",
  binding_generation: 7,
  balance: { total_balance: "175.00", version: 8 },
  subscription: {
    ...versioned.subscription!,
    current: { ...versioned.subscription!.current, plan: "free", status: "free", version: 10 },
  },
}, eventRequest!);
eq(useAccountStore.getState().balance?.total_balance, "175.00", "event cache advances the balance through the shared merge");
eq(useAccountStore.getState().subscription?.current.plan, "free", "event cache advances the subscription through the shared merge");

const conflictRequest = useAccountStore.getState().beginAssetRequest("asset-context")!;
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-assets",
  cloud_user_id: "user-assets",
  binding_generation: 7,
  balance: { total_balance: "conflict", version: 8 },
  subscription: {
    ...versioned.subscription!,
    current: { ...versioned.subscription!.current, plan: "conflict", version: 10 },
  },
}, conflictRequest);
eq(useAccountStore.getState().balance?.total_balance, "175.00", "same-version conflicting balance fails closed");
eq(useAccountStore.getState().subscription?.current.plan, "free", "same-version conflicting subscription fails closed");

const zeroRequest = useAccountStore.getState().beginAssetRequest("asset-context")!;
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-assets",
  cloud_user_id: "user-assets",
  binding_generation: 7,
  balance: { total_balance: "zero-version" },
  subscription: {
    ...versioned.subscription!,
    current: { ...versioned.subscription!.current, plan: "zero-version", version: 0 },
  },
}, zeroRequest);
eq(useAccountStore.getState().balance?.total_balance, "175.00", "missing balance version cannot overwrite a positive version");
eq(useAccountStore.getState().subscription?.current.plan, "free", "zero subscription version cannot overwrite a positive version");

const staleRequest = useAccountStore.getState().beginAssetRequest("asset-context")!;
const currentRequest = useAccountStore.getState().beginAssetRequest("asset-context")!;
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-assets",
  cloud_user_id: "user-assets",
  binding_generation: 7,
  balance: { total_balance: "999.00", version: 99 },
}, staleRequest);
eq(useAccountStore.getState().balance?.total_balance, "175.00", "superseded request generation cannot commit even a higher asset version");
useAccountStore.getState().recordAssetSyncFailure(staleRequest, "stale failure");
eq(useAccountStore.getState().assetSyncError, null, "superseded request failure cannot pollute diagnostics");
useAccountStore.getState().recordAssetSyncFailure(currentRequest, "cache unavailable");
eq(useAccountStore.getState().assetSyncError, "cache unavailable", "current request failure is recorded outside login and payment error");
eq(useAccountStore.getState().error, null, "asset read failure does not alter the account operation error");

const oldScopeRequest = currentRequest;
const rebound = bootstrap("profile-assets", "user-rebound", 8, "25.00", "starter");
rebound.balance = { ...rebound.balance!, version: 1 };
rebound.subscription = {
  ...rebound.subscription!,
  current: { ...rebound.subscription!.current, version: 1 },
};
useAccountStore.getState().applyBootstrap(rebound);
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-assets",
  cloud_user_id: "user-assets",
  binding_generation: 7,
  balance: { total_balance: "late-old-scope", version: 100 },
}, oldScopeRequest);
eq(useAccountStore.getState().account?.cloud_user_id, "user-rebound", "binding switch rejects an old cached asset scope");
eq(useAccountStore.getState().balance?.total_balance, "25.00", "old cached asset response cannot overwrite rebound assets");

useAccountStore.getState().reset();
useAccountStore.getState().activateContext("zero-version-context");
const zeroVersionBootstrap = bootstrap("profile-zero", "user-zero", 1, "1.00", "free");
useAccountStore.getState().applyBootstrap(zeroVersionBootstrap);
const supersededZeroRequest = useAccountStore.getState().beginAssetRequest("zero-version-context")!;
const currentZeroRequest = useAccountStore.getState().beginAssetRequest("zero-version-context")!;
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-zero",
  cloud_user_id: "user-zero",
  binding_generation: 1,
  balance: { total_balance: "stale-zero" },
}, supersededZeroRequest);
eq(useAccountStore.getState().balance?.total_balance, "1.00", "superseded zero-version response is rejected by request generation");
useAccountStore.getState().applyCachedAssets({
  profile_id: "profile-zero",
  cloud_user_id: "user-zero",
  binding_generation: 1,
  balance: { total_balance: "current-zero" },
}, currentZeroRequest);
eq(useAccountStore.getState().balance?.total_balance, "current-zero", "current request generation may advance a versionless asset");

useAccountStore.getState().reset();

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
