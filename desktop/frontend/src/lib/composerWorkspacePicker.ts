type ComposerWorkspacePickerInput = {
  workspaceView: string;
  sidebarImDetailConnection: boolean;
  activeTabId?: string | null;
  activeTabScope?: string;
  activeTabReadOnly?: boolean;
  currentSessionTurns: number;
  activeTopicTurns?: number;
};

export function currentSessionTurnCount(checkpointCount: number, visibleUserTurns: number): number {
  return Math.max(checkpointCount, visibleUserTurns);
}

export function shouldShowComposerWorkspacePicker(input: ComposerWorkspacePickerInput): boolean {
  void input.activeTopicTurns;
  return (
    input.workspaceView === "chat" &&
    !input.sidebarImDetailConnection &&
    Boolean(input.activeTabId) &&
    input.activeTabScope === "global" &&
    !input.activeTabReadOnly &&
    input.currentSessionTurns === 0
  );
}
