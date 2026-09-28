import { describe, expect, it, vi } from "vitest";
import { cancelTabTurn } from "../features/session/cancelTabTurn";

describe("project Composer target-Tab Stop", () => {
  it("unsends, cancels and returns pending text for a non-active target Tab", () => {
    const actions = { unsend: vi.fn(), markRequested: vi.fn(), cancel: vi.fn() };
    expect(cancelTabTurn({ running: true, pendingLocalUser: { text: "restore this project prompt" } }, actions)).toBe("restore this project prompt");
    expect(actions.unsend).toHaveBeenCalledTimes(1);
    expect(actions.cancel).toHaveBeenCalledTimes(1);
    expect(actions.markRequested).not.toHaveBeenCalled();
  });
});
