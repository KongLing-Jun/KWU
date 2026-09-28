import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { ArrowRight, Archive, Boxes, BriefcaseBusiness, Cloud, FileText, FolderKanban, ListChecks, LoaderCircle, MessageSquare, MoreVertical, PanelRightClose, PanelRightOpen, Pencil, Plus, RefreshCw, Search, Settings2, Sparkles, Trash2, UserPlus, Users, X } from "lucide-react";
import { clientErrorCode } from "../../../lib/clientError";
import { useT, type DictKey } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { projectGateway, type ProjectGateway } from "../data/projectGateway";
import { projectTemplateInstructions, type ProjectTemplateId } from "../model/projectTemplateInstructions";
import { formatProjectDateTime } from "../model/projectDateTime";
import { deriveProjectSurfacePolicy } from "../model/projectSurfacePolicy";
import type { CloudProject, CloudProjectCapabilityBinding, CloudProjectInstruction, CloudProjectPage, CloudProjectResult, CloudProjectSession } from "../model/projectTypes";
import { ProjectActionDialog, type ProjectDialogKind, type ProjectDraftCapabilities, type ProjectDraftCapabilityKind, type ProjectTemplateOption } from "./ProjectActionDialog";
import { ProjectCapabilityDialog, type ProjectCapabilityCatalogItem } from "./ProjectCapabilityDialog";
import { installMissingBoundProjectSkills, loadProjectCapabilityCatalog, type ProjectCatalogKind } from "./projectCapabilityCatalog";
import { ProjectCapabilityIcon } from "./ProjectCapabilityIcon";
import { ProjectActivityPanel } from "./ProjectActivityPanel";
import { ProjectInstructionDialog } from "./ProjectInstructionDialog";
import { ProjectJoinDialog } from "./ProjectJoinDialog";
import { ProjectMemberDrawer } from "./ProjectMemberDrawer";
import { ProjectAssetsPanel } from "./ProjectAssetsPanel";
import { WorkItemBoard } from "./WorkItemBoard";
import { useProjectContextController } from "./useProjectContextController";
import { initialJoinedProjectState, initialProjectDirectoryState, useProjectDirectoryController } from "./useProjectDirectoryController";
import { useProjectLifecycleController } from "./useProjectLifecycleController";
import { useProjectWorkspaceController } from "./useProjectWorkspaceController";

interface CloudProjectsSectionProps {
  accountScopeKey: string;
  authenticated: boolean;
  gateway?: ProjectGateway;
  experts?: ProjectCapabilityCatalogItem[];
  onWorkspaceChange?: (open: boolean) => void;
  renderComposer?: (context: { sessionKey: string; projectId: string; projectName: string; workspacePath: string; capabilityBindings: CloudProjectCapabilityBinding[]; instructionVersion: number; contentHash: string; projectWritable: boolean; projectDisabledReason?: string; onCreateSession: (tabId: string, workspacePath?: string, executionTarget?: "local" | "cloud") => Promise<CloudProjectSession>; onCapabilitiesChanged: (items: CloudProjectCapabilityBinding[]) => void }) => ReactNode;
  onOpenCapability?: (kind: ProjectCapabilityKind, trigger: HTMLButtonElement, context: {
    sessionKey: string;
    projectId: string;
    onCapabilitiesChanged: (items: CloudProjectCapabilityBinding[]) => void;
    onCapabilityCatalogChanged: (kind: ProjectCapabilityKind, items: ProjectCapabilityCatalogItem[]) => void;
  }) => void;
  selectedProjectId?: string;
  onSelectedProjectChange?: (projectId: string) => void;
  restoreFocusCapability?: ProjectCapabilityKind | null;
  onRestoreFocusConsumed?: () => void;
  onProjectComposerLeave?: (sessionKey: string) => void;
  onOpenPayments?: () => void;
}

export type ProjectCapabilityKind = "connector" | "expert" | "skill";

const projectTemplates: Array<{ id: ProjectTemplateId; title: DictKey; description: DictKey; instruction: string; Icon: typeof FileText }> = [
  { id: "report", title: "workspace.projects.starter.report.title", description: "workspace.projects.starter.report.desc", instruction: projectTemplateInstructions.report, Icon: FileText },
  { id: "research", title: "workspace.projects.starter.research.title", description: "workspace.projects.starter.research.desc", instruction: projectTemplateInstructions.research, Icon: Search },
  { id: "meeting", title: "workspace.projects.starter.meeting.title", description: "workspace.projects.starter.meeting.desc", instruction: projectTemplateInstructions.meeting, Icon: MessageSquare },
  { id: "plan", title: "workspace.projects.starter.plan.title", description: "workspace.projects.starter.plan.desc", instruction: projectTemplateInstructions.plan, Icon: BriefcaseBusiness },
  { id: "review", title: "workspace.projects.starter.review.title", description: "workspace.projects.starter.review.desc", instruction: projectTemplateInstructions.review, Icon: Sparkles },
];

interface ProjectCardMenuProps {
  projectName: string;
  disabled: boolean;
  onRename(trigger: HTMLButtonElement): void;
  onDelete(trigger: HTMLButtonElement): void;
}

function ProjectCardMenu({ projectName, disabled, onRename, onDelete }: ProjectCardMenuProps) {
  const t = useT();
  const menuId = useId();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const firstActionRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => firstActionRef.current?.focus());
    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", handlePointerDown);
    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener("pointerdown", handlePointerDown);
    };
  }, [open]);

  const invoke = (action: (trigger: HTMLButtonElement) => void) => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    setOpen(false);
    action(trigger);
  };

  return (
    <div ref={rootRef} className="cloud-projects__card-menu" onKeyDown={(event) => {
      if (event.key === "Escape" && open) {
        event.preventDefault();
        event.stopPropagation();
        setOpen(false);
        triggerRef.current?.focus();
        return;
      }
      if (open && (event.key === "ArrowDown" || event.key === "ArrowUp" || event.key === "Home" || event.key === "End")) {
        const actions = Array.from(rootRef.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') || []);
        if (actions.length === 0) return;
        event.preventDefault();
        const currentIndex = actions.indexOf(document.activeElement as HTMLButtonElement);
        const nextIndex = event.key === "Home" ? 0 : event.key === "End" ? actions.length - 1 : event.key === "ArrowDown" ? (currentIndex + 1 + actions.length) % actions.length : (currentIndex - 1 + actions.length) % actions.length;
        actions[nextIndex]?.focus();
      }
    }}>
      <button
        ref={triggerRef}
        type="button"
        className="cloud-projects__card-menu-trigger"
        aria-label="项目操作"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
      >
        <MoreVertical size={18} aria-hidden="true" />
      </button>
      {open ? (
        <div id={menuId} className="cloud-projects__card-menu-popover" role="menu" aria-label={`${projectName} 操作菜单`}>
          <button ref={firstActionRef} type="button" role="menuitem" className="cloud-projects__card-menu-action" onClick={() => invoke(onRename)}><Pencil size={16} aria-hidden="true" />{t("workspace.cloudProjects.rename")}</button>
          <button type="button" role="menuitem" className="cloud-projects__card-menu-action cloud-projects__card-menu-action--danger" onClick={() => invoke(onDelete)}><Trash2 size={16} aria-hidden="true" />{t("workspace.cloudProjects.delete")}</button>
        </div>
      ) : null}
    </div>
  );
}

interface ProjectLifecycleMenuProps {
  project: CloudProject;
  disabled: boolean;
  onAction(kind: Extract<ProjectDialogKind, "edit" | "archive" | "restore" | "delete">, trigger: HTMLButtonElement): void;
}

function ProjectLifecycleMenu({ project, disabled, onAction }: ProjectLifecycleMenuProps) {
  const t = useT();
  const menuId = useId();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const firstActionRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => firstActionRef.current?.focus());
    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", handlePointerDown);
    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener("pointerdown", handlePointerDown);
    };
  }, [open]);

  const invoke = (kind: Extract<ProjectDialogKind, "edit" | "archive" | "restore" | "delete">) => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    setOpen(false);
    onAction(kind, trigger);
  };

  return (
    <div ref={rootRef} className="cloud-projects__lifecycle-menu" onKeyDown={(event) => {
      if (event.key === "Escape" && open) {
        event.preventDefault();
        event.stopPropagation();
        setOpen(false);
        triggerRef.current?.focus();
        return;
      }
      if (open && (event.key === "ArrowDown" || event.key === "ArrowUp" || event.key === "Home" || event.key === "End")) {
        const actions = Array.from(rootRef.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not(:disabled)') || []);
        if (actions.length === 0) return;
        event.preventDefault();
        const currentIndex = actions.indexOf(document.activeElement as HTMLButtonElement);
        const nextIndex = event.key === "Home" ? 0 : event.key === "End" ? actions.length - 1 : event.key === "ArrowDown" ? (currentIndex + 1 + actions.length) % actions.length : (currentIndex - 1 + actions.length) % actions.length;
        actions[nextIndex]?.focus();
      }
    }}>
      <button
        ref={triggerRef}
        type="button"
        className="cloud-projects__lifecycle-menu-trigger"
        aria-label="项目操作"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        onClick={() => setOpen((current) => !current)}
      >
        <span aria-hidden="true">•••</span>
      </button>
      {open ? (
        <div id={menuId} role="menu" aria-label={`${project.name} 项目操作菜单`}>
          {project.status === "active" ? <>
            <button ref={firstActionRef} type="button" role="menuitem" disabled={disabled} onClick={() => invoke("edit")}>{t("workspace.cloudProjects.edit")}</button>
            <button type="button" role="menuitem" disabled={disabled} onClick={() => invoke("archive")}>{t("workspace.cloudProjects.archive")}</button>
          </> : <button ref={firstActionRef} type="button" role="menuitem" disabled={disabled} onClick={() => invoke("restore")}>{t("workspace.cloudProjects.restore")}</button>}
          <button type="button" role="menuitem" disabled={disabled} onClick={() => invoke("delete")}>永久删除</button>
        </div>
      ) : null}
    </div>
  );
}

function emptyDraftCapabilities(): ProjectDraftCapabilities {
  return { connector: [], expert: [], skill: [] };
}

const emptyProjectExperts: ProjectCapabilityCatalogItem[] = [];

