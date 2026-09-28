// useController is the frontend's state machine over the agent's event stream. It
// maintains per-tab state so background tabs preserve their streaming output, tool
// states, and approvals when the user switches away and back. The active tab's state
// is what components render.

import { useCallback, useEffect, useRef, useState } from "react";
import { asArray } from "../../lib/array";
import { addBreadcrumb } from "../../lib/breadcrumbs";
import { app, onEvent, onReady, waitForBackendReady } from "../../lib/bridge";
import { useTurnBillingRecovery } from "./useTurnBillingRecovery";
import { redactClientErrorText, reportClientError } from "../../lib/clientError";
import { invalidateCache } from "../../lib/composerHistory";
import { composerSessionIdentityFromSource, runtimeComposerSessionIdentity } from "../../lib/composerSessionIdentity";
import {
  currentComposerExpertTraceId,
  recordComposerExpertTrace,
} from "../../lib/composerExpertTrace";
import { applyEvent as projectEvent } from "./eventProjection";
import { createRafBatch } from "../../lib/rafBatch";
import { initialPromptProjection } from "./promptProjection";
import { reduceSession } from "./sessionReducer";
import { cancelTabTurn } from "./cancelTabTurn";
import { readTranscriptReplay, transcriptReplayNeedsFullRestore } from "./transcriptReplay";
import { historyRevalidationRequestIsCurrent, prepareHistoryRevalidation } from "./historyRevalidation";
import {
  STALE_TURN_RECONCILE_MS,
  foregroundRunningFromRuntimeMeta,
  metaFromTab,
  shouldReconcileStaleTurn,
} from "./sessionMeta";
import { type Item, type MessageActionScope } from "./sessionItems";
import { initialState, type Action, type HydrateReason, type SessionSurfaceIntent, type State } from "./sessionState";
import { coalesceRoutedStreamEvents, isStreamBatchingEnabled, type RoutedStreamEvent } from "./streamEventBatch";
import {
  IDLE_SESSION_VIEW_LIMIT,
  SessionViewSnapshotCache,
  normalizeSessionViewIdentity,
  snapshotSessionView,
  sessionViewIdentityKey,
  type SessionViewIdentityInput,
  type SessionScrollAnchor,
  type SessionViewSnapshot,
  type SessionViewWriter,
} from "./sessionViewSnapshot";
import { t } from "../../lib/i18n";
import { modeHasAutoApproveTools } from "../../lib/types";
import type {
  CollaborationMode,
  ComposerSessionIdentity,
  HistoryMessage,
  HistoryPage,
  MemoryView,
  Mode,
  PromptReplyReceipt,
  QuestionAnswer,
  Scene,
  SceneState,
  SessionMeta,
  TabMeta,
  TokenMode,
  ToolApprovalMode,
  WireEvent,
} from "../../lib/types";

const HISTORY_PAGE_SIZE = 200;

async function historyPageForTab(tabId: string, before = 0, generation = ""): Promise<HistoryPage> {
  if (typeof app.HistoryPageForTab === "function") {
    return app.HistoryPageForTab(tabId, before, HISTORY_PAGE_SIZE, generation);
  }
  const messages = asArray<HistoryMessage>(await app.HistoryForTab(tabId));
  return { messages, before: 0, hasMore: false, generation: "" };
}

export {
  foregroundRunningFromRuntimeMeta,
  metaFromTab,
  sameMeta,
  shouldReconcileStaleTurn,
} from "./sessionMeta";
export { isReadOnlyTool } from "./sessionItems";
export type { Item, LiveStream, MessageActionScope, MessageActionState, ToolStatus } from "./sessionItems";
export { historyMessagesToItems } from "./historyProjection";
export { initialState } from "./sessionState";
export type { HydrateReason, SessionSurfaceIntent } from "./sessionState";
export type { SessionScrollAnchor } from "./sessionViewSnapshot";

const CANCEL_RECONCILE_DELAYS_MS = [0, 100, 300, 1_000] as const;
const TERMINAL_RECONCILE_DELAYS_MS = [100, 300, 1_000] as const;

type TurnIdentity = {
  sessionGen: number;
  turnStartAt: number;
};

function hasCachedLiveTurn(state: State | undefined): boolean {
  if (!state?.running && !state?.turnActive) return false;
  if (
    state.live ||
    state.currentAssistant ||
    Object.keys(state.currentChildAssistantByRef).length > 0 ||
    state.pendingUser !== undefined
  ) return true;
  return state.items.some((item) =>
    (item.kind === "assistant" && item.streaming) ||
    (item.kind === "tool" && item.status === "running")
  );
}

function hasReusableCachedTranscript(state: State | undefined, sessionPath?: string): boolean {
  if (!state || state.items.length === 0) return false;
  const expectedSessionPath = (sessionPath ?? "").trim();
  if (!expectedSessionPath) return true;
  return (state.meta?.sessionPath ?? "").trim() === expectedSessionPath;
}

export function turnDoneMatchesStateSession(event: WireEvent, state: State | undefined): boolean {
  if (event.kind !== "turn_done") return true;
  const eventSessionPath = normalizeSessionViewIdentity({
    scope: "global",
    sessionPath: event.conversationSessionPath,
  }).sessionPath;
  const stateSessionPath = normalizeSessionViewIdentity({
    scope: "global",
    sessionPath: state?.meta?.sessionPath,
  }).sessionPath;
  // A first turn can finish before its newly-created session path reaches the
  // frontend. When both paths exist they must agree; turn ID remains the
  // primary ownership key while either side is still unsaved.
  if (eventSessionPath && stateSessionPath && eventSessionPath !== stateSessionPath) return false;
  return true;
}

export function turnDoneMatchesStateIdentity(event: WireEvent, state: State | undefined): boolean {
  if (!turnDoneMatchesStateSession(event, state)) return false;
  const eventTurnId = event.turnId?.trim() ?? "";
  const stateTurnId = state?.runtimeTurnId?.trim() ?? "";
  if (event.artifactDelivery !== undefined) {
    return Boolean(eventTurnId && stateTurnId && eventTurnId === stateTurnId);
  }
  return !eventTurnId || !stateTurnId || eventTurnId === stateTurnId;
}

export function reducer(s: State, a: Action): State {
  return reduceSession(s, a, projectEvent);
}

// ---- per-tab state map ----

type TabStates = Map<string, State>;

export type SurfaceTarget = {
  scope: string;
  workspaceRoot: string;
  topicId: string;
  sessionPath?: string;
  sessionTurns?: number;
  scene?: Scene;
  sceneState?: SceneState;
  composerIdentity?: ComposerSessionIdentity;
};

function surfaceIntentForTarget(
  sessionPath: string | undefined,
  sessionTurns?: number,
  fallback: Exclude<SessionSurfaceIntent, "existing-session"> = "unknown",
): SessionSurfaceIntent {
  if (sessionTurns !== undefined) return sessionTurns > 0 ? "existing-session" : "blank";
  return sessionPath?.trim() ? "unknown" : fallback;
}

type SurfaceTransition = {
  seq: number;
  target: SurfaceTarget;
  snapshot?: SessionViewSnapshot;
  state: State;
};

type SessionViewCacheScope = {
  actorId?: string;
  profileId?: string;
  bindingGeneration?: number;
};

function surfaceTargetFromTab(tab: TabMeta, previous?: SurfaceTarget): SurfaceTarget {
  const sessionPath = tab.sessionPath?.trim() || undefined;
  const previousSessionPath = previous?.sessionPath?.trim() || undefined;
  const hasRuntimeSceneStatus = tab.scene !== undefined || tab.sceneState !== undefined;
  const sameSession = sessionPath === previousSessionPath;
  const previousIdentity = sameSession ? previous?.composerIdentity : undefined;
  const tabIdentity = runtimeComposerSessionIdentity(tab, previousIdentity);
  return {
    scope: tab.scope,
    workspaceRoot: tab.workspaceRoot,
    topicId: tab.topicId,
    sessionPath,
    sessionTurns: tab.sessionTurns
      ?? (sameSession ? previous?.sessionTurns : undefined),
    scene: hasRuntimeSceneStatus ? tab.scene : sameSession ? previous?.scene : undefined,
    sceneState: hasRuntimeSceneStatus ? tab.sceneState : sameSession ? previous?.sceneState : undefined,
    composerIdentity: tabIdentity,
  };
}

function coldStateForSurfaceTarget(target: SurfaceTarget): State {
  const identity = target.composerIdentity;
  const autoApproveTools = identity?.toolApprovalMode === "yolo";
  return {
    ...initialState,
    meta: identity
      ? {
          label: "",
          modelRef: identity.modelRef,
          ready: false,
          eventChannel: "",
          cwd: target.workspaceRoot,
          workspaceRoot: target.workspaceRoot,
          workspacePath: target.workspaceRoot,
          sessionPath: target.sessionPath,
          autoApproveTools,
          bypass: autoApproveTools,
          collaborationMode: identity.collaborationMode,
          toolApprovalMode: identity.toolApprovalMode,
          tokenMode: identity.tokenMode,
          goal: identity.goal,
          goalStatus: identity.goalStatus,
        }
      : undefined,
    composerSelection: {
      ...initialState.composerSelection,
      modelRef: identity?.modelRef?.trim() ?? "",
      effortSnapshot: identity?.effort,
    },
    hydrating: true,
    surfaceIntent: surfaceIntentForTarget(target.sessionPath, target.sessionTurns),
    hydrateReason: "open-topic",
    backendActivationPending: true,
  };
}

function stateFromViewSnapshot(snapshot: SessionViewSnapshot, pending: boolean, preview = false): State {
  const meta = snapshot.meta
    ? { ...snapshot.meta, ready: false, eventChannel: "" }
    : undefined;
  return {
    ...initialState,
    items: preview ? [] : [...snapshot.items],
    seq: preview ? 0 : snapshot.seq,
    meta,
    effort: snapshot.effort
      ? { ...snapshot.effort, levels: [...snapshot.effort.levels] }
      : undefined,
    composerSelection: { ...snapshot.composerSelection },
    hydrateHistoryLoaded: !preview,
    hydratePreview: preview ? snapshot : undefined,
    historyFingerprint: snapshot.historyFingerprint,
    hydrating: false,
    surfaceIntent: surfaceIntentForTarget(snapshot.identity.sessionPath),
    backendActivationPending: pending,
  };
}

function hasActiveRuntimeTail(state: State | undefined): boolean {
  return Boolean(
    state
    && (
      state.running
      || state.turnActive
      || state.live
      || state.items.some((item) =>
        (item.kind === "assistant" && item.streaming)
        || (item.kind === "tool" && item.status === "running"))
    )
  );
}

function getOrCreateState(states: TabStates, tabId: string): State {
  if (!states.has(tabId)) states.set(tabId, { ...initialState, prompts: initialPromptProjection(tabId) });
  return states.get(tabId)!;
}

function messageActionBusyText(scope: MessageActionScope): string {
  switch (scope) {
    case "fork":
      return t("rewind.busyFork");
    case "summ-from":
      return t("rewind.busySummFrom");
    case "summ-upto":
      return t("rewind.busySummUpto");
    case "conversation":
      return t("rewind.busyConversation");
    case "code":
      return t("rewind.busyCode");
    default:
      return t("rewind.busyBoth");
  }
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return redactClientErrorText(err.message);
  if (typeof err === "string") return redactClientErrorText(err);
  return redactClientErrorText(String(err || ""));
}

function visibleErrorMessage(scope: string, err: unknown, fallbackMessage: string): string {
  return reportClientError(scope, err, { fallbackMessage }).userMessage;
}

export function replayPendingPromptsForActiveTab(activeTabId: string | undefined, replay: () => Promise<void> = () => app.ReplayPendingPrompts()): void {
  if (!activeTabId) return;
  void replay().catch(() => {});
}

function backendStatusActionFromTab(tab: TabMeta): Action {
  const foregroundRunning = foregroundRunningFromRuntimeMeta(tab);
  return {
    type: "backend_status",
    running: foregroundRunning,
    pendingPrompt: Boolean(tab.pendingPrompt),
    backgroundJobs: tab.backgroundJobs ?? 0,
    cancelRequested: Boolean(tab.cancelRequested),
    cancellable: foregroundRunning,
    activeTurnStartedAt: tab.activeTurnStartedAt,
    activeTurnCompletionTokens: tab.activeTurnCompletionTokens,
  };
}

export function reducerEntryTraceMessage(tabId: string, event: WireEvent, arrival: number): string | undefined {
  if (event.parentId?.trim() || event.subagentRef?.trim()) return undefined;
  if (
    event.kind !== "turn_started"
    && event.kind !== "message"
    && event.kind !== "final_candidate_rejected"
    && event.kind !== "turn_done"
  ) return undefined;
  return [
    "stage=use_controller_reducer",
    `tab=${tabId}`,
    `turn=${event.turnId ?? ""}`,
    `kind=${event.kind}`,
    `candidate=${event.candidateId ?? ""}`,
    `event=${event.eventId ?? ""}`,
    `source_sequence=${event.eventSequence ?? 0}`,
    `arrival=${arrival}`,
  ].join(" ");
}

