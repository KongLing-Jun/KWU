import { useCallback, useState, type ReactNode } from "react";
import { useT } from "../../../lib/i18n";
import type { CloudProjectCapabilityBinding, CloudProjectSession } from "../../projects/model/projectTypes";
import { useAccountStore } from "../../account/store/accountStore";
import { CloudProjectsSection, type ProjectCapabilityKind } from "../../projects/ui/CloudProjectsSectionImpl";
import type { ProjectCapabilityCatalogItem } from "../../projects/ui/ProjectCapabilityDialog";

interface ProjectDirectoryProps {
  renderProjectComposer?: (context: { sessionKey: string; projectId: string; projectName: string; workspacePath: string; capabilityBindings: CloudProjectCapabilityBinding[]; instructionVersion: number; contentHash: string; projectWritable: boolean; projectDisabledReason?: string; onCreateSession: (tabId: string, workspacePath?: string, executionTarget?: "local" | "cloud") => Promise<CloudProjectSession>; onCapabilitiesChanged: (items: CloudProjectCapabilityBinding[]) => void }) => ReactNode;
  onOpenProjectCapability?: (kind: ProjectCapabilityKind, trigger: HTMLButtonElement, context: {
    sessionKey: string;
    projectId: string;
    onCapabilitiesChanged: (items: CloudProjectCapabilityBinding[]) => void;
    onCapabilityCatalogChanged: (kind: ProjectCapabilityKind, items: ProjectCapabilityCatalogItem[]) => void;
  }) => void;
  projectExperts?: ProjectCapabilityCatalogItem[];
  selectedCloudProjectId?: string;
  onSelectedCloudProjectChange?: (projectId: string) => void;
  restoreProjectCapabilityFocus?: ProjectCapabilityKind | null;
  onRestoreProjectCapabilityFocusConsumed?: () => void;
  onProjectComposerLeave?: (sessionKey: string) => void;
  onOpenPayments?: () => void;
}

export function ProjectDirectory({ renderProjectComposer, onOpenProjectCapability, projectExperts, selectedCloudProjectId, onSelectedCloudProjectChange, restoreProjectCapabilityFocus, onRestoreProjectCapabilityFocusConsumed, onProjectComposerLeave, onOpenPayments }: ProjectDirectoryProps) {
  const t = useT();
  const accountScopeKey = useAccountStore((state) => state.scopeKey);
  const cloudAuthenticated = useAccountStore((state) => state.account?.status === "authenticated");
  const [cloudWorkspaceOpen, setCloudWorkspaceOpen] = useState(false);
  const handleCloudWorkspaceChange = useCallback((open: boolean) => setCloudWorkspaceOpen(open), []);

  return (
    <section className={`primary-workspace primary-workspace--projects${cloudWorkspaceOpen ? " primary-workspace--project-open" : ""}`} aria-label={t("workspace.projects.title")}>
      <CloudProjectsSection accountScopeKey={accountScopeKey} authenticated={cloudAuthenticated} experts={projectExperts} onWorkspaceChange={handleCloudWorkspaceChange} renderComposer={renderProjectComposer} onOpenCapability={onOpenProjectCapability} selectedProjectId={selectedCloudProjectId} onSelectedProjectChange={onSelectedCloudProjectChange} restoreFocusCapability={restoreProjectCapabilityFocus} onRestoreFocusConsumed={onRestoreProjectCapabilityFocusConsumed} onProjectComposerLeave={onProjectComposerLeave} onOpenPayments={onOpenPayments} />
    </section>
  );
}
