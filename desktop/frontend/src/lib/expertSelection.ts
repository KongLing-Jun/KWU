export interface ExpertSelectionLike {
  id: string;
  role?: string;
  mode?: string;
  prompt?: string;
  systemPrompt?: string;
  avatarSrc?: string;
  members?: unknown;
  selectedAt?: number;
}

export interface ResolveExpertRefreshSelectionInput<T extends ExpertSelectionLike> {
  previous: T | null;
  next: T | null;
  canClear: boolean;
  backendExpertIds: ReadonlySet<string>;
  pendingActivationId?: string | null;
  pendingClear?: boolean;
}

export interface PendingExpertActivation {
  // An empty expert id represents an in-flight clear for the target tab.
  expertId: string;
  tabId: string;
  requestSequence: number;
}

export function pendingExpertActivationIdForTab(
  pending: PendingExpertActivation | null,
  tabId: string | null,
): string | null {
  if (!pending || !tabId || pending.tabId !== tabId) return null;
  return pending.expertId;
}

export function expertActivationIds(
  expertCatalog: readonly ExpertSelectionLike[],
  userExpertCatalog: readonly ExpertSelectionLike[],
): ReadonlySet<string> {
  const ids = new Set<string>();
  for (const expert of [...expertCatalog, ...userExpertCatalog]) {
    const id = expert.id.trim();
    if (id) ids.add(id);
  }
  return ids;
}

export function removeDeletedExpertSelection<T extends ExpertSelectionLike>(selection: T | null, deletedId: string): T | null {
  return selection?.id === deletedId ? null : selection;
}

export function preserveCurrentExpertSelection<T extends ExpertSelectionLike>(next: T | null, previous: T | null): T | null {
  if (!next || !previous || next.id !== previous.id) return next;
  return {
    ...next,
    role: previous.role || next.role,
    mode: previous.mode || next.mode,
    prompt: previous.prompt || next.prompt,
    systemPrompt: previous.systemPrompt || next.systemPrompt,
    avatarSrc: previous.avatarSrc || next.avatarSrc,
    members: previous.members || next.members,
    selectedAt: previous.selectedAt,
  } as T;
}

export function resolveExpertRefreshSelection<T extends ExpertSelectionLike>({
  previous,
  next,
  canClear,
  backendExpertIds,
  pendingActivationId,
  pendingClear,
}: ResolveExpertRefreshSelectionInput<T>): T | null {
  if (pendingClear) return null;
  if (previous && pendingActivationId === previous.id && next?.id !== pendingActivationId) {
    return previous;
  }
  if (!next && previous && !canClear) return previous;
  const preserved = preserveCurrentExpertSelection(next, previous);
  if (preserved || !previous) return preserved;
  return backendExpertIds.has(previous.id) ? null : previous;
}
