import type { Scene, SceneState, TabMeta } from "./types";

export type ScenePresentation = {
  scene: Scene;
  sceneState: SceneState;
};

export const defaultScenePresentation: ScenePresentation = {
  scene: "office",
  sceneState: "draft",
};

export function normalizeScenePresentation(scene: unknown, sceneState: unknown): ScenePresentation {
  const validScene = scene === "office" || scene === "coding" || scene === "design";
  const validState = sceneState === "draft" || sceneState === "locked";
  if (!validScene || !validState) return defaultScenePresentation;
  return { scene, sceneState };
}

type BlankSceneTab = Pick<
  TabMeta,
  "id" | "scope" | "workspaceRoot" | "sessionTurns" | "scene" | "sceneState" | "running" | "pendingPrompt" | "backgroundJobs"
>;

export function blankSessionRequestScenePresentation({
  targetScope,
  targetWorkspaceRoot,
  activeTabId,
  activeTab,
  visibleScene,
  surfaceIntent,
  surfaceTransitionActive,
  hasVisibleContent,
  allowCurrentBlankSceneReuse,
}: {
  targetScope: string;
  targetWorkspaceRoot: string;
  activeTabId?: string;
  activeTab?: BlankSceneTab;
  visibleScene: unknown;
  surfaceIntent: "blank" | "existing-session" | "unknown";
  surfaceTransitionActive: boolean;
  hasVisibleContent: boolean;
  allowCurrentBlankSceneReuse: boolean;
}): ScenePresentation {
  const targetRoot = targetScope === "project" ? targetWorkspaceRoot : "";
  const activeRoot = activeTab?.scope === "project" ? activeTab.workspaceRoot : "";
  const reusableCurrentBlank = Boolean(
    allowCurrentBlankSceneReuse
    && activeTab
    && activeTabId
    && activeTab.id === activeTabId
    && activeTab.scope === targetScope
    && activeRoot === targetRoot
    && activeTab.sessionTurns === 0
    && surfaceIntent === "blank"
    && !surfaceTransitionActive
    && !hasVisibleContent
    && !activeTab.running
    && !activeTab.pendingPrompt
    && !activeTab.backgroundJobs,
  );
  if (!reusableCurrentBlank) return defaultScenePresentation;
  return normalizeScenePresentation(visibleScene, activeTab?.sceneState);
}
