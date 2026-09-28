import { describe, expect, it } from "vitest";
import { deriveProjectSurfacePolicy } from "../features/projects/model/projectSurfacePolicy";

describe("Cloud Project surface policy", () => {
  it("keeps only cached identity and device-local surfaces available while stale", () => {
    expect(deriveProjectSurfacePolicy(true)).toEqual({
      summary: "read_only",
      localBinding: "available",
      sessionHistory: "available",
      instruction: "requires_online",
      workItems: "requires_online",
      members: "requires_online",
      activity: "requires_online",
      assets: "requires_online",
      capabilities: "requires_online",
    });
  });

  it("makes every project surface available after an authoritative online read", () => {
    expect(deriveProjectSurfacePolicy(false)).toEqual({
      summary: "available",
      localBinding: "available",
      sessionHistory: "available",
      instruction: "available",
      workItems: "available",
      members: "available",
      activity: "available",
      assets: "available",
      capabilities: "available",
    });
  });
});
