// Run: tsx src/__tests__/account-workspace-contract.test.ts

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

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

const here = dirname(fileURLToPath(import.meta.url));
const source = (path: string) => readFileSync(resolve(here, path), "utf8");
const appSource = source("../App.tsx");
const workspaceNavigationSource = source("../app/useWorkspaceNavigation.ts");
const componentSource = source("../features/account/ui/AccountCenter.tsx");
const paymentSource = source("../features/account/ui/AccountPayments.tsx");
const rechargeSource = source("../features/account/ui/AccountRecharge.tsx");
const storeSource = source("../features/account/store/accountStore.ts");
const featureEntrySource = source("../features/account/index.ts");
const legacyComponentSource = source("../components/AccountCenter.tsx");
const legacyPaymentSource = source("../components/AccountPayments.tsx");
const legacyStoreSource = source("../store/account.ts");

console.log("\naccount workspace route");

ok(/type WorkspaceView\s*=\s*[^;]*"account"/.test(workspaceNavigationSource), "workspace view includes account");
ok(
  appSource.includes('from "./features/account/controller/useAccountNavigation"')
    && appSource.includes('from "./features/account/controller/useAccountSidebar"')
    && appSource.includes('import("./features/account/ui/AccountCenter")'),
  "app imports account UI and state from the feature boundary",
);
ok(appSource.includes('workspaceView === "account" ?'), "main workspace renders the account center");
ok(
  /const refreshAccountAfterAuthentication = useCallback\(async \(accountScopeKey: string\) => \{\s*ensureModelMarketplaceRefresh\(accountScopeKey, true\);\s*await refreshMeta\(\);/.test(
    appSource,
  ),
  "account authentication starts shared model metadata recovery without blocking its current-tab refresh",
);
ok(
  /<AccountCenter\s+avatarSrc=\{sidebarAccountAvatarSrc\}\s+contextKey=\{DESKTOP_ACCOUNT_CONTEXT_KEY\}\s+modelCatalog=\{sharedModelCatalog\}\s+request=\{accountCenterRequest\}\s+onAuthenticated=\{refreshAccountAfterAuthentication\}\s*\/>/.test(
    appSource,
  )
    && (appSource.match(/onAuthenticated=\{refreshAccountAfterAuthentication\}/g) ?? []).length === 1,
  "account center uses the stable desktop context and forwards authentication refresh through the named callback",
);
ok(
  !componentSource.includes("RefreshModelMarketplace")
    && componentSource.includes("onAuthenticated?.(cloudScopeKey(view.account))")
    && /const refresh = async \(\) => \{[\s\S]*api\.CloudRefreshAssets\(\)[\s\S]*notifyAuthenticated\(epoch, view\)/.test(componentSource),
  "account center reports authenticated asset refreshes without owning a second marketplace path",
);
ok(
  componentSource.includes("<AccountModelShowcase modelCatalog={modelCatalog} />")
    && appSource.includes("modelCatalog={sharedModelCatalog}"),
  "account overview continues to receive and render the current model catalog",
);
ok(
  appSource.includes("useAccountSidebar(DESKTOP_ACCOUNT_CONTEXT_KEY)")
    && !appSource.includes('useAccountSidebar(activeTabId ?? "default")')
    && !appSource.includes('key={activeTabId ?? "default"}'),
  "session switches do not reactivate or remount the global account context",
);
ok(
  /workspaceView === "chat" && \([\s\S]*?<SendAvailabilityBanner[\s\S]*?<HistoryLoadErrorBanner/.test(appSource),
  "chat startup errors are hidden while the account workspace is active",
);
ok((appSource.match(/t\("sidebar\.account"\)/g) ?? []).length >= 3, "all sidebar layouts expose account navigation");
ok(componentSource.includes('import { AccountPayments } from "./AccountPayments"'), "account center imports the payment workspace");
ok(componentSource.includes('import { AccountRecharge } from "./AccountRecharge"'), "account center imports the recharge workspace");
ok(featureEntrySource.includes('from "./ui/AccountCenter"'), "account feature exports its account center");
ok(featureEntrySource.includes('from "./ui/AccountPayments"'), "account feature exports its payment workspace");
ok(featureEntrySource.includes('from "./ui/AccountRecharge"'), "account feature exports its recharge workspace");
ok(featureEntrySource.includes('from "./store/accountStore"'), "account feature exports its store contract");
ok(legacyComponentSource.includes('from "../features/account"'), "legacy account center path remains a compatibility export");
ok(legacyPaymentSource.includes('from "../features/account"'), "legacy payment path remains a compatibility export");
ok(legacyStoreSource.includes('from "../features/account/store/accountStore"'), "legacy account store path remains a compatibility export");
ok(
  /<AccountPayments[\s\S]*?key=\{scopeKey\}[\s\S]*?api=\{api\}[\s\S]*?paymentsEnabled=\{capabilities\.payments\}[\s\S]*?scopeKey=\{scopeKey\}/.test(componentSource),
  "payment workspace is keyed, capability-gated and bound to the active account scope",
);
ok(
  rechargeSource.includes("CloudCreateMaaSRechargeOrder")
    && rechargeSource.includes("CloudGetMaaSRechargeOrder")
    && rechargeSource.includes("CloudGetMaaSRechargeKeyBalance")
    && rechargeSource.includes("data-recharge-balance")
    && !rechargeSource.includes("balance_cny")
    && !rechargeSource.includes("key_name")
    && rechargeSource.includes('order.status === "credited" && order.paid === true && order.credited === true'),
  "recharge workspace creates, polls, and shows only authoritative credit data after paid credited confirmation",
);
ok(
  componentSource.includes('data-account-page-panel="overview"')
    && componentSource.includes('data-account-page="payments"')
    && !componentSource.includes('className="account-center__pages"'),
  "authenticated account center exposes the overview and a real payment entry without a redundant tab strip",
);

console.log("\naccount workspace localization");

for (const locale of ["zh"]) {
  const localeSource = source(`../locales/${locale}.ts`);
  ok(localeSource.includes('"sidebar.account"'), `${locale} localizes account navigation`);
  ok(localeSource.includes('"sidebar.signInOrRegister"'), `${locale} localizes the anonymous account action`);
  ok(localeSource.includes('"account.title"'), `${locale} localizes the account center`);
  ok(localeSource.includes('"account.unavailable"'), `${locale} localizes the unavailable state`);
  for (const key of [
    "account.authenticatedSubtitle",
    "account.cloudUserID",
    "account.cloudStatus",
    "account.lastSync",
    "account.status.authenticated",
    "account.status.offline",
    "account.status.expired",
    "account.subscriptionStatus.active",
    "account.subscriptionStatus.free",
    "account.subscriptionStatus.expired",
    "account.planSummary",
    "account.planManageAvailable",
    "account.planBrowseOnly",
    "account.viewPlans",
    "account.overviewTab",
    "account.paymentsTab",
    "account.pagesLabel",
    "account.payment.plansTitle",
    "account.payment.currentPlan",
    "account.payment.disabledNotice",
    "account.payment.ordersTitle",
    "account.payment.noPlans",
    "account.payment.noOrders",
    "account.payment.price",
    "account.payment.credit",
    "account.payment.period",
    "account.payment.channel.alipay",
    "account.payment.channel.wechat",
    "account.payment.action.purchase",
    "account.payment.action.upgrade",
    "account.payment.action.confirm",
    "account.payment.orderPlan",
    "account.payment.orderAmount",
    "account.payment.amountUnit",
    "account.payment.currency",
    "account.payment.channelLabel",
    "account.payment.statusLabel",
    "account.payment.entryLabel",
    "account.payment.status.orderCreated",
    "account.payment.status.paying",
    "account.payment.status.confirming",
    "account.payment.status.succeeded",
    "account.payment.status.closed",
    "account.payment.status.failed",
    "account.payment.status.timeout",
    "account.payment.status.manualRequired",
  ]) {
    ok(localeSource.includes(`"${key}"`), `${locale} localizes ${key}`);
  }
}
ok(componentSource.includes("useT"), "account center renders through the typed translator");
ok(paymentSource.includes("useI18n"), "payment workspace renders through the typed translator");

console.log("\naccount workspace scope guardrails");

ok(!/password|sms_code|phone|localStorage/i.test(storeSource), "account store contains no credential or local persistence fields");
for (const canonicalConstant of ["9.90", "29.90", "59.90", "20000", "60000", "120000"]) {
  ok(!paymentSource.includes(canonicalConstant), `payment component contains no canonical business constant ${canonicalConstant}`);
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