export function useController() {
  const beforeSendRef = useRef<((tabId: string) => Promise<void>) | null>(null);
  const statesRef = useRef<TabStates>(new Map());
  const composerSelectionSeqByTab = useRef(new Map<string, number>());
  const lastTurnActivityAtByTab = useRef(new Map<string, number>());
  const reducerEventArrivalsByTab = useRef(new Map<string, number>());
  const cancelReconcileTimers = useRef(new Map<string, number>());
  const terminalReconcileTimers = useRef(new Map<string, number>());
  const terminalReconcileSeqByTab = useRef(new Map<string, number>());
  const contextRefreshTimers = useRef(new Map<string, number>());
  const surfaceActivationSeq = useRef(0);
  const sessionViewSnapshots = useRef(new SessionViewSnapshotCache());
  const sessionViewCacheScopeRef = useRef({ actorId: "anonymous", profileId: "default", bindingGeneration: 0 });
  const sessionViewTargetByTab = useRef(new Map<string, SurfaceTarget>());
  const transcriptLoads = useRef(new Map<string, { seq: number; events: WireEvent[] }>());
  const sessionLoadSeq = useRef(new Map<string, number>());
  const sessionViewTitleByTab = useRef(new Map<string, string>());
  const sessionViewScrollByKey = useRef(new Map<string, SessionScrollAnchor>());
  const surfaceTransitionRef = useRef<SurfaceTransition | null>(null);
  const surfaceLoadingHintTimer = useRef<number | null>(null);
  const [surfaceTransition, setSurfaceTransition] = useState<SurfaceTransition | null>(null);
  const [surfaceLoadingHintVisible, setSurfaceLoadingHintVisible] = useState(false);
  const [activeTabId, setActiveTabId] = useState<string | undefined>();
  const activeTabIdRef = useRef<string | undefined>(undefined);
  // A render-triggering counter so that mutations to a non-active tab's state still
  // cause a re-render when that tab becomes active.
  const [, setVersion] = useState(0);
  const bump = useCallback(() => setVersion((v) => v + 1), []);

  // The active tab's current state, with a stable identity for cancel().
  const runtimeActiveState = activeTabId ? getOrCreateState(statesRef.current, activeTabId) : initialState;
  const renderedState = surfaceTransition?.state ?? runtimeActiveState;
  const activeState = renderedState.hydratePreview
    ? { ...renderedState, items: renderedState.hydratePreview.items, live: undefined }
    : renderedState;
  const stateRef = useRef(activeState);
  const backendActiveTabIdRef = useRef<string | undefined>(undefined);
  const backendActivationPromises = useRef(new Map<string, Promise<boolean>>());
  const backendActivationQueueRef = useRef<Promise<void>>(Promise.resolve());
  activeTabIdRef.current = activeTabId;
  stateRef.current = activeState;

  const enqueueBackendActivation = useCallback((operation: () => Promise<boolean>): Promise<boolean> => {
    const result = backendActivationQueueRef.current.catch(() => undefined).then(operation);
    backendActivationQueueRef.current = result.then(() => undefined, () => undefined);
    return result;
  }, []);

  const clearSurfaceLoadingHintTimer = useCallback(() => {
    if (surfaceLoadingHintTimer.current === null) return;
    window.clearTimeout(surfaceLoadingHintTimer.current);
    surfaceLoadingHintTimer.current = null;
  }, []);

  useEffect(() => {
    if (surfaceTransition !== null) return;
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(false);
    if (
      activeState.surfaceIntent !== "existing-session"
      || !activeState.hydrating
      || activeState.hydrateHistoryLoaded === true
    ) return;
    const targetTabId = activeTabId;
    const targetSessionPath = activeState.meta?.sessionPath ?? "";
    surfaceLoadingHintTimer.current = window.setTimeout(() => {
      surfaceLoadingHintTimer.current = null;
      const current = stateRef.current;
      if (
        surfaceTransitionRef.current === null
        && activeTabIdRef.current === targetTabId
        && (current.meta?.sessionPath ?? "") === targetSessionPath
        && current.surfaceIntent === "existing-session"
        && current.hydrating
        && current.hydrateHistoryLoaded !== true
      ) {
        setSurfaceLoadingHintVisible(true);
      }
    }, 120);
    return clearSurfaceLoadingHintTimer;
  }, [
    activeState.hydrateHistoryLoaded,
    activeState.hydrating,
    activeState.meta?.sessionPath,
    activeState.surfaceIntent,
    activeTabId,
    clearSurfaceLoadingHintTimer,
    surfaceTransition,
  ]);

  const scopedViewIdentity = useCallback((target: SurfaceTarget): SessionViewIdentityInput => ({
    ...sessionViewCacheScopeRef.current,
    scope: target.scope,
    workspaceRoot: target.workspaceRoot,
    sessionPath: target.sessionPath,
    topicId: target.topicId,
  }), []);

  const rememberTabView = useCallback((tabId: string, target: SurfaceTarget, title = "") => {
    const previous = sessionViewTargetByTab.current.get(tabId);
    if (previous && sessionViewIdentityKey(scopedViewIdentity(previous)) !== sessionViewIdentityKey(scopedViewIdentity(target))) {
      sessionViewSnapshots.current.releaseOwner(tabId);
      sessionLoadSeq.current.set(tabId, (sessionLoadSeq.current.get(tabId) ?? 0) + 1);
    }
    sessionViewTargetByTab.current.set(tabId, target);
    if (title) sessionViewTitleByTab.current.set(tabId, title);
  }, [scopedViewIdentity]);

  const selectSessionView = useCallback((target: SurfaceTarget) => {
    const identity = scopedViewIdentity(target);
    const selected = sessionViewSnapshots.current.select(identity, (ownerId) => {
      const ownerTarget = sessionViewTargetByTab.current.get(ownerId);
      return ownerTarget && sessionViewIdentityKey(scopedViewIdentity(ownerTarget)) === sessionViewIdentityKey(identity)
        ? statesRef.current.get(ownerId) : undefined;
    });
    const snapshot = selected.state ? {
      ...snapshotSessionView(
        selected.state, identity,
        sessionViewScrollByKey.current.get(sessionViewIdentityKey(identity)) ?? { atBottom: true },
        sessionViewTitleByTab.current.get(selected.source!.writer.ownerId), target.sessionTurns,
      ),
      source: selected.source,
    } : selected.snapshot;
    return { ...selected, snapshot };
  }, [scopedViewIdentity]);

  const cacheIdleTabView = useCallback((tabId?: string) => {
    if (!tabId) return;
    const state = statesRef.current.get(tabId);
    const target = sessionViewTargetByTab.current.get(tabId);
    if (
      !state
      || !target
      || state.running
      || state.turnActive
      || state.live
      || state.hydratePreview
      || state.pendingUser !== undefined
      || state.pendingPrompt
      || state.approval
      || state.ask
      || state.hydrateHistoryLoaded !== true
    ) return;
    const identityInput = scopedViewIdentity(target);
    const source = sessionViewSnapshots.current.source(identityInput);
    if (!source?.reconciled || source.writer !== sessionViewSnapshots.current.writer(identityInput, tabId)) return;
    sessionViewSnapshots.current.set({ ...snapshotSessionView(
      state,
      identityInput,
      sessionViewScrollByKey.current.get(sessionViewIdentityKey(identityInput)) ?? { atBottom: true },
      sessionViewTitleByTab.current.get(tabId),
      target.sessionTurns,
    ), source });
  }, [scopedViewIdentity]);

  const pruneSessionRuntimeStates = useCallback((keepTabId: string) => {
    // Keep recently visited task states intact, including their journal cursor
    // and item identities. Evict only the oldest idle views beyond the budget.
    const retained = statesRef.current.get(keepTabId);
    if (retained) {
      statesRef.current.delete(keepTabId);
      statesRef.current.set(keepTabId, retained);
    }
    let idleCount = Array.from(statesRef.current.values()).filter((state) => !hasActiveRuntimeTail(state)).length;
    for (const [tabId, state] of statesRef.current) {
      if (tabId === keepTabId) continue;
      // A running transcript is not reconstructible from saved history: stream
      // output and terminal-delivery identity may not have been persisted yet.
      // Keep the immutable state intact; background events update only this tab.
      if (hasActiveRuntimeTail(state)) continue;
      if (idleCount <= IDLE_SESSION_VIEW_LIMIT) break;
      cacheIdleTabView(tabId);
      sessionLoadSeq.current.set(tabId, (sessionLoadSeq.current.get(tabId) ?? 0) + 1);
      transcriptLoads.current.delete(tabId);
      statesRef.current.delete(tabId);
      sessionViewSnapshots.current.releaseOwner(tabId);
      sessionViewTargetByTab.current.delete(tabId);
      sessionViewTitleByTab.current.delete(tabId);
      idleCount -= 1;
    }
  }, [cacheIdleTabView]);

  const setSessionViewCacheScope = useCallback((scope: SessionViewCacheScope) => {
    const next = {
      actorId: scope.actorId?.trim() || "anonymous",
      profileId: scope.profileId?.trim() || "default",
      bindingGeneration: scope.bindingGeneration ?? 0,
    };
    const current = sessionViewCacheScopeRef.current;
    if (current.actorId === next.actorId && current.profileId === next.profileId && current.bindingGeneration === next.bindingGeneration) return;
    sessionViewCacheScopeRef.current = next;
    surfaceActivationSeq.current += 1;
    surfaceTransitionRef.current = null;
    setSurfaceTransition(null);
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(false);
    for (const [tabId, state] of statesRef.current) {
      if (state.hydratePreview) statesRef.current.set(tabId, { ...state, hydratePreview: undefined });
    }
    sessionViewSnapshots.current.clear();
    sessionViewTargetByTab.current.clear();
    sessionViewTitleByTab.current.clear();
    sessionViewScrollByKey.current.clear();
    bump();
  }, [bump, clearSurfaceLoadingHintTimer]);

  const downgradeSurfaceTransitionToCold = useCallback((seq: number) => {
    const current = surfaceTransitionRef.current;
    if (!current || current.seq !== seq) return;
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(false);
    const cold: SurfaceTransition = {
      ...current,
      snapshot: undefined,
      state: coldStateForSurfaceTarget(current.target),
    };
    surfaceTransitionRef.current = cold;
    setSurfaceTransition(cold);
    surfaceLoadingHintTimer.current = window.setTimeout(() => {
      surfaceLoadingHintTimer.current = null;
      if (surfaceTransitionRef.current?.seq === seq) setSurfaceLoadingHintVisible(true);
    }, 120);
  }, [clearSurfaceLoadingHintTimer]);

  const beginSurfaceTransition = useCallback((target: SurfaceTarget): number => {
    const seq = surfaceActivationSeq.current + 1;
    surfaceActivationSeq.current = seq;
    cacheIdleTabView(activeTabIdRef.current);
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(false);
    const { state: retainedState, snapshot } = selectSessionView(target);
    if (snapshot) sessionViewScrollByKey.current.set(snapshot.key, snapshot.scroll);
    addBreadcrumb("tab.switch", `pending-view ${target.scope} ${target.topicId}`);
    const transition: SurfaceTransition = {
      seq,
      target,
      snapshot,
      state: retainedState
        ? { ...retainedState, approval: undefined, ask: undefined, backendActivationPending: true }
        : snapshot
        ? stateFromViewSnapshot(snapshot, true)
        : coldStateForSurfaceTarget(target),
    };
    recordComposerExpertTrace("transition.begin", {
      transitionSeq: seq,
      topicId: target.topicId,
      sessionPath: target.sessionPath ?? "",
      targetExpertKnown: target.composerIdentity?.expertKnown ?? false,
      targetExpertId: target.composerIdentity?.expertId ?? "",
      targetExpertName: target.composerIdentity?.expertName ?? "",
      snapshot: Boolean(snapshot),
    });
    surfaceTransitionRef.current = transition;
    setSurfaceTransition(transition);
    surfaceLoadingHintTimer.current = window.setTimeout(() => {
      surfaceLoadingHintTimer.current = null;
      if (surfaceTransitionRef.current?.seq === seq) {
        setSurfaceLoadingHintVisible(true);
      }
    }, 120);
    return seq;
  }, [cacheIdleTabView, clearSurfaceLoadingHintTimer, selectSessionView]);

  const finishSurfaceTransition = useCallback((seq: number) => {
    if (surfaceTransitionRef.current?.seq !== seq) return;
    recordComposerExpertTrace("transition.finish", {
      transitionSeq: seq,
      activeTabId: activeTabIdRef.current ?? "",
    });
    surfaceTransitionRef.current = null;
    setSurfaceTransition(null);
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(false);
  }, [clearSurfaceLoadingHintTimer]);

  const cancelSurfaceTransition = useCallback((seq: number) => {
    finishSurfaceTransition(seq);
  }, [finishSurfaceTransition]);

  const clearSurfaceTransition = useCallback(() => {
    surfaceTransitionRef.current = null;
    setSurfaceTransition(null);
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(false);
  }, [clearSurfaceLoadingHintTimer]);

  const failSurfaceTransition = useCallback((seq: number, error: unknown) => {
    const current = surfaceTransitionRef.current;
    if (!current || current.seq !== seq) return;
    clearSurfaceLoadingHintTimer();
    setSurfaceLoadingHintVisible(true);
    const failed: SurfaceTransition = {
      ...current,
      state: reducer(current.state, {
        type: "hydrate_error",
        reason: "open-topic",
        error: visibleErrorMessage("session.openTopic", error, "打开任务失败，请稍后重试。"),
      }),
    };
    surfaceTransitionRef.current = failed;
    setSurfaceTransition(failed);
  }, [clearSurfaceLoadingHintTimer]);

  const updateSessionViewScroll = useCallback((key: string, anchor: SessionScrollAnchor) => {
    if (!key) return;
    sessionViewScrollByKey.current.set(key, anchor);
  }, []);

  // Dispatch to a specific tab's state. If the tab doesn't have state yet, it's
  // created. Bumps the version so React re-renders when it becomes active.
  const dispatchTo = useCallback((tabId: string, action: Action, historyWriter?: SessionViewWriter | null) => {
    const states = statesRef.current;
    const target = sessionViewTargetByTab.current.get(tabId);
    const identity = target ? scopedViewIdentity(target) : undefined;
    if (action.type === "reset" && identity) {
      const wasWriter = sessionViewSnapshots.current.writer(identity, tabId);
      sessionViewSnapshots.current.invalidate(identity);
      if (wasWriter || backendActiveTabIdRef.current === tabId) sessionViewSnapshots.current.claimWriter(identity, tabId);
    }
    if (action.type === "user" && identity && backendActiveTabIdRef.current === tabId) {
      // An explicit new send may start a fresh lifecycle after cache eviction.
      sessionViewSnapshots.current.claimWriter(identity, tabId);
    }
    const writer = identity ? sessionViewSnapshots.current.writer(identity, tabId) : undefined;
    const current = getOrCreateState(states, tabId);
    const prev = current.prompts.tabId ? current : { ...current, prompts: { ...current.prompts, tabId } };
    if (prev !== current) states.set(tabId, prev);
    let next = reducer(prev, action);
    if (prev !== next) {
      const events = action.type === "event" ? [action.e] : action.type === "stream_batch" ? action.events : [];
      const currentIntent = action.type === "user" || events.some((event) =>
        event.transcriptUser || ["turn_started", "text", "reasoning", "message", "tool_dispatch", "tool_progress", "tool_result", "approval_request", "ask_request"].includes(event.kind));
      if (currentIntent && sessionViewSnapshots.current.acceptIntent(writer, action.type === "user")) {
        if (next.hydratePreview) next = { ...next, hydratePreview: undefined };
      }
      states.set(tabId, next);
      if (["transcript_replay", "history", "history_with_runtime_tail", "history_revalidated"].includes(action.type)) {
        sessionViewSnapshots.current.acceptHistory(historyWriter === undefined ? writer : historyWriter ?? undefined);
      }
      bump();
    }
  }, [bump, scopedViewIdentity]);

  const nextComposerSelectionSeq = useCallback((tabId: string): number => {
    const stateSeq = statesRef.current.get(tabId)?.composerSelection.seq ?? 0;
    const seq = Math.max(stateSeq, composerSelectionSeqByTab.current.get(tabId) ?? 0) + 1;
    composerSelectionSeqByTab.current.set(tabId, seq);
    return seq;
  }, []);

  useTurnBillingRecovery(surfaceTransition ? undefined : activeTabId, runtimeActiveState, dispatchTo);

  const refreshComposerSelectionForTab = useCallback(async (
    tabId: string,
    options: {
      seq?: number;
      modelRef?: string;
      invalidate?: boolean;
      meta?: State["meta"];
      noteFailure?: (label: string, err: unknown) => void;
    } = {},
  ): Promise<"ready" | "degraded" | "stale"> => {
    const previous = statesRef.current.get(tabId) ?? initialState;
    const invalidate = options.invalidate ?? false;
    const seq = options.seq ?? nextComposerSelectionSeq(tabId);
    if (options.seq !== undefined) {
      composerSelectionSeqByTab.current.set(
        tabId,
        Math.max(options.seq, composerSelectionSeqByTab.current.get(tabId) ?? 0),
      );
    }
    const modelRef = options.modelRef?.trim()
      || options.meta?.modelRef?.trim()
      || previous.composerSelection.modelRef
      || previous.meta?.modelRef?.trim()
      || "";
    if (options.seq !== undefined && previous.composerSelection.seq !== options.seq) return "stale";
    if (options.seq === undefined) {
      dispatchTo(tabId, {
        type: "composer_selection_start",
        seq,
        modelRef,
        invalidate,
        meta: options.meta,
      });
    }

    const [meta, effort] = await Promise.all([
      app.MetaForTab(tabId).catch((err) => {
        options.noteFailure?.("meta", err);
        return undefined;
      }),
      app.EffortForTab(tabId).catch((err) => {
        options.noteFailure?.("effort", err);
        return undefined;
      }),
    ]);
    if (statesRef.current.get(tabId)?.composerSelection.seq !== seq) return "stale";

    if (meta === undefined) {
      if (!invalidate && previous.composerSelection.phase !== "pending") {
        dispatchTo(tabId, {
          type: "composer_selection_restore",
          seq,
          meta: previous.meta,
          effort: previous.effort,
          selection: previous.composerSelection,
        });
        return previous.composerSelection.phase === "ready" ? "ready" : "degraded";
      }
      dispatchTo(tabId, { type: "composer_selection_degraded", seq, modelRef });
      return "degraded";
    }

    const returnedModelRef = meta.modelRef?.trim() ?? "";
    if (effort === undefined) {
      dispatchTo(tabId, {
        type: "composer_selection_degraded",
        seq,
        modelRef: returnedModelRef,
        meta,
      });
      return "degraded";
    }
    dispatchTo(tabId, {
      type: "composer_selection_ready",
      seq,
      modelRef: returnedModelRef,
      meta,
      effort,
    });
    return "ready";
  }, [dispatchTo, nextComposerSelectionSeq]);

  const refreshContextForTab = useCallback(async (tabId: string): Promise<void> => {
    const context = await app.ContextUsageForTab(tabId).catch(() => undefined);
    if (context !== undefined) dispatchTo(tabId, { type: "context", context });
  }, [dispatchTo]);

  const bindSessionPathToTab = useCallback((tabId: string, sessionPath: string) => {
    const target = sessionViewTargetByTab.current.get(tabId);
    if (target) {
      rememberTabView(tabId, { ...target, sessionPath }, sessionViewTitleByTab.current.get(tabId));
    }
    const state = statesRef.current.get(tabId);
    if (state?.meta) {
      dispatchTo(tabId, {
        type: "optimistic_meta",
        meta: { ...state.meta, sessionPath },
      });
    }
  }, [dispatchTo, rememberTabView]);

  const confirmBackendActiveTab = useCallback((tabId: string) => {
    backendActiveTabIdRef.current = tabId;
    const target = sessionViewTargetByTab.current.get(tabId);
    if (target) {
      const selected = selectSessionView(target);
      if (selected.snapshot) sessionViewSnapshots.current.set(selected.snapshot);
      sessionViewSnapshots.current.claimWriter(scopedViewIdentity(target), tabId);
    }
    dispatchTo(tabId, { type: "backend_activation_done" });
  }, [dispatchTo, scopedViewIdentity, selectSessionView]);

  const trackBackendActivation = useCallback((tabId: string, promise: Promise<boolean>) => {
    backendActivationPromises.current.set(tabId, promise);
    void promise.finally(() => {
      if (backendActivationPromises.current.get(tabId) === promise) {
        backendActivationPromises.current.delete(tabId);
      }
    });
  }, []);

  const waitForBackendActiveTab = useCallback(async (tabId: string): Promise<boolean> => {
    const pending = backendActivationPromises.current.get(tabId);
    if (pending) {
      const activated = await pending.catch(() => false);
      if (!activated) return false;
    }
    return backendActiveTabIdRef.current === tabId && activeTabIdRef.current === tabId;
  }, []);

  const checkpointRefreshSeq = useRef(new Map<string, number>());
  const bumpSessionLoadSeq = useCallback((tabId: string): number => {
    const seq = (sessionLoadSeq.current.get(tabId) ?? 0) + 1;
    sessionLoadSeq.current.set(tabId, seq);
    return seq;
  }, []);
  const sessionLoadCurrent = useCallback((tabId: string, seq: number): boolean => {
    return sessionLoadSeq.current.get(tabId) === seq;
  }, []);
  const clearContextRefreshTimer = useCallback((tabId: string) => {
    const timer = contextRefreshTimers.current.get(tabId);
    if (timer === undefined) return;
    window.clearTimeout(timer);
    contextRefreshTimers.current.delete(tabId);
  }, []);
  const scheduleContextRefresh = useCallback((tabId: string, seq: number) => {
    clearContextRefreshTimer(tabId);
    const delays = [250, 1000, 3000];
    const attempt = (index: number) => {
      if (!sessionLoadCurrent(tabId, seq) || index >= delays.length) return;
      const timer = window.setTimeout(() => {
        contextRefreshTimers.current.delete(tabId);
        if (!sessionLoadCurrent(tabId, seq)) return;
        void app.ContextUsageForTab(tabId)
          .then((context) => {
            if (sessionLoadCurrent(tabId, seq)) dispatchTo(tabId, { type: "context", context });
          })
          .catch(() => attempt(index + 1));
      }, delays[index]);
      contextRefreshTimers.current.set(tabId, timer);
    };
    attempt(0);
  }, [clearContextRefreshTimer, dispatchTo, sessionLoadCurrent]);
  const bumpCheckpointRefreshSeq = useCallback((tabId: string): number => {
    const seq = (checkpointRefreshSeq.current.get(tabId) ?? 0) + 1;
    checkpointRefreshSeq.current.set(tabId, seq);
    return seq;
  }, []);
  const refreshCheckpoints = useCallback(async (tabId: string) => {
    const seq = bumpCheckpointRefreshSeq(tabId);
    const checkpoints = await app.CheckpointsForTab(tabId).catch(() => undefined);
    if (checkpointRefreshSeq.current.get(tabId) !== seq || checkpoints === undefined) return;
    dispatchTo(tabId, { type: "checkpoints", checkpoints: asArray(checkpoints) });
  }, [bumpCheckpointRefreshSeq, dispatchTo]);

  const applyRuntimeStatusForTab = useCallback((tabId: string, tab: TabMeta) => {
    dispatchTo(tabId, backendStatusActionFromTab(tab));
    if (tab.pendingPrompt) replayPendingPromptsForActiveTab(tabId);
  }, [dispatchTo]);

  const turnIdentityIsCurrent = useCallback((tabId: string, identity: TurnIdentity): boolean => {
    const current = statesRef.current.get(tabId);
    return Boolean(
      current?.running
      && current.turnActive
      && current.sessionGen === identity.sessionGen
      && current.turnStartAt === identity.turnStartAt
    );
  }, []);

  const queryRuntimeStatusForTurn = useCallback(async (
    tabId: string,
    identity: TurnIdentity,
    isCurrent: () => boolean,
  ): Promise<"running" | "recovered" | "unavailable" | "stale"> => {
    const queryIsCurrent = () => isCurrent() && turnIdentityIsCurrent(tabId, identity);
    if (!queryIsCurrent()) return "stale";
    let tabs: TabMeta[];
    try {
      tabs = asArray(await app.ListTabs());
    } catch {
      return queryIsCurrent() ? "unavailable" : "stale";
    }
    if (!queryIsCurrent()) return "stale";
    const tab = tabs.find((candidate) => candidate.id === tabId);
    if (!tab) return "unavailable";
    if (!queryIsCurrent()) return "stale";
    applyRuntimeStatusForTab(tabId, tab);
    return foregroundRunningFromRuntimeMeta(tab) ? "running" : "recovered";
  }, [applyRuntimeStatusForTab, turnIdentityIsCurrent]);

  const refreshRuntimeStatusForTab = useCallback(async (tabId: string, isCurrent: () => boolean = () => true): Promise<TabMeta[]> => {
    const tabs = asArray(await app.ListTabs().catch(() => [] as TabMeta[]));
    if (!isCurrent()) return tabs;
    const tab = tabs.find((candidate) => candidate.id === tabId);
    if (tab) applyRuntimeStatusForTab(tabId, tab);
    return tabs;
  }, [applyRuntimeStatusForTab]);

  const waitForTabReady = useCallback(async (tabId: string): Promise<void> => {
    await waitForBackendReady(async () => {
      const tabs = asArray(await app.ListTabs().catch(() => [] as TabMeta[]));
      const tab = tabs.find((candidate) => candidate.id === tabId);
      return !tab || tab.ready || Boolean(tab.startupErr);
    });
  }, []);

  const loadSessionDataForTab = useCallback(async (
    tabId: string,
    reset = false,
    reason: HydrateReason = "startup",
    options: { skipHistory?: boolean; placeholderItems?: Item[]; placeholderTabId?: string; preserveCachedHistory?: boolean; sessionPath?: string; surfaceIntent?: SessionSurfaceIntent; waitUntilReady?: boolean; surfaceTransitionSeq?: number } = {},
  ) => {
    const seq = bumpSessionLoadSeq(tabId);
    const viewTarget = sessionViewTargetByTab.current.get(tabId);
    const historyWriter = viewTarget ? sessionViewSnapshots.current.writer(scopedViewIdentity(viewTarget), tabId) : undefined;
    clearContextRefreshTimer(tabId);
    const hydrateStartedAt = Date.now();
    const currentState = statesRef.current.get(tabId);
    const skipHistory = Boolean(
      options.skipHistory ||
      (options.preserveCachedHistory && !reset && hasReusableCachedTranscript(currentState, options.sessionPath)),
    );
    const currentSessionPath = currentState?.meta?.sessionPath?.trim() ?? "";
    const targetSessionPath = (options.sessionPath ?? currentState?.meta?.sessionPath)?.trim() ?? "";
    const currentSurfaceIntent = currentSessionPath === targetSessionPath
      ? currentState?.surfaceIntent
      : undefined;
    const surfaceIntent = options.surfaceIntent
      ?? currentSurfaceIntent
      ?? surfaceIntentForTarget(targetSessionPath);
    addBreadcrumb("tab.hydrate", `start ${reason} ${tabId}`);
    dispatchTo(tabId, {
      type: "hydrate_start",
      reason,
      surfaceIntent,
      placeholderItems: options.placeholderItems,
      placeholderTabId: options.placeholderTabId,
      preserveHistory: skipHistory,
    });
    const itemsBeforeHistoryLoad = statesRef.current.get(tabId)?.items;

    const stillCurrent = () => sessionLoadCurrent(tabId, seq);
    const noteFailure = (label: string, err: unknown) => {
      addBreadcrumb("tab.hydrate", `${label} failed ${tabId}: ${errorMessage(err)}`);
    };
    void refreshComposerSelectionForTab(tabId, { noteFailure });

    const historyStartedAt = Date.now();

    // Phase 1: history and the combined model/effort identity start together.
    // The identity publishes from its own promise as soon as both parts settle,
    // without waiting for transcript or ancillary controller data.
    let historyFailure = "";
    const replayLoad = { seq, events: [] as WireEvent[] };
    transcriptLoads.current.set(tabId, replayLoad);
    const replayCursor = !reset && currentState?.hydrateHistoryLoaded && currentState.transcriptJournalId
      ? { journalId: currentState.transcriptJournalId, through: currentState.transcriptThrough }
      : undefined;
    const replayFailure = (err: unknown) => {
      historyFailure = errorMessage(err);
      noteFailure("transcript database", err);
      return undefined;
    };
    let replay = await readTranscriptReplay(tabId, stillCurrent, replayCursor).catch(replayFailure);
    if (replay && stillCurrent() && transcriptReplayNeedsFullRestore(replay, statesRef.current.get(tabId))) {
      addBreadcrumb("tab.hydrate", `rebuild out-of-order transcript ${tabId}`);
      replay = await readTranscriptReplay(tabId, stillCurrent).catch(replayFailure);
    }
    const clearReplayLoad = () => {
      if (transcriptLoads.current.get(tabId) === replayLoad) transcriptLoads.current.delete(tabId);
    };
    if (!stillCurrent()) {
      clearReplayLoad();
      return;
    }
    // A deleted journal invalidates the cached transcript even when this load
    // would normally preserve it (for example, after a rewind in another view).
    const needsHistory = !skipHistory || Boolean(replayCursor && !replay);
    let history = !needsHistory || replay || historyFailure
      ? undefined
      : await historyPageForTab(tabId).catch((err) => {
        historyFailure = errorMessage(err);
        noteFailure("history", err);
        return undefined;
      });

    // Restore the previously visible range using the new owner's page cursor.
    const previewBefore = currentState?.hydratePreview?.historyBefore;
    while (history && previewBefore !== undefined && history.hasMore && history.before > previewBefore && stillCurrent()) {
      const previous = history;
      const older = await historyPageForTab(tabId, previous.before, previous.generation).catch((err) => {
        historyFailure = errorMessage(err);
        noteFailure("history", err);
        return undefined;
      });
      if (!older) { history = undefined; break; }
      if (older.before >= previous.before || older.generation !== previous.generation) {
        historyFailure = "History changed while restoring the visible range.";
        history = undefined;
        break;
      }
      history = { ...older, messages: [...asArray(older.messages), ...asArray(previous.messages)] };
    }

    if (!stillCurrent()) {
      clearReplayLoad();
      return;
    }
    if (replay) {
      dispatchTo(tabId, { type: "transcript_replay", replay, pending: replayLoad.events }, historyWriter ?? null);
      addBreadcrumb("tab.hydrate", `database transcript restored ${tabId} events=${replay.events.length}`);
    } else if (history !== undefined) {
      const messages = asArray<HistoryMessage>(history.messages);
      const stateAtHistoryResolution = statesRef.current.get(tabId);
      const runtimeItemsChanged = stateAtHistoryResolution?.items !== itemsBeforeHistoryLoad;
      const preserveRuntimeTail = runtimeItemsChanged || hasActiveRuntimeTail(stateAtHistoryResolution);
      if (reset && !preserveRuntimeTail) dispatchTo(tabId, { type: "hydrate_reset" });
      if (preserveRuntimeTail) {
        dispatchTo(tabId, {
          type: "history_with_runtime_tail",
          messages,
          fingerprint: prepareHistoryRevalidation(undefined, messages).fingerprint,
          before: history.before,
          hasMore: history.hasMore,
          generation: history.generation,
        }, historyWriter ?? null);
      } else {
        dispatchTo(tabId, { type: "history", messages, before: history.before, hasMore: history.hasMore, generation: history.generation }, historyWriter ?? null);
      }
      addBreadcrumb(
        "tab.hydrate",
        `history done ${tabId} count=${messages.length} ms=${Date.now() - historyStartedAt}`,
      );
      if (reason === "switch-tab") {
        addBreadcrumb(
          "tab.switch",
          `history-done ${tabId} count=${messages.length} ms=${Date.now() - historyStartedAt}`,
        );
      }
    } else if (skipHistory) {
      const skipReason = options.skipHistory ? "cached-live-turn" : "cached-transcript";
      addBreadcrumb("tab.hydrate", `history skipped ${tabId} reason=${skipReason}`);
      if (reason === "switch-tab") {
        addBreadcrumb("tab.switch", `history-done ${tabId} skipped ms=${Date.now() - historyStartedAt}`);
      }
    }

    clearReplayLoad();
    if (historyFailure) {
      dispatchTo(tabId, { type: "hydrate_error", reason, error: historyFailure });
    } else {
      dispatchTo(tabId, { type: "hydrate_done" });
    }
    if (options.surfaceTransitionSeq !== undefined) {
      finishSurfaceTransition(options.surfaceTransitionSeq);
    }

    // Saved history is disk-backed and does not require a ready controller.
    // Render it first; only controller-owned ancillary state waits for startup.
    if (options.waitUntilReady) {
      await waitForTabReady(tabId);
      if (!stillCurrent()) return;
    }

    // Phase 2: ancillary data (checkpoints, context, balance, jobs).
    // These don't block the transcript, so they're batched into one render.
    const [jobs, checkpoints, context, balance] = await Promise.all([
      app.JobsForTab(tabId).catch((err) => { noteFailure("jobs", err); return undefined; }),
      app.CheckpointsForTab(tabId).catch((err) => { noteFailure("checkpoints", err); return undefined; }),
      app.ContextUsageForTab(tabId).catch((err) => { noteFailure("context", err); return undefined; }),
      app.BalanceForTab(tabId).catch((err) => { noteFailure("balance", err); return undefined; }),
    ]);
    if (!stillCurrent()) return;
    if (jobs !== undefined) dispatchTo(tabId, { type: "jobs", jobs: asArray(jobs) });
    if (checkpoints !== undefined) dispatchTo(tabId, { type: "checkpoints", checkpoints: asArray(checkpoints) });
    if (context !== undefined) dispatchTo(tabId, { type: "context", context });
    else scheduleContextRefresh(tabId, seq);
    if (balance !== undefined) dispatchTo(tabId, { type: "balance", balance });
    void refreshRuntimeStatusForTab(tabId, stillCurrent);

    addBreadcrumb("tab.hydrate", `done ${reason} ${tabId} ms=${Date.now() - hydrateStartedAt}`);
  }, [bumpSessionLoadSeq, clearContextRefreshTimer, dispatchTo, finishSurfaceTransition, refreshComposerSelectionForTab, refreshRuntimeStatusForTab, scheduleContextRefresh, scopedViewIdentity, sessionLoadCurrent, waitForTabReady]);

  const retryHistoryLoad = useCallback(async () => {
    const tabId = activeTabIdRef.current;
    const state = tabId ? statesRef.current.get(tabId) : undefined;
    if (!tabId || !state?.hydrateError || state.hydrating || surfaceTransitionRef.current) return;
    await loadSessionDataForTab(tabId, false, state.hydrateReason ?? "open-topic", {
      sessionPath: state.meta?.sessionPath,
      waitUntilReady: true,
    });
  }, [loadSessionDataForTab]);

  const activeTabFromBackend = useCallback(async (): Promise<TabMeta | undefined> => {
    const tabs = asArray(await app.ListTabs().catch(() => [] as TabMeta[]));
    return tabs.find((tab) => tab.active) ?? tabs[0];
  }, []);

  const syncActiveTabFromBackend = useCallback(async (reset = false, guard = false): Promise<string | undefined> => {
    const active = await activeTabFromBackend();
    if (!active) return undefined;
    // When guard is true, skip if the frontend already settled on a
    // different tab while we were fetching — this prevents fire-and-forget
    // calls from mount/onReady from overwriting a user-initiated tab switch
    // (e.g. handleNewTab → ensureBlankSurface / switchTab).
    if (guard && activeTabIdRef.current && activeTabIdRef.current !== active.id) {
      return active.id;
    }
    setActiveTabId(active.id);
    activeTabIdRef.current = active.id;
    const target = surfaceTargetFromTab(active, sessionViewTargetByTab.current.get(active.id));
    if (!statesRef.current.has(active.id)) {
      statesRef.current.set(active.id, coldStateForSurfaceTarget(target));
    }
    rememberTabView(active.id, target, active.topicTitle);
    confirmBackendActiveTab(active.id);
    dispatchTo(active.id, { type: "optimistic_meta", meta: metaFromTab(active, statesRef.current.get(active.id)?.meta) });
    await loadSessionDataForTab(active.id, reset, "startup", {
      sessionPath: active.sessionPath,
      surfaceIntent: surfaceIntentForTarget(active.sessionPath, target.sessionTurns),
    });
    return active.id;
  }, [activeTabFromBackend, confirmBackendActiveTab, dispatchTo, loadSessionDataForTab, rememberTabView]);

  const reconcileTabRuntime = useCallback(async (
    tabId: string,
    options: { hydrateSessionData?: boolean; isCurrent?: () => boolean } = {},
  ): Promise<TabMeta[] | undefined> => {
    const hydrateSessionData = options.hydrateSessionData ?? true;
    const isCurrent = options.isCurrent ?? (() => true);
    if (!isCurrent()) return undefined;
    const tabs = asArray(await app.ListTabs().catch(() => [] as TabMeta[]));
    if (!isCurrent()) return undefined;
    const tab = tabs.find((candidate) => candidate.id === tabId);
    if (!tab) return undefined;
    const local = statesRef.current.get(tabId);
    const needsInitialLoad = !local?.meta;
    const foregroundRunning = foregroundRunningFromRuntimeMeta(tab);
    const missedTurnDone = Boolean(local?.running && !foregroundRunning);
    if (!isCurrent()) return undefined;
    applyRuntimeStatusForTab(tabId, tab);
    if (hydrateSessionData && (needsInitialLoad || missedTurnDone)) {
      await loadSessionDataForTab(tabId, missedTurnDone, "startup");
      return tabs;
    }
    if (!isCurrent()) return tabs;
    void refreshComposerSelectionForTab(tabId);
    const [jobs, balance] = await Promise.all([
      app.JobsForTab(tabId).catch(() => undefined),
      app.BalanceForTab(tabId).catch(() => undefined),
    ]);
    if (!isCurrent()) return tabs;
    if (jobs) dispatchTo(tabId, { type: "jobs", jobs: asArray(jobs) });
    if (balance) dispatchTo(tabId, { type: "balance", balance });
    return tabs;
  }, [dispatchTo, loadSessionDataForTab, refreshComposerSelectionForTab]);

  const clearCancelReconcileTimer = useCallback((tabId: string) => {
    const timer = cancelReconcileTimers.current.get(tabId);
    if (timer === undefined) return;
    window.clearTimeout(timer);
    cancelReconcileTimers.current.delete(tabId);
  }, []);

  const scheduleCancelReconcile = useCallback((tabId: string, attempt = 0) => {
    clearCancelReconcileTimer(tabId);
    const delay = CANCEL_RECONCILE_DELAYS_MS[Math.min(attempt, CANCEL_RECONCILE_DELAYS_MS.length - 1)];
    const timer = window.setTimeout(() => {
      cancelReconcileTimers.current.delete(tabId);
      void reconcileTabRuntime(tabId, { hydrateSessionData: false }).then((tabs) => {
        const tab = tabs?.find((candidate) => candidate.id === tabId);
        if (!tab) return;
        const stillReconciling = foregroundRunningFromRuntimeMeta(tab) || Boolean(tab.cancelRequested);
        if (stillReconciling && attempt + 1 < CANCEL_RECONCILE_DELAYS_MS.length) {
          scheduleCancelReconcile(tabId, attempt + 1);
        }
      }).catch(() => {});
    }, delay);
    cancelReconcileTimers.current.set(tabId, timer);
  }, [clearCancelReconcileTimer, reconcileTabRuntime]);

  const invalidateTerminalReconcile = useCallback((tabId: string) => {
    const timer = terminalReconcileTimers.current.get(tabId);
    if (timer !== undefined) window.clearTimeout(timer);
    terminalReconcileTimers.current.delete(tabId);
    terminalReconcileSeqByTab.current.set(
      tabId,
      (terminalReconcileSeqByTab.current.get(tabId) ?? 0) + 1,
    );
  }, []);

  const scheduleTerminalReconcile = useCallback((
    tabId: string,
    identity: TurnIdentity,
    attempt = 0,
    seq?: number,
  ) => {
    const chainSeq = seq ?? (terminalReconcileSeqByTab.current.get(tabId) ?? 0) + 1;
    if (seq === undefined) {
      const previous = terminalReconcileTimers.current.get(tabId);
      if (previous !== undefined) window.clearTimeout(previous);
      terminalReconcileSeqByTab.current.set(tabId, chainSeq);
    }
    if (terminalReconcileSeqByTab.current.get(tabId) !== chainSeq) return;
    const delay = TERMINAL_RECONCILE_DELAYS_MS[Math.min(attempt, TERMINAL_RECONCILE_DELAYS_MS.length - 1)];
    if (attempt <= 2) addBreadcrumb("terminal", `scheduled attempt=${attempt} tab=${tabId}`);
    const timer = window.setTimeout(() => {
      terminalReconcileTimers.current.delete(tabId);
      if (
        terminalReconcileSeqByTab.current.get(tabId) !== chainSeq
        || !turnIdentityIsCurrent(tabId, identity)
      ) return;
      const chainIsCurrent = () => terminalReconcileSeqByTab.current.get(tabId) === chainSeq;
      void queryRuntimeStatusForTurn(tabId, identity, chainIsCurrent).then((status) => {
        if (
          terminalReconcileSeqByTab.current.get(tabId) !== chainSeq
          || status === "stale"
        ) return;
        if (status === "recovered") {
          addBreadcrumb("terminal", `recovered attempt=${attempt} tab=${tabId}`);
          invalidateTerminalReconcile(tabId);
          return;
        }
        if (!turnIdentityIsCurrent(tabId, identity)) return;
        if (attempt <= 2) addBreadcrumb("terminal", `${status === "running" ? "waiting" : "unavailable"} attempt=${attempt} tab=${tabId}`);
        scheduleTerminalReconcile(tabId, identity, attempt + 1, chainSeq);
      });
    }, delay);
    terminalReconcileTimers.current.set(tabId, timer);
  }, [invalidateTerminalReconcile, queryRuntimeStatusForTurn, turnIdentityIsCurrent]);

  useEffect(() => {
    const batchStreamEvents = isStreamBatchingEnabled();
    const textBatch = createRafBatch<RoutedStreamEvent>((batch) => {
      const eventsByTab = new Map<string, WireEvent[]>();
      for (const { tabId, e } of coalesceRoutedStreamEvents(batch)) {
        const events = eventsByTab.get(tabId);
        if (events) events.push(e);
        else eventsByTab.set(tabId, [e]);
      }
      for (const [tabId, events] of eventsByTab) {
        dispatchTo(tabId, { type: "stream_batch", events });
      }
    });
    const off = onEvent((incoming) => {
      let e = incoming;
      const targetTabId = e.tabId || activeTabIdRef.current;
      if (!targetTabId) return;
      if (e.kind === "turn_done") {
        const targetState = statesRef.current.get(targetTabId);
        if (!turnDoneMatchesStateSession(e, targetState)) {
          addBreadcrumb("event.delivery", `turn_done-stale-session tab=${targetTabId}`);
          return;
        }
        if (!turnDoneMatchesStateIdentity(e, targetState)) {
          addBreadcrumb("event.delivery", `turn_done-historical tab=${targetTabId}`);
          transcriptLoads.current.get(targetTabId)?.events.push(e);
          dispatchTo(targetTabId, { type: "historical_turn_done", e });
          return;
        }
      }
      transcriptLoads.current.get(targetTabId)?.events.push(e);
      const traceArrival = (reducerEventArrivalsByTab.current.get(targetTabId) ?? 0) + 1;
      reducerEventArrivalsByTab.current.set(targetTabId, traceArrival);
      const traceMessage = reducerEntryTraceMessage(targetTabId, e, traceArrival);
      if (traceMessage) addBreadcrumb("event.delivery", traceMessage);
      if (e.kind === "turn_started" || e.kind === "turn_done") {
        invalidateTerminalReconcile(targetTabId);
      }
      if (
        e.kind === "turn_started" ||
        e.kind === "text" ||
        e.kind === "reasoning" ||
        e.kind === "message" ||
        e.kind === "tool_dispatch" ||
        e.kind === "tool_progress" ||
        e.kind === "tool_result"
      ) {
        lastTurnActivityAtByTab.current.set(targetTabId, Date.now());
      }
      if (batchStreamEvents && (e.kind === "text" || e.kind === "reasoning")) {
        textBatch.push({ tabId: targetTabId, e });
      } else {
        textBatch.drain();
        dispatchTo(targetTabId, { type: "event", e });
      }
      if (
        e.kind === "message"
        && !(e.parentId?.trim())
        && !(e.subagentRef?.trim())
      ) {
        const current = statesRef.current.get(targetTabId);
        if (current?.running && current.turnActive) {
          scheduleTerminalReconcile(targetTabId, {
            sessionGen: current.sessionGen,
            turnStartAt: current.turnStartAt,
          });
        }
      }
      if (e.kind === "turn_done") {
        void refreshComposerSelectionForTab(targetTabId);
        void refreshContextForTab(targetTabId);
        app.BalanceForTab(targetTabId).then((balance) => dispatchTo(targetTabId, { type: "balance", balance })).catch(() => {});
        void refreshCheckpoints(targetTabId);
      }
      if (e.kind === "turn_done" || e.kind === "notice") {
        app.JobsForTab(targetTabId).then((jobs) => dispatchTo(targetTabId, { type: "jobs", jobs: asArray(jobs) })).catch(() => {});
      }
    });

    const offReady = onReady(() => {
      const readyTabId = activeTabIdRef.current;
      if (readyTabId) {
        // Ready only means the runtime binding became available. Disk-backed
        // history is already owned by the in-flight/settled view hydration;
        // starting another hydration here can supersede its sequence and strand
        // a surface transition in the pending view.
        void reconcileTabRuntime(readyTabId, { hydrateSessionData: false });
        void refreshComposerSelectionForTab(readyTabId);
        void refreshContextForTab(readyTabId);
        void refreshCheckpoints(readyTabId);
        return;
      }
      void syncActiveTabFromBackend(false, true);
    });

    void syncActiveTabFromBackend(false, true);
    // The event subscription is live now, so ask the backend to re-emit any
    // approval/ask prompt that was already blocking a tab before this load —
    // otherwise a session left mid-confirmation shows "waiting" with no modal
    // and no way to stop (#3844).
    void app.ReplayPendingPrompts().catch(() => {});

    return () => {
      textBatch.drain();
      for (const timer of cancelReconcileTimers.current.values()) {
        window.clearTimeout(timer);
      }
      cancelReconcileTimers.current.clear();
      for (const timer of terminalReconcileTimers.current.values()) {
        window.clearTimeout(timer);
      }
      terminalReconcileTimers.current.clear();
      terminalReconcileSeqByTab.current.clear();
      reducerEventArrivalsByTab.current.clear();
      for (const timer of contextRefreshTimers.current.values()) {
        window.clearTimeout(timer);
      }
      contextRefreshTimers.current.clear();
      clearSurfaceLoadingHintTimer();
      off();
      offReady();
    };
  }, [clearSurfaceLoadingHintTimer, dispatchTo, invalidateTerminalReconcile, reconcileTabRuntime, refreshCheckpoints, refreshComposerSelectionForTab, refreshContextForTab, scheduleTerminalReconcile, syncActiveTabFromBackend]);

  // Stale-turn watchdog: if the frontend thinks the agent is running but the
  // turn stream has gone quiet, reconcile with the backend. This catches cases
  // where the Wails event channel silently drops turn_done after the final
  // message or synthetic todo update has already closed the live stream.
  useEffect(() => {
    if (!activeTabId) return;
    const s = statesRef.current.get(activeTabId);
    const lastTurnActivityAt = lastTurnActivityAtByTab.current.get(activeTabId) ?? 0;
    if (!s?.running || !s.turnActive || lastTurnActivityAt <= 0) return;
    const identity = { sessionGen: s.sessionGen, turnStartAt: s.turnStartAt };
    let timer: number | undefined;
    let cancelled = false;
    let attempt = 0;
    const schedule = (minimumDelay = 0) => {
      if (cancelled || !turnIdentityIsCurrent(activeTabId, identity)) return;
      const lastActivity = lastTurnActivityAtByTab.current.get(activeTabId) ?? 0;
      const untilStale = Math.max(0, STALE_TURN_RECONCILE_MS - (Date.now() - lastActivity));
      const delay = Math.max(minimumDelay, untilStale);
      timer = window.setTimeout(() => {
        timer = undefined;
        if (cancelled || !turnIdentityIsCurrent(activeTabId, identity)) return;
        const latestActivity = lastTurnActivityAtByTab.current.get(activeTabId) ?? 0;
        if (!shouldReconcileStaleTurn(statesRef.current.get(activeTabId), latestActivity)) {
          schedule();
          return;
        }
        const currentAttempt = attempt;
        attempt += 1;
        addBreadcrumb("watchdog", `scheduled attempt=${currentAttempt} tab=${activeTabId}`);
        const effectIsCurrent = () => !cancelled && turnIdentityIsCurrent(activeTabId, identity);
        void reconcileTabRuntime(activeTabId, { isCurrent: effectIsCurrent }).then((tabs) => {
          if (cancelled) return;
          const tab = tabs?.find((candidate) => candidate.id === activeTabId);
          const status = !tab
            ? "unavailable"
            : foregroundRunningFromRuntimeMeta(tab)
              ? "waiting"
              : "recovered";
          addBreadcrumb("watchdog", `${status} attempt=${currentAttempt} tab=${activeTabId}`);
          if (status !== "recovered") schedule(STALE_TURN_RECONCILE_MS);
        });
      }, delay);
    };
    schedule();
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [activeTabId, activeState.running, activeState.sessionGen, activeState.turnActive, activeState.turnStartAt, reconcileTabRuntime, turnIdentityIsCurrent]);

  // Replay any pending approval/ask prompts when switching tabs, so a
  // plan-mode session left awaiting confirmation rebuilds its modal (#4275).
  useEffect(() => {
    replayPendingPromptsForActiveTab(activeTabId);
  }, [activeTabId]);

  const sendToTab = useCallback(async (tabId: string, displayText: string, submitText = displayText, capabilities?: import("../../lib/types").ComposerCapabilitySubmit) => {
    if (!tabId) throw new Error("workspace is still starting");
    const before = getOrCreateState(statesRef.current, tabId);
    const sessionGen = before.sessionGen;
    const sessionPath = before.meta?.sessionPath;
    await beforeSendRef.current?.(tabId);
    const current = statesRef.current.get(tabId);
    if (!current || current.sessionGen !== sessionGen || current.meta?.sessionPath !== sessionPath) {
      throw new Error("conversation changed while preparing to send");
    }
    const seq = current.seq;
    const display = displayText.trim();
    const submit = submitText.trim();
    dispatchTo(tabId, { type: "user", text: displayText, submitText: display !== submit ? submit : undefined, seq });
    invalidateCache();
    try {
      await (capabilities
        ? app.SubmitCapabilitiesToTab(tabId, display, { ...capabilities, text: submit })
        : display !== submit
          ? app.SubmitDisplayToTab(tabId, display, submit)
          : app.SubmitToTab(tabId, submit));
    } catch (error) {
      dispatchTo(tabId, {
        type: "send_failed",
        error: visibleErrorMessage("session.send", error, "发送消息失败，请稍后重试。"),
      });
      throw error;
    }
  }, [dispatchTo]);

  const send = useCallback((displayText: string, submitText = displayText, capabilities?: import("../../lib/types").ComposerCapabilitySubmit) => {
    const renderedState = stateRef.current;
    if (
      surfaceTransitionRef.current
      || (
        renderedState.surfaceIntent === "existing-session"
        && (
          renderedState.hydrating
          || renderedState.hydrateHistoryLoaded !== true
          || renderedState.backendActivationPending
          || renderedState.meta?.ready !== true
        )
      )
    ) return Promise.reject(new Error("session view is still loading"));
    const tabId = activeTabIdRef.current ?? activeTabId;
    if (tabId) {
      return sendToTab(tabId, displayText, submitText, capabilities);
    }
    return activeTabFromBackend().then((active) => {
      if (!active?.id) throw new Error("workspace is still starting");
      setActiveTabId(active.id);
      activeTabIdRef.current = active.id;
      confirmBackendActiveTab(active.id);
      return sendToTab(active.id, displayText, submitText, capabilities);
    });
  }, [activeTabFromBackend, activeTabId, confirmBackendActiveTab, sendToTab]);

  const runShell = useCallback(async (command: string) => {
    if (surfaceTransitionRef.current) throw new Error("session view is still loading");
    if (!activeTabId) throw new Error("workspace is still starting");
    dispatchTo(activeTabId, { type: "user", text: `!${command}`, seq: getOrCreateState(statesRef.current, activeTabId).seq });
    try {
      await app.RunShellForTab(activeTabId, command);
    } catch (error) {
      dispatchTo(activeTabId, {
        type: "send_failed",
        error: visibleErrorMessage("session.command", error, "执行终端命令失败，请稍后重试。"),
      });
      throw error;
    }
  }, [activeTabId, dispatchTo]);

  const steer = useCallback(async (text: string) => {
    if (surfaceTransitionRef.current) throw new Error("session view is still loading");
    if (!activeTabId) throw new Error("workspace is still starting");
    // No optimistic user bubble: rewind/fork map turns by counting user items,
    // and a steer is not a backend turn — the Steer event's ↪ notice is the
    // visible confirmation (#3660).
    try {
      await app.SteerForTab(activeTabId, text);
    } catch (error) {
      dispatchTo(activeTabId, {
        type: "local_notice",
        level: "warn",
        text: visibleErrorMessage("session.steer", error, "追加指令失败，请稍后重试。"),
      });
      throw error;
    }
  }, [activeTabId, dispatchTo]);

  const notice = useCallback((text: string, level: "info" | "warn" = "info") => {
    if (!activeTabId) return;
    dispatchTo(activeTabId, { type: "local_notice", level, text });
  }, [activeTabId, dispatchTo]);

  const cancelTab = useCallback((tabId: string) => {
    app.CancelTab(tabId)
      .then(() => scheduleCancelReconcile(tabId))
      .catch((error) => {
        dispatchTo(tabId, {
          type: "local_notice",
          level: "warn",
          text: visibleErrorMessage("session.cancel", error, "取消当前操作失败，请稍后重试。"),
        });
      });
  }, [dispatchTo, scheduleCancelReconcile]);

  const cancelForTab = useCallback((tabId: string): string | undefined => {
    const cur = tabId ? (statesRef.current.get(tabId) ?? stateRef.current) : stateRef.current;
    return cancelTabTurn(cur, {
      unsend: () => dispatchTo(tabId, { type: "unsend" }),
      markRequested: () => dispatchTo(tabId, { type: "cancel_requested" }),
      cancel: () => cancelTab(tabId),
    });
  }, [cancelTab, dispatchTo]);

  const cancel = useCallback((): string | undefined => {
    const tabId = activeTabIdRef.current ?? activeTabId;
    return tabId ? cancelForTab(tabId) : undefined;
  }, [activeTabId, cancelForTab]);

  const applyReplyReceipt = useCallback((tabId: string, receipt: PromptReplyReceipt | void) => {
    if (!receipt?.promptId) return;
    dispatchTo(tabId, { type: "prompt_reply_receipt", receipt });
    if (receipt.status === "already_resolved") {
      void app.ReplayPendingPrompts().catch(() => {});
    }
  }, [dispatchTo]);

  const recordReplyFailure = useCallback(async (tabId: string, promptId: string) => {
    try {
      const receipt = await app.RecordPromptReplyFailureForTab(tabId, promptId);
      if (receipt?.promptId) {
        applyReplyReceipt(tabId, receipt);
        return;
      }
    } catch {
      // A partially rebound Desktop runtime may not expose the new binding.
    }
    dispatchTo(tabId, { type: "prompt_reply_failed", promptId, occurrenceId: `frontend-only:reply:${Date.now().toString(36)}` });
  }, [applyReplyReceipt, dispatchTo]);

  const approve = useCallback(async (id: string, allow: boolean, session: boolean, persist: boolean, ownerTabId?: string): Promise<PromptReplyReceipt | void> => {
    const tabId = ownerTabId || activeTabIdRef.current || activeTabId;
    if (!tabId) return;
    dispatchTo(tabId, { type: "prompt_reply_started", promptId: id, input: { kind: "approval", allow, session, persist } });
    try {
      const receipt = await app.ApproveTab(tabId, id, allow, session, persist);
      applyReplyReceipt(tabId, receipt);
      return receipt;
    } catch {
      await recordReplyFailure(tabId, id);
    }
  }, [activeTabId, applyReplyReceipt, dispatchTo, recordReplyFailure]);

  const answerQuestion = useCallback(async (id: string, answers: QuestionAnswer[], ownerTabId?: string) => {
    const tabId = ownerTabId || activeTabIdRef.current || activeTabId;
    if (!tabId) return;
    dispatchTo(tabId, { type: "prompt_reply_started", promptId: id, input: { kind: "ask", answers } });
    try {
      applyReplyReceipt(tabId, await app.AnswerQuestionForTab(tabId, id, answers));
    } catch {
      await recordReplyFailure(tabId, id);
    }
  }, [activeTabId, applyReplyReceipt, dispatchTo, recordReplyFailure]);

  const setControllerMode = useCallback((mode: Mode): Promise<void> => {
    if (!activeTabId) return Promise.resolve();
    return app.SetModeForTab(activeTabId, mode).then(() => {
      if (modeHasAutoApproveTools(mode) && activeTabId) dispatchTo(activeTabId, { type: "clearApproval" });
    }).catch(() => {});
  }, [activeTabId, dispatchTo]);

  const setCollaborationMode = useCallback(async (mode: CollaborationMode): Promise<void> => {
    if (!activeTabId) return;
    await app.SetCollaborationModeForTab(activeTabId, mode).catch(() => {});
    await Promise.all([refreshComposerSelectionForTab(activeTabId), refreshContextForTab(activeTabId)]);
  }, [activeTabId, refreshComposerSelectionForTab, refreshContextForTab]);

  const setToolApprovalMode = useCallback(async (mode: ToolApprovalMode): Promise<void> => {
    if (!activeTabId) return;
    await app.SetToolApprovalModeForTab(activeTabId, mode);
    if (mode === "auto" || mode === "yolo") dispatchTo(activeTabId, { type: "clearApproval" });
    await Promise.all([refreshComposerSelectionForTab(activeTabId), refreshContextForTab(activeTabId)]).catch(() => {});
  }, [activeTabId, dispatchTo, refreshComposerSelectionForTab, refreshContextForTab]);

  const syncToolApprovalMode = useCallback(async (mode: ToolApprovalMode): Promise<void> => {
    if (!activeTabId) return;
    await app.SyncToolApprovalModeForTab(activeTabId, mode);
    if (mode === "auto" || mode === "yolo") dispatchTo(activeTabId, { type: "clearApproval" });
  }, [activeTabId, dispatchTo]);

  const setGoal = useCallback(async (goal: string): Promise<void> => {
    if (!activeTabId) return;
    await app.SetGoalForTab(activeTabId, goal).catch(() => {});
    await Promise.all([refreshComposerSelectionForTab(activeTabId), refreshContextForTab(activeTabId)]);
  }, [activeTabId, refreshComposerSelectionForTab, refreshContextForTab]);

  const clearGoal = useCallback(async (): Promise<void> => {
    if (!activeTabId) return;
    await app.ClearGoalForTab(activeTabId).catch(() => {});
    await Promise.all([refreshComposerSelectionForTab(activeTabId), refreshContextForTab(activeTabId)]);
  }, [activeTabId, refreshComposerSelectionForTab, refreshContextForTab]);

  const newSession = useCallback(async () => {
    const tabId = activeTabId;
    if (tabId && !(await waitForBackendActiveTab(tabId))) return;
    if (tabId) {
      addBreadcrumb("session.new", `click ${tabId}`);
      bumpCheckpointRefreshSeq(tabId);
      bumpSessionLoadSeq(tabId);
      dispatchTo(tabId, { type: "reset", surfaceIntent: "blank" });
      dispatchTo(tabId, { type: "hydrate_start", reason: "new-session", surfaceIntent: "blank" });
      addBreadcrumb("session.new", `visible-reset ${tabId}`);
    }
    try {
      await app.NewSession();
      addBreadcrumb("session.new", `backend-done ${tabId ?? ""}`);
    } catch (err) {
      if (tabId) {
        const message = visibleErrorMessage("session.new", err, "创建新会话失败，请稍后重试。");
        dispatchTo(tabId, { type: "hydrate_error", reason: "new-session", error: message });
        void loadSessionDataForTab(tabId, true, "new-session", { surfaceIntent: "blank" }).then(() => {
          dispatchTo(tabId, { type: "local_notice", level: "warn", text: message });
        });
      }
      return; // backend refused (workspace starting / failed) — keep the transcript
    }
    invalidateCache();
    if (tabId) {
      dispatchTo(tabId, { type: "history", messages: [] });
      dispatchTo(tabId, { type: "hydrate_done" });
      void refreshComposerSelectionForTab(tabId);
      void refreshContextForTab(tabId);
      void refreshCheckpoints(tabId);
    }
  }, [activeTabId, bumpCheckpointRefreshSeq, bumpSessionLoadSeq, dispatchTo, loadSessionDataForTab, refreshCheckpoints, refreshComposerSelectionForTab, refreshContextForTab, waitForBackendActiveTab]);

  const clearSession = useCallback(async () => {
    const tabId = activeTabId;
    if (tabId && !(await waitForBackendActiveTab(tabId))) {
      throw new Error("Tab activation has not completed");
    }
    if (tabId) {
      bumpCheckpointRefreshSeq(tabId);
      bumpSessionLoadSeq(tabId);
    }
    try {
      await app.ClearSession();
    } catch {
      if (tabId) void loadSessionDataForTab(tabId);
      return;
    }
    if (tabId) bumpSessionLoadSeq(tabId);
    invalidateCache();
    if (tabId) {
      dispatchTo(tabId, { type: "reset", surfaceIntent: "blank" });
      // Clear placeholder items since no history action follows.
      dispatchTo(tabId, { type: "history", messages: [] });
    }
  }, [activeTabId, bumpCheckpointRefreshSeq, bumpSessionLoadSeq, dispatchTo, loadSessionDataForTab, waitForBackendActiveTab]);

  const listSessions = useCallback(async (): Promise<SessionMeta[]> => asArray<SessionMeta>(await app.ListSessions().catch(() => [])), []);
  const listTrashedSessions = useCallback(async (): Promise<SessionMeta[]> => asArray<SessionMeta>(await app.ListTrashedSessions().catch(() => [])), []);
  const resumeSession = useCallback(async (path: string, tabId?: string, surfaceTransitionSeq?: number) => {
    const targetTabId = tabId || activeTabId;
    if (!targetTabId) return;
    if (surfaceTransitionSeq !== undefined && surfaceActivationSeq.current !== surfaceTransitionSeq) return;
    if (!tabId && !(await waitForBackendActiveTab(targetTabId))) return;
    if (surfaceTransitionSeq !== undefined && surfaceActivationSeq.current !== surfaceTransitionSeq) return;
    bindSessionPathToTab(targetTabId, path);
    const seq = bumpSessionLoadSeq(targetTabId);
    const stillCurrent = () => sessionLoadCurrent(targetTabId, seq)
      && (surfaceTransitionSeq === undefined || surfaceActivationSeq.current === surfaceTransitionSeq);
    dispatchTo(targetTabId, { type: "hydrate_start", reason: "resume-session", surfaceIntent: "existing-session" });
    let history: HistoryPage;
    try {
      if (typeof app.ActivateSessionForTab === "function") {
        await app.ActivateSessionForTab(targetTabId, path);
        history = await historyPageForTab(targetTabId);
      } else {
        const messages = asArray(await (tabId ? app.ResumeSessionForTab(tabId, path) : app.ResumeSession(path)));
        history = { messages, before: 0, hasMore: false, generation: "" };
      }
    } catch (err) {
      if (stillCurrent()) {
        const message = visibleErrorMessage("session.resume", err, "打开历史会话失败，请稍后重试。");
        dispatchTo(targetTabId, { type: "hydrate_error", reason: "resume-session", error: message });
        dispatchTo(targetTabId, { type: "local_notice", level: "warn", text: message });
        if (surfaceTransitionSeq !== undefined) finishSurfaceTransition(surfaceTransitionSeq);
      }
      return;
    }
    if (!stillCurrent()) return;
    dispatchTo(targetTabId, { type: "reset" });
    dispatchTo(targetTabId, {
      type: "history",
      messages: asArray<HistoryMessage>(history.messages),
      before: history.before,
      hasMore: history.hasMore,
      generation: history.generation,
    });
    dispatchTo(targetTabId, { type: "hydrate_done" });
    if (surfaceTransitionSeq !== undefined) finishSurfaceTransition(surfaceTransitionSeq);
    void refreshRuntimeStatusForTab(targetTabId, stillCurrent);
    app.ContextUsageForTab(targetTabId).then((context) => dispatchTo(targetTabId, { type: "context", context })).catch(() => {});
    void refreshCheckpoints(targetTabId);
  }, [activeTabId, bindSessionPathToTab, bumpSessionLoadSeq, dispatchTo, finishSurfaceTransition, refreshCheckpoints, refreshRuntimeStatusForTab, sessionLoadCurrent, waitForBackendActiveTab, waitForTabReady]);

  const openChannelSession = useCallback(async (path: string, tabId: string, surfaceTransitionSeq?: number) => {
    if (!tabId) return;
    if (surfaceTransitionSeq !== undefined && surfaceActivationSeq.current !== surfaceTransitionSeq) return;
    await waitForTabReady(tabId);
    if (surfaceTransitionSeq !== undefined && surfaceActivationSeq.current !== surfaceTransitionSeq) return;
    bindSessionPathToTab(tabId, path);
    const seq = bumpSessionLoadSeq(tabId);
    const stillCurrent = () => sessionLoadCurrent(tabId, seq)
      && (surfaceTransitionSeq === undefined || surfaceActivationSeq.current === surfaceTransitionSeq);
    dispatchTo(tabId, { type: "hydrate_start", reason: "resume-session", surfaceIntent: "existing-session" });
    let history: HistoryPage;
    try {
      if (typeof app.ActivateChannelSessionForTab === "function") {
        await app.ActivateChannelSessionForTab(tabId, path);
        history = await historyPageForTab(tabId);
      } else {
        const messages = asArray(await app.OpenChannelSessionForTab(tabId, path));
        history = { messages, before: 0, hasMore: false, generation: "" };
      }
    } catch (err) {
      if (stillCurrent()) {
        const message = visibleErrorMessage("session.resume", err, "打开历史会话失败，请稍后重试。");
        dispatchTo(tabId, { type: "hydrate_error", reason: "resume-session", error: message });
        dispatchTo(tabId, { type: "local_notice", level: "warn", text: message });
        if (surfaceTransitionSeq !== undefined) finishSurfaceTransition(surfaceTransitionSeq);
      }
      return;
    }
    if (!stillCurrent()) return;
    dispatchTo(tabId, { type: "reset" });
    dispatchTo(tabId, {
      type: "history",
      messages: asArray<HistoryMessage>(history.messages),
      before: history.before,
      hasMore: history.hasMore,
      generation: history.generation,
    });
    dispatchTo(tabId, { type: "hydrate_done" });
    if (surfaceTransitionSeq !== undefined) finishSurfaceTransition(surfaceTransitionSeq);
    void refreshRuntimeStatusForTab(tabId, stillCurrent);
    app.ContextUsageForTab(tabId).then((context) => dispatchTo(tabId, { type: "context", context })).catch(() => {});
    void refreshCheckpoints(tabId);
  }, [bindSessionPathToTab, bumpSessionLoadSeq, dispatchTo, finishSurfaceTransition, refreshCheckpoints, refreshRuntimeStatusForTab, sessionLoadCurrent, waitForTabReady]);

  const revalidateChannelSession = useCallback(async (tabId: string, path: string) => {
    const sessionPath = path.trim();
    if (!tabId || !sessionPath || activeTabIdRef.current !== tabId) return;
    const target = sessionViewTargetByTab.current.get(tabId);
    if (!target) return;
    const expectedIdentityKey = sessionViewIdentityKey(scopedViewIdentity(target));
    if (expectedIdentityKey !== sessionViewIdentityKey(scopedViewIdentity({ ...target, sessionPath }))) return;

    const request = {
      tabId,
      activationSeq: surfaceActivationSeq.current,
      identityKey: expectedIdentityKey,
      loadSeq: bumpSessionLoadSeq(tabId),
      items: statesRef.current.get(tabId)?.items,
    };
    const history = await app.PreviewSession(sessionPath).catch((err) => {
      addBreadcrumb("tab.hydrate", `channel history failed ${tabId}: ${errorMessage(err)}`);
      return undefined;
    });
    const latestTarget = sessionViewTargetByTab.current.get(tabId);
    const latestState = statesRef.current.get(tabId);
    if (
      history === undefined
      || !historyRevalidationRequestIsCurrent(request, {
        activeTabId: activeTabIdRef.current,
        activationSeq: surfaceActivationSeq.current,
        identityKey: latestTarget
          ? sessionViewIdentityKey(scopedViewIdentity(latestTarget))
          : undefined,
        loadSeq: sessionLoadSeq.current.get(tabId),
        items: latestState?.items,
        hasActiveRuntimeTail: hasActiveRuntimeTail(latestState),
      })
    ) return;

    const messages = asArray(history);
    const revalidation = prepareHistoryRevalidation(latestState?.historyFingerprint, messages);
    if (!revalidation.changed) {
      addBreadcrumb("tab.hydrate", `channel history unchanged ${tabId}`);
      return;
    }
    dispatchTo(tabId, { type: "history_revalidated", messages, fingerprint: revalidation.fingerprint });
    cacheIdleTabView(tabId);
    addBreadcrumb("tab.hydrate", `channel history done ${tabId}`);
  }, [bumpSessionLoadSeq, cacheIdleTabView, dispatchTo, scopedViewIdentity]);

  const previewSession = useCallback(async (path: string): Promise<HistoryMessage[]> => asArray<HistoryMessage>(await app.PreviewSession(path)), []);
  const deleteSession = useCallback((path: string) => app.DeleteSession(path).finally(() => {
    sessionViewSnapshots.current.invalidateSessionPath(path);
    invalidateCache();
  }), []);
  const restoreSession = useCallback((path: string) => app.RestoreSession(path).catch(() => {}).finally(() => {
    sessionViewSnapshots.current.invalidateSessionPath(path);
    invalidateCache();
  }), []);
  const purgeTrashedSession = useCallback((path: string) => app.PurgeTrashedSession(path).catch(() => {}).finally(() => {
    sessionViewSnapshots.current.invalidateSessionPath(path);
    invalidateCache();
  }), []);
  const renameSession = useCallback((path: string, title: string) => app.RenameSession(path, title)
    .then(() => {
      sessionViewSnapshots.current.renameSession(path, title);
    })
    .catch(() => {})
    .finally(() => invalidateCache()), []);

  const refreshMeta = useCallback(async () => {
    if (!activeTabId) return;
    await Promise.all([refreshComposerSelectionForTab(activeTabId), refreshContextForTab(activeTabId)]);
  }, [activeTabId, refreshComposerSelectionForTab, refreshContextForTab]);

  const refreshWorkspaceState = useCallback(async (path: string): Promise<string> => {
    if (path) await syncActiveTabFromBackend(true);
    return path;
  }, [syncActiveTabFromBackend]);

  const pickWorkspace = useCallback(async (): Promise<string> => {
    const picked = await app.PickWorkspace().catch(() => "");
    if (!picked) return "";
    const path = await app.SwitchWorkspace(picked).catch(() => "");
    return refreshWorkspaceState(path);
  }, [refreshWorkspaceState]);
  const switchWorkspace = useCallback(async (path: string): Promise<string> => {
    const next = await app.SwitchWorkspace(path).catch(() => "");
    return refreshWorkspaceState(next);
  }, [refreshWorkspaceState]);

  const compact = useCallback(() => {
    const tabId = activeTabIdRef.current;
    if (!tabId) return;
    void waitForBackendActiveTab(tabId).then((active) => {
      if (active) app.Compact().catch(() => {});
    });
  }, [waitForBackendActiveTab]);

  const setModel = useCallback(async (name: string, billingSource?: string) => {
    if (!activeTabId) return;
    const previous = statesRef.current.get(activeTabId) ?? initialState;
    const selectionSeq = nextComposerSelectionSeq(activeTabId);
    const optimisticMeta = previous.meta ? { ...previous.meta, modelRef: name } : undefined;
    dispatchTo(activeTabId, {
      type: "composer_selection_start",
      seq: selectionSeq,
      modelRef: name,
      invalidate: true,
      meta: optimisticMeta,
    });
    try {
      if (billingSource) await app.SetModelRouteForTab(activeTabId, name, billingSource);
      else await app.SetModelForTab(activeTabId, name);
    } catch (err) {
      dispatchTo(activeTabId, {
        type: "composer_selection_restore",
        seq: selectionSeq,
        meta: previous.meta,
        effort: previous.effort,
        selection: previous.composerSelection,
      });
      dispatchTo(activeTabId, {
        type: "local_notice",
        level: "warn",
        text: visibleErrorMessage("session.switchModel", err, "切换模型失败，请稍后重试。"),
      });
      return;
    }
    await Promise.all([
      refreshComposerSelectionForTab(activeTabId, { seq: selectionSeq, modelRef: name, invalidate: true }),
      refreshContextForTab(activeTabId),
    ]);
  }, [activeTabId, dispatchTo, nextComposerSelectionSeq, refreshComposerSelectionForTab, refreshContextForTab]);

  const retryTabStartup = useCallback(async (): Promise<void> => {
    if (!activeTabId) return;
    try {
      const tab = await app.RetryTabStartup(activeTabId);
      dispatchTo(activeTabId, { type: "optimistic_meta", meta: metaFromTab(tab, statesRef.current.get(activeTabId)?.meta) });
      await loadSessionDataForTab(activeTabId, false, "startup", { preserveCachedHistory: true });
    } catch (err) {
      dispatchTo(activeTabId, {
        type: "local_notice",
        level: "warn",
        text: visibleErrorMessage("session.refresh", err, "刷新会话失败，请稍后重试。"),
      });
    }
  }, [activeTabId, dispatchTo, loadSessionDataForTab]);

  const setEffort = useCallback(async (level: string) => {
    if (!activeTabId) return;
    const previous = statesRef.current.get(activeTabId) ?? initialState;
    if (
      previous.composerSelection.phase !== "ready"
      || !previous.effort?.supported
      || previous.effort.current === level
    ) return;
    const selectionSeq = nextComposerSelectionSeq(activeTabId);
    const modelRef = previous.composerSelection.modelRef || previous.meta?.modelRef?.trim() || "";
    dispatchTo(activeTabId, {
      type: "composer_effort_update_start",
      seq: selectionSeq,
      modelRef,
      effort: { ...previous.effort, current: level },
    });
    try {
      await app.SetEffortForTab(activeTabId, level);
    } catch (err) {
      dispatchTo(activeTabId, {
        type: "composer_selection_restore",
        seq: selectionSeq,
        meta: previous.meta,
        effort: previous.effort,
        selection: previous.composerSelection,
      });
      dispatchTo(activeTabId, {
        type: "local_notice",
        level: "warn",
        text: visibleErrorMessage("session.switchEffort", err, "切换推理强度失败，请稍后重试。"),
      });
      return;
    }
    dispatchTo(activeTabId, { type: "composer_effort_update_done", seq: selectionSeq });
    await refreshContextForTab(activeTabId);
  }, [activeTabId, dispatchTo, nextComposerSelectionSeq, refreshContextForTab]);

  const setTokenMode = useCallback(async (mode: TokenMode) => {
    if (!activeTabId) return;
    try {
      await app.SetTokenModeForTab(activeTabId, mode);
    } catch (err) {
      dispatchTo(activeTabId, {
        type: "local_notice",
        level: "warn",
        text: visibleErrorMessage("session.switchTokenMode", err, "切换上下文模式失败，请稍后重试。"),
      });
      return;
    }
    await Promise.all([refreshComposerSelectionForTab(activeTabId), refreshContextForTab(activeTabId)]);
  }, [activeTabId, dispatchTo, refreshComposerSelectionForTab, refreshContextForTab]);

  const fetchMemory = useCallback((): Promise<MemoryView> =>
    app.Memory().catch(() => ({ docs: [], facts: [], archives: [], scopes: [], storeDir: "", available: false })), []);
  const remember = useCallback(async (scope: string, note: string) => { await app.Remember(scope, note); }, []);
  const forget = useCallback(async (name: string) => { await app.Forget(name); }, []);
  const saveDoc = useCallback(async (path: string, body: string) => { await app.SaveDoc(path, body); }, []);

  const rewind = useCallback(async (turn: number, scope: string): Promise<boolean> => {
    if (surfaceTransitionRef.current) return false;
    const sourceTabId = activeTabId;
    if (!sourceTabId) return false;
    if (!(await waitForBackendActiveTab(sourceTabId))) return false;
    const actionScope = (["fork", "summ-from", "summ-upto", "conversation", "code", "both"].includes(scope) ? scope : "both") as MessageActionScope;
    dispatchTo(sourceTabId, { type: "message_action_start", action: { turn, scope: actionScope } });
    dispatchTo(sourceTabId, { type: "local_notice", level: "info", text: messageActionBusyText(actionScope) });
    try {
      if (actionScope === "fork") {
        const tab = await app.Fork(turn);
        if (tab?.id) {
          setActiveTabId(tab.id);
          activeTabIdRef.current = tab.id;
          confirmBackendActiveTab(tab.id);
          // The fork's controller builds in a background goroutine: an immediate
          // load reads empty history, and the ready-event fallback can still
          // target the source tab, leaving the fork blank (#3742).
          await waitForTabReady(tab.id);
          await loadSessionDataForTab(tab.id, true);
        } else {
          await syncActiveTabFromBackend(true);
        }
        return true;
      }

      bumpSessionLoadSeq(sourceTabId);
      const target = sessionViewTargetByTab.current.get(sourceTabId);
      if (target) sessionViewSnapshots.current.invalidate(scopedViewIdentity(target));
      if (actionScope === "summ-from") await app.SummarizeFrom(turn);
      else if (actionScope === "summ-upto") await app.SummarizeUpTo(turn);
      else await app.Rewind(turn, actionScope);

      const history = await historyPageForTab(sourceTabId);
      dispatchTo(sourceTabId, { type: "reset" });
      dispatchTo(sourceTabId, {
        type: "history",
        messages: asArray<HistoryMessage>(history.messages),
        before: history.before,
        hasMore: history.hasMore,
        generation: history.generation,
      });
      dispatchTo(sourceTabId, { type: "context", context: await app.ContextUsageForTab(sourceTabId) });
      dispatchTo(sourceTabId, { type: "checkpoints", checkpoints: asArray(await app.CheckpointsForTab(sourceTabId)) });
      return true;
    } catch {
      /* The controller emits a warning notice with the specific failure reason. */
      return false;
    } finally {
      dispatchTo(sourceTabId, { type: "message_action_done" });
    }
  }, [activeTabId, bumpSessionLoadSeq, confirmBackendActiveTab, dispatchTo, loadSessionDataForTab, scopedViewIdentity, syncActiveTabFromBackend, waitForBackendActiveTab, waitForTabReady]);

  // Tab management: switch preserves per-tab state; open creates it.
  const switchTab = useCallback(async (tabId: string, optimisticTab?: TabMeta): Promise<TabMeta[] | undefined> => {
    const activationSeq = surfaceActivationSeq.current + 1;
    surfaceActivationSeq.current = activationSeq;
    clearSurfaceTransition();
    const startedAt = Date.now();
    const targetSessionPath = optimisticTab?.sessionPath ?? statesRef.current.get(tabId)?.meta?.sessionPath;
    const retainedTarget = sessionViewTargetByTab.current.get(tabId);
    const optimisticTarget = optimisticTab ? surfaceTargetFromTab(optimisticTab, retainedTarget) : retainedTarget;
    const preserveCachedHistory = hasReusableCachedTranscript(statesRef.current.get(tabId), targetSessionPath);
    addBreadcrumb("tab.switch", `click ${tabId}`);
    if (optimisticTarget && !statesRef.current.has(tabId)) {
      statesRef.current.set(tabId, coldStateForSurfaceTarget(optimisticTarget));
    }
    setActiveTabId(tabId);
    activeTabIdRef.current = tabId;
    dispatchTo(tabId, { type: "backend_activation_start" });
    if (optimisticTab) {
      rememberTabView(tabId, optimisticTarget!, optimisticTab.topicTitle);
      dispatchTo(tabId, { type: "optimistic_meta", meta: metaFromTab(optimisticTab, statesRef.current.get(tabId)?.meta) });
    }
    dispatchTo(tabId, {
      type: "hydrate_start",
      reason: "switch-tab",
      surfaceIntent: surfaceIntentForTarget(targetSessionPath, optimisticTarget?.sessionTurns),
      preserveHistory: true,
    });
    addBreadcrumb("tab.switch", `active-rendered ${tabId} ms=${Date.now() - startedAt}`);
    const requestIsCurrent = () => surfaceActivationSeq.current === activationSeq && activeTabIdRef.current === tabId;
    const backendActivation = enqueueBackendActivation(async () => {
      if (!requestIsCurrent()) {
        dispatchTo(tabId, { type: "backend_activation_done" });
        addBreadcrumb("tab.switch", `set-active-skipped ${tabId} superseded`);
        return false;
      }
      try {
        await app.SetActiveTab(tabId);
        confirmBackendActiveTab(tabId);
        addBreadcrumb("tab.switch", `set-active-done ${tabId} ms=${Date.now() - startedAt}`);
        return requestIsCurrent();
      } catch (err) {
        dispatchTo(tabId, { type: "backend_activation_done" });
        if (!requestIsCurrent()) return false;
        dispatchTo(tabId, {
          type: "hydrate_error",
          reason: "switch-tab",
          error: visibleErrorMessage("session.switchTab", err, "切换会话失败，请稍后重试。"),
        });
        return false;
      }
    });
    trackBackendActivation(tabId, backendActivation);
    const backendSwitch = backendActivation
      .then(async (activated) => {
        if (!activated || !requestIsCurrent()) return undefined;
        void loadSessionDataForTab(tabId, false, "switch-tab", {
          skipHistory: hasCachedLiveTurn(statesRef.current.get(tabId)),
          preserveCachedHistory,
          sessionPath: targetSessionPath,
        });
        return reconcileTabRuntime(tabId, { hydrateSessionData: false, isCurrent: requestIsCurrent });
      })
      .catch((err) => {
        if (!requestIsCurrent()) return undefined;
        dispatchTo(tabId, {
          type: "hydrate_error",
          reason: "switch-tab",
          error: visibleErrorMessage("session.switchTab", err, "切换会话失败，请稍后重试。"),
        });
        return undefined;
      });
    return backendSwitch;
  }, [clearSurfaceTransition, confirmBackendActiveTab, dispatchTo, enqueueBackendActivation, loadSessionDataForTab, reconcileTabRuntime, rememberTabView, trackBackendActivation]);

  const openProjectTab = useCallback(async (workspaceRoot: string, topicId: string): Promise<TabMeta> => {
    surfaceActivationSeq.current += 1;
    const meta = await app.OpenProjectTab(workspaceRoot, topicId);
    const placeholderTabId = activeTabIdRef.current;
    const prevItems = placeholderTabId ? statesRef.current.get(placeholderTabId)?.items : undefined;
    const prevState = statesRef.current.get(meta.id);
    const isNewTab = !prevState;
    const preserveCachedHistory = hasReusableCachedTranscript(prevState, meta.sessionPath);
    const surfaceIntent = surfaceIntentForTarget(meta.sessionPath, undefined, "blank");
    clearSurfaceTransition();
    rememberTabView(meta.id, surfaceTargetFromTab(meta), meta.topicTitle);
    setActiveTabId(meta.id);
    activeTabIdRef.current = meta.id;
    confirmBackendActiveTab(meta.id);
    dispatchTo(meta.id, { type: "optimistic_meta", meta: metaFromTab(meta, statesRef.current.get(meta.id)?.meta) });
    void loadSessionDataForTab(meta.id, isNewTab, "open-topic", {
      placeholderItems: isNewTab && surfaceIntent !== "existing-session" ? prevItems : undefined,
      placeholderTabId: isNewTab && surfaceIntent !== "existing-session" ? placeholderTabId : undefined,
      preserveCachedHistory,
      sessionPath: meta.sessionPath,
      surfaceIntent,
    });
    return meta;
  }, [clearSurfaceTransition, confirmBackendActiveTab, dispatchTo, loadSessionDataForTab, rememberTabView]);

  const openGlobalTab = useCallback(async (topicId: string): Promise<TabMeta> => {
    surfaceActivationSeq.current += 1;
    const meta = await app.OpenGlobalTab(topicId);
    const placeholderTabId = activeTabIdRef.current;
    const prevItems = placeholderTabId ? statesRef.current.get(placeholderTabId)?.items : undefined;
    const prevState = statesRef.current.get(meta.id);
    const isNewTab = !prevState;
    const preserveCachedHistory = hasReusableCachedTranscript(prevState, meta.sessionPath);
    const surfaceIntent = surfaceIntentForTarget(meta.sessionPath, undefined, "blank");
    clearSurfaceTransition();
    rememberTabView(meta.id, surfaceTargetFromTab(meta), meta.topicTitle);
    setActiveTabId(meta.id);
    activeTabIdRef.current = meta.id;
    confirmBackendActiveTab(meta.id);
    dispatchTo(meta.id, { type: "optimistic_meta", meta: metaFromTab(meta, statesRef.current.get(meta.id)?.meta) });
    void loadSessionDataForTab(meta.id, isNewTab, "open-topic", {
      placeholderItems: isNewTab && surfaceIntent !== "existing-session" ? prevItems : undefined,
      placeholderTabId: isNewTab && surfaceIntent !== "existing-session" ? placeholderTabId : undefined,
      preserveCachedHistory,
      sessionPath: meta.sessionPath,
      surfaceIntent,
    });
    return meta;
  }, [clearSurfaceTransition, confirmBackendActiveTab, dispatchTo, loadSessionDataForTab, rememberTabView]);

  const openTopicSession = useCallback(async (scope: string, workspaceRoot: string, topicId: string, sessionPath: string): Promise<TabMeta> => {
    surfaceActivationSeq.current += 1;
    const meta = await app.OpenTopicSession(scope, workspaceRoot, topicId, sessionPath);
    const placeholderTabId = activeTabIdRef.current;
    const prevItems = placeholderTabId ? statesRef.current.get(placeholderTabId)?.items : undefined;
    const prevState = statesRef.current.get(meta.id);
    const isNewTab = !prevState;
    const preserveCachedHistory = hasReusableCachedTranscript(prevState, meta.sessionPath);
    const surfaceIntent = surfaceIntentForTarget(meta.sessionPath);
    clearSurfaceTransition();
    rememberTabView(meta.id, surfaceTargetFromTab(meta), meta.topicTitle);
    setActiveTabId(meta.id);
    activeTabIdRef.current = meta.id;
    confirmBackendActiveTab(meta.id);
    dispatchTo(meta.id, { type: "optimistic_meta", meta: metaFromTab(meta, statesRef.current.get(meta.id)?.meta) });
    void loadSessionDataForTab(meta.id, isNewTab, "open-topic", {
      placeholderItems: isNewTab && surfaceIntent !== "existing-session" ? prevItems : undefined,
      placeholderTabId: isNewTab && surfaceIntent !== "existing-session" ? placeholderTabId : undefined,
      preserveCachedHistory,
      sessionPath: meta.sessionPath,
      surfaceIntent,
    });
    return meta;
  }, [clearSurfaceTransition, confirmBackendActiveTab, dispatchTo, loadSessionDataForTab, rememberTabView]);

  const activateTopic = useCallback(async (
    scope: string,
    workspaceRoot: string,
    topicId: string,
    sessionPath = "",
    transitionId?: number,
    sessionTurns?: number,
    scene?: Scene,
    sceneState?: SceneState,
    composerIdentity?: ComposerSessionIdentity,
  ): Promise<TabMeta> => {
    const expertTraceId = currentComposerExpertTraceId();
    const activationSeq = transitionId ?? beginSurfaceTransition({ scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity });
    recordComposerExpertTrace("activate-topic.start", {
      transitionSeq: activationSeq,
      topicId,
      sessionPath,
    }, expertTraceId);
    let meta: TabMeta;
    try {
      meta = await app.ActivateTopic(scope, workspaceRoot, topicId, sessionPath);
    } catch (error) {
      failSurfaceTransition(activationSeq, error);
      throw error;
    }
    recordComposerExpertTrace("activate-topic.resolved", {
      transitionSeq: activationSeq,
      tabId: meta.id,
      sessionPath: meta.sessionPath ?? sessionPath,
    }, expertTraceId);
    const latestTabs = foregroundRunningFromRuntimeMeta(meta)
      ? asArray(await app.ListTabs().catch(() => [] as TabMeta[]))
      : [];
    if (surfaceActivationSeq.current !== activationSeq) return meta;
    let transition = surfaceTransitionRef.current?.seq === activationSeq
      ? surfaceTransitionRef.current
      : undefined;
    const runtimeMeta = latestTabs.find((tab) => tab.id === meta.id) ?? meta;
    const requestedTarget = transition?.target ?? { scope, workspaceRoot, topicId, sessionPath, sessionTurns, scene, sceneState, composerIdentity };
    const returnedSessionTurns = runtimeMeta.sessionTurns ?? meta.sessionTurns;
    const hasReturnedSceneStatus = runtimeMeta.scene !== undefined
      || runtimeMeta.sceneState !== undefined
      || meta.scene !== undefined
      || meta.sceneState !== undefined;
    const target = {
      ...requestedTarget,
      sessionPath: meta.sessionPath || sessionPath,
      sessionTurns: returnedSessionTurns ?? requestedTarget.sessionTurns,
      scene: hasReturnedSceneStatus ? runtimeMeta.scene ?? meta.scene : requestedTarget.scene,
      sceneState: hasReturnedSceneStatus ? runtimeMeta.sceneState ?? meta.sceneState : requestedTarget.sceneState,
      composerIdentity: {
        ...composerSessionIdentityFromSource(runtimeMeta, requestedTarget.composerIdentity?.expertKnown ?? false),
        expertKnown: requestedTarget.composerIdentity?.expertKnown ?? false,
        expertId: requestedTarget.composerIdentity?.expertId ?? "",
        expertName: requestedTarget.composerIdentity?.expertName ?? "",
      },
    };
    if (transition) {
      transition = {
        ...transition,
        target,
        state: {
          ...transition.state,
          surfaceIntent: surfaceIntentForTarget(target.sessionPath, target.sessionTurns),
        },
      };
      surfaceTransitionRef.current = transition;
      setSurfaceTransition(transition);
      recordComposerExpertTrace("transition.runtime-meta", {
        transitionSeq: activationSeq,
        targetExpertKnown: target.composerIdentity.expertKnown,
        targetExpertId: target.composerIdentity.expertId,
        targetExpertName: target.composerIdentity.expertName,
      }, expertTraceId);
    }
    const targetIdentityKey = sessionViewIdentityKey(scopedViewIdentity(target));
    const retainedTarget = sessionViewTargetByTab.current.get(meta.id);
    const retainedCandidate = retainedTarget
      && sessionViewIdentityKey(scopedViewIdentity(retainedTarget)) === targetIdentityKey
      ? statesRef.current.get(meta.id)
      : undefined;
    const selected = selectSessionView(target);
    const retainedState = selected.source?.writer.ownerId === meta.id ? selected.state : undefined;
    const carriedSnapshot = transition?.snapshot?.key === targetIdentityKey
      && sessionViewSnapshots.current.canCarry(transition.snapshot) ? transition.snapshot : undefined;
    const cachedSnapshot = !retainedState
      ? selected.snapshot ?? carriedSnapshot
      : undefined;
    if (transition && !cachedSnapshot && !retainedState) {
      downgradeSurfaceTransitionToCold(activationSeq);
    }
    pruneSessionRuntimeStates(meta.id);
    rememberTabView(meta.id, target, meta.topicTitle);
    if (cachedSnapshot) {
      // Retained runtime state and cursors stay with their owner. A different
      // accepted source supplies display only until this owner's actual replay.
      statesRef.current.set(meta.id, retainedCandidate
        ? { ...retainedCandidate, hydratePreview: cachedSnapshot, hydrateHistoryLoaded: false }
        : stateFromViewSnapshot(cachedSnapshot, false, true));
    } else if (!retainedCandidate) {
      statesRef.current.set(meta.id, coldStateForSurfaceTarget(target));
    }
    dispatchTo(meta.id, backendStatusActionFromTab(runtimeMeta));
    recordComposerExpertTrace("active-tab.apply", {
      transitionSeq: activationSeq,
      previousTabId: activeTabIdRef.current ?? "",
      nextTabId: meta.id,
    }, expertTraceId);
    setActiveTabId(meta.id);
    activeTabIdRef.current = meta.id;
    confirmBackendActiveTab(meta.id);
    dispatchTo(meta.id, { type: "optimistic_meta", meta: metaFromTab(meta, statesRef.current.get(meta.id)?.meta) });
    if (cachedSnapshot) {
      finishSurfaceTransition(activationSeq);
      void loadSessionDataForTab(meta.id, false, "open-topic", {
        sessionPath: target.sessionPath,
        waitUntilReady: true,
      });
      return meta;
    }
    if (retainedState) {
      finishSurfaceTransition(activationSeq);
      void loadSessionDataForTab(meta.id, false, "open-topic", {
        // Fully loaded transcripts keep their exact items, live pointers and
        // pagination. Only an incomplete initial load needs disk hydration.
        skipHistory: retainedState.hydrateHistoryLoaded === true,
        sessionPath: target.sessionPath,
        waitUntilReady: true,
      });
      return meta;
    }
    void loadSessionDataForTab(meta.id, true, "open-topic", {
      sessionPath: meta.sessionPath,
      surfaceIntent: surfaceIntentForTarget(meta.sessionPath, target.sessionTurns),
      waitUntilReady: true,
      surfaceTransitionSeq: activationSeq,
    });
    return meta;
  }, [beginSurfaceTransition, confirmBackendActiveTab, dispatchTo, downgradeSurfaceTransitionToCold, failSurfaceTransition, finishSurfaceTransition, loadSessionDataForTab, pruneSessionRuntimeStates, rememberTabView, scopedViewIdentity, selectSessionView]);

  // Ensure a blank tab exists for the given scope — reuses an existing one
  // or creates a new tab, then loads its session data.
  const ensureBlankTab = useCallback(async (scope: string, workspaceRoot: string): Promise<TabMeta> => {
    surfaceActivationSeq.current += 1;
    const meta = await app.EnsureBlankTab(scope, workspaceRoot);
    rememberTabView(meta.id, surfaceTargetFromTab(meta), meta.topicTitle);
    setActiveTabId(meta.id);
    activeTabIdRef.current = meta.id;
    confirmBackendActiveTab(meta.id);
    dispatchTo(meta.id, { type: "optimistic_meta", meta: metaFromTab(meta, statesRef.current.get(meta.id)?.meta) });
    void loadSessionDataForTab(meta.id, !statesRef.current.has(meta.id), "open-topic", {
      surfaceIntent: "blank",
    });
    return meta;
  }, [confirmBackendActiveTab, dispatchTo, loadSessionDataForTab, rememberTabView]);

  const loadOlderHistory = useCallback(async () => {
    const tabId = activeTabIdRef.current;
    const current = tabId ? statesRef.current.get(tabId) : undefined;
    if (!tabId || !current?.historyHasMore || current.historyLoadingOlder) return;
    const expectedLoadSeq = sessionLoadSeq.current.get(tabId);
    const expectedBefore = current.historyBefore;
    const expectedGeneration = current.historyGeneration;
    const target = sessionViewTargetByTab.current.get(tabId);
    const expectedIdentity = target ? sessionViewIdentityKey(scopedViewIdentity(target)) : "";
    dispatchTo(tabId, { type: "history_older_loading", loading: true });
    try {
      const page = await historyPageForTab(tabId, expectedBefore, expectedGeneration);
      const latest = statesRef.current.get(tabId);
      const latestTarget = sessionViewTargetByTab.current.get(tabId);
      const latestIdentity = latestTarget ? sessionViewIdentityKey(scopedViewIdentity(latestTarget)) : "";
      if (
        activeTabIdRef.current !== tabId
        || sessionLoadSeq.current.get(tabId) !== expectedLoadSeq
        || latest?.historyBefore !== expectedBefore
        || latest?.historyGeneration !== expectedGeneration
        || latestIdentity !== expectedIdentity
      ) {
        dispatchTo(tabId, { type: "history_older_loading", loading: false });
        return;
      }
      dispatchTo(tabId, {
        type: "history_prepend",
        messages: asArray<HistoryMessage>(page.messages),
        before: page.before,
        hasMore: page.hasMore,
        generation: page.generation,
      });
    } catch (error) {
      dispatchTo(tabId, { type: "history_older_loading", loading: false });
      addBreadcrumb("tab.hydrate", `older history failed ${tabId}: ${errorMessage(error)}`);
    }
  }, [dispatchTo, scopedViewIdentity]);

  const ensureBlankSurface = useCallback(async (scope: string, workspaceRoot: string, transitionId?: number): Promise<TabMeta> => {
    const activationSeq = transitionId ?? surfaceActivationSeq.current + 1;
    if (transitionId === undefined) surfaceActivationSeq.current = activationSeq;
    cacheIdleTabView(activeTabIdRef.current);
    let meta: TabMeta;
    try {
      meta = await app.EnsureBlankSurface(scope, workspaceRoot);
    } catch (error) {
      if (transitionId !== undefined) failSurfaceTransition(transitionId, error);
      throw error;
    }
    const latestTabs = foregroundRunningFromRuntimeMeta(meta)
      ? asArray(await app.ListTabs().catch(() => [] as TabMeta[]))
      : [];
    if (surfaceActivationSeq.current !== activationSeq) return meta;
    const runtimeMeta = latestTabs.find((tab) => tab.id === meta.id) ?? meta;
    pruneSessionRuntimeStates(meta.id);
    rememberTabView(meta.id, surfaceTargetFromTab(meta), meta.topicTitle);
    dispatchTo(meta.id, backendStatusActionFromTab(runtimeMeta));
    setActiveTabId(meta.id);
    activeTabIdRef.current = meta.id;
    confirmBackendActiveTab(meta.id);
    dispatchTo(meta.id, { type: "optimistic_meta", meta: metaFromTab(meta, statesRef.current.get(meta.id)?.meta) });
    if (transitionId === undefined) {
      void loadSessionDataForTab(meta.id, true, "open-topic", {
        surfaceIntent: "blank",
        waitUntilReady: true,
      });
    }
    return meta;
  }, [cacheIdleTabView, confirmBackendActiveTab, dispatchTo, failSurfaceTransition, loadSessionDataForTab, pruneSessionRuntimeStates, rememberTabView]);

  const closeTab = useCallback(async (tabId: string) => {
    try {
      await app.CloseTab(tabId);
      cacheIdleTabView(tabId);
      sessionViewSnapshots.current.releaseOwner(tabId);
      bumpSessionLoadSeq(tabId);
      transcriptLoads.current.delete(tabId);
      statesRef.current.delete(tabId);
      sessionViewTargetByTab.current.delete(tabId);
      sessionViewTitleByTab.current.delete(tabId);
      bump();
      if (tabId === activeTabId) await syncActiveTabFromBackend(true);
    } catch { /* ignore */ }
  }, [activeTabId, bump, bumpSessionLoadSeq, cacheIdleTabView, syncActiveTabFromBackend]);

  const reorderTabs = useCallback(async (tabIds: string[]) => {
    try {
      await app.ReorderTabs(tabIds);
    } catch { /* ignore */ }
  }, []);

  const renderedViewTarget = surfaceTransition?.target
    ?? (activeTabId ? sessionViewTargetByTab.current.get(activeTabId) : undefined);
  const renderedViewTitle = surfaceTransition?.snapshot?.title
    ?? (activeTabId ? sessionViewTitleByTab.current.get(activeTabId) : undefined);
  const renderedViewKey = renderedViewTarget
    ? sessionViewIdentityKey(scopedViewIdentity(renderedViewTarget))
    : "";

  return {
    state: activeState,
    activeTabId,
    surfaceTransitionActive: surfaceTransition !== null,
    surfaceLoadingHintVisible,
    renderedViewTarget,
    renderedViewTitle,
    sessionViewKey: renderedViewKey,
    sessionViewScroll: surfaceTransition?.snapshot?.scroll ?? sessionViewScrollByKey.current.get(renderedViewKey),
    updateSessionViewScroll,
    send, sendToTab, beforeSendRef, runShell, steer, notice, cancel, cancelTab, cancelForTab, approve, answerQuestion, setControllerMode, setCollaborationMode, setToolApprovalMode, syncToolApprovalMode, setGoal, clearGoal,
    newSession, clearSession, listSessions, listTrashedSessions, resumeSession, openChannelSession, revalidateChannelSession, previewSession, deleteSession, restoreSession, purgeTrashedSession, renameSession, loadOlderHistory,
    refreshMeta, pickWorkspace, switchWorkspace, compact, rewind, setModel, retryTabStartup, retryHistoryLoad, setEffort, setTokenMode,
    fetchMemory, remember, forget, saveDoc,
    switchTab, openProjectTab, openGlobalTab, openTopicSession, ensureBlankTab, beginSurfaceTransition, cancelSurfaceTransition, setSessionViewCacheScope, activateTopic, ensureBlankSurface, closeTab, reorderTabs,
    syncActiveTab: syncActiveTabFromBackend,
  };
}
