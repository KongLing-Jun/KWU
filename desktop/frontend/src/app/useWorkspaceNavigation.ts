import { useCallback } from "react";

export type WorkspaceView = "chat" | "experts" | "account" | "projects" | "skills" | "connectors" | "automation" | "artifacts" | "superpowers";
export type CapabilityWorkspaceView = Extract<WorkspaceView, "experts" | "skills" | "connectors">;

interface WorkspaceNavigationOptions {
  closeTransientOverlays: () => void;
  closeNarrowSidebar: () => void;
  clearSidebarDetail: () => void;
  clearSettingsTarget: () => void;
  closeHeartbeatOverlay: () => void;
  setSelectedCloudProjectId: (projectId: string) => void;
  setWorkspaceView: (view: WorkspaceView) => void;
}

export function useWorkspaceNavigation({
  closeTransientOverlays,
  closeNarrowSidebar,
  clearSidebarDetail,
  clearSettingsTarget,
  closeHeartbeatOverlay,
  setSelectedCloudProjectId,
  setWorkspaceView,
}: WorkspaceNavigationOptions) {
  const prepare = useCallback((closeSidebar: boolean) => {
    closeTransientOverlays();
    if (closeSidebar) closeNarrowSidebar();
    clearSidebarDetail();
    clearSettingsTarget();
  }, [clearSettingsTarget, clearSidebarDetail, closeNarrowSidebar, closeTransientOverlays]);

  const openExpertDirectory = useCallback(() => {
    prepare(true);
    setWorkspaceView("experts");
  }, [prepare, setWorkspaceView]);
  const openModelMarketplace = useCallback(() => {
    prepare(false);
    setWorkspaceView("account");
  }, [prepare, setWorkspaceView]);
  const openProjectDirectory = useCallback(() => {
    prepare(true);
    setSelectedCloudProjectId("");
    setWorkspaceView("projects");
  }, [prepare, setSelectedCloudProjectId, setWorkspaceView]);
  const openCloudProject = useCallback((projectId: string) => {
    prepare(true);
    setSelectedCloudProjectId(projectId);
    setWorkspaceView("projects");
  }, [prepare, setSelectedCloudProjectId, setWorkspaceView]);
  const openSkillsWorkspace = useCallback(() => {
    prepare(true);
    setWorkspaceView("skills");
  }, [prepare, setWorkspaceView]);
  const openConnectorsWorkspace = useCallback(() => {
    prepare(true);
    setWorkspaceView("connectors");
  }, [prepare, setWorkspaceView]);
  const openAutomationWorkspace = useCallback(() => {
    prepare(true);
    closeHeartbeatOverlay();
    setWorkspaceView("automation");
  }, [closeHeartbeatOverlay, prepare, setWorkspaceView]);
  const openArtifactLibrary = useCallback(() => {
    prepare(true);
    setWorkspaceView("artifacts");
  }, [prepare, setWorkspaceView]);
  const openSuperpowerWorkspace = useCallback(() => {
    prepare(true);
    setWorkspaceView("superpowers");
  }, [prepare, setWorkspaceView]);
  const openCapabilityWorkspace = useCallback((view: CapabilityWorkspaceView) => {
    if (view === "experts") openExpertDirectory();
    else if (view === "skills") openSkillsWorkspace();
    else openConnectorsWorkspace();
  }, [openConnectorsWorkspace, openExpertDirectory, openSkillsWorkspace]);

  return {
    openExpertDirectory,
    openModelMarketplace,
    openProjectDirectory,
    openCloudProject,
    openSkillsWorkspace,
    openConnectorsWorkspace,
    openAutomationWorkspace,
    openArtifactLibrary,
    openSuperpowerWorkspace,
    openCapabilityWorkspace,
  };
}
