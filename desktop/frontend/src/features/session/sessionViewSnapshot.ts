import type { Item } from "./sessionItems";
import type { ComposerSelectionState, State } from "./sessionState";
import type { EffortInfo, Meta } from "../../lib/types";

export const SESSION_VIEW_SNAPSHOT_VERSION = 2;
export const IDLE_SESSION_VIEW_LIMIT = 8;

export type SessionViewIdentity = {
  actorId: string;
  profileId: string;
  scope: "global" | "project";
  workspaceRoot: string;
  sessionPath: string;
  topicId: string;
};

export type SessionViewIdentityInput = Partial<Pick<SessionViewIdentity, "actorId" | "profileId" | "topicId">> & {
  scope: string;
  workspaceRoot?: string;
  sessionPath?: string;
};

export type SessionViewBasicMeta = Pick<
  Meta,
  | "label"
  | "modelRef"
  | "cwd"
  | "workspaceRoot"
  | "workspaceName"
  | "workspacePath"
  | "sessionPath"
  | "gitBranch"
  | "autoApproveTools"
  | "bypass"
  | "collaborationMode"
  | "toolApprovalMode"
  | "tokenMode"
  | "goal"
  | "goalStatus"
>;

export type SessionScrollAnchor =
  | { atBottom: true }
  | { atBottom: false; anchorMessageId: string; offset: number };

export type SessionViewSnapshot = {
  version: typeof SESSION_VIEW_SNAPSHOT_VERSION;
  key: string;
  identity: SessionViewIdentity;
  items: Item[];
  seq: number;
  historyFingerprint?: string;
  meta?: SessionViewBasicMeta;
  effort?: EffortInfo;
  composerSelection: ComposerSelectionState;
  title?: string;
  sessionTurns?: number;
  scroll: SessionScrollAnchor;
  historyBefore?: number;
  source?: SessionViewSource;
};

export type SessionViewWriter = {
  key: string;
  generation: number;
  ownerId: string;
};

export type SessionViewSource = {
  writer: SessionViewWriter;
  basis: "history" | "intent";
  reconciled: boolean;
};

type SessionViewEntry = {
  identity: SessionViewIdentity;
  generation: number;
  writer?: SessionViewWriter;
  accepted?: SessionViewSource;
  snapshot?: SessionViewSnapshot;
};

export function snapshotSessionView(
  state: State,
  identityInput: SessionViewIdentityInput,
  scroll: SessionScrollAnchor,
  title?: string,
  sessionTurns?: number,
): SessionViewSnapshot {
  const identity = normalizeSessionViewIdentity(identityInput);
  if (state.hydratePreview) return { ...state.hydratePreview, scroll };
  return {
    version: SESSION_VIEW_SNAPSHOT_VERSION,
    key: sessionViewIdentityKey(identity),
    identity,
    items: state.items.map((item) => item.kind === "assistant"
      ? {
        ...item,
        ...(state.live?.id === item.id ? {
          text: state.live.text,
          reasoning: state.live.reasoning,
          reasoningComplete: state.live.reasoningComplete,
        } : {}),
        streaming: false,
      }
      : item),
    seq: state.seq,
    historyFingerprint: state.historyFingerprint,
    meta: sessionViewBasicMeta(state.meta),
    effort: state.effort ? { ...state.effort, levels: [...state.effort.levels] } : undefined,
    composerSelection: { ...state.composerSelection },
    title,
    sessionTurns,
    scroll,
    historyBefore: state.hydrateHistoryLoaded ? state.historyBefore : undefined,
  };
}

function normalizePath(value: string): string {
  const normalized = value.trim().replace(/\\/g, "/").replace(/\/+$/g, "");
  // Windows drive and UNC paths are case-insensitive. Unix, remote, and
  // container paths retain case because it can be semantically significant.
  if (/^[a-z]:\//i.test(normalized) || normalized.startsWith("//")) {
    return normalized.toLocaleLowerCase("en-US");
  }
  return normalized;
}

