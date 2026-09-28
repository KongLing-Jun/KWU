export type ProjectSurfaceMode = "available" | "read_only" | "requires_online";

export interface ProjectSurfacePolicy {
  summary: ProjectSurfaceMode;
  localBinding: ProjectSurfaceMode;
  sessionHistory: ProjectSurfaceMode;
  instruction: ProjectSurfaceMode;
  workItems: ProjectSurfaceMode;
  members: ProjectSurfaceMode;
  activity: ProjectSurfaceMode;
  assets: ProjectSurfaceMode;
  capabilities: ProjectSurfaceMode;
}

export function deriveProjectSurfacePolicy(stale: boolean): ProjectSurfacePolicy {
  const cloudSurface: ProjectSurfaceMode = stale ? "requires_online" : "available";
  return {
    summary: stale ? "read_only" : "available",
    localBinding: "available",
    sessionHistory: "available",
    instruction: cloudSurface,
    workItems: cloudSurface,
    members: cloudSurface,
    activity: cloudSurface,
    assets: cloudSurface,
    capabilities: cloudSurface,
  };
}
