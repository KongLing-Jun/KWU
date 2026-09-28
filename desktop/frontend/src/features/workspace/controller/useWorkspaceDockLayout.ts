import { useCallback } from "react";

import {
  RIGHT_DOCK_MIN_RENDER_WIDTH,
  RIGHT_DOCK_PREVIEW_MIN_WIDTH,
  RIGHT_DOCK_TREE_MIN_WIDTH,
  type RightDockMode,
} from "../../../store/layout";
import {
  availableWorkspacePanelWidth,
  resolveLiveWorkspacePanelWidth,
  resolveWorkspacePanelWidth,
  workspacePanelCanOpen,
} from "../../../lib/workspaceLayout";

export const CHAT_MIN_WIDTH = 400;
export const CHAT_COMFORT_MIN_WIDTH = 560;
export const WORKSPACE_RESIZER_WIDTH = 8;

interface WorkspaceDockLayoutOptions {
  rightDockMode: RightDockMode;
  workspacePreviewActive: boolean;
  rightDockPreviewWidth: number;
  rightDockTreeWidth: number;
  workspacePanelOpen: boolean;
  workspacePanelMaximized: boolean;
  liveWorkspacePanelRenderWidth: number | null;
  viewportWidth: number;
  sidebarCollapsed: boolean;
  sidebarWidth: number;
}

export function useWorkspaceDockLayout({
  rightDockMode,
  workspacePreviewActive,
  rightDockPreviewWidth,
  rightDockTreeWidth,
  workspacePanelOpen,
  workspacePanelMaximized,
  liveWorkspacePanelRenderWidth,
  viewportWidth,
  sidebarCollapsed,
  sidebarWidth,
}: WorkspaceDockLayoutOptions) {
  const rightDockDetailActive = rightDockMode !== "context" && workspacePreviewActive;
  const preferredWorkspacePanelWidth = rightDockDetailActive ? rightDockPreviewWidth : rightDockTreeWidth;
  const workspacePanelMinWidth = rightDockDetailActive ? RIGHT_DOCK_PREVIEW_MIN_WIDTH : RIGHT_DOCK_TREE_MIN_WIDTH;
  const chatReservedWidth = workspacePanelOpen && !workspacePanelMaximized ? CHAT_COMFORT_MIN_WIDTH : CHAT_MIN_WIDTH;
  const workspacePanelAvailableWidth = availableWorkspacePanelWidth({
    viewportWidth,
    sidebarCollapsed,
    sidebarWidth,
    chatMinWidth: chatReservedWidth,
    resizerWidth: WORKSPACE_RESIZER_WIDTH,
  });
  const resolvedWorkspacePanelWidth = resolveWorkspacePanelWidth({
    open: workspacePanelOpen,
    maximized: workspacePanelMaximized,
    preferredWidth: preferredWorkspacePanelWidth,
    minWidth: workspacePanelMinWidth,
    availableWidth: workspacePanelAvailableWidth,
  });
  const storedWorkspacePanelRenderWidth = workspacePanelMaximized ? preferredWorkspacePanelWidth : resolvedWorkspacePanelWidth;
  const workspacePanelRenderWidth = liveWorkspacePanelRenderWidth ?? storedWorkspacePanelRenderWidth;
  const workspacePanelRenderable = workspacePanelOpen
    && (workspacePanelMaximized || workspacePanelRenderWidth >= RIGHT_DOCK_MIN_RENDER_WIDTH);
  const workspacePanelGridOpen = workspacePanelRenderable && !workspacePanelMaximized;
  const workspacePanelExpandAvailableWidth = availableWorkspacePanelWidth({
    viewportWidth,
    sidebarCollapsed,
    sidebarWidth,
    chatMinWidth: CHAT_COMFORT_MIN_WIDTH,
    resizerWidth: WORKSPACE_RESIZER_WIDTH,
  });
  const workspacePanelExpandDisabled = !workspacePanelRenderable && !workspacePanelCanOpen({
    preferredWidth: rightDockTreeWidth,
    minWidth: RIGHT_DOCK_TREE_MIN_WIDTH,
    availableWidth: workspacePanelExpandAvailableWidth,
    minRenderWidth: RIGHT_DOCK_MIN_RENDER_WIDTH,
  });
  const resolveLiveWorkspacePanelRenderWidth = useCallback(
    (preferredWidth: number, nextSidebarWidth = sidebarWidth) => resolveLiveWorkspacePanelWidth({
      viewportWidth,
      sidebarCollapsed,
      sidebarWidth: nextSidebarWidth,
      chatMinWidth: chatReservedWidth,
      resizerWidth: WORKSPACE_RESIZER_WIDTH,
      open: workspacePanelOpen,
      maximized: workspacePanelMaximized,
      preferredWidth,
      minWidth: workspacePanelMinWidth,
    }),
    [chatReservedWidth, sidebarCollapsed, sidebarWidth, viewportWidth, workspacePanelMaximized, workspacePanelMinWidth, workspacePanelOpen],
  );

  return {
    rightDockDetailActive,
    preferredWorkspacePanelWidth,
    workspacePanelMinWidth,
    chatReservedWidth,
    workspacePanelAvailableWidth,
    resolvedWorkspacePanelWidth,
    storedWorkspacePanelRenderWidth,
    workspacePanelRenderWidth,
    workspacePanelRenderable,
    workspacePanelGridOpen,
    workspacePanelExpandAvailableWidth,
    workspacePanelExpandDisabled,
    resolveLiveWorkspacePanelRenderWidth,
  };
}
