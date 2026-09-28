export interface CancellableTabTurn {
  running: boolean;
  pendingUser?: string;
  pendingLocalUser?: { text: string } | null;
}

export function cancelTabTurn(
  state: CancellableTabTurn,
  actions: { unsend: () => void; markRequested: () => void; cancel: () => void },
): string | undefined {
  if (state.running && (state.pendingUser !== undefined || state.pendingLocalUser)) {
    const text = state.pendingUser ?? state.pendingLocalUser?.text ?? "";
    actions.unsend();
    actions.cancel();
    return text;
  }
  actions.markRequested();
  actions.cancel();
  return undefined;
}
