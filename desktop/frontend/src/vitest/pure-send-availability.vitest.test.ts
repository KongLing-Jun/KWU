import { describe, expect, it } from "vitest";
import {
  sendAvailabilityAllowsControllerActions,
  sendAvailabilityMessageKey,
} from "../lib/sendAvailability";
import type { TabSendAvailabilityState } from "../lib/types";

describe("send availability pure policy", () => {
  it.each([
    ["ready", true],
    ["starting", false],
    ["subscription_auth_required", false],
    ["subscription_account_action_required", false],
    ["byok_configuration_required", false],
    ["service_unavailable", false],
    ["startup_failed", false],
  ] satisfies Array<[TabSendAvailabilityState, boolean]>)(
    "allows controller actions for %s = %s",
    (state, expected) => {
      expect(sendAvailabilityAllowsControllerActions({ state, retryable: false })).toBe(expected);
    },
  );

  it("uses different recovery copy when local history exists", () => {
    expect(sendAvailabilityMessageKey("subscription_auth_required", false))
      .toBe("sendAvailability.subscriptionAuthBlank");
    expect(sendAvailabilityMessageKey("subscription_auth_required", true))
      .toBe("sendAvailability.subscriptionAuthHistory");
  });
});