export function CloudProjectsSection({ accountScopeKey, authenticated, gateway = projectGateway, experts = emptyProjectExperts, onWorkspaceChange, renderComposer, onOpenCapability, selectedProjectId = "", onSelectedProjectChange, restoreFocusCapability = null, onRestoreFocusConsumed, onProjectComposerLeave, onOpenPayments }: CloudProjectsSectionProps) {
  const t = useT();
  const { showError, showToast } = useToast();
  const instructionButtonRef = useRef<HTMLButtonElement>(null);
  const configurationToggleRef = useRef<HTMLButtonElement>(null);
  const configurationCloseRef = useRef<HTMLButtonElement>(null);
  const workspaceGridRef = useRef<HTMLDivElement>(null);
  const configurationAutoCollapsedRef = useRef(false);
  const previousConfigurationCompactRef = useRef(false);
  const searchComposingRef = useRef(false);
  const instructionFeatureRetryRef = useRef("");
  const directoryRequestRef = useRef(0);
  const detailRequestRef = useRef(0);
  const inviteRequestRef = useRef(0);
  const { ownerScopeRef, projectScopeRef, selectedProjectRef, featureRequestRef, joinedRequestRef, state, setState, joinedState, setJoinedState, query, setQuery, appliedQuery, setAppliedQuery, loadingMore, setLoadingMore, joinedLoadingMore, setJoinedLoadingMore } = useProjectDirectoryController(accountScopeKey, authenticated);
  const { instructionRequestRef, capabilityBindings, setCapabilityBindings, instructionStatus, setInstructionStatus, instruction, setInstruction, instructionHumanDraft, setInstructionHumanDraft, instructionAgentDraft, setInstructionAgentDraft, instructionEditing, setInstructionEditing, instructionPending, setInstructionPending, instructionError, setInstructionError, instructionNotice, setInstructionNotice, workItemsAvailable, setWorkItemsAvailable, instructionsAvailable, setInstructionsAvailable, membersAvailable, setMembersAvailable, activityAvailable, setActivityAvailable, assetsAvailable, setAssetsAvailable, projectMembers, setProjectMembers, currentRole, setCurrentRole } = useProjectContextController();
  const { dialog, setDialog, returnFocus, setReturnFocus, draftName, setDraftName, draftDescription, setDraftDescription, templateId, setTemplateId, templateInstruction, setTemplateInstruction, dialogError, setDialogError, dialogNameInvalid, setDialogNameInvalid, pending, setPending, liveMessage, setLiveMessage, actionProject, setActionProject } = useProjectLifecycleController();
  const { inviteButtonRef, memberDrawerCloseRef, projectSessionRef, restoredProjectRef, workspaceTab, setWorkspaceTab, memberDrawerOpen, setMemberDrawerOpen, closeMemberDrawer } = useProjectWorkspaceController();
  const [draftCapabilities, setDraftCapabilities] = useState<Record<ProjectDraftCapabilityKind, ProjectCapabilityCatalogItem[]>>(emptyDraftCapabilities);
  const [draftCapabilityKind, setDraftCapabilityKind] = useState<ProjectDraftCapabilityKind | null>(null);
  const draftCapabilityTriggerRef = useRef<HTMLButtonElement | null>(null);
  const [capabilityCatalogs, setCapabilityCatalogs] = useState<Partial<Record<ProjectCapabilityKind, ProjectCapabilityCatalogItem[]>>>({});
  const [capabilityBindingsStatus, setCapabilityBindingsStatus] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [configurationCollapsed, setConfigurationCollapsed] = useState(false);
  const [configurationCompact, setConfigurationCompact] = useState(false);
  const [configurationNarrow, setConfigurationNarrow] = useState(false);
  const [joinDialogOpen, setJoinDialogOpen] = useState(false);
  const [joinProjectId, setJoinProjectId] = useState("");
  const [joinPending, setJoinPending] = useState(false);
  const [joinError, setJoinError] = useState("");
  const [joinInvalid, setJoinInvalid] = useState(false);
  const [joinReturnFocus, setJoinReturnFocus] = useState<HTMLElement | null>(null);
  const [projectWorkspacePath, setProjectWorkspacePath] = useState("");

  if (ownerScopeRef.current !== accountScopeKey) {
    ownerScopeRef.current = accountScopeKey;
    projectScopeRef.current = "";
    selectedProjectRef.current = "";
    instructionRequestRef.current += 1;
    featureRequestRef.current += 1;
    joinedRequestRef.current += 1;
    directoryRequestRef.current += 1;
    detailRequestRef.current += 1;
    inviteRequestRef.current += 1;
  }

  useEffect(() => {
    setMemberDrawerOpen(false);
    projectSessionRef.current = null;
    setWorkspaceTab("activity");
    setDialog(null);
    setDraftName("");
    setDraftDescription("");
    setTemplateInstruction("");
    setDraftCapabilities(emptyDraftCapabilities());
    setDraftCapabilityKind(null);
    setCapabilityCatalogs({});
    setConfigurationCollapsed(false);
    setConfigurationCompact(false);
    setConfigurationNarrow(false);
    configurationAutoCollapsedRef.current = false;
    previousConfigurationCompactRef.current = false;
    setDialogNameInvalid(false);
    setActionProject(null);
    setJoinDialogOpen(false);
    setJoinProjectId("");
    setJoinPending(false);
    setJoinError("");
    setJoinInvalid(false);
    setJoinReturnFocus(null);
    setProjectWorkspacePath("");
  }, [accountScopeKey]);

  const acceptPage = useCallback((ownerScope: string, page: CloudProjectPage) => {
    if (ownerScopeRef.current !== ownerScope) return false;
    if (projectScopeRef.current && projectScopeRef.current !== page.projectScopeKey) return false;
    projectScopeRef.current = page.projectScopeKey;
    return true;
  }, []);

  const load = useCallback(async (search: string, cursor = "", append = false) => {
    const ownerScope = accountScopeKey;
    const request = ++directoryRequestRef.current;
    const featureRequest = featureRequestRef.current + 1;
    featureRequestRef.current = featureRequest;
    if (!authenticated || !ownerScope) return false;
    const isCurrentRequest = () => directoryRequestRef.current === request && ownerScopeRef.current === ownerScope;
    if (append) setLoadingMore(true);
    else {
      setWorkItemsAvailable(false);
      setInstructionsAvailable(false);
      setMembersAvailable(false);
      setActivityAvailable(false);
      setAssetsAvailable(false);
      setState((current) => ({ ...initialProjectDirectoryState(ownerScope, true), selected: current.ownerScope === ownerScope ? current.selected : null }));
    }
    try {
      const page = await gateway.listCloudProjects({ pageSize: 20, ...(search ? { search } : {}), ...(cursor ? { cursor } : {}) });
      if (!isCurrentRequest()) return false;
      if (!acceptPage(ownerScope, page)) return false;
      let pageItems = page.items;
      if (!search && !cursor && !page.stale && typeof gateway.claimCloudProjectOnboarding === "function") {
        try {
          const claim = await gateway.claimCloudProjectOnboarding();
          if (!isCurrentRequest()) return false;
          if (claim.projectScopeKey === page.projectScopeKey && claim.state === "active" && claim.project) {
            pageItems = [claim.project, ...page.items.filter((item) => item.projectId !== claim.project?.projectId)];
          }
        } catch {
          // Onboarding is additive: the authoritative project directory remains usable.
        }
      }
      let nextWorkItemsAvailable = false;
      let nextInstructionsAvailable = false;
      let nextMembersAvailable = false;
      let nextActivityAvailable = false;
      let nextAssetsAvailable = false;
      if (!page.stale && typeof gateway.getCloudProjectFeatures === "function") {
        try {
          const features = await gateway.getCloudProjectFeatures();
          if (isCurrentRequest() && featureRequestRef.current === featureRequest && features.projectScopeKey === projectScopeRef.current) {
            nextWorkItemsAvailable = features.workItemsAvailable;
            nextInstructionsAvailable = features.instructionsAvailable;
            nextMembersAvailable = features.membersAvailable;
            nextActivityAvailable = features.activityAvailable;
            nextAssetsAvailable = features.assetsAvailable;
          }
        } catch {
          // Feature slices fail closed while base Project identity remains usable.
        }
      }
      if (!isCurrentRequest() || featureRequestRef.current !== featureRequest) return false;
      setWorkItemsAvailable(nextWorkItemsAvailable);
      setInstructionsAvailable(nextInstructionsAvailable);
      setMembersAvailable(nextMembersAvailable);
      setActivityAvailable(nextActivityAvailable);
      setAssetsAvailable(nextAssetsAvailable);
      setState((current) => ({
        ownerScope,
        availability: "available",
        projectScope: page.projectScopeKey,
        items: append && current.ownerScope === ownerScope ? [...current.items, ...pageItems] : pageItems,
        nextCursor: page.nextCursor,
        selected: current.ownerScope === ownerScope ? current.selected : null,
        stale: page.stale,
        cachedAt: page.cachedAt,
        message: "",
      }));
      return true;
    } catch (error) {
      if (!isCurrentRequest()) return false;
      const code = clientErrorCode(error);
      setState({
        ...initialProjectDirectoryState(ownerScope, true),
        availability: code === "CAPABILITY_UNAVAILABLE" ? "unavailable" : "error",
        message: t("workspace.cloudProjects.loadFailed"),
      });
      showError("projects.operation", error, t("workspace.cloudProjects.loadFailed"));
      return false;
    } finally {
      if (isCurrentRequest()) setLoadingMore(false);
    }
  }, [acceptPage, accountScopeKey, authenticated, gateway, showError, t]);

  const loadJoined = useCallback(async (search: string, cursor = "", append = false) => {
    const ownerScope = accountScopeKey;
    const request = ++joinedRequestRef.current;
    if (!authenticated || !ownerScope) return false;
    const isCurrentRequest = () => joinedRequestRef.current === request && ownerScopeRef.current === ownerScope;
    if (append) setJoinedLoadingMore(true);
    else setJoinedState(initialJoinedProjectState(ownerScope, true));
    try {
      const page = await gateway.listJoinedCloudProjects({ pageSize: 20, ...(search ? { search } : {}), ...(cursor ? { cursor } : {}) });
      if (!isCurrentRequest() || !acceptPage(ownerScope, page)) return false;
      setJoinedState((current) => ({
        ownerScope,
        availability: "available",
        projectScope: page.projectScopeKey,
        items: append && current.ownerScope === ownerScope ? [...current.items, ...page.items] : page.items,
        nextCursor: page.nextCursor,
        message: "",
      }));
      return true;
    } catch (error) {
      if (!isCurrentRequest()) return false;
      const code = clientErrorCode(error);
      setJoinedState({
        ...initialJoinedProjectState(ownerScope, true),
        availability: code === "CAPABILITY_UNAVAILABLE" ? "unavailable" : "error",
        message: t("workspace.cloudProjects.joinedLoadFailed"),
      });
      showError("projects.operation", error, t("workspace.cloudProjects.joinedLoadFailed"));
      return false;
    } finally {
      if (isCurrentRequest()) setJoinedLoadingMore(false);
    }
  }, [acceptPage, accountScopeKey, authenticated, gateway, showError, t]);

  useEffect(() => {
    setQuery("");
    setAppliedQuery("");
    setLoadingMore(false);
    setJoinedLoadingMore(false);
    setDialog(null);
    setReturnFocus(null);
    setDraftName("");
    setDraftDescription("");
    setDialogError("");
    setDialogNameInvalid(false);
    setPending(false);
    setLiveMessage("");
    selectedProjectRef.current = "";
    instructionRequestRef.current += 1;
    setInstructionStatus("idle");
    setInstruction(null);
    setInstructionHumanDraft("");
    setInstructionAgentDraft("");
    setInstructionEditing(false);
    setInstructionPending(false);
    setInstructionError("");
    setInstructionNotice("");
    setWorkItemsAvailable(false);
    setInstructionsAvailable(false);
    setMembersAvailable(false);
    setActivityAvailable(false);
    setAssetsAvailable(false);
    setProjectMembers([]);
    setCurrentRole(null);
    setCapabilityBindings([]);
    setCapabilityBindingsStatus("idle");
    setCapabilityCatalogs({});
    setProjectWorkspacePath("");
    setWorkspaceTab("activity");
    setMemberDrawerOpen(false);
    onWorkspaceChange?.(false);
    setState(initialProjectDirectoryState(accountScopeKey, authenticated));
    setJoinedState(initialJoinedProjectState(accountScopeKey, authenticated));
    if (authenticated && accountScopeKey) {
      void load("");
      void loadJoined("");
    }
  }, [accountScopeKey, authenticated, load, loadJoined, onWorkspaceChange]);

  useEffect(() => {
    if (!memberDrawerOpen || membersAvailable) return;
    memberDrawerCloseRef.current?.focus();
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") closeMemberDrawer();
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [closeMemberDrawer, memberDrawerOpen, membersAvailable]);

  const visibleState = state.ownerScope === accountScopeKey ? state : initialProjectDirectoryState(accountScopeKey, authenticated);
  const controlledSelectionMismatch = Boolean(
    onSelectedProjectChange
      && visibleState.selected
      && visibleState.selected.projectId !== selectedProjectId,
  );
  const visible = controlledSelectionMismatch ? { ...visibleState, selected: null } : visibleState;
  const pendingSelectedProjectId = onSelectedProjectChange
    && selectedProjectId
    && visibleState.selected?.projectId !== selectedProjectId
    ? selectedProjectId
    : "";
  const visibleJoined = joinedState.ownerScope === accountScopeKey ? joinedState : initialJoinedProjectState(accountScopeKey, authenticated);
  const surfacePolicy = deriveProjectSurfacePolicy(visible.stale);

  const commitProjectSearch = useCallback(() => {
    if (!authenticated || !accountScopeKey || visible.selected) return;
    const next = query.trim();
    if (next === appliedQuery) return;
    setAppliedQuery(next);
    void load(next);
    void loadJoined(next);
  }, [accountScopeKey, appliedQuery, authenticated, load, loadJoined, query, visible.selected]);

  useEffect(() => {
    if (configurationCollapsed) return;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || !configurationCompact) return;
      setConfigurationCollapsed(true);
      configurationAutoCollapsedRef.current = false;
      requestAnimationFrame(() => configurationToggleRef.current?.focus());
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [configurationCollapsed, configurationCompact]);

  useEffect(() => {
    const target = workspaceGridRef.current;
    if (!target || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => {
      const width = entry?.contentRect.width ?? target.getBoundingClientRect().width;
      setConfigurationCompact(width <= 1100);
      setConfigurationNarrow(width <= 760);
    });
    observer.observe(target);
    return () => observer.disconnect();
  }, [visible.selected?.projectId]);

  useEffect(() => {
    const wasCompact = previousConfigurationCompactRef.current;
    previousConfigurationCompactRef.current = configurationCompact;
    if (configurationCompact && !wasCompact) {
      setConfigurationCollapsed((current) => {
        if (!current) configurationAutoCollapsedRef.current = true;
        return true;
      });
      return;
    }
    if (!configurationCompact && wasCompact && configurationAutoCollapsedRef.current) {
      configurationAutoCollapsedRef.current = false;
      setConfigurationCollapsed(false);
    }
  }, [configurationCompact]);

  const templateOptions: ProjectTemplateOption[] = projectTemplates.map((template) => ({
    id: template.id,
    label: t(template.title),
    description: t(template.description),
    instruction: template.instruction,
  }));

  const selectProject = async (projectId: string, installMissingSkills = false) => {
    const ownerScope = accountScopeKey;
    const detailRequest = ++detailRequestRef.current;
    let enteredWorkspace = false;
    const instructionRequest = instructionRequestRef.current + 1;
    instructionRequestRef.current = instructionRequest;
    instructionFeatureRetryRef.current = "";
    selectedProjectRef.current = projectId;
    onSelectedProjectChange?.(projectId);
    projectSessionRef.current = null;
    setWorkspaceTab("activity");
    setInstructionPending(false);
    setInstructionStatus("loading");
    setInstruction(null);
    setInstructionHumanDraft("");
    setInstructionAgentDraft("");
    setInstructionEditing(false);
    setInstructionError("");
    setInstructionNotice("");
    setProjectMembers([]);
    setCurrentRole(null);
    setCapabilityBindings([]);
    setCapabilityBindingsStatus("loading");
    setCapabilityCatalogs({});
    const isCurrentDetail = () => detailRequestRef.current === detailRequest && ownerScopeRef.current === ownerScope && selectedProjectRef.current === projectId;
    try {
      const result = await gateway.getCloudProject(projectId);
      if (!isCurrentDetail()) return false;
      if (!acceptPage(ownerScope, { projectScopeKey: result.projectScopeKey, items: [], nextCursor: null, stale: false, cachedAt: null })) return false;
      setState((current) => current.ownerScope === ownerScope ? {
        ...current,
        selected: result.project,
        stale: result.stale,
        cachedAt: result.cachedAt,
        message: "",
      } : current);
      setProjectWorkspacePath(result.workspacePath?.trim() || "");
      setWorkspaceTab("activity");
      onWorkspaceChange?.(true);
      enteredWorkspace = true;
      if (result.stale || !instructionsAvailable) {
        setInstructionStatus("unavailable");
      } else {
        try {
          const nextInstruction = await gateway.getCloudProjectInstruction(projectId);
          if (isCurrentDetail() && instructionRequestRef.current === instructionRequest && nextInstruction.projectScopeKey === projectScopeRef.current) {
            setInstruction(nextInstruction);
            setInstructionHumanDraft(nextInstruction.humanDescription);
            setInstructionAgentDraft(nextInstruction.agentInstruction);
            setInstructionStatus("ready");
          }
        } catch (error) {
          if (isCurrentDetail() && instructionRequestRef.current === instructionRequest) {
            setInstructionStatus("error");
            setInstructionError(t("workspace.cloudProjects.instruction.loadFailed"));
            showError("projects.operation", error, t("workspace.cloudProjects.instruction.loadFailed"));
          }
        }
      }
      let loadedCapabilityBindings: CloudProjectCapabilityBinding[] = [];
      if (!result.stale && typeof gateway.listCloudProjectCapabilities === "function") {
        try {
          const capabilities = await gateway.listCloudProjectCapabilities(projectId);
          if (!isCurrentDetail()) return false;
          if (capabilities.projectScopeKey === projectScopeRef.current) {
            loadedCapabilityBindings = capabilities.items;
            setCapabilityBindings(capabilities.items);
            setCapabilityBindingsStatus("ready");
          } else {
            setCapabilityBindingsStatus("error");
          }
        } catch {
          if (!isCurrentDetail()) return false;
          setCapabilityBindings([]);
          setCapabilityBindingsStatus("error");
        }
      } else {
        setCapabilityBindingsStatus(result.stale ? "ready" : "error");
      }
      if (installMissingSkills && loadedCapabilityBindings.some((binding) => binding.kind === "skill")) {
        const skillInstall = await installMissingBoundProjectSkills(loadedCapabilityBindings, gateway);
        if (!isCurrentDetail()) return false;
        if (skillInstall.installed.length > 0) {
          showToast(`已自动安装项目技能：${skillInstall.installed.join("、")}`, "info");
        }
        if (skillInstall.failed.length > 0) {
          showToast(`项目已加入，但以下技能自动安装失败：${skillInstall.failed.join("、")}`, "warn", { durationMs: 6_000 });
        }
      }
      if (!result.stale) {
        const catalogKinds: ProjectCatalogKind[] = ["connector", "expert", "skill"];
        const catalogs = await Promise.allSettled(catalogKinds.map(async (kind) => ({
          kind,
          items: await loadProjectCapabilityCatalog(kind, gateway, experts),
        })));
        if (!isCurrentDetail()) return false;
        setCapabilityCatalogs(Object.fromEntries(catalogs.flatMap((catalog) => catalog.status === "fulfilled"
          ? [[catalog.value.kind, catalog.value.items]]
          : [])));
      }
      if (!result.stale && membersAvailable) {
        try {
          const memberList = await gateway.listCloudProjectMembers(projectId);
          if (!isCurrentDetail()) return false;
          if (memberList.projectScopeKey === projectScopeRef.current) {
            setProjectMembers(memberList.items);
            setCurrentRole(memberList.currentRole);
          }
        } catch {
          if (!isCurrentDetail()) return false;
          setProjectMembers([]);
          setCurrentRole(null);
        }
      }
      return true;
    } catch (error) {
      if (isCurrentDetail()) {
        if (!enteredWorkspace) {
          selectedProjectRef.current = "";
          onSelectedProjectChange?.("");
          setInstructionStatus("idle");
          onWorkspaceChange?.(false);
        }
        setState((current) => ({ ...current, selected: enteredWorkspace ? current.selected : null, message: t("workspace.cloudProjects.detailFailed") }));
        showError("projects.operation", error, t("workspace.cloudProjects.detailFailed"));
      }
      return enteredWorkspace;
    }
  };

  useEffect(() => {
    if (!authenticated || !accountScopeKey || !window.location.hash.startsWith("#project-invite=")) return;
    const ownerScope = accountScopeKey;
    const request = ++inviteRequestRef.current;
    const raw = window.location.hash.slice(1);
    window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}`);
    const params = new URLSearchParams(raw);
    const inviteId = params.get("project-invite") || "";
    const token = params.get("token") || "";
    if (!inviteId || !token) {
      setLiveMessage("邀请链接无效或不完整");
      return;
    }
    const isCurrentInvite = () => inviteRequestRef.current === request && ownerScopeRef.current === ownerScope;
    void (async () => {
      try {
        const result = await gateway.acceptCloudProjectInvite({ inviteId, token });
        if (!isCurrentInvite()) return;
        setLiveMessage(result.status === "accepted" ? "已加入项目" : "邀请状态已恢复");
        await loadJoined("");
        if (!isCurrentInvite() || !result.projectId) return;
        await selectProject(result.projectId, true);
      } catch (error) {
        if (isCurrentInvite()) {
          setLiveMessage("邀请接受失败，请向项目所有者获取新链接");
          showError("projects.operation", error, "邀请接受失败，请向项目所有者获取新链接");
        }
      }
    })();
  }, [accountScopeKey, authenticated, gateway, loadJoined, showError]);

  const resetWorkspaceSelection = (notifyParent: boolean, notifyComposer: boolean) => {
    const leavingProjectId = selectedProjectRef.current;
    selectedProjectRef.current = "";
    if (notifyParent) onSelectedProjectChange?.("");
    instructionRequestRef.current += 1;
    detailRequestRef.current += 1;
    setState((current) => ({ ...current, selected: null, message: "" }));
    setWorkspaceTab("activity");
    setInstructionStatus("idle");
    setInstruction(null);
    setCapabilityBindings([]);
    setCapabilityBindingsStatus("idle");
    setProjectWorkspacePath("");
    setInstructionError("");
    setInstructionNotice("");
    setInstructionPending(false);
    setProjectMembers([]);
    setCurrentRole(null);
    onWorkspaceChange?.(false);
    if (notifyComposer && leavingProjectId) onProjectComposerLeave?.(JSON.stringify(["project-composer-v1", accountScopeKey, leavingProjectId]));
  };

  const leaveWorkspace = () => resetWorkspaceSelection(true, true);

  useLayoutEffect(() => {
    if (!onSelectedProjectChange || selectedProjectId || !selectedProjectRef.current) return;
    resetWorkspaceSelection(false, false);
  }, [onSelectedProjectChange, selectedProjectId]);

  const acceptInstruction = (ownerScope: string, projectId: string, nextInstruction: CloudProjectInstruction, updateDrafts: boolean) => {
    if (ownerScopeRef.current !== ownerScope || selectedProjectRef.current !== projectId || nextInstruction.projectScopeKey !== projectScopeRef.current) return false;
    setInstruction(nextInstruction);
    setInstructionStatus("ready");
    if (updateDrafts) {
      setInstructionHumanDraft(nextInstruction.humanDescription);
      setInstructionAgentDraft(nextInstruction.agentInstruction);
    }
    return true;
  };

  const saveInstruction = async () => {
    const ownerScope = accountScopeKey;
    const selected = visible.selected;
    const authoritative = instruction;
    if (!selected || !authoritative || selected.status !== "active" || visible.stale) return;
    if (!instructionHumanDraft && !instructionAgentDraft) {
      setInstructionError(t("workspace.cloudProjects.instruction.useClear"));
      return;
    }
    setInstructionPending(true);
    setInstructionError("");
    setInstructionNotice("");
    try {
      const nextInstruction = await gateway.editCloudProjectInstruction({
        projectId: selected.projectId,
        expectedVersion: authoritative.version,
        humanDescription: instructionHumanDraft,
        agentInstruction: instructionAgentDraft,
      });
      if (acceptInstruction(ownerScope, selected.projectId, nextInstruction, true)) {
        setInstructionEditing(false);
        setInstructionNotice(t("workspace.cloudProjects.instruction.saved"));
      }
    } catch (error) {
      if (ownerScopeRef.current !== ownerScope || selectedProjectRef.current !== selected.projectId) return;
      const message = clientErrorCode(error) === "CONFLICT"
        ? t("workspace.cloudProjects.instruction.conflictDraft")
        : t("workspace.cloudProjects.instruction.mutationFailed");
      if (clientErrorCode(error) === "CONFLICT") {
        try {
          const authoritativeInstruction = await gateway.getCloudProjectInstruction(selected.projectId);
          acceptInstruction(ownerScope, selected.projectId, authoritativeInstruction, false);
        } catch {
          // Keep the user's draft even when the authoritative refresh fails.
        }
      }
      setInstructionError(message);
      showError("projects.operation", error, message);
    } finally {
      if (ownerScopeRef.current === ownerScope && selectedProjectRef.current === selected.projectId) setInstructionPending(false);
    }
  };

  const clearInstruction = async () => {
    const ownerScope = accountScopeKey;
    const selected = visible.selected;
    const authoritative = instruction;
    if (!selected || !authoritative || selected.status !== "active" || visible.stale) return;
    setInstructionPending(true);
    setInstructionError("");
    setInstructionNotice("");
    try {
      const nextInstruction = await gateway.clearCloudProjectInstruction({ projectId: selected.projectId, expectedVersion: authoritative.version });
      if (acceptInstruction(ownerScope, selected.projectId, nextInstruction, true)) {
        setInstructionEditing(false);
        setInstructionNotice(t("workspace.cloudProjects.instruction.cleared"));
      }
    } catch (error) {
      if (ownerScopeRef.current !== ownerScope || selectedProjectRef.current !== selected.projectId) return;
      const message = clientErrorCode(error) === "CONFLICT"
        ? t("workspace.cloudProjects.instruction.conflictDraft")
        : t("workspace.cloudProjects.instruction.mutationFailed");
      if (clientErrorCode(error) === "CONFLICT") {
        try {
          const authoritativeInstruction = await gateway.getCloudProjectInstruction(selected.projectId);
          acceptInstruction(ownerScope, selected.projectId, authoritativeInstruction, false);
        } catch {
          // Keep the user's draft even when the authoritative refresh fails.
        }
      }
      setInstructionError(message);
      showError("projects.operation", error, message);
    } finally {
      if (ownerScopeRef.current === ownerScope && selectedProjectRef.current === selected.projectId) setInstructionPending(false);
    }
  };

  const startProjectSession = async (tabId: string, workspacePath = "", executionTarget?: "local" | "cloud"): Promise<CloudProjectSession> => {
    const ownerScope = accountScopeKey;
    const selected = visible.selected;
    const authoritative = instruction;
    if (!selected || !authoritative || selected.status !== "active" || visible.stale || !tabId) throw new Error("project session is not ready");
    setInstructionPending(true);
    setInstructionError("");
    setInstructionNotice("");
    try {
      const currentSession = projectSessionRef.current;
      if (currentSession?.projectId === selected.projectId
        && currentSession.tabId === tabId
        && currentSession.instructionVersion === authoritative.version
        && currentSession.contentHash === authoritative.contentHash) return currentSession;
      const session = await gateway.newCloudProjectSession({
        projectId: selected.projectId,
        tabId,
        workspacePath,
        ...(executionTarget ? { executionTarget } : {}),
      });
      if (ownerScopeRef.current !== ownerScope || selectedProjectRef.current !== selected.projectId) throw new Error("project selection changed during session creation");
      const empty = session.instructionVersion === authoritative.version
        && session.contentHash === authoritative.contentHash
        && !authoritative.humanDescription
        && !authoritative.agentInstruction;
      if (session.instructionVersion !== authoritative.version || session.contentHash !== authoritative.contentHash) {
        throw new Error("project instruction changed before session creation");
      }
      if (session.projectId !== selected.projectId || session.tabId !== tabId || !session.sessionPath.trim()) {
        throw new Error("project session returned a different tab or project");
      }
      projectSessionRef.current = session;
      setInstructionNotice(empty
        ? t("workspace.cloudProjects.instruction.sessionCreatedEmpty")
        : t("workspace.cloudProjects.instruction.sessionCreated", { version: session.instructionVersion }));
      return session;
    } catch (error) {
      if (ownerScopeRef.current === ownerScope && selectedProjectRef.current === selected.projectId) {
        const message = t("workspace.cloudProjects.instruction.sessionFailed");
        setInstructionError(message);
        showError("projects.operation", error, message);
      }
      throw error;
    } finally {
      if (ownerScopeRef.current === ownerScope && selectedProjectRef.current === selected.projectId) setInstructionPending(false);
    }
  };

  useEffect(() => {
    if (!selectedProjectId || visible.selected?.projectId === selectedProjectId) return;
    const ownedProject = visible.availability === "available" && visible.items.some((item) => item.projectId === selectedProjectId);
    const joinedProject = visibleJoined.availability === "available" && visibleJoined.items.some((item) => item.projectId === selectedProjectId);
    if (!ownedProject && !joinedProject) return;
    const restoreKey = `${accountScopeKey}:${selectedProjectId}:pending`;
    if (restoredProjectRef.current === restoreKey) return;
    restoredProjectRef.current = restoreKey;
    void selectProject(selectedProjectId, joinedProject).finally(() => {
      if (restoredProjectRef.current === restoreKey) restoredProjectRef.current = "";
    });
  }, [accountScopeKey, selectedProjectId, visible.availability, visible.items, visible.selected, visibleJoined.availability, visibleJoined.items]);

  useEffect(() => {
    const selected = visible.selected;
    if (!selected || visible.stale || !instructionsAvailable || instructionStatus !== "unavailable") return;
    const ownerScope = accountScopeKey;
    const projectId = selected.projectId;
    const retryKey = `${ownerScope}:${projectId}`;
    if (instructionFeatureRetryRef.current === retryKey) return;
    instructionFeatureRetryRef.current = retryKey;
    const instructionRequest = instructionRequestRef.current + 1;
    instructionRequestRef.current = instructionRequest;
    setInstructionStatus("loading");
    setInstructionError("");
    void gateway.getCloudProjectInstruction(projectId).then((nextInstruction) => {
      if (ownerScopeRef.current !== ownerScope
        || selectedProjectRef.current !== projectId
        || instructionRequestRef.current !== instructionRequest
        || nextInstruction.projectScopeKey !== projectScopeRef.current) return;
      setInstruction(nextInstruction);
      setInstructionHumanDraft(nextInstruction.humanDescription);
      setInstructionAgentDraft(nextInstruction.agentInstruction);
      setInstructionStatus("ready");
    }).catch((error) => {
      if (ownerScopeRef.current !== ownerScope
        || selectedProjectRef.current !== projectId
        || instructionRequestRef.current !== instructionRequest) return;
      setInstructionStatus("error");
      setInstructionError(t("workspace.cloudProjects.instruction.loadFailed"));
      showError("projects.operation", error, t("workspace.cloudProjects.instruction.loadFailed"));
    });
  }, [accountScopeKey, gateway, instructionStatus, instructionsAvailable, showError, t, visible.selected, visible.stale]);

  useEffect(() => {
    if (!visible.selected || !restoreFocusCapability) return;
    const frame = requestAnimationFrame(() => {
      document.querySelector<HTMLButtonElement>(`[data-project-capability="${restoreFocusCapability}"]`)?.focus();
      onRestoreFocusConsumed?.();
    });
    return () => cancelAnimationFrame(frame);
  }, [onRestoreFocusConsumed, restoreFocusCapability, visible.selected]);

  const openDialog = (kind: ProjectDialogKind, trigger: HTMLElement, selectedTemplateId?: ProjectTemplateId, targetProject?: CloudProject) => {
    const dialogProject = targetProject || visible.selected;
    setReturnFocus(trigger);
    setDialogError("");
    setDialogNameInvalid(false);
    setActionProject(kind === "edit" || kind === "archive" || kind === "restore" || kind === "delete" ? dialogProject : null);
    setDialog(kind);
    if (kind === "create") {
      setDraftName("");
      setDraftDescription("");
      setTemplateId("");
      setTemplateInstruction("");
      setDraftCapabilities(emptyDraftCapabilities());
    } else if (kind === "template") {
      const nextTemplate = projectTemplates.find((template) => template.id === selectedTemplateId) || projectTemplates[0];
      setDraftName("");
      setDraftDescription("");
      setTemplateId(nextTemplate.id);
      setTemplateInstruction(nextTemplate.instruction);
      setDraftDescription(t(nextTemplate.description));
      setDraftCapabilities(emptyDraftCapabilities());
    } else if (kind === "edit" && dialogProject) {
      setDraftName(dialogProject.name);
      setDraftDescription(dialogProject.description || "");
    }
  };

  const acceptResult = (ownerScope: string, result: CloudProjectResult, selectResult = true) => {
    if (!acceptPage(ownerScope, { projectScopeKey: result.projectScopeKey, items: [], nextCursor: null, stale: result.stale, cachedAt: result.cachedAt })) return false;
    setState((current) => current.ownerScope !== ownerScope ? current : {
      ...current,
      stale: result.stale,
      cachedAt: result.cachedAt,
      selected: selectResult ? result.project : current.selected,
      items: current.items.some((item) => item.projectId === result.project.projectId)
        ? current.items.map((item) => item.projectId === result.project.projectId ? result.project : item)
        : [result.project, ...current.items],
    });
    return true;
  };

  const submitDialog = async () => {
    const ownerScope = accountScopeKey;
    const selected = actionProject || visible.selected;
    if (!dialog) return;
    if ((dialog === "create" || dialog === "template" || dialog === "edit") && !draftName.trim()) {
      setDialogError(t("workspace.cloudProjects.nameRequired"));
      setDialogNameInvalid(true);
      return;
    }
    if ((dialog === "create" || dialog === "template" || dialog === "edit") && Array.from(draftName).length > 15) {
      setDialogError(t("workspace.cloudProjects.nameTooLong"));
      setDialogNameInvalid(true);
      return;
    }
    setPending(true);
    setDialogError("");
    setDialogNameInvalid(false);
    try {
      if (dialog === "create" || dialog === "template") {
        const selectedTemplate = templateOptions.find((template) => template.id === templateId);
        const hasInstruction = Boolean(selectedTemplate || templateInstruction);
        const initialBindings = (["connector", "expert", "skill"] as const).flatMap((capabilityKind) =>
          draftCapabilities[capabilityKind].map((item, position) => ({
            kind: capabilityKind,
            registryRef: item.id,
            position,
            config: {},
          })),
        );
        const result = await gateway.createCloudProject({
          name: draftName,
          descriptionSet: false,
          description: null,
          ...(hasInstruction ? { humanDescription: selectedTemplate?.description || "", agentInstruction: templateInstruction } : {}),
          ...(initialBindings.length > 0 ? { bindings: initialBindings } : {}),
        });
        if (acceptResult(ownerScope, result, false)) {
          if (await selectProject(result.project.projectId, true)) {
            setLiveMessage(t("workspace.cloudProjects.created"));
          }
        }
      } else if (dialog === "edit" && selected) {
        const result = await gateway.patchCloudProject({ projectId: selected.projectId, expectedVersion: selected.version, name: draftName, descriptionSet: true, description: draftDescription || null });
        if (acceptResult(ownerScope, result, selectedProjectRef.current === selected.projectId)) setLiveMessage(t("workspace.cloudProjects.saved"));
      } else if (dialog === "archive" && selected) {
        const result = await gateway.archiveCloudProject({ projectId: selected.projectId, expectedVersion: selected.version });
        if (acceptResult(ownerScope, result)) setLiveMessage(t("workspace.cloudProjects.archivedDone"));
      } else if (dialog === "restore" && selected) {
        const result = await gateway.restoreCloudProject({ projectId: selected.projectId, expectedVersion: selected.version });
        if (acceptResult(ownerScope, result)) setLiveMessage(t("workspace.cloudProjects.restoredDone"));
      } else if (dialog === "delete" && selected) {
        await gateway.deleteCloudProject({ projectId: selected.projectId, expectedVersion: selected.version });
        if (ownerScopeRef.current === ownerScope) {
          const deletedProjectId = selected.projectId;
          if (selectedProjectRef.current === deletedProjectId) leaveWorkspace();
          setState((current) => ({
            ...current,
            selected: current.selected?.projectId === deletedProjectId ? null : current.selected,
            items: current.items.filter((item) => item.projectId !== deletedProjectId),
          }));
          setLiveMessage("项目已永久删除");
        }
      }
      if (ownerScopeRef.current === ownerScope) setDialog(null);
    } catch (error) {
      if (ownerScopeRef.current !== ownerScope) return;
      const code = clientErrorCode(error);
      if (code === "QUOTA_EXCEEDED") {
        const message = t("workspace.cloudProjects.quotaExceeded");
        setDialogError(message);
        showError("projects.operation", error, message, {
          durationMs: 6_000,
          ...(onOpenPayments ? {
            actionLabel: t("workspace.cloudProjects.viewPlans"),
            onAction: () => {
              setDialog(null);
              onOpenPayments();
            },
          } : {}),
        });
        return;
      } else if (code === "CONFLICT" && selected) {
        try {
          const authoritative = await gateway.getCloudProject(selected.projectId);
          acceptResult(ownerScope, authoritative, selectedProjectRef.current === selected.projectId);
        } catch {
          // Keep the local draft even if the conflict readback also fails.
        }
        const message = t("workspace.cloudProjects.conflictDraft");
        setDialogError(message);
        showError("projects.operation", error, message);
      } else if (code === "NETWORK_UNAVAILABLE") {
        const message = t("workspace.cloudProjects.unknownDraft");
        setDialogError(message);
        showError("projects.operation", error, message);
      } else {
        const message = t("workspace.cloudProjects.mutationFailed");
        setDialogError(message);
        showError("projects.operation", error, message);
      }
    } finally {
      if (ownerScopeRef.current === ownerScope) setPending(false);
    }
  };

  const openJoinDialog = (trigger: HTMLElement) => {
    setJoinReturnFocus(trigger);
    setJoinProjectId("");
    setJoinError("");
    setJoinInvalid(false);
    setJoinDialogOpen(true);
  };

  const submitJoin = async () => {
    const projectId = joinProjectId.trim();
    if (!projectId) {
      setJoinInvalid(true);
      setJoinError(t("workspace.cloudProjects.dialog.join.required"));
      return;
    }
    const ownerScope = accountScopeKey;
    setJoinPending(true);
    setJoinError("");
    setJoinInvalid(false);
    try {
      const membership = await gateway.joinCloudProject({ projectId });
      if (ownerScopeRef.current !== ownerScope || membership.projectScopeKey !== projectScopeRef.current) return;
      setQuery("");
      setAppliedQuery("");
      const [, refreshed] = await Promise.all([load(""), loadJoined("")]);
      if (ownerScopeRef.current !== ownerScope) return;
      if (!refreshed) {
        setJoinError(t("workspace.cloudProjects.joinRefreshFailed"));
        return;
      }
      setJoinDialogOpen(false);
      setJoinProjectId("");
      setLiveMessage(t("workspace.cloudProjects.joined"));
      showToast(t("workspace.cloudProjects.joined"), "info");
      await selectProject(membership.projectId || projectId, true);
    } catch (error) {
      if (ownerScopeRef.current !== ownerScope) return;
      const code = clientErrorCode(error);
      let message: string;
      if (code === "VALIDATION_FAILED") {
        setJoinInvalid(true);
        message = t("workspace.cloudProjects.joinValidationFailed");
      } else if (code === "NOT_FOUND") {
        message = t("workspace.cloudProjects.joinNotFound");
      } else if (code === "AUTH_REQUIRED" || code === "UNAUTHORIZED") {
        message = t("workspace.cloudProjects.signIn");
      } else {
        message = t("workspace.cloudProjects.joinFailed");
      }
      setJoinError(message);
      showError("projects.operation", error, message);
    } finally {
      if (ownerScopeRef.current === ownerScope) setJoinPending(false);
    }
  };

  const selected = visible.selected;
  const selectedIsJoined = Boolean(selected && visibleJoined.items.some((project) => project.projectId === selected.projectId));
  const ownerOnly = selectedIsJoined || currentRole === "member";
  const sessionKey = selected ? JSON.stringify(["project-composer-v1", accountScopeKey, selected.projectId]) : "";
  const instructionPanel = selected && (instructionsAvailable || visible.stale) ? (
    <section className="cloud-projects__instruction" aria-labelledby="cloud-project-instruction-title" data-instruction-state={instructionStatus}>
      <div className="cloud-projects__instruction-head">
        <h4 id="cloud-project-instruction-title">{t("workspace.cloudProjects.instruction.title")}</h4>
        <div><button ref={instructionButtonRef} type="button" aria-label="编辑项目指令" disabled={instructionPending || instructionStatus !== "ready"} onClick={() => { setInstructionEditing(true); setInstructionError(""); setInstructionNotice(""); }}><Plus size={15} aria-hidden="true" /></button></div>
      </div>
      {instructionStatus === "loading" ? <p role="status">{t("common.loading")}</p> : null}
      {instructionStatus === "unavailable" ? <div role="status"><strong>{surfacePolicy.instruction === "requires_online" ? "项目指令必须联网" : t("workspace.cloudProjects.instruction.cacheUnavailable")}</strong>{surfacePolicy.instruction === "requires_online" ? <p>{t("workspace.cloudProjects.instruction.cacheUnavailable")}</p> : null}</div> : null}
      {instructionStatus === "error" ? <p role="alert" className="cloud-projects__inline-error">{instructionError}</p> : null}
      {instructionStatus === "ready" && instruction ? <><p className="cloud-projects__instruction-preview">{instructionAgentDraft.trim() || instructionHumanDraft.trim() || "尚未添加项目指令"}</p>{selected.status === "archived" ? <p className="cloud-projects__instruction-hint">{t("workspace.cloudProjects.instruction.archivedReadOnly")}</p> : null}{instructionNotice ? <p role="status" className="cloud-projects__instruction-notice">{instructionNotice}</p> : null}</> : null}
    </section>
  ) : null;

  return (
    <section className={`cloud-projects${selected ? " cloud-projects--workspace" : ""}`} aria-labelledby={selected ? "cloud-project-workspace-title" : pendingSelectedProjectId ? undefined : "cloud-projects-title"} aria-label={pendingSelectedProjectId ? t("common.loading") : undefined} data-cloud-availability={visible.availability} data-project-view={selected ? "workspace" : pendingSelectedProjectId ? "workspace-loading" : "directory"}>
      {pendingSelectedProjectId ? <CloudState Icon={LoaderCircle} text={t("common.loading")} busy /> : selected ? <>
        <header className="cloud-projects__workspace-header">
          <div className="cloud-projects__workspace-identity">
            <button type="button" className="cloud-projects__project-link" onClick={leaveWorkspace}><FolderKanban size={16} aria-hidden="true" /><span>{t("workspace.cloudProjects.heroTitle")}</span></button>
            <span className="cloud-projects__breadcrumb-separator" aria-hidden="true">/</span>
            <h2 id="cloud-project-workspace-title" title={selected.name}>{selected.name}</h2>
          </div>
          <div className="cloud-projects__workspace-actions">
            {!ownerOnly ? <div className="cloud-projects__invite-state">
              <button ref={inviteButtonRef} type="button" className="cloud-projects__invite" disabled={surfacePolicy.members === "requires_online"} aria-describedby={!membersAvailable || surfacePolicy.members === "requires_online" ? "cloud-project-invite-unavailable" : undefined} onClick={() => setMemberDrawerOpen(true)}><Users size={16} aria-hidden="true" />{t("workspace.cloudProjects.invite")}</button>
              {!membersAvailable || surfacePolicy.members === "requires_online" ? <span id="cloud-project-invite-unavailable">{surfacePolicy.members === "requires_online" ? "成员管理必须联网" : t("workspace.cloudProjects.inviteUnavailable")}</span> : null}
            </div> : null}
            {!ownerOnly ? <ProjectLifecycleMenu project={selected} disabled={visible.stale} onAction={(kind, trigger) => openDialog(kind, trigger)} /> : null}
          </div>
        </header>
        <nav className="cloud-projects__tabs" aria-label={t("workspace.cloudProjects.views")}>
          {(["activity", "plan", "assets"] as const).map((tab) => (
            <button type="button" key={tab} aria-current={workspaceTab === tab ? "page" : undefined} onClick={() => setWorkspaceTab(tab)}>
              {{ activity: t("workspace.cloudProjects.tabs.activity"), plan: t("workspace.cloudProjects.tabs.plan"), assets: t("workspace.cloudProjects.tabs.assets") }[tab]}
            </button>
          ))}
        </nav>
        {visible.stale ? <p className="cloud-projects__stale" role="status">{t("workspace.cloudProjects.stale", { time: visible.cachedAt ? formatProjectDateTime(visible.cachedAt) : t("workspace.cloudProjects.staleUnknownTime") })}</p> : null}
        {visible.message ? <p className="cloud-projects__inline-error" role="alert">{visible.message}</p> : null}
        <div ref={workspaceGridRef} className={`cloud-projects__workspace-grid${configurationCollapsed ? " cloud-projects__workspace-grid--configuration-hidden" : ""}${configurationCompact ? " cloud-projects__workspace-grid--configuration-compact" : ""}${configurationNarrow ? " cloud-projects__workspace-grid--configuration-narrow" : ""}`}>
          <div className="cloud-projects__workspace-center">
            <main className={`cloud-projects__workspace-view${workspaceTab === "assets" ? " cloud-projects__workspace-view--assets" : ""}`}>
              <>
                {workspaceTab === "activity" && surfacePolicy.activity === "requires_online" ? <RequiresOnlineProjectView Icon={MessageSquare} title="项目动态必须联网" /> : null}
                {workspaceTab === "activity" && surfacePolicy.activity === "available" && activityAvailable ? <ProjectActivityPanel projectId={selected.projectId} projectScopeKey={visible.projectScope} gateway={gateway} members={projectMembers} readOnly={selected.status !== "active"} /> : null}
                {workspaceTab === "activity" && surfacePolicy.activity === "available" && !activityAvailable ? <ActivityUnavailableShell /> : null}
                {workspaceTab === "plan" && surfacePolicy.workItems === "requires_online" ? <RequiresOnlineProjectView Icon={ListChecks} title="事项必须联网" /> : null}
                {workspaceTab === "plan" && surfacePolicy.workItems === "available" && workItemsAvailable && typeof gateway.listCloudProjectWorkItems === "function" ? <WorkItemBoard accountScopeKey={accountScopeKey} projectScopeKey={visible.projectScope} projectId={selected.projectId} projectName={selected.name} archived={selected.status === "archived"} stale={false} gateway={gateway} /> : null}
                {workspaceTab === "plan" && surfacePolicy.workItems === "available" && !workItemsAvailable ? <UnavailableProjectView Icon={ListChecks} title={t("workspace.cloudProjects.workItemsUnavailable")} hint={t("workspace.cloudProjects.workItemsUnavailableHint")} /> : null}
                {workspaceTab === "assets" && surfacePolicy.assets === "requires_online" ? <RequiresOnlineProjectView Icon={Boxes} title="项目资产必须联网" /> : null}
                {workspaceTab === "assets" && surfacePolicy.assets === "available" && assetsAvailable ? <ProjectAssetsPanel projectId={selected.projectId} projectScopeKey={visible.projectScope} gateway={gateway} readOnly={selected.status !== "active"} onUpgrade={onOpenPayments} /> : null}
                {workspaceTab === "assets" && surfacePolicy.assets === "available" && !assetsAvailable ? <AssetsUnavailableShell /> : null}
              </>
            </main>
            <footer className={`cloud-projects__composer${workspaceTab === "assets" ? " cloud-projects__composer--assets" : ""}`} data-project-composer data-project-composer-reused={renderComposer ? "true" : "false"}>
              {renderComposer ? renderComposer({
                sessionKey,
                projectId: selected.projectId,
                projectName: selected.name,
                workspacePath: projectWorkspacePath,
                capabilityBindings,
                instructionVersion: instruction?.version ?? 0,
                contentHash: instruction?.contentHash ?? "",
                projectWritable: selected.status === "active" && !visible.stale && capabilityBindingsStatus === "ready",
                projectDisabledReason: selected.status !== "active"
                  ? t("workspace.cloudProjects.instruction.archivedReadOnly")
                  : visible.stale
                    ? t("workspace.cloudProjects.staleUnknownTime")
                    : capabilityBindingsStatus === "error"
                      ? "项目能力加载失败，请重新打开项目后重试。"
                      : capabilityBindingsStatus === "loading"
                        ? "正在加载项目能力。"
                        : undefined,
                onCreateSession: startProjectSession,
                onCapabilitiesChanged: (items) => {
                  setCapabilityBindings(items);
                  setCapabilityBindingsStatus("ready");
                },
              }) : <>
                <div><strong>{t("workspace.cloudProjects.composerTitle")}</strong><span>{t("workspace.cloudProjects.composerHint")}</span></div>
                {instructionsAvailable ? <button type="button" disabled>{t("workspace.cloudProjects.instruction.newSession")}</button> : null}
              </>}
            </footer>
          </div>
          {configurationCollapsed ? <button ref={configurationToggleRef} type="button" className="cloud-projects__configuration-edge-toggle" aria-label="展开项目配置" aria-expanded="false" aria-controls="cloud-project-configuration" onClick={() => { configurationAutoCollapsedRef.current = false; setConfigurationCollapsed(false); requestAnimationFrame(() => configurationCloseRef.current?.focus()); }}><PanelRightOpen size={17} aria-hidden="true" /></button> : <aside id="cloud-project-configuration" className="cloud-projects__rail" aria-label={t("workspace.cloudProjects.configuration")}>
            <div className="cloud-projects__rail-header">
              <h3><Settings2 size={17} aria-hidden="true" />{t("workspace.cloudProjects.configuration")}</h3>
              <button ref={configurationCloseRef} type="button" className="cloud-projects__rail-toggle" aria-label="收起项目配置" aria-expanded="true" aria-controls="cloud-project-configuration" onClick={() => { configurationAutoCollapsedRef.current = false; setConfigurationCollapsed(true); requestAnimationFrame(() => configurationToggleRef.current?.focus()); }}><PanelRightClose size={17} aria-hidden="true" /></button>
            </div>
            <div className="cloud-projects__rail-content">
              {instructionPanel}
              <div className="cloud-projects__capability-heading"><strong>项目能力</strong></div>
              {(["connector", "expert", "skill"] as const).map((kind) => (
                onOpenCapability
                  ? <CapabilityConfigurationCard key={kind} kind={kind} label={t(`workspace.cloudProjects.extensions.${kind}`)} items={capabilityBindings.filter((binding) => binding.kind === kind).map((binding) => {
                    const catalogItem = capabilityCatalogs[kind]?.find((item) => item.id === binding.registryRef);
                    return {
                      id: binding.bindingId,
                      registryRef: binding.registryRef,
                      name: catalogItem?.name || binding.registryRef,
                      iconUrl: catalogItem?.iconUrl,
                      disabled: binding.publicDisabled,
                      connected: kind === "connector" ? catalogItem?.available : undefined,
                    };
                  })} readOnly={selected.status !== "active" || visible.stale} requiresOnline={surfacePolicy.capabilities === "requires_online"} onOpen={(nextKind, trigger) => onOpenCapability(nextKind, trigger, {
                    sessionKey,
                    projectId: selected.projectId,
                    onCapabilitiesChanged: setCapabilityBindings,
                    onCapabilityCatalogChanged: (catalogKind, items) => setCapabilityCatalogs((current) => ({ ...current, [catalogKind]: items })),
                  })} />
                  : <UnavailableConfigurationCard key={kind} label={t(`workspace.cloudProjects.extensions.${kind}`)} />
              ))}
            </div>
          </aside>}
        </div>
        <ProjectMemberDrawer open={memberDrawerOpen && membersAvailable} projectId={selected.projectId} projectScopeKey={visible.projectScope} gateway={gateway} onClose={() => closeMemberDrawer()} />
        {memberDrawerOpen && !membersAvailable ? <div className="cloud-projects__member-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeMemberDrawer(); }}><aside className="cloud-projects__member-drawer" role="dialog" aria-modal="true" aria-labelledby="cloud-project-members-title" data-project-member-drawer data-owner-relation="current-account" onKeyDown={(event) => { if (event.key === "Tab") { event.preventDefault(); memberDrawerCloseRef.current?.focus(); } }}><header><div><span>{t("workspace.cloudProjects.members.eyebrow")}</span><h3 id="cloud-project-members-title">{t("workspace.cloudProjects.members.title")}</h3></div><button ref={memberDrawerCloseRef} type="button" aria-label={t("common.close")} onClick={() => closeMemberDrawer()}><X size={18} aria-hidden="true" /></button></header><div className="cloud-projects__member-owner"><span className="cloud-projects__member-avatar" aria-hidden="true"><Users size={18} /></span><div><strong>{t("workspace.cloudProjects.members.currentAccountOwner")}</strong><small>{t("workspace.cloudProjects.members.ownerHint")}</small></div></div><p className="cloud-projects__member-unavailable">{t("workspace.cloudProjects.inviteUnavailable")}</p><button type="button" disabled>{t("workspace.cloudProjects.members.inviteDisabled")}</button></aside></div> : null}
      </> : <>
        <section className="cloud-projects__hero" data-project-hero>
          <div><h1>{t("workspace.cloudProjects.heroTitle")}</h1><p>{t("workspace.cloudProjects.heroSubtitle")}</p><div className="cloud-projects__hero-actions"><button type="button" className="cloud-projects__primary-action" disabled={visible.availability !== "available" || visible.stale} onClick={(event) => openDialog("create", event.currentTarget)}><Plus size={16} aria-hidden="true" />{t("workspace.cloudProjects.create")}</button><button type="button" className="cloud-projects__join-action" disabled={visible.availability !== "available" || visible.stale} onClick={(event) => openJoinDialog(event.currentTarget)}><UserPlus size={16} aria-hidden="true" />{t("workspace.cloudProjects.join")}</button></div></div>
          <img src={new URL("../../../assets/projects/project-team-collaboration.png", import.meta.url).href} alt="" width="629" height="210" />
        </section>
        <div className="project-directory__section-head cloud-projects__head">
          <div className="project-directory__section-title"><h2 id="cloud-projects-title">{t("workspace.cloudProjects.myProjects")}</h2></div>
          {visible.availability === "available" ? <div className="cloud-projects__directory-tools">
            <form className="project-directory__search cloud-projects__search" role="search" onSubmit={(event) => { event.preventDefault(); if (!searchComposingRef.current) commitProjectSearch(); }}><Search size={16} aria-hidden="true" /><input type="search" value={query} onChange={(event) => setQuery(event.currentTarget.value)} onBlur={commitProjectSearch} onCompositionStart={() => { searchComposingRef.current = true; }} onCompositionEnd={() => { searchComposingRef.current = false; }} onKeyDown={(event) => { if (event.key === "Enter" && (event.nativeEvent.isComposing || searchComposingRef.current)) event.preventDefault(); }} aria-label={t("workspace.cloudProjects.search")} placeholder={t("workspace.cloudProjects.searchPlaceholder")} /></form>
            <button type="button" className="cloud-projects__quiet-action" onClick={() => { void load(appliedQuery); void loadJoined(appliedQuery); }} aria-label={t("workspace.cloudProjects.refresh")} disabled={!authenticated}><RefreshCw size={15} aria-hidden="true" /></button>
          </div> : null}
        </div>
        {!authenticated ? <CloudState Icon={Cloud} text={t("workspace.cloudProjects.signIn")} /> : null}
        {visible.availability === "loading" ? <CloudState Icon={LoaderCircle} text={t("common.loading")} busy /> : null}
        {visible.availability === "unavailable" ? <CloudState Icon={Cloud} text={t("workspace.cloudProjects.unavailable")} /> : null}
        {visible.availability === "error" ? <CloudState Icon={Cloud} text={visible.message} retry={() => void load(appliedQuery)} /> : null}
        {visible.availability === "available" ? <>
          {visible.stale ? <p className="cloud-projects__stale" role="status">{t("workspace.cloudProjects.stale", { time: visible.cachedAt ? formatProjectDateTime(visible.cachedAt) : t("workspace.cloudProjects.staleUnknownTime") })}</p> : null}
          {visible.message ? <p className="cloud-projects__inline-error" role="alert">{visible.message}</p> : null}
          {visible.items.length === 0 ? <CloudState Icon={Cloud} text={appliedQuery ? t("workspace.cloudProjects.searchEmpty") : t("workspace.cloudProjects.empty")} /> : <ul className="cloud-projects__list" aria-label={t("workspace.cloudProjects.listLabel")}>{visible.items.map((item) => <li key={item.projectId}><button type="button" className="cloud-projects__card" onClick={() => void selectProject(item.projectId)} aria-label={t("workspace.cloudProjects.open", { name: item.name })}><span className="cloud-projects__card-icon"><FolderKanban size={20} aria-hidden="true" /></span><span className="cloud-projects__card-copy"><strong>{item.name}</strong><small className="cloud-projects__card-meta"><span>{formatProjectDateTime(item.updatedAt)}</span><span aria-hidden="true">·</span><span>{t("workspace.cloudProjects.version", { version: item.version })}</span></small></span>{item.status === "archived" ? <span className="cloud-projects__status"><Archive size={13} aria-hidden="true" />{t("workspace.cloudProjects.archived")}</span> : null}</button><ProjectCardMenu projectName={item.name} disabled={visible.stale} onRename={(trigger) => openDialog("edit", trigger, undefined, item)} onDelete={(trigger) => openDialog("delete", trigger, undefined, item)} /></li>)}</ul>}
          {visible.nextCursor ? <button type="button" className="cloud-projects__more" disabled={loadingMore} onClick={() => void load(appliedQuery, visible.nextCursor || "", true)}>{t("workspace.cloudProjects.loadMore")}</button> : null}
        </> : null}
        {authenticated ? <section className="cloud-projects__joined" aria-labelledby="cloud-projects-joined-title">
          <div className="project-directory__section-head cloud-projects__joined-head"><h2 id="cloud-projects-joined-title">{t("workspace.cloudProjects.myJoinedProjects")}</h2></div>
          {visibleJoined.availability === "loading" ? <CloudState Icon={LoaderCircle} text={t("common.loading")} busy /> : null}
          {visibleJoined.availability === "unavailable" ? <CloudState Icon={Users} text={t("workspace.cloudProjects.joinedUnavailable")} /> : null}
          {visibleJoined.availability === "error" ? <CloudState Icon={Users} text={visibleJoined.message} retry={() => void loadJoined(appliedQuery)} /> : null}
          {visibleJoined.availability === "available" ? <>
            {visibleJoined.items.length === 0 ? <CloudState Icon={Users} text={appliedQuery ? t("workspace.cloudProjects.joinedSearchEmpty") : t("workspace.cloudProjects.joinedEmpty")} /> : <ul className="cloud-projects__list cloud-projects__joined-list" aria-label={t("workspace.cloudProjects.joinedListLabel")}>{visibleJoined.items.map((item) => <li key={item.projectId}><button type="button" className="cloud-projects__card cloud-projects__card--joined" onClick={() => void selectProject(item.projectId, true)} aria-label={t("workspace.cloudProjects.open", { name: item.name })}><span className="cloud-projects__card-icon"><Users size={20} aria-hidden="true" /></span><span className="cloud-projects__card-copy"><strong>{item.name}</strong><small className="cloud-projects__card-meta"><span>{formatProjectDateTime(item.updatedAt)}</span><span aria-hidden="true">·</span><span>{t("workspace.cloudProjects.version", { version: item.version })}</span></small></span>{item.status === "archived" ? <span className="cloud-projects__status"><Archive size={13} aria-hidden="true" />{t("workspace.cloudProjects.archived")}</span> : null}</button></li>)}</ul>}
            {visibleJoined.nextCursor ? <button type="button" className="cloud-projects__more" disabled={joinedLoadingMore} onClick={() => void loadJoined(appliedQuery, visibleJoined.nextCursor || "", true)}>{t("workspace.cloudProjects.loadMoreJoined")}</button> : null}
          </> : null}
        </section> : null}
        {authenticated ? (
          <section className="project-directory__section project-directory__section--templates" aria-labelledby="project-directory-starters-title">
            <div className="project-directory__section-head">
              <h2 id="project-directory-starters-title">{t("workspace.projects.starters")}</h2>
            </div>
            <div className="project-directory__starter-grid">
              {projectTemplates.map(({ id, title, description, Icon }) => (
                <button className="project-directory__starter" type="button" key={id} disabled={visible.availability !== "available" || visible.stale} onClick={(event) => openDialog("template", event.currentTarget, id)}>
                  <span className="project-directory__starter-icon"><Icon size={18} aria-hidden="true" /></span>
                  <span className="project-directory__starter-copy"><strong>{t(title)}</strong><small>{t(description)}</small></span>
                  <ArrowRight size={16} aria-hidden="true" />
                </button>
              ))}
            </div>
          </section>
        ) : null}
      </>}
      <div className="sr-only" role="status" aria-live="polite">{liveMessage}</div>
      {dialog && authenticated && state.ownerScope === accountScopeKey ? (
        <ProjectActionDialog
          kind={dialog}
          name={draftName}
          description={draftDescription}
          pending={pending}
          suspended={Boolean(draftCapabilityKind)}
          error={dialogError}
          nameInvalid={dialogNameInvalid}
          returnFocus={returnFocus}
          templates={dialog === "create" || dialog === "template" ? templateOptions : undefined}
          templateId={dialog === "create" || dialog === "template" ? templateId : undefined}
          templateInstruction={dialog === "create" || dialog === "template" ? templateInstruction : undefined}
          capabilities={dialog === "create" || dialog === "template" ? draftCapabilities : undefined}
          onNameChange={(value) => {
            setDraftName(value);
            if (dialogNameInvalid) {
              setDialogNameInvalid(false);
              setDialogError("");
            }
          }}
          onDescriptionChange={setDraftDescription}
          onTemplateChange={(value) => {
            if (value === templateId) return;
            if (!value) {
              setTemplateId("");
              setTemplateInstruction("");
              setDraftDescription("");
              return;
            }
            const next = projectTemplates.find((template) => template.id === value);
            if (!next) return;
            setTemplateId(next.id);
            setTemplateInstruction(next.instruction);
            setDraftDescription(t(next.description));
          }}
          onTemplateInstructionChange={setTemplateInstruction}
          onOpenCapability={(kind, trigger) => { draftCapabilityTriggerRef.current = trigger; setDraftCapabilityKind(kind); }}
          onRemoveCapability={(kind, id) => setDraftCapabilities((current) => ({ ...current, [kind]: current[kind].filter((item) => item.id !== id) }))}
          onClose={() => { setDraftCapabilityKind(null); setDialog(null); }}
          onSubmit={() => void submitDialog()}
        />
      ) : null}
      {joinDialogOpen && authenticated && state.ownerScope === accountScopeKey ? (
        <ProjectJoinDialog
          projectId={joinProjectId}
          pending={joinPending}
          error={joinError}
          invalid={joinInvalid}
          returnFocus={joinReturnFocus}
          onProjectIdChange={(value) => {
            setJoinProjectId(value);
            if (joinInvalid || joinError) {
              setJoinInvalid(false);
              setJoinError("");
            }
          }}
          onClose={() => setJoinDialogOpen(false)}
          onSubmit={() => void submitJoin()}
        />
      ) : null}
      {draftCapabilityKind && dialog && (dialog === "create" || dialog === "template") ? (
        <ProjectCapabilityDialog
          kind={draftCapabilityKind}
          projectId=""
          gateway={gateway}
          experts={experts}
          mode="draft"
          draftSelectedItems={draftCapabilities[draftCapabilityKind]}
          returnFocus={draftCapabilityTriggerRef.current}
          onClose={() => setDraftCapabilityKind(null)}
          onDraftApplied={(items) => setDraftCapabilities((current) => ({ ...current, [draftCapabilityKind]: items }))}
          onCatalogLoaded={(kind, items) => setCapabilityCatalogs((current) => ({ ...current, [kind]: items }))}
        />
      ) : null}
      {instructionEditing && selected && instruction ? <ProjectInstructionDialog
        humanDescription={instructionHumanDraft}
        agentInstruction={instructionAgentDraft}
        pending={instructionPending}
        error={instructionError}
        readOnly={selected.status !== "active" || visible.stale}
        returnFocus={instructionButtonRef.current}
        onAgentInstructionChange={setInstructionAgentDraft}
        onSave={() => void saveInstruction()}
        onClear={() => void clearInstruction()}
        onClose={() => { setInstructionHumanDraft(instruction.humanDescription); setInstructionAgentDraft(instruction.agentInstruction); setInstructionEditing(false); setInstructionError(""); }}
      /> : null}
    </section>
  );
}

function UnavailableProjectView({ Icon, title, hint, compact = false }: { Icon: typeof Cloud; title: string; hint: string; compact?: boolean }) {
  return <section className={`cloud-projects__unavailable${compact ? " cloud-projects__unavailable--compact" : ""}`} aria-disabled="true"><Icon size={compact ? 18 : 30} aria-hidden="true" /><div><strong>{title}</strong><p>{hint}</p></div></section>;
}

function RequiresOnlineProjectView({ Icon, title }: { Icon: typeof Cloud; title: string }) {
  return <UnavailableProjectView Icon={Icon} title={title} hint="恢复网络并刷新项目后可使用。" />;
}

function ActivityUnavailableShell() {
  const t = useT();
  return <section className="cloud-projects__business-shell" data-project-activity-shell><header className="cloud-projects__business-toolbar"><div><strong>{t("workspace.cloudProjects.activity.title")}</strong><span>{t("workspace.cloudProjects.activity.scope")}</span></div><button type="button" disabled><Plus size={15} aria-hidden="true" />{t("workspace.cloudProjects.activity.post")}</button></header><UnavailableProjectView Icon={MessageSquare} title={t("workspace.cloudProjects.activityUnavailable")} hint={t("workspace.cloudProjects.activityUnavailableHint")} compact /></section>;
}

function AssetsUnavailableShell() {
  const t = useT();
  return <section className="cloud-projects__business-shell" data-project-assets-shell><header className="cloud-projects__business-toolbar"><div><strong>{t("workspace.cloudProjects.assets.title")}</strong><span>{t("workspace.cloudProjects.assets.scope")}</span></div><button type="button" disabled><Plus size={15} aria-hidden="true" />{t("workspace.cloudProjects.assets.upload")}</button></header><div className="cloud-projects__asset-head" aria-hidden="true"><span>{t("workspace.cloudProjects.assets.name")}</span><span>{t("workspace.cloudProjects.assets.type")}</span><span>{t("workspace.cloudProjects.assets.updated")}</span></div><UnavailableProjectView Icon={Boxes} title={t("workspace.cloudProjects.assetsUnavailable")} hint={t("workspace.cloudProjects.assetsUnavailableHint")} compact /></section>;
}

function UnavailableConfigurationCard({ label }: { label: string }) {
  const t = useT();
  return <section className="cloud-projects__configuration-card cloud-projects__configuration-card--unavailable" aria-disabled="true"><div><strong>{label}</strong><span>{t("workspace.cloudProjects.extensions.unavailable")}</span></div><p>{t("workspace.cloudProjects.extensionsUnavailableHint")}</p></section>;
}

function CapabilityConfigurationCard({ kind, label, items, readOnly, requiresOnline, onOpen }: { kind: ProjectCapabilityKind; label: string; items: Array<{ id: string; registryRef: string; name: string; iconUrl?: string; disabled: boolean; connected?: boolean }>; readOnly: boolean; requiresOnline: boolean; onOpen: (kind: ProjectCapabilityKind, trigger: HTMLButtonElement) => void }) {
  return <section className="cloud-projects__configuration-card cloud-projects__configuration-card--capability">
    <div className="cloud-projects__capability-card-head">
      <strong>{label} {items.length}</strong>
      <button type="button" disabled={readOnly || requiresOnline} data-project-capability={kind} aria-label={`添加${label}`} title={readOnly ? `当前项目不可配置${label}` : requiresOnline ? `${label}必须联网` : `添加${label}`} onClick={(event) => onOpen(kind, event.currentTarget)}><Plus size={16} aria-hidden="true" /></button>
    </div>
    <div className="cloud-projects__capability-icons">
      {items.length ? items.map((item) => {
        const unconnected = kind === "connector" && item.connected === false;
        return <span key={item.id} className={`cloud-projects__capability-icon${unconnected ? " is-unconnected" : ""}`} role="img" aria-label={`${label} ${item.name}${unconnected ? "，尚未连接" : ""}`} aria-disabled={item.disabled || undefined} title={unconnected ? `${item.name} · 尚未连接` : item.name}><ProjectCapabilityIcon kind={kind} iconUrl={item.iconUrl} /></span>;
      }) : <span className="cloud-projects__capability-empty">尚未添加</span>}
    </div>
  </section>;
}

function CloudState({ Icon, text, busy = false, retry }: { Icon: typeof Cloud; text: string; busy?: boolean; retry?: () => void }) {
  const t = useT();
  return (
    <div className="cloud-projects__state" role="status" aria-live="polite" aria-busy={busy || undefined}>
      <Icon size={22} aria-hidden="true" />
      <span>{text}</span>
      {retry ? <button type="button" onClick={retry}>{t("common.retry")}</button> : null}
    </div>
  );
}
