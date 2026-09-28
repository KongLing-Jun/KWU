import { useCallback, useRef, useState } from "react";
import type { CloudProjectSession } from "../model/projectTypes";

export type ProjectWorkspaceTab = "activity" | "plan" | "assets";

export function useProjectWorkspaceController() {
  const inviteButtonRef = useRef<HTMLButtonElement | null>(null);
  const memberDrawerCloseRef = useRef<HTMLButtonElement | null>(null);
  const projectSessionRef = useRef<CloudProjectSession | null>(null);
  const restoredProjectRef = useRef("");
  const [workspaceTab, setWorkspaceTab] = useState<ProjectWorkspaceTab>("activity");
  const [memberDrawerOpen, setMemberDrawerOpen] = useState(false);

  const closeMemberDrawer = useCallback((restoreFocus = true) => {
    setMemberDrawerOpen(false);
    if (restoreFocus) requestAnimationFrame(() => inviteButtonRef.current?.focus());
  }, []);

  return {
    inviteButtonRef,
    memberDrawerCloseRef,
    projectSessionRef,
    restoredProjectRef,
    workspaceTab,
    setWorkspaceTab,
    memberDrawerOpen,
    setMemberDrawerOpen,
    closeMemberDrawer,
  };
}