export function normalizeSessionViewIdentity(input: SessionViewIdentityInput): SessionViewIdentity {
  const scope = input.scope === "project" ? "project" : "global";
  return {
    actorId: input.actorId?.trim() || "anonymous",
    profileId: input.profileId?.trim() || "default",
    scope,
    workspaceRoot: scope === "project" ? normalizePath(input.workspaceRoot ?? "") : "",
    sessionPath: normalizePath(input.sessionPath ?? ""),
    topicId: input.topicId?.trim() ?? "",
  };
}

export function sessionViewIdentityKey(input: SessionViewIdentityInput): string {
  const identity = normalizeSessionViewIdentity(input);
  const sessionIdentity = identity.sessionPath
    ? `path:${identity.sessionPath}`
    : `topic:${identity.topicId}`;
  return [
    `session-view-v${SESSION_VIEW_SNAPSHOT_VERSION}`,
    identity.actorId,
    identity.profileId,
    identity.scope,
    identity.workspaceRoot,
    sessionIdentity,
  ].join("\u0000");
}

export function sessionViewBasicMeta(meta?: Meta): SessionViewBasicMeta | undefined {
  if (!meta) return undefined;
  return {
    label: meta.label,
    modelRef: meta.modelRef,
    cwd: meta.cwd,
    workspaceRoot: meta.workspaceRoot,
    workspaceName: meta.workspaceName,
    workspacePath: meta.workspacePath,
    sessionPath: meta.sessionPath,
    gitBranch: meta.gitBranch,
    autoApproveTools: meta.autoApproveTools,
    bypass: meta.bypass,
    collaborationMode: meta.collaborationMode,
    toolApprovalMode: meta.toolApprovalMode,
    tokenMode: meta.tokenMode,
    goal: meta.goal,
    goalStatus: meta.goalStatus,
  };
}

export class SessionViewSnapshotCache {
  readonly #entries = new Map<string, SessionViewEntry>();
  #generation = 0;

  constructor(readonly limit = IDLE_SESSION_VIEW_LIMIT) {}

  get(input: SessionViewIdentityInput): SessionViewSnapshot | undefined {
    const key = sessionViewIdentityKey(input);
    const entry = this.#entries.get(key);
    if (!entry) return undefined;
    this.#touch(key, entry);
    const snapshot = entry.snapshot;
    if (!snapshot) return undefined;
    return {
      ...snapshot,
      items: [...snapshot.items],
      effort: snapshot.effort ? { ...snapshot.effort, levels: [...snapshot.effort.levels] } : undefined,
      composerSelection: { ...snapshot.composerSelection },
    };
  }

  set(snapshot: SessionViewSnapshot): void {
    if (
      snapshot.version !== SESSION_VIEW_SNAPSHOT_VERSION
      || snapshot.key !== sessionViewIdentityKey(snapshot.identity)
    ) return;
    let entry = this.#entries.get(snapshot.key);
    // A borrowed view or a callback from an evicted lifecycle cannot publish.
    if (snapshot.source && (!entry || entry.accepted !== snapshot.source)) return;
    if (!entry) entry = { identity: snapshot.identity, generation: ++this.#generation };
    if (entry.accepted && !snapshot.source) return;
    entry.snapshot = {
      ...snapshot,
      items: [...snapshot.items],
      effort: snapshot.effort ? { ...snapshot.effort, levels: [...snapshot.effort.levels] } : undefined,
      composerSelection: { ...snapshot.composerSelection },
    };
    this.#touch(snapshot.key, entry);
  }

  #touch(key: string, entry: SessionViewEntry): void {
    this.#entries.delete(key);
    this.#entries.set(key, entry);
    while (this.#entries.size > this.limit) {
      const oldest = this.#entries.keys().next().value as string | undefined;
      if (oldest === undefined) break;
      this.#entries.delete(oldest);
    }
  }

