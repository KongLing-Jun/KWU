import assert from "node:assert/strict";

import {
  composerSessionIdentityFromSource,
  runtimeComposerSessionIdentity,
} from "../lib/composerSessionIdentity";
import type { ComposerSessionIdentity } from "../lib/types";

const persisted = composerSessionIdentityFromSource({
  modelRef: " provider/model-a ",
  billingSource: "newapi_byok",
  effort: " high ",
});
assert.equal(persisted.modelRef, "provider/model-a");
assert.equal(persisted.effort, "high", "persisted effort is normalized with its model identity");

const legacy = composerSessionIdentityFromSource({
  modelRef: "provider/legacy-model",
  billingSource: "newapi_byok",
});
assert.equal(legacy.effort, undefined, "missing legacy effort remains unknown");

const previous: ComposerSessionIdentity = {
  ...persisted,
  expertKnown: true,
  expertId: "team-a",
  expertName: "Team A",
};
const runtime = runtimeComposerSessionIdentity({
  modelRef: "provider/model-b",
  billingSource: "subscription_pool",
  effort: "low",
}, previous);
assert.equal(runtime.modelRef, "provider/model-b");
assert.equal(runtime.effort, "low", "runtime model and effort replace the stored pair together");
assert.equal(runtime.expertId, "team-a", "runtime identity keeps the existing expert fallback behavior");

const runtimeWithoutEffort = runtimeComposerSessionIdentity({
  modelRef: "provider/model-c",
  billingSource: "newapi_byok",
}, previous);
assert.equal(runtimeWithoutEffort.effort, undefined, "runtime unknown effort does not borrow a stored value");

console.log("composer session identity passed");
