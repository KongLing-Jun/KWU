import { createContext, useContext } from "react";
import { app, type AppBindings } from "./bridge";

export interface WorkspaceFileScope {
  tabId?: string;
  workspaceRoot?: string;
  allowActiveWorkspace?: boolean;
}

export type WorkspaceFileOwner =
  | { kind: "tab"; ownerKey: string; tabId: string }
  | { kind: "workspace"; ownerKey: string; workspaceRoot: string }
  | { kind: "active"; ownerKey: "active" }
  | { kind: "none"; ownerKey: "none" };

export interface WorkspaceFilePathState {
  ownerKey: string;
  paths: ReadonlySet<string>;
}

const noWorkspaceFilePaths = new Set<string>();

function nonblank(value: string | undefined): string {
  return value?.trim() ?? "";
}

export function resolveWorkspaceFileOwner(explicitTabId: string | undefined, scope: WorkspaceFileScope): WorkspaceFileOwner {
  const explicitTab = nonblank(explicitTabId);
  if (explicitTab) return { kind: "tab", ownerKey: `tab:${explicitTab}`, tabId: explicitTab };
  const workspaceRoot = nonblank(scope.workspaceRoot);
  if (workspaceRoot) return { kind: "workspace", ownerKey: `workspace:${workspaceRoot}`, workspaceRoot };
  const scopedTab = nonblank(scope.tabId);
  if (scopedTab) return { kind: "tab", ownerKey: `tab:${scopedTab}`, tabId: scopedTab };
  if (scope.allowActiveWorkspace === true) return { kind: "active", ownerKey: "active" };
  return { kind: "none", ownerKey: "none" };
}

export function visibleWorkspaceFilePathKeys(state: WorkspaceFilePathState, ownerKey: string): ReadonlySet<string> {
  return state.ownerKey === ownerKey ? state.paths : noWorkspaceFilePaths;
}

export function readWorkspaceFileForOwner(owner: WorkspaceFileOwner, path: string): ReturnType<AppBindings["ReadFile"]> {
  if (owner.kind === "tab") return app.ReadFileForTab(owner.tabId, path);
  if (owner.kind === "workspace") return app.ReadFileForWorkspace(owner.workspaceRoot, path);
  if (owner.kind === "active") return app.ReadFile(path);
  return Promise.reject(new Error("workspace file owner is unavailable"));
}

export function openWorkspacePathForOwner(owner: WorkspaceFileOwner, path: string): ReturnType<AppBindings["OpenWorkspacePath"]> {
  if (owner.kind === "tab") return app.OpenWorkspacePathForTab(owner.tabId, path);
  if (owner.kind === "workspace") return app.OpenWorkspacePathForWorkspace(owner.workspaceRoot, path);
  if (owner.kind === "active") return app.OpenWorkspacePath(path);
  return Promise.reject(new Error("workspace file owner is unavailable"));
}

export const WorkspaceFileScopeContext = createContext<WorkspaceFileScope>({});

export function useWorkspaceFileScope(): WorkspaceFileScope {
  return useContext(WorkspaceFileScopeContext);
}
