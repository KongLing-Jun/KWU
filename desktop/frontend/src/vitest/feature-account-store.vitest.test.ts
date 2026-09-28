import { beforeEach, describe, expect, it } from "vitest";
import { emptyAccountState, useAccountStore } from "../features/account/store/accountStore";

describe("account capability bootstrap", () => {
  beforeEach(() => {
    useAccountStore.getState().reset();
    useAccountStore.setState(emptyAccountState());
  });

  it("fails closed instead of retaining a previous registration mode", () => {
    useAccountStore.getState().applyBootstrap({
      available: true,
      capabilities: {
        registration: true,
        registration_mode: "real",
        payments: false,
        realtime: false,
      },
      account: { status: "anonymous", profile_id: "profile-a" },
    });
    expect(useAccountStore.getState().capabilities.registration_mode).toBe("real");

    useAccountStore.getState().applyBootstrap({
      available: true,
      account: { status: "anonymous", profile_id: "profile-b" },
    });

    expect(useAccountStore.getState().capabilities).toEqual({
      registration: false,
      registration_mode: "unavailable",
      payments: false,
      realtime: false,
    });
  });
});
