// Run: tsx src/__tests__/composer-model-entitlement.test.ts

import assert from "node:assert/strict";

import {
  resolveComposerModelEntitlementPresentation,
  type ComposerModelEntitlementSnapshot,
} from "../lib/composerModelEntitlement";
import type { SubscriptionAccess } from "../lib/subscriptionModelAccess";

const snapshot: ComposerModelEntitlementSnapshot = {
  accountKey: "account-a",
  routes: [
    { ref: "kwu/deepseek-v4-flash", model: "deepseek-v4-flash", billingSource: "subscription_pool" },
    { ref: "kwu/deepseek-v4-pro", model: "deepseek-v4-pro", billingSource: "subscription_pool" },
    { ref: "custom/local", model: "local", billingSource: "newapi_byok" },
  ],
  marketplace: {
    sources: [{
      id: "international",
      models: [
        { name: "deepseek-v4-flash", tags: ["Subscribe_Free"] },
        { name: "deepseek-v4-pro", tags: ["Subscribe_Pro"] },
      ],
    }],
  },
};
const freeAccess: SubscriptionAccess = { state: "ready", plan: "free", requiredTag: "Subscribe_Free" };
const proAccess: SubscriptionAccess = { state: "ready", plan: "pro", requiredTag: "Subscribe_Pro" };
const resolve = (overrides: Partial<Parameters<typeof resolveComposerModelEntitlementPresentation>[0]> = {}) => (
  resolveComposerModelEntitlementPresentation({
    modelRef: "kwu/deepseek-v4-pro",
    billingSource: "subscription_pool",
    accountKey: "account-a",
    access: proAccess,
    snapshot,
    ...overrides,
  })
);

assert.deepEqual(resolve({ access: { state: "unavailable" } }), { state: "choose" }, "signed-out access chooses from the first target frame");
assert.deepEqual(resolve({ access: freeAccess }), { state: "choose" }, "a downgraded plan cannot display its former Pro model");
assert.deepEqual(resolve(), {
  state: "resolved",
  ref: "kwu/deepseek-v4-pro",
  billingSource: "subscription_pool",
}, "an entitled plan resolves the exact target route");
assert.deepEqual(resolve({ modelRef: "deepseek-v4-flash", access: freeAccess }), {
  state: "resolved",
  ref: "kwu/deepseek-v4-flash",
  billingSource: "subscription_pool",
}, "the universal authenticated default remains available to Free");
assert.deepEqual(resolve({ access: { state: "loading" } }), { state: "pending" }, "unknown account evidence never displays a potentially unavailable model");
assert.deepEqual(resolve({ accountKey: "account-b" }), { state: "pending" }, "an old account snapshot cannot classify the new identity");
assert.deepEqual(resolve({ billingSource: "newapi_byok", modelRef: "custom/local", snapshot: null }), {
  state: "resolved",
  ref: "custom/local",
  billingSource: "newapi_byok",
}, "BYOK presentation stays independent from subscription entitlement");

console.log("composer model entitlement passed");
