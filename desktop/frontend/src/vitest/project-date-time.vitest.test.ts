import { describe, expect, it } from "vitest";
import { formatProjectDateTime } from "../features/projects/model/projectDateTime";

describe("project timestamp display", () => {
  it("formats local project timestamps with second-level precision", () => {
    const localDate = new Date(2026, 7, 22, 9, 5, 7);
    expect(formatProjectDateTime(localDate)).toBe("2026-08-22 09:05:07");
  });

  it("uses the requested fallback for invalid timestamps", () => {
    expect(formatProjectDateTime("not-a-date")).toBe("—");
    expect(formatProjectDateTime("not-a-date", "-")).toBe("-");
  });
});
