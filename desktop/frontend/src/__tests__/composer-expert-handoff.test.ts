import assert from "node:assert/strict";

import {
  composerExpertShouldUseTargetIdentity,
  type ComposerExpertResolution,
} from "../lib/composerExpertHandoff";

const target = {
  hasRenderedTarget: true,
  surfaceTransitionActive: false,
  renderedViewKey: "session-b",
  activeTabId: "tab-b",
};

const authoritativeTargetResolution: ComposerExpertResolution = {
  viewKey: "session-b",
  tabId: "tab-b",
  refreshSequence: 4,
  authoritative: true,
};

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  surfaceTransitionActive: true,
  resolution: authoritativeTargetResolution,
}), true, "an active transition always owns the displayed expert");

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  surfaceTransitionActive: true,
  optimisticExpertActivation: true,
  resolution: null,
}), false, "an expert being activated remains visible while its new task target loads");

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  resolution: { ...authoritativeTargetResolution, tabId: "tab-a" },
}), true, "a source-tab resolution cannot take ownership from the target identity");

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  resolution: { ...authoritativeTargetResolution, viewKey: "session-a" },
}), true, "a stale view generation cannot take ownership from the target identity");

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  resolution: { ...authoritativeTargetResolution, authoritative: false },
}), true, "a provisional runtime result cannot take ownership from the target identity");

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  resolution: authoritativeTargetResolution,
}), false, "the matching authoritative runtime result completes the handoff");

assert.equal(composerExpertShouldUseTargetIdentity({
  ...target,
  hasRenderedTarget: false,
  resolution: null,
}), false, "ordinary active tabs without a rendered target continue to use activeExpert");

console.log("composer expert handoff passed");
