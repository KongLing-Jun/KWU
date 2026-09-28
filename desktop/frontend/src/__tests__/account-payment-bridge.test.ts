// Run: tsx src/__tests__/account-payment-bridge.test.ts

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
const bridge = readFileSync(resolve(here, "../lib/bridge.ts"), "utf8");

console.log("\naccount payment bridge contract");

ok(
  /import type \{[^}]*CloudPaymentOperationView[^}]*CloudPaymentView[^}]*\} from "\.\.\/store\/account";/s.test(bridge),
  "bridge imports local payment operation and channel types",
);

const signatures: Array<[string, RegExp]> = [
  ["CloudCreateSubscriptionOrder", /CloudCreateSubscriptionOrder\(plan: string, channel: CloudPaymentView\["payment_channel"\]\): Promise<CloudPaymentOperationView>;/],
  ["CloudCreateUpgradeOrder", /CloudCreateUpgradeOrder\(targetPlan: string, channel: CloudPaymentView\["payment_channel"\]\): Promise<CloudPaymentOperationView>;/],
  ["CloudConfirmPayment", /CloudConfirmPayment\(orderID: string\): Promise<CloudPaymentOperationView>;/],
  ["CloudRefreshPayment", /CloudRefreshPayment\(orderID: string\): Promise<CloudPaymentOperationView>;/],
];

for (const [method, signature] of signatures) {
  ok(signature.test(bridge), `${method} has the frozen AppBindings signature`);
}

ok(
  bridge.includes("if (!bound) throw missingWailsBindingError()") &&
    bridge.includes("browser mock data is disabled") &&
    !bridge.includes("getMock()"),
  "payment calls require the live desktop binding instead of a browser fallback",
);

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