  writer(input: SessionViewIdentityInput, ownerId: string): SessionViewWriter | undefined {
    const writer = this.#entries.get(sessionViewIdentityKey(input))?.writer;
    return writer?.ownerId === ownerId ? writer : undefined;
  }

  claimWriter(input: SessionViewIdentityInput, ownerId: string): SessionViewWriter {
    const key = sessionViewIdentityKey(input);
    const entry = this.#entries.get(key) ?? { identity: normalizeSessionViewIdentity(input), generation: ++this.#generation };
    if (entry.writer?.ownerId !== ownerId) {
      entry.writer = { key, generation: entry.generation, ownerId };
    }
    this.#touch(key, entry);
    return entry.writer!;
  }

  isCurrent(writer: SessionViewWriter | undefined): writer is SessionViewWriter {
    return Boolean(writer && this.#entries.get(writer.key)?.writer === writer);
  }

  source(input: SessionViewIdentityInput): SessionViewSource | undefined {
    return this.#entries.get(sessionViewIdentityKey(input))?.accepted;
  }

  canCarry(snapshot: SessionViewSnapshot): boolean {
    const entry = this.#entries.get(snapshot.key);
    return Boolean(entry && entry.accepted === snapshot.source);
  }

  acceptHistory(writer: SessionViewWriter | undefined): void {
    if (!this.isCurrent(writer)) return;
    const entry = this.#entries.get(writer.key)!;
    // Keep a current intent's identity, including optimistic sends and failures.
    if (entry.accepted?.writer === writer && entry.accepted.basis === "intent") {
      entry.accepted.reconciled = true;
    } else {
      entry.accepted = { writer, basis: "history", reconciled: true };
    }
    entry.snapshot = undefined;
  }

  acceptIntent(writer: SessionViewWriter | undefined, newUser = false): boolean {
    if (!this.isCurrent(writer)) return false;
    const entry = this.#entries.get(writer.key)!;
    if (newUser || entry.accepted?.writer !== writer || entry.accepted.basis !== "intent") {
      entry.accepted = {
        writer, basis: "intent",
        reconciled: entry.accepted?.writer === writer && entry.accepted.reconciled === true,
      };
    }
    entry.snapshot = undefined;
    return true;
  }

  select(input: SessionViewIdentityInput, stateForOwner: (ownerId: string) => State | undefined): { state?: State; source?: SessionViewSource; snapshot?: SessionViewSnapshot } {
    const entry = this.#entries.get(sessionViewIdentityKey(input));
    const source = entry?.accepted;
    // Only the accepted, still-authorized owner can supply mutable runtime state.
    // Once ownership changes, use its frozen display, never scan retained owners.
    const state = source && source.writer === entry?.writer ? stateForOwner(source.writer.ownerId) : undefined;
    if (state && !state.hydratePreview) return { state, source };
    return { snapshot: this.get(input) };
  }

  invalidate(input: SessionViewIdentityInput): void {
    this.#entries.delete(sessionViewIdentityKey(input));
  }

  releaseOwner(ownerId: string): void {
    for (const entry of this.#entries.values()) {
      if (entry.writer?.ownerId === ownerId) entry.writer = undefined;
    }
  }

  invalidateSessionPath(path: string): void {
    const normalized = normalizePath(path);
    for (const [key, entry] of this.#entries) {
      if (entry.identity.sessionPath === normalized) this.#entries.delete(key);
    }
  }

  renameSession(path: string, title: string): void {
    const normalized = normalizePath(path);
    for (const entry of this.#entries.values()) {
      if (entry.identity.sessionPath === normalized && entry.snapshot) {
        entry.snapshot = { ...entry.snapshot, title };
      }
    }
  }

  clear(): void {
    this.#entries.clear();
  }

  keys(): string[] {
    return Array.from(this.#entries.keys());
  }
}
